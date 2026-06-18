package mello

import (
	"encoding/json"
	"strconv"
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
	return pickStr(o, "name", "title", "label", "value", "text", "id")
}

func pickStr(o map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := o[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// TicketMember is a user assigned to a ticket.
type TicketMember struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Members is a ticket's assignees. Tickets may have several. Accepts arrays of
// ids (strings) or of objects with id/user_id and name (optionally nested under
// "user").
type Members []TicketMember

// UnmarshalJSON accepts ["u1","u2"], [{"user_id":"u1","name":"Minh"}], or null.
func (ms *Members) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*ms = nil
		return nil
	}
	var ss []string
	if json.Unmarshal(data, &ss) == nil {
		out := make(Members, 0, len(ss))
		for _, s := range ss {
			out = append(out, TicketMember{ID: s})
		}
		*ms = out
		return nil
	}
	var objs []map[string]any
	if err := json.Unmarshal(data, &objs); err != nil {
		return err
	}
	out := make(Members, 0, len(objs))
	for _, o := range objs {
		tm := TicketMember{
			ID:   pickStr(o, "user_id", "id", "userId", "member_id"),
			Name: pickStr(o, "name", "display_name", "full_name", "username", "email"),
		}
		if u, ok := o["user"].(map[string]any); ok {
			if tm.ID == "" {
				tm.ID = pickStr(u, "id", "user_id")
			}
			if tm.Name == "" {
				tm.Name = pickStr(u, "name", "display_name", "username", "email")
			}
		}
		out = append(out, tm)
	}
	*ms = out
	return nil
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

// BoardDetail is a board with its columns (each with nested tickets), as
// returned by GET /boards/{id}.
type BoardDetail struct {
	ID          string   `json:"id"`
	WorkspaceID string   `json:"workspace_id,omitempty"`
	Code        string   `json:"code,omitempty"`
	Name        string   `json:"name"`
	Columns     []Column `json:"columns,omitempty"`
}

// Ticket is a single card on a board.
type Ticket struct {
	ID             string         `json:"id"`
	TicketCode     string         `json:"ticket_code,omitempty"`
	ColumnID       string         `json:"column_id,omitempty"`
	BoardID        string         `json:"board_id,omitempty"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	DescriptionHTML string        `json:"description_html,omitempty"`
	Position       int            `json:"position,omitempty"`
	Status         string         `json:"status,omitempty"`
	AssigneeID     string         `json:"assignee_id,omitempty"`
	Members        Members        `json:"members,omitempty"`
	Assignees      Members        `json:"assignees,omitempty"`
	Labels         Labels         `json:"labels,omitempty"`
	Attachments    []Attachment   `json:"attachments,omitempty"`
	Files          []Attachment   `json:"files,omitempty"`
	Media          []Attachment   `json:"media,omitempty"`
	ColumnName     string         `json:"column_name,omitempty"`
	Comments       []Comment      `json:"comments,omitempty"`   // embedded (internal API)
	Activities     []HistoryEntry `json:"activities,omitempty"` // embedded history (internal API)
	CreatedAt      *time.Time     `json:"created_at,omitempty"`
	UpdatedAt      *time.Time     `json:"updated_at,omitempty"`
}

// AttachmentList returns attachments embedded in the ticket, however they are
// named (attachments / files / media).
func (t Ticket) AttachmentList() []Attachment {
	switch {
	case len(t.Attachments) > 0:
		return t.Attachments
	case len(t.Files) > 0:
		return t.Files
	case len(t.Media) > 0:
		return t.Media
	}
	return nil
}

// AssigneeMembers returns the ticket's assignees, however the API names them
// (members, assignees, or a single assignee_id).
func (t Ticket) AssigneeMembers() Members {
	if len(t.Members) > 0 {
		return t.Members
	}
	if len(t.Assignees) > 0 {
		return t.Assignees
	}
	if t.AssigneeID != "" {
		return Members{{ID: t.AssigneeID}}
	}
	return nil
}

// HasMember reports whether userID is among the ticket's assignees.
func (t Ticket) HasMember(userID string) bool {
	for _, m := range t.AssigneeMembers() {
		if m.ID == userID {
			return true
		}
	}
	return false
}

// Comment is a markdown annotation on a ticket.
type Comment struct {
	ID         string     `json:"id"`
	TicketID   string     `json:"ticket_id,omitempty"`
	AuthorID   string     `json:"author_id,omitempty"`
	AuthorName string     `json:"author_name,omitempty"`
	Body       string     `json:"body,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
}

// UnmarshalJSON decodes comments across deployments: body from body/content/
// text, author from author_id/user_id or a nested author/user object.
func (cm *Comment) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	cm.ID = pickStr(m, "id", "uuid")
	cm.TicketID = pickStr(m, "ticket_id")
	cm.Body = pickStr(m, "body", "content", "text", "message", "comment")
	cm.AuthorID = pickStr(m, "author_id", "user_id", "created_by")
	cm.AuthorName = pickStr(m, "author_name", "user_name")
	for _, k := range []string{"author", "user", "member"} {
		if o, ok := m[k].(map[string]any); ok {
			if cm.AuthorID == "" {
				cm.AuthorID = pickStr(o, "id", "user_id")
			}
			if cm.AuthorName == "" {
				cm.AuthorName = pickStr(o, "name", "display_name", "username", "email")
			}
		}
	}
	if ts := pickStr(m, "created_at", "createdAt", "timestamp"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			cm.CreatedAt = &t
		}
	}
	return nil
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

// Attachment is a file attached to a ticket. Deployments vary in field names, so
// it decodes permissively.
type Attachment struct {
	ID        string     `json:"id,omitempty"`
	TicketID  string     `json:"ticket_id,omitempty"`
	Filename  string     `json:"filename,omitempty"`
	URL       string     `json:"url,omitempty"`
	Size      int64      `json:"size,omitempty"`
	MIME      string     `json:"content_type,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// UnmarshalJSON maps a variety of key names to the attachment fields.
func (a *Attachment) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	a.ID = pickStr(m, "id", "uuid")
	a.TicketID = pickStr(m, "ticket_id")
	a.Filename = pickStr(m, "filename", "file_name", "name", "title", "original_name", "originalName")
	a.URL = pickStr(m, "url", "href", "download_url", "downloadUrl", "path", "src", "link", "location")
	a.MIME = pickStr(m, "content_type", "contentType", "mime", "mime_type", "type")
	a.Size = pickInt(m, "size", "bytes", "byte_size", "file_size", "fileSize", "length")
	if ts := pickStr(m, "created_at", "createdAt", "uploaded_at", "timestamp"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			a.CreatedAt = &t
		}
	}
	return nil
}

// FileName returns the best available display name for an attachment.
func (a Attachment) FileName() string {
	if a.Filename != "" {
		return a.Filename
	}
	if a.URL != "" {
		return a.URL
	}
	return a.ID
}

func pickInt(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return int64(v)
		case string:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}

// HistoryEntry is one ticket activity record (GET /tickets/{id}/history).
// Deployments name the fields differently, so it decodes permissively from the
// raw object and exposes a normalized view.
type HistoryEntry struct {
	ID        string
	Type      string
	ActorID   string
	ActorName string
	CreatedAt *time.Time
	Raw       map[string]any
}

// UnmarshalJSON pulls the event type, actor, and timestamp from whatever keys the
// instance uses, keeping the full object in Raw.
func (h *HistoryEntry) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	h.Raw = m
	h.ID = pickStr(m, "id", "uuid")
	h.Type = pickStr(m, "type", "action", "event", "event_type", "kind", "activity", "verb", "name")
	h.ActorID = pickStr(m, "actor_id", "user_id", "author_id", "member_id", "by_id", "created_by")
	h.ActorName = pickStr(m, "actor_name", "user_name", "author_name")
	for _, k := range []string{"actor", "user", "author", "member", "by"} {
		if o, ok := m[k].(map[string]any); ok {
			if h.ActorID == "" {
				h.ActorID = pickStr(o, "id", "user_id")
			}
			if h.ActorName == "" {
				h.ActorName = pickStr(o, "name", "display_name", "full_name", "username", "email")
			}
		}
	}
	if ts := pickStr(m, "created_at", "timestamp", "time", "at", "date", "created"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			h.CreatedAt = &t
		}
	}
	return nil
}

// MarshalJSON emits the original object so `--json` preserves every field.
func (h HistoryEntry) MarshalJSON() ([]byte, error) {
	if h.Raw != nil {
		return json.Marshal(h.Raw)
	}
	return json.Marshal(map[string]any{})
}

// Summary returns a short human description of the event ("type" plus any field
// the entry references), e.g. "ticket.updated" or "moved".
func (h HistoryEntry) Summary() string {
	if h.Type != "" {
		return h.Type
	}
	// Fall back to a likely descriptive field.
	return pickStr(h.Raw, "message", "description", "field", "summary", "detail")
}
