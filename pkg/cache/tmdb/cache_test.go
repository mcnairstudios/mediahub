package tmdb

import (
	"context"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/cache"
)

func TestCacheImplementsInterface(t *testing.T) {
	var _ cache.Cache = New()
}

func TestType(t *testing.T) {
	c := New()
	if c.Type() != cache.CacheTMDB {
		t.Fatalf("expected %q, got %q", cache.CacheTMDB, c.Type())
	}
}

func TestSetMovieGetMovie(t *testing.T) {
	c := New()
	m := &Movie{ID: 550, Title: "Fight Club", Rating: 8.4}
	c.SetMovie("550", m)

	got, ok := c.GetMovie("550")
	if !ok {
		t.Fatal("expected movie to exist")
	}
	if got.ID != 550 || got.Title != "Fight Club" || got.Rating != 8.4 {
		t.Fatalf("movie mismatch: %+v", got)
	}
}

func TestSetSeriesGetSeries(t *testing.T) {
	c := New()
	s := &Series{
		ID:   1399,
		Name: "Breaking Bad",
		Seasons: []Season{
			{SeasonNumber: 1, EpisodeCount: 7},
		},
	}
	c.SetSeries("1399", s)

	got, ok := c.GetSeries("1399")
	if !ok {
		t.Fatal("expected series to exist")
	}
	if got.ID != 1399 || got.Name != "Breaking Bad" || len(got.Seasons) != 1 {
		t.Fatalf("series mismatch: %+v", got)
	}
}

func TestGetUnknownReturnsNilFalse(t *testing.T) {
	c := New()
	ctx := context.Background()

	val, ok := c.Get(ctx, "nonexistent")
	if ok || val != nil {
		t.Fatalf("expected nil/false, got %v/%v", val, ok)
	}

	m, ok := c.GetMovie("nonexistent")
	if ok || m != nil {
		t.Fatalf("expected nil/false for movie, got %v/%v", m, ok)
	}

	s, ok := c.GetSeries("nonexistent")
	if ok || s != nil {
		t.Fatalf("expected nil/false for series, got %v/%v", s, ok)
	}
}

func TestDelete(t *testing.T) {
	c := New()
	ctx := context.Background()

	c.SetMovie("1", &Movie{ID: 1, Title: "A"})
	c.SetSeries("1", &Series{ID: 1, Name: "B"})

	if err := c.Delete(ctx, "1"); err != nil {
		t.Fatal(err)
	}

	if _, ok := c.GetMovie("1"); ok {
		t.Fatal("movie should be deleted")
	}
	if _, ok := c.GetSeries("1"); ok {
		t.Fatal("series should be deleted")
	}
}

func TestClear(t *testing.T) {
	c := New()
	ctx := context.Background()

	c.SetMovie("1", &Movie{ID: 1})
	c.SetMovie("2", &Movie{ID: 2})
	c.SetSeries("3", &Series{ID: 3})

	if err := c.Clear(ctx); err != nil {
		t.Fatal(err)
	}

	if _, ok := c.GetMovie("1"); ok {
		t.Fatal("expected empty after clear")
	}
	if _, ok := c.GetMovie("2"); ok {
		t.Fatal("expected empty after clear")
	}
	if _, ok := c.GetSeries("3"); ok {
		t.Fatal("expected empty after clear")
	}
}

func TestGenericSetGet(t *testing.T) {
	c := New()
	ctx := context.Background()

	movie := &Movie{ID: 42, Title: "Generic Movie"}
	if err := c.Set(ctx, "m42", movie); err != nil {
		t.Fatal(err)
	}
	val, ok := c.Get(ctx, "m42")
	if !ok {
		t.Fatal("expected value from generic Get")
	}
	got, ok := val.(*Movie)
	if !ok {
		t.Fatal("expected *Movie type")
	}
	if got.Title != "Generic Movie" {
		t.Fatalf("expected Generic Movie, got %s", got.Title)
	}

	series := &Series{ID: 99, Name: "Generic Series"}
	if err := c.Set(ctx, "s99", series); err != nil {
		t.Fatal(err)
	}
	val, ok = c.Get(ctx, "s99")
	if !ok {
		t.Fatal("expected value from generic Get")
	}
	gotS, ok := val.(*Series)
	if !ok {
		t.Fatal("expected *Series type")
	}
	if gotS.Name != "Generic Series" {
		t.Fatalf("expected Generic Series, got %s", gotS.Name)
	}
}
