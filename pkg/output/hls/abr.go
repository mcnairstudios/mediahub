package hls

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
)

// ABRPlugin serves a master playlist that references multiple variant
// (rendition) playlists, each at a different resolution and bitrate.
// Internally it fans packets out to N single-rendition HLS plugins.
//
// Audio is muxed into every variant (no separate audio rendition for now),
// which is the simplest approach and broadly compatible.
type ABRPlugin struct {
	variants   []*abrVariant
	outputDir  string
	generation atomic.Int64
	stopped    bool
	mu         sync.Mutex
}

type abrVariant struct {
	rendition Rendition
	plugin    *Plugin
	// subPath is the URL path prefix for this variant, e.g. "v0", "v1".
	subPath string
	width   int
	height  int
}

// NewABR creates an ABR HLS plugin. Each rendition gets its own segment
// directory and HLS muxer. The supplied PluginConfig is used as a template;
// Video dimensions and Options are adjusted per-rendition.
//
// The caller must populate cfg.Options["abr_renditions"] with a []Rendition
// slice. If empty, DefaultRenditions() is used.
func NewABR(cfg output.PluginConfig) (*ABRPlugin, error) {
	if cfg.OutputDir == "" {
		return nil, fmt.Errorf("hls/abr: OutputDir is required")
	}
	if cfg.Video == nil {
		return nil, fmt.Errorf("hls/abr: Video info is required for ABR")
	}

	renditions := extractRenditions(cfg)
	if len(renditions) == 0 {
		renditions = DefaultRenditions()
	}

	// Filter renditions that exceed source height and compute widths.
	srcW, srcH := cfg.Video.Width, cfg.Video.Height
	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("hls/abr: source dimensions are required (got %dx%d)", srcW, srcH)
	}

	var filtered []Rendition
	seen := map[int]bool{}
	for _, r := range renditions {
		h := r.Height
		if h > srcH {
			h = srcH
		}
		if seen[h] {
			continue
		}
		seen[h] = true
		filtered = append(filtered, Rendition{Height: h, Bitrate: r.Bitrate})
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("hls/abr: no usable renditions for %dx%d source", srcW, srcH)
	}

	abr := &ABRPlugin{
		outputDir: cfg.OutputDir,
	}
	abr.generation.Store(1)

	for i, r := range filtered {
		outH := r.Height
		outW := srcW * outH / srcH
		outW = outW &^ 1 // ensure even

		subPath := fmt.Sprintf("v%d", i)
		varDir := filepath.Join(cfg.OutputDir, subPath)

		// Clone the config for this variant.
		varCfg := cfg
		varCfg.OutputDir = varDir

		// Clone video info with adjusted dimensions.
		vi := *cfg.Video
		vi.Width = outW
		vi.Height = outH
		varCfg.Video = &vi

		plugin, err := New(varCfg)
		if err != nil {
			// Clean up already-created variants.
			for _, v := range abr.variants {
				v.plugin.Stop()
			}
			return nil, fmt.Errorf("hls/abr: create variant %d (%dp): %w", i, outH, err)
		}

		abr.variants = append(abr.variants, &abrVariant{
			rendition: r,
			plugin:    plugin,
			subPath:   subPath,
			width:     outW,
			height:    outH,
		})
	}

	return abr, nil
}

// extractRenditions pulls []Rendition from cfg.Options["abr_renditions"].
func extractRenditions(cfg output.PluginConfig) []Rendition {
	if cfg.Options == nil {
		return nil
	}
	v, ok := cfg.Options["abr_renditions"]
	if !ok {
		return nil
	}
	r, ok := v.([]Rendition)
	if !ok {
		return nil
	}
	return r
}

func (a *ABRPlugin) Mode() output.DeliveryMode {
	return output.DeliveryHLS
}

// PushVideo fans video data to all variant plugins.
func (a *ABRPlugin) PushVideo(data []byte, pts, dts, duration int64, keyframe bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopped {
		return nil
	}

	var firstErr error
	for _, v := range a.variants {
		if err := v.plugin.PushVideo(data, pts, dts, duration, keyframe); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// PushAudio fans audio data to all variant plugins.
func (a *ABRPlugin) PushAudio(data []byte, pts, dts, duration int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopped {
		return nil
	}

	var firstErr error
	for _, v := range a.variants {
		if err := v.plugin.PushAudio(data, pts, dts, duration); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (a *ABRPlugin) PushSubtitle(data []byte, pts int64, duration int64) error {
	return nil
}

func (a *ABRPlugin) EndOfStream() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopped {
		return
	}
	for _, v := range a.variants {
		v.plugin.EndOfStream()
	}
	a.stopped = true
}

func (a *ABRPlugin) ResetForSeek() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopped {
		return
	}
	a.generation.Add(1)
	for _, v := range a.variants {
		v.plugin.ResetForSeek()
	}
}

func (a *ABRPlugin) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopped {
		return
	}
	a.stopped = true
	for _, v := range a.variants {
		v.plugin.Stop()
	}
}

func (a *ABRPlugin) Status() output.PluginStatus {
	a.mu.Lock()
	defer a.mu.Unlock()

	totalSegs := 0
	healthy := true
	var errStr string
	for _, v := range a.variants {
		st := v.plugin.Status()
		totalSegs += st.SegmentCount
		if !st.Healthy {
			healthy = false
		}
		if st.Error != "" && errStr == "" {
			errStr = st.Error
		}
	}
	return output.PluginStatus{
		Mode:         output.DeliveryHLS,
		SegmentCount: totalSegs,
		Healthy:      healthy && !a.stopped,
		Error:        errStr,
	}
}

func (a *ABRPlugin) Generation() int64 {
	return a.generation.Load()
}

func (a *ABRPlugin) WaitReady(ctx context.Context) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	// Wait for at least the first (highest quality) variant to be ready.
	if len(a.variants) > 0 {
		return a.variants[0].plugin.WaitReady(ctx)
	}
	return nil
}

// ServeHTTP routes requests to the master playlist or the appropriate variant.
//
// URL layout:
//
//	/master.m3u8           -> master playlist
//	/playlist.m3u8         -> master playlist (alias for compatibility)
//	/v0/playlist.m3u8      -> variant 0 media playlist
//	/v0/seg0.ts            -> variant 0 segment
//	/v1/playlist.m3u8      -> variant 1 media playlist
//	...
func (a *ABRPlugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := r.URL.Path

	// Master playlist.
	if path == "/master.m3u8" || path == "master.m3u8" ||
		path == "/playlist.m3u8" || path == "playlist.m3u8" {
		a.serveMasterPlaylist(w, r)
		return
	}

	// Route to variant. Path is e.g. "/v0/playlist.m3u8" or "/v0/seg0.ts".
	for _, v := range a.variants {
		prefix := "/" + v.subPath + "/"
		if strings.HasPrefix(path, prefix) {
			// Strip the variant prefix so the sub-plugin sees e.g. "/playlist.m3u8".
			subPath := "/" + strings.TrimPrefix(path, prefix)
			subReq := r.Clone(r.Context())
			subReq.URL.Path = subPath
			subReq.RequestURI = subPath
			v.plugin.ServeHTTP(w, subReq)
			return
		}
	}

	http.NotFound(w, r)
}

func (a *ABRPlugin) serveMasterPlaylist(w http.ResponseWriter, r *http.Request) {
	// Wait for the first variant to be ready before serving.
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := a.WaitReady(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")

	for _, v := range a.variants {
		sb.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n",
			v.rendition.Bitrate,
			v.width, v.height,
		))
		sb.WriteString(v.subPath + "/playlist.m3u8\n")
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Write([]byte(sb.String())) //nolint:errcheck
}

// Variants returns the configured variants for inspection and testing.
func (a *ABRPlugin) Variants() []Rendition {
	out := make([]Rendition, len(a.variants))
	for i, v := range a.variants {
		out[i] = Rendition{Height: v.height, Bitrate: v.rendition.Bitrate}
	}
	return out
}

// VariantOutputDirs returns the segment output directories for each variant.
func (a *ABRPlugin) VariantOutputDirs() []string {
	dirs := make([]string, len(a.variants))
	for i, v := range a.variants {
		dirs[i] = v.plugin.segDir
	}
	return dirs
}

// VariantCount returns the number of active variants.
func (a *ABRPlugin) VariantCount() int {
	return len(a.variants)
}

// MasterPlaylistPath returns the path where the master playlist is served.
func (a *ABRPlugin) MasterPlaylistPath() string {
	return "/master.m3u8"
}

// writeABRMasterPlaylist writes a master.m3u8 file to disk (for debugging/inspection).
func (a *ABRPlugin) writeABRMasterPlaylist() error {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	for _, v := range a.variants {
		sb.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n",
			v.rendition.Bitrate,
			v.width, v.height,
		))
		sb.WriteString(v.subPath + "/playlist.m3u8\n")
	}
	return os.WriteFile(filepath.Join(a.outputDir, "master.m3u8"), []byte(sb.String()), 0644)
}

// Compile-time interface checks.
var _ output.OutputPlugin = (*ABRPlugin)(nil)
var _ output.ServablePlugin = (*ABRPlugin)(nil)
