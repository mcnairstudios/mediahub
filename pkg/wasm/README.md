# pkg/wasm

WASM plugin runtime host for MediaHub. Uses [wazero](https://github.com/tetratelabs/wazero) (pure Go, no CGO) to load and execute `.wasm` plugin files.

## Architecture

Each plugin runs in its own isolated wazero runtime. Communication between host and plugin uses JSON over shared WASM memory.

### Memory Convention

- Plugin exports `alloc(size uint32) -> uint32` for memory allocation
- Functions return a packed `uint64`: high 32 bits = pointer, low 32 bits = length
- Host reads/writes data at the returned pointer

### Plugin Exports

| Export | Signature | Description |
|--------|-----------|-------------|
| `alloc` | `(size i32) -> i32` | Allocate memory in plugin |
| `describe` | `() -> i64` | Return JSON plugin descriptor |
| `refresh` | `(config_ptr i32, config_len i32) -> i64` | Refresh streams, return JSON |
| `interact` | `(action_ptr i32, action_len i32) -> i64` | Handle user interaction |

### Host Functions (env module)

| Function | Signature | Description |
|----------|-----------|-------------|
| `host_log` | `(level i32, msg_ptr i32, msg_len i32)` | Log a message (0=debug, 1=info, 2=warn, 3=error) |
| `host_http_request` | `(url_ptr, url_len, method_ptr, method_len, headers_ptr, headers_len, body_ptr, body_len) -> (status i32, resp i64)` | Make HTTP request |
| `host_kv_get` | `(key_ptr, key_len) -> i64` | Get value from plugin-scoped KV store |
| `host_kv_set` | `(key_ptr, key_len, val_ptr, val_len)` | Set value in plugin-scoped KV store |

## Files

- `host.go` - WASMHost: plugin lifecycle management (load, close, registry)
- `plugin.go` - WASMPlugin: wraps a single plugin instance with mutex-guarded calls
- `hostfuncs.go` - Host function implementations registered as the "env" module
- `memory.go` - WASM memory read/write helpers
- `kvstore.go` - Plugin-scoped key-value store backed by bbolt

## Usage

```go
kvStore, _ := wasm.NewBoltKVStore(boltDB)
host := wasm.NewHost(httpClient, kvStore)
plugins, _ := host.LoadDir(ctx, "/path/to/plugins")
defer host.Close(ctx)
```

## Concurrency

Each plugin is protected by a simple mutex. Only one call (describe/refresh/interact) can execute at a time per plugin. This is Phase 1; a pool-based approach may be added later for concurrent callers.
