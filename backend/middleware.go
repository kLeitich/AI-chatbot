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

	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid token claims")
	}

	// Extract user and tenant IDs from token for downstream handlers
	if sub, ok := claims["sub"].(float64); ok {
		c.Locals("user_id", uint(sub))
	}
	if tid, ok := claims["tenantId"].(float64); ok {
		c.Locals("tenant_id", uint(tid))
	}
	if role, ok := claims["role"].(string); ok {
		c.Locals("role", role)
	}

	return c.Next()
}

func platformAdminMiddleware(c *fiber.Ctx) error {
	role, ok := c.Locals("role").(string)
	if !ok || role != "platform_admin" {
		return fiber.NewError(fiber.StatusForbidden, "platform admin access required")
	}
	return c.Next()
}
