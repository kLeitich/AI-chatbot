package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// loadEnvFile loads environment variables from .env file if it exists
func loadEnvFile() {
	file, err := os.Open(".env")
	if err != nil {
		return // .env file doesn't exist, skip
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if len(value) > 0 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			if key != "" {
				os.Setenv(key, value)
			}
		}
	}
}

// isAllowedCORSOrigin checks whether an incoming request Origin is permitted to access the API.
func isAllowedCORSOrigin(origin string, configuredOrigins []string) bool {
	origin = strings.ToLower(strings.TrimSpace(origin))
	if origin == "" {
		return false
	}

	// 1. Check configured origins from FRONTEND_URL and ALLOWED_ORIGINS
	for _, allowed := range configuredOrigins {
		clean := strings.ToLower(strings.TrimSpace(allowed))
		if clean == "" {
			continue
		}
		if origin == clean {
			return true
		}
		if strings.HasPrefix(clean, "*.") && strings.HasSuffix(origin, clean[1:]) {
			return true
		}
	}

	// 2. Localhost development origins
	if strings.HasPrefix(origin, "http://localhost:") || origin == "http://localhost" ||
		strings.HasPrefix(origin, "http://127.0.0.1:") || origin == "http://127.0.0.1" {
		return true
	}

	// 3. Official domain and subdomains
	if origin == "https://www.booka.co.ke" || origin == "https://booka.co.ke" ||
		strings.HasSuffix(origin, ".booka.co.ke") {
		return true
	}

	// 4. Official Booka project Vercel deployments (restrict to booka/ai-chatbot prefixes to block arbitrary *.vercel.app attacker tenants)
	if strings.HasPrefix(origin, "https://") && strings.HasSuffix(origin, ".vercel.app") {
		subdomain := strings.TrimPrefix(origin, "https://")
		subdomain = strings.TrimSuffix(subdomain, ".vercel.app")
		if strings.HasPrefix(subdomain, "ai-chatbot") || strings.HasPrefix(subdomain, "booka") {
			return true
		}
	}

	// 5. Official Booka Render web service self-origin
	if origin == "https://ai-chatbot-1vkx.onrender.com" {
		return true
	}

	return false
}

func main() {
	loadEnvFile()

	// CLI subcommand for creating platform admins securely
	if len(os.Args) > 1 && os.Args[1] == "create-platform-admin" {
		dbPath := getEnv("SQLITE_PATH", "appointments.db")
		_ = initDatabase(dbPath, "cli")
		runCreatePlatformAdminCLI(os.Args[2:])
		return
	}

	rawAppEnv := os.Getenv("APP_ENV")
	appEnv, envErr := normalizeAndValidateAppEnv(rawAppEnv)
	if envErr != nil {
		log.Fatalf("[FATAL CONFIG] %v", envErr)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if err := validateConfig(rawAppEnv, jwtSecret); err != nil {
		log.Fatalf("[FATAL CONFIG] %v", err)
	}

	port := getEnv("PORT", "8080")
	dbPath := getEnv("SQLITE_PATH", "appointments.db")
	defaultTenantID := initDatabase(dbPath, appEnv)

	if strings.EqualFold(appEnv, "development") {
		// Seed default tenant admin only in development
		defaultAdminEmail := getEnv("DEFAULT_ADMIN_EMAIL", "admin@example.com")
		defaultAdminPass := getEnv("DEFAULT_ADMIN_PASSWORD", "admin123")
		_ = ensureDefaultAdmin(defaultTenantID, defaultAdminEmail, defaultAdminPass)
	}

	// Bootstrap platform admin if zero exist and credentials provided
	bootstrapPlatformAdmin(appEnv)

	fiberConfig := fiber.Config{
		ErrorHandler: customErrorHandler,
	}
	if strings.EqualFold(getEnv("TRUST_PROXY", "false"), "true") {
		fiberConfig.ProxyHeader = "X-Forwarded-For"
		log.Println("[config] TRUST_PROXY=true: enabled X-Forwarded-For proxy header tracking")
	}

	app := fiber.New(fiberConfig)

	// Rate limiters to prevent brute-force attacks and LLM quota exhaustion
	authLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "Too many requests. Please wait a minute.")
		},
	})

	platformLoginLimiter := limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "Too many login attempts. Please wait a minute.")
		},
	})

	chatLimiter := limiter.New(limiter.Config{
		Max:        30,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "Chat rate limit reached. Please wait a moment before sending more messages.")
		},
	})

	publicConfigLimiter := limiter.New(limiter.Config{
		Max:        60,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "Too many requests. Please wait a minute.")
		},
	})

	passwordResetLimiter := limiter.New(limiter.Config{
		Max:        5,
		Expiration: 15 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "Too many password reset requests. Please wait 15 minutes.")
		},
	})

	// CORS Configuration: support comma-separated origins, localhost, *.booka.co.ke, and *.vercel.app
	rawFrontendURL := getEnv("FRONTEND_URL", "http://localhost:3000")
	rawAllowedOrigins := getEnv("ALLOWED_ORIGINS", "")
	var configuredOrigins []string
	for _, item := range strings.Split(rawFrontendURL+","+rawAllowedOrigins, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			configuredOrigins = append(configuredOrigins, trimmed)
		}
	}

	log.Printf("[config] Configured frontend origins: %v (also allowing localhost, *.booka.co.ke, *.vercel.app)", configuredOrigins)

	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			return isAllowedCORSOrigin(origin, configuredOrigins)
		},
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Requested-With",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS,HEAD",
		AllowCredentials: true,
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "time": time.Now()})
	})

	// Global auth endpoints (rate limited)
	app.Post("/auth/login", authLimiter, globalLoginHandler)
	app.Post("/login", authLimiter, globalLoginHandler)
	app.Post("/auth/register", authLimiter, companyRegisterHandler)
	app.Post("/auth/platform/login", platformLoginLimiter, platformLoginHandler)
	app.Post("/platform/login", platformLoginLimiter, platformLoginHandler)
	app.Post("/auth/change-password", jwtMiddleware, changePasswordHandler)
	app.Post("/auth/forgot-password", passwordResetLimiter, forgotPasswordHandler)
	app.Post("/auth/reset-password", passwordResetLimiter, resetPasswordHandler)

	// Platform admin endpoints (strictly read-only views)
	platform := app.Group("/platform", jwtMiddleware, platformAdminMiddleware)
	platform.Get("/tenants", listPlatformTenants)
	platform.Get("/companies", listPlatformTenants)
	platform.Get("/users", listPlatformUsers)
	platform.Get("/doctors", listPlatformDoctors)
	platform.Get("/appointments", listPlatformAppointments)
	platform.Get("/audit", listPlatformAuditLogs)

	// Tenant-scoped API under /t/:tenant/...
	tenant := app.Group("/t/:tenant")
	tenant.Get("/public-config", publicConfigLimiter, getTenantPublicConfigHandler)
	tenant.Post("/chat", chatLimiter, chatHandler)
	tenant.Post("/login", authLimiter, loginHandler)
	tenant.Post("/change-password", jwtMiddleware, changePasswordHandler)
	tenant.Get("/me", jwtMiddleware, tenantUserMiddleware, getTenantMeHandler)
	tenant.Get("/doctors", listDoctors)
	tenant.Get("/reasons", listReasons)
	tenant.Get("/availability", getAvailability)

	// Admin and staff endpoints with granular permissions
	admin := tenant.Group("/admin", jwtMiddleware, tenantUserMiddleware)
	admin.Get("/me", getTenantMeHandler)
	admin.Get("/appointments", requirePermission(PermAppointmentsView), listAppointments)
	admin.Post("/appointments", requirePermission(PermAppointmentsCreate), createAppointment)
	admin.Put("/appointments/:id", updateAppointment)
	admin.Delete("/appointments/:id", requirePermission(PermAppointmentsDelete), deleteAppointment)
	admin.Get("/doctors", requirePermission(PermDoctorsView), listDoctors)
	admin.Post("/doctors", requirePermission(PermDoctorsManage), createDoctor)
	admin.Patch("/doctors/:id", requirePermission(PermDoctorsManage), patchDoctor)
	admin.Get("/reasons", listReasons)
	admin.Post("/reasons", requirePermission(PermReasonsManage), createReason)
	admin.Patch("/reasons/:id", requirePermission(PermReasonsManage), patchReason)
	admin.Get("/settings", requirePermission(PermSettingsManage), getTenantSettingsHandler)
	admin.Patch("/settings", requirePermission(PermSettingsManage), updateTenantSettingsHandler)

	// Tenant admin user management endpoints (tenant_admin role strictly required)
	admin.Get("/permissions", tenantAdminMiddleware, listPermissionsHandler)
	admin.Get("/users", tenantAdminMiddleware, listTenantUsersHandler)
	admin.Post("/users", tenantAdminMiddleware, createTenantUserHandler)
	admin.Patch("/users/:id/permissions", tenantAdminMiddleware, updateUserPermissionsHandler)
	admin.Patch("/users/:id/status", tenantAdminMiddleware, updateUserStatusHandler)
	admin.Post("/users/:id/reset-password", tenantAdminMiddleware, resetUserPasswordHandler)
	admin.Get("/audit", tenantAdminMiddleware, listTenantAuditLogs)

	// Graceful shutdown handling
	go func() {
		log.Printf("[startup] Listening on :%s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("[fatal] server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[shutdown] Gracefully shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("[error] Shutdown failed: %v", err)
	}
	log.Println("[shutdown] Server stopped.")
}

// getEnv returns env variable or fallback
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
