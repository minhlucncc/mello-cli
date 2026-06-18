package cmd

import (
	"fmt"

	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func stashCmd() *Command {
	return &Command{
		Name:  "stash",
		Short: "Set aside local changes so you can pull, then reapply them.",
		Subs: []*Command{
			{Name: "add", Short: "Save local changes and reset the working copy to baseline.", Run: stashSave},
			{Name: "save", Short: "Alias of add.", Run: stashSave},
			{Name: "list", Short: "List saved stashes.", Run: stashList},
			{Name: "show", Short: "Show a stash (default: latest).", Run: stashShow},
			{Name: "apply", Short: "Restore a stash, keeping it (default: latest).", Run: stashApply},
			{Name: "pop", Short: "Restore a stash and drop it (default: latest).", Run: stashPop},
			{Name: "drop", Short: "Delete a stash.", Run: stashDrop},
		},
	}
}

func stashApply(args []string) error { return stashApplyCmd(args, false) }
func stashPop(args []string) error   { return stashApplyCmd(args, true) }

func stashSave(args []string) error {
	fs, c := newFlags("stash")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	all := fs.Bool("all", false, "all boards in the workspace")
	msg := fs.String("message", "", "stash description")
	fs.StringVar(msg, "m", "", "stash description (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	saved := 0
	err := forEachBoard(c, *dir, *board, *all, func(s *syncpkg.Syncer) error {
		entry, err := s.Stash(*msg)
		if err != nil {
			return err
		}
		if entry == nil {
			fmt.Println(ui.Dim("nothing to stash on " + s.Board.Name))
			return nil
		}
		saved++
		ui.Successf("Stashed %d ticket(s) as %s on %s", len(entry.Tickets), ui.Bold(entry.ID), s.Board.Name)
		return nil
	})
	if err != nil {
		return err
	}
	if saved > 0 {
		fmt.Println(ui.Dim("working copy reset to baseline — `mello pull`, then `mello stash pop`"))
	}
	return nil
}

func stashList(args []string) error {
	fs, c := newFlags("stash list")
	dir := fs.String("dir", ".", "workspace directory")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	list, err := tree.ListStash()
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(list)
	}
	if len(list) == 0 {
		fmt.Println(ui.Dim("no stash entries"))
		return nil
	}
	rows := make([][]string, 0, len(list))
	for _, e := range list {
		rows = append(rows, []string{e.ID, e.Board, fmt.Sprintf("%d", len(e.Tickets)), e.CreatedAt, e.Message})
	}
	ui.Table([]string{"id", "board", "tickets", "created", "message"}, rows)
	return nil
}

func stashShow(args []string) error {
	fs, c := newFlags("stash show")
	dir := fs.String("dir", ".", "workspace directory")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	id := ""
	if fs.NArg() > 0 {
		id = fs.Arg(0)
	}
	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	entry, err := tree.GetStash(id)
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(entry)
	}
	fmt.Printf("%s  %s  %s\n", ui.Bold(entry.ID), entry.Board, ui.Dim(entry.CreatedAt))
	if entry.Message != "" {
		fmt.Println(entry.Message)
	}
	for _, st := range entry.Tickets {
		tag := ""
		if st.Created {
			tag = ui.Dim(" (new local)")
		}
		fmt.Printf("  %s%s\n", firstNonEmpty(st.Ref, st.Slug), tag)
	}
	return nil
}

func stashApplyCmd(args []string, pop bool) error {
	fs, c := newFlags("stash apply")
	dir := fs.String("dir", ".", "workspace directory")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	id := ""
	if fs.NArg() > 0 {
		id = fs.Arg(0)
	}
	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	entry, err := tree.GetStash(id)
	if err != nil {
		return err
	}
	bs, err := tree.ResolveBoard(entry.Board)
	if err != nil {
		return err
	}
	s := &syncpkg.Syncer{Tree: tree, Board: bs, Log: logfUI}
	if err := s.StashApply(entry, pop); err != nil {
		return err
	}
	verb := "Applied"
	if pop {
		verb = "Popped"
	}
	ui.Successf("%s %s (%d ticket(s)) — review then `mello push`", verb, entry.ID, len(entry.Tickets))
	return nil
}

func stashDrop(args []string) error {
	fs, c := newFlags("stash drop")
	dir := fs.String("dir", ".", "workspace directory")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello stash drop <id>")
	}
	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	if _, err := tree.GetStash(fs.Arg(0)); err != nil {
		return err
	}
	if err := tree.DropStash(fs.Arg(0)); err != nil {
		return err
	}
	ui.Successf("Dropped %s", fs.Arg(0))
	return nil
}
