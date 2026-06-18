// Package sync implements the local working copy of a Mello workspace.
//
// A working copy (the .mello directory) tracks one workspace and one or more
// boards beneath it. Each ticket is tracked at three reference points: the
// remote (the API), the baseline (state.json plus ticket.json, the last
// synchronized state used as the diff base), and the working copy (ticket.md
// and the comment/attachment files the user edits). Differences are detected by
// content hash. clone and pull bring remote state down; push applies local
// creates, updates, moves, and deletes; sync performs a pull followed by a push.
package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirName is the working-directory marker.
const DirName = ".mello"

// stateVersion is the on-disk schema version of state.json.
const stateVersion = 3

// State is the workspace ledger in .mello/state.json. It records the tracked
// workspace, a monotonic serial bumped on each successful synchronization, and
// the boards checked out beneath it.
type State struct {
	Version       int                    `json:"version"`
	Serial        int                    `json:"serial"`
	Profile       string                 `json:"profile,omitempty"`
	BaseURL       string                 `json:"base_url,omitempty"`
	WorkspaceID   string                 `json:"workspace_id"`
	WorkspaceName string                 `json:"workspace_name,omitempty"`
	CurrentBoard  string                 `json:"current_board,omitempty"` // default board slug
	Boards        map[string]*BoardState `json:"boards"`                  // keyed by board slug
}

// BoardState is one board checked out into the workspace.
type BoardState struct {
	BoardID string                   `json:"board_id"`
	Slug    string                   `json:"slug"`
	Name    string                   `json:"name,omitempty"`
	Code    string                   `json:"code,omitempty"`
	Cursor  string                   `json:"cursor,omitempty"` // updated_after (RFC3339)
	Tickets map[string]*TicketRecord `json:"tickets"`          // keyed by ticket slug
}

// TicketRecord is the baseline bookkeeping for one ticket. RemoteID empty means
// the ticket exists only locally and will be created on push.
type TicketRecord struct {
	Slug          string            `json:"slug"`
	RemoteID      string            `json:"remote_id,omitempty"`
	Code          string            `json:"code,omitempty"`
	ColumnID      string            `json:"column_id,omitempty"`
	ColumnName    string            `json:"column_name,omitempty"`
	BaselineHash  string            `json:"baseline_hash,omitempty"`
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
			return "", errors.New("not a mello workspace (no .mello here or in any parent) — run `mello init` or `mello sync clone` first")
		}
		dir = parent
	}
}

// Exists reports whether a .mello workspace exists in dir itself.
func Exists(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, DirName))
	return err == nil && fi.IsDir()
}

// Open loads the workspace whose .mello lives in or above start.
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

// InitWorkspace creates a fresh .mello workspace at root.
func InitWorkspace(root string, st *State) (*Tree, error) {
	st.Version = stateVersion
	if st.Boards == nil {
		st.Boards = map[string]*BoardState{}
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
	data, err := os.ReadFile(filepath.Join(root, DirName, "state.json"))
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	if st.Boards == nil {
		st.Boards = map[string]*BoardState{}
	}
	for _, bs := range st.Boards {
		if bs.Tickets == nil {
			bs.Tickets = map[string]*TicketRecord{}
		}
	}
	return &st, nil
}

// AddBoard registers a board in the workspace, making it current if it is the
// first one.
func (t *Tree) AddBoard(bs *BoardState) {
	if t.State.Boards == nil {
		t.State.Boards = map[string]*BoardState{}
	}
	if bs.Tickets == nil {
		bs.Tickets = map[string]*TicketRecord{}
	}
	t.State.Boards[bs.Slug] = bs
	if t.State.CurrentBoard == "" {
		t.State.CurrentBoard = bs.Slug
	}
}

// BoardSlugs returns the workspace's board slugs in sorted order.
func (t *Tree) BoardSlugs() []string {
	out := make([]string, 0, len(t.State.Boards))
	for slug := range t.State.Boards {
		out = append(out, slug)
	}
	sort.Strings(out)
	return out
}

// ResolveBoard selects a single board. A non-empty selector matches by slug,
// code, name, or id. An empty selector returns the only board, or the current
// board, and otherwise reports that a selection is required.
func (t *Tree) ResolveBoard(sel string) (*BoardState, error) {
	if sel != "" {
		for _, bs := range t.State.Boards {
			if bs.Slug == sel || bs.Code == sel || bs.Name == sel || bs.BoardID == sel {
				return bs, nil
			}
		}
		return nil, fmt.Errorf("no board %q in this workspace (see `mello board list`)", sel)
	}
	switch len(t.State.Boards) {
	case 0:
		return nil, errors.New("no working board — run `mello use <board>`")
	case 1:
		for _, bs := range t.State.Boards {
			return bs, nil
		}
	}
	if bs := t.State.Boards[t.State.CurrentBoard]; bs != nil {
		return bs, nil
	}
	return nil, errors.New("multiple boards in this workspace — specify -b <board>")
}

// SelectBoards returns the boards a command should act on: the selected one, or
// all boards when the selector is empty.
func (t *Tree) SelectBoards(sel string) ([]*BoardState, error) {
	if sel != "" {
		bs, err := t.ResolveBoard(sel)
		if err != nil {
			return nil, err
		}
		return []*BoardState{bs}, nil
	}
	if len(t.State.Boards) == 0 {
		return nil, errors.New("no boards in this workspace — run `mello sync clone -b <board>`")
	}
	out := make([]*BoardState, 0, len(t.State.Boards))
	for _, slug := range t.BoardSlugs() {
		out = append(out, t.State.Boards[slug])
	}
	return out, nil
}

// ---- board-relative paths ---------------------------------------------------

func (t *Tree) boardDir(boardSlug string) string {
	return filepath.Join(t.Root, DirName, "boards", boardSlug)
}

func (t *Tree) ticketsRoot(boardSlug string) string {
	return filepath.Join(t.boardDir(boardSlug), "tickets")
}

func (t *Tree) ticketDir(boardSlug, ticketSlug string) string {
	return filepath.Join(t.ticketsRoot(boardSlug), ticketSlug)
}

// TicketPath is the exported folder path for a ticket (used by `mello new`).
func (t *Tree) TicketPath(boardSlug, ticketSlug string) string {
	return t.ticketDir(boardSlug, ticketSlug)
}

func (t *Tree) scanTicketDirs(boardSlug string) []string {
	entries, err := os.ReadDir(t.ticketsRoot(boardSlug))
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

// uniqueSlug returns a ticket slug not already used in the board (in state or on
// disk).
func (t *Tree) uniqueSlug(bs *BoardState, base string) string {
	base = Slugify(base)
	slug := base
	for i := 2; ; i++ {
		_, inState := bs.Tickets[slug]
		_, statErr := os.Stat(t.ticketDir(bs.Slug, slug))
		if !inState && statErr != nil {
			return slug
		}
		slug = base + "-" + itoa(i)
	}
}

// UniqueSlug is the exported form used by `mello new`.
func (t *Tree) UniqueSlug(bs *BoardState, base string) string { return t.uniqueSlug(bs, base) }

// ---- board-scoped ticket bookkeeping ---------------------------------------

func (bs *BoardState) record(slug string) *TicketRecord {
	if bs.Tickets == nil {
		bs.Tickets = map[string]*TicketRecord{}
	}
	r := bs.Tickets[slug]
	if r == nil {
		r = &TicketRecord{Slug: slug, Attachments: map[string]string{}}
		bs.Tickets[slug] = r
	}
	if r.Attachments == nil {
		r.Attachments = map[string]string{}
	}
	return r
}

// FindTicketSlug resolves a ticket selector (slug, code, or remote id) to a
// tracked ticket slug in the working set.
func (bs *BoardState) FindTicketSlug(sel string) (string, bool) {
	if _, ok := bs.Tickets[sel]; ok {
		return sel, true
	}
	for slug, r := range bs.Tickets {
		if r.RemoteID == sel || (r.Code != "" && strings.EqualFold(r.Code, sel)) {
			return slug, true
		}
	}
	if s := Slugify(sel); s != sel {
		if _, ok := bs.Tickets[s]; ok {
			return s, true
		}
	}
	return "", false
}

// Untrack removes a ticket from the working set: its local folder and its state
// record. The remote ticket is not touched.
func (t *Tree) Untrack(bs *BoardState, ticketSlug string) {
	_ = os.RemoveAll(t.ticketDir(bs.Slug, ticketSlug))
	delete(bs.Tickets, ticketSlug)
}

func (bs *BoardState) slugByRemoteID() map[string]string {
	m := map[string]string{}
	for slug, r := range bs.Tickets {
		if r.RemoteID != "" {
			m[r.RemoteID] = slug
		}
	}
	return m
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
