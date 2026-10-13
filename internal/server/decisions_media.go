package server

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/flows"
	"aurago/internal/tools"
	_ "golang.org/x/image/webp"
)

var errDecisionImage = errors.New("invalid decision image")

const decisionImageBytes = 12 << 20
const decisionImagesTotalBytes = 24 << 20
const decisionImagePixels = 32 << 20

func (s *Server) evaluateDecisions(ctx context.Context, request decisionsAPIRequest) (decisions.Response, error) {
	if s.Decisions == nil {
		return decisions.Response{}, decisions.NewError(decisions.CodeServiceNotReady, nil)
	}
	if router, ok := s.Decisions.(*routedDecisions); ok {
		var cancel context.CancelFunc
		ctx, cancel = router.requestContext(ctx)
		defer cancel()
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.Decisions.Enabled {
		return decisions.Response{}, &decisions.Error{Code: "service_disabled"}
	}
	if len(request.Images) > 8 {
		return decisions.Response{}, errDecisionImage
	}
	input := decisions.Request{State: request.State, Questions: request.Questions}
	if err := decisions.ValidateRequest(input); err != nil {
		return decisions.Response{}, decisions.NewError(decisions.CodeInvalidRequest, err)
	}
	sourceRemaining, runtimeRemaining := int64(decisionImagesTotalBytes), int64(decisionImagesTotalBytes)
	for _, ref := range request.Images {
		if err := ctx.Err(); err != nil {
			return decisions.Response{}, err
		}
		file, err := s.openDecisionImage(ref, cfg)
		if err != nil {
			return decisions.Response{}, errDecisionImage
		}
		limit := int64(decisionImageBytes)
		if sourceRemaining < limit {
			limit = sourceRemaining
		}
		data, err := io.ReadAll(io.LimitReader(file, limit+1))
		_ = file.Close()
		if err != nil || len(data) > decisionImageBytes || int64(len(data)) > sourceRemaining {
			return decisions.Response{}, errDecisionImage
		}
		sourceRemaining -= int64(len(data))
		mime := http.DetectContentType(data)
		switch mime {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
		default:
			return decisions.Response{}, errDecisionImage
		}
		size, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || size.Width <= 0 || size.Height <= 0 || int64(size.Width)*int64(size.Height) > decisionImagePixels {
			return decisions.Response{}, errDecisionImage
		}
		if mime == "image/webp" || mime == "image/gif" {
			// GIF and WebP are reduced to one PNG frame because the runtime decoder is not the animated path.
			decoded, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				return decisions.Response{}, errDecisionImage
			}
			encoded := decisionImageBuffer{limit: min(decisionImageBytes, int(runtimeRemaining))}
			if err := png.Encode(&encoded, decoded); err != nil {
				return decisions.Response{}, errDecisionImage
			}
			data, mime = encoded.Bytes(), "image/png"
		}
		if int64(len(data)) > runtimeRemaining {
			return decisions.Response{}, errDecisionImage
		}
		runtimeRemaining -= int64(len(data))
		input.Images = append(input.Images, decisions.Image{Data: data, MIME: mime})
	}
	return s.Decisions.Evaluate(ctx, input)
}

type decisionImageBuffer struct {
	bytes.Buffer
	limit int
}

func (b *decisionImageBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errDecisionImage
	}
	return b.Buffer.Write(p)
}

func (s *Server) openDecisionImage(ref string, cfg *config.Config) (*os.File, error) {
	if ref == "" || len(ref) > 4096 || strings.Contains(ref, "://") {
		return nil, errDecisionImage
	}
	var item *tools.MediaItem
	var err error
	switch {
	case strings.HasPrefix(ref, "media:"):
		id, parseErr := strconv.ParseInt(strings.TrimPrefix(ref, "media:"), 10, 64)
		if parseErr != nil || id <= 0 || s.MediaRegistryDB == nil {
			return nil, errDecisionImage
		}
		item, err = tools.GetMedia(s.MediaRegistryDB, id)
	case strings.HasPrefix(ref, "/files/"):
		if s.MediaRegistryDB == nil {
			return nil, errDecisionImage
		}
		item, err = tools.GetMediaByWebPath(s.MediaRegistryDB, ref)
	default:
		// Workspace/document references retain the existing protected-file policy.
		file, _, openErr := tools.OpenOutgoingAttachment(ref, cfg)
		return file, openErr
	}
	if err != nil || item == nil || item.MediaType != "image" {
		return nil, errDecisionImage
	}
	resolved, err := tools.ResolveRegisteredMediaFilePath(item.FilePath, cfg)
	if err != nil {
		return nil, errDecisionImage
	}
	for _, rootPath := range []string{cfg.Directories.WorkspaceDir, cfg.Directories.DataDir} {
		if strings.TrimSpace(rootPath) == "" {
			continue
		}
		rootPath, err = filepath.Abs(rootPath)
		if err != nil {
			continue
		}
		rel, relErr := filepath.Rel(rootPath, resolved)
		if relErr != nil || !filepath.IsLocal(rel) {
			continue
		}
		root, rootErr := os.OpenRoot(rootPath)
		if rootErr != nil {
			continue
		}
		file, openErr := root.Open(rel)
		_ = root.Close()
		if openErr != nil {
			return nil, errDecisionImage
		}
		info, statErr := file.Stat()
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > decisionImageBytes {
			_ = file.Close()
			return nil, errDecisionImage
		}
		return file, nil
	}
	return nil, errDecisionImage
}

type flowDecisions struct{ s *Server }

func (adapter flowDecisions) Evaluate(ctx context.Context, request flows.DecisionRequest) (decisions.Response, error) {
	response, err := adapter.s.evaluateDecisions(ctx, decisionsAPIRequest{State: request.State, Questions: request.Questions, Images: request.Images})
	if err == nil {
		return response, nil
	}
	code := decisionsFailureCode(err)
	if ctx.Err() != nil && code != decisions.CodeExternalOutcomeUnknown && code != "configuration_changed" {
		return decisions.Response{}, ctx.Err()
	}
	if decisions.IsNonRetryable(err) {
		flowCode := "FLOW_AI_UNAVAILABLE"
		if code == "budget_exceeded" {
			flowCode = "FLOW_BUDGET_EXCEEDED"
		}
		if code == decisions.CodeInvalidRequest || code == "invalid_image" || code == decisions.CodeInputTokenBudgetExceeded || code == decisions.CodeCandidateNotScorable {
			flowCode = "FLOW_PARAM_INVALID"
		}
		return decisions.Response{}, flows.NewNodeError(flowCode, "Decisions: %s", code)
	}
	if ctx.Err() != nil {
		return decisions.Response{}, ctx.Err()
	}
	flowCode := "FLOW_AI_UNAVAILABLE"
	if code == decisions.CodeInvalidRequest || code == "invalid_image" || code == decisions.CodeInputTokenBudgetExceeded || code == decisions.CodeCandidateNotScorable {
		flowCode = "FLOW_PARAM_INVALID"
	}
	if code == "request_timeout" {
		flowCode = "FLOW_NODE_TIMEOUT"
	}
	if code == decisions.CodeQueueFull || code == decisions.CodeRuntimeBusy {
		flowCode = "FLOW_NODE_FAILED"
	}
	return decisions.Response{}, flows.NewNodeError(flowCode, "Decisions: %s", code)
}
