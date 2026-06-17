package mello

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
func (c *Client) CreateTicket(ctx context.Context, columnID, title, description string) (Ticket, error) {
	body := map[string]any{"title": title}
	if description != "" {
		body["description"] = description
	}
	var out Ticket
	path := fmt.Sprintf("/columns/%s/tickets", url.PathEscape(columnID))
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return Ticket{}, err
	}
	return out, nil
}

// MoveTicket relocates a ticket to a column + position atomically
// (PATCH /tickets/{id}/move).
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
	Title       *string
	Description *string
	Status      *string
	AssigneeID  *string
	Labels      *[]string
}

func (u TicketUpdate) empty() bool {
	return u.Title == nil && u.Description == nil && u.Status == nil &&
		u.AssigneeID == nil && u.Labels == nil
}

func (u TicketUpdate) body() map[string]any {
	b := map[string]any{}
	if u.Title != nil {
		b["title"] = *u.Title
	}
	if u.Description != nil {
		b["description"] = *u.Description
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
