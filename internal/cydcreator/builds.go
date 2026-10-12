package cydcreator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/fileutil"
)

const (
	// maxQueuedBuilds bounds waiting builds (each holds its generated tree).
	maxQueuedBuilds = 8
	maxELFBytes     = 128 << 20
	maxBacktrace    = 64
	buildIDPrefix   = "ccb_"
	// maxFontArtifactBytes bounds one kept font file: lv_font_conv writes at
	// most about 7 bytes of C text per byte of font data, and a font's data
	// must fit the largest app slot (0x640000, 16 MB flash) or the build fails
	// the flash budget before any font is kept.
	maxFontArtifactBytes = 7 * 0x640000
)

var (
	errBuildCancelled = errors.New("build cancelled")
	backtraceAddress  = regexp.MustCompile(`^0x[0-9a-fA-F]{1,16}$`)
	// downloadable artefacts; firmware.elf stays internal (backtrace decoding).
	servedArtifacts = map[string]bool{ArtifactMergedBin: true, ArtifactFirmwareBin: true, ArtifactResultJSON: true, ArtifactBuildLog: true}
	binaryArtifacts = []string{ArtifactMergedBin, ArtifactFirmwareBin, ArtifactFirmwareELF}
	// fontArtifact matches the files cyd-build copies to out/fonts/.
	fontArtifact = regexp.MustCompile(`^(ui_font_[a-z0-9_]{1,48}\.c|fonts\.json)$`)
)

// FontArtifactName reports whether name is a file cyd-build copies to
// out/fonts/: a converted font "ui_font_<family>_<size>.c" or "fonts.json".
func FontArtifactName(name string) bool { return fontArtifact.MatchString(name) }

// validBuildID reports an id the service could have issued: "ccb_" plus
// element id characters, so no separator and no dot. Every id is checked
// before it becomes part of an artefact path.
func validBuildID(id string) bool {
	return strings.HasPrefix(id, buildIDPrefix) && validElementID(id)
}

func buildNotFound(id string) error {
	return NewError(CodeNotFound, "build not found", map[string]any{"build_id": clipEcho(id)})
}

// artifactsRoot opens <data>/cyd_creator/artifacts as an os.Root, so no
// artefact path can leave it; create makes the directory first (writers).
func (s *Service) artifactsRoot(create bool) (*os.Root, error) {
	if create {
		if err := os.MkdirAll(s.artifactsDir, 0o750); err != nil {
			return nil, fmt.Errorf("create artefact directory: %w", err)
		}
	}
	return os.OpenRoot(s.artifactsDir)
}

// readArtifact reads the regular file <build_id>/<rel> below the artefact
// directory, at most max bytes. Callers validate buildID first.
func (s *Service) readArtifact(buildID, rel string, max int64) ([]byte, error) {
	root, err := s.artifactsRoot(false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return rootReadFile(root, path.Join(buildID, rel), max)
}

// StartBuild generates the tree now (a snapshot of the current version) and
// queues the build. The single worker runs builds FIFO; manual and agent
// builds share the queue.
func (s *Service) StartBuild(ctx context.Context, projectID string, opts BuildOptions) (Build, error) {
	origin := opts.Origin
	if origin == "" {
		origin = OriginUser
	}
	if origin != OriginUser && jobIDOf(origin) == "" || !validOrigin(origin) {
		return Build{}, NewError(CodeBadRequest, fmt.Sprintf("invalid build origin %q", clipEcho(origin)), nil)
	}
	if s.builder == nil {
		return Build{}, NewError(CodeDockerUnavailable, "the build container is not available", nil)
	}
	unlock := s.lockProject(projectID)
	defer unlock()
	row, err := s.loadProjectRow(ctx, projectID)
	if err != nil {
		return Build{}, s.internalError("start build", err)
	}
	if err := checkLock(row, origin); err != nil {
		return Build{}, err
	}
	status := s.builder.Status(ctx)
	if !status.DockerAvailable {
		return Build{}, NewError(CodeDockerUnavailable, "Docker is not available", map[string]any{"error": status.Error})
	}
	if !status.Present || status.Pulling {
		return Build{}, NewError(CodeImageMissing, "the toolchain image is not installed yet", map[string]any{"image": status.Image})
	}
	// The project directory (a missing one is CYD_INVALID_LAYOUT) or the job's
	// staging copy. Typed tree errors (invalid layout, oversized or broken
	// asset entries) and typed generator refusals reach the caller unchanged.
	var root *os.Root
	if opts.Dir == "" {
		root, err = s.openProjectRoot(row)
	} else {
		root, err = s.openRoot(row, opts.Dir)
	}
	if err != nil {
		return Build{}, s.internalError("start build", err)
	}
	defer root.Close()
	in, err := loadGenerateInput(row, root)
	if err != nil {
		return Build{}, s.internalError("start build", err)
	}
	out, err := s.runGenerate(in)
	if err != nil {
		return Build{}, err
	}
	board, err := BoardByID(in.Project.Board)
	if err != nil {
		return Build{}, err
	}
	b := Build{ID: s.ids.NewID("ccb"), ProjectID: projectID, JobID: jobIDOf(origin), Origin: BuildOriginUser,
		Status: BuildQueued, Board: board.ID, ProjectVersion: row.Version, Diagnostics: []Diagnostic{},
		Artifacts: []ArtifactInfo{}, CreatedAt: formatTime(s.now())}
	if b.JobID != "" {
		b.Origin = BuildOriginAgent
	}
	if !validBuildID(b.ID) {
		return Build{}, s.internalError("start build", fmt.Errorf("the id source issued the invalid build id %q", b.ID))
	}
	s.buildsMu.Lock()
	if s.ctx.Err() != nil {
		s.buildsMu.Unlock()
		return Build{}, NewError(CodeBusy, "the build service is shutting down", nil)
	}
	if len(s.queue) >= maxQueuedBuilds {
		s.buildsMu.Unlock()
		return Build{}, NewError(CodeBusy, "too many builds are waiting", map[string]any{"queued": len(s.queue)})
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO cc_builds(id,project_id,job_id,origin,status,board,project_version,created_at)
		VALUES(?,?,?,?,?,?,?,?)`, b.ID, b.ProjectID, b.JobID, b.Origin, b.Status, b.Board, b.ProjectVersion, b.CreatedAt); err != nil {
		s.buildsMu.Unlock()
		return Build{}, s.internalError("start build", err)
	}
	s.queue = append(s.queue, &queuedBuild{build: b, tree: out.Files, env: board.PIOEnv(), cacheKey: projectID + "-" + board.ID})
	s.waiters[b.ID] = make(chan struct{})
	// Announced before the worker can take the build, so "queued" always
	// precedes "running" (lock order buildsMu → eventsMu; no transaction or
	// open rows here).
	s.Emit(projectID, EventBuildStatus, b.JobID, b.ID, map[string]any{"build": b})
	s.buildsMu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return b, nil
}

// buildWorker runs queued builds one at a time until Close. Builds still
// queued at shutdown end as interrupted.
func (s *Service) buildWorker() {
	defer s.wg.Done()
	for {
		rb := s.nextBuild()
		if rb == nil {
			break
		}
		s.runBuild(rb)
	}
	s.buildsMu.Lock()
	queued := s.queue
	s.queue = nil
	s.buildsMu.Unlock()
	for _, qb := range queued {
		b := qb.build
		b.Status, b.ErrorCode, b.FinishedAt = BuildInterrupted, CodeBuildInterrupted, formatTime(s.now())
		s.finishBuild(b)
	}
}

func (s *Service) nextBuild() *runningBuild {
	for {
		s.buildsMu.Lock()
		if s.ctx.Err() != nil {
			s.buildsMu.Unlock()
			return nil
		}
		if len(s.queue) > 0 {
			qb := s.queue[0]
			s.queue = s.queue[1:]
			ctx, cancel := context.WithCancelCause(s.ctx)
			s.running = &runningBuild{queued: qb, ctx: ctx, cancel: cancel, log: &buildLog{}}
			rb := s.running
			s.buildsMu.Unlock()
			return rb
		}
		s.buildsMu.Unlock()
		select {
		case <-s.wake:
		case <-s.ctx.Done():
			return nil
		}
	}
}

func (s *Service) hasSucceededBuild(projectID, board string) bool {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM cc_builds WHERE project_id=? AND board=? AND status=? LIMIT 1`,
		projectID, board, BuildSucceeded).Scan(&one)
	return err == nil
}

// runBuild executes one build: cold timeout when no build of this
// (project, board) ever succeeded, throttled build_log events, result
// classification, artefact storage and retention.
func (s *Service) runBuild(rb *runningBuild) {
	defer rb.cancel(nil)
	b := rb.queued.build
	limits := s.Limits()
	timeout := limits.BuildTimeout
	if !s.hasSucceededBuild(b.ProjectID, b.Board) {
		timeout = limits.ColdBuildTimeout
	}
	started := s.now()
	b.Status, b.StartedAt = BuildRunning, formatTime(started)
	if _, err := s.db.Exec(`UPDATE cc_builds SET status=?, started_at=? WHERE id=?`, b.Status, b.StartedAt, b.ID); err != nil {
		s.logger.Warn("CYD Creator build state could not be stored", "build_id", b.ID, "error", err)
	}
	s.Emit(b.ProjectID, EventBuildStatus, b.JobID, b.ID, map[string]any{"build": b})

	throttle := newLogThrottle(buildLogInterval, func(lines []string) {
		s.EmitEphemeral(b.ProjectID, EventBuildLog, b.JobID, b.ID, map[string]any{"build_id": b.ID, "lines": lines})
	})
	// stop is idempotent: the call after Build flushes the last lines before
	// the final build_status; the deferred one ends the goroutine on any other
	// way out (a panicking Builder).
	defer throttle.stop()
	buildCtx, cancelTimeout := context.WithTimeout(rb.ctx, timeout)
	defer cancelTimeout()
	out, err := s.builder.Build(buildCtx, BuildRequest{BuildID: b.ID, Env: rb.queued.env, CacheKey: rb.queued.cacheKey,
		Tree: rb.queued.tree, Timeout: timeout}, func(line string) {
		rb.log.add(line)
		throttle.add(line)
	})
	timedOut := errors.Is(buildCtx.Err(), context.DeadlineExceeded)
	cancelTimeout()
	throttle.stop()
	rb.queued.tree = nil

	logText := out.Log
	if logText == "" {
		logText = rb.log.String()
	}
	logText = clipBuildLog(logText)
	elapsed := s.now().Sub(started)
	b = s.classifyBuild(b, out, err, errors.Is(context.Cause(rb.ctx), errBuildCancelled), timedOut, logText, elapsed)
	if b.Status != BuildInterrupted {
		artifacts, aerr := s.storeArtifacts(b, out, logText)
		if aerr != nil {
			s.logger.Warn("CYD Creator artefacts could not be stored", "build_id", b.ID, "error", aerr)
			b.Status, b.ErrorCode = BuildFailed, CodeBuildFailed
			b.Diagnostics = append(b.Diagnostics, Diagnostic{Severity: SeverityNote,
				Message: "The build artefacts could not be stored: " + aerr.Error()})
			artifacts, _ = s.storeArtifacts(b, BuildOutput{Result: out.Result}, logText)
		}
		b.Artifacts = artifacts
	}
	b.FinishedAt = formatTime(s.now())
	s.finishBuild(b)
}

// clipBuildLog keeps a stored log within MaxBuildLogBytes (the Builder caps
// it already; BuildLog reads at most that much).
func clipBuildLog(text string) string {
	if len(text) <= MaxBuildLogBytes {
		return text
	}
	cut := MaxBuildLogBytes - len(buildLogTruncated) - 1
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "\n" + buildLogTruncated
}

// classifyBuild maps the builder outcome to status, error code, diagnostics
// and usage (overview "Error codes"; spec §10).
func (s *Service) classifyBuild(b Build, out BuildOutput, err error, cancelled, timedOut bool, logText string, elapsed time.Duration) Build {
	switch {
	case cancelled:
		b.Status, b.ErrorCode = BuildCancelled, CodeBuildCancelled
		return b
	case err != nil && s.ctx.Err() != nil:
		b.Status, b.ErrorCode = BuildInterrupted, CodeBuildInterrupted
		return b
	case timedOut:
		b.Status, b.ErrorCode = BuildFailed, CodeBuildTimeout
		b.Diagnostics = ParseDiagnostics(logText, "/work/"+b.ID)
		return b
	case err != nil:
		note := "The build container failed; the server log has the details."
		var typed *Error
		if errors.As(err, &typed) {
			b.ErrorCode, note = typed.Code, typed.Message
		} else {
			s.logger.Warn("CYD Creator build failed in the builder", "build_id", b.ID, "error", err)
			b.ErrorCode = CodeBuildFailed
		}
		b.Status = BuildFailed
		// The builder failed (e.g. the artefact download after an OOM-killed
		// compiler): errors of the partial log are not agent-repairable.
		b.Diagnostics = append(demoteErrors(ParseDiagnostics(logText, "/work/"+b.ID)), Diagnostic{Severity: SeverityNote, Message: note})
		return b
	}
	b.Diagnostics = ParseDiagnostics(logText, "/work/"+b.ID)
	b.Usage = BuildUsage{FlashUsed: out.Result.Flash.Used, FlashTotal: out.Result.Flash.Total, RAMUsed: out.Result.RAM.Used,
		RAMTotal: out.Result.RAM.Total, DurationMS: out.Result.DurationMS, Warnings: out.Result.Warnings}
	if b.Usage.DurationMS == 0 {
		b.Usage.DurationMS = elapsed.Milliseconds()
	}
	if b.Usage.Warnings == 0 {
		for _, d := range b.Diagnostics {
			if d.Severity == SeverityWarning {
				b.Usage.Warnings++
			}
		}
	}
	switch out.ExitCode {
	case BuildExitOK:
		for _, name := range binaryArtifacts {
			if len(out.Files[name]) == 0 {
				b.Status, b.ErrorCode = BuildFailed, CodeBuildFailed
				b.Diagnostics = append(b.Diagnostics, Diagnostic{Severity: SeverityNote,
					Message: "The build finished without " + name + "."})
				return b
			}
		}
		b.Status = BuildSucceeded
	case BuildExitCompileError:
		b.Status, b.ErrorCode = BuildFailed, CodeBuildFailed
		if generatorFault(b.Diagnostics) {
			b.ErrorCode = CodeGeneratorError
		} else if !hasErrorDiagnostic(b.Diagnostics) {
			// e.g. "region ... overflowed": cyd-build saw a linker failure the
			// parser cannot attribute to a file.
			b.Diagnostics = append(b.Diagnostics, Diagnostic{Severity: SeverityNote,
				Message: "The compiler or linker failed without a diagnostic that names a file; " + ArtifactBuildLog + " has the details."})
		}
	case BuildExitToolingError:
		b.Status, b.ErrorCode = BuildFailed, CodeBuildFailed
		b.Diagnostics = append(demoteErrors(b.Diagnostics), Diagnostic{Severity: SeverityNote,
			Message: "The build toolchain failed (cyd-build exit code 2); this is not an error in the project code."})
	case BuildExitFlashExceeded:
		b.Status, b.ErrorCode = BuildFailed, CodeFlashBudgetExceeded
	default:
		b.Status, b.ErrorCode = BuildFailed, CodeBuildFailed
		b.Diagnostics = append(demoteErrors(b.Diagnostics), Diagnostic{Severity: SeverityNote,
			Message: fmt.Sprintf("cyd-build ended with exit code %d.", out.ExitCode)})
	}
	return b
}

// demoteErrors turns error diagnostics into notes. cyd-build exit code 2 (a
// compiler killed by the OOM killer, a tooling or build-tree failure),
// unknown exit codes and builder failures are never agent-repairable, but
// plan 3 repairs a CYD_BUILD_FAILED build that carries error diagnostics; the
// lines stay visible as notes.
func demoteErrors(diags []Diagnostic) []Diagnostic {
	for i := range diags {
		if diags[i].Severity == SeverityError {
			diags[i].Severity = SeverityNote
		}
	}
	return diags
}

func hasErrorDiagnostic(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// generatorFault reports a compile failure caused by generated or template
// code: at least one error in a generated path and none in an editable app
// path (app errors are actionable and win).
func generatorFault(diags []Diagnostic) bool {
	generated, app := false, false
	for _, d := range diags {
		if d.Severity != SeverityError {
			continue
		}
		if d.Generated {
			generated = true
		}
		if IsEditablePath(d.File) {
			app = true
		}
	}
	return generated && !app
}

// storeArtifacts writes the artefacts of a finished build into
// <data>/cyd_creator/artifacts/<build_id>/ (staged in "<id>.tmp", then
// renamed), all through an os.Root of the artefact directory. Binaries are
// kept only for succeeded builds and verified against result.json; build.log
// is kept for every finished build.
func (s *Service) storeArtifacts(b Build, out BuildOutput, logText string) ([]ArtifactInfo, error) {
	if !validBuildID(b.ID) {
		return nil, fmt.Errorf("invalid build id %q", clipEcho(b.ID))
	}
	root, err := s.artifactsRoot(true)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	final := b.ID
	tmp := final + ".tmp"
	_ = root.RemoveAll(tmp)
	if err := root.Mkdir(tmp, 0o750); err != nil {
		return nil, err
	}
	artifacts := []ArtifactInfo{}
	write := func(name string, data []byte, listed bool) error {
		if err := root.WriteFile(filepath.Join(tmp, name), data, 0o640); err != nil {
			return err
		}
		if listed {
			artifacts = append(artifacts, ArtifactInfo{Name: name, Size: int64(len(data)), SHA256: sha256Hex(data)})
		}
		return nil
	}
	fail := func(err error) ([]ArtifactInfo, error) {
		_ = root.RemoveAll(tmp)
		return nil, err
	}
	if b.Status == BuildSucceeded {
		for _, name := range binaryArtifacts {
			data := out.Files[name]
			if want := out.Result.Files[name].SHA256; want != "" && want != sha256Hex(data) {
				return fail(fmt.Errorf("%s does not match the checksum in result.json", name))
			}
			if err := write(name, data, name != ArtifactFirmwareELF); err != nil {
				return fail(err)
			}
		}
		if err := storeFonts(root, tmp, out); err != nil {
			return fail(err)
		}
	}
	if out.Result.Env != "" {
		data, err := json.MarshalIndent(out.Result, "", "  ")
		if err != nil {
			return fail(err)
		}
		if err := write(ArtifactResultJSON, append(data, '\n'), true); err != nil {
			return fail(err)
		}
	}
	if err := write(ArtifactBuildLog, []byte(logText), true); err != nil {
		return fail(err)
	}
	_ = root.RemoveAll(final)
	// The build is finished: publish even while the service is closing.
	if err := fileutil.RenameRootContext(context.Background(), root, tmp, final); err != nil {
		return fail(err)
	}
	return artifacts, nil
}

// storeFonts keeps the converted fonts and fonts.json of a succeeded build in
// <dir>/fonts/ below root, verified against result.json. They are not listed
// as downloadable artefacts; BuildFonts reads them for the ZIP export.
func storeFonts(root *os.Root, dir string, out BuildOutput) error {
	if len(out.Result.Fonts) == 0 {
		return nil
	}
	fontsDir := filepath.Join(dir, ArtifactFontsDir)
	if err := root.Mkdir(fontsDir, 0o750); err != nil {
		return err
	}
	for name, want := range out.Result.Fonts {
		if !FontArtifactName(name) {
			return fmt.Errorf("unexpected font artefact %q", clipEcho(name))
		}
		data, ok := out.Files[ArtifactFontsDir+"/"+name]
		if !ok || want.SHA256 != sha256Hex(data) {
			return fmt.Errorf("font artefact %s does not match the checksum in result.json", name)
		}
		if len(data) > maxFontArtifactBytes {
			return fmt.Errorf("font artefact %s is larger than %d bytes", name, maxFontArtifactBytes)
		}
		if err := root.WriteFile(filepath.Join(fontsDir, name), data, 0o640); err != nil {
			return err
		}
	}
	return nil
}

// finishBuild stores a terminal build, points the project's last_build at
// it, applies the retention, emits build_status and releases WaitBuild
// callers (in this order, so a released caller sees the final state).
func (s *Service) finishBuild(b Build) {
	if b.Diagnostics == nil {
		b.Diagnostics = []Diagnostic{}
	}
	if b.Artifacts == nil {
		b.Artifacts = []ArtifactInfo{}
	}
	if _, err := s.db.Exec(`UPDATE cc_builds SET status=?, error_code=?, diagnostics=?, usage=?, artifacts=?, started_at=?, finished_at=?
		WHERE id=?`, b.Status, b.ErrorCode, mustJSON(b.Diagnostics), mustJSON(b.Usage), mustJSON(b.Artifacts), b.StartedAt, b.FinishedAt, b.ID); err != nil {
		s.logger.Warn("CYD Creator build result could not be stored", "build_id", b.ID, "error", err)
	}
	if _, err := s.db.Exec(`UPDATE cc_projects SET last_build=? WHERE id=?`, b.ID, b.ProjectID); err != nil {
		s.logger.Warn("CYD Creator last build could not be stored", "build_id", b.ID, "error", err)
	}
	s.applyRetention(b.ProjectID)
	s.Emit(b.ProjectID, EventBuildStatus, b.JobID, b.ID, map[string]any{"build": b})
	s.buildsMu.Lock()
	if ch := s.waiters[b.ID]; ch != nil {
		close(ch)
		delete(s.waiters, b.ID)
	}
	if s.running != nil && s.running.queued.build.ID == b.ID {
		s.running = nil
	}
	s.buildsMu.Unlock()
}

// applyRetention keeps the newest KeepBuildsPerProject finished builds of a
// project and deletes older rows and artefacts, except builds a device still
// runs or targets, the project's last_build and its newest succeeded build
// (so failed builds never prune the only good firmware that exports and OTA
// use). It also drops events of deleted projects: an Emit that raced
// DeleteProject can store one after the delete committed (the statement
// recoverStore runs at startup).
func (s *Service) applyRetention(projectID string) {
	if _, err := s.db.Exec(`DELETE FROM cc_events WHERE project_id NOT IN (SELECT id FROM cc_projects)`); err != nil {
		s.logger.Warn("CYD Creator orphaned events could not be dropped", "error", err)
	}
	keep := s.Limits().KeepBuildsPerProject
	rows, err := s.db.Query(`SELECT b.id FROM cc_builds b WHERE b.project_id=? AND b.status NOT IN ('queued','running')
		AND b.id NOT IN (SELECT build_id FROM cc_devices WHERE project_id=? UNION SELECT target_build_id FROM cc_devices WHERE project_id=?)
		AND b.id <> (SELECT last_build FROM cc_projects WHERE id=?)
		AND b.id <> COALESCE((SELECT id FROM cc_builds WHERE project_id=? AND status=? ORDER BY created_at DESC, id DESC LIMIT 1), '')
		ORDER BY b.created_at DESC, b.id DESC`, projectID, projectID, projectID, projectID, projectID, BuildSucceeded)
	if err != nil {
		s.logger.Warn("CYD Creator build retention failed", "project_id", projectID, "error", err)
		return
	}
	var stale []string
	n := 0
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			continue
		}
		n++
		if n > keep-1 { // last_build is excluded above and always kept
			stale = append(stale, id)
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		s.logger.Warn("CYD Creator build retention failed", "project_id", projectID, "error", err)
		return
	}
	if len(stale) == 0 {
		return
	}
	args := make([]any, len(stale))
	for i, id := range stale {
		args[i] = id
	}
	if _, err := s.db.Exec(`DELETE FROM cc_builds WHERE id IN (`+sqlPlaceholders(len(stale))+`)`, args...); err != nil {
		s.logger.Warn("CYD Creator old builds could not be deleted", "project_id", projectID, "error", err)
		return
	}
	s.removeBuildArtifacts(stale)
}

// removeBuildArtifacts removes the artefact directories of builds through
// the artefact root. An id that fails validBuildID (a damaged row) is skipped
// with a warning, so "" or "../x" never reaches RemoveAll; a directory whose
// removal fails is swept at the next start (discardPartialArtifacts).
func (s *Service) removeBuildArtifacts(ids []string) {
	var root *os.Root
	defer func() {
		if root != nil {
			_ = root.Close()
		}
	}()
	for _, id := range ids {
		if !validBuildID(id) {
			s.logger.Warn("CYD Creator build id is malformed; its artefacts are left alone", "build_id", clipEcho(id))
			continue
		}
		if root == nil {
			var err error
			if root, err = s.artifactsRoot(false); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					s.logger.Warn("CYD Creator artefacts could not be removed", "error", err)
				}
				return
			}
		}
		if err := root.RemoveAll(id); err != nil {
			s.logger.Warn("CYD Creator artefacts of a build could not be removed", "build_id", id, "error", err)
		}
	}
}

// discardPartialArtifacts runs at startup, through the artefact root: it
// removes the artefact directories of interrupted builds, staging
// directories ("<id>.tmp") left by a crash and directories named like a build
// that has no cc_builds row any more (a removal that failed, e.g. while a file
// was open on Windows). Other entries are left alone.
func (s *Service) discardPartialArtifacts(recovered []recoveredBuild) {
	ids := make([]string, 0, len(recovered))
	for _, b := range recovered {
		ids = append(ids, b.ID)
	}
	s.removeBuildArtifacts(ids)
	known, err := s.allBuildIDs()
	if err != nil {
		s.logger.Warn("CYD Creator builds could not be listed; orphaned artefacts are kept", "error", err)
	}
	root, err := s.artifactsRoot(false)
	if err != nil {
		return
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		s.logger.Warn("CYD Creator artefact directory could not be listed", "error", err)
		return
	}
	for _, e := range entries {
		name := e.Name()
		orphan := known != nil && e.IsDir() && validBuildID(name) && !known[name]
		if !strings.HasSuffix(name, ".tmp") && !orphan {
			continue
		}
		if err := root.RemoveAll(name); err != nil {
			s.logger.Warn("CYD Creator stale artefacts could not be removed", "name", name, "error", err)
		}
	}
}

// allBuildIDs returns the id of every stored build.
func (s *Service) allBuildIDs() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT id FROM cc_builds`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// activeBuilds counts queued and running builds.
func (s *Service) activeBuilds() int {
	s.buildsMu.Lock()
	defer s.buildsMu.Unlock()
	n := len(s.queue)
	if s.running != nil {
		n++
	}
	return n
}

// CancelBuild cancels a queued build at once or stops a running one (the
// worker records it as cancelled). Finished builds are left alone.
func (s *Service) CancelBuild(ctx context.Context, buildID string) error {
	s.buildsMu.Lock()
	if s.running != nil && s.running.queued.build.ID == buildID {
		s.running.cancel(errBuildCancelled)
		s.buildsMu.Unlock()
		return nil
	}
	for i, qb := range s.queue {
		if qb.build.ID != buildID {
			continue
		}
		s.queue = append(s.queue[:i], s.queue[i+1:]...)
		s.buildsMu.Unlock()
		b := qb.build
		b.Status, b.ErrorCode, b.FinishedAt = BuildCancelled, CodeBuildCancelled, formatTime(s.now())
		s.finishBuild(b)
		return nil
	}
	s.buildsMu.Unlock()
	_, err := s.loadBuild(ctx, buildID)
	return s.internalError("cancel build", err)
}

// GetBuild returns one build.
func (s *Service) GetBuild(ctx context.Context, buildID string) (Build, error) {
	b, err := s.loadBuild(ctx, buildID)
	return b, s.internalError("read build", err)
}

// ListBuilds returns the project's builds, newest first.
func (s *Service) ListBuilds(ctx context.Context, projectID string) ([]Build, error) {
	if _, err := s.loadProjectRow(ctx, projectID); err != nil {
		return nil, s.internalError("list builds", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+buildColumns+` FROM cc_builds WHERE project_id=? ORDER BY created_at DESC, id DESC`, projectID)
	if err != nil {
		return nil, s.internalError("list builds", err)
	}
	defer rows.Close()
	out := []Build{}
	for rows.Next() {
		b, err := scanBuild(rows)
		if err != nil {
			return nil, s.internalError("list builds", err)
		}
		out = append(out, b)
	}
	return out, s.internalError("list builds", rows.Err())
}

// WaitBuild blocks until the build is finished (or ctx ends) and returns it.
func (s *Service) WaitBuild(ctx context.Context, buildID string) (Build, error) {
	s.buildsMu.Lock()
	ch := s.waiters[buildID]
	s.buildsMu.Unlock()
	if ch != nil {
		select {
		case <-ch:
		case <-ctx.Done():
			return Build{}, ctx.Err()
		}
	}
	return s.GetBuild(ctx, buildID)
}

// BuildLog returns the log so far (running build) or the stored build.log.
func (s *Service) BuildLog(ctx context.Context, buildID string) (string, error) {
	if !validBuildID(buildID) {
		return "", buildNotFound(buildID)
	}
	s.buildsMu.Lock()
	if s.running != nil && s.running.queued.build.ID == buildID {
		text := s.running.log.String()
		s.buildsMu.Unlock()
		return text, nil
	}
	s.buildsMu.Unlock()
	if _, err := s.loadBuild(ctx, buildID); err != nil {
		return "", s.internalError("read build log", err)
	}
	data, err := s.readArtifact(buildID, ArtifactBuildLog, MaxBuildLogBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", s.internalError("read build log", err)
	}
	return string(data), nil
}

// OpenArtifact opens merged.bin, firmware.bin (succeeded builds), result.json
// or build.log. The caller closes the reader.
func (s *Service) OpenArtifact(ctx context.Context, buildID, name string) (io.ReadSeekCloser, ArtifactInfo, error) {
	notFound := NewError(CodeNotFound, "artefact not found", map[string]any{"build_id": clipEcho(buildID), "name": clipEcho(name)})
	if !servedArtifacts[name] || !validBuildID(buildID) {
		return nil, ArtifactInfo{}, notFound
	}
	b, err := s.loadBuild(ctx, buildID)
	if err != nil {
		return nil, ArtifactInfo{}, s.internalError("open artefact", err)
	}
	var info ArtifactInfo
	for _, a := range b.Artifacts {
		if a.Name == name {
			info = a
		}
	}
	if info.Name == "" {
		return nil, ArtifactInfo{}, notFound
	}
	root, err := s.artifactsRoot(false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ArtifactInfo{}, notFound
	}
	if err != nil {
		return nil, ArtifactInfo{}, s.internalError("open artefact", err)
	}
	defer root.Close() // the opened file stays usable
	f, err := root.Open(filepath.Join(buildID, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ArtifactInfo{}, notFound
	}
	if err != nil {
		return nil, ArtifactInfo{}, s.internalError("open artefact", err)
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		_ = f.Close()
		if err != nil {
			return nil, ArtifactInfo{}, s.internalError("open artefact", err)
		}
		return nil, ArtifactInfo{}, notFound
	}
	return f, info, nil
}

// BuildFonts returns the converted fonts a succeeded build kept (keyed
// "<symbol>.c") and the SHA-256 of the fonts.json they were made from. A build
// that kept no fonts (failed, or older than this feature) returns "", nil, nil.
func (s *Service) BuildFonts(ctx context.Context, buildID string) (string, map[string][]byte, error) {
	if !validBuildID(buildID) {
		return "", nil, buildNotFound(buildID)
	}
	b, err := s.loadBuild(ctx, buildID)
	if err != nil {
		return "", nil, s.internalError("read build fonts", err)
	}
	if b.Status != BuildSucceeded {
		return "", nil, nil
	}
	root, err := s.artifactsRoot(false)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, s.internalError("read build fonts", err)
	}
	defer root.Close()
	dir := path.Join(buildID, ArtifactFontsDir)
	entries, err := fs.ReadDir(root.FS(), dir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, s.internalError("read build fonts", err)
	}
	fontsSHA := ""
	files := map[string][]byte{}
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !FontArtifactName(name) {
			continue
		}
		data, err := rootReadFile(root, path.Join(dir, name), maxFontArtifactBytes)
		if err != nil {
			return "", nil, s.internalError("read build fonts", err)
		}
		if name == "fonts.json" {
			fontsSHA = sha256Hex(data)
			continue
		}
		files[name] = data
	}
	if fontsSHA == "" {
		return "", nil, nil
	}
	return fontsSHA, files, nil
}

// DecodeBacktrace decodes up to 64 addresses against the build's firmware.elf.
func (s *Service) DecodeBacktrace(ctx context.Context, buildID string, addrs []string) ([]Frame, error) {
	if len(addrs) == 0 || len(addrs) > maxBacktrace {
		return nil, NewError(CodeBadRequest, fmt.Sprintf("send 1 to %d addresses", maxBacktrace), nil)
	}
	for _, a := range addrs {
		if !backtraceAddress.MatchString(a) {
			return nil, NewError(CodeBadRequest, fmt.Sprintf("invalid address %q", clipEcho(a)), map[string]any{"address": clipEcho(a)})
		}
	}
	if !validBuildID(buildID) {
		return nil, buildNotFound(buildID)
	}
	b, err := s.loadBuild(ctx, buildID)
	if err != nil {
		return nil, s.internalError("decode backtrace", err)
	}
	noImage := NewError(CodeNotFound, "this build has no firmware image", map[string]any{"build_id": buildID})
	if b.Status != BuildSucceeded {
		return nil, noImage
	}
	board, err := BoardByID(b.Board)
	if err != nil {
		return nil, err
	}
	elf, err := s.readArtifact(buildID, ArtifactFirmwareELF, maxELFBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, noImage
	}
	if err != nil {
		return nil, s.internalError("decode backtrace", err)
	}
	if s.builder == nil {
		return nil, NewError(CodeDockerUnavailable, "the build container is not available", nil)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	frames, err := s.builder.Addr2Line(ctx, Addr2LineRequest{BuildID: buildID, Chip: board.Chip, ELF: elf, Addresses: addrs})
	if err != nil {
		return nil, s.internalError("decode backtrace", err)
	}
	if frames == nil {
		frames = []Frame{}
	}
	return frames, nil
}

// PullToolchain starts an image pull in the background; progress is read
// through Capabilities. A pull that is already running is not started twice.
// The pull runs on the service context and Close waits for it.
func (s *Service) PullToolchain(ctx context.Context) error {
	if s.builder == nil {
		return NewError(CodeDockerUnavailable, "the build container is not available", nil)
	}
	if status := s.builder.Status(ctx); !status.DockerAvailable {
		return NewError(CodeDockerUnavailable, "Docker is not available", map[string]any{"error": status.Error})
	}
	// Under buildsMu with the context still live, the build worker holds the
	// WaitGroup above zero (it leaves only after the shutdown drain, which
	// takes buildsMu), so this Add never races Close's Wait.
	s.buildsMu.Lock()
	if s.ctx.Err() != nil {
		s.buildsMu.Unlock()
		return NewError(CodeBusy, "the build service is shutting down", nil)
	}
	if !s.pulling.CompareAndSwap(false, true) {
		s.buildsMu.Unlock()
		return nil
	}
	s.wg.Add(1)
	s.buildsMu.Unlock()
	go func() {
		defer s.wg.Done()
		defer s.pulling.Store(false)
		if err := s.builder.Pull(s.ctx); err != nil && s.ctx.Err() == nil {
			s.logger.Warn("CYD Creator toolchain image could not be pulled", "error", err)
		}
	}()
	return nil
}
