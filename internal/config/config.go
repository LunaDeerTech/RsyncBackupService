package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultDataDir        = "./data"
	defaultPort           = "8080"
	defaultWorkerPoolSize = 3
	defaultLogLevel       = "info"
	defaultDevMode        = false
)

var dataSubDirs = []string{"keys", "relay", "temp", "logs"}

type Config struct {
	DataDir        string
	Port           string
	JWTSecret      string
	WorkerPoolSize int
	LogLevel       string
	DevMode        bool
}

func Load() (*Config, error) {
	fileValues, err := loadDotEnv(".env")
	if err != nil {
		return nil, err
	}

	workerPoolSize, err := resolveInt("RBS_WORKER_POOL_SIZE", fileValues, defaultWorkerPoolSize)
	if err != nil {
		return nil, err
	}

	devMode, err := resolveBool("RBS_DEV_MODE", fileValues, defaultDevMode)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		DataDir:        resolveString("RBS_DATA_DIR", fileValues, defaultDataDir),
		Port:           resolveString("RBS_PORT", fileValues, defaultPort),
		JWTSecret:      resolveString("RBS_JWT_SECRET", fileValues, ""),
		WorkerPoolSize: workerPoolSize,
		LogLevel:       resolveString("RBS_LOG_LEVEL", fileValues, defaultLogLevel),
		DevMode:        devMode,
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("RBS_JWT_SECRET is required")
	}

	return cfg, nil
}

func EnsureDataDirs(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir %q: %w", dataDir, err)
	}

	for _, subDir := range dataSubDirs {
		path := filepath.Join(dataDir, subDir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create data subdir %q: %w", path, err)
		}
	}

	return nil
}

func loadDotEnv(path string) (map[string]string, error) {
	values := make(map[string]string)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return values, nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("parse %s line %d: invalid KEY=VALUE entry", path, lineNumber)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("parse %s line %d: empty key", path, lineNumber)
		}

		values[key] = trimQuotes(strings.TrimSpace(value))
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	return values, nil
}

func resolveString(key string, fileValues map[string]string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return trimQuotes(strings.TrimSpace(value))
	}
	if value, ok := fileValues[key]; ok {
		return value
	}
	return defaultValue
}

func resolveInt(key string, fileValues map[string]string, defaultValue int) (int, error) {
	rawValue, hasValue := os.LookupEnv(key)
	if !hasValue {
		var ok bool
		rawValue, ok = fileValues[key]
		if !ok {
			return defaultValue, nil
		}
	}

	rawValue = trimQuotes(strings.TrimSpace(rawValue))
	if rawValue == "" {
		return 0, fmt.Errorf("%s must be a valid integer", key)
	}

	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}

	return value, nil
}

func resolveBool(key string, fileValues map[string]string, defaultValue bool) (bool, error) {
	rawValue, hasValue := os.LookupEnv(key)
	if !hasValue {
		var ok bool
		rawValue, ok = fileValues[key]
		if !ok {
			return defaultValue, nil
		}
	}

	rawValue = trimQuotes(strings.TrimSpace(rawValue))
	if rawValue == "" {
		return false, fmt.Errorf("%s must be a valid boolean", key)
	}

	value, err := strconv.ParseBool(rawValue)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean: %w", key, err)
	}

	return value, nil
}

func trimQuotes(value string) string {
	if len(value) < 2 {
		return value
	}

	if (value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"') {
		return value[1 : len(value)-1]
	}

	return value
}
