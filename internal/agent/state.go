// Package agent implements the durable, platform-neutral recovery state for
// the per-user system-proxy helper. Platform code supplies the actual proxy
// reads and writes.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ProxyState is an opaque, platform-specific representation of all current
// per-user proxy settings. It is deliberately persisted byte-for-byte.
type ProxyState json.RawMessage

type Backend interface {
	Current(context.Context) (ProxyState, error)
	Apply(context.Context, ProxyState) error
}

type Recovery struct {
	Previous   ProxyState `json:"previous"`
	LeaseUntil time.Time  `json:"lease_until"`
}

type Manager struct {
	backend Backend
	path    string
	now     func() time.Time
}

func NewManager(backend Backend, recoveryPath string) *Manager {
	return &Manager{backend: backend, path: recoveryPath, now: time.Now}
}

// Apply records existing user proxy settings durably before changing them. An
// existing recovery record means a prior agent has already captured the true
// pre-Hiddify state and must not be overwritten.
func (m *Manager) Apply(ctx context.Context, desired ProxyState, lease time.Duration) error {
	if _, err := m.load(); errors.Is(err, os.ErrNotExist) {
		previous, err := m.backend.Current(ctx)
		if err != nil {
			return fmt.Errorf("read current proxy state: %w", err)
		}
		if err := m.save(Recovery{Previous: previous, LeaseUntil: m.now().Add(lease)}); err != nil {
			return fmt.Errorf("save proxy recovery state: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("read proxy recovery state: %w", err)
	} else if err := m.renew(lease); err != nil {
		return err
	}
	if err := m.backend.Apply(ctx, desired); err != nil {
		return fmt.Errorf("apply system proxy: %w", err)
	}
	return nil
}

func (m *Manager) Renew(lease time.Duration) error {
	return m.renew(lease)
}

func (m *Manager) renew(lease time.Duration) error {
	recovery, err := m.load()
	if err != nil {
		return fmt.Errorf("read proxy recovery state: %w", err)
	}
	recovery.LeaseUntil = m.now().Add(lease)
	if err := m.save(recovery); err != nil {
		return fmt.Errorf("renew proxy lease: %w", err)
	}
	return nil
}

// Restore applies the saved exact state, then removes recovery data. It is
// idempotent: no record means there is nothing left to restore.
func (m *Manager) Restore(ctx context.Context) error {
	recovery, err := m.load()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read proxy recovery state: %w", err)
	}
	if err := m.backend.Apply(ctx, recovery.Previous); err != nil {
		return fmt.Errorf("restore system proxy: %w", err)
	}
	if err := os.Remove(m.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove proxy recovery state: %w", err)
	}
	return nil
}

// RestoreExpired is safe to run at agent startup and periodically thereafter.
func (m *Manager) RestoreExpired(ctx context.Context) (bool, error) {
	recovery, err := m.load()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if m.now().Before(recovery.LeaseUntil) {
		return false, nil
	}
	return true, m.Restore(ctx)
}

func (m *Manager) load() (Recovery, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return Recovery{}, err
	}
	var recovery Recovery
	if err := json.Unmarshal(data, &recovery); err != nil {
		return Recovery{}, fmt.Errorf("decode recovery state: %w", err)
	}
	if len(recovery.Previous) == 0 {
		return Recovery{}, errors.New("recovery state has no previous proxy state")
	}
	return recovery, nil
}

func (m *Manager) save(recovery Recovery) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(recovery)
	if err != nil {
		return err
	}
	temporary := m.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, m.path)
}
