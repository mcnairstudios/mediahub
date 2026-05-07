package api

import (
	"net/http"

	"github.com/mcnairstudios/mediahub/pkg/httputil"
	"github.com/mcnairstudios/mediahub/pkg/hwcaps"
)

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	caps := hwcaps.Probe()
	if caps.Platforms == nil {
		caps.Platforms = []string{}
	}
	if caps.VideoEncoders == nil {
		caps.VideoEncoders = []hwcaps.CodecEntry{}
	}
	if caps.VideoDecoders == nil {
		caps.VideoDecoders = []hwcaps.CodecEntry{}
	}
	if caps.AudioEncoders == nil {
		caps.AudioEncoders = []hwcaps.CodecEntry{}
	}

	resp := map[string]any{
		"platforms":      caps.Platforms,
		"video_encoders": caps.VideoEncoders,
		"video_decoders": caps.VideoDecoders,
		"audio_encoders": caps.AudioEncoders,
		"max_bit_depth":  caps.MaxBitDepth,
		"best_codec":     hwcaps.BestHardwareCodec(),
	}
	httputil.RespondJSON(w, http.StatusOK, resp)
}
