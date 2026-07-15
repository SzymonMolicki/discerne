package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"discerne/backend/internal/quiz"
)

const (
	defaultAppName                   = "Discerne"
	defaultHTTPAddress               = ":8080"
	defaultAppTimezone               = "Europe/Warsaw"
	defaultDeviceCookieName          = "discerne_device"
	DefaultMutationRateLimitRequests = 30
	DefaultMutationRateLimitWindow   = time.Minute
)

// Config contains settings validated at startup.
type Config struct {
	AppName           string
	HTTPAddress       string
	AppTimezone       *time.Location
	DeviceCookieName  string
	SecureCookies     bool
	DistractorWeights quiz.DistractorWeights
	MutationRateLimit MutationRateLimitConfig
}

// MutationRateLimitConfig controls rate limiting for mutating quiz endpoints.
type MutationRateLimitConfig struct {
	Requests int
	Window   time.Duration
}

// Load builds Config from environment values.
func Load(environ []string) (Config, error) {
	values := parseEnvironment(environ)

	appName := strings.TrimSpace(values["APP_NAME"])
	if appName == "" {
		appName = defaultAppName
	}

	httpAddress := strings.TrimSpace(values["HTTP_ADDRESS"])
	if httpAddress == "" {
		httpAddress = defaultHTTPAddress
	}

	deviceCookieName := strings.TrimSpace(values["DEVICE_COOKIE_NAME"])
	if deviceCookieName == "" {
		deviceCookieName = defaultDeviceCookieName
	}

	secureCookies, err := boolFromEnv(values, "SECURE_COOKIES", false)
	if err != nil {
		return Config{}, err
	}

	timezoneName := strings.TrimSpace(values["APP_TIMEZONE"])
	if timezoneName == "" {
		timezoneName = defaultAppTimezone
	}

	location, err := time.LoadLocation(timezoneName)
	if err != nil {
		return Config{}, fmt.Errorf("load APP_TIMEZONE %q: %w", timezoneName, err)
	}

	distractorWeights, err := loadDistractorWeights(values)
	if err != nil {
		return Config{}, err
	}

	mutationRateLimit, err := loadMutationRateLimit(values)
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppName:           appName,
		HTTPAddress:       httpAddress,
		AppTimezone:       location,
		DeviceCookieName:  deviceCookieName,
		SecureCookies:     secureCookies,
		DistractorWeights: distractorWeights,
		MutationRateLimit: mutationRateLimit,
	}, nil
}

func loadMutationRateLimit(values map[string]string) (MutationRateLimitConfig, error) {
	requests, err := intFromEnv(values, "RATE_LIMIT_MUTATION_REQUESTS", DefaultMutationRateLimitRequests)
	if err != nil {
		return MutationRateLimitConfig{}, err
	}
	if requests <= 0 {
		return MutationRateLimitConfig{}, fmt.Errorf("RATE_LIMIT_MUTATION_REQUESTS must be greater than zero")
	}

	window, err := durationFromEnv(values, "RATE_LIMIT_MUTATION_WINDOW", DefaultMutationRateLimitWindow)
	if err != nil {
		return MutationRateLimitConfig{}, err
	}
	if window <= 0 {
		return MutationRateLimitConfig{}, fmt.Errorf("RATE_LIMIT_MUTATION_WINDOW must be greater than zero")
	}

	return MutationRateLimitConfig{
		Requests: requests,
		Window:   window,
	}, nil
}

func loadDistractorWeights(values map[string]string) (quiz.DistractorWeights, error) {
	weights := quiz.DefaultDistractorWeights()

	var err error
	weights.Base, err = intFromEnv(values, "DISTRACTOR_BASE_WEIGHT", weights.Base)
	if err != nil {
		return quiz.DistractorWeights{}, err
	}
	weights.SameFamily, err = intFromEnv(values, "DISTRACTOR_SAME_FAMILY_WEIGHT", weights.SameFamily)
	if err != nil {
		return quiz.DistractorWeights{}, err
	}
	weights.SameGroup, err = intFromEnv(values, "DISTRACTOR_SAME_GROUP_WEIGHT", weights.SameGroup)
	if err != nil {
		return quiz.DistractorWeights{}, err
	}
	weights.SameSubgroup, err = intFromEnv(values, "DISTRACTOR_SAME_SUBGROUP_WEIGHT", weights.SameSubgroup)
	if err != nil {
		return quiz.DistractorWeights{}, err
	}
	weights.SameContinent, err = intFromEnv(values, "DISTRACTOR_SAME_CONTINENT_WEIGHT", weights.SameContinent)
	if err != nil {
		return quiz.DistractorWeights{}, err
	}
	weights.SameScript, err = intFromEnv(values, "DISTRACTOR_SAME_SCRIPT_WEIGHT", weights.SameScript)
	if err != nil {
		return quiz.DistractorWeights{}, err
	}

	if err := weights.Validate(); err != nil {
		return quiz.DistractorWeights{}, fmt.Errorf("validate distractor weights: %w", err)
	}

	return weights, nil
}

func intFromEnv(values map[string]string, key string, fallback int) (int, error) {
	rawValue := strings.TrimSpace(values[key])
	if rawValue == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return 0, fmt.Errorf("parse %s %q: %w", key, rawValue, err)
	}
	return value, nil
}

func boolFromEnv(values map[string]string, key string, fallback bool) (bool, error) {
	rawValue := strings.TrimSpace(values[key])
	if rawValue == "" {
		return fallback, nil
	}

	value, err := strconv.ParseBool(rawValue)
	if err != nil {
		return false, fmt.Errorf("parse %s %q: %w", key, rawValue, err)
	}
	return value, nil
}

func durationFromEnv(values map[string]string, key string, fallback time.Duration) (time.Duration, error) {
	rawValue := strings.TrimSpace(values[key])
	if rawValue == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(rawValue)
	if err != nil {
		return 0, fmt.Errorf("parse %s %q: %w", key, rawValue, err)
	}
	return value, nil
}

func parseEnvironment(environ []string) map[string]string {
	values := make(map[string]string, len(environ))
	for _, item := range environ {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	return values
}
