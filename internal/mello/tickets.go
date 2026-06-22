package mello

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetTicket fetches one ticket by id (GET /tickets/{id}).
func (c *Client) GetTicket(ctx context.Context, ticketID string) (Ticket, error) {
	var out Ticket
	path := fmt.Sprintf("/tickets/%s", url.PathEscape(ticketID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return Ticket{}, err
	}
	return out, nil
}

// GetTicketRaw fetches a ticket as the server's raw JSON, preserving every field
// (including any the typed model doesn't map).
func (c *Client) GetTicketRaw(ctx context.Context, ticketID string) (json.RawMessage, error) {
	var out json.RawMessage
	path := fmt.Sprintf("/tickets/%s", url.PathEscape(ticketID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateTicket creates a ticket in a column (POST /columns/{id}/tickets).
//
// description is the plain-text body; descriptionHTML is the rich body the
// web UI renders (e.g. Markdown → HTML). Either or both may be empty; the
// server stores whatever is sent and auto-derives the plain text from the
// HTML when only HTML is provided.
func (c *Client) CreateTicket(ctx context.Context, columnID, title, description, descriptionHTML string) (Ticket, error) {
	body := map[string]any{"title": title}
	if description != "" {
		body["description"] = description
	}
	if descriptionHTML != "" {
		body["description_html"] = descriptionHTML
	}
	var out Ticket
	path := fmt.Sprintf("/columns/%s/tickets", url.PathEscape(columnID))
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return Ticket{}, err
	}
	return out, nil
}

// MoveTicket relocates a ticket to a column + position
// (PATCH /tickets/{id}/move with {column_id, position}) — same on both backends.
func (c *Client) MoveTicket(ctx context.Context, ticketID, columnID string, position int) (Ticket, error) {
	body := map[string]any{"column_id": columnID, "position": position}
	var out Ticket
	path := fmt.Sprintf("/tickets/%s/move", url.PathEscape(ticketID))
	if err := c.do(ctx, http.MethodPatch, path, body, &out); err != nil {
		return Ticket{}, err
	}
	return out, nil
}

// TicketUpdate carries the editable fields for UpdateTicket. Only non-nil fields
// are sent, so callers patch exactly what changed.
type TicketUpdate struct {
	Title          *string
	Description    *string
	DescriptionHTML *string
	Status         *string
	AssigneeID     *string
	Labels         *[]string
}

func (u TicketUpdate) empty() bool {
	return u.Title == nil && u.Description == nil && u.DescriptionHTML == nil &&
		u.Status == nil && u.AssigneeID == nil && u.Labels == nil
}

func (u TicketUpdate) body() map[string]any {
	b := map[string]any{}
	if u.Title != nil {
		b["title"] = *u.Title
	}
	if u.Description != nil {
		b["description"] = *u.Description
	}
	if u.DescriptionHTML != nil {
		b["description_html"] = *u.DescriptionHTML
	} else if u.Description != nil {
		// Auto-render Markdown to HTML for the description_html field.
		html := MarkdownToHTML(*u.Description)
		if html != "" {
			b["description_html"] = html
		}
	}
	if u.Status != nil {
		b["status"] = *u.Status
	}
	if u.AssigneeID != nil {
		b["assignee_id"] = *u.AssigneeID
	}
	if u.Labels != nil {
		b["labels"] = *u.Labels
	}
	return b
}

// UpdateTicket edits ticket fields (PATCH /tickets/{id}). Optional endpoint: on
// a 404 the caller should fall back (see mello.IsNotFound). Returns the updated
// ticket. A no-op update returns the current ticket without a request.
func (c *Client) UpdateTicket(ctx context.Context, ticketID string, upd TicketUpdate) (Ticket, error) {
	if upd.empty() {
		return c.GetTicket(ctx, ticketID)
	}
	var out Ticket
	path := fmt.Sprintf("/tickets/%s", url.PathEscape(ticketID))
	if err := c.do(ctx, http.MethodPatch, path, upd.body(), &out); err != nil {
		return Ticket{}, err
	}
	return out, nil
}

// DeleteTicket deletes a ticket (DELETE /tickets/{id}). Optional endpoint.
func (c *Client) DeleteTicket(ctx context.Context, ticketID string) error {
	path := fmt.Sprintf("/tickets/%s", url.PathEscape(ticketID))
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// ListTickets enumerates a workspace's tickets oldest-first for incremental
// sync. updatedAfter (RFC3339, optional) restricts to tickets changed since the
// last cursor. Pages are followed until a short page is returned
// (GET /workspaces/{id}/tickets?page&limit&sort&updated_after).
func (c *Client) ListTickets(ctx context.Context, workspaceID, updatedAfter string) ([]Ticket, error) {
	const limit = 100
	var all []Ticket
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("limit", fmt.Sprintf("%d", limit))
		q.Set("sort", "updated_at:asc")
		if updatedAfter != "" {
			q.Set("updated_after", updatedAfter)
		}
		var batch []Ticket
		path := fmt.Sprintf("/workspaces/%s/tickets?%s", url.PathEscape(workspaceID), q.Encode())
		if err := c.do(ctx, http.MethodGet, path, nil, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < limit {
			break
		}
	}
	return all, nil
}

// GetTicketHistory returns a ticket's activity log (GET /tickets/{id}/history).
// Optional endpoint.
func (c *Client) GetTicketHistory(ctx context.Context, ticketID string) ([]HistoryEntry, error) {
	var out []HistoryEntry
	path := fmt.Sprintf("/tickets/%s/history", url.PathEscape(ticketID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MarkdownToHTML converts simple Markdown to HTML for the description_html field.
// Handles bold, italic, lists, headings, links, images, and horizontal rules.
// When both caller-supplied DescriptionHTML and auto-rendered HTML are available,
// the caller-supplied value takes precedence.
func MarkdownToHTML(md string) string {
	if md == "" {
		return ""
	}
	lines := strings.Split(md, "\n")
	var b strings.Builder
	inList := false
	pOpen := false

	closeP := func() {
		if pOpen {
			b.WriteString("</p>\n")
			pOpen = false
		}
	}
	openP := func() {
		if !pOpen {
			if inList {
				b.WriteString("</ul>\n")
				inList = false
			}
			b.WriteString("<p>")
			pOpen = true
		}
	}
	closeList := func() {
		if inList {
			closeP()
			b.WriteString("</ul>\n")
			inList = false
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			closeP()
			closeList()
			continue
		}
		if trimmed == "---" {
			closeP()
			closeList()
			b.WriteString("<hr>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			closeP()
			closeList()
			b.WriteString("<h3>")
			b.WriteString(renderInline(trimmed[4:]))
			b.WriteString("</h3>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			closeP()
			closeList()
			b.WriteString("<h2>")
			b.WriteString(renderInline(trimmed[3:]))
			b.WriteString("</h2>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			closeP()
			closeList()
			b.WriteString("<h1>")
			b.WriteString(renderInline(trimmed[2:]))
			b.WriteString("</h1>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			closeP()
			if !inList {
				b.WriteString("<ul>\n")
				inList = true
			}
			content := strings.TrimPrefix(trimmed, "- ")
			content = strings.TrimPrefix(content, "* ")
			b.WriteString("<li>")
			b.WriteString(renderInline(content))
			b.WriteString("</li>\n")
			continue
		}
		closeList()
		if !pOpen {
			openP()
		} else {
			b.WriteString("<br>\n")
		}
		b.WriteString(renderInline(trimmed))
		if i == len(lines)-1 || strings.TrimSpace(lines[i+1]) == "" {
			closeP()
		}
	}
	closeP()
	closeList()
	return strings.TrimSpace(b.String())
}

func renderInline(text string) string {
	text = replacePairs(text, "**", "<strong>", "**", "</strong>")
	text = replacePairs(text, "__", "<strong>", "__", "</strong>")
	text = replacePairsStar(text)
	text = replacePairs(text, "`", "<code>", "`", "</code>")
	text = replaceImg(text)
	text = replaceLinks(text)
	return text
}

func replacePairs(s, openDelim, openTag, closeDelim, closeTag string) string {
	var b strings.Builder
	for {
		oi := strings.Index(s, openDelim)
		if oi < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:oi])
		rest := s[oi+len(openDelim):]
		ci := strings.Index(rest, closeDelim)
		if ci < 0 {
			b.WriteString(s[oi:])
			return b.String()
		}
		b.WriteString(openTag)
		b.WriteString(rest[:ci])
		b.WriteString(closeTag)
		s = rest[ci+len(closeDelim):]
	}
}

func replacePairsStar(s string) string {
	var b strings.Builder
	for {
		oi := strings.Index(s, "*")
		if oi < 0 {
			b.WriteString(s)
			return b.String()
		}
		if len(s) > oi+1 && s[oi+1] == '*' {
			b.WriteString(s[:oi+1])
			s = s[oi+1:]
			continue
		}
		b.WriteString(s[:oi])
		rest := s[oi+1:]
		ci := strings.Index(rest, "*")
		if ci < 0 || (len(rest) > ci+1 && rest[ci+1] == '*') {
			b.WriteString(s[oi : oi+1])
			s = rest
			continue
		}
		b.WriteString("<em>")
		b.WriteString(rest[:ci])
		b.WriteString("</em>")
		s = rest[ci+1:]
	}
}

func replaceLinks(s string) string {
	var b strings.Builder
	for {
		oi := strings.Index(s, "[")
		if oi < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:oi])
		rest := s[oi+1:]
		ci := strings.Index(rest, "](")
		if ci < 0 {
			b.WriteString(s[oi:])
			return b.String()
		}
		text := rest[:ci]
		afterParen := rest[ci+2:]
		pi := strings.Index(afterParen, ")")
		if pi < 0 {
			b.WriteString(s[oi:])
			return b.String()
		}
		url := afterParen[:pi]
		b.WriteString("<a href=\"")
		b.WriteString(htmlEscape(url))
		b.WriteString("\">")
		b.WriteString(text)
		b.WriteString("</a>")
		s = afterParen[pi+1:]
	}
}

func replaceImg(s string) string {
	var b strings.Builder
	for {
		oi := strings.Index(s, "![")
		if oi < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:oi])
		rest := s[oi+2:]
		ci := strings.Index(rest, "](")
		if ci < 0 {
			b.WriteString(s[oi:])
			return b.String()
		}
		alt := rest[:ci]
		afterParen := rest[ci+2:]
		pi := strings.Index(afterParen, ")")
		if pi < 0 {
			b.WriteString(s[oi:])
			return b.String()
		}
		src := afterParen[:pi]
		b.WriteString("<img alt=\"")
		b.WriteString(htmlEscape(alt))
		b.WriteString("\" src=\"")
		b.WriteString(htmlEscape(src))
		b.WriteString("\">")
		s = afterParen[pi+1:]
	}
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
