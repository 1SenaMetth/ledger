// Package config loads runtime configuration from environment variables.
// Nothing is read from disk and nothing is committed; see .env.example.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env             string
	HTTPPort        string
	ShutdownTimeout time.Duration
	Database        Database
	Redis           Redis
	JWT             JWT
}

type Database struct {
	URL      string
	MaxConns int32
	MinConns int32
}

type Redis struct {
	Addr     string
	Password string
}

type JWT struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

func (c Config) IsProduction() bool { return c.Env == "production" }

// Load reads the environment and reports every problem at once rather than
// failing on the first missing variable. Discovering all five missing vars in
// one run beats five restarts.
func Load() (Config, error) {
	var errs []error
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	cfg := Config{
		Env:      optional("ENV", "development"),
		HTTPPort: optional("HTTP_PORT", "8080"),
	}

	var err error
	cfg.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", 15*time.Second)
	collect(err)

	cfg.Database.URL, err = required("DATABASE_URL")
	collect(err)
	cfg.Database.MaxConns, err = integer("DATABASE_MAX_CONNS", 10)
	collect(err)
	cfg.Database.MinConns, err = integer("DATABASE_MIN_CONNS", 2)
	collect(err)

	cfg.Redis.Addr = optional("REDIS_ADDR", "localhost:6379")
	cfg.Redis.Password = optional("REDIS_PASSWORD", "")

	cfg.JWT.Secret, err = required("JWT_SECRET")
	collect(err)
	cfg.JWT.AccessTTL, err = duration("JWT_ACCESS_TTL", 15*time.Minute)
	collect(err)
	cfg.JWT.RefreshTTL, err = duration("JWT_REFRESH_TTL", 30*24*time.Hour)
	collect(err)

	if cfg.IsProduction() && len(cfg.JWT.Secret) < 32 {
		collect(errors.New("JWT_SECRET must be at least 32 bytes in production"))
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
	}
	return cfg, nil
}

func required(key string) (string, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

func optional(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a valid duration (try 15s, 5m, 720h)", key, raw)
	}
	return d, nil
}

func integer(key string, fallback int32) (int32, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a valid integer", key, raw)
	}
	return int32(n), nil
}
