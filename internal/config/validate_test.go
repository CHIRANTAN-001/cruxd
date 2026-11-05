package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// mutate changes a valid Default() config into the case under test.
		mutate func(c *Config)
		// wantInErr lists substrings that must all appear in the error;
		// empty means the config must be valid.
		wantInErr []string
	}{
		{name: "default is valid", mutate: func(c *Config) {}},
		{
			name: "bounds are inclusive",
			mutate: func(c *Config) {
				c.PostgresMaxConns = maxPostgresMaxConns
				c.WorkerConcurrency = maxWorkerConcurrency
				c.LeaseDuration = minLeaseDuration
				c.ConnectTimeout = maxConnectTimeout
			},
		},
		{
			name:   "ipv6 redis address",
			mutate: func(c *Config) { c.RedisAddr = "[::1]:6379" },
		},
		{
			name:      "empty dsn",
			mutate:    func(c *Config) { c.PostgresDSN = "" },
			wantInErr: []string{EnvPostgresDSN},
		},
		{
			name:      "zero max conns",
			mutate:    func(c *Config) { c.PostgresMaxConns = 0 },
			wantInErr: []string{EnvPostgresMaxConns, "got 0"},
		},
		{
			name:      "too many max conns",
			mutate:    func(c *Config) { c.PostgresMaxConns = maxPostgresMaxConns + 1 },
			wantInErr: []string{EnvPostgresMaxConns},
		},
		{
			name:      "no brokers",
			mutate:    func(c *Config) { c.KafkaBrokers = nil },
			wantInErr: []string{EnvKafkaBrokers},
		},
		{
			name:      "one bad broker among good ones",
			mutate:    func(c *Config) { c.KafkaBrokers = []string{"a:9092", "b-no-port"} },
			wantInErr: []string{EnvKafkaBrokers, "b-no-port"},
		},
		{
			name:      "redis without port",
			mutate:    func(c *Config) { c.RedisAddr = "localhost" },
			wantInErr: []string{EnvRedisAddr},
		},
		{
			name:      "zero concurrency",
			mutate:    func(c *Config) { c.WorkerConcurrency = 0 },
			wantInErr: []string{EnvWorkerConcurrency, "got 0"},
		},
		{
			name:      "too much concurrency",
			mutate:    func(c *Config) { c.WorkerConcurrency = maxWorkerConcurrency + 1 },
			wantInErr: []string{EnvWorkerConcurrency},
		},
		{
			name:      "lease too short",
			mutate:    func(c *Config) { c.LeaseDuration = time.Second },
			wantInErr: []string{EnvLeaseDuration},
		},
		{
			name:      "lease too long",
			mutate:    func(c *Config) { c.LeaseDuration = time.Hour },
			wantInErr: []string{EnvLeaseDuration},
		},
		{
			name:      "zero connect timeout",
			mutate:    func(c *Config) { c.ConnectTimeout = 0 },
			wantInErr: []string{EnvConnectTimeout},
		},
		{
			name:      "connect timeout too long",
			mutate:    func(c *Config) { c.ConnectTimeout = maxConnectTimeout + time.Second },
			wantInErr: []string{EnvConnectTimeout},
		},
		{
			// Proves Validate joins errors like Load does.
			name: "every problem is reported",
			mutate: func(c *Config) {
				c.WorkerConcurrency = 0
				c.LeaseDuration = 0
				c.RedisAddr = "nope"
			},
			wantInErr: []string{EnvWorkerConcurrency, EnvLeaseDuration, EnvRedisAddr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := Default()
			tt.mutate(&cfg)
			err := cfg.Validate()

			if len(tt.wantInErr) == 0 {
				if err != nil {
					t.Fatalf("Validate: unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate: expected an error, got nil")
			}
			for _, want := range tt.wantInErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

func TestValidateHostPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		addr    string
		wantErr bool
	}{
		{addr: "localhost:6379"},
		{addr: "10.0.0.5:9092"},
		{addr: "[::1]:6379"},
		{addr: "kafka-0.kafka.svc:9092"},
		{addr: "localhost", wantErr: true},       // no port
		{addr: ":6379", wantErr: true},           // no host
		{addr: "localhost:", wantErr: true},      // empty port
		{addr: "localhost:abc", wantErr: true},   // port not a number
		{addr: "localhost:0", wantErr: true},     // port 0
		{addr: "localhost:70000", wantErr: true}, // port out of range
		{addr: "", wantErr: true},
	}

	for _, tt := range tests {
		err := validateHostPort(tt.addr)
		if gotErr := err != nil; gotErr != tt.wantErr {
			t.Errorf("validateHostPort(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
		}
	}
}
