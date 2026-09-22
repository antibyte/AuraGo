package mqtt

import (
	"fmt"
	"log/slog"
	"sort"

	"aurago/internal/config"
)

func missionTriggerChanged() {
	defaultControllerMu.RLock()
	controller := defaultController
	defaultControllerMu.RUnlock()
	if controller != nil {
		controller.signalReconcile()
	}
}

// Subscription owner kinds are deliberately stable: they are also used by
// the bridge detail API and status JSON.
const (
	SubscriptionOwnerConfig  = "config"
	SubscriptionOwnerFrigate = "frigate"
	SubscriptionOwnerManual  = "manual"
	SubscriptionOwnerMission = "mission"
)

// SubscriptionOwner identifies one independent reason for retaining a broker
// filter. A filter remains subscribed until its final owner is removed.
type SubscriptionOwner struct {
	Kind string `json:"kind"`
	Key  string `json:"key,omitempty"`
	QoS  byte   `json:"qos"`
}

// MQTTSubscriptionStatus describes one desired filter and its latest broker
// acknowledgement. Pending/failed filters remain desired and are retried on
// a later connection or reconciliation.
type MQTTSubscriptionStatus struct {
	Filter     string              `json:"filter"`
	Owners     []SubscriptionOwner `json:"owners,omitempty"`
	QoS        byte                `json:"qos"`
	GrantedQoS byte                `json:"granted_qos,omitempty"`
	Pending    bool                `json:"pending"`
	Failed     bool                `json:"failed"`
	LastError  string              `json:"last_error,omitempty"`
	Attempts   uint64              `json:"attempts,omitempty"`
}

func subscriptionOwnerID(owner SubscriptionOwner) string {
	return owner.Kind + "\x00" + owner.Key
}

func addSubscriptionOwner(owners map[string][]SubscriptionOwner, filter string, owner SubscriptionOwner) {
	if filter == "" {
		return
	}
	for _, existing := range owners[filter] {
		if subscriptionOwnerID(existing) == subscriptionOwnerID(owner) {
			return
		}
	}
	owners[filter] = append(owners[filter], owner)
}

func mergeSubscriptionOwners(owners map[string][]SubscriptionOwner) map[string]byte {
	filters := make(map[string]byte, len(owners))
	for filter, entries := range owners {
		var qos byte
		for _, owner := range entries {
			if owner.QoS > qos {
				qos = owner.QoS
			}
		}
		filters[filter] = qos
	}
	return filters
}

func cloneSubscriptionOwners(src map[string][]SubscriptionOwner) map[string][]SubscriptionOwner {
	dst := make(map[string][]SubscriptionOwner, len(src))
	for filter, owners := range src {
		dst[filter] = append([]SubscriptionOwner(nil), owners...)
	}
	return dst
}

func sortedSubscriptionFilters(filters map[string]byte) []string {
	result := make([]string, 0, len(filters))
	for filter := range filters {
		result = append(result, filter)
	}
	sort.Strings(result)
	return result
}

func subscriptionOwnersForConfig(cfg *config.Config, manual map[string]byte) map[string][]SubscriptionOwner {
	owners := make(map[string][]SubscriptionOwner)
	if cfg == nil {
		return owners
	}
	qos := mqttQoS(cfg.MQTT.QoS, 0)
	for _, filter := range cfg.MQTT.Topics {
		if err := validateTopicFilter(filter); err == nil {
			addSubscriptionOwner(owners, filter, SubscriptionOwner{Kind: SubscriptionOwnerConfig, Key: filter, QoS: qos})
		}
	}
	for _, filter := range FrigateRelayTopics(cfg) {
		if err := validateTopicFilter(filter); err == nil {
			addSubscriptionOwner(owners, filter, SubscriptionOwner{Kind: SubscriptionOwnerFrigate, Key: filter, QoS: qos})
		}
	}
	for filter, requestedQoS := range manual {
		if err := validateTopicFilter(filter); err == nil {
			addSubscriptionOwner(owners, filter, SubscriptionOwner{Kind: SubscriptionOwnerManual, Key: filter, QoS: requestedQoS})
		}
	}
	missionTriggerMu.RLock()
	for _, trigger := range missionTriggers {
		if err := validateTopicFilter(trigger.topicFilter); err != nil {
			continue
		}
		key := trigger.key
		if key == "" {
			key = fmt.Sprintf("%d", trigger.id)
		}
		addSubscriptionOwner(owners, trigger.topicFilter, SubscriptionOwner{Kind: SubscriptionOwnerMission, Key: key, QoS: qos})
	}
	missionTriggerMu.RUnlock()
	for filter := range owners {
		sort.Slice(owners[filter], func(i, j int) bool {
			left, right := owners[filter][i], owners[filter][j]
			if left.Kind != right.Kind {
				return left.Kind < right.Kind
			}
			return left.Key < right.Key
		})
	}
	return owners
}

func subscriptionStatuses(owners map[string][]SubscriptionOwner, filters map[string]byte) map[string]MQTTSubscriptionStatus {
	result := make(map[string]MQTTSubscriptionStatus, len(filters))
	for filter, qos := range filters {
		result[filter] = MQTTSubscriptionStatus{
			Filter:  filter,
			Owners:  append([]SubscriptionOwner(nil), owners[filter]...),
			QoS:     qos,
			Pending: true,
		}
	}
	return result
}

func cloneSubscriptionStatuses(src map[string]MQTTSubscriptionStatus) []MQTTSubscriptionStatus {
	if len(src) == 0 {
		return nil
	}
	keys := make([]string, 0, len(src))
	for filter := range src {
		keys = append(keys, filter)
	}
	sort.Strings(keys)
	result := make([]MQTTSubscriptionStatus, 0, len(keys))
	for _, filter := range keys {
		status := src[filter]
		status.Owners = append([]SubscriptionOwner(nil), status.Owners...)
		result = append(result, status)
	}
	return result
}

// SubscriptionStatuses returns the latest desired/acknowledged filter state
// for the active generation.
func (c *MQTTController) SubscriptionStatuses() []MQTTSubscriptionStatus {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	gen := c.current
	c.mu.RUnlock()
	if gen == nil {
		return nil
	}
	return subscriptionStatusSlice(gen)
}

// UnsubscribeDetail removes the manual owner and reports any owners that keep
// the shared filter active.
func UnsubscribeDetail(topic string, log *slog.Logger) (MQTTUnsubscribeResult, error) {
	return defaultControllerInstance().unsubscribeDetail(topic, log)
}
