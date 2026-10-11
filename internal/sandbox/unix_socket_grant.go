package sandbox

import "strings"

// AddUnixSocketGrant appends the unix:// socket from DOCKER_HOST onto the
// sandbox read-write lists. It does not read Docker permission. A tcp host
// adds no filesystem path. A socket path containing ':', a newline, or NUL is
// omitted so the colon-separated lists are not split. The read-write list is
// extended only when it already exists; the required list is created when
// the socket is granted and that list is absent.
func AddUnixSocketGrant(env []string) []string {
	path, ok := unixSocketGrantPath(env)
	if !ok || !envHasKey(env, "AURAGO_SBX_RW") {
		return env
	}
	env = appendSandboxGrantPath(env, "AURAGO_SBX_RW", path)
	return appendSandboxGrantPath(env, "AURAGO_SBX_RW_REQUIRED", path)
}

func unixSocketGrantPath(env []string) (string, bool) {
	for _, kv := range env {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || name != "DOCKER_HOST" {
			continue
		}
		if !strings.HasPrefix(value, "unix://") {
			return "", false
		}
		path := strings.TrimPrefix(value, "unix://")
		if path == "" || strings.ContainsAny(path, ":\n\x00") {
			return "", false
		}
		return path, true
	}
	return "", false
}

func envHasKey(env []string, key string) bool {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return true
		}
	}
	return false
}

func appendSandboxGrantPath(env []string, key, path string) []string {
	prefix := key + "="
	found := false
	for i, kv := range env {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || name != key {
			continue
		}
		found = true
		if colonListContains(value, path) {
			continue
		}
		if value == "" {
			env[i] = prefix + path
			continue
		}
		env[i] = kv + ":" + path
	}
	if found {
		return env
	}
	return append(env, prefix+path)
}

func colonListContains(list, path string) bool {
	for _, entry := range strings.Split(list, ":") {
		if entry == path {
			return true
		}
	}
	return false
}
