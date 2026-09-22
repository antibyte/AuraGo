package mqtt

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func ledgerTestConfig(t *testing.T, broker, clientID string) *config.Config {
	t.Helper()
	cleanSession := false
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.MQTT.Broker = broker
	cfg.MQTT.ClientID = clientID
	cfg.MQTT.CleanSession = &cleanSession
	return cfg
}

func ledgerTestEntry(filter, ownerKind, ownerKey string, ownerQoS, grantedQoS byte, state string, pending, failed bool) subscriptionLedgerEntry {
	return subscriptionLedgerEntry{
		Filter: filter,
		Owners: []SubscriptionOwner{{Kind: ownerKind, Key: ownerKey, QoS: ownerQoS}},
		QoS:    ownerQoS, GrantedQoS: grantedQoS, State: state,
		Pending: pending, Failed: failed, UpdatedAt: time.Unix(1700000000, 0).UTC(),
	}
}

func writeLedgerTestEntries(t *testing.T, cfg *config.Config, entries []subscriptionLedgerEntry) {
	t.Helper()
	controller := NewMQTTController(nil)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := controller.Stop(ctx); err != nil {
			t.Errorf("stop ledger fixture controller: %v", err)
		}
	})
	if err := controller.writeSubscriptionLedger(cfg, entries); err != nil {
		t.Fatalf("write ledger fixture: %v", err)
	}
}

func newLedgerTestGeneration(t *testing.T, cfg *config.Config) (*MQTTController, *mqttGeneration, *fakeMQTTClient) {
	t.Helper()
	controller := NewMQTTController(nil)
	controller.loadManualLedger(cfg)
	workCtx, cancelWork := context.WithCancel(context.Background())
	fakeClient := newFakeMQTTClient()
	gen := &mqttGeneration{
		ctx:    workCtx,
		cancel: cancelWork,
		client: fakeClient,
		done:   make(chan struct{}),
	}
	controller.initializeGenerationSubscriptions(gen, cfg)
	controller.mu.Lock()
	controller.current = gen
	controller.active = cloneMQTTSnapshot(cfg)
	controller.activeRevision = 1
	controller.mu.Unlock()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := controller.Stop(ctx); err != nil {
			t.Errorf("stop ledger test controller: %v", err)
		}
	})
	return controller, gen, fakeClient
}

func ledgerTestStatus(gen *mqttGeneration, filter string) (MQTTSubscriptionStatus, bool) {
	gen.subscriptionMu.Lock()
	defer gen.subscriptionMu.Unlock()
	status, ok := gen.subscriptions[filter]
	return status, ok
}

func TestMQTTSubscriptionLedgerWriteFailurePreservesPreviousFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the fixture relies on Windows refusing replacement of a read-only target")
	}
	cfg := ledgerTestConfig(t, "mqtts://broker.example:8883", "ledger-client")
	old := ledgerTestEntry("configured/old", SubscriptionOwnerConfig, "configured/old", 1, 1, "active", false, false)
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{old})
	path := ManualSubscriptionLedgerPath(cfg)
	oldBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	controller := NewMQTTController(nil)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = controller.Stop(ctx)
	})
	newEntry := ledgerTestEntry("configured/new", SubscriptionOwnerConfig, "configured/new", 1, 1, "active", false, false)
	if err := controller.writeSubscriptionLedger(cfg, []subscriptionLedgerEntry{newEntry}); err == nil {
		t.Fatal("write unexpectedly replaced a read-only ledger")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, oldBytes) {
		t.Fatalf("failed write changed the previous ledger: before=%q after=%q", oldBytes, got)
	}
}

func TestMQTTSubscriptionLedgerContainsNoBrokerCredentialsOrPayloads(t *testing.T) {
	cfg := ledgerTestConfig(t, "mqtts://raw-user:raw-password@broker.example:8883/path", " spaced client ")
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{
		ledgerTestEntry("sensors/temperature", SubscriptionOwnerManual, "sensors/temperature", 1, 1, "active", false, false),
	})
	path := ManualSubscriptionLedgerPath(cfg)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"raw-user", "raw-password", "broker.example", "payload-sentinel"} {
		if bytes.Contains(data, []byte(secret)) || strings.Contains(path, secret) {
			t.Fatalf("ledger exposed %q: path=%q data=%q", secret, path, data)
		}
	}
}

func TestMQTTSubscriptionLedgerSeparatesBrokerAndClientIdentityWithSpaces(t *testing.T) {
	dataDir := t.TempDir()
	cleanSession := false
	makeConfig := func(broker, clientID string) *config.Config {
		cfg := &config.Config{}
		cfg.Directories.DataDir = dataDir
		cfg.MQTT.Broker = broker
		cfg.MQTT.ClientID = clientID
		cfg.MQTT.CleanSession = &cleanSession
		return cfg
	}
	withSpaces := makeConfig("mqtt://broker-a:1883", " client A ")
	trimmedID := makeConfig("mqtt://broker-a:1883", "client A")
	otherBroker := makeConfig("mqtt://broker-b:1883", " client A ")
	if ManualSubscriptionLedgerPath(withSpaces) == ManualSubscriptionLedgerPath(trimmedID) {
		t.Fatal("client IDs with and without surrounding spaces share a ledger")
	}
	if ManualSubscriptionLedgerPath(withSpaces) == ManualSubscriptionLedgerPath(otherBroker) {
		t.Fatal("different brokers share a ledger")
	}
	writeLedgerTestEntries(t, withSpaces, []subscriptionLedgerEntry{
		ledgerTestEntry("isolated/topic", SubscriptionOwnerManual, "isolated/topic", 1, 1, "active", false, false),
	})
	controller := NewMQTTController(nil)
	controller.loadManualLedger(trimmedID)
	if got := controller.manualSnapshot(); len(got) != 0 {
		t.Fatalf("different client identity restored entries: %v", got)
	}
	controller.loadManualLedger(otherBroker)
	if got := controller.manualSnapshot(); len(got) != 0 {
		t.Fatalf("different broker restored entries: %v", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := controller.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestMQTTSubscriptionLedgerRestoresConfirmedManualOwnerOnNewController(t *testing.T) {
	cfg := ledgerTestConfig(t, "mqtt://broker.example:1883", "manual-client")
	entry := ledgerTestEntry("manual/confirmed", SubscriptionOwnerManual, "manual/confirmed", 2, 2, "active", false, false)
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{entry})

	controller, gen, _ := newLedgerTestGeneration(t, cfg)
	manual := controller.manualSnapshot()
	if got, ok := manual[entry.Filter]; !ok || got != entry.Owners[0].QoS {
		t.Fatalf("confirmed manual owner was not restored: %v", manual)
	}
	status, ok := ledgerTestStatus(gen, entry.Filter)
	if !ok || status.Pending || status.Failed || status.GrantedQoS != entry.GrantedQoS {
		t.Fatalf("confirmed manual owner was not active: ok=%v status=%+v", ok, status)
	}
	if len(status.Owners) != 1 || status.Owners[0].Kind != SubscriptionOwnerManual {
		t.Fatalf("restored owner metadata = %+v", status.Owners)
	}
}

func TestMQTTSubscriptionLedgerDoesNotRestorePendingManualOwner(t *testing.T) {
	cfg := ledgerTestConfig(t, "mqtt://broker.example:1883", "pending-client")
	entry := ledgerTestEntry("manual/pending", SubscriptionOwnerManual, "manual/pending", 1, 0, "pending", true, false)
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{entry})

	controller, gen, fakeClient := newLedgerTestGeneration(t, cfg)
	if manual := controller.manualSnapshot(); len(manual) != 0 {
		t.Fatalf("pending manual owner was restored: %v", manual)
	}
	controller.reconcileGenerationSubscriptions(gen, cfg)
	if !ledgerUnsubscribedContains(fakeClient.unsubscribed, entry.Filter) {
		t.Fatalf("pending manual owner was not cleaned up at broker: %v", fakeClient.unsubscribed)
	}
}

func TestMQTTSubscriptionLedgerMarksKnownOldConfiguredFilterForUnsubscribe(t *testing.T) {
	cfg := ledgerTestConfig(t, "mqtt://broker.example:1883", "config-client")
	oldFilter := "configured/old/#"
	entry := ledgerTestEntry(oldFilter, SubscriptionOwnerConfig, oldFilter, 1, 1, "active", false, false)
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{entry})

	controller, gen, fakeClient := newLedgerTestGeneration(t, cfg)
	if _, ok := gen.grantedFilters[oldFilter]; !ok {
		t.Fatalf("active old configured filter was not loaded as known: %+v", gen.grantedFilters)
	}
	controller.reconcileGenerationSubscriptions(gen, cfg)
	found := false
	for _, filter := range fakeClient.unsubscribed {
		if filter == oldFilter {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("known old configured filter was not sent to unsubscribe: %v", fakeClient.unsubscribed)
	}
}

func ledgerUnsubscribedContains(filters []string, wanted ...string) bool {
	seen := make(map[string]struct{}, len(filters))
	for _, filter := range filters {
		seen[filter] = struct{}{}
	}
	for _, filter := range wanted {
		if _, ok := seen[filter]; !ok {
			return false
		}
	}
	return true
}

func TestMQTTSubscriptionLedgerUnsubscribesKnownPendingFiltersWithoutOwners(t *testing.T) {
	cfg := ledgerTestConfig(t, "mqtt://broker.example:1883", "pending-cleanup-client")
	pendingSubscribe := subscriptionLedgerEntry{
		Filter: "legacy/pending-sub/#", QoS: 1, State: "pending", Pending: true,
		UpdatedAt: time.Unix(1700000000, 0).UTC(),
	}
	pendingUnsubscribe := subscriptionLedgerEntry{
		Filter: "legacy/pending-unsub/#", QoS: 1, GrantedQoS: 1, State: "pending", Pending: true,
		UpdatedAt: time.Unix(1700000000, 0).UTC(),
	}
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{pendingSubscribe, pendingUnsubscribe})

	controller, gen, fakeClient := newLedgerTestGeneration(t, cfg)
	controller.reconcileGenerationSubscriptions(gen, cfg)
	if !ledgerUnsubscribedContains(fakeClient.unsubscribed, pendingSubscribe.Filter, pendingUnsubscribe.Filter) {
		t.Fatalf("known pending filters were not both unsubscribed: %v", fakeClient.unsubscribed)
	}
}

func TestMQTTSubscriptionLedgerRemovesOldConfirmedConfigAfterConnectionReset(t *testing.T) {
	cfg := ledgerTestConfig(t, "mqtt://broker.example:1883", "reset-client")
	oldFilter := "legacy/confirmed-config/#"
	entry := ledgerTestEntry(oldFilter, SubscriptionOwnerConfig, oldFilter, 1, 1, "active", false, false)
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{entry})

	controller, gen, fakeClient := newLedgerTestGeneration(t, cfg)
	if _, ok := gen.grantedFilters[oldFilter]; !ok {
		t.Fatalf("confirmed old filter was not loaded: %v", gen.grantedFilters)
	}
	resetGenerationAcknowledgements(gen)
	controller.reconcileGenerationSubscriptions(gen, cfg)
	if !ledgerUnsubscribedContains(fakeClient.unsubscribed, oldFilter) {
		t.Fatalf("old confirmed filter was not removed after connection reset: %v", fakeClient.unsubscribed)
	}
}

func TestMQTTSubscriptionLedgerResubscribesDesiredFilterAfterPersistentSessionReset(t *testing.T) {
	cfg := ledgerTestConfig(t, "mqtt://broker.example:1883", "persistent-reset-client")
	filter := "configured/reconnect/#"
	cfg.MQTT.Topics = []string{filter}
	cfg.MQTT.QoS = 1
	entry := ledgerTestEntry(filter, SubscriptionOwnerConfig, filter, 1, 1, "active", false, false)
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{entry})

	controller, gen, fakeClient := newLedgerTestGeneration(t, cfg)
	if status, ok := ledgerTestStatus(gen, filter); !ok || status.Pending || status.GrantedQoS != entry.GrantedQoS {
		t.Fatalf("active desired filter was not restored before reset: ok=%v status=%+v", ok, status)
	}
	resetGenerationAcknowledgements(gen)
	controller.reconcileGenerationSubscriptions(gen, cfg)
	if fakeClient.subscribeMultipleRuns == 0 {
		t.Fatalf("persistent session reset did not resubscribe desired filter")
	}
	if got, ok := fakeClient.subscriptions[filter]; !ok || got != entry.QoS {
		t.Fatalf("resubscribed filter = %q, present=%v; want qos %d", got, ok, entry.QoS)
	}
}

func TestMQTTSubscriptionLedgerKeepsConfirmedManualAWhenPendingManualBIsAdded(t *testing.T) {
	cfg := ledgerTestConfig(t, "mqtt://broker.example:1883", "mixed-manual-client")
	manualA := ledgerTestEntry("manual/confirmed-a", SubscriptionOwnerManual, "manual/confirmed-a", 1, 1, "active", false, false)
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{manualA})

	controller, gen, _ := newLedgerTestGeneration(t, cfg)
	manualB := "manual/pending-b"
	controller.setManualTopic(manualB, 1)
	controller.updateGenerationSubscriptions(gen, cfg)
	statusB, ok := ledgerTestStatus(gen, manualB)
	if !ok || !statusB.Pending {
		t.Fatalf("new manual B was not pending before broker confirmation: ok=%v status=%+v", ok, statusB)
	}
	// This is the pre-confirmation write performed while B is being added. A
	// confirmed A must retain its active ledger state across this pending write.
	if err := controller.persistManualLedger(cfg); err != nil {
		t.Fatalf("persist mixed manual ledger: %v", err)
	}

	restarted, restartedGen, _ := newLedgerTestGeneration(t, cfg)
	manual := restarted.manualSnapshot()
	if got, ok := manual[manualA.Filter]; !ok || got != manualA.Owners[0].QoS {
		t.Fatalf("confirmed manual A was not restored: %v", manual)
	}
	if _, ok := manual[manualB]; ok {
		t.Fatalf("pending manual B was restored as confirmed: %v", manual)
	}
	statusA, ok := ledgerTestStatus(restartedGen, manualA.Filter)
	if !ok || statusA.Pending || statusA.Failed || statusA.GrantedQoS != manualA.GrantedQoS {
		t.Fatalf("confirmed manual A lost active state after restart: ok=%v status=%+v", ok, statusA)
	}
}
