package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/profile"
)

func newTestDashboard() Dashboard {
	store, _ := profile.Open("/tmp/hiddify-tui-test-profiles.json")
	store.Profiles = []profile.Profile{{ID: "p1", Name: "Home", Kind: profile.KindLocal, Content: `{"inbounds":[]}`}}
	store.ActiveID = "p1"
	core := &client.FakeClient{}
	model := NewDashboard(core, store, nil)
	model.ctx = context.Background()
	return model
}

func TestDashboardRendersPanes(t *testing.T) {
	model := newTestDashboard()
	model.width, model.height = 100, 30
	view := model.render()
	for _, want := range []string{"profiles", "outbounds", "logs", "Home", "c connect"} {
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
