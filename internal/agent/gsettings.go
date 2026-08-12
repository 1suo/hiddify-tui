package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

var gsettingsSchemas = []string{
	"org.gnome.system.proxy",
	"org.gnome.system.proxy.http",
	"org.gnome.system.proxy.https",
	"org.gnome.system.proxy.ftp",
	"org.gnome.system.proxy.socks",
}

// CommandRunner makes the native GSettings boundary deterministic in tests.
type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// GSettingsBackend reads and writes every value in GNOME's proxy schemas. The
// values are saved as the CLI's native literals so a restore reproduces the
// original desktop state instead of inferring a partial proxy configuration.
type GSettingsBackend struct {
	runner CommandRunner
}

func NewGSettingsBackend() *GSettingsBackend {
	return &GSettingsBackend{runner: execRunner{}}
}

type gsettingsState struct {
	Version uint32           `json:"version"`
	Values  []gsettingsValue `json:"values"`
}

type gsettingsValue struct {
	Schema string `json:"schema"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

func (b *GSettingsBackend) Current(ctx context.Context) (ProxyState, error) {
	var values []gsettingsValue
	for _, schema := range gsettingsSchemas {
		output, err := b.runner.Run(ctx, "gsettings", "list-recursively", schema)
		if err != nil {
			return nil, fmt.Errorf("read GNOME proxy settings: %w", err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, " ", 3)
			if len(parts) != 3 || parts[0] != schema || parts[1] == "" || parts[2] == "" {
				return nil, fmt.Errorf("parse GNOME proxy setting %q", line)
			}
			values = append(values, gsettingsValue{Schema: parts[0], Key: parts[1], Value: parts[2]})
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("GNOME proxy schemas returned no settings")
	}
	state, err := json.Marshal(gsettingsState{Version: 1, Values: values})
	if err != nil {
		return nil, err
	}
	return ProxyState(state), nil
}

func (b *GSettingsBackend) Apply(ctx context.Context, state ProxyState) error {
	var parsed gsettingsState
	if err := json.Unmarshal(state, &parsed); err != nil {
		return fmt.Errorf("decode GNOME proxy state: %w", err)
	}
	if parsed.Version != 1 || len(parsed.Values) == 0 {
		return fmt.Errorf("unsupported GNOME proxy state")
	}
	for _, value := range parsed.Values {
		if !isGSettingsSchema(value.Schema) || value.Key == "" || value.Value == "" {
			return fmt.Errorf("invalid GNOME proxy setting")
		}
		if _, err := b.runner.Run(ctx, "gsettings", "set", value.Schema, value.Key, value.Value); err != nil {
			return fmt.Errorf("set GNOME proxy setting %s %s: %w", value.Schema, value.Key, err)
		}
	}
	return nil
}

func isGSettingsSchema(schema string) bool {
	for _, candidate := range gsettingsSchemas {
		if schema == candidate {
			return true
		}
	}
	return false
}

// DesiredGSettingsProxy constructs the minimum GNOME settings required for a
// loopback HTTP proxy. The manager still captures every existing schema value
// before applying this partial desired state, making restore exact.
func DesiredGSettingsProxy(host string, port uint32) ProxyState {
	state, _ := json.Marshal(gsettingsState{Version: 1, Values: []gsettingsValue{
		{Schema: "org.gnome.system.proxy", Key: "mode", Value: "'manual'"},
		{Schema: "org.gnome.system.proxy", Key: "use-same-proxy", Value: "true"},
		{Schema: "org.gnome.system.proxy.http", Key: "host", Value: "'" + host + "'"},
		{Schema: "org.gnome.system.proxy.http", Key: "port", Value: fmt.Sprintf("uint32 %d", port)},
	}})
	return ProxyState(state)
}
