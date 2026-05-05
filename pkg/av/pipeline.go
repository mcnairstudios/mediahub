package av

// PacketSink receives packets from the demux loop. Output plugins and
// the DecodeBridge implement this. Duration is in nanoseconds; it may be
// zero for live streams where the muxer infers duration from framerate.
type PacketSink interface {
	PushVideo(data []byte, pts, dts, duration int64, keyframe bool) error
	PushAudio(data []byte, pts, dts, duration int64) error
	PushSubtitle(data []byte, pts int64, duration int64) error
	EndOfStream()
}
