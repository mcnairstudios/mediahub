package wasm

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// writeToWASM allocates memory in the WASM module and writes data into it.
// It calls the plugin's exported "alloc" function to obtain a pointer.
// Returns (ptr, len, error).
func writeToWASM(ctx context.Context, mod api.Module, data []byte) (uint32, uint32, error) {
	if len(data) == 0 {
		return 0, 0, nil
	}

	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		return 0, 0, fmt.Errorf("plugin does not export alloc function")
	}

	size := uint64(len(data))
	results, err := alloc.Call(ctx, size)
	if err != nil {
		return 0, 0, fmt.Errorf("calling alloc(%d): %w", size, err)
	}
	if len(results) == 0 {
		return 0, 0, fmt.Errorf("alloc returned no result")
	}

	ptr := uint32(results[0])
	if ptr == 0 {
		return 0, 0, fmt.Errorf("alloc returned null pointer")
	}

	if !mod.Memory().Write(ptr, data) {
		return 0, 0, fmt.Errorf("failed to write %d bytes at offset %d", len(data), ptr)
	}

	return ptr, uint32(len(data)), nil
}

// readFromWASM reads bytes from WASM memory using a packed ptr+len uint64.
// High 32 bits = pointer, low 32 bits = length.
func readFromWASM(mod api.Module, ptrLen uint64) ([]byte, error) {
	ptr := uint32(ptrLen >> 32)
	length := uint32(ptrLen & 0xFFFFFFFF)

	if length == 0 {
		return nil, nil
	}

	data, ok := mod.Memory().Read(ptr, length)
	if !ok {
		return nil, fmt.Errorf("failed to read %d bytes at offset %d", length, ptr)
	}

	// Make a copy since WASM memory may be reused.
	result := make([]byte, length)
	copy(result, data)
	return result, nil
}

// packPtrLen packs a pointer and length into a single uint64.
// High 32 bits = pointer, low 32 bits = length.
func packPtrLen(ptr, length uint32) uint64 {
	return (uint64(ptr) << 32) | uint64(length)
}
