// Package fritzbox – TR-064 SOAP client for Fritz!Box.
// TR-064 uses HTTP with Digest Auth and SOAP envelopes.
// Service URIs and control URLs are hardcoded from AVM's TR-064 documentation:
// https://fritz.com/en/pages/interfaces
package fritzbox

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Known TR-064 service types and their control URL paths.
// The path segment after /upnp/control/ matches the service shortname on Fritz!Box.
const (
	// System
	svcDeviceInfo   = "urn:dslforum-org:service:DeviceInfo:1"
	ctlDeviceInfo   = "/upnp/control/deviceinfo"
	svcDeviceConfig = "urn:dslforum-org:service:DeviceConfig:1"
	ctlDeviceConfig = "/upnp/control/deviceconfig"

	// Network – WLAN
	svcWLAN1 = "urn:dslforum-org:service:WLANConfiguration:1"
	ctlWLAN1 = "/upnp/control/wlanconfig1"
	svcWLAN2 = "urn:dslforum-org:service:WLANConfiguration:2"
	ctlWLAN2 = "/upnp/control/wlanconfig2"
	svcWLAN3 = "urn:dslforum-org:service:WLANConfiguration:3"
	ctlWLAN3 = "/upnp/control/wlanconfig3"
	svcWLAN4 = "urn:dslforum-org:service:WLANConfiguration:4"
	ctlWLAN4 = "/upnp/control/wlanconfig4"

	// Network – Hosts / WOL
	svcHosts = "urn:dslforum-org:service:Hosts:1"
	ctlHosts = "/upnp/control/hosts"

	// Network – WANIPConn (port forward)
	svcWANIPConn = "urn:dslforum-org:service:WANIPConnection:1"
	ctlWANIPConn = "/upnp/control/wanipconnection1"

	// Telephony
	svcOnTel = "urn:dslforum-org:service:X_AVM-DE_OnTel:1"
	ctlOnTel = "/upnp/control/x_contact"
	svcTAM   = "urn:dslforum-org:service:X_AVM-DE_TAM:1"
	ctlTAM   = "/upnp/control/x_tam"

	// Storage
	svcStorage = "urn:dslforum-org:service:X_AVM-DE_Storage:1"
	ctlStorage = "/upnp/control/x_storage"

	// Media / TV (Cable models only)
	svcMedia = "urn:dslforum-org:service:X_AVM-DE_Media:1"
	ctlMedia = "/upnp/control/x_media"
)

// TR064Client performs SOAP calls against a Fritz!Box TR-064 interface.
type TR064Client struct {
	baseURL    string
	httpClient *http.Client
}

// newTR064Client sets up the SOAP client with Digest Auth.
func newTR064Client(baseURL, username, password string, timeout time.Duration) *TR064Client {
	transport := NewDigestTransport(username, password, nil)
	return &TR064Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// CallAction executes a TR-064 SOAP action and returns the response values.
//
//   - serviceType: full URN, e.g. "urn:dslforum-org:service:DeviceInfo:1"
//   - controlURL:  path, e.g. "/upnp/control/deviceinfo"
//   - action:      action name, e.g. "GetInfo"
//   - args:        input arguments (may be nil)
func (c *TR064Client) CallAction(serviceType, controlURL, action string, args map[string]string) (map[string]string, error) {
	envelope := buildSOAPEnvelope(serviceType, action, args)
	url := c.baseURL + controlURL

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(envelope))
	if err != nil {
		return nil, fmt.Errorf("tr064: build request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SoapAction", fmt.Sprintf(`"%s#%s"`, serviceType, action))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tr064: request %s#%s: %w", serviceType, action, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tr064: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		fault := parseSoapFault(body)
		return nil, fmt.Errorf("tr064: HTTP %d – %s", resp.StatusCode, fault)
	}

	result, err := parseSOAPResponse(body, action)
	if err != nil {
		return nil, fmt.Errorf("tr064: parse response for %s#%s: %w", serviceType, action, err)
	}
	return result, nil
}

// ──────────────────────────────────────────────
// SOAP helpers
// ──────────────────────────────────────────────

func buildSOAPEnvelope(serviceType, action string, args map[string]string) string {
	var argParts strings.Builder
	for k, v := range args {
		// Escape XML special characters in both key and value.
		argParts.WriteString(fmt.Sprintf("<%s>%s</%s>", xmlEscape(k), xmlEscape(v), xmlEscape(k)))
	}
	return fmt.Sprintf(
		`<?xml version="1.0" encoding="utf-8"?>`+
			`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" `+
			`s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">`+
			`<s:Body>`+
			`<u:%s xmlns:u="%s">%s</u:%s>`+
			`</s:Body></s:Envelope>`,
		xmlEscape(action), xmlEscape(serviceType), argParts.String(), xmlEscape(action),
	)
}

// soapResponseEnvelope parses the outer SOAP response envelope.
type soapResponseEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Inner []byte `xml:",innerxml"`
	} `xml:"Body"`
}

// parseSOAPResponse extracts all child elements from the <u:{action}Response> element.
func parseSOAPResponse(body []byte, action string) (map[string]string, error) {
	var env soapResponseEnvelope
	if err := xml.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("unmarshal soap envelope: %w", err)
	}

	// Wrap inner XML in a root element for parsing.
	wrapped := []byte("<root>" + string(env.Body.Inner) + "</root>")
	dec := xml.NewDecoder(strings.NewReader(string(wrapped)))
	result := make(map[string]string)
	var depth int
	var curKey string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 3 { // depth 1=root, 2={action}Response, 3=fields
				curKey = t.Name.Local
			}
		case xml.CharData:
			if depth == 3 && curKey != "" {
				result[curKey] = strings.TrimSpace(string(t))
			}
		case xml.EndElement:
			if depth == 3 {
				curKey = ""
			}
			depth--
		}
	}
	return result, nil
}

// soapFaultEnvelope extracts the fault string from a SOAP fault response.
type soapFaultEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Fault struct {
			FaultString string `xml:"faultstring"`
			Detail      struct {
				UPnPError struct {
					ErrorCode        string `xml:"errorCode"`
					ErrorDescription string `xml:"errorDescription"`
				} `xml:"UPnPError"`
			} `xml:"detail"`
		} `xml:"Fault"`
	} `xml:"Body"`
}

func parseSoapFault(body []byte) string {
	var f soapFaultEnvelope
	if err := xml.Unmarshal(body, &f); err != nil {
		return "(unparseable fault)"
	}
	if f.Body.Fault.Detail.UPnPError.ErrorCode != "" {
		return fmt.Sprintf("UPnP error %s: %s",
			f.Body.Fault.Detail.UPnPError.ErrorCode,
			f.Body.Fault.Detail.UPnPError.ErrorDescription)
	}
	return f.Body.Fault.FaultString
}

// xmlEscape replaces XML special characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
