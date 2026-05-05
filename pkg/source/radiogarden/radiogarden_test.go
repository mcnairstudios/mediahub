package radiogarden

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

type mockStreamStore struct {
	store.StreamStore
	upserted []media.Stream
	deleted  []string
	streams  []media.Stream
}

func (m *mockStreamStore) BulkUpsert(_ context.Context, streams []media.Stream) error {
	m.upserted = append(m.upserted, streams...)
	m.streams = streams
	return nil
}

func (m *mockStreamStore) DeleteStaleBySource(_ context.Context, _, _ string, _ []string) ([]string, error) {
	return m.deleted, nil
}

func (m *mockStreamStore) ListBySource(_ context.Context, _, _ string) ([]media.Stream, error) {
	return m.streams, nil
}

func (m *mockStreamStore) DeleteBySource(_ context.Context, _, _ string) error {
	m.streams = nil
	return nil
}

func sampleChannelsResponse() placeChannelsResponse {
	var resp placeChannelsResponse
	resp.Data.Content = []struct {
		Items []struct {
			Page channelPage `json:"page"`
		} `json:"items"`
	}{
		{
			Items: []struct {
				Page channelPage `json:"page"`
			}{
				{
					Page: channelPage{
						URL:   "/listen/24-7-blues-radio/hYpXtjOZ",
						Title: "24/7 Blues Radio",
						Place: struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						}{ID: "0eZoYyEW", Title: "London"},
						Country: struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						}{ID: "jm6Gyxm7", Title: "United Kingdom"},
						Website: "https://www.247bluesradio.com",
						Stream:  "yesstreaming.net",
					},
				},
				{
					Page: channelPage{
						URL:   "/listen/bbc-radio-1/abc12345",
						Title: "BBC Radio 1",
						Place: struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						}{ID: "0eZoYyEW", Title: "London"},
						Country: struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						}{ID: "jm6Gyxm7", Title: "United Kingdom"},
					},
				},
				{
					Page: channelPage{
						URL:   "/listen/capital-fm/def67890",
						Title: "Capital FM",
						Place: struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						}{ID: "0eZoYyEW", Title: "London"},
						Country: struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						}{ID: "jm6Gyxm7", Title: "United Kingdom"},
					},
				},
			},
		},
	}
	return resp
}

func TestRefresh(t *testing.T) {
	channels := sampleChannelsResponse()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("expected User-Agent header on request")
		}
		switch {
		case r.URL.Path == "/page/0eZoYyEW/channels":
			json.NewEncoder(w).Encode(channels)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-radio",
		Name:        "London Radio",
		IsEnabled:   true,
		Places:      []Place{{ID: "0eZoYyEW", Name: "London"}},
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if len(ss.upserted) != 3 {
		t.Fatalf("expected 3 upserted streams, got %d", len(ss.upserted))
	}

	// Find the Blues Radio stream.
	var blues *media.Stream
	for i := range ss.upserted {
		if ss.upserted[i].Name == "24/7 Blues Radio" {
			blues = &ss.upserted[i]
			break
		}
	}
	if blues == nil {
		t.Fatal("could not find 24/7 Blues Radio stream")
	}
	if blues.SourceType != "radiogarden" {
		t.Errorf("expected source_type=radiogarden, got %s", blues.SourceType)
	}
	expectedURL := srv.URL + "/listen/hYpXtjOZ/channel.mp3"
	if blues.URL != expectedURL {
		t.Errorf("expected URL=%s, got %s", expectedURL, blues.URL)
	}
	if blues.Group != "London" {
		t.Errorf("expected group=London, got %s", blues.Group)
	}
	if !blues.IsActive {
		t.Error("expected IsActive=true")
	}
	if blues.VODType != "" {
		t.Errorf("expected empty VODType for live radio, got %s", blues.VODType)
	}

	info := s.Info(context.Background())
	if info.StreamCount != 3 {
		t.Errorf("expected StreamCount=3, got %d", info.StreamCount)
	}
}

func TestExtractChannelID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/listen/24-7-blues-radio/hYpXtjOZ", "hYpXtjOZ"},
		{"/listen/bbc-radio-1/abc12345", "abc12345"},
		{"/listen/station/xyz", "xyz"},
		{"", ""},
		{"/listen/only-one", "only-one"},
	}
	for _, tc := range tests {
		got := extractChannelID(tc.input)
		if got != tc.expected {
			t.Errorf("extractChannelID(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestRefreshNoPlaces(t *testing.T) {
	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-radio",
		Name:        "No Place",
		IsEnabled:   true,
		Places:      nil,
		StreamStore: ss,
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error when no places configured")
	}
}

func TestRefreshDuplicateChannels(t *testing.T) {
	// Create response with duplicate channel IDs.
	var resp placeChannelsResponse
	resp.Data.Content = []struct {
		Items []struct {
			Page channelPage `json:"page"`
		} `json:"items"`
	}{
		{
			Items: []struct {
				Page channelPage `json:"page"`
			}{
				{Page: channelPage{URL: "/listen/station-a/sameID", Title: "Station A"}},
				{Page: channelPage{URL: "/listen/station-b/sameID", Title: "Station B"}},
				{Page: channelPage{URL: "/listen/station-c/diffID", Title: "Station C"}},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-dup",
		Name:        "Dup Test",
		IsEnabled:   true,
		Places:      []Place{{ID: "place1", Name: "Test City"}},
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// sameID should only appear once (deduplication).
	if len(ss.upserted) != 2 {
		t.Fatalf("expected 2 upserted streams (deduplicated), got %d", len(ss.upserted))
	}
}

func TestRefreshAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "test-err",
		Name:        "Error Test",
		IsEnabled:   true,
		Places:      []Place{{ID: "badplace", Name: "Nowhere"}},
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error from API failure")
	}
}

func TestStreamsAndDelete(t *testing.T) {
	ss := &mockStreamStore{
		streams: []media.Stream{
			{ID: "a", SourceType: string(source.TypeRadioGarden), SourceID: "src1"},
			{ID: "b", SourceType: string(source.TypeRadioGarden), SourceID: "src1"},
		},
	}
	s := New(Config{ID: "src1", Name: "Test", IsEnabled: true, Places: []Place{{ID: "p1", Name: "Test"}}, StreamStore: ss})

	ids, err := s.Streams(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}

	if err := s.DeleteStreams(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ss.streams != nil {
		t.Error("expected streams to be nil after delete")
	}
}

func TestType(t *testing.T) {
	s := New(Config{ID: "x", Name: "X", IsEnabled: true, Places: []Place{{ID: "p1", Name: "Test"}}})
	if s.Type() != "radiogarden" {
		t.Errorf("expected type=radiogarden, got %s", s.Type())
	}
}

func TestOnRefreshDoneCalled(t *testing.T) {
	var resp placeChannelsResponse
	resp.Data.Content = []struct {
		Items []struct {
			Page channelPage `json:"page"`
		} `json:"items"`
	}{
		{
			Items: []struct {
				Page channelPage `json:"page"`
			}{
				{Page: channelPage{URL: "/listen/station/ch1", Title: "Station 1"}},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	var calledID string
	var calledCount int
	ss := &mockStreamStore{}
	s := New(Config{
		ID:          "cb-test",
		Name:        "Callback Test",
		IsEnabled:   true,
		Places:      []Place{{ID: "p1", Name: "Test"}},
		StreamStore: ss,
		HTTPClient:  srv.Client(),
		OnRefreshDone: func(sourceID, etag string, streamCount int) {
			calledID = sourceID
			calledCount = streamCount
		},
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if calledID != "cb-test" {
		t.Errorf("expected OnRefreshDone called with sourceID=cb-test, got %s", calledID)
	}
	if calledCount != 1 {
		t.Errorf("expected OnRefreshDone called with count=1, got %d", calledCount)
	}
}

func TestRefreshMultiplePlaces(t *testing.T) {
	londonChannels := sampleChannelsResponse()

	var parisChannels placeChannelsResponse
	parisChannels.Data.Content = []struct {
		Items []struct {
			Page channelPage `json:"page"`
		} `json:"items"`
	}{
		{
			Items: []struct {
				Page channelPage `json:"page"`
			}{
				{
					Page: channelPage{
						URL:   "/listen/france-inter/parisID1",
						Title: "France Inter",
						Place: struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						}{ID: "xParis1", Title: "Paris"},
						Country: struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						}{ID: "france1", Title: "France"},
					},
				},
				{
					Page: channelPage{
						URL:   "/listen/nrj-paris/parisID2",
						Title: "NRJ Paris",
						Place: struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						}{ID: "xParis1", Title: "Paris"},
						Country: struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						}{ID: "france1", Title: "France"},
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page/0eZoYyEW/channels":
			json.NewEncoder(w).Encode(londonChannels)
		case "/page/xParis1/channels":
			json.NewEncoder(w).Encode(parisChannels)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:        "test-multi",
		Name:      "Multi Place Radio",
		IsEnabled: true,
		Places: []Place{
			{ID: "0eZoYyEW", Name: "London"},
			{ID: "xParis1", Name: "Paris"},
		},
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// 3 London + 2 Paris = 5 streams
	if len(ss.upserted) != 5 {
		t.Fatalf("expected 5 upserted streams, got %d", len(ss.upserted))
	}

	// Check groups are correct
	groupCounts := map[string]int{}
	for _, st := range ss.upserted {
		groupCounts[st.Group]++
	}
	if groupCounts["London"] != 3 {
		t.Errorf("expected 3 London streams, got %d", groupCounts["London"])
	}
	if groupCounts["Paris"] != 2 {
		t.Errorf("expected 2 Paris streams, got %d", groupCounts["Paris"])
	}
}

func TestRefreshPartialFailure(t *testing.T) {
	londonChannels := sampleChannelsResponse()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page/0eZoYyEW/channels":
			json.NewEncoder(w).Encode(londonChannels)
		case "/page/badplace/channels":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	apiBaseOverride = srv.URL
	defer func() { apiBaseOverride = "" }()

	ss := &mockStreamStore{}
	s := New(Config{
		ID:        "test-partial",
		Name:      "Partial Failure",
		IsEnabled: true,
		Places: []Place{
			{ID: "0eZoYyEW", Name: "London"},
			{ID: "badplace", Name: "Nowhere"},
		},
		StreamStore: ss,
		HTTPClient:  srv.Client(),
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Fatalf("expected partial success, got error: %v", err)
	}

	// Only London should succeed (3 streams)
	if len(ss.upserted) != 3 {
		t.Fatalf("expected 3 upserted streams from London, got %d", len(ss.upserted))
	}
}

func TestParsePlaces(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:    "valid two places",
			input:   `[{"id":"abc","name":"London"},{"id":"def","name":"Paris"}]`,
			want:    2,
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			want:    0,
			wantErr: false,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			want:    0,
			wantErr: false,
		},
		{
			name:    "empty array",
			input:   "[]",
			want:    0,
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   "not json",
			want:    0,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			places, err := ParsePlaces(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(places) != tc.want {
				t.Errorf("expected %d places, got %d", tc.want, len(places))
			}
		})
	}

	// Verify field values
	places, _ := ParsePlaces(`[{"id":"abc","name":"London"}]`)
	if places[0].ID != "abc" || places[0].Name != "London" {
		t.Errorf("unexpected place values: %+v", places[0])
	}
}
