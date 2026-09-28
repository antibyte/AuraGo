package meshcore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"
)

type DiagnosticRequest struct {
	Identity string `json:"identity"`
	Target   string `json:"target"`
	Kind     string `json:"kind"`
}
type Diagnostic struct {
	ID          string           `json:"id"`
	Identity    string           `json:"identity"`
	Target      string           `json:"target"`
	Kind        string           `json:"kind"`
	State       string           `json:"state"`
	StartedAt   int64            `json:"started_at"`
	CompletedAt int64            `json:"completed_at,omitempty"`
	Error       string           `json:"error,omitempty"`
	Route       *byte            `json:"route,omitempty"`
	Telemetry   []TelemetryValue `json:"telemetry,omitempty"`
	Outgoing    *PathInfo        `json:"outgoing,omitempty"`
	Incoming    *PathInfo        `json:"incoming,omitempty"`
}

func (m *Manager) pruneDiagnostics() {
	for id, d := range m.diagnostics {
		if id != m.activeDiagnostic && time.Now().Unix()-d.StartedAt >= 600 {
			delete(m.diagnostics, id)
		}
	}
	if len(m.diagnostics) >= 128 {
		oldest := ""
		for id, d := range m.diagnostics {
			if id != m.activeDiagnostic && (oldest == "" || d.StartedAt < m.diagnostics[oldest].StartedAt) {
				oldest = id
			}
		}
		delete(m.diagnostics, oldest)
	}
}

func (m *Manager) Diagnostic(id string) (Diagnostic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.diagnostics[id]
	if !ok || (id != m.activeDiagnostic && time.Now().Unix()-d.StartedAt >= 600) {
		delete(m.diagnostics, id)
		return Diagnostic{}, fmt.Errorf("diagnostic_expired")
	}
	return d, nil
}

func (m *Manager) StartDiagnostic(ctx context.Context, req DiagnosticRequest) (Diagnostic, error) {
	if !ValidKey(req.Identity) || !ValidKey(req.Target) || (req.Kind != "telemetry" && req.Kind != "path") {
		return Diagnostic{}, fmt.Errorf("invalid_request")
	}
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	c, cfg, busy, closed := m.conn, m.cfg, m.activeDiagnostic != "", m.closed
	m.mu.Unlock()
	if !cfg.AllowRemoteDiagnostics {
		return Diagnostic{}, fmt.Errorf("permission_denied")
	}
	if closed || c == nil || !cfg.Enabled {
		return Diagnostic{}, fmt.Errorf("not_connected")
	}
	if busy {
		return Diagnostic{}, fmt.Errorf("busy")
	}
	select {
	case m.writeSlot <- struct{}{}:
		defer func() { <-m.writeSlot }()
	case <-ctx.Done():
		return Diagnostic{}, ctx.Err()
	}
	st, err := m.refresh(ctx, c)
	if err != nil || st.State != "connected" || req.Identity != st.IdentityKey || cfg.IdentityKey != st.IdentityKey {
		return Diagnostic{}, fmt.Errorf("binding_required")
	}
	contact, ok := uniqueContact(st, req.Target)
	if !ok || contact.Key != req.Target || contact.Key == st.IdentityKey {
		return Diagnostic{}, fmt.Errorf("invalid_target")
	}
	if st.Device == nil || st.Device.ProtocolVersion < 7 {
		return Diagnostic{}, fmt.Errorf("unsupported")
	}
	m.mu.Lock()
	m.sends = slices.DeleteFunc(m.sends, func(t time.Time) bool { return time.Since(t) >= time.Minute })
	if len(m.sends) >= 6 {
		m.mu.Unlock()
		return Diagnostic{}, fmt.Errorf("busy")
	}
	m.sends = append(m.sends, time.Now())
	if m.diagnostics == nil {
		m.diagnostics = map[string]Diagnostic{}
	}
	m.pruneDiagnostics()
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		m.mu.Unlock()
		return Diagnostic{}, err
	}
	d := Diagnostic{ID: hex.EncodeToString(id), Identity: req.Identity, Target: req.Target, Kind: req.Kind, State: "pending", StartedAt: time.Now().Unix()}
	m.diagnostics[d.ID] = d
	m.activeDiagnostic = d.ID
	m.wg.Add(1)
	m.mu.Unlock()
	go func() { defer m.wg.Done(); m.runDiagnostic(c, d) }()
	return d, nil
}

func (m *Manager) runDiagnostic(c *companion, d Diagnostic) {
	ctx, cancel := context.WithTimeout(m.root, 60*time.Second)
	defer cancel()
	pushes := make(chan []byte, 16)
	c.pushMu.Lock()
	c.pushes = pushes
	c.pushMu.Unlock()
	defer func() {
		c.pushMu.Lock()
		c.pushes = nil
		c.pushMu.Unlock()
		// Even a successful path reply can be duplicated later. It has no tag,
		// so no subsequent path request may reuse this connection session.
		if d.Kind == "path" {
			c.Close()
		}
		d.CompletedAt = time.Now().Unix()
		m.mu.Lock()
		m.diagnostics[d.ID] = d
		if m.activeDiagnostic == d.ID {
			m.activeDiagnostic = ""
		}
		m.mu.Unlock()
		m.changed(Change{})
	}()
	key, _ := hex.DecodeString(d.Target)
	cmd := append([]byte{50}, key...)
	if d.Kind == "path" {
		cmd = append([]byte{52, 0}, key...)
	} else {
		payload := []byte{3, 0, 0, 0, 0, 0, 0, 0, 0} // request all telemetry permitted by the remote node
		if _, err := rand.Read(payload[5:]); err != nil {
			d.State = "failed"
			d.Error = "operation_failed"
			return
		}
		cmd = append(cmd, payload...)
	}
	var frames [][]byte
	var err error
	select {
	case m.writeSlot <- struct{}{}:
		m.mu.Lock()
		valid := m.conn == c && m.cfg.Enabled && m.cfg.AllowRemoteDiagnostics && m.cfg.IdentityKey == d.Identity && m.status.State == "connected"
		m.mu.Unlock()
		if valid {
			frames, err = c.request(ctx, cmd, 6)
		} else {
			err = fmt.Errorf("permission_changed")
		}
		<-m.writeSlot
	case <-ctx.Done():
		err = ctx.Err()
	case <-c.done:
		err = fmt.Errorf("not_connected")
	}
	if err != nil || len(frames) == 0 || len(frames[0]) != 10 {
		d.State = "failed"
		d.Error = commandFailure(err)
		if errors.Is(err, context.DeadlineExceeded) {
			d.State, d.Error = "timeout", "no_response"
		}
		if err == nil {
			d.Error = "invalid_frame"
			c.Close()
		}
		return
	}
	tag := binary.LittleEndian.Uint32(frames[0][2:6])
	route := frames[0][1]
	d.Route = &route
	c.pushMu.Lock()
	if c.diagnosticTags == nil {
		c.diagnosticTags = map[uint32]bool{}
	}
	reused := c.diagnosticTags[tag] || len(c.diagnosticTags) >= 128
	c.diagnosticTags[tag] = true
	c.pushMu.Unlock()
	if reused {
		d.State = "failed"
		d.Error = "ambiguous_response"
		c.Close()
		return
	}
	for {
		select {
		case <-ctx.Done():
			d.State = "timeout"
			d.Error = "no_response"
			return
		case <-c.done:
			d.State = "failed"
			d.Error = "not_connected"
			return
		case b := <-pushes:
			m.mu.Lock()
			valid := m.conn == c && m.cfg.AllowRemoteDiagnostics && m.cfg.IdentityKey == d.Identity
			m.mu.Unlock()
			if !valid {
				d.State = "failed"
				d.Error = "permission_changed"
				return
			}
			if d.Kind == "telemetry" {
				if len(b) < 6 || b[0] != 0x8c || b[1] != 0 || binary.LittleEndian.Uint32(b[2:6]) != tag {
					continue
				}
				d.Telemetry, err = decodeTelemetry(b[6:])
			} else {
				if len(b) < 8 || b[0] != 0x8d || b[1] != 0 || !bytes.Equal(b[2:8], key[:6]) {
					continue
				}
				d.Outgoing, d.Incoming, err = decodeDiscovery(b[8:])
			}
			if err != nil {
				d.State = "failed"
				d.Error = err.Error()
			} else {
				d.State = "completed"
			}
			return
		}
	}
}

func decodeDiscovery(b []byte) (*PathInfo, *PathInfo, error) {
	var paths []*PathInfo
	for i := 0; i < 2; i++ {
		if len(b) == 0 || b[0] == 0xff {
			return nil, nil, fmt.Errorf("invalid_frame")
		}
		n := int(b[0]&63) * (int(b[0]>>6) + 1)
		if n > 64 || len(b) < 1+n {
			return nil, nil, fmt.Errorf("invalid_frame")
		}
		p, err := decodePath(b[0], b[1:1+n])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid_frame")
		}
		paths = append(paths, &p)
		b = b[1+n:]
	}
	if len(b) != 0 {
		return nil, nil, fmt.Errorf("invalid_frame")
	}
	return paths[0], paths[1], nil
}
