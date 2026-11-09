package server

import (
	"net/http"

	"aurago/internal/storymaker"
)

// handleStorymakerReviewSheet serves GET and HEAD /projects/{id}/review/contact-sheet.png: the contact sheet of the agent's
// latest check. It is an inert PNG, served with the same headers as the audio route, and not cached, because the next check
// replaces it.
func handleStorymakerReviewSheet(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeStorymakerMethodError(w, "GET, HEAD")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
	if !ok {
		return
	}
	content, info, err := acc.rt.projects.OpenReviewSheet(r.Context(), id)
	if err != nil {
		mapStorymakerError(w, err, "review_not_found")
		return
	}
	defer content.Close()
	h := w.Header()
	h.Set("Content-Type", "image/png")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, storymaker.ReviewSheetName, info.ModTime, content)
}
