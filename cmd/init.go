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
		Short: "Create an empty local .mello workspace.",
		Run:   initRun,
	}
}

func initRun(args []string) error {
	fs, c := newFlags("init")
	wsFlag := fs.String("workspace", "", "optional workspace id to bind up front")
	fs.StringVar(wsFlag, "w", "", "optional workspace id (shorthand)")
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

	// init needs nothing — not even a token. The workspace is bound later, when
	// the first board is checked out (it is taken from that board). A workspace
	// may be pre-bound with -w, or inherited from config, but neither is required.
	r, _ := c.resolveConfig()

	tree, err := syncpkg.InitWorkspace(dir, &syncpkg.State{
		Profile:     r.Profile,
		BaseURL:     r.BaseURL,
		WorkspaceID: firstNonEmpty(*wsFlag, r.WorkspaceID),
	})
	if err != nil {
		return err
	}
	ui.Successf("Initialized empty mello workspace in %s/%s", tree.Root, syncpkg.DirName)
	fmt.Println(ui.Dim("next: mello use <board>   (e.g. mello use ROAD)"))
	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
