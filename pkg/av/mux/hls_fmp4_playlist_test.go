//go:build cgo

package mux

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/asticode/go-astiav"
)

func parsePlaylistSegmentURIs(content string) (mapURI string, segmentURIs []string) {
	mapRe := regexp.MustCompile(`#EXT-X-MAP:URI="([^"]+)"`)
	lines := strings.Split(strings.TrimSpace(content), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		if m := mapRe.FindStringSubmatch(line); len(m) > 1 {
			mapURI = m[1]
		}

		if strings.HasPrefix(line, "#EXTINF:") && i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if next != "" && !strings.HasPrefix(next, "#") {
				segmentURIs = append(segmentURIs, next)
			}
		}
	}
	return
}

func TestHLSfMP4_PlaylistSegmentsExistOnDisk(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	tmpRoot := t.TempDir()
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInputFormat(t, tmpRoot, 10, "h264", true, "mp4")
	muxThroughOurHLS(t, inputPath, ourDir, "fmp4")

	playlistData, err := os.ReadFile(filepath.Join(ourDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read playlist: %v", err)
	}

	content := string(playlistData)
	t.Logf("playlist:\n%s", content)

	mapURI, segmentURIs := parsePlaylistSegmentURIs(content)

	if mapURI == "" {
		t.Fatal("fMP4 playlist missing #EXT-X-MAP URI")
	}
	initPath := filepath.Join(ourDir, mapURI)
	if _, err := os.Stat(initPath); os.IsNotExist(err) {
		t.Errorf("EXT-X-MAP references %q but file does not exist on disk", mapURI)
	} else {
		t.Logf("EXT-X-MAP URI %q exists on disk", mapURI)
	}

	if len(segmentURIs) == 0 {
		t.Fatal("playlist contains no segment URIs")
	}

	for i, uri := range segmentURIs {
		segPath := filepath.Join(ourDir, uri)
		if _, err := os.Stat(segPath); os.IsNotExist(err) {
			t.Errorf("segment %d: playlist references %q but file does not exist on disk", i, uri)
		}

		if !strings.HasSuffix(uri, ".m4s") {
			t.Errorf("segment %d: expected .m4s extension for fMP4 segment, got %q", i, uri)
		}
	}

	t.Logf("all %d segment URIs verified on disk", len(segmentURIs))

	entries, _ := os.ReadDir(ourDir)
	m4sFiles := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".m4s") {
			m4sFiles++
		}
	}
	if m4sFiles != len(segmentURIs) {
		t.Errorf(".m4s files on disk (%d) != segment URIs in playlist (%d)", m4sFiles, len(segmentURIs))
	}
}

func TestHLSfMP4_ReferenceComparison(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	tmpRoot := t.TempDir()
	refDir := filepath.Join(tmpRoot, "ref")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(refDir, 0755)
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInputFormat(t, tmpRoot, 10, "h265", true, "mp4")

	generateHLSReference(t, inputPath, refDir, "fmp4")
	muxThroughOurHLS(t, inputPath, ourDir, "fmp4")

	refPlaylist, err := os.ReadFile(filepath.Join(refDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read ref playlist: %v", err)
	}
	ourPlaylist, err := os.ReadFile(filepath.Join(ourDir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("read our playlist: %v", err)
	}

	t.Logf("reference playlist:\n%s", string(refPlaylist))
	t.Logf("our playlist:\n%s", string(ourPlaylist))

	refMapURI, refSegURIs := parsePlaylistSegmentURIs(string(refPlaylist))
	ourMapURI, ourSegURIs := parsePlaylistSegmentURIs(string(ourPlaylist))

	for i, uri := range refSegURIs {
		refPath := filepath.Join(refDir, uri)
		if _, err := os.Stat(refPath); os.IsNotExist(err) {
			t.Errorf("reference segment %d: %q not on disk", i, uri)
		}
		if !strings.HasSuffix(uri, ".m4s") {
			t.Errorf("reference segment %d: expected .m4s, got %q", i, uri)
		}
	}

	if refMapURI != "" {
		refInitPath := filepath.Join(refDir, refMapURI)
		if _, err := os.Stat(refInitPath); os.IsNotExist(err) {
			t.Errorf("reference EXT-X-MAP %q not on disk", refMapURI)
		}
	}

	for i, uri := range ourSegURIs {
		ourPath := filepath.Join(ourDir, uri)
		if _, err := os.Stat(ourPath); os.IsNotExist(err) {
			t.Errorf("our segment %d: playlist says %q but file missing on disk", i, uri)
		}
		if !strings.HasSuffix(uri, ".m4s") {
			t.Errorf("our segment %d: expected .m4s extension, got %q", i, uri)
		}
	}

	if ourMapURI == "" {
		t.Error("our fMP4 playlist missing #EXT-X-MAP")
	} else {
		ourInitPath := filepath.Join(ourDir, ourMapURI)
		if _, err := os.Stat(ourInitPath); os.IsNotExist(err) {
			t.Errorf("our EXT-X-MAP %q not on disk", ourMapURI)
		}
		if ourMapURI != "init.mp4" {
			t.Errorf("expected init filename 'init.mp4', got %q", ourMapURI)
		}
	}

	refVersion := extractPlaylistVersion(string(refPlaylist))
	ourVersion := extractPlaylistVersion(string(ourPlaylist))
	t.Logf("EXT-X-VERSION: ref=%d ours=%d", refVersion, ourVersion)

	if refVersion >= 7 && ourVersion < 7 {
		t.Errorf("fMP4 HLS should use version >= 7, got %d (reference uses %d)", ourVersion, refVersion)
	}

	segDiff := len(ourSegURIs) - len(refSegURIs)
	if segDiff < 0 {
		segDiff = -segDiff
	}
	if segDiff > 1 {
		t.Errorf("segment count mismatch: ours=%d ref=%d", len(ourSegURIs), len(refSegURIs))
	}
	t.Logf("segment count: ours=%d ref=%d", len(ourSegURIs), len(refSegURIs))
}

func TestHLSfMP4_SegmentFilenamePattern(t *testing.T) {
	skipIfNoFFmpegBinary(t)

	tmpRoot := t.TempDir()
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	inputPath := generateTestInputFormat(t, tmpRoot, 5, "h264", true, "mp4")
	muxThroughOurHLS(t, inputPath, ourDir, "fmp4")

	entries, err := os.ReadDir(ourDir)
	if err != nil {
		t.Fatal(err)
	}

	var m4sFiles, tsFiles []string
	hasInit := false
	hasPlaylist := false

	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".m4s"):
			m4sFiles = append(m4sFiles, name)
		case strings.HasSuffix(name, ".ts"):
			tsFiles = append(tsFiles, name)
		case name == "init.mp4":
			hasInit = true
		case name == "playlist.m3u8":
			hasPlaylist = true
		}
	}

	if !hasPlaylist {
		t.Fatal("missing playlist.m3u8")
	}
	if !hasInit {
		t.Error("missing init.mp4 for fMP4 HLS")
	}

	if len(m4sFiles) == 0 {
		t.Error("no .m4s segment files found for fMP4 HLS")
	}
	if len(tsFiles) > 0 {
		t.Errorf("found unexpected .ts files in fMP4 output: %v", tsFiles)
	}

	t.Logf("fMP4 output: init.mp4=%v, %d .m4s files, %d .ts files", hasInit, len(m4sFiles), len(tsFiles))
}

func TestHLSfMP4_FFmpegReferenceVerified(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg binary not found")
	}

	refDir := t.TempDir()

	cmd := exec.Command(ffmpegPath,
		"-f", "lavfi", "-i", "testsrc2=duration=10:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10:sample_rate=48000",
		"-c:v", "libx265", "-preset", "ultrafast",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_segment_filename", filepath.Join(refDir, "seg%d.m4s"),
		"-y", filepath.Join(refDir, "playlist.m3u8"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "Unknown encoder") {
			t.Skipf("libx265 not available: %s", string(out))
		}
		t.Fatalf("ffmpeg reference generation failed: %v\n%s", err, out)
	}

	playlistData, err := os.ReadFile(filepath.Join(refDir, "playlist.m3u8"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(playlistData)
	t.Logf("ffmpeg reference playlist:\n%s", content)

	mapURI, segURIs := parsePlaylistSegmentURIs(content)

	if mapURI == "" {
		t.Error("ffmpeg reference missing EXT-X-MAP")
	} else {
		initPath := filepath.Join(refDir, mapURI)
		if _, err := os.Stat(initPath); os.IsNotExist(err) {
			t.Errorf("ffmpeg reference EXT-X-MAP %q not found on disk", mapURI)
		}
	}

	for i, uri := range segURIs {
		segPath := filepath.Join(refDir, uri)
		if _, err := os.Stat(segPath); os.IsNotExist(err) {
			t.Errorf("ffmpeg reference segment %d: %q not found on disk", i, uri)
		}
		if !strings.HasSuffix(uri, ".m4s") {
			t.Errorf("ffmpeg reference segment %d: expected .m4s, got %q", i, uri)
		}
	}

	version := extractPlaylistVersion(content)
	if version < 7 {
		t.Logf("ffmpeg reference uses version %d (expected >= 7 for fMP4)", version)
	}

	t.Logf("ffmpeg reference verified: init=%q, %d segments, version=%d", mapURI, len(segURIs), version)
}

func TestHLSMuxer_fMP4_EncodeAndVerifyPlaylist(t *testing.T) {
	codec := astiav.FindEncoderByName("libx264")
	if codec == nil {
		t.Skip("libx264 not available")
	}

	cc := astiav.AllocCodecContext(codec)
	cc.SetWidth(320)
	cc.SetHeight(240)
	cc.SetPixelFormat(astiav.PixelFormatYuv420P)
	cc.SetTimeBase(astiav.NewRational(1, 25))
	cc.SetFramerate(astiav.NewRational(25, 1))
	cc.SetGopSize(25)
	cc.SetFlags(astiav.NewCodecContextFlags(astiav.CodecContextFlagGlobalHeader))
	if err := cc.Open(codec, nil); err != nil {
		cc.Free()
		t.Fatalf("open encoder: %v", err)
	}
	defer cc.Free()

	dir := t.TempDir()

	m, err := NewHLSMuxer(HLSMuxOpts{
		OutputDir:          dir,
		SegmentDurationSec: 2,
		SegmentType:        "fmp4",
		VideoCodecID:       astiav.CodecIDH264,
		VideoExtradata:     cc.ExtraData(),
		VideoWidth:         320,
		VideoHeight:        240,
		VideoTimeBase:      astiav.NewRational(1, 90000),
		VideoFrameRate:     25,
	})
	if err != nil {
		t.Fatal(err)
	}

	frame := astiav.AllocFrame()
	defer frame.Free()
	frame.SetWidth(320)
	frame.SetHeight(240)
	frame.SetPixelFormat(astiav.PixelFormatYuv420P)
	frame.AllocBuffer(0)

	outTB := astiav.NewRational(1, 90000)
	var totalPackets int
	for i := 0; i < 150; i++ {
		frame.SetPts(int64(i))
		if err := cc.SendFrame(frame); err != nil {
			continue
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
			if err := m.WriteVideoPacket(pkt); err != nil {
				pkt.Free()
				t.Fatalf("write video pkt %d: %v", totalPackets, err)
			}
			pkt.Free()
			totalPackets++
		}
	}

	cc.SendFrame(nil)
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
		m.WriteVideoPacket(pkt)
		pkt.Free()
		totalPackets++
	}

	if err := m.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	playlistData, err := os.ReadFile(filepath.Join(dir, "playlist.m3u8"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(playlistData)
	t.Logf("fMP4 playlist:\n%s", content)

	mapURI, segURIs := parsePlaylistSegmentURIs(content)

	if mapURI == "" {
		t.Fatal("fMP4 playlist missing EXT-X-MAP")
	}
	initPath := filepath.Join(dir, mapURI)
	if _, err := os.Stat(initPath); os.IsNotExist(err) {
		t.Errorf("EXT-X-MAP references %q but file does not exist", mapURI)
	}

	if len(segURIs) == 0 {
		t.Fatal("no segments in playlist")
	}

	for i, uri := range segURIs {
		segPath := filepath.Join(dir, uri)
		if _, err := os.Stat(segPath); os.IsNotExist(err) {
			t.Errorf("CRITICAL: segment %d references %q in playlist but file DOES NOT EXIST on disk", i, uri)
		}
		if !strings.HasSuffix(uri, ".m4s") {
			t.Errorf("CRITICAL: segment %d has extension %q, expected .m4s for fMP4", i, filepath.Ext(uri))
		}
	}

	entries, _ := os.ReadDir(dir)
	t.Log("directory contents:")
	for _, e := range entries {
		info, _ := e.Info()
		t.Logf("  %s (%d bytes)", e.Name(), info.Size())
	}

	if !strings.Contains(content, "#EXT-X-MAP:") {
		t.Error("missing #EXT-X-MAP tag")
	}
	if !strings.Contains(content, "#EXT-X-ENDLIST") {
		t.Error("missing #EXT-X-ENDLIST")
	}
	if !strings.Contains(content, "#EXTM3U") {
		t.Error("missing #EXTM3U header")
	}
}

func extractPlaylistVersion(content string) int {
	re := regexp.MustCompile(`#EXT-X-VERSION:(\d+)`)
	m := re.FindStringSubmatch(content)
	if len(m) > 1 {
		v, _ := strings.CutPrefix(m[1], "")
		ver := 0
		for _, c := range v {
			ver = ver*10 + int(c-'0')
		}
		return ver
	}
	return 0
}
