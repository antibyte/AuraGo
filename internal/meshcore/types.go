// Package meshcore implements the bounded MeshCore Companion text protocol.
// Wire reference: meshcore-dev/MeshCore@0679dbeffc504d562d2f09eb072fdc223f8ffc2a.
package meshcore

import (
	"context"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const ProtocolRevision = "0679dbeffc504d562d2f09eb072fdc223f8ffc2a"

type Config struct {
	Enabled                bool          `yaml:"enabled" json:"enabled"`
	Transport              string        `yaml:"transport" json:"transport"`
	Port                   string        `yaml:"port" json:"port"`
	Address                string        `yaml:"address" json:"address"`
	IdentityKey            string        `yaml:"identity_key" json:"identity_key"`
	TrustedNodes           []string      `yaml:"trusted_nodes" json:"trusted_nodes"`
	DirectReplies          bool          `yaml:"direct_replies" json:"direct_replies"`
	ProactiveSend          bool          `yaml:"proactive_send" json:"proactive_send"`
	SendNodes              []string      `yaml:"send_nodes" json:"send_nodes"`
	Channels               []ChannelRule `yaml:"channels" json:"channels"`
	MaxCommandAgeSeconds   int           `yaml:"max_command_age_seconds" json:"max_command_age_seconds"`
	FutureToleranceSeconds int           `yaml:"future_tolerance_seconds" json:"future_tolerance_seconds"`
	RetentionDays          int           `yaml:"retention_days" json:"retention_days"`
	MaxMessages            int           `yaml:"max_messages" json:"max_messages"`
	PeerRunsPerMinute      int           `yaml:"peer_runs_per_minute" json:"peer_runs_per_minute"`
	RunsPerMinute          int           `yaml:"runs_per_minute" json:"runs_per_minute"`
}

type ChannelRule struct {
	Index     int    `yaml:"index" json:"index"`
	Binding   string `yaml:"binding" json:"binding"`
	Mode      string `yaml:"mode" json:"mode"` // receive, prefix, questions
	Prefix    string `yaml:"prefix" json:"prefix"`
	AllowSend bool   `yaml:"allow_send" json:"allow_send"`
}

func (c *Config) Normalize() error {
	if c.Transport == "" {
		c.Transport = "usb"
	}
	if c.Transport != "usb" && c.Transport != "ble" {
		return fmt.Errorf("meshcore: invalid transport")
	}
	c.Port = strings.TrimSpace(c.Port)
	c.Address = strings.ToUpper(strings.TrimSpace(c.Address))
	if c.Enabled && ((c.Transport == "usb" && c.Port == "") || (c.Transport == "ble" && !addressPattern.MatchString(c.Address))) {
		return fmt.Errorf("meshcore: select a serial port or Bluetooth address")
	}
	if strings.ContainsAny(c.Port, "\x00\r\n") {
		return fmt.Errorf("meshcore: invalid serial port")
	}
	c.IdentityKey = strings.ToLower(strings.TrimSpace(c.IdentityKey))
	if c.IdentityKey != "" && !ValidKey(c.IdentityKey) {
		return fmt.Errorf("meshcore: invalid device public key")
	}
	for _, list := range [][]string{c.TrustedNodes, c.SendNodes} {
		seen := map[string]bool{}
		for i, key := range list {
			key = strings.ToLower(strings.TrimSpace(key))
			list[i] = key
			if !ValidKey(key) || seen[key] {
				return fmt.Errorf("meshcore: node keys must be unique full public keys")
			}
			seen[key] = true
		}
	}
	seen := map[int]bool{}
	for i := range c.Channels {
		r := &c.Channels[i]
		if r.Mode == "" {
			r.Mode = "receive"
		}
		if r.Prefix == "" {
			r.Prefix = "!aura"
		}
		if r.Index < 0 || r.Index > 63 || seen[r.Index] {
			return fmt.Errorf("meshcore: invalid or duplicate channel slot")
		}
		seen[r.Index] = true
		if r.Mode != "receive" && r.Mode != "prefix" && r.Mode != "questions" {
			return fmt.Errorf("meshcore: invalid channel mode")
		}
		if len(r.Prefix) > 32 || strings.ContainsAny(r.Prefix, "\r\n\x00") {
			return fmt.Errorf("meshcore: invalid reply prefix")
		}
		if (r.Mode != "receive" || r.AllowSend) && (!ValidKey(r.Binding) || c.IdentityKey == "") {
			return fmt.Errorf("meshcore: confirm device and channel binding before enabling replies or sending")
		}
	}
	for _, v := range []struct {
		p        *int
		def, max int
	}{
		{&c.MaxCommandAgeSeconds, 600, 86400}, {&c.FutureToleranceSeconds, 120, 3600},
		{&c.RetentionDays, 7, 90}, {&c.MaxMessages, 1000, 10000}, {&c.PeerRunsPerMinute, 2, 60}, {&c.RunsPerMinute, 12, 120},
	} {
		if *v.p == 0 {
			*v.p = v.def
		}
		if *v.p < 1 || *v.p > v.max {
			return fmt.Errorf("meshcore: limit outside supported range")
		}
	}
	return nil
}

var addressPattern = regexp.MustCompile(`^[0-9A-F]{2}(:[0-9A-F]{2}){5}$`)

func ValidKey(key string) bool {
	b, err := hex.DecodeString(key)
	return err == nil && len(b) == 32 && key != strings.Repeat("0", 64)
}

type Contact struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type byte   `json:"type"`
}
type Channel struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Binding string `json:"binding"`
}
type Status struct {
	nameBytes        int
	State            string    `json:"state"`
	IdentityKey      string    `json:"identity_key"`
	Name             string    `json:"name"`
	Firmware         string    `json:"firmware"`
	Contacts         []Contact `json:"contacts"`
	Channels         []Channel `json:"channels"`
	ErrorCode        string    `json:"error_code,omitempty"`
	HardwareVerified bool      `json:"hardware_verified"`
}
type Message struct {
	Direction   string `json:"direction"`
	ID          string `json:"id"`
	IdentityKey string `json:"identity_key"`
	Kind        string `json:"kind"`
	Sender      string `json:"sender"`
	Channel     int    `json:"channel"`
	Binding     string `json:"-"`
	TextType    byte   `json:"text_type"`
	Timestamp   int64  `json:"timestamp"`
	ReceivedAt  int64  `json:"received_at"`
	Text        string `json:"text"`
	State       string `json:"state"`
	Review      string `json:"review"`
	Reason      string `json:"reason"`
	Reply       string `json:"reply,omitempty"`
	SendState   string `json:"send_state,omitempty"`
}
type Review struct {
	Decision string
	Reason   string
}
type Hooks struct {
	Scan   func(context.Context, Message) Review
	Run    func(context.Context, Message, string) (string, error)
	Notify func(Message) error
	Issue  func(string, bool)
	Scrub  func(string) string
}

// Fresh uses radio time only for the bounded command admission window.
func Fresh(m Message, c Config, now time.Time) bool {
	age := now.Unix() - m.Timestamp
	return age <= int64(c.MaxCommandAgeSeconds) && age >= -int64(c.FutureToleranceSeconds)
}
