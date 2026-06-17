package cmd

import (
	"fmt"

	"github.com/minhlucncc/mello-cli/internal/ui"
)

func boardCmd() *Command {
	return &Command{
		Name:  "board",
		Short: "List, create, and view boards.",
		Subs: []*Command{
			{Name: "list", Short: "List boards in a workspace.", Run: boardList},
			{Name: "create", Short: "Create a board.", Run: boardCreate},
			{Name: "view", Short: "Show a board's columns and ticket counts.", Run: boardView},
		},
	}
}

func boardList(args []string) error {
	fs, c := newFlags("board list")
	wsFlag := fs.String("workspace", "", "workspace id")
	fs.StringVar(wsFlag, "w", "", "workspace id (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
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
	if c.json {
		return ui.JSON(boards)
	}
	rows := make([][]string, 0, len(boards))
	for _, b := range boards {
		rows = append(rows, []string{b.ID, b.Code, b.Name})
	}
	ui.Table([]string{"id", "code", "name"}, rows)
	return nil
}

func boardCreate(args []string) error {
	fs, c := newFlags("board create")
	wsFlag := fs.String("workspace", "", "workspace id")
	fs.StringVar(wsFlag, "w", "", "workspace id (shorthand)")
	code := fs.String("code", "", "optional board code (auto-generated if empty)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello board create <name> [--code C] [-w <ws>]")
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
	b, err := cl.CreateBoard(cx, ws, fs.Arg(0), *code)
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(b)
	}
	ui.Successf("Created board %s (%s) — %s", ui.Bold(b.Name), b.Code, b.ID)
	return nil
}

func boardView(args []string) error {
	fs, c := newFlags("board view")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello board view <board-id>")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	cols, err := cl.ListColumns(cx, fs.Arg(0))
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(cols)
	}
	rows := make([][]string, 0, len(cols))
	for _, col := range cols {
		rows = append(rows, []string{fmt.Sprintf("%d", col.Position), col.Name, fmt.Sprintf("%d", len(col.Tickets)), col.ID})
	}
	ui.Table([]string{"pos", "column", "tickets", "id"}, rows)
	return nil
}
