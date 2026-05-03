package validate

import "encoding/binary"

func BuildFMP4InitForTest(codec string, includeCodecConfig bool) []byte {
	var buf []byte

	buf = append(buf, buildBox("ftyp", []byte("isom\x00\x00\x02\x00isomiso2mp41"))...)

	var codecBox []byte
	header := make([]byte, 78)
	switch codec {
	case "avc1":
		var inner []byte
		if includeCodecConfig {
			inner = buildBox("avcC", []byte{1, 100, 0, 31, 0xff, 0xe1, 0, 4, 0, 0, 0, 1, 1, 0, 4, 0, 0, 0, 1})
		}
		codecBox = buildBox("avc1", append(header, inner...))
	case "hvc1":
		var inner []byte
		if includeCodecConfig {
			inner = buildBox("hvcC", []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
		}
		codecBox = buildBox("hvc1", append(header, inner...))
	case "mp4a":
		audioHeader := make([]byte, 28)
		var inner []byte
		if includeCodecConfig {
			inner = buildBox("esds", []byte{0, 0, 0, 0})
		}
		codecBox = buildBox("mp4a", append(audioHeader, inner...))
	}

	stsdPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(stsdPayload[4:], 1)
	stsdPayload = append(stsdPayload, codecBox...)
	stsd := buildBox("stsd", stsdPayload)

	stbl := buildBox("stbl", stsd)
	minf := buildBox("minf", stbl)
	mdia := buildBox("mdia", minf)
	trak := buildBox("trak", mdia)
	moov := buildBox("moov", trak)

	buf = append(buf, moov...)
	return buf
}

func BuildFMP4SegmentForTest(sampleCount uint32) []byte {
	tfdtPayload := make([]byte, 12)
	tfdtPayload[0] = 1
	binary.BigEndian.PutUint64(tfdtPayload[4:], 90000)
	tfdt := buildBox("tfdt", tfdtPayload)

	tfhdPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(tfhdPayload[4:], 1)
	tfhd := buildBox("tfhd", tfhdPayload)

	trunPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(trunPayload[4:], sampleCount)
	trun := buildBox("trun", trunPayload)

	traf := buildBox("traf", append(append(tfhd, tfdt...), trun...))
	moof := buildBox("moof", traf)
	mdat := buildBox("mdat", []byte("sample data here"))

	return append(moof, mdat...)
}

func buildBox(boxType string, payload []byte) []byte {
	size := uint32(8 + len(payload))
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, size)
	buf = append(buf, []byte(boxType)...)
	buf = append(buf, payload...)
	return buf
}
