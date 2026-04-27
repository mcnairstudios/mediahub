package dlna

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

var dlnaNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

const didlHeader = `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:dlna="urn:schemas-dlna-org:metadata-1-0/">`
const emptyDIDL = `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"></DIDL-Lite>`

type ChannelLister interface {
	ListChannels(ctx context.Context) ([]ChannelItem, error)
	GetChannel(ctx context.Context, id string) (*ChannelItem, error)
	ListGroups(ctx context.Context) ([]GroupItem, error)
}

type SettingsChecker interface {
	IsEnabled(ctx context.Context) bool
}

type Server struct {
	channels ChannelLister
	settings SettingsChecker
	baseURL  string
	port     int
	log      zerolog.Logger
}

func NewServer(channels ChannelLister, settings SettingsChecker, baseURL string, port int, log zerolog.Logger) *Server {
	return &Server{
		channels: channels,
		settings: settings,
		baseURL:  baseURL,
		port:     port,
		log:      log.With().Str("frontend", "dlna").Logger(),
	}
}

func (s *Server) UDN() string {
	return "uuid:" + uuid.NewSHA1(dlnaNamespace, []byte("mediahub-dlna")).String()
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /dlna/device.xml", s.DeviceDescription)
	mux.HandleFunc("GET /dlna/ContentDirectory.xml", s.ContentDirectorySCPD)
	mux.HandleFunc("GET /dlna/ConnectionManager.xml", s.ConnectionManagerSCPD)
	mux.HandleFunc("POST /dlna/control/ContentDirectory", s.ContentDirectoryControl)
	mux.HandleFunc("POST /dlna/control/ConnectionManager", s.ConnectionManagerControl)
}

func (s *Server) DeviceDescription(w http.ResponseWriter, r *http.Request) {
	if !s.settings.IsEnabled(r.Context()) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write([]byte(s.deviceDescriptionXML()))
}

func (s *Server) ContentDirectorySCPD(w http.ResponseWriter, r *http.Request) {
	if !s.settings.IsEnabled(r.Context()) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write([]byte(contentDirectoryXML))
}

func (s *Server) ConnectionManagerSCPD(w http.ResponseWriter, r *http.Request) {
	if !s.settings.IsEnabled(r.Context()) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write([]byte(connectionManagerXML))
}

func (s *Server) ContentDirectoryControl(w http.ResponseWriter, r *http.Request) {
	if !s.settings.IsEnabled(r.Context()) {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	soapAction := r.Header.Get("SOAPAction")
	result, err := s.handleContentDirectoryAction(r.Context(), soapAction, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	w.Write([]byte(result))
}

func (s *Server) ConnectionManagerControl(w http.ResponseWriter, r *http.Request) {
	if !s.settings.IsEnabled(r.Context()) {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	soapAction := r.Header.Get("SOAPAction")
	result, err := s.handleConnectionManagerAction(soapAction, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	w.Write([]byte(result))
}

func (s *Server) streamBaseURL() string {
	return fmt.Sprintf("%s:%d", s.baseURL, s.port)
}

func (s *Server) deviceDescriptionXML() string {
	udn := s.UDN()
	return `<?xml version="1.0" encoding="UTF-8"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <deviceType>urn:schemas-upnp-org:device:MediaServer:1</deviceType>
    <friendlyName>MediaHub DLNA</friendlyName>
    <manufacturer>MediaHub</manufacturer>
    <modelName>MediaHub</modelName>
    <modelDescription>MediaHub DLNA MediaServer</modelDescription>
    <UDN>` + xmlEscape(udn) + `</UDN>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:ContentDirectory:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:ContentDirectory</serviceId>
        <SCPDURL>/dlna/ContentDirectory.xml</SCPDURL>
        <controlURL>/dlna/control/ContentDirectory</controlURL>
        <eventSubURL></eventSubURL>
      </service>
      <service>
        <serviceType>urn:schemas-upnp-org:service:ConnectionManager:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:ConnectionManager</serviceId>
        <SCPDURL>/dlna/ConnectionManager.xml</SCPDURL>
        <controlURL>/dlna/control/ConnectionManager</controlURL>
        <eventSubURL></eventSubURL>
      </service>
    </serviceList>
  </device>
</root>`
}

const contentDirectoryXML = `<?xml version="1.0" encoding="UTF-8"?>
<scpd xmlns="urn:schemas-upnp-org:service-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <actionList>
    <action>
      <name>Browse</name>
      <argumentList>
        <argument><name>ObjectID</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_ObjectID</relatedStateVariable></argument>
        <argument><name>BrowseFlag</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_BrowseFlag</relatedStateVariable></argument>
        <argument><name>Filter</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Filter</relatedStateVariable></argument>
        <argument><name>StartingIndex</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Index</relatedStateVariable></argument>
        <argument><name>RequestedCount</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
        <argument><name>SortCriteria</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_SortCriteria</relatedStateVariable></argument>
        <argument><name>Result</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Result</relatedStateVariable></argument>
        <argument><name>NumberReturned</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
        <argument><name>TotalMatches</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
        <argument><name>UpdateID</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_UpdateID</relatedStateVariable></argument>
      </argumentList>
    </action>
    <action><name>GetSearchCapabilities</name><argumentList>
      <argument><name>SearchCaps</name><direction>out</direction><relatedStateVariable>SearchCapabilities</relatedStateVariable></argument>
    </argumentList></action>
    <action><name>GetSortCapabilities</name><argumentList>
      <argument><name>SortCaps</name><direction>out</direction><relatedStateVariable>SortCapabilities</relatedStateVariable></argument>
    </argumentList></action>
    <action><name>GetSystemUpdateID</name><argumentList>
      <argument><name>Id</name><direction>out</direction><relatedStateVariable>SystemUpdateID</relatedStateVariable></argument>
    </argumentList></action>
  </actionList>
  <serviceStateTable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_ObjectID</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_Result</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_BrowseFlag</name><dataType>string</dataType><allowedValueList><allowedValue>BrowseMetadata</allowedValue><allowedValue>BrowseDirectChildren</allowedValue></allowedValueList></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_Filter</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_SortCriteria</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_Index</name><dataType>ui4</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_Count</name><dataType>ui4</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_UpdateID</name><dataType>ui4</dataType></stateVariable>
    <stateVariable sendEvents="yes"><name>SystemUpdateID</name><dataType>ui4</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>SearchCapabilities</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>SortCapabilities</name><dataType>string</dataType></stateVariable>
  </serviceStateTable>
</scpd>`

const connectionManagerXML = `<?xml version="1.0" encoding="UTF-8"?>
<scpd xmlns="urn:schemas-upnp-org:service-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <actionList>
    <action><name>GetProtocolInfo</name><argumentList>
      <argument><name>Source</name><direction>out</direction><relatedStateVariable>SourceProtocolInfo</relatedStateVariable></argument>
      <argument><name>Sink</name><direction>out</direction><relatedStateVariable>SinkProtocolInfo</relatedStateVariable></argument>
    </argumentList></action>
    <action><name>GetCurrentConnectionIDs</name><argumentList>
      <argument><name>ConnectionIDs</name><direction>out</direction><relatedStateVariable>CurrentConnectionIDs</relatedStateVariable></argument>
    </argumentList></action>
    <action><name>GetCurrentConnectionInfo</name><argumentList>
      <argument><name>ConnectionID</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_ConnectionID</relatedStateVariable></argument>
      <argument><name>RcsID</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_RcsID</relatedStateVariable></argument>
      <argument><name>AVTransportID</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_AVTransportID</relatedStateVariable></argument>
      <argument><name>ProtocolInfo</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_ProtocolInfo</relatedStateVariable></argument>
      <argument><name>PeerConnectionManager</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_ConnectionManager</relatedStateVariable></argument>
      <argument><name>PeerConnectionID</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_ConnectionID</relatedStateVariable></argument>
      <argument><name>Direction</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Direction</relatedStateVariable></argument>
      <argument><name>Status</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_ConnectionStatus</relatedStateVariable></argument>
    </argumentList></action>
  </actionList>
  <serviceStateTable>
    <stateVariable sendEvents="yes"><name>SourceProtocolInfo</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="yes"><name>SinkProtocolInfo</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="yes"><name>CurrentConnectionIDs</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_ConnectionStatus</name><dataType>string</dataType><allowedValueList><allowedValue>OK</allowedValue><allowedValue>ContentFormatMismatch</allowedValue><allowedValue>InsufficientBandwidth</allowedValue><allowedValue>UnreliableChannel</allowedValue><allowedValue>Unknown</allowedValue></allowedValueList></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_ConnectionManager</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_Direction</name><dataType>string</dataType><allowedValueList><allowedValue>Input</allowedValue><allowedValue>Output</allowedValue></allowedValueList></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_ProtocolInfo</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_ConnectionID</name><dataType>i4</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_AVTransportID</name><dataType>i4</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_RcsID</name><dataType>i4</dataType></stateVariable>
  </serviceStateTable>
</scpd>`

func (s *Server) handleContentDirectoryAction(ctx context.Context, soapAction string, body []byte) (string, error) {
	action := extractAction(soapAction)

	switch action {
	case "Browse":
		return s.handleBrowse(ctx, body)
	case "GetSearchCapabilities":
		return soapResponse("ContentDirectory", "GetSearchCapabilities", "<SearchCaps></SearchCaps>"), nil
	case "GetSortCapabilities":
		return soapResponse("ContentDirectory", "GetSortCapabilities", "<SortCaps></SortCaps>"), nil
	case "GetSystemUpdateID":
		return soapResponse("ContentDirectory", "GetSystemUpdateID", "<Id>1</Id>"), nil
	default:
		return "", fmt.Errorf("unsupported action: %s", action)
	}
}

func (s *Server) handleConnectionManagerAction(soapAction string, body []byte) (string, error) {
	action := extractAction(soapAction)

	switch action {
	case "GetProtocolInfo":
		return soapResponse("ConnectionManager", "GetProtocolInfo",
			"<Source>http-get:*:video/mp2t:*,http-get:*:video/mp4:*</Source><Sink></Sink>"), nil
	case "GetCurrentConnectionIDs":
		return soapResponse("ConnectionManager", "GetCurrentConnectionIDs", "<ConnectionIDs>0</ConnectionIDs>"), nil
	case "GetCurrentConnectionInfo":
		return soapResponse("ConnectionManager", "GetCurrentConnectionInfo",
			"<RcsID>-1</RcsID><AVTransportID>-1</AVTransportID><ProtocolInfo></ProtocolInfo>"+
				"<PeerConnectionManager></PeerConnectionManager><PeerConnectionID>-1</PeerConnectionID>"+
				"<Direction>Output</Direction><Status>OK</Status>"), nil
	default:
		return "", fmt.Errorf("unsupported action: %s", action)
	}
}

func (s *Server) handleBrowse(ctx context.Context, body []byte) (string, error) {
	var env SoapEnvelope
	if err := xml.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("parsing SOAP envelope: %w", err)
	}

	var req BrowseRequest
	if err := xml.Unmarshal(env.Body.Content, &req); err != nil {
		return "", fmt.Errorf("parsing Browse request: %w", err)
	}

	switch {
	case req.ObjectID == "0":
		return s.browseRoot(ctx, req.BrowseFlag)
	case strings.HasPrefix(req.ObjectID, "grp-"):
		return s.browseGroup(ctx, req.ObjectID, req.BrowseFlag, req.StartingIndex, req.RequestedCount)
	case strings.HasPrefix(req.ObjectID, "ch-"):
		return s.browseChannelItem(ctx, req.ObjectID, req.BrowseFlag)
	default:
		return soapBrowseResponse(xmlEscape(emptyDIDL), 0, 0), nil
	}
}

func (s *Server) browseRoot(ctx context.Context, browseFlag string) (string, error) {
	channels, err := s.channels.ListChannels(ctx)
	if err != nil {
		return "", fmt.Errorf("listing channels: %w", err)
	}
	groups, err := s.channels.ListGroups(ctx)
	if err != nil {
		return "", fmt.Errorf("listing groups: %w", err)
	}

	groupCounts := make(map[string]int)
	ungrouped := 0
	for _, ch := range channels {
		if ch.GroupID == "" {
			ungrouped++
		} else {
			groupCounts[ch.GroupID]++
		}
	}

	childCount := 0
	for _, g := range groups {
		if groupCounts[g.ID] > 0 {
			childCount++
		}
	}
	if ungrouped > 0 {
		childCount++
	}

	if browseFlag == "BrowseMetadata" {
		didl := `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">` +
			fmt.Sprintf(`<container id="0" parentID="-1" childCount="%d" restricted="1">`, childCount) +
			`<dc:title>MediaHub</dc:title><upnp:class>object.container</upnp:class>` +
			`</container></DIDL-Lite>`
		return soapBrowseResponse(xmlEscape(didl), 1, 1), nil
	}

	var b strings.Builder
	b.WriteString(didlHeader)
	for _, g := range groups {
		if c := groupCounts[g.ID]; c > 0 {
			b.WriteString(fmt.Sprintf(`<container id="grp-%s" parentID="0" childCount="%d" restricted="1">`,
				xmlEscape(g.ID), c))
			b.WriteString(fmt.Sprintf(`<dc:title>%s</dc:title>`, xmlEscape(g.Name)))
			b.WriteString(`<upnp:class>object.container</upnp:class></container>`)
		}
	}
	if ungrouped > 0 {
		b.WriteString(fmt.Sprintf(`<container id="grp-ungrouped" parentID="0" childCount="%d" restricted="1">`,
			ungrouped))
		b.WriteString(`<dc:title>Ungrouped</dc:title><upnp:class>object.container</upnp:class></container>`)
	}
	b.WriteString(`</DIDL-Lite>`)
	return soapBrowseResponse(xmlEscape(b.String()), childCount, childCount), nil
}

func (s *Server) browseGroup(ctx context.Context, objectID, browseFlag string, startIdx, reqCount int) (string, error) {
	groupID := strings.TrimPrefix(objectID, "grp-")

	channels, err := s.channels.ListChannels(ctx)
	if err != nil {
		return "", fmt.Errorf("listing channels: %w", err)
	}

	var matched []ChannelItem
	for _, ch := range channels {
		if groupID == "ungrouped" && ch.GroupID == "" {
			matched = append(matched, ch)
		} else if ch.GroupID == groupID {
			matched = append(matched, ch)
		}
	}

	if browseFlag == "BrowseMetadata" {
		title := "Ungrouped"
		if groupID != "ungrouped" {
			groups, err := s.channels.ListGroups(ctx)
			if err == nil {
				for _, g := range groups {
					if g.ID == groupID {
						title = g.Name
						break
					}
				}
			}
		}
		didl := didlHeader +
			fmt.Sprintf(`<container id="%s" parentID="0" childCount="%d" restricted="1">`, xmlEscape(objectID), len(matched)) +
			fmt.Sprintf(`<dc:title>%s</dc:title>`, xmlEscape(title)) +
			`<upnp:class>object.container</upnp:class></container></DIDL-Lite>`
		return soapBrowseResponse(xmlEscape(didl), 1, 1), nil
	}

	total := len(matched)
	if reqCount <= 0 {
		reqCount = total
	}
	end := startIdx + reqCount
	if end > total {
		end = total
	}
	if startIdx > total {
		startIdx = total
	}
	page := matched[startIdx:end]

	base := s.streamBaseURL()
	var b strings.Builder
	b.WriteString(didlHeader)
	for _, ch := range page {
		b.WriteString(fmt.Sprintf(`<item id="ch-%s" parentID="%s" restricted="1">`, xmlEscape(ch.ID), xmlEscape(objectID)))
		b.WriteString(fmt.Sprintf(`<dc:title>%s</dc:title>`, xmlEscape(ch.Name)))
		b.WriteString(`<upnp:class>object.item.videoItem.videoBroadcast</upnp:class>`)
		if ch.LogoURL != "" && strings.HasPrefix(ch.LogoURL, "http") {
			profile := logoProfile(ch.LogoURL)
			b.WriteString(fmt.Sprintf(`<upnp:albumArtURI dlna:profileID="%s">%s</upnp:albumArtURI>`, profile, xmlEscape(ch.LogoURL)))
		}
		b.WriteString(fmt.Sprintf(`<res protocolInfo="http-get:*:video/mp2t:DLNA.ORG_PN=MPEG_TS_SD_EU;DLNA.ORG_OP=00;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=89000000000000000000000000000000">%s/channel/%s</res>`,
			xmlEscape(base), xmlEscape(ch.ID)))
		b.WriteString(`</item>`)
	}
	b.WriteString(`</DIDL-Lite>`)
	return soapBrowseResponse(xmlEscape(b.String()), len(page), total), nil
}

func (s *Server) browseChannelItem(ctx context.Context, objectID, browseFlag string) (string, error) {
	if browseFlag != "BrowseMetadata" {
		return soapBrowseResponse(xmlEscape(emptyDIDL), 0, 0), nil
	}

	channelID := strings.TrimPrefix(objectID, "ch-")
	ch, err := s.channels.GetChannel(ctx, channelID)
	if err != nil || ch == nil {
		return soapBrowseResponse(xmlEscape(emptyDIDL), 0, 0), nil
	}

	parentID := "grp-ungrouped"
	if ch.GroupID != "" {
		parentID = "grp-" + ch.GroupID
	}

	base := s.streamBaseURL()
	var b strings.Builder
	b.WriteString(didlHeader)
	b.WriteString(fmt.Sprintf(`<item id="ch-%s" parentID="%s" restricted="1">`, xmlEscape(ch.ID), xmlEscape(parentID)))
	b.WriteString(fmt.Sprintf(`<dc:title>%s</dc:title>`, xmlEscape(ch.Name)))
	b.WriteString(`<upnp:class>object.item.videoItem.videoBroadcast</upnp:class>`)
	if ch.LogoURL != "" && strings.HasPrefix(ch.LogoURL, "http") {
		profile := logoProfile(ch.LogoURL)
		b.WriteString(fmt.Sprintf(`<upnp:albumArtURI dlna:profileID="%s">%s</upnp:albumArtURI>`, profile, xmlEscape(ch.LogoURL)))
	}
	b.WriteString(fmt.Sprintf(`<res protocolInfo="http-get:*:video/mp2t:DLNA.ORG_PN=MPEG_TS_SD_EU;DLNA.ORG_OP=00;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=89000000000000000000000000000000">%s/channel/%s</res>`,
		xmlEscape(base), xmlEscape(ch.ID)))
	b.WriteString(`</item></DIDL-Lite>`)
	return soapBrowseResponse(xmlEscape(b.String()), 1, 1), nil
}

func extractAction(soapAction string) string {
	soapAction = strings.Trim(soapAction, "\"")
	if i := strings.LastIndex(soapAction, "#"); i >= 0 {
		return soapAction[i+1:]
	}
	return soapAction
}

func soapResponse(service, action, innerXML string) string {
	ns := "urn:schemas-upnp-org:service:" + service + ":1"
	return `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
<s:Body><u:` + action + `Response xmlns:u="` + ns + `">` +
		innerXML + `</u:` + action + `Response></s:Body></s:Envelope>`
}

func soapBrowseResponse(didlEscaped string, numberReturned, totalMatches int) string {
	return soapResponse("ContentDirectory", "Browse",
		`<Result>`+didlEscaped+`</Result>`+
			`<NumberReturned>`+strconv.Itoa(numberReturned)+`</NumberReturned>`+
			`<TotalMatches>`+strconv.Itoa(totalMatches)+`</TotalMatches>`+
			`<UpdateID>1</UpdateID>`)
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func logoProfile(logoURL string) string {
	ext := strings.ToLower(filepath.Ext(strings.SplitN(logoURL, "?", 2)[0]))
	switch ext {
	case ".png":
		return "PNG_SM"
	case ".gif":
		return "GIF_LG"
	default:
		return "JPEG_SM"
	}
}
