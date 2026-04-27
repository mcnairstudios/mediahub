package mux

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/asticode/go-astiav"
)

const defaultIOBufSize = 32768

type MuxOpts struct {
	OutputDir         string
	SegmentDurationMs int
	AudioFragmentMs   int
	VideoCodecID      astiav.CodecID
	VideoExtradata    []byte
	VideoWidth        int
	VideoHeight       int
	VideoTimeBase     astiav.Rational
	AudioCodecID      astiav.CodecID
	AudioExtradata    []byte
	AudioChannels     int
	AudioSampleRate   int
}

type FragmentedMuxer struct {
	video         *trackMuxer
	audio         *trackMuxer
	videoCodecStr string
	closed        bool
	mu            sync.Mutex
}

type trackMuxer struct {
	fc              *astiav.FormatContext
	ioCtx           *astiav.IOContext
	stream          *astiav.Stream
	buf             bytes.Buffer
	initData        []byte
	initDone        bool
	outputDir       string
	prefix          string
	seq             int
	accumDurationUs int64
	fragThresholdUs int64
	pktCount        int
	lastDTS         int64
	dtsInited       bool

	codecID    astiav.CodecID
	extradata  []byte
	timeBase   astiav.Rational
	width      int
	height     int
	channels   int
	sampleRate int
}

const cmafMovflags = "frag_custom+dash+skip_sidx+skip_trailer"

func NewFragmentedMuxer(opts MuxOpts) (*FragmentedMuxer, error) {
	if opts.OutputDir == "" {
		return nil, errors.New("avmux: OutputDir is required")
	}
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("avmux: create output dir: %w", err)
	}

	audioFragMs := opts.AudioFragmentMs
	if audioFragMs <= 0 {
		audioFragMs = 2048
	}

	m := &FragmentedMuxer{}

	if opts.VideoCodecID != astiav.CodecIDNone {
		var videoThresholdUs int64
		if opts.SegmentDurationMs > 0 {
			videoThresholdUs = int64(opts.SegmentDurationMs) * 1000
		}
		tm, err := newTrackMuxer(opts.OutputDir, "video", opts.VideoCodecID,
			opts.VideoExtradata, opts.VideoTimeBase, opts.VideoWidth, opts.VideoHeight,
			0, 0, videoThresholdUs)
		if err != nil {
			return nil, fmt.Errorf("avmux: video track: %w", err)
		}
		m.video = tm
		m.videoCodecStr = extractCodecString(tm.initData)
	}

	if opts.AudioCodecID != astiav.CodecIDNone {
		tb := astiav.NewRational(1, opts.AudioSampleRate)
		if opts.AudioSampleRate == 0 {
			tb = astiav.NewRational(1, 48000)
		}
		tm, err := newTrackMuxer(opts.OutputDir, "audio", opts.AudioCodecID,
			opts.AudioExtradata, tb, 0, 0,
			opts.AudioChannels, opts.AudioSampleRate, int64(audioFragMs)*1000)
		if err != nil {
			if m.video != nil {
				m.video.close()
			}
			return nil, fmt.Errorf("avmux: audio track: %w", err)
		}
		m.audio = tm
	}

	return m, nil
}

func newTrackMuxer(outputDir, prefix string, codecID astiav.CodecID,
	extradata []byte, timeBase astiav.Rational, width, height int,
	channels, sampleRate int, fragThresholdUs int64) (*trackMuxer, error) {

	tm := &trackMuxer{
		outputDir:       outputDir,
		prefix:          prefix,
		seq:             1,
		fragThresholdUs: fragThresholdUs,
		codecID:         codecID,
		extradata:       extradata,
		timeBase:        timeBase,
		width:           width,
		height:          height,
		channels:        channels,
		sampleRate:      sampleRate,
	}

	fc, err := astiav.AllocOutputFormatContext(nil, "mp4", "")
	if err != nil {
		return nil, fmt.Errorf("alloc output format: %w", err)
	}
	tm.fc = fc

	ioCtx, err := astiav.AllocIOContext(defaultIOBufSize, true, nil, nil,
		func(b []byte) (int, error) {
			return tm.buf.Write(b)
		},
	)
	if err != nil {
		fc.Free()
		return nil, fmt.Errorf("alloc io context: %w", err)
	}
	tm.ioCtx = ioCtx
	fc.SetPb(ioCtx)

	s := fc.NewStream(nil)
	if s == nil {
		tm.close()
		return nil, errors.New("failed to allocate stream")
	}
	tm.stream = s

	cp := s.CodecParameters()
	cp.SetCodecID(codecID)
	if width > 0 && height > 0 {
		cp.SetMediaType(astiav.MediaTypeVideo)
		cp.SetWidth(width)
		cp.SetHeight(height)
	} else {
		cp.SetMediaType(astiav.MediaTypeAudio)
		if sampleRate > 0 {
			cp.SetSampleRate(sampleRate)
		}
		switch channels {
		case 1:
			cp.SetChannelLayout(astiav.ChannelLayoutMono)
		case 2:
			cp.SetChannelLayout(astiav.ChannelLayoutStereo)
		case 6:
			cp.SetChannelLayout(astiav.ChannelLayout5Point1)
		case 8:
			cp.SetChannelLayout(astiav.ChannelLayout7Point1)
		}
		if codecID == astiav.CodecIDAac {
			cp.SetFrameSize(1024)
		}
	}
	s.SetTimeBase(timeBase)

	if len(extradata) > 0 {
		if err := cp.SetExtraData(extradata); err != nil {
			tm.close()
			return nil, fmt.Errorf("set extradata: %w", err)
		}
	}

	opts := astiav.NewDictionary()
	defer opts.Free()
	opts.Set("movflags", cmafMovflags, 0)

	if err := fc.WriteHeader(opts); err != nil {
		tm.close()
		return nil, fmt.Errorf("write header: %w", err)
	}

	ioCtx.Flush()

	tm.initData = make([]byte, tm.buf.Len())
	copy(tm.initData, tm.buf.Bytes())
	tm.buf.Reset()

	initPath := filepath.Join(outputDir, fmt.Sprintf("init_%s.mp4", prefix))
	if err := atomicWrite(initPath, tm.initData); err != nil {
		tm.close()
		return nil, fmt.Errorf("write init segment: %w", err)
	}
	tm.initDone = true

	return tm, nil
}

func (m *FragmentedMuxer) WriteVideoPacket(pkt *astiav.Packet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("avmux: muxer is closed")
	}
	if m.video == nil {
		return errors.New("avmux: no video track configured")
	}

	isKeyframe := pkt.Flags().Has(astiav.PacketFlagKey)

	shouldFlush := m.video.pktCount > 0 && (isKeyframe ||
		(m.video.fragThresholdUs > 0 && m.video.accumDurationUs >= m.video.fragThresholdUs))
	if shouldFlush {
		if err := m.video.flushFragment(); err != nil {
			return err
		}
	}

	pkt.SetStreamIndex(m.video.stream.Index())
	m.video.ensureMonotonicDTS(pkt)
	dur := pktDurationUs(pkt, m.video.stream)
	if err := m.video.fc.WriteFrame(pkt); err != nil {
		return fmt.Errorf("avmux: write video frame: %w", err)
	}
	m.video.pktCount++
	m.video.accumDurationUs += dur
	return nil
}

func (m *FragmentedMuxer) WriteAudioPacket(pkt *astiav.Packet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("avmux: muxer is closed")
	}
	if m.audio == nil {
		return errors.New("avmux: no audio track configured")
	}

	pkt.SetStreamIndex(m.audio.stream.Index())
	m.audio.ensureMonotonicDTS(pkt)
	dur := pktDurationUs(pkt, m.audio.stream)
	if err := m.audio.fc.WriteFrame(pkt); err != nil {
		return fmt.Errorf("avmux: write audio frame: %w", err)
	}
	m.audio.pktCount++
	m.audio.accumDurationUs += dur

	if m.audio.fragThresholdUs > 0 && m.audio.accumDurationUs >= m.audio.fragThresholdUs {
		if err := m.audio.flushFragment(); err != nil {
			return err
		}
	}
	return nil
}

func (m *FragmentedMuxer) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	var firstErr error
	if m.video != nil {
		seq := m.video.seq
		if m.video.pktCount > 0 {
			m.video.flushFragment() //nolint:errcheck
			seq = m.video.seq
		}
		m.video.close()
		tm, err := newTrackMuxer(m.video.outputDir, m.video.prefix, m.video.codecID,
			m.video.extradata, m.video.timeBase, m.video.width, m.video.height,
			0, 0, m.video.fragThresholdUs)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			tm.seq = seq
			m.video = tm
		}
	}
	if m.audio != nil {
		seq := m.audio.seq
		if m.audio.pktCount > 0 {
			m.audio.flushFragment() //nolint:errcheck
			seq = m.audio.seq
		}
		m.audio.close()
		tm, err := newTrackMuxer(m.audio.outputDir, m.audio.prefix, m.audio.codecID,
			m.audio.extradata, m.audio.timeBase, 0, 0,
			m.audio.channels, m.audio.sampleRate, m.audio.fragThresholdUs)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			tm.seq = seq
			m.audio = tm
		}
	}
	return firstErr
}

func (m *FragmentedMuxer) VideoCodecString() string {
	return m.videoCodecStr
}

func (m *FragmentedMuxer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true

	var firstErr error
	if m.video != nil {
		if m.video.pktCount > 0 {
			if err := m.video.flushFragment(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		m.video.close()
	}
	if m.audio != nil {
		if m.audio.pktCount > 0 {
			if err := m.audio.flushFragment(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		m.audio.close()
	}
	return firstErr
}

func (t *trackMuxer) flushFragment() error {
	if err := t.fc.WriteFrame(nil); err != nil {
		return fmt.Errorf("avmux: flush %s fragment: %w", t.prefix, err)
	}
	t.ioCtx.Flush()

	if t.buf.Len() == 0 {
		return nil
	}

	segPath := filepath.Join(t.outputDir, fmt.Sprintf("%s_%04d.m4s", t.prefix, t.seq))
	if err := atomicWrite(segPath, t.buf.Bytes()); err != nil {
		return fmt.Errorf("avmux: write %s segment %d: %w", t.prefix, t.seq, err)
	}
	t.buf.Reset()
	t.seq++
	t.pktCount = 0
	t.accumDurationUs = 0
	return nil
}

func (t *trackMuxer) close() {
	if t.fc != nil {
		t.fc.WriteTrailer() //nolint:errcheck
		t.fc.Free()
		t.fc = nil
	}
	if t.ioCtx != nil {
		t.ioCtx.Free()
		t.ioCtx = nil
	}
}

func (t *trackMuxer) ensureMonotonicDTS(pkt *astiav.Packet) {
	dts := pkt.Dts()
	pts := pkt.Pts()
	if t.dtsInited && dts <= t.lastDTS {
		dts = t.lastDTS + 1
		if pts < dts {
			pts = dts
		}
		pkt.SetDts(dts)
		pkt.SetPts(pts)
	}
	t.lastDTS = dts
	t.dtsInited = true
}

func pktDurationUs(pkt *astiav.Packet, s *astiav.Stream) int64 {
	tb := s.TimeBase()
	if tb.Den() == 0 {
		return 0
	}
	return pkt.Duration() * int64(tb.Num()) * 1_000_000 / int64(tb.Den())
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

type StreamMuxer struct {
	fc            *astiav.FormatContext
	ioCtx         *astiav.IOContext
	w             io.Writer
	headerWritten bool
	closed        bool
	mu            sync.Mutex
}

func NewStreamMuxer(format string, w io.Writer) (*StreamMuxer, error) {
	if format == "" {
		return nil, errors.New("avmux: format is required")
	}
	if w == nil {
		return nil, errors.New("avmux: writer is required")
	}

	fc, err := astiav.AllocOutputFormatContext(nil, format, "")
	if err != nil {
		return nil, err
	}

	ioCtx, err := astiav.AllocIOContext(
		defaultIOBufSize,
		true,
		nil,
		nil,
		func(b []byte) (int, error) {
			return w.Write(b)
		},
	)
	if err != nil {
		fc.Free()
		return nil, err
	}

	fc.SetPb(ioCtx)

	return &StreamMuxer{
		fc:    fc,
		ioCtx: ioCtx,
		w:     w,
	}, nil
}

func (m *StreamMuxer) AddStream(codecParams *astiav.CodecParameters) (*astiav.Stream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("avmux: muxer is closed")
	}
	if codecParams == nil {
		return nil, errors.New("avmux: codecParams is required")
	}

	s := m.fc.NewStream(nil)
	if s == nil {
		return nil, errors.New("avmux: failed to allocate stream")
	}

	if err := codecParams.Copy(s.CodecParameters()); err != nil {
		return nil, err
	}
	return s, nil
}

func (m *StreamMuxer) WriteHeader() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("avmux: muxer is closed")
	}
	if err := m.fc.WriteHeader(nil); err != nil {
		return err
	}
	m.headerWritten = true
	return nil
}

func (m *StreamMuxer) WritePacket(pkt *astiav.Packet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("avmux: muxer is closed")
	}
	return m.fc.WriteInterleavedFrame(pkt)
}

func (m *StreamMuxer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true

	var firstErr error
	if m.fc != nil {
		if m.headerWritten {
			if err := m.fc.WriteTrailer(); err != nil {
				firstErr = err
			}
		}
		m.fc.Free()
		m.fc = nil
	}
	if m.ioCtx != nil {
		m.ioCtx.Free()
		m.ioCtx = nil
	}
	return firstErr
}
