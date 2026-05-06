package wasm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

func TestLoadPluginInvalidWASM(t *testing.T) {
	host := NewHost(nil, nil)
	defer host.Close(context.Background())

	_, err := host.LoadPlugin(context.Background(), []byte("not a wasm module"))
	if err == nil {
		t.Fatal("expected error for invalid WASM bytes")
	}
}

func TestLoadPluginEmptyBytes(t *testing.T) {
	host := NewHost(nil, nil)
	defer host.Close(context.Background())

	_, err := host.LoadPlugin(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil WASM bytes")
	}
}

func TestLoadDirNonExistent(t *testing.T) {
	host := NewHost(nil, nil)
	defer host.Close(context.Background())

	plugins, err := host.LoadDir(context.Background(), "/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("LoadDir should not error on non-existent path (glob returns empty): %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestLoadDirEmptyDir(t *testing.T) {
	dir := t.TempDir()
	host := NewHost(nil, nil)
	defer host.Close(context.Background())

	plugins, err := host.LoadDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestLoadDirSkipsInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	// Write an invalid .wasm file.
	os.WriteFile(filepath.Join(dir, "bad.wasm"), []byte("not wasm"), 0644)

	host := NewHost(nil, nil)
	defer host.Close(context.Background())

	plugins, err := host.LoadDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins (invalid file skipped), got %d", len(plugins))
	}
}

func TestHostPluginsEmpty(t *testing.T) {
	host := NewHost(nil, nil)
	defer host.Close(context.Background())

	if p := host.Plugin("nonexistent"); p != nil {
		t.Fatal("expected nil for nonexistent plugin")
	}
	if len(host.Plugins()) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(host.Plugins()))
	}
}

func TestHostCloseIdempotent(t *testing.T) {
	host := NewHost(nil, nil)
	if err := host.Close(context.Background()); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := host.Close(context.Background()); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// --- KVStore tests ---

func newTestBoltDB(t *testing.T) *bbolt.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("opening bolt: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestKVStoreGetSetDelete(t *testing.T) {
	db := newTestBoltDB(t)
	store, err := NewBoltKVStore(db)
	if err != nil {
		t.Fatalf("NewBoltKVStore: %v", err)
	}

	// Get on non-existent key returns empty.
	val, err := store.Get("myplugin", "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty, got %q", val)
	}

	// Set and get.
	if err := store.Set("myplugin", "key1", "value1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err = store.Get("myplugin", "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "value1" {
		t.Fatalf("expected value1, got %q", val)
	}

	// Different plugin type is isolated.
	val, err = store.Get("other", "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty for different plugin, got %q", val)
	}

	// Delete.
	if err := store.Delete("myplugin", "key1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	val, err = store.Get("myplugin", "key1")
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty after delete, got %q", val)
	}
}

func TestKVStoreDeleteNonExistent(t *testing.T) {
	db := newTestBoltDB(t)
	store, err := NewBoltKVStore(db)
	if err != nil {
		t.Fatalf("NewBoltKVStore: %v", err)
	}

	// Deleting non-existent key should not error.
	if err := store.Delete("myplugin", "nonexistent"); err != nil {
		t.Fatalf("Delete non-existent: %v", err)
	}
}

func TestKVStoreOverwrite(t *testing.T) {
	db := newTestBoltDB(t)
	store, err := NewBoltKVStore(db)
	if err != nil {
		t.Fatalf("NewBoltKVStore: %v", err)
	}

	store.Set("p", "k", "v1")
	store.Set("p", "k", "v2")

	val, _ := store.Get("p", "k")
	if val != "v2" {
		t.Fatalf("expected v2, got %q", val)
	}
}
