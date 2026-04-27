package api

import (
	"net/http"

	"github.com/mcnairstudios/mediahub/pkg/av/probe"
	"github.com/mcnairstudios/mediahub/pkg/httputil"
)

func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL        string `json:"url"`
		TimeoutSec int    `json:"timeout_sec"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL == "" {
		httputil.RespondError(w, http.StatusBadRequest, "url required")
		return
	}
	if req.TimeoutSec <= 0 {
		req.TimeoutSec = 10
	}

	result, err := probe.Probe(req.URL, req.TimeoutSec)
	if err != nil {
		httputil.RespondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	type videoResponse struct {
		Codec      string  `json:"codec"`
		Width      int     `json:"width"`
		Height     int     `json:"height"`
		BitDepth   int     `json:"bit_depth"`
		Interlaced bool    `json:"interlaced"`
		FPS        float64 `json:"fps"`
		Profile    string  `json:"profile,omitempty"`
		PixFmt     string  `json:"pix_fmt,omitempty"`
	}

	type audioResponse struct {
		Index      int    `json:"index"`
		Codec      string `json:"codec"`
		Language   string `json:"language,omitempty"`
		Channels   int    `json:"channels"`
		SampleRate int    `json:"sample_rate"`
		BitRate    int    `json:"bit_rate,omitempty"`
	}

	type subtitleResponse struct {
		Index    int    `json:"index"`
		Codec    string `json:"codec"`
		Language string `json:"language,omitempty"`
	}

	type probeResponse struct {
		Video      *videoResponse     `json:"video,omitempty"`
		Audio      []audioResponse    `json:"audio"`
		Subtitles  []subtitleResponse `json:"subtitles"`
		DurationMs int64              `json:"duration_ms"`
	}

	resp := probeResponse{
		Audio:      make([]audioResponse, 0),
		Subtitles:  make([]subtitleResponse, 0),
		DurationMs: result.DurationMs,
	}

	if result.Video != nil {
		resp.Video = &videoResponse{
			Codec:      result.Video.Codec,
			Width:      result.Video.Width,
			Height:     result.Video.Height,
			BitDepth:   result.Video.BitDepth,
			Interlaced: result.Video.Interlaced,
			FPS:        result.Video.FPS(),
			Profile:    result.Video.Profile,
			PixFmt:     result.Video.PixFmt,
		}
	}

	for _, at := range result.AudioTracks {
		resp.Audio = append(resp.Audio, audioResponse{
			Index:      at.Index,
			Codec:      at.Codec,
			Language:   at.Language,
			Channels:   at.Channels,
			SampleRate: at.SampleRate,
			BitRate:    at.BitRate,
		})
	}

	for _, st := range result.SubTracks {
		resp.Subtitles = append(resp.Subtitles, subtitleResponse{
			Index:    st.Index,
			Codec:    st.Codec,
			Language: st.Language,
		})
	}

	httputil.RespondJSON(w, http.StatusOK, resp)
}
