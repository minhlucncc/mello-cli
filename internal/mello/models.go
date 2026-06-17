package mello

import (
	"encoding/json"
	"time"
)

// Labels is a ticket's labels. Mello deployments return these either as plain
// strings or as objects (e.g. {"id":…,"name":…}); Labels accepts both and is
// treated as a list of label names everywhere else. It marshals back to a
// string array so local baselines stay simple.
type Labels []string

// UnmarshalJSON accepts ["a","b"], [{"name":"a"}, …], or null.
func (l *Labels) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*l = nil
		return nil
	}
	var ss []string
	if err := json.Unmarshal(data, &ss); err == nil {
		*l = ss
		return nil
	}
	var objs []map[string]any
	if err := json.Unmarshal(data, &objs); err != nil {
		return err
	}
	out := make([]string, 0, len(objs))
	for _, o := range objs {
		if n := labelName(o); n != "" {
			out = append(out, n)
		}
	}
	*l = out
	return nil
}

func labelName(o map[string]any) string {
	for _, k := range []string{"name", "title", "label", "value", "text", "id"} {
		if v, ok := o[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// Workspace is a scoped container for boards and members. JSON tags match the
// upstream mello-sdk so behavior matches the worker.
type Workspace struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OwnerID string `json:"owner_id,omitempty"`
	Role    string `json:"role,omitempty"`
}

// Board is an organizational unit within a workspace.
type Board struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Name        string `json:"name"`
	Code        string `json:"code,omitempty"`
}

// Column is a status lane on a board; tickets are nested when listing columns.
type Column struct {
	ID       string   `json:"id"`
	BoardID  string   `json:"board_id,omitempty"`
	Name     string   `json:"name"`
	Position int      `json:"position,omitempty"`
	Tickets  []Ticket `json:"tickets,omitempty"`
}

// Ticket is a single card on a board.
type Ticket struct {
	ID          string     `json:"id"`
	TicketCode  string     `json:"ticket_code,omitempty"`
	ColumnID    string     `json:"column_id,omitempty"`
	BoardID     string     `json:"board_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Position    int        `json:"position,omitempty"`
	Status      string     `json:"status,omitempty"`
	AssigneeID  string     `json:"assignee_id,omitempty"`
	Labels      Labels     `json:"labels,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// Comment is a markdown annotation on a ticket.
type Comment struct {
	ID        string     `json:"id"`
	TicketID  string     `json:"ticket_id,omitempty"`
	AuthorID  string     `json:"author_id,omitempty"`
	Body      string     `json:"body,omitempty"`
	BodyHTML  string     `json:"body_html,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// Member is a workspace user.
type Member struct {
	UserID   string   `json:"user_id"`
	Name     string   `json:"name,omitempty"`
	Email    string   `json:"email,omitempty"`
	Role     string   `json:"role,omitempty"`
	BoardIDs []string `json:"board_ids,omitempty"`
}

// User is the authenticated identity (GET /me). Optional endpoint.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// Attachment is a file attached to a ticket. Field names are best-effort against
// the (undocumented) attachments endpoint and tolerate alternates.
type Attachment struct {
	ID        string     `json:"id"`
	TicketID  string     `json:"ticket_id,omitempty"`
	Filename  string     `json:"filename,omitempty"`
	Name      string     `json:"name,omitempty"`
	URL       string     `json:"url,omitempty"`
	Size      int64      `json:"size,omitempty"`
	MIME      string     `json:"content_type,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// FileName returns the best available display name for an attachment.
func (a Attachment) FileName() string {
	if a.Filename != "" {
		return a.Filename
	}
	if a.Name != "" {
		return a.Name
	}
	return a.ID
}

// HistoryEntry is one ticket activity record (GET /tickets/{id}/history).
// Optional endpoint; shape is permissive.
type HistoryEntry struct {
	ID        string         `json:"id,omitempty"`
	Type      string         `json:"type,omitempty"`
	ActorID   string         `json:"actor_id,omitempty"`
	ActorName string         `json:"actor_name,omitempty"`
	CreatedAt *time.Time     `json:"created_at,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}
