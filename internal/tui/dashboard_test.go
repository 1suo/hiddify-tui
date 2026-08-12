package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
)

func TestDashboardRendersUnavailableDaemon(t *testing.T) {
	view := NewDashboard(control.Snapshot{ConnectionState: control.ConnectionStopped}, errors.New("hiddify daemon is unavailable")).View().Content
	for _, want := range []string{"Hiddify", "Connection  stopped", "Daemon unavailable", "q / Ctrl+C"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not contain %q:\n%s", want, view)
		}
	}
}

func TestDashboardRendersLiveStatistics(t *testing.T) {
	view := NewDashboard(control.Snapshot{
		ConnectionState: control.ConnectionStarted,
		APIMajor:        1,
		APIMinor:        2,
		DaemonVersion:   "1.2.3",
		CoreVersion:     "4.5.6",
		RequestedMode:   "tun",
		EffectiveMode:   "tun",
		LastError:       "agent needs attention",
		Traffic:         control.TrafficStats{DownlinkBytesPerSecond: 2048, UplinkBytesPerSecond: 1024, TotalDownloadBytes: 3 * 1024 * 1024},
		System:          control.SystemStats{ConnectionCount: 7, MemoryBytes: 12 * 1024 * 1024},
		Agent:           control.AgentHealth{Required: true, Connected: true},
	}, nil).View().Content
	for _, want := range []string{"Down        2.0 KiB/s", "Connections 7", "Memory      12.0 MiB", "Agent       connected", "Requested   tun", "Daemon      1.2.3 (API 1.2)", "Core        4.5.6", "Last warning", "agent needs attention"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not contain %q:\n%s", want, view)
		}
	}
}

func TestDashboardFitsTypicalContentAt80By24(t *testing.T) {
	model := NewDashboard(control.Snapshot{APIMajor: 1, ConnectionState: control.ConnectionStopped, DaemonVersion: "1.0", CoreVersion: "4.0"}, nil)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := updated.(Dashboard).View().Content
	lines := strings.Split(view, "\n")
	if len(lines) > 24 {
		t.Fatalf("80x24 view has %d lines:\n%s", len(lines), view)
	}
	for _, line := range lines {
		if len([]rune(line)) > 80 {
			t.Fatalf("line wider than 80 columns: %q", line)
		}
	}
	for _, want := range []string{"1 Dash", "x x disconnect", "q/Ctrl+C quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("compact view missing %q:\n%s", want, view)
		}
	}
}

func TestDashboardQuitsWithoutConnectionAction(t *testing.T) {
	model, command := NewDashboard(control.Snapshot{}, nil).Update(tea.KeyPressMsg(tea.Key{Text: "q"}))
	if model == nil || command == nil {
		t.Fatal("q should return a quit command")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatalf("q command = %T, want tea.QuitMsg", command())
	}
}

func TestDashboardRequestsConnectionActions(t *testing.T) {
	operator := &recordingConnectionOperator{}
	model := NewDashboard(control.Snapshot{}, nil)
	model.connection = operator
	model.ctx = context.Background()

	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
	if result, ok := command().(actionResult); !ok || result.err != nil {
		t.Fatalf("connect result = %#v", command())
	}
	if operator.connects != 1 {
		t.Fatalf("connect requests = %d, want 1", operator.connects)
	}
	model = updated.(Dashboard)
	updated, command = model.Update(tea.KeyPressMsg(tea.Key{Text: "x"}))
	if command != nil {
		t.Fatal("first disconnect key must request confirmation")
	}
	updated, command = updated.(Dashboard).Update(tea.KeyPressMsg(tea.Key{Text: "x"}))
	if result, ok := command().(actionResult); !ok || result.err != nil {
		t.Fatalf("disconnect result = %#v", command())
	}
	if operator.disconnects != 1 {
		t.Fatalf("disconnect requests = %d, want 1", operator.disconnects)
	}
	model = updated.(Dashboard)
	_, command = model.Update(tea.KeyPressMsg(tea.Key{Text: "r"}))
	if result, ok := command().(actionResult); !ok || result.err != nil {
		t.Fatalf("restart result = %#v", command())
	}
	if operator.restarts != 1 {
		t.Fatalf("restart requests = %d, want 1", operator.restarts)
	}
}

func TestDashboardRequiresSecondDisconnectKey(t *testing.T) {
	operator := &recordingConnectionOperator{}
	model := NewDashboard(control.Snapshot{}, nil)
	model.connection, model.ctx = operator, context.Background()
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "x"}))
	if command != nil || operator.disconnects != 0 || !strings.Contains(updated.(Dashboard).View().Content, "Press x again") {
		t.Fatal("first x must only request confirmation")
	}
	_, command = updated.(Dashboard).Update(tea.KeyPressMsg(tea.Key{Text: "x"}))
	if result, ok := command().(actionResult); !ok || result.err != nil || operator.disconnects != 1 {
		t.Fatalf("confirmed disconnect = %#v", command())
	}
}

func TestDashboardActivatesProfileAndSelectsOutbound(t *testing.T) {
	profileAPI := &recordingProfileWriter{}
	outboundAPI := &recordingOutboundOperator{}
	model := NewDashboard(control.Snapshot{}, nil)
	model.ctx, model.profilesAPI, model.outboundsAPI = context.Background(), profileAPI, outboundAPI
	model.profiles = []control.Profile{{ID: "profile-1", Name: "Home"}}
	model.page = pageProfiles
	_, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter"}))
	if result, ok := command().(actionResult); !ok || result.err != nil || profileAPI.active != "profile-1" {
		t.Fatalf("activate = %#v", command())
	}
	model.page = pageOutbounds
	model.groups = []control.OutboundGroup{{ID: "group-1", Outbounds: []control.Outbound{{ID: "outbound-1", Selectable: true}}}}
	_, command = model.Update(tea.KeyPressMsg(tea.Key{Text: "enter"}))
	if result, ok := command().(actionResult); !ok || result.err != nil || outboundAPI.selected != "group-1/outbound-1" {
		t.Fatalf("select = %#v", command())
	}
}

type recordingProfileWriter struct{ active string }

func (r *recordingProfileWriter) AddRemoteProfile(context.Context, string, string, bool) (control.Profile, error) {
	panic("unused")
}
func (r *recordingProfileWriter) UpdateProfileName(context.Context, string, string) (control.Profile, error) {
	panic("unused")
}
func (r *recordingProfileWriter) RefreshProfile(context.Context, string) error { panic("unused") }
func (r *recordingProfileWriter) DeleteProfile(context.Context, string) error  { panic("unused") }
func (r *recordingProfileWriter) SetActiveProfile(_ context.Context, id string) error {
	r.active = id
	return nil
}

type recordingOutboundOperator struct{ selected string }

func (r *recordingOutboundOperator) ListOutboundGroups(context.Context) ([]control.OutboundGroup, error) {
	return nil, nil
}
func (r *recordingOutboundOperator) SelectOutbound(_ context.Context, group, outbound string) error {
	r.selected = group + "/" + outbound
	return nil
}
func (r *recordingOutboundOperator) TestOutbounds(context.Context, control.TestScope) error {
	return nil
}

type recordingConnectionOperator struct {
	connects    int
	disconnects int
	restarts    int
}

func (r *recordingConnectionOperator) Connect(context.Context, string, control.ConnectionMode) error {
	r.connects++
	return nil
}

func (r *recordingConnectionOperator) Disconnect(context.Context) error {
	r.disconnects++
	return nil
}

func (r *recordingConnectionOperator) Restart(context.Context) error {
	r.restarts++
	return nil
}

func TestDashboardAppliesLiveUpdate(t *testing.T) {
	updates := make(chan dashboardUpdate, 1)
	model := NewDashboard(control.Snapshot{}, nil)
	model.updates = updates
	updated, command := model.Update(dashboardUpdate{snapshot: control.Snapshot{ConnectionState: control.ConnectionStarted}})
	got := updated.(Dashboard)
	if got.snapshot.ConnectionState != control.ConnectionStarted || command == nil {
		t.Fatalf("live update = %#v, command %v", got, command)
	}
}

func TestDashboardNavigatesToProfiles(t *testing.T) {
	model := NewDashboard(control.Snapshot{}, nil)
	model.profiles = []control.Profile{{Name: "Home", Kind: control.ProfileRemote, Active: true, RedactedURL: "https://example.test/…"}}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "p"}))
	view := updated.(Dashboard).View().Content
	if !strings.Contains(view, "Profiles") || !strings.Contains(view, "Home") {
		t.Fatalf("profiles view:\n%s", view)
	}
}

func TestDashboardRendersAndConfirmsLogClear(t *testing.T) {
	reader := &recordingLogReader{}
	model := NewDashboard(control.Snapshot{}, nil)
	model.page, model.ctx, model.logsAPI = pageLogs, context.Background(), reader
	model.logs = []control.LogEntry{{TimestampUnix: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).UnixNano(), Level: control.LogWarn, Component: "core", Message: "redacted message"}}
	if view := model.View().Content; !strings.Contains(view, "warn  core") || !strings.Contains(view, "redacted message") {
		t.Fatalf("logs view: %s", view)
	}
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "C"}))
	if command != nil || reader.clears != 0 {
		t.Fatal("first C must only request confirmation")
	}
	_, command = updated.(Dashboard).Update(tea.KeyPressMsg(tea.Key{Text: "C"}))
	if result, ok := command().(actionResult); !ok || result.err != nil || reader.clears != 1 {
		t.Fatalf("clear result = %#v", command())
	}
}

func TestDashboardRendersRedactedSettings(t *testing.T) {
	model := NewDashboard(control.Snapshot{}, nil)
	model.page = pageSettings
	model.settings = control.Settings{RedactedJSON: []byte(`{"mode":"tun","token":"[redacted]"}`)}
	view := model.View().Content
	for _, want := range []string{"Settings (redacted)", `"mode": "tun"`, `"token": "[redacted]"`, "settings validate|set|import|export"} {
		if !strings.Contains(view, want) {
			t.Fatalf("settings view missing %q:\n%s", want, view)
		}
	}
}

func TestDashboardRendersServiceDiagnostics(t *testing.T) {
	model := NewDashboard(control.Snapshot{}, nil)
	model.page = pageService
	model.service = control.ServiceInfo{Installed: true, Enabled: true, Running: false, LastError: "failed to start"}
	model.diagnostics = control.Diagnostics{SocketPath: "/run/hiddify/control.sock", ActiveListeners: []string{"127.0.0.1:8080"}, DaemonVersion: "1.2.3", CoreVersion: "4.5.6", LastServiceError: "unit exited"}
	view := model.View().Content
	for _, want := range []string{"Installed   true", "Running     false", "/run/hiddify/control.sock", "127.0.0.1:8080", "failed to start", "unit exited"} {
		if !strings.Contains(view, want) {
			t.Fatalf("service view missing %q:\n%s", want, view)
		}
	}
}

type recordingLogReader struct{ clears int }

func (r *recordingLogReader) TailLogs(context.Context, uint32, control.LogLevel, bool) (<-chan control.LogEntry, error) {
	entries := make(chan control.LogEntry)
	close(entries)
	return entries, nil
}
func (r *recordingLogReader) ClearLogs(context.Context) error { r.clears++; return nil }

func TestStreamDashboardAppliesEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan dashboardUpdate, 4)
	daemon := client.FakeControl{
		Snapshot: control.Snapshot{APIMajor: 1, ConnectionState: control.ConnectionStopped},
		Events:   []control.Event{{Sequence: 1, Kind: control.EventProfile, ActiveProfileName: "Home"}},
	}
	go streamDashboard(ctx, daemon, daemon, updates)
	deadline := time.After(time.Second)
	for {
		select {
		case update := <-updates:
			if update.snapshot.ActiveProfileName == "Home" {
				return
			}
		case <-deadline:
			t.Fatal("did not receive event-applied dashboard update")
		}
	}
}
