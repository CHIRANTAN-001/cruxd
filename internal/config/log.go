package config

import (
	"log/slog"
	"net/url"
)

const (
	// redactedPassword replaces a password inside an otherwise readable DSN.
	redactedPassword = "xxxxx"
	// redactedDSN replaces the whole DSN when it can't be safely parsed.
	redactedDSN = "[redacted]"
)

// LogValue implements slog.LogValuer, so logging a Config anywhere through
// slog emits its fields with the Postgres password redacted. Durations are
// logged as strings ("30s") because the JSON handler would otherwise print
// them as integer nanoseconds.
//
// fmt does not use LogValue: print a Config only through slog.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("postgres_dsn", redactDSN(c.PostgresDSN)),
		slog.Int("postgres_max_conns", int(c.PostgresMaxConns)),
		slog.Any("kafka_brokers", c.KafkaBrokers),
		slog.String("redis_addr", c.RedisAddr),
		slog.Int("worker_concurrency", c.WorkerConcurrency),
		slog.String("lease_duration", c.LeaseDuration.String()),
		slog.String("connect_timeout", c.ConnectTimeout.String()),
		slog.String("log_level", c.LogLevel.String()),
	)
}

// redactDSN returns dsn with any password replaced by redactedPassword,
// keeping user, host, port, database and options readable. If the DSN cannot
// be parsed safely, it returns redactedDSN instead: every failure path hides
// the whole value rather than risk printing the password.
func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return redactedDSN
	}
	// The key/value form ("host=db password=secret") parses without error as
	// a relative path, so only a URL scheme proves the password can be found.
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return redactedDSN
	}
	// url.URL.Query drops malformed pairs silently, which would leave a
	// password in RawQuery; ParseQuery reports the error instead.
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return redactedDSN
	}
	// pgx also accepts the password as a query parameter.
	if q.Has("password") {
		q.Set("password", redactedPassword)
		u.RawQuery = q.Encode()
	}
	// Redacted masks the password in the user:pass@ part.
	return u.Redacted()
}
