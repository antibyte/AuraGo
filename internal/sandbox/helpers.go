package sandbox

import (
	"os"
	"strconv"
	"strings"
)

// FilterEnv removes AURAGO_SBX_* and other sensitive env vars from the environment
// passed to any child process (sandbox helper, Python scripts, etc.).
func FilterEnv(env []string) []string {
	// Prefixes and exact names of env vars that must never be inherited by sandboxed processes.
	sensitivePrefixes := []string{
		"AURAGO_SBX_",
		"AURAGO_MASTER_KEY",
		"LLM_API_KEY",
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
		"OPENROUTER_API_KEY",
		"GEMINI_API_KEY",
		"GROQ_API_KEY",
		"MISTRAL_API_KEY",
		"COHERE_API_KEY",
		"TOGETHER_API_KEY",
		"TAILSCALE_API_KEY",
		"ANSIBLE_API_TOKEN",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_SECURITY_TOKEN",
		"AZURE_CLIENT_SECRET",
		"AZURE_API_KEY",
		"GOOGLE_APPLICATION_CREDENTIALS",
		"GCP_SERVICE_ACCOUNT_KEY",
		"GOOGLE_API_KEY",
		"TF_VAR_",
		"ANSIBLE_VAULT_",
	}
	var filtered []string
	for _, e := range env {
		skip := false
		for _, prefix := range sensitivePrefixes {
			if strings.HasPrefix(e, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// splitPaths splits a colon-separated path list, trimming whitespace and skipping empty segments.
func splitPaths(s string) []string {
	if s == "" {
		return nil
	}
	var paths []string
	for _, p := range strings.Split(s, ":") {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// envInt reads an environment variable and parses it as an integer. Returns 0 on error or if empty.
func envInt(key string) int {
	s := os.Getenv(key)
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}
