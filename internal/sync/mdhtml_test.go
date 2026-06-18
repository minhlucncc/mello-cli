package sync

import (
	"strings"
	"testing"
)

// TestRenderMarkdownToSafeHTML_BasicElements pins the conversion for the
// elements a ticket description typically needs: headings, paragraphs,
// emphasis, inline code, fenced code blocks, links, lists, blockquotes, and
// tables. The exact HTML is whatever goldmark emits through bluemonday's UGC
// policy — if goldmark is ever upgraded and the output shape changes, this
// test will catch it.
func TestRenderMarkdownToSafeHTML_BasicElements(t *testing.T) {
	in := `# H1

Some **bold** and *italic* and ` + "`inline`" + ` code.

` + "```go" + `
func main() {}
` + "```" + `

A [link](https://example.com).

- a
- b
- c

> quoted

| a | b |
|---|---|
| 1 | 2 |
`
	out, err := RenderMarkdownToSafeHTML(in)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		`<h1 id="h1">H1</h1>`,
		"<strong>bold</strong>",
		"<em>italic</em>",
		"<code>inline</code>",
		"<pre><code", "func main()",
		`<a href="https://example.com" rel="nofollow">link</a>`,
		"<ul>", "<li>a</li>", "<li>b</li>",
		"<blockquote>", "quoted", "</blockquote>",
		"<table>", "<th>a</th>", "<td>1</td>", "</table>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

// TestRenderMarkdownToSafeHTML_StripsDangerous confirms the sanitizer removes
// script tags, inline event handlers, and javascript: URLs. Without this, a
// pushed ticket description could XSS the web UI.
func TestRenderMarkdownToSafeHTML_StripsDangerous(t *testing.T) {
	cases := map[string]string{
		"<script>alert(1)</script>":                              "<script",
		"<img src=x onerror=alert(1)>":                           "onerror",
		"[click](javascript:alert(1))":                           "javascript:",
		"<iframe src=//evil></iframe>":                           "<iframe",
		"<a href=\"javascript:alert(1)\">x</a>":                  "javascript:",
	}
	for in, banned := range cases {
		out, err := RenderMarkdownToSafeHTML(in)
		if err != nil {
			t.Fatalf("render %q: %v", in, err)
		}
		if strings.Contains(strings.ToLower(out), strings.ToLower(banned)) {
			t.Errorf("input %q produced output still containing %q\n--- got ---\n%s", in, banned, out)
		}
	}
}

// TestRenderMarkdownToSafeHTML_Empty confirms the empty case is the empty
// string (no error, no `<p></p>` noise).
func TestRenderMarkdownToSafeHTML_Empty(t *testing.T) {
	out, err := RenderMarkdownToSafeHTML("")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}

// TestWrapDescription confirms the scoped-div wrapper is added and the empty
// case is a no-op.
func TestWrapDescription(t *testing.T) {
	if got := WrapDescription(""); got != "" {
		t.Errorf("empty wrap = %q", got)
	}
	if got := WrapDescription("<p>hi</p>"); got != `<div class="mello-md"><p>hi</p></div>` {
		t.Errorf("wrap = %q", got)
	}
}
