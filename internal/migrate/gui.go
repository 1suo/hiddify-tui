// Package migrate reads Hiddify GUI data without modifying it.
package migrate

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/1suo/hiddify-tui/internal/control"
	_ "modernc.org/sqlite"
)

type Profile struct {
	SourceID    string `json:"source_id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	URL         string `json:"-"`
	RedactedURL string `json:"redacted_url,omitempty"`
	Content     []byte `json:"-"`
	Active      bool   `json:"active"`
}

type Warning struct {
	SourceID string `json:"source_id,omitempty"`
	Message  string `json:"message"`
}

type Plan struct {
	Profiles []Profile `json:"profiles"`
	Warnings []Warning `json:"warnings,omitempty"`
}

type ImportedProfile struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
}

type Result struct {
	Imported []ImportedProfile `json:"imported"`
	Warnings []Warning         `json:"warnings,omitempty"`
}

type Target interface {
	control.ProfileWriter
	control.LocalProfileWriter
}

// ReadPlan opens the GUI database read-only. Local content is read from the
// matching configs directory but neither source is changed.
func ReadPlan(databasePath, configsDir string) (Plan, error) {
	uri := (&url.URL{Scheme: "file", Path: databasePath, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return Plan{}, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, type, active, name, url FROM profile_entries ORDER BY rowid`)
	if err != nil {
		return Plan{}, fmt.Errorf("read GUI profiles: %w", err)
	}
	defer rows.Close()
	plan := Plan{}
	seen := map[string]string{}
	for rows.Next() {
		var profile Profile
		var sourceURL sql.NullString
		if err := rows.Scan(&profile.SourceID, &profile.Kind, &profile.Active, &profile.Name, &sourceURL); err != nil {
			return Plan{}, fmt.Errorf("read GUI profile: %w", err)
		}
		profile.URL = sourceURL.String
		profile.RedactedURL = redactURL(profile.URL)
		switch profile.Kind {
		case "remote":
			if profile.URL == "" {
				plan.Warnings = append(plan.Warnings, Warning{profile.SourceID, "remote profile has no URL and was skipped"})
				continue
			}
			if duplicate, ok := seen[profile.URL]; ok {
				plan.Warnings = append(plan.Warnings, Warning{profile.SourceID, "duplicate remote URL; first source ID is " + duplicate})
				continue
			}
			seen[profile.URL] = profile.SourceID
		case "local":
			content, err := os.ReadFile(filepath.Join(configsDir, profile.SourceID+".json"))
			if err != nil {
				plan.Warnings = append(plan.Warnings, Warning{profile.SourceID, "local config unavailable: " + err.Error()})
				continue
			}
			profile.Content = content
		default:
			plan.Warnings = append(plan.Warnings, Warning{profile.SourceID, "unknown profile type " + profile.Kind})
			continue
		}
		plan.Profiles = append(plan.Profiles, profile)
	}
	return plan, rows.Err()
}

// Apply imports a previously-read plan into the daemon. It never accesses the
// GUI database or config directory, which lets callers separate review from a
// deliberate write operation.
func Apply(ctx context.Context, plan Plan, target Target) Result {
	result := Result{Warnings: append([]Warning(nil), plan.Warnings...)}
	activeID := ""
	for _, profile := range plan.Profiles {
		var (
			imported control.Profile
			err      error
		)
		switch profile.Kind {
		case "remote":
			imported, err = target.AddRemoteProfile(ctx, profile.URL, profile.Name, false)
		case "local":
			imported, err = target.AddLocalProfile(ctx, profile.Name, false, bytesReader(profile.Content))
		}
		if err != nil {
			result.Warnings = append(result.Warnings, Warning{profile.SourceID, "import failed: " + err.Error()})
			continue
		}
		result.Imported = append(result.Imported, ImportedProfile{profile.SourceID, imported.ID})
		if profile.Active {
			activeID = imported.ID
		}
	}
	if activeID != "" {
		if err := target.SetActiveProfile(ctx, activeID); err != nil {
			result.Warnings = append(result.Warnings, Warning{Message: "set active profile failed: " + err.Error()})
		}
	}
	return result
}

func bytesReader(content []byte) *bytes.Reader { return bytes.NewReader(content) }

func redactURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "redacted"
	}
	return parsed.Scheme + "://" + parsed.Host + "/…"
}
