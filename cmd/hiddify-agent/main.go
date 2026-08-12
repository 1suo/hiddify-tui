// hiddify-agent is the per-user system-proxy recovery helper. It has no TUI
// dependency and can be run by a systemd user unit.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/1suo/hiddify-tui/internal/agent"
)

func main() {
	flags := flag.NewFlagSet("hiddify-agent", flag.ExitOnError)
	socket := flags.String("socket", "/run/hiddify/control.sock", "daemon control socket")
	recoveryFile := flags.String("recovery-file", defaultRecoveryFile(), "proxy recovery state")
	restore := flags.Bool("restore", false, "restore saved system proxy state and exit")
	checkInterval := flags.Duration("check-interval", 30*time.Second, "expired lease check interval")
	flags.Parse(os.Args[1:])
	if *socket == "" || *checkInterval <= 0 {
		fmt.Fprintln(os.Stderr, "hiddify-agent: socket and check-interval must be set")
		os.Exit(2)
	}

	manager := agent.NewManager(agent.NewGSettingsBackend(), *recoveryFile)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *restore {
		if err := manager.Restore(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "hiddify-agent: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if _, err := manager.RestoreExpired(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "hiddify-agent: restore expired proxy state: %v\n", err)
	}
	ticker := time.NewTicker(*checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := manager.Restore(shutdownCtx)
			cancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				fmt.Fprintf(os.Stderr, "hiddify-agent: restore on shutdown: %v\n", err)
				os.Exit(1)
			}
			return
		case <-ticker.C:
			if _, err := manager.RestoreExpired(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "hiddify-agent: restore expired proxy state: %v\n", err)
			}
		}
	}
}

func defaultRecoveryFile() string {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		return filepath.Join(stateHome, "hiddify", "proxy-recovery.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "proxy-recovery.json"
	}
	return filepath.Join(home, ".local", "state", "hiddify", "proxy-recovery.json")
}
