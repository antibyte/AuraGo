package webhooks

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)

// Manager provides CRUD for webhook configurations.
type Manager struct {
	mu       sync.RWMutex
	filePath string
	webhooks []Webhook
	log      *Log
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
		m.webhooks = []Webhook{}
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
	if len(data) == 0 {
		m.webhooks = []Webhook{}
		return nil
	}
	return json.Unmarshal(data, &m.webhooks)
}

func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.webhooks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0644)
}

// List returns all webhooks.
func (m *Manager) List() []Webhook {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Webhook, len(m.webhooks))
	copy(result, m.webhooks)
	return result
}

// Get returns a webhook by ID.
func (m *Manager) Get(id string) (Webhook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, w := range m.webhooks {
		if w.ID == id {
			return w, nil
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
			return w, nil
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

	w.ID = uuid.New().String()
	w.CreatedAt = time.Now().UTC()
	w.FireCount = 0

	if w.Delivery.Mode == "" {
		w.Delivery.Mode = DeliveryModeMessage
	}
	if w.Delivery.Priority == "" {
		w.Delivery.Priority = "queue"
	}
	if w.Delivery.PromptTemplate == "" {
		w.Delivery.PromptTemplate = DefaultPromptTemplate
	}
	if len(w.Format.AcceptedContentTypes) == 0 {
		w.Format.AcceptedContentTypes = []string{"application/json"}
	}

	m.webhooks = append(m.webhooks, w)
	if err := m.save(); err != nil {
		m.webhooks = m.webhooks[:len(m.webhooks)-1]
		return Webhook{}, err
	}
	return w, nil
}

// Update modifies an existing webhook.
func (m *Manager) Update(id string, patch Webhook) (Webhook, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.webhooks {
		if m.webhooks[i].ID != id {
			continue
		}
		if patch.Name != "" {
			m.webhooks[i].Name = patch.Name
		}
		// Allow toggling enabled
		m.webhooks[i].Enabled = patch.Enabled

		if patch.Slug != "" && patch.Slug != m.webhooks[i].Slug {
			slug := strings.ToLower(strings.TrimSpace(patch.Slug))
			if !slugRegex.MatchString(slug) {
				return Webhook{}, fmt.Errorf("invalid slug")
			}
			for _, existing := range m.webhooks {
				if existing.ID != id && existing.Slug == slug {
					return Webhook{}, fmt.Errorf("slug '%s' already in use", slug)
				}
			}
			m.webhooks[i].Slug = slug
		}
		if patch.TokenID != "" {
			m.webhooks[i].TokenID = patch.TokenID
		}

		// Update format if provided
		if len(patch.Format.AcceptedContentTypes) > 0 {
			m.webhooks[i].Format.AcceptedContentTypes = patch.Format.AcceptedContentTypes
		}
		if patch.Format.Fields != nil {
			m.webhooks[i].Format.Fields = patch.Format.Fields
		}
		if patch.Format.Description != "" {
			m.webhooks[i].Format.Description = patch.Format.Description
		}
		m.webhooks[i].Format.SignatureHeader = patch.Format.SignatureHeader
		m.webhooks[i].Format.SignatureAlgo = patch.Format.SignatureAlgo
		m.webhooks[i].Format.SignatureSecret = patch.Format.SignatureSecret

		// Update delivery
		if patch.Delivery.Mode != "" {
			m.webhooks[i].Delivery.Mode = patch.Delivery.Mode
		}
		if patch.Delivery.PromptTemplate != "" {
			m.webhooks[i].Delivery.PromptTemplate = patch.Delivery.PromptTemplate
		}
		if patch.Delivery.Priority != "" {
			m.webhooks[i].Delivery.Priority = patch.Delivery.Priority
		}

		if err := m.save(); err != nil {
			return Webhook{}, err
		}
		return m.webhooks[i], nil
	}
	return Webhook{}, fmt.Errorf("webhook not found")
}

// Delete removes a webhook by ID.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.webhooks {
		if m.webhooks[i].ID == id {
			m.webhooks = append(m.webhooks[:i], m.webhooks[i+1:]...)
			return m.save()
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
			m.webhooks[i].FireCount++
			m.webhooks[i].LastFiredAt = &now
			_ = m.save()
			return
		}
	}
}

// Log returns the webhook event log.
func (m *Manager) GetLog() *Log {
	return m.log
}
