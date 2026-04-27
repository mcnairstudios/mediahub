package channel

import (
	"context"
	"testing"
)

func TestChannelFields(t *testing.T) {
	ch := Channel{
		ID:         "ch-1",
		Name:       "BBC One",
		Number:     1,
		GroupID:    "grp-uk",
		StreamIDs:  []string{"s-1", "s-2"},
		LogoURL:    "https://example.com/bbc1.png",
		TvgID:      "bbc1.uk",
		IsEnabled:  true,
		IsFavorite: false,
	}

	if ch.ID != "ch-1" {
		t.Fatalf("expected ID ch-1, got %s", ch.ID)
	}
	if ch.Name != "BBC One" {
		t.Fatalf("expected Name BBC One, got %s", ch.Name)
	}
	if ch.Number != 1 {
		t.Fatalf("expected Number 1, got %d", ch.Number)
	}
	if ch.GroupID != "grp-uk" {
		t.Fatalf("expected GroupID grp-uk, got %s", ch.GroupID)
	}
	if len(ch.StreamIDs) != 2 {
		t.Fatalf("expected 2 StreamIDs, got %d", len(ch.StreamIDs))
	}
	if ch.TvgID != "bbc1.uk" {
		t.Fatalf("expected TvgID bbc1.uk, got %s", ch.TvgID)
	}
	if !ch.IsEnabled {
		t.Fatal("expected IsEnabled true")
	}
}

type mockStore struct{}

func (m *mockStore) Get(_ context.Context, _ string) (*Channel, error)    { return nil, nil }
func (m *mockStore) List(_ context.Context) ([]Channel, error)            { return nil, nil }
func (m *mockStore) Create(_ context.Context, _ *Channel) error           { return nil }
func (m *mockStore) Update(_ context.Context, _ *Channel) error           { return nil }
func (m *mockStore) Delete(_ context.Context, _ string) error             { return nil }
func (m *mockStore) AssignStreams(_ context.Context, _ string, _ []string) error { return nil }
func (m *mockStore) RemoveStreamMappings(_ context.Context, _ []string) error   { return nil }

func TestMockSatisfiesStore(t *testing.T) {
	var _ Store = (*mockStore)(nil)
}

type mockGroupStore struct{}

func (m *mockGroupStore) List(_ context.Context) ([]Group, error)  { return nil, nil }
func (m *mockGroupStore) Create(_ context.Context, _ *Group) error { return nil }
func (m *mockGroupStore) Delete(_ context.Context, _ string) error { return nil }

func TestMockSatisfiesGroupStore(t *testing.T) {
	var _ GroupStore = (*mockGroupStore)(nil)
}

func TestChannelNoStreams(t *testing.T) {
	ch := Channel{
		ID:        "ch-empty",
		Name:      "Empty Channel",
		Number:    99,
		IsEnabled: true,
	}

	if ch.StreamIDs != nil {
		t.Fatal("expected nil StreamIDs for channel with no streams")
	}
}

func TestChannelMultipleStreamsFailover(t *testing.T) {
	ch := Channel{
		ID:        "ch-failover",
		Name:      "Failover Channel",
		Number:    10,
		StreamIDs: []string{"primary", "backup-1", "backup-2"},
		IsEnabled: true,
	}

	if len(ch.StreamIDs) != 3 {
		t.Fatalf("expected 3 streams, got %d", len(ch.StreamIDs))
	}
	if ch.StreamIDs[0] != "primary" {
		t.Fatalf("expected first stream to be primary, got %s", ch.StreamIDs[0])
	}
	if ch.StreamIDs[1] != "backup-1" {
		t.Fatalf("expected second stream to be backup-1, got %s", ch.StreamIDs[1])
	}
}
