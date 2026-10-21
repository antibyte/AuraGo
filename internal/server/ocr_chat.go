package server

import (
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/ocr"
	"aurago/internal/tools"
	"context"
	"strings"
)

func (s *Server) ocrDesktopContext(ctx context.Context, cfg *config.Config, session string, chat desktopChatContext) string {
	if !cfg.OCR.Enabled || !cfg.OCR.AutomaticChat {
		return ""
	}
	ctx = tools.WithOCRChatSession(ctx, session)
	paths := append([]string(nil), chat.OpenFiles...)
	if chat.CurrentFile != "" {
		paths = append(paths, chat.CurrentFile)
	}
	if len(paths) == 0 {
		return ""
	}
	service, _, err := s.getDesktopService(ctx)
	if err != nil {
		return ""
	}
	seen := map[string]bool{}
	var out strings.Builder
	for _, path := range paths {
		if len(seen) >= 20 {
			break
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		if !ocrSupportedPath(path) {
			continue
		}
		data, entry, err := service.ReadFileBytes(ctx, path)
		if err != nil {
			continue
		}
		out.WriteString(tools.OCRChatBytes(ctx, cfg, entry.Name, data))
	}
	return out.String()
}
func (s *Server) ocrAgoDeskAttachments(ctx context.Context, session string, records []memory.AgoDeskAttachmentRecord) string {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.OCR.Enabled || !cfg.OCR.AutomaticChat {
		return ""
	}
	ctx = tools.WithOCRChatSession(ctx, session)
	var out strings.Builder
	for _, record := range records {
		out.WriteString(tools.OCRChatPath(ctx, cfg, agodeskAttachmentAgentPath(record.RelativePath)))
	}
	return out.String()
}
func ocrSupportedPath(path string) bool {
	return ocr.Supported(path)
}

func ocrWebAttachments(ctx context.Context, cfg *config.Config, session, text string) string {
	if cfg == nil || !cfg.OCR.Enabled || !cfg.OCR.AutomaticChat {
		return ""
	}
	ctx = tools.WithOCRChatSession(ctx, session)
	seen := map[string]bool{}
	var out strings.Builder
	for _, match := range attachmentPathRe.FindAllStringSubmatch(text, ocr.MaxFiles) {
		relative := cleanMatchedAttachmentPath(match[1])
		if relative == "" || seen[relative] {
			continue
		}
		seen[relative] = true
		out.WriteString(tools.OCRChatPath(ctx, cfg, "agent_workspace/workdir/attachments/"+relative))
	}
	return out.String()
}
