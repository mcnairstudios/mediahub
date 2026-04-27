package demuxloop

import (
	"context"
	"fmt"
	"io"

	"github.com/mcnairstudios/mediahub/pkg/av"
)

type PacketReader interface {
	ReadPacket() (*av.Packet, error)
}

type Config struct {
	Reader PacketReader
	Sink   av.PacketSink
}

func Run(ctx context.Context, cfg Config) error {
	reader := cfg.Reader
	sink := cfg.Sink

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		pkt, err := reader.ReadPacket()
		if err != nil {
			if err == io.EOF {
				sink.EndOfStream()
				return nil
			}
			return fmt.Errorf("demuxloop: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}

		switch pkt.Type {
		case av.Video:
			if err := sink.PushVideo(pkt.Data, pkt.PTS, pkt.DTS, pkt.Keyframe); err != nil {
				return fmt.Errorf("demuxloop: push video: %w", err)
			}
		case av.Audio:
			if err := sink.PushAudio(pkt.Data, pkt.PTS, pkt.DTS); err != nil {
				return fmt.Errorf("demuxloop: push audio: %w", err)
			}
		case av.Subtitle:
			if err := sink.PushSubtitle(pkt.Data, pkt.PTS, pkt.Duration); err != nil {
				return fmt.Errorf("demuxloop: push subtitle: %w", err)
			}
		}
	}
}
