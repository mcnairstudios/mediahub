package jellyfin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
)

func (s *Server) playbackInfo(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	streamID := addDashes(itemID)

	var mediaStreams []MediaStream
	var ticks int64

	if s.streams != nil {
		if stream, err := s.streams.Get(r.Context(), streamID); err == nil && stream != nil {
			mediaStreams = buildMediaStreams(stream)
			if stream.Duration > 0 {
				ticks = secondsToTicks(stream.Duration)
			}
		}
	}

	if mediaStreams == nil {
		mediaStreams = []MediaStream{
			{Type: "Video", Codec: "h264", Index: 0, IsDefault: true, Width: 1920, Height: 1080},
			{Type: "Audio", Codec: "aac", Index: 1, IsDefault: true, Channels: 2},
		}
	}

	playSessionID := itemID[:min(16, len(itemID))]
	ms := MediaSource{
		Protocol: "Http", ID: itemID, Type: "Default", Name: "Default",
		Container: "mp4", IsRemote: true,
		SupportsTranscoding:     true,
		SupportsDirectStream:    false,
		SupportsDirectPlay:      false,
		RunTimeTicks:            ticks,
		DefaultAudioStreamIndex: 1,
		TranscodingURL:          fmt.Sprintf("/Videos/%s/master.m3u8?MediaSourceId=%s&PlaySessionId=%s", itemID, itemID, playSessionID),
		TranscodingSubProtocol:  "hls",
		TranscodingContainer:    "ts",
		MediaStreams:            mediaStreams,
	}

	s.respondJSON(w, http.StatusOK, map[string]any{
		"MediaSources":  []MediaSource{ms},
		"PlaySessionId": playSessionID,
	})
}

func (s *Server) hlsMasterPlaylist(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	streamID := addDashes(itemID)

	if s.playback != nil {
		headers := make(map[string]string)
		for key := range r.Header {
			headers[key] = r.Header.Get(key)
		}
		if err := s.playback.StartPlayback(streamID, 8096, headers); err != nil {
			s.log.Error().Err(err).Str("stream_id", streamID).Msg("jellyfin: failed to start playback")
			http.Error(w, "playback failed", http.StatusInternalServerError)
			return
		}
	}

	playlistURL := fmt.Sprintf("/Videos/%s/main.m3u8", itemID)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	fmt.Fprintln(w, "#EXTM3U")
	fmt.Fprintf(w, "#EXT-X-STREAM-INF:BANDWIDTH=10000000\n")
	fmt.Fprintln(w, playlistURL)
}

func (s *Server) hlsMediaPlaylist(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	streamID := addDashes(itemID)

	plugin := s.findServablePlugin(streamID)
	if plugin == nil {
		if s.playback != nil {
			headers := make(map[string]string)
			for key := range r.Header {
				headers[key] = r.Header.Get(key)
			}
			if err := s.playback.StartPlayback(streamID, 8096, headers); err != nil {
				s.log.Error().Err(err).Str("stream_id", streamID).Msg("jellyfin: failed to start playback for media playlist")
				http.Error(w, "session not found", http.StatusNotFound)
				return
			}
			plugin = s.findServablePlugin(streamID)
		}
		if plugin == nil {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := plugin.WaitReady(ctx); err != nil {
		s.log.Warn().Err(err).Str("stream_id", streamID).Msg("jellyfin: HLS not ready")
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}

	r.URL.Path = "/playlist.m3u8"
	plugin.ServeHTTP(w, r)
}

func (s *Server) hlsLivePlaylist(w http.ResponseWriter, r *http.Request) {
	s.hlsMediaPlaylist(w, r)
}

func (s *Server) hlsSegment(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	streamID := addDashes(itemID)
	segment := r.PathValue("segment")

	plugin := s.findServablePlugin(streamID)
	if plugin == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	r.URL.Path = "/" + segment
	plugin.ServeHTTP(w, r)
}

func (s *Server) findServablePlugin(streamID string) output.ServablePlugin {
	if s.sessionMgr == nil {
		return nil
	}
	sess := s.sessionMgr.Get(streamID)
	if sess == nil {
		return nil
	}
	for _, p := range sess.FanOut.Plugins() {
		if sp, ok := p.(output.ServablePlugin); ok {
			return sp
		}
	}
	return nil
}

func (s *Server) videoStream(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	streamID := addDashes(itemID)

	if s.channels != nil {
		if _, err := s.channels.Get(r.Context(), streamID); err == nil {
			s.log.Info().Str("channel_id", streamID).Msg("jellyfin video stream for channel")
			http.Error(w, "channel streaming not yet implemented", http.StatusNotImplemented)
			return
		}
	}

	if s.streams == nil {
		http.Error(w, "stream not found", http.StatusNotFound)
		return
	}

	stream, err := s.streams.Get(r.Context(), streamID)
	if err != nil || stream == nil || stream.URL == "" {
		http.Error(w, "stream not found", http.StatusNotFound)
		return
	}

	s.log.Info().Str("stream", streamID).Str("url", stream.URL).Msg("jellyfin video stream requested")

	if strings.HasPrefix(stream.URL, "http://") || strings.HasPrefix(stream.URL, "https://") {
		http.Redirect(w, r, stream.URL, http.StatusTemporaryRedirect)
		return
	}

	http.Error(w, "streaming not yet supported for this source", http.StatusNotImplemented)
}
