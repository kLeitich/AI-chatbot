package main

import (
	"errors"
	"net/mail"
	"os"
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

func createJWTToken(userID uint, email string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "supersecret"
	}
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
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
	pw, err := hashPassword(req.Password)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to hash password")
	}
	user := User{Email: req.Email, Password: pw}
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
	var user User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}
	if !checkPasswordHash(req.Password, user.Password) {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}
	tok, err := createJWTToken(user.ID, user.Email)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create token")
	}
	return c.JSON(authResponse{Token: tok})
}

func ensureDefaultAdmin(email, password string) error {
	if email == "" || password == "" {
		return errors.New("email or password empty")
	}
	var count int64
	db.Model(&User{}).Where("email = ?", email).Count(&count)
	if count > 0 {
		return nil
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return db.Create(&User{Email: email, Password: hash}).Error
}
