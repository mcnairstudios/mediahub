package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Memory management (required WASM exports)
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
// Host convenience wrappers
// ---------------------------------------------------------------------------

func logMsg(level uint32, msg string) {
	b := []byte(msg)
	if len(b) == 0 {
		return
	}
	hostLog(level, uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)))
}

func logInfo(msg string)  { logMsg(1, msg) }
func logWarn(msg string)  { logMsg(2, msg) }
func logError(msg string) { logMsg(3, msg) }

func httpGet(url string) ([]byte, error) {
	urlBytes := []byte(url)
	methodBytes := []byte("GET")
	headersBytes := []byte("{}")

	result := hostHTTPRequest(
		uint32(uintptr(unsafe.Pointer(&urlBytes[0]))), uint32(len(urlBytes)),
		uint32(uintptr(unsafe.Pointer(&methodBytes[0]))), uint32(len(methodBytes)),
		uint32(uintptr(unsafe.Pointer(&headersBytes[0]))), uint32(len(headersBytes)),
		0, 0,
	)

	if result == 0 {
		return nil, fmt.Errorf("http request failed for %s", url)
	}

	ptr := uint32(result >> 32)
	length := uint32(result & 0xFFFFFFFF)
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length), nil
}

// ---------------------------------------------------------------------------
// Describe
// ---------------------------------------------------------------------------

type ConfigField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
	Default     string `json:"default,omitempty"`
}

type ViewConfig struct {
	Layout     string `json:"layout"`
	GroupBy    string `json:"group_by"`
	Searchable bool   `json:"searchable"`
	Sortable   bool   `json:"sortable"`
}

type DescribeResponse struct {
	Type         string        `json:"type"`
	Label        string        `json:"label"`
	ShortLabel   string        `json:"short_label"`
	Color        string        `json:"color"`
	Version      string        `json:"version"`
	Description  string        `json:"description"`
	ConfigFields []ConfigField `json:"config_fields"`
	View         ViewConfig    `json:"view"`
	Interactions []any         `json:"interactions"`
}

//export describe
func describe() uint64 {
	resp := DescribeResponse{
		Type:         "spacex",
		Label:        "Space Launches",
		ShortLabel:   "SPACE",
		Color:        "#1e88e5",
		Version:      "1.0.0",
		Description:  "Space launch streams from the Launch Library 2 API (thespacedevs.com)",
		ConfigFields: []ConfigField{},
		View: ViewConfig{
			Layout:     "grouped_list",
			GroupBy:    "group",
			Searchable: true,
			Sortable:   true,
		},
		Interactions: []any{},
	}
	return returnJSON(resp)
}

// ---------------------------------------------------------------------------
// API types — flexible to handle both detailed and list mode
// ---------------------------------------------------------------------------

type apiResponse struct {
	Count   int             `json:"count"`
	Next    *string         `json:"next"`
	Results json.RawMessage `json:"results"`
}

type vidURL struct {
	URL      string `json:"url"`
	Priority int    `json:"priority"`
}

// launchDetailed handles the detailed mode response where nested objects are full.
type launchDetailed struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Net   string `json:"net"`
	Image string `json:"image"`

	Status  json.RawMessage `json:"status"`
	LSP     json.RawMessage `json:"launch_service_provider"`
	Mission json.RawMessage `json:"mission"`

	// list mode flat fields
	LSPName string `json:"lsp_name"`

	VidURLs []vidURL `json:"vidURLs"`
}

type statusObj struct {
	ID     int    `json:"id"`
	Abbrev string `json:"abbrev"`
}

type lspObj struct {
	Name string `json:"name"`
}

type missionObj struct {
	Description string `json:"description"`
}

// ---------------------------------------------------------------------------
// Stream type
// ---------------------------------------------------------------------------

type Stream struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Group       string   `json:"group"`
	Logo        string   `json:"logo,omitempty"`
	VodType     string   `json:"vod_type"`
	Year        string   `json:"year,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	EpisodeName string   `json:"episode_name,omitempty"`
}

type RefreshResponse struct {
	Streams []Stream `json:"streams"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func formatDate(netStr string) (formatted string, year string) {
	t, err := time.Parse(time.RFC3339, netStr)
	if err != nil {
		// Try without timezone
		t, err = time.Parse("2006-01-02T15:04:05Z", netStr)
		if err != nil {
			return netStr, ""
		}
	}
	formatted = t.Format("Jan 02, 2006")
	year = t.Format("2006")
	return
}

func bestVideoURL(urls []vidURL) string {
	if len(urls) == 0 {
		return ""
	}
	best := urls[0]
	for _, u := range urls[1:] {
		if u.Priority < best.Priority {
			best = u
		}
	}
	return best.URL
}

// extractString tries to read a JSON field as either a string or an object with a specific key.
func extractStatusAbbrev(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try as object first
	var obj statusObj
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Abbrev != "" {
		return obj.Abbrev
	}
	// Try as string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func extractLSPName(raw json.RawMessage, flatName string) string {
	if flatName != "" {
		return flatName
	}
	if len(raw) == 0 {
		return "Unknown"
	}
	var obj lspObj
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Name != "" {
		return obj.Name
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return "Unknown"
}

func extractMissionDescription(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Could be null
	if string(raw) == "null" {
		return ""
	}
	// Try as object
	var obj missionObj
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.Description
	}
	// Try as string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func launchToStream(l launchDetailed) Stream {
	dateFormatted, year := formatDate(l.Net)
	statusAbbrev := extractStatusAbbrev(l.Status)
	lspName := extractLSPName(l.LSP, l.LSPName)
	missionDesc := extractMissionDescription(l.Mission)
	videoURL := bestVideoURL(l.VidURLs)

	name := l.Name
	if dateFormatted != l.Net && dateFormatted != "" {
		name = fmt.Sprintf("%s (%s)", l.Name, dateFormatted)
	}

	var tags []string
	if statusAbbrev != "" {
		tags = []string{strings.ToLower(statusAbbrev)}
	}

	return Stream{
		ID:          l.ID,
		Name:        name,
		URL:         videoURL,
		Group:       lspName,
		Logo:        l.Image,
		VodType:     "movie",
		Year:        year,
		Tags:        tags,
		EpisodeName: missionDesc,
	}
}

// fetchPages fetches paginated results from the given URL, up to maxPages.
func fetchPages(startURL string, maxPages int) []launchDetailed {
	var all []launchDetailed
	url := startURL

	for page := 0; page < maxPages; page++ {
		if url == "" {
			break
		}

		logInfo(fmt.Sprintf("fetching page %d: %s", page+1, url))

		body, err := httpGet(url)
		if err != nil {
			logError(fmt.Sprintf("http error on page %d: %s", page+1, err.Error()))
			break
		}

		var resp apiResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			logError(fmt.Sprintf("json parse error on page %d: %s", page+1, err.Error()))
			break
		}

		var launches []launchDetailed
		if err := json.Unmarshal(resp.Results, &launches); err != nil {
			logError(fmt.Sprintf("results parse error on page %d: %s", page+1, err.Error()))
			break
		}

		all = append(all, launches...)

		if resp.Next == nil || *resp.Next == "" {
			break
		}
		url = *resp.Next
	}

	return all
}

// ---------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------

//export refresh
func refresh(configPtr uint32, configLen uint32) uint64 {
	// We don't need config fields for this plugin, but read it anyway.
	_ = readInput(configPtr, configLen)

	var streams []Stream

	// Fetch past launches (detailed mode, up to 10 pages)
	pastLaunches := fetchPages(
		"https://ll.thespacedevs.com/2.2.0/launch/previous/?mode=detailed&limit=50",
		10,
	)
	for _, l := range pastLaunches {
		streams = append(streams, launchToStream(l))
	}
	logInfo(fmt.Sprintf("fetched %d past launches", len(pastLaunches)))

	// Fetch upcoming launches (list mode, up to 5 pages)
	upcomingLaunches := fetchPages(
		"https://ll.thespacedevs.com/2.2.0/launch/upcoming/?mode=list&limit=50",
		5,
	)
	for _, l := range upcomingLaunches {
		streams = append(streams, launchToStream(l))
	}
	logInfo(fmt.Sprintf("fetched %d upcoming launches", len(upcomingLaunches)))

	logInfo(fmt.Sprintf("total streams: %d", len(streams)))

	resp := RefreshResponse{Streams: streams}
	return returnJSON(resp)
}

// ---------------------------------------------------------------------------
// Interact
// ---------------------------------------------------------------------------

//export interact
func interact(actionPtr uint32, actionLen uint32) uint64 {
	_ = readInput(actionPtr, actionLen)
	data := []byte("{}")
	return packResult(data)
}

// ---------------------------------------------------------------------------
// Main (required by TinyGo)
// ---------------------------------------------------------------------------

func main() {}
