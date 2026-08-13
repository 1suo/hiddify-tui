package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/1suo/hiddify-tui/internal/client"
)

type statusOutput struct {
	State           string `json:"state"`
	Message         string `json:"message,omitempty"`
	Memory          int64  `json:"memory_bytes"`
	Uplink          int64  `json:"uplink_bytes_per_second"`
	Downlink        int64  `json:"downlink_bytes_per_second"`
	UplinkTotal     int64  `json:"total_upload_bytes"`
	DownlinkTotal   int64  `json:"total_download_bytes"`
	Connections     int32  `json:"connection_count"`
	CurrentOutbound string `json:"current_outbound,omitempty"`
	CurrentProfile  string `json:"current_profile,omitempty"`
}

func statusFrom(s client.Snapshot) statusOutput {
	return statusOutput{
		State:           string(s.State),
		Message:         s.Message,
		Memory:          s.Memory,
		Uplink:          s.Uplink,
		Downlink:        s.Downlink,
		UplinkTotal:     s.UplinkTotal,
		DownlinkTotal:   s.DownlinkTotal,
		Connections:     s.Connections,
		CurrentOutbound: s.CurrentOutbound,
		CurrentProfile:  s.CurrentProfile,
	}
}

// Status prints a one-shot core status.
func Status(ctx context.Context, core client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	snapshot, err := core.Snapshot(ctx)
	if err != nil {
		WriteError(stderr, "status", err)
		return ExitUnavailable
	}
	if jsonOutput {
		if err := writeJSON(stdout, statusFrom(snapshot)); err != nil {
			return ExitRejected
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "State:      %s\n", snapshot.State)
	fmt.Fprintf(stdout, "Profile:    %s\n", valueOr(snapshot.CurrentProfile, "none"))
	fmt.Fprintf(stdout, "Outbound:   %s\n", valueOr(snapshot.CurrentOutbound, "none"))
	fmt.Fprintf(stdout, "Down:       %d B/s\n", snapshot.Downlink)
	fmt.Fprintf(stdout, "Up:         %d B/s\n", snapshot.Uplink)
	fmt.Fprintf(stdout, "Total:      %d B down / %d B up\n", snapshot.DownlinkTotal, snapshot.UplinkTotal)
	fmt.Fprintf(stdout, "Connections %d\n", snapshot.Connections)
	fmt.Fprintf(stdout, "Memory:     %d B\n", snapshot.Memory)
	if snapshot.Message != "" {
		fmt.Fprintf(stdout, "Message:    %s\n", snapshot.Message)
	}
	return ExitOK
}

// StatusWatch streams status snapshots as JSON Lines (or human lines).
func StatusWatch(ctx context.Context, core client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	updates, err := core.WatchStatus(ctx)
	if err != nil {
		WriteError(stderr, "status", err)
		return ExitUnavailable
	}
	for snapshot := range updates {
		if jsonOutput {
			if err := json.NewEncoder(stdout).Encode(statusFrom(snapshot)); err != nil {
				return ExitRejected
			}
			continue
		}
		fmt.Fprintf(stdout, "state=%s profile=%s down=%d up=%d outbound=%s\n",
			snapshot.State, valueOr(snapshot.CurrentProfile, "none"), snapshot.Downlink, snapshot.Uplink, valueOr(snapshot.CurrentOutbound, "none"))
	}
	return ExitOK
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
