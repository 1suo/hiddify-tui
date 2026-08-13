package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/1suo/hiddify-tui/internal/client"
)

// SettingsSet applies a Hiddify settings JSON through the core. The core has no
// settings read RPC, so this surface is set-only.
func SettingsSet(ctx context.Context, core client.Client, candidate []byte, stdout, stderr io.Writer) int {
	if err := core.ChangeSettings(ctx, string(candidate)); err != nil {
		WriteError(stderr, "settings set", err)
		return ExitRejected
	}
	fmt.Fprintln(stdout, "settings applied")
	return ExitOK
}

// SettingsValidate validates a config document (not Hiddify settings) via the
// core's parser.
func SettingsValidate(ctx context.Context, core client.Client, candidate []byte, stdout, stderr io.Writer) int {
	if err := core.Parse(ctx, string(candidate)); err != nil {
		WriteError(stderr, "settings validate", err)
		return ExitRejected
	}
	fmt.Fprintln(stdout, "valid")
	return ExitOK
}
