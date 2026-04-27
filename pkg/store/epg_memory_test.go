package store

import (
	"context"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/epg"
)

func TestEPGSourceStore_CreateAndGet(t *testing.T) {
	s := NewMemoryEPGSourceStore()
	ctx := context.Background()

	src := &epg.Source{
		ID:        "epg-1",
		Name:      "XMLTV UK",
		URL:       "http://example.com/uk.xml",
		IsEnabled: true,
	}

	if err := s.Create(ctx, src); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "epg-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Name != "XMLTV UK" {
		t.Errorf("Name = %q, want %q", got.Name, "XMLTV UK")
	}
	if got.URL != "http://example.com/uk.xml" {
		t.Errorf("URL = %q, want %q", got.URL, "http://example.com/uk.xml")
	}
}

func TestEPGSourceStore_GetUnknownID(t *testing.T) {
	s := NewMemoryEPGSourceStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestEPGSourceStore_List(t *testing.T) {
	s := NewMemoryEPGSourceStore()
	ctx := context.Background()

	s.Create(ctx, &epg.Source{ID: "epg-1", Name: "UK"})
	s.Create(ctx, &epg.Source{ID: "epg-2", Name: "US"})

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("got %d sources, want 2", len(list))
	}
}

func TestEPGSourceStore_Update(t *testing.T) {
	s := NewMemoryEPGSourceStore()
	ctx := context.Background()

	s.Create(ctx, &epg.Source{ID: "epg-1", Name: "Old"})
	s.Update(ctx, &epg.Source{ID: "epg-1", Name: "New"})

	got, _ := s.Get(ctx, "epg-1")
	if got.Name != "New" {
		t.Errorf("Name = %q, want %q", got.Name, "New")
	}
}

func TestEPGSourceStore_Delete(t *testing.T) {
	s := NewMemoryEPGSourceStore()
	ctx := context.Background()

	s.Create(ctx, &epg.Source{ID: "epg-1"})
	s.Create(ctx, &epg.Source{ID: "epg-2"})

	if err := s.Delete(ctx, "epg-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Errorf("got %d sources, want 1", len(list))
	}
}

func TestProgramStore_NowPlaying(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	now := time.Now()
	programs := []epg.Program{
		{ChannelID: "ch-1", Title: "Past Show", StartTime: now.Add(-2 * time.Hour), EndTime: now.Add(-1 * time.Hour)},
		{ChannelID: "ch-1", Title: "Current Show", StartTime: now.Add(-30 * time.Minute), EndTime: now.Add(30 * time.Minute)},
		{ChannelID: "ch-1", Title: "Future Show", StartTime: now.Add(1 * time.Hour), EndTime: now.Add(2 * time.Hour)},
		{ChannelID: "ch-2", Title: "Other Channel", StartTime: now.Add(-30 * time.Minute), EndTime: now.Add(30 * time.Minute)},
	}

	if err := s.BulkInsert(ctx, programs); err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	got, err := s.NowPlaying(ctx, "ch-1")
	if err != nil {
		t.Fatalf("NowPlaying: %v", err)
	}
	if got == nil {
		t.Fatal("NowPlaying returned nil")
	}
	if got.Title != "Current Show" {
		t.Errorf("Title = %q, want %q", got.Title, "Current Show")
	}
}

func TestProgramStore_NowPlayingNoMatch(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	now := time.Now()
	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "ch-1", Title: "Past", StartTime: now.Add(-2 * time.Hour), EndTime: now.Add(-1 * time.Hour)},
	})

	got, err := s.NowPlaying(ctx, "ch-1")
	if err != nil {
		t.Fatalf("NowPlaying: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestProgramStore_Range(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	programs := []epg.Program{
		{ChannelID: "ch-1", Title: "Show A", StartTime: base, EndTime: base.Add(1 * time.Hour)},
		{ChannelID: "ch-1", Title: "Show B", StartTime: base.Add(1 * time.Hour), EndTime: base.Add(2 * time.Hour)},
		{ChannelID: "ch-1", Title: "Show C", StartTime: base.Add(3 * time.Hour), EndTime: base.Add(4 * time.Hour)},
		{ChannelID: "ch-2", Title: "Other", StartTime: base, EndTime: base.Add(1 * time.Hour)},
	}
	s.BulkInsert(ctx, programs)

	got, err := s.Range(ctx, "ch-1", base, base.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d programs, want 2", len(got))
	}
}

func TestProgramStore_RangeEmpty(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "ch-1", Title: "Show", StartTime: base, EndTime: base.Add(1 * time.Hour)},
	})

	got, err := s.Range(ctx, "ch-1", base.Add(5*time.Hour), base.Add(6*time.Hour))
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d programs, want 0", len(got))
	}
}

func TestProgramStore_BulkInsertAppends(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "ch-1", Title: "First", StartTime: base, EndTime: base.Add(1 * time.Hour)},
	})
	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "ch-1", Title: "Second", StartTime: base.Add(1 * time.Hour), EndTime: base.Add(2 * time.Hour)},
	})

	got, _ := s.Range(ctx, "ch-1", base, base.Add(2*time.Hour))
	if len(got) != 2 {
		t.Errorf("got %d programs, want 2", len(got))
	}
}

func TestProgramStore_DeleteBySource(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "src-1", Title: "A", StartTime: base, EndTime: base.Add(1 * time.Hour)},
		{ChannelID: "src-1", Title: "B", StartTime: base.Add(1 * time.Hour), EndTime: base.Add(2 * time.Hour)},
		{ChannelID: "src-2", Title: "C", StartTime: base, EndTime: base.Add(1 * time.Hour)},
	})

	if err := s.DeleteBySource(ctx, "src-1"); err != nil {
		t.Fatalf("DeleteBySource: %v", err)
	}

	got, _ := s.Range(ctx, "src-2", base, base.Add(2*time.Hour))
	if len(got) != 1 {
		t.Fatalf("got %d programs, want 1", len(got))
	}
	if got[0].Title != "C" {
		t.Errorf("Title = %q, want %q", got[0].Title, "C")
	}

	got2, _ := s.Range(ctx, "src-1", base, base.Add(2*time.Hour))
	if len(got2) != 0 {
		t.Errorf("got %d programs for deleted source, want 0", len(got2))
	}
}
