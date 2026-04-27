package extradata

import (
	"encoding/hex"
	"fmt"
)

func ToCodecData(codec string, extradata []byte) ([]byte, error) {
	if len(extradata) == 0 {
		return nil, nil
	}

	switch codec {
	case "h264":
		return h264ToAvcC(extradata)
	case "hevc", "h265":
		return h265ToHvcC(extradata)
	default:
		return nil, nil
	}
}

func ToHexString(data []byte) string {
	return hex.EncodeToString(data)
}

func SplitNALUnits(data []byte) [][]byte {
	var nalus [][]byte
	i := 0
	n := len(data)

	start := -1
	for i < n {
		if i+3 < n && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
			if start >= 0 {
				nalu := trimTrailingZeros(data[start:i])
				if len(nalu) > 0 {
					nalus = append(nalus, nalu)
				}
			}
			i += 4
			start = i
			continue
		}
		if i+2 < n && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			if start >= 0 {
				nalu := trimTrailingZeros(data[start:i])
				if len(nalu) > 0 {
					nalus = append(nalus, nalu)
				}
			}
			i += 3
			start = i
			continue
		}
		i++
	}

	if start >= 0 && start < n {
		nalu := data[start:n]
		if len(nalu) > 0 {
			nalus = append(nalus, nalu)
		}
	}

	return nalus
}

func trimTrailingZeros(data []byte) []byte {
	end := len(data)
	for end > 0 && data[end-1] == 0x00 {
		end--
	}
	return data[:end]
}

func putU16BE(b []byte, v uint16) {
	b[0] = byte(v >> 8)
	b[1] = byte(v)
}

var errNoSPS = fmt.Errorf("extradata: no SPS NAL unit found")

var errNoPPS = fmt.Errorf("extradata: no PPS NAL unit found")
