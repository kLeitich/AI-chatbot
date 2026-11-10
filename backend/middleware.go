package main

import (
	"log"
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
		log.Println("[security warning] JWT_SECRET is not set; using insecure default fallback.")
	}
	tok, err := jwt.Parse(tokStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "unexpected signing method")
		}
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

// tenantAdminMiddleware ensures only tenant admins (or platform admins) can access admin routes
// and prevents cross-tenant access.
func tenantAdminMiddleware(c *fiber.Ctx) error {
	role, ok := c.Locals("role").(string)
	if !ok || (role != "tenant_admin" && role != "platform_admin") {
		return fiber.NewError(fiber.StatusForbidden, "tenant admin access required")
	}

	// platform_admin can access any tenant's admin
	if role == "platform_admin" {
		return c.Next()
	}

	// Ensure tenant in route matches tenant in token
	tenantSlug := c.Params("tenant")
	if tenantSlug != "" {
		tokenTenantID, ok := c.Locals("tenant_id").(uint)
		if !ok || tokenTenantID == 0 {
			return fiber.NewError(fiber.StatusUnauthorized, "tenant not found in token")
		}

		var targetTenant Tenant
		if err := db.Where("slug = ?", tenantSlug).First(&targetTenant).Error; err != nil {
			return fiber.NewError(fiber.StatusNotFound, "tenant not found")
		}

		if targetTenant.ID != tokenTenantID {
			return fiber.NewError(fiber.StatusForbidden, "access denied for this tenant")
		}
	}

	return c.Next()
}
