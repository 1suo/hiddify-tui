package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLauncherSpawnsAndStops verifies the process lifecycle with a fake core
// binary that blocks until signalled. The dial always fails because the fake
// serves no gRPC, but the spawn and stop must still work.
func TestLauncherSpawnsAndStops(t *testing.T) {
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	stopped := filepath.Join(dir, "stopped")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"run\" ]; then\n" +
		"  echo ok > \"" + started + "\"\n" +
		"  trap 'echo ok > \"" + stopped + "\"; exit 0' INT TERM\n" +
		"  while :; do sleep 0.1; done\n" +
		"fi\n"
	binary := filepath.Join(dir, "fake-core")
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	launcher := NewLauncher(binary)
	ctx := context.Background()
	core, spawned, err := launcher.Ensure(ctx, "127.0.0.1:1", 1*time.Second)
	if err == nil {
		t.Fatal("expected dial to fail against a fake that serves no gRPC")
	}
	if !spawned {
		t.Fatal("Ensure should report that it spawned the core")
	}
	if core != nil {
		t.Fatal("Ensure should return a nil client when dial fails")
	}
	if _, err := os.Stat(started); err != nil {
		t.Fatalf("fake core was not spawned: %v", err)
	}

	launcher.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(stopped); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("Stop did not terminate the fake core")
}
