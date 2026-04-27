package store

import (
	"testing"
)

func TestFactory_MemoryStreamStore(t *testing.T) {
	f := NewFactory("")

	s, err := f.StreamStore(BackendMemory)
	if err != nil {
		t.Fatalf("StreamStore(memory): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil StreamStore")
	}
}

func TestFactory_MemorySettingsStore(t *testing.T) {
	f := NewFactory("")

	s, err := f.SettingsStore(BackendMemory)
	if err != nil {
		t.Fatalf("SettingsStore(memory): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil SettingsStore")
	}
}

func TestFactory_BoltStreamStore(t *testing.T) {
	dir := t.TempDir()
	f := NewFactory(dir)

	s, err := f.StreamStore(BackendBolt)
	if err != nil {
		t.Fatalf("StreamStore(bolt): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil StreamStore")
	}
}

func TestFactory_BoltSettingsStore(t *testing.T) {
	dir := t.TempDir()
	f := NewFactory(dir)

	s, err := f.SettingsStore(BackendBolt)
	if err != nil {
		t.Fatalf("SettingsStore(bolt): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil SettingsStore")
	}
}

func TestFactory_UnknownBackendStreamStore(t *testing.T) {
	f := NewFactory("")

	_, err := f.StreamStore("postgres")
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestFactory_UnknownBackendSettingsStore(t *testing.T) {
	f := NewFactory("")

	_, err := f.SettingsStore("postgres")
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}
