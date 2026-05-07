package output

import (
	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/media"
)

// PluginConfig holds configuration for creating an OutputPlugin.
type PluginConfig struct {
	OutputDir          string
	OutputFilePath     string
	OutputFormat       string
	IsLive             bool
	SegmentDurationSec int
	Video              *media.VideoInfo
	Audio              *media.AudioTrack
	VideoCodecParams   any    // *astiav.CodecParameters from demuxer (copy mode)
	AudioCodecParams   any    // *astiav.CodecParameters from demuxer (copy mode)
	VideoExtradata     []byte         // from encoder when transcoding
	AudioExtradata     []byte         // from encoder when transcoding
	Options            map[string]any // plugin-specific config; avoids interface changes

	// CopyVideoParams copies the video encoder's full codec parameters
	// (including correctly formatted extradata) to a muxer output stream.
	// Wraps avcodec_parameters_from_context. Set when transcoding.
	CopyVideoParams func(cp *astiav.CodecParameters) error
	// CopyAudioParams does the same for audio.
	CopyAudioParams func(cp *astiav.CodecParameters) error
}
