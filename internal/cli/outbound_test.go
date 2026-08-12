package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/1suo/hiddify-tui/internal/cli"
	"github.com/1suo/hiddify-tui/internal/control"
)

type outboundOperator struct {
	groups   []control.OutboundGroup
	groupID  string
	outbound string
	scope    control.TestScope
}

func (o *outboundOperator) ListOutboundGroups(context.Context) ([]control.OutboundGroup, error) {
	return o.groups, nil
}
func (o *outboundOperator) SelectOutbound(_ context.Context, groupID, outboundID string) error {
	o.groupID, o.outbound = groupID, outboundID
	return nil
}
func (o *outboundOperator) TestOutbounds(_ context.Context, scope control.TestScope) error {
	o.scope = scope
	return nil
}

func TestOutboundListAndOperations(t *testing.T) {
	var stdout, stderr bytes.Buffer
	operator := &outboundOperator{groups: []control.OutboundGroup{{ID: "g-1", Name: "Auto", SelectedOutboundID: "o-1", Outbounds: []control.Outbound{{ID: "o-1", Tag: "Fast", Protocol: "vless", DelayMillis: 42}}}}}
	if code := cli.OutboundList(context.Background(), operator, false, &stdout, &stderr); code != cli.ExitOK || !strings.Contains(stdout.String(), "42 ms") {
		t.Fatalf("list code=%d stdout=%q", code, stdout.String())
	}
	if code := cli.OutboundSelect(context.Background(), operator, "g-1", "o-1", &stdout, &stderr); code != cli.ExitOK || operator.outbound != "o-1" {
		t.Fatalf("select code=%d operator=%#v", code, operator)
	}
	if code := cli.OutboundTest(context.Background(), operator, control.TestScope{AllVisible: true}, &stdout, &stderr); code != cli.ExitOK || !operator.scope.AllVisible {
		t.Fatalf("test code=%d operator=%#v", code, operator)
	}
}
