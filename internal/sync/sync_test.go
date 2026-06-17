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
	noComments bool
	noAttach   bool
	clock      int

	gotUpdates int
	gotMoves   int
	gotCreates int
	gotDeletes int
	gotComment []string
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
func (s *stubAPI) CreateTicket(ctx context.Context, colID, title, desc string) (mello.Ticket, error) {
	s.gotCreates++
	id := fmt.Sprintf("new-%d", s.gotCreates)
	t := &mello.Ticket{ID: id, TicketCode: fmt.Sprintf("NEW-%d", s.gotCreates), BoardID: "b1",
		ColumnID: colID, Title: title, Description: desc, Status: "open", UpdatedAt: s.tick()}
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
func (s *stubAPI) AddComment(ctx context.Context, id, body string) (mello.Comment, error) {
	s.gotComment = append(s.gotComment, body)
	c := mello.Comment{ID: "c-new", TicketID: id, Body: body}
	s.comments[id] = append(s.comments[id], c)
	return c, nil
}
func (s *stubAPI) ListAttachments(ctx context.Context, id string) ([]mello.Attachment, error) {
	if s.noAttach {
		return nil, &mello.APIError{Status: http.StatusNotFound, Path: "/tickets/x/attachments"}
	}
	return nil, nil
}
func (s *stubAPI) UploadAttachment(ctx context.Context, id, path string) (mello.Attachment, error) {
	return mello.Attachment{ID: "a1", Filename: filepath.Base(path)}, nil
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

func TestDeleteLocalTicket(t *testing.T) {
	s := newStub()
	tree, bs := cloneInto(t, s)
	os.RemoveAll(tree.ticketDir(bs.Slug, "t-1"))

	sy, p := plan(t, s, tree, bs, false)
	if len(p.Changes) != 1 || p.Changes[0].Kind != KindDelete {
		t.Fatalf("plan = %+v", p.Changes)
	}
	if err := sy.Apply(context.Background(), p, false, false); err != nil {
		t.Fatal(err)
	}
	if s.gotDeletes != 1 {
		t.Errorf("DeleteTicket calls = %d", s.gotDeletes)
	}
	if _, ok := s.tickets["t1"]; ok {
		t.Errorf("ticket not deleted on server")
	}
	if _, ok := bs.Tickets["t-1"]; ok {
		t.Errorf("record not removed")
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
	if err := sy.Apply(context.Background(), p, false, false); err != nil {
		t.Fatal(err)
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
