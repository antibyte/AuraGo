package meshcentral

import (
	"fmt"
	"log/slog"
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

// GetClient returns a connected Client, reconnecting if necessary.
func (cc *CachedClient) GetClient() (*Client, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Close idle connection
	if cc.client != nil && time.Since(cc.lastUsed) > cc.idleTimeout {
		cc.logger.Info("[MeshCentral] Connection idle, closing")
		cc.invalidateUnlocked()
	}

	// Create new connection if needed
	if cc.client == nil {
		client, err := NewClient(cc.url, cc.username, cc.password, cc.loginToken, cc.insecure)
		if err != nil {
			return nil, fmt.Errorf("failed to create MeshCentral client: %w", err)
		}
		client.SetLogger(cc.logger)

		if err := client.Connect(); err != nil {
			return nil, fmt.Errorf("failed to connect to MeshCentral: %w", err)
		}

		cc.client = client
		cc.logger.Info("[MeshCentral] New connection established")
	}

	cc.lastUsed = time.Now()
	return cc.client, nil
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

// executeWithRetry runs an operation, reconnecting once on connection failure.
func (cc *CachedClient) executeWithRetry(op func(*Client) error) error {
	client, err := cc.GetClient()
	if err != nil {
		return err
	}

	if err := op(client); err != nil {
		if isConnectionError(err) {
			cc.invalidate()
			client, err = cc.GetClient()
			if err != nil {
				return err
			}
			return op(client)
		}
		return err
	}
	return nil
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not connected") ||
		strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "websocket") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset")
}

// ListDeviceGroups requests the meshes/groups list with auto-reconnect.
func (cc *CachedClient) ListDeviceGroups() ([]interface{}, error) {
	var result []interface{}
	err := cc.executeWithRetry(func(c *Client) error {
		var err error
		result, err = c.ListDeviceGroups()
		return err
	})
	return result, err
}

// ServerInfo returns server metadata with auto-reconnect.
func (cc *CachedClient) ServerInfo() (map[string]interface{}, error) {
	var result map[string]interface{}
	err := cc.executeWithRetry(func(c *Client) error {
		var err error
		result, err = c.ServerInfo()
		return err
	})
	return result, err
}

// ListDevices requests the nodes/devices list with auto-reconnect.
func (cc *CachedClient) ListDevices(meshID string) ([]interface{}, error) {
	var result []interface{}
	err := cc.executeWithRetry(func(c *Client) error {
		var err error
		result, err = c.ListDevices(meshID)
		return err
	})
	return result, err
}

// ListEvents returns audit events with auto-reconnect.
func (cc *CachedClient) ListEvents(nodeID, userID string, limit int) ([]interface{}, error) {
	var result []interface{}
	err := cc.executeWithRetry(func(c *Client) error {
		var err error
		result, err = c.ListEvents(nodeID, userID, limit)
		return err
	})
	return result, err
}

// DeviceInfo returns device detail responses with auto-reconnect.
func (cc *CachedClient) DeviceInfo(nodeID string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := cc.executeWithRetry(func(c *Client) error {
		var err error
		result, err = c.DeviceInfo(nodeID)
		return err
	})
	return result, err
}

// WakeOnLan sends a WOL magic packet with auto-reconnect.
func (cc *CachedClient) WakeOnLan(nodeIDs []string) (string, error) {
	var result string
	err := cc.executeWithRetry(func(c *Client) error {
		var err error
		result, err = c.WakeOnLan(nodeIDs)
		return err
	})
	return result, err
}

// PowerAction sends a power action with auto-reconnect.
func (cc *CachedClient) PowerAction(nodeIDs []string, powerAction int) (string, error) {
	var result string
	err := cc.executeWithRetry(func(c *Client) error {
		var err error
		result, err = c.PowerAction(nodeIDs, powerAction)
		return err
	})
	return result, err
}

// RunCommand executes a shell command with auto-reconnect.
func (cc *CachedClient) RunCommand(nodeID, command string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := cc.executeWithRetry(func(c *Client) error {
		var err error
		result, err = c.RunCommand(nodeID, command)
		return err
	})
	return result, err
}

// Shell is intentionally unsupported until the MeshRelay tunnel protocol is implemented.
func (cc *CachedClient) Shell(nodeID, command string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("unsupported: MeshCentral shell requires a meshrelay.ashx tunnel, which is not implemented")
}
