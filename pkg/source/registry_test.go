package source

import (
	"context"
	"errors"
	"sort"
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	reg.Register("m3u", func(_ context.Context, id string) (Source, error) {
		return &mockSource{
			info: SourceInfo{ID: id, Type: "m3u", Name: "Test M3U"},
		}, nil
	})

	ctx := context.Background()
	src, err := reg.Create(ctx, "m3u", "src-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Type() != "m3u" {
		t.Fatalf("expected type m3u, got %s", src.Type())
	}
	info := src.Info(ctx)
	if info.ID != "src-1" {
		t.Fatalf("expected ID src-1, got %s", info.ID)
	}
}

func TestRegistryUnknownType(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Create(context.Background(), "unknown", "src-1")
	if err == nil {
		t.Fatal("expected error for unknown source type")
	}
	if !errors.Is(err, ErrUnknownSourceType) {
		t.Fatalf("expected ErrUnknownSourceType, got %v", err)
	}
}

func TestRegistryTypes(t *testing.T) {
	reg := NewRegistry()
	reg.Register("m3u", func(_ context.Context, id string) (Source, error) {
		return &mockSource{info: SourceInfo{ID: id, Type: "m3u"}}, nil
	})
	reg.Register("hdhr", func(_ context.Context, id string) (Source, error) {
		return &mockSource{info: SourceInfo{ID: id, Type: "hdhr"}}, nil
	})
	reg.Register("satip", func(_ context.Context, id string) (Source, error) {
		return &mockSource{info: SourceInfo{ID: id, Type: "satip"}}, nil
	})

	types := reg.Types()
	if len(types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(types))
	}

	strs := make([]string, len(types))
	for i, st := range types {
		strs[i] = string(st)
	}
	sort.Strings(strs)

	expected := []string{"hdhr", "m3u", "satip"}
	for i, e := range expected {
		if strs[i] != e {
			t.Fatalf("expected type %s at index %d, got %s", e, i, strs[i])
		}
	}
}

func TestRegistryFactoryError(t *testing.T) {
	reg := NewRegistry()
	factoryErr := errors.New("failed to create source")
	reg.Register("broken", func(_ context.Context, _ string) (Source, error) {
		return nil, factoryErr
	})

	_, err := reg.Create(context.Background(), "broken", "src-1")
	if err == nil {
		t.Fatal("expected error from factory")
	}
	if !errors.Is(err, factoryErr) {
		t.Fatalf("expected factory error, got %v", err)
	}
}

func TestRegistryOverwriteFactory(t *testing.T) {
	reg := NewRegistry()
	reg.Register("m3u", func(_ context.Context, id string) (Source, error) {
		return &mockSource{info: SourceInfo{ID: id, Type: "m3u", Name: "Original"}}, nil
	})
	reg.Register("m3u", func(_ context.Context, id string) (Source, error) {
		return &mockSource{info: SourceInfo{ID: id, Type: "m3u", Name: "Replaced"}}, nil
	})

	ctx := context.Background()
	src, err := reg.Create(ctx, "m3u", "src-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := src.Info(ctx)
	if info.Name != "Replaced" {
		t.Fatalf("expected factory to be replaced, got name %s", info.Name)
	}
}

func TestRegistryTypesEmpty(t *testing.T) {
	reg := NewRegistry()
	types := reg.Types()
	if len(types) != 0 {
		t.Fatalf("expected 0 types, got %d", len(types))
	}
}
