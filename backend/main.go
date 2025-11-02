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

	port := getEnv("PORT", "8080")
	dbPath := getEnv("SQLITE_PATH", "appointments.db")
	initDatabase(dbPath)

	// Ensure default admin exists
	_ = ensureDefaultAdmin(getEnv("DEFAULT_ADMIN_EMAIL", "admin@example.com"), getEnv("DEFAULT_ADMIN_PASSWORD", "admin123"))

	app := fiber.New()

	// ✅ CORS Configuration (dynamic)
	frontend := getEnv("FRONTEND_URL", "https://ai-chatbot-gamma-blue-98.vercel.app")
	log.Printf("[config] Allowing frontend origin: %s", frontend)

	app.Use(cors.New(cors.Config{
		AllowOrigins: frontend + ", http://localhost:3000",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	// ✅ Routes
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "time": time.Now()})
	})
	app.Post("/chat", chatHandler)
	app.Post("/register", registerHandler)
	app.Post("/login", loginHandler)

	admin := app.Group("/admin", jwtMiddleware)
	admin.Get("/appointments", listAppointments)
	admin.Post("/appointments", createAppointment)
	admin.Put("/appointments/:id", updateAppointment)
	admin.Delete("/appointments/:id", deleteAppointment)

	// ✅ Graceful shutdown handling
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
