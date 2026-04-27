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

func TestFactory_MemoryChannelStore(t *testing.T) {
	f := NewFactory("")
	s, err := f.ChannelStore(BackendMemory)
	if err != nil {
		t.Fatalf("ChannelStore(memory): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil ChannelStore")
	}
}

func TestFactory_BoltChannelStore(t *testing.T) {
	f := NewFactory(t.TempDir())
	s, err := f.ChannelStore(BackendBolt)
	if err != nil {
		t.Fatalf("ChannelStore(bolt): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil ChannelStore")
	}
}

func TestFactory_MemoryGroupStore(t *testing.T) {
	f := NewFactory("")
	s, err := f.GroupStore(BackendMemory)
	if err != nil {
		t.Fatalf("GroupStore(memory): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil GroupStore")
	}
}

func TestFactory_BoltGroupStore(t *testing.T) {
	f := NewFactory(t.TempDir())
	s, err := f.GroupStore(BackendBolt)
	if err != nil {
		t.Fatalf("GroupStore(bolt): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil GroupStore")
	}
}

func TestFactory_MemoryEPGSourceStore(t *testing.T) {
	f := NewFactory("")
	s, err := f.EPGSourceStore(BackendMemory)
	if err != nil {
		t.Fatalf("EPGSourceStore(memory): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil EPGSourceStore")
	}
}

func TestFactory_BoltEPGSourceStore(t *testing.T) {
	f := NewFactory(t.TempDir())
	s, err := f.EPGSourceStore(BackendBolt)
	if err != nil {
		t.Fatalf("EPGSourceStore(bolt): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil EPGSourceStore")
	}
}

func TestFactory_MemoryProgramStore(t *testing.T) {
	f := NewFactory("")
	s, err := f.ProgramStore(BackendMemory)
	if err != nil {
		t.Fatalf("ProgramStore(memory): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil ProgramStore")
	}
}

func TestFactory_BoltProgramStore(t *testing.T) {
	f := NewFactory(t.TempDir())
	s, err := f.ProgramStore(BackendBolt)
	if err != nil {
		t.Fatalf("ProgramStore(bolt): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil ProgramStore")
	}
}

func TestFactory_MemoryRecordingStore(t *testing.T) {
	f := NewFactory("")
	s, err := f.RecordingStore(BackendMemory)
	if err != nil {
		t.Fatalf("RecordingStore(memory): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil RecordingStore")
	}
}

func TestFactory_BoltRecordingStore(t *testing.T) {
	f := NewFactory(t.TempDir())
	s, err := f.RecordingStore(BackendBolt)
	if err != nil {
		t.Fatalf("RecordingStore(bolt): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil RecordingStore")
	}
}

func TestFactory_MemoryUserStore(t *testing.T) {
	f := NewFactory("")
	s, err := f.UserStore(BackendMemory)
	if err != nil {
		t.Fatalf("UserStore(memory): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil UserStore")
	}
}

func TestFactory_BoltUserStore(t *testing.T) {
	f := NewFactory(t.TempDir())
	s, err := f.UserStore(BackendBolt)
	if err != nil {
		t.Fatalf("UserStore(bolt): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil UserStore")
	}
}
