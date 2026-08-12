package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/1suo/hiddify-tui/internal/cli"
	"github.com/1suo/hiddify-tui/internal/control"
)

type logReader struct {
	cleared bool
}

func (r *logReader) TailLogs(context.Context, uint32, control.LogLevel, bool) (<-chan control.LogEntry, error) {
	entries := make(chan control.LogEntry, 1)
	entries <- control.LogEntry{Sequence: 1, TimestampUnix: 1, Level: control.LogWarn, Component: "core", Message: "redacted message"}
	close(entries)
	return entries, nil
}
func (r *logReader) ClearLogs(context.Context) error { r.cleared = true; return nil }

func TestLogsJSONLinesAndClear(t *testing.T) {
	var stdout, stderr bytes.Buffer
	reader := &logReader{}
	if code := cli.Logs(context.Background(), reader, 10, control.LogInfo, false, true, &stdout, &stderr); code != cli.ExitOK || stdout.String() != "{\"sequence\":1,\"timestamp_unix_nano\":1,\"level\":\"warn\",\"component\":\"core\",\"message\":\"redacted message\"}\n" {
		t.Fatalf("logs code=%d stdout=%q", code, stdout.String())
	}
	stdout.Reset()
	if code := cli.ClearLogs(context.Background(), reader, &stdout, &stderr); code != cli.ExitOK || !reader.cleared {
		t.Fatalf("clear code=%d cleared=%v", code, reader.cleared)
	}
}
