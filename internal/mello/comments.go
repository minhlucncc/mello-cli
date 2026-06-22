package mello

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// AddComment posts a comment on a ticket (POST /tickets/{id}/comments).
// body is the plain-text/markdown body; bodyHTML is the HTML-rendered version
// for rich display in the web UI. Either or both may be empty.
func (c *Client) AddComment(ctx context.Context, ticketID, body, bodyHTML string) (Comment, error) {
	var out Comment
	payload := map[string]any{}
	if body != "" {
		payload["body"] = body
	}
	if bodyHTML != "" {
		payload["body_html"] = bodyHTML
	} else if body != "" {
		// Auto-render markdown to HTML if no explicit HTML provided.
		if html := MarkdownToHTML(body); html != "" {
			payload["body_html"] = html
		}
	}
	path := fmt.Sprintf("/tickets/%s/comments", url.PathEscape(ticketID))
	if err := c.do(ctx, http.MethodPost, path, payload, &out); err != nil {
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
