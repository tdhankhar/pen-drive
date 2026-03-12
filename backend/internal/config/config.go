package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	AppEnv  string
	HTTP    HTTPConfig
	Logging LoggingConfig
}

type HTTPConfig struct {
	Port string
}

type LoggingConfig struct {
	Dir      string
	FileName string
}

func (h HTTPConfig) Address() string {
	return ":" + h.Port
}

func (l LoggingConfig) FilePath() string {
	return filepath.Join(l.Dir, l.FileName)
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv: envOrDefault("APP_ENV", "development"),
		HTTP: HTTPConfig{
			Port: envOrDefault("HTTP_PORT", "8080"),
		},
		Logging: LoggingConfig{
			Dir:      envOrDefault("LOG_DIR", "../logs"),
			FileName: envOrDefault("LOG_FILE", "backend.log"),
		},
	}

	if cfg.HTTP.Port == "" {
		return Config{}, fmt.Errorf("HTTP_PORT is required")
	}

	if cfg.Logging.Dir == "" {
		return Config{}, fmt.Errorf("LOG_DIR is required")
	}

	if cfg.Logging.FileName == "" {
		return Config{}, fmt.Errorf("LOG_FILE is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
