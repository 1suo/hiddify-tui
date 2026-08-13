package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseNetworksetupProxy(t *testing.T) {
	output := "Enabled: Yes\nServer: 127.0.0.1\nPort: 8080\nAuthenticated Proxy Enabled: 0\n"
	proxy, err := parseNetworksetupProxy(output)
	if err != nil {
		t.Fatal(err)
	}
	if !proxy.Enabled || proxy.Server != "127.0.0.1" || proxy.Port != "8080" || proxy.Authenticated {
		t.Fatalf("parsed proxy = %#v", proxy)
	}
}

func TestParseNetworksetupProxyDisabled(t *testing.T) {
	proxy, err := parseNetworksetupProxy("Enabled: No\nServer:\nPort: 0\nAuthenticated Proxy Enabled: 0\n")
	if err != nil {
		t.Fatal(err)
	}
	if proxy.Enabled {
		t.Fatalf("parsed proxy = %#v, want disabled", proxy)
	}
}

func TestParseNetworksetupService(t *testing.T) {
	service, err := parseNetworksetupService("Wi-Fi",
		"Enabled: Yes\nServer: 127.0.0.1\nPort: 1080\nAuthenticated Proxy Enabled: 0\n",
		"Enabled: No\nServer:\nPort: 0\nAuthenticated Proxy Enabled: 0\n",
		"Enabled: Yes\nServer: 127.0.0.1\nPort: 1080\nAuthenticated Proxy Enabled: 1\n",
	)
	if err != nil {
		t.Fatal(err)
	}
	if service.Name != "Wi-Fi" || !service.Web.Enabled || service.SecureWeb.Enabled || !service.Socks.Enabled || !service.Socks.Authenticated {
		t.Fatalf("parsed service = %#v", service)
	}
}

func TestDesiredMacOSProxyUnmarshalsToInstruction(t *testing.T) {
	state := desiredMacOSProxy("127.0.0.1", 1080)
	var desired macosDesired
	if err := json.Unmarshal(state, &desired); err != nil {
		t.Fatal(err)
	}
	if desired.Kind != "system-proxy" || desired.Host != "127.0.0.1" || desired.Port != 1080 {
		t.Fatalf("desired = %#v", desired)
	}
}

func TestDesiredWindowsProxy(t *testing.T) {
	state := desiredWindowsProxy("127.0.0.1", 1080)
	var parsed windowsProxyState
	if err := json.Unmarshal(state, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Version != 1 || parsed.ProxyEnable != 1 || parsed.ProxyServer != "127.0.0.1:1080" || !strings.Contains(parsed.ProxyOverride, "<local>") {
		t.Fatalf("desired = %#v", parsed)
	}
}

type recordingBackend struct {
	current ProxyState
	applied []ProxyState
}

func (r *recordingBackend) Current(context.Context) (ProxyState, error) {
	return r.current, nil
}

func (r *recordingBackend) Apply(_ context.Context, state ProxyState) error {
	r.applied = append(r.applied, state)
	return nil
}

func TestManagerRestoresSavedState(t *testing.T) {
	backend := &recordingBackend{current: ProxyState(`{"previous":true}`)}
	manager := NewManager(backend, t.TempDir()+"/recovery.json")
	if err := manager.Apply(context.Background(), ProxyState(`{"desired":true}`), 0); err != nil {
		t.Fatal(err)
	}
	if err := manager.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(backend.applied) != 2 || string(backend.applied[0]) != `{"desired":true}` || string(backend.applied[1]) != `{"previous":true}` {
		t.Fatalf("applied = %s", backend.applied)
	}
}
