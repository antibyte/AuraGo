package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
)

// ExecuteOpenSCADRender renders OpenSCAD source through the virtual desktop
// OpenSCAD service and returns a compact JSON tool result plus a desktop event.
func ExecuteOpenSCADRender(ctx context.Context, cfg *config.Config, args map[string]interface{}) VirtualDesktopExecution {
	if cfg == nil {
		return virtualDesktopJSON("error", "configuration is unavailable", nil, nil)
	}
	if !cfg.VirtualDesktop.Enabled {
		return virtualDesktopJSON("error", "virtual desktop is disabled in config", nil, nil)
	}
	if !cfg.VirtualDesktop.AllowAgentControl {
		return virtualDesktopJSON("error", "agent control for the virtual desktop is disabled in config", nil, nil)
	}
	if !cfg.VirtualDesktop.OpenSCAD.Enabled {
		return virtualDesktopJSON("error", "openscad is disabled in config", nil, nil)
	}
	var req desktop.OpenSCADRenderRequest
	if err := mapToStruct(args, &req); err != nil {
		return virtualDesktopJSON("error", fmt.Sprintf("invalid openscad_render arguments: %v", err), nil, nil)
	}
	svc, cleanup, err := getToolDesktopService(ctx, cfg)
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	defer cleanup()
	result, err := svc.OpenSCADContainer().Render(ctx, req)
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), result, nil)
	}
	if req.SaveToDesktop {
		result, err = svc.OpenSCADContainer().SaveJob(ctx, svc, result.JobID)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), result, nil)
		}
	}
	payload := map[string]interface{}{
		"job_id":        result.JobID,
		"model_name":    result.ModelName,
		"files":         result.Files,
		"source_path":   result.SourcePath,
		"source_scad":   req.SourceSCAD,
		"exit_code":     result.ExitCode,
		"duration_ms":   result.DurationMS,
		"stdout":        result.Stdout,
		"stderr":        result.Stderr,
		"saved_paths":   result.SavedPaths,
		"download_base": result.DownloadBase,
		"created_at":    result.CreatedAt,
	}
	if windowID := strings.TrimSpace(req.WindowID); windowID != "" {
		payload["window_id"] = windowID
	}
	event := &desktop.Event{
		Type:      "openscad_result",
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
	return virtualDesktopJSON("ok", "openscad render complete", result, event)
}
