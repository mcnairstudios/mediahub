package av

// PacketSink receives packets from the demux loop. Output plugins and
// the DecodeBridge implement this.
type PacketSink interface {
	PushVideo(data []byte, pts, dts int64, keyframe bool) error
	PushAudio(data []byte, pts, dts int64) error
	PushSubtitle(data []byte, pts int64, duration int64) error
	EndOfStream()
}
