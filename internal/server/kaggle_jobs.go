package server

import (
	"database/sql"
	"errors"
	"net/http"

	"aurago/internal/kaggle"
)

// Called only after Desktop authentication; authenticated browser owners are admins.
func (s *Server) kaggleCanResolveUnknown(r *http.Request) bool {
	if token, bearer := bearerCredential(r.Header.Get("Authorization")); bearer {
		manager := s.currentTokenManager()
		if manager == nil {
			return false
		}
		_, allowed := manager.Validate(token, "admin")
		return allowed && desktopTokenHasScope(s, token, desktopScopeWrite)
	}
	return true
}

func (s *Server) resolveKaggleUnknownJob(w http.ResponseWriter, r *http.Request, b *kaggleServices, id string) {
	var request struct {
		ConfirmedInactive bool `json:"confirmed_inactive"`
	}
	if !decodeKaggle(w, r, &request) {
		return
	}
	if !request.ConfirmedInactive {
		kaggleJSON(w, http.StatusBadRequest, map[string]string{"error": "confirm on Kaggle that this job is no longer running before releasing its slot"})
		return
	}
	job, err := b.Platform.Store.ResolveUnknown(r.Context(), id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		kaggleJSON(w, http.StatusNotFound, map[string]string{"error": "unknown Kaggle job"})
	case errors.Is(err, kaggle.ErrJobStateConflict):
		kaggleJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case err != nil:
		kaggleFailure(w, err)
	default:
		kaggleJSON(w, http.StatusOK, job)
	}
}
