package client_test

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	controlv1 "github.com/1suo/hiddify-tui/gen/control/v1"
	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
	"google.golang.org/grpc"
)

type testControlServer struct {
	controlv1.UnimplementedControlServiceServer
	snapshot *controlv1.Snapshot
	events   []*controlv1.Event
	profiles []*controlv1.Profile
}

func (s *testControlServer) ListProfiles(context.Context, *controlv1.ListProfilesRequest) (*controlv1.ListProfilesResponse, error) {
	return &controlv1.ListProfilesResponse{Profiles: s.profiles}, nil
}

func (s *testControlServer) GetProfile(_ context.Context, request *controlv1.GetProfileRequest) (*controlv1.Profile, error) {
	for _, profile := range s.profiles {
		if profile.GetId() == request.GetProfileId() {
			return profile, nil
		}
	}
	return nil, nil
}

func (s *testControlServer) GetSnapshot(context.Context, *controlv1.GetSnapshotRequest) (*controlv1.Snapshot, error) {
	return s.snapshot, nil
}

func (s *testControlServer) WatchEvents(_ *controlv1.WatchRequest, stream grpc.ServerStreamingServer[controlv1.Event]) error {
	for _, event := range s.events {
		if err := stream.Send(event); err != nil {
			return err
		}
	}
	return nil
}

func startUnixServer(t *testing.T, server controlv1.ControlServiceServer) string {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "control.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
			t.Skipf("Unix-domain sockets unavailable in this environment: %v", err)
		}
		t.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	controlv1.RegisterControlServiceServer(grpcServer, server)
	go grpcServer.Serve(listener)
	t.Cleanup(func() {
		grpcServer.Stop()
		listener.Close()
	})
	return socket
}

func TestGRPCClientSnapshotAndEventsOverUnixSocket(t *testing.T) {
	socket := startUnixServer(t, &testControlServer{
		snapshot: &controlv1.Snapshot{
			ApiMajor:          1,
			Revision:          2,
			EventSequence:     4,
			ConnectionState:   controlv1.ConnectionState_CONNECTION_STATE_STARTED,
			ActiveProfileName: "Home",
			Traffic:           &controlv1.TrafficStats{DownlinkBytesPerSecond: 2048},
			System:            &controlv1.SystemStats{ConnectionCount: 2},
			Agent:             &controlv1.AgentHealth{Required: true, Connected: true},
		},
		events: []*controlv1.Event{{
			Sequence: 5,
			Revision: 3,
			Change:   &controlv1.Event_Outbound{Outbound: &controlv1.OutboundChange{SelectedOutbound: "fast"}},
		}},
		profiles: []*controlv1.Profile{{Id: "p-1", Name: "Home", Kind: controlv1.ProfileKind_PROFILE_KIND_REMOTE, RedactedUrl: "https://example.test/…"}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	daemon, err := client.DialUnix(ctx, socket)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Close()

	state, err := client.NewState(ctx, daemon)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Watch(ctx, daemon, daemon); err != nil {
		t.Fatal(err)
	}
	if state.Snapshot.ConnectionState != control.ConnectionStarted || state.Snapshot.SelectedOutbound != "fast" || state.LastSequence != 5 || state.Snapshot.Traffic.DownlinkBytesPerSecond != 2048 || state.Snapshot.System.ConnectionCount != 2 || !state.Snapshot.Agent.Connected {
		t.Fatalf("unexpected recovered state: %#v", state)
	}
	profiles, err := daemon.ListProfiles(ctx)
	if err != nil || len(profiles) != 1 || profiles[0].RedactedURL != "https://example.test/…" {
		t.Fatalf("profiles = %#v, %v", profiles, err)
	}
}
