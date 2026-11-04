// Command cruxd is the single binary for the Cruxd job scheduler.
//
// The first argument selects the role the process plays:
//
//	cruxd api        HTTP API for creating and managing jobs
//	cruxd scheduler  leader-elected loop that queues due jobs
//	cruxd worker     consumes queued jobs and executes them
//	cruxd ledger     loads execution events into history tables
//	cruxd all        every role in one process, for local development
//
// The process runs until it receives SIGINT (Ctrl+C) or SIGTERM, then shuts
// down cleanly. Exit codes: 0 clean shutdown, 1 runtime error, 2 bad usage.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// Mode is the role this cruxd process plays. Each mode can be deployed and
// scaled on its own; ModeAll runs every role in one process.
type Mode string

// Supported modes, matched exactly (case-sensitive) against the first argument.
const (
	ModeAPI       Mode = "api"
	ModeScheduler Mode = "scheduler"
	ModeWorker    Mode = "worker"
	ModeLedger    Mode = "ledger"
	ModeAll       Mode = "all"
)

// errUsage reports a malformed command line. Errors that wrap it make main
// exit with status 2 (the shell convention for bad usage) instead of 1.
var errUsage = errors.New("usage: cruxd <api|scheduler|worker|ledger|all>")

// parseMode returns the Mode named by args[0], where args is os.Args[1:].
// Extra arguments are ignored. A missing or unknown mode returns an error
// that wraps errUsage.
func parseMode(args []string) (Mode, error) {
	if len(args) == 0 {
		return "", errUsage
	}
	switch args[0] {
	case string(ModeAPI):
		return ModeAPI, nil
	case string(ModeScheduler):
		return ModeScheduler, nil
	case string(ModeWorker):
		return ModeWorker, nil
	case string(ModeLedger):
		return ModeLedger, nil
	case string(ModeAll):
		return ModeAll, nil
	default:
		return "", fmt.Errorf("unknown mode %q: %w", args[0], errUsage)
	}
}

// run starts the process in the mode named by args and blocks until ctx is
// cancelled. It returns an error instead of exiting so that deferred cleanup
// always runs and tests can call it directly; only main chooses the exit code.
func run(ctx context.Context, args []string, logger *slog.Logger) error {
	mode, err := parseMode(args)
	if err != nil {
		return err
	}

	// Tag every log line with the mode so logs from different roles can be
	// told apart once they are collected in one place.
	logger = logger.With("mode", string(mode))

	logger.Info("starting")

	// Placeholder until the real components exist: wait for a shutdown signal.
	<-ctx.Done()

	logger.Info("shutting down")
	return nil
}

func main() {
	// JSON logs on stdout: container runtimes collect stdout, and structured
	// lines can be filtered by field (mode, job_id, ...) in a log store.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// ctx is cancelled on Ctrl+C or SIGTERM (what Docker and Kubernetes send
	// before killing a container). Calling stop restores default signal
	// handling, so a second Ctrl+C terminates immediately.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	// os.Exit skips deferred calls, so stop is called explicitly on every path.
	err := run(ctx, os.Args[1:], logger)
	if err != nil {
		if errors.Is(err, errUsage) {
			fmt.Fprintln(os.Stderr, err)
			stop()
			os.Exit(2)
		} else {
			logger.Error("exiting", "err", err)
			stop()
			os.Exit(1)
		}
	}

	stop()
}
