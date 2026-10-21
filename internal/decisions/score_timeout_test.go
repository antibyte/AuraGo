package decisions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"testing/synctest"
	"time"
)

type contextScoreBody struct{ ctx context.Context }

func (body contextScoreBody) Read([]byte) (int, error) { <-body.ctx.Done(); return 0, body.ctx.Err() }
func (body contextScoreBody) Close() error             { return nil }

func TestScoreTimeoutAndCancellationKeepGenerationReusable(t *testing.T) {
	for _, phase := range []string{"headers", "body", "trailing body", "caller deadline", "caller cause"} {
		t.Run(phase, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fixture := validTestSession()
				manager := readyManagerForTest(t, fixture)
				metadata := fixture.metadataValue
				session := &nativeRuntimeSession{baseURL: "http://127.0.0.1:18765", cached: metadata, device: manager.activeDevice, deployment: "native"}
				calls, cancellations := 0, 0
				session.client = &http.Client{Timeout: 2 * time.Minute, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					response := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: request}
					var payload any = metadata
					switch request.URL.Path {
					case "/cancel":
						cancellations++
						payload = map[string]bool{"cancelled": true}
					case "/score":
						calls++
						if calls == 1 && phase != "trailing body" {
							if phase == "headers" {
								<-request.Context().Done()
								return nil, request.Context().Err()
							}
							response.Body = contextScoreBody{request.Context()}
							return response, nil
						}
						var wire runtimeScoreWireRequest
						if err := json.NewDecoder(request.Body).Decode(&wire); err != nil {
							return nil, err
						}
						value := runtimeScoreWireResponse{RequestID: wire.RequestID, Protocol: metadata.Protocol, Ready: metadata.Ready, Model: metadata.Model, Projector: metadata.Projector, Build: metadata.Build, Hardware: metadata.Hardware, Placement: metadata.Placement, ModelLayers: metadata.ModelLayers, Limits: metadata.Limits, ImageMarker: metadata.ImageMarker, Usage: runtimeUsage{PromptTokens: 1, ContextTokens: 1}}
						for i, candidate := range wire.Candidates {
							value.Scores = append(value.Scores, scoredCandidate{Code: candidate.Code, TokenID: i + 1, Logit: float64(i)})
						}
						payload = value
						if calls == 1 && phase == "trailing body" {
							raw, _ := json.Marshal(value)
							response.Body = io.NopCloser(io.MultiReader(bytes.NewReader(raw), contextScoreBody{request.Context()}))
							return response, nil
						}
					}
					raw, err := json.Marshal(payload)
					if err != nil {
						return nil, err
					}
					response.Body = io.NopCloser(bytes.NewReader(raw))
					return response, nil
				})}
				manager.session = session
				generation := manager.CacheGeneration()
				ctx := context.Background()
				wantCode, wantCause := CodeRuntimeScoreTimeout, error(context.DeadlineExceeded)
				if phase == "caller deadline" {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, time.Second)
					defer cancel()
					wantCode = CodeCancelled
				}
				if phase == "caller cause" {
					var cancel context.CancelCauseFunc
					ctx, cancel = context.WithCancelCause(ctx)
					wantCode, wantCause = CodeCancelled, errors.New("caller stopped score")
					go func() { time.Sleep(time.Second); cancel(wantCause) }()
				}
				_, err := manager.Evaluate(ctx, testDecisionRequest())
				if ErrorCode(err) != wantCode || !errors.Is(err, wantCause) {
					t.Fatalf("score: %v (cause %v)", err, errors.Unwrap(err))
				}
				synctest.Wait()
				if cancellations != 1 || calls != 1 || !manager.Status().Ready || manager.CacheGeneration() != generation {
					t.Fatalf("timeout retired/retried generation or missed cancellation: scores=%d cancel=%d status=%+v", calls, cancellations, manager.Status())
				}
				if _, err := manager.Evaluate(context.Background(), testDecisionRequest()); err != nil {
					t.Fatalf("generation no longer reusable: %v", err)
				}
				if calls != 2 || manager.CacheGeneration() != generation {
					t.Fatal("successful subsequent score switched generation")
				}
			})
		})
	}
}
