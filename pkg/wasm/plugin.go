package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mcnairstudios/mediahub/pkg/source"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WASMPlugin represents a loaded WASM plugin instance.
type WASMPlugin struct {
	pluginType string
	Descriptor source.PluginDescriptor
	compiled   wazero.CompiledModule
	runtime    wazero.Runtime
	hostModule api.Module
	mu         sync.Mutex
	module     api.Module
	env        *hostEnv
}

// Type returns the plugin's source type identifier.
func (p *WASMPlugin) Type() string {
	return p.pluginType
}

// CallDescribe calls the plugin's "describe" export and returns raw JSON.
func (p *WASMPlugin) CallDescribe(ctx context.Context) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callExport(ctx, "describe", nil)
}

// CallRefresh calls the plugin's "refresh" export with the given config JSON.
func (p *WASMPlugin) CallRefresh(ctx context.Context, configJSON []byte) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callExport(ctx, "refresh", configJSON)
}

// CallInteract calls the plugin's "interact" export with the given action JSON.
func (p *WASMPlugin) CallInteract(ctx context.Context, actionJSON []byte) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callExport(ctx, "interact", actionJSON)
}

// Close releases all resources associated with this plugin.
func (p *WASMPlugin) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	if p.module != nil {
		if err := p.module.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		p.module = nil
	}
	if p.hostModule != nil {
		if err := p.hostModule.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		p.hostModule = nil
	}
	if p.runtime != nil {
		if err := p.runtime.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		p.runtime = nil
	}
	return firstErr
}

// callExport calls a named WASM export function with optional input data.
// The caller must hold p.mu.
func (p *WASMPlugin) callExport(ctx context.Context, name string, input []byte) (result []byte, err error) {
	// Wrap in recover to prevent WASM traps from crashing the host.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("wasm trap in %s.%s: %v", p.pluginType, name, r)
		}
	}()

	fn := p.module.ExportedFunction(name)
	if fn == nil {
		return nil, fmt.Errorf("plugin %s does not export function %q", p.pluginType, name)
	}

	ctx = withHostEnv(ctx, p.env)

	var args []uint64
	if len(input) > 0 {
		ptr, length, writeErr := writeToWASM(ctx, p.module, input)
		if writeErr != nil {
			return nil, fmt.Errorf("writing input for %s: %w", name, writeErr)
		}
		args = []uint64{uint64(ptr), uint64(length)}
	}

	results, callErr := fn.Call(ctx, args...)
	if callErr != nil {
		return nil, fmt.Errorf("calling %s.%s: %w", p.pluginType, name, callErr)
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Result is a packed uint64: high 32 = ptr, low 32 = len.
	return readFromWASM(p.module, results[0])
}

// parseDescriptor parses a PluginDescriptor from the describe export result.
func parseDescriptor(data []byte) (source.PluginDescriptor, error) {
	var desc source.PluginDescriptor
	if err := json.Unmarshal(data, &desc); err != nil {
		return desc, fmt.Errorf("parsing plugin descriptor: %w", err)
	}
	if desc.Type == "" {
		return desc, fmt.Errorf("plugin descriptor missing type")
	}
	if desc.Label == "" {
		return desc, fmt.Errorf("plugin descriptor missing label")
	}
	return desc, nil
}
