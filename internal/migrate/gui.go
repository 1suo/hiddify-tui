// Package migrate reads Hiddify GUI data without modifying it.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/1suo/hiddify-tui/internal/profile"
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

// Apply imports a previously-read plan into the client-side profile store. It
// never accesses the GUI database or config directory, which lets callers
// separate review from a deliberate write operation.
func Apply(ctx context.Context, plan Plan, store *profile.Store) Result {
	result := Result{Warnings: append([]Warning(nil), plan.Warnings...)}
	activeID := ""
	for _, source := range plan.Profiles {
		var added profile.Profile
		switch source.Kind {
		case "remote":
			remote, err := profile.FetchRemote(ctx, source.URL)
			if err != nil {
				result.Warnings = append(result.Warnings, Warning{source.SourceID, "import failed: " + err.Error()})
				continue
			}
			if source.Name != "" {
				remote.Name = source.Name
			}
			added = store.Add(profile.Profile{
				Name: remote.Name, Kind: profile.KindRemote, URL: source.URL,
				UpdateInterval: remote.UpdateInterval, Usage: remote.Usage, Content: remote.Content,
			}, false)
		case "local":
			added = store.Add(profile.Profile{Name: source.Name, Kind: profile.KindLocal, Content: string(source.Content)}, false)
		default:
			result.Warnings = append(result.Warnings, Warning{source.SourceID, "unknown profile type " + source.Kind})
			continue
		}
		result.Imported = append(result.Imported, ImportedProfile{source.SourceID, added.ID})
		if source.Active {
			activeID = added.ID
		}
	}
	if err := store.Save(); err != nil {
		result.Warnings = append(result.Warnings, Warning{Message: "save profile store: " + err.Error()})
		return result
	}
	if activeID != "" {
		if err := store.SetActive(activeID); err != nil {
			result.Warnings = append(result.Warnings, Warning{Message: "set active profile failed: " + err.Error()})
		}
	}
	return result
}

func redactURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "redacted"
	}
	return parsed.Scheme + "://" + parsed.Host + "/~"
}
