//go:build linux

package agent

import (
	"os"
	"path/filepath"
)

func NewPlatformBackend() Backend {
	return NewGSettingsBackend()
}

func DesiredProxy(host string, port uint32) ProxyState {
	return DesiredGSettingsProxy(host, port)
}

func DefaultRecoveryPath() string {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		return filepath.Join(stateHome, "hiddify", "proxy-recovery.json")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", "hiddify", "proxy-recovery.json")
	}
	return "proxy-recovery.json"
}
