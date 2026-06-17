package cmd

import (
	"fmt"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/config"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func workspaceCmd() *Command {
	return &Command{
		Name:  "workspace",
		Short: "List workspaces and set the default one.",
		Subs: []*Command{
			{Name: "list", Short: "List accessible workspaces.", Run: workspaceList},
			{Name: "use", Short: "Set the default workspace for this profile.", Run: workspaceUse},
		},
	}
}

func workspaceList(args []string) error {
	fs, c := newFlags("workspace list")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	ws, err := cachedWorkspaces(cx, cl)
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(ws)
	}
	rows := make([][]string, 0, len(ws))
	for _, w := range ws {
		rows = append(rows, []string{w.ID, w.Name, w.Role})
	}
	ui.Table([]string{"id", "name", "role"}, rows)
	return nil
}

func workspaceUse(args []string) error {
	fs, c := newFlags("workspace use")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello workspace use <id|name|board|url>")
	}
	want := normalizeSelector(fs.Arg(0))

	cl, r, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	ws, err := cachedWorkspaces(cx, cl)
	if err != nil {
		return err
	}

	// 1. Match a workspace by id or name.
	for _, w := range ws {
		if w.ID == want || strings.EqualFold(w.Name, want) {
			if err := config.SetProfile(r.Profile, "", "", w.ID, false); err != nil {
				return err
			}
			ui.Successf("Default workspace set to %s (%s)", ui.Bold(w.Name), w.ID)
			return nil
		}
	}

	// 2. Maybe it's a board (id/code/url) — use that board's workspace.
	if wsID, wsName, _, berr := findBoardAnywhere(cx, cl, want); berr == nil {
		if err := config.SetProfile(r.Profile, "", "", wsID, false); err != nil {
			return err
		}
		ui.Successf("Default workspace set to %s (%s) — from board %q", ui.Bold(wsName), wsID, want)
		return nil
	}

	// 3. Not found — show the choices.
	return fmt.Errorf("no workspace or board matching %q. Your workspaces:\n%s", want, workspaceLines(ws))
}
