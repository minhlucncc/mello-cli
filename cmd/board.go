package cmd

import (
	"fmt"

	"github.com/minhlucncc/mello-cli/internal/mello"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func boardCmd() *Command {
	return &Command{
		Name:  "board",
		Short: "List, create, and view boards.",
		Subs: []*Command{
			{Name: "list", Short: "List boards (across your workspaces, or the current one).", Run: boardList},
			{Name: "create", Short: "Create a board.", Run: boardCreate},
			{Name: "view", Short: "Show a board's columns and ticket counts.", Run: boardView},
			{Name: "use", Short: "Set the working board for this workspace.", Run: useRun},
		},
	}
}

func boardList(args []string) error {
	fs, c := newFlags("board list")
	wsFlag := fs.String("workspace", "", "limit to a workspace (id or name)")
	fs.StringVar(wsFlag, "w", "", "workspace (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()

	// Scope to one workspace if -w is given; otherwise list boards across every
	// workspace the token can access (no setup required).
	var workspaces []mello.Workspace
	if *wsFlag != "" {
		id, err := matchWorkspace(cx, cl, *wsFlag)
		if err != nil {
			return err
		}
		workspaces = []mello.Workspace{{ID: id, Name: *wsFlag}}
	} else if id, name, ok := currentWorkspace(); ok {
		// Inside a workspace: show that workspace's boards.
		workspaces = []mello.Workspace{{ID: id, Name: name}}
	} else {
		// Otherwise discover boards across all accessible workspaces.
		workspaces, err = cl.ListWorkspaces(cx)
		if err != nil {
			return err
		}
	}

	type entry struct {
		Workspace string      `json:"workspace"`
		Board     mello.Board `json:"board"`
	}
	var all []entry
	for _, w := range workspaces {
		boards, berr := cl.ListBoards(cx, w.ID)
		if berr != nil {
			continue
		}
		name := w.Name
		for _, b := range boards {
			all = append(all, entry{Workspace: name, Board: b})
		}
	}
	if c.json {
		return ui.JSON(all)
	}
	if len(all) == 0 {
		fmt.Println(ui.Dim("no boards"))
		return nil
	}
	multi := len(workspaces) > 1
	header := []string{"code", "name", "id"}
	if multi {
		header = []string{"workspace", "code", "name", "id"}
	}
	rows := make([][]string, 0, len(all))
	for _, e := range all {
		if multi {
			rows = append(rows, []string{e.Workspace, e.Board.Code, e.Board.Name, e.Board.ID})
		} else {
			rows = append(rows, []string{e.Board.Code, e.Board.Name, e.Board.ID})
		}
	}
	ui.Table(header, rows)
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
	cx, cancel := ctx()
	defer cancel()
	ws, err := workspaceContext(cx, cl, c, *wsFlag, r)
	if err != nil {
		return err
	}
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
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	sel := *board
	if sel == "" && fs.NArg() > 0 {
		sel = fs.Arg(0)
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	boardID, _, err := resolveBoardID(cx, cl, sel)
	if err != nil {
		return err
	}
	cols, err := cl.ListColumns(cx, boardID)
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
