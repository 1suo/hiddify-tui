// Package tui holds the interactive terminal screens.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/profile"
)

type pane int

const (
	paneProfiles pane = iota
	paneOutbounds
	paneLogs
)

// Dashboard is the terminal screen. It is a client of the running core and the
// client-side profile store; quitting it never disconnects the core.
type Dashboard struct {
	core  client.Client
	store *profile.Store
	err   error
	ctx   context.Context

	snapshot client.Snapshot
	groups   []client.OutboundGroup
	logs     []client.LogEntry
	profiles []profile.Profile

	pane           pane
	profileCursor  int
	outboundCursor int

	width             int
	height            int
	confirmDisconnect bool
	action            string
	adding            bool
	input             string
}

type update struct {
	snapshot *client.Snapshot
	groups   []client.OutboundGroup
	logs     []client.LogEntry
	err      error
}

type actionResult struct {
	action string
	err    error
}

const maxLogLines = 500

func NewDashboard(core client.Client, store *profile.Store, err error) Dashboard {
	return Dashboard{core: core, store: store, err: err, profiles: store.List()}
}

func (m Dashboard) Init() tea.Cmd { return waitForUpdate(m.ctx, m.core) }

func (m Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.adding {
			return m.inputKey(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.pane = (m.pane + 1) % 3
		case "1":
			m.pane = paneProfiles
		case "2":
			m.pane = paneOutbounds
		case "3":
			m.pane = paneLogs
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter":
			return m, m.activate()
		case "t":
			return m, m.testOutbound()
		case "a":
			if m.pane == paneProfiles {
				m.adding = true
				m.input = ""
			}
		case "d":
			if m.pane == paneProfiles {
				return m, m.deleteProfile()
			}
		case "c":
			return m, m.connect()
		case "x":
			if !m.confirmDisconnect {
				m.confirmDisconnect = true
				m.action = "press x again to disconnect"
				return m, nil
			}
			m.confirmDisconnect = false
			return m, m.disconnect()
		case "r":
			return m, m.restart()
		}
		m.confirmDisconnect = false
	case tea.PasteMsg:
		if m.adding {
			m.input += msg.Content
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case update:
		if msg.err != nil && m.err == nil {
			m.err = msg.err
		}
		if msg.snapshot != nil {
			m.snapshot = *msg.snapshot
		}
		if msg.groups != nil {
			m.groups = msg.groups
		}
		if msg.logs != nil {
			m.logs = append(m.logs, msg.logs...)
			if len(m.logs) > maxLogLines {
				m.logs = m.logs[len(m.logs)-maxLogLines:]
			}
		}
		return m, waitForUpdate(m.ctx, m.core)
	case actionResult:
		if msg.err != nil {
			m.action = msg.action + ": " + msg.err.Error()
		} else {
			m.action = msg.action + " requested"
		}
	}
	return m, nil
}

func (m Dashboard) inputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.adding = false
		m.input = ""
	case "enter":
		m.adding = false
		content := strings.TrimSpace(m.input)
		m.input = ""
		if content == "" {
			return m, nil
		}
		return m, m.addProfile(content)
	case "backspace":
		runes := []rune(m.input)
		if len(runes) > 0 {
			m.input = string(runes[:len(runes)-1])
		}
	default:
		if msg.Text != "" {
			m.input += msg.Text
		}
	}
	return m, nil
}

func (m Dashboard) addProfile(content string) tea.Cmd {
	return func() tea.Msg {
		var err error
		if looksLikeURL(content) {
			remote, fetchErr := profile.FetchRemote(m.ctx, content)
			if fetchErr == nil {
				remote.URL = content
				m.store.Add(profile.Profile{Name: remote.Name, Kind: profile.KindRemote, URL: content, UpdateInterval: remote.UpdateInterval, Usage: remote.Usage, Content: remote.Content}, false)
				err = m.store.Save()
			} else {
				err = fetchErr
			}
		} else {
			if parseErr := m.core.Parse(m.ctx, content); parseErr == nil {
				m.store.Add(profile.Profile{Name: "Local", Kind: profile.KindLocal, Content: content}, false)
				err = m.store.Save()
			} else {
				err = parseErr
			}
		}
		m.profiles = m.store.List()
		return actionResult{action: "add profile", err: err}
	}
}

func (m Dashboard) activate() tea.Cmd {
	if m.pane == paneProfiles {
		if m.profileCursor >= len(m.profiles) {
			return nil
		}
		id := m.profiles[m.profileCursor].ID
		return func() tea.Msg {
			err := m.store.SetActive(id)
			m.profiles = m.store.List()
			return actionResult{action: "activate profile", err: err}
		}
	}
	if m.pane == paneOutbounds {
		choices := m.outboundChoices()
		if m.outboundCursor >= len(choices) {
			return nil
		}
		choice := choices[m.outboundCursor]
		return func() tea.Msg {
			return actionResult{action: "select outbound", err: m.core.SelectOutbound(m.ctx, choice.group, choice.tag)}
		}
	}
	return nil
}

func (m Dashboard) testOutbound() tea.Cmd {
	if m.pane != paneOutbounds {
		return nil
	}
	choices := m.outboundChoices()
	if m.outboundCursor >= len(choices) {
		return nil
	}
	tag := choices[m.outboundCursor].tag
	return func() tea.Msg {
		return actionResult{action: "test outbound", err: m.core.TestOutbound(m.ctx, tag)}
	}
}

func (m Dashboard) deleteProfile() tea.Cmd {
	if m.profileCursor >= len(m.profiles) {
		return nil
	}
	id := m.profiles[m.profileCursor].ID
	return func() tea.Msg {
		err := m.store.Delete(id)
		m.profiles = m.store.List()
		if m.profileCursor >= len(m.profiles) {
			m.profileCursor = len(m.profiles) - 1
		}
		if m.profileCursor < 0 {
			m.profileCursor = 0
		}
		return actionResult{action: "delete profile", err: err}
	}
}

func (m Dashboard) connect() tea.Cmd {
	target, ok := m.store.Active()
	if !ok {
		return func() tea.Msg { return actionResult{action: "connect", err: errors.New("no active profile")} }
	}
	return func() tea.Msg {
		return actionResult{action: "connect", err: m.core.Connect(m.ctx, target.Content, target.Name)}
	}
}

func (m Dashboard) disconnect() tea.Cmd {
	return func() tea.Msg { return actionResult{action: "disconnect", err: m.core.Disconnect(m.ctx)} }
}

func (m Dashboard) restart() tea.Cmd {
	target, ok := m.store.Active()
	if !ok {
		return func() tea.Msg { return actionResult{action: "restart", err: errors.New("no active profile")} }
	}
	return func() tea.Msg {
		return actionResult{action: "restart", err: m.core.Restart(m.ctx, target.Content, target.Name)}
	}
}

func (m *Dashboard) moveCursor(delta int) {
	limit := 0
	switch m.pane {
	case paneProfiles:
		limit = len(m.profiles)
	case paneOutbounds:
		limit = len(m.outboundChoices())
	}
	if limit == 0 {
		return
	}
	switch m.pane {
	case paneProfiles:
		m.profileCursor = (m.profileCursor + delta + limit) % limit
	case paneOutbounds:
		m.outboundCursor = (m.outboundCursor + delta + limit) % limit
	}
}

type outboundChoice struct {
	group string
	tag   string
}

func (m Dashboard) outboundChoices() []outboundChoice {
	var choices []outboundChoice
	for _, group := range m.groups {
		for _, item := range group.Items {
			choices = append(choices, outboundChoice{group: group.Tag, tag: item.Tag})
		}
	}
	return choices
}

func looksLikeURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// Run opens the alternate-screen dashboard.
func Run(core client.Client, store *profile.Store, err error) error {
	return RunWithOptions(core, store, err, false)
}

func RunWithOptions(core client.Client, store *profile.Store, err error, noColor bool) error {
	model := NewDashboard(core, store, err)
	model.ctx = context.Background()
	options := []tea.ProgramOption{}
	if noColor {
		options = append(options, tea.WithColorProfile(colorprofile.ASCII))
	}
	_, runErr := tea.NewProgram(model, options...).Run()
	if errors.Is(runErr, tea.ErrInterrupted) {
		return nil
	}
	return runErr
}

func waitForUpdate(ctx context.Context, core client.Client) tea.Cmd {
	if core == nil {
		return nil
	}
	return func() tea.Msg {
		statusCh, _ := core.WatchStatus(ctx)
		logCh, _ := core.WatchLogs(ctx, client.LogInfo)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case snapshot, ok := <-statusCh:
				if !ok {
					return update{err: errors.New("core status stream ended")}
				}
				return update{snapshot: &snapshot}
			case entry, ok := <-logCh:
				if ok {
					return update{logs: []client.LogEntry{entry}}
				}
			case <-ticker.C:
				groups, err := core.OutboundGroups(ctx)
				if err == nil {
					return update{groups: groups}
				}
			case <-ctx.Done():
				return update{err: errors.New("core unavailable")}
			}
		}
	}
}

var (
	activeBorder = lipgloss.Color("6")
	idleBorder   = lipgloss.Color("8")
)

func bordered(title string, width, height int, focused bool, content string) string {
	color := idleBorder
	if focused {
		color = activeBorder
	}
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1)
	body := strings.TrimRight(content, "\n")
	lines := strings.Split(body, "\n")
	if len(lines) > height-1 {
		lines = lines[:height-1]
	}
	return style.Render(title + "\n" + strings.Join(lines, "\n"))
}

func (m Dashboard) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m Dashboard) render() string {
	if m.core == nil {
		content := "hiddify core unavailable\n" + m.err.Error()
		if m.err == nil {
			content = "hiddify core unavailable"
		}
		return content
	}

	width := m.width
	height := m.height
	if width < 40 {
		width = 80
	}
	if height < 10 {
		height = 24
	}

	status := m.statusBar(width)

	logsHeight := height * 2 / 5
	topHeight := height - logsHeight - 3
	profilesWidth := width * 2 / 5
	outboundsWidth := width - profilesWidth - 1

	profilesPane := bordered("profiles", profilesWidth, topHeight, m.pane == paneProfiles, m.profilesView())
	outboundsPane := bordered("outbounds", outboundsWidth, topHeight, m.pane == paneOutbounds, m.outboundsView())
	logsPane := bordered("logs", width, logsHeight, m.pane == paneLogs, m.logsView())

	top := lipgloss.JoinHorizontal(lipgloss.Top, profilesPane, outboundsPane)

	footer := m.footer(width)
	return status + "\n" + top + "\n" + logsPane + "\n" + footer
}

func (m Dashboard) statusBar(width int) string {
	state := string(m.snapshot.State)
	if state == "" {
		state = "stopped"
	}
	profile := m.snapshot.CurrentProfile
	if profile == "" {
		profile = "none"
	}
	line := fmt.Sprintf("state %s  profile %s  ↓ %s  ↑ %s  outbound %s",
		state, profile, formatBytes(m.snapshot.Downlink), formatBytes(m.snapshot.Uplink), valueOr(m.snapshot.CurrentOutbound, "none"))
	if m.action != "" {
		line += "  ·  " + m.action
	}
	style := lipgloss.NewStyle().Width(width).Padding(0, 1).Bold(true)
	return style.Render(line)
}

func (m Dashboard) profilesView() string {
	if len(m.profiles) == 0 {
		return "no profiles\n\npress a to add (paste a URL or config)"
	}
	var builder strings.Builder
	for index, profile := range m.profiles {
		marker := "  "
		if index == m.profileCursor && m.pane == paneProfiles {
			marker = "> "
		}
		active := " "
		if profile.Active {
			active = "*"
		}
		line := fmt.Sprintf("%s%s %s", marker, active, profile.Name)
		if index == m.profileCursor && m.pane == paneProfiles {
			line = lipgloss.NewStyle().Reverse(true).Render(line)
		}
		builder.WriteString(line + "\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}

func (m Dashboard) outboundsView() string {
	if len(m.groups) == 0 {
		return "no outbounds"
	}
	var builder strings.Builder
	index := 0
	for _, group := range m.groups {
		builder.WriteString(group.Tag + " [" + group.Type + "]\n")
		for _, item := range group.Items {
			marker := "  "
			if index == m.outboundCursor && m.pane == paneOutbounds {
				marker = "> "
			}
			selected := " "
			if item.Selected {
				selected = "*"
			}
			delay := "-"
			if item.DelayMillis > 0 {
				delay = fmt.Sprintf("%dms", item.DelayMillis)
			}
			line := fmt.Sprintf("%s%s %s  %s  %s", marker, selected, item.Tag, item.Type, delay)
			if index == m.outboundCursor && m.pane == paneOutbounds {
				line = lipgloss.NewStyle().Reverse(true).Render(line)
			}
			builder.WriteString(line + "\n")
			index++
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

func (m Dashboard) logsView() string {
	if len(m.logs) == 0 {
		return "no log entries"
	}
	start := len(m.logs) - m.height/2
	if start < 0 {
		start = 0
	}
	var builder strings.Builder
	for _, entry := range m.logs[start:] {
		builder.WriteString(fmt.Sprintf("%s %-5s %-10s %s\n", entry.Time.Format("15:04:05"), entry.Level, entry.Component, entry.Message))
	}
	return strings.TrimRight(builder.String(), "\n")
}

func (m Dashboard) footer(width int) string {
	line := "c connect  x disconnect  r restart  tab pane  ↑/↓ move  enter select  a add  d delete  t test  q quit"
	if m.adding {
		line = "add profile: paste URL or config, enter to confirm, esc to cancel"
	}
	if m.width > 0 && len(line) > width {
		line = "c/x/r conn  tab pane  ↑↓ move  enter select  a add  d del  t test  q quit"
	}
	return line
}

func formatBytes(value int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	amount := float64(value)
	unit := 0
	for amount >= 1024 && unit < len(units)-1 {
		amount /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d%s", value, units[unit])
	}
	return fmt.Sprintf("%.1f%s", amount, units[unit])
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
