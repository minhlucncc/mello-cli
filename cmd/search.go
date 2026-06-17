package cmd

import (
	"fmt"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/mello"
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

	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()

	// Search the -w workspace, else the current workspace, else all of them.
	var wsIDs []string
	if *wsFlag != "" {
		id, err := matchWorkspace(cx, cl, *wsFlag)
		if err != nil {
			return err
		}
		wsIDs = []string{id}
	} else if id, _, ok := currentWorkspace(); ok {
		wsIDs = []string{id}
	} else {
		wss, err := cl.ListWorkspaces(cx)
		if err != nil {
			return err
		}
		for _, w := range wss {
			wsIDs = append(wsIDs, w.ID)
		}
	}

	var tickets []mello.Ticket
	seen := map[string]bool{}
	for _, id := range wsIDs {
		hits, serr := cl.Search(cx, id, query)
		if serr != nil {
			continue
		}
		for _, t := range hits {
			if !seen[t.ID] {
				seen[t.ID] = true
				tickets = append(tickets, t)
			}
		}
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
