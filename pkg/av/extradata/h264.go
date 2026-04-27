package extradata

import "fmt"

const h264NALTypeSPS = 7

const h264NALTypePPS = 8

func h264ToAvcC(extradata []byte) ([]byte, error) {
	if len(extradata) == 0 {
		return nil, nil
	}

	if extradata[0] == 0x01 {
		out := make([]byte, len(extradata))
		copy(out, extradata)
		return out, nil
	}

	nalus := SplitNALUnits(extradata)
	if len(nalus) == 0 {
		return nil, fmt.Errorf("extradata: h264: no NAL units found in Annex B data")
	}

	var spsList, ppsList [][]byte
	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}
		nalType := nalu[0] & 0x1F
		switch nalType {
		case h264NALTypeSPS:
			spsList = append(spsList, nalu)
		case h264NALTypePPS:
			ppsList = append(ppsList, nalu)
		}
	}

	if len(spsList) == 0 {
		return nil, errNoSPS
	}
	if len(ppsList) == 0 {
		return nil, errNoPPS
	}

	sps := spsList[0]
	if len(sps) < 4 {
		return nil, fmt.Errorf("extradata: h264: SPS too short (%d bytes)", len(sps))
	}

	size := 6
	for _, s := range spsList {
		size += 2 + len(s)
	}
	size++
	for _, p := range ppsList {
		size += 2 + len(p)
	}

	out := make([]byte, size)
	i := 0

	out[i] = 0x01
	out[i+1] = sps[1]
	out[i+2] = sps[2]
	out[i+3] = sps[3]
	out[i+4] = 0xFF
	out[i+5] = 0xE0 | byte(len(spsList)&0x1F)
	i += 6

	for _, s := range spsList {
		putU16BE(out[i:], uint16(len(s)))
		i += 2
		copy(out[i:], s)
		i += len(s)
	}

	out[i] = byte(len(ppsList))
	i++

	for _, p := range ppsList {
		putU16BE(out[i:], uint16(len(p)))
		i += 2
		copy(out[i:], p)
		i += len(p)
	}

	return out, nil
}
