package client

import (
	"context"
)

// FakeClient is a deterministic in-memory Client for tests.
type FakeClient struct {
	SnapshotFn   func() Snapshot
	StatusEvents []Snapshot
	Groups       []OutboundGroup
	Logs         []LogEntry
	ParseErr     error
	Connects     int
	Disconnects  int
	Restarts     int
	connectErr   error
}

func (f *FakeClient) Connect(context.Context, string, string) error {
	f.Connects++
	return f.connectErr
}

func (f *FakeClient) Disconnect(context.Context) error {
	f.Disconnects++
	return nil
}

func (f *FakeClient) Restart(context.Context, string, string) error {
	f.Restarts++
	return nil
}

func (f *FakeClient) Snapshot(context.Context) (Snapshot, error) {
	if f.SnapshotFn != nil {
		return f.SnapshotFn(), nil
	}
	return Snapshot{State: StateStopped}, nil
}

func (f *FakeClient) WatchStatus(ctx context.Context) (<-chan Snapshot, error) {
	out := make(chan Snapshot)
	go func() {
		defer close(out)
		for _, snapshot := range f.StatusEvents {
			select {
			case out <- snapshot:
			case <-ctx.Done():
				return
			}
		}
		<-ctx.Done()
	}()
	return out, nil
}

func (f *FakeClient) OutboundGroups(context.Context) ([]OutboundGroup, error) {
	return f.Groups, nil
}

func (f *FakeClient) SelectOutbound(context.Context, string, string) error { return nil }

func (f *FakeClient) TestOutbound(context.Context, string) error { return nil }

func (f *FakeClient) Parse(context.Context, string) error { return f.ParseErr }

func (f *FakeClient) ChangeSettings(context.Context, string) error { return nil }

func (f *FakeClient) WatchLogs(ctx context.Context, _ LogLevel) (<-chan LogEntry, error) {
	out := make(chan LogEntry)
	go func() {
		defer close(out)
		for _, entry := range f.Logs {
			select {
			case out <- entry:
			case <-ctx.Done():
				return
			}
		}
		<-ctx.Done()
	}()
	return out, nil
}

func (f *FakeClient) Close() error { return nil }

var _ Client = (*FakeClient)(nil)
