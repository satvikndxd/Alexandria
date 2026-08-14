// Package config loads runtime configuration from the environment.
// Alexandria follows 12-factor strictly: no config files in production,
// everything injectable, everything with a sane local-dev default that
// matches infrastructure/docker-compose.yml.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// HTTP
	Addr            string        // listen address for the API server
	ShutdownTimeout time.Duration // graceful drain window

	// Postgres
	DatabaseURL string

	// NATS JetStream
	NATSURL string

	// Meilisearch
	MeiliURL string
	MeiliKey string

	// MinIO / S3
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string

	// Anti-slop friction knobs (overridable for tests, never disabled in prod)
	ReviewMinChars    int
	NewAccountDaily   int // reviews/day for accounts below the reputation gate
	TrustedDaily      int // reviews/day once "Trusted Reader"
	TrustedReputation int
}

func Load() Config {
	return Config{
		Addr:            getenv("API_ADDR", ":8080"),
		ShutdownTimeout: getdur("API_SHUTDOWN_TIMEOUT", 15*time.Second),

		DatabaseURL: getenv("DATABASE_URL", "postgres://alexandria:alexandria@localhost:5432/alexandria?sslmode=disable"),
		NATSURL:     getenv("NATS_URL", "nats://localhost:4222"),
		MeiliURL:    getenv("MEILI_URL", "http://localhost:7700"),
		MeiliKey:    getenv("MEILI_MASTER_KEY", "alexandria-dev-key"),

		S3Endpoint:  getenv("S3_ENDPOINT", "http://localhost:9000"),
		S3AccessKey: getenv("S3_ACCESS_KEY", "alexandria"),
		S3SecretKey: getenv("S3_SECRET_KEY", "alexandria-dev"),
		S3Bucket:    getenv("S3_BUCKET", "alexandria"),

		ReviewMinChars:    getint("FRICTION_REVIEW_MIN_CHARS", 150),
		NewAccountDaily:   getint("FRICTION_NEW_ACCOUNT_DAILY_REVIEWS", 2),
		TrustedDaily:      getint("FRICTION_TRUSTED_DAILY_REVIEWS", 10),
		TrustedReputation: getint("FRICTION_TRUSTED_REPUTATION", 100),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getint(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getdur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func (c Config) String() string {
	return fmt.Sprintf("addr=%s db=%s nats=%s meili=%s", c.Addr, redactDSN(c.DatabaseURL), c.NATSURL, c.MeiliURL)
}

// redactDSN strips credentials before a DSN reaches logs.
func redactDSN(dsn string) string {
	// cheap and safe: show only the tail after '@' if present
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '@' {
			return "postgres://…@" + dsn[i+1:]
		}
	}
	return dsn
}
