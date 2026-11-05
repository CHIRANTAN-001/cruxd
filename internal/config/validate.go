package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

// Validation bounds. All ranges are inclusive.
const (
	minPostgresMaxConns  int32 = 1
	maxPostgresMaxConns  int32 = 100
	minWorkerConcurrency       = 1
	maxWorkerConcurrency       = 1024
	// A lease shorter than this expires during normal heartbeat delays and
	// causes duplicate runs; a longer one leaves a crashed worker's job stuck.
	minLeaseDuration  = 5 * time.Second
	maxLeaseDuration  = 10 * time.Minute
	maxConnectTimeout = 2 * time.Minute
)

// Validate reports whether c holds values Cruxd is willing to run with.
// Load only checks that values parse; Validate checks ranges and address
// formats, so a Config built by hand (for example in a test) gets the same
// checks. Every problem is reported, joined with errors.Join, and each message
// names the environment variable to fix rather than the struct field.
func (c Config) Validate() error {
	var errs []error

	// Load never produces an empty DSN, but a hand-built Config can.
	if c.PostgresDSN == "" {
		errs = append(errs, fmt.Errorf("%s: must not be empty", EnvPostgresDSN))
	}

	if c.PostgresMaxConns < minPostgresMaxConns || c.PostgresMaxConns > maxPostgresMaxConns {
		errs = append(errs, fmt.Errorf("%s: must be between %d and %d, got %d", EnvPostgresMaxConns, minPostgresMaxConns, maxPostgresMaxConns, c.PostgresMaxConns))
	}

	if len(c.KafkaBrokers) == 0 {
		errs = append(errs, fmt.Errorf("%s: must have at least one entry", EnvKafkaBrokers))
	} else {
		for _, b := range c.KafkaBrokers {
			if err := validateHostPort(b); err != nil {
				errs = append(errs, fmt.Errorf("%s: broker %q: %w", EnvKafkaBrokers, b, err))
			}
		}
	}

	if err := validateHostPort(c.RedisAddr); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvRedisAddr, err))
	}

	if c.WorkerConcurrency < minWorkerConcurrency || c.WorkerConcurrency > maxWorkerConcurrency {
		errs = append(errs, fmt.Errorf("%s: must be between %d and %d, got %d", EnvWorkerConcurrency, minWorkerConcurrency, maxWorkerConcurrency, c.WorkerConcurrency))
	}

	if c.LeaseDuration < minLeaseDuration || c.LeaseDuration > maxLeaseDuration {
		errs = append(errs, fmt.Errorf("%s: must be between %v and %v, got %v", EnvLeaseDuration, minLeaseDuration, maxLeaseDuration, c.LeaseDuration))
	}

	// Zero would make every connect fail instantly; an unbounded value would
	// bring back the "startup hangs on a dead dependency" bug.
	if c.ConnectTimeout <= 0 || c.ConnectTimeout > maxConnectTimeout {
		errs = append(errs, fmt.Errorf("%s: must be greater than 0 and at most %v, got %v", EnvConnectTimeout, maxConnectTimeout, c.ConnectTimeout))
	}

	// errors.Join returns nil when errs is empty.
	return errors.Join(errs...)
}

// validateHostPort reports whether addr has the form host:port with a
// non-empty host and a port in 1..65535. It uses net.SplitHostPort rather
// than splitting on ":" so IPv6 addresses such as "[::1]:6379" work.
func validateHostPort(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid host:port %q: %w", addr, err)
	}
	// ":6379" splits without error but has no host.
	if host == "" {
		return fmt.Errorf("invalid host:port %q: host must not be empty", addr)
	}
	// bitSize 16 rejects anything above 65535 and any non-number; "localhost:"
	// splits without error, so an empty port is also caught here.
	portNum, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return fmt.Errorf("invalid host:port %q: port must be a number between 1 and 65535: %w", addr, err)
	}
	if portNum == 0 {
		return fmt.Errorf("invalid host:port %q: port must not be 0", addr)
	}
	return nil
}
