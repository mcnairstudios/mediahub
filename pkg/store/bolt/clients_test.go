package bolt

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/client"
)

func TestClientStore_CRUD(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.ClientStore()
	ctx := context.Background()

	c := &client.Client{
		ID:        "test-1",
		Name:      "Browser",
		Priority:  100,
		IsEnabled: true,
		IsSystem:  true,
		MatchRules: []client.MatchRule{
			{HeaderName: "User-Agent", MatchType: "contains", MatchValue: "Mozilla/"},
		},
		Profile: client.Profile{
			Delivery:   "mse",
			VideoCodec: "copy",
			AudioCodec: "aac",
			Container:  "mp4",
		},
	}

	if err := store.Create(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.Get(ctx, "test-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected client, got nil")
	}
	if got.Name != "Browser" {
		t.Errorf("name = %s, want Browser", got.Name)
	}
	if got.Profile.Delivery != "mse" {
		t.Errorf("delivery = %s, want mse", got.Profile.Delivery)
	}
	if got.Profile.AudioCodec != "aac" {
		t.Errorf("audio_codec = %s, want aac", got.Profile.AudioCodec)
	}
	if len(got.MatchRules) != 1 {
		t.Fatalf("match_rules len = %d, want 1", len(got.MatchRules))
	}

	got.Profile.VideoCodec = "h264"
	if err := store.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	updated, _ := store.Get(ctx, "test-1")
	if updated.Profile.VideoCodec != "h264" {
		t.Errorf("updated video_codec = %s, want h264", updated.Profile.VideoCodec)
	}

	clients, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("list len = %d, want 1", len(clients))
	}

	if err := store.Delete(ctx, "test-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	deleted, _ := store.Get(ctx, "test-1")
	if deleted != nil {
		t.Error("expected nil after delete")
	}
}

func TestClientStore_GetNonExistent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := db.ClientStore()
	got, err := store.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent client")
	}
}
