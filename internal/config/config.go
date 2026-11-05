// Package config loads Cruxd's runtime settings from environment variables.
//
// Every setting has a default that points at the local docker-compose stack.
// A variable that is unset or blank keeps its default; a variable that is set
// but cannot be parsed is an error, so a typo never silently falls back.
package config

import (
	"log/slog"
	"time"
)

// Environment variable names. Every name carries the CRUXD_ prefix.
const (
	EnvPostgresDSN       = "CRUXD_POSTGRES_DSN"
	EnvPostgresMaxConns  = "CRUXD_POSTGRES_MAX_CONNS"
	EnvKafkaBrokers      = "CRUXD_KAFKA_BROKERS"
	EnvRedisAddr         = "CRUXD_REDIS_ADDR"
	EnvWorkerConcurrency = "CRUXD_WORKER_CONCURRENCY"
	EnvLeaseDuration     = "CRUXD_LEASE_DURATION"
	EnvConnectTimeout    = "CRUXD_CONNECT_TIMEOUT"
	EnvLogLevel          = "CRUXD_LOG_LEVEL"
)

// Defaults point at the local docker-compose stack.
const (
	defaultPostgresDSN             = "postgres://cruxd:cruxd@localhost:5432/cruxd?sslmode=disable"
	defaultPostgresMaxConns  int32 = 10
	defaultKafkaBroker             = "localhost:9092"
	defaultRedisAddr               = "localhost:6379"
	defaultWorkerConcurrency       = 8
	defaultLeaseDuration           = 30 * time.Second
	defaultConnectTimeout          = 10 * time.Second
	defaultLogLevel                = slog.LevelInfo
)

// Config holds every runtime setting for one cruxd process. It is built once
// at startup by Load and treated as read-only afterwards.
type Config struct {
	// PostgresDSN is the connection string for the job store.
	PostgresDSN string
	// PostgresMaxConns caps this process's connection pool. It is set per
	// node because the sum across all nodes must stay under the server's
	// max_connections.
	PostgresMaxConns int32
	// KafkaBrokers are the seed brokers, as host:port.
	KafkaBrokers []string
	// RedisAddr is the Redis server, as host:port.
	RedisAddr string
	// WorkerConcurrency is the maximum number of jobs a worker runs at once.
	WorkerConcurrency int
	// LeaseDuration is how long a claim stays valid without a heartbeat.
	LeaseDuration time.Duration
	// ConnectTimeout bounds each dependency connection at startup, so a dead
	// dependency fails startup instead of hanging it.
	ConnectTimeout time.Duration
	// LogLevel is the minimum level the logger emits.
	LogLevel slog.Level
}

// LookupFunc reports the value of an environment variable and whether it is
// set. os.LookupEnv satisfies it; tests pass a map-backed function instead so
// they never touch the real process environment.
type LookupFunc func(key string) (string, bool)

// Default returns the configuration used when no variables are set.
// It is a function rather than a package-level variable so each caller gets
// its own KafkaBrokers slice and cannot change another caller's defaults.
func Default() Config {
	return Config{
		PostgresDSN:       defaultPostgresDSN,
		PostgresMaxConns:  defaultPostgresMaxConns,
		KafkaBrokers:      []string{defaultKafkaBroker},
		RedisAddr:         defaultRedisAddr,
		WorkerConcurrency: defaultWorkerConcurrency,
		LeaseDuration:     defaultLeaseDuration,
		ConnectTimeout:    defaultConnectTimeout,
		LogLevel:          defaultLogLevel,
	}
}
