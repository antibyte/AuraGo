package mqtt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aurago/internal/config"
)

const manualLedgerVersion = 2

func (c *MQTTController) recordLedgerError(err error) {
	if err == nil {
		return
	}
	recordError(err)
	c.mu.Lock()
	c.ledgerError = err.Error()
	c.mu.Unlock()
}

// subscriptionLedgerEntry is deliberately limited to filter ownership and
// acknowledgement state. Broker URLs are represented only by the hashed file
// name; credentials, payloads and raw URL userinfo never enter the ledger.
type subscriptionLedgerEntry struct {
	Filter             string              `json:"filter"`
	Owners             []SubscriptionOwner `json:"owners,omitempty"`
	QoS                byte                `json:"qos"`
	GrantedQoS         byte                `json:"granted_qos,omitempty"`
	ConfirmedManualQoS *byte               `json:"confirmed_manual_qos,omitempty"`
	State              string              `json:"state"`
	Pending            bool                `json:"pending"`
	Failed             bool                `json:"failed"`
	// LastError stays runtime-only; broker errors may contain untrusted URL
	// text and must never be persisted in the ledger.
	LastError string    `json:"-"`
	Attempts  uint64    `json:"attempts,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type subscriptionLedger struct {
	Version int                       `json:"version"`
	Entries []subscriptionLedgerEntry `json:"entries"`
}

func mqttLedgerClientID(cfg *config.Config) string {
	if cfg == nil || cfg.MQTT.ClientID == "" {
		return "aurago"
	}
	return cfg.MQTT.ClientID
}

// ManualSubscriptionLedgerPath returns the broker/client-scoped ledger path.
// The identity is hashed so broker URLs containing userinfo never appear on
// disk. The ledger itself contains only ownership metadata.
func ManualSubscriptionLedgerPath(cfg *config.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.Directories.DataDir) == "" || cfg.MQTT.Broker == "" {
		return ""
	}
	identity := cfg.MQTT.Broker + "\x00" + mqttLedgerClientID(cfg)
	hash := sha256.Sum256([]byte(identity))
	return filepath.Join(cfg.Directories.DataDir, "mqtt", "subscriptions-"+hex.EncodeToString(hash[:])+".json")
}

func (c *MQTTController) manualSnapshot() map[string]byte {
	c.manualMu.RLock()
	defer c.manualMu.RUnlock()
	result := make(map[string]byte, len(c.manualTopics))
	for topic, qos := range c.manualTopics {
		result[topic] = qos
	}
	return result
}

func (c *MQTTController) manualTopic(topic string) (byte, bool) {
	c.manualMu.RLock()
	defer c.manualMu.RUnlock()
	qos, ok := c.manualTopics[topic]
	return qos, ok
}

func (c *MQTTController) restoreManualTopic(topic string, qos byte, present bool) {
	if present {
		c.setManualTopic(topic, qos)
		return
	}
	c.removeManualTopic(topic)
}

func (c *MQTTController) restoreManualSubscription(topic string, qos byte, present bool) {
	c.restoreManualTopic(topic, qos, present)
	if present {
		rememberRuntimeSubscription(topic, qos)
	} else {
		forgetRuntimeSubscription(topic)
	}
}

func (c *MQTTController) setManualTopic(topic string, qos byte) {
	c.manualMu.Lock()
	if c.manualTopics == nil {
		c.manualTopics = make(map[string]byte)
	}
	c.manualTopics[topic] = qos
	c.manualMu.Unlock()
}

func (c *MQTTController) removeManualTopic(topic string) {
	c.manualMu.Lock()
	delete(c.manualTopics, topic)
	c.manualMu.Unlock()
}

func (c *MQTTController) loadManualLedger(cfg *config.Config) {
	if cfg == nil {
		return
	}
	identity := cfg.MQTT.Broker + "\x00" + mqttLedgerClientID(cfg)
	c.ledgerMu.Lock()
	if c.ledgerIdentity != identity {
		c.ledgerIdentity = identity
		c.ledger = make(map[string]subscriptionLedgerEntry)
		c.manualMu.Lock()
		c.manualTopics = make(map[string]byte)
		c.manualMu.Unlock()
	}
	c.ledgerMu.Unlock()
	if mqttCleanSession(cfg) {
		return
	}
	path := ManualSubscriptionLedgerPath(cfg)
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			c.recordLedgerError(fmt.Errorf("read MQTT subscription ledger: %w", err))
		}
		return
	}
	var ledger subscriptionLedger
	if err := json.Unmarshal(data, &ledger); err != nil || ledger.Version != manualLedgerVersion {
		c.recordLedgerError(fmt.Errorf("read MQTT subscription ledger: invalid ledger"))
		return
	}
	loaded := make(map[string]byte)
	cleaned := false
	known := make(map[string]subscriptionLedgerEntry)
	for _, entry := range ledger.Entries {
		if err := validateTopicFilter(entry.Filter); err != nil || entry.QoS > 2 || entry.State == "removed" {
			cleaned = true
			continue
		}
		known[entry.Filter] = entry
		for _, owner := range entry.Owners {
			if owner.Kind == SubscriptionOwnerManual && entry.State == "active" && !entry.Pending && !entry.Failed {
				if owner.QoS > 2 {
					cleaned = true
					continue
				}
				loaded[entry.Filter] = owner.QoS
			}
		}
		if entry.ConfirmedManualQoS != nil && *entry.ConfirmedManualQoS <= 2 {
			loaded[entry.Filter] = *entry.ConfirmedManualQoS
		}
	}
	c.manualMu.Lock()
	if c.manualTopics == nil {
		c.manualTopics = make(map[string]byte)
	}
	for topic, qos := range loaded {
		c.manualTopics[topic] = qos
	}
	c.manualMu.Unlock()
	c.ledgerMu.Lock()
	if c.ledger == nil {
		c.ledger = make(map[string]subscriptionLedgerEntry)
	}
	for filter, entry := range known {
		c.ledger[filter] = entry
	}
	c.ledgerMu.Unlock()
	if cleaned {
		_ = c.persistManualLedger(cfg)
	}
}

// persistManualLedger writes the complete known ownership state for the
// current generation. Generation reconciliation writes the richer status
// ledger before and after broker mutations.
func (c *MQTTController) persistManualLedger(cfg *config.Config) error {
	if cfg == nil || mqttCleanSession(cfg) {
		return nil
	}
	return c.writeSubscriptionLedger(cfg, c.manualLedgerEntries(cfg))
}

func (c *MQTTController) manualLedgerEntries(cfg *config.Config) []subscriptionLedgerEntry {
	manual := c.manualSnapshot()
	owners := subscriptionOwnersForConfig(cfg, manual)
	filters := mergeSubscriptionOwners(owners)
	c.ledgerMu.RLock()
	known := make(map[string]subscriptionLedgerEntry, len(c.ledger))
	for filter, entry := range c.ledger {
		known[filter] = entry
	}
	c.ledgerMu.RUnlock()
	for filter, qos := range filters {
		entry, exists := known[filter]
		entry.Filter = filter
		entry.Owners = append([]SubscriptionOwner(nil), owners[filter]...)
		entry.QoS = qos
		if !exists {
			entry.State = "pending"
			entry.Pending = true
			entry.Failed = false
			entry.GrantedQoS = 0
			entry.LastError = ""
			entry.Attempts = 0
		}
		entry.UpdatedAt = time.Now().UTC()
		known[filter] = entry
	}
	entries := make([]subscriptionLedgerEntry, 0, len(known))
	for _, entry := range known {
		entries = append(entries, entry)
	}
	return entries
}

// persistManualMutationLedger records a mutation intent while retaining all
// unrelated confirmed and stale entries from the previous ledger.
func (c *MQTTController) persistManualMutationLedger(cfg *config.Config, topic string) error {
	if cfg == nil || mqttCleanSession(cfg) {
		return nil
	}
	c.ledgerMu.RLock()
	previous, hadPrevious := c.ledger[topic]
	c.ledgerMu.RUnlock()
	entries := c.manualLedgerEntries(cfg)
	byFilter := make(map[string]subscriptionLedgerEntry, len(entries)+1)
	for _, entry := range entries {
		byFilter[entry.Filter] = entry
	}
	entry, ok := byFilter[topic]
	if !ok {
		entry = subscriptionLedgerEntry{Filter: topic, QoS: 0}
	}
	if hadPrevious && entry.ConfirmedManualQoS == nil && previous.State == "active" && !previous.Pending && !previous.Failed {
		for _, owner := range previous.Owners {
			if owner.Kind == SubscriptionOwnerManual && owner.QoS <= 2 {
				qos := owner.QoS
				entry.ConfirmedManualQoS = &qos
				break
			}
		}
	}
	entry.State = "pending"
	entry.Pending = true
	entry.Failed = false
	entry.LastError = "pending manual mutation"
	entry.UpdatedAt = time.Now().UTC()
	byFilter[topic] = entry
	entries = entries[:0]
	for _, entry := range byFilter {
		entries = append(entries, entry)
	}
	return c.writeSubscriptionLedger(cfg, entries)
}

// persistConfirmedManualLedger is used by the legacy direct-client path,
// where no generation has status maps to persist after a successful ACK.
func (c *MQTTController) persistConfirmedManualLedger(cfg *config.Config) error {
	if cfg == nil || mqttCleanSession(cfg) {
		return nil
	}
	manual := c.manualSnapshot()
	owners := subscriptionOwnersForConfig(cfg, manual)
	filters := mergeSubscriptionOwners(owners)
	entries := make([]subscriptionLedgerEntry, 0, len(filters))
	for filter, qos := range filters {
		var confirmedManualQoS *byte
		for _, owner := range owners[filter] {
			if owner.Kind == SubscriptionOwnerManual {
				ownerQoS := owner.QoS
				confirmedManualQoS = &ownerQoS
				break
			}
		}
		entries = append(entries, subscriptionLedgerEntry{
			Filter: filter, Owners: append([]SubscriptionOwner(nil), owners[filter]...),
			QoS: qos, GrantedQoS: qos, ConfirmedManualQoS: confirmedManualQoS,
			State: "active", UpdatedAt: time.Now().UTC(),
		})
	}
	return c.writeSubscriptionLedger(cfg, entries)
}

func (c *MQTTController) writeSubscriptionLedger(cfg *config.Config, entries []subscriptionLedgerEntry) error {
	path := ManualSubscriptionLedgerPath(cfg)
	if path == "" {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Filter < entries[j].Filter })
	data, err := json.MarshalIndent(subscriptionLedger{Version: manualLedgerVersion, Entries: entries}, "", "  ")
	if err != nil {
		err = fmt.Errorf("encode MQTT subscription ledger: %w", err)
		c.recordLedgerError(err)
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		err = fmt.Errorf("create MQTT subscription ledger directory: %w", err)
		c.recordLedgerError(err)
		return err
	}
	if err := config.WriteFileAtomic(path, data, 0o600); err != nil {
		err = fmt.Errorf("write MQTT subscription ledger: %w", err)
		c.recordLedgerError(err)
		return err
	}
	c.ledgerMu.Lock()
	c.ledger = make(map[string]subscriptionLedgerEntry, len(entries))
	for _, entry := range entries {
		c.ledger[entry.Filter] = entry
	}
	c.ledgerMu.Unlock()
	return nil
}

func (c *MQTTController) generationLedgerEntries(cfg *config.Config, gen *mqttGeneration) []subscriptionLedgerEntry {
	if cfg == nil || mqttCleanSession(cfg) || gen == nil {
		return nil
	}
	entries := make([]subscriptionLedgerEntry, 0, len(gen.subscriptions)+len(gen.grantedFilters))
	seen := make(map[string]struct{})
	for filter, status := range gen.subscriptions {
		var confirmedManualQoS *byte
		c.ledgerMu.RLock()
		previous, hadPrevious := c.ledger[filter]
		c.ledgerMu.RUnlock()
		if !status.Pending && !status.Failed {
			for _, owner := range status.Owners {
				if owner.Kind == SubscriptionOwnerManual {
					ownerQoS := owner.QoS
					confirmedManualQoS = &ownerQoS
					break
				}
			}
		}
		if confirmedManualQoS == nil && (status.Pending || status.Failed) && hadPrevious && previous.ConfirmedManualQoS != nil {
			qos := *previous.ConfirmedManualQoS
			confirmedManualQoS = &qos
		}
		state := "pending"
		if status.Failed {
			state = "failed"
		} else if !status.Pending {
			state = "active"
		}
		entries = append(entries, subscriptionLedgerEntry{
			Filter: status.Filter, Owners: append([]SubscriptionOwner(nil), status.Owners...),
			QoS: status.QoS, GrantedQoS: status.GrantedQoS, ConfirmedManualQoS: confirmedManualQoS, State: state,
			Pending: status.Pending, Failed: status.Failed, LastError: status.LastError,
			Attempts: status.Attempts, UpdatedAt: time.Now().UTC(),
		})
		seen[filter] = struct{}{}
	}
	for filter, granted := range gen.grantedFilters {
		if _, ok := seen[filter]; ok {
			continue
		}
		entries = append(entries, subscriptionLedgerEntry{
			Filter: filter, GrantedQoS: granted, QoS: gen.grantedDesired[filter],
			State: "pending", Pending: true, UpdatedAt: time.Now().UTC(),
		})
	}
	return entries
}

func (c *MQTTController) persistGenerationLedger(cfg *config.Config, gen *mqttGeneration) error {
	if gen == nil {
		return nil
	}
	gen.subscriptionMu.Lock()
	entries := c.generationLedgerEntries(cfg, gen)
	gen.subscriptionMu.Unlock()
	if entries == nil {
		return nil
	}
	return c.writeSubscriptionLedger(cfg, entries)
}
