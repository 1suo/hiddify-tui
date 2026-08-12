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
		Traffic:         control.TrafficStats{DownlinkBytesPerSecond: 2048, UplinkBytesPerSecond: 1024, TotalDownloadBytes: 3 * 1024 * 1024},
		System:          control.SystemStats{ConnectionCount: 7, MemoryBytes: 12 * 1024 * 1024},
		Agent:           control.AgentHealth{Required: true, Connected: true},
	}, nil).View().Content
	for _, want := range []string{"Down        2.0 KiB/s", "Connections 7", "Memory      12.0 MiB", "Agent       connected"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not contain %q:\n%s", want, view)
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
	if result, ok := command().(connectionActionResult); !ok || result.err != nil {
		t.Fatalf("connect result = %#v", command())
	}
	if operator.connects != 1 {
		t.Fatalf("connect requests = %d, want 1", operator.connects)
	}
	model = updated.(Dashboard)
	updated, command = model.Update(tea.KeyPressMsg(tea.Key{Text: "x"}))
	if result, ok := command().(connectionActionResult); !ok || result.err != nil {
		t.Fatalf("disconnect result = %#v", command())
	}
	if operator.disconnects != 1 {
		t.Fatalf("disconnect requests = %d, want 1", operator.disconnects)
	}
	model = updated.(Dashboard)
	_, command = model.Update(tea.KeyPressMsg(tea.Key{Text: "r"}))
	if result, ok := command().(connectionActionResult); !ok || result.err != nil {
		t.Fatalf("restart result = %#v", command())
	}
	if operator.restarts != 1 {
		t.Fatalf("restart requests = %d, want 1", operator.restarts)
	}
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
