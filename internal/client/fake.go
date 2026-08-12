package client

import (
	"context"
	"errors"

	"github.com/1suo/hiddify-tui/internal/control"
)

// FakeControl is a deterministic in-memory control endpoint for client tests.
type FakeControl struct {
	Snapshot control.Snapshot
	Err      error
	Events   []control.Event
}

func (f FakeControl) WatchEvents(ctx context.Context, afterSequence uint64) (<-chan control.Event, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	events := make(chan control.Event, len(f.Events))
	for _, event := range f.Events {
		if event.Sequence > afterSequence {
			events <- event
		}
	}
	close(events)
	return events, nil
}

func (f FakeControl) GetSnapshot(context.Context) (control.Snapshot, error) {
	if f.Err != nil {
		return control.Snapshot{}, f.Err
	}
	if f.Snapshot.APIMajor == 0 {
		return control.Snapshot{}, errors.New("fake snapshot has no API major version")
	}
	return f.Snapshot, nil
}
