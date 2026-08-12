package client

import (
	"context"
	"fmt"

	"github.com/1suo/hiddify-tui/internal/control"
)

// State is the client-side projection rebuilt from a snapshot and ordered
// daemon events. It never persists daemon-owned state.
type State struct {
	Snapshot     control.Snapshot
	LastSequence uint64
}

func NewState(ctx context.Context, daemon control.Client) (State, error) {
	snapshot, err := Snapshot(ctx, daemon)
	if err != nil {
		return State{}, err
	}
	return State{Snapshot: snapshot, LastSequence: snapshot.EventSequence}, nil
}

// Apply applies a contiguous event. A sequence gap and an explicit resync
// marker both reload the complete daemon snapshot.
func (s *State) Apply(ctx context.Context, daemon control.Client, event control.Event) error {
	if event.Kind == control.EventResync || (s.LastSequence != 0 && event.Sequence != s.LastSequence+1) {
		refreshed, err := NewState(ctx, daemon)
		if err != nil {
			return fmt.Errorf("resynchronize client state: %w", err)
		}
		*s = refreshed
		return nil
	}
	if event.Sequence == 0 {
		return fmt.Errorf("received event without sequence number")
	}

	s.LastSequence = event.Sequence
	if event.Revision > s.Snapshot.Revision {
		s.Snapshot.Revision = event.Revision
	}
	switch event.Kind {
	case control.EventConnection:
		s.Snapshot.ConnectionState = event.ConnectionState
		s.Snapshot.RequestedMode = event.RequestedMode
		s.Snapshot.EffectiveMode = event.EffectiveMode
	case control.EventProfile:
		s.Snapshot.ActiveProfileID = event.ActiveProfileID
		s.Snapshot.ActiveProfileName = event.ActiveProfileName
	case control.EventOutbound:
		s.Snapshot.SelectedOutbound = event.SelectedOutbound
	case control.EventWarning:
		s.Snapshot.LastError = event.LastError
	case control.EventAgent:
		s.Snapshot.Agent = event.Agent
	default:
		return fmt.Errorf("received unsupported event kind %q", event.Kind)
	}
	return nil
}

// Watch keeps State current until the caller cancels ctx or the daemon closes
// the stream. A reconnecting transport can call it again with LastSequence.
func (s *State) Watch(ctx context.Context, daemon control.Client, watcher control.Watcher) error {
	events, err := watcher.WatchEvents(ctx, s.LastSequence)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := s.Apply(ctx, daemon, event); err != nil {
				return err
			}
		}
	}
}
