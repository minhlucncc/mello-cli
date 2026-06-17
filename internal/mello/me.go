package mello

import (
	"context"
	"net/http"
)

// GetMe returns the authenticated identity (GET /me). Optional endpoint: on a
// 404 the CLI falls back to validating the token via ListWorkspaces.
func (c *Client) GetMe(ctx context.Context) (User, error) {
	var out User
	if err := c.do(ctx, http.MethodGet, "/me", nil, &out); err != nil {
		return User{}, err
	}
	return out, nil
}
