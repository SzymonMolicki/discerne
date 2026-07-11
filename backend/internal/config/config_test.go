package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddress != ":8080" {
		t.Fatalf("HTTPAddress = %q, want %q", cfg.HTTPAddress, ":8080")
	}

	if cfg.AppName != "Discerne" {
		t.Fatalf("AppName = %q, want %q", cfg.AppName, "Discerne")
	}

	if cfg.AppTimezone.String() != "Europe/Warsaw" {
		t.Fatalf("AppTimezone = %q, want %q", cfg.AppTimezone.String(), "Europe/Warsaw")
	}
}

func TestLoadAllowsOverrides(t *testing.T) {
	cfg, err := Load([]string{"APP_NAME=Local Discerne", "APP_TIMEZONE=UTC", "HTTP_ADDRESS=:9090"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppName != "Local Discerne" {
		t.Fatalf("AppName = %q, want %q", cfg.AppName, "Local Discerne")
	}

	if cfg.HTTPAddress != ":9090" {
		t.Fatalf("HTTPAddress = %q, want %q", cfg.HTTPAddress, ":9090")
	}

	if cfg.AppTimezone.String() != "UTC" {
		t.Fatalf("AppTimezone = %q, want %q", cfg.AppTimezone.String(), "UTC")
	}
}

func TestLoadRejectsInvalidTimezone(t *testing.T) {
	_, err := Load([]string{"APP_TIMEZONE=not-a-zone"})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
