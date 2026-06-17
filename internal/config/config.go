// Package config is the global credential/profile store for the mello CLI.
//
// Credentials live in ~/.config/mello/config.json (mode 0600), keyed by named
// profile. Env vars override the file: MELLO_TOKEN, MELLO_BASE_URL,
// MELLO_PROFILE, MELLO_WORKSPACE. The token is a Mello PAT (mello_pat_…) sent as
// Authorization: Bearer and never logged.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultBaseURL is used when no profile/env overrides it.
const DefaultBaseURL = "https://mello.mezon.vn/api/v1"

// Profile is one named set of credentials.
type Profile struct {
	BaseURL     string `json:"base_url,omitempty"`
	Token       string `json:"token,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	UserID      string `json:"user_id,omitempty"`
}

// Store is the on-disk config file.
type Store struct {
	Current  string             `json:"current"`
	Profiles map[string]Profile `json:"profiles"`
}

// Resolved is the effective config for a command invocation.
type Resolved struct {
	BaseURL     string
	Token       string
	WorkspaceID string
	UserID      string
	Profile     string
}

// Dir returns the config directory (~/.config/mello, overridable via
// MELLO_CONFIG_DIR).
func Dir() string {
	if d := os.Getenv("MELLO_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".mello-config"
	}
	return filepath.Join(home, ".config", "mello")
}

// Path returns the config file path.
func Path() string { return filepath.Join(Dir(), "config.json") }

// Load reads the config file, returning an empty store if it doesn't exist.
func Load() (*Store, error) {
	data, err := os.ReadFile(Path())
	if errors.Is(err, os.ErrNotExist) {
		return &Store{Current: "default", Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", Path(), err)
	}
	if s.Profiles == nil {
		s.Profiles = map[string]Profile{}
	}
	if s.Current == "" {
		s.Current = "default"
	}
	return &s, nil
}

// Save writes the config file at mode 0600.
func (s *Store) Save() error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(Path(), data, 0o600); err != nil {
		return err
	}
	// Re-assert mode in case the file pre-existed with looser perms.
	_ = os.Chmod(Path(), 0o600)
	return nil
}

// ActiveProfile returns the profile name to use: override > MELLO_PROFILE >
// current > "default".
func ActiveProfile(override string) string {
	if override != "" {
		return override
	}
	if p := os.Getenv("MELLO_PROFILE"); p != "" {
		return p
	}
	s, err := Load()
	if err == nil && s.Current != "" {
		return s.Current
	}
	return "default"
}

// Resolve computes the effective config: env > profile-file > default.
func Resolve(profileOverride string) (Resolved, error) {
	s, err := Load()
	if err != nil {
		return Resolved{}, err
	}
	name := ActiveProfile(profileOverride)
	prof := s.Profiles[name]

	baseURL := firstNonEmpty(os.Getenv("MELLO_BASE_URL"), prof.BaseURL, DefaultBaseURL)
	token := firstNonEmpty(os.Getenv("MELLO_TOKEN"), prof.Token)
	ws := firstNonEmpty(os.Getenv("MELLO_WORKSPACE"), prof.WorkspaceID)

	return Resolved{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		Token:       token,
		WorkspaceID: ws,
		UserID:      prof.UserID,
		Profile:     name,
	}, nil
}

// SetUserID records the authenticated user's id on a profile (used to resolve
// "me" in filters without an extra request).
func SetUserID(name, userID string) error {
	s, err := Load()
	if err != nil {
		return err
	}
	prof := s.Profiles[name]
	prof.UserID = userID
	s.Profiles[name] = prof
	return s.Save()
}

// SetProfile creates/updates a profile and optionally makes it current. Empty
// string args leave the corresponding field untouched.
func SetProfile(name, baseURL, token, workspaceID string, makeCurrent bool) error {
	s, err := Load()
	if err != nil {
		return err
	}
	prof := s.Profiles[name]
	if baseURL != "" {
		prof.BaseURL = strings.TrimRight(baseURL, "/")
	}
	if token != "" {
		prof.Token = token
	}
	if workspaceID != "" {
		prof.WorkspaceID = workspaceID
	}
	s.Profiles[name] = prof
	if makeCurrent {
		s.Current = name
	}
	return s.Save()
}

// ClearToken removes only the token from a profile (logout).
func ClearToken(name string) error {
	s, err := Load()
	if err != nil {
		return err
	}
	prof, ok := s.Profiles[name]
	if !ok {
		return nil
	}
	prof.Token = ""
	s.Profiles[name] = prof
	return s.Save()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
