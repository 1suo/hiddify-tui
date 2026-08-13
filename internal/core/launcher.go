// Package core launches and stops a headless hiddify-core process so the TUI
// can own the full lifecycle without a separate terminal.
package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/1suo/hiddify-tui/internal/client"
)

// bootstrapConfig is a minimal config with a non-predefined outbound so the
// core can start and serve its gRPC API without a real profile. The TUI then
// switches to the active profile via Connect. A bare "direct" outbound panics
// the core config builder, which filters predefined tags and then indexes an
// empty list.
const bootstrapConfig = `{"outbounds":[{"type":"vless","tag":"bootstrap","server":"127.0.0.1","server_port":1,"uuid":"00000000-0000-0000-0000-000000000000"}]}`

// Launcher owns a headless core process when the client needs to start one.
type Launcher struct {
	binary string
	cmd    *exec.Cmd
}

// NewLauncher resolves the core binary, falling back to PATH lookup.
func NewLauncher(binary string) *Launcher {
	if binary == "" {
		if path, err := exec.LookPath("hiddify-core"); err == nil {
			binary = path
		}
	}
	return &Launcher{binary: binary}
}

// Ensure returns a client for a running core, launching one headless if none is
// reachable. The bool reports whether this call launched the core.
func (l *Launcher) Ensure(ctx context.Context, address string, timeout time.Duration) (*client.GRPCClient, bool, error) {
	if core, err := dial(ctx, address, 500*time.Millisecond); err == nil {
		return core, false, nil
	}
	if l.binary == "" {
		return nil, false, fmt.Errorf("core not reachable and hiddify-core binary not found")
	}
	configPath := filepath.Join(os.TempDir(), "hiddify-tui-bootstrap.json")
	if err := os.WriteFile(configPath, []byte(bootstrapConfig), 0o600); err != nil {
		return nil, false, err
	}
	l.cmd = exec.Command(l.binary, "run", "-c", configPath)
	l.cmd.Stdout = os.Stderr
	l.cmd.Stderr = os.Stderr
	if err := l.cmd.Start(); err != nil {
		return nil, false, fmt.Errorf("launch core: %w", err)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if core, err := dial(ctx, address, 500*time.Millisecond); err == nil {
			return core, true, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, true, fmt.Errorf("core did not become reachable")
}

// Stop terminates the launched core, if any.
func (l *Launcher) Stop() {
	if l.cmd != nil && l.cmd.Process != nil {
		_ = l.cmd.Process.Signal(os.Interrupt)
		_ = l.cmd.Wait()
		l.cmd = nil
	}
}

func dial(ctx context.Context, address string, timeout time.Duration) (*client.GRPCClient, error) {
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return client.Dial(dialCtx, address)
}
