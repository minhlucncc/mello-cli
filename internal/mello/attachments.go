package mello

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

// Attachment endpoints are not in the upstream SDK and the public docs don't pin
// exact paths. We use the conventional shape and degrade gracefully (404) when a
// Mello deployment doesn't implement them.

// ListAttachments returns a ticket's attachments. The internal API embeds them
// in the ticket; the public API exposes GET /tickets/{id}/attachments.
func (c *Client) ListAttachments(ctx context.Context, ticketID string) ([]Attachment, error) {
	if c.internal {
		t, err := c.GetTicket(ctx, ticketID)
		if err != nil {
			return nil, err
		}
		return t.AttachmentList(), nil
	}
	var out []Attachment
	path := fmt.Sprintf("/tickets/%s/attachments", url.PathEscape(ticketID))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UploadAttachment uploads a local file to a ticket as multipart/form-data
// (POST /tickets/{id}/attachments, field "file"). Optional endpoint.
func (c *Client) UploadAttachment(ctx context.Context, ticketID, filePath string) (Attachment, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return Attachment{}, err
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return Attachment{}, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return Attachment{}, err
	}
	if err := mw.Close(); err != nil {
		return Attachment{}, err
	}

	path := fmt.Sprintf("/tickets/%s/attachments", url.PathEscape(ticketID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &buf)
	if err != nil {
		return Attachment{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return Attachment{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Attachment{}, newAPIError(http.MethodPost, path, resp.StatusCode, data)
	}
	var out Attachment
	if len(data) > 0 {
		_ = json.Unmarshal(data, &out)
	}
	if out.Filename == "" {
		out.Filename = filepath.Base(filePath)
	}
	return out, nil
}

// DeleteAttachment removes an attachment from a ticket. The internal API uses
// DELETE /attachments/{id}; the public API DELETE /tickets/{ticket}/attachments/{id}.
// Optional endpoint (degrades gracefully with 404).
func (c *Client) DeleteAttachment(ctx context.Context, ticketID, attachmentID string) error {
	var path string
	if c.internal {
		path = fmt.Sprintf("/attachments/%s", url.PathEscape(attachmentID))
	} else {
		path = fmt.Sprintf("/tickets/%s/attachments/%s",
			url.PathEscape(ticketID), url.PathEscape(attachmentID))
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// DownloadAttachment streams an attachment's bytes to w. If att.URL is an
// absolute URL it is fetched directly; otherwise the canonical endpoint
// (GET /tickets/{ticket}/attachments/{id}) is used. Optional endpoint.
func (c *Client) DownloadAttachment(ctx context.Context, ticketID string, att Attachment, w io.Writer) error {
	target := att.URL
	if target == "" {
		if c.internal {
			target = fmt.Sprintf("%s/attachments/%s/download", c.baseURL, url.PathEscape(att.ID))
		} else {
			target = fmt.Sprintf("%s/tickets/%s/attachments/%s",
				c.baseURL, url.PathEscape(ticketID), url.PathEscape(att.ID))
		}
	} else if !hasScheme(target) {
		target = c.baseURL + ensureLeadingSlash(target)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return newAPIError(http.MethodGet, target, resp.StatusCode, data)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func hasScheme(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || (len(s) > 8 && s[:8] == "https://"))
}

func ensureLeadingSlash(s string) string {
	if s == "" || s[0] == '/' {
		return s
	}
	return "/" + s
}
