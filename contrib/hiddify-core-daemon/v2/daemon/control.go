package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	controlv1 "github.com/hiddify/hiddify-core/v2/controlv1"
	"github.com/hiddify/hiddify-core/v2/config"
	"github.com/hiddify/hiddify-core/v2/db"
	hcommon "github.com/hiddify/hiddify-core/v2/hcommon"
	HC "github.com/hiddify/hiddify-core/v2/hcommon/constants"
	hcore "github.com/hiddify/hiddify-core/v2/hcore"
	profiles "github.com/hiddify/hiddify-core/v2/profile"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const controlAPIMajor = 1
const maxLocalProfileBytes = 8 << 20
const eventQueueSize = 32
const maxLogEntries = 1000
const defaultRefreshInterval = 24 * time.Hour
const maxRefreshBackoff = 6 * time.Hour
const eventQueueSize = 32

// ControlServer is the daemon-owned local control surface. It starts with a
// stopped snapshot and gains core-backed operations incrementally; unsupported
// requests use the generated gRPC Unimplemented response rather than silently
// reporting success.
type ControlServer struct {
	controlv1.UnimplementedControlServiceServer
	mu          sync.RWMutex
	operationMu sync.Mutex
	snapshot    *controlv1.Snapshot
	watchers    map[chan *controlv1.Event]struct{}
}

func TestStartFailureCode(t *testing.T) {
	if code := startFailureCode(fmt.Errorf("listen tcp 127.0.0.1:1234: bind: address already in use")); code != controlv1.ErrorCode_ERROR_CODE_PORT_CONFLICT {
		t.Fatalf("port conflict code = %v", code)
	}
	if code := startFailureCode(fmt.Errorf("invalid config")); code != controlv1.ErrorCode_ERROR_CODE_INVALID_CONFIGURATION {
		t.Fatalf("invalid configuration code = %v", code)
	}
}

func errOrResponse(err error, response *hcore.CoreInfoResponse) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("%s", response.GetMessage())
}

func (s *ControlServer) recordRefreshFailure(id string, now time.Time) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	retry := s.refreshRetry[id]
	if retry.attempts < 5 {
		retry.attempts++
	}
	delay := time.Minute << (retry.attempts - 1)
	if delay > maxRefreshBackoff {
		delay = maxRefreshBackoff
	}
	retry.next = now.Add(delay)
	s.refreshRetry[id] = retry
}

func (s *ControlServer) clearRefreshFailure(id string) {
	s.refreshMu.Lock()
	delete(s.refreshRetry, id)
	s.refreshMu.Unlock()
}

func redactValidationError(err error) string {
	message := err.Error()
	if len(message) > 240 {
		message = message[:240] + "…"
	}
	return strings.ReplaceAll(message, "\n", " ")
}

func (s *ControlServer) agentConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot.GetAgent().GetConnected()
}

func (s *ControlServer) PollAgent(_ context.Context, request *controlv1.AgentPollRequest) (*controlv1.AgentInstruction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Agent = &controlv1.AgentHealth{Required: s.snapshot.GetRequestedMode() == "system-proxy", Connected: request.GetLastError() == "", LastError: request.GetLastError()}
	if s.snapshot.GetRequestedMode() != "system-proxy" || s.snapshot.GetConnectionState() != controlv1.ConnectionState_CONNECTION_STATE_STARTED {
		return &controlv1.AgentInstruction{}, nil
	}
	return &controlv1.AgentInstruction{SystemProxyEnabled: true, Host: "127.0.0.1", Port: hcore.LocalProxyPort(), LeaseSeconds: 45}, nil
}

func (s *ControlServer) startLogCapture() {
	updates, stop := hcore.SubscribeLogs(maxLogEntries)
	s.logStop = stop
	go func() {
		for entry := range updates {
			s.appendLog(&controlv1.LogEntry{
				TimestampUnixNano: entry.GetTime().AsTime().UnixNano(), Level: controlLogLevel(entry.GetLevel()),
				Component: entry.GetType().String(), Message: entry.GetMessage(),
			})
		}
	}()
}

func (s *ControlServer) Close() {
	if s.logStop != nil {
		s.logStop()
		s.logStop = nil
	}
	s.logMu.Lock()
	for updates := range s.logWatchers {
		close(updates)
	}
	s.logWatchers = nil
	s.logMu.Unlock()
}

func (s *ControlServer) appendLog(entry *controlv1.LogEntry) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.logSequence++
	entry.Sequence = s.logSequence
	s.logs = append(s.logs, entry)
	if len(s.logs) > maxLogEntries {
		copy(s.logs, s.logs[len(s.logs)-maxLogEntries:])
		s.logs = s.logs[:maxLogEntries]
	}
	for updates := range s.logWatchers {
		select {
		case updates <- entry:
		default:
			delete(s.logWatchers, updates)
			close(updates)
		}
	}
}

func (s *ControlServer) logSnapshot(tail uint32, minimum controlv1.LogLevel, follow bool) ([]*controlv1.LogEntry, chan *controlv1.LogEntry) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	entries := make([]*controlv1.LogEntry, 0, len(s.logs))
	for _, entry := range s.logs {
		if entry.GetLevel() >= minimum {
			entries = append(entries, entry)
		}
	}
	if tail < uint32(len(entries)) {
		entries = entries[len(entries)-int(tail):]
	}
	if !follow {
		return entries, nil
	}
	updates := make(chan *controlv1.LogEntry, eventQueueSize)
	s.logWatchers[updates] = struct{}{}
	return entries, updates
}

func (s *ControlServer) unsubscribeLogs(updates chan *controlv1.LogEntry) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	if _, ok := s.logWatchers[updates]; ok {
		delete(s.logWatchers, updates)
		close(updates)
	}
}

func controlLogLevel(level hcore.LogLevel) controlv1.LogLevel {
	switch level {
	case hcore.LogLevel_TRACE, hcore.LogLevel_DEBUG:
		return controlv1.LogLevel_LOG_LEVEL_DEBUG
	case hcore.LogLevel_INFO:
		return controlv1.LogLevel_LOG_LEVEL_INFO
	case hcore.LogLevel_WARNING:
		return controlv1.LogLevel_LOG_LEVEL_WARN
	default:
		return controlv1.LogLevel_LOG_LEVEL_ERROR
	}
}

func (s *ControlServer) TailLogs(request *controlv1.TailLogsRequest, stream grpc.ServerStreamingServer[controlv1.LogEntry]) error {
	entries, updates := s.logSnapshot(request.GetInitialTail(), request.GetMinimumLevel(), request.GetFollow())
	if updates != nil {
		defer s.unsubscribeLogs(updates)
	}
	for _, entry := range entries {
		if err := stream.Send(entry); err != nil {
			return err
		}
	}
	if updates == nil {
		return nil
	}
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case entry, ok := <-updates:
			if !ok {
				return nil
			}
			if entry.GetLevel() < request.GetMinimumLevel() {
				continue
			}
			if err := stream.Send(entry); err != nil {
				return err
			}
		}
	}
}

func (s *ControlServer) ClearLogs(context.Context, *controlv1.ClearLogsRequest) (*controlv1.OperationResult, error) {
	s.logMu.Lock()
	s.logs = nil
	s.logMu.Unlock()
	return operation(controlv1.ErrorCode_ERROR_CODE_OK, "logs cleared"), nil
}

func (s *ControlServer) subscribe(after uint64) (chan *controlv1.Event, *controlv1.Event) {
	updates := make(chan *controlv1.Event, eventQueueSize)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchers[updates] = struct{}{}
	if after != 0 && after < s.snapshot.GetEventSequence() {
		return updates, &controlv1.Event{
			Sequence: s.snapshot.GetEventSequence(), Revision: s.snapshot.GetRevision(),
			Change: &controlv1.Event_ResyncRequired{ResyncRequired: &controlv1.ResyncRequired{}},
		}
	}
	return updates, nil
}

func (s *ControlServer) unsubscribe(updates chan *controlv1.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.watchers[updates]; ok {
		delete(s.watchers, updates)
		close(updates)
	}
}

// publishLocked must be called with s.mu held.
func (s *ControlServer) publishLocked(change controlv1.IsEvent_Change) {
	s.snapshot.EventSequence++
	update := &controlv1.Event{Sequence: s.snapshot.GetEventSequence(), Revision: s.snapshot.GetRevision(), Change: change}
	for updates := range s.watchers {
		select {
		case updates <- update:
		default:
			delete(s.watchers, updates)
			close(updates)
		}
	}
}

func autoConnectEnabled() bool {
	setting, err := db.GetTable[hcommon.AppSettings]().Get("daemon.auto_connect")
	if err != nil || setting == nil {
		return false
	}
	value, ok := setting.Value.(string)
	if !ok {
		return false
	}
	enabled, _ := strconv.ParseBool(value)
	return enabled
}

func (s *ControlServer) subscribe(after uint64) (chan *controlv1.Event, *controlv1.Event) {
	updates := make(chan *controlv1.Event, eventQueueSize)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchers[updates] = struct{}{}
	if after != 0 && after < s.snapshot.GetEventSequence() {
		return updates, &controlv1.Event{
			Sequence: s.snapshot.GetEventSequence(), Revision: s.snapshot.GetRevision(),
			Change: &controlv1.Event_ResyncRequired{ResyncRequired: &controlv1.ResyncRequired{}},
		}
	}
	return updates, nil
}

func (s *ControlServer) unsubscribe(updates chan *controlv1.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.watchers[updates]; ok {
		delete(s.watchers, updates)
		close(updates)
	}
}

// publishLocked must be called with s.mu held.
func (s *ControlServer) publishLocked(update *controlv1.Event) {
	s.snapshot.EventSequence++
	update.Sequence = s.snapshot.GetEventSequence()
	update.Revision = s.snapshot.GetRevision()
	for updates := range s.watchers {
		select {
		case updates <- update:
		default:
			delete(s.watchers, updates)
			close(updates)
		}
	}
}

func (s *ControlServer) GetSettings(context.Context, *controlv1.GetSettingsRequest) (*controlv1.Settings, error) {
	encoded, err := currentSettingsJSON()
	if err != nil {
		return nil, status.Error(codes.Internal, "load settings")
	}
	return &controlv1.Settings{RedactedJson: redactSettings(encoded)}, nil
}

func (s *ControlServer) ValidateSettings(_ context.Context, request *controlv1.ValidateSettingsRequest) (*controlv1.ValidationResult, error) {
	if err := validateSettings(request.GetCandidateJson()); err != nil {
		return &controlv1.ValidationResult{Errors: []*controlv1.FieldError{{Field: "$", Message: err.Error()}}}, nil
	}
	return &controlv1.ValidationResult{Valid: true}, nil
}

func (s *ControlServer) UpdateSettings(_ context.Context, request *controlv1.UpdateSettingsRequest) (*controlv1.Settings, error) {
	if err := validateSettings(request.GetCandidateJson()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid settings")
	}
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	if _, err := hcore.ChangeHiddifySettings(&hcore.ChangeHiddifySettingsRequest{HiddifySettingsJson: string(request.GetCandidateJson())}, true); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid settings")
	}
	return &controlv1.Settings{RedactedJson: redactSettings(request.GetCandidateJson())}, nil
}

func (s *ControlServer) ResetSettings(_ con<truncated omitted_approx_tokens=

func (s *ControlServer) Connect(ctx context.Context, request *controlv1.ConnectRequest) (*controlv1.OperationResult, error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	entity, result := s.profileForConnect(request.GetProfileId())
	if result != nil {
		return result, nil
	}
	requestedMode := controlModeName(request.GetMode())
	if requestedMode == "" {
		return operation(controlv1.ErrorCode_ERROR_CODE_INVALID_CONFIGURATION, "unsupported connection mode"), nil
	}
	if requestedMode == "system-proxy" && !s.agentConnected() {
		return operation(controlv1.ErrorCode_ERROR_CODE_AGENT_UNAVAILABLE, "system-proxy mode requires a connected session agent"), nil
	}
	if err := hcore.SetConnectionMode(requestedMode); err != nil {
		return operation(controlv1.ErrorCode_ERROR_CODE_INTERNAL, "configure connection mode"), nil
	}
	s.setConnection(controlv1.ConnectionState_CONNECTION_STATE_STARTING, requestedMode, "")
	response, err := hcore.Start(ctx, &hcore.StartRequest{ConfigPath: filepath.Join("data", "profiles", entity.GetId()+".info"), ConfigName: entity.GetName()})
	if err != nil || response.GetCoreState() != hcore.CoreStates_STARTED {
		if response.GetMessageType() == hcore.MessageType_ALREADY_STARTED {
			s.setConnection(controlv1.ConnectionState_CONNECTION_STATE_STARTED, requestedMode, "")
			return &controlv1.OperationResult{ErrorCode: controlv1.ErrorCode_ERROR_CODE_ALREADY_IN_REQUESTED_STATE, Message: "already connected", AlreadyInRequestedState: true}, nil
		}
		s.setConnection(controlv1.Connection<truncated omitted_approx_tokens=

func NewControlServer() *ControlServer {
	return &ControlServer{watchers: make(map[chan *controlv1.Event]struct{}), snapshot: &controlv1.Snapshot{
		ApiMajor:        controlAPIMajor,
		ConnectionState: controlv1.ConnectionState_CONNECTION_STATE_STOPPED,
		Capabilities:    []string{"daemon-lifecycle", "profiles"},
		AutoConnect:     autoConnectEnabled(),
	}}
	server.startLogCapture()
	return server
}

// StartProfileScheduler refreshes remote profiles at their explicit interval.
// Failed refreshes leave the prior validated content intact and retry with a
// capped backoff; it never blocks serving control requests.
func (s *ControlServer) StartProfileScheduler(ctx context.Context) {
	if s.refreshStop != nil {
		return
	}
	ctx, s.refreshStop = context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if err := s.refreshDueProfiles(ctx); err != nil {
				hcore.Log(hcore.LogLevel_WARNING, hcore.LogType_CORE, "profile refresh scheduler: ", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *ControlServer) refreshDueProfiles(_ context.Context) error {
	all, err := profiles.GetAll()
	if err != nil {
		return err
	}
	now := time.Now()
	for _, profile := range all {
		if profile.GetUrl() == "" || !s.profileRefreshDue(profile, now) {
			continue
		}
		s.clearRefreshFailure(profile.GetId())
		if err := profiles.UpdateSubscription(profile, true); err != nil {
			s.recordRefreshFailure(profile.GetId(), now)
			hcore.Log(hcore.LogLevel_WARNING, hcore.LogType_CORE, "profile refresh failed for ", profile.GetId(), ": ", err)
			continue
		}
	}
	return nil
}

type refreshRetry struct {
	next     time.Time
	attempts uint
}

func (s *ControlServer) profileRefreshDue(profile *profiles.ProfileEntity, now time.Time) bool {
	s.refreshMu.Lock()
	retry := s.refreshRetry[profile.GetId()]
	s.refreshMu.Unlock()
	if !retry.next.IsZero() {
		return !now.Before(retry.next)
	}
	interval := defaultRefreshInterval
	if options := profile.GetOptions(); options != nil && options.GetUpdateInterval() > 0 {
		interval = time.Duration(options.GetUpdateInterval()) * time.Millisecond
	}
	if interval > maxRefreshBackoff {
		interval = maxRefreshBackoff
	}
	return profile.GetLastUpdate() == 0 || now.Sub(time.UnixMilli(profile.GetLastUpdate())) >= interval
}

func (s *ControlServer) GetSnapshot(context.Context, *controlv1.GetSnapshotRequest) (*controlv1.Snapshot, error) {
	s.refreshSystemStats()
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSnapshot(s.snapshot), nil
}

func (s *ControlServer) GetServiceInfo(context.Context, *controlv1.GetServiceInfoRequest) (*controlv1.ServiceInfo, error) {
	return &controlv1.ServiceInfo{Running: true}, nil
}

func (s *ControlServer) GetDiagnostics(context.Context, *controlv1.GetDiagnosticsRequest) (*controlv1.Diagnostics, error) {
	s.mu.RLock()
	socket := s.socket
	s.mu.RUnlock()
	return &controlv1.Diagnostics{
		DaemonVersion: HC.Version,
		CoreVersion:   HC.Version,
		SocketPath:    socket,
		ActiveListeners: []string{
			"unix://" + socket,
			"runtime=" + runtime.GOOS + "/" + runtime.GOARCH,
		},
	}, nil
}

func (s *ControlServer) refreshSystemStats() {
	stats := hcore.CurrentSystemInfo()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Traffic = &controlv1.TrafficStats{
		UplinkBytesPerSecond: uint64(maxInt64(stats.GetUplink())), DownlinkBytesPerSecond: uint64(maxInt64(stats.GetDownlink())),
		TotalUploadBytes: uint64(maxInt64(stats.GetUplinkTotal())), TotalDownloadBytes: uint64(maxInt64(stats.GetDownlinkTotal())),
	}
	s.snapshot.System = &controlv1.SystemStats{MemoryBytes: uint64(maxInt64(stats.GetMemory())), ConnectionCount: uint32(maxInt64(int64(stats.GetConnectionsIn() + stats.GetConnectionsOut())))}
	if stats.GetCurrentOutbound() != "" {
		s.snapshot.SelectedOutbound = stats.GetCurrentOutbound()
	}
}

func maxInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

// WatchEvents delivers revisioned changes after a client snapshot. Slow
// subscribers are disconnected rather than allowed to block daemon work.
func (s *ControlServer) WatchEvents(request *controlv1.WatchRequest, stream grpc.ServerStreamingServer[controlv1.Event]) error {
	updates, initial := s.subscribe(request.GetAfterSequence())
	defer s.unsubscribe(updates)
	if initial != nil {
		if err := stream.Send(initial); err != nil {
			return err
		}
	}
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if err := stream.Send(update); err != nil {
				return err
			}
		}
	}
}

func (s *ControlServer) ListProfiles(context.Context, *controlv1.ListProfilesRequest) (*controlv1.ListProfilesResponse, error) {
	all, err := profiles.GetAll()
	if err != nil {
		return nil, status.Error(codes.Internal, "load profiles")
	}
	activeID := s.activeProfileID()
	result := &controlv1.ListProfilesResponse{Profiles: make([]*controlv1.Profile, 0, len(all))}
	for _, entity := range all {
		result.Profiles = append(result.Profiles, profileToControl(entity, entity.GetId() == activeID))
	}
	return result, nil
}

func (s *ControlServer) GetProfile(_ context.Context, request *controlv1.GetProfileRequest) (*controlv1.Profile, error) {
	if request.GetProfileId() == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}
	entity, err := profiles.GetById(request.GetProfileId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	return profileToCon<truncated omitted_approx_tokens=

func cloneSnapshot(snapshot *controlv1.Snapshot) *controlv1.Snapshot {
	return proto.Clone(snapshot).(*controlv1.Snapshot)
}

// ServeControl runs control.v1 on the runtime's Unix listener until ctx ends.
func (r *Runtime) ServeControl(ctx context.Context, service *ControlServer) error {
	defer service.Close()
	service.mu.Lock()
	service.socket = r.socket
	service.mu.Unlock()
	server := grpc.NewServer()
	controlv1.RegisterControlServiceServer(server, service)
	errs := make(chan error, 1)
	go func() { errs <- s<truncated omitted_approx_tokens=
