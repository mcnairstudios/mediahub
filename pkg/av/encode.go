package av

// Encoder encodes raw frames into compressed packets.
type Encoder interface {
	Encode(frame *Frame) ([]*Packet, error)
	Flush() ([]*Packet, error)
	Extradata() []byte
	FrameSize() int // audio encoder frame size
	Close()
}

// EncoderConfig configures a video or audio encoder.
type EncoderConfig struct {
	Codec       string // "h264", "h265", "av1", "aac", etc.
	HWAccel     string // "vaapi", "qsv", "nvenc", "videotoolbox", ""
	EncoderName string // explicit encoder override (e.g. "av1_vaapi")
	Bitrate     int    // kbps, 0 = default
	Width       int
	Height      int
	Framerate   int
	MaxBitDepth int
	SampleRate  int // audio
	Channels    int // audio
}
