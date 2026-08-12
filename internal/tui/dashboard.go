// Package tui holds interactive terminal screens.
package tui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/1suo/hiddify-tui/internal/control"
)

// Dashboard is the first interactive screen. It intentionally has no
// connection-changing key: quitting this client must never disconnect the
// daemon.
type Dashboard struct {
	snapshot control.Snapshot
	err      error
	width    int
	height   int
}

func NewDashboard(snapshot control.Snapshot, err error) Dashboard {
	return Dashboard{snapshot: snapshot, err: err}
}

func (m Dashboard) Init() tea.Cmd { return nil }

func (m Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	}
	return m, nil
}

func (m Dashboard) View() tea.View {
	state := m.snapshot.ConnectionState
	if state == "" {
		state = control.ConnectionStopped
	}
	profile := valueOr(m.snapshot.ActiveProfileName, "none")
	mode := valueOr(m.snapshot.EffectiveMode, "none")
	outbound := valueOr(m.snapshot.SelectedOutbound, "none")

	content := fmt.Sprintf("Hiddify\n\nConnection  %s\nProfile     %s\nMode        %s\nOutbound    %s", state, profile, mode, outbound)
	if m.err != nil {
		content += fmt.Sprintf("\n\nDaemon unavailable\n%s", m.err)
	}
	content += "\n\nq / Ctrl+C: quit (connection stays active)"

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

// Run opens the alternate-screen dashboard. Ctrl+C is a normal detach.
func Run(snapshot control.Snapshot, err error) error {
	_, runErr := tea.NewProgram(NewDashboard(snapshot, err)).Run()
	if errors.Is(runErr, tea.ErrInterrupted) {
		return nil
	}
	return runErr
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
