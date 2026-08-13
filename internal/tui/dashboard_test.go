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
	for _, want := range []string{"profiles", "outbounds", "logs", "Home", "[c/x/r] conn"} {
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

func TestDashboardDisconnectRequiresConfirmation(t *testing.T) {
	model := newTestDashboard()
	core := model.core.(*client.FakeClient)
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "x"}))
	if command != nil || core.Disconnects != 0 {
		t.Fatal("first x must only request confirmation")
	}
	_, command = updated.(Dashboard).Update(tea.KeyPressMsg(tea.Key{Text: "x"}))
	if command == nil {
		t.Fatal("second x must disconnect")
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

func TestDashboardAddInputTreatsEnterAsNewline(t *testing.T) {
	model := newTestDashboard()
	model.adding = true
	model.input = "line1"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter"}))
	if cmd != nil {
		t.Fatal("enter must insert a newline, not submit")
	}
	m := updated.(Dashboard)
	if m.adding != true || m.input != "line1\n" {
		t.Fatalf("input after enter = %q, adding=%t", m.input, m.adding)
	}

	// ctrl+d submits
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("ctrl+d must submit")
	}
	if updated.(Dashboard).adding {
		t.Fatal("submit should leave add mode")
	}
	if result := cmd().(actionResult); result.err != nil {
		// "line1" is not a valid config; addProfile reports the parse error.
		t.Logf("submit result (expected error for invalid config): %v", result.err)
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
	if !strings.Contains(view, "core off") || strings.Contains(view, "rpc error") {
		t.Fatalf("unavailable status:\n%s", view)
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
