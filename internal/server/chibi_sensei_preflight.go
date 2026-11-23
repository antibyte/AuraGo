package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/chibisensei/trainer"
	"aurago/internal/config"
)

// errChibiProfileUnknown: the project's profile (or its suggestion) is not in the catalog.
var errChibiProfileUnknown = errors.New("chibi sensei: profile unknown")

// chibiTrainingState is what the preflight, prepare and start compute from project, config and Kaggle.
type chibiTrainingState struct {
	proj    chibisensei.Project
	prof    *chibisensei.Profile
	profSHA string
	plan    chibisensei.Plan
	split   chibisensei.SplitResult
	in      chibisensei.PreflightInput
	pf      chibisensei.Preflight
	checked time.Time
}

// chibiNeedBaseGGUF reports whether a run of prof must also convert the base model: true unless base_gguf holds the
// GGUF of the profile's base revision made by the pinned converter. A failed lookup counts as needed (the larger
// estimate).
func chibiNeedBaseGGUF(ctx context.Context, st *chibisensei.Store, prof *chibisensei.Profile) bool {
	lock, err := trainer.LoadLock()
	if err != nil {
		return true
	}
	_, found, err := st.GetBaseGGUF(ctx, prof.ID, prof.Base.Revision, chibisensei.BaseGGUFConverter(lock.LlamaCppTag))
	return err != nil || !found
}

// chibiValidPreset accepts "" (the project's preset) and the three preset names; otherwise it answers 422.
func chibiValidPreset(w http.ResponseWriter, preset string) bool {
	switch preset {
	case "", "quick", "balanced", "thorough":
		return true
	}
	chibiValidation(w, "preset", "project.preset", "Unknown preset.")
	return false
}

// chibiTrainingState loads the project, computes the plan and collects and evaluates the preflight. fresh bypasses the
// cache of the Kaggle reads (prepare, start).
func (s *Server) chibiTrainingState(ctx context.Context, r *http.Request, acc *chibiSenseiAccess, projectID, preset string,
	fresh bool) (*chibiTrainingState, error) {
	rows, proj, err := acc.rt.store.LoadSplitRows(ctx, projectID)
	if err != nil {
		return nil, err
	}
	prof, ok := chibiProfileOf(ctx, acc, proj)
	if !ok {
		return nil, errChibiProfileUnknown
	}
	split := chibiSplitOf(rows, proj)
	plan, err := chibisensei.ComputePlan(chibisensei.PlanInput{Project: proj, Profile: prof, Rows: rows, Split: split,
		Preset: preset, RemoteTimeoutMinutes: acc.cfg.Kaggle.RemoteTimeoutMinutes,
		NeedBaseGGUF: chibiNeedBaseGGUF(ctx, acc.rt.store, prof)})
	if err != nil {
		return nil, err
	}
	in, checked, err := s.chibiPreflightInput(ctx, r, acc, proj, prof, plan, fresh)
	if err != nil {
		return nil, err
	}
	return &chibiTrainingState{proj: proj, prof: prof, profSHA: acc.rt.catalog.SHA(prof.ID), plan: plan, split: split,
		in: in, pf: chibisensei.EvaluatePreflight(in), checked: checked}, nil
}

// planHash binds a prepared start to the plan, the profile, the script, the Kaggle account and the training settings.
func (t *chibiTrainingState) planHash(cfg *config.Config) string {
	tc := cfg.ChibiSensei.Training
	return chibisensei.PlanHash(t.plan, t.profSHA, trainer.ScriptSHA256(), t.in.Account, tc.MachineShape, tc.DeleteRemoteAfterRun)
}

func chibiTrainingError(w http.ResponseWriter, err error) {
	if errors.Is(err, errChibiProfileUnknown) {
		chibiProfileUnknown(w)
		return
	}
	chibiDomainError(w, err)
}

// chibiPreflightInput collects the plain values of the preflight. The Kaggle reads run only when every earlier Kaggle
// and account item passes (KaggleReachable), so a disabled integration is never contacted or opened.
func (s *Server) chibiPreflightInput(ctx context.Context, r *http.Request, acc *chibiSenseiAccess, proj chibisensei.Project,
	prof *chibisensei.Profile, plan chibisensei.Plan, fresh bool) (chibisensei.PreflightInput, time.Time, error) {
	st := acc.rt.store
	in := chibisensei.PreflightInput{DataRev: proj.DataRev, Admin: chibiRequestIsAdmin(s, r)}
	res, _, err := st.GetChecks(ctx, proj.ID)
	if err != nil {
		return in, time.Time{}, err
	}
	if res != nil {
		in.CheckPresent, in.CheckDataRev, in.CheckReady = true, res.DataRev, res.Ready
	}
	active, err := st.ActiveRuns(ctx)
	if err != nil {
		return in, time.Time{}, err
	}
	if len(active) > 0 {
		in.ActiveRunID, in.ActiveRunProjectID = active[0].ID, active[0].ProjectID
	}

	snap := s.kaggleSnapshot()
	k := snap.Config
	in.KaggleEnabled, in.KaggleReadOnly = k.Enabled, k.ReadOnly
	in.AllowCreate, in.AllowRemoteExecute, in.AllowGPU = k.AllowCreate, k.AllowRemoteExecute, k.AllowGPU
	in.AllowInternet, in.AllowDownload, in.AllowDelete = k.AllowInternet, k.AllowDownload, k.AllowDelete
	in.AllowedMachineShapes = append([]string(nil), k.AllowedMachineShapes...)
	in.RemoteTimeoutMinutes, in.MaxTransferMB, in.MaxStorageMB = k.RemoteTimeoutMinutes, k.MaxTransferMB, k.MaxStorageMB
	tc := acc.cfg.ChibiSensei.Training
	in.MachineShape, in.DeleteRemoteAfterRun = tc.MachineShape, tc.DeleteRemoteAfterRun
	in.Network, in.Write = snap.Access.Network, snap.Access.Write
	if env, ok := acc.rt.driver.(chibiKaggleEnvironment); ok {
		e := env.kaggleEnv()
		in.VaultAvailable, in.CredentialsStored, in.KaggleStorageUsedBytes = e.VaultAvailable, e.CredentialsStored, e.StorageUsedBytes
	}

	in.EstimateMinutes, in.ArtifactsEstimatedMB, in.LargestArtifactMB = plan.EstimateMinutes, plan.ArtifactsEstimatedMB, prof.GGUF.EstimatedMB
	if free, err := s.chibiFreeDisk(chibisensei.ChibiDir(acc.rt.dataDir)); err == nil {
		in.DiskFreeKnown, in.DiskFreeBytes = true, free
	}
	in.StorageUsedBytes = chibisensei.StorageBytes(acc.rt.dataDir) + st.SizeBytes()
	in.StorageMaxGB = acc.cfg.ChibiSensei.Storage.MaxGB

	lic := prof.License
	in.ConsentRequired, in.LicenseID, in.LicenseVersion = lic.RequiresConsent, lic.ID, lic.Version
	if lic.RequiresConsent {
		if in.ConsentRecorded, err = st.HasConsent(ctx, prof.ID, lic.ID, lic.Version); err != nil {
			return in, time.Time{}, err
		}
	}

	checked := time.Now().UTC()
	if chibisensei.KaggleReachable(in) {
		v := acc.rt.kaggleReads.get(ctx, chibiKaggleReadKey(snap), fresh, checked, acc.rt.driver)
		in.ConnectionTested, in.ConnectionOK, in.Account = true, v.ConnectionOK, v.Account
		in.QuotaRead, in.QuotaKnown, in.QuotaHours = v.QuotaRead, v.QuotaKnown, v.QuotaHours
		in.JobsRead, in.ComputeJobs, in.UnknownJobs = v.JobsRead, v.ComputeJobs, v.UnknownJobs
		checked = v.At
	}
	return in, checked, nil
}

// handleChibiPreflight answers GET /projects/{id}/preflight?preset=.
func handleChibiPreflight(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	preset := r.URL.Query().Get("preset")
	if !chibiValidPreset(w, preset) {
		return
	}
	tr, err := s.chibiTrainingState(r.Context(), r, acc, id, preset, false)
	if err != nil {
		chibiTrainingError(w, err)
		return
	}
	var fix any
	if flags := tr.pf.FixFlags(); len(flags) > 0 {
		fix = map[string]any{"flags": flags}
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"ready": tr.pf.Ready, "admin": tr.in.Admin, "preset": tr.plan.Preset,
		"checked_at": tr.checked.UTC().Format(time.RFC3339), "items": tr.pf.Items, "fix": fix})
}

// chibiSameOrigin refuses a browser write from another origin. A bearer client is not a browser; any other caller must
// send a matching Origin header (strict, as /api/kaggle/credentials requires).
func chibiSameOrigin(w http.ResponseWriter, r *http.Request) bool {
	if _, bearer := bearerCredential(r.Header.Get("Authorization")); bearer || checkCSRFOriginWithPolicy(r, true) {
		return true
	}
	writeChibiError(w, http.StatusForbidden, "csrf_check_failed", "Request origin does not match server host.", nil)
	return false
}

// chibiUniqueFlags trims the flag names and drops empty ones and repeats, keeping the order.
func chibiUniqueFlags(flags []string) []string {
	out := []string{}
	for _, f := range flags {
		if f = strings.TrimSpace(f); f != "" && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

// handleChibiPreflightEnable answers POST /preflight/enable {"flags":[...]}: an administrator switches Kaggle settings on by
// name. The server maps each name to its value (PreflightFlagPatch) and saves one patch through applyConfigPatch; the
// client never sends values. Only the flag names are logged.
func handleChibiPreflightEnable(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	if !chibiRequestIsAdmin(s, r) {
		writeChibiError(w, http.StatusForbidden, "admin_required", "Only an administrator can change these settings.", nil)
		return
	}
	var req struct {
		Flags []string `json:"flags"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	flags := chibiUniqueFlags(req.Flags)
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	current := s.ConfigSnapshot()
	if current == nil {
		writeChibiError(w, http.StatusServiceUnavailable, "configuration_invalid", "The configuration is unavailable.", nil)
		return
	}
	patch, err := chibisensei.PreflightFlagPatch(flags, current.Kaggle.AllowedMachineShapes, acc.cfg.ChibiSensei.Training.MachineShape)
	if err != nil {
		chibiValidation(w, "flags", "preflight.unknown_flag", "Unknown setting name.")
		return
	}
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	reloaded, err := applyConfigPatch(s, patch)
	if err != nil {
		logger.Warn("[ChibiSensei] Enabling Kaggle settings failed", "flags", flags, "error", err)
		writeChibiError(w, http.StatusInternalServerError, "config_save_failed", "The configuration could not be saved.", nil)
		return
	}
	s.CfgMu.Lock()
	reloaded.Runtime = s.Cfg.Runtime
	s.replaceConfigSnapshot(reloaded)
	s.CfgMu.Unlock()
	acc.rt.kaggleReads.invalidate()
	logger.Info("[ChibiSensei] Kaggle settings enabled for training", "flags", flags)
	writeChibiJSON(w, http.StatusOK, map[string]any{"applied": flags})
}

// handleChibiConsents answers POST /consents {"profile_id"}: records acceptance of the profile's license version
// (idempotent; a repeat returns the first row).
func handleChibiConsents(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	var req struct {
		ProfileID string `json:"profile_id"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	prof, known := acc.rt.catalog.Get(strings.TrimSpace(req.ProfileID))
	if !known {
		chibiProfileUnknown(w)
		return
	}
	c, err := acc.rt.store.RecordConsent(r.Context(), prof.ID, prof.License.ID, prof.License.Version)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusCreated, map[string]any{"consent": c})
}
