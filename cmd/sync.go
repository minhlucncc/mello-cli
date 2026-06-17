package cmd

import (
	"fmt"

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
	fs, c := newFlags("sync clone")
	board := fs.String("board", "", "board id or code")
	fs.StringVar(board, "b", "", "board id or code (shorthand)")
	wsFlag := fs.String("workspace", "", "workspace id")
	fs.StringVar(wsFlag, "w", "", "workspace id (shorthand)")
	dir := fs.String("dir", ".", "workspace directory")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if *board == "" {
		return fmt.Errorf("usage: mello sync clone -b <board> [-w <ws>] [--dir D]")
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
		if *wsFlag != "" && *wsFlag != tree.State.WorkspaceID {
			return fmt.Errorf("this workspace tracks %s; -w %s does not match", tree.State.WorkspaceID, *wsFlag)
		}
	} else {
		ws, werr := requireWorkspace(*wsFlag, r)
		if werr != nil {
			return werr
		}
		tree, err = syncpkg.InitWorkspace(*dir, &syncpkg.State{
			Profile: r.Profile, BaseURL: r.BaseURL, WorkspaceID: ws,
		})
		if err != nil {
			return err
		}
	}

	boards, err := cl.ListBoards(cx, tree.State.WorkspaceID)
	if err != nil {
		return err
	}
	var bs *syncpkg.BoardState
	for _, b := range boards {
		if b.ID == *board || b.Code == *board {
			slug := syncpkg.Slugify(firstNonEmpty(b.Code, b.Name))
			if existing := tree.State.Boards[slug]; existing != nil {
				bs = existing
			} else {
				bs = &syncpkg.BoardState{BoardID: b.ID, Slug: slug, Name: b.Name, Code: b.Code}
				tree.AddBoard(bs)
			}
			break
		}
	}
	if bs == nil {
		return fmt.Errorf("no board %q in the workspace (run `mello board list`)", *board)
	}

	s := &syncpkg.Syncer{API: cl, Tree: tree, Board: bs, Log: logfUI}
	n, err := s.Clone(cx)
	if err != nil {
		return err
	}
	ui.Successf("Checked out board %s — %d ticket(s)", ui.Bold(bs.Name), n)
	return nil
}

// forEachBoard runs fn for each selected board, printing a header when more than
// one board is involved.
func forEachBoard(c *common, dir, boardSel string, fn func(*syncpkg.Syncer) error) error {
	tree, err := syncpkg.Open(dir)
	if err != nil {
		return err
	}
	boards, err := tree.SelectBoards(boardSel)
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
	board := fs.String("board", "", "limit to one board (id, code, name, or slug)")
	fs.StringVar(board, "b", "", "limit to one board (shorthand)")
	remote := fs.Bool("remote", false, "also fetch remote to detect drift/conflicts")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	return forEachBoard(c, *dir, *board, func(s *syncpkg.Syncer) error {
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
	fs, c := newFlags("sync pull")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "limit to one board")
	fs.StringVar(board, "b", "", "limit to one board (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	return forEachBoard(c, *dir, *board, func(s *syncpkg.Syncer) error {
		updated, deleted, err := s.Pull(cx)
		if err != nil {
			return err
		}
		if len(s.Conflicts) > 0 {
			ui.Warnf("%d conflict(s) — resolve, then `mello sync push`", len(s.Conflicts))
		}
		ui.Successf("Pulled: %d updated, %d removed (serial %d)", updated, deleted, s.Tree.State.Serial)
		return nil
	})
}

func syncPush(args []string) error {
	fs, c := newFlags("sync push")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "limit to one board")
	fs.StringVar(board, "b", "", "limit to one board (shorthand)")
	dryRun := fs.Bool("dry-run", false, "show what would be pushed, change nothing")
	force := fs.Bool("force", false, "push conflicts (local over remote)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	return forEachBoard(c, *dir, *board, func(s *syncpkg.Syncer) error {
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
	board := fs.String("board", "", "limit to one board")
	fs.StringVar(board, "b", "", "limit to one board (shorthand)")
	force := fs.Bool("force", false, "push conflicts (local over remote)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	return forEachBoard(c, *dir, *board, func(s *syncpkg.Syncer) error {
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
