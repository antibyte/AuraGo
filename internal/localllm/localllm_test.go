package localllm

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestDownloadArtifactResumeAndServerIgnoringRange(t *testing.T) {
	payload := []byte("immutable-model-bytes")
	sum := sha256.Sum256(payload)
	artifact := Artifact{Name: "model.gguf", Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}
	for _, test := range []struct {
		name      string
		status    int
		response  []byte
		wantRange bool
	}{
		{name: "resume 206", status: http.StatusPartialContent, response: payload[5:], wantRange: true},
		{name: "restart on 200", status: http.StatusOK, response: payload, wantRange: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			destination := filepath.Join(dir, artifact.Name)
			if err := os.WriteFile(destination+".part", payload[:5], 0o600); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.wantRange && r.Header.Get("Range") != "bytes=5-" {
					t.Errorf("Range = %q", r.Header.Get("Range"))
				}
				w.WriteHeader(test.status)
				_, _ = w.Write(test.response)
			}))
			defer server.Close()
			if err := downloadArtifact(context.Background(), server.Client(), server.URL, destination, artifact, nil); err != nil {
				t.Fatalf("downloadArtifact() error = %v", err)
			}
			got, _ := os.ReadFile(destination)
			if string(got) != string(payload) {
				t.Fatalf("downloaded = %q", got)
			}
		})
	}
}

func TestDownloadArtifactPreservesPreviousValidFileOnHashFailure(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "model.gguf")
	previous := []byte("previous")
	if err := os.WriteFile(destination, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	payload := []byte("corrupt")
	artifact := Artifact{Name: "model.gguf", Size: int64(len(payload)), SHA256: strings.Repeat("0", 64)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	if err := downloadArtifact(context.Background(), server.Client(), server.URL, destination, artifact, nil); err == nil {
		t.Fatal("expected hash mismatch")
	}
	got, _ := os.ReadFile(destination)
	if string(got) != string(previous) {
		t.Fatalf("valid predecessor changed: %q", got)
	}
}

func TestDownloadArtifactReplacesInvalidPredecessorOnlyAfterVerification(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "model.gguf")
	previous := []byte("invalid-old-file")
	if err := os.WriteFile(destination, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	payload := []byte("verified-new-model")
	sum := sha256.Sum256(payload)
	artifact := Artifact{Name: filepath.Base(destination), Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	if err := downloadArtifact(context.Background(), server.Client(), server.URL, destination, artifact, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("published model = %q, err = %v", got, err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o444 != 0o444 {
		t.Fatalf("model mode = %o, sidecar needs read permission", info.Mode().Perm())
	}
	backups, err := filepath.Glob(destination + ".previous-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("previous model backups = %#v, err = %v", backups, err)
	}
	if preserved, readErr := os.ReadFile(backups[0]); readErr != nil || !bytes.Equal(preserved, previous) {
		t.Fatalf("preserved predecessor = %q, err = %v", preserved, readErr)
	}
}

func TestDownloadArtifactCancellationPreservesPartialFile(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "model.gguf")
	part := destination + ".part"
	if err := os.WriteFile(part, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requested = true
		_, _ = w.Write([]byte("remaining"))
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	artifact := Artifact{Name: "model.gguf", Size: 16, SHA256: strings.Repeat("0", 64)}
	if err := downloadArtifact(ctx, server.Client(), server.URL, destination, artifact, nil); err == nil {
		t.Fatal("expected cancellation")
	}
	if requested {
		t.Fatal("cancelled download reached the server")
	}
	got, err := os.ReadFile(part)
	if err != nil || string(got) != "partial" {
		t.Fatalf("partial file = %q, err = %v", got, err)
	}
}

func TestDownloadArtifactPublishesValidCompletePartialWithoutRequest(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "model.gguf")
	payload := []byte("already-complete-and-valid")
	sum := sha256.Sum256(payload)
	artifact := Artifact{Name: filepath.Base(destination), Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}
	if err := os.WriteFile(destination+".part", payload, 0o600); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("valid complete partial triggered a network request")
		return nil, nil
	})}
	if err := downloadArtifact(context.Background(), client, "https://example.invalid/model", destination, artifact, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("published payload=%q err=%v", got, err)
	}
}

func TestDownloadArtifactPublishGuardPreservesResumableVerifiedPartial(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "model.gguf")
	payload := []byte("verified-but-stale-plan")
	sum := sha256.Sum256(payload)
	artifact := Artifact{Name: filepath.Base(destination), Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	err := downloadArtifactGuarded(
		context.Background(),
		server.Client(),
		server.URL,
		destination,
		artifact,
		nil,
		func(func() error) error { return errors.New("desired_state_changed") },
	)
	if err == nil || err.Error() != "desired_state_changed" {
		t.Fatalf("guarded download error=%v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("obsolete plan published destination, stat error=%v", err)
	}
	ok, err := verifyArtifact(destination+".part", artifact)
	if err != nil || !ok {
		t.Fatalf("verified partial was not preserved: ok=%v err=%v", ok, err)
	}
}

func TestDownloadArtifactRestartsAfterRangeNotSatisfiable(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "model.gguf")
	payload := []byte("complete-restarted-model")
	sum := sha256.Sum256(payload)
	artifact := Artifact{Name: filepath.Base(destination), Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}
	if err := os.WriteFile(destination+".part", payload[:5], 0o600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			if r.Header.Get("Range") != "bytes=5-" {
				t.Errorf("first Range=%q", r.Header.Get("Range"))
			}
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if r.Header.Get("Range") != "" {
			t.Errorf("restart Range=%q, want empty", r.Header.Get("Range"))
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	if err := downloadArtifact(context.Background(), server.Client(), server.URL, destination, artifact, nil); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests=%d, want 2", requests)
	}
}

func TestDownloadArtifactRejectsInsufficientDiskBeforeRequest(t *testing.T) {
	original := availableDiskBytes
	availableDiskBytes = func(string) (int64, error) { return 1, nil }
	t.Cleanup(func() { availableDiskBytes = original })
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requested = true
		_, _ = w.Write([]byte("model"))
	}))
	defer server.Close()
	artifact := Artifact{Name: "model.gguf", Size: 5, SHA256: strings.Repeat("0", 64)}
	err := downloadArtifact(context.Background(), server.Client(), server.URL, filepath.Join(t.TempDir(), artifact.Name), artifact, nil)
	if err == nil || err.Error() != "insufficient_disk_space" {
		t.Fatalf("downloadArtifact() error = %v", err)
	}
	if requested {
		t.Fatal("download request started despite insufficient disk space")
	}
}

func TestEvaluateMTPThresholdsAndSemanticToolCalls(t *testing.T) {
	target := []BenchmarkSample{
		{ToolCall: `{"id":"a","function":{"name":"shell","arguments":"{\"b\":2,\"a\":1}"}}`, GenerationTokensS: 10, TTFTMilliseconds: 100},
		{ToolCall: `{"function":{"name":"shell","arguments":{"a":1,"b":2}}}`, GenerationTokensS: 10, TTFTMilliseconds: 100},
		{ToolCall: `{"name":"shell","arguments":{"a":1,"b":2}}`, GenerationTokensS: 10, TTFTMilliseconds: 100},
	}
	speculative := []BenchmarkSample{
		{ToolCall: `{"function":{"name":"shell","arguments":{"b":2,"a":1}}}`, GenerationTokensS: 12, TTFTMilliseconds: 110, DraftAcceptance: .82},
		{ToolCall: `{"name":"shell","arguments":"{\"a\":1,\"b\":2}"}`, GenerationTokensS: 11, TTFTMilliseconds: 120, DraftAcceptance: .80},
		{ToolCall: `{"function":{"name":"shell","arguments":{"a":1,"b":2}}}`, GenerationTokensS: 12, TTFTMilliseconds: 110, DraftAcceptance: .85},
	}
	decision := EvaluateMTP(target, speculative)
	if !decision.Selected {
		t.Fatalf("EvaluateMTP() = %#v", decision)
	}
	speculative[1].DraftAcceptance = .20
	speculative[0].DraftAcceptance = .20
	if got := EvaluateMTP(target, speculative); got.Selected || got.Reason != "mtp_acceptance_below_80_percent" {
		t.Fatalf("low acceptance decision = %#v", got)
	}
}

func TestMTPAutoMeasurementUsesTemporary2KTargetPlan(t *testing.T) {
	original := runtimePlan{
		Config:  config.LocalLLMConfig{ContextSize: 8192},
		Profile: HardwareProfile{SelectedBackend: "sycl"},
		Draft:   &Artifact{SHA256: strings.Repeat("a", 64)},
	}
	measurement := mtpMeasurementPlan(original)
	if measurement.Config.ContextSize != 2048 || measurement.Draft != nil ||
		!containsString(measurement.ResolvedParameters, "--ctx-size=2048") ||
		!containsString(measurement.ResolvedParameters, "--spec-type=none") {
		t.Fatalf("MTP measurement plan = %#v", measurement)
	}
	if original.Config.ContextSize != 8192 || original.Draft == nil {
		t.Fatalf("immutable original plan was mutated = %#v", original)
	}
}

func TestContextCapabilityCacheIsBoundToCompleteRuntimeFingerprint(t *testing.T) {
	manager := &Manager{modelDir: t.TempDir()}
	plan := runtimePlan{
		Config: config.LocalLLMConfig{ContextSize: 8192},
		Profile: HardwareProfile{
			SelectedBackend: "sycl",
			Fingerprint:     "arc-b580-driver-a",
		},
		Image: Image{Reference: "image@sha256:" + strings.Repeat("a", 64)},
		Model: Artifact{SHA256: strings.Repeat("b", 64)},
		ResolvedParameters: resolvedParameters(
			config.LocalLLMConfig{ContextSize: 8192},
			false,
		),
	}
	if err := manager.saveContextCapability(plan, 32768); err != nil {
		t.Fatal(err)
	}
	if maxContext, ok := manager.loadContextCapability(plan); !ok || maxContext != 32768 {
		t.Fatalf("saved context capability = %d, %v", maxContext, ok)
	}

	driverChanged := plan
	driverChanged.Profile.Fingerprint = "arc-b580-driver-b"
	if _, ok := manager.loadContextCapability(driverChanged); ok {
		t.Fatal("32K capability survived a hardware/driver fingerprint change")
	}
	backendChanged := plan
	backendChanged.Profile.SelectedBackend = "vulkan"
	if _, ok := manager.loadContextCapability(backendChanged); ok {
		t.Fatal("32K capability survived a backend change")
	}
}

func TestBenchmarkSampleMeasuresStreamingTTFTAndDraftAcceptance(t *testing.T) {
	key := "benchmark-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request["stream"] != true {
			t.Fatalf("benchmark request stream=%#v", request["stream"])
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("test writer is not a flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		time.Sleep(25 * time.Millisecond)
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"name\":\"report_\",\"arguments\":\"{\\\"status\\\":\"}}]}}]}\n\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"name\":\"status\",\"arguments\":\"\\\"ready\\\",\\\"code\\\":\\\"benchmark\\\"}\"}}]}}],\"timings\":{\"predicted_per_second\":42,\"draft_n\":10,\"draft_n_accepted\":8}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	_, portText, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	manager := &Manager{
		vault: &localLLMTestVault{values: map[string]string{
			config.LocalLLMRuntimeAPIKeyVaultKey: key,
		}},
	}
	sample, err := manager.benchmarkSample(context.Background(), config.LocalLLMConfig{ListenPort: port})
	if err != nil {
		t.Fatal(err)
	}
	if sample.TTFTMilliseconds < 20 || sample.GenerationTokensS != 42 || sample.DraftAcceptance != 0.8 {
		t.Fatalf("stream benchmark sample=%#v", sample)
	}
	if normalizeToolCall(sample.ToolCall) != normalizeToolCall(
		`{"function":{"name":"report_status","arguments":{"status":"ready","code":"benchmark"}}}`,
	) {
		t.Fatalf("stream tool call=%s", sample.ToolCall)
	}
}

func TestBenchmarkFailureSampleClassifiesOOMAndDeviceLossWithoutFalsePositive(t *testing.T) {
	if sample := benchmarkFailureSample(`{"error":"CUDA out of memory"}`); !sample.OOM || sample.OffloadError {
		t.Fatalf("OOM sample = %#v", sample)
	}
	if sample := benchmarkFailureSample(`{"error":"SYCL device lost during offload"}`); sample.OOM || !sample.OffloadError {
		t.Fatalf("offload sample = %#v", sample)
	}
	if sample := benchmarkFailureSample(`{"message":"tool completed"}`); sample.OOM || sample.OffloadError {
		t.Fatalf("ordinary sample misclassified = %#v", sample)
	}
}

func TestSelectedArtifactsNeverMixNormalAndMTPQuantizations(t *testing.T) {
	manifest := DefaultManifest()
	for _, test := range []struct {
		variant    string
		mtp        string
		targetHash string
		draftHash  string
	}{
		{
			variant: "q4_k_m", mtp: "off",
			targetHash: "148c82694a60d279fa21316ede2ba5ebcc9f7633da6db7e328640487e154f5f4",
		},
		{
			variant: "q4_k_m", mtp: "mtp2",
			targetHash: "211f9455a99c84a92942e4fba13af164f0707e44008eabce9406cb2cb111daa4",
			draftHash:  "ae7df638ba4136f98a49b7d8814389681f9e11c18ebc609ee2ef9d693a2b1818",
		},
		{
			variant: "q8_0", mtp: "mtp2",
			targetHash: manifest.Artifacts["mtp_target_q8_0"].SHA256,
			draftHash:  manifest.Artifacts["mtp_sidecar_q8_0"].SHA256,
		},
	} {
		t.Run(test.variant+"_"+test.mtp, func(t *testing.T) {
			manager := &Manager{
				cfg:      config.LocalLLMConfig{ModelVariant: test.variant, MTP: test.mtp},
				manifest: manifest,
			}
			target, draft, err := manager.selectedArtifacts()
			if err != nil {
				t.Fatal(err)
			}
			if target.SHA256 != test.targetHash {
				t.Fatalf("target hash = %s, want %s", target.SHA256, test.targetHash)
			}
			if test.draftHash == "" && draft != nil {
				t.Fatalf("normal model was combined with draft %#v", draft)
			}
			if test.draftHash != "" && (draft == nil || draft.SHA256 != test.draftHash) {
				t.Fatalf("draft = %#v, want hash %s", draft, test.draftHash)
			}
		})
	}
}

func TestDefaultManifestPinsPublicHuggingFaceArtifacts(t *testing.T) {
	manifest := DefaultManifest()
	if manifest.ReleaseReady {
		t.Fatal("public artifact pins must not bypass the native Linux release gate")
	}

	expected := map[string]struct {
		repository string
		revision   string
		path       string
	}{
		"normal_q4_k_m": {
			repository: "antibyte/AuraGo-Qwen3.5-4B-GGUF",
			revision:   ggufRepositoryRevision,
			path:       "AuraGo-Qwen3.5-4B.Q4_K_M.gguf",
		},
		"normal_q8_0": {
			repository: "antibyte/AuraGo-Qwen3.5-4B-GGUF",
			revision:   ggufRepositoryRevision,
			path:       "AuraGo-Qwen3.5-4B.Q8_0.gguf",
		},
		"mtp_target_q4_k_m": {
			repository: "antibyte/AuraGo-Qwen3.5-4B-MTP-v1",
			revision:   mtpRepositoryRevision,
			path:       "gguf/target/aurago-qwen35-4b-target.Q4_K_M.gguf",
		},
		"mtp_target_q8_0": {
			repository: "antibyte/AuraGo-Qwen3.5-4B-MTP-v1",
			revision:   mtpRepositoryRevision,
			path:       "gguf/target/aurago-qwen35-4b-target.Q8_0.gguf",
		},
		"mtp_sidecar_q4_k_m": {
			repository: "antibyte/AuraGo-Qwen3.5-4B-MTP-v1",
			revision:   mtpRepositoryRevision,
			path:       "gguf/sidecar/aurago-qwen35-4b-mtp-v1-sidecar.Q4_K_M.gguf",
		},
		"mtp_sidecar_q8_0": {
			repository: "antibyte/AuraGo-Qwen3.5-4B-MTP-v1",
			revision:   mtpRepositoryRevision,
			path:       "gguf/sidecar/aurago-qwen35-4b-mtp-v1-sidecar.Q8_0.gguf",
		},
	}

	for name, want := range expected {
		artifact, ok := manifest.Artifacts[name]
		if !ok {
			t.Fatalf("artifact %q is missing", name)
		}
		if artifact.Repository != want.repository || artifact.Revision != want.revision || artifact.Path != want.path {
			t.Fatalf("artifact %q pin = %#v, want repository=%q revision=%q path=%q",
				name, artifact, want.repository, want.revision, want.path)
		}
		wantURL := "https://huggingface.co/" + want.repository + "/resolve/" + want.revision + "/" + want.path
		if artifact.URL() != wantURL {
			t.Fatalf("artifact %q URL = %q, want %q", name, artifact.URL(), wantURL)
		}
	}
}

func TestReleaseManifestRequiresDigestPinsAndNativeValidatedProfiles(t *testing.T) {
	manifest := DefaultManifest()
	manifest.ReleaseReady = true
	manifest.HardwareProfiles = nil
	for key, artifact := range manifest.Artifacts {
		if artifact.Path == "" {
			artifact.Path = artifact.Name
		}
		artifact.Revision = strings.Repeat("a", 40)
		manifest.Artifacts[key] = artifact
	}
	for backend, image := range manifest.Images {
		image.Supported = true
		manifest.Images[backend] = image
		manifest.HardwareProfiles = append(manifest.HardwareProfiles, ValidatedHardwareProfile{
			ID: backend + "-native-smoke", Backend: backend, Vendor: backend,
			Device: "validated-device", MinVRAM: 8 << 30, Status: "validated-linux",
		})
	}
	if err := manifest.validate(); err != nil {
		t.Fatalf("ready manifest rejected: %v", err)
	}
	brokenDigest := manifest
	brokenDigest.Images = cloneImages(manifest.Images)
	image := brokenDigest.Images["cuda"]
	image.Reference = "ghcr.io/antibyte/aurago-llm-cuda:mutable"
	brokenDigest.Images["cuda"] = image
	if err := brokenDigest.validate(); err == nil || !strings.Contains(err.Error(), "digest-pinned") {
		t.Fatalf("mutable image validation error = %v", err)
	}
	nonHexDigest := manifest
	nonHexDigest.Images = cloneImages(manifest.Images)
	image = nonHexDigest.Images["cuda"]
	image.Reference = "ghcr.io/antibyte/aurago-llm-cuda@sha256:" + strings.Repeat("z", 64)
	nonHexDigest.Images["cuda"] = image
	if err := nonHexDigest.validate(); err == nil || !strings.Contains(err.Error(), "digest-pinned") {
		t.Fatalf("non-hex image digest validation error = %v", err)
	}
	missingProfile := manifest
	missingProfile.HardwareProfiles = append([]ValidatedHardwareProfile(nil), manifest.HardwareProfiles[1:]...)
	if err := missingProfile.validate(); err == nil || !strings.Contains(err.Error(), "validated Linux") {
		t.Fatalf("missing native profile validation error = %v", err)
	}
	partialRelease := manifest
	partialRelease.Images = cloneImages(manifest.Images)
	vulkan := partialRelease.Images["vulkan"]
	vulkan.Supported = false
	partialRelease.Images["vulkan"] = vulkan
	filteredProfiles := make([]ValidatedHardwareProfile, 0, len(partialRelease.HardwareProfiles))
	for _, profile := range partialRelease.HardwareProfiles {
		if profile.Backend != "vulkan" {
			filteredProfiles = append(filteredProfiles, profile)
		}
	}
	partialRelease.HardwareProfiles = filteredProfiles
	if err := partialRelease.validate(); err != nil {
		t.Fatalf("manifest with intentionally unsupported backend rejected: %v", err)
	}
}

func TestManifestCompatibilityKeepsCandidateAndMissingDockerExperimental(t *testing.T) {
	base := HardwareProfile{
		OS: "linux", Architecture: "amd64", DockerAvailable: true,
		SelectedBackend: "sycl", SelectedDevice: "0000:03:00.0", Compatibility: "recommended",
		Devices: []GPUDevice{{
			ID: "0000:03:00.0", Vendor: "intel", Device: "0xe20b",
			VRAMBytes: 12 << 30, Discrete: true, RenderNode: "/dev/dri/renderD129",
		}},
	}
	manager := &Manager{manifest: DefaultManifest()}
	candidate := manager.applyManifestCompatibility(base)
	if candidate.Compatibility != "experimental" || !candidate.AcknowledgementDue ||
		!containsString(candidate.Warnings, "hardware_profile_candidate_linux") {
		t.Fatalf("candidate compatibility=%#v", candidate)
	}

	manifest := DefaultManifest()
	image := manifest.Images["sycl"]
	image.Supported = true
	manifest.Images["sycl"] = image
	manifest.HardwareProfiles[0].Status = "validated-linux"
	manager.manifest = manifest
	validated := manager.applyManifestCompatibility(base)
	if validated.Compatibility != "recommended" || validated.AcknowledgementDue {
		t.Fatalf("validated compatibility=%#v", validated)
	}
	base.DockerAvailable = false
	noDocker := manager.applyManifestCompatibility(base)
	if noDocker.Compatibility != "experimental" || !noDocker.AcknowledgementDue {
		t.Fatalf("no-Docker compatibility=%#v", noDocker)
	}
}

func cloneImages(source map[string]Image) map[string]Image {
	cloned := make(map[string]Image, len(source))
	for key, image := range source {
		cloned[key] = image
	}
	return cloned
}

func TestMTPCacheInvalidatesOnDriverFingerprintChange(t *testing.T) {
	manager := &Manager{modelDir: t.TempDir(), manifest: DefaultManifest()}
	cfg := config.LocalLLMConfig{Backend: "sycl", ModelVariant: "q4_k_m", MTP: "auto", ContextSize: 8192}
	first := HardwareProfile{
		OS: "linux", Architecture: "amd64", SelectedBackend: "sycl",
		SelectedDevice: "0000:03:00.0",
		Devices:        []GPUDevice{{ID: "0000:03:00.0", Vendor: "intel", Driver: "xe:1.0"}},
	}
	first.Fingerprint = hardwareFingerprint(first)
	manager.desiredFingerprint = manager.computeDesiredFingerprint(cfg, first)
	manager.saveMTPDecisionLocked(MTPDecision{Selected: true, Reason: "mtp_selected"})
	if decision, ok := manager.loadMTPDecisionLocked(); !ok || !decision.Selected {
		t.Fatalf("cached decision = %#v, ok=%v", decision, ok)
	}
	second := first
	second.Devices = append([]GPUDevice(nil), first.Devices...)
	second.Devices[0].Driver = "xe:1.1"
	second.Fingerprint = hardwareFingerprint(second)
	manager.desiredFingerprint = manager.computeDesiredFingerprint(cfg, second)
	if decision, ok := manager.loadMTPDecisionLocked(); ok {
		t.Fatalf("stale driver cache was accepted: %#v", decision)
	}
}

func TestProbeHardwareSelectsExactIntelRenderNode(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "card1", "device")
	for _, dir := range []string{device, filepath.Join(device, "drm", "renderD129")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(device, "vendor"), []byte("0x8086"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(device, "device"), []byte("0xe20b"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(device, "mem_info_vram_total"), []byte("12884901888"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := probeHardwareWithOptions("auto", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: root, dockerOnline: true,
	})
	if profile.SelectedBackend != "sycl" || profile.Compatibility != "recommended" {
		t.Fatalf("profile = %#v", profile)
	}
	if len(profile.Devices) != 1 || profile.Devices[0].RenderNode != "/dev/dri/renderD129" {
		t.Fatalf("devices = %#v", profile.Devices)
	}
}

func TestProbeHardwareAutoPrefersCUDAAndUsesPhysicalNVIDIAIdentity(t *testing.T) {
	root := t.TempDir()
	writeGPUFixture := func(card, vendor, deviceID, vram string) {
		t.Helper()
		device := filepath.Join(root, card, "device")
		if err := os.MkdirAll(filepath.Join(device, "drm", "renderD"+strings.TrimPrefix(card, "card")), 0o700); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]string{"vendor": vendor, "device": deviceID, "mem_info_vram_total": vram} {
			if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeGPUFixture("card0", "0x8086", "0xe20b", "12884901888")
	writeGPUFixture("card1", "0x10de", "0x2684", "0")
	profile := probeHardwareWithOptions("auto", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: root, dockerOnline: true,
		nvidiaToolkit: true, nvidiaSMI: "card1:2684, GPU-exact-uuid, 12288, 580.82\n",
	})
	if profile.SelectedBackend != "cuda" || profile.SelectedDevice != "card1:2684" {
		t.Fatalf("auto profile = %#v", profile)
	}
	gpu, err := profile.selectedGPU()
	if err != nil || gpu.DockerID != "GPU-exact-uuid" || gpu.VRAMBytes != 12288<<20 {
		t.Fatalf("selected GPU = %#v, err = %v", gpu, err)
	}
}

func TestAutoBackendSelectionSkipsBackendsWithoutNativeReleaseValidation(t *testing.T) {
	root := t.TempDir()
	writeGPU := func(card, vendor, deviceID, vram string) {
		t.Helper()
		device := filepath.Join(root, card, "device")
		if err := os.MkdirAll(filepath.Join(device, "drm", "renderD"+strings.TrimPrefix(card, "card")), 0o700); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]string{
			"vendor": vendor, "device": deviceID, "mem_info_vram_total": vram, "boot_vga": "0",
		} {
			if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeGPU("card0", "0x10de", "0x2684", "12884901888")
	writeGPU("card1", "0x8086", "0xe20b", "12884901888")
	profile := probeHardwareWithOptionsAllowed("auto", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: root, dockerOnline: true,
		nvidiaToolkit: true, nvidiaSMI: "card0:2684, GPU-exact-uuid, 12288, 580.82\n",
	}, map[string]bool{"sycl": true})
	if profile.SelectedBackend != "sycl" || profile.SelectedDevice != "card1:e20b" {
		t.Fatalf("auto release-validated profile=%#v", profile)
	}
}

func TestProbeHardwareDoesNotTreatDRMCardIndexAsNVIDIAContainerIdentity(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "card2", "device")
	if err := os.MkdirAll(filepath.Join(device, "drm", "renderD130"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"vendor": "0x10de", "device": "0x2684", "mem_info_vram_total": "12884901888",
	} {
		if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	profile := probeHardwareWithOptions("cuda", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: root, dockerOnline: true,
		nvidiaSMI: "card2:2684, GPU-exact-uuid, 12288\n",
	})
	if profile.Compatibility != "unsupported" || profile.SelectedBackend != "" {
		t.Fatalf("CUDA without NVIDIA container runtime = %#v", profile)
	}
	if !containsString(profile.Warnings, "nvidia_container_toolkit_unavailable") {
		t.Fatalf("warnings = %#v", profile.Warnings)
	}
}

func TestProbeHardwarePrefersArcDGPUOverIntelIGPU(t *testing.T) {
	root := t.TempDir()
	writeIntel := func(card, deviceID, bootVGA, vram string) {
		t.Helper()
		device := filepath.Join(root, card, "device")
		if err := os.MkdirAll(filepath.Join(device, "drm", "renderD"+strings.TrimPrefix(card, "card")), 0o700); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]string{
			"vendor": "0x8086", "device": deviceID, "boot_vga": bootVGA, "mem_info_vram_total": vram,
		} {
			if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeIntel("card0", "0x7d55", "1", "0")
	writeIntel("card1", "0xe20b", "0", "12884901888")
	profile := probeHardwareWithOptions("auto", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: root, dockerOnline: true,
	})
	if profile.SelectedBackend != "sycl" || profile.SelectedDevice != "card1:e20b" ||
		profile.Compatibility != "recommended" {
		t.Fatalf("Intel hybrid profile = %#v", profile)
	}
}

func TestModernIntelIGPUWithSharedMemoryRemainsExperimental(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "card0", "device")
	if err := os.MkdirAll(filepath.Join(device, "drm", "renderD128"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"vendor": "0x8086", "device": "0x7d55", "boot_vga": "1", "mem_info_vram_total": "12884901888",
	} {
		if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	profile := probeHardwareWithOptions("sycl", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: root, dockerOnline: true,
	})
	if profile.Compatibility != "experimental" || !profile.AcknowledgementDue ||
		!containsString(profile.Warnings, "integrated_gpu_may_be_unacceptably_slow") {
		t.Fatalf("shared-memory iGPU profile=%#v", profile)
	}
}

func TestProbeHardwareCPUIsExplicitOnlyAndUnknownVRAMNeedsAcknowledgement(t *testing.T) {
	cpu := probeHardwareWithOptions("cpu", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: t.TempDir(), dockerOnline: true,
	})
	if cpu.SelectedBackend != "cpu" || cpu.Compatibility != "experimental" || !cpu.AcknowledgementDue {
		t.Fatalf("CPU profile = %#v", cpu)
	}
	if auto := probeHardwareWithOptions("auto", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: t.TempDir(), dockerOnline: true,
	}); auto.SelectedBackend == "cpu" {
		t.Fatalf("automatic selection silently chose CPU: %#v", auto)
	}

	root := t.TempDir()
	device := filepath.Join(root, "card3", "device")
	if err := os.MkdirAll(filepath.Join(device, "drm", "renderD131"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"vendor": "0x1002", "device": "0xffff", "boot_vga": "0",
	} {
		if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	unknown := probeHardwareWithOptions("vulkan", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: root, dockerOnline: true,
	})
	if unknown.Compatibility != "experimental" || !containsString(unknown.Warnings, "vram_unknown") {
		t.Fatalf("unknown-VRAM profile = %#v", unknown)
	}
}

func TestVulkanRequiresPassiveVersion12Verification(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "card0", "device")
	if err := os.MkdirAll(filepath.Join(device, "drm", "renderD128"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"vendor": "0x1002", "device": "0x744c", "boot_vga": "0", "mem_info_vram_total": "12884901888",
	} {
		if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	unverified := probeHardwareWithOptions("vulkan", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: root, dockerOnline: true,
	})
	if unverified.Compatibility != "experimental" || !unverified.AcknowledgementDue ||
		!containsString(unverified.Warnings, "vulkan_1_2_not_verified") {
		t.Fatalf("unverified Vulkan profile=%#v", unverified)
	}
	verified := probeHardwareWithOptions("vulkan", hardwareProbeOptions{
		goos: "linux", goarch: "amd64", drmRoot: root, dockerOnline: true,
		vulkanSummary: "Vulkan Instance Version: 1.3.280\n",
	})
	if !verified.Vulkan12Verified || verified.Compatibility != "recommended" {
		t.Fatalf("verified Vulkan profile=%#v", verified)
	}
}

func TestExactSmokeToolCallRejectsExtraCallsAndArguments(t *testing.T) {
	valid := json.RawMessage(`{"type":"function","function":{"name":"report_status","arguments":"{\"status\":\"ok\"}"}}`)
	if !validReportStatusToolCall(valid) {
		t.Fatal("exact report_status call was rejected")
	}
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"type":"function","function":{"name":"other","arguments":"{\"status\":\"ok\"}"}}`),
		json.RawMessage(`{"type":"function","function":{"name":"report_status","arguments":"{\"status\":\"ready\"}"}}`),
		json.RawMessage(`{"type":"function","function":{"name":"report_status","arguments":"{\"status\":\"ok\",\"extra\":true}"}}`),
	} {
		if validReportStatusToolCall(raw) {
			t.Fatalf("invalid smoke call was accepted: %s", raw)
		}
	}
}

func TestStartupManifestAttestsExactDeviceHashesParametersAndOffload(t *testing.T) {
	plan := runtimePlan{
		Config: config.LocalLLMConfig{Backend: "sycl"},
		Profile: HardwareProfile{
			SelectedBackend: "sycl", SelectedDevice: "0000:03:00.0",
			Devices: []GPUDevice{{
				ID: "0000:03:00.0", DockerID: "1", RenderNode: "/dev/dri/renderD129",
			}},
		},
		Model:              Artifact{SHA256: strings.Repeat("a", 64)},
		Draft:              &Artifact{SHA256: strings.Repeat("b", 64)},
		Image:              Image{Reference: "image@sha256:" + strings.Repeat("c", 64)},
		ResolvedParameters: []string{"--ctx-size=8192", "--spec-draft-n-max=2"},
	}
	valid := startupManifest{
		GPUOffload: true, KVOffload: true, MemoryProfileVerified: true,
		ImageDigest:        "sha256:" + strings.Repeat("c", 64),
		TargetSHA256:       strings.Repeat("a", 64),
		DraftSHA256:        strings.Repeat("b", 64),
		PhysicalDevice:     "0000:03:00.0",
		ActualDevice:       "SYCL0",
		ResolvedParameters: []string{"--ctx-size=8192", "--spec-draft-n-max=2"},
	}
	if err := validateStartupManifest(plan, valid); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	for name, mutate := range map[string]func(*startupManifest){
		"model hash":      func(value *startupManifest) { value.TargetSHA256 = strings.Repeat("d", 64) },
		"draft hash":      func(value *startupManifest) { value.DraftSHA256 = strings.Repeat("d", 64) },
		"physical device": func(value *startupManifest) { value.PhysicalDevice = "0000:04:00.0" },
		"actual device":   func(value *startupManifest) { value.ActualDevice = "/dev/dri/renderD128" },
		"parameters":      func(value *startupManifest) { value.ResolvedParameters = []string{"--ctx-size=2048"} },
		"kv offload":      func(value *startupManifest) { value.KVOffload = false },
		"memory profile":  func(value *startupManifest) { value.MemoryProfileVerified = false },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := valid
			invalid.ResolvedParameters = append([]string(nil), valid.ResolvedParameters...)
			mutate(&invalid)
			if err := validateStartupManifest(plan, invalid); err == nil {
				t.Fatalf("invalid startup manifest accepted: %#v", invalid)
			}
		})
	}
	cpuPlan := plan
	cpuPlan.Config.Backend = "cpu"
	cpuPlan.Profile = HardwareProfile{SelectedBackend: "cpu"}
	cpuPlan.Draft = nil
	cpu := valid
	cpu.GPUOffload = false
	cpu.KVOffload = false
	cpu.DraftSHA256 = ""
	cpu.PhysicalDevice = ""
	cpu.ActualDevice = "cpu"
	if err := validateStartupManifest(cpuPlan, cpu); err != nil {
		t.Fatalf("explicit CPU manifest rejected: %v", err)
	}
	cpu.GPUOffload = true
	if err := validateStartupManifest(cpuPlan, cpu); err == nil {
		t.Fatal("CPU manifest with claimed GPU offload was accepted")
	}
}

func TestStaleGenerationCannotPublishFailureOrVerification(t *testing.T) {
	manager := &Manager{
		generation: 2,
		status: Status{
			State: "ready_to_install", DesiredFingerprint: "new",
		},
	}
	err := manager.failGeneration(1, "download_model_failed", errors.New("old operation"))
	if errorCode(err) != "desired_state_changed" {
		t.Fatalf("stale error=%v", err)
	}
	status := manager.Status()
	if status.State != "ready_to_install" || status.ErrorCode != "" {
		t.Fatalf("stale generation changed status=%#v", status)
	}
}

func TestContextCapabilityCacheIncludesDriverAndBackendFingerprint(t *testing.T) {
	manager := &Manager{modelDir: t.TempDir()}
	plan := runtimePlan{
		Profile: HardwareProfile{Fingerprint: "driver-a", SelectedBackend: "sycl"},
		Model:   Artifact{SHA256: strings.Repeat("a", 64)},
		Image:   Image{Reference: "image@sha256:" + strings.Repeat("b", 64)},
	}
	if err := manager.saveContextCapability(plan, 32768); err != nil {
		t.Fatal(err)
	}
	if size, ok := manager.loadContextCapability(plan); !ok || size != 32768 {
		t.Fatalf("cached context size=%d ok=%v", size, ok)
	}
	plan.Profile.Fingerprint = "driver-b"
	if size, ok := manager.loadContextCapability(plan); ok {
		t.Fatalf("stale hardware cache accepted size=%d", size)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestContainerSpecHardeningAndMTPPair(t *testing.T) {
	originalStat := statRenderNode
	statRenderNode = func(path string) (fileStat, error) {
		if path != "/dev/dri/renderD129" {
			t.Fatalf("stat render node path=%q", path)
		}
		return fileStat{groupID: "109"}, nil
	}
	t.Cleanup(func() { statRenderNode = originalStat })
	cfg := &config.Config{}
	cfg.LocalLLM = config.LocalLLMConfig{Enabled: true, Backend: "sycl", ModelVariant: "q4_k_m", MTP: "mtp2", ContextSize: 8192, ListenPort: 18081}
	cfg.Directories.DataDir = t.TempDir()
	manager := NewManager(cfg, nil, nil)
	defer manager.Close()
	manager.desiredFingerprint = "fingerprint"
	profile := HardwareProfile{
		SelectedBackend: "sycl", SelectedDevice: "0000:03:00.0",
		Devices: []GPUDevice{{ID: "0000:03:00.0", RenderNode: "/dev/dri/renderD129", DockerID: "1"}},
	}
	model := DefaultManifest().Artifacts["mtp_target_q4_k_m"]
	draft := DefaultManifest().Artifacts["mtp_sidecar_q4_k_m"]
	spec, err := manager.containerSpec(profile, model, &draft, Image{Reference: "image@sha256:" + strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if spec.User != "65532:65532" || !spec.HostConfig.ReadonlyRootfs || spec.HostConfig.PidsLimit != 512 {
		t.Fatalf("hardening = %#v", spec)
	}
	if tmpfs := spec.HostConfig.Tmpfs["/tmp"]; !strings.Contains(tmpfs, "size=12g") ||
		!strings.Contains(tmpfs, "nosuid") || !strings.Contains(tmpfs, "nodev") {
		t.Fatalf("tmpfs hardening = %q", tmpfs)
	}
	if len(spec.HostConfig.Devices) != 1 || spec.HostConfig.Devices[0].PathOnHost != "/dev/dri/renderD129" {
		t.Fatalf("device mapping = %#v", spec.HostConfig.Devices)
	}
	if got := strings.Join(spec.HostConfig.GroupAdd, ","); got != "109" {
		t.Fatalf("native GPU groups=%q", got)
	}
	if spec.HostConfig.NetworkMode != "bridge" ||
		len(spec.HostConfig.PortBindings["8080/tcp"]) != 1 ||
		spec.HostConfig.PortBindings["8080/tcp"][0].HostIP != "127.0.0.1" {
		t.Fatalf("native network contract = %#v", spec.HostConfig)
	}
	runtimeMountFound := false
	for _, mount := range spec.HostConfig.Mounts {
		if mount.Source == runtimeKeyVolumeName {
			runtimeMountFound = mount.ReadOnly && mount.Target == "/run/aurago-local-llm"
		}
	}
	if !runtimeMountFound {
		t.Fatalf("runtime-key volume is not mounted read-only: %#v", spec.HostConfig.Mounts)
	}
	joined := strings.Join(spec.Env, "\n")
	for _, required := range []string{
		"AURAGO_BACKEND=sycl",
		"AURAGO_DEVICE=SYCL0",
		"AURAGO_DRAFT_DEVICE=SYCL0",
		"AURAGO_GPU_LAYERS=all",
		"ONEAPI_DEVICE_SELECTOR=level_zero:gpu",
		"AURAGO_FIT=off",
		"AURAGO_KV_OFFLOAD=on",
		"AURAGO_REASONING=off",
		"AURAGO_SPEC_DRAFT_N_MAX=2",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("missing %s in env:\n%s", required, joined)
		}
	}
	if strings.Contains(joined, "api-key=") {
		t.Fatal("runtime key leaked into container environment")
	}
}

func TestContainerSpecInsideDockerUsesPrivateApplicationNetworkOnly(t *testing.T) {
	t.Setenv("AURAGO_GPU_GROUP_IDS", "109,44,109")
	cfg := &config.Config{}
	cfg.Runtime.IsDocker = true
	cfg.LocalLLM = config.LocalLLMConfig{
		Enabled: true, Backend: "sycl", ModelVariant: "q4_k_m",
		MTP: "off", ContextSize: 8192, ListenPort: 18081,
	}
	cfg.Directories.DataDir = t.TempDir()
	manager := NewManager(cfg, nil, nil)
	defer manager.Close()
	profile := HardwareProfile{
		SelectedBackend: "sycl", SelectedDevice: "0000:03:00.0",
		Devices: []GPUDevice{{ID: "0000:03:00.0", RenderNode: "/dev/dri/renderD129", DockerID: "1"}},
	}
	spec, err := manager.containerSpec(
		profile,
		DefaultManifest().Artifacts["normal_q4_k_m"],
		nil,
		Image{Reference: "image@sha256:" + strings.Repeat("a", 64)},
	)
	if err != nil {
		t.Fatal(err)
	}
	if spec.HostConfig.NetworkMode != "aurago-app" || len(spec.HostConfig.PortBindings) != 0 {
		t.Fatalf("Docker network contract = %#v", spec.HostConfig)
	}
	if got := strings.Join(spec.HostConfig.GroupAdd, ","); got != "109,44" {
		t.Fatalf("GPU groups = %q", got)
	}
	for _, mount := range spec.HostConfig.Mounts {
		if (mount.Source == "aurago_models" || mount.Source == runtimeKeyVolumeName) && !mount.ReadOnly {
			t.Fatalf("managed volume is writable in sidecar: %#v", mount)
		}
	}
}

func TestAuraGoImagePrecreatesWritableNestedModelMountpoint(t *testing.T) {
	dockerfile, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dockerfile), "/app/data/models/aurago-qwen35") {
		t.Fatal("AuraGo image does not precreate the nested aurago_models mountpoint before ownership is assigned")
	}
}

func TestContainerSpecRejectsMissingNativeRenderNodeGroup(t *testing.T) {
	originalStat := statRenderNode
	statRenderNode = func(string) (fileStat, error) {
		return fileStat{}, errors.New("render node unavailable")
	}
	t.Cleanup(func() { statRenderNode = originalStat })
	cfg := &config.Config{}
	cfg.LocalLLM = config.LocalLLMConfig{
		Enabled: true, Backend: "vulkan", ModelVariant: "q4_k_m",
		MTP: "off", ContextSize: 8192, ListenPort: 18081,
	}
	manager := NewManager(cfg, nil, nil)
	defer manager.Close()
	profile := HardwareProfile{
		SelectedBackend: "vulkan", SelectedDevice: "0000:03:00.0",
		Devices: []GPUDevice{{ID: "0000:03:00.0", RenderNode: "/dev/dri/renderD129"}},
	}
	_, err := manager.containerSpec(
		profile,
		DefaultManifest().Artifacts["normal_q4_k_m"],
		nil,
		Image{Reference: "image@sha256:" + strings.Repeat("a", 64)},
	)
	if errorCode(err) != "gpu_group_ids_unavailable" {
		t.Fatalf("containerSpec() error = %v", err)
	}
}

func TestResolvedCPUParametersNeverRequestDraftGPUOffload(t *testing.T) {
	params := resolvedParametersForPlan(
		config.LocalLLMConfig{ContextSize: 8192},
		true,
		HardwareProfile{SelectedBackend: "cpu"},
	)
	joined := strings.Join(params, "\n")
	for _, expected := range []string{
		"--parallel=1",
		"--spec-type=draft-mtp",
		"--draft-device=cpu",
		"--spec-draft-ngl=0",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("CPU MTP parameters missing %q: %v", expected, params)
		}
	}
	if strings.Contains(joined, "--spec-draft-ngl=999") ||
		strings.Contains(joined, "--n-gpu-layers=all") {
		t.Fatalf("CPU parameters requested GPU offload: %v", params)
	}
}

type recordingDockerEngine struct {
	client *http.Client
	paths  []string
	bodies []any
}

func (engine *recordingDockerEngine) DoJSON(_ context.Context, method, path string, body, _ any) (int, error) {
	engine.paths = append(engine.paths, method+" "+path)
	engine.bodies = append(engine.bodies, body)
	return http.StatusOK, nil
}

func (engine *recordingDockerEngine) HTTPClient() *http.Client { return engine.client }

func TestRuntimeKeyVolumeUsesMode0600ArchiveAndNoSecretEnvironment(t *testing.T) {
	const secret = "runtime-secret-must-not-leak"
	var header *tar.Header
	var copied string
	engine := &recordingDockerEngine{}
	engine.client = &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPut || !strings.Contains(request.URL.Path, "/archive") {
			t.Fatalf("unexpected Docker HTTP request: %s %s", request.Method, request.URL)
		}
		reader := tar.NewReader(request.Body)
		var err error
		header, err = reader.Next()
		if err != nil {
			t.Fatal(err)
		}
		payload, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		copied = string(payload)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})}
	manager := &Manager{docker: engine}
	if err := manager.prepareRuntimeKeyVolume(context.Background(), "image@sha256:"+strings.Repeat("a", 64), secret, "test-fingerprint"); err != nil {
		t.Fatal(err)
	}
	if header == nil || header.Name != "api-key" || header.Mode != 0o600 ||
		header.Uid != 65532 || header.Gid != 65532 || copied != secret+"\n" {
		t.Fatalf("runtime key archive header=%#v payload=%q", header, copied)
	}
	for _, body := range engine.bodies {
		if volume, ok := body.(map[string]any); ok && volume["Name"] == runtimeKeyVolumeName {
			labels, _ := volume["Labels"].(map[string]string)
			if labels["aurago.managed"] != "local-llm" ||
				labels["aurago.fingerprint"] != "test-fingerprint" {
				t.Fatalf("runtime volume labels=%#v", labels)
			}
		}
		if spec, ok := body.(dockerContainerSpec); ok {
			if strings.Contains(strings.Join(spec.Env, "\n"), secret) {
				t.Fatal("runtime key leaked into seed container environment")
			}
			if spec.Labels["aurago.managed"] != "local-llm" ||
				spec.Labels["aurago.fingerprint"] != "test-fingerprint" {
				t.Fatalf("seed labels=%#v", spec.Labels)
			}
		}
	}
	if strings.Contains(strings.Join(engine.paths, "\n"), secret) {
		t.Fatal("runtime key leaked into Docker API path")
	}
}

type localLLMTestVault struct{ values map[string]string }

func (vault *localLLMTestVault) ReadSecret(key string) (string, error) {
	value := vault.values[key]
	if value == "" {
		return "", os.ErrNotExist
	}
	return value, nil
}

func (vault *localLLMTestVault) WriteSecret(key, value string) error {
	vault.values[key] = value
	return nil
}

func TestFakeLlamaServerSmokeTestValidatesToolCallAndStartupManifest(t *testing.T) {
	key := "test-runtime-key"
	status := Status{
		ImageDigest: "sha256:" + strings.Repeat("a", 64),
		ModelSHA256: strings.Repeat("b", 64),
		ContextSize: 8192,
	}
	status.ResolvedParameters = resolvedParameters(config.LocalLLMConfig{ContextSize: 8192}, false)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+key {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/chat/completions":
			_, _ = io.WriteString(writer, `{"choices":[{"message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"report_status","arguments":"{\"status\":\"ok\"}"}}]}}]}`)
		case "/startup-manifest":
			_, _ = io.WriteString(writer, `{"gpu_offload":false,"kv_offload":false,"memory_profile_verified":true,`+
				`"image_digest":"`+status.ImageDigest+`","target_sha256":"`+status.ModelSHA256+`",`+
				`"draft_sha256":"","physical_device":"","actual_device":"cpu",`+
				`"resolved_parameters":["--alias=aurago-qwen","--fit=off","--kv-offload=on","--reasoning=off","--ctx-size=8192"]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	host, portText, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil || host != "127.0.0.1" {
		t.Fatalf("test server address = %s, err = %v", server.URL, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	manager := &Manager{
		cfg: config.LocalLLMConfig{Enabled: true, Backend: "cpu", ContextSize: 8192, ListenPort: port},
		vault: &localLLMTestVault{values: map[string]string{
			config.LocalLLMRuntimeAPIKeyVaultKey: key,
		}},
		status: status,
	}
	manager.desiredFingerprint = "smoke-plan"
	plan := runtimePlan{
		Config: config.LocalLLMConfig{Enabled: true, Backend: "cpu", ContextSize: 8192, ListenPort: port},
		Profile: HardwareProfile{
			SelectedBackend: "cpu",
		},
		Fingerprint: "smoke-plan",
		Model:       Artifact{SHA256: status.ModelSHA256},
		Image:       Image{Reference: "image@" + status.ImageDigest},
		ResolvedParameters: append(
			[]string(nil),
			status.ResolvedParameters...,
		),
	}
	if err := manager.smokeTestPlan(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	verified := manager.Status()
	if !verified.ToolCallVerified || !verified.MemoryProfileVerified || verified.ActualDevice != "cpu" {
		t.Fatalf("verified status = %#v", verified)
	}
}

func TestReleaseBodyHoldsActiveRequestUntilClose(t *testing.T) {
	manager := &Manager{cfg: config.LocalLLMConfig{IdleTimeoutMinutes: 15}}
	manager.acquire()
	body := &releaseBody{ReadCloser: io.NopCloser(strings.NewReader("ok")), release: manager.release}
	if manager.Status().ActiveRequests != 1 {
		t.Fatal("request was not counted")
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if manager.Status().ActiveRequests != 0 || manager.Status().IdleDeadline == nil {
		t.Fatalf("status after close = %#v", manager.Status())
	}
}

func TestLocalUnavailableHTTPStatusesAreImmediateFailoverOnly(t *testing.T) {
	for _, status := range []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		if !isLocalUnavailableStatus(status) {
			t.Fatalf("status %d was not classified unavailable", status)
		}
	}
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError} {
		if isLocalUnavailableStatus(status) {
			t.Fatalf("ordinary provider status %d was classified immediate", status)
		}
	}
}

func TestConfigureDefersDesiredStateUntilActiveRequestCloses(t *testing.T) {
	cfg := &config.Config{}
	cfg.LocalLLM = config.LocalLLMConfig{
		Enabled: true, Backend: "sycl", ModelVariant: "q4_k_m", MTP: "off",
		ContextSize: 8192, IdleTimeoutMinutes: 15, ListenPort: 18081,
	}
	manager := NewManager(cfg, nil, nil)
	defer manager.Close()
	manager.mu.Lock()
	manager.status.State = "stopped"
	manager.status.ToolCallVerified = true
	manager.status.GPUOffloadVerified = true
	manager.status.MemoryProfileVerified = true
	manager.status.VerifiedFingerprint = "old"
	manager.status.VerifiedContextSize = 32768
	manager.mu.Unlock()
	manager.acquire()
	updated := cfg.LocalLLM
	updated.ContextSize = 2048
	manager.Configure(updated)
	if status := manager.Status(); !status.PendingRestart || status.ActiveRequests != 1 ||
		status.ToolCallVerified || status.GPUOffloadVerified || status.MemoryProfileVerified ||
		status.VerifiedFingerprint != "" || status.VerifiedContextSize != 0 {
		t.Fatalf("deferred status = %#v", status)
	}
	manager.release()
	manager.mu.Lock()
	applied := manager.cfg.ContextSize
	pending := manager.pendingCfg
	manager.mu.Unlock()
	if applied != 2048 || pending != nil {
		t.Fatalf("applied context=%d pending=%#v", applied, pending)
	}
}

func TestConcurrentStartWaiterHonorsCancellation(t *testing.T) {
	manager := &Manager{starting: true, startDone: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := make(chan error, 1)
	go func() { started <- manager.Start(ctx) }()
	select {
	case err := <-started:
		if err == nil || !strings.Contains(err.Error(), "local_start_cancelled") {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled singleflight waiter did not return")
	}
}

func TestConcurrentInstallWaitersShareResultAndRemainCancelable(t *testing.T) {
	sentinel := errors.New("shared install result")
	manager := &Manager{installing: true, installDone: make(chan struct{})}

	waiter := make(chan error, 1)
	go func() { waiter <- manager.Install(context.Background()) }()
	manager.mu.Lock()
	manager.installErr = sentinel
	close(manager.installDone)
	manager.mu.Unlock()
	select {
	case err := <-waiter:
		if !errors.Is(err, sentinel) {
			t.Fatalf("Install() waiter error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Install() waiter did not receive the shared result")
	}

	manager.installDone = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := manager.Install(ctx)
	if errorCode(err) != "local_install_cancelled" {
		t.Fatalf("cancelled Install() error = %v", err)
	}
}

func TestManagerControlGateWaitIsCancelable(t *testing.T) {
	lifecycle, lifecycleCancel := context.WithCancel(context.Background())
	defer lifecycleCancel()
	manager := &Manager{
		control:      make(chan struct{}, 1),
		lifecycleCtx: lifecycle,
	}
	release, err := manager.acquireControl(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.acquireControl(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting control error=%v, want context cancellation", err)
	}
	release()
}

func TestShutdownWaitsForActiveResponseThenCleansEphemeralResources(t *testing.T) {
	engine := &recordingDockerEngine{client: http.DefaultClient}
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	manager := NewManager(cfg, nil, nil, withDockerEngine(engine))
	manager.acquire()
	go func() {
		time.Sleep(20 * time.Millisecond)
		manager.release()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(engine.paths, "\n")
	for _, expected := range []string{
		"containers/" + managedContainerName,
		"containers/" + runtimeKeySeedName,
		"volumes/" + runtimeKeyVolumeName,
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("shutdown did not clean %q; paths=%s", expected, joined)
		}
	}
}

func TestShutdownRejectsNewRequestLeasesAtomically(t *testing.T) {
	engine := &recordingDockerEngine{client: http.DefaultClient}
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	manager := NewManager(cfg, nil, nil, withDockerEngine(engine))
	manager.acquire()

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		done <- manager.Shutdown(ctx)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		manager.mu.Lock()
		shuttingDown := manager.shuttingDown
		manager.mu.Unlock()
		if shuttingDown {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("manager did not enter shutdown")
		}
		time.Sleep(time.Millisecond)
	}
	if err := manager.acquireRequest(); err == nil || !strings.Contains(err.Error(), "local_llm_shutting_down") {
		t.Fatalf("acquireRequest() during shutdown = %v, want typed shutdown rejection", err)
	}

	manager.release()
	if err := <-done; err != nil {
		t.Fatalf("Shutdown() failed: %v", err)
	}
}
