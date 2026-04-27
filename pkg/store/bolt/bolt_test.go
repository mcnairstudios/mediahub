package bolt

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

func tempDB(t *testing.T) (*DB, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db, path
}

func TestOpenClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file should exist after Open")
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestStreamStore_CreateAndGet(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	stream := media.Stream{
		ID:         "stream-1",
		SourceType: "m3u",
		SourceID:   "account-1",
		Name:       "BBC One",
		URL:        "http://example.com/bbc1",
	}

	err := s.BulkUpsert(ctx, []media.Stream{stream})
	if err != nil {
		t.Fatalf("BulkUpsert: %v", err)
	}

	got, err := s.Get(ctx, "stream-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "BBC One" {
		t.Errorf("got name %q, want %q", got.Name, "BBC One")
	}
	if got.URL != "http://example.com/bbc1" {
		t.Errorf("got URL %q, want %q", got.URL, "http://example.com/bbc1")
	}
}

func TestStreamStore_BulkUpsertUpdatesExisting(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	original := media.Stream{
		ID:   "stream-1",
		Name: "BBC One",
		URL:  "http://example.com/bbc1",
	}
	s.BulkUpsert(ctx, []media.Stream{original})

	updated := media.Stream{
		ID:   "stream-1",
		Name: "BBC One HD",
		URL:  "http://example.com/bbc1hd",
	}
	s.BulkUpsert(ctx, []media.Stream{updated})

	got, err := s.Get(ctx, "stream-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "BBC One HD" {
		t.Errorf("got name %q, want %q", got.Name, "BBC One HD")
	}
	if got.URL != "http://example.com/bbc1hd" {
		t.Errorf("got URL %q, want %q", got.URL, "http://example.com/bbc1hd")
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Errorf("got %d streams, want 1", len(list))
	}
}

func TestStreamStore_ListBySource(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "a1", Name: "One"},
		{ID: "s2", SourceType: "m3u", SourceID: "a1", Name: "Two"},
		{ID: "s3", SourceType: "m3u", SourceID: "a2", Name: "Three"},
		{ID: "s4", SourceType: "satip", SourceID: "b1", Name: "Four"},
	}
	s.BulkUpsert(ctx, streams)

	got, err := s.ListBySource(ctx, "m3u", "a1")
	if err != nil {
		t.Fatalf("ListBySource: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d streams, want 2", len(got))
	}

	names := []string{got[0].Name, got[1].Name}
	sort.Strings(names)
	if names[0] != "One" || names[1] != "Two" {
		t.Errorf("got names %v, want [One Two]", names)
	}
}

func TestStreamStore_DeleteBySource(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "a1"},
		{ID: "s2", SourceType: "m3u", SourceID: "a1"},
		{ID: "s3", SourceType: "m3u", SourceID: "a2"},
	}
	s.BulkUpsert(ctx, streams)

	err := s.DeleteBySource(ctx, "m3u", "a1")
	if err != nil {
		t.Fatalf("DeleteBySource: %v", err)
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Fatalf("got %d streams, want 1", len(list))
	}
	if list[0].ID != "s3" {
		t.Errorf("remaining stream ID = %q, want %q", list[0].ID, "s3")
	}
}

func TestStreamStore_DeleteStaleBySource(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "a1", Name: "Keep"},
		{ID: "s2", SourceType: "m3u", SourceID: "a1", Name: "Delete"},
		{ID: "s3", SourceType: "m3u", SourceID: "a1", Name: "Also Delete"},
		{ID: "s4", SourceType: "m3u", SourceID: "a2", Name: "Other Source"},
	}
	s.BulkUpsert(ctx, streams)

	deleted, err := s.DeleteStaleBySource(ctx, "m3u", "a1", []string{"s1"})
	if err != nil {
		t.Fatalf("DeleteStaleBySource: %v", err)
	}

	sort.Strings(deleted)
	if len(deleted) != 2 {
		t.Fatalf("got %d deleted, want 2", len(deleted))
	}
	if deleted[0] != "s2" || deleted[1] != "s3" {
		t.Errorf("deleted = %v, want [s2 s3]", deleted)
	}

	list, _ := s.List(ctx)
	if len(list) != 2 {
		t.Fatalf("got %d streams, want 2", len(list))
	}
}

func TestStreamStore_List(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", Name: "One"},
		{ID: "s2", Name: "Two"},
		{ID: "s3", Name: "Three"},
	}
	s.BulkUpsert(ctx, streams)

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("got %d streams, want 3", len(list))
	}
}

func TestStreamStore_GetUnknownID(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error for unknown ID, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unknown ID, got %+v", got)
	}
}

func TestStreamStore_SaveIsNoop(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	if err := s.Save(); err != nil {
		t.Errorf("Save should be no-op, got: %v", err)
	}
}

func TestSettingsStore_SetAndGet(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SettingsStore()
	ctx := context.Background()

	err := s.Set(ctx, "theme", "dark")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, err := s.Get(ctx, "theme")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "dark" {
		t.Errorf("got %q, want %q", val, "dark")
	}
}

func TestSettingsStore_GetUnknownKey(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SettingsStore()
	ctx := context.Background()

	val, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error for unknown key, got: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for unknown key, got %q", val)
	}
}

func TestSettingsStore_List(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SettingsStore()
	ctx := context.Background()

	s.Set(ctx, "theme", "dark")
	s.Set(ctx, "language", "en")

	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d settings, want 2", len(all))
	}
	if all["theme"] != "dark" {
		t.Errorf("theme = %q, want %q", all["theme"], "dark")
	}
	if all["language"] != "en" {
		t.Errorf("language = %q, want %q", all["language"], "en")
	}
}

func TestSettingsStore_OverwriteExistingKey(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SettingsStore()
	ctx := context.Background()

	s.Set(ctx, "theme", "dark")
	s.Set(ctx, "theme", "light")

	val, _ := s.Get(ctx, "theme")
	if val != "light" {
		t.Errorf("got %q, want %q", val, "light")
	}

	all, _ := s.List(ctx)
	if len(all) != 1 {
		t.Errorf("got %d settings, want 1 (no duplicates)", len(all))
	}
}

func TestDataSurvivesCloseAndReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ctx := context.Background()

	db.StreamStore().BulkUpsert(ctx, []media.Stream{
		{ID: "s1", Name: "Persisted Stream", SourceType: "m3u", SourceID: "a1"},
	})
	db.SettingsStore().Set(ctx, "persist_key", "persist_value")

	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()

	got, err := db2.StreamStore().Get(ctx, "s1")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got == nil {
		t.Fatal("stream should persist across close/reopen")
	}
	if got.Name != "Persisted Stream" {
		t.Errorf("got name %q, want %q", got.Name, "Persisted Stream")
	}

	val, err := db2.SettingsStore().Get(ctx, "persist_key")
	if err != nil {
		t.Fatalf("Get setting after reopen: %v", err)
	}
	if val != "persist_value" {
		t.Errorf("got %q, want %q", val, "persist_value")
	}
}

func TestChannelStore_CreateAndGet(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ChannelStore()
	ctx := context.Background()

	ch := &channel.Channel{
		ID:        "ch-1",
		Name:      "BBC One",
		Number:    1,
		GroupID:   "g-1",
		StreamIDs: []string{"s-1", "s-2"},
		IsEnabled: true,
	}

	if err := s.Create(ctx, ch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "ch-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Name != "BBC One" {
		t.Errorf("Name = %q, want %q", got.Name, "BBC One")
	}
	if got.Number != 1 {
		t.Errorf("Number = %d, want 1", got.Number)
	}
	if len(got.StreamIDs) != 2 {
		t.Errorf("StreamIDs len = %d, want 2", len(got.StreamIDs))
	}
}

func TestChannelStore_GetUnknownID(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ChannelStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestChannelStore_List(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", Name: "One"})
	s.Create(ctx, &channel.Channel{ID: "ch-2", Name: "Two"})
	s.Create(ctx, &channel.Channel{ID: "ch-3", Name: "Three"})

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("got %d channels, want 3", len(list))
	}
}

func TestChannelStore_Update(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", Name: "Old Name"})
	s.Update(ctx, &channel.Channel{ID: "ch-1", Name: "New Name"})

	got, _ := s.Get(ctx, "ch-1")
	if got.Name != "New Name" {
		t.Errorf("Name = %q, want %q", got.Name, "New Name")
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Errorf("got %d channels, want 1", len(list))
	}
}

func TestChannelStore_Delete(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", Name: "One"})
	s.Create(ctx, &channel.Channel{ID: "ch-2", Name: "Two"})

	if err := s.Delete(ctx, "ch-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Fatalf("got %d channels, want 1", len(list))
	}
	if list[0].ID != "ch-2" {
		t.Errorf("remaining ID = %q, want %q", list[0].ID, "ch-2")
	}
}

func TestChannelStore_AssignStreams(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", StreamIDs: []string{"s-old"}})

	if err := s.AssignStreams(ctx, "ch-1", []string{"s-1", "s-2", "s-3"}); err != nil {
		t.Fatalf("AssignStreams: %v", err)
	}

	got, _ := s.Get(ctx, "ch-1")
	if len(got.StreamIDs) != 3 {
		t.Fatalf("StreamIDs len = %d, want 3", len(got.StreamIDs))
	}

	sort.Strings(got.StreamIDs)
	if got.StreamIDs[0] != "s-1" || got.StreamIDs[1] != "s-2" || got.StreamIDs[2] != "s-3" {
		t.Errorf("StreamIDs = %v, want [s-1 s-2 s-3]", got.StreamIDs)
	}
}

func TestChannelStore_AssignStreamsUnknownChannel(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ChannelStore()
	ctx := context.Background()

	err := s.AssignStreams(ctx, "nonexistent", []string{"s-1"})
	if err != nil {
		t.Fatalf("AssignStreams on unknown channel should not error: %v", err)
	}
}

func TestChannelStore_RemoveStreamMappings(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ChannelStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Channel{ID: "ch-1", StreamIDs: []string{"s-1", "s-2", "s-3"}})
	s.Create(ctx, &channel.Channel{ID: "ch-2", StreamIDs: []string{"s-2", "s-4"}})

	if err := s.RemoveStreamMappings(ctx, []string{"s-2", "s-3"}); err != nil {
		t.Fatalf("RemoveStreamMappings: %v", err)
	}

	ch1, _ := s.Get(ctx, "ch-1")
	if len(ch1.StreamIDs) != 1 || ch1.StreamIDs[0] != "s-1" {
		t.Errorf("ch-1 StreamIDs = %v, want [s-1]", ch1.StreamIDs)
	}

	ch2, _ := s.Get(ctx, "ch-2")
	if len(ch2.StreamIDs) != 1 || ch2.StreamIDs[0] != "s-4" {
		t.Errorf("ch-2 StreamIDs = %v, want [s-4]", ch2.StreamIDs)
	}
}

func TestGroupStore_CreateAndList(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.GroupStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Group{ID: "g-1", Name: "News"})
	s.Create(ctx, &channel.Group{ID: "g-2", Name: "Sports"})

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("got %d groups, want 2", len(list))
	}
}

func TestGroupStore_Delete(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.GroupStore()
	ctx := context.Background()

	s.Create(ctx, &channel.Group{ID: "g-1", Name: "News"})
	s.Create(ctx, &channel.Group{ID: "g-2", Name: "Sports"})

	if err := s.Delete(ctx, "g-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Fatalf("got %d groups, want 1", len(list))
	}
	if list[0].ID != "g-2" {
		t.Errorf("remaining ID = %q, want %q", list[0].ID, "g-2")
	}
}

func TestEPGSourceStore_CreateAndGet(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.EPGSourceStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.EPGSourceStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.EPGSourceStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.EPGSourceStore()
	ctx := context.Background()

	s.Create(ctx, &epg.Source{ID: "epg-1", Name: "Old"})
	s.Update(ctx, &epg.Source{ID: "epg-1", Name: "New"})

	got, _ := s.Get(ctx, "epg-1")
	if got.Name != "New" {
		t.Errorf("Name = %q, want %q", got.Name, "New")
	}
}

func TestEPGSourceStore_Delete(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.EPGSourceStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ProgramStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ProgramStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ProgramStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ProgramStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ProgramStore()
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
	db, _ := tempDB(t)
	defer db.Close()

	s := db.ProgramStore()
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

func TestRecordingStore_CreateAndGet(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.RecordingStore()
	ctx := context.Background()

	r := &recording.Recording{
		ID:         "rec-1",
		StreamID:   "s-1",
		StreamName: "BBC One",
		ChannelID:  "ch-1",
		Title:      "News at Ten",
		UserID:     "user-1",
		Status:     recording.StatusScheduled,
		StartedAt:  time.Now().Truncate(time.Millisecond),
	}

	if err := s.Create(ctx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "rec-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Title != "News at Ten" {
		t.Errorf("Title = %q, want %q", got.Title, "News at Ten")
	}
	if got.Status != recording.StatusScheduled {
		t.Errorf("Status = %q, want %q", got.Status, recording.StatusScheduled)
	}
}

func TestRecordingStore_GetUnknownID(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.RecordingStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestRecordingStore_Update(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.RecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1", Status: recording.StatusScheduled})
	s.Update(ctx, &recording.Recording{ID: "rec-1", Status: recording.StatusRecording})

	got, _ := s.Get(ctx, "rec-1")
	if got.Status != recording.StatusRecording {
		t.Errorf("Status = %q, want %q", got.Status, recording.StatusRecording)
	}
}

func TestRecordingStore_Delete(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.RecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1"})
	s.Create(ctx, &recording.Recording{ID: "rec-2"})

	if err := s.Delete(ctx, "rec-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, _ := s.Get(ctx, "rec-1")
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}

	got2, _ := s.Get(ctx, "rec-2")
	if got2 == nil {
		t.Error("rec-2 should still exist")
	}
}

func TestRecordingStore_ListAdminSeesAll(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.RecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1", UserID: "user-1"})
	s.Create(ctx, &recording.Recording{ID: "rec-2", UserID: "user-2"})
	s.Create(ctx, &recording.Recording{ID: "rec-3", UserID: "user-1"})

	list, err := s.List(ctx, "user-1", true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("admin got %d recordings, want 3", len(list))
	}
}

func TestRecordingStore_ListNonAdminSeesOwn(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.RecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1", UserID: "user-1"})
	s.Create(ctx, &recording.Recording{ID: "rec-2", UserID: "user-2"})
	s.Create(ctx, &recording.Recording{ID: "rec-3", UserID: "user-1"})

	list, err := s.List(ctx, "user-1", false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("non-admin got %d recordings, want 2", len(list))
	}

	ids := []string{list[0].ID, list[1].ID}
	sort.Strings(ids)
	if ids[0] != "rec-1" || ids[1] != "rec-3" {
		t.Errorf("IDs = %v, want [rec-1 rec-3]", ids)
	}
}

func TestRecordingStore_ListByStatus(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.RecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1", Status: recording.StatusScheduled})
	s.Create(ctx, &recording.Recording{ID: "rec-2", Status: recording.StatusRecording})
	s.Create(ctx, &recording.Recording{ID: "rec-3", Status: recording.StatusScheduled})
	s.Create(ctx, &recording.Recording{ID: "rec-4", Status: recording.StatusCompleted})

	list, err := s.ListByStatus(ctx, recording.StatusScheduled)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d recordings, want 2", len(list))
	}

	ids := []string{list[0].ID, list[1].ID}
	sort.Strings(ids)
	if ids[0] != "rec-1" || ids[1] != "rec-3" {
		t.Errorf("IDs = %v, want [rec-1 rec-3]", ids)
	}
}

func TestRecordingStore_ListScheduled(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.RecordingStore()
	ctx := context.Background()

	s.Create(ctx, &recording.Recording{ID: "rec-1", Status: recording.StatusScheduled})
	s.Create(ctx, &recording.Recording{ID: "rec-2", Status: recording.StatusRecording})
	s.Create(ctx, &recording.Recording{ID: "rec-3", Status: recording.StatusScheduled})

	list, err := s.ListScheduled(ctx)
	if err != nil {
		t.Fatalf("ListScheduled: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("got %d scheduled, want 2", len(list))
	}
}

func TestUserStore_CreateAndGet(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	user := &auth.User{
		ID:       "u-1",
		Username: "alice",
		IsAdmin:  true,
		Role:     auth.RoleAdmin,
	}

	if err := s.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "u-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("Username = %q, want %q", got.Username, "alice")
	}
	if got.Role != auth.RoleAdmin {
		t.Errorf("Role = %q, want %q", got.Role, auth.RoleAdmin)
	}
}

func TestUserStore_GetUnknownID(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	_, err := s.Get(ctx, "nonexistent")
	if err != auth.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_GetByUsername(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	s.Create(ctx, &auth.User{ID: "u-1", Username: "alice", Role: auth.RoleAdmin})
	s.Create(ctx, &auth.User{ID: "u-2", Username: "bob", Role: auth.RoleStandard})

	got, err := s.GetByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if got.ID != "u-2" {
		t.Errorf("ID = %q, want %q", got.ID, "u-2")
	}
}

func TestUserStore_GetByUsernameNotFound(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	_, err := s.GetByUsername(ctx, "nobody")
	if err != auth.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_List(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	s.Create(ctx, &auth.User{ID: "u-1", Username: "alice"})
	s.Create(ctx, &auth.User{ID: "u-2", Username: "bob"})

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("got %d users, want 2", len(list))
	}
}

func TestUserStore_DuplicateUsername(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	s.Create(ctx, &auth.User{ID: "u-1", Username: "alice"})
	err := s.Create(ctx, &auth.User{ID: "u-2", Username: "alice"})
	if err != auth.ErrUsernameExists {
		t.Errorf("expected ErrUsernameExists, got %v", err)
	}
}

func TestUserStore_Delete(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	s.Create(ctx, &auth.User{ID: "u-1", Username: "alice"})

	if err := s.Delete(ctx, "u-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.Get(ctx, "u-1")
	if err != auth.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound after delete, got %v", err)
	}
}

func TestUserStore_DeleteNotFound(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	err := s.Delete(ctx, "nonexistent")
	if err != auth.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_UpdatePassword(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	s.Create(ctx, &auth.User{ID: "u-1", Username: "alice"})

	if err := s.UpdatePassword(ctx, "u-1", "$2a$10$fakehash"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}

	hash, err := s.GetPasswordHash(ctx, "u-1")
	if err != nil {
		t.Fatalf("GetPasswordHash: %v", err)
	}
	if hash != "$2a$10$fakehash" {
		t.Errorf("hash = %q, want %q", hash, "$2a$10$fakehash")
	}
}

func TestUserStore_UpdatePasswordNotFound(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	err := s.UpdatePassword(ctx, "nonexistent", "hash")
	if err != auth.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_GetPasswordHashNotFound(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.UserStore()
	ctx := context.Background()

	_, err := s.GetPasswordHash(ctx, "nonexistent")
	if err != auth.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestSettingsStore_SeedDefaults(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SettingsStore()
	ctx := context.Background()

	defaults := map[string]string{
		"default_hwaccel":        "none",
		"default_video_codec":    "copy",
		"default_decode_hwaccel": "",
		"dlna_enabled":           "true",
		"delivery":               "mse",
		"container":              "mp4",
	}
	for k, v := range defaults {
		existing, _ := s.Get(ctx, k)
		if existing == "" {
			s.Set(ctx, k, v)
		}
	}

	val, _ := s.Get(ctx, "default_hwaccel")
	if val != "none" {
		t.Errorf("default_hwaccel = %q, want %q", val, "none")
	}
	val, _ = s.Get(ctx, "dlna_enabled")
	if val != "true" {
		t.Errorf("dlna_enabled = %q, want %q", val, "true")
	}

	s.Set(ctx, "default_hwaccel", "vaapi")

	for k, v := range defaults {
		existing, _ := s.Get(ctx, k)
		if existing == "" {
			s.Set(ctx, k, v)
		}
	}

	val, _ = s.Get(ctx, "default_hwaccel")
	if val != "vaapi" {
		t.Errorf("default_hwaccel after re-seed = %q, want %q (should not overwrite)", val, "vaapi")
	}
}

func TestSourceConfigStore_CreateAndGet(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SourceConfigStore()
	ctx := context.Background()

	sc := &sourceconfig.SourceConfig{
		ID:        "src-1",
		Type:      "m3u",
		Name:      "UK IPTV",
		IsEnabled: true,
		Config: map[string]string{
			"url":      "http://example.com/playlist.m3u",
			"username": "user1",
			"password": "pass1",
		},
	}

	if err := s.Create(ctx, sc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "src-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Name != "UK IPTV" {
		t.Errorf("Name = %q, want %q", got.Name, "UK IPTV")
	}
	if got.Type != "m3u" {
		t.Errorf("Type = %q, want %q", got.Type, "m3u")
	}
	if !got.IsEnabled {
		t.Error("IsEnabled should be true")
	}
	if got.Config["url"] != "http://example.com/playlist.m3u" {
		t.Errorf("Config[url] = %q, want %q", got.Config["url"], "http://example.com/playlist.m3u")
	}
}

func TestSourceConfigStore_GetUnknownID(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SourceConfigStore()
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get should not error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestSourceConfigStore_List(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SourceConfigStore()
	ctx := context.Background()

	s.Create(ctx, &sourceconfig.SourceConfig{ID: "src-1", Type: "m3u", Name: "One", Config: map[string]string{}})
	s.Create(ctx, &sourceconfig.SourceConfig{ID: "src-2", Type: "hdhr", Name: "Two", Config: map[string]string{}})
	s.Create(ctx, &sourceconfig.SourceConfig{ID: "src-3", Type: "m3u", Name: "Three", Config: map[string]string{}})

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("got %d sources, want 3", len(list))
	}
}

func TestSourceConfigStore_ListByType(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SourceConfigStore()
	ctx := context.Background()

	s.Create(ctx, &sourceconfig.SourceConfig{ID: "src-1", Type: "m3u", Name: "One", Config: map[string]string{}})
	s.Create(ctx, &sourceconfig.SourceConfig{ID: "src-2", Type: "hdhr", Name: "Two", Config: map[string]string{}})
	s.Create(ctx, &sourceconfig.SourceConfig{ID: "src-3", Type: "m3u", Name: "Three", Config: map[string]string{}})

	list, err := s.ListByType(ctx, "m3u")
	if err != nil {
		t.Fatalf("ListByType: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d m3u sources, want 2", len(list))
	}

	names := []string{list[0].Name, list[1].Name}
	sort.Strings(names)
	if names[0] != "One" || names[1] != "Three" {
		t.Errorf("names = %v, want [One Three]", names)
	}
}

func TestSourceConfigStore_Update(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SourceConfigStore()
	ctx := context.Background()

	s.Create(ctx, &sourceconfig.SourceConfig{ID: "src-1", Type: "m3u", Name: "Old Name", Config: map[string]string{"url": "http://old.com"}})

	err := s.Update(ctx, &sourceconfig.SourceConfig{ID: "src-1", Type: "m3u", Name: "New Name", Config: map[string]string{"url": "http://new.com"}})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := s.Get(ctx, "src-1")
	if got.Name != "New Name" {
		t.Errorf("Name = %q, want %q", got.Name, "New Name")
	}
	if got.Config["url"] != "http://new.com" {
		t.Errorf("Config[url] = %q, want %q", got.Config["url"], "http://new.com")
	}
}

func TestSourceConfigStore_UpdateNotFound(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SourceConfigStore()
	ctx := context.Background()

	err := s.Update(ctx, &sourceconfig.SourceConfig{ID: "nonexistent", Config: map[string]string{}})
	if err != ErrSourceConfigNotFound {
		t.Errorf("expected ErrSourceConfigNotFound, got %v", err)
	}
}

func TestSourceConfigStore_Delete(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.SourceConfigStore()
	ctx := context.Background()

	s.Create(ctx, &sourceconfig.SourceConfig{ID: "src-1", Config: map[string]string{}})
	s.Create(ctx, &sourceconfig.SourceConfig{ID: "src-2", Config: map[string]string{}})

	if err := s.Delete(ctx, "src-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, _ := s.Get(ctx, "src-1")
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}

	list, _ := s.List(ctx)
	if len(list) != 1 {
		t.Errorf("got %d sources, want 1", len(list))
	}
}

func TestSourceConfigStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ctx := context.Background()
	db.SourceConfigStore().Create(ctx, &sourceconfig.SourceConfig{
		ID:     "src-1",
		Type:   "m3u",
		Name:   "Persisted",
		Config: map[string]string{"url": "http://example.com"},
	})
	db.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()

	got, err := db2.SourceConfigStore().Get(ctx, "src-1")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got == nil {
		t.Fatal("source config should persist across close/reopen")
	}
	if got.Name != "Persisted" {
		t.Errorf("Name = %q, want %q", got.Name, "Persisted")
	}
}
