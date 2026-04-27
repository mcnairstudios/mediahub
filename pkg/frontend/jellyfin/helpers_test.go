package jellyfin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJellyfinID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "aaaaaaaabbbbccccddddeeeeeeeeeeee"},
		{"1234567890abcdef1234567890abcdef", "1234567890abcdef1234567890abcdef"},
		{"no-dashes", "nodashes"},
	}

	for _, tt := range tests {
		result := jellyfinID(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestJellyfinIDLength(t *testing.T) {
	uuid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	result := jellyfinID(uuid)
	assert.Equal(t, 32, len(result))
}

func TestStripDashes(t *testing.T) {
	assert.Equal(t, "aaaaaaaabbbbccccddddeeeeeeeeeeee", stripDashes("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"))
	assert.Equal(t, "nodashes", stripDashes("nodashes"))
}

func TestAddDashes(t *testing.T) {
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", addDashes("aaaaaaaabbbbccccddddeeeeeeeeeeee"))
	assert.Equal(t, "short", addDashes("short"))
}

func TestAddDashesRoundTrip(t *testing.T) {
	uuid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	assert.Equal(t, uuid, addDashes(stripDashes(uuid)))
}

func TestSeriesIDFromName(t *testing.T) {
	id := seriesIDFromName("Breaking Bad")
	assert.Equal(t, 32, len(id))
	assert.True(t, len(id) == 32 && id[:4] == "cccc")

	id2 := seriesIDFromName("Breaking Bad")
	assert.Equal(t, id, id2)

	id3 := seriesIDFromName("Better Call Saul")
	assert.NotEqual(t, id, id3)
}

func TestSeasonItemID(t *testing.T) {
	id := seasonItemID("Breaking Bad", 3)
	assert.Equal(t, 32, len(id))
	assert.True(t, id[:4] == "cccd")
	assert.True(t, isSeasonItemID(id))

	h, num, ok := parseSeasonItemID(id)
	assert.True(t, ok)
	assert.Equal(t, 3, num)
	assert.Equal(t, hashString("Breaking Bad"), h)
}

func TestIsSeasonItemID(t *testing.T) {
	assert.True(t, isSeasonItemID(seasonItemID("Test", 1)))
	assert.False(t, isSeasonItemID("not-a-season-id"))
	assert.False(t, isSeasonItemID("cccc00000000000000000000000000ff"))
	assert.False(t, isSeasonItemID("short"))
}

func TestGroupItemID(t *testing.T) {
	uuid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	id := groupItemID(uuid)
	assert.Equal(t, 32, len(id))
	assert.True(t, isGroupItemID(id))
	assert.True(t, id[:4] == "bbbb")
}

func TestIsGroupItemID(t *testing.T) {
	assert.True(t, isGroupItemID(groupItemID("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")))
	assert.False(t, isGroupItemID("not-a-group-id"))
	assert.False(t, isGroupItemID("cccc00000000000000000000000000ff"))
}

func TestSortName(t *testing.T) {
	assert.Equal(t, "Matrix", sortName("The Matrix"))
	assert.Equal(t, "Beautiful Mind", sortName("A Beautiful Mind"))
	assert.Equal(t, "Unexpected Journey", sortName("An Unexpected Journey"))
	assert.Equal(t, "Inception", sortName("Inception"))
}

func TestHashStringDeterministic(t *testing.T) {
	h1 := hashString("test")
	h2 := hashString("test")
	assert.Equal(t, h1, h2)

	h3 := hashString("different")
	assert.NotEqual(t, h1, h3)
}

func TestEmptyResultNonNil(t *testing.T) {
	r := emptyResult()
	assert.NotNil(t, r.Items)
	assert.Equal(t, 0, len(r.Items))
}

func TestGenreItems(t *testing.T) {
	items := genreItems([]string{"Action", "Drama"})
	assert.Len(t, items, 2)
	assert.Equal(t, "Action", items[0].Name)
	assert.NotEmpty(t, items[0].ID)
	assert.Equal(t, "Drama", items[1].Name)
}

func TestDurationToTicks(t *testing.T) {
	assert.Equal(t, int64(72000000000), secondsToTicks(7200))
}

func TestSortItems(t *testing.T) {
	items := []BaseItemDto{
		{Name: "Zebra", SortName: "Zebra"},
		{Name: "Alpha", SortName: "Alpha"},
		{Name: "Middle", SortName: "Middle"},
	}

	sortItems(items, "SortName", "Ascending")
	assert.Equal(t, "Alpha", items[0].Name)
	assert.Equal(t, "Middle", items[1].Name)
	assert.Equal(t, "Zebra", items[2].Name)

	sortItems(items, "SortName", "Descending")
	assert.Equal(t, "Zebra", items[0].Name)
	assert.Equal(t, "Alpha", items[2].Name)
}

func TestFirstOf(t *testing.T) {
	req := map[string][]string{
		"a": {""},
		"b": {"value"},
	}
	v := make(map[string][]string)
	for k, vals := range req {
		v[k] = vals
	}
	result := firstOf(v, "a", "b")
	assert.Equal(t, "value", result)
}

func TestMatchesGenres(t *testing.T) {
	assert.True(t, matchesGenres([]string{"Action", "Drama"}, []string{"Action"}))
	assert.True(t, matchesGenres([]string{"Action", "Drama"}, []string{"action"}))
	assert.False(t, matchesGenres([]string{"Action"}, []string{"Comedy"}))
	assert.True(t, matchesGenres([]string{"Action"}, nil))
}
