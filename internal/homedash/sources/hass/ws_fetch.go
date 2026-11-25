package hass

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"aurago/internal/security"
)

// nopHandler ignores websocket callbacks (one-shot fetches use none).
type nopHandler struct{}

func (nopHandler) OnStates([]HAState)          {}
func (nopHandler) OnStateChanged(StateChanged) {}
func (nopHandler) OnRegistries(Registries)     {}
func (nopHandler) OnStatus(WSStatus)           {}

// Fetch opens one websocket session, reads get_states and the three registry lists, and closes the
// session. It is the adapter's catalog read while no run is live (Home Assistant has no REST registry
// API). A failed registry list leaves that list empty; a failed get_states, a rejected token
// (errAuthInvalid) or the end of ctx is an error. The token only travels in the auth message.
func (c *WSClient) Fetch(ctx context.Context) ([]HAState, Registries, error) {
	if c.token != "" {
		security.RegisterSensitive(c.token)
	}
	dctx, cancel := context.WithTimeout(ctx, c.o.HandshakeTimeout)
	conn, resp, err := c.dialer.DialContext(dctx, c.url, nil)
	cancel()
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, Registries{}, errors.Join(errDial, err)
	}
	defer conn.Close()
	conn.SetReadLimit(c.o.ReadLimit)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	s := &wsSession{c: c, conn: conn, pending: map[int64]string{}, pendingBatch: map[int64]int64{}}
	if err := s.authenticate(); err != nil {
		return nil, Registries{}, err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetReadDeadline(dl)
	}
	want := map[int64]string{1: kindStates, 2: kindAreas, 3: kindDevices, 4: kindEntities}
	for id := int64(1); id <= 4; id++ {
		typ := "get_states"
		if k := want[id]; k != kindStates {
			typ = registryCommand[k]
		}
		if err := s.write(map[string]any{"id": id, "type": typ}); err != nil {
			return nil, Registries{}, err
		}
	}
	var states []HAState
	var reg Registries
	for len(want) > 0 {
		var m wsMsg
		if err := conn.ReadJSON(&m); err != nil {
			if ctx.Err() != nil {
				return nil, Registries{}, ctx.Err()
			}
			return nil, Registries{}, err
		}
		kind, ok := want[m.ID]
		if m.Type != "result" || !ok {
			continue
		}
		delete(want, m.ID)
		success := m.Success != nil && *m.Success
		switch kind {
		case kindStates:
			if !success {
				return nil, Registries{}, errProtocol
			}
			if err := json.Unmarshal(m.Result, &states); err != nil {
				return nil, Registries{}, fmt.Errorf("hass: decode states: %w", err)
			}
		case kindAreas:
			if success {
				_ = json.Unmarshal(m.Result, &reg.Areas)
			}
		case kindDevices:
			if success {
				_ = json.Unmarshal(m.Result, &reg.Devices)
			}
		case kindEntities:
			if success {
				_ = json.Unmarshal(m.Result, &reg.Entities)
			}
		}
	}
	return states, reg, nil
}
