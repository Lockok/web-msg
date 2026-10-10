package config

import (
	"strings"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	const testSecret = "0123456789abcdef0123456789abcdef"

	tests := []struct {
		name        string
		port        string
		databaseURL string
		jwtSecret   string
		wantPort    string
		wantError   string
	}{
		{
			name:        "valid configuration with default port",
			databaseURL: "postgres://localhost/messenger",
			jwtSecret:   testSecret,
			wantPort:    "8080",
		},
		{
			name:        "valid configuration with custom port",
			port:        "9000",
			databaseURL: "postgres://localhost/messenger",
			jwtSecret:   testSecret,
			wantPort:    "9000",
		},
		{
			name:      "missing database URL",
			jwtSecret: testSecret,
			wantError: "DATABASE_URL is required",
		},
		{
			name:        "blank database URL",
			databaseURL: "   ",
			jwtSecret:   testSecret,
			wantError:   "DATABASE_URL is required",
		},
		{
			name:        "missing JWT secret",
			databaseURL: "postgres://localhost/messenger",
			wantError:   "JWT_SECRET is required",
		},
		{
			name:        "blank JWT secret",
			databaseURL: "postgres://localhost/messenger",
			jwtSecret:   strings.Repeat(" ", 32),
			wantError:   "JWT_SECRET is required",
		},
		{
			name:        "short JWT secret",
			databaseURL: "postgres://localhost/messenger",
			jwtSecret:   strings.Repeat("a", 31),
			wantError:   "JWT_SECRET must contain at least 32 bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PORT", tt.port)
			t.Setenv("DATABASE_URL", tt.databaseURL)
			t.Setenv("JWT_SECRET", tt.jwtSecret)

			cfg, err := loadFromEnv()

			if tt.wantError != "" {
				if err == nil {
					t.Fatal("expected configuration error")
				}

				if err.Error() != tt.wantError {
					t.Fatalf("expected %q, got %q", tt.wantError, err.Error())
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.Port != tt.wantPort {
				t.Errorf("expected port %q, got %q", tt.wantPort, cfg.Port)
			}

			if cfg.DatabaseURL != tt.databaseURL {
				t.Error("unexpected database URL")
			}

			if cfg.JWTSecret != tt.jwtSecret {
				t.Error("unexpected JWT secret")
			}
		})
	}
}
