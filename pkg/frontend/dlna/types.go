package dlna

import "encoding/xml"

type SoapEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    SoapBody `xml:"Body"`
}

type SoapBody struct {
	Content []byte `xml:",innerxml"`
}

type BrowseRequest struct {
	ObjectID       string `xml:"ObjectID"`
	BrowseFlag     string `xml:"BrowseFlag"`
	Filter         string `xml:"Filter"`
	StartingIndex  int    `xml:"StartingIndex"`
	RequestedCount int    `xml:"RequestedCount"`
	SortCriteria   string `xml:"SortCriteria"`
}

type ChannelItem struct {
	ID      string
	Name    string
	LogoURL string
	GroupID string
}

type GroupItem struct {
	ID   string
	Name string
}
