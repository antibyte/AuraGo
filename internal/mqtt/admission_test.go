package mqtt

import (
	"fmt"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestMQTTAdmissionBoundsDistinctTopicsAndSharesMissionQuota(t *testing.T) {
	now := time.Unix(10_000, 0)
	c := &MQTTController{now: func() time.Time { return now }}
	if !c.admitJob("home/a", true) || c.admitJob("home/a", true) {
		t.Fatal("same-topic admission must debounce at receipt")
	}
	for i := 1; i < maxJobsPerMinute; i++ {
		if !c.admitJob(fmt.Sprintf("home/%d", i), i%2 == 0) {
			t.Fatal("quota rejected an available slot")
		}
	}
	if c.admitJob("home/new", true) || c.admitJob("home/mission", false) {
		t.Fatal("rotating topics or mission jobs bypassed the shared quota")
	}
	now = now.Add(time.Minute)
	if !c.admitJob("home/a", true) {
		t.Fatal("rolling quota did not recover")
	}
	if !c.jobExpired(now.Add(-JobMaxAge)) || c.jobExpired(now.Add(-JobMaxAge+time.Millisecond)) {
		t.Fatal("queue TTL boundary is wrong")
	}
}

func TestMQTTAdmissionSnapshotsOwnRelayFiltersAndGrants(t *testing.T) {
	cfg := &config.Config{}
	cfg.MQTT.RelayTopics = []string{"home/#"}
	cfg.MQTT.RelayGrants = []config.ActionPolicyRule{{Tool: "mqtt_get_messages", Operation: "read", Target: "tool:mqtt_get_messages"}}
	snapshot := cloneMQTTSnapshot(cfg)
	cfg.MQTT.RelayTopics[0] = "changed/#"
	cfg.MQTT.RelayGrants[0].Tool = "execute_shell"
	copy := snapshot.config()
	if copy.MQTT.RelayTopics[0] != "home/#" || copy.MQTT.RelayGrants[0].Tool != "mqtt_get_messages" {
		t.Fatal("snapshot retained mutable caller-owned relay permissions")
	}
	copy.MQTT.RelayTopics[0] = "another/#"
	if snapshot.config().MQTT.RelayTopics[0] != "home/#" {
		t.Fatal("snapshot exposed its relay filters")
	}
}

func TestMQTTMissionPayloadIsIndependentOfObservationBuffer(t *testing.T) {
	buffer.Configure(10, 0, 4, 0)
	t.Cleanup(func() { buffer.Configure(0, 0, 0, 0) })
	probe := newRelayQueueProbe(t, true)
	probe.gen.missionQueue = make(chan missionJob, 1)
	missionTriggerMu.Lock()
	previous := missionTriggers
	missionTriggers = []missionTriggerEntry{{topicFilter: "sensors/temp", payloadContains: "matching-suffix"}}
	missionTriggerMu.Unlock()
	t.Cleanup(func() { missionTriggerMu.Lock(); missionTriggers = previous; missionTriggerMu.Unlock() })
	probe.deliver("sensors/temp", "prefix matching-suffix")
	if len(probe.gen.missionQueue) != 1 {
		t.Fatal("matching suffix was lost to observation buffer truncation")
	}
	if got := <-probe.gen.missionQueue; got.payload != "prefix matching-suffix" {
		t.Fatalf("queued mission lost its complete payload: %+v", got)
	}
}

func enableMQTTRelayForTest(cfg *config.Config) {
	cfg.MQTT.Enabled = true
	cfg.MQTT.AllowUnauthenticatedRelay, cfg.MQTT.AllowUntrustedTransport = true, true
	cfg.MQTT.RelayTopics = []string{"home/#", "sensors/#", "frigate/#", "mission/#", "first", "second", "third"}
	cfg.MQTT.RelayGrants = []config.ActionPolicyRule{{Tool: "mqtt_get_messages", Operation: "read", Target: "tool:mqtt_get_messages"}}
}
