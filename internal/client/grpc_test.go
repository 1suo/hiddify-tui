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
	snapshot      *controlv1.Snapshot
	events        []*controlv1.Event
	profiles      []*controlv1.Profile
	content       []byte
	connected     *controlv1.ConnectRequest
	connectResult *controlv1.OperationResult
	groups        []*controlv1.OutboundGroup
	selected      *controlv1.SelectOutboundRequest
	testScope     *controlv1.TestOutboundsRequest
	logs          []*controlv1.LogEntry
	cleared       bool
	settings      []byte
	auto          bool
}

func (s *testControlServer) Connect(_ context.Context, request *controlv1.ConnectRequest) (*controlv1.OperationResult, error) {
	s.connected = request
	if s.connectResult != nil {
		return s.connectResult, nil
	}
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

func (s *testControlServer) TailLogs(_ *controlv1.TailLogsRequest, stream grpc.ServerStreamingServer[controlv1.LogEntry]) error {
	for _, entry := range s.logs {
		if err := stream.Send(entry); err != nil {
			return err
		}
	}
	return nil
}

func (s *testControlServer) ClearLogs(context.Context, *controlv1.ClearLogsRequest) (*controlv1.OperationResult, error) {
	s.cleared = true
	return &controlv1.OperationResult{}, nil
}

func (s *testControlServer) GetSettings(context.Context, *controlv1.GetSettingsRequest) (*controlv1.Settings, error) {
	return &controlv1.Settings{RedactedJson: s.settings}, nil
}

func (s *testControlServer) ValidateSettings(_ context.Context, request *controlv1.ValidateSettingsRequest) (*controlv1.ValidationResult, error) {
	return &controlv1.ValidationResult{Valid: string(request.GetCandidateJson()) == "{}"}, nil
}

func (s *testControlServer) UpdateSettings(_ context.Context, request *controlv1.UpdateSettingsRequest) (*controlv1.Settings, error) {
	s.settings = request.GetCandidateJson()
	return &controlv1.Settings{RedactedJson: s.settings}, nil
}

func (s *testControlServer) ResetSettings(context.Context, *controlv1.ResetSettingsRequest) (*controlv1.Settings, error) {
	s.settings = []byte(`{}`)
	return &controlv1.Settings{RedactedJson: s.settings}, nil
}

func (s *testControlServer) ExportSettings(_ context.Context, request *controlv1.ExportSettingsRequest) (*controlv1.ExportSettingsResponse, error) {
	if request.GetIncludeSecrets() {
		return &controlv1.ExportSettingsResponse{Json: []byte(`{"secret":true}`)}, nil
	}
	return &controlv1.ExportSettingsResponse{Json: s.settings}, nil
}

func (s *testControlServer) ImportSettings(_ context.Context, request *controlv1.ImportSettingsRequest) (*controlv1.Settings, error) {
	s.settings = request.GetJson()
	return &controlv1.Settings{RedactedJson: s.settings}, nil
}

func (s *testControlServer) GetServiceInfo(context.Context, *controlv1.GetServiceInfoRequest) (*controlv1.ServiceInfo, error) {
	return &controlv1.ServiceInfo{Installed: true, Enabled: true, Running: true}, nil
}

func (s *testControlServer) SetAutoConnect(_ context.Context, request *controlv1.SetAutoConnectRequest) (*controlv1.OperationResult, error) {
	s.auto = request.GetEnabled()
	return &controlv1.OperationResult{}, nil
}

func (s *testControlServer) GetDiagnostics(context.Context, *controlv1.GetDiagnosticsRequest) (*controlv1.Diagnostics, error) {
	return &controlv1.Diagnostics{DaemonVersion: "1.0", CoreVersion: "core", SocketPath: "/run/hiddify/control.sock"}, nil
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
			AutoConnect:       true,
		},
		events: []*controlv1.Event{{
			Sequence: 5,
			Revision: 3,
			Change:   &controlv1.Event_Outbound{Outbound: &controlv1.OutboundChange{SelectedOutbound: "fast"}},
		}},
		profiles: []*controlv1.Profile{{Id: "p-1", Name: "Home", Kind: controlv1.ProfileKind_PROFILE_KIND_REMOTE, RedactedUrl: "https://example.test/…"}},
		groups:   []*controlv1.OutboundGroup{{Id: "g-1", Name: "Auto", SelectedOutboundId: "o-1", Outbounds: []*controlv1.Outbound{{Id: "o-1", Tag: "Fast", Selectable: true, DelayMillis: 42}}}},
		logs:     []*controlv1.LogEntry{{Sequence: 1, Level: controlv1.LogLevel_LOG_LEVEL_WARN, Component: "core", Message: "redacted"}},
		settings: []byte(`{"mode":"tun"}`),
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
	if state.Snapshot.ConnectionState != control.ConnectionStarted || state.Snapshot.SelectedOutbound != "fast" || state.LastSequence != 5 || state.Snapshot.Traffic.DownlinkBytesPerSecond != 2048 || state.Snapshot.System.ConnectionCount != 2 || !state.Snapshot.Agent.Connected || !state.Snapshot.AutoConnect {
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
	entries, err := daemon.TailLogs(ctx, 10, control.LogInfo, false)
	if err != nil || (<-entries).Message != "redacted" {
		t.Fatalf("logs err=%v", err)
	}
	if err := daemon.ClearLogs(ctx); err != nil || !server.cleared {
		t.Fatalf("clear logs err=%v cleared=%v", err, server.cleared)
	}
	service, err := daemon.GetServiceInfo(ctx)
	if err != nil || !service.Running {
		t.Fatalf("service=%#v err=%v", service, err)
	}
	if err := daemon.SetAutoConnect(ctx, false); err != nil || server.auto {
		t.Fatalf("auto=%v err=%v", server.auto, err)
	}
	diagnostics, err := daemon.GetDiagnostics(ctx)
	if err != nil || diagnostics.SocketPath != "/run/hiddify/control.sock" {
		t.Fatalf("diagnostics=%#v err=%v", diagnostics, err)
	}
	settings, err := daemon.GetSettings(ctx)
	if err != nil || string(settings.RedactedJSON) != `{"mode":"tun"}` {
		t.Fatalf("settings=%#v err=%v", settings, err)
	}
	validation, err := daemon.ValidateSettings(ctx, []byte(`{}`))
	if err != nil || !validation.Valid {
		t.Fatalf("validation=%#v err=%v", validation, err)
	}
	settings, err = daemon.UpdateSettings(ctx, []byte(`{"mode":"local-proxy"}`))
	if err != nil || string(settings.RedactedJSON) != `{"mode":"local-proxy"}` {
		t.Fatalf("updated=%#v err=%v", settings, err)
	}
}

func TestGRPCClientReturnsTypedOperationFailure(t *testing.T) {
	server := &testControlServer{connectResult: &controlv1.OperationResult{ErrorCode: controlv1.ErrorCode_ERROR_CODE_NO_ACTIVE_PROFILE, Message: "active profile required"}}
	socket := startUnixServer(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	daemon, err := client.DialUnix(ctx, socket)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Close()
	err = daemon.Connect(ctx, "", control.ModeTUN)
	if err == nil || !strings.Contains(err.Error(), "ERROR_CODE_NO_ACTIVE_PROFILE") {
		t.Fatalf("connect error = %v", err)
	}
}
