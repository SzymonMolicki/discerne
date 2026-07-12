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

	if cfg.DistractorWeights.Base != 10 {
		t.Fatalf("DistractorWeights.Base = %d, want %d", cfg.DistractorWeights.Base, 10)
	}

	if cfg.DistractorWeights.SameSubgroup != 16 {
		t.Fatalf("DistractorWeights.SameSubgroup = %d, want %d", cfg.DistractorWeights.SameSubgroup, 16)
	}
}

func TestLoadAllowsOverrides(t *testing.T) {
	cfg, err := Load([]string{
		"APP_NAME=Local Discerne",
		"APP_TIMEZONE=UTC",
		"HTTP_ADDRESS=:9090",
		"DISTRACTOR_BASE_WEIGHT=12",
		"DISTRACTOR_SAME_SCRIPT_WEIGHT=7",
	})
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

	if cfg.DistractorWeights.Base != 12 {
		t.Fatalf("DistractorWeights.Base = %d, want %d", cfg.DistractorWeights.Base, 12)
	}

	if cfg.DistractorWeights.SameScript != 7 {
		t.Fatalf("DistractorWeights.SameScript = %d, want %d", cfg.DistractorWeights.SameScript, 7)
	}
}

func TestLoadRejectsInvalidTimezone(t *testing.T) {
	_, err := Load([]string{"APP_TIMEZONE=not-a-zone"})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsInvalidDistractorWeights(t *testing.T) {
	_, err := Load([]string{"DISTRACTOR_BASE_WEIGHT=0"})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}

	_, err = Load([]string{"DISTRACTOR_SAME_GROUP_WEIGHT=-1"})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}

	_, err = Load([]string{"DISTRACTOR_SAME_SCRIPT_WEIGHT=not-a-number"})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
