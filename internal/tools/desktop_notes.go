package tools

import (
	"context"
	"fmt"
	"math"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
)

// ExecuteDesktopNotes exposes only read/search/create; the service owns the mutation policy.
func ExecuteDesktopNotes(ctx context.Context, cfg *config.Config, args map[string]interface{}) VirtualDesktopExecution {
	if cfg == nil || !cfg.VirtualDesktop.Enabled || !cfg.VirtualDesktop.AllowAgentControl || !cfg.Tools.VirtualDesktop.Enabled {
		return virtualDesktopJSON("error", "agent access to desktop notes is disabled", nil, nil)
	}
	svc, cleanup, err := getToolDesktopService(ctx, cfg)
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	defer cleanup()
	var result interface{}
	var event *desktop.Event
	operation := virtualDesktopString(args, "operation")
	offset, limit := 0, 50
	for key, target := range map[string]*int{"offset": &offset, "limit": &limit} {
		if value, present := args[key]; present {
			n, ok := value.(float64)
			maxValue := float64(2 * desktop.MaxNoteBytes)
			if key == "limit" {
				maxValue = 200
			}
			if !ok || math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) || n < 0 || n > maxValue || (key == "limit" && n == 0) {
				return virtualDesktopJSON("error", "invalid notes pagination: integer offset >= 0 and limit 1..200 required", nil, nil)
			}
			*target = int(n)
		}
	}
	switch operation {
	case "list", "search":
		result, err = svc.SearchNotes(ctx, desktop.NotesQuery{Query: virtualDesktopString(args, "query"), Folder: virtualDesktopString(args, "folder"), Tag: virtualDesktopString(args, "tag"), Limit: limit, Offset: offset})
	case "read":
		var note desktop.Note
		note, err = svc.ReadNote(ctx, virtualDesktopString(args, "path"))
		if err == nil {
			chars := []rune(note.Content)
			if offset < 0 || offset > len(chars) {
				err = fmt.Errorf("invalid note character offset")
				break
			}
			end := min(offset+16384, len(chars))
			note.Content = string(chars[offset:end])
			result = map[string]interface{}{"note": note, "offset": offset, "next_offset": end, "total_chars": len(chars), "has_more": end < len(chars)}
		}
	case "create":
		var note desktop.Note
		note, err = svc.CreateNote(ctx, virtualDesktopString(args, "title"), virtualDesktopString(args, "content"), virtualDesktopString(args, "folder"), desktop.SourceAgent)
		if err == nil {
			note.Content = ""
			result = note
			event = &desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "write_file", "path": note.Path}, CreatedAt: time.Now().UTC()}
		}
	default:
		err = fmt.Errorf("desktop_notes supports only list, search, read and create; existing notes cannot be modified")
	}
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	return virtualDesktopJSON("ok", "", result, event)
}
