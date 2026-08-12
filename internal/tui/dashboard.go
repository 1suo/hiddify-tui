// Package tui holds interactive terminal screens.
package tui

import (
	"errors"
	"fmt"
	"strconv"

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

	content := fmt.Sprintf("Hiddify\n\nConnection  %s\nProfile     %s\nMode        %s\nOutbound    %s\n\nDown        %s/s\nUp          %s/s\nTotal       %s down / %s up\nConnections %d\nMemory      %s\nAgent       %s", state, profile, mode, outbound, formatBytes(m.snapshot.Traffic.DownlinkBytesPerSecond), formatBytes(m.snapshot.Traffic.UplinkBytesPerSecond), formatBytes(m.snapshot.Traffic.TotalDownloadBytes), formatBytes(m.snapshot.Traffic.TotalUploadBytes), m.snapshot.System.ConnectionCount, formatBytes(m.snapshot.System.MemoryBytes), agentStatus(m.snapshot.Agent))
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

func agentStatus(agent control.AgentHealth) string {
	if !agent.Required {
		return "not required"
	}
	if agent.Connected {
		return "connected"
	}
	if agent.LastError != "" {
		return "unavailable: " + agent.LastError
	}
	return "unavailable"
}

func formatBytes(value uint64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	amount := float64(value)
	unit := 0
	for amount >= 1024 && unit < len(units)-1 {
		amount /= 1024
		unit++
	}
	if unit == 0 {
		return strconv.FormatUint(value, 10) + " " + units[unit]
	}
	return fmt.Sprintf("%.1f %s", amount, units[unit])
}
