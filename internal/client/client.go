// Package client contains the thin client's daemon connection boundary.
package client

import (
	"context"
	"errors"

	"github.com/1suo/hiddify-tui/internal/control"
)

var ErrUnavailable = errors.New("hiddify daemon is unavailable")

// Snapshot loads and validates daemon state for a CLI or TUI render.
func Snapshot(ctx context.Context, daemon control.Client) (control.Snapshot, error) {
	snapshot, err := daemon.GetSnapshot(ctx)
	if err != nil {
		return control.Snapshot{}, err
	}
	if err := snapshot.ValidateCompatibility(); err != nil {
		return control.Snapshot{}, err
	}
	return snapshot, nil
}
