package audit

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// credentialedRedirectAllowlist names production files whose credentialed
// HTTP client legitimately has to follow cross-origin redirects, with the
// reason. Keep it empty unless a vendor documents such a redirect.
var credentialedRedirectAllowlist = map[string]string{}

const (
	credentialedRedirectPendingReason = "credentialed client not yet bound to its origin; pending same-origin binding (tracked in the backlog)"
	credentialedRedirectOAuthReason   = "OAuth flow, needs behaviour review before same-origin binding (tracked in the backlog)"
	credentialedRedirectVendorReason  = "vendor API, needs behaviour review before same-origin binding (tracked in the backlog)"
)

// credentialedRedirectPending is a ratchet of known credentialed clients that
// still follow redirects to any origin. It may only shrink: bind a client,
// then delete its entry. New files are never added here.
var credentialedRedirectPending = map[string]string{
	"internal/a2a/client.go":                          credentialedRedirectPendingReason,
	"internal/agentmail/client.go":                    credentialedRedirectPendingReason,
	"internal/discord/connection.go":                  credentialedRedirectVendorReason,
	"internal/embeddings/llama_docker.go":             credentialedRedirectPendingReason,
	"internal/embeddings/llama_embedder.go":           credentialedRedirectPendingReason,
	"internal/evomap/client.go":                       credentialedRedirectPendingReason,
	"internal/jellyfin/client.go":                     credentialedRedirectPendingReason,
	"internal/realtimespeech/client.go":               credentialedRedirectPendingReason,
	"internal/server/copilot_handlers.go":             credentialedRedirectOAuthReason,
	"internal/server/onedrive_handlers.go":            credentialedRedirectOAuthReason,
	"internal/telnyx/client.go":                       credentialedRedirectPendingReason,
	"internal/tools/adguard.go":                       credentialedRedirectPendingReason,
	"internal/tools/ansible.go":                       credentialedRedirectPendingReason,
	"internal/tools/cloudflare_tunnel.go":             credentialedRedirectVendorReason,
	"internal/tools/github.go":                        credentialedRedirectPendingReason,
	"internal/tools/go2rtc.go":                        credentialedRedirectPendingReason,
	"internal/tools/grafana.go":                       credentialedRedirectPendingReason,
	"internal/tools/integration_connection_checks.go": credentialedRedirectPendingReason,
	"internal/tools/koofr.go":                         credentialedRedirectPendingReason,
	"internal/tools/music_generation.go":              credentialedRedirectPendingReason,
	"internal/tools/netlify.go":                       credentialedRedirectPendingReason,
	"internal/tools/notification.go":                  credentialedRedirectPendingReason,
	"internal/tools/proxmox.go":                       credentialedRedirectPendingReason,
	"internal/tools/space_agent.go":                   credentialedRedirectPendingReason,
	"internal/tools/tailscale.go":                     credentialedRedirectVendorReason,
	"internal/tools/uptime_kuma.go":                   credentialedRedirectPendingReason,
	"internal/tools/vercel.go":                        credentialedRedirectPendingReason,
	"internal/tools/video_generation.go":              credentialedRedirectPendingReason,
	"internal/tools/webdav.go":                        credentialedRedirectPendingReason,
	"internal/truenas/client.go":                      credentialedRedirectPendingReason,
	"internal/virtualcomputers/client.go":             credentialedRedirectPendingReason,
}

// TestCredentialedHTTPClientsBindRedirectsToOrigin keeps credential-bearing
// HTTP clients on the house same-origin redirect policy: a production file that
// builds a raw http.Client (&http.Client{}) or an SSRF-protected client
// (security.NewSSRFProtectedHTTPClient / ...ForURL) and sets an Authorization,
// xi-api-key or cf-aig-authorization header (including SetBasicAuth and
// WithRequestHeader) must set a CheckRedirect policy. Use httporigin.NewClient,
// security.NewSSRFProtectedHTTPClientSameOrigin or an explicit CheckRedirect.
// internal/llm, internal/localllm and internal/httporigin own their policy and
// have their own fixture tests. Files still pending are listed in
// credentialedRedirectPending, which may only shrink.
//
// A second check covers go-openai clients: a file outside internal/llm and
// internal/localllm that calls openai.DefaultConfig must also assign
// .HTTPClient, so the provider client gets an origin-bound http.Client.
//
// This is a tripwire, not a complete check. Known blind spots:
//   - Header names that are not literals are invisible, such as the vault
//     secret headers in internal/tools/webhooks.go and the agent-supplied
//     headers in internal/tools/api_client.go.
//   - Granularity is per file: any CheckRedirect in a file satisfies every
//     client built in that file, and any .HTTPClient assignment satisfies
//     every openai.DefaultConfig call.
//   - A client built in one file and given credentials in another is
//     invisible. imageGenHTTPClient has that shape; it is caught only because
//     image_gen_openrouter.go itself also sets Authorization.
//   - The bare word CheckRedirect anywhere in a file, even in a comment,
//     makes the file count as compliant.
//   - http.Client{ without &, http.DefaultClient and map-style headers
//     (req.Header["Authorization"] = ...) are not matched.
//   - Credential headers outside the three names above are not matched.
//     Unbound today: internal/tools/virustotal.go (x-apikey) and
//     internal/manus/client.go (x-manus-api-key); the tree also uses
//     x-goog-api-key, x-api-key, X-Subscription-Token and X-Auth-Token.
func TestCredentialedHTTPClientsBindRedirectsToOrigin(t *testing.T) {
	t.Parallel()

	clientPattern := regexp.MustCompile(`&http\.Client\s*\{|security\.NewSSRFProtectedHTTPClient(?:ForURL)?\(`)
	credentialPattern := regexp.MustCompile(`(?i)\b(?:Set|Add|WithRequestHeader)\(\s*"(?:authorization|xi-api-key|cf-aig-authorization)"|\.SetBasicAuth\(`)
	openAIConfigPattern := regexp.MustCompile(`openai\.DefaultConfig\(`)
	exempt := []string{"internal/llm/", "internal/localllm/", "internal/httporigin/"}

	for path, reason := range credentialedRedirectPending {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("credentialedRedirectPending[%q] has no reason", path)
		}
	}

	var unbound, unboundOpenAI []string
	stillPending := map[string]bool{}
	// walkGoFiles already skips disposable/ and other non-source directories.
	walkGoFiles(t, repoPath("."), func(path string, content string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		if slices.ContainsFunc(exempt, func(prefix string) bool { return strings.HasPrefix(path, prefix) }) {
			return
		}
		if openAIConfigPattern.MatchString(content) && !strings.Contains(content, ".HTTPClient =") {
			unboundOpenAI = append(unboundOpenAI, path)
		}
		if !clientPattern.MatchString(content) || !credentialPattern.MatchString(content) || strings.Contains(content, "CheckRedirect") {
			return
		}
		if strings.TrimSpace(credentialedRedirectAllowlist[path]) != "" {
			return
		}
		if _, pending := credentialedRedirectPending[path]; pending {
			stillPending[path] = true
			return
		}
		unbound = append(unbound, path)
	})

	if len(unbound) > 0 {
		slices.Sort(unbound)
		t.Errorf("credentialed HTTP clients follow redirects to any origin; bind them with httporigin.NewClient, security.NewSSRFProtectedHTTPClientSameOrigin or a CheckRedirect policy (do not add them to credentialedRedirectPending):\n%s", strings.Join(unbound, "\n"))
	}
	if len(unboundOpenAI) > 0 {
		slices.Sort(unboundOpenAI)
		t.Errorf("go-openai clients built with openai.DefaultConfig keep the default http.Client; assign an origin-bound .HTTPClient (httporigin.NewClient):\n%s", strings.Join(unboundOpenAI, "\n"))
	}

	var resolved []string
	for path := range credentialedRedirectPending {
		if !stillPending[path] {
			resolved = append(resolved, path)
		}
	}
	if len(resolved) > 0 {
		slices.Sort(resolved)
		t.Errorf("these files no longer hold an unbound credentialed client (bound, moved or deleted); remove their entries from credentialedRedirectPending:\n%s", strings.Join(resolved, "\n"))
	}
}
