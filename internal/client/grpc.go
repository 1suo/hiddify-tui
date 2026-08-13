package client

import (
	"context"
	"fmt"
	"time"

	"github.com/1suo/hiddify-tui/gen/hcommon"
	"github.com/1suo/hiddify-tui/gen/hcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DefaultAddress is where the core serves its insecure Core gRPC service when
// run headless (HiddifyCli run) or by the GUI.
const DefaultAddress = "127.0.0.1:17078"

// GRPCClient is the gRPC implementation of Client over the core's Core service.
type GRPCClient struct {
	conn *grpc.ClientConn
	api  hcore.CoreClient
}

// Dial connects to the core's Core gRPC service. The transport is plain TCP
// with no TLS, matching the core's SetupMode_GRPC_*_INSECURE modes.
func Dial(ctx context.Context, address string) (*GRPCClient, error) {
	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to core at %s: %w", address, err)
	}
	return &GRPCClient{conn: conn, api: hcore.NewCoreClient(conn)}, nil
}

func (c *GRPCClient) Close() error { return c.conn.Close() }

func (c *GRPCClient) Connect(ctx context.Context, content, name string) error {
	response, err := c.api.Start(ctx, &hcore.StartRequest{ConfigContent: content, ConfigName: name})
	if err != nil {
		return err
	}
	return operationError(response)
}

func (c *GRPCClient) Disconnect(ctx context.Context) error {
	response, err := c.api.Stop(ctx, &hcommon.Empty{})
	if err != nil {
		return err
	}
	return operationError(response)
}

func (c *GRPCClient) Restart(ctx context.Context, content, name string) error {
	response, err := c.api.Restart(ctx, &hcore.StartRequest{ConfigContent: content, ConfigName: name})
	if err != nil {
		return err
	}
	return operationError(response)
}

func (c *GRPCClient) Snapshot(ctx context.Context) (Snapshot, error) {
	info, err := c.api.GetSystemInfo(ctx, &hcommon.Empty{})
	if err != nil {
		return Snapshot{}, err
	}
	state, message := StateStopped, ""
	if stream, err := c.api.CoreInfoListener(ctx, &hcommon.Empty{}); err == nil {
		if first, err := stream.Recv(); err == nil {
			state, message = stateFromCore(first)
		}
	}
	return snapshotFromParts(state, message, info), nil
}

// WatchStatus merges the core's state and system-info streams into one snapshot
// channel. The caller cancels via ctx.
func (c *GRPCClient) WatchStatus(ctx context.Context) (<-chan Snapshot, error) {
	stateStream, err := c.api.CoreInfoListener(ctx, &hcommon.Empty{})
	if err != nil {
		return nil, err
	}
	infoStream, err := c.api.GetSystemInfoStream(ctx, &hcommon.Empty{})
	if err != nil {
		return nil, err
	}

	out := make(chan Snapshot, 1)
	state := StateStopped
	message := ""
	var info *hcore.SystemInfo

	emit := func() {
		snapshot := Snapshot{State: state, Message: message}
		if info != nil {
			snapshot.Memory = info.GetMemory()
			snapshot.Uplink = info.GetUplink()
			snapshot.Downlink = info.GetDownlink()
			snapshot.UplinkTotal = info.GetUplinkTotal()
			snapshot.DownlinkTotal = info.GetDownlinkTotal()
			snapshot.Connections = info.GetConnectionsIn() + info.GetConnectionsOut()
			snapshot.CurrentOutbound = info.GetCurrentOutbound()
			snapshot.CurrentProfile = info.GetCurrentProfile()
		}
		select {
		case out <- snapshot:
		default:
		}
	}

	go func() {
		defer close(out)
		for {
			entry, err := stateStream.Recv()
			if err != nil {
				return
			}
			state, message = stateFromCore(entry)
			emit()
		}
	}()
	go func() {
		for {
			entry, err := infoStream.Recv()
			if err != nil {
				return
			}
			info = entry
			emit()
		}
	}()

	return out, nil
}

func (c *GRPCClient) OutboundGroups(ctx context.Context) ([]OutboundGroup, error) {
	stream, err := c.api.OutboundsInfo(ctx, &hcommon.Empty{})
	if err != nil {
		return nil, err
	}
	list, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	return groupsFromCore(list), nil
}

func (c *GRPCClient) SelectOutbound(ctx context.Context, group, outbound string) error {
	response, err := c.api.SelectOutbound(ctx, &hcore.SelectOutboundRequest{GroupTag: group, OutboundTag: outbound})
	if err != nil {
		return err
	}
	if response.GetCode() != hcommon.ResponseCode_OK {
		return fmt.Errorf("select outbound: %s", response.GetMessage())
	}
	return nil
}

func (c *GRPCClient) TestOutbound(ctx context.Context, tag string) error {
	response, err := c.api.UrlTest(ctx, &hcore.UrlTestRequest{Tag: tag})
	if err != nil {
		return err
	}
	if response.GetCode() != hcommon.ResponseCode_OK {
		return fmt.Errorf("url test: %s", response.GetMessage())
	}
	return nil
}

func (c *GRPCClient) Parse(ctx context.Context, content string) error {
	response, err := c.api.Parse(ctx, &hcore.ParseRequest{Content: content})
	if err != nil {
		return err
	}
	if response.GetResponseCode() != hcommon.ResponseCode_OK {
		return fmt.Errorf("%s", response.GetMessage())
	}
	return nil
}

func (c *GRPCClient) ChangeSettings(ctx context.Context, json string) error {
	if _, err := c.api.ChangeHiddifySettings(ctx, &hcore.ChangeHiddifySettingsRequest{HiddifySettingsJson: json}); err != nil {
		return err
	}
	return nil
}

// WatchLogs streams the core's bounded log output. The caller cancels via ctx.
func (c *GRPCClient) WatchLogs(ctx context.Context, level LogLevel) (<-chan LogEntry, error) {
	stream, err := c.api.LogListener(ctx, &hcore.LogRequest{Level: logLevelToCore(level)})
	if err != nil {
		return nil, err
	}
	out := make(chan LogEntry)
	go func() {
		defer close(out)
		for {
			entry, err := stream.Recv()
			if err != nil {
				return
			}
			select {
			case out <- logFromCore(entry):
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func operationError(response *hcore.CoreInfoResponse) error {
	if response.GetCoreState() == hcore.CoreStates_STARTED || response.GetCoreState() == hcore.CoreStates_STOPPED {
		return nil
	}
	// A non-terminal state with a message means the operation did not succeed
	// (e.g. already-started/stopped is informational).
	switch response.GetMessageType() {
	case hcore.MessageType_ALREADY_STARTED, hcore.MessageType_ALREADY_STOPPED, hcore.MessageType_EMPTY:
		return nil
	}
	return fmt.Errorf("%s", response.GetMessage())
}

func stateFromCore(response *hcore.CoreInfoResponse) (ConnectionState, string) {
	state := StateStopped
	switch response.GetCoreState() {
	case hcore.CoreStates_STARTING:
		state = StateStarting
	case hcore.CoreStates_STARTED:
		state = StateStarted
	case hcore.CoreStates_STOPPING:
		state = StateStopping
	}
	return state, response.GetMessage()
}

func snapshotFromParts(state ConnectionState, message string, info *hcore.SystemInfo) Snapshot {
	snapshot := Snapshot{State: state, Message: message}
	snapshot.Memory = info.GetMemory()
	snapshot.Uplink = info.GetUplink()
	snapshot.Downlink = info.GetDownlink()
	snapshot.UplinkTotal = info.GetUplinkTotal()
	snapshot.DownlinkTotal = info.GetDownlinkTotal()
	snapshot.Connections = info.GetConnectionsIn() + info.GetConnectionsOut()
	snapshot.CurrentOutbound = info.GetCurrentOutbound()
	snapshot.CurrentProfile = info.GetCurrentProfile()
	return snapshot
}

func groupsFromCore(list *hcore.OutboundGroupList) []OutboundGroup {
	groups := make([]OutboundGroup, 0, len(list.GetItems()))
	for _, group := range list.GetItems() {
		converted := OutboundGroup{Tag: group.GetTag(), Type: group.GetType(), Selected: group.GetSelected(), Selectable: group.GetSelectable()}
		for _, item := range group.GetItems() {
			converted.Items = append(converted.Items, Outbound{
				Tag: item.GetTag(), Type: item.GetType(), DelayMillis: int64(item.GetUrlTestDelay()),
				Selected: item.GetIsSelected(), IsSecure: item.GetIsSecure(), Host: item.GetHost(), Port: item.GetPort(),
			})
		}
		groups = append(groups, converted)
	}
	return groups
}

func logFromCore(entry *hcore.LogMessage) LogEntry {
	var timestamp time.Time
	if entry.GetTime() != nil {
		timestamp = entry.GetTime().AsTime()
	} else {
		timestamp = time.Now()
	}
	return LogEntry{Level: logLevelFromCore(entry.GetLevel()), Component: entry.GetType().String(), Message: entry.GetMessage(), Time: timestamp}
}

func logLevelToCore(level LogLevel) hcore.LogLevel {
	switch level {
	case LogDebug:
		return hcore.LogLevel_DEBUG
	case LogWarn:
		return hcore.LogLevel_WARNING
	case LogError:
		return hcore.LogLevel_ERROR
	default:
		return hcore.LogLevel_INFO
	}
}

func logLevelFromCore(level hcore.LogLevel) LogLevel {
	switch level {
	case hcore.LogLevel_DEBUG, hcore.LogLevel_TRACE:
		return LogDebug
	case hcore.LogLevel_WARNING:
		return LogWarn
	case hcore.LogLevel_ERROR, hcore.LogLevel_FATAL:
		return LogError
	default:
		return LogInfo
	}
}
