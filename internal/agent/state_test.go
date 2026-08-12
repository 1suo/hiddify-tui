package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type fakeBackend struct {
	current ProxyState
	applied []ProxyState
	err     error
}

func (f *fakeBackend) Current(context.Context) (ProxyState, error) { return f.current, f.err }
func (f *fakeBackend) Apply(_ context.Context, state ProxyState) error {
	if f.err != nil {
		return f.err
	}
	f.applied = append(f.applied, state)
	return nil
}

func TestApplyAndRestorePreservesOriginalState(t *testing.T) {
	backend := &fakeBackend{current: ProxyState(`{"mode":"manual","host":"old"}`)}
	path := filepath.Join(t.TempDir(), "recovery.json")
	manager := NewManager(backend, path)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	desired := ProxyState(`{"mode":"manual","host":"127.0.0.1"}`)
	if err := manager.Apply(context.Background(), desired, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(context.Background(), ProxyState(`{"mode":"manual","host":"127.0.0.2"}`), time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := manager.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(backend.applied, []ProxyState{desired, ProxyState(`{"mode":"manual","host":"127.0.0.2"}`), backend.current}) {
		t.Fatalf("applied states: %s", backend.applied)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovery file remains: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary recovery file remains: %v", err)
	}
}

func TestApplyCreatesPrivateRecoveryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "recovery.json")
	manager := NewManager(&fakeBackend{current: ProxyState(`{"old":true}`)}, path)
	if err := manager.Apply(context.Background(), ProxyState(`{"new":true}`), time.Minute); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("recovery mode = %o", info.Mode().Perm())
	}
}

func TestRestoreExpired(t *testing.T) {
	backend := &fakeBackend{current: ProxyState(`{"auto":true}`)}
	manager := NewManager(backend, filepath.Join(t.TempDir(), "recovery.json"))
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	if err := manager.Apply(context.Background(), ProxyState(`{"auto":false}`), time.Minute); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	restored, err := manager.RestoreExpired(context.Background())
	if err != nil || !restored || !reflect.DeepEqual(backend.applied[len(backend.applied)-1], backend.current) {
		t.Fatalf("restored=%v err=%v applied=%s", restored, err, backend.applied)
	}
}

func TestRestoreRetainsRecoveryOnBackendFailure(t *testing.T) {
	backend := &fakeBackend{current: ProxyState(`{"old":true}`)}
	path := filepath.Join(t.TempDir(), "recovery.json")
	manager := NewManager(backend, path)
	if err := manager.Apply(context.Background(), ProxyState(`{"new":true}`), time.Minute); err != nil {
		t.Fatal(err)
	}
	backend.err = errors.New("backend down")
	if err := manager.Restore(context.Background()); err == nil {
		t.Fatal("expected restore error")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("recovery file was removed: %v", err)
	}
}
