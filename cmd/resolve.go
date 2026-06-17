package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/config"
	"github.com/minhlucncc/mello-cli/internal/mello"
	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

// resolveBoardID returns the board a command should act on. With no selector it
// uses the working board recorded in the .mello workspace (in or above the
// current directory). A selector (-b) overrides it, matched first against the
// workspace's checked-out boards, then across all boards the token can access.
func resolveBoardID(cx context.Context, cl *mello.Client, sel string) (id, name string, err error) {
	if tree, terr := syncpkg.Open("."); terr == nil {
		if bs, rerr := tree.ResolveBoard(sel); rerr == nil {
			return bs.BoardID, bs.Name, nil
		} else if sel == "" {
			return "", "", rerr
		}
	}
	if sel == "" {
		return "", "", errors.New("no working board — run inside a workspace (`mello clone <board>`) or pass -b <board>")
	}
	_, _, b, ferr := findBoardAnywhere(cx, cl, sel)
	if ferr != nil {
		return "", "", ferr
	}
	return b.ID, b.Name, nil
}

// currentWorkspace returns the workspace bound to the .mello working copy in or
// above the current directory, if any.
func currentWorkspace() (id, name string, ok bool) {
	if tree, err := syncpkg.Open("."); err == nil && tree.State.WorkspaceID != "" {
		return tree.State.WorkspaceID, tree.State.WorkspaceName, true
	}
	return "", "", false
}

// workspaceContext resolves the workspace a command should act on: the -w flag,
// then the workspace bound to the current .mello (no friction inside a
// workspace), then the saved default / interactive selection.
func workspaceContext(cx context.Context, cl *mello.Client, c *common, flagVal string, r config.Resolved) (string, error) {
	if flagVal != "" {
		return matchWorkspace(cx, cl, flagVal)
	}
	if id, _, ok := currentWorkspace(); ok {
		return id, nil
	}
	return resolveWorkspace(cx, cl, c, "", r)
}

// resolveWorkspace returns a single workspace id for commands that act on one
// workspace. Resolution order: the -w flag (matched by id or name), the saved
// default, then the token's workspaces — auto-selecting when there is exactly
// one, prompting interactively when there are several (and remembering the
// choice), or listing them when input is not a terminal.
func resolveWorkspace(cx context.Context, cl *mello.Client, c *common, flagVal string, r config.Resolved) (string, error) {
	if flagVal != "" {
		return matchWorkspace(cx, cl, flagVal)
	}
	if r.WorkspaceID != "" {
		return r.WorkspaceID, nil
	}
	wss, err := cl.ListWorkspaces(cx)
	if err != nil {
		return "", err
	}
	switch len(wss) {
	case 0:
		return "", errors.New("your token has access to no workspaces")
	case 1:
		rememberWorkspace(c, wss[0].ID)
		return wss[0].ID, nil
	}
	if ui.IsInteractive() {
		opts := make([]string, len(wss))
		for i, w := range wss {
			opts[i] = fmt.Sprintf("%s  %s", w.Name, ui.Dim(w.ID))
		}
		idx, err := ui.Select("Select a workspace:", opts)
		if err != nil {
			return "", err
		}
		rememberWorkspace(c, wss[idx].ID)
		ui.Warnf("saved %s as the default workspace (change it with `mello workspace use`)", wss[idx].Name)
		return wss[idx].ID, nil
	}
	return "", fmt.Errorf("multiple workspaces available — pass -w <id> or run `mello workspace use <id>`:\n%s",
		workspaceLines(wss))
}

// matchWorkspace turns a -w value (id or name) into a workspace id.
func matchWorkspace(cx context.Context, cl *mello.Client, sel string) (string, error) {
	wss, err := cl.ListWorkspaces(cx)
	if err != nil {
		return sel, nil // can't list; assume the caller passed an id
	}
	for _, w := range wss {
		if w.ID == sel || strings.EqualFold(w.Name, sel) {
			return w.ID, nil
		}
	}
	return "", fmt.Errorf("no workspace matching %q (run `mello workspace list`)", sel)
}

func rememberWorkspace(c *common, id string) {
	_ = config.SetProfile(config.ActiveProfile(c.profile), "", "", id, false)
}

func workspaceLines(wss []mello.Workspace) string {
	var b strings.Builder
	for _, w := range wss {
		fmt.Fprintf(&b, "  %s  %s\n", w.ID, w.Name)
	}
	return strings.TrimRight(b.String(), "\n")
}
