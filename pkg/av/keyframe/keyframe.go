package keyframe

func IsKeyframe(data []byte, codec string) bool {
	nalus := splitNALUnits(data)
	switch codec {
	case "h264":
		return isKeyframeH264(nalus)
	case "hevc", "h265":
		return isKeyframeH265(nalus)
	default:
		return false
	}
}

func FixDeltaUnit(data []byte, codec string) bool {
	return !IsKeyframe(data, codec)
}

type KeyframeTracker struct {
	isLive       bool
	seenKeyframe bool
}

func NewKeyframeTracker(isLive bool) *KeyframeTracker {
	return &KeyframeTracker{isLive: isLive}
}

func (t *KeyframeTracker) Reset() {
	t.seenKeyframe = false
}

func (t *KeyframeTracker) ShouldDrop(data []byte, codec string) bool {
	if t.isLive {
		FixDeltaUnit(data, codec)
		return false
	}

	if !t.seenKeyframe {
		if IsKeyframe(data, codec) {
			t.seenKeyframe = true
			return false
		}
		return true
	}

	FixDeltaUnit(data, codec)
	return false
}

func isKeyframeH264(nalus [][]byte) bool {
	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}
		nalType := nalu[0] & 0x1F
		if nalType == 5 {
			return true
		}
	}
	return false
}

func isKeyframeH265(nalus [][]byte) bool {
	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}
		nalType := (nalu[0] >> 1) & 0x3F
		if nalType >= 16 && nalType <= 21 {
			return true
		}
	}
	return false
}

func splitNALUnits(data []byte) [][]byte {
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
