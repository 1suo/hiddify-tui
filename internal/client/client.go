// Package client is the thin boundary between the frontend and the running
// hiddify-core. It talks only to the core's existing Core gRPC service.
package client

import (
	"context"
	"time"
)

// ConnectionState is derived from the core's CoreStates.
type ConnectionState string

const (
	StateStopped  ConnectionState = "stopped"
	StateStarting ConnectionState = "starting"
	StateStarted  ConnectionState = "started"
	StateStopping ConnectionState = "stopping"
)

// Snapshot is the one-shot view a client renders from the core.
type Snapshot struct {
	State           ConnectionState
	Message         string
	Memory          int64
	Uplink          int64
	Downlink        int64
	UplinkTotal     int64
	DownlinkTotal   int64
	Connections     int32
	CurrentOutbound string
	CurrentProfile  string
}

// Outbound is a single outbound (or group member) as reported by the core.
type Outbound struct {
	Tag         string
	Type        string
	DelayMillis int64
	Selected    bool
	IsSecure    bool
	Host        string
	Port        uint32
}

// OutboundGroup is a selector/urltest group with its members.
type OutboundGroup struct {
	Tag        string
	Type       string
	Selected   string
	Selectable bool
	Items      []Outbound
}

// LogLevel filters the core's log stream.
type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

// LogEntry is a single structured log line from the core.
type LogEntry struct {
	Level     LogLevel
	Component string
	Message   string
	Time      time.Time
}

// Client is the frontend's view of the core's control surface.
type Client interface {
	Connect(ctx context.Context, content, name string) error
	Disconnect(ctx context.Context) error
	Restart(ctx context.Context, content, name string) error
	Snapshot(ctx context.Context) (Snapshot, error)
	WatchStatus(ctx context.Context) (<-chan Snapshot, error)
	OutboundGroups(ctx context.Context) ([]OutboundGroup, error)
	SelectOutbound(ctx context.Context, group, outbound string) error
	TestOutbound(ctx context.Context, tag string) error
	Parse(ctx context.Context, content string) error
	ChangeSettings(ctx context.Context, json string) error
	WatchLogs(ctx context.Context, level LogLevel) (<-chan LogEntry, error)
	Close() error
}
