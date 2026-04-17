package agent

import "context"

// dispatchInfra handles network, cloud platform, and external service tool calls
// (co_agent, mdns, tts, chromecast, proxmox, ollama, tailscale, ansible, invasion, github, netlify, mqtt, mcp, adguard, firewall).
func dispatchInfra(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	if result, handled := dispatchNetwork(ctx, tc, dc); handled {
		return result, true
	}
	if result, handled := dispatchCloud(ctx, tc, dc); handled {
		return result, true
	}
	if result, handled := dispatchPlatform(ctx, tc, dc); handled {
		return result, true
	}
	return "", false
}
