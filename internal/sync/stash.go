package sync

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Stash lets you set aside locally-modified tickets so the working copy can be
// reset to its baseline and a pull performed cleanly, then reapply the saved
// changes afterwards (the local analogue of `git stash`). Snapshots live under
// .mello/stash/<id>/ and never touch the server.

// StashEntry is one saved snapshot of un-pushed ticket changes.
type StashEntry struct {
	ID        string        `json:"id"`
	Message   string        `json:"message,omitempty"`
	CreatedAt string        `json:"created_at"`
	Board     string        `json:"board"`
	Tickets   []StashTicket `json:"tickets"`
}

// StashTicket records one stashed ticket folder.
type StashTicket struct {
	Slug    string `json:"slug"`
	Ref     string `json:"ref,omitempty"`
	Created bool   `json:"created,omitempty"` // local-only ticket (no remote baseline)
}

func (t *Tree) stashRoot() string { return filepath.Join(t.Root, DirName, "stash") }

// Stash snapshots every locally-modified ticket on the Syncer's board, then
// resets each to its baseline (tickets that exist only locally are removed and
// fully captured in the stash). It returns the saved entry, or nil if there was
// nothing to stash. Offline — it makes no API calls.
func (s *Syncer) Stash(message string) (*StashEntry, error) {
	entry := &StashEntry{
		ID:        s.nextStashID(),
		Message:   message,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Board:     s.Board.Slug,
	}
	for _, slug := range s.workingSlugs() {
		dir := s.ticketDir(slug)
		rec := s.Board.Tickets[slug]
		dirty, created := dirtyTicket(dir, rec)
		if !dirty {
			continue
		}
		dest := filepath.Join(s.Tree.stashRoot(), entry.ID, slug)
		if err := copyTree(dir, dest); err != nil {
			return nil, err
		}
		entry.Tickets = append(entry.Tickets, StashTicket{Slug: slug, Ref: stashRef(rec, slug), Created: created})
		if created {
			_ = os.RemoveAll(dir)
			delete(s.Board.Tickets, slug)
		} else if err := resetTicketToBaseline(dir, rec); err != nil {
			return nil, err
		}
	}
	if len(entry.Tickets) == 0 {
		return nil, nil
	}
	if err := s.Tree.writeStashMeta(entry); err != nil {
		return nil, err
	}
	return entry, s.Tree.Save()
}

// StashApply restores a stashed entry over the working copy. When pop is true the
// entry is dropped afterwards. If a restored ticket.md diverges from the current
// one (e.g. a version just pulled from the server), the current file is kept as
// ticket.remote.md and a warning is logged so the user can reconcile.
func (s *Syncer) StashApply(entry *StashEntry, pop bool) error {
	for _, st := range entry.Tickets {
		src := filepath.Join(s.Tree.stashRoot(), entry.ID, st.Slug)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("stash %s is missing files for %s", entry.ID, stashRefName(st))
		}
		dir := s.ticketDir(st.Slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		curMD := filepath.Join(dir, "ticket.md")
		stMD := filepath.Join(src, "ticket.md")
		if differs(curMD, stMD) {
			if data, rerr := os.ReadFile(curMD); rerr == nil {
				_ = os.WriteFile(filepath.Join(dir, "ticket.remote.md"), data, 0o644)
				s.logf("! %s: current ticket.md kept as ticket.remote.md — reconcile", stashRefName(st))
			}
		}
		if err := copyTree(src, dir); err != nil {
			return err
		}
		s.logf("~ restored %s", stashRefName(st))
	}
	if pop {
		return s.Tree.DropStash(entry.ID)
	}
	return nil
}

// workingSlugs is the union of ticket folders on disk and tracked records.
func (s *Syncer) workingSlugs() []string {
	set := map[string]bool{}
	for _, slug := range s.Tree.scanTicketDirs(s.Board.Slug) {
		set[slug] = true
	}
	for slug := range s.Board.Tickets {
		set[slug] = true
	}
	out := make([]string, 0, len(set))
	for slug := range set {
		out = append(out, slug)
	}
	sort.Strings(out)
	return out
}

func (s *Syncer) nextStashID() string {
	max := 0
	if list, err := s.Tree.ListStash(); err == nil {
		for _, e := range list {
			var n int
			if _, err := fmt.Sscanf(e.ID, "stash-%d", &n); err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("stash-%d", max+1)
}

// dirtyTicket reports whether a ticket folder has local modifications versus its
// baseline, without any network access. created is true for a ticket that exists
// only locally (no remote id yet).
func dirtyTicket(dir string, rec *TicketRecord) (dirty, created bool) {
	data, err := os.ReadFile(filepath.Join(dir, "ticket.md"))
	if err != nil {
		return false, false
	}
	if rec == nil || rec.RemoteID == "" {
		return true, true
	}
	doc, err := ParseTicket(data)
	if err != nil {
		return false, false
	}
	base, err := readBaseline(dir)
	if err != nil {
		return false, false
	}
	baseCol := firstNonEmpty(rec.ColumnName, base.ColumnName)
	switch {
	case HashDoc(doc) != rec.BaselineHash:
		return true, false
	case doc.Column != "" && baseCol != "" && doc.Column != baseCol:
		return true, false
	case len(scanNewComments(dir)) > 0:
		return true, false
	case len(scanNewAttachments(dir, rec.Attachments)) > 0:
		return true, false
	}
	return false, false
}

// resetTicketToBaseline rewrites ticket.md from the baseline and removes
// un-pushed comments and new/changed attachment files (all captured in the stash
// beforehand). Baseline attachment bytes are restored on the next pull.
func resetTicketToBaseline(dir string, rec *TicketRecord) error {
	base, err := readBaseline(dir)
	if err != nil {
		return err
	}
	col := firstNonEmpty(rec.ColumnName, base.ColumnName)
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), RenderTicket(base, col), 0o644); err != nil {
		return err
	}
	for _, pc := range scanNewComments(dir) {
		_ = os.Remove(pc.Path)
	}
	for _, p := range scanNewAttachments(dir, rec.Attachments) {
		_ = os.Remove(p)
	}
	_ = os.Remove(filepath.Join(dir, "ticket.remote.json"))
	return nil
}

func stashRef(rec *TicketRecord, slug string) string {
	if rec != nil && rec.Code != "" {
		return rec.Code
	}
	return slug
}

func stashRefName(st StashTicket) string {
	if st.Ref != "" {
		return st.Ref
	}
	return st.Slug
}

// ---- stash store -----------------------------------------------------------

func (t *Tree) writeStashMeta(e *StashEntry) error {
	dir := filepath.Join(t.stashRoot(), e.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(e, "", "  ")
	return os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o644)
}

// ListStash returns all stash entries, newest first.
func (t *Tree) ListStash() ([]StashEntry, error) {
	entries, err := os.ReadDir(t.stashRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []StashEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(t.stashRoot(), e.Name(), "meta.json"))
		if rerr != nil {
			continue
		}
		var se StashEntry
		if json.Unmarshal(data, &se) == nil {
			out = append(out, se)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// GetStash returns a stash entry by id, or the latest when id is empty.
func (t *Tree) GetStash(id string) (*StashEntry, error) {
	list, err := t.ListStash()
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no stash entries (nothing was stashed)")
	}
	if id == "" {
		return &list[0], nil
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("no stash entry %q (see `mello stash list`)", id)
}

// DropStash deletes a stash entry's saved files.
func (t *Tree) DropStash(id string) error {
	return os.RemoveAll(filepath.Join(t.stashRoot(), id))
}

// ---- file helpers ----------------------------------------------------------

// copyTree recursively copies src into dst, creating directories and overwriting
// files; files already present in dst that are absent from src are left intact.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// differs reports whether two files have different contents. A file that exists
// on only one side counts as differing; both missing counts as equal.
func differs(a, b string) bool {
	da, ea := os.ReadFile(a)
	db, eb := os.ReadFile(b)
	if ea != nil && eb != nil {
		return false
	}
	if ea != nil || eb != nil {
		return true
	}
	return string(da) != string(db)
}
