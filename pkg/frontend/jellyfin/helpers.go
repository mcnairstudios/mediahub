package jellyfin

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func stripDashes(id string) string {
	return strings.ReplaceAll(id, "-", "")
}

func addDashes(id string) string {
	id = strings.ReplaceAll(id, "-", "")
	if len(id) == 32 {
		return id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
	}
	return id
}

func hashString(s string) uint32 {
	var h uint32
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

func sortName(name string) string {
	lower := strings.ToLower(name)
	for _, prefix := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(lower, prefix) {
			return name[len(prefix):]
		}
	}
	return name
}

func seriesIDFromName(name string) string {
	h := hashString(name)
	return fmt.Sprintf("cccc%028x", h)
}

func seasonItemID(seriesName string, seasonNum int) string {
	h := hashString(seriesName)
	return fmt.Sprintf("cccd%024x%04x", h, seasonNum)
}

func isSeasonItemID(id string) bool {
	return len(id) == 32 && strings.HasPrefix(id, "cccd")
}

func parseSeasonItemID(id string) (seriesHash uint32, seasonNum int, ok bool) {
	if !isSeasonItemID(id) {
		return 0, 0, false
	}
	var h uint32
	var n int
	if _, err := fmt.Sscanf(id[4:28], "%x", &h); err != nil {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(id[28:], "%x", &n); err != nil {
		return 0, 0, false
	}
	return h, n, true
}

func groupItemID(uuid string) string {
	stripped := stripDashes(uuid)
	if len(stripped) >= 28 {
		return "bbbb" + stripped[:28]
	}
	return "bbbb" + fmt.Sprintf("%-28s", stripped)[:28]
}

func isGroupItemID(id string) bool {
	return len(id) == 32 && strings.HasPrefix(id, "bbbb")
}

func groupUUIDFromItemID(id string) string {
	if len(id) < 32 || !strings.HasPrefix(id, "bbbb") {
		return id
	}
	return addDashes(id[4:] + "0000")
}

func genreItems(genres []string) []NameIDPair {
	var items []NameIDPair
	for _, g := range genres {
		items = append(items, NameIDPair{Name: g, ID: fmt.Sprintf("genre_%x", hashString(g))})
	}
	return items
}

func firstOf(q url.Values, keys ...string) string {
	for _, k := range keys {
		if v := q.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func parseGenres(genres string) []string {
	if genres == "" {
		return nil
	}
	return strings.Split(genres, ",")
}

func matchesGenres(itemGenres, filter []string) bool {
	for _, f := range filter {
		found := false
		for _, g := range itemGenres {
			if strings.EqualFold(g, strings.TrimSpace(f)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func sortItems(items []BaseItemDto, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "SortName"
	}
	if strings.Contains(sortBy, ",") {
		sortBy = strings.Split(sortBy, ",")[0]
	}
	desc := strings.EqualFold(sortOrder, "Descending")

	sort.Slice(items, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "DateCreated", "PremiereDate", "ProductionYear":
			less = items[i].PremiereDate < items[j].PremiereDate
		case "CommunityRating":
			less = items[i].CommunityRating < items[j].CommunityRating
		case "Runtime":
			less = items[i].RunTimeTicks < items[j].RunTimeTicks
		case "Random":
			less = hashString(items[i].ID)%1000 < hashString(items[j].ID)%1000
		default:
			si := items[i].SortName
			if si == "" {
				si = items[i].Name
			}
			sj := items[j].SortName
			if sj == "" {
				sj = items[j].Name
			}
			less = strings.ToLower(si) < strings.ToLower(sj)
		}
		if desc {
			return !less
		}
		return less
	})
}

func (s *Server) paginateAndRespond(w http.ResponseWriter, r *http.Request, items []BaseItemDto) {
	if items == nil {
		items = []BaseItemDto{}
	}
	total := len(items)

	startIndex, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
	limit := total
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	if startIndex >= total {
		items = []BaseItemDto{}
	} else if startIndex+limit > total {
		items = items[startIndex:]
	} else {
		items = items[startIndex : startIndex+limit]
	}

	s.respondJSON(w, http.StatusOK, BaseItemDtoQueryResult{
		Items: items, TotalRecordCount: total, StartIndex: startIndex,
	})
}

func emptyResult() BaseItemDtoQueryResult {
	return BaseItemDtoQueryResult{Items: []BaseItemDto{}, TotalRecordCount: 0}
}

func boolPtr(b bool) *bool {
	return &b
}

func durationToTicks(d time.Duration) int64 {
	return int64(d.Seconds() * 10000000)
}

func secondsToTicks(secs float64) int64 {
	return int64(secs * 10000000)
}

func channelStreamURL(itemID string) string {
	return fmt.Sprintf("/Videos/%s/stream.mp4?static=true", itemID)
}
