package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"aurago/internal/remote"

	"github.com/gorilla/websocket"
)

// Build-injected variables (fallback for trailer injection).
var (
	BuildVersion = "dev"
)

func main() {
	installFlag := flag.Bool("install", false, "Install as system service and start")
	uninstallFlag := flag.Bool("uninstall", false, "Stop service and remove")
	statusFlag := flag.Bool("status", false, "Show connection status")
	supervisorFlag := flag.String("supervisor", "", "Supervisor WebSocket URL")
	tokenFlag := flag.String("token", "", "Enrollment token")
	nameFlag := flag.String("name", "", "Device name")
	foregroundFlag := flag.Bool("foreground", false, "Run in foreground")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("AuraGo Remote %s (%s/%s)\n", BuildVersion, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	if *statusFlag {
		printStatus()
		os.Exit(0)
	}

	if *installFlag {
		if err := installService(); err != nil {
			log.Fatalf("Failed to install service: %v", err)
		}
		fmt.Println("AuraGo Remote service installed and started.")
		os.Exit(0)
	}

	if *uninstallFlag {
		if err := uninstallService(); err != nil {
			log.Fatalf("Failed to uninstall service: %v", err)
		}
		fmt.Println("AuraGo Remote service stopped and removed.")
		os.Exit(0)
	}

	// Load configuration: CLI flags > trailer config > stored config
	cfg := loadConfig(*supervisorFlag, *tokenFlag, *nameFlag)
	if cfg.SupervisorURL == "" {
		log.Fatal("No supervisor URL configured. Use --supervisor or download a personalized binary.")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if !*foregroundFlag && !isRunningAsService() {
		fmt.Println("Running in foreground. Use --install to install as a service, or --foreground to suppress this message.")
	}

	client := &Client{
		cfg:     cfg,
		logger:  logger,
		version: BuildVersion,
		done:    make(chan struct{}),
	}

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Shutdown signal received")
		client.Stop()
	}()

	client.Run()
}

// ── Config loading ──────────────────────────────────────────────────────────

type clientConfig struct {
	SupervisorURL string `json:"supervisor_url"`
	EnrollToken   string `json:"enroll_token,omitempty"`
	DeviceName    string `json:"device_name,omitempty"`
	DeviceID      string `json:"device_id,omitempty"`
	SharedKey     string `json:"shared_key,omitempty"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aurago-remote")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func loadConfig(supervisorURL, token, name string) clientConfig {
	var cfg clientConfig

	// 1. Try binary trailer
	if trailer := loadTrailerConfig(); trailer != nil {
		cfg.SupervisorURL = trailer.SupervisorURL
		cfg.EnrollToken = trailer.EnrollToken
		cfg.DeviceName = trailer.DeviceName
	}

	// 2. Try stored config (restores device_id, shared_key from previous enrollment).
	// The supervisor_url from stored config is only used when the binary has no trailer
	// (i.e. not a personalized download). A personalized binary's trailer URL always wins.
	if stored := loadStoredConfig(); stored != nil {
		if stored.SupervisorURL != "" && cfg.SupervisorURL == "" {
			cfg.SupervisorURL = stored.SupervisorURL
		}
		cfg.DeviceID = stored.DeviceID
		cfg.SharedKey = stored.SharedKey
		if stored.DeviceName != "" {
			cfg.DeviceName = stored.DeviceName
		}
		// If we already have a device_id the enrollment token has been consumed.
		// Clear it so we take the reconnect path (Case 1) instead of re-sending
		// the trailer token and getting "enrollment token already used".
		if cfg.DeviceID != "" {
			cfg.EnrollToken = ""
		}
	}

	// 3. CLI flags override everything
	if supervisorURL != "" {
		cfg.SupervisorURL = supervisorURL
	}
	if token != "" {
		cfg.EnrollToken = token
	}
	if name != "" {
		cfg.DeviceName = name
	}

	return cfg
}

func loadTrailerConfig() *remote.BinaryConfig {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}

	// Only read the last 1MB + trailer overhead to find the config trailer
	// This is much more efficient than reading the entire binary
	const maxTrailerSize = 1<<20 + 64 // 1MB + overhead
	fi, err := os.Stat(exePath)
	if err != nil {
		return nil
	}

	// If file is small enough, read it all
	if fi.Size() <= maxTrailerSize*2 {
		data, err := os.ReadFile(exePath)
		if err != nil {
			return nil
		}
		cfg, err := remote.ParseBinaryTrailer(data)
		if err != nil {
			return nil
		}
		return cfg
	}

	// Read only the tail of the file
	file, err := os.Open(exePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	// Seek to position: file size - maxTrailerSize
	_, err = file.Seek(-maxTrailerSize, io.SeekEnd)
	if err != nil {
		return nil
	}

	data := make([]byte, maxTrailerSize)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		return nil
	}
	data = data[:n]

	cfg, err := remote.ParseBinaryTrailer(data)
	if err != nil {
		return nil
	}
	return cfg
}

func loadStoredConfig() *clientConfig {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil
	}
	var cfg clientConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func saveConfig(cfg clientConfig) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}

// ── Client ──────────────────────────────────────────────────────────────────

// Client manages the WebSocket connection to the supervisor.
type Client struct {
	cfg     clientConfig
	logger  *slog.Logger
	version string
	conn    *websocket.Conn
	connMu  sync.Mutex
	seq     uint64
	seqMu   sync.Mutex
	done    chan struct{}

	// executor handles command execution
	executor *Executor

	// State
	readOnly     bool
	allowedPaths []string
	stateMu      sync.RWMutex
}

func (c *Client) nextSeq() uint64 {
	c.seqMu.Lock()
	defer c.seqMu.Unlock()
	c.seq++
	return c.seq
}

func (c *Client) send(msg *remote.RemoteMessage) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteJSON(msg)
}

// Run connects to the supervisor with auto-reconnect.
func (c *Client) Run() {
	c.executor = NewExecutor(c.logger)
	backoff := 5 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-c.done:
			return
		default:
		}

		err := c.connect()
		if err != nil {
			c.logger.Error("Connection failed", "error", err, "retry_in", backoff)
			select {
			case <-time.After(backoff):
			case <-c.done:
				return
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		// Connected — reset backoff
		backoff = 5 * time.Second
		c.readMessages()
	}
}

// Stop gracefully disconnects.
func (c *Client) Stop() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	c.connMu.Lock()
	if c.conn != nil {
		_ = c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()
}

func (c *Client) connect() error {
	u, err := url.Parse(c.cfg.SupervisorURL)
	if err != nil {
		return fmt.Errorf("invalid supervisor URL: %w", err)
	}

	c.logger.Info("Connecting to supervisor", "url", u.String())

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("WebSocket dial failed: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	// Send auth
	hostname, _ := os.Hostname()
	auth := remote.AuthPayload{
		Version:  c.version,
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Token:    c.cfg.EnrollToken,
		DeviceID: c.cfg.DeviceID,
	}

	msg, err := remote.NewMessage(remote.MsgAuth, c.cfg.DeviceID, c.cfg.SharedKey, c.nextSeq(), auth)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create auth message: %w", err)
	}

	if err := conn.WriteJSON(msg); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send auth: %w", err)
	}

	// Wait for auth response
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, data, err := conn.ReadMessage()
	conn.SetReadDeadline(time.Time{}) // clear deadline
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	var resp remote.RemoteMessage
	if err := json.Unmarshal(data, &resp); err != nil {
		conn.Close()
		return fmt.Errorf("invalid auth response: %w", err)
	}

	var authResp remote.AuthResponsePayload
	if err := json.Unmarshal(resp.Payload, &authResp); err != nil {
		conn.Close()
		return fmt.Errorf("invalid auth response payload: %w", err)
	}

	switch authResp.Status {
	case "enrolled":
		c.logger.Info("Enrolled successfully", "device_id", authResp.DeviceID)
		c.cfg.DeviceID = authResp.DeviceID
		c.cfg.SharedKey = authResp.SharedKey
		c.cfg.EnrollToken = "" // consumed
		if err := saveConfig(c.cfg); err != nil {
			c.logger.Error("Failed to save config after enrollment", "error", err)
		}
	case "authenticated":
		c.logger.Info("Authenticated", "device_id", authResp.DeviceID)
	case "pending":
		c.logger.Info("Awaiting approval in AuraGo UI", "device_id", authResp.DeviceID)
		c.cfg.DeviceID = authResp.DeviceID
		if err := saveConfig(c.cfg); err != nil {
			c.logger.Error("Failed to save config", "error", err)
		}
		conn.Close()
		return fmt.Errorf("pending approval")
	case "rejected":
		conn.Close()
		return fmt.Errorf("enrollment rejected: %s", authResp.Message)
	default:
		conn.Close()
		return fmt.Errorf("unknown auth status: %s", authResp.Status)
	}

	// Start heartbeat
	go c.heartbeatLoop()

	c.logger.Info("Connected to supervisor")
	return nil
}

func (c *Client) readMessages() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Warn("Connection error", "error", err)
			}
			return
		}

		var msg remote.RemoteMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.logger.Warn("Invalid message", "error", err)
			continue
		}

		// Verify HMAC if we have a shared key
		if c.cfg.SharedKey != "" {
			ok, err := remote.VerifyMessage(msg, c.cfg.SharedKey)
			if err != nil || !ok {
				c.logger.Warn("HMAC verification failed, ignoring message")
				continue
			}
		}

		c.handleMessage(msg)
	}
}

func (c *Client) handleMessage(msg remote.RemoteMessage) {
	switch msg.Type {
	case remote.MsgCommand:
		go c.handleCommand(msg)
	case remote.MsgConfigUpdate:
		c.handleConfigUpdate(msg)
	case remote.MsgRevoke:
		c.handleRevoke()
	case remote.MsgError:
		var ep remote.ErrorPayload
		if json.Unmarshal(msg.Payload, &ep) == nil {
			c.logger.Warn("Error from supervisor", "code", ep.Code, "msg", ep.Message)
		}
	default:
		c.logger.Debug("Unknown message type", "type", msg.Type)
	}
}

func (c *Client) handleCommand(msg remote.RemoteMessage) {
	var cmd remote.CommandPayload
	if err := json.Unmarshal(msg.Payload, &cmd); err != nil {
		c.logger.Warn("Invalid command payload", "error", err)
		return
	}

	// Client-side read-only enforcement
	c.stateMu.RLock()
	readOnly := c.readOnly
	allowedPaths := c.allowedPaths
	c.stateMu.RUnlock()

	if readOnly && !remote.ReadOnlySafe(cmd.Operation) {
		c.sendResult(cmd.CommandID, "denied", "", "device is in read-only mode", 0)
		return
	}

	start := time.Now()
	result := c.executor.Execute(cmd, readOnly, allowedPaths)
	result.DurationMs = time.Since(start).Milliseconds()

	c.sendResult(result.CommandID, result.Status, result.Output, result.Error, result.DurationMs)
}

func (c *Client) sendResult(cmdID, status, output, errMsg string, durationMs int64) {
	result := remote.ResultPayload{
		CommandID:  cmdID,
		Status:     status,
		Output:     output,
		Error:      errMsg,
		DurationMs: durationMs,
	}
	msg, err := remote.NewMessage(remote.MsgResult, c.cfg.DeviceID, c.cfg.SharedKey, c.nextSeq(), result)
	if err != nil {
		c.logger.Error("Failed to create result message", "error", err)
		return
	}
	if err := c.send(msg); err != nil {
		c.logger.Error("Failed to send result", "error", err)
	}
}

func (c *Client) handleConfigUpdate(msg remote.RemoteMessage) {
	var update remote.ConfigUpdatePayload
	if err := json.Unmarshal(msg.Payload, &update); err != nil {
		return
	}
	c.stateMu.Lock()
	if update.ReadOnly != nil {
		c.readOnly = *update.ReadOnly
	}
	if update.AllowedPaths != nil {
		c.allowedPaths = update.AllowedPaths
	}
	c.stateMu.Unlock()
	c.logger.Info("Config updated", "read_only", c.readOnly)
}

func (c *Client) handleRevoke() {
	c.logger.Warn("Device revoked by supervisor — uninstalling")
	// Clean up stored config
	_ = os.RemoveAll(configDir())
	// Try to uninstall service
	_ = uninstallService()
	c.Stop()
	os.Exit(0)
}

func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hb := c.executor.CollectSysinfo()
			hb.Version = c.version
			msg, err := remote.NewMessage(remote.MsgHeartbeat, c.cfg.DeviceID, c.cfg.SharedKey, c.nextSeq(), hb)
			if err != nil {
				continue
			}
			if err := c.send(msg); err != nil {
				c.logger.Debug("Heartbeat send failed", "error", err)
				return
			}
		case <-c.done:
			return
		}
	}
}

func printStatus() {
	cfg := loadStoredConfig()
	if cfg == nil {
		fmt.Println("Not configured. Run with --supervisor URL or download a personalized binary.")
		return
	}
	fmt.Printf("Device ID:      %s\n", cfg.DeviceID)
	fmt.Printf("Supervisor:     %s\n", cfg.SupervisorURL)
	fmt.Printf("Device Name:    %s\n", cfg.DeviceName)
	if cfg.SharedKey != "" {
		fmt.Println("Status:         Enrolled (shared key present)")
	} else {
		fmt.Println("Status:         Not yet enrolled")
	}
}

func isRunningAsService() bool {
	// Heuristic: if stdin is not a terminal, likely running as service
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice == 0
}
