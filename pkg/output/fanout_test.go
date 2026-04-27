package output

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestFanOutPushVideoToAllPlugins(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	if err := fan.PushVideo([]byte("frame"), 1000, 1000, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p1.videoPackets != 1 {
		t.Fatalf("p1: expected 1 video packet, got %d", p1.videoPackets)
	}
	if p2.videoPackets != 1 {
		t.Fatalf("p2: expected 1 video packet, got %d", p2.videoPackets)
	}
}

func TestFanOutPushAudioToAllPlugins(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	if err := fan.PushAudio([]byte("audio"), 2000, 2000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p1.audioPackets != 1 {
		t.Fatalf("p1: expected 1 audio packet, got %d", p1.audioPackets)
	}
	if p2.audioPackets != 1 {
		t.Fatalf("p2: expected 1 audio packet, got %d", p2.audioPackets)
	}
}

func TestFanOutPushSubtitleToAllPlugins(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	if err := fan.PushSubtitle([]byte("sub"), 3000, 500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p1.subPackets != 1 {
		t.Fatalf("p1: expected 1 sub packet, got %d", p1.subPackets)
	}
	if p2.subPackets != 1 {
		t.Fatalf("p2: expected 1 sub packet, got %d", p2.subPackets)
	}
}

func TestFanOutOnePluginErrorDoesNotAffectOthers(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p1.err = errors.New("broken")
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	err := fan.PushVideo([]byte("frame"), 1000, 1000, true)
	if err == nil {
		t.Fatal("expected error from fanout")
	}

	if p1.videoPackets != 0 {
		t.Fatalf("p1: expected 0 video packets (errored), got %d", p1.videoPackets)
	}
	if p2.videoPackets != 1 {
		t.Fatalf("p2: expected 1 video packet despite p1 error, got %d", p2.videoPackets)
	}
}

func TestFanOutAddPluginMidStream(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	fan := NewFanOut(p1)

	fan.PushVideo([]byte("frame1"), 1000, 1000, true)

	p2 := newMockPlugin(DeliveryHLS)
	fan.Add(p2)

	fan.PushVideo([]byte("frame2"), 2000, 2000, false)

	if p1.videoPackets != 2 {
		t.Fatalf("p1: expected 2 video packets, got %d", p1.videoPackets)
	}
	if p2.videoPackets != 1 {
		t.Fatalf("p2: expected 1 video packet (added after first push), got %d", p2.videoPackets)
	}
}

func TestFanOutRemovePlugin(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	fan.Remove(DeliveryHLS)

	if p2.stopCount != 1 {
		t.Fatalf("removed plugin should be stopped, stop count: %d", p2.stopCount)
	}

	fan.PushVideo([]byte("frame"), 1000, 1000, true)

	if p1.videoPackets != 1 {
		t.Fatalf("p1: expected 1 video packet, got %d", p1.videoPackets)
	}
	if p2.videoPackets != 0 {
		t.Fatalf("p2: expected 0 video packets after removal, got %d", p2.videoPackets)
	}
}

func TestFanOutStopAllPlugins(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	fan.Stop()

	if p1.stopCount != 1 {
		t.Fatalf("p1: expected 1 stop, got %d", p1.stopCount)
	}
	if p2.stopCount != 1 {
		t.Fatalf("p2: expected 1 stop, got %d", p2.stopCount)
	}
}

func TestFanOutStopPreventsSubsequentPush(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	fan := NewFanOut(p1)

	fan.Stop()

	err := fan.PushVideo([]byte("frame"), 1000, 1000, true)
	if err == nil {
		t.Fatal("expected error pushing to stopped fanout")
	}
	if p1.videoPackets != 0 {
		t.Fatalf("expected 0 packets after stop, got %d", p1.videoPackets)
	}
}

func TestFanOutEndOfStreamPropagated(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	fan.EndOfStream()

	if p1.eosCount != 1 {
		t.Fatalf("p1: expected 1 EOS, got %d", p1.eosCount)
	}
	if p2.eosCount != 1 {
		t.Fatalf("p2: expected 1 EOS, got %d", p2.eosCount)
	}
}

func TestFanOutResetForSeekPropagated(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	fan.ResetForSeek()

	if p1.seekCount != 1 {
		t.Fatalf("p1: expected 1 seek reset, got %d", p1.seekCount)
	}
	if p2.seekCount != 1 {
		t.Fatalf("p2: expected 1 seek reset, got %d", p2.seekCount)
	}
}

func TestFanOutPluginCount(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	if fan.PluginCount() != 2 {
		t.Fatalf("expected 2 plugins, got %d", fan.PluginCount())
	}

	p3 := newMockPlugin(DeliveryRecord)
	fan.Add(p3)
	if fan.PluginCount() != 3 {
		t.Fatalf("expected 3 plugins after add, got %d", fan.PluginCount())
	}

	fan.Remove(DeliveryHLS)
	if fan.PluginCount() != 2 {
		t.Fatalf("expected 2 plugins after remove, got %d", fan.PluginCount())
	}
}

func TestFanOutStatus(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	fan.PushVideo([]byte("frame"), 1000, 1000, true)

	statuses := fan.Status()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	found := map[DeliveryMode]bool{}
	for _, s := range statuses {
		found[s.Mode] = true
		if !s.Healthy {
			t.Errorf("plugin %s not healthy", s.Mode)
		}
	}
	if !found[DeliveryMSE] || !found[DeliveryHLS] {
		t.Fatal("missing expected modes in status")
	}
}

type atomicMockPlugin struct {
	mode  DeliveryMode
	count atomic.Int64
}

func (m *atomicMockPlugin) Mode() DeliveryMode                                  { return m.mode }
func (m *atomicMockPlugin) PushVideo(data []byte, pts, dts int64, kf bool) error { m.count.Add(1); return nil }
func (m *atomicMockPlugin) PushAudio(data []byte, pts, dts int64) error          { return nil }
func (m *atomicMockPlugin) PushSubtitle(data []byte, pts int64, dur int64) error { return nil }
func (m *atomicMockPlugin) EndOfStream()                                         {}
func (m *atomicMockPlugin) ResetForSeek()                                        {}
func (m *atomicMockPlugin) Stop()                                                {}
func (m *atomicMockPlugin) Status() PluginStatus                                 { return PluginStatus{Mode: m.mode, Healthy: true} }

func TestFanOutConcurrentPushAndAdd(t *testing.T) {
	fan := NewFanOut()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fan.Add(&atomicMockPlugin{mode: DeliveryMSE})
		}()
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fan.PushVideo([]byte("frame"), 1000, 1000, true)
		}()
	}

	wg.Wait()

	if fan.PluginCount() != 10 {
		t.Fatalf("expected 10 plugins, got %d", fan.PluginCount())
	}
}

func TestFanOutEmptyIsValid(t *testing.T) {
	fan := NewFanOut()

	if err := fan.PushVideo([]byte("frame"), 1000, 1000, true); err != nil {
		t.Fatalf("empty fanout push should not error: %v", err)
	}

	fan.EndOfStream()
	fan.ResetForSeek()
	fan.Stop()
}

type panicPlugin struct {
	mode DeliveryMode
}

func (p *panicPlugin) Mode() DeliveryMode                                  { return p.mode }
func (p *panicPlugin) PushVideo(data []byte, pts, dts int64, kf bool) error { panic("video panic") }
func (p *panicPlugin) PushAudio(data []byte, pts, dts int64) error          { panic("audio panic") }
func (p *panicPlugin) PushSubtitle(data []byte, pts int64, dur int64) error { panic("subtitle panic") }
func (p *panicPlugin) EndOfStream()                                         { panic("eos panic") }
func (p *panicPlugin) ResetForSeek()                                        { panic("seek panic") }
func (p *panicPlugin) Stop()                                                { panic("stop panic") }
func (p *panicPlugin) Status() PluginStatus                                 { return PluginStatus{Mode: p.mode, Healthy: true} }

func TestFanOutPanicInPushVideoDoesNotKillOtherPlugins(t *testing.T) {
	p1 := &panicPlugin{mode: DeliveryRecord}
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	err := fan.PushVideo([]byte("frame"), 1000, 1000, true)
	if err == nil {
		t.Fatal("expected error from panicking plugin")
	}

	if p2.videoPackets != 1 {
		t.Fatalf("p2: expected 1 video packet despite p1 panic, got %d", p2.videoPackets)
	}
}

func TestFanOutPanicInPushAudioDoesNotKillOtherPlugins(t *testing.T) {
	p1 := &panicPlugin{mode: DeliveryRecord}
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	err := fan.PushAudio([]byte("audio"), 2000, 2000)
	if err == nil {
		t.Fatal("expected error from panicking plugin")
	}

	if p2.audioPackets != 1 {
		t.Fatalf("p2: expected 1 audio packet despite p1 panic, got %d", p2.audioPackets)
	}
}

func TestFanOutPanicInPushSubtitleDoesNotKillOtherPlugins(t *testing.T) {
	p1 := &panicPlugin{mode: DeliveryRecord}
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	err := fan.PushSubtitle([]byte("sub"), 3000, 500)
	if err == nil {
		t.Fatal("expected error from panicking plugin")
	}

	if p2.subPackets != 1 {
		t.Fatalf("p2: expected 1 sub packet despite p1 panic, got %d", p2.subPackets)
	}
}

func TestFanOutPanicInEndOfStreamDoesNotKillOtherPlugins(t *testing.T) {
	p1 := &panicPlugin{mode: DeliveryRecord}
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	fan.EndOfStream()

	if p2.eosCount != 1 {
		t.Fatalf("p2: expected 1 EOS despite p1 panic, got %d", p2.eosCount)
	}
}

func TestFanOutPanicInResetForSeekDoesNotKillOtherPlugins(t *testing.T) {
	p1 := &panicPlugin{mode: DeliveryRecord}
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	fan.ResetForSeek()

	if p2.seekCount != 1 {
		t.Fatalf("p2: expected 1 seek reset despite p1 panic, got %d", p2.seekCount)
	}
}

func TestFanOutPanicInStopDoesNotKillOtherPlugins(t *testing.T) {
	p1 := &panicPlugin{mode: DeliveryRecord}
	p2 := newMockPlugin(DeliveryHLS)
	fan := NewFanOut(p1, p2)

	fan.Stop()

	if p2.stopCount != 1 {
		t.Fatalf("p2: expected 1 stop despite p1 panic, got %d", p2.stopCount)
	}
}

func TestFanOutDoubleStopSafe(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	fan := NewFanOut(p1)

	fan.Stop()
	fan.Stop()
}
