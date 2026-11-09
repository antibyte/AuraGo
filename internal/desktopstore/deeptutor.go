package desktopstore

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"aurago/internal/security"
)

// DeepTutorAppID is the catalog ID of the DeepTutor tutoring app.
const DeepTutorAppID = "deeptutor"

// DeepTutorRelayPath is the AuraGo path prefix of the token-gated LLM relay
// that DeepTutor uses instead of a provider key. The server owns the handlers;
// the Store only writes the URL and the token into the container.
const DeepTutorRelayPath = "/api/store-llm/deeptutor/v1"

const (
	// deepTutorTokenKey is the generated-secret key; the Vault entry is
	// storeSecretVaultKey(DeepTutorAppID, deepTutorTokenKey).
	deepTutorTokenKey = "llm_relay_token"

	deepTutorTokenEnv       = "AURAGO_DT_RELAY_TOKEN"
	deepTutorRelayURLEnv    = "AURAGO_DT_RELAY_URL"
	deepTutorVisionEnv      = "AURAGO_DT_VISION"
	deepTutorEmbeddingsEnv  = "AURAGO_DT_EMBEDDINGS"
	deepTutorContextEnv     = "AURAGO_DT_CONTEXT"
	deepTutorMergeScriptEnv = "AURAGO_DT_MERGE_PY"

	// DeepTutor installs a 3.9 GB image and its healthcheck has a 60 s start
	// period; the first start also runs the catalog merge.
	deepTutorReadinessTimeout = 5 * time.Minute
)

// deepTutorWrapper runs in front of the image entrypoint. python -I keeps the
// working directory (/app) and PYTHON* variables out of the interpreter's
// search path: the script runs as root and /app may be writable by DeepTutor's
// user. The merge is fail-open: a problem is reported on stderr and DeepTutor
// still starts. The script and the token leave the environment before
// DeepTutor's processes start, so they do not appear in what those processes
// inherit. The image has no Cmd, so "$@" is empty.
const deepTutorWrapper = `python -I -c "$AURAGO_DT_MERGE_PY" || echo "aurago-deeptutor: the AuraGo model profile could not be merged; starting DeepTutor without it" >&2
unset AURAGO_DT_MERGE_PY AURAGO_DT_RELAY_TOKEN
exec /app/entrypoint.sh "$@"`

// deepTutorMergeScript merges the managed AuraGo profiles into DeepTutor's
// model_catalog.json; see the script header for its contract.
//
//go:embed deeptutor_assets/aurago_catalog_merge.py
var deepTutorMergeScript string

// RelayInfo describes how a Store container reaches the AuraGo LLM relay.
type RelayInfo struct {
	// BaseURL is the container-reachable AuraGo origin without a trailing
	// slash, for example http://host.docker.internal:8088.
	BaseURL string
	// ExtraHosts are added to the container, for example
	// host.docker.internal:host-gateway.
	ExtraHosts []string
	// Vision reports that the AuraGo main model accepts images.
	Vision bool
	// EmbeddingsAvailable reports that the relay can serve /embeddings.
	EmbeddingsAvailable bool
	// ContextWindow is the main model's context size in tokens; 0 is unknown.
	ContextWindow int
}

// LLMRelayProvider is implemented by the server. It returns the relay origin a
// container on the given Docker network ("" for the default bridge) can reach,
// or an error whose text explains why DeepTutor cannot use the AuraGo LLM.
type LLMRelayProvider interface {
	RelayInfo(ctx context.Context, networkMode string) (RelayInfo, error)
}

// VerifyRelayToken reports whether token is the relay token of the installed
// DeepTutor app. It is the only check behind the relay's bearer authentication:
// it fails closed for any other app, an app that is not installed, a missing
// or empty Vault secret and an empty token, and compares SHA-256 digests in
// constant time. The token is never logged.
func (s *Service) VerifyRelayToken(ctx context.Context, appID, token string) bool {
	if s == nil || token == "" || normalizeAppID(appID) != DeepTutorAppID || s.cfg.Secrets == nil {
		return false
	}
	if _, installed, err := s.GetInstalled(ctx, DeepTutorAppID); err != nil || !installed {
		return false
	}
	stored, err := s.cfg.Secrets.ReadSecret(storeSecretVaultKey(DeepTutorAppID, deepTutorTokenKey))
	if err != nil || stored == "" {
		return false
	}
	storedSum := sha256.Sum256([]byte(stored))
	givenSum := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(storedSum[:], givenSum[:]) == 1
}

// prepareDeepTutorInstall fails an install early, before the 3.9 GB image is
// pulled, when the AuraGo LLM relay cannot serve the container.
func (s *Service) prepareDeepTutorInstall(ctx context.Context) error {
	_, err := s.deepTutorRelayInfo(ctx, "")
	return err
}

// deepTutorRelayInfo asks the server for the relay origin and validates the
// answer. Its errors are operation errors: clear and free of secrets.
func (s *Service) deepTutorRelayInfo(ctx context.Context, networkMode string) (RelayInfo, error) {
	if s.cfg.LLMRelay == nil {
		return RelayInfo{}, errors.New("DeepTutor needs the AuraGo LLM relay, which is not available in this AuraGo setup")
	}
	info, err := s.cfg.LLMRelay.RelayInfo(ctx, networkMode)
	if err != nil {
		return RelayInfo{}, fmt.Errorf("DeepTutor cannot use the AuraGo LLM: %s", security.Scrub(err.Error()))
	}
	info.BaseURL = strings.TrimRight(strings.TrimSpace(info.BaseURL), "/")
	parsed, err := url.Parse(info.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		strings.IndexFunc(info.BaseURL, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return RelayInfo{}, errors.New("DeepTutor cannot use the AuraGo LLM: the relay address that Docker containers can reach is invalid")
	}
	hosts := make([]string, 0, len(info.ExtraHosts))
	for _, host := range info.ExtraHosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if !strings.Contains(host, ":") || strings.IndexFunc(host, unicode.IsControl) >= 0 {
			return RelayInfo{}, errors.New("DeepTutor cannot use the AuraGo LLM: the relay host mapping is invalid")
		}
		hosts = append(hosts, host)
	}
	info.ExtraHosts = hosts
	if info.ContextWindow < 0 {
		info.ContextWindow = 0
	}
	return info, nil
}

func (s *Service) readDeepTutorToken() (string, error) {
	if s.cfg.Secrets == nil {
		return "", errors.New("DeepTutor needs the AuraGo vault for its relay token")
	}
	token, err := s.cfg.Secrets.ReadSecret(storeSecretVaultKey(DeepTutorAppID, deepTutorTokenKey))
	if err != nil || strings.TrimSpace(token) == "" {
		return "", errors.New("the DeepTutor relay token is missing in the AuraGo vault; uninstall DeepTutor with its data and install it again")
	}
	security.RegisterSensitive(token)
	return token, nil
}

// deepTutorContainerSpec adds what must never be stored in the installed
// record: the relay token (read from the Vault) and the relay settings, which
// follow the current AuraGo address on every create, so an Update picks up a
// changed URL or token. The record's Env only holds the catalog values.
func (s *Service) deepTutorContainerSpec(ctx context.Context, spec ContainerSpec) (ContainerSpec, error) {
	info, err := s.deepTutorRelayInfo(ctx, spec.NetworkMode)
	if err != nil {
		return spec, err
	}
	token, err := s.readDeepTutorToken()
	if err != nil {
		return spec, err
	}
	managed := []string{deepTutorTokenEnv, deepTutorRelayURLEnv, deepTutorVisionEnv, deepTutorEmbeddingsEnv, deepTutorContextEnv, deepTutorMergeScriptEnv}
	env := make([]string, 0, len(spec.Env)+len(managed))
	for _, item := range spec.Env {
		if !hasEnvKey(item, managed) {
			env = append(env, item)
		}
	}
	env = append(env,
		deepTutorRelayURLEnv+"="+info.BaseURL+DeepTutorRelayPath,
		deepTutorTokenEnv+"="+token,
		deepTutorVisionEnv+"="+boolFlag(info.Vision),
		deepTutorEmbeddingsEnv+"="+boolFlag(info.EmbeddingsAvailable),
	)
	if info.ContextWindow > 0 {
		env = append(env, deepTutorContextEnv+"="+strconv.Itoa(info.ContextWindow))
	}
	env = append(env, deepTutorMergeScriptEnv+"="+deepTutorMergeScript)
	spec.Env = env
	spec.ExtraHosts = appendMissing(spec.ExtraHosts, info.ExtraHosts...)
	spec.Entrypoint = []string{"/bin/sh", "-c", deepTutorWrapper, "aurago-deeptutor"}
	return spec, nil
}

func hasEnvKey(item string, keys []string) bool {
	for _, key := range keys {
		if strings.HasPrefix(item, key+"=") {
			return true
		}
	}
	return false
}

func boolFlag(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func appendMissing(items []string, extra ...string) []string {
	out := append([]string(nil), items...)
	for _, candidate := range extra {
		found := false
		for _, item := range out {
			if item == candidate {
				found = true
				break
			}
		}
		if !found {
			out = append(out, candidate)
		}
	}
	return out
}

// withoutDeepTutorToken removes the generated relay token from the environment
// that is stored in the installed record; runtimeContainerSpec adds it back
// from the Vault when it creates the container.
func withoutDeepTutorToken(env []string) []string {
	out := make([]string, 0, len(env))
	for _, item := range env {
		if !hasEnvKey(item, []string{deepTutorTokenEnv}) {
			out = append(out, item)
		}
	}
	return out
}
