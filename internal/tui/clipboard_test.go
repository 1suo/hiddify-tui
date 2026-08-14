package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestClipboardCandidates(t *testing.T) {
	if got := clipboardCandidates("darwin"); len(got) != 1 || got[0][0] != "pbpaste" {
		t.Fatalf("darwin candidates = %#v", got)
	}
	windows := clipboardCandidates("windows")
	if len(windows) != 2 || windows[0][0] != "powershell.exe" || windows[1][0] != "pwsh.exe" {
		t.Fatalf("windows candidates = %#v", windows)
	}
	linux := clipboardCandidates("linux")
	if linux[0][0] != "wl-paste" || linux[len(linux)-1][0] != "powershell.exe" {
		t.Fatalf("linux candidates = %#v", linux)
	}
}

func TestLimitedWriterRejectsOversizedClipboard(t *testing.T) {
	writer := &limitedWriter{remaining: 4}
	if _, err := writer.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("5")); !errors.Is(err, errClipboardTooLarge) {
		t.Fatalf("oversized write error = %v", err)
	}
	if got := writer.builder.String(); got != "1234" {
		t.Fatalf("buffer = %q", got)
	}
}

func TestOpenAddModeDoesNotReadClipboard(t *testing.T) {
	model := newTestDashboard()
	model.pane = paneProfiles
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	if command != nil {
		t.Fatal("opening add mode must not read the clipboard automatically")
	}
	if !updated.(Dashboard).adding || strings.TrimSpace(updated.(Dashboard).input) != "" {
		t.Fatal("add mode was not opened with empty input")
	}
}

func TestPasteLimitPreservesExistingInput(t *testing.T) {
	model := newTestDashboard()
	model.adding = true
	model.input = "keep"
	updated, _ := model.Update(tea.PasteMsg{Content: strings.Repeat("x", maxClipboardBytes)})
	got := updated.(Dashboard)
	if got.input != "keep" {
		t.Fatalf("oversized paste changed input to %q", got.input)
	}
	if !strings.Contains(got.action, "exceeds 8 MiB") {
		t.Fatalf("oversized paste action = %q", got.action)
	}
}

func TestEmptyClipboardDoesNotEraseInput(t *testing.T) {
	model := newTestDashboard()
	model.adding = true
	model.input = "keep"
	updated, _ := model.Update(clipboardMsg{replace: true})
	if got := updated.(Dashboard).input; got != "keep" {
		t.Fatalf("failed clipboard read changed input to %q", got)
	}
}
