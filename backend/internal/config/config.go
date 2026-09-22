package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	APIPort     int
	WorkerID    string

	// Worker polling interval when no jobs are available.
	PollInterval time.Duration

	// How long a job can run before the worker cancels it.
	JobTimeout time.Duration

	// Visibility timeout for claimed jobs in Redis before they are considered abandoned.
	VisibilityTimeout time.Duration

	// Heartbeat interval for worker liveness in Redis.
	HeartbeatInterval time.Duration

	// Number of concurrent jobs a single worker process can execute.
	Concurrency int

	// Duration of silence after which a worker is considered dead.
	WorkerDeadTimeout time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/workflow_engine?sslmode=disable"),
		RedisURL:          getEnv("REDIS_URL", "redis://localhost:6379/0"),
		APIPort:           getEnvInt("API_PORT", 8080),
		WorkerID:          getEnv("WORKER_ID", ""),
		PollInterval:      getEnvDuration("POLL_INTERVAL", 500*time.Millisecond),
		JobTimeout:        getEnvDuration("JOB_TIMEOUT", 5*time.Minute),
		VisibilityTimeout: getEnvDuration("VISIBILITY_TIMEOUT", 30*time.Second),
		HeartbeatInterval: getEnvDuration("HEARTBEAT_INTERVAL", 10*time.Second),
		Concurrency:       getEnvInt("CONCURRENCY", 5),
		WorkerDeadTimeout: getEnvDuration("WORKER_DEAD_TIMEOUT", 30*time.Second),
	}

	if cfg.WorkerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		cfg.WorkerID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
