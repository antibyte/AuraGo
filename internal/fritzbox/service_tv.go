// Package fritzbox – TV / multimedia service calls (Fritz!Box Cable models only).
// Covers: DVB-C channel list and stream URL retrieval.
// Only relevant for Fritz!Box models with built-in cable tuner (e.g., Fritz!Box 6660 Cable).
package fritzbox

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// TVChannel represents a DVB-C TV or radio channel.
type TVChannel struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Program   string `json:"program"`
	IsHD      bool   `json:"is_hd"`
	IsRadio   bool   `json:"is_radio"`
	StreamURL string `json:"stream_url"` // rtsp:// or http:// stream URL (may be empty if TV is disabled)
}

// GetTVChannelList retrieves the DVB-C channel list.
func (c *Client) GetTVChannelList() ([]TVChannel, error) {
	res, err := c.SOAP(svcMedia, ctlMedia, "X_AVM-DE_GetChannelListURL", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox tv: GetChannelListURL: %w", err)
	}
	listURL, ok := res["NewX_AVM-DE_ChannelListURL"]
	if !ok || listURL == "" {
		return nil, fmt.Errorf("fritzbox tv: GetChannelListURL returned no URL")
	}
	return c.fetchChannelListXML(listURL)
}

// channelListDoc is the XML schema for the Fritz!Box DVB-C channel list.
type channelListDoc struct {
	XMLName  xml.Name     `xml:"ChannelList"`
	Channels []channelXML `xml:"Channel"`
}

type channelXML struct {
	Index     string `xml:"Index"`
	Name      string `xml:"Name"`
	Program   string `xml:"Program"`
	IsHD      string `xml:"IsHD"`
	IsRadio   string `xml:"IsRadio"`
	StreamURL string `xml:"StreamURL"`
}

func (c *Client) fetchChannelListXML(rawURL string) ([]TVChannel, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.tr.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch channel list: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var doc channelListDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse channel list xml: %w", err)
	}

	channels := make([]TVChannel, 0, len(doc.Channels))
	for _, ch := range doc.Channels {
		idx, _ := strconv.Atoi(ch.Index)
		channels = append(channels, TVChannel{
			Index:     idx,
			Name:      ch.Name,
			Program:   ch.Program,
			IsHD:      ch.IsHD == "1",
			IsRadio:   ch.IsRadio == "1",
			StreamURL: ch.StreamURL,
		})
	}
	return channels, nil
}
