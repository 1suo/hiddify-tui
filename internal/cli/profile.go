package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/1suo/hiddify-tui/internal/client"
	"github.com/1suo/hiddify-tui/internal/profile"
)

type profileOutput struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Kind           string        `json:"kind"`
	Active         bool          `json:"active"`
	RedactedURL    string        `json:"redacted_url,omitempty"`
	UpdateInterval int64         `json:"update_interval_ms,omitempty"`
	Usage          profile.Usage `json:"subscription"`
	LastUpdate     int64         `json:"last_update_unix,omitempty"`
}

func profileToOutput(profile profile.Profile) profileOutput {
	return profileOutput{
		ID:             profile.ID,
		Name:           profile.Name,
		Kind:           string(profile.Kind),
		Active:         profile.Active,
		RedactedURL:    redactURL(profile.URL),
		UpdateInterval: profile.UpdateInterval,
		Usage:          profile.Usage,
		LastUpdate:     profile.LastUpdate,
	}
}

// ProfileList lists client-side profiles.
func ProfileList(store *profile.Store, jsonOutput bool, stdout, stderr io.Writer) int {
	profiles := store.List()
	if jsonOutput {
		out := make([]profileOutput, 0, len(profiles))
		for _, profile := range profiles {
			out = append(out, profileToOutput(profile))
		}
		if err := writeJSON(stdout, out); err != nil {
			return ExitRejected
		}
		return ExitOK
	}
	if len(profiles) == 0 {
		fmt.Fprintln(stdout, "no profiles")
		return ExitOK
	}
	for _, profile := range profiles {
		active := " "
		if profile.Active {
			active = "*"
		}
		source := redactURL(profile.URL)
		if source == "" {
			source = "local"
		}
		fmt.Fprintf(stdout, "%s %s  [%s]  %s\n", active, profile.Name, profile.Kind, source)
	}
	return ExitOK
}

// ProfileShow prints one profile's metadata.
func ProfileShow(store *profile.Store, id string, jsonOutput bool, stdout, stderr io.Writer) int {
	profile, ok := store.Get(id)
	if !ok {
		WriteError(stderr, "profile show", fmt.Errorf("profile %q not found", id))
		return ExitRejected
	}
	if jsonOutput {
		if err := writeJSON(stdout, profileToOutput(profile)); err != nil {
			return ExitRejected
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "ID:       %s\n", profile.ID)
	fmt.Fprintf(stdout, "Name:     %s\n", profile.Name)
	fmt.Fprintf(stdout, "Kind:     %s\n", profile.Kind)
	fmt.Fprintf(stdout, "Active:   %t\n", profile.Active)
	if profile.URL != "" {
		fmt.Fprintf(stdout, "URL:      %s\n", redactURL(profile.URL))
	}
	if profile.Usage.Total > 0 {
		fmt.Fprintf(stdout, "Usage:    %d / %d bytes\n", profile.Usage.Download+profile.Usage.Upload, profile.Usage.Total)
	}
	return ExitOK
}

// ProfileAddRemote downloads a subscription URL and stores it.
func ProfileAddRemote(ctx context.Context, core client.Client, store *profile.Store, rawURL, name string, setActive, jsonOutput bool, stdout, stderr io.Writer) int {
	remote, err := profile.FetchRemote(ctx, rawURL)
	if err != nil {
		WriteError(stderr, "profile add", err)
		return ExitRejected
	}
	if err := core.Parse(ctx, remote.Content); err != nil {
		WriteError(stderr, "profile add", err)
		return ExitRejected
	}
	if name != "" {
		remote.Name = name
	}
	added := store.Add(profile.Profile{
		Name: remote.Name, Kind: profile.KindRemote, URL: rawURL,
		UpdateInterval: remote.UpdateInterval, Usage: remote.Usage, Content: remote.Content,
	}, setActive)
	if err := store.Save(); err != nil {
		WriteError(stderr, "profile add", err)
		return ExitRejected
	}
	return profileSaved(added, store, jsonOutput, stdout)
}

// ProfileAddLocal stores local config content, validated against the core.
func ProfileAddLocal(ctx context.Context, core client.Client, store *profile.Store, content, name string, setActive, jsonOutput bool, stdout, stderr io.Writer) int {
	if err := core.Parse(ctx, content); err != nil {
		WriteError(stderr, "profile add", err)
		return ExitRejected
	}
	if name == "" {
		name = profile.DefaultName(content)
	}
	added := store.Add(profile.Profile{Name: name, Kind: profile.KindLocal, Content: content}, setActive)
	if err := store.Save(); err != nil {
		WriteError(stderr, "profile add", err)
		return ExitRejected
	}
	return profileSaved(added, store, jsonOutput, stdout)
}

// ProfileDelete removes a profile.
func ProfileDelete(store *profile.Store, id string, jsonOutput bool, stdout, stderr io.Writer) int {
	if err := store.Delete(id); err != nil {
		WriteError(stderr, "profile delete", err)
		return ExitRejected
	}
	return operationResult("profile delete", jsonOutput, stdout)
}

// ProfileActivate marks a profile active.
func ProfileActivate(store *profile.Store, id string, jsonOutput bool, stdout, stderr io.Writer) int {
	if err := store.SetActive(id); err != nil {
		WriteError(stderr, "profile activate", err)
		return ExitRejected
	}
	return operationResult("profile activate", jsonOutput, stdout)
}

// ProfileRename changes a profile's display name.
func ProfileRename(store *profile.Store, id, name string, jsonOutput bool, stdout, stderr io.Writer) int {
	if err := store.Rename(id, name); err != nil {
		WriteError(stderr, "profile rename", err)
		return ExitRejected
	}
	return operationResult("profile rename", jsonOutput, stdout)
}

// ProfileRefresh re-downloads a remote profile's subscription.
func ProfileRefresh(ctx context.Context, core client.Client, store *profile.Store, id string, jsonOutput bool, stdout, stderr io.Writer) int {
	target, ok := store.Get(id)
	if !ok {
		WriteError(stderr, "profile refresh", fmt.Errorf("profile %q not found", id))
		return ExitRejected
	}
	if target.Kind != profile.KindRemote || target.URL == "" {
		WriteError(stderr, "profile refresh", fmt.Errorf("local profiles cannot be refreshed"))
		return ExitRejected
	}
	remote, err := profile.FetchRemote(ctx, target.URL)
	if err != nil {
		WriteError(stderr, "profile refresh", err)
		return ExitRejected
	}
	if err := core.Parse(ctx, remote.Content); err != nil {
		WriteError(stderr, "profile refresh", err)
		return ExitRejected
	}
	target.Content = remote.Content
	target.Usage = remote.Usage
	target.UpdateInterval = remote.UpdateInterval
	target.LastUpdate = nowUnix()
	if err := store.Update(target); err != nil {
		WriteError(stderr, "profile refresh", err)
		return ExitRejected
	}
	return operationResult("profile refresh", jsonOutput, stdout)
}

func profileSaved(profile profile.Profile, store *profile.Store, jsonOutput bool, stdout io.Writer) int {
	if jsonOutput {
		profile.Active = profile.ID == store.ActiveID
		if err := writeJSON(stdout, profileToOutput(profile)); err != nil {
			return ExitRejected
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "Profile: %s (%s)\n", profile.Name, profile.ID)
	return ExitOK
}
