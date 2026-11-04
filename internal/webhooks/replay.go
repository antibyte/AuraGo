package webhooks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"aurago/internal/config"
	"github.com/gofrs/flock"
)

const maxReplayClaims = 10000

var webhookNoncePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

func replaySignatureBody(r *http.Request, body []byte, now time.Time) ([]byte, string, error) {
	timestamp, nonce := r.Header.Get("X-AuraGo-Webhook-Timestamp"), r.Header.Get("X-AuraGo-Webhook-Nonce")
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || strconv.FormatInt(seconds, 10) != timestamp || seconds < now.Unix()-300 || seconds > now.Unix()+300 ||
		len(r.Header.Values("X-AuraGo-Webhook-Timestamp")) != 1 || len(r.Header.Values("X-AuraGo-Webhook-Nonce")) != 1 || !webhookNoncePattern.MatchString(nonce) {
		return nil, "", fmt.Errorf("invalid webhook timestamp or nonce")
	}
	return append([]byte(timestamp+"."+nonce+"."), body...), nonce, nil
}

type replayState struct {
	Version int              `json:"version"`
	Claims  map[string]int64 `json:"claims"`
}

func (m *Manager) replayPath() string { return m.filePath + ".replay.json" }

// Claims serialize across manager instances and survive restarts. Only digests
// and expiry times are persisted; no payload, raw nonce or credential is stored.
func (m *Manager) claimNonce(ctx context.Context, webhookID, nonce string, now time.Time, admit func() bool) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	path := m.replayPath()
	lock := flock.New(path+".lock", flock.SetPermissions(0600))
	defer lock.Close()
	locked, err := lock.TryLockContext(ctx, 10*time.Millisecond)
	if err != nil || !locked {
		return false, fmt.Errorf("webhook replay store lock unavailable")
	}
	state := replayState{Version: 1, Claims: map[string]int64{}}
	if info, err := os.Lstat(path); err == nil && (!info.Mode().IsRegular() || info.Size() > 2<<20) {
		return false, fmt.Errorf("webhook replay store is not a bounded regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("inspect webhook replay store: %w", err)
	}
	f, err := os.Open(path)
	if err == nil {
		data, readErr := io.ReadAll(io.LimitReader(f, (2<<20)+1))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil || len(data) > 2<<20 || json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Claims == nil {
			return false, fmt.Errorf("webhook replay store is invalid")
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("webhook replay store unavailable: %w", err)
	}
	for key, expiry := range state.Claims {
		decoded, err := hex.DecodeString(key)
		if err != nil || len(decoded) != sha256.Size || expiry <= 0 {
			return false, fmt.Errorf("webhook replay claim is invalid")
		}
		// Retain the expiry second to cover the inclusive signature freshness window.
		if expiry < now.Unix() {
			delete(state.Claims, key)
		}
	}
	hash := sha256.Sum256([]byte(webhookID + "\x00" + nonce))
	key := hex.EncodeToString(hash[:])
	if _, exists := state.Claims[key]; exists {
		return true, nil
	}
	if len(state.Claims) >= maxReplayClaims {
		return false, fmt.Errorf("webhook replay store is full")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if admit != nil && !admit() {
		return false, fmt.Errorf("webhook delivery queue full")
	}
	state.Claims[key] = now.Add(10 * time.Minute).Unix()
	data, err := json.Marshal(state)
	if err != nil {
		return false, err
	}
	if err := config.WriteFileAtomic(path, data, 0600); err != nil {
		return false, fmt.Errorf("persist webhook replay claim: %w", err)
	}
	return false, nil
}
