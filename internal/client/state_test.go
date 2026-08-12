package client_test

import (
	"context"
	"testing"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
)

func TestStateAppliesContiguousEvents(t *testing.T) {
	ctx := context.Background()
	daemon := client.FakeControl{Snapshot: control.Snapshot{APIMajor: 1, Revision: 1, ConnectionState: control.ConnectionStopped}}
	state, err := client.NewState(ctx, daemon)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []control.Event{
		{Sequence: 1, Revision: 2, Kind: control.EventProfile, ActiveProfileID: "p-1", ActiveProfileName: "Home"},
		{Sequence: 2, Revision: 3, Kind: control.EventConnection, ConnectionState: control.ConnectionStarted, EffectiveMode: "tun"},
		{Sequence: 3, Revision: 4, Kind: control.EventAgent, Agent: control.AgentHealth{Required: true, Connected: true, Applied: true}},
	} {
		if err := state.Apply(ctx, daemon, event); err != nil {
			t.Fatal(err)
		}
	}
	if state.LastSequence != 3 || state.Snapshot.Revision != 4 || state.Snapshot.ActiveProfileName != "Home" || state.Snapshot.ConnectionState != control.ConnectionStarted || !state.Snapshot.Agent.Applied {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestStateResynchronizesOnSequenceGap(t *testing.T) {
	ctx := context.Background()
	state := client.State{Snapshot: control.Snapshot{APIMajor: 1, Revision: 1}, LastSequence: 1}
	daemon := client.FakeControl{Snapshot: control.Snapshot{APIMajor: 1, Revision: 7, EventSequence: 8, ConnectionState: control.ConnectionStarted, ActiveProfileName: "Recovered"}}
	if err := state.Apply(ctx, daemon, control.Event{Sequence: 3, Revision: 3, Kind: control.EventConnection}); err != nil {
		t.Fatal(err)
	}
	if state.LastSequence != 8 || state.Snapshot.Revision != 7 || state.Snapshot.ActiveProfileName != "Recovered" {
		t.Fatalf("gap did not reload snapshot: %#v", state)
	}
}

func TestWatchStartsAfterCurrentSequence(t *testing.T) {
	ctx := context.Background()
	state := client.State{Snapshot: control.Snapshot{APIMajor: 1}, LastSequence: 1}
	daemon := client.FakeControl{Snapshot: control.Snapshot{APIMajor: 1}, Events: []control.Event{
		{Sequence: 1, Kind: control.EventWarning, LastError: "old"},
		{Sequence: 2, Kind: control.EventWarning, LastError: "new"},
	}}
	if err := state.Watch(ctx, daemon, daemon); err != nil {
		t.Fatal(err)
	}
	if state.LastSequence != 2 || state.Snapshot.LastError != "new" {
		t.Fatalf("watch state: %#v", state)
	}
}
