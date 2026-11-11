package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

var jwtSecretKey []byte

var knownDefaultSecrets = map[string]bool{
	"supersecret":                      true,
	"secret":                           true,
	"changeme":                         true,
	"default":                          true,
	"password":                         true,
	"admin123":                         true,
	"platform123":                      true,
	"12345678901234567890123456789012": true,
	"dev_insecure_jwt_secret_change_in_production": true,
}

// validateConfig validates application configuration based on environment
func validateConfig(appEnv, jwtSecret string) error {
	if strings.EqualFold(appEnv, "development") {
		if strings.TrimSpace(jwtSecret) == "" {
			log.Println("[CONFIG] APP_ENV=development: JWT_SECRET unset, using local development fallback key")
			jwtSecretKey = []byte("dev_local_jwt_secret_min_32_characters_for_testing_only!")
			return nil
		}
		jwtSecretKey = []byte(jwtSecret)
		return nil
	}

	if strings.TrimSpace(jwtSecret) == "" {
		return errors.New("JWT_SECRET must not be empty in non-development environments")
	}
	if len(jwtSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters long in %s environment (got %d)", appEnv, len(jwtSecret))
	}
	if knownDefaultSecrets[strings.ToLower(jwtSecret)] {
		return fmt.Errorf("JWT_SECRET is set to an insecure known default in %s environment", appEnv)
	}

	jwtSecretKey = []byte(jwtSecret)
	return nil
}

// getJWTSecret returns the validated JWT secret
func getJWTSecret() []byte {
	if len(jwtSecretKey) == 0 {
		sec := os.Getenv("JWT_SECRET")
		if sec != "" {
			return []byte(sec)
		}
	}
	return jwtSecretKey
}

