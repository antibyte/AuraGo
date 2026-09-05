package meshcore

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const publicChannelKey = "8b3387e9c5cdea6ac9e5edbaa115cd72"

func (m *Manager) scrubStatus(st *Status) {
	st.Name = m.scrub(st.Name)
	st.Firmware = m.scrub(st.Firmware)
	for i := range st.Contacts {
		st.Contacts[i].Name = m.scrub(st.Contacts[i].Name)
	}
	for i := range st.Channels {
		st.Channels[i].Name = m.scrub(st.Channels[i].Name)
	}
}

func channelKind(name string, key []byte) string {
	if hex.EncodeToString(key) == publicChannelKey {
		return "public"
	}
	h := sha256.Sum256([]byte(name))
	if strings.HasPrefix(name, "#") && bytes.Equal(key, h[:16]) {
		return "hashtag"
	}
	return "private"
}

type EditRequest struct {
	Action       string `json:"action"`
	Identity     string `json:"identity"`
	Conversation string `json:"conversation"`
	Name         string `json:"name"`
	Key          string `json:"key"`
	Type         byte   `json:"type"`
	Kind         string `json:"kind"`
	Secret       string `json:"secret"`
	Invitation   string `json:"invitation"`
	Flood        bool   `json:"flood"`
}

func validName(name string) bool {
	return name != "" && len(name) <= 31 && utf8.ValidString(name) && !strings.ContainsAny(name, "\x00\r\n")
}

func parseInvitation(req *EditRequest) error {
	if req.Invitation == "" {
		return nil
	}
	if len(req.Invitation) > 2048 {
		return fmt.Errorf("invalid_invitation")
	}
	u, err := url.Parse(req.Invitation)
	if err != nil || u.Scheme != "meshcore" || u.Path != "/add" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("invalid_invitation")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return fmt.Errorf("invalid_invitation")
	}
	allowed := map[string]bool{"name": true}
	if u.Host == "contact" && req.Action == "contact_add" {
		allowed["public_key"] = true
		allowed["type"] = true
		req.Key = q.Get("public_key")
		n, e := strconv.Atoi(q.Get("type"))
		if e != nil || n < 1 || n > 4 {
			return fmt.Errorf("invalid_invitation")
		}
		req.Type = byte(n)
	} else if u.Host == "channel" && req.Action == "channel_add" {
		allowed["secret"] = true
		req.Secret = q.Get("secret")
		if len(req.Secret) != 32 {
			return fmt.Errorf("invalid_invitation")
		}
		req.Kind = "private"
	} else {
		return fmt.Errorf("invalid_invitation")
	}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 {
			return fmt.Errorf("unsupported_invitation")
		}
	}
	req.Name = q.Get("name")
	req.Invitation = ""
	return nil
}

// Invitation only exposes public contact data, except for the explicit channel
// share action. Callers must keep its result out of caches, logs, and tools.
func (m *Manager) Invitation(ctx context.Context, identity, id string) (string, error) {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	c := m.conn
	m.mu.Unlock()
	if c == nil {
		return "", fmt.Errorf("not_connected")
	}
	st, err := m.refresh(ctx, c)
	if err != nil || st.State != "connected" || identity != st.IdentityKey {
		return "", fmt.Errorf("binding_required")
	}
	q := url.Values{}
	if id == "self" {
		q.Set("name", st.Name)
		q.Set("public_key", st.IdentityKey)
		q.Set("type", "1")
		return "meshcore://contact/add?" + q.Encode(), nil
	}
	conv, err := m.store.conversation(id)
	if err != nil || conv.IdentityKey != identity {
		return "", fmt.Errorf("invalid_target")
	}
	if conv.Kind == "direct" {
		for _, contact := range st.Contacts {
			if contact.Key == conv.Target {
				q.Set("name", contact.Name)
				q.Set("public_key", contact.Key)
				q.Set("type", strconv.Itoa(int(contact.Type)))
				return "meshcore://contact/add?" + q.Encode(), nil
			}
		}
		return "", fmt.Errorf("invalid_target")
	}
	if conv.Kind != "channel" || !channelMatches(st, ChannelRule{Index: conv.Channel, Binding: conv.Target}) {
		return "", fmt.Errorf("invalid_target")
	}
	frames, err := c.request(ctx, []byte{31, byte(conv.Channel)}, 18)
	if err != nil {
		return "", fmt.Errorf("device_error")
	}
	b := frames[0]
	defer clear(b)
	if len(b) != 50 || int(b[1]) != conv.Channel {
		return "", fmt.Errorf("device_error")
	}
	// Recompute the exact binding before releasing the secret; slots can change.
	if channelBinding(st.IdentityKey, b, m.store.salt) != conv.Target {
		return "", fmt.Errorf("binding_required")
	}
	q.Set("name", wireText(b[2:34]))
	q.Set("secret", hex.EncodeToString(b[34:50]))
	return "meshcore://channel/add?" + q.Encode(), nil
}

// Edit serializes device edits with configuration publication. The server holds
// its existing CfgSaveMu while calling it. Failed edits leave a durable lock.
func (m *Manager) Edit(ctx context.Context, req EditRequest, publish func(Config) error) error {
	if err := parseInvitation(&req); err != nil {
		return err
	}
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	c, cfg := m.conn, m.cfg
	m.mu.Unlock()
	if c == nil || !cfg.Enabled {
		return fmt.Errorf("not_connected")
	}
	st, err := m.refresh(ctx, c)
	if err != nil || req.Identity != st.IdentityKey || cfg.IdentityKey != st.IdentityKey {
		return fmt.Errorf("binding_required")
	}
	if st.State != "connected" && req.Action != "confirm_mapping" {
		return fmt.Errorf("binding_required")
	}
	cfg.TrustedNodes = slices.Clone(cfg.TrustedNodes)
	cfg.SendNodes = slices.Clone(cfg.SendNodes)
	cfg.Channels = slices.Clone(cfg.Channels)
	var command []byte
	slot := -1
	switch req.Action {
	case "advert":
		m.mu.Lock()
		m.sends = slices.DeleteFunc(m.sends, func(t time.Time) bool { return time.Since(t) >= time.Minute })
		available := len(m.sends) < 6
		if available {
			m.sends = append(m.sends, time.Now())
		}
		m.mu.Unlock()
		if !available {
			return fmt.Errorf("busy")
		}
		param := byte(0)
		if req.Flood {
			param = 1
		}
		_, err := c.request(ctx, []byte{7, param}, 0)
		if err != nil {
			return fmt.Errorf("outcome_unknown")
		}
		return nil
	case "contact_add":
		req.Key = strings.ToLower(strings.TrimSpace(req.Key))
		req.Name = strings.TrimSpace(req.Name)
		if !ValidKey(req.Key) || !validName(req.Name) || req.Type < 1 || req.Type > 4 || req.Key == st.IdentityKey {
			return fmt.Errorf("invalid_contact")
		}
		for _, contact := range st.Contacts {
			if contact.Key == req.Key {
				return fmt.Errorf("contact_exists")
			}
		}
		command = make([]byte, 148)
		command[0] = 9
		key, _ := hex.DecodeString(req.Key)
		copy(command[1:33], key)
		command[33] = req.Type
		command[35] = 0xff
		copy(command[100:132], req.Name)
		// An imported contact never inherits stale agent authorizations.
		cfg.TrustedNodes = slices.DeleteFunc(cfg.TrustedNodes, func(k string) bool { return k == req.Key })
		cfg.SendNodes = slices.DeleteFunc(cfg.SendNodes, func(k string) bool { return k == req.Key })
	case "contact_remove", "channel_remove":
		conv, e := m.store.conversation(req.Conversation)
		if e != nil || conv.IdentityKey != st.IdentityKey {
			return fmt.Errorf("invalid_target")
		}
		if req.Action == "contact_remove" {
			if conv.Kind != "direct" || !ValidKey(conv.Target) {
				return fmt.Errorf("invalid_target")
			}
			req.Key = conv.Target
			key, _ := hex.DecodeString(conv.Target)
			command = append([]byte{15}, key...)
			cfg.TrustedNodes = slices.DeleteFunc(cfg.TrustedNodes, func(k string) bool { return k == conv.Target })
			cfg.SendNodes = slices.DeleteFunc(cfg.SendNodes, func(k string) bool { return k == conv.Target })
		} else {
			if conv.Kind != "channel" || !channelMatches(st, ChannelRule{Index: conv.Channel, Binding: conv.Target}) {
				return fmt.Errorf("binding_required")
			}
			slot = conv.Channel
			command = make([]byte, 50)
			command[0] = 32
			command[1] = byte(slot)
		}
	case "channel_add":
		req.Name = strings.TrimSpace(req.Name)
		if !validName(req.Name) {
			return fmt.Errorf("invalid_channel")
		}
		var key []byte
		switch req.Kind {
		case "public":
			req.Name = "Public"
			key, _ = hex.DecodeString(publicChannelKey)
		case "hashtag":
			if !strings.HasPrefix(req.Name, "#") {
				req.Name = "#" + req.Name
			}
			if !validName(req.Name) {
				return fmt.Errorf("invalid_channel")
			}
			h := sha256.Sum256([]byte(req.Name))
			key = h[:16]
		case "private":
			if req.Secret == "" {
				key = make([]byte, 16)
				if _, err = rand.Read(key); err != nil {
					return err
				}
			} else {
				key, err = hex.DecodeString(req.Secret)
				if err != nil || len(key) != 16 {
					return fmt.Errorf("invalid_channel")
				}
			}
		default:
			return fmt.Errorf("invalid_channel")
		}
		defer clear(key)
		for i := 0; i < st.ChannelCapacity; i++ {
			used := false
			for _, ch := range st.Channels {
				if ch.Index == i {
					used = true
					break
				}
			}
			if !used {
				slot = i
				break
			}
		}
		if slot < 0 {
			return fmt.Errorf("channels_full")
		}
		command = make([]byte, 50)
		command[0] = 32
		command[1] = byte(slot)
		copy(command[2:34], req.Name)
		copy(command[34:50], key)
	case "confirm_mapping":
		// Explicit recovery never restores previous automation on changed slots.
		cfg.Channels = nil
		for _, ch := range st.Channels {
			cfg.Channels = append(cfg.Channels, ChannelRule{Index: ch.Index, Binding: ch.Binding, Mode: "receive", Prefix: "!aura"})
		}
	default:
		return fmt.Errorf("invalid_request")
	}
	defer clear(command)
	select {
	case m.writeSlot <- struct{}{}:
		defer func() { <-m.writeSlot }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if _, err = m.store.db.Exec("INSERT INTO meshcore_meta(key,value) VALUES('mutation_pending','1'),('input_binding_uncertain','1') ON CONFLICT(key) DO UPDATE SET value='1'"); err != nil {
		return err
	}
	m.mu.Lock()
	m.editing = true
	m.status.State = "updating"
	if m.runCancel != nil {
		m.runCancel()
	}
	m.mu.Unlock()
	m.changed(Change{})
	success := false
	defer func() {
		m.mu.Lock()
		m.editing = false
		if !success {
			m.status.State = "binding_changed"
		}
		m.mu.Unlock()
		m.changed(Change{})
	}()
	if len(command) > 0 {
		if _, err = c.request(ctx, command, 0); err != nil {
			return fmt.Errorf("outcome_unknown")
		}
	}
	next, err := c.snapshot(ctx, m.store.salt)
	if err != nil || next.IdentityKey != st.IdentityKey {
		return fmt.Errorf("outcome_unknown")
	}
	if slot >= 0 {
		cfg.Channels = slices.DeleteFunc(cfg.Channels, func(r ChannelRule) bool { return r.Index == slot })
		if req.Action == "channel_add" {
			found := false
			for _, ch := range next.Channels {
				if ch.Index == slot && ch.Name == req.Name && ch.Binding == channelBinding(st.IdentityKey, command, m.store.salt) {
					found = true
					cfg.Channels = append(cfg.Channels, ChannelRule{Index: slot, Binding: ch.Binding, Mode: "receive", Prefix: "!aura"})
				}
			}
			if !found {
				return fmt.Errorf("outcome_unknown")
			}
		}
		if req.Action == "channel_remove" {
			for _, ch := range next.Channels {
				if ch.Index == slot {
					return fmt.Errorf("outcome_unknown")
				}
			}
		}
	}
	if req.Action == "contact_add" || req.Action == "contact_remove" {
		found := false
		for _, contact := range next.Contacts {
			if contact.Key == req.Key {
				found = true
				if req.Action == "contact_add" && (contact.Name != req.Name || contact.Type != req.Type) {
					return fmt.Errorf("outcome_unknown")
				}
			}
		}
		if found != (req.Action == "contact_add") {
			return fmt.Errorf("outcome_unknown")
		}
	}
	if err = cfg.Normalize(); err != nil {
		return fmt.Errorf("invalid_config")
	}
	if publish == nil {
		return fmt.Errorf("config_unavailable")
	}
	if err = publish(cfg); err != nil {
		return fmt.Errorf("config_unavailable")
	}
	m.scrubStatus(&next)
	if err = m.store.syncConversations(next); err != nil {
		return err
	}
	if _, err = m.store.db.Exec("UPDATE meshcore_meta SET value='0' WHERE key='mutation_pending'"); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = cfg
	m.status = next
	m.mu.Unlock()
	success = true
	return nil
}
