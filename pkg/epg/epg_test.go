package epg

import (
	"context"
	"testing"
	"time"
)

type mockSourceStore struct{}

func (m *mockSourceStore) Get(_ context.Context, _ string) (*Source, error)  { return nil, nil }
func (m *mockSourceStore) List(_ context.Context) ([]Source, error)         { return nil, nil }
func (m *mockSourceStore) Create(_ context.Context, _ *Source) error        { return nil }
func (m *mockSourceStore) Update(_ context.Context, _ *Source) error        { return nil }
func (m *mockSourceStore) Delete(_ context.Context, _ string) error         { return nil }

type mockProgramStore struct{}

func (m *mockProgramStore) NowPlaying(_ context.Context, _ string) (*Program, error) { return nil, nil }
func (m *mockProgramStore) Range(_ context.Context, _ string, _, _ time.Time) ([]Program, error) {
	return nil, nil
}
func (m *mockProgramStore) ListAll(_ context.Context) ([]Program, error)     { return nil, nil }
func (m *mockProgramStore) BulkInsert(_ context.Context, _ []Program) error  { return nil }
func (m *mockProgramStore) DeleteBySource(_ context.Context, _ string) error { return nil }

var _ SourceStore = (*mockSourceStore)(nil)
var _ ProgramStore = (*mockProgramStore)(nil)

func TestSourceZeroValue(t *testing.T) {
	var s Source
	if s.ID != "" {
		t.Fatal("expected empty ID")
	}
	if s.IsEnabled {
		t.Fatal("expected IsEnabled false")
	}
	if s.LastRefreshed != nil {
		t.Fatal("expected nil LastRefreshed")
	}
}

func TestProgramZeroValue(t *testing.T) {
	var p Program
	if p.ChannelID != "" {
		t.Fatal("expected empty ChannelID")
	}
	if p.IsNew {
		t.Fatal("expected IsNew false")
	}
	if p.Categories != nil {
		t.Fatal("expected nil Categories")
	}
}

func TestProgramTimeRange(t *testing.T) {
	start := time.Date(2026, 4, 25, 20, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 25, 21, 0, 0, 0, time.UTC)
	p := Program{
		ChannelID: "ch1",
		Title:     "News",
		StartTime: start,
		EndTime:   end,
	}
	if !p.EndTime.After(p.StartTime) {
		t.Fatal("expected EndTime after StartTime")
	}
}
