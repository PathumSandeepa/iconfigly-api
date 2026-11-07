package config

import (
	"strings"
	"testing"
)

const validMasterKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func clearOptionalEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"PORT",
		"ICONFIGLY_ENV",
		"ICONFIGLY_SESSION_COOKIE",
		"ICONFIGLY_SESSION_TTL_HOURS",
		"ICONFIGLY_MASTER_KEY",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "")

	_, err := Load()

	if err == nil {
		t.Fatal("expected error when DATABASE_URL is missing")
	}
}

func TestLoadUsesDefaultPort(t *testing.T) {
	clearOptionalEnv(t)
	t.Setenv("DATABASE_URL", "postgres://example")

	cfg, err := Load()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != "8080" {
		t.Fatalf("expected port 8080, got %s", cfg.Port)
	}
}

func TestLoadUsesDefaults(t *testing.T) {
	clearOptionalEnv(t)
	t.Setenv("DATABASE_URL", "postgres://example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Env != "development" {
		t.Fatalf("expected env development, got %s", cfg.Env)
	}
	if cfg.SessionCookieName != "iconfigly_session" {
		t.Fatalf("expected default cookie name, got %s", cfg.SessionCookieName)
	}
	if cfg.SessionTTLHours != 24 {
		t.Fatalf("expected default session TTL 24, got %d", cfg.SessionTTLHours)
	}
	if len(cfg.MasterKeyHex) != 64 {
		t.Fatalf("expected a 64-character development master key, got %d", len(cfg.MasterKeyHex))
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	clearOptionalEnv(t)
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("PORT", "9090")
	t.Setenv("ICONFIGLY_ENV", "production")
	t.Setenv("ICONFIGLY_SESSION_COOKIE", "custom_session")
	t.Setenv("ICONFIGLY_SESSION_TTL_HOURS", "48")
	t.Setenv("ICONFIGLY_MASTER_KEY", validMasterKey)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != "9090" || cfg.Env != "production" || cfg.SessionCookieName != "custom_session" ||
		cfg.SessionTTLHours != 48 || cfg.MasterKeyHex != validMasterKey {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadDevelopmentMasterKeyIsDeterministic(t *testing.T) {
	clearOptionalEnv(t)
	t.Setenv("DATABASE_URL", "postgres://example")

	first, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	second, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if first.MasterKeyHex != second.MasterKeyHex {
		t.Fatal("expected the development master key to be stable between loads")
	}
}

func TestLoadRequiresMasterKeyOutsideDevelopment(t *testing.T) {
	clearOptionalEnv(t)
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ICONFIGLY_ENV", "production")

	_, err := Load()

	if err == nil || !strings.Contains(err.Error(), "ICONFIGLY_MASTER_KEY") {
		t.Fatalf("expected a master key error, got %v", err)
	}
}

func TestLoadRejectsInvalidMasterKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"not hex", strings.Repeat("z", 64)},
		{"too short", "abcd"},
		{"too long", validMasterKey + "00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearOptionalEnv(t)
			t.Setenv("DATABASE_URL", "postgres://example")
			t.Setenv("ICONFIGLY_MASTER_KEY", tt.key)

			if _, err := Load(); err == nil {
				t.Fatal("expected an error for an invalid master key")
			}
		})
	}
}

func TestLoadRejectsInvalidSessionTTL(t *testing.T) {
	for _, value := range []string{"abc", "0", "-5", "1.5"} {
		t.Run(value, func(t *testing.T) {
			clearOptionalEnv(t)
			t.Setenv("DATABASE_URL", "postgres://example")
			t.Setenv("ICONFIGLY_SESSION_TTL_HOURS", value)

			if _, err := Load(); err == nil {
				t.Fatalf("expected an error for TTL %q", value)
			}
		})
	}
}
