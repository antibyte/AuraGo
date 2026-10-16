package decisions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

// This opt-in acceptance test runs the real pinned model with CPU consent off.
// Point AURAGO_DECISIONS_LIVE_MODELS at the two verified model files and
// AURAGO_DECISIONS_LIVE_RUNTIME at the application root containing its bundle.
// AURAGO_DECISIONS_LIVE_IGPU=1 additionally requires an integrated accelerator.
func TestDecisionsLiveNative(t *testing.T) {
	models, runtimeRoot := os.Getenv("AURAGO_DECISIONS_LIVE_MODELS"), os.Getenv("AURAGO_DECISIONS_LIVE_RUNTIME")
	if models == "" || runtimeRoot == "" {
		t.Skip("requires explicitly selected local model and native runtime directories")
	}
	dataDir := t.TempDir()
	modelDir := filepath.Join(dataDir, "decisions", "models")
	if err := os.MkdirAll(modelDir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := DefaultManifest()
	for _, artifact := range []Artifact{manifest.Model, manifest.Projector} {
		if err := os.Link(filepath.Join(models, artifact.Filename), filepath.Join(modelDir, artifact.Filename)); err != nil {
			t.Fatalf("hard-link local model fixture on the same volume: %v", err)
		}
	}
	modes := []string{"preferred_accelerator"}
	if os.Getenv("AURAGO_DECISIONS_LIVE_IGPU") == "1" {
		modes = append(modes, "integrated_accelerator")
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: "native", CPUFallbackAllowed: false}, dataDir, WithRuntimeDirectory(runtimeRoot))
			factory := &liveDeviceFactory{runtimeFactory: manager.factory, integratedOnly: mode == "integrated_accelerator"}
			manager.factory = factory
			defer func() {
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer stopCancel()
				if err := manager.Shutdown(stopCtx); err != nil {
					t.Errorf("shutdown: %s", liveErrorChain(err))
				}
			}()
			if err := manager.Install(ctx); err != nil {
				t.Fatalf("install verified local artifacts: %s", liveErrorChain(err))
			}
			if err := manager.Start(ctx); err != nil {
				t.Fatalf("start with CPU consent denied: %s; status=%+v", liveErrorChain(err), manager.Status())
			}
			status := manager.Status()
			if !status.Ready || !status.DeviceAttested || !status.GPUOffloadVerified || status.ExecutionMode != "gpu" || status.CPUFallbackActive || status.CPUFallbackAllowed {
				t.Fatalf("full accelerator attestation required: %+v", status)
			}
			if len(factory.attempts) == 0 {
				t.Fatal("manager did not attempt a discovered device")
			}
			if factory.integratedOnly && (factory.attempts[0].Integrated == nil || !*factory.attempts[0].Integrated) {
				t.Fatal("integrated acceptance did not select an iGPU")
			}
			if !factory.integratedOnly {
				for _, device := range factory.discovered {
					if !strings.EqualFold(device.Type, "CPU") && device.Integrated != nil && !*device.Integrated {
						if factory.attempts[0].Integrated == nil || *factory.attempts[0].Integrated {
							t.Fatal("dedicated accelerator was not attempted first")
						}
						break
					}
				}
			}
			t.Logf("actual device: %s; backend=%s; full offload=%v", status.Device, status.Backend, status.GPUOffloadVerified)
			zero, one := 0.0, 1.0
			request := Request{State: "A red apple is on the table.", Questions: []Question{
				{ID: "object", Type: "choice", Instructions: "Which object is on the table?", Options: []Option{{ID: "apple", Label: "An apple"}, {ID: "car", Label: "A car"}, {ID: "planet", Label: "A planet"}}},
				{ID: "red", Type: "noul", Instructions: "Is the apple red?"},
				{ID: "match", Type: "score", Instructions: "Rate whether the statement describes a red apple.", Options: []Option{{ID: "no", Label: "No match", Value: &zero}, {ID: "yes", Label: "Complete match", Value: &one}}},
			}}
			check := func(request Request) Response {
				t.Helper()
				result, err := manager.Evaluate(ctx, request)
				if err != nil {
					t.Fatalf("evaluate: %s", liveErrorChain(err))
				}
				if result.ModelRevision != modelRevision || result.Calibration != "uncalibrated" || !result.Runtime.GPUOffloadVerified || !result.Runtime.DeviceAttested {
					t.Fatalf("response provenance: %+v", result)
				}
				if len(result.Answers) != len(request.Questions) || result.Usage.ContextTokens < 1 || result.Usage.ContextTokens > maxEvaluationTokens {
					t.Fatalf("response completeness or token accounting: %+v", result)
				}
				for _, question := range request.Questions {
					answer, ok := result.Answers[question.ID]
					options := optionsForQuestion(question)
					if !ok || len(answer.Probabilities) != len(options) {
						t.Fatalf("incomplete distribution for %q: %+v", question.ID, answer)
					}
					sum, largest, expected := 0.0, 0.0, 0.0
					for index, option := range options {
						p, ok := answer.Probabilities[option.ID]
						if !ok || math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
							t.Fatalf("invalid candidate probability: %q=%v", option.ID, p)
						}
						sum += p
						largest = math.Max(largest, p)
						value := float64(index)
						if option.Value != nil {
							value = *option.Value
						}
						expected += p * value
					}
					if math.Abs(sum-1) > 1e-12 || answer.Confidence != largest {
						t.Fatalf("distribution not normalized: %+v", answer)
					}
					if question.Type == "noul" && (answer.Noul == nil || *answer.Noul != answer.Probabilities["yes"]) {
						t.Fatalf("yes probability missing: %+v", answer)
					}
					if question.Type == "score" && (answer.Score == nil || math.Abs(*answer.Score-expected) > 1e-12) {
						t.Fatalf("expected rating mismatch: %+v", answer)
					}
				}
				encoded, _ := json.Marshal(result)
				t.Logf("result: %s", encoded)
				return result
			}
			textResult := check(request)
			if textResult.Answers["object"].Decision != "apple" || textResult.Answers["red"].Decision != "yes" {
				t.Fatal("basic text semantic smoke test failed")
			}
			var encoded bytes.Buffer
			red := image.NewRGBA(image.Rect(0, 0, 224, 224))
			for y := 0; y < 224; y++ {
				for x := 0; x < 224; x++ {
					red.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
				}
			}
			if err := png.Encode(&encoded, red); err != nil {
				t.Fatal(err)
			}
			imageResult := check(Request{State: "Use the attached image.", Images: []Image{{Data: encoded.Bytes(), MIME: "image/png"}}, Questions: []Question{{ID: "color", Type: "choice", Instructions: "What is the dominant color?", Options: []Option{{ID: "red", Label: "Red"}, {ID: "blue", Label: "Blue"}, {ID: "green", Label: "Green"}}}}})
			if imageResult.Usage.ImageTokens == 0 {
				t.Fatal("image inference did not account for image tokens")
			}
			if imageResult.Answers["color"].Decision != "red" {
				t.Fatal("basic image semantic smoke test failed")
			}
			options := make([]Option, 62)
			for index := range options {
				options[index] = Option{ID: fmt.Sprintf("option_%d", index), Label: fmt.Sprintf("The number %d", index)}
			}
			check(Request{State: "The number is zero.", Questions: []Question{{ID: "all_codes", Type: "choice", Instructions: "Choose the number in the state.", Options: options}}})
			_, err := manager.Evaluate(ctx, Request{State: strings.Repeat(" hi", 10000), Questions: request.Questions[:1]})
			if ErrorCode(err) != CodeInputTokenBudgetExceeded {
				t.Fatalf("over-budget input should be rejected: %s", liveErrorChain(err))
			}
			if err := manager.Reconfigure(config.DecisionsConfig{Enabled: false, Deployment: "native"}); err != nil {
				t.Fatal(err)
			}
			if manager.Status().Ready || manager.Status().State != "disabled" {
				t.Fatal("disabling did not stop the real runtime")
			}
		})
	}
}

type liveDeviceFactory struct {
	runtimeFactory
	integratedOnly bool
	discovered     []runtimeDevice
	attempts       []runtimeDevice
}

func (factory *liveDeviceFactory) discover(ctx context.Context, deployment string, access RuntimeAccess, paths modelPaths, dataDir string) ([]runtimeDevice, error) {
	devices, err := factory.runtimeFactory.discover(ctx, deployment, access, paths, dataDir)
	factory.discovered = devices
	if err != nil || !factory.integratedOnly {
		return devices, err
	}
	var integrated []runtimeDevice
	for _, device := range devices {
		if device.Integrated != nil && *device.Integrated && !strings.EqualFold(device.Type, "CPU") {
			integrated = append(integrated, device)
		}
	}
	return integrated, nil
}

func (factory *liveDeviceFactory) start(ctx context.Context, deployment string, device runtimeDevice, paths modelPaths, cfg config.DecisionsConfig, access RuntimeAccess, dataDir string, progress func(string)) (runtimeSession, error) {
	factory.attempts = append(factory.attempts, device)
	return factory.runtimeFactory.start(ctx, deployment, device, paths, cfg, access, dataDir, progress)
}

func liveErrorChain(err error) string {
	var causes []string
	for err != nil {
		causes = append(causes, err.Error())
		err = errors.Unwrap(err)
	}
	return strings.Join(causes, ": ")
}
