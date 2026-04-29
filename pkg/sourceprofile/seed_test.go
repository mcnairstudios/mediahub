package sourceprofile

import (
	"context"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/defaults"
)

type memStore struct {
	profiles []Profile
}

func (m *memStore) Get(_ context.Context, id string) (*Profile, error) {
	for i := range m.profiles {
		if m.profiles[i].ID == id {
			return &m.profiles[i], nil
		}
	}
	return nil, nil
}

func (m *memStore) List(_ context.Context) ([]Profile, error) {
	return m.profiles, nil
}

func (m *memStore) Create(_ context.Context, p *Profile) error {
	m.profiles = append(m.profiles, *p)
	return nil
}

func (m *memStore) Update(_ context.Context, p *Profile) error {
	for i := range m.profiles {
		if m.profiles[i].ID == p.ID {
			m.profiles[i] = *p
			return nil
		}
	}
	return nil
}

func (m *memStore) Delete(_ context.Context, id string) error {
	for i := range m.profiles {
		if m.profiles[i].ID == id {
			m.profiles = append(m.profiles[:i], m.profiles[i+1:]...)
			return nil
		}
	}
	return nil
}

func loadTestProfiles(t *testing.T) []defaults.SourceProfileDef {
	t.Helper()
	defs, err := defaults.LoadSourceProfiles("")
	if err != nil {
		t.Fatalf("loading embedded source profiles: %v", err)
	}
	return defs
}

func TestSeedDefaults_EmptyStore(t *testing.T) {
	store := &memStore{}
	ctx := context.Background()

	if err := SeedDefaults(ctx, store, loadTestProfiles(t)); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}

	if len(store.profiles) == 0 {
		t.Fatal("expected seeded profiles, got 0")
	}

	names := make(map[string]bool)
	for _, p := range store.profiles {
		names[p.Name] = true
		if p.ID == "" {
			t.Errorf("profile %q has empty ID", p.Name)
		}
	}

	expected := []string{"Default", "SAT>IP DVB-T", "DVB Satellite", "HDHomeRun", "Remote IPTV", "Local Network"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing expected profile %q", name)
		}
	}
}

func TestSeedDefaults_NonEmptyStore(t *testing.T) {
	store := &memStore{
		profiles: []Profile{
			{ID: "existing", Name: "Custom"},
		},
	}
	ctx := context.Background()

	if err := SeedDefaults(ctx, store, loadTestProfiles(t)); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}

	if len(store.profiles) != 1 {
		t.Errorf("expected 1 profile (no seeding), got %d", len(store.profiles))
	}
	if store.profiles[0].Name != "Custom" {
		t.Errorf("expected Custom, got %s", store.profiles[0].Name)
	}
}

func TestSeedDefaults_UniqueIDs(t *testing.T) {
	store := &memStore{}
	ctx := context.Background()

	SeedDefaults(ctx, store, loadTestProfiles(t))

	ids := make(map[string]bool)
	for _, p := range store.profiles {
		if ids[p.ID] {
			t.Errorf("duplicate ID %s", p.ID)
		}
		ids[p.ID] = true
	}
}
