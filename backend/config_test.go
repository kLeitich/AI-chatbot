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

