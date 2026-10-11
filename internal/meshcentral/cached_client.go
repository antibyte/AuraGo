package meshcentral

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

// CachedClient wraps Client with connection reuse, auto-reconnect, and idle timeout.
type CachedClient struct {
	url         string
	username    string
	password    string
	loginToken  string
	insecure    bool
	caFile      string
	logger      *slog.Logger
	idleTimeout time.Duration

	mu       sync.Mutex
	client   *Client
	lastUsed time.Time
}

// NewCachedClient creates a new cached MeshCentral client.
func NewCachedClient(url, username, password, loginToken string, insecure bool, logger *slog.Logger) *CachedClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &CachedClient{
		url:         url,
		username:    username,
		password:    password,
		loginToken:  loginToken,
		insecure:    insecure,
		logger:      logger,
		idleTimeout: 5 * time.Minute,
	}
}

// SetLogger updates the logger used by this cached client.
func (cc *CachedClient) SetLogger(l *slog.Logger) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.logger = l
}

// SetCAFile sets the PEM CA passed to every connection this cache opens.
func (cc *CachedClient) SetCAFile(path string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.caFile = strings.TrimSpace(path)
}

// GetClientContext returns a connected Client with context cancellation, reconnecting if necessary.
func (cc *CachedClient) GetClientContext(ctx context.Context) (*Client, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Close idle or disconnected connection
	if cc.client != nil && (time.Since(cc.lastUsed) > cc.idleTimeout || !cc.client.IsConnected()) {
		if !cc.client.IsConnected() {
			cc.logger.Info("[MeshCentral] Connection lost, reconnecting")
		} else {
			cc.logger.Info("[MeshCentral] Connection idle, closing")
		}
		cc.invalidateUnlocked()
	}

	// Create new connection if needed
	if cc.client == nil {
		client, err := NewClient(cc.url, cc.username, cc.password, cc.loginToken, cc.insecure)
		if err != nil {
			return nil, fmt.Errorf("failed to create MeshCentral client: %w", err)
		}
		client.SetCAFile(cc.caFile)
		client.SetLogger(cc.logger)

		if err := client.ConnectContext(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to MeshCentral: %w", err)
		}

		cc.client = client
		cc.logger.Info("[MeshCentral] New connection established")
	}

	cc.lastUsed = time.Now()
	return cc.client, nil
}

// GetClient returns a connected Client with default Background context.
func (cc *CachedClient) GetClient() (*Client, error) {
	return cc.GetClientContext(context.Background())
}

// invalidateUnlocked closes the current connection without acquiring the lock.
// Caller must hold cc.mu.
func (cc *CachedClient) invalidateUnlocked() {
	if cc.client != nil {
		cc.client.Close()
		cc.client = nil
	}
}

// invalidate closes the current connection so the next GetClient creates a new one.
func (cc *CachedClient) invalidate() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.invalidateUnlocked()
}

// Close closes the cached connection.
func (cc *CachedClient) Close() {
	cc.invalidate()
}

// executeWithRetry runs a read-only operation, reconnecting once on connection failure.
func (cc *CachedClient) executeWithRetry(ctx context.Context, op func(context.Context, *Client) error) error {
	client, err := cc.GetClientContext(ctx)
	if err != nil {
		return err
	}

	if err := op(ctx, client); err != nil {
		if isConnectionError(err) {
			cc.invalidate()
			client, err = cc.GetClientContext(ctx)
			if err != nil {
				return err
			}
			return op(ctx, client)
		}
		return err
	}
	return nil
}

// executeMutation runs a mutating operation without automatically retrying on failure.
// If a connection error occurs, the cached connection is invalidated so subsequent
// requests start with a fresh connection, but the mutation itself is never replayed.
func (cc *CachedClient) executeMutation(ctx context.Context, op func(context.Context, *Client) error) error {
	client, err := cc.GetClientContext(ctx)
	if err != nil {
		return err
	}

	if err := op(ctx, client); err != nil {
		if isConnectionError(err) {
			cc.invalidate()
		}
		return err
	}
	return nil
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errNotConnected) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "websocket disconnected") ||
		strings.Contains(msg, "not connected") ||
		strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset")
}

// ListDeviceGroups requests the meshes/groups list with auto-reconnect.
func (cc *CachedClient) ListDeviceGroups() ([]interface{}, error) {
	return cc.ListDeviceGroupsContext(context.Background())
}

// ListDeviceGroupsContext requests the meshes/groups list with auto-reconnect and context cancellation.
func (cc *CachedClient) ListDeviceGroupsContext(ctx context.Context) ([]interface{}, error) {
	var result []interface{}
	err := cc.executeWithRetry(ctx, func(cctx context.Context, c *Client) error {
		var err error
		result, err = c.ListDeviceGroupsContext(cctx)
		return err
	})
	return result, err
}

// ServerInfo returns server metadata with auto-reconnect.
func (cc *CachedClient) ServerInfo() (map[string]interface{}, error) {
	return cc.ServerInfoContext(context.Background())
}

// ServerInfoContext returns server metadata with auto-reconnect and context cancellation.
func (cc *CachedClient) ServerInfoContext(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := cc.executeWithRetry(ctx, func(cctx context.Context, c *Client) error {
		var err error
		result, err = c.ServerInfoContext(cctx)
		return err
	})
	return result, err
}

// ListDevices requests the nodes/devices list with auto-reconnect.
func (cc *CachedClient) ListDevices(meshID string) ([]interface{}, error) {
	return cc.ListDevicesContext(context.Background(), meshID)
}

// ListDevicesContext requests the nodes/devices list with auto-reconnect and context cancellation.
func (cc *CachedClient) ListDevicesContext(ctx context.Context, meshID string) ([]interface{}, error) {
	var result []interface{}
	err := cc.executeWithRetry(ctx, func(cctx context.Context, c *Client) error {
		var err error
		result, err = c.ListDevicesContext(cctx, meshID)
		return err
	})
	return result, err
}

// ListEvents returns audit events with auto-reconnect.
func (cc *CachedClient) ListEvents(nodeID, userID string, limit int) ([]interface{}, error) {
	return cc.ListEventsContext(context.Background(), nodeID, userID, limit)
}

// ListEventsContext returns audit events with auto-reconnect and context cancellation.
func (cc *CachedClient) ListEventsContext(ctx context.Context, nodeID, userID string, limit int) ([]interface{}, error) {
	var result []interface{}
	err := cc.executeWithRetry(ctx, func(cctx context.Context, c *Client) error {
		var err error
		result, err = c.ListEventsContext(cctx, nodeID, userID, limit)
		return err
	})
	return result, err
}

// DeviceInfo returns device detail responses with auto-reconnect.
func (cc *CachedClient) DeviceInfo(nodeID string) (map[string]interface{}, error) {
	return cc.DeviceInfoContext(context.Background(), nodeID)
}

// DeviceInfoContext returns device detail responses with auto-reconnect and context cancellation.
func (cc *CachedClient) DeviceInfoContext(ctx context.Context, nodeID string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := cc.executeWithRetry(ctx, func(cctx context.Context, c *Client) error {
		var err error
		result, err = c.DeviceInfoContext(cctx, nodeID)
		return err
	})
	return result, err
}

// WakeOnLan sends a WOL magic packet without replaying on failure.
func (cc *CachedClient) WakeOnLan(nodeIDs []string) (string, error) {
	return cc.WakeOnLanContext(context.Background(), nodeIDs)
}

// WakeOnLanContext sends a WOL magic packet without replaying on failure and with context cancellation.
func (cc *CachedClient) WakeOnLanContext(ctx context.Context, nodeIDs []string) (string, error) {
	var result string
	err := cc.executeMutation(ctx, func(cctx context.Context, c *Client) error {
		var err error
		result, err = c.WakeOnLanContext(cctx, nodeIDs)
		return err
	})
	return result, err
}

// PowerAction sends a power action without replaying on failure.
func (cc *CachedClient) PowerAction(nodeIDs []string, powerAction int) (string, error) {
	return cc.PowerActionContext(context.Background(), nodeIDs, powerAction)
}

// PowerActionContext sends a power action without replaying on failure and with context cancellation.
func (cc *CachedClient) PowerActionContext(ctx context.Context, nodeIDs []string, powerAction int) (string, error) {
	var result string
	err := cc.executeMutation(ctx, func(cctx context.Context, c *Client) error {
		var err error
		result, err = c.PowerActionContext(cctx, nodeIDs, powerAction)
		return err
	})
	return result, err
}

// RunCommand executes a shell command with single-execution semantics (never replayed on connection error).
func (cc *CachedClient) RunCommand(nodeID, command string) (map[string]interface{}, error) {
	return cc.RunCommandContext(context.Background(), nodeID, command)
}

// RunCommandContext executes a shell command with single-execution semantics and context cancellation.
func (cc *CachedClient) RunCommandContext(ctx context.Context, nodeID, command string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := cc.executeMutation(ctx, func(cctx context.Context, c *Client) error {
		var err error
		result, err = c.RunCommandContext(cctx, nodeID, command)
		return err
	})
	return result, err
}

// Shell is intentionally unsupported until the MeshRelay tunnel protocol is implemented.
func (cc *CachedClient) Shell(nodeID, command string) (map[string]interface{}, error) {
	return cc.ShellContext(context.Background(), nodeID, command)
}

// ShellContext is intentionally unsupported until the MeshRelay tunnel protocol is implemented.
func (cc *CachedClient) ShellContext(ctx context.Context, nodeID, command string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("unsupported: MeshCentral shell requires a meshrelay.ashx tunnel, which is not implemented")
}
