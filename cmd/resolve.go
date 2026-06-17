package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/config"
	"github.com/minhlucncc/mello-cli/internal/mello"
	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

// normalizeSelector lets a board/ticket/workspace be given as a URL, id, code,
// or name. From a Mello URL it returns the last path segment — e.g.
// https://mello.mezon.vn/boards/<id> → <id>, …/tickets/<code> → <code>.
// Non-URLs are returned unchanged (trimmed).
func normalizeSelector(s string) string {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "://") {
		return s
	}
	u, err := url.Parse(s)
	if err != nil {
		return s
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return s
}

// resolveAssignee turns a filter value into a user id, expanding "me"/"@me" to
// the authenticated user (from config, falling back to /me).
func resolveAssignee(cx context.Context, cl *mello.Client, c *common, val string) (string, error) {
	if val != "me" && val != "@me" {
		return val, nil
	}
	if r, err := c.resolveConfig(); err == nil && r.UserID != "" {
		return r.UserID, nil
	}
	u, err := cl.GetMe(cx)
	if err != nil {
		if mello.IsNotFound(err) {
			return "", fmt.Errorf("cannot resolve \"me\": this Mello instance has no /me endpoint — pass --assignee <your-user-id>")
		}
		return "", err
	}
	return u.ID, nil
}

// resolveTicketID maps a ticket selector (code or id) to a remote id, using the
// working board — the working-set record first, then a live scan of the board's
// columns — and falling back to the selector itself.
func resolveTicketID(cx context.Context, cl *mello.Client, sel string) string {
	sel = normalizeSelector(sel)
	tree, err := syncpkg.Open(".")
	if err != nil {
		return sel
	}
	bs, err := tree.ResolveBoard("")
	if err != nil {
		return sel
	}
	if slug, ok := bs.FindTicketSlug(sel); ok {
		if rec := bs.Tickets[slug]; rec != nil && rec.RemoteID != "" {
			return rec.RemoteID
		}
	}
	if cols, cerr := cl.ListColumns(cx, bs.BoardID); cerr == nil {
		for _, col := range cols {
			for _, t := range col.Tickets {
				if t.ID == sel || strings.EqualFold(t.TicketCode, sel) {
					return t.ID
				}
			}
		}
	}
	return sel
}

// resolveBoardID returns the board a command should act on. With no selector it
// uses the working board recorded in the .mello workspace (in or above the
// current directory). A selector (-b) overrides it, matched first against the
// workspace's checked-out boards, then across all boards the token can access.
func resolveBoardID(cx context.Context, cl *mello.Client, sel string) (id, name string, err error) {
	sel = normalizeSelector(sel)
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

// matchWorkspace turns a -w value (id, name, or URL) into a workspace id.
func matchWorkspace(cx context.Context, cl *mello.Client, sel string) (string, error) {
	sel = normalizeSelector(sel)
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
