package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
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
