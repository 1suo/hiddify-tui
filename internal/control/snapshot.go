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
	DaemonVersion     string          `json:"daemon_version"`
	CoreVersion       string          `json:"core_version"`
	ConnectionState   ConnectionState `json:"connection_state"`
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

func (s Snapshot) ValidateCompatibility() error {
	if s.APIMajor != APIMajor {
		return fmt.Errorf("control API major version %d is incompatible with client major version %d", s.APIMajor, APIMajor)
	}
	return nil
}
