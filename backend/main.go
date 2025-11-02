package main

import (
	"bufio"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// loadEnvFile loads environment variables from .env file if it exists
func loadEnvFile() {
	file, err := os.Open(".env")
	if err != nil {
		return // .env file doesn't exist, that's okay
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // skip empty lines and comments
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
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
	// Load .env file if it exists
	loadEnvFile()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "appointments.db"
	}
	initDatabase(dbPath)

	// Ensure default admin
	_ = ensureDefaultAdmin(getEnv("DEFAULT_ADMIN_EMAIL", "admin@example.com"), getEnv("DEFAULT_ADMIN_PASSWORD", "admin123"))

	app := fiber.New()
	app.Use(cors.New(cors.Config{AllowOrigins: "*", AllowHeaders: "*", AllowMethods: "GET,POST,PUT,DELETE,OPTIONS"}))

	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Post("/chat", chatHandler)
	app.Post("/register", registerHandler)
	app.Post("/login", loginHandler)

	admin := app.Group("/admin", jwtMiddleware)
	admin.Get("/appointments", listAppointments)
	admin.Post("/appointments", createAppointment)
	admin.Put("/appointments/:id", updateAppointment)
	admin.Delete("/appointments/:id", deleteAppointment)

	log.Printf("listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
