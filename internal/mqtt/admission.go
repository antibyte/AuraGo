package mqtt

import (
	"sync/atomic"
	"time"

	"aurago/internal/config"
)

const (
	RelayPayloadLimit   = 16 << 10
	MissionPayloadLimit = 64 << 10
	JobMaxAge           = time.Minute
	maxJobsPerMinute    = 10
	relayTopicInterval  = 2 * time.Second
)

// RelayAllowedTopic selects received traffic without creating broker subscriptions.
func RelayAllowedTopic(cfg *config.Config, topic string) bool {
	return config.MQTTRelayTopicAllowed(cfg, topic)
}

func (c *MQTTController) clockNow() time.Time {
	if c.now != nil {
		return c.now().UTC()
	}
	return time.Now().UTC()
}

func (c *MQTTController) jobExpired(stamp time.Time) bool {
	return !stamp.IsZero() && c.clockNow().Sub(stamp) >= JobMaxAge
}

func (c *MQTTController) admitJob(topic string, relay bool) bool {
	c.admissionMu.Lock()
	defer c.admissionMu.Unlock()
	now := c.clockNow()
	kept := c.admittedAt[:0]
	for _, stamp := range c.admittedAt {
		if now.Sub(stamp) < time.Minute {
			kept = append(kept, stamp)
		}
	}
	c.admittedAt = kept
	for topic, stamp := range c.lastRelayTopic {
		if now.Sub(stamp) >= relayTopicInterval {
			delete(c.lastRelayTopic, topic)
		}
	}
	if relay {
		if _, exists := c.lastRelayTopic[topic]; exists {
			atomic.AddUint64(&stats.debouncedRelayMessages, 1)
			return false
		}
	}
	if len(c.admittedAt) >= maxJobsPerMinute {
		atomic.AddUint64(&stats.rateLimitedJobs, 1)
		c.warnOverload("rate_limited")
		return false
	}
	c.admittedAt = append(c.admittedAt, now)
	if relay {
		if c.lastRelayTopic == nil {
			c.lastRelayTopic = make(map[string]time.Time)
		}
		c.lastRelayTopic[topic] = now
	}
	return true
}

func (c *MQTTController) warnOverload(reason string) {
	if c.log == nil {
		return
	}
	now := c.clockNow().Unix()
	last := c.lastOverloadLog.Load()
	if (last == 0 || now-last >= 60) && c.lastOverloadLog.CompareAndSwap(last, now) {
		c.log.Warn("[MQTT] Agent delivery dropped; inspect MQTT status counters", "reason", reason)
	}
}
