package jellyfin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mcnairstudios/mediahub/pkg/media"
)

func TestPlaybackInfoMediaSourceStructure(t *testing.T) {
	srv := newTestServerFull()
	itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
	req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.playbackInfo(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	sources, ok := result["MediaSources"].([]any)
	require.True(t, ok, "MediaSources must be an array")
	require.Len(t, sources, 1)

	ms := sources[0].(map[string]any)

	assert.Equal(t, "Http", ms["Protocol"])
	assert.Equal(t, itemID, ms["Id"])
	assert.Equal(t, "Default", ms["Type"])
	assert.Equal(t, "Default", ms["Name"])
	assert.Equal(t, "mp4", ms["Container"])
	assert.Equal(t, true, ms["IsRemote"])
	assert.Equal(t, true, ms["SupportsTranscoding"])
	assert.Equal(t, false, ms["SupportsDirectStream"])
	assert.Equal(t, false, ms["SupportsDirectPlay"])
}

func TestPlaybackInfoTranscodingURL(t *testing.T) {
	srv := newTestServerFull()
	itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
	req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.playbackInfo(w, req)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	ms := result["MediaSources"].([]any)[0].(map[string]any)

	transcodingURL, ok := ms["TranscodingUrl"].(string)
	require.True(t, ok, "TranscodingUrl must be a string")
	assert.Contains(t, transcodingURL, "/Videos/"+itemID+"/master.m3u8")
	assert.Contains(t, transcodingURL, "MediaSourceId="+itemID)
	assert.Contains(t, transcodingURL, "PlaySessionId=")
}

func TestPlaybackInfoHLSFields(t *testing.T) {
	srv := newTestServerFull()
	itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
	req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.playbackInfo(w, req)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	ms := result["MediaSources"].([]any)[0].(map[string]any)

	assert.Equal(t, "hls", ms["TranscodingSubProtocol"])
	assert.Equal(t, "ts", ms["TranscodingContainer"])
}

func TestPlaybackInfoPlaySessionID(t *testing.T) {
	srv := newTestServerFull()
	itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
	req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.playbackInfo(w, req)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	psID, ok := result["PlaySessionId"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, psID)
	assert.LessOrEqual(t, len(psID), 16)
}

func TestPlaybackInfoMediaStreams(t *testing.T) {
	srv := newTestServerFull()
	itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
	req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.playbackInfo(w, req)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	ms := result["MediaSources"].([]any)[0].(map[string]any)
	streams, ok := ms["MediaStreams"].([]any)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(streams), 2)

	video := streams[0].(map[string]any)
	assert.Equal(t, "Video", video["Type"])
	assert.Equal(t, "h264", video["Codec"])
	assert.Equal(t, true, video["IsDefault"])

	audio := streams[1].(map[string]any)
	assert.Equal(t, "Audio", audio["Type"])
	assert.Equal(t, "aac", audio["Codec"])
	assert.Equal(t, true, audio["IsDefault"])
}

func TestPlaybackInfoRunTimeTicks(t *testing.T) {
	srv := newTestServerFull()
	itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
	req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.playbackInfo(w, req)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	ms := result["MediaSources"].([]any)[0].(map[string]any)
	ticks, ok := ms["RunTimeTicks"].(float64)
	require.True(t, ok)
	assert.Equal(t, int64(72000000000), int64(ticks))
}

func TestPlaybackInfoFallbackMediaStreams(t *testing.T) {
	srv := newTestServer()
	itemID := "abcd1234abcd1234abcd1234abcd1234"
	req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.playbackInfo(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	ms := result["MediaSources"].([]any)[0].(map[string]any)
	streams := ms["MediaStreams"].([]any)
	require.Len(t, streams, 2)

	video := streams[0].(map[string]any)
	assert.Equal(t, "h264", video["Codec"])
	assert.Equal(t, float64(1920), video["Width"])
	assert.Equal(t, float64(1080), video["Height"])

	audio := streams[1].(map[string]any)
	assert.Equal(t, "aac", audio["Codec"])
	assert.Equal(t, float64(2), audio["Channels"])
}

func TestHLSMasterPlaylistFormat(t *testing.T) {
	srv := newTestServer()
	itemID := "abcd1234abcd1234abcd1234abcd1234"
	req := httptest.NewRequest(http.MethodGet, "/Videos/"+itemID+"/master.m3u8", nil)
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.hlsMasterPlaylist(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/vnd.apple.mpegurl", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache, no-store", w.Header().Get("Cache-Control"))

	body := w.Body.String()
	assert.True(t, strings.HasPrefix(body, "#EXTM3U\n"))
	assert.Contains(t, body, "#EXT-X-STREAM-INF:BANDWIDTH=10000000")
	assert.Contains(t, body, fmt.Sprintf("/Videos/%s/main.m3u8", itemID))
}

func TestHLSMasterPlaylistCORSHeaders(t *testing.T) {
	srv := newTestServer()
	itemID := "abcd1234abcd1234abcd1234abcd1234"
	req := httptest.NewRequest(http.MethodGet, "/Videos/"+itemID+"/master.m3u8", nil)
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.hlsMasterPlaylist(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestHLSMediaPlaylistNoSession(t *testing.T) {
	srv := newTestServer()
	itemID := "abcd1234abcd1234abcd1234abcd1234"
	req := httptest.NewRequest(http.MethodGet, "/Videos/"+itemID+"/main.m3u8", nil)
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.hlsMediaPlaylist(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHLSSegmentNoSession(t *testing.T) {
	srv := newTestServer()
	itemID := "abcd1234abcd1234abcd1234abcd1234"
	req := httptest.NewRequest(http.MethodGet, "/Videos/"+itemID+"/hls/seg0.ts", nil)
	req.SetPathValue("itemId", itemID)
	req.SetPathValue("segment", "seg0.ts")
	w := httptest.NewRecorder()

	srv.hlsSegment(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHLSSegmentDirectNoSession(t *testing.T) {
	srv := newTestServer()
	itemID := "abcd1234abcd1234abcd1234abcd1234"
	req := httptest.NewRequest(http.MethodGet, "/Videos/"+itemID+"/hls/seg0.ts", nil)
	req.SetPathValue("itemId", itemID)
	req.SetPathValue("segment", "seg0.ts")
	w := httptest.NewRecorder()

	srv.hlsSegmentDirect(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPlaylistRewriting(t *testing.T) {
	itemID := "abcd1234abcd1234abcd1234abcd1234"
	original := "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:6\n#EXTINF:6.000,\nseg0.ts\n#EXTINF:6.000,\nseg1.ts\n#EXTINF:4.500,\nseg2.ts\n#EXT-X-ENDLIST\n"

	segBase := fmt.Sprintf("/Videos/%s/hls/", itemID)
	var rewritten strings.Builder
	for _, line := range strings.Split(original, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			rewritten.WriteString("\n")
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			rewritten.WriteString(line)
			rewritten.WriteString("\n")
			continue
		}
		if strings.HasSuffix(trimmed, ".ts") || strings.HasSuffix(trimmed, ".m4s") || strings.HasSuffix(trimmed, ".mp4") {
			rewritten.WriteString(segBase)
			rewritten.WriteString(trimmed)
			rewritten.WriteString("\n")
			continue
		}
		rewritten.WriteString(line)
		rewritten.WriteString("\n")
	}

	result := rewritten.String()
	assert.Contains(t, result, "#EXTM3U")
	assert.Contains(t, result, "#EXT-X-TARGETDURATION:6")
	assert.Contains(t, result, segBase+"seg0.ts")
	assert.Contains(t, result, segBase+"seg1.ts")
	assert.Contains(t, result, segBase+"seg2.ts")
	assert.Contains(t, result, "#EXT-X-ENDLIST")

	assert.NotContains(t, result, "\nseg0.ts\n")
	assert.NotContains(t, result, "\nseg1.ts\n")
	assert.NotContains(t, result, "\nseg2.ts\n")
}

func TestPlaylistRewritingM4SSegments(t *testing.T) {
	itemID := "abcd1234abcd1234abcd1234abcd1234"
	original := "#EXTM3U\n#EXTINF:6.000,\ninit.mp4\n#EXTINF:6.000,\nseg0.m4s\n"

	segBase := fmt.Sprintf("/Videos/%s/hls/", itemID)
	var rewritten strings.Builder
	for _, line := range strings.Split(original, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			rewritten.WriteString("\n")
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			rewritten.WriteString(line)
			rewritten.WriteString("\n")
			continue
		}
		if strings.HasSuffix(trimmed, ".ts") || strings.HasSuffix(trimmed, ".m4s") || strings.HasSuffix(trimmed, ".mp4") {
			rewritten.WriteString(segBase)
			rewritten.WriteString(trimmed)
			rewritten.WriteString("\n")
			continue
		}
		rewritten.WriteString(line)
		rewritten.WriteString("\n")
	}

	result := rewritten.String()
	assert.Contains(t, result, segBase+"init.mp4")
	assert.Contains(t, result, segBase+"seg0.m4s")
}

func TestPlaybackInfoVideoCodecVariants(t *testing.T) {
	tests := []struct {
		name          string
		videoCodec    string
		expectedCodec string
	}{
		{"h264", "h264", "h264"},
		{"hevc", "hevc", "hevc"},
		{"h265", "h265", "hevc"},
		{"avc", "avc", "h264"},
		{"av1", "av1", "av1"},
		{"empty defaults to h264", "", "h264"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer()
			srv.streams = &mockStreamStore{
				streams: []media.Stream{
					{ID: "aaaaaaaa-1111-2222-3333-444444444444", Name: "Test", VideoCodec: tt.videoCodec, AudioCodec: "aac", Width: 1920, Height: 1080, Duration: 3600},
				},
			}

			itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
			req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
			req.SetPathValue("itemId", itemID)
			w := httptest.NewRecorder()

			srv.playbackInfo(w, req)

			var result map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

			ms := result["MediaSources"].([]any)[0].(map[string]any)
			streams := ms["MediaStreams"].([]any)
			video := streams[0].(map[string]any)
			assert.Equal(t, tt.expectedCodec, video["Codec"])
		})
	}
}

func TestPlaybackInfoAudioCodecVariants(t *testing.T) {
	tests := []struct {
		name          string
		audioCodec    string
		expectedCodec string
	}{
		{"aac", "aac", "aac"},
		{"ac3", "ac3", "ac3"},
		{"eac3", "eac3", "ac3"},
		{"dts", "dts", "dca"},
		{"opus", "opus", "opus"},
		{"mp3", "mp3", "mp3"},
		{"empty defaults to aac", "", "aac"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer()
			srv.streams = &mockStreamStore{
				streams: []media.Stream{
					{ID: "aaaaaaaa-1111-2222-3333-444444444444", Name: "Test", VideoCodec: "h264", AudioCodec: tt.audioCodec, Width: 1920, Height: 1080, Duration: 3600},
				},
			}

			itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
			req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
			req.SetPathValue("itemId", itemID)
			w := httptest.NewRecorder()

			srv.playbackInfo(w, req)

			var result map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

			ms := result["MediaSources"].([]any)[0].(map[string]any)
			streams := ms["MediaStreams"].([]any)
			audio := streams[1].(map[string]any)
			assert.Equal(t, tt.expectedCodec, audio["Codec"])
		})
	}
}

func TestPlaybackInfoDefaultAudioStreamIndex(t *testing.T) {
	srv := newTestServerFull()
	itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
	req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.playbackInfo(w, req)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	ms := result["MediaSources"].([]any)[0].(map[string]any)
	assert.Equal(t, float64(1), ms["DefaultAudioStreamIndex"])
}

func TestVideoStreamNoSession(t *testing.T) {
	srv := newTestServer()
	itemID := "abcd1234abcd1234abcd1234abcd1234"
	req := httptest.NewRequest(http.MethodGet, "/Videos/"+itemID+"/stream", nil)
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.videoStream(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestVideoStreamRedirectsHTTPURLs(t *testing.T) {
	srv := newTestServer()
	srv.streams = &mockStreamStore{
		streams: []media.Stream{
			{ID: "aaaaaaaa-1111-2222-3333-444444444444", Name: "Test", URL: "http://example.com/stream.ts"},
		},
	}

	itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
	req := httptest.NewRequest(http.MethodGet, "/Videos/"+itemID+"/stream", nil)
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.videoStream(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Equal(t, "http://example.com/stream.ts", w.Header().Get("Location"))
}

func TestHLSLivePlaylistFallsThrough(t *testing.T) {
	srv := newTestServer()
	itemID := "abcd1234abcd1234abcd1234abcd1234"
	req := httptest.NewRequest(http.MethodGet, "/Videos/"+itemID+"/live.m3u8", nil)
	req.SetPathValue("itemId", itemID)
	w := httptest.NewRecorder()

	srv.hlsLivePlaylist(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
