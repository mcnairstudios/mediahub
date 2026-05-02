package main

import (
	"github.com/mcnairstudios/mediahub/pkg/output"
	"github.com/mcnairstudios/mediahub/pkg/output/hls"
	"github.com/mcnairstudios/mediahub/pkg/output/mse"
	"github.com/mcnairstudios/mediahub/pkg/output/record"
	"github.com/mcnairstudios/mediahub/pkg/output/stream"
)

func registerOutputs(reg *output.Registry) {
	reg.Register(output.DeliveryMSE, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return mse.New(cfg)
	})
	reg.Register(output.DeliveryHLS, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return hls.New(cfg)
	})
	reg.Register(output.DeliveryStream, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return stream.New(cfg)
	})
	reg.Register(output.DeliveryRecord, func(cfg output.PluginConfig) (output.OutputPlugin, error) {
		return record.New(cfg)
	})
}
