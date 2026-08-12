// Package tui holds terminal screen models. Rendering remains dependency-free
// until Bubble Tea is added with the first interactive event stream.
package tui

import (
	"fmt"

	"github.com/1suo/hiddify-tui/internal/control"
)

func Dashboard(snapshot control.Snapshot) string {
	profile := snapshot.ActiveProfileName
	if profile == "" {
		profile = "none"
	}
	return fmt.Sprintf("Hiddify\n\nState: %s\nProfile: %s\nMode: %s\nOutbound: %s\n\nq: quit (connection stays active)", snapshot.ConnectionState, profile, snapshot.EffectiveMode, snapshot.SelectedOutbound)
}
