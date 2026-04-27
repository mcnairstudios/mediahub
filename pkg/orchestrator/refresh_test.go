package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/source"
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

func newTestRefreshDeps(sources map[source.SourceType]*mockSource) RefreshDeps {
	reg := source.NewRegistry()
	for st, src := range sources {
		captured := src
		reg.Register(st, func(_ context.Context, _ string) (source.Source, error) {
			return captured, nil
		})
	}
	return RefreshDeps{SourceReg: reg}
}

func TestRefreshSource(t *testing.T) {
	mock := &mockSource{info: source.SourceInfo{ID: "src-1", Type: "m3u"}}
	deps := newTestRefreshDeps(map[source.SourceType]*mockSource{
		"m3u": mock,
	})

	err := RefreshSource(context.Background(), deps, "m3u", "src-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.refreshed {
		t.Error("expected source to be refreshed")
	}
}

func TestRefreshSource_UnknownType(t *testing.T) {
	deps := newTestRefreshDeps(nil)

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
	deps := newTestRefreshDeps(map[source.SourceType]*mockSource{
		"m3u":  m3u,
		"hdhr": hdhr,
	})

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
	deps := newTestRefreshDeps(map[source.SourceType]*mockSource{
		"m3u":  m3u,
		"hdhr": hdhr,
	})

	errs := RefreshAll(context.Background(), deps)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !hdhr.refreshed {
		t.Error("expected hdhr to still be refreshed despite m3u failure")
	}
}
