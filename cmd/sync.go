package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/mello"
	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

// relPath shows p relative to the current directory when that's shorter/cleaner.
func relPath(p string) string {
	if cwd, err := os.Getwd(); err == nil {
		if r, rerr := filepath.Rel(cwd, p); rerr == nil && !strings.HasPrefix(r, "..") {
			return r
		}
	}
	return p
}

func syncCmd() *Command {
	return &Command{
		Name:  "sync",
		Short: "Synchronize the local workspace with Mello.",
		Subs: []*Command{
			{Name: "clone", Short: "Check a board out into the workspace.", Run: syncClone},
			{Name: "status", Short: "Show the plan of pending local changes.", Run: syncStatus},
			{Name: "pull", Short: "Fetch remote changes into the workspace.", Run: syncPull},
			{Name: "push", Short: "Apply local changes to the server.", Run: syncPush},
			{Name: "sync", Short: "Reconcile: pull then push.", Run: syncReconcile},
		},
	}
}

func syncClone(args []string) error {
	fs, c := newFlags("clone")
	board := fs.String("board", "", "board id, code, or slug")
	fs.StringVar(board, "b", "", "board (shorthand)")
	wsFlag := fs.String("workspace", "", "limit the search to a workspace id")
	fs.StringVar(wsFlag, "w", "", "workspace id (shorthand)")
	dir := fs.String("dir", ".", "workspace directory")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	sel := *board
	if sel == "" && fs.NArg() > 0 {
		sel = fs.Arg(0)
	}
	if sel == "" {
		return fmt.Errorf("usage: mello clone <board> [--dir D]")
	}
	cl, r, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()

	// Open an existing workspace or initialize a new one in --dir.
	var tree *syncpkg.Tree
	if syncpkg.Exists(*dir) {
		tree, err = syncpkg.Open(*dir)
		if err != nil {
			return err
		}
	} else {
		tree, err = syncpkg.InitWorkspace(*dir, &syncpkg.State{
			Profile: r.Profile, BaseURL: r.BaseURL,
			WorkspaceID: firstNonEmpty(*wsFlag, r.WorkspaceID),
		})
		if err != nil {
			return err
		}
	}

	bs, n, err := attachAndClone(cx, cl, tree, sel)
	if err != nil {
		return err
	}
	ui.Successf("Checked out board %s — %d ticket(s)", ui.Bold(bs.Name), n)
	return nil
}

// attachBoard registers a board in the workspace WITHOUT fetching its tickets.
// When the workspace is not yet known, the board is located across all of the
// token's workspaces and its workspace is bound to the working copy. This is the
// lightweight default: tickets are pulled lazily (one at a time, or with
// `pull --all`) or created locally.
func attachBoard(cx context.Context, cl *mello.Client, tree *syncpkg.Tree, sel string) (*syncpkg.BoardState, error) {
	sel = normalizeSelector(sel)
	if tree.State.WorkspaceID == "" {
		wsID, wsName, b, err := findBoardAnywhere(cx, cl, sel)
		if err != nil {
			return nil, err
		}
		tree.State.WorkspaceID = wsID
		tree.State.WorkspaceName = wsName
		bs := registerBoard(tree, b)
		return bs, tree.Save()
	}
	boards, err := cl.ListBoards(cx, tree.State.WorkspaceID)
	if err != nil {
		return nil, err
	}
	for _, b := range boards {
		if matchBoard(b, sel) {
			bs := registerBoard(tree, b)
			return bs, tree.Save()
		}
	}
	return nil, fmt.Errorf("no board %q in this workspace (run `mello board list`)", sel)
}

func registerBoard(tree *syncpkg.Tree, b mello.Board) *syncpkg.BoardState {
	slug := syncpkg.Slugify(firstNonEmpty(b.Code, b.Name))
	bs := tree.State.Boards[slug]
	if bs == nil {
		bs = &syncpkg.BoardState{BoardID: b.ID, Slug: slug, Name: b.Name, Code: b.Code}
		tree.AddBoard(bs)
	}
	return bs
}

// attachAndClone registers a board and mirrors all of its tickets locally.
func attachAndClone(cx context.Context, cl *mello.Client, tree *syncpkg.Tree, sel string) (*syncpkg.BoardState, int, error) {
	bs, err := attachBoard(cx, cl, tree, sel)
	if err != nil {
		return nil, 0, err
	}
	s := &syncpkg.Syncer{API: cl, Tree: tree, Board: bs, Log: logfUI}
	n, err := s.Clone(cx)
	return bs, n, err
}

// findBoardAnywhere searches every workspace the token can see for a board
// matching sel, returning the owning workspace and the board.
func findBoardAnywhere(cx context.Context, cl *mello.Client, sel string) (string, string, mello.Board, error) {
	sel = normalizeSelector(sel)
	workspaces, err := cl.ListWorkspaces(cx)
	if err != nil {
		return "", "", mello.Board{}, err
	}
	for _, w := range workspaces {
		boards, berr := cl.ListBoards(cx, w.ID)
		if berr != nil {
			continue
		}
		for _, b := range boards {
			if matchBoard(b, sel) {
				return w.ID, w.Name, b, nil
			}
		}
	}
	return "", "", mello.Board{}, fmt.Errorf("no board %q found in your workspaces (run `mello board list`)", sel)
}

func matchBoard(b mello.Board, sel string) bool {
	if b.ID == sel || b.Code == sel || b.Name == sel {
		return true
	}
	return strings.EqualFold(b.Code, sel) ||
		syncpkg.Slugify(firstNonEmpty(b.Code, b.Name)) == syncpkg.Slugify(sel)
}

// forEachBoard runs fn for each selected board: the -b board, else the working
// board, or every board when all is set. A header is printed when more than one
// board is involved.
func forEachBoard(c *common, dir, boardSel string, all bool, fn func(*syncpkg.Syncer) error) error {
	tree, err := syncpkg.Open(dir)
	if err != nil {
		return err
	}
	var boards []*syncpkg.BoardState
	if boardSel == "" && all {
		boards, err = tree.SelectBoards("")
	} else {
		var bs *syncpkg.BoardState
		bs, err = tree.ResolveBoard(boardSel)
		if err == nil {
			boards = []*syncpkg.BoardState{bs}
		}
	}
	if err != nil {
		return err
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	multi := len(boards) > 1
	for _, bs := range boards {
		if multi {
			fmt.Printf("%s\n", ui.Bold(bs.Name))
		}
		s := &syncpkg.Syncer{API: cl, Tree: tree, Board: bs, Log: logfUI}
		if err := fn(s); err != nil {
			return fmt.Errorf("board %s: %w", bs.Name, err)
		}
	}
	return nil
}

func syncStatus(args []string) error {
	fs, c := newFlags("sync status")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	all := fs.Bool("all", false, "all boards in the workspace")
	remote := fs.Bool("remote", false, "also fetch remote to detect drift/conflicts")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	return forEachBoard(c, *dir, *board, *all, func(s *syncpkg.Syncer) error {
		plan, err := s.ComputePlan(cx, *remote)
		if err != nil {
			return err
		}
		if c.json {
			return ui.JSON(plan)
		}
		printStatus(s, plan)
		return nil
	})
}

func syncPull(args []string) error {
	fs, c := newFlags("pull")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	all := fs.Bool("all", false, "mirror every ticket on the board")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	ticketSel := ""
	if fs.NArg() > 0 {
		ticketSel = fs.Arg(0)
	}
	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	bs, err := tree.ResolveBoard(*board)
	if err != nil {
		return err
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	s := &syncpkg.Syncer{API: cl, Tree: tree, Board: bs, Log: logfUI}

	switch {
	case *all:
		n, err := s.Clone(cx)
		if err != nil {
			return err
		}
		ui.Successf("Mirrored board %s — %d ticket(s)", ui.Bold(bs.Name), n)
	case ticketSel != "":
		t, dir, err := s.PullTicket(cx, ticketSel)
		if err != nil {
			return err
		}
		ui.Successf("Pulled %s → %s", ui.Bold(ticketRef(t)), relPath(dir))
		fmt.Println(ui.Dim("edit " + relPath(filepath.Join(dir, "ticket.md")) + " then `mello push`"))
	default:
		updated, deleted, err := s.RefreshWorkingSet(cx)
		if err != nil {
			return err
		}
		if updated == 0 && deleted == 0 {
			fmt.Println(ui.Dim("working set is empty — `mello pull <ticket>` or `mello pull --all`"))
			return nil
		}
		ui.Successf("Refreshed working set: %d updated, %d removed", updated, deleted)
	}
	return nil
}

func syncPush(args []string) error {
	fs, c := newFlags("sync push")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	all := fs.Bool("all", false, "all boards in the workspace")
	dryRun := fs.Bool("dry-run", false, "show what would be pushed, change nothing")
	force := fs.Bool("force", false, "push conflicts (local over remote)")
	note := fs.String("comment", "", "post a comment on each changed ticket noting the push")
	fs.StringVar(note, "m", "", "comment/message (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	return forEachBoard(c, *dir, *board, *all, func(s *syncpkg.Syncer) error {
		s.Note = *note
		plan, err := s.ComputePlan(cx, true)
		if err != nil {
			return err
		}
		if len(plan.Changes) == 0 {
			fmt.Println(ui.Dim("clean — nothing to push"))
			return nil
		}
		if err := s.Apply(cx, plan, *dryRun, *force); err != nil {
			return err
		}
		if *dryRun {
			ui.Warnf("dry-run — nothing was sent")
		} else {
			ui.Successf("Push complete (serial %d)", s.Tree.State.Serial)
		}
		return nil
	})
}

func syncReconcile(args []string) error {
	fs, c := newFlags("sync sync")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	all := fs.Bool("all", false, "all boards in the workspace")
	force := fs.Bool("force", false, "push conflicts (local over remote)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	return forEachBoard(c, *dir, *board, *all, func(s *syncpkg.Syncer) error {
		if _, _, err := s.RefreshWorkingSet(cx); err != nil {
			return err
		}
		if len(s.Conflicts) > 0 && !*force {
			ui.Warnf("%d conflict(s) — resolve then push (or rerun with --force)", len(s.Conflicts))
		}
		plan, err := s.ComputePlan(cx, true)
		if err != nil {
			return err
		}
		if len(plan.Changes) == 0 {
			fmt.Println(ui.Dim("clean"))
			return nil
		}
		if err := s.Apply(cx, plan, false, *force); err != nil {
			return err
		}
		ui.Successf("Reconciled (serial %d)", s.Tree.State.Serial)
		return nil
	})
}

// printStatus lists the working set (tracked tickets) with each one's state,
// followed by a summary of pending changes. Clean tickets are shown too, so the
// command answers "what do I have checked out locally?".
func printStatus(s *syncpkg.Syncer, plan syncpkg.Plan) {
	byChangeSlug := make(map[string]syncpkg.Change, len(plan.Changes))
	for _, ch := range plan.Changes {
		byChangeSlug[ch.Slug] = ch
	}

	// Collect tracked slugs from state + any new-local slugs from the plan.
	slugSet := map[string]bool{}
	for slug := range s.Board.Tickets {
		slugSet[slug] = true
	}
	for _, ch := range plan.Changes {
		slugSet[ch.Slug] = true
	}
	slugs := make([]string, 0, len(slugSet))
	for slug := range slugSet {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	fmt.Printf("%s · %s\n", ui.Bold(s.Board.Name), ui.Dim(fmt.Sprintf("working set: %d ticket(s)", len(slugs))))
	if len(slugs) == 0 {
		fmt.Println(ui.Dim("  empty — `mello pull <ticket>` to check one out, or `mello new ticket`"))
		return
	}
	for _, slug := range slugs {
		ref := slug
		if rec := s.Board.Tickets[slug]; rec != nil && rec.Code != "" {
			ref = rec.Code
		}
		if ch, ok := byChangeSlug[slug]; ok {
			if ch.Ref != "" {
				ref = ch.Ref
			}
			fmt.Printf("  %s %s  %s\n", changeSymbol(ch), ui.Bold(ref), ui.Dim(changeTags(ch)))
		} else {
			fmt.Printf("  %s %s  %s\n", ui.Dim("·"), ui.Bold(ref), ui.Dim("clean"))
		}
	}

	cr, up, de, cf, rm := plan.Summary()
	fmt.Println()
	if cr+up+de+cf+rm == 0 {
		fmt.Println(ui.Dim("clean — nothing to push"))
		return
	}
	fmt.Printf("Pending: %d to create, %d to update, %d to delete", cr, up, de)
	if cf > 0 {
		fmt.Printf(", %s", ui.Red(fmt.Sprintf("%d conflict", cf)))
	}
	if rm > 0 {
		fmt.Printf(", %s", ui.Yellow(fmt.Sprintf("%d remote-changed", rm)))
	}
	fmt.Printf("  (serial %d) — `mello push`\n", s.Tree.State.Serial)
}

// changeSymbol returns the colored status marker for a change.
func changeSymbol(ch syncpkg.Change) string {
	sym := ch.Symbol()
	switch ch.Kind {
	case syncpkg.KindCreate:
		return ui.Green(sym)
	case syncpkg.KindDelete, syncpkg.KindConflict:
		return ui.Red(sym)
	default:
		return ui.Yellow(sym)
	}
}

func changeTags(ch syncpkg.Change) string {
	tags := []string{}
	switch ch.Kind {
	case syncpkg.KindCreate:
		tags = append(tags, "new local ticket")
	case syncpkg.KindDelete:
		tags = append(tags, "removed locally")
	case syncpkg.KindRemote:
		tags = append(tags, "remote changed — pull")
	case syncpkg.KindConflict:
		tags = append(tags, "both sides changed")
	}
	if ch.HasFieldChange {
		tags = append(tags, "fields")
	}
	if ch.MoveToColumn != "" {
		tags = append(tags, "move→"+ch.MoveToColumn)
	}
	if n := len(ch.NewComments); n > 0 {
		tags = append(tags, fmt.Sprintf("%d comment(s)", n))
	}
	if n := len(ch.NewAttachments); n > 0 {
		tags = append(tags, fmt.Sprintf("%d file(s)", n))
	}
	out := ""
	for i, t := range tags {
		if i > 0 {
			out += ", "
		}
		out += t
	}
	return out
}

func logfUI(format string, a ...any) {
	fmt.Printf("  %s\n", fmt.Sprintf(format, a...))
}
