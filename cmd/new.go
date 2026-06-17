package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minhlucncc/mello-cli/internal/mello"
	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func newCmd() *Command {
	return &Command{
		Name:  "new",
		Short: "Create local objects in the workspace (push to create remotely).",
		Subs: []*Command{
			{Name: "ticket", Short: "Scaffold a new local ticket.", Run: newTicket},
		},
	}
}

func newTicket(args []string) error {
	fs, c := newFlags("new ticket")
	dir := fs.String("dir", ".", "workspace directory")
	board := fs.String("board", "", "target board (id, code, name, or slug)")
	fs.StringVar(board, "b", "", "target board (shorthand)")
	column := fs.String("column", "", "target column NAME (e.g. \"Todo\")")
	fs.StringVar(column, "c", "", "target column name (shorthand)")
	title := fs.String("title", "", "ticket title")
	fs.StringVar(title, "t", "", "ticket title (shorthand)")
	desc := fs.String("description", "", "ticket description")
	fs.StringVar(desc, "d", "", "ticket description (shorthand)")
	descFile := fs.String("body-file", "", "read description from a file")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if *title == "" || *column == "" {
		return fmt.Errorf("usage: mello new ticket --column <name> -t <title> [-b board] [-d desc|--body-file f]")
	}
	body := *desc
	if body == "" && *descFile != "" {
		b, err := bodyFrom("", *descFile)
		if err != nil {
			return err
		}
		body = b
	}

	tree, err := syncpkg.Open(*dir)
	if err != nil {
		return err
	}
	bs, err := tree.ResolveBoard(*board)
	if err != nil {
		return err
	}
	slug := tree.UniqueSlug(bs, *title)
	tdir := tree.TicketPath(bs.Slug, slug)
	if err := os.MkdirAll(tdir, 0o755); err != nil {
		return err
	}
	md := syncpkg.RenderTicket(mello.Ticket{Title: *title, Description: body, Status: "open"}, *column)
	if err := os.WriteFile(filepath.Join(tdir, "ticket.md"), md, 0o644); err != nil {
		return err
	}
	ui.Successf("New local ticket %s on board %s — edit %s then `mello sync push`",
		ui.Bold(slug), ui.Bold(bs.Name), filepath.Join(tdir, "ticket.md"))
	return nil
}
