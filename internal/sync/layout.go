// Package sync implements the local working copy of a Mello board.
//
// Each ticket is tracked at three reference points: the remote (the API), the
// baseline (.mello/state.json plus ticket.json, the last synchronized state used
// as the diff base), and the working copy (ticket.md and the comment/attachment
// files the user edits). Differences are detected by content hash. clone and
// pull bring remote state down; push applies local creates, updates, moves, and
// deletes; sync performs a pull followed by a push.
package sync

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirName is the working-directory marker (like .git).
const DirName = ".mello"

// stateVersion is the on-disk schema version of state.json.
const stateVersion = 2

// State is the ledger in .mello/state.json: what this checkout tracks, the
// incremental cursor, a monotonically increasing serial (a version stamp bumped
// on every successful sync), and per-ticket baselines keyed by stable local slug.
type State struct {
	Version     int                      `json:"version"`
	Serial      int                      `json:"serial"`
	Profile     string                   `json:"profile,omitempty"`
	BaseURL     string                   `json:"base_url,omitempty"`
	WorkspaceID string                   `json:"workspace_id"`
	BoardID     string                   `json:"board_id"`
	BoardSlug   string                   `json:"board_slug"`
	BoardName   string                   `json:"board_name,omitempty"`
	Cursor      string                   `json:"cursor,omitempty"` // updated_after (RFC3339)
	Tickets     map[string]*TicketRecord `json:"tickets"`          // keyed by local slug
}

// TicketRecord is the baseline bookkeeping for one ticket. RemoteID empty means
// the ticket exists only locally and will be CREATED on push.
type TicketRecord struct {
	Slug          string            `json:"slug"`
	RemoteID      string            `json:"remote_id,omitempty"`
	Code          string            `json:"code,omitempty"` // ticket code, for display
	ColumnID      string            `json:"column_id,omitempty"`
	BaselineHash  string            `json:"baseline_hash,omitempty"` // HashTicket at last sync
	RemoteUpdated string            `json:"remote_updated_at,omitempty"`
	CommentIDs    []string          `json:"comment_ids,omitempty"`
	Attachments   map[string]string `json:"attachments,omitempty"` // filename -> content hash
}

// Tree is an opened working copy rooted at the dir that contains .mello.
type Tree struct {
	Root  string
	State *State
}

// FindRoot walks up from start looking for a .mello directory.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, DirName)); err == nil && fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not a mello working copy (no .mello here or in any parent) — run `mello sync clone` first")
		}
		dir = parent
	}
}

// Open loads the working copy whose .mello lives in or above start.
func Open(start string) (*Tree, error) {
	root, err := FindRoot(start)
	if err != nil {
		return nil, err
	}
	st, err := loadState(root)
	if err != nil {
		return nil, err
	}
	return &Tree{Root: root, State: st}, nil
}

// Init creates a fresh .mello working copy at root with the given state.
func Init(root string, st *State) (*Tree, error) {
	st.Version = stateVersion
	if st.Tickets == nil {
		st.Tickets = map[string]*TicketRecord{}
	}
	if err := os.MkdirAll(filepath.Join(root, DirName), 0o755); err != nil {
		return nil, err
	}
	t := &Tree{Root: root, State: st}
	return t, t.Save()
}

func (t *Tree) statePath() string { return filepath.Join(t.Root, DirName, "state.json") }

// Save persists state.json.
func (t *Tree) Save() error {
	data, err := json.MarshalIndent(t.State, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(t.statePath(), data, 0o644)
}

func loadState(root string) (*State, error) {
	// state.json is canonical; fall back to the legacy "config" name.
	path := filepath.Join(root, DirName, "state.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data, err = os.ReadFile(filepath.Join(root, DirName, "config"))
	}
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	if st.Tickets == nil {
		st.Tickets = map[string]*TicketRecord{}
	}
	return &st, nil
}

// record returns (creating if needed) the record for a slug.
func (t *Tree) record(slug string) *TicketRecord {
	r := t.State.Tickets[slug]
	if r == nil {
		r = &TicketRecord{Slug: slug, Attachments: map[string]string{}}
		t.State.Tickets[slug] = r
	}
	if r.Attachments == nil {
		r.Attachments = map[string]string{}
	}
	return r
}

// slugByRemoteID builds a reverse index for pull (remote id -> local slug).
func (t *Tree) slugByRemoteID() map[string]string {
	m := map[string]string{}
	for slug, r := range t.State.Tickets {
		if r.RemoteID != "" {
			m[r.RemoteID] = slug
		}
	}
	return m
}

// boardDir returns .../.mello/boards/<board-slug>.
func (t *Tree) boardDir() string {
	return filepath.Join(t.Root, DirName, "boards", t.State.BoardSlug)
}

func (t *Tree) ticketsRoot() string { return filepath.Join(t.boardDir(), "tickets") }

// ticketDir returns the folder for a ticket slug.
func (t *Tree) ticketDir(slug string) string {
	return filepath.Join(t.ticketsRoot(), slug)
}

// TicketPath is the exported folder path for a ticket slug (used by `mello new`).
func (t *Tree) TicketPath(slug string) string { return t.ticketDir(slug) }

// UniqueSlug returns an unused slug derived from base.
func (t *Tree) UniqueSlug(base string) string { return t.uniqueSlug(base) }

// scanTicketDirs returns the slugs of every ticket folder present on disk.
func (t *Tree) scanTicketDirs() []string {
	entries, err := os.ReadDir(t.ticketsRoot())
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// uniqueSlug returns a slug not already used by another ticket folder/record.
func (t *Tree) uniqueSlug(base string) string {
	base = Slugify(base)
	slug := base
	for i := 2; ; i++ {
		_, inState := t.State.Tickets[slug]
		_, statErr := os.Stat(t.ticketDir(slug))
		if !inState && statErr != nil {
			return slug
		}
		slug = base + "-" + itoa(i)
	}
}

// Slugify makes a filesystem-safe, lowercase slug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
