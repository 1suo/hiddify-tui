//go:build windows

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const internetSettingsPath = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// registryBackend reads and writes the current-user Internet Settings proxy
// values, then notifies the system that the configuration changed.
type registryBackend struct{}

func NewPlatformBackend() Backend {
	return &registryBackend{}
}

func DesiredProxy(host string, port uint32) ProxyState {
	return desiredWindowsProxy(host, port)
}

func DefaultRecoveryPath() string {
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "hiddify", "proxy-recovery.json")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "AppData", "Local", "hiddify", "proxy-recovery.json")
	}
	return "proxy-recovery.json"
}

func (b *registryBackend) Current(ctx context.Context) (ProxyState, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("open Internet Settings: %w", err)
	}
	defer key.Close()

	enable, _, _ := key.GetIntegerValue("ProxyEnable")
	server, _, _ := key.GetStringValue("ProxyServer")
	override, _, _ := key.GetStringValue("ProxyOverride")
	autoConfigURL, _, _ := key.GetStringValue("AutoConfigURL")
	autoDetect, _, _ := key.GetIntegerValue("AutoDetect")

	state := windowsProxyState{
		Version:       1,
		ProxyEnable:   uint32(enable),
		ProxyServer:   server,
		ProxyOverride: override,
		AutoConfigURL: autoConfigURL,
		AutoDetect:    uint32(autoDetect),
	}
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return ProxyState(data), nil
}

func (b *registryBackend) Apply(ctx context.Context, state ProxyState) error {
	var parsed windowsProxyState
	if err := json.Unmarshal(state, &parsed); err != nil {
		return fmt.Errorf("decode Windows proxy state: %w", err)
	}
	if parsed.Version != 1 {
		return fmt.Errorf("unsupported Windows proxy state")
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, internetSettingsPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Internet Settings: %w", err)
	}
	defer key.Close()

	if err := key.SetDWordValue("ProxyEnable", parsed.ProxyEnable); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}
	if err := key.SetStringValue("ProxyServer", parsed.ProxyServer); err != nil {
		return fmt.Errorf("set ProxyServer: %w", err)
	}
	if err := key.SetStringValue("ProxyOverride", parsed.ProxyOverride); err != nil {
		return fmt.Errorf("set ProxyOverride: %w", err)
	}
	if err := key.SetStringValue("AutoConfigURL", parsed.AutoConfigURL); err != nil {
		return fmt.Errorf("set AutoConfigURL: %w", err)
	}
	if err := key.SetDWordValue("AutoDetect", parsed.AutoDetect); err != nil {
		return fmt.Errorf("set AutoDetect: %w", err)
	}
	notifyWindowsProxyChange()
	return nil
}

func notifyWindowsProxyChange() {
	const (
		internetOptionSettingsChanged = 39
		internetOptionRefresh         = 37
	)
	wininet := windows.NewLazySystemDLL("wininet.dll")
	internetSetOption := wininet.NewProc("InternetSetOptionW")
	internetSetOption.Call(0, internetOptionSettingsChanged, 0, 0)
	internetSetOption.Call(0, internetOptionRefresh, 0, 0)
}
