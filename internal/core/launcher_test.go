package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLauncherRefusesCustomAddress verifies that the fixed-address standalone
// core is never spawned when it could not serve the requested endpoint.
func TestLauncherRefusesCustomAddress(t *testing.T) {
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"run\" ]; then\n" +
		"  echo ok > \"" + started + "\"\n" +
		"  while :; do sleep 0.1; done\n" +
		"fi\n"
	binary := filepath.Join(dir, "fake-core")
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	launcher := NewLauncher(binary).SetStateDir(filepath.Join(dir, "state"))
	ctx := context.Background()
	core, err := launcher.Start(ctx, "127.0.0.1:17079", "", 1*time.Second)
	if err == nil {
		t.Fatal("expected dial to fail against a fake that serves no gRPC")
	}
	if core != nil {
		t.Fatal("Start should return a nil client when dial fails")
	}
	if _, err := os.Stat(started); !os.IsNotExist(err) {
		t.Fatalf("fake core must not be spawned for a custom address: %v", err)
	}
	if launcher.Spawned() {
		t.Fatal("refused Start must not spawn the core")
	}
}
