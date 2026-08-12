package cli_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/1suo/hiddify-tui/internal/cli"
	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/control"
)

type profileWriter struct {
	added   control.Profile
	renamed control.Profile
	deleted string
}

func (w *profileWriter) AddRemoteProfile(_ context.Context, url, name string, active bool) (control.Profile, error) {
	w.added = control.Profile{ID: "p-2", Name: name, Kind: control.ProfileRemote, Active: active, RedactedURL: "https://example.test/…"}
	if url == "" {
		return control.Profile{}, context.Canceled
	}
	return w.added, nil
}
func (w *profileWriter) UpdateProfileName(_ context.Context, id, name string) (control.Profile, error) {
	w.renamed = control.Profile{ID: id, Name: name}
	return w.renamed, nil
}
func (w *profileWriter) RefreshProfile(context.Context, string) error     { return nil }
func (w *profileWriter) DeleteProfile(_ context.Context, id string) error { w.deleted = id; return nil }
func (w *profileWriter) SetActiveProfile(context.Context, string) error   { return nil }

func (w *profileWriter) AddLocalProfile(_ context.Context, name string, active bool, content io.Reader) (control.Profile, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return control.Profile{}, err
	}
	if len(data) == 0 {
		return control.Profile{}, context.Canceled
	}
	return control.Profile{ID: "p-local", Name: name, Kind: control.ProfileLocal, Active: active}, nil
}

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

func TestProfileAddAndDelete(t *testing.T) {
	var stdout, stderr bytes.Buffer
	writer := &profileWriter{}
	if code := cli.ProfileAddRemote(context.Background(), writer, "https://example.test/sub", "Home", true, true, &stdout, &stderr); code != cli.ExitOK || !writer.added.Active || !strings.Contains(stdout.String(), `"id":"p-2"`) {
		t.Fatalf("add code=%d stdout=%q", code, stdout.String())
	}
	stdout.Reset()
	if code := cli.ProfileOperation(context.Background(), "delete", "p-2", writer.DeleteProfile, &stdout, &stderr); code != cli.ExitOK || writer.deleted != "p-2" {
		t.Fatalf("delete code=%d deleted=%q", code, writer.deleted)
	}
}

func TestProfileAddLocal(t *testing.T) {
	var stdout, stderr bytes.Buffer
	writer := &profileWriter{}
	code := cli.ProfileAddLocal(context.Background(), writer, strings.NewReader("vmess://example"), "Imported", false, true, &stdout, &stderr)
	if code != cli.ExitOK || !strings.Contains(stdout.String(), `"id":"p-local"`) || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
