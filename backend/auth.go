package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token              string `json:"token"`
	Role               string `json:"role,omitempty"`
	TenantID           string `json:"tenant_id,omitempty"`
	TenantSlug         string `json:"tenant_slug,omitempty"`
	MustChangePassword bool   `json:"must_change_password,omitempty"`
	RedirectURL        string `json:"redirect_url,omitempty"`
}

type companyRegisterRequest struct {
	CompanyName   string `json:"company_name"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
}

type companyRegisterResponse struct {
	TenantID    string   `json:"tenant_id"`
	Token       string   `json:"token"`
	Role        string   `json:"role,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Message     string   `json:"message"`
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
		recordAuditLog(0, "anonymous", tenant.ID, "login_failure", "auth", 0, c.IP())
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	if user.Status == "disabled" {
		recordAuditLog(user.ID, user.Role, tenant.ID, "login_failure_disabled", "auth", user.ID, c.IP())
		return sendError(c, fiber.StatusUnauthorized, ErrCodeAccountDisabled, "account is disabled; please contact your administrator")
	}

	if !checkPasswordHash(req.Password, user.Password) {
		recordAuditLog(user.ID, user.Role, tenant.ID, "login_failure", "auth", user.ID, c.IP())
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	tok, err := createTenantJWTToken(user.ID, user.Email, user.TenantID, user.Role, user.TokenVersion)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create token")
	}
	recordAuditLog(user.ID, user.Role, user.TenantID, "login_success", "auth", user.ID, c.IP())
	redirectURL := fmt.Sprintf("/t/%s/admin/dashboard", tenant.Slug)
	return c.JSON(authResponse{
		Token:              tok,
		Role:               user.Role,
		TenantID:           tenant.Slug,
		TenantSlug:         tenant.Slug,
		MustChangePassword: user.MustChangePassword,
		RedirectURL:        redirectURL,
	})
}

// globalLoginHandler authenticates any user (platform admin, tenant admin, or staff) without requiring a tenant slug in the URL.
// It automatically determines the user's role and associated company, returning the appropriate token and target redirect URL.
func globalLoginHandler(c *fiber.Ctx) error {
	var req authRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	cleanEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if !validateEmail(cleanEmail) || len(req.Password) < 6 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid email or password")
	}

	var users []User
	if err := db.Where("email = ?", cleanEmail).Find(&users).Error; err != nil || len(users) == 0 {
		recordAuditLog(0, "anonymous", 0, "login_failure", "auth", 0, c.IP())
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	var matchedUser *User
	for i := range users {
		if checkPasswordHash(req.Password, users[i].Password) {
			matchedUser = &users[i]
			break
		}
	}
	if matchedUser == nil {
		recordAuditLog(0, "anonymous", 0, "login_failure", "auth", 0, c.IP())
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	if matchedUser.Status == "disabled" {
		recordAuditLog(matchedUser.ID, matchedUser.Role, matchedUser.TenantID, "login_failure_disabled", "auth", matchedUser.ID, c.IP())
		return sendError(c, fiber.StatusUnauthorized, ErrCodeAccountDisabled, "account is disabled; please contact your administrator")
	}

	// Platform admin authentication
	if matchedUser.Role == RolePlatformAdmin {
		tok, err := createPlatformAdminToken(matchedUser.ID, matchedUser.Email)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to create token")
		}
		recordAuditLog(matchedUser.ID, matchedUser.Role, 0, "login_success", "platform_auth", matchedUser.ID, c.IP())
		return c.JSON(authResponse{
			Token:              tok,
			Role:               RolePlatformAdmin,
			MustChangePassword: matchedUser.MustChangePassword,
			RedirectURL:        "/platform/dashboard",
		})
	}

	// Tenant users (tenant_admin or staff)
	var tenant Tenant
	if err := db.First(&tenant, matchedUser.TenantID).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "tenant not found for user")
	}

	tok, err := createTenantJWTToken(matchedUser.ID, matchedUser.Email, matchedUser.TenantID, matchedUser.Role, matchedUser.TokenVersion)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create token")
	}
	recordAuditLog(matchedUser.ID, matchedUser.Role, matchedUser.TenantID, "login_success", "auth", matchedUser.ID, c.IP())

	redirectURL := fmt.Sprintf("/t/%s/admin/dashboard", tenant.Slug)
	return c.JSON(authResponse{
		Token:              tok,
		Role:               matchedUser.Role,
		TenantID:           tenant.Slug,
		TenantSlug:         tenant.Slug,
		MustChangePassword: matchedUser.MustChangePassword,
		RedirectURL:        redirectURL,
	})
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
		recordAuditLog(0, "anonymous", 0, "login_failure", "platform_auth", 0, c.IP())
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	if user.Status == "disabled" {
		recordAuditLog(user.ID, user.Role, 0, "login_failure_disabled", "platform_auth", user.ID, c.IP())
		return sendError(c, fiber.StatusUnauthorized, ErrCodeAccountDisabled, "account is disabled")
	}

	if !checkPasswordHash(req.Password, user.Password) {
		recordAuditLog(user.ID, user.Role, 0, "login_failure", "platform_auth", user.ID, c.IP())
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	tok, err := createPlatformAdminToken(user.ID, user.Email)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create token")
	}
	recordAuditLog(user.ID, user.Role, 0, "login_success", "platform_auth", user.ID, c.IP())
	return c.JSON(authResponse{
		Token:       tok,
		Role:        RolePlatformAdmin,
		RedirectURL: "/platform/dashboard",
	})
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
	recordAuditLog(admin.ID, RolePlatformAdmin, 0, "create_platform_admin", "user", admin.ID, "bootstrap")
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

	if !validateEmail(req.AdminEmail) {
		return fiber.NewError(fiber.StatusBadRequest, "invalid email address")
	}
	if err := validatePasswordPolicy(req.AdminPassword); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if len(strings.TrimSpace(req.CompanyName)) < 2 {
		return fiber.NewError(fiber.StatusBadRequest, "company name must be at least 2 characters")
	}

	tenantSlug := generateTenantSlug(req.CompanyName)
	if len(tenantSlug) < 2 {
		return fiber.NewError(fiber.StatusBadRequest, "company name must contain at least 2 alphanumeric characters")
	}

	reservedSlugs := map[string]bool{
		"platform": true, "admin": true, "auth": true, "api": true, "root": true,
		"system": true, "default": true, "login": true, "register": true, "settings": true,
		"dashboard": true, "health": true, "embed": true, "book": true,
	}
	if reservedSlugs[tenantSlug] {
		return fiber.NewError(fiber.StatusBadRequest, "company name is reserved")
	}

	var existingTenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&existingTenant).Error; err == nil {
		return fiber.NewError(fiber.StatusConflict, "company name already registered")
	}

	tenant := Tenant{
		Slug:                tenantSlug,
		Name:                req.CompanyName,
		BookingLinkEnabled:  true,
		AllowedEmbedOrigins: "[]",
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
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&adminUser).Error; err != nil {
			return err
		}
		return assignUserPermissions(tx, adminUser.ID, AllPermissions)
	}); err != nil {
		return fiber.NewError(fiber.StatusConflict, "admin email already registered")
	}

	token, err := createTenantJWTToken(adminUser.ID, adminUser.Email, tenant.ID, adminUser.Role, adminUser.TokenVersion)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create token")
	}

	return c.Status(fiber.StatusCreated).JSON(companyRegisterResponse{
		TenantID:    tenantSlug,
		Token:       token,
		Role:        adminUser.Role,
		Permissions: append([]string(nil), AllPermissions...),
		Message:     "company registered successfully",
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

	recordAuditLog(user.ID, user.Role, user.TenantID, "change_password", "user", user.ID, c.IP())

	return c.JSON(fiber.Map{"message": "password changed successfully"})
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func hashResetToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// sendPasswordResetEmail sends the reset link via configured SMTP or logs safely
func sendPasswordResetEmail(recipientEmail, resetLink string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := getEnv("SMTP_PORT", "587")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	smtpFrom := getEnv("SMTP_FROM", smtpUser)

	if smtpHost != "" && smtpUser != "" && smtpPass != "" {
		auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
		subject := "Subject: Reset your Booka password\r\n"
		mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\n\r\n"
		body := fmt.Sprintf(`<html><body>
			<h2>Booka Password Reset</h2>
			<p>We received a request to reset your password. Click the link below to set a new password:</p>
			<p><a href="%s" style="padding: 10px 18px; background-color: #2563eb; color: #ffffff; text-decoration: none; border-radius: 6px; font-weight: bold;">Reset Password</a></p>
			<p>Or copy and paste this link in your browser:</p>
			<p><a href="%s">%s</a></p>
			<p>This link expires in 1 hour. If you did not request this, you can safely ignore this email.</p>
		</body></html>`, resetLink, resetLink, resetLink)
		msg := []byte(subject + mime + body)
		addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
		if err := smtp.SendMail(addr, auth, smtpFrom, []string{recipientEmail}, msg); err != nil {
			log.Printf("[EMAIL ERROR] Failed to send password reset email to %s: %v", recipientEmail, err)
			return err
		}
		log.Printf("[EMAIL] Password reset email successfully dispatched to %s", recipientEmail)
		return nil
	}

	// Development / Fallback mode: log safely to server logs
	log.Printf("[AUTH] Password reset requested for '%s'. Reset link: %s", recipientEmail, resetLink)
	return nil
}

func forgotPasswordHandler(c *fiber.Ctx) error {
	var req forgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	cleanEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if cleanEmail == "" || !validateEmail(cleanEmail) {
		return fiber.NewError(fiber.StatusBadRequest, "valid email address is required")
	}

	genericSuccess := fiber.Map{
		"message": "If an account exists with this email address, a password reset link has been sent.",
	}

	var user User
	if err := db.Where("email = ?", cleanEmail).First(&user).Error; err != nil {
		return c.JSON(genericSuccess)
	}

	if user.Status == "disabled" {
		return c.JSON(genericSuccess)
	}

	tokenBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, tokenBytes); err != nil {
		log.Printf("[AUTH ERROR] Failed to generate reset token: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to generate security token")
	}
	rawToken := hex.EncodeToString(tokenBytes)
	tokenHash := hashResetToken(rawToken)

	// Invalidate any previous unused reset tokens for this user
	db.Model(&PasswordResetToken{}).Where("user_id = ? AND used = ?", user.ID, false).Update("used", true)

	resetRecord := PasswordResetToken{
		UserID:    user.ID,
		Email:     user.Email,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Used:      false,
	}
	if err := db.Create(&resetRecord).Error; err != nil {
		log.Printf("[AUTH ERROR] Failed to store password reset token: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to process request")
	}

	frontendBase := getEnv("FRONTEND_URL", "https://www.booka.co.ke")
	if strings.Contains(frontendBase, ",") {
		frontendBase = strings.TrimSpace(strings.Split(frontendBase, ",")[0])
	}
	frontendBase = strings.TrimRight(frontendBase, "/")
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendBase, rawToken)

	_ = sendPasswordResetEmail(user.Email, resetLink)

	return c.JSON(genericSuccess)
}

func resetPasswordHandler(c *fiber.Ctx) error {
	var req resetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	rawToken := strings.TrimSpace(req.Token)
	if rawToken == "" || len(rawToken) < 32 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid or missing reset token")
	}

	if err := validatePasswordPolicy(req.NewPassword); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	tokenHash := hashResetToken(rawToken)

	var resetRecord PasswordResetToken
	if err := db.Where("token_hash = ? AND used = ? AND expires_at > ?", tokenHash, false, time.Now()).First(&resetRecord).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "password reset link is invalid or has expired")
	}

	var user User
	if err := db.First(&user, resetRecord.UserID).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user not found")
	}

	if user.Status == "disabled" {
		return fiber.NewError(fiber.StatusForbidden, "account is disabled; please contact support")
	}

	hashedPassword, err := hashPassword(req.NewPassword)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to process password")
	}

	user.Password = hashedPassword
	user.MustChangePassword = false
	user.TokenVersion++
	if err := db.Save(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update password")
	}

	resetRecord.Used = true
	db.Save(&resetRecord)

	recordAuditLog(user.ID, user.Role, user.TenantID, "reset_password", "user", user.ID, c.IP())

	return c.JSON(fiber.Map{
		"message": "password reset successfully. You can now sign in with your new password.",
	})
}

// runCreatePlatformAdminCLI creates a platform admin from CLI subcommand with flag, positional, or stdin password prompt
func runCreatePlatformAdminCLI(args []string) {
	fs := flag.NewFlagSet("create-platform-admin", flag.ExitOnError)
	emailFlag := fs.String("email", "", "Platform admin email address")
	passwordFlag := fs.String("password", "", "Platform admin password (optional, prompted if omitted)")
	_ = fs.Parse(args)

	cleanEmail := strings.ToLower(strings.TrimSpace(*emailFlag))
	if cleanEmail == "" && len(fs.Args()) > 0 {
		cleanEmail = strings.ToLower(strings.TrimSpace(fs.Args()[0]))
	}

	if cleanEmail == "" || !validateEmail(cleanEmail) {
		fmt.Fprintln(os.Stderr, "Error: valid --email flag or positional email argument is required")
		os.Exit(1)
	}

	var count int64
	db.Model(&User{}).Where("email = ?", cleanEmail).Count(&count)
	if count > 0 {
		fmt.Fprintf(os.Stderr, "Error: user with email '%s' already exists\n", cleanEmail)
		os.Exit(1)
	}

	cleanPass := strings.TrimSpace(*passwordFlag)
	if cleanPass == "" && len(fs.Args()) > 1 {
		cleanPass = strings.TrimSpace(fs.Args()[1])
	}

	if cleanPass == "" {
		fmt.Print("Enter platform admin password: ")
		reader := bufio.NewReader(os.Stdin)
		password, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			fmt.Fprintf(os.Stderr, "Error reading password from stdin: %v\n", err)
			os.Exit(1)
		}
		cleanPass = strings.TrimSpace(password)
	}

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

	recordAuditLog(admin.ID, RolePlatformAdmin, 0, "create_platform_admin", "user", admin.ID, "cli")

	fmt.Printf("Successfully created platform admin: %s\n", cleanEmail)
	os.Exit(0)
}
