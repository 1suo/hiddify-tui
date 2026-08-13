// Package profile implements the client-side profile store, mirroring how the
// Hiddify GUI owns profiles locally and hands the active config to the core.
package profile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Kind distinguishes a remote subscription from a local config.
type Kind string

const (
	KindRemote Kind = "remote"
	KindLocal  Kind = "local"
)

// Usage holds a remote profile's subscription accounting.
type Usage struct {
	Upload     int64  `json:"upload_bytes,omitempty"`
	Download   int64  `json:"download_bytes,omitempty"`
	Total      int64  `json:"total_bytes,omitempty"`
	Expire     int64  `json:"expire_unix,omitempty"`
	WebPageURL string `json:"web_page_url,omitempty"`
	SupportURL string `json:"support_url,omitempty"`
}

// Profile is a single client-owned profile. URL and Content are never rendered
// in full; the URL is redacted for display and Content is stored verbatim.
type Profile struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Kind           Kind   `json:"kind"`
	URL            string `json:"url,omitempty"`
	LastUpdate     int64  `json:"last_update_unix,omitempty"`
	UpdateInterval int64  `json:"update_interval_ms,omitempty"`
	Usage          Usage  `json:"subscription"`
	Content        string `json:"-"`
	Active         bool   `json:"active"`
}

// Store is the on-disk profile list. ActiveID selects which profile the client
// connects.
type Store struct {
	Path     string    `json:"-"`
	ActiveID string    `json:"active_id"`
	Profiles []Profile `json:"profiles"`
}

// DefaultPath returns the user profile store path under XDG data directories.
func DefaultPath() string {
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "hiddify", "profiles.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "profiles.json"
	}
	return filepath.Join(home, ".local", "share", "hiddify", "profiles.json")
}

// Open loads the store from path, creating an empty one if absent.
func Open(path string) (*Store, error) {
	store := &Store{Path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, store); err != nil {
		return nil, fmt.Errorf("decode profile store: %w", err)
	}
	return store, nil
}

// Save persists the store atomically.
func (s *Store) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	temporary := s.Path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, s.Path)
}

// List returns a copy of the profiles with Active resolved.
func (s *Store) List() []Profile {
	out := make([]Profile, len(s.Profiles))
	for i, profile := range s.Profiles {
		profile.Active = profile.ID == s.ActiveID
		out[i] = profile
	}
	return out
}

// Get returns the profile with the given id.
func (s *Store) Get(id string) (Profile, bool) {
	for _, profile := range s.Profiles {
		if profile.ID == id {
			profile.Active = profile.ID == s.ActiveID
			return profile, true
		}
	}
	return Profile{}, false
}

// Active returns the active profile.
func (s *Store) Active() (Profile, bool) {
	if s.ActiveID == "" {
		return Profile{}, false
	}
	return s.Get(s.ActiveID)
}

// Add inserts a profile and returns it. If setActive, it becomes the active one.
func (s *Store) Add(profile Profile, setActive bool) Profile {
	profile.ID = newID()
	s.Profiles = append(s.Profiles, profile)
	if setActive {
		s.ActiveID = profile.ID
	}
	return profile
}

// SetActive marks the profile active and persists.
func (s *Store) SetActive(id string) error {
	if _, ok := s.Get(id); !ok {
		return fmt.Errorf("profile %q not found", id)
	}
	s.ActiveID = id
	return s.Save()
}

// Delete removes a profile. If it was active, the selection is cleared.
func (s *Store) Delete(id string) error {
	for i, profile := range s.Profiles {
		if profile.ID == id {
			s.Profiles = append(s.Profiles[:i], s.Profiles[i+1:]...)
			if s.ActiveID == id {
				s.ActiveID = ""
			}
			return s.Save()
		}
	}
	return fmt.Errorf("profile %q not found", id)
}

// Rename updates a profile's display name.
func (s *Store) Rename(id, name string) error {
	for i := range s.Profiles {
		if s.Profiles[i].ID == id {
			s.Profiles[i].Name = name
			return s.Save()
		}
	}
	return fmt.Errorf("profile %q not found", id)
}

// Update replaces the stored profile with the given one (matched by ID).
func (s *Store) Update(profile Profile) error {
	for i := range s.Profiles {
		if s.Profiles[i].ID == profile.ID {
			s.Profiles[i] = profile
			return s.Save()
		}
	}
	return fmt.Errorf("profile %q not found", profile.ID)
}

func newID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
