// Package tui holds the interactive terminal screens.
package tui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/core"
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
	core     client.Client
	store    *profile.Store
	launcher *core.Launcher
	address  string
	timeout  time.Duration
	err      error
	ctx      context.Context
	spawned  bool

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

	pending      string
	pendingUntil time.Time
	spinner      int

	updates      chan update
	streamCancel context.CancelFunc
}

type update struct {
	snapshot *client.Snapshot
	groups   []client.OutboundGroup
	logs     []client.LogEntry
	err      error
	tick     bool
}

type actionResult struct {
	action string
	err    error
}

// coreStarted carries the result of starting the core from the TUI.
type coreStarted struct {
	core    client.Client
	spawned bool
	err     error
}

type coreStopped struct{}

// clipboardMsg carries a read from the system clipboard into add-profile mode.
type clipboardMsg struct {
	content string
	replace bool
}

const maxLogLines = 500

const spinnerFrames = "|/-\\"

const spinnerInterval = 150 * time.Millisecond

const pendingTimeout = 15 * time.Second

const requestTimeout = 10 * time.Second

var pulseFrames = [...]string{".", "o", "O", "o"}

func NewDashboard(core client.Client, store *profile.Store, err error) Dashboard {
	return Dashboard{core: core, store: store, err: err, profiles: store.List()}
}

func (m Dashboard) Init() tea.Cmd {
	cmd := waitForStream(m.updates)
	if m.core == nil && m.store.AutoStart {
		return tea.Batch(cmd, m.startCore())
	}
	return cmd
}

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
				return m, m.readClipboardCmd(false)
			}
		case "d":
			if m.pane == paneProfiles {
				return m, m.deleteProfile()
			}
		case "c":
			if m.snapshot.State == client.StateStarted {
				if !m.confirmDisconnect {
					m.confirmDisconnect = true
					m.action = "press c again to disconnect"
					return m, nil
				}
				m.confirmDisconnect = false
				m.request("disconnect")
				return m, m.disconnect()
			}
			m.request("connect")
			return m, m.connect()
		case "r":
			m.request("restart")
			return m, m.restart()
		case "s":
			if m.core == nil {
				return m, m.startCore()
			}
			if m.spawned {
				return m, m.stopCore()
			}
			m.action = "core is external; start/stop it there"
		case "A":
			return m, m.toggleAutoStart()
		}
		m.confirmDisconnect = false
	case tea.PasteMsg:
		if m.adding {
			m.input += msg.Content
		}
	case clipboardMsg:
		if m.adding {
			if msg.replace || m.input == "" {
				m.input = msg.content
			}
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case update:
		if msg.tick {
			m.spinner++
			if m.pending != "" && time.Now().After(m.pendingUntil) {
				m.pending = ""
			}
			return m, waitForStream(m.updates)
		}
		if msg.err != nil {
			m.stopStream()
			m.core = nil
			m.spawned = false
			m.pending = ""
			m.snapshot = client.Snapshot{}
			m.groups = nil
			m.logs = nil
			m.action = "core stopped (press s to start)"
			return m, nil
		}
		if msg.snapshot != nil {
			m.snapshot = *msg.snapshot
			m.action = ""
			m.clearPending(msg.snapshot.State)
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
		if isConnectionAction(msg.action) {
			if msg.err != nil {
				m.pending = ""
				m.action = msg.action + ": " + simplifyError(msg.err)
			}
			// On success keep the pending marker until the core's state stream
			// confirms the new state, so the UI does not flash a stale state.
		} else if msg.err != nil {
			m.action = msg.action + ": " + simplifyError(msg.err)
		} else {
			m.action = msg.action
		}
		m.refreshProfiles()
	case coreStarted:
		if msg.err != nil {
			m.action = "start core: " + simplifyError(msg.err)
			return m, nil
		}
		m.core = msg.core
		m.spawned = msg.spawned
		m.err = nil
		m.action = "core started"
		return m, m.restartStream()
	case coreStopped:
		m.stopStream()
		m.core = nil
		m.spawned = false
		m.pending = ""
		m.snapshot = client.Snapshot{}
		m.groups = nil
		m.logs = nil
		m.action = "core stopped"
	}
	return m, nil
}

func (m *Dashboard) restartStream() tea.Cmd {
	m.stopStream()
	streamCtx, cancel := context.WithCancel(m.ctx)
	m.streamCancel = cancel
	m.updates = streamUpdates(streamCtx, m.core)
	return waitForStream(m.updates)
}

func (m *Dashboard) stopStream() {
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
	m.updates = nil
}

func (m Dashboard) startCore() tea.Cmd {
	return func() tea.Msg {
		coreClient, err := m.launcher.Start(m.ctx, m.address, core.BootstrapConfig, m.timeout)
		if err != nil {
			return coreStarted{err: err}
		}
		if active, ok := m.store.Active(); ok {
			_ = coreClient.Connect(m.ctx, active.Content, active.Name)
		}
		return coreStarted{core: coreClient, spawned: true}
	}
}

func (m Dashboard) stopCore() tea.Cmd {
	return func() tea.Msg {
		m.launcher.Stop()
		return coreStopped{}
	}
}

func (m Dashboard) toggleAutoStart() tea.Cmd {
	return func() tea.Msg {
		err := m.store.ToggleAutoStart()
		return actionResult{action: "auto-start", err: err}
	}
}

func (m Dashboard) inputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.adding = false
		m.input = ""
	case "enter":
		return m.submitAdd()
	case "ctrl+d":
		return m.submitAdd()
	case "ctrl+v":
		return m, m.readClipboardCmd(true)
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

// readClipboardCmd reads the system clipboard asynchronously. replace forces
// the result over the current input; otherwise it only fills an empty input.
func (m Dashboard) readClipboardCmd(replace bool) tea.Cmd {
	return func() tea.Msg {
		return clipboardMsg{content: readClipboard(), replace: replace}
	}
}

func (m Dashboard) submitAdd() (tea.Model, tea.Cmd) {
	m.adding = false
	content := strings.TrimSpace(m.input)
	m.input = ""
	if content == "" {
		return m, nil
	}
	return m, m.addProfile(content)
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
			if m.core != nil {
				if parseErr := m.core.Parse(m.ctx, content); parseErr == nil {
					m.store.Add(profile.Profile{Name: profile.DefaultName(content), Kind: profile.KindLocal, Content: content}, false)
					err = m.store.Save()
				} else {
					err = parseErr
				}
			} else {
				err = errors.New("core is not running (press s) to validate the profile")
			}
		}
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
			return actionResult{action: "activate profile", err: m.store.SetActive(id)}
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
		return actionResult{action: "delete profile", err: m.store.Delete(id)}
	}
}

func (m Dashboard) connect() tea.Cmd {
	if m.core == nil {
		return func() tea.Msg { return actionResult{action: "connect", err: errors.New("core is not running (press s)")} }
	}
	target, ok := m.store.Active()
	if !ok {
		return func() tea.Msg { return actionResult{action: "connect", err: errors.New("no active profile")} }
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, requestTimeout)
		defer cancel()
		return actionResult{action: "connect", err: m.core.Connect(ctx, target.Content, target.Name)}
	}
}

func (m Dashboard) disconnect() tea.Cmd {
	if m.core == nil {
		return func() tea.Msg { return actionResult{action: "disconnect", err: errors.New("core is not running")} }
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, requestTimeout)
		defer cancel()
		return actionResult{action: "disconnect", err: m.core.Disconnect(ctx)}
	}
}

func (m Dashboard) restart() tea.Cmd {
	if m.core == nil {
		return func() tea.Msg { return actionResult{action: "restart", err: errors.New("core is not running (press s)")} }
	}
	target, ok := m.store.Active()
	if !ok {
		return func() tea.Msg { return actionResult{action: "restart", err: errors.New("no active profile")} }
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, requestTimeout)
		defer cancel()
		return actionResult{action: "restart", err: m.core.Restart(ctx, target.Content, target.Name)}
	}
}

// request marks a connection action as in-flight until the core acknowledges it,
// a result arrives, or the pending timeout expires.
func (m *Dashboard) request(action string) {
	m.pending = action
	m.pendingUntil = time.Now().Add(pendingTimeout)
}

// refreshProfiles reloads the profile list from the store and clamps the cursor.
// Store mutations happen inside action commands on a value copy, so the model
// re-reads the store here to keep the active marker and list current.
func (m *Dashboard) refreshProfiles() {
	m.profiles = m.store.List()
	if m.profileCursor >= len(m.profiles) {
		m.profileCursor = len(m.profiles) - 1
	}
	if m.profileCursor < 0 {
		m.profileCursor = 0
	}
}

// isConnectionAction reports whether an action result is already reflected in
// the connection state display, so it should not be echoed into m.action.
func isConnectionAction(action string) bool {
	return action == "connect" || action == "disconnect" || action == "restart"
}

// clearPending drops the "requested" marker once the core has acknowledged the
// action by moving to a matching state.
func (m *Dashboard) clearPending(state client.ConnectionState) {
	switch m.pending {
	case "connect":
		if state == client.StateStarting || state == client.StateStarted {
			m.pending = ""
		}
	case "disconnect":
		if state == client.StateStopping || state == client.StateStopped {
			m.pending = ""
		}
	case "restart":
		if state == client.StateStarting || state == client.StateStarted {
			m.pending = ""
		}
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
func Run(core client.Client, store *profile.Store, launcher *core.Launcher, address string, timeout time.Duration) error {
	return RunWithOptions(core, store, launcher, address, timeout, false)
}

func RunWithOptions(core client.Client, store *profile.Store, launcher *core.Launcher, address string, timeout time.Duration, noColor bool) error {
	model := NewDashboard(core, store, nil)
	model.ctx = context.Background()
	model.launcher = launcher
	model.address = address
	model.timeout = timeout
	if core != nil {
		streamCtx, cancel := context.WithCancel(model.ctx)
		model.streamCancel = cancel
		model.updates = streamUpdates(streamCtx, core)
	}
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
		spinnerTicker := time.NewTicker(spinnerInterval)
		defer spinnerTicker.Stop()
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
			case <-spinnerTicker.C:
				updates <- update{tick: true}
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

// Layout thresholds. Below these the layout switches from a comfortable
// three-pane grid to a stacked, collapsed view where only the focused pane is
// expanded.
const (
	comfortableWidth = 72
	comfortableArea  = 12 // pane rows (height - status/footer) for the grid
	minimumWidth     = 40
	minimumHeight    = 10
)

// paneTitle returns the inscribed border title for a pane, carrying its
// selection number and the keys local to that pane so each panel is
// self-describing.
func (m Dashboard) paneTitle(p pane) string {
	switch p {
	case paneProfiles:
		return "profiles [1] enter:use a:add d:del"
	case paneOutbounds:
		return "outbounds [2] enter:use t:test"
	default:
		return "logs [3]"
	}
}

// paneBar renders a single-line collapsed pane: only the inscribed top border.
func paneBar(title string, width int, focused bool) string {
	border := lipgloss.NewStyle().Foreground(idleBorder)
	if focused {
		border = lipgloss.NewStyle().Foreground(activeBorder)
	}
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	return border.Render(paneTopLine(title, innerWidth))
}

// paneTopLine builds the top border line with the title inscribed.
func paneTopLine(title string, innerWidth int) string {
	titleText := truncate(" "+title+" ", innerWidth)
	fill := innerWidth - runeCount(titleText)
	if fill < 0 {
		fill = 0
	}
	return "+" + titleText + strings.Repeat("-", fill) + "+"
}

// renderPane draws a fixed-size ASCII-bordered pane with the title inscribed
// in the top border. The cursor line is highlighted; the pane is always
// exactly width x height.
func renderPane(title string, width, height int, focused bool, lines []string, cursor int) string {
	border := lipgloss.NewStyle().Foreground(idleBorder)
	if focused {
		border = lipgloss.NewStyle().Foreground(activeBorder)
	}

	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	innerHeight := height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	top := border.Render(paneTopLine(title, innerWidth))

	rows := make([]string, 0, innerHeight)
	for i, line := range lines {
		if len(rows) >= innerHeight {
			break
		}
		line = truncate(line, innerWidth)
		line += strings.Repeat(" ", innerWidth-runeCount(line))
		if i == cursor && focused {
			line = lipgloss.NewStyle().Reverse(true).Render(line)
		}
		rows = append(rows, border.Render("|")+line+border.Render("|"))
	}
	blank := strings.Repeat(" ", innerWidth)
	for len(rows) < innerHeight {
		rows = append(rows, border.Render("|")+blank+border.Render("|"))
	}
	bottom := border.Render("+" + strings.Repeat("-", innerWidth) + "+")

	return strings.Join(append([]string{top}, append(rows, bottom)...), "\n")
}

func runeCount(value string) int {
	return len([]rune(value))
}

func (m Dashboard) launcherAvailable() bool {
	return m.launcher != nil && m.launcher.Available()
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "~"
}

func (m Dashboard) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m Dashboard) render() string {
	if m.core == nil && !m.launcherAvailable() {
		return core.InstallHint + "\n\nrun `sudo packaging/linux/install.sh`, or set --core-binary"
	}

	width, height := m.width, m.height
	if width < minimumWidth {
		width = minimumWidth
	}
	if height < minimumHeight {
		height = minimumHeight
	}

	status := truncate(m.statusLine(), width)
	footer := truncate(m.footerLine(), width)

	profileLines, profileCursor := m.profileLines()
	outboundLines, outboundCursor := m.outboundLines()
	logLines := m.logLines()

	paneArea := height - 2
	var body string
	if width >= comfortableWidth && paneArea >= comfortableArea {
		body = m.renderGrid(width, paneArea, profileLines, profileCursor, outboundLines, outboundCursor, logLines)
	} else {
		body = m.renderStacked(width, paneArea, profileLines, profileCursor, outboundLines, outboundCursor, logLines)
	}

	return strings.Join([]string{status, body, footer}, "\n")
}

// renderGrid lays the three panes out with profiles and outbounds side by side
// and logs beneath, when there is room for it.
func (m Dashboard) renderGrid(width, paneArea int, profileLines []string, profileCursor int, outboundLines []string, outboundCursor int, logLines []string) string {
	logsHeight := paneArea * 2 / 5
	if logsHeight < 4 {
		logsHeight = 4
	}
	topHeight := paneArea - logsHeight
	if topHeight < 4 {
		topHeight = 4
		logsHeight = paneArea - topHeight
	}

	profilesWidth := width * 2 / 5
	if profilesWidth < 24 {
		profilesWidth = 24
	}
	if profilesWidth > width-24 {
		profilesWidth = width - 24
	}
	outboundsWidth := width - profilesWidth - 1

	profilesPane := renderPane(m.paneTitle(paneProfiles), profilesWidth, topHeight, m.pane == paneProfiles, profileLines, profileCursor)
	outboundsPane := renderPane(m.paneTitle(paneOutbounds), outboundsWidth, topHeight, m.pane == paneOutbounds, outboundLines, outboundCursor)
	logsPane := renderPane(m.paneTitle(paneLogs), width, logsHeight, m.pane == paneLogs, logLines, -1)

	top := lipgloss.JoinHorizontal(lipgloss.Top, profilesPane, outboundsPane)
	return strings.Join([]string{top, logsPane}, "\n")
}

// renderStacked collapses the two unfocused panes to single title bars and
// gives the focused pane the remaining vertical space.
func (m Dashboard) renderStacked(width, paneArea int, profileLines []string, profileCursor int, outboundLines []string, outboundCursor int, logLines []string) string {
	inactive := 2
	activeHeight := paneArea - inactive
	if activeHeight < 4 {
		activeHeight = 4
	}

	var profiles, outbounds, logs string
	switch m.pane {
	case paneProfiles:
		profiles = renderPane(m.paneTitle(paneProfiles), width, activeHeight, true, profileLines, profileCursor)
		outbounds = paneBar(m.paneTitle(paneOutbounds), width, false)
		logs = paneBar(m.paneTitle(paneLogs), width, false)
	case paneOutbounds:
		profiles = paneBar(m.paneTitle(paneProfiles), width, false)
		outbounds = renderPane(m.paneTitle(paneOutbounds), width, activeHeight, true, outboundLines, outboundCursor)
		logs = paneBar(m.paneTitle(paneLogs), width, false)
	default:
		profiles = paneBar(m.paneTitle(paneProfiles), width, false)
		outbounds = paneBar(m.paneTitle(paneOutbounds), width, false)
		logs = renderPane(m.paneTitle(paneLogs), width, activeHeight, true, logLines, -1)
	}

	return strings.Join([]string{profiles, outbounds, logs}, "\n")
}

func (m Dashboard) statusLine() string {
	if m.core == nil {
		line := "[c] " + m.connView()
		if m.err != nil {
			line += " | " + simplifyError(m.err)
		}
		if m.action != "" {
			line += " | " + m.action
		}
		return line
	}

	state := string(m.snapshot.State)
	if state == "" {
		state = "stopped"
	}
	line := "[c] " + m.connView()
	if state == "started" {
		profile := m.snapshot.CurrentProfile
		if profile == "" {
			profile = "none"
		}
		line += " (" + profile + ")"
		line += " | down " + formatBytes(m.snapshot.Downlink) + " up " + formatBytes(m.snapshot.Uplink)
		line += " | outbound " + valueOr(m.snapshot.CurrentOutbound, "none")
	}
	if m.err != nil {
		line += " | " + simplifyError(m.err)
	}
	if m.action != "" {
		line += " | " + m.action
	}
	return line
}

// connView renders the connection state, animating in-flight and connected
// states with an ASCII spinner or pulse.
func (m Dashboard) connView() string {
	spinner := string(spinnerFrames[m.spinner%len(spinnerFrames)])
	switch m.pending {
	case "connect":
		return spinner + " connecting"
	case "disconnect":
		return spinner + " disconnecting"
	case "restart":
		return spinner + " restarting"
	}
	switch m.snapshot.State {
	case client.StateStarting:
		return spinner + " connecting"
	case client.StateStopping:
		return spinner + " disconnecting"
	case client.StateStarted:
		return pulseFrames[m.spinner%len(pulseFrames)] + " connected"
	case client.StateStopped:
		if m.snapshot.Message != "" {
			return "! failed (" + m.snapshot.Message + ")"
		}
		return "- disconnected"
	default:
		return "- disconnected"
	}
}

// simplifyError strips gRPC transport boilerplate so the status line stays
// readable.
func simplifyError(err error) string {
	message := err.Error()
	re := regexp.MustCompile(`^rpc error: code = \w+ desc = `)
	message = re.ReplaceAllString(message, "")
	if idx := strings.Index(message, " desc = "); idx >= 0 {
		message = message[idx+len(" desc = "):]
	}
	return message
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
			return "add profile > paste | ctrl+v clipboard | enter confirm | esc cancel"
		}
		return "add profile > " + display + "  |  enter confirm"
	}
	corePart := "[s] core off"
	if m.core != nil {
		if m.spawned {
			corePart = "[s] core on"
		} else {
			corePart = "core on (external)"
		}
	}
	autostart := "off"
	if m.store.AutoStart {
		autostart = "on"
	}
	return corePart + " | [A] autostart " + autostart + " | [r] restart | [tab] next | [j/k] move | [q] quit"
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
