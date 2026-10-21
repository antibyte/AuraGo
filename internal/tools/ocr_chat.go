package tools

import (
	"aurago/internal/config"
	"aurago/internal/ocr"
	"aurago/internal/security"
	"bytes"
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

type ocrChatKey struct{}
type ocrChatContext struct {
	session  string
	deadline time.Time
}

// WithOCRChatSession shares one fifteen-second OCR budget across this turn.
func WithOCRChatSession(ctx context.Context, session string) context.Context {
	if binding, ok := ctx.Value(ocrChatKey{}).(ocrChatContext); ok && binding.session == session {
		return ctx
	}
	return context.WithValue(ctx, ocrChatKey{}, ocrChatContext{session: session, deadline: time.Now().Add(15 * time.Second)})
}
func OCRChatPath(ctx context.Context, cfg *config.Config, path string) string {
	binding, _ := ctx.Value(ocrChatKey{}).(ocrChatContext)
	if !ocrChatAllowed(cfg, binding) || !ocr.Supported(path) {
		return ""
	}
	file, err := stageOCRFile(ctx, cfg, "agent:"+binding.session, path)
	if err != nil {
		return ""
	}
	return ocrChatResult(ctx, binding, file)
}
func OCRChatBytes(ctx context.Context, cfg *config.Config, name string, data []byte) string {
	binding, _ := ctx.Value(ocrChatKey{}).(ocrChatContext)
	if !ocrChatAllowed(cfg, binding) || !ocr.Supported(name) {
		return ""
	}
	manager := OCRManager()
	if manager == nil {
		return ""
	}
	file, err := manager.Stage("agent:"+binding.session, name, bytes.NewReader(data))
	if err != nil {
		return ""
	}
	return ocrChatResult(ctx, binding, file)
}
func ocrChatAllowed(cfg *config.Config, binding ocrChatContext) bool {
	manager := OCRManager()
	return cfg != nil && cfg.OCR.Enabled && cfg.OCR.AutomaticChat && binding.session != "" && manager != nil && manager.Status().Ready
}
func ocrChatResult(ctx context.Context, binding ocrChatContext, file ocr.File) string {
	manager := OCRManager()
	if manager == nil {
		return ""
	}
	owner := "agent:" + binding.session
	job, err := manager.SubmitAutomatic(owner, file.ID)
	if err != nil {
		return ""
	}
	waitCtx, cancel := context.WithDeadline(ctx, binding.deadline)
	defer cancel()
	text, err := WaitOCRText(waitCtx, manager, owner, job.ID)
	if err != nil {
		return "\n[Local OCR job_id: " + job.ID + ". Use ocr status/result for the authorized attachment; extraction is pending or incomplete.]\n"
	}
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if len(text) > 16<<10 {
		text = text[:16<<10]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
		text += "\n[OCR excerpt shortened; use ocr result with job_id " + job.ID + " for the full result.]"
	}
	return "\n[Local OCR job_id: " + job.ID + "]\n" + security.IsolateExternalData(text) + "\n"
}
