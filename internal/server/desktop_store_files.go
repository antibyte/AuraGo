package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"aurago/internal/desktop"
	"aurago/internal/storeapps"
	"aurago/internal/tools"
)

func storeFileLimit(svc *desktop.Service) int64 {
	size := int64(svc.Config().MaxFileSizeMB) << 20
	if size <= 0 {
		size = 50 << 20
	}
	if size > 64<<20 {
		return 64 << 20
	}
	return size
}

func (s *Server) publishStoreFile(ctx context.Context, svc *desktop.Service, app, extension string, data []byte) (any, error) {
	cfg := s.ConfigSnapshot()
	if perms, ok := tools.EffectiveRuntimePermissions(ctx); ok && !perms.AllowFilesystemWrite {
		return nil, errors.New("File publication is disabled for this run")
	}
	if cfg.VirtualDesktop.ReadOnly || !cfg.Agent.AllowFilesystemWrite {
		return nil, errors.New("File publication permission is disabled")
	}
	destination := "Shared/Store/" + app + "/" + rand.Text() + "/result" + extension
	entry, err := svc.WriteFileStreamConditional(ctx, destination, bytes.NewReader(data), storeFileLimit(svc), "agent", nil)
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": entry.Path, "bytes": entry.Size}, nil
}

func (s *Server) readStoreFile(ctx context.Context, client storeapps.Connection, name string) (any, error) {
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		return nil, err
	}
	data, filename, err := client.FileBrowserRead(ctx, name, storeFileLimit(svc))
	if err != nil {
		return nil, err
	}
	if len(data) <= 64<<10 && utf8.Valid(data) && !bytes.ContainsRune(data, 0) {
		return map[string]any{"path": name, "content": string(data), "bytes": len(data)}, nil
	}
	extension := strings.ToLower(path.Ext(filename))
	// Publish imported bytes as inert downloads, never as an executable web page.
	switch extension {
	case ".pdf", ".png", ".jpg", ".jpeg", ".webp", ".gif", ".txt", ".csv", ".json", ".zip":
	default:
		extension = ".bin"
	}
	return s.publishStoreFile(ctx, svc, "filebrowser", extension, data)
}

func (s *Server) processStorePDF(ctx context.Context, client storeapps.Connection, req tools.StoreAppRequest) (any, error) {
	if len(req.Files) < 1 || len(req.Files) > 20 {
		return nil, errors.New("Select 1-20 saved Desktop PDFs")
	}
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		return nil, err
	}
	limit := storeFileLimit(svc)
	inputs := make([]storeapps.PDFInput, 0, len(req.Files))
	var total int64
	for _, name := range req.Files {
		if !filepath.IsLocal(name) || strings.Contains(name, "\\") || path.Clean(name) != name || strings.ToLower(path.Ext(name)) != ".pdf" {
			return nil, errors.New("Use saved PDF paths relative to the Desktop workspace")
		}
		in, entry, _, e := svc.OpenPreviewFile(ctx, name)
		if e != nil {
			return nil, e
		}
		if entry.Size > limit-total {
			in.Close()
			return nil, errors.New("PDF inputs exceed the total file-size limit")
		}
		data, e := io.ReadAll(io.LimitReader(in, limit-total+1))
		closeErr := in.Close()
		if e != nil {
			return nil, e
		}
		if closeErr != nil {
			return nil, closeErr
		}
		total += int64(len(data))
		if total > limit {
			return nil, errors.New("PDF inputs exceed the total file-size limit")
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		inputs = append(inputs, storeapps.PDFInput{Data: data})
	}
	data, extension, err := client.StirlingProcess(ctx, storeapps.PDFRequest{Operation: req.Operation, Languages: req.Languages, Pages: req.Pages}, inputs, limit)
	if err != nil {
		return nil, err
	}
	return s.publishStoreFile(ctx, svc, "stirling-pdf", extension, data)
}
