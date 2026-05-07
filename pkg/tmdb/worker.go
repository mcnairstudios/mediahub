package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type MetadataWorker struct {
	store    *Store
	apiKey   func() string
	imageDir string
	client   *http.Client
}

func NewMetadataWorker(store *Store, apiKeyFn func() string, imageDir string) *MetadataWorker {
	return &MetadataWorker{
		store:    store,
		apiKey:   apiKeyFn,
		imageDir: imageDir,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (w *MetadataWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		key := w.apiKey()
		if key == "" {
			sleep(ctx, 30*time.Second)
			continue
		}

		entry, err := w.store.PickOldestResolving()
		if err != nil {
			log.Printf("tmdb worker: pick queue entry: %v", err)
			sleep(ctx, 5*time.Second)
			continue
		}
		if entry == nil {
			sleep(ctx, 5*time.Second)
			continue
		}

		switch entry.MediaType {
		case "movie":
			w.processMovie(ctx, key, entry)
		case "series":
			w.processSeries(ctx, key, entry)
		default:
			log.Printf("tmdb worker: unknown media type %q for tmdb_id=%d", entry.MediaType, entry.TMDBID)
			w.store.DeleteQueueEntry(entry.TMDBID)
		}

		sleep(ctx, 500*time.Millisecond)
	}
}

func (w *MetadataWorker) ResolveStreams(ctx context.Context, streams []StreamToResolve) []ResolvedStream {
	key := w.apiKey()
	if key == "" {
		return nil
	}

	var resolved []ResolvedStream
	seen := make(map[string]int)
	apiErrors := 0
	const maxAPIErrors = 10

	for _, st := range streams {
		select {
		case <-ctx.Done():
			return resolved
		default:
		}

		if apiErrors >= maxAPIErrors {
			log.Printf("tmdb resolver: stopping batch early after %d API errors", apiErrors)
			break
		}

		tmdbID := 0
		if st.TMDBID != "" {
			tmdbID, _ = strconv.Atoi(st.TMDBID)
		}

		mediaType := st.MediaType
		if mediaType == "episode" {
			mediaType = "series"
		}

		if tmdbID > 0 {
			normalized := NormalizeName(st.Name)
			if normalized != "" {
				w.store.SetName(normalized, tmdbID, mediaType)
			}
			if has, _ := w.store.HasBlobTyped(mediaType, tmdbID); !has {
				w.store.EnqueueMetadata(QueueEntry{
					TMDBID:    tmdbID,
					MediaType: mediaType,
					Status:    "resolving",
					CreatedAt: time.Now().Unix(),
				})
			}
			resolved = append(resolved, ResolvedStream{StreamID: st.StreamID, TMDBID: tmdbID})
			continue
		}

		normalized := NormalizeName(st.Name)
		if normalized == "" {
			continue
		}

		if cachedID, ok := seen[normalized]; ok {
			if cachedID > 0 {
				resolved = append(resolved, ResolvedStream{StreamID: st.StreamID, TMDBID: cachedID})
			}
			continue
		}

		if indexID, indexType, found := w.store.LookupName(normalized); found {
			seen[normalized] = indexID
			_ = indexType
			if indexID == 0 {
				// Negative cache entry — previously searched with no result.
				continue
			}
			if has, _ := w.store.HasBlob(indexID); !has {
				w.store.EnqueueMetadata(QueueEntry{
					TMDBID:    indexID,
					MediaType: mediaType,
					Status:    "resolving",
					CreatedAt: time.Now().Unix(),
				})
			}
			resolved = append(resolved, ResolvedStream{StreamID: st.StreamID, TMDBID: indexID})
			continue
		}

		searchID, searchErr := w.searchTMDB(ctx, key, st.Name, st.Year, mediaType)
		if searchErr != nil {
			apiErrors++
			log.Printf("tmdb resolver: search error (%d/%d): %v", apiErrors, maxAPIErrors, searchErr)
			seen[normalized] = 0
			continue
		}
		if searchID <= 0 {
			// No results — store negative cache entry so we never search again.
			seen[normalized] = 0
			w.store.SetName(normalized, 0, mediaType)
			continue
		}

		seen[normalized] = searchID
		w.store.SetName(normalized, searchID, mediaType)

		if has, _ := w.store.HasBlob(searchID); !has {
			w.store.EnqueueMetadata(QueueEntry{
				TMDBID:    searchID,
				MediaType: mediaType,
				Status:    "resolving",
				CreatedAt: time.Now().Unix(),
			})
		}

		resolved = append(resolved, ResolvedStream{StreamID: st.StreamID, TMDBID: searchID})
		sleep(ctx, 250*time.Millisecond)
	}

	return resolved
}

func (w *MetadataWorker) searchTMDB(ctx context.Context, apiKey, name, year, mediaType string) (int, error) {
	clean, extractedYear := cleanVODName(name)
	if year == "" {
		year = extractedYear
	}

	var u string
	switch mediaType {
	case "series":
		u = "https://api.themoviedb.org/3/search/tv?api_key=" + url.QueryEscape(apiKey) +
			"&query=" + url.QueryEscape(clean) + "&language=en-GB"
	default:
		u = "https://api.themoviedb.org/3/search/movie?api_key=" + url.QueryEscape(apiKey) +
			"&query=" + url.QueryEscape(clean) + "&language=en-GB"
		if year != "" {
			u += "&year=" + url.QueryEscape(year)
		}
	}

	raw, err := w.apiGet(ctx, u)
	if err != nil {
		return 0, fmt.Errorf("search %q: %w", clean, err)
	}

	var result struct {
		Results []struct {
			ID int `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return 0, fmt.Errorf("decode search %q: %w", clean, err)
	}

	if len(result.Results) == 0 {
		return 0, nil
	}

	return result.Results[0].ID, nil
}

func (w *MetadataWorker) processMovie(ctx context.Context, apiKey string, entry *QueueEntry) {
	u := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?api_key=%s&language=en-GB&append_to_response=credits,release_dates",
		entry.TMDBID, url.QueryEscape(apiKey))

	raw, err := w.apiGet(ctx, u)
	if err != nil {
		log.Printf("tmdb worker: movie %d: %v", entry.TMDBID, err)
		if isPermanent(err) {
			w.store.DeleteQueueEntry(entry.TMDBID)
		}
		return
	}

	var resp movieWorkerResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		log.Printf("tmdb worker: movie %d decode: %v", entry.TMDBID, err)
		return
	}

	blob := MovieBlob{
		ID:        resp.ID,
		MediaType: "movie",
		Title:     resp.Title,
		Overview:  resp.Overview,
		Year:      extractYear(resp.ReleaseDate),
		Rating:    resp.VoteAverage,
		Runtime:   resp.Runtime,
	}

	for _, g := range resp.Genres {
		blob.Genres = append(blob.Genres, g.Name)
	}

	blob.Certification = extractMovieCertification(resp.ReleaseDates, "GB", "US")

	if resp.Credits.Cast != nil {
		limit := 20
		if len(resp.Credits.Cast) < limit {
			limit = len(resp.Credits.Cast)
		}
		for _, p := range resp.Credits.Cast[:limit] {
			blob.Cast = append(blob.Cast, CastMember{
				Name:      p.Name,
				Character: p.Character,
				TMDBID:    p.ID,
			})
			if p.ProfilePath != "" {
				w.enqueueImage(fmt.Sprintf("people/%d.jpg", p.ID), p.ProfilePath, "w185")
			}
		}
	}

	if resp.Credits.Crew != nil {
		for _, p := range resp.Credits.Crew {
			if p.Job == "Director" || p.Job == "Writer" || p.Job == "Screenplay" {
				blob.Crew = append(blob.Crew, CrewMember{
					Name:       p.Name,
					Job:        p.Job,
					Department: p.Department,
					TMDBID:     p.ID,
				})
			}
		}
	}

	if resp.BelongsToCollection.ID > 0 {
		blob.CollectionID = resp.BelongsToCollection.ID
		blob.CollectionName = resp.BelongsToCollection.Name
		if resp.BelongsToCollection.PosterPath != "" {
			w.enqueueImage(
				fmt.Sprintf("%d/collection_poster.jpg", entry.TMDBID),
				resp.BelongsToCollection.PosterPath, "w500",
			)
		}
		if resp.BelongsToCollection.BackdropPath != "" {
			w.enqueueImage(
				fmt.Sprintf("%d/collection_backdrop.jpg", entry.TMDBID),
				resp.BelongsToCollection.BackdropPath, "w1280",
			)
		}
	}

	if resp.PosterPath != "" {
		w.enqueueImage(fmt.Sprintf("%d/poster.jpg", entry.TMDBID), resp.PosterPath, "w500")
	}
	if resp.BackdropPath != "" {
		w.enqueueImage(fmt.Sprintf("%d/backdrop.jpg", entry.TMDBID), resp.BackdropPath, "w1280")
	}

	data, err := json.Marshal(blob)
	if err != nil {
		log.Printf("tmdb worker: movie %d marshal: %v", entry.TMDBID, err)
		return
	}

	if err := w.store.WriteBlobTyped(entry.MediaType, entry.TMDBID, data); err != nil {
		log.Printf("tmdb worker: movie %d write blob: %v", entry.TMDBID, err)
		return
	}

	w.store.DeleteQueueEntry(entry.TMDBID)
	log.Printf("tmdb worker: resolved movie %d (%s)", entry.TMDBID, blob.Title)
}

func (w *MetadataWorker) processSeries(ctx context.Context, apiKey string, entry *QueueEntry) {
	var seriesResp tvWorkerResponse

	if len(entry.SeasonsDone) == 0 {
		u := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d?api_key=%s&language=en-GB&append_to_response=credits,content_ratings",
			entry.TMDBID, url.QueryEscape(apiKey))

		raw, err := w.apiGet(ctx, u)
		if err != nil {
			log.Printf("tmdb worker: series %d: %v", entry.TMDBID, err)
			if isPermanent(err) {
				w.store.DeleteQueueEntry(entry.TMDBID)
			}
			return
		}

		if err := json.Unmarshal(raw, &seriesResp); err != nil {
			log.Printf("tmdb worker: series %d decode: %v", entry.TMDBID, err)
			return
		}

		if seriesResp.PosterPath != "" {
			w.enqueueImage(fmt.Sprintf("%d/poster.jpg", entry.TMDBID), seriesResp.PosterPath, "w500")
		}
		if seriesResp.BackdropPath != "" {
			w.enqueueImage(fmt.Sprintf("%d/backdrop.jpg", entry.TMDBID), seriesResp.BackdropPath, "w1280")
		}

		sleep(ctx, 250*time.Millisecond)
	} else {
		u := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d?api_key=%s&language=en-GB",
			entry.TMDBID, url.QueryEscape(apiKey))

		raw, err := w.apiGet(ctx, u)
		if err != nil {
			log.Printf("tmdb worker: series %d refetch: %v", entry.TMDBID, err)
			return
		}

		if err := json.Unmarshal(raw, &seriesResp); err != nil {
			log.Printf("tmdb worker: series %d decode: %v", entry.TMDBID, err)
			return
		}

		sleep(ctx, 250*time.Millisecond)
	}

	doneSet := make(map[int]bool)
	for _, s := range entry.SeasonsDone {
		doneSet[s] = true
	}

	blob := SeriesBlob{
		ID:        seriesResp.ID,
		MediaType: "series",
		Name:      seriesResp.Name,
		Overview:  seriesResp.Overview,
		Year:      extractYear(seriesResp.FirstAirDate),
		Rating:    seriesResp.VoteAverage,
	}

	for _, g := range seriesResp.Genres {
		blob.Genres = append(blob.Genres, g.Name)
	}

	for _, sb := range seriesResp.Seasons {
		seasonBlob := SeasonBlob{
			SeasonNumber: sb.SeasonNumber,
			Name:         sb.Name,
			Overview:     sb.Overview,
			EpisodeCount: sb.EpisodeCount,
		}

		if sb.SeasonNumber > 0 && !doneSet[sb.SeasonNumber] {
			episodes, err := w.fetchSeason(ctx, apiKey, entry.TMDBID, sb.SeasonNumber, sb.PosterPath)
			if err != nil {
				log.Printf("tmdb worker: series %d season %d: %v", entry.TMDBID, sb.SeasonNumber, err)
				blob.Seasons = append(blob.Seasons, seasonBlob)

				w.checkpointSeasons(entry)
				continue
			}
			seasonBlob.Episodes = episodes
			entry.SeasonsDone = append(entry.SeasonsDone, sb.SeasonNumber)

			w.checkpointSeasons(entry)
			sleep(ctx, 250*time.Millisecond)
		}

		blob.Seasons = append(blob.Seasons, seasonBlob)
	}

	data, err := json.Marshal(blob)
	if err != nil {
		log.Printf("tmdb worker: series %d marshal: %v", entry.TMDBID, err)
		return
	}

	if err := w.store.WriteBlobTyped(entry.MediaType, entry.TMDBID, data); err != nil {
		log.Printf("tmdb worker: series %d write blob: %v", entry.TMDBID, err)
		return
	}

	w.store.DeleteQueueEntry(entry.TMDBID)
	log.Printf("tmdb worker: resolved series %d (%s, %d seasons)", entry.TMDBID, blob.Name, len(blob.Seasons))
}

func (w *MetadataWorker) fetchSeason(ctx context.Context, apiKey string, tvID, seasonNum int, seasonPosterPath string) ([]EpisodeBlob, error) {
	u := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/season/%d?api_key=%s&language=en-GB",
		tvID, seasonNum, url.QueryEscape(apiKey))

	raw, err := w.apiGet(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp seasonWorkerResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode season %d: %w", seasonNum, err)
	}

	if resp.PosterPath != "" {
		w.enqueueImage(fmt.Sprintf("%d/s%d_poster.jpg", tvID, seasonNum), resp.PosterPath, "w342")
	} else if seasonPosterPath != "" {
		w.enqueueImage(fmt.Sprintf("%d/s%d_poster.jpg", tvID, seasonNum), seasonPosterPath, "w342")
	}

	var episodes []EpisodeBlob
	for _, ep := range resp.Episodes {
		episodes = append(episodes, EpisodeBlob{
			EpisodeNumber: ep.EpisodeNumber,
			Name:          ep.Name,
			Overview:      ep.Overview,
			AirDate:       ep.AirDate,
			Runtime:       ep.Runtime,
		})
		if ep.StillPath != "" {
			w.enqueueImage(
				fmt.Sprintf("%d/s%de%d.jpg", tvID, seasonNum, ep.EpisodeNumber),
				ep.StillPath, "w300",
			)
		}
	}

	return episodes, nil
}

func (w *MetadataWorker) checkpointSeasons(entry *QueueEntry) {
	if err := w.store.UpdateQueueEntry(*entry); err != nil {
		log.Printf("tmdb worker: checkpoint seasons for %d: %v", entry.TMDBID, err)
	}
}

func (w *MetadataWorker) enqueueImage(localPath, tmdbPath, size string) {
	if err := w.store.EnqueueImage(localPath, ImageQueueEntry{
		TMDBPath: tmdbPath,
		Size:     size,
	}); err != nil {
		log.Printf("tmdb worker: enqueue image %s: %v", localPath, err)
	}
}

// permanentError indicates a non-retriable TMDB error (404, 401, etc).
type permanentError struct {
	code int
}

func (e *permanentError) Error() string {
	return fmt.Sprintf("TMDB returned %d", e.code)
}

func isPermanent(err error) bool {
	var pe *permanentError
	return errors.As(err, &pe)
}

func (w *MetadataWorker) apiGet(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("TMDB rate limited (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &permanentError{code: resp.StatusCode}
	}

	return io.ReadAll(resp.Body)
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func extractYear(dateStr string) string {
	if len(dateStr) >= 4 {
		return dateStr[:4]
	}
	return ""
}

type movieWorkerResponse struct {
	ID                  int                     `json:"id"`
	Title               string                  `json:"title"`
	Overview            string                  `json:"overview"`
	PosterPath          string                  `json:"poster_path"`
	BackdropPath        string                  `json:"backdrop_path"`
	ReleaseDate         string                  `json:"release_date"`
	VoteAverage         float64                 `json:"vote_average"`
	Runtime             int                     `json:"runtime"`
	Genres              []genre                 `json:"genres"`
	BelongsToCollection movieCollectionResponse `json:"belongs_to_collection"`
	Credits             credits                 `json:"credits"`
	ReleaseDates        releaseDates            `json:"release_dates"`
}

type movieCollectionResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
}

type tvWorkerResponse struct {
	ID             int                  `json:"id"`
	Name           string               `json:"name"`
	Overview       string               `json:"overview"`
	PosterPath     string               `json:"poster_path"`
	BackdropPath   string               `json:"backdrop_path"`
	FirstAirDate   string               `json:"first_air_date"`
	VoteAverage    float64              `json:"vote_average"`
	Genres         []genre              `json:"genres"`
	Seasons        []seasonBriefWorker  `json:"seasons"`
	ContentRatings contentRatings       `json:"content_ratings"`
}

type seasonBriefWorker struct {
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	EpisodeCount int    `json:"episode_count"`
}

type contentRatings struct {
	Results []contentRating `json:"results"`
}

type contentRating struct {
	ISO    string `json:"iso_3166_1"`
	Rating string `json:"rating"`
}

type seasonWorkerResponse struct {
	SeasonNumber int                   `json:"season_number"`
	Name         string                `json:"name"`
	Overview     string                `json:"overview"`
	PosterPath   string                `json:"poster_path"`
	Episodes     []episodeWorkerDetail `json:"episodes"`
}

type episodeWorkerDetail struct {
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	StillPath     string `json:"still_path"`
	AirDate       string `json:"air_date"`
	Runtime       int    `json:"runtime"`
}

