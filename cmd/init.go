package cmd

import (
	"fmt"
	"path/filepath"

	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func initCmd() *Command {
	return &Command{
		Name:  "init",
		Short: "Create a local .mello workspace in a directory.",
		Run:   initRun,
	}
}

func initRun(args []string) error {
	fs, c := newFlags("init")
	wsFlag := fs.String("workspace", "", "workspace id or name")
	fs.StringVar(wsFlag, "w", "", "workspace id or name (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if syncpkg.Exists(dir) {
		return fmt.Errorf("%s is already a mello workspace", filepath.Join(dir, syncpkg.DirName))
	}

	r, err := c.resolveConfig()
	if err != nil {
		return err
	}

	wsID := firstNonEmpty(*wsFlag, r.WorkspaceID)
	wsName := ""
	// If authenticated, resolve the workspace to validate it and capture its name.
	if r.Token != "" {
		cl, _, cerr := c.client()
		if cerr == nil {
			cx, cancel := ctx()
			defer cancel()
			workspaces, lerr := cl.ListWorkspaces(cx)
			if lerr != nil {
				return lerr
			}
			if wsID == "" && len(workspaces) == 1 {
				wsID = workspaces[0].ID
			}
			matched := false
			for _, w := range workspaces {
				if w.ID == wsID || w.Name == wsID {
					wsID, wsName, matched = w.ID, w.Name, true
					break
				}
			}
			if !matched && wsID != "" {
				return fmt.Errorf("no workspace matching %q (see `mello workspace list`)", wsID)
			}
		}
	}
	if wsID == "" {
		return fmt.Errorf("no workspace set — pass -w <id|name> or run `mello workspace use <id>`")
	}

	tree, err := syncpkg.InitWorkspace(dir, &syncpkg.State{
		Profile: r.Profile, BaseURL: r.BaseURL, WorkspaceID: wsID, WorkspaceName: wsName,
	})
	if err != nil {
		return err
	}
	label := wsName
	if label == "" {
		label = wsID
	}
	ui.Successf("Initialized empty mello workspace for %s in %s/%s",
		ui.Bold(label), tree.Root, syncpkg.DirName)
	fmt.Println(ui.Dim("next: mello sync clone -b <board>"))
	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
