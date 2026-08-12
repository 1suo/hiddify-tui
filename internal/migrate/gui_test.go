package migrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1suo/hiddify-tui/internal/control"
	_ "modernc.org/sqlite"
)

func TestReadPlanDoesNotModifyGUIData(t *testing.T) {
	dir := t.TempDir()
	database := filepath.Join(dir, "db")
	db, err := sql.Open("sqlite", database)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE profile_entries (id TEXT, type TEXT, active BOOLEAN, name TEXT, url TEXT); INSERT INTO profile_entries VALUES ('r1','remote',1,'Remote','https://one.example/sub'), ('r2','remote',0,'Duplicate','https://one.example/sub'), ('l1','local',0,'Local',NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	configs := filepath.Join(dir, "configs")
	if err := os.Mkdir(configs, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configs, "l1.json"), []byte(`{"inbounds":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := ReadPlan(database, configs)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("migration modified GUI database")
	}
	if len(plan.Profiles) != 2 || plan.Profiles[0].SourceID != "r1" || string(plan.Profiles[1].Content) != `{"inbounds":[]}` {
		t.Fatalf("plan = %#v", plan)
	}
	if len(plan.Warnings) != 1 {
		t.Fatalf("warnings = %#v", plan.Warnings)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "https://one.example/sub") {
		t.Fatalf("encoded plan leaked URL: %s", encoded)
	}
}

func TestApplyImportsAndActivatesSuccessfulProfile(t *testing.T) {
	target := &fakeTarget{}
	result := Apply(context.Background(), Plan{Profiles: []Profile{
		{SourceID: "remote", Kind: "remote", Name: "Remote", URL: "https://example.test/sub"},
		{SourceID: "local", Kind: "local", Name: "Local", Content: []byte(`{"inbounds":[]}`), Active: true},
	}}, target)
	if len(result.Imported) != 2 || target.active != "local" || target.localContent != `{"inbounds":[]}` {
		t.Fatalf("result=%#v target=%#v", result, target)
	}
}

type fakeTarget struct {
	active       string
	localContent string
}

func (f *fakeTarget) AddRemoteProfile(_ context.Context, _ string, _ string, _ bool) (control.Profile, error) {
	return control.Profile{ID: "remote"}, nil
}
func (f *fakeTarget) AddLocalProfile(_ context.Context, _ string, _ bool, content io.Reader) (control.Profile, error) {
	data, _ := io.ReadAll(content)
	f.localContent = string(data)
	return control.Profile{ID: "local"}, nil
}
func (f *fakeTarget) UpdateProfileName(context.Context, string, string) (control.Profile, error) {
	panic("unused")
}
func (f *fakeTarget) RefreshProfile(context.Context, string) error        { panic("unused") }
func (f *fakeTarget) DeleteProfile(context.Context, string) error         { panic("unused") }
func (f *fakeTarget) SetActiveProfile(_ context.Context, id string) error { f.active = id; return nil }
