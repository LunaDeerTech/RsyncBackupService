package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultPort = 8080

type Config struct {
	Port          int
	DataDir       string
	JWTSecret     string
	AdminUser     string
	AdminPassword string
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	cfg := Config{
		Port: defaultPort,
	}

	var validationErrors []string

	portValue := strings.TrimSpace(os.Getenv("RBS_PORT"))
	if portValue != "" {
		port, err := strconv.Atoi(portValue)
		if err != nil || port < 1 || port > 65535 {
			validationErrors = append(validationErrors, "RBS_PORT must be a valid TCP port")
		} else {
			cfg.Port = port
		}
	}

	cfg.DataDir = strings.TrimSpace(os.Getenv("RBS_DATA_DIR"))
	cfg.JWTSecret = strings.TrimSpace(os.Getenv("RBS_JWT_SECRET"))
	cfg.AdminUser = strings.TrimSpace(os.Getenv("RBS_ADMIN_USER"))
	cfg.AdminPassword = strings.TrimSpace(os.Getenv("RBS_ADMIN_PASSWORD"))

	if cfg.DataDir == "" {
		validationErrors = append(validationErrors, "RBS_DATA_DIR is required")
	}
	if cfg.JWTSecret == "" {
		validationErrors = append(validationErrors, "RBS_JWT_SECRET is required")
	}
	if cfg.AdminUser == "" {
		validationErrors = append(validationErrors, "RBS_ADMIN_USER is required")
	}
	if cfg.AdminPassword == "" {
		validationErrors = append(validationErrors, "RBS_ADMIN_PASSWORD is required")
	}

	if len(validationErrors) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %s", strings.Join(validationErrors, "; "))
	}

	return cfg, nil
}

func loadDotEnv() error {
	file, err := os.Open(".env")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("line %d missing '=' separator", lineNumber)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("line %d has an empty key", lineNumber)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}