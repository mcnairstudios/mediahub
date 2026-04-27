package av

// Decoder decodes compressed packets into raw frames.
type Decoder interface {
	Decode(pkt *Packet) ([]Frame, error)
	FlushBuffers()
	Close()
}

// Frame represents a decoded video or audio frame.
type Frame struct {
	Type       StreamType
	PTS        int64  // nanoseconds
	Width      int    // video only
	Height     int    // video only
	PixelFmt   string // video only (e.g. "yuv420p")
	SampleRate int    // audio only
	Channels   int    // audio only
	Samples    int    // audio only — number of samples in this frame
	Raw        any    // opaque — implementation stores its native frame handle
}
