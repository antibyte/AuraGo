package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"strings"
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
	logFileFlag := flag.String("log-file", "", "Write logs to a rotating file instead of stderr")
	logMaxMBFlag := flag.Int("log-max-mb", 5, "Maximum remote log file size in MiB before rotation")
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
		exePath, err := os.Executable()
		if err != nil {
			log.Fatalf("Failed to get executable path: %v", err)
		}
		exePath, _ = filepath.Abs(exePath)
		if err := installService(exePath); err != nil {
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

	// The logger depends only on flags. Build it before loadConfig so config
	// warnings reach the --log-file instead of stderr.
	logWriter := io.Writer(os.Stderr)
	if strings.TrimSpace(*logFileFlag) != "" {
		maxBytes := int64(*logMaxMBFlag) * 1024 * 1024
		writer, err := newRotatingFileWriter(*logFileFlag, maxBytes, defaultRemoteLogBackups)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer writer.Close()
		logWriter = writer
	}
	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	// slog.SetDefault also redirects the log package into the handler; keep
	// log.Fatal start-up errors on stderr as before.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags)

	// Load configuration: CLI flags > trailer config > stored config
	cfg := loadConfig(*supervisorFlag, *tokenFlag, *nameFlag)

	// Interactive menu when running in a terminal without explicit flags
	if !*foregroundFlag && !*installFlag && !*uninstallFlag && !*statusFlag && !*versionFlag && *supervisorFlag == "" && *tokenFlag == "" && *nameFlag == "" && isTerminal() {
		choice := showMenu()
		switch choice {
		case "1":
			if err := installPermanent(); err != nil {
				fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "2":
			*foregroundFlag = true
		case "3", "q", "Q":
			fmt.Println("Goodbye.")
			os.Exit(0)
		default:
			fmt.Println("Invalid choice. Exiting.")
			os.Exit(1)
		}
	}

	if cfg.SupervisorURL == "" {
		log.Fatal("No supervisor URL configured. Use --supervisor or download a personalized binary.")
	}

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
		// Older agents persisted the device id from an unsigned "pending" answer.
		// Without a shared key that id can never authenticate, and keeping it
		// would suppress the enrollment token below. The file is left as is;
		// the next "enrolled" answer saves a consistent config.
		if cfg.DeviceID != "" && cfg.SharedKey == "" {
			slog.Warn("stored device id without shared key ignored; re-enrolling", "device_id", cfg.DeviceID)
			cfg.DeviceID = ""
		}
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
	cfg      clientConfig
	logger   *slog.Logger
	version  string
	conn     *websocket.Conn
	connMu   sync.Mutex
	stopOnce sync.Once
	seq      uint64
	seqMu    sync.Mutex
	done     chan struct{}

	// executor handles command execution
	executor *Executor

	// replay holds supervisor frame nonces across reconnects; see rejectReplayedFrame.
	replay       *remote.NonceReplayCache
	replayOnce   sync.Once
	logNoKeyOnce sync.Once

	// State
	readOnly     bool
	allowedPaths []string
	stateMu      sync.RWMutex
}

// handleMessageHook lets tests observe which frames pass the read-loop checks
// without executing them. It is nil in production.
var handleMessageHook func(remote.RemoteMessage)

// agentReplayCacheEntries caps the agent's fail-closed nonce cache. Once it is
// full of live nonces, new frames are dropped until entries expire.
// Legitimate supervisor traffic stays far below it. An on-path attacker who
// hangs up every session right after auth gets one dial per backoff wait (at
// least initialBackoff, since Run resets the backoff only after a
// minStableSession), and error frames are not cached. Even counting eight
// cached frames per dial that is at most ~96 entries per minute, ~3000 per
// NonceReplayTTL, well under the cap.
const agentReplayCacheEntries = 10000

func (c *Client) nextSeq() uint64 {
	c.seqMu.Lock()
	defer c.seqMu.Unlock()
	c.seq++
	return c.seq
}

func (c *Client) send(msg *remote.RemoteMessage) error {
	// connMu serializes all writes and connection replacement/close. gorilla/websocket
	// allows one reader and one writer, but concurrent writers are not safe.
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteJSON(msg)
}

// Reconnect backoff bounds.
const (
	initialBackoff = 5 * time.Second
	maxBackoff     = 60 * time.Second
)

// minStableSession is how long a supervisor session must last before the
// reconnect backoff resets to initialBackoff. Every dial adds an auth-response
// nonce to the fail-closed replay cache, so a session that an on-path attacker
// hangs up right after auth must cost a backoff wait, not an immediate redial.
const minStableSession = 60 * time.Second

// Clock seams for the reconnect loop; tests replace them.
var (
	nowFn   = time.Now
	afterFn = time.After
)

// Run connects to the supervisor with auto-reconnect.
func (c *Client) Run() {
	c.executor = NewExecutor(c.logger, remote.DefaultMaxFileSizeMB)
	backoff := initialBackoff

	// waitBackoff sleeps the current backoff and then grows it. It reports
	// false when the client was stopped while waiting.
	waitBackoff := func() bool {
		select {
		case <-afterFn(backoff):
		case <-c.done:
			return false
		}
		backoff = min(backoff*2, maxBackoff)
		return true
	}

	for {
		select {
		case <-c.done:
			return
		default:
		}

		err := c.connect()
		if err != nil {
			c.logger.Error("Connection failed", "error", err, "retry_in", backoff)
			if !waitBackoff() {
				return
			}
			continue
		}

		started := nowFn()
		c.readMessages()
		select {
		case <-c.done:
			return
		default:
		}
		if nowFn().Sub(started) >= minStableSession {
			backoff = initialBackoff
			continue
		}
		c.logger.Warn("Supervisor session ended early, backing off", "retry_in", backoff)
		if !waitBackoff() {
			return
		}
	}
}

// Stop gracefully disconnects.
func (c *Client) Stop() {
	c.stopOnce.Do(func() {
		close(c.done)
	})
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

	// Send auth
	hostname, _ := os.Hostname()
	auth := remote.AuthPayload{
		Version:   c.version,
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		DeviceID:  c.cfg.DeviceID,
		TokenHash: "",
	}
	authSigningKey := c.cfg.SharedKey
	if c.cfg.DeviceID == "" && c.cfg.EnrollToken != "" {
		// The frame carries only the lookup hash. The MAC key derived from the
		// same token signs it and never leaves this process, so whoever reads
		// this frame cannot sign an answer to it.
		auth.KDF = remote.EnrollmentKDFVersion
		auth.TokenHash = remote.DeriveEnrollmentLookupHash(c.cfg.EnrollToken)
		authSigningKey = remote.DeriveEnrollmentAuthKey(c.cfg.EnrollToken)
	}

	msg, err := remote.NewMessage(remote.MsgAuth, c.cfg.DeviceID, authSigningKey, c.nextSeq(), auth)
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
	signed, err := c.verifyAuthResponse(resp)
	if err != nil {
		conn.Close()
		return err
	}
	if signed {
		if reason := c.rejectReplayedAuthResponse(resp); reason != "" {
			conn.Close()
			return errors.New(reason)
		}
	}

	var authResp remote.AuthResponsePayload
	if err := json.Unmarshal(resp.Payload, &authResp); err != nil {
		conn.Close()
		return fmt.Errorf("invalid auth response payload: %w", err)
	}
	if !signed && authResp.Status != "pending" && authResp.Status != "rejected" {
		conn.Close()
		return fmt.Errorf("unsigned %q auth response rejected: no bootstrap secret to verify it", authResp.Status)
	}

	switch authResp.Status {
	case "enrolled":
		if !validSharedKeyHex(authResp.SharedKey) {
			conn.Close()
			return fmt.Errorf("enrolled response carries an invalid shared key")
		}
		if authResp.DeviceID == "" {
			conn.Close()
			return fmt.Errorf("enrolled response carries no device id")
		}
		c.applyBootstrapSettings(authResp)
		c.logger.Info("Enrolled successfully", "device_id", authResp.DeviceID)
		c.cfg.DeviceID = authResp.DeviceID
		c.cfg.SharedKey = authResp.SharedKey
		c.cfg.EnrollToken = "" // consumed
		if err := saveConfig(c.cfg); err != nil {
			c.logger.Error("Failed to save config after enrollment", "error", err)
		}
	case "authenticated":
		if c.cfg.SharedKey == "" {
			conn.Close()
			return fmt.Errorf("authenticated response without a device shared key")
		}
		c.applyBootstrapSettings(authResp)
		c.logger.Info("Authenticated", "device_id", authResp.DeviceID)
	case "pending":
		// Unsigned by design; nothing from it is persisted. The supervisor keys
		// pending rows by hostname and peer address, and a later --token run
		// must start with an empty DeviceID.
		c.logger.Info("Awaiting approval in AuraGo UI", "observed_device_id", authResp.DeviceID)
		conn.Close()
		return fmt.Errorf("pending approval")
	case "rejected":
		conn.Close()
		return fmt.Errorf("enrollment rejected: %s", authResp.Message)
	default:
		conn.Close()
		return fmt.Errorf("unknown auth status: %s", authResp.Status)
	}

	select {
	case <-c.done:
		conn.Close()
		return fmt.Errorf("client stopped")
	default:
	}

	c.connMu.Lock()
	select {
	case <-c.done:
		c.connMu.Unlock()
		_ = conn.Close()
		return fmt.Errorf("client stopped during connect")
	default:
	}
	if c.conn != nil && c.conn != conn {
		_ = c.conn.Close()
	}
	c.conn = conn
	c.connMu.Unlock()

	// Start heartbeat
	go c.heartbeatLoop()

	c.logger.Info("Connected to supervisor")
	return nil
}

func (c *Client) readMessages() {
	conn := c.currentConn()
	if conn == nil {
		return
	}
	defer c.clearConnection(conn)

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, data, err := conn.ReadMessage()
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

		if c.cfg.SharedKey == "" {
			// Fail closed: without a device key nothing from the supervisor can be
			// trusted. C8 guarantees connect() never reaches this loop without one.
			c.logNoKeyOnce.Do(func() {
				c.logger.Warn("no device shared key; ignoring supervisor frames until enrolled")
			})
			continue
		}
		ok, err := remote.VerifyMessage(msg, c.cfg.SharedKey)
		if err != nil || !ok {
			c.logger.Warn("HMAC verification failed, ignoring message")
			continue
		}
		if reason := c.rejectReplayedFrame(msg); reason != "" {
			c.logger.Warn("Discarding frame", "type", msg.Type, "reason", reason)
			continue
		}

		c.handleMessage(msg)
	}
}

// replayCache returns the agent's fail-closed nonce cache, created on first use
// and kept across reconnects.
func (c *Client) replayCache() *remote.NonceReplayCache {
	c.replayOnce.Do(func() {
		if c.replay == nil {
			// ValidateTimestamp accepts ±MaxTimestampDrift, so a nonce must stay
			// cached for the full window.
			c.replay = remote.NewFailClosedNonceReplayCache(remote.NonceReplayTTL, agentReplayCacheEntries)
		}
	})
	return c.replay
}

// rejectReplayedFrame mirrors the supervisor's checks for frames the agent
// receives after HMAC verification, in the supervisor's order: device binding,
// nonce format, timestamp window and a per-nonce replay cache kept across
// reconnects. It returns the rejection reason or "" when the frame is fresh.
func (c *Client) rejectReplayedFrame(msg remote.RemoteMessage) string {
	// Every supervisor frame carries the device id.
	if msg.DeviceID != c.cfg.DeviceID {
		return "device_id mismatch"
	}
	// hmacData has no field delimiters; a malformed nonce could be sequence
	// digits shifted into it, which the cache would see as a fresh nonce.
	if !remote.ValidNonce(msg.Nonce) {
		return "invalid nonce format"
	}
	if err := remote.ValidateTimestamp(msg.Timestamp); err != nil {
		return err.Error()
	}
	// The agent only logs supervisor errors, so a replayed one is harmless.
	// Not caching them stops the supervisor's error replies to injected bad
	// frames from filling the fail-closed cache.
	if msg.Type == remote.MsgError {
		return ""
	}
	if c.replayCache().Seen(c.cfg.DeviceID, msg.Nonce, time.Now()) {
		return "nonce replayed or replay cache full"
	}
	return ""
}

// authResponseReplayNamespace keys auth-response nonces in the replay cache.
// It cannot be the device id, which is empty while enrolling.
const authResponseReplayNamespace = "auth"

// rejectReplayedAuthResponse makes a signed auth response fresh and single-use
// within this process. connect() applies its read-only flag, allowed paths and
// file-size limit, so a captured reply must not restore stale, looser settings
// on a forced reconnect.
//
// This is a wire-compatible stopgap. The proper fix is for the supervisor to
// echo the agent's auth nonce in the response, a wire change scheduled with
// task C12.
func (c *Client) rejectReplayedAuthResponse(resp remote.RemoteMessage) string {
	if !remote.ValidNonce(resp.Nonce) ||
		remote.ValidateTimestamp(resp.Timestamp) != nil ||
		c.replayCache().Seen(authResponseReplayNamespace, resp.Nonce, time.Now()) {
		return "stale or replayed auth response"
	}
	return ""
}

func (c *Client) currentConn() *websocket.Conn {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return c.conn
}

func (c *Client) clearConnection(conn *websocket.Conn) {
	c.connMu.Lock()
	if c.conn == conn {
		c.conn = nil
	}
	c.connMu.Unlock()
	_ = conn.Close()
}

func (c *Client) handleMessage(msg remote.RemoteMessage) {
	if hook := handleMessageHook; hook != nil {
		hook(msg)
		return
	}
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
	if update.MaxFileSizeMB != nil {
		c.executor.SetMaxFileSizeMB(*update.MaxFileSizeMB)
	}
	c.logger.Info("Config updated", "read_only", c.readOnly, "max_file_size_mb", maxFileSizeFromUpdate(update.MaxFileSizeMB))
}

// verifyAuthResponse verifies the auth response with the bootstrap secret.
// It returns signed=false when the agent holds no secret at all (tokenless
// knock); the caller must then accept only pending/rejected.
func (c *Client) verifyAuthResponse(resp remote.RemoteMessage) (signed bool, err error) {
	verifyKey := ""
	if c.cfg.SharedKey != "" {
		verifyKey = c.cfg.SharedKey
	} else if c.cfg.EnrollToken != "" {
		verifyKey = remote.DeriveEnrollmentAuthKey(c.cfg.EnrollToken)
	}
	if verifyKey == "" {
		return false, nil
	}
	if resp.HMAC == "" {
		return false, fmt.Errorf("received unsigned auth response despite bootstrap key")
	}
	ok, err := remote.VerifyMessage(resp, verifyKey)
	if err != nil {
		return false, fmt.Errorf("failed to verify auth response: %w", err)
	}
	if !ok {
		return false, fmt.Errorf("auth response signature verification failed")
	}
	return true, nil
}

func validSharedKeyHex(key string) bool {
	if len(key) != 64 {
		return false
	}
	_, err := hex.DecodeString(key)
	return err == nil
}

func (c *Client) applyBootstrapSettings(authResp remote.AuthResponsePayload) {
	c.stateMu.Lock()
	if authResp.ReadOnly != nil {
		c.readOnly = *authResp.ReadOnly
	}
	if authResp.AllowedPaths != nil {
		c.allowedPaths = authResp.AllowedPaths
	}
	c.stateMu.Unlock()
	if authResp.MaxFileSizeMB > 0 {
		c.executor.SetMaxFileSizeMB(authResp.MaxFileSizeMB)
	}
}

func maxFileSizeFromUpdate(v *int) int {
	if v == nil {
		return remote.DefaultMaxFileSizeMB
	}
	return *v
}

func (c *Client) handleRevoke() {
	c.logger.Warn("Device revoked by supervisor — uninstalling")
	// Clean up stored config
	_ = os.RemoveAll(configDir())
	// Try to uninstall service
	_ = uninstallService()
	// Remove installed binary
	if installPath, err := getInstallPath(); err == nil {
		_ = os.Remove(installPath)
	}
	c.Stop()
	os.Exit(0)
}

func (c *Client) heartbeatLoop() {
	c.heartbeatLoopWithInterval(30 * time.Second)
}

func (c *Client) heartbeatLoopWithInterval(interval time.Duration) {
	trackedConn := c.currentConn()
	c.heartbeatLoopForConn(interval, trackedConn)
}

func (c *Client) heartbeatLoopForConn(interval time.Duration, trackedConn *websocket.Conn) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if trackedConn != nil && c.currentConn() != trackedConn {
				return
			}
			hb := c.executor.CollectSysinfo()
			hb.Version = c.version
			msg, err := remote.NewMessage(remote.MsgHeartbeat, c.cfg.DeviceID, c.cfg.SharedKey, c.nextSeq(), hb)
			if err != nil {
				continue
			}
			if err := c.send(msg); err != nil {
				c.logger.Debug("Heartbeat send failed", "error", err)
				if trackedConn != nil && c.currentConn() != trackedConn {
					return
				}
				continue
			}
		case <-c.done:
			return
		}
	}
}

func printStatus() {
	writeStatus(os.Stdout, loadStoredConfig())
}

func writeStatus(w io.Writer, cfg *clientConfig) {
	if cfg == nil {
		fmt.Fprintln(w, "Not configured. Run with --supervisor URL or download a personalized binary.")
		return
	}
	// Same rule as loadConfig: a device id without a shared key is a stale
	// pending observation, not an enrollment.
	deviceID := ""
	if cfg.SharedKey != "" {
		deviceID = cfg.DeviceID
	}
	fmt.Fprintf(w, "Device ID:      %s\n", deviceID)
	fmt.Fprintf(w, "Supervisor:     %s\n", cfg.SupervisorURL)
	fmt.Fprintf(w, "Device Name:    %s\n", cfg.DeviceName)
	switch {
	case cfg.SharedKey != "":
		fmt.Fprintln(w, "Status:         Enrolled (shared key present)")
	case cfg.DeviceID != "":
		fmt.Fprintln(w, "Status:         Not yet enrolled (stale pending id, ignored)")
	default:
		fmt.Fprintln(w, "Status:         Not yet enrolled")
	}
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func showMenu() string {
	version := fmt.Sprintf("%s (%s/%s)", BuildVersion, runtime.GOOS, runtime.GOARCH)
	if len(version) > 42 {
		version = version[:42]
	}

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Printf("║ %-42s ║\n", "AuraGo Remote Agent")
	fmt.Printf("║ %-42s ║\n", version)
	fmt.Println("╠════════════════════════════════════════════╣")
	fmt.Println("║                                            ║")
	fmt.Println("║  1) Install AuraGo Remote permanently      ║")
	fmt.Println("║  2) Run only now                           ║")
	fmt.Println("║  3) Quit                                   ║")
	fmt.Println("║                                            ║")
	fmt.Println("╚════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Print("Select an option [1-3]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	return strings.TrimSpace(choice)
}

func installPermanent() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, _ = filepath.Abs(exePath)

	installPath, err := getInstallPath()
	if err != nil {
		return fmt.Errorf("failed to determine install path: %w", err)
	}

	// If already at install path, just install the service
	if exePath == installPath {
		if err := installService(exePath); err != nil {
			return fmt.Errorf("failed to install service: %w", err)
		}
		fmt.Println("AuraGo Remote service installed and started.")
		return nil
	}

	// Create install directory
	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Copy binary
	if err := copyFile(exePath, installPath); err != nil {
		return fmt.Errorf("failed to copy binary to %s: %w", installPath, err)
	}

	// Make executable on Unix
	if runtime.GOOS != "windows" {
		if err := os.Chmod(installPath, 0755); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}
	}

	fmt.Printf("Binary installed to: %s\n", installPath)

	// Install and start service
	if err := installService(installPath); err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	fmt.Println("AuraGo Remote installed and started as a system service.")
	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	return destination.Sync()
}
