package mqtt

import (
	"context"
	"fmt"
	"time"

	"aurago/internal/config"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type subscribeResult struct {
	granted  map[string]byte
	rejected map[string]byte
	err      error
}

func desiredSubscriptionState(c *MQTTController, cfg *config.Config) (map[string][]SubscriptionOwner, map[string]byte) {
	owners := subscriptionOwnersForConfig(cfg, c.manualSnapshot())
	return owners, mergeSubscriptionOwners(owners)
}

func (c *MQTTController) initializeGenerationSubscriptions(gen *mqttGeneration, cfg *config.Config) {
	owners, filters := desiredSubscriptionState(c, cfg)
	gen.subscriptionMu.Lock()
	gen.desiredOwners = cloneSubscriptionOwners(owners)
	gen.desiredFilters = filters
	gen.grantedFilters = make(map[string]byte)
	gen.grantedDesired = make(map[string]byte)
	gen.knownFilters = make(map[string]struct{})
	gen.subscriptions = subscriptionStatuses(owners, filters)
	confirmed := make(map[string]struct{})
	c.ledgerMu.RLock()
	for filter, entry := range c.ledger {
		if !mqttCleanSession(cfg) && entry.GrantedQoS <= 2 && (entry.State == "active" || entry.Pending || entry.Failed) {
			gen.knownFilters[filter] = struct{}{}
			if entry.State == "active" && !entry.Pending && !entry.Failed {
				gen.grantedFilters[filter] = entry.GrantedQoS
				gen.grantedDesired[filter] = entry.QoS
				confirmed[filter] = struct{}{}
			} else if entry.GrantedQoS > 0 {
				gen.grantedFilters[filter] = entry.GrantedQoS
				gen.grantedDesired[filter] = entry.QoS
			}
		}
	}
	c.ledgerMu.RUnlock()
	for filter, granted := range gen.grantedFilters {
		if _, wanted := gen.desiredFilters[filter]; !wanted {
			continue
		}
		if _, ok := confirmed[filter]; !ok {
			continue
		}
		if status, ok := gen.subscriptions[filter]; ok && gen.grantedDesired[filter] == status.QoS {
			status.GrantedQoS = granted
			status.Pending = false
			status.Failed = false
			status.Attempts = 0
			gen.subscriptions[filter] = status
		}
	}
	gen.subscriptionMu.Unlock()
}

func resetGenerationAcknowledgements(gen *mqttGeneration) {
	if gen == nil {
		return
	}
	gen.subscriptionMu.Lock()
	for filter := range gen.desiredFilters {
		delete(gen.grantedFilters, filter)
		delete(gen.grantedDesired, filter)
	}
	if gen.cleanSession {
		gen.grantedFilters = make(map[string]byte)
		gen.grantedDesired = make(map[string]byte)
		gen.knownFilters = make(map[string]struct{})
	}
	for filter, status := range gen.subscriptions {
		status.Pending = true
		if gen.cleanSession {
			status.Failed = false
			status.GrantedQoS = 0
		}
		gen.subscriptions[filter] = status
	}
	gen.subscriptionMu.Unlock()
}

func (c *MQTTController) updateGenerationSubscriptions(gen *mqttGeneration, cfg *config.Config) {
	if gen == nil {
		return
	}
	owners, filters := desiredSubscriptionState(c, cfg)
	gen.subscriptionMu.Lock()
	old := gen.subscriptions
	gen.desiredOwners = cloneSubscriptionOwners(owners)
	gen.desiredFilters = filters
	newStatuses := subscriptionStatuses(owners, filters)
	for filter, status := range newStatuses {
		if previous, ok := old[filter]; ok && previous.QoS == status.QoS {
			status.GrantedQoS = previous.GrantedQoS
			status.Pending = previous.Pending
			status.Failed = previous.Failed
			status.LastError = previous.LastError
			status.Attempts = previous.Attempts
		}
		newStatuses[filter] = status
	}
	for filter, previous := range old {
		if _, wanted := newStatuses[filter]; wanted {
			continue
		}
		if previous.Pending || previous.Failed {
			newStatuses[filter] = previous
		}
	}
	gen.subscriptions = newStatuses
	gen.subscriptionMu.Unlock()
}

func (c *MQTTController) reconcileGenerationSubscriptions(gen *mqttGeneration, cfg *config.Config) {
	if gen == nil || gen.client == nil {
		return
	}
	gen.subscriptionOpMu.Lock()
	defer gen.subscriptionOpMu.Unlock()
	if !c.generationCurrent(gen) || gen.ctx.Err() != nil {
		return
	}
	gen.subscriptionMu.Lock()
	desired := make(map[string]byte, len(gen.desiredFilters))
	for filter, qos := range gen.desiredFilters {
		desired[filter] = qos
	}
	if gen.grantedFilters == nil {
		gen.grantedFilters = make(map[string]byte)
	}
	if gen.grantedDesired == nil {
		gen.grantedDesired = make(map[string]byte)
	}
	if gen.subscriptions == nil {
		gen.subscriptions = subscriptionStatuses(gen.desiredOwners, desired)
	}
	granted := make(map[string]byte, len(gen.grantedFilters))
	for filter, qos := range gen.grantedFilters {
		granted[filter] = qos
	}
	grantedDesired := make(map[string]byte, len(gen.grantedDesired))
	for filter, qos := range gen.grantedDesired {
		grantedDesired[filter] = qos
	}
	known := make(map[string]struct{}, len(gen.knownFilters))
	for filter := range gen.knownFilters {
		known[filter] = struct{}{}
	}
	statuses := make(map[string]MQTTSubscriptionStatus, len(gen.subscriptions))
	for filter, status := range gen.subscriptions {
		status.Owners = append([]SubscriptionOwner(nil), status.Owners...)
		statuses[filter] = status
	}
	gen.subscriptionMu.Unlock()
	if !gen.client.IsConnectionOpen() {
		gen.subscriptionMu.Lock()
		for filter, status := range statuses {
			if desired[filter] != status.QoS {
				continue
			}
			status.Pending = true
			statuses[filter] = status
		}
		gen.subscriptions = statuses
		gen.subscriptionMu.Unlock()
		return
	}

	removed := make([]string, 0)
	for filter := range known {
		if _, ok := desired[filter]; !ok {
			removed = append(removed, filter)
		}
	}
	for filter := range granted {
		if _, already := known[filter]; already {
			continue
		}
		if _, ok := desired[filter]; !ok {
			removed = append(removed, filter)
		}
	}
	toSubscribe := make(map[string]byte)
	for filter, qos := range desired {
		if grantedQoS, ok := granted[filter]; ok && grantedDesired[filter] == qos {
			status := statuses[filter]
			status.Pending = false
			status.Failed = false
			status.GrantedQoS = grantedQoS
			grantedDesired[filter] = qos
			statuses[filter] = status
			continue
		}
		toSubscribe[filter] = qos
	}
	for filter := range toSubscribe {
		status := statuses[filter]
		status.Pending = true
		status.Attempts++
		statuses[filter] = status
		// A SUBSCRIBE may have reached the broker even when its ACK is
		// delayed or malformed; retain the filter as known so a later owner
		// removal can always issue the compensating UNSUBSCRIBE.
		known[filter] = struct{}{}
	}
	for _, filter := range removed {
		if _, ok := statuses[filter]; !ok {
			statuses[filter] = MQTTSubscriptionStatus{Filter: filter, QoS: grantedDesired[filter], GrantedQoS: granted[filter], Pending: true, Failed: true, LastError: "pending unsubscribe"}
		} else {
			status := statuses[filter]
			status.Owners = nil
			status.Pending = true
			statuses[filter] = status
		}
	}
	gen.subscriptionMu.Lock()
	gen.grantedFilters, gen.grantedDesired, gen.knownFilters, gen.subscriptions = granted, grantedDesired, known, statuses
	gen.subscriptionMu.Unlock()
	if len(removed) > 0 || len(toSubscribe) > 0 {
		if err := c.persistGenerationLedger(cfg, gen); err != nil {
			for filter := range toSubscribe {
				gen.subscriptionMu.Lock()
				status := gen.subscriptions[filter]
				status.Pending = true
				status.Failed = true
				status.LastError = err.Error()
				gen.subscriptions[filter] = status
				gen.subscriptionMu.Unlock()
			}
			return
		}
	}
	if !c.generationCurrent(gen) || gen.ctx.Err() != nil {
		return
	}
	if len(removed) > 0 {
		token := gen.client.Unsubscribe(removed...)
		if err := waitTokenContext(token, gen.ctx, 10*time.Second); err == nil {
			if !c.generationCurrent(gen) || gen.ctx.Err() != nil {
				return
			}
			for _, filter := range removed {
				delete(granted, filter)
				delete(grantedDesired, filter)
				delete(known, filter)
				delete(statuses, filter)
			}
		} else {
			for _, filter := range removed {
				status := statuses[filter]
				status.Pending = true
				status.Failed = true
				status.LastError = fmt.Sprintf("MQTT unsubscribe removed filters: %v", err)
				statuses[filter] = status
			}
		}
	}

	if len(toSubscribe) == 0 {
		gen.subscriptionMu.Lock()
		gen.grantedFilters, gen.grantedDesired, gen.knownFilters, gen.subscriptions = granted, grantedDesired, known, statuses
		gen.subscriptionMu.Unlock()
		_ = c.persistGenerationLedger(cfg, gen)
		return
	}
	result := subscribeMultipleResult(gen.client, toSubscribe, c.messageHandler(gen), gen.ctx, 10*time.Second)
	if !c.generationCurrent(gen) || gen.ctx.Err() != nil {
		return
	}
	for filter, qos := range result.granted {
		granted[filter] = qos
		grantedDesired[filter] = toSubscribe[filter]
		known[filter] = struct{}{}
		status := statuses[filter]
		status.GrantedQoS = qos
		status.Pending = false
		status.Failed = false
		status.LastError = ""
		statuses[filter] = status
	}
	for filter, code := range result.rejected {
		delete(granted, filter)
		delete(grantedDesired, filter)
		status := statuses[filter]
		status.Pending = true
		status.Failed = true
		status.LastError = fmt.Sprintf("MQTT SUBACK rejected topic %q (return code 0x%02x)", filter, code)
		statuses[filter] = status
	}
	if result.err != nil {
		for filter := range toSubscribe {
			if _, granted := result.granted[filter]; granted {
				continue
			}
			status := statuses[filter]
			if !status.Failed {
				status.Pending = true
				status.Failed = true
				status.LastError = result.err.Error()
				statuses[filter] = status
			}
		}
	}
	gen.subscriptionMu.Lock()
	gen.grantedFilters, gen.grantedDesired, gen.knownFilters, gen.subscriptions = granted, grantedDesired, known, statuses
	gen.subscriptionMu.Unlock()
	if err := c.persistGenerationLedger(cfg, gen); err != nil {
		for filter := range toSubscribe {
			gen.subscriptionMu.Lock()
			status := gen.subscriptions[filter]
			status.Failed = true
			status.LastError = err.Error()
			gen.subscriptions[filter] = status
			gen.subscriptionMu.Unlock()
		}
	}
}

func subscribeMultipleResult(client pahomqtt.Client, filters map[string]byte, handler pahomqtt.MessageHandler, ctx context.Context, timeout time.Duration) subscribeResult {
	if len(filters) == 0 {
		return subscribeResult{granted: map[string]byte{}}
	}
	result := subscribeResult{granted: make(map[string]byte), rejected: make(map[string]byte)}
	token := client.SubscribeMultiple(filters, handler)
	if err := waitTokenContext(token, ctx, timeout); err != nil {
		result.err = err
		return result
	}
	resultToken, ok := token.(interface{ Result() map[string]byte })
	if !ok {
		result.err = fmt.Errorf("MQTT subscribe did not return SUBACK results")
		return result
	}
	ack := resultToken.Result()
	for filter := range ack {
		if _, expected := filters[filter]; !expected {
			// An extra key makes the complete SUBACK structurally invalid. Do
			// not commit any grants from that response.
			result.err = fmt.Errorf("MQTT SUBACK returned unexpected topic %q", filter)
			return result
		}
	}
	for filter := range filters {
		code, present := ack[filter]
		if !present {
			result.err = fmt.Errorf("MQTT SUBACK omitted topic %q", filter)
			continue
		}
		if code <= 2 {
			if code > filters[filter] {
				result.err = fmt.Errorf("MQTT SUBACK granted QoS %d above requested QoS %d for topic %q", code, filters[filter], filter)
				result.rejected[filter] = code
				continue
			}
			result.granted[filter] = code
		} else {
			result.rejected[filter] = code
		}
	}
	return result
}

func waitTokenContext(token pahomqtt.Token, ctx context.Context, timeout time.Duration) error {
	if token == nil {
		return fmt.Errorf("MQTT operation returned no token")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-token.Done():
		return token.Error()
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("MQTT operation timed out")
	}
}

func desiredFilterSnapshot(gen *mqttGeneration) map[string]byte {
	gen.subscriptionMu.Lock()
	defer gen.subscriptionMu.Unlock()
	result := make(map[string]byte, len(gen.desiredFilters))
	for filter, qos := range gen.desiredFilters {
		result[filter] = qos
	}
	return result
}

func subscriptionStatusSlice(gen *mqttGeneration) []MQTTSubscriptionStatus {
	gen.subscriptionMu.Lock()
	defer gen.subscriptionMu.Unlock()
	return cloneSubscriptionStatuses(gen.subscriptions)
}
