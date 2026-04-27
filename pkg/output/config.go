package output

import "github.com/mcnairstudios/mediahub/pkg/media"

// PluginConfig holds configuration for creating an OutputPlugin.
type PluginConfig struct {
	OutputDir      string
	OutputFilePath string
	OutputFormat   string
	IsLive         bool
	Video          *media.VideoInfo
	Audio          *media.AudioTrack
}
