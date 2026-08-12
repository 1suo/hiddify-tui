// Package control defines the frontend's stable view of the local control API.
// Generated protobuf bindings will implement this contract once the daemon
// transport is available upstream.
package control

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

const APIMajor uint32 = 1

type ConnectionState string

const (
	ConnectionStopped       ConnectionState = "stopped"
	ConnectionStarting      ConnectionState = "starting"
	ConnectionStarted       ConnectionState = "started"
	ConnectionStopping      ConnectionState = "stopping"
	ConnectionReconnectWait ConnectionState = "reconnect-wait"
	ConnectionFailed        ConnectionState = "failed"
)

type ConnectionMode string

const (
	ModeTUN         ConnectionMode = "tun"
	ModeSystemProxy ConnectionMode = "system-proxy"
	ModeLocalProxy  ConnectionMode = "local-proxy"
)

// Snapshot is the complete state a client needs before consuming events.
type Snapshot struct {
	APIMajor          uint32          `json:"api_major"`
	APIMinor          uint32          `json:"api_minor"`
	Revision          uint64          `json:"revision"`
	EventSequence     uint64          `json:"event_sequence"`
	DaemonVersion     string          `json:"daemon_version"`
	CoreVersion       string          `json:"core_version"`
	ConnectionState   ConnectionState `json:"connection_state"`
	ActiveProfileID   string          `json:"active_profile_id,omitempty"`
	ActiveProfileName string          `json:"active_profile_name,omitempty"`
	RequestedMode     string          `json:"requested_mode,omitempty"`
	EffectiveMode     string          `json:"effective_mode,omitempty"`
	SelectedOutbound  string          `json:"selected_outbound,omitempty"`
	LastError         string          `json:"last_error,omitempty"`
	Traffic           TrafficStats    `json:"traffic"`
	System            SystemStats     `json:"system"`
	Agent             AgentHealth     `json:"agent"`
	Capabilities      []string        `json:"capabilities,omitempty"`
}

type TrafficStats struct {
	UplinkBytesPerSecond   uint64 `json:"uplink_bytes_per_second"`
	DownlinkBytesPerSecond uint64 `json:"downlink_bytes_per_second"`
	TotalUploadBytes       uint64 `json:"total_upload_bytes"`
	TotalDownloadBytes     uint64 `json:"total_download_bytes"`
}

type SystemStats struct {
	MemoryBytes     uint64 `json:"memory_bytes"`
	ConnectionCount uint32 `json:"connection_count"`
}

type AgentHealth struct {
	Required  bool   `json:"required"`
	Connected bool   `json:"connected"`
	LastError string `json:"last_error,omitempty"`
}

type ProfileKind string

const (
	ProfileRemote ProfileKind = "remote"
	ProfileLocal  ProfileKind = "local"
)

// Profile contains daemon-owned profile metadata only. Remote URLs are already
// redacted by the daemon and the original URL is never available to the UI.
type Profile struct {
	ID                    string      `json:"id"`
	Name                  string      `json:"name"`
	Kind                  ProfileKind `json:"kind"`
	Active                bool        `json:"active"`
	RedactedURL           string      `json:"redacted_url,omitempty"`
	LastSuccessfulRefresh int64       `json:"last_successful_refresh_unix,omitempty"`
	LastAttemptedRefresh  int64       `json:"last_attempted_refresh_unix,omitempty"`
	UpdateIntervalSeconds int64       `json:"update_interval_seconds,omitempty"`
	Subscription          Usage       `json:"subscription"`
	LastRefreshError      string      `json:"last_refresh_error,omitempty"`
}

type Usage struct {
	UploadBytes   int64 `json:"upload_bytes"`
	DownloadBytes int64 `json:"download_bytes"`
	TotalBytes    int64 `json:"total_bytes"`
	ExpiryUnix    int64 `json:"expiry_unix,omitempty"`
}

// EventKind identifies the part of the snapshot changed by an event.
type EventKind string

const (
	EventConnection EventKind = "connection"
	EventProfile    EventKind = "profile"
	EventOutbound   EventKind = "outbound"
	EventWarning    EventKind = "warning"
	EventResync     EventKind = "resync-required"
)

// Event is delivered after a Snapshot. Sequence is monotonic per daemon
// instance; clients must resynchronize if it is not contiguous.
type Event struct {
	Sequence uint64    `json:"sequence"`
	Revision uint64    `json:"revision"`
	Kind     EventKind `json:"kind"`

	ConnectionState   ConnectionState `json:"connection_state,omitempty"`
	ActiveProfileID   string          `json:"active_profile_id,omitempty"`
	ActiveProfileName string          `json:"active_profile_name,omitempty"`
	RequestedMode     string          `json:"requested_mode,omitempty"`
	EffectiveMode     string          `json:"effective_mode,omitempty"`
	SelectedOutbound  string          `json:"selected_outbound,omitempty"`
	LastError         string          `json:"last_error,omitempty"`
}

// Client is deliberately transport-neutral so the CLI and TUI can be tested
// against a fake before the upstream daemon exposes local gRPC.
type Client interface {
	GetSnapshot(context.Context) (Snapshot, error)
}

// Watcher is implemented by a daemon client that supports the event stream.
// The stream must begin strictly after afterSequence.
type Watcher interface {
	WatchEvents(context.Context, uint64) (<-chan Event, error)
}

type ProfileReader interface {
	ListProfiles(context.Context) ([]Profile, error)
	GetProfile(context.Context, string) (Profile, error)
}

type ConnectionOperator interface {
	Connect(context.Context, string, ConnectionMode) error
	Disconnect(context.Context) error
	Restart(context.Context) error
}

type ProfileWriter interface {
	AddRemoteProfile(context.Context, string, string, bool) (Profile, error)
	UpdateProfileName(context.Context, string, string) (Profile, error)
	RefreshProfile(context.Context, string) error
	DeleteProfile(context.Context, string) error
	SetActiveProfile(context.Context, string) error
}

type LocalProfileWriter interface {
	AddLocalProfile(context.Context, string, bool, io.Reader) (Profile, error)
}

type Outbound struct {
	ID              string `json:"id"`
	Tag             string `json:"tag"`
	Protocol        string `json:"protocol"`
	Selectable      bool   `json:"selectable"`
	DelayMillis     int64  `json:"delay_millis,omitempty"`
	LastTestUnix    int64  `json:"last_test_unix,omitempty"`
	EndpointSummary string `json:"endpoint_summary,omitempty"`
}

type OutboundGroup struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	SelectedOutboundID string     `json:"selected_outbound_id,omitempty"`
	Outbounds          []Outbound `json:"outbounds"`
}

type TestScope struct {
	OutboundID string
	GroupID    string
	AllVisible bool
}

type OutboundOperator interface {
	ListOutboundGroups(context.Context) ([]OutboundGroup, error)
	SelectOutbound(context.Context, string, string) error
	TestOutbounds(context.Context, TestScope) error
}

type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

type LogEntry struct {
	Sequence      uint64   `json:"sequence"`
	TimestampUnix int64    `json:"timestamp_unix_nano"`
	Level         LogLevel `json:"level"`
	Component     string   `json:"component"`
	Message       string   `json:"message"`
}

type LogReader interface {
	TailLogs(context.Context, uint32, LogLevel, bool) (<-chan LogEntry, error)
	ClearLogs(context.Context) error
}

type Settings struct {
	RedactedJSON json.RawMessage `json:"settings"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationResult struct {
	Valid  bool         `json:"valid"`
	Errors []FieldError `json:"errors,omitempty"`
}

type SettingsOperator interface {
	GetSettings(context.Context) (Settings, error)
	ValidateSettings(context.Context, []byte) (ValidationResult, error)
	UpdateSettings(context.Context, []byte) (Settings, error)
	ResetSettings(context.Context) (Settings, error)
	ExportSettings(context.Context, bool) ([]byte, error)
	ImportSettings(context.Context, []byte) (Settings, error)
}

func (s Snapshot) ValidateCompatibility() error {
	if s.APIMajor != APIMajor {
		return fmt.Errorf("control API major version %d is incompatible with client major version %d", s.APIMajor, APIMajor)
	}
	return nil
}
