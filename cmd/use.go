package cmd

import (
	"fmt"

	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func useCmd() *Command {
	return &Command{
		Name:  "use",
		Short: "Attach a board to the workspace and make it the working board.",
		Run:   useRun,
	}
}

// useRun checks a board out into the current workspace (if not already present)
// and sets it as the working board. Every other command then acts on it with no
// further flags.
func useRun(args []string) error {
	fs, c := newFlags("use")
	dir := fs.String("dir", ".", "workspace directory")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello use <board>")
	}
	sel := fs.Arg(0)

	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return fmt.Errorf("not a mello workspace — run `mello init` here first")
	}

	// Already checked out: just switch the working board (no network needed).
	if bs, rerr := tree.ResolveBoard(sel); rerr == nil {
		tree.State.CurrentBoard = bs.Slug
		if err := tree.Save(); err != nil {
			return err
		}
		ui.Successf("Working board: %s", ui.Bold(bs.Name))
		return nil
	}

	// Otherwise resolve and register the board (no ticket download) and make it
	// current. Tickets are pulled lazily or created locally.
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	bs, err := attachBoard(cx, cl, tree, sel)
	if err != nil {
		return err
	}
	tree.State.CurrentBoard = bs.Slug
	if err := tree.Save(); err != nil {
		return err
	}
	ui.Successf("Working board: %s", ui.Bold(bs.Name))
	fmt.Println(ui.Dim("browse: mello ticket list · fetch one: mello pull <ticket> · create: mello new ticket"))
	return nil
}
