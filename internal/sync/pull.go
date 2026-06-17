package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/mello"
)

// API is the subset of the Mello client the sync engine needs (an interface so
// it can be stubbed in tests).
type API interface {
	ListColumns(ctx context.Context, boardID string) ([]mello.Column, error)
	ListTickets(ctx context.Context, workspaceID, updatedAfter string) ([]mello.Ticket, error)
	GetTicket(ctx context.Context, ticketID string) (mello.Ticket, error)
	CreateTicket(ctx context.Context, columnID, title, description string) (mello.Ticket, error)
	UpdateTicket(ctx context.Context, ticketID string, upd mello.TicketUpdate) (mello.Ticket, error)
	DeleteTicket(ctx context.Context, ticketID string) error
	MoveTicket(ctx context.Context, ticketID, columnID string, position int) (mello.Ticket, error)
	ListComments(ctx context.Context, ticketID string) ([]mello.Comment, error)
	AddComment(ctx context.Context, ticketID, body string) (mello.Comment, error)
	ListAttachments(ctx context.Context, ticketID string) ([]mello.Attachment, error)
	UploadAttachment(ctx context.Context, ticketID, filePath string) (mello.Attachment, error)
	DownloadAttachment(ctx context.Context, ticketID string, att mello.Attachment, w io.Writer) error
}

// Logf is an optional progress sink.
type Logf func(format string, a ...any)

// Syncer drives clone/pull/push for a single board within a Tree.
type Syncer struct {
	API   API
	Tree  *Tree
	Board *BoardState
	Log   Logf

	// Note, when set, is posted as a comment on each ticket changed by a push.
	Note string

	// capability flags discovered at runtime (optional endpoints).
	noComments    bool
	noAttachments bool

	// conflicts collected during a pull, for reporting.
	Conflicts []string
}

func (s *Syncer) logf(format string, a ...any) {
	if s.Log != nil {
		s.Log(format, a...)
	}
}

// ticketDir is the folder for a ticket slug on this Syncer's board.
func (s *Syncer) ticketDir(slug string) string {
	return s.Tree.ticketDir(s.Board.Slug, slug)
}

func (s *Syncer) record(slug string) *TicketRecord { return s.Board.record(slug) }

func (s *Syncer) columnMaps(ctx context.Context) (idToName, nameToID map[string]string, err error) {
	cols, err := s.API.ListColumns(ctx, s.Board.BoardID)
	if err != nil {
		return nil, nil, err
	}
	idToName = map[string]string{}
	nameToID = map[string]string{}
	for _, c := range cols {
		idToName[c.ID] = c.Name
		nameToID[c.Name] = c.ID
	}
	return idToName, nameToID, nil
}

// Clone performs the initial full pull using columns (with nested tickets).
func (s *Syncer) Clone(ctx context.Context) (int, error) {
	cols, err := s.API.ListColumns(ctx, s.Board.BoardID)
	if err != nil {
		return 0, err
	}
	idToName := map[string]string{}
	for _, c := range cols {
		idToName[c.ID] = c.Name
	}
	count := 0
	var maxUpdated string
	for _, c := range cols {
		for _, t := range c.Tickets {
			if t.ColumnID == "" {
				t.ColumnID = c.ID
			}
			slug := s.desiredSlug(t)
			if err := s.writeTicket(ctx, t, c.Name, slug, true); err != nil {
				return count, err
			}
			count++
			maxUpdated = maxTime(maxUpdated, t)
		}
	}
	if maxUpdated != "" {
		s.Board.Cursor = maxUpdated
	}
	s.Tree.State.Serial++
	return count, s.Tree.Save()
}

// Pull performs an incremental refresh using the saved cursor. Locally-modified
// tickets are preserved; tickets changed on both sides are flagged as conflicts.
func (s *Syncer) Pull(ctx context.Context) (updated, deleted int, err error) {
	idToName, _, err := s.columnMaps(ctx)
	if err != nil {
		return 0, 0, err
	}
	tickets, err := s.API.ListTickets(ctx, s.Tree.State.WorkspaceID, s.Board.Cursor)
	if err != nil {
		return 0, 0, err
	}
	byRemote := s.Board.slugByRemoteID()
	var maxUpdated string
	for _, t := range tickets {
		if t.BoardID != "" && t.BoardID != s.Board.BoardID {
			maxUpdated = maxTime(maxUpdated, t)
			continue
		}
		if strings.EqualFold(t.Status, "deleted") {
			if slug, ok := byRemote[t.ID]; ok && s.removeTicket(slug) {
				deleted++
			}
			maxUpdated = maxTime(maxUpdated, t)
			continue
		}
		slug := byRemote[t.ID]
		if slug == "" {
			slug = s.desiredSlug(t)
		}
		if err := s.writeTicket(ctx, t, idToName[t.ColumnID], slug, true); err != nil {
			return updated, deleted, err
		}
		updated++
		maxUpdated = maxTime(maxUpdated, t)
	}
	if maxUpdated != "" {
		s.Board.Cursor = maxUpdated
	}
	s.Tree.State.Serial++
	return updated, deleted, s.Tree.Save()
}

// PullTicket fetches a single ticket (by code or id) into the working set,
// returning the ticket and its local folder path.
func (s *Syncer) PullTicket(ctx context.Context, sel string) (mello.Ticket, string, error) {
	cols, err := s.API.ListColumns(ctx, s.Board.BoardID)
	if err != nil {
		return mello.Ticket{}, "", err
	}
	idToName := map[string]string{}
	var found *mello.Ticket
	var colName string
	for _, c := range cols {
		idToName[c.ID] = c.Name
		for i := range c.Tickets {
			t := c.Tickets[i]
			if t.ColumnID == "" {
				t.ColumnID = c.ID
			}
			if t.ID == sel || strings.EqualFold(t.TicketCode, sel) {
				found = &t
				colName = c.Name
			}
		}
	}
	if found == nil {
		return mello.Ticket{}, "", fmt.Errorf("no ticket %q on board %s (see `mello ticket list`)", sel, s.Board.Name)
	}
	if full, gerr := s.API.GetTicket(ctx, found.ID); gerr == nil {
		found = &full
		if n, ok := idToName[full.ColumnID]; ok {
			colName = n
		}
	}
	slug := s.desiredSlug(*found)
	if err := s.writeTicket(ctx, *found, colName, slug, true); err != nil {
		return mello.Ticket{}, "", err
	}
	s.Tree.State.Serial++
	if err := s.Tree.Save(); err != nil {
		return mello.Ticket{}, "", err
	}
	return *found, s.ticketDir(slug), nil
}

// RefreshWorkingSet re-fetches the tickets already present in the working set
// (leaving the rest of the board untouched). Tickets deleted on the server are
// removed locally.
func (s *Syncer) RefreshWorkingSet(ctx context.Context) (updated, deleted int, err error) {
	idToName, _, err := s.columnMaps(ctx)
	if err != nil {
		return 0, 0, err
	}
	type item struct{ slug, id string }
	var items []item
	for slug, rec := range s.Board.Tickets {
		if rec.RemoteID != "" {
			items = append(items, item{slug, rec.RemoteID})
		}
	}
	for _, it := range items {
		t, gerr := s.API.GetTicket(ctx, it.id)
		if gerr != nil {
			if mello.IsNotFound(gerr) {
				if s.removeTicket(it.slug) {
					deleted++
				}
				continue
			}
			return updated, deleted, gerr
		}
		if werr := s.writeTicket(ctx, t, idToName[t.ColumnID], it.slug, true); werr != nil {
			return updated, deleted, werr
		}
		updated++
	}
	s.Tree.State.Serial++
	return updated, deleted, s.Tree.Save()
}

// writeTicket writes ticket.md + ticket.json into the given slug folder, records
// the baseline hash, and pulls comments/attachments. allowPreserve protects
// local edits during a pull (and flags conflicts when the remote also changed).
func (s *Syncer) writeTicket(ctx context.Context, t mello.Ticket, columnName, slug string, allowPreserve bool) error {
	dir := s.ticketDir(slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mdPath := filepath.Join(dir, "ticket.md")
	jsonPath := filepath.Join(dir, "ticket.json")
	remoteHash := HashTicket(t, columnName)
	rec := s.record(slug)

	preserve := false
	if allowPreserve {
		if existingMD, err := os.ReadFile(mdPath); err == nil {
			doc, perr := ParseTicket(existingMD)
			if perr == nil {
				localModified := HashDoc(doc) != rec.BaselineHash && rec.BaselineHash != ""
				remoteChanged := remoteHash != rec.BaselineHash && rec.BaselineHash != ""
				if localModified && remoteChanged {
					preserve = true // conflict: keep the working copy, refresh baseline
					s.Conflicts = append(s.Conflicts, slug)
					s.logf("CONFLICT %s: both local and remote changed — review ticket.remote.json", refOf(t))
					remote, _ := json.MarshalIndent(t, "", "  ")
					_ = os.WriteFile(filepath.Join(dir, "ticket.remote.json"), remote, 0o644)
				} else if localModified && !remoteChanged {
					preserve = true // local-only edit, nothing new from remote: keep it
				}
			}
		}
	}

	raw, _ := json.MarshalIndent(t, "", "  ")
	if err := os.WriteFile(jsonPath, raw, 0o644); err != nil {
		return err
	}
	if !preserve {
		if err := os.WriteFile(mdPath, RenderTicket(t, columnName), 0o644); err != nil {
			return err
		}
		// Clean baseline matches remote.
		_ = os.Remove(filepath.Join(dir, "ticket.remote.json"))
	}

	rec.Slug = slug
	rec.RemoteID = t.ID
	rec.Code = t.TicketCode
	rec.ColumnID = t.ColumnID
	rec.BaselineHash = remoteHash
	if t.UpdatedAt != nil {
		rec.RemoteUpdated = t.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}

	if !s.noComments {
		if ids, err := s.pullComments(ctx, dir, t.ID); err == nil {
			rec.CommentIDs = ids
		} else if mello.IsNotFound(err) {
			s.noComments = true
		} else {
			return err
		}
	}
	if !s.noAttachments {
		if hashes, err := s.pullAttachments(ctx, dir, t.ID); err == nil {
			rec.Attachments = hashes
		} else if mello.IsNotFound(err) {
			s.noAttachments = true
		} else {
			return err
		}
	}
	return nil
}

func (s *Syncer) pullComments(ctx context.Context, ticketDir, ticketID string) ([]string, error) {
	comments, err := s.API.ListComments(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(ticketDir, "comments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	raw, _ := json.MarshalIndent(comments, "", "  ")
	_ = os.WriteFile(filepath.Join(ticketDir, "comments.json"), raw, 0o644)

	ids := make([]string, 0, len(comments))
	for i, cm := range comments {
		ids = append(ids, cm.ID)
		name := fmt.Sprintf("%04d-%s.md", i+1, Slugify(firstNonEmpty(cm.AuthorID, "author")))
		var b strings.Builder
		b.WriteString("---\n")
		writeKV(&b, "id", cm.ID)
		writeKV(&b, "author", cm.AuthorID)
		if cm.CreatedAt != nil {
			writeKV(&b, "created_at", cm.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"))
		}
		b.WriteString("---\n\n")
		b.WriteString(cm.Body)
		b.WriteString("\n")
		_ = os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o644)
	}
	return ids, nil
}

func (s *Syncer) pullAttachments(ctx context.Context, ticketDir, ticketID string) (map[string]string, error) {
	atts, err := s.API.ListAttachments(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(ticketDir, "attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	raw, _ := json.MarshalIndent(atts, "", "  ")
	_ = os.WriteFile(filepath.Join(ticketDir, "attachments.json"), raw, 0o644)

	hashes := map[string]string{}
	for _, a := range atts {
		name := a.FileName()
		dest := filepath.Join(dir, name)
		if _, err := os.Stat(dest); err != nil {
			f, err := os.Create(dest)
			if err != nil {
				return hashes, err
			}
			derr := s.API.DownloadAttachment(ctx, ticketID, a, f)
			_ = f.Close()
			if derr != nil {
				return hashes, derr
			}
		}
		if h, err := HashFile(dest); err == nil {
			hashes[name] = h
		}
	}
	return hashes, nil
}

func (s *Syncer) removeTicket(slug string) bool {
	if _, ok := s.Board.Tickets[slug]; !ok {
		return false
	}
	_ = os.RemoveAll(s.ticketDir(slug))
	delete(s.Board.Tickets, slug)
	return true
}

// desiredSlug reuses the recorded slug for a known remote id, else derives a
// stable, unique slug from the ticket code/id.
func (s *Syncer) desiredSlug(t mello.Ticket) string {
	for slug, r := range s.Board.Tickets {
		if r.RemoteID == t.ID && t.ID != "" {
			return slug
		}
	}
	base := t.TicketCode
	if base == "" {
		base = t.ID
	}
	return s.Tree.uniqueSlug(s.Board, base)
}

func maxTime(cur string, t mello.Ticket) string {
	if t.UpdatedAt == nil {
		return cur
	}
	ts := t.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	if ts > cur {
		return ts
	}
	return cur
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
