// Package cmd implements the mello CLI command tree on top of the standard
// library (no cobra). A Command is either a leaf (has Run) or a group (has Subs).
package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/minhlucncc/mello-cli/internal/config"
	"github.com/minhlucncc/mello-cli/internal/mello"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

// Build metadata, set at build time via -ldflags "-X ...cmd.Version=...".
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Command is a node in the CLI tree.
type Command struct {
	Name  string
	Short string
	Run   func(args []string) error // leaf only
	Subs  []*Command                // group only
}

// Root builds the full command tree.
func Root() *Command {
	return &Command{
		Name:  "mello",
		Short: "Command-line client for the Mello project-management platform.",
		Subs: []*Command{
			authCmd(),
			workspaceCmd(),
			boardCmd(),
			columnCmd(),
			ticketCmd(),
			commentCmd(),
			attachmentCmd(),
			memberCmd(),
			searchCmd(),
			initCmd(),
			syncCmd(),
			newCmd(),
			versionCmd(),
		},
	}
}

// Execute runs the CLI for the given args (os.Args[1:]).
func Execute(args []string) int {
	root := Root()
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-v") {
		printVersion()
		return 0
	}
	cmd, rest := resolve(root, args)
	if cmd == nil {
		printHelp(root, nil)
		return 2
	}
	if cmd.Run == nil { // group: print its help
		printHelp(root, cmd)
		if len(rest) > 0 {
			ui.Errorf("unknown command: %s %s", cmd.Name, strings.Join(rest, " "))
			return 2
		}
		return 0
	}
	if err := cmd.Run(rest); err != nil {
		reportError(err)
		return 1
	}
	return 0
}

// resolve walks the tree consuming command names; returns the matched node and
// the remaining args (flags + positionals).
func resolve(root *Command, args []string) (*Command, []string) {
	cur := root
	i := 0
	for i < len(args) {
		tok := args[i]
		if strings.HasPrefix(tok, "-") { // a flag — stop descending
			break
		}
		if tok == "help" { // `mello help` / `mello x help`
			i++
			break
		}
		next := child(cur, tok)
		if next == nil {
			break
		}
		cur = next
		i++
	}
	if cur == root {
		// No subcommand matched. If the first token was a flag or "help", show root help.
		if len(args) == 0 {
			return nil, nil
		}
		if strings.HasPrefix(args[0], "-") || args[0] == "help" {
			return nil, nil
		}
		return nil, args
	}
	return cur, args[i:]
}

func child(c *Command, name string) *Command {
	for _, s := range c.Subs {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// ---- shared flag + client plumbing -----------------------------------------

// common holds the global flags every leaf accepts.
type common struct {
	profile string
	baseURL string
	json    bool
	noColor bool
}

// newFlags returns a FlagSet pre-bound with the global flags. Leaves add their
// own flags before calling parse().
func newFlags(name string) (*flag.FlagSet, *common) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	c := &common{}
	fs.StringVar(&c.profile, "profile", "", "config profile")
	fs.StringVar(&c.profile, "p", "", "config profile (shorthand)")
	fs.StringVar(&c.baseURL, "base-url", "", "override API base URL")
	fs.BoolVar(&c.json, "json", false, "output raw JSON")
	fs.BoolVar(&c.noColor, "no-color", false, "disable ANSI color")
	return fs, c
}

// parse parses args and applies global side effects (color). Usage is printed to
// stderr on error.
func parse(fs *flag.FlagSet, c *common, args []string) error {
	if err := fs.Parse(args); err != nil {
		return errSilent{err}
	}
	if c.noColor {
		ui.NoColor = true
	}
	return nil
}

// resolveConfig returns the effective config honoring --profile / --base-url.
func (c *common) resolveConfig() (config.Resolved, error) {
	r, err := config.Resolve(c.profile)
	if err != nil {
		return config.Resolved{}, err
	}
	if c.baseURL != "" {
		r.BaseURL = strings.TrimRight(c.baseURL, "/")
	}
	return r, nil
}

// client builds an authenticated client, erroring clearly if no token is set.
func (c *common) client() (*mello.Client, config.Resolved, error) {
	r, err := c.resolveConfig()
	if err != nil {
		return nil, config.Resolved{}, err
	}
	if r.Token == "" {
		return nil, r, fmt.Errorf("not logged in — run `mello auth login` (or set MELLO_TOKEN)")
	}
	return mello.NewClient(r.BaseURL, r.Token, 30*time.Second), r, nil
}

// requireWorkspace resolves the workspace id from --workspace flag, config, or
// MELLO_WORKSPACE; errors with guidance if unset.
func requireWorkspace(flagVal string, r config.Resolved) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if r.WorkspaceID != "" {
		return r.WorkspaceID, nil
	}
	return "", fmt.Errorf("no workspace set — pass -w <id> or run `mello workspace use <id>`")
}

// ctx returns a background context with a sane timeout for one command.
func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}

// ---- help + errors ----------------------------------------------------------

func printHelp(root, group *Command) {
	if group == nil || group == root {
		fmt.Printf("%s\n\n%s\n\nUsage:\n  mello <command> [subcommand] [flags]\n\nCommands:\n", ui.Bold("mello "+Version), root.Short)
		for _, s := range root.Subs {
			fmt.Printf("  %-12s %s\n", s.Name, s.Short)
		}
		fmt.Printf("\nGlobal flags:\n  -p, --profile   config profile\n      --base-url  override API base URL\n      --json      output raw JSON\n      --no-color  disable color\n  -v, --version   print version information\n\nRun `mello <command> help` for subcommands.\n")
		return
	}
	fmt.Printf("%s — %s\n\n", ui.Bold("mello "+group.Name), group.Short)
	if len(group.Subs) > 0 {
		fmt.Println("Subcommands:")
		for _, s := range group.Subs {
			fmt.Printf("  %-12s %s\n", s.Name, s.Short)
		}
	}
}

func versionCmd() *Command {
	return &Command{Name: "version", Short: "Print version and build information.", Run: func(args []string) error {
		printVersion()
		return nil
	}}
}

func printVersion() {
	fmt.Printf("mello %s\n", Version)
	fmt.Printf("commit:  %s\n", Commit)
	fmt.Printf("built:   %s\n", Date)
	fmt.Printf("go:      %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// errSilent wraps an error whose message was already printed (e.g. flag usage).
type errSilent struct{ err error }

func (e errSilent) Error() string { return e.err.Error() }

func reportError(err error) {
	if _, ok := err.(errSilent); ok {
		return
	}
	var ae *mello.APIError
	if errors.As(err, &ae) {
		switch {
		case ae.Unauthorized():
			ui.Errorf("unauthorized — your token is missing or invalid (run `mello auth login`)")
		case ae.Forbidden():
			ui.Errorf("forbidden — your token lacks the required scope for %s", ae.Path)
		case ae.RateLimited():
			ui.Errorf("rate limited — Mello allows 100 requests / 10s per token; retry shortly")
		case ae.NotFound():
			ui.Errorf("not found: %s (this Mello instance may not implement %s)", ae.Code, ae.Path)
		default:
			ui.Errorf("%v", ae)
		}
		return
	}
	ui.Errorf("%v", err)
}

// bodyFrom reads a body from --body, --body-file, or stdin (in that order).
func bodyFrom(inline, file string) (string, error) {
	if inline != "" {
		return inline, nil
	}
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(data), "\n"), nil
	}
	return ui.ReadAllStdin()
}
