package radiogarden

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/mcnairstudios/mediahub/pkg/store"
)

var apiBaseOverride string

func apiBase() string {
	if apiBaseOverride != "" {
		return apiBaseOverride
	}
	return "https://radio.garden/api/ara/content"
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Place represents a Radio Garden location.
type Place struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ParsePlaces parses a JSON string into a slice of Place.
func ParsePlaces(jsonStr string) ([]Place, error) {
	if strings.TrimSpace(jsonStr) == "" {
		return nil, nil
	}
	var places []Place
	if err := json.Unmarshal([]byte(jsonStr), &places); err != nil {
		return nil, fmt.Errorf("parsing places JSON: %w", err)
	}
	return places, nil
}

type Config struct {
	ID            string
	Name          string
	IsEnabled     bool
	Places        []Place
	StreamStore   store.StreamStore
	HTTPClient    *http.Client
	OnRefreshDone func(sourceID, etag string, streamCount int)
}

type Source struct {
	source.BaseSource
	cfg Config
}

func New(cfg Config) *Source {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				req.Header.Set("User-Agent", userAgent)
				return nil
			},
		}
	}
	return &Source{
		BaseSource: source.NewBaseSource(cfg.ID, cfg.Name, source.TypeRadioGarden, cfg.IsEnabled, 0),
		cfg:        cfg,
	}
}

// Radio Garden API response types.

type placeChannelsResponse struct {
	Data struct {
		Content []struct {
			Items []struct {
				Page channelPage `json:"page"`
			} `json:"items"`
		} `json:"content"`
	} `json:"data"`
}

type channelPage struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Place   struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"place"`
	Country struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"country"`
	Website string `json:"website,omitempty"`
	Stream  string `json:"stream,omitempty"`
}

// extractChannelID extracts the channel ID from a URL path like "/listen/station-name/hYpXtjOZ".
func extractChannelID(urlPath string) string {
	parts := strings.Split(strings.TrimRight(urlPath, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func (s *Source) Refresh(ctx context.Context) error {
	if len(s.cfg.Places) == 0 {
		s.SetError("no places configured")
		return fmt.Errorf("radiogarden: no places configured")
	}

	log.Printf("radiogarden: refreshing channels for %d place(s)", len(s.cfg.Places))

	seen := make(map[string]struct{})
	var streams []media.Stream
	var keepIDs []string
	var lastErr error
	successCount := 0

	for _, place := range s.cfg.Places {
		log.Printf("radiogarden: fetching channels for place %s (%s)", place.Name, place.ID)

		channels, err := s.fetchChannels(ctx, place.ID)
		if err != nil {
			log.Printf("radiogarden: failed to fetch channels for place %s (%s): %v", place.Name, place.ID, err)
			lastErr = err
			continue
		}
		successCount++

		log.Printf("radiogarden: fetched %d channels for %s", len(channels), place.Name)

		for _, ch := range channels {
			channelID := extractChannelID(ch.URL)
			if channelID == "" {
				continue
			}

			id := deterministicStreamID(s.cfg.ID, channelID)
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			keepIDs = append(keepIDs, id)

			// Store the Radio Garden listen URL directly. The demuxer follows the 302
			// redirect to the actual Icecast/Shoutcast MP3 stream.
			streamURL := apiBase() + "/listen/" + channelID + "/channel.mp3"

			group := place.Name
			if group == "" && ch.Place.Title != "" {
				group = ch.Place.Title
			}

			st := media.Stream{
				ID:         id,
				SourceType: string(source.TypeRadioGarden),
				SourceID:   s.cfg.ID,
				Name:       ch.Title,
				URL:        streamURL,
				Group:      group,
				IsActive:   true,
			}
			streams = append(streams, st)
		}
	}

	if successCount == 0 {
		errMsg := "all places failed"
		if lastErr != nil {
			errMsg = fmt.Sprintf("all places failed, last error: %v", lastErr)
		}
		s.SetError(errMsg)
		return fmt.Errorf("radiogarden: %s", errMsg)
	}

	log.Printf("radiogarden: built %d streams from %d/%d places", len(streams), successCount, len(s.cfg.Places))

	if len(streams) == 0 {
		s.SetRefreshResult(0)
		return nil
	}

	if err := s.cfg.StreamStore.BulkUpsert(ctx, streams); err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("upserting streams: %w", err)
	}

	deleted, err := s.cfg.StreamStore.DeleteStaleBySource(ctx, string(source.TypeRadioGarden), s.cfg.ID, keepIDs)
	if err != nil {
		s.SetError(err.Error())
		return fmt.Errorf("deleting stale streams: %w", err)
	}
	log.Printf("radiogarden: upserted %d, deleted %d stale", len(streams), len(deleted))

	s.SetRefreshResult(len(streams))
	if s.cfg.OnRefreshDone != nil {
		s.cfg.OnRefreshDone(s.cfg.ID, "", len(streams))
	}
	return nil
}

func (s *Source) Streams(ctx context.Context) ([]string, error) {
	streams, err := s.cfg.StreamStore.ListBySource(ctx, string(source.TypeRadioGarden), s.cfg.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(streams))
	for i, st := range streams {
		ids[i] = st.ID
	}
	return ids, nil
}

func (s *Source) DeleteStreams(ctx context.Context) error {
	return s.cfg.StreamStore.DeleteBySource(ctx, string(source.TypeRadioGarden), s.cfg.ID)
}

// fetchChannels fetches all channels for a given place ID.
func (s *Source) fetchChannels(ctx context.Context, placeID string) ([]channelPage, error) {
	url := apiBase() + "/page/" + placeID + "/channels"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Radio Garden API returned %d for place %s", resp.StatusCode, placeID)
	}

	var result placeChannelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var channels []channelPage
	for _, content := range result.Data.Content {
		for _, item := range content.Items {
			channels = append(channels, item.Page)
		}
	}
	return channels, nil
}

func deterministicStreamID(sourceID, channelID string) string {
	h := sha256.Sum256([]byte(sourceID + ":" + channelID))
	return fmt.Sprintf("%x", h[:16])
}
