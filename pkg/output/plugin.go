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
)

// OutputPlugin is the core interface for all delivery plugins. Each plugin
// receives raw encoded packets and handles muxing, segmenting, and delivery
// for its specific mode.
type OutputPlugin interface {
	Mode() DeliveryMode
	PushVideo(data []byte, pts, dts int64, keyframe bool) error
	PushAudio(data []byte, pts, dts int64) error
	PushSubtitle(data []byte, pts int64, duration int64) error
	EndOfStream()
	ResetForSeek()
	Stop()
	Status() PluginStatus
}

// PluginStatus reports the current state of a plugin.
type PluginStatus struct {
	Mode         DeliveryMode
	SegmentCount int
	BytesWritten int64
	Healthy      bool
	Error        string
}

// ServablePlugin extends OutputPlugin with HTTP serving capabilities.
// Plugins that serve content to clients (MSE, HLS) implement this.
type ServablePlugin interface {
	OutputPlugin
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	Generation() int64
	WaitReady(ctx context.Context) error
}
