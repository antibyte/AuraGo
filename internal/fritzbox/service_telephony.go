// Package fritzbox – telephony service calls.
// Covers: call list, phonebooks, call deflection, TAM (answering machine).
// Note: Call list and TAM entries contain caller/callee names and numbers –
// these are treated as external data and must NOT be passed to the LLM unescaped.
package fritzbox

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// CallEntry represents a single call record from the Fritz!Box call list.
type CallEntry struct {
	Type     string `json:"type"` // 1=incoming, 2=missed, 3=outgoing, 9=active
	Date     string `json:"date"`
	Name     string `json:"name"`     // caller name (external – must be wrapped in <external_data>)
	Number   string `json:"number"`   // caller number
	Called   string `json:"called"`   // called number
	Duration string `json:"duration"` // HH:MM
	Count    string `json:"count"`    // number of attempts for missed calls
}

// PhonebookEntry is a name-to-numbers mapping from the Fritz!Box phonebook.
type PhonebookEntry struct {
	Name    string   `json:"name"` // external – must be wrapped in <external_data>
	Numbers []string `json:"numbers"`
}

// TAMEntry represents an answering machine message.
type TAMEntry struct {
	Index    int    `json:"index"`
	Date     string `json:"date"`
	Name     string `json:"name"`   // caller name (external – must be wrapped in <external_data>)
	Number   string `json:"number"` // caller number
	Duration string `json:"duration"`
	Read     bool   `json:"read"`
	Path     string `json:"path"` // internal path (only for display, no direct access)
}

// GetCallList retrieves the call list as XML and parses into CallEntry slices.
func (c *Client) GetCallList() ([]CallEntry, error) {
	// TR-064 OnTel returns a URL to a call-list XML document.
	res, err := c.SOAP(svcOnTel, ctlOnTel, "GetCallList", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox telephony: GetCallList: %w", err)
	}
	listURL, ok := res["NewCallListURL"]
	if !ok || listURL == "" {
		return nil, fmt.Errorf("fritzbox telephony: GetCallList returned no URL")
	}

	return c.fetchCallListXML(listURL)
}

// callListDoc is the XML schema for the Fritz!Box call list.
type callListDoc struct {
	XMLName xml.Name    `xml:"root"`
	Calls   []callEntry `xml:"Call"`
}

type callEntry struct {
	Type     string `xml:"Type"`
	Date     string `xml:"Date"`
	Name     string `xml:"Name"`
	Number   string `xml:"Number"`
	Called   string `xml:"Called"`
	Duration string `xml:"Duration"`
	Count    string `xml:"Count"`
}

func (c *Client) fetchCallListXML(rawURL string) ([]CallEntry, error) {
	// Use the TR-064 base URL to build the full URL if rawURL is a relative path.
	if strings.HasPrefix(rawURL, "/") {
		rawURL = c.tr.baseURL + rawURL
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.tr.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch call list: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var doc callListDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse call list xml: %w", err)
	}

	entries := make([]CallEntry, 0, len(doc.Calls))
	for _, e := range doc.Calls {
		entries = append(entries, CallEntry{
			Type:     e.Type,
			Date:     e.Date,
			Name:     e.Name,
			Number:   e.Number,
			Called:   e.Called,
			Duration: e.Duration,
			Count:    e.Count,
		})
	}
	return entries, nil
}

// GetPhonebookList returns the indices of all available phonebooks.
func (c *Client) GetPhonebookList() ([]int, error) {
	res, err := c.SOAP(svcOnTel, ctlOnTel, "GetPhonebookList", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox telephony: GetPhonebookList: %w", err)
	}
	raw := strings.TrimSpace(res["NewPhonebookList"])
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		if id, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// GetPhonebookEntries retrieves entries from a Fritz!Box phonebook by index.
func (c *Client) GetPhonebookEntries(phonebookID int) ([]PhonebookEntry, error) {
	res, err := c.SOAP(svcOnTel, ctlOnTel, "GetPhonebook",
		map[string]string{"NewPhonebookID": strconv.Itoa(phonebookID)})
	if err != nil {
		return nil, fmt.Errorf("fritzbox telephony: GetPhonebook %d: %w", phonebookID, err)
	}
	pbURL, ok := res["NewPhonebookURL"]
	if !ok || pbURL == "" {
		return nil, fmt.Errorf("fritzbox telephony: GetPhonebook returned no URL")
	}
	return c.fetchPhonebookXML(pbURL)
}

// phonebookDoc is the XML schema of the Fritz!Box phonebook download.
type phonebookDoc struct {
	XMLName xml.Name       `xml:"phonebooks"`
	Books   []phonebookXML `xml:"phonebook"`
}

type phonebookXML struct {
	Name    string       `xml:"name,attr"`
	Entries []contactXML `xml:"contact"`
}

type contactXML struct {
	Person struct {
		RealName string `xml:"realName"`
	} `xml:"person"`
	Telephony struct {
		Numbers []struct {
			Value string `xml:",chardata"`
		} `xml:"number"`
	} `xml:"telephony"`
}

func (c *Client) fetchPhonebookXML(rawURL string) ([]PhonebookEntry, error) {
	if strings.HasPrefix(rawURL, "/") {
		rawURL = c.tr.baseURL + rawURL
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.tr.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch phonebook: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var doc phonebookDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse phonebook xml: %w", err)
	}

	var entries []PhonebookEntry
	for _, book := range doc.Books {
		for _, ct := range book.Entries {
			e := PhonebookEntry{Name: ct.Person.RealName}
			for _, n := range ct.Telephony.Numbers {
				e.Numbers = append(e.Numbers, n.Value)
			}
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// GetTAMList returns the list of answering machine messages.
func (c *Client) GetTAMList(tamIndex int) ([]TAMEntry, error) {
	res, err := c.SOAP(svcTAM, ctlTAM, "GetMessageList",
		map[string]string{"NewIndex": strconv.Itoa(tamIndex)})
	if err != nil {
		return nil, fmt.Errorf("fritzbox telephony: TAM%d GetMessageList: %w", tamIndex, err)
	}
	listURL, ok := res["NewURL"]
	if !ok || listURL == "" {
		return nil, fmt.Errorf("fritzbox telephony: TAM returned no URL")
	}
	return c.fetchTAMListXML(listURL)
}

// tamListDoc is the XML schema for TAM message lists.
type tamListDoc struct {
	XMLName  xml.Name    `xml:"Root"`
	Messages []tamMsgXML `xml:"Message"`
}

type tamMsgXML struct {
	Index    string `xml:"Index"`
	Date     string `xml:"Date"`
	Name     string `xml:"Name"`
	Number   string `xml:"Number"`
	Duration string `xml:"Duration"`
	Read     string `xml:"Read"`
	Path     string `xml:"Path"`
}

func (c *Client) fetchTAMListXML(rawURL string) ([]TAMEntry, error) {
	if strings.HasPrefix(rawURL, "/") {
		rawURL = c.webURL + rawURL
	}
	// Prefer SID-authenticated fetch; fall back to Digest for TR-064-hosted URLs.
	resp, err := c.sid.GetWithSID(rawURL)
	if err != nil {
		// SID not available yet – try unauthenticated/Digest
		req, rerr := http.NewRequest(http.MethodGet, rawURL, nil)
		if rerr != nil {
			return nil, rerr
		}
		resp, err = c.tr.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch tam list: %w", err)
		}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var doc tamListDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse tam list xml: %w", err)
	}

	entries := make([]TAMEntry, 0, len(doc.Messages))
	for _, m := range doc.Messages {
		idx, _ := strconv.Atoi(m.Index)
		entries = append(entries, TAMEntry{
			Index:    idx,
			Date:     m.Date,
			Name:     m.Name,
			Number:   m.Number,
			Duration: m.Duration,
			Read:     m.Read == "1",
			Path:     m.Path,
		})
	}
	return entries, nil
}

// MarkTAMMessageRead marks a TAM message as read.
// Blocked when ReadOnly is true.
func (c *Client) MarkTAMMessageRead(tamIndex, msgIndex int) error {
	if c.TelephonyReadOnly() {
		return fmt.Errorf("fritzbox telephony: TAM changes blocked (readonly mode)")
	}
	_, err := c.SOAP(svcTAM, ctlTAM, "MarkMessage", map[string]string{
		"NewIndex":        strconv.Itoa(tamIndex),
		"NewMessageIndex": strconv.Itoa(msgIndex),
		"NewMarkedAsRead": "1",
	})
	if err != nil {
		return fmt.Errorf("fritzbox telephony: TAM MarkMessage: %w", err)
	}
	return nil
}

// GetTAMMessageURL returns an HTTP URL to download a TAM message's audio file (WAV).
// It reads the path from the TAM message list XML (the Path field), which is more
// broadly supported than the GetPhoneURL SOAP action (not available on Vodafone cable models).
func (c *Client) GetTAMMessageURL(tamIndex, msgIndex int) (string, error) {
	msgs, err := c.GetTAMList(tamIndex)
	if err != nil {
		return "", fmt.Errorf("fritzbox telephony: TAM%d GetTAMMessageURL: %w", tamIndex, err)
	}
	for _, m := range msgs {
		if m.Index == msgIndex {
			if m.Path == "" {
				return "", fmt.Errorf("fritzbox telephony: TAM%d message %d has no audio path", tamIndex, msgIndex)
			}
			if strings.HasPrefix(m.Path, "/") {
				return c.webURL + m.Path, nil
			}
			return m.Path, nil
		}
	}
	return "", fmt.Errorf("fritzbox telephony: TAM%d has no message at index %d", tamIndex, msgIndex)
}

// DownloadTAMMessage downloads a TAM message's audio file and saves it to destPath.
// Uses SID authentication, which is required for Fritz!Box audio file resources.
func (c *Client) DownloadTAMMessage(tamIndex, msgIndex int, destPath string) error {
	rawURL, err := c.GetTAMMessageURL(tamIndex, msgIndex)
	if err != nil {
		return err
	}

	resp, err := c.sid.GetWithSID(rawURL)
	if err != nil {
		return fmt.Errorf("download TAM audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download TAM audio: HTTP %d (url: %s)", resp.StatusCode, rawURL)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create TAM audio file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write TAM audio file: %w", err)
	}
	return nil
}
