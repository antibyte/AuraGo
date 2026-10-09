package server

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/retronet"

	"github.com/gorilla/websocket"
)

const (
	retroNetReadLimit    = 64 << 10 // largest accepted browser frame
	retroNetWriteTimeout = 10 * time.Second
	retroNetEventQueue   = 16

	retroNetMinCols, retroNetMaxCols, retroNetDefaultCols = 20, 400, 80
	retroNetMinRows, retroNetMaxRows, retroNetDefaultRows = 5, 200, 25
)

// retroNetUpgrader uses the strict same-host Origin check (an empty Origin is
// refused) rather than the lenient Desktop event-socket upgrader.
var retroNetUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     desktop.SameHostWebSocketOrigin,
}

// retroNetWSClient adapts one browser WebSocket to retronet.Client: binary
// frames are keystrokes, text frames are JSON controls. One reader goroutine
// feeds Events and closes it when the browser disconnects or a read fails; a
// hijacked connection keeps its request context until the handler returns, so
// that close is how Manager.Run learns the browser is gone. Writes are
// synchronous, serialized and bounded by a deadline, so a stalled browser
// cannot block the pump forever and no payload is retained after a send.
type retroNetWSClient struct {
	conn      *websocket.Conn
	events    chan retronet.ClientEvent
	done      chan struct{}
	closeOnce sync.Once
	writeMu   sync.Mutex
}

func newRetroNetWSClient(conn *websocket.Conn) *retroNetWSClient {
	client := &retroNetWSClient{
		conn:   conn,
		events: make(chan retronet.ClientEvent, retroNetEventQueue),
		done:   make(chan struct{}),
	}
	conn.SetReadLimit(retroNetReadLimit)
	go client.readLoop()
	return client
}

// Events is closed when the browser disconnects or the socket is closed.
func (c *retroNetWSClient) Events() <-chan retronet.ClientEvent { return c.events }

// SendData writes UTF-8 terminal output as one binary frame.
func (c *retroNetWSClient) SendData(p []byte) error { return c.write(websocket.BinaryMessage, p) }

// SendControl writes one JSON control frame.
func (c *retroNetWSClient) SendControl(control retronet.Control) error {
	payload, err := json.Marshal(control)
	if err != nil {
		return err
	}
	return c.write(websocket.TextMessage, payload)
}

func (c *retroNetWSClient) write(messageType int, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(retroNetWriteTimeout)); err != nil {
		return err
	}
	return c.conn.WriteMessage(messageType, payload)
}

func (c *retroNetWSClient) readLoop() {
	defer close(c.events)
	for {
		messageType, payload, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var event retronet.ClientEvent
		switch messageType {
		case websocket.BinaryMessage:
			if len(payload) == 0 {
				continue
			}
			event = retronet.ClientEvent{Data: payload}
		case websocket.TextMessage:
			decoded, ok := decodeRetroNetClientMessage(payload)
			if !ok {
				continue
			}
			event = decoded
		default:
			continue
		}
		select {
		case c.events <- event:
		case <-c.done:
			return
		}
	}
}

// closeSocket sends a normal close frame (best effort) and closes the socket.
// It is idempotent and unblocks the reader. WriteControl and Close may run
// concurrently with a data write, so it does not wait for writeMu: a stalled
// write cannot hold the close beyond its own one-second deadline.
func (c *retroNetWSClient) closeSocket() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		_ = c.conn.Close()
	})
}

// decodeRetroNetClientMessage accepts only the two browser controls of the
// wire contract; unknown types, malformed JSON and incomplete decisions are
// ignored. The browser never sends a host or port.
func decodeRetroNetClientMessage(payload []byte) (retronet.ClientEvent, bool) {
	var message struct {
		Type   string `json:"type"`
		Cols   int    `json:"cols"`
		Rows   int    `json:"rows"`
		Accept *bool  `json:"accept"`
	}
	if err := json.Unmarshal(payload, &message); err != nil {
		return retronet.ClientEvent{}, false
	}
	switch message.Type {
	case "resize":
		size := retronet.Size{
			Cols: clampRetroNetDimension(message.Cols, retroNetMinCols, retroNetMaxCols, retroNetDefaultCols),
			Rows: clampRetroNetDimension(message.Rows, retroNetMinRows, retroNetMaxRows, retroNetDefaultRows),
		}
		return retronet.ClientEvent{Resize: &size}, true
	case "hostkey_decision":
		if message.Accept == nil {
			return retronet.ClientEvent{}, false
		}
		accept := *message.Accept
		return retronet.ClientEvent{HostKeyAccept: &accept}, true
	}
	return retronet.ClientEvent{}, false
}

func retroNetSizeFromQuery(query url.Values) retronet.Size {
	return retronet.Size{
		Cols: clampRetroNetDimension(parseRetroNetDimension(query.Get("cols")), retroNetMinCols, retroNetMaxCols, retroNetDefaultCols),
		Rows: clampRetroNetDimension(parseRetroNetDimension(query.Get("rows")), retroNetMinRows, retroNetMaxRows, retroNetDefaultRows),
	}
}

func parseRetroNetDimension(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

// clampRetroNetDimension maps missing or non-positive values to the default
// and clamps everything else into [low, high].
func clampRetroNetDimension(value, low, high, fallback int) int {
	switch {
	case value <= 0:
		return fallback
	case value < low:
		return low
	case value > high:
		return high
	}
	return value
}
