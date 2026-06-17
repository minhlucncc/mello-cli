package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/minhlucncc/mello-cli/internal/config"
	"github.com/minhlucncc/mello-cli/internal/mello"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func authCmd() *Command {
	return &Command{
		Name:  "auth",
		Short: "Log in, log out, and check your Mello identity.",
		Subs: []*Command{
			{Name: "login", Short: "Store a Mello PAT for a profile.", Run: authLogin},
			{Name: "logout", Short: "Remove the active profile's token.", Run: authLogout},
			{Name: "status", Short: "Show the current identity (whoami).", Run: authStatus},
		},
	}
}

func authLogin(args []string) error {
	fs, c := newFlags("auth login")
	token := fs.String("token", "", "Mello PAT (mello_pat_…); omit to be prompted")
	withToken := fs.Bool("with-token", false, "read the token from stdin (CI-friendly)")
	if err := parse(fs, c, args); err != nil {
		return err
	}

	r, err := c.resolveConfig()
	if err != nil {
		return err
	}

	tok := *token
	switch {
	case *withToken:
		s, err := ui.ReadAllStdin()
		if err != nil {
			return err
		}
		tok = strings.TrimSpace(s)
	case tok == "":
		s, err := ui.PromptSecret("Paste your Mello token (mello_pat_…): ")
		if err != nil {
			return err
		}
		tok = strings.TrimSpace(s)
	}
	if tok == "" {
		return errors.New("no token provided")
	}

	// Validate the token before persisting. Prefer /me for the identity; the
	// workspace list both validates the token (when /me is unavailable) and lets
	// us auto-select a default workspace.
	cl := mello.NewClient(r.BaseURL, tok, 30*time.Second)
	cx, cancel := ctx()
	defer cancel()

	identity := ""
	if u, err := cl.GetMe(cx); err == nil {
		identity = userLabel(u)
	} else if !mello.IsNotFound(err) {
		return err
	}

	workspaces, err := cl.ListWorkspaces(cx)
	if err != nil {
		return err
	}
	if identity == "" {
		identity = fmt.Sprintf("token valid (%d workspace(s) accessible)", len(workspaces))
	}

	// From just the key, make the common single-workspace case turnkey: select
	// it as the default so workspace-scoped commands work without `workspace use`.
	ws := r.WorkspaceID
	if ws == "" && len(workspaces) == 1 {
		ws = workspaces[0].ID
	}

	if err := config.SetProfile(r.Profile, r.BaseURL, tok, ws, true); err != nil {
		return err
	}
	ui.Successf("Logged in to %s as %s", ui.Bold(r.BaseURL), ui.Bold(identity))
	if ws != "" {
		name := ws
		for _, w := range workspaces {
			if w.ID == ws {
				name = w.Name
				break
			}
		}
		fmt.Printf("Default workspace: %s\n", ui.Bold(name))
	} else if len(workspaces) > 1 {
		ui.Warnf("%d workspaces available — set one with `mello workspace use <id>`", len(workspaces))
	}
	ui.Warnf("token stored in %s (mode 0600)", config.Path())
	return nil
}

func authLogout(args []string) error {
	fs, c := newFlags("auth logout")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	name := config.ActiveProfile(c.profile)
	if err := config.ClearToken(name); err != nil {
		return err
	}
	ui.Successf("Logged out of profile %q", name)
	return nil
}

func authStatus(args []string) error {
	fs, c := newFlags("auth status")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	r, err := c.resolveConfig()
	if err != nil {
		return err
	}
	if r.Token == "" {
		ui.Warnf("not logged in (profile %q) — run `mello auth login`", r.Profile)
		return nil
	}
	cl := mello.NewClient(r.BaseURL, r.Token, 30*time.Second)
	cx, cancel := ctx()
	defer cancel()

	fmt.Printf("Profile:   %s\n", r.Profile)
	fmt.Printf("Base URL:  %s\n", r.BaseURL)
	fmt.Printf("Token:     %s\n", maskToken(r.Token))
	if r.WorkspaceID != "" {
		fmt.Printf("Workspace: %s\n", r.WorkspaceID)
	}
	if u, err := cl.GetMe(cx); err == nil {
		fmt.Printf("Identity:  %s\n", userLabel(u))
	} else if mello.IsNotFound(err) {
		// Validate via workspaces instead.
		if ws, werr := cl.ListWorkspaces(cx); werr == nil {
			fmt.Printf("Identity:  (token valid; %d workspace(s) accessible)\n", len(ws))
		} else {
			return werr
		}
	} else {
		return err
	}
	return nil
}

func userLabel(u mello.User) string {
	switch {
	case u.Email != "" && u.Name != "":
		return fmt.Sprintf("%s <%s>", u.Name, u.Email)
	case u.Email != "":
		return u.Email
	case u.Name != "":
		return u.Name
	default:
		return u.ID
	}
}

func maskToken(t string) string {
	if len(t) <= 12 {
		return "****"
	}
	return t[:10] + "…" + t[len(t)-4:]
}
