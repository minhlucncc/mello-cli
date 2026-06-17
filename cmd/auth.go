package cmd

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/minhlucncc/mello-cli/internal/config"
	"github.com/minhlucncc/mello-cli/internal/mello"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

// decodeJWT extracts sub/email/name from a JWT's payload (no signature check).
// Returns empty strings if tok is not a JWT.
func decodeJWT(tok string) (sub, email, name string) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", "", ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		if payload, err = base64.StdEncoding.DecodeString(parts[1]); err != nil {
			return "", "", ""
		}
	}
	var m map[string]any
	if json.Unmarshal(payload, &m) != nil {
		return "", "", ""
	}
	get := func(k string) string { s, _ := m[k].(string); return s }
	return get("sub"), get("email"), get("name")
}

// jwtExp returns the token's exp (unix seconds), or 0 if absent/not a JWT.
func jwtExp(tok string) int64 {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return 0
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		if payload, err = base64.StdEncoding.DecodeString(parts[1]); err != nil {
			return 0
		}
	}
	var m map[string]any
	if json.Unmarshal(payload, &m) != nil {
		return 0
	}
	if f, ok := m["exp"].(float64); ok {
		return int64(f)
	}
	return 0
}

// internalBase returns the internal API base for refresh: --base-url,
// MELLO_BASE_URL, else the production internal API.
func internalBase(c *common) string {
	if c.baseURL != "" {
		return strings.TrimRight(c.baseURL, "/")
	}
	if e := os.Getenv("MELLO_BASE_URL"); e != "" {
		return strings.TrimRight(e, "/")
	}
	return "https://mello.mezon.vn/api"
}

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
	token := fs.String("token", "", "Mello PAT (mello_pat_…) or session JWT; omit to be prompted")
	withToken := fs.Bool("with-token", false, "read the token from stdin (CI-friendly)")
	refreshFlag := fs.String("refresh-token", "", "session refresh token; exchanged for an access token and auto-renewed")
	if err := parse(fs, c, args); err != nil {
		return err
	}

	r, err := c.resolveConfig()
	if err != nil {
		return err
	}

	cx, cancel := ctx()
	defer cancel()

	// Determine the access token: exchange a refresh token, or take --token /
	// --with-token / an interactive prompt.
	tok := *token
	refreshTok := ""
	switch {
	case *refreshFlag != "":
		access, newRT, rerr := mello.RefreshSession(cx, internalBase(c), *refreshFlag, 30*time.Second)
		if rerr != nil {
			// Don't wrap the APIError, so the message isn't flattened to the
			// generic "unauthorized" text — be specific about refresh tokens.
			return errors.New("refresh-token login failed: that token is expired or already used " +
				"(refresh tokens are single-use and rotate). Get the current value from the browser: " +
				"DevTools → Application → Cookies → mello.mezon.vn → refresh_token")
		}
		tok = access
		refreshTok = newRT
	case *withToken:
		s, err := ui.ReadAllStdin()
		if err != nil {
			return err
		}
		tok = strings.TrimSpace(s)
	case tok == "":
		s, err := ui.PromptSecret("Paste your Mello token (mello_pat_… or session JWT): ")
		if err != nil {
			return err
		}
		tok = strings.TrimSpace(s)
	}
	if tok == "" {
		return errors.New("no token provided")
	}

	// Pick the backend from the token type unless the base URL was set
	// explicitly: a mello_pat_ key uses the public API (/api/v1); a session JWT
	// uses the internal API (/api).
	base := r.BaseURL
	if c.baseURL == "" && os.Getenv("MELLO_BASE_URL") == "" {
		if strings.HasPrefix(tok, "mello_pat_") {
			base = "https://mello.mezon.vn/api/v1"
		} else {
			base = "https://mello.mezon.vn/api"
		}
	}

	cl := mello.NewClient(base, tok, 30*time.Second)

	// Identity + user id: decode a JWT directly, else try /me.
	identity, userID := "", ""
	if sub, email, name := decodeJWT(tok); sub != "" {
		userID = sub
		switch {
		case name != "" && email != "":
			identity = fmt.Sprintf("%s <%s>", name, email)
		default:
			identity = firstNonEmpty(name, email)
		}
	} else if u, err := cl.GetMe(cx); err == nil {
		identity = userLabel(u)
		userID = u.ID
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

	// From just the token, make the common single-workspace case turnkey.
	ws := r.WorkspaceID
	if ws == "" && len(workspaces) == 1 {
		ws = workspaces[0].ID
	}

	if err := config.SetProfile(r.Profile, base, tok, ws, true); err != nil {
		return err
	}
	if userID != "" {
		_ = config.SetUserID(r.Profile, userID)
	}
	if refreshTok != "" {
		_ = config.SetTokens(r.Profile, "", refreshTok)
	}
	ui.Successf("Logged in to %s as %s", ui.Bold(base), ui.Bold(identity))
	if refreshTok != "" {
		fmt.Println(ui.Dim("session will auto-renew using the refresh token"))
	}
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
