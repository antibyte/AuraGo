package server

import (
	"errors"
	"net/http"

	"aurago/internal/config"
	"aurago/internal/infra"
)

type desktopInfraIgnoreRequest struct {
	Sources []infra.SourceIdentity `json:"sources"`
	Ignored *bool                  `json:"ignored"`
}

// desktopInfraIgnoreSources hides sources from discoveries (ignored: true) or
// restores them (ignored: false). The flag is required so an incomplete body
// never restores by accident.
func desktopInfraIgnoreSources(w http.ResponseWriter, r *http.Request, cfg *config.Config, store *infra.Store) {
	var request desktopInfraIgnoreRequest
	if !desktopInfraDecode(w, r, &request) {
		return
	}
	if request.Ignored == nil || len(request.Sources) == 0 || len(request.Sources) > 1000 {
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "One to 1000 sources and the ignored flag are required")
		return
	}
	sources, err := store.SetSourcesIgnored(r.Context(), request.Sources, *request.Ignored)
	if err != nil {
		if errors.Is(err, infra.ErrSourceLinked) {
			desktopInfraError(w, http.StatusConflict, "source_linked", "A source linked to an asset cannot be ignored")
			return
		}
		desktopInfraStoreError(w, err)
		return
	}
	desktopInfraJSON(w, http.StatusOK, map[string]any{"sources": desktopInfraProjectSourceStates(cfg, sources)})
}
