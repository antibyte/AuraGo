package push

import (
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"aurago/internal/dbutil"
	"aurago/internal/security"

	"github.com/SherClockHolmes/webpush-go"
	_ "modernc.org/sqlite"
)

var GlobalManager *Manager

type Manager struct {
	db        *sql.DB
	vault     *security.Vault
	logger    *slog.Logger
	publicKey string
	mu        sync.RWMutex
}

// PushSubscription represents a Web Push subscription object sent by the browser.
type PushSubscription struct {
	Endpoint string            `json:"endpoint"`
	Keys     map[string]string `json:"keys"` // p256dh and auth
}

func NewManager(dataDir string, vault *security.Vault, logger *slog.Logger) (*Manager, error) {
	dbPath := filepath.Join(dataDir, "push.db")
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open push db: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS subscriptions (
		endpoint TEXT PRIMARY KEY,
		auth_key TEXT,
		p256dh_key TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to create push schema: %w", err)
	}

	m := &Manager{
		db:     db,
		vault:  vault,
		logger: logger,
	}

	if err := m.initKeys(); err != nil {
		return nil, err
	}

	GlobalManager = m
	return m, nil
}

func (m *Manager) initKeys() error {
	privKeyURL, errPriv := m.vault.ReadSecret("vapid_private_key")
	pubKeyURL, errPub := m.vault.ReadSecret("vapid_public_key")

	if errPriv != nil || errPub != nil || privKeyURL == "" || pubKeyURL == "" {
		m.logger.Info("Generating new VAPID keys for Web Push")
		privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			return fmt.Errorf("failed to generate VAPID keys: %w", err)
		}

		if err := m.vault.WriteSecret("vapid_private_key", privateKey); err != nil {
			return fmt.Errorf("failed to store vapid_private_key: %w", err)
		}
		if err := m.vault.WriteSecret("vapid_public_key", publicKey); err != nil {
			return fmt.Errorf("failed to store vapid_public_key: %w", err)
		}

		privKeyURL = privateKey
		pubKeyURL = publicKey
	}

	m.publicKey = pubKeyURL
	return nil
}

func (m *Manager) GetPublicKey() string {
	return m.publicKey
}

func (m *Manager) Subscribe(sub PushSubscription) error {
	if sub.Endpoint == "" || sub.Keys["auth"] == "" || sub.Keys["p256dh"] == "" {
		return fmt.Errorf("invalid subscription data")
	}

	stmt := `
	INSERT INTO subscriptions (endpoint, auth_key, p256dh_key) 
	VALUES (?, ?, ?)
	ON CONFLICT(endpoint) DO UPDATE SET 
		auth_key = excluded.auth_key,
		p256dh_key = excluded.p256dh_key,
		last_used = CURRENT_TIMESTAMP;`

	_, err := m.db.Exec(stmt, sub.Endpoint, sub.Keys["auth"], sub.Keys["p256dh"])
	if err != nil {
		return fmt.Errorf("failed to save subscription: %w", err)
	}

	m.logger.Info("New Web Push subscription added", "endpoint", sub.Endpoint)
	return nil
}

// SendPush sends a payload to all active subscribers. Returns the number of successful pushes.
func (m *Manager) SendPush(payload []byte) (int, error) {
	privKey, err := m.vault.ReadSecret("vapid_private_key")
	if err != nil {
		return 0, fmt.Errorf("failed to read vapid private key: %w", err)
	}

	rows, err := m.db.Query("SELECT endpoint, auth_key, p256dh_key FROM subscriptions")
	if err != nil {
		return 0, fmt.Errorf("failed to query subscriptions: %w", err)
	}
	defer rows.Close()

	successCount := 0
	var toDelete []string

	for rows.Next() {
		var endpoint, auth, p256dh string
		if err := rows.Scan(&endpoint, &auth, &p256dh); err != nil {
			continue
		}

		sub := &webpush.Subscription{
			Endpoint: endpoint,
			Keys: webpush.Keys{
				Auth:   auth,
				P256dh: p256dh,
			},
		}

		// Send notification
		res, err := webpush.SendNotification(payload, sub, &webpush.Options{
			Subscriber:      "mailto:aurago@localhost", // Replace with an email for the VAPID claim if needed, but this works
			VAPIDPublicKey:  m.publicKey,
			VAPIDPrivateKey: privKey,
			TTL:             30,
		})

		if err != nil || (res != nil && (res.StatusCode == 410 || res.StatusCode == 404)) {
			// Subscription expired or invalid tracking
			m.logger.Warn("Failed to send push, removing subscription", "endpoint", endpoint[:30]+"...", "error", err)
			toDelete = append(toDelete, endpoint)
		} else {
			successCount++
			if res != nil {
				res.Body.Close()
			}
		}
	}

	// Clean up dead subscriptions
	if len(toDelete) > 0 {
		m.mu.Lock()
		for _, endpoint := range toDelete {
			m.db.Exec("DELETE FROM subscriptions WHERE endpoint = ?", endpoint)
		}
		m.mu.Unlock()
	}

	return successCount, nil
}

func (m *Manager) Close() error {
	return m.db.Close()
}

// CountSubscriptions returns the number of active push subscriptions.
func (m *Manager) CountSubscriptions() int {
	var count int
	m.db.QueryRow("SELECT COUNT(*) FROM subscriptions").Scan(&count)
	return count
}

// Unsubscribe removes a push subscription by endpoint URL.
func (m *Manager) Unsubscribe(endpoint string) error {
	_, err := m.db.Exec("DELETE FROM subscriptions WHERE endpoint = ?", endpoint)
	return err
}
