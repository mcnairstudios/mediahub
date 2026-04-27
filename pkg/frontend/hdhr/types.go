package hdhr

import "encoding/xml"

type DiscoverResponse struct {
	FriendlyName    string `json:"FriendlyName"`
	Manufacturer    string `json:"Manufacturer"`
	ManufacturerURL string `json:"ManufacturerURL"`
	ModelNumber     string `json:"ModelNumber"`
	FirmwareName    string `json:"FirmwareName"`
	FirmwareVersion string `json:"FirmwareVersion"`
	DeviceID        string `json:"DeviceID"`
	DeviceAuth      string `json:"DeviceAuth"`
	BaseURL         string `json:"BaseURL"`
	LineupURL       string `json:"LineupURL"`
	TunerCount      int    `json:"TunerCount"`
}

type LineupEntry struct {
	GuideNumber string `json:"GuideNumber"`
	GuideName   string `json:"GuideName"`
	VideoCodec  string `json:"VideoCodec,omitempty"`
	AudioCodec  string `json:"AudioCodec,omitempty"`
	HD          int    `json:"HD,omitempty"`
	URL         string `json:"URL"`
}

type LineupStatus struct {
	ScanInProgress int      `json:"ScanInProgress"`
	ScanPossible   int      `json:"ScanPossible"`
	Source         string   `json:"Source"`
	SourceList     []string `json:"SourceList"`
}

type DeviceXML struct {
	XMLName     xml.Name       `xml:"root"`
	XMLNS       string         `xml:"xmlns,attr"`
	URLBase     string         `xml:"URLBase"`
	SpecVersion specVersionXML `xml:"specVersion"`
	Device      deviceInnerXML `xml:"device"`
}

type specVersionXML struct {
	Major int `xml:"major"`
	Minor int `xml:"minor"`
}

type deviceInnerXML struct {
	DeviceType   string `xml:"deviceType"`
	FriendlyName string `xml:"friendlyName"`
	Manufacturer string `xml:"manufacturer"`
	ModelName    string `xml:"modelName"`
	ModelNumber  string `xml:"modelNumber"`
	SerialNumber string `xml:"serialNumber"`
	UDN          string `xml:"UDN"`
}
