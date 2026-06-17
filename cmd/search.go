package cmd

import (
	"fmt"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/ui"
)

func searchCmd() *Command {
	return &Command{
		Name:  "search",
		Short: "Full-text search tickets in a workspace.",
		Run:   searchRun,
	}
}

func searchRun(args []string) error {
	fs, c := newFlags("search")
	wsFlag := fs.String("workspace", "", "workspace id")
	fs.StringVar(wsFlag, "w", "", "workspace id (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello search <query> [-w <ws>]")
	}
	query := strings.Join(fs.Args(), " ")

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
	tickets, err := cl.Search(cx, ws, query)
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(tickets)
	}
	rows := make([][]string, 0, len(tickets))
	for _, t := range tickets {
		rows = append(rows, []string{ticketRef(t), t.Status, ui.Truncate(t.Title, 60)})
	}
	ui.Table([]string{"ticket", "status", "title"}, rows)
	if len(tickets) == 0 {
		fmt.Println(ui.Dim("no matches"))
	}
	return nil
}
