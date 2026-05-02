package demo

import (
	"context"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

type mockStreamStore struct {
	store.StreamStore
	upserted []media.Stream
	deleted  []string
	streams  []media.Stream
}

func (m *mockStreamStore) BulkUpsert(_ context.Context, streams []media.Stream) error {
	m.upserted = append(m.upserted, streams...)
	m.streams = streams
	return nil
}

func (m *mockStreamStore) DeleteStaleBySource(_ context.Context, _, _ string, _ []string) ([]string, error) {
	return m.deleted, nil
}

func (m *mockStreamStore) ListBySource(_ context.Context, _, _ string) ([]media.Stream, error) {
	return m.streams, nil
}

func (m *mockStreamStore) DeleteBySource(_ context.Context, _, _ string) error {
	m.streams = nil
	return nil
}

func TestRefresh(t *testing.T) {
	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "demo-1",
		Name:        "Demo Streams",
		IsEnabled:   true,
		StreamStore: ss,
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if len(ss.upserted) != 6 {
		t.Fatalf("expected 6 upserted streams, got %d", len(ss.upserted))
	}

	st := ss.upserted[0]
	if st.SourceType != "demo" {
		t.Errorf("expected source_type=demo, got %s", st.SourceType)
	}
	if st.Name != "Big Buck Bunny" {
		t.Errorf("expected name=Big Buck Bunny, got %s", st.Name)
	}
	if st.VODType != "movie" {
		t.Errorf("expected vod_type=movie, got %s", st.VODType)
	}
	if st.Group != "Demo - Movies" {
		t.Errorf("expected group=Demo - Movies, got %s", st.Group)
	}

	live := ss.upserted[4]
	if live.Name != "NASA Live" {
		t.Errorf("expected name=NASA Live, got %s", live.Name)
	}
	if live.VODType != "" {
		t.Errorf("expected empty vod_type for live stream, got %s", live.VODType)
	}
	if live.Group != "Demo - Live" {
		t.Errorf("expected group=Demo - Live, got %s", live.Group)
	}

	info := s.Info(context.Background())
	if info.StreamCount != 6 {
		t.Errorf("expected StreamCount=6, got %d", info.StreamCount)
	}
}

func TestStreamsAndDelete(t *testing.T) {
	ss := &mockStreamStore{
		streams: []media.Stream{
			{ID: "a", SourceType: string(source.TypeDemo), SourceID: "src1"},
			{ID: "b", SourceType: string(source.TypeDemo), SourceID: "src1"},
		},
	}
	s := New(Config{ID: "src1", Name: "Test", IsEnabled: true, StreamStore: ss})

	ids, err := s.Streams(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}

	if err := s.DeleteStreams(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ss.streams != nil {
		t.Error("expected streams to be nil after delete")
	}
}

func TestType(t *testing.T) {
	s := New(Config{ID: "x", Name: "X", IsEnabled: true})
	if s.Type() != "demo" {
		t.Errorf("expected type=demo, got %s", s.Type())
	}
}

func TestDeterministicIDs(t *testing.T) {
	id1 := deterministicStreamID("src1", "http://example.com/a.mp4")
	id2 := deterministicStreamID("src1", "http://example.com/a.mp4")
	id3 := deterministicStreamID("src1", "http://example.com/b.mp4")

	if id1 != id2 {
		t.Error("same inputs should produce same ID")
	}
	if id1 == id3 {
		t.Error("different inputs should produce different IDs")
	}
}
