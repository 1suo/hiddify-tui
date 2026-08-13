// Package cli implements the noninteractive command surface and its stable
// output contract.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/profile"
)

// Exit codes are part of the scriptable contract.
const (
	ExitOK          = 0
	ExitUsage       = 2
	ExitUnavailable = 3
	ExitRejected    = 4
	ExitPrivilege   = 5
)

// Dial opens the core client with the configured timeout.
func Dial(address string, timeout time.Duration) (*client.GRPCClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return client.Dial(ctx, address)
}

// ProfileStore opens the client-side profile store.
func ProfileStore(path string) (*profile.Store, error) {
	return profile.Open(path)
}

// writeJSON emits a stable JSON envelope with the schema version.
func writeJSON(stdout io.Writer, value any) error {
	return json.NewEncoder(stdout).Encode(struct {
		SchemaVersion uint32 `json:"schema_version"`
		Result        any    `json:"result"`
	}{SchemaVersion: 1, Result: value})
}

func WriteError(stderr io.Writer, command string, err error) {
	fmt.Fprintf(stderr, "%s: %v\n", command, err)
}

// redactURL strips credential-bearing query parameters for display.
func redactURL(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := parsed.Query()
	for key := range query {
		query.Set(key, "[redacted]")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func nowUnix() int64 {
	return time.Now().Unix()
}
