package mqtt

import (
	"testing"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type subscriptionSnapshotClient struct {
	pahomqtt.Client
	onSubscribe func()
}

func (c *subscriptionSnapshotClient) SubscribeMultiple(filters map[string]byte, handler pahomqtt.MessageHandler) pahomqtt.Token {
	c.onSubscribe()
	return c.Client.SubscribeMultiple(filters, handler)
}

func TestMQTTReconciliationKeepsPublishedMapsSeparateFromBrokerResults(t *testing.T) {
	const added, removed = "snapshot/new", "snapshot/old"
	client := &subscriptionSnapshotClient{Client: newSubscriptionRegressionClient(map[string]byte{added: 1})}
	cfg := subscriptionRegressionConfig(added)
	controller, gen := newSubscriptionRegressionFixture(t, cfg, client)
	gen.grantedFilters[removed] = 1
	gen.grantedDesired[removed] = 1
	gen.knownFilters[removed] = struct{}{}

	var pendingStatuses map[string]MQTTSubscriptionStatus
	var pendingGrants, pendingDesired map[string]byte
	var pendingKnown map[string]struct{}
	client.onSubscribe = func() {
		// Retain the published maps while the broker reply is still pending.
		// Reconciliation must not mutate them outside subscriptionMu afterward.
		gen.subscriptionMu.Lock()
		pendingStatuses = gen.subscriptions
		pendingGrants = gen.grantedFilters
		pendingDesired = gen.grantedDesired
		pendingKnown = gen.knownFilters
		gen.subscriptionMu.Unlock()
	}
	controller.reconcileGenerationSubscriptions(gen, cfg)

	if !pendingStatuses[added].Pending || !pendingStatuses[removed].Pending {
		t.Fatalf("broker results mutated published pending statuses: %+v", pendingStatuses)
	}
	if len(pendingGrants) != 1 || pendingGrants[removed] != 1 || len(pendingDesired) != 1 || pendingDesired[removed] != 1 {
		t.Fatalf("broker results mutated published grants: grants=%v desired=%v", pendingGrants, pendingDesired)
	}
	if _, ok := pendingKnown[removed]; !ok {
		t.Fatalf("unsubscribe mutated published known filters: %v", pendingKnown)
	}
	status := controller.Status()
	if len(status.Subscriptions) != 1 || status.Subscriptions[0].Filter != added || status.Subscriptions[0].Pending || status.Subscriptions[0].GrantedQoS != 1 {
		t.Fatalf("completed reconciliation did not publish the broker results: %+v", status.Subscriptions)
	}
}

func TestMQTTBrokerReplyCannotOverwriteNewSubscriptionOwners(t *testing.T) {
	const oldFilter, newFilter = "mission/home/#", "mission/garage/#"
	client := &subscriptionSnapshotClient{Client: newSubscriptionRegressionClient(map[string]byte{oldFilter: 1})}
	cfg := subscriptionRegressionConfig(oldFilter)
	controller, gen := newSubscriptionRegressionFixture(t, cfg, client)
	pending, release := make(chan struct{}), make(chan struct{})
	client.onSubscribe = func() {
		close(pending)
		<-release
	}
	reconciled, updated := make(chan struct{}), make(chan struct{})
	go func() {
		controller.reconcileGenerationSubscriptions(gen, cfg)
		close(reconciled)
	}()
	<-pending
	go func() {
		controller.updateGenerationSubscriptions(gen, subscriptionRegressionConfig(newFilter))
		close(updated)
	}()
	// Give an unsynchronized update a chance to publish while the old reply is held.
	select {
	case <-updated:
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	for _, done := range []<-chan struct{}{reconciled, updated} {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("subscription update did not finish after the broker reply")
		}
	}
	status := subscriptionRegressionStatus(t, gen, newFilter)
	if status.Filter != newFilter || len(status.Owners) != 1 || !status.Pending {
		t.Fatalf("broker reply replaced newer ownership: %+v", status)
	}
	gen.subscriptionMu.Lock()
	_, stale := gen.subscriptions[oldFilter]
	gen.subscriptionMu.Unlock()
	if stale {
		t.Fatal("old ownership survived the configuration update")
	}
}
