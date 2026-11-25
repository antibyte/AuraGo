package desktop

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

const (
	rdpDefaultPort     = 3389
	rdpTicketTTL       = 60 * time.Second
	rdpTicketMaxActive = 256
	rdpMaxTPKTSize     = 64 << 10

	// At most this many tickets per device per window (successful issues count).
	rdpTicketRateLimit  = 10
	rdpTicketRateWindow = time.Minute
)

// Issue errors; both map to HTTP 429.
var (
	ErrRDPTicketRateLimited = errors.New("too many RDP tickets for this device, try again in a minute")
	ErrRDPTicketCapacity    = errors.New("too many pending RDP tickets")
)

// rdpUpgrader upgrades the RDCleanPath WebSocket. Same-origin only, like VNC.
var rdpUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return sameHostWebSocketOrigin(r)
	},
}

// RDPTicketStore holds single-use, short-lived tickets that authorise one
// RDCleanPath WebSocket for one device. Safe for concurrent use.
type RDPTicketStore struct {
	mu      sync.Mutex
	tickets map[string]rdpTicket
	issued  map[string][]time.Time // successful issues per device within the rate window
	ttl     time.Duration
	max     int
	now     func() time.Time
}

type rdpTicket struct {
	deviceID string
	expires  time.Time
}

// NewRDPTicketStore creates an empty store (60 s TTL, at most 256 live tickets,
// at most 10 issues per device per minute).
func NewRDPTicketStore() *RDPTicketStore {
	return &RDPTicketStore{
		tickets: make(map[string]rdpTicket),
		issued:  make(map[string][]time.Time),
		ttl:     rdpTicketTTL,
		max:     rdpTicketMaxActive,
		now:     time.Now,
	}
}

func (s *RDPTicketStore) sweepLocked(now time.Time) {
	for key, t := range s.tickets {
		if !now.Before(t.expires) {
			delete(s.tickets, key)
		}
	}
}

// Issue creates a ticket bound to deviceID.
func (s *RDPTicketStore) Issue(deviceID string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.sweepLocked(now)
	recent := s.issued[deviceID][:0]
	for _, at := range s.issued[deviceID] {
		if now.Sub(at) < rdpTicketRateWindow {
			recent = append(recent, at)
		}
	}
	if len(recent) >= rdpTicketRateLimit {
		s.issued[deviceID] = recent
		return "", ErrRDPTicketRateLimited
	}
	if len(s.tickets) >= s.max {
		s.issued[deviceID] = recent
		return "", ErrRDPTicketCapacity
	}
	s.tickets[ticket] = rdpTicket{deviceID: deviceID, expires: now.Add(s.ttl)}
	s.issued[deviceID] = append(recent, now)
	if len(s.issued) > 4*rdpTicketMaxActive {
		for id, times := range s.issued {
			if len(times) == 0 || now.Sub(times[len(times)-1]) >= rdpTicketRateWindow {
				delete(s.issued, id)
			}
		}
	}
	return ticket, nil
}

// RetryAfter returns how long until the device may be issued a ticket again
// after ErrRDPTicketRateLimited: the time until the oldest issue in the window
// expires, at least one second.
func (s *RDPTicketStore) RetryAfter(deviceID string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	var oldest time.Time
	for _, at := range s.issued[deviceID] {
		if now.Sub(at) < rdpTicketRateWindow && (oldest.IsZero() || at.Before(oldest)) {
			oldest = at
		}
	}
	wait := time.Second
	if !oldest.IsZero() {
		if d := oldest.Add(rdpTicketRateWindow).Sub(now); d > wait {
			wait = d
		}
	}
	return wait
}

// Consume validates and removes a ticket. A ticket is single use: it is removed
// even when the device does not match.
func (s *RDPTicketStore) Consume(ticket, deviceID string) bool {
	if ticket == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[ticket]
	if !ok {
		return false
	}
	delete(s.tickets, ticket)
	return s.now().Before(t.expires) && t.deviceID == deviceID
}

type rdpTicketResponse struct {
	Ticket      string `json:"ticket"`
	Username    string `json:"username"`
	Domain      string `json:"domain"`
	Password    string `json:"password"`
	HasPassword bool   `json:"has_password"`
}

// splitRDPUsername turns "DOMAIN\user" into (domain, user). "user@domain" and
// plain names stay untouched as the username.
func splitRDPUsername(username string) (domain, user string) {
	username = strings.TrimSpace(username)
	if i := strings.Index(username, `\`); i > 0 && i < len(username)-1 {
		return username[:i], username[i+1:]
	}
	return "", username
}

// RDPAuditFunc records a desktop audit event for an RDP gateway action. details
// never contains secrets.
type RDPAuditFunc func(r *http.Request, action, deviceID string, details map[string]any)

func (f RDPAuditFunc) emit(r *http.Request, action, deviceID string, details map[string]any) {
	if f != nil {
		f(r, action, deviceID, details)
	}
}

// HandleRDPTicket issues a one-time ticket for the RDCleanPath WebSocket. The
// stored password is released to the browser only when allowSecret reports that
// the caller is strongly authenticated (nil means never); otherwise the ticket is
// still issued with an empty password so the UI prompts. audit (optional) is
// called after a successful issue.
func HandleRDPTicket(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, store *RDPTicketStore, allowSecret func(*http.Request) bool, audit RDPAuditFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
		if deviceID == "" {
			http.Error(w, "missing device_id", http.StatusBadRequest)
			return
		}
		device, err := inventory.GetDeviceByID(inventoryDB, deviceID)
		if err != nil {
			http.Error(w, "device not found", http.StatusNotFound)
			return
		}
		if device.Protocol != inventory.ProtocolRDP {
			http.Error(w, fmt.Sprintf("device protocol is %q, expected rdp", device.Protocol), http.StatusBadRequest)
			return
		}
		releaseSecret := allowSecret != nil && allowSecret(r)
		var username, password string
		if releaseSecret {
			_, _, username, password, err = resolveRDPAccess(device, inventoryDB, vault)
		} else {
			_, _, username, _, err = resolveRDPTarget(device, inventoryDB)
		}
		if err != nil {
			logger.Warn("RDP ticket: credential resolution failed", "device_id", deviceID, "error", err)
			http.Error(w, "credential unavailable", http.StatusInternalServerError)
			return
		}
		ticket, err := store.Issue(deviceID)
		if err != nil {
			logger.Warn("RDP ticket refused", "device_id", deviceID, "reason", err.Error())
			writeRDPTicketRefusal(w, store, deviceID, err)
			return
		}
		domain, user := splitRDPUsername(username)
		secretReleased := password != ""
		logger.Info("RDP ticket issued", "device_id", deviceID, "secret_released", secretReleased, "secret_allowed", releaseSecret)
		audit.emit(r, "desktop_rdp_ticket_issued", deviceID, map[string]any{
			"secret_released": secretReleased,
			"secret_allowed":  releaseSecret,
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rdpTicketResponse{
			Ticket:      ticket,
			Username:    user,
			Domain:      domain,
			Password:    password,
			HasPassword: secretReleased,
		})
	}
}

// writeRDPTicketRefusal answers a refused ticket issue with HTTP 429, a
// Retry-After header and a JSON body the UI can localise by code.
func writeRDPTicketRefusal(w http.ResponseWriter, store *RDPTicketStore, deviceID string, err error) {
	code := "rdp_ticket_capacity"
	retry := 5
	if errors.Is(err, ErrRDPTicketRateLimited) {
		code = "rdp_ticket_rate_limited"
		retry = int((store.RetryAfter(deviceID) + time.Second - 1) / time.Second)
		if retry < 1 {
			retry = 1
		}
	}
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "code": code, "retry_after": retry})
}

// rdpTSRequestInfo is the non-secret shape of a CredSSP TSRequest: which fields
// are present and how long they are, never their content.
type rdpTSRequestInfo struct {
	Version        int64
	NegoTokensLen  int
	HasNegoTokens  bool
	AuthInfoLen    int
	HasAuthInfo    bool
	PubKeyAuthLen  int
	HasPubKeyAuth  bool
	ErrorCode      uint32
	HasErrorCode   bool
	ClientNonceLen int
	HasClientNonce bool
}

// parseRDPTSRequest parses a CredSSP TSRequest (all fields EXPLICIT context
// tags 0..5). It returns false when data is not one.
func parseRDPTSRequest(data []byte) (rdpTSRequestInfo, bool) {
	var info rdpTSRequestInfo
	var seq asn1.RawValue
	if len(data) == 0 || data[0] != 0x30 {
		return info, false
	}
	if _, err := asn1.Unmarshal(data, &seq); err != nil || seq.Class != asn1.ClassUniversal || seq.Tag != asn1.TagSequence {
		return info, false
	}
	rest := seq.Bytes
	hasVersion := false
	for len(rest) > 0 {
		var field asn1.RawValue
		var err error
		rest, err = asn1.Unmarshal(rest, &field)
		if err != nil || field.Class != asn1.ClassContextSpecific {
			return rdpTSRequestInfo{}, false
		}
		octets := func() (int, bool) {
			var b []byte
			if _, err := asn1.Unmarshal(field.Bytes, &b); err != nil {
				return 0, false
			}
			return len(b), true
		}
		switch field.Tag {
		case 0:
			if _, err := asn1.Unmarshal(field.Bytes, &info.Version); err != nil {
				return rdpTSRequestInfo{}, false
			}
			hasVersion = true
		case 1:
			info.HasNegoTokens, info.NegoTokensLen = true, len(field.Bytes)
		case 2:
			n, ok := octets()
			if !ok {
				return rdpTSRequestInfo{}, false
			}
			info.HasAuthInfo, info.AuthInfoLen = true, n
		case 3:
			n, ok := octets()
			if !ok {
				return rdpTSRequestInfo{}, false
			}
			info.HasPubKeyAuth, info.PubKeyAuthLen = true, n
		case 4:
			var code int64
			if _, err := asn1.Unmarshal(field.Bytes, &code); err != nil {
				return rdpTSRequestInfo{}, false
			}
			info.HasErrorCode, info.ErrorCode = true, uint32(code)
		case 5:
			n, ok := octets()
			if !ok {
				return rdpTSRequestInfo{}, false
			}
			info.HasClientNonce, info.ClientNonceLen = true, n
		}
	}
	return info, hasVersion
}

const rdpCredSSPDiagLimit = 4

// rdpCredSSPDiag logs the first few CredSSP TSRequests of a relay (shape only,
// never bytes). Not safe for concurrent use; one instance per direction.
type rdpCredSSPDiag struct {
	logger    *slog.Logger
	deviceID  string
	direction string
	remaining int
}

func (d *rdpCredSSPDiag) observe(chunk []byte) {
	if d == nil || d.remaining <= 0 {
		return
	}
	d.remaining--
	info, ok := parseRDPTSRequest(chunk)
	if !ok {
		return
	}
	attrs := []any{
		"device_id", d.deviceID, "direction", d.direction, "version", info.Version,
		"nego_tokens", info.HasNegoTokens, "nego_tokens_len", info.NegoTokensLen,
		"auth_info", info.HasAuthInfo, "pub_key_auth", info.HasPubKeyAuth, "pub_key_auth_len", info.PubKeyAuthLen,
		"client_nonce", info.HasClientNonce, "total_len", len(chunk),
	}
	if info.HasErrorCode {
		attrs = append(attrs, "error_code", fmt.Sprintf("0x%08x", info.ErrorCode))
	}
	d.logger.Info("RDP CredSSP TSRequest", attrs...)
}

// rdpDiagWriter observes each chunk written through it (server to browser).
type rdpDiagWriter struct {
	w    io.Writer
	diag *rdpCredSSPDiag
}

func (d *rdpDiagWriter) Write(p []byte) (int, error) {
	d.diag.observe(p)
	return d.w.Write(p)
}

// HandleRDPPinReset forgets the pinned server certificate of an RDP device so
// the next connection pins the current one. Only callers accepted by allow
// (cookie session or global desktop:admin) may reset a pin.
func HandleRDPPinReset(inventoryDB *sql.DB, logger *slog.Logger, allow func(*http.Request) bool, audit RDPAuditFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
		if deviceID == "" {
			http.Error(w, "missing device_id", http.StatusBadRequest)
			return
		}
		if allow == nil || !allow(r) {
			logger.Warn("RDP pin reset refused", "device_id", deviceID, "reason", "not strongly authenticated")
			http.Error(w, "administrator session required", http.StatusForbidden)
			return
		}
		device, err := inventory.GetDeviceByID(inventoryDB, deviceID)
		if err != nil {
			http.Error(w, "device not found", http.StatusNotFound)
			return
		}
		if device.Protocol != inventory.ProtocolRDP {
			http.Error(w, fmt.Sprintf("device protocol is %q, expected rdp", device.Protocol), http.StatusBadRequest)
			return
		}
		if err := inventory.DeleteRDPCertPin(inventoryDB, deviceID); err != nil {
			logger.Warn("RDP pin reset failed", "device_id", deviceID, "error", err)
			http.Error(w, "pin reset failed", http.StatusInternalServerError)
			return
		}
		logger.Info("RDP certificate pin reset", "device_id", deviceID)
		audit.emit(r, "desktop_rdp_pin_reset", deviceID, nil)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}

// resolveRDPTarget resolves host, port and username without touching the vault.
func resolveRDPTarget(device inventory.DeviceRecord, inventoryDB *sql.DB) (host string, port int, username string, cred *credentials.Record, err error) {
	host = strings.TrimSpace(device.IPAddress)
	if host == "" {
		host = strings.TrimSpace(device.Name)
	}
	username = strings.TrimSpace(device.Username)
	port = device.Port
	if port <= 0 {
		port = rdpDefaultPort
	}
	if strings.TrimSpace(device.CredentialID) != "" {
		c, credErr := credentials.GetByID(inventoryDB, device.CredentialID)
		if credErr != nil {
			return "", 0, "", nil, fmt.Errorf("linked credential %q could not be loaded: %w", device.CredentialID, credErr)
		}
		cred = &c
		if strings.TrimSpace(c.Host) != "" {
			host = strings.TrimSpace(c.Host)
		}
		if strings.TrimSpace(c.Username) != "" {
			username = strings.TrimSpace(c.Username)
		}
	}
	if host == "" {
		return "", 0, "", nil, fmt.Errorf("device host is missing")
	}
	return host, port, username, cred, nil
}

// resolveRDPAccess resolves host, port, username and (optional) password for an
// RDP device, mirroring resolveVNCAccess.
func resolveRDPAccess(device inventory.DeviceRecord, inventoryDB *sql.DB, vault *security.Vault) (host string, port int, username, password string, err error) {
	host, port, username, cred, err := resolveRDPTarget(device, inventoryDB)
	if err != nil {
		return "", 0, "", "", err
	}
	secretID := ""
	if cred != nil {
		secretID = strings.TrimSpace(cred.PasswordVaultID)
	} else {
		secretID = strings.TrimSpace(device.VaultSecretID)
	}
	if secretID != "" {
		if vault == nil {
			return "", 0, "", "", fmt.Errorf("vault is not available")
		}
		secret, vaultErr := vault.ReadSecret(secretID)
		if vaultErr != nil {
			return "", 0, "", "", fmt.Errorf("read vault secret %q: %w", secretID, vaultErr)
		}
		password = secret
	}
	return host, port, username, password, nil
}

func sendRDPCleanPathError(conn *websocket.Conn, code, httpStatus, wsaError, tlsAlert int, x224 []byte) {
	_ = conn.SetWriteDeadline(time.Now().Add(remoteProxyWriteTimeout))
	_ = conn.WriteMessage(websocket.BinaryMessage, buildRDCleanPathErrorPDU(code, httpStatus, wsaError, tlsAlert, x224))
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
}

// readTPKT reads exactly one TPKT frame (4 byte header + payload).
func readTPKT(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	if header[0] != 3 {
		return nil, fmt.Errorf("not a TPKT frame (version %d)", header[0])
	}
	length := int(binary.BigEndian.Uint16(header[2:4]))
	if length < 7 || length > rdpMaxTPKTSize {
		return nil, fmt.Errorf("invalid TPKT length %d", length)
	}
	frame := make([]byte, length)
	copy(frame, header[:])
	if _, err := io.ReadFull(r, frame[4:]); err != nil {
		return nil, err
	}
	return frame, nil
}

func rdpDialErrorCode(err error) int {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return rdCleanPathErrDNS
	}
	return rdCleanPathErrTargetRefused
}

// rdpTLSClientConfig is the gateway's TLS config towards RDP servers. Chain
// verification is skipped because RDP hosts use self-signed certificates; the
// leaf is pinned trust-on-first-use instead. TLS is capped at 1.2: Windows RDP
// servers (HYBRID_EX) abort TLS 1.3 handshakes from Go clients right after the
// handshake ("tls: internal error", SEC_E_INVALID_TOKEN 0x80090308), while
// mstsc and FreeRDP also negotiate TLS 1.2 here.
func rdpTLSClientConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // self-signed RDP certs, pinned instead
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS12,
	}
}

// countingWriter counts bytes written through it (gateway to browser).
type countingWriter struct {
	w io.Writer
	n atomic.Int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n.Add(int64(n))
	return n, err
}

// rdpLeafFingerprint returns the lower-case hex SHA-256 of a DER certificate.
func rdpLeafFingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// HandleRDPProxy is the RDCleanPath gateway: it upgrades to WebSocket, validates
// the one-time ticket, connects to the inventory device (the client-supplied
// destination is ignored), completes X.224 + TLS, checks the trust-on-first-use
// certificate pin and then relays raw bytes.
func HandleRDPProxy(inventoryDB *sql.DB, logger *slog.Logger, store *RDPTicketStore, options ...RemoteProxyOptions) http.HandlerFunc {
	proxyOptions := normalizeRemoteProxyOptions(options...)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), proxyOptions.MaxSessionDuration)
		defer cancel()
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
		if deviceID == "" {
			logger.Warn("RDP proxy refused", "reason", "missing device_id")
			http.Error(w, "missing device_id", http.StatusBadRequest)
			return
		}
		conn, err := rdpUpgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Warn("RDP proxy WebSocket upgrade failed", "device_id", deviceID, "error", err)
			return
		}
		defer conn.Close()
		stopWS := context.AfterFunc(ctx, func() { _ = conn.Close() })
		defer stopWS()

		// fail logs once at Warn (no ticket, no secrets) and answers with an
		// RDCleanPath error PDU.
		fail := func(reason string, code, httpStatus int, x224 []byte) {
			logger.Warn("RDP proxy failed", "device_id", deviceID, "reason", reason, "rdcleanpath_code", code, "http_status", httpStatus)
			sendRDPCleanPathError(conn, code, httpStatus, 0, 0, x224)
		}

		// Step 1: the RDCleanPath request.
		conn.SetReadLimit(rdpMaxTPKTSize)
		_ = conn.SetReadDeadline(time.Now().Add(remoteProxyWriteTimeout))
		mt, data, err := conn.ReadMessage()
		if err != nil || mt != websocket.BinaryMessage {
			fail("no RDCleanPath request received", rdCleanPathErrGeneral, http.StatusBadRequest, nil)
			return
		}
		req, err := parseRDCleanPathPDU(data)
		if err != nil || len(req.X224ConnectionPDU) < 4 || req.X224ConnectionPDU[0] != 3 {
			fail("invalid RDCleanPath request", rdCleanPathErrGeneral, http.StatusBadRequest, nil)
			return
		}
		if !store.Consume(req.ProxyAuth, deviceID) {
			fail("ticket invalid, expired or for another device", rdCleanPathErrGeneral, http.StatusUnauthorized, nil)
			return
		}
		if req.PreconnectionBlob != "" {
			fail("preconnection blob not supported", rdCleanPathErrGeneral, http.StatusBadRequest, nil)
			return
		}

		device, err := inventory.GetDeviceByID(inventoryDB, deviceID)
		if err != nil {
			fail("device not found", rdCleanPathErrGeneral, http.StatusNotFound, nil)
			return
		}
		if device.Protocol != inventory.ProtocolRDP {
			fail("device is not an RDP device", rdCleanPathErrGeneral, http.StatusBadRequest, nil)
			return
		}
		host, port, _, _, err := resolveRDPTarget(device, inventoryDB)
		if err != nil {
			fail("target resolution failed: "+err.Error(), rdCleanPathErrGeneral, http.StatusInternalServerError, nil)
			return
		}

		// Step 2: TCP + X.224 negotiation. The target is the inventory device only.
		dialer := net.Dialer{Timeout: remoteProxyWriteTimeout}
		tcp, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
		if err != nil {
			fail("dial failed: "+err.Error(), rdpDialErrorCode(err), 0, nil)
			return
		}
		defer tcp.Close()
		stopTCP := context.AfterFunc(ctx, func() { _ = tcp.Close() })
		defer stopTCP()
		_ = tcp.SetDeadline(time.Now().Add(remoteProxyWriteTimeout))
		if _, err := tcp.Write(req.X224ConnectionPDU); err != nil {
			fail("X.224 request write failed", rdCleanPathErrTargetRefused, 0, nil)
			return
		}
		confirm, err := readTPKT(tcp)
		if err != nil {
			fail("X.224 confirm read failed: "+err.Error(), rdCleanPathErrTargetRefused, 0, nil)
			return
		}
		if len(confirm) < 6 || confirm[5] != 0xD0 {
			fail("unexpected X.224 reply", rdCleanPathErrGeneral, 0, nil)
			return
		}
		// RDP_NEG_FAILURE (type 0x03) follows the 11 byte TPKT + X.224 CC header.
		if len(confirm) >= 19 && confirm[11] == 0x03 {
			fail("server refused the negotiation", rdCleanPathErrNegotiation, 0, confirm)
			return
		}

		// Step 3: TLS. RDP servers present self-signed certificates by default,
		// so chain verification would reject nearly every host. Instead the leaf
		// is pinned trust-on-first-use per device, and CredSSP/NLA binds to the
		// server public key inside the client.
		tlsConn := tls.Client(tcp, rdpTLSClientConfig())
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			fail("TLS handshake failed: "+err.Error(), rdCleanPathErrTLS, 0, nil)
			return
		}
		defer tlsConn.Close()
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) == 0 {
			fail("server sent no certificate", rdCleanPathErrTLS, 0, nil)
			return
		}
		fingerprint := rdpLeafFingerprint(state.PeerCertificates[0].Raw)
		pinned, found, err := inventory.GetRDPCertPin(inventoryDB, deviceID)
		if err != nil {
			fail("certificate pin lookup failed: "+err.Error(), rdCleanPathErrGeneral, http.StatusInternalServerError, nil)
			return
		}
		switch {
		case !found:
			if err := inventory.SetRDPCertPin(inventoryDB, deviceID, fingerprint); err != nil {
				fail("certificate pin store failed: "+err.Error(), rdCleanPathErrGeneral, http.StatusInternalServerError, nil)
				return
			}
			logger.Info("RDP certificate pinned on first use", "device_id", deviceID, "sha256", fingerprint)
		case pinned != fingerprint:
			logger.Warn("RDP server certificate changed", "device_id", deviceID, "pinned_sha256", pinned, "presented_sha256", fingerprint)
			sendRDPCleanPathError(conn, rdCleanPathErrCertChanged, http.StatusConflict, 0, 0, nil)
			return
		}
		chain := make([][]byte, 0, len(state.PeerCertificates))
		for _, cert := range state.PeerCertificates {
			chain = append(chain, cert.Raw)
		}
		resp, err := rdCleanPathPDU{
			Version:           rdCleanPathVersion,
			X224ConnectionPDU: confirm,
			ServerCertChain:   chain,
			ServerAddr:        tcp.RemoteAddr().String(),
		}.marshal()
		if err != nil {
			fail("response encoding failed", rdCleanPathErrGeneral, 0, nil)
			return
		}
		writer := &wsBinaryWriter{conn: conn}
		if _, err := writer.Write(resp); err != nil {
			logger.Warn("RDP proxy failed", "device_id", deviceID, "reason", "response write failed: "+err.Error())
			return
		}

		// Step 4: raw relay.
		started := time.Now()
		toBrowser := &countingWriter{w: writer}
		var fromBrowser atomic.Int64
		var closedMu sync.Mutex
		closedBy := ""
		markClosed := func(side string) {
			closedMu.Lock()
			if closedBy == "" {
				closedBy = side
			}
			closedMu.Unlock()
		}
		serverDiag := &rdpDiagWriter{w: toBrowser, diag: &rdpCredSSPDiag{logger: logger, deviceID: deviceID, direction: "server_to_browser", remaining: rdpCredSSPDiagLimit}}
		browserDiag := &rdpCredSSPDiag{logger: logger, deviceID: deviceID, direction: "browser_to_server", remaining: rdpCredSSPDiagLimit}
		_ = tcp.SetDeadline(time.Time{})
		conn.SetReadLimit(remoteProxyReadLimit)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(serverDiag, tlsConn)
			markClosed("server")
			// The RDP server closed its side: tell the browser properly.
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "remote closed"), time.Now().Add(time.Second))
			cancel()
		}()
		go func() {
			defer wg.Done()
			defer cancel()
			for {
				_ = conn.SetReadDeadline(time.Now().Add(proxyOptions.IdleTimeout))
				mt, msg, readErr := conn.ReadMessage()
				if readErr != nil {
					markClosed("browser")
					return
				}
				if mt != websocket.BinaryMessage {
					continue
				}
				browserDiag.observe(msg)
				n, werr := tlsConn.Write(msg)
				fromBrowser.Add(int64(n))
				if werr != nil {
					markClosed("server")
					return
				}
			}
		}()
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-ctx.Done():
			markClosed("timeout")
			_ = tlsConn.Close()
			_ = conn.Close()
			<-done
		case <-done:
		}
		logger.Info("RDP session ended", "device_id", deviceID, "duration", time.Since(started).Round(time.Millisecond).String(),
			"bytes_to_browser", toBrowser.n.Load(), "bytes_from_browser", fromBrowser.Load(), "closed_by", func() string {
				closedMu.Lock()
				defer closedMu.Unlock()
				return closedBy
			}())
	}
}
