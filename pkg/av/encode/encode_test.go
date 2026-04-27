package encode

import (
	"testing"

	"github.com/asticode/go-astiav"
)

func zeroFrameData(frame *astiav.Frame, channels, samples int) {
	size, err := frame.SamplesBufferSize(0)
	if err != nil || size <= 0 {
		return
	}
	buf := make([]byte, size)
	_ = frame.Data().SetBytes(buf, 0)
}

func TestResolveEncoderNameExplicitOverride(t *testing.T) {
	name, err := ResolveEncoderName(EncodeOpts{EncoderName: "custom_encoder"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "custom_encoder" {
		t.Fatalf("expected custom_encoder, got %s", name)
	}
}

func TestResolveEncoderNameH264Software(t *testing.T) {
	name, err := ResolveEncoderName(EncodeOpts{Codec: "h264"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "libx264" {
		t.Fatalf("expected libx264, got %s", name)
	}
}

func TestResolveEncoderNameH264VAAPI(t *testing.T) {
	name, err := ResolveEncoderName(EncodeOpts{Codec: "h264", HWAccel: "vaapi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "h264_vaapi" {
		t.Fatalf("expected h264_vaapi, got %s", name)
	}
}

func TestResolveEncoderNameH265QSV(t *testing.T) {
	name, err := ResolveEncoderName(EncodeOpts{Codec: "h265", HWAccel: "qsv"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "hevc_qsv" {
		t.Fatalf("expected hevc_qsv, got %s", name)
	}
}

func TestResolveEncoderNameH265VideoToolbox(t *testing.T) {
	name, err := ResolveEncoderName(EncodeOpts{Codec: "h265", HWAccel: "videotoolbox"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "hevc_videotoolbox" {
		t.Fatalf("expected hevc_videotoolbox, got %s", name)
	}
}

func TestResolveEncoderNameAV1NVENC(t *testing.T) {
	name, err := ResolveEncoderName(EncodeOpts{Codec: "av1", HWAccel: "nvenc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "av1_nvenc" {
		t.Fatalf("expected av1_nvenc, got %s", name)
	}
}

func TestResolveEncoderNameDefaultHWAccel(t *testing.T) {
	name, err := ResolveEncoderName(EncodeOpts{Codec: "h264", HWAccel: "default"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "libx264" {
		t.Fatalf("expected libx264 for default hwaccel, got %s", name)
	}
}

func TestResolveEncoderNameUnsupportedCodec(t *testing.T) {
	_, err := ResolveEncoderName(EncodeOpts{Codec: "vp9"})
	if err == nil {
		t.Fatal("expected error for unsupported codec")
	}
}

func TestResolveEncoderNameUnsupportedHWAccel(t *testing.T) {
	_, err := ResolveEncoderName(EncodeOpts{Codec: "av1", HWAccel: "videotoolbox"})
	if err == nil {
		t.Fatal("expected error for unsupported hwaccel+codec combination")
	}
}

func TestResolveAudioEncoderName(t *testing.T) {
	tests := []struct {
		codec    string
		expected string
	}{
		{"aac", "aac"},
		{"opus", "libopus"},
		{"mp3", "libmp3lame"},
		{"vorbis", "libvorbis"},
		{"ac3", "ac3"},
		{"eac3", "eac3"},
		{"flac", "flac"},
		{"mp2", "mp2"},
		{"unknown_codec", "unknown_codec"},
	}
	for _, tt := range tests {
		got := ResolveAudioEncoderName(tt.codec)
		if got != tt.expected {
			t.Errorf("ResolveAudioEncoderName(%q) = %q, want %q", tt.codec, got, tt.expected)
		}
	}
}

func TestNewVideoEncoderInvalidDimensions(t *testing.T) {
	_, err := NewVideoEncoder(EncodeOpts{Codec: "h264", Width: 0, Height: 1080})
	if err == nil {
		t.Fatal("expected error for zero width")
	}

	_, err = NewVideoEncoder(EncodeOpts{Codec: "h264", Width: 1920, Height: -1})
	if err == nil {
		t.Fatal("expected error for negative height")
	}
}

func TestNewAudioEncoderNoCodec(t *testing.T) {
	_, err := NewAudioEncoder(AudioEncodeOpts{})
	if err == nil {
		t.Fatal("expected error for empty codec")
	}
}

func TestNewAudioEncoderAAC(t *testing.T) {
	enc, err := NewAudioEncoder(AudioEncodeOpts{
		Codec:      "aac",
		Channels:   2,
		SampleRate: 48000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer enc.Close()

	fs := enc.FrameSize()
	if fs <= 0 {
		t.Fatalf("expected positive frame size, got %d", fs)
	}
}

func TestNewAACEncoder(t *testing.T) {
	enc, err := NewAACEncoder(2, 44100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer enc.Close()

	if enc.FrameSize() <= 0 {
		t.Fatal("expected positive frame size")
	}
}

func TestNewAudioEncoderMonoAndSurround(t *testing.T) {
	for _, ch := range []int{1, 6, 8} {
		enc, err := NewAudioEncoder(AudioEncodeOpts{
			Codec:      "aac",
			Channels:   ch,
			SampleRate: 48000,
		})
		if err != nil {
			t.Fatalf("unexpected error for %d channels: %v", ch, err)
		}
		enc.Close()
	}
}

func TestEncoderCloseIdempotent(t *testing.T) {
	enc, err := NewAACEncoder(2, 48000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enc.Close()
	enc.Close()
}

func TestExtradataBeforeEncode(t *testing.T) {
	enc, err := NewAACEncoder(2, 48000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer enc.Close()

	ed := enc.Extradata()
	if len(ed) == 0 {
		t.Fatal("expected non-empty extradata from AAC encoder")
	}
}

func TestFrameSizeNilCodecCtx(t *testing.T) {
	enc := &Encoder{}
	if enc.FrameSize() != 0 {
		t.Fatal("expected 0 from nil codecCtx")
	}
}

func TestEncodeNilCodecCtx(t *testing.T) {
	enc := &Encoder{}
	_, err := enc.Encode(nil)
	if err == nil {
		t.Fatal("expected error from nil codecCtx")
	}
}

func TestProbeMaxBitDepthSoftware(t *testing.T) {
	if ProbeMaxBitDepth("") != 0 {
		t.Fatal("expected 0 for empty hwaccel")
	}
	if ProbeMaxBitDepth("none") != 0 {
		t.Fatal("expected 0 for none hwaccel")
	}
	if ProbeMaxBitDepth("default") != 0 {
		t.Fatal("expected 0 for default hwaccel")
	}
}

func TestProbeMaxBitDepthCaching(t *testing.T) {
	probedBitDepthMu.Lock()
	probedBitDepthCache["test_cached"] = 10
	probedBitDepthMu.Unlock()

	result := ProbeMaxBitDepth("test_cached")
	if result != 10 {
		t.Fatalf("expected cached value 10, got %d", result)
	}

	probedBitDepthMu.Lock()
	delete(probedBitDepthCache, "test_cached")
	probedBitDepthMu.Unlock()
}

func TestIsHWPixelFormat(t *testing.T) {
	if !isHWPixelFormat(astiav.PixelFormatVaapi) {
		t.Fatal("expected VAAPI to be HW pixel format")
	}
	if !isHWPixelFormat(astiav.PixelFormatCuda) {
		t.Fatal("expected CUDA to be HW pixel format")
	}
	if isHWPixelFormat(astiav.PixelFormatYuv420P) {
		t.Fatal("expected YUV420P to NOT be HW pixel format")
	}
	if isHWPixelFormat(astiav.PixelFormatNv12) {
		t.Fatal("expected NV12 to NOT be HW pixel format")
	}
}

func TestEncoderTableCompleteness(t *testing.T) {
	for codec, hwMap := range encoderTable {
		if _, ok := hwMap["none"]; !ok {
			t.Errorf("codec %q missing 'none' (software) entry", codec)
		}
	}

	for codec := range softwareFallback {
		if _, ok := encoderTable[codec]; !ok {
			t.Errorf("softwareFallback has %q but encoderTable does not", codec)
		}
	}
}

func TestAudioFIFOPTSInterpolation(t *testing.T) {
	enc, err := NewAACEncoder(2, 48000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer enc.Close()

	fifo := NewAudioFIFOFromEncoder(enc, 2, astiav.ChannelLayoutStereo, 48000)
	defer fifo.Close()

	frameSize := enc.FrameSize()
	if frameSize <= 0 {
		t.Fatal("expected positive frame size")
	}

	inputPTS := int64(0)
	inputSamples := 512
	totalPkts := 0

	for i := 0; i < 20; i++ {
		frame := astiav.AllocFrame()
		frame.SetNbSamples(inputSamples)
		frame.SetSampleFormat(astiav.SampleFormatFltp)
		frame.SetChannelLayout(astiav.ChannelLayoutStereo)
		frame.SetSampleRate(48000)
		if err := frame.AllocBuffer(0); err != nil {
			frame.Free()
			t.Fatalf("alloc buffer: %v", err)
		}
		zeroFrameData(frame, 2, inputSamples)
		frame.SetPts(inputPTS)
		inputPTS += int64(inputSamples)

		pkts, err := fifo.Write(frame)
		frame.Free()
		if err != nil {
			t.Fatalf("fifo.Write: %v", err)
		}
		totalPkts += len(pkts)
		for _, pkt := range pkts {
			pkt.Free()
		}
	}

	expectedOutputFrames := (20 * inputSamples) / frameSize
	if totalPkts < expectedOutputFrames-1 {
		t.Errorf("expected at least %d packets, got %d", expectedOutputFrames-1, totalPkts)
	}
}

func TestAudioFIFOReset(t *testing.T) {
	enc, err := NewAACEncoder(2, 48000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer enc.Close()

	fifo := NewAudioFIFOFromEncoder(enc, 2, astiav.ChannelLayoutStereo, 48000)
	defer fifo.Close()

	frame := astiav.AllocFrame()
	frame.SetNbSamples(512)
	frame.SetSampleFormat(astiav.SampleFormatFltp)
	frame.SetChannelLayout(astiav.ChannelLayoutStereo)
	frame.SetSampleRate(48000)
	if err := frame.AllocBuffer(0); err != nil {
		frame.Free()
		t.Fatalf("alloc buffer: %v", err)
	}
	zeroFrameData(frame, 2, 512)
	frame.SetPts(0)

	_, err = fifo.Write(frame)
	frame.Free()
	if err != nil {
		t.Fatalf("fifo.Write: %v", err)
	}

	fifo.Reset()

	if fifo.totalInputSamples != 0 {
		t.Fatal("expected totalInputSamples=0 after reset")
	}
	if fifo.totalOutputSamples != 0 {
		t.Fatal("expected totalOutputSamples=0 after reset")
	}
	if fifo.ptsQueue != nil {
		t.Fatal("expected nil ptsQueue after reset")
	}
	if fifo.fifo != nil {
		t.Fatal("expected nil fifo after reset")
	}
}

func TestAudioFIFOPTSFromInputFrames(t *testing.T) {
	enc, err := NewAACEncoder(2, 48000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer enc.Close()

	fifo := NewAudioFIFOFromEncoder(enc, 2, astiav.ChannelLayoutStereo, 48000)
	defer fifo.Close()

	frameSize := enc.FrameSize()
	seekPTS := int64(48000 * 60)

	var collectedPTS []int64

	for i := 0; i < 30; i++ {
		frame := astiav.AllocFrame()
		frame.SetNbSamples(512)
		frame.SetSampleFormat(astiav.SampleFormatFltp)
		frame.SetChannelLayout(astiav.ChannelLayoutStereo)
		frame.SetSampleRate(48000)
		if err := frame.AllocBuffer(0); err != nil {
			frame.Free()
			t.Fatalf("alloc buffer: %v", err)
		}
		zeroFrameData(frame, 2, 512)
		frame.SetPts(seekPTS + int64(i*512))

		pkts, err := fifo.Write(frame)
		frame.Free()
		if err != nil {
			t.Fatalf("fifo.Write: %v", err)
		}
		for _, pkt := range pkts {
			collectedPTS = append(collectedPTS, pkt.Pts())
			pkt.Free()
		}
	}

	if len(collectedPTS) == 0 {
		t.Fatal("expected at least one output packet")
	}

	drift := collectedPTS[0] - seekPTS
	if drift < -int64(frameSize) || drift > int64(frameSize) {
		t.Errorf("first output PTS %d too far from seekPTS %d (drift=%d, frameSize=%d)",
			collectedPTS[0], seekPTS, drift, frameSize)
	}

	if collectedPTS[0] < seekPTS/2 {
		t.Errorf("PTS %d appears counter-based (near zero), expected near seekPTS %d",
			collectedPTS[0], seekPTS)
	}

	for i := 1; i < len(collectedPTS); i++ {
		expected := collectedPTS[i-1] + int64(frameSize)
		if collectedPTS[i] != expected {
			t.Errorf("PTS[%d]=%d, expected %d (delta=%d, frameSize=%d)",
				i, collectedPTS[i], expected, collectedPTS[i]-collectedPTS[i-1], frameSize)
		}
	}
}

func TestNewAudioFIFOManualParams(t *testing.T) {
	enc, err := NewAACEncoder(2, 48000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer enc.Close()

	fifo := NewAudioFIFO(enc, 1024, 2, astiav.SampleFormatFltp, astiav.ChannelLayoutStereo, 48000)
	defer fifo.Close()

	if fifo.frameSize != 1024 {
		t.Fatalf("expected frameSize=1024, got %d", fifo.frameSize)
	}
	if fifo.channels != 2 {
		t.Fatalf("expected channels=2, got %d", fifo.channels)
	}
	if fifo.rate != 48000 {
		t.Fatalf("expected rate=48000, got %d", fifo.rate)
	}
}

func TestNewVideoEncoderSoftwareH264(t *testing.T) {
	enc, err := NewVideoEncoder(EncodeOpts{
		Codec:  "h264",
		Width:  320,
		Height: 240,
	})
	if err != nil {
		t.Skipf("libx264 not available: %v", err)
	}
	defer enc.Close()

	ed := enc.Extradata()
	if len(ed) == 0 {
		t.Fatal("expected non-empty extradata from H.264 encoder")
	}
}

func TestNewVideoEncoderFramerate(t *testing.T) {
	enc, err := NewVideoEncoder(EncodeOpts{
		Codec:     "h264",
		Width:     320,
		Height:    240,
		Framerate: 30,
	})
	if err != nil {
		t.Skipf("libx264 not available: %v", err)
	}
	defer enc.Close()
}

func TestNewVideoEncoderWithBitrateAndGOP(t *testing.T) {
	enc, err := NewVideoEncoder(EncodeOpts{
		Codec:            "h264",
		Width:            320,
		Height:           240,
		Bitrate:          2000,
		KeyframeInterval: 60,
	})
	if err != nil {
		t.Skipf("libx264 not available: %v", err)
	}
	defer enc.Close()
}
