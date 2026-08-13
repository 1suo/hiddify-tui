//go:build darwin

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// networksetupBackend reads and writes macOS System Configuration proxy
// settings through the networksetup CLI, which exposes the same per-network
// service state the SystemConfiguration framework owns.
type networksetupBackend struct {
	runner CommandRunner
}

func NewPlatformBackend() Backend {
	return &networksetupBackend{runner: execRunner{}}
}

func DesiredProxy(host string, port uint32) ProxyState {
	return desiredMacOSProxy(host, port)
}

func DefaultRecoveryPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Library", "Application Support", "hiddify", "proxy-recovery.json")
	}
	return "proxy-recovery.json"
}

func (b *networksetupBackend) Current(ctx context.Context) (ProxyState, error) {
	names, err := b.services(ctx)
	if err != nil {
		return nil, err
	}
	state := networksetupState{Version: 1}
	for _, name := range names {
		web, webErr := b.runner.Run(ctx, "networksetup", "-getwebproxy", name)
		secureWeb, secureWebErr := b.runner.Run(ctx, "networksetup", "-getsecurewebproxy", name)
		socks, socksErr := b.runner.Run(ctx, "networksetup", "-getsocksfirewallproxy", name)
		if webErr != nil || secureWebErr != nil || socksErr != nil {
			return nil, fmt.Errorf("read service %q proxy settings", name)
		}
		service, err := parseNetworksetupService(name, string(web), string(secureWeb), string(socks))
		if err != nil {
			return nil, err
		}
		state.Services = append(state.Services, service)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return ProxyState(data), nil
}

func (b *networksetupBackend) Apply(ctx context.Context, state ProxyState) error {
	var desired macosDesired
	if err := json.Unmarshal(state, &desired); err == nil && desired.Kind == "system-proxy" {
		if desired.Host == "" || desired.Port == 0 {
			return fmt.Errorf("invalid macOS proxy instruction")
		}
		services, err := b.services(ctx)
		if err != nil {
			return err
		}
		port := fmt.Sprintf("%d", desired.Port)
		for _, name := range services {
			if err := b.setServiceProxy(ctx, name, desired.Host, port); err != nil {
				return err
			}
		}
		return nil
	}

	var full networksetupState
	if err := json.Unmarshal(state, &full); err != nil {
		return fmt.Errorf("decode macOS proxy state: %w", err)
	}
	if full.Version != 1 {
		return fmt.Errorf("unsupported macOS proxy state")
	}
	for _, service := range full.Services {
		if err := b.restoreService(ctx, service); err != nil {
			return err
		}
	}
	return nil
}

func (b *networksetupBackend) services(ctx context.Context) ([]string, error) {
	output, err := b.runner.Run(ctx, "networksetup", "-listallnetworkservices")
	if err != nil {
		return nil, fmt.Errorf("list macOS network services: %w", err)
	}
	var names []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "An asterisk") || strings.HasPrefix(line, "*") {
			continue
		}
		names = append(names, line)
	}
	return names, nil
}

func (b *networksetupBackend) setServiceProxy(ctx context.Context, service, host, port string) error {
	for _, kind := range []string{"webproxy", "securewebproxy", "socksfirewallproxy"} {
		if _, err := b.runner.Run(ctx, "networksetup", "-set"+kind, service, host, port); err != nil {
			return fmt.Errorf("set %s for %s: %w", kind, service, err)
		}
		if _, err := b.runner.Run(ctx, "networksetup", "-set"+kind+"state", service, "on"); err != nil {
			return fmt.Errorf("enable %s for %s: %w", kind, service, err)
		}
	}
	return nil
}

func (b *networksetupBackend) restoreService(ctx context.Context, service networksetupService) error {
	proxies := map[string]networksetupProxy{
		"webproxy":           service.Web,
		"securewebproxy":     service.SecureWeb,
		"socksfirewallproxy": service.Socks,
	}
	for kind, proxy := range proxies {
		if proxy.Server != "" {
			if _, err := b.runner.Run(ctx, "networksetup", "-set"+kind, service.Name, proxy.Server, portOrZero(proxy.Port)); err != nil {
				return fmt.Errorf("restore %s for %s: %w", kind, service.Name, err)
			}
		}
		state := "off"
		if proxy.Enabled {
			state = "on"
		}
		if _, err := b.runner.Run(ctx, "networksetup", "-set"+kind+"state", service.Name, state); err != nil {
			return fmt.Errorf("restore %s state for %s: %w", kind, service.Name, err)
		}
	}
	return nil
}

func portOrZero(value string) string {
	if value == "" {
		return "0"
	}
	return value
}
