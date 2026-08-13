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

	updates chan update
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

func (m Dashboard) Init() tea.Cmd { return waitForStream(m.updates) }

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
		return m, waitForStream(m.updates)
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
	model.updates = streamUpdates(model.ctx, core)
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

// streamUpdates owns the long-lived status/log streams and outbound polling for
// the lifetime of the program. Subscribing once avoids re-dialing gRPC streams
// on every update, which would starve the input loop.
func streamUpdates(ctx context.Context, core client.Client) chan update {
	updates := make(chan update, 16)
	if core == nil {
		close(updates)
		return updates
	}
	go func() {
		defer close(updates)
		statusCh, err := core.WatchStatus(ctx)
		if err != nil {
			updates <- update{err: err}
			return
		}
		logCh, err := core.WatchLogs(ctx, client.LogInfo)
		if err != nil {
			logCh = nil
		}
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case snapshot, ok := <-statusCh:
				if !ok {
					updates <- update{err: errors.New("core status stream ended")}
					return
				}
				updates <- update{snapshot: &snapshot}
			case entry, ok := <-logCh:
				if ok {
					updates <- update{logs: []client.LogEntry{entry}}
				}
			case <-ticker.C:
				if groups, err := core.OutboundGroups(ctx); err == nil {
					updates <- update{groups: groups}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return updates
}

func waitForStream(updates chan update) tea.Cmd {
	if updates == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-updates
		if !ok {
			return update{err: errors.New("core unavailable")}
		}
		return msg
	}
}

var (
	activeBorder = lipgloss.Color("6")
	idleBorder   = lipgloss.Color("8")
)

// renderPane draws a fixed-size bordered pane. Each body line is truncated to
// the inner width and the cursor line is highlighted; the pane is always
// exactly width x height.
func renderPane(title string, width, height int, focused bool, lines []string, cursor int) string {
	color := idleBorder
	if focused {
		color = activeBorder
	}
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	innerHeight := height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}
	body := make([]string, 0, innerHeight)
	body = append(body, truncate(title, innerWidth))
	for i, line := range lines {
		if len(body) >= innerHeight {
			break
		}
		line = truncate(line, innerWidth)
		if i == cursor && focused {
			line = lipgloss.NewStyle().Reverse(true).Render(line)
		}
		body = append(body, line)
	}
	for len(body) < innerHeight {
		body = append(body, "")
	}
	style := lipgloss.NewStyle().
		Width(width - 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color)
	return style.Render(strings.Join(body, "\n"))
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func (m Dashboard) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m Dashboard) render() string {
	if m.core == nil {
		if m.err != nil {
			return "hiddify core unavailable\n" + m.err.Error()
		}
		return "hiddify core unavailable"
	}

	width, height := m.width, m.height
	if width < 60 {
		width = 80
	}
	if height < 12 {
		height = 24
	}

	status := truncate(m.statusLine(), width)
	footer := truncate(m.footerLine(), width)

	paneHeight := height - 2
	logsHeight := paneHeight * 2 / 5
	if logsHeight < 4 {
		logsHeight = 4
	}
	topHeight := paneHeight - logsHeight
	if topHeight < 4 {
		topHeight = 4
		logsHeight = paneHeight - topHeight
	}

	profilesWidth := width * 2 / 5
	if profilesWidth < 24 {
		profilesWidth = 24
	}
	if profilesWidth > width-24 {
		profilesWidth = width - 24
	}
	outboundsWidth := width - profilesWidth - 1

	profileLines, profileCursor := m.profileLines()
	outboundLines, outboundCursor := m.outboundLines()
	logLines := m.logLines()

	profilesPane := renderPane("profiles", profilesWidth, topHeight, m.pane == paneProfiles, profileLines, profileCursor)
	outboundsPane := renderPane("outbounds", outboundsWidth, topHeight, m.pane == paneOutbounds, outboundLines, outboundCursor)
	logsPane := renderPane("logs", width, logsHeight, m.pane == paneLogs, logLines, -1)

	top := lipgloss.JoinHorizontal(lipgloss.Top, profilesPane, outboundsPane)
	return strings.Join([]string{status, top, logsPane, footer}, "\n")
}

func (m Dashboard) statusLine() string {
	state := string(m.snapshot.State)
	if state == "" {
		state = "stopped"
	}
	profile := m.snapshot.CurrentProfile
	if profile == "" {
		profile = "none"
	}
	line := fmt.Sprintf("state %s  running %s  ↓ %s  ↑ %s  outbound %s",
		state, profile, formatBytes(m.snapshot.Downlink), formatBytes(m.snapshot.Uplink), valueOr(m.snapshot.CurrentOutbound, "none"))
	if m.action != "" {
		line += "  ·  " + m.action
	}
	return line
}

func (m Dashboard) profileLines() ([]string, int) {
	if len(m.profiles) == 0 {
		lines := []string{"no profiles", "", "a add (paste URL or config)", "migrate gui to import from the GUI"}
		return lines, -1
	}
	lines := make([]string, 0, len(m.profiles))
	for _, profile := range m.profiles {
		active := " "
		if profile.Active {
			active = "*"
		}
		lines = append(lines, fmt.Sprintf("%s %s", active, profile.Name))
	}
	return lines, m.profileCursor
}

func (m Dashboard) outboundLines() ([]string, int) {
	if len(m.groups) == 0 {
		return []string{"no outbounds"}, -1
	}
	lines := make([]string, 0)
	cursor := -1
	index := 0
	for _, group := range m.groups {
		lines = append(lines, group.Tag+" ["+group.Type+"]")
		for _, item := range group.Items {
			selected := " "
			if item.Selected {
				selected = "*"
			}
			delay := "-"
			if item.DelayMillis > 0 {
				delay = fmt.Sprintf("%dms", item.DelayMillis)
			}
			if index == m.outboundCursor {
				cursor = len(lines)
			}
			lines = append(lines, fmt.Sprintf("%s %s  %s  %s", selected, item.Tag, item.Type, delay))
			index++
		}
	}
	return lines, cursor
}

func (m Dashboard) logLines() []string {
	if len(m.logs) == 0 {
		return []string{"no log entries"}
	}
	start := len(m.logs) - m.height/2
	if start < 0 {
		start = 0
	}
	lines := make([]string, 0, len(m.logs)-start)
	for _, entry := range m.logs[start:] {
		lines = append(lines, fmt.Sprintf("%s %-5s %-10s %s", entry.Time.Format("15:04:05"), entry.Level, entry.Component, entry.Message))
	}
	return lines
}

func (m Dashboard) footerLine() string {
	if m.adding {
		display := strings.ReplaceAll(m.input, "\n", " ")
		if display == "" {
			return "add profile › paste URL or config, enter to confirm, esc to cancel"
		}
		return "add profile › " + display
	}
	return "tab pane · ↑↓ move · enter select · c/x/r conn · a add · d del · q quit"
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
