package bolt

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

func TestStreamStore_ListBySourceIsolation(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "src-1", Name: "A"},
		{ID: "s2", SourceType: "m3u", SourceID: "src-1", Name: "B"},
		{ID: "s3", SourceType: "m3u", SourceID: "src-2", Name: "C"},
		{ID: "s4", SourceType: "satip", SourceID: "src-1", Name: "D"},
		{ID: "s5", SourceType: "satip", SourceID: "src-3", Name: "E"},
		{ID: "s6", SourceType: "xtream", SourceID: "src-4", Name: "F"},
	}
	s.BulkUpsert(ctx, streams)

	tests := []struct {
		sourceType string
		sourceID   string
		wantIDs    []string
	}{
		{"m3u", "src-1", []string{"s1", "s2"}},
		{"m3u", "src-2", []string{"s3"}},
		{"satip", "src-1", []string{"s4"}},
		{"satip", "src-3", []string{"s5"}},
		{"xtream", "src-4", []string{"s6"}},
		{"m3u", "nonexistent", nil},
		{"hdhr", "src-1", nil},
	}

	for _, tt := range tests {
		got, err := s.ListBySource(ctx, tt.sourceType, tt.sourceID)
		if err != nil {
			t.Fatalf("ListBySource(%s, %s): %v", tt.sourceType, tt.sourceID, err)
		}
		var gotIDs []string
		for _, g := range got {
			gotIDs = append(gotIDs, g.ID)
		}
		sort.Strings(gotIDs)
		sort.Strings(tt.wantIDs)

		if len(gotIDs) != len(tt.wantIDs) {
			t.Errorf("ListBySource(%s, %s): got %v, want %v", tt.sourceType, tt.sourceID, gotIDs, tt.wantIDs)
			continue
		}
		for i := range gotIDs {
			if gotIDs[i] != tt.wantIDs[i] {
				t.Errorf("ListBySource(%s, %s): got %v, want %v", tt.sourceType, tt.sourceID, gotIDs, tt.wantIDs)
				break
			}
		}
	}
}

func TestStreamStore_CountBySource(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "src-1", Name: "A"},
		{ID: "s2", SourceType: "m3u", SourceID: "src-1", Name: "B"},
		{ID: "s3", SourceType: "m3u", SourceID: "src-1", Name: "C"},
		{ID: "s4", SourceType: "m3u", SourceID: "src-2", Name: "D"},
		{ID: "s5", SourceType: "satip", SourceID: "src-1", Name: "E"},
	}
	s.BulkUpsert(ctx, streams)

	count, err := s.CountBySource(ctx, "m3u", "src-1")
	if err != nil {
		t.Fatalf("CountBySource: %v", err)
	}
	if count != 3 {
		t.Errorf("CountBySource(m3u, src-1) = %d, want 3", count)
	}

	count, err = s.CountBySource(ctx, "m3u", "src-2")
	if err != nil {
		t.Fatalf("CountBySource: %v", err)
	}
	if count != 1 {
		t.Errorf("CountBySource(m3u, src-2) = %d, want 1", count)
	}

	count, err = s.CountBySource(ctx, "satip", "src-1")
	if err != nil {
		t.Fatalf("CountBySource: %v", err)
	}
	if count != 1 {
		t.Errorf("CountBySource(satip, src-1) = %d, want 1", count)
	}

	count, err = s.CountBySource(ctx, "m3u", "nonexistent")
	if err != nil {
		t.Fatalf("CountBySource: %v", err)
	}
	if count != 0 {
		t.Errorf("CountBySource(m3u, nonexistent) = %d, want 0", count)
	}
}

func TestStreamStore_DeleteStaleBySourceKeepsCorrectStreams(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "src-1", Name: "Keep1"},
		{ID: "s2", SourceType: "m3u", SourceID: "src-1", Name: "Keep2"},
		{ID: "s3", SourceType: "m3u", SourceID: "src-1", Name: "Delete1"},
		{ID: "s4", SourceType: "m3u", SourceID: "src-1", Name: "Delete2"},
		{ID: "s5", SourceType: "m3u", SourceID: "src-2", Name: "OtherSource"},
		{ID: "s6", SourceType: "satip", SourceID: "src-1", Name: "DiffType"},
	}
	s.BulkUpsert(ctx, streams)

	deleted, err := s.DeleteStaleBySource(ctx, "m3u", "src-1", []string{"s1", "s2"})
	if err != nil {
		t.Fatalf("DeleteStaleBySource: %v", err)
	}

	sort.Strings(deleted)
	if len(deleted) != 2 || deleted[0] != "s3" || deleted[1] != "s4" {
		t.Errorf("deleted = %v, want [s3 s4]", deleted)
	}

	kept, _ := s.ListBySource(ctx, "m3u", "src-1")
	if len(kept) != 2 {
		t.Fatalf("kept %d streams for m3u/src-1, want 2", len(kept))
	}
	var keptIDs []string
	for _, k := range kept {
		keptIDs = append(keptIDs, k.ID)
	}
	sort.Strings(keptIDs)
	if keptIDs[0] != "s1" || keptIDs[1] != "s2" {
		t.Errorf("kept IDs = %v, want [s1 s2]", keptIDs)
	}

	other, _ := s.ListBySource(ctx, "m3u", "src-2")
	if len(other) != 1 || other[0].ID != "s5" {
		t.Errorf("other source should be unaffected, got %v", other)
	}

	satip, _ := s.ListBySource(ctx, "satip", "src-1")
	if len(satip) != 1 || satip[0].ID != "s6" {
		t.Errorf("satip source should be unaffected, got %v", satip)
	}

	for _, id := range []string{"s3", "s4"} {
		got, _ := s.Get(ctx, id)
		if got != nil {
			t.Errorf("deleted stream %s still accessible via Get", id)
		}
	}

	for _, id := range []string{"s1", "s2", "s5", "s6"} {
		got, _ := s.Get(ctx, id)
		if got == nil {
			t.Errorf("kept stream %s not accessible via Get", id)
		}
	}
}

func TestStreamStore_MigrateFromFlatKeys(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketStreams)
		if err != nil {
			return err
		}

		streams := []media.Stream{
			{ID: "flat-1", SourceType: "m3u", SourceID: "src-a", Name: "Stream One"},
			{ID: "flat-2", SourceType: "m3u", SourceID: "src-a", Name: "Stream Two"},
			{ID: "flat-3", SourceType: "satip", SourceID: "src-b", Name: "Stream Three"},
		}
		for _, s := range streams {
			data, _ := json.Marshal(s)
			b.Put([]byte(s.ID), data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed flat keys: %v", err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d streams after migration, want 3", len(all))
	}

	got, err := s.Get(ctx, "flat-1")
	if err != nil {
		t.Fatalf("Get flat-1: %v", err)
	}
	if got == nil {
		t.Fatal("flat-1 not found after migration")
	}
	if got.Name != "Stream One" {
		t.Errorf("Name = %q, want %q", got.Name, "Stream One")
	}

	m3u, err := s.ListBySource(ctx, "m3u", "src-a")
	if err != nil {
		t.Fatalf("ListBySource: %v", err)
	}
	if len(m3u) != 2 {
		t.Errorf("ListBySource(m3u, src-a) = %d, want 2", len(m3u))
	}

	satip, err := s.ListBySource(ctx, "satip", "src-b")
	if err != nil {
		t.Fatalf("ListBySource: %v", err)
	}
	if len(satip) != 1 {
		t.Errorf("ListBySource(satip, src-b) = %d, want 1", len(satip))
	}

	count, _ := s.CountBySource(ctx, "m3u", "src-a")
	if count != 2 {
		t.Errorf("CountBySource after migration = %d, want 2", count)
	}
}

func TestStreamStore_MigrateFromFlatKeysIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate2.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketStreams)
		if err != nil {
			return err
		}
		s := media.Stream{ID: "s1", SourceType: "m3u", SourceID: "src-1", Name: "Test"}
		data, _ := json.Marshal(s)
		b.Put([]byte(s.ID), data)
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	s := db.StreamStore()
	ctx := context.Background()

	list1, _ := s.List(ctx)
	if len(list1) != 1 {
		t.Fatalf("first migration: got %d, want 1", len(list1))
	}
	db.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()

	s2 := db2.StreamStore()
	list2, _ := s2.List(ctx)
	if len(list2) != 1 {
		t.Fatalf("second migration: got %d, want 1 (should be idempotent)", len(list2))
	}

	got, _ := s2.Get(ctx, "s1")
	if got == nil || got.Name != "Test" {
		t.Errorf("data should survive double migration, got %+v", got)
	}
}

func TestStreamStore_DeleteStaleBySourceEmptyKeepList(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	s.BulkUpsert(ctx, []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "src-1", Name: "A"},
		{ID: "s2", SourceType: "m3u", SourceID: "src-1", Name: "B"},
	})

	deleted, err := s.DeleteStaleBySource(ctx, "m3u", "src-1", nil)
	if err != nil {
		t.Fatalf("DeleteStaleBySource: %v", err)
	}

	sort.Strings(deleted)
	if len(deleted) != 2 || deleted[0] != "s1" || deleted[1] != "s2" {
		t.Errorf("deleted = %v, want [s1 s2]", deleted)
	}

	count, _ := s.CountBySource(ctx, "m3u", "src-1")
	if count != 0 {
		t.Errorf("count after delete all = %d, want 0", count)
	}
}

func TestStreamStore_CountBySourceAfterBulkOperations(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	s.BulkUpsert(ctx, []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "src-1", Name: "A"},
		{ID: "s2", SourceType: "m3u", SourceID: "src-1", Name: "B"},
		{ID: "s3", SourceType: "m3u", SourceID: "src-1", Name: "C"},
	})

	count, _ := s.CountBySource(ctx, "m3u", "src-1")
	if count != 3 {
		t.Fatalf("initial count = %d, want 3", count)
	}

	s.BulkUpsert(ctx, []media.Stream{
		{ID: "s1", SourceType: "m3u", SourceID: "src-1", Name: "A Updated"},
	})

	count, _ = s.CountBySource(ctx, "m3u", "src-1")
	if count != 3 {
		t.Errorf("count after upsert = %d, want 3 (upsert should not duplicate)", count)
	}

	s.DeleteStaleBySource(ctx, "m3u", "src-1", []string{"s1"})

	count, _ = s.CountBySource(ctx, "m3u", "src-1")
	if count != 1 {
		t.Errorf("count after stale delete = %d, want 1", count)
	}
}

func TestStreamStore_ListBySourceAndType(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "xtream", SourceID: "src-1", Name: "Movie A", VODType: "movie"},
		{ID: "s2", SourceType: "xtream", SourceID: "src-1", Name: "Movie B", VODType: "movie"},
		{ID: "s3", SourceType: "xtream", SourceID: "src-1", Name: "Series A", VODType: "series"},
		{ID: "s4", SourceType: "xtream", SourceID: "src-1", Name: "Live A"},
		{ID: "s5", SourceType: "m3u", SourceID: "src-2", Name: "Movie C", VODType: "movie"},
	}
	s.BulkUpsert(ctx, streams)

	movies, err := s.ListBySourceAndType(ctx, "xtream", "src-1", "movie")
	if err != nil {
		t.Fatalf("ListBySourceAndType: %v", err)
	}
	if len(movies) != 2 {
		t.Errorf("movies = %d, want 2", len(movies))
	}

	series, err := s.ListBySourceAndType(ctx, "xtream", "src-1", "series")
	if err != nil {
		t.Fatalf("ListBySourceAndType: %v", err)
	}
	if len(series) != 1 {
		t.Errorf("series = %d, want 1", len(series))
	}

	live, err := s.ListBySourceAndType(ctx, "xtream", "src-1", "")
	if err != nil {
		t.Fatalf("ListBySourceAndType: %v", err)
	}
	if len(live) != 1 {
		t.Errorf("live = %d, want 1", len(live))
	}

	all, err := s.ListBySource(ctx, "xtream", "src-1")
	if err != nil {
		t.Fatalf("ListBySource: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("all = %d, want 4", len(all))
	}

	m3uMovies, err := s.ListBySourceAndType(ctx, "m3u", "src-2", "movie")
	if err != nil {
		t.Fatalf("ListBySourceAndType: %v", err)
	}
	if len(m3uMovies) != 1 {
		t.Errorf("m3u movies = %d, want 1", len(m3uMovies))
	}

	none, err := s.ListBySourceAndType(ctx, "xtream", "src-1", "episode")
	if err != nil {
		t.Fatalf("ListBySourceAndType: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("episode = %d, want 0", len(none))
	}
}

func TestStreamStore_CountBySourceAndType(t *testing.T) {
	db, _ := tempDB(t)
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	streams := []media.Stream{
		{ID: "s1", SourceType: "xtream", SourceID: "src-1", VODType: "movie"},
		{ID: "s2", SourceType: "xtream", SourceID: "src-1", VODType: "movie"},
		{ID: "s3", SourceType: "xtream", SourceID: "src-1", VODType: "series"},
		{ID: "s4", SourceType: "xtream", SourceID: "src-1"},
	}
	s.BulkUpsert(ctx, streams)

	count, _ := s.CountBySourceAndType(ctx, "xtream", "src-1", "movie")
	if count != 2 {
		t.Errorf("movie count = %d, want 2", count)
	}

	count, _ = s.CountBySourceAndType(ctx, "xtream", "src-1", "series")
	if count != 1 {
		t.Errorf("series count = %d, want 1", count)
	}

	count, _ = s.CountBySourceAndType(ctx, "xtream", "src-1", "")
	if count != 1 {
		t.Errorf("live count = %d, want 1", count)
	}

	total, _ := s.CountBySource(ctx, "xtream", "src-1")
	if total != 4 {
		t.Errorf("total count = %d, want 4", total)
	}
}

func TestStreamStore_MigrateFrom4SegmentKeys(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/migrate4seg.db"

	raw, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}

	err = raw.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketStreams)
		if err != nil {
			return err
		}

		oldSchema := keyenc.NewSchema[struct {
			Kind       string `key:"streams"`
			SourceType string
			SourceID   string
			StreamID   string
		}]()

		streams := []media.Stream{
			{ID: "m1", SourceType: "xtream", SourceID: "src-1", Name: "Movie", VODType: "movie"},
			{ID: "s1", SourceType: "xtream", SourceID: "src-1", Name: "Series", VODType: "series"},
			{ID: "l1", SourceType: "m3u", SourceID: "src-2", Name: "Live"},
		}
		for _, s := range streams {
			data, _ := json.Marshal(s)
			key := oldSchema.Key(struct {
				Kind       string `key:"streams"`
				SourceType string
				SourceID   string
				StreamID   string
			}{
				SourceType: s.SourceType,
				SourceID:   s.SourceID,
				StreamID:   s.ID,
			})
			b.Put(key, data)
			b.Put(keyenc.Reverse("streamidx", s.ID), key)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed old keys: %v", err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	s := db.StreamStore()
	ctx := context.Background()

	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d streams after migration, want 3", len(all))
	}

	got, err := s.Get(ctx, "m1")
	if err != nil || got == nil {
		t.Fatalf("Get m1: err=%v, got=%v", err, got)
	}
	if got.Name != "Movie" {
		t.Errorf("Name = %q, want %q", got.Name, "Movie")
	}

	movies, err := s.ListBySourceAndType(ctx, "xtream", "src-1", "movie")
	if err != nil {
		t.Fatalf("ListBySourceAndType: %v", err)
	}
	if len(movies) != 1 || movies[0].ID != "m1" {
		t.Errorf("movies after migration = %v, want [m1]", movies)
	}

	live, err := s.ListBySourceAndType(ctx, "m3u", "src-2", "")
	if err != nil {
		t.Fatalf("ListBySourceAndType: %v", err)
	}
	if len(live) != 1 || live[0].ID != "l1" {
		t.Errorf("live after migration = %v, want [l1]", live)
	}

	allXtream, err := s.ListBySource(ctx, "xtream", "src-1")
	if err != nil {
		t.Fatalf("ListBySource: %v", err)
	}
	if len(allXtream) != 2 {
		t.Errorf("all xtream after migration = %d, want 2", len(allXtream))
	}
}
