package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/mello"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

func ticketCmd() *Command {
	return &Command{
		Name:  "ticket",
		Short: "List, view, create, edit, move, and delete tickets.",
		Subs: []*Command{
			{Name: "list", Short: "List tickets on a board.", Run: ticketList},
			{Name: "view", Short: "Show a ticket with comments.", Run: ticketView},
			{Name: "create", Short: "Create a ticket in a column.", Run: ticketCreate},
			{Name: "edit", Short: "Edit ticket fields (PATCH).", Run: ticketEdit},
			{Name: "move", Short: "Move a ticket to a column/position.", Run: ticketMove},
			{Name: "delete", Short: "Delete a ticket.", Run: ticketDelete},
			{Name: "history", Short: "Show a ticket's activity history.", Run: ticketHistory},
		},
	}
}

// ticketRef returns the human reference for a ticket (code if present, else id).
func ticketRef(t mello.Ticket) string {
	if t.TicketCode != "" {
		return t.TicketCode
	}
	return shortID(t.ID)
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func ticketList(args []string) error {
	fs, c := newFlags("ticket list")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	colFilter := fs.String("column", "", "filter by column name or id")
	assignee := fs.String("assignee", "", "filter by assignee (\"me\" for yourself)")
	mine := fs.Bool("mine", false, "only tickets assigned to you (alias for --assignee me)")
	statusFilter := fs.String("status", "", "filter by status")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()

	assigneeSel := *assignee
	if *mine {
		assigneeSel = "me"
	}
	assigneeID, err := resolveAssignee(cx, cl, c, assigneeSel)
	if err != nil {
		return err
	}

	boardID, _, err := resolveBoardID(cx, cl, *board)
	if err != nil {
		return err
	}
	cols, err := cachedColumns(cx, cl, boardID)
	if err != nil {
		return err
	}
	idToName := map[string]string{}
	for _, col := range cols {
		idToName[col.ID] = col.Name
	}

	// Prefer the workspace tickets endpoint, which returns full ticket records
	// (status, members). Fall back to the columns summary when unavailable.
	var tickets []mello.Ticket
	nameByID := map[string]string{}
	enriched := false
	if wsID, _, ok := currentWorkspace(); ok {
		if all, lerr := cl.ListTickets(cx, wsID, ""); lerr == nil {
			enriched = true
			for _, t := range all {
				if t.BoardID == boardID {
					tickets = append(tickets, t)
				}
			}
		} else if !mello.IsNotFound(lerr) {
			return lerr
		}
		nameByID = memberNames(cx, cl, wsID)
	}
	if !enriched {
		for _, col := range cols {
			for _, t := range col.Tickets {
				if t.ColumnID == "" {
					t.ColumnID = col.ID
				}
				tickets = append(tickets, t)
			}
		}
	}

	type row struct {
		t   mello.Ticket
		col string
	}
	var collected []row
	for _, t := range tickets {
		colN := idToName[t.ColumnID]
		if *colFilter != "" && colN != *colFilter && t.ColumnID != *colFilter {
			continue
		}
		if assigneeID != "" && !t.HasMember(assigneeID) {
			continue
		}
		if *statusFilter != "" && !strings.EqualFold(t.Status, *statusFilter) {
			continue
		}
		collected = append(collected, row{t: t, col: colN})
	}

	if c.json {
		out := make([]mello.Ticket, 0, len(collected))
		for _, r := range collected {
			out = append(out, r.t)
		}
		return ui.JSON(out)
	}
	rows := make([][]string, 0, len(collected))
	for _, r := range collected {
		rows = append(rows, []string{
			ticketRef(r.t), r.col, membersDisplay(r.t, nameByID),
			emptyDash(r.t.Status), ui.Truncate(r.t.Title, 50),
		})
	}
	ui.Table([]string{"ticket", "column", "members", "status", "title"}, rows)
	if len(rows) == 0 {
		fmt.Println(ui.Dim("no tickets"))
	}
	return nil
}

func ticketView(args []string) error {
	fs, c := newFlags("ticket view")
	noComments := fs.Bool("no-comments", false, "skip fetching comments")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello ticket view <id|code>")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	id := resolveTicketID(cx, cl, fs.Arg(0))

	// --json dumps the raw server payload, so every field is visible.
	if c.json {
		raw, err := cl.GetTicketRaw(cx, id)
		if err != nil {
			return err
		}
		comments, _ := cl.ListComments(cx, id)
		return ui.JSON(struct {
			Ticket   json.RawMessage `json:"ticket"`
			Comments []mello.Comment `json:"comments"`
		}{raw, comments})
	}

	t, err := cl.GetTicket(cx, id)
	if err != nil {
		return err
	}

	// Resolve the column name when the board is known.
	colName := t.ColumnID
	if t.BoardID != "" {
		if cols, cerr := cachedColumns(cx, cl, t.BoardID); cerr == nil {
			for _, col := range cols {
				if col.ID == t.ColumnID {
					colName = col.Name
					break
				}
			}
		}
	}

	nameByID := map[string]string{}
	if wsID, _, ok := currentWorkspace(); ok {
		nameByID = memberNames(cx, cl, wsID)
	}

	fmt.Printf("%s  %s\n\n", ui.Bold(ticketRef(t)), t.Title)
	field("Status", emptyDash(t.Status))
	field("Members", membersDisplay(t, nameByID))
	field("Labels", emptyDash(strings.Join(t.Labels, ", ")))
	field("Column", emptyDash(colName))
	if t.BoardID != "" {
		field("Board", t.BoardID)
	}
	if t.CreatedAt != nil {
		field("Created", t.CreatedAt.Format("2006-01-02 15:04"))
	}
	if t.UpdatedAt != nil {
		field("Updated", t.UpdatedAt.Format("2006-01-02 15:04"))
	}
	field("ID", t.ID)
	if t.Description != "" {
		fmt.Printf("\n%s\n", t.Description)
	}

	if !*noComments {
		if comments, err := cl.ListComments(cx, id); err == nil {
			if len(comments) > 0 {
				fmt.Printf("\n%s\n", ui.Bold(fmt.Sprintf("Comments (%d)", len(comments))))
				for _, cm := range comments {
					when := ""
					if cm.CreatedAt != nil {
						when = cm.CreatedAt.Format("2006-01-02 15:04")
					}
					fmt.Printf("  %s %s\n  %s\n", ui.Dim(emptyDash(cm.AuthorID)), ui.Dim(when), cm.Body)
				}
			}
		} else if !mello.IsNotFound(err) {
			return err
		}
	}

	if atts, err := cl.ListAttachments(cx, id); err == nil && len(atts) > 0 {
		fmt.Printf("\n%s\n", ui.Bold(fmt.Sprintf("Attachments (%d)", len(atts))))
		for _, a := range atts {
			fmt.Printf("  %s\n", a.FileName())
		}
	}
	return nil
}

// field prints an aligned "Label: value" line.
func field(label, val string) {
	fmt.Printf("%s %s\n", ui.Dim(fmt.Sprintf("%-9s", label+":")), val)
}

func ticketCreate(args []string) error {
	fs, c := newFlags("ticket create")
	board := fs.String("board", "", "board (default the working board)")
	fs.StringVar(board, "b", "", "board (shorthand)")
	column := fs.String("column", "", "column name (default the first column)")
	fs.StringVar(column, "c", "", "column (shorthand)")
	title := fs.String("title", "", "ticket title")
	fs.StringVar(title, "t", "", "ticket title (shorthand)")
	desc := fs.String("description", "", "ticket description")
	fs.StringVar(desc, "d", "", "ticket description (shorthand)")
	descFile := fs.String("body-file", "", "read description from a file")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if *title == "" {
		return fmt.Errorf("usage: mello ticket create -t <title> [-b board] [-c column] [-d desc]")
	}
	body := *desc
	if body == "" && *descFile != "" {
		b, err := bodyFrom("", *descFile)
		if err != nil {
			return err
		}
		body = b
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	boardID, _, err := resolveBoardID(cx, cl, *board)
	if err != nil {
		return err
	}
	cols, err := cl.ListColumns(cx, boardID)
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return fmt.Errorf("board has no columns — create one with `mello column create <name>`")
	}
	colID := cols[0].ID
	if *column != "" {
		colID = ""
		for _, cc := range cols {
			if cc.ID == *column || cc.Name == *column || strings.EqualFold(cc.Name, *column) {
				colID = cc.ID
				break
			}
		}
		if colID == "" {
			return fmt.Errorf("no column %q on this board (see `mello column list`)", *column)
		}
	}
	t, err := cl.CreateTicket(cx, colID, *title, body)
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(t)
	}
	ui.Successf("Created ticket %s — %s", ui.Bold(ticketRef(t)), t.Title)
	return nil
}

func ticketEdit(args []string) error {
	fs, c := newFlags("ticket edit")
	title := fs.String("title", "", "new title")
	fs.StringVar(title, "t", "", "new title (shorthand)")
	desc := fs.String("description", "", "new description")
	fs.StringVar(desc, "d", "", "new description (shorthand)")
	descFile := fs.String("body-file", "", "read description from a file")
	status := fs.String("status", "", "new status")
	assignee := fs.String("assignee", "", "new assignee (\"me\" for yourself)")
	labels := fs.String("labels", "", "comma-separated labels (replaces existing)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello ticket edit <id> [-t][-d][--status][--assignee][--labels]")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()

	upd := mello.TicketUpdate{}
	if isSet(fs, "title", "t") {
		upd.Title = title
	}
	if isSet(fs, "description", "d") {
		upd.Description = desc
	} else if *descFile != "" {
		b, err := bodyFrom("", *descFile)
		if err != nil {
			return err
		}
		upd.Description = &b
	}
	if isSet(fs, "status") {
		upd.Status = status
	}
	if isSet(fs, "assignee") {
		a, err := resolveAssignee(cx, cl, c, *assignee)
		if err != nil {
			return err
		}
		upd.AssigneeID = &a
	}
	if isSet(fs, "labels") {
		l := splitCSV(*labels)
		upd.Labels = &l
	}

	t, err := cl.UpdateTicket(cx, fs.Arg(0), upd)
	if err != nil {
		if mello.IsNotFound(err) {
			return fmt.Errorf("ticket edit not supported by this Mello instance (PATCH /tickets/{id} → 404)")
		}
		return err
	}
	if c.json {
		return ui.JSON(t)
	}
	ui.Successf("Updated ticket %s", ui.Bold(ticketRef(t)))
	return nil
}

func ticketMove(args []string) error {
	fs, c := newFlags("ticket move")
	column := fs.String("column", "", "destination column id")
	fs.StringVar(column, "c", "", "destination column id (shorthand)")
	position := fs.Int("position", 0, "position within the column")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 || *column == "" {
		return fmt.Errorf("usage: mello ticket move <id> --column <col> [--position N]")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	t, err := cl.MoveTicket(cx, fs.Arg(0), *column, *position)
	if err != nil {
		return err
	}
	if c.json {
		return ui.JSON(t)
	}
	ui.Successf("Moved ticket %s", ui.Bold(ticketRef(t)))
	return nil
}

func ticketDelete(args []string) error {
	fs, c := newFlags("ticket delete")
	yes := fs.Bool("yes", false, "skip confirmation")
	fs.BoolVar(yes, "y", false, "skip confirmation (shorthand)")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello ticket delete <id> [-y]")
	}
	if !*yes {
		ans, _ := ui.PromptSecret(fmt.Sprintf("Delete ticket %s? type 'yes': ", fs.Arg(0)))
		if strings.TrimSpace(ans) != "yes" {
			return fmt.Errorf("aborted")
		}
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	if err := cl.DeleteTicket(cx, fs.Arg(0)); err != nil {
		if mello.IsNotFound(err) {
			return fmt.Errorf("ticket delete not supported by this Mello instance (DELETE /tickets/{id} → 404)")
		}
		return err
	}
	ui.Successf("Deleted ticket %s", fs.Arg(0))
	return nil
}

func ticketHistory(args []string) error {
	fs, c := newFlags("ticket history")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: mello ticket history <id>")
	}
	cl, _, err := c.client()
	if err != nil {
		return err
	}
	cx, cancel := ctx()
	defer cancel()
	hist, err := cl.GetTicketHistory(cx, fs.Arg(0))
	if err != nil {
		if mello.IsNotFound(err) {
			return fmt.Errorf("ticket history not supported by this Mello instance (GET /tickets/{id}/history → 404)")
		}
		return err
	}
	if c.json {
		return ui.JSON(hist)
	}
	rows := make([][]string, 0, len(hist))
	for _, h := range hist {
		when := ""
		if h.CreatedAt != nil {
			when = h.CreatedAt.Format("2006-01-02 15:04")
		}
		actor := h.ActorName
		if actor == "" {
			actor = h.ActorID
		}
		rows = append(rows, []string{when, h.Type, actor})
	}
	ui.Table([]string{"when", "event", "actor"}, rows)
	return nil
}

// ---- helpers ----------------------------------------------------------------

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// membersDisplay joins a ticket's assignees, preferring inline names, then the
// cached member-name map, then a short id.
func membersDisplay(t mello.Ticket, names map[string]string) string {
	ms := t.AssigneeMembers()
	if len(ms) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ms))
	for _, m := range ms {
		n := m.Name
		if n == "" {
			n = names[m.ID]
		}
		if n == "" {
			n = ui.Truncate(m.ID, 12)
		}
		parts = append(parts, n)
	}
	return strings.Join(parts, ", ")
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// isSet reports whether any of the given flag names was explicitly set on the
// command line (so we only PATCH the fields the user actually passed).
func isSet(fs *flag.FlagSet, names ...string) bool {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	found := false
	fs.Visit(func(f *flag.Flag) {
		if want[f.Name] {
			found = true
		}
	})
	return found
}
