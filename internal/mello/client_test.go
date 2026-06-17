package mello

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestShapingAndAuth(t *testing.T) {
	var gotAuth, gotAccept, gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotMethod = r.Method
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode([]Workspace{{ID: "w1", Name: "Acme"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "mello_pat_abc", 5*time.Second)
	ws, err := c.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 1 || ws[0].Name != "Acme" {
		t.Fatalf("workspaces = %+v", ws)
	}
	if gotAuth != "Bearer mello_pat_abc" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Errorf("accept = %q", gotAccept)
	}
	if gotMethod != http.MethodGet || gotPath != "/workspaces" {
		t.Errorf("%s %s", gotMethod, gotPath)
	}
}

func TestAPIErrorCodeParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "t", time.Second)
	_, err := c.ListWorkspaces(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok {
		t.Fatalf("not an APIError: %T", err)
	}
	if !ae.Forbidden() || ae.Code != "forbidden" {
		t.Errorf("ae = %+v", ae)
	}
}

func TestNotFoundDegrade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "t", time.Second)
	_, err := c.GetMe(context.Background())
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound, got %v", err)
	}
}

func TestMoveTicketSendsColumnAndPosition(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(Ticket{ID: "t1", ColumnID: "c2"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "t", time.Second)
	if _, err := c.MoveTicket(context.Background(), "t1", "c2", 3); err != nil {
		t.Fatal(err)
	}
	if body["column_id"] != "c2" {
		t.Errorf("column_id = %v", body["column_id"])
	}
	if body["position"].(float64) != 3 {
		t.Errorf("position = %v", body["position"])
	}
}

func TestUpdateTicketNoOpDoesNotCallAPI(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path == "/tickets/t1" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(Ticket{ID: "t1"})
			return
		}
		json.NewEncoder(w).Encode(Ticket{ID: "t1"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "t", time.Second)
	// Empty update should fetch (GET), not PATCH.
	if _, err := c.UpdateTicket(context.Background(), "t1", TicketUpdate{}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected a GET fallback for empty update")
	}
}
