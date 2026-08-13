//go:build !linux && !darwin && !windows

package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

var errUnsupportedPlatform = errors.New("system-proxy is not supported on this platform")

type unsupportedBackend struct{}

func (unsupportedBackend) Current(context.Context) (ProxyState, error) {
	return nil, errUnsupportedPlatform
}

func (unsupportedBackend) Apply(context.Context, ProxyState) error {
	return errUnsupportedPlatform
}

func NewPlatformBackend() Backend {
	return unsupportedBackend{}
}

func DesiredProxy(host string, port uint32) ProxyState {
	return ProxyState("{}")
}

func DefaultRecoveryPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", "hiddify", "proxy-recovery.json")
	}
	return "proxy-recovery.json"
}
