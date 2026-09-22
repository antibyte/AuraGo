package config

import "testing"

func TestConfigCloneDetachesMutableSnapshotState(t *testing.T) {
	clean := false
	original := &Config{}
	original.MQTT.Topics = []string{"home/old"}
	original.MQTT.CleanSession = &clean
	original.MQTT.Password = "fixture"
	original.Webhooks.Outgoing = []OutgoingWebhook{{Headers: map[string]string{"X-Fixture": "old"}}}
	original.AuthorizationSnapshots = func() (*Config, *Config) { return original, original }
	cloned := original.Clone()
	cloned.MQTT.Topics[0] = "home/new"
	*cloned.MQTT.CleanSession = true
	cloned.Webhooks.Outgoing[0].Headers["X-Fixture"] = "new"
	if original.MQTT.Topics[0] != "home/old" || clean || original.Webhooks.Outgoing[0].Headers["X-Fixture"] != "old" {
		t.Fatal("cloned data still shares mutable state")
	}
	if cloned.MQTT.Password != "fixture" || cloned.AuthorizationSnapshots == nil {
		t.Fatal("clone lost runtime-only values")
	}
}
