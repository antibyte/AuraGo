package decisions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/config"
)

const runtimeProtocol = "aurago.decisions.v1"
const maxRuntimeRequestBytes = 40 << 20

type runtimeFactory interface {
	discover(context.Context, string, RuntimeAccess, modelPaths, string) ([]runtimeDevice, error)
	start(context.Context, string, runtimeDevice, modelPaths, config.DecisionsConfig, RuntimeAccess, string, func(string)) (runtimeSession, error)
	install(context.Context, string, RuntimeAccess, string) error
}

type runtimeSession interface {
	metadata(context.Context) (runtimeMetadata, error)
	score(context.Context, runtimeScoreRequest) (runtimeScoreResult, error)
	Close(context.Context) error
}

type runtimeDevice struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Backend       string `json:"backend"`
	Type          string `json:"type"`
	Integrated    *bool  `json:"integrated"`
	MemoryTotal   int64  `json:"memory_total_bytes"`
	MemoryFree    int64  `json:"memory_free_bytes"`
	BinaryPath    string `json:"-"`
	BinarySHA     string `json:"-"`
	ModelLayers   int    `json:"-"`
	DockerImageID string `json:"-"`
	GPULayers     int    `json:"-"`
}

type runtimeScoreRequest struct {
	Prompt         string
	Candidates     []runtimeCandidate
	Images         []Image
	MaxInputTokens int
}

type runtimeScoreResult struct {
	Scores   []scoredCandidate
	usage    runtimeUsage
	Metadata runtimeMetadata
}

type runtimeUsage struct {
	PromptTokens  int `json:"prompt_tokens"`
	ImageTokens   int `json:"image_tokens"`
	ContextTokens int `json:"context_tokens"`
}

type runtimeMetadata struct {
	Protocol    string           `json:"protocol"`
	Ready       bool             `json:"ready"`
	Model       runtimeModelPin  `json:"model"`
	Projector   runtimeModelPin  `json:"projector"`
	Build       runtimeBuildInfo `json:"build"`
	Hardware    runtimeHardware  `json:"hardware"`
	Placement   runtimePlacement `json:"placement"`
	ModelLayers int              `json:"model_layers"`
	Limits      runtimeLimits    `json:"limits"`
	ImageMarker string           `json:"image_marker"`
	Deployment  string           `json:"-"`
}

type runtimeModelPin struct {
	ID       string `json:"id,omitempty"`
	Revision string `json:"revision"`
	SHA256   string `json:"sha256"`
}

type runtimeBuildInfo struct {
	LlamaCPPCommit      string `json:"llama_cpp_commit"`
	LlamaCPPPatchSHA256 string `json:"llama_cpp_patch_sha256"`
	BinarySHA256        string `json:"binary_sha256"`
	Backend             string `json:"backend"`
}

type runtimeHardware struct {
	SelectedDevice runtimeSelectedDevice `json:"selected_device"`
	Offload        runtimeOffload        `json:"offload"`
}

type runtimeSelectedDevice struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Backend    string `json:"backend"`
	Type       string `json:"type"`
	Integrated *bool  `json:"integrated"`
}

type runtimeOffload struct {
	Attested                 bool `json:"attested"`
	ModelLayersOnDevice      bool `json:"all_model_layers_on_device"`
	ModelLayersOnCPU         bool `json:"all_model_layers_on_cpu"`
	ProjectorOnDevice        bool `json:"projector_on_device"`
	ProjectorComputeOnDevice bool `json:"projector_compute_on_device"`
	ProjectorWeightsOnCPU    bool `json:"projector_weights_on_cpu"`
	ProjectorComputeOnCPU    bool `json:"projector_compute_on_cpu"`
	KVOnDevice               bool `json:"kv_on_device"`
	ModelCPUCompute          bool `json:"model_cpu_compute"`
	ProjectorCPUCompute      bool `json:"projector_cpu_compute"`
	KVCpu                    bool `json:"kv_cpu"`
}

type runtimePlacement struct {
	Hybrid             bool `json:"hybrid"`
	RequestedGPULayers int  `json:"requested_gpu_layers"`
	TotalModelLayers   int  `json:"total_model_layers"`
}

type runtimeLimits struct {
	ContextTokens      int   `json:"context_tokens"`
	MaxRequestBytes    int64 `json:"max_request_bytes"`
	MaxImageBytes      int64 `json:"max_image_bytes"`
	MaxImages          int   `json:"max_images"`
	MaxTotalImageBytes int64 `json:"max_total_image_bytes"`
}

type runtimeScoreWireRequest struct {
	RequestID      string             `json:"request_id"`
	Prompt         string             `json:"prompt"`
	Candidates     []runtimeCandidate `json:"candidates"`
	Images         []runtimeImageWire `json:"images"`
	MaxInputTokens int                `json:"max_input_tokens"`
}

type runtimeImageWire struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data_base64"`
}

type runtimeScoreWireResponse struct {
	RequestID   string            `json:"request_id"`
	Protocol    string            `json:"protocol"`
	Ready       bool              `json:"ready"`
	Model       runtimeModelPin   `json:"model"`
	Projector   runtimeModelPin   `json:"projector"`
	Build       runtimeBuildInfo  `json:"build"`
	Hardware    runtimeHardware   `json:"hardware"`
	Placement   runtimePlacement  `json:"placement"`
	ModelLayers int               `json:"model_layers"`
	Limits      runtimeLimits     `json:"limits"`
	ImageMarker string            `json:"image_marker"`
	Usage       runtimeUsage      `json:"usage"`
	Scores      []scoredCandidate `json:"scores"`
}

type runtimeWireError struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func (metadata runtimeMetadata) device() runtimeDevice {
	selected := metadata.Hardware.SelectedDevice
	return runtimeDevice{ID: selected.ID, Name: selected.Name, Backend: selected.Backend, Type: selected.Type, Integrated: selected.Integrated}
}

func validateRuntimeMetadata(metadata runtimeMetadata, selected runtimeDevice, deployment string, cpuAllowed bool) error {
	manifest := DefaultManifest()
	if metadata.Protocol != runtimeProtocol || !metadata.Ready {
		return fmt.Errorf("runtime did not report the required ready protocol")
	}
	if metadata.Model.ID != modelID || metadata.Model.Revision != manifest.Model.Revision || !strings.EqualFold(metadata.Model.SHA256, manifest.Model.SHA256) {
		return fmt.Errorf("runtime model pin differs from application pin")
	}
	if metadata.Projector.Revision != manifest.Projector.Revision || !strings.EqualFold(metadata.Projector.SHA256, manifest.Projector.SHA256) {
		return fmt.Errorf("runtime projector pin differs from application pin")
	}
	if metadata.Build.LlamaCPPCommit != llamaCPPCommit || metadata.Build.LlamaCPPPatchSHA256 != llamaCPPPatchSHA256 || !validSHA256(metadata.Build.BinarySHA256) ||
		(metadata.Build.Backend != "cpu" && metadata.Build.Backend != "cuda" && metadata.Build.Backend != "vulkan" && metadata.Build.Backend != "metal") {
		return fmt.Errorf("runtime build is not pinned or verifiable")
	}
	if !strings.EqualFold(metadata.Build.BinarySHA256, selected.BinarySHA) && selected.BinarySHA != "" {
		return fmt.Errorf("runtime binary differs from discovered binary")
	}
	hardware := metadata.Hardware
	device := hardware.SelectedDevice
	if device.ID == "" || device.ID != selected.ID || (selected.Name != "" && device.Name != selected.Name) ||
		!strings.EqualFold(normalizeDeviceType(device.Type), normalizeDeviceType(selected.Type)) ||
		!deviceBoolEqual(device.Integrated, selected.Integrated) || !strings.EqualFold(device.Backend, selected.Backend) ||
		(metadata.Build.Backend != "cpu" && !strings.EqualFold(metadata.Build.Backend, selected.Backend)) {
		return fmt.Errorf("runtime selected a different device or backend")
	}
	if !strings.EqualFold(device.Type, "CPU") && !strings.EqualFold(device.Type, "GPU") && !strings.EqualFold(device.Type, "IGPU") && !strings.EqualFold(device.Type, "ACCEL") {
		return fmt.Errorf("runtime selected an unsupported device class")
	}
	if deployment != configDecisionsDeploymentNative && deployment != configDecisionsDeploymentDocker {
		return fmt.Errorf("runtime deployment is invalid")
	}
	offload := hardware.Offload
	if !offload.Attested {
		return fmt.Errorf("runtime could not attest compute placement")
	}
	cpuUsed := offload.ModelCPUCompute || offload.ModelLayersOnCPU || offload.ProjectorCPUCompute || offload.ProjectorWeightsOnCPU ||
		offload.ProjectorComputeOnCPU || offload.KVCpu || strings.EqualFold(device.Type, "CPU")
	if cpuUsed && !cpuAllowed {
		return fmt.Errorf("runtime reported CPU compute without persistent consent")
	}
	if strings.EqualFold(device.Type, "CPU") {
		if metadata.Build.Backend != "cpu" || metadata.ModelLayers < 1 || metadata.Placement.Hybrid || metadata.Placement.RequestedGPULayers != 0 ||
			metadata.Placement.TotalModelLayers != metadata.ModelLayers || !offload.ModelCPUCompute || offload.ModelLayersOnDevice || !offload.ModelLayersOnCPU ||
			offload.ProjectorOnDevice || offload.ProjectorComputeOnDevice || !offload.ProjectorWeightsOnCPU || !offload.ProjectorComputeOnCPU ||
			!offload.ProjectorCPUCompute || offload.KVOnDevice || !offload.KVCpu {
			return fmt.Errorf("CPU runtime did not attest the selected CPU execution mode")
		}
		return nil
	}
	if !offload.ModelLayersOnDevice && !offload.ModelCPUCompute ||
		!offload.ProjectorOnDevice && !offload.ProjectorComputeOnDevice && !offload.ProjectorCPUCompute || !offload.KVOnDevice && !offload.KVCpu {
		return fmt.Errorf("runtime did not account for model, projector, and KV placement")
	}
	if cpuUsed && (!metadata.Placement.Hybrid || offload.ModelLayersOnCPU || selected.GPULayers < 1 || metadata.Placement.RequestedGPULayers != selected.GPULayers ||
		metadata.Placement.RequestedGPULayers < 1 || metadata.Placement.TotalModelLayers < 2 || metadata.Placement.RequestedGPULayers >= metadata.Placement.TotalModelLayers ||
		metadata.Placement.TotalModelLayers != metadata.ModelLayers || metadata.ModelLayers < 1 || offload.ModelLayersOnDevice || !offload.ModelCPUCompute) {
		return fmt.Errorf("runtime did not attest the authorized hybrid layer placement")
	}
	if cpuUsed && !((offload.ProjectorOnDevice && offload.ProjectorComputeOnDevice && !offload.ProjectorWeightsOnCPU && !offload.ProjectorComputeOnCPU && !offload.ProjectorCPUCompute) ||
		(!offload.ProjectorOnDevice && !offload.ProjectorComputeOnDevice && offload.ProjectorWeightsOnCPU && offload.ProjectorComputeOnCPU && offload.ProjectorCPUCompute)) {
		return fmt.Errorf("runtime did not attest a complete hybrid projector placement")
	}
	if !cpuUsed && (metadata.Placement.Hybrid || metadata.Placement.RequestedGPULayers != 0 || metadata.Placement.TotalModelLayers != metadata.ModelLayers ||
		metadata.ModelLayers < 1 || !offload.ModelLayersOnDevice || offload.ModelLayersOnCPU || !offload.ProjectorOnDevice || !offload.ProjectorComputeOnDevice || !offload.KVOnDevice ||
		offload.ModelCPUCompute || offload.ProjectorWeightsOnCPU || offload.ProjectorComputeOnCPU || offload.ProjectorCPUCompute || offload.KVCpu) {
		return fmt.Errorf("runtime did not attest full GPU model, projector, and KV placement")
	}
	if metadata.ModelLayers < 1 {
		return fmt.Errorf("runtime did not report the loaded model layer count")
	}
	return nil
}

func normalizeDeviceType(value string) string {
	if strings.EqualFold(value, "IGPU") {
		return "GPU"
	}
	return strings.ToUpper(value)
}

func deviceBoolEqual(left, right *bool) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func metadataUsesCPU(metadata runtimeMetadata) bool {
	offload := metadata.Hardware.Offload
	return offload.ModelCPUCompute || offload.ModelLayersOnCPU || offload.ProjectorCPUCompute || offload.ProjectorWeightsOnCPU ||
		offload.ProjectorComputeOnCPU || offload.KVCpu || strings.EqualFold(metadata.Hardware.SelectedDevice.Type, "CPU")
}

const (
	configDecisionsDeploymentNative = "native"
	configDecisionsDeploymentDocker = "docker"
	llamaCPPCommit                  = "d81235049384534c167caea52b85a694f6103d14"
	llamaCPPPatchSHA256             = "a3e895bce9fafa23873a481ae29f31fdb98287e0f02939bdd1213c103271a790"
)

func statusFromMetadata(cfg config.DecisionsConfig, deployment string, metadata runtimeMetadata, cpuActive bool) Status {
	device := metadata.Hardware.SelectedDevice
	info := runtimeInfo(metadata)
	info.Deployment = deployment
	mode := "gpu"
	if cpuActive {
		mode = "mixed"
		if strings.EqualFold(device.Type, "CPU") {
			mode = "cpu"
		}
	}
	return Status{
		Enabled: cfg.Enabled, Installed: true, Ready: true, State: "ready", Deployment: deployment,
		ExecutionMode: mode, Backend: metadata.Build.Backend,
		Device: device.Name, ModelRevision: metadata.Model.Revision, ModelSHA256: metadata.Model.SHA256,
		ProjectorRevision: metadata.Projector.Revision, ProjectorSHA256: metadata.Projector.SHA256,
		DeviceAttested: metadata.Hardware.Offload.Attested, GPUOffloadVerified: !cpuActive,
		CPUFallbackAllowed: cfg.CPUFallbackAllowed, CPUFallbackActive: cpuActive, Runtime: info,
	}
}

func runtimeInfo(metadata runtimeMetadata) RuntimeInfo {
	device := metadata.Hardware.SelectedDevice
	offload := metadata.Hardware.Offload
	return RuntimeInfo{
		Deployment: metadata.Deployment, Backend: metadata.Build.Backend, Device: device.Name,
		RuntimeBuild: metadata.Build.LlamaCPPCommit, ModelRevision: metadata.Model.Revision, ModelSHA256: metadata.Model.SHA256,
		ProjectorRevision: metadata.Projector.Revision, ProjectorSHA256: metadata.Projector.SHA256,
		RuntimeBinarySHA256: metadata.Build.BinarySHA256,
		GPUOffloadVerified: offload.Attested && offload.ModelLayersOnDevice && offload.ProjectorOnDevice && offload.ProjectorComputeOnDevice && offload.KVOnDevice &&
			!offload.ModelCPUCompute && !offload.ModelLayersOnCPU && !offload.ProjectorWeightsOnCPU && !offload.ProjectorComputeOnCPU && !offload.ProjectorCPUCompute && !offload.KVCpu,
		DeviceAttested: offload.Attested,
	}
}

func decodeRuntimeError(data []byte, status int) error {
	var wire runtimeWireError
	if json.Unmarshal(data, &wire) == nil {
		switch wire.Error.Code {
		case "candidate_not_single_token":
			return NewError(CodeCandidateNotScorable, fmt.Errorf("runtime rejected candidate tokenization"))
		case "input_token_budget_exceeded", "context_exceeded":
			return NewError(CodeInputTokenBudgetExceeded, fmt.Errorf("runtime input exceeded token budget"))
		case "invalid_request":
			return NewError(CodeInvalidRequest, fmt.Errorf("runtime rejected request"))
		case "request_cancelled":
			return NewError(CodeCancelled, context.Canceled)
		case "runtime_busy":
			return NewError(CodeRuntimeBusy, nil)
		case "runtime_not_ready":
			return NewError(CodeRuntimeUnavailable, nil)
		case "device_attestation_changed":
			return NewError(CodeRuntimeAttestationFailed, nil)
		}
	}
	return NewError(CodeInferenceFailed, fmt.Errorf("runtime returned HTTP %d", status))
}
