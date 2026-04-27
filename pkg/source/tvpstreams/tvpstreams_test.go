package tvpstreams

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/cache/tmdb"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

const testPlaylist = `#EXTM3U
#EXTINF:-1 tvg-name="The Matrix" tvg-logo="http://img.example.com/matrix.jpg" group-title="Movies" tvp-type="movie" tvp-year="1999" tvp-tmdb="603" tvp-resolution="1080p" tvp-codec="h264" tvp-audio="aac" tvp-collection="The Matrix Collection" tvp-collection-id="2344",The Matrix (1999)
http://streams.example.com/movies/matrix.mkv
#EXTINF:-1 tvg-name="Inception" tvg-logo="http://img.example.com/inception.jpg" group-title="Movies" tvp-type="movie" tvp-year="2010" tvp-tmdb="27205" tvp-resolution="4K" tvp-codec="hevc" tvp-audio="eac3",Inception (2010)
http://streams.example.com/movies/inception.mkv
#EXTINF:-1 tvg-name="Breaking Bad S01E01" group-title="TV Series" tvp-type="episode" tvp-tmdb="1396" tvp-season="1" tvp-episode="1" tvp-episode-name="Pilot" tvp-resolution="720p" tvp-codec="h264" tvp-audio="ac3" tvp-local="true",Breaking Bad - S01E01 - Pilot
http://streams.example.com/tv/bb/s01e01.mkv
#EXTINF:-1 tvg-name="Breaking Bad S01E02" group-title="TV Series" tvp-type="episode" tvp-tmdb="1396" tvp-season="1" tvp-episode="2" tvp-episode-name="Cat's in the Bag..." tvp-resolution="720p" tvp-codec="h264" tvp-audio="ac3",Breaking Bad - S01E02 - Cat's in the Bag...
http://streams.example.com/tv/bb/s01e02.mkv
`

func TestImplementsSource(t *testing.T) {
	var _ source.Source = (*Source)(nil)
}

func TestImplementsConditionalRefresher(t *testing.T) {
	var _ source.ConditionalRefresher = (*Source)(nil)
}

func TestImplementsVPNRoutable(t *testing.T) {
	var _ source.VPNRoutable = (*Source)(nil)
}

func TestImplementsVODProvider(t *testing.T) {
	var _ source.VODProvider = (*Source)(nil)
}

func TestImplementsClearable(t *testing.T) {
	var _ source.Clearable = (*Source)(nil)
}

func TestImplementsTLSProvider(t *testing.T) {
	var _ source.TLSProvider = (*Source)(nil)
}

func TestType(t *testing.T) {
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test TVP Streams",
		URL:         "http://example.com/playlist.m3u",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if s.Type() != "tvpstreams" {
		t.Fatalf("expected type tvpstreams, got %s", s.Type())
	}
}

func TestVODTypes(t *testing.T) {
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         "http://example.com/playlist.m3u",
		StreamStore: store.NewMemoryStreamStore(),
	})

	if !s.SupportsVOD() {
		t.Fatal("expected SupportsVOD true")
	}

	types := s.VODTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 VOD types, got %d", len(types))
	}

	expected := map[string]bool{"movie": true, "series": true}
	for _, typ := range types {
		if !expected[typ] {
			t.Fatalf("unexpected VOD type: %s", typ)
		}
	}
}

func TestInfo(t *testing.T) {
	s := New(Config{
		ID:        "tvp-1",
		Name:      "My Library",
		URL:       "http://example.com/playlist.m3u",
		IsEnabled: true,
		StreamStore: store.NewMemoryStreamStore(),
	})

	info := s.Info(context.Background())
	if info.ID != "tvp-1" {
		t.Fatalf("expected ID tvp-1, got %s", info.ID)
	}
	if info.Name != "My Library" {
		t.Fatalf("expected Name My Library, got %s", info.Name)
	}
	if info.Type != "tvpstreams" {
		t.Fatalf("expected Type tvpstreams, got %s", info.Type)
	}
	if !info.IsEnabled {
		t.Fatal("expected IsEnabled true")
	}
	if info.StreamCount != 0 {
		t.Fatalf("expected StreamCount 0, got %d", info.StreamCount)
	}
}

func TestRefresh(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		IsEnabled:   true,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, err := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	if err != nil {
		t.Fatalf("listing streams: %v", err)
	}
	if len(streams) != 4 {
		t.Fatalf("expected 4 streams, got %d", len(streams))
	}

	info := s.Info(context.Background())
	if info.StreamCount != 4 {
		t.Fatalf("expected StreamCount 4, got %d", info.StreamCount)
	}
	if info.LastRefreshed == nil {
		t.Fatal("expected LastRefreshed to be set")
	}
}

func TestRefreshMovieAttributes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	var matrix, inception *struct {
		name, vodType, tmdbID, year, videoCodec, audioCodec, collection, collectionID string
		width, height                                                                 int
	}

	for _, stream := range streams {
		if stream.Name == "The Matrix (1999)" {
			matrix = &struct {
				name, vodType, tmdbID, year, videoCodec, audioCodec, collection, collectionID string
				width, height                                                                 int
			}{
				stream.Name, stream.VODType, stream.TMDBID, stream.Year,
				stream.VideoCodec, stream.AudioCodec,
				stream.CollectionName, stream.CollectionID,
				stream.Width, stream.Height,
			}
		}
		if stream.Name == "Inception (2010)" {
			inception = &struct {
				name, vodType, tmdbID, year, videoCodec, audioCodec, collection, collectionID string
				width, height                                                                 int
			}{
				stream.Name, stream.VODType, stream.TMDBID, stream.Year,
				stream.VideoCodec, stream.AudioCodec,
				stream.CollectionName, stream.CollectionID,
				stream.Width, stream.Height,
			}
		}
	}

	if matrix == nil {
		t.Fatal("The Matrix stream not found")
	}
	if matrix.vodType != "movie" {
		t.Fatalf("expected vodType movie, got %s", matrix.vodType)
	}
	if matrix.tmdbID != "603" {
		t.Fatalf("expected tmdbID 603, got %s", matrix.tmdbID)
	}
	if matrix.year != "1999" {
		t.Fatalf("expected year 1999, got %s", matrix.year)
	}
	if matrix.videoCodec != "h264" {
		t.Fatalf("expected videoCodec h264, got %s", matrix.videoCodec)
	}
	if matrix.audioCodec != "aac" {
		t.Fatalf("expected audioCodec aac, got %s", matrix.audioCodec)
	}
	if matrix.collection != "The Matrix Collection" {
		t.Fatalf("expected collection The Matrix Collection, got %s", matrix.collection)
	}
	if matrix.collectionID != "2344" {
		t.Fatalf("expected collectionID 2344, got %s", matrix.collectionID)
	}
	if matrix.width != 1920 || matrix.height != 1080 {
		t.Fatalf("expected 1920x1080, got %dx%d", matrix.width, matrix.height)
	}

	if inception == nil {
		t.Fatal("Inception stream not found")
	}
	if inception.videoCodec != "hevc" {
		t.Fatalf("expected videoCodec hevc, got %s", inception.videoCodec)
	}
	if inception.audioCodec != "eac3" {
		t.Fatalf("expected audioCodec eac3, got %s", inception.audioCodec)
	}
	if inception.width != 3840 || inception.height != 2160 {
		t.Fatalf("expected 3840x2160, got %dx%d", inception.width, inception.height)
	}
}

func TestRefreshEpisodeAttributes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")

	var pilot *struct {
		vodType, tmdbID, episodeName string
		season, episode              int
		isLocal                      bool
		width, height                int
	}

	for _, stream := range streams {
		if stream.Episode == 1 && stream.Season == 1 && stream.TMDBID == "1396" {
			pilot = &struct {
				vodType, tmdbID, episodeName string
				season, episode              int
				isLocal                      bool
				width, height                int
			}{
				stream.VODType, stream.TMDBID, stream.EpisodeName,
				stream.Season, stream.Episode,
				stream.IsLocal,
				stream.Width, stream.Height,
			}
		}
	}

	if pilot == nil {
		t.Fatal("BB S01E01 stream not found")
	}
	if pilot.vodType != "episode" {
		t.Fatalf("expected vodType episode, got %s", pilot.vodType)
	}
	if pilot.tmdbID != "1396" {
		t.Fatalf("expected tmdbID 1396, got %s", pilot.tmdbID)
	}
	if pilot.season != 1 {
		t.Fatalf("expected season 1, got %d", pilot.season)
	}
	if pilot.episode != 1 {
		t.Fatalf("expected episode 1, got %d", pilot.episode)
	}
	if pilot.episodeName != "Pilot" {
		t.Fatalf("expected episodeName Pilot, got %s", pilot.episodeName)
	}
	if !pilot.isLocal {
		t.Fatal("expected isLocal true")
	}
	if pilot.width != 1280 || pilot.height != 720 {
		t.Fatalf("expected 1280x720, got %dx%d", pilot.width, pilot.height)
	}
}

func TestResolutionParsing(t *testing.T) {
	tests := []struct {
		input  string
		width  int
		height int
	}{
		{"1080p", 1920, 1080},
		{"4K", 3840, 2160},
		{"720p", 1280, 720},
		{"480p", 854, 480},
		{"2160p", 3840, 2160},
		{"", 0, 0},
		{"unknown", 0, 0},
	}

	for _, tt := range tests {
		w, h := parseResolution(tt.input)
		if w != tt.width || h != tt.height {
			t.Errorf("parseResolution(%q) = %dx%d, want %dx%d", tt.input, w, h, tt.width, tt.height)
		}
	}
}

func TestGroupFromVODType(t *testing.T) {
	tests := []struct {
		vodType  string
		original string
		expected string
	}{
		{"movie", "Movies", "Movies"},
		{"episode", "TV Series", "TV Series"},
		{"movie", "", "Movies"},
		{"series", "", "TV Series"},
		{"episode", "", "TV Series"},
		{"", "Custom Group", "Custom Group"},
	}

	for _, tt := range tests {
		got := resolveGroup(tt.vodType, tt.original)
		if got != tt.expected {
			t.Errorf("resolveGroup(%q, %q) = %q, want %q", tt.vodType, tt.original, got, tt.expected)
		}
	}
}

func TestConditionalRefreshETag(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		etag := `"lib-v1"`
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("second refresh failed: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}

	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	if len(streams) != 4 {
		t.Fatalf("expected 4 streams after conditional refresh, got %d", len(streams))
	}
}

func TestDeterministicStreamID(t *testing.T) {
	id1 := deterministicStreamID("tvp-1", "http://example.com/movie.mkv")
	id2 := deterministicStreamID("tvp-1", "http://example.com/movie.mkv")
	if id1 != id2 {
		t.Fatalf("expected deterministic IDs, got %s and %s", id1, id2)
	}
	if id1 == "" {
		t.Fatal("expected non-empty stream ID")
	}

	id3 := deterministicStreamID("tvp-2", "http://example.com/movie.mkv")
	if id1 == id3 {
		t.Fatal("expected different IDs for different source IDs")
	}

	h := sha256.Sum256([]byte("tvp-1:http://example.com/movie.mkv"))
	expected := fmt.Sprintf("%x", h[:16])
	if id1 != expected {
		t.Fatalf("expected ID %s, got %s", expected, id1)
	}
}

func TestUsesVPN(t *testing.T) {
	s := New(Config{
		ID:           "tvp-1",
		UseWireGuard: true,
		StreamStore:  store.NewMemoryStreamStore(),
	})
	if !s.UsesVPN() {
		t.Fatal("expected UsesVPN true")
	}

	s2 := New(Config{
		ID:          "tvp-2",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if s2.UsesVPN() {
		t.Fatal("expected UsesVPN false")
	}
}

func TestSupportsConditionalRefresh(t *testing.T) {
	s := New(Config{
		ID:          "tvp-1",
		StreamStore: store.NewMemoryStreamStore(),
	})
	if !s.SupportsConditionalRefresh() {
		t.Fatal("expected SupportsConditionalRefresh true")
	}
}

func TestStreams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	ids, err := s.Streams(context.Background())
	if err != nil {
		t.Fatalf("streams failed: %v", err)
	}
	if len(ids) != 4 {
		t.Fatalf("expected 4 stream IDs, got %d", len(ids))
	}
}

func TestDeleteStreams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	if err := s.DeleteStreams(context.Background()); err != nil {
		t.Fatalf("delete streams failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	if len(streams) != 0 {
		t.Fatalf("expected 0 streams after delete, got %d", len(streams))
	}
}

func TestClear(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	info := s.Info(context.Background())
	if info.StreamCount != 4 {
		t.Fatalf("expected 4 streams before clear, got %d", info.StreamCount)
	}

	if err := s.Clear(context.Background()); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	if len(streams) != 0 {
		t.Fatalf("expected 0 streams after clear, got %d", len(streams))
	}

	info = s.Info(context.Background())
	if info.StreamCount != 0 {
		t.Fatalf("expected StreamCount 0 after clear, got %d", info.StreamCount)
	}
}

func TestStaleStreamsRemoved(t *testing.T) {
	playlist1 := `#EXTM3U
#EXTINF:-1 tvp-type="movie" tvp-tmdb="603",The Matrix
http://streams.example.com/matrix.mkv
#EXTINF:-1 tvp-type="movie" tvp-tmdb="27205",Inception
http://streams.example.com/inception.mkv
`
	playlist2 := `#EXTM3U
#EXTINF:-1 tvp-type="movie" tvp-tmdb="603",The Matrix
http://streams.example.com/matrix.mkv
`

	current := playlist1
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, current)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())
	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}

	current = playlist2
	_ = s.Refresh(context.Background())
	streams, _ = ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream after stale removal, got %d", len(streams))
	}
}

func TestDuplicateURLsDeduped(t *testing.T) {
	playlist := `#EXTM3U
#EXTINF:-1 tvp-type="movie",Movie A
http://streams.example.com/same.mkv
#EXTINF:-1 tvp-type="movie",Movie B
http://streams.example.com/same.mkv
`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, playlist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream (deduped), got %d", len(streams))
	}
}

func TestTMDBEnrichment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	tc := tmdb.New()
	tc.SetMovie("603", &tmdb.Movie{
		ID:         603,
		Title:      "The Matrix",
		PosterPath: "/poster.jpg",
		Overview:   "A computer hacker learns about the true nature of reality.",
	})

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
		TMDBCache:   tc,
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	var matrixLogo string
	for _, stream := range streams {
		if stream.TMDBID == "603" {
			matrixLogo = stream.TvgLogo
			break
		}
	}

	if matrixLogo != "http://img.example.com/matrix.jpg" {
		t.Fatalf("expected original logo preserved when set, got %s", matrixLogo)
	}
}

func TestTMDBEnrichmentFillsEmptyLogo(t *testing.T) {
	playlist := `#EXTM3U
#EXTINF:-1 tvp-type="movie" tvp-tmdb="603",The Matrix
http://streams.example.com/matrix.mkv
`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, playlist)
	}))
	defer ts.Close()

	tc := tmdb.New()
	tc.SetMovie("603", &tmdb.Movie{
		ID:         603,
		Title:      "The Matrix",
		PosterPath: "/poster.jpg",
	})

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
		TMDBCache:   tc,
	})

	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}
	if streams[0].TvgLogo != "https://image.tmdb.org/t/p/w500/poster.jpg" {
		t.Fatalf("expected TMDB poster URL, got %s", streams[0].TvgLogo)
	}
}

func TestLocalStreamMarked(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	localCount := 0
	for _, stream := range streams {
		if stream.IsLocal {
			localCount++
		}
	}
	if localCount != 1 {
		t.Fatalf("expected 1 local stream, got %d", localCount)
	}
}

func TestRefreshError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error on 500 response")
	}

	info := s.Info(context.Background())
	if info.LastError == "" {
		t.Fatal("expected LastError to be set")
	}
}

func TestTLSInfoNotEnrolled(t *testing.T) {
	s := New(Config{
		ID:          "tvp-1",
		StreamStore: store.NewMemoryStreamStore(),
	})

	info := s.TLSInfo()
	if info.Enrolled {
		t.Fatal("expected not enrolled")
	}
	if info.Fingerprint != "" {
		t.Fatalf("expected empty fingerprint, got %s", info.Fingerprint)
	}
}

func TestTLSInfoEnrolled(t *testing.T) {
	s := New(Config{
		ID:          "tvp-1",
		TLSEnrolled: true,
		DataDir:     t.TempDir(),
		StreamStore: store.NewMemoryStreamStore(),
	})

	info := s.TLSInfo()
	if !info.Enrolled {
		t.Fatal("expected enrolled")
	}
}

func TestRefreshWithEnrollment(t *testing.T) {
	enrollCalled := false
	enrollServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/enroll" {
			enrollCalled = true
			fmt.Fprint(w, `{"cert":"-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n","key":"-----BEGIN EC PRIVATE KEY-----\ntest\n-----END EC PRIVATE KEY-----\n","ca":"-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n","fingerprint":"AB:CD"}`)
			return
		}
		fmt.Fprint(w, testPlaylist)
	}))
	defer enrollServer.Close()

	dataDir := t.TempDir()
	var enrolledSourceID string

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:              "tvp-enroll",
		Name:            "Test",
		URL:             enrollServer.URL + "/playlist.m3u",
		IsEnabled:       true,
		DataDir:         dataDir,
		EnrollmentToken: "test-token",
		StreamStore:     ss,
		HTTPClient:      enrollServer.Client(),
		OnEnrolled: func(sourceID string) error {
			enrolledSourceID = sourceID
			return nil
		},
	})

	err := s.Refresh(context.Background())
	if err != nil {
		t.Logf("refresh error (expected due to test certs): %v", err)
	}

	if !enrollCalled {
		t.Fatal("expected enrollment to be called")
	}
	if enrolledSourceID != "tvp-enroll" {
		t.Fatalf("expected OnEnrolled with tvp-enroll, got %s", enrolledSourceID)
	}

	info := s.TLSInfo()
	if !info.Enrolled {
		t.Fatal("expected enrolled after refresh")
	}
}

func TestSourceFieldsSet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPlaylist)
	}))
	defer ts.Close()

	ss := store.NewMemoryStreamStore()
	s := New(Config{
		ID:          "tvp-1",
		Name:        "Test",
		URL:         ts.URL,
		StreamStore: ss,
		HTTPClient:  ts.Client(),
	})

	_ = s.Refresh(context.Background())

	streams, _ := ss.ListBySource(context.Background(), "tvpstreams", "tvp-1")
	for _, stream := range streams {
		if stream.SourceType != "tvpstreams" {
			t.Fatalf("expected SourceType tvpstreams, got %s", stream.SourceType)
		}
		if stream.SourceID != "tvp-1" {
			t.Fatalf("expected SourceID tvp-1, got %s", stream.SourceID)
		}
		if !stream.IsActive {
			t.Fatalf("expected IsActive for %s", stream.Name)
		}
	}
}
