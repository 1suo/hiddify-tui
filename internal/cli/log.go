package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/1suo/hiddify-tui/internal/control"
)

func Logs(ctx context.Context, daemon control.LogReader, tail uint32, level control.LogLevel, follow, jsonOutput bool, stdout, stderr io.Writer) int {
	entries, err := daemon.TailLogs(ctx, tail, level, follow)
	if err != nil {
		fmt.Fprintf(stderr, "logs: %v\n", err)
		return ExitUnavailable
	}
	for entry := range entries {
		if jsonOutput {
			if err := json.NewEncoder(stdout).Encode(entry); err != nil {
				fmt.Fprintf(stderr, "logs: %v\n", err)
				return ExitRejected
			}
			continue
		}
		timestamp := time.Unix(0, entry.TimestampUnix).Format(time.RFC3339)
		fmt.Fprintf(stdout, "%s %-5s %-12s %s\n", timestamp, entry.Level, entry.Component, entry.Message)
	}
	return ExitOK
}

func ClearLogs(ctx context.Context, daemon control.LogReader, stdout, stderr io.Writer) int {
	if err := daemon.ClearLogs(ctx); err != nil {
		fmt.Fprintf(stderr, "logs clear: %v\n", err)
		return ExitRejected
	}
	fmt.Fprintln(stdout, "Daemon log buffer cleared")
	return ExitOK
}
