package cmd

import (
	"fmt"

	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func syncCmd() *Command {
	return &Command{
		Name:  "sync",
		Short: "Local↔remote board mirror: clone, status, pull, push, sync.",
		Subs: []*Command{
			{Name: "clone", Short: "Pull a board into a local .mello working copy.", Run: syncClone},
			{Name: "status", Short: "Show the plan of pending local changes.", Run: syncStatus},
			{Name: "pull", Short: "Fetch remote changes into the working copy.", Run: syncPull},
			{Name: "push", Short: "Apply local creates/edits/moves/deletes to remote.", Run: syncPush},
			{Name: "sync", Short: "Reconcile: pull then push.", Run: syncReconcile},
		},
	}
}

func syncClone(args []string) error {
	fs, c := newFlags("sync clone")
	board := fs.String("board", "", "board id or code")
	fs.StringVar(board, "b", "", "board id (shorthand)")
	wsFlag := fs.String("workspace", "", "workspace id")
	fs.StringVar(wsFlag, "w", "", "workspace id (shorthand)")
	dir := fs.String("dir", ".", "destination directory for the .mello working copy")
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
	ws, err := requireWorkspace(*wsFlag, r)
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()

	boards, err := cl.ListBoards(cx, ws)
	if err != nil {
		return err
	}
	name, code, id := "", "", ""
	for _, b := range boards {
		if b.ID == *board || b.Code == *board {
			name, code, id = b.Name, b.Code, b.ID
			break
		}
	}
	if id == "" {
		return fmt.Errorf("no board %q in workspace (run `mello board list`)", *board)
	}
	slug := syncpkg.Slugify(firstSlug(code, name))

	tree, err := syncpkg.Init(*dir, &syncpkg.State{
		Profile: r.Profile, BaseURL: r.BaseURL, WorkspaceID: ws,
		BoardID: id, BoardSlug: slug, BoardName: name,
	})
	if err != nil {
		return err
	}
	s := &syncpkg.Syncer{API: cl, Tree: tree, Log: logfUI}
	n, err := s.Clone(cx)
	if err != nil {
		return err
	}
	ui.Successf("Cloned board %s — %d ticket(s) into %s/%s", ui.Bold(name), n, tree.Root, syncpkg.DirName)
	return nil
}

func syncStatus(args []string) error {
	fs, c := newFlags("sync status")
	dir := fs.String("dir", ".", "working copy directory")
	remote := fs.Bool("remote", false, "also fetch remote to detect drift/conflicts")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	s := &syncpkg.Syncer{API: cl, Tree: tree}
	plan, err := s.ComputePlan(cx, *remote)
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(plan)
	}
	printPlan(plan, tree.State.Serial)
	return nil
}

func syncPull(args []string) error {
	fs, c := newFlags("sync pull")
	dir := fs.String("dir", ".", "working copy directory")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	s := &syncpkg.Syncer{API: cl, Tree: tree, Log: logfUI}
	updated, deleted, err := s.Pull(cx)
	if err != nil {
		return err
	}
	if len(s.Conflicts) > 0 {
		ui.Warnf("%d conflict(s) — resolve, then `mello sync push`", len(s.Conflicts))
	}
	ui.Successf("Pulled: %d updated, %d removed (serial %d)", updated, deleted, tree.State.Serial)
	return nil
}

func syncPush(args []string) error {
	fs, c := newFlags("sync push")
	dir := fs.String("dir", ".", "working copy directory")
	dryRun := fs.Bool("dry-run", false, "show what would be pushed, change nothing")
	force := fs.Bool("force", false, "push conflicts (local over remote)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	s := &syncpkg.Syncer{API: cl, Tree: tree, Log: logfUI}
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
		ui.Successf("Push complete (serial %d)", tree.State.Serial)
	}
	return nil
}

func syncReconcile(args []string) error {
	fs, c := newFlags("sync sync")
	dir := fs.String("dir", ".", "working copy directory")
	force := fs.Bool("force", false, "push conflicts (local over remote)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	s := &syncpkg.Syncer{API: cl, Tree: tree, Log: logfUI}

	fmt.Println(ui.Bold("pull:"))
	updated, deleted, err := s.Pull(cx)
	if err != nil {
		return err
	}
	fmt.Printf("  %d updated, %d removed\n", updated, deleted)
	if len(s.Conflicts) > 0 && !*force {
		ui.Warnf("%d conflict(s) — resolve then push (or rerun with --force)", len(s.Conflicts))
	}

	fmt.Println(ui.Bold("push:"))
	plan, err := s.ComputePlan(cx, true)
	if err != nil {
		return err
	}
	if len(plan.Changes) == 0 {
		fmt.Println(ui.Dim("  clean"))
		return nil
	}
	if err := s.Apply(cx, plan, false, *force); err != nil {
		return err
	}
	ui.Successf("Reconciled (serial %d)", tree.State.Serial)
	return nil
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
		case syncpkg.KindDelete:
			sym = ui.Red(sym)
		case syncpkg.KindConflict:
			sym = ui.Red(sym)
		case syncpkg.KindRemote:
			sym = ui.Yellow(sym)
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

func firstSlug(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
