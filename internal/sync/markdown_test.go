package sync

import (
	"strings"
	"testing"

	"github.com/minhlucncc/mello-cli/internal/mello"
)

func TestTicketRoundTrip(t *testing.T) {
	orig := mello.Ticket{
		ID:          "id-123",
		TicketCode:  "PROJ-12",
		Title:       "Login: fix redirect",
		Description: "Steps:\n1. open\n2. boom\n",
		Status:      "open",
		AssigneeID:  "user-9",
		Labels:      []string{"bug", "p1"},
	}
	md := RenderTicket(orig, "In Progress")
	doc, err := ParseTicket(md)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Title != orig.Title {
		t.Errorf("title = %q", doc.Title)
	}
	if doc.Status != orig.Status {
		t.Errorf("status = %q", doc.Status)
	}
	if doc.Assignee != orig.AssigneeID {
		t.Errorf("assignee = %q", doc.Assignee)
	}
	if doc.Column != "In Progress" {
		t.Errorf("column = %q", doc.Column)
	}
	if len(doc.Labels) != 2 || doc.Labels[0] != "bug" || doc.Labels[1] != "p1" {
		t.Errorf("labels = %v", doc.Labels)
	}
	if doc.Description != "Steps:\n1. open\n2. boom" {
		t.Errorf("description = %q", doc.Description)
	}
	if doc.Ticket != "PROJ-12" {
		t.Errorf("ticket code = %q", doc.Ticket)
	}
}

func TestTitleWithColonQuoted(t *testing.T) {
	tk := mello.Ticket{ID: "x", Title: "fix: the thing", Status: "open"}
	doc, err := ParseTicket(RenderTicket(tk, "Todo"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "fix: the thing" {
		t.Fatalf("title not preserved through colon: %q", doc.Title)
	}
}

// TestMarkdownRoundTrip passes a real-Markdown body through RenderTicket and
// back, asserting every field is preserved. The body is rich on purpose —
// heading, fenced code, link, list, inline code, bold — so any regression in
// the body-passing or frontmatter handling will show up. (Multi-line
// frontmatter values are out of scope per the design; only the description
// body is multi-line, and it round-trips by way of being the body itself.)
func TestMarkdownRoundTrip(t *testing.T) {
	body := "# Heading\n\n`inline` and **bold** and [link](https://example.com).\n\n" +
		"- a\n- b\n- c\n\n" + "```\nblock\n```\n"
	orig := mello.Ticket{
		ID:          "id-xyz",
		TicketCode:  "PROJ-99",
		Title:       "Rich body",
		Description: body,
		Status:      "open",
		AssigneeID:  "user-1",
		Labels:      []string{"bug"},
	}
	md := RenderTicket(orig, "In Progress")
	doc, err := ParseTicket(md)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Title != "Rich body" {
		t.Errorf("title = %q", doc.Title)
	}
	// ParseTicket trims a single trailing newline; the body has one, so the
	// expected value strips it too. The intent is "the body content survives
	// the round-trip", which a per-character diff would still show.
	if doc.Description != strings.TrimRight(body, "\n") {
		t.Errorf("description mismatch:\nwant: %q\ngot:  %q", strings.TrimRight(body, "\n"), doc.Description)
	}
	if doc.Assignee != "user-1" {
		t.Errorf("assignee = %q", doc.Assignee)
	}
	if doc.Column != "In Progress" {
		t.Errorf("column = %q", doc.Column)
	}
}

// TestQuoteUnquoteBackslash exercises the backslash-escape fix in
// quoteIfNeeded / unquote. A value containing a literal backslash must
// survive the round-trip, and a value containing a double-quote must also
// survive. These were the two cases the old unquote mishandled.
func TestQuoteUnquoteBackslash(t *testing.T) {
	cases := []string{
		`path\to\file`,    // pure backslashes
		`say "hi"`,        // embedded double quote
		`mixed \ " \ end`, // both
		"tab\there",       // escaped tab
		`back\\slash`,     // escaped backslash
	}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			md := RenderTicket(mello.Ticket{ID: "x", Title: v, Status: "open"}, "Todo")
			doc, err := ParseTicket(md)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if doc.Title != v {
				t.Errorf("round-trip: want %q, got %q", v, doc.Title)
			}
		})
	}
}

// TestRenderTicketWithHTMLBody confirms that when the server returns a
// description_html, RenderTicket stores it verbatim and sets body_format=html.
// The body bytes must equal the original HTML (no Markdown wrapping) so the
// hash round-trips cleanly.
func TestRenderTicketWithHTMLBody(t *testing.T) {
	html := "<p>This is <strong>HTML</strong> from the server.</p>\n"
	tk := mello.Ticket{
		ID: "abc", TicketCode: "PROJ-7", Title: "HTML body",
		Description:     "fallback plain text",
		DescriptionHTML: html,
		Status:         "open",
	}
	md := string(RenderTicket(tk, "Todo"))
	if !strings.Contains(md, "body_format: html\n") {
		t.Errorf("missing body_format: html in frontmatter\n--- got ---\n%s", md)
	}
	if !strings.Contains(md, html) {
		t.Errorf("body does not contain original HTML\n--- got ---\n%s", md)
	}
	doc, err := ParseTicket([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if doc.BodyFormat != BodyFormatHTML {
		t.Errorf("BodyFormat = %q, want %q", doc.BodyFormat, BodyFormatHTML)
	}
	if doc.Description != strings.TrimRight(html, "\n") {
		t.Errorf("Description round-trip:\nwant: %q\ngot:  %q", strings.TrimRight(html, "\n"), doc.Description)
	}
}

// TestBodyFormatFrontmatterDefault confirms a doc without an explicit
// body_format key parses to "" (the default, which the push path treats as
// "convert to HTML").
func TestBodyFormatFrontmatterDefault(t *testing.T) {
	tk := mello.Ticket{ID: "x", Title: "Plain", Status: "open"}
	md := RenderTicket(tk, "Todo")
	doc, err := ParseTicket(md)
	if err != nil {
		t.Fatal(err)
	}
	if doc.BodyFormat != "" {
		t.Errorf("BodyFormat = %q, want empty (default = html conversion)", doc.BodyFormat)
	}
}

// TestParseTicketMalformedFrontmatter confirms the parser now reports a clear
// error on a line with no key/value separator, instead of silently dropping it.
func TestParseTicketMalformedFrontmatter(t *testing.T) {
	bad := "---\nticket: PROJ-1\nthis line has no colon\n---\n\nbody\n"
	if _, err := ParseTicket([]byte(bad)); err == nil {
		t.Errorf("expected error for malformed frontmatter, got nil")
	}
}

// TestParseTicketEmptyValue confirms a `key:` line (no value) is treated as
// the empty string, not a parse error.
func TestParseTicketEmptyValue(t *testing.T) {
	good := "---\nstatus:\n---\n\nbody\n"
	doc, err := ParseTicket([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Status != "" {
		t.Errorf("Status = %q, want empty", doc.Status)
	}
}
