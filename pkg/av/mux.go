package av

// Muxer writes encoded packets to an output container.
type Muxer interface {
	WriteVideoPacket(pkt *Packet) error
	WriteAudioPacket(pkt *Packet) error
	Close() error
}

// SegmentedMuxer extends Muxer for segment-based outputs (HLS, fMP4).
type SegmentedMuxer interface {
	Muxer
	SegmentCount() int
	Reset() error // for seek
}
