package main

import (
	"strings"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		appEnv    string
		jwtSecret string
		wantErr   bool
		errSubstr string
	}{
		// Development environment allows empty or short secrets for local testing convenience
		{
			name:      "Dev allows empty secret",
			appEnv:    "development",
			jwtSecret: "",
			wantErr:   false,
		},
		{
			name:      "Dev allows short secret",
			appEnv:    "development",
			jwtSecret: "short",
			wantErr:   false,
		},
		{
			name:      "Dev allows default secret",
			appEnv:    "development",
			jwtSecret: "supersecret",
			wantErr:   false,
		},

		// Unset APP_ENV fails closed to production
		{
			name:      "Unset APP_ENV fails closed and rejects empty secret",
			appEnv:    "",
			jwtSecret: "",
			wantErr:   true,
			errSubstr: "must not be empty",
		},
		{
			name:      "Unset APP_ENV fails closed and rejects weak secret",
			appEnv:    "",
			jwtSecret: "weak-secret",
			wantErr:   true,
			errSubstr: "at least 32 characters",
		},
		{
			name:      "Unset APP_ENV fails closed and accepts valid 32+ char secret",
			appEnv:    "",
			jwtSecret: "v3ry_str0ng_pr0duct10n_jwt_s3cr3t_k3y_999!",
			wantErr:   false,
		},

		// Invalid APP_ENV is rejected
		{
			name:      "Invalid APP_ENV is rejected",
			appEnv:    "invalid_env",
			jwtSecret: "v3ry_str0ng_pr0duct10n_jwt_s3cr3t_k3y_999!",
			wantErr:   true,
			errSubstr: "invalid APP_ENV",
		},
		{
			name:      "Arbitrary APP_ENV like 'test' is rejected",
			appEnv:    "test",
			jwtSecret: "v3ry_str0ng_pr0duct10n_jwt_s3cr3t_k3y_999!",
			wantErr:   true,
			errSubstr: "invalid APP_ENV",
		},

		// Production environment strictly enforces non-empty, >=32 length, non-default
		{
			name:      "Prod rejects empty secret",
			appEnv:    "production",
			jwtSecret: "",
			wantErr:   true,
			errSubstr: "must not be empty",
		},
		{
			name:      "Prod rejects whitespace-only secret",
			appEnv:    "production",
			jwtSecret: "    ",
			wantErr:   true,
			errSubstr: "must not be empty",
		},
		{
			name:      "Prod rejects secret under 32 chars",
			appEnv:    "production",
			jwtSecret: "too-short-jwt-secret-under-32",
			wantErr:   true,
			errSubstr: "at least 32 characters",
		},
		{
			name:      "Prod rejects known default supersecret",
			appEnv:    "production",
			jwtSecret: "supersecret",
			wantErr:   true,
			errSubstr: "at least 32 characters",
		},
		{
			name:      "Prod rejects known 32-char default numeric secret",
			appEnv:    "production",
			jwtSecret: "12345678901234567890123456789012",
			wantErr:   true,
			errSubstr: "insecure known default",
		},
		{
			name:      "Prod rejects compose dev default key",
			appEnv:    "production",
			jwtSecret: "your-development-only-jwt-secret-key-32-chars!",
			wantErr:   true,
			errSubstr: "insecure known default",
		},
		{
			name:      "Prod rejects known dev default key",
			appEnv:    "production",
			jwtSecret: "dev_insecure_jwt_secret_change_in_production",
			wantErr:   true,
			errSubstr: "insecure known default",
		},
		{
			name:      "Staging rejects weak secret",
			appEnv:    "staging",
			jwtSecret: "weak-staging-secret",
			wantErr:   true,
			errSubstr: "at least 32 characters",
		},
		{
			name:      "Prod accepts strong 32+ char secret",
			appEnv:    "production",
			jwtSecret: "v3ry_str0ng_pr0duct10n_jwt_s3cr3t_k3y_999!",
			wantErr:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfig(tc.appEnv, tc.jwtSecret)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error for env=%s, secret=%q, but got nil", tc.appEnv, tc.jwtSecret)
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("Expected error containing %q, got: %v", tc.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error for env=%s, secret=%q, got: %v", tc.appEnv, tc.jwtSecret, err)
				}
			}
		})
	}
}

func TestIsAllowedCORSOrigin(t *testing.T) {
	configured := []string{
		"https://custom-clinic.com",
		"*.partner-health.org",
	}

	tests := []struct {
		origin  string
		allowed bool
	}{
		// Vercel deployment origins
		{"https://ai-chatbot-gamma-blue-98.vercel.app", true},
		{"https://ai-chatbot-preview-123.vercel.app", true},
		{"https://booka-front.vercel.app", true},

		// Booka official domains
		{"https://www.booka.co.ke", true},
		{"https://booka.co.ke", true},
		{"https://admin.booka.co.ke", true},

		// Localhost development
		{"http://localhost:3000", true},
		{"http://localhost:8080", true},
		{"http://127.0.0.1:3000", true},

		// Configured origins
		{"https://custom-clinic.com", true},
		{"https://sub.partner-health.org", true},

		// Render service origins
		{"https://ai-chatbot-1vkx.onrender.com", true},

		// Disallowed origins
		{"https://malicious-site.com", false},
		{"https://evil-vercel.app.attacker.com", false},
		{"http://vercel.app", false},
		{"", false},
	}

	for _, tc := range tests {
		got := isAllowedCORSOrigin(tc.origin, configured)
		if got != tc.allowed {
			t.Errorf("isAllowedCORSOrigin(%q) = %v, want %v", tc.origin, got, tc.allowed)
		}
	}
}
