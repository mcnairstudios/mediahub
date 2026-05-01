package encode

import (
	"fmt"

	"github.com/asticode/go-astiav"
)

type ptsEntry struct {
	pts          int64
	sampleOffset int64
}

type AudioFIFO struct {
	encoder            *Encoder
	fifo               *astiav.AudioFifo
	outFrame           *astiav.Frame
	frameSize          int
	channels           int
	sampleFmt          astiav.SampleFormat
	layout             astiav.ChannelLayout
	rate               int
	totalInputSamples  int64
	totalOutputSamples int64
	ptsQueue           []ptsEntry
}

func NewAudioFIFOFromEncoder(encoder *Encoder, channels int, layout astiav.ChannelLayout, rate int) *AudioFIFO {
	fs := encoder.FrameSize()
	if fs <= 0 {
		fs = 1024
	}
	f := &AudioFIFO{
		encoder:   encoder,
		frameSize: fs,
		channels:  channels,
		sampleFmt: astiav.SampleFormatFltp,
		layout:    layout,
		rate:      rate,
	}
	f.allocOutFrame()
	return f
}

func NewAudioFIFO(encoder *Encoder, frameSize, channels int, sampleFmt astiav.SampleFormat, layout astiav.ChannelLayout, rate int) *AudioFIFO {
	f := &AudioFIFO{
		encoder:   encoder,
		frameSize: frameSize,
		channels:  channels,
		sampleFmt: sampleFmt,
		layout:    layout,
		rate:      rate,
	}
	f.allocOutFrame()
	return f
}

func (f *AudioFIFO) allocOutFrame() {
	frame := astiav.AllocFrame()
	if frame == nil {
		return
	}
	frame.SetNbSamples(f.frameSize)
	frame.SetSampleFormat(f.sampleFmt)
	frame.SetChannelLayout(f.layout)
	frame.SetSampleRate(f.rate)
	if err := frame.AllocBuffer(0); err != nil {
		frame.Free()
		return
	}
	f.outFrame = frame
}

func (f *AudioFIFO) Write(frame *astiav.Frame) ([]*astiav.Packet, error) {
	if f.fifo == nil {
		fifo := astiav.AllocAudioFifo(f.sampleFmt, f.channels, f.frameSize*4)
		if fifo == nil {
			return nil, fmt.Errorf("audiofifo: failed to allocate")
		}
		f.fifo = fifo
	}

	f.ptsQueue = append(f.ptsQueue, ptsEntry{pts: frame.Pts(), sampleOffset: f.totalInputSamples})
	f.totalInputSamples += int64(frame.NbSamples())

	if _, err := f.fifo.Write(frame); err != nil {
		return nil, fmt.Errorf("audiofifo: write: %w", err)
	}

	return f.drainFullFrames(false)
}

func (f *AudioFIFO) Flush() ([]*astiav.Packet, error) {
	if f.fifo == nil || f.fifo.Size() <= 0 {
		return nil, nil
	}
	return f.drainFullFrames(true)
}

func (f *AudioFIFO) drainFullFrames(flush bool) ([]*astiav.Packet, error) {
	if f.outFrame == nil {
		return nil, fmt.Errorf("audiofifo: output frame not allocated")
	}

	var allPkts []*astiav.Packet
	for {
		if flush {
			if f.fifo.Size() <= 0 {
				break
			}
		} else {
			if f.fifo.Size() < f.frameSize {
				break
			}
		}

		readSize := f.frameSize
		if flush && f.fifo.Size() < f.frameSize {
			readSize = f.fifo.Size()
		}
		f.outFrame.SetNbSamples(readSize)

		n, err := f.fifo.Read(f.outFrame)
		if err != nil {
			return allPkts, fmt.Errorf("audiofifo: read: %w", err)
		}
		f.outFrame.SetNbSamples(n)

		useIdx := 0
		for i := 1; i < len(f.ptsQueue); i++ {
			if f.ptsQueue[i].sampleOffset <= f.totalOutputSamples {
				useIdx = i
			} else {
				break
			}
		}
		entry := f.ptsQueue[useIdx]
		f.outFrame.SetPts(entry.pts + (f.totalOutputSamples - entry.sampleOffset))
		f.totalOutputSamples += int64(n)

		if useIdx > 0 {
			f.ptsQueue = f.ptsQueue[useIdx:]
		}

		pkts, err := f.encoder.Encode(f.outFrame)
		if err != nil {
			return allPkts, err
		}
		allPkts = append(allPkts, pkts...)
	}

	return allPkts, nil
}

func (f *AudioFIFO) Reset() {
	if f.fifo != nil {
		f.fifo.Free()
		f.fifo = nil
	}
	f.totalInputSamples = 0
	f.totalOutputSamples = 0
	f.ptsQueue = nil
}

func (f *AudioFIFO) Close() {
	if f.fifo != nil {
		f.fifo.Free()
		f.fifo = nil
	}
	if f.outFrame != nil {
		f.outFrame.Free()
		f.outFrame = nil
	}
}
