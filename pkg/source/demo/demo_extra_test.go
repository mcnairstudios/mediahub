package demo

import (
	"context"
	"fmt"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

type errorStreamStore struct {
	store.StreamStore
}

func (e *errorStreamStore) BulkUpsert(_ context.Context, _ []media.Stream) error {
	return fmt.Errorf("store error")
}

func (e *errorStreamStore) DeleteStaleBySource(_ context.Context, _, _ string, _ []string) ([]string, error) {
	return nil, nil
}

func (e *errorStreamStore) ListBySource(_ context.Context, _, _ string) ([]media.Stream, error) {
	return nil, fmt.Errorf("list error")
}

func (e *errorStreamStore) DeleteBySource(_ context.Context, _, _ string) error {
	return fmt.Errorf("delete error")
}

func TestRefresh_StoreError(t *testing.T) {
	ss := &errorStreamStore{}
	s := New(Config{
		ID:          "demo-1",
		Name:        "Demo",
		IsEnabled:   true,
		StreamStore: ss,
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error from store")
	}

	info := s.Info(context.Background())
	if info.LastError == "" {
		t.Error("expected LastError to be set")
	}
}

func TestStreams_StoreError(t *testing.T) {
	ss := &errorStreamStore{}
	s := New(Config{ID: "demo-1", Name: "Demo", IsEnabled: true, StreamStore: ss})

	_, err := s.Streams(context.Background())
	if err == nil {
		t.Fatal("expected error from store")
	}
}

func TestDeleteStreams_StoreError(t *testing.T) {
	ss := &errorStreamStore{}
	s := New(Config{ID: "demo-1", Name: "Demo", IsEnabled: true, StreamStore: ss})

	err := s.DeleteStreams(context.Background())
	if err == nil {
		t.Fatal("expected error from store")
	}
}

func TestRefresh_AllStreamsHaveExpectedFields(t *testing.T) {
	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-id",
		Name:        "Test Demo",
		IsEnabled:   true,
		StreamStore: ss,
	})

	s.Refresh(context.Background())

	for i, st := range ss.upserted {
		if st.ID == "" {
			t.Errorf("stream %d: empty ID", i)
		}
		if st.Name == "" {
			t.Errorf("stream %d: empty Name", i)
		}
		if st.URL == "" {
			t.Errorf("stream %d: empty URL", i)
		}
		if st.SourceType != "demo" {
			t.Errorf("stream %d: SourceType = %q, want demo", i, st.SourceType)
		}
		if st.SourceID != "test-id" {
			t.Errorf("stream %d: SourceID = %q, want test-id", i, st.SourceID)
		}
		if !st.IsActive {
			t.Errorf("stream %d: expected IsActive=true", i)
		}
		if st.Group == "" {
			t.Errorf("stream %d: empty Group", i)
		}
	}
}

func TestRefresh_IDsDeterministic(t *testing.T) {
	ss1 := &mockStreamStore{}
	s1 := New(Config{ID: "same-id", Name: "Demo", IsEnabled: true, StreamStore: ss1})
	s1.Refresh(context.Background())

	ss2 := &mockStreamStore{}
	s2 := New(Config{ID: "same-id", Name: "Demo", IsEnabled: true, StreamStore: ss2})
	s2.Refresh(context.Background())

	if len(ss1.upserted) != len(ss2.upserted) {
		t.Fatalf("different stream counts: %d vs %d", len(ss1.upserted), len(ss2.upserted))
	}

	for i := range ss1.upserted {
		if ss1.upserted[i].ID != ss2.upserted[i].ID {
			t.Errorf("stream %d: IDs differ between runs: %s vs %s", i, ss1.upserted[i].ID, ss2.upserted[i].ID)
		}
	}
}

func TestInfo_InitialState(t *testing.T) {
	s := New(Config{ID: "demo-1", Name: "My Demo", IsEnabled: true})

	info := s.Info(context.Background())
	if info.ID != "demo-1" {
		t.Errorf("ID = %q, want demo-1", info.ID)
	}
	if info.Name != "My Demo" {
		t.Errorf("Name = %q, want My Demo", info.Name)
	}
	if !info.IsEnabled {
		t.Error("expected IsEnabled=true")
	}
	if info.StreamCount != 0 {
		t.Errorf("expected StreamCount=0 before refresh, got %d", info.StreamCount)
	}
	if info.LastRefreshed != nil {
		t.Error("expected nil LastRefreshed before refresh")
	}
	if info.LastError != "" {
		t.Errorf("expected empty LastError, got %q", info.LastError)
	}
}
