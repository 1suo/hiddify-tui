package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
)

func AutoConnect(ctx context.Context, daemon interface {
	control.ServiceReader
	control.Client
}, action string, jsonOutput bool, stdout, stderr io.Writer) int {
	if action == "status" {
		snapshot, err := client.Snapshot(ctx, daemon)
		if err != nil {
			fmt.Fprintf(stderr, "autoconnect status: %v\n", err)
			return ExitUnavailable
		}
		return writeAutoConnect(snapshot.AutoConnect, jsonOutput, stdout, stderr)
	}
	enabled := action == "enable"
	if action != "enable" && action != "disable" {
		return ExitUsage
	}
	if err := daemon.SetAutoConnect(ctx, enabled); err != nil {
		fmt.Fprintf(stderr, "autoconnect %s: %v\n", action, err)
		return ExitRejected
	}
	return writeAutoConnect(enabled, jsonOutput, stdout, stderr)
}

func writeAutoConnect(enabled, jsonOutput bool, stdout, stderr io.Writer) int {
	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(struct {
			SchemaVersion uint32 `json:"schema_version"`
			Enabled       bool   `json:"enabled"`
		}{SchemaVersion: 1, Enabled: enabled}); err != nil {
			fmt.Fprintf(stderr, "autoconnect: %v\n", err)
			return ExitRejected
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "Auto-connect: %t\n", enabled)
	return ExitOK
}

func ServiceStatus(ctx context.Context, daemon control.ServiceReader, jsonOutput bool, stdout, stderr io.Writer) int {
	info, err := daemon.GetServiceInfo(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "service status: %v\n", err)
		return ExitUnavailable
	}
	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(struct {
			SchemaVersion uint32              `json:"schema_version"`
			Service       control.ServiceInfo `json:"service"`
		}{SchemaVersion: 1, Service: info}); err != nil {
			fmt.Fprintf(stderr, "service status: %v\n", err)
			return ExitRejected
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "Installed: %t\nEnabled: %t\nRunning: %t\n", info.Installed, info.Enabled, info.Running)
	if info.LastError != "" {
		fmt.Fprintf(stdout, "Last error: %s\n", info.LastError)
	}
	return ExitOK
}

func Diagnostics(ctx context.Context, daemon control.ServiceReader, jsonOutput bool, stdout, stderr io.Writer) int {
	diagnostics, err := daemon.GetDiagnostics(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "diagnostics: %v\n", err)
		return ExitUnavailable
	}
	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(struct {
			SchemaVersion uint32              `json:"schema_version"`
			Diagnostics   control.Diagnostics `json:"diagnostics"`
		}{SchemaVersion: 1, Diagnostics: diagnostics}); err != nil {
			fmt.Fprintf(stderr, "diagnostics: %v\n", err)
			return ExitRejected
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "Daemon: %s\nCore: %s\nSocket: %s\nListeners: %v\n", diagnostics.DaemonVersion, diagnostics.CoreVersion, diagnostics.SocketPath, diagnostics.ActiveListeners)
	return ExitOK
}

func AgentStatus(ctx context.Context, daemon control.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	snapshot, err := client.Snapshot(ctx, daemon)
	if err != nil {
		fmt.Fprintf(stderr, "agent status: %v\n", err)
		return ExitUnavailable
	}
	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(struct {
			SchemaVersion uint32              `json:"schema_version"`
			Agent         control.AgentHealth `json:"agent"`
		}{SchemaVersion: 1, Agent: snapshot.Agent}); err != nil {
			fmt.Fprintf(stderr, "agent status: %v\n", err)
			return ExitRejected
		}
		return ExitOK
	}
	if !snapshot.Agent.Required {
		fmt.Fprintln(stdout, "Agent: not required")
		return ExitOK
	}
	if snapshot.Agent.Connected {
		fmt.Fprintln(stdout, "Agent: connected")
		return ExitOK
	}
	fmt.Fprintln(stdout, "Agent: unavailable")
	if snapshot.Agent.LastError != "" {
		fmt.Fprintf(stdout, "Last error: %s\n", snapshot.Agent.LastError)
	}
	return ExitOK
}
