package av

import (
	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/media"
)

type Demuxer interface {
	StreamInfo() *media.ProbeResult
	ReadPacket() (*Packet, error)
	SeekTo(posMs int64) error
	RequestSeek(posMs int64) error
	SetOnSeek(fn func())
	SetAudioTrack(idx int) error
	Reconnect() error
	VideoCodecParameters() *astiav.CodecParameters
	AudioCodecParameters() *astiav.CodecParameters
	Close()
}
