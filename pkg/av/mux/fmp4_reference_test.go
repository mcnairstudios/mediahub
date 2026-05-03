//go:build cgo

package mux

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/output/validate"
)

type testBox struct {
	boxType string
	payload []byte
}

func testParseBoxes(data []byte) []*testBox {
	var boxes []*testBox
	offset := 0
	for offset+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		fourcc := string(data[offset+4 : offset+8])
		headerSize := 8
		if size == 1 && offset+16 <= len(data) {
			extSize := binary.BigEndian.Uint64(data[offset+8 : offset+16])
			if extSize > uint64(len(data)-offset) {
				size = len(data) - offset
			} else {
				size = int(extSize)
			}
			headerSize = 16
		}
		if size < headerSize {
			break
		}
		if size > len(data)-offset {
			size = len(data) - offset
		}
		boxes = append(boxes, &testBox{
			boxType: fourcc,
			payload: data[offset+headerSize : offset+size],
		})
		offset += size
	}
	return boxes
}

func testHasBox(boxes []*testBox, boxType string) bool {
	return testFindBox(boxes, boxType) != nil
}

func testFindBox(boxes []*testBox, boxType string) *testBox {
	for _, b := range boxes {
		if b.boxType == boxType {
			return b
		}
	}
	return nil
}

func testBoxTypeList(boxes []*testBox) []string {
	var types []string
	for _, b := range boxes {
		types = append(types, b.boxType)
	}
	return types
}

func testCountBoxes(boxes []*testBox, boxType string) int {
	count := 0
	for _, b := range boxes {
		if b.boxType == boxType {
			count++
		}
	}
	return count
}

func TestFMP4Reference_EndToEnd(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg binary not found, skipping reference test")
	}
	t.Logf("using ffmpeg at %s", ffmpegPath)

	tmpRoot := t.TempDir()
	inputPath := filepath.Join(tmpRoot, "input.ts")
	refPath := filepath.Join(tmpRoot, "ref.mp4")
	ourDir := filepath.Join(tmpRoot, "ours")

	if err := os.MkdirAll(ourDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(ffmpegPath,
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "25",
		"-c:a", "aac", "-ac", "2", "-ar", "48000",
		"-f", "mp4", "-y", inputPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate test input: %v", err)
	}

	cmd = exec.Command(ffmpegPath,
		"-i", inputPath,
		"-c:v", "copy",
		"-c:a", "copy",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-y", refPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate reference fMP4: %v", err)
	}

	refData, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("read reference fMP4: %v", err)
	}
	t.Logf("reference fMP4: %d bytes", len(refData))

	refBoxes := testParseBoxes(refData)
	t.Logf("reference top-level boxes: %v", testBoxTypeList(refBoxes))

	if !testHasBox(refBoxes, "ftyp") {
		t.Error("reference fMP4 missing ftyp box")
	}
	if !testHasBox(refBoxes, "moov") {
		t.Error("reference fMP4 missing moov box")
	}

	refMoofCount := testCountBoxes(refBoxes, "moof")
	refMdatCount := testCountBoxes(refBoxes, "mdat")
	t.Logf("reference: %d moof + %d mdat boxes", refMoofCount, refMdatCount)

	if refMoofCount == 0 {
		t.Error("reference fMP4 has no moof boxes")
	}
	if refMoofCount != refMdatCount {
		t.Errorf("reference fMP4 moof/mdat count mismatch: %d moof, %d mdat", refMoofCount, refMdatCount)
	}

	fc := astiav.AllocFormatContext()
	if fc == nil {
		t.Fatal("alloc format context")
	}
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	if err := fc.OpenInput(inputPath, nil, inputDict); err != nil {
		t.Fatalf("open input: %v", err)
	}
	defer fc.CloseInput()
	if err := fc.FindStreamInfo(nil); err != nil {
		t.Fatalf("find stream info: %v", err)
	}

	var videoStream, audioStream *astiav.Stream
	for _, s := range fc.Streams() {
		switch s.CodecParameters().MediaType() {
		case astiav.MediaTypeVideo:
			if videoStream == nil {
				videoStream = s
			}
		case astiav.MediaTypeAudio:
			if audioStream == nil {
				audioStream = s
			}
		}
	}

	if videoStream == nil {
		t.Fatal("no video stream in test input")
	}

	vcp := videoStream.CodecParameters()

	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}

	muxOpts := MuxOpts{
		OutputDir:      ourDir,
		VideoCodecID:   vcp.CodecID(),
		VideoExtradata: videoExtradata,
		VideoWidth:     vcp.Width(),
		VideoHeight:    vcp.Height(),
		VideoTimeBase:  videoStream.TimeBase(),
	}

	if audioStream != nil {
		acp := audioStream.CodecParameters()
		var audioExtradata []byte
		if ed := acp.ExtraData(); len(ed) > 0 {
			audioExtradata = make([]byte, len(ed))
			copy(audioExtradata, ed)
		}
		muxOpts.AudioCodecID = acp.CodecID()
		muxOpts.AudioExtradata = audioExtradata
		muxOpts.AudioChannels = acp.ChannelLayout().Channels()
		muxOpts.AudioSampleRate = acp.SampleRate()
	}

	fmp4Muxer, err := NewFragmentedMuxer(muxOpts)
	if err != nil {
		t.Fatalf("create fMP4 muxer: %v", err)
	}

	pkt := astiav.AllocPacket()
	if pkt == nil {
		t.Fatal("alloc packet")
	}
	defer pkt.Free()

	audioOutTB := astiav.NewRational(1, 48000)
	if audioStream != nil {
		sr := audioStream.CodecParameters().SampleRate()
		if sr > 0 {
			audioOutTB = astiav.NewRational(1, sr)
		}
	}

	var videoPkts, audioPkts int
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		switch pkt.StreamIndex() {
		case videoStream.Index():
			if err := fmp4Muxer.WriteVideoPacket(pkt); err != nil {
				t.Fatalf("write video pkt %d: %v", videoPkts, err)
			}
			videoPkts++
		default:
			if audioStream != nil && pkt.StreamIndex() == audioStream.Index() {
				pkt.RescaleTs(audioStream.TimeBase(), audioOutTB)
				if err := fmp4Muxer.WriteAudioPacket(pkt); err != nil {
					t.Fatalf("write audio pkt %d: %v", audioPkts, err)
				}
				audioPkts++
			}
		}
		pkt.Unref()
	}

	t.Logf("wrote %d video + %d audio packets through our muxer", videoPkts, audioPkts)

	if err := fmp4Muxer.Close(); err != nil {
		t.Fatalf("close muxer: %v", err)
	}

	initVideoPath := filepath.Join(ourDir, "init_video.mp4")
	initVideoData, err := os.ReadFile(initVideoPath)
	if err != nil {
		t.Fatalf("read init_video.mp4: %v", err)
	}
	t.Logf("init_video.mp4: %d bytes", len(initVideoData))

	initErrs := validate.ValidateFMP4Init(initVideoData)
	for _, e := range initErrs {
		t.Errorf("init segment validation error: %v", e)
	}

	initBoxes := testParseBoxes(initVideoData)
	t.Logf("init_video.mp4 boxes: %v", testBoxTypeList(initBoxes))

	if !testHasBox(initBoxes, "ftyp") {
		t.Error("init_video.mp4 missing ftyp box")
	}
	if !testHasBox(initBoxes, "moov") {
		t.Error("init_video.mp4 missing moov box")
	}

	videoSegments, _ := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
	if len(videoSegments) == 0 {
		t.Error("no video segments produced")
		entries, _ := os.ReadDir(ourDir)
		for _, e := range entries {
			info, _ := e.Info()
			t.Logf("  %s (%d bytes)", e.Name(), info.Size())
		}
	} else {
		t.Logf("produced %d video segments", len(videoSegments))
	}

	for _, seg := range videoSegments {
		segData, err := os.ReadFile(seg)
		if err != nil {
			t.Errorf("read segment %s: %v", filepath.Base(seg), err)
			continue
		}

		segErrs := validate.ValidateFMP4Segment(segData)
		for _, e := range segErrs {
			t.Errorf("segment %s validation error: %v", filepath.Base(seg), e)
		}

		boxes := testParseBoxes(segData)
		if !testHasBox(boxes, "moof") {
			t.Errorf("segment %s missing moof box", filepath.Base(seg))
		}
		if !testHasBox(boxes, "mdat") {
			t.Errorf("segment %s missing mdat box", filepath.Base(seg))
		}

		t.Logf("segment %s: %d bytes, boxes=%v", filepath.Base(seg), len(segData), testBoxTypeList(boxes))
	}

	if audioStream != nil {
		initAudioPath := filepath.Join(ourDir, "init_audio.mp4")
		if initAudioData, err := os.ReadFile(initAudioPath); err == nil {
			t.Logf("init_audio.mp4: %d bytes", len(initAudioData))
			audioInitErrs := validate.ValidateFMP4Init(initAudioData)
			for _, e := range audioInitErrs {
				t.Errorf("audio init validation error: %v", e)
			}
		}

		audioSegments, _ := filepath.Glob(filepath.Join(ourDir, "audio_*.m4s"))
		t.Logf("produced %d audio segments", len(audioSegments))

		for _, seg := range audioSegments {
			segData, err := os.ReadFile(seg)
			if err != nil {
				continue
			}
			segErrs := validate.ValidateFMP4Segment(segData)
			for _, e := range segErrs {
				t.Errorf("audio segment %s validation error: %v", filepath.Base(seg), e)
			}
		}
	}

	codecStr := fmp4Muxer.VideoCodecString()
	if codecStr == "" {
		t.Error("VideoCodecString() is empty")
	} else {
		t.Logf("codec string: %s", codecStr)
		if vcp.CodecID() == astiav.CodecIDH264 {
			if len(codecStr) < 5 || codecStr[:4] != "avc1" {
				t.Errorf("expected avc1.* codec string, got %s", codecStr)
			}
		}
	}
}

func TestFMP4Reference_VideoOnly(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg binary not found")
	}

	tmpRoot := t.TempDir()
	inputPath := filepath.Join(tmpRoot, "input.ts")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	cmd := exec.Command(ffmpegPath,
		"-f", "lavfi", "-i", "testsrc2=duration=3:size=320x240:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "25",
		"-an",
		"-f", "mpegts", "-y", inputPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate input: %v", err)
	}

	fc := astiav.AllocFormatContext()
	if fc == nil {
		t.Fatal("alloc format context")
	}
	defer fc.Free()

	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	if err := fc.OpenInput(inputPath, nil, inputDict); err != nil {
		t.Fatalf("open input: %v", err)
	}
	defer fc.CloseInput()
	if err := fc.FindStreamInfo(nil); err != nil {
		t.Fatalf("find stream info: %v", err)
	}

	var videoStream *astiav.Stream
	for _, s := range fc.Streams() {
		if s.CodecParameters().MediaType() == astiav.MediaTypeVideo {
			videoStream = s
			break
		}
	}
	if videoStream == nil {
		t.Fatal("no video stream")
	}

	vcp := videoStream.CodecParameters()
	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}

	fmp4Muxer, err := NewFragmentedMuxer(MuxOpts{
		OutputDir:      ourDir,
		VideoCodecID:   vcp.CodecID(),
		VideoExtradata: videoExtradata,
		VideoWidth:     vcp.Width(),
		VideoHeight:    vcp.Height(),
		VideoTimeBase:  videoStream.TimeBase(),
	})
	if err != nil {
		t.Fatalf("create muxer: %v", err)
	}

	pkt := astiav.AllocPacket()
	defer pkt.Free()

	var count int
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		if pkt.StreamIndex() == videoStream.Index() {
			if err := fmp4Muxer.WriteVideoPacket(pkt); err != nil {
				t.Fatalf("write video pkt %d: %v", count, err)
			}
			count++
		}
		pkt.Unref()
	}

	if err := fmp4Muxer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	t.Logf("wrote %d video packets", count)

	initData, err := os.ReadFile(filepath.Join(ourDir, "init_video.mp4"))
	if err != nil {
		t.Fatalf("read init: %v", err)
	}

	initErrs := validate.ValidateFMP4Init(initData)
	for _, e := range initErrs {
		t.Errorf("init validation: %v", e)
	}

	segments, _ := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
	if len(segments) == 0 {
		t.Error("no video segments produced")
	}

	for _, seg := range segments {
		segData, _ := os.ReadFile(seg)
		segErrs := validate.ValidateFMP4Segment(segData)
		for _, e := range segErrs {
			t.Errorf("segment %s: %v", filepath.Base(seg), e)
		}
	}

	t.Logf("video-only fMP4: init=%d bytes, %d segments", len(initData), len(segments))
}

func TestFMP4Reference_BoxStructureMatch(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg binary not found")
	}

	tmpRoot := t.TempDir()
	inputPath := filepath.Join(tmpRoot, "input.ts")
	refPath := filepath.Join(tmpRoot, "ref.mp4")
	ourDir := filepath.Join(tmpRoot, "ours")
	os.MkdirAll(ourDir, 0755)

	cmd := exec.Command(ffmpegPath,
		"-f", "lavfi", "-i", "testsrc2=duration=5:size=640x360:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "25",
		"-an",
		"-f", "mpegts", "-y", inputPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate input: %v", err)
	}

	cmd = exec.Command(ffmpegPath,
		"-i", inputPath,
		"-c:v", "copy",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-y", refPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate reference fMP4: %v", err)
	}

	refData, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("read ref: %v", err)
	}

	refBoxes := testParseBoxes(refData)

	refMoov := testFindBox(refBoxes, "moov")
	if refMoov == nil {
		t.Fatal("reference missing moov")
	}
	refMoovChildren := testParseBoxes(refMoov.payload)
	t.Logf("reference moov children: %v", testBoxTypeList(refMoovChildren))

	refTrak := testFindBox(refMoovChildren, "trak")
	if refTrak != nil {
		trakChildren := testParseBoxes(refTrak.payload)
		t.Logf("reference trak children: %v", testBoxTypeList(trakChildren))
	}

	fc := astiav.AllocFormatContext()
	if fc == nil {
		t.Fatal("alloc")
	}
	defer fc.Free()
	inputDict := astiav.NewDictionary()
	defer inputDict.Free()
	fc.OpenInput(inputPath, nil, inputDict)
	defer fc.CloseInput()
	fc.FindStreamInfo(nil)

	var videoStream *astiav.Stream
	for _, s := range fc.Streams() {
		if s.CodecParameters().MediaType() == astiav.MediaTypeVideo {
			videoStream = s
			break
		}
	}

	vcp := videoStream.CodecParameters()
	var videoExtradata []byte
	if ed := vcp.ExtraData(); len(ed) > 0 {
		videoExtradata = make([]byte, len(ed))
		copy(videoExtradata, ed)
	}

	fmp4Muxer, err := NewFragmentedMuxer(MuxOpts{
		OutputDir:      ourDir,
		VideoCodecID:   vcp.CodecID(),
		VideoExtradata: videoExtradata,
		VideoWidth:     vcp.Width(),
		VideoHeight:    vcp.Height(),
		VideoTimeBase:  videoStream.TimeBase(),
	})
	if err != nil {
		t.Fatal(err)
	}

	pkt := astiav.AllocPacket()
	defer pkt.Free()
	for {
		if err := fc.ReadFrame(pkt); err != nil {
			break
		}
		if pkt.StreamIndex() == videoStream.Index() {
			fmp4Muxer.WriteVideoPacket(pkt) //nolint:errcheck
		}
		pkt.Unref()
	}
	fmp4Muxer.Close()

	ourInitData, err := os.ReadFile(filepath.Join(ourDir, "init_video.mp4"))
	if err != nil {
		t.Fatal(err)
	}

	ourInitBoxes := testParseBoxes(ourInitData)
	t.Logf("our init boxes: %v", testBoxTypeList(ourInitBoxes))

	if !testHasBox(ourInitBoxes, "ftyp") {
		t.Error("our init missing ftyp (reference has ftyp)")
	}
	if !testHasBox(ourInitBoxes, "moov") {
		t.Error("our init missing moov (reference has moov)")
	}

	ourMoov := testFindBox(ourInitBoxes, "moov")
	if ourMoov != nil {
		ourMoovChildren := testParseBoxes(ourMoov.payload)
		t.Logf("our moov children: %v", testBoxTypeList(ourMoovChildren))

		requiredMoovChildren := []string{"mvhd", "trak"}
		for _, req := range requiredMoovChildren {
			if !testHasBox(ourMoovChildren, req) {
				t.Errorf("our moov missing %s box (reference has it)", req)
			}
		}

		ourTrak := testFindBox(ourMoovChildren, "trak")
		if ourTrak != nil {
			ourTrakChildren := testParseBoxes(ourTrak.payload)
			t.Logf("our trak children: %v", testBoxTypeList(ourTrakChildren))

			requiredTrakChildren := []string{"tkhd", "mdia"}
			for _, req := range requiredTrakChildren {
				if !testHasBox(ourTrakChildren, req) {
					t.Errorf("our trak missing %s box", req)
				}
			}
		}
	}

	ourSegments, _ := filepath.Glob(filepath.Join(ourDir, "video_*.m4s"))
	if len(ourSegments) == 0 {
		t.Fatal("no segments to compare")
	}

	firstSegData, _ := os.ReadFile(ourSegments[0])
	segBoxes := testParseBoxes(firstSegData)
	t.Logf("our first segment boxes: %v", testBoxTypeList(segBoxes))

	refMoof := testFindBox(refBoxes, "moof")
	if refMoof != nil {
		refMoofChildren := testParseBoxes(refMoof.payload)
		t.Logf("reference moof children: %v", testBoxTypeList(refMoofChildren))

		ourMoof := testFindBox(segBoxes, "moof")
		if ourMoof != nil {
			ourMoofChildren := testParseBoxes(ourMoof.payload)
			t.Logf("our moof children: %v", testBoxTypeList(ourMoofChildren))

			requiredMoofChildren := []string{"traf"}
			for _, req := range requiredMoofChildren {
				if !testHasBox(ourMoofChildren, req) {
					t.Errorf("our moof missing %s (reference has it)", req)
				}
			}

			refTraf := testFindBox(refMoofChildren, "traf")
			ourTraf := testFindBox(ourMoofChildren, "traf")
			if refTraf != nil && ourTraf != nil {
				refTrafChildren := testParseBoxes(refTraf.payload)
				ourTrafChildren := testParseBoxes(ourTraf.payload)
				t.Logf("reference traf children: %v", testBoxTypeList(refTrafChildren))
				t.Logf("our traf children: %v", testBoxTypeList(ourTrafChildren))

				requiredTrafChildren := []string{"tfhd", "trun"}
				for _, req := range requiredTrafChildren {
					if testHasBox(refTrafChildren, req) && !testHasBox(ourTrafChildren, req) {
						t.Errorf("our traf missing %s (reference has it)", req)
					}
				}
			}
		}
	}

	refMoofTotal := testCountBoxes(refBoxes, "moof")
	ourSegTotal := len(ourSegments)
	t.Logf("fragment count: ours=%d (segments) reference=%d (moof boxes)", ourSegTotal, refMoofTotal)
}
