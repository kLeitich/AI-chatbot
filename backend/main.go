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

	fiberConfig := fiber.Config{}
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

	// CORS Configuration
	frontendURL := getEnv("FRONTEND_URL", "http://localhost:3000")
	allowedOrigins := "http://localhost:3000"
	if frontendURL != "http://localhost:3000" {
		allowedOrigins = frontendURL + ", http://localhost:3000"
	}

	log.Printf("[config] Allowing frontend origins: %s", allowedOrigins)

	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "time": time.Now()})
	})

	// Global auth endpoints (rate limited)
	app.Post("/auth/register", authLimiter, companyRegisterHandler)
	app.Post("/auth/platform/login", platformLoginLimiter, platformLoginHandler)
	app.Post("/platform/login", platformLoginLimiter, platformLoginHandler)
	app.Post("/auth/change-password", jwtMiddleware, changePasswordHandler)

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
	tenant.Post("/chat", chatLimiter, chatHandler)
	tenant.Post("/register", authLimiter, registerHandler)
	tenant.Post("/login", authLimiter, loginHandler)
	tenant.Post("/change-password", jwtMiddleware, changePasswordHandler)
	tenant.Get("/doctors", listDoctors)
	tenant.Get("/reasons", listReasons)
	tenant.Get("/availability", getAvailability)

	// Admin endpoints require tenant_admin role and matching tenant
	admin := tenant.Group("/admin", jwtMiddleware, tenantAdminMiddleware)
	admin.Get("/appointments", listAppointments)
	admin.Post("/appointments", createAppointment)
	admin.Put("/appointments/:id", updateAppointment)
	admin.Delete("/appointments/:id", deleteAppointment)
	admin.Post("/users", createUserHandler)
	admin.Post("/doctors", createDoctor)
	admin.Post("/reasons", createReason)

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
