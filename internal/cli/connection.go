package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/profile"
)

type operationOutput struct {
	Action string `json:"action"`
}

// Connect starts the active profile, or the one given by profileID.
func Connect(ctx context.Context, core client.Client, store *profile.Store, profileID string, jsonOutput bool, stdout, stderr io.Writer) int {
	target, ok := activeOr(store, profileID)
	if !ok {
		WriteError(stderr, "connect", fmt.Errorf("no active profile"))
		return ExitRejected
	}
	if err := core.Connect(ctx, target.Content, target.Name); err != nil {
		WriteError(stderr, "connect", err)
		return ExitRejected
	}
	return operationResult("connect", jsonOutput, stdout)
}

// Disconnect stops the core.
func Disconnect(ctx context.Context, core client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	if err := core.Disconnect(ctx); err != nil {
		WriteError(stderr, "disconnect", err)
		return ExitRejected
	}
	return operationResult("disconnect", jsonOutput, stdout)
}

// Restart restarts the active profile.
func Restart(ctx context.Context, core client.Client, store *profile.Store, jsonOutput bool, stdout, stderr io.Writer) int {
	target, ok := store.Active()
	if !ok {
		WriteError(stderr, "restart", fmt.Errorf("no active profile"))
		return ExitRejected
	}
	if err := core.Restart(ctx, target.Content, target.Name); err != nil {
		WriteError(stderr, "restart", err)
		return ExitRejected
	}
	return operationResult("restart", jsonOutput, stdout)
}

func activeOr(store *profile.Store, profileID string) (profile.Profile, bool) {
	if profileID != "" {
		return store.Get(profileID)
	}
	return store.Active()
}

func operationResult(action string, jsonOutput bool, stdout io.Writer) int {
	if jsonOutput {
		_ = json.NewEncoder(stdout).Encode(struct {
			SchemaVersion uint32 `json:"schema_version"`
			Action        string `json:"action"`
		}{SchemaVersion: 1, Action: action})
		return ExitOK
	}
	fmt.Fprintf(stdout, "%s requested\n", action)
	return ExitOK
}
