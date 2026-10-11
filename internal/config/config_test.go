package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "")

	_, err := Load()

	if err == nil {
		t.Fatal("expected error when DATABASE_URL is missing")
	}
}

func TestLoadUsesDefaultPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")

	cfg, err := Load()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != "8080" {
		t.Fatalf("expected port 8080, got %s", cfg.Port)
	}
}