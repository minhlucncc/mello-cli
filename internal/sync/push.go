package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/minhlucncc/mello-cli/internal/mello"
)

// ComputePlan walks the working copy and returns the pending changes per ticket.
// When checkRemote is true it fetches each tracked ticket to detect remote drift
// and classify conflicts (both sides changed) and remote-only changes.
func (s *Syncer) ComputePlan(ctx context.Context, checkRemote bool) (Plan, error) {
	idToName, _, err := s.columnMaps(ctx)
	if err != nil {
		return Plan{}, err
	}

	// Union of slugs present on disk and tracked in state.
	slugSet := map[string]bool{}
	for _, slug := range s.Tree.scanTicketDirs(s.Board.Slug) {
		slugSet[slug] = true
	}
	for slug := range s.Board.Tickets {
		slugSet[slug] = true
	}
	slugs := make([]string, 0, len(slugSet))
	for slug := range slugSet {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	var plan Plan
	for _, slug := range slugs {
		dir := s.ticketDir(slug)
		rec := s.Board.Tickets[slug]
		_, statErr := os.Stat(filepath.Join(dir, "ticket.md"))
		folderExists := statErr == nil

		// Folder removed → untrack locally (the remote ticket is kept; use
		// `mello ticket delete` to remove it on the server).
		if !folderExists {
			if rec != nil {
				delete(s.Board.Tickets, slug)
			}
			continue
		}

		doc, err := readDoc(dir)
		if err != nil {
			continue
		}

		// New locally: no record or never pushed.
		if rec == nil || rec.RemoteID == "" {
			plan.Changes = append(plan.Changes, Change{
				Slug: slug, Ref: firstNonEmpty(doc.Ticket, slug), Kind: KindCreate,
				CreateDoc: &doc, CreateColumn: doc.Column,
				NewComments:    scanNewComments(dir),
				NewAttachments: scanNewAttachments(dir, nil),
			})
			continue
		}

		// Tracked: diff working vs baseline.
		base, err := readBaseline(dir)
		if err != nil {
			continue
		}
		fieldUpd, fieldChanged := diffFields(doc, base)
		baseCol := idToName[base.ColumnID]
		move := ""
		if doc.Column != "" && baseCol != "" && doc.Column != baseCol {
			move = doc.Column
		}
		newComments := scanNewComments(dir)
		newAtt := scanNewAttachments(dir, rec.Attachments)
		localModified := HashDoc(doc) != rec.BaselineHash || move != "" ||
			len(newComments) > 0 || len(newAtt) > 0

		kind := KindNone
		if localModified {
			kind = KindUpdate
		}
		if checkRemote {
			if cur, gerr := s.API.GetTicket(ctx, rec.RemoteID); gerr == nil {
				remoteHash := HashTicket(cur, idToName[cur.ColumnID])
				remoteChanged := remoteHash != rec.BaselineHash
				switch {
				case remoteChanged && localModified:
					kind = KindConflict
				case remoteChanged && !localModified:
					kind = KindRemote
				}
			}
		}
		if kind == KindNone {
			continue
		}
		plan.Changes = append(plan.Changes, Change{
			Slug: slug, Ref: refDoc(doc, base), RemoteID: rec.RemoteID, ColumnID: base.ColumnID,
			Kind: kind, Update: fieldUpd, HasFieldChange: fieldChanged, UpdateBodyFormat: doc.BodyFormat,
			MoveToColumn:   move,
			NewComments:    newComments,
			NewAttachments: newAtt,
		})
	}
	return plan, nil
}

// Apply executes a plan. dryRun mutates nothing; force pushes conflicts (local
// over remote).
func (s *Syncer) Apply(ctx context.Context, plan Plan, dryRun, force bool) error {
	idToName, nameToID, err := s.columnMaps(ctx)
	if err != nil {
		return err
	}
	// Pre-flight: a push must never clobber remote changes. If the remote drifted
	// since the last sync — a conflict (both sides changed) or a remote-only
	// change to a tracked ticket — refuse the whole push and require a pull/sync
	// first. --force overrides (local wins); --dry-run still describes the plan.
	if !dryRun && !force {
		var blocked []string
		for _, ch := range plan.Changes {
			if ch.Kind == KindConflict || ch.Kind == KindRemote {
				blocked = append(blocked, ch.Ref)
			}
		}
		if len(blocked) > 0 {
			return fmt.Errorf("remote changed since last sync (%s) — run `mello pull` (or `mello sync`) before pushing; use --force to overwrite, or `mello stash` to set local changes aside",
				strings.Join(blocked, ", "))
		}
	}
	applied := 0
	for _, ch := range plan.Changes {
		if dryRun {
			s.describe(ch)
			continue
		}
		switch ch.Kind {
		case KindRemote:
			s.logf("skip %s: remote changed — run `mello sync pull`", ch.Ref)
			continue
		case KindConflict:
			if !force {
				s.logf("skip %s: CONFLICT — reconcile then push, or use --force", ch.Ref)
				continue
			}
		}

		switch ch.Kind {
		case KindDelete:
			if err := s.API.DeleteTicket(ctx, ch.RemoteID); err != nil {
				if mello.IsNotFound(err) {
					s.logf("skip delete %s: not supported here", ch.Ref)
				} else {
					return err
				}
			} else {
				delete(s.Board.Tickets, ch.Slug)
				s.logf("- deleted %s", ch.Ref)
				applied++
			}
			continue

		case KindCreate:
			if err := s.applyCreate(ctx, ch, idToName, nameToID); err != nil {
				return err
			}
			applied++
			continue

		default: // KindUpdate or forced KindConflict
			if err := s.applyUpdate(ctx, ch, idToName, nameToID); err != nil {
				return err
			}
			applied++
		}
	}
	if dryRun {
		return nil
	}
	s.Tree.State.Serial++
	s.appendJournal(plan)
	return s.Tree.Save()
}

func (s *Syncer) applyCreate(ctx context.Context, ch Change, idToName, nameToID map[string]string) error {
	doc := ch.CreateDoc
	colID, ok := nameToID[ch.CreateColumn]
	if !ok {
		s.logf("skip create %s: no column named %q", ch.Ref, ch.CreateColumn)
		return nil
	}
	outPlain, outHTML := outgoingDescription(*doc)
	t, err := s.API.CreateTicket(ctx, colID, doc.Title, outPlain, outHTML)
	if err != nil {
		return err
	}
	// Apply any non-default fields the create endpoint doesn't accept.
	extra := mello.TicketUpdate{}
	if doc.Status != "" {
		extra.Status = &doc.Status
	}
	if doc.Assignee != "" {
		extra.AssigneeID = &doc.Assignee
	}
	if len(doc.Labels) > 0 {
		l := append([]string(nil), doc.Labels...)
		extra.Labels = &l
	}
	if extra.Status != nil || extra.AssigneeID != nil || extra.Labels != nil {
		if u, uerr := s.API.UpdateTicket(ctx, t.ID, extra); uerr == nil {
			t = u
		} else if !mello.IsNotFound(uerr) {
			return uerr
		}
	}
	for _, pc := range ch.NewComments {
		if _, err := s.API.AddComment(ctx, t.ID, pc.Body); err != nil {
			return err
		}
		_ = os.Remove(pc.Path)
	}
	for _, path := range ch.NewAttachments {
		if _, err := s.API.UploadAttachment(ctx, t.ID, path); err != nil && !mello.IsNotFound(err) {
			return err
		}
	}
	s.logf("+ created %s (%s)", firstNonEmpty(t.TicketCode, shortRef(t.ID)), ch.Slug)
	s.postNote(ctx, t.ID, ch.Ref)
	fresh, err := s.API.GetTicket(ctx, t.ID)
	if err != nil {
		fresh = t
	}
	return s.writeTicket(ctx, fresh, idToName[fresh.ColumnID], ch.Slug, false)
}

func (s *Syncer) applyUpdate(ctx context.Context, ch Change, idToName, nameToID map[string]string) error {
	if ch.HasFieldChange {
		// If the working doc had a description change, the diff helper stashed
		// the raw body in ch.Update.Description. Convert it to the right
		// (description, description_html) pair here, at the last moment, so the
		// bytes the server sees render in the web UI.
		if ch.Update.Description != nil {
			plain, html := outgoingDescription(TicketDoc{Description: *ch.Update.Description, BodyFormat: ch.UpdateBodyFormat})
			if plain == "" {
				ch.Update.Description = nil
			} else {
				ch.Update.Description = &plain
			}
			if html == "" {
				ch.Update.DescriptionHTML = nil
			} else {
				ch.Update.DescriptionHTML = &html
			}
		}
		if _, err := s.API.UpdateTicket(ctx, ch.RemoteID, ch.Update); err != nil {
			if mello.IsNotFound(err) {
				s.logf("skip field edit on %s: PATCH not supported here", ch.Ref)
			} else {
				return err
			}
		} else {
			s.logf("~ updated fields: %s", ch.Ref)
		}
	}
	if ch.MoveToColumn != "" {
		if colID, ok := nameToID[ch.MoveToColumn]; ok {
			if _, err := s.API.MoveTicket(ctx, ch.RemoteID, colID, 0); err != nil {
				return err
			}
			s.logf("~ moved %s → %s", ch.Ref, ch.MoveToColumn)
		} else {
			s.logf("skip move on %s: no column named %q", ch.Ref, ch.MoveToColumn)
		}
	}
	for _, pc := range ch.NewComments {
		if _, err := s.API.AddComment(ctx, ch.RemoteID, pc.Body); err != nil {
			return err
		}
		_ = os.Remove(pc.Path)
		s.logf("~ commented on %s", ch.Ref)
	}
	if len(ch.NewAttachments) > 0 {
		// Map existing remote attachments by filename so a changed file replaces
		// its predecessor instead of piling up duplicates with the same name.
		existing := map[string][]string{}
		if atts, lerr := s.API.ListAttachments(ctx, ch.RemoteID); lerr == nil {
			for _, a := range atts {
				name := a.FileName()
				existing[name] = append(existing[name], a.ID)
			}
		} else if !mello.IsNotFound(lerr) {
			return lerr
		}
		for _, path := range ch.NewAttachments {
			name := filepath.Base(path)
			// Delete any prior remote attachments with this name first (replace).
			for _, id := range existing[name] {
				if derr := s.API.DeleteAttachment(ctx, ch.RemoteID, id); derr != nil && !mello.IsNotFound(derr) {
					return derr
				}
			}
			if _, err := s.API.UploadAttachment(ctx, ch.RemoteID, path); err != nil {
				if mello.IsNotFound(err) {
					s.logf("skip attachment on %s: uploads not supported here", ch.Ref)
					break
				}
				return err
			}
			if len(existing[name]) > 0 {
				s.logf("~ replaced %s on %s", name, ch.Ref)
			} else {
				s.logf("~ uploaded %s to %s", name, ch.Ref)
			}
			delete(existing, name) // a name handled once won't be re-deleted
		}
	}
	s.postNote(ctx, ch.RemoteID, ch.Ref)
	if t, err := s.API.GetTicket(ctx, ch.RemoteID); err == nil {
		// Restore the Markdown description from the working copy into the
		// fetched ticket so the baseline hash stays consistent. The API
		// returns plain text in `description` when `description_html` is
		// also sent, which would break hash comparison on the next status.
		if ch.HasFieldChange && ch.Update.Description != nil {
			t.Description = *ch.Update.Description
		}
		return s.writeTicket(ctx, t, idToName[t.ColumnID], ch.Slug, false)
	}
	return nil
}

// postNote posts the push note (if set) as a comment on a ticket.
func (s *Syncer) postNote(ctx context.Context, ticketID, ref string) {
	if s.Note == "" {
		return
	}
	if _, err := s.API.AddComment(ctx, ticketID, s.Note); err != nil {
		s.logf("note comment failed on %s: %v", ref, err)
		return
	}
	s.logf("~ noted change on %s", ref)
}

// outgoingDescription returns the (plain, html) description pair the API
// should receive, based on the working doc's body_format:
//   - "" / "source" (default): body is Markdown source → convert to HTML
//     (description_html); description is left empty (the server auto-derives
//     a plain-text fallback from the HTML).
//   - "html": body is already HTML → send as description_html verbatim.
//   - "plain": body is plain text → send as description only.
//
// On a conversion error, the raw Markdown is returned in `plain` with `html`
// empty (the push is not blocked by a broken converter).
func outgoingDescription(doc TicketDoc) (plain, html string) {
	switch doc.BodyFormat {
	case BodyFormatHTML:
		return "", doc.Description
	case BodyFormatPlain:
		return doc.Description, ""
	default: // "" or "source"
		converted, err := RenderMarkdownToSafeHTML(doc.Description)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: md→html conversion failed (%v); sending plain text only\n", err)
			return doc.Description, ""
		}
		// Send both plain Markdown (so the baseline hash stays consistent)
		// and rendered HTML (so the web UI displays formatted content).
		return doc.Description, converted
	}
}

func (s *Syncer) describe(ch Change) {
	var parts []string
	switch ch.Kind {
	case KindCreate:
		parts = append(parts, "create")
	case KindDelete:
		parts = append(parts, "delete")
	case KindRemote:
		parts = append(parts, "remote-changed (pull)")
	case KindConflict:
		parts = append(parts, "CONFLICT")
	}
	if ch.HasFieldChange {
		parts = append(parts, "fields")
	}
	if ch.MoveToColumn != "" {
		parts = append(parts, "move→"+ch.MoveToColumn)
	}
	if len(ch.NewComments) > 0 {
		parts = append(parts, plural(len(ch.NewComments), "comment"))
	}
	if len(ch.NewAttachments) > 0 {
		parts = append(parts, plural(len(ch.NewAttachments), "file"))
	}
	s.logf("%s %s  %s", ch.Symbol(), ch.Ref, strings.Join(parts, ", "))
}

// appendJournal records a one-line audit entry per push (versioning trail).
func (s *Syncer) appendJournal(plan Plan) {
	c, u, d, cf, _ := plan.Summary()
	line := fmt.Sprintf("serial=%d create=%d update=%d delete=%d conflict=%d ts=%s\n",
		s.Tree.State.Serial, c, u, d, cf, nowStamp())
	f, err := os.OpenFile(filepath.Join(s.Tree.Root, DirName, "journal.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

func nowStamp() string { return time.Now().UTC().Format(time.RFC3339) }

// ---- file scanning ----------------------------------------------------------

func readBaseline(ticketDir string) (mello.Ticket, error) {
	var t mello.Ticket
	data, err := os.ReadFile(filepath.Join(ticketDir, "ticket.json"))
	if err != nil {
		return t, err
	}
	return t, json.Unmarshal(data, &t)
}

func readDoc(ticketDir string) (TicketDoc, error) {
	data, err := os.ReadFile(filepath.Join(ticketDir, "ticket.md"))
	if err != nil {
		return TicketDoc{}, err
	}
	return ParseTicket(data)
}

// scanNewComments returns comment files with no upstream id (locally authored).
func scanNewComments(ticketDir string) []PendingComment {
	dir := filepath.Join(ticketDir, "comments")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []PendingComment
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		id, _, body := parseCommentFile(data)
		if id == "" && strings.TrimSpace(body) != "" {
			out = append(out, PendingComment{Path: filepath.Join(dir, e.Name()), Body: body})
		}
	}
	return out
}

// scanNewAttachments returns files whose content hash differs from the recorded
// baseline (new or changed). Skips cache / temp / lock files.
func scanNewAttachments(ticketDir string, known map[string]string) []string {
	dir := filepath.Join(ticketDir, "attachments")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Skip cache / temp / lock files
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") ||
			strings.HasSuffix(name, "#") || strings.HasSuffix(name, "_") ||
			strings.HasPrefix(name, "~$") || strings.HasPrefix(name, ".~") {
			continue
		}
		path := filepath.Join(dir, name)
		h, err := HashFile(path)
		if err != nil {
			continue
		}
		if known == nil || known[name] != h {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}

func parseCommentFile(data []byte) (id, author, body string) {
	text := string(data)
	if !strings.HasPrefix(text, "---") {
		return "", "", strings.TrimRight(text, "\n")
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(text, "---"), "\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", strings.TrimRight(text, "\n")
	}
	front := rest[:end]
	b := strings.TrimPrefix(rest[end+len("\n---"):], "\n")
	b = strings.TrimPrefix(b, "\n")
	for _, line := range strings.Split(front, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "id":
			id = unquote(strings.TrimSpace(v))
		case "author":
			author = unquote(strings.TrimSpace(v))
		}
	}
	return id, author, strings.TrimRight(b, "\n")
}

func refDoc(doc TicketDoc, base mello.Ticket) string {
	if doc.Ticket != "" {
		return doc.Ticket
	}
	return refOf(base)
}

func refOf(t mello.Ticket) string {
	if t.TicketCode != "" {
		return t.TicketCode
	}
	return shortRef(t.ID)
}

func shortRef(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return itoa(n) + " " + word + "s"
}
