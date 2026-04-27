package activity

import (
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("expected non-nil service")
	}
	if s.Count() != 0 {
		t.Fatalf("expected 0 viewers, got %d", s.Count())
	}
}

func TestAddAndList(t *testing.T) {
	s := New()

	v := &Viewer{
		SessionID:  "sess-1",
		StreamID:   "stream-1",
		StreamName: "BBC One",
		UserID:     "user-1",
		Username:   "admin",
		ClientName: "Browser",
		Delivery:   "mse",
		StartedAt:  time.Now(),
		RemoteAddr: "192.168.1.10:54321",
	}
	s.Add(v)

	if s.Count() != 1 {
		t.Fatalf("expected 1 viewer, got %d", s.Count())
	}

	list := s.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 viewer in list, got %d", len(list))
	}
	if list[0].SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", list[0].SessionID, "sess-1")
	}
	if list[0].StreamName != "BBC One" {
		t.Errorf("StreamName = %q, want %q", list[0].StreamName, "BBC One")
	}
	if list[0].Username != "admin" {
		t.Errorf("Username = %q, want %q", list[0].Username, "admin")
	}
}

func TestRemove(t *testing.T) {
	s := New()

	s.Add(&Viewer{SessionID: "sess-1", StreamName: "BBC One"})
	s.Add(&Viewer{SessionID: "sess-2", StreamName: "BBC Two"})

	if s.Count() != 2 {
		t.Fatalf("expected 2 viewers, got %d", s.Count())
	}

	s.Remove("sess-1")

	if s.Count() != 1 {
		t.Fatalf("expected 1 viewer after remove, got %d", s.Count())
	}

	list := s.List()
	if list[0].SessionID != "sess-2" {
		t.Errorf("remaining viewer SessionID = %q, want %q", list[0].SessionID, "sess-2")
	}
}

func TestRemoveNonexistent(t *testing.T) {
	s := New()
	s.Add(&Viewer{SessionID: "sess-1"})
	s.Remove("nonexistent")

	if s.Count() != 1 {
		t.Fatalf("expected 1 viewer, got %d", s.Count())
	}
}

func TestAddOverwritesSameSessionID(t *testing.T) {
	s := New()

	s.Add(&Viewer{SessionID: "sess-1", StreamName: "BBC One"})
	s.Add(&Viewer{SessionID: "sess-1", StreamName: "BBC Two"})

	if s.Count() != 1 {
		t.Fatalf("expected 1 viewer, got %d", s.Count())
	}

	list := s.List()
	if list[0].StreamName != "BBC Two" {
		t.Errorf("StreamName = %q, want %q (should overwrite)", list[0].StreamName, "BBC Two")
	}
}

func TestListReturnsEmptySlice(t *testing.T) {
	s := New()
	list := s.List()
	if list == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(list) != 0 {
		t.Fatalf("expected 0, got %d", len(list))
	}
}
