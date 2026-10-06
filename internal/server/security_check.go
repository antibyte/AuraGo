package server

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"slices"
	"strings"

	"aurago/internal/config"
	"aurago/internal/mqtt"
	"aurago/internal/sandbox"

	"gopkg.in/yaml.v3"
)

var runtimeGOOS = runtime.GOOS

// Severity levels for security hints.
const (
	SevCritical = "critical"
	SevWarning  = "warning"
	SevInfo     = "info"
)

// SecurityHint describes a single security issue found in the current configuration.
// AutoFixable hints carry a FixPatch that ApplyHardening can merge into config.yaml.
type SecurityHint struct {
	ID          string                 `json:"id"`
	Severity    string                 `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	AutoFixable bool                   `json:"auto_fixable"`
	FixPatch    map[string]interface{} `json:"-"` // applied server-side only, never serialised
}

// isInternetFacing returns true when the server likely has public internet exposure.
// LAN binds and private tailnet access are handled by isNetworkFacing instead.
func isInternetFacing(cfg *config.Config) bool {
	return hasPublicInternetExposure(cfg)
}

// isNetworkFacing returns true when the server is reachable from any non-localhost
// network, including LAN and tailnet access. This is broader than public internet
// exposure and is used for local-network hygiene warnings such as plain HTTP.
func isNetworkFacing(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if hasPublicInternetExposure(cfg) {
		return true
	}
	h := cfg.Server.Host
	if h == "0.0.0.0" || h == "::" || h == "" {
		return true
	}
	if hostLooksNetworkFacing(h) {
		return true
	}
	// Tailscale Serve makes services reachable inside the private tailnet only.
	if cfg.Tailscale.TsNet.Enabled && (cfg.Tailscale.TsNet.ServeHTTP || cfg.Tailscale.TsNet.ExposeHomepage || cfg.Tailscale.TsNet.ExposeManifest || cfg.Tailscale.TsNet.ExposeSpaceAgent) {
		return true
	}
	// Security Proxy binds on a network interface even when it only serves LAN clients.
	if cfg.SecurityProxy.Enabled {
		return true
	}
	return false
}

func hasPublicInternetExposure(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	// Cloudflare Tunnel exposing the Web UI routes public internet traffic directly to AuraGo.
	if cfg.CloudflareTunnel.Enabled && cfg.CloudflareTunnel.ExposeWebUI {
		return true
	}
	for _, rule := range cfg.CloudflareTunnel.CustomIngress {
		if strings.TrimSpace(rule.Hostname) != "" && ingressTargetsAuraGoWebUI(rule.Service, cfg) {
			return true
		}
	}
	// Tailscale Funnel is public internet exposure; normal tsnet Serve is tailnet-only.
	if cfg.Tailscale.TsNet.Enabled && cfg.Tailscale.TsNet.Funnel {
		return true
	}
	// A public-looking domain with ACME/proxy TLS usually means the operator is
	// publishing the instance beyond the LAN. Self-signed HTTPS and .ts.net names
	// are intentionally not treated as public by themselves.
	if cfg.Server.HTTPS.Enabled && strings.EqualFold(cfg.Server.HTTPS.CertMode, "auto") && publicHostnameLikely(cfg.Server.HTTPS.Domain) {
		return true
	}
	if cfg.SecurityProxy.Enabled && publicHostnameLikely(cfg.SecurityProxy.Domain) {
		return true
	}
	return false
}

func ingressTargetsAuraGoWebUI(service string, cfg *config.Config) bool {
	service = strings.ToLower(strings.TrimSpace(service))
	if service == "" {
		return false
	}
	webPorts := []string{
		fmt.Sprintf(":%d", cfg.Server.Port),
		fmt.Sprintf(":%d", cfg.Server.HTTPS.HTTPSPort),
		fmt.Sprintf(":%d", cfg.CloudflareTunnel.LoopbackPort),
	}
	for _, marker := range []string{"localhost", "127.0.0.1", "[::1]"} {
		if strings.Contains(service, marker) {
			for _, port := range webPorts {
				if port != ":0" && strings.Contains(service, port) {
					return true
				}
			}
		}
	}
	return false
}

func hostLooksNetworkFacing(host string) bool {
	host = stripHostPort(host)
	if host == "" {
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback()
	}
	return true
}

func publicHostnameLikely(host string) bool {
	host = strings.ToLower(strings.Trim(strings.TrimSpace(stripHostPort(host)), "."))
	if host == "" || host == "localhost" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return publicIPLikely(ip)
	}
	if !strings.Contains(host, ".") {
		return false
	}
	privateSuffixes := []string{
		".local", ".localhost", ".lan", ".home", ".internal", ".localdomain",
		".test", ".invalid", ".example", ".ts.net",
	}
	for _, suffix := range privateSuffixes {
		if strings.HasSuffix(host, suffix) {
			return false
		}
	}
	return true
}

func publicIPLikely(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return !ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!isCGNATIP(ip)
}

func isCGNATIP(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127
}

func stripHostPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(parsedHost, "[]")
	}
	return strings.Trim(host, "[]")
}

// weakAuth returns true when no effective authentication is configured.
func weakAuth(cfg *config.Config) bool {
	return !cfg.Auth.Enabled || cfg.Auth.PasswordHash == ""
}

// CheckSecurity evaluates the current config and returns a list of security hints.
// The returned slice is ordered from most to least severe.
func CheckSecurity(cfg *config.Config) []SecurityHint {
	var hints []SecurityHint
	facing := isInternetFacing(cfg)
	networkFacing := isNetworkFacing(cfg)
	dangerCount := countDangerZoneCapabilities(cfg)

	// 1. auth_disabled
	if !cfg.Auth.Enabled {
		sev := SevWarning
		desc := "Authentication is disabled. Any visitor with network access can use the agent."
		if facing {
			sev = SevCritical
			desc = "Authentication is disabled but the server is reachable from the internet. " +
				"Anyone can read your conversations, execute tools, and access all integrations."
		}
		hints = append(hints, SecurityHint{
			ID: "auth_disabled", Severity: sev,
			Title:       "Authentication disabled",
			Description: desc,
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"auth": map[string]interface{}{"enabled": true}},
		})
	}

	// 2. no_password — auth enabled but no password hash configured
	if cfg.Auth.Enabled && cfg.Auth.PasswordHash == "" {
		hints = append(hints, SecurityHint{
			ID: "no_password", Severity: SevCritical,
			Title: "No password configured",
			Description: "Login guard is enabled but no password has been set. " +
				"Normal access is locked until a password is configured. Set a password in the Login Guard section.",
			AutoFixable: false,
		})
	}

	// 2b. critical_public_exposure — public instance with weak perimeter and multiple powerful capabilities
	if facing &&
		(!cfg.Server.HTTPS.Enabled || !cfg.Auth.Enabled || cfg.Auth.PasswordHash == "") &&
		dangerCount >= 2 {
		hints = append(hints, SecurityHint{
			ID: "critical_public_exposure", Severity: SevCritical,
			Title: "Public instance is broadly exposed",
			Description: "The server is reachable from the public internet, key perimeter protections are missing, " +
				"and multiple high-risk capabilities are enabled. Lock down access before using AuraGo on this network.",
			AutoFixable: false,
		})
	}

	// 3. no_https — network-facing without TLS
	if networkFacing && !cfg.Server.HTTPS.Enabled &&
		cfg.Server.Host != "127.0.0.1" && cfg.Server.Host != "localhost" {
		hints = append(hints, SecurityHint{
			ID: "no_https", Severity: SevWarning,
			Title: "No HTTPS configured",
			Description: "The server appears to be accessible from outside localhost but is running " +
				"plain HTTP. Credentials and session cookies are transmitted unencrypted.",
			AutoFixable: false,
		})
	}

	// 4. a2a_no_auth — A2A dedicated server port open without any auth
	if cfg.A2A.Server.Port > 0 && !cfg.A2A.Auth.APIKeyEnabled && !cfg.A2A.Auth.BearerEnabled {
		hints = append(hints, SecurityHint{
			ID: "a2a_no_auth", Severity: SevWarning,
			Title: "A2A server running without authentication",
			Description: fmt.Sprintf(
				"The Agent-to-Agent server is listening on a dedicated port (%d) with no API key or Bearer token configured.",
				cfg.A2A.Server.Port,
			),
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"a2a": map[string]interface{}{"auth": map[string]interface{}{"api_key_enabled": true}}},
		})
	}

	// 5. insecure_ssh_key — SSH host key verification disabled
	if cfg.RemoteControl.SSHInsecureHostKey {
		hints = append(hints, SecurityHint{
			ID: "insecure_ssh_key", Severity: SevWarning,
			Title: "SSH host key verification disabled",
			Description: "remote_control.ssh_insecure_host_key is true. SSH connections accept any host key — " +
				"this enables man-in-the-middle attacks.",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"remote_control": map[string]interface{}{"ssh_insecure_host_key": false}},
		})
	}

	// 7. danger_all_public — all danger-zone capabilities enabled on a public server
	if facing &&
		cfg.Agent.AllowShell &&
		cfg.Agent.AllowPython &&
		cfg.Agent.AllowFilesystemWrite &&
		cfg.Agent.AllowNetworkRequests &&
		cfg.Agent.AllowRemoteShell {
		hints = append(hints, SecurityHint{
			ID: "danger_all_public", Severity: SevWarning,
			Title: "All danger-zone capabilities enabled on public server",
			Description: "Shell execution, Python, filesystem write, network requests, and remote shell are all enabled. " +
				"Consider disabling capabilities you don't need and enabling the LLM Guardian.",
			AutoFixable: false,
		})
	}

	// 7b. python_no_sandbox — Python is enabled without an effective sandbox backend
	if cfg.Agent.AllowPython && !pythonSandboxReady(cfg) {
		sev := SevWarning
		if facing {
			sev = SevCritical
		}
		hints = append(hints, SecurityHint{
			ID: "python_no_sandbox", Severity: sev,
			Title: "Python execution without effective sandbox",
			Description: "Python execution is enabled, but the isolated sandbox backend is not effectively available. " +
				"Enable the sandbox or disable Python execution for this environment.",
			AutoFixable: false,
		})
	}

	// 7c. shell_no_sandbox — Shell is enabled without an effective shell sandbox
	if cfg.Agent.AllowShell && !shellSandboxReady(cfg) {
		sev := SevWarning
		if facing {
			sev = SevCritical
		}
		description := "Shell execution is enabled, but the Landlock shell sandbox is not effectively available. " +
			"Unisolated commands run with the AuraGo process user's permissions and can bypass Desktop Notes protection in native file tools. " +
			"Enable shell_sandbox on a supported Linux host, or disable shell execution for this environment."
		if runtimeGOOS != "windows" {
			description += " Without agent.allow_unsandboxed_shell or agent.allow_unsafe_host_execution the Linux/macOS host shell is refused."
		}
		hints = append(hints, SecurityHint{
			ID: "shell_no_sandbox", Severity: sev,
			Title:       "Shell execution without effective sandbox",
			Description: description,
			AutoFixable: false,
		})
	}

	// 7c2. shell_unsafe_host_legacy — host shell permitted by the pre-flag default.
	// A blocked sandbox refuses the shell anyway, so the hint would be wrong there.
	if cfg.Agent.AllowShell && cfg.Agent.LegacyUnsandboxedShell && !cfg.Agent.AllowUnsafeHostExecution && !cfg.Agent.AllowUnsandboxedShell && !shellSandboxReady(cfg) && !sandbox.IsBlocked() && runtimeGOOS != "windows" {
		hints = append(hints, SecurityHint{
			ID: "shell_unsafe_host_legacy", Severity: SevWarning,
			Title: "Host shell runs under a legacy default",
			Description: "agent.allow_shell is enabled, no shell sandbox is active and agent.allow_unsandboxed_shell is not written in config.yaml. " +
				"AuraGo keeps the Linux/macOS host shell working for this existing configuration. Write agent.allow_unsandboxed_shell: true to make the decision explicit, " +
				"or enable shell_sandbox on a supported Linux host. Writing the key as false stops the host shell unless agent.allow_unsafe_host_execution is true.",
			AutoFixable: false,
		})
	}

	// 7d. mcp_public — MCP exposed on an internet-facing instance
	if facing && cfg.Agent.AllowMCP && cfg.MCP.Enabled {
		hints = append(hints, SecurityHint{
			ID: "mcp_public", Severity: SevWarning,
			Title: "MCP enabled on an internet-facing instance",
			Description: "Model Context Protocol is enabled while the server is reachable from the public internet. " +
				"Expose it only behind authentication and transport security, or disable MCP if not needed.",
			AutoFixable: false,
		})
	}

	// 8. meshcentral_insecure — TLS verification disabled
	if cfg.MeshCentral.Enabled && cfg.MeshCentral.Insecure {
		hints = append(hints, SecurityHint{
			ID: "meshcentral_insecure", Severity: SevWarning,
			Title: "MeshCentral: TLS verification disabled",
			Description: "meshcentral.insecure is true. TLS certificate errors are ignored, " +
				"enabling potential man-in-the-middle attacks.",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"meshcentral": map[string]interface{}{"insecure": false}},
		})
	}

	// 9. proxmox_insecure — TLS verification disabled
	if cfg.Proxmox.Enabled && cfg.Proxmox.Insecure {
		hints = append(hints, SecurityHint{
			ID: "proxmox_insecure", Severity: SevWarning,
			Title: "Proxmox: TLS verification disabled",
			Description: "proxmox.insecure is true. TLS certificate errors are ignored, " +
				"enabling potential man-in-the-middle attacks.",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"proxmox": map[string]interface{}{"insecure": false}},
		})
	}

	// 10. totp_disabled — internet-facing with password-only auth
	if cfg.Auth.Enabled && cfg.Auth.PasswordHash != "" && !cfg.Auth.TOTPEnabled && facing {
		hints = append(hints, SecurityHint{
			ID: "totp_disabled", Severity: SevInfo,
			Title: "Two-factor authentication not enabled",
			Description: "The server is internet-facing but only password authentication is active. " +
				"Enable TOTP 2FA in the Login Guard section.",
			AutoFixable: false,
		})
	}

	// ── P0: Critical – Exposure without authentication ──────────────────────

	// 11. cf_tunnel_expose_webui_no_auth — Cloudflare Tunnel exposes the Web UI without auth
	if cfg.CloudflareTunnel.Enabled && cfg.CloudflareTunnel.ExposeWebUI && weakAuth(cfg) {
		hints = append(hints, SecurityHint{
			ID: "cf_tunnel_expose_webui_no_auth", Severity: SevCritical,
			Title: "Cloudflare Tunnel: Web UI exposed without authentication",
			Description: "The Cloudflare Tunnel is routing the AuraGo Web UI to the internet, " +
				"but no login password is configured. Anyone with the tunnel URL has full access to the agent.",
			AutoFixable: false,
		})
	}

	// 12. tsnet_funnel_no_auth — Tailscale Funnel exposes the instance publicly without auth
	if cfg.Tailscale.TsNet.Enabled && cfg.Tailscale.TsNet.Funnel && weakAuth(cfg) {
		hints = append(hints, SecurityHint{
			ID: "tsnet_funnel_no_auth", Severity: SevCritical,
			Title: "Tailscale Funnel: public access without authentication",
			Description: "Tailscale Funnel is active and makes this instance reachable from the public internet, " +
				"but no login password is configured. Disable Funnel or enable authentication.",
			AutoFixable: false,
		})
	}

	// 13. security_proxy_enabled_no_auth — Security Proxy terminates TLS on 0.0.0.0 without auth
	if cfg.SecurityProxy.Enabled && weakAuth(cfg) {
		hints = append(hints, SecurityHint{
			ID: "security_proxy_enabled_no_auth", Severity: SevCritical,
			Title: "Security Proxy is active without authentication",
			Description: "The built-in Security Proxy (Caddy) is enabled and binds on 0.0.0.0:443, " +
				"making AuraGo reachable from the network. No login password is configured — " +
				"anyone on the network has full access.",
			AutoFixable: false,
		})
	}

	// 14. homepage_webserver_public_no_auth — Homepage WebServer is public without auth
	if cfg.Homepage.WebServerEnabled && !cfg.Homepage.WebServerInternalOnly && weakAuth(cfg) {
		hints = append(hints, SecurityHint{
			ID: "homepage_webserver_public_no_auth", Severity: SevCritical,
			Title: "Homepage web server is publicly accessible without authentication",
			Description: "The Homepage web server is enabled and not restricted to localhost. " +
				"AuraGo has no login password configured. Anyone who can reach the server " +
				"can access the interface and the agent.",
			AutoFixable: false,
		})
	}

	// 15. remote_control_auto_approve — any device can join without approval
	if cfg.RemoteControl.Enabled && cfg.RemoteControl.AutoApprove {
		hints = append(hints, SecurityHint{
			ID: "remote_control_auto_approve", Severity: SevCritical,
			Title: "Remote Control: new devices are auto-approved",
			Description: "remote_control.auto_approve is enabled. Any device that connects to the " +
				"remote control endpoint is automatically granted access without manual confirmation. " +
				"Disable auto-approve and review devices manually.",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"remote_control": map[string]interface{}{"auto_approve": false}},
		})
	}

	// ── P1: Warning – Elevated risk ─────────────────────────────────────────

	// 16. tsnet_funnel_danger_zone — Tailscale Funnel + multiple dangerous capabilities
	if cfg.Tailscale.TsNet.Enabled && cfg.Tailscale.TsNet.Funnel && dangerCount >= 2 {
		hints = append(hints, SecurityHint{
			ID: "tsnet_funnel_danger_zone", Severity: SevWarning,
			Title: "Tailscale Funnel enabled with multiple high-risk capabilities",
			Description: fmt.Sprintf(
				"Tailscale Funnel exposes this instance to the public internet and %d high-risk "+
					"capabilities (shell, Python, filesystem write, etc.) are active. "+
					"Reduce the capability surface or restrict access to the tailnet only.",
				dangerCount,
			),
			AutoFixable: false,
		})
	}

	// 17. discord_bot_no_user_restriction — Discord bot responds to everyone
	if cfg.Discord.Enabled && cfg.Discord.AllowedUserID == "" {
		hints = append(hints, SecurityHint{
			ID: "discord_bot_no_user_restriction", Severity: SevWarning,
			Title: "Discord bot: no user restriction configured",
			Description: "The Discord bot is active but allowed_user_id is not set. " +
				"Any Discord user who can message the bot can interact with the agent. " +
				"Set allowed_user_id to your Discord user ID.",
			AutoFixable: false,
		})
	}

	// 18. telegram_bot_no_user_id — Telegram bot responds to everyone
	if cfg.Telegram.BotToken != "" && cfg.Telegram.UserID == 0 {
		hints = append(hints, SecurityHint{
			ID: "telegram_bot_no_user_id", Severity: SevWarning,
			Title: "Telegram bot: no user ID restriction configured",
			Description: "The Telegram bot token is set but telegram.user_id is 0. " +
				"The bot will respond to messages from any Telegram user. " +
				"Set your Telegram user ID to restrict access.",
			AutoFixable: false,
		})
	}

	// 19. docker_tcp_socket — Docker exposed over unencrypted TCP
	if cfg.Docker.Enabled && strings.HasPrefix(cfg.Docker.Host, "tcp://") {
		hints = append(hints, SecurityHint{
			ID: "docker_tcp_socket", Severity: SevWarning,
			Title: "Docker: using unencrypted TCP socket",
			Description: "docker.host is set to a tcp:// address without TLS. " +
				"Anyone who can reach that address has unauthenticated root-equivalent access to the Docker daemon. " +
				"Use a Unix socket or enable TLS on the TCP endpoint.",
			AutoFixable: false,
		})
	}

	// 20. docker_enabled_no_readonly — Docker write access on network-facing instance
	if cfg.Docker.Enabled && !cfg.Docker.ReadOnly && networkFacing {
		hints = append(hints, SecurityHint{
			ID: "docker_enabled_no_readonly", Severity: SevWarning,
			Title: "Docker: write access enabled on network-facing instance",
			Description: "The Docker integration has full write access (create, start, stop, remove containers) " +
				"and this instance is reachable from the network. Enable docker.readonly to limit the " +
				"agent to read-only Docker operations.",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"docker": map[string]interface{}{"readonly": true}},
		})
	}

	if cfg.NetworkShares.Enabled && !cfg.NetworkShares.ReadOnly &&
		(cfg.NetworkShares.AllowCreate || cfg.NetworkShares.AllowUpdate || cfg.NetworkShares.AllowDelete) {
		scope := "The local SMB/NFS integration may change host share definitions."
		if networkFacing {
			scope += " This AuraGo instance is reachable from the network."
		}
		if cfg.NetworkShares.AllowDelete {
			scope += " Delete permission is enabled; it removes share definitions but never directories or their contents."
		}
		hints = append(hints, SecurityHint{
			ID:          "network_shares_write_enabled",
			Severity:    SevWarning,
			Title:       "Local network-share writes enabled",
			Description: scope + " Keep allowed roots, SMB principals, and NFS clients narrowly scoped.",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"network_shares": map[string]interface{}{"readonly": true}},
		})
	}

	// 21. self_update_public — self-update enabled on internet-facing instance
	if facing && cfg.Agent.AllowSelfUpdate {
		hints = append(hints, SecurityHint{
			ID: "self_update_public", Severity: SevWarning,
			Title: "Self-update enabled on internet-facing instance",
			Description: "allow_self_update is enabled and the server is reachable from the network. " +
				"A compromised LLM response could trigger a binary replacement. " +
				"Disable self-update or ensure the LLM Guardian is active.",
			AutoFixable: false,
		})
	}

	// 22. llm_guardian_disabled_danger_zone — multiple dangerous tools but no Guardian
	if dangerCount >= 2 && !cfg.LLMGuardian.Enabled {
		hints = append(hints, SecurityHint{
			ID: "llm_guardian_disabled_danger_zone", Severity: SevWarning,
			Title: "LLM Guardian disabled with multiple high-risk capabilities active",
			Description: fmt.Sprintf(
				"%d high-risk capabilities are enabled (shell, Python, filesystem write, etc.) "+
					"but the LLM Guardian is disabled. The Guardian is the last safeguard that "+
					"inspects LLM-requested tool calls before execution.",
				dangerCount,
			),
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"llm_guardian": map[string]interface{}{"enabled": true}},
		})
	}

	if cfg.LLMGuardian.Enabled && strings.TrimSpace(cfg.LLMGuardian.Provider) != "" && cfg.FindProvider(strings.TrimSpace(cfg.LLMGuardian.Provider)) == nil {
		hints = append(hints, SecurityHint{
			ID:       "llm_guardian_provider_missing",
			Severity: SevWarning,
			Title:    "LLM Guardian provider not found",
			Description: "llm_guardian.provider references a provider entry that does not exist. " +
				"AuraGo will fallback to the main LLM for Guardian checks; fix the provider ID or clear the field intentionally.",
			AutoFixable: false,
		})
	}

	// 23. webhooks_no_rate_limit — public webhook endpoints without rate limiting
	if cfg.Webhooks.Enabled && cfg.Webhooks.RateLimit == 0 && facing {
		hints = append(hints, SecurityHint{
			ID: "webhooks_no_rate_limit", Severity: SevWarning,
			Title: "Webhooks: no rate limit configured",
			Description: "Incoming webhook endpoints are enabled on an internet-facing instance, " +
				"but webhooks.rate_limit is 0 (unlimited). Without rate limiting, endpoints are " +
				"vulnerable to flood attacks. Set a sensible requests-per-minute limit.",
			AutoFixable: false,
		})
	}

	// 24. mqtt_relay_no_auth — broker traffic may start agent runs (relay_to_agent,
	// Frigate relays, MQTT-triggered missions) although AuraGo logs in with
	// neither a username nor a client certificate over TLS. Critical while
	// allow_unauthenticated_relay (written or grandfathered at load) lets it run;
	// with the flag false the relay is refused and a configured relay is
	// reported as a warning.
	if cfg.MQTT.Enabled && !config.MQTTBrokerAuthenticated(cfg) {
		if cfg.MQTT.AllowUnauthenticatedRelay {
			hints = append(hints, SecurityHint{
				ID: "mqtt_relay_no_auth", Severity: SevCritical,
				Title: "MQTT: agent runs accepted from an unauthenticated broker",
				Description: "mqtt.allow_unauthenticated_relay is true and AuraGo connects to the broker with neither a username nor a client certificate over TLS. " +
					"Any host that can publish to the broker can start agent runs through relay_to_agent, the Frigate relays and MQTT-triggered missions. " +
					"Set a broker username/password or a client certificate over TLS; allow_unauthenticated_relay: true accepts that any LAN host can start agent runs. " +
					"Configurations from before this check were allowed automatically; if you use none of these, set the key to false.",
				AutoFixable: false,
			})
		} else if cfg.MQTT.RelayToAgent || mqtt.FrigateRelayEnabled(cfg) {
			hints = append(hints, SecurityHint{
				ID: "mqtt_relay_no_auth", Severity: SevWarning,
				Title: "MQTT: relay refused without broker authentication",
				Description: "relay_to_agent or a Frigate relay is enabled, but AuraGo connects to the broker with neither a username nor a client certificate over TLS " +
					"and mqtt.allow_unauthenticated_relay is false, so AuraGo does not start agent runs from broker messages, MQTT-triggered missions included. " +
					"Configure mqtt.username with the Vault password, or a client certificate over TLS (mqtt.tls.cert_file with an mqtts:// or ssl:// broker URL), to enable the relay.",
				AutoFixable: false,
			})
		}
	}

	// 24b. sip_auto_answer_all_interfaces / sip_wildcard_callers_cidr_peers — an
	// auto-answering agent route reachable on every interface, or a wildcard caller
	// list whose only network check is a whole subnet.
	if cfg.SIP.Enabled && !cfg.SIP.ReadOnly && cfg.SIP.Permissions.AnswerInbound {
		route := strings.ToLower(strings.TrimSpace(cfg.SIP.Inbound.Route))
		if route == "agent" && sipBindsAllInterfaces(cfg.SIP.BindHost) {
			hints = append(hints, SecurityHint{
				ID: "sip_auto_answer_all_interfaces", Severity: SevWarning,
				Title: "SIP: agent auto-answers on all interfaces",
				Description: "sip.inbound.route is \"agent\" and the SIP socket binds to every interface. " +
					"Any host that can reach the socket and pass the peer and caller lists is answered automatically. " +
					"Set sip.bind_host to the concrete LAN address that reaches your phone system unless NAT or Tailscale needs the wildcard, " +
					"or use route \"manual\" until auto-answer is intended.",
				AutoFixable: false,
			})
		}
		if route != "reject" && sipCallersWildcard(cfg.SIP.Inbound.AllowedCallers) && sipPeersIncludeSubnet(cfg.SIP.Inbound.TrustedPeerCIDRs) {
			hints = append(hints, SecurityHint{
				ID: "sip_wildcard_callers_cidr_peers", Severity: SevWarning,
				Title: "SIP: every caller accepted from a whole subnet",
				Description: "sip.inbound.allowed_callers contains a pattern that matches every caller (such as \"*\" or \"*@*\") and sip.inbound.trusted_peer_cidrs lists a subnet. " +
					"Caller identity then rests on the source address alone. " +
					"List the exact registrar or PBX IP instead of a subnet, or name the callers.",
				AutoFixable: false,
			})
		}
	}

	// 24c. fritzbox_plaintext_sessions — over HTTP, Fritz!Box session IDs travel in
	// clear text on read and write paths alike, and a sniffed SID carries the
	// Fritz!Box account's rights, not AuraGo's readonly limits. The hint fires once
	// AuraGo itself is allowed to write.
	if cfg.FritzBox.Enabled && !cfg.FritzBox.HTTPS && fritzBoxAnyGroupWritable(cfg) {
		hints = append(hints, SecurityHint{
			ID: "fritzbox_plaintext_sessions", Severity: SevWarning,
			Title: "Fritz!Box control over plain HTTP",
			Description: "fritzbox.https is false while at least one feature group allows writes. " +
				"Session IDs and digest exchanges cross the LAN unencrypted on reads and writes alike. " +
				"AuraGo's readonly flags limit AuraGo, not a captured session, which carries the rights of the Fritz!Box account. " +
				"Set fritzbox.https: true first (TR-064 moves from port 49000 to 49443; set fritzbox.insecure_skip_verify for the box's self-signed certificate), " +
				"and log in with a Fritz!Box user restricted to the rights AuraGo needs.",
			AutoFixable: false,
		})
	}

	// 25. telnyx_no_allowed_numbers — Telnyx has no permitted inbound/outbound numbers
	if cfg.Telnyx.Enabled && len(cfg.Telnyx.AllowedNumbers) == 0 {
		hints = append(hints, SecurityHint{
			ID: "telnyx_no_allowed_numbers", Severity: SevWarning,
			Title: "Telnyx: no allowed numbers configured",
			Description: "The Telnyx integration is enabled but telnyx.allowed_numbers is empty. " +
				"Inbound calls/SMS and outbound notifications are blocked until at least one " +
				"E.164 number is explicitly allowed.",
			AutoFixable: false,
		})
	}

	// ── P2: Informational ────────────────────────────────────────────────────

	// 26. server_host_wildcard_no_https — wildcard bind without any TLS or proxy
	if (cfg.Server.Host == "0.0.0.0" || cfg.Server.Host == "") &&
		!cfg.Server.HTTPS.Enabled &&
		!cfg.SecurityProxy.Enabled &&
		!cfg.Tailscale.TsNet.Enabled &&
		!cfg.CloudflareTunnel.Enabled {
		hints = append(hints, SecurityHint{
			ID: "server_host_wildcard_no_https", Severity: SevWarning,
			Title: "Server bound to all interfaces without TLS or proxy",
			Description: "The server listens on 0.0.0.0 (all network interfaces) over plain HTTP, " +
				"with no reverse proxy, Cloudflare Tunnel, or Tailscale providing TLS. " +
				"Credentials and session cookies travel unencrypted on the network.",
			AutoFixable: false,
		})
	}

	// 27. auth_no_lockout — authentication enabled but no brute-force protection
	if cfg.Auth.Enabled && cfg.Auth.MaxLoginAttempts <= 0 {
		hints = append(hints, SecurityHint{
			ID: "auth_no_lockout", Severity: SevWarning,
			Title: "Login lockout is disabled",
			Description: "auth.max_login_attempts is 0 or negative, meaning there is no limit on " +
				"failed login attempts. This allows unlimited brute-force attacks against the password. " +
				"Set max_login_attempts to a sensible value (e.g. 10).",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"auth": map[string]interface{}{"max_login_attempts": 10}},
		})
	}

	return hints
}

// ApplyHardening merges the fix patches for the given hint IDs into config.yaml and hot-reloads.
// Returns the list of hint IDs that were actually applied.
func ApplyHardening(s *Server, ids []string) ([]string, error) {
	wantedSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		wantedSet[id] = true
	}

	s.CfgMu.RLock()
	hints := CheckSecurity(s.Cfg)
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()

	combined := map[string]interface{}{}
	applied := []string{}
	for _, h := range hints {
		if !h.AutoFixable || !wantedSet[h.ID] || len(h.FixPatch) == 0 {
			continue
		}
		deepMerge(combined, h.FixPatch, "")
		applied = append(applied, h.ID)
	}
	if len(applied) == 0 {
		return nil, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if rawCfg == nil {
		rawCfg = map[string]interface{}{}
	}
	deepMerge(rawCfg, combined, "")

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	// Validate before writing
	var validateCfg config.Config
	if err := yaml.Unmarshal(out, &validateCfg); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Hot-reload: use the same pattern as handleUpdateConfig
	s.CfgMu.Lock()
	if newCfg, loadErr := config.Load(configPath); loadErr == nil {
		newCfg.ApplyVaultSecrets(s.Vault)
		newCfg.ResolveProviders()
		newCfg.ApplyOAuthTokens(s.Vault)
		newCfg.Runtime = s.Cfg.Runtime
		newCfg.ConfigPath = configPath
		s.replaceConfigSnapshot(newCfg)
	}
	s.CfgMu.Unlock()

	s.Logger.Info("[Security] Hardening applied", "ids", strings.Join(applied, ", "))
	return applied, nil
}

func countDangerZoneCapabilities(cfg *config.Config) int {
	count := 0
	for _, enabled := range []bool{
		cfg.Agent.AllowShell,
		cfg.Agent.AllowPython,
		cfg.Agent.AllowFilesystemWrite,
		cfg.Agent.AllowNetworkRequests,
		cfg.Agent.AllowRemoteShell,
		cfg.Agent.AllowSelfUpdate,
		cfg.Agent.AllowMCP && cfg.MCP.Enabled,
		cfg.Agent.SudoEnabled,
		cfg.Agent.SudoUnrestricted,
		cfg.NetworkShares.Enabled && !cfg.NetworkShares.ReadOnly &&
			(cfg.NetworkShares.AllowCreate || cfg.NetworkShares.AllowUpdate || cfg.NetworkShares.AllowDelete),
	} {
		if enabled {
			count++
		}
	}
	return count
}

// sipBindsAllInterfaces reports whether the SIP socket listens on every interface.
func sipBindsAllInterfaces(bindHost string) bool {
	bindHost = strings.TrimSpace(bindHost)
	if bindHost == "" {
		return true
	}
	ip := net.ParseIP(strings.Trim(bindHost, "[]"))
	return ip != nil && ip.IsUnspecified()
}

// sipCallersWildcard reports whether the caller allowlist admits every caller.
// It mirrors the wildcardMatch syntax in internal/sipphone/policy.go, where '*'
// matches any sequence and '?' one character, and patterns are matched against
// the canonical "sip:user@host" form: a pattern (optionally "sip:"-prefixed)
// whose user and domain parts each hold only wildcards with at least one '*',
// such as "*", "**", "*@*" or "sip:*@*", matches every caller.
func sipCallersWildcard(callers []string) bool {
	return slices.ContainsFunc(callers, func(caller string) bool {
		caller = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(caller)), "sip:")
		parts := strings.Split(caller, "@")
		if len(parts) > 2 {
			return false
		}
		for _, part := range parts {
			if !strings.Contains(part, "*") || strings.Trim(part, "*?") != "" {
				return false
			}
		}
		return true
	})
}

// sipPeersIncludeSubnet reports whether a trusted peer entry covers more than one
// address. A /32 or /128 entry names a single host and does not count.
func sipPeersIncludeSubnet(peers []string) bool {
	for _, peer := range peers {
		_, network, err := net.ParseCIDR(strings.TrimSpace(peer))
		if err != nil {
			continue
		}
		if ones, bits := network.Mask.Size(); ones < bits {
			return true
		}
	}
	return false
}

// fritzBoxAnyGroupWritable reports whether an enabled Fritz!Box feature group may
// change router state. The agent gates every write on the group being enabled
// first; the TV group has no write actions.
func fritzBoxAnyGroupWritable(cfg *config.Config) bool {
	fb := cfg.FritzBox
	return (fb.System.Enabled && !fb.System.ReadOnly) ||
		(fb.Network.Enabled && !fb.Network.ReadOnly) ||
		(fb.Telephony.Enabled && !fb.Telephony.ReadOnly) ||
		(fb.SmartHome.Enabled && !fb.SmartHome.ReadOnly) ||
		(fb.Storage.Enabled && !fb.Storage.ReadOnly)
}

func pythonSandboxReady(cfg *config.Config) bool {
	if !cfg.Sandbox.Enabled {
		return false
	}
	if cfg.Runtime.IsDocker && !cfg.Runtime.DockerSocketOK {
		return false
	}
	return true
}

func shellSandboxReady(cfg *config.Config) bool {
	if !cfg.ShellSandbox.Enabled {
		return false
	}
	if runtimeGOOS != "linux" {
		return false
	}
	if cfg.Runtime.IsDocker {
		return false
	}
	sb := sandbox.Get()
	return sb != nil && sb.Available()
}
