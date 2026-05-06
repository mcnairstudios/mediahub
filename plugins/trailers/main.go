package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Host imports (provided by MediaHub runtime)
// ---------------------------------------------------------------------------

//go:wasmimport env host_log
func hostLog(level uint32, msgPtr uint32, msgLen uint32)

//go:wasmimport env host_http_request
func hostHTTPRequest(urlPtr, urlLen, methodPtr, methodLen, headersPtr, headersLen, bodyPtr, bodyLen uint32) uint64

//go:wasmimport env host_kv_get
func hostKVGet(keyPtr, keyLen uint32) uint64

//go:wasmimport env host_kv_set
func hostKVSet(keyPtr, keyLen, valPtr, valLen uint32)

// ---------------------------------------------------------------------------
// Memory management exports
// ---------------------------------------------------------------------------

//export alloc
func alloc(size uint32) uint32 {
	buf := make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&buf[0])))
}

//export dealloc
func dealloc(ptr uint32, size uint32) {
	// TinyGo GC handles this
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func packResult(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}
	ptr := uint32(uintptr(unsafe.Pointer(&data[0])))
	return (uint64(ptr) << 32) | uint64(len(data))
}

func readInput(ptr, length uint32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)
}

func returnJSON(v any) uint64 {
	data, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return packResult(data)
}

func logMsg(level uint32, msg string) {
	b := []byte(msg)
	hostLog(level, uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)))
}

func logInfo(msg string)  { logMsg(1, msg) }
func logError(msg string) { logMsg(3, msg) }

func httpGet(url string) ([]byte, error) {
	urlBytes := []byte(url)
	methodBytes := []byte("GET")
	headersBytes := []byte("{}")
	bodyBytes := []byte("")

	result := hostHTTPRequest(
		uint32(uintptr(unsafe.Pointer(&urlBytes[0]))), uint32(len(urlBytes)),
		uint32(uintptr(unsafe.Pointer(&methodBytes[0]))), uint32(len(methodBytes)),
		uint32(uintptr(unsafe.Pointer(&headersBytes[0]))), uint32(len(headersBytes)),
		uint32(uintptr(unsafe.Pointer(&bodyBytes[0]))), uint32(len(bodyBytes)),
	)

	if result == 0 {
		return nil, fmt.Errorf("http request failed: %s", url)
	}

	ptr := uint32(result >> 32)
	length := uint32(result & 0xFFFFFFFF)
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length), nil
}

// ---------------------------------------------------------------------------
// TMDB response types
// ---------------------------------------------------------------------------

type tmdbMovie struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	ReleaseDate string `json:"release_date"`
	PosterPath  string `json:"poster_path"`
	Overview    string `json:"overview"`
}

type tmdbMovieResponse struct {
	Results []tmdbMovie `json:"results"`
}

type tmdbVideo struct {
	Type string `json:"type"`
	Site string `json:"site"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type tmdbVideoResponse struct {
	Results []tmdbVideo `json:"results"`
}

// ---------------------------------------------------------------------------
// Stream/plugin types
// ---------------------------------------------------------------------------

type stream struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Group       string   `json:"group"`
	Logo        string   `json:"logo"`
	VodType     string   `json:"vod_type"`
	Year        string   `json:"year"`
	Tags        []string `json:"tags"`
	EpisodeName string   `json:"episode_name"`
}

type refreshResponse struct {
	Streams []stream `json:"streams"`
}

type configField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
}

type viewConfig struct {
	Layout     string `json:"layout"`
	GroupBy    string `json:"group_by"`
	Searchable bool   `json:"searchable"`
}

type describeResponse struct {
	Type         string        `json:"type"`
	Label        string        `json:"label"`
	ShortLabel   string        `json:"short_label"`
	Color        string        `json:"color"`
	Version      string        `json:"version"`
	Description  string        `json:"description"`
	ConfigFields []configField `json:"config_fields"`
	View         viewConfig    `json:"view"`
	Interactions []any         `json:"interactions"`
}

// ---------------------------------------------------------------------------
// Exported plugin functions
// ---------------------------------------------------------------------------

//export describe
func describe() uint64 {
	resp := describeResponse{
		Type:       "trailers",
		Label:      "Movie Trailers",
		ShortLabel: "TRAILERS",
		Color:      "#e91e63",
		Version:    "1.0.0",
		Description: "Browse trending and now-playing movie trailers from TMDB",
		ConfigFields: []configField{
			{
				Key:         "tmdb_key",
				Label:       "TMDB API Key",
				Type:        "password",
				Required:    true,
				Placeholder: "Enter your TMDB API key",
			},
		},
		View: viewConfig{
			Layout:     "grouped_list",
			GroupBy:    "group",
			Searchable: true,
		},
		Interactions: []any{},
	}
	return returnJSON(resp)
}

//export refresh
func refresh(configPtr uint32, configLen uint32) uint64 {
	input := readInput(configPtr, configLen)

	var cfg map[string]string
	if err := json.Unmarshal(input, &cfg); err != nil {
		logError("failed to parse config: " + err.Error())
		return returnJSON(refreshResponse{Streams: []stream{}})
	}

	apiKey := cfg["tmdb_key"]
	if apiKey == "" {
		logError("tmdb_key is required")
		return returnJSON(refreshResponse{Streams: []stream{}})
	}

	seen := make(map[int]bool)
	var streams []stream

	// Fetch trending movies
	trendingMovies := fetchMovieList(
		"https://api.themoviedb.org/3/trending/movie/week?api_key="+apiKey+"&language=en-GB",
		"trending movies",
	)
	for _, m := range trendingMovies {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		s := fetchTrailerStream(m, apiKey, "Trending")
		if s != nil {
			streams = append(streams, *s)
		}
		if len(streams) >= 40 {
			break
		}
	}

	// Fetch now playing
	if len(streams) < 40 {
		nowPlayingMovies := fetchMovieList(
			"https://api.themoviedb.org/3/movie/now_playing?api_key="+apiKey+"&language=en-GB",
			"now playing movies",
		)
		for _, m := range nowPlayingMovies {
			if seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			s := fetchTrailerStream(m, apiKey, "Now Playing")
			if s != nil {
				streams = append(streams, *s)
			}
			if len(streams) >= 40 {
				break
			}
		}
	}

	logInfo(fmt.Sprintf("trailers plugin: found %d streams", len(streams)))
	return returnJSON(refreshResponse{Streams: streams})
}

//export interact
func interact(actionPtr uint32, actionLen uint32) uint64 {
	empty := []byte("{}")
	return packResult(empty)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func fetchMovieList(url string, label string) []tmdbMovie {
	body, err := httpGet(url)
	if err != nil {
		logError("failed to fetch " + label + ": " + err.Error())
		return nil
	}

	var resp tmdbMovieResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		logError("failed to parse " + label + " response: " + err.Error())
		return nil
	}

	return resp.Results
}

func fetchTrailerStream(movie tmdbMovie, apiKey string, group string) *stream {
	url := fmt.Sprintf(
		"https://api.themoviedb.org/3/movie/%d/videos?api_key=%s&language=en-GB",
		movie.ID, apiKey,
	)

	body, err := httpGet(url)
	if err != nil {
		return nil
	}

	var resp tmdbVideoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	// Find first YouTube trailer
	for _, v := range resp.Results {
		if v.Type == "Trailer" && v.Site == "YouTube" {
			year := ""
			if len(movie.ReleaseDate) >= 4 {
				year = movie.ReleaseDate[:4]
			}

			logo := ""
			if movie.PosterPath != "" {
				logo = "https://image.tmdb.org/t/p/w300" + movie.PosterPath
			}

			return &stream{
				ID:          strconv.Itoa(movie.ID),
				Name:        movie.Title + " - " + v.Name,
				URL:         "https://www.youtube.com/watch?v=" + v.Key,
				Group:       group,
				Logo:        logo,
				VodType:     "movie",
				Year:        year,
				Tags:        []string{},
				EpisodeName: movie.Overview,
			}
		}
	}

	return nil
}

func main() {}
