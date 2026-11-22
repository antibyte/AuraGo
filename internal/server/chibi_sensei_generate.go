package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"aurago/internal/chibisensei"
	"aurago/internal/security"
)

// chibiLLMJobTimeout bounds one generation or cleaning job.
const chibiLLMJobTimeout = 60 * time.Minute

var chibiLanguagePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$`)

// chibiDay is the local day the daily row cap counts in.
func chibiDay() string { return time.Now().Format("2006-01-02") }

type chibiGenerateRequest struct {
	Count        int      `json:"count"`
	SeedRowIDs   []int64  `json:"seed_row_ids"`
	DocumentIDs  []string `json:"document_ids"`
	Instructions string   `json:"instructions"`
	Language     string   `json:"language"`
}

type chibiCleanRequest struct {
	Select       chibisensei.Select `json:"select"`
	Instructions string             `json:"instructions"`
}

// chibiLLMGate answers the checks a generation or cleaning request passes before a job exists: storage, the
// generator route and the budget. It returns the route; the caller owns route.release.
func chibiLLMGate(s *Server, w http.ResponseWriter, acc *chibiSenseiAccess) (*chibiRoute, bool) {
	if chibiStorageFull(w, acc) {
		return nil, false
	}
	rt, err := s.chibiGeneratorRoute(acc.cfg)
	if err != nil {
		writeChibiError(w, http.StatusServiceUnavailable, "provider_unavailable", "No generator model is available.", nil)
		return nil, false
	}
	if s.chibiBudgetBlocked() {
		rt.release()
		writeChibiError(w, http.StatusTooManyRequests, "budget_exceeded", "The daily AI budget is used up.", nil)
		return nil, false
	}
	return rt, true
}

// chibiReserveDaily reserves rows of the daily cap; it answers 429 generator_daily_limit when the cap is reached.
func chibiReserveDaily(w http.ResponseWriter, r *http.Request, acc *chibiSenseiAccess, day string, n int) bool {
	ok, err := acc.rt.store.ReserveDaily(r.Context(), day, n, acc.cfg.ChibiSensei.Generator.MaxDailyRows)
	if err != nil {
		chibiDomainError(w, err)
		return false
	}
	if !ok {
		writeChibiError(w, http.StatusTooManyRequests, "generator_daily_limit", "The daily limit of generated rows is reached.", nil)
		return false
	}
	return true
}

// handleChibiGenerate starts a generation job.
func handleChibiGenerate(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	var req chibiGenerateRequest
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	maxRows := acc.cfg.ChibiSensei.Generator.MaxRowsPerJob
	switch {
	case req.Count < 1 || req.Count > maxRows:
		chibiValidation(w, "count", "generate.bad_count", "The count is outside the allowed range.")
		return
	case len(req.SeedRowIDs) > chibisensei.MaxSeeds:
		chibiValidation(w, "seed_row_ids", "generate.too_many_seeds", "Use at most 10 example rows.")
		return
	case len(req.DocumentIDs) > chibisensei.MaxDocuments:
		chibiValidation(w, "document_ids", "generate.too_many_documents", "Too many documents.")
		return
	case utf8.RuneCountInString(req.Instructions) > 2000:
		chibiValidation(w, "instructions", "generate.instructions_too_long", "The instructions are too long.")
		return
	case req.Language != "" && !chibiLanguagePattern.MatchString(req.Language):
		chibiValidation(w, "language", "generate.bad_language", "Use a language code such as en or pt-BR.")
		return
	}
	store := acc.rt.store
	project, err := store.GetProject(r.Context(), id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	seeds, docs, ok := chibiGenerateInputs(w, r, store, id, req)
	if !ok {
		return
	}
	route, ok := chibiLLMGate(s, w, acc)
	if !ok {
		return
	}
	day := chibiDay()
	if !chibiReserveDaily(w, r, acc, day, req.Count) {
		route.release()
		return
	}
	// From here the job owns the reservation and the route; every early exit gives them back.
	giveBack := func() {
		route.release()
		_ = store.ReleaseDaily(context.Background(), day, req.Count)
	}
	ctx, release, err := s.beginDesktopBackgroundRun(chibiLLMJobTimeout)
	if err != nil {
		giveBack()
		writeChibiError(w, http.StatusForbidden, "desktop_readonly", "The desktop does not allow this action.", nil)
		return
	}
	datasetMax := acc.cfg.ChibiSensei.Dataset.MaxRows
	in := chibiGenerateJob{project: project, seeds: seeds, docs: docs, count: req.Count, instructions: req.Instructions,
		language: req.Language, datasetMax: datasetMax, day: day, full: func() bool { return chibiStoreFull(acc) }}
	params := map[string]any{"count": req.Count, "seed_row_ids": req.SeedRowIDs, "document_ids": req.DocumentIDs, "language": req.Language}
	job, err := acc.rt.jobs.Start(ctx, release, id, "generate", params, req.Count,
		func(jctx context.Context, report func(chibisensei.Progress)) (chibisensei.JobResult, error) {
			return s.runChibiGenerate(jctx, store, route, in, report)
		})
	if err != nil {
		giveBack()
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

// chibiGenerateInputs loads the seed rows and documents of a request; a foreign or unknown one is a 422.
func chibiGenerateInputs(w http.ResponseWriter, r *http.Request, store *chibisensei.Store, id string, req chibiGenerateRequest) ([]chibisensei.Row, []chibisensei.Document, bool) {
	var seeds []chibisensei.Row
	if len(req.SeedRowIDs) > 0 {
		seen := map[int64]bool{}
		for _, rid := range req.SeedRowIDs {
			if rid <= 0 || seen[rid] {
				chibiValidation(w, "seed_row_ids", "generate.bad_seed", "An example row is invalid or listed twice.")
				return nil, nil, false
			}
			seen[rid] = true
		}
		rows, err := store.RowsByIDs(r.Context(), id, req.SeedRowIDs)
		if err != nil {
			chibiDomainError(w, err)
			return nil, nil, false
		}
		if len(rows) != len(req.SeedRowIDs) {
			chibiValidation(w, "seed_row_ids", "generate.bad_seed", "An example row does not belong to this project.")
			return nil, nil, false
		}
		seeds = rows
	}
	var docs []chibisensei.Document
	if len(req.DocumentIDs) > 0 {
		seen := map[string]bool{}
		for _, did := range req.DocumentIDs {
			if !chibisensei.ValidID(did) || seen[did] {
				chibiValidation(w, "document_ids", "generate.bad_document", "A document is invalid or listed twice.")
				return nil, nil, false
			}
			seen[did] = true
		}
		got, err := store.GetDocuments(r.Context(), id, req.DocumentIDs)
		if errors.Is(err, chibisensei.ErrNotFound) {
			chibiValidation(w, "document_ids", "generate.bad_document", "A document does not belong to this project.")
			return nil, nil, false
		}
		if err != nil {
			chibiDomainError(w, err)
			return nil, nil, false
		}
		docs = got
	}
	return seeds, docs, true
}

type chibiGenerateJob struct {
	project      chibisensei.Project
	seeds        []chibisensei.Row
	docs         []chibisensei.Document
	count        int
	instructions string
	language     string
	datasetMax   int
	day          string
	full         func() bool // storage limit reached; checked before every batch of rows (nil: never)
}

// runChibiGenerate is the body of a generation job. It asks for at most 10 rows per call, makes at most
// ceil(count/10)+2 calls and stops after the current call when ctx is cancelled. The rows it adds share one batch
// (history kind "generate"); the batch is closed when the job ends, whatever its outcome, and the unused part of
// the daily reservation is refunded.
func (s *Server) runChibiGenerate(ctx context.Context, st *chibisensei.Store, rt *chibiRoute, in chibiGenerateJob, report func(chibisensei.Progress)) (res chibisensei.JobResult, err error) {
	batch := uuid.NewString()
	defer rt.release()
	defer func() { _ = st.ReleaseDaily(context.Background(), in.day, in.count-res.RowsAdded) }()
	defer func() { _ = st.CloseBatch(context.Background(), batch) }()
	defer func() {
		if res.RowsAdded > 0 {
			res.BatchID = batch
		}
	}()
	if err := ctx.Err(); err != nil {
		return res, err // cancelled while queued: only the defers run
	}

	free := in.project.Recipe == chibisensei.RecipeFree
	chunks := chibisensei.ChunkText(in.docs, chibisensei.ChunkRunes)
	maxCalls := (in.count+chibisensei.GenerateBatch-1)/chibisensei.GenerateBatch + 2
	calls, limitHit := 0, false
	for res.RowsAdded < in.count && calls < maxCalls {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		want := min(chibisensei.GenerateBatch, in.count-res.RowsAdded)
		chunk := ""
		if len(chunks) > 0 {
			chunk = chunks[calls%len(chunks)]
		}
		system, user := chibisensei.BuildGenerate(chibisensei.GenerateInput{Project: in.project, Seeds: in.seeds, Chunk: chunk,
			Instructions: in.instructions, Language: in.language, Want: want}, security.IsolateExternalData)
		out, cerr := chibiLLM{s}.Complete(ctx, rt, system, user, chibisensei.GenerateSchema(free), 0.8, 4096)
		calls++
		if errors.Is(cerr, errChibiWithheld) {
			res.Withheld++
			continue
		}
		if cerr != nil {
			return res, s.chibiJobError(cerr)
		}
		rows, rejected, perr := chibisensei.ParseGenerated(out, in.project, in.seeds, want)
		if perr != nil {
			res.RowsSkipped += want
			continue
		}
		res.RowsSkipped += rejected
		if len(rows) > 0 {
			added, aerr := st.AddRows(ctx, in.project.ID, rows, chibisensei.AddOptions{Source: "generated", BatchID: batch, BatchKind: "generate",
				MaxRows: in.datasetMax, Partial: true, SkipDuplicates: true, Strict: true, Full: in.full})
			switch {
			case errors.Is(aerr, chibisensei.ErrAlreadyUndone):
				return res, context.Canceled // the user undid the batch: stop, this is a cancel and not a failure
			case errors.Is(aerr, chibisensei.ErrStorageFull):
				return res, chibisensei.StorageFullError() // keeps the rows added so far
			case aerr != nil && ctx.Err() != nil:
				return res, ctx.Err()
			case aerr != nil:
				return res, aerr
			}
			res.RowsAdded += added.Added
			res.RowsSkipped += added.Skipped
			if added.LimitReached {
				limitHit = true
				report(chibisensei.Progress{Done: res.RowsAdded, Total: in.count, Phase: "generating"})
				break
			}
		}
		report(chibisensei.Progress{Done: res.RowsAdded, Total: in.count, Phase: "generating"})
	}
	if err := ctx.Err(); err != nil {
		return res, err
	}
	if res.RowsAdded == 0 && calls > 0 && !limitHit {
		return res, chibiNothingUsable(calls, res.Withheld)
	}
	return res, nil
}

// chibiNothingUsable is the failure of a job that made calls and produced nothing: every call withheld by the
// security scan, or an answer that could not be used (unparsable, or every row or item rejected).
func chibiNothingUsable(calls, withheld int) error {
	if withheld == calls {
		return &chibisensei.JobError{Code: "prompt_withheld", Message: "The security scan withheld the request."}
	}
	return &chibisensei.JobError{Code: "invalid_answer", Message: "The model returned an answer that could not be used."}
}

// handleChibiClean starts a cleaning job over at most 500 selected rows.
func handleChibiClean(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	var req chibiCleanRequest
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	if utf8.RuneCountInString(req.Instructions) > 2000 {
		chibiValidation(w, "instructions", "generate.instructions_too_long", "The instructions are too long.")
		return
	}
	store := acc.rt.store
	project, err := store.GetProject(r.Context(), id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	ids, err := store.ResolveSelect(r.Context(), id, req.Select)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	switch {
	case len(ids) == 0:
		chibiValidation(w, "select", "clean.no_rows", "No rows are selected.")
		return
	case len(ids) > chibisensei.CleanMaxRows:
		writeChibiError(w, http.StatusUnprocessableEntity, "too_many_rows", "Clean at most 500 rows at a time.",
			[]chibisensei.Issue{{Field: "select", MessageKey: "chibi_sensei.clean.too_many_rows", Message: "Clean at most 500 rows at a time."}})
		return
	}
	route, ok := chibiLLMGate(s, w, acc)
	if !ok {
		return
	}
	day := chibiDay()
	if !chibiReserveDaily(w, r, acc, day, len(ids)) {
		route.release()
		return
	}
	giveBack := func() {
		route.release()
		_ = store.ReleaseDaily(context.Background(), day, len(ids))
	}
	ctx, release, err := s.beginDesktopBackgroundRun(chibiLLMJobTimeout)
	if err != nil {
		giveBack()
		writeChibiError(w, http.StatusForbidden, "desktop_readonly", "The desktop does not allow this action.", nil)
		return
	}
	jobIDs := make(chan string, 1) // the job learns its own id after Start returned
	in := chibiCleanJob{project: project, ids: ids, instructions: req.Instructions, day: day, jobID: jobIDs}
	job, err := acc.rt.jobs.Start(ctx, release, id, "clean", map[string]any{"rows": len(ids)}, len(ids),
		func(jctx context.Context, report func(chibisensei.Progress)) (chibisensei.JobResult, error) {
			return s.runChibiClean(jctx, store, route, in, report)
		})
	if err != nil {
		giveBack()
		chibiDomainError(w, err)
		return
	}
	jobIDs <- job.ID
	writeChibiJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

type chibiCleanJob struct {
	project      chibisensei.Project
	ids          []int64
	instructions string
	day          string
	jobID        <-chan string
}

// chibiRowRunes counts the runes of a row's content.
func chibiRowRunes(r chibisensei.Row) int {
	n := 0
	if r.Text != nil {
		n += utf8.RuneCountInString(*r.Text)
	}
	for _, m := range r.Messages {
		n += utf8.RuneCountInString(m.Content)
	}
	return n
}

// runChibiClean is the body of a cleaning job: batches of 10 rows, one call each, the verdicts stored with the
// rev each row had when it was sent. Rows over 8000 runes and rows deleted since the selection are skipped. The
// reservation of the daily cap covers the rows selected; the rows not sent are refunded.
func (s *Server) runChibiClean(ctx context.Context, st *chibisensei.Store, rt *chibiRoute, in chibiCleanJob, report func(chibisensei.Progress)) (res chibisensei.JobResult, err error) {
	sent := 0
	defer rt.release()
	defer func() { _ = st.ReleaseDaily(context.Background(), in.day, len(in.ids)-sent) }()
	if err := ctx.Err(); err != nil {
		return res, err // cancelled while queued: only the defers run
	}
	var jobID string
	select {
	case jobID = <-in.jobID:
	case <-ctx.Done():
		return res, ctx.Err()
	}
	free := in.project.Recipe == chibisensei.RecipeFree
	calls := 0
	total := len(in.ids)
	for start := 0; start < total; start += chibisensei.CleanBatch {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		batchIDs := in.ids[start:min(start+chibisensei.CleanBatch, total)]
		rows, rerr := st.RowsByIDs(ctx, in.project.ID, batchIDs)
		if rerr != nil {
			return res, rerr
		}
		res.RowsSkipped += len(batchIDs) - len(rows)
		send := rows[:0:0]
		for _, row := range rows {
			if chibiRowRunes(row) > chibisensei.CleanMaxRowRunes {
				res.RowsSkipped++
				continue
			}
			send = append(send, row)
		}
		if len(send) > 0 {
			system, user := chibisensei.BuildClean(in.project, send, in.instructions, security.IsolateExternalData)
			out, cerr := chibiLLM{s}.Complete(ctx, rt, system, user, chibisensei.CleanSchema(free), 0.2, 8192)
			calls++
			switch {
			case errors.Is(cerr, errChibiWithheld):
				res.Withheld++
				res.RowsSkipped += len(send)
			case cerr != nil:
				return res, s.chibiJobError(cerr)
			default:
				sent += len(send)
				items, invalid, perr := chibisensei.ParseClean(out, send)
				if perr != nil {
					res.RowsSkipped += len(send)
					break
				}
				res.RowsSkipped += invalid
				revs := make(map[int64]int64, len(send))
				for _, row := range send {
					revs[row.ID] = row.Rev
				}
				if serr := st.SaveSuggestions(ctx, jobID, items, revs); serr != nil {
					if ctx.Err() != nil {
						return res, ctx.Err()
					}
					return res, serr
				}
				res.Suggestions += len(items)
			}
		}
		report(chibisensei.Progress{Done: min(start+chibisensei.CleanBatch, total), Total: total, Phase: "cleaning"})
	}
	if res.Suggestions == 0 && calls > 0 {
		return res, chibiNothingUsable(calls, res.Withheld)
	}
	return res, nil
}

// chibiLoadJob loads a job for the suggestion routes.
func chibiLoadJob(w http.ResponseWriter, r *http.Request, acc *chibiSenseiAccess, jobID string) (chibisensei.Job, bool) {
	job, err := acc.rt.store.GetJob(r.Context(), jobID)
	if errors.Is(err, chibisensei.ErrNotFound) {
		chibiJobNotFound(w)
		return chibisensei.Job{}, false
	}
	if err != nil {
		chibiDomainError(w, err)
		return chibisensei.Job{}, false
	}
	return job, true
}

// handleChibiSuggestions pages the suggestions of a cleaning job.
func handleChibiSuggestions(s *Server, w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	offset, ok1 := chibiQueryInt(w, r, "offset", 0)
	if !ok1 {
		return
	}
	limit, ok2 := chibiQueryInt(w, r, "limit", 50)
	if !ok2 {
		return
	}
	if offset < 0 || limit < 1 || limit > 200 {
		chibiValidation(w, "limit", "generate.bad_paging", "Use offset 0 or more and a limit from 1 to 200.")
		return
	}
	if _, ok := chibiLoadJob(w, r, acc, jobID); !ok {
		return
	}
	list, total, err := acc.rt.store.ListSuggestions(r.Context(), jobID, offset, limit)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"suggestions": list, "total": total})
}

// handleChibiApply applies the accepted suggestions of a finished cleaning job.
func handleChibiApply(s *Server, w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok {
		return
	}
	var req struct {
		Accept json.RawMessage `json:"accept"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	var accept []int64 // nil = "all"
	var word string
	if json.Unmarshal(req.Accept, &word) == nil && word == "all" {
		accept = nil
	} else if err := json.Unmarshal(req.Accept, &accept); err != nil || accept == nil || len(accept) > chibisensei.CleanMaxRows {
		chibiValidation(w, "accept", "apply.bad_accept", `Send "all" or a list of row ids.`)
		return
	}
	for _, id := range accept {
		if id <= 0 {
			chibiValidation(w, "accept", "apply.bad_accept", `Send "all" or a list of row ids.`)
			return
		}
	}
	job, ok := chibiLoadJob(w, r, acc, jobID)
	if !ok {
		return
	}
	if job.Kind != "clean" {
		chibiValidation(w, "job", "apply.not_a_clean_job", "Only a cleaning job has suggestions.")
		return
	}
	// A failed job keeps the suggestions it saved before it stopped, so every finished state may apply them.
	if job.State == chibisensei.JobQueued || job.State == chibisensei.JobRunning {
		writeChibiError(w, http.StatusConflict, "job_not_finished", "The cleaning job is not finished.", nil)
		return
	}
	out, err := acc.rt.store.ApplySuggestions(r.Context(), job.ProjectID, jobID, accept)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, out)
}
