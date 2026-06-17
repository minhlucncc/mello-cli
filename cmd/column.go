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
	board := fs.String("board", "", "board id")
	fs.StringVar(board, "b", "", "board id (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if *board == "" {
		return fmt.Errorf("usage: mello column list -b <board>")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	cols, err := cl.ListColumns(cx, *board)
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
	board := fs.String("board", "", "board id")
	fs.StringVar(board, "b", "", "board id (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if *board == "" || fs.NArg() < 1 {
		return fmt.Errorf("usage: mello column create -b <board> <name>")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	col, err := cl.CreateColumn(cx, *board, fs.Arg(0))
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(col)
	}
	ui.Successf("Created column %s — %s", ui.Bold(col.Name), col.ID)
	return nil
}
