package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/1suo/hiddify-tui/internal/cli"
	"github.com/1suo/hiddify-tui/internal/control"
)

type connectionOperator struct {
	action  string
	profile string
	mode    control.ConnectionMode
}

func (o *connectionOperator) Connect(_ context.Context, profile string, mode control.ConnectionMode) error {
	o.action, o.profile, o.mode = "connect", profile, mode
	return nil
}
func (o *connectionOperator) Disconnect(context.Context) error { o.action = "disconnect"; return nil }
func (o *connectionOperator) Restart(context.Context) error    { o.action = "restart"; return nil }

func TestConnectionOperationJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	operator := &connectionOperator{}
	code := cli.ConnectionOperation(context.Background(), operator, "connect", "p-1", control.ModeTUN, true, &stdout, &stderr)
	if code != cli.ExitOK || operator.action != "connect" || operator.profile != "p-1" || stdout.String() != "{\"schema_version\":1,\"operation\":\"connect\",\"profile_id\":\"p-1\",\"mode\":\"tun\"}\n" {
		t.Fatalf("code=%d operator=%#v stdout=%q", code, operator, stdout.String())
	}
}
