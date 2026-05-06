package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"unsafe"
)

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
// Host imports
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

func logInfo(msg string) {
	b := []byte(msg)
	hostLog(1, uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)))
}

func logError(msg string) {
	b := []byte(msg)
	hostLog(3, uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)))
}

func ptrLen(b []byte) (uint32, uint32) {
	if len(b) == 0 {
		// TinyGo needs a valid pointer even for zero-length; allocate 1 byte.
		dummy := []byte{0}
		return uint32(uintptr(unsafe.Pointer(&dummy[0]))), 0
	}
	return uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b))
}

func httpGet(url string) ([]byte, error) {
	urlBytes := []byte(url)
	methodBytes := []byte("GET")
	headersBytes := []byte(`{"User-Agent":"Mozilla/5.0"}`)
	bodyBytes := []byte("")

	urlPtr, urlLen := ptrLen(urlBytes)
	methodPtr, methodLen := ptrLen(methodBytes)
	headersPtr, headersLen := ptrLen(headersBytes)
	bodyPtr, bodyLen := ptrLen(bodyBytes)

	result := hostHTTPRequest(
		urlPtr, urlLen,
		methodPtr, methodLen,
		headersPtr, headersLen,
		bodyPtr, bodyLen,
	)

	if result == 0 {
		return nil, fmt.Errorf("http request failed for %s", url)
	}

	rPtr := uint32(result >> 32)
	rLen := uint32(result & 0xFFFFFFFF)
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(rPtr))), rLen), nil
}

func kvGet(key string) string {
	kb := []byte(key)
	kPtr, kLen := ptrLen(kb)
	result := hostKVGet(kPtr, kLen)
	if result == 0 {
		return ""
	}
	rPtr := uint32(result >> 32)
	rLen := uint32(result & 0xFFFFFFFF)
	return string(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(rPtr))), rLen))
}

func kvSet(key, value string) {
	kb := []byte(key)
	vb := []byte(value)
	kPtr, kLen := ptrLen(kb)
	vPtr, vLen := ptrLen(vb)
	hostKVSet(kPtr, kLen, vPtr, vLen)
}

// ---------------------------------------------------------------------------
// describe
// ---------------------------------------------------------------------------

//export describe
func describe() uint64 {
	meta := map[string]any{
		"type":        "radiogarden",
		"label":       "Radio Garden",
		"short_label": "RADIO",
		"color":       "#43a047",
		"version":     "1.0.0",
		"description": "Live radio streams from Radio Garden",
		"config_fields": []map[string]any{
			{
				"key":       "places",
				"label":     "Locations",
				"type":      "custom",
				"component": "place-picker",
			},
		},
		"view": map[string]any{
			"layout":     "grouped_list",
			"group_by":   "group",
			"searchable": true,
			"sortable":   true,
		},
		"interactions": []map[string]any{
			{
				"id":           "search_places",
				"label":        "Search Location",
				"type":         "search",
				"target_field": "places",
			},
		},
	}
	return returnJSON(meta)
}

// ---------------------------------------------------------------------------
// refresh
// ---------------------------------------------------------------------------

type configPlace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type stream struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Group       string   `json:"group"`
	Logo        string   `json:"logo"`
	VODType     string   `json:"vod_type"`
	Tags        []string `json:"tags"`
	EpisodeName string   `json:"episode_name,omitempty"`
}

type channelPage struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type channelItem struct {
	Page channelPage `json:"page"`
}

type contentSection struct {
	Items []channelItem `json:"items"`
}

type channelsData struct {
	Content []contentSection `json:"content"`
}

type channelsResponse struct {
	Data channelsData `json:"data"`
}

//export refresh
func refresh(configPtr uint32, configLen uint32) uint64 {
	input := readInput(configPtr, configLen)

	var config map[string]json.RawMessage
	if err := json.Unmarshal(input, &config); err != nil {
		logError("failed to parse config: " + err.Error())
		return returnJSON(map[string]any{"streams": []any{}})
	}

	var places []configPlace
	if raw, ok := config["places"]; ok {
		// places may be a JSON string (stringified array) or a direct array.
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			// It was a string — parse the inner JSON.
			json.Unmarshal([]byte(s), &places)
		} else {
			json.Unmarshal(raw, &places)
		}
	}

	if len(places) == 0 {
		logInfo("no places configured")
		return returnJSON(map[string]any{"streams": []any{}})
	}

	seen := make(map[string]bool)
	var streams []stream

	for _, place := range places {
		url := "https://radio.garden/api/ara/content/page/" + place.ID + "/channels"
		body, err := httpGet(url)
		if err != nil {
			logError("failed to fetch channels for " + place.Name + ": " + err.Error())
			continue
		}

		var resp channelsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			logError("failed to parse channels for " + place.Name + ": " + err.Error())
			continue
		}

		for _, section := range resp.Data.Content {
			for _, item := range section.Items {
				channelID := extractChannelID(item.Page.URL)
				if channelID == "" {
					continue
				}
				if seen[channelID] {
					continue
				}
				seen[channelID] = true

				streams = append(streams, stream{
					ID:      channelID,
					Name:    item.Page.Title,
					URL:     "https://radio.garden/api/ara/content/listen/" + channelID + "/channel.mp3",
					Group:   place.Name,
					Logo:    "",
					VODType: "",
					Tags:    []string{},
				})
			}
		}

		logInfo(fmt.Sprintf("fetched %d channels for %s", len(resp.Data.Content), place.Name))
	}

	logInfo(fmt.Sprintf("refresh complete: %d streams from %d places", len(streams), len(places)))
	return returnJSON(map[string]any{"streams": streams})
}

// extractChannelID returns the last path segment from a URL like
// "/listen/bbc-radio-1/hYpXtjOZ".
func extractChannelID(urlPath string) string {
	idx := strings.LastIndex(urlPath, "/")
	if idx < 0 || idx == len(urlPath)-1 {
		return ""
	}
	return urlPath[idx+1:]
}

// ---------------------------------------------------------------------------
// interact
// ---------------------------------------------------------------------------

type placeEntry struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Country string `json:"country"`
	Size    int    `json:"size"`
}

type placesListData struct {
	List []placeEntry `json:"list"`
}

type placesResponse struct {
	Data placesListData `json:"data"`
}

type searchResult struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

//export interact
func interact(actionPtr uint32, actionLen uint32) uint64 {
	input := readInput(actionPtr, actionLen)

	var req struct {
		Action string         `json:"action"`
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		logError("failed to parse interact request: " + err.Error())
		return returnJSON(map[string]any{})
	}

	if req.Action != "search_places" {
		return returnJSON(map[string]any{})
	}

	query, _ := req.Params["query"].(string)
	if query == "" {
		return returnJSON(map[string]any{"results": []any{}})
	}

	places := loadPlaces()
	if places == nil {
		return returnJSON(map[string]any{"results": []any{}})
	}

	queryLower := strings.ToLower(query)
	var results []searchResult
	for _, p := range places {
		if len(results) >= 20 {
			break
		}
		titleLower := strings.ToLower(p.Title)
		countryLower := strings.ToLower(p.Country)
		if strings.Contains(titleLower, queryLower) || strings.Contains(countryLower, queryLower) {
			results = append(results, searchResult{
				ID:       p.ID,
				Title:    p.Title + ", " + p.Country,
				Subtitle: fmt.Sprintf("%d stations", p.Size),
			})
		}
	}

	return returnJSON(map[string]any{"results": results})
}

// loadPlaces returns the places list, using the KV cache when available.
func loadPlaces() []placeEntry {
	cached := kvGet("places_cache")
	if cached != "" {
		var places []placeEntry
		if err := json.Unmarshal([]byte(cached), &places); err == nil && len(places) > 0 {
			return places
		}
	}

	logInfo("fetching places list from Radio Garden API")
	body, err := httpGet("https://radio.garden/api/ara/content/places")
	if err != nil {
		logError("failed to fetch places: " + err.Error())
		return nil
	}

	var resp placesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		logError("failed to parse places: " + err.Error())
		return nil
	}

	logInfo(fmt.Sprintf("fetched %d places, caching", len(resp.Data.List)))

	cacheData, _ := json.Marshal(resp.Data.List)
	kvSet("places_cache", string(cacheData))

	return resp.Data.List
}

// ---------------------------------------------------------------------------
// main (required by TinyGo)
// ---------------------------------------------------------------------------

func main() {}
