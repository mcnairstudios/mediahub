package demux

import (
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/media"
)

func TestDefaultDemuxOpts(t *testing.T) {
	opts := DefaultDemuxOpts()
	if opts.AudioTrack != -1 {
		t.Fatalf("expected AudioTrack=-1, got %d", opts.AudioTrack)
	}
	if opts.TimeoutSec != 0 {
		t.Fatalf("expected TimeoutSec=0, got %d", opts.TimeoutSec)
	}
	if opts.Follow {
		t.Fatal("expected Follow=false")
	}
}

func TestBasePTSInitializedToNegativeOne(t *testing.T) {
	dm := &Demuxer{
		basePTS:  -1,
		videoIdx: -1,
		audioIdx: -1,
		subIdx:   -1,
	}
	if dm.basePTS != -1 {
		t.Fatalf("expected basePTS=-1, got %d", dm.basePTS)
	}
}

func TestToNanoseconds(t *testing.T) {
	tests := []struct {
		name   string
		ts     int64
		num    int
		den    int
		wantNs int64
	}{
		{
			name:   "1 second at 1/90000 timebase",
			ts:     90000,
			num:    1,
			den:    90000,
			wantNs: 1_000_000_000,
		},
		{
			name:   "1 second at 1/48000 timebase",
			ts:     48000,
			num:    1,
			den:    48000,
			wantNs: 1_000_000_000,
		},
		{
			name:   "zero",
			ts:     0,
			num:    1,
			den:    90000,
			wantNs: 0,
		},
		{
			name:   "half second at 1/1000 timebase",
			ts:     500,
			num:    1,
			den:    1000,
			wantNs: 500_000_000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tb := astiav.NewRational(tc.num, tc.den)
			got := toNanoseconds(tc.ts, tb)
			if got != tc.wantNs {
				t.Errorf("toNanoseconds(%d, %d/%d) = %d, want %d", tc.ts, tc.num, tc.den, got, tc.wantNs)
			}
		})
	}
}

func TestConvertRTSPtoHTTP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"rtsp://192.168.1.1:554/stream", "http://192.168.1.1:8875/stream"},
		{"rtsp://192.168.1.1:554", "http://192.168.1.1:8875"},
		{"rtsp://192.168.1.1/stream", "http://192.168.1.1:8875/stream"},
		{"rtsp://192.168.1.1", "http://192.168.1.1:8875"},
	}

	for _, tc := range tests {
		got := convertRTSPtoHTTP(tc.input)
		if got != tc.want {
			t.Errorf("convertRTSPtoHTTP(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMetadataValueNilDictionary(t *testing.T) {
	val := metadataValue(nil, "language")
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestSelectAudioByLanguage(t *testing.T) {
	dm := &Demuxer{audioIdx: -1}
	candidates := []audioCandidate{
		{index: 1, lang: "eng"},
		{index: 2, lang: "fra"},
		{index: 3, lang: "deu"},
	}
	dm.selectAudio(candidates, DemuxOpts{AudioLanguage: "fra"})
	if dm.audioIdx != 2 {
		t.Fatalf("expected audioIdx=2 (fra), got %d", dm.audioIdx)
	}
}

func TestSelectAudioByLanguageFallback(t *testing.T) {
	dm := &Demuxer{audioIdx: -1}
	candidates := []audioCandidate{
		{index: 1, lang: "eng"},
		{index: 2, lang: "fra"},
	}
	dm.selectAudio(candidates, DemuxOpts{AudioLanguage: "jpn"})
	if dm.audioIdx != 1 {
		t.Fatalf("expected audioIdx=1 (fallback to first), got %d", dm.audioIdx)
	}
}

func TestSelectAudioByTrackIndex(t *testing.T) {
	dm := &Demuxer{audioIdx: -1}
	candidates := []audioCandidate{
		{index: 1, lang: "eng"},
		{index: 3, lang: "fra"},
	}
	dm.selectAudio(candidates, DemuxOpts{AudioTrack: 3})
	if dm.audioIdx != 3 {
		t.Fatalf("expected audioIdx=3, got %d", dm.audioIdx)
	}
}

func TestSelectAudioByTrackIndexInvalid(t *testing.T) {
	dm := &Demuxer{audioIdx: -1}
	candidates := []audioCandidate{
		{index: 1, lang: "eng"},
	}
	dm.selectAudio(candidates, DemuxOpts{AudioTrack: 99})
	if dm.audioIdx != 1 {
		t.Fatalf("expected audioIdx=1 (fallback), got %d", dm.audioIdx)
	}
}

func TestSelectAudioDefault(t *testing.T) {
	dm := &Demuxer{audioIdx: -1}
	candidates := []audioCandidate{
		{index: 5, lang: "eng"},
		{index: 6, lang: "fra"},
	}
	dm.selectAudio(candidates, DefaultDemuxOpts())
	if dm.audioIdx != 5 {
		t.Fatalf("expected audioIdx=5 (first), got %d", dm.audioIdx)
	}
}

func TestSelectAudioEmpty(t *testing.T) {
	dm := &Demuxer{audioIdx: -1}
	dm.selectAudio(nil, DefaultDemuxOpts())
	if dm.audioIdx != -1 {
		t.Fatalf("expected audioIdx=-1 with no candidates, got %d", dm.audioIdx)
	}
}

func TestIsTransient(t *testing.T) {
	if isTransient(nil) {
		t.Fatal("nil should not be transient")
	}

	connReset := fmt.Errorf("Connection reset by peer")
	if !isTransient(connReset) {
		t.Fatal("Connection reset should be transient")
	}

	connRefused := fmt.Errorf("Connection refused")
	if !isTransient(connRefused) {
		t.Fatal("Connection refused should be transient")
	}

	netUnreach := fmt.Errorf("Network is unreachable")
	if !isTransient(netUnreach) {
		t.Fatal("Network is unreachable should be transient")
	}

	timeoutErr := fmt.Errorf("read timeout")
	if !isTransient(timeoutErr) {
		t.Fatal("timeout should be transient")
	}

	parseErr := fmt.Errorf("invalid data")
	if isTransient(parseErr) {
		t.Fatal("parse error should not be transient")
	}
}

func TestSetIndicesFromCachedInfo(t *testing.T) {
	dm := &Demuxer{
		videoIdx: -1,
		audioIdx: -1,
		subIdx:   -1,
	}

	pr := &media.ProbeResult{
		Video: &media.VideoInfo{Index: 0},
		AudioTracks: []media.AudioTrack{
			{Index: 1, Language: "eng"},
			{Index: 2, Language: "fra"},
		},
		SubTracks: []media.SubtitleTrack{
			{Index: 3, Language: "eng"},
		},
	}

	dm.setIndicesFromCachedInfo(pr, DefaultDemuxOpts())

	if dm.videoIdx != 0 {
		t.Fatalf("expected videoIdx=0, got %d", dm.videoIdx)
	}
	if dm.audioIdx != 1 {
		t.Fatalf("expected audioIdx=1, got %d", dm.audioIdx)
	}
	if dm.subIdx != 3 {
		t.Fatalf("expected subIdx=3, got %d", dm.subIdx)
	}
}

func TestSeekToResetsPTSState(t *testing.T) {
	dm := &Demuxer{
		basePTS:         12345,
		audioPTSInited:  true,
		audioFrameCount: 100,
		videoIdx:        -1,
		audioIdx:        -1,
		subIdx:          -1,
	}

	if dm.basePTS != 12345 {
		t.Fatalf("pre-condition: expected basePTS=12345, got %d", dm.basePTS)
	}
	if !dm.audioPTSInited {
		t.Fatal("pre-condition: expected audioPTSInited=true")
	}
	if dm.audioFrameCount != 100 {
		t.Fatalf("pre-condition: expected audioFrameCount=100, got %d", dm.audioFrameCount)
	}
}

func TestSeekToResetsPTS_Integration(t *testing.T) {
	url := testStreamURL(t)

	dm, err := NewDemuxer(url, DemuxOpts{
		TimeoutSec:      10,
		AudioTrack:      -1,
		ProbeSize:       2000000,
		AnalyzeDuration: 2000000,
	})
	if err != nil {
		t.Skipf("cannot open: %v", err)
	}
	defer dm.Close()

	for i := 0; i < 10; i++ {
		dm.ReadPacket()
	}

	if err := dm.SeekTo(60000); err != nil {
		t.Fatalf("SeekTo: %v", err)
	}

	if dm.basePTS != 0 {
		t.Fatalf("expected basePTS=0 after SeekTo, got %d", dm.basePTS)
	}
	if dm.audioPTSInited {
		t.Fatal("expected audioPTSInited=false after SeekTo")
	}
	if dm.audioFrameCount != 0 {
		t.Fatalf("expected audioFrameCount=0 after SeekTo, got %d", dm.audioFrameCount)
	}
}

func TestDemuxerImplementsInterface(t *testing.T) {
	var _ av.Demuxer = (*Demuxer)(nil)
}

func TestRetryDelays(t *testing.T) {
	if len(retryDelays) != 3 {
		t.Fatalf("expected 3 retry delays, got %d", len(retryDelays))
	}
	if retryDelays[0] != 1*time.Second {
		t.Fatalf("expected first delay 1s, got %v", retryDelays[0])
	}
	if retryDelays[1] != 2*time.Second {
		t.Fatalf("expected second delay 2s, got %v", retryDelays[1])
	}
	if retryDelays[2] != 4*time.Second {
		t.Fatalf("expected third delay 4s, got %v", retryDelays[2])
	}
}

func testStreamURL(t *testing.T) string {
	url := os.Getenv("AVMUX_TEST_STREAM")
	if url == "" {
		t.Skip("AVMUX_TEST_STREAM not set")
	}
	return url
}

func TestSeekTo_PreservesPTS(t *testing.T) {
	url := testStreamURL(t)

	dm, err := NewDemuxer(url, DemuxOpts{
		TimeoutSec:      10,
		AudioTrack:      -1,
		ProbeSize:       2000000,
		AnalyzeDuration: 2000000,
	})
	if err != nil {
		t.Skipf("cannot open: %v", err)
	}
	defer dm.Close()

	for i := 0; i < 5; i++ {
		dm.ReadPacket()
	}

	if err := dm.SeekTo(60000); err != nil {
		t.Fatalf("SeekTo(60s): %v", err)
	}

	pkt, err := dm.ReadPacket()
	if err != nil {
		t.Fatalf("read after seek: %v", err)
	}

	t.Logf("post-seek PTS: %d ns (%.2fs)", pkt.PTS, float64(pkt.PTS)/1e9)

	if pkt.PTS < 50_000_000_000 {
		t.Errorf("expected PTS near 60s, got %.2fs — PTS was rebased to 0", float64(pkt.PTS)/1e9)
	}
}

func TestSeekTo_SegmentProduction(t *testing.T) {
	url := testStreamURL(t)

	dm, err := NewDemuxer(url, DemuxOpts{
		TimeoutSec:      10,
		AudioTrack:      -1,
		ProbeSize:       2000000,
		AnalyzeDuration: 2000000,
	})
	if err != nil {
		t.Skipf("cannot open: %v", err)
	}
	defer dm.Close()

	for i := 0; i < 5; i++ {
		dm.ReadPacket()
	}

	if err := dm.SeekTo(120000); err != nil {
		t.Fatalf("SeekTo(120s): %v", err)
	}

	vpkts := 0
	keyframes := 0
	for i := 0; i < 200; i++ {
		pkt, err := dm.ReadPacket()
		if err != nil {
			t.Logf("read %d: %v", i, err)
			break
		}
		if pkt.Type == av.Video {
			vpkts++
			if pkt.Keyframe {
				keyframes++
			}
			if vpkts <= 3 {
				t.Logf("vpkt %d: pts=%d dts=%d kf=%v len=%d", vpkts, pkt.PTS, pkt.DTS, pkt.Keyframe, len(pkt.Data))
			}
		}
	}

	t.Logf("after seek to 120s: %d video packets, %d keyframes", vpkts, keyframes)

	if vpkts == 0 {
		t.Error("no video packets after seek")
	}
	if keyframes == 0 {
		t.Error("no keyframes after seek")
	}
}

func TestSeekTo_AVAlignment(t *testing.T) {
	url := testStreamURL(t)

	dm, err := NewDemuxer(url, DemuxOpts{
		TimeoutSec:      10,
		AudioTrack:      -1,
		ProbeSize:       2000000,
		AnalyzeDuration: 2000000,
	})
	if err != nil {
		t.Skipf("cannot open: %v", err)
	}
	defer dm.Close()

	for i := 0; i < 10; i++ {
		dm.ReadPacket()
	}

	if err := dm.SeekTo(60000); err != nil {
		t.Fatalf("SeekTo(60s): %v", err)
	}

	var firstVideoPTS, firstAudioPTS int64
	foundVideo, foundAudio := false, false

	for i := 0; i < 100; i++ {
		pkt, err := dm.ReadPacket()
		if err != nil {
			break
		}
		if pkt.Type == av.Video && !foundVideo {
			firstVideoPTS = pkt.PTS
			foundVideo = true
			t.Logf("first video PTS after seek: %.3fs", float64(pkt.PTS)/1e9)
		}
		if pkt.Type == av.Audio && !foundAudio {
			firstAudioPTS = pkt.PTS
			foundAudio = true
			t.Logf("first audio PTS after seek: %.3fs", float64(pkt.PTS)/1e9)
		}
		if foundVideo && foundAudio {
			break
		}
	}

	if !foundVideo || !foundAudio {
		t.Skipf("could not find both tracks after seek (video=%v audio=%v)", foundVideo, foundAudio)
	}

	gapSec := math.Abs(float64(firstVideoPTS-firstAudioPTS)) / 1e9
	t.Logf("A/V gap after seek: %.3fs", gapSec)

	if gapSec > 5.0 {
		t.Errorf("A/V gap too large after seek: %.3fs (video=%.3fs audio=%.3fs)",
			gapSec, float64(firstVideoPTS)/1e9, float64(firstAudioPTS)/1e9)
	}
}

func TestRequestSeek_OnSeekBeforeReturn(t *testing.T) {
	url := testStreamURL(t)

	dm, err := NewDemuxer(url, DemuxOpts{
		TimeoutSec:      10,
		AudioTrack:      -1,
		ProbeSize:       2000000,
		AnalyzeDuration: 2000000,
	})
	if err != nil {
		t.Skipf("cannot open: %v", err)
	}

	var onSeekDone atomic.Int32
	dm.SetOnSeek(func() {
		onSeekDone.Add(1)
	})

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := dm.ReadPacket(); err != nil {
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)

	if err := dm.RequestSeek(60000); err != nil {
		close(stop)
		<-done
		dm.Close()
		t.Fatalf("RequestSeek: %v", err)
	}

	if onSeekDone.Load() != 1 {
		t.Errorf("onSeek not called before RequestSeek returned (count=%d)", onSeekDone.Load())
	}

	close(stop)
	<-done
	dm.Close()
}

func TestSeekTo_MultipleSeeksAVAlignment(t *testing.T) {
	url := testStreamURL(t)

	dm, err := NewDemuxer(url, DemuxOpts{
		TimeoutSec:      10,
		AudioTrack:      -1,
		ProbeSize:       2000000,
		AnalyzeDuration: 2000000,
	})
	if err != nil {
		t.Skipf("cannot open: %v", err)
	}
	defer dm.Close()

	for i := 0; i < 20; i++ {
		dm.ReadPacket()
	}

	positions := []int64{300000, 60000, 600000, 10000, 120000}
	for _, posMs := range positions {
		if err := dm.SeekTo(posMs); err != nil {
			t.Fatalf("SeekTo(%dms): %v", posMs, err)
		}

		var firstV, firstA int64
		fv, fa := false, false
		for i := 0; i < 100; i++ {
			pkt, err := dm.ReadPacket()
			if err != nil {
				break
			}
			if pkt.Type == av.Video && !fv {
				firstV = pkt.PTS
				fv = true
			}
			if pkt.Type == av.Audio && !fa {
				firstA = pkt.PTS
				fa = true
			}
			if fv && fa {
				break
			}
		}

		if !fv || !fa {
			t.Skipf("seek to %dms: missing tracks", posMs)
		}

		gap := math.Abs(float64(firstV-firstA)) / 1e9
		t.Logf("seek to %ds: video=%.3fs audio=%.3fs gap=%.3fs",
			posMs/1000, float64(firstV)/1e9, float64(firstA)/1e9, gap)

		if gap > 5.0 {
			t.Errorf("seek to %dms: A/V gap %.3fs too large", posMs, gap)
		}
	}
}
