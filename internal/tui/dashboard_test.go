package tui

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/1suo/hiddify-tui/internal/client"
	corepkg "github.com/1suo/hiddify-tui/internal/core"
	"github.com/1suo/hiddify-tui/internal/profile"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

func newTestDashboard() Dashboard {
	store, _ := profile.Open("/tmp/hiddify-tui-test-profiles.json")
	store.Profiles = []profile.Profile{{ID: "p1", Name: "Home", Kind: profile.KindLocal, Content: `{"inbounds":[]}`}}
	store.ActiveID = "p1"
	core := &client.FakeClient{}
	model := NewDashboard(core, store, nil)
	model.ctx = context.Background()
	model.launcher = corepkg.NewLauncher("/bin/true")
	return model
}

func TestDashboardRendersPanes(t *testing.T) {
	model := newTestDashboard()
	model.width, model.height = 100, 30
	view := model.render()
	for _, want := range []string{"profiles", "outbounds", "logs", "Home", "[c] ", "core on"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not contain %q:\n%s", want, view)
		}
	}
}

func TestDashboardQuitsWithoutDisconnect(t *testing.T) {
	model := newTestDashboard()
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "q"}))
	if updated == nil || command == nil {
		t.Fatal("q should return a quit command")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatalf("q command = %T, want tea.QuitMsg", command())
	}
	if model.core.(*client.FakeClient).Disconnects != 0 {
		t.Fatal("quitting the TUI must not disconnect")
	}
}

func TestDashboardActivatesProfile(t *testing.T) {
	model := newTestDashboard()
	store := model.store
	store.Profiles = []profile.Profile{
		{ID: "a", Name: "A", Kind: profile.KindLocal},
		{ID: "b", Name: "B", Kind: profile.KindLocal},
	}
	model.profiles = store.List()
	model.pane = paneProfiles
	model.profileCursor = 1
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter"}))
	if command == nil {
		t.Fatal("enter should activate the selected profile")
	}
	if result := command().(actionResult); result.err != nil {
		t.Fatalf("activate result = %v", result.err)
	}
	_ = updated
	if store.ActiveID != "b" {
		t.Fatalf("active profile = %q, want b", store.ActiveID)
	}
}

func TestActivateProfileRefreshesList(t *testing.T) {
	model := newTestDashboard()
	store := model.store
	store.Profiles = []profile.Profile{
		{ID: "a", Name: "A", Kind: profile.KindLocal},
		{ID: "b", Name: "B", Kind: profile.KindLocal},
	}
	store.ActiveID = "a"
	model.profiles = store.List()
	model.pane = paneProfiles
	model.profileCursor = 1

	_, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter"}))
	result := command().(actionResult)
	settled, _ := model.Update(result)

	active := ""
	for _, p := range settled.(Dashboard).profiles {
		if p.Active {
			active = p.ID
		}
	}
	if active != "b" {
		t.Fatalf("active profile after activation = %q, want b", active)
	}
}

func TestDashboardDisconnectRequiresConfirmation(t *testing.T) {
	model := newTestDashboard()
	model.snapshot = client.Snapshot{State: client.StateStarted}
	core := model.core.(*client.FakeClient)
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
	if command != nil || core.Disconnects != 0 {
		t.Fatal("first c must only request confirmation")
	}
	_, command = updated.(Dashboard).Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
	if command == nil {
		t.Fatal("second c must disconnect")
	}
	if result := command().(actionResult); result.err != nil {
		t.Fatalf("disconnect result = %v", result.err)
	}
	if core.Disconnects != 1 {
		t.Fatalf("disconnects = %d, want 1", core.Disconnects)
	}
}

func TestProgramQuitsOnQ(t *testing.T) {
	core := &client.FakeClient{StatusEvents: []client.Snapshot{{State: client.StateStarted}}}
	store, _ := profile.Open("/tmp/hiddify-tui-test-profiles.json")
	model := NewDashboard(core, store, nil)
	model.ctx = context.Background()
	model.updates = streamUpdates(model.ctx, core)

	_, err := tea.NewProgram(model,
		tea.WithInput(strings.NewReader("q")),
		tea.WithOutput(io.Discard),
		tea.WithEnvironment([]string{"TERM=xterm-256color"}),
	).Run()
	if err != nil {
		t.Fatalf("quit run returned error: %v", err)
	}
}

func TestDashboardAddEnterSubmits(t *testing.T) {
	model := newTestDashboard()
	model.adding = true
	model.input = `{"outbounds":[{"tag":"x"}]}`

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter"}))
	if cmd == nil {
		t.Fatal("enter must submit the pasted input")
	}
	if updated.(Dashboard).adding {
		t.Fatal("submit should leave add mode")
	}
	if result := cmd().(actionResult); result.err != nil {
		t.Fatalf("submit result = %v", result.err)
	}
}

func TestDashboardPasteFromClipboard(t *testing.T) {
	model := newTestDashboard()
	model.adding = true

	settled, _ := model.Update(clipboardMsg{content: "  https://sub.example.com/\n", replace: true})
	if settled.(Dashboard).input != "  https://sub.example.com/\n" {
		t.Fatalf("clipboard paste = %q", settled.(Dashboard).input)
	}
	// Auto-fill must not overwrite non-empty input.
	settled, _ = settled.(Dashboard).Update(clipboardMsg{content: "other", replace: false})
	if settled.(Dashboard).input == "other" {
		t.Fatal("non-replace paste must not overwrite existing input")
	}
}

func TestConnectionPendingClearsOnState(t *testing.T) {
	model := newTestDashboard()
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
	if command == nil {
		t.Fatal("c must issue a connect command")
	}
	if updated.(Dashboard).pending != "connect" {
		t.Fatal("pending should be set to connect after pressing c")
	}
	result := command().(actionResult)
	settled, _ := updated.(Dashboard).Update(result)
	if settled.(Dashboard).pending != "connect" {
		t.Fatalf("pending = %q, want kept until the core confirms the state", settled.(Dashboard).pending)
	}
	snapshot := client.Snapshot{State: client.StateStarted}
	confirmed, _ := settled.(Dashboard).Update(update{snapshot: &snapshot})
	if confirmed.(Dashboard).pending != "" {
		t.Fatalf("pending = %q, want cleared once the started state arrives", confirmed.(Dashboard).pending)
	}
}

func TestConnectWithoutCoreIsSafe(t *testing.T) {
	model := newTestDashboard()
	model.core = nil
	_, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
	if command == nil {
		t.Fatal("c without a core must return a command, not panic")
	}
	if result := command().(actionResult); result.err == nil {
		t.Fatal("connect without a core should report an error")
	}
}

func TestSimplifyError(t *testing.T) {
	cases := map[string]string{
		"rpc error: code = Unavailable desc = connection error: desc = transport: connection refused": "transport: connection refused",
		"rpc error: code = InvalidArgument desc = profile content is invalid":                         "profile content is invalid",
		"plain error": "plain error",
	}
	for in, want := range cases {
		if got := simplifyError(errors.New(in)); got != want {
			t.Errorf("simplifyError(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDashboardStatusUnavailable(t *testing.T) {
	model := newTestDashboard()
	model.core = nil
	model.err = errors.New("rpc error: code = Unavailable desc = connection error: desc = refused")
	view := model.render()
	if !strings.Contains(view, "disconnected") || !strings.Contains(view, "[s] core off") || strings.Contains(view, "rpc error") {
		t.Fatalf("unavailable status:\n%s", view)
	}
}

func TestDashboardCollapsesOnSmallScreen(t *testing.T) {
	model := newTestDashboard()
	model.pane = paneProfiles
	model.profiles = []profile.Profile{{ID: "a", Name: "A", Kind: profile.KindLocal}}
	model.width, model.height = 40, 10
	view := ansi.ReplaceAllString(model.render(), "")
	lines := strings.Split(view, "\n")
	if len(lines) > 10 {
		t.Fatalf("small screen produced %d lines:\n%s", len(lines), view)
	}
	for _, want := range []string{"profiles [1]", "outbounds [2]", "logs [3]"} {
		if !strings.Contains(view, want) {
			t.Errorf("collapsed layout missing %q:\n%s", want, view)
		}
	}
}

func TestDashboardFitsScreen(t *testing.T) {
	core := &client.FakeClient{}
	store, _ := profile.Open("/tmp/hiddify-tui-test-profiles.json")
	store.Profiles = []profile.Profile{
		{ID: "a", Name: "a very long profile name that should be truncated", Kind: profile.KindLocal},
		{ID: "b", Name: "B", Kind: profile.KindLocal},
	}
	store.ActiveID = "a"
	model := NewDashboard(core, store, nil)
	model.groups = []client.OutboundGroup{{
		Tag: "selector", Type: "selector", Items: []client.Outbound{
			{Tag: "vless § 0 with a long tag", Type: "VLESS", DelayMillis: 80, Selected: true},
		},
	}}
	model.logs = []client.LogEntry{{Level: client.LogInfo, Component: "core", Message: "a long log message that should be truncated to fit the pane width"}}

	for _, size := range [][2]int{{80, 24}, {100, 30}, {120, 40}, {60, 20}} {
		model.width, model.height = size[0], size[1]
		rendered := ansi.ReplaceAllString(model.render(), "")
		lines := strings.Split(rendered, "\n")
		if len(lines) > size[1] {
			t.Fatalf("render at %dx%d produced %d lines:\n%s", size[0], size[1], len(lines), rendered)
		}
		for i, line := range lines {
			if n := len([]rune(line)); n > size[0] {
				t.Fatalf("render at %dx%d line %d is %d wide: %q", size[0], size[1], i, n, line)
			}
		}
	}
}
