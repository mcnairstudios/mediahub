package demuxloop

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/av"
)

type mockDemuxer struct {
	packets []*av.Packet
	idx     int
	err     error
}

func (m *mockDemuxer) ReadPacket() (*av.Packet, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.idx >= len(m.packets) {
		return nil, io.EOF
	}
	pkt := m.packets[m.idx]
	m.idx++
	return pkt, nil
}

type received struct {
	typ      av.StreamType
	data     []byte
	pts      int64
	dts      int64
	keyframe bool
	duration int64
}

type mockSink struct {
	mu       sync.Mutex
	items    []received
	eos      bool
	pushErr  error
}

func (s *mockSink) PushVideo(data []byte, pts, dts int64, keyframe bool) error {
	if s.pushErr != nil {
		return s.pushErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, received{typ: av.Video, data: data, pts: pts, dts: dts, keyframe: keyframe})
	return nil
}

func (s *mockSink) PushAudio(data []byte, pts, dts int64) error {
	if s.pushErr != nil {
		return s.pushErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, received{typ: av.Audio, data: data, pts: pts, dts: dts})
	return nil
}

func (s *mockSink) PushSubtitle(data []byte, pts int64, duration int64) error {
	if s.pushErr != nil {
		return s.pushErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, received{typ: av.Subtitle, data: data, pts: pts, duration: duration})
	return nil
}

func (s *mockSink) EndOfStream() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eos = true
}

func TestRunEOFCallsEndOfStream(t *testing.T) {
	dm := &mockDemuxer{packets: []*av.Packet{
		{Type: av.Video, Data: []byte{1}, PTS: 100, DTS: 100, Keyframe: true},
		{Type: av.Audio, Data: []byte{2}, PTS: 200, DTS: 200},
	}}
	sink := &mockSink{}

	err := Run(context.Background(), Config{Reader: dm, Sink: sink})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !sink.eos {
		t.Fatal("expected EndOfStream to be called")
	}
	if len(sink.items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(sink.items))
	}
	if sink.items[0].typ != av.Video {
		t.Fatalf("expected first item to be Video, got %v", sink.items[0].typ)
	}
	if sink.items[1].typ != av.Audio {
		t.Fatalf("expected second item to be Audio, got %v", sink.items[1].typ)
	}
}

func TestRunContextCancel(t *testing.T) {
	dm := &mockDemuxer{packets: make([]*av.Packet, 1000)}
	for i := range dm.packets {
		dm.packets[i] = &av.Packet{Type: av.Video, Data: []byte{0}, PTS: int64(i), DTS: int64(i)}
	}
	sink := &mockSink{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(ctx, Config{Reader: dm, Sink: sink})
	if err != nil {
		t.Fatalf("expected nil on cancel, got %v", err)
	}
	if sink.eos {
		t.Fatal("EndOfStream should not be called on context cancel")
	}
}

func TestRunReaderError(t *testing.T) {
	dm := &mockDemuxer{err: fmt.Errorf("network failure")}
	sink := &mockSink{}

	err := Run(context.Background(), Config{Reader: dm, Sink: sink})
	if err == nil {
		t.Fatal("expected error")
	}
	if sink.eos {
		t.Fatal("EndOfStream should not be called on error")
	}
}

func TestRunSinkPushError(t *testing.T) {
	dm := &mockDemuxer{packets: []*av.Packet{
		{Type: av.Video, Data: []byte{1}, PTS: 100, DTS: 100},
	}}
	sink := &mockSink{pushErr: fmt.Errorf("sink full")}

	err := Run(context.Background(), Config{Reader: dm, Sink: sink})
	if err == nil {
		t.Fatal("expected error from sink push")
	}
}

func TestRunSubtitlePackets(t *testing.T) {
	dm := &mockDemuxer{packets: []*av.Packet{
		{Type: av.Subtitle, Data: []byte("hello"), PTS: 500, Duration: 3000},
	}}
	sink := &mockSink{}

	err := Run(context.Background(), Config{Reader: dm, Sink: sink})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sink.items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(sink.items))
	}
	item := sink.items[0]
	if item.typ != av.Subtitle {
		t.Fatalf("expected Subtitle, got %v", item.typ)
	}
	if item.pts != 500 {
		t.Fatalf("expected PTS 500, got %d", item.pts)
	}
	if item.duration != 3000 {
		t.Fatalf("expected duration 3000, got %d", item.duration)
	}
}

func TestRunMixedPackets(t *testing.T) {
	dm := &mockDemuxer{packets: []*av.Packet{
		{Type: av.Video, Data: []byte{1}, PTS: 0, DTS: 0, Keyframe: true},
		{Type: av.Audio, Data: []byte{2}, PTS: 10, DTS: 10},
		{Type: av.Video, Data: []byte{3}, PTS: 33, DTS: 33, Keyframe: false},
		{Type: av.Subtitle, Data: []byte("sub"), PTS: 40, Duration: 1000},
		{Type: av.Audio, Data: []byte{4}, PTS: 50, DTS: 50},
	}}
	sink := &mockSink{}

	err := Run(context.Background(), Config{Reader: dm, Sink: sink})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sink.items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(sink.items))
	}
	if !sink.items[0].keyframe {
		t.Fatal("first video packet should be a keyframe")
	}
	if sink.items[2].keyframe {
		t.Fatal("third packet should not be a keyframe")
	}
}

func TestRunEmptyStream(t *testing.T) {
	dm := &mockDemuxer{}
	sink := &mockSink{}

	err := Run(context.Background(), Config{Reader: dm, Sink: sink})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sink.eos {
		t.Fatal("expected EndOfStream on empty stream")
	}
	if len(sink.items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(sink.items))
	}
}

func TestRunCancelDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	blockingDm := &blockingDemuxer{cancel: cancel}
	sink := &mockSink{}

	err := Run(ctx, Config{Reader: blockingDm, Sink: sink})
	if err != nil {
		t.Fatalf("expected nil on cancel, got %v", err)
	}
}

type blockingDemuxer struct {
	cancel context.CancelFunc
	called bool
}

func (b *blockingDemuxer) ReadPacket() (*av.Packet, error) {
	if !b.called {
		b.called = true
		return &av.Packet{Type: av.Video, Data: []byte{1}, PTS: 0, DTS: 0}, nil
	}
	b.cancel()
	time.Sleep(10 * time.Millisecond)
	return &av.Packet{Type: av.Video, Data: []byte{2}, PTS: 1, DTS: 1}, nil
}

func TestRunAudioPushError(t *testing.T) {
	dm := &mockDemuxer{packets: []*av.Packet{
		{Type: av.Audio, Data: []byte{1}, PTS: 100, DTS: 100},
	}}
	sink := &mockSink{pushErr: fmt.Errorf("audio push failed")}

	err := Run(context.Background(), Config{Reader: dm, Sink: sink})
	if err == nil {
		t.Fatal("expected error from audio push")
	}
}

func TestRunSubtitlePushError(t *testing.T) {
	dm := &mockDemuxer{packets: []*av.Packet{
		{Type: av.Subtitle, Data: []byte("sub"), PTS: 100, Duration: 500},
	}}
	sink := &mockSink{pushErr: fmt.Errorf("subtitle push failed")}

	err := Run(context.Background(), Config{Reader: dm, Sink: sink})
	if err == nil {
		t.Fatal("expected error from subtitle push")
	}
}
