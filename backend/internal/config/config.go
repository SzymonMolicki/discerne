package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	defaultAppName     = "Discerne"
	defaultHTTPAddress = ":8080"
	defaultAppTimezone = "Europe/Warsaw"
)

// Config contains settings validated at startup.
type Config struct {
	AppName     string
	HTTPAddress string
	AppTimezone *time.Location
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

	timezoneName := strings.TrimSpace(values["APP_TIMEZONE"])
	if timezoneName == "" {
		timezoneName = defaultAppTimezone
	}

	location, err := time.LoadLocation(timezoneName)
	if err != nil {
		return Config{}, fmt.Errorf("load APP_TIMEZONE %q: %w", timezoneName, err)
	}

	return Config{
		AppName:     appName,
		HTTPAddress: httpAddress,
		AppTimezone: location,
	}, nil
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
