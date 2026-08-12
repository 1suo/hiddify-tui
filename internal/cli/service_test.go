package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/1suo/hiddify-tui/internal/cli"
	"github.com/1suo/hiddify-tui/internal/control"
)

type serviceReader struct {
	auto bool
}

func (s *serviceReader) GetSnapshot(context.Context) (control.Snapshot, error) {
	return control.Snapshot{APIMajor: 1, AutoConnect: s.auto}, nil
}
func (s *serviceReader) GetServiceInfo(context.Context) (control.ServiceInfo, error) {
	return control.ServiceInfo{Installed: true, Enabled: true, Running: true}, nil
}
func (s *serviceReader) SetAutoConnect(_ context.Context, enabled bool) error {
	s.auto = enabled
	return nil
}
func (s *serviceReader) GetDiagnostics(context.Context) (control.Diagnostics, error) {
	return control.Diagnostics{DaemonVersion: "1.0", SocketPath: "/run/hiddify/control.sock"}, nil
}

func TestAutoConnectAndServiceStatus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	reader := &serviceReader{}
	if code := cli.AutoConnect(context.Background(), reader, "enable", true, &stdout, &stderr); code != cli.ExitOK || stdout.String() != "{\"schema_version\":1,\"enabled\":true}\n" {
		t.Fatalf("auto code=%d stdout=%q", code, stdout.String())
	}
	stdout.Reset()
	if code := cli.ServiceStatus(context.Background(), reader, false, &stdout, &stderr); code != cli.ExitOK || stdout.String() != "Installed: true\nEnabled: true\nRunning: true\n" {
		t.Fatalf("status code=%d stdout=%q", code, stdout.String())
	}
}

func TestAgentStatus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	reader := &serviceReader{}
	if code := cli.AgentStatus(context.Background(), reader, true, &stdout, &stderr); code != cli.ExitOK || stdout.String() != "{\"schema_version\":1,\"agent\":{\"required\":false,\"connected\":false}}\n" {
		t.Fatalf("status code=%d stdout=%q", code, stdout.String())
	}
}
