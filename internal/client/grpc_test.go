package client_test

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strings"
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
	snapshot  *controlv1.Snapshot
	events    []*controlv1.Event
	profiles  []*controlv1.Profile
	content   []byte
	connected *controlv1.ConnectRequest
	groups    []*controlv1.OutboundGroup
	selected  *controlv1.SelectOutboundRequest
	testScope *controlv1.TestOutboundsRequest
}

func (s *testControlServer) Connect(_ context.Context, request *controlv1.ConnectRequest) (*controlv1.OperationResult, error) {
	s.connected = request
	return &controlv1.OperationResult{}, nil
}

func (s *testControlServer) Disconnect(context.Context, *controlv1.DisconnectRequest) (*controlv1.OperationResult, error) {
	return &controlv1.OperationResult{}, nil
}

func (s *testControlServer) Restart(context.Context, *controlv1.RestartRequest) (*controlv1.OperationResult, error) {
	return &controlv1.OperationResult{}, nil
}

func (s *testControlServer) ListOutboundGroups(context.Context, *controlv1.ListOutboundGroupsRequest) (*controlv1.ListOutboundGroupsResponse, error) {
	return &controlv1.ListOutboundGroupsResponse{Groups: s.groups}, nil
}

func (s *testControlServer) SelectOutbound(_ context.Context, request *controlv1.SelectOutboundRequest) (*controlv1.OperationResult, error) {
	s.selected = request
	return &controlv1.OperationResult{}, nil
}

func (s *testControlServer) TestOutbounds(_ context.Context, request *controlv1.TestOutboundsRequest) (*controlv1.OperationResult, error) {
	s.testScope = request
	return &controlv1.OperationResult{}, nil
}

func (s *testControlServer) AddLocalProfile(stream grpc.ClientStreamingServer[controlv1.AddLocalProfileRequest, controlv1.Profile]) error {
	var content []byte
	for {
		request, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		content = append(content, request.GetContentChunk()...)
	}
	s.content = content
	return stream.SendAndClose(&controlv1.Profile{Id: "p-local", Name: "Imported", Kind: controlv1.ProfileKind_PROFILE_KIND_LOCAL})
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
	server := &testControlServer{
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
		groups:   []*controlv1.OutboundGroup{{Id: "g-1", Name: "Auto", SelectedOutboundId: "o-1", Outbounds: []*controlv1.Outbound{{Id: "o-1", Tag: "Fast", Selectable: true, DelayMillis: 42}}}},
	}
	socket := startUnixServer(t, server)
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
	added, err := daemon.AddLocalProfile(ctx, "Imported", false, strings.NewReader("vmess://example"))
	if err != nil || added.ID != "p-local" || string(server.content) != "vmess://example" {
		t.Fatalf("added = %#v, content=%q, err=%v", added, server.content, err)
	}
	if err := daemon.Connect(ctx, "p-1", control.ModeTUN); err != nil || server.connected.GetProfileId() != "p-1" || server.connected.GetMode() != controlv1.ConnectionMode_CONNECTION_MODE_TUN {
		t.Fatalf("connect request=%#v err=%v", server.connected, err)
	}
	groups, err := daemon.ListOutboundGroups(ctx)
	if err != nil || len(groups) != 1 || groups[0].Outbounds[0].DelayMillis != 42 {
		t.Fatalf("outbound groups = %#v, %v", groups, err)
	}
	if err := daemon.SelectOutbound(ctx, "g-1", "o-1"); err != nil || server.selected.GetGroupId() != "g-1" {
		t.Fatalf("select request=%#v err=%v", server.selected, err)
	}
	if err := daemon.TestOutbounds(ctx, control.TestScope{AllVisible: true}); err != nil || !server.testScope.GetAllVisible() {
		t.Fatalf("test request=%#v err=%v", server.testScope, err)
	}
}
