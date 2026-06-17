package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minhlucncc/mello-cli/internal/mello"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func attachmentCmd() *Command {
	return &Command{
		Name:  "attachment",
		Short: "List, upload, and download ticket attachments.",
		Subs: []*Command{
			{Name: "list", Short: "List a ticket's attachments.", Run: attachmentList},
			{Name: "add", Short: "Upload one or more files to a ticket.", Run: attachmentAdd},
			{Name: "download", Short: "Download a ticket's attachments.", Run: attachmentDownload},
		},
	}
}

const attachUnsupported = "attachments not supported by this Mello instance (/tickets/{id}/attachments → 404)"

func attachmentList(args []string) error {
	fs, c := newFlags("attachment list")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello attachment list <ticket>")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	atts, err := ticketAttachments(cx, cl, fs.Arg(0))
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(atts)
	}
	rows := make([][]string, 0, len(atts))
	for _, a := range atts {
		rows = append(rows, []string{a.ID, a.FileName(), fmt.Sprintf("%d", a.Size)})
	}
	ui.Table([]string{"id", "filename", "bytes"}, rows)
	if len(rows) == 0 {
		fmt.Println(ui.Dim("no attachments"))
	}
	return nil
}

// ticketAttachments lists a ticket's attachments via the endpoint, falling back
// to those embedded in the ticket payload when the endpoint isn't supported.
func ticketAttachments(cx context.Context, cl *mello.Client, sel string) ([]mello.Attachment, error) {
	id := resolveTicketID(cx, cl, sel)
	if atts, err := cl.ListAttachments(cx, id); err == nil {
		if len(atts) > 0 {
			return atts, nil
		}
	} else if !mello.IsNotFound(err) {
		return nil, err
	}
	t, err := cl.GetTicket(cx, id)
	if err != nil {
		return nil, err
	}
	return t.AttachmentList(), nil
}

func attachmentAdd(args []string) error {
	fs, c := newFlags("attachment add")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: mello attachment add <ticket> <file>...")
	}
	ticket := fs.Arg(0)
	files := fs.Args()[1:]

	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	for _, f := range files {
		a, err := cl.UploadAttachment(cx, ticket, f)
		if err != nil {
			if mello.IsNotFound(err) {
				return fmt.Errorf(attachUnsupported)
			}
			return fmt.Errorf("upload %s: %w", f, err)
		}
		ui.Successf("Uploaded %s%s", filepath.Base(f), idSuffix(a.ID))
	}
	return nil
}

func attachmentDownload(args []string) error {
	fs, c := newFlags("attachment download")
	dir := fs.String("dir", ".", "destination directory")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello attachment download <ticket> [--dir D]")
	}
	ticket := fs.Arg(0)
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	atts, err := cl.ListAttachments(cx, ticket)
	if err != nil {
		if mello.IsNotFound(err) {
			return fmt.Errorf(attachUnsupported)
		}
		return err
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return err
	}
	for _, a := range atts {
		dest := filepath.Join(*dir, a.FileName())
		f, err := os.Create(dest)
		if err != nil {
			return err
		}
		err = cl.DownloadAttachment(cx, ticket, a, f)
		cerr := f.Close()
		if err != nil {
			return fmt.Errorf("download %s: %w", a.FileName(), err)
		}
		if cerr != nil {
			return cerr
		}
		ui.Successf("Downloaded %s", dest)
	}
	if len(atts) == 0 {
		fmt.Println(ui.Dim("no attachments"))
	}
	return nil
}

func idSuffix(id string) string {
	if id == "" {
		return ""
	}
	return " (" + id + ")"
}
