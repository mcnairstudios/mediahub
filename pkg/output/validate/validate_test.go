package validate

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validHLSPlaylist() []byte {
	return []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
segment0.ts
#EXTINF:6.000,
segment1.ts
#EXTINF:6.000,
segment2.ts
`)
}

func validHLSVODPlaylist() []byte {
	return []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
segment0.ts
#EXTINF:6.000,
segment1.ts
#EXTINF:4.500,
segment2.ts
#EXT-X-ENDLIST
`)
}

func TestValidateHLSPlaylist_Valid(t *testing.T) {
	errs := ValidateHLSPlaylist(validHLSPlaylist())
	assert.Empty(t, errs)
}

func TestValidateHLSPlaylist_VODValid(t *testing.T) {
	errs := ValidateHLSPlaylist(validHLSVODPlaylist())
	assert.Empty(t, errs)
}

func TestValidateHLSPlaylist_MissingHeader(t *testing.T) {
	data := []byte(`#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
segment0.ts
`)
	errs := ValidateHLSPlaylist(data)
	require.NotEmpty(t, errs)
	assert.Equal(t, "header", errs[0].Field)
}

func TestValidateHLSPlaylist_MissingTargetDuration(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
segment0.ts
#EXT-X-ENDLIST
`)
	errs := ValidateHLSPlaylist(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "targetduration")
}

func TestValidateHLSPlaylist_TargetDurationTooLow(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-TARGETDURATION:1
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:1.000,
segment0.ts
#EXT-X-ENDLIST
`)
	errs := ValidateHLSPlaylist(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "targetduration")
}

func TestValidateHLSPlaylist_TargetDurationTooHigh(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-TARGETDURATION:30
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
segment0.ts
#EXT-X-ENDLIST
`)
	errs := ValidateHLSPlaylist(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "targetduration")
}

func TestValidateHLSPlaylist_DuplicateSegments(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
segment0.ts
#EXTINF:6.000,
segment0.ts
#EXT-X-ENDLIST
`)
	errs := ValidateHLSPlaylist(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "segments")
}

func TestValidateHLSPlaylist_NonSequentialSegments(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
segment0.ts
#EXTINF:6.000,
segment5.ts
#EXT-X-ENDLIST
`)
	errs := ValidateHLSPlaylist(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "sequence")
}

func TestValidateHLSPlaylist_MissingMediaSequenceLive(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-TARGETDURATION:6
#EXTINF:6.000,
segment0.ts
`)
	errs := ValidateHLSPlaylist(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "mediasequence")
}

func TestValidateHLSPlaylist_SegmentExceedsTargetDuration(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:6.000,
segment0.ts
#EXT-X-ENDLIST
`)
	errs := ValidateHLSPlaylist(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "duration")
}

func validMPD() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="dynamic">
  <Period>
    <AdaptationSet>
      <SegmentTemplate timescale="90000" duration="540000" media="seg_$Number$.m4s" initialization="init.mp4"/>
      <Representation id="1" bandwidth="2000000"/>
    </AdaptationSet>
  </Period>
</MPD>`)
}

func TestValidateMPD_Valid(t *testing.T) {
	errs := ValidateMPD(validMPD())
	assert.Empty(t, errs)
}

func TestValidateMPD_InvalidXML(t *testing.T) {
	data := []byte(`not xml at all`)
	errs := ValidateMPD(data)
	require.NotEmpty(t, errs)
	assert.Equal(t, "xml", errs[0].Field)
}

func TestValidateMPD_NoPeriod(t *testing.T) {
	data := []byte(`<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011"></MPD>`)
	errs := ValidateMPD(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "period")
}

func TestValidateMPD_MissingTimescale(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011">
  <Period>
    <AdaptationSet>
      <SegmentTemplate duration="540000" media="seg_$Number$.m4s"/>
      <Representation id="1" bandwidth="2000000"/>
    </AdaptationSet>
  </Period>
</MPD>`)
	errs := ValidateMPD(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "timescale")
}

func TestValidateMPD_MissingSegmentDuration(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011">
  <Period>
    <AdaptationSet>
      <SegmentTemplate timescale="90000" media="seg_$Number$.m4s"/>
      <Representation id="1" bandwidth="2000000"/>
    </AdaptationSet>
  </Period>
</MPD>`)
	errs := ValidateMPD(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "segment_duration")
}

func TestValidateMPD_MissingMediaAttribute(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011">
  <Period>
    <AdaptationSet>
      <SegmentTemplate timescale="90000" duration="540000"/>
      <Representation id="1" bandwidth="2000000"/>
    </AdaptationSet>
  </Period>
</MPD>`)
	errs := ValidateMPD(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "segment_template")
}

func buildFMP4Init(codec string, includeCodecConfig bool) []byte {
	var buf []byte

	buf = append(buf, makeBox("ftyp", []byte("isom\x00\x00\x02\x00isomiso2mp41"))...)

	var codecBox []byte
	header := make([]byte, 78)
	switch codec {
	case "avc1":
		var inner []byte
		if includeCodecConfig {
			inner = makeBox("avcC", []byte{1, 100, 0, 31, 0xff, 0xe1, 0, 4, 0, 0, 0, 1, 1, 0, 4, 0, 0, 0, 1})
		}
		codecBox = makeBox("avc1", append(header, inner...))
	case "hvc1":
		var inner []byte
		if includeCodecConfig {
			inner = makeBox("hvcC", []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
		}
		codecBox = makeBox("hvc1", append(header, inner...))
	case "mp4a":
		audioHeader := make([]byte, 28)
		var inner []byte
		if includeCodecConfig {
			inner = makeBox("esds", []byte{0, 0, 0, 0})
		}
		codecBox = makeBox("mp4a", append(audioHeader, inner...))
	}

	stsdPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(stsdPayload[4:], 1)
	stsdPayload = append(stsdPayload, codecBox...)
	stsd := makeFullBox("stsd", stsdPayload)

	stbl := makeBox("stbl", stsd)
	minf := makeBox("minf", stbl)
	mdia := makeBox("mdia", minf)
	trak := makeBox("trak", mdia)
	moov := makeBox("moov", trak)

	buf = append(buf, moov...)
	return buf
}

func buildFMP4Segment(sampleCount uint32) []byte {
	tfdtPayload := make([]byte, 12)
	tfdtPayload[0] = 1
	binary.BigEndian.PutUint64(tfdtPayload[4:], 90000)
	tfdt := makeFullBox("tfdt", tfdtPayload)

	tfhdPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(tfhdPayload[4:], 1)
	tfhd := makeFullBox("tfhd", tfhdPayload)

	trunPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(trunPayload[4:], sampleCount)
	trun := makeFullBox("trun", trunPayload)

	traf := makeBox("traf", append(append(tfhd, tfdt...), trun...))
	moof := makeBox("moof", traf)
	mdat := makeBox("mdat", []byte("sample data here"))

	return append(moof, mdat...)
}

func makeBox(boxType string, payload []byte) []byte {
	size := uint32(8 + len(payload))
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, size)
	buf = append(buf, []byte(boxType)...)
	buf = append(buf, payload...)
	return buf
}

func makeFullBox(boxType string, payload []byte) []byte {
	return makeBox(boxType, payload)
}

func TestValidateFMP4Init_ValidAVC(t *testing.T) {
	data := buildFMP4Init("avc1", true)
	errs := ValidateFMP4Init(data)
	assert.Empty(t, errs)
}

func TestValidateFMP4Init_ValidHEVC(t *testing.T) {
	data := buildFMP4Init("hvc1", true)
	errs := ValidateFMP4Init(data)
	assert.Empty(t, errs)
}

func TestValidateFMP4Init_ValidAAC(t *testing.T) {
	data := buildFMP4Init("mp4a", true)
	errs := ValidateFMP4Init(data)
	assert.Empty(t, errs)
}

func TestValidateFMP4Init_MissingMoov(t *testing.T) {
	data := makeBox("ftyp", []byte("isom\x00\x00\x02\x00isomiso2mp41"))
	errs := ValidateFMP4Init(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "moov")
}

func TestValidateFMP4Init_MissingFtyp(t *testing.T) {
	header := make([]byte, 78)
	codecBox := makeBox("avc1", append(header, makeBox("avcC", []byte{1, 100, 0, 31})...))
	stsdPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(stsdPayload[4:], 1)
	stsdPayload = append(stsdPayload, codecBox...)
	stsd := makeFullBox("stsd", stsdPayload)
	stbl := makeBox("stbl", stsd)
	minf := makeBox("minf", stbl)
	mdia := makeBox("mdia", minf)
	trak := makeBox("trak", mdia)
	moov := makeBox("moov", trak)

	errs := ValidateFMP4Init(moov)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "ftyp")
}

func TestValidateFMP4Init_MissingAvcC(t *testing.T) {
	data := buildFMP4Init("avc1", false)
	errs := ValidateFMP4Init(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "avcC")
}

func TestValidateFMP4Init_MissingHvcC(t *testing.T) {
	data := buildFMP4Init("hvc1", false)
	errs := ValidateFMP4Init(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "hvcC")
}

func TestValidateFMP4Segment_Valid(t *testing.T) {
	data := buildFMP4Segment(30)
	errs := ValidateFMP4Segment(data)
	assert.Empty(t, errs)
}

func TestValidateFMP4Segment_MissingMoof(t *testing.T) {
	data := makeBox("mdat", []byte("data"))
	errs := ValidateFMP4Segment(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "moof")
}

func TestValidateFMP4Segment_MissingMdat(t *testing.T) {
	tfhd := makeFullBox("tfhd", make([]byte, 8))
	trun := makeFullBox("trun", []byte{0, 0, 0, 0, 0, 0, 0, 30})
	traf := makeBox("traf", append(tfhd, trun...))
	moof := makeBox("moof", traf)

	errs := ValidateFMP4Segment(moof)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "mdat")
}

func TestValidateFMP4Segment_ZeroSamples(t *testing.T) {
	data := buildFMP4Segment(0)
	errs := ValidateFMP4Segment(data)
	require.NotEmpty(t, errs)
	assertHasField(t, errs, "samples")
}

func assertHasField(t *testing.T, errs []ValidationError, field string) {
	t.Helper()
	for _, e := range errs {
		if e.Field == field {
			return
		}
	}
	var fields []string
	for _, e := range errs {
		fields = append(fields, e.Field+": "+e.Message)
	}
	t.Errorf("expected error with field %q, got: %v", field, fields)
}
