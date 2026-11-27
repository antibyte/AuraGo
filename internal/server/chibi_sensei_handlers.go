package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/chibisensei"
)

const chibiSenseiAPIBase = "/api/desktop/chibi-sensei"

func chibiNotFound(w http.ResponseWriter) {
	writeChibiError(w, http.StatusNotFound, "not_found", "Chibi Sensei endpoint not found.", nil)
}

// handleChibiSensei routes /api/desktop/chibi-sensei/*. Every handler passes chibiSenseiAuthorize;
// IDs are validated here before any handler touches the store. The runtime a request holds is
// released when the router returns.
func handleChibiSensei(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, chibiSenseiAPIBase+"/") {
			chibiNotFound(w)
			return
		}
		lease := &chibiLease{}
		r = r.WithContext(context.WithValue(r.Context(), chibiLeaseKey{}, lease))
		defer func() {
			if lease.release != nil {
				lease.release()
			}
		}()
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, chibiSenseiAPIBase), "/"), "/")
		switch {
		case len(parts) == 1 && parts[0] == "meta":
			handleChibiMeta(s, w, r)
		case len(parts) == 1 && parts[0] == "projects":
			handleChibiProjects(s, w, r)
		case len(parts) >= 2 && parts[0] == "projects":
			if !chibisensei.ValidID(parts[1]) {
				chibiNotFound(w)
				return
			}
			handleChibiProjectRoute(s, w, r, parts[1], parts[2:])
		case len(parts) == 2 && parts[0] == "jobs":
			if !chibisensei.ValidID(parts[1]) {
				writeChibiError(w, http.StatusNotFound, "job_not_found", "Job not found.", nil)
				return
			}
			handleChibiJob(s, w, r, parts[1])
		case len(parts) == 3 && parts[0] == "jobs" && parts[2] == "cancel":
			if !chibisensei.ValidID(parts[1]) {
				writeChibiError(w, http.StatusNotFound, "job_not_found", "Job not found.", nil)
				return
			}
			handleChibiJobCancel(s, w, r, parts[1])
		case len(parts) == 3 && parts[0] == "jobs" && (parts[2] == "suggestions" || parts[2] == "apply"):
			if !chibisensei.ValidID(parts[1]) {
				writeChibiError(w, http.StatusNotFound, "job_not_found", "Job not found.", nil)
				return
			}
			if parts[2] == "apply" {
				handleChibiApply(s, w, r, parts[1])
			} else {
				handleChibiSuggestions(s, w, r, parts[1])
			}
		case len(parts) >= 2 && parts[0] == "runs":
			if !chibisensei.ValidID(parts[1]) {
				chibiRunNotFound(w)
				return
			}
			handleChibiRunRoute(s, w, r, parts[1], parts[2:])
		case len(parts) == 1 && parts[0] == "runtime":
			handleChibiRuntime(s, w, r)
		case len(parts) == 2 && parts[0] == "runtime":
			switch parts[1] {
			case "chat":
				handleChibiChat(s, w, r)
			case "install":
				handleChibiRuntimeInstall(s, w, r)
			case "stop":
				handleChibiRuntimeStop(s, w, r)
			default:
				chibiNotFound(w)
			}
		case len(parts) == 1 && parts[0] == "consents":
			handleChibiConsents(s, w, r)
		case len(parts) == 2 && parts[0] == "preflight" && parts[1] == "enable":
			handleChibiPreflightEnable(s, w, r)
		default:
			chibiNotFound(w)
		}
	}
}

// handleChibiProjectRoute routes /projects/{id}/<rest...>.
func handleChibiProjectRoute(s *Server, w http.ResponseWriter, r *http.Request, id string, rest []string) {
	switch {
	case len(rest) == 0:
		handleChibiProject(s, w, r, id)
	case len(rest) == 1:
		switch rest[0] {
		case "archive":
			handleChibiProjectArchive(s, w, r, id)
		case "delete":
			handleChibiProjectDelete(s, w, r, id)
		case "rows":
			handleChibiRows(s, w, r, id)
		case "history":
			handleChibiHistory(s, w, r, id)
		case "checks":
			handleChibiChecks(s, w, r, id)
		case "split":
			handleChibiSplit(s, w, r, id)
		case "plan":
			handleChibiPlan(s, w, r, id)
		case "preflight":
			handleChibiPreflight(s, w, r, id)
		case "documents":
			handleChibiDocuments(s, w, r, id)
		case "imports":
			handleChibiImport(s, w, r, id)
		case "exports":
			handleChibiExport(s, w, r, id)
		case "generate":
			handleChibiGenerate(s, w, r, id)
		case "clean":
			handleChibiClean(s, w, r, id)
		case "jobs":
			handleChibiProjectJobs(s, w, r, id)
		case "events":
			handleChibiEvents(s, w, r, id)
		case "runs":
			handleChibiProjectRuns(s, w, r, id)
		default:
			chibiNotFound(w)
		}
	case len(rest) == 2 && rest[0] == "rows":
		switch rest[1] {
		case "bulk":
			handleChibiRowsBulk(s, w, r, id)
		case "delete-preview":
			handleChibiRowsDeletePreview(s, w, r, id)
		case "dedupe":
			handleChibiRowsDedupe(s, w, r, id)
		default:
			rowID, err := strconv.ParseInt(rest[1], 10, 64)
			if err != nil || rowID <= 0 {
				chibiNotFound(w)
				return
			}
			handleChibiRow(s, w, r, id, rowID)
		}
	case len(rest) == 2 && rest[0] == "checks" && rest[1] == "run":
		handleChibiChecksRun(s, w, r, id)
	case len(rest) == 2 && rest[0] == "split" && rest[1] == "reseed":
		handleChibiSplitReseed(s, w, r, id)
	case len(rest) == 2 && rest[0] == "imports" && rest[1] == "preview":
		handleChibiImportPreview(s, w, r, id)
	case len(rest) == 2 && rest[0] == "documents":
		if !chibisensei.ValidID(rest[1]) {
			chibiNotFound(w)
			return
		}
		handleChibiDocument(s, w, r, id, rest[1])
	case len(rest) == 3 && rest[0] == "history" && rest[2] == "undo":
		if !chibisensei.ValidID(rest[1]) {
			chibiNotFound(w)
			return
		}
		handleChibiUndo(s, w, r, id, rest[1])
	case len(rest) == 2 && rest[0] == "runs" && rest[1] == "prepare":
		handleChibiRunPrepare(s, w, r, id)
	default:
		chibiNotFound(w)
	}
}

// handleChibiRunRoute routes /runs/{runId}/<rest...>.
func handleChibiRunRoute(s *Server, w http.ResponseWriter, r *http.Request, runID string, rest []string) {
	switch {
	case len(rest) == 0:
		handleChibiRun(s, w, r, runID)
	case len(rest) == 1:
		switch rest[0] {
		case "cancel":
			handleChibiRunCancel(s, w, r, runID)
		case "delete":
			handleChibiRunDelete(s, w, r, runID)
		case "metrics":
			handleChibiRunMetrics(s, w, r, runID)
		case "events":
			handleChibiRunEvents(s, w, r, runID)
		case "artifacts":
			handleChibiRunArtifacts(s, w, r, runID)
		case "report":
			handleChibiRunReport(s, w, r, runID)
		case "log":
			handleChibiRunLog(s, w, r, runID)
		case "test":
			handleChibiRunTest(s, w, r, runID)
		case "samples":
			handleChibiRunSamples(s, w, r, runID)
		default:
			chibiNotFound(w)
		}
	case len(rest) == 2 && rest[0] == "report" && rest[1] == "seen":
		handleChibiReportSeen(s, w, r, runID)
	case len(rest) == 3 && rest[0] == "artifacts" && rest[2] == "file":
		if !chibisensei.ValidID(rest[1]) {
			chibiArtifactNotFound(w)
			return
		}
		handleChibiArtifactFile(s, w, r, runID, rest[1])
	default:
		chibiNotFound(w)
	}
}
