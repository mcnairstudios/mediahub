package record

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/conv"
	"github.com/mcnairstudios/mediahub/pkg/output"
)

type Plugin struct {
	filePath      string
	fc            *astiav.FormatContext
	ioCtx         *astiav.IOContext
	videoStream   *astiav.Stream
	audioStream   *astiav.Stream
	videoTB       astiav.Rational
	audioTB       astiav.Rational
	headerWritten bool
	bytesWritten  atomic.Int64
	preserved     atomic.Bool
	stopped       bool
	mu            sync.Mutex
}

func New(cfg output.PluginConfig) (*Plugin, error) {
	if cfg.OutputFilePath == "" {
		return nil, errors.New("record: OutputFilePath is required")
	}
	if cfg.Video == nil {
		return nil, errors.New("record: Video info is required")
	}
	if cfg.Audio == nil {
		return nil, errors.New("record: Audio info is required")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.OutputFilePath), 0755); err != nil {
		return nil, fmt.Errorf("record: create directory: %w", err)
	}

	format := cfg.OutputFormat
	if format == "" {
		format = "mpegts"
	}
	fc, err := astiav.AllocOutputFormatContext(nil, format, "")
	if err != nil {
		return nil, fmt.Errorf("record: alloc output format: %w", err)
	}

	ioCtx, err := astiav.OpenIOContext(cfg.OutputFilePath,
		astiav.NewIOContextFlags(astiav.IOContextFlagWrite), nil, nil)
	if err != nil {
		fc.Free()
		return nil, fmt.Errorf("record: open io context: %w", err)
	}
	fc.SetPb(ioCtx)

	videoCP, err := conv.CodecParamsFromVideoProbe(cfg.Video)
	if err != nil {
		ioCtx.Close()
		fc.Free()
		return nil, fmt.Errorf("record: video codec params: %w", err)
	}

	vs := fc.NewStream(nil)
	if vs == nil {
		videoCP.Free()
		ioCtx.Close()
		fc.Free()
		return nil, errors.New("record: failed to allocate video stream")
	}
	if err := videoCP.Copy(vs.CodecParameters()); err != nil {
		videoCP.Free()
		ioCtx.Close()
		fc.Free()
		return nil, fmt.Errorf("record: copy video params: %w", err)
	}
	videoCP.Free()
	vs.SetTimeBase(astiav.NewRational(1, 90000))

	audioCP, err := conv.CodecParamsFromAudioProbe(cfg.Audio)
	if err != nil {
		ioCtx.Close()
		fc.Free()
		return nil, fmt.Errorf("record: audio codec params: %w", err)
	}

	as := fc.NewStream(nil)
	if as == nil {
		audioCP.Free()
		ioCtx.Close()
		fc.Free()
		return nil, errors.New("record: failed to allocate audio stream")
	}
	if err := audioCP.Copy(as.CodecParameters()); err != nil {
		audioCP.Free()
		ioCtx.Close()
		fc.Free()
		return nil, fmt.Errorf("record: copy audio params: %w", err)
	}
	audioCP.Free()
	if as.CodecParameters().CodecID() == astiav.CodecIDAac {
		as.CodecParameters().SetFrameSize(1024)
	}

	sampleRate := cfg.Audio.SampleRate
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	as.SetTimeBase(astiav.NewRational(1, sampleRate))

	if err := fc.WriteHeader(nil); err != nil {
		ioCtx.Close()
		fc.Free()
		return nil, fmt.Errorf("record: write header: %w", err)
	}

	return &Plugin{
		filePath:      cfg.OutputFilePath,
		fc:            fc,
		ioCtx:         ioCtx,
		videoStream:   vs,
		audioStream:   as,
		videoTB:       vs.TimeBase(),
		audioTB:       as.TimeBase(),
		headerWritten: true,
	}, nil
}

func (p *Plugin) Mode() output.DeliveryMode {
	return output.DeliveryRecord
}

func (p *Plugin) PushVideo(data []byte, pts, dts int64, keyframe bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return errors.New("record: plugin stopped")
	}

	pkt, err := conv.ToAVPacket(&av.Packet{
		Data:     data,
		PTS:      pts,
		DTS:      dts,
		Keyframe: keyframe,
	}, p.videoTB)
	if err != nil {
		return err
	}
	defer pkt.Free()

	pkt.SetStreamIndex(p.videoStream.Index())
	if err := p.fc.WriteInterleavedFrame(pkt); err != nil {
		return fmt.Errorf("record: write video: %w", err)
	}
	p.updateBytesWritten()
	return nil
}

func (p *Plugin) PushAudio(data []byte, pts, dts int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return errors.New("record: plugin stopped")
	}

	pkt, err := conv.ToAVPacket(&av.Packet{
		Data: data,
		PTS:  pts,
		DTS:  dts,
	}, p.audioTB)
	if err != nil {
		return err
	}
	defer pkt.Free()

	pkt.SetStreamIndex(p.audioStream.Index())
	if err := p.fc.WriteInterleavedFrame(pkt); err != nil {
		return fmt.Errorf("record: write audio: %w", err)
	}
	p.updateBytesWritten()
	return nil
}

func (p *Plugin) PushSubtitle(_ []byte, _ int64, _ int64) error {
	return nil
}

func (p *Plugin) EndOfStream() {
	p.Stop()
}

func (p *Plugin) ResetForSeek() {}

func (p *Plugin) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	p.stopped = true

	if p.fc != nil && p.headerWritten {
		p.fc.WriteTrailer() //nolint:errcheck
	}
	if p.fc != nil {
		p.fc.Free()
		p.fc = nil
	}
	if p.ioCtx != nil {
		p.ioCtx.Close() //nolint:errcheck
		p.ioCtx = nil
	}
	p.updateBytesWritten()
}

func (p *Plugin) Status() output.PluginStatus {
	p.mu.Lock()
	stopped := p.stopped
	p.mu.Unlock()
	return output.PluginStatus{
		Mode:         output.DeliveryRecord,
		BytesWritten: p.bytesWritten.Load(),
		Healthy:      !stopped,
	}
}

func (p *Plugin) FilePath() string {
	return p.filePath
}

func (p *Plugin) FileSize() int64 {
	return p.bytesWritten.Load()
}

func (p *Plugin) SetPreserved(v bool) {
	p.preserved.Store(v)
}

func (p *Plugin) IsPreserved() bool {
	return p.preserved.Load()
}

func (p *Plugin) updateBytesWritten() {
	info, err := os.Stat(p.filePath)
	if err == nil {
		p.bytesWritten.Store(info.Size())
	}
}
