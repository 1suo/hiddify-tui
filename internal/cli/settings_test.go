package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/1suo/hiddify-tui/internal/cli"
	"github.com/1suo/hiddify-tui/internal/control"
)

type settingsOperator struct{}

func (settingsOperator) GetSettings(context.Context) (control.Settings, error) {
	return control.Settings{RedactedJSON: []byte(`{"dns":"redacted"}`)}, nil
}
func (settingsOperator) ValidateSettings(context.Context, []byte) (control.ValidationResult, error) {
	return control.ValidationResult{Valid: true}, nil
}
func (settingsOperator) UpdateSettings(context.Context, []byte) (control.Settings, error) {
	return control.Settings{RedactedJSON: []byte(`{"mode":"tun"}`)}, nil
}
func (settingsOperator) ResetSettings(context.Context) (control.Settings, error) {
	return control.Settings{RedactedJSON: []byte(`{}`)}, nil
}
func (settingsOperator) ExportSettings(context.Context, bool) ([]byte, error) {
	return []byte(`{"export":true}`), nil
}
func (settingsOperator) ImportSettings(context.Context, []byte) (control.Settings, error) {
	return control.Settings{RedactedJSON: []byte(`{"imported":true}`)}, nil
}

func TestSettingsWorkflows(t *testing.T) {
	var stdout, stderr bytes.Buffer
	daemon := settingsOperator{}
	if code := cli.SettingsShow(context.Background(), daemon, &stdout, &stderr); code != cli.ExitOK || stdout.String() != "{\"dns\":\"redacted\"}\n" {
		t.Fatalf("show code=%d stdout=%q", code, stdout.String())
	}
	stdout.Reset()
	if code := cli.SettingsValidate(context.Background(), daemon, []byte(`{}`), &stdout, &stderr); code != cli.ExitOK || stdout.String() != "{\"schema_version\":1,\"result\":{\"valid\":true}}\n" {
		t.Fatalf("validate code=%d stdout=%q", code, stdout.String())
	}
	stdout.Reset()
	if code := cli.SettingsWrite(context.Background(), daemon, "set", []byte(`{}`), &stdout, &stderr); code != cli.ExitOK || stdout.String() != "{\"mode\":\"tun\"}\n" {
		t.Fatalf("set code=%d stdout=%q", code, stdout.String())
	}
}
