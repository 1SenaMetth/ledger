package config

import (
	"strings"
	"testing"
)

func TestLoadReportsAllMissingVars(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded with no DATABASE_URL or JWT_SECRET")
	}
	for _, want := range []string{"DATABASE_URL", "JWT_SECRET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %s, got: %v", want, err)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/ledger")
	t.Setenv("JWT_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.HTTPPort != "8080" {
		t.Errorf("HTTPPort = %q, want 8080", cfg.HTTPPort)
	}
	if cfg.Database.MaxConns != 10 {
		t.Errorf("MaxConns = %d, want 10", cfg.Database.MaxConns)
	}
}

func TestLoadRejectsBadDuration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/ledger")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("JWT_ACCESS_TTL", "fifteen minutes")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an invalid duration")
	}
}

func TestProductionRequiresStrongSecret(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost/ledger")
	t.Setenv("JWT_SECRET", "short")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted a weak JWT_SECRET in production")
	}
}
