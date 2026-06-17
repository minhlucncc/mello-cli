package cmd

import (
	"fmt"

	"github.com/minhlucncc/mello-cli/internal/ui"
)

func columnCmd() *Command {
	return &Command{
		Name:  "column",
		Short: "List and create board columns (status lanes).",
		Subs: []*Command{
			{Name: "list", Short: "List a board's columns.", Run: columnList},
			{Name: "create", Short: "Add a column to a board.", Run: columnCreate},
		},
	}
}

func columnList(args []string) error {
	fs, c := newFlags("column list")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	boardID, _, err := resolveBoardID(cx, cl, *board)
	if err != nil {
		return err
	}
	cols, err := cachedColumns(cx, cl, boardID)
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(cols)
	}
	rows := make([][]string, 0, len(cols))
	for _, col := range cols {
		rows = append(rows, []string{fmt.Sprintf("%d", col.Position), col.Name, col.ID})
	}
	ui.Table([]string{"pos", "name", "id"}, rows)
	return nil
}

func columnCreate(args []string) error {
	fs, c := newFlags("column create")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello column create <name> [-b <board>]")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	boardID, _, err := resolveBoardID(cx, cl, *board)
	if err != nil {
		return err
	}
	col, err := cl.CreateColumn(cx, boardID, fs.Arg(0))
	if err != nil {
		return err
	}
	invalidateCache("columns:" + boardID)
	if c.json {
		return ui.JSON(col)
	}
	ui.Successf("Created column %s — %s", ui.Bold(col.Name), col.ID)
	return nil
}
