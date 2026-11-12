package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var jwtSecretKey []byte

var knownDefaultSecrets = map[string]bool{
	"supersecret":                                           true,
	"secret":                                                true,
	"changeme":                                              true,
	"default":                                               true,
	"password":                                              true,
	"admin123":                                              true,
	"platform123":                                           true,
	"12345678901234567890123456789012":                     true,
	"dev_insecure_jwt_secret_change_in_production":          true,
	"your-development-only-jwt-secret-key-32-chars!":        true,
	"your-32-plus-character-secret-key-goes-here-min-32":    true,
	"your-production-jwt-secret-key-min-32-chars-long!":     true,
	"dev_local_jwt_secret_min_32_characters_for_testing_only!": true,
}

// normalizeAndValidateAppEnv validates APP_ENV and returns the normalized string.
// If unset, it fails closed by defaulting to "production".
// Accepted values are "development", "staging", "production" (case-insensitive).
// Any other value returns an error.
func normalizeAndValidateAppEnv(rawEnv string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(rawEnv))
	if trimmed == "" {
		return "production", nil
	}
	switch trimmed {
	case "development", "staging", "production":
		return trimmed, nil
	default:
		return "", fmt.Errorf("invalid APP_ENV '%s'. Accepted values are: development, staging, production", rawEnv)
	}
}

// validateConfig validates application configuration based on environment.
// Fails closed: unset APP_ENV is treated as production.
func validateConfig(rawAppEnv, jwtSecret string) error {
	appEnv, err := normalizeAndValidateAppEnv(rawAppEnv)
	if err != nil {
		return err
	}

	if appEnv == "development" {
		if strings.TrimSpace(jwtSecret) == "" {
			log.Println("[CONFIG] APP_ENV=development: JWT_SECRET unset, using local development fallback key")
			jwtSecretKey = []byte("dev_local_jwt_secret_min_32_characters_for_testing_only!")
			return nil
		}
		jwtSecretKey = []byte(jwtSecret)
		return nil
	}

	// Outside development (staging, production)
	if strings.TrimSpace(jwtSecret) == "" {
		return fmt.Errorf("JWT_SECRET must not be empty in %s environment", appEnv)
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
