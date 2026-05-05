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
	outVideo := media.NormalizeVideoCodec(out.VideoCodec)
	outAudio := media.NormalizeAudioCodec(out.AudioCodec)

	d := Decision{
		Container: media.Container(out.Container),
	}

	needsTranscode := false

	isDefaultOrCopy := outVideo == media.VideoCopy || outVideo == "" || outVideo == "default"
	inputUnknown := srcVideo == "" || in.VideoCodec == ""

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
		needsTranscode = true
	}

	if inputUnknown && isDefaultOrCopy {
		needsTranscode = false
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

	if outAudio == media.AudioCopy {
		d.AudioCodec = media.AudioCopy
	} else if outAudio == "" || outAudio == "default" {
		d.NeedsAudioTranscode = true
		d.AudioCodec = media.AudioAAC
	} else {
		d.NeedsAudioTranscode = true
		d.AudioCodec = outAudio
	}

	if d.Container == media.ContainerWebM && d.AudioCodec != media.AudioOpus && d.AudioCodec != media.AudioCopy {
		d.NeedsAudioTranscode = true
		d.AudioCodec = media.AudioOpus
	}

	return d
}
