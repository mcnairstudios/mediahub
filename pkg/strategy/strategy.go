package strategy

import "github.com/mcnairstudios/mediahub/pkg/media"

type Decision struct {
	VideoCodec          media.VideoCodec
	AudioCodec          media.AudioCodec
	Container           media.Container
	NeedsTranscode      bool
	NeedsAudioTranscode bool
	Deinterlace         bool
	HWAccel             string
}

type Input struct {
	VideoCodec string
	AudioCodec string
	Width      int
	Height     int
	Interlaced bool
	BitDepth   int
}

type Output struct {
	VideoCodec   string
	AudioCodec   string
	Container    string
	HWAccel      string
	OutputHeight int
	MaxBitDepth  int
}

func Resolve(in Input, out Output) Decision {
	srcVideo := media.NormalizeVideoCodec(in.VideoCodec)
	srcAudio := media.NormalizeAudioCodec(in.AudioCodec)
	outVideo := media.NormalizeVideoCodec(out.VideoCodec)
	outAudio := media.NormalizeAudioCodec(out.AudioCodec)

	d := Decision{
		Container: media.Container(out.Container),
	}

	needsTranscode := false

	isDefaultOrCopy := outVideo == media.VideoCopy || outVideo == "default"

	if in.Interlaced {
		d.Deinterlace = true
		needsTranscode = true
	}

	if out.MaxBitDepth > 0 && in.BitDepth > out.MaxBitDepth {
		needsTranscode = true
	}

	if out.OutputHeight > 0 && in.Height > out.OutputHeight {
		needsTranscode = true
	}

	if !isDefaultOrCopy {
		if outVideo != srcVideo {
			needsTranscode = true
		}
	}

	d.NeedsTranscode = needsTranscode

	if needsTranscode {
		if isDefaultOrCopy {
			d.VideoCodec = srcVideo
		} else {
			d.VideoCodec = outVideo
		}
		d.HWAccel = out.HWAccel
	} else {
		d.VideoCodec = media.VideoCopy
	}

	isAudioDefaultOrCopy := outAudio == media.AudioCopy || outAudio == "default"

	if isAudioDefaultOrCopy {
		d.AudioCodec = media.AudioCopy
	} else if outAudio == srcAudio {
		d.AudioCodec = media.AudioCopy
	} else {
		d.NeedsAudioTranscode = true
		d.AudioCodec = outAudio
	}

	return d
}
