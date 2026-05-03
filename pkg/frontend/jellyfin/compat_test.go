package jellyfin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var hex32RE = regexp.MustCompile(`^[0-9a-f]{32}$`)

func assertHex32(t *testing.T, label, v string) {
	t.Helper()
	if !hex32RE.MatchString(v) {
		t.Errorf("%s: want 32-char dashless hex, got %q", label, v)
	}
}

func TestAllIDFields32CharHex(t *testing.T) {
	t.Run("generated server ID is hex", func(t *testing.T) {
		srv := NewServer(ServerDeps{
			ServerName: "Test",
			StateDir:   t.TempDir(),
			Auth:       &mockAuthService{},
			Log:        zerolog.Nop(),
		})
		assertHex32(t, "serverID", srv.serverID)
	})

	t.Run("user views IDs are 32 char", func(t *testing.T) {
		srv := newTestServerFull()
		req := httptest.NewRequest(http.MethodGet, "/UserViews", nil)
		w := httptest.NewRecorder()
		srv.userViews(w, req)

		var result BaseItemDtoQueryResult
		require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

		for _, item := range result.Items {
			assert.Equal(t, 32, len(item.ID), "%s.ID should be 32 chars", item.Name)
			assert.Equal(t, 32, len(item.ServerID), "%s.ServerID should be 32 chars", item.Name)
		}
	})

	t.Run("live TV channel IDs are 32 char", func(t *testing.T) {
		srv := newTestServerFull()
		req := httptest.NewRequest(http.MethodGet, "/LiveTv/Channels", nil)
		w := httptest.NewRecorder()
		srv.liveTvChannels(w, req)

		var result BaseItemDtoQueryResult
		require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

		for _, item := range result.Items {
			assert.Equal(t, 32, len(item.ID), "%s.ID should be 32 chars", item.Name)
			assert.Equal(t, 32, len(item.ServerID), "%s.ServerID should be 32 chars", item.Name)
		}
	})

	t.Run("playback info IDs", func(t *testing.T) {
		srv := newTestServerFull()
		itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
		req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
		req.SetPathValue("itemId", itemID)
		w := httptest.NewRecorder()
		srv.playbackInfo(w, req)

		var result map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

		ms := result["MediaSources"].([]any)[0].(map[string]any)
		msID, _ := ms["Id"].(string)
		assert.Equal(t, 32, len(msID))
	})
}

func TestArrayFieldsInitialized(t *testing.T) {
	srv := newTestServerFull()

	t.Run("collection folder arrays are initialized not nil", func(t *testing.T) {
		item := srv.newCollectionFolderItem("Movies", viewMoviesID, "movies", map[string]string{})

		assert.NotNil(t, item.ExternalUrls)
		assert.NotNil(t, item.Taglines)
		assert.NotNil(t, item.Genres)
		assert.NotNil(t, item.RemoteTrailers)
		assert.NotNil(t, item.Tags)
		assert.NotNil(t, item.LockedFields)
		assert.NotNil(t, item.BackdropImageTags)

		assert.Empty(t, item.ExternalUrls)
		assert.Empty(t, item.Taglines)
		assert.Empty(t, item.Genres)
		assert.Empty(t, item.RemoteTrailers)
		assert.Empty(t, item.Tags)
		assert.Empty(t, item.LockedFields)
		assert.Empty(t, item.BackdropImageTags)
	})

	t.Run("collection folder object fields are initialized not nil", func(t *testing.T) {
		item := srv.newCollectionFolderItem("Movies", viewMoviesID, "movies", map[string]string{})

		assert.NotNil(t, item.ProviderIds)
		assert.NotNil(t, item.ImageTags)
		assert.NotNil(t, item.ImageBlurHashes)

		assert.Empty(t, item.ProviderIds)
		assert.Empty(t, item.ImageTags)
		assert.Empty(t, item.ImageBlurHashes)
	})

	t.Run("collection folder people and studios initialized", func(t *testing.T) {
		item := srv.newCollectionFolderItem("Movies", viewMoviesID, "movies", map[string]string{})

		assert.NotNil(t, item.People)
		assert.NotNil(t, item.Studios)
		assert.NotNil(t, item.GenreItems)
	})
}

func TestMediaSourceRequiredFields(t *testing.T) {
	ms := MediaSource{
		Protocol:                "Http",
		ID:                      "abcd1234abcd1234abcd1234abcd1234",
		Type:                    "Default",
		Name:                    "Default",
		Container:               "mp4",
		IsRemote:                true,
		SupportsTranscoding:     true,
		SupportsDirectStream:    false,
		SupportsDirectPlay:      false,
		TranscodingURL:          "/Videos/test/master.m3u8",
		TranscodingSubProtocol:  "hls",
		TranscodingContainer:    "ts",
		DefaultAudioStreamIndex: 1,
		MediaStreams: []MediaStream{
			{Type: "Video", Codec: "h264", Index: 0, IsDefault: true},
			{Type: "Audio", Codec: "aac", Index: 1, IsDefault: true},
		},
	}

	data, err := json.Marshal(ms)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	requiredFields := []string{
		"Protocol", "Id", "Type", "Name", "IsRemote",
		"SupportsTranscoding", "SupportsDirectStream", "SupportsDirectPlay",
		"IsInfiniteStream", "RequiresOpening", "RequiresClosing",
	}
	for _, f := range requiredFields {
		_, ok := m[f]
		assert.True(t, ok, "MediaSource missing required field %q", f)
	}
}

func TestMediaSourceTranscodingFields(t *testing.T) {
	ms := MediaSource{
		Protocol:               "Http",
		ID:                     "test",
		Type:                   "Default",
		Name:                   "Default",
		SupportsTranscoding:    true,
		TranscodingURL:         "/Videos/test/master.m3u8",
		TranscodingSubProtocol: "hls",
		TranscodingContainer:   "ts",
	}

	data, err := json.Marshal(ms)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, "hls", m["TranscodingSubProtocol"])
	assert.Equal(t, "ts", m["TranscodingContainer"])
	assert.NotEmpty(t, m["TranscodingUrl"])
}

func TestBaseItemDtoRequiredFields(t *testing.T) {
	item := BaseItemDto{
		Name:     "Test Movie",
		ServerID: "abcd1234abcd1234abcd1234abcd1234",
		ID:       "1234abcd1234abcd1234abcd1234abcd",
		Type:     "Movie",
		IsFolder: false,
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	requiredFields := []string{"Name", "ServerId", "Id", "Type", "IsFolder"}
	for _, f := range requiredFields {
		_, ok := m[f]
		assert.True(t, ok, "BaseItemDto missing required field %q", f)
	}
}

func TestBaseItemDtoOmitEmptyFields(t *testing.T) {
	item := BaseItemDto{
		Name:     "Test",
		ServerID: "abcd1234abcd1234abcd1234abcd1234",
		ID:       "1234abcd1234abcd1234abcd1234abcd",
		Type:     "Movie",
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	omitEmptyFields := []string{
		"Etag", "DateCreated", "Container", "SortName", "PremiereDate",
		"Path", "OfficialRating", "Overview", "Genres", "CollectionType",
		"ParentId", "ImageTags", "BackdropImageTags", "MediaType",
	}
	for _, f := range omitEmptyFields {
		_, ok := m[f]
		assert.False(t, ok, "field %q should be omitted when empty", f)
	}
}

func TestUserItemDataJSONShape(t *testing.T) {
	ud := UserItemData{
		PlaybackPositionTicks: 0,
		PlayCount:             0,
		IsFavorite:            false,
		Played:                false,
		Key:                   "test-key",
	}

	data, err := json.Marshal(ud)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	requiredFields := []string{"PlaybackPositionTicks", "PlayCount", "IsFavorite", "Played", "Key"}
	for _, f := range requiredFields {
		_, ok := m[f]
		assert.True(t, ok, "UserItemData missing required field %q", f)
	}
}

func TestMediaStreamJSONShape(t *testing.T) {
	ms := MediaStream{
		Type:      "Video",
		Codec:     "h264",
		Index:     0,
		IsDefault: true,
		Width:     1920,
		Height:    1080,
	}

	data, err := json.Marshal(ms)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	requiredFields := []string{"Codec", "Type", "Index", "IsDefault", "IsForced", "IsExternal"}
	for _, f := range requiredFields {
		_, ok := m[f]
		assert.True(t, ok, "MediaStream missing required field %q", f)
	}
}

func TestTypesMatchTVProxy(t *testing.T) {
	t.Run("MediaSource JSON keys match", func(t *testing.T) {
		ms := MediaSource{
			Protocol:                "Http",
			ID:                      "test",
			Type:                    "Default",
			Container:               "mp4",
			Name:                    "Default",
			IsRemote:                true,
			SupportsTranscoding:     true,
			SupportsDirectStream:    false,
			SupportsDirectPlay:      false,
			TranscodingURL:          "/test",
			TranscodingSubProtocol:  "hls",
			TranscodingContainer:    "ts",
			DefaultAudioStreamIndex: 1,
			MediaStreams:             []MediaStream{{Type: "Video", Codec: "h264"}},
		}

		data, err := json.Marshal(ms)
		require.NoError(t, err)

		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m))

		tvproxyKeys := []string{
			"Protocol", "Id", "Type", "Container", "Name", "IsRemote",
			"SupportsTranscoding", "SupportsDirectStream", "SupportsDirectPlay",
			"IsInfiniteStream", "RequiresOpening", "RequiresClosing",
			"TranscodingUrl", "TranscodingSubProtocol", "TranscodingContainer",
			"DefaultAudioStreamIndex", "MediaStreams",
		}
		for _, k := range tvproxyKeys {
			_, ok := m[k]
			assert.True(t, ok, "missing tvproxy-compatible key %q", k)
		}
	})

	t.Run("BaseItemDto struct has tvproxy-compatible fields for CollectionFolder", func(t *testing.T) {
		srv := newTestServerFull()
		item := srv.newCollectionFolderItem("Movies", viewMoviesID, "movies", map[string]string{})

		assert.NotEmpty(t, item.Name)
		assert.NotEmpty(t, item.ServerID)
		assert.NotEmpty(t, item.ID)
		assert.NotEmpty(t, item.DateCreated)
		assert.NotEmpty(t, item.DateLastMediaAdded)
		assert.NotEmpty(t, item.SortName)
		assert.True(t, item.IsFolder)
		assert.Equal(t, "CollectionFolder", item.Type)
		assert.Equal(t, "movies", item.CollectionType)
		assert.NotEmpty(t, item.ParentID)
		assert.NotNil(t, item.ImageTags)
		assert.NotNil(t, item.BackdropImageTags)
		assert.NotNil(t, item.ImageBlurHashes)
		assert.Equal(t, "FileSystem", item.LocationType)
		assert.Equal(t, "Unknown", item.MediaType)
		assert.NotNil(t, item.UserData)
		assert.NotNil(t, item.Genres)
		assert.NotNil(t, item.ExternalUrls)
		assert.NotNil(t, item.RemoteTrailers)
		assert.NotNil(t, item.ProviderIds)
		assert.NotNil(t, item.Tags)
		assert.NotNil(t, item.LockedFields)
		assert.NotNil(t, item.People)
		assert.NotNil(t, item.Studios)
		assert.NotNil(t, item.GenreItems)
		assert.NotNil(t, item.Taglines)
		assert.Equal(t, "Full", item.PlayAccess)
		assert.True(t, item.EnableMediaSourceDisplay)
		assert.NotNil(t, item.CanDelete)
		assert.NotNil(t, item.CanDownload)
		assert.NotNil(t, item.LockData)
		assert.NotEmpty(t, item.DisplayPreferencesId)
	})

	t.Run("PlaybackInfo response shape matches tvproxy", func(t *testing.T) {
		srv := newTestServerFull()
		itemID := stripDashes("aaaaaaaa-1111-2222-3333-444444444444")
		req := httptest.NewRequest(http.MethodPost, "/Items/"+itemID+"/PlaybackInfo", bytes.NewBufferString("{}"))
		req.SetPathValue("itemId", itemID)
		w := httptest.NewRecorder()

		srv.playbackInfo(w, req)

		var result map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

		_, ok := result["MediaSources"]
		assert.True(t, ok, "PlaybackInfo must have MediaSources")
		_, ok = result["PlaySessionId"]
		assert.True(t, ok, "PlaybackInfo must have PlaySessionId")

		sources := result["MediaSources"].([]any)
		require.Len(t, sources, 1)

		ms := sources[0].(map[string]any)
		assert.Equal(t, "Http", ms["Protocol"])
		assert.Equal(t, "Default", ms["Type"])
		assert.Equal(t, "mp4", ms["Container"])
		assert.Equal(t, true, ms["IsRemote"])
		assert.Equal(t, true, ms["SupportsTranscoding"])
		assert.Equal(t, false, ms["SupportsDirectStream"])
		assert.Equal(t, false, ms["SupportsDirectPlay"])
		assert.Equal(t, "hls", ms["TranscodingSubProtocol"])
		assert.Equal(t, "ts", ms["TranscodingContainer"])
	})
}

func TestPublicSystemInfoJSONKeys(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/System/Info/Public", nil)
	w := httptest.NewRecorder()

	srv.systemInfoPublic(w, req)

	var m map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&m))

	tvproxyKeys := []string{
		"LocalAddress", "ServerName", "Version", "ProductName",
		"OperatingSystem", "Id", "StartupWizardCompleted",
	}
	for _, k := range tvproxyKeys {
		_, ok := m[k]
		assert.True(t, ok, "missing tvproxy-compatible key %q", k)
	}
}

func TestUserDtoPolicyArraysNotNull(t *testing.T) {
	policy := defaultPolicy(true)
	data, err := json.Marshal(policy)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	policyArrayFields := []string{
		"BlockedTags", "AllowedTags", "AccessSchedules", "BlockUnratedItems",
		"EnableContentDeletionFromFolders", "EnabledDevices", "EnabledChannels",
		"EnabledFolders", "BlockedMediaFolders", "BlockedChannels",
	}
	for _, f := range policyArrayFields {
		v, ok := m[f]
		if !ok {
			t.Errorf("Policy missing field %q", f)
			continue
		}
		_, isArr := v.([]any)
		assert.True(t, isArr, "Policy.%s must be array, got %T", f, v)
	}
}

func TestUserConfigArraysNotNull(t *testing.T) {
	cfg := defaultUserConfig()
	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	cfgArrayFields := []string{"GroupedFolders", "OrderedViews", "LatestItemsExcludes", "MyMediaExcludes"}
	for _, f := range cfgArrayFields {
		v, ok := m[f]
		if !ok {
			t.Errorf("Configuration missing field %q", f)
			continue
		}
		_, isArr := v.([]any)
		assert.True(t, isArr, "Configuration.%s must be array, got %T", f, v)
	}
}

func TestSessionInfoArraysNotNull(t *testing.T) {
	si := SessionInfo{
		AdditionalUsers:          []any{},
		PlayableMediaTypes:       []string{},
		ID:                       "test",
		UserID:                   "test",
		NowPlayingQueue:          []any{},
		NowPlayingQueueFullItems: []any{},
		SupportedCommands:        []string{},
	}

	data, err := json.Marshal(si)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	arrayFields := []string{
		"AdditionalUsers", "PlayableMediaTypes",
		"NowPlayingQueue", "NowPlayingQueueFullItems", "SupportedCommands",
	}
	for _, f := range arrayFields {
		v, ok := m[f]
		if !ok {
			t.Errorf("SessionInfo missing field %q", f)
			continue
		}
		_, isArr := v.([]any)
		assert.True(t, isArr, "SessionInfo.%s must be array, got %T", f, v)
	}
}

func TestBrandingConfigurationShape(t *testing.T) {
	bc := BrandingConfiguration{}
	data, err := json.Marshal(bc)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	for _, k := range []string{"LoginDisclaimer", "CustomCss", "SplashscreenEnabled"} {
		_, ok := m[k]
		assert.True(t, ok, "BrandingConfiguration missing key %q", k)
	}
}

func TestAuthenticationResultShape(t *testing.T) {
	srv := newTestServerWithAuth()
	body := `{"Username":"admin","Pw":"password"}`
	req := httptest.NewRequest(http.MethodPost, "/Users/AuthenticateByName", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.authenticateByName(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var m map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&m))

	requiredKeys := []string{"User", "SessionInfo", "AccessToken", "ServerId"}
	for _, k := range requiredKeys {
		_, ok := m[k]
		assert.True(t, ok, "AuthenticationResult missing key %q", k)
	}

	user, ok := m["User"].(map[string]any)
	require.True(t, ok)
	userID, _ := user["Id"].(string)
	assert.Equal(t, 32, len(userID), "User.Id must be 32 chars")

	serverID, _ := m["ServerId"].(string)
	assert.Equal(t, 32, len(serverID), "ServerId must be 32 chars")

	token, _ := m["AccessToken"].(string)
	assert.Equal(t, 32, len(token), "AccessToken must be 32 chars")
}

func TestCollectionFolderUserData(t *testing.T) {
	srv := newTestServerFull()
	item := srv.newCollectionFolderItem("Movies", viewMoviesID, "movies", map[string]string{})

	require.NotNil(t, item.UserData)
	assert.NotEmpty(t, item.UserData.Key)
	assert.NotEmpty(t, item.UserData.ItemID)

	dashedID := addDashes(item.ID)
	assert.Equal(t, dashedID, item.UserData.Key)
	assert.Equal(t, item.ID, item.UserData.ItemID)
}

func TestCollectionFolderDisplayPreferencesId(t *testing.T) {
	srv := newTestServerFull()
	item := srv.newCollectionFolderItem("Movies", viewMoviesID, "movies", map[string]string{})
	assert.Equal(t, item.ID, item.DisplayPreferencesId)
}

func TestRootFolderIDIs32CharHex(t *testing.T) {
	srv := newTestServer()
	id := srv.rootFolderID()
	assertHex32(t, "rootFolderID", id)
}

func TestRootFolderIDDeterministic(t *testing.T) {
	srv := newTestServer()
	id1 := srv.rootFolderID()
	id2 := srv.rootFolderID()
	assert.Equal(t, id1, id2)
}

func TestMediaSourceFieldNaming(t *testing.T) {
	ms := MediaSource{
		Protocol:               "Http",
		ID:                     "test123",
		TranscodingURL:         "/test",
		TranscodingSubProtocol: "hls",
		TranscodingContainer:   "ts",
	}

	data, err := json.Marshal(ms)
	require.NoError(t, err)

	raw := string(data)

	assert.Contains(t, raw, `"Id"`)
	assert.NotContains(t, raw, `"ID"`)

	assert.Contains(t, raw, `"TranscodingUrl":`)
	assert.NotContains(t, raw, `"TranscodingURL":`)

	assert.Contains(t, raw, `"SupportsTranscoding"`)
	assert.Contains(t, raw, `"SupportsDirectStream"`)
	assert.Contains(t, raw, `"SupportsDirectPlay"`)
}

func TestLiveTvChannelItemMediaSource(t *testing.T) {
	srv := newTestServerFull()
	req := httptest.NewRequest(http.MethodGet, "/LiveTv/Channels", nil)
	w := httptest.NewRecorder()
	srv.liveTvChannels(w, req)

	var result BaseItemDtoQueryResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))

	for _, item := range result.Items {
		require.NotEmpty(t, item.MediaSources, "LiveTvChannel %q must have MediaSources", item.Name)
		ms := item.MediaSources[0]
		assert.Equal(t, "Http", ms.Protocol)
		assert.True(t, ms.IsInfiniteStream || ms.SupportsTranscoding)
		assert.Contains(t, ms.TranscodingURL, "/Videos/")
	}
}
