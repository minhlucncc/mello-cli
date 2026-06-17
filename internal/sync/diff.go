package sync

import "github.com/minhlucncc/mello-cli/internal/mello"

// ChangeKind classifies a ticket's pending change.
type ChangeKind string

const (
	KindNone     ChangeKind = ""
	KindCreate   ChangeKind = "create"   // local-only ticket → POST
	KindUpdate   ChangeKind = "update"   // field/body/move/comment/attachment edits → PATCH/…
	KindDelete   ChangeKind = "delete"   // folder removed locally → DELETE
	KindConflict ChangeKind = "conflict" // both local and remote changed since baseline
	KindRemote   ChangeKind = "remote"   // only remote changed (pull to update)
)

// PendingComment is a locally-authored comment file to be posted on push.
type PendingComment struct {
	Path string
	Body string
}

// Change is the full set of pending edits to one ticket relative to baseline.
type Change struct {
	Slug     string
	Ref      string // code or short id, for display
	RemoteID string // empty for a create
	ColumnID string // baseline column id (for move resolution context)

	Kind ChangeKind

	Update         mello.TicketUpdate
	HasFieldChange bool
	MoveToColumn   string // destination column NAME; empty if no move
	NewComments    []PendingComment
	NewAttachments []string // local file paths to upload

	// For create: the full desired doc + target column name.
	CreateDoc    *TicketDoc
	CreateColumn string
}

// Empty reports whether the change carries nothing actionable.
func (c Change) Empty() bool {
	return c.Kind == KindNone
}

// Symbol returns the one-character status marker for plan output.
func (c Change) Symbol() string {
	switch c.Kind {
	case KindCreate:
		return "+"
	case KindDelete:
		return "-"
	case KindConflict:
		return "!"
	case KindRemote:
		return "↓"
	case KindUpdate:
		return "~"
	default:
		return " "
	}
}

// Plan is an ordered set of ticket changes (the output of status / the input to
// push), plus a count summary.
type Plan struct {
	Changes []Change `json:"changes"`
}

// Summary tallies changes by kind.
func (p Plan) Summary() (create, update, del, conflict, remote int) {
	for _, c := range p.Changes {
		switch c.Kind {
		case KindCreate:
			create++
		case KindUpdate:
			update++
		case KindDelete:
			del++
		case KindConflict:
			conflict++
		case KindRemote:
			remote++
		}
	}
	return
}

// diffFields compares the working doc against the baseline ticket and returns
// the minimal TicketUpdate plus whether any field changed.
func diffFields(doc TicketDoc, base mello.Ticket) (mello.TicketUpdate, bool) {
	var upd mello.TicketUpdate
	changed := false
	if doc.Title != base.Title {
		v := doc.Title
		upd.Title = &v
		changed = true
	}
	if doc.Description != base.Description {
		v := doc.Description
		upd.Description = &v
		changed = true
	}
	if doc.Status != base.Status {
		v := doc.Status
		upd.Status = &v
		changed = true
	}
	if doc.Assignee != base.AssigneeID {
		v := doc.Assignee
		upd.AssigneeID = &v
		changed = true
	}
	if !sameStrings(doc.Labels, base.Labels) {
		v := append([]string(nil), doc.Labels...)
		upd.Labels = &v
		changed = true
	}
	return upd, changed
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
