package audit

import (
	"os"
	"strings"
	"testing"
)

// TestPersonDetectionContractsAreRouted keeps the onnxrt and persondetect
// package contracts in the root routing table and Child DOX Index. Child
// AGENTS.md files are git-ignored by pattern and must be added with git add -f.
func TestPersonDetectionContractsAreRouted(t *testing.T) {
	t.Parallel()
	root := readRepoFile(t, "AGENTS.md")
	for _, path := range []string{"internal/onnxrt/AGENTS.md", "internal/persondetect/AGENTS.md"} {
		if got := strings.Count(root, "`"+path+"`"); got < 2 {
			t.Errorf("root AGENTS.md must route %s in the contract table and the Child DOX Index, found %d references", path, got)
		}
		requireAll(t, path, readRepoFile(t, path), []string{
			"## Purpose", "## Ownership", "## Local Contracts", "## Work Guidance", "## Verification", "## Child DOX Index",
		})
	}
	requireAll(t, "internal/persondetect/AGENTS.md", readRepoFile(t, "internal/persondetect/AGENTS.md"), []string{
		"WorkerFlag", "SameConfidentDetections", "PatchVectorMatMuls", "ErrDetectorFailed", "-timeout 30m", "--group-add",
	})
	requireAll(t, "internal/onnxrt/AGENTS.md", readRepoFile(t, "internal/onnxrt/AGENTS.md"), []string{
		"ConfigureWorkerCommand", "AccessibleRenderNode", ".complete.json", "ValidateAssetSpec", "-timeout 30m",
	})
}

// TestPersonDetectionThirdPartyRecords keeps the attribution of the downloaded
// person-detection models, the WebGPU runtime (with the DirectX Shader Compiler
// libraries of its Windows wheel) and the embedded self-test photo in the
// third-party records.
func TestPersonDetectionThirdPartyRecords(t *testing.T) {
	t.Parallel()
	notices := readRepoFile(t, "THIRD_PARTY_NOTICES.md")
	requireAll(t, "THIRD_PARTY_NOTICES.md", notices, []string{
		"| ONNX Runtime WebGPU 1.27.0", "| DirectX Shader Compiler", "| D-FINE-N", "| D-FINE-S", "| \"Pedestrians using crosswalk (Unsplash)\"",
		"https://huggingface.co/AnnotateIt/dfine-nano-coco-onnx", "https://huggingface.co/onnx-community/dfine_s_coco-ONNX",
		"https://pypi.org/project/onnxruntime-webgpu/", "https://github.com/microsoft/DirectXShaderCompiler",
		"https://commons.wikimedia.org/wiki/File:Pedestrians_using_crosswalk_(Unsplash).jpg",
		"## Person detection models and runtime", "571d3971bfb47dc5ddd3746134dd464954bc9df0", "a3cf03147a9b86c78475139115c8ac142577352d",
		"declares no license", "`dxcompiler.dll`", "`dxil.dll`", "University of Illinois/NCSA", "Peter Miranda", "CC0 1.0",
	})
	libraries := readRepoFile(t, "THIRD_PARTY_LIBRARIES.md")
	requireAll(t, "THIRD_PARTY_LIBRARIES.md", libraries, []string{
		"ONNX Runtime WebGPU 1.27.0", "DirectX Shader Compiler", "D-FINE-N ONNX", "D-FINE-S ONNX",
		"### Embedded third-party media", "internal/persondetect/selftest_reference.jpg", "Peter Miranda",
	})
	if _, err := os.Stat(repoPath("internal", "persondetect", "selftest_reference.jpg")); err != nil {
		t.Errorf("the third-party records name the embedded self-test photo, which must exist: %v", err)
	}
}
