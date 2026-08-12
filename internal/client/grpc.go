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

var _ control.Client = (*GRPCClient)(nil)
var _ control.Watcher = (*GRPCClient)(nil)
var _ io.Closer = (*GRPCClient)(nil)
