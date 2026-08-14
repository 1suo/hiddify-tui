// Package core launches and stops a headless hiddify-core process so the TUI
// can own the full lifecycle without a separate terminal.
package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/1suo/hiddify-tui/internal/client"
)

// ErrAddressInUse means another core or application already owns the control
// address. Callers must leave that process untouched.
var ErrAddressInUse = errors.New("core control address is already in use")

// BootstrapConfig supplies the non-predefined outbound required by the core's
// selector builder. The builder adds its own direct outbounds; including one in
// the input makes current cores generate an invalid empty-direct DNS detour.
const BootstrapConfig = `{"outbounds":[{"type":"vless","tag":"bootstrap","server":"127.0.0.1","server_port":1,"uuid":"00000000-0000-0000-0000-000000000000"}]}`

// Launcher owns a headless core process when the client needs to start one.
type Launcher struct {
	binary string
	state  string
	mu     sync.Mutex
	cmd    *exec.Cmd
	done   chan error
}

// NewLauncher resolves the core binary, falling back to PATH and common
// install locations.
func NewLauncher(binary string) *Launcher {
	if binary == "" {
		binary = findBinary()
	}
	return &Launcher{binary: binary, state: defaultStateDir()}
}

// SetStateDir overrides the core's working state directory. Service installers
// use a system-owned path; interactive launches use a per-user path by default.
func (l *Launcher) SetStateDir(path string) *Launcher {
	l.state = path
	return l
}

func defaultStateDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "hiddify-tui-core")
	}
	return filepath.Join(configDir, "hiddify", "core")
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
func (l *Launcher) Spawned() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cmd != nil && l.cmd.Process != nil
}

// Start launches the core headless and returns a client once it is reachable.
func (l *Launcher) Start(ctx context.Context, address, configContent string, timeout time.Duration) (*client.GRPCClient, error) {
	if l.binary == "" {
		return nil, fmt.Errorf("hiddify-core binary not found")
	}
	if configContent == "" {
		configContent = BootstrapConfig
	}
	if address != client.DefaultAddress {
		return nil, fmt.Errorf("standalone core listens at %s; cannot launch it for custom address %s", client.DefaultAddress, address)
	}
	// Never launch beside an existing GUI, service, or user-started core. Without
	// this guard a newly spawned process that loses the port race could dial and
	// issue lifecycle commands to the process that already owns the address.
	if existing, err := dial(ctx, address, 250*time.Millisecond); err == nil {
		existing.Close()
		return nil, fmt.Errorf("%w: core is already reachable at %s", ErrAddressInUse, address)
	}
	if connection, err := net.DialTimeout("tcp", address, 500*time.Millisecond); err == nil {
		connection.Close()
		return nil, fmt.Errorf("%w: %s", ErrAddressInUse, address)
	}
	if err := os.MkdirAll(l.state, 0o700); err != nil {
		return nil, fmt.Errorf("create core state directory: %w", err)
	}
	configFile, err := os.CreateTemp("", "hiddify-tui-bootstrap-*.json")
	if err != nil {
		return nil, err
	}
	configPath := configFile.Name()
	defer os.Remove(configPath)
	if _, err := configFile.WriteString(configContent); err != nil {
		configFile.Close()
		return nil, err
	}
	if err := configFile.Close(); err != nil {
		return nil, err
	}

	cmd := exec.Command(l.binary, "run", "-c", configPath, "-D", l.state)
	// Upstream initializes part of its storage from the process working
	// directory before applying -D. Pin both to the isolated state directory so
	// it never reads or writes another project's data directory.
	cmd.Dir = l.state
	configureProcess(cmd)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch core: %w", err)
	}
	l.mu.Lock()
	l.cmd = cmd
	l.done = make(chan error, 1)
	done := l.done
	l.mu.Unlock()
	go func() {
		err := cmd.Wait()
		l.mu.Lock()
		if l.cmd == cmd {
			l.cmd = nil
		}
		l.mu.Unlock()
		done <- err
		close(done)
	}()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if core, err := dial(ctx, address, 500*time.Millisecond); err == nil {
			if err := stopBootstrap(ctx, core, deadline, done); err != nil {
				core.Close()
				l.Stop()
				return nil, err
			}
			return core, nil
		} else {
			lastErr = err
		}
		select {
		case waitErr := <-done:
			if waitErr == nil {
				return nil, fmt.Errorf("core exited before becoming reachable")
			}
			return nil, fmt.Errorf("core exited before becoming reachable: %w", waitErr)
		default:
		}
		time.Sleep(200 * time.Millisecond)
	}
	l.Stop()
	return nil, fmt.Errorf("core did not become reachable: %w", lastErr)
}

// stopBootstrap waits for the standalone CLI to finish starting its mandatory
// bootstrap profile, then leaves the gRPC server alive in the stopped state.
func stopBootstrap(ctx context.Context, core *client.GRPCClient, deadline time.Time, done <-chan error) error {
	for time.Now().Before(deadline) {
		requestCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
		snapshot, err := core.Snapshot(requestCtx)
		cancel()
		if err == nil && snapshot.State == client.StateStarted {
			requestCtx, cancel = context.WithTimeout(ctx, 3*time.Second)
			err = core.Disconnect(requestCtx)
			cancel()
			if err != nil {
				return fmt.Errorf("stop bootstrap core: %w", err)
			}
			return waitForStopped(ctx, core, deadline, done)
		}
		select {
		case waitErr := <-done:
			if waitErr == nil {
				return fmt.Errorf("core exited while starting bootstrap")
			}
			return fmt.Errorf("core exited while starting bootstrap: %w", waitErr)
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("core bootstrap did not become ready before timeout")
}

func waitForStopped(ctx context.Context, core *client.GRPCClient, deadline time.Time, done <-chan error) error {
	for time.Now().Before(deadline) {
		requestCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
		snapshot, err := core.Snapshot(requestCtx)
		cancel()
		if err == nil && snapshot.State == client.StateStopped {
			return nil
		}
		select {
		case waitErr := <-done:
			if waitErr == nil {
				return fmt.Errorf("core exited while stopping bootstrap")
			}
			return fmt.Errorf("core exited while stopping bootstrap: %w", waitErr)
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("core bootstrap did not stop before timeout")
}

// Stop terminates the launched core, if any.
func (l *Launcher) Stop() {
	l.mu.Lock()
	cmd, done := l.cmd, l.done
	l.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

// Done reports when the launched core exits. It is used by service wrappers
// to propagate unexpected core failures to the service manager.
func (l *Launcher) Done() <-chan error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.done
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
