package mqtt

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
)

// invalidTLSGenerationSnapshot fails while building client options, before
// Paho opens a socket. That makes the generation guard tests deterministic.
func invalidTLSGenerationSnapshot(t *testing.T) *mqttSnapshot {
	t.Helper()
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = "mqtts://127.0.0.1:1883"
	cfg.MQTT.TLS.CAFile = filepath.Join(t.TempDir(), "missing-ca.pem")
	return cloneMQTTSnapshot(cfg)
}

func stopMQTTControllerGuard(t *testing.T, controller *MQTTController) {
	t.Helper()
	if controller == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := controller.Stop(ctx); err != nil {
		t.Fatalf("stop MQTT controller: %v", err)
	}
}

func currentDefaultControllerGuard() *MQTTController {
	defaultControllerMu.RLock()
	defer defaultControllerMu.RUnlock()
	return defaultController
}

func controllerGuardStopped(controller *MQTTController) bool {
	if controller == nil {
		return true
	}
	controller.mu.RLock()
	defer controller.mu.RUnlock()
	return controller.stopped || controller.workerDone == nil
}

func waitMQTTControllerGuard(t *testing.T, controller *MQTTController, predicate func(MQTTStatus) bool) MQTTStatus {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		status := controller.Status()
		if predicate(status) {
			return status
		}
		if time.Now().After(deadline) {
			t.Fatalf("MQTT controller did not converge: %+v", status)
		}
		time.Sleep(time.Millisecond)
	}
}

func waitMQTTControllerGuardClosed(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("MQTT controller worker did not stop")
	}
}

func restoreDefaultControllerGuard(t *testing.T, previous, current *MQTTController) {
	t.Helper()
	if latest := currentDefaultControllerGuard(); latest != nil && latest != previous && latest != current {
		stopMQTTControllerGuard(t, latest)
	}
	if current != nil && current != previous {
		stopMQTTControllerGuard(t, current)
	}
	defaultControllerMu.Lock()
	defaultController = previous
	defaultControllerMu.Unlock()
}

func TestMQTTControllerStartGenerationAfterStopIsIgnored(t *testing.T) {
	controller := NewMQTTController(nil)
	stopMQTTControllerGuard(t, controller)

	controller.startGeneration(invalidTLSGenerationSnapshot(t), 1)

	controller.mu.RLock()
	active, current := controller.active, controller.current
	controller.mu.RUnlock()
	if active != nil || current != nil {
		t.Fatalf("stopped controller accepted a generation: active=%v current=%v", active != nil, current != nil)
	}
	if status := controller.Status(); status.Connected || status.Generation != 0 {
		t.Fatalf("stopped controller became active after direct start: %+v", status)
	}
}

func TestMQTTControllerStartGenerationWithSupersededRevisionIsIgnored(t *testing.T) {
	controller := NewMQTTController(nil)
	t.Cleanup(func() { stopMQTTControllerGuard(t, controller) })

	// Install a newer desired revision without signalling the worker. The
	// direct call below represents a queued start for the older revision.
	controller.mu.Lock()
	controller.desired = cloneMQTTSnapshot(&config.Config{})
	controller.desiredRevision = 2
	controller.mu.Unlock()

	controller.startGeneration(invalidTLSGenerationSnapshot(t), 1)

	controller.mu.RLock()
	active, current := controller.active, controller.current
	controller.mu.RUnlock()
	if active != nil || current != nil {
		t.Fatalf("superseded generation was installed: active=%v current=%v", active != nil, current != nil)
	}
	if status := controller.Status(); status.Generation != 0 || status.ActiveRevision != 0 {
		t.Fatalf("superseded generation changed active state: %+v", status)
	}
}

func TestMQTTStartStopStartClientRestartsDefaultController(t *testing.T) {
	previous := currentDefaultControllerGuard()
	controller := NewMQTTController(nil)
	SetDefaultController(controller)
	t.Cleanup(func() { restoreDefaultControllerGuard(t, previous, controller) })

	disabled := &config.Config{}
	StartClient(disabled, nil)
	waitMQTTControllerGuard(t, controller, func(status MQTTStatus) bool {
		return status.DesiredRevision > 0 && status.ActiveRevision == status.DesiredRevision && status.State == "disabled"
	})

	StopClient()
	if !controllerGuardStopped(currentDefaultControllerGuard()) {
		t.Fatal("StopClient left the default controller running")
	}

	StartClient(disabled, nil)
	deadline := time.Now().Add(3 * time.Second)
	for {
		started := currentDefaultControllerGuard()
		if started != nil && !controllerGuardStopped(started) {
			status := started.Status()
			if status.DesiredRevision > 0 && status.ActiveRevision == status.DesiredRevision && status.State == "disabled" {
				return
			}
		}
		if time.Now().After(deadline) {
			if started == nil {
				t.Fatal("StartClient did not restart the default controller: no default controller")
			}
			t.Fatalf("StartClient did not restart the default controller: %+v", started.Status())
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSetDefaultControllerStopsPreviousWorker(t *testing.T) {
	previous := currentDefaultControllerGuard()
	prior := NewMQTTController(nil)
	SetDefaultController(prior)
	prior.mu.RLock()
	priorDone := prior.workerDone
	prior.mu.RUnlock()

	replacement := NewMQTTController(nil)
	SetDefaultController(replacement)
	t.Cleanup(func() { restoreDefaultControllerGuard(t, previous, replacement) })

	waitMQTTControllerGuardClosed(t, priorDone)
	if !controllerGuardStopped(prior) {
		t.Fatal("replaced default controller still has a live worker")
	}
}
