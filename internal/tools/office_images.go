package tools

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/office"
	"aurago/internal/qrcode"
)

func officeInsertImage(ctx context.Context, cfg *config.Config, svc *desktop.Service, args map[string]interface{}, kind string) VirtualDesktopExecution {
	fail := func(err error) VirtualDesktopExecution { return virtualDesktopJSON("error", err.Error(), nil, nil) }
	target := virtualDesktopString(args, "path", "file_path")
	expected := virtualDesktopString(args, "expected_version")
	decoded, err := hex.DecodeString(expected)
	if err != nil || len(decoded) != 32 {
		return fail(fmt.Errorf("expected_version from the document/workbook read result is required"))
	}
	ext := strings.ToLower(path.Ext(target))
	if (kind == "document" && ext != ".docx") || (kind == "workbook" && ext != ".xlsx" && ext != ".xlsm") {
		return fail(fmt.Errorf("insert_image requires DOCX or XLSX/XLSM; text and CSV cannot retain images"))
	}
	var ref qrFileRef
	raw, err := json.Marshal(args["image_ref"])
	if err != nil || json.Unmarshal(raw, &ref) != nil {
		return fail(fmt.Errorf("image_ref must contain root and path"))
	}
	image, err := qrReadReference(ctx, cfg, &ref, qrcode.MaxResourceBytes)
	if err != nil {
		return fail(err)
	}
	resource, err := qrcode.NormalizeImage(ctx, "", image)
	if err != nil {
		return fail(err)
	}
	original, entry, err := svc.ReadFileBytes(ctx, target)
	if err != nil {
		return fail(fmt.Errorf("Office target could not be read"))
	}
	if officeToolVersionForEntry(entry, original).ETag != expected {
		return fail(fmt.Errorf("Office version conflict; read the target again before inserting"))
	}
	width, height := virtualDesktopInt(args, 0, "width"), virtualDesktopInt(args, 0, "height")
	alt := virtualDesktopString(args, "alt")
	if len(alt) > 1024 {
		return fail(fmt.Errorf("image description is too long"))
	}
	var result []byte
	if kind == "document" {
		result, err = office.InsertDOCXImage(original, resource.Data, office.DocumentImageOptions{Anchor: virtualDesktopString(args, "anchor"), Width: width, Height: height, Alt: alt})
	} else {
		result, err = office.InsertWorkbookImage(original, resource.Data, office.WorkbookImageOptions{Sheet: virtualDesktopString(args, "sheet"), Anchor: virtualDesktopString(args, "anchor", "cell"), Width: width, Height: height, Alt: alt})
	}
	if err != nil {
		return fail(err)
	}
	updated, err := svc.WriteFileBytesConditional(ctx, target, result, desktop.SourceAgent, func(state desktop.FileWriteState) error {
		if !state.Exists || officeToolVersionForEntry(state.Entry, state.Data).ETag != expected {
			return fmt.Errorf("Office version conflict; original file retained")
		}
		if err := officeToolAllowed(cfg, kind, "insert_image"); err != nil {
			return err
		}
		if cfg.AuthorizationSnapshots != nil {
			_, current := cfg.AuthorizationSnapshots()
			if err := officeToolAllowed(current, kind, "insert_image"); err != nil {
				return err
			}
			if current.VirtualDesktop.ReadOnly {
				return fmt.Errorf("desktop is read-only")
			}
		}
		return ctx.Err()
	})
	if err != nil {
		return fail(err)
	}
	return virtualDesktopJSON("ok", "Image inserted; original Office parts preserved", map[string]interface{}{"path": updated.Path, "entry": updated, "office_version": officeToolVersionForEntry(updated, result)}, &desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "insert_image", "path": updated.Path}})
}
