package server

import (
	"net/http"
	"strconv"

	"aurago/internal/chibisensei"
)

// chibiBulkTokenIDs is the number of ids a bulk delete may name without a delete-preview token.
const chibiBulkTokenIDs = 50

// chibiQueryInt reads an integer query parameter; an unreadable one answers 400.
func chibiQueryInt(w http.ResponseWriter, r *http.Request, key string, def int) (int, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		writeChibiError(w, http.StatusBadRequest, "invalid_request", "The query parameter "+key+" is not a number.", nil)
		return 0, false
	}
	return n, true
}

func handleChibiRows(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
		if !ok {
			return
		}
		q := r.URL.Query()
		var rq chibisensei.RowQuery
		rq.Q, rq.Split, rq.Source = q.Get("q"), q.Get("split"), q.Get("source")
		rq.Flag, rq.Label, rq.Batch = q.Get("flag"), q.Get("label"), q.Get("batch")
		rq.Sort, rq.Dir = q.Get("sort"), q.Get("dir")
		if (rq.Sort != "" && rq.Sort != "seq" && rq.Sort != "est_tokens") || (rq.Dir != "" && rq.Dir != "asc" && rq.Dir != "desc") {
			writeChibiError(w, http.StatusBadRequest, "invalid_request", "Unknown sort or direction.", nil)
			return
		}
		var good bool
		if rq.Offset, good = chibiQueryInt(w, r, "offset", 0); !good {
			return
		}
		if rq.Limit, good = chibiQueryInt(w, r, "limit", chibisensei.PageDefault); !good {
			return
		}
		page, err := acc.rt.store.ListRows(r.Context(), id, rq)
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		writeChibiJSON(w, http.StatusOK, page)
	case http.MethodPost:
		acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
		if !ok {
			return
		}
		var req struct {
			Rows []chibisensei.RowInput `json:"rows"`
		}
		if !chibiDecode(w, r, &req, chibiRowsJSONLimit) {
			return
		}
		if len(req.Rows) < 1 || len(req.Rows) > chibisensei.MaxRowsPerPost {
			chibiValidation(w, "rows", "row.count", "Send 1 to 200 rows.")
			return
		}
		if chibiStorageFull(w, acc) {
			return
		}
		res, err := acc.rt.store.AddRows(r.Context(), id, req.Rows, chibisensei.AddOptions{
			Source: "manual", BatchKind: "add", MaxRows: acc.cfg.ChibiSensei.Dataset.MaxRows})
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		if res.Rows == nil {
			res.Rows = []chibisensei.Row{}
		}
		writeChibiJSON(w, http.StatusCreated, map[string]any{"rows": res.Rows, "data_rev": res.DataRev})
	default:
		writeChibiMethodError(w, "GET, POST")
	}
}

func handleChibiRow(s *Server, w http.ResponseWriter, r *http.Request, id string, rowID int64) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
		if !ok {
			return
		}
		row, err := acc.rt.store.GetRow(r.Context(), id, rowID)
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		writeChibiJSON(w, http.StatusOK, map[string]any{"row": row})
	case http.MethodPut:
		acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
		if !ok {
			return
		}
		rev, ok := chibiIfMatch(w, r)
		if !ok {
			return
		}
		var in chibisensei.RowInput
		if !chibiDecode(w, r, &in, chibiJSONLimit) {
			return
		}
		row, dataRev, err := acc.rt.store.UpdateRow(r.Context(), id, rowID, rev, in)
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		writeChibiJSON(w, http.StatusOK, map[string]any{"row": row, "data_rev": dataRev})
	default:
		writeChibiMethodError(w, "GET, PUT")
	}
}

// handleChibiRowsBulk applies a bulk action. A delete of more than 50 named rows, or any delete by
// filter, needs the token of a delete preview of the same selection and data revision.
func handleChibiRowsBulk(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	var req struct {
		Select chibisensei.Select `json:"select"`
		Action string             `json:"action"`
		Value  string             `json:"value"`
		Token  string             `json:"token"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	ctx := r.Context()
	if req.Action == "delete" && (req.Select.Filter != nil || len(req.Select.IDs) > chibiBulkTokenIDs) {
		if req.Token == "" {
			writeChibiError(w, http.StatusPreconditionRequired, "confirmation_required", "Preview the delete and send its token.", nil)
			return
		}
		p, err := acc.rt.store.GetProject(ctx, id)
		if err != nil {
			chibiDomainError(w, err)
			return
		}
		if !acc.rt.deletes.Consume(req.Token, id, req.Select, p.DataRev) {
			writeChibiError(w, http.StatusConflict, "confirmation_invalid", "The confirmation is wrong or expired; preview the delete again.", nil)
			return
		}
	}
	ids, err := acc.rt.store.ResolveSelect(ctx, id, req.Select)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	res, err := acc.rt.store.Bulk(ctx, id, ids, req.Action, req.Value)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, res)
}

func handleChibiRowsDeletePreview(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	var req struct {
		Select chibisensei.Select `json:"select"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	ctx := r.Context()
	p, err := acc.rt.store.GetProject(ctx, id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	ids, err := acc.rt.store.ResolveSelect(ctx, id, req.Select)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	token, ttl := acc.rt.deletes.Issue(id, req.Select, p.DataRev)
	writeChibiJSON(w, http.StatusOK, map[string]any{"count": len(ids), "token": token, "expires_in_s": int(ttl.Seconds())})
}

func handleChibiRowsDedupe(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	res, err := acc.rt.store.Dedupe(r.Context(), id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"removed": res.Affected, "batch_id": res.BatchID, "data_rev": res.DataRev})
}

func handleChibiHistory(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	limit, ok := chibiQueryInt(w, r, "limit", 20)
	if !ok {
		return
	}
	batches, err := acc.rt.store.History(r.Context(), id, limit)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"batches": batches})
}

func handleChibiUndo(s *Server, w http.ResponseWriter, r *http.Request, id, batchID string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	res, err := acc.rt.store.Undo(r.Context(), id, batchID)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, res)
}
