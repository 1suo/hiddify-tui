package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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
