package hwcaps

import (
	"strings"
	"sync"

	"github.com/asticode/go-astiav"
)

type CodecEntry struct {
	Name    string `json:"name"`
	Codec   string `json:"codec"`
	HWAccel string `json:"hwaccel"`
	HW      bool   `json:"hw"`
}

type Capabilities struct {
	Platforms     []string     `json:"platforms"`
	VideoEncoders []CodecEntry `json:"video_encoders"`
	VideoDecoders []CodecEntry `json:"video_decoders"`
	AudioEncoders []CodecEntry `json:"audio_encoders"`
	MaxBitDepth   int          `json:"max_bit_depth"`
}

var probeVideoEncoders = []struct {
	name    string
	codec   string
	hwaccel string
}{
	{"libx264", "h264", ""},
	{"libx265", "h265", ""},
	{"libsvtav1", "av1", ""},
	{"libvpx", "vp8", ""},
	{"libvpx-vp9", "vp9", ""},
	{"h264_videotoolbox", "h264", "videotoolbox"},
	{"hevc_videotoolbox", "h265", "videotoolbox"},
	{"h264_vaapi", "h264", "vaapi"},
	{"hevc_vaapi", "h265", "vaapi"},
	{"av1_vaapi", "av1", "vaapi"},
	{"h264_qsv", "h264", "qsv"},
	{"hevc_qsv", "h265", "qsv"},
	{"av1_qsv", "av1", "qsv"},
	{"h264_nvenc", "h264", "nvenc"},
	{"hevc_nvenc", "h265", "nvenc"},
	{"av1_nvenc", "av1", "nvenc"},
}

var probeVideoDecoders = []struct {
	name    string
	codec   string
	hwaccel string
}{
	{"h264", "h264", ""},
	{"hevc", "h265", ""},
	{"av1", "av1", ""},
	{"libdav1d", "av1", ""},
	{"mpeg2video", "mpeg2", ""},
	{"vp8", "vp8", ""},
	{"vp9", "vp9", ""},
}

var probeAudioEncoders = []struct {
	name  string
	codec string
}{
	{"aac", "aac"},
	{"libmp3lame", "mp3"},
	{"libopus", "opus"},
	{"libvorbis", "vorbis"},
	{"flac", "flac"},
	{"ac3", "ac3"},
	{"eac3", "eac3"},
	{"mp2", "mp2"},
}

var hwPlatformMap = map[string]astiav.HardwareDeviceType{
	"vaapi":        astiav.HardwareDeviceTypeVAAPI,
	"qsv":          astiav.HardwareDeviceTypeQSV,
	"cuda":         astiav.HardwareDeviceTypeCUDA,
	"nvenc":        astiav.HardwareDeviceTypeCUDA,
	"videotoolbox": astiav.HardwareDeviceTypeVideoToolbox,
	"vulkan":       astiav.HardwareDeviceTypeVulkan,
}

var (
	cached     *Capabilities
	cachedOnce sync.Once
)

// Probe detects available hardware platforms, encoders, and decoders.
// Results are cached after the first call.
func Probe() *Capabilities {
	cachedOnce.Do(func() {
		availablePlatforms := make(map[string]bool)
		for name, hwType := range hwPlatformMap {
			if name == "nvenc" {
				continue
			}
			ctx, err := astiav.CreateHardwareDeviceContext(hwType, "", nil, 0)
			if err == nil {
				ctx.Free()
				availablePlatforms[name] = true
				if name == "cuda" {
					availablePlatforms["nvenc"] = true
				}
			}
		}

		platformSet := map[string]bool{}

		var videoEncoders []CodecEntry
		for _, e := range probeVideoEncoders {
			if astiav.FindEncoderByName(e.name) == nil {
				continue
			}
			isHW := e.hwaccel != ""
			if isHW && !availablePlatforms[e.hwaccel] {
				continue
			}
			if isHW {
				platformSet[e.hwaccel] = true
			}
			videoEncoders = append(videoEncoders, CodecEntry{
				Name:    e.name,
				Codec:   e.codec,
				HWAccel: e.hwaccel,
				HW:      isHW,
			})
		}

		var videoDecoders []CodecEntry
		seen := map[string]bool{}
		for _, d := range probeVideoDecoders {
			c := astiav.FindDecoderByName(d.name)
			if c == nil {
				continue
			}
			videoDecoders = append(videoDecoders, CodecEntry{
				Name:  d.name,
				Codec: d.codec,
			})
			for _, hwc := range c.HardwareConfigs() {
				hwName := strings.ToLower(hwc.HardwareDeviceType().String())
				if hwName == "" || hwName == "none" {
					continue
				}
				if !availablePlatforms[hwName] {
					continue
				}
				key := d.codec + ":" + hwName
				if seen[key] {
					continue
				}
				seen[key] = true
				platformSet[hwName] = true
				videoDecoders = append(videoDecoders, CodecEntry{
					Name:    d.name,
					Codec:   d.codec,
					HWAccel: hwName,
					HW:      true,
				})
			}
		}

		var audioEncoders []CodecEntry
		for _, ae := range probeAudioEncoders {
			if astiav.FindEncoderByName(ae.name) != nil {
				audioEncoders = append(audioEncoders, CodecEntry{
					Name:  ae.name,
					Codec: ae.codec,
				})
			}
		}

		var platforms []string
		for p := range platformSet {
			platforms = append(platforms, p)
		}

		maxBitDepth := probeMaxBitDepth(availablePlatforms)

		cached = &Capabilities{
			Platforms:     platforms,
			VideoEncoders: videoEncoders,
			VideoDecoders: videoDecoders,
			AudioEncoders: audioEncoders,
			MaxBitDepth:   maxBitDepth,
		}
	})
	return cached
}

func probeMaxBitDepth(platforms map[string]bool) int {
	hwDeviceType := map[string]string{
		"vaapi":        "vaapi",
		"qsv":          "qsv",
		"videotoolbox": "videotoolbox",
		"nvenc":        "cuda",
	}
	for name := range platforms {
		devTypeName, ok := hwDeviceType[name]
		if !ok {
			continue
		}
		hwType := astiav.FindHardwareDeviceTypeByName(devTypeName)
		ctx, err := astiav.CreateHardwareDeviceContext(hwType, "", nil, 0)
		if err != nil {
			continue
		}
		constraints := ctx.HardwareFramesConstraints()
		ctx.Free()
		if constraints == nil {
			continue
		}
		has10bit := false
		for _, pf := range constraints.ValidSoftwarePixelFormats() {
			if strings.Contains(pf.Name(), "10") {
				has10bit = true
				break
			}
		}
		constraints.Free()
		if !has10bit {
			return 8
		}
	}
	return 0
}

// BestHardwareCodec returns the strongest codec with hardware encoding support.
// Preference order: av1 > h265 > h264. Returns "" if no HW encoders found.
func BestHardwareCodec() string {
	caps := Probe()
	rank := map[string]int{"av1": 3, "h265": 2, "h264": 1}
	best := ""
	bestRank := 0
	for _, enc := range caps.VideoEncoders {
		if !enc.HW {
			continue
		}
		if r, ok := rank[enc.Codec]; ok && r > bestRank {
			best = enc.Codec
			bestRank = r
		}
	}
	return best
}

// ResolveCodec resolves "auto" to the best hardware codec, or returns
// the explicit codec unchanged. Falls back to h264 if no HW encoder found.
func ResolveCodec(codec string) string {
	if codec == "" || codec == "auto" || codec == "default" {
		best := BestHardwareCodec()
		if best == "" {
			return "h264"
		}
		return best
	}
	return codec
}

// BestCodecForBrowser returns the strongest codec that has both hardware
// encoding support and is offered by the browser in the SDP codec list.
// browserCodecs should be a set of codec names (e.g. "h264", "h265", "av1").
// VP8/VP9 are ignored (no hardware encode support on relevant platforms).
func BestCodecForBrowser(browserCodecs map[string]bool) string {
	caps := Probe()
	hwAvailable := map[string]bool{}
	for _, enc := range caps.VideoEncoders {
		if enc.HW {
			hwAvailable[enc.Codec] = true
		}
	}
	// Preference: AV1 > H.265 > H.264 — intersect HW capability with browser support
	ranked := []string{"av1", "h265", "h264"}
	for _, codec := range ranked {
		if hwAvailable[codec] && browserCodecs[codec] {
			return codec
		}
	}
	// Fallback: any codec the browser supports (SW encode)
	for _, codec := range ranked {
		if browserCodecs[codec] {
			return codec
		}
	}
	return "h264"
}

// ParseSDPVideoCodecs extracts the set of video codec names offered in
// an SDP body. Recognises H264, H265, VP8, VP9, and AV1 from rtpmap lines.
// Handles both \n and \r\n line endings.
func ParseSDPVideoCodecs(sdp string) map[string]bool {
	codecs := map[string]bool{}
	// Normalize line endings
	sdp = strings.ReplaceAll(sdp, "\r\n", "\n")
	lines := strings.Split(sdp, "\n")
	inVideo := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Track whether we're in a video media section
		if strings.HasPrefix(line, "m=video") {
			inVideo = true
			continue
		} else if strings.HasPrefix(line, "m=") {
			inVideo = false
			continue
		}
		if !inVideo {
			continue
		}
		if !strings.HasPrefix(line, "a=rtpmap:") {
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.Contains(upper, "H264"):
			codecs["h264"] = true
		case strings.Contains(upper, "H265") || strings.Contains(upper, "HEVC"):
			codecs["h265"] = true
		case strings.Contains(upper, "AV1"):
			codecs["av1"] = true
		case strings.Contains(upper, "VP9"):
			codecs["vp9"] = true
		case strings.Contains(upper, "VP8"):
			codecs["vp8"] = true
		}
	}
	return codecs
}
