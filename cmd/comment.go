package cmd

import (
	"fmt"

	"github.com/minhlucncc/mello-cli/internal/mello"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func commentCmd() *Command {
	return &Command{
		Name:  "comment",
		Short: "List and add ticket comments.",
		Subs: []*Command{
			{Name: "list", Short: "List a ticket's comments.", Run: commentList},
			{Name: "add", Short: "Add a comment to a ticket.", Run: commentAdd},
		},
	}
}

func commentList(args []string) error {
	fs, c := newFlags("comment list")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello comment list <ticket>")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	comments, err := cl.ListComments(cx, fs.Arg(0))
	if err != nil {
		if mello.IsNotFound(err) {
			return fmt.Errorf("reading comments not supported by this Mello instance (GET /tickets/{id}/comments → 404)")
		}
		return err
	}
	if c.json {
		return ui.JSON(comments)
	}
	for _, cm := range comments {
		when := ""
		if cm.CreatedAt != nil {
			when = cm.CreatedAt.Format("2006-01-02 15:04")
		}
		fmt.Printf("%s %s\n%s\n\n", ui.Bold(cm.AuthorID), ui.Dim(when), cm.Body)
	}
	if len(comments) == 0 {
		fmt.Println(ui.Dim("no comments"))
	}
	return nil
}

func commentAdd(args []string) error {
	fs, c := newFlags("comment add")
	body := fs.String("body", "", "comment body (markdown)")
	fs.StringVar(body, "b", "", "comment body (shorthand)")
	bodyFile := fs.String("body-file", "", "read the body from a file")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello comment add <ticket> [-b <body>|--body-file F| (stdin)]")
	}
	text, err := bodyFrom(*body, *bodyFile)
	if err != nil {
		return err
	}
	if text == "" {
		return fmt.Errorf("empty comment body")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	cm, err := cl.AddComment(cx, fs.Arg(0), text, "")
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(cm)
	}
	ui.Successf("Comment added to ticket %s", fs.Arg(0))
	return nil
}
