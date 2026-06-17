package cmd

import (
	"fmt"

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
	ws, err := cl.ListWorkspaces(cx)
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
		return fmt.Errorf("usage: mello workspace use <id|name>")
	}
	want := fs.Arg(0)

	cl, r, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	ws, err := cl.ListWorkspaces(cx)
	if err != nil {
		return err
	}
	id, name := "", ""
	for _, w := range ws {
		if w.ID == want || w.Name == want {
			id, name = w.ID, w.Name
			break
		}
	}
	if id == "" {
		return fmt.Errorf("no workspace matching %q (run `mello workspace list`)", want)
	}
	if err := config.SetProfile(r.Profile, "", "", id, false); err != nil {
		return err
	}
	ui.Successf("Default workspace set to %s (%s)", ui.Bold(name), id)
	return nil
}
