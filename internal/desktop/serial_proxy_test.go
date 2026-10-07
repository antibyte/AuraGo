package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/serialutil"

	"github.com/gorilla/websocket"
	"go.bug.st/serial"
)

type serialProxyRead struct {
	data []byte
	err  error
}

type serialProxyFakePort struct {
	reads        chan serialProxyRead
	writes       chan []byte
	ops          chan string
	breaks       chan time.Duration
	closed       chan struct{}
	writeStarted chan struct{}
	writeGateMu  sync.RWMutex
	writeGate    <-chan struct{}
	closeMu      sync.Once
}

func newSerialProxyFakePort() *serialProxyFakePort {
	return &serialProxyFakePort{
		reads: make(chan serialProxyRead, 4), writes: make(chan []byte, 8),
		ops: make(chan string, 8), breaks: make(chan time.Duration, 2), closed: make(chan struct{}),
		writeStarted: make(chan struct{}, 1),
	}
}

func (p *serialProxyFakePort) SetMode(*serial.Mode) error { return nil }
func (p *serialProxyFakePort) Drain() error               { return nil }
func (p *serialProxyFakePort) ResetInputBuffer() error    { return nil }
func (p *serialProxyFakePort) ResetOutputBuffer() error   { return nil }
func (p *serialProxyFakePort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}
func (p *serialProxyFakePort) SetReadTimeout(time.Duration) error { return nil }
func (p *serialProxyFakePort) SetDTR(value bool) error {
	p.ops <- "dtr:" + boolString(value)
	return nil
}
func (p *serialProxyFakePort) SetRTS(value bool) error {
	p.ops <- "rts:" + boolString(value)
	return nil
}
func (p *serialProxyFakePort) Break(duration time.Duration) error { p.breaks <- duration; return nil }
func (p *serialProxyFakePort) Close() error {
	p.closeMu.Do(func() { close(p.closed) })
	return nil
}
func (p *serialProxyFakePort) Read(dst []byte) (int, error) {
	select {
	case next := <-p.reads:
		return copy(dst, next.data), next.err
	case <-p.closed:
		return 0, errors.New("closed")
	}
}
func (p *serialProxyFakePort) Write(data []byte) (int, error) {
	p.writeGateMu.RLock()
	gate := p.writeGate
	p.writeGateMu.RUnlock()
	if gate != nil {
		select {
		case p.writeStarted <- struct{}{}:
		default:
		}
		select {
		case <-gate:
		case <-p.closed:
			return 0, errors.New("closed")
		}
	}
	p.writes <- append([]byte(nil), data...)
	return len(data), nil
}

func (p *serialProxyFakePort) setWriteGate(gate <-chan struct{}) {
	p.writeGateMu.Lock()
	p.writeGate = gate
	p.writeGateMu.Unlock()
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func startSerialProxyTest(t *testing.T, options RemoteProxyOptions, authorize func(*http.Request) bool) (*websocket.Conn, *serialProxyFakePort) {
	t.Helper()
	port := newSerialProxyFakePort()
	handler := handleSerialProxy(options, authorize, func(_ context.Context, name, owner string, mode *serial.Mode) (serial.Port, error) {
		if name != "COM1" || owner != serialutil.OwnerQuickConnectHost || mode == nil || mode.BaudRate != 115200 {
			return nil, errors.New("unexpected serial open arguments")
		}
		return port, nil
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	endpoint.Scheme = "ws"
	endpoint.Path = "/serial"
	header := http.Header{"Origin": []string{server.URL}}
	conn, _, err := websocket.DefaultDialer.Dial(endpoint.String(), header)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	open := `{"type":"open","port":"COM1","options":{"baud_rate":115200,"data_bits":8,"stop_bits":1,"parity":"none","flow_control":"none","dtr":false,"rts":false}}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(open)); err != nil {
		t.Fatalf("send open: %v", err)
	}
	if status := readSerialProxyStatus(t, conn); status.Type != "connected" || status.Code != "connected" {
		t.Fatalf("open status = %+v", status)
	}
	return conn, port
}

func readSerialProxyStatus(t *testing.T, conn *websocket.Conn) serialStatus {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	kind, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if kind != websocket.TextMessage {
		t.Fatalf("status message type = %d, want text", kind)
	}
	var status serialStatus
	if err := json.Unmarshal(payload, &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	return status
}

func waitSerialProxyValue[T any](t *testing.T, values <-chan T) T {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(2 * time.Second):
		var zero T
		t.Fatal("timed out waiting for serial proxy operation")
		return zero
	}
}

func TestSerialProxyRawBytesControlsAndClose(t *testing.T) {
	conn, port := startSerialProxyTest(t, RemoteProxyOptions{MaxSessionDuration: 5 * time.Second, IdleTimeout: 3 * time.Second}, nil)
	for _, want := range [][]byte{{0x00, 0xff, 0x12, 0x7f}, {0x43, 0x00, 0x44}} {
		if err := conn.WriteMessage(websocket.BinaryMessage, want); err != nil {
			t.Fatalf("send binary: %v", err)
		}
		if got := waitSerialProxyValue(t, port.writes); !bytes.Equal(got, want) {
			t.Fatalf("serial write = %v, want %v", got, want)
		}
	}
	// Empty binary messages are valid no-ops, not malformed text controls.
	if err := conn.WriteMessage(websocket.BinaryMessage, nil); err != nil {
		t.Fatalf("send empty binary: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"signals","dtr":true,"rts":false}`)); err != nil {
		t.Fatalf("send signals: %v", err)
	}
	if got := waitSerialProxyValue(t, port.ops); got != "dtr:true" {
		t.Fatalf("first signal = %q", got)
	}
	if got := waitSerialProxyValue(t, port.ops); got != "rts:false" {
		t.Fatalf("second signal = %q", got)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"break"}`)); err != nil {
		t.Fatalf("send break: %v", err)
	}
	if got := waitSerialProxyValue(t, port.breaks); got != 250*time.Millisecond {
		t.Fatalf("break duration = %s", got)
	}

	rx := []byte{0x00, 0xfe, 0x81, 0x0a}
	port.reads <- serialProxyRead{data: rx}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	kind, got, err := conn.ReadMessage()
	if err != nil || kind != websocket.BinaryMessage || !bytes.Equal(got, rx) {
		t.Fatalf("serial receive = type %d bytes %v err %v", kind, got, err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"disconnect"}`)); err != nil {
		t.Fatalf("send disconnect: %v", err)
	}
	waitSerialProxyValue(t, port.closed)
}

func TestSerialProxyRevocationAndReadFailureClosePort(t *testing.T) {
	var allowed atomic.Bool
	allowed.Store(true)
	conn, port := startSerialProxyTest(t, RemoteProxyOptions{MaxSessionDuration: 5 * time.Second, IdleTimeout: 3 * time.Second}, func(*http.Request) bool {
		return allowed.Load()
	})
	allowed.Store(false)
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte{0x01}); err != nil {
		t.Fatalf("send after revoke: %v", err)
	}
	if status := readSerialProxyStatus(t, conn); status.Type != "error" || status.Code != "unauthorized" {
		t.Fatalf("revocation status = %+v", status)
	}
	waitSerialProxyValue(t, port.closed)
	select {
	case got := <-port.writes:
		t.Fatalf("revoked bytes reached serial port: %v", got)
	default:
	}

	conn, port = startSerialProxyTest(t, RemoteProxyOptions{MaxSessionDuration: 5 * time.Second, IdleTimeout: 3 * time.Second}, nil)
	port.reads <- serialProxyRead{err: errors.New("simulated read failure")}
	waitSerialProxyValue(t, port.closed)
	_ = conn.Close()
}

func TestSerialProxyIdleTimeoutClosesPort(t *testing.T) {
	conn, port := startSerialProxyTest(t, RemoteProxyOptions{MaxSessionDuration: 5 * time.Second, IdleTimeout: 80 * time.Millisecond}, nil)
	waitSerialProxyValue(t, port.closed)
	_ = conn.Close()
}

func TestSerialProxyDisconnectAndQueueOverflowCancelStalledWrite(t *testing.T) {
	for _, cause := range []string{"disconnect", "queue overflow", "serial read failure"} {
		t.Run(cause, func(t *testing.T) {
			conn, port := startSerialProxyTest(t, RemoteProxyOptions{MaxSessionDuration: 5 * time.Second, IdleTimeout: 3 * time.Second}, nil)
			port.setWriteGate(make(chan struct{}))
			if err := conn.WriteMessage(websocket.BinaryMessage, []byte{0x01}); err != nil {
				t.Fatalf("send stalled write: %v", err)
			}
			waitSerialProxyValue(t, port.writeStarted)
			switch cause {
			case "queue overflow":
				for i := 0; i < serialProxyQueueDepth+2; i++ {
					if err := conn.WriteMessage(websocket.BinaryMessage, []byte{byte(i)}); err != nil {
						t.Fatalf("fill input queue: %v", err)
					}
				}
			case "serial read failure":
				port.reads <- serialProxyRead{err: errors.New("simulated read failure")}
			default:
				if err := conn.Close(); err != nil {
					t.Fatalf("close client: %v", err)
				}
			}
			waitSerialProxyValue(t, port.closed)
		})
	}
}

func TestSerialModeAndControlValidation(t *testing.T) {
	mode := serialMode(defaultSerialOptions())
	if mode.BaudRate != 115200 || mode.DataBits != 8 || mode.StopBits != serial.OneStopBit || mode.Parity != serial.NoParity || mode.InitialStatusBits == nil || mode.InitialStatusBits.DTR || mode.InitialStatusBits.RTS {
		t.Fatalf("default serial mode = %+v", mode)
	}
	if !validSerialControl(serialControl{Type: "signals", DTR: boolPointer(true), RTS: boolPointer(false)}) {
		t.Fatal("valid signals control rejected")
	}
	for _, invalid := range []serialControl{{Type: "signals"}, {Type: "signals", DTR: boolPointer(true)}, {Type: "break", DTR: boolPointer(false)}, {Type: "unknown"}} {
		if validSerialControl(invalid) {
			t.Fatalf("invalid control accepted: %+v", invalid)
		}
	}
}

func boolPointer(value bool) *bool { return &value }
