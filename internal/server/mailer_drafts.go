package server

import (
	"io"
	"net/http"
	"strconv"

	"aurago/internal/mailer"
)

func (s *Server) mailerDraftsHTTP(w http.ResponseWriter, r *http.Request, p []string) {
	ctx := r.Context()
	if p[0] == "outbox" {
		switch {
		case len(p) == 1 && r.Method == http.MethodGet:
			page, err := s.Mailer.ListOutboxPage(ctx, r.URL.Query().Get("cursor"), mailerQueryInt(r, "limit", 50))
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, page)
		case len(p) == 1 && r.Method == http.MethodPost:
			var body mailer.SendRequest
			if err := decodeMailerJSON(w, r, &body, 8192); err != nil {
				mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
				return
			}
			item, err := s.Mailer.QueueSend(ctx, body)
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 201, item)
		case len(p) == 2 && r.Method == http.MethodDelete:
			if err := s.Mailer.CancelSend(ctx, p[1]); err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, map[string]bool{"ok": true})
		case len(p) == 3 && p[2] == "retry" && r.Method == http.MethodPost:
			item, err := s.Mailer.RetrySend(ctx, p[1])
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, item)
		default:
			mailerJSON(w, 404, map[string]string{"error": "not_found"})
		}
		return
	}
	if len(p) == 1 && r.Method == http.MethodGet {
		items, err := s.Mailer.ListDrafts(ctx)
		if err != nil {
			mailerHTTPError(w, err)
			return
		}
		mailerJSON(w, 200, map[string]any{"items": items})
		return
	}
	if len(p) == 2 && r.Method == http.MethodGet {
		draft, err := s.Mailer.GetDraft(ctx, p[1])
		if err != nil {
			mailerHTTPError(w, err)
			return
		}
		mailerJSON(w, 200, draft)
		return
	}
	if (len(p) == 1 && r.Method == http.MethodPost) || (len(p) == 2 && r.Method == http.MethodPut) {
		var body mailer.Draft
		if err := decodeMailerJSON(w, r, &body, 1<<20); err != nil {
			mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		if len(p) == 1 {
			if body.ID != "" || body.Revision != 0 {
				mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
				return
			}
		} else {
			if body.ID != "" && body.ID != p[1] {
				mailerJSON(w, 400, map[string]string{"error": "draft_mismatch"})
				return
			}
			body.ID = p[1]
		}
		draft, err := s.Mailer.SaveDraft(ctx, body)
		if err != nil {
			mailerHTTPError(w, err)
			return
		}
		mailerJSON(w, 200, draft)
		return
	}
	if len(p) == 2 && r.Method == http.MethodDelete {
		revision, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		if err != nil || revision < 1 {
			mailerJSON(w, 428, map[string]string{"error": "revision_required"})
			return
		}
		if err := s.Mailer.DeleteDraft(ctx, p[1], revision); err != nil {
			mailerHTTPError(w, err)
			return
		}
		mailerJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if len(p) >= 3 && p[2] == "attachments" {
		revision, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		if err != nil || revision < 1 {
			mailerJSON(w, 428, map[string]string{"error": "revision_required"})
			return
		}
		if len(p) == 4 && p[3] == "from-message" && r.Method == http.MethodPost {
			var body struct {
				MessageID    string `json:"message_id"`
				AttachmentID string `json:"attachment_id"`
			}
			if decodeMailerJSON(w, r, &body, 4096) != nil || body.MessageID == "" || body.AttachmentID == "" {
				mailerHTTPError(w, mailer.ErrInvalid)
				return
			}
			draft, err := s.Mailer.GetDraft(ctx, p[1])
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			if draft.Revision != revision {
				mailerHTTPError(w, mailer.ErrConflict)
				return
			}
			attachment, stream, err := s.Mailer.MessageAttachment(ctx, body.MessageID, body.AttachmentID)
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			defer stream.Close()
			draft, err = s.Mailer.AddDraftAttachment(ctx, draft.ID, revision, attachment.Filename, attachment.ContentType, stream)
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, draft)
			return
		}
		if len(p) == 4 && r.Method == http.MethodDelete {
			draft, err := s.Mailer.RemoveDraftAttachment(ctx, p[1], revision, p[3])
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, draft)
			return
		}
		if len(p) == 3 && r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 26<<20)
			reader, err := r.MultipartReader()
			if err != nil {
				mailerJSON(w, 400, map[string]string{"error": "invalid_upload"})
				return
			}
			part, err := reader.NextPart()
			if err != nil || part.FormName() != "file" || part.FileName() == "" {
				mailerJSON(w, 400, map[string]string{"error": "invalid_upload"})
				return
			}
			defer part.Close()
			draft, err := s.Mailer.AddDraftAttachment(ctx, p[1], revision, part.FileName(), part.Header.Get("Content-Type"), io.LimitReader(part, (25<<20)+1))
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, draft)
			return
		}
	}
	mailerJSON(w, 404, map[string]string{"error": "not_found"})
}
