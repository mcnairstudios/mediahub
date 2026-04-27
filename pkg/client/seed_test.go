package client

import (
	"context"
	"testing"
)

type memClientStore struct {
	clients map[string]*Client
}

func newMemClientStore() *memClientStore {
	return &memClientStore{clients: make(map[string]*Client)}
}

func (s *memClientStore) Get(_ context.Context, id string) (*Client, error) {
	c, ok := s.clients[id]
	if !ok {
		return nil, nil
	}
	return c, nil
}

func (s *memClientStore) List(_ context.Context) ([]Client, error) {
	var result []Client
	for _, c := range s.clients {
		result = append(result, *c)
	}
	return result, nil
}

func (s *memClientStore) Create(_ context.Context, c *Client) error {
	s.clients[c.ID] = c
	return nil
}

func (s *memClientStore) Update(_ context.Context, c *Client) error {
	s.clients[c.ID] = c
	return nil
}

func (s *memClientStore) Delete(_ context.Context, id string) error {
	delete(s.clients, id)
	return nil
}

func TestSeedDefaults(t *testing.T) {
	store := newMemClientStore()
	ctx := context.Background()

	err := SeedDefaults(ctx, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clients, _ := store.List(ctx)
	if len(clients) == 0 {
		t.Fatal("expected default clients to be seeded")
	}

	names := make(map[string]bool)
	for _, c := range clients {
		names[c.Name] = true
		if c.ID == "" {
			t.Errorf("client %s has empty ID", c.Name)
		}
	}

	for _, expected := range []string{"Browser", "Jellyfin", "Plex", "VLC"} {
		if !names[expected] {
			t.Errorf("expected %s client to be seeded", expected)
		}
	}
}

func TestSeedDefaults_NoOverwrite(t *testing.T) {
	store := newMemClientStore()
	ctx := context.Background()

	existing := &Client{ID: "existing", Name: "Existing", IsEnabled: true}
	store.Create(ctx, existing)

	err := SeedDefaults(ctx, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clients, _ := store.List(ctx)
	if len(clients) != 1 {
		t.Fatalf("expected seed to skip when clients exist, got %d clients", len(clients))
	}
	if clients[0].Name != "Existing" {
		t.Errorf("expected existing client, got %s", clients[0].Name)
	}
}

func TestSeedDefaults_BrowserHasMSEDelivery(t *testing.T) {
	store := newMemClientStore()
	ctx := context.Background()

	SeedDefaults(ctx, store)

	clients, _ := store.List(ctx)
	for _, c := range clients {
		if c.Name == "Browser" {
			if c.Profile.Delivery != "mse" {
				t.Errorf("Browser delivery = %s, want mse", c.Profile.Delivery)
			}
			if c.Profile.AudioCodec != "aac" {
				t.Errorf("Browser audio = %s, want aac", c.Profile.AudioCodec)
			}
			if !c.IsSystem {
				t.Error("Browser should be a system client")
			}
			return
		}
	}
	t.Error("Browser client not found")
}

func TestSeedDefaults_JellyfinHasHLSDelivery(t *testing.T) {
	store := newMemClientStore()
	ctx := context.Background()

	SeedDefaults(ctx, store)

	clients, _ := store.List(ctx)
	for _, c := range clients {
		if c.Name == "Jellyfin" {
			if c.Profile.Delivery != "hls" {
				t.Errorf("Jellyfin delivery = %s, want hls", c.Profile.Delivery)
			}
			if c.ListenPort != 8096 {
				t.Errorf("Jellyfin port = %d, want 8096", c.ListenPort)
			}
			return
		}
	}
	t.Error("Jellyfin client not found")
}
