package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"aurago/internal/fritzbox"
	"aurago/internal/inventory"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func dispatchNetwork(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	cfg := dc.Cfg
	logger := dc.Logger
	vault := dc.Vault
	registry := dc.Registry
	inventoryDB := dc.InventoryDB
	budgetTracker := dc.BudgetTracker
	handled := true

	result := func() string {
		switch tc.Action {
		case "mdns_scan":
			if !cfg.Tools.NetworkScan.Enabled {
				return `Tool Output: {"status":"error","message":"mdns_scan is disabled. Enable tools.network_scan.enabled in config."}`
			}
			req := decodeMDNSScanArgs(tc)
			logger.Info("LLM requested mdns_scan", "service_type", req.ServiceType, "timeout", req.Timeout, "auto_register", req.AutoRegister)
			scanResult := tools.MDNSScan(logger, req.ServiceType, req.Timeout)
			if req.AutoRegister && inventoryDB != nil {
				scanResult = mdnsAutoRegister(scanResult, inventoryDB, req.RegisterType, req.RegisterTags, req.OverwriteExisting, logger)
			}
			return "Tool Output: " + security.Scrub(scanResult)

		case "mac_lookup":
			if !cfg.Tools.NetworkScan.Enabled {
				return `Tool Output: {"status":"error","message":"mac_lookup is disabled. Enable tools.network_scan.enabled in config."}`
			}
			req := decodeMACLookupArgs(tc)
			if req.IP == "" {
				return `Tool Output: {"status":"error","message":"'ip' parameter is required"}`
			}
			logger.Info("LLM requested mac_lookup", "ip", req.IP)
			return "Tool Output: " + tools.LookupMACAddress(req.IP, "")

		case "tailscale":
			if !cfg.Tailscale.Enabled {
				return `Tool Output: {"status":"error","message":"Tailscale integration is not enabled. Set tailscale.enabled=true in config.yaml."}`
			}
			req := decodeTailscaleArgs(tc)
			if cfg.Tailscale.ReadOnly {
				switch req.Operation {
				case "enable_routes", "disable_routes":
					return `Tool Output: {"status":"error","message":"Tailscale is in read-only mode. Disable tailscale.read_only to allow changes."}`
				}
			}
			tsCfg := tools.TailscaleConfig{APIKey: cfg.Tailscale.APIKey, Tailnet: cfg.Tailscale.Tailnet, ReadOnly: cfg.Tailscale.ReadOnly}
			// query is hostname, IP, or node ID for device-specific operations
			query := req.deviceQuery()
			switch req.Operation {
			case "devices", "list", "list_devices":
				logger.Info("LLM requested Tailscale list devices")
				return "Tool Output: " + tools.TailscaleListDevices(tsCfg)
			case "device", "get", "get_device":
				logger.Info("LLM requested Tailscale get device", "query", query)
				return "Tool Output: " + tools.TailscaleGetDevice(tsCfg, query)
			case "routes", "get_routes":
				logger.Info("LLM requested Tailscale get routes", "query", query)
				return "Tool Output: " + tools.TailscaleGetRoutes(tsCfg, query)
			case "enable_routes":
				routes := req.routes()
				logger.Info("LLM requested Tailscale enable routes", "query", query, "routes", routes)
				return "Tool Output: " + tools.TailscaleSetRoutes(tsCfg, query, routes, true)
			case "disable_routes":
				routes := req.routes()
				logger.Info("LLM requested Tailscale disable routes", "query", query, "routes", routes)
				return "Tool Output: " + tools.TailscaleSetRoutes(tsCfg, query, routes, false)
			case "dns", "get_dns":
				logger.Info("LLM requested Tailscale DNS config")
				return "Tool Output: " + tools.TailscaleGetDNS(tsCfg)
			case "acl", "get_acl":
				logger.Info("LLM requested Tailscale ACL policy")
				return "Tool Output: " + tools.TailscaleGetACL(tsCfg)
			case "local_status", "status":
				logger.Info("LLM requested Tailscale local status")
				return "Tool Output: " + tools.TailscaleLocalStatus()
			default:
				return `Tool Output: {"status":"error","message":"Unknown tailscale operation. Use: devices, device, routes, enable_routes, disable_routes, dns, acl, local_status"}`
			}

		case "cloudflare_tunnel":
			if !cfg.CloudflareTunnel.Enabled {
				return `Tool Output: {"status":"error","message":"Cloudflare Tunnel integration is not enabled. Set cloudflare_tunnel.enabled=true in config.yaml."}`
			}
			req := decodeCloudflareTunnelArgs(tc)
			if cfg.CloudflareTunnel.ReadOnly {
				switch req.Operation {
				case "start", "stop", "restart", "quick_tunnel", "install":
					return `Tool Output: {"status":"error","message":"Cloudflare Tunnel is in read-only mode. Disable cloudflare_tunnel.readonly to allow changes."}`
				}
			}
			tunnelCfg := tools.CloudflareTunnelConfig{
				Enabled:        cfg.CloudflareTunnel.Enabled,
				ReadOnly:       cfg.CloudflareTunnel.ReadOnly,
				Mode:           cfg.CloudflareTunnel.Mode,
				AutoStart:      cfg.CloudflareTunnel.AutoStart,
				AuthMethod:     cfg.CloudflareTunnel.AuthMethod,
				TunnelName:     cfg.CloudflareTunnel.TunnelName,
				AccountID:      cfg.CloudflareTunnel.AccountID,
				TunnelID:       cfg.CloudflareTunnel.TunnelID,
				LoopbackPort:   cfg.CloudflareTunnel.LoopbackPort,
				ExposeWebUI:    cfg.CloudflareTunnel.ExposeWebUI,
				ExposeHomepage: cfg.CloudflareTunnel.ExposeHomepage,
				MetricsPort:    cfg.CloudflareTunnel.MetricsPort,
				LogLevel:       cfg.CloudflareTunnel.LogLevel,
				DockerHost:     cfg.Docker.Host,
				WebUIPort:      cfg.Server.Port,
				HomepagePort:   cfg.Homepage.WebServerPort,
				DataDir:        cfg.Directories.DataDir,
				HTTPSEnabled:   cfg.Server.HTTPS.Enabled,
				HTTPSPort:      cfg.Server.HTTPS.HTTPSPort,
			}
			for _, r := range cfg.CloudflareTunnel.CustomIngress {
				tunnelCfg.CustomIngress = append(tunnelCfg.CustomIngress, tools.CloudflareIngress{
					Hostname: r.Hostname,
					Service:  r.Service,
					Path:     r.Path,
				})
			}
			switch req.Operation {
			case "start":
				logger.Info("LLM requested Cloudflare Tunnel start")
				return "Tool Output: " + tools.CloudflareTunnelStart(tunnelCfg, vault, registry, logger)
			case "stop":
				logger.Info("LLM requested Cloudflare Tunnel stop")
				return "Tool Output: " + tools.CloudflareTunnelStop(tunnelCfg, registry, logger)
			case "restart":
				logger.Info("LLM requested Cloudflare Tunnel restart")
				return "Tool Output: " + tools.CloudflareTunnelRestart(tunnelCfg, vault, registry, logger)
			case "status":
				logger.Info("LLM requested Cloudflare Tunnel status")
				return "Tool Output: " + tools.CloudflareTunnelStatus(tunnelCfg, registry, logger)
			case "quick_tunnel":
				port := req.Port
				logger.Info("LLM requested Cloudflare quick tunnel", "port", port)
				return "Tool Output: " + tools.CloudflareTunnelQuickTunnel(tunnelCfg, registry, logger, port)
			case "logs":
				logger.Info("LLM requested Cloudflare Tunnel logs")
				return "Tool Output: " + tools.CloudflareTunnelLogs(registry, logger)
			case "list_routes":
				logger.Info("LLM requested Cloudflare Tunnel list routes")
				return "Tool Output: " + tools.CloudflareTunnelListRoutes(tunnelCfg, logger)
			case "install":
				logger.Info("LLM requested Cloudflare Tunnel install binary")
				return "Tool Output: " + tools.CloudflareTunnelInstall(tunnelCfg, logger)
			default:
				return `Tool Output: {"status":"error","message":"Unknown cloudflare_tunnel operation. Use: start, stop, restart, status, quick_tunnel, logs, list_routes, install"}`
			}

		case "mqtt_publish":
			if !cfg.MQTT.Enabled {
				return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
			}
			if cfg.MQTT.ReadOnly {
				return `Tool Output: {"status":"error","message":"MQTT is in read-only mode. Disable mqtt.read_only to allow changes."}`
			}
			req := decodeMQTTArgs(tc)
			topic := req.Topic
			if topic == "" {
				return `Tool Output: {"status": "error", "message": "'topic' is required"}`
			}
			qos := req.QoS
			if qos < 0 || qos > 2 {
				qos = cfg.MQTT.QoS
			}
			logger.Info("LLM requested MQTT publish", "topic", topic, "retain", req.Retain, "payload_len", len(req.Payload))
			if err := tools.MQTTPublish(topic, req.Payload, qos, req.Retain, logger); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT publish failed: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Published to topic '%s'"}`, topic)

		case "mqtt_subscribe":
			if !cfg.MQTT.Enabled {
				return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
			}
			req := decodeMQTTArgs(tc)
			topic := req.Topic
			if topic == "" {
				return `Tool Output: {"status": "error", "message": "'topic' is required"}`
			}
			qos := req.QoS
			if qos < 0 || qos > 2 {
				qos = cfg.MQTT.QoS
			}
			logger.Info("LLM requested MQTT subscribe", "topic", topic, "qos", qos)
			if err := tools.MQTTSubscribe(topic, qos, logger); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT subscribe failed: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Subscribed to topic '%s' with QoS %d"}`, topic, qos)

		case "mqtt_unsubscribe":
			if !cfg.MQTT.Enabled {
				return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
			}
			req := decodeMQTTArgs(tc)
			topic := req.Topic
			if topic == "" {
				return `Tool Output: {"status": "error", "message": "'topic' is required"}`
			}
			logger.Info("LLM requested MQTT unsubscribe", "topic", topic)
			if err := tools.MQTTUnsubscribe(topic, logger); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT unsubscribe failed: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Unsubscribed from topic '%s'"}`, topic)

		case "mqtt_get_messages":
			if !cfg.MQTT.Enabled {
				return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
			}
			req := decodeMQTTArgs(tc)
			topic := req.Topic // empty = all topics
			limit := req.Limit
			if limit <= 0 {
				limit = 20
			}
			logger.Info("LLM requested MQTT get messages", "topic", topic, "limit", limit)
			msgs, err := tools.MQTTGetMessages(topic, limit, logger)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT get messages failed: %v"}`, err)
			}
			data, _ := json.Marshal(map[string]interface{}{
				"status": "success",
				"count":  len(msgs),
				"data":   msgs,
			})
			return "Tool Output: " + string(data)

		case "adguard", "adguard_home":
			if !cfg.AdGuard.Enabled {
				return `Tool Output: {"status":"error","message":"AdGuard Home is not enabled. Configure the adguard section in config.yaml."}`
			}
			req := decodeAdGuardArgs(tc)
			adgCfg := tools.AdGuardConfig{
				URL:      cfg.AdGuard.URL,
				Username: cfg.AdGuard.Username,
				Password: cfg.AdGuard.Password,
			}
			op := strings.ToLower(strings.TrimSpace(req.Operation))

			// Read-only operations
			switch op {
			case "status":
				logger.Info("LLM requested AdGuard status")
				return "Tool Output: " + tools.AdGuardStatus(adgCfg)
			case "stats":
				logger.Info("LLM requested AdGuard stats")
				return "Tool Output: " + tools.AdGuardStats(adgCfg)
			case "stats_top":
				logger.Info("LLM requested AdGuard top stats")
				return "Tool Output: " + tools.AdGuardStatsTop(adgCfg)
			case "query_log":
				logger.Info("LLM requested AdGuard query log", "search", req.Query, "limit", req.Limit)
				return "Tool Output: " + tools.AdGuardQueryLog(adgCfg, req.Query, req.Limit, req.Offset)
			case "filtering_status":
				logger.Info("LLM requested AdGuard filtering status")
				return "Tool Output: " + tools.AdGuardFilteringStatus(adgCfg)
			case "rewrite_list":
				logger.Info("LLM requested AdGuard rewrite list")
				return "Tool Output: " + tools.AdGuardRewriteList(adgCfg)
			case "blocked_services_list":
				logger.Info("LLM requested AdGuard blocked services list")
				return "Tool Output: " + tools.AdGuardBlockedServicesList(adgCfg)
			case "safebrowsing_status":
				logger.Info("LLM requested AdGuard safe browsing status")
				return "Tool Output: " + tools.AdGuardSafeBrowsingStatus(adgCfg)
			case "parental_status":
				logger.Info("LLM requested AdGuard parental status")
				return "Tool Output: " + tools.AdGuardParentalStatus(adgCfg)
			case "dhcp_status":
				logger.Info("LLM requested AdGuard DHCP status")
				return "Tool Output: " + tools.AdGuardDHCPStatus(adgCfg)
			case "clients":
				logger.Info("LLM requested AdGuard clients")
				return "Tool Output: " + tools.AdGuardClients(adgCfg)
			case "dns_info":
				logger.Info("LLM requested AdGuard DNS info")
				return "Tool Output: " + tools.AdGuardDNSInfo(adgCfg)
			case "test_upstream":
				logger.Info("LLM requested AdGuard test upstream", "servers", req.Services)
				return "Tool Output: " + tools.AdGuardTestUpstream(adgCfg, req.Services)
			}

			// Write operations — check readonly
			if cfg.AdGuard.ReadOnly {
				return `Tool Output: {"status":"error","message":"AdGuard Home is in read-only mode. Disable adguard.readonly to allow changes."}`
			}
			switch op {
			case "query_log_clear":
				logger.Info("LLM requested AdGuard query log clear")
				return "Tool Output: " + tools.AdGuardQueryLogClear(adgCfg)
			case "filtering_toggle":
				logger.Info("LLM requested AdGuard filtering toggle", "enabled", req.Enabled)
				return "Tool Output: " + tools.AdGuardFilteringToggle(adgCfg, req.Enabled)
			case "filtering_add_url":
				logger.Info("LLM requested AdGuard add filter URL", "url", req.URL)
				return "Tool Output: " + tools.AdGuardFilteringAddURL(adgCfg, req.Name, req.URL)
			case "filtering_remove_url":
				logger.Info("LLM requested AdGuard remove filter URL", "url", req.URL)
				return "Tool Output: " + tools.AdGuardFilteringRemoveURL(adgCfg, req.URL)
			case "filtering_refresh":
				logger.Info("LLM requested AdGuard filtering refresh")
				return "Tool Output: " + tools.AdGuardFilteringRefresh(adgCfg)
			case "filtering_set_rules":
				logger.Info("LLM requested AdGuard set filtering rules")
				return "Tool Output: " + tools.AdGuardFilteringSetRules(adgCfg, req.Rules)
			case "rewrite_add":
				logger.Info("LLM requested AdGuard add rewrite", "domain", req.Domain, "answer", req.Answer)
				return "Tool Output: " + tools.AdGuardRewriteAdd(adgCfg, req.Domain, req.Answer)
			case "rewrite_delete":
				logger.Info("LLM requested AdGuard delete rewrite", "domain", req.Domain, "answer", req.Answer)
				return "Tool Output: " + tools.AdGuardRewriteDelete(adgCfg, req.Domain, req.Answer)
			case "blocked_services_set":
				logger.Info("LLM requested AdGuard set blocked services", "services", req.Services)
				return "Tool Output: " + tools.AdGuardBlockedServicesSet(adgCfg, req.Services)
			case "safebrowsing_toggle":
				logger.Info("LLM requested AdGuard safe browsing toggle", "enabled", req.Enabled)
				return "Tool Output: " + tools.AdGuardSafeBrowsingToggle(adgCfg, req.Enabled)
			case "parental_toggle":
				logger.Info("LLM requested AdGuard parental toggle", "enabled", req.Enabled)
				return "Tool Output: " + tools.AdGuardParentalToggle(adgCfg, req.Enabled)
			case "dhcp_set_config":
				logger.Info("LLM requested AdGuard DHCP set config")
				return "Tool Output: " + tools.AdGuardDHCPSetConfig(adgCfg, req.Content)
			case "dhcp_add_lease":
				logger.Info("LLM requested AdGuard DHCP add lease", "mac", req.MAC, "ip", req.IP)
				return "Tool Output: " + tools.AdGuardDHCPAddLease(adgCfg, req.MAC, req.IP, req.Hostname)
			case "dhcp_remove_lease":
				logger.Info("LLM requested AdGuard DHCP remove lease", "mac", req.MAC, "ip", req.IP)
				return "Tool Output: " + tools.AdGuardDHCPRemoveLease(adgCfg, req.MAC, req.IP, req.Hostname)
			case "client_add":
				logger.Info("LLM requested AdGuard client add")
				return "Tool Output: " + tools.AdGuardClientAdd(adgCfg, req.Content)
			case "client_update":
				logger.Info("LLM requested AdGuard client update")
				return "Tool Output: " + tools.AdGuardClientUpdate(adgCfg, req.Content)
			case "client_delete":
				logger.Info("LLM requested AdGuard client delete", "name", req.Name)
				return "Tool Output: " + tools.AdGuardClientDelete(adgCfg, req.Name)
			case "dns_config":
				logger.Info("LLM requested AdGuard DNS config update")
				return "Tool Output: " + tools.AdGuardDNSConfig(adgCfg, req.Content)
			default:
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown adguard operation '%s'. Use: status, stats, stats_top, query_log, query_log_clear, filtering_status, filtering_toggle, filtering_add_url, filtering_remove_url, filtering_refresh, filtering_set_rules, rewrite_list, rewrite_add, rewrite_delete, blocked_services_list, blocked_services_set, safebrowsing_status, safebrowsing_toggle, parental_status, parental_toggle, dhcp_status, dhcp_set_config, dhcp_add_lease, dhcp_remove_lease, clients, client_add, client_update, client_delete, dns_info, dns_config, test_upstream"}`, op)
			}

		case "uptime_kuma":
			if !cfg.UptimeKuma.Enabled {
				return `Tool Output: {"status":"error","message":"Uptime Kuma is not enabled. Configure the uptime_kuma section in config.yaml."}`
			}
			req := decodeUptimeKumaArgs(tc)
			ukCfg := tools.UptimeKumaConfig{
				BaseURL:        cfg.UptimeKuma.BaseURL,
				APIKey:         cfg.UptimeKuma.APIKey,
				InsecureSSL:    cfg.UptimeKuma.InsecureSSL,
				RequestTimeout: cfg.UptimeKuma.RequestTimeout,
			}
			op := strings.ToLower(strings.TrimSpace(req.Operation))
			switch op {
			case "summary":
				logger.Info("LLM requested Uptime Kuma summary")
				return "Tool Output: " + tools.UptimeKumaSummaryJSON(context.Background(), ukCfg, logger)
			case "list_monitors":
				logger.Info("LLM requested Uptime Kuma monitor list")
				return "Tool Output: " + tools.UptimeKumaListMonitorsJSON(context.Background(), ukCfg, logger)
			case "get_monitor":
				logger.Info("LLM requested Uptime Kuma monitor", "monitor_name", req.MonitorName)
				return "Tool Output: " + tools.UptimeKumaGetMonitorJSON(context.Background(), ukCfg, req.MonitorName, logger)
			default:
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown uptime_kuma operation '%s'. Use: summary, list_monitors, get_monitor"}`, op)
			}

		case "fritzbox", "fritzbox_system", "fritzbox_network", "fritzbox_telephony", "fritzbox_smarthome", "fritzbox_storage", "fritzbox_tv":
			if !cfg.FritzBox.Enabled {
				return `Tool Output: {"status":"error","message":"Fritz!Box integration is not enabled. Set fritzbox.enabled=true in config.yaml."}`
			}
			fbClient, fbErr := fritzbox.NewClient(*cfg)
			if fbErr != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Fritz!Box client init failed: %s"}`, fbErr)
			}
			defer fbClient.Close()
			return handleFritzBoxToolCall(tc, fbClient, cfg, logger, budgetTracker)

		// ── TrueNAS Storage Management ──
		case "firewall", "firewall_rules", "iptables":
			if !cfg.Firewall.Enabled {
				return `Tool Output: {"status":"error","message":"Firewall management is not enabled. Set firewall.enabled=true in config.yaml."}`
			}
			firewallAccessOK := cfg.Runtime.FirewallAccessOK || (cfg.Agent.SudoEnabled && !cfg.Runtime.IsDocker)
			if !firewallAccessOK {
				return `Tool Output: {"status":"error","message":"Firewall is not accessible. Run as root, add NOPASSWD sudo for iptables, or enable sudo in the Danger Zone settings."}`
			}
			sudoPass := ""
			if cfg.Agent.SudoEnabled && !cfg.Runtime.FirewallAccessOK {
				if cfg.Runtime.NoNewPrivileges {
					return `Tool Output: {"status":"error","message":"sudo is blocked by the \"no new privileges\" flag — cannot escalate to run iptables. Remove no-new-privileges from your container/systemd config."}`
				}
				var vaultErr error
				sudoPass, vaultErr = vault.ReadSecret("sudo_password")
				if vaultErr != nil || sudoPass == "" {
					return `Tool Output: {"status":"error","message":"sudo_password not found in vault. Store it first via the secrets_vault tool."}`
				}
			}
			req := decodeFirewallArgs(tc)
			switch req.Operation {
			case "get_rules":
				rules, err := tools.FirewallGetRules(sudoPass)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
				}
				return "Tool Output: " + rules
			case "modify_rule":
				if cfg.Firewall.Mode == "readonly" {
					return `Tool Output: {"status":"error","message":"Firewall is in read-only mode. Disable firewall.read_only to allow changes."}`
				}
				if req.Command == "" {
					return `Tool Output: {"status":"error","message":"'command' is required for modify_rule (e.g. 'iptables -A INPUT -p tcp --dport 80 -j ACCEPT')"}`
				}
				out, err := tools.FirewallModifyRule(req.Command, sudoPass)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
				}
				return "Tool Output: " + out
			default:
				return `Tool Output: {"status":"error","message":"Unknown firewall operation. Available: get_rules, modify_rule"}`
			}
		default:
			handled = false
			return ""
		}
	}()
	return result, handled
}

// mdnsAutoRegister parses the JSON result from MDNSScan, bulk-inserts every discovered
// device into the inventory, and appends a registration summary to the result JSON.
func mdnsAutoRegister(scanJSON string, db *sql.DB, deviceType string, tags []string, overwrite bool, logger *slog.Logger) string {
	if deviceType == "" {
		deviceType = "mdns-device"
	}

	var result struct {
		Status  string              `json:"status"`
		Count   int                 `json:"count"`
		Devices []tools.MDNSService `json:"devices"`
		Message string              `json:"message,omitempty"`
	}
	if err := json.Unmarshal([]byte(scanJSON), &result); err != nil || result.Status != "success" || len(result.Devices) == 0 {
		return scanJSON // nothing to register
	}

	var created, updated, skipped int
	for _, dev := range result.Devices {
		name := mdnsCleanHostname(dev.Host)
		if name == "" {
			name = mdnsCleanName(dev.Name)
		}
		ip := ""
		if len(dev.IPs) > 0 {
			ip = dev.IPs[0]
			if net.ParseIP(ip) == nil {
				logger.Warn("mdns auto-register: skipping device with invalid IP", "host", name, "ip", ip)
				skipped++
				continue
			}
		}
		desc := dev.Info
		// Try to enrich with MAC address from ARP cache (best-effort, non-blocking).
		mac := ""
		if ip != "" {
			var macResult struct {
				Status string `json:"status"`
				MAC    string `json:"mac_address"`
			}
			if merr := json.Unmarshal([]byte(tools.LookupMACAddress(ip, "")), &macResult); merr == nil && macResult.Status == "success" {
				mac = macResult.MAC
			}
		}
		record := inventory.DeviceRecord{
			Name:        name,
			Type:        deviceType,
			IPAddress:   ip,
			Port:        dev.Port,
			Description: desc,
			Tags:        tags,
			MACAddress:  mac,
		}
		c, u, err := inventory.UpsertDeviceByName(db, record, overwrite)
		if err != nil {
			logger.Warn("mdns auto-register: failed to upsert device", "name", name, "error", err)
			skipped++
			continue
		}
		switch {
		case c:
			created++
		case u:
			updated++
		default:
			skipped++
		}
	}

	type regSummary struct {
		Created int `json:"created"`
		Updated int `json:"updated"`
		Skipped int `json:"skipped"`
	}
	out := map[string]interface{}{
		"status":        result.Status,
		"count":         result.Count,
		"devices":       result.Devices,
		"auto_register": regSummary{Created: created, Updated: updated, Skipped: skipped},
	}
	if result.Message != "" {
		out["message"] = result.Message
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// mdnsCleanHostname strips the ".local." suffix from a mDNS hostname.
func mdnsCleanHostname(host string) string {
	h := strings.TrimSuffix(host, ".")
	h = strings.TrimSuffix(h, ".local")
	return h
}

// mdnsCleanName strips the service-type suffix from a full mDNS service name.
func mdnsCleanName(name string) string {
	if idx := strings.Index(name, "._"); idx > 0 {
		return name[:idx]
	}
	return strings.TrimSuffix(name, ".")
}
