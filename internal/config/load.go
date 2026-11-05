package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Load builds a Config by applying the variables found through lookup on top
// of Default. It only parses values; range and format checks are done by
// Validate. Every parse failure is collected and returned together, joined
// with errors.Join, alongside a zero Config.
func Load(lookup LookupFunc) (Config, error) {
	// Collect every problem instead of stopping at the first, so one run
	// reports all bad variables.
	var errs []error
	cfg := Default()

	if v, ok := lookupValue(lookup, EnvPostgresDSN); ok {
		cfg.PostgresDSN = v
	}

	if v, ok := lookupValue(lookup, EnvRedisAddr); ok {
		cfg.RedisAddr = v
	}

	if v, ok := lookupValue(lookup, EnvKafkaBrokers); ok {
		cfg.KafkaBrokers = splitList(v)
	}

	if v, ok := lookupValue(lookup, EnvPostgresMaxConns); ok {
		// bitSize 32 turns an out-of-range value into an error; converting an
		// int afterwards would silently wrap it.
		if maxConns, err := strconv.ParseInt(v, 10, 32); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", EnvPostgresMaxConns, err))
		} else {
			cfg.PostgresMaxConns = int32(maxConns)
		}
	}

	if v, ok := lookupValue(lookup, EnvWorkerConcurrency); ok {
		if concurrency, err := strconv.Atoi(v); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", EnvWorkerConcurrency, err))
		} else {
			cfg.WorkerConcurrency = concurrency
		}
	}

	// A bare number such as "30" is the most common mistake, so the error
	// says to add a unit.
	if v, ok := lookupValue(lookup, EnvLeaseDuration); ok {
		if duration, err := time.ParseDuration(v); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w (use a unit, e.g. 30s)", EnvLeaseDuration, err))
		} else {
			cfg.LeaseDuration = duration
		}
	}

	if v, ok := lookupValue(lookup, EnvConnectTimeout); ok {
		if duration, err := time.ParseDuration(v); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w (use a unit, e.g. 10s)", EnvConnectTimeout, err))
		} else {
			cfg.ConnectTimeout = duration
		}
	}

	if v, ok := lookupValue(lookup, EnvLogLevel); ok {
		if err := cfg.LogLevel.UnmarshalText([]byte(v)); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", EnvLogLevel, err))
		}
	}

	// Return a zero Config on error so callers cannot use half-parsed values.
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}

	return cfg, nil
}

// lookupValue returns the trimmed value of key and whether it is usable.
// A blank value counts as unset, so a line like `CRUXD_REDIS_ADDR=` in a
// .env or compose file keeps the default instead of producing an empty value.
func lookupValue(lookup LookupFunc, key string) (string, bool) {
	v, ok := lookup(key)
	if !ok {
		return "", false
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", false
	}
	return v, true
}

// splitList splits a comma-separated list, trimming spaces and dropping empty
// entries. It returns nil when nothing remains; Validate rejects that case.
func splitList(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
