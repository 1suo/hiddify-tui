// Package tui holds interactive terminal screens.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
)

// Dashboard is the first interactive screen. Quitting it never disconnects the
// daemon; connection actions are explicit local-control requests.
type Dashboard struct {
	snapshot          control.Snapshot
	err               error
	width             int
	height            int
	updates           <-chan dashboardUpdate
	page              page
	profiles          []control.Profile
	groups            []control.OutboundGroup
	connection        control.ConnectionOperator
	profilesAPI       control.ProfileWriter
	outboundsAPI      control.OutboundOperator
	ctx               context.Context
	action            string
	cursor            int
	confirmDisconnect bool
}

type page string

const (
	pageDashboard page = "dashboard"
	pageProfiles  page = "profiles"
	pageOutbounds page = "outbounds"
	pageLogs      page = "logs"
	pageSettings  page = "settings"
)

type dashboardUpdate struct {
	snapshot control.Snapshot
	err      error
	profiles []control.Profile
	groups   []control.OutboundGroup
}

type actionResult struct {
	action string
	err    error
}

func NewDashboard(snapshot control.Snapshot, err error) Dashboard {
	return Dashboard{snapshot: snapshot, err: err}
}

func (m Dashboard) Init() tea.Cmd { return waitForDashboardUpdate(m.updates) }

func (m Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() != "x" {
			m.confirmDisconnect = false
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1", "d":
			m.page = pageDashboard
		case "2", "p":
			m.page = pageProfiles
			m.cursor = 0
		case "3", "o":
			m.page = pageOutbounds
			m.cursor = 0
		case "4", "l":
			m.page = pageLogs
		case "5", "s":
			m.page = pageSettings
		case "c":
			return m, m.connectionAction("connect")
		case "x":
			if !m.confirmDisconnect {
				m.confirmDisconnect = true
				m.action = "Press x again to disconnect"
				return m, nil
			}
			m.confirmDisconnect = false
			return m, m.connectionAction("disconnect")
		case "r":
			return m, m.connectionAction("restart")
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter":
			return m, m.selectionAction(false)
		case "t":
			return m, m.selectionAction(true)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case dashboardUpdate:
		m.snapshot, m.err, m.profiles, m.groups = msg.snapshot, msg.err, msg.profiles, msg.groups
		return m, waitForDashboardUpdate(m.updates)
	case actionResult:
		if msg.err != nil {
			m.action = fmt.Sprintf("%s failed: %v", msg.action, msg.err)
		} else {
			m.action = msg.action + " requested"
		}
	}
	return m, nil
}

func (m Dashboard) View() tea.View {
	var content string
	switch m.page {
	case pageProfiles:
		content = m.profilesView()
	case pageOutbounds:
		content = m.outboundsView()
	case pageLogs:
		content = "Logs\n\nUse hiddify-tui logs --follow for a scriptable live stream.\nDaemon-side redaction is always preserved."
	case pageSettings:
		content = "Settings\n\nUse hiddify-tui settings show|validate|set|import|export.\nSettings changes are validated and committed by the daemon."
	default:
		content = m.dashboardView()
	}
	if m.err != nil {
		content += fmt.Sprintf("\n\nDaemon unavailable\n%s", m.err)
	}
	if m.action != "" {
		content += "\n\n" + m.action
	}
	content += "\n\n1 Dashboard  2 Profiles  3 Outbounds  4 Logs  5 Settings\nc connect  x disconnect  r restart  ↑/↓ select  Enter activate/select  t test outbound\nq / Ctrl+C quit (connection stays active)"

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m Dashboard) dashboardView() string {
	state := m.snapshot.ConnectionState
	if state == "" {
		state = control.ConnectionStopped
	}
	profile := valueOr(m.snapshot.ActiveProfileName, "none")
	mode := valueOr(m.snapshot.EffectiveMode, "none")
	outbound := valueOr(m.snapshot.SelectedOutbound, "none")

	return fmt.Sprintf("Hiddify\n\nConnection  %s\nProfile     %s\nMode        %s\nOutbound    %s\n\nDown        %s/s\nUp          %s/s\nTotal       %s down / %s up\nConnections %d\nMemory      %s\nAgent       %s", state, profile, mode, outbound, formatBytes(m.snapshot.Traffic.DownlinkBytesPerSecond), formatBytes(m.snapshot.Traffic.UplinkBytesPerSecond), formatBytes(m.snapshot.Traffic.TotalDownloadBytes), formatBytes(m.snapshot.Traffic.TotalUploadBytes), m.snapshot.System.ConnectionCount, formatBytes(m.snapshot.System.MemoryBytes), agentStatus(m.snapshot.Agent))
}

func (m Dashboard) profilesView() string {
	content := "Profiles\n"
	if len(m.profiles) == 0 {
		return content + "\nNo profiles available.\nUse hiddify-tui profile add or profile add-file."
	}
	for index, profile := range m.profiles {
		active := " "
		if profile.Active {
			active = "*"
		}
		cursor := " "
		if index == m.cursor {
			cursor = ">"
		}
		content += fmt.Sprintf("\n%s%s %s  [%s]  %s", cursor, active, profile.Name, profile.Kind, valueOr(profile.RedactedURL, "local"))
	}
	return content
}

func (m Dashboard) outboundsView() string {
	content := "Outbounds\n"
	if len(m.groups) == 0 {
		return content + "\nNo outbounds available."
	}
	index := 0
	for _, group := range m.groups {
		content += "\n\n" + group.Name
		for _, outbound := range group.Outbounds {
			selected := " "
			if outbound.ID == group.SelectedOutboundID {
				selected = "*"
			}
			delay := "-"
			if outbound.DelayMillis > 0 {
				delay = fmt.Sprintf("%d ms", outbound.DelayMillis)
			}
			cursor := " "
			if index == m.cursor {
				cursor = ">"
			}
			content += fmt.Sprintf("\n%s%s %s  %s  %s", cursor, selected, outbound.Tag, outbound.Protocol, delay)
			index++
		}
	}
	return content
}

// Run opens the alternate-screen dashboard. Ctrl+C is a normal detach.
func Run(snapshot control.Snapshot, err error) error {
	_, runErr := tea.NewProgram(NewDashboard(snapshot, err)).Run()
	if errors.Is(runErr, tea.ErrInterrupted) {
		return nil
	}
	return runErr
}

// RunLive receives an initial snapshot and then keeps it current from the
// daemon event stream. A dropped stream is retried with bounded backoff; the
// client-side state reducer handles sequence gaps by fetching a fresh snapshot.
func RunLive(ctx context.Context, daemon control.Client, watcher control.Watcher) error {
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	updates := make(chan dashboardUpdate, 1)
	model := NewDashboard(control.Snapshot{}, nil)
	model.updates = updates
	model.ctx = ctx
	model.connection, _ = daemon.(control.ConnectionOperator)
	model.profilesAPI, _ = daemon.(control.ProfileWriter)
	model.outboundsAPI, _ = daemon.(control.OutboundOperator)
	go streamDashboard(watchCtx, daemon, watcher, updates)
	_, runErr := tea.NewProgram(model).Run()
	if errors.Is(runErr, tea.ErrInterrupted) {
		return nil
	}
	return runErr
}

func (m Dashboard) connectionAction(action string) tea.Cmd {
	if m.connection == nil {
		return func() tea.Msg {
			return actionResult{action: action, err: errors.New("daemon does not support connection controls")}
		}
	}
	return func() tea.Msg {
		var err error
		switch action {
		case "connect":
			err = m.connection.Connect(m.ctx, "", "")
		case "disconnect":
			err = m.connection.Disconnect(m.ctx)
		case "restart":
			err = m.connection.Restart(m.ctx)
		}
		return actionResult{action: action, err: err}
	}
}

func (m *Dashboard) moveCursor(delta int) {
	limit := 0
	if m.page == pageProfiles {
		limit = len(m.profiles)
	}
	if m.page == pageOutbounds {
		limit = len(m.outboundChoices())
	}
	if limit == 0 {
		m.cursor = 0
		return
	}
	m.cursor = (m.cursor + delta + limit) % limit
}

type outboundChoice struct {
	groupID  string
	outbound control.Outbound
}

func (m Dashboard) outboundChoices() []outboundChoice {
	var choices []outboundChoice
	for _, group := range m.groups {
		for _, outbound := range group.Outbounds {
			choices = append(choices, outboundChoice{group.ID, outbound})
		}
	}
	return choices
}

func (m Dashboard) selectionAction(test bool) tea.Cmd {
	if m.page == pageProfiles {
		if m.profilesAPI == nil || m.cursor >= len(m.profiles) {
			return nil
		}
		profile := m.profiles[m.cursor]
		return func() tea.Msg {
			return actionResult{action: "activate profile", err: m.profilesAPI.SetActiveProfile(m.ctx, profile.ID)}
		}
	}
	if m.page == pageOutbounds {
		choices := m.outboundChoices()
		if m.outboundsAPI == nil || m.cursor >= len(choices) {
			return nil
		}
		choice := choices[m.cursor]
		return func() tea.Msg {
			if test {
				return actionResult{action: "test outbound", err: m.outboundsAPI.TestOutbounds(m.ctx, control.TestScope{OutboundID: choice.outbound.ID})}
			}
			if !choice.outbound.Selectable {
				return actionResult{action: "select outbound", err: errors.New("outbound is not selectable")}
			}
			return actionResult{action: "select outbound", err: m.outboundsAPI.SelectOutbound(m.ctx, choice.groupID, choice.outbound.ID)}
		}
	}
	return nil
}

func waitForDashboardUpdate(updates <-chan dashboardUpdate) tea.Cmd {
	if updates == nil {
		return nil
	}
	return func() tea.Msg {
		update := <-updates
		return update
	}
}

func streamDashboard(ctx context.Context, daemon control.Client, watcher control.Watcher, updates chan<- dashboardUpdate) {
	backoff := 200 * time.Millisecond
	var state client.State
	for {
		loaded, err := client.NewState(ctx, daemon)
		if err == nil {
			state = loaded
			update := dashboardUpdate{snapshot: state.Snapshot}
			if reader, ok := daemon.(control.ProfileReader); ok {
				update.profiles, _ = reader.ListProfiles(ctx)
			}
			if operator, ok := daemon.(control.OutboundOperator); ok {
				update.groups, _ = operator.ListOutboundGroups(ctx)
			}
			sendDashboardUpdate(ctx, updates, update)
			err = state.Watch(ctx, daemon, watcher)
		}
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			err = errors.New("daemon event stream ended")
		}
		sendDashboardUpdate(ctx, updates, dashboardUpdate{snapshot: state.Snapshot, err: err})
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
}

func sendDashboardUpdate(ctx context.Context, updates chan<- dashboardUpdate, update dashboardUpdate) {
	select {
	case updates <- update:
	case <-ctx.Done():
	}
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
