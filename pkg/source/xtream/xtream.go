package xtream

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

type Config struct {
	ID           string
	Name         string
	Server       string
	Username     string
	Password     string
	IsEnabled    bool
	UseWireGuard bool
	MaxStreams   int
	StreamStore  store.StreamStore
	HTTPClient   *http.Client
	WGClient     *http.Client
}

type Source struct {
	cfg           Config
	streamCount   int
	lastRefreshed *time.Time
	lastError     string
	mu            sync.RWMutex
}

func New(cfg Config) *Source {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &Source{cfg: cfg}
}

func (s *Source) Type() source.SourceType {
	return "xtream"
}

func (s *Source) Info(_ context.Context) source.SourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return source.SourceInfo{
		ID:                  s.cfg.ID,
		Type:                "xtream",
		Name:                s.cfg.Name,
		IsEnabled:           s.cfg.IsEnabled,
		StreamCount:         s.streamCount,
		LastRefreshed:       s.lastRefreshed,
		LastError:           s.lastError,
		MaxConcurrentStreams: s.cfg.MaxStreams,
	}
}

func (s *Source) Refresh(ctx context.Context) error {
	client := s.cfg.HTTPClient
	if s.cfg.UseWireGuard && s.cfg.WGClient != nil {
		client = s.cfg.WGClient
	}

	auth, err := authenticate(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	if err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("authenticating: %w", err)
	}

	if auth.UserInfo.Status != "Active" {
		err := fmt.Errorf("account not active: %s", auth.UserInfo.Status)
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return err
	}

	categories, err := fetchCategories(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	if err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("fetching categories: %w", err)
	}

	categoryMap := make(map[string]string, len(categories))
	for _, cat := range categories {
		categoryMap[cat.ID] = cat.Name
	}

	liveStreams, err := fetchLiveStreams(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	if err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("fetching live streams: %w", err)
	}

	seen := make(map[string]struct{}, len(liveStreams))
	streams := make([]media.Stream, 0, len(liveStreams))
	keepIDs := make([]string, 0, len(liveStreams))

	for _, ls := range liveStreams {
		id := deterministicStreamID(s.cfg.ID, ls.StreamID)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		keepIDs = append(keepIDs, id)

		group := categoryMap[ls.CategoryID]

		streams = append(streams, media.Stream{
			ID:         id,
			SourceType: "xtream",
			SourceID:   s.cfg.ID,
			Name:       ls.Name,
			URL:        liveStreamURL(s.cfg.Server, s.cfg.Username, s.cfg.Password, ls.StreamID),
			Group:      group,
			TvgID:      ls.EPGChannelID,
			TvgLogo:    ls.Icon(),
			IsActive:   true,
		})
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("upserting streams: %w", err)
	}

	if _, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, "xtream", s.cfg.ID, keepIDs); err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("deleting stale streams: %w", err)
	}

	s.mu.Lock()
	s.streamCount = len(streams)
	now := time.Now()
	s.lastRefreshed = &now
	s.lastError = ""
	s.mu.Unlock()

	return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, "xtream", s.cfg.ID)
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(streams))
	for i, stream := range streams {
		ids[i] = stream.ID
	}
	return ids, nil
}

func (s *Source) DeleteStreams(ctx context.Context) error {
	return s.cfg.StreamStore.DeleteBySource(ctx, "xtream", s.cfg.ID)
}

func (s *Source) UsesVPN() bool {
	return s.cfg.UseWireGuard
}

func (s *Source) SupportsVOD() bool {
	return true
}

func (s *Source) VODTypes() []string {
	return []string{"movie", "series"}
}

func (s *Source) Clear(ctx context.Context) error {
	if err := s.cfg.StreamStore.DeleteBySource(ctx, "xtream", s.cfg.ID); err != nil {
		return err
	}

	s.mu.Lock()
	s.streamCount = 0
	s.lastError = ""
	s.mu.Unlock()

	return nil
}

type UserInfo struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	Status         string `json:"status"`
	MaxConnections string `json:"max_connections"`
}

type ServerInfo struct {
	URL            string `json:"url"`
	Port           string `json:"port"`
	ServerProtocol string `json:"server_protocol"`
}

type AuthResponse struct {
	UserInfo   UserInfo   `json:"user_info"`
	ServerInfo ServerInfo `json:"server_info"`
}

type Category struct {
	ID   string `json:"category_id"`
	Name string `json:"category_name"`
}

type LiveStream struct {
	Num          int    `json:"num"`
	Name         string `json:"name"`
	StreamType   string `json:"stream_type"`
	StreamID     int    `json:"stream_id"`
	StreamIcon   any    `json:"stream_icon"`
	EPGChannelID string `json:"epg_channel_id"`
	CategoryID   string `json:"category_id"`
}

func (ls LiveStream) Icon() string {
	if str, ok := ls.StreamIcon.(string); ok {
		return str
	}
	return ""
}

type VODStream struct {
	Num          int    `json:"num"`
	Name         string `json:"name"`
	StreamType   string `json:"stream_type"`
	StreamID     int    `json:"stream_id"`
	StreamIcon   any    `json:"stream_icon"`
	CategoryID   string `json:"category_id"`
	ContainerExt string `json:"container_extension"`
}

type Series struct {
	Num        int    `json:"num"`
	Name       string `json:"name"`
	SeriesID   int    `json:"series_id"`
	Cover      string `json:"cover"`
	CategoryID string `json:"category_id"`
}

func authenticate(ctx context.Context, client *http.Client, server, username, password string) (*AuthResponse, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s", server, username, password)
	var resp AuthResponse
	if err := apiGet(ctx, client, url, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func fetchCategories(ctx context.Context, client *http.Client, server, username, password string) ([]Category, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_live_categories", server, username, password)
	var categories []Category
	if err := apiGet(ctx, client, url, &categories); err != nil {
		return nil, err
	}
	return categories, nil
}

func fetchLiveStreams(ctx context.Context, client *http.Client, server, username, password string) ([]LiveStream, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_live_streams", server, username, password)
	var streams []LiveStream
	if err := apiGet(ctx, client, url, &streams); err != nil {
		return nil, err
	}
	return streams, nil
}

func apiGet(ctx context.Context, client *http.Client, url string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

func liveStreamURL(server, username, password string, streamID int) string {
	return fmt.Sprintf("%s/%s/%s/%d", server, username, password, streamID)
}

func vodStreamURL(server, username, password string, streamID int, ext string) string {
	if ext == "" {
		ext = "mp4"
	}
	return fmt.Sprintf("%s/movie/%s/%s/%d.%s", server, username, password, streamID, ext)
}

func seriesStreamURL(server, username, password string, streamID int) string {
	return fmt.Sprintf("%s/series/%s/%s/%d", server, username, password, streamID)
}

func deterministicStreamID(sourceID string, streamID int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:xtream:%d", sourceID, streamID)))
	return fmt.Sprintf("%x", h[:16])
}
