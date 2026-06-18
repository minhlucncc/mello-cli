package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/minhlucncc/mello-cli/internal/mello"
)

// canon is the normalized, order-stable view of a ticket's editable content that
// we hash. Field order is fixed by the struct, so json.Marshal is deterministic.
//
// Note: BodyFormat is intentionally NOT part of the hash. It is a push-time
// directive ("convert this body to HTML before sending"), not content. Including
// it would cause spurious local-change detections when the user toggles the
// format on an otherwise-untouched ticket, and the remote ticket has no
// corresponding field.
type canon struct {
	Title          string   `json:"title"`
	Status         string   `json:"status"`
	Assignee       string   `json:"assignee"`
	Column         string   `json:"column"`
	Description    string   `json:"description"`
	DescriptionHTML string   `json:"description_html"`
	Labels         []string `json:"labels"`
}

// HashDoc returns the content hash of an editable ticket doc (working copy).
// When the doc carries HTML (body_format = "html"), DescriptionHTML is the
// source of truth and the plain Description is ignored.
func HashDoc(d TicketDoc) string {
	c := canon{
		Title:    d.Title,
		Status:   d.Status,
		Assignee: d.Assignee,
		Column:   d.Column,
		Labels:   normLabels(d.Labels),
	}
	if d.BodyFormat == BodyFormatHTML && d.Description != "" {
		c.DescriptionHTML = normDescription(d.Description)
	} else {
		c.Description = normDescription(d.Description)
	}
	return hashCanon(c)
}

// HashTicket returns the content hash of a remote ticket given its column name —
// the baseline hash recorded at sync time. Must agree with HashDoc for an
// unedited ticket. When the remote carries description_html, that is the source
// of truth; the plain description is ignored (it is auto-derived).
func HashTicket(t mello.Ticket, columnName string) string {
	c := canon{
		Title:    t.Title,
		Status:   t.Status,
		Assignee: t.AssigneeID,
		Column:   columnName,
		Labels:   normLabels(t.Labels),
	}
	if t.DescriptionHTML != "" {
		c.DescriptionHTML = normDescription(t.DescriptionHTML)
	} else {
		c.Description = normDescription(t.Description)
	}
	return hashCanon(c)
}

// normDescription trims a single trailing newline so that a remote description
// that happens to end in "\n" (e.g. goldmark-emitted HTML) hashes the same as
// the local document body (which ParseTicket already trims).
func normDescription(s string) string {
	return strings.TrimRight(s, "\n")
}

func normLabels(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	return in
}

func hashCanon(c canon) string {
	b, _ := json.Marshal(c)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// HashFile returns the sha256 of a file's contents (for attachment diffing).
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
