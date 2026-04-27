package mux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/asticode/go-astiav"
)

func TestFragmentedMuxer_VideoSegmentProduction(t *testing.T) {
	dir := t.TempDir()

	extradata := []byte{
		0x01, 0x42, 0xC0, 0x1E, 0xFF, 0xE1,
		0x00, 0x04, 0x67, 0x42, 0xC0, 0x1E,
		0x01,
		0x00, 0x02, 0x68, 0xCE,
	}

	m, err := NewFragmentedMuxer(MuxOpts{
		OutputDir:      dir,
		VideoCodecID:   astiav.CodecIDH264,
		VideoExtradata: extradata,
		VideoWidth:     640,
		VideoHeight:    480,
		VideoTimeBase:  astiav.NewRational(1, 90000),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	if _, err := os.Stat(filepath.Join(dir, "init_video.mp4")); err != nil {
		t.Fatalf("init_video.mp4 missing: %v", err)
	}

	codec := astiav.FindEncoderByName("libx264")
	if codec == nil {
		t.Skip("libx264 not available")
	}
	cc := astiav.AllocCodecContext(codec)
	if cc == nil {
		t.Fatal("failed to alloc codec context")
	}
	cc.SetWidth(640)
	cc.SetHeight(480)
	cc.SetPixelFormat(astiav.PixelFormatYuv420P)
	cc.SetTimeBase(astiav.NewRational(1, 25))
	cc.SetFramerate(astiav.NewRational(25, 1))
	cc.SetGopSize(5)
	cc.SetFlags(astiav.NewCodecContextFlags(astiav.CodecContextFlagGlobalHeader))

	if err := cc.Open(codec, nil); err != nil {
		cc.Free()
		t.Fatalf("open encoder: %v", err)
	}
	defer cc.Free()

	frame := astiav.AllocFrame()
	if frame == nil {
		t.Fatal("alloc frame")
	}
	defer frame.Free()
	frame.SetWidth(640)
	frame.SetHeight(480)
	frame.SetPixelFormat(astiav.PixelFormatYuv420P)
	if err := frame.AllocBuffer(0); err != nil {
		t.Fatalf("alloc buffer: %v", err)
	}

	var totalPackets int
	outTB := astiav.NewRational(1, 90000)
	for i := 0; i < 50; i++ {
		frame.SetPts(int64(i))
		if err := cc.SendFrame(frame); err != nil {
			t.Fatalf("send frame %d: %v", i, err)
		}

		for {
			pkt := astiav.AllocPacket()
			if err := cc.ReceivePacket(pkt); err != nil {
				pkt.Free()
				break
			}

			pkt.RescaleTs(cc.TimeBase(), outTB)
			if pkt.Duration() == 0 {
				pkt.SetDuration(int64(outTB.Den()) / int64(cc.Framerate().Num()))
			}

			dur := pktDurationUs(pkt, m.video.stream)
			isKf := pkt.Flags().Has(astiav.PacketFlagKey)
			t.Logf("pkt %d: dur=%d durationUs=%d keyframe=%v", totalPackets, pkt.Duration(), dur, isKf)

			if err := m.WriteVideoPacket(pkt); err != nil {
				pkt.Free()
				t.Fatalf("write video pkt %d: %v", totalPackets, err)
			}
			pkt.Free()
			totalPackets++
		}
	}

	cc.SendFrame(nil) //nolint:errcheck
	for {
		pkt := astiav.AllocPacket()
		if err := cc.ReceivePacket(pkt); err != nil {
			pkt.Free()
			break
		}
		pkt.RescaleTs(cc.TimeBase(), outTB)
		if pkt.Duration() == 0 {
			pkt.SetDuration(int64(outTB.Den()) / int64(cc.Framerate().Num()))
		}
		m.WriteVideoPacket(pkt) //nolint:errcheck
		pkt.Free()
		totalPackets++
	}

	if totalPackets == 0 {
		t.Fatal("encoder produced no packets")
	}

	if err := m.Close(); err != nil {
		t.Fatalf("close muxer: %v", err)
	}

	segments, err := filepath.Glob(filepath.Join(dir, "video_*.m4s"))
	if err != nil {
		t.Fatal(err)
	}

	if len(segments) == 0 {
		t.Errorf("no video segments produced after %d packets (accumDurationUs was never > 0 at keyframe?)", totalPackets)

		entries, _ := os.ReadDir(dir)
		t.Log("directory contents:")
		for _, e := range entries {
			info, _ := e.Info()
			t.Logf("  %s (%d bytes)", e.Name(), info.Size())
		}
	} else {
		t.Logf("produced %d video segments from %d packets", len(segments), totalPackets)
		for _, seg := range segments {
			info, _ := os.Stat(seg)
			t.Logf("  %s (%d bytes)", filepath.Base(seg), info.Size())
		}
	}
}

func TestFragmentedMuxer_AudioSegmentProduction(t *testing.T) {
	dir := t.TempDir()

	codec := astiav.FindEncoderByName("aac")
	if codec == nil {
		t.Skip("aac encoder not available")
	}
	cc := astiav.AllocCodecContext(codec)
	if cc == nil {
		t.Fatal("alloc codec context")
	}
	cc.SetSampleRate(48000)
	cc.SetSampleFormat(astiav.SampleFormatFltp)
	cc.SetChannelLayout(astiav.ChannelLayoutStereo)
	cc.SetFlags(astiav.NewCodecContextFlags(astiav.CodecContextFlagGlobalHeader))
	cc.SetTimeBase(astiav.NewRational(1, 48000))

	if err := cc.Open(codec, nil); err != nil {
		cc.Free()
		t.Fatalf("open AAC encoder: %v", err)
	}
	defer cc.Free()

	aacExtradata := cc.ExtraData()
	t.Logf("AAC extradata: %d bytes, frame_size=%d", len(aacExtradata), cc.FrameSize())

	m, err := NewFragmentedMuxer(MuxOpts{
		OutputDir:       dir,
		AudioCodecID:    astiav.CodecIDAac,
		AudioExtradata:  aacExtradata,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		AudioFragmentMs: 500,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "init_audio.mp4")); err != nil {
		t.Fatalf("init_audio.mp4 missing: %v", err)
	}

	outTB := astiav.NewRational(1, 48000)
	var totalPackets int
	planeSizeBytes := cc.FrameSize() * 4
	zeroBuf := make([]byte, planeSizeBytes)

	for i := 0; i < 100; i++ {
		frame := astiav.AllocFrame()
		frame.SetSampleRate(48000)
		frame.SetSampleFormat(astiav.SampleFormatFltp)
		frame.SetChannelLayout(astiav.ChannelLayoutStereo)
		frame.SetNbSamples(cc.FrameSize())
		if err := frame.AllocBuffer(0); err != nil {
			frame.Free()
			t.Fatalf("alloc audio buffer: %v", err)
		}
		_ = frame.Data().SetBytes(zeroBuf, 0)
		_ = frame.Data().SetBytes(zeroBuf, 1)
		frame.SetPts(int64(i) * int64(cc.FrameSize()))

		if err := cc.SendFrame(frame); err != nil {
			frame.Free()
			continue
		}
		frame.Free()

		for {
			pkt := astiav.AllocPacket()
			if err := cc.ReceivePacket(pkt); err != nil {
				pkt.Free()
				break
			}

			pkt.RescaleTs(cc.TimeBase(), outTB)
			if pkt.Duration() == 0 {
				pkt.SetDuration(int64(cc.FrameSize()))
			}

			dur := pktDurationUs(pkt, m.audio.stream)
			if totalPackets < 3 {
				t.Logf("audio pkt %d: dur=%d durationUs=%d", totalPackets, pkt.Duration(), dur)
			}

			if err := m.WriteAudioPacket(pkt); err != nil {
				pkt.Free()
				t.Fatalf("write audio pkt %d: %v", totalPackets, err)
			}
			pkt.Free()
			totalPackets++
		}
	}

	if totalPackets == 0 {
		t.Fatal("AAC encoder produced no packets")
	}

	if err := m.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	segments, err := filepath.Glob(filepath.Join(dir, "audio_*.m4s"))
	if err != nil {
		t.Fatal(err)
	}

	if len(segments) == 0 {
		t.Errorf("no audio segments produced after %d packets", totalPackets)
	} else {
		t.Logf("produced %d audio segments from %d packets", len(segments), totalPackets)
	}
}

func TestFragmentedMuxer_CopyMode_BFrameDTS(t *testing.T) {
	dir := t.TempDir()

	extradata := []byte{
		0x01, 0x42, 0xC0, 0x1E, 0xFF, 0xE1,
		0x00, 0x04, 0x67, 0x42, 0xC0, 0x1E,
		0x01,
		0x00, 0x02, 0x68, 0xCE,
	}

	m, err := NewFragmentedMuxer(MuxOpts{
		OutputDir:      dir,
		VideoCodecID:   astiav.CodecIDH264,
		VideoExtradata: extradata,
		VideoWidth:     1920,
		VideoHeight:    1080,
		VideoTimeBase:  astiav.NewRational(1, 90000),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	type pktDef struct {
		pts, dts int64
		dur      int64
		keyframe bool
		size     int
	}

	pkts := []pktDef{
		{pts: 0, dts: 0, dur: 3000, keyframe: true, size: 5000},
		{pts: 6000, dts: 3000, dur: 3000, keyframe: false, size: 500},
		{pts: 3000, dts: 3000, dur: 3000, keyframe: false, size: 300},
		{pts: 9000, dts: 6000, dur: 3000, keyframe: false, size: 600},
		{pts: 12000, dts: 9000, dur: 3000, keyframe: true, size: 4000},
		{pts: 18000, dts: 12000, dur: 3000, keyframe: false, size: 500},
		{pts: 15000, dts: 12000, dur: 3000, keyframe: false, size: 300},
		{pts: 21000, dts: 15000, dur: 3000, keyframe: false, size: 600},
		{pts: 24000, dts: 18000, dur: 3000, keyframe: true, size: 4000},
	}

	for i, pd := range pkts {
		pkt := astiav.AllocPacket()
		if pkt == nil {
			t.Fatalf("alloc pkt %d", i)
		}
		data := make([]byte, pd.size)
		data[0] = 0x65
		if err := pkt.FromData(data); err != nil {
			pkt.Free()
			t.Fatalf("from data %d: %v", i, err)
		}
		pkt.SetPts(pd.pts)
		pkt.SetDts(pd.dts)
		pkt.SetDuration(pd.dur)
		if pd.keyframe {
			pkt.SetFlags(pkt.Flags().Add(astiav.PacketFlagKey))
		}

		err := m.WriteVideoPacket(pkt)
		pkt.Free()
		if err != nil {
			t.Errorf("pkt %d (pts=%d dts=%d kf=%v): %v", i, pd.pts, pd.dts, pd.keyframe, err)
		}
	}

	if err := m.Close(); err != nil {
		t.Errorf("close: %v", err)
	}

	segments, _ := filepath.Glob(filepath.Join(dir, "video_*.m4s"))
	if len(segments) < 2 {
		t.Errorf("expected >= 2 video segments (3 keyframes), got %d", len(segments))
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			info, _ := e.Info()
			t.Logf("  %s (%d bytes)", e.Name(), info.Size())
		}
	} else {
		t.Logf("produced %d segments with B-frame DTS (including duplicates)", len(segments))
	}
}

func TestPktDurationUs_NonZero(t *testing.T) {
	pkt := astiav.AllocPacket()
	if pkt == nil {
		t.Fatal("alloc packet")
	}
	defer pkt.Free()

	fc, err := astiav.AllocOutputFormatContext(nil, "mp4", "")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Free()

	s := fc.NewStream(nil)
	if s == nil {
		t.Fatal("new stream")
	}
	s.SetTimeBase(astiav.NewRational(1, 90000))
	s.CodecParameters().SetCodecID(astiav.CodecIDH264)
	s.CodecParameters().SetMediaType(astiav.MediaTypeVideo)

	pkt.SetDuration(3600)
	pkt.SetStreamIndex(0)

	dur := pktDurationUs(pkt, s)
	if dur == 0 {
		t.Errorf("pktDurationUs returned 0 for duration=%d, timebase=1/90000", pkt.Duration())
	}
	expectedUs := int64(3600) * 1 * 1_000_000 / 90000
	if dur != expectedUs {
		t.Errorf("pktDurationUs = %d, want %d", dur, expectedUs)
	}
}

func TestFragmentedMuxer_Reset_SeekContinues(t *testing.T) {
	dir := t.TempDir()

	extradata := []byte{
		0x01, 0x42, 0xC0, 0x1E, 0xFF, 0xE1,
		0x00, 0x04, 0x67, 0x42, 0xC0, 0x1E,
		0x01,
		0x00, 0x02, 0x68, 0xCE,
	}

	m, err := NewFragmentedMuxer(MuxOpts{
		OutputDir:      dir,
		VideoCodecID:   astiav.CodecIDH264,
		VideoExtradata: extradata,
		VideoWidth:     1920,
		VideoHeight:    1080,
		VideoTimeBase:  astiav.NewRational(1, 90000),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	writePkt := func(pts, dts int64, kf bool) {
		pkt := astiav.AllocPacket()
		data := make([]byte, 500)
		pkt.FromData(data)
		pkt.SetPts(pts)
		pkt.SetDts(dts)
		pkt.SetDuration(3600)
		if kf {
			pkt.SetFlags(pkt.Flags().Add(astiav.PacketFlagKey))
		}
		m.WriteVideoPacket(pkt)
		pkt.Free()
	}

	writePkt(0, 0, true)
	writePkt(3600, 3600, false)
	writePkt(7200, 7200, false)
	writePkt(10800, 10800, true)
	writePkt(14400, 14400, false)

	segsBefore, _ := filepath.Glob(filepath.Join(dir, "video_*.m4s"))
	t.Logf("before reset: %d segments", len(segsBefore))

	m.Reset()

	writePkt(5400000, 5400000, true)
	writePkt(5403600, 5403600, false)
	writePkt(5407200, 5407200, false)
	writePkt(5410800, 5410800, true)
	writePkt(5414400, 5414400, false)

	segsAfter, _ := filepath.Glob(filepath.Join(dir, "video_*.m4s"))
	newSegs := len(segsAfter) - len(segsBefore)
	if newSegs <= 0 {
		t.Error("no new segments after seek Reset")
	} else {
		t.Logf("after reset: %d new segments (seq continues)", newSegs)
	}

	m.Reset()
	writePkt(2700000, 2700000, true)
	writePkt(2703600, 2703600, false)
	writePkt(2707200, 2707200, true)

	segsFinal, _ := filepath.Glob(filepath.Join(dir, "video_*.m4s"))
	if len(segsFinal) <= len(segsAfter) {
		t.Error("no segments after backward seek Reset")
	} else {
		t.Logf("after backward seek: %d total segments", len(segsFinal))
	}
}

func TestFragmentedMuxer_MaxDurationFlush(t *testing.T) {
	dir := t.TempDir()

	extradata := []byte{
		0x01, 0x42, 0xC0, 0x1E, 0xFF, 0xE1,
		0x00, 0x04, 0x67, 0x42, 0xC0, 0x1E,
		0x01,
		0x00, 0x02, 0x68, 0xCE,
	}

	m, err := NewFragmentedMuxer(MuxOpts{
		OutputDir:         dir,
		VideoCodecID:      astiav.CodecIDH264,
		VideoExtradata:    extradata,
		VideoWidth:        1920,
		VideoHeight:       1080,
		VideoTimeBase:     astiav.NewRational(1, 90000),
		SegmentDurationMs: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	for i := 0; i < 100; i++ {
		pkt := astiav.AllocPacket()
		data := make([]byte, 5000)
		pkt.FromData(data)
		pkt.SetPts(int64(i) * 3600)
		pkt.SetDts(int64(i) * 3600)
		pkt.SetDuration(3600)
		if i == 0 {
			pkt.SetFlags(pkt.Flags().Add(astiav.PacketFlagKey))
		}
		m.WriteVideoPacket(pkt)
		pkt.Free()
	}

	m.Close()

	segments, _ := filepath.Glob(filepath.Join(dir, "video_*.m4s"))
	if len(segments) == 0 {
		t.Error("no segments with SegmentDurationMs=2000 and no second keyframe")
	} else {
		t.Logf("produced %d segments with max duration flush (no second keyframe)", len(segments))
	}
}

func TestFragmentedMuxer_ZeroDurationPackets(t *testing.T) {
	dir := t.TempDir()

	extradata := []byte{
		0x01, 0x42, 0xC0, 0x1E, 0xFF, 0xE1,
		0x00, 0x04, 0x67, 0x42, 0xC0, 0x1E,
		0x01,
		0x00, 0x02, 0x68, 0xCE,
	}

	m, err := NewFragmentedMuxer(MuxOpts{
		OutputDir:      dir,
		VideoCodecID:   astiav.CodecIDH264,
		VideoExtradata: extradata,
		VideoWidth:     1920,
		VideoHeight:    1080,
		VideoTimeBase:  astiav.NewRational(1, 90000),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	for i := 0; i < 20; i++ {
		pkt := astiav.AllocPacket()
		data := make([]byte, 500)
		data[0] = 0x65
		pkt.FromData(data)
		pkt.SetPts(int64(i) * 3600)
		pkt.SetDts(int64(i) * 3600)
		pkt.SetDuration(0)
		if i%5 == 0 {
			pkt.SetFlags(pkt.Flags().Add(astiav.PacketFlagKey))
		}

		err := m.WriteVideoPacket(pkt)
		pkt.Free()
		if err != nil {
			t.Errorf("pkt %d: %v", i, err)
		}
	}

	m.Close()

	segments, _ := filepath.Glob(filepath.Join(dir, "video_*.m4s"))
	if len(segments) == 0 {
		t.Errorf("no segments produced with zero-duration packets and 4 keyframes")
	} else {
		t.Logf("produced %d segments with zero-duration packets", len(segments))
	}
}
