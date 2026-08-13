package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// networksetupState captures every per-service proxy the macOS System
// Configuration reports, so a restore reproduces the original state exactly
// rather than inferring a partial proxy configuration.
type networksetupState struct {
	Version  uint32                `json:"version"`
	Services []networksetupService `json:"services"`
}

type networksetupService struct {
	Name      string            `json:"name"`
	Web       networksetupProxy `json:"web"`
	SecureWeb networksetupProxy `json:"secure_web"`
	Socks     networksetupProxy `json:"socks"`
}

type networksetupProxy struct {
	Enabled       bool   `json:"enabled"`
	Server        string `json:"server,omitempty"`
	Port          string `json:"port,omitempty"`
	Authenticated bool   `json:"authenticated,omitempty"`
}

// macosDesired marks a ProxyState that instructs the macOS backend to point
// every active network service at a loopback mixed proxy.
type macosDesired struct {
	Kind string `json:"kind"`
	Host string `json:"host"`
	Port uint32 `json:"port"`
}

func desiredMacOSProxy(host string, port uint32) ProxyState {
	state, _ := json.Marshal(macosDesired{Kind: "system-proxy", Host: host, Port: port})
	return ProxyState(state)
}

func parseNetworksetupService(name, webOutput, secureWebOutput, socksOutput string) (networksetupService, error) {
	web, err := parseNetworksetupProxy(webOutput)
	if err != nil {
		return networksetupService{}, fmt.Errorf("service %s web proxy: %w", name, err)
	}
	secureWeb, err := parseNetworksetupProxy(secureWebOutput)
	if err != nil {
		return networksetupService{}, fmt.Errorf("service %s secure web proxy: %w", name, err)
	}
	socks, err := parseNetworksetupProxy(socksOutput)
	if err != nil {
		return networksetupService{}, fmt.Errorf("service %s socks proxy: %w", name, err)
	}
	return networksetupService{Name: name, Web: web, SecureWeb: secureWeb, Socks: socks}, nil
}

func parseNetworksetupProxy(output string) (networksetupProxy, error) {
	var proxy networksetupProxy
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return networksetupProxy{}, fmt.Errorf("unexpected line %q", line)
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "Enabled":
			proxy.Enabled = value == "Yes"
		case "Server":
			proxy.Server = value
		case "Port":
			proxy.Port = value
		case "Authenticated Proxy Enabled":
			proxy.Authenticated = value == "1"
		}
	}
	return proxy, nil
}
