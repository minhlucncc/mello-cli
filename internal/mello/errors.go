package mello

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError is a non-2xx response from the Mello API. The CLI branches on
// NotFound (so unverified/optional endpoints degrade gracefully) and Gone.
type APIError struct {
	Method string
	Path   string
	Status int
	// Code is the machine error from the body, e.g. "unauthorized",
	// "forbidden", "rate_limited" — empty if the body wasn't the standard shape.
	Code string
	Body string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("mello %s %s: %d %s", e.Method, e.Path, e.Status, e.Code)
	}
	return fmt.Sprintf("mello %s %s: status %d: %s", e.Method, e.Path, e.Status, e.Body)
}

// NotFound reports whether the endpoint or resource returned 404. The CLI uses
// this to detect Mello deployments that don't implement an optional endpoint
// (attachments, history, /me, ticket edit) and print a friendly message instead
// of a stack trace.
func (e *APIError) NotFound() bool { return e.Status == http.StatusNotFound }

// Gone reports HTTP 410 (Mello uses it to mean "permanently gone").
func (e *APIError) Gone() bool { return e.Status == http.StatusGone }

// Unauthorized reports a bad/missing token (401).
func (e *APIError) Unauthorized() bool { return e.Status == http.StatusUnauthorized }

// Forbidden reports an insufficient-scope error (403).
func (e *APIError) Forbidden() bool { return e.Status == http.StatusForbidden }

// RateLimited reports HTTP 429.
func (e *APIError) RateLimited() bool { return e.Status == http.StatusTooManyRequests }

// IsNotFound reports whether err is (or wraps) a 404 APIError. Commands call
// this to fall back when an optional endpoint is missing.
func IsNotFound(err error) bool {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.NotFound()
	}
	return false
}
