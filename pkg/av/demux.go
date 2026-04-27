package av

import "github.com/mcnairstudios/mediahub/pkg/media"

// Demuxer opens a media source and reads packets.
// ReadPacket returns io.EOF at end of stream.
type Demuxer interface {
	StreamInfo() *media.ProbeResult
	ReadPacket() (*Packet, error)
	SeekTo(posMs int64) error
	SetAudioTrack(index int)
	Close() error
}
