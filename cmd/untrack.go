package cmd

import (
	"fmt"

	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func untrackCmd() *Command {
	return &Command{
		Name:  "untrack",
		Short: "Remove tickets from the local working set (keeps them on the server).",
		Run:   untrackRun,
	}
}

// untrackRun drops tickets from the working set: it removes the local files and
// stops tracking them, without deleting anything on the server. Use it when you
// are done with a ticket and want to focus on others.
func untrackRun(args []string) error {
	fs, c := newFlags("untrack")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	all := fs.Bool("all", false, "untrack every ticket in the working set")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	bs, err := tree.ResolveBoard(*board)
	if err != nil {
		return err
	}

	if *all {
		n := len(bs.Tickets)
		for slug := range bs.Tickets {
			tree.Untrack(bs, slug)
		}
		if err := tree.Save(); err != nil {
			return err
		}
		ui.Successf("Untracked %d ticket(s) from %s", n, ui.Bold(bs.Name))
		return nil
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello untrack <ticket>... [--all]")
	}
	untracked := 0
	for _, sel := range fs.Args() {
		slug, ok := bs.FindTicketSlug(sel)
		if !ok {
			ui.Warnf("not in the working set: %s", sel)
			continue
		}
		tree.Untrack(bs, slug)
		ui.Successf("Untracked %s", sel)
		untracked++
	}
	if untracked == 0 {
		return nil
	}
	return tree.Save()
}
