// Package mello is a small, dependency-free client for the Mello REST API
// (https://mello.mezon.vn/api/v1). The transport (do/headers/error shape) is
// ported from the platform mello-sdk so behavior matches the worker; it is
// extended here with the endpoints the CLI needs (ticket edit/delete, comment
// read, attachments, /me, history).
package mello

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is Mello's public API base.
const DefaultBaseURL = "https://mello.mezon.vn/api/v1"

// Client is a Mello API client bound to one base URL + token. It supports two
// backends: the public API (`…/api/v1`, personal access token) and the internal
// API (`…/api`, session JWT). Mode is inferred from the base URL.
type Client struct {
	baseURL  string
	token    string
	http     *http.Client
	internal bool // true for the internal /api backend, false for public /api/v1
}

// NewClient builds a client. An empty baseURL falls back to DefaultBaseURL; a
// non-positive timeout falls back to 30s. A base URL not ending in "/v1" is
// treated as the internal API.
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	base := strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL:  base,
		token:    token,
		http:     &http.Client{Timeout: timeout},
		internal: !strings.HasSuffix(base, "/v1"),
	}
}

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// Internal reports whether the client targets the internal /api backend.
func (c *Client) Internal() bool { return c.internal }

// ListWorkspaces returns the token's workspaces (GET /workspaces).
func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	var out []Workspace
	if err := c.do(ctx, http.MethodGet, "/workspaces", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListBoards returns the boards in a workspace (GET /workspaces/{id}/boards).
func (c *Client) ListBoards(ctx context.Context, workspaceID string) ([]Board, error) {
	var out []Board
	path := fmt.Sprintf("/workspaces/%s/boards", url.PathEscape(workspaceID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateBoard creates a board in a workspace (POST /workspaces/{id}/boards).
// Code may be empty — Mello auto-generates it.
func (c *Client) CreateBoard(ctx context.Context, workspaceID, name, code string) (Board, error) {
	body := map[string]any{"name": name}
	if code != "" {
		body["code"] = code
	}
	var out Board
	path := fmt.Sprintf("/workspaces/%s/boards", url.PathEscape(workspaceID))
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return Board{}, err
	}
	return out, nil
}

// ListMembers returns workspace members (GET /workspaces/{id}/members).
func (c *Client) ListMembers(ctx context.Context, workspaceID string) ([]Member, error) {
	var out []Member
	path := fmt.Sprintf("/workspaces/%s/members", url.PathEscape(workspaceID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListColumns returns a board's columns, each with its tickets nested. The
// internal API serves these via the board detail (GET /boards/{id}); the public
// API via GET /boards/{id}/columns.
func (c *Client) ListColumns(ctx context.Context, boardID string) ([]Column, error) {
	if c.internal {
		var bd BoardDetail
		path := fmt.Sprintf("/boards/%s", url.PathEscape(boardID))
		if err := c.do(ctx, http.MethodGet, path, nil, &bd); err != nil {
			return nil, err
		}
		return bd.Columns, nil
	}
	var out []Column
	path := fmt.Sprintf("/boards/%s/columns", url.PathEscape(boardID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateColumn adds a column to a board (POST /boards/{id}/columns).
func (c *Client) CreateColumn(ctx context.Context, boardID, name string) (Column, error) {
	var out Column
	path := fmt.Sprintf("/boards/%s/columns", url.PathEscape(boardID))
	if err := c.do(ctx, http.MethodPost, path, map[string]any{"name": name}, &out); err != nil {
		return Column{}, err
	}
	return out, nil
}

// Search runs a full-text ticket query scoped to a workspace
// (GET /search?workspace_id=&q=).
func (c *Client) Search(ctx context.Context, workspaceID, query string) ([]Ticket, error) {
	q := url.Values{}
	q.Set("workspace_id", workspaceID)
	q.Set("q", query)
	var out []Ticket
	if err := c.do(ctx, http.MethodGet, "/search?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// do performs a JSON request and decodes the response into out (if non-nil).
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newAPIError(method, path, resp.StatusCode, data)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("mello %s %s: decode: %w", method, path, err)
		}
	}
	return nil
}

// newAPIError builds an APIError, extracting the standard {"error":"..."} code.
func newAPIError(method, path string, status int, data []byte) *APIError {
	e := &APIError{Method: method, Path: path, Status: status, Body: truncate(data, 300)}
	var parsed struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &parsed) == nil {
		if parsed.Error != "" {
			e.Code = parsed.Error
		} else if parsed.Message != "" {
			e.Code = parsed.Message
		}
	}
	return e
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}
