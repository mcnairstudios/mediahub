package scale

import (
	"github.com/asticode/go-astiav"
)

type Scaler struct {
	swsCtx *astiav.SoftwareScaleContext
	dstW   int
	dstH   int
	dstFmt astiav.PixelFormat
}

func NewScaler(srcW, srcH int, srcFmt astiav.PixelFormat,
	dstW, dstH int, dstFmt astiav.PixelFormat) (*Scaler, error) {

	flags := astiav.NewSoftwareScaleContextFlags(astiav.SoftwareScaleContextFlagBilinear)
	ctx, err := astiav.CreateSoftwareScaleContext(srcW, srcH, srcFmt, dstW, dstH, dstFmt, flags)
	if err != nil {
		return nil, err
	}
	return &Scaler{
		swsCtx: ctx,
		dstW:   dstW,
		dstH:   dstH,
		dstFmt: dstFmt,
	}, nil
}

func (s *Scaler) Scale(src *astiav.Frame) (*astiav.Frame, error) {
	dst := astiav.AllocFrame()
	dst.SetWidth(s.dstW)
	dst.SetHeight(s.dstH)
	dst.SetPixelFormat(s.dstFmt)
	if err := dst.AllocBuffer(0); err != nil {
		dst.Free()
		return nil, err
	}
	if err := s.swsCtx.ScaleFrame(src, dst); err != nil {
		dst.Free()
		return nil, err
	}
	return dst, nil
}

func (s *Scaler) Close() {
	if s.swsCtx != nil {
		s.swsCtx.Free()
		s.swsCtx = nil
	}
}
