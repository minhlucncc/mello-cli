package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"

	"github.com/minhlucncc/mello-cli/internal/mello"
)

// canon is the normalized, order-stable view of a ticket's editable content that
// we hash. Field order is fixed by the struct, so json.Marshal is deterministic.
type canon struct {
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	Assignee    string   `json:"assignee"`
	Column      string   `json:"column"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
}

// HashDoc returns the content hash of an editable ticket doc (working copy).
func HashDoc(d TicketDoc) string {
	return hashCanon(canon{
		Title:       d.Title,
		Status:      d.Status,
		Assignee:    d.Assignee,
		Column:      d.Column,
		Description: d.Description,
		Labels:      normLabels(d.Labels),
	})
}

// HashTicket returns the content hash of a remote ticket given its column name —
// the baseline hash recorded at sync time. Must agree with HashDoc for an
// unedited ticket.
func HashTicket(t mello.Ticket, columnName string) string {
	return hashCanon(canon{
		Title:       t.Title,
		Status:      t.Status,
		Assignee:    t.AssigneeID,
		Column:      columnName,
		Description: t.Description,
		Labels:      normLabels(t.Labels),
	})
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

func normLabels(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	return in
}
