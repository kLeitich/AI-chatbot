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

func createUniqueTenantSlug(baseName string) string {
	baseSlug := generateTenantSlug(baseName)
	if len(baseSlug) < 2 {
		baseSlug = "clinic"
	}
	reservedSlugs := map[string]bool{
		"platform": true, "admin": true, "auth": true, "api": true, "root": true,
		"system": true, "default": true, "login": true, "register": true, "settings": true,
		"dashboard": true, "health": true, "embed": true, "book": true, "signup": true,
	}

	candidate := baseSlug
	counter := 1
	for {
		if !reservedSlugs[candidate] {
			var count int64
			db.Model(&Tenant{}).Where("slug = ?", candidate).Count(&count)
			if count == 0 {
				return candidate
			}
		}
		candidate = fmt.Sprintf("%s-%d", baseSlug, counter)
		counter++
	}
}

// fetchClerkJWKS retrieves and caches RSA public keys from Clerk's JWKS endpoint
func fetchClerkPublicKey(kid string, tenant Tenant) (*rsa.PublicKey, error) {
	// Determine Clerk JWKS URL
	jwksURL := os.Getenv("CLERK_JWKS_URL")
	if jwksURL == "" {
		frontendAPI := os.Getenv("CLERK_FRONTEND_API")
		if frontendAPI == "" {
			pubKeyToUse := tenant.ClerkPublishableKey
			if pubKeyToUse == "" {
				pubKeyToUse = os.Getenv("CLERK_PUBLISHABLE_KEY")
			}
			if pubKeyToUse == "" {
				pubKeyToUse = os.Getenv("NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY")
			}
			if pubKeyToUse != "" {
				parts := strings.Split(pubKeyToUse, "_")
				if len(parts) >= 3 {
					decoded, err := base64.RawURLEncoding.DecodeString(parts[2])
					if err != nil {
						decoded, _ = base64.RawStdEncoding.DecodeString(parts[2])
					}
					if len(decoded) > 0 {
						frontendAPI = strings.TrimSuffix(string(decoded), "$")
					}
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

	cacheKey := fmt.Sprintf("%s:%s", jwksURL, kid)

	clerkJWKSCacheMu.RLock()
	if time.Now().Before(clerkJWKSCacheExp) {
		if key, exists := clerkJWKSCache[cacheKey]; exists {
			clerkJWKSCacheMu.RUnlock()
			return key, nil
		}
	}
	clerkJWKSCacheMu.RUnlock()

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
			cKey := fmt.Sprintf("%s:%s", jwksURL, k.Kid)
			clerkJWKSCache[cKey] = pubKey
		}
	}
	clerkJWKSCacheExp = time.Now().Add(1 * time.Hour)

	if key, exists := clerkJWKSCache[cacheKey]; exists {
		return key, nil
	}

	return nil, fmt.Errorf("key with kid '%s' not found in Clerk JWKS (%s)", kid, jwksURL)
}

// validateClerkToken verifies an incoming Clerk session JWT and bridges to Booka User
func validateClerkToken(tokStr string, tenantSlug string, companyName string) (*User, bool, error) {
	// Fast parse unverified to inspect header kid
	token, _, err := new(jwt.Parser).ParseUnverified(tokStr, jwt.MapClaims{})
	if err != nil {
		return nil, false, err
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
		return nil, false, fmt.Errorf("clerk token verification failed: %w", err)
	}

	claims, ok := validatedTok.Claims.(jwt.MapClaims)
	if !ok {
		return nil, false, errors.New("invalid clerk claims")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, false, errors.New("missing sub claim in clerk token")
	}

	email, _ := claims["email"].(string)
	if email == "" {
		if primaryEmail, ok := claims["primary_email"].(string); ok {
			email = primaryEmail
		}
	}

	fullName := ""
	if nameVal, ok := claims["name"].(string); ok && nameVal != "" {
		fullName = nameVal
	} else {
		fName, _ := claims["first_name"].(string)
		if fName == "" {
			fName, _ = claims["given_name"].(string)
		}
		lName, _ := claims["last_name"].(string)
		if lName == "" {
			lName, _ = claims["family_name"].(string)
		}
		fullName = strings.TrimSpace(fName + " " + lName)
	}

	// Multitenant User Resolution & Auto-Registration
	var user User

	if targetTenant.ID > 0 {
		// Scoped explicitly to this clinic practice (e.g. logging into /t/:tenant/login)
		if err := db.Where("clerk_user_id = ? AND tenant_id = ?", sub, targetTenant.ID).First(&user).Error; err == nil {
			return &user, false, nil
		}
		if email != "" {
			cleanEmail := strings.ToLower(strings.TrimSpace(email))
			if err := db.Where("email = ? AND tenant_id = ?", cleanEmail, targetTenant.ID).First(&user).Error; err == nil {
				user.ClerkUserID = sub
				db.Model(&user).Update("clerk_user_id", sub)
				return &user, false, nil
			}

			// Auto-provision user under target tenant as staff
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
				return &newUser, true, nil
			}
		}
	} else {
		// Global login/signup: check if token claims contain an organization or tenant slug
		if orgSlug, ok := claims["org_slug"].(string); ok && orgSlug != "" {
			_ = db.Where("slug = ?", orgSlug).First(&targetTenant).Error
		} else if tenantClaim, ok := claims["tenant_slug"].(string); ok && tenantClaim != "" {
			_ = db.Where("slug = ?", tenantClaim).First(&targetTenant).Error
		}

		if targetTenant.ID > 0 {
			cleanEmail := strings.ToLower(strings.TrimSpace(email))
			if err := db.Where("clerk_user_id = ? AND tenant_id = ?", sub, targetTenant.ID).First(&user).Error; err == nil {
				return &user, false, nil
			}
			if cleanEmail != "" {
				if err := db.Where("email = ? AND tenant_id = ?", cleanEmail, targetTenant.ID).First(&user).Error; err == nil {
					user.ClerkUserID = sub
					db.Model(&user).Update("clerk_user_id", sub)
					return &user, false, nil
				}
				newUser := User{
					TenantID:     targetTenant.ID,
					Email:        cleanEmail,
					Role:         RoleStaff,
					Status:       "active",
					TokenVersion: 1,
					ClerkUserID:  sub,
				}
				if cErr := db.Create(&newUser).Error; cErr == nil {
					return &newUser, true, nil
				}
			}
		} else {
			// 1. Match existing user by clerk_user_id across tenants
			if err := db.Where("clerk_user_id = ?", sub).First(&user).Error; err == nil {
				return &user, false, nil
			}
			// 2. Match existing user by email across tenants
			cleanEmail := strings.ToLower(strings.TrimSpace(email))
			if cleanEmail != "" {
				if err := db.Where("email = ?", cleanEmail).First(&user).Error; err == nil {
					user.ClerkUserID = sub
					db.Model(&user).Update("clerk_user_id", sub)
					return &user, false, nil
				}
			}

			// 3. User is signing up with Google on global /login or /signup: Auto-create their clinic practice!
			if cleanEmail == "" {
				return nil, false, errors.New("cannot create practice: missing email in Google/Clerk token")
			}

			practiceName := strings.TrimSpace(companyName)
			if practiceName == "" {
				if fullName != "" {
					practiceName = fmt.Sprintf("%s's Clinic", fullName)
				} else {
					parts := strings.Split(cleanEmail, "@")
					practiceName = fmt.Sprintf("%s Clinic", strings.Title(parts[0]))
				}
			}

			uniqueSlug := createUniqueTenantSlug(practiceName)
			newTenant := Tenant{
				Name:                practiceName,
				Slug:                uniqueSlug,
				BookingLinkEnabled:  true,
				AllowedEmbedOrigins: "[]",
			}
			if err := db.Create(&newTenant).Error; err != nil {
				return nil, false, fmt.Errorf("failed to create clinic tenant: %w", err)
			}

			// Seed starter doctors & consultation reasons so their practice chatbot works immediately
			seedSampleData(newTenant.ID)

			newUser := User{
				TenantID:     newTenant.ID,
				Email:        cleanEmail,
				Role:         RoleTenantAdmin, // Owner/Admin of their practice
				Status:       "active",
				TokenVersion: 1,
				ClerkUserID:  sub,
			}
			if err := db.Create(&newUser).Error; err != nil {
				return nil, false, fmt.Errorf("failed to create admin user: %w", err)
			}

			log.Printf("[CLERK SSO SIGNUP] Successfully registered new clinic practice '%s' (%s) for user '%s'", newTenant.Name, newTenant.Slug, cleanEmail)
			return &newUser, true, nil
		}
	}

	return nil, false, fmt.Errorf("user for clerk subject '%s' could not be resolved or provisioned", sub)
}

// clerkSyncHandler exchanges an authenticated Clerk session for Booka session and tenant user info
func clerkSyncHandler(c *fiber.Ctx) error {
	tenantSlug := strings.TrimSpace(c.Params("tenant"))

	var req struct {
		Token       string `json:"token"`
		CompanyName string `json:"company_name"`
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

	user, isNew, err := validateClerkToken(req.Token, tenantSlug, req.CompanyName)
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
		"token":          bookaToken,
		"is_new_account": isNew,
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

