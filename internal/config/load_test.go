package config

import (
	"errors"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// mapLookup turns a map into a LookupFunc, so tests never touch the real
// process environment and can run in parallel.
func mapLookup(env map[string]string) LookupFunc {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load(mapLookup(nil))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if want := Default(); !reflect.DeepEqual(cfg, want) {
		t.Errorf("Load with empty env:\n got  %+v\n want %+v", cfg, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		env   map[string]string
		check func(t *testing.T, cfg Config)
	}{
		{
			name: "worker concurrency",
			env:  map[string]string{EnvWorkerConcurrency: "32"},
			check: func(t *testing.T, cfg Config) {
				if cfg.WorkerConcurrency != 32 {
					t.Errorf("WorkerConcurrency = %d, want 32", cfg.WorkerConcurrency)
				}
			},
		},
		{
			name: "log level is case-insensitive",
			env:  map[string]string{EnvLogLevel: "DEBUG"},
			check: func(t *testing.T, cfg Config) {
				if cfg.LogLevel != slog.LevelDebug {
					t.Errorf("LogLevel = %v, want DEBUG", cfg.LogLevel)
				}
			},
		},
		{
			name: "lease duration",
			env:  map[string]string{EnvLeaseDuration: "45s"},
			check: func(t *testing.T, cfg Config) {
				if cfg.LeaseDuration != 45*time.Second {
					t.Errorf("LeaseDuration = %v, want 45s", cfg.LeaseDuration)
				}
			},
		},
		{
			name: "connect timeout",
			env:  map[string]string{EnvConnectTimeout: "2500ms"},
			check: func(t *testing.T, cfg Config) {
				if cfg.ConnectTimeout != 2500*time.Millisecond {
					t.Errorf("ConnectTimeout = %v, want 2.5s", cfg.ConnectTimeout)
				}
			},
		},
		{
			name: "kafka brokers are split and trimmed",
			env:  map[string]string{EnvKafkaBrokers: "a:9092, b:9092"},
			check: func(t *testing.T, cfg Config) {
				want := []string{"a:9092", "b:9092"}
				if !reflect.DeepEqual(cfg.KafkaBrokers, want) {
					t.Errorf("KafkaBrokers = %q, want %q", cfg.KafkaBrokers, want)
				}
			},
		},
		{
			name: "postgres max conns",
			env:  map[string]string{EnvPostgresMaxConns: "25"},
			check: func(t *testing.T, cfg Config) {
				if cfg.PostgresMaxConns != 25 {
					t.Errorf("PostgresMaxConns = %d, want 25", cfg.PostgresMaxConns)
				}
			},
		},
		{
			name: "postgres dsn",
			env:  map[string]string{EnvPostgresDSN: "postgres://u:p@db:5432/x"},
			check: func(t *testing.T, cfg Config) {
				if cfg.PostgresDSN != "postgres://u:p@db:5432/x" {
					t.Errorf("PostgresDSN = %q, want postgres://u:p@db:5432/x", cfg.PostgresDSN)
				}
			},
		},
		{
			name: "blank value falls back to default",
			env:  map[string]string{EnvRedisAddr: "  "},
			check: func(t *testing.T, cfg Config) {
				if cfg.RedisAddr != defaultRedisAddr {
					t.Errorf("RedisAddr = %q, want default %q", cfg.RedisAddr, defaultRedisAddr)
				}
			},
		},
		{
			name: "surrounding spaces are trimmed",
			env:  map[string]string{EnvRedisAddr: "  redis:6379\t"},
			check: func(t *testing.T, cfg Config) {
				if cfg.RedisAddr != "redis:6379" {
					t.Errorf("RedisAddr = %q, want %q", cfg.RedisAddr, "redis:6379")
				}
			},
		},
		{
			// Load only parses; range checks belong to Validate, so 0 must load.
			name: "zero concurrency parses",
			env:  map[string]string{EnvWorkerConcurrency: "0"},
			check: func(t *testing.T, cfg Config) {
				if cfg.WorkerConcurrency != 0 {
					t.Errorf("WorkerConcurrency = %d, want 0", cfg.WorkerConcurrency)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := Load(mapLookup(tt.env))
			if err != nil {
				t.Fatalf("Load: unexpected error: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		// wantInErr lists substrings that must all appear in the error text.
		wantInErr []string
		// wantIs, if set, must be found in the error chain with errors.Is.
		wantIs error
	}{
		{
			name:      "concurrency not a number",
			env:       map[string]string{EnvWorkerConcurrency: "abc"},
			wantInErr: []string{EnvWorkerConcurrency},
			wantIs:    strconv.ErrSyntax,
		},
		{
			name:      "duration without a unit",
			env:       map[string]string{EnvLeaseDuration: "30"},
			wantInErr: []string{EnvLeaseDuration, "use a unit"},
		},
		{
			name:      "connect timeout garbage",
			env:       map[string]string{EnvConnectTimeout: "soon"},
			wantInErr: []string{EnvConnectTimeout},
		},
		{
			name:      "unknown log level",
			env:       map[string]string{EnvLogLevel: "loud"},
			wantInErr: []string{EnvLogLevel},
		},
		{
			// bitSize 32 must reject this instead of silently wrapping it.
			name:      "max conns overflows int32",
			env:       map[string]string{EnvPostgresMaxConns: "99999999999"},
			wantInErr: []string{EnvPostgresMaxConns},
			wantIs:    strconv.ErrRange,
		},
		{
			// Proves errors are joined, not returned on the first failure.
			name: "every bad variable is reported",
			env: map[string]string{
				EnvWorkerConcurrency: "many",
				EnvLeaseDuration:     "30",
				EnvLogLevel:          "loud",
			},
			wantInErr: []string{EnvWorkerConcurrency, EnvLeaseDuration, EnvLogLevel},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := Load(mapLookup(tt.env))
			if err == nil {
				t.Fatal("Load: expected an error, got nil")
			}
			for _, want := range tt.wantInErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Errorf("errors.Is(err, %v) = false; was the cause wrapped with %%w?", tt.wantIs)
			}
			// A failed Load must not hand back half-parsed values.
			if !reflect.DeepEqual(cfg, Config{}) {
				t.Errorf("Load returned non-zero Config on error: %+v", cfg)
			}
		})
	}
}

func TestSplitList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want []string
	}{
		{in: "a:9092", want: []string{"a:9092"}},
		{in: "a:9092, b:9092", want: []string{"a:9092", "b:9092"}},
		{in: "a:9092, ,b:9092,", want: []string{"a:9092", "b:9092"}},
		{in: " , ", want: nil},
		{in: "", want: nil},
	}

	for _, tt := range tests {
		if got := splitList(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitList(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
