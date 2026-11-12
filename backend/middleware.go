package main

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func jwtMiddleware(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if auth == "" || len(auth) < 8 || !strings.HasPrefix(auth, "Bearer ") {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "missing token")
	}
	tokStr := strings.TrimSpace(auth[7:])
	secret := getJWTSecret()
	if len(secret) == 0 {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "server configuration error: JWT secret not initialized")
	}
	tok, err := jwt.Parse(tokStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !tok.Valid {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "invalid or expired token")
	}

	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "invalid token claims")
	}

	subVal, ok := claims["sub"]
	if !ok {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "invalid token subject")
	}
	userIDFloat, ok := subVal.(float64)
	if !ok {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "invalid token subject type")
	}
	userID := uint(userIDFloat)

	aud, _ := claims["aud"].(string)

	// Fetch user from database to ensure account is active and token_version is current
	var user User
	if err := db.Select("id, email, role, status, must_change_password, token_version, tenant_id").First(&user, userID).Error; err != nil {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeAccountDisabled, "user not found or deactivated")
	}

	if user.Status == "disabled" {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeAccountDisabled, "account is disabled; please contact support")
	}

	// Validate token_version if present in claims
	if tvClaim, exists := claims["token_version"]; exists {
		if tvFloat, ok := tvClaim.(float64); ok {
			if int(tvFloat) != user.TokenVersion {
				return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenRevoked, "token has been revoked")
			}
		}
	}

	c.Locals("user_id", user.ID)
	c.Locals("email", user.Email)
	c.Locals("aud", aud)
	c.Locals("role", user.Role)
	c.Locals("tenant_id", user.TenantID)
	c.Locals("must_change_password", user.MustChangePassword)

	return c.Next()
}

func platformAdminMiddleware(c *fiber.Ctx) error {
	aud, _ := c.Locals("aud").(string)
	role, _ := c.Locals("role").(string)
	if aud != "platform_admin" || role != RolePlatformAdmin {
		return sendError(c, fiber.StatusForbidden, ErrCodePermissionDenied, "platform admin access required")
	}
	return c.Next()
}

// validateTenantUser validates that the token belongs to a tenant user and matches the URL tenant
func validateTenantUser(c *fiber.Ctx) error {
	aud, _ := c.Locals("aud").(string)
	if aud != "tenant_user" {
		return sendError(c, fiber.StatusForbidden, ErrCodePermissionDenied, "tenant user token required")
	}

	tenantSlug := c.Params("tenant")
	if tenantSlug != "" {
		tokenTenantID, ok := c.Locals("tenant_id").(uint)
		if !ok || tokenTenantID == 0 {
			return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "tenant not found in token")
		}

		var targetTenant Tenant
		if err := db.Where("slug = ?", tenantSlug).First(&targetTenant).Error; err != nil {
			return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "tenant not found")
		}

		if targetTenant.ID != tokenTenantID {
			return sendError(c, fiber.StatusForbidden, ErrCodePermissionDenied, "access denied for this tenant")
		}
	}

	return nil
}

// tenantUserMiddleware ensures the caller has a valid tenant_user token matching the URL tenant slug.
func tenantUserMiddleware(c *fiber.Ctx) error {
	if err := validateTenantUser(c); err != nil {
		return err
	}
	return c.Next()
}

// tenantAdminMiddleware ensures only tenant admins can access admin routes and prevents cross-tenant access.
// Platform admin tokens are strictly forbidden on tenant admin endpoints.
func tenantAdminMiddleware(c *fiber.Ctx) error {
	if err := validateTenantUser(c); err != nil {
		return err
	}

	role, _ := c.Locals("role").(string)
	if role != RoleTenantAdmin {
		return sendError(c, fiber.StatusForbidden, ErrCodePermissionDenied, "tenant admin access required")
	}

	return c.Next()
}
