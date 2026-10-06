package webhooks

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/uid"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)

// Manager provides CRUD for webhook configurations.
type Manager struct {
	mu              sync.RWMutex
	filePath        string
	webhooks        []Webhook
	log             *Log
	missionTriggers map[string]*missionTrigger // registration key → callback
}

type missionTrigger struct {
	webhookID string
	callback  func([]byte)
}

func cloneWebhook(w Webhook) Webhook {
	w.Format.AcceptedContentTypes = slices.Clone(w.Format.AcceptedContentTypes)
	w.Format.Fields = slices.Clone(w.Format.Fields)
	if w.LastFiredAt != nil {
		value := *w.LastFiredAt
		w.LastFiredAt = &value
	}
	return w
}

// MigrateSignatureSecrets atomically moves legacy plaintext signature secrets into the vault.
func (m *Manager) MigrateSignatureSecrets(vault *security.Vault) error {
	if vault == nil {
		return fmt.Errorf("vault is required to migrate webhook signature secrets")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	next := append([]Webhook(nil), m.webhooks...)
	type vaultSnapshot struct {
		key    string
		value  string
		exists bool
	}
	snapshots := make([]vaultSnapshot, 0)
	rollback := func() {
		for i := len(snapshots) - 1; i >= 0; i-- {
			if snapshots[i].exists {
				_ = vault.WriteSecret(snapshots[i].key, snapshots[i].value)
			} else {
				_ = vault.DeleteSecret(snapshots[i].key)
			}
		}
	}

	changed := false
	for i := range next {
		secret := strings.TrimSpace(next[i].Format.SignatureSecret)
		if secret == "" {
			continue
		}
		key := SignatureSecretVaultKey(next[i].ID)
		previous, err := vault.ReadSecret(key)
		if err == nil && strings.TrimSpace(previous) != "" {
			// A vault value may have been rotated after a partially completed legacy
			// migration. The vault is authoritative; only remove the stale plaintext.
			next[i].Format.SignatureSecret = ""
			changed = true
			continue
		}
		if err != nil && !isMissingVaultSecretError(err) {
			rollback()
			return fmt.Errorf("read existing signature secret for webhook %s: %w", next[i].ID, err)
		}
		snapshot := vaultSnapshot{key: key, value: previous, exists: err == nil}
		snapshots = append(snapshots, snapshot)
		if err := vault.WriteSecret(key, secret); err != nil {
			rollback()
			return fmt.Errorf("migrate signature secret for webhook %s: %w", next[i].ID, err)
		}
		next[i].Format.SignatureSecret = ""
		changed = true
	}
	if !changed {
		return nil
	}
	if err := m.saveWebhooks(next); err != nil {
		rollback()
		return fmt.Errorf("persist migrated webhook signature secrets: %w", err)
	}
	m.webhooks = next
	return nil
}

// sha1WebhookWarnings records the webhook IDs that already logged the sha1
// deprecation warning. The guard is process-wide because the agent's
// manage_webhooks tool builds a fresh Manager (and loads the file) per call.
var sha1WebhookWarnings sync.Map

// warnDeprecatedSHA1Webhooks logs once per webhook ID that sha1 signatures no
// longer authenticate a request on their own; such webhooks need their bearer
// token (the signature is still verified) or a switch to sha256.
func warnDeprecatedSHA1Webhooks(logger *slog.Logger, list []Webhook) {
	if logger == nil {
		logger = slog.Default()
	}
	for _, wh := range list {
		if strings.ToLower(strings.TrimSpace(wh.Format.SignatureAlgo)) != "sha1" {
			continue
		}
		if _, alreadyWarned := sha1WebhookWarnings.LoadOrStore(wh.ID, struct{}{}); alreadyWarned {
			continue
		}
		logger.Warn("[Webhooks] sha1 signatures are deprecated; add a bearer token or switch to sha256", "webhook", wh.ID)
	}
}

func isMissingVaultSecretError(err error) bool {
	return err != nil && strings.EqualFold(strings.TrimSpace(err.Error()), "secret not found")
}

// UpdateOptions records which zero-value fields were explicitly provided by the caller.
type UpdateOptions struct {
	EnabledSet         bool
	SignatureHeaderSet bool
	SignatureAlgoSet   bool
	SignatureSecretSet bool
}

// NewManager creates a Manager and loads existing webhooks from disk.
func NewManager(filePath string, logPath string) (*Manager, error) {
	wl, err := NewLog(logPath, 100)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		filePath: filePath,
		log:      wl,
	}
	if err := m.load(); err != nil {
		return nil, fmt.Errorf("load webhooks: %w", err)
	}
	return m, nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			m.webhooks = []Webhook{}
			return nil
		}
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 || strings.TrimSpace(string(data)) == "null" {
		return fmt.Errorf("webhook configuration is empty or null")
	}
	if err := json.Unmarshal(data, &m.webhooks); err != nil {
		return err
	}
	warnDeprecatedSHA1Webhooks(slog.Default(), m.webhooks)
	return nil
}

func (m *Manager) saveWebhooks(next []Webhook) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	return config.WriteFileAtomic(m.filePath, data, 0600)
}

// List returns all webhooks.
func (m *Manager) List() []Webhook {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Webhook, len(m.webhooks))
	for i, w := range m.webhooks {
		result[i] = cloneWebhook(w)
	}
	return result
}

// Get returns a webhook by ID.
func (m *Manager) Get(id string) (Webhook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, w := range m.webhooks {
		if w.ID == id {
			return cloneWebhook(w), nil
		}
	}
	return Webhook{}, fmt.Errorf("webhook not found")
}

// GetBySlug returns a webhook by its URL slug.
func (m *Manager) GetBySlug(slug string) (Webhook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, w := range m.webhooks {
		if w.Slug == slug {
			return cloneWebhook(w), nil
		}
	}
	return Webhook{}, fmt.Errorf("webhook not found")
}

// Create adds a new webhook. Returns error if max reached or slug invalid/duplicate.
func (m *Manager) Create(w Webhook) (Webhook, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.webhooks) >= MaxWebhooks {
		return Webhook{}, fmt.Errorf("maximum of %d webhooks reached", MaxWebhooks)
	}

	w.Slug = strings.ToLower(strings.TrimSpace(w.Slug))
	if !slugRegex.MatchString(w.Slug) {
		return Webhook{}, fmt.Errorf("invalid slug: must be 3-50 lowercase alphanumeric with hyphens")
	}

	for _, existing := range m.webhooks {
		if existing.Slug == w.Slug {
			return Webhook{}, fmt.Errorf("slug '%s' already in use", w.Slug)
		}
	}

	w.ID = uid.New()
	w.CreatedAt = time.Now().UTC()
	w.FireCount = 0

	if w.Delivery.Mode == "" {
		w.Delivery.Mode = DeliveryModeMessage
	}
	// Priority is deprecated; every new delivery is asynchronous.
	w.Delivery.Priority = "queue"
	if w.Delivery.PromptTemplate == "" {
		w.Delivery.PromptTemplate = DefaultPromptTemplate
	}
	if err := ValidatePromptTemplate(w.Delivery.PromptTemplate); err != nil {
		return Webhook{}, err
	}
	if len(w.Format.AcceptedContentTypes) == 0 {
		w.Format.AcceptedContentTypes = []string{"application/json"}
	}

	next := append(slices.Clone(m.webhooks), cloneWebhook(w))
	if err := m.saveWebhooks(next); err != nil {
		return Webhook{}, err
	}
	m.webhooks = next
	return cloneWebhook(w), nil
}

// Update modifies an existing webhook.
func (m *Manager) Update(id string, patch Webhook) (Webhook, error) {
	return m.UpdateWithOptions(id, patch, UpdateOptions{
		EnabledSet:         true,
		SignatureHeaderSet: true,
		SignatureAlgoSet:   true,
		SignatureSecretSet: true,
	})
}

// UpdateWithOptions modifies an existing webhook while preserving omitted zero-value fields.
func (m *Manager) UpdateWithOptions(id string, patch Webhook, opts UpdateOptions) (Webhook, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.webhooks {
		if m.webhooks[i].ID != id {
			continue
		}
		updated := cloneWebhook(m.webhooks[i])
		if patch.Name != "" {
			updated.Name = patch.Name
		}
		if opts.EnabledSet {
			updated.Enabled = patch.Enabled
		}

		if patch.Slug != "" && patch.Slug != updated.Slug {
			slug := strings.ToLower(strings.TrimSpace(patch.Slug))
			if !slugRegex.MatchString(slug) {
				return Webhook{}, fmt.Errorf("invalid slug")
			}
			for _, existing := range m.webhooks {
				if existing.ID != id && existing.Slug == slug {
					return Webhook{}, fmt.Errorf("slug '%s' already in use", slug)
				}
			}
			updated.Slug = slug
		}
		if patch.TokenID != "" {
			updated.TokenID = patch.TokenID
		}

		// Update format if provided
		if len(patch.Format.AcceptedContentTypes) > 0 {
			updated.Format.AcceptedContentTypes = slices.Clone(patch.Format.AcceptedContentTypes)
		}
		if patch.Format.Fields != nil {
			updated.Format.Fields = slices.Clone(patch.Format.Fields)
		}
		if patch.Format.Description != "" {
			updated.Format.Description = patch.Format.Description
		}
		if opts.SignatureHeaderSet {
			updated.Format.SignatureHeader = patch.Format.SignatureHeader
		}
		if opts.SignatureAlgoSet {
			updated.Format.SignatureAlgo = patch.Format.SignatureAlgo
		}
		if opts.SignatureSecretSet {
			updated.Format.SignatureSecret = patch.Format.SignatureSecret
		}

		// Update delivery
		if patch.Delivery.Mode != "" {
			updated.Delivery.Mode = patch.Delivery.Mode
		}
		if patch.Delivery.PromptTemplate != "" {
			if err := ValidatePromptTemplate(patch.Delivery.PromptTemplate); err != nil {
				return Webhook{}, err
			}
			updated.Delivery.PromptTemplate = patch.Delivery.PromptTemplate
		}
		if patch.Delivery.Priority != "" {
			updated.Delivery.Priority = "queue"
		}

		next := slices.Clone(m.webhooks)
		next[i] = updated
		if err := m.saveWebhooks(next); err != nil {
			return Webhook{}, err
		}
		m.webhooks = next
		return cloneWebhook(updated), nil
	}
	return Webhook{}, fmt.Errorf("webhook not found")
}

// Delete removes a webhook by ID.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.webhooks {
		if m.webhooks[i].ID == id {
			next := slices.Delete(slices.Clone(m.webhooks), i, i+1)
			if err := m.saveWebhooks(next); err != nil {
				return err
			}
			m.webhooks = next
			for key, trigger := range m.missionTriggers {
				if trigger.webhookID == id {
					delete(m.missionTriggers, key)
				}
			}
			return nil
		}
	}
	return fmt.Errorf("webhook not found")
}

// RecordFire updates the webhook's fire count and last-fired timestamp.
func (m *Manager) RecordFire(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for i := range m.webhooks {
		if m.webhooks[i].ID == id {
			next := slices.Clone(m.webhooks)
			next[i].FireCount++
			next[i].LastFiredAt = &now
			if err := m.saveWebhooks(next); err != nil {
				slog.Warn("Failed to persist webhook event count", "error", err)
				return
			}
			m.webhooks = next
			return
		}
	}
}

// Log returns the webhook event log.
func (m *Manager) GetLog() *Log {
	return m.log
}

// RegisterMissionTrigger registers an independent anonymous observer.
func (m *Manager) RegisterMissionTrigger(webhookID string, callback func([]byte)) {
	m.RegisterMissionTriggerForKey(uid.New(), webhookID, callback)
}

// RegisterMissionTriggerForKey atomically replaces this mission's registration.
func (m *Manager) RegisterMissionTriggerForKey(key, webhookID string, callback func([]byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if key == "" {
		return
	}
	if m.missionTriggers == nil {
		m.missionTriggers = make(map[string]*missionTrigger)
	}
	if callback == nil || webhookID == "" {
		delete(m.missionTriggers, key)
		return
	}
	m.missionTriggers[key] = &missionTrigger{webhookID: webhookID, callback: callback}
}

func (m *Manager) UnregisterMissionTrigger(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.missionTriggers, key)
}

// NotifyWebhookFired snapshots callbacks and drops replaced registrations.
func (m *Manager) NotifyWebhookFired(webhookID string, payload []byte) {
	m.mu.RLock()
	callbacks := make(map[string]*missionTrigger)
	for key, trigger := range m.missionTriggers {
		if trigger.webhookID == webhookID {
			callbacks[key] = trigger
		}
	}
	m.mu.RUnlock()
	for key, trigger := range callbacks {
		m.mu.RLock()
		current := m.missionTriggers[key] == trigger
		m.mu.RUnlock()
		if current {
			trigger.callback(slices.Clone(payload))
		}
	}
}
