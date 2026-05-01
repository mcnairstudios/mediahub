package bolt

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mcnairstudios/mediahub/pkg/frontend/hdhr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHDHRDeviceStoreCRUD(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer db.Close()
	defer os.Remove(filepath.Join(dir, "test.db"))

	store := db.HDHRDeviceStore()
	ctx := context.Background()

	devices, err := store.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, devices)

	device := &hdhr.Device{
		ID:          "dev-1",
		Name:        "HDHR Device 1",
		DeviceUUID:  "AABBCCDD",
		Port:        5004,
		GroupIDs:    []string{"news", "sports"},
		IsEnabled:   true,
		MaxChannels: 200,
	}
	require.NoError(t, store.Create(ctx, device))

	got, err := store.Get(ctx, "dev-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "HDHR Device 1", got.Name)
	assert.Equal(t, "AABBCCDD", got.DeviceUUID)
	assert.Equal(t, 5004, got.Port)
	assert.Equal(t, []string{"news", "sports"}, got.GroupIDs)
	assert.True(t, got.IsEnabled)
	assert.Equal(t, 200, got.MaxChannels)

	got.Name = "Updated Device"
	got.Port = 5005
	require.NoError(t, store.Update(ctx, got))

	got2, err := store.Get(ctx, "dev-1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Device", got2.Name)
	assert.Equal(t, 5005, got2.Port)

	device2 := &hdhr.Device{
		ID:          "dev-2",
		Name:        "HDHR Device 2",
		DeviceUUID:  "11223344",
		Port:        5006,
		IsEnabled:   true,
		MaxChannels: 200,
	}
	require.NoError(t, store.Create(ctx, device2))

	all, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	require.NoError(t, store.Delete(ctx, "dev-1"))

	deleted, err := store.Get(ctx, "dev-1")
	require.NoError(t, err)
	assert.Nil(t, deleted)

	remaining, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, remaining, 1)
	assert.Equal(t, "dev-2", remaining[0].ID)
}
