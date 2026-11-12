package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
}

type companyRegisterRequest struct {
	CompanyName   string `json:"company_name"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
}

type companyRegisterResponse struct {
	TenantID string `json:"tenant_id"`
	Token    string `json:"token"`
	Message  string `json:"message"`
}

var commonPasswords = map[string]bool{
	"password":      true,
	"password123":   true,
	"admin12345":    true,
	"admin123456":   true,
	"1234567890":    true,
	"12345678901":   true,
	"qwerty12345":   true,
	"welcome123":    true,
	"letmein123":    true,
	"platform123":   true,
	"platformadmin": true,
}

func validateEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func validatePasswordPolicy(password string) error {
	p := strings.TrimSpace(password)
	if len(p) < 10 {
		return errors.New("password must be at least 10 characters long")
	}
	if commonPasswords[strings.ToLower(p)] {
		return errors.New("password is too common or insecure; please choose a stronger password")
	}
	return nil
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// createPlatformAdminToken generates short-lived tokens (2h) specifically for platform admins with aud: platform_admin and no tenant_id
func createPlatformAdminToken(userID uint, email string) (string, error) {
	secret := getJWTSecret()
	if len(secret) == 0 {
		return "", errors.New("JWT secret not configured")
	}
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"aud":   "platform_admin",
		"role":  RolePlatformAdmin,
		"exp":   time.Now().Add(2 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// createTenantJWTToken generates tenant-scoped tokens with token_version and aud: tenant_user
func createTenantJWTToken(userID uint, email string, tenantID uint, role string, tokenVersion int) (string, error) {
	secret := getJWTSecret()
	if len(secret) == 0 {
		return "", errors.New("JWT secret not configured")
	}
	claims := jwt.MapClaims{
		"sub":           userID,
		"email":         email,
		"aud":           "tenant_user",
		"tenantId":      tenantID,
		"role":          role,
		"token_version": tokenVersion,
		"exp":           time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// Legacy helper compatibility
func createJWTToken(userID uint, email string, tenantID uint, role string) (string, error) {
	if role == RolePlatformAdmin {
		return createPlatformAdminToken(userID, email)
	}
	return createTenantJWTToken(userID, email, tenantID, role, 1)
}

func registerHandler(c *fiber.Ctx) error {
	var req authRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if !validateEmail(req.Email) || len(req.Password) < 6 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid email or password")
	}

	tenantSlug := c.Params("tenant")
	if tenantSlug == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing tenant")
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "tenant not found")
	}
	pw, err := hashPassword(req.Password)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to hash password")
	}

	// Strictly assign RoleStaff; client cannot supply role
	user := User{
		Email:              strings.ToLower(strings.TrimSpace(req.Email)),
		Password:           pw,
		TenantID:           tenant.ID,
		Role:               RoleStaff,
		Status:             "active",
		MustChangePassword: false,
		TokenVersion:       1,
	}
	if err := db.Create(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "user may already exist")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "registered"})
}

func loginHandler(c *fiber.Ctx) error {
	var req authRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if !validateEmail(req.Email) || len(req.Password) < 6 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid email or password")
	}

	tenantSlug := c.Params("tenant")
	if tenantSlug == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing tenant")
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "tenant not found")
	}

	var user User
	if err := db.Where("email = ? AND tenant_id = ?", strings.ToLower(strings.TrimSpace(req.Email)), tenant.ID).First(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	if user.Status == "disabled" {
		return fiber.NewError(fiber.StatusForbidden, "account is disabled; please contact your administrator")
	}

	if !checkPasswordHash(req.Password, user.Password) {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	tok, err := createTenantJWTToken(user.ID, user.Email, user.TenantID, user.Role, user.TokenVersion)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create token")
	}
	return c.JSON(authResponse{Token: tok})
}

func platformLoginHandler(c *fiber.Ctx) error {
	var req authRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if !validateEmail(req.Email) || len(req.Password) < 6 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid email or password")
	}

	var user User
	if err := db.Where("email = ? AND role = ?", strings.ToLower(strings.TrimSpace(req.Email)), RolePlatformAdmin).First(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	if user.Status == "disabled" {
		return fiber.NewError(fiber.StatusForbidden, "account is disabled")
	}

	if !checkPasswordHash(req.Password, user.Password) {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	tok, err := createPlatformAdminToken(user.ID, user.Email)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create token")
	}
	return c.JSON(authResponse{Token: tok})
}

// bootstrapPlatformAdmin performs optional one-time bootstrap if zero platform admins exist
func bootstrapPlatformAdmin(appEnv string) {
	var count int64
	db.Model(&User{}).Where("role = ?", RolePlatformAdmin).Count(&count)
	if count > 0 {
		return
	}

	platEmail := strings.TrimSpace(os.Getenv("PLATFORM_ADMIN_EMAIL"))
	platPass := strings.TrimSpace(os.Getenv("PLATFORM_ADMIN_PASSWORD"))

	if strings.EqualFold(appEnv, "development") {
		if platEmail == "" {
			platEmail = getEnv("DEFAULT_PLATFORM_ADMIN_EMAIL", "platform@example.com")
		}
		if platPass == "" {
			platPass = getEnv("DEFAULT_PLATFORM_ADMIN_PASSWORD", "platformAdmin123!")
		}
	}

	if platEmail == "" || platPass == "" {
		return
	}

	if err := validatePasswordPolicy(platPass); err != nil {
		log.Printf("[bootstrap warning] PLATFORM_ADMIN_PASSWORD failed policy: %v", err)
		return
	}

	hashed, err := hashPassword(platPass)
	if err != nil {
		log.Printf("[bootstrap error] Failed to hash platform admin password: %v", err)
		return
	}

	admin := User{
		Email:              strings.ToLower(platEmail),
		Password:           hashed,
		Role:               RolePlatformAdmin,
		Status:             "active",
		MustChangePassword: false,
		TokenVersion:       1,
		TenantID:           0,
	}
	if err := db.Create(&admin).Error; err != nil {
		log.Printf("[bootstrap error] Failed to create bootstrap platform admin: %v", err)
		return
	}
	if !strings.EqualFold(appEnv, "development") {
		log.Printf("[SECURITY WARNING] Created initial platform admin '%s' from environment variables. PLEASE REMOVE PLATFORM_ADMIN_EMAIL and PLATFORM_ADMIN_PASSWORD from your environment immediately!", platEmail)
	}
}

func ensureDefaultAdmin(tenantID uint, email, password string) error {
	if email == "" || password == "" {
		return errors.New("email or password empty")
	}
	var count int64
	db.Model(&User{}).Where("email = ? AND tenant_id = ?", email, tenantID).Count(&count)
	if count > 0 {
		return nil
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return db.Create(&User{
		Email:              email,
		Password:           hash,
		Role:               RoleTenantAdmin,
		Status:             "active",
		MustChangePassword: false,
		TokenVersion:       1,
		TenantID:           tenantID,
	}).Error
}

func companyRegisterHandler(c *fiber.Ctx) error {
	var req companyRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	if !validateEmail(req.AdminEmail) || len(req.AdminPassword) < 6 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid email or password (min 6 chars)")
	}
	if len(req.CompanyName) < 2 {
		return fiber.NewError(fiber.StatusBadRequest, "company name must be at least 2 characters")
	}

	tenantSlug := generateTenantSlug(req.CompanyName)

	var existingTenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&existingTenant).Error; err == nil {
		return fiber.NewError(fiber.StatusConflict, "company name already registered")
	}

	tenant := Tenant{
		Slug: tenantSlug,
		Name: req.CompanyName,
	}
	if err := db.Create(&tenant).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create company")
	}

	hashedPassword, err := hashPassword(req.AdminPassword)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to process password")
	}

	// Always creates RoleTenantAdmin - never accepts role from request
	adminUser := User{
		Email:              strings.ToLower(strings.TrimSpace(req.AdminEmail)),
		Password:           hashedPassword,
		Role:               RoleTenantAdmin,
		Status:             "active",
		MustChangePassword: false,
		TokenVersion:       1,
		TenantID:           tenant.ID,
	}
	if err := db.Create(&adminUser).Error; err != nil {
		return fiber.NewError(fiber.StatusConflict, "admin email already registered")
	}

	token, err := createTenantJWTToken(adminUser.ID, adminUser.Email, tenant.ID, adminUser.Role, adminUser.TokenVersion)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create token")
	}

	return c.Status(fiber.StatusCreated).JSON(companyRegisterResponse{
		TenantID: tenantSlug,
		Token:    token,
		Message:  "company registered successfully",
	})
}

func generateTenantSlug(name string) string {
	s := strings.ToLower(name)
	s = strings.TrimSpace(s)
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func createUserHandler(c *fiber.Ctx) error {
	return createTenantUserHandler(c)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func changePasswordHandler(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	var req changePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	if err := validatePasswordPolicy(req.NewPassword); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var user User
	if err := db.First(&user, userID).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user not found")
	}

	if !checkPasswordHash(req.CurrentPassword, user.Password) {
		return fiber.NewError(fiber.StatusUnauthorized, "current password is incorrect")
	}

	hashedPassword, err := hashPassword(req.NewPassword)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to hash password")
	}

	// Bump token_version to invalidate existing tokens on password change
	user.Password = hashedPassword
	user.MustChangePassword = false
	user.TokenVersion++
	if err := db.Save(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update password")
	}

	return c.JSON(fiber.Map{"message": "password changed successfully"})
}

// runCreatePlatformAdminCLI creates a platform admin from CLI subcommand with stdin password prompt
func runCreatePlatformAdminCLI(args []string) {
	fs := flag.NewFlagSet("create-platform-admin", flag.ExitOnError)
	emailFlag := fs.String("email", "", "Platform admin email address")
	_ = fs.Parse(args)

	cleanEmail := strings.ToLower(strings.TrimSpace(*emailFlag))
	if cleanEmail == "" || !validateEmail(cleanEmail) {
		fmt.Fprintln(os.Stderr, "Error: valid --email flag is required")
		os.Exit(1)
	}

	var count int64
	db.Model(&User{}).Where("email = ?", cleanEmail).Count(&count)
	if count > 0 {
		fmt.Fprintf(os.Stderr, "Error: user with email '%s' already exists\n", cleanEmail)
		os.Exit(1)
	}

	fmt.Print("Enter platform admin password: ")
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "Error reading password from stdin: %v\n", err)
		os.Exit(1)
	}
	cleanPass := strings.TrimSpace(password)

	if err := validatePasswordPolicy(cleanPass); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	hashed, err := hashPassword(cleanPass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error hashing password: %v\n", err)
		os.Exit(1)
	}

	admin := User{
		Email:              cleanEmail,
		Password:           hashed,
		Role:               RolePlatformAdmin,
		Status:             "active",
		MustChangePassword: false,
		TokenVersion:       1,
		TenantID:           0,
	}

	if err := db.Create(&admin).Error; err != nil {
		fmt.Fprintf(os.Stderr, "Error creating platform admin in database: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully created platform admin: %s\n", cleanEmail)
	os.Exit(0)
}
