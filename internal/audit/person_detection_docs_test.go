package audit

import (
	"os"
	"strings"
	"testing"

	"aurago/internal/onnxrt"
	"aurago/internal/persondetect"
)

// TestPersonDetectionContractsAreRouted keeps the onnxrt and persondetect
// package contracts in the root's combined Child DOX Index / routing table. Child
// AGENTS.md files are git-ignored by pattern and must be added with git add -f.
func TestPersonDetectionContractsAreRouted(t *testing.T) {
	t.Parallel()
	root := readRepoFile(t, "AGENTS.md")
	for _, contract := range []string{"internal/onnxrt/AGENTS.md", "internal/persondetect/AGENTS.md"} {
		if !strings.Contains(root, "`"+contract+"`") {
			t.Errorf("root AGENTS.md must route %s in the Child DOX Index", contract)
		}
		requireAll(t, contract, readRepoFile(t, contract), []string{
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

// huggingFacePin splits a pinned model URL
// https://huggingface.co/<owner>/<repository>/resolve/<revision>/<file> into
// "<owner>/<repository>" and the revision.
func huggingFacePin(t *testing.T, rawURL string) (repository, revision string) {
	t.Helper()
	rest, ok := strings.CutPrefix(rawURL, "https://huggingface.co/")
	repository, tail, found := strings.Cut(rest, "/resolve/")
	revision, _, _ = strings.Cut(tail, "/")
	if !ok || !found || repository == "" || len(revision) < 12 {
		t.Fatalf("unexpected pinned model URL %q", rawURL)
	}
	return repository, revision
}

// TestPersonDetectionThirdPartyRecords keeps the attribution of the downloaded
// person-detection models, the WebGPU runtime (with the DirectX Shader Compiler
// libraries of its Windows wheel) and the embedded self-test photo in the
// third-party records. Versions, repositories, revisions and wheel members come
// from the code that pins them, so a pin change fails here until both records
// follow.
func TestPersonDetectionThirdPartyRecords(t *testing.T) {
	t.Parallel()
	notices := readRepoFile(t, "THIRD_PARTY_NOTICES.md")
	libraries := readRepoFile(t, "THIRD_PARTY_LIBRARIES.md")
	records := map[string]string{"THIRD_PARTY_NOTICES.md": notices, "THIRD_PARTY_LIBRARIES.md": libraries}

	requireAll(t, "THIRD_PARTY_NOTICES.md", notices, []string{
		"| DirectX Shader Compiler", "| \"Pedestrians using crosswalk (Unsplash)\"",
		"https://pypi.org/project/onnxruntime-webgpu/", "https://github.com/microsoft/DirectXShaderCompiler",
		"https://commons.wikimedia.org/wiki/File:Pedestrians_using_crosswalk_(Unsplash).jpg",
		"## Person detection models and runtime", "declares no license", "`dxcompiler.dll`", "`dxil.dll`",
		"University of Illinois/NCSA", "Peter Miranda", "CC0 1.0",
		"model.webgpu-" + onnxrt.WebGPURuntimeVersion + ".onnx",
	})
	requireAll(t, "THIRD_PARTY_LIBRARIES.md", libraries, []string{
		"DirectX Shader Compiler", "### Embedded third-party media", "internal/persondetect/selftest_reference.jpg", "Peter Miranda",
	})

	for name, doc := range records {
		requireAll(t, name, doc, []string{
			"ONNX Runtime " + onnxrt.CPURuntimeVersion,
			"ONNX Runtime WebGPU " + onnxrt.WebGPURuntimeVersion,
		})
	}
	for _, tier := range []persondetect.ModelTier{persondetect.ModelStandard, persondetect.ModelAccurate} {
		spec, ok := persondetect.Model(tier)
		if !ok {
			t.Fatalf("no pinned model for tier %s", tier)
		}
		repository, revision := huggingFacePin(t, spec.Asset.URL)
		for name, doc := range records {
			// The tables carry the first 12 hex digits of the revision.
			requireAll(t, name, doc, []string{spec.Name, repository, revision[:12]})
		}
		requireAll(t, "THIRD_PARTY_NOTICES.md", notices, []string{"https://huggingface.co/" + repository, revision})
	}
	for _, platform := range [][2]string{{"linux", "amd64"}, {"windows", "amd64"}, {"darwin", "arm64"}} {
		asset, ok := onnxrt.WebGPURuntimeAsset(platform[0], platform[1])
		if !ok {
			t.Fatalf("no pinned WebGPU runtime for %s/%s", platform[0], platform[1])
		}
		for _, member := range asset.Members {
			requireAll(t, "THIRD_PARTY_NOTICES.md", notices, []string{"`" + member[strings.LastIndex(member, "/")+1:] + "`"})
		}
	}
	if _, err := os.Stat(repoPath("internal", "persondetect", "selftest_reference.jpg")); err != nil {
		t.Errorf("the third-party records name the embedded self-test photo, which must exist: %v", err)
	}
}
