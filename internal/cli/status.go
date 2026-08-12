// Package cli implements noninteractive command behavior.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
)

const (
	ExitOK          = 0
	ExitUsage       = 2
	ExitUnavailable = 3
	ExitRejected    = 4
	ExitPrivilege   = 5
)

func Status(ctx context.Context, daemon control.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	snapshot, err := client.Snapshot(ctx, daemon)
	if err != nil {
		fmt.Fprintf(stderr, "status: %v\n", err)
		return ExitUnavailable
	}
	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(struct {
			SchemaVersion uint32           `json:"schema_version"`
			Snapshot      control.Snapshot `json:"snapshot"`
		}{SchemaVersion: 1, Snapshot: snapshot}); err != nil {
			fmt.Fprintf(stderr, "status: %v\n", err)
			return ExitRejected
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "Connection: %s\n", snapshot.ConnectionState)
	fmt.Fprintf(stdout, "Profile: %s\n", valueOr(snapshot.ActiveProfileName, "none"))
	fmt.Fprintf(stdout, "Mode: %s\n", valueOr(snapshot.EffectiveMode, "none"))
	fmt.Fprintf(stdout, "Outbound: %s\n", valueOr(snapshot.SelectedOutbound, "none"))
	return ExitOK
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
