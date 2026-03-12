package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv   string
	HTTP     HTTPConfig
	Logging  LoggingConfig
	Database DatabaseConfig
	S3       S3Config
	JWT      JWTConfig
}

type HTTPConfig struct {
	Port string
}

type LoggingConfig struct {
	Dir      string
	FileName string
}

type DatabaseConfig struct {
	URL string
}

type S3Config struct {
	Region       string
	Endpoint     string
	AccessKeyID  string
	SecretAccess string
	PingBucket   string
	UsePathStyle bool
}

type JWTConfig struct {
	Issuer          string
	Audience        string
	Secret          string
	AccessTokenTTL  string
	RefreshTokenTTL string
}

func (h HTTPConfig) Address() string {
	return ":" + h.Port
}

func (l LoggingConfig) FilePath() string {
	return filepath.Join(l.Dir, l.FileName)
}

func Load() (Config, error) {
	loadDotEnv()

	cfg := Config{
		AppEnv: envOrDefault("APP_ENV", "development"),
		HTTP: HTTPConfig{
			Port: envOrDefault("HTTP_PORT", envOrDefault("PORT", "8080")),
		},
		Logging: LoggingConfig{
			Dir:      envOrDefault("LOG_DIR", "../logs"),
			FileName: envOrDefault("LOG_FILE", "backend.log"),
		},
		Database: DatabaseConfig{
			URL: envOrDefault("DATABASE_URL", "postgres://pen_drive:pen_drive@localhost:5433/pen_drive?sslmode=disable"),
		},
		S3: S3Config{
			Region:       envOrDefault("S3_REGION", "auto"),
			Endpoint:     resolveS3Endpoint(),
			AccessKeyID:  envOrDefault("S3_ACCESS_KEY_ID", envOrDefault("R2_ACCESS_KEY_ID", "")),
			SecretAccess: envOrDefault("S3_SECRET_ACCESS_KEY", envOrDefault("R2_SECRET_ACCESS_KEY", "")),
			PingBucket:   envOrDefault("S3_PING_BUCKET", envOrDefault("R2_DATA_BUCKET", "")),
			UsePathStyle: boolEnvOrDefault("S3_USE_PATH_STYLE", false),
		},
		JWT: JWTConfig{
			Issuer:          envOrDefault("JWT_ISSUER", "pen-drive-local"),
			Audience:        envOrDefault("JWT_AUDIENCE", "pen-drive-api"),
			Secret:          envOrDefault("JWT_SECRET", ""),
			AccessTokenTTL:  envOrDefault("ACCESS_TOKEN_TTL", "15m"),
			RefreshTokenTTL: envOrDefault("REFRESH_TOKEN_TTL", "720h"),
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

	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.S3.Region == "" {
		return Config{}, fmt.Errorf("S3_REGION is required")
	}

	if cfg.S3.Endpoint == "" {
		return Config{}, fmt.Errorf("S3_ENDPOINT is required")
	}

	if cfg.S3.AccessKeyID == "" {
		return Config{}, fmt.Errorf("S3_ACCESS_KEY_ID is required")
	}

	if cfg.S3.SecretAccess == "" {
		return Config{}, fmt.Errorf("S3_SECRET_ACCESS_KEY is required")
	}

	if cfg.S3.PingBucket == "" {
		return Config{}, fmt.Errorf("S3_PING_BUCKET or R2_DATA_BUCKET is required")
	}

	if cfg.JWT.Issuer == "" {
		return Config{}, fmt.Errorf("JWT_ISSUER is required")
	}

	if cfg.JWT.Audience == "" {
		return Config{}, fmt.Errorf("JWT_AUDIENCE is required")
	}

	if cfg.JWT.Secret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	if _, err := cfg.JWT.AccessTTL(); err != nil {
		return Config{}, fmt.Errorf("ACCESS_TOKEN_TTL: %w", err)
	}

	if _, err := cfg.JWT.RefreshTTL(); err != nil {
		return Config{}, fmt.Errorf("REFRESH_TOKEN_TTL: %w", err)
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func boolEnvOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func loadDotEnv() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load("../.env.local")
}

func resolveS3Endpoint() string {
	if endpoint := os.Getenv("S3_ENDPOINT"); endpoint != "" {
		return endpoint
	}

	if accountID := os.Getenv("R2_ACCOUNT_ID"); accountID != "" {
		return fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	}

	return ""
}

func (j JWTConfig) AccessTTL() (time.Duration, error) {
	return time.ParseDuration(j.AccessTokenTTL)
}

func (j JWTConfig) RefreshTTL() (time.Duration, error) {
	return time.ParseDuration(j.RefreshTokenTTL)
}
