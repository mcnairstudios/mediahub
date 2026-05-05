package output

import (
	"sync"
	"testing"
)

func TestSeek_FanOut_PropagatesResetToAllPlugins(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)
	p3 := newMockPlugin(DeliveryRecord)

	fo := NewFanOut()
	fo.Add(p1)
	fo.Add(p2)
	fo.Add(p3)

	fo.ResetForSeek()

	if p1.seekCount != 1 {
		t.Fatalf("expected MSE plugin seek count 1, got %d", p1.seekCount)
	}
	if p2.seekCount != 1 {
		t.Fatalf("expected HLS plugin seek count 1, got %d", p2.seekCount)
	}
	if p3.seekCount != 1 {
		t.Fatalf("expected Record plugin seek count 1, got %d", p3.seekCount)
	}
}

func TestSeek_FanOut_MultipleSeeks(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryDASH)

	fo := NewFanOut()
	fo.Add(p1)
	fo.Add(p2)

	for i := 0; i < 10; i++ {
		fo.ResetForSeek()
	}

	if p1.seekCount != 10 {
		t.Fatalf("expected MSE seek count 10, got %d", p1.seekCount)
	}
	if p2.seekCount != 10 {
		t.Fatalf("expected DASH seek count 10, got %d", p2.seekCount)
	}
}

func TestSeek_FanOut_AcceptsPacketsAfterSeek(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)

	fo := NewFanOut()
	fo.Add(p1)
	fo.Add(p2)

	if err := fo.PushVideo([]byte{0x01}, 1000, 1000, 0, true); err != nil {
		t.Fatal(err)
	}

	fo.ResetForSeek()

	if err := fo.PushVideo([]byte{0x02}, 50000, 50000, 0, true); err != nil {
		t.Fatal(err)
	}

	if p1.videoPackets != 2 {
		t.Fatalf("expected MSE 2 video packets, got %d", p1.videoPackets)
	}
	if p2.videoPackets != 2 {
		t.Fatalf("expected HLS 2 video packets, got %d", p2.videoPackets)
	}
}

func TestSeek_FanOut_AudioAcceptedAfterSeek(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)

	fo := NewFanOut()
	fo.Add(p1)

	if err := fo.PushAudio([]byte{0xFF}, 1000, 1000, 0); err != nil {
		t.Fatal(err)
	}

	fo.ResetForSeek()

	if err := fo.PushAudio([]byte{0xFF}, 50000, 50000, 0); err != nil {
		t.Fatal(err)
	}

	if p1.audioPackets != 2 {
		t.Fatalf("expected 2 audio packets, got %d", p1.audioPackets)
	}
}

func TestSeek_FanOut_ConcurrentSeekAndPush(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryHLS)

	fo := NewFanOut()
	fo.Add(p1)
	fo.Add(p2)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = fo.PushVideo([]byte{0x01}, int64(i*3600), int64(i*3600), 0, i%10 == 0)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = fo.PushAudio([]byte{0xFF}, int64(i*1024), int64(i*1024), 0)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			fo.ResetForSeek()
		}
	}()

	wg.Wait()

	if p1.seekCount != 20 {
		t.Fatalf("expected MSE seek count 20, got %d", p1.seekCount)
	}
	if p2.seekCount != 20 {
		t.Fatalf("expected HLS seek count 20, got %d", p2.seekCount)
	}
}

func TestSeek_FanOut_SeekAfterStop(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)

	fo := NewFanOut()
	fo.Add(p1)

	fo.Stop()
	fo.ResetForSeek()

	if p1.seekCount != 1 {
		t.Fatalf("expected seek count 1 after stop, got %d", p1.seekCount)
	}
}

func TestSeek_FanOut_SeekWithNilPluginSafe(t *testing.T) {
	fo := NewFanOut()
	fo.ResetForSeek()
}

func TestSeek_FanOut_BackwardsPTSAfterSeek(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)

	fo := NewFanOut()
	fo.Add(p1)

	_ = fo.PushVideo([]byte{0x01}, 90000, 90000, 0, true)

	fo.ResetForSeek()

	_ = fo.PushVideo([]byte{0x01}, 45000, 45000, 0, true)

	if p1.videoPackets != 2 {
		t.Fatalf("expected 2 video packets, got %d", p1.videoPackets)
	}
}

func TestSeek_FanOut_RapidSeekBurst(t *testing.T) {
	p1 := newMockPlugin(DeliveryMSE)
	p2 := newMockPlugin(DeliveryWebRTC)

	fo := NewFanOut()
	fo.Add(p1)
	fo.Add(p2)

	for i := 0; i < 50; i++ {
		_ = fo.PushVideo([]byte{0x01}, int64(i*90000), int64(i*90000), 0, true)
		fo.ResetForSeek()
	}

	if p1.seekCount != 50 {
		t.Fatalf("expected 50 seeks on MSE, got %d", p1.seekCount)
	}
	if p2.seekCount != 50 {
		t.Fatalf("expected 50 seeks on WebRTC, got %d", p2.seekCount)
	}
}
