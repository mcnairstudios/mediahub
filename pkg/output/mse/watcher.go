package mse

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type segmentIndex struct {
	mu    sync.RWMutex
	files []string
	count int
}

func (si *segmentIndex) Add(path string) int {
	si.mu.Lock()
	si.files = append(si.files, path)
	sort.Strings(si.files)
	si.count = len(si.files)
	n := si.count
	si.mu.Unlock()
	return n
}

func (si *segmentIndex) Get(seq int) (string, bool) {
	si.mu.RLock()
	defer si.mu.RUnlock()
	if seq < 1 || seq > len(si.files) {
		return "", false
	}
	return si.files[seq-1], true
}

func (si *segmentIndex) Count() int {
	si.mu.RLock()
	n := si.count
	si.mu.RUnlock()
	return n
}

func (si *segmentIndex) Reset() {
	si.mu.Lock()
	si.files = nil
	si.count = 0
	si.mu.Unlock()
}

type watcher struct {
	segDir string

	videoInit atomic.Pointer[[]byte]
	audioInit atomic.Pointer[[]byte]

	videoSegs segmentIndex
	audioSegs segmentIndex

	seen sync.Map
	done chan struct{}
}

func newWatcher(segDir string) *watcher {
	w := &watcher{
		segDir: segDir,
		done:   make(chan struct{}),
	}
	w.scan()
	go w.poll()
	return w
}

func (w *watcher) poll() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			w.scan()
		}
	}
}

func (w *watcher) scan() {
	entries, err := os.ReadDir(w.segDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if _, loaded := w.seen.LoadOrStore(name, struct{}{}); loaded {
			continue
		}
		w.handleFile(name)
	}
}

func (w *watcher) handleFile(name string) {
	path := filepath.Join(w.segDir, name)
	switch {
	case name == "init_video.mp4":
		w.loadInit(path, &w.videoInit)
	case name == "init_audio.mp4":
		w.loadInit(path, &w.audioInit)
	case strings.HasPrefix(name, "video_") && strings.HasSuffix(name, ".m4s"):
		w.videoSegs.Add(path)
	case strings.HasPrefix(name, "audio_") && strings.HasSuffix(name, ".m4s"):
		w.audioSegs.Add(path)
	}
}

func (w *watcher) loadInit(path string, dest *atomic.Pointer[[]byte]) {
	data, err := readFileRetry(path, 3, 20*time.Millisecond)
	if err != nil || len(data) == 0 {
		return
	}
	dest.Store(&data)
}

func readFileRetry(path string, retries int, delay time.Duration) ([]byte, error) {
	var data []byte
	var err error
	for i := 0; i < retries; i++ {
		data, err = os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return data, nil
		}
		time.Sleep(delay)
	}
	return data, err
}

func (w *watcher) VideoInit() []byte {
	p := w.videoInit.Load()
	if p == nil {
		return nil
	}
	return *p
}

func (w *watcher) AudioInit() []byte {
	p := w.audioInit.Load()
	if p == nil {
		return nil
	}
	return *p
}

func (w *watcher) VideoSegment(seq int) ([]byte, bool) {
	return w.readSegment(&w.videoSegs, seq)
}

func (w *watcher) AudioSegment(seq int) ([]byte, bool) {
	return w.readSegment(&w.audioSegs, seq)
}

func (w *watcher) readSegment(idx *segmentIndex, seq int) ([]byte, bool) {
	path, ok := idx.Get(seq)
	if !ok {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (w *watcher) Reset() {
	w.videoInit.Store(nil)
	w.audioInit.Store(nil)
	w.videoSegs.Reset()
	w.audioSegs.Reset()
	w.seen = sync.Map{}
}

func (w *watcher) Close() {
	select {
	case <-w.done:
	default:
		close(w.done)
	}
}
