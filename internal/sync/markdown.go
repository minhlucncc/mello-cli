package sync

import (
	"fmt"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/mello"
)

// TicketDoc is the editable view of a ticket stored in ticket.md: frontmatter
// fields plus the description body. Read-only fields (ticket code, id) are kept
// for display but ignored on push.
type TicketDoc struct {
	Ticket      string // code, read-only
	ID          string // read-only
	Title       string
	Status      string
	Assignee    string
	Labels      []string
	Column      string // column NAME (resolved to id on push)
	Description string // body below frontmatter
}

// RenderTicket serializes a ticket to Markdown (frontmatter + body). columnName
// is the human column name for the ticket's column.
func RenderTicket(t mello.Ticket, columnName string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	writeKV(&b, "ticket", t.TicketCode)
	writeKV(&b, "id", t.ID)
	writeKV(&b, "title", t.Title)
	writeKV(&b, "status", t.Status)
	writeKV(&b, "assignee", t.AssigneeID)
	writeKV(&b, "column", columnName)
	b.WriteString("labels: " + renderList(t.Labels) + "\n")
	b.WriteString("---\n\n")
	b.WriteString(t.Description)
	if !strings.HasSuffix(t.Description, "\n") {
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// ParseTicket reads a ticket.md back into a TicketDoc.
func ParseTicket(data []byte) (TicketDoc, error) {
	text := string(data)
	var doc TicketDoc
	if !strings.HasPrefix(text, "---") {
		// No frontmatter: treat the whole file as description.
		doc.Description = strings.TrimRight(text, "\n")
		return doc, nil
	}
	// Split off the frontmatter block between the first two "---" lines.
	rest := strings.TrimPrefix(text, "---")
	rest = strings.TrimPrefix(rest, "\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return doc, fmt.Errorf("frontmatter not closed with '---'")
	}
	front := rest[:end]
	body := rest[end+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimPrefix(body, "\n") // drop the blank line after frontmatter

	for _, line := range strings.Split(front, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "ticket":
			doc.Ticket = unquote(val)
		case "id":
			doc.ID = unquote(val)
		case "title":
			doc.Title = unquote(val)
		case "status":
			doc.Status = unquote(val)
		case "assignee":
			doc.Assignee = unquote(val)
		case "column":
			doc.Column = unquote(val)
		case "labels":
			doc.Labels = parseList(val)
		}
	}
	doc.Description = strings.TrimRight(body, "\n")
	return doc, nil
}

func writeKV(b *strings.Builder, key, val string) {
	b.WriteString(key + ": " + quoteIfNeeded(val) + "\n")
}

// quoteIfNeeded wraps scalars that could confuse the minimal parser.
func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#\"'") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") ||
		strings.HasPrefix(s, "[") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"') {
		inner := s[1 : len(s)-1]
		return strings.ReplaceAll(inner, `\"`, `"`)
	}
	if len(s) >= 2 && (s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

// renderList prints a labels list as inline-flow YAML: [a, b, c].
func renderList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = quoteIfNeeded(it)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// parseList accepts inline "[a, b]" form.
func parseList(val string) []string {
	val = strings.TrimSpace(val)
	val = strings.TrimPrefix(val, "[")
	val = strings.TrimSuffix(val, "]")
	if strings.TrimSpace(val) == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := unquote(strings.TrimSpace(p)); v != "" {
			out = append(out, v)
		}
	}
	return out
}
