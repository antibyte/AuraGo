package server

import (
	"net/http"
	"strconv"

	"aurago/internal/cydcreator"
)

func (s *Server) cydIcons(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if _, err := cydcreator.LoadIconLibrary(uiFiles); err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, cydcreator.Icons())
}

func (s *Server) cydImportIcon(w http.ResponseWriter, r *http.Request, params map[string]string) {
	raw, ok := cydIfMatch(r)
	version, err := strconv.ParseInt(raw, 10, 64)
	if !ok || err != nil || version < 1 {
		cydWriteError(w, http.StatusBadRequest, cydcreator.CodeBadRequest, "If-Match with the layout version is required", nil)
		return
	}
	var request cydcreator.IconRequest
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &request) {
		return
	}
	library, err := cydcreator.LoadIconLibrary(uiFiles)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	result, err := s.CYDCreator.ImportIcon(r.Context(), params["id"], version, request, library)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, result)
}
