package cmd

import "github.com/minhlucncc/mello-cli/internal/ui"

func memberCmd() *Command {
	return &Command{
		Name:  "member",
		Short: "List workspace members.",
		Subs: []*Command{
			{Name: "list", Short: "List members of a workspace.", Run: memberList},
		},
	}
}

func memberList(args []string) error {
	fs, c := newFlags("member list")
	wsFlag := fs.String("workspace", "", "workspace id")
	fs.StringVar(wsFlag, "w", "", "workspace id (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
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
	members, err := cachedMembers(cx, cl, ws)
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(members)
	}
	rows := make([][]string, 0, len(members))
	for _, m := range members {
		rows = append(rows, []string{m.UserID, m.Name, m.Email, m.Role})
	}
	ui.Table([]string{"user id", "name", "email", "role"}, rows)
	return nil
}
