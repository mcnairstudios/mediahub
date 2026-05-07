package mux

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/asticode/go-astiav"
)

type HLSMuxOpts struct {
	OutputDir          string
	SegmentDurationSec int
	SegmentType        string
	IsLive             bool
	VideoCodecID       astiav.CodecID
	VideoExtradata     []byte
	VideoWidth         int
	VideoHeight        int
	VideoTimeBase      astiav.Rational
	VideoFrameRate     int
	AudioCodecID       astiav.CodecID
	AudioExtradata     []byte
	AudioChannels      int
	AudioSampleRate    int
	AudioTimeBase      astiav.Rational
	AudioFrameSize     int
}

type HLSMuxer struct {
	opts         HLSMuxOpts
	fc           *astiav.FormatContext
	videoIdx     int
	audioIdx     int
	videoOutTB   astiav.Rational
	audioOutTB   astiav.Rational
	closed       bool
	mu           sync.Mutex
	lastVideoDTS int64
	lastAudioDTS int64
	videoDTSInit bool
	audioDTSInit bool
}

func NewHLSMuxer(opts HLSMuxOpts) (*HLSMuxer, error) {
	if opts.OutputDir == "" {
		return nil, errors.New("avmux: OutputDir is required")
	}
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("avmux: create output dir: %w", err)
	}
	if opts.SegmentDurationSec <= 0 {
		opts.SegmentDurationSec = 6
	}
	if opts.SegmentType == "" {
		opts.SegmentType = "mpegts"
	}

	m := &HLSMuxer{
		opts:     opts,
		videoIdx: -1,
		audioIdx: -1,
	}

	if err := m.openFormatContext(); err != nil {
		return nil, fmt.Errorf("avmux: open hls muxer: %w", err)
	}

	return m, nil
}

func (m *HLSMuxer) openFormatContext() error {
	playlistPath := filepath.Join(m.opts.OutputDir, "playlist.m3u8")

	fc, err := astiav.AllocOutputFormatContext(nil, "hls", playlistPath)
	if err != nil {
		return fmt.Errorf("alloc output format context: %w", err)
	}
	m.fc = fc

	if m.opts.VideoCodecID != astiav.CodecIDNone {
		vs := fc.NewStream(nil)
		if vs == nil {
			fc.Free()
			m.fc = nil
			return errors.New("failed to allocate video stream")
		}
		cp := vs.CodecParameters()
		cp.SetCodecID(m.opts.VideoCodecID)
		cp.SetMediaType(astiav.MediaTypeVideo)
		cp.SetWidth(m.opts.VideoWidth)
		cp.SetHeight(m.opts.VideoHeight)
		if len(m.opts.VideoExtradata) > 0 {
			ed := m.opts.VideoExtradata
			// Scrub HEVC hvcC general_profile_compatibility_flags for HLS.js.
			// HLS.js reads bytes 2-5 of hvcC as a decimal integer for the codec
			// string. VT encoder writes 0x60000000 which becomes "60000000" —
			// Chrome rejects this. Reverse the bit order so HLS.js gets "6".
			if m.opts.VideoCodecID == astiav.CodecIDHevc && len(ed) >= 13 && ed[0] == 1 {
				ed = make([]byte, len(m.opts.VideoExtradata))
				copy(ed, m.opts.VideoExtradata)
				// Reverse bits in each of the 4 flag bytes (bytes 2-5)
				for i := 2; i <= 5; i++ {
					b := ed[i]
					ed[i] = ((b & 0x80) >> 7) | ((b & 0x40) >> 5) | ((b & 0x20) >> 3) | ((b & 0x10) >> 1) |
						((b & 0x08) << 1) | ((b & 0x04) << 3) | ((b & 0x02) << 5) | ((b & 0x01) << 7)
				}
			}
			if err := cp.SetExtraData(ed); err != nil {
				fc.Free()
				m.fc = nil
				return fmt.Errorf("set video extradata: %w", err)
			}
		}
		// HEVC: set hvc1 tag + profile/level for clean hvcC box.
		if m.opts.VideoCodecID == astiav.CodecIDHevc {
			cp.SetCodecTag(0x31637668) // 'hvc1' in little-endian
			cp.SetProfile(astiav.ProfileHevcMain)
			if m.opts.VideoHeight >= 2160 {
				cp.SetLevel(153) // Level 5.1 for 4K
			} else if m.opts.VideoHeight >= 1080 {
				cp.SetLevel(120) // Level 4.0 for 1080p
			} else {
				cp.SetLevel(93) // Level 3.1
			}
		} else if m.opts.VideoCodecID == astiav.CodecIDH264 {
			cp.SetProfile(astiav.ProfileH264Main)
		}
		vs.SetTimeBase(m.opts.VideoTimeBase)
		m.videoIdx = vs.Index()
	}

	if m.opts.AudioCodecID != astiav.CodecIDNone {
		as := fc.NewStream(nil)
		if as == nil {
			fc.Free()
			m.fc = nil
			return errors.New("failed to allocate audio stream")
		}
		cp := as.CodecParameters()
		cp.SetCodecID(m.opts.AudioCodecID)
		cp.SetMediaType(astiav.MediaTypeAudio)
		if m.opts.AudioSampleRate > 0 {
			cp.SetSampleRate(m.opts.AudioSampleRate)
		}
		switch m.opts.AudioChannels {
		case 1:
			cp.SetChannelLayout(astiav.ChannelLayoutMono)
		case 2:
			cp.SetChannelLayout(astiav.ChannelLayoutStereo)
		case 6:
			cp.SetChannelLayout(astiav.ChannelLayout5Point1)
		case 8:
			cp.SetChannelLayout(astiav.ChannelLayout7Point1)
		}
		audioExtra := m.opts.AudioExtradata
		// Generate AAC AudioSpecificConfig if not provided.
		// Without it, esds lacks the AudioObjectType and Chrome gets
		// mp4a.40 instead of mp4a.40.2 (AAC-LC).
		if len(audioExtra) == 0 && m.opts.AudioCodecID == astiav.CodecIDAac {
			audioExtra = buildAACExtradata(m.opts.AudioSampleRate, m.opts.AudioChannels)
		}
		if len(audioExtra) > 0 {
			if err := cp.SetExtraData(audioExtra); err != nil {
				fc.Free()
				m.fc = nil
				return fmt.Errorf("set audio extradata: %w", err)
			}
		}
		// AAC: set profile to AAC-LC so esds contains AudioObjectType 2.
		// Without this, Chrome gets mp4a.40 instead of mp4a.40.2.
		if m.opts.AudioCodecID == astiav.CodecIDAac {
			cp.SetProfile(astiav.ProfileAacLow)
		}
		as.SetTimeBase(m.opts.AudioTimeBase)
		m.audioIdx = as.Index()
	}

	dict := astiav.NewDictionary()
	defer dict.Free()
	dict.Set("hls_time", strconv.Itoa(m.opts.SegmentDurationSec), 0)
	if m.opts.IsLive {
		// Live: sliding window playlist, delete old segments
		dict.Set("hls_list_size", "6", 0)
		dict.Set("hls_flags", "delete_segments", 0)
	} else {
		// VOD: keep all segments, EVENT type tells HLS.js this is a growing
		// VOD playlist (not live) so it shows duration and enables seeking
		dict.Set("hls_list_size", "0", 0)
		dict.Set("hls_playlist_type", "event", 0)
	}

	if m.opts.SegmentType == "fmp4" {
		dict.Set("hls_segment_type", "fmp4", 0)
		dict.Set("hls_fmp4_init_filename", "init.mp4", 0)
		dict.Set("hls_segment_filename", filepath.Join(m.opts.OutputDir, "seg%d.m4s"), 0)
	} else {
		dict.Set("hls_segment_filename", filepath.Join(m.opts.OutputDir, "seg%d.ts"), 0)
	}

	if err := fc.WriteHeader(dict); err != nil {
		fc.Free()
		m.fc = nil
		return fmt.Errorf("write header: %w", err)
	}

	if m.videoIdx >= 0 {
		m.videoOutTB = fc.Streams()[m.videoIdx].TimeBase()
	}
	if m.audioIdx >= 0 {
		m.audioOutTB = fc.Streams()[m.audioIdx].TimeBase()
	}

	return nil
}

func (m *HLSMuxer) fixVideoDuration(pkt *astiav.Packet) {
	if pkt.Duration() > 0 {
		return
	}
	fps := m.opts.VideoFrameRate
	if fps <= 0 {
		fps = 25
	}
	outTB := m.videoOutTB
	if outTB.Den() > 0 && outTB.Num() > 0 {
		pkt.SetDuration(int64(outTB.Den()) / (int64(fps) * int64(outTB.Num())))
	}
}

func (m *HLSMuxer) fixAudioDuration(pkt *astiav.Packet) {
	if pkt.Duration() > 0 {
		return
	}
	frameSize := m.opts.AudioFrameSize
	if frameSize <= 0 {
		frameSize = 1024
	}
	sampleRate := m.opts.AudioSampleRate
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	outTB := m.audioOutTB
	if outTB.Den() > 0 && outTB.Num() > 0 {
		pkt.SetDuration(int64(frameSize) * int64(outTB.Den()) / (int64(sampleRate) * int64(outTB.Num())))
	}
}

func (m *HLSMuxer) WriteVideoPacket(pkt *astiav.Packet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("avmux: muxer is closed")
	}
	if m.fc == nil || m.videoIdx < 0 {
		return errors.New("avmux: no video track configured")
	}

	pkt.RescaleTs(m.opts.VideoTimeBase, m.videoOutTB)
	pkt.SetStreamIndex(m.videoIdx)
	m.fixVideoDuration(pkt)

	dts := pkt.Dts()
	if m.videoDTSInit && dts <= m.lastVideoDTS {
		bump := pkt.Duration()
		if bump <= 0 {
			bump = 1
		}
		dts = m.lastVideoDTS + bump
		pts := pkt.Pts()
		if pts < dts {
			pts = dts
		}
		pkt.SetDts(dts)
		pkt.SetPts(pts)
	}
	m.lastVideoDTS = dts
	m.videoDTSInit = true

	if err := m.fc.WriteInterleavedFrame(pkt); err != nil {
		return fmt.Errorf("avmux: write video packet: %w", err)
	}
	return nil
}

func (m *HLSMuxer) WriteAudioPacket(pkt *astiav.Packet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("avmux: muxer is closed")
	}
	if m.fc == nil || m.audioIdx < 0 {
		return errors.New("avmux: no audio track configured")
	}

	pkt.RescaleTs(m.opts.AudioTimeBase, m.audioOutTB)
	pkt.SetStreamIndex(m.audioIdx)
	m.fixAudioDuration(pkt)

	if err := m.fc.WriteInterleavedFrame(pkt); err != nil {
		return fmt.Errorf("avmux: write audio packet: %w", err)
	}
	return nil
}

func (m *HLSMuxer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true

	if m.fc == nil {
		return nil
	}

	var firstErr error
	if err := m.fc.WriteTrailer(); err != nil {
		firstErr = err
	}
	m.fc.Free()
	m.fc = nil

	return firstErr
}

func (m *HLSMuxer) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}

	if m.fc != nil {
		m.fc.WriteTrailer() //nolint:errcheck
		m.fc.Free()
		m.fc = nil
	}

	segExt := "ts"
	if m.opts.SegmentType == "fmp4" {
		segExt = "m4s"
	}
	matches, _ := filepath.Glob(filepath.Join(m.opts.OutputDir, "seg*."+segExt))
	for _, f := range matches {
		os.Remove(f)
	}
	if m.opts.SegmentType == "fmp4" {
		os.Remove(filepath.Join(m.opts.OutputDir, "init.mp4"))
	}
	os.Remove(filepath.Join(m.opts.OutputDir, "playlist.m3u8"))

	m.videoIdx = -1
	m.audioIdx = -1

	return m.openFormatContext()
}

// buildAACExtradata generates a 2-byte AudioSpecificConfig for AAC-LC.
// This ensures the esds box contains AudioObjectType=2 (AAC-LC) so that
// HLS.js generates mp4a.40.2 instead of mp4a.40.
func buildAACExtradata(sampleRate, channels int) []byte {
	// AudioSpecificConfig (ISO 14496-3):
	// 5 bits: audioObjectType (2 = AAC-LC)
	// 4 bits: samplingFrequencyIndex
	// 4 bits: channelConfiguration
	// 1 bit: frameLengthFlag (0)
	// 1 bit: dependsOnCoreCoder (0)
	// 1 bit: extensionFlag (0)
	freqIdx := aacSampleRateIndex(sampleRate)
	if channels <= 0 {
		channels = 2
	}
	// Byte 0: objectType(5) | freqIdx(top 3 bits)
	// Byte 1: freqIdx(bottom 1 bit) | channels(4) | 000
	b0 := byte((2 << 3) | ((freqIdx >> 1) & 0x07))
	b1 := byte(((freqIdx & 0x01) << 7) | ((channels & 0x0f) << 3))
	return []byte{b0, b1}
}

func aacSampleRateIndex(rate int) int {
	rates := []int{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350}
	for i, r := range rates {
		if rate == r {
			return i
		}
	}
	return 3 // default to 48000
}

func (m *HLSMuxer) SegmentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	segExt := "ts"
	if m.opts.SegmentType == "fmp4" {
		segExt = "m4s"
	}
	matches, _ := filepath.Glob(filepath.Join(m.opts.OutputDir, "seg*."+segExt))
	return len(matches)
}

func (m *HLSMuxer) PlaylistContent() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(m.opts.OutputDir, "playlist.m3u8"))
	if err != nil {
		return ""
	}
	return string(data)
}
