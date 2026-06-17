package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/mello"
	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

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

// attachAndClone resolves a board selector and checks it out. When the workspace
// is not yet known, the board is located across all of the token's workspaces and
// its workspace is bound to the working copy.
func attachAndClone(cx context.Context, cl *mello.Client, tree *syncpkg.Tree, sel string) (*syncpkg.BoardState, int, error) {
	if tree.State.WorkspaceID == "" {
		wsID, wsName, b, err := findBoardAnywhere(cx, cl, sel)
		if err != nil {
			return nil, 0, err
		}
		tree.State.WorkspaceID = wsID
		tree.State.WorkspaceName = wsName
		return checkout(cx, cl, tree, b)
	}
	boards, err := cl.ListBoards(cx, tree.State.WorkspaceID)
	if err != nil {
		return nil, 0, err
	}
	for _, b := range boards {
		if matchBoard(b, sel) {
			return checkout(cx, cl, tree, b)
		}
	}
	return nil, 0, fmt.Errorf("no board %q in this workspace (run `mello board list`)", sel)
}

func checkout(cx context.Context, cl *mello.Client, tree *syncpkg.Tree, b mello.Board) (*syncpkg.BoardState, int, error) {
	slug := syncpkg.Slugify(firstNonEmpty(b.Code, b.Name))
	bs := tree.State.Boards[slug]
	if bs == nil {
		bs = &syncpkg.BoardState{BoardID: b.ID, Slug: slug, Name: b.Name, Code: b.Code}
		tree.AddBoard(bs)
	}
	s := &syncpkg.Syncer{API: cl, Tree: tree, Board: bs, Log: logfUI}
	n, err := s.Clone(cx)
	return bs, n, err
}

// findBoardAnywhere searches every workspace the token can see for a board
// matching sel, returning the owning workspace and the board.
func findBoardAnywhere(cx context.Context, cl *mello.Client, sel string) (string, string, mello.Board, error) {
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
		printPlan(plan, s.Tree.State.Serial)
		return nil
	})
}

func syncPull(args []string) error {
	fs, c := newFlags("pull")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "board to pull (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	all := fs.Bool("all", false, "all boards in the workspace")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	sel := *board
	if sel == "" && fs.NArg() > 0 {
		sel = fs.Arg(0)
	}
	cx, cancel := ctx()
	defer cancel()

	// Clone-on-demand: `mello pull <board>` checks the board out if it is not
	// already part of the workspace.
	if sel != "" {
		if tree, err := syncpkg.Open(*dir); err == nil {
			if _, rerr := tree.ResolveBoard(sel); rerr != nil {
				cl, _, cerr := c.client()
				if cerr != nil {
					return cerr
				}
				bs, n, e := attachAndClone(cx, cl, tree, sel)
				if e != nil {
					return e
				}
				ui.Successf("Checked out board %s — %d ticket(s)", ui.Bold(bs.Name), n)
				return nil
			}
		}
	}

	return forEachBoard(c, *dir, sel, *all, func(s *syncpkg.Syncer) error {
		updated, deleted, err := s.Pull(cx)
		if err != nil {
			return err
		}
		if len(s.Conflicts) > 0 {
			ui.Warnf("%d conflict(s) — resolve, then `mello push`", len(s.Conflicts))
		}
		ui.Successf("Pulled: %d updated, %d removed (serial %d)", updated, deleted, s.Tree.State.Serial)
		return nil
	})
}

func syncPush(args []string) error {
	fs, c := newFlags("sync push")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	all := fs.Bool("all", false, "all boards in the workspace")
	dryRun := fs.Bool("dry-run", false, "show what would be pushed, change nothing")
	force := fs.Bool("force", false, "push conflicts (local over remote)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	return forEachBoard(c, *dir, *board, *all, func(s *syncpkg.Syncer) error {
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
		if _, _, err := s.Pull(cx); err != nil {
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

// printPlan renders the set of pending changes with status markers.
func printPlan(plan syncpkg.Plan, serial int) {
	if len(plan.Changes) == 0 {
		fmt.Println(ui.Dim("clean — working copy matches baseline"))
		return
	}
	for _, ch := range plan.Changes {
		sym := ch.Symbol()
		switch ch.Kind {
		case syncpkg.KindCreate:
			sym = ui.Green(sym)
		case syncpkg.KindDelete, syncpkg.KindConflict:
			sym = ui.Red(sym)
		default:
			sym = ui.Yellow(sym)
		}
		fmt.Printf("  %s %s  %s\n", sym, ui.Bold(ch.Ref), ui.Dim(changeTags(ch)))
	}
	cr, up, de, cf, rm := plan.Summary()
	fmt.Printf("\nPlan: %d to create, %d to update, %d to delete", cr, up, de)
	if cf > 0 {
		fmt.Printf(", %s", ui.Red(fmt.Sprintf("%d conflict", cf)))
	}
	if rm > 0 {
		fmt.Printf(", %s", ui.Yellow(fmt.Sprintf("%d remote-changed", rm)))
	}
	fmt.Printf("  (serial %d)\n", serial)
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
