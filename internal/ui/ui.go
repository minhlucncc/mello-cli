// Package ui holds small output helpers: aligned tables, JSON dumps, colored
// status lines, and a no-echo prompt. No external dependencies.
package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// NoColor disables ANSI color (set from --no-color or a non-TTY stdout).
var NoColor = os.Getenv("NO_COLOR") != "" || os.Getenv("MELLO_NO_COLOR") != ""

func color(code, s string) string {
	if NoColor {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// Green, Yellow, Red, Dim, Bold wrap text in ANSI styles (no-ops if NoColor).
func Green(s string) string  { return color("32", s) }
func Yellow(s string) string { return color("33", s) }
func Red(s string) string    { return color("31", s) }
func Dim(s string) string    { return color("2", s) }
func Bold(s string) string   { return color("1", s) }

// Successf prints a green check line to stdout.
func Successf(format string, a ...any) {
	fmt.Printf("%s %s\n", Green("✓"), fmt.Sprintf(format, a...))
}

// Warnf prints a yellow warning to stderr.
func Warnf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", Yellow("!"), fmt.Sprintf(format, a...))
}

// Errorf prints a red error to stderr.
func Errorf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", Red("x"), fmt.Sprintf(format, a...))
}

// Table renders an aligned table with a dimmed header. Column widths are
// computed from the visible (ANSI-stripped) length of each cell, so colored
// cells and the dimmed header line up correctly.
func Table(header []string, rows [][]string) {
	cols := len(header)
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	widths := make([]int, cols)
	measure := func(cells []string) {
		for i, c := range cells {
			if w := visibleLen(c); w > widths[i] {
				widths[i] = w
			}
		}
	}
	measure(header)
	for _, r := range rows {
		measure(r)
	}

	line := func(cells []string, dim bool) string {
		var b strings.Builder
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			if i < cols-1 { // pad every column but the last
				cell = padRight(cell, widths[i])
			}
			if dim {
				cell = Dim(cell)
			}
			b.WriteString(cell)
			if i < cols-1 {
				b.WriteString("  ")
			}
		}
		return b.String()
	}

	if len(header) > 0 {
		up := make([]string, len(header))
		for i, h := range header {
			up[i] = strings.ToUpper(h)
		}
		fmt.Println(line(up, true))
	}
	for _, r := range rows {
		fmt.Println(line(r, false))
	}
}

func padRight(s string, w int) string {
	if pad := w - visibleLen(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// visibleLen counts runes excluding ANSI escape sequences.
func visibleLen(s string) int {
	n, inEsc := 0, false
	for _, r := range s {
		switch {
		case inEsc:
			if r == 'm' {
				inEsc = false
			}
		case r == '\x1b':
			inEsc = true
		default:
			n++
		}
	}
	return n
}

// JSON pretty-prints v to stdout (used for --json).
func JSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Truncate shortens s to n runes with an ellipsis.
func Truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// ReadAllStdin returns all of stdin, trimmed of a trailing newline. Used for
// non-interactive token input (`auth login --with-token`).
func ReadAllStdin() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}
