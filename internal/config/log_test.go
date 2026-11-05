package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// secret is the password used in every test DSN; it must never appear in output.
const secret = "s3cret"

func TestRedactDSN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "password in user info",
			dsn:  "postgres://cruxd:" + secret + "@localhost:5432/cruxd?sslmode=disable",
			want: "postgres://cruxd:xxxxx@localhost:5432/cruxd?sslmode=disable",
		},
		{
			name: "postgresql scheme",
			dsn:  "postgresql://cruxd:" + secret + "@db:5432/cruxd",
			want: "postgresql://cruxd:xxxxx@db:5432/cruxd",
		},
		{
			name: "password in query parameter",
			dsn:  "postgres://cruxd@localhost/cruxd?password=" + secret + "&sslmode=disable",
			want: "postgres://cruxd@localhost/cruxd?password=xxxxx&sslmode=disable",
		},
		{
			name: "user without password is unchanged",
			dsn:  "postgres://cruxd@localhost:5432/cruxd",
			want: "postgres://cruxd@localhost:5432/cruxd",
		},
		{
			name: "no user info is unchanged",
			dsn:  "postgres://localhost:5432/cruxd",
			want: "postgres://localhost:5432/cruxd",
		},
		{
			// url.Parse accepts this without error, so only the scheme check stops the leak.
			name: "key/value form is fully redacted",
			dsn:  "host=localhost user=cruxd password=" + secret,
			want: redactedDSN,
		},
		{
			// url.URL.Query silently drops the malformed pair, so the password would survive.
			name: "malformed query is fully redacted",
			dsn:  "postgres://cruxd@db/cruxd?password=" + secret + ";x&sslmode=disable",
			want: redactedDSN,
		},
		{
			name: "unparseable URL is fully redacted",
			dsn:  "postgres://cruxd:" + secret + "@%zz/cruxd",
			want: redactedDSN,
		},
		{
			name: "empty",
			dsn:  "",
			want: redactedDSN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := redactDSN(tt.dsn)
			if strings.Contains(got, secret) {
				t.Fatalf("redactDSN(%q) = %q: password leaked", tt.dsn, got)
			}
			if got != tt.want {
				t.Errorf("redactDSN(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}

func TestConfigLogValue(t *testing.T) {
	t.Parallel()

	cfg := Default()
	cfg.PostgresDSN = "postgres://cruxd:" + secret + "@localhost:5432/cruxd"

	// Log through the real JSON handler, the way run() will, so the test sees
	// exactly what ends up in the log store.
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("config", "config", cfg)
	out := buf.String()

	if strings.Contains(out, secret) {
		t.Fatalf("password leaked into log output: %s", out)
	}

	wants := []string{
		`"postgres_dsn":"postgres://cruxd:xxxxx@localhost:5432/cruxd"`,
		`"postgres_max_conns":10`,
		`"kafka_brokers":["localhost:9092"]`,
		`"redis_addr":"localhost:6379"`,
		`"worker_concurrency":8`,
		`"lease_duration":"30s"`,
		`"connect_timeout":"10s"`,
		`"log_level":"INFO"`,
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %s\nfull output: %s", want, out)
		}
	}
}
