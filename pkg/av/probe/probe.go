package probe

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av/extradata"
	"github.com/mcnairstudios/mediahub/pkg/media"
)

func Probe(url string, timeoutSec int) (*media.ProbeResult, error) {
	fc := astiav.AllocFormatContext()
	if fc == nil {
		return nil, fmt.Errorf("avprobe: failed to allocate format context")
	}
	defer fc.Free()

	var d *astiav.Dictionary
	if timeoutSec > 0 {
		d = astiav.NewDictionary()
		defer d.Free()
		d.Set("timeout", fmt.Sprintf("%d", timeoutSec*1_000_000), 0)
	}

	if err := fc.OpenInput(url, nil, d); err != nil {
		return nil, fmt.Errorf("avprobe: open input %q: %w", url, err)
	}
	defer fc.CloseInput()

	if err := fc.FindStreamInfo(nil); err != nil {
		return nil, fmt.Errorf("avprobe: find stream info: %w", err)
	}

	return ExtractProbeResult(fc), nil
}

func ExtractProbeResult(fc *astiav.FormatContext) *media.ProbeResult {
	result := &media.ProbeResult{
		DurationMs: fc.Duration() / 1000,
	}

	for _, s := range fc.Streams() {
		cp := s.CodecParameters()
		if cp.CodecID() == astiav.CodecIDNone {
			continue
		}
		switch cp.MediaType() {
		case astiav.MediaTypeVideo:
			if result.Video != nil {
				continue
			}
			fr := s.RFrameRate()
			ext := cp.ExtraData()
			result.Video = &media.VideoInfo{
				Codec:      cp.CodecID().String(),
				Width:      cp.Width(),
				Height:     cp.Height(),
				Interlaced: detectInterlaced(cp.CodecID().String(), ext),
				BitDepth:   bitDepthFromPixelFormat(cp.PixelFormat()),
				FramerateN: fr.Num(),
				FramerateD: fr.Den(),
				Extradata:  ext,
				Profile:    fmt.Sprintf("%d", int(cp.Profile())),
				PixFmt:     pixFmtName(cp.PixelFormat()),
			}
		case astiav.MediaTypeAudio:
			result.AudioTracks = append(result.AudioTracks, media.AudioTrack{
				Index:      s.Index(),
				Codec:      cp.CodecID().String(),
				Channels:   cp.ChannelLayout().Channels(),
				SampleRate: cp.SampleRate(),
				Language:   metadataValue(s.Metadata(), "language"),
				IsAD:       s.DispositionFlags().Has(astiav.DispositionFlagVisualImpaired),
				BitRate:    int(cp.BitRate()),
			})
		}
	}

	return result
}

func detectInterlaced(codec string, ext []byte) bool {
	if codec != "h264" || len(ext) == 0 {
		return false
	}
	var spsData []byte
	if ext[0] == 0x01 && len(ext) > 8 {
		numSPS := int(ext[5] & 0x1F)
		if numSPS > 0 && len(ext) > 8 {
			spsLen := int(ext[6])<<8 | int(ext[7])
			if len(ext) >= 8+spsLen {
				spsData = ext[8 : 8+spsLen]
			}
		}
	} else {
		nalus := extradata.SplitNALUnits(ext)
		for _, nalu := range nalus {
			if len(nalu) > 0 && (nalu[0]&0x1F) == 7 {
				spsData = nalu
				break
			}
		}
	}
	if spsData == nil {
		return false
	}
	info := extradata.ParseH264SPS(spsData)
	if info == nil {
		return false
	}
	return !info.FrameMBSOnlyFlag
}

var bitDepthRe = regexp.MustCompile(`(\d+)(le|be)?$`)

func bitDepthFromPixelFormat(pf astiav.PixelFormat) int {
	desc := pf.Descriptor()
	if desc == nil {
		return 8
	}
	name := desc.Name()
	if m := bitDepthRe.FindStringSubmatch(name); m != nil {
		if bits, err := strconv.Atoi(m[1]); err == nil && bits > 8 && bits <= 16 {
			return bits
		}
	}
	return 8
}

func metadataValue(d *astiav.Dictionary, key string) string {
	if d == nil {
		return ""
	}
	entry := d.Get(key, nil, 0)
	if entry == nil {
		return ""
	}
	return entry.Value()
}

func pixFmtName(pf astiav.PixelFormat) string {
	desc := pf.Descriptor()
	if desc == nil {
		return ""
	}
	return desc.Name()
}
