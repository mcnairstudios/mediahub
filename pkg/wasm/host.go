package wasm

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WASMHost manages the lifecycle of WASM plugins.
type WASMHost struct {
	mu         sync.RWMutex
	plugins    map[string]*WASMPlugin
	httpClient *http.Client
	kvStore    KVStore
}

// NewHost creates a new WASM plugin host.
func NewHost(httpClient *http.Client, kvStore KVStore) *WASMHost {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &WASMHost{
		plugins:    make(map[string]*WASMPlugin),
		httpClient: httpClient,
		kvStore:    kvStore,
	}
}

// LoadPlugin compiles and instantiates a WASM plugin from raw bytes.
// It calls the plugin's "describe" export to obtain its descriptor,
// then stores the plugin for later use.
func (h *WASMHost) LoadPlugin(ctx context.Context, wasmBytes []byte) (*WASMPlugin, error) {
	// Each plugin gets its own runtime for isolation.
	rt := wazero.NewRuntime(ctx)

	// WASI is required for TinyGo-compiled plugins.
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("instantiating WASI: %w", err)
	}

	env := &hostEnv{
		httpClient: h.httpClient,
		kvStore:    h.kvStore,
	}

	// Register host functions.
	hostCtx := withHostEnv(ctx, env)
	hostMod, err := registerHostFunctions(hostCtx, rt)
	if err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("registering host functions: %w", err)
	}

	// Compile the module.
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		closeIgnoreErr(hostMod, ctx)
		rt.Close(ctx)
		return nil, fmt.Errorf("compiling wasm module: %w", err)
	}

	// Instantiate the module without running _start (TinyGo's main() would exit).
	// Rust WASI modules need _initialize called separately.
	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().
		WithStartFunctions().
		WithStdout(os.Stdout).
		WithStderr(os.Stderr))
	if err == nil {
		if initFn := mod.ExportedFunction("_initialize"); initFn != nil {
			if _, initErr := initFn.Call(ctx); initErr != nil {
				log.Printf("wasm: _initialize failed: %v", initErr)
			}
		}
	}
	if err != nil {
		closeIgnoreErr(hostMod, ctx)
		rt.Close(ctx)
		return nil, fmt.Errorf("instantiating wasm module: %w", err)
	}

	plugin := &WASMPlugin{
		compiled:   compiled,
		runtime:    rt,
		hostModule: hostMod,
		module:     mod,
		env:        env,
	}

	// Call describe to get the plugin descriptor.
	descCtx := withHostEnv(ctx, env)
	descData, err := plugin.callExportUnlocked(descCtx, "describe", nil)
	if err != nil {
		plugin.Close(ctx)
		return nil, fmt.Errorf("calling describe: %w", err)
	}
	if descData == nil {
		plugin.Close(ctx)
		return nil, fmt.Errorf("describe returned no data")
	}

	desc, err := parseDescriptor(descData)
	if err != nil {
		plugin.Close(ctx)
		return nil, err
	}

	plugin.pluginType = string(desc.Type)
	plugin.Descriptor = desc
	env.pluginType = plugin.pluginType

	h.mu.Lock()
	h.plugins[plugin.pluginType] = plugin
	h.mu.Unlock()

	return plugin, nil
}

// LoadDir loads all .wasm files from the given directory.
// It logs success/failure per file and returns successfully loaded plugins.
func (h *WASMHost) LoadDir(ctx context.Context, dir string) ([]*WASMPlugin, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.wasm"))
	if err != nil {
		return nil, fmt.Errorf("globbing %s: %w", dir, err)
	}

	var loaded []*WASMPlugin
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("wasm: failed to read %s: %v", path, err)
			continue
		}

		plugin, err := h.LoadPlugin(ctx, data)
		if err != nil {
			log.Printf("wasm: failed to load %s: %v", filepath.Base(path), err)
			continue
		}

		log.Printf("wasm: loaded plugin %q from %s (type=%s, version=%s)",
			plugin.Descriptor.Label, filepath.Base(path),
			plugin.pluginType, plugin.Descriptor.Version)
		loaded = append(loaded, plugin)
	}

	return loaded, nil
}

// Plugin returns the plugin with the given type, or nil if not found.
func (h *WASMHost) Plugin(pluginType string) *WASMPlugin {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.plugins[pluginType]
}

// Plugins returns all loaded plugins.
func (h *WASMHost) Plugins() []*WASMPlugin {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*WASMPlugin, 0, len(h.plugins))
	for _, p := range h.plugins {
		result = append(result, p)
	}
	return result
}

// Close shuts down all plugins and releases resources.
func (h *WASMHost) Close(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var firstErr error
	for name, p := range h.plugins {
		if err := p.Close(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("closing plugin %s: %w", name, err)
		}
	}
	h.plugins = make(map[string]*WASMPlugin)
	return firstErr
}

// callExportUnlocked is like callExport but without locking the mutex.
// Used during LoadPlugin when we haven't exposed the plugin yet.
func (p *WASMPlugin) callExportUnlocked(ctx context.Context, name string, input []byte) (result []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("wasm trap in %s: %v", name, r)
		}
	}()

	fn := p.module.ExportedFunction(name)
	if fn == nil {
		return nil, fmt.Errorf("plugin does not export function %q", name)
	}

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
		return nil, fmt.Errorf("calling %s: %w", name, callErr)
	}

	if len(results) == 0 {
		return nil, nil
	}

	return readFromWASM(p.module, results[0])
}

func closeIgnoreErr(mod api.Module, ctx context.Context) {
	if mod != nil {
		mod.Close(ctx)
	}
}
