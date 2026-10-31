package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/infra"
	"aurago/internal/infra/atlaspdf"
	"aurago/internal/webassets"
)

// desktopInfraExportPDF renders the stored Infra graph as a PDF map. It never
// refreshes providers: the export shows what the Infra app shows right now.
func desktopInfraExportPDF(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store) {
	query := r.URL.Query()
	paper, ok := desktopInfraExportPaper(query.Get("paper"))
	if !ok {
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "paper must be a4, a3 or letter")
		return
	}
	opts := atlaspdf.Options{Paper: paper}
	if !desktopInfraExportSections(query.Get("sections"), &opts) {
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "sections must be a comma list of map, hosts and inventory")
		return
	}
	snapshot, err := store.Snapshot(r.Context())
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	desktopInfraDecorateSnapshot(r.Context(), s, cfg, &snapshot)

	lang := cfg.Server.UILanguage
	opts.Text = func(key string) string { return i18n.T(lang, key) }
	opts.Icons = uiFiles
	opts.CJKFonts = webassets.Namespace("newspaper/fonts")
	now := time.Now()
	opts.Now = now
	data, err := atlaspdf.Render(snapshot, opts)
	if err != nil {
		if errors.Is(err, atlaspdf.ErrTooLarge) {
			desktopInfraError(w, http.StatusRequestEntityTooLarge, "export_too_large", "The Infra graph is too large to export as PDF")
			return
		}
		slog.Default().Warn("Infra PDF export failed", "error", err)
		desktopInfraError(w, http.StatusInternalServerError, "export_failed", "The PDF could not be created")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="homelab-map-`+now.Format("2006-01-02")+`.pdf"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func desktopInfraExportPaper(value string) (atlaspdf.Paper, bool) {
	switch p := strings.ToLower(strings.TrimSpace(value)); p {
	case "", "a4":
		return atlaspdf.PaperA4, true
	case "a3":
		return atlaspdf.PaperA3, true
	case "letter":
		return atlaspdf.PaperLetter, true
	}
	return "", false
}

// desktopInfraExportSections parses the sections list into opts; an empty list
// selects every section and an unknown name is rejected.
func desktopInfraExportSections(value string, opts *atlaspdf.Options) bool {
	if strings.TrimSpace(value) == "" {
		opts.Map, opts.Hosts, opts.Inventory = true, true, true
		return true
	}
	for _, part := range strings.Split(value, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "map":
			opts.Map = true
		case "hosts":
			opts.Hosts = true
		case "inventory":
			opts.Inventory = true
		default:
			return false
		}
	}
	return true
}
