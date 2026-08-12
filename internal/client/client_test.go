package client_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
)

func TestSnapshotRejectsIncompatibleMajor(t *testing.T) {
	_, err := client.Snapshot(context.Background(), client.FakeControl{Snapshot: control.Snapshot{APIMajor: 2}})
	if err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("expected incompatible API error, got %v", err)
	}
}

func TestSnapshotReturnsCompatibleResponse(t *testing.T) {
	want := control.Snapshot{APIMajor: 1, ConnectionState: control.ConnectionStarted}
	got, err := client.Snapshot(context.Background(), client.FakeControl{Snapshot: want})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %#v, %v; want %#v, nil", got, err, want)
	}
}
