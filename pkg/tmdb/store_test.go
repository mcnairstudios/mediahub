package tmdb

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	bbolt "go.etcd.io/bbolt"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := NewStore(db)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestEnqueueMetadata_CreatesEntry(t *testing.T) {
	s := testStore(t)

	err := s.EnqueueMetadata(QueueEntry{
		TMDBID:    603,
		MediaType: "movie",
		Status:    "resolving",
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("EnqueueMetadata: %v", err)
	}

	count, _ := s.QueueCount()
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	entry, err := s.PickOldestResolving()
	if err != nil {
		t.Fatalf("PickOldestResolving: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.TMDBID != 603 {
		t.Errorf("TMDBID = %d, want 603", entry.TMDBID)
	}
	if entry.MediaType != "movie" {
		t.Errorf("MediaType = %q, want %q", entry.MediaType, "movie")
	}
	if entry.Status != "resolving" {
		t.Errorf("Status = %q, want %q", entry.Status, "resolving")
	}
}

func TestEnqueueMetadata_OverwritesSameID(t *testing.T) {
	s := testStore(t)

	s.EnqueueMetadata(QueueEntry{TMDBID: 603, MediaType: "movie", Status: "resolving", CreatedAt: 1000})
	s.EnqueueMetadata(QueueEntry{TMDBID: 603, MediaType: "movie", Status: "resolving", CreatedAt: 2000})

	count, _ := s.QueueCount()
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestPickOldestResolving_EmptyQueue(t *testing.T) {
	s := testStore(t)

	entry, err := s.PickOldestResolving()
	if err != nil {
		t.Fatalf("PickOldestResolving: %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil, got %+v", entry)
	}
}

func TestPickOldestResolving_ReturnsOldest(t *testing.T) {
	s := testStore(t)

	s.EnqueueMetadata(QueueEntry{TMDBID: 300, MediaType: "movie", Status: "resolving", CreatedAt: 3000})
	s.EnqueueMetadata(QueueEntry{TMDBID: 100, MediaType: "movie", Status: "resolving", CreatedAt: 1000})
	s.EnqueueMetadata(QueueEntry{TMDBID: 200, MediaType: "series", Status: "resolving", CreatedAt: 2000})

	entry, _ := s.PickOldestResolving()
	if entry.TMDBID != 100 {
		t.Errorf("TMDBID = %d, want 100 (oldest by CreatedAt)", entry.TMDBID)
	}
}

func TestUpdateQueueEntry(t *testing.T) {
	s := testStore(t)

	s.EnqueueMetadata(QueueEntry{TMDBID: 1396, MediaType: "series", Status: "resolving", CreatedAt: 1000})

	err := s.UpdateQueueEntry(QueueEntry{
		TMDBID:      1396,
		MediaType:   "series",
		Status:      "resolving",
		SeasonsDone: []int{1, 2},
		CreatedAt:   1000,
	})
	if err != nil {
		t.Fatalf("UpdateQueueEntry: %v", err)
	}

	entry, _ := s.PickOldestResolving()
	if len(entry.SeasonsDone) != 2 {
		t.Fatalf("SeasonsDone len = %d, want 2", len(entry.SeasonsDone))
	}
	if entry.SeasonsDone[0] != 1 || entry.SeasonsDone[1] != 2 {
		t.Errorf("SeasonsDone = %v, want [1 2]", entry.SeasonsDone)
	}
}

func TestDeleteQueueEntry(t *testing.T) {
	s := testStore(t)

	s.EnqueueMetadata(QueueEntry{TMDBID: 603, MediaType: "movie", Status: "resolving", CreatedAt: 1000})
	if err := s.DeleteQueueEntry(603); err != nil {
		t.Fatalf("DeleteQueueEntry: %v", err)
	}

	count, _ := s.QueueCount()
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestQueueCount_Empty(t *testing.T) {
	s := testStore(t)

	count, err := s.QueueCount()
	if err != nil {
		t.Fatalf("QueueCount: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestWriteBlob_AndGetBlob(t *testing.T) {
	s := testStore(t)

	blob := MovieBlob{
		ID:        603,
		MediaType: "movie",
		Title:     "The Matrix",
		Year:      "1999",
		Rating:    8.7,
		Runtime:   136,
		Genres:    []string{"Action", "Science Fiction"},
		Cast:      []CastMember{{Name: "Keanu Reeves", Character: "Neo", TMDBID: 6384}},
	}
	data, _ := json.Marshal(blob)

	if err := s.WriteBlob(603, data); err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}

	got, err := s.GetBlob(603)
	if err != nil {
		t.Fatalf("GetBlob: %v", err)
	}
	if got == nil {
		t.Fatal("expected data, got nil")
	}

	var decoded MovieBlob
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Title != "The Matrix" {
		t.Errorf("Title = %q, want %q", decoded.Title, "The Matrix")
	}
	if decoded.Rating != 8.7 {
		t.Errorf("Rating = %v, want 8.7", decoded.Rating)
	}
	if len(decoded.Cast) != 1 || decoded.Cast[0].Name != "Keanu Reeves" {
		t.Errorf("Cast = %+v, want Keanu Reeves", decoded.Cast)
	}
}

func TestGetBlob_NotFound(t *testing.T) {
	s := testStore(t)

	data, err := s.GetBlob(999)
	if err != nil {
		t.Fatalf("GetBlob: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil, got %d bytes", len(data))
	}
}

func TestHasBlob(t *testing.T) {
	s := testStore(t)

	found, _ := s.HasBlob(603)
	if found {
		t.Error("HasBlob should be false before WriteBlob")
	}

	s.WriteBlob(603, []byte(`{"id":603}`))

	found, _ = s.HasBlob(603)
	if !found {
		t.Error("HasBlob should be true after WriteBlob")
	}
}

func TestDeleteBlob(t *testing.T) {
	s := testStore(t)

	s.WriteBlob(603, []byte(`{"id":603}`))
	if err := s.DeleteBlob(603); err != nil {
		t.Fatalf("DeleteBlob: %v", err)
	}

	found, _ := s.HasBlob(603)
	if found {
		t.Error("HasBlob should be false after DeleteBlob")
	}
}

func TestSeriesBlob(t *testing.T) {
	s := testStore(t)

	blob := SeriesBlob{
		ID:        1396,
		MediaType: "series",
		Name:      "Breaking Bad",
		Year:      "2008",
		Rating:    8.9,
		Genres:    []string{"Drama", "Crime"},
		Seasons: []SeasonBlob{
			{
				SeasonNumber: 1,
				Name:         "Season 1",
				EpisodeCount: 7,
				Episodes: []EpisodeBlob{
					{EpisodeNumber: 1, Name: "Pilot", Runtime: 58},
					{EpisodeNumber: 2, Name: "Cat's in the Bag...", Runtime: 48},
				},
			},
		},
	}
	data, _ := json.Marshal(blob)
	s.WriteBlob(1396, data)

	got, _ := s.GetBlob(1396)
	var decoded SeriesBlob
	json.Unmarshal(got, &decoded)

	if decoded.Name != "Breaking Bad" {
		t.Errorf("Name = %q, want %q", decoded.Name, "Breaking Bad")
	}
	if len(decoded.Seasons) != 1 {
		t.Fatalf("Seasons len = %d, want 1", len(decoded.Seasons))
	}
	if len(decoded.Seasons[0].Episodes) != 2 {
		t.Fatalf("Episodes len = %d, want 2", len(decoded.Seasons[0].Episodes))
	}
	if decoded.Seasons[0].Episodes[0].Name != "Pilot" {
		t.Errorf("Episode 1 Name = %q, want %q", decoded.Seasons[0].Episodes[0].Name, "Pilot")
	}
}

func TestEnqueueImage_AndPickImage(t *testing.T) {
	s := testStore(t)

	entry := ImageQueueEntry{
		TMDBPath: "/abc123.jpg",
		Size:     "w500",
	}
	if err := s.EnqueueImage("603/poster.jpg", entry); err != nil {
		t.Fatalf("EnqueueImage: %v", err)
	}

	count, _ := s.ImageQueueCount()
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	path, got, err := s.PickImage()
	if err != nil {
		t.Fatalf("PickImage: %v", err)
	}
	if path != "603/poster.jpg" {
		t.Errorf("path = %q, want %q", path, "603/poster.jpg")
	}
	if got.TMDBPath != "/abc123.jpg" {
		t.Errorf("TMDBPath = %q, want %q", got.TMDBPath, "/abc123.jpg")
	}
	if got.Size != "w500" {
		t.Errorf("Size = %q, want %q", got.Size, "w500")
	}
}

func TestPickImage_EmptyQueue(t *testing.T) {
	s := testStore(t)

	path, entry, err := s.PickImage()
	if err != nil {
		t.Fatalf("PickImage: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
	if entry != nil {
		t.Errorf("expected nil, got %+v", entry)
	}
}

func TestDeleteImageEntry(t *testing.T) {
	s := testStore(t)

	s.EnqueueImage("603/poster.jpg", ImageQueueEntry{TMDBPath: "/abc.jpg", Size: "w500"})
	if err := s.DeleteImageEntry("603/poster.jpg"); err != nil {
		t.Fatalf("DeleteImageEntry: %v", err)
	}

	count, _ := s.ImageQueueCount()
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestImageQueue_MultipleEntries(t *testing.T) {
	s := testStore(t)

	s.EnqueueImage("603/poster.jpg", ImageQueueEntry{TMDBPath: "/a.jpg", Size: "w500"})
	s.EnqueueImage("603/backdrop.jpg", ImageQueueEntry{TMDBPath: "/b.jpg", Size: "w1280"})
	s.EnqueueImage("people/12345.jpg", ImageQueueEntry{TMDBPath: "/c.jpg", Size: "w185"})

	count, _ := s.ImageQueueCount()
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}

	s.DeleteImageEntry("603/poster.jpg")
	count, _ = s.ImageQueueCount()
	if count != 2 {
		t.Errorf("count = %d, want 2 after delete", count)
	}
}

func TestImageQueue_OverwritesDuplicate(t *testing.T) {
	s := testStore(t)

	s.EnqueueImage("603/poster.jpg", ImageQueueEntry{TMDBPath: "/old.jpg", Size: "w500"})
	s.EnqueueImage("603/poster.jpg", ImageQueueEntry{TMDBPath: "/new.jpg", Size: "w500"})

	count, _ := s.ImageQueueCount()
	if count != 1 {
		t.Errorf("count = %d, want 1 (overwrite, not duplicate)", count)
	}

	_, entry, _ := s.PickImage()
	if entry.TMDBPath != "/new.jpg" {
		t.Errorf("TMDBPath = %q, want %q (should be latest)", entry.TMDBPath, "/new.jpg")
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Breaking Bad", "breaking bad"},
		{"  Breaking Bad  ", "breaking bad"},
		{"Gladiator (2000)", "gladiator"},
		{"The Matrix (1999)", "the matrix"},
		{"UPPER CASE", "upper case"},
		{"No Year Here", "no year here"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSetNameAndLookup(t *testing.T) {
	s := testStore(t)

	_, _, found := s.LookupName("Breaking Bad")
	if found {
		t.Fatal("expected not found for empty index")
	}

	if err := s.SetName("Breaking Bad", 1396, "series"); err != nil {
		t.Fatalf("SetName: %v", err)
	}

	id, mt, found := s.LookupName("Breaking Bad")
	if !found {
		t.Fatal("expected found after SetName")
	}
	if id != 1396 {
		t.Errorf("tmdb_id = %d, want 1396", id)
	}
	if mt != "series" {
		t.Errorf("media_type = %q, want series", mt)
	}
}

func TestLookupNameNormalized(t *testing.T) {
	s := testStore(t)

	s.SetName("the matrix (1999)", 603, "movie")

	id, mt, found := s.LookupName("The Matrix (1999)")
	if !found {
		t.Fatal("expected found with different casing/year")
	}
	if id != 603 || mt != "movie" {
		t.Errorf("got id=%d type=%s, want 603 movie", id, mt)
	}

	id, _, found = s.LookupName("THE MATRIX")
	if !found {
		t.Fatal("expected found with uppercase no year")
	}
	if id != 603 {
		t.Errorf("got id=%d, want 603", id)
	}
}

func TestSetNameOverwrite(t *testing.T) {
	s := testStore(t)

	s.SetName("breaking bad", 100, "movie")
	s.SetName("breaking bad", 1396, "series")

	id, mt, found := s.LookupName("breaking bad")
	if !found {
		t.Fatal("expected found")
	}
	if id != 1396 || mt != "series" {
		t.Errorf("got id=%d type=%s, want 1396 series", id, mt)
	}
}

func TestLookupEmptyName(t *testing.T) {
	s := testStore(t)

	_, _, found := s.LookupName("")
	if found {
		t.Fatal("expected not found for empty name")
	}

	if err := s.SetName("", 123, "movie"); err != nil {
		t.Fatal("SetName with empty should not error")
	}

	_, _, found = s.LookupName("")
	if found {
		t.Fatal("expected not found for empty name even after set")
	}
}

func TestPickOldestResolving_SkipsComplete(t *testing.T) {
	s := testStore(t)

	s.EnqueueMetadata(QueueEntry{TMDBID: 100, MediaType: "movie", Status: "complete", CreatedAt: 1000})
	s.EnqueueMetadata(QueueEntry{TMDBID: 200, MediaType: "series", Status: "resolving", CreatedAt: 2000})

	entry, _ := s.PickOldestResolving()
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.TMDBID != 200 {
		t.Errorf("TMDBID = %d, want 200 (should skip complete)", entry.TMDBID)
	}
}
