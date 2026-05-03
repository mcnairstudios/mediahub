package filter

import (
	"errors"
	"fmt"
	"strings"

	"github.com/asticode/go-astiav"
	"github.com/rs/zerolog/log"
)

type Deinterlacer struct {
	graph      *astiav.FilterGraph
	bufferSrc  *astiav.BuffersrcFilterContext
	bufferSink *astiav.BuffersinkFilterContext
	hwAccel    string
}

type DeinterlaceOpts struct {
	Width       int
	Height      int
	PixFmt      astiav.PixelFormat
	TimeBase    astiav.Rational
	HWAccel     string
	HWDeviceCtx *astiav.HardwareDeviceContext
	HWFramesCtx *astiav.HardwareFramesContext
}

func NewDeinterlacer(width, height int, pixFmt astiav.PixelFormat, timeBase astiav.Rational) (*Deinterlacer, error) {
	return NewDeinterlacerWithOpts(DeinterlaceOpts{
		Width:    width,
		Height:   height,
		PixFmt:   pixFmt,
		TimeBase: timeBase,
	})
}

func NewDeinterlacerWithOpts(opts DeinterlaceOpts) (*Deinterlacer, error) {
	if opts.HWAccel == "videotoolbox" && opts.HWDeviceCtx != nil && opts.HWFramesCtx != nil {
		d, err := newHWDeinterlacer(opts)
		if err != nil {
			log.Warn().Err(err).Str("hwaccel", opts.HWAccel).Msg("filter: HW deinterlacer failed, falling back to software")
			return newSWDeinterlacer(opts)
		}
		return d, nil
	}
	return newSWDeinterlacer(opts)
}

func newHWDeinterlacer(opts DeinterlaceOpts) (*Deinterlacer, error) {
	filterStr := "yadif_videotoolbox=mode=send_frame:parity=auto:deint=interlaced,scale_vt=format=nv12"

	graph := astiav.AllocFilterGraph()
	if graph == nil {
		return nil, errors.New("filter: failed to allocate filter graph")
	}

	buffersrc := astiav.FindFilterByName("buffer")
	if buffersrc == nil {
		graph.Free()
		return nil, errors.New("filter: buffer filter not found")
	}
	buffersink := astiav.FindFilterByName("buffersink")
	if buffersink == nil {
		graph.Free()
		return nil, errors.New("filter: buffersink filter not found")
	}

	srcCtx, err := graph.NewBuffersrcFilterContext(buffersrc, "in")
	if err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: creating buffersrc: %w", err)
	}
	sinkCtx, err := graph.NewBuffersinkFilterContext(buffersink, "out")
	if err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: creating buffersink: %w", err)
	}

	params := astiav.AllocBuffersrcFilterContextParameters()
	defer params.Free()
	params.SetWidth(opts.Width)
	params.SetHeight(opts.Height)
	params.SetPixelFormat(opts.PixFmt)
	params.SetTimeBase(opts.TimeBase)
	params.SetHardwareFramesContext(opts.HWFramesCtx)

	if err := srcCtx.SetParameters(params); err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: setting buffersrc params: %w", err)
	}
	if err := srcCtx.Initialize(nil); err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: initializing buffersrc: %w", err)
	}

	outputs := astiav.AllocFilterInOut()
	if outputs == nil {
		graph.Free()
		return nil, errors.New("filter: failed to allocate filter outputs")
	}
	defer outputs.Free()
	outputs.SetName("in")
	outputs.SetFilterContext(srcCtx.FilterContext())
	outputs.SetPadIdx(0)
	outputs.SetNext(nil)

	inputs := astiav.AllocFilterInOut()
	if inputs == nil {
		graph.Free()
		return nil, errors.New("filter: failed to allocate filter inputs")
	}
	defer inputs.Free()
	inputs.SetName("out")
	inputs.SetFilterContext(sinkCtx.FilterContext())
	inputs.SetPadIdx(0)
	inputs.SetNext(nil)

	for _, f := range graph.Filters() {
		if f.Filter().Flags().Has(astiav.FilterFlagHardwareDevice) {
			f.SetHardwareDeviceContext(opts.HWDeviceCtx)
		}
	}

	if err := graph.Parse(filterStr, inputs, outputs); err != nil {
		graph.Free()
		if strings.Contains(err.Error(), "No such filter") {
			return nil, fmt.Errorf("filter: HW filter not available: %w", err)
		}
		return nil, fmt.Errorf("filter: parsing HW filter: %w", err)
	}

	for _, f := range graph.Filters() {
		if f.Filter().Flags().Has(astiav.FilterFlagHardwareDevice) {
			f.SetHardwareDeviceContext(opts.HWDeviceCtx)
		}
	}

	if err := graph.Configure(); err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: configuring HW graph: %w", err)
	}

	log.Info().Str("filter", filterStr).Msg("filter: GPU deinterlacer+scaler initialized")

	return &Deinterlacer{
		graph:      graph,
		bufferSrc:  srcCtx,
		bufferSink: sinkCtx,
		hwAccel:    opts.HWAccel,
	}, nil
}

func newSWDeinterlacer(opts DeinterlaceOpts) (*Deinterlacer, error) {
	graph := astiav.AllocFilterGraph()
	if graph == nil {
		return nil, errors.New("filter: failed to allocate filter graph")
	}

	buffersrc := astiav.FindFilterByName("buffer")
	if buffersrc == nil {
		graph.Free()
		return nil, errors.New("filter: buffer filter not found")
	}
	buffersink := astiav.FindFilterByName("buffersink")
	if buffersink == nil {
		graph.Free()
		return nil, errors.New("filter: buffersink filter not found")
	}

	srcCtx, err := graph.NewBuffersrcFilterContext(buffersrc, "in")
	if err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: creating buffersrc: %w", err)
	}
	sinkCtx, err := graph.NewBuffersinkFilterContext(buffersink, "out")
	if err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: creating buffersink: %w", err)
	}

	params := astiav.AllocBuffersrcFilterContextParameters()
	defer params.Free()
	params.SetWidth(opts.Width)
	params.SetHeight(opts.Height)
	params.SetPixelFormat(opts.PixFmt)
	params.SetTimeBase(opts.TimeBase)
	if err := srcCtx.SetParameters(params); err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: setting buffersrc params: %w", err)
	}
	if err := srcCtx.Initialize(nil); err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: initializing buffersrc: %w", err)
	}

	outputs := astiav.AllocFilterInOut()
	if outputs == nil {
		graph.Free()
		return nil, errors.New("filter: failed to allocate filter outputs")
	}
	defer outputs.Free()
	outputs.SetName("in")
	outputs.SetFilterContext(srcCtx.FilterContext())
	outputs.SetPadIdx(0)
	outputs.SetNext(nil)

	inputs := astiav.AllocFilterInOut()
	if inputs == nil {
		graph.Free()
		return nil, errors.New("filter: failed to allocate filter inputs")
	}
	defer inputs.Free()
	inputs.SetName("out")
	inputs.SetFilterContext(sinkCtx.FilterContext())
	inputs.SetPadIdx(0)
	inputs.SetNext(nil)

	if err := graph.Parse("yadif=mode=send_frame:parity=auto:deint=interlaced", inputs, outputs); err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: parsing yadif filter: %w", err)
	}
	if err := graph.Configure(); err != nil {
		graph.Free()
		return nil, fmt.Errorf("filter: configuring graph: %w", err)
	}

	return &Deinterlacer{
		graph:      graph,
		bufferSrc:  srcCtx,
		bufferSink: sinkCtx,
	}, nil
}

func (d *Deinterlacer) Process(frame *astiav.Frame) (*astiav.Frame, error) {
	if err := d.bufferSrc.AddFrame(frame, astiav.NewBuffersrcFlags(astiav.BuffersrcFlagKeepRef)); err != nil {
		return nil, fmt.Errorf("filter: adding frame to buffersrc: %w", err)
	}
	out := astiav.AllocFrame()
	if err := d.bufferSink.GetFrame(out, astiav.NewBuffersinkFlags()); err != nil {
		out.Free()
		if errors.Is(err, astiav.ErrEagain) {
			return nil, nil
		}
		return nil, fmt.Errorf("filter: getting frame from buffersink: %w", err)
	}
	return out, nil
}

func (d *Deinterlacer) Close() {
	if d.graph != nil {
		d.graph.Free()
		d.graph = nil
	}
}
