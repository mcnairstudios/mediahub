package mux

import (
	"encoding/binary"
	"fmt"
)

func extractCodecString(initSegment []byte) string {
	moov := findBox(initSegment, "moov")
	if moov == nil {
		return ""
	}
	trak := findBox(moov, "trak")
	if trak == nil {
		return ""
	}
	mdia := findBox(trak, "mdia")
	if mdia == nil {
		return ""
	}
	minf := findBox(mdia, "minf")
	if minf == nil {
		return ""
	}
	stbl := findBox(minf, "stbl")
	if stbl == nil {
		return ""
	}
	stsd := findBox(stbl, "stsd")
	if stsd == nil {
		return ""
	}

	if len(stsd) < 8 {
		return ""
	}
	entries := stsd[8:]

	if avc1 := findBox(entries, "avc1"); avc1 != nil {
		return parseAVC1CodecString(avc1)
	}
	if hev1 := findBox(entries, "hev1"); hev1 != nil {
		return parseHEVCCodecString(hev1, "hev1")
	}
	if hvc1 := findBox(entries, "hvc1"); hvc1 != nil {
		return parseHEVCCodecString(hvc1, "hvc1")
	}

	return ""
}

const visualSampleEntrySize = 78

func parseAVC1CodecString(avc1 []byte) string {
	if len(avc1) < visualSampleEntrySize+8 {
		return ""
	}
	avcC := findBox(avc1[visualSampleEntrySize:], "avcC")
	if avcC == nil || len(avcC) < 4 {
		return ""
	}
	profile := avcC[1]
	compat := avcC[2]
	level := avcC[3]
	return fmt.Sprintf("avc1.%02X%02X%02X", profile, compat, level)
}

func parseHEVCCodecString(entry []byte, tag string) string {
	if len(entry) < visualSampleEntrySize+8 {
		return ""
	}
	hvcC := findBox(entry[visualSampleEntrySize:], "hvcC")
	if hvcC == nil || len(hvcC) < 13 {
		return ""
	}
	if hvcC[0] != 1 {
		return ""
	}
	profileSpace := (hvcC[1] >> 6) & 0x03
	tierFlag := (hvcC[1] >> 5) & 0x01
	profileIDC := hvcC[1] & 0x1F
	profileCompat := binary.BigEndian.Uint32(hvcC[2:6])
	levelIDC := hvcC[12]

	var constraints [6]byte
	copy(constraints[:], hvcC[6:12])

	var spacePrefix string
	if profileSpace > 0 {
		spacePrefix = string(rune('A' + profileSpace - 1))
	}

	tier := "L"
	if tierFlag == 1 {
		tier = "H"
	}

	constraintStr := ""
	for i := 5; i >= 0; i-- {
		if constraints[i] != 0 || constraintStr != "" {
			if constraintStr != "" {
				constraintStr = fmt.Sprintf("%02X.", constraints[i]) + constraintStr
			} else {
				constraintStr = fmt.Sprintf("%02X", constraints[i])
			}
		}
	}
	if constraintStr == "" {
		constraintStr = "B0"
	}

	_ = profileCompat

	return fmt.Sprintf("%s.%s%d.%s%d.%s", tag, spacePrefix, profileIDC, tier, levelIDC, constraintStr)
}

func findBox(data []byte, boxType string) []byte {
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

		if fourcc == boxType {
			return data[offset+headerSize : offset+size]
		}

		offset += size
	}
	return nil
}
