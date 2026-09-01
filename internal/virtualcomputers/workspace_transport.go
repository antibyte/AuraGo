package virtualcomputers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	workspaceMaxWireMessageBytes   = 8 * 1024 * 1024
	workspaceTransportWriteTimeout = 3 * time.Second
)

type WorkspaceTransport interface {
	Call(ctx context.Context, machineID, method string, params interface{}, result interface{}) error
}

type WorkspaceRPCError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

func (e WorkspaceRPCError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return strings.TrimSpace(e.Code)
	}
	if strings.TrimSpace(e.Code) == "" {
		return strings.TrimSpace(e.Message)
	}
	return strings.TrimSpace(e.Code) + ": " + strings.TrimSpace(e.Message)
}

type workspaceRPCRequest struct {
	JSONRPC  string      `json:"jsonrpc"`
	Protocol string      `json:"protocol"`
	ID       string      `json:"id"`
	Method   string      `json:"method"`
	Deadline string      `json:"deadline,omitempty"`
	Sequence uint64      `json:"sequence"`
	Params   interface{} `json:"params,omitempty"`
}

type workspaceRPCResponse struct {
	JSONRPC  string             `json:"jsonrpc"`
	Protocol string             `json:"protocol"`
	ID       string             `json:"id"`
	Sequence uint64             `json:"sequence"`
	Result   json.RawMessage    `json:"result,omitempty"`
	Error    *WorkspaceRPCError `json:"error,omitempty"`
}

type WebSocketWorkspaceTransport struct {
	client   *Client
	dialer   *websocket.Dialer
	sequence atomic.Uint64
}

func NewWebSocketWorkspaceTransport(client *Client) *WebSocketWorkspaceTransport {
	return &WebSocketWorkspaceTransport{
		client: client,
		dialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
			Proxy:            http.ProxyFromEnvironment,
		},
	}
}

func (t *WebSocketWorkspaceTransport) Call(ctx context.Context, machineID, method string, params interface{}, result interface{}) error {
	if t == nil || t.client == nil {
		return fmt.Errorf("workspace transport is not configured")
	}
	machineID = strings.TrimSpace(machineID)
	method = strings.TrimSpace(method)
	if machineID == "" || method == "" {
		return fmt.Errorf("workspace machine id and method are required")
	}
	endpoint, header, err := t.client.WebSocketURL(machineID, WorkspaceRPCPath)
	if err != nil {
		return err
	}
	conn, response, err := t.dialer.DialContext(ctx, endpoint, header)
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
				return WorkspaceRPCError{Code: "workspace_busy", Message: "workspace operation capacity reached"}
			}
			if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUpgradeRequired || response.StatusCode == http.StatusNotImplemented {
				return WorkspaceRPCError{Code: "workspace_agent_upgrade_required", Message: "boringd does not expose the AuraGo workspace capability"}
			}
		}
		return fmt.Errorf("connect workspace agent: %w", err)
	}
	defer conn.Close()
	conn.SetReadLimit(workspaceMaxWireMessageBytes)
	requestID, err := newWorkspaceRequestID()
	if err != nil {
		return err
	}
	request := workspaceRPCRequest{
		JSONRPC:  "2.0",
		Protocol: WorkspaceProtocolVersion,
		ID:       requestID,
		Method:   method,
		Sequence: t.sequence.Add(1),
		Params:   params,
	}
	writeDeadline := time.Now().Add(workspaceTransportWriteTimeout)
	if deadline, ok := ctx.Deadline(); ok {
		request.Deadline = deadline.UTC().Format(time.RFC3339Nano)
		if deadline.Before(writeDeadline) {
			writeDeadline = deadline
		}
		_ = conn.SetReadDeadline(deadline)
	}
	_ = conn.SetWriteDeadline(writeDeadline)
	if err := conn.WriteJSON(request); err != nil {
		return fmt.Errorf("send workspace request: %w", err)
	}
	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read workspace response: %w", err)
		}
		if messageType != websocket.TextMessage {
			continue
		}
		var response workspaceRPCResponse
		if err := json.Unmarshal(payload, &response); err != nil {
			return fmt.Errorf("decode workspace response: %w", err)
		}
		if response.ID != requestID {
			continue
		}
		if response.Protocol != "" && response.Protocol != WorkspaceProtocolVersion {
			return WorkspaceRPCError{Code: "workspace_protocol_mismatch", Message: "guest workspace protocol version does not match AuraGo"}
		}
		if response.Error != nil {
			return *response.Error
		}
		if result == nil || len(response.Result) == 0 || string(response.Result) == "null" {
			return nil
		}
		if err := json.Unmarshal(response.Result, result); err != nil {
			return fmt.Errorf("decode workspace result: %w", err)
		}
		return nil
	}
}

func newWorkspaceRequestID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate workspace request id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

type WorkspaceHandshakeRequest struct {
	ProtocolVersion string `json:"protocol_version"`
	MachineID       string `json:"machine_id"`
	InstanceNonce   string `json:"instance_nonce"`
}

func WorkspaceHandshake(ctx context.Context, transport WorkspaceTransport, machineID, previousNonce string) (WorkspaceCapabilities, error) {
	if transport == nil {
		return WorkspaceCapabilities{}, fmt.Errorf("workspace transport is not configured")
	}
	var capabilities WorkspaceCapabilities
	err := transport.Call(ctx, machineID, "system.capabilities", WorkspaceHandshakeRequest{
		ProtocolVersion: WorkspaceProtocolVersion,
		MachineID:       machineID,
		InstanceNonce:   strings.TrimSpace(previousNonce),
	}, &capabilities)
	if err != nil {
		return WorkspaceCapabilities{}, err
	}
	if capabilities.ProtocolVersion != WorkspaceProtocolVersion {
		return WorkspaceCapabilities{}, WorkspaceRPCError{
			Code:    "workspace_protocol_mismatch",
			Message: fmt.Sprintf("guest reports %q, AuraGo requires %q", capabilities.ProtocolVersion, WorkspaceProtocolVersion),
		}
	}
	if capabilities.MachineID != "" && capabilities.MachineID != machineID {
		return WorkspaceCapabilities{}, WorkspaceRPCError{Code: "workspace_machine_mismatch", Message: "guest workspace agent is bound to another machine"}
	}
	if strings.TrimSpace(capabilities.InstanceNonce) == "" {
		return WorkspaceCapabilities{}, WorkspaceRPCError{Code: "workspace_invalid_handshake", Message: "guest workspace agent omitted its instance nonce"}
	}
	return capabilities, nil
}
