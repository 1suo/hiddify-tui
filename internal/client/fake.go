package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/1suo/hiddify-tui/internal/control"
)

// FakeControl is a deterministic in-memory control endpoint for client tests.
type FakeControl struct {
	Snapshot control.Snapshot
	Err      error
	Events   []control.Event
	Profiles []control.Profile
}

func (f FakeControl) ListProfiles(context.Context) ([]control.Profile, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Profiles, nil
}

func (f FakeControl) GetProfile(_ context.Context, id string) (control.Profile, error) {
	if f.Err != nil {
		return control.Profile{}, f.Err
	}
	for _, profile := range f.Profiles {
		if profile.ID == id {
			return profile, nil
		}
	}
	return control.Profile{}, fmt.Errorf("profile %q not found", id)
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
