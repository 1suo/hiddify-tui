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

func TestProfileListJSONRedactsSource(t *testing.T) {
	var stdout, stderr bytes.Buffer
	daemon := client.FakeControl{Profiles: []control.Profile{{ID: "p-1", Name: "Home", Kind: control.ProfileRemote, Active: true, RedactedURL: "https://example.test/…"}}}
	code := cli.ProfileList(context.Background(), daemon, true, &stdout, &stderr)
	if code != cli.ExitOK || !strings.Contains(stdout.String(), `"redacted_url":"https://example.test/…"`) || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestProfileShow(t *testing.T) {
	var stdout, stderr bytes.Buffer
	daemon := client.FakeControl{Profiles: []control.Profile{{ID: "p-1", Name: "Home", Kind: control.ProfileLocal}}}
	code := cli.ProfileShow(context.Background(), daemon, "p-1", false, &stdout, &stderr)
	if code != cli.ExitOK || !strings.Contains(stdout.String(), "Name: Home") || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
