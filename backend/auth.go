package main

import (
	"errors"
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

func validateEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func createJWTToken(userID uint, email string, tenantID uint, role string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "supersecret"
	}
	claims := jwt.MapClaims{
		"sub":      userID,
		"email":    email,
		"tenantId": tenantID,
		"role":     role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
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
	user := User{Email: req.Email, Password: pw, TenantID: tenant.ID, Role: "user"}
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

	// Resolve tenant from URL path (e.g. /t/:tenant/login)
	tenantSlug := c.Params("tenant")
	if tenantSlug == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing tenant")
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "tenant not found")
	}

	var user User
	if err := db.Where("email = ? AND tenant_id = ?", req.Email, tenant.ID).First(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}
	if !checkPasswordHash(req.Password, user.Password) {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}
	tok, err := createJWTToken(user.ID, user.Email, user.TenantID, user.Role)
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
	if err := db.Where("email = ? AND role = ?", req.Email, "platform_admin").First(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}
	if !checkPasswordHash(req.Password, user.Password) {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	tok, err := createJWTToken(user.ID, user.Email, user.TenantID, user.Role)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create token")
	}
	return c.JSON(authResponse{Token: tok})
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
		Email:    email,
		Password: hash,
		Role:     "tenant_admin",
		TenantID: tenantID,
	}).Error
}

func ensurePlatformAdmin(email, password string) error {
	if email == "" || password == "" {
		return errors.New("email or password empty")
	}
	var count int64
	db.Model(&User{}).Where("email = ? AND role = ?", email, "platform_admin").Count(&count)
	if count > 0 {
		return nil
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return db.Create(&User{
		Email:    email,
		Password: hash,
		Role:     "platform_admin",
		TenantID: 0,
	}).Error
}

func companyRegisterHandler(c *fiber.Ctx) error {
	var req companyRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	// Validate input
	if !validateEmail(req.AdminEmail) || len(req.AdminPassword) < 6 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid email or password (min 6 chars)")
	}
	if len(req.CompanyName) < 2 {
		return fiber.NewError(fiber.StatusBadRequest, "company name must be at least 2 characters")
	}

	// Generate unique tenant slug from company name
	tenantSlug := generateTenantSlug(req.CompanyName)

	// Check if slug already exists
	var existingTenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&existingTenant).Error; err == nil {
		return fiber.NewError(fiber.StatusConflict, "company name already registered")
	}

	// Create new tenant
	tenant := Tenant{
		Slug: tenantSlug,
		Name: req.CompanyName,
	}
	if err := db.Create(&tenant).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create company")
	}

	// Hash password
	hashedPassword, err := hashPassword(req.AdminPassword)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to process password")
	}

	// Create admin user for this tenant
	adminUser := User{
		Email:    req.AdminEmail,
		Password: hashedPassword,
		Role:     "tenant_admin",
		TenantID: tenant.ID,
	}
	if err := db.Create(&adminUser).Error; err != nil {
		return fiber.NewError(fiber.StatusConflict, "admin email already registered")
	}

	// Create JWT token
	token, err := createJWTToken(adminUser.ID, adminUser.Email, tenant.ID, adminUser.Role)
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
	// Remove invalid characters
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
