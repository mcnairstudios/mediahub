package validate

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func ValidateHLSPlaylist(data []byte) []ValidationError {
	var errs []ValidationError
	text := string(data)
	lines := strings.Split(strings.TrimSpace(text), "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "#EXTM3U" {
		errs = append(errs, ValidationError{Field: "header", Message: "missing #EXTM3U header"})
		return errs
	}

	targetDuration := -1.0
	hasMediaSequence := false
	var segmentDurations []float64
	var segmentNames []string
	seenSegments := map[string]bool{}
	isLive := true

	for i, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			val := strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:")
			td, err := strconv.ParseFloat(val, 64)
			if err != nil {
				errs = append(errs, ValidationError{Field: "targetduration", Message: fmt.Sprintf("invalid value: %s", val)})
			} else {
				targetDuration = td
			}
		}

		if strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:") {
			hasMediaSequence = true
		}

		if strings.HasPrefix(line, "#EXT-X-ENDLIST") {
			isLive = false
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			val := strings.TrimPrefix(line, "#EXTINF:")
			val = strings.TrimSuffix(val, ",")
			if idx := strings.Index(val, ","); idx >= 0 {
				val = val[:idx]
			}
			dur, err := strconv.ParseFloat(val, 64)
			if err != nil {
				errs = append(errs, ValidationError{Field: "extinf", Message: fmt.Sprintf("line %d: invalid duration: %s", i+1, val)})
			} else {
				segmentDurations = append(segmentDurations, dur)
			}

			if i+1 < len(lines) {
				segName := strings.TrimSpace(lines[i+1])
				if !strings.HasPrefix(segName, "#") && segName != "" {
					segmentNames = append(segmentNames, segName)
					if seenSegments[segName] {
						errs = append(errs, ValidationError{Field: "segments", Message: fmt.Sprintf("duplicate segment: %s", segName)})
					}
					seenSegments[segName] = true
				}
			}
		}
	}

	if targetDuration < 0 {
		errs = append(errs, ValidationError{Field: "targetduration", Message: "missing #EXT-X-TARGETDURATION"})
	} else if targetDuration < 2 || targetDuration > 10 {
		errs = append(errs, ValidationError{Field: "targetduration", Message: fmt.Sprintf("value %v outside reasonable range (2-10s)", targetDuration)})
	}

	if isLive && !hasMediaSequence {
		errs = append(errs, ValidationError{Field: "mediasequence", Message: "missing #EXT-X-MEDIA-SEQUENCE for live playlist"})
	}

	for _, dur := range segmentDurations {
		if targetDuration > 0 && dur > targetDuration+0.5 {
			errs = append(errs, ValidationError{Field: "duration", Message: fmt.Sprintf("segment duration %.3f exceeds target duration %.0f", dur, targetDuration)})
		}
	}

	numRe := regexp.MustCompile(`(\d+)`)
	if len(segmentNames) > 1 {
		var nums []int
		for _, name := range segmentNames {
			matches := numRe.FindStringSubmatch(name)
			if len(matches) > 1 {
				n, err := strconv.Atoi(matches[1])
				if err == nil {
					nums = append(nums, n)
				}
			}
		}
		for i := 1; i < len(nums); i++ {
			if nums[i] != nums[i-1]+1 {
				errs = append(errs, ValidationError{Field: "sequence", Message: fmt.Sprintf("non-sequential segments: %d followed by %d", nums[i-1], nums[i])})
				break
			}
		}
	}

	return errs
}

type mpd struct {
	XMLName xml.Name  `xml:"MPD"`
	Periods []mpdPeriod `xml:"Period"`
}

type mpdPeriod struct {
	AdaptationSets []mpdAdaptationSet `xml:"AdaptationSet"`
}

type mpdAdaptationSet struct {
	Representations []mpdRepresentation `xml:"Representation"`
	SegmentTemplate *mpdSegmentTemplate `xml:"SegmentTemplate"`
}

type mpdRepresentation struct {
	ID              string              `xml:"id,attr"`
	Bandwidth       int                 `xml:"bandwidth,attr"`
	SegmentTemplate *mpdSegmentTemplate `xml:"SegmentTemplate"`
}

type mpdSegmentTemplate struct {
	Timescale         int                `xml:"timescale,attr"`
	Duration          int                `xml:"duration,attr"`
	Media             string             `xml:"media,attr"`
	Initialization    string             `xml:"initialization,attr"`
	SegmentTimeline   *mpdSegmentTimeline `xml:"SegmentTimeline"`
}

type mpdSegmentTimeline struct {
	Segments []mpdTimelineS `xml:"S"`
}

type mpdTimelineS struct {
	T int64 `xml:"t,attr"`
	D int64 `xml:"d,attr"`
	R int   `xml:"r,attr"`
}

func ValidateMPD(data []byte) []ValidationError {
	var errs []ValidationError
	var doc mpd

	if err := xml.Unmarshal(data, &doc); err != nil {
		errs = append(errs, ValidationError{Field: "xml", Message: fmt.Sprintf("invalid XML: %v", err)})
		return errs
	}

	if doc.XMLName.Space != "urn:mpeg:dash:schema:mpd:2011" && doc.XMLName.Space != "" {
		errs = append(errs, ValidationError{Field: "namespace", Message: fmt.Sprintf("unexpected namespace: %s", doc.XMLName.Space)})
	}

	if len(doc.Periods) == 0 {
		errs = append(errs, ValidationError{Field: "period", Message: "no Period elements found"})
		return errs
	}

	for pi, period := range doc.Periods {
		if len(period.AdaptationSets) == 0 {
			errs = append(errs, ValidationError{Field: "adaptationset", Message: fmt.Sprintf("Period %d has no AdaptationSet", pi)})
			continue
		}

		for ai, as := range period.AdaptationSets {
			if len(as.Representations) == 0 {
				errs = append(errs, ValidationError{Field: "representation", Message: fmt.Sprintf("Period %d AdaptationSet %d has no Representation", pi, ai)})
			}

			templates := collectTemplates(as)
			for _, tmpl := range templates {
				if tmpl.Timescale <= 0 {
					errs = append(errs, ValidationError{Field: "timescale", Message: fmt.Sprintf("Period %d AdaptationSet %d: timescale must be > 0", pi, ai)})
				}
				hasTimeline := tmpl.SegmentTimeline != nil && len(tmpl.SegmentTimeline.Segments) > 0
				if tmpl.Duration <= 0 && !hasTimeline {
					errs = append(errs, ValidationError{Field: "segment_duration", Message: fmt.Sprintf("Period %d AdaptationSet %d: segment duration must be > 0 (or use SegmentTimeline)", pi, ai)})
				}
				if tmpl.Media == "" {
					errs = append(errs, ValidationError{Field: "segment_template", Message: fmt.Sprintf("Period %d AdaptationSet %d: SegmentTemplate missing media attribute", pi, ai)})
				}
			}

			if len(templates) == 0 {
				errs = append(errs, ValidationError{Field: "segment_template", Message: fmt.Sprintf("Period %d AdaptationSet %d: no SegmentTemplate found", pi, ai)})
			}
		}
	}

	return errs
}

func collectTemplates(as mpdAdaptationSet) []*mpdSegmentTemplate {
	var templates []*mpdSegmentTemplate
	if as.SegmentTemplate != nil {
		templates = append(templates, as.SegmentTemplate)
	}
	for _, rep := range as.Representations {
		if rep.SegmentTemplate != nil {
			templates = append(templates, rep.SegmentTemplate)
		}
	}
	return templates
}

func ValidateFMP4Init(data []byte) []ValidationError {
	var errs []ValidationError
	boxes := parseBoxes(data)

	if !hasBox(boxes, "ftyp") {
		errs = append(errs, ValidationError{Field: "ftyp", Message: "missing ftyp box"})
	}

	moov := findBox(boxes, "moov")
	if moov == nil {
		errs = append(errs, ValidationError{Field: "moov", Message: "missing moov box"})
		return errs
	}

	moovChildren := parseBoxes(moov.payload)
	trak := findBox(moovChildren, "trak")
	if trak == nil {
		errs = append(errs, ValidationError{Field: "trak", Message: "moov missing trak box"})
		return errs
	}

	trakChildren := parseBoxes(trak.payload)
	mdia := findBox(trakChildren, "mdia")
	if mdia == nil {
		errs = append(errs, ValidationError{Field: "mdia", Message: "trak missing mdia box"})
		return errs
	}

	mdiaChildren := parseBoxes(mdia.payload)
	minf := findBox(mdiaChildren, "minf")
	if minf == nil {
		errs = append(errs, ValidationError{Field: "minf", Message: "mdia missing minf box"})
		return errs
	}

	minfChildren := parseBoxes(minf.payload)
	stbl := findBox(minfChildren, "stbl")
	if stbl == nil {
		errs = append(errs, ValidationError{Field: "stbl", Message: "minf missing stbl box"})
		return errs
	}

	stblChildren := parseBoxes(stbl.payload)
	stsd := findBox(stblChildren, "stsd")
	if stsd == nil {
		errs = append(errs, ValidationError{Field: "stsd", Message: "stbl missing stsd box"})
		return errs
	}

	validCodecs := map[string]bool{"avc1": true, "hvc1": true, "hev1": true, "mp4a": true, "Opus": true}
	stsdPayload := stsd.payload
	if len(stsdPayload) >= 8 {
		stsdPayload = stsdPayload[8:]
	}
	codecBoxes := parseBoxes(stsdPayload)
	foundCodec := false
	for _, box := range codecBoxes {
		if validCodecs[box.boxType] {
			foundCodec = true
			errs = append(errs, validateCodecBox(box)...)
			break
		}
	}
	if !foundCodec {
		errs = append(errs, ValidationError{Field: "codec", Message: "stsd contains no recognized codec entry (avc1/hvc1/mp4a)"})
	}

	return errs
}

func validateCodecBox(box *mp4Box) []ValidationError {
	var errs []ValidationError

	switch box.boxType {
	case "avc1":
		children := parseBoxes(box.payload[78:])
		if !hasBox(children, "avcC") {
			errs = append(errs, ValidationError{Field: "avcC", Message: "avc1 missing avcC box"})
		}
	case "hvc1", "hev1":
		children := parseBoxes(box.payload[78:])
		if !hasBox(children, "hvcC") {
			errs = append(errs, ValidationError{Field: "hvcC", Message: "hvc1/hev1 missing hvcC box"})
		}
	case "mp4a":
		if len(box.payload) >= 28 {
			children := parseBoxes(box.payload[28:])
			if !hasBox(children, "esds") {
				errs = append(errs, ValidationError{Field: "esds", Message: "mp4a missing esds box"})
			}
		}
	}

	return errs
}

func ValidateFMP4Segment(data []byte) []ValidationError {
	var errs []ValidationError
	boxes := parseBoxes(data)

	moof := findBox(boxes, "moof")
	if moof == nil {
		errs = append(errs, ValidationError{Field: "moof", Message: "missing moof box"})
		return errs
	}

	if !hasBox(boxes, "mdat") {
		errs = append(errs, ValidationError{Field: "mdat", Message: "missing mdat box"})
	}

	moofChildren := parseBoxes(moof.payload)
	traf := findBox(moofChildren, "traf")
	if traf == nil {
		errs = append(errs, ValidationError{Field: "traf", Message: "moof missing traf box"})
		return errs
	}

	trafChildren := parseBoxes(traf.payload)

	if !hasBox(trafChildren, "tfhd") {
		errs = append(errs, ValidationError{Field: "tfhd", Message: "traf missing tfhd box"})
	}

	trun := findBox(trafChildren, "trun")
	if trun == nil {
		errs = append(errs, ValidationError{Field: "trun", Message: "traf missing trun box"})
	} else if len(trun.payload) >= 8 {
		sampleCount := binary.BigEndian.Uint32(trun.payload[4:8])
		if sampleCount == 0 {
			errs = append(errs, ValidationError{Field: "samples", Message: "trun sample count is 0"})
		}
	}

	tfdt := findBox(trafChildren, "tfdt")
	if tfdt != nil && len(tfdt.payload) >= 12 {
		version := tfdt.payload[0]
		var baseDecodeTime uint64
		if version == 1 {
			baseDecodeTime = binary.BigEndian.Uint64(tfdt.payload[4:12])
		} else if len(tfdt.payload) >= 8 {
			baseDecodeTime = uint64(binary.BigEndian.Uint32(tfdt.payload[4:8]))
		}
		_ = baseDecodeTime
	}

	return errs
}

type mp4Box struct {
	boxType string
	payload []byte
}

func parseBoxes(data []byte) []*mp4Box {
	var boxes []*mp4Box
	r := bytes.NewReader(data)

	for r.Len() >= 8 {
		offset, _ := r.Seek(0, 1)
		var size uint32
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			break
		}

		typeBytes := make([]byte, 4)
		if _, err := r.Read(typeBytes); err != nil {
			break
		}
		boxType := string(typeBytes)

		if size < 8 {
			break
		}

		payloadSize := int(size) - 8
		if payloadSize > r.Len() {
			payloadSize = r.Len()
		}

		payload := data[int(offset)+8 : int(offset)+8+payloadSize]
		boxes = append(boxes, &mp4Box{boxType: boxType, payload: payload})

		if _, err := r.Seek(int64(payloadSize), 1); err != nil {
			break
		}
	}

	return boxes
}

func hasBox(boxes []*mp4Box, boxType string) bool {
	return findBox(boxes, boxType) != nil
}

func findBox(boxes []*mp4Box, boxType string) *mp4Box {
	for _, b := range boxes {
		if b.boxType == boxType {
			return b
		}
	}
	return nil
}

const tsPacketSize = 188
const tsSyncByte = 0x47

func ValidateTSSegment(data []byte) []ValidationError {
	var errs []ValidationError

	if len(data) == 0 {
		errs = append(errs, ValidationError{Field: "data", Message: "empty TS segment"})
		return errs
	}

	if len(data)%tsPacketSize != 0 {
		errs = append(errs, ValidationError{Field: "alignment", Message: fmt.Sprintf("data length %d is not a multiple of 188", len(data))})
	}

	packetCount := len(data) / tsPacketSize
	if packetCount == 0 {
		errs = append(errs, ValidationError{Field: "packets", Message: "no complete TS packets"})
		return errs
	}

	hasPAT := false
	pids := map[uint16]int{}

	for i := 0; i < packetCount; i++ {
		offset := i * tsPacketSize
		pkt := data[offset : offset+tsPacketSize]

		if pkt[0] != tsSyncByte {
			errs = append(errs, ValidationError{Field: "sync", Message: fmt.Sprintf("packet %d: invalid sync byte 0x%02x (expected 0x47)", i, pkt[0])})
			if i < 3 {
				return errs
			}
			continue
		}

		pid := uint16(pkt[1]&0x1f)<<8 | uint16(pkt[2])
		pids[pid]++

		if pid == 0x0000 {
			hasPAT = true
		}
	}

	if !hasPAT {
		errs = append(errs, ValidationError{Field: "pat", Message: "no PAT packet found (PID 0x0000)"})
	}

	contentPIDs := 0
	for pid, count := range pids {
		if pid != 0x0000 && pid != 0x0001 && pid != 0x1FFF && count > 0 {
			contentPIDs++
		}
	}
	if contentPIDs == 0 && packetCount > 5 {
		errs = append(errs, ValidationError{Field: "content", Message: "no content PIDs found (only PAT/null)"})
	}

	return errs
}
