package mello

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// AddComment posts a markdown comment on a ticket (POST /tickets/{id}/comments).
func (c *Client) AddComment(ctx context.Context, ticketID, body string) (Comment, error) {
	var out Comment
	path := fmt.Sprintf("/tickets/%s/comments", url.PathEscape(ticketID))
	if err := c.do(ctx, http.MethodPost, path, map[string]any{"body": body}, &out); err != nil {
		return Comment{}, err
	}
	return out, nil
}

// ListComments returns a ticket's comments (GET /tickets/{id}/comments).
// Optional endpoint: on 404 callers fall back (see mello.IsNotFound).
func (c *Client) ListComments(ctx context.Context, ticketID string) ([]Comment, error) {
	var out []Comment
	path := fmt.Sprintf("/tickets/%s/comments", url.PathEscape(ticketID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
