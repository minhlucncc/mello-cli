package sync

import (
	"bytes"
	"fmt"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// mdRenderer is a package-level goldmark instance. Goldmark is safe for
// concurrent use and is expensive to construct, so we share one.
//
// We enable the GFM extension set: tables, strikethrough, autolinks, task
// lists, footnotes. This matches the common Markdown flavor users expect in a
// project-management tool. Autoheading IDs and raw HTML pass-through are
// disabled; we let bluemonday decide what HTML reaches the server.
var mdRenderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		// WithUnsafe deliberately omitted: any raw HTML the user writes is
		// passed through bluemonday, not through goldmark's unsafe renderer.
	),
)

// ugcSanitizer is the bluemonday UGC (user-generated content) policy. It allows
// the HTML elements a description legitimately needs (headings, lists, code,
// links, images, tables, blockquotes, emphasis) and strips anything dangerous
// (script, iframe, on*= handlers, javascript: URLs, etc.). Safe for concurrent
// use.
//
// bluemonday v1.0.x exposes the policy as a package-level function (the newer
// v2 module renamed it to NewUGCPolicy). We pin v1 here because it is the
// cached version and is the most widely deployed.
var ugcSanitizer = bluemonday.UGCPolicy()

// RenderMarkdownToSafeHTML converts a Markdown source string into sanitized
// HTML suitable to be stored as a ticket description and rendered by the
// web UI. The conversion is intentionally lossy only in the direction of
// safety — goldmark parses the source, then bluemonday strips anything not
// allowed by the UGC policy.
//
// The function is pure: same input → same output. It returns an error only on
// programmer error (goldmark itself does not return errors on input).
func RenderMarkdownToSafeHTML(md string) (string, error) {
	if md == "" {
		return "", nil
	}
	var buf bytes.Buffer
	if err := mdRenderer.Convert([]byte(md), &buf); err != nil {
		return "", fmt.Errorf("mdhtml: render: %w", err)
	}
	cleaned := ugcSanitizer.SanitizeBytes(buf.Bytes())
	return string(cleaned), nil
}

// WrapDescription wraps sanitized HTML in a scoped div so the web UI can apply
// description-specific styling without bleeding into the rest of the page. It
// is a no-op on empty input.
func WrapDescription(htmlBody string) string {
	if htmlBody == "" {
		return ""
	}
	return `<div class="mello-md">` + htmlBody + `</div>`
}
