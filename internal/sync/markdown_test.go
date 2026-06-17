package sync

import (
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
