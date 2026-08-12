package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"runtime"

	controlv1 "github.com/1suo/hiddify-tui/gen/control/v1"
	"github.com/1suo/hiddify-tui/internal/control"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCClient is the local-IPC implementation of the thin client boundary.
// Unix-domain socket permissions authenticate the caller; TLS is deliberately
// not layered over this host-local transport.
type GRPCClient struct {
	connection *grpc.ClientConn
	api        controlv1.ControlServiceClient
}

const MaxLocalProfileBytes int64 = 10 << 20

func DefaultSocket() string {
	if runtime.GOOS == "darwin" {
		return "/var/run/hiddify/control.sock"
	}
	return "/run/hiddify/control.sock"
}

func DialUnix(ctx context.Context, socket string) (*GRPCClient, error) {
	connection, err := grpc.DialContext(ctx, "passthrough:///hiddify-control",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		}),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return &GRPCClient{connection: connection, api: controlv1.NewControlServiceClient(connection)}, nil
}

func (c *GRPCClient) Close() error {
	return c.connection.Close()
}

func (c *GRPCClient) GetSnapshot(ctx context.Context) (control.Snapshot, error) {
	response, err := c.api.GetSnapshot(ctx, &controlv1.GetSnapshotRequest{})
	if err != nil {
		return control.Snapshot{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return snapshotFromProto(response), nil
}

func (c *GRPCClient) Connect(ctx context.Context, profileID string, mode control.ConnectionMode) error {
	_, err := c.api.Connect(ctx, &controlv1.ConnectRequest{ProfileId: profileID, Mode: connectionModeToProto(mode)})
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return nil
}

func (c *GRPCClient) Disconnect(ctx context.Context) error {
	_, err := c.api.Disconnect(ctx, &controlv1.DisconnectRequest{})
	if err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}
	return nil
}

func (c *GRPCClient) Restart(ctx context.Context) error {
	_, err := c.api.Restart(ctx, &controlv1.RestartRequest{})
	if err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	return nil
}

func (c *GRPCClient) WatchEvents(ctx context.Context, afterSequence uint64) (<-chan control.Event, error) {
	stream, err := c.api.WatchEvents(ctx, &controlv1.WatchRequest{AfterSequence: afterSequence})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	events := make(chan control.Event)
	go func() {
		defer close(events)
		for {
			event, err := stream.Recv()
			if err != nil {
				return
			}
			converted := eventFromProto(event)
			select {
			case events <- converted:
			case <-ctx.Done():
				return
			}
		}
	}()
	return events, nil
}

func (c *GRPCClient) ListProfiles(ctx context.Context) ([]control.Profile, error) {
	response, err := c.api.ListProfiles(ctx, &controlv1.ListProfilesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	profiles := make([]control.Profile, 0, len(response.GetProfiles()))
	for _, profile := range response.GetProfiles() {
		profiles = append(profiles, profileFromProto(profile))
	}
	return profiles, nil
}

func (c *GRPCClient) GetProfile(ctx context.Context, id string) (control.Profile, error) {
	response, err := c.api.GetProfile(ctx, &controlv1.GetProfileRequest{ProfileId: id})
	if err != nil {
		return control.Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return profileFromProto(response), nil
}

func (c *GRPCClient) AddRemoteProfile(ctx context.Context, url, name string, active bool) (control.Profile, error) {
	response, err := c.api.AddRemoteProfile(ctx, &controlv1.AddRemoteProfileRequest{Url: url, Name: name, SetActive: active})
	if err != nil {
		return control.Profile{}, fmt.Errorf("add remote profile: %w", err)
	}
	return profileFromProto(response), nil
}

func (c *GRPCClient) AddLocalProfile(ctx context.Context, name string, active bool, content io.Reader) (control.Profile, error) {
	stream, err := c.api.AddLocalProfile(ctx)
	if err != nil {
		return control.Profile{}, fmt.Errorf("add local profile: %w", err)
	}
	if err := stream.Send(&controlv1.AddLocalProfileRequest{Part: &controlv1.AddLocalProfileRequest_Metadata{Metadata: &controlv1.LocalProfileMetadata{Name: name, SetActive: active}}}); err != nil {
		return control.Profile{}, fmt.Errorf("send local profile metadata: %w", err)
	}
	buffer := make([]byte, 32*1024)
	var total int64
	for {
		count, readErr := content.Read(buffer)
		if count > 0 {
			total += int64(count)
			if total > MaxLocalProfileBytes {
				return control.Profile{}, fmt.Errorf("local profile exceeds %d byte limit", MaxLocalProfileBytes)
			}
			chunk := append([]byte(nil), buffer[:count]...)
			if err := stream.Send(&controlv1.AddLocalProfileRequest{Part: &controlv1.AddLocalProfileRequest_ContentChunk{ContentChunk: chunk}}); err != nil {
				return control.Profile{}, fmt.Errorf("send local profile content: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return control.Profile{}, fmt.Errorf("read local profile content: %w", readErr)
		}
	}
	response, err := stream.CloseAndRecv()
	if err != nil {
		return control.Profile{}, fmt.Errorf("add local profile: %w", err)
	}
	return profileFromProto(response), nil
}

func (c *GRPCClient) UpdateProfileName(ctx context.Context, id, name string) (control.Profile, error) {
	response, err := c.api.UpdateProfile(ctx, &controlv1.UpdateProfileRequest{ProfileId: id, Name: name})
	if err != nil {
		return control.Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return profileFromProto(response), nil
}

func (c *GRPCClient) RefreshProfile(ctx context.Context, id string) error {
	_, err := c.api.RefreshProfile(ctx, &controlv1.RefreshProfileRequest{ProfileId: id})
	if err != nil {
		return fmt.Errorf("refresh profile: %w", err)
	}
	return nil
}

func (c *GRPCClient) DeleteProfile(ctx context.Context, id string) error {
	_, err := c.api.DeleteProfile(ctx, &controlv1.DeleteProfileRequest{ProfileId: id})
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	return nil
}

func (c *GRPCClient) SetActiveProfile(ctx context.Context, id string) error {
	_, err := c.api.SetActiveProfile(ctx, &controlv1.SetActiveProfileRequest{ProfileId: id})
	if err != nil {
		return fmt.Errorf("activate profile: %w", err)
	}
	return nil
}

func profileFromProto(profile *controlv1.Profile) control.Profile {
	return control.Profile{
		ID:                    profile.GetId(),
		Name:                  profile.GetName(),
		Kind:                  profileKindFromProto(profile.GetKind()),
		Active:                profile.GetActive(),
		RedactedURL:           profile.GetRedactedUrl(),
		LastSuccessfulRefresh: profile.GetLastSuccessfulRefreshUnix(),
		LastAttemptedRefresh:  profile.GetLastAttemptedRefreshUnix(),
		UpdateIntervalSeconds: profile.GetUpdateIntervalSeconds(),
		Subscription: control.Usage{
			UploadBytes:   profile.GetSubscription().GetUploadBytes(),
			DownloadBytes: profile.GetSubscription().GetDownloadBytes(),
			TotalBytes:    profile.GetSubscription().GetTotalBytes(),
			ExpiryUnix:    profile.GetSubscription().GetExpiryUnix(),
		},
		LastRefreshError: profile.GetLastRefreshError(),
	}
}

func profileKindFromProto(kind controlv1.ProfileKind) control.ProfileKind {
	if kind == controlv1.ProfileKind_PROFILE_KIND_LOCAL {
		return control.ProfileLocal
	}
	return control.ProfileRemote
}

func snapshotFromProto(snapshot *controlv1.Snapshot) control.Snapshot {
	return control.Snapshot{
		APIMajor:          snapshot.GetApiMajor(),
		APIMinor:          snapshot.GetApiMinor(),
		Revision:          snapshot.GetRevision(),
		EventSequence:     snapshot.GetEventSequence(),
		DaemonVersion:     snapshot.GetDaemonVersion(),
		CoreVersion:       snapshot.GetCoreVersion(),
		ConnectionState:   connectionStateFromProto(snapshot.GetConnectionState()),
		ActiveProfileID:   snapshot.GetActiveProfileId(),
		ActiveProfileName: snapshot.GetActiveProfileName(),
		RequestedMode:     snapshot.GetRequestedMode(),
		EffectiveMode:     snapshot.GetEffectiveMode(),
		SelectedOutbound:  snapshot.GetSelectedOutbound(),
		LastError:         snapshot.GetLastError(),
		Traffic: control.TrafficStats{
			UplinkBytesPerSecond:   snapshot.GetTraffic().GetUplinkBytesPerSecond(),
			DownlinkBytesPerSecond: snapshot.GetTraffic().GetDownlinkBytesPerSecond(),
			TotalUploadBytes:       snapshot.GetTraffic().GetTotalUploadBytes(),
			TotalDownloadBytes:     snapshot.GetTraffic().GetTotalDownloadBytes(),
		},
		System: control.SystemStats{
			MemoryBytes:     snapshot.GetSystem().GetMemoryBytes(),
			ConnectionCount: snapshot.GetSystem().GetConnectionCount(),
		},
		Agent: control.AgentHealth{
			Required:  snapshot.GetAgent().GetRequired(),
			Connected: snapshot.GetAgent().GetConnected(),
			LastError: snapshot.GetAgent().GetLastError(),
		},
		Capabilities: snapshot.GetCapabilities(),
	}
}

func eventFromProto(event *controlv1.Event) control.Event {
	converted := control.Event{Sequence: event.GetSequence(), Revision: event.GetRevision()}
	switch change := event.GetChange().(type) {
	case *controlv1.Event_Connection:
		converted.Kind = control.EventConnection
		converted.ConnectionState = connectionStateFromProto(change.Connection.GetState())
		converted.RequestedMode = change.Connection.GetRequestedMode()
		converted.EffectiveMode = change.Connection.GetEffectiveMode()
	case *controlv1.Event_Profile:
		converted.Kind = control.EventProfile
		converted.ActiveProfileID = change.Profile.GetActiveProfileId()
		converted.ActiveProfileName = change.Profile.GetActiveProfileName()
	case *controlv1.Event_Outbound:
		converted.Kind = control.EventOutbound
		converted.SelectedOutbound = change.Outbound.GetSelectedOutbound()
	case *controlv1.Event_Warning:
		converted.Kind = control.EventWarning
		converted.LastError = change.Warning.GetMessage()
	case *controlv1.Event_ResyncRequired:
		converted.Kind = control.EventResync
	default:
		converted.Kind = control.EventResync
	}
	return converted
}

func connectionStateFromProto(state controlv1.ConnectionState) control.ConnectionState {
	switch state {
	case controlv1.ConnectionState_CONNECTION_STATE_STARTING:
		return control.ConnectionStarting
	case controlv1.ConnectionState_CONNECTION_STATE_STARTED:
		return control.ConnectionStarted
	case controlv1.ConnectionState_CONNECTION_STATE_STOPPING:
		return control.ConnectionStopping
	case controlv1.ConnectionState_CONNECTION_STATE_RECONNECT_WAIT:
		return control.ConnectionReconnectWait
	case controlv1.ConnectionState_CONNECTION_STATE_FAILED:
		return control.ConnectionFailed
	default:
		return control.ConnectionStopped
	}
}

func connectionModeToProto(mode control.ConnectionMode) controlv1.ConnectionMode {
	switch mode {
	case control.ModeTUN:
		return controlv1.ConnectionMode_CONNECTION_MODE_TUN
	case control.ModeSystemProxy:
		return controlv1.ConnectionMode_CONNECTION_MODE_SYSTEM_PROXY
	case control.ModeLocalProxy:
		return controlv1.ConnectionMode_CONNECTION_MODE_LOCAL_PROXY
	default:
		return controlv1.ConnectionMode_CONNECTION_MODE_UNSPECIFIED
	}
}

var _ control.Client = (*GRPCClient)(nil)
var _ control.ConnectionOperator = (*GRPCClient)(nil)
var _ control.Watcher = (*GRPCClient)(nil)
var _ control.ProfileReader = (*GRPCClient)(nil)
var _ control.ProfileWriter = (*GRPCClient)(nil)
var _ control.LocalProfileWriter = (*GRPCClient)(nil)
var _ io.Closer = (*GRPCClient)(nil)
