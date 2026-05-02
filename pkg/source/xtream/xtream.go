package xtream

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

type Config struct {
	ID            string
	Name          string
	Server        string
	Username      string
	Password      string
	IsEnabled     bool
	UseWireGuard  bool
	MaxStreams    int
	StreamStore   store.StreamStore
	HTTPClient    *http.Client
	WGClient      *http.Client
	OnRefreshDone func(sourceID, etag string, streamCount int)
}

type Source struct {
	source.BaseSource
	cfg Config
}

func New(cfg Config) *Source {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &Source{
		BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, "xtream", cfg.IsEnabled, cfg.MaxStreams),
		cfg:        cfg,
	}
}

func (s *Source) Refresh(ctx context.Context) error {
	client := source.HTTPClientFor(s.cfg.HTTPClient, s.cfg.WGClient, s.cfg.UseWireGuard)

	auth, err := authenticate(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("authenticating: %w", err)
	}

	if auth.UserInfo.Status != "Active" {
		err := fmt.Errorf("account not active: %s", auth.UserInfo.Status)
		s.SetError(err.Error())
		return err
	}

	categories, err := fetchCategories(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("fetching categories: %w", err)
	}

	categoryMap := make(map[string]string, len(categories))
	for _, cat := range categories {
		categoryMap[cat.ID] = cat.Name
	}

	liveStreams, err := fetchLiveStreams(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	if err != nil {
		s.SetError(err.Error())
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

	vodCats, _ := fetchVODCategories(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	vodCatMap := make(map[string]string, len(vodCats))
	for _, cat := range vodCats {
		vodCatMap[cat.ID] = cat.Name
	}

	vodStreams, err := fetchVODStreams(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	if err != nil {
		log.Printf("xtream: failed to fetch VOD streams for %s: %v", s.cfg.Name, err)
	} else {
		log.Printf("xtream: fetched %d VOD streams for %s", len(vodStreams), s.cfg.Name)
		for _, vs := range vodStreams {
			id := deterministicStreamID(s.cfg.ID, vs.StreamID)
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			keepIDs = append(keepIDs, id)

			name, year := parseNameAndYear(vs.Name)
			group := vodCatMap[vs.CategoryID]

			streams = append(streams, media.Stream{
				ID:         id,
				SourceType: "xtream",
				SourceID:   s.cfg.ID,
				Name:       name,
				URL:        vodStreamURL(s.cfg.Server, s.cfg.Username, s.cfg.Password, vs.StreamID, vs.ContainerExt),
				Group:      group,
				TvgLogo:    vs.Icon(),
				VODType:    "movie",
				Year:       year,
				IsActive:   true,
			})
		}
	}

	seriesCats, _ := fetchSeriesCategories(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	seriesCatMap := make(map[string]string, len(seriesCats))
	for _, cat := range seriesCats {
		seriesCatMap[cat.ID] = cat.Name
	}

	seriesList, err := fetchSeries(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	if err != nil {
		log.Printf("xtream: failed to fetch series for %s: %v", s.cfg.Name, err)
	} else {
		log.Printf("xtream: fetched %d series for %s, fetching episode info in background", len(seriesList), s.cfg.Name)
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("upserting streams: %w", err)
	}

	if _, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, "xtream", s.cfg.ID, keepIDs); err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("deleting stale streams: %w", err)
	}

	s.SetRefreshResult(len(streams))

	if s.cfg.OnRefreshDone != nil {
		s.cfg.OnRefreshDone(s.cfg.ID, "", len(streams))
	}

	if len(seriesList) > 0 {
		go s.fetchSeriesEpisodes(seriesList, seriesCatMap)
	}

	return nil
}

func (s *Source) fetchSeriesEpisodes(seriesList []Series, seriesCatMap map[string]string) {
	client := source.HTTPClientFor(s.cfg.HTTPClient, s.cfg.WGClient, s.cfg.UseWireGuard)

	ctx := context.Background()
	const batchSize = 50
	const delayBetween = 100 * time.Millisecond

	var allStreams []media.Stream
	var allIDs []string
	seen := make(map[string]struct{})

	for i, series := range seriesList {
		if i > 0 && i%batchSize == 0 {
			log.Printf("xtream: series progress for %s: %d/%d", s.cfg.Name, i, len(seriesList))

			if len(allStreams) > 0 {
				if err := s.cfg.StreamStore.BulkUpsert(ctx, allStreams); err != nil {
					log.Printf("xtream: failed to upsert series batch for %s: %v", s.cfg.Name, err)
				}
				allStreams = allStreams[:0]
			}
		}

		info, err := fetchSeriesInfo(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password, series.SeriesID)
		if err != nil {
			log.Printf("xtream: failed to fetch series info for %q (id=%d): %v", series.Name, series.SeriesID, err)
			time.Sleep(delayBetween)
			continue
		}

		seriesName, year := parseNameAndYear(series.Name)
		group := seriesCatMap[series.CategoryID]

		for seasonStr, episodes := range info.Seasons {
			seasonNum, _ := strconv.Atoi(seasonStr)

			for _, ep := range episodes {
				epID, _ := strconv.Atoi(ep.ID)
				if epID == 0 {
					continue
				}

				id := deterministicStreamID(s.cfg.ID, epID)
				if _, dup := seen[id]; dup {
					continue
				}
				seen[id] = struct{}{}
				allIDs = append(allIDs, id)

				season := seasonNum
				if ep.Info.SeasonNum() > 0 {
					season = ep.Info.SeasonNum()
				}
				epNum := ep.EpisodeNum

				epName := ep.Title
				displayName := fmt.Sprintf("%s - S%02dE%02d", seriesName, season, epNum)
				if epName != "" {
					displayName = fmt.Sprintf("%s - %s", displayName, epName)
				}

				ext := ep.ContainerExt
				if ext == "" {
					ext = "mkv"
				}

				allStreams = append(allStreams, media.Stream{
					ID:          id,
					SourceType:  "xtream",
					SourceID:    s.cfg.ID,
					Name:        displayName,
					URL:         seriesEpisodeURL(s.cfg.Server, s.cfg.Username, s.cfg.Password, epID, ext),
					Group:       group,
					TvgLogo:     series.Cover,
					VODType:     "series",
					SeriesName:  seriesName,
					Season:      season,
					Episode:     epNum,
					EpisodeName: epName,
					Year:        year,
					IsActive:    true,
				})
			}
		}

		time.Sleep(delayBetween)
	}

	if len(allStreams) > 0 {
		if err := s.cfg.StreamStore.BulkUpsert(ctx, allStreams); err != nil {
			log.Printf("xtream: failed to upsert final series batch for %s: %v", s.cfg.Name, err)
		}
	}

	s.AddStreamCount(len(allIDs))

	log.Printf("xtream: series episode fetch complete for %s: %d episodes from %d series", s.cfg.Name, len(allIDs), len(seriesList))
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

type AccountInfo struct {
	ServerURL        string `json:"server_url"`
	Username         string `json:"username"`
	Status           string `json:"status"`
	MaxConnections   string `json:"max_connections"`
	ActiveConnections string `json:"active_connections,omitempty"`
	ServerProtocol   string `json:"server_protocol"`
	LiveCategories   int    `json:"live_categories"`
	LiveStreams       int    `json:"live_streams"`
	VODStreams        int    `json:"vod_streams"`
	SeriesCount       int    `json:"series_count"`
}

func (s *Source) GetAccountInfo(ctx context.Context) (any, error) {
	client := source.HTTPClientFor(s.cfg.HTTPClient, s.cfg.WGClient, s.cfg.UseWireGuard)

	auth, err := authenticate(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("authenticating: %w", err)
	}

	categories, _ := fetchCategories(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	liveStreams, _ := fetchLiveStreams(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	vodStreams, _ := fetchVODStreams(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)
	series, _ := fetchSeries(ctx, client, s.cfg.Server, s.cfg.Username, s.cfg.Password)

	return &AccountInfo{
		ServerURL:      auth.ServerInfo.URL,
		Username:       auth.UserInfo.Username,
		Status:         auth.UserInfo.Status,
		MaxConnections: auth.UserInfo.MaxConnections,
		ActiveConnections: auth.UserInfo.ActiveConnections,
		ServerProtocol: auth.ServerInfo.ServerProtocol,
		LiveCategories: len(categories),
		LiveStreams:     len(liveStreams),
		VODStreams:      len(vodStreams),
		SeriesCount:     len(series),
	}, nil
}

func (s *Source) Clear(ctx context.Context) error {
	if err := s.cfg.StreamStore.DeleteBySource(ctx, "xtream", s.cfg.ID); err != nil {
		return err
	}

	s.ClearState()
	return nil
}

type UserInfo struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	Status            string `json:"status"`
	MaxConnections    string `json:"max_connections"`
	ActiveConnections string `json:"active_cons"`
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

func (vs VODStream) Icon() string {
	if str, ok := vs.StreamIcon.(string); ok {
		return str
	}
	return ""
}

type Series struct {
	Num        int    `json:"num"`
	Name       string `json:"name"`
	SeriesID   int    `json:"series_id"`
	Cover      string `json:"cover"`
	CategoryID string `json:"category_id"`
}

type SeriesInfo struct {
	RawSeasons json.RawMessage            `json:"episodes"`
	Seasons    map[string][]SeriesEpisode `json:"-"`
}

func (si *SeriesInfo) ParseSeasons() {
	if len(si.Seasons) > 0 {
		return
	}
	si.Seasons = make(map[string][]SeriesEpisode)
	if len(si.RawSeasons) == 0 {
		return
	}
	if err := json.Unmarshal(si.RawSeasons, &si.Seasons); err != nil {
		si.Seasons = make(map[string][]SeriesEpisode)
	}
}

func (si SeriesInfo) MarshalJSON() ([]byte, error) {
	if len(si.Seasons) > 0 {
		data, err := json.Marshal(si.Seasons)
		if err != nil {
			return nil, err
		}
		return json.Marshal(struct {
			Episodes json.RawMessage `json:"episodes"`
		}{Episodes: data})
	}
	return json.Marshal(struct {
		Episodes json.RawMessage `json:"episodes"`
	}{Episodes: si.RawSeasons})
}

type SeriesEpisode struct {
	ID           string            `json:"id"`
	EpisodeNum   int               `json:"episode_num"`
	Title        string            `json:"title"`
	ContainerExt string            `json:"container_extension"`
	Info         SeriesEpisodeInfo `json:"info"`
}

type SeriesEpisodeInfo struct {
	Season any `json:"season"`
}

func (i SeriesEpisodeInfo) SeasonNum() int {
	switch v := i.Season.(type) {
	case float64:
		return int(v)
	case string:
		n := 0
		fmt.Sscanf(v, "%d", &n)
		return n
	}
	return 0
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

func fetchVODStreams(ctx context.Context, client *http.Client, server, username, password string) ([]VODStream, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_vod_streams", server, username, password)
	var streams []VODStream
	if err := apiGet(ctx, client, url, &streams); err != nil {
		return nil, err
	}
	return streams, nil
}

func fetchSeries(ctx context.Context, client *http.Client, server, username, password string) ([]Series, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_series", server, username, password)
	var series []Series
	if err := apiGet(ctx, client, url, &series); err != nil {
		return nil, err
	}
	return series, nil
}

func fetchSeriesInfo(ctx context.Context, client *http.Client, server, username, password string, seriesID int) (*SeriesInfo, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_series_info&series_id=%d", server, username, password, seriesID)
	var info SeriesInfo
	if err := apiGet(ctx, client, url, &info); err != nil {
		return nil, err
	}
	info.ParseSeasons()
	return &info, nil
}

func fetchVODCategories(ctx context.Context, client *http.Client, server, username, password string) ([]Category, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_vod_categories", server, username, password)
	var categories []Category
	if err := apiGet(ctx, client, url, &categories); err != nil {
		return nil, err
	}
	return categories, nil
}

func fetchSeriesCategories(ctx context.Context, client *http.Client, server, username, password string) ([]Category, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_series_categories", server, username, password)
	var categories []Category
	if err := apiGet(ctx, client, url, &categories); err != nil {
		return nil, err
	}
	return categories, nil
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

func seriesEpisodeURL(server, username, password string, episodeID int, ext string) string {
	if ext == "" {
		ext = "mkv"
	}
	return fmt.Sprintf("%s/series/%s/%s/%d.%s", server, username, password, episodeID, ext)
}

var yearSuffixRe = regexp.MustCompile(`\s*\((\d{4})\)\s*$`)

func parseNameAndYear(raw string) (name, year string) {
	m := yearSuffixRe.FindStringSubmatch(raw)
	if m != nil {
		return strings.TrimSpace(raw[:len(raw)-len(m[0])]), m[1]
	}
	return raw, ""
}

func deterministicStreamID(sourceID string, streamID int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:xtream:%d", sourceID, streamID)))
	return fmt.Sprintf("%x", h[:16])
}
