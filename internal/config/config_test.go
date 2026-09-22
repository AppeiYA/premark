package config_test

import (
	"os"
	"testing"
	"time"

	"premark/internal/config"
)

func TestConfig_Load(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Port != "8080" {
			t.Errorf("expected default Port=8080, got %s", cfg.Port)
		}
		if cfg.Storage != "sqlite" {
			t.Errorf("expected default Storage=sqlite, got %s", cfg.Storage)
		}
		if cfg.DBPath != "premark.db" {
			t.Errorf("expected default DBPath=premark.db, got %s", cfg.DBPath)
		}
		if cfg.JupiterImpactIsFraction != true {
			t.Errorf("expected JupiterImpactIsFraction=true")
		}
		if cfg.ScanInterval != 60*time.Second {
			t.Errorf("expected ScanInterval=60s, got %v", cfg.ScanInterval)
		}
	})

	t.Run("environment overrides", func(t *testing.T) {
		os.Setenv("PORT", "9090")
		os.Setenv("STORAGE", "memory")
		os.Setenv("JUPITER_IMPACT_IS_FRACTION", "false")
		os.Setenv("SCAN_INTERVAL", "15s")
		defer func() {
			os.Unsetenv("PORT")
			os.Unsetenv("STORAGE")
			os.Unsetenv("JUPITER_IMPACT_IS_FRACTION")
			os.Unsetenv("SCAN_INTERVAL")
		}()

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Port != "9090" {
			t.Errorf("expected Port=9090, got %s", cfg.Port)
		}
		if cfg.Storage != "memory" {
			t.Errorf("expected Storage=memory, got %s", cfg.Storage)
		}
		if cfg.JupiterImpactIsFraction != false {
			t.Errorf("expected JupiterImpactIsFraction=false")
		}
		if cfg.ScanInterval != 15*time.Second {
			t.Errorf("expected ScanInterval=15s, got %v", cfg.ScanInterval)
		}
	})

	t.Run("invalid port returns error", func(t *testing.T) {
		os.Setenv("PORT", "invalid")
		defer os.Unsetenv("PORT")

		_, err := config.Load()
		if err == nil {
			t.Fatalf("expected error for invalid port, got nil")
		}
	})

	t.Run("invalid storage returns error", func(t *testing.T) {
		os.Setenv("STORAGE", "postgres")
		defer os.Unsetenv("STORAGE")

		_, err := config.Load()
		if err == nil {
			t.Fatalf("expected error for invalid storage, got nil")
		}
	})

	t.Run("scan interval below 5s returns error", func(t *testing.T) {
		os.Setenv("SCAN_INTERVAL", "2s")
		defer os.Unsetenv("SCAN_INTERVAL")

		_, err := config.Load()
		if err == nil {
			t.Fatalf("expected error for scan interval < 5s, got nil")
		}
	})
}
