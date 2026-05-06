package main

import (
	"encoding/json"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Memory management — required WASM exports
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

func logInfo(msg string) {
	b := []byte(msg)
	hostLog(1, uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)))
}

// ---------------------------------------------------------------------------
// Type definitions
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
	Sortable   bool   `json:"sortable,omitempty"`
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

type Stream struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Group       string   `json:"group,omitempty"`
	Logo        string   `json:"logo,omitempty"`
	VodType     string   `json:"vod_type,omitempty"`
	Year        string   `json:"year,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	EpisodeName string   `json:"episode_name,omitempty"`
}

type RefreshResponse struct {
	Streams []Stream `json:"streams"`
}

// ---------------------------------------------------------------------------
// Exported functions
// ---------------------------------------------------------------------------

//export describe
func describe() uint64 {
	resp := DescribeResponse{
		Type:        "demo",
		Label:       "Demo Streams",
		ShortLabel:  "DEMO",
		Color:       "#607d8b",
		Version:     "1.0.0",
		Description: "Free public test video streams for demonstration and testing",
		ConfigFields: []ConfigField{},
		View: ViewConfig{
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
	logInfo("demo: refreshing stream list")

	resp := RefreshResponse{
		Streams: []Stream{
			{
				ID:    "nasa-live",
				Name:  "NASA Live",
				URL:   "https://ntv1.akamaized.net/hls/live/2014075/NASA-NTV1-HLS/master.m3u8",
				Group: "Live",
				Logo:  "https://upload.wikimedia.org/wikipedia/commons/thumb/e/e5/NASA_logo.svg/200px-NASA_logo.svg.png",
				Tags:  []string{"live"},
			},
			{
				ID:    "bloomberg",
				Name:  "Bloomberg TV",
				URL:   "https://liveproduseast.akamaized.net/us/Channel-USTV-AWS-virginia-2/Source-USTV-10000-1-slxdlg-BP-HD-1080.m3u8",
				Group: "Live",
				Tags:  []string{"live"},
			},
			{
				ID:      "big-buck-bunny",
				Name:    "Big Buck Bunny",
				URL:     "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8",
				Group:   "Test Streams",
				VodType: "movie",
				Year:    "2008",
			},
			{
				ID:      "elephants-dream",
				Name:    "Elephant's Dream",
				URL:     "https://test-streams.mux.dev/elephantsdream/ed-vp9-opus.webm",
				Group:   "Test Streams",
				VodType: "movie",
				Year:    "2006",
			},
			{
				ID:      "sintel",
				Name:    "Sintel",
				URL:     "https://test-streams.mux.dev/sintel/sintel.m3u8",
				Group:   "Test Streams",
				VodType: "movie",
				Year:    "2010",
			},
			{
				ID:      "tears-of-steel",
				Name:    "Tears of Steel",
				URL:     "https://test-streams.mux.dev/tears_of_steel/tears_of_steel.m3u8",
				Group:   "Test Streams",
				VodType: "movie",
				Year:    "2012",
			},
		},
	}
	return returnJSON(resp)
}

//export interact
func interact(actionPtr uint32, actionLen uint32) uint64 {
	data := []byte("{}")
	return packResult(data)
}

func main() {}
