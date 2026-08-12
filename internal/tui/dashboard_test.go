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

func TestDashboardQuitsWithoutConnectionAction(t *testing.T) {
	model, command := NewDashboard(control.Snapshot{}, nil).Update(tea.KeyPressMsg(tea.Key{Text: "q"}))
	if model == nil || command == nil {
		t.Fatal("q should return a quit command")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatalf("q command = %T, want tea.QuitMsg", command())
	}
}
