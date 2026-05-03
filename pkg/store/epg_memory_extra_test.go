package store

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/epg"
)

func TestProgramStore_ListAll(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "ch-1", Title: "Show A", StartTime: base, EndTime: base.Add(time.Hour)},
		{ChannelID: "ch-2", Title: "Show B", StartTime: base, EndTime: base.Add(time.Hour)},
		{ChannelID: "ch-1", Title: "Show C", StartTime: base.Add(time.Hour), EndTime: base.Add(2 * time.Hour)},
	})

	all, err := s.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 programs, got %d", len(all))
	}
}

func TestProgramStore_ListAllEmpty(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	all, err := s.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if all == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(all) != 0 {
		t.Fatalf("expected 0 programs, got %d", len(all))
	}
}

func TestProgramStore_ListChannelIDs(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "ch-1", Title: "A", StartTime: base, EndTime: base.Add(time.Hour)},
		{ChannelID: "ch-2", Title: "B", StartTime: base, EndTime: base.Add(time.Hour)},
		{ChannelID: "ch-1", Title: "C", StartTime: base.Add(time.Hour), EndTime: base.Add(2 * time.Hour)},
		{ChannelID: "ch-3", Title: "D", StartTime: base, EndTime: base.Add(time.Hour)},
	})

	ids, err := s.ListChannelIDs(ctx)
	if err != nil {
		t.Fatalf("ListChannelIDs: %v", err)
	}
	sort.Strings(ids)
	if len(ids) != 3 {
		t.Fatalf("expected 3 unique channel IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "ch-1" || ids[1] != "ch-2" || ids[2] != "ch-3" {
		t.Errorf("IDs = %v, want [ch-1 ch-2 ch-3]", ids)
	}
}

func TestProgramStore_ListChannelIDsEmpty(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	ids, err := s.ListChannelIDs(ctx)
	if err != nil {
		t.Fatalf("ListChannelIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected 0, got %d", len(ids))
	}
}

func TestProgramStore_ListBySeriesID(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "ch-1", Title: "Episode 1", SeriesID: "series-abc", StartTime: base, EndTime: base.Add(time.Hour)},
		{ChannelID: "ch-1", Title: "Episode 2", SeriesID: "series-abc", StartTime: base.Add(time.Hour), EndTime: base.Add(2 * time.Hour)},
		{ChannelID: "ch-1", Title: "Other Show", SeriesID: "series-xyz", StartTime: base, EndTime: base.Add(time.Hour)},
		{ChannelID: "ch-2", Title: "Episode 3", SeriesID: "series-abc", StartTime: base, EndTime: base.Add(time.Hour)},
	})

	got, err := s.ListBySeriesID(ctx, "series-abc")
	if err != nil {
		t.Fatalf("ListBySeriesID: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 programs for series-abc, got %d", len(got))
	}

	titles := make([]string, len(got))
	for i, p := range got {
		titles[i] = p.Title
	}
	sort.Strings(titles)
	if titles[0] != "Episode 1" || titles[1] != "Episode 2" || titles[2] != "Episode 3" {
		t.Errorf("titles = %v", titles)
	}
}

func TestProgramStore_ListBySeriesIDNotFound(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "ch-1", Title: "Show", SeriesID: "series-abc", StartTime: base, EndTime: base.Add(time.Hour)},
	})

	got, err := s.ListBySeriesID(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ListBySeriesID: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

func TestProgramStore_ListBySeriesIDEmpty(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	got, err := s.ListBySeriesID(ctx, "anything")
	if err != nil {
		t.Fatalf("ListBySeriesID: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

func TestProgramStore_ListAllReturnsCopy(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "ch-1", Title: "Original", StartTime: base, EndTime: base.Add(time.Hour)},
	})

	all, _ := s.ListAll(ctx)
	all[0].Title = "Modified"

	all2, _ := s.ListAll(ctx)
	if all2[0].Title != "Original" {
		t.Error("ListAll should return a copy, not a reference to internal data")
	}
}

func TestProgramStore_DeleteBySourcePreservesOthers(t *testing.T) {
	s := NewMemoryProgramStore()
	ctx := context.Background()

	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	s.BulkInsert(ctx, []epg.Program{
		{ChannelID: "src-1", Title: "A", StartTime: base, EndTime: base.Add(time.Hour)},
		{ChannelID: "src-2", Title: "B", StartTime: base, EndTime: base.Add(time.Hour)},
		{ChannelID: "src-3", Title: "C", StartTime: base, EndTime: base.Add(time.Hour)},
	})

	s.DeleteBySource(ctx, "src-2")

	all, _ := s.ListAll(ctx)
	if len(all) != 2 {
		t.Fatalf("expected 2 programs after delete, got %d", len(all))
	}

	ids, _ := s.ListChannelIDs(ctx)
	sort.Strings(ids)
	if len(ids) != 2 || ids[0] != "src-1" || ids[1] != "src-3" {
		t.Errorf("remaining channel IDs = %v, want [src-1 src-3]", ids)
	}
}

func TestEPGSourceStore_GetAndUpdateRoundtrip(t *testing.T) {
	s := NewMemoryEPGSourceStore()
	ctx := context.Background()

	now := time.Now()
	s.Create(ctx, &epg.Source{ID: "epg-1", Name: "Old Name", URL: "http://test.com/epg.xml", IsEnabled: true, LastRefreshed: &now})

	got, err := s.Get(ctx, "epg-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Name != "Old Name" {
		t.Errorf("Name = %q, want Old Name", got.Name)
	}
	if got.LastRefreshed == nil {
		t.Error("expected non-nil LastRefreshed")
	}

	s.Update(ctx, &epg.Source{ID: "epg-1", Name: "New Name", URL: "http://new.com/epg.xml"})
	got2, _ := s.Get(ctx, "epg-1")
	if got2.Name != "New Name" {
		t.Errorf("after update Name = %q, want New Name", got2.Name)
	}
	if got2.URL != "http://new.com/epg.xml" {
		t.Errorf("after update URL = %q, want http://new.com/epg.xml", got2.URL)
	}
}

func TestEPGSourceStore_DeleteNonexistent(t *testing.T) {
	s := NewMemoryEPGSourceStore()
	ctx := context.Background()

	err := s.Delete(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Delete on nonexistent should not error: %v", err)
	}
}
