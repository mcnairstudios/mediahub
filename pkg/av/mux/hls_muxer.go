package mux

import (
	"errors"
	"fmt"
	"log"
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

	// CopyVideoParams copies the video encoder's full codec parameters
	// to the output stream, producing correctly formatted extradata.
	// When set, this is used instead of manually setting codec fields.
	CopyVideoParams func(cp *astiav.CodecParameters) error
	// CopyAudioParams does the same for audio.
	CopyAudioParams func(cp *astiav.CodecParameters) error
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
	initPatched  bool
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

		if m.opts.CopyVideoParams != nil {
			// Use encoder's ToCodecParameters — produces correct extradata
			if err := m.opts.CopyVideoParams(cp); err != nil {
				fc.Free()
				m.fc = nil
				return fmt.Errorf("copy video encoder params: %w", err)
			}
		} else {
			// Fallback: manual codec parameter setup (copy mode)
			cp.SetCodecID(m.opts.VideoCodecID)
			cp.SetMediaType(astiav.MediaTypeVideo)
			cp.SetWidth(m.opts.VideoWidth)
			cp.SetHeight(m.opts.VideoHeight)
			if len(m.opts.VideoExtradata) > 0 {
				if err := cp.SetExtraData(m.opts.VideoExtradata); err != nil {
					fc.Free()
					m.fc = nil
					return fmt.Errorf("set video extradata: %w", err)
				}
			}
		}
		// Set hvc1 tag for HEVC — required for browser HLS playback
		if cp.CodecID() == astiav.CodecIDHevc {
			cp.SetCodecTag(0x31637668) // 'hvc1'
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

		if m.opts.CopyAudioParams != nil {
			// Use encoder's ToCodecParameters — produces correct esds
			if err := m.opts.CopyAudioParams(cp); err != nil {
				fc.Free()
				m.fc = nil
				return fmt.Errorf("copy audio encoder params: %w", err)
			}
		} else {
			// Fallback: manual codec parameter setup (copy mode)
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
			if len(m.opts.AudioExtradata) > 0 {
				if err := cp.SetExtraData(m.opts.AudioExtradata); err != nil {
					fc.Free()
					m.fc = nil
					return fmt.Errorf("set audio extradata: %w", err)
				}
			}
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

	// Post-process fMP4 init segment to fix hvcC codec string for HLS.js/video.js.
	// ffmpeg generates hvcC from Annex-B extradata with raw compat flags (0x60000000)
	// which browsers reject. Patch the init.mp4 on disk to reverse the flag bits.
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

	// Patch init.mp4 after first write — ffmpeg flushes the init segment lazily
	if !m.initPatched && m.opts.SegmentType == "fmp4" {
		initPath := filepath.Join(m.opts.OutputDir, "init.mp4")
		if info, err := os.Stat(initPath); err == nil && info.Size() > 0 {
			m.initPatched = true
			if m.opts.VideoCodecID == astiav.CodecIDHevc {
				patchHvcCFlags(initPath)
			}
			if m.opts.AudioCodecID == astiav.CodecIDAac {
				patchAACEsds(initPath, m.opts.AudioSampleRate, m.opts.AudioChannels)
			}
		}
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

// patchHvcCFlags patches the hvcC box in an fMP4 init segment to reverse
// the general_profile_compatibility_flags bit order. ffmpeg writes MSB-first
// flags (e.g. 0x60000000) but HLS.js/video.js reads them as a decimal
// integer for the codec string, producing "60000000" instead of "6".
// RFC 6381 expects reversed bits. This patches the file in-place.
func patchHvcCFlags(initPath string) {
	data, err := os.ReadFile(initPath)
	if err != nil {
		log.Printf("hls: patchHvcCFlags: read error: %v", err)
		return
	}
	// Find "hvcC" box
	needle := []byte("hvcC")
	idx := -1
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == needle[0] && data[i+1] == needle[1] && data[i+2] == needle[2] && data[i+3] == needle[3] {
			idx = i
			break
		}
	}
	if idx < 0 {
		log.Printf("hls: patchHvcCFlags: no hvcC box found in %s (%d bytes)", initPath, len(data))
		return
	}
	hvcC := idx + 4 // start of hvcC payload
	if hvcC+13 > len(data) || data[hvcC] != 1 {
		log.Printf("hls: patchHvcCFlags: hvcC too short or bad version (len=%d, version=%d)", len(data)-hvcC, data[hvcC])
		return
	}
	log.Printf("hls: patchHvcCFlags: found hvcC at offset %d, flags before: %02x%02x%02x%02x", idx, data[hvcC+2], data[hvcC+3], data[hvcC+4], data[hvcC+5])
	// Reverse bits in bytes 2-5 (general_profile_compatibility_flags)
	changed := false
	for i := hvcC + 2; i <= hvcC+5; i++ {
		b := data[i]
		rev := ((b & 0x80) >> 7) | ((b & 0x40) >> 5) | ((b & 0x20) >> 3) | ((b & 0x10) >> 1) |
			((b & 0x08) << 1) | ((b & 0x04) << 3) | ((b & 0x02) << 5) | ((b & 0x01) << 7)
		if rev != b {
			data[i] = rev
			changed = true
		}
	}
	if changed {
		os.WriteFile(initPath, data, 0644) //nolint:errcheck
		log.Printf("hls: patched hvcC compat flags in %s", filepath.Base(initPath))
	}
}

// patchAACEsds patches the esds box in an fMP4 init segment to ensure
// the AudioSpecificConfig contains AudioObjectType=2 (AAC-LC).
// Without this, HLS.js/video.js generates "mp4a.40" instead of "mp4a.40.2".
func patchAACEsds(initPath string, sampleRate, channels int) {
	data, err := os.ReadFile(initPath)
	if err != nil {
		return
	}
	// Find "esds" box
	needle := []byte("esds")
	idx := -1
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == needle[0] && data[i+1] == needle[1] && data[i+2] == needle[2] && data[i+3] == needle[3] {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	// The esds box contains nested descriptors. Find the DecoderSpecificInfo
	// descriptor (tag 0x05) which contains the AudioSpecificConfig.
	esds := data[idx+4:]
	// Search for tag 0x05 in the esds payload
	for i := 0; i < len(esds)-6; i++ {
		if esds[i] == 0x05 {
			// Skip length bytes (could be 1-4 bytes with 0x80 extension)
			j := i + 1
			for j < len(esds) && esds[j] == 0x80 {
				j++
			}
			if j >= len(esds) {
				break
			}
			configLen := int(esds[j])
			j++
			if configLen >= 2 && j+1 < len(esds) {
				// AudioSpecificConfig: first 5 bits = AudioObjectType
				aot := (esds[j] >> 3) & 0x1f
				if aot == 0 || aot > 5 {
					// Fix: write AAC-LC AudioSpecificConfig
					freqIdx := aacSampleRateIndex(sampleRate)
					if channels <= 0 {
						channels = 2
					}
					esds[j] = byte((2 << 3) | ((freqIdx >> 1) & 0x07))
					esds[j+1] = byte(((freqIdx & 0x01) << 7) | ((channels & 0x0f) << 3))
					os.WriteFile(initPath, data, 0644) //nolint:errcheck
					log.Printf("hls: patched AAC esds AudioObjectType in %s (was %d)", filepath.Base(initPath), aot)
				}
			}
			break
		}
	}
}

// PatchInitSegment patches an fMP4 init segment in memory to fix codec
// strings for browser compatibility. Reverses hvcC compat flag bits so
// HLS.js/video.js generates "hvc1.1.6.L153.B0" instead of "hvc1.1.60000000.L153.B0".
func PatchInitSegment(data []byte) []byte {
	patched := make([]byte, len(data))
	copy(patched, data)

	// Fix hvcC general_profile_compatibility_flags
	if idx := findBoxOffset(patched, "hvcC"); idx >= 0 {
		hvcC := idx + 4
		if hvcC+13 <= len(patched) && patched[hvcC] == 1 {
			for i := hvcC + 2; i <= hvcC+5; i++ {
				patched[i] = reverseByte(patched[i])
			}
		}
	}
	return patched
}

func findBoxOffset(data []byte, name string) int {
	needle := []byte(name)
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == needle[0] && data[i+1] == needle[1] && data[i+2] == needle[2] && data[i+3] == needle[3] {
			return i
		}
	}
	return -1
}

func reverseByte(b byte) byte {
	return ((b & 0x80) >> 7) | ((b & 0x40) >> 5) | ((b & 0x20) >> 3) | ((b & 0x10) >> 1) |
		((b & 0x08) << 1) | ((b & 0x04) << 3) | ((b & 0x02) << 5) | ((b & 0x01) << 7)
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
