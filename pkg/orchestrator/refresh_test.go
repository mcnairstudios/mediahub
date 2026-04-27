package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
)

type mockSource struct {
	info       source.SourceInfo
	refreshErr error
	refreshed  bool
}

func (m *mockSource) Info(_ context.Context) source.SourceInfo    { return m.info }
func (m *mockSource) Refresh(_ context.Context) error             { m.refreshed = true; return m.refreshErr }
func (m *mockSource) Streams(_ context.Context) ([]string, error) { return nil, nil }
func (m *mockSource) DeleteStreams(_ context.Context) error        { return nil }
func (m *mockSource) Type() source.SourceType                     { return m.info.Type }

type mockSourceConfigStore struct {
	configs []sourceconfig.SourceConfig
}

func (m *mockSourceConfigStore) Get(_ context.Context, id string) (*sourceconfig.SourceConfig, error) {
	for _, c := range m.configs {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, nil
}

func (m *mockSourceConfigStore) List(_ context.Context) ([]sourceconfig.SourceConfig, error) {
	return m.configs, nil
}

func (m *mockSourceConfigStore) ListByType(_ context.Context, t string) ([]sourceconfig.SourceConfig, error) {
	var result []sourceconfig.SourceConfig
	for _, c := range m.configs {
		if c.Type == t {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockSourceConfigStore) Create(_ context.Context, sc *sourceconfig.SourceConfig) error {
	m.configs = append(m.configs, *sc)
	return nil
}

func (m *mockSourceConfigStore) Update(_ context.Context, _ *sourceconfig.SourceConfig) error {
	return nil
}

func (m *mockSourceConfigStore) Delete(_ context.Context, _ string) error { return nil }

func newTestRefreshDeps(sources map[source.SourceType]*mockSource, configs []sourceconfig.SourceConfig) RefreshDeps {
	reg := source.NewRegistry()
	for st, src := range sources {
		captured := src
		reg.Register(st, func(_ context.Context, _ string) (source.Source, error) {
			return captured, nil
		})
	}
	return RefreshDeps{
		SourceReg:         reg,
		SourceConfigStore: &mockSourceConfigStore{configs: configs},
	}
}

func TestRefreshSource(t *testing.T) {
	mock := &mockSource{info: source.SourceInfo{ID: "src-1", Type: "m3u"}}
	deps := newTestRefreshDeps(map[source.SourceType]*mockSource{
		"m3u": mock,
	}, nil)

	err := RefreshSource(context.Background(), deps, "m3u", "src-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.refreshed {
		t.Error("expected source to be refreshed")
	}
}

func TestRefreshSource_UnknownType(t *testing.T) {
	deps := newTestRefreshDeps(nil, nil)

	err := RefreshSource(context.Background(), deps, "unknown", "src-1")
	if err == nil {
		t.Fatal("expected error for unknown source type")
	}
	if !errors.Is(err, source.ErrUnknownSourceType) {
		t.Errorf("expected ErrUnknownSourceType, got: %v", err)
	}
}

func TestRefreshAll(t *testing.T) {
	m3u := &mockSource{info: source.SourceInfo{ID: "src-1", Type: "m3u"}}
	hdhr := &mockSource{info: source.SourceInfo{ID: "src-2", Type: "hdhr"}}
	configs := []sourceconfig.SourceConfig{
		{ID: "src-1", Type: "m3u", Name: "test-m3u", IsEnabled: true},
		{ID: "src-2", Type: "hdhr", Name: "test-hdhr", IsEnabled: true},
	}
	deps := newTestRefreshDeps(map[source.SourceType]*mockSource{
		"m3u":  m3u,
		"hdhr": hdhr,
	}, configs)

	errs := RefreshAll(context.Background(), deps)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !m3u.refreshed {
		t.Error("expected m3u source to be refreshed")
	}
	if !hdhr.refreshed {
		t.Error("expected hdhr source to be refreshed")
	}
}

func TestRefreshAll_PartialFailure(t *testing.T) {
	m3u := &mockSource{info: source.SourceInfo{ID: "src-1", Type: "m3u"}, refreshErr: errors.New("network error")}
	hdhr := &mockSource{info: source.SourceInfo{ID: "src-2", Type: "hdhr"}}
	configs := []sourceconfig.SourceConfig{
		{ID: "src-1", Type: "m3u", Name: "test-m3u", IsEnabled: true},
		{ID: "src-2", Type: "hdhr", Name: "test-hdhr", IsEnabled: true},
	}
	deps := newTestRefreshDeps(map[source.SourceType]*mockSource{
		"m3u":  m3u,
		"hdhr": hdhr,
	}, configs)

	errs := RefreshAll(context.Background(), deps)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !hdhr.refreshed {
		t.Error("expected hdhr to still be refreshed despite m3u failure")
	}
}

func TestRefreshAll_SkipsDisabled(t *testing.T) {
	m3u := &mockSource{info: source.SourceInfo{ID: "src-1", Type: "m3u"}}
	configs := []sourceconfig.SourceConfig{
		{ID: "src-1", Type: "m3u", Name: "test-m3u", IsEnabled: false},
	}
	deps := newTestRefreshDeps(map[source.SourceType]*mockSource{
		"m3u": m3u,
	}, configs)

	errs := RefreshAll(context.Background(), deps)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if m3u.refreshed {
		t.Error("disabled source should not be refreshed")
	}
}
