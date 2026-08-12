// Package control defines the frontend's stable view of the local control API.
// Generated protobuf bindings will implement this contract once the daemon
// transport is available upstream.
package control

import (
	"context"
	"fmt"
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

func (s Snapshot) ValidateCompatibility() error {
	if s.APIMajor != APIMajor {
		return fmt.Errorf("control API major version %d is incompatible with client major version %d", s.APIMajor, APIMajor)
	}
	return nil
}
