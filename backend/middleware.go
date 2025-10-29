package main

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func jwtMiddleware(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if auth == "" || len(auth) < 8 || auth[:7] != "Bearer " {
		return fiber.NewError(fiber.StatusUnauthorized, "missing token")
	}
	tokStr := auth[7:]
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "supersecret"
	}
	tok, err := jwt.Parse(tokStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid token")
	}
	return c.Next()
}
