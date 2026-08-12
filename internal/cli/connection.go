package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/1suo/hiddify-tui/internal/control"
)

func ConnectionOperation(ctx context.Context, daemon control.ConnectionOperator, action string, profileID string, mode control.ConnectionMode, jsonOutput bool, stdout, stderr io.Writer) int {
	var err error
	switch action {
	case "connect":
		err = daemon.Connect(ctx, profileID, mode)
	case "disconnect":
		err = daemon.Disconnect(ctx)
	case "restart":
		err = daemon.Restart(ctx)
	default:
		fmt.Fprintf(stderr, "%s: unsupported connection operation\n", action)
		return ExitUsage
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", action, err)
		return ExitRejected
	}
	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(struct {
			SchemaVersion uint32 `json:"schema_version"`
			Operation     string `json:"operation"`
			ProfileID     string `json:"profile_id,omitempty"`
			Mode          string `json:"mode,omitempty"`
		}{SchemaVersion: 1, Operation: action, ProfileID: profileID, Mode: string(mode)}); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", action, err)
			return ExitRejected
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "%s requested\n", action)
	return ExitOK
}
