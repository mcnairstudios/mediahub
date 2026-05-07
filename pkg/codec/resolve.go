// Package codec provides a single codec resolution engine that layers
// preferences from multiple sources (defaults, settings, client profile,
// client override) and applies hard delivery constraints (e.g. WebRTC
// requires Opus audio).
package codec

import (
	"github.com/mcnairstudios/mediahub/pkg/hwcaps"
)

// Preference represents codec preferences from one source.
// Zero values mean "no opinion" and are skipped during layering.
type Preference struct {
	VideoCodec   string
	AudioCodec   string
	Container    string
	HWAccel      string
	OutputHeight int
	MaxBitDepth  int
	Bitrate      int
}

// Constraints represents hard requirements imposed by a delivery mode.
type Constraints struct {
	RequiredAudioCodec string          // e.g. "opus" for WebRTC
	AllowedVideoCodecs map[string]bool // nil = no constraint; non-nil = must be one of these
	ForceTranscode     bool
	DisableDecodeHW    bool
}

// ResolvedCodecs is the final resolved codec configuration after all
// preferences have been layered and constraints applied.
type ResolvedCodecs struct {
	VideoCodec   string
	AudioCodec   string
	Container    string
	HWAccel      string
	OutputHeight int
	MaxBitDepth  int
	Bitrate      int
	// Set by DeliveryConstraints
	ForceTranscode  bool
	DisableDecodeHW bool
}

// Input collects all sources of codec preferences and constraints.
type Input struct {
	Defaults            Preference
	Settings            Preference
	ClientProfile       Preference
	ClientOverride      Preference
	DeliveryConstraints Constraints
}

// Resolve layers preferences in priority order (Defaults < Settings <
// ClientProfile < ClientOverride), resolves "auto" via hardware capability
// detection, and applies delivery constraints as hard overrides.
func Resolve(in Input) ResolvedCodecs {
	r := ResolvedCodecs{}

	// Layer in order: Defaults < Settings < ClientProfile < ClientOverride
	r = applyPreference(r, in.Defaults)
	r = applyPreference(r, in.Settings)
	r = applyPreference(r, in.ClientProfile)
	r = applyPreference(r, in.ClientOverride)

	// Resolve "auto"/"default" video codec to best hardware codec
	if r.VideoCodec == "auto" || r.VideoCodec == "default" {
		r.VideoCodec = hwcaps.ResolveCodec(r.VideoCodec)
	}

	// Resolve "auto"/"default" audio codec
	if r.AudioCodec == "auto" || r.AudioCodec == "default" {
		r.AudioCodec = "aac"
	}

	// Apply delivery constraints last — hard overrides
	c := in.DeliveryConstraints

	if c.RequiredAudioCodec != "" {
		r.AudioCodec = c.RequiredAudioCodec
	}

	if c.AllowedVideoCodecs != nil {
		if !c.AllowedVideoCodecs[r.VideoCodec] {
			// Current video codec not allowed — pick the best from the allowed set
			r.VideoCodec = hwcaps.BestCodecForBrowser(c.AllowedVideoCodecs)
		}
	}

	if c.ForceTranscode {
		r.ForceTranscode = true
	}

	if c.DisableDecodeHW {
		r.DisableDecodeHW = true
	}

	return r
}

// DeliveryConstraints returns the hard codec constraints for a given
// delivery mode string. WebRTC requires Opus audio and forces transcoding
// with software decode. Other modes have no constraints.
func DeliveryConstraints(mode string) Constraints {
	switch mode {
	case "webrtc":
		return Constraints{
			RequiredAudioCodec: "opus",
			ForceTranscode:     true,
			DisableDecodeHW:    true,
		}
	default:
		return Constraints{}
	}
}

// applyPreference overlays non-zero fields from p onto base.
func applyPreference(base ResolvedCodecs, p Preference) ResolvedCodecs {
	if p.VideoCodec != "" {
		base.VideoCodec = p.VideoCodec
	}
	if p.AudioCodec != "" {
		base.AudioCodec = p.AudioCodec
	}
	if p.Container != "" {
		base.Container = p.Container
	}
	if p.HWAccel != "" {
		base.HWAccel = p.HWAccel
	}
	if p.OutputHeight > 0 {
		base.OutputHeight = p.OutputHeight
	}
	if p.MaxBitDepth > 0 {
		base.MaxBitDepth = p.MaxBitDepth
	}
	if p.Bitrate > 0 {
		base.Bitrate = p.Bitrate
	}
	return base
}
