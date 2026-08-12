package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/1suo/hiddify-tui/internal/control"
)

func SettingsShow(ctx context.Context, daemon control.SettingsOperator, stdout, stderr io.Writer) int {
	settings, err := daemon.GetSettings(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "settings show: %v\n", err)
		return ExitUnavailable
	}
	return writeJSON(stdout, stderr, "settings show", settings.RedactedJSON)
}

func SettingsValidate(ctx context.Context, daemon control.SettingsOperator, candidate []byte, stdout, stderr io.Writer) int {
	result, err := daemon.ValidateSettings(ctx, candidate)
	if err != nil {
		fmt.Fprintf(stderr, "settings validate: %v\n", err)
		return ExitRejected
	}
	if err := json.NewEncoder(stdout).Encode(struct {
		SchemaVersion uint32                   `json:"schema_version"`
		Result        control.ValidationResult `json:"result"`
	}{SchemaVersion: 1, Result: result}); err != nil {
		fmt.Fprintf(stderr, "settings validate: %v\n", err)
		return ExitRejected
	}
	if !result.Valid {
		return ExitRejected
	}
	return ExitOK
}

func SettingsWrite(ctx context.Context, daemon control.SettingsOperator, action string, candidate []byte, stdout, stderr io.Writer) int {
	var settings control.Settings
	var err error
	switch action {
	case "set":
		settings, err = daemon.UpdateSettings(ctx, candidate)
	case "import":
		settings, err = daemon.ImportSettings(ctx, candidate)
	case "reset":
		settings, err = daemon.ResetSettings(ctx)
	default:
		return ExitUsage
	}
	if err != nil {
		fmt.Fprintf(stderr, "settings %s: %v\n", action, err)
		return ExitRejected
	}
	return writeJSON(stdout, stderr, "settings "+action, settings.RedactedJSON)
}

func SettingsExport(ctx context.Context, daemon control.SettingsOperator, includeSecrets bool, stdout, stderr io.Writer) int {
	data, err := daemon.ExportSettings(ctx, includeSecrets)
	if err != nil {
		fmt.Fprintf(stderr, "settings export: %v\n", err)
		return ExitRejected
	}
	return writeJSON(stdout, stderr, "settings export", data)
}

func writeJSON(stdout, stderr io.Writer, operation string, data []byte) int {
	var parsed json.RawMessage
	if !json.Valid(data) {
		fmt.Fprintf(stderr, "%s: daemon returned invalid JSON\n", operation)
		return ExitRejected
	}
	parsed = data
	if err := json.NewEncoder(stdout).Encode(parsed); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", operation, err)
		return ExitRejected
	}
	return ExitOK
}
