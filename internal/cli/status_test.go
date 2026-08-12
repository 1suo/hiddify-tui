package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/1suo/hiddify-tui/internal/cli"
	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
)

func TestStatusJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Status(context.Background(), client.FakeControl{Snapshot: control.Snapshot{APIMajor: 1, ConnectionState: control.ConnectionStarted}}, true, &stdout, &stderr)
	if code != cli.ExitOK || !strings.Contains(stdout.String(), `"schema_version":1`) || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestStatusUnavailable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Status(context.Background(), client.FakeControl{Err: client.ErrUnavailable}, false, &stdout, &stderr)
	if code != cli.ExitUnavailable || !strings.Contains(stderr.String(), "unavailable") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}
