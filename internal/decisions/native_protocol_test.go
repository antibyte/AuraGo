package decisions

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestNativeProtocolMetadataAndScoreSupportIndependentDeviceAttestations(t *testing.T) {
	integrated := true
	selected := runtimeSelectedDevice{ID: "vk-igpu", Name: "Integrated Vulkan", Backend: "vulkan", Type: "GPU", Integrated: &integrated}
	metadata := baseRuntimeMetadata(selected, "vulkan")
	metadata.Build.BinarySHA256 = strings.Repeat("e", 64)
	metadata.Hardware.Offload.ModelLayersOnDevice = true
	metadata.Hardware.Offload.ProjectorOnDevice = true
	metadata.Hardware.Offload.ProjectorComputeOnDevice = true
	metadata.Hardware.Offload.KVOnDevice = true
	metadata.ImageMarker = "<|image|>"
	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	token := t.TempDir() + "/runtime-token"
	if err := os.WriteFile(token, []byte("private-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scoreImageMarker := metadata.ImageMarker
	device := runtimeDevice{ID: selected.ID, Name: selected.Name, Backend: selected.Backend, Type: selected.Type, Integrated: &integrated, BinarySHA: metadata.Build.BinarySHA256}
	session := &nativeRuntimeSession{
		baseURL: "http://127.0.0.1:18765", token: token, device: device,
		deployment: config.DecisionsDeploymentNative,
		client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Header.Get("Authorization") != "Bearer private-token" {
				t.Errorf("authorization header = %q", request.Header.Get("Authorization"))
			}
			var payload []byte
			switch {
			case request.Method == http.MethodGet && (request.URL.Path == "/capabilities" || request.URL.Path == "/healthz"):
				payload = append([]byte(nil), encodedMetadata...)
			case request.Method == http.MethodPost && request.URL.Path == "/score":
				var scoreRequest runtimeScoreWireRequest
				if err := json.NewDecoder(request.Body).Decode(&scoreRequest); err != nil {
					return nil, err
				}
				if scoreRequest.RequestID == "" || scoreRequest.MaxInputTokens != maxEvaluationTokens {
					t.Errorf("score request identity/budget = %q/%d", scoreRequest.RequestID, scoreRequest.MaxInputTokens)
				}
				scoreResponse := runtimeScoreWireResponse{
					RequestID: scoreRequest.RequestID, Protocol: runtimeProtocol, Ready: true,
					Model: metadata.Model, Projector: metadata.Projector, Build: metadata.Build,
					Hardware: metadata.Hardware, Placement: metadata.Placement, ModelLayers: metadata.ModelLayers,
					Limits: metadata.Limits, ImageMarker: scoreImageMarker, Usage: runtimeUsage{PromptTokens: 8, ContextTokens: 8},
					Scores: []scoredCandidate{{Code: "option-a", TokenID: 101, Logit: 1.25}, {Code: "option-b", TokenID: 102, Logit: -0.5}},
				}
				payload, err = json.Marshal(scoreResponse)
				if err != nil {
					return nil, err
				}
			default:
				return nil, io.EOF
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(payload)), Request: request}, nil
		})},
	}
	capabilities, err := session.metadata(context.Background())
	if err != nil {
		t.Fatalf("independently decoded capabilities and health were treated as different: %v", err)
	}
	session.cached = capabilities
	score, err := session.score(context.Background(), runtimeScoreRequest{
		Prompt:         "Task: route\nState: status\nOptions: [A: first, B: second]\n### Decision:\n",
		Candidates:     []runtimeCandidate{{Code: "option-a", TokenText: "A"}, {Code: "option-b", TokenText: "B"}},
		MaxInputTokens: maxEvaluationTokens,
	})
	if err != nil {
		t.Fatalf("native score protocol rejected valid response: %v", err)
	}
	if score.usage.ContextTokens != 8 || score.Metadata.Deployment != config.DecisionsDeploymentNative || len(score.Scores) != 2 {
		t.Fatalf("native score result = %#v", score)
	}
	scoreImageMarker = "<|unexpected|>"
	if _, err := session.score(context.Background(), runtimeScoreRequest{
		Prompt:     "Task: route\nState: status\nOptions: [A: first, B: second]\n### Decision:\n",
		Candidates: []runtimeCandidate{{Code: "option-a", TokenText: "A"}, {Code: "option-b", TokenText: "B"}}, MaxInputTokens: maxEvaluationTokens,
	}); ErrorCode(err) != CodeRuntimeAttestationFailed {
		t.Fatalf("score response with a changed image marker error = %v", err)
	}
}
