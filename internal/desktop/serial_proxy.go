package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/serialutil"

	"github.com/gorilla/websocket"
	"go.bug.st/serial"
)

const (
	serialProxyReadLimit  = 64 << 10
	serialProxyQueueDepth = 8
	serialProxyChunkSize  = 16 << 10
)

var serialUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     sameHostWebSocketOrigin,
}

type serialStatus struct {
	Type string `json:"type"`
	Code string `json:"code"`
}

type serialOpenRequest struct {
	Type    string          `json:"type"`
	Port    string          `json:"port"`
	Options json.RawMessage `json:"options"`
}

type serialControl struct {
	Type string `json:"type"`
	DTR  *bool  `json:"dtr,omitempty"`
	RTS  *bool  `json:"rts,omitempty"`
}

type serialProxyInput struct {
	binary   []byte
	isBinary bool
	control  serialControl
}

type serialProxyOutput struct {
	data []byte
	err  error
}

type serialOpenFunc func(context.Context, string, string, *serial.Mode) (serial.Port, error)

// HandleSerialPorts lists the canonical names returned by the native serial
// enumerator together with their current in-process lease owner.
func HandleSerialPorts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ports, err := serialutil.List()
		if err != nil {
			writeSerialHTTPError(w, http.StatusServiceUnavailable, "serial_ports_unavailable")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ports": ports})
	}
}

// HandleSerialProxy upgrades one same-origin request into a raw serial bridge.
// The optional authorizer is rechecked before opening and for every byte/control
// operation so revocation takes effect during an active session.
func HandleSerialProxy(options RemoteProxyOptions, authorize ...func(*http.Request) bool) http.HandlerFunc {
	var check func(*http.Request) bool
	if len(authorize) > 0 {
		check = authorize[0]
	}
	return handleSerialProxy(options, check, func(ctx context.Context, name, owner string, mode *serial.Mode) (serial.Port, error) {
		return serialutil.OpenContext(ctx, name, owner, mode)
	})
}

func handleSerialProxy(options RemoteProxyOptions, authorize func(*http.Request) bool, open serialOpenFunc) http.HandlerFunc {
	proxyOptions := normalizeRemoteProxyOptions(options)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameHostWebSocketOrigin(r) {
			http.Error(w, "same-host origin required", http.StatusForbidden)
			return
		}
		if !serialAuthorized(authorize, r) {
			writeSerialHTTPError(w, http.StatusForbidden, "unauthorized")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), proxyOptions.MaxSessionDuration)
		defer cancel()
		conn, err := serialUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetReadLimit(serialProxyReadLimit)
		stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
		defer stopClose()
		if !serialAuthorized(authorize, r) {
			writeSerialStatus(conn, "error", "unauthorized")
			return
		}

		_ = conn.SetReadDeadline(time.Now().Add(remoteProxyWriteTimeout))
		messageType, payload, err := conn.ReadMessage()
		_ = conn.SetReadDeadline(time.Time{})
		if err != nil || messageType != websocket.TextMessage {
			writeSerialStatus(conn, "error", "open_required")
			return
		}
		var request serialOpenRequest
		if err := decodeSerialJSON(payload, &request); err != nil || request.Type != "open" {
			writeSerialStatus(conn, "error", "invalid_open")
			return
		}
		serialOpts, err := parseSerialOptions(request.Options, "host")
		if err != nil {
			writeSerialStatus(conn, "error", "invalid_options")
			return
		}
		if request.Port == "" || len(request.Port) > 256 || strings.TrimSpace(request.Port) != request.Port {
			writeSerialStatus(conn, "error", "invalid_port")
			return
		}
		if !serialAuthorized(authorize, r) {
			writeSerialStatus(conn, "error", "unauthorized")
			return
		}
		port, err := open(ctx, request.Port, serialutil.OwnerQuickConnectHost, serialMode(serialOpts))
		if err != nil {
			writeSerialStatus(conn, "error", serialOpenErrorCode(err))
			return
		}
		if port == nil {
			writeSerialStatus(conn, "error", "open_failed")
			return
		}
		var portMu sync.Mutex
		activePort := port
		closePort := func() {
			portMu.Lock()
			p := activePort
			activePort = nil
			portMu.Unlock()
			if p != nil {
				_ = p.Close()
			}
		}
		stopPortClose := context.AfterFunc(ctx, closePort)
		defer stopPortClose()
		defer closePort()
		if ctx.Err() != nil {
			return
		}
		if err := writeSerialStatus(conn, "connected", "connected"); err != nil {
			return
		}

		inputs := make(chan serialProxyInput, serialProxyQueueDepth)
		outputs := make(chan serialProxyOutput, serialProxyQueueDepth)
		go readSerialWebSocket(ctx, cancel, conn, proxyOptions.IdleTimeout, inputs)
		go readSerialPort(ctx, cancel, port, outputs)

		for {
			select {
			case <-ctx.Done():
				return
			case output := <-outputs:
				if output.err != nil {
					if ctx.Err() == nil {
						_ = writeSerialStatus(conn, "error", "serial_read_failed")
					}
					return
				}
				if !serialAuthorized(authorize, r) {
					_ = writeSerialStatus(conn, "error", "unauthorized")
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(remoteProxyWriteTimeout))
				if err := conn.WriteMessage(websocket.BinaryMessage, output.data); err != nil {
					return
				}
			case input := <-inputs:
				if !serialAuthorized(authorize, r) {
					_ = writeSerialStatus(conn, "error", "unauthorized")
					return
				}
				if input.isBinary {
					if len(input.binary) == 0 {
						continue
					}
					if err := writeAllSerial(port, input.binary); err != nil {
						_ = writeSerialStatus(conn, "error", "serial_write_failed")
						return
					}
					continue
				}
				switch input.control.Type {
				case "signals":
					if err := port.SetDTR(*input.control.DTR); err != nil {
						_ = writeSerialStatus(conn, "error", "serial_control_failed")
						return
					}
					if err := port.SetRTS(*input.control.RTS); err != nil {
						_ = writeSerialStatus(conn, "error", "serial_control_failed")
						return
					}
				case "break":
					if err := port.Break(250 * time.Millisecond); err != nil {
						_ = writeSerialStatus(conn, "error", "serial_control_failed")
						return
					}
				case "disconnect":
					_ = writeSerialStatus(conn, "disconnected", "disconnected")
					return
				default:
					_ = writeSerialStatus(conn, "error", "invalid_control")
					return
				}
			}
		}
	}
}

func serialAuthorized(check func(*http.Request) bool, r *http.Request) bool {
	return r.Context().Err() == nil && (check == nil || check(r))
}

func decodeSerialJSON(payload []byte, target interface{}) error {
	dec := json.NewDecoder(strings.NewReader(string(payload)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing serial JSON")
	}
	return nil
}

func readSerialWebSocket(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, idleTimeout time.Duration, inputs chan<- serialProxyInput) {
	for {
		_ = conn.SetReadDeadline(time.Now().Add(idleTimeout))
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			cancel()
			return
		}
		var input serialProxyInput
		switch messageType {
		case websocket.BinaryMessage:
			input.isBinary = true
			input.binary = append([]byte(nil), payload...)
		case websocket.TextMessage:
			if err := decodeSerialJSON(payload, &input.control); err != nil || !validSerialControl(input.control) {
				cancel()
				return
			}
		default:
			cancel()
			return
		}
		if !input.isBinary && input.control.Type == "disconnect" {
			cancel()
			return
		}
		select {
		case inputs <- input:
		case <-ctx.Done():
			return
		default:
			cancel()
			return
		}
	}
}

func validSerialControl(control serialControl) bool {
	switch control.Type {
	case "signals":
		return control.DTR != nil && control.RTS != nil
	case "break", "disconnect":
		return control.DTR == nil && control.RTS == nil
	default:
		return false
	}
}

func readSerialPort(ctx context.Context, cancel context.CancelFunc, port serial.Port, outputs chan<- serialProxyOutput) {
	buf := make([]byte, serialProxyChunkSize)
	for {
		n, err := port.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			select {
			case outputs <- serialProxyOutput{data: chunk}:
			case <-ctx.Done():
				return
			default:
				cancel()
				return
			}
		}
		if err != nil {
			cancel()
			select {
			case outputs <- serialProxyOutput{err: err}:
			case <-ctx.Done():
			}
			return
		}
		if n == 0 {
			timer := time.NewTimer(time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return
			}
		}
	}
}

func writeAllSerial(port io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := port.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func serialMode(options serialOptions) *serial.Mode {
	parity := serial.NoParity
	switch options.Parity {
	case "even":
		parity = serial.EvenParity
	case "odd":
		parity = serial.OddParity
	}
	stopBits := serial.OneStopBit
	if options.StopBits == 2 {
		stopBits = serial.TwoStopBits
	}
	return &serial.Mode{
		BaudRate: options.BaudRate,
		DataBits: options.DataBits,
		Parity:   parity,
		StopBits: stopBits,
		InitialStatusBits: &serial.ModemOutputBits{
			DTR: options.DTR,
			RTS: options.RTS,
		},
	}
}

func serialOpenErrorCode(err error) string {
	switch {
	case errors.Is(err, serialutil.ErrInvalidPort):
		return "invalid_port"
	case errors.Is(err, serialutil.ErrPortNotFound):
		return "port_not_found"
	case errors.Is(err, serialutil.ErrPortBusy):
		return "port_busy"
	default:
		return "open_failed"
	}
}

func writeSerialStatus(conn *websocket.Conn, kind, code string) error {
	_ = conn.SetWriteDeadline(time.Now().Add(remoteProxyWriteTimeout))
	payload, _ := json.Marshal(serialStatus{Type: kind, Code: code})
	return conn.WriteMessage(websocket.TextMessage, payload)
}

func writeSerialHTTPError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
