package migrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1suo/hiddify-tui/internal/profile"
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

func TestApplyImportsLocalProfile(t *testing.T) {
	store, err := profile.Open(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	result := Apply(context.Background(), Plan{Profiles: []Profile{
		{SourceID: "local", Kind: "local", Name: "Local", Content: []byte(`{"inbounds":[]}`), Active: true},
	}}, store)
	if len(result.Imported) != 1 {
		t.Fatalf("result = %#v", result)
	}
	active, ok := store.Active()
	if !ok || active.Name != "Local" || active.Content != `{"inbounds":[]}` {
		t.Fatalf("active = %#v ok=%t", active, ok)
	}
}
