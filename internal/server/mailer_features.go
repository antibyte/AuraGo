package server

import (
	"net/http"
	"strconv"

	"aurago/internal/mailer"
)

func (s *Server) mailerFeaturesHTTP(w http.ResponseWriter, r *http.Request, p []string) {
	ctx := r.Context()
	if p[0] == "templates" || p[0] == "signatures" {
		if len(p) == 1 && r.Method == http.MethodGet {
			items, err := s.Mailer.ListTemplates(ctx)
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			filtered := []mailer.Template{}
			for _, item := range items {
				if (p[0] == "signatures") == (item.Kind == "signature") {
					filtered = append(filtered, item)
				}
			}
			mailerJSON(w, 200, map[string]any{"items": filtered})
			return
		}
		if (len(p) == 1 && r.Method == http.MethodPost) || (len(p) == 2 && r.Method == http.MethodPut) {
			var body mailer.Template
			if err := decodeMailerJSON(w, r, &body, 128<<10); err != nil {
				mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
				return
			}
			if len(p) == 2 {
				body.ID = p[1]
			} else if body.ID != "" {
				mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
				return
			}
			if p[0] == "signatures" {
				body.Kind = "signature"
			} else {
				body.Kind = "template"
			}
			item, err := s.Mailer.SaveTemplate(ctx, body)
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, item)
			return
		}
		if len(p) == 2 && r.Method == http.MethodDelete {
			revision, e := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
			if e != nil || revision < 1 {
				mailerJSON(w, 428, map[string]string{"error": "revision_required"})
				return
			}
			if err := s.Mailer.DeleteTemplate(ctx, p[1], revision); err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	if p[0] == "rules" {
		if len(p) == 1 && r.Method == http.MethodGet {
			items, err := s.Mailer.ListRules(ctx)
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, map[string]any{"items": items})
			return
		}
		if len(p) == 2 && p[1] == "preview" && r.Method == http.MethodPost {
			var body struct {
				Rule       mailer.Rule `json:"rule"`
				MessageIDs []string    `json:"message_ids"`
			}
			if err := decodeMailerJSON(w, r, &body, 64<<10); err != nil {
				mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
				return
			}
			items, err := s.Mailer.PreviewRule(ctx, body.Rule, body.MessageIDs)
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			matches := 0
			for _, item := range items {
				if item.Matches {
					matches++
				}
			}
			mailerJSON(w, 200, map[string]any{"items": items, "match_count": matches, "revision": body.Rule.Revision})
			return
		}
		if len(p) == 3 && p[2] == "apply" && r.Method == http.MethodPost {
			var body struct {
				MessageIDs []string `json:"message_ids"`
				Revision   int64    `json:"revision"`
			}
			if err := decodeMailerJSON(w, r, &body, 64<<10); err != nil {
				mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
				return
			}
			if err := s.Mailer.ApplyRule(ctx, p[1], body.Revision, body.MessageIDs); err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		if (len(p) == 1 && r.Method == http.MethodPost) || (len(p) == 2 && r.Method == http.MethodPut) {
			var body mailer.Rule
			if err := decodeMailerJSON(w, r, &body, 64<<10); err != nil {
				mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
				return
			}
			if len(p) == 2 {
				body.ID = p[1]
			} else if body.ID != "" {
				mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
				return
			}
			item, err := s.Mailer.SaveRule(ctx, body)
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, item)
			return
		}
		if len(p) == 2 && r.Method == http.MethodDelete {
			revision, e := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
			if e != nil || revision < 1 {
				mailerJSON(w, 428, map[string]string{"error": "revision_required"})
				return
			}
			if err := s.Mailer.DeleteRule(ctx, p[1], revision); err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	mailerJSON(w, 404, map[string]string{"error": "not_found"})
}
