package activity

import (
	"sync"
	"testing"
	"time"
)

func TestTouchUser_NewSession(t *testing.T) {
	s := New()

	s.TouchUser("u-1", "alice", "dashboard", "192.168.1.1:1234", "Mozilla/5.0")

	users := s.RecentUsers()
	if len(users) != 1 {
		t.Fatalf("expected 1 recent user, got %d", len(users))
	}
	if users[0].UserID != "u-1" {
		t.Errorf("UserID = %q, want %q", users[0].UserID, "u-1")
	}
	if users[0].Username != "alice" {
		t.Errorf("Username = %q, want %q", users[0].Username, "alice")
	}
	if users[0].Source != "dashboard" {
		t.Errorf("Source = %q, want %q", users[0].Source, "dashboard")
	}
	if users[0].RemoteAddr != "192.168.1.1:1234" {
		t.Errorf("RemoteAddr = %q, want %q", users[0].RemoteAddr, "192.168.1.1:1234")
	}
	if users[0].UserAgent != "Mozilla/5.0" {
		t.Errorf("UserAgent = %q, want %q", users[0].UserAgent, "Mozilla/5.0")
	}
	if users[0].FirstSeen.IsZero() {
		t.Error("FirstSeen should not be zero")
	}
}

func TestTouchUser_UpdatesExisting(t *testing.T) {
	s := New()

	s.TouchUser("u-1", "alice", "dashboard", "192.168.1.1:1234", "Mozilla/5.0")
	time.Sleep(10 * time.Millisecond)
	s.TouchUser("u-1", "alice", "dashboard", "192.168.1.2:5678", "Chrome/131")

	users := s.RecentUsers()
	if len(users) != 1 {
		t.Fatalf("expected 1 user (updated, not duplicated), got %d", len(users))
	}
	if users[0].RemoteAddr != "192.168.1.2:5678" {
		t.Errorf("RemoteAddr should be updated, got %q", users[0].RemoteAddr)
	}
	if users[0].UserAgent != "Chrome/131" {
		t.Errorf("UserAgent should be updated, got %q", users[0].UserAgent)
	}
	if !users[0].LastSeen.After(users[0].FirstSeen) {
		t.Error("LastSeen should be after FirstSeen")
	}
}

func TestTouchUser_DifferentSourcesSeparateSessions(t *testing.T) {
	s := New()

	s.TouchUser("u-1", "alice", "dashboard", "192.168.1.1:1234", "Mozilla/5.0")
	s.TouchUser("u-1", "alice", "jellyfin", "192.168.1.1:4567", "Jellyfin/1.0")

	users := s.RecentUsers()
	if len(users) != 2 {
		t.Fatalf("expected 2 sessions (different sources), got %d", len(users))
	}
}

func TestTouchUser_DifferentUsersSeparateSessions(t *testing.T) {
	s := New()

	s.TouchUser("u-1", "alice", "dashboard", "192.168.1.1:1234", "Mozilla/5.0")
	s.TouchUser("u-2", "bob", "dashboard", "192.168.1.2:1234", "Mozilla/5.0")

	users := s.RecentUsers()
	if len(users) != 2 {
		t.Fatalf("expected 2 sessions (different users), got %d", len(users))
	}
}

func TestRecentUsers_SessionTimeout(t *testing.T) {
	s := New()

	key := "u-1:dashboard"
	s.mu.Lock()
	s.sessions[key] = &UserSession{
		UserID:   "u-1",
		Username: "alice",
		Source:   "dashboard",
		LastSeen: time.Now().Add(-21 * time.Minute),
	}
	s.mu.Unlock()

	users := s.RecentUsers()
	if len(users) != 0 {
		t.Fatalf("expected 0 recent users (session expired), got %d", len(users))
	}
}

func TestRecentUsers_JustBeforeTimeout(t *testing.T) {
	s := New()

	key := "u-1:dashboard"
	s.mu.Lock()
	s.sessions[key] = &UserSession{
		UserID:   "u-1",
		Username: "alice",
		Source:   "dashboard",
		LastSeen: time.Now().Add(-19 * time.Minute),
	}
	s.mu.Unlock()

	users := s.RecentUsers()
	if len(users) != 1 {
		t.Fatalf("expected 1 recent user (just before timeout), got %d", len(users))
	}
}

func TestRecentUsers_MixedActiveAndExpired(t *testing.T) {
	s := New()

	s.mu.Lock()
	s.sessions["u-1:dashboard"] = &UserSession{
		UserID:   "u-1",
		Username: "alice",
		Source:   "dashboard",
		LastSeen: time.Now().Add(-25 * time.Minute),
	}
	s.sessions["u-2:dashboard"] = &UserSession{
		UserID:   "u-2",
		Username: "bob",
		Source:   "dashboard",
		LastSeen: time.Now(),
	}
	s.mu.Unlock()

	users := s.RecentUsers()
	if len(users) != 1 {
		t.Fatalf("expected 1 active user, got %d", len(users))
	}
	if users[0].UserID != "u-2" {
		t.Errorf("expected bob (active), got %q", users[0].UserID)
	}
}

func TestRecentUsers_EmptyService(t *testing.T) {
	s := New()
	users := s.RecentUsers()
	if users == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(users) != 0 {
		t.Fatalf("expected 0, got %d", len(users))
	}
}

func TestConcurrentTouchAndList(t *testing.T) {
	s := New()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.TouchUser("u-1", "alice", "dashboard", "1.2.3.4:5678", "test")
		}()
		go func() {
			defer wg.Done()
			s.RecentUsers()
		}()
	}
	wg.Wait()
}

func TestConcurrentViewerAddRemoveList(t *testing.T) {
	s := New()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			s.Add(&Viewer{SessionID: "sess-race", StreamName: "test"})
		}()
		go func() {
			defer wg.Done()
			s.Remove("sess-race")
		}()
		go func() {
			defer wg.Done()
			s.List()
		}()
	}
	wg.Wait()
}
