package database

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// URLFromEnvironment returns DATABASE_URL from the process environment or a .env file.
func URLFromEnvironment() string {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}

	for _, path := range []string{".env", "../.env"} {
		value, err := readEnvValue(path, "DATABASE_URL")
		if err == nil && value != "" {
			return value
		}
	}

	return ""
}

func readEnvValue(path string, name string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != name {
			continue
		}

		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		return value, nil
	}

	return "", scanner.Err()
}
