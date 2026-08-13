package agent

import (
	"encoding/json"
	"fmt"
)

// windowsProxyState captures the current-user Internet Settings registry
// values that fully describe proxy behavior on Windows. A restore writes these
// values back byte-for-byte.
type windowsProxyState struct {
	Version       uint32 `json:"version"`
	ProxyEnable   uint32 `json:"proxy_enable"`
	ProxyServer   string `json:"proxy_server"`
	ProxyOverride string `json:"proxy_override,omitempty"`
	AutoConfigURL string `json:"auto_config_url,omitempty"`
	AutoDetect    uint32 `json:"auto_detect"`
}

func desiredWindowsProxy(host string, port uint32) ProxyState {
	state, _ := json.Marshal(windowsProxyState{
		Version:       1,
		ProxyEnable:   1,
		ProxyServer:   fmt.Sprintf("%s:%d", host, port),
		ProxyOverride: "<local>;127.0.0.1;localhost",
	})
	return ProxyState(state)
}
