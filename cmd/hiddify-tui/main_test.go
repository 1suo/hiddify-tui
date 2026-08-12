package main

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1suo/hiddify-tui/internal/cli"
)

func TestRunWithoutSubcommandIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run(nil, &stdout, &stderr); got != cli.ExitUsage {
		t.Fatalf("run() exit code = %d, want %d", got, cli.ExitUsage)
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"--version"}, &stdout, &stderr); got != cli.ExitOK {
		t.Fatalf("run() exit code = %d, want %d", got, cli.ExitOK)
	}
	if stdout.String() != version+"\n" {
		t.Fatalf("version output = %q", stdout.String())
	}
}

func TestRunMigrationGUIDryRun(t *testing.T) {
	dir := t.TempDir()
	database := filepath.Join(dir, "profiles.db")
	db, err := sql.Open("sqlite", database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE profile_entries (id TEXT, type TEXT, active BOOLEAN, name TEXT, url TEXT); INSERT INTO profile_entries VALUES ('r1','remote',1,'Remote','https://example.test/sub');`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	configs := filepath.Join(dir, "configs")
	if err := os.Mkdir(configs, 0700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"migrate", "gui", "--database", database, "--configs", configs, "--dry-run"}
	if got := run(args, &stdout, &stderr); got != cli.ExitOK {
		t.Fatalf("run() exit code = %d stderr=%q", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"source_id":"r1"`) || strings.Contains(stdout.String(), "https://example.test/sub") {
		t.Fatalf("dry-run output = %s", stdout.String())
	}
}
