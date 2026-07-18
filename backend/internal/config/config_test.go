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

	if cfg.DeviceCookieName != "discerne_device" {
		t.Fatalf("DeviceCookieName = %q, want %q", cfg.DeviceCookieName, "discerne_device")
	}

	if cfg.SecureCookies {
		t.Fatal("SecureCookies = true, want false")
	}

	if cfg.DistractorWeights.Base != 1 {
		t.Fatalf("DistractorWeights.Base = %d, want %d", cfg.DistractorWeights.Base, 1)
	}

	if cfg.DistractorWeights.SameSubgroup != 10 {
		t.Fatalf("DistractorWeights.SameSubgroup = %d, want %d", cfg.DistractorWeights.SameSubgroup, 10)
	}

	if cfg.DistractorWeights.SameContinent != 10 {
		t.Fatalf("DistractorWeights.SameContinent = %d, want %d", cfg.DistractorWeights.SameContinent, 10)
	}

	if cfg.DistractorWeights.SameScript != 10 {
		t.Fatalf("DistractorWeights.SameScript = %d, want %d", cfg.DistractorWeights.SameScript, 10)
	}

	if cfg.MutationRateLimit.Requests != 30 {
		t.Fatalf("MutationRateLimit.Requests = %d, want %d", cfg.MutationRateLimit.Requests, 30)
	}

	if cfg.MutationRateLimit.Window.String() != "1m0s" {
		t.Fatalf("MutationRateLimit.Window = %s, want %s", cfg.MutationRateLimit.Window, "1m0s")
	}
}

func TestLoadAllowsOverrides(t *testing.T) {
	cfg, err := Load([]string{
		"APP_NAME=Local Discerne",
		"APP_TIMEZONE=UTC",
		"HTTP_ADDRESS=:9090",
		"DEVICE_COOKIE_NAME=local_device",
		"SECURE_COOKIES=true",
		"DISTRACTOR_BASE_WEIGHT=12",
		"DISTRACTOR_SAME_SCRIPT_WEIGHT=7",
		"RATE_LIMIT_MUTATION_REQUESTS=9",
		"RATE_LIMIT_MUTATION_WINDOW=30s",
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

	if cfg.DeviceCookieName != "local_device" {
		t.Fatalf("DeviceCookieName = %q, want %q", cfg.DeviceCookieName, "local_device")
	}

	if !cfg.SecureCookies {
		t.Fatal("SecureCookies = false, want true")
	}

	if cfg.DistractorWeights.Base != 12 {
		t.Fatalf("DistractorWeights.Base = %d, want %d", cfg.DistractorWeights.Base, 12)
	}

	if cfg.DistractorWeights.SameScript != 7 {
		t.Fatalf("DistractorWeights.SameScript = %d, want %d", cfg.DistractorWeights.SameScript, 7)
	}

	if cfg.MutationRateLimit.Requests != 9 {
		t.Fatalf("MutationRateLimit.Requests = %d, want %d", cfg.MutationRateLimit.Requests, 9)
	}

	if cfg.MutationRateLimit.Window.String() != "30s" {
		t.Fatalf("MutationRateLimit.Window = %s, want %s", cfg.MutationRateLimit.Window, "30s")
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

func TestLoadRejectsInvalidBool(t *testing.T) {
	_, err := Load([]string{"SECURE_COOKIES=maybe"})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsInvalidRateLimit(t *testing.T) {
	_, err := Load([]string{"RATE_LIMIT_MUTATION_REQUESTS=0"})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}

	_, err = Load([]string{"RATE_LIMIT_MUTATION_WINDOW=0s"})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}

	_, err = Load([]string{"RATE_LIMIT_MUTATION_WINDOW=not-a-duration"})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
