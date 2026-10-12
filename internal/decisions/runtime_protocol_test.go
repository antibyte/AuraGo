package decisions

import (
	"strings"
	"testing"
)

func baseRuntimeMetadata(device runtimeSelectedDevice, backend string) runtimeMetadata {
	manifest := DefaultManifest()
	return runtimeMetadata{
		Protocol: runtimeProtocol,
		Ready:    true,
		Model: runtimeModelPin{
			ID: modelID, Revision: manifest.Model.Revision, SHA256: manifest.Model.SHA256,
		},
		Projector: runtimeModelPin{Revision: manifest.Projector.Revision, SHA256: manifest.Projector.SHA256},
		Build:     runtimeBuildInfo{LlamaCPPCommit: llamaCPPCommit, LlamaCPPPatchSHA256: llamaCPPPatchSHA256, BinarySHA256: strings.Repeat("a", 64), Backend: backend},
		Hardware: runtimeHardware{
			SelectedDevice: device,
			Offload:        runtimeOffload{Attested: true},
		},
		ModelLayers: 32,
		Placement:   runtimePlacement{TotalModelLayers: 32},
		ImageMarker: "<|image|>",
		Limits:      runtimeLimits{ContextTokens: maxEvaluationTokens, MaxRequestBytes: maxRuntimeRequestBytes, MaxImages: maxImages, MaxImageBytes: maxImageBytes, MaxTotalImageBytes: maxTotalImageBytes},
	}
}

func TestValidateRuntimeMetadataRequiresFullGPUOffloadWithoutCPUConsent(t *testing.T) {
	selected := runtimeDevice{ID: "cuda0", Name: "GPU 0", Backend: "cuda", Type: "GPU", BinarySHA: strings.Repeat("a", 64)}
	metadata := baseRuntimeMetadata(runtimeSelectedDevice{ID: selected.ID, Name: selected.Name, Backend: selected.Backend, Type: selected.Type}, "cuda")
	metadata.Hardware.Offload.ModelLayersOnDevice = true
	metadata.Hardware.Offload.ProjectorOnDevice = true
	metadata.Hardware.Offload.ProjectorComputeOnDevice = true
	metadata.Hardware.Offload.KVOnDevice = true
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentNative, false); err != nil {
		t.Fatalf("fully offloaded GPU runtime rejected: %v", err)
	}
	metadata.Hardware.Offload.ModelCPUCompute = true
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentNative, false); err == nil {
		t.Fatal("CPU participation was accepted without consent")
	}
	metadata.Hardware.Offload.ModelCPUCompute = false
	metadata.Hardware.Offload.ProjectorOnDevice = false
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentNative, false); err == nil {
		t.Fatal("unattested CPU projector placement was accepted")
	}
}

func TestValidateRuntimeMetadataAllowsOnlyAttestedConsentedHybridPlacement(t *testing.T) {
	selected := runtimeDevice{ID: "vulkan0", Name: "Integrated GPU", Backend: "vulkan", Type: "GPU", GPULayers: 24, ModelLayers: 32, BinarySHA: strings.Repeat("b", 64)}
	metadata := baseRuntimeMetadata(runtimeSelectedDevice{ID: selected.ID, Name: selected.Name, Backend: selected.Backend, Type: selected.Type}, "vulkan")
	metadata.Build.BinarySHA256 = selected.BinarySHA
	metadata.Hardware.Offload.ModelCPUCompute = true
	metadata.Hardware.Offload.ProjectorOnDevice = true
	metadata.Hardware.Offload.ProjectorComputeOnDevice = true
	metadata.Hardware.Offload.KVOnDevice = true
	metadata.Placement = runtimePlacement{Hybrid: true, RequestedGPULayers: 24, TotalModelLayers: 32}
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentDocker, false); err == nil {
		t.Fatal("hybrid placement was accepted without explicit consent")
	}
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentDocker, true); err != nil {
		t.Fatalf("consented, attested GPU-projector hybrid placement rejected: %v", err)
	}
	metadata.Hardware.Offload.ProjectorOnDevice = false
	metadata.Hardware.Offload.ProjectorComputeOnDevice = false
	metadata.Hardware.Offload.ProjectorCPUCompute = true
	metadata.Hardware.Offload.ProjectorWeightsOnCPU = true
	metadata.Hardware.Offload.ProjectorComputeOnCPU = true
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentDocker, true); err != nil {
		t.Fatalf("consented, attested CPU-projector hybrid placement rejected: %v", err)
	}
	metadata.Hardware.Offload.ProjectorOnDevice = true
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentDocker, true); err == nil {
		t.Fatal("hybrid projector with split or incomplete placement was accepted")
	}
	metadata.Hardware.Offload.ProjectorOnDevice = false
	metadata.Hardware.Offload.ProjectorComputeOnDevice = false
	metadata.Hardware.Offload.ProjectorCPUCompute = true
	metadata.Hardware.Offload.ProjectorWeightsOnCPU = true
	metadata.Hardware.Offload.ProjectorComputeOnCPU = true
	metadata.Placement.RequestedGPULayers = 8
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentDocker, true); err == nil {
		t.Fatal("runtime silently changed the requested hybrid layer count")
	}
}

func TestValidateRuntimeMetadataRequiresExpectedDeviceAndAttestation(t *testing.T) {
	selected := runtimeDevice{ID: "metal0", Name: "Apple GPU", Backend: "metal", Type: "GPU", BinarySHA: strings.Repeat("c", 64)}
	metadata := baseRuntimeMetadata(runtimeSelectedDevice{ID: "other", Name: selected.Name, Backend: selected.Backend, Type: selected.Type}, "metal")
	metadata.Hardware.Offload.ModelLayersOnDevice = true
	metadata.Hardware.Offload.ProjectorOnDevice = true
	metadata.Hardware.Offload.ProjectorComputeOnDevice = true
	metadata.Hardware.Offload.KVOnDevice = true
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentNative, false); err == nil {
		t.Fatal("runtime selecting an unexpected device was accepted")
	}
	metadata.Hardware.SelectedDevice.ID = selected.ID
	metadata.Hardware.Offload.Attested = false
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentNative, false); err == nil {
		t.Fatal("unattested compute placement was accepted")
	}
}

func TestValidateRuntimeMetadataRequiresExplicitConsentAndCompleteCPUPlacement(t *testing.T) {
	selected := runtimeDevice{ID: "cpu0", Name: "CPU", Backend: "cpu", Type: "CPU", BinarySHA: strings.Repeat("d", 64), ModelLayers: 32}
	metadata := baseRuntimeMetadata(runtimeSelectedDevice{ID: selected.ID, Name: selected.Name, Backend: selected.Backend, Type: selected.Type}, "cpu")
	metadata.Build.BinarySHA256 = selected.BinarySHA
	metadata.Hardware.Offload.ModelCPUCompute = true
	metadata.Hardware.Offload.ModelLayersOnCPU = true
	metadata.Hardware.Offload.ProjectorCPUCompute = true
	metadata.Hardware.Offload.ProjectorWeightsOnCPU = true
	metadata.Hardware.Offload.ProjectorComputeOnCPU = true
	metadata.Hardware.Offload.KVCpu = true
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentNative, false); err == nil {
		t.Fatal("CPU runtime was accepted without persistent consent")
	}
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentNative, true); err != nil {
		t.Fatalf("fully attested consented CPU runtime rejected: %v", err)
	}
	metadata.Hardware.Offload.ProjectorWeightsOnCPU = false
	if err := validateRuntimeMetadata(metadata, selected, configDecisionsDeploymentNative, true); err == nil {
		t.Fatal("CPU runtime with unaccounted projector weights was accepted")
	}
}

func TestDecodeRuntimeErrorsClassifiesCancellationAndBudget(t *testing.T) {
	for input, want := range map[string]string{
		`{"error":{"code":"request_cancelled"}}`:           CodeCancelled,
		`{"error":{"code":"input_token_budget_exceeded"}}`: CodeInputTokenBudgetExceeded,
		`{"error":{"code":"candidate_not_single_token"}}`:  CodeCandidateNotScorable,
		`{"error":{"code":"runtime_busy"}}`:                CodeRuntimeBusy,
		`{"error":{"code":"runtime_not_ready"}}`:           CodeRuntimeUnavailable,
		`{"error":{"code":"device_attestation_changed"}}`:  CodeRuntimeAttestationFailed,
	} {
		if got := ErrorCode(decodeRuntimeError([]byte(input), 409)); got != want {
			t.Errorf("decodeRuntimeError(%s) = %q, want %q", input, got, want)
		}
	}
}

func TestRetiredRuntimeFailuresAreNonRetryableButBusyIsRetryable(t *testing.T) {
	for _, code := range []string{CodeRuntimeUnavailable, CodeRuntimeAttestationFailed, CodeInvalidRuntimeResponse, CodeInferenceFailed} {
		if !IsNonRetryable(NewError(code, nil)) {
			t.Errorf("retired runtime code %q was marked retryable", code)
		}
	}
	for _, code := range []string{CodeRuntimeBusy, CodeQueueFull, CodeCancelled} {
		if IsNonRetryable(NewError(code, nil)) {
			t.Errorf("request-local/transient code %q was marked non-retryable", code)
		}
	}
	for _, code := range []string{CodeInputTokenBudgetExceeded, CodeCandidateNotScorable} {
		if !IsNonRetryable(NewError(code, nil)) {
			t.Errorf("input-specific code %q was marked retryable", code)
		}
	}
}

func TestNativeStartupFailureOnlyPublishesAllowlistedCode(t *testing.T) {
	buffer := &nativeStartupStderr{}
	_, _ = buffer.Write([]byte("AURAGO_DECISIONS_STARTUP_ERROR code=offload_unavailable\n/path/to/private/model"))
	if got := buffer.code(); got != "offload_unavailable" {
		t.Fatalf("safe native startup code = %q", got)
	}
	if got := runtimeFailureReason(NewError(CodeRuntimeStartFailed, nativeStartupFailure{code: buffer.code()})); got != "offload_unavailable" {
		t.Fatalf("runtime failure reason = %q", got)
	}
	unsafe := &nativeStartupStderr{}
	_, _ = unsafe.Write([]byte("AURAGO_DECISIONS_STARTUP_ERROR code=offload_unavailable /private/model.gguf\n"))
	if got := unsafe.code(); got != "" {
		t.Fatalf("unsafe startup output was surfaced as a code: %q", got)
	}
	if got := runtimeFailureReason(NewError(CodeRuntimeStartFailed, nativeStartupFailure{code: "untrusted /private/path"})); got != CodeRuntimeStartFailed {
		t.Fatalf("unallowlisted failure reason = %q", got)
	}
}
