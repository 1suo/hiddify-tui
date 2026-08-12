package main

import (
	"bytes"
	"testing"

	"github.com/1suo/hiddify-tui/internal/cli"
)

func TestRunWithoutSubcommandIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run(nil, &stdout, &stderr); got != cli.ExitUsage {
		t.Fatalf("run() exit code = %d, want %d", got, cli.ExitUsage)
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"--version"}, &stdout, &stderr); got != cli.ExitOK {
		t.Fatalf("run() exit code = %d, want %d", got, cli.ExitOK)
	}
	if stdout.String() != version+"\n" {
		t.Fatalf("version output = %q", stdout.String())
	}
}
