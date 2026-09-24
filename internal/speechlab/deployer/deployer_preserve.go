package deployer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"aurago/internal/dockerutil"
)

const labResponseMaxBytes = 1 << 20

func (m *Manager) abortUpdatePreflight(previous State, err error) error {
	if previous.State != "ready" {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	m.state = cloneState(previous)
	m.state.LastErrorCode = Code(err)
	m.state.LastError = safeError(err)
	m.mu.Unlock()
	if persistErr := m.persist(); persistErr != nil {
		return &Error{Code: "speech_lab_state_persist_failed", Err: persistErr}
	}
	return err
}

func (m *Manager) labRequest(ctx context.Context, base, method, path string, payload any, timeout time.Duration, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode Speech Lab request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, body)
	if err != nil {
		return fmt.Errorf("prepare Speech Lab request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := *m.httpClient
	client.Timeout = timeout
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Speech Lab %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, labResponseMaxBytes+1))
	if err != nil {
		return fmt.Errorf("read Speech Lab response: %w", err)
	}
	if len(data) > labResponseMaxBytes {
		return fmt.Errorf("Speech Lab %s response is too large", path)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Speech Lab %s returned HTTP %d", path, resp.StatusCode)
	}
	if out != nil && json.Unmarshal(data, out) != nil {
		return fmt.Errorf("Speech Lab %s returned invalid JSON", path)
	}
	return nil
}

func (m *Manager) readStack(ctx context.Context, base string) (StackSelection, error) {
	var status struct {
		ASR *struct {
			BackendID string `json:"backend_id"`
		} `json:"asr"`
		TTS *struct {
			BackendID string `json:"backend_id"`
		} `json:"tts"`
		LLM *struct {
			BackendID string `json:"backend_id"`
		} `json:"llm"`
		Runtime struct {
			ASR   string `json:"asr"`
			TTS   string `json:"tts"`
			LLM   string `json:"llm"`
			Voice string `json:"voice"`
		} `json:"runtime"`
		ASRTransition struct {
			Phase string `json:"phase"`
		} `json:"asr_transition"`
		TTSTransition struct {
			Phase string `json:"phase"`
		} `json:"tts_transition"`
		LLMTransition struct {
			Phase string `json:"phase"`
		} `json:"llm_transition"`
	}
	if err := m.labRequest(ctx, base, http.MethodGet, "/api/v1/stack", nil, 20*time.Second, &status); err != nil {
		return StackSelection{}, err
	}
	for _, phase := range []string{status.ASRTransition.Phase, status.TTSTransition.Phase, status.LLMTransition.Phase} {
		if phase != "" && phase != "idle" && phase != "ready" {
			return StackSelection{}, fmt.Errorf("Speech Lab stack transition is still active")
		}
	}
	selection := StackSelection{
		ASRID: status.Runtime.ASR, TTSID: status.Runtime.TTS,
		LLMID: status.Runtime.LLM, Voice: status.Runtime.Voice,
	}
	if status.ASR != nil && status.ASR.BackendID != "" {
		selection.ASRID = status.ASR.BackendID
	}
	if status.TTS != nil && status.TTS.BackendID != "" {
		selection.TTSID = status.TTS.BackendID
	}
	if status.LLM != nil && status.LLM.BackendID != "" {
		selection.LLMID = status.LLM.BackendID
	}
	if selection.ASRID == "" || selection.TTSID == "" {
		return StackSelection{}, fmt.Errorf("Speech Lab stack has no active ASR or TTS selection")
	}
	return selection, nil
}

func (m *Manager) restoreStack(ctx context.Context, base string, selection *StackSelection) error {
	if selection == nil {
		return nil
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := m.labRequest(ctx, base, http.MethodPut, "/api/v1/stack", selection, 5*time.Minute, &result); err != nil {
		return fmt.Errorf("restore Speech Lab stack: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("Speech Lab rejected the saved stack")
	}
	actual, err := m.readStack(ctx, base)
	if err != nil {
		return fmt.Errorf("verify restored Speech Lab stack: %w", err)
	}
	if actual.ASRID != selection.ASRID || actual.TTSID != selection.TTSID ||
		actual.LLMID != selection.LLMID || actual.Voice != selection.Voice {
		return fmt.Errorf("Speech Lab did not retain the saved ASR, TTS, LLM and voice selection")
	}
	return nil
}

func captureModules(containers []moduleContainer, manifest BundleManifest) ([]ModuleSelection, error) {
	modules := make([]ModuleSelection, 0, len(containers))
	for _, container := range containers {
		selection := ModuleSelection{
			ID: container.ID, BackendID: container.Labels["backend-id"],
			VariantID: container.Labels["variant-id"], Stage: container.Labels["stage"],
			Running: strings.EqualFold(container.State, "running"),
		}
		if len(container.Names) > 0 {
			selection.Name = strings.TrimPrefix(container.Names[0], "/")
		}
		if selection.Name == "" || runtimeForSelection(manifest, selection) == nil {
			return nil, fmt.Errorf("installed module %q has no compatible signed runtime in the update", selection.VariantID)
		}
		modules = append(modules, selection)
	}
	return modules, nil
}

func runtimeForSelection(manifest BundleManifest, selection ModuleSelection) *BundleRuntime {
	for i := range manifest.Runtimes {
		runtime := &manifest.Runtimes[i]
		if runtime.BackendID == selection.BackendID && runtime.VariantID == selection.VariantID && runtime.Stage == selection.Stage {
			return runtime
		}
	}
	return nil
}

func (m *Manager) restoreModules(ctx context.Context, op operationSnapshot, manifest BundleManifest, modules []ModuleSelection, fingerprint string) ([]string, error) {
	ids := make([]string, 0, len(modules))
	for _, module := range modules {
		runtime := runtimeForSelection(manifest, module)
		if runtime == nil {
			return nil, fmt.Errorf("saved module %q is unavailable", module.VariantID)
		}
		path := "/api/v1/modules/" + url.PathEscape(module.BackendID) + "/install"
		var status struct {
			VariantID    string `json:"variant_id"`
			State        string `json:"state"`
			ModelState   string `json:"model_state"`
			RuntimeState string `json:"runtime_state"`
		}
		if err := m.labRequest(ctx, op.readinessBaseURL, http.MethodGet, path, nil, 20*time.Second, &status); err != nil {
			return nil, fmt.Errorf("check saved module %q: %w", module.BackendID, err)
		}
		if status.VariantID != module.VariantID {
			return nil, fmt.Errorf("saved module %q resolves to a different runtime variant", module.BackendID)
		}
		if status.State != "ready" {
			if err := m.labRequest(ctx, op.readinessBaseURL, http.MethodPost, path, map[string]any{}, 20*time.Second, &status); err != nil {
				return nil, fmt.Errorf("install saved module %q: %w", module.BackendID, err)
			}
			pollCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
			ticker := time.NewTicker(time.Second)
			var pollErr error
			consecutiveErrors := 0
			for status.State != "ready" && status.State != "failed" && status.State != "cancelled" {
				select {
				case <-pollCtx.Done():
					status.State = "timeout"
				case <-ticker.C:
					if err := m.labRequest(pollCtx, op.readinessBaseURL, http.MethodGet, path, nil, 20*time.Second, &status); err != nil {
						pollErr = err
						consecutiveErrors++
						if consecutiveErrors >= 3 {
							status.State = "error"
						}
					} else {
						consecutiveErrors = 0
					}
				}
				if status.State == "timeout" || status.State == "error" {
					break
				}
			}
			ticker.Stop()
			cancel()
			if status.State == "error" && pollErr != nil {
				return nil, fmt.Errorf("poll saved module %q: %w", module.BackendID, pollErr)
			}
		}
		if status.State != "ready" || status.VariantID != module.VariantID ||
			!slices.Contains([]string{"ready", "running"}, status.RuntimeState) ||
			!slices.Contains([]string{"installed", "bundled"}, status.ModelState) {
			return nil, fmt.Errorf("saved module %q was not restored (state %s)", module.BackendID, status.State)
		}
		container, found, err := m.inspectContainer(ctx, op, runtime.Container)
		if err != nil {
			return nil, fmt.Errorf("inspect restored module %q: %w", module.BackendID, err)
		}
		if !found {
			return nil, fmt.Errorf("restored module %q is missing", module.BackendID)
		}
		labels := container.Config.Labels
		if err := validateManagedModule(container.Config.Image, labels, manifest.BundleVersion); err != nil ||
			labels["aurago.fingerprint"] != fingerprint || labels["backend-id"] != module.BackendID ||
			labels["variant-id"] != module.VariantID || labels["stage"] != module.Stage || container.Config.Image != runtime.Image {
			return nil, fmt.Errorf("restored module %q does not match the signed runtime", module.BackendID)
		}
		ids = append(ids, container.ID)
	}
	return ids, nil
}

func (m *Manager) removeNewModules(ctx context.Context, op operationSnapshot, transaction *DeploymentTransaction) error {
	if transaction.NewFingerprint == "" || transaction.NewBundle == "" {
		return nil
	}
	var containers []moduleContainer
	if _, err := op.docker.DoJSON(ctx, http.MethodGet, "/containers/json?all=1", nil, &containers); err != nil {
		return fmt.Errorf("list new Speech Lab modules: %w", err)
	}
	oldIDs := make(map[string]bool, len(transaction.PreviousModules))
	for _, module := range transaction.PreviousModules {
		oldIDs[module.ID] = true
	}
	for _, container := range containers {
		labels := container.Labels
		if oldIDs[container.ID] || labels["aurago.role"] != "module" || labels["aurago.bundle"] != transaction.NewBundle ||
			labels["aurago.fingerprint"] != transaction.NewFingerprint || !dockerutil.ManagedBy(labels, OwnerLabel) {
			continue
		}
		if err := validateManagedModule(container.Image, labels, transaction.NewBundle); err != nil {
			return fmt.Errorf("refusing malformed new module %q: %w", container.ID, err)
		}
		if err := m.removeOwnedContainer(ctx, op, container.ID); err != nil {
			return err
		}
	}
	return nil
}
