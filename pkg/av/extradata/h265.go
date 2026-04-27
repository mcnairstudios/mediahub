package extradata

import "fmt"

const (
	h265NALTypeVPS = 32
	h265NALTypeSPS = 33
	h265NALTypePPS = 34
)

func h265ToHvcC(extradata []byte) ([]byte, error) {
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
		return nil, fmt.Errorf("extradata: h265: no NAL units found in Annex B data")
	}

	var vpsList, spsList, ppsList [][]byte
	for _, nalu := range nalus {
		if len(nalu) < 2 {
			continue
		}
		nalType := (nalu[0] >> 1) & 0x3F
		switch nalType {
		case h265NALTypeVPS:
			vpsList = append(vpsList, nalu)
		case h265NALTypeSPS:
			spsList = append(spsList, nalu)
		case h265NALTypePPS:
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

	var profileSpace, tierFlag, profileIDC byte
	var profileCompat [4]byte
	var constraintFlags [6]byte
	var levelIDC byte

	if len(sps) >= 15 {
		ptlByte := sps[3]
		profileSpace = (ptlByte >> 6) & 0x03
		tierFlag = (ptlByte >> 5) & 0x01
		profileIDC = ptlByte & 0x1F

		copy(profileCompat[:], sps[4:8])
		copy(constraintFlags[:], sps[8:14])
		levelIDC = sps[14]
	}

	chromaFormat := byte(1)
	bitDepthLumaMinus8 := byte(0)
	bitDepthChromaMinus8 := byte(0)
	if parsed := ParseH265SPS(sps); parsed != nil {
		chromaFormat = byte(parsed.ChromaFormatIDC)
		bitDepthLumaMinus8 = byte(parsed.BitDepthLuma - 8)
		bitDepthChromaMinus8 = byte(parsed.BitDepthChroma - 8)
	}

	type naluArray struct {
		nalType byte
		nalus   [][]byte
	}
	var arrays []naluArray
	if len(vpsList) > 0 {
		arrays = append(arrays, naluArray{h265NALTypeVPS, vpsList})
	}
	arrays = append(arrays, naluArray{h265NALTypeSPS, spsList})
	arrays = append(arrays, naluArray{h265NALTypePPS, ppsList})

	size := 23
	for _, arr := range arrays {
		size += 3
		for _, nalu := range arr.nalus {
			size += 2 + len(nalu)
		}
	}

	out := make([]byte, size)
	i := 0

	out[i] = 0x01
	i++
	out[i] = (profileSpace << 6) | (tierFlag << 5) | profileIDC
	i++
	copy(out[i:i+4], profileCompat[:])
	i += 4
	copy(out[i:i+6], constraintFlags[:])
	i += 6
	out[i] = levelIDC
	i++
	out[i] = 0xF0
	out[i+1] = 0x00
	i += 2
	out[i] = 0xFC
	i++
	out[i] = 0xFC | (chromaFormat & 0x03)
	i++
	out[i] = 0xF8 | (bitDepthLumaMinus8 & 0x07)
	i++
	out[i] = 0xF8 | (bitDepthChromaMinus8 & 0x07)
	i++
	out[i] = 0x00
	out[i+1] = 0x00
	i += 2
	out[i] = 0x0F
	i++
	out[i] = byte(len(arrays))
	i++

	for _, arr := range arrays {
		out[i] = 0x80 | (arr.nalType & 0x3F)
		i++
		putU16BE(out[i:], uint16(len(arr.nalus)))
		i += 2
		for _, nalu := range arr.nalus {
			putU16BE(out[i:], uint16(len(nalu)))
			i += 2
			copy(out[i:], nalu)
			i += len(nalu)
		}
	}

	if i != size {
		return nil, fmt.Errorf("extradata: h265: size mismatch: wrote %d, expected %d", i, size)
	}

	return out, nil
}
