// Package output defines the plugin architecture for media delivery.
// Each delivery mode (MSE, HLS, stream, record) is an OutputPlugin that
// receives demuxed packets from the pipeline. A FanOut distributes packets
// to multiple plugins simultaneously.
package output

import (
	"context"
	"net/http"
)

// DeliveryMode identifies how media is delivered to a consumer.
type DeliveryMode string

const (
	DeliveryMSE    DeliveryMode = "mse"
	DeliveryHLS    DeliveryMode = "hls"
	DeliveryStream DeliveryMode = "stream"
	DeliveryRecord DeliveryMode = "record"
	DeliveryDASH   DeliveryMode = "dash"
	DeliveryWebRTC DeliveryMode = "webrtc"
)

// OutputPlugin is the core interface for all delivery plugins. Each plugin
// receives raw encoded packets and handles muxing, segmenting, and delivery
// for its specific mode.
type OutputPlugin interface {
	Mode() DeliveryMode
	PushVideo(data []byte, pts, dts, duration int64, keyframe bool) error
	PushAudio(data []byte, pts, dts, duration int64) error
	PushSubtitle(data []byte, pts int64, duration int64) error
	EndOfStream()
	ResetForSeek()
	Stop()
	Status() PluginStatus
}

// PluginStatus reports the current state of a plugin.
type PluginStatus struct {
	Mode         DeliveryMode `json:"mode"`
	SegmentCount int          `json:"segment_count"`
	BytesWritten int64        `json:"bytes_written"`
	Healthy      bool         `json:"healthy"`
	Error        string       `json:"error,omitempty"`
}

// ServablePlugin extends OutputPlugin with HTTP serving capabilities.
// Plugins that serve content to clients (MSE, HLS) implement this.
type ServablePlugin interface {
	OutputPlugin
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	Generation() int64
	WaitReady(ctx context.Context) error
}

// CodecConstrainer is an optional interface that output plugins can implement
// to declare hard codec constraints for their delivery mode. The orchestrator
// uses codec.DeliveryConstraints() for static lookup since plugins are created
// after codec resolution, but this interface can be used for validation or
// future dynamic constraint discovery.
//
// CodecConstraints returns a map with optional keys:
//   - "required_audio_codec": string (e.g. "opus")
//   - "force_transcode": bool
//   - "disable_decode_hw": bool
//   - "allowed_video_codecs": map[string]bool
type CodecConstrainer interface {
	CodecConstraints() map[string]any
}
