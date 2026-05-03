package stream

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av"
	"github.com/mcnairstudios/mediahub/pkg/av/conv"
	"github.com/mcnairstudios/mediahub/pkg/av/mux"
	"github.com/mcnairstudios/mediahub/pkg/output"
)

type Plugin struct {
	muxer    *mux.StreamMuxer
	file     *os.File
	filePath string
	videoIdx int
	audioIdx int
	videoTB  astiav.Rational
	audioTB  astiav.Rational
	stopped  bool
	mu       sync.Mutex
	written  atomic.Int64
	lastErr  error
}

func New(cfg output.PluginConfig) (*Plugin, error) {
	if cfg.OutputFilePath == "" {
		return nil, errors.New("stream: OutputFilePath is required")
	}

	format := cfg.OutputFormat
	if format == "" {
		format = "mpegts"
	}

	f, err := os.Create(cfg.OutputFilePath)
	if err != nil {
		return nil, fmt.Errorf("stream: create output file: %w", err)
	}

	cw := &countingWriter{w: f}

	muxer, err := mux.NewStreamMuxer(format, cw)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stream: create muxer: %w", err)
	}

	p := &Plugin{
		muxer:    muxer,
		file:     f,
		filePath: cfg.OutputFilePath,
		videoIdx: -1,
		audioIdx: -1,
	}
	cw.written = &p.written

	var videoStream, audioStream *astiav.Stream

	if cfg.Video != nil || cfg.VideoCodecParams != nil {
		var videoCP *astiav.CodecParameters
		var freeVideoCP bool
		if cfg.VideoCodecParams != nil {
			videoCP = cfg.VideoCodecParams.(*astiav.CodecParameters)
		} else {
			var err error
			videoCP, err = conv.CodecParamsFromVideoProbe(cfg.Video)
			if err != nil {
				muxer.Close()
				f.Close()
				return nil, fmt.Errorf("stream: video codec params: %w", err)
			}
			freeVideoCP = true
		}
		var err error
		videoStream, err = muxer.AddStream(videoCP)
		if err != nil {
			if freeVideoCP {
				videoCP.Free()
			}
			muxer.Close()
			f.Close()
			return nil, fmt.Errorf("stream: add video stream: %w", err)
		}
		if freeVideoCP {
			videoCP.Free()
		}
		p.videoIdx = videoStream.Index()
	}

	if cfg.Audio != nil || cfg.AudioCodecParams != nil {
		var audioCP *astiav.CodecParameters
		var freeAudioCP bool
		if cfg.AudioCodecParams != nil {
			audioCP = cfg.AudioCodecParams.(*astiav.CodecParameters)
		} else {
			var err error
			audioCP, err = conv.CodecParamsFromAudioProbe(cfg.Audio)
			if err != nil {
				muxer.Close()
				f.Close()
				return nil, fmt.Errorf("stream: audio codec params: %w", err)
			}
			freeAudioCP = true
		}
		var err error
		audioStream, err = muxer.AddStream(audioCP)
		if err != nil {
			if freeAudioCP {
				audioCP.Free()
			}
			muxer.Close()
			f.Close()
			return nil, fmt.Errorf("stream: add audio stream: %w", err)
		}
		if freeAudioCP {
			audioCP.Free()
		}
		p.audioIdx = audioStream.Index()
	}

	if err := muxer.WriteHeader(); err != nil {
		muxer.Close()
		f.Close()
		return nil, fmt.Errorf("stream: write header: %w", err)
	}

	if videoStream != nil {
		p.videoTB = videoStream.TimeBase()
	}
	if audioStream != nil {
		p.audioTB = audioStream.TimeBase()
	}

	return p, nil
}

func (p *Plugin) Mode() output.DeliveryMode {
	return output.DeliveryStream
}

func (p *Plugin) PushVideo(data []byte, pts, dts int64, keyframe bool) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: stream PushVideo: %v\n%s", r, debug.Stack())
			retErr = fmt.Errorf("stream: PushVideo panic: %v", r)
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped || p.muxer == nil || p.videoIdx < 0 {
		return nil
	}
	pkt := &av.Packet{Type: av.Video, Data: data, PTS: pts, DTS: dts, Keyframe: keyframe}
	return p.writePacket(pkt, p.videoTB, p.videoIdx)
}

func (p *Plugin) PushAudio(data []byte, pts, dts int64) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: stream PushAudio: %v\n%s", r, debug.Stack())
			retErr = fmt.Errorf("stream: PushAudio panic: %v", r)
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped || p.muxer == nil || p.audioIdx < 0 {
		return nil
	}
	pkt := &av.Packet{Type: av.Audio, Data: data, PTS: pts, DTS: dts}
	return p.writePacket(pkt, p.audioTB, p.audioIdx)
}

func (p *Plugin) PushSubtitle(data []byte, pts int64, duration int64) error {
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
	if p.muxer != nil {
		p.muxer.Close()
	}
	if p.file != nil {
		p.file.Close()
	}
}

func (p *Plugin) Status() output.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	errStr := ""
	if p.lastErr != nil {
		errStr = p.lastErr.Error()
	}
	return output.PluginStatus{
		Mode:         output.DeliveryStream,
		BytesWritten: p.written.Load(),
		Healthy:      !p.stopped,
		Error:        errStr,
	}
}

func (p *Plugin) FilePath() string {
	return p.filePath
}

func (p *Plugin) FileSize() int64 {
	return p.written.Load()
}

func (p *Plugin) writePacket(pkt *av.Packet, tb astiav.Rational, streamIdx int) error {
	avPkt, err := conv.ToAVPacket(pkt, tb)
	if err != nil {
		p.lastErr = err
		return err
	}
	avPkt.SetStreamIndex(streamIdx)
	err = p.muxer.WritePacket(avPkt)
	avPkt.Free()
	if err != nil {
		p.lastErr = err
	}
	return err
}

type countingWriter struct {
	w       *os.File
	written *atomic.Int64
}

func (cw *countingWriter) Write(b []byte) (int, error) {
	n, err := cw.w.Write(b)
	if n > 0 && cw.written != nil {
		cw.written.Add(int64(n))
	}
	return n, err
}
