package dockerstacks

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---- networks ----

func (c *converter) network() {
	var user []string
	mode := ""
	for _, n := range c.run.values["network"] {
		switch {
		case n == "host" || n == "none" || n == "bridge" || strings.HasPrefix(n, "container:"):
			if mode == "" {
				mode = n
			}
		case n == "" || n == "default":
		default:
			user = append(user, n)
		}
	}
	aliases := c.run.values["network-alias"]
	switch {
	case mode != "":
		c.svc.add("network_mode", strNode(mode))
		if len(user) > 0 {
			c.warn(warnValueInvalid, "--network", "network_mode cannot be combined with other networks; "+strings.Join(user, ", ")+" was left out.")
		}
		if len(aliases) > 0 {
			c.warn(warnIgnored, "--network-alias", "Network aliases only apply to user-defined networks.")
		}
	case len(user) == 0:
		if len(aliases) > 0 {
			c.warn(warnIgnored, "--network-alias", "Network aliases only apply to user-defined networks.")
		}
	case len(aliases) == 0:
		c.svc.add("networks", strSeq(user, false))
	default:
		m := newMapping()
		for _, n := range user {
			entry := newMapping()
			entry.add("aliases", strSeq(aliases, false))
			m.add(n, entry.n)
		}
		c.svc.add("networks", m.n)
	}
	for _, n := range user {
		if mode == "" && !stringIn(c.networks, n) {
			c.networks = append(c.networks, n)
		}
	}
}

// ---- health, logging, GPUs ----

func (c *converter) healthcheck() {
	if c.run.bools["no-healthcheck"] {
		hc := newMapping()
		hc.add("disable", boolNode(true))
		c.svc.add("healthcheck", hc.n)
		return
	}
	hc := newMapping()
	if cmd, ok := c.run.last("health-cmd"); ok && cmd != "" {
		if strings.EqualFold(cmd, "none") {
			hc.add("disable", boolNode(true))
		} else {
			hc.add("test", flowSeqNode(quotedNode("CMD-SHELL"), quotedNode(cmd)))
		}
	}
	for _, d := range [][2]string{{"health-interval", "interval"}, {"health-timeout", "timeout"}, {"health-start-period", "start_period"}} {
		v, ok := c.run.last(d[0])
		switch {
		case !ok || v == "":
		case !durationPattern.MatchString(v):
			c.invalid(d[0], v, "expected a duration such as 30s or 1m30s")
		default:
			hc.add(d[1], strNode(v))
		}
	}
	if v, ok := c.run.last("health-retries"); ok && v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 0 {
			c.invalid("health-retries", v, "expected a number")
		} else {
			hc.add("retries", numberNode(v))
		}
	}
	if !hc.empty() {
		c.svc.add("healthcheck", hc.n)
	}
}

func (c *converter) logging() {
	driver, hasDriver := c.run.last("log-driver")
	opts := newMapping()
	for _, o := range c.run.values["log-opt"] {
		k, v, ok := strings.Cut(o, "=")
		if k = strings.TrimSpace(k); !ok || k == "" {
			c.invalid("log-opt", o, "expected KEY=VALUE")
			continue
		}
		opts.set(k, strNode(v))
	}
	logging := newMapping()
	if hasDriver && driver != "" {
		logging.add("driver", strNode(driver))
	}
	if !opts.empty() {
		logging.add("options", opts.n)
	}
	if !logging.empty() {
		c.svc.add("logging", logging.n)
	}
}

// gpus converts --gpus (all, a count, or a device list) to a device reservation.
func (c *converter) gpus() {
	v, ok := c.run.last("gpus")
	if !ok || v == "" {
		return
	}
	fields, err := csvFields(v)
	if err != nil {
		c.invalid("gpus", v, "the value cannot be read")
		return
	}
	count, driver, current := "", "nvidia", ""
	var ids, caps []string
	for _, f := range fields {
		key, val, hasVal := strings.Cut(f, "=")
		if !hasVal {
			switch current { // a bare item continues the previous list: device=0,1
			case "device":
				ids = append(ids, f)
			case "capabilities":
				caps = append(caps, f)
			default:
				count = f
			}
			continue
		}
		current = ""
		switch strings.ToLower(key) {
		case "count":
			count = val
		case "driver":
			driver = val
		case "device":
			ids, current = strings.Split(val, ","), "device"
		case "capabilities":
			caps, current = strings.Split(val, ","), "capabilities"
		default:
			c.warn(warnUnsupported, "--gpus", fmt.Sprintf("--gpus option %q is not converted; add it by hand.", key))
		}
	}
	if len(ids) == 0 && (count == "" || count == "-1") {
		count = "all"
	}
	if n, err := strconv.Atoi(count); count != "" && count != "all" && (err != nil || n < 1) {
		c.invalid("gpus", v, "expected all, a number of GPUs or device=...")
		return
	}
	if !stringIn(caps, "gpu") {
		caps = append([]string{"gpu"}, caps...)
	}
	dev := newMapping()
	dev.add("driver", strNode(driver))
	if len(ids) > 0 {
		dev.add("device_ids", flowSeqNode(quotedNodes(ids)...))
	} else {
		dev.add("count", numberNode(count))
	}
	capNodes := make([]*yaml.Node, 0, len(caps))
	for _, cp := range caps {
		capNodes = append(capNodes, strNode(cp))
	}
	dev.add("capabilities", flowSeqNode(capNodes...))
	reservations := newMapping()
	reservations.add("devices", seqNode(dev.n))
	resources := newMapping()
	resources.add("reservations", reservations.n)
	deploy := newMapping()
	deploy.add("resources", resources.n)
	c.svc.add("deploy", deploy.n)
}
