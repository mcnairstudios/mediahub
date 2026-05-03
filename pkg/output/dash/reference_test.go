package dash

import (
	"encoding/binary"
	"encoding/xml"
	"math"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/output/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const referenceMPD = `<?xml version="1.0" encoding="utf-8"?>
<MPD xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
	xmlns="urn:mpeg:dash:schema:mpd:2011"
	xmlns:xlink="http://www.w3.org/1999/xlink"
	xsi:schemaLocation="urn:mpeg:DASH:schema:MPD:2011 http://standards.iso.org/ittf/PubliclyAvailableStandards/MPEG-DASH_schema_files/DASH-MPD.xsd"
	profiles="urn:mpeg:dash:profile:isoff-live:2011"
	type="static"
	mediaPresentationDuration="PT10.0S"
	maxSegmentDuration="PT2.0S"
	minBufferTime="PT4.0S">
	<ProgramInformation>
	</ProgramInformation>
	<ServiceDescription id="0">
	</ServiceDescription>
	<Period id="0" start="PT0.0S">
		<AdaptationSet id="0" contentType="video" startWithSAP="1" segmentAlignment="true" bitstreamSwitching="true" frameRate="25/1" maxWidth="640" maxHeight="360" par="16:9">
			<Representation id="0" mimeType="video/mp4" codecs="avc1.42c01e" bandwidth="1825049" width="640" height="360" sar="1:1">
				<SegmentTemplate timescale="12800" initialization="init-$RepresentationID$.m4s" media="chunk-$RepresentationID$-$Number%05d$.m4s" startNumber="1">
					<SegmentTimeline>
						<S t="0" d="25600" r="4" />
					</SegmentTimeline>
				</SegmentTemplate>
			</Representation>
		</AdaptationSet>
		<AdaptationSet id="1" contentType="audio" startWithSAP="1" segmentAlignment="true" bitstreamSwitching="true">
			<Representation id="1" mimeType="audio/mp4" codecs="mp4a.40.2" bandwidth="128000" audioSamplingRate="48000">
				<AudioChannelConfiguration schemeIdUri="urn:mpeg:dash:23003:3:audio_channel_configuration:2011" value="2" />
				<SegmentTemplate timescale="48000" initialization="init-$RepresentationID$.m4s" media="chunk-$RepresentationID$-$Number%05d$.m4s" startNumber="1">
					<SegmentTimeline>
						<S t="0" d="95232" />
						<S d="96256" r="2" />
						<S d="96000" />
					</SegmentTimeline>
				</SegmentTemplate>
			</Representation>
		</AdaptationSet>
	</Period>
</MPD>`

func skipIfNoFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
}

func skipIfNoFFprobe(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

func generateDASHReference(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=10:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline",
		"-g", "50", "-keyint_min", "50",
		"-c:a", "aac", "-ac", "2", "-ar", "48000", "-b:a", "128k",
		"-f", "dash", "-seg_duration", "2",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		filepath.Join(dir, "manifest.mpd"),
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "ffmpeg DASH generation failed: %s", out)
}

type refMPD struct {
	XMLName                   xml.Name       `xml:"MPD"`
	Namespace                 string         `xml:"xmlns,attr"`
	Profiles                  string         `xml:"profiles,attr"`
	Type                      string         `xml:"type,attr"`
	MediaPresentationDuration string         `xml:"mediaPresentationDuration,attr"`
	MinBufferTime             string         `xml:"minBufferTime,attr"`
	Periods                   []refPeriod    `xml:"Period"`
}

type refPeriod struct {
	ID             string              `xml:"id,attr"`
	Start          string              `xml:"start,attr"`
	AdaptationSets []refAdaptationSet  `xml:"AdaptationSet"`
}

type refAdaptationSet struct {
	ID               string              `xml:"id,attr"`
	ContentType      string              `xml:"contentType,attr"`
	MimeType         string              `xml:"mimeType,attr"`
	StartWithSAP     string              `xml:"startWithSAP,attr"`
	SegmentAlignment string              `xml:"segmentAlignment,attr"`
	Representations  []refRepresentation `xml:"Representation"`
}

type refRepresentation struct {
	ID                string                  `xml:"id,attr"`
	MimeType          string                  `xml:"mimeType,attr"`
	Codecs            string                  `xml:"codecs,attr"`
	Bandwidth         int                     `xml:"bandwidth,attr"`
	Width             int                     `xml:"width,attr"`
	Height            int                     `xml:"height,attr"`
	AudioSamplingRate int                     `xml:"audioSamplingRate,attr"`
	SegmentTemplate   *refSegmentTemplate     `xml:"SegmentTemplate"`
	AudioChannelCfg   *refAudioChannelCfg     `xml:"AudioChannelConfiguration"`
}

type refSegmentTemplate struct {
	Timescale      int    `xml:"timescale,attr"`
	Initialization string `xml:"initialization,attr"`
	Media          string `xml:"media,attr"`
	StartNumber    int    `xml:"startNumber,attr"`
}

type refAudioChannelCfg struct {
	SchemeIdUri string `xml:"schemeIdUri,attr"`
	Value       string `xml:"value,attr"`
}

func TestReference_MPD_Namespace(t *testing.T) {
	var doc refMPD
	require.NoError(t, xml.Unmarshal([]byte(referenceMPD), &doc))
	assert.Equal(t, "urn:mpeg:dash:schema:mpd:2011", doc.XMLName.Space)
}

func TestReference_MPD_TypeAndDuration(t *testing.T) {
	var doc refMPD
	require.NoError(t, xml.Unmarshal([]byte(referenceMPD), &doc))
	assert.Equal(t, "static", doc.Type)
	assert.Equal(t, "PT10.0S", doc.MediaPresentationDuration)
}

func TestReference_MPD_Profile(t *testing.T) {
	var doc refMPD
	require.NoError(t, xml.Unmarshal([]byte(referenceMPD), &doc))
	assert.Contains(t, doc.Profiles, "urn:mpeg:dash:profile:isoff-live:2011")
}

func TestReference_MPD_HasPeriod(t *testing.T) {
	var doc refMPD
	require.NoError(t, xml.Unmarshal([]byte(referenceMPD), &doc))
	require.Len(t, doc.Periods, 1)
	assert.Equal(t, "0", doc.Periods[0].ID)
	assert.Equal(t, "PT0.0S", doc.Periods[0].Start)
}

func TestReference_MPD_VideoAdaptationSet(t *testing.T) {
	var doc refMPD
	require.NoError(t, xml.Unmarshal([]byte(referenceMPD), &doc))

	period := doc.Periods[0]
	require.GreaterOrEqual(t, len(period.AdaptationSets), 2)

	videoAS := period.AdaptationSets[0]
	assert.Equal(t, "video", videoAS.ContentType)
	assert.Equal(t, "1", videoAS.StartWithSAP)
	assert.Equal(t, "true", videoAS.SegmentAlignment)
	require.Len(t, videoAS.Representations, 1)

	videoRep := videoAS.Representations[0]
	assert.Equal(t, "video/mp4", videoRep.MimeType)
	assert.True(t, strings.HasPrefix(videoRep.Codecs, "avc1."), "video codecs should start with avc1.")
	assert.Equal(t, 640, videoRep.Width)
	assert.Equal(t, 360, videoRep.Height)
	assert.Greater(t, videoRep.Bandwidth, 0)
}

func TestReference_MPD_AudioAdaptationSet(t *testing.T) {
	var doc refMPD
	require.NoError(t, xml.Unmarshal([]byte(referenceMPD), &doc))

	period := doc.Periods[0]
	require.GreaterOrEqual(t, len(period.AdaptationSets), 2)

	audioAS := period.AdaptationSets[1]
	assert.Equal(t, "audio", audioAS.ContentType)
	assert.Equal(t, "1", audioAS.StartWithSAP)
	require.Len(t, audioAS.Representations, 1)

	audioRep := audioAS.Representations[0]
	assert.Equal(t, "audio/mp4", audioRep.MimeType)
	assert.Equal(t, "mp4a.40.2", audioRep.Codecs)
	assert.Equal(t, 48000, audioRep.AudioSamplingRate)
	assert.Equal(t, 128000, audioRep.Bandwidth)
	require.NotNil(t, audioRep.AudioChannelCfg)
	assert.Equal(t, "2", audioRep.AudioChannelCfg.Value)
	assert.Equal(t, "urn:mpeg:dash:23003:3:audio_channel_configuration:2011", audioRep.AudioChannelCfg.SchemeIdUri)
}

func TestReference_MPD_VideoSegmentTemplate(t *testing.T) {
	var doc refMPD
	require.NoError(t, xml.Unmarshal([]byte(referenceMPD), &doc))

	videoRep := doc.Periods[0].AdaptationSets[0].Representations[0]
	require.NotNil(t, videoRep.SegmentTemplate)

	tmpl := videoRep.SegmentTemplate
	assert.Equal(t, 12800, tmpl.Timescale)
	assert.Equal(t, 1, tmpl.StartNumber)
	assert.NotEmpty(t, tmpl.Initialization)
	assert.NotEmpty(t, tmpl.Media)
}

func TestReference_MPD_AudioSegmentTemplate(t *testing.T) {
	var doc refMPD
	require.NoError(t, xml.Unmarshal([]byte(referenceMPD), &doc))

	audioRep := doc.Periods[0].AdaptationSets[1].Representations[0]
	require.NotNil(t, audioRep.SegmentTemplate)

	tmpl := audioRep.SegmentTemplate
	assert.Equal(t, 48000, tmpl.Timescale)
	assert.Equal(t, 1, tmpl.StartNumber)
}

func TestReference_MPD_PassesValidator(t *testing.T) {
	errs := validate.ValidateMPD([]byte(referenceMPD))
	assert.Empty(t, errs, "reference MPD should pass validator: %v", errs)
}

func TestReference_Generated_MPDStructure(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	mpdData, err := os.ReadFile(filepath.Join(dir, "manifest.mpd"))
	require.NoError(t, err)

	var doc refMPD
	require.NoError(t, xml.Unmarshal(mpdData, &doc))

	assert.Equal(t, "urn:mpeg:dash:schema:mpd:2011", doc.XMLName.Space)
	assert.Equal(t, "static", doc.Type)
	assert.Equal(t, "PT10.0S", doc.MediaPresentationDuration)
	require.Len(t, doc.Periods, 1)
	require.GreaterOrEqual(t, len(doc.Periods[0].AdaptationSets), 2)

	videoAS := doc.Periods[0].AdaptationSets[0]
	assert.Equal(t, "video", videoAS.ContentType)
	videoRep := videoAS.Representations[0]
	assert.Equal(t, "video/mp4", videoRep.MimeType)
	assert.True(t, strings.HasPrefix(videoRep.Codecs, "avc1."))
	assert.Equal(t, 640, videoRep.Width)
	assert.Equal(t, 360, videoRep.Height)

	audioAS := doc.Periods[0].AdaptationSets[1]
	assert.Equal(t, "audio", audioAS.ContentType)
	audioRep := audioAS.Representations[0]
	assert.Equal(t, "audio/mp4", audioRep.MimeType)
	assert.Equal(t, "mp4a.40.2", audioRep.Codecs)
	assert.Equal(t, 48000, audioRep.AudioSamplingRate)
}

func TestReference_Generated_MPDPassesValidator(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	mpdData, err := os.ReadFile(filepath.Join(dir, "manifest.mpd"))
	require.NoError(t, err)

	errs := validate.ValidateMPD(mpdData)
	assert.Empty(t, errs, "generated MPD should pass validator: %v", errs)
}

func TestReference_Generated_VideoInitHasFtypAndMoov(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	data, err := os.ReadFile(filepath.Join(dir, "init-0.m4s"))
	require.NoError(t, err)
	require.Greater(t, len(data), 0, "video init should not be empty")

	errs := validate.ValidateFMP4Init(data)
	assert.Empty(t, errs, "video init should pass fMP4 init validator: %v", errs)

	assert.Equal(t, "ftyp", string(data[4:8]), "first box should be ftyp")
	majorBrand := string(data[8:12])
	assert.Contains(t, []string{"isom", "iso5", "iso6", "mp41", "mp42", "avc1", "dash"}, majorBrand)
}

func TestReference_Generated_AudioInitHasFtypAndMoov(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	data, err := os.ReadFile(filepath.Join(dir, "init-1.m4s"))
	require.NoError(t, err)
	require.Greater(t, len(data), 0, "audio init should not be empty")

	errs := validate.ValidateFMP4Init(data)
	assert.Empty(t, errs, "audio init should pass fMP4 init validator: %v", errs)

	assert.Equal(t, "ftyp", string(data[4:8]), "first box should be ftyp")
}

func TestReference_Generated_VideoInitHasAvc1Codec(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	data, err := os.ReadFile(filepath.Join(dir, "init-0.m4s"))
	require.NoError(t, err)

	assert.True(t, containsBoxType(data, "avc1"), "video init should contain avc1 codec box")
	assert.True(t, containsBoxType(data, "avcC"), "video init should contain avcC config box")
}

func TestReference_Generated_AudioInitHasMp4aCodec(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	data, err := os.ReadFile(filepath.Join(dir, "init-1.m4s"))
	require.NoError(t, err)

	assert.True(t, containsBoxType(data, "mp4a"), "audio init should contain mp4a codec box")
	assert.True(t, containsBoxType(data, "esds"), "audio init should contain esds config box")
}

func TestReference_Generated_VideoSegmentHasMoofAndMdat(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var videoSegPaths []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chunk-0-") {
			videoSegPaths = append(videoSegPaths, filepath.Join(dir, e.Name()))
		}
	}
	require.GreaterOrEqual(t, len(videoSegPaths), 1, "should have at least 1 video segment")

	for _, segPath := range videoSegPaths {
		data, err := os.ReadFile(segPath)
		require.NoError(t, err)
		assert.True(t, containsBoxType(data, "moof"), "%s should contain moof box", filepath.Base(segPath))
		assert.True(t, containsBoxType(data, "mdat"), "%s should contain mdat box", filepath.Base(segPath))
	}
}

func TestReference_Generated_AudioSegmentHasMoofAndMdat(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var audioSegPaths []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chunk-1-") {
			audioSegPaths = append(audioSegPaths, filepath.Join(dir, e.Name()))
		}
	}
	require.GreaterOrEqual(t, len(audioSegPaths), 1, "should have at least 1 audio segment")

	for _, segPath := range audioSegPaths {
		data, err := os.ReadFile(segPath)
		require.NoError(t, err)
		assert.True(t, containsBoxType(data, "moof"), "%s should contain moof box", filepath.Base(segPath))
		assert.True(t, containsBoxType(data, "mdat"), "%s should contain mdat box", filepath.Base(segPath))
	}
}

func TestReference_Generated_VideoSegmentPassesValidator(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "chunk-0-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)

		segData := extractFirstMoofMdat(data)
		if segData == nil {
			continue
		}
		errs := validate.ValidateFMP4Segment(segData)
		assert.Empty(t, errs, "video segment %s should pass validator: %v", e.Name(), errs)
	}
}

func TestReference_Generated_AudioSegmentPassesValidator(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "chunk-1-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)

		segData := extractFirstMoofMdat(data)
		if segData == nil {
			continue
		}
		errs := validate.ValidateFMP4Segment(segData)
		assert.Empty(t, errs, "audio segment %s should pass validator: %v", e.Name(), errs)
	}
}

func TestReference_Generated_SegmentCount(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var videoSegs, audioSegs int
	for _, e := range entries {
		switch {
		case strings.HasPrefix(e.Name(), "chunk-0-"):
			videoSegs++
		case strings.HasPrefix(e.Name(), "chunk-1-"):
			audioSegs++
		}
	}

	assert.GreaterOrEqual(t, videoSegs, 4, "10s at 2s segments should produce >= 4 video segments")
	assert.LessOrEqual(t, videoSegs, 6, "10s at 2s segments should produce <= 6 video segments")
	assert.GreaterOrEqual(t, audioSegs, 4, "10s at 2s segments should produce >= 4 audio segments")
	assert.LessOrEqual(t, audioSegs, 6, "10s at 2s segments should produce <= 6 audio segments")
}

func TestReference_Generated_SegmentSizeReasonable(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var videoSizes []int64
	var audioSizes []int64
	for _, e := range entries {
		info, err := e.Info()
		require.NoError(t, err)
		switch {
		case strings.HasPrefix(e.Name(), "chunk-0-"):
			videoSizes = append(videoSizes, info.Size())
		case strings.HasPrefix(e.Name(), "chunk-1-"):
			audioSizes = append(audioSizes, info.Size())
		}
	}

	for _, s := range videoSizes {
		assert.Greater(t, s, int64(1000), "video segment should be > 1KB")
		assert.Less(t, s, int64(10*1024*1024), "video segment should be < 10MB")
	}

	for _, s := range audioSizes {
		assert.Greater(t, s, int64(100), "audio segment should be > 100B")
		assert.Less(t, s, int64(1024*1024), "audio segment should be < 1MB")
	}

	if len(videoSizes) > 1 {
		var totalVideo int64
		for _, s := range videoSizes {
			totalVideo += s
		}
		avgVideo := float64(totalVideo) / float64(len(videoSizes))
		for _, s := range videoSizes {
			ratio := float64(s) / avgVideo
			assert.Greater(t, ratio, 0.2, "video segment size %d too small vs avg %.0f", s, avgVideo)
			assert.Less(t, ratio, 5.0, "video segment size %d too large vs avg %.0f", s, avgVideo)
		}
	}
}

func TestReference_Generated_InitSegmentSizes(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	videoInit, err := os.Stat(filepath.Join(dir, "init-0.m4s"))
	require.NoError(t, err)
	audioInit, err := os.Stat(filepath.Join(dir, "init-1.m4s"))
	require.NoError(t, err)

	assert.Greater(t, videoInit.Size(), int64(100), "video init should be > 100B")
	assert.Less(t, videoInit.Size(), int64(10000), "video init should be < 10KB")
	assert.Greater(t, audioInit.Size(), int64(100), "audio init should be > 100B")
	assert.Less(t, audioInit.Size(), int64(10000), "audio init should be < 10KB")
}

func TestReference_Generated_SegmentTimingFromMPD(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	mpdData, err := os.ReadFile(filepath.Join(dir, "manifest.mpd"))
	require.NoError(t, err)

	text := string(mpdData)
	assert.Equal(t, "PT10.0S", extractAttr(text, "mediaPresentationDuration"))

	assert.Contains(t, text, "SegmentTimeline")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var videoSegCount, audioSegCount int
	for _, e := range entries {
		switch {
		case strings.HasPrefix(e.Name(), "chunk-0-"):
			videoSegCount++
		case strings.HasPrefix(e.Name(), "chunk-1-"):
			audioSegCount++
		}
	}

	assert.GreaterOrEqual(t, videoSegCount, 4, "should have >= 4 video segments for 10s at 2s")
	assert.GreaterOrEqual(t, audioSegCount, 4, "should have >= 4 audio segments for 10s at 2s")
}

func TestReference_Generated_TotalDurationSumsCorrectly(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	mpdData, err := os.ReadFile(filepath.Join(dir, "manifest.mpd"))
	require.NoError(t, err)

	text := string(mpdData)

	var videoTimescale int
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "contentType=\"video\"") || strings.Contains(line, "id=\"0\" contentType=\"video\"") {
			continue
		}
		if strings.Contains(line, "timescale=") && strings.Contains(line, "chunk-$RepresentationID$") {
			for _, part := range strings.Fields(line) {
				if strings.HasPrefix(part, "timescale=\"") {
					val := strings.TrimPrefix(part, "timescale=\"")
					val = strings.TrimSuffix(val, "\"")
					ts, err := strconv.Atoi(val)
					if err == nil && videoTimescale == 0 {
						videoTimescale = ts
					}
				}
			}
		}
	}

	if videoTimescale > 0 {
		assert.Greater(t, videoTimescale, 0, "video timescale should be positive")
	}
}

func TestReference_PluginMPD_MatchesReferenceStructure(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video: &media.VideoInfo{
			Codec:      "h264",
			Width:      640,
			Height:     360,
			FramerateN: 25,
			FramerateD: 1,
		},
		Audio: &media.AudioTrack{
			Codec:      "aac",
			SampleRate: 48000,
			Channels:   2,
			BitRate:    128000,
		},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)

	body := rec.Body.String()

	assert.Contains(t, body, "urn:mpeg:dash:schema:mpd:2011")
	assert.Contains(t, body, `mimeType="video/mp4"`)
	assert.Contains(t, body, `mimeType="audio/mp4"`)
	assert.Contains(t, body, `width="640"`)
	assert.Contains(t, body, `height="360"`)
	assert.Contains(t, body, `audioSamplingRate="48000"`)
	assert.Contains(t, body, `codecs="mp4a.40.2"`)
	assert.Contains(t, body, `startWithSAP="1"`)
	assert.Contains(t, body, `segmentAlignment="true"`)
}

func TestReference_PluginMPD_HasCorrectTimescales(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
		Audio:     &media.AudioTrack{Codec: "aac", SampleRate: 48000, Channels: 2},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()

	assert.Contains(t, body, `timescale="90000"`, "video timescale should be 90000")
	assert.Contains(t, body, `timescale="48000"`, "audio timescale should be 48000")
}

func TestReference_PluginMPD_SegmentDurationMath(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
		Audio:     &media.AudioTrack{Codec: "aac", SampleRate: 48000, Channels: 2},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()

	assert.Contains(t, body, `duration="540000"`, "video duration should be segDur(6)*timescale(90000)=540000")
	assert.Contains(t, body, `duration="288000"`, "audio duration should be segDur(6)*timescale(48000)=288000")
}

func TestReference_PluginMPD_PassesValidator(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
		Audio:     &media.AudioTrack{Codec: "aac", SampleRate: 48000, Channels: 2},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	errs := validate.ValidateMPD(rec.Body.Bytes())
	assert.Empty(t, errs, "plugin MPD should pass validator: %v", errs)
}

func TestReference_PluginMPD_LiveIsDynamic(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `type="dynamic"`)
	assert.Contains(t, body, "minimumUpdatePeriod")
	assert.Contains(t, body, "availabilityStartTime")
}

func TestReference_PluginMPD_VODIsStatic(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    false,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	p.EndOfStream()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `type="static"`)
	assert.Contains(t, body, "mediaPresentationDuration")
}

func TestReference_PluginMPD_VideoOnlyNoAudioAS(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `mimeType="video/mp4"`)
	assert.NotContains(t, body, `mimeType="audio/mp4"`)
}

func TestReference_PluginInitVideo_PassesValidator(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	initData := validate.BuildFMP4InitForTest("avc1", true)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "init_video.mp4"), initData, 0644))
	time.Sleep(200 * time.Millisecond)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/init-video.mp4", nil)
	p.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	errs := validate.ValidateFMP4Init(rec.Body.Bytes())
	assert.Empty(t, errs, "plugin video init should pass validator: %v", errs)
}

func TestReference_PluginInitAudio_PassesValidator(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
		Audio:     &media.AudioTrack{Codec: "aac", SampleRate: 48000, Channels: 2},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	initData := validate.BuildFMP4InitForTest("mp4a", true)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "init_audio.mp4"), initData, 0644))
	time.Sleep(200 * time.Millisecond)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/init-audio.mp4", nil)
	p.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	errs := validate.ValidateFMP4Init(rec.Body.Bytes())
	assert.Empty(t, errs, "plugin audio init should pass validator: %v", errs)
}

func TestReference_PluginMediaSegment_PassesValidator(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	segDir := filepath.Join(cfg.OutputDir, "segments")
	segData := validate.BuildFMP4SegmentForTest(30)
	require.NoError(t, os.WriteFile(filepath.Join(segDir, "video_00001.m4s"), segData, 0644))
	time.Sleep(200 * time.Millisecond)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/video/1.m4s", nil)
	p.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	errs := validate.ValidateFMP4Segment(rec.Body.Bytes())
	assert.Empty(t, errs, "plugin media segment should pass validator: %v", errs)
}

func TestReference_Generated_VideoSegmentTimingFromTimeline(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	mpdData, err := os.ReadFile(filepath.Join(dir, "manifest.mpd"))
	require.NoError(t, err)
	text := string(mpdData)

	videoTimescaleStr := extractTimescaleForAdaptationSet(text, "video")
	require.NotEmpty(t, videoTimescaleStr, "should find video timescale in MPD")
	videoTimescale, err := strconv.Atoi(videoTimescaleStr)
	require.NoError(t, err)
	assert.Greater(t, videoTimescale, 0)

	videoDurations := extractTimelineDurations(text, "video")
	require.GreaterOrEqual(t, len(videoDurations), 1, "should have at least one S element in video timeline")

	targetDurTicks := float64(videoTimescale) * 2.0
	for _, d := range videoDurations {
		deviation := math.Abs(float64(d)-targetDurTicks) / targetDurTicks
		actualSec := float64(d) / float64(videoTimescale)
		assert.Less(t, deviation, 0.15,
			"video timeline duration %d (%.3fs) deviates >15%% from target %.1fs", d, actualSec, 2.0)
	}
}

func TestReference_Generated_AudioSegmentTimingFromTimeline(t *testing.T) {
	skipIfNoFFmpeg(t)
	dir := t.TempDir()
	generateDASHReference(t, dir)

	mpdData, err := os.ReadFile(filepath.Join(dir, "manifest.mpd"))
	require.NoError(t, err)
	text := string(mpdData)

	audioTimescaleStr := extractTimescaleForAdaptationSet(text, "audio")
	require.NotEmpty(t, audioTimescaleStr, "should find audio timescale in MPD")
	audioTimescale, err := strconv.Atoi(audioTimescaleStr)
	require.NoError(t, err)
	assert.Equal(t, 48000, audioTimescale)

	audioDurations := extractTimelineDurations(text, "audio")
	require.GreaterOrEqual(t, len(audioDurations), 1, "should have at least one S element in audio timeline")

	targetDurTicks := float64(audioTimescale) * 2.0
	for _, d := range audioDurations {
		deviation := math.Abs(float64(d)-targetDurTicks) / targetDurTicks
		actualSec := float64(d) / float64(audioTimescale)
		assert.Less(t, deviation, 0.15,
			"audio timeline duration %d (%.3fs) deviates >15%% from target %.1fs", d, actualSec, 2.0)
	}
}

func TestReference_PluginMPD_AudioChannelConfiguration(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
		Audio:     &media.AudioTrack{Codec: "aac", SampleRate: 48000, Channels: 2, BitRate: 128000},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "AudioChannelConfiguration")
	assert.Contains(t, body, "urn:mpeg:dash:23003:3:audio_channel_configuration:2011")
	assert.Contains(t, body, `value="2"`)
}

func TestReference_PluginMPD_SegmentTemplatePatterns(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
		Audio:     &media.AudioTrack{Codec: "aac", SampleRate: 48000, Channels: 2},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `media="video/$Number$.m4s"`)
	assert.Contains(t, body, `initialization="init-video.mp4"`)
	assert.Contains(t, body, `media="audio/$Number$.m4s"`)
	assert.Contains(t, body, `initialization="init-audio.mp4"`)
	assert.Contains(t, body, `startNumber="1"`)
}

func TestReference_PluginMPD_BandwidthValues(t *testing.T) {
	cfg := output.PluginConfig{
		OutputDir: t.TempDir(),
		IsLive:    true,
		Video:     &media.VideoInfo{Codec: "h264", Width: 640, Height: 360},
		Audio:     &media.AudioTrack{Codec: "aac", SampleRate: 48000, Channels: 2, BitRate: 128000},
	}

	p, err := New(cfg)
	require.NoError(t, err)
	defer p.Stop()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/manifest.mpd", nil)
	p.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, `bandwidth="128000"`, "audio bandwidth should match audio bitrate")
	assert.Contains(t, body, `bandwidth="2000000"`, "video bandwidth should be default 2mbps")
}

func extractAttr(xmlText, attrName string) string {
	prefix := attrName + `="`
	idx := strings.Index(xmlText, prefix)
	if idx < 0 {
		return ""
	}
	start := idx + len(prefix)
	end := strings.Index(xmlText[start:], `"`)
	if end < 0 {
		return ""
	}
	return xmlText[start : start+end]
}

func extractTimescaleForAdaptationSet(mpdText, contentType string) string {
	marker := `contentType="` + contentType + `"`
	idx := strings.Index(mpdText, marker)
	if idx < 0 {
		return ""
	}
	rest := mpdText[idx:]
	endAS := strings.Index(rest, "</AdaptationSet>")
	if endAS < 0 {
		return ""
	}
	asBlock := rest[:endAS]
	tsPrefix := `timescale="`
	tsIdx := strings.Index(asBlock, tsPrefix)
	if tsIdx < 0 {
		return ""
	}
	start := tsIdx + len(tsPrefix)
	end := strings.Index(asBlock[start:], `"`)
	if end < 0 {
		return ""
	}
	return asBlock[start : start+end]
}

func extractTimelineDurations(mpdText, contentType string) []int64 {
	marker := `contentType="` + contentType + `"`
	idx := strings.Index(mpdText, marker)
	if idx < 0 {
		return nil
	}
	rest := mpdText[idx:]
	endAS := strings.Index(rest, "</AdaptationSet>")
	if endAS < 0 {
		return nil
	}
	asBlock := rest[:endAS]

	var durations []int64
	remaining := asBlock
	for {
		sIdx := strings.Index(remaining, "<S ")
		if sIdx < 0 {
			break
		}
		sEnd := strings.Index(remaining[sIdx:], "/>")
		if sEnd < 0 {
			break
		}
		sElem := remaining[sIdx : sIdx+sEnd+2]
		remaining = remaining[sIdx+sEnd+2:]

		dVal := extractAttr(sElem, "d")
		if dVal == "" {
			continue
		}
		d, err := strconv.ParseInt(dVal, 10, 64)
		if err != nil {
			continue
		}

		rVal := extractAttr(sElem, "r")
		repeat := 0
		if rVal != "" {
			r, err := strconv.Atoi(rVal)
			if err == nil {
				repeat = r
			}
		}

		for i := 0; i <= repeat; i++ {
			durations = append(durations, d)
		}
	}
	return durations
}

func containsBoxType(data []byte, boxType string) bool {
	needle := []byte(boxType)
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == needle[0] && data[i+1] == needle[1] && data[i+2] == needle[2] && data[i+3] == needle[3] {
			return true
		}
	}
	return false
}

func extractFirstMoofMdat(data []byte) []byte {
	moofStart := -1
	mdatEnd := -1
	pos := 0
	for pos+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		if size < 8 || pos+size > len(data) {
			break
		}
		boxType := string(data[pos+4 : pos+8])
		if boxType == "moof" && moofStart == -1 {
			moofStart = pos
		}
		if boxType == "mdat" && moofStart >= 0 {
			mdatEnd = pos + size
			break
		}
		pos += size
	}
	if moofStart >= 0 && mdatEnd > moofStart {
		return data[moofStart:mdatEnd]
	}
	return nil
}
