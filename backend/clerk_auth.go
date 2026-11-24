package main

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type ClerkJWKS struct {
	Keys []ClerkJSONWebKey `json:"keys"`
}

type ClerkJSONWebKey struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

var (
	clerkJWKSCache     = map[string]*rsa.PublicKey{}
	clerkJWKSCacheMu   sync.RWMutex
	clerkJWKSCacheExp  time.Time
	clerkHTTPClient    = &http.Client{Timeout: 5 * time.Second}
)

// parseRSAPublicKey parses RSA modulus (n) and exponent (e) from Base64URL
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	var eInt int
	for _, b := range eBytes {
		eInt = (eInt << 8) | int(b)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: eInt,
	}, nil
}

// fetchClerkJWKS retrieves and caches RSA public keys from Clerk's JWKS endpoint
func fetchClerkPublicKey(kid string, tenant Tenant) (*rsa.PublicKey, error) {
	clerkJWKSCacheMu.RLock()
	if time.Now().Before(clerkJWKSCacheExp) {
		if key, exists := clerkJWKSCache[kid]; exists {
			clerkJWKSCacheMu.RUnlock()
			return key, nil
		}
	}
	clerkJWKSCacheMu.RUnlock()

	// Determine Clerk JWKS URL
	jwksURL := os.Getenv("CLERK_JWKS_URL")
	if jwksURL == "" {
		frontendAPI := os.Getenv("CLERK_FRONTEND_API")
		if frontendAPI == "" && tenant.ClerkPublishableKey != "" {
			// Extract domain from publishable key if format pk_test_<base64>
			parts := strings.Split(tenant.ClerkPublishableKey, "_")
			if len(parts) >= 3 {
				decoded, _ := base64.RawStdEncoding.DecodeString(parts[2])
				if len(decoded) > 0 {
					frontendAPI = strings.TrimSuffix(string(decoded), "$")
				}
			}
		}
		if frontendAPI != "" {
			if !strings.HasPrefix(frontendAPI, "http") {
				frontendAPI = "https://" + frontendAPI
			}
			jwksURL = fmt.Sprintf("%s/.well-known/jwks.json", strings.TrimRight(frontendAPI, "/"))
		}
	}

	if jwksURL == "" {
		return nil, errors.New("clerk JWKS URL not configured")
	}

	resp, err := clerkHTTPClient.Get(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Clerk JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("clerk JWKS HTTP error %d: %s", resp.StatusCode, string(body))
	}

	var jwks ClerkJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, err
	}

	clerkJWKSCacheMu.Lock()
	defer clerkJWKSCacheMu.Unlock()

	for _, k := range jwks.Keys {
		if pubKey, err := parseRSAPublicKey(k.N, k.E); err == nil {
			clerkJWKSCache[k.Kid] = pubKey
		}
	}
	clerkJWKSCacheExp = time.Now().Add(1 * time.Hour)

	if key, exists := clerkJWKSCache[kid]; exists {
		return key, nil
	}

	return nil, fmt.Errorf("key with kid '%s' not found in Clerk JWKS", kid)
}

// validateClerkToken verifies an incoming Clerk session JWT and bridges to Booka User
func validateClerkToken(tokStr string, tenantSlug string) (*User, error) {
	// Fast parse unverified to inspect header kid
	token, _, err := new(jwt.Parser).ParseUnverified(tokStr, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	kid, _ := token.Header["kid"].(string)

	var targetTenant Tenant
	if tenantSlug != "" {
		_ = db.Where("slug = ?", tenantSlug).First(&targetTenant).Error
	}

	// Verify using Clerk RSA public key
	validatedTok, err := jwt.Parse(tokStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method for Clerk token: %v", t.Header["alg"])
		}
		// If PEM public key is configured directly in env
		if pemKey := os.Getenv("CLERK_PEM_PUBLIC_KEY"); pemKey != "" {
			return jwt.ParseRSAPublicKeyFromPEM([]byte(pemKey))
		}
		return fetchClerkPublicKey(kid, targetTenant)
	})

	if err != nil || !validatedTok.Valid {
		return nil, fmt.Errorf("clerk token verification failed: %w", err)
	}

	claims, ok := validatedTok.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid clerk claims")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, errors.New("missing sub claim in clerk token")
	}

	email, _ := claims["email"].(string)
	if email == "" {
		if primaryEmail, ok := claims["primary_email"].(string); ok {
			email = primaryEmail
		}
	}

	// Match or link to Booka User
	var user User
	// 1. Try matching by clerk_user_id
	if err := db.Where("clerk_user_id = ?", sub).First(&user).Error; err == nil {
		return &user, nil
	}

	// 2. Try matching by email
	if email != "" {
		cleanEmail := strings.ToLower(strings.TrimSpace(email))
		if err := db.Where("email = ?", cleanEmail).First(&user).Error; err == nil {
			// Link clerk_user_id for future fast lookups
			user.ClerkUserID = sub
			db.Model(&user).Update("clerk_user_id", sub)
			return &user, nil
		}

		// 3. Auto-provision user under tenant if tenant is found
		if targetTenant.ID > 0 {
			newUser := User{
				TenantID:     targetTenant.ID,
				Email:        cleanEmail,
				Role:         RoleStaff,
				Status:       "active",
				TokenVersion: 1,
				ClerkUserID:  sub,
			}
			if cErr := db.Create(&newUser).Error; cErr == nil {
				log.Printf("[CLERK SSO] Auto-provisioned user '%s' under tenant '%s' via Clerk", cleanEmail, targetTenant.Slug)
				return &newUser, nil
			}
		}
	}

	return nil, fmt.Errorf("user for clerk subject '%s' could not be resolved or provisioned", sub)
}

// clerkSyncHandler exchanges an authenticated Clerk session for Booka session and tenant user info
func clerkSyncHandler(c *fiber.Ctx) error {
	tenantSlug := strings.TrimSpace(c.Params("tenant"))
	if tenantSlug == "" {
		tenantSlug = "default"
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Token) == "" {
		// check Authorization header if token not in body
		auth := c.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			req.Token = strings.TrimSpace(auth[7:])
		}
	}

	if req.Token == "" {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "missing clerk session token")
	}

	user, err := validateClerkToken(req.Token, tenantSlug)
	if err != nil {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, fmt.Sprintf("clerk token verification failed: %v", err))
	}

	// Generate Booka native JWT token
	bookaToken, err := createTenantJWTToken(user.ID, user.Email, user.TenantID, user.Role, user.TokenVersion)
	if err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "failed to issue session token")
	}

	var tenant Tenant
	_ = db.First(&tenant, user.TenantID).Error

	return c.JSON(fiber.Map{
		"token": bookaToken,
		"user": fiber.Map{
			"id":                   user.ID,
			"email":                user.Email,
			"role":                 user.Role,
			"status":               user.Status,
			"must_change_password": user.MustChangePassword,
			"tenant_id":            user.TenantID,
			"clerk_user_id":        user.ClerkUserID,
		},
		"tenant": fiber.Map{
			"id":   tenant.ID,
			"slug": tenant.Slug,
			"name": tenant.Name,
		},
	})
}

