package wasm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// hostEnv holds the dependencies injected into host functions.
type hostEnv struct {
	httpClient *http.Client
	kvStore    KVStore
	pluginType string
}

// contextKey is the key used to store hostEnv in context.
type contextKey struct{}

// withHostEnv stores host environment in context for host function access.
func withHostEnv(ctx context.Context, env *hostEnv) context.Context {
	return context.WithValue(ctx, contextKey{}, env)
}

// getHostEnv retrieves host environment from context.
func getHostEnv(ctx context.Context) *hostEnv {
	if v := ctx.Value(contextKey{}); v != nil {
		return v.(*hostEnv)
	}
	return nil
}

// registerHostFunctions registers the "env" module with host function
// implementations for WASM plugins to call.
func registerHostFunctions(ctx context.Context, rt wazero.Runtime) (api.Module, error) {
	builder := rt.NewHostModuleBuilder("env")

	builder.NewFunctionBuilder().
		WithFunc(hostLog).
		WithParameterNames("level", "msg_ptr", "msg_len").
		Export("host_log")

	builder.NewFunctionBuilder().
		WithFunc(hostHTTPRequest).
		WithParameterNames("url_ptr", "url_len", "method_ptr", "method_len",
			"headers_ptr", "headers_len", "body_ptr", "body_len").
		Export("host_http_request")

	builder.NewFunctionBuilder().
		WithFunc(hostKVGet).
		WithParameterNames("key_ptr", "key_len").
		Export("host_kv_get")

	builder.NewFunctionBuilder().
		WithFunc(hostKVSet).
		WithParameterNames("key_ptr", "key_len", "val_ptr", "val_len").
		Export("host_kv_set")

	return builder.Instantiate(ctx)
}

// hostLog implements host_log(level i32, msg_ptr i32, msg_len i32).
func hostLog(ctx context.Context, mod api.Module, level uint32, msgPtr, msgLen uint32) {
	env := getHostEnv(ctx)
	prefix := "wasm"
	if env != nil {
		prefix = fmt.Sprintf("wasm[%s]", env.pluginType)
	}

	msg, ok := mod.Memory().Read(msgPtr, msgLen)
	if !ok {
		log.Printf("%s: failed to read log message from memory", prefix)
		return
	}

	text := string(msg)
	switch level {
	case 0:
		log.Printf("%s [DEBUG] %s", prefix, text)
	case 1:
		log.Printf("%s [INFO] %s", prefix, text)
	case 2:
		log.Printf("%s [WARN] %s", prefix, text)
	case 3:
		log.Printf("%s [ERROR] %s", prefix, text)
	default:
		log.Printf("%s %s", prefix, text)
	}
}

// hostHTTPRequest implements host_http_request.
// Parameters: url_ptr, url_len, method_ptr, method_len, headers_ptr, headers_len, body_ptr, body_len
// Returns: (status_code i32, resp_ptr_len i64)
// The response body is written back to WASM memory; resp_ptr_len is packed as high32=ptr, low32=len.
func hostHTTPRequest(ctx context.Context, mod api.Module,
	urlPtr, urlLen, methodPtr, methodLen, headersPtr, headersLen, bodyPtr, bodyLen uint32,
) uint64 {
	env := getHostEnv(ctx)
	if env == nil {
		log.Printf("wasm: host_http_request called without host environment")
		return 0
	}

	urlBytes, ok := mod.Memory().Read(urlPtr, urlLen)
	if !ok {
		return 0
	}
	methodBytes, ok := mod.Memory().Read(methodPtr, methodLen)
	if !ok {
		return 0
	}

	var reqBody io.Reader
	if bodyLen > 0 {
		bodyBytes, ok := mod.Memory().Read(bodyPtr, bodyLen)
		if !ok {
			return 0
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, string(methodBytes), string(urlBytes), reqBody)
	if err != nil {
		log.Printf("wasm[%s]: http request creation failed: %v", env.pluginType, err)
		return 0
	}

	// Parse headers — supports both JSON object {"Key": "Value"} and
	// newline-separated "Key: Value" pairs.
	if headersLen > 0 {
		headerBytes, ok := mod.Memory().Read(headersPtr, headersLen)
		if ok {
			headerStr := strings.TrimSpace(string(headerBytes))
			if strings.HasPrefix(headerStr, "{") {
				var headerMap map[string]string
				if json.Unmarshal(headerBytes, &headerMap) == nil {
					for k, v := range headerMap {
						req.Header.Set(k, v)
					}
				}
			} else {
				for _, line := range strings.Split(headerStr, "\n") {
					if idx := strings.IndexByte(line, ':'); idx > 0 {
						req.Header.Set(strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]))
					}
				}
			}
		}
	}

	client := env.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("wasm[%s]: http request failed: %v", env.pluginType, err)
		return 0
	}
	defer resp.Body.Close()

	// Limit response body to 10MB to prevent runaway plugins.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		log.Printf("wasm[%s]: reading http response: %v", env.pluginType, err)
		return 0
	}

	if len(respBody) == 0 {
		log.Printf("wasm[%s]: http %d empty response for %s", env.pluginType, resp.StatusCode, string(urlBytes))
		return 0
	}

	log.Printf("wasm[%s]: http %d %d bytes for %s", env.pluginType, resp.StatusCode, len(respBody), string(urlBytes)[:80])

	ptr, length, err := writeToWASM(ctx, mod, respBody)
	if err != nil {
		log.Printf("wasm[%s]: writing response to wasm memory: %v", env.pluginType, err)
		return 0
	}

	return packPtrLen(ptr, length)
}

// hostKVGet implements host_kv_get(key_ptr, key_len) -> (val_ptr_len i64).
func hostKVGet(ctx context.Context, mod api.Module, keyPtr, keyLen uint32) uint64 {
	env := getHostEnv(ctx)
	if env == nil || env.kvStore == nil {
		return 0
	}

	keyBytes, ok := mod.Memory().Read(keyPtr, keyLen)
	if !ok {
		return 0
	}

	val, err := env.kvStore.Get(env.pluginType, string(keyBytes))
	if err != nil || val == "" {
		return 0
	}

	ptr, length, err := writeToWASM(ctx, mod, []byte(val))
	if err != nil {
		return 0
	}
	return packPtrLen(ptr, length)
}

// hostKVSet implements host_kv_set(key_ptr, key_len, val_ptr, val_len).
func hostKVSet(ctx context.Context, mod api.Module, keyPtr, keyLen, valPtr, valLen uint32) {
	env := getHostEnv(ctx)
	if env == nil || env.kvStore == nil {
		return
	}

	keyBytes, ok := mod.Memory().Read(keyPtr, keyLen)
	if !ok {
		return
	}
	valBytes, ok := mod.Memory().Read(valPtr, valLen)
	if !ok {
		return
	}

	if err := env.kvStore.Set(env.pluginType, string(keyBytes), string(valBytes)); err != nil {
		log.Printf("wasm[%s]: kv set failed: %v", env.pluginType, err)
	}
}
