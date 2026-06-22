package sync

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/minhlucncc/mello-cli/internal/mello"
)

// stubAPI is an in-memory Mello backend implementing the sync.API interface.
type stubAPI struct {
	cols       []mello.Column
	tickets    map[string]*mello.Ticket
	comments   map[string][]mello.Comment
	attach     map[string][]mello.Attachment
	noComments bool
	noAttach   bool
	clock      int
	attachSeq  int

	gotUpdates       int
	gotMoves         int
	gotCreates       int
	gotDeletes       int
	gotComment       []string
	gotUploads       int
	gotAttachDeletes int
}

func tptr(s string) *time.Time {
	tt, _ := time.Parse(time.RFC3339, s)
	return &tt
}

func (s *stubAPI) tick() *time.Time {
	s.clock++
	tt := time.Date(2026, 1, 1, 0, 0, s.clock, 0, time.UTC)
	return &tt
}

func (s *stubAPI) ListColumns(ctx context.Context, boardID string) ([]mello.Column, error) {
	return s.cols, nil
}
func (s *stubAPI) ListTickets(ctx context.Context, ws, after string) ([]mello.Ticket, error) {
	var out []mello.Ticket
	for _, t := range s.tickets {
		out = append(out, *t)
	}
	return out, nil
}
func (s *stubAPI) GetTicket(ctx context.Context, id string) (mello.Ticket, error) {
	t, ok := s.tickets[id]
	if !ok {
		return mello.Ticket{}, &mello.APIError{Status: http.StatusNotFound}
	}
	return *t, nil
}
func (s *stubAPI) CreateTicket(ctx context.Context, colID, title, desc, descHTML string) (mello.Ticket, error) {
	s.gotCreates++
	id := fmt.Sprintf("new-%d", s.gotCreates)
	t := &mello.Ticket{ID: id, TicketCode: fmt.Sprintf("NEW-%d", s.gotCreates), BoardID: "b1",
		ColumnID: colID, Title: title, Description: desc, DescriptionHTML: descHTML, Status: "open", UpdatedAt: s.tick()}
	s.tickets[id] = t
	return *t, nil
}
func (s *stubAPI) DeleteTicket(ctx context.Context, id string) error {
	if _, ok := s.tickets[id]; !ok {
		return &mello.APIError{Status: http.StatusNotFound}
	}
	delete(s.tickets, id)
	s.gotDeletes++
	return nil
}
func (s *stubAPI) ListComments(ctx context.Context, id string) ([]mello.Comment, error) {
	if s.noComments {
		return nil, &mello.APIError{Status: http.StatusNotFound, Path: "/tickets/x/comments"}
	}
	return s.comments[id], nil
}
func (s *stubAPI) AddComment(ctx context.Context, id, body, bodyHTML string) (mello.Comment, error) {
	s.gotComment = append(s.gotComment, body)
	c := mello.Comment{ID: "c-new", TicketID: id, Body: body}
	s.comments[id] = append(s.comments[id], c)
	return c, nil
}
func (s *stubAPI) ListAttachments(ctx context.Context, id string) ([]mello.Attachment, error) {
	if s.noAttach {
		return nil, &mello.APIError{Status: http.StatusNotFound, Path: "/tickets/x/attachments"}
	}
	return s.attach[id], nil
}
func (s *stubAPI) UploadAttachment(ctx context.Context, id, path string) (mello.Attachment, error) {
	if s.noAttach {
		return mello.Attachment{}, &mello.APIError{Status: http.StatusNotFound, Path: "/tickets/x/attachments"}
	}
	s.attachSeq++
	a := mello.Attachment{ID: fmt.Sprintf("a%d", s.attachSeq), Filename: filepath.Base(path)}
	if s.attach == nil {
		s.attach = map[string][]mello.Attachment{}
	}
	s.attach[id] = append(s.attach[id], a)
	s.gotUploads++
	return a, nil
}
func (s *stubAPI) DeleteAttachment(ctx context.Context, ticketID, attachmentID string) error {
	list := s.attach[ticketID]
	for i, a := range list {
		if a.ID == attachmentID {
			s.attach[ticketID] = append(list[:i:i], list[i+1:]...)
			s.gotAttachDeletes++
			return nil
		}
	}
	return &mello.APIError{Status: http.StatusNotFound}
}
func (s *stubAPI) DownloadAttachment(ctx context.Context, id string, a mello.Attachment, w io.Writer) error {
	_, err := w.Write([]byte("data"))
	return err
}
func (s *stubAPI) UpdateTicket(ctx context.Context, id string, upd mello.TicketUpdate) (mello.Ticket, error) {
	t := s.tickets[id]
	if upd.Title != nil {
		t.Title = *upd.Title
	}
	if upd.Description != nil {
		t.Description = *upd.Description
	}
	if upd.DescriptionHTML != nil {
		t.DescriptionHTML = *upd.DescriptionHTML
	}
	if upd.Status != nil {
		t.Status = *upd.Status
	}
	if upd.AssigneeID != nil {
		t.AssigneeID = *upd.AssigneeID
	}
	if upd.Labels != nil {
		t.Labels = *upd.Labels
	}
	t.UpdatedAt = s.tick()
	s.gotUpdates++
	return *t, nil
}
func (s *stubAPI) MoveTicket(ctx context.Context, id, colID string, pos int) (mello.Ticket, error) {
	t := s.tickets[id]
	t.ColumnID = colID
	t.UpdatedAt = s.tick()
	s.gotMoves++
	return *t, nil
}

func newStub() *stubAPI {
	t1 := &mello.Ticket{
		ID: "t1", TicketCode: "T-1", BoardID: "b1", ColumnID: "col-todo",
		Title: "Old title", Description: "old body", Status: "open",
		UpdatedAt: tptr("2026-01-01T00:00:00Z"),
	}
	return &stubAPI{
		tickets:  map[string]*mello.Ticket{"t1": t1},
		comments: map[string][]mello.Comment{},
		cols: []mello.Column{
			{ID: "col-todo", Name: "Todo", Tickets: []mello.Ticket{*t1}},
			{ID: "col-doing", Name: "Doing"},
		},
	}
}

// cloneInto builds a one-board workspace and clones the stub into it.
func cloneInto(t *testing.T, s *stubAPI) (*Tree, *BoardState) {
	t.Helper()
	root := t.TempDir()
	tree, err := InitWorkspace(root, &State{WorkspaceID: "ws1"})
	if err != nil {
		t.Fatal(err)
	}
	bs := &BoardState{BoardID: "b1", Slug: "b1", Name: "Board One"}
	tree.AddBoard(bs)
	sy := &Syncer{API: s, Tree: tree, Board: bs}
	if _, err := sy.Clone(context.Background()); err != nil {
		t.Fatal(err)
	}
	return tree, bs
}

func plan(t *testing.T, s *stubAPI, tree *Tree, bs *BoardState, remote bool) (*Syncer, Plan) {
	t.Helper()
	sy := &Syncer{API: s, Tree: tree, Board: bs}
	p, err := sy.ComputePlan(context.Background(), remote)
	if err != nil {
		t.Fatal(err)
	}
	return sy, p
}

func TestUpdateAndMove(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)

	mdPath := filepath.Join(tree.ticketDir(bs.Slug, "t-1"), "ticket.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("ticket.md not written: %v", err)
	}
	edited := strings.ReplaceAll(string(data), "Old title", "New title")
	edited = strings.ReplaceAll(edited, "column: Todo", "column: Doing")
	os.WriteFile(mdPath, []byte(edited), 0o644)

	sy, p := plan(t, s, tree, bs, false)
	if len(p.Changes) != 1 || p.Changes[0].Kind != KindUpdate {
		t.Fatalf("plan = %+v", p.Changes)
	}
	if !p.Changes[0].HasFieldChange || p.Changes[0].MoveToColumn != "Doing" {
		t.Errorf("change = %+v", p.Changes[0])
	}
	if err := sy.Apply(context.Background(), p, false, false); err != nil {
		t.Fatal(err)
	}
	if s.gotUpdates != 1 || s.gotMoves != 1 {
		t.Errorf("updates=%d moves=%d", s.gotUpdates, s.gotMoves)
	}
	if s.tickets["t1"].Title != "New title" || s.tickets["t1"].ColumnID != "col-doing" {
		t.Errorf("server state = %+v", s.tickets["t1"])
	}
	_, p2 := plan(t, s, tree, bs, false)
	if len(p2.Changes) != 0 {
		t.Errorf("expected clean, got %+v", p2.Changes)
	}
}

func TestCreateLocalTicket(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)

	slug := tree.UniqueSlug(bs, "Ship the thing")
	dir := tree.TicketPath(bs.Slug, slug)
	os.MkdirAll(dir, 0o755)
	md := RenderTicket(mello.Ticket{Title: "Ship the thing", Description: "do it", Status: "open"}, "Doing")
	os.WriteFile(filepath.Join(dir, "ticket.md"), md, 0o644)

	sy, p := plan(t, s, tree, bs, false)
	found := false
	for _, ch := range p.Changes {
		if ch.Kind == KindCreate {
			found = true
		}
	}
	if !found {
		t.Fatalf("no create change in %+v", p.Changes)
	}
	if err := sy.Apply(context.Background(), p, false, false); err != nil {
		t.Fatal(err)
	}
	if s.gotCreates != 1 {
		t.Fatalf("CreateTicket calls = %d", s.gotCreates)
	}
	rec := bs.Tickets[slug]
	if rec == nil || rec.RemoteID == "" {
		t.Fatalf("new ticket not tracked with a remote id: %+v", rec)
	}
	_, p2 := plan(t, s, tree, bs, false)
	if len(p2.Changes) != 0 {
		t.Errorf("expected clean after create, got %+v", p2.Changes)
	}
}

func TestFolderRemovalUntracksKeepsRemote(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)
	os.RemoveAll(tree.ticketDir(bs.Slug, "t-1"))

	_, p := plan(t, s, tree, bs, false)
	if len(p.Changes) != 0 {
		t.Fatalf("removing a folder must not be a remote change, got %+v", p.Changes)
	}
	if _, ok := bs.Tickets["t-1"]; ok {
		t.Errorf("record not pruned after folder removal")
	}
	if _, ok := s.tickets["t1"]; !ok {
		t.Errorf("remote ticket must NOT be deleted")
	}
	if s.gotDeletes != 0 {
		t.Errorf("no remote delete expected, got %d", s.gotDeletes)
	}
}

func TestUntrackKeepsRemote(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)
	slug, ok := bs.FindTicketSlug("T-1") // by code
	if !ok {
		t.Fatal("ticket not found by code")
	}
	tree.Untrack(bs, slug)
	if _, ok := bs.Tickets[slug]; ok {
		t.Error("record remains after untrack")
	}
	if _, err := os.Stat(tree.ticketDir(bs.Slug, slug)); err == nil {
		t.Error("folder remains after untrack")
	}
	if _, ok := s.tickets["t1"]; !ok {
		t.Error("remote ticket must survive untrack")
	}
}

func TestPullSingleTicketIntoWorkingSet(t *testing.T) {
	s := newStub()
	root := t.TempDir()
	tree, _ := InitWorkspace(root, &State{WorkspaceID: "ws1"})
	bs := &BoardState{BoardID: "b1", Slug: "b1", Name: "Board One"}
	tree.AddBoard(bs)
	sy := &Syncer{API: s, Tree: tree, Board: bs}

	if len(bs.Tickets) != 0 {
		t.Fatalf("working set should start empty, got %d", len(bs.Tickets))
	}
	tk, _, err := sy.PullTicket(context.Background(), "T-1")
	if err != nil {
		t.Fatal(err)
	}
	if tk.ID != "t1" {
		t.Fatalf("pulled %s", tk.ID)
	}
	if _, ok := bs.Tickets["t-1"]; !ok {
		t.Fatalf("ticket not added to working set: %+v", bs.Tickets)
	}
}

func TestConflictDetectionAndForce(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)

	mdPath := filepath.Join(tree.ticketDir(bs.Slug, "t-1"), "ticket.md")
	data, _ := os.ReadFile(mdPath)
	os.WriteFile(mdPath, []byte(strings.ReplaceAll(string(data), "Old title", "Local title")), 0o644)
	s.tickets["t1"].Title = "Remote title"
	s.tickets["t1"].UpdatedAt = tptr("2026-02-02T00:00:00Z")

	sy, p := plan(t, s, tree, bs, true)
	if len(p.Changes) != 1 || p.Changes[0].Kind != KindConflict {
		t.Fatalf("expected conflict, got %+v", p.Changes)
	}
	if err := sy.Apply(context.Background(), p, false, false); err == nil {
		t.Fatal("conflict push without --force must be rejected")
	}
	if s.gotUpdates != 0 {
		t.Errorf("conflict applied without force: updates=%d", s.gotUpdates)
	}
	sy2, p2 := plan(t, s, tree, bs, true)
	if err := sy2.Apply(context.Background(), p2, false, true); err != nil {
		t.Fatal(err)
	}
	if s.tickets["t1"].Title != "Local title" {
		t.Errorf("force push did not apply local: %q", s.tickets["t1"].Title)
	}
}

func TestPushNewComment(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)

	commentsDir := filepath.Join(tree.ticketDir(bs.Slug, "t-1"), "comments")
	os.MkdirAll(commentsDir, 0o755)
	body := "---\nauthor: me\n---\n\nLooks good, shipping.\n"
	os.WriteFile(filepath.Join(commentsDir, "draft.md"), []byte(body), 0o644)

	sy, p := plan(t, s, tree, bs, false)
	if len(p.Changes) != 1 || len(p.Changes[0].NewComments) != 1 {
		t.Fatalf("expected 1 new comment, got %+v", p.Changes)
	}
	if err := sy.Apply(context.Background(), p, false, false); err != nil {
		t.Fatal(err)
	}
	if len(s.gotComment) != 1 || s.gotComment[0] != "Looks good, shipping." {
		t.Errorf("AddComment payload = %v", s.gotComment)
	}
	_, p2 := plan(t, s, tree, bs, false)
	if len(p2.Changes) != 0 {
		t.Errorf("expected clean after comment push, got %+v", p2.Changes)
	}
}

func TestPushAttachmentReplacesSameName(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)

	attDir := filepath.Join(tree.ticketDir(bs.Slug, "t-1"), "attachments")
	os.MkdirAll(attDir, 0o755)
	attPath := filepath.Join(attDir, "spec.md")
	os.WriteFile(attPath, []byte("version one"), 0o644)

	// First push uploads the new file.
	sy, p := plan(t, s, tree, bs, false)
	if err := sy.Apply(context.Background(), p, false, false); err != nil {
		t.Fatal(err)
	}
	if s.gotUploads != 1 || len(s.attach["t1"]) != 1 {
		t.Fatalf("after first push: uploads=%d remote=%d", s.gotUploads, len(s.attach["t1"]))
	}

	// Edit the same file and push again: it must REPLACE, not duplicate.
	os.WriteFile(attPath, []byte("version two — much longer content"), 0o644)
	sy2, p2 := plan(t, s, tree, bs, false)
	if len(p2.Changes) != 1 || len(p2.Changes[0].NewAttachments) != 1 {
		t.Fatalf("expected the edited attachment to be pending, got %+v", p2.Changes)
	}
	if err := sy2.Apply(context.Background(), p2, false, false); err != nil {
		t.Fatal(err)
	}
	if s.gotUploads != 2 || s.gotAttachDeletes != 1 {
		t.Errorf("expected 1 replace (uploads=2, deletes=1), got uploads=%d deletes=%d", s.gotUploads, s.gotAttachDeletes)
	}
	if len(s.attach["t1"]) != 1 {
		t.Errorf("same-named attachment duplicated on server: %d copies", len(s.attach["t1"]))
	}
}

func TestPushRejectsConflictRequiresPull(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)

	mdPath := filepath.Join(tree.ticketDir(bs.Slug, "t-1"), "ticket.md")
	data, _ := os.ReadFile(mdPath)
	os.WriteFile(mdPath, []byte(strings.ReplaceAll(string(data), "Old title", "Local title")), 0o644)
	s.tickets["t1"].Title = "Remote title"
	s.tickets["t1"].UpdatedAt = tptr("2026-03-03T00:00:00Z")

	sy, p := plan(t, s, tree, bs, true)
	if err := sy.Apply(context.Background(), p, false, false); err == nil {
		t.Fatal("push must be rejected when the remote drifted")
	}
	if s.gotUpdates != 0 || s.gotMoves != 0 {
		t.Errorf("nothing should be pushed on rejection: updates=%d moves=%d", s.gotUpdates, s.gotMoves)
	}
	// --force overrides.
	sy2, p2 := plan(t, s, tree, bs, true)
	if err := sy2.Apply(context.Background(), p2, false, true); err != nil {
		t.Fatalf("force push should succeed: %v", err)
	}
	if s.tickets["t1"].Title != "Local title" {
		t.Errorf("force push did not apply local: %q", s.tickets["t1"].Title)
	}
}

func TestPushRejectsRemoteOnlyChange(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)
	s.tickets["t1"].Title = "Remote title"
	s.tickets["t1"].UpdatedAt = tptr("2026-03-03T00:00:00Z")

	sy, p := plan(t, s, tree, bs, true)
	if len(p.Changes) != 1 || p.Changes[0].Kind != KindRemote {
		t.Fatalf("expected a remote-only change, got %+v", p.Changes)
	}
	if err := sy.Apply(context.Background(), p, false, false); err == nil {
		t.Fatal("push must require a pull when the remote changed")
	}
}

func TestStashResetsThenPopRestores(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)

	mdPath := filepath.Join(tree.ticketDir(bs.Slug, "t-1"), "ticket.md")
	orig, _ := os.ReadFile(mdPath)
	os.WriteFile(mdPath, []byte(strings.ReplaceAll(string(orig), "old body", "my local work")), 0o644)

	sy := &Syncer{API: s, Tree: tree, Board: bs}
	entry, err := sy.Stash("wip")
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil || len(entry.Tickets) != 1 {
		t.Fatalf("stash entry = %+v", entry)
	}

	// Working copy is reset → a status check is clean.
	_, p := plan(t, s, tree, bs, false)
	if len(p.Changes) != 0 {
		t.Fatalf("expected clean working copy after stash, got %+v", p.Changes)
	}
	if cur, _ := os.ReadFile(mdPath); strings.Contains(string(cur), "my local work") {
		t.Error("ticket.md was not reset to baseline by stash")
	}

	// Pop restores the edit and drops the stash.
	if err := sy.StashApply(entry, true); err != nil {
		t.Fatal(err)
	}
	if cur, _ := os.ReadFile(mdPath); !strings.Contains(string(cur), "my local work") {
		t.Error("stash pop did not restore the local edit")
	}
	_, p2 := plan(t, s, tree, bs, false)
	if len(p2.Changes) != 1 || p2.Changes[0].Kind != KindUpdate {
		t.Fatalf("expected the edit back as an update, got %+v", p2.Changes)
	}
	if list, _ := tree.ListStash(); len(list) != 0 {
		t.Errorf("pop should drop the stash, %d remain", len(list))
	}
}

func TestStashApplyKeepsEntry(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)
	mdPath := filepath.Join(tree.ticketDir(bs.Slug, "t-1"), "ticket.md")
	orig, _ := os.ReadFile(mdPath)
	os.WriteFile(mdPath, []byte(strings.ReplaceAll(string(orig), "Old title", "Edited")), 0o644)

	sy := &Syncer{API: s, Tree: tree, Board: bs}
	entry, err := sy.Stash("")
	if err != nil || entry == nil {
		t.Fatalf("stash failed: %v / %+v", err, entry)
	}
	if err := sy.StashApply(entry, false); err != nil {
		t.Fatal(err)
	}
	if list, _ := tree.ListStash(); len(list) != 1 {
		t.Errorf("apply (no pop) should keep the stash, got %d", len(list))
	}
}

func TestDryRunMutatesNothing(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)
	mdPath := filepath.Join(tree.ticketDir(bs.Slug, "t-1"), "ticket.md")
	data, _ := os.ReadFile(mdPath)
	os.WriteFile(mdPath, []byte(strings.ReplaceAll(string(data), "Old title", "X")), 0o644)

	sy, p := plan(t, s, tree, bs, false)
	if err := sy.Apply(context.Background(), p, true, false); err != nil {
		t.Fatal(err)
	}
	if s.gotUpdates != 0 || s.gotMoves != 0 {
		t.Errorf("dry-run mutated: updates=%d moves=%d", s.gotUpdates, s.gotMoves)
	}
}

func TestMultipleBoardsIsolated(t *testing.T) {
	s := newStub()
	tree, _ := cloneInto(t, s)
	// Attach a second board with its own ticket namespace.
	o1 := &mello.Ticket{ID: "u1", TicketCode: "OPS-1", BoardID: "b2", ColumnID: "col-todo",
		Title: "Ops task", Status: "open", UpdatedAt: tptr("2026-01-03T00:00:00Z")}
	api2 := &stubAPI{
		tickets:  map[string]*mello.Ticket{"u1": o1},
		comments: map[string][]mello.Comment{},
		cols:     []mello.Column{{ID: "col-todo", Name: "Todo", Tickets: []mello.Ticket{*o1}}},
	}
	bs2 := &BoardState{BoardID: "b2", Slug: "ops", Name: "Ops"}
	tree.AddBoard(bs2)
	if _, err := (&Syncer{API: api2, Tree: tree, Board: bs2}).Clone(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(tree.State.Boards) != 2 {
		t.Fatalf("want 2 boards, got %d", len(tree.State.Boards))
	}
	if _, ok := tree.State.Boards["b1"].Tickets["t-1"]; !ok {
		t.Errorf("board b1 missing its ticket")
	}
	if _, ok := tree.State.Boards["ops"].Tickets["ops-1"]; !ok {
		t.Errorf("board ops missing its ticket: %+v", tree.State.Boards["ops"].Tickets)
	}
	if _, err := os.Stat(tree.ticketDir("ops", "ops-1")); err != nil {
		t.Errorf("ops ticket dir not created: %v", err)
	}
}

func TestCloneDegradesWhenOptionalEndpointsMissing(t *testing.T) {
	s := newStub()
	s.noComments = true
	s.noAttach = true
	root := t.TempDir()
	tree, _ := InitWorkspace(root, &State{WorkspaceID: "ws1"})
	bs := &BoardState{BoardID: "b1", Slug: "b1", Name: "Board One"}
	tree.AddBoard(bs)
	sy := &Syncer{API: s, Tree: tree, Board: bs}
	n, err := sy.Clone(context.Background())
	if err != nil {
		t.Fatalf("clone should not fail on missing optional endpoints: %v", err)
	}
	if n != 1 {
		t.Fatalf("cloned %d tickets, want 1", n)
	}
	if !sy.noComments || !sy.noAttachments {
		t.Errorf("capability flags not set: comments=%v attach=%v", sy.noComments, sy.noAttachments)
	}
}

// TestPushConvertsMarkdownToHTML confirms that on push, a ticket.md whose
// body contains Markdown is sent to the server as sanitized HTML. The test
// edits the existing T-1 ticket's local file with a rich body, then plans
// and applies, and asserts the stub's recorded UpdateTicket call received
// HTML.
// TestPushConvertsMarkdownToHTMLOnDescriptionHTML confirms that on push, a
// ticket.md whose body contains Markdown is converted to sanitized HTML and
// sent to the server as description_html (not description). The web UI
// renders description_html.
func TestPushConvertsMarkdownToHTMLOnDescriptionHTML(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)
	dir := tree.ticketDir(bs.Slug, "t-1")

	body := "# Heading\n\n`inline` and a [link](https://example.com).\n"
	md := RenderTicket(mello.Ticket{
		ID: "t1", TicketCode: "T-1", Title: "Old title", Description: body, Status: "open",
	}, "Todo")
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), md, 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-baseline so the next plan/diff detects only the description change.
	sy := &Syncer{API: s, Tree: tree, Board: bs}
	if _, _, err := sy.RefreshWorkingSet(context.Background()); err != nil {
		t.Fatal(err)
	}
	sy2, p := plan(t, s, tree, bs, false)
	if len(p.Changes) != 1 || p.Changes[0].Kind != KindUpdate {
		t.Fatalf("expected 1 update change, got %+v", p.Changes)
	}
	if err := sy2.Apply(context.Background(), p, false, false); err != nil {
		t.Fatal(err)
	}
	if s.gotUpdates != 1 {
		t.Errorf("UpdateTicket calls = %d, want 1", s.gotUpdates)
	}
	// The stub received the converted HTML on description_html, and the plain
	// description is empty (the server would auto-derive it).
	got := s.tickets["t1"].DescriptionHTML
	for _, want := range []string{"<h1", "Heading", "<code>inline</code>", `<a href="https://example.com"`, "link"} {
		if !strings.Contains(got, want) {
			t.Errorf("server description_html missing %q\n--- got ---\n%s", want, got)
		}
	}
	// The local file is re-rendered with body_format: html and the stored
	// body is the HTML (not the original Markdown).
	newMD, err := os.ReadFile(filepath.Join(dir, "ticket.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newMD), "body_format: html") {
		t.Errorf("expected body_format: html in re-rendered ticket.md\n--- got ---\n%s", newMD)
	}
	doc, err := ParseTicket(newMD)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Description, "<h1") {
		t.Errorf("local body is not the HTML we sent; got: %q", doc.Description)
	}
}

// TestPushBodyFormatPlainSendsToDescription confirms that a doc with
// body_format: plain sends the body verbatim to description (and does not
// set description_html).
func TestPushBodyFormatPlainSendsToDescription(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)
	dir := tree.ticketDir(bs.Slug, "t-1")

	body := "Just a plain description, no Markdown.\n"
	md := RenderTicket(mello.Ticket{
		ID: "t1", TicketCode: "T-1", Title: "Old title", Description: body, Status: "open",
	}, "Todo")
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), md, 0o644); err != nil {
		t.Fatal(err)
	}
	sy := &Syncer{API: s, Tree: tree, Board: bs}
	if _, _, err := sy.RefreshWorkingSet(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Flip body_format to plain in the local file.
	withFM := strings.Replace(string(md), "---\n", "---\nbody_format: plain\n", 1)
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), []byte(withFM), 0o644); err != nil {
		t.Fatal(err)
	}
	sy2, p := plan(t, s, tree, bs, false)
	if err := sy2.Apply(context.Background(), p, false, false); err != nil {
		t.Fatal(err)
	}
	got := s.tickets["t1"].Description
	html := s.tickets["t1"].DescriptionHTML
	if strings.Contains(got, "<h1>") || strings.Contains(got, "<strong>") || strings.Contains(got, "<p>") {
		t.Errorf("body_format=plain should not produce HTML in description; got: %q", got)
	}
	if html != "" {
		t.Errorf("body_format=plain should not send description_html; got: %q", html)
	}
	if !strings.Contains(got, "Just a plain description") {
		t.Errorf("body_format=plain should send the raw body; got: %q", got)
	}
}

// TestPushSanitizesScriptTag confirms a Markdown body that tries to inject a
// <script> tag is stripped by the sanitizer before the server sees it.
func TestPushSanitizesScriptTag(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)
	dir := tree.ticketDir(bs.Slug, "t-1")

	body := "# Heading\n\nplease do **not** run `<script>alert(1)</script>` ever.\n"
	md := RenderTicket(mello.Ticket{
		ID: "t1", TicketCode: "T-1", Title: "Old title", Description: body, Status: "open",
	}, "Todo")
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), md, 0o644); err != nil {
		t.Fatal(err)
	}
	sy := &Syncer{API: s, Tree: tree, Board: bs}
	if _, _, err := sy.RefreshWorkingSet(context.Background()); err != nil {
		t.Fatal(err)
	}
	sy2, p := plan(t, s, tree, bs, false)
	if err := sy2.Apply(context.Background(), p, false, false); err != nil {
		t.Fatal(err)
	}
	got := strings.ToLower(s.tickets["t1"].Description)
	if strings.Contains(got, "<script") {
		t.Errorf("script tag not sanitized: %q", s.tickets["t1"].Description)
	}
}
