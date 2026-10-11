package server

import (
	"net/http"

	"aurago/internal/cydcreator"
)

// cydJobBodyLimit fits a prompt (8000 characters) plus 16 KiB of attachments.
const cydJobBodyLimit = 128 << 10

// cydCreatorJobRoutes are the plan 3 routes: agent jobs, chat messages and
// revisions (overview "HTTP API"). Job cancel is a Stop route: allowed while
// the desktop is read-only, desktop:admin for bearer tokens.
func cydCreatorJobRoutes(s *Server) []cydRoute {
	return []cydRoute{
		{Method: http.MethodPost, Pattern: "projects/{id}/jobs", Write: true, Handler: s.cydStartJob},
		{Method: http.MethodGet, Pattern: "projects/{id}/jobs", Handler: s.cydListJobs},
		{Method: http.MethodGet, Pattern: "jobs/{id}", Handler: s.cydGetJob},
		{Method: http.MethodPost, Pattern: "jobs/{id}/confirm", Write: true, Handler: s.cydConfirmJob},
		{Method: http.MethodPost, Pattern: "jobs/{id}/cancel", Stop: true, Handler: s.cydCancelJob},
		{Method: http.MethodPost, Pattern: "projects/{id}/draft/discard", Write: true, Handler: s.cydDiscardDraft},
		{Method: http.MethodGet, Pattern: "projects/{id}/messages", Handler: s.cydListMessages},
		{Method: http.MethodGet, Pattern: "projects/{id}/revisions", Handler: s.cydListRevisions},
		{Method: http.MethodGet, Pattern: "revisions/{id}/diff", Handler: s.cydRevisionDiff},
		{Method: http.MethodPost, Pattern: "revisions/{id}/restore", Write: true, Handler: s.cydRestoreRevision},
	}
}

func (s *Server) cydStartJob(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var in cydcreator.StartJobInput
	if !cydDecodeJSON(w, r, cydJobBodyLimit, &in) {
		return
	}
	job, err := s.CYDCreator.StartJob(r.Context(), params["id"], in)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

func (s *Server) cydListJobs(w http.ResponseWriter, r *http.Request, params map[string]string) {
	jobs, err := s.CYDCreator.ListJobs(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (s *Server) cydGetJob(w http.ResponseWriter, r *http.Request, params map[string]string) {
	job, err := s.CYDCreator.GetJob(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) cydConfirmJob(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var in struct {
		Prompt string `json:"prompt"`
	}
	if !cydDecodeJSON(w, r, cydJobBodyLimit, &in) {
		return
	}
	job, err := s.CYDCreator.ConfirmJob(r.Context(), params["id"], in.Prompt)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) cydCancelJob(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var body struct{}
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &body) {
		return
	}
	if err := s.CYDCreator.CancelJob(r.Context(), params["id"]); err != nil {
		s.cydError(w, r, err)
		return
	}
	job, err := s.CYDCreator.GetJob(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

func (s *Server) cydDiscardDraft(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var body struct{}
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &body) {
		return
	}
	if err := s.CYDCreator.DiscardDraft(r.Context(), params["id"]); err != nil {
		s.cydError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) cydListMessages(w http.ResponseWriter, r *http.Request, params map[string]string) {
	messages, err := s.CYDCreator.ListMessages(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

func (s *Server) cydListRevisions(w http.ResponseWriter, r *http.Request, params map[string]string) {
	revisions, err := s.CYDCreator.ListRevisions(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"revisions": revisions})
}

func (s *Server) cydRevisionDiff(w http.ResponseWriter, r *http.Request, params map[string]string) {
	diff, err := s.CYDCreator.RevisionDiff(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, diff)
}

func (s *Server) cydRestoreRevision(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var body struct{}
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &body) {
		return
	}
	version, revision, err := s.CYDCreator.RestoreRevision(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"version": version, "revision": revision})
}
