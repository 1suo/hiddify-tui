package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/1suo/hiddify-tui/internal/client"
)

// Logs streams the core's log output until ctx ends or the stream closes.
func Logs(ctx context.Context, core client.Client, level client.LogLevel, jsonOutput bool, stdout, stderr io.Writer) int {
	entries, err := core.WatchLogs(ctx, level)
	if err != nil {
		WriteError(stderr, "logs", err)
		return ExitUnavailable
	}
	for entry := range entries {
		if jsonOutput {
			if err := json.NewEncoder(stdout).Encode(entry); err != nil {
				return ExitRejected
			}
			continue
		}
		fmt.Fprintf(stdout, "%s %-5s %-10s %s\n", entry.Time.Format("15:04:05"), entry.Level, entry.Component, entry.Message)
	}
	return ExitOK
}
