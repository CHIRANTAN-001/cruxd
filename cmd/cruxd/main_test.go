package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Mode
		wantErr bool
	}{
		{name: "worker", args: []string{"worker"}, want: ModeWorker},
		{name: "api", args: []string{"api"}, want: ModeAPI},
		{name: "scheduler", args: []string{"scheduler"}, want: ModeScheduler},
		{name: "ledger", args: []string{"ledger"}, want: ModeLedger},
		{name: "all", args: []string{"all"}, want: ModeAll},
		{name: "extra args are ignored", args: []string{"worker", "--verbose"}, want: ModeWorker},
		{name: "unknown", args: []string{"banana"}, wantErr: true},
		{name: "wrong case", args: []string{"Worker"}, wantErr: true},
		{name: "empty string", args: []string{""}, wantErr: true},
		{name: "missing args", args: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMode(tt.args)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			// main maps errUsage to exit code 2, so every parse error must wrap it.
			if tt.wantErr && !errors.Is(err, errUsage) {
				t.Errorf("err = %v, want it to wrap errUsage", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunInvalidModeReturnsUsageError(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background() // never cancelled: run must fail before it starts waiting

	err := run(ctx, []string{"banana"}, newTestLogger(&buf))

	if !errors.Is(err, errUsage) {
		t.Fatalf("err = %v, want it to wrap errUsage", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no log output for a usage error, got %q", buf.String())
	}
}

func TestRunStopsWhenContextIsCancelled(t *testing.T) {
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx, []string{"worker"}, newTestLogger(&buf)) }()

	// run must keep running until the context is cancelled.
	select {
	case err := <-errCh:
		t.Fatalf("run returned before cancellation: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel() // simulates Ctrl+C / SIGTERM

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not return within 2s of cancellation")
	}

	// Reading buf is safe here: run has returned, so nothing writes to it any more.
	lines := decodeLogLines(t, &buf)
	if len(lines) != 2 {
		t.Fatalf("got %d log lines, want 2: %q", len(lines), buf.String())
	}
	for i, wantMsg := range []string{"starting", "shutting down"} {
		if lines[i]["msg"] != wantMsg {
			t.Errorf("line %d msg = %v, want %q", i, lines[i]["msg"], wantMsg)
		}
		if lines[i]["mode"] != "worker" {
			t.Errorf("line %d mode = %v, want %q", i, lines[i]["mode"], "worker")
		}
		if lines[i]["level"] != "INFO" {
			t.Errorf("line %d level = %v, want %q", i, lines[i]["level"], "INFO")
		}
	}
}

// newTestLogger returns a JSON logger like the one main builds, but writing to w.
func newTestLogger(w *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, nil))
}

// decodeLogLines parses one JSON object per line, failing the test on invalid JSON.
func decodeLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("log line is not valid JSON: %q: %v", raw, err)
		}
		lines = append(lines, m)
	}
	return lines
}
