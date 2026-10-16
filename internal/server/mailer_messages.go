package server

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"

	"aurago/internal/mailer"
)

func mailerAttachmentCount(message mailer.Message) int {
	count := len(message.Attachments)
	if message.AttachmentCount > count {
		return message.AttachmentCount
	}
	return count
}

func (s *Server) mailerMessagesHTTP(w http.ResponseWriter, r *http.Request, p []string) {
	ctx := r.Context()
	if p[0] == "threads" && len(p) == 1 && r.Method == http.MethodGet {
		q := r.URL.Query()
		query := mailer.MessageQuery{AccountID: q.Get("account_id"), FolderID: q.Get("folder_id"), View: q.Get("view"), Query: q.Get("query"), Unread: q.Get("unread") == "true", Starred: q.Get("starred") == "true", Limit: mailerQueryInt(r, "limit", 50), Offset: mailerQueryInt(r, "offset", 0)}
		if query.FolderID == "" {
			query.FolderID = q.Get("folder")
		}
		if query.Query == "" {
			query.Query = q.Get("q")
		}
		switch query.FolderID {
		case "all_inboxes", "inbox", "unread", "starred", "snoozed":
			if query.AccountID == "" {
				query.View = query.FolderID
				query.FolderID = ""
			}
		}
		if query.View == "all_inboxes" {
			query.View = "inbox"
		}
		query.Snoozed = query.View == "snoozed"
		if query.FolderID == "" {
			query.FolderID = q.Get("label_id")
		}
		query.Cursor = q.Get("cursor")
		var page mailer.ThreadPage
		var err error
		remote := q.Get("remote") == "true"
		if remote {
			page, err = s.Mailer.SearchRemote(ctx, query)
		} else {
			page, err = s.Mailer.QueryThreads(ctx, query)
		}
		if err != nil {
			mailerHTTPError(w, err)
			return
		}
		items := []map[string]any{}
		for _, thread := range page.Threads {
			var latest mailer.Message
			attachments := 0
			for _, message := range thread.Messages {
				attachments += mailerAttachmentCount(message)
				if latest.ID == "" || message.Date.After(latest.Date) {
					latest = message
				}
			}
			items = append(items, map[string]any{"id": thread.ID, "message_id": latest.ID, "account_id": thread.AccountID, "folder_id": latest.FolderID, "subject": thread.Subject, "from": latest.From, "to": latest.To, "participants": thread.Participants, "preview": thread.Preview, "date": thread.LastMessageAt, "received_at": thread.LastMessageAt, "unread": thread.Unread, "starred": thread.Starred, "attachment_count": attachments, "message_count": thread.Count, "snoozed_until": latest.SnoozedUntil})
		}
		next := page.NextCursor
		scope := "local"
		if remote {
			scope = "provider"
		}
		mailerJSON(w, 200, map[string]any{"items": items, "total": page.Total, "next_cursor": next, "offset": page.Offset, "limit": page.Limit, "search_scope": scope, "partial": remote && (next != "" || page.Total > len(items))})
		return
	}
	if p[0] == "threads" && len(p) >= 2 {
		thread, err := s.Mailer.GetThread(ctx, p[1])
		if err != nil {
			mailerHTTPError(w, err)
			return
		}
		if len(p) == 2 && r.Method == http.MethodGet {
			mailerJSON(w, 200, thread)
			return
		}
		if len(p) == 3 && r.Method == http.MethodPost && (p[2] == "read" || p[2] == "actions") {
			var body struct {
				Action              string              `json:"action"`
				Kind                mailer.MutationKind `json:"kind"`
				Read                *bool               `json:"read"`
				FolderID            string              `json:"folder_id"`
				DestinationFolderID string              `json:"destination_folder_id"`
				Destination         string              `json:"destination"`
				Label               string              `json:"label"`
				LabelID             string              `json:"label_id"`
				Until               time.Time           `json:"until"`
			}
			if err := decodeMailerJSON(w, r, &body, 8192); err != nil {
				mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
				return
			}
			kind := body.Kind
			if kind == "" {
				kind = mailer.MutationKind(body.Action)
			}
			if p[2] == "read" {
				kind = mailer.MutationRead
				if body.Read != nil && !*body.Read {
					kind = mailer.MutationUnread
				}
			}
			if kind == "mark_unread" {
				kind = mailer.MutationUnread
			}
			if kind == "mark_read" {
				kind = mailer.MutationRead
			}
			dest := body.DestinationFolderID
			if dest == "" {
				dest = body.FolderID
			}
			if dest == "" {
				dest = body.Destination
			}
			if dest == "" {
				dest = body.LabelID
			}
			if dest == "" {
				dest = body.Label
			}
			label := ""
			if kind == mailer.MutationLabelAdd || kind == mailer.MutationLabelRemove {
				label, dest = dest, ""
			}
			completed := []string{}
			for _, m := range thread.Messages {
				if kind == "snooze" {
					err = s.Mailer.Snooze(ctx, m.ID, body.Until)
				} else if kind == "unsnooze" {
					err = s.Mailer.Unsnooze(ctx, m.ID)
				} else {
					err = s.Mailer.Mutate(ctx, m.ID, mailer.Mutation{Kind: kind, DestinationFolderID: dest, Label: label})
				}
				if err != nil {
					if len(completed) > 0 {
						mailerJSON(w, http.StatusConflict, map[string]any{"error": "partial_action", "completed": completed, "failed": m.ID})
						return
					}
					mailerHTTPError(w, err)
					return
				}
				completed = append(completed, m.ID)
			}
			mailerJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	if p[0] == "messages" && len(p) >= 2 {
		if len(p) == 3 && p[2] == "rule-runs" && r.Method == http.MethodGet {
			items, err := s.Mailer.ListRuleRuns(ctx, p[1])
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, map[string]any{"items": items})
			return
		}
		if len(p) == 5 && p[2] == "rule-runs" && p[4] == "retry" && r.Method == http.MethodPost {
			if err := s.Mailer.RetryRuleRun(ctx, p[1], p[3]); err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		if len(p) == 2 && r.Method == http.MethodGet {
			m, err := s.Mailer.GetMessage(ctx, p[1])
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, m)
			return
		}
		if len(p) == 3 && p[2] == "eml" && r.Method == http.MethodGet {
			data, err := s.Mailer.RawMessage(ctx, p[1])
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			w.Header().Set("Content-Type", "message/rfc822")
			w.Header().Set("Content-Disposition", `attachment; filename="message.eml"`)
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = w.Write(data)
			return
		}
		if len(p) == 4 && p[2] == "attachments" && r.Method == http.MethodGet {
			a, stream, err := s.Mailer.MessageAttachment(ctx, p[1], p[3])
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			defer stream.Close()
			mailerAttachmentResponse(w, stream, a)
			return
		}
		if len(p) == 3 && r.Method == http.MethodPost && (p[2] == "snooze" || p[2] == "unsnooze") {
			var body struct {
				Until time.Time `json:"until"`
			}
			if err := decodeMailerJSON(w, r, &body, 4096); err != nil {
				mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
				return
			}
			var err error
			if p[2] == "snooze" {
				err = s.Mailer.Snooze(ctx, p[1], body.Until)
			} else {
				err = s.Mailer.Unsnooze(ctx, p[1])
			}
			if err != nil {
				mailerHTTPError(w, err)
				return
			}
			mailerJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	if p[0] == "attachments" && len(p) == 2 && r.Method == http.MethodGet {
		a, stream, err := s.Mailer.ReadAttachment(ctx, p[1])
		if err != nil {
			mailerHTTPError(w, err)
			return
		}
		defer stream.Close()
		mailerAttachmentResponse(w, stream, a)
		return
	}
	mailerJSON(w, 404, map[string]string{"error": "not_found"})
}

func mailerAttachmentResponse(w http.ResponseWriter, r io.Reader, a mailer.Attachment) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Filename}))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	if a.Size < 0 || a.Size > 25<<20 {
		mailerJSON(w, 413, map[string]string{"error": "attachment_too_large"})
		return
	}
	w.Header().Set("Content-Length", fmt.Sprint(a.Size))
	if n, err := io.CopyN(w, r, a.Size); err != nil || n != a.Size {
		panic(http.ErrAbortHandler)
	}
}
