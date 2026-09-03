package localllm

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	ReleaseManifestVersion = 4
	LlamaCPPCommit         = "555881ebc8b0fc0402b30e09258a32a7bfd13c52"
	ggufRepositoryRevision = "37e44d3534c05447be9e486cadca5d1da9838539"
	mtpRepositoryRevision  = "abf7f625cc52c019ef5a14afa0c56713d5183818"
)

// Artifact is one immutable model file.
type Artifact struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
	Path       string `json:"path"`
	Revision   string `json:"revision"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
}

func (a Artifact) URL() string {
	return fmt.Sprintf("https://huggingface.co/%s/resolve/%s/%s", a.Repository, a.Revision, a.Path)
}

// Image is one digest-pinned llama.cpp runtime backend.
type Image struct {
	Backend   string `json:"backend"`
	Reference string `json:"reference"`
	Supported bool   `json:"supported"`
}

// ValidatedHardwareProfile records native Linux acceptance for one exact family.
type ValidatedHardwareProfile struct {
	ID      string `json:"id"`
	Backend string `json:"backend"`
	Vendor  string `json:"vendor"`
	Device  string `json:"device"`
	MinVRAM int64  `json:"min_vram_bytes"`
	Status  string `json:"status"` // candidate-linux or validated-linux
}

// Manifest pins every external byte used by the managed local runtime.
type Manifest struct {
	Version          int                        `json:"version"`
	ReleaseReady     bool                       `json:"release_ready"`
	LlamaCPPCommit   string                     `json:"llama_cpp_commit"`
	Artifacts        map[string]Artifact        `json:"artifacts"`
	Images           map[string]Image           `json:"images"`
	HardwareProfiles []ValidatedHardwareProfile `json:"hardware_profiles"`
}

// DefaultManifest contains the public, commit-pinned model artifacts and runtime
// images. Backends remain unavailable to automatic selection until their image
// and hardware profile have passed native Linux GPU qualification.
func DefaultManifest() Manifest {
	return Manifest{
		Version:        ReleaseManifestVersion,
		ReleaseReady:   true,
		LlamaCPPCommit: LlamaCPPCommit,
		Artifacts: map[string]Artifact{
			"normal_q4_k_m": {
				Name: "AuraGo-Qwen3.5-4B.Q4_K_M.gguf", Repository: "antibyte/AuraGo-Qwen3.5-4B-GGUF",
				Path: "AuraGo-Qwen3.5-4B.Q4_K_M.gguf", Revision: ggufRepositoryRevision,
				Size: 2783446400, SHA256: "148c82694a60d279fa21316ede2ba5ebcc9f7633da6db7e328640487e154f5f4",
			},
			"normal_q8_0": {
				Name: "AuraGo-Qwen3.5-4B.Q8_0.gguf", Repository: "antibyte/AuraGo-Qwen3.5-4B-GGUF",
				Path: "AuraGo-Qwen3.5-4B.Q8_0.gguf", Revision: ggufRepositoryRevision,
				Size: 4610579840, SHA256: "4e075060b85de48a8a6b242a3c4034c3cd0b7bd16ff6678f6ab6941a1ad11efb",
			},
			"mtp_target_q4_k_m": {
				Name: "aurago-qwen35-4b-target.Q4_K_M.gguf", Repository: "antibyte/AuraGo-Qwen3.5-4B-MTP-v1",
				Path: "gguf/target/aurago-qwen35-4b-target.Q4_K_M.gguf", Revision: mtpRepositoryRevision,
				Size:   2708804000,
				SHA256: "211f9455a99c84a92942e4fba13af164f0707e44008eabce9406cb2cb111daa4",
			},
			"mtp_target_q8_0": {
				Name: "aurago-qwen35-4b-target.Q8_0.gguf", Repository: "antibyte/AuraGo-Qwen3.5-4B-MTP-v1",
				Path: "gguf/target/aurago-qwen35-4b-target.Q8_0.gguf", Revision: mtpRepositoryRevision,
				Size:   4482402720,
				SHA256: "6829ee7ec4fed178993641e129e68aac17c013df37c686090d196d314a5be7e7",
			},
			"mtp_sidecar_q4_k_m": {
				Name: "aurago-qwen35-4b-mtp-v1-sidecar.Q4_K_M.gguf", Repository: "antibyte/AuraGo-Qwen3.5-4B-MTP-v1",
				Path: "gguf/sidecar/aurago-qwen35-4b-mtp-v1-sidecar.Q4_K_M.gguf", Revision: mtpRepositoryRevision,
				Size:   607067040,
				SHA256: "ae7df638ba4136f98a49b7d8814389681f9e11c18ebc609ee2ef9d693a2b1818",
			},
			"mtp_sidecar_q8_0": {
				Name: "aurago-qwen35-4b-mtp-v1-sidecar.Q8_0.gguf", Repository: "antibyte/AuraGo-Qwen3.5-4B-MTP-v1",
				Path: "gguf/sidecar/aurago-qwen35-4b-mtp-v1-sidecar.Q8_0.gguf", Revision: mtpRepositoryRevision,
				Size:   814560160,
				SHA256: "64686ce6f348f5cdd61615b63bad259fafc7daed1de95e07f3a3160b0511b0d4",
			},
		},
		Images: map[string]Image{
			// Candidate images require an explicit backend choice and experimental acknowledgement.
			"cuda": {
				Backend: "cuda",
				Reference: "ghcr.io/antibyte/aurago-llm-cuda:sha-bf2a63b51926@" +
					"sha256:553cf8dd940b8e5a65ba0ca90866fa4e15db1ed2405226efbcb90e5e28cc291b",
			},
			"sycl": {
				Backend: "sycl",
				Reference: "ghcr.io/antibyte/aurago-llm-sycl:sha-bf2a63b51926@" +
					"sha256:ba9541b05b91ff6c7a2f2901b51a601804e68d28ce516715f8cceb8c2108ca58",
			},
			"vulkan": {
				Backend: "vulkan",
				Reference: "ghcr.io/antibyte/aurago-llm-vulkan:sha-bf2a63b51926@" +
					"sha256:fa662d58f3a3c78aae33cc01d7845b713b5fb50fea536647f970cd11de20deb3",
			},
		},
		HardwareProfiles: []ValidatedHardwareProfile{
			{
				ID: "intel-arc-b580", Backend: "sycl", Vendor: "intel", Device: "0xe20b",
				MinVRAM: 8 << 30, Status: "candidate-linux",
			},
		},
	}
}

func (m Manifest) validate() error {
	if m.Version != ReleaseManifestVersion || (m.LlamaCPPCommit != LlamaCPPCommit && m.LlamaCPPCommit != LingEngineCommit) {
		return fmt.Errorf("release_manifest_invalid")
	}
	if !m.ReleaseReady {
		return fmt.Errorf("release_artifacts_unavailable")
	}
	for name, artifact := range m.Artifacts {
		if artifact.Name == "" || artifact.Path == "" || !isHexDigest(artifact.Revision, 40) ||
			artifact.Size <= 0 || !isHexDigest(artifact.SHA256, 64) {
			return fmt.Errorf("release_manifest_invalid: artifact %s", name)
		}
	}
	if len(m.Images) == 0 {
		return fmt.Errorf("release_manifest_invalid: no runtime images")
	}
	for backend, image := range m.Images {
		if !isDigestPinned(image.Reference) {
			return fmt.Errorf("release_manifest_invalid: backend %s is not digest-pinned", backend)
		}
		if !image.Supported {
			continue
		}
		validated := false
		for _, profile := range m.HardwareProfiles {
			if profile.Backend == backend && profile.Status == "validated-linux" &&
				profile.ID != "" && profile.Vendor != "" && profile.Device != "" && profile.MinVRAM >= 8<<30 {
				validated = true
				break
			}
		}
		if !validated {
			return fmt.Errorf("release_manifest_invalid: backend %s has no validated Linux hardware profile", backend)
		}
	}
	return nil
}

func (m Manifest) profileFor(hardware HardwareProfile) string {
	gpu, err := hardware.selectedGPU()
	if err != nil {
		if hardware.SelectedBackend == "cpu" {
			return "experimental-cpu"
		}
		return ""
	}
	for _, profile := range m.HardwareProfiles {
		if profile.Backend == hardware.SelectedBackend &&
			profile.Vendor == gpu.Vendor &&
			strings.EqualFold(strings.TrimPrefix(profile.Device, "0x"), strings.TrimPrefix(gpu.Device, "0x")) {
			return profile.ID + ":" + profile.Status
		}
	}
	return "unvalidated-hardware"
}

func isDigestPinned(reference string) bool {
	const marker = "@sha256:"
	pos := len(reference) - 64 - len(marker)
	return pos > 0 && reference[pos:pos+len(marker)] == marker &&
		isHexDigest(reference[pos+len(marker):], 64)
}

func isHexDigest(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
