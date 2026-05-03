package resample

import (
	"errors"
	"fmt"

	"github.com/asticode/go-astiav"
)

type Resampler struct {
	swrCtx    *astiav.SoftwareResampleContext
	dstFrame  *astiav.Frame
	dstLayout astiav.ChannelLayout
	dstRate   int
	dstFmt    astiav.SampleFormat
	nextPts   int64
	ptsInited bool
}

func channelLayoutForCount(channels int) (astiav.ChannelLayout, error) {
	switch channels {
	case 1:
		return astiav.ChannelLayoutMono, nil
	case 2:
		return astiav.ChannelLayoutStereo, nil
	case 6:
		return astiav.ChannelLayout5Point1, nil
	case 8:
		return astiav.ChannelLayout7Point1, nil
	default:
		return astiav.ChannelLayout{}, fmt.Errorf("resample: unsupported channel count %d", channels)
	}
}

func NewResampler(srcChannels, srcRate int, srcFmt astiav.SampleFormat,
	dstChannels, dstRate int, dstFmt astiav.SampleFormat) (*Resampler, error) {

	dstLayout, err := channelLayoutForCount(dstChannels)
	if err != nil {
		return nil, fmt.Errorf("resample: destination: %w", err)
	}

	ctx := astiav.AllocSoftwareResampleContext()
	if ctx == nil {
		return nil, fmt.Errorf("resample: failed to allocate SoftwareResampleContext")
	}

	dstFrame := astiav.AllocFrame()
	if dstFrame == nil {
		ctx.Free()
		return nil, fmt.Errorf("resample: failed to allocate destination frame")
	}
	dstFrame.SetChannelLayout(dstLayout)
	dstFrame.SetSampleFormat(dstFmt)
	dstFrame.SetSampleRate(dstRate)
	dstFrame.SetNbSamples(1024)
	if err := dstFrame.AllocBuffer(0); err != nil {
		dstFrame.Free()
		ctx.Free()
		return nil, fmt.Errorf("resample: alloc destination buffer: %w", err)
	}

	return &Resampler{
		swrCtx:    ctx,
		dstFrame:  dstFrame,
		dstLayout: dstLayout,
		dstRate:   dstRate,
		dstFmt:    dstFmt,
	}, nil
}

func (r *Resampler) Convert(src *astiav.Frame) (*astiav.Frame, error) {
	if src != nil && !r.ptsInited {
		r.nextPts = src.Pts()
		r.ptsInited = true
	}
	if err := r.swrCtx.ConvertFrame(src, r.dstFrame); err != nil {
		if errors.Is(err, astiav.ErrInputChanged) {
			r.swrCtx.Free()
			r.swrCtx = astiav.AllocSoftwareResampleContext()
			if r.swrCtx == nil {
				return nil, fmt.Errorf("resample: failed to reallocate after input change")
			}
			if retryErr := r.swrCtx.ConvertFrame(src, r.dstFrame); retryErr != nil {
				return nil, fmt.Errorf("resample: convert frame after input change: %w", retryErr)
			}
		} else {
			return nil, fmt.Errorf("resample: convert frame: %w", err)
		}
	}

	if r.dstFrame.NbSamples() == 0 {
		return nil, nil
	}

	out := astiav.AllocFrame()
	if out == nil {
		return nil, fmt.Errorf("resample: failed to allocate output frame")
	}
	if err := out.Ref(r.dstFrame); err != nil {
		out.Free()
		return nil, fmt.Errorf("resample: ref frame: %w", err)
	}
	out.SetPts(r.nextPts)
	r.nextPts += int64(out.NbSamples())
	return out, nil
}

func (r *Resampler) Flush() ([]*astiav.Frame, error) {
	var frames []*astiav.Frame
	for {
		if err := r.swrCtx.ConvertFrame(nil, r.dstFrame); err != nil {
			return frames, fmt.Errorf("resample: flush convert: %w", err)
		}
		if r.dstFrame.NbSamples() == 0 {
			break
		}
		out := astiav.AllocFrame()
		if out == nil {
			return frames, fmt.Errorf("resample: failed to allocate flushed frame")
		}
		if err := out.Ref(r.dstFrame); err != nil {
			out.Free()
			return frames, fmt.Errorf("resample: ref flushed frame: %w", err)
		}
		out.SetPts(r.nextPts)
		r.nextPts += int64(out.NbSamples())
		frames = append(frames, out)
	}
	return frames, nil
}

func (r *Resampler) Delay() int64 {
	return r.swrCtx.Delay(int64(r.dstRate))
}

func (r *Resampler) Reset() {
	if r.swrCtx != nil {
		r.swrCtx.Free()
	}
	r.swrCtx = astiav.AllocSoftwareResampleContext()
	r.ptsInited = false
}

func (r *Resampler) Close() {
	if r.dstFrame != nil {
		r.dstFrame.Free()
		r.dstFrame = nil
	}
	if r.swrCtx != nil {
		r.swrCtx.Free()
		r.swrCtx = nil
	}
}
