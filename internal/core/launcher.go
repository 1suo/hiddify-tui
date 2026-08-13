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

// BootstrapConfig is a minimal config the core can start and serve from: a
// "direct" outbound (required by the core's DNS detour) plus a non-predefined
// outbound (required by the selector builder). The TUI switches to the real
// profile via Connect afterward.
const BootstrapConfig = `{"outbounds":[{"type":"direct","tag":"direct"},{"type":"vless","tag":"bootstrap","server":"127.0.0.1","server_port":1,"uuid":"00000000-0000-0000-0000-000000000000"}]}`

// Launcher owns a headless core process when the client needs to start one.
type Launcher struct {
	binary string
	cmd    *exec.Cmd
}

// NewLauncher resolves the core binary, falling back to PATH and common
// install locations.
func NewLauncher(binary string) *Launcher {
	if binary == "" {
		binary = findBinary()
	}
	return &Launcher{binary: binary}
}

// InstallHint is the guidance shown when no core binary can be found.
const InstallHint = "install hiddify-core: github.com/hiddify/hiddify-core"

func findBinary() string {
	if path, err := exec.LookPath("hiddify-core"); err == nil {
		return path
	}
	for _, candidate := range []string{
		"/usr/lib/hiddify/hiddify-core",
		"/usr/local/lib/hiddify/hiddify-core",
		"/usr/local/bin/hiddify-core",
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// Available reports whether a core binary was found.
func (l *Launcher) Available() bool { return l.binary != "" }

// Spawned reports whether this launcher started the core.
func (l *Launcher) Spawned() bool { return l.cmd != nil && l.cmd.Process != nil }

// Start launches the core headless and returns a client once it is reachable.
func (l *Launcher) Start(ctx context.Context, address, configContent string, timeout time.Duration) (*client.GRPCClient, error) {
	if l.binary == "" {
		return nil, fmt.Errorf("hiddify-core binary not found")
	}
	if configContent == "" {
		configContent = BootstrapConfig
	}
	configPath := filepath.Join(os.TempDir(), "hiddify-tui-bootstrap.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return nil, err
	}
	l.cmd = exec.Command(l.binary, "run", "-c", configPath)
	l.cmd.Stdout = os.Stderr
	l.cmd.Stderr = os.Stderr
	if err := l.cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch core: %w", err)
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if core, err := dial(ctx, address, 500*time.Millisecond); err == nil {
			return core, nil
		} else {
			lastErr = err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("core did not become reachable: %w", lastErr)
}

// Stop terminates the launched core, if any.
func (l *Launcher) Stop() {
	if l.cmd != nil && l.cmd.Process != nil {
		_ = l.cmd.Process.Signal(os.Interrupt)
		_ = l.cmd.Wait()
		l.cmd = nil
	}
}

// Dial returns a client for an already-running core.
func Dial(ctx context.Context, address string, timeout time.Duration) (*client.GRPCClient, error) {
	return dial(ctx, address, timeout)
}

func dial(ctx context.Context, address string, timeout time.Duration) (*client.GRPCClient, error) {
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return client.Dial(dialCtx, address)
}
