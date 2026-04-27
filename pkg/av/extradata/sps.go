package extradata

type H264SPSInfo struct {
	ProfileIDC       uint8
	LevelIDC         uint8
	ChromaFormatIDC  uint32
	BitDepthLuma     uint32
	BitDepthChroma   uint32
	FrameMBSOnlyFlag bool
	Width            uint32
	Height           uint32
}

var h264HighProfiles = map[uint8]bool{
	100: true, 110: true, 122: true, 244: true,
	44: true, 83: true, 86: true, 118: true,
	128: true, 138: true, 139: true, 134: true,
}

func ParseH264SPS(sps []byte) *H264SPSInfo {
	if len(sps) < 4 {
		return nil
	}

	info := &H264SPSInfo{}
	info.ProfileIDC = sps[1]
	info.LevelIDC = sps[3]

	info.ChromaFormatIDC = 1
	info.BitDepthLuma = 8
	info.BitDepthChroma = 8

	br := newBitReader(sps[4:])

	if _, err := br.readUE(); err != nil {
		return nil
	}

	if h264HighProfiles[info.ProfileIDC] {
		chromaFmt, err := br.readUE()
		if err != nil {
			return nil
		}
		info.ChromaFormatIDC = chromaFmt

		if chromaFmt == 3 {
			if _, err := br.readBit(); err != nil {
				return nil
			}
		}

		bdl, err := br.readUE()
		if err != nil {
			return nil
		}
		info.BitDepthLuma = bdl + 8

		bdc, err := br.readUE()
		if err != nil {
			return nil
		}
		info.BitDepthChroma = bdc + 8

		if _, err := br.readBit(); err != nil {
			return nil
		}

		scalingFlag, err := br.readBit()
		if err != nil {
			return nil
		}
		if scalingFlag == 1 {
			return nil
		}
	}

	if _, err := br.readUE(); err != nil {
		return nil
	}

	pocType, err := br.readUE()
	if err != nil {
		return nil
	}

	switch pocType {
	case 0:
		if _, err := br.readUE(); err != nil {
			return nil
		}
	case 1:
		if _, err := br.readBit(); err != nil {
			return nil
		}
		if _, err := br.readSE(); err != nil {
			return nil
		}
		if _, err := br.readSE(); err != nil {
			return nil
		}
		numRefFrames, err := br.readUE()
		if err != nil {
			return nil
		}
		for i := uint32(0); i < numRefFrames; i++ {
			if _, err := br.readSE(); err != nil {
				return nil
			}
		}
	}

	if _, err := br.readUE(); err != nil {
		return nil
	}

	if _, err := br.readBit(); err != nil {
		return nil
	}

	widthMbs, err := br.readUE()
	if err != nil {
		return nil
	}
	info.Width = widthMbs + 1

	heightMap, err := br.readUE()
	if err != nil {
		return nil
	}
	info.Height = heightMap + 1

	fmo, err := br.readBit()
	if err != nil {
		return nil
	}
	info.FrameMBSOnlyFlag = fmo == 1

	return info
}

type H265SPSInfo struct {
	ChromaFormatIDC uint32
	BitDepthLuma    uint32
	BitDepthChroma  uint32
	Width           uint32
	Height          uint32
}

func skipH265ProfileTierLevel(br *bitReader, profilePresentFlag bool, maxNumSubLayersMinus1 uint32) error {
	if profilePresentFlag {
		br.skip(88)
	}
	br.skip(8)

	if maxNumSubLayersMinus1 > 0 {
		subLayerProfilePresent := make([]uint8, maxNumSubLayersMinus1)
		subLayerLevelPresent := make([]uint8, maxNumSubLayersMinus1)

		for i := uint32(0); i < maxNumSubLayersMinus1; i++ {
			pp, err := br.readBit()
			if err != nil {
				return err
			}
			subLayerProfilePresent[i] = pp

			lp, err := br.readBit()
			if err != nil {
				return err
			}
			subLayerLevelPresent[i] = lp
		}

		if maxNumSubLayersMinus1 < 8 {
			br.skip(int(8-maxNumSubLayersMinus1) * 2)
		}

		for i := uint32(0); i < maxNumSubLayersMinus1; i++ {
			if subLayerProfilePresent[i] == 1 {
				br.skip(88)
			}
			if subLayerLevelPresent[i] == 1 {
				br.skip(8)
			}
		}
	}

	return nil
}

func ParseH265SPS(sps []byte) *H265SPSInfo {
	if len(sps) < 4 {
		return nil
	}

	br := newBitReader(sps[2:])

	if _, err := br.readBits(4); err != nil {
		return nil
	}

	maxSubLayers, err := br.readBits(3)
	if err != nil {
		return nil
	}

	if _, err := br.readBit(); err != nil {
		return nil
	}

	if err := skipH265ProfileTierLevel(br, true, maxSubLayers); err != nil {
		return nil
	}

	if _, err := br.readUE(); err != nil {
		return nil
	}

	chromaFmt, err := br.readUE()
	if err != nil {
		return nil
	}

	if chromaFmt == 3 {
		if _, err := br.readBit(); err != nil {
			return nil
		}
	}

	width, err := br.readUE()
	if err != nil {
		return nil
	}

	height, err := br.readUE()
	if err != nil {
		return nil
	}

	confWin, err := br.readBit()
	if err != nil {
		return nil
	}
	if confWin == 1 {
		for i := 0; i < 4; i++ {
			if _, err := br.readUE(); err != nil {
				return nil
			}
		}
	}

	bdl, err := br.readUE()
	if err != nil {
		return nil
	}

	bdc, err := br.readUE()
	if err != nil {
		return nil
	}

	return &H265SPSInfo{
		ChromaFormatIDC: chromaFmt,
		BitDepthLuma:    bdl + 8,
		BitDepthChroma:  bdc + 8,
		Width:           width,
		Height:          height,
	}
}
