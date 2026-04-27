package xmltv

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type Channel struct {
	ID          string
	DisplayName string
	Icon        string
}

type Programme struct {
	ChannelID   string
	Title       string
	Subtitle    string
	Description string
	Start       time.Time
	Stop        time.Time
	Categories  []string
	Rating      string
	EpisodeNum  string
	IsNew       bool
	Credits     Credits
}

type Credits struct {
	Directors []string
	Actors    []string
}

type xmlTV struct {
	XMLName    xml.Name       `xml:"tv"`
	Channels   []xmlChannel   `xml:"channel"`
	Programmes []xmlProgramme `xml:"programme"`
}

type xmlChannel struct {
	ID          string  `xml:"id,attr"`
	DisplayName string  `xml:"display-name"`
	Icon        xmlIcon `xml:"icon"`
}

type xmlIcon struct {
	Src string `xml:"src,attr"`
}

type xmlProgramme struct {
	Start           string          `xml:"start,attr"`
	Stop            string          `xml:"stop,attr"`
	Channel         string          `xml:"channel,attr"`
	Title           string          `xml:"title"`
	SubTitle        string          `xml:"sub-title"`
	Desc            string          `xml:"desc"`
	Categories      []string        `xml:"category"`
	Rating          xmlRating       `xml:"rating"`
	EpisodeNum      xmlEpisodeNum   `xml:"episode-num"`
	PreviouslyShown *xmlPrevShown   `xml:"previously-shown"`
	Credits         xmlCredits      `xml:"credits"`
}

type xmlRating struct {
	Value string `xml:"value"`
}

type xmlEpisodeNum struct {
	System string `xml:"system,attr"`
	Value  string `xml:",chardata"`
}

type xmlPrevShown struct{}

type xmlCredits struct {
	Directors []string `xml:"director"`
	Actors    []string `xml:"actor"`
}

func Parse(r io.Reader) ([]Channel, []Programme, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("reading input: %w", err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return []Channel{}, []Programme{}, nil
	}

	var tv xmlTV
	if err := xml.Unmarshal(data, &tv); err != nil {
		return nil, nil, fmt.Errorf("parsing XML: %w", err)
	}

	channels := make([]Channel, 0, len(tv.Channels))
	for _, xc := range tv.Channels {
		channels = append(channels, Channel{
			ID:          xc.ID,
			DisplayName: xc.DisplayName,
			Icon:        xc.Icon.Src,
		})
	}

	programmes := make([]Programme, 0, len(tv.Programmes))
	for _, xp := range tv.Programmes {
		start, err := parseXMLTVTime(xp.Start)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing start time %q: %w", xp.Start, err)
		}
		stop, err := parseXMLTVTime(xp.Stop)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing stop time %q: %w", xp.Stop, err)
		}

		categories := xp.Categories
		if categories == nil {
			categories = []string{}
		}

		directors := xp.Credits.Directors
		if directors == nil {
			directors = []string{}
		}
		actors := xp.Credits.Actors
		if actors == nil {
			actors = []string{}
		}

		episodeNum := strings.TrimSpace(xp.EpisodeNum.Value)
		isNew := episodeNum != "" && xp.PreviouslyShown == nil

		programmes = append(programmes, Programme{
			ChannelID:   xp.Channel,
			Title:       xp.Title,
			Subtitle:    xp.SubTitle,
			Description: xp.Desc,
			Start:       start,
			Stop:        stop,
			Categories:  categories,
			Rating:      xp.Rating.Value,
			EpisodeNum:  episodeNum,
			IsNew:       isNew,
			Credits: Credits{
				Directors: directors,
				Actors:    actors,
			},
		})
	}

	return channels, programmes, nil
}

func parseXMLTVTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, " ", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid XMLTV time format: %q", s)
	}

	dt := parts[0]
	tz := parts[1]

	if len(dt) != 14 {
		return time.Time{}, fmt.Errorf("invalid XMLTV datetime: %q", dt)
	}

	year, err := strconv.Atoi(dt[0:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid year: %w", err)
	}
	month, err := strconv.Atoi(dt[4:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month: %w", err)
	}
	day, err := strconv.Atoi(dt[6:8])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day: %w", err)
	}
	hour, err := strconv.Atoi(dt[8:10])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid hour: %w", err)
	}
	min, err := strconv.Atoi(dt[10:12])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid minute: %w", err)
	}
	sec, err := strconv.Atoi(dt[12:14])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid second: %w", err)
	}

	if len(tz) != 5 {
		return time.Time{}, fmt.Errorf("invalid timezone offset: %q", tz)
	}

	sign := 1
	if tz[0] == '-' {
		sign = -1
	} else if tz[0] != '+' {
		return time.Time{}, fmt.Errorf("invalid timezone sign: %q", tz)
	}

	tzHours, err := strconv.Atoi(tz[1:3])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone hours: %w", err)
	}
	tzMins, err := strconv.Atoi(tz[3:5])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone minutes: %w", err)
	}

	offsetSeconds := sign * (tzHours*3600 + tzMins*60)
	loc := time.FixedZone(tz, offsetSeconds)

	return time.Date(year, time.Month(month), day, hour, min, sec, 0, loc), nil
}
