// Package ui holds small output helpers: aligned tables, JSON dumps, colored
// status lines, and a no-echo prompt. No external dependencies.
package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
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

// Table renders rows with a header using tab alignment. Header cells are dimmed.
func Table(header []string, rows [][]string) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	if len(header) > 0 {
		cells := make([]string, len(header))
		for i, h := range header {
			cells[i] = Dim(strings.ToUpper(h))
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	_ = tw.Flush()
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

// PromptSecret reads a line from stdin without echo when stdin is a terminal.
// Falls back to a plain read otherwise (e.g. piped input).
func PromptSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	defer fmt.Fprintln(os.Stderr)
	if restore, ok := disableEcho(); ok {
		defer restore()
	}
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// ReadAllStdin returns all of stdin, trimmed of a trailing newline.
func ReadAllStdin() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

// IsInteractive reports whether stdin is a terminal (so prompts are sensible).
func IsInteractive() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

// Prompt writes a label to stderr and reads a trimmed line from stdin.
func Prompt(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Select prints a numbered menu and returns the chosen zero-based index.
func Select(title string, options []string) (int, error) {
	fmt.Fprintln(os.Stderr, title)
	for i, o := range options {
		fmt.Fprintf(os.Stderr, "  %s %s\n", Dim(fmt.Sprintf("%d)", i+1)), o)
	}
	in, err := Prompt(fmt.Sprintf("Enter a number [1-%d]: ", len(options)))
	if err != nil {
		return -1, err
	}
	n, err := strconv.Atoi(in)
	if err != nil || n < 1 || n > len(options) {
		return -1, fmt.Errorf("invalid selection %q", in)
	}
	return n - 1, nil
}
