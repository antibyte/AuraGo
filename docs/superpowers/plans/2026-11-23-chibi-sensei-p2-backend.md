# Chibi Sensei P2 backend plan

Six parts, one per backend batch, each written by its own planner against the coordinator contract in the index (`2026-11-23-chibi-sensei-p2.md`). Every part starts with its "Contract changes"; the ones that bind other batches are collected in the index under "Amendments", and the amendments win where a part differs.

| Part | Batch | Content |
|---|---|---|
| 1 | B1 | Kaggle extensions and config |
| 2 | B2a | Go side of the training contract: plan fields, profiles, lock, job spec, notebook, snapshot, parsers, golden files, proof data |
| 3 | B2b | The trainer script and its Go test |
| 4 | B3 | Run domain: store, consent, plan token, preflight, report, driver interface and the in-memory driver |
| 5 | B4 | Run orchestrator, artifacts, GGUF check, the acceptance suite |
| 6 | B5 | HTTP layer, Kaggle driver adapter, wiring, docs and DOX |

---

## Chibi Sensei P2 backend plan, part 1 (B1)

### Contract changes

1. **`ReadKernelLog` scrubs its result and refuses JSON objects.** These are additions; the signature stays as the contract has it.
   - The whole read passes through `security.Scrub` before the tail is cut. Reason: `internal/kaggle/AGENTS.md` says Kaggle output never leaks tokens or signed URLs. B4 may scrub again; scrubbing twice does no harm.
   - A 200 body that starts with `{` (after leading whitespace) is refused with `Kaggle returned an unexpected notebook log response`. Reason: Kaggle's response type is `FileDownload`, and whether it is served while the session runs is UNVERIFIED. If Kaggle answers with a JSON object that carries a log URL, that URL is not registered as sensitive, so passing the body through could leak a signed URL into Chibi's storage. The proof run (step 3) shows the real shape. A JSON array or plain text is returned unchanged.
2. **What `ReadKernelLog` returns for each status.** The contract only says "empty means no log yet".
   - Kaggle API 404, or storage 404 after the redirect: `nil, nil`.
   - 204: `nil, nil`. 200 with an empty body (after scrubbing): `nil, nil`.
   - Any other status: `*kaggle.APIError{Status: n}` (`Uncertain` is false, because this is a download grant).
   - A transport failure: the client's `*APIError`, or `Kaggle download failed` when the storage request fails.
3. **`maxBytes` and the 16 MiB cap.**
   - `maxBytes <= 0` or `> 16 MiB` means 16 MiB.
   - The read stops after 16 MiB. A log longer than that therefore returns the end of its first 16 MiB, not its true end.
   - **Note for B2/B4:** a tail cut out of a JSON-array log is not valid JSON. The plain-text fallback then sees JSON-escaped `@@chibi {\"seq\":…}` fragments. Either B4 calls `ReadKernelLog` with `maxBytes = 16<<20` (it then gets the whole array whenever the log is under 16 MiB) and keeps only the last 64 KiB for display, or B2's `ParseKernelLog` un-escapes fragments. I recommend the first.
4. **Argument shape of `ReadKernelLog`.**
   - `slug` is the bare kernel slug, without the owner prefix. That is the form `get_kernel_session_status` uses in `remote_jobs.go:60-69`.
   - `owner` and `slug` must match `^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`, and `version` must be ≥ 1.
   - The function honours a `WithConfigSnapshot` context the way `Execute` does (`service.go:132-135`).
   - Each call makes two requests: the identity check, then the log.
5. **`RemoveFile` applies no policy gate.** It does not check `enabled` or `Access.Write`. Reason: it is in-process cleanup of files Chibi staged itself, and a policy change in the middle of a run must not strand files that count against `max_storage_mb`. It has no HTTP route and no agent operation. The callers (B5's driver adapter) apply their own gates. This fills a gap in the contract; it is not a change.
6. **Added helpers** (the contract allows helpers):
   - `config.CanonicalKaggleMachineShape(name string) (string, bool)`
   - `const config.DefaultChibiSenseiMachineShape = "NvidiaTeslaT4"`
7. **How the machine-shape names are normalized.**
   - Matching is case-insensitive and maps to the canonical spelling, for example `" nvidial4 "` → `NvidiaL4`.
   - `NormalizeKaggleConfig` always leaves a non-nil slice, so the JSON is `[]` and never `null`.
   - `chibi_sensei.training.machine_shape`: empty becomes `NvidiaTeslaT4`; an unknown name is the error `chibi_sensei: training.machine_shape must be one of NvidiaTeslaT4, NvidiaL4`.
   - `CheckGrants` and `intersectConfig` compare exactly, because the configuration is canonical after load.
8. **Refusal texts in `CheckGrants`.** The old text `custom Kaggle compute images and machine shapes are not supported` is split into:
   - `custom Kaggle compute images are not supported`
   - `a Kaggle machine shape requires a GPU run and the GPU permission`
   - `Kaggle machine shape is not in kaggle.allowed_machine_shapes`

   No test or code depends on the old string (checked with grep).
9. **agotrain (note for the coordinator, no work planned here).**
   - The agent tool schema does not change: `machineShape` is already a field of `save_kernel` (`contracts.json:2131-2133`, `wire_generated.go`), and `contracts.json` is not edited.
   - The exporter (`disposable/export_tools/*.go`) has no `machineShape` literal; `kaggle_test.go:51` only lists `save_kernel`. `.github/agotrain-revision` (`5ecf3575a43db699384c59e594283525b25178b2`) stays unchanged, so no reconcile is needed.
   - I did not read `../agotrain`, because it is outside the worktree. Please confirm with `grep -n machineShape ../agotrain/training/operation_contracts.json` that no curated case expects the old refusal. A curated case that sends a shape would now be accepted only with the grants.
10. **`config.go` `Save` is not changed.**
    - `Save` already writes the whole `KaggleConfig` (`config.go:3162`), so `allowed_machine_shapes` is persisted without an edit.
    - `chibi_sensei` is not in `Save` today. Like `storymaker`, it is saved through the config-patch path, so `training` follows that path.
    - Both config save validators already call `NormalizeChibiSenseiConfig` (`config_patch.go:118`, `config_handlers_main.go:710`), so an unknown machine shape is rejected on save without a server change.
11. **`Service.Status`** already returns `"policy": snap.Config` (`service.go:116`), so `allowed_machine_shapes` appears there with no code change.

### File map

| File | Batch | Responsibility |
|---|---|---|
| `internal/config/kaggle.go` | B1 | `AllowedMachineShapes` field, `KaggleMachineShapes`, `CanonicalKaggleMachineShape`, normalization |
| `internal/config/kaggle_test.go` | B1 | normalization, load, save round trip, template block |
| `internal/config/chibi_sensei.go` | B1 | `ChibiSenseiTrainingConfig`, `DefaultChibiSenseiMachineShape`, machine-shape normalization |
| `internal/config/chibi_sensei_test.go` | B1 | defaults with `training`, the explicit-false bool, shape errors, template parity |
| `internal/config/config.go` | B1 | loader defaults for `training` (next to :602) |
| `internal/kaggle/policy.go` | B1 | `WithConfigSnapshot` copy, `intersectConfig` intersection, `CheckGrants` machine-shape rule |
| `internal/kaggle/transfers.go` | B1 | `followStorageRedirect` helper (taken out of `download`), `RemoveFile` |
| `internal/kaggle/logs.go` (new) | B1 | `ReadKernelLog` |
| `internal/kaggle/logs_test.go` (new) | B1 | `ReadKernelLog` fixtures: grants, owner, references, bodies, redirect, scrub, no job row |
| `internal/kaggle/kaggle_test.go` | B1 | machine-shape grant matrix, snapshot intersection, `RemoveFile` path safety |
| `internal/kaggle/AGENTS.md` | B1 | contracts and verification for the three extensions |
| `config_template.yaml` | B1 | `kaggle.allowed_machine_shapes`, `chibi_sensei.training` |
| `documentation/configuration.md` | B1 | rows in the `chibi_sensei` and `kaggle` tables |
| `documentation/manuals/en/07-configuration.md`, `documentation/manuals/de/07-configuration.md` | B1 | YAML blocks |
| `documentation/kaggle.md` | B1 | machine-shape paragraph, sentence on the in-process helpers |
| `documentation/chibi-sensei.md` | B1 | two settings rows |
| `prompts/tools_manuals/kaggle_notebooks.md` | B1 | `machineShape` permission sentence |

---

#### Task B1: Kaggle extensions and config

**Impact analysis (done by the coordinator; drives the verification):**
- `NormalizeKaggleConfig` is CRITICAL: 23 impacted symbols, and it reaches `main` and `Start` through the config load.
- `WithConfigSnapshot` is HIGH.
- `CheckGrants` and `intersectConfig` are LOW.

Because of that, this task runs the whole `internal/config` and `internal/kaggle` packages plus the Kaggle-focused server, tools and agent tests. The coordinator should also run impact on `(*Service).download` and `load` (both edited here; not analysed yet). Implementers do not run GitNexus.

**Config keys for the frontend plan (F1).** All keys go through the normal `/api/config` save.

- `kaggle.allowed_machine_shapes`
  - Array of strings. The allowed values are exactly `NvidiaTeslaT4` and `NvidiaL4`; show them in this order as checkboxes.
  - The default is `[]`. An old `config.yaml` may lack the key or hold `null` in the raw config; treat that as `[]`.
  - On load the backend drops unknown names and puts the list in canonical order.
  - Grant meaning: a notebook run may request a listed shape only together with `allow_gpu` and a GPU run.
- `chibi_sensei.training.machine_shape`
  - String; exactly one of `NvidiaTeslaT4` or `NvidiaL4` (a dropdown). The default is `NvidiaTeslaT4`, and an empty value is stored and loaded as the default.
  - An unknown value makes the save answer HTTP 400 with `chibi_sensei: training.machine_shape must be one of NvidiaTeslaT4, NvidiaL4`.
- `chibi_sensei.training.delete_remote_after_run`
  - Bool, default `true`. A missing key in an old config loads as `true`.
  - It only takes effect when `kaggle.allow_delete` is granted. Without it the uploaded data stays on Kaggle; that is the preflight warn item `kaggle_allow_delete`.

**UI tests and F1.** No existing UI test is expected to fail before F1 lands. The new template leaves get no derived validation rule: `machine_shape` is a non-URL string, `delete_remote_after_run` is a bool and `allowed_machine_shapes` is an array. `TestConfigChibiSenseiSection` and `TestKaggleConfigLimitsMatchBackendAndRejectOverflow` only check existing markers and limits. Step 17 runs them to prove it.

**Files**
- Create:
  - `internal/kaggle/logs.go`
  - `internal/kaggle/logs_test.go`
- Modify:
  - `internal/config/kaggle.go:3` (imports), `:23` (field), `:49-76` (`NormalizeKaggleConfig`)
  - `internal/config/kaggle_test.go:3-9` (imports), append tests after `:26`
  - `internal/config/chibi_sensei.go:8-17` (type and comment), `:41` (new type after it), `:43-68` (`NormalizeChibiSenseiConfig`)
  - `internal/config/chibi_sensei_test.go:3-10` (imports), `:21-48` (`TestChibiSenseiDefaults`), append after `:136`
  - `internal/config/config.go:601-602`
  - `internal/kaggle/policy.go:3-10` (imports), `:14-17`, `:18-42`, `:154-156`
  - `internal/kaggle/transfers.go:76` (insert `RemoveFile` after `OpenFile`), `:274-318` (`download`)
  - `internal/kaggle/kaggle_test.go:3-19` (imports), append after `:271`
  - `internal/kaggle/AGENTS.md` (Local Contracts, Verification)
  - `config_template.yaml:2176`, `:2436`
  - `documentation/configuration.md:623`, `:658`
  - `documentation/manuals/en/07-configuration.md:1344-1361`, `:1395`
  - `documentation/manuals/de/07-configuration.md:906-923`, `:957`
  - `documentation/kaggle.md:37`, `:88`
  - `documentation/chibi-sensei.md:94`
  - `prompts/tools_manuals/kaggle_notebooks.md:35`

**Existing symbols modified:** `config.KaggleConfig` (struct), `config.NormalizeKaggleConfig`, `config.ChibiSenseiConfig` (struct), `config.NormalizeChibiSenseiConfig`, `config.load` (the defaults block at config.go:601-602), `kaggle.WithConfigSnapshot`, `kaggle.intersectConfig`, `kaggle.CheckGrants`, `(*kaggle.Service).download`, test `config.TestChibiSenseiDefaults`.

**Steps**

- [ ] 1. Before you edit, read the DOX chain: root `AGENTS.md`, `internal/kaggle/AGENTS.md` and `internal/server/AGENTS.md` (only the Kaggle and config parts). `internal/config`, `prompts` and `documentation` have no own guide, so the root guide binds them. Check every line number below against the file before you edit. Work only in the worktree `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\trusting-roentgen-94e148`. Scratch files go to `C:\disposable\chibi-sensei\`. Write real UTF-8 characters, never `\u` escapes, and no BOM.

- [ ] 2. `internal/config/kaggle.go`.
  - Add the field directly after `AllowedOwners` (line 23), aligned like its neighbours:

```go
	AllowedMachineShapes  []string          `yaml:"allowed_machine_shapes" json:"allowed_machine_shapes"`
```

  - Insert after `KaggleLocalConfig` (after line 41), before `DefaultKaggleConfig`:

```go
// KaggleMachineShapes lists the Kaggle machine shapes AuraGo may request for a notebook run, in
// display order. NormalizeKaggleConfig keeps only these names. machineShape has no enum in the
// pinned SDK contract, so this list and kaggle.allowed_machine_shapes are its only restriction.
var KaggleMachineShapes = []string{"NvidiaTeslaT4", "NvidiaL4"}

// CanonicalKaggleMachineShape returns the known spelling of name (trimmed, case-insensitive) and
// whether name is a known machine shape.
func CanonicalKaggleMachineShape(name string) (string, bool) {
	name = strings.TrimSpace(name)
	for _, known := range KaggleMachineShapes {
		if strings.EqualFold(name, known) {
			return known, true
		}
	}
	return "", false
}
```

  - In `NormalizeKaggleConfig`, directly after the `AllowedOwners` trim loop (after line 57), add:

```go
	// Known names only, canonical spelling, each once, in KaggleMachineShapes order. Never nil, so
	// JSON shows [] instead of null.
	shapes := []string{}
	for _, known := range KaggleMachineShapes {
		for _, v := range c.AllowedMachineShapes {
			if canonical, ok := CanonicalKaggleMachineShape(v); ok && canonical == known {
				shapes = append(shapes, known)
				break
			}
		}
	}
	c.AllowedMachineShapes = shapes
```

- [ ] 3. `internal/config/chibi_sensei.go`.
  - Replace the type comment and struct (lines 8-17):

```go
// ChibiSenseiConfig configures the Chibi Sensei desktop app (small-model training studio). Enabled
// and Training.DeleteRemoteAfterRun default to true in the loader, because a YAML bool cannot tell
// "absent" from false. Later phases add Agent and Runtime as sibling fields; none of the fields
// below is renamed then.
type ChibiSenseiConfig struct {
	Enabled   bool                       `yaml:"enabled" json:"enabled"`
	Storage   ChibiSenseiStorageConfig   `yaml:"storage" json:"storage"`
	Dataset   ChibiSenseiDatasetConfig   `yaml:"dataset" json:"dataset"`
	Import    ChibiSenseiImportConfig    `yaml:"import" json:"import"`
	Generator ChibiSenseiGeneratorConfig `yaml:"generator" json:"generator"`
	Training  ChibiSenseiTrainingConfig  `yaml:"training" json:"training"`
}
```

  - Insert after `ChibiSenseiGeneratorConfig` (after line 41):

```go
// DefaultChibiSenseiMachineShape is the Kaggle machine shape of a training run when none is set.
const DefaultChibiSenseiMachineShape = "NvidiaTeslaT4"

// ChibiSenseiTrainingConfig controls remote training runs on Kaggle.
type ChibiSenseiTrainingConfig struct {
	MachineShape         string `yaml:"machine_shape" json:"machine_shape"`                     // one of KaggleMachineShapes; default NvidiaTeslaT4
	DeleteRemoteAfterRun bool   `yaml:"delete_remote_after_run" json:"delete_remote_after_run"` // default true (set by the loader)
}
```

  - Replace `NormalizeChibiSenseiConfig` (lines 43-68):

```go
// NormalizeChibiSenseiConfig trims the provider and model, applies defaults for zero values and
// rejects values outside their range or an unknown training machine shape. It changes neither
// Enabled nor Training.DeleteRemoteAfterRun; the loader sets their defaults before decoding.
func NormalizeChibiSenseiConfig(c *ChibiSenseiConfig) error {
	c.Generator.Provider = strings.TrimSpace(c.Generator.Provider)
	c.Generator.Model = strings.TrimSpace(c.Generator.Model)
	if strings.TrimSpace(c.Training.MachineShape) == "" {
		c.Training.MachineShape = DefaultChibiSenseiMachineShape
	} else if shape, ok := CanonicalKaggleMachineShape(c.Training.MachineShape); ok {
		c.Training.MachineShape = shape
	} else {
		return fmt.Errorf("chibi_sensei: training.machine_shape must be one of %s", strings.Join(KaggleMachineShapes, ", "))
	}
	for _, f := range []struct {
		p           *int
		def, lo, hi int
		key         string
	}{
		{&c.Storage.MaxGB, 20, 1, 500, "storage.max_gb"},
		{&c.Dataset.MaxRows, 50000, 1000, 50000, "dataset.max_rows"},
		{&c.Import.MaxRows, 20000, 100, 50000, "import.max_rows"},
		{&c.Import.MaxFileMB, 50, 1, 500, "import.max_file_mb"},
		{&c.Generator.MaxRowsPerJob, 500, 10, 5000, "generator.max_rows_per_job"},
		{&c.Generator.MaxDailyRows, 5000, 100, 100000, "generator.max_daily_rows"},
	} {
		if *f.p == 0 {
			*f.p = f.def
		}
		if *f.p < f.lo || *f.p > f.hi {
			return fmt.Errorf("chibi_sensei: %s must be %d–%d", f.key, f.lo, f.hi)
		}
	}
	return nil
}
```

- [ ] 4. `internal/config/config.go`. Replace lines 601-602 (check them first: the comment `// Chibi Sensei is on by default…` and `cfg.ChibiSensei.Enabled = true`):

```go
	// Chibi Sensei is on by default: local dataset studio; never widens other permissions.
	// Training defaults: T4 GPU, and Kaggle copies are removed after a run when allow_delete permits.
	cfg.ChibiSensei.Enabled = true
	cfg.ChibiSensei.Training.MachineShape = DefaultChibiSenseiMachineShape
	cfg.ChibiSensei.Training.DeleteRemoteAfterRun = true
```

  `cfg.Kaggle = DefaultKaggleConfig()` (line 1039) and `NormalizeKaggleConfig(&cfg.Kaggle)` (line 1170) stay as they are, as does the `NormalizeChibiSenseiConfig` call (line 1136). `Save` needs no change (Contract changes 10).

- [ ] 5. `config_template.yaml`.
  - After line 2176 (`allowed_owners: [] …`) insert:

```yaml
  allowed_machine_shapes: [] # Machine shapes a GPU notebook run may request: NvidiaTeslaT4, NvidiaL4. Each also needs allow_gpu.
```

  - After line 2436 (`max_daily_rows: 5000 …` of `chibi_sensei.generator`) insert, with the block's 4-space indent:

```yaml
    training:
        machine_shape: "NvidiaTeslaT4"  # Kaggle GPU for training runs: NvidiaTeslaT4 or NvidiaL4; must also be in kaggle.allowed_machine_shapes
        delete_remote_after_run: true   # delete the uploaded dataset and the notebook on Kaggle after a run (needs kaggle.allow_delete)
```

- [ ] 6. `internal/kaggle/policy.go`.
  - Add `"slices"` to the imports (lines 3-10).
  - Replace `WithConfigSnapshot` (lines 14-17):

```go
func WithConfigSnapshot(ctx context.Context, c config.KaggleConfig) context.Context {
	c.AllowedOwners = append([]string(nil), c.AllowedOwners...)
	c.AllowedMachineShapes = append([]string(nil), c.AllowedMachineShapes...)
	return context.WithValue(ctx, grantSnapshotKey{}, c)
}
```

  - In `intersectConfig`, before `return a` (line 41), insert:

```go
	// A run keeps only the machine shapes both the current config and its admission snapshot allow.
	shapes := []string{}
	for _, v := range a.AllowedMachineShapes {
		if slices.Contains(b.AllowedMachineShapes, v) {
			shapes = append(shapes, v)
		}
	}
	a.AllowedMachineShapes = shapes
```

  - Replace lines 154-156 in the `save_kernel` block of `CheckGrants`:

```go
		if stringValue(params, "dockerImage") != "" {
			return errors.New("custom Kaggle compute images are not supported")
		}
		if shape := stringValue(params, "machineShape"); shape != "" {
			if !boolValue(params, "enableGpu") || !c.AllowGPU {
				return errors.New("a Kaggle machine shape requires a GPU run and the GPU permission")
			}
			if !slices.Contains(c.AllowedMachineShapes, shape) {
				return errors.New("Kaggle machine shape is not in kaggle.allowed_machine_shapes")
			}
		}
```

  `CheckPolicy` calls `CheckGrants`, so the run's `check()` (`service.go:253-256`) checks again against the current config intersected with the snapshot. `Execute` does not default `machineShape` (`service.go:154-179`), so an absent shape keeps today's behaviour.

- [ ] 7. `internal/kaggle/transfers.go`: take the redirect handling out of `download`.
  - Insert this helper directly above `func (s *Service) download` (line 274):

```go
// followStorageRedirect follows one Kaggle storage redirect with an uncredentialed, SSRF-checked
// client from storageFactory. A response that is not a redirect is returned unchanged. The caller
// closes the returned body. timeout 0 keeps the storage client's default.
func (s *Service) followStorageRedirect(ctx context.Context, resp *http.Response, timeout time.Duration) (*http.Response, error) {
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return resp, nil
	}
	location := resp.Header.Get("Location")
	resp.Body.Close()
	h, err := s.storageFactory(ctx, location, timeout)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, errors.New("invalid storage request")
	}
	next, err := h.Do(r)
	if err != nil {
		return nil, errors.New("Kaggle download failed")
	}
	return next, nil
}
```

  - In `download`, replace lines 285-305 (from `resp, err := c.request(ctx, op, params)` through `defer resp.Body.Close()`) with the block below. Behaviour stays the same; `download` keeps timeout 0 as before.

```go
	resp, err := c.request(ctx, op, params)
	if err != nil {
		return nil, err
	}
	resp, err = s.followStorageRedirect(ctx, resp, 0)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
```

- [ ] 8. `internal/kaggle/transfers.go`: insert `RemoveFile` directly after `OpenFile` (after line 76):

```go
// RemoveFile deletes one regular file of a project. A missing file is not an error. Symlinks,
// directories and invalid names are refused. It applies no policy gate: it is in-process cleanup
// for callers that apply their own gates, with no HTTP route and no agent operation.
func (s *Service) RemoveFile(project, name string) error {
	if !validFile(name) {
		return errors.New("invalid project filename")
	}
	s.filesMu.Lock()
	defer s.filesMu.Unlock()
	r, err := s.projectRoot(project)
	if err != nil {
		return err
	}
	defer r.Close()
	info, err := r.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("project file must be a regular file")
	}
	if err := r.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
```

- [ ] 9. Create `internal/kaggle/logs.go`:

```go
package kaggle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// kernelLogReadCap bounds one notebook log read; ReadKernelLog never buffers more.
const kernelLogReadCap int64 = 16 << 20

// kernelLogWaitSeconds lets Kaggle wait briefly for the log URL of a running session.
const kernelLogWaitSeconds = 10

// kernelRefPattern accepts a Kaggle user name or a bare kernel slug (no owner prefix).
var kernelRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)

// ReadKernelLog reads the session log of one notebook version without a durable job and without a
// project file. It needs the download grant, filesystem write access and network access, and owner
// must be the authenticated account. slug is the bare kernel slug. It returns at most the last
// maxBytes bytes (maxBytes <= 0 or above 16 MiB means 16 MiB; the read itself stops at 16 MiB) of
// the scrubbed log text. An empty result with a nil error means "no log yet" (HTTP 404, 204 or an
// empty body). Another HTTP status returns *APIError. A JSON object body is refused, because only
// a JSON array of log entries or plain text is a log.
func (s *Service) ReadKernelLog(ctx context.Context, access Access, owner, slug string, version int, maxBytes int64) ([]byte, error) {
	if !kernelRefPattern.MatchString(owner) || !kernelRefPattern.MatchString(slug) {
		return nil, errors.New("invalid Kaggle notebook reference")
	}
	if version <= 0 {
		return nil, errors.New("Kaggle notebook version must be positive")
	}
	if maxBytes <= 0 || maxBytes > kernelLogReadCap {
		maxBytes = kernelLogReadCap
	}
	op, err := Lookup("notebooks", "get_kernel_session_logs_stream")
	if err != nil {
		return nil, err
	}
	params := map[string]any{"userName": owner, "kernelSlug": slug, "versionLabel": strconv.Itoa(version), "waitForLogsUrlSeconds": kernelLogWaitSeconds}
	snap, c, err := s.client()
	if err != nil {
		return nil, err
	}
	if run, ok := ctx.Value(grantSnapshotKey{}).(config.KaggleConfig); ok {
		snap.Config = intersectConfig(snap.Config, run)
	}
	access = intersect(access, snap.Access)
	if err := CheckGrants(snap.Config, access, op, params); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode Kaggle log request: %w", err)
	}
	_, typed, err := Decode(op, raw)
	if err != nil {
		return nil, err
	}
	account, err := c.Identity(ctx)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(account, owner) {
		return nil, errors.New("Kaggle notebook logs are readable only for the authenticated account")
	}
	resp, err := c.request(ctx, op, typed)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	resp, err = s.followStorageRedirect(ctx, resp, time.Duration(snap.Config.RequestTimeoutSeconds)*time.Second)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNoContent, http.StatusNotFound:
		return nil, nil
	default:
		return nil, &APIError{Status: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, kernelLogReadCap))
	if err != nil {
		return nil, fmt.Errorf("read Kaggle notebook log: %w", err)
	}
	if trimmed := bytes.TrimLeft(body, " \t\r\n"); len(trimmed) > 0 && trimmed[0] == '{' {
		return nil, errors.New("Kaggle returned an unexpected notebook log response")
	}
	text := security.Scrub(string(body))
	if int64(len(text)) > maxBytes {
		text = text[len(text)-int(maxBytes):]
	}
	if text == "" {
		return nil, nil
	}
	return []byte(text), nil
}
```

  Order of checks, so that a refusal never reaches the network:
  1. Reference validation.
  2. Grants: `enabled`, network, `allow_download` and filesystem write. Read-only does not block downloads (`policy.go:110-115`).
  3. Identity.
  4. Owner.
  5. The log request.

  Nothing touches `s.Store`, `s.Root`, `s.mu` or `BeginJob`.

- [ ] 10. `internal/kaggle/kaggle_test.go`: tests for the grant matrix, the snapshot and `RemoveFile`. Add `"reflect"` to the imports, then append:

```go
func TestKaggleMachineShapeGrants(t *testing.T) {
	base := config.DefaultKaggleConfig()
	base.Enabled, base.ReadOnly = true, false
	base.AllowCreate, base.AllowRemoteExecute, base.AllowGPU = true, true, true
	base.AllowedMachineShapes = []string{"NvidiaTeslaT4"}
	a := Access{Network: true, Write: true}
	op, _ := Lookup("notebooks", "save_kernel")
	params := func(extra map[string]any) map[string]any {
		p := map[string]any{"kernelExecutionType": "SAVE_AND_RUN_ALL", "enableGpu": true}
		for k, v := range extra {
			if v == nil {
				delete(p, k)
			} else {
				p[k] = v
			}
		}
		return p
	}
	for _, tc := range []struct {
		name   string
		mutate func(*config.KaggleConfig)
		params map[string]any
		ok     bool
	}{
		{"listed shape with GPU", nil, params(map[string]any{"machineShape": "NvidiaTeslaT4"}), true},
		{"no shape", nil, params(nil), true},
		{"empty shape", nil, params(map[string]any{"machineShape": ""}), true},
		{"quick save with shape and GPU", nil, params(map[string]any{"kernelExecutionType": "QUICK_SAVE", "machineShape": "NvidiaTeslaT4"}), true},
		{"second listed shape", func(c *config.KaggleConfig) { c.AllowedMachineShapes = []string{"NvidiaTeslaT4", "NvidiaL4"} }, params(map[string]any{"machineShape": "NvidiaL4"}), true},
		{"shape without enableGpu", nil, params(map[string]any{"machineShape": "NvidiaTeslaT4", "enableGpu": nil}), false},
		{"shape with enableGpu false", nil, params(map[string]any{"machineShape": "NvidiaTeslaT4", "enableGpu": false}), false},
		{"shape without GPU grant", func(c *config.KaggleConfig) { c.AllowGPU = false }, params(map[string]any{"machineShape": "NvidiaTeslaT4"}), false},
		{"known shape not listed", nil, params(map[string]any{"machineShape": "NvidiaL4"}), false},
		{"unknown shape", nil, params(map[string]any{"machineShape": "NvidiaA100"}), false},
		{"wrong case", nil, params(map[string]any{"machineShape": "nvidiateslat4"}), false},
		{"empty allow-list", func(c *config.KaggleConfig) { c.AllowedMachineShapes = nil }, params(map[string]any{"machineShape": "NvidiaTeslaT4"}), false},
		{"docker image with listed shape", nil, params(map[string]any{"machineShape": "NvidiaTeslaT4", "dockerImage": "gcr.io/kaggle-gpu-images/python"}), false},
		{"docker image alone", nil, params(map[string]any{"dockerImage": "gcr.io/kaggle-gpu-images/python"}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			c.AllowedMachineShapes = append([]string(nil), base.AllowedMachineShapes...)
			if tc.mutate != nil {
				tc.mutate(&c)
			}
			err := CheckGrants(c, a, op, tc.params)
			if (err == nil) != tc.ok {
				t.Fatalf("CheckGrants(%v) = %v, want ok=%v", tc.params, err, tc.ok)
			}
		})
	}
}

func TestKaggleMachineShapeSnapshotIntersection(t *testing.T) {
	current := config.KaggleConfig{AllowedMachineShapes: []string{"NvidiaTeslaT4", "NvidiaL4"}}
	for _, tc := range []struct{ snapshot, want []string }{
		{[]string{"NvidiaL4"}, []string{"NvidiaL4"}},
		{[]string{"NvidiaL4", "NvidiaTeslaT4"}, []string{"NvidiaTeslaT4", "NvidiaL4"}},
		{nil, []string{}},
	} {
		got := intersectConfig(current, config.KaggleConfig{AllowedMachineShapes: tc.snapshot}).AllowedMachineShapes
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("intersect with %v = %v, want %v", tc.snapshot, got, tc.want)
		}
	}
	// Revoking a shape in the current config also revokes it for a running snapshot.
	revoked := intersectConfig(config.KaggleConfig{AllowedMachineShapes: []string{"NvidiaL4"}}, current).AllowedMachineShapes
	if !reflect.DeepEqual(revoked, []string{"NvidiaL4"}) {
		t.Fatalf("revocation ignored: %v", revoked)
	}
	snap := config.KaggleConfig{AllowedMachineShapes: []string{"NvidiaTeslaT4"}}
	ctx := WithConfigSnapshot(context.Background(), snap)
	snap.AllowedMachineShapes[0] = "NvidiaL4"
	held := ctx.Value(grantSnapshotKey{}).(config.KaggleConfig)
	if held.AllowedMachineShapes[0] != "NvidiaTeslaT4" {
		t.Fatal("WithConfigSnapshot shares the machine-shape slice with its caller")
	}
}

func TestKaggleRemoveFilePathSafety(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	p, err := s.CreateProject(ctx, "Staging")
	if err != nil {
		t.Fatal(err)
	}
	const name = "r0123456789ab-train.jsonl"
	if _, err = s.WriteFile(ctx, p.ID, name, strings.NewReader("{}\n"), nil); err != nil {
		t.Fatal(err)
	}
	if err = s.RemoveFile(p.ID, name); err != nil {
		t.Fatal(err)
	}
	if files, _ := s.Files(p.ID); len(files) != 0 {
		t.Fatalf("file not removed: %v", files)
	}
	if err = s.RemoveFile(p.ID, name); err != nil {
		t.Fatalf("missing file must not be an error: %v", err)
	}
	for _, bad := range []string{"", ".", "..", "../secret", "a/b", "a\\b", ".hidden", "CON", "C:x"} {
		if s.RemoveFile(p.ID, bad) == nil {
			t.Fatalf("unsafe name %q accepted", bad)
		}
	}
	if s.RemoveFile("not-a-project", "x.txt") == nil {
		t.Fatal("invalid project ID accepted")
	}
	dir := filepath.Join(s.Root, p.ID, "folder")
	if err = os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if s.RemoveFile(p.ID, "folder") == nil {
		t.Fatal("directory removed")
	}
	if _, err = os.Stat(dir); err != nil {
		t.Fatal("directory lost")
	}
	target := filepath.Join(s.Root, p.ID, "keep.txt")
	if err = os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Symlink creation needs a privilege on Windows; the existing transfer test uses the same guard.
	if err = os.Symlink(target, filepath.Join(s.Root, p.ID, "link.txt")); err == nil {
		if s.RemoveFile(p.ID, "link.txt") == nil {
			t.Fatal("symlink removed")
		}
		if _, err = os.Stat(target); err != nil {
			t.Fatal("symlink target lost")
		}
	}
}
```

- [ ] 11. Create `internal/kaggle/logs_test.go`:

```go
package kaggle

import (
	"context"
	"errors"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/security"
)

type logFixture struct {
	requests atomic.Int32 // every request to the Kaggle API fixture
	logCalls atomic.Int32 // requests to GetKernelSessionLogsStream
	params   atomic.Value // map[string]any of the last log request
}

// newLogTestService returns a service whose Kaggle client is a local fixture. The account is
// "owner", downloads are granted, and respond answers the log operation.
func newLogTestService(t *testing.T, mutate func(*Snapshot), respond func(http.ResponseWriter)) (*Service, *logFixture) {
	t.Helper()
	s := newTestService(t)
	snap := s.current()
	snap.Config.AllowDownload = true
	if mutate != nil {
		mutate(&snap)
	}
	s.current = func() Snapshot { return snap }
	f := &logFixture{}
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		var p map[string]any
		_ = json.NewDecoder(r.Body).Decode(&p)
		switch {
		case strings.HasSuffix(r.URL.Path, "IntrospectToken"):
			io.WriteString(w, `{"active":true,"username":"owner"}`)
		case strings.HasSuffix(r.URL.Path, "GetKernelSessionLogsStream"):
			f.logCalls.Add(1)
			f.params.Store(p)
			respond(w)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	s.clientFactory = func(Credentials, time.Duration) (*Client, error) { return c, nil }
	return s, f
}

// assertNoDurableTrace fails when a log read left a job row or a project directory behind.
func assertNoDurableTrace(t *testing.T, s *Service) {
	t.Helper()
	jobs, err := s.Store.List(context.Background())
	if err != nil || len(jobs) != 0 {
		t.Fatalf("log read created job rows: %v %v", jobs, err)
	}
	entries, err := os.ReadDir(s.Root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("log read wrote project storage: %v %v", entries, err)
	}
}

var logAccess = Access{Network: true, Write: true}

func TestKaggleReadKernelLogGrantsOwnerAndReferences(t *testing.T) {
	ok := func(w http.ResponseWriter) { io.WriteString(w, "log") }
	for _, tc := range []struct {
		name   string
		mutate func(*Snapshot)
		access Access
		ctx    func(context.Context, Snapshot) context.Context
	}{
		{"integration disabled", func(s *Snapshot) { s.Config.Enabled = false }, logAccess, nil},
		{"download grant missing", func(s *Snapshot) { s.Config.AllowDownload = false }, logAccess, nil},
		{"network denied by caller", nil, Access{Write: true}, nil},
		{"write denied by caller", nil, Access{Network: true}, nil},
		{"write denied by host", func(s *Snapshot) { s.Access.Write = false }, logAccess, nil},
		{"run snapshot without download", nil, logAccess, func(ctx context.Context, s Snapshot) context.Context {
			c := s.Config
			c.AllowDownload = false
			return WithConfigSnapshot(ctx, c)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := newLogTestService(t, tc.mutate, ok)
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx(ctx, s.current())
			}
			if _, err := s.ReadKernelLog(ctx, tc.access, "owner", "chibi-run", 3, 1024); err == nil {
				t.Fatal("refused read succeeded")
			}
			if f.requests.Load() != 0 {
				t.Fatalf("a refused read contacted Kaggle %d times", f.requests.Load())
			}
			assertNoDurableTrace(t, s)
		})
	}
	s, f := newLogTestService(t, nil, ok)
	for _, ref := range []struct {
		owner, slug string
		version     int
	}{{"owner", "owner/chibi-run", 3}, {"", "chibi-run", 3}, {"owner", "../x", 3}, {"owner", "chibi-run", 0}, {"owner", "chibi-run", -1}} {
		if _, err := s.ReadKernelLog(context.Background(), logAccess, ref.owner, ref.slug, ref.version, 1024); err == nil {
			t.Fatalf("invalid reference accepted: %+v", ref)
		}
	}
	if f.requests.Load() != 0 {
		t.Fatal("invalid references contacted Kaggle")
	}
	if _, err := s.ReadKernelLog(context.Background(), logAccess, "stranger", "chibi-run", 3, 1024); err == nil {
		t.Fatal("foreign owner accepted")
	}
	if f.logCalls.Load() != 0 {
		t.Fatal("foreign owner reached the log operation")
	}
	assertNoDurableTrace(t, s)
}

func TestKaggleReadKernelLogBodies(t *testing.T) {
	jsonArray := `[{"stream_name":"stdout","time":1.5,"data":"@@chibi {\"seq\":1,\"t\":\"stage\"}\n"}]`
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		max     int64
		want    string
		wantErr bool
	}{
		{"plain text", 200, "line\n@@chibi {\"seq\":1}\n", 1 << 20, "line\n@@chibi {\"seq\":1}\n", false},
		{"tail", 200, "0123456789", 4, "6789", false},
		{"zero max means the cap", 200, "abc", 0, "abc", false},
		{"json array", 200, jsonArray, 1 << 20, jsonArray, false},
		{"empty body", 200, "", 1024, "", false},
		{"not found yet", 404, "", 1024, "", false},
		{"no content", 204, "", 1024, "", false},
		{"server error", 500, "", 1024, "", true},
		{"json object refused", 200, `{"logsUrl":"https://signed.invalid/x?sig=abc"}`, 1024, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := newLogTestService(t, nil, func(w http.ResponseWriter) {
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			got, err := s.ReadKernelLog(context.Background(), logAccess, "owner", "chibi-run", 3, tc.max)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "signed.invalid") {
				t.Fatal("error leaked the response body")
			}
			if tc.status == 500 {
				var api *APIError
				if !errors.As(err, &api) || api.Status != 500 {
					t.Fatalf("want APIError 500, got %#v", err)
				}
			}
			if string(got) != tc.want || tc.want == "" && got != nil {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			p, _ := f.params.Load().(map[string]any)
			if p["userName"] != "owner" || p["kernelSlug"] != "chibi-run" || p["versionLabel"] != "3" || p["waitForLogsUrlSeconds"] != float64(10) {
				t.Fatalf("wire parameters %v", p)
			}
			assertNoDurableTrace(t, s)
		})
	}
}

func TestKaggleReadKernelLogRedirectAndScrub(t *testing.T) {
	secret := "chibi-log-secret-" + newID()
	security.RegisterSensitive(secret)
	for _, tc := range []struct {
		name          string
		storageStatus int
		want          string
	}{
		{"storage body", 200, "before  after"}, // compared after removing the scrub placeholder, see below
		{"storage not found", 404, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newLogTestService(t, nil, func(w http.ResponseWriter) {
				w.Header().Set("Location", "https://storage.invalid/log")
				w.WriteHeader(http.StatusFound)
			})
			var storageCalls atomic.Int32
			s.storageFactory = func(ctx context.Context, location string, timeout time.Duration) (*http.Client, error) {
				if location != "https://storage.invalid/log" {
					t.Errorf("storage location %q", location)
				}
				return &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
					storageCalls.Add(1)
					if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Method != http.MethodGet {
						t.Error("Kaggle credentials or a non-GET reached storage")
					}
					return &http.Response{StatusCode: tc.storageStatus, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("before " + secret + " after"))}, nil
				})}, nil
			}
			got, err := s.ReadKernelLog(context.Background(), logAccess, "owner", "chibi-run", 3, 1024)
			if err != nil {
				t.Fatal(err)
			}
			if storageCalls.Load() != 1 {
				t.Fatalf("storage calls = %d", storageCalls.Load())
			}
			if strings.Contains(string(got), secret) {
				t.Fatal("registered secret survived the log read")
			}
			if tc.want == "" && got != nil {
				t.Fatalf("storage 404 must mean no log yet, got %q", got)
			}
			if tc.want != "" && (!strings.HasPrefix(string(got), "before ") || !strings.HasSuffix(string(got), " after")) {
				t.Fatalf("storage body not returned: %q", got)
			}
			assertNoDurableTrace(t, s)
		})
	}
}
```

  Order the import block the way `gofmt` does (`errors` after `encoding/json`); run `gofmt -l internal/kaggle` in step 17. The comment on the `"storage body"` row is only a reminder: the check uses the prefix and suffix, because the scrub placeholder text is not part of this contract.

- [ ] 12. `internal/config/kaggle_test.go`: tests for normalization, load, save and the template. Extend the imports to `"encoding/json"`, `"os"`, `"path/filepath"`, `"reflect"`, `"strings"`, `"testing"`, `"gopkg.in/yaml.v3"` and `"bytes"`, then append:

```go
func TestKaggleMachineShapesNormalize(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []string
	}{
		{"nil", nil, []string{}},
		{"canonical order", []string{"NvidiaL4", "NvidiaTeslaT4"}, []string{"NvidiaTeslaT4", "NvidiaL4"}},
		{"trim, case and duplicates", []string{" nvidial4 ", "NvidiaL4", "NVIDIATESLAT4"}, []string{"NvidiaTeslaT4", "NvidiaL4"}},
		{"unknown names dropped", []string{"NvidiaA100", "", "TpuV38"}, []string{}},
	} {
		c := KaggleConfig{AllowedMachineShapes: tc.in}
		NormalizeKaggleConfig(&c)
		if !reflect.DeepEqual(c.AllowedMachineShapes, tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, c.AllowedMachineShapes, tc.want)
		}
	}
	b, _ := json.Marshal(DefaultKaggleConfig())
	if !strings.Contains(string(b), `"allowed_machine_shapes":[]`) {
		t.Fatalf("default must serialize an empty list: %s", b)
	}
	if shape, ok := CanonicalKaggleMachineShape(" nvidiateslat4 "); !ok || shape != "NvidiaTeslaT4" {
		t.Fatalf("CanonicalKaggleMachineShape = %q %v", shape, ok)
	}
	if _, ok := CanonicalKaggleMachineShape("NvidiaA100"); ok {
		t.Fatal("unknown shape accepted")
	}
}

func TestKaggleMachineShapesLoadSaveAndTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("kaggle:\n  allowed_machine_shapes: [\" NvidiaL4\", NvidiaA100, NvidiaTeslaT4, NvidiaL4]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"NvidiaTeslaT4", "NvidiaL4"}; !reflect.DeepEqual(cfg.Kaggle.AllowedMachineShapes, want) {
		t.Fatalf("loaded %v, want %v", cfg.Kaggle.AllowedMachineShapes, want)
	}
	template, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(template, &doc); err != nil {
		t.Fatal(err)
	}
	node := doc["kaggle"]
	raw, _ := yaml.Marshal(&node)
	var tpl KaggleConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("template kaggle block: %v", err)
	}
	if len(tpl.AllowedMachineShapes) != 0 {
		t.Fatalf("template must grant no machine shape: %v", tpl.AllowedMachineShapes)
	}
	full := filepath.Join(dir, "full.yaml")
	if err := os.WriteFile(full, template, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(full)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Kaggle.AllowedMachineShapes = []string{"NvidiaL4"}
	if err := cfg.Save(full); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(full)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded.Kaggle.AllowedMachineShapes, []string{"NvidiaL4"}) {
		t.Fatalf("saved list not reloaded: %v", reloaded.Kaggle.AllowedMachineShapes)
	}
}
```

- [ ] 13. `internal/config/chibi_sensei_test.go`.
  - Add `"strings"` to the imports.
  - In `TestChibiSenseiDefaults`, add `Training: ChibiSenseiTrainingConfig{MachineShape: "NvidiaTeslaT4", DeleteRemoteAfterRun: true},` to `want`.
  - Add the map case `"null training": "chibi_sensei:\n  training:\n",`.
  - Append:

```go
func TestChibiSenseiTrainingConfig(t *testing.T) {
	cfg, err := loadChibiSenseiYAML(t, "chibi_sensei:\n  training:\n    delete_remote_after_run: false\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChibiSensei.Training.DeleteRemoteAfterRun || cfg.ChibiSensei.Training.MachineShape != "NvidiaTeslaT4" {
		t.Fatalf("explicit false or default shape lost: %+v", cfg.ChibiSensei.Training)
	}
	cfg, err = loadChibiSenseiYAML(t, "chibi_sensei:\n  training:\n    machine_shape: \" nvidial4 \"\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChibiSensei.Training.MachineShape != "NvidiaL4" || !cfg.ChibiSensei.Training.DeleteRemoteAfterRun {
		t.Fatalf("shape not canonical or delete default lost: %+v", cfg.ChibiSensei.Training)
	}
	for _, shape := range []string{"NvidiaA100", "T4", "NvidiaTeslaT4,NvidiaL4"} {
		c := ChibiSenseiConfig{Training: ChibiSenseiTrainingConfig{MachineShape: shape}}
		err := NormalizeChibiSenseiConfig(&c)
		if err == nil || !strings.Contains(err.Error(), "training.machine_shape must be one of NvidiaTeslaT4, NvidiaL4") {
			t.Errorf("shape %q: %v", shape, err)
		}
	}
	if _, err := loadChibiSenseiYAML(t, "chibi_sensei:\n  training:\n    machine_shape: NvidiaA100\n"); err == nil {
		t.Error("Load accepted an unknown machine shape")
	}
	var c ChibiSenseiConfig
	if err := NormalizeChibiSenseiConfig(&c); err != nil || c.Training.MachineShape != "NvidiaTeslaT4" || c.Training.DeleteRemoteAfterRun {
		t.Fatalf("normalize must default the shape and leave the bool alone: %+v %v", c.Training, err)
	}
}
```

  `TestChibiSenseiTemplateMatchesLoadDefaults` needs no edit. After step 5 it proves that the template's `training` block equals the loader defaults: `KnownFields(true)` rejects a misspelt key, and the struct stays comparable with `!=`.

- [ ] 14. Documentation (English, plain wording).
  - `documentation/configuration.md`, after line 623 (`generator.max_daily_rows`):

```markdown
| `training.machine_shape` | `NvidiaTeslaT4` | Kaggle GPU for training runs: `NvidiaTeslaT4` or `NvidiaL4`. It must also be listed in `kaggle.allowed_machine_shapes`. Any other value is rejected when you save. |
| `training.delete_remote_after_run` | `true` | After a run, delete the uploaded dataset and the notebook on Kaggle. Needs `kaggle.allow_delete`; without it they stay on Kaggle and the run lists them. |
```

  - Same file, after line 658 (`allowed_owners`):

```markdown
| `allowed_machine_shapes` | `[]` | Machine shapes a notebook run may request: `NvidiaTeslaT4`, `NvidiaL4`. A run that names one also needs `allow_gpu` and a GPU run. Unknown names are dropped when the configuration loads. |
```

  - `documentation/manuals/en/07-configuration.md`:
    - After line 1360 (`max_daily_rows` in the YAML block):

```yaml
    training:
        machine_shape: "NvidiaTeslaT4"  # Kaggle GPU for training runs: NvidiaTeslaT4 or NvidiaL4; must also be in kaggle.allowed_machine_shapes
        delete_remote_after_run: true   # delete the uploaded dataset and the notebook on Kaggle after a run (needs kaggle.allow_delete)
```

    - After line 1395 (`allowed_owners: []`):

```yaml
  allowed_machine_shapes: []  # NvidiaTeslaT4, NvidiaL4; each also needs allow_gpu
```

  - `documentation/manuals/de/07-configuration.md`:
    - After line 922:

```yaml
    training:
        machine_shape: "NvidiaTeslaT4"  # Kaggle-GPU für Trainingsläufe: NvidiaTeslaT4 oder NvidiaL4; muss auch in kaggle.allowed_machine_shapes stehen
        delete_remote_after_run: true   # löscht nach einem Lauf den hochgeladenen Datensatz und das Notebook auf Kaggle (braucht kaggle.allow_delete)
```

    - After line 957:

```yaml
  allowed_machine_shapes: []  # NvidiaTeslaT4, NvidiaL4; jeder Typ braucht zusätzlich allow_gpu
```

  - `documentation/chibi-sensei.md`, after line 94:

```markdown
| `training.machine_shape` | `NvidiaTeslaT4` | `NvidiaTeslaT4` or `NvidiaL4`; also listed in `kaggle.allowed_machine_shapes` |
| `training.delete_remote_after_run` | `true` | needs `kaggle.allow_delete` |
```

  - `documentation/kaggle.md`:
    - After line 37, add this paragraph:

```markdown
`allowed_machine_shapes` (default empty) lists the Kaggle machine shapes a notebook run may request with `machineShape`; the known names are `NvidiaTeslaT4` and `NvidiaL4`. A run that names a shape must also set `enableGpu: true`, and `allow_gpu` must be granted. Unknown names are removed when the configuration loads. Custom compute images (`dockerImage`) stay unsupported.
```

    - After line 88, add:

```markdown
Chibi Sensei uses two in-process helpers that are not agent tools or HTTP routes: a read of its own notebook's session log (download grant and filesystem write, own account only, no job receipt and no project file, at most 16 MiB, scrubbed) and the removal of its staged project files.
```

- [ ] 15. Agent-facing text: `prompts/tools_manuals/kaggle_notebooks.md`.
  - In the paragraph that starts `` `save_kernel` requires `` (line 35), replace the sentence `GPU, TPU and internet require separate grants.` with:

```markdown
GPU, TPU and internet require separate grants. `machineShape` is accepted only for a name listed in `kaggle.allowed_machine_shapes` (known: `NvidiaTeslaT4`, `NvidiaL4`), together with `enableGpu: true` and the GPU grant; `dockerImage` is not supported.
```

  - Leave the operations table unchanged: `machineShape` is already listed, and the agent schema does not change.
  - Grep confirmed that no other text under `prompts/`, `internal/agent` or `internal/tools` says machine shapes are refused.

- [ ] 16. `internal/kaggle/AGENTS.md`.
  - Append to **Local Contracts**:

```markdown
- `kaggle.allowed_machine_shapes` is the only gate for `save_kernel.machineShape` (the SDK has no enum). `NormalizeKaggleConfig` keeps the names of `config.KaggleMachineShapes` in canonical spelling and order; `CheckGrants` requires a listed name, `enableGpu: true` and `allow_gpu`. `dockerImage` stays refused. `WithConfigSnapshot` copies the list and `intersectConfig` intersects it, like `allowed_owners`.
- `ReadKernelLog` is the only non-durable download-grant path: download grant, filesystem write and network; owner equal to the authenticated account; `get_kernel_session_logs_stream` through the raw client request; a storage redirect through `storageFactory`; a 16 MiB read cap; scrubbed text and the last `maxBytes`. It writes no job row and no project file. 404, 204 or an empty body mean no log yet (empty, nil); a JSON object body is refused. Do not expose it as an agent operation or HTTP route without a receipt decision.
- `RemoveFile` deletes one regular project file under `filesMu`; a missing file is success; invalid names, symlinks and directories are refused. It applies no policy gate: in-process callers apply their own, and there is no HTTP or agent route.
```

  - Append to **Verification**: ``Machine shapes, log reads and file removal: `TestKaggleMachineShape*`, `TestKaggleReadKernelLog*`, `TestKaggleRemoveFilePathSafety`, and in `internal/config` `TestKaggleMachineShapes*` and `TestChibiSenseiTrainingConfig`.``
  - DOX closeout:
    - The root `AGENTS.md` Kaggle routing row ("account grants, durable jobs, project transfers") still covers the scope, so it stays unchanged; B5 owns root guide edits.
    - `internal/config`, `prompts` and `documentation` have no own guide.
    - `internal/server/AGENTS.md` does not change, because no server code changes in B1.

- [ ] 17. Verification. Run these in Git Bash from the worktree root. If Go reports `no such tool "vet"`, prefix the command with `GOTOOLCHAIN=go1.27.4`. Do not set `GOTMPDIR` or `GOCACHE`.

```bash
gofmt -l internal/config internal/kaggle          # expected: no output
git diff --check                                  # expected: no output
go build ./...                                    # expected: exit 0
go test ./internal/config -count=1                # expected: ok  aurago/internal/config (whole package; NormalizeKaggleConfig is CRITICAL)
go test ./internal/kaggle -count=1                # expected: ok  aurago/internal/kaggle
go test ./internal/kaggle -run '(?i)(MachineShape|ReadKernelLog|RemoveFile)' -count=1 -v | grep -E '^(=== RUN|--- (PASS|FAIL)|ok|FAIL)'
                                                  # expected: PASS lines for TestKaggleMachineShapeGrants (14 subtests),
                                                  # TestKaggleMachineShapeSnapshotIntersection, TestKaggleRemoveFilePathSafety,
                                                  # TestKaggleReadKernelLogGrantsOwnerAndReferences (6 subtests),
                                                  # TestKaggleReadKernelLogBodies (9 subtests), TestKaggleReadKernelLogRedirectAndScrub (2 subtests)
go test ./internal/server -run '(?i)(kaggle)' -count=1   # expected: ok
go test ./internal/tools -run '(?i)(kaggle)' -count=1    # expected: ok
go test ./internal/agent -run '(?i)(kaggle|ManualSchema)' -count=1   # expected: ok (tool manual edited)
go test ./ui -run '(?i)(ConfigDerivedValidationRulesCoverTemplate|ConfigChibiSenseiSection|KaggleConfigLimitsMatchBackend|KaggleLocalesComplete)' -count=1
                                                  # expected: ok (proves the template and config changes break no UI test before F1)
```

  Also check the encoding of the touched German file: `head -c 3 documentation/manuals/de/07-configuration.md | od -An -tx1` must not print `ef bb bf`.

  Report in the implementer format: files created, files modified, each command with its PASS/FAIL lines, deviations with their reason, anything left open. Do not commit, stage, stash or switch branches.

---

## Chibi Sensei P2 backend plan, part 2 (B2a)

Part B2a is the Go side of the training contract: plan fields, profile corrections, trainer embed and lock, job spec, notebook, snapshot, log and output parsers, golden files, the security test, proof data and the proof bundle. The sibling part B2b plans `trainer/chibi_train.py` and `trainer/selftest_test.go`. All six tasks below belong to batch B2 and run in the order B2a.1 → B2a.6, before B2b. B2b's tests build jobs with `BuildJobSpec` and parse outputs with these parsers.

Conventions for every task: work only in the worktree. Run Go commands in Git Bash from the worktree root and run only the packages a task names, with `-count=1`. If Go reports `no such tool "vet"`, prefix with `GOTOOLCHAIN=go1.27.4`. Write real UTF-8 characters, never `\u` escapes. Do not commit, stage, stash or switch branches. Do not run GitNexus.

### Contract changes

1. **`.gitattributes` gains two lines** (a root file, outside B2's paths). `trainer/**` and `testdata/**` must keep LF, for two reasons:
   - `ScriptSHA256` is recorded in every run, so it must be the same on Windows and Linux.
   - The golden files and the proof data are compared byte for byte.
2. **Placeholder trainer.** `//go:embed chibi_train.py` does not compile without the file.
   - B2a.1 creates a stdlib-only placeholder, but only when the file is absent.
   - The placeholder contains the marker `CHIBI_TRAINER_PLACEHOLDER = True`. `TestProofBundle` refuses to run while the marker is present.
   - B2b replaces the file.
   - Because `chibisensei` imports `trainer`, B2b's `selftest_test.go` must be in package `trainer_test` to import `chibisensei`.
3. **One test outside B2's paths changes.** `internal/server/chibi_sensei_api_test.go:318` (in `TestChibiSenseiChecksSplitPlan`) asserts `trainable == false` and `reason_key == "chibi_sensei.plan.training_later"`. B2a.2 changes that one condition, because `ComputePlan` now returns `trainable: true`.
4. **The marker file has more fields than the spec.**
   - Name: `r<run12>-marker.json` (D12).
   - Body: `{"v":1,"run_id","format","data_rev","files":{"<split>":{"name","sha256","rows","bytes"}}}`.
   - Added against the spec: `format`, `data_rev` and `bytes`.
   - A split with zero rows has no entry and no file (D11).
5. **Metric lines carry a `kind` field** (`"train"` or `"eval"`).
   - An eval metric puts the validation loss in `loss`. `eval_loss` is also accepted when `loss` is missing.
   - The spec lists `eval_loss` without `kind`.
   - Metric lines need `step` and a loss; a line without either is dropped.
   - The same objects (with or without `v`, `seq`, `ts`, `t`) are the lines of `chibi-metrics.jsonl`.
6. **The wrapper's exit line has no `seq`.** It is written as `@@chibi {"v":1,"t":"exit","code":rc}`, as the spec says. The parser gives it `ExitSeq = 2^53-1`. A `seq` is optional only on `exit` lines; every other line without one is dropped.
7. **Stage names are a fixed list** (`TrainerStages`): `boot, env_check, install, data, load_base, base_eval, train, tuned_eval, save, gguf, manifest`. `LogEvent.Stage()` and `ParseStatus` reject other names.
8. **Manifest paths are fixed per kind** (`ManifestKindFile`). Each kind may appear at most once.

   | Kind | File |
   |---|---|
   | `gguf_tuned` | `chibi-model-q8_0.gguf` |
   | `gguf_base` | `chibi-base-q8_0.gguf` |
   | `adapter` | `chibi-adapter-model.safetensors` |
   | `adapter_config` | `chibi-adapter-config.json` |
   | `metrics` | `chibi-metrics.jsonl` |
   | `eval_samples` | `chibi-eval_samples.jsonl` |
   | `status` | `chibi-status.json` (optional, because the status file is also written in `finally`) |

   The manifest never lists itself.
9. **Eval-sample `id`.** Snapshot lines carry no row id, so a sample's `id` is the 0-based line index in the eval file. `FlexString` accepts it as a string or a number.
10. **Free-recipe lines.**
    - `data.format` is `text` for `free` and `messages` otherwise.
    - P1 allows chat rows in a free project, so every snapshot line is `{"messages":[…]}` or `{"text":"…"}` according to its own row.
    - The trainer must handle each line by its shape. `hyper.loss_on` (`assistant` | `all`) decides the masking.
11. **Lock contents and `deps.pip`.**
    - The initial lock pins only `peft==0.21.3`. `accelerate` is dropped by D2. The converter's own requirements come in through D9.
    - `deps.pip` is the lock's `pip` lines plus `profile.training.extra_pip`, deduplicated and validated together.
12. **`hyper.lora` is omitted (not `null`) for `full`.** `JobSpec.JSON()` indents with one space so golden diffs stay readable.
13. **`MaxTrainSeconds`.** Besides the contract formula, preset `quick` caps it at 1800 s (the spec's "training budget at most 30 min").
14. **Stricter profile validation** (additive):
    - `base.revision` must be a 40-hex sha.
    - `base.auto_class` and `base.auto_class_fallbacks` are class identifiers: at most 4, distinct, none equal to `auto_class`.
    - `training.lora.target_modules`: 1 to 32 distinct `[A-Za-z0-9_]` names.
    - `training.extra_pip`: exact `name==version` pins.
    - `gguf.out_type` must be `q8_0`.
15. **Added helpers** (B3 and B4 should reuse them rather than re-derive):
    - Names: `SnapshotFileName`, `MarkerFileName`, `DatasetSlug(runID)` = `chibi-data-<run12>`, `NotebookSlug(projectID, runID)` = `chibi-<first 8 hex of the project id>-<run12>`. The last one is my reading of the contract's example `chibi-1a2b3c4d-0123456789ab`.
    - Formats and constants: `FormatFor`, `ExitSeq`, `TrainerStages`, `ManifestKindFile`, the `Out*` file-name constants.
    - Parsing: `LogEvent.ExitCode`, `FlexString`, `ValidManifestPath`.
    - Trainer package: `trainer.ParseLock`, `Lock.Validate`, `trainer.ValidPin`.
16. **Proof bundle extras.**
    - An optional `CHIBI_PROOF_TIMEOUT_MINUTES` (default 60) stands in for `kaggle.remote_timeout_minutes`.
    - Output file names: `r<run12>-job.json`, `r<run12>-notebook.ipynb`, `bundle.json`.
    - `owner` is an empty field the coordinator fills in.
17. **DOX is deferred to B5**, which owns the `AGENTS.md` files. B5 must record in `internal/chibisensei/AGENTS.md`:
    - the trainer embed and the lock format;
    - the golden and proof-data `-update` commands;
    - the `.gitattributes` lines;
    - `TestProofBundle`;
    - the stage list and the output names.

### File map

| File | Batch | Responsibility |
|---|---|---|
| `.gitattributes` | B2 (B2a.1) | LF for trainer and testdata |
| `internal/chibisensei/trainer/embed.go` | B2 (B2a.1) | embed script and lock, `Script`, `ScriptSHA256`, `Lock`, `LoadLock`, `ParseLock`, `ValidPin` |
| `internal/chibisensei/trainer/requirements.lock` | B2 (B2a.1) | pins |
| `internal/chibisensei/trainer/embed_test.go` | B2 (B2a.1) | lock parser and embed tests |
| `internal/chibisensei/trainer/chibi_train.py` | B2 (B2a.1 placeholder; B2b owns it) | trainer |
| `internal/chibisensei/profiles.go`, `profiles/*.json`, `profiles_test.go` | B2 (B2a.2) | `auto_class_fallbacks`, name validation, D6 corrections |
| `internal/chibisensei/plan.go`, `plan_test.go` | B2 (B2a.2) | new plan fields, trainable |
| `internal/server/chibi_sensei_api_test.go` | B2 (B2a.2) | one assertion (contract change 3) |
| `internal/chibisensei/snapshot.go`, `snapshot_test.go` | B2 (B2a.3) | `FreezeSnapshot`, `Run12`, names, slugs, marker, `marshalStable` |
| `internal/chibisensei/jobspec.go`, `jobspec_test.go` | B2 (B2a.4) | `JobSpec`, `BuildJobSpec`, `JSON` |
| `internal/chibisensei/notebook.go`, `notebook_test.go` | B2 (B2a.4) | `BuildNotebook` |
| `internal/chibisensei/golden_test.go`, `security_test.go` | B2 (B2a.4) | golden files, `-update` flag, payload security test |
| `internal/chibisensei/testdata/golden/job-{gemma3-270m,qwen35-0p8b,lfm25-1p2b}.json`, `notebook.ipynb` | B2 (B2a.4) | golden files (generated) |
| `internal/chibisensei/logparse.go`, `logparse_test.go` | B2 (B2a.5) | `ParseKernelLog`, `LogEvent`, `Metric`, `Stage`, `Error`, `ExitCode` |
| `internal/chibisensei/outputs.go`, `outputs_test.go` | B2 (B2a.5) | `Status`, `Manifest`, metrics file, eval samples |
| `internal/chibisensei/testdata/logs/*`, `testdata/outputs/*` | B2 (B2a.5) | parser fixtures |
| `internal/chibisensei/proofdata_test.go`, `testdata/proof/{assistant,specialist,free}.jsonl` | B2 (B2a.6) | proof data generator and files |
| `internal/chibisensei/proofbundle_test.go` | B2 (B2a.6) | gated `TestProofBundle` |

---

#### Task B2a.1: Trainer package, lock file, line endings

**Files**
- Create:
  - `internal/chibisensei/trainer/embed.go`, `internal/chibisensei/trainer/requirements.lock`, `internal/chibisensei/trainer/embed_test.go`
  - `internal/chibisensei/trainer/chibi_train.py`, only if it does not exist yet
- Modify: `.gitattributes` (append after line 111)

**Existing symbols modified:** none

**Steps**

- [ ] 1. Append to `.gitattributes`:

```
# Chibi Sensei: the trainer's bytes are hashed into every run; golden files and proof data are compared byte for byte.
internal/chibisensei/trainer/** text eol=lf
internal/chibisensei/testdata/** text eol=lf
```

- [ ] 2. If `internal/chibisensei/trainer/chibi_train.py` does not exist, create this placeholder. Leave an existing file untouched.

```python
#!/usr/bin/env python3
"""Chibi Sensei trainer placeholder. The trainer task of batch B2 replaces this file."""
import sys

CHIBI_TRAINER_PLACEHOLDER = True

if __name__ == "__main__":
    print("chibi_train.py placeholder: the trainer is not built yet", file=sys.stderr)
    sys.exit(2)
```

- [ ] 3. Create `internal/chibisensei/trainer/requirements.lock`:

```
# Chibi Sensei trainer pins, parsed by trainer.ParseLock. One "key value" pair per line; "#" starts a comment.
# pip <name>==<version>   installed with pip before training (repeatable; never torch or transformers)
# transformers <version>  reinstalled only when it differs from the preinstalled one; empty keeps it
# llama_cpp_tag <tag>     converter release; must match the local runtime build
# llama_cpp_url <url>     the codeload tarball of that tag
# llama_cpp_sha256 <hex>  empty until a proof run records it; the trainer then logs the measured value and warns
pip peft==0.21.3
transformers
llama_cpp_tag b9994
llama_cpp_url https://codeload.github.com/ggml-org/llama.cpp/tar.gz/refs/tags/b9994
llama_cpp_sha256
```

- [ ] 4. Create `internal/chibisensei/trainer/embed.go`:

```go
// Package trainer embeds the Chibi Sensei training script and its dependency pins. It must never import
// aurago/internal/chibisensei, which imports it.
package trainer

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

//go:embed chibi_train.py
var script []byte

//go:embed requirements.lock
var lockText []byte

// LlamaCppURLPrefix is the only allowed source of the converter tarball; the tag is appended.
const LlamaCppURLPrefix = "https://codeload.github.com/ggml-org/llama.cpp/tar.gz/refs/tags/"

// Script returns a copy of the embedded chibi_train.py.
func Script() []byte { return bytes.Clone(script) }

// ScriptSHA256 is the hex sha256 of the embedded script.
func ScriptSHA256() string {
	sum := sha256.Sum256(script)
	return hex.EncodeToString(sum[:])
}

// Lock is the parsed requirements.lock.
type Lock struct {
	Pip            []string // "name==version" lines
	Transformers   string   // "" keeps the preinstalled version
	LlamaCppTag    string
	LlamaCppURL    string
	LlamaCppSHA256 string // "" = not pinned yet: the trainer logs the measured value and warns
}

var (
	pinPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}==[0-9][0-9A-Za-z.+!-]{0,31}$`)
	versionPattern = regexp.MustCompile(`^[0-9][0-9A-Za-z.+!-]{0,31}$`)
	tagPattern     = regexp.MustCompile(`^b[0-9]{1,6}$`)
	sha256Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ValidPin reports whether s is an exact pip pin "name==version" without options, URLs or markers.
func ValidPin(s string) bool { return pinPattern.MatchString(s) }

// LoadLock parses the embedded requirements.lock.
func LoadLock() (Lock, error) { return ParseLock(lockText) }

// ParseLock parses the lock format. Keys: pip (repeatable), transformers, llama_cpp_tag, llama_cpp_url and
// llama_cpp_sha256 (each exactly once; transformers and llama_cpp_sha256 may have an empty value).
func ParseLock(b []byte) (Lock, error) {
	var l Lock
	seen := map[string]bool{}
	for i, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) > 2 {
			return Lock{}, fmt.Errorf("requirements.lock line %d: expected \"key value\"", i+1)
		}
		key, value := f[0], ""
		if len(f) == 2 {
			value = f[1]
		}
		if key != "pip" {
			if seen[key] {
				return Lock{}, fmt.Errorf("requirements.lock line %d: duplicate key %q", i+1, key)
			}
			seen[key] = true
		}
		switch key {
		case "pip":
			l.Pip = append(l.Pip, value)
		case "transformers":
			l.Transformers = value
		case "llama_cpp_tag":
			l.LlamaCppTag = value
		case "llama_cpp_url":
			l.LlamaCppURL = value
		case "llama_cpp_sha256":
			l.LlamaCppSHA256 = value
		default:
			return Lock{}, fmt.Errorf("requirements.lock line %d: unknown key %q", i+1, key)
		}
	}
	for _, k := range []string{"transformers", "llama_cpp_tag", "llama_cpp_url", "llama_cpp_sha256"} {
		if !seen[k] {
			return Lock{}, fmt.Errorf("requirements.lock: missing key %q", k)
		}
	}
	if err := l.Validate(); err != nil {
		return Lock{}, fmt.Errorf("requirements.lock: %w", err)
	}
	return l, nil
}

// Validate checks the pins: exact pip pins, no torch or transformers pip line, no package twice, the
// converter URL derived from the tag and an optional sha256.
func (l Lock) Validate() error {
	names := map[string]bool{}
	for _, p := range l.Pip {
		if !ValidPin(p) {
			return fmt.Errorf("pip %q is not an exact name==version pin", p)
		}
		name := strings.ToLower(strings.NewReplacer("_", "-", ".", "-").Replace(strings.SplitN(p, "==", 2)[0]))
		if name == "torch" || name == "transformers" {
			return fmt.Errorf("pip must not pin %s", name)
		}
		if names[name] {
			return fmt.Errorf("pip pins %s twice", name)
		}
		names[name] = true
	}
	if l.Transformers != "" && !versionPattern.MatchString(l.Transformers) {
		return fmt.Errorf("transformers %q is not a version", l.Transformers)
	}
	if !tagPattern.MatchString(l.LlamaCppTag) {
		return fmt.Errorf("llama_cpp_tag %q is not a b<number> tag", l.LlamaCppTag)
	}
	if l.LlamaCppURL != LlamaCppURLPrefix+l.LlamaCppTag {
		return fmt.Errorf("llama_cpp_url must be %s%s", LlamaCppURLPrefix, l.LlamaCppTag)
	}
	if l.LlamaCppSHA256 != "" && !sha256Pattern.MatchString(l.LlamaCppSHA256) {
		return fmt.Errorf("llama_cpp_sha256 is not 64 lower-case hex characters")
	}
	return nil
}
```

- [ ] 5. Create `internal/chibisensei/trainer/embed_test.go` (package `trainer`):
  - `TestLoadLockEmbedded`: `LoadLock()` succeeds with `Pip == ["peft==0.21.3"]`, `Transformers == ""`, tag `b9994`, URL `LlamaCppURLPrefix+"b9994"` and `LlamaCppSHA256 == ""`.
  - `TestParseLockRejects`: table-driven. Each row is a lock text and a substring the error must contain. Base the rows on a valid five-line lock with one change each:

    | Change | Expected error substring |
    |---|---|
    | unknown key `foo bar` | `unknown key` |
    | `transformers` twice | `duplicate key` |
    | `pip peft>=0.21` | `exact name==version` |
    | `pip torch==2.9.0` | `must not pin torch` |
    | `pip peft==0.21.3` twice | `twice` |
    | `llama_cpp_tag master` | `b<number>` |
    | URL with tag `b9993` | `llama_cpp_url must be` |
    | `llama_cpp_sha256 XYZ` | `64 lower-case hex` |
    | `llama_cpp_url` line missing | `missing key` |
    | three fields on a line | `expected "key value"` |

  - `TestScriptEmbed`:
    - `ScriptSHA256()` matches `^[0-9a-f]{64}$` and equals the hex sha256 of `Script()`;
    - changing a byte of `Script()` leaves a second `Script()` call unchanged (it returns a copy);
    - `ValidPin("peft==0.21.3")` is true; `ValidPin("peft==0.21.3 --index-url x")` and `ValidPin("git+https://x")` are false.

- [ ] 6. Verify:

```
go build ./...
go test ./internal/chibisensei/trainer -count=1
```

Expected: build OK; `ok aurago/internal/chibisensei/trainer`.

---

#### Task B2a.2: Profile corrections and plan fields

**Files**
- Modify:
  - `internal/chibisensei/profiles.go:3-12` (imports), `:48-54` (`ProfileBase`), `:293-296` (end of `ValidateProfile`)
  - `internal/chibisensei/profiles/qwen35-0p8b.json` (whole file), `profiles/gemma3-270m.json:6`, `profiles/lfm25-1p2b.json:6,11`
  - `internal/chibisensei/profiles_test.go:44-70`
  - `internal/chibisensei/plan.go` (whole file), `internal/chibisensei/plan_test.go:28-42`
  - `internal/server/chibi_sensei_api_test.go:318`

**Existing symbols modified:** `ProfileBase`, `ValidateProfile`, `Plan`, `ComputePlan`, `TestValidateProfileRules`, `TestPresetsAndPlan`, `TestChibiSenseiChecksSplitPlan`

**Steps**

- [ ] 1. In `profiles.go`, add `"regexp"` and `"aurago/internal/chibisensei/trainer"` to the imports. Replace `ProfileBase` (`:48-54`) with:

```go
type ProfileBase struct {
	Source             string   `json:"source"`
	Repo               string   `json:"repo"`
	Revision           string   `json:"revision"`
	AutoClass          string   `json:"auto_class"`
	AutoClassFallbacks []string `json:"auto_class_fallbacks,omitempty"` // tried in order after AutoClass
	Gated              bool     `json:"gated"`
}
```

- [ ] 2. In `ValidateProfile`, replace the final `return nil` (`:296`) with `return validateProfileNames(p)`. Add below the function:

```go
var (
	revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	pyClassPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,79}$`)
	modulePattern   = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
)

const maxAutoClassFallbacks = 4

// validateProfileNames checks the values the trainer hands to Python: the pinned revision, the class names it
// resolves with getattr(transformers, name), the LoRA module names, the extra pip pins and the GGUF type.
func validateProfileNames(p *Profile) error {
	if !revisionPattern.MatchString(p.Base.Revision) {
		return profileErr("base.revision", "must be a pinned 40-character commit sha")
	}
	if !pyClassPattern.MatchString(p.Base.AutoClass) {
		return profileErr("base.auto_class", "must be a transformers class name")
	}
	if len(p.Base.AutoClassFallbacks) > maxAutoClassFallbacks {
		return profileErr("base.auto_class_fallbacks", "at most 4 entries")
	}
	seen := map[string]bool{p.Base.AutoClass: true}
	for _, c := range p.Base.AutoClassFallbacks {
		if !pyClassPattern.MatchString(c) || seen[c] {
			return profileErr("base.auto_class_fallbacks", "entries must be distinct class names other than auto_class")
		}
		seen[c] = true
	}
	mods := p.Training.LoRA.TargetModules
	if len(mods) == 0 || len(mods) > 32 {
		return profileErr("training.lora.target_modules", "needs 1 to 32 module names")
	}
	seenMod := map[string]bool{}
	for _, m := range mods {
		if !modulePattern.MatchString(m) || seenMod[m] {
			return profileErr("training.lora.target_modules", "entries must be distinct module names")
		}
		seenMod[m] = true
	}
	for _, pin := range p.Training.ExtraPip {
		if !trainer.ValidPin(pin) {
			return profileErr("training.extra_pip", `entries must be exact "name==version" pins`)
		}
	}
	if p.GGUF.OutType != "q8_0" {
		return profileErr("gguf.out_type", `must be "q8_0"`)
	}
	return nil
}
```

- [ ] 3. Replace `profiles/qwen35-0p8b.json` with the following. The changes are the revision, `auto_class`, `auto_class_fallbacks`, the target modules and `extra_pip` (D6), plus `base.auto_class_fallbacks` in `unverified`. Every other value is unchanged.

```json
{"id":"qwen35-0p8b","display_name":"Qwen3.5 0.8B","schema":1,"status":"experimental","verified":null,
 "unverified":["base.revision","base.auto_class","base.auto_class_fallbacks","arch.llama_cpp_arch","gguf.expected_arch",
   "training.lora.target_modules","training.batch_tokens","training.t4_tokens_per_second","training.t4_gen_tokens_per_second",
   "training.min_vram_gb","training.extra_pip","gguf.estimated_mb"],
 "license":{"id":"apache-2.0","name":"Apache License 2.0","url":"https://www.apache.org/licenses/LICENSE-2.0","version":"2.0",
   "requires_consent":false,"note_key":"chibi_sensei.license.apache"},
 "base":{"source":"hf","repo":"Qwen/Qwen3.5-0.8B","revision":"2fc06364715b967f1860aea9cf38778875588b17",
   "auto_class":"Qwen3_5ForCausalLM",
   "auto_class_fallbacks":["AutoModelForCausalLM","AutoModelForMultimodalLM","AutoModelForImageTextToText"],"gated":false},
 "arch":{"llama_cpp_arch":"qwen35","context_max":262144,"params_b":0.8,"text_only":true,"languages":["*"]},
 "chat":{"system_role":"native","stop":["<|im_end|>"]},
 "training":{"method_default":"lora","methods":["lora"],"dtype":"float32",
   "lora":{"r":16,"alpha":16,"dropout":0.05,"target_modules":["q_proj","k_proj","v_proj","o_proj","gate_proj","up_proj",
     "down_proj","in_proj_qkv","in_proj_z","out_proj"]},
   "lr":{"lora":0.0002},"max_seq_cap":2048,"batch_tokens":4096,"grad_checkpointing":true,
   "t4_tokens_per_second":2500,"t4_gen_tokens_per_second":30,"min_gpu_capability":"7.0","min_vram_gb":14,
   "extra_pip":[]},
 "sampling":{"temperature":0.7,"top_p":0.8,"top_k":20,"repeat_penalty":1.0,"presence_penalty":1.5},
 "eval":{"max_new_tokens_assistant":256,"max_new_tokens_specialist":64},
 "gguf":{"out_type":"q8_0","expected_arch":"qwen35","estimated_mb":900}}
```

- [ ] 4. Two more profile edits:
  - `gemma3-270m.json:6`: `"revision":"main"` becomes `"revision":"23cf460f6bb16954176b3ddcc8d4f250501458a9"`.
  - `lfm25-1p2b.json`:
    - `:6`: `"revision":"main"` becomes `"revision":"b9d816ab510f9a28dca37cb182e9da8b0226e13c"`;
    - `:11`: `target_modules` becomes `["q_proj","k_proj","v_proj","out_proj","w1","w2","w3","in_proj"]`.

  Their `unverified` lists stay as they are: `base.revision` and `training.lora.target_modules` stay listed until the proof run.

- [ ] 5. `profiles_test.go`: add these rows to the `cases` table of `TestValidateProfileRules`:

| Name | Mutation | Expected error substring |
|---|---|---|
| `revision not pinned` | `Base.Revision = "main"` | `base.revision` |
| `auto_class` | `Base.AutoClass = "os.system"` | `base.auto_class` |
| `fallback repeats auto_class` | `Base.AutoClassFallbacks = []string{"Qwen3_5ForCausalLM"}` | `base.auto_class_fallbacks` |
| `too many fallbacks` | `Base.AutoClassFallbacks = []string{"A","B","C","D","E"}` | `base.auto_class_fallbacks` |
| `target module` | `Training.LoRA.TargetModules = []string{"q_proj; rm"}` | `training.lora.target_modules` |
| `extra pip unpinned` | `Training.ExtraPip = []string{"flash-linear-attention"}` | `training.extra_pip` |
| `out type` | `GGUF.OutType = "f16"` | `gguf.out_type` |

   Also add `TestLaunchProfilePins`:
   - the three revisions above are exact;
   - qwen `AutoClass == "Qwen3_5ForCausalLM"`, its fallbacks equal the three names in order, its `TargetModules` equal the 10 names, and `ExtraPip` is empty;
   - lfm `TargetModules` equal the 8 names;
   - every profile is `experimental` with `Verified == nil`;
   - qwen `Unverified` contains `base.auto_class_fallbacks`.

- [ ] 6. Replace `plan.go` with:

```go
package chibisensei

import "math"

const (
	minMaxTrainSeconds   = 300  // the trainer always gets five minutes
	quickMaxTrainSeconds = 1800 // preset quick trains at most 30 minutes
	artifactSlackMB      = 50   // adapter, metrics, samples, status and manifest
)

type PlanSplit struct {
	Train int `json:"train"`
	Val   int `json:"val"`
	Eval  int `json:"eval"`
}

type Plan struct {
	Preset                string    `json:"preset"`
	ProfileID             string    `json:"profile_id"`
	Method                string    `json:"method"`
	Hyper                 Hyper     `json:"hyper"`
	Split                 PlanSplit `json:"split"`
	EstimateMinutes       int       `json:"estimate_minutes"`
	SessionTimeoutMinutes int       `json:"session_timeout_minutes"`
	EvalSamples           int       `json:"eval_samples"`
	MaxNewTokens          int       `json:"max_new_tokens"`         // profile.Eval by recipe
	MaxTrainSeconds       int       `json:"max_train_seconds"`      // session minus 1.5 x the non-training estimate, >= 300
	NeedBaseGGUF          bool      `json:"need_base_gguf"`
	ArtifactsEstimatedMB  int       `json:"artifacts_estimated_mb"` // tuned GGUF (+ base GGUF) + 50
	DataRev               int64     `json:"data_rev"`
	Trainable             bool      `json:"trainable"`
	ReasonKey             string    `json:"reason_key"`
}

type PlanInput struct {
	Project              Project
	Profile              *Profile
	Rows                 []SplitRow
	Split                SplitResult
	Preset               string // "" = the project's preset
	RemoteTimeoutMinutes int
	NeedBaseGGUF         bool
}

// ComputePlan derives the training plan; a computed plan is trainable. Overrides that do not fit the profile
// are a *ValidationError.
func ComputePlan(in PlanInput) (Plan, error) {
	p := in.Profile
	if is := ValidateOverrides(in.Project.Overrides, p); len(is) > 0 {
		return Plan{}, &ValidationError{Issues: is}
	}
	preset := in.Preset
	if preset == "" {
		preset = in.Project.Preset
	}
	if preset == "" {
		preset = "balanced"
	}
	method := in.Project.Overrides.Method
	if method == "" {
		method = p.Training.MethodDefault
	}
	if p.Training.LR[method] <= 0 && in.Project.Overrides.LearningRate <= 0 {
		return Plan{}, &ValidationError{Issues: []Issue{badOverride("method", "The profile does not support this method.")}}
	}
	var trainEst []int
	for _, r := range in.Rows {
		if in.Split.Assign[r.ID] == "train" {
			trainEst = append(trainEst, r.EstTokens)
		}
	}
	p95 := Percentile(trainEst, 0.95)
	hyper, evalSamples := PresetHyper(p, preset, method, len(trainEst), p95, in.Project.Overrides)
	var trainTok float64
	for _, e := range trainEst {
		trainTok += float64(min(e, hyper.MaxSeqLen))
	}
	maxNew := p.Eval.MaxNewTokensAssistant
	if in.Project.Recipe == RecipeSpecialist {
		maxNew = p.Eval.MaxNewTokensSpecialist
	}
	samples := min(evalSamples, in.Split.Counts.Eval)
	trainMin := trainTok * float64(hyper.Epochs) / p.Training.T4TokensPerSecond / 60
	if preset == "quick" && trainMin > 30 {
		trainMin = 30
	}
	evalMin := float64(samples) * 2 * float64(maxNew) / p.Training.T4GenTokensPerSecond / 60
	nonTrain := 8 + 3 + evalMin + 6 // environment and install, base download and load, evaluation, merge and GGUF
	if in.NeedBaseGGUF {
		nonTrain += 3
	}
	estimate := int(math.Ceil(nonTrain + trainMin - 1e-9))
	session := int(math.Ceil(1.5*float64(estimate) + 15))
	if session < 30 {
		session = 30
	}
	if in.RemoteTimeoutMinutes > 0 && session > in.RemoteTimeoutMinutes {
		session = in.RemoteTimeoutMinutes
	}
	maxTrain := session*60 - int(math.Ceil(1.5*nonTrain*60-1e-6))
	if preset == "quick" && maxTrain > quickMaxTrainSeconds {
		maxTrain = quickMaxTrainSeconds
	}
	if maxTrain < minMaxTrainSeconds {
		maxTrain = minMaxTrainSeconds
	}
	artifacts := p.GGUF.EstimatedMB + artifactSlackMB
	if in.NeedBaseGGUF {
		artifacts += p.GGUF.EstimatedMB
	}
	return Plan{Preset: preset, ProfileID: p.ID, Method: method, Hyper: hyper,
		Split:           PlanSplit{Train: in.Split.Counts.Train, Val: in.Split.Counts.Val, Eval: in.Split.Counts.Eval},
		EstimateMinutes: estimate, SessionTimeoutMinutes: session, EvalSamples: samples, MaxNewTokens: maxNew,
		MaxTrainSeconds: maxTrain, NeedBaseGGUF: in.NeedBaseGGUF, ArtifactsEstimatedMB: artifacts,
		DataRev: in.Project.DataRev, Trainable: true, ReasonKey: ""}, nil
}
```

- [ ] 7. Update `plan_test.go` and `internal/server/chibi_sensei_api_test.go`.

   (a) Replace `plan_test.go:32-34` with:

```go
	if !b.Trainable || b.ReasonKey != "" || b.DataRev != 5 || b.EvalSamples != 20 || b.MaxNewTokens != 256 ||
		b.MaxTrainSeconds != 1048 || !b.NeedBaseGGUF || b.ArtifactsEstimatedMB != 1850 {
		t.Fatalf("plan flags = %+v", b)
	}
```

   (b) Extend `plan_test.go` with these checks. Explanation of the expected numbers for qwen balanced:
   - 371 train rows of 180 tokens; eval samples = min(100, 20) = 20;
   - the evaluation part is 20 × 2 × 256 / 30 / 60 = 5.689 min;
   - the non-training estimate is 8 + 3 + 6 + 3 + 5.689 = 25.689 min, and 1.5 × 25.689 × 60 = 2312 s;
   - session 56 min = 3360 s, so `MaxTrainSeconds` = 3360 − 2312 = 1048.

   | Case | Expected |
   |---|---|
   | quick | `MaxTrainSeconds == 1048` |
   | thorough | `MaxTrainSeconds == 1108` (session 57) |
   | balanced with `RemoteTimeoutMinutes: 30` | `SessionTimeoutMinutes == 30`, `MaxTrainSeconds == 300` |
   | balanced with `NeedBaseGGUF: false` | `EstimateMinutes == 24`, `SessionTimeoutMinutes == 51`, `MaxTrainSeconds == 1018`, `ArtifactsEstimatedMB == 950` |
   | quick cap: 3000 train rows of `EstTokens: 2000`, `SplitCounts{Train: 3000, Eval: 20}`, `RemoteTimeoutMinutes: 720` | `EstimateMinutes == 56`, `SessionTimeoutMinutes == 99`, `MaxTrainSeconds == 1800` |

   (c) In `internal/server/chibi_sensei_api_test.go:318`, change the condition to:

   `if plan["trainable"] != true || plan["reason_key"] != "" || plan["max_train_seconds"] == nil ||`

- [ ] 8. Verify:

```
go build ./...
go test ./internal/chibisensei/... -count=1
go test ./internal/server -run '(?i)(TestChibiSenseiChecksSplitPlan)' -count=1
```

Expected: all `ok`. In the `-v` output, `TestPresetsAndPlan`, `TestValidateProfileRules`, `TestLaunchProfilePins` and `TestCatalogLoadsLaunchProfiles` show PASS.

---

#### Task B2a.3: Snapshot

**Files**
- Create: `internal/chibisensei/snapshot.go`, `internal/chibisensei/snapshot_test.go`

**Existing symbols modified:** none. It calls `LoadSplitRows`, `ComputeSplit`, `EachRow`, `ExportLine` and `GetProject` unchanged.

I checked `validFile` (`internal/kaggle/transfers.go:37-43`). It refuses `/ \ :` and NUL, a leading `.`, a trailing `.` or space, length 240 or more, and the reserved DOS stems. `upload` sets `fileName = req.File` (`:336`), so the dataset file names equal the project file names (D12). `r<12 hex>-{train,val,eval}.jsonl` and `r<12 hex>-marker.json` pass every rule.

**Steps**

- [ ] 1. Create `snapshot.go`:

```go
package chibisensei

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"strings"
)

// SnapshotSplits is the order of the split files in the marker, the data hash and Snapshot.Files.
var SnapshotSplits = []string{"train", "val", "eval"}

type SnapshotFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Rows   int    `json:"rows"`
	Bytes  int64  `json:"bytes"`
}

type Snapshot struct {
	RunID   string
	Dir     string
	Files   map[string]SnapshotFile // keys train, val, eval; a split with zero rows is absent
	Marker  SnapshotFile            // Rows is 0
	Format  string                  // messages | text
	DataRev int64
	DataSHA string // sha256 over "<split>:<sha256>\n" for the present splits in the order train, val, eval
	Bytes   int64  // sum of all files
}

type snapshotMarker struct {
	V       int                     `json:"v"`
	RunID   string                  `json:"run_id"`
	Format  string                  `json:"format"`
	DataRev int64                   `json:"data_rev"`
	Files   map[string]SnapshotFile `json:"files"`
}

// freezeAfterLoad is a test seam called after the split rows are loaded; nil in production.
var freezeAfterLoad func()

// Run12 is the first 12 hex characters of an ID with the dashes removed.
func Run12(runID string) string {
	h := strings.ToLower(strings.ReplaceAll(runID, "-", ""))
	if len(h) > 12 {
		h = h[:12]
	}
	return h
}

// SnapshotFileName is the staged and dataset file name of one split (D12).
func SnapshotFileName(runID, split string) string { return "r" + Run12(runID) + "-" + split + ".jsonl" }

// MarkerFileName is the staged and dataset file name of the marker; job.json names it in data.marker.
func MarkerFileName(runID string) string { return "r" + Run12(runID) + "-marker.json" }

// DatasetSlug is the Kaggle dataset slug of a run.
func DatasetSlug(runID string) string { return "chibi-data-" + Run12(runID) }

// NotebookSlug is the Kaggle notebook slug: chibi-<first 8 hex of the project id>-<run12>.
func NotebookSlug(projectID, runID string) string {
	p := strings.ToLower(strings.ReplaceAll(projectID, "-", ""))
	if len(p) > 8 {
		p = p[:8]
	}
	return "chibi-" + p + "-" + Run12(runID)
}

// FormatFor names the data format of a recipe: text for free, messages otherwise ("" when unknown).
func FormatFor(recipe string) string {
	switch recipe {
	case RecipeAssistant, RecipeSpecialist:
		return "messages"
	case RecipeFree:
		return "text"
	}
	return ""
}

// marshalStable encodes v in struct order without HTML escaping, indented by one space, with one trailing newline.
func marshalStable(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type splitFile struct {
	name string
	f    *os.File
	h    hash.Hash
	bw   *bufio.Writer
	rows int
	n    int64
}

// FreezeSnapshot writes the split files and the marker into dir and returns their facts. It fails with
// ErrDataChanged when the rows changed while it read them. Excluded rows are skipped; a split with zero rows
// gets no file (D11). Older files of the same run are replaced; on failure none of the run's files remain.
func (s *Store) FreezeSnapshot(ctx context.Context, projectID, runID, dir string) (Snapshot, error) {
	if !ValidID(runID) {
		return Snapshot{}, fmt.Errorf("snapshot: invalid run id")
	}
	srows, proj, err := s.LoadSplitRows(ctx, projectID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: load rows: %w", err)
	}
	if freezeAfterLoad != nil {
		freezeAfterLoad()
	}
	format := FormatFor(proj.Recipe)
	if format == "" {
		return Snapshot{}, fmt.Errorf("snapshot: unknown recipe %q", proj.Recipe)
	}
	sp := ComputeSplit(srows, proj.SplitSeed, proj.TaskKind == "classify")
	if sp.Counts.Train == 0 {
		return Snapshot{}, &ValidationError{Issues: []Issue{issue("rows", "snapshot.no_train_rows", "The project has no training rows.")}}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: create directory: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: open directory: %w", err)
	}
	defer root.Close()
	names := []string{MarkerFileName(runID)}
	for _, split := range SnapshotSplits {
		names = append(names, SnapshotFileName(runID, split))
	}
	cleanup := func() error {
		for _, n := range names {
			if err := removeQuiet(root, n); err != nil {
				return err
			}
			if err := removeQuiet(root, n+".part"); err != nil {
				return err
			}
		}
		return nil
	}
	if err := cleanup(); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: remove old files: %w", err)
	}
	snap, err := s.writeSnapshot(ctx, root, proj, sp, runID, format)
	if err != nil {
		_ = cleanup()
		return Snapshot{}, err
	}
	snap.Dir = dir
	return snap, nil
}

func (s *Store) writeSnapshot(ctx context.Context, root *os.Root, proj Project, sp SplitResult, runID, format string) (Snapshot, error) {
	want := map[string]int{"train": sp.Counts.Train, "val": sp.Counts.Val, "eval": sp.Counts.Eval}
	files := map[string]*splitFile{}
	defer func() {
		for _, sf := range files {
			if sf.f != nil {
				sf.f.Close()
			}
		}
	}()
	for _, split := range SnapshotSplits {
		if want[split] == 0 {
			continue
		}
		name := SnapshotFileName(runID, split)
		fh, err := root.OpenFile(name+".part", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return Snapshot{}, fmt.Errorf("snapshot: create %s: %w", name, err)
		}
		sf := &splitFile{name: name, f: fh, h: sha256.New()}
		sf.bw = bufio.NewWriterSize(io.MultiWriter(fh, sf.h), 64<<10)
		files[split] = sf
	}
	err := s.EachRow(ctx, proj.ID, func(r Row) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		sf := files[sp.Assign[r.ID]] // excluded rows and rows added after the load have no file
		if sf == nil {
			return nil
		}
		line := ExportLine(r)
		sf.rows++
		sf.n += int64(len(line))
		_, err := sf.bw.Write(line)
		return err
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: write rows: %w", err)
	}
	now, err := s.GetProject(ctx, proj.ID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: recheck: %w", err)
	}
	if now.DataRev != proj.DataRev {
		return Snapshot{}, ErrDataChanged
	}
	snap := Snapshot{RunID: runID, Files: map[string]SnapshotFile{}, Format: format, DataRev: proj.DataRev}
	for _, split := range SnapshotSplits {
		sf := files[split]
		if sf == nil {
			continue
		}
		if sf.rows != want[split] {
			return Snapshot{}, ErrDataChanged
		}
		if err := sf.bw.Flush(); err != nil {
			return Snapshot{}, fmt.Errorf("snapshot: flush %s: %w", sf.name, err)
		}
		if err := sf.f.Sync(); err != nil {
			return Snapshot{}, fmt.Errorf("snapshot: sync %s: %w", sf.name, err)
		}
		err := sf.f.Close()
		sf.f = nil
		if err != nil {
			return Snapshot{}, fmt.Errorf("snapshot: close %s: %w", sf.name, err)
		}
		if err := root.Rename(sf.name+".part", sf.name); err != nil {
			return Snapshot{}, fmt.Errorf("snapshot: publish %s: %w", sf.name, err)
		}
		snap.Files[split] = SnapshotFile{Name: sf.name, SHA256: hex.EncodeToString(sf.h.Sum(nil)), Rows: sf.rows, Bytes: sf.n}
		snap.Bytes += sf.n
	}
	marker, err := marshalStable(snapshotMarker{V: 1, RunID: runID, Format: format, DataRev: proj.DataRev, Files: snap.Files})
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: encode marker: %w", err)
	}
	mname := MarkerFileName(runID)
	if err := writeRootFile(root, mname, marker); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: write marker: %w", err)
	}
	sum := sha256.Sum256(marker)
	snap.Marker = SnapshotFile{Name: mname, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(marker))}
	snap.Bytes += snap.Marker.Bytes
	snap.DataSHA = snapshotDataSHA(snap.Files)
	return snap, nil
}

func snapshotDataSHA(files map[string]SnapshotFile) string {
	h := sha256.New()
	for _, split := range SnapshotSplits {
		if f, ok := files[split]; ok {
			fmt.Fprintf(h, "%s:%s\n", split, f.SHA256)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func removeQuiet(root *os.Root, name string) error {
	if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// writeRootFile publishes data under root through a synced .part file.
func writeRootFile(root *os.Root, name string, data []byte) error {
	f, err := root.OpenFile(name+".part", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return root.Rename(name+".part", name)
}
```

- [ ] 2. Create `snapshot_test.go`. Use `newTestStore`, `CreateProject` and `AddRows` with `AddOptions{Source: "manual", BatchKind: "add", MaxRows: 50000}`.
  - `TestRun12AndNames` (table):
    - `Run12("0123abcd-4567-4890-abcd-ef0123456789") == "0123abcd4567"`;
    - `SnapshotFileName(id,"val") == "r0123abcd4567-val.jsonl"`, `MarkerFileName(id) == "r0123abcd4567-marker.json"`;
    - `DatasetSlug(id) == "chibi-data-0123abcd4567"`;
    - `NotebookSlug("1a2b3c4d-0000-4000-8000-000000000000", id) == "chibi-1a2b3c4d-0123abcd4567"`;
    - all four file names match `^r[0-9a-f]{12}-(train|val|eval)\.jsonl$|^r[0-9a-f]{12}-marker\.json$`;
    - `FormatFor`: assistant → messages, free → text, x → "".
  - `TestFreezeSnapshotAssistant`:
    - Setup: an assistant project with 240 unique chat rows (`chatInput(fmt.Sprintf("question %d", i), fmt.Sprintf("answer number %d is long enough", i))`), one of them with `Split: "exclude"`. Freeze with `uuid.NewString()` into `t.TempDir()`.
    - Files: the train, val and eval files exist (n = 239 ≥ 200, so val > 0), and no `.part` file is left.
    - Row counts: `Files[x].Rows` equal `ComputeSplit` counts and sum to 239; the excluded row's text appears in no file.
    - Hashes and sizes: each `SHA256` and `Bytes` equal the file contents.
    - Marker: it decodes into `snapshotMarker` with `V == 1`, `Format == "messages"`, `DataRev == project.DataRev`, and `Files` equal to `snap.Files`.
    - `snap.DataSHA == snapshotDataSHA(snap.Files)`.
    - `snap.Bytes` is the sum of the four file sizes.
    - Determinism: a second freeze with the same run id into a second directory gives identical bytes and the same `DataSHA`.
  - `TestFreezeSnapshotNoVal`: 60 rows (n < 200 means val = 0). `Files` has no `val` key, `r…-val.jsonl` does not exist and the marker JSON has no `val` entry (D11).
  - `TestFreezeSnapshotFreeText`: a free project with 30 text rows. `Format == "text"` and every line decodes as `{"text":…}`.
  - `TestFreezeSnapshotDataChanged`:
    - set `freezeAfterLoad = func() { st.AddRows(ctx, pid, []RowInput{chatInput("late", "late answer arrives here")}, opts) }` and restore `nil` in `t.Cleanup`;
    - expect `errors.Is(err, ErrDataChanged)` and an empty directory (`os.ReadDir` returns nothing).
  - `TestFreezeSnapshotRejects`: an invalid run id returns an error; a project whose rows are all `exclude` returns a `*ValidationError`.

- [ ] 3. Verify:

```
go build ./...
go test ./internal/chibisensei -run '(?i)(TestRun12AndNames|TestFreezeSnapshot)' -count=1 -v
```

Expected: every listed test shows `--- PASS`; `ok aurago/internal/chibisensei`.

---

#### Task B2a.4: Job spec, notebook, golden files, security test

**Files**
- Create:
  - `internal/chibisensei/jobspec.go`, `notebook.go`
  - `jobspec_test.go`, `notebook_test.go`, `golden_test.go`, `security_test.go`
  - `testdata/golden/job-gemma3-270m.json`, `job-qwen35-0p8b.json`, `job-lfm25-1p2b.json`, `notebook.ipynb` (generated)

**Existing symbols modified:** none

**Steps**

- [ ] 1. Create `jobspec.go`:

```go
package chibisensei

import (
	"fmt"
	"regexp"
	"time"

	"aurago/internal/chibisensei/trainer"
)

const (
	JobSchemaVersion = 1
	LogPrefix        = "@@chibi"
	OutputPrefix     = "chibi-"
	jobLRScheduler   = "cosine"
	jobWarmupRatio   = 0.03
	jobWeightDecay   = 0.0
	jobSeed          = 42
	jobMetricEvery   = 10
)

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// JobSpec is job.json, schema v1; the field order is the spec's.
type JobSpec struct {
	V             int        `json:"v"`
	RunID         string     `json:"run_id"`
	Created       string     `json:"created"`
	AuraGoVersion string     `json:"aurago_version"`
	Profile       *Profile   `json:"profile"`
	ProfileSHA    string     `json:"profile_sha"`
	Recipe        string     `json:"recipe"`
	TaskKind      *string    `json:"task_kind"` // null unless specialist
	Labels        []string   `json:"labels"`
	Method        string     `json:"method"`
	Data          JobData    `json:"data"`
	Hyper         JobHyper   `json:"hyper"`
	Budget        JobBudget  `json:"budget"`
	Outputs       JobOutputs `json:"outputs"`
	Deps          JobDeps    `json:"deps"`
	Log           JobLog     `json:"log"`
}

type JobData struct {
	Marker string `json:"marker"` // exact dataset file name of the marker
	Format string `json:"format"` // messages | text
}

type JobHyper struct {
	Epochs                int      `json:"epochs"`
	LearningRate          float64  `json:"learning_rate"`
	LRScheduler           string   `json:"lr_scheduler"`
	WarmupRatio           float64  `json:"warmup_ratio"`
	WeightDecay           float64  `json:"weight_decay"`
	PerDeviceBatch        int      `json:"per_device_batch"`
	GradAccum             int      `json:"grad_accum"`
	MaxSeqLen             int      `json:"max_seq_len"`
	Seed                  int      `json:"seed"`
	GradientCheckpointing bool     `json:"gradient_checkpointing"`
	LossOn                string   `json:"loss_on"` // assistant | all
	LoRA                  *JobLoRA `json:"lora,omitempty"`
}

type JobLoRA struct {
	R             int      `json:"r"`
	Alpha         int      `json:"alpha"`
	Dropout       float64  `json:"dropout"`
	TargetModules []string `json:"target_modules"`
}

type JobBudget struct {
	SessionTimeoutS int `json:"session_timeout_s"`
	MaxTrainS       int `json:"max_train_s"`
	EvalMaxSamples  int `json:"eval_max_samples"`
	MaxNewTokens    int `json:"max_new_tokens"`
}

type JobOutputs struct {
	EmitBaseGGUF bool   `json:"emit_base_gguf"`
	GGUFOutType  string `json:"gguf_outtype"`
	Prefix       string `json:"prefix"`
}

type JobDeps struct {
	Pip          []string    `json:"pip"`
	Transformers string      `json:"transformers"`
	LlamaCpp     JobLlamaCpp `json:"llama_cpp"`
}

type JobLlamaCpp struct {
	Tag    string `json:"tag"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type JobLog struct {
	Prefix           string `json:"prefix"`
	MetricEverySteps int    `json:"metric_every_steps"`
}

type JobInput struct {
	RunID         string
	Created       time.Time
	AuraGoVersion string
	Project       Project
	Profile       *Profile
	ProfileSHA    string
	Plan          Plan
	Marker        string // file name of the dataset marker
	Format        string // messages | text
	Lock          trainer.Lock
}

func jobErr(format string, a ...any) error { return fmt.Errorf("job spec: "+format, a...) }

// BuildJobSpec builds job.json from a computed plan. Hyper-parameters come from the plan; scheduler, warmup,
// weight decay and seed are the fixed values of the spec; deps are the lock plus the profile's extra pins.
func BuildJobSpec(in JobInput) (JobSpec, error) {
	p, pl := in.Profile, in.Plan
	switch {
	case p == nil:
		return JobSpec{}, jobErr("profile required")
	case !ValidID(in.RunID):
		return JobSpec{}, jobErr("invalid run id")
	case in.Created.IsZero():
		return JobSpec{}, jobErr("created time required")
	case in.AuraGoVersion == "" || len(in.AuraGoVersion) > 64:
		return JobSpec{}, jobErr("aurago version must have 1 to 64 characters")
	case !sha256Hex.MatchString(in.ProfileSHA):
		return JobSpec{}, jobErr("profile sha must be 64 hex characters")
	case !pl.Trainable:
		return JobSpec{}, jobErr("plan is not trainable")
	case pl.ProfileID != p.ID:
		return JobSpec{}, jobErr("plan is for profile %q, not %q", pl.ProfileID, p.ID)
	case !containsString(p.Training.Methods, pl.Method):
		return JobSpec{}, jobErr("profile does not support method %q", pl.Method)
	case (pl.Method == "lora") != (pl.Hyper.LoRA != nil):
		return JobSpec{}, jobErr("LoRA hyper-parameters must be set exactly for method lora")
	case pl.Hyper.Epochs < 1 || pl.Hyper.PerDeviceBatch < 1 || pl.Hyper.GradAccum < 1 ||
		pl.Hyper.MaxSeqLen < 256 || pl.Hyper.LearningRate <= 0:
		return JobSpec{}, jobErr("plan hyper-parameters out of range")
	case pl.SessionTimeoutMinutes < 1 || pl.MaxTrainSeconds < minMaxTrainSeconds || pl.MaxNewTokens < 1 || pl.EvalSamples < 0:
		return JobSpec{}, jobErr("plan budget out of range")
	case in.Marker != MarkerFileName(in.RunID):
		return JobSpec{}, jobErr("marker must be %s", MarkerFileName(in.RunID))
	}
	recipe := in.Project.Recipe
	format := FormatFor(recipe)
	if format == "" {
		return JobSpec{}, jobErr("unknown recipe %q", recipe)
	}
	if in.Format != format {
		return JobSpec{}, jobErr("format %q does not fit recipe %s", in.Format, recipe)
	}
	var taskKind *string
	if recipe == RecipeSpecialist {
		if !validTaskKind(in.Project.TaskKind) {
			return JobSpec{}, jobErr("a specialist needs a task kind")
		}
		tk := in.Project.TaskKind
		taskKind = &tk
	} else if in.Project.TaskKind != "" {
		return JobSpec{}, jobErr("only a specialist has a task kind")
	}
	deps := in.Lock
	deps.Pip = append(append([]string{}, in.Lock.Pip...), p.Training.ExtraPip...)
	if err := deps.Validate(); err != nil {
		return JobSpec{}, jobErr("dependencies: %w", err)
	}
	labels := append([]string{}, in.Project.Labels...)
	lossOn := "assistant"
	if recipe == RecipeFree {
		lossOn = "all"
	}
	prof := *p
	spec := JobSpec{
		V: JobSchemaVersion, RunID: in.RunID, Created: in.Created.UTC().Format(time.RFC3339),
		AuraGoVersion: in.AuraGoVersion, Profile: &prof, ProfileSHA: in.ProfileSHA,
		Recipe: recipe, TaskKind: taskKind, Labels: labels, Method: pl.Method,
		Data: JobData{Marker: in.Marker, Format: format},
		Hyper: JobHyper{Epochs: pl.Hyper.Epochs, LearningRate: pl.Hyper.LearningRate, LRScheduler: jobLRScheduler,
			WarmupRatio: jobWarmupRatio, WeightDecay: jobWeightDecay, PerDeviceBatch: pl.Hyper.PerDeviceBatch,
			GradAccum: pl.Hyper.GradAccum, MaxSeqLen: pl.Hyper.MaxSeqLen, Seed: jobSeed,
			GradientCheckpointing: p.Training.GradCheckpointing, LossOn: lossOn},
		Budget: JobBudget{SessionTimeoutS: pl.SessionTimeoutMinutes * 60, MaxTrainS: pl.MaxTrainSeconds,
			EvalMaxSamples: pl.EvalSamples, MaxNewTokens: pl.MaxNewTokens},
		Outputs: JobOutputs{EmitBaseGGUF: pl.NeedBaseGGUF, GGUFOutType: p.GGUF.OutType, Prefix: OutputPrefix},
		Deps: JobDeps{Pip: deps.Pip, Transformers: deps.Transformers,
			LlamaCpp: JobLlamaCpp{Tag: deps.LlamaCppTag, URL: deps.LlamaCppURL, SHA256: deps.LlamaCppSHA256}},
		Log: JobLog{Prefix: LogPrefix, MetricEverySteps: jobMetricEvery},
	}
	if l := pl.Hyper.LoRA; l != nil {
		spec.Hyper.LoRA = &JobLoRA{R: l.R, Alpha: l.Alpha, Dropout: l.Dropout,
			TargetModules: append([]string{}, p.Training.LoRA.TargetModules...)}
	}
	return spec, nil
}

// JSON is the deterministic job.json: struct order, no HTML escaping, one-space indent, one trailing newline.
func (j JobSpec) JSON() ([]byte, error) {
	b, err := marshalStable(j)
	if err != nil {
		return nil, jobErr("encode: %w", err)
	}
	return b, nil
}
```

  `containsString` is in `checks.go:393`, `validTaskKind` in `projects.go:39` and `minMaxTrainSeconds` in `plan.go`.

- [ ] 2. Create `notebook.go`:

```go
package chibisensei

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// maxNotebookBytes keeps the save_kernel request below Kaggle's 20 MiB cap.
const maxNotebookBytes = 15 << 20

type nbMarkdownCell struct {
	CellType string         `json:"cell_type"`
	Metadata map[string]any `json:"metadata"`
	Source   []string       `json:"source"`
}

type nbCodeCell struct {
	CellType       string         `json:"cell_type"`
	ExecutionCount *int           `json:"execution_count"`
	Metadata       map[string]any `json:"metadata"`
	Outputs        []any          `json:"outputs"`
	Source         []string       `json:"source"`
}

type nbKernelSpec struct {
	DisplayName string `json:"display_name"`
	Language    string `json:"language"`
	Name        string `json:"name"`
}

type nbLanguageInfo struct {
	Name string `json:"name"`
}

type nbMetadata struct {
	KernelSpec   nbKernelSpec   `json:"kernelspec"`
	LanguageInfo nbLanguageInfo `json:"language_info"`
}

type nbDocument struct {
	Cells         []any      `json:"cells"`
	Metadata      nbMetadata `json:"metadata"`
	NBFormat      int        `json:"nbformat"`
	NBFormatMinor int        `json:"nbformat_minor"`
}

// BuildNotebook returns the nbformat 4 notebook of a run: a markdown note and one Python cell that decodes
// job.json and the trainer into /tmp/chibi, runs the trainer and always ends with an exit line. Deterministic.
func BuildNotebook(runID string, job, script []byte) ([]byte, error) {
	if !ValidID(runID) {
		return nil, fmt.Errorf("notebook: invalid run id")
	}
	if !json.Valid(job) {
		return nil, fmt.Errorf("notebook: job is not valid JSON")
	}
	if len(script) == 0 {
		return nil, fmt.Errorf("notebook: empty script")
	}
	run12 := Run12(runID)
	code := wrapperSource(run12, base64.StdEncoding.EncodeToString(job), base64.StdEncoding.EncodeToString(script))
	doc := nbDocument{
		Cells: []any{
			nbMarkdownCell{CellType: "markdown", Metadata: map[string]any{},
				Source: sourceLines("Generated by AuraGo Chibi Sensei for run " + run12 + ". Do not edit.")},
			nbCodeCell{CellType: "code", Metadata: map[string]any{}, Outputs: []any{}, Source: sourceLines(code)},
		},
		Metadata: nbMetadata{KernelSpec: nbKernelSpec{DisplayName: "Python 3", Language: "python", Name: "python3"},
			LanguageInfo: nbLanguageInfo{Name: "python"}},
		NBFormat: 4, NBFormatMinor: 4,
	}
	b, err := marshalStable(doc)
	if err != nil {
		return nil, fmt.Errorf("notebook: encode: %w", err)
	}
	if len(b) > maxNotebookBytes {
		return nil, fmt.Errorf("notebook: %d bytes exceed the %d byte limit", len(b), maxNotebookBytes)
	}
	return b, nil
}

// sourceLines splits text into Jupyter source lines, each keeping its "\n" except the last.
func sourceLines(s string) []string {
	parts := strings.SplitAfter(s, "\n")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	return parts
}

// wrapperSource is the Python cell. It never raises: the session ends COMPLETE and its outputs stay
// downloadable; chibi-status.json, not the Kaggle state, decides success.
func wrapperSource(run12, jobB64, scriptB64 string) string {
	return strings.Join([]string{
		"# Generated by AuraGo Chibi Sensei for run " + run12 + ". Do not edit.",
		"import base64, hashlib, json, os, subprocess, sys",
		"",
		`JOB_B64 = "` + jobB64 + `"`,
		`SCRIPT_B64 = "` + scriptB64 + `"`,
		`WORK = "/tmp/chibi"`,
		`OUT = "/kaggle/working"`,
		"",
		"rc = 1",
		"try:",
		"    os.makedirs(WORK, exist_ok=True)",
		"    job_bytes = base64.b64decode(JOB_B64)",
		"    script_bytes = base64.b64decode(SCRIPT_B64)",
		`    job_path = os.path.join(WORK, "job.json")`,
		`    script_path = os.path.join(WORK, "chibi_train.py")`,
		`    with open(job_path, "wb") as fh:`,
		"        fh.write(job_bytes)",
		`    with open(script_path, "wb") as fh:`,
		"        fh.write(script_bytes)",
		`    print(job_bytes.decode("utf-8"), flush=True)`,
		`    print("chibi script sha256 " + hashlib.sha256(script_bytes).hexdigest(), flush=True)`,
		`    rc = subprocess.call([sys.executable, "-u", script_path, "--job", job_path, "--out", OUT])`,
		"except BaseException as exc:",
		`    print("chibi wrapper error: " + type(exc).__name__, flush=True)`,
		"    rc = 1",
		`print("@@chibi " + json.dumps({"v": 1, "t": "exit", "code": rc}, separators=(",", ":")), flush=True)`,
	}, "\n")
}
```

- [ ] 3. Create `golden_test.go`:

```go
package chibisensei

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/chibisensei/trainer"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files and the proof data under testdata")

const goldenRunID = "0123abcd-4567-4890-abcd-ef0123456789"

// goldenLock is fixed so pin changes at the proof runs do not churn the golden files.
var goldenLock = trainer.Lock{Pip: []string{"peft==0.21.3"}, LlamaCppTag: "b9994",
	LlamaCppURL: trainer.LlamaCppURLPrefix + "b9994"}

type goldenCase struct {
	profile, recipe, taskKind string
	labels                    []string
}

var goldenCases = []goldenCase{
	{"gemma3-270m", RecipeSpecialist, "classify", []string{"account", "billing", "delivery", "technical"}},
	{"qwen35-0p8b", RecipeAssistant, "", nil},
	{"lfm25-1p2b", RecipeFree, "", nil},
}

func goldenJob(t *testing.T, c *Catalog, gc goldenCase, lock trainer.Lock) JobSpec {
	t.Helper()
	prof, ok := c.Get(gc.profile)
	if !ok {
		t.Fatalf("profile %s missing", gc.profile)
	}
	rows := make([]SplitRow, 412)
	assign := map[int64]string{}
	for i := range rows {
		rows[i] = SplitRow{ID: int64(i + 1), Split: "auto", EstTokens: 180}
		switch {
		case i < 371:
			assign[rows[i].ID] = "train"
		case i < 392:
			assign[rows[i].ID] = "val"
		default:
			assign[rows[i].ID] = "eval"
		}
	}
	labels := gc.labels
	if labels == nil {
		labels = []string{}
	}
	proj := Project{ProjectSummary: ProjectSummary{ID: "11111111-2222-4333-8444-555555555555", Recipe: gc.recipe,
		TaskKind: gc.taskKind, ProfileID: gc.profile, DataRev: 7}, Labels: labels, Preset: "balanced"}
	plan, err := ComputePlan(PlanInput{Project: proj, Profile: prof, Rows: rows,
		Split:                SplitResult{Counts: SplitCounts{Train: 371, Val: 21, Eval: 20}, Assign: assign},
		RemoteTimeoutMinutes: 60, NeedBaseGGUF: true})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := BuildJobSpec(JobInput{RunID: goldenRunID, Created: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC),
		AuraGoVersion: "test", Project: proj, Profile: prof, ProfileSHA: c.SHA(gc.profile), Plan: plan,
		Marker: MarkerFileName(goldenRunID), Format: FormatFor(gc.recipe), Lock: lock})
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (create it with -update)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs; review the change, then run go test ./internal/chibisensei -run '(?i)golden' -update", path)
	}
}

func TestJobSpecGolden(t *testing.T) {
	c := mustCatalog(t)
	for _, gc := range goldenCases {
		a, err := goldenJob(t, c, gc, goldenLock).JSON()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := goldenJob(t, c, gc, goldenLock).JSON()
		if !bytes.Equal(a, b) {
			t.Fatalf("%s: job.json is not deterministic", gc.profile)
		}
		checkGolden(t, "job-"+gc.profile+".json", a)
	}
}

func TestNotebookGolden(t *testing.T) {
	job, err := goldenJob(t, mustCatalog(t), goldenCases[1], goldenLock).JSON()
	if err != nil {
		t.Fatal(err)
	}
	stub := []byte("print(\"chibi stub\")\n")
	nb, err := BuildNotebook(goldenRunID, job, stub)
	if err != nil {
		t.Fatal(err)
	}
	nb2, _ := BuildNotebook(goldenRunID, job, stub)
	if !bytes.Equal(nb, nb2) {
		t.Fatal("notebook is not deterministic")
	}
	checkGolden(t, "notebook.ipynb", nb)
}
```

- [ ] 4. Create `security_test.go`:

```go
package chibisensei

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"

	"aurago/internal/chibisensei/trainer"
)

// jobTopLevelKeys is the allow-list of job.json keys (schema v1).
var jobTopLevelKeys = []string{"v", "run_id", "created", "aurago_version", "profile", "profile_sha", "recipe",
	"task_kind", "labels", "method", "data", "hyper", "budget", "outputs", "deps", "log"}

// allowedURLHosts are the only hosts the trainer and the notebook may name.
var allowedURLHosts = map[string]bool{"pypi.org": true, "files.pythonhosted.org": true, "huggingface.co": true,
	"codeload.github.com": true, "github.com": true, "download.pytorch.org": true}

// jobURLExempt lists job.json paths whose URLs are metadata the trainer never fetches.
var jobURLExempt = map[string]bool{"profile.license.url": true}

var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"authorization header", regexp.MustCompile(`(?i)authorization`)},
	{"bearer credential", regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/-]{8,}`)},
	{"kaggle credential name", regexp.MustCompile(`(?i)\bkaggle_(key|username|api_token|token)\b`)},
	{"kaggle variable assignment", regexp.MustCompile(`\bKAGGLE_[A-Z0-9_]+["']?\s*[:=]\s*\S`)},
	{"secret assignment", regexp.MustCompile(`(?i)\b[a-z0-9_]*(token|secret|password|passwd|api_?key)[a-z0-9_]*["']?\s*[:=]\s*["'][A-Za-z0-9_-]{16,}["']`)},
	{"hugging face token", regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}`)},
	{"github token", regexp.MustCompile(`\b(ghp|gho|ghs|ghu)_[A-Za-z0-9]{30,}|\bgithub_pat_[A-Za-z0-9_]{20,}`)},
	{"api key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)},
	{"aws key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"slack token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
	{"windows path", regexp.MustCompile(`(?i)\b[a-z]:(\\|/(users|windows|program))`)},
	{"home path", regexp.MustCompile(`/home/[A-Za-z0-9_.-]+|/Users/[A-Za-z0-9_.-]+|/root/|~/`)},
}

var urlPattern = regexp.MustCompile("(?i)\\bhttps?://[^\\s\"'<>()\\[\\]{}\\\\`]+")

var b64Assign = regexp.MustCompile(`(?m)^(JOB_B64|SCRIPT_B64) = "([A-Za-z0-9+/=]*)"$`)

// secretFindings names the forbidden patterns in text (never the matched value).
func secretFindings(text string) []string {
	var out []string
	for _, p := range secretPatterns {
		if p.re.MatchString(text) {
			out = append(out, p.name)
		}
	}
	return out
}

// urlFindings reports URLs that are not https or not on an allowed host.
func urlFindings(text string) []string {
	var out []string
	for _, raw := range urlPattern.FindAllString(text, -1) {
		u, err := url.Parse(strings.TrimRight(raw, ".,;:!?"))
		switch {
		case err != nil:
			out = append(out, "unparsable URL")
		case !strings.EqualFold(u.Scheme, "https"):
			out = append(out, "plain http URL")
		case !allowedURLHosts[strings.ToLower(u.Hostname())]:
			out = append(out, "URL host "+u.Hostname())
		}
	}
	return out
}

func walkJSONStrings(v any, path string, fn func(path, s string)) {
	switch x := v.(type) {
	case string:
		fn(path, x)
	case map[string]any:
		for k, e := range x {
			p := k
			if path != "" {
				p = path + "." + k
			}
			walkJSONStrings(e, p, fn)
		}
	case []any:
		for _, e := range x {
			walkJSONStrings(e, path, fn)
		}
	}
}

// jobFindings checks job.json: exact top-level keys, no secret shape anywhere, allowed URLs only.
func jobFindings(job []byte) []string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(job, &top); err != nil {
		return []string{"job.json is not a JSON object"}
	}
	var out []string
	allowed := map[string]bool{}
	for _, k := range jobTopLevelKeys {
		allowed[k] = true
		if _, ok := top[k]; !ok {
			out = append(out, "missing key "+k)
		}
	}
	for k := range top {
		if !allowed[k] {
			out = append(out, "unexpected key "+k)
		}
	}
	out = append(out, secretFindings(string(job))...)
	var v any
	_ = json.Unmarshal(job, &v)
	walkJSONStrings(v, "", func(path, s string) {
		if !jobURLExempt[path] {
			out = append(out, urlFindings(s)...)
		}
	})
	sort.Strings(out)
	return out
}

// notebookFindings checks the notebook document; the decoded job and script are checked by the caller.
func notebookFindings(nb, job, script []byte) []string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(nb, &top); err != nil {
		return []string{"notebook is not a JSON object"}
	}
	var out []string
	for k := range top {
		if k != "cells" && k != "metadata" && k != "nbformat" && k != "nbformat_minor" {
			out = append(out, "unexpected notebook key "+k)
		}
	}
	var doc struct {
		Cells []struct {
			CellType string   `json:"cell_type"`
			Source   []string `json:"source"`
		} `json:"cells"`
		NBFormat int `json:"nbformat"`
	}
	if err := json.Unmarshal(nb, &doc); err != nil || doc.NBFormat != 4 || len(doc.Cells) != 2 ||
		doc.Cells[0].CellType != "markdown" || doc.Cells[1].CellType != "code" {
		return append(out, "notebook is not the two-cell nbformat 4 document")
	}
	code := strings.Join(doc.Cells[1].Source, "")
	matches := b64Assign.FindAllStringSubmatch(code, -1)
	if len(matches) != 2 {
		out = append(out, "code cell must hold exactly JOB_B64 and SCRIPT_B64")
	}
	decoded := map[string][]byte{}
	for _, m := range matches {
		b, err := base64.StdEncoding.DecodeString(m[2])
		if err != nil {
			out = append(out, m[1]+" is not base64")
			continue
		}
		decoded[m[1]] = b
	}
	if !bytes.Equal(decoded["JOB_B64"], job) {
		out = append(out, "JOB_B64 does not decode to job.json")
	}
	if !bytes.Equal(decoded["SCRIPT_B64"], script) {
		out = append(out, "SCRIPT_B64 does not decode to the trainer")
	}
	for _, text := range []string{b64Assign.ReplaceAllString(code, `$1 = ""`), strings.Join(doc.Cells[0].Source, ""),
		string(top["metadata"])} {
		out = append(out, secretFindings(text)...)
		out = append(out, urlFindings(text)...)
	}
	sort.Strings(out)
	return out
}

func TestTrainingPayloadHasNoSecrets(t *testing.T) {
	c := mustCatalog(t)
	lock, err := trainer.LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	script := trainer.Script()
	if f := append(secretFindings(string(script)), urlFindings(string(script))...); len(f) > 0 {
		t.Errorf("trainer script: %v", f)
	}
	for _, gc := range goldenCases {
		job, err := goldenJob(t, c, gc, lock).JSON()
		if err != nil {
			t.Fatal(err)
		}
		if f := jobFindings(job); len(f) > 0 {
			t.Errorf("%s job.json: %v", gc.profile, f)
		}
		nb, err := BuildNotebook(goldenRunID, job, script)
		if err != nil {
			t.Fatal(err)
		}
		if f := notebookFindings(nb, job, script); len(f) > 0 {
			t.Errorf("%s notebook: %v", gc.profile, f)
		}
	}
}

func TestPayloadScannerCatchesPlantedValues(t *testing.T) {
	bad := []string{
		"headers = {'Authorization': x}", "Bearer abcdefghijklmnop", `os.environ["KAGGLE_KEY"]`,
		`KAGGLE_USERNAME = "someone"`, `hf_token = "abcdefghijklmnopqrstuvwx"`, "hf_" + strings.Repeat("a", 34),
		"sk-" + strings.Repeat("b", 24), "AKIA" + strings.Repeat("C", 16), "-----BEGIN OPENSSH PRIVATE KEY-----",
		`C:\Users\someone\aurago`, "c:/Users/someone", "/home/someone/data", "/Users/someone/data",
		"~/.kaggle/kaggle.json", "https://evil.example.com/x.tar.gz", "http://pypi.org/simple",
	}
	for _, s := range bad {
		if len(secretFindings(s))+len(urlFindings(s)) == 0 {
			t.Errorf("not caught: %q", s)
		}
	}
	good := []string{
		"https://huggingface.co/Qwen/Qwen3.5-0.8B", trainer.LlamaCppURLPrefix + "b9994", "https://pypi.org/simple/peft/",
		"/kaggle/working/chibi-status.json", `"max_new_tokens": 256`, "tokenizer = AutoTokenizer.from_pretrained(path)",
		`os.environ.pop("HF_TOKEN", None)`,
	}
	for _, s := range good {
		if f := append(secretFindings(s), urlFindings(s)...); len(f) > 0 {
			t.Errorf("false positive %q: %v", s, f)
		}
	}
}
```

- [ ] 5. Create `jobspec_test.go`.

   `TestBuildJobSpecFields` uses `goldenJob(t, c, gc, goldenLock)`:

   | Case | Asserts |
   |---|---|
   | qwen (assistant) | `TaskKind == nil` and the JSON contains `"task_kind": null`; `Labels` is `[]` (JSON `"labels": []`); `Method == "lora"`; `Hyper.LoRA.TargetModules` equal the profile's 10 names; `LossOn == "assistant"`; `LRScheduler == "cosine"`, `WarmupRatio == 0.03`, `WeightDecay == 0`, `Seed == 42`; `GradientCheckpointing == true`; `Data == {"r0123abcd4567-marker.json","messages"}`; `Budget.SessionTimeoutS == plan.SessionTimeoutMinutes*60`, `MaxTrainS == plan.MaxTrainSeconds`, `EvalMaxSamples == 20`, `MaxNewTokens == 256`; `Outputs == {true,"q8_0","chibi-"}`; `Deps.Pip == ["peft==0.21.3"]`; `Deps.LlamaCpp.Tag == "b9994"`; `Log == {"@@chibi",10}` |
   | gemma (specialist, full) | `*TaskKind == "classify"`; `Labels` equal the four labels; decoding the JSON into `map[string]any` finds no `lora` key under `hyper`; `MaxNewTokens == 64` |
   | lfm (free) | `Data.Format == "text"`, `LossOn == "all"` |

   For every case, `JSON()` contains `<|im_end|>` or `<end_of_turn>` literally (no `\u003c`), ends with exactly one `\n` and parses with `json.Valid`.

   `TestBuildJobSpecRejects` starts from a valid qwen `JobInput` (built like `goldenJob`). Each row applies one `mutate func(*JobInput)` and expects an error containing the substring:

   | Mutation | Expected error substring |
   |---|---|
   | `RunID = "x"` | `invalid run id` |
   | `Profile = nil` | `profile required` |
   | `Plan.Trainable = false` | `not trainable` |
   | `Plan.ProfileID = "gemma3-270m"` | `plan is for profile` |
   | `Plan.Method = "full"` | `does not support method` |
   | `Plan.Hyper.LoRA = nil` | `exactly for method lora` |
   | `Marker = "x.json"` | `marker must be` |
   | `Format = "text"` | `does not fit recipe` |
   | `Project.TaskKind = "classify"` | `only a specialist` |
   | `ProfileSHA = "abc"` | `profile sha` |
   | `Lock.LlamaCppURL = "https://example.com/x"` | `dependencies` |
   | `Lock.Pip = append(Lock.Pip, "peft==0.21.3")` | `twice` |
   | `AuraGoVersion = ""` | `aurago version` |
   | `Plan.MaxTrainSeconds = 10` | `budget` |

- [ ] 6. Create `notebook_test.go`. `TestBuildNotebookShape` uses the qwen golden job and the stub script `print("chibi stub")\n`:
  - top-level keys are exactly `cells, metadata, nbformat, nbformat_minor`, with `nbformat == 4` and `nbformat_minor == 4`;
  - cell 0 is markdown and its joined source equals `Generated by AuraGo Chibi Sensei for run 0123abcd4567. Do not edit.`;
  - cell 1 is code with `"execution_count": null` and `"outputs": []`; its joined source contains:
    - `subprocess.call([sys.executable, "-u", script_path, "--job", job_path, "--out", OUT])`,
    - `except BaseException as exc:`,
    - `print("@@chibi " + json.dumps({"v": 1, "t": "exit", "code": rc}`;
  - the two base64 literals (`b64Assign`) decode to the job and the stub;
  - the output ends with `}\n`.

  `TestBuildNotebookRejects`: an invalid run id, job bytes `{"v":` and an empty script each return an error.

- [ ] 7. Generate the golden files, review them, then run without `-update`:

```
go test ./internal/chibisensei -run '(?i)(TestJobSpecGolden|TestNotebookGolden)' -count=1 -update
go test ./internal/chibisensei -run '(?i)(TestJobSpecGolden|TestNotebookGolden|TestBuildJobSpec|TestBuildNotebook|TestTrainingPayloadHasNoSecrets|TestPayloadScannerCatchesPlantedValues)' -count=1 -v
```

   Review before accepting:
   - `job-qwen35-0p8b.json`: `"task_kind": null`; a `lora` block with 10 `target_modules`; `"loss_on": "assistant"`.
   - `job-gemma3-270m.json`: `"method": "full"`; no `lora` key in `hyper`; `"task_kind": "classify"`.
   - `job-lfm25-1p2b.json`: `"format": "text"` and `"loss_on": "all"`.
   - `notebook.ipynb`: two cells.

   Expected: every listed test shows `--- PASS`. `-update` is defined only in package `chibisensei`, so never pass it to `./internal/chibisensei/...`.

- [ ] 8. Verify:

```
go build ./...
go test ./internal/chibisensei/... -count=1
```

Expected: `ok` for `aurago/internal/chibisensei` and `aurago/internal/chibisensei/trainer`.

---

#### Task B2a.5: Log and output parsers with fixtures

**Files**
- Create:
  - `internal/chibisensei/logparse.go`, `logparse_test.go`, `outputs.go`, `outputs_test.go`
  - `testdata/logs/kernel-log.json`, `testdata/logs/kernel-log.txt`
  - `testdata/outputs/chibi-status.json`, `chibi-manifest.json`, `chibi-metrics.jsonl`, `chibi-eval_samples.jsonl`

**Existing symbols modified:** none. It reuses `cutRunes` (`lang.go:117`), `ValidID`, `profileIDPattern`, `sha256Hex` and `LogPrefix`.

**Steps**

- [ ] 1. Create `logparse.go`:

```go
package chibisensei

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

const (
	logLinePrefix   = LogPrefix + " "
	maxLogLineBytes = 4 << 10  // a whole "@@chibi {...}" line
	maxLogBodyBytes = 16 << 20 // only the tail beyond this is read
	maxLogEvents    = 20000
	maxLogMessage   = 2000 // runes kept of a message
	maxMetricStep   = 10_000_000
	// ExitSeq is the seq the parser gives the notebook wrapper's exit line, which carries none.
	ExitSeq int64 = 1<<53 - 1
)

// TrainerStages are the stage names of the training contract in run order.
var TrainerStages = []string{"boot", "env_check", "install", "data", "load_base", "base_eval", "train",
	"tuned_eval", "save", "gguf", "manifest"}

var (
	logEventTypes = map[string]bool{"stage": true, "metric": true, "eval": true, "warn": true, "error": true,
		"done": true, "exit": true, "info": true}
	codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

func isTrainerStage(s string) bool { return containsString(TrainerStages, s) }

// LogEvent is one structured trainer line.
type LogEvent struct {
	Seq int64           `json:"seq"`
	TS  string          `json:"ts"`
	T   string          `json:"t"`   // stage | metric | eval | warn | error | done | exit | info
	Raw json.RawMessage `json:"raw"` // the whole object, compacted
}

type kaggleLogEntry struct {
	StreamName string `json:"stream_name"`
	Data       string `json:"data"`
}

// ParseKernelLog accepts a JSON array of {stream_name, time, data} entries (also cut at the front by a tail
// read) or plain text, extracts the "@@chibi " lines, drops malformed and over-long ones, dedupes by seq
// (first wins) and returns them in seq order.
func ParseKernelLog(body []byte) []LogEvent {
	if len(body) > maxLogBodyBytes {
		body = body[len(body)-maxLogBodyBytes:]
	}
	texts := kaggleLogStreams(body)
	if texts == nil {
		texts = []string{string(body)}
	}
	seen := map[int64]bool{}
	out := []LogEvent{}
	for _, text := range texts {
		for _, line := range strings.Split(text, "\n") {
			ev, ok := parseLogLine(line)
			if !ok || seen[ev.Seq] || len(out) >= maxLogEvents {
				continue
			}
			seen[ev.Seq] = true
			out = append(out, ev)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// kaggleLogStreams joins the data of each stream in first-seen order; nil when the body is not a Kaggle JSON log.
func kaggleLogStreams(body []byte) []string {
	var entries []kaggleLogEntry
	if t := bytes.TrimSpace(body); len(t) > 0 && t[0] == '[' {
		if err := json.Unmarshal(t, &entries); err != nil {
			entries = nil
		}
	}
	if entries == nil {
		entries = scanLogEntries(body)
	}
	if len(entries) == 0 {
		return nil
	}
	var order []string
	joined := map[string]*strings.Builder{}
	for _, e := range entries {
		b := joined[e.StreamName]
		if b == nil {
			b = &strings.Builder{}
			joined[e.StreamName] = b
			order = append(order, e.StreamName)
		}
		b.WriteString(e.Data)
	}
	out := make([]string, 0, len(order))
	for _, name := range order {
		out = append(out, joined[name].String())
	}
	return out
}

var streamKey = []byte(`"stream_name"`)

// scanLogEntries decodes the entries of a JSON log whose head is missing, one object at a time.
func scanLogEntries(body []byte) []kaggleLogEntry {
	var out []kaggleLogEntry
	for off := 0; off < len(body); {
		i := bytes.Index(body[off:], streamKey)
		if i < 0 {
			break
		}
		at := off + i
		if open := bytes.LastIndexByte(body[off:at], '{'); open >= 0 {
			var e kaggleLogEntry
			if err := json.NewDecoder(bytes.NewReader(body[off+open:])).Decode(&e); err == nil {
				out = append(out, e)
			}
		}
		off = at + len(streamKey)
	}
	return out
}

func parseLogLine(line string) (LogEvent, bool) {
	line = strings.TrimRight(line, "\r")
	i := strings.Index(line, logLinePrefix)
	if i < 0 {
		return LogEvent{}, false
	}
	payload := strings.TrimSpace(line[i+len(logLinePrefix):])
	if len(logLinePrefix)+len(payload) > maxLogLineBytes || !strings.HasPrefix(payload, "{") {
		return LogEvent{}, false
	}
	var head struct {
		V   int    `json:"v"`
		Seq *int64 `json:"seq"`
		TS  string `json:"ts"`
		T   string `json:"t"`
	}
	if err := json.Unmarshal([]byte(payload), &head); err != nil || head.V != 1 || !logEventTypes[head.T] {
		return LogEvent{}, false
	}
	seq := ExitSeq
	switch {
	case head.Seq != nil && *head.Seq >= 0 && *head.Seq < ExitSeq:
		seq = *head.Seq
	case head.Seq == nil && head.T == "exit":
	default:
		return LogEvent{}, false
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(payload)); err != nil {
		return LogEvent{}, false
	}
	return LogEvent{Seq: seq, TS: cutRunes(head.TS, 40), T: head.T, Raw: compact.Bytes()}, true
}

// Metric is one training or validation point.
type Metric struct {
	Step     int     `json:"step"`
	MaxSteps int     `json:"max_steps,omitempty"`
	Kind     string  `json:"kind"` // train | eval
	Epoch    float64 `json:"epoch"`
	Loss     float64 `json:"loss"`
	LR       float64 `json:"lr"`
	GradNorm float64 `json:"grad_norm"`
	TokS     float64 `json:"tok_s"`
	ElapsedS float64 `json:"elapsed_s,omitempty"`
	EtaS     float64 `json:"eta_s,omitempty"`
	VRAMGB   float64 `json:"vram_gb,omitempty"`
	TS       string  `json:"ts"`
}

type metricWire struct {
	T        string   `json:"t"`
	TS       string   `json:"ts"`
	Step     *int     `json:"step"`
	MaxSteps int      `json:"max_steps"`
	Kind     string   `json:"kind"`
	Epoch    float64  `json:"epoch"`
	Loss     *float64 `json:"loss"`
	EvalLoss *float64 `json:"eval_loss"`
	LR       float64  `json:"lr"`
	GradNorm float64  `json:"grad_norm"`
	TokS     float64  `json:"tok_s"`
	ElapsedS float64  `json:"elapsed_s"`
	EtaS     float64  `json:"eta_s"`
	VRAMGB   float64  `json:"vram_gb"`
}

// parseMetric decodes a metric object (a log payload or a chibi-metrics.jsonl line).
func parseMetric(raw []byte) (Metric, bool) {
	var w metricWire
	if json.Unmarshal(raw, &w) != nil || (w.T != "" && w.T != "metric") || w.Step == nil ||
		*w.Step < 0 || *w.Step > maxMetricStep || w.MaxSteps < 0 || w.MaxSteps > maxMetricStep {
		return Metric{}, false
	}
	kind := w.Kind
	switch kind {
	case "train", "eval":
	case "":
		kind = "train"
		if w.Loss == nil && w.EvalLoss != nil {
			kind = "eval"
		}
	default:
		return Metric{}, false
	}
	loss := w.Loss
	if loss == nil && kind == "eval" {
		loss = w.EvalLoss
	}
	if loss == nil {
		return Metric{}, false
	}
	return Metric{Step: *w.Step, MaxSteps: w.MaxSteps, Kind: kind, Epoch: w.Epoch, Loss: *loss, LR: w.LR,
		GradNorm: w.GradNorm, TokS: w.TokS, ElapsedS: w.ElapsedS, EtaS: w.EtaS, VRAMGB: w.VRAMGB,
		TS: cutRunes(w.TS, 40)}, true
}

// Metric decodes a metric event.
func (e LogEvent) Metric() (Metric, bool) {
	if e.T != "metric" {
		return Metric{}, false
	}
	m, ok := parseMetric(e.Raw)
	if ok && m.TS == "" {
		m.TS = e.TS
	}
	return m, ok
}

type StageEvent struct{ Stage, State string } // state: start | done | failed

// Stage decodes a stage event; the stage must be one of TrainerStages.
func (e LogEvent) Stage() (StageEvent, bool) {
	if e.T != "stage" {
		return StageEvent{}, false
	}
	var w struct {
		Stage string `json:"stage"`
		State string `json:"state"`
	}
	if json.Unmarshal(e.Raw, &w) != nil || !isTrainerStage(w.Stage) ||
		(w.State != "start" && w.State != "done" && w.State != "failed") {
		return StageEvent{}, false
	}
	return StageEvent{Stage: w.Stage, State: w.State}, true
}

type ErrorEvent struct{ Stage, Code, Message string }

// Error decodes an error event. The message is untrusted: callers scrub it before storing.
func (e LogEvent) Error() (ErrorEvent, bool) {
	if e.T != "error" {
		return ErrorEvent{}, false
	}
	var w struct {
		Stage   string `json:"stage"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(e.Raw, &w) != nil || !codePattern.MatchString(w.Code) || (w.Stage != "" && !isTrainerStage(w.Stage)) {
		return ErrorEvent{}, false
	}
	return ErrorEvent{Stage: w.Stage, Code: w.Code, Message: cutRunes(w.Message, maxLogMessage)}, true
}

// ExitCode decodes the wrapper's exit event.
func (e LogEvent) ExitCode() (int, bool) {
	if e.T != "exit" {
		return 0, false
	}
	var w struct {
		Code *int `json:"code"`
	}
	if json.Unmarshal(e.Raw, &w) != nil || w.Code == nil {
		return 0, false
	}
	return *w.Code, true
}
```

- [ ] 2. Create `outputs.go`. Complete code for the shared helpers and `ParseManifest`; the other parsers follow the precise rules below the block.

```go
package chibisensei

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Output file names of the trainer (flat in /kaggle/working).
const (
	OutStatus        = "chibi-status.json"
	OutManifest      = "chibi-manifest.json"
	OutMetrics       = "chibi-metrics.jsonl"
	OutEvalSamples   = "chibi-eval_samples.jsonl"
	OutModelGGUF     = "chibi-model-q8_0.gguf"
	OutBaseGGUF      = "chibi-base-q8_0.gguf"
	OutAdapter       = "chibi-adapter-model.safetensors"
	OutAdapterConfig = "chibi-adapter-config.json"
)

// ManifestKindFile maps each manifest kind to its only allowed file name.
var ManifestKindFile = map[string]string{"gguf_tuned": OutModelGGUF, "gguf_base": OutBaseGGUF,
	"adapter": OutAdapter, "adapter_config": OutAdapterConfig, "metrics": OutMetrics,
	"eval_samples": OutEvalSamples, "status": OutStatus}

const (
	maxStatusBytes      = 1 << 20
	maxManifestBytes    = 1 << 20
	maxArtifactBytes    = 8 << 30
	maxMetricsFileBytes = 64 << 20
	maxMetricLineBytes  = 4 << 10
	maxMetricPoints     = 20000
	maxEvalFileBytes    = 256 << 20
	maxEvalLineBytes    = 1 << 20
	maxEvalSamplesCap   = 1000
	maxEvalTextRunes    = 8000
)

// FlexString decodes a JSON string or number (null gives "").
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, []byte("null")):
		*f = ""
	case len(b) > 0 && b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexString(s)
	default:
		var n json.Number
		if err := json.Unmarshal(b, &n); err != nil {
			return errors.New("expected a string or a number")
		}
		*f = FlexString(n.String())
	}
	return nil
}

type ManifestFile struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	V          int            `json:"v"`
	RunID      string         `json:"run_id"`
	ProfileID  string         `json:"profile_id"`
	ProfileSHA string         `json:"profile_sha"`
	Files      []ManifestFile `json:"files"`
}

// ValidManifestPath accepts a flat "chibi-" file name: no separator, no "..", no colon or NUL, trimmed.
func ValidManifestPath(p string) bool {
	return len(p) > len(OutputPrefix) && len(p) <= 128 && strings.HasPrefix(p, OutputPrefix) &&
		!strings.ContainsAny(p, "/\\:\x00") && !strings.Contains(p, "..") && strings.TrimSpace(p) == p
}

// ParseManifest decodes and validates chibi-manifest.json. Errors never quote file content.
func ParseManifest(b []byte) (Manifest, error) {
	if len(b) > maxManifestBytes {
		return Manifest{}, errors.New("manifest: file too large")
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest: %w", err)
	}
	switch {
	case m.V != 1:
		return Manifest{}, errors.New("manifest: unsupported version")
	case !ValidID(m.RunID):
		return Manifest{}, errors.New("manifest: invalid run id")
	case !profileIDPattern.MatchString(m.ProfileID):
		return Manifest{}, errors.New("manifest: invalid profile id")
	case !sha256Hex.MatchString(m.ProfileSHA):
		return Manifest{}, errors.New("manifest: invalid profile sha")
	case len(m.Files) == 0 || len(m.Files) > len(ManifestKindFile):
		return Manifest{}, errors.New("manifest: wrong number of files")
	}
	seenPath, seenKind := map[string]bool{}, map[string]bool{}
	for i, f := range m.Files {
		if !ValidManifestPath(f.Path) {
			return Manifest{}, fmt.Errorf("manifest: file %d: invalid path", i)
		}
		want, ok := ManifestKindFile[f.Kind]
		if !ok {
			return Manifest{}, fmt.Errorf("manifest: file %d: unknown kind", i)
		}
		if f.Path != want {
			return Manifest{}, fmt.Errorf("manifest: file %d: kind %s must be named %s", i, f.Kind, want)
		}
		if seenPath[f.Path] || seenKind[f.Kind] {
			return Manifest{}, fmt.Errorf("manifest: file %d: listed twice", i)
		}
		seenPath[f.Path], seenKind[f.Kind] = true, true
		if f.Bytes < 0 || f.Bytes > maxArtifactBytes {
			return Manifest{}, fmt.Errorf("manifest: file %d: size out of range", i)
		}
		if !sha256Hex.MatchString(f.SHA256) {
			return Manifest{}, fmt.Errorf("manifest: file %d: invalid sha256", i)
		}
	}
	return m, nil
}

// eachLine calls fn for every non-blank line of at most maxLine bytes (CR/LF stripped). Longer lines are
// skipped whole. fn returns false to stop.
func eachLine(r io.Reader, maxLine int, fn func([]byte) bool) error {
	br := bufio.NewReaderSize(r, maxLine+2)
	for {
		line, err := br.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			for errors.Is(err, bufio.ErrBufferFull) {
				_, err = br.ReadSlice('\n')
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			continue
		}
		if t := bytes.TrimRight(line, "\r\n"); len(t) <= maxLine && len(bytes.TrimSpace(t)) > 0 {
			if !fn(t) {
				return nil
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
```

   The rest of `outputs.go`, with exact types and rules:

   Status types:

   ```go
   type StatusGPU struct {
       Name       string     `json:"name"`
       Capability FlexString `json:"capability"`
       VRAMGB     float64    `json:"vram_gb"`
   }
   type StatusVersions struct {
       Python       string `json:"python"`
       Torch        string `json:"torch"`
       Transformers string `json:"transformers"`
       PEFT         string `json:"peft"`
       LlamaCpp     string `json:"llama_cpp"`
   }
   type StatusTrain struct {
       Steps         int      `json:"steps"`
       Epochs        float64  `json:"epochs"`
       TrainLoss     *float64 `json:"train_loss"`
       EvalLossBase  *float64 `json:"eval_loss_base"`
       EvalLossTuned *float64 `json:"eval_loss_tuned"`
       StopReason    string   `json:"stop_reason"`
       TruncatedRows int      `json:"truncated_rows"`
   }
   type Status struct {
       V           int            `json:"v"`
       RunID       string         `json:"run_id"`
       State       string         `json:"state"`
       Stage       string         `json:"stage"`
       Code        string         `json:"code"`
       Message     string         `json:"message"`
       WarningCode string         `json:"warning_code"`
       Started     string         `json:"started"`
       Finished    string         `json:"finished"`
       GPU         StatusGPU      `json:"gpu"`
       Versions    StatusVersions `json:"versions"`
       Train       StatusTrain    `json:"train"`
       Discovery   map[string]any `json:"discovery"`
   }
   ```

   `func ParseStatus(b []byte) (Status, error)`:
   - Each refusal returns its own error, prefixed `status: …`:
     - the file is over `maxStatusBytes`;
     - the JSON does not decode;
     - `V != 1`;
     - `!ValidID(RunID)`;
     - `State` is not `succeeded` or `failed`;
     - `Stage` is not empty and not in `TrainerStages`;
     - `Code` or `WarningCode` is not empty and does not match `codePattern`;
     - `Train.StopReason` is not one of `"", done, time_budget, train_diverged, oom`;
     - `Train.Steps < 0` or `Train.TruncatedRows < 0`.
   - On success it cuts these fields: `Message` to `maxLogMessage` runes, `GPU.Name` to 128, `GPU.Capability` to 16 and every `Versions` field to 64.

   `func (s Status) Succeeded() bool { return s.State == "succeeded" }`

   `func ParseMetricsFile(r io.Reader) ([]Metric, error)`:
   - reads through `eachLine(io.LimitReader(r, maxMetricsFileBytes), maxMetricLineBytes, …)`;
   - every line goes through `parseMetric`, so a line whose `t` is set to something other than `metric` is skipped;
   - dedupes by `(Step, Kind)`, first wins, and stops after `maxMetricPoints`;
   - sorts by `Step`, then `Kind`;
   - reader errors come back as `metrics: %w`.

   `EvalSample`:

   ```go
   type EvalSample struct {
       ID             FlexString `json:"id"` // 0-based line index of the eval file
       PromptMessages []Message  `json:"prompt_messages,omitempty"`
       PromptText     string     `json:"prompt_text,omitempty"`
       Expected       string     `json:"expected"`
       BaseOutput     string     `json:"base_output"`
       TunedOutput    string     `json:"tuned_output"`
       BaseMS         int64      `json:"base_ms"`
       TunedMS        int64      `json:"tuned_ms"`
       Truncated      bool       `json:"truncated"`
   }
   ```

   `func ParseEvalSamples(r io.Reader, limit int) ([]EvalSample, error)`:
   - A `limit` ≤ 0 or above `maxEvalSamplesCap` becomes `maxEvalSamplesCap`.
   - It reads through `eachLine(io.LimitReader(r, maxEvalFileBytes), maxEvalLineBytes, …)`.
   - It skips a line that does not decode, has neither `prompt_messages` nor a non-blank `prompt_text`, has more than `MaxMessages` messages, or has a role outside `system|user|assistant`.
   - It cuts each message content, `PromptText`, `Expected`, `BaseOutput` and `TunedOutput` to `maxEvalTextRunes`, and `ID` to 40.
   - Negative `BaseMS`/`TunedMS` become 0.
   - It stops at `limit` and returns a non-nil slice.

- [ ] 3. Create the log fixtures.

   `testdata/logs/kernel-log.txt` (LF line endings). `{{PAD4200}}` is a literal placeholder that the test replaces with 4,200 `a` characters:

```
   3.1s    1 Collecting peft==0.21.3
   3.4s    2 @@chibi {"v":1,"seq":1,"ts":"2026-11-23T12:00:01Z","t":"stage","stage":"boot","state":"start"}
   3.5s    3 noise on stderr @@chibi without json
   3.6s    4 @@chibi {"v":1,"seq":2,"ts":"2026-11-23T12:00:02Z","t":"stage","stage":"boot","state":"done"}
   3.7s    5 @@chibi {"v":1,"seq":2,"ts":"2026-11-23T12:00:02Z","t":"stage","stage":"boot","state":"done"}
   4.0s    6 @@chibi {"v":1,"seq":3,"ts":"2026-11-23T12:00:03Z","t":"metric","step":10,"max_steps":40,"epoch":0.25,"kind":"train","loss":2.5,"lr":0.0002,"grad_norm":1.5,"tok_s":2400,"elapsed_s":30,"eta_s":90,"vram_gb":6.1}
   4.1s    7 @@chibi {"v":1,"seq":4,"ts":"2026-11-23T12:00:04Z","t":"bogus"}
   4.2s    8 @@chibi {"v":2,"seq":5,"ts":"2026-11-23T12:00:05Z","t":"warn"}
   4.3s    9 @@chibi {"v":1,"seq":6,"ts":"2026-11-23T12:00:06Z","t":"info","pad":"{{PAD4200}}"}
   4.4s   10 @@chibi {"v":1,"seq":8,"ts":"2026-11-23T12:00:08Z","t":"metric","step":20,"max_steps":40,"epoch":0.5,"kind":"eval","loss":2.1}
   4.5s   11 @@chibi {"v":1,"seq":7,"ts":"2026-11-23T12:00:07Z","t":"error","stage":"train","code":"oom","message":"CUDA out of memory"}
   4.6s   12 @@chibi {"v":1,"t":"exit","code":0}
```

   `testdata/logs/kernel-log.json`. Event 2 is split across two stdout entries with a stderr entry between them; seq 3 repeats inside one entry:

```
[{"stream_name":"stderr","time":1.2,"data":"Collecting peft==0.21.3\n"}
,{"stream_name":"stdout","time":3.4,"data":"@@chibi {\"v\":1,\"seq\":1,\"ts\":\"2026-11-23T12:00:01Z\",\"t\":\"stage\",\"stage\":\"boot\",\"state\":\"start\"}\n"}
,{"stream_name":"stdout","time":3.5,"data":"@@chibi {\"v\":1,\"seq\":2,\"ts\":\"2026-11-23T12:00:02Z\",\"t\":\"stage\","}
,{"stream_name":"stderr","time":3.55,"data":"warning: noise\n"}
,{"stream_name":"stdout","time":3.6,"data":"\"stage\":\"boot\",\"state\":\"done\"}\n"}
,{"stream_name":"stdout","time":4.0,"data":"@@chibi {\"v\":1,\"seq\":3,\"ts\":\"2026-11-23T12:00:03Z\",\"t\":\"metric\",\"step\":10,\"max_steps\":40,\"epoch\":0.25,\"kind\":\"train\",\"loss\":2.5}\n@@chibi {\"v\":1,\"seq\":3,\"ts\":\"2026-11-23T12:00:03Z\",\"t\":\"metric\",\"step\":10,\"max_steps\":40,\"epoch\":0.25,\"kind\":\"train\",\"loss\":9.9}\n"}
,{"stream_name":"stdout","time":4.6,"data":"@@chibi {\"v\":1,\"t\":\"exit\",\"code\":0}\n"}
]
```

- [ ] 4. Create the output fixtures in `testdata/outputs/`:

   `chibi-status.json`:

```json
{"v":1,"run_id":"0123abcd-4567-4890-abcd-ef0123456789","state":"succeeded","stage":"manifest","code":"","message":"",
 "warning_code":"base_gguf_skipped","started":"2026-11-23T12:00:00Z","finished":"2026-11-23T12:31:00Z",
 "gpu":{"name":"Tesla T4","capability":"7.5","vram_gb":14.6},
 "versions":{"python":"3.13.5","torch":"2.9.0","transformers":"5.2.0","peft":"0.21.3","llama_cpp":"b9994"},
 "train":{"steps":40,"epochs":1,"train_loss":0.92,"eval_loss_base":2.31,"eval_loss_tuned":1.12,"stop_reason":"done","truncated_rows":2},
 "discovery":{"base_class":"Gemma3ForCausalLM","lora_targets":[],"converter_env":"system"}}
```

   `chibi-manifest.json`. Each sha value is one digit repeated 64 times; write exactly 64 characters.

```json
{"v":1,"run_id":"0123abcd-4567-4890-abcd-ef0123456789","profile_id":"gemma3-270m",
 "profile_sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
 "files":[{"path":"chibi-model-q8_0.gguf","kind":"gguf_tuned","bytes":291234567,"sha256":"1111111111111111111111111111111111111111111111111111111111111111"},
  {"path":"chibi-metrics.jsonl","kind":"metrics","bytes":4096,"sha256":"2222222222222222222222222222222222222222222222222222222222222222"},
  {"path":"chibi-eval_samples.jsonl","kind":"eval_samples","bytes":2048,"sha256":"3333333333333333333333333333333333333333333333333333333333333333"}]}
```

   `chibi-metrics.jsonl`:

```
{"v":1,"seq":3,"ts":"2026-11-23T12:00:03Z","t":"metric","step":10,"max_steps":40,"epoch":0.25,"kind":"train","loss":2.5,"lr":0.0002,"grad_norm":1.5,"tok_s":2400,"elapsed_s":30,"eta_s":90,"vram_gb":6.1}
{"v":1,"seq":9,"ts":"2026-11-23T12:00:09Z","t":"metric","step":20,"max_steps":40,"epoch":0.5,"kind":"train","loss":2.2,"lr":0.00019,"grad_norm":1.3,"tok_s":2410,"elapsed_s":60,"eta_s":60,"vram_gb":6.1}
{"v":1,"seq":10,"ts":"2026-11-23T12:00:10Z","t":"metric","step":20,"max_steps":40,"epoch":0.5,"kind":"eval","loss":2.0}
not json at all
{"v":1,"seq":11,"ts":"2026-11-23T12:00:11Z","t":"stage","stage":"train","state":"done"}
{"step":40,"max_steps":40,"epoch":1,"loss":1.9}
{"v":1,"seq":12,"ts":"2026-11-23T12:00:12Z","t":"metric","step":10,"max_steps":40,"epoch":0.25,"kind":"train","loss":9.9}
```

   `chibi-eval_samples.jsonl`:

```
{"id":"0","prompt_messages":[{"role":"system","content":"Classify the customer message."},{"role":"user","content":"I was charged twice for the travel mug."}],"expected":"billing","base_output":"payment","tuned_output":"billing","base_ms":812,"tuned_ms":640,"truncated":false}
{"id":1,"prompt_text":"Day 12 at the Gull Point lighthouse.","expected":"The sea was calm.","base_output":"It rained.","tuned_output":"The sea was calm.","base_ms":1500,"tuned_ms":1400,"truncated":true}
{"id":"2","expected":"x","base_output":"y","tuned_output":"z"}
{broken
```

- [ ] 5. Create `logparse_test.go`. The helper `loadLogFixture(t, name) []byte` reads `testdata/logs/<name>` and replaces `{{PAD4200}}` with `strings.Repeat("a", 4200)`.
  - `TestParseKernelLogPlainText`:
    - The seqs are `[1, 2, 3, 7, 8, ExitSeq]`. That is: the duplicate is removed; seq 4 (`bogus`), seq 5 (`v:2`) and seq 6 (over-long) are dropped; the order is sorted.
    - `ev[2].Metric()` equals `Metric{Step:10, MaxSteps:40, Kind:"train", Epoch:0.25, Loss:2.5, LR:0.0002, GradNorm:1.5, TokS:2400, ElapsedS:30, EtaS:90, VRAMGB:6.1, TS:"2026-11-23T12:00:03Z"}`.
    - `ev[4].Metric()` has `Kind "eval"`, `Step 20`, `Loss 2.1`.
    - `ev[3].Error()` equals `{train, oom, "CUDA out of memory"}`.
    - `ev[0].Stage()` equals `{boot, start}`.
    - `ev[5].ExitCode()` equals `(0, true)`.
    - `Metric()` on a stage event returns false.
  - `TestParseKernelLogJSONArray`: the seqs are `[1, 2, 3, ExitSeq]`. Event 2 is reassembled and its `Stage()` is `{boot, done}`. Seq 3 keeps `Loss 2.5` (first wins).
  - `TestParseKernelLogTailCut`: cut `body` at the byte offset of the third occurrence of `{"stream_name"` minus 3. The parsed seqs are `[2, 3, ExitSeq]`.
  - `TestParseKernelLogLines` (table, inline bodies):

    | Body | Expected |
    |---|---|
    | CRLF line endings | still parsed |
    | `@@chibi` line exactly 4096 bytes long (`pad` length = 4096 − 8 − `len("{\"v\":1,\"seq\":1,\"t\":\"info\",\"pad\":\"")` − 2) | kept |
    | the same line 4097 bytes long | dropped |
    | `"seq":-1` | dropped |
    | a non-exit line without a seq | dropped |
    | trailing garbage after the object | dropped |
    | stage name `"compile"` | the event is kept, but `Stage()` returns false |
    | an error with code `"Bad Code"` | `Error()` returns false |
    | an empty body | an empty, non-nil slice |

- [ ] 6. Create `outputs_test.go`:
  - `TestParseStatus`:
    - The fixture parses: `Succeeded()`, `GPU.Capability == "7.5"`, `*Train.EvalLossTuned == 1.12`, `Train.TruncatedRows == 2`, `WarningCode == "base_gguf_skipped"`, `Discovery["converter_env"] == "system"`.
    - A variant with `"capability":7.5` (a number) parses to `"7.5"`.
    - A variant with `"train_loss":null` gives a nil `TrainLoss`.
    - A 3000-rune message is cut to 2000.
    - Rejected variants (table): `"v":2`, state `"running"`, stage `"compile"`, code `"Bad-Code"`, stop reason `"later"`, run id `"x"`, and a body over 1 MiB.
  - `TestParseManifest`:
    - The fixture parses with 3 files.
    - Each mutation in this table gives an error: path `"../chibi-x"`, path `"chibi-a/b"`, path `"model.gguf"`, path `"chibi-x:y"`, kind `"weights"`, kind `metrics` with path `chibi-model-q8_0.gguf`, the same file twice, sha `"ABC"`, bytes `-1`, bytes `9<<30`, `"v":2`, an invalid run id, and an empty `files` list.
    - `ValidManifestPath`: `"chibi-model-q8_0.gguf"` is true; `".."`, `"chibi-"`, `" chibi-x"` and `"chibi-..x"` are false.
  - `TestParseMetricsFile`:
    - The fixture gives exactly 4 points in order: `(10 train 2.5)`, `(20 eval 2.0)`, `(20 train 2.2)`, `(40 train 1.9)`.
    - A 5000-byte line followed by a valid line yields only the valid line.
    - A 20,005-line body yields 20,000 points.
  - `TestParseEvalSamples`:
    - The fixture gives 2 samples with IDs `"0"` and `"1"`, `PromptMessages` of 2 messages and `PromptText` set on the second.
    - `ParseEvalSamples(r, 1)` gives 1 sample.
    - A `tuned_output` of 9000 runes is cut to 8000.

- [ ] 7. Verify:

```
go build ./...
go test ./internal/chibisensei -run '(?i)(TestParseKernelLog|TestParseStatus|TestParseManifest|TestParseMetricsFile|TestParseEvalSamples)' -count=1 -v
go test ./internal/chibisensei/... -count=1
```

Expected: every listed test shows `--- PASS`; both packages `ok`.

---

#### Task B2a.6: Proof data and proof bundle

**Files**
- Create:
  - `internal/chibisensei/proofdata_test.go`, `internal/chibisensei/proofbundle_test.go`
  - `testdata/proof/assistant.jsonl`, `specialist.jsonl`, `free.jsonl` (generated)

**Existing symbols modified:** none

**Steps**

- [ ] 1. Create `proofdata_test.go`. All text is original and in English, about a fictional café, shop and lighthouse.

```go
package chibisensei

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var proofLabels = []string{"account", "billing", "delivery", "technical"}

var proofGoals = map[string]string{
	RecipeAssistant:  "Answer guests of the Lantern Corner café in one or two friendly sentences.",
	RecipeSpecialist: "Sort customer messages into account, billing, delivery or technical.",
	RecipeFree:       "Write logbook entries in the voice of the Gull Point lighthouse keeper.",
}

// proofDefaultRecipe is the recipe of each profile's proof run (contract, "Proof runs" step 6).
func proofDefaultRecipe(profileID string) string {
	if profileID == "gemma3-270m" {
		return RecipeSpecialist
	}
	return RecipeAssistant
}

type proofItem struct {
	name, base, temp, sweet, sizes, when, tip, other string
	price                                            int
}

func proofItems() []proofItem {
	adjectives := []string{"Amber", "Copper", "Misty", "Velvet", "Harbor"}
	nouns := []string{"Fizz", "Latte", "Cocoa", "Tonic"}
	bases := []string{"oat milk", "black tea", "pressed apple juice", "dark chocolate", "roasted barley",
		"sparkling water", "honey and lemon"}
	temps := []string{"hot", "iced", "warm", "chilled"}
	sweets := []string{"not sweet at all", "lightly sweet", "quite sweet"}
	sizes := []string{"small and large cups", "one standard cup", "small, medium and large cups"}
	whens := []string{"all day", "only in the morning", "from noon until closing", "on weekends"}
	tips := []string{"sip it slowly while it is fresh", "pair it with a slice of seed cake",
		"ask for a cinnamon stick on top", "enjoy it at the window seats by the harbor lights",
		"add a pinch of sea salt if you like a stronger taste"}
	items := make([]proofItem, 20)
	for i := range items {
		items[i] = proofItem{name: adjectives[i/4] + " " + nouns[i%4], base: bases[i%7], temp: temps[(i/2)%4],
			sweet: sweets[i%3], sizes: sizes[(i+1)%3], when: whens[(i/3)%4], tip: tips[i%5], price: 3 + (i*7)%6}
	}
	for i := range items {
		items[i].other = items[(i+5)%20].name
	}
	return items
}

var proofAssistantQA = []func(it proofItem) (string, string){
	func(it proofItem) (string, string) {
		return "What is in the " + it.name + "?", "The " + it.name + " is made with " + it.base + ", and it is " + it.sweet + "."
	},
	func(it proofItem) (string, string) {
		return "How much does the " + it.name + " cost?", fmt.Sprintf("The %s costs %d coins at Lantern Corner.", it.name, it.price)
	},
	func(it proofItem) (string, string) {
		return "Is the " + it.name + " served hot or cold?", "We serve the " + it.name + " " + it.temp + "."
	},
	func(it proofItem) (string, string) {
		return "Can you recommend something like the " + it.name + "?", "If you enjoy the " + it.name + ", try the " + it.other + " next time; many guests like both."
	},
	func(it proofItem) (string, string) {
		return "Is the " + it.name + " sweet?", "The " + it.name + " is " + it.sweet + "."
	},
	func(it proofItem) (string, string) {
		return "What sizes does the " + it.name + " come in?", "You can order the " + it.name + " in " + it.sizes + "."
	},
	func(it proofItem) (string, string) {
		return "Can I take the " + it.name + " away?", "Yes, we pack the " + it.name + " in a paper cup to take away."
	},
	func(it proofItem) (string, string) {
		return "When can I order the " + it.name + "?", "The " + it.name + " is available " + it.when + "."
	},
	func(it proofItem) (string, string) {
		return "Do you have a tip for enjoying the " + it.name + "?", "For the " + it.name + ", " + it.tip + "."
	},
	func(it proofItem) (string, string) {
		return "Does the " + it.name + " contain " + it.base + "?", "Yes, " + it.base + " is the main ingredient of the " + it.name + "."
	},
}

const proofAssistantSystem = "You are the friendly helper of Lantern Corner, a small café in a story world. Answer in one or two short sentences."

const proofSpecialistSystem = "Classify the customer message as account, billing, delivery or technical. Answer with the label only."

var proofObjects = []string{"the garden lamp", "my winter boots", "the tea sampler", "a set of brushes", "the travel mug",
	"my puzzle box", "the desk plant", "a pair of headphones", "the wool scarf", "my board game"}

var proofTemplates = map[string][]string{
	"account": {"I forgot the password for the account I used to buy %s.",
		"Please change the email address on my profile; I ordered %s last week.",
		"How do I delete my customer account after returning %s?",
		"My account is locked since I tried to review %s.",
		"Can I merge two customer accounts? One of them has the order for %s."},
	"billing": {"I was charged twice for %s.", "Where can I download the invoice for %s?",
		"The price of %s changed after I paid. Can you refund the difference?",
		"My card was declined when I tried to pay for %s.", "Please explain the extra fee on my bill for %s."},
	"delivery": {"My parcel with %s has not arrived yet.", "The courier left %s at the wrong door.",
		"Can I change the delivery address for %s?", "The tracking page for %s has not updated for four days.",
		"The box with %s arrived damaged."},
	"technical": {"The shop app crashes when I open the page for %s.", "The product photos of %s do not load in my browser.",
		"I cannot scan the QR code on the manual for %s.", "The checkout button does nothing when %s is in my basket.",
		"The website shows an error message when I search for %s."},
}

var (
	proofWeather = []string{"The morning began with a thin fog that lifted slowly over the cliffs.",
		"A steady west wind pushed grey clouds across the sky all day.", "Bright sunshine warmed the stone steps before noon.",
		"Light rain tapped on the lantern room windows until the evening.", "A cold north wind made the old door rattle in its frame.",
		"The sky stayed clear and pale blue from sunrise to sunset.", "Heavy clouds gathered in the afternoon, but no storm arrived."}
	proofSea = []string{"The sea was calm, and the fishing boats left the harbor early.",
		"Long swells rolled in from the open water and broke white on the rocks.",
		"The tide ran unusually low, showing the green weed on the lower ledges.",
		"Small choppy waves kept the ferry waiting at the pier for an hour.",
		"The water was so still that the cliffs were mirrored near the shore."}
	proofVisitors = []string{"No visitors climbed the path today.",
		"A family of four came to see the lamp and asked many questions about the clockwork.",
		"The postman brought two letters and a parcel of lamp wicks.",
		"A painter set up an easel near the fence and stayed until the light changed.",
		"Two hikers rested on the bench and shared their apples with the keeper.",
		"A school class visited and counted all one hundred and twelve steps aloud.",
		"The harbor master stopped by to compare notes on the tides.",
		"A birdwatcher reported a pair of puffins on the southern ledge.",
		"An old sailor told a long story about a winter storm from his youth."}
	proofTasks = []string{"The keeper polished the brass fittings until they shone.",
		"The lens was cleaned with a soft cloth and a little vinegar water.",
		"Fresh oil was carried up in two trips and the reservoir was filled.",
		"The weather vane was oiled because it had started to squeak.",
		"Loose paint was scraped from the railing and a new coat was applied.",
		"The logbook from last month was copied neatly for the archive.",
		"The clockwork that turns the lamp was wound and checked for wear.",
		"The rain gutter was cleared of leaves and gull feathers.",
		"New putty was pressed around a draughty window in the stairwell.",
		"The signal flags were counted, folded and stored in the dry chest.",
		"The garden by the cottage was weeded and the beans were watered."}
	proofClosings = []string{"The lamp was lit at dusk and burned steadily through the night.",
		"At sunset the lamp was lit, and the keeper watched the first ships pass.",
		"By evening everything was in order, and the beam swept the water as usual.",
		"The night watch was quiet, with only the sound of the waves below."}
)

// proofRows returns the 200 deterministic rows of a recipe.
func proofRows(recipe string) []RowInput {
	var out []RowInput
	switch recipe {
	case RecipeAssistant:
		for _, it := range proofItems() {
			for _, qa := range proofAssistantQA {
				q, a := qa(it)
				out = append(out, RowInput{Messages: []Message{{"system", proofAssistantSystem}, {"user", q}, {"assistant", a}}})
			}
		}
	case RecipeSpecialist:
		for _, obj := range proofObjects {
			for ti := 0; ti < 5; ti++ {
				for _, label := range proofLabels {
					out = append(out, RowInput{Messages: []Message{{"system", proofSpecialistSystem},
						{"user", fmt.Sprintf(proofTemplates[label][ti], obj)}, {"assistant", label}}})
				}
			}
		}
	case RecipeFree:
		for d := 1; d <= 200; d++ {
			text := fmt.Sprintf("Day %d at the Gull Point lighthouse. %s %s %s %s The barometer read %d millibars at noon, and the keeper counted %d gulls on the roof of the oil store. %s",
				d, proofWeather[d%7], proofSea[d%5], proofVisitors[d%9], proofTasks[d%11], 990+(d*13)%40, 3+(d*7)%23, proofClosings[d%4])
			out = append(out, RowInput{Text: &text})
		}
	}
	return out
}

func proofJSONL(rows []RowInput) []byte {
	var buf bytes.Buffer
	for _, r := range rows {
		row := Row{Kind: rowKindChat, Messages: r.Messages}
		if r.Text != nil {
			row = Row{Kind: rowKindText, Text: r.Text}
		}
		buf.Write(ExportLine(row))
	}
	return buf.Bytes()
}

// readProofRows reads the committed proof file of a recipe.
func readProofRows(t *testing.T, recipe string) []RowInput {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "proof", recipe+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out []RowInput
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var w struct {
			Messages []Message `json:"messages"`
			Text     *string   `json:"text"`
		}
		if err := json.Unmarshal([]byte(line), &w); err != nil {
			t.Fatal(err)
		}
		out = append(out, RowInput{Messages: w.Messages, Text: w.Text})
	}
	return out
}

func TestProofData(t *testing.T) {
	for _, recipe := range []string{RecipeAssistant, RecipeSpecialist, RecipeFree} {
		rows := proofRows(recipe)
		if len(rows) != 200 {
			t.Fatalf("%s: %d rows", recipe, len(rows))
		}
		hashes, labels, tokens := map[string]bool{}, map[string]int{}, 0
		for i, r := range rows {
			if is := ValidateRowInput(r); len(is) > 0 {
				t.Fatalf("%s row %d: %+v", recipe, i, is)
			}
			kind, text := rowKindChat, ""
			if r.Text != nil {
				kind, text = rowKindText, *r.Text
			}
			if p := StructureProblems(kind, r.Messages, recipe); len(p) > 0 {
				t.Fatalf("%s row %d: %v", recipe, i, p)
			}
			h := ContentHash(kind, r.Messages, text)
			if hashes[h] {
				t.Fatalf("%s row %d duplicates an earlier row", recipe, i)
			}
			hashes[h] = true
			tokens += EstimateTokens(r.Messages, text)
			if recipe == RecipeSpecialist {
				labels[LastAssistant(r.Messages)]++
			}
		}
		if recipe == RecipeSpecialist {
			for _, l := range proofLabels {
				if labels[l] != 50 {
					t.Fatalf("label %s has %d rows", l, labels[l])
				}
			}
		}
		if recipe == RecipeFree && tokens < freeMinTokens {
			t.Fatalf("free rows hold %d estimated tokens, want >= %d", tokens, freeMinTokens)
		}
		got := proofJSONL(rows)
		path := filepath.Join("testdata", "proof", recipe+".jsonl")
		if *updateGolden {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if want, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s differs from the generator (err %v); run with -update", path, err)
		}
	}
}
```

- [ ] 2. Create `proofbundle_test.go`:

```go
package chibisensei

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"aurago/internal/chibisensei/trainer"
)

const trainerPlaceholderMarker = "CHIBI_TRAINER_PLACEHOLDER = True"

type proofBundleFile struct {
	Name   string `json:"name"`
	Role   string `json:"role"`   // train | val | eval | marker | job | notebook
	Upload bool   `json:"upload"` // part of the Kaggle dataset
	Rows   int    `json:"rows"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type proofBundle struct {
	V                     int               `json:"v"`
	RunID                 string            `json:"run_id"`
	Run12                 string            `json:"run12"`
	ProfileID             string            `json:"profile_id"`
	ProfileSHA            string            `json:"profile_sha"`
	Recipe                string            `json:"recipe"`
	Preset                string            `json:"preset"`
	Method                string            `json:"method"`
	Owner                 string            `json:"owner"` // filled in by hand: the Kaggle account
	DatasetSlug           string            `json:"dataset_slug"`
	NotebookSlug          string            `json:"notebook_slug"`
	MachineShape          string            `json:"machine_shape"`
	SessionTimeoutSeconds int               `json:"session_timeout_seconds"`
	EstimateMinutes       int               `json:"estimate_minutes"`
	MaxTrainSeconds       int               `json:"max_train_seconds"`
	ScriptSHA256          string            `json:"script_sha256"`
	DataSHA               string            `json:"data_sha"`
	Files                 []proofBundleFile `json:"files"`
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func writeBundleFile(t *testing.T, dir, name, role string, data []byte) proofBundleFile {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return proofBundleFile{Name: name, Role: role, Bytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
}

// TestProofBundle writes the files of one manual proof run (contract, "Proof runs" step 2). It is skipped
// unless CHIBI_PROOF_OUT is set.
func TestProofBundle(t *testing.T) {
	out := os.Getenv("CHIBI_PROOF_OUT")
	if out == "" {
		t.Skip("set CHIBI_PROOF_OUT=<dir> and CHIBI_PROOF_PROFILE=<id> to build a proof bundle")
	}
	profileID := os.Getenv("CHIBI_PROOF_PROFILE")
	c := mustCatalog(t)
	prof, ok := c.Get(profileID)
	if !ok {
		t.Fatalf("CHIBI_PROOF_PROFILE %q is not a profile", profileID)
	}
	recipe := envOr("CHIBI_PROOF_RECIPE", proofDefaultRecipe(profileID))
	preset := envOr("CHIBI_PROOF_PRESET", "quick")
	timeout, err := strconv.Atoi(envOr("CHIBI_PROOF_TIMEOUT_MINUTES", "60"))
	if err != nil || timeout < 30 || timeout > 720 {
		t.Fatalf("CHIBI_PROOF_TIMEOUT_MINUTES must be 30 to 720")
	}
	script := trainer.Script()
	if bytes.Contains(script, []byte(trainerPlaceholderMarker)) {
		t.Fatal("the embedded trainer is still the placeholder")
	}
	lock, err := trainer.LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, err := OpenStore(filepath.Join(t.TempDir(), "chibi_sensei.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	in := ProjectInput{Name: "Proof " + profileID, Recipe: recipe, GoalText: proofGoals[recipe], ProfileID: profileID}
	if recipe == RecipeSpecialist {
		in.TaskKind, in.Labels = "classify", proofLabels
	}
	proj, err := st.CreateProject(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	rows := readProofRows(t, recipe)
	res, err := st.AddRows(ctx, proj.ID, rows, AddOptions{Source: "file", BatchKind: "import", MaxRows: 50000, Strict: true})
	if err != nil || res.Added != len(rows) {
		t.Fatalf("add rows: added %d of %d, err %v", res.Added, len(rows), err)
	}
	crows, proj, err := st.LoadCheckRows(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	srows := make([]SplitRow, 0, len(crows))
	for _, r := range crows {
		srows = append(srows, SplitRow{ID: r.ID, Split: r.Split, Label: r.Label, EstTokens: r.EstTokens})
	}
	split := ComputeSplit(srows, proj.SplitSeed, proj.TaskKind == "classify")
	plan, err := ComputePlan(PlanInput{Project: proj, Profile: prof, Rows: srows, Split: split, Preset: preset,
		RemoteTimeoutMinutes: timeout, NeedBaseGGUF: true})
	if err != nil {
		t.Fatal(err)
	}
	checks, flags := RunChecks(CheckInput{Project: proj, Profile: prof, Rows: crows, Split: split,
		MaxSeqLen: plan.Hyper.MaxSeqLen}, time.Now().UTC())
	if !checks.Ready {
		var blocks []string
		for _, ch := range checks.Checks {
			if ch.Level == "block" {
				blocks = append(blocks, ch.ID)
			}
		}
		t.Fatalf("dataset not ready: %v", blocks)
	}
	if err := st.SaveChecks(ctx, checks, flags); err != nil {
		t.Fatal(err)
	}
	runID := uuid.NewString()
	snap, err := st.FreezeSnapshot(ctx, proj.ID, runID, out)
	if err != nil {
		t.Fatal(err)
	}
	if snap.DataRev != plan.DataRev {
		t.Fatalf("snapshot data_rev %d, plan %d", snap.DataRev, plan.DataRev)
	}
	spec, err := BuildJobSpec(JobInput{RunID: runID, Created: time.Now().UTC().Truncate(time.Second),
		AuraGoVersion: "proof-bundle", Project: proj, Profile: prof, ProfileSHA: c.SHA(profileID), Plan: plan,
		Marker: snap.Marker.Name, Format: snap.Format, Lock: lock})
	if err != nil {
		t.Fatal(err)
	}
	job, err := spec.JSON()
	if err != nil {
		t.Fatal(err)
	}
	nb, err := BuildNotebook(runID, job, script)
	if err != nil {
		t.Fatal(err)
	}
	if f := append(jobFindings(job), notebookFindings(nb, job, script)...); len(f) > 0 {
		t.Fatalf("payload check: %v", f)
	}
	run12 := Run12(runID)
	b := proofBundle{V: 1, RunID: runID, Run12: run12, ProfileID: profileID, ProfileSHA: c.SHA(profileID),
		Recipe: recipe, Preset: plan.Preset, Method: plan.Method, DatasetSlug: DatasetSlug(runID),
		NotebookSlug: NotebookSlug(proj.ID, runID), MachineShape: "NvidiaTeslaT4",
		SessionTimeoutSeconds: plan.SessionTimeoutMinutes * 60, EstimateMinutes: plan.EstimateMinutes,
		MaxTrainSeconds: plan.MaxTrainSeconds, ScriptSHA256: trainer.ScriptSHA256(), DataSHA: snap.DataSHA}
	for _, split := range SnapshotSplits {
		if f, ok := snap.Files[split]; ok {
			b.Files = append(b.Files, proofBundleFile{Name: f.Name, Role: split, Upload: true, Rows: f.Rows, Bytes: f.Bytes, SHA256: f.SHA256})
		}
	}
	b.Files = append(b.Files, proofBundleFile{Name: snap.Marker.Name, Role: "marker", Upload: true,
		Bytes: snap.Marker.Bytes, SHA256: snap.Marker.SHA256})
	b.Files = append(b.Files, writeBundleFile(t, out, "r"+run12+"-job.json", "job", job))
	b.Files = append(b.Files, writeBundleFile(t, out, "r"+run12+"-notebook.ipynb", "notebook", nb))
	summary, err := marshalStable(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "bundle.json"), summary, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("proof bundle %s for %s (%s, %s): %d files in %s", runID, profileID, recipe, plan.Preset, len(b.Files), out)
}
```

- [ ] 3. Generate the proof data, check it, and confirm the bundle test skips by default:

```
go test ./internal/chibisensei -run '(?i)TestProofData' -count=1 -update
go test ./internal/chibisensei -run '(?i)(TestProofData|TestProofBundle)' -count=1 -v
```

   Expected:
   - `TestProofData` shows `--- PASS`.
   - `TestProofBundle` shows `--- SKIP` with the CHIBI_PROOF_OUT message.
   - `testdata/proof/` holds three files of 200 lines each.
   - In `specialist.jsonl`, `grep -c '"content":"billing"'` reports 50.

- [ ] 4. Smoke-test the bundle path only once B2b has replaced the placeholder. If the placeholder is still in place, the test must fail with "still the placeholder"; report that and stop.

```
CHIBI_PROOF_OUT=C:/disposable/chibi-sensei/proof/gemma CHIBI_PROOF_PROFILE=gemma3-270m go test ./internal/chibisensei -run TestProofBundle -count=1 -v
```

   Expected: `--- PASS`. The directory holds:
   - `r<run12>-train.jsonl`, `r<run12>-val.jsonl`, `r<run12>-eval.jsonl` (200 rows, so val is 10);
   - `r<run12>-marker.json`, `r<run12>-job.json`, `r<run12>-notebook.ipynb`;
   - `bundle.json`, with `machine_shape` `NvidiaTeslaT4` and an empty `owner`.

- [ ] 5. Verify:

```
go build ./...
go test ./internal/chibisensei/... -count=1
```

Expected: `ok` for `aurago/internal/chibisensei` and `aurago/internal/chibisensei/trainer`.

Guides: no `AGENTS.md` changes in B2. B5 records the facts in contract change 17.

---

## Chibi Sensei P2 backend plan, part 3 (B2b)

### Contract changes

1. **New stage `converter` between `load_base` and `base_eval`.** It downloads and verifies the llama.cpp tarball, picks the converter environment (D9), and writes the base GGUF right after the base download (or, when `emit_base_gguf` is false, runs a `--vocab-only` probe into the work dir). Reason: when the converter cannot read the architecture, the run fails after about 5 minutes and not after training (about 30 minutes of GPU quota). At the end, the `gguf` stage converts only the tuned model, using the environment the probe chose. The UI shows stage names as they arrive, so it needs no change except a label key `converter`.
2. **`data` runs before `install`.** It uses the standard library only, and a missing marker or hash mismatch now fails before minutes of pip installs. Final order: `boot, env_check, data, install, load_base, converter, base_eval, train, tuned_eval, save, gguf, manifest`.
3. **Extra argument `--input <dir>`** (default `/kaggle/input`): the root that is searched for the marker. The dry-run test and later P5 need it. The notebook does not pass it.
4. **More run failure codes**, in addition to the spec list and `lora_targets_missing`: `job_invalid` (boot), `env_low_disk`, `env_unsupported_python`, `base_load_failed` (download worked, no class could load), `eval_failed`, `train_failed`, `output_failed`, `interrupted` (SIGTERM/SIGINT), `internal`. B3 and F2 need message keys for them, or show a generic message for an unknown code. `time_budget` with zero finished steps is a failure. With at least one step it is a success, with `warning_code: "time_budget"`.
5. **`chibi-status.json` shape.**
   - `train.epochs` is a number and may be fractional (`1.47` after a time-budget stop), so the Go field must be `float64`.
   - New top-level `warning_code` (`time_budget` or `base_gguf_skipped`, the first of these to occur). All other warnings are listed under `discovery.warnings`.
   - `discovery` carries more keys than D3 lists (see "What the first real run will tell us").
   - `ParseStatus` must not use `DisallowUnknownFields`.
6. **Eval samples.**
   - `id` is a string: the row's own `id` if the snapshot line has one, otherwise `"eval-<line>"` (1-based line in the eval file).
   - `base_output`, `tuned_output`, `base_ms` and `tuned_ms` are `null` when that answer was not generated (time cap or generation failure). `EvalSample` should use `*string` / `*int`, or `ScoreReport` must skip null answers.
   - The file always exists (it is empty when there are no eval rows).
7. **Metric lines** carry `kind` (`train` | `eval`). An eval metric repeats the val loss in both `loss` and `eval_loss`. Lines in `chibi-metrics.jsonl` are the full log objects (`v`, `seq`, `ts`, `t` included). `t` also takes the value `info` (discovery lines, with a `kind` field).
8. **The manifest is written in `finalize`, after the final status**, so it can list `chibi-status.json` with its hash. It is also written on failure, listing whatever outputs exist. The `manifest` stage only checks that the required outputs are present.
9. **Base GGUF skip rule**, evaluated at the `converter` stage: skip when `remaining − max_train_s − 420 s < 360 s`. Here `remaining` = session timeout − 90 s notebook lead − elapsed.
10. **VRAM is measured in decimal GB (1e9 bytes).** A T4 reports about 15.8 GB but only 14.75 GiB, and the LFM profile asks for 15.
11. **When the lock lists no pip pins, missing packages are installed unpinned:** `peft` (required for LoRA), `huggingface_hub`, `safetensors` (required), `sentencepiece` (optional). Each install gives a `warn unpinned_dependency` line with the installed version, which the proof run records into the lock.
12. **Tokenization happens at the end of `load_base`.** A row that keeps no answer tokens after templating fails with `data_mismatch`.

### File map

| File | Batch | Responsibility |
|---|---|---|
| `internal/chibisensei/trainer/chibi_train.py` | B2 | The trainer: stages, structured log, status, manifest, selftest, dry run |
| `internal/chibisensei/trainer/selftest_test.go` | B2 | Runs `--selftest` and per-profile `--dry-run`, then parses every output with the production parsers |

#### Task B2b: Trainer script and its Go test

**Files**
- Create: `internal/chibisensei/trainer/chibi_train.py`, `internal/chibisensei/trainer/selftest_test.go`
- Depends on B2a (same batch, done first): `trainer/embed.go` (`Script`, `LoadLock`), `jobspec.go`, `snapshot.go`, `logparse.go`, `outputs.go`, `plan.go` fields.

**Existing symbols modified:** none.

**Steps**

Write the script exactly as given. Steps 1–16 are contiguous parts of one file, in file order; concatenate them. Use LF line endings and no BOM.

- [ ] 1. Header, imports, constants, small helpers.

```python
#!/usr/bin/env python3
"""AuraGo Chibi Sensei trainer, job.json schema v1.

One fine-tuning run in stages (see STAGES). Every stage reports through "@@chibi {json}" lines on
stdout; chibi-status.json and chibi-manifest.json are written in a finally block. Only the standard
library is imported at module level; torch, transformers, peft, huggingface_hub and gguf are imported
inside the stages, so --selftest and the dry run work without them.

  chibi_train.py --job job.json --out /kaggle/working [--input /kaggle/input]
  chibi_train.py --selftest
  CHIBI_FAKE_ENV=1 chibi_train.py --dry-run --job job.json --out DIR --input DIR
"""
import argparse
import gc
import hashlib
import importlib.metadata
import importlib.util
import io
import json
import math
import os
import random
import re
import shutil
import signal
import struct
import subprocess
import sys
import tarfile
import tempfile
import time
import traceback
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone

LOG_PREFIX = "@@chibi"
MAX_LINE_BYTES = 4096
DEFAULT_INPUT = "/kaggle/input"
WORK_ROOT = "/tmp/chibi/work"
SESSION_LEAD_S = 90          # kernel start and notebook cell before this script began
SAVE_GGUF_RESERVE_S = 420    # merge, save, tuned conversion, manifest
BASE_GGUF_MIN_S = 360        # contract: skip the base GGUF below six spare minutes
TAIL_LINES = 40
MARKER_DEPTH = 4
HEAVY_MODULES = ("torch", "transformers", "peft", "huggingface_hub", "gguf", "safetensors")
SECRET_ENV = ("HF_TOKEN", "HUGGING_FACE_HUB_TOKEN", "HUGGINGFACE_HUB_TOKEN", "HF_API_TOKEN")
NON_TEXT_MARKERS = ("vision", "visual", "audio", "mm_projector", "multi_modal_projector")
TOKENIZER_FILES = ("tokenizer.json", "tokenizer_config.json", "tokenizer.model", "special_tokens_map.json",
                   "vocab.json", "merges.txt", "added_tokens.json", "chat_template.jinja", "chat_template.json")
SNAPSHOT_IGNORE = ["*.gguf", "*.onnx", "*.msgpack", "*.h5", "*.ot", "*.pth", "original/*", "onnx/*"]
MANIFEST_NAME = "chibi-manifest.json"
MANIFEST_KINDS = ("gguf_tuned", "gguf_base", "adapter", "adapter_config", "metrics", "eval_samples", "status")


def output_names(outtype):
    return {"gguf_tuned": "chibi-model-%s.gguf" % outtype, "gguf_base": "chibi-base-%s.gguf" % outtype,
            "adapter": "chibi-adapter-model.safetensors", "adapter_config": "chibi-adapter-config.json",
            "metrics": "chibi-metrics.jsonl", "eval_samples": "chibi-eval_samples.jsonl",
            "status": "chibi-status.json"}


def utc_now():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def finite(x):
    try:
        f = float(x)
    except (TypeError, ValueError):
        return None
    return f if math.isfinite(f) else None


def rnd(x, n=4):
    f = finite(x)
    return None if f is None else round(f, n)


def sanitize(obj):
    """JSON-safe copy: non-finite floats become None, unknown objects become strings."""
    if isinstance(obj, float):
        return obj if math.isfinite(obj) else None
    if obj is None or isinstance(obj, (str, int, bool)):
        return obj
    if isinstance(obj, dict):
        return {str(k): sanitize(v) for k, v in obj.items()}
    if isinstance(obj, (list, tuple)):
        return [sanitize(v) for v in obj]
    return str(obj)


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def read_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def _tmp_path(path):
    return os.path.join(os.path.dirname(path) or ".", ".tmp-" + os.path.basename(path))


def write_json_atomic(path, obj):
    tmp = _tmp_path(path)
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(sanitize(obj), f, ensure_ascii=False, indent=1, allow_nan=False)
        f.write("\n")
    os.replace(tmp, path)


def write_jsonl_atomic(path, rows):
    tmp = _tmp_path(path)
    with open(tmp, "w", encoding="utf-8") as f:
        for row in rows:
            f.write(json.dumps(sanitize(row), ensure_ascii=False, separators=(",", ":"), allow_nan=False) + "\n")
    os.replace(tmp, path)


def one_line(text, limit=700):
    text = " ".join(str(text).split())
    return text if len(text) <= limit else text[: limit - 3] + "..."


def tail_lines(text, n=TAIL_LINES):
    return [line.rstrip() for line in str(text).splitlines() if line.strip()][-n:]


def last_line(text):
    lines = tail_lines(text, 1)
    return one_line(lines[0], 300) if lines else ""


def safe_name(name):
    return (isinstance(name, str) and 0 < len(name) <= 200 and "/" not in name and "\\" not in name
            and not name.startswith(".") and "\x00" not in name)


def dist_version(name):
    try:
        return importlib.metadata.version(name)
    except importlib.metadata.PackageNotFoundError:
        return ""


def heavy_loaded():
    return sorted(m for m in HEAVY_MODULES if m in sys.modules)


def dir_bytes(path):
    total = 0
    for dirpath, _, files in os.walk(path):
        for name in files:
            total += os.path.getsize(os.path.join(dirpath, name))
    return total
```

- [ ] 2. Structured log.

```python
def encode_line(prefix, obj, limit=MAX_LINE_BYTES):
    """One ASCII line of at most limit bytes; the largest fields are halved until it fits."""
    obj = sanitize(obj)
    for _ in range(32):
        line = prefix + " " + json.dumps(obj, ensure_ascii=True, separators=(",", ":"))
        if len(line) <= limit:
            return line
        key, size = None, 0
        for k, v in obj.items():
            if k in ("v", "seq", "ts", "t", "cut"):
                continue
            s = len(json.dumps(v, ensure_ascii=True))
            if s > size:
                key, size = k, s
        if key is None or size <= 24:
            break
        value = obj[key]
        text = value if isinstance(value, str) else json.dumps(value, ensure_ascii=True)
        obj = dict(obj)
        obj[key] = text[: max(8, len(text) // 2)] + "..."
        obj["cut"] = True
    keep = {k: obj[k] for k in ("v", "seq", "ts", "t", "stage", "state", "code", "kind") if k in obj}
    keep["cut"] = True
    return prefix + " " + json.dumps(keep, ensure_ascii=True, separators=(",", ":"))


class Log:
    """'@@chibi {json}' lines flushed one by one; metric objects also go to chibi-metrics.jsonl."""

    def __init__(self, stream=None, prefix=LOG_PREFIX):
        self.stream = stream or sys.stdout
        self.prefix = prefix
        self.seq = 0
        self.metrics_path = None

    def emit(self, t, **fields):
        self.seq += 1
        obj = {"v": 1, "seq": self.seq, "ts": utc_now(), "t": t}
        obj.update(fields)
        try:
            self.stream.write(encode_line(self.prefix, obj) + "\n")
            self.stream.flush()
        except (OSError, ValueError):
            pass
        return obj

    def info(self, kind, **fields):
        return self.emit("info", kind=kind, **fields)

    def metric(self, **fields):
        obj = self.emit("metric", **fields)
        if self.metrics_path:
            with open(self.metrics_path, "a", encoding="utf-8") as f:
                f.write(json.dumps(sanitize(obj), ensure_ascii=False, separators=(",", ":")) + "\n")
        return obj

    def raw(self, title, lines):
        """Unprefixed text for people reading the Kaggle log; the parser ignores it."""
        try:
            self.stream.write("----- %s -----\n%s\n-----\n" % (title, "\n".join(lines)))
            self.stream.flush()
        except (OSError, ValueError):
            pass
```

- [ ] 3. Errors, environment, budget, schedule.

```python
class StageError(Exception):
    """A failure with a run failure code (spec 'Error handling' plus the contract additions)."""

    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = one_line(message)


class RunFailed(Exception):
    pass


class Interrupted(BaseException):
    pass


def is_oom(exc):
    for _ in range(6):
        if exc is None:
            return False
        if isinstance(exc, MemoryError) or type(exc).__name__ == "OutOfMemoryError":
            return True
        if "out of memory" in str(exc).lower():
            return True
        exc = exc.__cause__ or exc.__context__
    return False


def exc_summary(exc):
    return one_line("%s: %s" % (type(exc).__name__, exc), 700)


def trace_tail(exc, frames=6):
    tb = traceback.extract_tb(exc.__traceback__)[-frames:]
    return ["%s:%d %s" % (os.path.basename(f.filename), f.lineno, f.name) for f in tb]


def harden_stdio():
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(errors="backslashreplace")
        except (AttributeError, ValueError):
            pass


def scrub_environment():
    """Remove tokens and Kaggle variables (never printed). Returns (count, kernel run type)."""
    run_type = os.environ.get("KAGGLE_KERNEL_RUN_TYPE", "")
    removed = 0
    for key in list(os.environ):
        if key in SECRET_ENV or key.startswith("KAGGLE_"):
            del os.environ[key]
            removed += 1
    os.environ.update({"HF_HUB_DISABLE_TELEMETRY": "1", "HF_HUB_DISABLE_PROGRESS_BARS": "1",
                       "TOKENIZERS_PARALLELISM": "false", "CUDA_VISIBLE_DEVICES": "0",
                       "PYTORCH_CUDA_ALLOC_CONF": "expandable_segments:True",
                       "PIP_DISABLE_PIP_VERSION_CHECK": "1", "PIP_NO_INPUT": "1",
                       "PIP_ROOT_USER_ACTION": "ignore", "PYTHONUNBUFFERED": "1"})
    return removed, run_type


def install_signal_handlers():
    def handler(signum, frame):
        raise Interrupted("signal %d" % signum)
    for name in ("SIGTERM", "SIGINT"):
        sig = getattr(signal, name, None)
        if sig is not None:
            try:
                signal.signal(sig, handler)
            except (ValueError, OSError):
                pass


class Budget:
    """One monotonic clock from script start; the Kaggle session clock started SESSION_LEAD_S earlier."""

    def __init__(self, session_s, max_train_s, clock=time.monotonic, t0=None):
        self.clock = clock
        self.t0 = clock() if t0 is None else t0
        self.session_s = max(0, int(session_s))
        self.max_train_s = max(0, int(max_train_s))

    def elapsed(self):
        return self.clock() - self.t0

    def remaining(self):
        return self.session_s - SESSION_LEAD_S - self.elapsed()

    def train_deadline(self, post_reserve_s):
        """Monotonic time at which training stops: max_train_s from now, or earlier for the session."""
        by_budget = self.clock() + self.max_train_s
        by_session = self.t0 + self.session_s - SESSION_LEAD_S - post_reserve_s
        return min(by_budget, by_session)


def generation_plan(n, max_new, gen_tps, available_s):
    """(per-sample cap, total cap) in seconds for n greedy answers."""
    gen_tps = max(1.0, float(gen_tps or 1.0))
    per = max(15.0, 3.0 * max_new / gen_tps)
    want = 1.5 * n * max_new / gen_tps + 60.0
    return per, max(0.0, min(want, available_s))


def lr_factor(step, warmup, total):
    """Linear warmup, then cosine decay to zero; step is 0-based."""
    if warmup > 0 and step < warmup:
        return (step + 1) / float(warmup)
    progress = (step - warmup) / float(max(1, total - warmup))
    return 0.5 * (1.0 + math.cos(math.pi * min(1.0, max(0.0, progress))))
```

- [ ] 4. Job validation.

```python
JOB_FIELDS = (
    ("v", int), ("run_id", str), ("profile_sha", str), ("recipe", str), ("method", str),
    ("data.marker", str), ("data.format", str),
    ("hyper.epochs", int), ("hyper.learning_rate", float), ("hyper.per_device_batch", int),
    ("hyper.grad_accum", int), ("hyper.max_seq_len", int), ("hyper.seed", int),
    ("budget.session_timeout_s", int), ("budget.max_train_s", int), ("budget.eval_max_samples", int),
    ("budget.max_new_tokens", int), ("outputs.gguf_outtype", str), ("outputs.emit_base_gguf", bool),
    ("deps.llama_cpp.tag", str), ("deps.llama_cpp.url", str),
    ("profile.id", str), ("profile.base.repo", str), ("profile.base.revision", str),
    ("profile.base.auto_class", str), ("profile.chat.system_role", str), ("profile.gguf.expected_arch", str),
    ("profile.training.min_gpu_capability", str), ("profile.training.min_vram_gb", float),
    ("profile.training.t4_gen_tokens_per_second", float), ("profile.arch.params_b", float),
)


def jget(obj, path, default=None):
    cur = obj
    for part in path.split("."):
        if not isinstance(cur, dict) or part not in cur:
            return default
        cur = cur[part]
    return cur


def type_ok(value, kind):
    if kind is bool:
        return isinstance(value, bool)
    if isinstance(value, bool):
        return False
    if kind is float:
        return isinstance(value, (int, float)) and math.isfinite(value)
    return isinstance(value, kind)


def str_list(value):
    return isinstance(value, list) and all(isinstance(x, str) for x in value)


def validate_job(job):
    """Problems of a job.json (schema v1) as 'path: reason' strings; empty when valid."""
    if not isinstance(job, dict):
        return ["job: must be a JSON object"]
    problems = []
    for path, kind in JOB_FIELDS:
        if not type_ok(jget(job, path), kind):
            problems.append("%s: missing or not %s" % (path, kind.__name__))
    if problems:
        return problems
    h, b = job["hyper"], job["budget"]
    if job["v"] != 1:
        problems.append("v: must be 1")
    if job["method"] not in ("lora", "full"):
        problems.append("method: must be lora or full")
    if job["recipe"] not in ("assistant", "specialist", "free"):
        problems.append("recipe: must be assistant, specialist or free")
    fmt = job["data"]["format"]
    if fmt not in ("messages", "text"):
        problems.append("data.format: must be messages or text")
    elif (job["recipe"] == "free") != (fmt == "text"):
        problems.append("data.format: the free recipe uses text rows, the others messages")
    if not safe_name(job["data"]["marker"]):
        problems.append("data.marker: must be a plain file name")
    for key in ("epochs", "per_device_batch", "grad_accum"):
        if h[key] < 1:
            problems.append("hyper.%s: must be at least 1" % key)
    if not 16 <= h["max_seq_len"] <= 8192:
        problems.append("hyper.max_seq_len: must be 16 to 8192")
    if h["learning_rate"] <= 0:
        problems.append("hyper.learning_rate: must be above 0")
    if job["method"] == "lora":
        lora = h.get("lora")
        if (not isinstance(lora, dict) or not type_ok(lora.get("r"), int) or lora["r"] < 1
                or not type_ok(lora.get("alpha"), float) or not type_ok(lora.get("dropout", 0.0), float)):
            problems.append("hyper.lora: r, alpha and dropout are required for method lora")
        else:
            targets = lora.get("target_modules") or jget(job, "profile.training.lora.target_modules") or []
            if not str_list(targets) or not targets:
                problems.append("hyper.lora.target_modules: needs at least one name")
    if b["max_train_s"] < 1 or b["session_timeout_s"] <= b["max_train_s"]:
        problems.append("budget: max_train_s must be positive and below session_timeout_s")
    if b["eval_max_samples"] < 0 or b["max_new_tokens"] < 1:
        problems.append("budget: eval_max_samples >= 0 and max_new_tokens >= 1 required")
    if not re.fullmatch(r"[a-z0-9_]{1,16}", job["outputs"]["gguf_outtype"]):
        problems.append("outputs.gguf_outtype: invalid")
    for path in ("deps.pip", "profile.chat.stop", "profile.training.extra_pip", "profile.base.auto_class_fallbacks"):
        if not str_list(jget(job, path) or []):
            problems.append("%s: must be a list of strings" % path)
    for path in ("deps.transformers", "deps.llama_cpp.sha256"):
        if not isinstance(jget(job, path) or "", str):
            problems.append("%s: must be a string" % path)
    return problems
```

- [ ] 5. Dataset marker, hashes and rows.

```python
def find_files(root, names, max_depth=MARKER_DEPTH):
    """Breadth-first, depth-limited search; returns ({name: path}, directories searched)."""
    found, searched, queue = {}, 0, [(root, 0)]
    while queue:
        directory, depth = queue.pop(0)
        try:
            with os.scandir(directory) as it:
                entries = sorted(it, key=lambda e: e.name)
        except OSError:
            continue
        searched += 1
        for e in entries:
            try:
                if e.is_file() and e.name in names and e.name not in found:
                    found[e.name] = e.path
                elif e.is_dir() and depth < max_depth:
                    queue.append((e.path, depth + 1))
            except OSError:
                continue
    return found, searched


def list_tree(root, max_depth=2, limit=40):
    out = []
    for dirpath, dirnames, filenames in os.walk(root):
        if dirpath[len(root):].count(os.sep) >= max_depth:
            dirnames[:] = []
        for name in sorted(dirnames + filenames):
            out.append(os.path.relpath(os.path.join(dirpath, name), root))
            if len(out) >= limit:
                return out
    return out


def load_marker(root, name, run_id):
    found, searched = find_files(root, {name})
    if name not in found:
        raise StageError("data_mismatch", "marker %s not found under %s (%d directories searched; is the "
                                          "dataset attached to the notebook?)" % (name, root, searched))
    try:
        marker = read_json(found[name])
    except (OSError, ValueError) as e:
        raise StageError("data_mismatch", "marker cannot be read: %s" % exc_summary(e))
    if not isinstance(marker, dict) or marker.get("v") != 1 or not isinstance(marker.get("files"), dict):
        raise StageError("data_mismatch", "marker has no v=1 files object")
    if marker.get("run_id") != run_id:
        raise StageError("data_mismatch", "marker belongs to run %s, not %s" % (marker.get("run_id"), run_id))
    return marker, os.path.dirname(found[name])


def verify_split_files(marker, base_dir, root):
    files = {}
    for split in ("train", "val", "eval"):
        entry = marker["files"].get(split)
        if entry is None:
            continue
        name = entry.get("name") if isinstance(entry, dict) else None
        if not safe_name(name) or not isinstance(entry.get("sha256"), str):
            raise StageError("data_mismatch", "marker entry %s is malformed" % split)
        path = os.path.join(base_dir, name)
        if not os.path.isfile(path):
            path = find_files(root, {name})[0].get(name)
        if not path:
            raise StageError("data_mismatch", "%s file %s is missing" % (split, name))
        got = sha256_file(path)
        if got != entry["sha256"].lower():
            raise StageError("data_mismatch", "%s file %s has sha256 %s, the marker says %s"
                             % (split, name, got, entry["sha256"]))
        files[split] = (path, entry)
    if "train" not in files:
        raise StageError("data_mismatch", "the marker lists no train file")
    return files


def row_problem(obj, fmt):
    if not isinstance(obj, dict):
        return "row is not an object"
    if fmt == "text":
        text = obj.get("text")
        return None if isinstance(text, str) and text.strip() else "text row without text"
    msgs = obj.get("messages")
    if not isinstance(msgs, list) or len(msgs) < 2:
        return "needs at least two messages"
    for m in msgs:
        if (not isinstance(m, dict) or m.get("role") not in ("system", "user", "assistant")
                or not isinstance(m.get("content"), str)):
            return "every message needs role system|user|assistant and string content"
    if msgs[-1]["role"] != "assistant":
        return "the last message must be from the assistant"
    if not any(m["role"] == "user" for m in msgs[:-1]):
        return "no user message before the answer"
    return None


def parse_rows(path, fmt, expect_rows, split):
    rows = []
    with open(path, "r", encoding="utf-8") as f:
        for n, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            try:
                obj = json.loads(line)
            except ValueError as e:
                raise StageError("data_mismatch", "%s line %d: invalid JSON (%s)" % (split, n, e))
            problem = row_problem(obj, fmt)
            if problem:
                raise StageError("data_mismatch", "%s line %d: %s" % (split, n, problem))
            obj["_line"] = n
            rows.append(obj)
    if isinstance(expect_rows, int) and len(rows) != expect_rows:
        raise StageError("data_mismatch", "%s has %d rows, the marker says %d" % (split, len(rows), expect_rows))
    return rows


def sample_id(row):
    rid = row.get("id")
    return rid if isinstance(rid, str) and rid else "eval-%d" % row["_line"]
```

- [ ] 6. Templating, masking (D7), batching, LoRA targets (D4), answers, stub tokenizer.

```python
def merge_system(messages):
    msgs = [{"role": m["role"], "content": m["content"]} for m in messages]
    if not msgs or msgs[0]["role"] != "system":
        return msgs
    system, rest = msgs[0]["content"], msgs[1:]
    for m in rest:
        if m["role"] == "user":
            m["content"] = system + "\n\n" + m["content"]
            return rest
    return [{"role": "user", "content": system}] + rest


def render_prompt(tok, messages):
    msgs = [{"role": m["role"], "content": m["content"]} for m in messages]
    try:
        out = tok.apply_chat_template(msgs, add_generation_prompt=True, tokenize=False)
    except TypeError:
        out = tok.apply_chat_template(msgs, add_generation_prompt=True)
    if isinstance(out, str):
        return out
    ids = out.get("input_ids") if hasattr(out, "get") else out
    if ids and isinstance(ids[0], list):
        ids = ids[0]
    return tok.decode(ids, skip_special_tokens=False)


def encode(tok, text):
    return list(tok.encode(text, add_special_tokens=False))


def drop_double_bos(tok, ids):
    bos = getattr(tok, "bos_token_id", None)
    if bos is not None and len(ids) >= 2 and ids[0] == bos and ids[1] == bos:
        return ids[1:]
    return ids


def chat_prompt_ids(tok, messages, system_role):
    msgs = merge_system(messages) if system_role == "merge_into_first_user" else messages
    return drop_double_bos(tok, encode(tok, render_prompt(tok, msgs)))


def truncate_keep_first(seq, max_len):
    return seq if len(seq) <= max_len else seq[:1] + seq[len(seq) - (max_len - 1):]


def build_example(tok, row, fmt, system_role, stop, max_len):
    """(input_ids, labels, truncated): loss on the last answer plus stop, or on all text tokens."""
    if fmt == "text":
        ids = encode(tok, row["text"])
        if tok.bos_token_id is not None:
            ids = [tok.bos_token_id] + ids
        if tok.eos_token_id is not None:
            ids = ids + [tok.eos_token_id]
        labels = list(ids)
    else:
        msgs = row["messages"]
        prompt = chat_prompt_ids(tok, msgs[:-1], system_role)
        answer = encode(tok, msgs[-1]["content"] + stop)
        ids, labels = prompt + answer, [-100] * len(prompt) + answer
    truncated = len(ids) > max_len
    return truncate_keep_first(ids, max_len), truncate_keep_first(labels, max_len), truncated


def split_text_prompt(text):
    mid = len(text) // 2
    cut = text.rfind(" ", 0, mid)
    if cut <= 0:
        cut = mid
    return text[:cut], text[cut:]


def plan_batches(lengths, size, seed, shuffle=True):
    """Length buckets: indices sorted by length, cut into batches, batch order shuffled."""
    order = sorted(range(len(lengths)), key=lambda i: (lengths[i], i))
    batches = [order[i:i + size] for i in range(0, len(order), size)]
    if shuffle:
        random.Random(seed).shuffle(batches)
    return batches


def pad_batch(examples, idxs, pad_id, multiple=8):
    """Right padding to the longest row (rounded up to multiple): (ids, mask, labels)."""
    width = max(len(examples[i][0]) for i in idxs)
    width = ((width + multiple - 1) // multiple) * multiple
    ids, mask, labels = [], [], []
    for i in idxs:
        x, y = examples[i][0], examples[i][1]
        pad = width - len(x)
        ids.append(x + [pad_id] * pad)
        mask.append([1] * len(x) + [0] * pad)
        labels.append(y + [-100] * pad)
    return ids, mask, labels


def label_targets(examples, idxs):
    return sum(sum(1 for y in examples[i][1][1:] if y != -100) for i in idxs)


def steps_per_epoch(n_batches, grad_accum):
    return int(math.ceil(n_batches / float(grad_accum)))


def resolve_targets(wanted, leaves):
    """(resolved, missing, fallback): profile names present in the model, else every text leaf."""
    resolved = [n for n in wanted if n in leaves]
    missing = [n for n in wanted if n not in leaves]
    if resolved:
        return resolved, missing, False
    return sorted(leaves), missing, True


def lora_regex(names):
    alt = "|".join(re.escape(n) for n in sorted(set(names)))
    bad = "|".join(re.escape(m) for m in NON_TEXT_MARKERS)
    return r"^(?!.*(?:%s))(?:.*\.)?(?:%s)$" % (bad, alt)


def stop_token_ids(tok, stop_strings, extra):
    ids = set(x for x in extra if isinstance(x, int))
    if getattr(tok, "eos_token_id", None) is not None:
        ids.add(int(tok.eos_token_id))
    for s in stop_strings:
        enc = encode(tok, s)
        if len(enc) == 1:
            ids.add(enc[0])
    return sorted(ids)


def decode_answer(tok, new_ids, stop_ids, stop_strings):
    stopped = False
    for i, t in enumerate(new_ids):
        if t in stop_ids:
            new_ids, stopped = new_ids[:i], True
            break
    text = tok.decode(new_ids, skip_special_tokens=True)
    for s in stop_strings:
        if s and s in text:
            text, stopped = text[:text.index(s)], True
    return text.strip(), stopped


class StubTokenizer:
    """Character-level tokenizer with a ChatML-like template (selftest and dry run)."""
    SPECIAL = ("<pad>", "<bos>", "<eos>", "<|im_start|>", "<|im_end|>")

    def __init__(self):
        self.vocab = {s: i for i, s in enumerate(self.SPECIAL)}
        self.pad_token_id, self.bos_token_id, self.eos_token_id = 0, 1, 2
        self.pad_token, self.bos_token, self.eos_token = "<pad>", "<bos>", "<eos>"
        self.chat_template = "stub"

    def __len__(self):
        return len(self.vocab)

    def encode(self, text, add_special_tokens=False):
        ids, i = [], 0
        while i < len(text):
            for s in self.SPECIAL:
                if text.startswith(s, i):
                    ids.append(self.vocab[s])
                    i += len(s)
                    break
            else:
                ids.append(self.vocab.setdefault(text[i], len(self.vocab)))
                i += 1
        return ([self.bos_token_id] + ids) if add_special_tokens else ids

    def decode(self, ids, skip_special_tokens=False):
        inv = {v: k for k, v in self.vocab.items()}
        return "".join(inv.get(i, "?") for i in ids if not (skip_special_tokens and i < len(self.SPECIAL)))

    def apply_chat_template(self, messages, add_generation_prompt=False, tokenize=False):
        text = self.bos_token
        for m in messages:
            text += "<|im_start|>" + m["role"] + "\n" + m["content"] + "<|im_end|>\n"
        if add_generation_prompt:
            text += "<|im_start|>assistant\n"
        return self.encode(text) if tokenize else text
```

- [ ] 7. File formats: GGUF, safetensors, manifest, requirements.

```python
GGUF_SCALAR = {0: "<B", 1: "<b", 2: "<H", 3: "<h", 4: "<I", 5: "<i", 6: "<f", 7: "<?", 10: "<Q", 11: "<q", 12: "<d"}


def write_fake_gguf(path, arch):
    """Minimal valid GGUF v3: two keys (architecture, alignment), one F32 tensor of 4 values."""
    def s(b):
        return struct.pack("<Q", len(b)) + b
    kv = s(b"general.architecture") + struct.pack("<I", 8) + s(arch.encode("utf-8"))
    kv += s(b"general.alignment") + struct.pack("<I", 4) + struct.pack("<I", 32)
    info = s(b"chibi.fake") + struct.pack("<I", 1) + struct.pack("<Q", 4) + struct.pack("<I", 0) + struct.pack("<Q", 0)
    head = b"GGUF" + struct.pack("<I", 3) + struct.pack("<Q", 1) + struct.pack("<Q", 2) + kv + info
    with open(path, "wb") as f:
        f.write(head + b"\x00" * ((-len(head)) % 32) + struct.pack("<4f", 0.0, 0.0, 0.0, 0.0))


def read_gguf_header(path):
    """Version, tensor count, key count and general.architecture, with the standard library only."""
    with open(path, "rb") as f:
        def take(n):
            b = f.read(n)
            if len(b) != n:
                raise ValueError("truncated GGUF")
            return b

        def u32():
            return struct.unpack("<I", take(4))[0]

        def u64():
            return struct.unpack("<Q", take(8))[0]

        def string():
            n = u64()
            if n > (1 << 24):
                raise ValueError("GGUF string too long")
            return take(n)

        def skip(vtype):
            if vtype == 8:
                string()
            elif vtype == 9:
                etype, count = u32(), u64()
                if etype in GGUF_SCALAR:
                    f.seek(struct.calcsize(GGUF_SCALAR[etype]) * count, 1)
                else:
                    for _ in range(count):
                        skip(etype)
            elif vtype in GGUF_SCALAR:
                take(struct.calcsize(GGUF_SCALAR[vtype]))
            else:
                raise ValueError("unknown GGUF value type %d" % vtype)

        if take(4) != b"GGUF":
            raise ValueError("not a GGUF file")
        version, tensors, kvs = u32(), u64(), u64()
        arch = None
        for _ in range(kvs):
            key, vtype = string().decode("utf-8", "replace"), u32()
            if key == "general.architecture" and vtype == 8:
                arch = string().decode("utf-8", "replace")
                break
            skip(vtype)
    return {"version": version, "tensors": tensors, "kv": kvs, "arch": arch}


def write_safetensors_empty(path, metadata):
    header = json.dumps({"__metadata__": metadata}, separators=(",", ":")).encode("utf-8")
    header += b" " * ((-len(header)) % 8)
    with open(path, "wb") as f:
        f.write(struct.pack("<Q", len(header)) + header)


def safetensors_keys(path):
    with open(path, "rb") as f:
        n = struct.unpack("<Q", f.read(8))[0]
        if n > (100 << 20):
            raise ValueError("safetensors header too large")
        header = json.loads(f.read(n).decode("utf-8"))
    return [k for k in header if k != "__metadata__"]


def model_dir_keys(directory):
    keys = []
    for name in sorted(os.listdir(directory)):
        if name.endswith(".safetensors"):
            keys += safetensors_keys(os.path.join(directory, name))
    return keys


def write_manifest(out_dir, outtype, run_id, profile_id, profile_sha):
    names = output_names(outtype)
    files = []
    for kind in MANIFEST_KINDS:
        path = os.path.join(out_dir, names[kind])
        if os.path.isfile(path):
            files.append({"path": names[kind], "kind": kind, "bytes": os.path.getsize(path),
                          "sha256": sha256_file(path)})
    write_json_atomic(os.path.join(out_dir, MANIFEST_NAME), {"v": 1, "run_id": run_id, "profile_id": profile_id,
                                                             "profile_sha": profile_sha, "files": files})
    return files


def flatten_requirements(path, depth=0):
    lines = []
    with open(path, "r", encoding="utf-8") as f:
        for raw in f:
            line = raw.split(" #", 1)[0].strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("-r ") or line.startswith("--requirement"):
                include = line.split(None, 1)[1].strip()
                if depth < 3:
                    lines += flatten_requirements(os.path.join(os.path.dirname(path), include), depth + 1)
                continue
            lines.append(line)
    return lines


def adapt_requirements(lines, version):
    """Drop gguf (local gguf-py is installed) and relax numpy~=1.26 on Python 3.13+, which has no wheel."""
    out, relaxed = [], []
    for line in lines:
        name = re.split(r"[<>=~!;\s\[]", line, 1)[0].lower()
        if name == "gguf":
            relaxed.append(line + " (local gguf-py)")
            continue
        if name == "numpy" and tuple(version[:2]) >= (3, 13):
            out.append("numpy>=2.1,<3")
            relaxed.append(line)
            continue
        out.append(line)
    return out, relaxed
```

- [ ] 8. Run context and process helpers.

```python
def script_sha():
    try:
        return sha256_file(os.path.abspath(__file__))
    except OSError:
        return ""


class Ctx:
    """Shared state of one run; every stage updates status, finalize writes it."""

    def __init__(self, args, log, fake):
        self.args, self.log, self.fake = args, log, fake
        self.t0 = time.monotonic()
        self.out = os.path.abspath(args.out)
        self.input = os.path.abspath(args.input)
        self.work = tempfile.mkdtemp(prefix="chibi-dry-") if fake else WORK_ROOT
        self.job, self.profile, self.outtype = None, {}, "q8_0"
        self.budget = Budget(0, 0, t0=self.t0)
        self.status = {
            "v": 1, "run_id": "", "state": "running", "stage": "boot", "code": "", "message": "",
            "warning_code": "", "started": utc_now(), "finished": "",
            "gpu": {"name": "", "capability": "", "vram_gb": None},
            "versions": {"python": "%d.%d.%d" % sys.version_info[:3], "torch": "", "transformers": "",
                         "peft": "", "llama_cpp": ""},
            "train": {"steps": 0, "epochs": 0, "train_loss": None, "eval_loss_base": None,
                      "eval_loss_tuned": None, "stop_reason": "", "truncated_rows": 0},
            "discovery": {"script_sha256": script_sha(), "fake_env": fake, "stage_seconds": {},
                          "warnings": [], "gguf": {}}}
        self.discovery = self.status["discovery"]
        self.torch, self.device, self.model, self.tok, self.loss = None, "cuda:0", None, None, None
        self.rows, self.examples, self.samples = {}, {}, []
        self.snapshot_dir, self.llama_root, self.conv, self.pinned, self.help_cache = None, None, None, None, {}
        self.linear_leaves, self.pad_id, self.stop_ids, self.stop_strings = {}, 0, [], []
        self.base_eval_s, self.gen_manual = 0.0, False
        self.merged_dir = os.path.join(self.work, "merged")
        os.makedirs(self.out, exist_ok=True)
        os.makedirs(self.work, exist_ok=True)
        for name in list(output_names(self.outtype).values()) + [MANIFEST_NAME]:
            if os.path.isfile(os.path.join(self.out, name)):
                os.remove(os.path.join(self.out, name))
        log.metrics_path = self.path("metrics")
        open(log.metrics_path, "w").close()

    def configure(self, job):
        self.job, self.profile = job, job["profile"]
        self.outtype = job["outputs"]["gguf_outtype"]
        self.budget = Budget(job["budget"]["session_timeout_s"], job["budget"]["max_train_s"], t0=self.t0)
        self.status["run_id"] = job["run_id"]
        self.status["versions"]["llama_cpp"] = job["deps"]["llama_cpp"]["tag"]
        self.stop_strings = [s for s in (jget(self.profile, "chat.stop") or []) if s]
        for kind in ("gguf_tuned", "gguf_base"):
            if os.path.isfile(self.path(kind)):
                os.remove(self.path(kind))

    def path(self, kind):
        return os.path.join(self.out, output_names(self.outtype)[kind])

    def warn(self, code, message, primary=False, **fields):
        self.log.emit("warn", code=code, message=one_line(message), **fields)
        self.discovery["warnings"].append(code)
        if primary and not self.status["warning_code"]:
            self.status["warning_code"] = code


def run_cmd(cmd, cwd=None, env_extra=None, timeout=600):
    """(returncode, combined output, seconds); a timeout gives -9, a start failure -1."""
    env = dict(os.environ)
    env.update(env_extra or {})
    t0 = time.monotonic()
    try:
        p = subprocess.run(cmd, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout)
        rc, out = p.returncode, p.stdout
    except subprocess.TimeoutExpired as e:
        rc, out = -9, (e.stdout or b"") + (b"\n[timeout after %d s]" % timeout)
    except OSError as e:
        rc, out = -1, str(e).encode("utf-8", "replace")
    text = out.decode("utf-8", "replace") if isinstance(out, bytes) else str(out)
    return rc, text, time.monotonic() - t0


def torch_constraint_file(ctx):
    version = dist_version("torch")
    if not version:
        return None
    path = os.path.join(ctx.work, "constraints.txt")
    with open(path, "w", encoding="utf-8") as f:
        f.write("torch==%s\n" % version)
    return path


def pip_install(ctx, specs, required=True, timeout=900):
    cmd = [sys.executable, "-m", "pip", "install", "-q", "--disable-pip-version-check"]
    constraints = torch_constraint_file(ctx)
    if constraints:
        cmd += ["-c", constraints]
    rc, out, secs = run_cmd(cmd + list(specs), timeout=timeout)
    ctx.log.info("pip", specs=list(specs), rc=rc, seconds=rnd(secs, 1))
    if rc == 0:
        return True
    ctx.log.raw("pip install " + " ".join(specs), tail_lines(out))
    message = "pip install %s failed: %s" % (" ".join(specs), last_line(out))
    if required:
        raise StageError("install_failed", message)
    ctx.warn("pip_failed", message)
    return False


def http_get(url, timeout, limit=1 << 20):
    req = urllib.request.Request(url, headers={"User-Agent": "aurago-chibi-trainer", "Accept": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read(limit)
    except urllib.error.HTTPError as e:
        return e.code, b""


def download(url, path, timeout):
    req = urllib.request.Request(url, headers={"User-Agent": "aurago-chibi-trainer"})
    with urllib.request.urlopen(req, timeout=timeout) as r, open(path, "wb") as f:
        shutil.copyfileobj(r, f, 1 << 20)


def vram_gb(torch):
    try:
        return round(torch.cuda.max_memory_allocated() / 1e9, 2)
    except Exception:
        return None


def note_vram(ctx):
    if ctx.torch is not None:
        peak = vram_gb(ctx.torch)
        if peak is not None:
            ctx.discovery["peak_vram_gb"] = max(peak, ctx.discovery.get("peak_vram_gb") or 0.0)


def release(ctx):
    gc.collect()
    if ctx.torch is not None:
        try:
            ctx.torch.cuda.empty_cache()
        except Exception:
            pass


def set_use_cache(model, value):
    try:
        model.config.use_cache = value
    except Exception:
        pass
```

- [ ] 9. Stages `boot`, `env_check`, `data`, `install`.

```python
def stage_boot(ctx):
    try:
        job = read_json(ctx.args.job)
    except (OSError, ValueError) as e:
        raise StageError("job_invalid", "job.json cannot be read: %s" % exc_summary(e))
    problems = validate_job(job)
    if problems:
        raise StageError("job_invalid", "; ".join(problems[:6]))
    ctx.configure(job)
    h = job["hyper"]
    ctx.log.info("job", run_id=job["run_id"], profile=ctx.profile["id"], method=job["method"], recipe=job["recipe"],
                 format=job["data"]["format"], epochs=h["epochs"], lr=h["learning_rate"],
                 per_device_batch=h["per_device_batch"], grad_accum=h["grad_accum"], max_seq_len=h["max_seq_len"],
                 gradient_checkpointing=bool(h.get("gradient_checkpointing")), budget=job["budget"],
                 emit_base_gguf=job["outputs"]["emit_base_gguf"], script_sha256=ctx.discovery["script_sha256"],
                 dry_run=ctx.fake)


def parse_capability(text):
    parts = (str(text).split(".") + ["0"])[:2]
    return int(parts[0]), int(parts[1])


def nvidia_smi():
    exe = shutil.which("nvidia-smi")
    if not exe:
        return []
    rc, out, _ = run_cmd([exe, "--query-gpu=name,compute_cap,memory.total,driver_version",
                          "--format=csv,noheader"], timeout=20)
    if rc != 0:
        return [{"error": last_line(out)}]
    keys = ("name", "compute_cap", "memory", "driver")
    return [dict(zip(keys, [x.strip() for x in line.split(",")])) for line in out.splitlines() if line.strip()]


def stage_env_check(ctx):
    if sys.version_info < (3, 10):
        raise StageError("env_unsupported_python", "Python %s is too old; 3.10 or newer is needed"
                         % ctx.status["versions"]["python"])
    if ctx.fake:
        ctx.status["gpu"].update(name="dry-run T4", capability="7.5", vram_gb=15.8)
        return
    smi = nvidia_smi()
    ctx.discovery["nvidia_smi"] = smi
    import torch
    ctx.torch = torch
    if not torch.cuda.is_available():
        raise StageError("env_no_gpu", "CUDA is not available (nvidia-smi: %s); enable the GPU accelerator"
                         % (json.dumps(smi)[:300] or "missing"))
    props = torch.cuda.get_device_properties(0)
    gpu = {"name": props.name, "capability": "%d.%d" % (props.major, props.minor),
           "vram_gb": round(props.total_memory / 1e9, 2)}
    ctx.status["gpu"].update(gpu)
    ctx.log.info("gpu", vram_gib=round(props.total_memory / float(1 << 30), 2),
                 visible=torch.cuda.device_count(), host_gpus=len(smi), torch_cuda=torch.version.cuda, **gpu)
    training = ctx.profile["training"]
    if (props.major, props.minor) < parse_capability(training["min_gpu_capability"]):
        raise StageError("env_unsupported_gpu", "GPU %s has compute capability %s; this profile needs %s or newer. "
                         "Choose the machine shape NvidiaTeslaT4." % (props.name, gpu["capability"],
                                                                      training["min_gpu_capability"]))
    if gpu["vram_gb"] < float(training["min_vram_gb"]):
        raise StageError("env_unsupported_gpu", "GPU %s has %.1f GB VRAM; this profile needs %.1f GB"
                         % (props.name, gpu["vram_gb"], float(training["min_vram_gb"])))
    need_work = 3 * float(ctx.profile["arch"]["params_b"]) * 4e9
    est = float(jget(ctx.profile, "gguf.estimated_mb") or 0) * 1e6
    need_out = 1.2 * est * (2 if ctx.job["outputs"]["emit_base_gguf"] else 1)
    free_work, free_out = shutil.disk_usage(ctx.work).free, shutil.disk_usage(ctx.out).free
    ctx.log.info("disk", work_free_gb=rnd(free_work / 1e9, 1), out_free_gb=rnd(free_out / 1e9, 1),
                 work_need_gb=rnd(need_work / 1e9, 1), out_need_gb=rnd(need_out / 1e9, 1))
    if free_work < need_work or free_out < need_out:
        raise StageError("env_low_disk", "free disk %.1f GB (work) and %.1f GB (output) is below the needed "
                         "%.1f GB and %.1f GB" % (free_work / 1e9, free_out / 1e9, need_work / 1e9, need_out / 1e9))
    check_network(ctx)
    check_base_repo(ctx)


def check_network(ctx):
    for url in ("https://pypi.org/simple/pip/", "https://huggingface.co/api/models/" + ctx.profile["base"]["repo"]):
        host = urllib.parse.urlparse(url).netloc
        t0 = time.monotonic()
        try:
            status, _ = http_get(url, 10, 4096)
        except (urllib.error.URLError, OSError, ValueError) as e:
            raise StageError("env_no_internet", "%s is not reachable (%s). Turn on Internet in the notebook "
                             "settings; Kaggle needs a phone-verified account for it." % (host, exc_summary(e)))
        ctx.log.info("network", host=host, http=status, ms=int((time.monotonic() - t0) * 1000))


def check_base_repo(ctx):
    repo, rev = ctx.profile["base"]["repo"], ctx.profile["base"]["revision"]
    url = "https://huggingface.co/api/models/%s/revision/%s" % (repo, urllib.parse.quote(rev, safe=""))
    try:
        status, body = http_get(url, 15)
    except (urllib.error.URLError, OSError, ValueError) as e:
        ctx.warn("hf_api_unreachable", "model info for %s could not be read: %s" % (repo, exc_summary(e)))
        return
    if status in (401, 403, 404):
        raise StageError("base_download_failed", "%s@%s is not available anonymously (HTTP %d)" % (repo, rev, status))
    info = {"http": status}
    try:
        data = json.loads(body.decode("utf-8"))
        info.update(sha=data.get("sha"), gated=data.get("gated"), private=data.get("private"),
                    last_modified=data.get("lastModified"))
    except ValueError:
        info["parse_error"] = True
    ctx.discovery["hf_repo"] = info
    ctx.log.info("hf_repo", repo=repo, revision=rev, **info)
    if info.get("gated") not in (None, False):
        raise StageError("base_download_failed", "%s is gated now (%s); profiles need an ungated base"
                         % (repo, info.get("gated")))
    if not re.fullmatch(r"[0-9a-f]{40}", rev):
        ctx.warn("unpinned_revision", "base.revision %s is not a commit; it resolved to %s" % (rev, info.get("sha")))


def stage_data(ctx):
    try:
        marker, base = load_marker(ctx.input, ctx.job["data"]["marker"], ctx.job["run_id"])
    except StageError:
        ctx.discovery["input_tree"] = list_tree(ctx.input)
        raise
    files = verify_split_files(marker, base, ctx.input)
    for split, (path, entry) in files.items():
        ctx.rows[split] = parse_rows(path, ctx.job["data"]["format"], entry.get("rows"), split)
    counts = {k: len(v) for k, v in ctx.rows.items()}
    ctx.discovery["data"] = counts
    ctx.log.info("data", marker_dir=os.path.relpath(base, ctx.input), **counts)


def stage_install(ctx):
    lora = ctx.job["method"] == "lora"
    if ctx.fake:
        ctx.status["versions"].update(torch="dry-run", transformers="dry-run", peft="dry-run" if lora else "")
        return
    torch_before = dist_version("torch")
    pins = list(jget(ctx.job, "deps.pip") or []) + list(jget(ctx.profile, "training.extra_pip") or [])
    if pins:
        pip_install(ctx, pins)
    else:
        ctx.warn("unpinned_dependencies", "the lock names no pip pins; missing packages are installed unpinned")
    want = jget(ctx.job, "deps.transformers") or ""
    if want and want != dist_version("transformers"):
        pip_install(ctx, ["transformers==" + want])
    ensure = [("huggingface_hub", "huggingface_hub", True), ("safetensors", "safetensors", True),
              ("sentencepiece", "sentencepiece", False)]
    if lora:
        ensure.insert(0, ("peft", "peft", True))
    for module, package, required in ensure:
        importlib.invalidate_caches()
        if importlib.util.find_spec(module) is None and pip_install(ctx, [package], required=required):
            ctx.warn("unpinned_dependency", "%s was missing and is now installed unpinned (%s)"
                     % (package, dist_version(package)))
    importlib.invalidate_caches()
    for module in ["torch", "transformers", "huggingface_hub", "safetensors"] + (["peft"] if lora else []):
        try:
            importlib.import_module(module)
        except Exception as e:
            raise StageError("install_failed", "import %s failed: %s" % (module, exc_summary(e)))
    import torch
    ctx.torch = torch
    versions = ctx.status["versions"]
    versions.update(torch=torch.__version__, transformers=dist_version("transformers"),
                    peft=dist_version("peft") if lora else "")
    extra = {"huggingface_hub": dist_version("huggingface_hub"), "safetensors": dist_version("safetensors"),
             "sentencepiece": dist_version("sentencepiece"), "numpy": dist_version("numpy"),
             "cuda": torch.version.cuda, "cudnn": torch.backends.cudnn.version()}
    ctx.discovery["versions_extra"] = extra
    ctx.log.info("versions", **versions, **extra)
    if dist_version("torch") != torch_before:
        ctx.warn("torch_changed", "torch changed from %s to %s during install" % (torch_before, dist_version("torch")))
```

- [ ] 10. Stage `load_base` and tokenization.

```python
def download_base(ctx):
    from huggingface_hub import snapshot_download
    repo, rev = ctx.profile["base"]["repo"], ctx.profile["base"]["revision"]
    dest = os.path.join(ctx.work, "base")
    last = None
    for attempt in range(1, 4):
        t0 = time.monotonic()
        try:
            try:
                path = snapshot_download(repo_id=repo, revision=rev, local_dir=dest, token=False,
                                         ignore_patterns=SNAPSHOT_IGNORE)
            except TypeError:
                path = snapshot_download(repo_id=repo, revision=rev, local_dir=dest, ignore_patterns=SNAPSHOT_IGNORE)
            names = sorted(os.listdir(path))
            ctx.log.info("base_snapshot", repo=repo, revision=rev, files=names[:40], bytes=dir_bytes(path),
                         seconds=rnd(time.monotonic() - t0, 1))
            return path
        except Exception as e:
            last = e
            ctx.warn("base_download_retry", "attempt %d: %s" % (attempt, exc_summary(e)))
            time.sleep(10 * attempt)
    raise StageError("base_download_failed", "%s@%s: %s" % (repo, rev, exc_summary(last)))


def summarize_config(path):
    cfg = read_json(os.path.join(path, "config.json"))
    text = cfg.get("text_config") if isinstance(cfg.get("text_config"), dict) else {}
    return {"model_type": cfg.get("model_type"), "architectures": cfg.get("architectures"),
            "dtype": cfg.get("dtype", cfg.get("torch_dtype")), "text_model_type": text.get("model_type"),
            "has_vision_config": "vision_config" in cfg,
            "tie_word_embeddings": cfg.get("tie_word_embeddings", text.get("tie_word_embeddings")),
            "vocab_size": cfg.get("vocab_size", text.get("vocab_size")),
            "final_logit_softcapping": cfg.get("final_logit_softcapping", text.get("final_logit_softcapping"))}


def load_tokenizer(ctx, path):
    import transformers
    try:
        tok, how = transformers.AutoTokenizer.from_pretrained(path), "AutoTokenizer"
    except Exception as e:
        ctx.warn("tokenizer_fallback", "AutoTokenizer failed: %s" % exc_summary(e))
        tok, how = transformers.AutoProcessor.from_pretrained(path).tokenizer, "AutoProcessor"
    ctx.discovery["tokenizer"] = {"loader": how, "class": type(tok).__name__}
    return tok


def load_model_chain(ctx, path):
    """D5: try base.auto_class, then base.auto_class_fallbacks; fp32 via dtype, else torch_dtype."""
    import torch
    import transformers
    names = []
    for name in [ctx.profile["base"]["auto_class"]] + list(jget(ctx.profile, "base.auto_class_fallbacks") or []):
        if name not in names:
            names.append(name)
    errors = []
    for name in names:
        cls = getattr(transformers, name, None)
        if cls is None:
            errors.append("%s: not in transformers %s" % (name, transformers.__version__))
            continue
        t0 = time.monotonic()
        try:
            try:
                model = cls.from_pretrained(path, dtype=torch.float32)
            except TypeError as e:
                if "dtype" not in str(e):
                    raise
                model = cls.from_pretrained(path, torch_dtype=torch.float32)
        except Exception as e:
            if is_oom(e):
                raise
            errors.append("%s: %s" % (name, exc_summary(e)[:300]))
            ctx.warn("base_class_failed", errors[-1])
            gc.collect()
            continue
        ctx.log.info("base_loaded", cls=name, model_class=type(model).__name__, seconds=rnd(time.monotonic() - t0, 1))
        return model, name, errors
    raise StageError("base_load_failed", "no class could load the base: " + " | ".join(errors))


def dtype_counts(model):
    out = {}
    for p in model.parameters():
        key = str(p.dtype).replace("torch.", "")
        out[key] = out.get(key, 0) + p.numel()
    return out


def linear_leaves(model, nn):
    """Histogram of nn.Linear leaf names of the text model (no LM head, no vision or audio tower)."""
    counts, examples = {}, {}
    for name, module in model.named_modules():
        if not isinstance(module, nn.Linear):
            continue
        low = name.lower()
        if "lm_head" in low or any(m in low for m in NON_TEXT_MARKERS):
            continue
        leaf = name.rsplit(".", 1)[-1]
        counts[leaf] = counts.get(leaf, 0) + 1
        examples.setdefault(leaf, name)
    return counts, examples


def lora_wanted(ctx):
    return ((ctx.job["hyper"].get("lora") or {}).get("target_modules")
            or jget(ctx.profile, "training.lora.target_modules") or [])


def finish_tokenizer(ctx, model):
    tok = ctx.tok
    eos = tok.eos_token_id
    ctx.pad_id = tok.pad_token_id if tok.pad_token_id is not None else (eos if eos is not None else 0)
    extra = getattr(getattr(model, "generation_config", None), "eos_token_id", None) if model is not None else None
    extra = [extra] if isinstance(extra, int) else list(extra or [])
    ctx.stop_ids = stop_token_ids(tok, ctx.stop_strings, extra)
    info = {"bos": [str(tok.bos_token), tok.bos_token_id], "eos": [str(tok.eos_token), eos], "pad_id": ctx.pad_id,
            "stop_ids": ctx.stop_ids, "vocab": len(tok), "chat_template": bool(getattr(tok, "chat_template", None))}
    ctx.discovery.setdefault("tokenizer", {}).update(info)
    ctx.log.info("tokenizer", **ctx.discovery["tokenizer"])


def template_check(ctx):
    ex = ctx.examples.get("train") or []
    if not ex:
        return
    tok, (ids, labels, _) = ctx.tok, ex[0]
    first = next((i for i, y in enumerate(labels) if y != -100), 0)
    bos = getattr(tok, "bos_token_id", None)
    info = {"prompt_tokens": first, "answer_tokens": len(ids) - first,
            "starts_with_bos": bos is not None and ids[0] == bos, "bos_count": ids.count(bos) if bos is not None else 0,
            "prompt_tail": tok.decode(ids[max(0, first - 24):first]), "answer_head": tok.decode(ids[first:first + 12]),
            "sequence_end": tok.decode(ids[-4:])}
    ctx.discovery["template_check"] = info
    ctx.log.info("template_check", **info)


def make_samples(ctx, fmt, role, max_len):
    tok, cap = ctx.tok, ctx.job["budget"]["eval_max_samples"]
    max_prompt = max(8, max_len - ctx.job["budget"]["max_new_tokens"])
    out = []
    for row in (ctx.rows.get("eval") or [])[:cap]:
        s = {"id": sample_id(row)}
        if fmt == "text":
            prompt, expected = split_text_prompt(row["text"])
            s["prompt_text"] = prompt
            ids = ([tok.bos_token_id] if tok.bos_token_id is not None else []) + encode(tok, prompt)
        else:
            msgs = row["messages"]
            s["prompt_messages"] = [{"role": m["role"], "content": m["content"]} for m in msgs[:-1]]
            expected = msgs[-1]["content"]
            ids = chat_prompt_ids(tok, msgs[:-1], role)
        s.update(expected=expected, base_output=None, tuned_output=None, base_ms=None, tuned_ms=None, truncated=False)
        s["_prompt_ids"] = truncate_keep_first(ids, max_prompt)
        out.append(s)
    return out


def write_samples(ctx):
    write_jsonl_atomic(ctx.path("eval_samples"),
                       [{k: v for k, v in s.items() if not k.startswith("_")} for s in ctx.samples])


def prepare_examples(ctx):
    tok, job = ctx.tok, ctx.job
    fmt, role = job["data"]["format"], ctx.profile["chat"]["system_role"]
    stop = ctx.stop_strings[0] if ctx.stop_strings else (getattr(tok, "eos_token", None) or "")
    max_len = job["hyper"]["max_seq_len"]
    stats = {}
    for split, rows in ctx.rows.items():
        ex = []
        for row in rows:
            ids, labels, cut = build_example(tok, row, fmt, role, stop, max_len)
            if not any(y != -100 for y in labels[1:]):
                raise StageError("data_mismatch", "%s line %d has no answer tokens after templating"
                                 % (split, row["_line"]))
            ex.append((ids, labels, cut))
        ctx.examples[split] = ex
        lens = sorted(len(e[0]) for e in ex)
        stats[split] = {"rows": len(ex), "tokens": sum(lens), "max": lens[-1] if lens else 0,
                        "p95": lens[int(0.95 * (len(lens) - 1))] if lens else 0,
                        "truncated": sum(1 for e in ex if e[2]), "targets": label_targets(ex, range(len(ex)))}
    truncated = stats.get("train", {}).get("truncated", 0)
    ctx.status["train"]["truncated_rows"] = truncated
    if truncated:
        ctx.warn("rows_truncated", "%d train rows are longer than max_seq_len %d" % (truncated, max_len))
    ctx.discovery["tokens"] = stats
    ctx.log.info("tokens", **stats)
    template_check(ctx)
    ctx.samples = make_samples(ctx, fmt, role, max_len)


def stage_load_base(ctx):
    if ctx.fake:
        ctx.tok = StubTokenizer()
        ctx.linear_leaves = {name: 4 for name in (lora_wanted(ctx) or ["q_proj"])}
        ctx.discovery.update(base_class=ctx.profile["base"]["auto_class"], linear_leaves=ctx.linear_leaves)
        finish_tokenizer(ctx, None)
        prepare_examples(ctx)
        return
    import torch
    ctx.torch = torch
    path = download_base(ctx)
    ctx.snapshot_dir = path
    ctx.discovery["base_config"] = summarize_config(path)
    ctx.log.info("base_config", **ctx.discovery["base_config"])
    ctx.tok = load_tokenizer(ctx, path)
    model, name, errors = load_model_chain(ctx, path)
    loaded = dtype_counts(model)
    model = model.to(dtype=torch.float32)
    model.to(ctx.device)
    set_use_cache(model, True)
    ctx.model = model
    leaves, examples = linear_leaves(model, torch.nn)
    ctx.linear_leaves = leaves
    non_text = any(any(m in n.lower() for m in NON_TEXT_MARKERS) for n, _ in model.named_modules())
    ctx.discovery.update(base_class=name, model_class=type(model).__name__, base_class_errors=errors,
                         params=sum(p.numel() for p in model.parameters()), dtype_loaded=loaded,
                         dtype_now=dtype_counts(model), has_non_text_modules=non_text, linear_leaves=leaves,
                         linear_examples=examples, vram_after_load_gb=vram_gb(torch),
                         attn_implementation=getattr(model.config, "_attn_implementation", None))
    ctx.log.info("linear_leaves", leaves=leaves, examples=examples)
    if non_text:
        ctx.warn("non_text_modules", "class %s loaded vision or audio modules; LoRA skips them" % name)
    finish_tokenizer(ctx, model)
    prepare_examples(ctx)
```

- [ ] 11. Stage `converter` and the converter helpers (D9).

```python
def extract_tar(path, dest):
    with tarfile.open(path, "r:gz") as tf:
        members = [m for m in tf.getmembers() if (m.isfile() or m.isdir()) and not m.name.startswith("/")
                   and ".." not in m.name.replace("\\", "/").split("/")]
        try:
            tf.extractall(dest, members=members, filter="data")
        except TypeError:
            tf.extractall(dest, members=members)


def fetch_llama(ctx):
    dep = ctx.job["deps"]["llama_cpp"]
    tar = os.path.join(ctx.work, "llama.tar.gz")
    last, t0 = None, time.monotonic()
    for attempt in range(1, 4):
        try:
            download(dep["url"], tar, 120)
            break
        except (urllib.error.URLError, OSError, ValueError) as e:
            last = e
            ctx.warn("llama_download_retry", "attempt %d: %s" % (attempt, exc_summary(e)))
            time.sleep(5 * attempt)
    else:
        raise StageError("gguf_failed", "llama.cpp %s could not be downloaded: %s" % (dep["tag"], exc_summary(last)))
    got, want = sha256_file(tar), (dep.get("sha256") or "").lower()
    ctx.discovery["llama_cpp_sha256"] = got
    ctx.log.info("llama_cpp", tag=dep["tag"], sha256=got, bytes=os.path.getsize(tar), pinned=bool(want),
                 seconds=rnd(time.monotonic() - t0, 1))
    if want and got != want:
        raise StageError("gguf_failed", "llama.cpp tarball sha256 %s does not match the pin %s" % (got, want))
    if not want:
        ctx.warn("llama_cpp_unpinned", "llama.cpp %s tarball sha256 is %s; pin it in requirements.lock" % (dep["tag"], got))
    dest = os.path.join(ctx.work, "llama")
    shutil.rmtree(dest, ignore_errors=True)
    extract_tar(tar, dest)
    candidates = [dest] + [os.path.join(dest, n) for n in sorted(os.listdir(dest))]
    root = next((c for c in candidates if os.path.isfile(os.path.join(c, "convert_hf_to_gguf.py"))), None)
    if root is None or not os.path.isdir(os.path.join(root, "gguf-py")):
        raise StageError("gguf_failed", "the llama.cpp tarball has no convert_hf_to_gguf.py with gguf-py")
    ctx.llama_root = root
    ctx.discovery["llama_cpp_layout"] = {"conversion_dir": os.path.isdir(os.path.join(root, "conversion")),
                                         "requirements": os.path.isdir(os.path.join(root, "requirements"))}


def system_env():
    return {"name": "system", "python": sys.executable, "env": {}, "detail": "notebook"}


def build_pinned_env(ctx):
    """Virtual environment from the tarball's converter requirements; pip --target when venv is unavailable."""
    root = ctx.llama_root
    reqs = flatten_requirements(os.path.join(root, "requirements", "requirements-convert_hf_to_gguf.txt"))
    reqs, relaxed = adapt_requirements(reqs, sys.version_info)
    req_path = os.path.join(ctx.work, "convert-requirements.txt")
    with open(req_path, "w", encoding="utf-8") as f:
        f.write("\n".join(reqs) + "\n")
    ctx.log.info("pinned_env_requirements", lines=reqs, relaxed=relaxed)
    gguf_py = os.path.join(root, "gguf-py")
    venv = os.path.join(ctx.work, "convenv")
    rc, out, _ = run_cmd([sys.executable, "-m", "venv", venv], timeout=300)
    if rc == 0:
        py = os.path.join(venv, "Scripts" if os.name == "nt" else "bin", "python")
        rc, out, _ = run_cmd([py, "-m", "pip", "install", "-q", "-r", req_path, gguf_py], timeout=1500)
        if rc == 0:
            return {"name": "pinned", "python": py, "env": {}, "detail": "venv"}
        ctx.log.raw("pinned venv pip", tail_lines(out))
        ctx.warn("pinned_env_failed", "venv install failed: %s" % last_line(out))
    else:
        ctx.warn("venv_unavailable", "python -m venv failed: %s" % last_line(out))
    target = os.path.join(ctx.work, "convlib")
    rc, out, _ = run_cmd([sys.executable, "-m", "pip", "install", "-q", "--target", target, "-r", req_path, gguf_py],
                         timeout=1500)
    if rc == 0:
        return {"name": "pinned", "python": sys.executable, "env": {"PYTHONPATH": target}, "detail": "target"}
    ctx.log.raw("pinned target pip", tail_lines(out))
    ctx.warn("pinned_env_failed", "pip --target failed: %s" % last_line(out))
    return None


def ensure_pinned_env(ctx):
    if ctx.pinned is not None:
        return ctx.pinned or None
    if ctx.budget.remaining() < 600:
        ctx.warn("pinned_env_skipped", "too little session time to build the pinned converter environment")
        ctx.pinned = False
        return None
    t0 = time.monotonic()
    ctx.pinned = build_pinned_env(ctx) or False
    ctx.log.info("pinned_env", ok=bool(ctx.pinned), detail=(ctx.pinned or {}).get("detail"),
                 seconds=rnd(time.monotonic() - t0, 1))
    return ctx.pinned or None


def converter_help(ctx, env):
    key = env["name"]
    if key not in ctx.help_cache:
        script = os.path.join(ctx.llama_root, "convert_hf_to_gguf.py")
        rc, out, secs = run_cmd([env["python"], script, "--help"], cwd=ctx.llama_root, env_extra=env["env"], timeout=600)
        ctx.log.info("converter_help", env=key, rc=rc, seconds=rnd(secs, 1), error=None if rc == 0 else last_line(out))
        if rc != 0:
            ctx.log.raw("convert_hf_to_gguf.py --help (%s)" % key, tail_lines(out))
        ctx.help_cache[key] = out if rc == 0 else None
    return ctx.help_cache[key]


def converter_flags(help_text, model_dir):
    """--no-mtp when the converter knows it and the checkpoint carries no MTP tensors."""
    if "--no-mtp" not in help_text:
        return []
    try:
        keys = model_dir_keys(model_dir)
    except (OSError, ValueError):
        return []
    return ["--no-mtp"] if keys and not any("mtp" in k for k in keys) else []


def convert(ctx, src, dst, label, vocab_only=False):
    """Run the converter in the chosen environment; the system one falls back to the pinned one (D9)."""
    script = os.path.join(ctx.llama_root, "convert_hf_to_gguf.py")
    candidates = [ctx.conv or system_env()]
    if candidates[0]["name"] == "system":
        candidates.append(None)
    attempts = []
    for env in candidates:
        if env is None:
            env = ensure_pinned_env(ctx)
            if env is None:
                attempts.append("pinned: environment not available")
                break
        help_text = converter_help(ctx, env)
        if help_text is None:
            attempts.append("%s: the converter does not start" % env["name"])
            continue
        flags = converter_flags(help_text, src) + (["--vocab-only"] if vocab_only else [])
        timeout = max(120, min(1800, int(ctx.budget.remaining()) - 60))
        cmd = [env["python"], script, src, "--outtype", ctx.outtype, "--outfile", dst] + flags
        rc, out, secs = run_cmd(cmd, cwd=ctx.llama_root, env_extra=env["env"], timeout=timeout)
        ok = rc == 0 and os.path.isfile(dst) and os.path.getsize(dst) > 0
        ctx.log.info("converter_run", label=label, env=env["name"], flags=flags, rc=rc, ok=ok, seconds=rnd(secs, 1))
        if ok:
            ctx.conv = env
            ctx.discovery.update(converter_env=env["name"], converter_env_detail=env["detail"])
            return
        tail = tail_lines(out)
        ctx.log.raw("converter %s (%s)" % (label, env["name"]), tail)
        ctx.warn("converter_failed", "%s conversion in the %s environment failed: %s" % (label, env["name"], last_line(out)),
                 tail=tail[-12:])
        ctx.discovery.setdefault("converter_tail", {})["%s/%s" % (label, env["name"])] = "\n".join(tail)[-3000:]
        attempts.append("%s: %s" % (env["name"], last_line(out)))
        if os.path.isfile(dst):
            os.remove(dst)
    raise StageError("gguf_failed", "%s conversion failed (%s)" % (label, " | ".join(attempts)))


def gguf_summary(ctx, path):
    gguf_py = os.path.join(ctx.llama_root, "gguf-py")
    if gguf_py not in sys.path:
        sys.path.insert(0, gguf_py)
    from gguf import GGUFReader
    reader = GGUFReader(path)
    info = {"tensors": len(reader.tensors), "types": {}}
    field = reader.fields.get("general.architecture")
    if field is not None:
        info["arch"] = bytes(field.parts[field.data[0]]).decode("utf-8", "replace")
    for t in reader.tensors:
        key = getattr(t.tensor_type, "name", str(t.tensor_type))
        info["types"][key] = info["types"].get(key, 0) + 1
    del reader
    return info


def verify_gguf(ctx, path, label, need_tensors=True):
    hdr = read_gguf_header(path)
    info = {"bytes": os.path.getsize(path), "version": hdr["version"], "tensors": hdr["tensors"], "arch": hdr["arch"]}
    if not ctx.fake:
        try:
            r = gguf_summary(ctx, path)
            info.update(reader_arch=r.get("arch"), reader_tensors=r.get("tensors"), types=r.get("types"))
        except Exception as e:
            info["reader_error"] = exc_summary(e)[:300]
            ctx.warn("gguf_reader_failed", info["reader_error"])
    ctx.discovery["gguf"][label] = info
    ctx.log.info("gguf", label=label, **info)
    expected = ctx.profile["gguf"]["expected_arch"]
    arch = info.get("reader_arch") or info["arch"]
    if arch != expected:
        raise StageError("gguf_arch_mismatch", "%s GGUF architecture is %r, the profile expects %r" % (label, arch, expected))
    if need_tensors and not (info.get("reader_tensors") or info["tensors"]):
        raise StageError("gguf_failed", "%s GGUF has no tensors" % label)


def base_gguf_wanted(ctx):
    if not ctx.job["outputs"]["emit_base_gguf"]:
        return False
    spare = ctx.budget.remaining() - ctx.budget.max_train_s - SAVE_GGUF_RESERVE_S
    if spare < BASE_GGUF_MIN_S:
        ctx.warn("base_gguf_skipped", "base GGUF skipped: %d s of spare session time, %d s needed"
                 % (spare, BASE_GGUF_MIN_S), primary=True)
        return False
    return True


def stage_converter(ctx):
    emit_base = base_gguf_wanted(ctx)
    if ctx.fake:
        ctx.discovery.update(llama_cpp_sha256="dry-run", converter_env="system", converter_env_detail="dry-run")
        if emit_base:
            write_fake_gguf(ctx.path("gguf_base"), ctx.profile["gguf"]["expected_arch"])
            verify_gguf(ctx, ctx.path("gguf_base"), "base")
        return
    fetch_llama(ctx)
    pip_install(ctx, [os.path.join(ctx.llama_root, "gguf-py")], required=False)
    ctx.conv = system_env()
    if emit_base:
        convert(ctx, ctx.snapshot_dir, ctx.path("gguf_base"), "base")
        verify_gguf(ctx, ctx.path("gguf_base"), "base")
        return
    probe = os.path.join(ctx.work, "vocab-probe.gguf")
    convert(ctx, ctx.snapshot_dir, probe, "vocab_probe", vocab_only=True)
    verify_gguf(ctx, probe, "vocab_probe", need_tensors=False)
    os.remove(probe)
```

- [ ] 12. Loss without full-batch logits, eval loss, greedy answers.

```python
class LossFn:
    """Next-token cross entropy. 'chunked_head' runs the decoder and applies the LM head in checkpointed
    chunks over label positions only; 'full_logits' uses the model forward (fallback)."""

    def __init__(self, ctx, model):
        import torch
        import torch.utils.checkpoint  # noqa: F401
        self.torch, self.model = torch, model
        self.head = model.get_output_embeddings()
        try:
            self.decoder = model.get_decoder()
        except Exception:
            self.decoder = None
        cfg = getattr(model, "config", None)
        text_cfg = cfg.get_text_config() if hasattr(cfg, "get_text_config") else cfg
        self.softcap = getattr(text_cfg, "final_logit_softcapping", None)
        vocab = int(self.head.weight.shape[0]) if self.head is not None else 1
        self.chunk = max(64, min(4096, (1 << 27) // max(1, vocab)))
        self.path = "chunked_head" if self.decoder is not None and self.head is not None else "full_logits"

    def _ce(self, logits, targets):
        return self.torch.nn.functional.cross_entropy(logits, targets, reduction="sum")

    def _chunk(self, h, t):
        logits = self.head(h).float()
        if self.softcap:
            logits = self.torch.tanh(logits / self.softcap) * self.softcap
        return self._ce(logits, t)

    def sum_loss(self, ids, mask, labels, grad):
        """(sum of token losses, number of target tokens); (None, 0) without targets."""
        y = labels[:, 1:]
        keep = y != -100
        n = int(keep.sum().item())
        if n == 0:
            return None, 0
        if self.path == "chunked_head":
            out = self.decoder(input_ids=ids, attention_mask=mask, use_cache=False)
            h = out.last_hidden_state if hasattr(out, "last_hidden_state") else out[0]
            h, t = h[:, :-1][keep], y[keep]
            total = None
            for s in range(0, n, self.chunk):
                hs, ts = h[s:s + self.chunk], t[s:s + self.chunk]
                part = (self.torch.utils.checkpoint.checkpoint(self._chunk, hs, ts, use_reentrant=False)
                        if grad else self._chunk(hs, ts))
                total = part if total is None else total + part
            return total, n
        logits = self.model(input_ids=ids, attention_mask=mask, use_cache=False).logits
        return self._ce(logits[:, :-1][keep].float(), y[keep]), n


def to_tensors(ctx, examples, idxs):
    ids, mask, labels = pad_batch(examples, idxs, ctx.pad_id)
    t = ctx.torch
    return (t.tensor(ids, device=ctx.device), t.tensor(mask, device=ctx.device), t.tensor(labels, device=ctx.device))


def check_loss_path(ctx, model):
    """Compare the chunked loss with the plain model loss on the shortest train row once."""
    loss = ctx.loss
    if loss.path == "chunked_head":
        ex = ctx.examples["train"]
        i = min(range(len(ex)), key=lambda k: len(ex[k][0]))
        try:
            with ctx.torch.no_grad():
                a, n = loss.sum_loss(*to_tensors(ctx, ex, [i]), grad=False)
                loss.path = "full_logits"
                b, _ = loss.sum_loss(*to_tensors(ctx, ex, [i]), grad=False)
                loss.path = "chunked_head"
            a, b = float(a) / n, float(b) / n
            rel = abs(a - b) / max(1e-6, abs(b))
            ctx.discovery["loss_path_check"] = {"chunked": rnd(a, 6), "full": rnd(b, 6), "rel_diff": rnd(rel, 8)}
            ctx.log.info("loss_path_check", **ctx.discovery["loss_path_check"])
            if not rel <= 1e-3:
                loss.path = "full_logits"
                ctx.warn("loss_path_mismatch", "chunked loss %.6f differs from model loss %.6f; using full logits" % (a, b))
        except Exception as e:
            if is_oom(e):
                raise
            loss.path = "full_logits"
            ctx.warn("loss_path_fallback", "chunked loss failed, using full logits: %s" % exc_summary(e))
    ctx.discovery["loss_path"] = loss.path


def eval_loss(ctx, model, examples, batch_size):
    if not examples:
        return None
    model.eval()
    for size in (batch_size, 1):
        total, count, oom = 0.0, 0, False
        try:
            with ctx.torch.no_grad():
                for idxs in plan_batches([len(e[0]) for e in examples], size, 0, shuffle=False):
                    s, n = ctx.loss.sum_loss(*to_tensors(ctx, examples, idxs), grad=False)
                    if n:
                        total += float(s.item())
                        count += n
        except Exception as e:
            if not is_oom(e) or size == 1:
                raise
            oom = True
        if not oom:
            return total / count if count else None
        release(ctx)
        ctx.warn("eval_batch_reduced", "eval loss ran out of memory at batch %d; retrying with 1" % size)
    return None


def manual_greedy(ctx, model, prompt_ids, max_new, cap_s):
    ids, new, t0 = list(prompt_ids), [], time.monotonic()
    for _ in range(max_new):
        if time.monotonic() - t0 > cap_s:
            break
        x = ctx.torch.tensor([ids], device=ctx.device)
        nxt = int(ctx.torch.argmax(model(input_ids=x, use_cache=False).logits[0, -1]).item())
        new.append(nxt)
        ids.append(nxt)
        if nxt in ctx.stop_ids:
            break
    return new


def greedy_answer(ctx, model, prompt_ids, max_new, cap_s):
    """(text, ms, truncated, new tokens): greedy, stops at EOS and chat.stop, time-capped."""
    t0, new = time.monotonic(), None
    if not ctx.gen_manual:
        try:
            import transformers
            cfg = transformers.GenerationConfig(max_new_tokens=max_new, do_sample=False, eos_token_id=ctx.stop_ids or None,
                                                pad_token_id=ctx.pad_id, max_time=cap_s, use_cache=True)
            ids = ctx.torch.tensor([prompt_ids], device=ctx.device)
            out = model.generate(input_ids=ids, attention_mask=ctx.torch.ones_like(ids), generation_config=cfg)
            new = out[0, ids.shape[1]:].tolist()
        except Exception as e:
            if is_oom(e):
                raise
            ctx.gen_manual = True
            ctx.warn("generate_fallback", "model.generate failed, using a plain greedy loop: %s" % exc_summary(e))
    if new is None:
        new = manual_greedy(ctx, model, prompt_ids, max_new, cap_s - (time.monotonic() - t0))
    text, stopped = decode_answer(ctx.tok, new, ctx.stop_ids, ctx.stop_strings)
    return text, int((time.monotonic() - t0) * 1000), not stopped, len(new)


def run_eval(ctx, model, which, available_s):
    result = {"loss": None, "answers": 0, "tok_s": None}
    examples = ctx.examples.get("eval") or []
    if not examples:
        ctx.warn("no_eval_rows", "the snapshot has no eval split; %s evaluation skipped" % which)
        return result
    t0 = time.monotonic()
    result["loss"] = eval_loss(ctx, model, examples, ctx.job["hyper"]["per_device_batch"])
    loss_s = time.monotonic() - t0
    max_new = ctx.job["budget"]["max_new_tokens"]
    per_cap, total_cap = generation_plan(len(ctx.samples), max_new,
                                         ctx.profile["training"]["t4_gen_tokens_per_second"], available_s - loss_s)
    set_use_cache(model, True)
    model.eval()
    g0, tokens = time.monotonic(), 0
    with ctx.torch.no_grad():
        for i, s in enumerate(ctx.samples):
            if time.monotonic() - g0 > total_cap:
                ctx.warn("eval_samples_shortened", "%s answers stopped after %d of %d samples (cap %d s)"
                         % (which, i, len(ctx.samples), total_cap))
                break
            text, ms, truncated, n = greedy_answer(ctx, model, s["_prompt_ids"], max_new, per_cap)
            s[which + "_output"], s[which + "_ms"] = text, ms
            s["truncated"] = bool(s["truncated"] or truncated)
            tokens += n
            result["answers"] += 1
            if i == 0:
                ctx.log.info("first_answer", which=which, ms=ms, tokens=n, truncated=truncated, preview=text[:160])
    gen_s = time.monotonic() - g0
    result["tok_s"] = tokens / gen_s if gen_s > 0 and tokens else None
    ctx.log.emit("eval", which=which, loss=rnd(result["loss"]), rows=len(examples), answers=result["answers"],
                 samples=len(ctx.samples), tok_s=rnd(result["tok_s"], 1), loss_s=rnd(loss_s, 1), gen_s=rnd(gen_s, 1))
    return result
```

- [ ] 13. Stages `base_eval` and `tuned_eval`, plus the eval stub.

```python
def fake_eval(ctx, which):
    loss = 2.5 if which == "base" else 1.2
    if ctx.examples.get("eval"):
        ctx.status["train"]["eval_loss_" + which] = loss
    for s in ctx.samples:
        s[which + "_output"] = "" if which == "base" else s["expected"]
        s[which + "_ms"] = 1
    ctx.log.emit("eval", which=which, loss=loss if ctx.examples.get("eval") else None,
                 rows=len(ctx.examples.get("eval") or []), answers=len(ctx.samples), samples=len(ctx.samples))


def stage_base_eval(ctx):
    if ctx.fake:
        fake_eval(ctx, "base")
        write_samples(ctx)
        return
    ctx.loss = LossFn(ctx, ctx.model)
    check_loss_path(ctx, ctx.model)
    t0 = time.monotonic()
    available = ctx.budget.remaining() - ctx.budget.max_train_s - SAVE_GGUF_RESERVE_S - 60
    res = run_eval(ctx, ctx.model, "base", available)
    ctx.base_eval_s = time.monotonic() - t0
    ctx.status["train"]["eval_loss_base"] = rnd(res["loss"])
    ctx.discovery["gen_tok_s_base"] = rnd(res["tok_s"], 1)
    write_samples(ctx)


def stage_tuned_eval(ctx):
    if ctx.fake:
        fake_eval(ctx, "tuned")
        write_samples(ctx)
        return
    try:
        ctx.model.gradient_checkpointing_disable()
    except Exception:
        pass
    res = run_eval(ctx, ctx.model, "tuned", ctx.budget.remaining() - SAVE_GGUF_RESERVE_S - 30)
    ctx.status["train"]["eval_loss_tuned"] = rnd(res["loss"])
    ctx.discovery["gen_tok_s_tuned"] = rnd(res["tok_s"], 1)
    write_samples(ctx)
```

- [ ] 14. Stage `train` (D2 manual loop, D4 targets, memory probe).

```python
def record_targets(ctx, wanted, resolved, missing, fallback):
    for name in missing:
        ctx.warn("lora_target_missing", "profile LoRA target %s matches no linear layer" % name)
    if fallback and resolved:
        ctx.warn("lora_targets_fallback", "no profile target matched; using every text linear layer: %s"
                 % ", ".join(resolved))
    if not resolved:
        raise StageError("lora_targets_missing", "the model has no text linear layers to adapt")
    ctx.discovery["lora_targets"] = {"wanted": wanted, "resolved": resolved, "missing": missing, "fallback": fallback}


def enable_checkpointing(ctx, model):
    try:
        try:
            model.gradient_checkpointing_enable(gradient_checkpointing_kwargs={"use_reentrant": False})
        except TypeError:
            model.gradient_checkpointing_enable()
        ctx.discovery["gradient_checkpointing"] = True
    except Exception as e:
        ctx.discovery["gradient_checkpointing"] = False
        ctx.warn("checkpointing_unavailable", "gradient checkpointing could not be enabled: %s" % exc_summary(e))


def wrap_lora(ctx, model, resolved):
    import peft
    lora = ctx.job["hyper"]["lora"]
    cfg = peft.LoraConfig(r=int(lora["r"]), lora_alpha=int(lora["alpha"]), lora_dropout=float(lora.get("dropout") or 0.0),
                          target_modules=lora_regex(resolved), bias="none", task_type="CAUSAL_LM")
    if ctx.job["hyper"].get("gradient_checkpointing") and hasattr(model, "enable_input_require_grads"):
        model.enable_input_require_grads()
    model = peft.get_peft_model(model, cfg)
    wrapped = sum(1 for _, m in model.named_modules() if hasattr(m, "lora_A") and hasattr(m, "base_layer"))
    ctx.discovery["lora_wrapped_modules"] = wrapped
    ctx.log.info("lora", targets=resolved, wrapped_modules=wrapped, r=lora["r"], alpha=lora["alpha"])
    if wrapped == 0:
        raise StageError("lora_targets_missing", "peft wrapped no module for targets %s" % ", ".join(resolved))
    return model


def probe_batch(ctx, model, params, pdb, ga):
    """Forward and backward on the longest rows with the AdamW state reserved; halve the batch on OOM."""
    torch, ex = ctx.torch, ctx.examples["train"]
    longest = sorted(range(len(ex)), key=lambda i: (-len(ex[i][0]), i))
    opt_elems = 2 * sum(p.numel() for p in params)
    while True:
        idxs, oom, t0, reserve = longest[:pdb], False, time.monotonic(), None
        torch.cuda.reset_peak_memory_stats()
        try:
            reserve = torch.empty(opt_elems, dtype=torch.float32, device=ctx.device)
            s, n = ctx.loss.sum_loss(*to_tensors(ctx, ex, idxs), grad=True)
            if s is not None:
                (s / n).backward()
            torch.cuda.synchronize()
        except Exception as e:
            if not is_oom(e):
                raise
            oom = True
        reserve = None
        model.zero_grad(set_to_none=True)
        release(ctx)
        info = {"per_device_batch": pdb, "width": len(ex[idxs[0]][0]), "ok": not oom,
                "peak_vram_gb": rnd(torch.cuda.max_memory_allocated() / 1e9, 2), "seconds": rnd(time.monotonic() - t0, 2)}
        ctx.discovery.setdefault("memory_probe", []).append(info)
        ctx.log.info("memory_probe", **info)
        if not oom:
            return pdb, ga
        if pdb == 1:
            raise StageError("oom", "one sequence of %d tokens does not fit into the memory of %s (method %s); "
                             "choose preset quick or a lower max_seq_len"
                             % (info["width"], ctx.status["gpu"]["name"], ctx.job["method"]))
        pdb, ga = max(1, pdb // 2), ga * 2
        ctx.warn("batch_reduced", "per_device_batch lowered to %d and grad_accum raised to %d after an "
                 "out-of-memory probe" % (pdb, ga))


def fake_train(ctx):
    h = ctx.job["hyper"]
    lengths = [len(e[0]) for e in ctx.examples["train"]]
    spe = steps_per_epoch(len(plan_batches(lengths, h["per_device_batch"], 0, shuffle=False)), h["grad_accum"])
    max_steps = spe * h["epochs"]
    warmup = int(math.ceil(max_steps * float(h.get("warmup_ratio") or 0.0)))
    every = max(1, int(jget(ctx.job, "log.metric_every_steps") or 10))
    loss = None
    for step in range(1, max_steps + 1):
        loss = 0.5 + 2.0 * math.exp(-3.0 * step / max_steps)
        if step == 1 or step % every == 0 or step == max_steps:
            ctx.log.metric(kind="train", step=step, max_steps=max_steps, epoch=rnd(step / float(spe), 3), loss=rnd(loss),
                           lr=rnd(h["learning_rate"] * lr_factor(step - 1, warmup, max_steps), 8), grad_norm=1.0,
                           tok_s=1000.0, elapsed_s=step, eta_s=max_steps - step, vram_gb=1.0)
        if step % spe == 0 and ctx.examples.get("val"):
            ctx.log.metric(kind="eval", step=step, max_steps=max_steps, epoch=step // spe, loss=1.5, eval_loss=1.5,
                           elapsed_s=step, vram_gb=1.0)
    ctx.status["train"].update(steps=max_steps, epochs=float(h["epochs"]), train_loss=rnd(loss), stop_reason="done")
    ctx.discovery["train_tok_s"] = 1000.0


def stage_train(ctx):
    h, lora = ctx.job["hyper"], ctx.job["method"] == "lora"
    resolved = []
    if lora:
        wanted = lora_wanted(ctx)
        resolved, missing, fallback = resolve_targets(wanted, ctx.linear_leaves)
        record_targets(ctx, wanted, resolved, missing, fallback)
    if ctx.fake:
        fake_train(ctx)
        return
    torch = ctx.torch
    random.seed(h["seed"])
    torch.manual_seed(h["seed"])
    torch.cuda.manual_seed_all(h["seed"])
    model = ctx.model
    if h.get("gradient_checkpointing"):
        enable_checkpointing(ctx, model)
    set_use_cache(model, False)
    if lora:
        model = wrap_lora(ctx, model, resolved)
    else:
        for p in model.parameters():
            p.requires_grad_(True)
    ctx.model = model
    params = [p for p in model.parameters() if p.requires_grad]
    ctx.discovery["trainable_params"] = sum(p.numel() for p in params)
    path = ctx.loss.path
    ctx.loss = LossFn(ctx, model)
    if ctx.loss.path == "chunked_head":
        ctx.loss.path = path
    pdb, ga = probe_batch(ctx, model, params, h["per_device_batch"], h["grad_accum"])
    opt = torch.optim.AdamW(params, lr=h["learning_rate"], weight_decay=float(h.get("weight_decay") or 0.0))
    examples = ctx.examples["train"]
    lengths = [len(e[0]) for e in examples]
    n_batches = len(plan_batches(lengths, pdb, 0, shuffle=False))
    spe = steps_per_epoch(n_batches, ga)
    max_steps = spe * h["epochs"]
    warmup = int(math.ceil(max_steps * float(h.get("warmup_ratio") or 0.0)))
    post = SAVE_GGUF_RESERVE_S + 1.2 * ctx.base_eval_s + 30
    deadline = ctx.budget.train_deadline(post)
    every = max(1, int(jget(ctx.job, "log.metric_every_steps") or 10))
    ctx.log.info("train_plan", per_device_batch=pdb, grad_accum=ga, batches_per_epoch=n_batches, steps_per_epoch=spe,
                 max_steps=max_steps, warmup_steps=warmup, train_window_s=int(deadline - time.monotonic()),
                 post_reserve_s=int(post), trainable_params=ctx.discovery["trainable_params"], loss_path=ctx.loss.path)
    if deadline - time.monotonic() < 60:
        raise StageError("time_budget", "only %d s are left for training" % int(deadline - time.monotonic()))
    model.train()
    step, stop_reason, oom_skips, rates = 0, "done", 0, []
    win_loss, win_tok, win_t, last_loss, gn, lr_now = [], 0, time.monotonic(), None, None, 0.0
    t_train = time.monotonic()

    def emit(now):
        elapsed = now - t_train
        tok_s = win_tok / (now - win_t) if now > win_t else None
        if step > 1 and tok_s:
            rates.append(tok_s)
        eta = min(elapsed / step * (max_steps - step), max(0.0, deadline - now))
        ctx.log.metric(kind="train", step=step, max_steps=max_steps, epoch=rnd(step / float(spe), 3),
                       loss=rnd(sum(win_loss) / len(win_loss)), lr=rnd(lr_now, 8), grad_norm=rnd(gn),
                       tok_s=rnd(tok_s, 1), elapsed_s=int(elapsed), eta_s=int(eta), vram_gb=vram_gb(torch))

    for epoch in range(h["epochs"]):
        batches = plan_batches(lengths, pdb, h["seed"] + epoch)
        for g in range(0, len(batches), ga):
            group = batches[g:g + ga]
            if time.monotonic() >= deadline:
                stop_reason = "time_budget"
                break
            targets = sum(label_targets(examples, b) for b in group)
            if targets == 0:
                continue
            lr_now = h["learning_rate"] * lr_factor(step, warmup, max_steps)
            for pg in opt.param_groups:
                pg["lr"] = lr_now
            opt.zero_grad(set_to_none=True)
            loss_sum, tokens, oom = 0.0, 0, False
            try:
                for b in group:
                    ids, mask, labels = to_tensors(ctx, examples, b)
                    s, n = ctx.loss.sum_loss(ids, mask, labels, grad=True)
                    if s is None:
                        continue
                    (s / targets).backward()
                    loss_sum += float(s.detach().item())
                    tokens += int(mask.sum().item())
            except Exception as e:
                if not is_oom(e):
                    raise
                oom = True
            if oom:
                opt.zero_grad(set_to_none=True)
                release(ctx)
                oom_skips += 1
                ctx.warn("oom_step_skipped", "step %d ran out of memory (width %d); skipped"
                         % (step + 1, max(lengths[i] for b in group for i in b)))
                if oom_skips > 3:
                    raise StageError("oom", "training ran out of memory %d times; choose preset quick or a lower "
                                     "max_seq_len" % oom_skips)
                continue
            loss = loss_sum / targets
            if not math.isfinite(loss):
                raise StageError("train_diverged", "loss became %s at step %d; lower the learning rate" % (loss, step + 1))
            gn = float(torch.nn.utils.clip_grad_norm_(params, 1.0))
            if not math.isfinite(gn):
                raise StageError("train_diverged", "gradient norm became %s at step %d" % (gn, step + 1))
            opt.step()
            step += 1
            win_loss.append(loss)
            win_tok += tokens
            if step == 1 or step % every == 0 or step == max_steps:
                now = time.monotonic()
                emit(now)
                last_loss = sum(win_loss) / len(win_loss)
                win_loss, win_tok, win_t = [], 0, now
        if stop_reason == "time_budget":
            break
        if ctx.examples.get("val") and time.monotonic() < deadline:
            vl = eval_loss(ctx, model, ctx.examples["val"], pdb)
            ctx.log.metric(kind="eval", step=step, max_steps=max_steps, epoch=epoch + 1, loss=rnd(vl), eval_loss=rnd(vl),
                           elapsed_s=int(time.monotonic() - t_train), vram_gb=vram_gb(torch))
            model.train()
    if win_loss and step:
        emit(time.monotonic())
        last_loss = sum(win_loss) / len(win_loss)
    if step == 0:
        raise StageError("time_budget", "no training step finished within the time budget")
    ctx.status["train"].update(steps=step, epochs=rnd(step / float(spe), 3), train_loss=rnd(last_loss),
                               stop_reason=stop_reason)
    ctx.discovery.update(train_tok_s=rnd(sorted(rates)[len(rates) // 2], 1) if rates else None, oom_skips=oom_skips,
                         train_seconds=rnd(time.monotonic() - t_train, 1))
    if stop_reason == "time_budget":
        ctx.warn("time_budget", "training stopped at step %d of %d by the time budget" % (step, max_steps), primary=True)
    model.eval()
```

- [ ] 15. Stages `save`, `gguf`, `manifest`.

```python
def copy_tokenizer_files(src, dst):
    """D8: the vendor's tokenizer files from the base snapshot, not re-saved ones."""
    copied = []
    for name in sorted(os.listdir(src)):
        if name in TOKENIZER_FILES or (name.startswith("tokenizer") and not name.endswith(".safetensors")):
            path = os.path.join(src, name)
            if os.path.isfile(path):
                shutil.copyfile(path, os.path.join(dst, name))
                copied.append(name)
    gen = os.path.join(src, "generation_config.json")
    if os.path.isfile(gen) and not os.path.isfile(os.path.join(dst, "generation_config.json")):
        shutil.copyfile(gen, os.path.join(dst, "generation_config.json"))
        copied.append("generation_config.json")
    return copied


def fake_save(ctx):
    if ctx.job["method"] != "lora":
        return
    lora = ctx.job["hyper"]["lora"]
    write_safetensors_empty(ctx.path("adapter"), {"format": "pt", "chibi": "dry-run"})
    write_json_atomic(ctx.path("adapter_config"), {
        "peft_type": "LORA", "task_type": "CAUSAL_LM", "r": lora["r"], "lora_alpha": lora["alpha"],
        "lora_dropout": lora.get("dropout", 0.0), "target_modules": ctx.discovery["lora_targets"]["resolved"],
        "base_model_name_or_path": ctx.profile["base"]["repo"]})


def stage_save(ctx):
    if ctx.fake:
        fake_save(ctx)
        return
    model = ctx.model
    shutil.rmtree(ctx.merged_dir, ignore_errors=True)
    if ctx.job["method"] == "lora":
        adir = os.path.join(ctx.work, "adapter")
        shutil.rmtree(adir, ignore_errors=True)
        model.save_pretrained(adir)
        for src, kind in (("adapter_model.safetensors", "adapter"), ("adapter_config.json", "adapter_config")):
            path = os.path.join(adir, src)
            if not os.path.isfile(path):
                raise StageError("merge_failed", "peft wrote no %s (files: %s)" % (src, ", ".join(sorted(os.listdir(adir)))))
            shutil.copyfile(path, ctx.path(kind))
        t0 = time.monotonic()
        model = model.merge_and_unload()
        ctx.log.info("merged", seconds=rnd(time.monotonic() - t0, 1), model_class=type(model).__name__)
    set_use_cache(model, True)
    model = model.to("cpu")
    ctx.model = None
    release(ctx)
    try:
        model.save_pretrained(ctx.merged_dir, safe_serialization=True, max_shard_size="2GB")
    except TypeError:
        model.save_pretrained(ctx.merged_dir)
    if not any(n.endswith(".safetensors") for n in os.listdir(ctx.merged_dir)):
        raise StageError("merge_failed", "save_pretrained wrote no safetensors file")
    copied = copy_tokenizer_files(ctx.snapshot_dir, ctx.merged_dir)
    cfg = read_json(os.path.join(ctx.merged_dir, "config.json"))
    keys = model_dir_keys(ctx.merged_dir)
    info = {"architectures": cfg.get("architectures"), "model_type": cfg.get("model_type"),
            "dtype": cfg.get("dtype", cfg.get("torch_dtype")), "tensors": len(keys), "bytes": dir_bytes(ctx.merged_dir),
            "mtp_tensors": sum(1 for k in keys if "mtp" in k), "tokenizer_files": copied}
    ctx.discovery["merged"] = info
    ctx.log.info("merged_model", **info)
    del model
    release(ctx)


def stage_gguf(ctx):
    dst = ctx.path("gguf_tuned")
    if ctx.fake:
        write_fake_gguf(dst, ctx.profile["gguf"]["expected_arch"])
    else:
        convert(ctx, ctx.merged_dir, dst, "tuned")
    verify_gguf(ctx, dst, "tuned")
    shutil.rmtree(ctx.merged_dir, ignore_errors=True)


def stage_manifest(ctx):
    names = output_names(ctx.outtype)
    need = ["gguf_tuned", "metrics", "eval_samples"] + (["adapter", "adapter_config"] if ctx.job["method"] == "lora" else [])
    missing = [names[k] for k in need if not os.path.isfile(os.path.join(ctx.out, names[k]))]
    if missing:
        raise StageError("output_failed", "missing outputs: " + ", ".join(missing))
    for kind in MANIFEST_KINDS:
        path = os.path.join(ctx.out, names[kind])
        if os.path.isfile(path):
            ctx.log.info("output", kind_name=kind, path=names[kind], bytes=os.path.getsize(path))
```

- [ ] 16. Stage table, stage runner, finalize, main.

```python
STAGES = (
    # name          failure code for unexpected exceptions   function
    ("boot",        "job_invalid",                           stage_boot),
    ("env_check",   "env_no_gpu",                            stage_env_check),
    ("data",        "data_mismatch",                         stage_data),
    ("install",     "install_failed",                        stage_install),
    ("load_base",   "base_load_failed",                      stage_load_base),
    ("converter",   "gguf_failed",                           stage_converter),
    ("base_eval",   "eval_failed",                           stage_base_eval),
    ("train",       "train_failed",                          stage_train),
    ("tuned_eval",  "eval_failed",                           stage_tuned_eval),
    ("save",        "merge_failed",                          stage_save),
    ("gguf",        "gguf_failed",                           stage_gguf),
    ("manifest",    "output_failed",                         stage_manifest),
)


def run_stage(ctx, name, code, fn):
    ctx.status["stage"] = name
    ctx.log.emit("stage", stage=name, state="start", elapsed_s=int(ctx.budget.elapsed()),
                 remaining_s=int(ctx.budget.remaining()) if ctx.job else None)
    t0, failure = time.monotonic(), None
    try:
        fn(ctx)
    except StageError as e:
        failure = ("oom" if is_oom(e.__cause__ or e.__context__) else e.code, e.message, None)
    except Interrupted as e:
        failure = ("interrupted", "the session was stopped (%s)" % e, None)
    except Exception as e:
        failure = ("oom" if is_oom(e) else code, exc_summary(e), trace_tail(e))
        traceback.print_exc()
    secs = round(time.monotonic() - t0, 1)
    ctx.discovery["stage_seconds"][name] = secs
    note_vram(ctx)
    if not ctx.fake:
        release(ctx)
    if failure is None:
        ctx.log.emit("stage", stage=name, state="done", duration_s=secs)
        return
    fcode, message, trace = failure
    ctx.log.emit("stage", stage=name, state="failed", duration_s=secs)
    ctx.log.emit("error", stage=name, code=fcode, message=message, trace=trace)
    ctx.status.update(state="failed", code=fcode, message=message)
    raise RunFailed()


def finalize(ctx):
    st = ctx.status
    try:
        if ctx.fake and heavy_loaded() and st["state"] == "succeeded":
            st.update(state="failed", code="internal", message="the dry run imported " + ", ".join(heavy_loaded()))
        if st["state"] == "running":
            st.update(state="failed", code=st["code"] or "internal", message=st["message"] or "the run ended without a result")
        st["finished"] = utc_now()
        st["discovery"]["total_s"] = round(time.monotonic() - ctx.t0, 1)
        note_vram(ctx)
        write_json_atomic(ctx.path("status"), st)
        files = write_manifest(ctx.out, ctx.outtype, st["run_id"], ctx.profile.get("id", ""),
                               (ctx.job or {}).get("profile_sha", ""))
        ctx.log.emit("done", state=st["state"], code=st["code"], message=st["message"], warning_code=st["warning_code"],
                     files=len(files), duration_s=st["discovery"]["total_s"])
    except BaseException as e:
        ctx.log.emit("error", stage="finalize", code="output_failed", message=exc_summary(e))
        traceback.print_exc()
    finally:
        if ctx.fake:
            shutil.rmtree(ctx.work, ignore_errors=True)


def parse_args(argv):
    p = argparse.ArgumentParser(description="AuraGo Chibi Sensei trainer")
    p.add_argument("--job")
    p.add_argument("--out")
    p.add_argument("--input", default=DEFAULT_INPUT)
    p.add_argument("--selftest", action="store_true")
    p.add_argument("--dry-run", dest="dry_run", action="store_true")
    return p.parse_args(argv)


def main(argv=None):
    harden_stdio()
    args = parse_args(argv)
    if args.selftest:
        return selftest()
    removed, run_type = scrub_environment()
    log = Log()
    if not args.job or not args.out:
        log.emit("error", stage="boot", code="job_invalid", message="--job and --out are required")
        return 2
    if args.dry_run and os.environ.get("CHIBI_FAKE_ENV") != "1":
        log.emit("error", stage="boot", code="job_invalid", message="--dry-run runs only with CHIBI_FAKE_ENV=1")
        return 2
    ctx = Ctx(args, log, args.dry_run)
    ctx.discovery.update(env_removed=removed, kaggle_run_type=run_type)
    install_signal_handlers()
    rc = 1
    try:
        for name, code, fn in STAGES:
            run_stage(ctx, name, code, fn)
        ctx.status["state"] = "succeeded"
        rc = 0
    except RunFailed:
        pass
    except Interrupted as e:
        ctx.status.update(state="failed", code="interrupted", message="the session was stopped (%s)" % e)
    except BaseException as e:
        ctx.status.update(state="failed", code="internal", message=exc_summary(e))
        traceback.print_exc()
    finally:
        finalize(ctx)
    return rc
```

- [ ] 17. Selftest and entry point (end of file).

```python
_DELETE = object()


def expect(cond, detail):
    if not cond:
        raise AssertionError(detail)


def expect_stage_error(fn, code, text):
    try:
        fn()
    except StageError as e:
        expect(e.code == code and text in e.message, "got %s: %s" % (e.code, e.message))
        return
    raise AssertionError("no StageError %s (%s)" % (code, text))


def jset(obj, path, value):
    parts = path.split(".")
    for part in parts[:-1]:
        obj = obj[part]
    if value is _DELETE:
        obj.pop(parts[-1], None)
    else:
        obj[parts[-1]] = value


def sample_job():
    profile = {"id": "stub-1", "chat": {"system_role": "native", "stop": ["<|im_end|>"]},
               "base": {"repo": "org/model", "revision": "a" * 40, "auto_class": "AutoModelForCausalLM",
                        "auto_class_fallbacks": ["AutoModelForMultimodalLM"]},
               "gguf": {"expected_arch": "stub", "estimated_mb": 10}, "arch": {"params_b": 0.1},
               "training": {"min_gpu_capability": "7.0", "min_vram_gb": 8, "t4_gen_tokens_per_second": 30,
                            "extra_pip": [], "lora": {"target_modules": ["q_proj"]}}}
    return {"v": 1, "run_id": "11111111-2222-4333-8444-555555555555", "created": "2026-11-23T00:00:00Z",
            "aurago_version": "test", "profile": profile, "profile_sha": "b" * 64, "recipe": "assistant",
            "task_kind": None, "labels": [], "method": "lora",
            "data": {"marker": "r111111112222-marker.json", "format": "messages"},
            "hyper": {"epochs": 1, "learning_rate": 0.0002, "lr_scheduler": "cosine", "warmup_ratio": 0.03,
                      "weight_decay": 0, "per_device_batch": 2, "grad_accum": 2, "max_seq_len": 256, "seed": 42,
                      "gradient_checkpointing": True, "loss_on": "assistant",
                      "lora": {"r": 8, "alpha": 16, "dropout": 0.05, "target_modules": ["q_proj"]}},
            "budget": {"session_timeout_s": 3600, "max_train_s": 1800, "eval_max_samples": 10, "max_new_tokens": 32},
            "outputs": {"emit_base_gguf": True, "gguf_outtype": "q8_0", "prefix": "chibi-"},
            "deps": {"pip": None, "transformers": "",
                     "llama_cpp": {"tag": "b9994", "url": "https://example.invalid/x.tar.gz", "sha256": ""}},
            "log": {"prefix": "@@chibi", "metric_every_steps": 10}}


def st_log():
    buf = io.StringIO()
    log = Log(stream=buf)
    log.emit("info", kind="x", message="\u00e4" * 5000, value=float("nan"))
    log.emit("warn", code="c", message="m")
    lines = buf.getvalue().splitlines()
    expect(len(lines) == 2, "two lines expected, got %d" % len(lines))
    for n, line in enumerate(lines, 1):
        expect(line.startswith(LOG_PREFIX + " {") and len(line.encode("utf-8")) <= MAX_LINE_BYTES, "line %d shape" % n)
        obj = json.loads(line[len(LOG_PREFIX) + 1:])
        expect(obj["v"] == 1 and obj["seq"] == n and obj["t"] and obj["ts"].endswith("Z"), "line %d fields" % n)
    first = json.loads(lines[0][len(LOG_PREFIX) + 1:])
    expect(first.get("cut") is True and first.get("value") is None, "long line must be cut and NaN must be null")
    expect(all(ord(c) < 128 for c in buf.getvalue()), "log lines must be ASCII")


def st_job():
    expect(validate_job(sample_job()) == [], "sample job: %r" % validate_job(sample_job()))
    cases = (("v", 2, "v:"), ("run_id", _DELETE, "run_id:"), ("method", "qlora", "method:"),
             ("data.format", "text", "data.format:"), ("data.marker", "../x.json", "data.marker:"),
             ("hyper.epochs", 0, "hyper.epochs:"), ("hyper.lora", None, "hyper.lora:"),
             ("budget.max_train_s", 4000, "budget:"), ("outputs.gguf_outtype", "Q8 0", "outputs.gguf_outtype:"),
             ("hyper.learning_rate", True, "hyper.learning_rate:"), ("deps.pip", "peft", "deps.pip:"))
    for path, value, prefix in cases:
        job = sample_job()
        jset(job, path, value)
        problems = validate_job(job)
        expect(any(p.startswith(prefix) for p in problems), "%s=%r gave %r" % (path, value, problems))
    job = sample_job()
    job["method"], job["hyper"]["lora"] = "full", None
    expect(validate_job(job) == [], "full without lora: %r" % validate_job(job))


def st_marker():
    with tempfile.TemporaryDirectory() as root:
        d = os.path.join(root, "ds", "v1")
        os.makedirs(d)
        train = os.path.join(d, "r1-train.jsonl")
        write_jsonl_atomic(train, [{"messages": [{"role": "user", "content": "hi %d" % i},
                                                 {"role": "assistant", "content": "yo"}]} for i in range(3)])
        write_json_atomic(os.path.join(d, "r1-marker.json"), {"v": 1, "run_id": "run", "files": {
            "train": {"name": "r1-train.jsonl", "sha256": sha256_file(train), "rows": 3}}})
        marker, base = load_marker(root, "r1-marker.json", "run")
        files = verify_split_files(marker, base, root)
        expect(list(files) == ["train"], "splits: %r" % list(files))
        rows = parse_rows(files["train"][0], "messages", 3, "train")
        expect(len(rows) == 3 and rows[0]["_line"] == 1 and sample_id(rows[0]) == "eval-1", "rows")
        expect_stage_error(lambda: load_marker(root, "r1-marker.json", "other"), "data_mismatch", "belongs to run")
        expect_stage_error(lambda: load_marker(root, "missing.json", "run"), "data_mismatch", "not found")
        with open(train, "a", encoding="utf-8") as f:
            f.write("\n")
        expect_stage_error(lambda: verify_split_files(marker, base, root), "data_mismatch", "sha256")
        bad = os.path.join(d, "bad.jsonl")
        with open(bad, "w", encoding="utf-8") as f:
            f.write('{"messages":[{"role":"user","content":"x"}]}\n')
        expect_stage_error(lambda: parse_rows(bad, "messages", 1, "train"), "data_mismatch", "line 1")
        deep = os.path.join(root, "a", "b", "c", "d", "e")
        os.makedirs(deep)
        write_json_atomic(os.path.join(deep, "deep.json"), {})
        expect(find_files(root, {"deep.json"})[0] == {}, "a marker below depth 4 must not be found")


def st_masking():
    tok = StubTokenizer()
    msgs = [{"role": "system", "content": "S"}, {"role": "user", "content": "hi"}, {"role": "assistant", "content": "yo"}]
    ids, labels, cut = build_example(tok, {"messages": msgs}, "messages", "native", "<|im_end|>", 512)
    prompt = encode(tok, tok.apply_chat_template(msgs[:-1], add_generation_prompt=True))
    expect(ids[:len(prompt)] == prompt and labels[:len(prompt)] == [-100] * len(prompt), "prompt must be masked")
    answer = tok.decode([y for y in labels if y != -100])
    expect(answer == "yo<|im_end|>", "answer labels: %r" % answer)
    expect(ids.count(tok.bos_token_id) == 1 and not cut, "exactly one BOS, no truncation")
    merged = tok.decode(build_example(tok, {"messages": msgs}, "messages", "merge_into_first_user", "<|im_end|>", 512)[0])
    expect("S\n\nhi" in merged and "system" not in merged, "merge: %r" % merged)
    short, slab, cut = build_example(tok, {"messages": msgs}, "messages", "native", "<|im_end|>", 8)
    expect(cut and len(short) == 8 and short[0] == ids[0] and short[1:] == ids[-7:] and slab[1:] == labels[-7:],
           "truncation keeps the first and the last tokens")
    t_ids, t_labels, _ = build_example(tok, {"text": "abc"}, "text", "native", "", 512)
    expect(t_ids == [1] + encode(tok, "abc") + [2] and t_labels == t_ids, "text rows: bos + text + eos, all labelled")
    expect(drop_double_bos(tok, [1, 1, 7]) == [1, 7] and drop_double_bos(tok, [1, 7]) == [1, 7], "double BOS")
    expect(merge_system([{"role": "system", "content": "S"}, {"role": "assistant", "content": "a"}])[0]
           == {"role": "user", "content": "S"}, "system without user")


def st_batching():
    lengths = [5, 1, 9, 3, 7, 2, 8]
    batches = plan_batches(lengths, 3, 42)
    expect(sorted(i for b in batches for i in b) == list(range(7)), "every row once")
    order = sorted(range(7), key=lambda i: (lengths[i], i))
    expect(sorted(map(tuple, batches)) == sorted(tuple(order[i:i + 3]) for i in range(0, 7, 3)), "length buckets")
    ex = [([1, 2, 3], [-100, 2, 3], False), ([4], [4], False)]
    ids, mask, labels = pad_batch(ex, [0, 1], 0)
    expect(len(ids[0]) == 8 and mask[1] == [1] + [0] * 7 and labels[1] == [4] + [-100] * 7 and ids[1][1:] == [0] * 7,
           "dynamic right padding to a multiple of 8")
    expect(label_targets(ex, [0, 1]) == 2 and steps_per_epoch(7, 2) == 4, "targets and steps")
    expect(abs(lr_factor(0, 2, 10) - 0.5) < 1e-9 and abs(lr_factor(2, 2, 10) - 1.0) < 1e-9
           and lr_factor(10, 2, 10) < 1e-9, "warmup and cosine schedule")


def st_targets():
    leaves = {"q_proj": 4, "k_proj": 4, "out_proj": 6, "w1": 6}
    expect(resolve_targets(["q_proj", "o_proj", "w1"], leaves) == (["q_proj", "w1"], ["o_proj"], False), "intersection")
    expect(resolve_targets(["gate_proj"], leaves) == (sorted(leaves), ["gate_proj"], True), "fallback to every leaf")
    rx = lora_regex(["q_proj", "w1"])
    expect(re.fullmatch(rx, "model.layers.0.self_attn.q_proj") and re.fullmatch(rx, "model.layers.3.feed_forward.w1"),
           "text modules match")
    for name in ("model.visual.blocks.0.attn.q_proj", "model.layers.0.self_attn.q_proj_x", "lm_head"):
        expect(not re.fullmatch(rx, name), "%s must not match" % name)


def st_outputs():
    with tempfile.TemporaryDirectory() as d:
        names = output_names("q8_0")
        write_fake_gguf(os.path.join(d, names["gguf_tuned"]), "gemma3")
        hdr = read_gguf_header(os.path.join(d, names["gguf_tuned"]))
        expect(hdr == {"version": 3, "tensors": 1, "kv": 2, "arch": "gemma3"}, "gguf header: %r" % hdr)
        write_safetensors_empty(os.path.join(d, names["adapter"]), {"format": "pt"})
        expect(safetensors_keys(os.path.join(d, names["adapter"])) == [], "empty safetensors")
        write_json_atomic(os.path.join(d, names["status"]), {"v": 1, "x": float("nan")})
        expect(read_json(os.path.join(d, names["status"])) == {"v": 1, "x": None}, "status NaN becomes null")
        open(os.path.join(d, names["metrics"]), "w").close()
        open(os.path.join(d, "chibi-other.txt"), "w").close()
        write_manifest(d, "q8_0", "run", "prof", "sha")
        man = read_json(os.path.join(d, MANIFEST_NAME))
        expect([f["kind"] for f in man["files"]] == ["gguf_tuned", "adapter", "metrics", "status"], "kinds: %r" % man)
        for f in man["files"]:
            expect(f["sha256"] == sha256_file(os.path.join(d, f["path"])) and safe_name(f["path"])
                   and f["path"].startswith("chibi-"), "manifest entry %r" % f)
        expect(man["v"] == 1 and man["run_id"] == "run" and not any(n.startswith(".tmp-") for n in os.listdir(d)),
               "manifest header and no temp files")


def st_requirements():
    with tempfile.TemporaryDirectory() as d:
        with open(os.path.join(d, "a.txt"), "w", encoding="utf-8") as f:
            f.write('-r ./b.txt\n--extra-index-url https://x\ntorch==2.11.0; platform_machine != "s390x"\n')
        with open(os.path.join(d, "b.txt"), "w", encoding="utf-8") as f:
            f.write("numpy~=1.26.4\ngguf>=0.1.0\ntransformers==4.57.6\n")
        lines = flatten_requirements(os.path.join(d, "a.txt"))
        expect(lines[0] == "numpy~=1.26.4" and len(lines) == 5, "flatten: %r" % lines)
        new, relaxed = adapt_requirements(lines, (3, 13))
        expect("numpy>=2.1,<3" in new and not any(x.startswith("gguf") for x in new) and len(relaxed) == 2, "3.13: %r" % new)
        new, _ = adapt_requirements(lines, (3, 12))
        expect("numpy~=1.26.4" in new, "3.12 keeps numpy")


def st_budget():
    now = [100.0]
    b = Budget(3600, 1800, clock=lambda: now[0])
    now[0] = 200.0
    expect(abs(b.remaining() - (3600 - SESSION_LEAD_S - 100)) < 1e-9, "remaining")
    expect(abs(b.train_deadline(600) - 2000.0) < 1e-9, "deadline by max_train_s")
    expect(abs(b.train_deadline(2000) - (100 + 3600 - SESSION_LEAD_S - 2000)) < 1e-9, "deadline by session")
    expect(generation_plan(10, 64, 32.0, 1000.0) == (15.0, 90.0), "generation caps")
    expect(generation_plan(10, 64, 32.0, 30.0)[1] == 30.0, "generation capped by available time")


def st_answers():
    tok = StubTokenizer()
    expect(decode_answer(tok, encode(tok, "yes<|im_end|>more"), [4], ["<|im_end|>"]) == ("yes", True), "stop id")
    expect(decode_answer(tok, encode(tok, "abc"), [4], ["<|im_end|>"]) == ("abc", False), "no stop")
    expect(stop_token_ids(tok, ["<|im_end|>", "##"], [99]) == [2, 4, 99], "stop ids")
    expect(split_text_prompt("one two three four") == ("one two", " three four"), "text prompt split")


SELFTESTS = (("log encoder", st_log), ("job schema", st_job), ("marker and sha256", st_marker),
             ("masking", st_masking), ("batching", st_batching), ("lora targets", st_targets),
             ("status and manifest", st_outputs), ("converter requirements", st_requirements),
             ("budget", st_budget), ("answers", st_answers))


def selftest():
    for name, fn in SELFTESTS:
        try:
            fn()
        except Exception as e:
            print("selftest FAILED: %s: %s" % (name, exc_summary(e)))
            traceback.print_exc()
            return 1
    if heavy_loaded():
        print("selftest FAILED: imports: heavy modules loaded: %s" % ", ".join(heavy_loaded()))
        return 1
    print("selftest ok (%d checks)" % len(SELFTESTS))
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] 18. Check the script from Git Bash at the worktree root:
  - `python -m py_compile internal/chibisensei/trainer/chibi_train.py && echo compiled` → `compiled`. Remove the generated `internal/chibisensei/trainer/__pycache__/` afterwards.
  - `python -I internal/chibisensei/trainer/chibi_train.py --selftest` → `selftest ok (10 checks)`, exit 0.
  - `wc -l internal/chibisensei/trainer/chibi_train.py` → about 1,550 lines. That is above the brief's 900–1,200 target, because the selftest, the dry-run stubs and the converter fallback add about 400. Report the number. Do not shorten the script to reach the target.

- [ ] 19. Create `internal/chibisensei/trainer/selftest_test.go`. Use the `Status` field names `State` and `RunID` and the `Metric` / `EvalSample` / `ManifestFile` names exactly as B2a's `outputs.go` and `logparse.go` define them. If they differ, adapt this file and report the difference.

```go
package trainer_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/chibisensei/trainer"

	"github.com/google/uuid"
)

// findPython returns python3 or python (3.10+), or skips the test (D19).
func findPython(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		p, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		out, err := exec.Command(p, "-I", "-c", "import sys; print(sys.version_info[0]*100+sys.version_info[1])").Output()
		if err != nil {
			continue
		}
		if v, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && v >= 310 {
			return p
		}
	}
	t.Skip("no Python 3.10+ (python3 or python) on PATH")
	return ""
}

func writeScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chibi_train.py")
	if err := os.WriteFile(path, trainer.Script(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runPython(t *testing.T, py string, env []string, args ...string) ([]byte, []byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, py, append([]string{"-I"}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func tail(b []byte) string {
	if len(b) > 4000 {
		b = b[len(b)-4000:]
	}
	return string(b)
}

func TestTrainerSelftest(t *testing.T) {
	py := findPython(t)
	stdout, stderr, err := runPython(t, py, nil, writeScript(t), "--selftest")
	if err != nil || !strings.Contains(string(stdout), "selftest ok") {
		t.Fatalf("selftest: %v\nstdout:\n%s\nstderr:\n%s", err, tail(stdout), tail(stderr))
	}
}

func TestTrainerDryRunNeedsFakeEnv(t *testing.T) {
	py := findPython(t)
	dir := t.TempDir()
	_, _, err := runPython(t, py, []string{"CHIBI_FAKE_ENV=0"}, writeScript(t), "--dry-run",
		"--job", filepath.Join(dir, "job.json"), "--out", filepath.Join(dir, "out"))
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("dry run without CHIBI_FAKE_ENV=1 must exit 2, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out")); !os.IsNotExist(err) {
		t.Fatalf("a refused dry run must not create the output directory")
	}
}

type dryCase struct {
	name, profile, recipe, taskKind string
	labels                          []string
	rows                            int
	method, format                  string
}

var dryCases = []dryCase{
	{name: "qwen-lora-assistant", profile: "qwen35-0p8b", recipe: chibisensei.RecipeAssistant, rows: 60, method: "lora", format: "messages"},
	{name: "lfm-lora-assistant", profile: "lfm25-1p2b", recipe: chibisensei.RecipeAssistant, rows: 60, method: "lora", format: "messages"},
	{name: "gemma-full-classify", profile: "gemma3-270m", recipe: chibisensei.RecipeSpecialist, taskKind: "classify",
		labels: []string{"pos", "neg"}, rows: 240, method: "full", format: "messages"},
	{name: "qwen-lora-free", profile: "qwen35-0p8b", recipe: chibisensei.RecipeFree, rows: 60, method: "lora", format: "text"},
}

func caseRows(c dryCase) []chibisensei.RowInput {
	out := make([]chibisensei.RowInput, 0, c.rows)
	for i := 0; i < c.rows; i++ {
		switch {
		case c.recipe == chibisensei.RecipeFree:
			s := fmt.Sprintf("Note %d: the quick brown fox number %d jumps over the lazy dog and keeps running.", i, i*7)
			out = append(out, chibisensei.RowInput{Text: &s})
		case c.taskKind == "classify":
			label := c.labels[i%len(c.labels)]
			out = append(out, chibisensei.RowInput{Messages: []chibisensei.Message{
				{Role: "user", Content: fmt.Sprintf("Review %d: this product is %s.", i, label)},
				{Role: "assistant", Content: label}}})
		default:
			var msgs []chibisensei.Message
			if i%3 == 0 {
				msgs = append(msgs, chibisensei.Message{Role: "system", Content: "Antworte kurz und freundlich."})
			}
			msgs = append(msgs, chibisensei.Message{Role: "user", Content: fmt.Sprintf("Frage %d: Wie spät ist es?", i)},
				chibisensei.Message{Role: "assistant", Content: fmt.Sprintf("Antwort %d: Zeit für eine Pause. ☕", i)})
			out = append(out, chibisensei.RowInput{Messages: msgs})
		}
	}
	return out
}

type dryJob struct {
	runID string
	snap  chibisensei.Snapshot
	plan  chibisensei.Plan
	job   chibisensei.JobSpec
}

func buildDryJob(t *testing.T, cat *chibisensei.Catalog, lock trainer.Lock, c dryCase, inputRoot string) dryJob {
	t.Helper()
	ctx := context.Background()
	st, err := chibisensei.OpenStore(filepath.Join(t.TempDir(), "chibi_sensei.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	proj, err := st.CreateProject(ctx, chibisensei.ProjectInput{Name: c.name, Recipe: c.recipe, TaskKind: c.taskKind,
		Labels: c.labels, GoalText: "dry run", ProfileID: c.profile})
	if err != nil {
		t.Fatal(err)
	}
	rows := caseRows(c)
	for start := 0; start < len(rows); start += 100 {
		end := min(start+100, len(rows))
		if _, err := st.AddRows(ctx, proj.ID, rows[start:end], chibisensei.AddOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	srows, proj, err := st.LoadSplitRows(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	prof, ok := cat.Get(c.profile)
	if !ok {
		t.Fatalf("profile %s missing", c.profile)
	}
	split := chibisensei.ComputeSplit(srows, proj.SplitSeed, proj.TaskKind == "classify")
	plan, err := chibisensei.ComputePlan(chibisensei.PlanInput{Project: proj, Profile: prof, Rows: srows, Split: split,
		RemoteTimeoutMinutes: 120, NeedBaseGGUF: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != c.method {
		t.Fatalf("plan method %s, want %s", plan.Method, c.method)
	}
	runID := uuid.NewString()
	snapDir := filepath.Join(inputRoot, "chibi-data-"+chibisensei.Run12(runID))
	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		t.Fatal(err)
	}
	snap, err := st.FreezeSnapshot(ctx, proj.ID, runID, snapDir)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Format != c.format {
		t.Fatalf("snapshot format %s, want %s", snap.Format, c.format)
	}
	job, err := chibisensei.BuildJobSpec(chibisensei.JobInput{RunID: runID, Created: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC),
		AuraGoVersion: "test", Project: proj, Profile: prof, ProfileSHA: cat.SHA(c.profile), Plan: plan,
		Marker: snap.Marker.Name, Format: snap.Format, Lock: lock})
	if err != nil {
		t.Fatal(err)
	}
	return dryJob{runID: runID, snap: snap, plan: plan, job: job}
}

func TestTrainerDryRun(t *testing.T) {
	py := findPython(t)
	script := writeScript(t)
	cat, err := chibisensei.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	lock, err := trainer.LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range dryCases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input")
			dj := buildDryJob(t, cat, lock, c, input)
			body, err := dj.job.JSON()
			if err != nil {
				t.Fatal(err)
			}
			jobPath, out := filepath.Join(dir, "job.json"), filepath.Join(dir, "out")
			if err := os.WriteFile(jobPath, body, 0o600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, err := runPython(t, py, []string{"CHIBI_FAKE_ENV=1"}, script, "--dry-run",
				"--job", jobPath, "--out", out, "--input", input)
			if err != nil {
				t.Fatalf("dry run: %v\nstdout:\n%s\nstderr:\n%s", err, tail(stdout), tail(stderr))
			}
			checkDryRunOutputs(t, c, dj, out, stdout)
		})
	}
}

var trainerStages = []string{"boot", "env_check", "data", "install", "load_base", "converter",
	"base_eval", "train", "tuned_eval", "save", "gguf", "manifest"}

// checkDryRunOutputs parses every output of one dry run with the production parsers.
// B4 extends this function with the ReadGGUFInfo assertions (see the B4 note in the plan).
func checkDryRunOutputs(t *testing.T, c dryCase, dj dryJob, out string, stdout []byte) {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return b
	}
	st, err := chibisensei.ParseStatus(read("chibi-status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "succeeded" || st.RunID != dj.runID {
		t.Fatalf("status: %+v", st)
	}
	if c.method == "lora" && st.Discovery["lora_targets"] == nil {
		t.Fatalf("status.discovery.lora_targets missing")
	}
	man, err := chibisensei.ParseManifest(read("chibi-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if man.RunID != dj.runID || man.ProfileID != c.profile {
		t.Fatalf("manifest header: %+v", man)
	}
	kinds := map[string]bool{}
	for _, f := range man.Files {
		b := read(f.Path)
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != f.SHA256 || int64(len(b)) != f.Bytes {
			t.Fatalf("manifest entry %s does not match the file", f.Path)
		}
		kinds[f.Kind] = true
	}
	want := []string{"gguf_tuned", "metrics", "eval_samples", "status"}
	if c.method == "lora" {
		want = append(want, "adapter", "adapter_config")
	}
	for _, k := range want {
		if !kinds[k] {
			t.Fatalf("manifest lacks kind %s: %v", k, kinds)
		}
	}
	mf, err := os.Open(filepath.Join(out, "chibi-metrics.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer mf.Close()
	metrics, err := chibisensei.ParseMetricsFile(mf)
	if err != nil || len(metrics) == 0 {
		t.Fatalf("metrics: %d, %v", len(metrics), err)
	}
	last := metrics[len(metrics)-1]
	for i := len(metrics) - 1; i >= 0 && last.Kind != "train"; i-- {
		last = metrics[i]
	}
	if last.Kind != "train" || last.Step != last.MaxSteps || last.MaxSteps < 1 {
		t.Fatalf("last train metric: %+v", last)
	}
	ef, err := os.Open(filepath.Join(out, "chibi-eval_samples.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer ef.Close()
	samples, err := chibisensei.ParseEvalSamples(ef, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if want := min(dj.snap.Files["eval"].Rows, dj.plan.EvalSamples); len(samples) != want {
		t.Fatalf("eval samples: %d, want %d", len(samples), want)
	}
	events := chibisensei.ParseKernelLog(stdout)
	done := map[string]bool{}
	var final map[string]any
	for _, e := range events {
		if se, ok := e.Stage(); ok && se.State == "done" {
			done[se.Stage] = true
		}
		if ee, ok := e.Error(); ok {
			t.Fatalf("error event: %+v", ee)
		}
		if e.T == "done" {
			if err := json.Unmarshal(e.Raw, &final); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, s := range trainerStages {
		if !done[s] {
			t.Fatalf("stage %s has no done event", s)
		}
	}
	if final == nil || final["state"] != "succeeded" {
		t.Fatalf("done event: %v", final)
	}
	// B4: ReadGGUFInfo assertions go here.
}
```

- [ ] 20. Verify from Git Bash at the worktree root:
  - `go build ./...` → no output.
  - `go vet ./internal/chibisensei/trainer/` → no output. If Go reports `no such tool "vet"`, prefix the command with `GOTOOLCHAIN=go1.27.4`.
  - `go test ./internal/chibisensei/trainer/ -count=1 -v -run '(?i)(TrainerSelftest|TrainerDryRun)'` → `PASS` for `TestTrainerSelftest`, `TestTrainerDryRunNeedsFakeEnv` and the four `TestTrainerDryRun/*` subtests, or `SKIP` with "no Python 3.10+" on a machine without Python. Count the PASS lines in the report.
  - `go test ./internal/chibisensei/... -count=1` → `ok`.
  - Delete any `__pycache__` the runs created inside the repository.

**B4 note (for the B4 writer).** Extend `checkDryRunOutputs` in `internal/chibisensei/trainer/selftest_test.go` at the line `// B4: ReadGGUFInfo assertions go here.`:
- Call `chibisensei.ReadGGUFInfo` on every manifest file of kind `gguf_tuned` and `gguf_base`.
- Assert `Version == 3`, `Tensors == 1`, and `Architecture` equal to the profile's `gguf.expected_arch` (`cat.Get(c.profile)`, which is the same profile `buildDryJob` used).
- Add the catalog as a parameter, or look the profile up through `chibisensei.LoadCatalog()` inside the function.

**DOX.** The B2a guide step (owner of `internal/chibisensei/AGENTS.md`) adds the trainer contract. B5 then lists `trainer/` in the guide. Say so in that guide's Ownership section:
- the stage order;
- the `--input` argument;
- the `CHIBI_FAKE_ENV=1` gate;
- that the selftest runs without heavy imports;
- that the dry run fails when it imports torch;
- the verification command `go test ./internal/chibisensei/trainer/ -count=1`.

##### What the first real run will tell us

Each line names the unverified assumption and the log line or status field that answers it. "`info x`" means an `@@chibi` line with `t:"info"` and `kind:"x"`; `d.` is `status.discovery`.

- **Kaggle granted a T4 and honours `CUDA_VISIBLE_DEVICES=0`:** `info gpu` (`name`, `capability`, `vram_gb`, `vram_gib`, `visible`, `host_gpus`), `d.nvidia_smi`, `status.gpu`. A P100 fails at `env_check` with `env_unsupported_gpu`.
- **Internet on (phone verification) and HF reachable anonymously:** `info network` per host; `info hf_repo` / `d.hf_repo` (`sha`, `gated`). Failures: `env_no_internet` or `base_download_failed`.
- **Pinned revisions still exist and are ungated:** `d.hf_repo.sha` equals the pin; `warn unpinned_revision` appears only while a profile still says `main`.
- **Preinstalled versions on Kaggle (Python 3.13, transformers ≥ 5, torch, no peft):** `info versions`, `status.versions`, `d.versions_extra`.
- **Lock pins:** `warn unpinned_dependencies` and `warn unpinned_dependency` give the versions to write into `requirements.lock`.
- **Mount layout of the dataset:** `info data.marker_dir`. When the marker is missing, `d.input_tree` lists what was mounted.
- **Which class loads each base, and in which dtype:**
  - `info base_loaded.cls`, `d.base_class`, `d.base_class_errors`;
  - `d.dtype_loaded` (Gemma's bfloat16 config) against `d.dtype_now` (must be all float32);
  - `d.has_non_text_modules` is true if Qwen3.5 loaded its vision tower;
  - `d.base_config`.
- **Real `nn.Linear` names (DeltaNet, LFM conv):** `info linear_leaves` / `d.linear_leaves` and `d.linear_examples`. The resolved LoRA targets are in `d.lora_targets`, with `warn lora_target_missing` / `lora_targets_fallback`. `d.lora_wrapped_modules` and `d.trainable_params` give the counts.
- **Chat template, BOS handling and masking:**
  - `info template_check` (`starts_with_bos`, `bos_count`, `prompt_tail`, `answer_head`, `sequence_end`);
  - Qwen's `prompt_tail` should end with the empty think block;
  - Gemma's `bos_count` should be 1;
  - `info tokenizer` gives the stop ids.
- **The chunked loss equals the model loss (Gemma softcap, the hybrid models):** `info loss_path_check` (`rel_diff`), `d.loss_path`.
- **fp32 memory on a T4 (LFM 1.2B at seq 2048):**
  - `info memory_probe` / `d.memory_probe` (`peak_vram_gb` per batch size);
  - `warn batch_reduced`, `warn oom_step_skipped`;
  - `d.peak_vram_gb`, `d.vram_after_load_gb`.
- **Gradient checkpointing works with each architecture:** `d.gradient_checkpointing`, `warn checkpointing_unavailable`.
- **Training throughput (Qwen3.5 on the plain PyTorch path without `fla`):** `tok_s` on every train metric; `d.train_tok_s`, `d.train_seconds`. These replace `t4_tokens_per_second`.
- **Generation works and its speed:** `info first_answer`, the `eval` lines (`tok_s`, `answers`), `d.gen_tok_s_base`, `d.gen_tok_s_tuned`. These replace `t4_gen_tokens_per_second`. `warn generate_fallback` shows that `model.generate` failed for the architecture.
- **Time budget realism:**
  - `d.stage_seconds` per stage;
  - `info train_plan` (`train_window_s`, `post_reserve_s`);
  - `warn time_budget`, `warn eval_samples_shortened`, `warn base_gguf_skipped`;
  - `d.total_s` against `budget.session_timeout_s`.
- **llama.cpp tarball pin:** `info llama_cpp.sha256` / `d.llama_cpp_sha256`, the value for `requirements.lock`, with `warn llama_cpp_unpinned`. `d.llama_cpp_layout` confirms the `conversion/` layout.
- **Converter in the notebook environment (transformers 5 against its 4.57.6 pin) or in the pinned one:**
  - `info converter_help` per environment and `d.converter_env` / `d.converter_env_detail` (`system`, or `pinned` with `venv` or `target`);
  - `info pinned_env_requirements` (numpy relaxed on 3.13);
  - `warn converter_failed` with its tail, `d.converter_tail`.
- **b9994 converts each base and each tuned model, and the arch strings `gemma3`, `qwen35`, `lfm2`:**
  - `info converter_run` (label `base` / `vocab_probe` / `tuned`, `flags`, `seconds`);
  - `info gguf` / `d.gguf.base` and `d.gguf.tuned` (`reader_arch`, `reader_tensors`, `types`, `bytes`);
  - `gguf_arch_mismatch` names the real arch.
- **What the merged Qwen3.5 checkpoint looks like, and whether `--no-mtp` was needed:** `info merged_model` / `d.merged` (`architectures`, `model_type`, `mtp_tensors`, `tensors`, `tokenizer_files`), and the `flags` of the tuned `converter_run`.
- **Real GGUF sizes (`gguf.estimated_mb`):** `d.gguf.tuned.bytes` and `d.gguf.base.bytes`.
- **Outputs survive `COMPLETE` and download by flat name:** the B4/coordinator downloads of the manifest files. `info output` lines list what the script left in `/kaggle/working`.
- **Kaggle stopped the session (timeout or user):** `status.code` is `interrupted`; the `done` line still appears when SIGTERM arrived before SIGKILL.

---

## Chibi Sensei P2 backend plan, part 4 (B3)

### Contract changes

1. **New files beyond the contract's B3 list** (one responsibility per file): `runstore.go` (run, event, metric, artifact and report SQL), `consents.go` (consents and settings), `quota.go` (quota parser). The contract puts the store methods in `runs.go`; `runs.go` keeps the run model, the state constants and the JSON view. **`schema.go` gains four `CREATE INDEX IF NOT EXISTS` lines**, which D1 allows (`runs(project_id, created)`, `runs(state)`, `run_events(run_id, id)`, `artifacts(run_id, created)`). `schema.go` is not in any batch's file list; B3 claims it.
2. **Additive exported helpers** that B4 and B5 may use:
   - runs and steps: `RunStates()`, `BackendKaggle`, `LiveYes/LiveNo/LiveUnknown`, step-name constants (`StepUploadPrefix`, `StepCreateDataset`, `StepSaveKernel`, `StepDownloadPrefix`, `StepDeleteDataset`, `StepDeleteKernel`), `NewRunEvent`, `RunView`, `(*RunRemote).Step/PutStep`, `ListRunIDs`;
   - slugs and URLs: `DatasetSlug`, `NotebookSlug`, `KaggleRef`, `KaggleNotebookURL`, `KaggleDatasetURL`;
   - artifacts: `ArtifactID`, `ArtifactKinds`;
   - Kaggle driver helpers: `KaggleRequestID`, `NewKaggleRequest`, `IsKaggleNotFound`, `IsKaggleUncertain`, the `KaggleJob*` status constants, `KaggleJobActive`, `KaggleJobTerminal`, `KaggleProjectTitle`, `SettingKaggleProjectID`;
   - preflight: `KaggleReachable`, `(Preflight).FixFlags`, `PreflightFlagPatch`, `ErrUnknownPreflightFlag`, `ParseGPUQuota`.
3. **`UpdateRun` with `ev.Kind == ""` writes no event.** This lets an update stay silent (for example the log sequence number). The contract says "appends one run_events row"; that still holds whenever the caller passes an event.
4. **Times in `Run`:** `deadline`, `remote_started` and `finished` are `""` when absent, never null. The contract only shows `"…"`.
5. **`Report` details** (the contract left them open):
   - `loss.base` and `loss.tuned`, and the same keys under `perplexity`, may each be null. The whole object is null when both values are missing.
   - `label_accuracy.confusion.base` and `.tuned` are `len(labels)` rows (the expected label) by `len(labels)+1` columns (the predicted label). The **last column counts answers that match no label** (the "other" bucket), so the contract's 2×2 example becomes 2×3.
   - A sample whose expected answer is no project label is left out of `label_accuracy` and of its `n`.
   - `run_id` is left empty by `ScoreReport`; B4 sets it before `SaveReport`.
6. **`GET /runs/{id}/metrics?since_step=N` is inclusive** (`step >= N`). A client that already holds step N asks for `since_step=N` and replaces points with the same `(step, kind)`. Without this, an `eval` point that arrives later at the same step as a `train` point would be lost. A point whose loss is NaN or infinite is not stored, because JSON cannot carry it.
7. **`DeleteProject` refuses a project with a non-terminal run** and returns `ErrRunActive`; it does not cancel first. Reasons:
   - A cancel needs the orchestrator and Kaggle calls, and the store's transaction cannot make them.
   - A remote cancel normally fails, so the notebook keeps running. The run must keep tracking until a terminal state so it can download and clean up.
   - Deleting the rows under a running orchestrator would orphan the Kaggle dataset and notebook without any leftover record.

   The user cancels, waits for the terminal state, then deletes. **B5** maps `ErrRunActive` → 409 `run_active` and `ErrRunNotTerminal` → 409 `run_not_terminal` in `chibiDomainError` (`internal/server/chibi_sensei_service.go:353`). Until then the error surfaces as 500 `storage_failed`, but no run can exist before B4 and B5 land.
8. **File hook for B4** (new name in B4's API): `func RemoveRunFiles(dataDir, runID string) error` in `artifacts.go`. It removes `RunDir(dataDir, runID)` and `ModelDir(dataDir, runID)`; a missing directory is not an error, and it never touches `BaseDir`. B5 calls it after `DeleteRun` succeeds. For a project delete, B5 calls `ListRunIDs` before `DeleteProject`, then `RemoveRunFiles` for each ID after the delete succeeds. Rows go first, so a failed file removal leaves only orphan files, which are logged.
9. **Assumptions about B2's types** (B3 compiles against them; each is used in exactly one helper, so a different B2 name is a one-line fix):
   - `Status.Train.EvalLossBase` and `Status.Train.EvalLossTuned` are `*float64` (null when the stage did not run): helper `statusLosses` in `report.go`.
   - `EvalSample.Expected`, `.BaseOutput`, `.TunedOutput` are `string`: helper `sampleAnswers` in `report.go`.
   - `Metric` has `Step int`, `Kind string`, `Epoch`, `Loss`, `LR`, `GradNorm`, `TokS float64`, `TS string`.
   - `Run12(runID string) string` exists (snapshot.go).
   - The fake's output builders write `chibi-status.json` with exactly the spec's keys plus `discovery` and `train.truncated_rows`, and eval samples with an **integer** `id`. If B2's parsers reject unknown fields or expect a string `id`, adjust `fakeStatusJSON` / `fakeEvalSamplesJSONL`.
10. **`EnsureProject` persistence**: B5's adapter keeps the project id in the Chibi setting `kaggle_project_id` (`SettingKaggleProjectID`, already named in the spec's `settings` comment). It recreates the project when the stored id no longer resolves.
11. **Artifact JSON** hides `run_id`, `project_id` and `rel_path` (`json:"-"`). `name` is derived from the base name of `rel_path` (there is no `name` column, per D1). `rel_path` is slash-separated and relative to `<data>/chibi_sensei`.
12. **Preflight item arguments, and items that were skipped**:
    - Item args:
      - `run_active`: `{run_id, project_id}` when it fails;
      - `kaggle_machine_shape`: `{shape}`;
      - `kaggle_connection`: `{account}`, or `{skipped:true}`;
      - `kaggle_timeout`: `{minutes, needed}`;
      - `kaggle_quota`: `{hours, needed}`, `{unknown:true}` or `{skipped:true}`;
      - `kaggle_slot`: `{jobs, unknown}` or `{skipped:true}`;
      - `kaggle_transfer`: `{needed_mb, max_transfer_mb, free_mb}`;
      - `disk_space`: `{needed_mb, free_mb, cap_free_mb}`;
      - `license_consent`: `{license_id, license_version}`.
    - Skipped items:
      - A quota that was not read (because an earlier Kaggle item failed) is reported as level `warn`, not OK. The earlier item already blocks.
      - A compute-job list that could not be read reports `kaggle_slot` as `block`, not OK.

### File map

| File | Batch | Responsibility |
|---|---|---|
| `internal/chibisensei/schema.go` | B3 | four run indexes (D1) |
| `internal/chibisensei/runs.go` | B3 | run states, `Run`, `RunRemote` and sub-types, JSON view (`RunView`, `RunSummary`), slugs, URLs, `Artifact`, artifact kinds and IDs, errors |
| `internal/chibisensei/runstore.go` | B3 | `CreateRun` (one active run), `GetRun`, `ListRuns`, `ActiveRuns`, `ListRunIDs`, `UpdateRun`, `DeleteRun`, run events (5,000 cap), metrics, artifacts, reports |
| `internal/chibisensei/consents.go` | B3 | `Consent`, `RecordConsent`, `HasConsent`, `ListConsents`, `GetSetting`, `SetSetting` |
| `internal/chibisensei/projects.go` | B3 | `steps.training` from the latest run state in the summary query; `DeleteProject` run guard |
| `internal/chibisensei/plantoken.go` | B3 | `PlanTokens`, `PlanTicket`, `PlanHash` |
| `internal/chibisensei/quota.go` | B3 | `ParseGPUQuota` (provisional, tolerant) |
| `internal/chibisensei/preflight.go` | B3 | `PreflightInput`, `PreflightItem`, `Preflight`, `EvaluatePreflight`, `KaggleReachable`, `PreflightFlags`, `PreflightFlagPatch` |
| `internal/chibisensei/report.go` | B3 | `Report`, `ScoreReport` |
| `internal/chibisensei/driver.go` | B3 | `KaggleDriver` interface, request IDs, request builder, Kaggle job-status helpers |
| `internal/chibisensei/fakekaggle_test.go` | B3 | `fakeKaggle` (in-memory driver), output and log builders, self-tests |
| `internal/chibisensei/runs_test.go` | B3 | run store, JSON shape, events, metrics, artifacts, reports, consents, settings, steps, delete guards |
| `internal/chibisensei/plantoken_test.go`, `preflight_test.go`, `report_test.go` | B3 | token, preflight, quota, flag patch, request builder and report tests |

---

#### Task B3: Run domain (store, consent, plan token, preflight, report, driver and fake)

Work in the worktree only. Scratch files go to `C:\disposable\chibi-sensei\`. Go commands run in Git Bash from the worktree root. Do not commit, stage, stash or switch branches. Write real UTF-8 characters into source files, never `\u` escapes. B1 and B2 must have landed: this task uses `config.KaggleMachineShapes`, `Run12`, `Status`, `EvalSample` and `Metric`.

**Files**
- Create:
  - `internal/chibisensei/runs.go`, `runstore.go`, `consents.go`, `plantoken.go`, `quota.go`, `preflight.go`, `report.go`, `driver.go`;
  - `internal/chibisensei/fakekaggle_test.go`, `runs_test.go`, `plantoken_test.go`, `preflight_test.go`, `report_test.go`.
- Modify:
  - `internal/chibisensei/schema.go:56-57` (append after the `base_gguf` statement, inside the string);
  - `internal/chibisensei/projects.go:149` (`CreateProject`), `:158-181` (`scanProject`), `:183-184` (`projectSelect`), `:221-237` (`computeSteps`), `:401-403` (`DeleteProject`, after the confirm-name check).

**Existing symbols modified:** `schemaSQL` (const), `projectSelect` (const), `scanProject`, `computeSteps`, `CreateProject`, `DeleteProject`.

**Steps**

- [ ] 1. `schema.go`: append these lines at the end of `schemaSQL`. Put them after `PRIMARY KEY(profile_id, base_revision, converter));` and before the closing backtick. `schemaVersion` stays 1.

```sql
CREATE INDEX IF NOT EXISTS runs_project ON runs(project_id, created);
CREATE INDEX IF NOT EXISTS runs_state ON runs(state);
CREATE INDEX IF NOT EXISTS run_events_run ON run_events(run_id, id);
CREATE INDEX IF NOT EXISTS artifacts_run ON artifacts(run_id, created);
```

- [ ] 2. Create `internal/chibisensei/runs.go`.

Column mapping (D1, no schema change). `runs` columns from `schema.go:47-49` and the Go field each one holds:

| Column | Go field | Notes |
|---|---|---|
| `id` | `Run.ID` | UUID |
| `project_id` | `Run.ProjectID` | |
| `backend` | `Run.Backend` | `"kaggle"` |
| `state` | `Run.State` | one of `runStates` |
| `step_attempt` | `Run.StepAttempt` | attempt of the current state; B4 resets it on a transition |
| `profile_id` | `Run.ProfileID` | |
| `profile_sha` | `Run.ProfileSHA` | |
| `preset` | `Run.Preset` | |
| `hyper_json` | `Run.Hyper` | the plan's `Hyper` |
| `job_json` | `Run.JobJSON` | the job spec bytes (`JobSpec.JSON()`); `''` until packaging |
| `data_sha` | `Run.DataSHA` | `Snapshot.DataSHA` |
| `remote_json` | `Run.Remote` | `RunRemote`, everything without a column |
| `deadline` | `Run.Deadline` | RFC 3339; `''` until remote start |
| `cancel_requested` | `Run.CancelRequested` | 0/1 |
| `error_code`, `error` | `Run.ErrorCode`, `Run.Error` | public text only |
| `leftovers_json` | `Run.Leftovers` | never null, `[]` |
| `created`, `updated` | `Run.Created`, `Run.Updated` | store stamps |

```go
package chibisensei

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Run states in lifecycle order (spec "Run state machine").
const (
	RunQueued          = "queued"
	RunPackaging       = "packaging"
	RunUploading       = "uploading"
	RunCreatingDataset = "creating_dataset"
	RunWaitingDataset  = "waiting_dataset"
	RunStarting        = "starting"
	RunRunning         = "running"
	RunDownloading     = "downloading"
	RunVerifying       = "verifying"
	RunCleaning        = "cleaning"
	RunSucceeded       = "succeeded"
	RunFailed          = "failed"
	RunCancelled       = "cancelled"
)

var runStates = []string{RunQueued, RunPackaging, RunUploading, RunCreatingDataset, RunWaitingDataset, RunStarting,
	RunRunning, RunDownloading, RunVerifying, RunCleaning, RunSucceeded, RunFailed, RunCancelled}

// RunStates lists every run state in lifecycle order (GET /meta "run_states").
func RunStates() []string { return append([]string(nil), runStates...) }

// RunTerminal reports whether state is succeeded, failed or cancelled.
func RunTerminal(state string) bool {
	return state == RunSucceeded || state == RunFailed || state == RunCancelled
}

func validRunState(state string) bool {
	for _, s := range runStates {
		if s == state {
			return true
		}
	}
	return false
}

// terminalStatesSQL is the SQL list of the terminal states.
const terminalStatesSQL = `('succeeded','failed','cancelled')`

// BackendKaggle is the only training backend of P2.
const BackendKaggle = "kaggle"

// Values of RunRemote.Live and the API field "live".
const (
	LiveYes     = "yes"
	LiveNo      = "no"
	LiveUnknown = "unknown"
)

// Kaggle step names. A step name plus its attempt gives the request ID (KaggleRequestID).
const (
	StepUploadPrefix   = "upload_"   // + split: upload_train, upload_val, upload_eval, upload_marker
	StepCreateDataset  = "create_dataset"
	StepSaveKernel     = "save_kernel"
	StepDownloadPrefix = "download_" // + kernel output file name, e.g. download_chibi-status.json
	StepDeleteDataset  = "delete_dataset"
	StepDeleteKernel   = "delete_kernel"
)

var (
	ErrRunActive      = errors.New("chibisensei: a training run is active")
	ErrRunNotTerminal = errors.New("chibisensei: the training run has not ended")
)

// maxRunEvents caps run_events per run; a variable so the cap test can lower it.
var maxRunEvents = 5000

// Leftover is a Kaggle resource a run left behind (cleanup off, delete not granted, or a failed delete).
type Leftover struct {
	Kind string `json:"kind"` // dataset | notebook
	Ref  string `json:"ref"`  // owner/slug
	URL  string `json:"url"`
}

// RunStage is the latest trainer stage seen in the log or the status file.
type RunStage struct {
	Name     string  `json:"name"`
	State    string  `json:"state"` // start | done | failed
	Step     int     `json:"step"`
	MaxSteps int     `json:"max_steps"`
	Epoch    float64 `json:"epoch"`
	EtaS     int     `json:"eta_s"`
}

// RunVersions are the software versions the trainer measured.
type RunVersions struct {
	Python       string `json:"python"`
	Torch        string `json:"torch"`
	Transformers string `json:"transformers"`
	PEFT         string `json:"peft"`
	LlamaCpp     string `json:"llama_cpp"`
}

// RunResult summarizes chibi-status.json once it was downloaded.
type RunResult struct {
	Steps         int          `json:"steps"`
	Epochs        float64      `json:"epochs"`
	TrainLoss     *float64     `json:"train_loss"`
	EvalLossBase  *float64     `json:"eval_loss_base"`
	EvalLossTuned *float64     `json:"eval_loss_tuned"`
	StopReason    string       `json:"stop_reason"`
	GPU           string       `json:"gpu"`
	Versions      *RunVersions `json:"versions,omitempty"` // always present in Run, dropped in RunSummary
}

// StagedFile is one dataset file staged in the Kaggle project (D12: the name inside the dataset equals Name).
type StagedFile struct {
	Split   string `json:"split"` // train | val | eval | marker
	Name    string `json:"name"`  // r<run12>-train.jsonl … r<run12>-marker.json
	SHA256  string `json:"sha256"`
	Bytes   int64  `json:"bytes"`
	Rows    int    `json:"rows"`
	Removed bool   `json:"removed"` // the staged copy was removed after create_dataset completed
}

// KaggleStep is the receipt of one Kaggle mutation of the run.
type KaggleStep struct {
	Step      string `json:"step"`                 // see the Step* constants
	Attempt   int    `json:"attempt"`              // attempt that produced RequestID (0-based)
	RequestID string `json:"request_id"`           // KaggleRequestID(run, step, attempt)
	JobID     string `json:"job_id"`               // "" until Execute admitted the request; then equal to RequestID
	Status    string `json:"status"`               // last Kaggle job status seen
	UploadRef string `json:"upload_ref,omitempty"` // upload steps: "upload:<job id>" from the completed receipt
	File      string `json:"file,omitempty"`       // upload: staged project file; download: kernel output path
	Error     string `json:"error,omitempty"`      // Job.Error (public text of the Kaggle package)
	Updated   string `json:"updated"`
}

// RunOutput is one file of the downloaded manifest.
type RunOutput struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
	Copied bool   `json:"copied"` // verified and copied into Chibi storage
}

// RunRemote holds everything of a run that has no column (runs.remote_json). Every field B4 needs to
// resume a state after a restart lives here.
type RunRemote struct {
	// Plan facts captured at submit.
	ProfileStatus         string    `json:"profile_status"`
	Method                string    `json:"method"`
	Split                 PlanSplit `json:"split"`
	EstimateMinutes       int       `json:"estimate_minutes"`
	SessionTimeoutMinutes int       `json:"session_timeout_minutes"`
	MaxTrainSeconds       int       `json:"max_train_seconds"`
	DataRev               int64     `json:"data_rev"`
	PlanHash              string    `json:"plan_hash"`
	MachineShape          string    `json:"machine_shape"`
	DeleteRemoteAfterRun  bool      `json:"delete_remote_after_run"` // the value the user confirmed

	// Destinations.
	Owner         string       `json:"owner"`
	DatasetSlug   string       `json:"dataset_slug"`   // without owner: chibi-data-<run12>
	NotebookSlug  string       `json:"notebook_slug"`  // without owner: chibi-<project8>-<run12>
	KaggleProject string       `json:"kaggle_project"` // local Kaggle project id (staging and downloads)
	Marker        string       `json:"marker"`         // marker file name (job.json data.marker)
	Staged        []StagedFile `json:"staged"`
	SnapshotBytes int64        `json:"snapshot_bytes"`

	// Kaggle receipts.
	Jobs          []KaggleStep `json:"jobs"`
	KernelVersion int          `json:"kernel_version"` // save_kernel versionNumber; 0 = unknown
	KernelID      int64        `json:"kernel_id"`
	RemoteStatus  string       `json:"remote_status"` // last Kaggle session status (upper case)
	Untracked     bool         `json:"untracked"`     // running without a usable save_kernel receipt (status reads only)

	// Timeline (RFC 3339; "" = not yet).
	Submitted          string `json:"submitted"`            // save_kernel admitted
	RemoteStarted      string `json:"remote_started"`       // first remote_running observation; the deadline is computed from it
	Finished           string `json:"finished"`             // terminal state reached
	DatasetWaitStarted string `json:"dataset_wait_started"` // waiting_dataset entered (15-minute cap)
	LastPolled         string `json:"last_polled"`          // last status poll
	LastLogRead        string `json:"last_log_read"`        // last ReadKernelLog call

	// Live progress.
	Stage      *RunStage `json:"stage"`
	Live       string    `json:"live"`         // yes | no | unknown
	LastLogSeq int64     `json:"last_log_seq"` // highest @@chibi seq stored

	// Outcome.
	Result      *RunResult  `json:"result"`
	WarningCode string      `json:"warning_code"`
	Outputs     []RunOutput `json:"outputs"`

	// Cancel.
	CancelRemoteStopped bool   `json:"cancel_remote_stopped"`
	CancelError         string `json:"cancel_error"`

	// Attempts counts tries per state or step name (for example "downloading": at most 3).
	Attempts map[string]int `json:"attempts"`
}

// Step returns the receipt of a step, or nil.
func (r *RunRemote) Step(name string) *KaggleStep {
	for i := range r.Jobs {
		if r.Jobs[i].Step == name {
			return &r.Jobs[i]
		}
	}
	return nil
}

// PutStep replaces the receipt with the same Step or appends it.
func (r *RunRemote) PutStep(s KaggleStep) {
	if cur := r.Step(s.Step); cur != nil {
		*cur = s
		return
	}
	r.Jobs = append(r.Jobs, s)
}

// Run is one training run. Its JSON form is the API shape "Run" (see MarshalJSON).
type Run struct {
	ID              string
	ProjectID       string
	Backend         string
	State           string
	StepAttempt     int
	ProfileID       string
	ProfileSHA      string
	Preset          string
	Hyper           Hyper
	JobJSON         []byte
	DataSHA         string
	Remote          RunRemote
	Deadline        string
	CancelRequested bool
	ErrorCode       string
	Error           string
	Leftovers       []Leftover
	Created         string
	Updated         string
}

// RunEvent is one row of run_events. Kind: state | stage | warn | error | kaggle | log.
type RunEvent struct {
	ID   int64           `json:"id"`
	TS   string          `json:"ts"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

// NewRunEvent marshals data (nil or an encoding error gives {}).
func NewRunEvent(kind string, data any) RunEvent {
	b, err := json.Marshal(data)
	if err != nil || data == nil {
		b = []byte("{}")
	}
	return RunEvent{Kind: kind, Data: b}
}

// KaggleJobRef is one entry of remote.kaggle_jobs.
type KaggleJobRef struct {
	Step  string `json:"step"`
	JobID string `json:"job_id"`
}

// RunCancelView is the "cancel" object, present once cancel_requested is true.
type RunCancelView struct {
	RemoteStopped    bool   `json:"remote_stopped"`
	StopURL          string `json:"stop_url"` // "" before the notebook was submitted
	TimeLimitMinutes int    `json:"time_limit_minutes"`
}

// RunRemoteView is the "remote" object.
type RunRemoteView struct {
	Owner       string          `json:"owner"`
	Dataset     string          `json:"dataset"`  // owner/slug or ""
	Notebook    string          `json:"notebook"` // owner/slug or ""
	NotebookURL string          `json:"notebook_url"`
	Status      string          `json:"status"`
	Untracked   bool            `json:"untracked"`
	KaggleJobs  *[]KaggleJobRef `json:"kaggle_jobs,omitempty"` // Run only
}

// RunView is the API form of a run.
type RunView struct {
	ID                    string         `json:"id"`
	ProjectID             string         `json:"project_id"`
	Backend               string         `json:"backend"`
	State                 string         `json:"state"`
	ProfileID             string         `json:"profile_id"`
	ProfileStatus         string         `json:"profile_status"`
	Preset                string         `json:"preset"`
	Method                string         `json:"method"`
	Hyper                 *Hyper         `json:"hyper,omitempty"` // Run only
	Split                 PlanSplit      `json:"split"`
	Stage                 *RunStage      `json:"stage"`
	Live                  string         `json:"live"`
	EstimateMinutes       int            `json:"estimate_minutes"`
	SessionTimeoutMinutes int            `json:"session_timeout_minutes"`
	Deadline              string         `json:"deadline"`
	RemoteStarted         string         `json:"remote_started"`
	Finished              string         `json:"finished"`
	CancelRequested       bool           `json:"cancel_requested"`
	Cancel                *RunCancelView `json:"cancel"`
	Remote                RunRemoteView  `json:"remote"`
	ErrorCode             string         `json:"error_code"`
	Error                 string         `json:"error"`
	WarningCode           string         `json:"warning_code"`
	Result                *RunResult     `json:"result"`
	Leftovers             []Leftover     `json:"leftovers"`
	Created               string         `json:"created"`
	Updated               string         `json:"updated"`
}

// RunSummary is the list form: a RunView without hyper, remote.kaggle_jobs and result.versions.
type RunSummary RunView

func (r Run) view(full bool) RunView {
	rem := r.Remote
	v := RunView{ID: r.ID, ProjectID: r.ProjectID, Backend: r.Backend, State: r.State, ProfileID: r.ProfileID,
		ProfileStatus: rem.ProfileStatus, Preset: r.Preset, Method: rem.Method, Split: rem.Split, Live: rem.Live,
		EstimateMinutes: rem.EstimateMinutes, SessionTimeoutMinutes: rem.SessionTimeoutMinutes, Deadline: r.Deadline,
		RemoteStarted: rem.RemoteStarted, Finished: rem.Finished, CancelRequested: r.CancelRequested,
		ErrorCode: r.ErrorCode, Error: r.Error, WarningCode: rem.WarningCode, Leftovers: r.Leftovers,
		Created: r.Created, Updated: r.Updated}
	if v.Live == "" {
		v.Live = LiveUnknown
	}
	if v.Leftovers == nil {
		v.Leftovers = []Leftover{}
	}
	if rem.Stage != nil {
		st := *rem.Stage
		v.Stage = &st
	}
	dataset, notebook := KaggleRef(rem.Owner, rem.DatasetSlug), KaggleRef(rem.Owner, rem.NotebookSlug)
	v.Remote = RunRemoteView{Owner: rem.Owner, Dataset: dataset, Notebook: notebook, Status: rem.RemoteStatus,
		Untracked: rem.Untracked}
	if notebook != "" {
		v.Remote.NotebookURL = KaggleNotebookURL(notebook)
	}
	if r.CancelRequested {
		c := &RunCancelView{RemoteStopped: rem.CancelRemoteStopped, TimeLimitMinutes: rem.SessionTimeoutMinutes}
		if notebook != "" && (rem.Submitted != "" || rem.Untracked) {
			c.StopURL = KaggleNotebookURL(notebook)
		}
		v.Cancel = c
	}
	if rem.Result != nil {
		res := *rem.Result
		if full && res.Versions == nil {
			res.Versions = &RunVersions{}
		}
		if !full {
			res.Versions = nil
		}
		v.Result = &res
	}
	if full {
		h := r.Hyper
		v.Hyper = &h
		jobs := []KaggleJobRef{}
		for _, s := range rem.Jobs {
			if s.JobID != "" {
				jobs = append(jobs, KaggleJobRef{Step: s.Step, JobID: s.JobID})
			}
		}
		v.Remote.KaggleJobs = &jobs
	}
	return v
}

// View is the full API form ("Run").
func (r Run) View() RunView { return r.view(true) }

// Summary is the list form ("RunSummary").
func (r Run) Summary() RunSummary { return RunSummary(r.view(false)) }

// MarshalJSON writes the API shape "Run". Run has no UnmarshalJSON; decode API answers into RunView.
func (r Run) MarshalJSON() ([]byte, error) { return json.Marshal(r.View()) }

// DatasetSlug is the Kaggle dataset slug of a run (without owner).
func DatasetSlug(runID string) string { return "chibi-data-" + Run12(runID) }

// NotebookSlug is the Kaggle notebook slug of a run: chibi-<first 8 hex of the project>-<run12>.
func NotebookSlug(projectID, runID string) string {
	p := strings.ReplaceAll(projectID, "-", "")
	if len(p) > 8 {
		p = p[:8]
	}
	return "chibi-" + p + "-" + Run12(runID)
}

// KaggleRef joins owner and slug; "" when either is empty.
func KaggleRef(owner, slug string) string {
	if owner == "" || slug == "" {
		return ""
	}
	return owner + "/" + slug
}

// KaggleNotebookURL and KaggleDatasetURL are the public Kaggle pages of an owner/slug reference.
func KaggleNotebookURL(ref string) string { return "https://www.kaggle.com/code/" + ref }
func KaggleDatasetURL(ref string) string  { return "https://www.kaggle.com/datasets/" + ref }

// Artifact is one file a run registered under <data>/chibi_sensei.
type Artifact struct {
	ID        string `json:"id"`
	RunID     string `json:"-"`
	ProjectID string `json:"-"`
	Kind      string `json:"kind"`
	Name      string `json:"name"` // base name of RelPath; not stored
	RelPath   string `json:"-"`    // slash-separated, relative to <data>/chibi_sensei
	Bytes     int64  `json:"bytes"`
	SHA256    string `json:"sha256"`
	Created   string `json:"created"`
}

// ArtifactKinds are the manifest kinds plus log_tail.
var ArtifactKinds = []string{"gguf_tuned", "gguf_base", "adapter", "adapter_config", "metrics", "eval_samples", "status", "log_tail"}

func validArtifactKind(k string) bool {
	for _, x := range ArtifactKinds {
		if x == k {
			return true
		}
	}
	return false
}

// ArtifactID is the deterministic artifact ID of a run and kind (a UUID v5, so ValidID accepts it and a redone
// registration after a restart replaces the row instead of duplicating it).
func ArtifactID(runID, kind string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("aurago:chibi-sensei:artifact:"+runID+":"+kind)).String()
}
```

- [ ] 3. Create `internal/chibisensei/runstore.go`.

```go
package chibisensei

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"strings"
)

// jsonText marshals without HTML escaping and fails on values JSON cannot carry (NaN, Inf).
func jsonText(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

type runVals struct {
	hyper, job, remote, leftovers string
	cancel                        int
}

func runValues(r Run) (runVals, error) {
	var v runVals
	var err error
	if v.hyper, err = jsonText(r.Hyper); err != nil {
		return v, fmt.Errorf("encode hyper: %w", err)
	}
	if v.remote, err = jsonText(r.Remote); err != nil {
		return v, fmt.Errorf("encode remote: %w", err)
	}
	left := r.Leftovers
	if left == nil {
		left = []Leftover{}
	}
	if v.leftovers, err = jsonText(left); err != nil {
		return v, fmt.Errorf("encode leftovers: %w", err)
	}
	if len(r.JobJSON) > 0 && !json.Valid(r.JobJSON) {
		return v, errors.New("job_json is not valid JSON")
	}
	v.job = string(r.JobJSON)
	if r.CancelRequested {
		v.cancel = 1
	}
	return v, nil
}

const runCols = `id, COALESCE(project_id,''), COALESCE(backend,''), COALESCE(state,''), COALESCE(step_attempt,0),
	COALESCE(profile_id,''), COALESCE(profile_sha,''), COALESCE(preset,''), COALESCE(hyper_json,'{}'), COALESCE(job_json,''),
	COALESCE(data_sha,''), COALESCE(remote_json,'{}'), COALESCE(deadline,''), COALESCE(cancel_requested,0),
	COALESCE(error_code,''), COALESCE(error,''), COALESCE(leftovers_json,'[]'), COALESCE(created,''), COALESCE(updated,'')`

func scanRun(sc rowScanner) (Run, error) {
	var r Run
	var hyper, job, remote, leftovers string
	var cancel int
	if err := sc.Scan(&r.ID, &r.ProjectID, &r.Backend, &r.State, &r.StepAttempt, &r.ProfileID, &r.ProfileSHA, &r.Preset,
		&hyper, &job, &r.DataSHA, &remote, &r.Deadline, &cancel, &r.ErrorCode, &r.Error, &leftovers, &r.Created, &r.Updated); err != nil {
		return Run{}, err
	}
	if err := json.Unmarshal([]byte(hyper), &r.Hyper); err != nil {
		return Run{}, fmt.Errorf("decode run %s hyper: %w", r.ID, err)
	}
	if job != "" {
		r.JobJSON = []byte(job)
	}
	if err := json.Unmarshal([]byte(remote), &r.Remote); err != nil {
		return Run{}, fmt.Errorf("decode run %s remote: %w", r.ID, err)
	}
	if err := json.Unmarshal([]byte(leftovers), &r.Leftovers); err != nil {
		return Run{}, fmt.Errorf("decode run %s leftovers: %w", r.ID, err)
	}
	if r.Leftovers == nil {
		r.Leftovers = []Leftover{}
	}
	if r.Remote.Live == "" {
		r.Remote.Live = LiveUnknown
	}
	r.CancelRequested = cancel != 0
	return r, nil
}

func loadRun(ctx context.Context, q queryer, id string) (Run, error) {
	r, err := scanRun(q.QueryRowContext(ctx, `SELECT `+runCols+` FROM runs WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	return r, err
}

// CreateRun inserts a new run in state queued and its first "state" event. It refuses with ErrRunActive while any
// run of the installation is not terminal (D13) and with ErrNotFound when the project does not exist. The single
// store connection serializes the check and the insert.
func (s *Store) CreateRun(ctx context.Context, r Run) error {
	if !ValidID(r.ID) || !ValidID(r.ProjectID) {
		return errors.New("create run: invalid run or project id")
	}
	if r.State == "" {
		r.State = RunQueued
	}
	if r.State != RunQueued {
		return fmt.Errorf("create run: a new run starts queued, not %q", r.State)
	}
	if r.Backend == "" {
		r.Backend = BackendKaggle
	}
	if r.Remote.Live == "" {
		r.Remote.Live = LiveUnknown
	}
	r.Created = s.stamp()
	r.Updated = r.Created
	vals, err := runValues(r)
	if err != nil {
		return fmt.Errorf("create run: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id=?`, r.ProjectID).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE state NOT IN `+terminalStatesSQL).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrRunActive
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO runs(id,project_id,backend,state,step_attempt,profile_id,profile_sha,preset,
		hyper_json,job_json,data_sha,remote_json,deadline,cancel_requested,error_code,error,leftovers_json,created,updated)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, r.ID, r.ProjectID, r.Backend, r.State, r.StepAttempt, r.ProfileID,
		r.ProfileSHA, r.Preset, vals.hyper, vals.job, r.DataSHA, vals.remote, r.Deadline, vals.cancel, r.ErrorCode, r.Error,
		vals.leftovers, r.Created, r.Updated); err != nil {
		return fmt.Errorf("create run: %w", err)
	}
	if _, err := s.addRunEvent(ctx, tx, r.ID, NewRunEvent("state", map[string]any{"state": RunQueued})); err != nil {
		return err
	}
	return tx.Commit()
}

// GetRun returns one run or ErrNotFound.
func (s *Store) GetRun(ctx context.Context, id string) (Run, error) { return loadRun(ctx, s.db, id) }

func (s *Store) queryRuns(ctx context.Context, query string, args ...any) ([]Run, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+runCols+` FROM runs `+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListRuns lists the runs of a project, newest first; limit defaults to 20 and is capped at 100.
func (s *Store) ListRuns(ctx context.Context, projectID string, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.queryRuns(ctx, `WHERE project_id=? ORDER BY created DESC, rowid DESC LIMIT ?`, projectID, limit)
}

// ActiveRuns lists every non-terminal run, oldest first (the orchestrator resumes them at boot).
func (s *Store) ActiveRuns(ctx context.Context) ([]Run, error) {
	return s.queryRuns(ctx, `WHERE state NOT IN `+terminalStatesSQL+` ORDER BY created, rowid`)
}

// ListRunIDs lists the IDs of every run of a project (the project-delete file hook).
func (s *Store) ListRunIDs(ctx context.Context, projectID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM runs WHERE project_id=? ORDER BY created, rowid`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// UpdateRun loads the run, calls fn, stores it and, when ev.Kind is not empty, appends ev, all in one transaction.
// fn cannot change ID, ProjectID, Backend or Created; an error from fn rolls everything back. It publishes nothing;
// the caller publishes after the commit.
func (s *Store) UpdateRun(ctx context.Context, id string, ev RunEvent, fn func(*Run) error) (Run, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, err
	}
	defer tx.Rollback()
	cur, err := loadRun(ctx, tx, id)
	if err != nil {
		return Run{}, err
	}
	next := cur
	if err := fn(&next); err != nil {
		return Run{}, err
	}
	next.ID, next.ProjectID, next.Backend, next.Created = cur.ID, cur.ProjectID, cur.Backend, cur.Created
	if !validRunState(next.State) {
		return Run{}, fmt.Errorf("update run %s: unknown state %q", id, next.State)
	}
	if next.Leftovers == nil {
		next.Leftovers = []Leftover{}
	}
	if next.Remote.Live == "" {
		next.Remote.Live = LiveUnknown
	}
	next.Updated = s.stamp()
	vals, err := runValues(next)
	if err != nil {
		return Run{}, fmt.Errorf("update run %s: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE runs SET state=?, step_attempt=?, profile_id=?, profile_sha=?, preset=?,
		hyper_json=?, job_json=?, data_sha=?, remote_json=?, deadline=?, cancel_requested=?, error_code=?, error=?,
		leftovers_json=?, updated=? WHERE id=?`, next.State, next.StepAttempt, next.ProfileID, next.ProfileSHA, next.Preset,
		vals.hyper, vals.job, next.DataSHA, vals.remote, next.Deadline, vals.cancel, next.ErrorCode, next.Error,
		vals.leftovers, next.Updated, id); err != nil {
		return Run{}, fmt.Errorf("update run %s: %w", id, err)
	}
	if ev.Kind != "" {
		if _, err := s.addRunEvent(ctx, tx, id, ev); err != nil {
			return Run{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Run{}, err
	}
	return next, nil
}

// DeleteRun deletes a terminal run with its events, metrics, reports and artifact rows. It returns ErrNotFound or
// ErrRunNotTerminal. Files are not touched: the caller then calls RemoveRunFiles (B4, artifacts.go).
func (s *Store) DeleteRun(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(state,'') FROM runs WHERE id=?`, id).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !RunTerminal(state) {
		return ErrRunNotTerminal
	}
	for _, q := range []string{
		`DELETE FROM run_events WHERE run_id=?`,
		`DELETE FROM metrics WHERE run_id=?`,
		`DELETE FROM eval_reports WHERE run_id=?`,
		`DELETE FROM artifacts WHERE run_id=?`,
		`DELETE FROM runs WHERE id=?`,
	} {
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return fmt.Errorf("delete run %s: %w", id, err)
		}
	}
	return tx.Commit()
}

// addRunEvent inserts one event and prunes the run to the newest maxRunEvents events.
func (s *Store) addRunEvent(ctx context.Context, q queryer, runID string, ev RunEvent) (int64, error) {
	if ev.Kind == "" {
		return 0, errors.New("run event: empty kind")
	}
	data := ev.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	if !json.Valid(data) {
		return 0, errors.New("run event: data is not valid JSON")
	}
	ts := ev.TS
	if ts == "" {
		ts = s.stamp()
	}
	res, err := q.ExecContext(ctx, `INSERT INTO run_events(run_id,ts,kind,data_json) VALUES(?,?,?,?)`, runID, ts, ev.Kind, string(data))
	if err != nil {
		return 0, fmt.Errorf("add run event: %w", err)
	}
	id, _ := res.LastInsertId()
	if _, err := q.ExecContext(ctx, `DELETE FROM run_events WHERE run_id=? AND id <= (SELECT id FROM run_events WHERE run_id=?
		ORDER BY id DESC LIMIT 1 OFFSET ?)`, runID, runID, maxRunEvents); err != nil {
		return 0, fmt.Errorf("prune run events: %w", err)
	}
	return id, nil
}

// AddRunEvent appends one event and keeps the newest 5,000 per run.
func (s *Store) AddRunEvent(ctx context.Context, runID string, ev RunEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := s.addRunEvent(ctx, tx, runID, ev); err != nil {
		return err
	}
	return tx.Commit()
}

// ListRunEvents returns the events with id > afterID in id order; limit defaults to 200, at most 1000.
func (s *Store) ListRunEvents(ctx context.Context, runID string, afterID int64, limit int) ([]RunEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(ts,''), COALESCE(kind,''), COALESCE(data_json,'{}')
		FROM run_events WHERE run_id=? AND id>? ORDER BY id LIMIT ?`, runID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RunEvent{}
	for rows.Next() {
		var e RunEvent
		var data string
		if err := rows.Scan(&e.ID, &e.TS, &e.Kind, &data); err != nil {
			return nil, err
		}
		e.Data = json.RawMessage(data)
		out = append(out, e)
	}
	return out, rows.Err()
}

func finiteOr0(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// insertMetrics upserts points; a point with an unknown kind, a negative step or a non-finite loss is skipped.
func insertMetrics(ctx context.Context, tx *sql.Tx, runID string, m []Metric) error {
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO metrics(run_id,step,kind,epoch,loss,lr,grad_norm,tok_s,ts)
		VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(run_id,step,kind) DO UPDATE SET epoch=excluded.epoch, loss=excluded.loss,
		lr=excluded.lr, grad_norm=excluded.grad_norm, tok_s=excluded.tok_s, ts=excluded.ts`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, p := range m {
		if (p.Kind != "train" && p.Kind != "eval") || p.Step < 0 || math.IsNaN(p.Loss) || math.IsInf(p.Loss, 0) {
			continue
		}
		if _, err := stmt.ExecContext(ctx, runID, p.Step, p.Kind, finiteOr0(p.Epoch), p.Loss, finiteOr0(p.LR),
			finiteOr0(p.GradNorm), finiteOr0(p.TokS), p.TS); err != nil {
			return fmt.Errorf("store metric: %w", err)
		}
	}
	return nil
}

// UpsertMetrics adds or replaces live points by (step, kind).
func (s *Store) UpsertMetrics(ctx context.Context, runID string, m []Metric) error {
	if len(m) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertMetrics(ctx, tx, runID, m); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceMetrics replaces every point of the run (chibi-metrics.jsonl after the run).
func (s *Store) ReplaceMetrics(ctx context.Context, runID string, m []Metric) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM metrics WHERE run_id=?`, runID); err != nil {
		return err
	}
	if err := insertMetrics(ctx, tx, runID, m); err != nil {
		return err
	}
	return tx.Commit()
}

const maxListedMetrics = 20000

// ListMetrics returns the points with step >= sinceStep (inclusive) ordered by step and kind, at most 20,000.
func (s *Store) ListMetrics(ctx context.Context, runID string, sinceStep int) ([]Metric, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT step, COALESCE(kind,''), COALESCE(epoch,0), COALESCE(loss,0), COALESCE(lr,0),
		COALESCE(grad_norm,0), COALESCE(tok_s,0), COALESCE(ts,'') FROM metrics WHERE run_id=? AND step>=?
		ORDER BY step, kind LIMIT ?`, runID, sinceStep, maxListedMetrics)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Metric{}
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.Step, &m.Kind, &m.Epoch, &m.Loss, &m.LR, &m.GradNorm, &m.TokS, &m.TS); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func validArtifactPath(rel string) bool {
	return rel != "" && !strings.Contains(rel, `\`) && filepath.IsLocal(filepath.FromSlash(rel))
}

// AddArtifact registers (or, for the same ID, replaces) a file. ID defaults to ArtifactID(RunID, Kind), Created to now.
func (s *Store) AddArtifact(ctx context.Context, a Artifact) error {
	if !validArtifactKind(a.Kind) {
		return fmt.Errorf("add artifact: unknown kind %q", a.Kind)
	}
	if !ValidID(a.RunID) || !ValidID(a.ProjectID) {
		return errors.New("add artifact: invalid run or project id")
	}
	if !validArtifactPath(a.RelPath) {
		return fmt.Errorf("add artifact: invalid path %q", a.RelPath)
	}
	if a.ID == "" {
		a.ID = ArtifactID(a.RunID, a.Kind)
	}
	if a.Created == "" {
		a.Created = s.stamp()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO artifacts(id,run_id,project_id,kind,rel_path,bytes,sha256,created)
		VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET run_id=excluded.run_id, project_id=excluded.project_id,
		kind=excluded.kind, rel_path=excluded.rel_path, bytes=excluded.bytes, sha256=excluded.sha256`,
		a.ID, a.RunID, a.ProjectID, a.Kind, a.RelPath, a.Bytes, a.SHA256, a.Created)
	if err != nil {
		return fmt.Errorf("add artifact: %w", err)
	}
	return nil
}

const artifactCols = `id, COALESCE(run_id,''), COALESCE(project_id,''), COALESCE(kind,''), COALESCE(rel_path,''),
	COALESCE(bytes,0), COALESCE(sha256,''), COALESCE(created,'')`

func scanArtifact(sc rowScanner) (Artifact, error) {
	var a Artifact
	if err := sc.Scan(&a.ID, &a.RunID, &a.ProjectID, &a.Kind, &a.RelPath, &a.Bytes, &a.SHA256, &a.Created); err != nil {
		return Artifact{}, err
	}
	a.Name = path.Base(a.RelPath)
	return a, nil
}

// ListArtifacts lists the artifacts of a run in registration order.
func (s *Store) ListArtifacts(ctx context.Context, runID string) ([]Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+artifactCols+` FROM artifacts WHERE run_id=? ORDER BY created, rowid`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Artifact{}
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetArtifact returns one artifact or ErrNotFound.
func (s *Store) GetArtifact(ctx context.Context, id string) (Artifact, error) {
	a, err := scanArtifact(s.db.QueryRowContext(ctx, `SELECT `+artifactCols+` FROM artifacts WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Artifact{}, ErrNotFound
	}
	return a, err
}

// SaveReport stores (or replaces) the report of a run; kind is auto or judge.
func (s *Store) SaveReport(ctx context.Context, runID, kind string, result any) error {
	if kind != "auto" && kind != "judge" {
		return fmt.Errorf("save report: unknown kind %q", kind)
	}
	body, err := jsonText(result)
	if err != nil {
		return fmt.Errorf("save report: %w", err)
	}
	if body == "null" {
		return errors.New("save report: empty report")
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO eval_reports(run_id,kind,result_json,created) VALUES(?,?,?,?)
		ON CONFLICT(run_id,kind) DO UPDATE SET result_json=excluded.result_json, created=excluded.created`,
		runID, kind, body, s.stamp())
	if err != nil {
		return fmt.Errorf("save report: %w", err)
	}
	return nil
}

// GetReport returns the stored report JSON and whether one exists.
func (s *Store) GetReport(ctx context.Context, runID, kind string) (json.RawMessage, bool, error) {
	var body string
	err := s.db.QueryRowContext(ctx, `SELECT result_json FROM eval_reports WHERE run_id=? AND kind=?`, runID, kind).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return json.RawMessage(body), true, nil
}
```

- [ ] 4. Create `internal/chibisensei/consents.go`.

```go
package chibisensei

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Consent is one accepted base-model license version.
type Consent struct {
	ProfileID      string `json:"profile_id"`
	LicenseID      string `json:"license_id"`
	LicenseVersion string `json:"license_version"`
	AcceptedAt     string `json:"accepted_at"`
}

// RecordConsent records the consent once; a repeat returns the original row (idempotent).
func (s *Store) RecordConsent(ctx context.Context, profileID, licenseID, version string) (Consent, error) {
	if profileID == "" || licenseID == "" || version == "" {
		return Consent{}, errors.New("record consent: profile, license and version are required")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO consents(profile_id,license_id,license_version,accepted_at)
		VALUES(?,?,?,?)`, profileID, licenseID, version, s.stamp()); err != nil {
		return Consent{}, fmt.Errorf("record consent: %w", err)
	}
	c := Consent{ProfileID: profileID, LicenseID: licenseID, LicenseVersion: version}
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(accepted_at,'') FROM consents WHERE profile_id=? AND license_id=?
		AND license_version=?`, profileID, licenseID, version).Scan(&c.AcceptedAt)
	return c, err
}

// HasConsent reports whether the exact license version was accepted for the profile.
func (s *Store) HasConsent(ctx context.Context, profileID, licenseID, version string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM consents WHERE profile_id=? AND license_id=? AND license_version=?`,
		profileID, licenseID, version).Scan(&n)
	return n > 0, err
}

// ListConsents lists every consent, oldest first.
func (s *Store) ListConsents(ctx context.Context) ([]Consent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT profile_id, license_id, license_version, COALESCE(accepted_at,'') FROM consents
		ORDER BY accepted_at, profile_id, license_id, license_version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Consent{}
	for rows.Next() {
		var c Consent
		if err := rows.Scan(&c.ProfileID, &c.LicenseID, &c.LicenseVersion, &c.AcceptedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetSetting returns a settings value and whether the key exists.
func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// SetSetting stores a settings value.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	if key == "" {
		return errors.New("set setting: empty key")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
```

- [ ] 5. `projects.go`: add the training step and the delete guard. The summary query gets the latest run state and "has a succeeded run" as two correlated subqueries in the same `SELECT`. That is one statement per list, not N+1 round trips, and the `runs_project` index serves both.

  The active run is always the newest run of its project, because D13 allows no second run while one is active. So "latest state not terminal" means "in progress".

  - Replace `projectSelect` (`:183-184`):

```go
const projectSelect = `SELECT ` + projectCols + `, (SELECT COUNT(*) FROM dataset_rows r WHERE r.project_id=p.id), c.data_rev, c.ready,
	(SELECT x.state FROM runs x WHERE x.project_id=p.id ORDER BY x.created DESC, x.rowid DESC LIMIT 1),
	EXISTS(SELECT 1 FROM runs x WHERE x.project_id=p.id AND x.state='succeeded')
	FROM projects p LEFT JOIN checks c ON c.project_id=p.id`
```

  - In `scanProject` (`:159-181`): declare `var latest sql.NullString` and `var succeeded int`, and extend the `Scan` call with `&latest, &succeeded` after `&ready`. Replace the `computeSteps` call at `:179` with:

```go
	p.Steps = computeSteps(p, rows, checkRev, ready.Valid && ready.Int64 != 0,
		projectRuns{Latest: latest.String, Succeeded: succeeded != 0})
```

  - `CreateProject` `:149`: `p.Steps = computeSteps(p, 0, sql.NullInt64{}, false, projectRuns{})`.
  - Replace `computeSteps` (`:221-237`):

```go
// projectRuns are the run facts the training step needs: the state of the newest run and whether any succeeded.
type projectRuns struct {
	Latest    string
	Succeeded bool
}

func computeSteps(p Project, rows int, checkRev sql.NullInt64, checkReady bool, runs projectRuns) Steps {
	st := Steps{Goal: "in_progress", Data: "todo", Training: "todo", Test: "todo", Use: "todo"}
	if p.GoalText != "" && p.ProfileID != "" && (p.Recipe != RecipeSpecialist || p.TaskKind != "") &&
		(p.TaskKind != "classify" || len(p.Labels) >= 2) {
		st.Goal = "done"
	}
	switch {
	case rows == 0:
	case checkRev.Valid && checkRev.Int64 == p.DataRev && checkReady:
		st.Data = "ready"
	case checkRev.Valid && checkRev.Int64 == p.DataRev:
		st.Data = "blocked"
	default:
		st.Data = "in_progress"
	}
	// Precedence: an active run, then any success, then a failed or cancelled latest run, then the data state.
	switch {
	case runs.Latest != "" && !RunTerminal(runs.Latest):
		st.Training = "in_progress"
	case runs.Succeeded:
		st.Training = "done"
	case runs.Latest == RunFailed || runs.Latest == RunCancelled:
		st.Training = "blocked"
	case st.Data == "ready":
		st.Training = "ready"
	}
	return st
}
```

  - `DeleteProject`: insert after the confirm-name check (`:401-403`), before `stmts`:

```go
	var activeRuns int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE project_id=? AND state NOT IN `+terminalStatesSQL,
		id).Scan(&activeRuns); err != nil {
		return err
	}
	if activeRuns > 0 {
		return ErrRunActive
	}
```

  Update the doc comment of `DeleteProject`: "… It refuses with ErrRunActive while a run of the project is not terminal. Files under data/chibi_sensei are the caller's (ListRunIDs before, RemoveRunFiles after)."

- [ ] 6. Create `internal/chibisensei/plantoken.go` (the `confirm.go` pattern).

```go
package chibisensei

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// PlanTokenTTL is how long a prepared start stays valid.
const PlanTokenTTL = 15 * time.Minute

const maxPlanTokens = 64

// PlanTicket binds a prepared start: project, the run ID shown in the dialog, the preset and the plan hash.
type PlanTicket struct {
	ProjectID, RunID, Preset, Hash string
	Expires                        time.Time
}

// PlanTokens holds plan tokens in memory (never persisted); at most 64, the earliest-expiring is evicted.
type PlanTokens struct {
	mu    sync.Mutex
	now   func() time.Time
	items map[string]PlanTicket
}

// NewPlanTokens creates the holder; now may be nil.
func NewPlanTokens(now func() time.Time) *PlanTokens {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &PlanTokens{now: now, items: map[string]PlanTicket{}}
}

// Issue stores t with Expires = now + PlanTokenTTL and returns a random 32-hex token and the TTL.
func (p *PlanTokens) Issue(t PlanTicket) (string, time.Duration) {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	token := hex.EncodeToString(raw[:])
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	for k, v := range p.items {
		if now.After(v.Expires) {
			delete(p.items, k)
		}
	}
	for len(p.items) >= maxPlanTokens {
		oldest, first := "", true
		var oldestAt time.Time
		for k, v := range p.items {
			if first || v.Expires.Before(oldestAt) {
				oldest, oldestAt, first = k, v.Expires, false
			}
		}
		delete(p.items, oldest)
	}
	t.Expires = now.Add(PlanTokenTTL)
	p.items[token] = t
	return token, PlanTokenTTL
}

// Consume returns the ticket when the token exists, has not expired and belongs to projectID. It always removes the
// token. The caller still compares Hash with a freshly computed PlanHash.
func (p *PlanTokens) Consume(token, projectID string) (PlanTicket, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	t, ok := p.items[token]
	if !ok {
		return PlanTicket{}, false
	}
	delete(p.items, token)
	if p.now().After(t.Expires) || t.ProjectID != projectID {
		return PlanTicket{}, false
	}
	return t, true
}

// PlanHash is the sha256 (hex) over the canonical JSON of the plan, its data_rev, the profile SHA, the script SHA,
// the owner, the machine shape and the delete flag. Struct order makes the encoding deterministic.
func PlanHash(plan Plan, profileSHA, scriptSHA, owner, machineShape string, deleteAfterRun bool) string {
	payload := encodeJSON(struct {
		V              int    `json:"v"`
		Plan           Plan   `json:"plan"`
		DataRev        int64  `json:"data_rev"`
		ProfileSHA     string `json:"profile_sha"`
		ScriptSHA      string `json:"script_sha"`
		Owner          string `json:"owner"`
		MachineShape   string `json:"machine_shape"`
		DeleteAfterRun bool   `json:"delete_remote_after_run"`
	}{1, plan, plan.DataRev, profileSHA, scriptSHA, owner, machineShape, deleteAfterRun})
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
```

- [ ] 7. Create `internal/chibisensei/quota.go`.

```go
package chibisensei

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
)

// ParseGPUQuota reads the remaining Kaggle GPU quota in hours from the raw JSON answer of
// get_accelerator_quota_statistics. PROVISIONAL: the pinned SDK contract has no typed response, so the real
// shape is unknown until the proof run records one (add it as a fixture to TestParseGPUQuota then). The
// parser is tolerant:
//   - GPU entries are objects with a type-like key (containing "type", "accelerator", "name" or "kind")
//     whose string value contains "gpu", "nvidia" or "tesla"; every object below a key containing "gpu";
//     and the gpu-named numeric keys of any object.
//   - In an entry a key containing "remaining", "available" or "left" wins; otherwise limit ("limit",
//     "total", "quota", "max", "allowed") minus used ("used", "consumed", "spent"). "used" is tested
//     before "quota", so "quotaUsed" counts as used.
//   - A value is a JSON number, a numeric string, or a protobuf duration string ("99000s", "1500ms").
//     The unit comes from the suffix or the key ("milli"/…ms, "second"/…sec, "minute"/…mins, "hour");
//     without a hint a value above 500 is seconds, otherwise hours (the weekly quota is tens of hours).
//   - Several GPU entries give the smallest remaining value (conservative); negatives become 0.
// ok is false when no GPU entry carries a usable number or the JSON is invalid.
func ParseGPUQuota(raw []byte) (hours float64, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return 0, false
	}
	var q quotaScan
	q.walk(doc, false, 0)
	if !q.found {
		return 0, false
	}
	return math.Round(q.min*100) / 100, true
}

type quotaScan struct {
	min   float64
	found bool
}

func (q *quotaScan) note(h float64) {
	if h < 0 {
		h = 0
	}
	if !q.found || h < q.min {
		q.min, q.found = h, true
	}
}

func (q *quotaScan) walk(v any, gpu bool, depth int) {
	if depth > 10 {
		return
	}
	switch x := v.(type) {
	case map[string]any:
		entry := gpu || quotaTypedGPU(x)
		if h, ok := quotaRemaining(x, !entry); ok {
			q.note(h)
		}
		for k, child := range x {
			q.walk(child, entry || strings.Contains(strings.ToLower(k), "gpu"), depth+1)
		}
	case []any:
		for _, child := range x {
			q.walk(child, gpu, depth+1)
		}
	}
}

func quotaKeyHas(key string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(key, p) {
			return true
		}
	}
	return false
}

func quotaTypedGPU(m map[string]any) bool {
	for k, v := range m {
		s, ok := v.(string)
		if !ok || !quotaKeyHas(strings.ToLower(k), "type", "accelerator", "name", "kind") {
			continue
		}
		if quotaKeyHas(strings.ToLower(s), "gpu", "nvidia", "tesla") {
			return true
		}
	}
	return false
}

func quotaRemaining(m map[string]any, gpuKeysOnly bool) (float64, bool) {
	var remaining, limit, used []float64
	for k, v := range m {
		lk := strings.ToLower(k)
		if gpuKeysOnly && !strings.Contains(lk, "gpu") {
			continue
		}
		h, ok := quotaHours(lk, v)
		if !ok {
			continue
		}
		switch {
		case quotaKeyHas(lk, "remaining", "available", "left"):
			remaining = append(remaining, h)
		case quotaKeyHas(lk, "used", "consumed", "spent"):
			used = append(used, h)
		case quotaKeyHas(lk, "limit", "total", "quota", "max", "allowed"):
			limit = append(limit, h)
		}
	}
	if len(remaining) > 0 {
		return slices.Min(remaining), true
	}
	if len(limit) > 0 && len(used) > 0 {
		return slices.Min(limit) - slices.Max(used), true
	}
	return 0, false
}

func quotaHours(key string, v any) (float64, bool) {
	var n float64
	unit := ""
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return 0, false
		}
		n = f
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		switch {
		case strings.HasSuffix(s, "ms"):
			unit, s = "ms", strings.TrimSuffix(s, "ms")
		case strings.HasSuffix(s, "s"):
			unit, s = "s", strings.TrimSuffix(s, "s")
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		n = f
	default:
		return 0, false
	}
	if unit == "" {
		switch {
		case strings.Contains(key, "milli") || strings.HasSuffix(key, "ms"):
			unit = "ms"
		case strings.Contains(key, "second") || strings.HasSuffix(key, "sec") || strings.HasSuffix(key, "secs"):
			unit = "s"
		case strings.Contains(key, "minute") || strings.HasSuffix(key, "mins"):
			unit = "min"
		case strings.Contains(key, "hour"):
			unit = "h"
		}
	}
	switch unit {
	case "ms":
		n /= 3_600_000
	case "s":
		n /= 3600
	case "min":
		n /= 60
	case "h":
	default:
		if n > 500 {
			n /= 3600
		}
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}
```

- [ ] 8. Create `internal/chibisensei/preflight.go` (pure, no I/O).

```go
package chibisensei

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"aurago/internal/config"
)

// Preflight levels.
const (
	PreflightBlock = "block"
	PreflightWarn  = "warn"
)

const preflightKeyPrefix = "chibi_sensei.preflight."

// KaggleCredentialsLink is the Config UI anchor of the Kaggle credentials.
const KaggleCredentialsLink = "/config#kaggle"

// PreflightInput holds the plain values the server collects. EvaluatePreflight does no I/O.
type PreflightInput struct {
	// Data (store and check result).
	DataRev            int64  // the project's current data_rev
	CheckPresent       bool   // a check result exists
	CheckDataRev       int64  // data_rev of that result
	CheckReady         bool   // that result is ready
	ActiveRunID        string // the installation's non-terminal run; "" = none
	ActiveRunProjectID string

	// Kaggle configuration (kaggleSnapshot().Config).
	KaggleEnabled          bool
	KaggleReadOnly         bool
	AllowCreate            bool
	AllowRemoteExecute     bool
	AllowGPU               bool
	AllowInternet          bool
	AllowDownload          bool
	AllowDelete            bool
	AllowedMachineShapes   []string
	RemoteTimeoutMinutes   int
	MaxTransferMB          int
	MaxStorageMB           int
	KaggleStorageUsedBytes int64 // Service.Status()["storage_bytes"]

	// Chibi training configuration.
	MachineShape         string // chibi_sensei.training.machine_shape
	DeleteRemoteAfterRun bool   // chibi_sensei.training.delete_remote_after_run

	// Access (kaggleSnapshot().Access).
	Network bool
	Write   bool

	// Account.
	VaultAvailable    bool
	CredentialsStored bool
	ConnectionTested  bool   // the server called Service.Test (it does so only when KaggleReachable is true)
	ConnectionOK      bool
	Account           string // the account Service.Test returned

	// Quota and compute slots.
	QuotaRead   bool    // the server read get_accelerator_quota_statistics
	QuotaKnown  bool    // ParseGPUQuota could tell the remaining hours
	QuotaHours  float64 // remaining GPU hours
	JobsRead    bool    // KaggleDriver.ActiveComputeJobs answered
	ComputeJobs int     // active compute jobs, unknown included
	UnknownJobs int     // of those, the jobs in status unknown

	// Plan and profile.
	EstimateMinutes      int // plan.estimate_minutes
	ArtifactsEstimatedMB int // plan.artifacts_estimated_mb
	LargestArtifactMB    int // profile.GGUF.EstimatedMB (the largest single download)

	// Local storage.
	DiskFreeKnown    bool
	DiskFreeBytes    int64 // free bytes on the volume of <data>/chibi_sensei
	StorageUsedBytes int64 // StorageBytes(dataDir) plus Store.SizeBytes()
	StorageMaxGB     int   // chibi_sensei.storage.max_gb

	// License.
	ConsentRequired bool
	ConsentRecorded bool // consent for this exact license version exists
	LicenseID       string
	LicenseVersion  string

	Admin bool // reported by the server next to the items; not used by EvaluatePreflight
}

// PreflightItem is one row of the preflight list.
type PreflightItem struct {
	ID         string         `json:"id"`
	Group      string         `json:"group"` // kaggle | account | quota | data | license | storage
	OK         bool           `json:"ok"`
	Level      string         `json:"level"` // block | warn
	MessageKey string         `json:"message_key"`
	Args       map[string]any `json:"args,omitempty"`
	Flag       string         `json:"flag,omitempty"`
	Link       string         `json:"link,omitempty"`
}

// Preflight is the evaluated list; Ready is true when every block item is OK.
type Preflight struct {
	Ready bool            `json:"ready"`
	Items []PreflightItem `json:"items"`
}

// PreflightFlags are the flag names POST /preflight/enable accepts.
var PreflightFlags = []string{"kaggle.enabled", "kaggle.read_only", "kaggle.allow_create", "kaggle.allow_remote_execute",
	"kaggle.allow_gpu", "kaggle.allow_internet", "kaggle.allow_download", "kaggle.allowed_machine_shapes", "kaggle.allow_delete"}

// ErrUnknownPreflightFlag refuses a flag outside PreflightFlags (the route answers 422).
var ErrUnknownPreflightFlag = errors.New("chibisensei: unknown preflight flag")

func preflightItem(id, group, level string, ok bool, flag string, args map[string]any) PreflightItem {
	return PreflightItem{ID: id, Group: group, OK: ok, Level: level, MessageKey: preflightKeyPrefix + id, Args: args, Flag: flag}
}

// preflightBase builds the items dataset_ready … kaggle_credentials.
func preflightBase(in PreflightInput) []PreflightItem {
	var active map[string]any
	if in.ActiveRunID != "" {
		active = map[string]any{"run_id": in.ActiveRunID, "project_id": in.ActiveRunProjectID}
	}
	cred := preflightItem("kaggle_credentials", "account", PreflightBlock, in.CredentialsStored, "", nil)
	cred.Link = KaggleCredentialsLink
	return []PreflightItem{
		preflightItem("dataset_ready", "data", PreflightBlock,
			in.CheckPresent && in.CheckDataRev == in.DataRev && in.CheckReady, "", nil),
		preflightItem("run_active", "data", PreflightBlock, in.ActiveRunID == "", "", active),
		preflightItem("kaggle_enabled", "kaggle", PreflightBlock, in.KaggleEnabled, "kaggle.enabled", nil),
		preflightItem("kaggle_not_readonly", "kaggle", PreflightBlock, !in.KaggleReadOnly, "kaggle.read_only", nil),
		preflightItem("kaggle_allow_create", "kaggle", PreflightBlock, in.AllowCreate, "kaggle.allow_create", nil),
		preflightItem("kaggle_allow_remote_execute", "kaggle", PreflightBlock, in.AllowRemoteExecute, "kaggle.allow_remote_execute", nil),
		preflightItem("kaggle_allow_gpu", "kaggle", PreflightBlock, in.AllowGPU, "kaggle.allow_gpu", nil),
		preflightItem("kaggle_allow_internet", "kaggle", PreflightBlock, in.AllowInternet, "kaggle.allow_internet", nil),
		preflightItem("kaggle_allow_download", "kaggle", PreflightBlock, in.AllowDownload, "kaggle.allow_download", nil),
		preflightItem("kaggle_machine_shape", "kaggle", PreflightBlock,
			in.MachineShape != "" && slices.Contains(in.AllowedMachineShapes, in.MachineShape),
			"kaggle.allowed_machine_shapes", map[string]any{"shape": in.MachineShape}),
		preflightItem("kaggle_allow_delete", "kaggle", PreflightWarn, in.AllowDelete || !in.DeleteRemoteAfterRun,
			"kaggle.allow_delete", nil),
		preflightItem("network_access", "account", PreflightBlock, in.Network, "", nil),
		preflightItem("filesystem_write", "account", PreflightBlock, in.Write, "", nil),
		preflightItem("kaggle_vault", "account", PreflightBlock, in.VaultAvailable, "", nil),
		cred,
	}
}

func reachable(items []PreflightItem) bool {
	for _, it := range items {
		if (it.Group == "kaggle" || it.Group == "account") && it.Level == PreflightBlock && !it.OK {
			return false
		}
	}
	return true
}

// KaggleReachable reports whether every blocking Kaggle and account item before kaggle_connection passes. The
// server calls Service.Test and reads the quota only then.
func KaggleReachable(in PreflightInput) bool { return reachable(preflightBase(in)) }

func round1(x float64) float64 { return math.Round(x*10) / 10 }

// EvaluatePreflight evaluates the contract's item table in its order.
func EvaluatePreflight(in PreflightInput) Preflight {
	items := preflightBase(in)
	canReach := reachable(items)

	conn := preflightItem("kaggle_connection", "account", PreflightBlock, canReach && in.ConnectionTested && in.ConnectionOK, "", nil)
	switch {
	case !canReach || !in.ConnectionTested:
		conn.Args = map[string]any{"skipped": true}
	case in.Account != "":
		conn.Args = map[string]any{"account": in.Account}
	}
	items = append(items, conn)

	needed := int(math.Ceil(1.5*float64(in.EstimateMinutes) + 15))
	items = append(items, preflightItem("kaggle_timeout", "quota", PreflightWarn, in.RemoteTimeoutMinutes >= needed, "",
		map[string]any{"minutes": in.RemoteTimeoutMinutes, "needed": needed}))

	var quota PreflightItem
	switch {
	case !in.QuotaRead:
		quota = preflightItem("kaggle_quota", "quota", PreflightWarn, false, "", map[string]any{"skipped": true})
	case !in.QuotaKnown:
		quota = preflightItem("kaggle_quota", "quota", PreflightWarn, false, "", map[string]any{"unknown": true})
	default:
		need := 1.2 * float64(in.EstimateMinutes) / 60
		quota = preflightItem("kaggle_quota", "quota", PreflightBlock, in.QuotaHours >= need, "",
			map[string]any{"hours": round1(in.QuotaHours), "needed": round1(need)})
	}
	items = append(items, quota)

	if !in.JobsRead {
		items = append(items, preflightItem("kaggle_slot", "quota", PreflightBlock, false, "", map[string]any{"skipped": true}))
	} else {
		items = append(items, preflightItem("kaggle_slot", "quota", PreflightBlock, in.ComputeJobs == 0, "",
			map[string]any{"jobs": in.ComputeJobs, "unknown": in.UnknownJobs}))
	}

	largest := int64(max(in.LargestArtifactMB, 0))
	kaggleFreeMB := max((int64(in.MaxStorageMB)<<20-in.KaggleStorageUsedBytes)>>20, 0)
	items = append(items, preflightItem("kaggle_transfer", "storage", PreflightBlock,
		int64(in.MaxTransferMB) >= largest && kaggleFreeMB >= largest, "",
		map[string]any{"needed_mb": largest, "max_transfer_mb": in.MaxTransferMB, "free_mb": kaggleFreeMB}))

	neededMB := int64(math.Ceil(1.2 * float64(max(in.ArtifactsEstimatedMB, 0))))
	capFree := max(int64(in.StorageMaxGB)<<30-in.StorageUsedBytes, 0)
	freeMB := int64(-1)
	if in.DiskFreeKnown {
		freeMB = in.DiskFreeBytes >> 20
	}
	items = append(items, preflightItem("disk_space", "storage", PreflightBlock,
		in.DiskFreeKnown && in.DiskFreeBytes >= neededMB<<20 && capFree >= neededMB<<20, "",
		map[string]any{"needed_mb": neededMB, "free_mb": freeMB, "cap_free_mb": capFree >> 20}))

	items = append(items, preflightItem("license_consent", "license", PreflightBlock,
		!in.ConsentRequired || in.ConsentRecorded, "",
		map[string]any{"license_id": in.LicenseID, "license_version": in.LicenseVersion}))

	ready := true
	for _, it := range items {
		if it.Level == PreflightBlock && !it.OK {
			ready = false
		}
	}
	return Preflight{Ready: ready, Items: items}
}

// FixFlags lists the flags of the failing items in item order without repeats; nil when none (B5 sends fix: null).
func (p Preflight) FixFlags() []string {
	var out []string
	for _, it := range p.Items {
		if !it.OK && it.Flag != "" && !slices.Contains(out, it.Flag) {
			out = append(out, it.Flag)
		}
	}
	return out
}

// PreflightFlagPatch maps flag names to one config patch shaped like config.yaml, for applyConfigPatch:
// kaggle.enabled → true, kaggle.read_only → false, kaggle.allow_* → true, kaggle.allowed_machine_shapes → the
// current list plus machineShape (deduplicated, current order kept, as []any). A flag outside PreflightFlags, an
// empty list, or an unknown machine shape is an error wrapping ErrUnknownPreflightFlag.
func PreflightFlagPatch(flags, currentShapes []string, machineShape string) (map[string]any, error) {
	if len(flags) == 0 {
		return nil, fmt.Errorf("%w: no flags", ErrUnknownPreflightFlag)
	}
	kaggle := map[string]any{}
	for _, f := range flags {
		switch f {
		case "kaggle.enabled":
			kaggle["enabled"] = true
		case "kaggle.read_only":
			kaggle["read_only"] = false
		case "kaggle.allow_create", "kaggle.allow_remote_execute", "kaggle.allow_gpu", "kaggle.allow_internet",
			"kaggle.allow_download", "kaggle.allow_delete":
			kaggle[strings.TrimPrefix(f, "kaggle.")] = true
		case "kaggle.allowed_machine_shapes":
			if !slices.Contains(config.KaggleMachineShapes, machineShape) {
				return nil, fmt.Errorf("%w: machine shape %q is not a known name", ErrUnknownPreflightFlag, machineShape)
			}
			list := []any{}
			seen := map[string]bool{}
			for _, s := range append(append([]string(nil), currentShapes...), machineShape) {
				s = strings.TrimSpace(s)
				if s != "" && !seen[s] {
					seen[s] = true
					list = append(list, s)
				}
			}
			kaggle["allowed_machine_shapes"] = list
		default:
			return nil, fmt.Errorf("%w: %s", ErrUnknownPreflightFlag, f)
		}
	}
	return map[string]any{"kaggle": kaggle}, nil
}
```

- [ ] 9. Create `internal/chibisensei/report.go`.

```go
package chibisensei

import (
	"encoding/json"
	"io"
	"math"
	"reflect"
	"strings"

	"golang.org/x/text/unicode/norm"
)

const smallSampleN = 30

// LossPair holds a base and a tuned value; either may be null.
type LossPair struct {
	Base  *float64 `json:"base"`
	Tuned *float64 `json:"tuned"`
}

// ExactMatch is the share of eval samples whose answer equals the expected one.
type ExactMatch struct {
	Base  float64 `json:"base"`
	Tuned float64 `json:"tuned"`
	N     int     `json:"n"`
}

// ConfusionPair holds two matrices: rows = expected label, columns = predicted label plus a last "other" column.
type ConfusionPair struct {
	Base  [][]int `json:"base"`
	Tuned [][]int `json:"tuned"`
}

// LabelAccuracy is the classify score.
type LabelAccuracy struct {
	Base      float64       `json:"base"`
	Tuned     float64       `json:"tuned"`
	N         int           `json:"n"`
	Labels    []string      `json:"labels"`
	Confusion ConfusionPair `json:"confusion"`
}

// Report is the automatic report card (API shape "Report").
type Report struct {
	RunID         string         `json:"run_id"` // set by the caller
	Kind          string         `json:"kind"`   // auto
	Samples       int            `json:"samples"`
	SmallSample   bool           `json:"small_sample"`
	Loss          *LossPair      `json:"loss"`
	ExactMatch    *ExactMatch    `json:"exact_match"`    // extract, rewrite, to_json
	LabelAccuracy *LabelAccuracy `json:"label_accuracy"` // classify
	Perplexity    *LossPair      `json:"perplexity"`     // free
}

// statusLosses is the only place that reads B2's Status (adapt here if B2's field names differ).
func statusLosses(st Status) (base, tuned *float64) {
	return finitePtr(st.Train.EvalLossBase), finitePtr(st.Train.EvalLossTuned)
}

// sampleAnswers is the only place that reads B2's EvalSample.
func sampleAnswers(s EvalSample) (expected, base, tuned string) { return s.Expected, s.BaseOutput, s.TunedOutput }

func finitePtr(v *float64) *float64 {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	x := *v
	return &x
}

func round4(x float64) float64 { return math.Round(x*1e4) / 1e4 }

// ScoreReport computes the P2 report card: loss base vs tuned for every recipe; exact match for extract and rewrite
// (normalized string equality: NFC, trimmed, whitespace runs collapsed, case kept) and for to_json (semantic JSON
// equality); label accuracy and confusion tables for classify; perplexity (exp of the loss) for free. small_sample
// is set when fewer than 30 eval samples exist. RunID stays empty.
func ScoreReport(recipe, taskKind string, labels []string, st Status, samples []EvalSample) Report {
	rep := Report{Kind: "auto", Samples: len(samples), SmallSample: len(samples) < smallSampleN}
	base, tuned := statusLosses(st)
	if base != nil || tuned != nil {
		rep.Loss = &LossPair{Base: base, Tuned: tuned}
	}
	switch {
	case recipe == RecipeFree:
		pb, pt := perplexity(base), perplexity(tuned)
		if pb != nil || pt != nil {
			rep.Perplexity = &LossPair{Base: pb, Tuned: pt}
		}
	case recipe == RecipeSpecialist && (taskKind == "extract" || taskKind == "rewrite" || taskKind == "to_json"):
		rep.ExactMatch = exactMatch(taskKind, samples)
	case recipe == RecipeSpecialist && taskKind == "classify":
		rep.LabelAccuracy = labelAccuracy(labels, samples)
	}
	return rep
}

func perplexity(loss *float64) *float64 {
	if loss == nil {
		return nil
	}
	p := math.Exp(*loss)
	if math.IsNaN(p) || math.IsInf(p, 0) {
		return nil
	}
	p = math.Round(p*1000) / 1000
	return &p
}

// normalizeAnswer applies NFC, trims and collapses whitespace runs; case is kept.
func normalizeAnswer(s string) string { return strings.Join(strings.Fields(norm.NFC.String(s)), " ") }

func exactMatch(taskKind string, samples []EvalSample) *ExactMatch {
	if len(samples) == 0 {
		return nil
	}
	var okBase, okTuned int
	for _, s := range samples {
		want, base, tuned := sampleAnswers(s)
		if answerMatches(taskKind, want, base) {
			okBase++
		}
		if answerMatches(taskKind, want, tuned) {
			okTuned++
		}
	}
	n := len(samples)
	return &ExactMatch{Base: round4(float64(okBase) / float64(n)), Tuned: round4(float64(okTuned) / float64(n)), N: n}
}

func answerMatches(taskKind, want, got string) bool {
	if taskKind == "to_json" {
		if w, ok := decodeJSONValue(want); ok {
			g, ok := decodeJSONValue(got)
			return ok && reflect.DeepEqual(w, g)
		}
	}
	return normalizeAnswer(want) == normalizeAnswer(got)
}

// decodeJSONValue decodes exactly one JSON value; numbers become float64 (so 1 equals 1.0), or their literal text
// when they do not fit.
func decodeJSONValue(s string) (any, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil {
		return nil, false
	}
	if dec.Decode(new(any)) != io.EOF {
		return nil, false
	}
	return canonicalJSONValue(v), true
}

func canonicalJSONValue(v any) any {
	switch x := v.(type) {
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case map[string]any:
		for k, c := range x {
			x[k] = canonicalJSONValue(c)
		}
		return x
	case []any:
		for i, c := range x {
			x[i] = canonicalJSONValue(c)
		}
		return x
	}
	return v
}

func normalizeLabel(s string) string {
	return normalizeAnswer(strings.Trim(normalizeAnswer(s), "\"'`.!"))
}

// matchLabel returns the index of the label an answer names: exact (normalized) first, then case-insensitive; -1 = none.
func matchLabel(labels []string, answer string) int {
	a := normalizeLabel(answer)
	for i, l := range labels {
		if normalizeLabel(l) == a {
			return i
		}
	}
	for i, l := range labels {
		if strings.EqualFold(normalizeLabel(l), a) {
			return i
		}
	}
	return -1
}

func labelAccuracy(labels []string, samples []EvalSample) *LabelAccuracy {
	k := len(labels)
	if k == 0 {
		return nil
	}
	newMatrix := func() [][]int {
		m := make([][]int, k)
		for i := range m {
			m[i] = make([]int, k+1)
		}
		return m
	}
	cb, ct := newMatrix(), newMatrix()
	var n, okBase, okTuned int
	for _, s := range samples {
		want, base, tuned := sampleAnswers(s)
		e := matchLabel(labels, want)
		if e < 0 {
			continue
		}
		n++
		b, t := matchLabel(labels, base), matchLabel(labels, tuned)
		if b == e {
			okBase++
		}
		if t == e {
			okTuned++
		}
		if b < 0 {
			b = k
		}
		if t < 0 {
			t = k
		}
		cb[e][b]++
		ct[e][t]++
	}
	if n == 0 {
		return nil
	}
	return &LabelAccuracy{Base: round4(float64(okBase) / float64(n)), Tuned: round4(float64(okTuned) / float64(n)), N: n,
		Labels: append([]string(nil), labels...), Confusion: ConfusionPair{Base: cb, Tuned: ct}}
}
```

- [ ] 10. Create `internal/chibisensei/driver.go`. The guarantees come from `internal/kaggle/service.go:59-88,119-240,242-315,413-435,471-492`, `store.go:91-125`, `transfers.go:166-386` and `remote_jobs.go:12-153,193-248`.

```go
package chibisensei

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"aurago/internal/kaggle"
)

// KaggleProjectTitle is the title of the one local Kaggle project that stages Chibi files.
const KaggleProjectTitle = "Chibi Sensei"

// SettingKaggleProjectID is the Chibi settings key that keeps that project's id.
const SettingKaggleProjectID = "kaggle_project_id"

// Kaggle job statuses (string literals in internal/kaggle).
const (
	KaggleJobQueued          = "queued"
	KaggleJobRunning         = "running"
	KaggleJobUnknown         = "unknown"
	KaggleJobRemoteRunning   = "remote_running"
	KaggleJobCancelRequested = "cancel_requested"
	KaggleJobCompleted       = "completed"
	KaggleJobFailed          = "failed"
	KaggleJobCancelled       = "cancelled"
	KaggleJobResolvedUnknown = "resolved_unknown"
)

// KaggleJobActive reports whether a status holds (or may hold) a compute slot, as checkComputeSlots counts it.
func KaggleJobActive(status string) bool {
	switch status {
	case KaggleJobQueued, KaggleJobRunning, KaggleJobUnknown, KaggleJobRemoteRunning, KaggleJobCancelRequested:
		return true
	}
	return false
}

// KaggleJobTerminal reports whether a status is final (unknown is not: it needs an administrator).
func KaggleJobTerminal(status string) bool {
	switch status {
	case KaggleJobCompleted, KaggleJobFailed, KaggleJobCancelled, KaggleJobResolvedUnknown:
		return true
	}
	return false
}

// KaggleRequestID is the deterministic request ID of one step attempt: the first 32 hex characters of
// sha256(run_id + ":" + step + ":" + attempt).
func KaggleRequestID(runID, step string, attempt int) string {
	sum := sha256.Sum256([]byte(runID + ":" + step + ":" + strconv.Itoa(attempt)))
	return hex.EncodeToString(sum[:])[:32]
}

// IsKaggleNotFound reports a Kaggle 404 (for example a read of a dataset that does not exist).
func IsKaggleNotFound(err error) bool {
	var ae *kaggle.APIError
	return errors.As(err, &ae) && ae.Status == 404
}

// IsKaggleUncertain reports an error whose remote outcome is unknown.
func IsKaggleUncertain(err error) bool {
	var ae *kaggle.APIError
	return errors.As(err, &ae) && ae.Uncertain
}

var kaggleOpDomains = map[string]string{
	"upload_dataset_file": "datasets", "create_dataset": "datasets", "get_dataset": "datasets",
	"get_dataset_status": "datasets", "delete_dataset": "datasets",
	"save_kernel": "notebooks", "get_kernel": "notebooks", "get_kernel_session_status": "notebooks",
	"list_kernel_session_output": "notebooks", "download_kernel_output": "notebooks", "delete_kernel": "notebooks",
	"get_accelerator_quota_statistics": "notebooks",
}

// NewKaggleRequest builds a request for an operation Chibi uses. params is marshalled as the SDK wire object
// (field names of the pinned contract, e.g. "ownerSlug"); nil gives {}.
func NewKaggleRequest(id, op, project, file string, params any) (kaggle.Request, error) {
	domain, ok := kaggleOpDomains[op]
	if !ok {
		return kaggle.Request{}, fmt.Errorf("kaggle request: operation %q is not used by Chibi Sensei", op)
	}
	if params == nil {
		params = map[string]any{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return kaggle.Request{}, fmt.Errorf("kaggle request %s: %w", op, err)
	}
	return kaggle.Request{ID: id, Domain: domain, Operation: op, Parameters: raw, ProjectID: project, File: file}, nil
}

// KaggleDriver is the orchestrator's only path to Kaggle. B5 implements it over *kaggle.Service with the current
// kaggleSnapshot().Access; fakeKaggle implements it for tests.
type KaggleDriver interface {
	// Account returns the authenticated Kaggle account (Service.Test → "account"). It needs network access and never
	// mutates; an error means the account is unknown right now (credentials, network or Kaggle).
	Account(ctx context.Context) (string, error)

	// Execute runs one catalog operation.
	//
	// Reads (grant read: get_dataset, get_dataset_status, get_kernel, get_kernel_session_status,
	// list_kernel_session_output, get_accelerator_quota_statistics) run synchronously. Result.Data is the scrubbed
	// JSON answer ({} when empty), Result.Job is nil, req.ID is ignored and nothing is persisted. A missing resource
	// is an error for which IsKaggleNotFound is true.
	//
	// Every other operation (upload_dataset_file, create_dataset, save_kernel, download_kernel_output,
	// delete_dataset, delete_kernel) is a durable job. Execute admits it and returns at once with Result.Job in
	// status queued (or already failed when the compute slot check refused a SAVE_AND_RUN_ALL: Job.Error
	// "concurrent Kaggle compute limit reached", nil error). The job then runs in the Kaggle service; poll it with Job.
	//
	// req.ID must be 32 hex characters (KaggleRequestID); the job ID equals req.ID. Execute with a known ID and the
	// same canonical request (account, domain, operation, project, file, parameters) returns the stored job in its
	// current status and does nothing else. A known ID with a different request is an error. An error means the
	// request was refused before it ran (validation, grants, policy, account, timeout bound, the host's job gate).
	// Only the host gate admits the job as failed before it returns the error; so after any error the same ID
	// can be retried safely: it is either admitted fresh or returns the original receipt.
	//
	// Job statuses: queued → running → completed | failed | unknown. unknown means the outcome is uncertain (an
	// uncertain network error, or an AuraGo restart while queued or running); the service never retries it and a
	// replay of the ID returns unknown again, so a new attempt needs a new ID (attempt+1). save_kernel with
	// kernelExecutionType SAVE_AND_RUN_ALL ends running as remote_running; the service polls the session every
	// 10 s, writes Result.remote_status (upper-case Kaggle status) and ends completed | failed | cancelled.
	// remote_running survives a restart. Result of save_kernel: ref, url, versionNumber, kernelId; a
	// kernelSessionId is not promised.
	//
	// The service applies defaults: save_kernel isPrivate true, enableGpu/enableTpu/enableInternet false unless
	// given, sessionTimeoutSeconds must lie in (0, remote_timeout_minutes×60] (otherwise an error); create_dataset
	// isPrivate true and ownerSlug = account.
	//
	// upload_dataset_file needs ProjectID and File (the staged project file) and Parameters {}; the service fills
	// fileName (= File), contentLength and lastModifiedEpochSeconds. Completed Result:
	// {"upload_ref":"upload:<job id>","bytes":n,"file":name}; upload_ref goes into create_dataset files[].token.
	// download_kernel_output needs ProjectID and File, a project file name that must not exist yet (an existing
	// file fails the job with "project file revision changed"). Completed Result: {"project_id","file","bytes"}.
	// Job.Result and Job.Error are public, scrubbed text.
	Execute(ctx context.Context, req kaggle.Request) (kaggle.Result, error)

	// Job returns the stored job (Store.Get). Reading never changes the job on the real service. An unknown id is
	// an error (sql.ErrNoRows on the real store).
	Job(ctx context.Context, id string) (kaggle.Job, error)

	// ActiveComputeJobs lists the jobs that hold a compute slot: status queued, running, unknown, remote_running or
	// cancel_requested and operation save_kernel with SAVE_AND_RUN_ALL (or a benchmark batch). It includes jobs of
	// the agent and other apps; an unknown job stays until an administrator resolves it in the Kaggle app.
	ActiveComputeJobs(ctx context.Context) ([]kaggle.Job, error)

	// EnsureProject returns the id (32 hex) of the local Kaggle project titled KaggleProjectTitle. It creates the
	// project on first use and keeps its id under SettingKaggleProjectID; later calls return the same id while the
	// project exists and recreate it when it is gone. Needs Kaggle enabled and filesystem write access.
	EnsureProject(ctx context.Context) (string, error)

	// WriteFile writes r to project/name and replaces an existing regular file atomically, within max_transfer_mb,
	// the free part of max_storage_mb and free disk minus 64 MiB. Names follow the Kaggle package rules (no
	// separators, no leading dot, under 240 bytes). It returns the bytes written.
	WriteFile(ctx context.Context, project, name string, r io.Reader) (int64, error)

	// OpenFile opens a regular project file for reading; the caller closes it. A missing file matches os.ErrNotExist.
	OpenFile(project, name string) (*os.File, error)

	// RemoveFile deletes one regular project file; a missing file is not an error.
	RemoveFile(project, name string) error

	// ReadKernelLog reads the session log of one notebook version without a durable job and without a project file:
	// at most max bytes from the end (the read itself is capped at 16 MiB). An empty body with a nil error means
	// "no log yet". Needs the download grant, filesystem write and network; owner must be the authenticated account.
	// The body is raw (a JSON array of {stream_name,time,data} or plain text); the caller scrubs before storing.
	ReadKernelLog(ctx context.Context, owner, slug string, version int, max int64) ([]byte, error)

	// Cancel is Service.Cancel. queued or running: cancels the local job (it ends failed or unknown).
	// remote_running: sends cancel_kernel_session, which needs a kernelSessionId that SaveKernel normally does not
	// return, so it normally returns an error and the job stays remote_running (the user stops the notebook on
	// Kaggle). cancel_requested: nil. Any other status: error "job is not active".
	Cancel(ctx context.Context, jobID string) error
}
```

- [ ] 11. Create `internal/chibisensei/fakekaggle_test.go`. This is the complete fake; B4's acceptance suite builds on it. It has no goroutines and no clock, so it is deterministic.

```go
package chibisensei

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/kaggle"
)

var _ KaggleDriver = (*fakeKaggle)(nil)

// fakeProjectID is the id EnsureProject returns.
const fakeProjectID = "0123456789abcdef0123456789abcdef"

// fakeKaggle is an in-memory KaggleDriver that models kaggle.Service for the run state-machine suite:
//   - Execute validates the request with kaggle.Lookup and kaggle.Decode, like the real service.
//   - Reads answer at once (Result.Data, no job); see readLocked for the defaults.
//   - A mutation is admitted as a job in status queued and returned at once. The job moves only when it is
//     observed: each Job(id) read applies one step (Polls reads answer running, then the final status of the
//     operation's script; the default script is "completed").
//   - A known request ID returns the stored job unchanged (a replay, Fresh=false); a known ID with another
//     canonical request is an error. Only fresh admissions are side effects (Mutations).
//   - save_kernel with SAVE_AND_RUN_ALL completes as remote_running. Each observation (a Job read of that job or
//     a default get_kernel_session_status read of its slug) applies the next scripted Kaggle status (default
//     RUNNING, COMPLETE). A terminal Kaggle status turns the job completed, failed or cancelled; the last status
//     of the script repeats.
//   - A compute job over maxRemoteJobs (default 1, foreign jobs included) is admitted already failed.
//   - RestartService turns queued and running jobs into unknown, as kaggle.NewService does.
//   - Files live in a temporary directory, so OpenFile returns a real *os.File.
type fakeKaggle struct {
	t   *testing.T
	mu  sync.Mutex
	dir string

	account           string
	accountErr        error
	projectErr        error
	maxRemoteJobs     int
	maxSessionSeconds int64

	scripts    map[string][]fakeOutcome
	reads      map[string][]fakeRead
	logs       []fakeLog
	cancelErrs []error
	writeErrs  map[string]error
	outputs    map[string][]byte

	jobs      map[string]*fakeJob
	order     []string
	seq       int
	kernelSeq int
	datasets  map[string]*fakeDataset
	kernels   map[string]*fakeKernel
	uploads   map[string]fakeUpload
	calls     []fakeCall
}

// fakeOutcome scripts one fresh admission of an operation.
type fakeOutcome struct {
	Status           string         // final status: "" or completed (default), failed, unknown
	Error            string         // Job.Error for failed and unknown (a default text when empty)
	Result           map[string]any // merged into the default result of a completed job
	Polls            int            // Job reads that answer running before the final status
	Remote           []string       // save_kernel SAVE_AND_RUN_ALL: Kaggle statuses per observation (default RUNNING, COMPLETE)
	ExecErr          error          // Execute returns this error; nothing is admitted ...
	AdmitBeforeError bool           // ... unless set: then the job is admitted as failed first (the host gate)
}

// fakeRead scripts one answer of a read operation.
type fakeRead struct {
	Data any   // marshalled to JSON; []byte, json.RawMessage and string are used as they are
	Err  error // for example fakeNotFound()
}

// fakeLog scripts one ReadKernelLog answer.
type fakeLog struct {
	Body []byte
	Err  error
}

// fakeCall records one driver call.
type fakeCall struct {
	Method  string         // Account | Execute | Job | ActiveComputeJobs | EnsureProject | WriteFile | OpenFile | RemoveFile | ReadKernelLog | Cancel
	Op      string         // Execute: the operation
	ID      string         // Execute: the request (job) ID; Job and Cancel: the job ID
	Project string         // Execute, WriteFile, OpenFile, RemoveFile
	File    string         // Execute, WriteFile, OpenFile, RemoveFile; ReadKernelLog: owner/slug
	Version int            // ReadKernelLog
	Max     int64          // ReadKernelLog
	Params  map[string]any // Execute: decoded parameters (numbers are json.Number)
	Fresh   bool           // Execute of a mutation: a new job was admitted
	Err     error
}

// fakeUpload is a completed upload receipt.
type fakeUpload struct {
	JobID, Name, SHA256 string
	Bytes               int64
}

type fakeJob struct {
	job     kaggle.Job
	hash    string
	op      kaggle.Operation
	params  map[string]any
	project string
	file    string
	outcome fakeOutcome
	polls   int
	compute bool
	kernel  string // save_kernel: owner/slug
}

type fakeDataset struct {
	ref     string
	files   []string
	deleted bool
}

type fakeKernel struct {
	ref     string
	jobID   string
	version int
	remote  []string
	status  string
	deleted bool
}

func newFakeKaggle(t *testing.T) *fakeKaggle {
	t.Helper()
	return &fakeKaggle{t: t, dir: t.TempDir(), account: "tester", maxRemoteJobs: 1, maxSessionSeconds: 720 * 60,
		scripts: map[string][]fakeOutcome{}, reads: map[string][]fakeRead{}, writeErrs: map[string]error{},
		outputs: map[string][]byte{}, jobs: map[string]*fakeJob{}, datasets: map[string]*fakeDataset{},
		kernels: map[string]*fakeKernel{}, uploads: map[string]fakeUpload{}}
}

// ---- scripting -------------------------------------------------------------------------------------------

// Script appends outcomes for the next fresh admissions of op, consumed in order; afterwards the default applies.
func (f *fakeKaggle) Script(op string, outcomes ...fakeOutcome) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scripts[op] = append(f.scripts[op], outcomes...)
}

// ScriptRead appends answers for a read operation; each call takes the next, the last one repeats.
func (f *fakeKaggle) ScriptRead(op string, answers ...fakeRead) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads[op] = append(f.reads[op], answers...)
}

// ScriptLogs appends ReadKernelLog answers; each call takes the next, the last one repeats. Without a script the
// log is empty ("no log yet").
func (f *fakeKaggle) ScriptLogs(logs ...fakeLog) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, logs...)
}

// SetOutputs replaces the kernel output files that download_kernel_output and list_kernel_session_output serve.
func (f *fakeKaggle) SetOutputs(files map[string][]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outputs = map[string][]byte{}
	for k, v := range files {
		f.outputs[k] = append([]byte(nil), v...)
	}
}

// SetOutput sets one kernel output file.
func (f *fakeKaggle) SetOutput(name string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outputs[name] = append([]byte(nil), body...)
}

// SetAccount sets the account and an error for Account and for every mutation (Identity fails before admission).
func (f *fakeKaggle) SetAccount(account string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.account, f.accountErr = account, err
}

// FailEnsureProject makes EnsureProject return err until reset with nil.
func (f *fakeKaggle) FailEnsureProject(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectErr = err
}

// FailWrite makes the next WriteFile of name return err (once).
func (f *fakeKaggle) FailWrite(name string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeErrs[name] = err
}

// FailCancel makes the next Cancel calls return these errors in order, whatever the job status.
func (f *fakeKaggle) FailCancel(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelErrs = append(f.cancelErrs, errs...)
}

// SetMaxRemoteJobs sets kaggle.max_remote_jobs (default 1).
func (f *fakeKaggle) SetMaxRemoteJobs(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxRemoteJobs = n
}

// SetMaxSessionSeconds sets kaggle.remote_timeout_minutes×60 (default 720×60).
func (f *fakeKaggle) SetMaxSessionSeconds(n int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxSessionSeconds = n
}

// SetQuota scripts the answer of get_accelerator_quota_statistics (raw JSON or a value); it replaces earlier answers.
func (f *fakeKaggle) SetQuota(data any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads["get_accelerator_quota_statistics"] = []fakeRead{{Data: data}}
}

// AddForeignComputeJob adds a compute job of another app (not a Chibi mutation) in the given status.
func (f *fakeKaggle) AddForeignComputeJob(status string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := fakeRandomID()
	f.seq++
	op, _ := kaggle.Lookup("notebooks", "save_kernel")
	f.jobs[id] = &fakeJob{job: kaggle.Job{ID: id, Account: f.account, Domain: "notebooks", Operation: "save_kernel",
		Status: status, Created: fakeStamp(f.seq), Updated: fakeStamp(f.seq)}, op: op, compute: true}
	f.order = append(f.order, id)
	return id
}

// SetJobStatus forces a job status (and Job.Error) without running anything.
func (f *fakeKaggle) SetJobStatus(id, status, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if j := f.jobs[id]; j != nil {
		f.setLocked(j, status, message, nil)
	}
}

// RestartService models an AuraGo restart of the Kaggle service: queued and running jobs become unknown;
// remote_running and cancel_requested stay.
func (f *fakeKaggle) RestartService() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range f.order {
		j := f.jobs[id]
		if j.job.Status == KaggleJobQueued || j.job.Status == KaggleJobRunning {
			f.setLocked(j, KaggleJobUnknown, "AuraGo restarted; no mutation was replayed", nil)
		}
	}
}

// ---- recorded calls and state ----------------------------------------------------------------------------

// Calls returns every recorded call in order.
func (f *fakeKaggle) Calls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCall(nil), f.calls...)
}

func (f *fakeKaggle) filter(keep func(fakeCall) bool) []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []fakeCall
	for _, c := range f.calls {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

// CallsTo returns the calls of one driver method.
func (f *fakeKaggle) CallsTo(method string) []fakeCall {
	return f.filter(func(c fakeCall) bool { return c.Method == method })
}

// ExecCalls returns every Execute of op (reads, admissions, replays and refusals).
func (f *fakeKaggle) ExecCalls(op string) []fakeCall {
	return f.filter(func(c fakeCall) bool { return c.Method == "Execute" && c.Op == op })
}

// Mutations returns the fresh admissions of op ("" = every operation): the real side effects.
func (f *fakeKaggle) Mutations(op string) []fakeCall {
	return f.filter(func(c fakeCall) bool { return c.Method == "Execute" && c.Fresh && (op == "" || c.Op == op) })
}

// Replays returns the Executes of op that hit a known ID and returned the stored receipt.
func (f *fakeKaggle) Replays(op string) []fakeCall {
	return f.filter(func(c fakeCall) bool {
		return c.Method == "Execute" && !c.Fresh && c.Err == nil && c.Op == op && c.ID != "" && f.isMutation(c.Op)
	})
}

func (f *fakeKaggle) isMutation(op string) bool {
	o, err := kaggle.Lookup(kaggleOpDomains[op], op)
	return err == nil && o.Grant != "read"
}

// PeekJob returns a job without observing (advancing) it.
func (f *fakeKaggle) PeekJob(id string) (kaggle.Job, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return kaggle.Job{}, false
	}
	return j.job, true
}

// JobsOf returns the jobs of op in admission order, without observing them.
func (f *fakeKaggle) JobsOf(op string) []kaggle.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []kaggle.Job
	for _, id := range f.order {
		if j := f.jobs[id]; j.job.Operation == op {
			out = append(out, j.job)
		}
	}
	return out
}

// Upload returns the receipt of a completed upload job.
func (f *fakeKaggle) Upload(jobID string) (fakeUpload, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.uploads[jobID]
	return u, ok
}

// Dataset returns the file names of a dataset (owner/slug) and whether it exists and is not deleted.
func (f *fakeKaggle) Dataset(ref string) ([]string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.datasets[ref]
	if d == nil || d.deleted {
		return nil, false
	}
	return append([]string(nil), d.files...), true
}

// Kernel returns the current Kaggle status and version of a notebook (owner/slug) and whether it exists.
func (f *fakeKaggle) Kernel(ref string) (status string, version int, live bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := f.kernels[ref]
	if k == nil || k.deleted {
		return "", 0, false
	}
	return k.status, k.version, true
}

// ProjectFiles lists the files in the staging project, sorted.
func (f *fakeKaggle) ProjectFiles() []string {
	entries, _ := os.ReadDir(filepath.Join(f.dir, fakeProjectID))
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// assertMutations checks the number of fresh admissions per operation; an operation not listed must have none.
func assertMutations(t *testing.T, f *fakeKaggle, want map[string]int) {
	t.Helper()
	got := map[string]int{}
	for _, c := range f.Mutations("") {
		got[c.Op]++
	}
	for op, n := range want {
		if got[op] != n {
			t.Errorf("%s admitted %d times, want %d", op, got[op], n)
		}
	}
	for op, n := range got {
		if _, ok := want[op]; !ok {
			t.Errorf("%s admitted %d times, want none", op, n)
		}
	}
}

// ---- KaggleDriver ----------------------------------------------------------------------------------------

func (f *fakeKaggle) record(c fakeCall) { f.calls = append(f.calls, c) }

func (f *fakeKaggle) Account(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(fakeCall{Method: "Account", Err: f.accountErr})
	if f.accountErr != nil {
		return "", f.accountErr
	}
	return f.account, nil
}

func (f *fakeKaggle) Execute(ctx context.Context, req kaggle.Request) (kaggle.Result, error) {
	if err := ctx.Err(); err != nil {
		return kaggle.Result{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "Execute", Op: req.Operation, ID: req.ID, Project: req.ProjectID, File: req.File}
	op, err := kaggle.Lookup(req.Domain, req.Operation)
	if err == nil {
		call.Params, _, err = kaggle.Decode(op, req.Parameters)
	}
	if err != nil {
		call.Err = err
		f.record(call)
		return kaggle.Result{}, err
	}
	if op.Grant == "read" {
		data, err := f.readLocked(op.Name, call.Params)
		call.Err = err
		f.record(call)
		return kaggle.Result{Data: data}, err
	}
	res, fresh, err := f.admitLocked(op, req, call.Params)
	call.Fresh, call.Err = fresh, err
	if res.Job != nil {
		call.ID = res.Job.ID
	}
	f.record(call)
	return res, err
}

func (f *fakeKaggle) admitLocked(op kaggle.Operation, req kaggle.Request, params map[string]any) (kaggle.Result, bool, error) {
	if f.accountErr != nil {
		return kaggle.Result{}, false, f.accountErr
	}
	if op.Name == "save_kernel" {
		if n, ok := params["sessionTimeoutSeconds"].(json.Number); ok {
			if v, _ := n.Int64(); v <= 0 || v > f.maxSessionSeconds {
				return kaggle.Result{}, false, errors.New("remote run exceeds configured timeout")
			}
		}
	}
	id := req.ID
	if id == "" {
		id = fakeRandomID()
	} else if !fakeValidID(id) {
		return kaggle.Result{}, false, errors.New("request ID must be 32 hexadecimal characters")
	}
	canonical, _ := json.Marshal(struct {
		Domain, Operation, Project, File string
		Parameters                       map[string]any
	}{op.Domain, op.Name, req.ProjectID, req.File, params})
	sum := sha256.Sum256(canonical)
	hash := hex.EncodeToString(sum[:])
	if j, ok := f.jobs[id]; ok {
		if j.hash != hash {
			return kaggle.Result{}, false, errors.New("request ID already belongs to a different account or request")
		}
		job := j.job
		return kaggle.Result{Job: &job}, false, nil
	}
	var out fakeOutcome
	if queue := f.scripts[op.Name]; len(queue) > 0 {
		out, f.scripts[op.Name] = queue[0], queue[1:]
	}
	if out.ExecErr != nil && !out.AdmitBeforeError {
		return kaggle.Result{}, false, out.ExecErr
	}
	f.seq++
	j := &fakeJob{job: kaggle.Job{ID: id, Account: f.account, Domain: op.Domain, Operation: op.Name,
		Status: KaggleJobQueued, Created: fakeStamp(f.seq), Updated: fakeStamp(f.seq)},
		hash: hash, op: op, params: params, project: req.ProjectID, file: req.File, outcome: out, polls: out.Polls,
		compute: op.Name == "save_kernel" && fakeString(params, "kernelExecutionType") == "SAVE_AND_RUN_ALL"}
	f.jobs[id] = j
	f.order = append(f.order, id)
	if out.ExecErr != nil {
		f.setLocked(j, KaggleJobFailed, out.ExecErr.Error(), nil)
		return kaggle.Result{}, true, out.ExecErr
	}
	if j.compute && f.activeComputeLocked() > f.maxRemoteJobs {
		f.setLocked(j, KaggleJobFailed, "concurrent Kaggle compute limit reached", nil)
	}
	job := j.job
	return kaggle.Result{Job: &job}, true, nil
}

func (f *fakeKaggle) activeComputeLocked() int {
	n := 0
	for _, j := range f.jobs {
		if j.compute && KaggleJobActive(j.job.Status) {
			n++
		}
	}
	return n
}

func (f *fakeKaggle) setLocked(j *fakeJob, status, message string, result json.RawMessage) {
	j.job.Status, j.job.Error = status, message
	if result != nil {
		j.job.Result = result
	}
	f.seq++
	j.job.Updated = fakeStamp(f.seq)
}

func (f *fakeKaggle) Job(ctx context.Context, id string) (kaggle.Job, error) {
	if err := ctx.Err(); err != nil {
		return kaggle.Job{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(fakeCall{Method: "Job", ID: id})
	j, ok := f.jobs[id]
	if !ok {
		return kaggle.Job{}, sql.ErrNoRows
	}
	f.observeLocked(j)
	return j.job, nil
}

func (f *fakeKaggle) observeLocked(j *fakeJob) {
	switch j.job.Status {
	case KaggleJobQueued, KaggleJobRunning:
		if j.polls > 0 {
			j.polls--
			f.setLocked(j, KaggleJobRunning, "", nil)
			return
		}
		f.finishLocked(j)
	case KaggleJobRemoteRunning, KaggleJobCancelRequested:
		if k := f.kernels[j.kernel]; k != nil && k.jobID == j.job.ID {
			f.advanceKernelLocked(k)
		}
	}
}

func (f *fakeKaggle) finishLocked(j *fakeJob) {
	out := j.outcome
	switch out.Status {
	case KaggleJobUnknown:
		msg := out.Error
		if msg == "" {
			msg = "Kaggle outcome is unknown; this mutation was not retried"
		}
		f.setLocked(j, KaggleJobUnknown, msg, nil)
		return
	case KaggleJobFailed:
		msg := out.Error
		if msg == "" {
			msg = "Kaggle request failed (HTTP 500)"
		}
		f.setLocked(j, KaggleJobFailed, msg, nil)
		return
	case "", KaggleJobCompleted:
	default:
		f.t.Errorf("fakeOutcome.Status %q is not completed, failed or unknown", out.Status)
		f.setLocked(j, KaggleJobFailed, "bad fake script", nil)
		return
	}
	result, status, err := f.applyLocked(j)
	if err != nil {
		f.setLocked(j, KaggleJobFailed, err.Error(), nil)
		return
	}
	for k, v := range out.Result {
		result[k] = v
	}
	b, _ := json.Marshal(result)
	f.setLocked(j, status, "", b)
}

func fakeNotFound() error { return &kaggle.APIError{Status: 404} }

func (f *fakeKaggle) applyLocked(j *fakeJob) (map[string]any, string, error) {
	p := j.params
	switch j.op.Name {
	case "upload_dataset_file":
		if j.project != fakeProjectID {
			return nil, "", errors.New("invalid project directory")
		}
		b, err := os.ReadFile(filepath.Join(f.dir, j.project, j.file))
		if err != nil {
			return nil, "", fmt.Errorf("open %s: file does not exist", j.file)
		}
		sum := sha256.Sum256(b)
		f.uploads[j.job.ID] = fakeUpload{JobID: j.job.ID, Name: j.file, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(b))}
		return map[string]any{"upload_ref": "upload:" + j.job.ID, "bytes": len(b), "file": j.file}, KaggleJobCompleted, nil
	case "create_dataset":
		owner := fakeString(p, "ownerSlug")
		if owner == "" {
			owner = f.account
		}
		ref := owner + "/" + fakeString(p, "slug")
		if d := f.datasets[ref]; d != nil && !d.deleted {
			return nil, "", errors.New("Kaggle rejected the requested operation")
		}
		files, _ := p["files"].([]any)
		names := []string{}
		for _, item := range files {
			m, _ := item.(map[string]any)
			token := fakeString(m, "token")
			up, ok := f.uploads[strings.TrimPrefix(token, "upload:")]
			if !strings.HasPrefix(token, "upload:") || !ok {
				return nil, "", errors.New("Kaggle rejected the requested operation")
			}
			names = append(names, up.Name)
		}
		f.datasets[ref] = &fakeDataset{ref: ref, files: names}
		return map[string]any{"ref": ref, "url": KaggleDatasetURL(ref), "status": "Ok"}, KaggleJobCompleted, nil
	case "save_kernel":
		ref := fakeString(p, "slug")
		if strings.Count(ref, "/") != 1 {
			return nil, "", errors.New("notebook slug must be owner/name")
		}
		version := 1
		if k := f.kernels[ref]; k != nil && !k.deleted {
			version = k.version + 1
		}
		k := &fakeKernel{ref: ref, jobID: j.job.ID, version: version}
		status := KaggleJobCompleted
		if j.compute {
			status = KaggleJobRemoteRunning
			k.remote = append([]string(nil), j.outcome.Remote...)
			if len(k.remote) == 0 {
				k.remote = []string{"RUNNING", "COMPLETE"}
			}
		} else {
			k.status = "COMPLETE"
		}
		f.kernels[ref] = k
		j.kernel = ref
		f.kernelSeq++
		return map[string]any{"ref": ref, "url": KaggleNotebookURL(ref), "versionNumber": version,
			"kernelId": 9000 + f.kernelSeq}, status, nil
	case "download_kernel_output":
		if j.project == "" {
			return nil, "", errors.New("download requires a destination project")
		}
		if j.project != fakeProjectID {
			return nil, "", errors.New("invalid project directory")
		}
		name := j.file
		if name == "" {
			name = j.job.ID + ".zip"
		}
		body, ok := f.outputs[fakeString(p, "filePath")]
		if !ok {
			return nil, "", fakeNotFound()
		}
		dest := filepath.Join(f.dir, j.project, name)
		if _, err := os.Lstat(dest); err == nil {
			return nil, "", kaggle.ErrFileConflict
		}
		if err := os.WriteFile(dest, body, 0o600); err != nil {
			return nil, "", err
		}
		return map[string]any{"project_id": j.project, "file": name, "bytes": len(body)}, KaggleJobCompleted, nil
	case "delete_dataset":
		d := f.datasets[fakeString(p, "ownerSlug")+"/"+fakeString(p, "datasetSlug")]
		if d == nil || d.deleted {
			return nil, "", fakeNotFound()
		}
		d.deleted = true
	case "delete_kernel":
		k := f.kernels[fakeString(p, "userName")+"/"+fakeString(p, "kernelSlug")]
		if k == nil || k.deleted {
			return nil, "", fakeNotFound()
		}
		k.deleted = true
	}
	return map[string]any{}, KaggleJobCompleted, nil
}

func (f *fakeKaggle) advanceKernelLocked(k *fakeKernel) {
	if len(k.remote) > 0 {
		k.status = k.remote[0]
		if len(k.remote) > 1 {
			k.remote = k.remote[1:]
		}
	}
	j := f.jobs[k.jobID]
	if j == nil || (j.job.Status != KaggleJobRemoteRunning && j.job.Status != KaggleJobCancelRequested) {
		return
	}
	result := map[string]any{}
	_ = json.Unmarshal(j.job.Result, &result)
	result["remote_status"] = k.status
	b, _ := json.Marshal(result)
	status := j.job.Status
	if final := fakeRemoteState(k.status); final != "" {
		status = final
	}
	f.setLocked(j, status, j.job.Error, b)
}

// readLocked answers a read: the scripted answer first; otherwise the modelled default:
//   - get_dataset, get_dataset_status: a live dataset → {"ref","isPrivate":true} / {"status":"ready"}; else 404.
//     PROVISIONAL status vocabulary until the proof run.
//   - get_kernel: a live notebook → {"ref","isPrivate":true}; else 404.
//   - get_kernel_session_status: a live notebook (and matching versionLabel) → advances it one step and answers
//     {"status": <current>}; else 404.
//   - list_kernel_session_output: {"files":[{"fileName","size"}],"nextPageToken":""} from SetOutputs (PROVISIONAL shape).
//   - get_accelerator_quota_statistics: fakeQuotaJSON(27.5) (PROVISIONAL shape).
//   - anything else: {}.
func (f *fakeKaggle) readLocked(name string, p map[string]any) (json.RawMessage, error) {
	if answers := f.reads[name]; len(answers) > 0 {
		a := answers[0]
		if len(answers) > 1 {
			f.reads[name] = answers[1:]
		}
		if a.Err != nil {
			return nil, a.Err
		}
		return fakeJSON(a.Data), nil
	}
	switch name {
	case "get_dataset", "get_dataset_status":
		ref := fakeString(p, "ownerSlug") + "/" + fakeString(p, "datasetSlug")
		if d := f.datasets[ref]; d == nil || d.deleted {
			return nil, fakeNotFound()
		}
		if name == "get_dataset" {
			return fakeJSON(map[string]any{"ref": ref, "isPrivate": true}), nil
		}
		return fakeJSON(map[string]any{"status": "ready"}), nil
	case "get_kernel", "get_kernel_session_status":
		ref := fakeString(p, "userName") + "/" + fakeString(p, "kernelSlug")
		k := f.kernels[ref]
		if k == nil || k.deleted {
			return nil, fakeNotFound()
		}
		if name == "get_kernel" {
			return fakeJSON(map[string]any{"ref": ref, "isPrivate": true}), nil
		}
		if v := fakeString(p, "versionLabel"); v != "" && v != fmt.Sprint(k.version) {
			return nil, fakeNotFound()
		}
		f.advanceKernelLocked(k)
		status := k.status
		if status == "" {
			status = "QUEUED"
		}
		return fakeJSON(map[string]any{"status": status}), nil
	case "list_kernel_session_output":
		names := make([]string, 0, len(f.outputs))
		for n := range f.outputs {
			names = append(names, n)
		}
		sort.Strings(names)
		files := []map[string]any{}
		for _, n := range names {
			files = append(files, map[string]any{"fileName": n, "size": len(f.outputs[n])})
		}
		return fakeJSON(map[string]any{"files": files, "nextPageToken": ""}), nil
	case "get_accelerator_quota_statistics":
		return fakeJSON(fakeQuotaJSON(27.5)), nil
	}
	return json.RawMessage(`{}`), nil
}

func (f *fakeKaggle) ActiveComputeJobs(ctx context.Context) ([]kaggle.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(fakeCall{Method: "ActiveComputeJobs"})
	out := []kaggle.Job{}
	for _, id := range f.order {
		if j := f.jobs[id]; j.compute && KaggleJobActive(j.job.Status) {
			out = append(out, j.job)
		}
	}
	return out, nil
}

func (f *fakeKaggle) EnsureProject(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(fakeCall{Method: "EnsureProject", Err: f.projectErr})
	if f.projectErr != nil {
		return "", f.projectErr
	}
	if err := os.MkdirAll(filepath.Join(f.dir, fakeProjectID), 0o700); err != nil {
		return "", err
	}
	return fakeProjectID, nil
}

func (f *fakeKaggle) WriteFile(ctx context.Context, project, name string, r io.Reader) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	body, readErr := io.ReadAll(r) // read before locking: r may be a pipe fed by the caller
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "WriteFile", Project: project, File: name}
	fail := func(err error) (int64, error) {
		call.Err = err
		f.record(call)
		return 0, err
	}
	if err, ok := f.writeErrs[name]; ok {
		delete(f.writeErrs, name)
		return fail(err)
	}
	if readErr != nil {
		return fail(fmt.Errorf("read upload source: %w", readErr))
	}
	if project != fakeProjectID {
		return fail(errors.New("invalid project directory"))
	}
	if !fakeValidFile(name) {
		return fail(errors.New("invalid project filename"))
	}
	dest := filepath.Join(f.dir, project, name)
	if err := os.WriteFile(dest+".tmp", body, 0o600); err != nil {
		return fail(err)
	}
	if err := os.Rename(dest+".tmp", dest); err != nil {
		return fail(err)
	}
	f.record(call)
	return int64(len(body)), nil
}

func (f *fakeKaggle) OpenFile(project, name string) (*os.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "OpenFile", Project: project, File: name}
	var file *os.File
	var err error
	switch {
	case project != fakeProjectID:
		err = errors.New("invalid project directory")
	case !fakeValidFile(name):
		err = errors.New("invalid project filename")
	default:
		file, err = os.Open(filepath.Join(f.dir, project, name))
	}
	call.Err = err
	f.record(call)
	return file, err
}

func (f *fakeKaggle) RemoveFile(project, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "RemoveFile", Project: project, File: name}
	var err error
	switch {
	case project != fakeProjectID:
		err = errors.New("invalid project directory")
	case !fakeValidFile(name):
		err = errors.New("invalid project filename")
	default:
		if e := os.Remove(filepath.Join(f.dir, project, name)); e != nil && !errors.Is(e, fs.ErrNotExist) {
			err = e
		}
	}
	call.Err = err
	f.record(call)
	return err
}

func (f *fakeKaggle) ReadKernelLog(ctx context.Context, owner, slug string, version int, max int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "ReadKernelLog", File: owner + "/" + slug, Version: version, Max: max}
	if len(f.logs) == 0 {
		f.record(call)
		return nil, nil
	}
	l := f.logs[0]
	if len(f.logs) > 1 {
		f.logs = f.logs[1:]
	}
	call.Err = l.Err
	f.record(call)
	if l.Err != nil {
		return nil, l.Err
	}
	b := l.Body
	if max > 0 && int64(len(b)) > max {
		b = b[int64(len(b))-max:]
	}
	return append([]byte(nil), b...), nil
}

func (f *fakeKaggle) Cancel(ctx context.Context, jobID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "Cancel", ID: jobID}
	err := f.cancelLocked(jobID)
	call.Err = err
	f.record(call)
	return err
}

func (f *fakeKaggle) cancelLocked(id string) error {
	if len(f.cancelErrs) > 0 {
		err := f.cancelErrs[0]
		f.cancelErrs = f.cancelErrs[1:]
		return err
	}
	j, ok := f.jobs[id]
	if !ok {
		return sql.ErrNoRows
	}
	switch j.job.Status {
	case KaggleJobCancelRequested:
		return nil
	case KaggleJobQueued, KaggleJobRunning:
		f.setLocked(j, KaggleJobFailed, "context canceled", nil)
		return nil
	case KaggleJobRemoteRunning:
		r := map[string]any{}
		_ = json.Unmarshal(j.job.Result, &r)
		if r["kernelSessionId"] == nil {
			return errors.New("Kaggle did not return a verifiable session ID; cancel this run on Kaggle")
		}
		f.setLocked(j, KaggleJobCancelRequested, "", nil)
		return nil
	}
	return errors.New("job is not active")
}

// ---- helpers ---------------------------------------------------------------------------------------------

func fakeRandomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func fakeValidID(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// fakeValidFile copies kaggle.validFile (transfers.go:37-43).
func fakeValidFile(name string) bool {
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || len(stem) == 4 &&
		(strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9' {
		return false
	}
	return name != "" && name != "." && name != ".." && len(name) < 240 && !strings.ContainsAny(name, "/\\:\x00") &&
		!strings.HasPrefix(name, ".") && strings.TrimSpace(name) == name && !strings.HasSuffix(name, ".")
}

// fakeRemoteState copies kaggle.remoteState (remote_jobs.go:142-153).
func fakeRemoteState(v string) string {
	v = strings.ToUpper(v)
	switch {
	case v == "COMPLETE" || v == "COMPLETED" || strings.HasSuffix(v, "_SUCCEEDED") || strings.HasSuffix(v, "_COMPLETED"):
		return KaggleJobCompleted
	case v == "ERROR" || v == "FAILED" || strings.HasSuffix(v, "_FAILED") || strings.HasSuffix(v, "_ERRORED"):
		return KaggleJobFailed
	case v == "CANCELLED" || v == "CANCELED" || v == "CANCEL_ACKNOWLEDGED" || strings.HasSuffix(v, "_CANCELLED"):
		return KaggleJobCancelled
	}
	return ""
}

func fakeStamp(n int) string {
	return time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC).Add(time.Duration(n) * time.Second).Format(time.RFC3339Nano)
}

func fakeString(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func fakeJSON(v any) json.RawMessage {
	switch x := v.(type) {
	case json.RawMessage:
		return x
	case []byte:
		return x
	case string:
		return json.RawMessage(x)
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// ---- builders --------------------------------------------------------------------------------------------

// fakeQuotaJSON is a PROVISIONAL quota answer with the given remaining GPU hours (replace it with the proof
// run's real shape).
func fakeQuotaJSON(hours float64) map[string]any {
	used := 108000 - int(hours*3600)
	return map[string]any{"acceleratorQuotaStatistics": []any{
		map[string]any{"acceleratorType": "GPU", "quotaLimit": "108000s", "quotaUsed": fmt.Sprintf("%ds", used)},
		map[string]any{"acceleratorType": "TPU", "quotaLimit": "72000s", "quotaUsed": "0s"},
	}}
}

// fakeLogLine is one trainer line: "@@chibi " + {"v":1,"seq","ts","t", fields…}.
func fakeLogLine(seq int, t string, fields map[string]any) string {
	m := map[string]any{"v": 1, "seq": seq, "ts": fakeStamp(seq), "t": t}
	for k, v := range fields {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return "@@chibi " + string(b)
}

// fakeKernelLogJSON wraps lines in the JSON-array log form [{"stream_name","time","data"}].
func fakeKernelLogJSON(lines ...string) []byte {
	entries := make([]map[string]any, 0, len(lines))
	for i, l := range lines {
		entries = append(entries, map[string]any{"stream_name": "stdout", "time": 1.5 + float64(i), "data": l + "\n"})
	}
	b, _ := json.Marshal(entries)
	return b
}

// fakeKernelLogText is the plain-text log form.
func fakeKernelLogText(lines ...string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

// fakeTrainLog is a train stage start plus a metric line every 10 steps up to lastStep.
func fakeTrainLog(lastStep, maxSteps int) []string {
	lines := []string{fakeLogLine(1, "stage", map[string]any{"stage": "train", "state": "start"})}
	seq := 2
	for s := 10; s <= lastStep; s += 10 {
		frac := float64(s) / float64(maxSteps)
		lines = append(lines, fakeLogLine(seq, "metric", map[string]any{"step": s, "max_steps": maxSteps, "epoch": frac,
			"loss": 2.0 - frac, "lr": 0.0002, "grad_norm": 1.0, "tok_s": 2400.0, "elapsed_s": float64(s),
			"eta_s": float64(maxSteps - s), "vram_gb": 6.1}))
		seq++
	}
	return lines
}

// fakeRunOutputs describes a set of kernel outputs.
type fakeRunOutputs struct {
	RunID, ProfileID, ProfileSHA string
	Failed                       bool   // status failed with Code and Message; no GGUF, samples or adapter
	Code, Message                string
	StopReason                   string // default "done"
	Steps                        int    // default 40
	Arch                         string // GGUF architecture, default "gemma3"
	BaseGGUF                     bool   // also chibi-base-q8_0.gguf
	Adapter                      bool   // also chibi-adapter-model.safetensors and chibi-adapter-config.json
	Samples                      []fakeSample
	WrongSHA                     string         // manifest path whose sha256 is replaced by 64 zeros
	Extra                        map[string]any // merged into the status object (for example "warning_code")
}

type fakeSample struct{ Expected, Base, Tuned string }

// fakeOutputs builds the output files of one run, keyed by file name, with a consistent manifest.
func fakeOutputs(o fakeRunOutputs) map[string][]byte {
	if o.StopReason == "" {
		o.StopReason = "done"
	}
	if o.Steps == 0 {
		o.Steps = 40
	}
	if o.Arch == "" {
		o.Arch = "gemma3"
	}
	if len(o.Samples) == 0 {
		o.Samples = []fakeSample{{"pos", "neg", "pos"}, {"neg", "neg", "neg"}, {"pos", "pos", "pos"}}
	}
	files, kinds := map[string][]byte{}, map[string]string{}
	add := func(name, kind string, b []byte) { files[name], kinds[name] = b, kind }
	add("chibi-metrics.jsonl", "metrics", fakeMetricsJSONL(o.Steps))
	if !o.Failed {
		add("chibi-eval_samples.jsonl", "eval_samples", fakeEvalSamplesJSONL(o.Samples))
		add("chibi-model-q8_0.gguf", "gguf_tuned", fakeGGUF(o.Arch))
		if o.BaseGGUF {
			add("chibi-base-q8_0.gguf", "gguf_base", fakeGGUF(o.Arch))
		}
		if o.Adapter {
			add("chibi-adapter-model.safetensors", "adapter", []byte("fake-safetensors"))
			add("chibi-adapter-config.json", "adapter_config", []byte(`{"r":16,"lora_alpha":32}`))
		}
	}
	add("chibi-status.json", "status", fakeStatusJSON(o))
	files["chibi-manifest.json"] = fakeManifestJSON(o, files, kinds)
	return files
}

func fakeStatusJSON(o fakeRunOutputs) []byte {
	train := map[string]any{"steps": o.Steps, "epochs": 2, "train_loss": 0.92, "eval_loss_base": 2.31,
		"eval_loss_tuned": 1.12, "stop_reason": o.StopReason, "truncated_rows": 0}
	st := map[string]any{"v": 1, "run_id": o.RunID, "state": "succeeded", "stage": "manifest", "code": "", "message": "",
		"started": "2026-11-23T10:00:00Z", "finished": "2026-11-23T10:30:00Z",
		"gpu":      map[string]any{"name": "Tesla T4", "capability": "7.5", "vram_gb": 15.0},
		"versions": map[string]any{"python": "3.11.13", "torch": "2.6.0", "transformers": "4.57.0", "peft": "0.17.1", "llama_cpp": "b9994"},
		"train":    train,
		"discovery": map[string]any{"gpu_name": "Tesla T4", "lora_targets": []string{"q_proj", "v_proj"},
			"converter_env": "system"}}
	if o.Failed {
		st["state"], st["stage"], st["code"], st["message"] = "failed", "train", o.Code, o.Message
		train["eval_loss_tuned"] = nil
	}
	for k, v := range o.Extra {
		st[k] = v
	}
	b, _ := json.Marshal(st)
	return b
}

func fakeManifestJSON(o fakeRunOutputs, files map[string][]byte, kinds map[string]string) []byte {
	names := make([]string, 0, len(kinds))
	for n := range kinds {
		names = append(names, n)
	}
	sort.Strings(names)
	list := []map[string]any{}
	for _, n := range names {
		sum := sha256.Sum256(files[n])
		sha := hex.EncodeToString(sum[:])
		if n == o.WrongSHA {
			sha = strings.Repeat("0", 64)
		}
		list = append(list, map[string]any{"path": n, "kind": kinds[n], "bytes": len(files[n]), "sha256": sha})
	}
	b, _ := json.Marshal(map[string]any{"v": 1, "run_id": o.RunID, "profile_id": o.ProfileID,
		"profile_sha": o.ProfileSHA, "files": list})
	return b
}

// fakeMetricsJSONL writes train metric objects (the log objects without the prefix), one every 10 steps.
func fakeMetricsJSONL(steps int) []byte {
	var buf bytes.Buffer
	for i, line := range fakeTrainLog(steps, steps)[1:] {
		_ = i
		buf.WriteString(strings.TrimPrefix(line, "@@chibi "))
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func fakeEvalSamplesJSONL(samples []fakeSample) []byte {
	var buf bytes.Buffer
	for i, s := range samples {
		b, _ := json.Marshal(map[string]any{"id": i + 1,
			"prompt_messages": []map[string]string{{"role": "user", "content": fmt.Sprintf("question %d", i+1)}},
			"expected": s.Expected, "base_output": s.Base, "tuned_output": s.Tuned, "base_ms": 120, "tuned_ms": 110,
			"truncated": false})
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// fakeGGUF is a minimal valid GGUF v3 file: magic, version 3, one tensor, the key general.architecture.
func fakeGGUF(arch string) []byte {
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	str := func(s string) { w(uint64(len(s))); b.WriteString(s) }
	b.WriteString("GGUF")
	w(uint32(3)) // version
	w(uint64(1)) // tensor count
	w(uint64(1)) // metadata count
	str("general.architecture")
	w(uint32(8)) // GGUF_TYPE_STRING
	str(arch)
	str("t")     // tensor name
	w(uint32(1)) // n_dims
	w(uint64(1)) // dim 0
	w(uint32(0)) // GGML_TYPE_F32
	w(uint64(0)) // offset
	for b.Len()%32 != 0 {
		b.WriteByte(0)
	}
	w(float32(0))
	return b.Bytes()
}

// ---- self-tests ------------------------------------------------------------------------------------------

func fakeExec(t *testing.T, f *fakeKaggle, id, op, project, file string, params any) (kaggle.Result, error) {
	t.Helper()
	req, err := NewKaggleRequest(id, op, project, file, params)
	if err != nil {
		t.Fatal(err)
	}
	return f.Execute(context.Background(), req)
}

func TestFakeKaggleReplayUnknownAndSlots(t *testing.T) {
	ctx := context.Background()
	f := newFakeKaggle(t)
	proj, _ := f.EnsureProject(ctx)
	if _, err := f.WriteFile(ctx, proj, "r1-train.jsonl", strings.NewReader("{}\n")); err != nil {
		t.Fatal(err)
	}
	f.Script("upload_dataset_file", fakeOutcome{Status: KaggleJobUnknown})
	id := KaggleRequestID("run", "upload_train", 0)
	res, err := fakeExec(t, f, id, "upload_dataset_file", proj, "r1-train.jsonl", nil)
	if err != nil || res.Job.Status != KaggleJobQueued {
		t.Fatalf("admit: %+v %v", res.Job, err)
	}
	if j, _ := f.Job(ctx, id); j.Status != KaggleJobUnknown {
		t.Fatalf("status %q, want unknown", j.Status)
	}
	res, err = fakeExec(t, f, id, "upload_dataset_file", proj, "r1-train.jsonl", nil)
	if err != nil || res.Job.Status != KaggleJobUnknown || len(f.Replays("upload_dataset_file")) != 1 {
		t.Fatalf("replay: %+v %v", res.Job, err)
	}
	if _, err := fakeExec(t, f, id, "upload_dataset_file", proj, "other.jsonl", nil); err == nil {
		t.Fatal("same ID with another request was accepted")
	}
	f.AddForeignComputeJob(KaggleJobUnknown)
	res, _ = fakeExec(t, f, KaggleRequestID("run", "save_kernel", 0), "save_kernel", "", "", map[string]any{
		"slug": "tester/chibi-x", "kernelExecutionType": "SAVE_AND_RUN_ALL", "enableGpu": true})
	if res.Job.Status != KaggleJobFailed || res.Job.Error != "concurrent Kaggle compute limit reached" {
		t.Fatalf("slot: %+v", res.Job)
	}
	assertMutations(t, f, map[string]int{"upload_dataset_file": 1, "save_kernel": 1})
}
```

  Add two more self-tests to the same file:
  - `TestFakeKaggleRemoteSequenceAndCancel`:
    - script `save_kernel` with `Remote: []string{"QUEUED","RUNNING","RUNNING","COMPLETE"}`; admit with SAVE_AND_RUN_ALL;
    - Job reads give `remote_running`, then (with `remote_status` QUEUED, RUNNING, RUNNING) `remote_running`, then `completed` with `remote_status` COMPLETE;
    - `Kernel("tester/chibi-x")` gives version 1;
    - a second job with `Remote: {"RUNNING"}`: `Cancel` returns the "verifiable session ID" error and the job stays `remote_running`;
    - a third job with `Result: {"kernelSessionId": 7}`: `Cancel` → nil and `cancel_requested`, then scripted `CANCEL_ACKNOWLEDGED` → `cancelled`;
    - `FailCancel(errors.New("x"))` makes the next Cancel fail;
    - `RestartService` turns a fresh queued upload into `unknown` and leaves the remote job untouched.
  - `TestFakeKaggleDatasetDownloadAndBuilders`:
    - upload two files, run `create_dataset` with their `upload_ref` tokens: `Dataset("tester/chibi-data-x")` lists both names; `get_dataset_status` answers `{"status":"ready"}`; `get_dataset` of an unknown slug fails with `IsKaggleNotFound`;
    - `SetOutputs(fakeOutputs(fakeRunOutputs{RunID:"r", WrongSHA:"chibi-model-q8_0.gguf"}))`: `download_kernel_output` of `chibi-status.json` into `r1-chibi-status.json` completes and `OpenFile` reads bytes equal to the output; a second download into the same name fails with "project file revision changed"; a missing output fails with "Kaggle request failed (HTTP 404)";
    - the manifest entry of the GGUF has 64 zeros, the others match their sha256;
    - `fakeGGUF("gemma3")` starts with `GGUF`, version 3, and its length is a multiple of 4;
    - `ReadKernelLog` with `ScriptLogs(fakeLog{Body: []byte("abcdef")})` and max 3 returns `def`;
    - `ParseGPUQuota(fakeJSON(fakeQuotaJSON(27.5)))` returns 27.5, true.

- [ ] 12. Create `internal/chibisensei/runs_test.go`. Helpers and the main tests in full:

```go
package chibisensei

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func newTestRun(t *testing.T, st *Store, projectID string) Run {
	t.Helper()
	id := uuid.NewString()
	r := Run{ID: id, ProjectID: projectID, ProfileID: "gemma3-270m", ProfileSHA: "abc", Preset: "quick",
		Hyper: Hyper{Epochs: 1, LearningRate: 5e-5, PerDeviceBatch: 4, GradAccum: 4, MaxSeqLen: 512},
		Remote: RunRemote{ProfileStatus: "experimental", Method: "full", Split: PlanSplit{Train: 40, Eval: 10},
			EstimateMinutes: 30, SessionTimeoutMinutes: 60, Owner: "tester",
			DatasetSlug: DatasetSlug(id), NotebookSlug: NotebookSlug(projectID, id)}}
	if err := st.CreateRun(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetRun(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func setRunState(t *testing.T, st *Store, id, state string) Run {
	t.Helper()
	r, err := st.UpdateRun(context.Background(), id, NewRunEvent("state", map[string]any{"state": state}),
		func(r *Run) error { r.State = state; return nil })
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRunCreateOneActiveAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p1, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	p2, _ := st.CreateProject(ctx, ProjectInput{Name: "B", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p1.ID)
	if r.State != RunQueued || r.Backend != BackendKaggle || r.Remote.Live != LiveUnknown || r.JobJSON != nil ||
		len(r.Leftovers) != 0 || r.Hyper.MaxSeqLen != 512 || r.Remote.Method != "full" {
		t.Fatalf("round trip: %+v", r)
	}
	other := Run{ID: uuid.NewString(), ProjectID: p2.ID}
	if err := st.CreateRun(ctx, other); !errors.Is(err, ErrRunActive) {
		t.Fatalf("second active run: %v", err)
	}
	if err := st.CreateRun(ctx, Run{ID: uuid.NewString(), ProjectID: uuid.NewString()}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown project: %v", err)
	}
	active, _ := st.ActiveRuns(ctx)
	if len(active) != 1 || active[0].ID != r.ID {
		t.Fatalf("active: %+v", active)
	}
	setRunState(t, st, r.ID, RunFailed)
	if err := st.CreateRun(ctx, other); err != nil {
		t.Fatalf("after terminal: %v", err)
	}
	ev, _ := st.ListRunEvents(ctx, r.ID, 0, 0)
	if len(ev) != 2 || ev[0].Kind != "state" || string(ev[0].Data) != `{"state":"queued"}` {
		t.Fatalf("events: %+v", ev)
	}
}

func TestUpdateRunIdentityRollbackAndSilentUpdate(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	got, err := st.UpdateRun(ctx, r.ID, RunEvent{}, func(x *Run) error {
		x.ID, x.ProjectID = "changed", "changed"
		x.Remote.LastLogSeq = 42
		x.Remote.PutStep(KaggleStep{Step: StepSaveKernel, RequestID: KaggleRequestID(r.ID, StepSaveKernel, 0)})
		x.JobJSON = []byte(`{"v":1}`)
		return nil
	})
	if err != nil || got.ID != r.ID || got.ProjectID != p.ID {
		t.Fatalf("identity: %+v %v", got, err)
	}
	again, _ := st.GetRun(ctx, r.ID)
	if again.Remote.LastLogSeq != 42 || again.Remote.Step(StepSaveKernel) == nil || string(again.JobJSON) != `{"v":1}` {
		t.Fatalf("stored: %+v", again.Remote)
	}
	if ev, _ := st.ListRunEvents(ctx, r.ID, 0, 0); len(ev) != 1 {
		t.Fatalf("silent update wrote an event: %d", len(ev))
	}
	boom := errors.New("boom")
	if _, err := st.UpdateRun(ctx, r.ID, NewRunEvent("warn", nil), func(x *Run) error { x.State = RunFailed; return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if again, _ = st.GetRun(ctx, r.ID); again.State != RunQueued {
		t.Fatalf("rollback: %s", again.State)
	}
	if _, err := st.UpdateRun(ctx, r.ID, RunEvent{}, func(x *Run) error { x.State = "bogus"; return nil }); err == nil {
		t.Fatal("unknown state accepted")
	}
	if _, err := st.UpdateRun(ctx, uuid.NewString(), RunEvent{}, func(*Run) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestRunJSONShape(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "A", Recipe: RecipeAssistant})
	r := newTestRun(t, st, p.ID)
	decode := func(v any) map[string]any {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		return m
	}
	full := decode(r)
	for _, k := range []string{"stage", "cancel", "result"} {
		if v, ok := full[k]; !ok || v != nil {
			t.Errorf("%s = %v (present %v), want null", k, v, ok)
		}
	}
	if l, ok := full["leftovers"].([]any); !ok || len(l) != 0 {
		t.Errorf("leftovers = %v", full["leftovers"])
	}
	remote := full["remote"].(map[string]any)
	if jobs, ok := remote["kaggle_jobs"].([]any); !ok || len(jobs) != 0 || full["hyper"] == nil {
		t.Errorf("full form: %v", full)
	}
	loss := 0.5
	r.CancelRequested = true
	r.Remote.Submitted = "2026-11-22T12:00:00Z"
	r.Remote.Result = &RunResult{Steps: 10, TrainLoss: &loss}
	r.Remote.PutStep(KaggleStep{Step: StepSaveKernel, JobID: "ab"})
	full = decode(r)
	c := full["cancel"].(map[string]any)
	if c["stop_url"] != "https://www.kaggle.com/code/tester/"+r.Remote.NotebookSlug || c["time_limit_minutes"].(float64) != 60 {
		t.Errorf("cancel = %v", c)
	}
	if res := full["result"].(map[string]any); res["versions"] == nil || res["eval_loss_base"] != nil {
		t.Errorf("result = %v", res)
	}
	sum := decode(r.Summary())
	if _, ok := sum["hyper"]; ok {
		t.Error("summary has hyper")
	}
	if _, ok := sum["remote"].(map[string]any)["kaggle_jobs"]; ok {
		t.Error("summary has kaggle_jobs")
	}
	if _, ok := sum["result"].(map[string]any)["versions"]; ok {
		t.Error("summary has result.versions")
	}
}
```

  Remaining tests in `runs_test.go`, with exact assertions:
  - `TestRunEventsCap`:
    - `old := maxRunEvents; maxRunEvents = 10; t.Cleanup(func(){ maxRunEvents = old })`;
    - add 15 `log` events via `AddRunEvent` (data `{"n":i}`): `ListRunEvents(id, 0, 0)` returns 10 events, the first with data `{"n":5}`, and ids strictly increasing;
    - `ListRunEvents(id, events[7].ID, 0)` returns 2;
    - `AddRunEvent` with `Data: []byte("{")` errors.
  - `TestMetricsUpsertReplaceList`:
    - upsert `{Step:10,Kind:"train",Loss:2}`, `{Step:10,Kind:"eval",Loss:2.5}`, `{Step:20,Kind:"train",Loss:math.NaN()}` and `{Step:30,Kind:"bogus",Loss:1}`: `ListMetrics(id,0)` has 2 points;
    - upserting step 10 train with loss 1.5 updates it;
    - `ListMetrics(id,10)` includes both step-10 points (inclusive);
    - `ReplaceMetrics` with one point leaves exactly that point;
    - `UpsertMetrics(nil)` is a no-op.
  - `TestArtifactsReportsConsentsSettings`:
    - `AddArtifact` with an empty ID stores `ArtifactID(run, "gguf_tuned")` (`ValidID` true, deterministic); `Name == "model-q8_0.gguf"` for `models/<run>/model-q8_0.gguf`;
    - adding the same kind again replaces it (`ListArtifacts` length 1, new `Bytes`);
    - kind `bogus`, path `../x` and path `a\b` are refused;
    - `GetArtifact(unknown)` returns `ErrNotFound`;
    - `SaveReport(run,"auto",Report{Kind:"auto"})`, then save again with `Samples: 3`: `GetReport` returns `samples` 3 and found true;
    - `GetReport(run,"judge")` returns found false; kind `x` is refused;
    - `RecordConsent` twice (move `st.now` one hour between the calls) returns the same `AcceptedAt`; `HasConsent` true for `1.0` and false for `2.0`; `ListConsents` length 1;
    - `GetSetting("kaggle_project_id")` returns found false; `SetSetting` twice keeps the last value.
  - `TestDeleteRunAndProjectGuards`:
    - `DeleteRun` of a queued run returns `ErrRunNotTerminal`;
    - archive the project; `DeleteProject` returns `ErrRunActive`;
    - set the run `cancelled`, add an event, a metric, an artifact and a report; `ListRunIDs` returns `[run]`;
    - `DeleteRun` removes all of them (`GetRun` gives `ErrNotFound`; `ListRunEvents`, `ListMetrics` and `ListArtifacts` are empty; `GetReport` found false); `DeleteRun` again gives `ErrNotFound`;
    - `DeleteProject` now succeeds.
  - `TestComputeStepsTraining` (table over `computeSteps`, with `rows: 10`, `checkRev: {DataRev, true}`, project `GoalText`/`ProfileID` set):

    | case | ready | runs | want |
    |---|---|---|---|
    | none, data ready | true | {} | `ready` |
    | none, data not ready | false | {} | `todo` |
    | active | false | {Latest: running} | `in_progress` |
    | failed | true | {Latest: failed} | `blocked` |
    | cancelled | true | {Latest: cancelled} | `blocked` |
    | succeeded | true | {Latest: succeeded, Succeeded: true} | `done` |
    | failed after success | true | {Latest: failed, Succeeded: true} | `done` |
    | active after success | true | {Latest: queued, Succeeded: true} | `in_progress` |

  - `TestProjectTrainingStepFromStore`:
    - create a project; `newTestRun`; `GetProject(...).Steps.Training == "in_progress"`; `ListProjects(false)[0].Steps.Training == "in_progress"`;
    - set `failed`: `blocked`;
    - a second run set `succeeded`: `done`.
  - Also update `TestProjectLifecycle` in `store_test.go`? No change needed: its expectations (`todo`) still hold with no runs.

- [ ] 13. Create `internal/chibisensei/plantoken_test.go`, `preflight_test.go` and `report_test.go`.

  `plantoken_test.go`:

```go
package chibisensei

import (
	"testing"
	"time"
)

func TestPlanTokens(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	p := NewPlanTokens(func() time.Time { return now })
	tok, ttl := p.Issue(PlanTicket{ProjectID: "p1", RunID: "r1", Preset: "quick", Hash: "h"})
	if len(tok) != 32 || ttl != PlanTokenTTL {
		t.Fatalf("issue: %q %v", tok, ttl)
	}
	if _, ok := p.Consume(tok, "p2"); ok {
		t.Fatal("wrong project accepted")
	}
	if _, ok := p.Consume(tok, "p1"); ok {
		t.Fatal("token survived a failed consume")
	}
	tok, _ = p.Issue(PlanTicket{ProjectID: "p1", RunID: "r1", Hash: "h"})
	got, ok := p.Consume(tok, "p1")
	if !ok || got.RunID != "r1" || got.Hash != "h" || !got.Expires.Equal(now.Add(PlanTokenTTL)) {
		t.Fatalf("consume: %+v %v", got, ok)
	}
	if _, ok := p.Consume(tok, "p1"); ok {
		t.Fatal("token consumed twice")
	}
	tok, _ = p.Issue(PlanTicket{ProjectID: "p1"})
	now = now.Add(PlanTokenTTL + time.Second)
	if _, ok := p.Consume(tok, "p1"); ok {
		t.Fatal("expired token accepted")
	}
	first, _ := p.Issue(PlanTicket{ProjectID: "p1"})
	for i := 0; i < maxPlanTokens; i++ {
		now = now.Add(time.Millisecond)
		p.Issue(PlanTicket{ProjectID: "p1"})
	}
	if _, ok := p.Consume(first, "p1"); ok {
		t.Fatal("the earliest-expiring token was not evicted")
	}
}
```

  `TestPlanHash` (same file):
  - `base := PlanHash(plan, "ps", "ss", "tester", "NvidiaTeslaT4", true)` with a small `Plan` literal: it is 64 hex characters and stable across two calls;
  - each change gives a different hash: `plan.DataRev++`, `plan.Hyper.Epochs++`, profile SHA, script SHA, owner, machine shape (`NvidiaL4`), delete flag.

  `preflight_test.go`:

```go
package chibisensei

import (
	"errors"
	"reflect"
	"sort"
	"testing"
)

func okPreflightInput() PreflightInput {
	return PreflightInput{DataRev: 3, CheckPresent: true, CheckDataRev: 3, CheckReady: true,
		KaggleEnabled: true, AllowCreate: true, AllowRemoteExecute: true, AllowGPU: true, AllowInternet: true,
		AllowDownload: true, AllowDelete: true, AllowedMachineShapes: []string{"NvidiaTeslaT4"}, RemoteTimeoutMinutes: 120,
		MaxTransferMB: 2048, MaxStorageMB: 10240, MachineShape: "NvidiaTeslaT4", DeleteRemoteAfterRun: true,
		Network: true, Write: true, VaultAvailable: true, CredentialsStored: true, ConnectionTested: true,
		ConnectionOK: true, Account: "tester", QuotaRead: true, QuotaKnown: true, QuotaHours: 27.5, JobsRead: true,
		EstimateMinutes: 34, ArtifactsEstimatedMB: 650, LargestArtifactMB: 300, DiskFreeKnown: true,
		DiskFreeBytes: 50 << 30, StorageMaxGB: 20, ConsentRequired: true, ConsentRecorded: true,
		LicenseID: "gemma", LicenseVersion: "1.0"}
}

var preflightOrder = []string{"dataset_ready", "run_active", "kaggle_enabled", "kaggle_not_readonly", "kaggle_allow_create",
	"kaggle_allow_remote_execute", "kaggle_allow_gpu", "kaggle_allow_internet", "kaggle_allow_download",
	"kaggle_machine_shape", "kaggle_allow_delete", "network_access", "filesystem_write", "kaggle_vault",
	"kaggle_credentials", "kaggle_connection", "kaggle_timeout", "kaggle_quota", "kaggle_slot", "kaggle_transfer",
	"disk_space", "license_consent"}

func failing(p Preflight) []string {
	var out []string
	for _, it := range p.Items {
		if !it.OK {
			out = append(out, it.ID)
		}
	}
	sort.Strings(out)
	return out
}

func TestEvaluatePreflightAllOKAndOrder(t *testing.T) {
	p := EvaluatePreflight(okPreflightInput())
	if !p.Ready || len(failing(p)) != 0 {
		t.Fatalf("not ready: %v", failing(p))
	}
	var ids []string
	for _, it := range p.Items {
		ids = append(ids, it.ID)
		if it.MessageKey != "chibi_sensei.preflight."+it.ID {
			t.Errorf("%s key %s", it.ID, it.MessageKey)
		}
	}
	if !reflect.DeepEqual(ids, preflightOrder) {
		t.Fatalf("order = %v", ids)
	}
	if p.FixFlags() != nil {
		t.Fatalf("fix flags on a ready preflight: %v", p.FixFlags())
	}
}

func TestEvaluatePreflightCases(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*PreflightInput)
		fail  []string
		ready bool
	}{
		{"no check", func(in *PreflightInput) { in.CheckPresent = false }, []string{"dataset_ready"}, false},
		{"stale check", func(in *PreflightInput) { in.CheckDataRev = 2 }, []string{"dataset_ready"}, false},
		{"kaggle off", func(in *PreflightInput) { in.KaggleEnabled = false; in.ConnectionTested = false },
			[]string{"kaggle_connection", "kaggle_enabled"}, false},
		{"delete not granted", func(in *PreflightInput) { in.AllowDelete = false }, []string{"kaggle_allow_delete"}, true},
		{"quota unknown", func(in *PreflightInput) { in.QuotaKnown = false }, []string{"kaggle_quota"}, true},
		{"timeout short", func(in *PreflightInput) { in.RemoteTimeoutMinutes = 60 }, []string{"kaggle_timeout"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := okPreflightInput()
			c.edit(&in)
			p := EvaluatePreflight(in)
			want := append([]string(nil), c.fail...)
			sort.Strings(want)
			if got := failing(p); !reflect.DeepEqual(got, want) || p.Ready != c.ready {
				t.Fatalf("failing %v ready %v, want %v %v", got, p.Ready, want, c.ready)
			}
		})
	}
}
```

  Add these rows to the table (each changes one field of `okPreflightInput`):
  - `ActiveRunID:"r"` → `run_active`, not ready;
  - `KaggleReadOnly:true` → `kaggle_not_readonly` and `kaggle_connection`, not ready;
  - each of `AllowCreate`, `AllowRemoteExecute`, `AllowGPU`, `AllowInternet`, `AllowDownload` set false → its item plus `kaggle_connection`, not ready;
  - `AllowedMachineShapes:nil` → `kaggle_machine_shape` and `kaggle_connection`;
  - `AllowDelete:false, DeleteRemoteAfterRun:false` → none failing, ready;
  - `Network:false` → `network_access` and `kaggle_connection`;
  - `Write:false` → `filesystem_write` and `kaggle_connection`;
  - `VaultAvailable:false` → `kaggle_vault` and `kaggle_connection`;
  - `CredentialsStored:false` → `kaggle_credentials` and `kaggle_connection`;
  - `ConnectionOK:false` → `kaggle_connection`;
  - `QuotaHours:0.5` → `kaggle_quota`, not ready;
  - `QuotaRead:false` → `kaggle_quota`, ready;
  - `ComputeJobs:1, UnknownJobs:1` → `kaggle_slot`;
  - `JobsRead:false` → `kaggle_slot`;
  - `MaxTransferMB:100` → `kaggle_transfer`;
  - `KaggleStorageUsedBytes: 10240<<20` → `kaggle_transfer`;
  - `DiskFreeBytes: 100<<20` → `disk_space`;
  - `StorageUsedBytes: 20<<30` → `disk_space`;
  - `DiskFreeKnown:false` → `disk_space`;
  - `ConsentRecorded:false` → `license_consent`;
  - `ConsentRequired:false, ConsentRecorded:false` → none, ready.

  Further tests in the same file:
  - `TestPreflightArgsAndFix`:
    - with `KaggleEnabled=false, AllowGPU=false, ConnectionTested=false`: `FixFlags()` equals `["kaggle.enabled","kaggle.allow_gpu"]`;
    - the `kaggle_connection` item has args `{"skipped":true}`;
    - `kaggle_credentials` has `Link == "/config#kaggle"`;
    - `kaggle_timeout` args are `{"minutes":120,"needed":66}`;
    - `kaggle_quota` args are `{"hours":27.5,"needed":0.7}`;
    - `KaggleReachable` is false for this input and true for `okPreflightInput()`.
  - `TestPreflightFlagPatch`:
    - flags `["kaggle.enabled","kaggle.read_only","kaggle.allow_gpu","kaggle.allowed_machine_shapes"]` with current `["NvidiaL4"]` and shape `NvidiaTeslaT4` give exactly `{"kaggle":{"enabled":true,"read_only":false,"allow_gpu":true,"allowed_machine_shapes":[]any{"NvidiaL4","NvidiaTeslaT4"}}}`;
    - a current list that already contains the shape is not duplicated;
    - `kaggle.allow_tpu`, an empty list and shape `"X"` each return an error that `errors.Is(err, ErrUnknownPreflightFlag)`;
    - every entry of `PreflightFlags` is accepted alone.
  - `TestParseGPUQuota` (table; mark the file's fixtures `// PROVISIONAL until the proof run`):
    - `fakeJSON(fakeQuotaJSON(27.5))` → 27.5, true;
    - `{"gpu":{"remainingHours":12.5,"totalHours":30}}` → 12.5;
    - `{"gpuQuotaRemaining":99000}` → 27.5;
    - `{"quotas":[{"type":"GPU","limitSeconds":108000,"usedSeconds":120000}]}` → 0;
    - `{"accelerators":[{"name":"NvidiaTeslaT4","remainingSeconds":"36000"}]}` → 10;
    - `{"gpu":{"remainingMinutes":90}}` → 1.5;
    - `{"tpu":{"remaining":20}}` → false;
    - `{}` → false;
    - `not json` → false;
    - two GPU entries with 5 and 9 remaining hours → 5.
  - `TestNewKaggleRequestOperations`:
    - for every key of `kaggleOpDomains`, `kaggle.Lookup(domain, op)` succeeds;
    - the grant is `create` for `upload_dataset_file` and `create_dataset`; `update` for `save_kernel`; `download` for `download_kernel_output`; `delete` for both deletes; `read` for the rest;
    - `NewKaggleRequest("", "cancel_kernel_session", …)` errors;
    - `KaggleRequestID("r","s",0)` is 32 hex characters and differs from attempt 1.

  `report_test.go`. B2 dependency: if `Status.Train` differs from the assumed form, adapt `testStatus` only.

```go
package chibisensei

import (
	"encoding/json"
	"reflect"
	"testing"
)

func fp(v float64) *float64 { return &v }

func testStatus(base, tuned *float64) Status {
	var st Status
	st.Train.EvalLossBase, st.Train.EvalLossTuned = base, tuned
	return st
}

func samples(rows ...[3]string) []EvalSample {
	out := make([]EvalSample, 0, len(rows))
	for _, r := range rows {
		out = append(out, EvalSample{Expected: r[0], BaseOutput: r[1], TunedOutput: r[2]})
	}
	return out
}

func TestScoreReportClassify(t *testing.T) {
	rep := ScoreReport(RecipeSpecialist, "classify", []string{"pos", "neg"}, testStatus(fp(2.31), fp(1.12)), samples(
		[3]string{"pos", "pos", "pos"}, [3]string{"neg", "pos", "neg"},
		[3]string{"pos", "Positive", "pos."}, [3]string{"neg", "neg", "NEG"}))
	la := rep.LabelAccuracy
	if la == nil || la.N != 4 || la.Base != 0.5 || la.Tuned != 1 || rep.ExactMatch != nil || rep.Perplexity != nil {
		t.Fatalf("report: %+v", rep)
	}
	if !reflect.DeepEqual(la.Confusion.Base, [][]int{{1, 0, 1}, {1, 1, 0}}) ||
		!reflect.DeepEqual(la.Confusion.Tuned, [][]int{{2, 0, 0}, {0, 2, 0}}) {
		t.Fatalf("confusion: %+v", la.Confusion)
	}
	if !rep.SmallSample || rep.Samples != 4 || *rep.Loss.Base != 2.31 {
		t.Fatalf("meta: %+v", rep)
	}
	if _, err := json.Marshal(rep); err != nil {
		t.Fatal(err)
	}
}
```

  `TestScoreReportCases` (table), with assertions per row:
  - assistant recipe, 3 samples: only `Loss` non-nil; `SmallSample` true.
  - `extract`: `{"Berlin"," berlin","Berlin  "}`, `{"Paris","Paris","Paris"}` → base 0.5, tuned 1, n 2 (case kept, whitespace normalized).
  - `rewrite` NFC: expected `"Café"` (composed é typed directly), tuned `"Cafe" + string(rune(0x301))` (decomposed, built without a `\u` escape) → tuned 1.
  - `to_json`:
    - expected `{"a":1,"b":[1,2]}`, tuned `{ "b": [1, 2], "a": 1.0 }` → equal;
    - base `{"a":1}` → not equal;
    - base `not json` → not equal;
    - expected `plain` (not JSON) with tuned ` plain ` → equal by string;
    - so base 0, tuned 1, n 2.
  - free: losses 2 and 1 → perplexity base 7.389, tuned 2.718; `ExactMatch` and `LabelAccuracy` nil.
  - free with a nil tuned loss → `Perplexity.Tuned == nil` and `Loss.Tuned == nil`.
  - both losses nil → `Loss == nil`.
  - loss 1000 (exp overflows) → `Perplexity` nil.
  - 30 samples → `SmallSample` false.
  - classify with no labels → `LabelAccuracy` nil.
  - a sample whose expected is not a label is excluded from `n`.

- [ ] 14. DOX hand-off (B5 owns the `AGENTS.md` files). Put these facts in the implementer report for B5 to record in `internal/chibisensei/AGENTS.md`:
  - Ownership gains `runs.go` / `runstore.go` (run model, view, run store), `consents.go`, `plantoken.go`, `preflight.go`, `quota.go`, `report.go`, `driver.go`, `fakekaggle_test.go`.
  - Local Contracts:
    - one non-terminal run per installation (`CreateRun`, `ErrRunActive`);
    - `DeleteProject` refuses while a project run is not terminal;
    - `DeleteRun` only for terminal runs, rows only, files via `RemoveRunFiles`;
    - the newest 5,000 events per run;
    - non-finite metric losses are dropped;
    - `remote_json` holds everything without a column;
    - plan tokens live in memory, 15 min, at most 64;
    - `ParseGPUQuota` is provisional until the proof run;
    - the fake advances jobs only when observed.
  - This task changes no other guide: the root `AGENTS.md` and `internal/kaggle/AGENTS.md` are untouched because no Kaggle behaviour changes.

- [ ] 15. Verification (Git Bash, worktree root). Prefix with `GOTOOLCHAIN=go1.27.4` if Go reports `no such tool "vet"`.

```bash
gofmt -l internal/chibisensei
go build ./...
go test ./internal/chibisensei/ -count=1 -run '(?i)(RunCreate|UpdateRun|RunJSON|RunEvents|Metrics|Artifacts|DeleteRun|ComputeSteps|TrainingStep|PlanToken|PlanHash|Preflight|GPUQuota|KaggleRequest|ScoreReport|FakeKaggle|ProjectLifecycle)'
go test ./internal/chibisensei/... -count=1
go test ./internal/server -count=1 -run '(?i)Chibi'
```

  Expected results:
  - `gofmt -l` prints nothing;
  - `go build ./...` exits 0;
  - each `go test` line ends with `ok  aurago/internal/chibisensei` or `ok  aurago/internal/server`, with no `FAIL`;
  - the focused run shows `PASS` for every named test (count the `--- PASS` lines with `-v` if in doubt).

  The server suite proves that the changed `projectSelect` and `DeleteProject` keep the P1 handlers green.

---

## Chibi Sensei P2 backend plan, part 5 (B4)

Part B4 builds the run orchestrator: the state machine and reconciler, downloads, the artifact registry, the GGUF header check, cleanup, cancel and deadline, plus the fake-Kaggle acceptance suite. It builds on B1 (`ReadKernelLog`, `RemoveFile`), B2a (snapshot, job spec, notebook, parsers) and B3 (run store, driver, `fakeKaggle`). Their names are used as those plans define them.

Conventions for every task:
- Work only in the worktree `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\trusting-roentgen-94e148`. Scratch files go to `C:\disposable\chibi-sensei\`.
- Run Go commands in Git Bash from the worktree root, with `-count=1`. If Go reports `no such tool "vet"`, prefix with `GOTOOLCHAIN=go1.27.4`.
- Write real UTF-8, never `\u` escapes.
- Do not commit, stage, stash or switch branches. Do not run GitNexus.

### Contract changes

1. **`OrchestratorConfig` gains two fields:** `MaxTransferBytes int64` (`kaggle.max_transfer_mb` × 1 MiB) and `StorageMaxBytes int64` (`chibi_sensei.storage.max_gb` × 1 GiB). 0 means no limit. B5 fills both. Reason: the size checks before each download need them.
2. **Extensions of B3's types in `runs.go`.** All are additive JSON inside `remote_json`, so D1 holds. None of them appears in `RunView`.
   - `RunRemote` gains `Plan`, `EndState`, `BlockedSince`, `BlockedCode`, `CancelTried`, `EmptyLogReads`, `FinalLogRead`, `OutputsPlanned`, `Cleanup`, `GGUF`.
   - `KaggleStep` gains `Refused`.
   - `RunOutput` gains `Missing`.
   - Reason: each field is a fact a restart needs to resume a state, or a fact Submit must store so that `queued` can build the job.
3. **Extension of `fakekaggle_test.go`:** `fakeOutcome.Applied bool`. With `Status: unknown`, the fake applies the side effect first, so the request reached Kaggle but the outcome is unknown. Without it, the "`unknown` but the dataset or notebook exists" branches cannot be tested.
4. **New exported helpers (additive):**
   - `ErrDesktopReadOnly`, `ChibiDir`, `ReadLogTail`, `OpenArtifact`;
   - `BaseGGUF`, `Store.PutBaseGGUF`, `Store.GetBaseGGUF`, `BaseGGUFConverter`;
   - `ErrGGUFInvalid`, JSON tags on `GGUFInfo`, `(*Orchestrator).SetLogger`.

   Requirements for B5:
   - Compute `PlanInput.NeedBaseGGUF` as `!found` from `GetBaseGGUF(profile.ID, profile.Base.Revision, BaseGGUFConverter(lock.LlamaCppTag))`.
   - Serve `GET /runs/{id}/log` with `ReadLogTail`.
   - Serve artifact files with `OpenArtifact`.
5. **Rules for B5's driver adapter:**
   - It wraps a refusal caused by the read-only desktop with `ErrDesktopReadOnly`. The orchestrator then records the code `desktop_readonly`; any other refusal gets `kaggle_unavailable`.
   - It checks desktop read-only, Kaggle enabled, network and grants **before** it calls `Service.Execute` for a mutation. Otherwise the host gate (`BeginJob`) admits a failed job row on every retry. The orchestrator still survives such a row: an earlier refused attempt whose replay is `failed` moves to the next attempt.
6. **New run failure codes.** Message keys are needed in F2 to F4.
   - Set by the orchestrator: `data_changed`, `packaging_failed`, `profile_changed`, `upload_failed`, `dataset_failed`, `dataset_timeout`, `submit_failed`, `kaggle_slot_busy`, `manifest_invalid`, `artifact_too_large`, `storage_full`, `trainer_failed` (the status says failed without a code), `kaggle_unavailable` and `desktop_readonly` (refused for over 6 hours before submission or while downloading), `internal_error` (5 local errors in a row).
   - New warning codes in `warning_code`: `time_budget`, `base_gguf_invalid`, `remote_timeout` (a cancelled run that passed its deadline).
   - Event-only codes: `manifest_missing` and `cleanup_failed` (`warn` events).
7. **Cancel details beyond the spec:**
   - A cancel in `downloading` or `verifying` ends `cancelled` at once and skips the downloads.
   - A cancelled run that was submitted downloads nothing; only the log tail is kept.
   - `Cancel` never calls Kaggle. It sets `cancel_requested` in one transaction, publishes `run` and wakes the loop. The next step tries the remote cancel once.
   - **The UI reads `run.cancel_requested`**, true at once, and `run.cancel`, which is non-null at once with `stop_url` as soon as `save_kernel` was admitted. A moment later it reads `run.cancel.remote_stopped`.
8. **A run whose status says failed downloads only `chibi-metrics.jsonl`** (if listed): no GGUF, adapter or eval samples, and no report. A missing manifest is then accepted (`warn` `manifest_missing`).
9. **Dataset and notebook titles equal their slugs.** Kaggle derives a slug from the title. `licenseName` is `"unknown"`. The `get_dataset_status` vocabulary is PROVISIONAL: case-insensitive `ready`/`complete`/`completed` counts as ready and `error`/`failed`/`deleted` as failed, as a `{"status"}` object or a bare JSON string. Verify at the Gemma proof run.
10. **An untracked run uses kernel version 1.** The notebook slug is new per run, so its first and only version is 1.
11. **The `metric` hub event is not sent after `ReplaceMetrics`** (up to 20,000 points). F3 reloads `GET /runs/{id}/metrics?since_step=0` when a `run` event shows the state leaving `running`, `downloading` or `verifying`.
12. **Cleaning never waits.** A refused or failed delete becomes a leftover (spec: "failures are recorded as leftovers"). The blocked limit of 6 hours applies only before submission and while downloading.
13. **Observation for the coordinator (not B4's change):** B2a's `snapshot.go` and B3's `runs.go` both define `DatasetSlug` and `NotebookSlug`, so the package does not compile. Keep B2a's (it lower-cases) and drop B3's.

### File map

| File | Batch | Responsibility |
|---|---|---|
| `internal/chibisensei/runs.go` | B4 (B4.1) | additive fields on `RunRemote`, `KaggleStep`, `RunOutput` |
| `internal/chibisensei/gguf.go`, `gguf_test.go` | B4 (B4.1) | `GGUFInfo`, `ReadGGUFInfo`, bounded header reader, tests |
| `internal/chibisensei/artifacts.go`, `artifacts_test.go` | B4 (B4.1) | storage layout, verified copy, `StorageBytes`, `RemoveRunFiles`, log tail, `OpenArtifact`, `base_gguf` store |
| `internal/chibisensei/trainer/gguf_selftest_test.go` | B4 (B4.1) | `assertDryRunGGUF` for B2b's dry-run test |
| `internal/chibisensei/fakekaggle_test.go` | B4 (B4.2) | `fakeOutcome.Applied` |
| `internal/chibisensei/reconcile.go` | B4 (B4.2) | `Orchestrator`, loop, `Submit`, `Cancel`, `Step`, commit, transition, blocked and error helpers |
| `internal/chibisensei/reconcile_kaggle.go` | B4 (B4.2) | mutation driver (`kaggleStep`), job and result helpers, status classifiers |
| `internal/chibisensei/reconcile_stages.go` | B4 (B4.2) | `queued`, `packaging`, `uploading`, `creating_dataset`, `waiting_dataset`, `starting` |
| `internal/chibisensei/reconcile_running.go` | B4 (B4.2) | `running`, deadline, live log |
| `internal/chibisensei/reconcile_outputs.go` | B4 (B4.2) | `downloading`, `verifying`, artifact registration, report |
| `internal/chibisensei/reconcile_cleanup.go` | B4 (B4.2) | `cleaning`, leftovers, end state |
| `internal/chibisensei/reconcile_test.go` | B4 (B4.3) | fixture, happy paths, outcome table, cancel, live log, blocked, loop |
| `internal/chibisensei/reconcile_restart_test.go` | B4 (B4.3) | restart at every state, mid-flight restarts |

Storage layout below `<data>/chibi_sensei` (every name is generated by AuraGo):

| Path | Content |
|---|---|
| `runs/<run_id>/data/r<run12>-{train,val,eval}.jsonl`, `r<run12>-marker.json` | frozen snapshot |
| `runs/<run_id>/notebook.ipynb` | the exact notebook sent to Kaggle |
| `runs/<run_id>/out/{status.json, manifest.json, metrics.jsonl, eval_samples.jsonl, log-tail.txt}` | small outputs and the log tail |
| `models/<run_id>/model-q8_0.gguf` | tuned model |
| `models/<run_id>/adapter/{adapter_model.safetensors, adapter_config.json}` | adapter with PEFT names, so the folder loads as is |
| `base/<profile>-<rev12>-llamacpp-<tag>.gguf` | shared base GGUF, row in `base_gguf`; never removed by `RemoveRunFiles` |

---

#### Task B4.1: GGUF reader, artifact storage, type extensions

**Files**
- Create: `internal/chibisensei/gguf.go`, `gguf_test.go`, `artifacts.go`, `artifacts_test.go`, `internal/chibisensei/trainer/gguf_selftest_test.go`
- Modify: `internal/chibisensei/runs.go` (B3): the `KaggleStep` struct, the `RunOutput` struct, the end of the `RunRemote` struct.

**Existing symbols modified:** `KaggleStep`, `RunOutput`, `RunRemote` (types planned by B3 in `runs.go`).

**Steps**

- [ ] 1. `runs.go`:
  - In `KaggleStep`, after `Error`, add:
    `Refused bool `json:"refused,omitempty"` // Execute refused this attempt before admission`
  - In `RunOutput`, after `Copied`, add:
    `Missing bool `json:"missing,omitempty"` // optional output not available after the attempt limit`
  - In `RunRemote`, after `Attempts`, add:

```go
	// Orchestrator facts (B4).
	Plan           *Plan             `json:"plan,omitempty"`            // the confirmed plan, stored by Submit
	EndState       string            `json:"end_state,omitempty"`       // succeeded | failed | cancelled once cleaning starts
	BlockedSince   string            `json:"blocked_since,omitempty"`   // first refused driver call of the current wait
	BlockedCode    string            `json:"blocked_code,omitempty"`    // desktop_readonly | kaggle_unavailable (one warn event per code)
	CancelTried    bool              `json:"cancel_tried,omitempty"`    // the remote cancel was tried (once)
	EmptyLogReads  int               `json:"empty_log_reads,omitempty"` // log reads without a @@chibi line in a row
	FinalLogRead   bool              `json:"final_log_read,omitempty"`  // downloading read the log once more
	OutputsPlanned bool              `json:"outputs_planned,omitempty"` // the manifest files are in Outputs
	Cleanup        map[string]string `json:"cleanup,omitempty"`         // staged | dataset | notebook → done | deleted | absent | kept | failed
	GGUF           *GGUFInfo         `json:"gguf,omitempty"`            // verified header of the tuned model
```

- [ ] 2. Create `gguf.go`:

```go
package chibisensei

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
)

// GGUFInfo is the verified header of a GGUF file.
type GGUFInfo struct {
	Version      uint32 `json:"version"`
	Tensors      uint64 `json:"tensors"`
	Architecture string `json:"architecture"`
}

// ErrGGUFInvalid wraps every refusal of ReadGGUFInfo.
var ErrGGUFInvalid = errors.New("chibisensei: invalid GGUF file")

const (
	ggufVersion       = 3
	ggufMaxTensors    = 1 << 24
	ggufMaxKV         = 1 << 16
	ggufMaxKeyBytes   = 1 << 10
	ggufMaxValueBytes = 1 << 20   // one skipped string value
	ggufMaxArrayLen   = 1 << 24
	ggufMaxScanBytes  = 256 << 20 // metadata read before general.architecture must appear
	ggufTypeString    = 8
	ggufTypeArray     = 9
	ggufArchKey       = "general.architecture"
)

var (
	ggufScalarBytes = map[uint32]uint64{0: 1, 1: 1, 2: 2, 3: 2, 4: 4, 5: 4, 6: 4, 7: 1, 10: 8, 11: 8, 12: 8}
	ggufArchPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)
)

// ReadGGUFInfo reads only the header and the metadata up to general.architecture. The file comes from a remote
// machine, so every length is capped: keys 1 KiB, skipped strings 1 MiB, arrays 2^24 values, no nested arrays,
// 2^16 keys, 2^24 tensors and 256 MiB of metadata in total. Only version 3 is accepted.
func ReadGGUFInfo(path string) (GGUFInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return GGUFInfo{}, err
	}
	defer f.Close()
	return readGGUF(f)
}

func readGGUFInRoot(root *os.Root, rel string) (GGUFInfo, error) {
	f, err := root.Open(rel)
	if err != nil {
		return GGUFInfo{}, err
	}
	defer f.Close()
	return readGGUF(f)
}

func ggufErr(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrGGUFInvalid}, a...)...)
}

type ggufReader struct{ r *bufio.Reader }

func (g ggufReader) read(p []byte) error {
	if _, err := io.ReadFull(g.r, p); err != nil {
		return ggufErr("truncated")
	}
	return nil
}

func (g ggufReader) u32() (uint32, error) {
	var b [4]byte
	if err := g.read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[:]), nil
}

func (g ggufReader) u64() (uint64, error) {
	var b [8]byte
	if err := g.read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

func (g ggufReader) discard(n uint64) error {
	if n > ggufMaxScanBytes {
		return ggufErr("value too large")
	}
	if _, err := g.r.Discard(int(n)); err != nil {
		return ggufErr("truncated")
	}
	return nil
}

func (g ggufReader) str(limit uint64) (string, error) {
	n, err := g.u64()
	if err != nil {
		return "", err
	}
	if n > limit {
		return "", ggufErr("string of %d bytes exceeds the limit", n)
	}
	b := make([]byte, n)
	if err := g.read(b); err != nil {
		return "", err
	}
	return string(b), nil
}

func (g ggufReader) skipValue(typ uint32, inArray bool) error {
	if n, ok := ggufScalarBytes[typ]; ok {
		return g.discard(n)
	}
	switch typ {
	case ggufTypeString:
		n, err := g.u64()
		if err != nil {
			return err
		}
		if n > ggufMaxValueBytes {
			return ggufErr("string of %d bytes exceeds the limit", n)
		}
		return g.discard(n)
	case ggufTypeArray:
		if inArray {
			return ggufErr("nested arrays are not supported")
		}
		elem, err := g.u32()
		if err != nil {
			return err
		}
		count, err := g.u64()
		if err != nil {
			return err
		}
		if count > ggufMaxArrayLen {
			return ggufErr("array of %d values exceeds the limit", count)
		}
		if n, ok := ggufScalarBytes[elem]; ok {
			return g.discard(count * n)
		}
		for i := uint64(0); i < count; i++ {
			if err := g.skipValue(elem, true); err != nil {
				return err
			}
		}
		return nil
	}
	return ggufErr("unknown value type %d", typ)
}

func readGGUF(src io.Reader) (GGUFInfo, error) {
	g := ggufReader{r: bufio.NewReaderSize(io.LimitReader(src, ggufMaxScanBytes), 64<<10)}
	var magic [4]byte
	if err := g.read(magic[:]); err != nil {
		return GGUFInfo{}, err
	}
	if string(magic[:]) != "GGUF" {
		return GGUFInfo{}, ggufErr("wrong magic")
	}
	version, err := g.u32()
	if err != nil {
		return GGUFInfo{}, err
	}
	if version != ggufVersion {
		return GGUFInfo{}, ggufErr("version %d is not supported", version)
	}
	tensors, err := g.u64()
	if err != nil {
		return GGUFInfo{}, err
	}
	if tensors == 0 || tensors > ggufMaxTensors {
		return GGUFInfo{}, ggufErr("tensor count %d out of range", tensors)
	}
	kvs, err := g.u64()
	if err != nil {
		return GGUFInfo{}, err
	}
	if kvs > ggufMaxKV {
		return GGUFInfo{}, ggufErr("%d metadata keys exceed the limit", kvs)
	}
	for i := uint64(0); i < kvs; i++ {
		key, err := g.str(ggufMaxKeyBytes)
		if err != nil {
			return GGUFInfo{}, err
		}
		typ, err := g.u32()
		if err != nil {
			return GGUFInfo{}, err
		}
		if key != ggufArchKey {
			if err := g.skipValue(typ, false); err != nil {
				return GGUFInfo{}, err
			}
			continue
		}
		if typ != ggufTypeString {
			return GGUFInfo{}, ggufErr("%s is not a string", ggufArchKey)
		}
		arch, err := g.str(64)
		if err != nil {
			return GGUFInfo{}, err
		}
		if !ggufArchPattern.MatchString(arch) {
			return GGUFInfo{}, ggufErr("invalid architecture name")
		}
		return GGUFInfo{Version: version, Tensors: tensors, Architecture: arch}, nil
	}
	return GGUFInfo{}, ggufErr("%s is missing", ggufArchKey)
}
```

- [ ] 3. Create `gguf_test.go` (package `chibisensei`; it uses B3's `fakeGGUF`):

```go
package chibisensei

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadGGUFInfo(t *testing.T) {
	put := func(b *bytes.Buffer, v any) { _ = binary.Write(b, binary.LittleEndian, v) }
	str := func(b *bytes.Buffer, s string) { put(b, uint64(len(s))); b.WriteString(s) }
	head := func(version uint32, tensors, kvs uint64) *bytes.Buffer {
		b := &bytes.Buffer{}
		b.WriteString("GGUF")
		put(b, version)
		put(b, tensors)
		put(b, kvs)
		return b
	}
	patch := func(off int, v any) []byte {
		b := bytes.NewBuffer(append([]byte(nil), fakeGGUF("gemma3")...))
		out := b.Bytes()
		var p bytes.Buffer
		put(&p, v)
		copy(out[off:], p.Bytes())
		return out
	}
	skipped := head(3, 2, 3)
	str(skipped, "general.alignment")
	put(skipped, uint32(4))
	put(skipped, uint32(32))
	str(skipped, "tokenizer.ggml.tokens")
	put(skipped, uint32(9))
	put(skipped, uint32(8))
	put(skipped, uint64(2))
	str(skipped, "a")
	str(skipped, "bc")
	str(skipped, "general.architecture")
	put(skipped, uint32(8))
	str(skipped, "qwen35")
	oversized := head(3, 1, 1)
	put(oversized, uint64(1)<<40)
	notString := head(3, 1, 1)
	str(notString, "general.architecture")
	put(notString, uint32(4))
	put(notString, uint32(7))
	nested := head(3, 1, 1)
	str(nested, "x")
	put(nested, uint32(9))
	put(nested, uint32(9))
	put(nested, uint64(1))
	cases := []struct {
		name string
		body []byte
		want GGUFInfo
		err  string
	}{
		{"fake writer", fakeGGUF("gemma3"), GGUFInfo{3, 1, "gemma3"}, ""},
		{"skips other keys", skipped.Bytes(), GGUFInfo{3, 2, "qwen35"}, ""},
		{"wrong magic", append([]byte("GGML"), fakeGGUF("gemma3")[4:]...), GGUFInfo{}, "wrong magic"},
		{"version 2", patch(4, uint32(2)), GGUFInfo{}, "version 2"},
		{"zero tensors", patch(8, uint64(0)), GGUFInfo{}, "tensor count 0"},
		{"oversized string", oversized.Bytes(), GGUFInfo{}, "exceeds the limit"},
		{"truncated", fakeGGUF("gemma3")[:30], GGUFInfo{}, "truncated"},
		{"architecture missing", head(3, 1, 0).Bytes(), GGUFInfo{}, "is missing"},
		{"architecture not a string", notString.Bytes(), GGUFInfo{}, "is not a string"},
		{"nested array", nested.Bytes(), GGUFInfo{}, "nested arrays"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := readGGUF(bytes.NewReader(c.body))
			if c.err == "" {
				if err != nil || got != c.want {
					t.Fatalf("got %+v %v, want %+v", got, err, c.want)
				}
				return
			}
			if !errors.Is(err, ErrGGUFInvalid) || !strings.Contains(err.Error(), c.err) {
				t.Fatalf("err = %v, want %q", err, c.err)
			}
		})
	}
	p := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(p, fakeGGUF("lfm2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadGGUFInfo(p); err != nil || got.Architecture != "lfm2" {
		t.Fatalf("file: %+v %v", got, err)
	}
}
```

- [ ] 4. Create `artifacts.go`:

```go
package chibisensei

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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
	"unicode/utf8"

	"aurago/internal/security"
)

// Layout below <data>/chibi_sensei; every name is generated here, never taken from Kaggle (see the plan's table).
const (
	chibiDirName     = "chibi_sensei"
	snapshotSubdir   = "data"
	outSubdir        = "out"
	notebookFileName = "notebook.ipynb"
	logTailFileName  = "log-tail.txt"
	logTailBytes     = 64 << 10
	smallOutputMax   = 1 << 20 // status and manifest before their size is known
	minFreeDiskBytes = 64 << 20
)

var (
	errChecksumMismatch = errors.New("chibisensei: checksum mismatch")
	errOutputTooLarge   = errors.New("chibisensei: output larger than allowed")
	converterPattern    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,40}$`)
)

func ChibiDir(dataDir string) string         { return filepath.Join(dataDir, chibiDirName) }
func RunDir(dataDir, runID string) string     { return filepath.Join(ChibiDir(dataDir), "runs", runID) }
func ModelDir(dataDir, runID string) string   { return filepath.Join(ChibiDir(dataDir), "models", runID) }
func BaseDir(dataDir string) string           { return filepath.Join(ChibiDir(dataDir), "base") }
func snapshotDir(dataDir, runID string) string { return filepath.Join(RunDir(dataDir, runID), snapshotSubdir) }

// runRel is a slash path below <data>/chibi_sensei inside the run directory.
func runRel(runID string, parts ...string) string {
	return path.Join(append([]string{"runs", runID}, parts...)...)
}

// stagedOutputName is the Kaggle project file a kernel output is downloaded into.
func stagedOutputName(runID, output string) string { return "r" + Run12(runID) + "-" + output }

// BaseGGUFConverter names the converter of a base GGUF (key of base_gguf): "llamacpp-<tag>".
func BaseGGUFConverter(llamaCppTag string) string { return "llamacpp-" + llamaCppTag }

func baseGGUFName(profileID, revision, converter string) (string, error) {
	if !profileIDPattern.MatchString(profileID) || !revisionPattern.MatchString(revision) || !converterPattern.MatchString(converter) {
		return "", errors.New("base gguf: invalid profile, revision or converter")
	}
	return profileID + "-" + revision[:12] + "-" + converter + ".gguf", nil
}

// outputRel is the storage location of a kernel output kind (and of manifest and log_tail).
func outputRel(r Run, f jobFacts, kind string) (string, error) {
	switch kind {
	case "status":
		return runRel(r.ID, outSubdir, "status.json"), nil
	case "manifest":
		return runRel(r.ID, outSubdir, "manifest.json"), nil
	case "metrics":
		return runRel(r.ID, outSubdir, "metrics.jsonl"), nil
	case "eval_samples":
		return runRel(r.ID, outSubdir, "eval_samples.jsonl"), nil
	case "log_tail":
		return runRel(r.ID, outSubdir, logTailFileName), nil
	case "gguf_tuned":
		return path.Join("models", r.ID, "model-q8_0.gguf"), nil
	case "adapter":
		return path.Join("models", r.ID, "adapter", "adapter_model.safetensors"), nil
	case "adapter_config":
		return path.Join("models", r.ID, "adapter", "adapter_config.json"), nil
	case "gguf_base":
		name, err := baseGGUFName(r.ProfileID, f.Profile.Base.Revision, f.converter())
		if err != nil {
			return "", err
		}
		return path.Join("base", name), nil
	}
	return "", fmt.Errorf("no storage location for output kind %q", kind)
}

func openChibiRoot(dataDir string) (*os.Root, error) {
	if err := os.MkdirAll(ChibiDir(dataDir), 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", chibiDirName, err)
	}
	return os.OpenRoot(ChibiDir(dataDir))
}

// copyVerified copies src to rel below root through a .part file and publishes it only when the size and the
// sha256 match (wantBytes < 0 and wantSHA == "" accept any content up to limit). It returns the measured size and hash.
func copyVerified(root *os.Root, rel string, src io.Reader, wantBytes int64, wantSHA string, limit int64) (int64, string, error) {
	if err := root.MkdirAll(path.Dir(rel), 0o700); err != nil {
		return 0, "", fmt.Errorf("create %s: %w", path.Dir(rel), err)
	}
	part := rel + ".part"
	if err := removeQuiet(root, part); err != nil {
		return 0, "", err
	}
	f, err := root.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(src, limit+1))
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	sum := hex.EncodeToString(h.Sum(nil))
	switch {
	case err != nil:
	case n > limit:
		err = errOutputTooLarge
	case (wantBytes >= 0 && n != wantBytes) || (wantSHA != "" && sum != wantSHA):
		err = errChecksumMismatch
	}
	if err != nil {
		_ = removeQuiet(root, part)
		return n, sum, err
	}
	return n, sum, root.Rename(part, rel)
}

func writeChibiFile(dataDir, rel string, data []byte) error {
	root, err := openChibiRoot(dataDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll(path.Dir(rel), 0o700); err != nil {
		return err
	}
	if err := removeQuiet(root, rel+".part"); err != nil {
		return err
	}
	return writeRootFile(root, rel, data)
}

func readChibiFile(dataDir, rel string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(ChibiDir(dataDir))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(rel)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errOutputTooLarge
	}
	return b, nil
}

// StorageBytes sums the regular files under <data>/chibi_sensei.
func StorageBytes(dataDir string) int64 {
	var total int64
	_ = filepath.WalkDir(ChibiDir(dataDir), func(_ string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		if info, e := d.Info(); e == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// RemoveRunFiles removes RunDir and ModelDir of a run; a missing directory is not an error; BaseDir is never touched.
func RemoveRunFiles(dataDir, runID string) error {
	if !ValidID(runID) {
		return errors.New("remove run files: invalid run id")
	}
	root, err := os.OpenRoot(ChibiDir(dataDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	var errs []error
	for _, rel := range []string{"runs/" + runID, "models/" + runID} {
		if err := root.RemoveAll(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// logTailText turns a kernel log body into display text (a Kaggle JSON log joins its entries' data in order), scrubs
// it and keeps the last 64 KiB at a rune boundary.
func logTailText(body []byte) []byte {
	text := string(body)
	var entries []kaggleLogEntry
	if t := bytes.TrimSpace(body); len(t) > 0 && t[0] == '[' && json.Unmarshal(t, &entries) != nil {
		entries = nil
	}
	if entries == nil {
		entries = scanLogEntries(body)
	}
	if len(entries) > 0 {
		var b strings.Builder
		for _, e := range entries {
			b.WriteString(e.Data)
		}
		text = b.String()
	}
	if len(text) > 2*logTailBytes {
		text = text[len(text)-2*logTailBytes:]
	}
	text = strings.ToValidUTF8(security.Scrub(text), "")
	if len(text) > logTailBytes {
		cut := len(text) - logTailBytes
		for cut < len(text) && !utf8.RuneStart(text[cut]) {
			cut++
		}
		text = text[cut:]
	}
	return []byte(text)
}

func writeLogTail(dataDir, runID string, body []byte) error {
	return writeChibiFile(dataDir, runRel(runID, outSubdir, logTailFileName), logTailText(body))
}

// ReadLogTail returns the stored, scrubbed log tail of a run (nil, nil when there is none).
func ReadLogTail(dataDir, runID string) ([]byte, error) {
	if !ValidID(runID) {
		return nil, errors.New("read log tail: invalid run id")
	}
	b, err := readChibiFile(dataDir, runRel(runID, outSubdir, logTailFileName), 2*logTailBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// OpenArtifact opens a registered artifact under <data>/chibi_sensei through an os.Root.
func OpenArtifact(dataDir string, a Artifact) (*os.File, error) {
	if !validArtifactPath(a.RelPath) {
		return nil, errors.New("open artifact: invalid path")
	}
	root, err := os.OpenRoot(ChibiDir(dataDir))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(a.RelPath)
}

// BaseGGUF is one shared base-model GGUF (table base_gguf).
type BaseGGUF struct {
	ProfileID, BaseRevision, Converter, RelPath, SHA256 string
}

func (s *Store) PutBaseGGUF(ctx context.Context, b BaseGGUF) error {
	if !validArtifactPath(b.RelPath) || !sha256Hex.MatchString(b.SHA256) {
		return errors.New("put base gguf: invalid path or hash")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO base_gguf(profile_id,base_revision,converter,rel_path,sha256)
		VALUES(?,?,?,?,?) ON CONFLICT(profile_id,base_revision,converter) DO UPDATE SET rel_path=excluded.rel_path,
		sha256=excluded.sha256`, b.ProfileID, b.BaseRevision, b.Converter, b.RelPath, b.SHA256); err != nil {
		return fmt.Errorf("put base gguf: %w", err)
	}
	return nil
}

func (s *Store) GetBaseGGUF(ctx context.Context, profileID, revision, converter string) (BaseGGUF, bool, error) {
	b := BaseGGUF{ProfileID: profileID, BaseRevision: revision, Converter: converter}
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(rel_path,''), COALESCE(sha256,'') FROM base_gguf
		WHERE profile_id=? AND base_revision=? AND converter=?`, profileID, revision, converter).Scan(&b.RelPath, &b.SHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return BaseGGUF{}, false, nil
	}
	if err != nil {
		return BaseGGUF{}, false, err
	}
	return b, true, nil
}

func (s *Store) deleteBaseGGUF(ctx context.Context, profileID, revision, converter string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM base_gguf WHERE profile_id=? AND base_revision=? AND converter=?`,
		profileID, revision, converter)
	return err
}

func (s *Store) deleteArtifact(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM artifacts WHERE id=?`, id)
	return err
}
```

  (`jobFacts` and its `converter` method come in B4.2. Until then, B4.1 adds a temporary copy at the end of `artifacts.go`, and B4.2 moves it to `reconcile_outputs.go`. Simpler: implement B4.1 and B4.2 in one sitting before the first build.)

- [ ] 5. Create `artifacts_test.go`. Test `TestRunFilesLayout`:
  - Write `runs/<id>/out/a.txt` (3 bytes), `models/<id>/m.bin` (5 bytes) and `base/b.gguf` (7 bytes) with `writeChibiFile`. `StorageBytes == 15`.
  - `RemoveRunFiles(dir, id)` removes `runs/<id>` and `models/<id>`; `base/b.gguf` stays. A second call returns nil; `"x"` returns an error.
  - `security.RegisterSensitive("tok-" + strings.Repeat("q7", 16))`. Then `logTailText(fakeKernelLogJSON("line one", "secret tok-…"))` contains `line one` and not the token.
  - A 100 KiB plain-text body gives exactly 65,536 bytes ending with the body's last byte.
  - `ReadLogTail` of an unknown run returns `nil, nil`.

- [ ] 6. Create `internal/chibisensei/trainer/gguf_selftest_test.go`. The coordinator wires the call `assertDryRunGGUF(t, outDir, profile.GGUF.ExpectedArch)` into B2b's dry-run test in `selftest_test.go`.

```go
package trainer_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/chibisensei"
)

// assertDryRunGGUF checks the GGUF files of a trainer --dry-run in dir with the production reader: the tuned model
// must exist; the base model is checked when present.
func assertDryRunGGUF(t *testing.T, dir, wantArch string) {
	t.Helper()
	for _, name := range []string{chibisensei.OutModelGGUF, chibisensei.OutBaseGGUF} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
			if name == chibisensei.OutModelGGUF {
				t.Fatalf("dry run wrote no %s", name)
			}
			continue
		}
		info, err := chibisensei.ReadGGUFInfo(p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if info.Version != 3 || info.Tensors == 0 || info.Architecture != wantArch {
			t.Fatalf("%s: %+v, want version 3, tensors > 0, architecture %q", name, info, wantArch)
		}
	}
}
```

- [ ] 7. Verify, after B4.2 compiles:

```bash
gofmt -l internal/chibisensei
go build ./...
go test ./internal/chibisensei/ -count=1 -run '(?i)(ReadGGUFInfo|RunFilesLayout)' -v
go test ./internal/chibisensei/trainer -count=1
```
  Expected: `gofmt` prints nothing. `--- PASS: TestReadGGUFInfo` with 10 subtests, and `--- PASS: TestRunFilesLayout`. Both packages end with `ok`.

---

#### Task B4.2: Orchestrator

**Files**
- Create: `internal/chibisensei/reconcile.go`, `reconcile_kaggle.go`, `reconcile_stages.go`, `reconcile_running.go`, `reconcile_outputs.go`, `reconcile_cleanup.go`
- Modify: `internal/chibisensei/fakekaggle_test.go` (B3): the `fakeOutcome` struct and the `case KaggleJobUnknown:` of `finishLocked`.

**Existing symbols modified:** `fakeOutcome`, `(*fakeKaggle).finishLocked` (B3 test file).

What each state does when a driver call is refused (read-only desktop or a revoked grant):

| State | Refused call | Behaviour |
|---|---|---|
| `queued`, `verifying` | none (local only) | runs |
| `packaging` | `EnsureProject`, `WriteFile` | waits; one `warn` per code; fails after 6 h of refusals |
| `uploading`, `creating_dataset`, `starting` | `Execute` | waits; the receipt is marked `refused` and the same ID is retried; a refused attempt whose replay is `failed` (the host gate) moves to the next attempt; 6 h → fail |
| `waiting_dataset` | status read | waits; the first successful read after a wait restarts the 15-minute window |
| `running` | `Job` or status read | waits, warns once, no limit; past the deadline the final read decides, and a refused final read gives `remote_timeout` |
| `downloading` | `RemoveFile`, `Execute`, `OpenFile` | waits; 6 h → fail |
| `cleaning` | any | never waits: the resource becomes a leftover |
| cancel | none | `Cancel` only writes the store, so it works read-only; a refused remote cancel records `remote_stopped: false` |

**Steps**

- [ ] 1. `fakekaggle_test.go`:
  - Add to `fakeOutcome`, after `AdmitBeforeError`:
    `Applied bool // with Status unknown: apply the side effect first (the request reached Kaggle)`
  - In `finishLocked`, start `case KaggleJobUnknown:` with:

```go
		if out.Applied {
			if _, _, err := f.applyLocked(j); err != nil {
				f.t.Errorf("fakeOutcome.Applied: %v", err)
			}
		}
```

- [ ] 2. Create `reconcile.go`:

```go
package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"aurago/internal/chibisensei/trainer"
	"aurago/internal/security"
)

// ErrDesktopReadOnly marks a driver refusal caused by the read-only desktop (the driver adapter wraps it).
var ErrDesktopReadOnly = errors.New("chibisensei: the desktop is read-only")

// errRunMoved aborts an update whose run changed state since the step loaded it.
var errRunMoved = errors.New("chibisensei: the run changed state")

var kaggleOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)

const (
	codeCancelled      = "cancelled_by_user"
	msgDataChanged     = "The dataset changed after the start. Start the run again."
	msgUploadFailed    = "Kaggle did not accept the training data."
	msgDatasetFailed   = "Kaggle could not prepare the private dataset."
	msgDownloadFailed  = "A result file could not be downloaded from Kaggle."
	msgChecksum        = "A downloaded file does not match its checksum."
	msgManifestInvalid = "The result list of the run is invalid."
	msgStatusMissing   = "The run left no readable status file. The log tail is kept."

	maxStepAttempts   = 3 // attempts 0, 1, 2 of one Kaggle step
	maxLocalErrors    = 5
	datasetWaitMax    = 15 * time.Minute
	deadlineSlack     = 20 * time.Minute
	blockedLimit      = 6 * time.Hour
	maxImmediateSteps = 200
	logReadMaxBytes   = 16 << 20 // A2: the whole JSON array, cut to 64 KiB only for display
)

// OrchestratorConfig is read at every step.
type OrchestratorConfig struct {
	MachineShape         string
	DeleteRemoteAfterRun bool
	AllowDelete          bool
	RemoteTimeoutMinutes int
	AuraGoVersion        string
	MaxTransferBytes     int64 // kaggle.max_transfer_mb in bytes; 0 = no limit
	StorageMaxBytes      int64 // chibi_sensei.storage.max_gb in bytes; 0 = no limit
}

// SubmitInput is a confirmed start (the server checked plan token, preflight and consent).
type SubmitInput struct {
	RunID      string
	Project    Project
	Profile    *Profile
	ProfileSHA string
	Plan       Plan
	Owner      string
}

type intervals struct {
	localJob, running, dataset, logRead, untrackedPoll, blocked, idle, errorWait time.Duration
}

var defaultIntervals = intervals{localJob: 2 * time.Second, running: 10 * time.Second, dataset: 15 * time.Second,
	logRead: 60 * time.Second, untrackedPoll: 10 * time.Second, blocked: 30 * time.Second, idle: 60 * time.Second,
	errorWait: 15 * time.Second}

// stepResult: progressed = something was persisted and another step may continue at once; wait = the pause the
// state asks for otherwise.
type stepResult struct {
	progressed bool
	wait       time.Duration
}

// Orchestrator drives the training runs. One goroutine (Start) owns the active run (D13); Step is the unit of work.
type Orchestrator struct {
	st      *Store
	hub     *Hub
	cat     *Catalog
	drv     KaggleDriver
	dataDir string
	cfg     func() OrchestratorConfig
	now     func() time.Time
	iv      intervals
	log     *slog.Logger

	stepMu sync.Mutex
	wake   chan struct{}
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	closed bool
}

func NewOrchestrator(st *Store, hub *Hub, cat *Catalog, drv KaggleDriver, dataDir string, cfg func() OrchestratorConfig) *Orchestrator {
	return &Orchestrator{st: st, hub: hub, cat: cat, drv: drv, dataDir: dataDir, cfg: cfg,
		now: func() time.Time { return time.Now().UTC() }, iv: defaultIntervals, log: slog.Default(),
		wake: make(chan struct{}, 1)}
}

// SetLogger sets the logger; call it before Start.
func (o *Orchestrator) SetLogger(l *slog.Logger) {
	if l != nil {
		o.log = l
	}
}

// Start runs the reconcile loop until ctx ends or Close; it resumes every non-terminal run. Ending the context
// never changes a run.
func (o *Orchestrator) Start(ctx context.Context) {
	o.mu.Lock()
	if o.cancel != nil || o.closed {
		o.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	o.cancel, o.done = cancel, make(chan struct{})
	o.mu.Unlock()
	go o.loop(ctx)
}

func (o *Orchestrator) loop(ctx context.Context) {
	defer close(o.done)
	immediate := 0
	for ctx.Err() == nil {
		wait := o.iv.idle
		runs, err := o.st.ActiveRuns(ctx)
		if err != nil {
			if ctx.Err() == nil {
				o.log.Warn("Chibi Sensei runs could not be loaded", "error", err)
			}
			wait = o.iv.errorWait
		} else if len(runs) > 0 {
			res, err := o.step(ctx, runs[0].ID)
			switch {
			case err != nil:
				if ctx.Err() != nil {
					return
				}
				o.log.Warn("Chibi Sensei run step failed", "run", runs[0].ID, "error", err)
				wait = max(res.wait, o.iv.errorWait)
			case res.progressed && immediate < maxImmediateSteps:
				immediate++
				continue
			case res.progressed:
				wait = o.iv.localJob
			default:
				wait = res.wait
			}
		}
		immediate = 0
		if wait <= 0 {
			wait = o.iv.localJob
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-o.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (o *Orchestrator) poke() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

// Close stops the loop and waits for it (at most 15 s).
func (o *Orchestrator) Close() {
	o.mu.Lock()
	o.closed = true
	cancel, done := o.cancel, o.done
	o.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		o.log.Warn("Chibi Sensei orchestrator still busy at close")
	}
}

// Submit creates the run in state queued with everything later states need and wakes the loop. Snapshot, job and
// notebook are built in queued. It returns ErrRunActive while another run is not terminal.
func (o *Orchestrator) Submit(ctx context.Context, in SubmitInput) (Run, error) {
	p, pl := in.Profile, in.Plan
	switch {
	case !ValidID(in.RunID) || !ValidID(in.Project.ID):
		return Run{}, errors.New("submit run: invalid run or project id")
	case p == nil || pl.ProfileID != p.ID || !pl.Trainable:
		return Run{}, errors.New("submit run: the plan does not fit the profile")
	case !sha256Hex.MatchString(in.ProfileSHA):
		return Run{}, errors.New("submit run: invalid profile sha")
	case !kaggleOwnerPattern.MatchString(in.Owner):
		return Run{}, errors.New("submit run: invalid Kaggle owner")
	}
	cfg := o.cfg()
	plan := pl
	r := Run{ID: in.RunID, ProjectID: in.Project.ID, Backend: BackendKaggle, State: RunQueued, ProfileID: p.ID,
		ProfileSHA: in.ProfileSHA, Preset: pl.Preset, Hyper: pl.Hyper,
		Remote: RunRemote{ProfileStatus: p.Status, Method: pl.Method, Split: pl.Split, EstimateMinutes: pl.EstimateMinutes,
			SessionTimeoutMinutes: pl.SessionTimeoutMinutes, MaxTrainSeconds: pl.MaxTrainSeconds, DataRev: pl.DataRev,
			PlanHash:     PlanHash(pl, in.ProfileSHA, trainer.ScriptSHA256(), in.Owner, cfg.MachineShape, cfg.DeleteRemoteAfterRun),
			MachineShape: cfg.MachineShape, DeleteRemoteAfterRun: cfg.DeleteRemoteAfterRun, Owner: in.Owner,
			DatasetSlug: DatasetSlug(in.RunID), NotebookSlug: NotebookSlug(in.Project.ID, in.RunID),
			Marker: MarkerFileName(in.RunID), Live: LiveUnknown, Plan: &plan, Attempts: map[string]int{}}}
	if err := o.st.CreateRun(ctx, r); err != nil {
		return Run{}, err
	}
	got, err := o.st.GetRun(ctx, r.ID)
	if err != nil {
		return Run{}, err
	}
	o.publishRun(got)
	o.poke()
	return got, nil
}

// Cancel marks the run cancel_requested in one transaction and wakes the loop; it never calls Kaggle. The returned
// run shows cancel_requested and the cancel object at once. A terminal run is returned unchanged.
func (o *Orchestrator) Cancel(ctx context.Context, runID string) (Run, error) {
	r, err := o.st.GetRun(ctx, runID)
	if err != nil || RunTerminal(r.State) || r.CancelRequested {
		return r, err
	}
	next, err := o.st.UpdateRun(ctx, runID, NewRunEvent("state", map[string]any{"state": r.State, "cancel_requested": true}),
		func(x *Run) error {
			if RunTerminal(x.State) {
				return errRunMoved
			}
			x.CancelRequested = true
			return nil
		})
	if errors.Is(err, errRunMoved) {
		return o.st.GetRun(ctx, runID)
	}
	if err != nil {
		return Run{}, err
	}
	o.publishRun(next)
	o.poke()
	return next, nil
}

// Step performs one unit of work for the run: at most one Kaggle mutation and one state transition. It returns
// whether something was persisted that allows another step at once.
func (o *Orchestrator) Step(ctx context.Context, runID string) (bool, error) {
	res, err := o.step(ctx, runID)
	return res.progressed, err
}

func (o *Orchestrator) step(ctx context.Context, runID string) (stepResult, error) {
	o.stepMu.Lock()
	defer o.stepMu.Unlock()
	r, err := o.st.GetRun(ctx, runID)
	if err != nil {
		return stepResult{}, fmt.Errorf("load run %s: %w", runID, err)
	}
	if RunTerminal(r.State) {
		return stepResult{}, nil
	}
	var res stepResult
	handled := false
	if r.CancelRequested && r.State != RunCleaning {
		res, handled, err = o.handleCancel(ctx, r)
	}
	if !handled && err == nil {
		switch r.State {
		case RunQueued:
			res, err = o.stepQueued(ctx, r)
		case RunPackaging:
			res, err = o.stepPackaging(ctx, r)
		case RunUploading:
			res, err = o.stepUploading(ctx, r)
		case RunCreatingDataset:
			res, err = o.stepCreatingDataset(ctx, r)
		case RunWaitingDataset:
			res, err = o.stepWaitingDataset(ctx, r)
		case RunStarting:
			res, err = o.stepStarting(ctx, r)
		case RunRunning:
			res, err = o.stepRunning(ctx, r)
		case RunDownloading:
			res, err = o.stepDownloading(ctx, r)
		case RunVerifying:
			res, err = o.stepVerifying(ctx, r)
		case RunCleaning:
			res, err = o.stepCleaning(ctx, r)
		default:
			return stepResult{}, fmt.Errorf("run %s: unknown state %q", r.ID, r.State)
		}
	}
	if errors.Is(err, errRunMoved) {
		return stepResult{progressed: true}, nil
	}
	return res, err
}

// handleCancel: before submission (and after the remote ended) the run goes to cleaning as cancelled. After
// submission the remote cancel is tried once; the state handler then keeps tracking until the remote is terminal.
func (o *Orchestrator) handleCancel(ctx context.Context, r Run) (stepResult, bool, error) {
	switch {
	case !submitted(r) || r.State == RunDownloading || r.State == RunVerifying:
		res, err := o.endRun(ctx, r, RunCancelled, codeCancelled, "", nil)
		return res, true, err
	case r.Remote.CancelTried:
		return stepResult{}, false, nil
	}
	stopped, note := false, "No tracked Kaggle job; stop the notebook on Kaggle."
	if rec := r.Remote.Step(StepSaveKernel); rec != nil && rec.JobID != "" && !r.Remote.Untracked {
		err := o.drv.Cancel(ctx, rec.JobID)
		if err != nil && ctx.Err() != nil {
			return stepResult{}, true, ctx.Err()
		}
		stopped, note = err == nil, ""
		if err != nil {
			note = scrubText(err.Error(), 300)
		}
	}
	_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": "cancel", "remote_stopped": stopped}),
		func(x *Run) { x.Remote.CancelTried, x.Remote.CancelRemoteStopped, x.Remote.CancelError = true, stopped, note })
	return stepResult{progressed: true}, true, err
}

// submitted reports whether save_kernel was admitted (or the run tracks the notebook by status reads).
func submitted(r Run) bool {
	if r.Remote.Untracked {
		return true
	}
	rec := r.Remote.Step(StepSaveKernel)
	return rec != nil && rec.JobID != ""
}

// commit applies mut in one UpdateRun while the run is still in r's state, appends ev (Kind "" = none), clears the
// blocked marker and the local error count, and publishes the run summary after the commit.
func (o *Orchestrator) commit(ctx context.Context, r Run, ev RunEvent, mut func(*Run)) (Run, error) {
	next, err := o.st.UpdateRun(ctx, r.ID, ev, func(x *Run) error {
		if x.State != r.State {
			return errRunMoved
		}
		mut(x)
		x.Remote.BlockedSince, x.Remote.BlockedCode, x.StepAttempt = "", "", 0
		return nil
	})
	if err != nil {
		return Run{}, err
	}
	o.publishRun(next)
	return next, nil
}

func (o *Orchestrator) transition(ctx context.Context, r Run, to string, data map[string]any, mut func(*Run)) (stepResult, error) {
	if data == nil {
		data = map[string]any{}
	}
	data["state"], data["from"] = to, r.State
	_, err := o.commit(ctx, r, NewRunEvent("state", data), func(x *Run) {
		if mut != nil {
			mut(x)
		}
		x.State = to
	})
	return stepResult{progressed: true}, err
}

// endRun moves the run to cleaning; cleaning ends it in end (succeeded | failed | cancelled).
func (o *Orchestrator) endRun(ctx context.Context, r Run, end, code, msg string, extra func(*Run)) (stepResult, error) {
	data := map[string]any{"end_state": end}
	if code != "" {
		data["error_code"] = code
	}
	return o.transition(ctx, r, RunCleaning, data, func(x *Run) {
		if extra != nil {
			extra(x)
		}
		x.Remote.EndState = end
		if code != "" {
			x.ErrorCode, x.Error = code, msg
		}
	})
}

func (o *Orchestrator) fail(ctx context.Context, r Run, code, msg string, extra func(*Run)) (stepResult, error) {
	return o.endRun(ctx, r, RunFailed, code, msg, extra)
}

// blocked handles a refused driver call: the run waits. A new wait or a new code records one warn event. Before
// submission and while downloading, a wait longer than blockedLimit fails the run with the code.
func (o *Orchestrator) blocked(ctx context.Context, r Run, cause error) (stepResult, error) {
	if err := ctx.Err(); err != nil {
		return stepResult{}, err
	}
	code := "kaggle_unavailable"
	if errors.Is(cause, ErrDesktopReadOnly) {
		code = "desktop_readonly"
	}
	o.log.Warn("Chibi Sensei run waits for Kaggle", "run", r.ID, "state", r.State, "code", code, "error", cause)
	now := o.now()
	if since, err := time.Parse(time.RFC3339, r.Remote.BlockedSince); err == nil && now.Sub(since) > blockedLimit &&
		r.State != RunRunning && r.State != RunCleaning {
		return o.fail(ctx, r, code, "Kaggle stayed unavailable for six hours.", nil)
	}
	if r.Remote.BlockedCode == code {
		return stepResult{wait: o.iv.blocked}, nil
	}
	next, err := o.st.UpdateRun(ctx, r.ID, NewRunEvent("warn", map[string]any{"code": code, "state": r.State}),
		func(x *Run) error {
			if x.State != r.State {
				return errRunMoved
			}
			if x.Remote.BlockedSince == "" {
				x.Remote.BlockedSince = ts(now)
			}
			x.Remote.BlockedCode = code
			return nil
		})
	if err != nil {
		return stepResult{}, err
	}
	o.publishRun(next)
	return stepResult{wait: o.iv.blocked}, nil
}

// localError counts a local failure (store, files). The fifth in a row fails the run with internal_error; cleaning
// only waits. The cause is returned so the loop logs it.
func (o *Orchestrator) localError(ctx context.Context, r Run, cause error) (stepResult, error) {
	if ctx.Err() != nil || errors.Is(cause, errRunMoved) || r.State == RunCleaning {
		return stepResult{wait: o.iv.errorWait}, cause
	}
	if r.StepAttempt+1 >= maxLocalErrors {
		if _, err := o.fail(ctx, r, "internal_error", "AuraGo could not continue the run.", nil); err != nil {
			return stepResult{}, errors.Join(cause, err)
		}
		return stepResult{progressed: true}, cause
	}
	if _, err := o.st.UpdateRun(ctx, r.ID, RunEvent{}, func(x *Run) error {
		if x.State != r.State {
			return errRunMoved
		}
		x.StepAttempt++
		return nil
	}); err != nil {
		return stepResult{}, errors.Join(cause, err)
	}
	return stepResult{wait: o.iv.errorWait}, cause
}

func (o *Orchestrator) publishRun(r Run) {
	if o.hub != nil {
		o.hub.Publish(r.ProjectID, Event{Name: "run", Data: r.Summary()})
	}
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func scrubText(s string, n int) string { return cutRunes(security.Scrub(s), n) }
```

- [ ] 3. Create `reconcile_kaggle.go`:

```go
package chibisensei

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/kaggle"
)

// kaggleTurn is the outcome of one call on a Kaggle mutation step.
type kaggleTurn struct {
	job     kaggle.Job
	ok      bool        // job is the receipt of the current attempt
	blocked error       // the driver refused the call; the run waits
	rec     *KaggleStep // receipt to store with the caller's commit; nil = unchanged
	extra   func(*Run)  // stored together with rec
}

func (t kaggleTurn) save(x *Run) {
	if t.rec != nil {
		x.Remote.PutStep(*t.rec)
		if t.extra != nil {
			t.extra(x)
		}
	}
}

func jobActive(status string) bool { return status == KaggleJobQueued || status == KaggleJobRunning }

func setAttempt(x *Run, step string, n int) {
	if x.Remote.Attempts == nil {
		x.Remote.Attempts = map[string]int{}
	}
	x.Remote.Attempts[step] = n
}

// kaggleStep advances the current attempt of a mutation step by one driver call:
//   - With a stored job ID it reads the job (Job); a status change comes back in rec.
//   - Otherwise it executes the deterministic request (a known ID returns the original receipt).
//   - A refused Execute marks the receipt refused and waits.
//   - A refused attempt whose replay is failed (the host gate admitted it as failed) never ran: the step moves to
//     attempt+1 and returns ok=false.
func (o *Orchestrator) kaggleStep(ctx context.Context, r Run, step, op, project, file string, params any) (kaggleTurn, error) {
	attempt := r.Remote.Attempts[step]
	id := KaggleRequestID(r.ID, step, attempt)
	rec := r.Remote.Step(step)
	now := ts(o.now())
	if rec != nil && rec.RequestID == id && rec.JobID != "" {
		job, err := o.drv.Job(ctx, rec.JobID)
		if err != nil {
			if ctx.Err() != nil {
				return kaggleTurn{}, ctx.Err()
			}
			return kaggleTurn{blocked: err}, nil
		}
		t := kaggleTurn{job: job, ok: true}
		if job.Status != rec.Status || job.Error != rec.Error {
			next := *rec
			next.Status, next.Error, next.Updated = job.Status, job.Error, now
			t.rec = &next
		}
		return t, nil
	}
	req, err := NewKaggleRequest(id, op, project, file, params)
	if err != nil {
		return kaggleTurn{}, err
	}
	res, err := o.drv.Execute(ctx, req)
	if err != nil {
		if ctx.Err() != nil {
			return kaggleTurn{}, ctx.Err()
		}
		if rec == nil || rec.RequestID != id || !rec.Refused {
			mark := KaggleStep{Step: step, Attempt: attempt, RequestID: id, File: file, Refused: true, Updated: now}
			if _, uerr := o.st.UpdateRun(ctx, r.ID, RunEvent{}, func(x *Run) error {
				if x.State != r.State {
					return errRunMoved
				}
				x.Remote.PutStep(mark)
				return nil
			}); uerr != nil {
				return kaggleTurn{}, uerr
			}
		}
		return kaggleTurn{blocked: err}, nil
	}
	if res.Job == nil {
		return kaggleTurn{}, fmt.Errorf("kaggle %s: no job receipt", op)
	}
	job := *res.Job
	if rec != nil && rec.RequestID == id && rec.Refused && job.Status == KaggleJobFailed {
		_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": step, "attempt": attempt + 1, "reason": "refused"}),
			func(x *Run) { setAttempt(x, step, attempt+1) })
		return kaggleTurn{}, err
	}
	return kaggleTurn{job: job, ok: true, rec: &KaggleStep{Step: step, Attempt: attempt, RequestID: id, JobID: job.ID,
		Status: job.Status, File: file, Error: job.Error, Updated: now}}, nil
}

// pending returns (result, true) when the step must stop here: refused, moved to a new attempt, or the job is still
// queued or running locally (a new receipt is stored first).
func (o *Orchestrator) pending(ctx context.Context, r Run, t kaggleTurn) (stepResult, bool, error) {
	switch {
	case t.blocked != nil:
		res, err := o.blocked(ctx, r, t.blocked)
		return res, true, err
	case !t.ok:
		return stepResult{progressed: true}, true, nil
	case jobActive(t.job.Status) && t.rec != nil:
		_, err := o.commit(ctx, r, kaggleEvent(*t.rec), t.save)
		return stepResult{progressed: true}, true, err
	case jobActive(t.job.Status):
		return stepResult{wait: o.iv.localJob}, true, nil
	}
	return stepResult{}, false, nil
}

// nextAttempt stores the receipt and moves the step to attempt+1; the third failed attempt fails the run with code.
func (o *Orchestrator) nextAttempt(ctx context.Context, r Run, t kaggleTurn, step, code, msg string) (stepResult, error) {
	n := r.Remote.Attempts[step] + 1
	if n >= maxStepAttempts {
		return o.fail(ctx, r, code, msg, t.save)
	}
	_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": step, "attempt": n, "status": t.job.Status}),
		func(x *Run) { t.save(x); setAttempt(x, step, n) })
	return stepResult{progressed: true}, err
}

// readOp runs a read operation (no receipt, no request ID).
func (o *Orchestrator) readOp(ctx context.Context, op string, params any) (json.RawMessage, error) {
	req, err := NewKaggleRequest("", op, "", "", params)
	if err != nil {
		return nil, err
	}
	res, err := o.drv.Execute(ctx, req)
	return res.Data, err
}

func kaggleEvent(rec KaggleStep) RunEvent {
	return NewRunEvent("kaggle", map[string]any{"step": rec.Step, "job_id": rec.JobID, "status": rec.Status, "attempt": rec.Attempt})
}

type jobResultFields struct {
	UploadRef     string `json:"upload_ref"`
	VersionNumber int    `json:"versionNumber"`
	KernelID      int64  `json:"kernelId"`
	RemoteStatus  string `json:"remote_status"`
}

func jobFields(j kaggle.Job) jobResultFields {
	var f jobResultFields
	_ = json.Unmarshal(j.Result, &f)
	return f
}

// remoteTerminalState mirrors kaggle's remoteState (remote_jobs.go): completed, failed, cancelled or "" while running.
func remoteTerminalState(v string) string {
	v = strings.ToUpper(v)
	switch {
	case v == "COMPLETE" || v == "COMPLETED" || strings.HasSuffix(v, "_SUCCEEDED") || strings.HasSuffix(v, "_COMPLETED"):
		return KaggleJobCompleted
	case v == "ERROR" || v == "FAILED" || strings.HasSuffix(v, "_FAILED") || strings.HasSuffix(v, "_ERRORED"):
		return KaggleJobFailed
	case v == "CANCELLED" || v == "CANCELED" || v == "CANCEL_ACKNOWLEDGED" || strings.HasSuffix(v, "_CANCELLED"):
		return KaggleJobCancelled
	}
	return ""
}

// statusText reads {"status": "..."} or a bare JSON string.
func statusText(data []byte) string {
	var s string
	if json.Unmarshal(data, &s) != nil {
		var m struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(data, &m)
		s = m.Status
	}
	return strings.TrimSpace(s)
}

func sessionStatus(data []byte) string { return strings.ToUpper(statusText(data)) }

// datasetReady classifies get_dataset_status. PROVISIONAL until the proof run records the real vocabulary.
func datasetReady(data []byte) (ready, failed bool) {
	switch strings.ToLower(statusText(data)) {
	case "ready", "complete", "completed":
		return true, false
	case "error", "failed", "deleted":
		return false, true
	}
	return false, false
}
```

- [ ] 4. Create `reconcile_stages.go`:

```go
package chibisensei

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"aurago/internal/chibisensei/trainer"
)

var errSnapshotChanged = errors.New("chibisensei: the frozen snapshot changed")

func stagedFiles(s Snapshot) []StagedFile {
	var out []StagedFile
	for _, split := range SnapshotSplits {
		if f, ok := s.Files[split]; ok {
			out = append(out, StagedFile{Split: split, Name: f.Name, SHA256: f.SHA256, Bytes: f.Bytes, Rows: f.Rows})
		}
	}
	return append(out, StagedFile{Split: "marker", Name: s.Marker.Name, SHA256: s.Marker.SHA256, Bytes: s.Marker.Bytes})
}

// queued: freeze the snapshot, build job.json and the notebook (redone after a restart; local and idempotent).
func (o *Orchestrator) stepQueued(ctx context.Context, r Run) (stepResult, error) {
	plan := r.Remote.Plan
	if plan == nil {
		return o.fail(ctx, r, "packaging_failed", "The run has no stored plan.", nil)
	}
	prof, ok := o.cat.Get(r.ProfileID)
	if !ok || o.cat.SHA(r.ProfileID) != r.ProfileSHA {
		return o.fail(ctx, r, "profile_changed", "The model profile changed after the start.", nil)
	}
	proj, err := o.st.GetProject(ctx, r.ProjectID)
	if err != nil {
		return o.localError(ctx, r, fmt.Errorf("load project: %w", err))
	}
	if proj.DataRev != plan.DataRev {
		return o.fail(ctx, r, "data_changed", msgDataChanged, nil)
	}
	snap, err := o.st.FreezeSnapshot(ctx, r.ProjectID, r.ID, snapshotDir(o.dataDir, r.ID))
	var invalid *ValidationError
	switch {
	case errors.Is(err, ErrDataChanged) || errors.As(err, &invalid):
		return o.fail(ctx, r, "data_changed", msgDataChanged, nil)
	case err != nil:
		return o.localError(ctx, r, fmt.Errorf("freeze snapshot: %w", err))
	}
	if snap.DataRev != plan.DataRev || snap.Files["train"].Rows != plan.Split.Train ||
		snap.Files["val"].Rows != plan.Split.Val || snap.Files["eval"].Rows != plan.Split.Eval {
		return o.fail(ctx, r, "data_changed", msgDataChanged, nil)
	}
	created, err := time.Parse(time.RFC3339, r.Created)
	if err != nil {
		created = o.now()
	}
	lock, err := trainer.LoadLock()
	if err != nil {
		return o.fail(ctx, r, "packaging_failed", "The trainer lock file is invalid.", nil)
	}
	spec, err := BuildJobSpec(JobInput{RunID: r.ID, Created: created, AuraGoVersion: o.cfg().AuraGoVersion, Project: proj,
		Profile: prof, ProfileSHA: r.ProfileSHA, Plan: *plan, Marker: snap.Marker.Name, Format: snap.Format, Lock: lock})
	if err != nil {
		o.log.Warn("Chibi Sensei job spec refused", "run", r.ID, "error", err)
		return o.fail(ctx, r, "packaging_failed", "The training job could not be built.", nil)
	}
	job, err := spec.JSON()
	if err != nil {
		return o.localError(ctx, r, err)
	}
	nb, err := BuildNotebook(r.ID, job, trainer.Script())
	if err != nil {
		o.log.Warn("Chibi Sensei notebook refused", "run", r.ID, "error", err)
		return o.fail(ctx, r, "packaging_failed", "The notebook could not be built.", nil)
	}
	if err := writeChibiFile(o.dataDir, runRel(r.ID, notebookFileName), nb); err != nil {
		return o.localError(ctx, r, fmt.Errorf("write notebook: %w", err))
	}
	staged := stagedFiles(snap)
	return o.transition(ctx, r, RunPackaging, map[string]any{"files": len(staged), "bytes": snap.Bytes}, func(x *Run) {
		x.JobJSON, x.DataSHA = job, snap.DataSHA
		x.Remote.Staged, x.Remote.SnapshotBytes, x.Remote.Marker = staged, snap.Bytes, snap.Marker.Name
	})
}

// packaging: the Kaggle project first (stored so cleanup can find staged files), then every staged file whose
// project copy is missing or differs.
func (o *Orchestrator) stepPackaging(ctx context.Context, r Run) (stepResult, error) {
	project, err := o.drv.EnsureProject(ctx)
	if err != nil {
		return o.blocked(ctx, r, err)
	}
	if r.Remote.KaggleProject != project {
		_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": "project"}),
			func(x *Run) { x.Remote.KaggleProject = project })
		return stepResult{progressed: true}, err
	}
	root, err := os.OpenRoot(RunDir(o.dataDir, r.ID))
	if err != nil {
		return o.localError(ctx, r, err)
	}
	defer root.Close()
	for _, f := range r.Remote.Staged {
		if o.projectFileMatches(project, f) {
			continue
		}
		if err := o.stageFile(ctx, root, project, f); err != nil {
			if errors.Is(err, errSnapshotChanged) {
				return o.fail(ctx, r, "data_changed", msgDataChanged, nil)
			}
			return o.blocked(ctx, r, err)
		}
	}
	return o.transition(ctx, r, RunUploading, map[string]any{"files": len(r.Remote.Staged)}, nil)
}

func (o *Orchestrator) projectFileMatches(project string, f StagedFile) bool {
	src, err := o.drv.OpenFile(project, f.Name)
	if err != nil {
		return false
	}
	defer src.Close()
	h := sha256.New()
	n, err := io.Copy(h, src)
	return err == nil && n == f.Bytes && hex.EncodeToString(h.Sum(nil)) == f.SHA256
}

func (o *Orchestrator) stageFile(ctx context.Context, root *os.Root, project string, f StagedFile) error {
	src, err := root.Open(path.Join(snapshotSubdir, f.Name))
	if err != nil {
		return fmt.Errorf("%w: %v", errSnapshotChanged, err)
	}
	defer src.Close()
	h := sha256.New()
	n, err := o.drv.WriteFile(ctx, project, f.Name, io.TeeReader(src, h))
	if err != nil {
		return err
	}
	if n != f.Bytes || hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
		_ = o.drv.RemoveFile(project, f.Name)
		return errSnapshotChanged
	}
	return nil
}

// uploading: one upload_dataset_file per staged file, in order. completed → next file; unknown → new attempt and new
// ID (at most 3); failed → the run fails.
func (o *Orchestrator) stepUploading(ctx context.Context, r Run) (stepResult, error) {
	files := r.Remote.Staged
	for i, f := range files {
		step := StepUploadPrefix + f.Split
		if rec := r.Remote.Step(step); rec != nil && rec.Status == KaggleJobCompleted && rec.UploadRef != "" {
			continue
		}
		t, err := o.kaggleStep(ctx, r, step, "upload_dataset_file", r.Remote.KaggleProject, f.Name, nil)
		if err != nil {
			return o.localError(ctx, r, err)
		}
		if res, wait, err := o.pending(ctx, r, t); wait {
			return res, err
		}
		switch t.job.Status {
		case KaggleJobCompleted:
			ref := jobFields(t.job).UploadRef
			if !strings.HasPrefix(ref, "upload:") {
				return o.fail(ctx, r, "upload_failed", msgUploadFailed, t.save)
			}
			if t.rec == nil {
				cur := *r.Remote.Step(step)
				t.rec = &cur
			}
			t.rec.UploadRef = ref
			if i == len(files)-1 {
				return o.transition(ctx, r, RunCreatingDataset, map[string]any{"uploads": len(files)}, t.save)
			}
			_, err := o.commit(ctx, r, kaggleEvent(*t.rec), t.save)
			return stepResult{progressed: true}, err
		case KaggleJobUnknown:
			return o.nextAttempt(ctx, r, t, step, "upload_failed", msgUploadFailed)
		default:
			return o.fail(ctx, r, "upload_failed", msgUploadFailed, t.save)
		}
	}
	return o.transition(ctx, r, RunCreatingDataset, nil, nil)
}

// creating_dataset: create_dataset with the upload refs. unknown → a get_dataset read decides: exists → done;
// 404 → new attempt.
func (o *Orchestrator) stepCreatingDataset(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	files := make([]map[string]any, 0, len(rem.Staged))
	for _, f := range rem.Staged {
		rec := rem.Step(StepUploadPrefix + f.Split)
		if rec == nil || rec.UploadRef == "" {
			return o.fail(ctx, r, "upload_failed", msgUploadFailed, nil)
		}
		files = append(files, map[string]any{"token": rec.UploadRef})
	}
	params := map[string]any{"ownerSlug": rem.Owner, "slug": rem.DatasetSlug, "title": rem.DatasetSlug,
		"licenseName": "unknown", "isPrivate": true, "files": files}
	t, err := o.kaggleStep(ctx, r, StepCreateDataset, "create_dataset", "", "", params)
	if err != nil {
		return o.localError(ctx, r, err)
	}
	if res, wait, err := o.pending(ctx, r, t); wait {
		return res, err
	}
	switch t.job.Status {
	case KaggleJobCompleted:
		return o.datasetCreated(ctx, r, t)
	case KaggleJobUnknown:
		_, err := o.readOp(ctx, "get_dataset", map[string]any{"ownerSlug": rem.Owner, "datasetSlug": rem.DatasetSlug})
		switch {
		case err == nil:
			return o.datasetCreated(ctx, r, t)
		case IsKaggleNotFound(err):
			return o.nextAttempt(ctx, r, t, StepCreateDataset, "dataset_failed", msgDatasetFailed)
		}
		return o.blocked(ctx, r, err)
	}
	return o.fail(ctx, r, "dataset_failed", msgDatasetFailed, t.save)
}

func (o *Orchestrator) datasetCreated(ctx context.Context, r Run, t kaggleTurn) (stepResult, error) {
	removed := map[string]bool{}
	for _, f := range r.Remote.Staged {
		if err := o.drv.RemoveFile(r.Remote.KaggleProject, f.Name); err != nil {
			o.log.Warn("Chibi Sensei staged file not removed", "run", r.ID, "file", f.Name, "error", err)
			continue
		}
		removed[f.Name] = true
	}
	now := ts(o.now())
	return o.transition(ctx, r, RunWaitingDataset, map[string]any{"dataset": KaggleRef(r.Remote.Owner, r.Remote.DatasetSlug)},
		func(x *Run) {
			t.save(x)
			for i := range x.Remote.Staged {
				if removed[x.Remote.Staged[i].Name] {
					x.Remote.Staged[i].Removed = true
				}
			}
			x.Remote.DatasetWaitStarted = now
		})
}

// waiting_dataset: get_dataset_status every 15 s, up to 15 minutes of successful reads.
func (o *Orchestrator) stepWaitingDataset(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	data, err := o.readOp(ctx, "get_dataset_status", map[string]any{"ownerSlug": rem.Owner, "datasetSlug": rem.DatasetSlug})
	if err != nil && !IsKaggleNotFound(err) {
		return o.blocked(ctx, r, err)
	}
	if err == nil {
		ready, failed := datasetReady(data)
		if ready {
			return o.transition(ctx, r, RunStarting, nil, nil)
		}
		if failed {
			return o.fail(ctx, r, "dataset_failed", msgDatasetFailed, nil)
		}
	}
	if rem.BlockedSince != "" || rem.BlockedCode != "" { // a refused wait restarts the 15-minute window
		now := ts(o.now())
		if _, err := o.commit(ctx, r, RunEvent{}, func(x *Run) { x.Remote.DatasetWaitStarted = now }); err != nil {
			return o.localError(ctx, r, err)
		}
		return stepResult{wait: o.iv.dataset}, nil
	}
	if since, perr := time.Parse(time.RFC3339, rem.DatasetWaitStarted); perr == nil && o.now().Sub(since) > datasetWaitMax {
		return o.fail(ctx, r, "dataset_timeout", "Kaggle did not finish the dataset within 15 minutes.", nil)
	}
	return stepResult{wait: o.iv.dataset}, nil
}

// starting: save_kernel SAVE_AND_RUN_ALL. remote_running → running. unknown → a get_kernel_session_status read:
// it answers → running untracked; 404 → kaggle_submit_unknown (nothing is sent again).
func (o *Orchestrator) stepStarting(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	if cfg := o.cfg(); !submitted(r) && cfg.RemoteTimeoutMinutes > 0 && rem.SessionTimeoutMinutes > cfg.RemoteTimeoutMinutes {
		return o.fail(ctx, r, "submit_failed", "kaggle.remote_timeout_minutes is now shorter than the session of this run.", nil)
	}
	nb, err := readChibiFile(o.dataDir, runRel(r.ID, notebookFileName), maxNotebookBytes)
	if err != nil {
		return o.fail(ctx, r, "packaging_failed", "The notebook of the run is missing.", nil)
	}
	params := map[string]any{"slug": KaggleRef(rem.Owner, rem.NotebookSlug), "newTitle": rem.NotebookSlug,
		"text": string(nb), "kernelType": "notebook", "language": "python", "kernelExecutionType": "SAVE_AND_RUN_ALL",
		"isPrivate": true, "enableGpu": true, "enableInternet": true, "machineShape": rem.MachineShape,
		"datasetDataSources":    []string{KaggleRef(rem.Owner, rem.DatasetSlug)},
		"sessionTimeoutSeconds": rem.SessionTimeoutMinutes * 60}
	t, err := o.kaggleStep(ctx, r, StepSaveKernel, "save_kernel", "", "", params)
	if err != nil {
		return o.localError(ctx, r, err)
	}
	if t.rec != nil && t.rec.JobID != "" && rem.Submitted == "" {
		at := ts(o.now())
		t.extra = func(x *Run) { x.Remote.Submitted = at }
	}
	if res, wait, err := o.pending(ctx, r, t); wait {
		return res, err
	}
	f := jobFields(t.job)
	switch {
	case t.job.Status == KaggleJobUnknown:
		data, err := o.readOp(ctx, "get_kernel_session_status", map[string]any{"userName": rem.Owner, "kernelSlug": rem.NotebookSlug})
		switch {
		case err == nil:
			return o.startRunning(ctx, r, t, true, 1, sessionStatus(data))
		case IsKaggleNotFound(err) && r.CancelRequested:
			return o.endRun(ctx, r, RunCancelled, codeCancelled, "", t.save)
		case IsKaggleNotFound(err):
			return o.fail(ctx, r, "kaggle_submit_unknown", "Kaggle did not confirm the notebook start. Nothing was sent again.", t.save)
		}
		return o.blocked(ctx, r, err)
	case t.job.Status == KaggleJobRemoteRunning || t.job.Status == KaggleJobCancelRequested || f.VersionNumber > 0:
		return o.startRunning(ctx, r, t, false, f.VersionNumber, f.RemoteStatus)
	case r.CancelRequested:
		return o.endRun(ctx, r, RunCancelled, codeCancelled, "", t.save)
	case t.job.Error == "concurrent Kaggle compute limit reached":
		return o.fail(ctx, r, "kaggle_slot_busy", "Another Kaggle notebook run holds the compute slot.", t.save)
	}
	return o.fail(ctx, r, "submit_failed", "Kaggle refused the notebook start.", t.save)
}

func (o *Orchestrator) startRunning(ctx context.Context, r Run, t kaggleTurn, untracked bool, version int, status string) (stepResult, error) {
	now := o.now().UTC()
	if version < 1 {
		version = 1 // the slug is new per run, so its first and only version is 1
	}
	kernelID := jobFields(t.job).KernelID
	deadline := now.Add(time.Duration(r.Remote.SessionTimeoutMinutes)*time.Minute + deadlineSlack)
	return o.transition(ctx, r, RunRunning, map[string]any{"untracked": untracked, "deadline": ts(deadline)}, func(x *Run) {
		t.save(x)
		if x.Remote.Submitted == "" {
			x.Remote.Submitted = ts(now)
		}
		x.Remote.Untracked, x.Remote.KernelVersion, x.Remote.KernelID = untracked, version, kernelID
		x.Remote.RemoteStatus, x.Remote.RemoteStarted, x.Deadline = status, ts(now), ts(deadline)
	})
}
```

- [ ] 5. Create `reconcile_running.go`:

```go
package chibisensei

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

const maxLogEventsPerRead = 200

type remoteView struct {
	status  string // upper-case Kaggle session status
	final   string // completed | failed | cancelled; "" while running
	polled  bool   // a read happened
	untrack bool   // the tracked job lost its outcome: switch to status reads
}

// readRemote reads the tracked save_kernel job, or (untracked) get_kernel_session_status at most every 10 s
// unless force.
func (o *Orchestrator) readRemote(ctx context.Context, r Run, force bool) (remoteView, error) {
	rem := r.Remote
	if !rem.Untracked {
		rec := rem.Step(StepSaveKernel)
		if rec == nil || rec.JobID == "" {
			return remoteView{untrack: true}, nil
		}
		job, err := o.drv.Job(ctx, rec.JobID)
		if err != nil {
			return remoteView{}, err
		}
		v := remoteView{status: jobFields(job).RemoteStatus, polled: true}
		switch job.Status {
		case KaggleJobCompleted, KaggleJobFailed, KaggleJobCancelled:
			v.final = job.Status
		case KaggleJobUnknown, KaggleJobResolvedUnknown:
			v.untrack = true
		}
		return v, nil
	}
	if last, err := time.Parse(time.RFC3339, rem.LastPolled); !force && err == nil && o.now().Sub(last) < o.iv.untrackedPoll {
		return remoteView{status: rem.RemoteStatus}, nil
	}
	data, err := o.readOp(ctx, "get_kernel_session_status", map[string]any{"userName": rem.Owner,
		"kernelSlug": rem.NotebookSlug, "versionLabel": strconv.Itoa(max(rem.KernelVersion, 1))})
	if IsKaggleNotFound(err) {
		return remoteView{status: "NOT_FOUND", final: KaggleJobFailed, polled: true}, nil
	}
	if err != nil {
		return remoteView{}, err
	}
	st := sessionStatus(data)
	return remoteView{status: st, final: remoteTerminalState(st), polled: true}, nil
}

// running: watch the remote, enforce the own deadline (remote start + session + 20 min; one final read decides) and
// read the live log every 60 s.
func (o *Orchestrator) stepRunning(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	now := o.now().UTC()
	deadline, _ := time.Parse(time.RFC3339, r.Deadline)
	late := !deadline.IsZero() && now.After(deadline)
	v, err := o.readRemote(ctx, r, late)
	switch {
	case err != nil && ctx.Err() != nil:
		return stepResult{}, ctx.Err()
	case err != nil && late:
		return o.timedOut(ctx, r)
	case err != nil:
		return o.blocked(ctx, r, err)
	case v.untrack:
		_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": StepSaveKernel, "untracked": true}),
			func(x *Run) { x.Remote.Untracked, x.Remote.KernelVersion = true, max(x.Remote.KernelVersion, 1) })
		return stepResult{progressed: true}, err
	case v.final != "":
		setStatus := func(x *Run) {
			if v.status != "" {
				x.Remote.RemoteStatus = v.status
			}
		}
		if r.CancelRequested {
			return o.endRun(ctx, r, RunCancelled, codeCancelled, "", setStatus)
		}
		return o.transition(ctx, r, RunDownloading, map[string]any{"remote": v.final, "remote_status": v.status}, setStatus)
	case late:
		return o.timedOut(ctx, r)
	}
	var muts []func(*Run)
	ev := RunEvent{}
	if v.polled && rem.Untracked {
		muts = append(muts, func(x *Run) { x.Remote.LastPolled = ts(now) })
	}
	if v.status != "" && v.status != rem.RemoteStatus {
		muts = append(muts, func(x *Run) { x.Remote.RemoteStatus = v.status })
		ev = NewRunEvent("kaggle", map[string]any{"remote_status": v.status})
	}
	var logged *logOutcome
	if last, perr := time.Parse(time.RFC3339, rem.LastLogRead); perr != nil || now.Sub(last) >= o.iv.logRead {
		out, err := o.readLog(ctx, r)
		if err != nil {
			return stepResult{}, err
		}
		if err := o.st.UpsertMetrics(ctx, r.ID, out.metrics); err != nil {
			return o.localError(ctx, r, err)
		}
		logged = &out
		muts = append(muts, out.apply)
	}
	if len(muts) == 0 && rem.BlockedSince == "" && rem.BlockedCode == "" {
		return stepResult{wait: o.iv.running}, nil
	}
	next, err := o.commit(ctx, r, ev, func(x *Run) {
		for _, m := range muts {
			m(x)
		}
	})
	if err != nil {
		return o.localError(ctx, r, err)
	}
	if logged != nil {
		o.afterLog(ctx, next, *logged)
	}
	return stepResult{wait: o.iv.running}, nil
}

func (o *Orchestrator) timedOut(ctx context.Context, r Run) (stepResult, error) {
	if r.CancelRequested {
		return o.endRun(ctx, r, RunCancelled, codeCancelled, "", func(x *Run) { x.Remote.WarningCode = "remote_timeout" })
	}
	url := KaggleNotebookURL(KaggleRef(r.Remote.Owner, r.Remote.NotebookSlug))
	return o.fail(ctx, r, "remote_timeout", "The Kaggle session did not end in time. Stop it on Kaggle: "+url, nil)
}

type logOutcome struct {
	readAt  string
	events  []LogEvent // new events (seq > last stored)
	metrics []Metric
	stage   *RunStage
	parsed  bool // the body had at least one @@chibi line
	maxSeq  int64
}

// readLog reads the whole log (A1, A2), stores the scrubbed 64 KiB tail and parses the new @@chibi lines. Only a
// context error is returned; a refused or empty read counts as an empty read.
func (o *Orchestrator) readLog(ctx context.Context, r Run) (logOutcome, error) {
	rem := r.Remote
	out := logOutcome{readAt: ts(o.now()), maxSeq: rem.LastLogSeq}
	body, err := o.drv.ReadKernelLog(ctx, rem.Owner, rem.NotebookSlug, max(rem.KernelVersion, 1), logReadMaxBytes)
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	if err != nil || len(body) == 0 {
		if err != nil {
			o.log.Info("Chibi Sensei kernel log not readable", "run", r.ID, "error", err)
		}
		return out, nil
	}
	if err := writeLogTail(o.dataDir, r.ID, body); err != nil {
		o.log.Warn("Chibi Sensei log tail not stored", "run", r.ID, "error", err)
	}
	evs := ParseKernelLog(body)
	out.parsed = len(evs) > 0
	var stage *RunStage
	if rem.Stage != nil {
		s := *rem.Stage
		stage = &s
	}
	for _, e := range evs {
		if e.Seq <= rem.LastLogSeq {
			continue
		}
		out.events = append(out.events, e)
		out.maxSeq = max(out.maxSeq, e.Seq)
		if s, ok := e.Stage(); ok {
			if stage == nil || stage.Name != s.Stage {
				stage = &RunStage{Name: s.Stage}
			}
			stage.State = s.State
		}
		if m, ok := e.Metric(); ok {
			out.metrics = append(out.metrics, m)
			if m.Kind == "train" {
				if stage == nil || stage.Name != "train" {
					stage = &RunStage{Name: "train", State: "start"}
				}
				stage.Step, stage.MaxSteps, stage.Epoch, stage.EtaS = m.Step, m.MaxSteps, m.Epoch, int(m.EtaS)
			}
		}
	}
	out.stage = stage
	return out, nil
}

// apply: live is yes after the first parsed line, no after three reads without one (unless already yes).
func (l logOutcome) apply(x *Run) {
	x.Remote.LastLogRead = l.readAt
	if l.parsed {
		x.Remote.Live, x.Remote.EmptyLogReads = LiveYes, 0
	} else {
		x.Remote.EmptyLogReads++
		if x.Remote.EmptyLogReads >= 3 && x.Remote.Live != LiveYes {
			x.Remote.Live = LiveNo
		}
	}
	if l.maxSeq > x.Remote.LastLogSeq {
		x.Remote.LastLogSeq = l.maxSeq
	}
	if l.stage != nil {
		s := *l.stage
		x.Remote.Stage = &s
	}
}

// afterLog writes the run events of the new lines (at most 200 per read) and publishes metric and stage events.
func (o *Orchestrator) afterLog(ctx context.Context, r Run, l logOutcome) {
	if o.hub != nil && len(l.metrics) > 0 {
		o.hub.Publish(r.ProjectID, Event{Name: "metric", Data: map[string]any{"run_id": r.ID, "points": l.metrics}})
	}
	n := 0
	for _, e := range l.events {
		ev, ok := logRunEvent(e)
		if !ok {
			continue
		}
		if n++; n > maxLogEventsPerRead {
			return
		}
		if err := o.st.AddRunEvent(ctx, r.ID, ev); err != nil {
			o.log.Warn("Chibi Sensei log event not stored", "run", r.ID, "error", err)
			return
		}
		if s, ok := e.Stage(); ok && o.hub != nil {
			o.hub.Publish(r.ProjectID, Event{Name: "stage", Data: map[string]any{"run_id": r.ID, "stage": s.Stage, "state": s.State}})
		}
	}
}

func logRunEvent(e LogEvent) (RunEvent, bool) {
	if s, ok := e.Stage(); ok {
		return NewRunEvent("stage", map[string]any{"stage": s.Stage, "state": s.State, "seq": e.Seq}), true
	}
	if x, ok := e.Error(); ok {
		return NewRunEvent("error", map[string]any{"stage": x.Stage, "code": x.Code, "message": scrubText(x.Message, 500), "seq": e.Seq}), true
	}
	if c, ok := e.ExitCode(); ok {
		return NewRunEvent("log", map[string]any{"exit_code": c}), true
	}
	if e.T == "warn" {
		var w struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(e.Raw, &w)
		if !codePattern.MatchString(w.Code) {
			w.Code = ""
		}
		return NewRunEvent("warn", map[string]any{"source": "trainer", "code": w.Code, "message": scrubText(w.Message, 500), "seq": e.Seq}), true
	}
	return RunEvent{}, false
}
```

- [ ] 6. Create `reconcile_outputs.go`:

```go
package chibisensei

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"aurago/internal/fileutil"
)

// jobFacts are the job.json fields the later states need (read from runs.job_json, never from the catalog).
type jobFacts struct {
	Recipe   string   `json:"recipe"`
	TaskKind *string  `json:"task_kind"`
	Labels   []string `json:"labels"`
	Profile  struct {
		Base struct {
			Revision string `json:"revision"`
		} `json:"base"`
	} `json:"profile"`
	Deps struct {
		LlamaCpp struct {
			Tag string `json:"tag"`
		} `json:"llama_cpp"`
	} `json:"deps"`
}

func parseJobFacts(b []byte) (jobFacts, error) {
	var f jobFacts
	if err := json.Unmarshal(b, &f); err != nil {
		return jobFacts{}, fmt.Errorf("job facts: %w", err)
	}
	return f, nil
}

func (f jobFacts) converter() string { return BaseGGUFConverter(f.Deps.LlamaCpp.Tag) }

func (f jobFacts) taskKind() string {
	if f.TaskKind == nil {
		return ""
	}
	return *f.TaskKind
}

type exhaustedFunc func(ctx context.Context, r Run, t kaggleTurn) (stepResult, error)

// downloading: a final log read; chibi-status.json; chibi-manifest.json; the manifest's files (one call per step);
// then metrics, report and the next state.
func (o *Orchestrator) stepDownloading(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	if !rem.FinalLogRead {
		return o.finalLog(ctx, r)
	}
	facts, err := parseJobFacts(r.JobJSON)
	if err != nil {
		return o.fail(ctx, r, "packaging_failed", "The job of the run is unreadable.", nil)
	}
	st := findOutput(rem.Outputs, OutStatus)
	if st == nil || !st.Copied {
		return o.fetch(ctx, r, facts, RunOutput{Path: OutStatus, Kind: "status", Bytes: -1},
			func(ctx context.Context, r Run, t kaggleTurn) (stepResult, error) {
				return o.fail(ctx, r, "status_missing", msgStatusMissing, t.save)
			})
	}
	b, err := o.readOutput(r, facts, "status", smallOutputMax)
	var status Status
	if err == nil {
		status, err = ParseStatus(b)
	}
	if err != nil || status.RunID != r.ID {
		return o.fail(ctx, r, "status_missing", msgStatusMissing, nil)
	}
	mf := findOutput(rem.Outputs, OutManifest)
	if mf == nil || (!mf.Copied && !mf.Missing) {
		return o.fetch(ctx, r, facts, RunOutput{Path: OutManifest, Kind: "manifest", Bytes: -1},
			func(ctx context.Context, r Run, t kaggleTurn) (stepResult, error) {
				if status.Succeeded() {
					return o.fail(ctx, r, "download_failed", msgDownloadFailed, t.save)
				}
				_, err := o.commit(ctx, r, NewRunEvent("warn", map[string]any{"code": "manifest_missing"}), func(x *Run) {
					t.save(x)
					putOutput(x, RunOutput{Path: OutManifest, Kind: "manifest", Bytes: -1, Missing: true})
				})
				return stepResult{progressed: true}, err
			})
	}
	if !rem.OutputsPlanned {
		return o.planOutputs(ctx, r, facts, status, *st, *mf)
	}
	for _, out := range rem.Outputs {
		if out.Copied || out.Missing || out.Kind == "status" || out.Kind == "manifest" {
			continue
		}
		return o.fetch(ctx, r, facts, out, func(ctx context.Context, r Run, t kaggleTurn) (stepResult, error) {
			return o.fail(ctx, r, "download_failed", msgDownloadFailed, t.save)
		})
	}
	return o.finishDownloads(ctx, r, facts, status)
}

func (o *Orchestrator) finalLog(ctx context.Context, r Run) (stepResult, error) {
	l, err := o.readLog(ctx, r)
	if err != nil {
		return stepResult{}, err
	}
	if err := o.st.UpsertMetrics(ctx, r.ID, l.metrics); err != nil {
		return o.localError(ctx, r, err)
	}
	next, err := o.commit(ctx, r, RunEvent{}, func(x *Run) { l.apply(x); x.Remote.FinalLogRead = true })
	if err != nil {
		return o.localError(ctx, r, err)
	}
	o.afterLog(ctx, next, l)
	return stepResult{progressed: true}, nil
}

// fetch moves one output one call further. A new attempt first checks the limits, drops a stale staged copy and
// stores its request ID; then download_kernel_output into r<run12>-<name>; on completion the copy is verified into
// Chibi storage, registered and the staged copy removed. failed, unknown or a vanished staged copy → attempt+1
// (at most 3, then exhausted).
func (o *Orchestrator) fetch(ctx context.Context, r Run, facts jobFacts, out RunOutput, exhausted exhaustedFunc) (stepResult, error) {
	rem := r.Remote
	step := StepDownloadPrefix + out.Path
	attempt := rem.Attempts[step]
	id := KaggleRequestID(r.ID, step, attempt)
	staged := stagedOutputName(r.ID, out.Path)
	if rec := rem.Step(step); rec == nil || rec.RequestID != id {
		if code, msg := o.checkRoom(out); code != "" {
			return o.fail(ctx, r, code, msg, nil)
		}
		if err := o.drv.RemoveFile(rem.KaggleProject, staged); err != nil {
			return o.blocked(ctx, r, err)
		}
		prep := KaggleStep{Step: step, Attempt: attempt, RequestID: id, File: out.Path, Updated: ts(o.now())}
		_, err := o.commit(ctx, r, RunEvent{}, func(x *Run) { x.Remote.PutStep(prep); putOutput(x, out) })
		return stepResult{progressed: true}, err
	}
	params := map[string]any{"ownerSlug": rem.Owner, "kernelSlug": rem.NotebookSlug, "filePath": out.Path}
	if rem.KernelVersion > 0 {
		params["versionNumber"] = rem.KernelVersion
	}
	t, err := o.kaggleStep(ctx, r, step, "download_kernel_output", rem.KaggleProject, staged, params)
	if err != nil {
		return o.localError(ctx, r, err)
	}
	if res, wait, err := o.pending(ctx, r, t); wait {
		return res, err
	}
	if t.job.Status != KaggleJobCompleted {
		return o.nextOutputAttempt(ctx, r, t, step, exhausted)
	}
	rel, err := outputRel(r, facts, out.Kind)
	if err != nil {
		return o.fail(ctx, r, "manifest_invalid", msgManifestInvalid, t.save)
	}
	src, err := o.drv.OpenFile(rem.KaggleProject, staged)
	if errors.Is(err, fs.ErrNotExist) {
		return o.nextOutputAttempt(ctx, r, t, step, exhausted)
	}
	if err != nil {
		return o.blocked(ctx, r, err)
	}
	limit := out.Bytes
	if limit < 0 {
		limit = smallOutputMax
	}
	root, err := openChibiRoot(o.dataDir)
	if err != nil {
		src.Close()
		return o.localError(ctx, r, err)
	}
	n, sum, err := copyVerified(root, rel, src, out.Bytes, out.SHA256, limit)
	src.Close()
	root.Close()
	switch {
	case errors.Is(err, errChecksumMismatch):
		return o.fail(ctx, r, "checksum_mismatch", msgChecksum, t.save)
	case errors.Is(err, errOutputTooLarge):
		return o.fail(ctx, r, "artifact_too_large", "A result file is larger than its manifest entry.", t.save)
	case err != nil:
		return o.localError(ctx, r, err)
	}
	got := out
	got.Bytes, got.SHA256, got.Copied = n, sum, true
	if err := o.register(ctx, r, facts, got, rel); err != nil {
		return o.localError(ctx, r, err)
	}
	if err := o.drv.RemoveFile(rem.KaggleProject, staged); err != nil {
		o.log.Warn("Chibi Sensei staged download not removed", "run", r.ID, "file", staged, "error", err)
	}
	_, err = o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": step, "status": KaggleJobCompleted, "bytes": n}),
		func(x *Run) { t.save(x); putOutput(x, got) })
	return stepResult{progressed: true}, err
}

func (o *Orchestrator) nextOutputAttempt(ctx context.Context, r Run, t kaggleTurn, step string, exhausted exhaustedFunc) (stepResult, error) {
	n := r.Remote.Attempts[step] + 1
	if n >= maxStepAttempts {
		return exhausted(ctx, r, t)
	}
	_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": step, "attempt": n, "status": t.job.Status}),
		func(x *Run) { t.save(x); setAttempt(x, step, n) })
	return stepResult{progressed: true}, err
}

// checkRoom refuses a download that exceeds kaggle.max_transfer_mb, the storage cap or free disk, before anything
// is transferred.
func (o *Orchestrator) checkRoom(out RunOutput) (string, string) {
	cfg := o.cfg()
	need := out.Bytes
	if need < 0 {
		need = smallOutputMax
	}
	if cfg.MaxTransferBytes > 0 && need > cfg.MaxTransferBytes {
		return "artifact_too_large", "A result file is larger than kaggle.max_transfer_mb allows."
	}
	if cfg.StorageMaxBytes > 0 && StorageBytes(o.dataDir)+o.st.SizeBytes()+need > cfg.StorageMaxBytes {
		return "storage_full", "The Chibi Sensei storage limit cannot hold the results."
	}
	if free, err := fileutil.FreeDiskBytes(o.dataDir); err == nil && free < need+minFreeDiskBytes {
		return "storage_full", "The disk has no room for the results."
	}
	return "", ""
}

// planOutputs puts the manifest's files into Outputs: all of them for a succeeded status (gguf_tuned required, the
// status entry must match the downloaded status), only metrics for a failed one.
func (o *Orchestrator) planOutputs(ctx context.Context, r Run, facts jobFacts, status Status, st, manifest RunOutput) (stepResult, error) {
	ok := status.Succeeded()
	var list []RunOutput
	if manifest.Copied {
		b, err := o.readOutput(r, facts, "manifest", smallOutputMax)
		var m Manifest
		if err == nil {
			m, err = ParseManifest(b)
		}
		if err == nil && (m.RunID != r.ID || m.ProfileID != r.ProfileID || m.ProfileSHA != r.ProfileSHA) {
			err = errors.New("the manifest belongs to another run")
		}
		if err != nil {
			if ok {
				return o.fail(ctx, r, "manifest_invalid", msgManifestInvalid, nil)
			}
			m = Manifest{}
		}
		for _, f := range m.Files {
			if f.Kind == "status" {
				if ok && (f.Bytes != st.Bytes || f.SHA256 != st.SHA256) {
					return o.fail(ctx, r, "checksum_mismatch", msgChecksum, nil)
				}
				continue
			}
			if !ok && f.Kind != "metrics" {
				continue
			}
			out := RunOutput{Path: f.Path, Kind: f.Kind, Bytes: f.Bytes, SHA256: f.SHA256}
			if f.Kind == "gguf_base" {
				reused, err := o.reuseBaseGGUF(ctx, r, facts, out)
				if err != nil {
					return o.localError(ctx, r, err)
				}
				out.Copied = reused
			}
			list = append(list, out)
		}
	}
	if ok && findKind(list, "gguf_tuned") == nil {
		return o.fail(ctx, r, "manifest_invalid", msgManifestInvalid, nil)
	}
	_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": "outputs", "files": len(list)}), func(x *Run) {
		for _, out := range list {
			putOutput(x, out)
		}
		x.Remote.OutputsPlanned = true
	})
	return stepResult{progressed: true}, err
}

// reuseBaseGGUF registers an identical base GGUF that is already stored instead of downloading it again.
func (o *Orchestrator) reuseBaseGGUF(ctx context.Context, r Run, facts jobFacts, out RunOutput) (bool, error) {
	b, found, err := o.st.GetBaseGGUF(ctx, r.ProfileID, facts.Profile.Base.Revision, facts.converter())
	if err != nil || !found || b.SHA256 != out.SHA256 {
		return false, err
	}
	root, err := openChibiRoot(o.dataDir)
	if err != nil {
		return false, err
	}
	defer root.Close()
	if info, err := root.Stat(b.RelPath); err != nil || info.Size() != out.Bytes {
		return false, nil
	}
	return true, o.st.AddArtifact(ctx, Artifact{RunID: r.ID, ProjectID: r.ProjectID, Kind: "gguf_base", RelPath: b.RelPath,
		Bytes: out.Bytes, SHA256: out.SHA256})
}

func (o *Orchestrator) finishDownloads(ctx context.Context, r Run, facts jobFacts, status Status) (stepResult, error) {
	if out := findKind(r.Remote.Outputs, "metrics"); out != nil && out.Copied {
		f, err := o.openOutput(r, facts, "metrics")
		if err != nil {
			return o.localError(ctx, r, err)
		}
		points, err := ParseMetricsFile(f)
		f.Close()
		if err != nil {
			return o.localError(ctx, r, err)
		}
		if err := o.st.ReplaceMetrics(ctx, r.ID, points); err != nil {
			return o.localError(ctx, r, err)
		}
	}
	if out := findKind(r.Remote.Outputs, "eval_samples"); status.Succeeded() && out != nil && out.Copied {
		f, err := o.openOutput(r, facts, "eval_samples")
		if err != nil {
			return o.localError(ctx, r, err)
		}
		limit := 0
		if r.Remote.Plan != nil {
			limit = r.Remote.Plan.EvalSamples
		}
		samples, err := ParseEvalSamples(f, limit)
		f.Close()
		if err != nil {
			return o.localError(ctx, r, err)
		}
		rep := ScoreReport(facts.Recipe, facts.taskKind(), facts.Labels, status, samples)
		rep.RunID = r.ID
		if err := o.st.SaveReport(ctx, r.ID, "auto", rep); err != nil {
			return o.localError(ctx, r, err)
		}
	}
	res := runResult(status)
	warning := status.WarningCode
	if warning == "" && status.Train.StopReason == "time_budget" {
		warning = "time_budget"
	}
	mut := func(x *Run) { x.Remote.Result, x.Remote.WarningCode = &res, warning }
	if !status.Succeeded() {
		code := status.Code
		if code == "" {
			code = "trainer_failed"
		}
		return o.fail(ctx, r, code, scrubText(status.Message, 500), mut)
	}
	return o.transition(ctx, r, RunVerifying, map[string]any{"stop_reason": status.Train.StopReason}, mut)
}

func (o *Orchestrator) readOutput(r Run, facts jobFacts, kind string, limit int64) ([]byte, error) {
	rel, err := outputRel(r, facts, kind)
	if err != nil {
		return nil, err
	}
	return readChibiFile(o.dataDir, rel, limit)
}

func (o *Orchestrator) openOutput(r Run, facts jobFacts, kind string) (*os.File, error) {
	rel, err := outputRel(r, facts, kind)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(ChibiDir(o.dataDir))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(rel)
}

func (o *Orchestrator) register(ctx context.Context, r Run, facts jobFacts, out RunOutput, rel string) error {
	if out.Kind == "manifest" {
		return nil
	}
	if err := o.st.AddArtifact(ctx, Artifact{RunID: r.ID, ProjectID: r.ProjectID, Kind: out.Kind, RelPath: rel,
		Bytes: out.Bytes, SHA256: out.SHA256}); err != nil {
		return err
	}
	if out.Kind != "gguf_base" {
		return nil
	}
	return o.st.PutBaseGGUF(ctx, BaseGGUF{ProfileID: r.ProfileID, BaseRevision: facts.Profile.Base.Revision,
		Converter: facts.converter(), RelPath: rel, SHA256: out.SHA256})
}

func findOutput(outs []RunOutput, p string) *RunOutput {
	for i := range outs {
		if outs[i].Path == p {
			return &outs[i]
		}
	}
	return nil
}

func findKind(outs []RunOutput, kind string) *RunOutput {
	for i := range outs {
		if outs[i].Kind == kind {
			return &outs[i]
		}
	}
	return nil
}

func putOutput(x *Run, out RunOutput) {
	if cur := findOutput(x.Remote.Outputs, out.Path); cur != nil {
		*cur = out
		return
	}
	x.Remote.Outputs = append(x.Remote.Outputs, out)
}

func runResult(s Status) RunResult {
	return RunResult{Steps: s.Train.Steps, Epochs: s.Train.Epochs, TrainLoss: finitePtr(s.Train.TrainLoss),
		EvalLossBase: finitePtr(s.Train.EvalLossBase), EvalLossTuned: finitePtr(s.Train.EvalLossTuned),
		StopReason: s.Train.StopReason, GPU: s.GPU.Name, Versions: &RunVersions{Python: s.Versions.Python,
			Torch: s.Versions.Torch, Transformers: s.Versions.Transformers, PEFT: s.Versions.PEFT, LlamaCpp: s.Versions.LlamaCpp}}
}

// verifying: GGUF header of the tuned model (and of the base model when present) against profile.gguf.expected_arch.
// A bad base GGUF is dropped with the warning base_gguf_invalid; a bad tuned model fails the run.
func (o *Orchestrator) stepVerifying(ctx context.Context, r Run) (stepResult, error) {
	prof, ok := o.cat.Get(r.ProfileID)
	facts, err := parseJobFacts(r.JobJSON)
	if !ok || err != nil {
		return o.fail(ctx, r, "profile_changed", "The model profile is no longer available.", nil)
	}
	root, err := openChibiRoot(o.dataDir)
	if err != nil {
		return o.localError(ctx, r, err)
	}
	defer root.Close()
	rel, _ := outputRel(r, facts, "gguf_tuned")
	info, err := readGGUFInRoot(root, rel)
	if err != nil {
		if !errors.Is(err, ErrGGUFInvalid) && !errors.Is(err, fs.ErrNotExist) {
			return o.localError(ctx, r, err)
		}
		return o.fail(ctx, r, "gguf_failed", "The trained model is not a valid GGUF file.", nil)
	}
	if info.Architecture != prof.GGUF.ExpectedArch {
		return o.fail(ctx, r, "gguf_arch_mismatch", fmt.Sprintf("The model file has architecture %q instead of %q.",
			info.Architecture, prof.GGUF.ExpectedArch), nil)
	}
	warning := r.Remote.WarningCode
	if base := findKind(r.Remote.Outputs, "gguf_base"); base != nil && base.Copied {
		brel, err := outputRel(r, facts, "gguf_base")
		if err != nil {
			return o.localError(ctx, r, err)
		}
		if binfo, err := readGGUFInRoot(root, brel); err != nil || binfo.Architecture != prof.GGUF.ExpectedArch {
			if err := o.dropBaseGGUF(ctx, r, facts, root, brel); err != nil {
				return o.localError(ctx, r, err)
			}
			if warning == "" {
				warning = "base_gguf_invalid"
			}
		}
	}
	return o.transition(ctx, r, RunCleaning, map[string]any{"end_state": RunSucceeded, "gguf": info}, func(x *Run) {
		x.Remote.EndState, x.Remote.GGUF, x.Remote.WarningCode = RunSucceeded, &info, warning
	})
}

func (o *Orchestrator) dropBaseGGUF(ctx context.Context, r Run, facts jobFacts, root *os.Root, rel string) error {
	if err := removeQuiet(root, rel); err != nil {
		return err
	}
	if err := o.st.deleteBaseGGUF(ctx, r.ProfileID, facts.Profile.Base.Revision, facts.converter()); err != nil {
		return err
	}
	return o.st.deleteArtifact(ctx, ArtifactID(r.ID, "gguf_base"))
}
```

- [ ] 7. Create `reconcile_cleanup.go`:

```go
package chibisensei

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
)

const (
	cleanupStaged   = "staged"
	cleanupDataset  = "dataset"
	cleanupNotebook = "notebook"
	cleanDeleted    = "deleted"
	cleanAbsent     = "absent"
	cleanKept       = "kept"
	cleanFailed     = "failed"
)

func markCleanup(x *Run, res, outcome string) {
	if x.Remote.Cleanup == nil {
		x.Remote.Cleanup = map[string]string{}
	}
	x.Remote.Cleanup[res] = outcome
}

// cleaning (every run passes here): wait for this run's local Kaggle jobs, remove every staged r<run12>- file,
// delete dataset then notebook when the run's delete flag and kaggle.allow_delete allow it, record leftovers, end
// in Remote.EndState. It never waits for Kaggle: a refused or failed delete becomes a leftover.
func (o *Orchestrator) stepCleaning(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	for _, rec := range rem.Jobs {
		if rec.JobID == "" || !jobActive(rec.Status) || rec.Step == StepDeleteDataset || rec.Step == StepDeleteKernel {
			continue
		}
		if job, err := o.drv.Job(ctx, rec.JobID); err == nil && jobActive(job.Status) {
			return stepResult{wait: o.iv.localJob}, nil
		}
	}
	if rem.Cleanup[cleanupStaged] == "" {
		return o.removeStaged(ctx, r)
	}
	for _, res := range []string{cleanupDataset, cleanupNotebook} {
		if rem.Cleanup[res] == "" {
			return o.cleanResource(ctx, r, res)
		}
	}
	return o.finishRun(ctx, r)
}

func (o *Orchestrator) removeStaged(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	if rem.KaggleProject != "" {
		var names []string
		for _, f := range rem.Staged {
			names = append(names, f.Name)
		}
		for _, out := range []string{OutStatus, OutManifest, OutMetrics, OutEvalSamples, OutModelGGUF, OutBaseGGUF, OutAdapter, OutAdapterConfig} {
			names = append(names, stagedOutputName(r.ID, out))
		}
		for _, n := range names {
			if err := o.drv.RemoveFile(rem.KaggleProject, n); err != nil {
				o.log.Warn("Chibi Sensei staged file not removed", "run", r.ID, "file", n, "error", err)
			}
		}
	}
	_, err := o.commit(ctx, r, RunEvent{}, func(x *Run) {
		markCleanup(x, cleanupStaged, "done")
		for i := range x.Remote.Staged {
			x.Remote.Staged[i].Removed = true
		}
	})
	return stepResult{progressed: true}, err
}

func (o *Orchestrator) cleanResource(ctx context.Context, r Run, res string) (stepResult, error) {
	rem := r.Remote
	step, op, read := StepDeleteDataset, "delete_dataset", "get_dataset"
	params := map[string]any{"ownerSlug": rem.Owner, "datasetSlug": rem.DatasetSlug}
	created := rem.Step(StepCreateDataset)
	exists := created != nil && created.JobID != ""
	if res == cleanupNotebook {
		step, op, read = StepDeleteKernel, "delete_kernel", "get_kernel"
		params = map[string]any{"userName": rem.Owner, "kernelSlug": rem.NotebookSlug}
		exists = submitted(r)
	}
	if !exists {
		return o.cleanupDone(ctx, r, res, cleanAbsent, kaggleTurn{})
	}
	if !(o.cfg().AllowDelete && rem.DeleteRemoteAfterRun) {
		return o.cleanupDone(ctx, r, res, cleanKept, kaggleTurn{})
	}
	if rec := rem.Step(step); rec == nil || rec.JobID == "" {
		if _, err := o.readOp(ctx, read, params); err != nil {
			if ctx.Err() != nil {
				return stepResult{}, ctx.Err()
			}
			if IsKaggleNotFound(err) {
				return o.cleanupDone(ctx, r, res, cleanAbsent, kaggleTurn{})
			}
			return o.cleanupDone(ctx, r, res, cleanFailed, kaggleTurn{})
		}
	}
	t, err := o.kaggleStep(ctx, r, step, op, "", "", params)
	if err != nil {
		return o.localError(ctx, r, err)
	}
	if t.blocked != nil {
		return o.cleanupDone(ctx, r, res, cleanFailed, kaggleTurn{})
	}
	if out, wait, err := o.pending(ctx, r, t); wait {
		return out, err
	}
	switch t.job.Status {
	case KaggleJobCompleted:
		return o.cleanupDone(ctx, r, res, cleanDeleted, t)
	case KaggleJobUnknown:
		_, err := o.readOp(ctx, read, params)
		if IsKaggleNotFound(err) {
			return o.cleanupDone(ctx, r, res, cleanDeleted, t)
		}
		if n := rem.Attempts[step] + 1; err == nil && n < maxStepAttempts {
			_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": step, "attempt": n, "status": t.job.Status}),
				func(x *Run) { t.save(x); setAttempt(x, step, n) })
			return stepResult{progressed: true}, err
		}
	}
	return o.cleanupDone(ctx, r, res, cleanFailed, t)
}

func (o *Orchestrator) cleanupDone(ctx context.Context, r Run, res, outcome string, t kaggleTurn) (stepResult, error) {
	ev := NewRunEvent("kaggle", map[string]any{"cleanup": res, "result": outcome})
	if outcome == cleanFailed {
		ev = NewRunEvent("warn", map[string]any{"code": "cleanup_failed", "resource": res})
	}
	_, err := o.commit(ctx, r, ev, func(x *Run) { t.save(x); markCleanup(x, res, outcome) })
	return stepResult{progressed: true}, err
}

func (o *Orchestrator) finishRun(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	left := []Leftover{}
	if c := rem.Cleanup[cleanupDataset]; c == cleanKept || c == cleanFailed {
		ref := KaggleRef(rem.Owner, rem.DatasetSlug)
		left = append(left, Leftover{Kind: "dataset", Ref: ref, URL: KaggleDatasetURL(ref)})
	}
	if c := rem.Cleanup[cleanupNotebook]; c == cleanKept || c == cleanFailed {
		ref := KaggleRef(rem.Owner, rem.NotebookSlug)
		left = append(left, Leftover{Kind: "notebook", Ref: ref, URL: KaggleNotebookURL(ref)})
	}
	if err := o.registerLogTail(ctx, r); err != nil {
		return o.localError(ctx, r, err)
	}
	end := rem.EndState
	if end != RunSucceeded && end != RunCancelled {
		end = RunFailed
	}
	now := ts(o.now())
	return o.transition(ctx, r, end, map[string]any{"error_code": r.ErrorCode, "leftovers": len(left)}, func(x *Run) {
		x.Leftovers, x.Remote.Finished = left, now
	})
}

func (o *Orchestrator) registerLogTail(ctx context.Context, r Run) error {
	rel := runRel(r.ID, outSubdir, logTailFileName)
	b, err := readChibiFile(o.dataDir, rel, 2*logTailBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	return o.st.AddArtifact(ctx, Artifact{RunID: r.ID, ProjectID: r.ProjectID, Kind: "log_tail", RelPath: rel,
		Bytes: int64(len(b)), SHA256: hex.EncodeToString(sum[:])})
}
```

- [ ] 8. Verify:

```bash
gofmt -l internal/chibisensei
go build ./...
go vet ./internal/chibisensei/...
go test ./internal/chibisensei/... -count=1
```
  Expected: no `gofmt` output; build and vet exit 0; `ok  aurago/internal/chibisensei` and `ok  aurago/internal/chibisensei/trainer`.

---

#### Task B4.3: Fake-Kaggle acceptance suite

**Files**
- Create: `internal/chibisensei/reconcile_test.go`, `internal/chibisensei/reconcile_restart_test.go`

**Existing symbols modified:** none.

**Steps**

- [ ] 1. Create `reconcile_test.go` with the fixture and helpers, complete:

```go
package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"aurago/internal/chibisensei/trainer"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

type fixtureOpts struct {
	profile string                // qwen35-0p8b (LoRA, assistant) or gemma3-270m (full, classify)
	base    bool                  // the plan asks for the base GGUF
	cfg     func(*OrchestratorConfig)
	outputs func(*fakeRunOutputs) // shape of the kernel outputs
	before  func(*runFixture)     // scripts; runs after SetOutputs and before Submit
}

type runFixture struct {
	t       *testing.T
	st      *Store
	hub     *Hub
	cat     *Catalog
	f       *fakeKaggle
	clk     *fakeClock
	dataDir string
	cfg     OrchestratorConfig
	o       *Orchestrator
	run     Run
	outputs map[string][]byte
}

func newRunFixture(t *testing.T, opt fixtureOpts) *runFixture {
	t.Helper()
	ctx := context.Background()
	fx := &runFixture{t: t, st: newTestStore(t), hub: NewHub(), cat: mustCatalog(t), f: newFakeKaggle(t),
		clk: &fakeClock{now: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)}, dataDir: t.TempDir()}
	fx.cfg = OrchestratorConfig{MachineShape: "NvidiaTeslaT4", DeleteRemoteAfterRun: true, AllowDelete: true,
		RemoteTimeoutMinutes: 720, AuraGoVersion: "test"}
	if opt.cfg != nil {
		opt.cfg(&fx.cfg)
	}
	prof, ok := fx.cat.Get(opt.profile)
	if !ok {
		t.Fatalf("profile %s", opt.profile)
	}
	in := ProjectInput{Name: "Run", Recipe: RecipeAssistant, GoalText: "Answer shop questions.", ProfileID: prof.ID}
	if prof.ID == "gemma3-270m" {
		in.Recipe, in.TaskKind, in.Labels = RecipeSpecialist, "classify", []string{"pos", "neg"}
	}
	p, err := fx.st.CreateProject(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]RowInput, 80)
	for i := range rows {
		answer := fmt.Sprintf("The shop opens at %d o'clock on day %d.", 8+i%4, i)
		if in.TaskKind == "classify" {
			answer = in.Labels[i%2]
		}
		rows[i] = chatInput(fmt.Sprintf("Question %d: when does the shop open?", i), answer)
	}
	if _, err := fx.st.AddRows(ctx, p.ID, rows, AddOptions{Source: "manual", BatchKind: "add", MaxRows: 50000}); err != nil {
		t.Fatal(err)
	}
	srows, proj, err := fx.st.LoadSplitRows(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	split := ComputeSplit(srows, proj.SplitSeed, proj.TaskKind == "classify")
	plan, err := ComputePlan(PlanInput{Project: proj, Profile: prof, Rows: srows, Split: split,
		RemoteTimeoutMinutes: 720, NeedBaseGGUF: opt.base})
	if err != nil {
		t.Fatal(err)
	}
	fx.newOrchestrator()
	runID := uuid.NewString()
	out := fakeRunOutputs{RunID: runID, ProfileID: prof.ID, ProfileSHA: fx.cat.SHA(prof.ID), Arch: prof.GGUF.ExpectedArch,
		Adapter: plan.Method == "lora", BaseGGUF: opt.base}
	if opt.outputs != nil {
		opt.outputs(&out)
	}
	fx.outputs = fakeOutputs(out)
	fx.f.SetOutputs(fx.outputs)
	if opt.before != nil {
		opt.before(fx)
	}
	fx.run, err = fx.o.Submit(ctx, SubmitInput{RunID: runID, Project: proj, Profile: prof, ProfileSHA: fx.cat.SHA(prof.ID),
		Plan: plan, Owner: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	return fx
}

func (fx *runFixture) newOrchestrator() {
	fx.o = NewOrchestrator(fx.st, fx.hub, fx.cat, fx.f, fx.dataDir, func() OrchestratorConfig { return fx.cfg })
	fx.o.now = fx.clk.Now
}

// restart drops the orchestrator (it keeps no goroutine and no run state) and restarts the Kaggle service, which
// turns queued and running jobs into unknown as AuraGo's restart does. Store and data directory stay.
func (fx *runFixture) restart() {
	fx.o = nil
	fx.f.RestartService()
	fx.newOrchestrator()
}

func (fx *runFixture) get() Run {
	fx.t.Helper()
	r, err := fx.st.GetRun(context.Background(), fx.run.ID)
	if err != nil {
		fx.t.Fatal(err)
	}
	return r
}

// drive steps the run, advancing the clock 61 s per step, until stop holds or the run is terminal.
func (fx *runFixture) drive(stop func(Run) bool) Run {
	fx.t.Helper()
	for i := 0; i < 400; i++ {
		r := fx.get()
		if (stop != nil && stop(r)) || RunTerminal(r.State) {
			return r
		}
		if _, err := fx.o.Step(context.Background(), r.ID); err != nil {
			fx.t.Fatalf("step in %s: %v", r.State, err)
		}
		fx.clk.Advance(61 * time.Second)
	}
	fx.t.Fatalf("run did not settle in 400 steps (state %s)", fx.get().State)
	return Run{}
}

func (fx *runFixture) untilState(s string) func(Run) bool { return func(r Run) bool { return r.State == s } }

// happyMutations: one admission per staged file, dataset, notebook, kernel output (status + manifest + the other
// manifest files) and delete.
func (fx *runFixture) happyMutations(r Run) map[string]int {
	return map[string]int{"upload_dataset_file": len(r.Remote.Staged), "create_dataset": 1, "save_kernel": 1,
		"download_kernel_output": len(fx.outputs), "delete_dataset": 1, "delete_kernel": 1}
}

// assertNoReplays: no Execute of a mutation hit a known ID; strict also requires every admission to be the receipt
// the run holds for its step (no orphan admission).
func (fx *runFixture) assertNoReplays(r Run, strict bool) {
	t := fx.t
	t.Helper()
	for _, op := range []string{"upload_dataset_file", "create_dataset", "save_kernel", "download_kernel_output", "delete_dataset", "delete_kernel"} {
		if n := len(fx.f.Replays(op)); n != 0 {
			t.Errorf("%s replayed %d times", op, n)
		}
	}
	if !strict {
		return
	}
	tracked := map[string]bool{}
	for _, rec := range r.Remote.Jobs {
		tracked[rec.JobID] = rec.JobID != ""
	}
	for _, c := range fx.f.Mutations("") {
		if !tracked[c.ID] {
			t.Errorf("%s admission %s is not a receipt of the run", c.Op, c.ID)
		}
	}
}

func (fx *runFixture) artifactKinds(r Run) []string {
	fx.t.Helper()
	arts, err := fx.st.ListArtifacts(context.Background(), r.ID)
	if err != nil {
		fx.t.Fatal(err)
	}
	kinds := []string{}
	for _, a := range arts {
		kinds = append(kinds, a.Kind)
		f, err := OpenArtifact(fx.dataDir, a)
		if err != nil {
			fx.t.Errorf("artifact %s: %v", a.Kind, err)
			continue
		}
		f.Close()
	}
	sort.Strings(kinds)
	return kinds
}

func (fx *runFixture) assertSucceeded(r Run, kinds []string) {
	t := fx.t
	t.Helper()
	if r.State != RunSucceeded || r.ErrorCode != "" || len(r.Leftovers) != 0 || r.Remote.Finished == "" || r.Remote.GGUF == nil {
		t.Fatalf("run = %s %s %q leftovers %v", r.State, r.ErrorCode, r.Error, r.Leftovers)
	}
	if got := fx.artifactKinds(r); !reflect.DeepEqual(got, kinds) {
		t.Errorf("artifact kinds = %v, want %v", got, kinds)
	}
	for _, n := range fx.f.ProjectFiles() {
		if strings.HasPrefix(n, "r"+Run12(r.ID)+"-") {
			t.Errorf("staged file %s kept", n)
		}
	}
	if _, ok := fx.f.Dataset(KaggleRef("tester", r.Remote.DatasetSlug)); ok {
		t.Error("dataset not deleted")
	}
	if _, _, ok := fx.f.Kernel(KaggleRef("tester", r.Remote.NotebookSlug)); ok {
		t.Error("notebook not deleted")
	}
}

func countEvents(t *testing.T, st *Store, runID, kind string) int {
	t.Helper()
	evs, err := st.ListRunEvents(context.Background(), runID, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range evs {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// collectHub counts hub events by name until the returned func is called.
func collectHub(t *testing.T, h *Hub, projectID string) func() map[string]int {
	ch, stop, err := h.Subscribe(projectID)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	seen := map[string]int{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range ch {
			mu.Lock()
			seen[e.Name]++
			mu.Unlock()
		}
	}()
	return func() map[string]int { stop(); <-done; mu.Lock(); defer mu.Unlock(); return seen }
}
```

- [ ] 2. `TestOrchestratorHappyPaths` in `reconcile_test.go`, complete:

```go
func TestOrchestratorHappyPaths(t *testing.T) {
	lock, err := trainer.LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, profile, method string
		base                  bool
		kinds                 []string
	}{
		{"lora", "qwen35-0p8b", "lora", true, []string{"adapter", "adapter_config", "eval_samples", "gguf_base", "gguf_tuned", "metrics", "status"}},
		{"full", "gemma3-270m", "full", false, []string{"eval_samples", "gguf_tuned", "metrics", "status"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			fx := newRunFixture(t, fixtureOpts{profile: c.profile, base: c.base})
			r := fx.drive(nil)
			fx.assertSucceeded(r, c.kinds)
			assertMutations(t, fx.f, fx.happyMutations(r))
			fx.assertNoReplays(r, true)
			if r.Remote.Method != c.method {
				t.Errorf("method = %s", r.Remote.Method)
			}
			if got := fx.f.JobsOf("upload_dataset_file")[0].ID; got != KaggleRequestID(r.ID, StepUploadPrefix+"train", 0) {
				t.Errorf("upload id %s is not the deterministic request id", got)
			}
			prof, _ := fx.cat.Get(c.profile)
			if r.Remote.GGUF.Architecture != prof.GGUF.ExpectedArch || r.Remote.Result == nil || r.Remote.Result.GPU != "Tesla T4" {
				t.Errorf("gguf %+v result %+v", r.Remote.GGUF, r.Remote.Result)
			}
			if pts, _ := fx.st.ListMetrics(ctx, r.ID, 0); len(pts) != 4 {
				t.Errorf("metrics = %d, want the 4 points of chibi-metrics.jsonl", len(pts))
			}
			raw, found, _ := fx.st.GetReport(ctx, r.ID, "auto")
			if !found || !strings.Contains(string(raw), `"run_id":"`+r.ID+`"`) {
				t.Errorf("report = %s", raw)
			}
			if c.name == "full" && !strings.Contains(string(raw), `"label_accuracy":{`) {
				t.Errorf("classify report has no label accuracy: %s", raw)
			}
			if c.base {
				b, ok, err := fx.st.GetBaseGGUF(ctx, prof.ID, prof.Base.Revision, BaseGGUFConverter(lock.LlamaCppTag))
				if err != nil || !ok || !strings.HasPrefix(b.RelPath, "base/"+prof.ID+"-") {
					t.Errorf("base gguf row = %+v %v %v", b, ok, err)
				}
			}
			if countEvents(t, fx.st, r.ID, "state") < 11 {
				t.Error("missing state events")
			}
		})
	}
}
```

- [ ] 3. `TestOrchestratorOutcomes` in `reconcile_test.go`, a table on one harness. Full harness and the first row:

```go
func TestOrchestratorOutcomes(t *testing.T) {
	cases := []struct {
		name  string
		opts  fixtureOpts
		state string
		code  string
		adj   func(m map[string]int) // changes of the happy-path admission counts
		check func(t *testing.T, fx *runFixture, r Run)
	}{
		{name: "upload unknown gets a new attempt",
			opts:  fixtureOpts{before: func(fx *runFixture) { fx.f.Script("upload_dataset_file", fakeOutcome{Status: KaggleJobUnknown}) }},
			state: RunSucceeded,
			adj:   func(m map[string]int) { m["upload_dataset_file"]++ },
			check: func(t *testing.T, fx *runFixture, r Run) {
				jobs := fx.f.JobsOf("upload_dataset_file")
				if jobs[0].Status != KaggleJobUnknown || jobs[0].ID != KaggleRequestID(r.ID, "upload_train", 0) ||
					jobs[1].ID != KaggleRequestID(r.ID, "upload_train", 1) {
					t.Errorf("upload jobs = %+v", jobs[:2])
				}
			}},
		// rows below
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.opts.profile == "" {
				c.opts.profile = "gemma3-270m"
			}
			fx := newRunFixture(t, c.opts)
			r := fx.drive(nil)
			if r.State != c.state || r.ErrorCode != c.code {
				t.Fatalf("run = %s/%s (%q), want %s/%s", r.State, r.ErrorCode, r.Error, c.state, c.code)
			}
			want := fx.happyMutations(r)
			if c.adj != nil {
				c.adj(want)
			}
			for op, n := range want {
				if n == 0 {
					delete(want, op)
				}
			}
			assertMutations(t, fx.f, want)
			fx.assertNoReplays(r, false)
			if c.check != nil {
				c.check(t, fx, r)
			}
		})
	}
}
```

  The remaining rows, each written exactly like the first. The profile is `gemma3-270m` unless named. "→ 0" sets the count to 0.

  | Name | Options | State / code | Count changes | Check |
  |---|---|---|---|---|
  | upload unknown three times | `Script("upload_dataset_file", unknown, unknown, unknown)` | failed / `upload_failed` | all ops → 0, then upload → 3 | no `r<run12>-` file in `ProjectFiles()`; `Leftovers` empty |
  | create unknown, dataset exists | `Script("create_dataset", fakeOutcome{Status: unknown, Applied: true})` | succeeded | none | `len(ExecCalls("get_dataset")) >= 1` |
  | create unknown, dataset absent | `Script("create_dataset", fakeOutcome{Status: unknown})` | succeeded | create → 2 | none |
  | submit unknown, notebook runs | `Script("save_kernel", fakeOutcome{Status: unknown, Applied: true})` | succeeded | none | `r.Remote.Untracked`, `r.Remote.KernelVersion == 1`, `len(ExecCalls("get_kernel_session_status")) >= 2` |
  | submit unknown, notebook absent | `Script("save_kernel", fakeOutcome{Status: unknown})` | failed / `kaggle_submit_unknown` | download → 0, delete_kernel → 0 | `Leftovers` empty |
  | download unknown | `Script("download_kernel_output", fakeOutcome{Status: unknown})` | succeeded | download +1 | none |
  | deadline | `Script("save_kernel", fakeOutcome{Remote: []string{"RUNNING"}})` | failed / `remote_timeout` | download → 0 | `strings.Contains(r.Error, KaggleNotebookURL(KaggleRef("tester", r.Remote.NotebookSlug)))` |
  | checksum mismatch | `outputs: o.WrongSHA = OutModelGGUF` | failed / `checksum_mismatch` | none | no `gguf_tuned` in `artifactKinds` |
  | trainer failed | `outputs: o.Failed, o.Code, o.Message = true, "oom", "CUDA out of memory"` | failed / `oom` | none | `r.Error` contains `CUDA out of memory`; 4 metrics; kinds exactly `[metrics status]`; `GetReport` not found |
  | status missing | `before: delete(fx.outputs, OutStatus); fx.f.SetOutputs(fx.outputs); fx.f.ScriptLogs(fakeLog{Body: fakeKernelLogText("Traceback (most recent call last)", "RuntimeError: boom")})` | failed / `status_missing` | download → 3 | `ReadLogTail` contains `RuntimeError: boom`; kinds `[log_tail]` |
  | AllowDelete off | `cfg: c.AllowDelete = false` | succeeded | delete_dataset → 0, delete_kernel → 0 | two leftovers: `{dataset, tester/<ds>, KaggleDatasetURL}` and `{notebook, tester/<nb>, KaggleNotebookURL}` |
  | storage cap | `cfg: c.StorageMaxBytes = 1` | failed / `storage_full` | download → 0 | `artifactKinds` empty |
  | architecture mismatch | `outputs: o.Arch = "llama"` | failed / `gguf_arch_mismatch` | none | `r.Error` contains `"llama"` |
  | compute slot busy | `before: fx.f.AddForeignComputeJob(KaggleJobRemoteRunning)` | failed / `kaggle_slot_busy` | download → 0, delete_kernel → 0 | `Leftovers` empty |

- [ ] 4. Cancel, live log, blocked and loop tests in `reconcile_test.go`.

  - **`TestOrchestratorCancelBeforeSubmit`**, two subtests with gemma.
    - Subtest A: stop with `untilState(RunWaitingDataset)`.
      - `got, err := fx.o.Cancel(ctx, id)`.
      - Assert `err == nil`, `got.CancelRequested`, `got.State == RunWaitingDataset`, `got.View().Cancel != nil`, `got.View().Cancel.StopURL == ""`.
      - `r := fx.drive(nil)`: `cancelled` / `cancelled_by_user`, `Leftovers` empty.
      - `assertMutations` with `{"upload_dataset_file": len(Staged), "create_dataset": 1, "delete_dataset": 1}`.
      - `fx.f.Dataset(...)` is gone, and no `r<run12>-` project file is left.
    - Subtest B: stop when the `upload_train` receipt has a job ID (the upload is still queued).
      - Cancel and drive: `cancelled`.
      - `assertMutations({"upload_dataset_file": 1})`.
      - The fake's upload job is `completed` (cleaning waited for it before it removed the staged files).
  - **`TestOrchestratorCancelAfterSubmit`**, gemma with `Script("save_kernel", fakeOutcome{Remote: []string{"RUNNING", "RUNNING", "RUNNING", "COMPLETE"}})`.
    - Drive to `running`, then `got := Cancel`.
      - Assert `got.CancelRequested`, `got.View().Cancel.StopURL == KaggleNotebookURL(KaggleRef("tester", got.Remote.NotebookSlug))`, `!got.View().Cancel.RemoteStopped`, `got.View().Cancel.TimeLimitMinutes == got.Remote.SessionTimeoutMinutes`.
    - One `Step`, then `r := fx.get()`.
      - `r.State == RunRunning`, `r.Remote.CancelTried`, `!r.Remote.CancelRemoteStopped`, `r.Remote.CancelError` contains `session ID`.
    - `r = fx.drive(nil)`.
      - `cancelled` / `cancelled_by_user`.
      - `len(fx.f.CallsTo("Cancel")) == 1`.
      - `assertMutations({"upload_dataset_file": len(Staged), "create_dataset": 1, "save_kernel": 1, "delete_dataset": 1, "delete_kernel": 1})`, so there are no downloads.
  - **`TestOrchestratorLiveLog`**, two subtests with gemma.
    - Subtest "available":
      - Scripts: `Remote: RUNNING, RUNNING, COMPLETE` and `ScriptLogs(fakeLog{Body: fakeKernelLogJSON(fakeTrainLog(20, 40)...)})`. Start `collect := collectHub(...)` before driving.
      - Drive until `r.Remote.LastLogRead != ""`. Assert:
        - state `running`, `Live == LiveYes`, `LastLogSeq == 3`;
        - `Stage` is `{Name: train, State: start, Step: 20, MaxSteps: 40, Epoch: 0.5}`;
        - `ListMetrics` returns 2 points;
        - `countEvents(..., "stage") == 1`;
        - `ReadLogTail` contains `@@chibi`.
      - Then `seen := collect()`: `seen["metric"] >= 1`, `seen["stage"] >= 1`, `seen["run"] >= 1`.
      - Drive to the end: `succeeded`, `Live == LiveYes`, `ListMetrics` returns 4 (replaced by the file), `log_tail` is among the artifact kinds.
    - Subtest "unavailable":
      - Scripts: `Remote: RUNNING ×3, COMPLETE` and `ScriptLogs(fakeLog{Err: errors.New("Kaggle returned an unexpected notebook log response")}, fakeLog{})`.
      - Drive until `EmptyLogReads == 2`: `Live == LiveUnknown`.
      - Drive until `EmptyLogReads >= 3`: state `running`, `Live == LiveNo`.
      - Drive to the end: `succeeded`, `Live == LiveNo`, 4 metrics.
  - **`TestOrchestratorWaitsWhileBlocked`**, gemma.
    - Drive to `uploading`. `fx.f.SetAccount("tester", fmt.Errorf("grant check: %w", ErrDesktopReadOnly))`.
    - Three `Step`s with `fx.clk.Advance(time.Minute)`. Assert:
      - state `uploading`, `BlockedCode == "desktop_readonly"`, `BlockedSince != ""`;
      - `countEvents(..., "warn") == 1`;
      - `len(fx.f.Mutations("")) == 0`.
    - `SetAccount("tester", nil)`, then `fx.drive(nil)`: `succeeded`, `BlockedCode == ""`.
    - Second subtest:
      - Same block. One `Step`, then `fx.clk.Advance(7 * time.Hour)` and one more `Step`: the state is `cleaning` with `ErrorCode == "desktop_readonly"`.
      - `drive(nil)` ends `failed`.
  - **`TestOrchestratorStartLoop`**, gemma.
    - Drive to `uploading`, then `fx.restart()`. Set the intervals (every interval 1 ms except `logRead: time.Hour` and `idle: 20ms`):
      `fx.o.iv = intervals{localJob: time.Millisecond, running: time.Millisecond, dataset: time.Millisecond, logRead: time.Hour, untrackedPoll: time.Millisecond, blocked: time.Millisecond, idle: 20 * time.Millisecond, errorWait: time.Millisecond}`
    - `fx.o.Start(context.Background())`. Poll `fx.get()` every 10 ms for up to 20 s until terminal, then `fx.o.Close()`.
    - Assert `succeeded` and `assertMutations(fx.happyMutations(r))`.
    - Second fixture:
      - `ctx, cancel := context.WithCancel(...)`; `cancel()`; `Start(ctx)`; `Close()`.
      - The run is still `queued` with an unchanged `Updated`.

- [ ] 5. Create `reconcile_restart_test.go`, complete:

```go
package chibisensei

import "testing"

var restartArtifactKinds = []string{"adapter", "adapter_config", "eval_samples", "gguf_base", "gguf_tuned", "metrics", "status"}

// At the entry of every non-terminal state a new Orchestrator (and a restarted Kaggle service) continues on the same
// store and data directory to succeeded, with exactly the admissions of an uninterrupted run and no replay.
func TestOrchestratorRestartAtEveryState(t *testing.T) {
	for _, state := range []string{RunQueued, RunPackaging, RunUploading, RunCreatingDataset, RunWaitingDataset,
		RunStarting, RunRunning, RunDownloading, RunVerifying, RunCleaning} {
		t.Run(state, func(t *testing.T) {
			fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
			if r := fx.drive(fx.untilState(state)); r.State != state {
				t.Fatalf("run reached %s before %s", r.State, state)
			}
			fx.restart()
			r := fx.drive(nil)
			fx.assertSucceeded(r, restartArtifactKinds)
			assertMutations(t, fx.f, fx.happyMutations(r))
			fx.assertNoReplays(r, true)
		})
	}
}

// A restart while a Kaggle job of the run is queued turns it unknown: the step resolves it (a read where the spec
// says so) and admits exactly one new attempt with a new ID; nothing is replayed. remote_running survives.
func TestOrchestratorRestartMidFlight(t *testing.T) {
	admitted := func(step string) func(Run) bool {
		return func(r Run) bool { rec := r.Remote.Step(step); return rec != nil && rec.JobID != "" }
	}
	cases := []struct {
		name, step, op string
		stop           func(Run) bool
	}{
		{"upload queued", StepUploadPrefix + "train", "upload_dataset_file", admitted(StepUploadPrefix + "train")},
		{"create_dataset queued", StepCreateDataset, "create_dataset", admitted(StepCreateDataset)},
		{"notebook running", StepSaveKernel, "", func(r Run) bool { return r.State == RunRunning && r.Remote.RemoteStatus == "RUNNING" }},
		{"status download queued", StepDownloadPrefix + OutStatus, "download_kernel_output", admitted(StepDownloadPrefix + OutStatus)},
		{"delete_dataset queued", StepDeleteDataset, "delete_dataset", admitted(StepDeleteDataset)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
			before := fx.drive(c.stop)
			first := before.Remote.Step(c.step).RequestID
			fx.restart()
			if c.op != "" {
				if j, ok := fx.f.PeekJob(first); !ok || j.Status != KaggleJobUnknown {
					t.Fatalf("job %s after restart = %+v", first, j)
				}
			}
			r := fx.drive(nil)
			fx.assertSucceeded(r, restartArtifactKinds)
			want := fx.happyMutations(r)
			if c.op != "" {
				want[c.op]++
				if got := r.Remote.Step(c.step).RequestID; got != KaggleRequestID(r.ID, c.step, 1) {
					t.Errorf("new attempt id = %s, want attempt 1", got)
				}
			}
			assertMutations(t, fx.f, want)
			fx.assertNoReplays(r, false)
		})
	}
}
```

- [ ] 6. Verify:

```bash
gofmt -l internal/chibisensei
go build ./...
go test ./internal/chibisensei/ -count=1 -run '(?i)(Orchestrator|ReadGGUFInfo|RunFilesLayout|FakeKaggle)' -v
go test ./internal/chibisensei/... -count=1
```
  Expected:
  - `gofmt` prints nothing and the build exits 0.
  - `--- PASS` for `TestOrchestratorHappyPaths` (2 subtests), `TestOrchestratorOutcomes` (15), `TestOrchestratorCancelBeforeSubmit` (2), `TestOrchestratorCancelAfterSubmit`, `TestOrchestratorLiveLog` (2), `TestOrchestratorWaitsWhileBlocked` (2), `TestOrchestratorStartLoop`, `TestOrchestratorRestartAtEveryState` (10), `TestOrchestratorRestartMidFlight` (5), `TestReadGGUFInfo`, `TestRunFilesLayout`, and B3's `TestFakeKaggle*`.
  - Both packages end with `ok`.

- [ ] 7. DOX hand-off for B5, which owns `AGENTS.md`. Put these facts in the implementer report for `internal/chibisensei/AGENTS.md`:
  - **Ownership:** `reconcile*.go` (orchestrator), `artifacts.go` (storage layout, verified copies, `base_gguf`), `gguf.go`, `reconcile_test.go`, `reconcile_restart_test.go`.
  - **Local Contracts:**
    - One goroutine owns the active run; `Step` is the unit of work: at most one Kaggle mutation and one transition, each transition one `UpdateRun` plus hub events after the commit.
    - Request IDs are `KaggleRequestID(run, step, attempt)`. `unknown` is never replayed: uploads and downloads take a new attempt, `create_dataset` is decided by `get_dataset`, `save_kernel` by `get_kernel_session_status` (answers → untracked; 404 → `kaggle_submit_unknown`). At most 3 attempts.
    - A driver refusal makes the run wait with one `warn` per code. It fails after 6 h before submission or while downloading. Cleaning never waits; it records leftovers.
    - The deadline is remote start + session + 20 min; one final read decides.
    - Every run passes `cleaning`. A cancel before submission stops locally; after submission it tries one remote cancel and keeps tracking.
    - Downloads are checked against `max_transfer_mb`, the storage cap and free disk before transfer. Copies go through `os.Root` with the size and sha256 from the manifest. Names are generated by AuraGo.
    - The log is read in full (16 MiB). Only the scrubbed last 64 KiB is stored as `out/log-tail.txt`.
    - `ReadGGUFInfo` reads only the header with fixed caps.
    - Base GGUFs live in `base/` with a `base_gguf` row and are never removed by `RemoveRunFiles`.
  - **Verification:** `go test ./internal/chibisensei/ -count=1 -run '(?i)Orchestrator'`, which includes the restart table.
  - No other guide changes in this task. The root `AGENTS.md` and `internal/kaggle/AGENTS.md` describe no behaviour this part changes.

---

## Chibi Sensei P2 backend plan, part 6 (B5)

Part B5 is the last backend batch. It covers the HTTP layer of training, the in-process Kaggle driver adapter, the runtime wiring of the orchestrator, preflight and admin enabling, the run routes, the changes to existing handlers, the server tests and the docs/DOX. It builds on B1 (`ReadKernelLog`, `RemoveFile`, machine shapes), B2 (plan fields, snapshot, parsers), B3 (`runs.go`, `runstore.go`, `consents.go`, `plantoken.go`, `preflight.go`, `quota.go`, `report.go`, `driver.go`, all present in the worktree) and B4 (`Orchestrator`, `artifacts.go`, `gguf.go`, from `plan-b4.md`). Where code exists, it wins over plan text.

Conventions for every task:
- Work only in the worktree `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\trusting-roentgen-94e148`. Scratch files go to `C:\disposable\chibi-sensei\`.
- Run Go commands in Git Bash from the worktree root with `-count=1`. If Go reports `no such tool "vet"`, prefix the command with `GOTOOLCHAIN=go1.27.4`. Do not set `GOTMPDIR` or `GOCACHE`.
- Write real UTF-8, never `\u` escapes.
- Do not commit, stage, stash or switch branches. Do not run GitNexus.
- `.gitignore` ignores `AGENTS.md`, but the guides are tracked. The coordinator stages them (`git add -f`); the implementer only edits them.

### Contract changes

1. **`ProfileSummary` gains fields; B5 edits `internal/chibisensei/profiles.go`.** A32 assigns `methods` and `max_seq_cap` to B5. The contract's `license.version` and `license.accepted` go in the same place: `License.Version` is set by `Summary()`, and `License.Accepted` is a field that `Summary()` leaves false and `GET /meta` fills from the consents table. Reason: `/meta` serializes `ProfileSummary` directly. This is the only B5 edit inside `internal/chibisensei`.
2. **A35: the artifact file route ships (yes).**
   - `GET /runs/{id}/artifacts/{aid}/file` follows the Kaggle project-file download (`kaggle_handlers.go:343-362`): `http.ServeContent` (Range and HEAD work) with `application/octet-stream`, `Content-Disposition: attachment; filename="chibi-<run12>-<name>"`, `nosniff`, a sandbox CSP and `private, no-store`.
   - `internal/httpstream` renews deadlines only for SSE and audio/video. So this response would keep the main listener's absolute write timeout (`agentHTTPWriteTimeout` = 40 min, `server_internal_http.go:20,72`).
   - The handler therefore sets one finite deadline through `http.ResponseController`: 10 min plus the file size at 256 KiB/s, at most 4 h (a 1.3 GB GGUF gets about 95 min). Video Studio's upload window does the same (`video_studio_service.go:288-316`). It is bounded, so the httpstream rule ("never an unlimited timeout") holds.
   - A slower client is cut off, and a browser can resume through Range.
3. **New POST routes check same-origin themselves.** These are `preflight/enable`, `consents`, `runs/prepare`, `runs`, `runs/{id}/cancel` and `runs/{id}/delete`. For a caller without a bearer token, they require a matching `Origin` header (`checkCSRFOriginWithPolicy(r, true)`, as `/api/kaggle/credentials` does) and answer 403 `csrf_check_failed` otherwise. The global auth guard still applies. Reason: these routes change config, spend GPU quota or send data. The P1 routes rely on the global guard, which lets header-less requests through while login is off. The UI always sends `Origin` on POST; `csrf_check_failed` is not in the UI's `ERROR_CODES`, so the UI shows its generic text.
4. **How preflight refusals map to codes.** `prepare` and `start` answer 409 with `missing` (the IDs of the failing block items):
   - `run_active` when the `run_active` item fails;
   - `license_consent_required` when `license_consent` is the only failing item;
   - `preflight_failed` otherwise.

   `Submit` returning `ErrRunActive` (a race) gives 409 `run_active` through `chibiDomainError`.
5. **No route answers 503 `kaggle_unavailable` in P2.** An unreachable Kaggle shows up as preflight items (`kaggle_connection`, `kaggle_quota`, `kaggle_slot`) and as the run failure code. The UI keeps the code.
6. **Details of the enable route.**
   - An unknown flag, or an empty list, answers 422 `validation_failed` with one issue `{field:"flags", message_key:"chibi_sensei.preflight.unknown_flag"}`.
   - A save error answers 500 `config_save_failed`.
   - The route is a desktop write: a read-only desktop answers 403 `desktop_readonly` first.
   - `applied` echoes the trimmed, deduplicated list.
7. **Preflight cache.** The Kaggle-side reads (connection test, quota, compute jobs) are cached per runtime for 20 s.
   - The cache key is a sha256 over Kaggle enabled, read-only, auth mode, username, token, legacy key, network access and write access. So a credential change in the Vault misses the cache.
   - The enable route clears the cache. `prepare` and `start` always read fresh.
   - `checked_at` is the time of the Kaggle reads, or the evaluation time when no read ran.
8. **`license.accepted` is true for a profile that needs no consent**, in both `GET /meta` and `prepare`.
9. **Start order.**
   1. Consume the token.
   2. Recompute plan and preflight with fresh reads.
   3. If the connection failed: refuse with the preflight mapping. The hash needs the account.
   4. If the hash differs: `plan_expired`.
   5. If the preflight is not ready: refuse with the preflight mapping.
   6. Record the license note for a profile without required consent.
   7. `Submit`.

   So a row change gives `plan_expired`, not `preflight_failed`.
10. **Operation classes.**
    - `POST /runs/{id}/delete` is a desktop write, so a read-only desktop refuses it.
    - `POST /runs/{id}/cancel` is `desktopStop`, both in the handler and in `desktopRequestOperation`.
    - `POST /consents` and `POST /preflight/enable` are desktop writes.
11. **Driver adapter gates beyond B4 item 5.**
    - Mutations: `Execute` of a non-read operation, `EnsureProject` and `WriteFile` check the read-only desktop (wrapping `ErrDesktopReadOnly`) and then the grants, before the service runs.
    - `ReadKernelLog` refuses a read-only desktop the same way, because its download grant needs write access.
    - `Account` needs `kaggle.enabled`.
    - Reads, `Job`, `ActiveComputeJobs`, `OpenFile`, `RemoveFile` (cleanup of Chibi's own staged files, A3) and `Cancel` (a stop) have no desktop or enabled gate.
12. **When the orchestrator starts.**
    - `openChibiRuntime` starts it only when a non-terminal run exists.
    - `start` and a `cancel` of a non-terminal run start it otherwise; `Start` is idempotent.
    - When `data_dir` changes, the old runtime is retired: orchestrator first, then job runner, then store. An active run of the old directory is paused, not failed. It resumes when that directory is active again, because the next runtime open sees the non-terminal run. Remote GPU time continues until the session ends. A pause longer than the deadline is decided by the first step after resuming (final status read).
13. **Bug found in `plan-b4.md` (for the coordinator; not part of B5).** A34 fixes `state` event data as `{"to":"<state>"}`, but:
    - B4's `transition` writes `{"state":to,"from":…}`;
    - B4's `Cancel` writes `{"state":…,"cancel_requested":true}`.

    The UI skips shapes it does not know, so the timeline would miss every transition except the initial `queued` (written by `NewStateRunEvent`). B4 should add `"to": to` to the `transition` data, for example `data["to"] = to`.
14. **Project delete checks active runs first.** `handleChibiProjectDelete` checks for an active run of the project before `jobs.CancelProject` and answers 409 `run_active`. This keeps P1's rule that a refused delete leaves running jobs alone; `DeleteProject` would refuse only after the jobs were cancelled. After a successful delete it removes the files of every run (`ListRunIDs`, then `RemoveRunFiles`).
15. **Three test hooks on `Server`.**
    - `chibiSenseiKaggleOverride` is read in `openChibiRuntime`.
    - `chibiSenseiNoReconcile` is read in `chibiStartOrchestrator`.
    - `chibiFreeDiskOverride` is read in `chibiFreeDisk`.

### File map

| File | Batch | Responsibility |
|---|---|---|
| `internal/server/chibi_sensei_kaggle.go` (new) | B5.1 | `chibiKaggleDriver` (the `KaggleDriver` adapter), compute-job filter, Kaggle read cache |
| `internal/server/chibi_sensei_service.go` | B5.1, B5.2, B5.3 | runtime fields, open, retire order, orchestrator start and config, free-disk hook (B5.1); `/meta` training fields (B5.2); `chibiDomainError` run codes (B5.3) |
| `internal/server/server.go` | B5.1 | three test-hook fields |
| `internal/server/chibi_sensei_kaggle_stub_test.go` (new) | B5.1 | `chibiStubKaggle`, `chibiUseStubKaggle`, filter and cache tests |
| `internal/chibisensei/profiles.go` | B5.2 | `ProfileSummary` gains `methods`, `max_seq_cap`, `license.version`, `license.accepted` |
| `internal/server/chibi_sensei_preflight.go` (new) | B5.2 | training state, preflight collection, `GET preflight`, `POST preflight/enable`, `POST consents`, same-origin helper |
| `internal/server/chibi_sensei_preflight_test.go` (new) | B5.2 | preflight, cache use, enable, consent, meta |
| `internal/server/chibi_sensei_handlers.go` | B5.2, B5.3 | router entries |
| `internal/server/chibi_sensei_data.go` | B5.2 | plan route `NeedBaseGGUF` from the base GGUF cache |
| `internal/server/chibi_sensei_runs.go` (new) | B5.3 | prepare, start, cancel, delete, refusal mapping, data bytes |
| `internal/server/chibi_sensei_run_views.go` (new) | B5.3 | list, get, metrics, events, artifacts, artifact file, report, log |
| `internal/server/chibi_sensei_jobs.go` | B5.3 | SSE: active run after `rows` |
| `internal/server/chibi_sensei_projects.go` | B5.3 | project delete with runs |
| `internal/server/desktop_operation.go` | B5.3 | run cancel is `desktopStop` |
| `internal/server/desktop_audit_lifetime_test.go`, `chibi_sensei_api_test.go` | B5.3 | classification table, read-only list |
| `internal/server/chibi_sensei_runs_test.go` (new) | B5.3 | run flow, reads, delete, SSE |
| `internal/server/chibi_sensei_app_browser_test.go` | B5.3 | training-step smoke |
| `internal/chibisensei/AGENTS.md`, `internal/server/AGENTS.md`, `internal/kaggle/AGENTS.md`, `AGENTS.md` | B5.4 | DOX |
| `documentation/chibi-sensei.md`, `documentation/configuration.md`, `documentation/manuals/{en,de}/07-configuration.md`, `documentation/manuals/{en,de}/21-api-reference.md` | B5.4 | operator and user docs |

---

#### Task B5.1: Kaggle driver adapter and runtime wiring

**Files**
- Create: `internal/server/chibi_sensei_kaggle.go`, `internal/server/chibi_sensei_kaggle_stub_test.go`
- Modify: `internal/server/chibi_sensei_service.go:32-48` (struct), `:64-88` (`openChibiRuntime`), `:155-169` (`finishChibiRetire`); `internal/server/server.go:397-398` (fields)

**Existing symbols modified:** `chibiSenseiRuntime`, `(*Server).openChibiRuntime`, `(*Server).finishChibiRetire`, `Server` (struct fields)

**Steps**

- [ ] 1. Create `internal/server/chibi_sensei_kaggle.go`:

```go
package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/kaggle"
)

// errChibiKaggleUnavailable marks a driver call that found no Kaggle service (it could not be opened).
var errChibiKaggleUnavailable = errors.New("the Kaggle service is unavailable")

// chibiKaggleEnv holds the Kaggle facts the preflight needs besides the driver calls.
type chibiKaggleEnv struct {
	VaultAvailable    bool
	CredentialsStored bool
	StorageUsedBytes  int64 // bytes in the Kaggle project storage (kaggle.max_storage_mb counts them)
}

// chibiKaggleEnvironment is implemented by the driver adapter and by the server test stub.
type chibiKaggleEnvironment interface {
	kaggleEnv() chibiKaggleEnv
}

// chibiKaggleDriver is chibisensei.KaggleDriver over the server's *kaggle.Service. Every call opens the service through
// ensureKaggleServices (cached there; a failure is errChibiKaggleUnavailable) and passes the current
// s.kaggleSnapshot().Access, so a config or Vault change applies to the next call. Gates:
//   - mutations (Execute of a non-read operation, EnsureProject, WriteFile) refuse a read-only desktop with
//     chibisensei.ErrDesktopReadOnly and check kaggle.CheckGrants before Service.Execute runs, so the host job gate
//     (BeginJob) never admits a failed job row for a refused attempt;
//   - ReadKernelLog refuses a read-only desktop the same way (its download grant needs filesystem write);
//   - Account needs kaggle.enabled;
//   - reads, Job, ActiveComputeJobs, OpenFile, RemoveFile (cleanup of files Chibi staged, A3) and Cancel (a stop) have no
//     desktop or enabled gate; Service.Execute still refuses reads while Kaggle is disabled or the network is denied.
type chibiKaggleDriver struct {
	s     *Server
	store *chibisensei.Store
	mu    sync.Mutex // serializes EnsureProject, so one installation creates one project
}

var (
	_ chibisensei.KaggleDriver = (*chibiKaggleDriver)(nil)
	_ chibiKaggleEnvironment   = (*chibiKaggleDriver)(nil)
)

func newChibiKaggleDriver(s *Server, store *chibisensei.Store) *chibiKaggleDriver {
	return &chibiKaggleDriver{s: s, store: store}
}

func (d *chibiKaggleDriver) service() (*kaggle.Service, error) {
	b, err := d.s.ensureKaggleServices()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errChibiKaggleUnavailable, kaggle.PublicError(err))
	}
	if b == nil || b.KaggleBackend == nil || b.Platform == nil {
		return nil, errChibiKaggleUnavailable
	}
	return b.Platform, nil
}

func (d *chibiKaggleDriver) desktopReadOnly() bool {
	cfg := d.s.ConfigSnapshot()
	return cfg != nil && cfg.VirtualDesktop.ReadOnly
}

func (d *chibiKaggleDriver) Account(ctx context.Context) (string, error) {
	if !d.s.kaggleSnapshot().Config.Enabled {
		return "", errors.New("kaggle account: the Kaggle integration is disabled")
	}
	svc, err := d.service()
	if err != nil {
		return "", err
	}
	res, err := svc.Test(ctx)
	if err != nil {
		return "", fmt.Errorf("kaggle account: %w", err)
	}
	account, _ := res["account"].(string)
	if account == "" {
		return "", errors.New("kaggle account: the connection test named no account")
	}
	return account, nil
}

// admit checks a mutation before Service.Execute sees it (see the type comment).
func (d *chibiKaggleDriver) admit(op kaggle.Operation, req kaggle.Request) error {
	if d.desktopReadOnly() {
		return fmt.Errorf("kaggle %s: %w", op.Name, chibisensei.ErrDesktopReadOnly)
	}
	params, _, err := kaggle.Decode(op, req.Parameters)
	if err != nil {
		return fmt.Errorf("kaggle %s: %w", op.Name, err)
	}
	if op.Name == "save_kernel" && params["kernelExecutionType"] == nil {
		params["kernelExecutionType"] = "QUICK_SAVE" // the service applies the same default
	}
	snap := d.s.kaggleSnapshot()
	if err := kaggle.CheckGrants(snap.Config, snap.Access, op, params); err != nil {
		return fmt.Errorf("kaggle %s refused: %w", op.Name, err)
	}
	return nil
}

func (d *chibiKaggleDriver) Execute(ctx context.Context, req kaggle.Request) (kaggle.Result, error) {
	op, err := kaggle.Lookup(req.Domain, req.Operation)
	if err != nil {
		return kaggle.Result{}, err
	}
	if op.Grant != "read" {
		if err := d.admit(op, req); err != nil {
			return kaggle.Result{}, err
		}
	}
	svc, err := d.service()
	if err != nil {
		return kaggle.Result{}, err
	}
	return svc.Execute(ctx, req, d.s.kaggleSnapshot().Access)
}

func (d *chibiKaggleDriver) Job(ctx context.Context, id string) (kaggle.Job, error) {
	svc, err := d.service()
	if err != nil {
		return kaggle.Job{}, err
	}
	return svc.Store.Get(ctx, id)
}

func (d *chibiKaggleDriver) ActiveComputeJobs(ctx context.Context) ([]kaggle.Job, error) {
	svc, err := d.service()
	if err != nil {
		return nil, err
	}
	jobs, err := svc.Store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Kaggle jobs: %w", err)
	}
	out := []kaggle.Job{}
	for _, j := range jobs {
		if chibisensei.KaggleJobActive(j.Status) && chibiComputeJob(j) {
			out = append(out, j)
		}
	}
	return out, nil
}

// chibiComputeJob mirrors checkComputeSlots (internal/kaggle/service.go:413-435): a benchmark batch, or save_kernel with
// kernelExecutionType SAVE_AND_RUN_ALL in its stored request.
func chibiComputeJob(j kaggle.Job) bool {
	if j.Operation == "batch_schedule_benchmark_task_runs" {
		return true
	}
	if j.Operation != "save_kernel" {
		return false
	}
	var req kaggle.Request
	var p map[string]any
	if json.Unmarshal(j.Request, &req) != nil || json.Unmarshal(req.Parameters, &p) != nil {
		return false
	}
	kind, _ := p["kernelExecutionType"].(string)
	return kind == "SAVE_AND_RUN_ALL"
}

// EnsureProject returns the local Kaggle project of Chibi Sensei. The id lives in the Chibi setting kaggle_project_id;
// a project that no longer resolves (row or directory gone) is created again (A28).
func (d *chibiKaggleDriver) EnsureProject(ctx context.Context) (string, error) {
	if d.desktopReadOnly() {
		return "", fmt.Errorf("kaggle project: %w", chibisensei.ErrDesktopReadOnly)
	}
	snap := d.s.kaggleSnapshot()
	if !snap.Config.Enabled || !snap.Access.Write {
		return "", errors.New("kaggle project: Kaggle is disabled or filesystem writes are not allowed")
	}
	svc, err := d.service()
	if err != nil {
		return "", err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	id, found, err := d.store.GetSetting(ctx, chibisensei.SettingKaggleProjectID)
	if err != nil {
		return "", fmt.Errorf("kaggle project setting: %w", err)
	}
	if found && id != "" {
		ok, err := chibiKaggleProjectExists(ctx, svc, id)
		if err != nil {
			return "", err
		}
		if ok {
			return id, nil
		}
	}
	p, err := svc.CreateProject(ctx, chibisensei.KaggleProjectTitle)
	if err != nil {
		return "", fmt.Errorf("create the Chibi Sensei Kaggle project: %s", kaggle.PublicError(err))
	}
	if err := d.store.SetSetting(ctx, chibisensei.SettingKaggleProjectID, p.ID); err != nil {
		return "", fmt.Errorf("store the Kaggle project id: %w", err)
	}
	return p.ID, nil
}

// chibiKaggleProjectExists reports whether the project row and its directory exist. A store error is returned, so a
// transient failure never creates a second project.
func chibiKaggleProjectExists(ctx context.Context, svc *kaggle.Service, id string) (bool, error) {
	projects, err := svc.Store.Projects(ctx)
	if err != nil {
		return false, fmt.Errorf("list Kaggle projects: %w", err)
	}
	for _, p := range projects {
		if p.ID == id {
			_, ferr := svc.Files(id)
			return ferr == nil, nil
		}
	}
	return false, nil
}

func (d *chibiKaggleDriver) WriteFile(ctx context.Context, project, name string, r io.Reader) (int64, error) {
	if d.desktopReadOnly() {
		return 0, fmt.Errorf("write %s: %w", name, chibisensei.ErrDesktopReadOnly)
	}
	svc, err := d.service()
	if err != nil {
		return 0, err
	}
	return svc.WriteFile(ctx, project, name, r, nil) // nil: the service checks enabled and write access per chunk
}

func (d *chibiKaggleDriver) OpenFile(project, name string) (*os.File, error) {
	svc, err := d.service()
	if err != nil {
		return nil, err
	}
	return svc.OpenFile(project, name)
}

func (d *chibiKaggleDriver) RemoveFile(project, name string) error {
	svc, err := d.service()
	if err != nil {
		return err
	}
	return svc.RemoveFile(project, name)
}

func (d *chibiKaggleDriver) ReadKernelLog(ctx context.Context, owner, slug string, version int, max int64) ([]byte, error) {
	if d.desktopReadOnly() {
		return nil, fmt.Errorf("read the Kaggle log: %w", chibisensei.ErrDesktopReadOnly)
	}
	svc, err := d.service()
	if err != nil {
		return nil, err
	}
	return svc.ReadKernelLog(ctx, d.s.kaggleSnapshot().Access, owner, slug, version, max)
}

func (d *chibiKaggleDriver) Cancel(ctx context.Context, jobID string) error {
	svc, err := d.service()
	if err != nil {
		return err
	}
	return svc.Cancel(ctx, jobID)
}

func (d *chibiKaggleDriver) kaggleEnv() chibiKaggleEnv {
	snap := d.s.kaggleSnapshot()
	env := chibiKaggleEnv{VaultAvailable: d.s.Vault != nil, CredentialsStored: snap.Config.Token != "" || snap.Config.LegacyKey != ""}
	if snap.Config.Enabled { // a disabled integration is never opened just for the preflight
		if svc, err := d.service(); err == nil {
			if used, ok := svc.Status()["storage_bytes"].(int64); ok {
				env.StorageUsedBytes = used
			}
		}
	}
	return env
}

// chibiKaggleReadTTL is how long the preflight reuses the Kaggle-side reads. The Training step loads three preset cards
// and has a refresh button; within the TTL they share one round of calls.
const chibiKaggleReadTTL = 20 * time.Second

// chibiKaggleRead is one round of Kaggle-side reads.
type chibiKaggleRead struct {
	ConnectionOK bool
	Account      string
	QuotaRead    bool
	QuotaKnown   bool
	QuotaHours   float64
	JobsRead     bool
	ComputeJobs  int
	UnknownJobs  int
	At           time.Time
}

// chibiKaggleReads caches one chibiKaggleRead per runtime. The key covers what changes the answers (chibiKaggleReadKey),
// so a credential change misses; the enable route calls invalidate.
type chibiKaggleReads struct {
	mu  sync.Mutex
	key string
	val chibiKaggleRead
}

func (c *chibiKaggleReads) invalidate() {
	c.mu.Lock()
	c.key = ""
	c.mu.Unlock()
}

// get returns the cached reads for key while they are younger than the TTL, unless fresh is set (prepare and start).
// It holds the lock while it reads, so concurrent preflights wait for one round instead of starting more.
func (c *chibiKaggleReads) get(ctx context.Context, key string, fresh bool, now time.Time, drv chibisensei.KaggleDriver) chibiKaggleRead {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !fresh && c.key != "" && c.key == key && now.Sub(c.val.At) < chibiKaggleReadTTL {
		return c.val
	}
	v := chibiReadKaggle(ctx, drv)
	v.At = now
	if ctx.Err() == nil {
		c.key, c.val = key, v
	}
	return v
}

// chibiReadKaggle runs the connection test, then (only when it passed) the quota read, then the compute-job list.
func chibiReadKaggle(ctx context.Context, drv chibisensei.KaggleDriver) chibiKaggleRead {
	var v chibiKaggleRead
	if account, err := drv.Account(ctx); err == nil && account != "" {
		v.ConnectionOK, v.Account = true, account
	}
	if v.ConnectionOK {
		if req, err := chibisensei.NewKaggleRequest("", "get_accelerator_quota_statistics", "", "", nil); err == nil {
			if res, err := drv.Execute(ctx, req); err == nil {
				v.QuotaRead = true
				v.QuotaHours, v.QuotaKnown = chibisensei.ParseGPUQuota(res.Data)
			}
		}
	}
	if jobs, err := drv.ActiveComputeJobs(ctx); err == nil {
		v.JobsRead, v.ComputeJobs = true, len(jobs)
		for _, j := range jobs {
			if j.Status == chibisensei.KaggleJobUnknown {
				v.UnknownJobs++
			}
		}
	}
	return v
}

// chibiKaggleReadKey hashes what changes the Kaggle reads. Secrets only enter the hash, never a log.
func chibiKaggleReadKey(snap kaggle.Snapshot) string {
	h := sha256.New()
	c := snap.Config
	fmt.Fprintf(h, "%t|%t|%s|%s|%t|%t|", c.Enabled, c.ReadOnly, c.AuthMode, c.Username, snap.Access.Network, snap.Access.Write)
	io.WriteString(h, c.Token)
	io.WriteString(h, "|")
	io.WriteString(h, c.LegacyKey)
	return hex.EncodeToString(h.Sum(nil))
}
```

- [ ] 2. In `internal/server/server.go`, add the import `"aurago/internal/chibisensei"` if it is missing, and after line 398 (`chibiSenseiLLMOverride llm.ChatClient`) add:

```go
	// chibiSenseiKaggleOverride replaces the Kaggle driver of the next opened runtime in tests (read in openChibiRuntime).
	chibiSenseiKaggleOverride chibisensei.KaggleDriver
	// chibiSenseiNoReconcile keeps the run orchestrator from starting in tests (read in chibiStartOrchestrator).
	chibiSenseiNoReconcile bool
	// chibiFreeDiskOverride replaces fileutil.FreeDiskBytes in the training preflight in tests (read in chibiFreeDisk).
	chibiFreeDiskOverride func(path string) (int64, error)
```

- [ ] 3. In `chibi_sensei_service.go`:
  - Extend the struct comment: "…the job runner, the delete-preview tokens, the Kaggle driver, the run orchestrator, the plan tokens and the cache of the Kaggle reads."
  - Add the fields after `deletes` (line 38):

```go
	driver      chibisensei.KaggleDriver
	orch        *chibisensei.Orchestrator
	plans       *chibisensei.PlanTokens
	kaggleReads *chibiKaggleReads
```

  - Replace `openChibiRuntime` (lines 63-88) with:

```go
// openChibiRuntime opens the store of dataDir and everything that hangs on it. The run orchestrator starts only when a
// non-terminal run exists; otherwise the first start (or a cancel) starts it.
func (s *Server) openChibiRuntime(dataDir string) (*chibiSenseiRuntime, error) {
	catalog, err := chibisensei.LoadCatalog()
	if err != nil {
		return nil, err
	}
	store, err := chibisensei.OpenStore(filepath.Join(dataDir, config.ChibiSenseiDBFilename))
	if err != nil {
		return nil, err
	}
	hub := chibisensei.NewHub()
	store.SetNotifier(hub.DataChanged)
	jobs := chibisensei.NewRunner(store, hub)
	if s.Logger != nil {
		jobs.SetLogger(s.Logger)
	}
	if err := jobs.Recover(context.Background()); err != nil {
		_ = store.Close()
		return nil, err
	}
	var driver chibisensei.KaggleDriver = newChibiKaggleDriver(s, store)
	if s.chibiSenseiKaggleOverride != nil {
		driver = s.chibiSenseiKaggleOverride
	}
	orch := chibisensei.NewOrchestrator(store, hub, catalog, driver, dataDir, s.chibiOrchestratorConfig)
	if s.Logger != nil {
		orch.SetLogger(s.Logger)
	}
	rt := &chibiSenseiRuntime{
		dataDir: dataDir, store: store, catalog: catalog, hub: hub, jobs: jobs,
		deletes: chibisensei.NewDeleteTokens(nil),
		driver:  driver, orch: orch, plans: chibisensei.NewPlanTokens(nil), kaggleReads: &chibiKaggleReads{},
		retiring: make(chan struct{}), drained: make(chan struct{}),
	}
	active, err := store.ActiveRuns(context.Background())
	switch {
	case err != nil:
		if s.Logger != nil {
			s.Logger.Warn("[ChibiSensei] Active training runs could not be read; the reconciler starts with the next run action", "error", err)
		}
	case len(active) > 0:
		s.chibiStartOrchestrator(rt)
	}
	return rt, nil
}

// chibiStartOrchestrator starts the reconcile loop of rt (idempotent; a closed orchestrator ignores it). The loop lives
// as long as the server, or until the runtime is retired.
func (s *Server) chibiStartOrchestrator(rt *chibiSenseiRuntime) {
	if s.chibiSenseiNoReconcile {
		return
	}
	ctx := s.integrationCtx
	if ctx == nil {
		ctx = context.Background()
	}
	rt.orch.Start(ctx)
}

// chibiOrchestratorConfig feeds the orchestrator from the live configuration at every step.
func (s *Server) chibiOrchestratorConfig() chibisensei.OrchestratorConfig {
	out := chibisensei.OrchestratorConfig{MachineShape: config.DefaultChibiSenseiMachineShape, DeleteRemoteAfterRun: true,
		AuraGoVersion: chibiAuraGoVersion()}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return out
	}
	cs := cfg.ChibiSensei
	_ = config.NormalizeChibiSenseiConfig(&cs) // a range error elsewhere leaves the training values usable
	if cs.Training.MachineShape != "" {
		out.MachineShape = cs.Training.MachineShape
	}
	out.DeleteRemoteAfterRun = cs.Training.DeleteRemoteAfterRun
	out.StorageMaxBytes = int64(cs.Storage.MaxGB) << 30
	out.AllowDelete = cfg.Kaggle.AllowDelete
	out.RemoteTimeoutMinutes = cfg.Kaggle.RemoteTimeoutMinutes
	out.MaxTransferBytes = int64(cfg.Kaggle.MaxTransferMB) << 20
	return out
}

// chibiAuraGoVersion is the build id written into job.json (1 to 64 characters).
func chibiAuraGoVersion() string {
	v := buildinfo.Current().BuildID
	if len(v) > 64 {
		v = v[:64]
	}
	return v
}

// chibiFreeDisk is fileutil.FreeDiskBytes, replaceable in tests.
func (s *Server) chibiFreeDisk(path string) (int64, error) {
	if s.chibiFreeDiskOverride != nil {
		return s.chibiFreeDiskOverride(path)
	}
	return fileutil.FreeDiskBytes(path)
}
```

  Add the imports `"aurago/internal/buildinfo"` and `"aurago/internal/fileutil"`.

  - In `finishChibiRetire` (line 155), change the comment to "…stops the run orchestrator and the job runner of a retired runtime (before its store goes)…" and make `rt.orch.Close()` the first statement, before `rt.jobs.Close()`.
  - Order on retire and shutdown: orchestrator (≤ 15 s; its step context ends at once), then job runner, then store after the last lease. `closeChibiSensei` runs before `closeKaggleServices` (`server.go:960-963`, deferred in reverse order), so the Kaggle service outlives the orchestrator.

- [ ] 4. Create `internal/server/chibi_sensei_kaggle_stub_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/kaggle"
)

var errChibiStubRefused = errors.New("stub: Kaggle mutations are refused in server tests")

// chibiStubKaggle is the in-memory chibisensei.KaggleDriver of the server tests (fakeKaggle lives in a _test.go file of
// internal/chibisensei and cannot be imported). Reads answer from the fields; every mutation is refused, so a run a
// test starts never leaves this process.
type chibiStubKaggle struct {
	mu         sync.Mutex
	account    string
	accountErr error
	quota      json.RawMessage
	jobs       []kaggle.Job
	env        chibiKaggleEnv
	counts     map[string]int
}

var (
	_ chibisensei.KaggleDriver = (*chibiStubKaggle)(nil)
	_ chibiKaggleEnvironment   = (*chibiStubKaggle)(nil)
)

func newChibiStubKaggle() *chibiStubKaggle {
	return &chibiStubKaggle{account: "tester", quota: json.RawMessage(`{"gpuQuota":{"remainingSeconds":108000}}`),
		env: chibiKaggleEnv{VaultAvailable: true, CredentialsStored: true}, counts: map[string]int{}}
}

func (k *chibiStubKaggle) note(name string) { k.mu.Lock(); k.counts[name]++; k.mu.Unlock() }

func (k *chibiStubKaggle) calls(name string) int { k.mu.Lock(); defer k.mu.Unlock(); return k.counts[name] }

func (k *chibiStubKaggle) kaggleEnv() chibiKaggleEnv { k.mu.Lock(); defer k.mu.Unlock(); return k.env }

func (k *chibiStubKaggle) Account(context.Context) (string, error) {
	k.note("account")
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.account, k.accountErr
}

func (k *chibiStubKaggle) Execute(_ context.Context, req kaggle.Request) (kaggle.Result, error) {
	k.note(req.Operation)
	if req.Operation == "get_accelerator_quota_statistics" {
		k.mu.Lock()
		defer k.mu.Unlock()
		return kaggle.Result{Data: append(json.RawMessage(nil), k.quota...)}, nil
	}
	return kaggle.Result{}, errChibiStubRefused
}

func (k *chibiStubKaggle) Job(context.Context, string) (kaggle.Job, error) {
	return kaggle.Job{}, errors.New("stub: unknown job")
}

func (k *chibiStubKaggle) ActiveComputeJobs(context.Context) ([]kaggle.Job, error) {
	k.note("jobs")
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]kaggle.Job(nil), k.jobs...), nil
}

func (k *chibiStubKaggle) EnsureProject(context.Context) (string, error) { return "", errChibiStubRefused }

func (k *chibiStubKaggle) WriteFile(context.Context, string, string, io.Reader) (int64, error) {
	return 0, errChibiStubRefused
}

func (k *chibiStubKaggle) OpenFile(string, string) (*os.File, error) { return nil, os.ErrNotExist }

func (k *chibiStubKaggle) RemoveFile(string, string) error { return nil }

func (k *chibiStubKaggle) ReadKernelLog(context.Context, string, string, int, int64) ([]byte, error) {
	return nil, nil
}

func (k *chibiStubKaggle) Cancel(context.Context, string) error { return errChibiStubRefused }

// chibiUseStubKaggle installs the stub, keeps the orchestrator off and reports 100 GiB of free disk. Call it before the
// first Chibi request: openChibiRuntime and chibiStartOrchestrator read the hooks.
func chibiUseStubKaggle(t *testing.T, s *Server) *chibiStubKaggle {
	t.Helper()
	s.chibiSenseiMu.Lock()
	opened := s.chibiSensei != nil
	s.chibiSenseiMu.Unlock()
	if opened {
		t.Fatal("chibiUseStubKaggle must run before the first Chibi Sensei request")
	}
	k := newChibiStubKaggle()
	s.chibiSenseiKaggleOverride = k
	s.chibiSenseiNoReconcile = true
	s.chibiFreeDiskOverride = func(string) (int64, error) { return 100 << 30, nil }
	return k
}

func TestChibiSenseiComputeJobFilter(t *testing.T) {
	save := func(kind string) json.RawMessage {
		b, err := json.Marshal(kaggle.Request{Domain: "notebooks", Operation: "save_kernel",
			Parameters: json.RawMessage(`{"kernelExecutionType":"` + kind + `"}`)})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for _, c := range []struct {
		name string
		job  kaggle.Job
		want bool
	}{
		{"run all", kaggle.Job{Operation: "save_kernel", Request: save("SAVE_AND_RUN_ALL")}, true},
		{"quick save", kaggle.Job{Operation: "save_kernel", Request: save("QUICK_SAVE")}, false},
		{"benchmark batch", kaggle.Job{Operation: "batch_schedule_benchmark_task_runs"}, true},
		{"upload", kaggle.Job{Operation: "upload_dataset_file"}, false},
		{"broken request", kaggle.Job{Operation: "save_kernel", Request: json.RawMessage(`{`)}, false},
	} {
		if got := chibiComputeJob(c.job); got != c.want {
			t.Errorf("%s: chibiComputeJob = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestChibiSenseiKaggleReadCache(t *testing.T) {
	stub := newChibiStubKaggle()
	var c chibiKaggleReads
	ctx := context.Background()
	t0 := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	steps := []struct {
		key   string
		fresh bool
		at    time.Duration
		calls int
	}{
		{"a", false, 0, 1},                                     // first read
		{"a", false, 10 * time.Second, 1},                      // cached
		{"a", false, chibiKaggleReadTTL + time.Second, 2},      // expired
		{"b", false, chibiKaggleReadTTL + 2*time.Second, 3},    // another key (credentials changed)
		{"b", true, chibiKaggleReadTTL + 3*time.Second, 4},     // fresh (prepare, start)
	}
	for i, st := range steps {
		v := c.get(ctx, st.key, st.fresh, t0.Add(st.at), stub)
		if !v.ConnectionOK || v.Account != "tester" || !v.QuotaKnown || v.QuotaHours != 30 || !v.JobsRead {
			t.Fatalf("step %d: read = %+v", i, v)
		}
		if n := stub.calls("account"); n != st.calls {
			t.Fatalf("step %d: %d connection tests, want %d", i, n, st.calls)
		}
	}
	c.invalidate()
	c.get(ctx, "b", false, t0.Add(chibiKaggleReadTTL+4*time.Second), stub)
	if n := stub.calls("account"); n != 5 {
		t.Fatalf("after invalidate: %d connection tests, want 5", n)
	}
}
```

- [ ] 5. Verify:

```bash
gofmt -l internal/server
go build ./...
go test ./internal/server -count=1 -run '(?i)(ChibiSensei)' -timeout 10m
```

  Expected:
  - no gofmt output and a clean build;
  - `--- PASS: TestChibiSenseiComputeJobFilter`, `--- PASS: TestChibiSenseiKaggleReadCache`;
  - the existing `TestChibiSenseiRuntimeOutlivesDataDirChange` still passes (its orchestrator never started, so `Close` returns at once);
  - `ok aurago/internal/server`.

---

#### Task B5.2: Preflight, admin enabling, consents, meta and plan

**Files**
- Create: `internal/server/chibi_sensei_preflight.go`, `internal/server/chibi_sensei_preflight_test.go`
- Modify:
  - `internal/chibisensei/profiles.go:115-133` (`ProfileSummary`), `:205-213` (`Summary`)
  - `internal/server/chibi_sensei_service.go:431-461` (`handleChibiMeta`)
  - `internal/server/chibi_sensei_data.go:142-144` (`handleChibiPlan`)
  - `internal/server/chibi_sensei_handlers.go:35-70` and `:79-111` (router)

**Existing symbols modified:** `ProfileSummary`, `(*Profile).Summary`, `handleChibiMeta`, `handleChibiPlan`, `handleChibiSensei`, `handleChibiProjectRoute`

**Steps**

- [ ] 1. `internal/chibisensei/profiles.go`. In `ProfileSummary`, after `GGUFEstimatedMB`, add:

```go
	Methods   []string `json:"methods"`     // training methods the profile allows (A32)
	MaxSeqCap int      `json:"max_seq_cap"` // largest max_seq_len an override may pick (A32)
```

  Change the `License` struct to:

```go
	License         struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		URL             string `json:"url"`
		Version         string `json:"version"`
		RequiresConsent bool   `json:"requires_consent"`
		Accepted        bool   `json:"accepted"` // filled by the server from the consents table; Summary leaves it false
	} `json:"license"`
```

  In `Summary()`, add `Methods: append([]string{}, p.Training.Methods...), MaxSeqCap: p.Training.MaxSeqCap` to the literal, and `s.License.Version = p.License.Version` after the license line.

- [ ] 2. Create `internal/server/chibi_sensei_preflight.go`:

```go
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
```

- [ ] 3. `handleChibiMeta` (`chibi_sensei_service.go:431-461`). Replace the profile loop and the JSON body with:

```go
	profiles := []chibisensei.ProfileSummary{}
	for _, p := range acc.rt.catalog.List() {
		sum := p.Summary()
		sum.License.Accepted = !p.License.RequiresConsent
		if p.License.RequiresConsent {
			accepted, err := acc.rt.store.HasConsent(r.Context(), p.ID, p.License.ID, p.License.Version)
			if err != nil {
				chibiDomainError(w, err)
				return
			}
			sum.License.Accepted = accepted
		}
		profiles = append(profiles, sum)
	}
	training := acc.cfg.ChibiSensei.Training
	writeChibiJSON(w, http.StatusOK, map[string]any{
		// readonly, admin, limits, recipes, profiles, generator: unchanged
		"training": map[string]any{"backend": chibisensei.BackendKaggle, "machine_shape": training.MachineShape,
			"delete_remote_after_run": training.DeleteRemoteAfterRun},
		"run_states": chibisensei.RunStates(),
	})
```

  Keep the six existing keys in the literal exactly as they are.

- [ ] 4. `handleChibiPlan` (`chibi_sensei_data.go:142-144`): replace `NeedBaseGGUF: true` with `NeedBaseGGUF: chibiNeedBaseGGUF(ctx, acc.rt.store, prof)`. Leave `handleChibiChecksRun` (line 44) at `true`: it only needs `MaxSeqLen`.

- [ ] 5. Router (`chibi_sensei_handlers.go`):
  - In `handleChibiSensei`, add before `default:` (line 68):

```go
		case len(parts) == 1 && parts[0] == "consents":
			handleChibiConsents(s, w, r)
		case len(parts) == 2 && parts[0] == "preflight" && parts[1] == "enable":
			handleChibiPreflightEnable(s, w, r)
```

  - In `handleChibiProjectRoute`, in the `len(rest) == 1` switch, add `case "preflight": handleChibiPreflight(s, w, r, id)`.

- [ ] 6. Create `internal/server/chibi_sensei_preflight_test.go`. The helpers below are shared with B5.3:

```go
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"aurago/internal/chibisensei"
	"aurago/internal/config"

	"gopkg.in/yaml.v3"
)

// chibiReadyKaggle grants everything a Chibi run needs (contract U3) in the live config.
func chibiReadyKaggle(t *testing.T, s *Server) {
	t.Helper()
	cfg := *s.ConfigSnapshot()
	k := config.DefaultKaggleConfig()
	k.Enabled, k.ReadOnly = true, false
	k.AllowCreate, k.AllowRemoteExecute, k.AllowGPU, k.AllowInternet, k.AllowDownload, k.AllowDelete = true, true, true, true, true, true
	k.AllowedMachineShapes = []string{"NvidiaTeslaT4"}
	k.RemoteTimeoutMinutes = 240
	cfg.Kaggle = k
	cfg.Agent.AllowFilesystemWrite, cfg.Agent.AllowNetworkRequests = true, true
	s.replaceConfigSnapshot(&cfg)
}

// chibiReadyProject creates a qwen35 assistant project with 40 rows and a ready check result.
func chibiReadyProject(t *testing.T, s *Server, token string) string {
	t.Helper()
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant", "profile_id": "qwen35-0p8b"})
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/rows", map[string]any{"rows": chibiPairs(40, true)}), http.StatusCreated, "")
	w := chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/checks/run", nil)
	chibiWant(t, w, http.StatusOK, "")
	if chibiBody(t, w)["ready"] != true {
		t.Fatalf("the dataset is not ready: %s", w.Body.String())
	}
	return pid
}

type chibiPreflightAnswer struct {
	Ready     bool                        `json:"ready"`
	Admin     bool                        `json:"admin"`
	Preset    string                      `json:"preset"`
	CheckedAt string                      `json:"checked_at"`
	Items     []chibisensei.PreflightItem `json:"items"`
	Fix       *struct {
		Flags []string `json:"flags"`
	} `json:"fix"`
}

func chibiGetPreflight(t *testing.T, s *Server, token, pid string) chibiPreflightAnswer {
	t.Helper()
	w := chibiDo(t, s, token, http.MethodGet, "/projects/"+pid+"/preflight?preset=quick", nil)
	chibiWant(t, w, http.StatusOK, "")
	var a chibiPreflightAnswer
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	return a
}

func (a chibiPreflightAnswer) item(t *testing.T, id string) chibisensei.PreflightItem {
	t.Helper()
	for _, it := range a.Items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("preflight has no item %q", id)
	return chibisensei.PreflightItem{}
}
```

  The tests in this file are below. The first is complete; the rest are listed precisely.

```go
func TestChibiSenseiPreflightKaggleDisabled(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	stub := chibiUseStubKaggle(t, s)
	pid := chibiReadyProject(t, s, token)
	pf := chibiGetPreflight(t, s, token, pid)
	if pf.Ready || pf.Preset != "quick" || len(pf.Items) != 22 || pf.CheckedAt == "" {
		t.Fatalf("preflight = %+v", pf)
	}
	if it := pf.item(t, "kaggle_enabled"); it.OK || it.Level != "block" || it.Flag != "kaggle.enabled" {
		t.Fatalf("kaggle_enabled = %+v", it)
	}
	if it := pf.item(t, "dataset_ready"); !it.OK {
		t.Fatalf("dataset_ready = %+v", it)
	}
	if it := pf.item(t, "kaggle_connection"); it.OK || it.Args["skipped"] != true {
		t.Fatalf("kaggle_connection = %+v", it)
	}
	if pf.Fix == nil || !slices.Contains(pf.Fix.Flags, "kaggle.enabled") {
		t.Fatalf("fix = %+v", pf.Fix)
	}
	if n := stub.calls("account"); n != 0 {
		t.Fatalf("the preflight called Kaggle %d times while Kaggle is off", n)
	}
}
```

  - `TestChibiSenseiPreflightReady`:
    1. `chibiAPIFixture`, `chibiUseStubKaggle`, `chibiReadyKaggle`, `chibiReadyProject`.
    2. One preflight: `Ready` true, `Fix` nil, every `block` item `OK`; `kaggle_connection.args.account == "tester"`; `kaggle_quota` is `block`, `OK`, `args.hours == 30`.
    3. Two more preflights, then `stub.calls("account") == 1` (cache).
    4. Switch `ChibiSensei.Enabled` off through `replaceConfigSnapshot`; the preflight route answers 403 `chibi_disabled` (capability gating).
  - `TestChibiSenseiPreflightEnable`:
    1. `s, _, writeToken := chibiAPIFixture(t)`; `adminToken` from `s.TokenManager.Create("chibi admin", []string{"admin", desktopScopeRead, desktopScopeWrite}, nil)` (precedent `kaggle_jobs_test.go:40`).
    2. Same-origin. Set `Auth.Enabled = false` through `replaceConfigSnapshot`, then send without a token:
       - `chibiDo(t, s, "", POST, "/preflight/enable", `{"flags":["kaggle.allow_tpu"]}`, "Origin", "https://evil.example")` gives 403 `csrf_check_failed`;
       - the same with `"Origin", "http://example.com"` gives 422 `validation_failed`: it passed the origin check, and `allow_tpu` is not a preflight flag.
    3. With `writeToken`: 403 `admin_required`.
    4. Write `config.yaml` into `t.TempDir()`:
       ```
       "server:\n  host: 127.0.0.1\ndirectories:\n  data_dir: " + strconv.Quote(dataDir) + "\nvirtual_desktop:\n  enabled: true\nchibi_sensei:\n  enabled: true\nkaggle:\n  enabled: false\n"
       ```
       Take `dataDir` from `s.ConfigSnapshot().Directories.DataDir` and set `cfg.ConfigPath` through `replaceConfigSnapshot`.
    5. With `adminToken`:
       - `{"flags":["kaggle.allow_tpu"]}` gives 422;
       - `{"flags":["kaggle.enabled","kaggle.allow_gpu","kaggle.allowed_machine_shapes","kaggle.allow_gpu"]}` gives 200 with `applied == ["kaggle.enabled","kaggle.allow_gpu","kaggle.allowed_machine_shapes"]`.
    6. Unmarshal the file with `yaml.v3` into `struct{ Kaggle struct{ Enabled bool `yaml:"enabled"`; AllowGPU bool `yaml:"allow_gpu"`; Shapes []string `yaml:"allowed_machine_shapes"` } `yaml:"kaggle"` }` and check `true`, `true`, `["NvidiaTeslaT4"]`.
    7. `s.ConfigSnapshot().Kaggle` shows the same values. Make no further Chibi request after the reload.
  - `TestChibiSenseiConsentAndMeta`:
    1. `POST /consents` `{"profile_id":"nope"}` gives 422 `profile_unknown`.
    2. `GET /meta`:
       - gemma `license.accepted` false and `license.version` `"1.0"`;
       - qwen `license.accepted` true, `methods` non-empty, `max_seq_cap` > 0;
       - `training.machine_shape == "NvidiaTeslaT4"`, `training.backend == "kaggle"`;
       - `run_states` has 13 entries.
    3. `POST /consents` `{"profile_id":"gemma3-270m"}` gives 201 with `consent.license_id == "gemma"`, `license_version == "1.0"`, non-empty `accepted_at`. A second POST gives 201 with the same `accepted_at`.
    4. `GET /meta` now shows gemma `license.accepted` true.

- [ ] 7. Verify:

```bash
gofmt -l internal/server internal/chibisensei
go build ./...
go test ./internal/chibisensei/ -count=1 -run '(?i)Profile'
go test ./internal/server -count=1 -run '(?i)(ChibiSensei)' -timeout 10m
```

  Expected:
  - no gofmt output;
  - `--- PASS` for `TestChibiSenseiPreflightKaggleDisabled`, `TestChibiSenseiPreflightReady`, `TestChibiSenseiPreflightEnable` and `TestChibiSenseiConsentAndMeta`; the existing Chibi tests stay green;
  - `ok` for both packages.

---

#### Task B5.3: Run routes, router, SSE, project delete, stop classification, tests

**Files**
- Create: `internal/server/chibi_sensei_runs.go`, `internal/server/chibi_sensei_run_views.go`, `internal/server/chibi_sensei_runs_test.go`
- Modify:
  - `internal/server/chibi_sensei_handlers.go:35-70`, `:79-148`
  - `internal/server/chibi_sensei_service.go:346-386` (`chibiDomainError`)
  - `internal/server/chibi_sensei_jobs.go:86-151` (`handleChibiEvents`)
  - `internal/server/chibi_sensei_projects.go:199-235` (`handleChibiProjectDelete`)
  - `internal/server/desktop_operation.go:46`
  - `internal/server/desktop_audit_lifetime_test.go:32-41`
  - `internal/server/chibi_sensei_api_test.go:121-150`
  - `internal/server/chibi_sensei_app_browser_test.go:195-231`

**Existing symbols modified:** `handleChibiSensei`, `handleChibiProjectRoute`, `chibiDomainError`, `handleChibiEvents`, `handleChibiProjectDelete`, `desktopRequestOperation` (HIGH risk, every desktop route), `TestDesktopReadonlyCleanupRoutes`, `TestChibiSenseiReadOnlyRefusesWrites`, `TestChibiSenseiAppRealServerBrowser`

**Steps**

- [ ] 1. `chibiDomainError` (`chibi_sensei_service.go`): add two cases before `default:`, and extend the doc comment with "ErrRunActive → 409 run_active; ErrRunNotTerminal → 409 run_not_terminal":

```go
	case errors.Is(err, chibisensei.ErrRunActive):
		writeChibiError(w, http.StatusConflict, "run_active", "A training run is active.", nil)
	case errors.Is(err, chibisensei.ErrRunNotTerminal):
		writeChibiError(w, http.StatusConflict, "run_not_terminal", "The training run has not ended.", nil)
```

- [ ] 2. Create `internal/server/chibi_sensei_runs.go`:

```go
package server

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"aurago/internal/chibisensei"

	"github.com/google/uuid"
)

func chibiRunNotFound(w http.ResponseWriter) {
	writeChibiError(w, http.StatusNotFound, "run_not_found", "Run not found.", nil)
}

func chibiArtifactNotFound(w http.ResponseWriter) {
	writeChibiError(w, http.StatusNotFound, "artifact_not_found", "Artifact not found.", nil)
}

func chibiPlanExpired(w http.ResponseWriter) {
	writeChibiError(w, http.StatusConflict, "plan_expired", "The plan changed or the confirmation expired. Prepare the start again.", nil)
}

// chibiPreflightRefusal answers 409 with the failing block items in "missing": run_active when another run is active,
// license_consent_required when the license is the only gap, preflight_failed otherwise.
func chibiPreflightRefusal(w http.ResponseWriter, pf chibisensei.Preflight) {
	missing := []string{}
	for _, it := range pf.Items {
		if it.Level == chibisensei.PreflightBlock && !it.OK {
			missing = append(missing, it.ID)
		}
	}
	code, msg := "preflight_failed", "A requirement for training is not met."
	switch {
	case slices.Contains(missing, "run_active"):
		code, msg = "run_active", "A training run is active."
	case len(missing) == 1 && missing[0] == "license_consent":
		code, msg = "license_consent_required", "Accept the license of the base model first."
	}
	writeChibiJSON(w, http.StatusConflict, map[string]any{"error": code, "code": code, "message": msg, "missing": missing})
}

// chibiSnapshotBytes is the size of the canonical JSONL of the train, val and eval rows (what the snapshot will hold).
func chibiSnapshotBytes(ctx context.Context, st *chibisensei.Store, projectID string, split chibisensei.SplitResult) (int64, error) {
	var n int64
	err := st.EachRow(ctx, projectID, func(r chibisensei.Row) error {
		switch split.Assign[r.ID] {
		case "train", "val", "eval":
			n += int64(len(chibisensei.ExportLine(r)))
		}
		return nil
	})
	return n, err
}

// handleChibiRunPrepare answers POST /projects/{id}/runs/prepare {"preset"}: a fresh preflight, the confirmation data and
// a plan token bound to run ID, preset and plan hash. Nothing leaves the machine here.
func handleChibiRunPrepare(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	var req struct {
		Preset string `json:"preset"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) || !chibiValidPreset(w, req.Preset) {
		return
	}
	ctx := r.Context()
	tr, err := s.chibiTrainingState(ctx, r, acc, id, req.Preset, true)
	if err != nil {
		chibiTrainingError(w, err)
		return
	}
	if !tr.pf.Ready {
		chibiPreflightRefusal(w, tr.pf)
		return
	}
	dataBytes, err := chibiSnapshotBytes(ctx, acc.rt.store, id, tr.split)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	runID := uuid.NewString()
	token, ttl := acc.rt.plans.Issue(chibisensei.PlanTicket{ProjectID: id, RunID: runID, Preset: tr.plan.Preset, Hash: tr.planHash(acc.cfg)})
	owner, lic, tc := tr.in.Account, tr.prof.License, acc.cfg.ChibiSensei.Training
	var quota any
	if tr.in.QuotaKnown {
		quota = math.Round(tr.in.QuotaHours*10) / 10
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{
		"plan_token": token, "expires_in_s": int(ttl / time.Second), "run_id": runID, "plan": tr.plan,
		"destinations": map[string]any{"owner": owner,
			"dataset":       chibisensei.KaggleRef(owner, chibisensei.DatasetSlug(runID)),
			"notebook":      chibisensei.KaggleRef(owner, chibisensei.NotebookSlug(id, runID)),
			"machine_shape": tc.MachineShape, "private": true},
		"data": map[string]any{"rows": tr.plan.Split, "bytes": dataBytes},
		"license": map[string]any{"id": lic.ID, "name": lic.Name, "url": lic.URL, "version": lic.Version,
			"requires_consent": lic.RequiresConsent, "accepted": !lic.RequiresConsent || tr.in.ConsentRecorded},
		"time": map[string]any{"estimate_minutes": tr.plan.EstimateMinutes,
			"session_timeout_minutes": tr.plan.SessionTimeoutMinutes, "quota_hours_remaining": quota},
		"cleanup": map[string]any{"delete_remote_after_run": tc.DeleteRemoteAfterRun, "allow_delete": tr.in.AllowDelete},
	})
}

// handleChibiRunStart answers POST /projects/{id}/runs {"plan_token"}: consume the token (always), recompute plan and
// preflight with fresh Kaggle reads, compare the hash, submit. 409 plan_expired for an unknown, used, expired or
// mismatched token.
func handleChibiRunStart(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	var req struct {
		PlanToken string `json:"plan_token"`
	}
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	ticket, ok := acc.rt.plans.Consume(strings.TrimSpace(req.PlanToken), id)
	if !ok {
		chibiPlanExpired(w)
		return
	}
	ctx := r.Context()
	tr, err := s.chibiTrainingState(ctx, r, acc, id, ticket.Preset, true)
	if err != nil {
		chibiTrainingError(w, err)
		return
	}
	if !tr.in.ConnectionOK { // the plan hash needs the account; the preflight names the cause
		chibiPreflightRefusal(w, tr.pf)
		return
	}
	if tr.planHash(acc.cfg) != ticket.Hash {
		chibiPlanExpired(w)
		return
	}
	if !tr.pf.Ready {
		chibiPreflightRefusal(w, tr.pf)
		return
	}
	if lic := tr.prof.License; !lic.RequiresConsent {
		if _, err := acc.rt.store.RecordConsent(ctx, tr.prof.ID, lic.ID, lic.Version); err != nil {
			slog.Warn("[ChibiSensei] The license note could not be recorded", "profile", tr.prof.ID, "error", err)
		}
	}
	run, err := acc.rt.orch.Submit(ctx, chibisensei.SubmitInput{RunID: ticket.RunID, Project: tr.proj, Profile: tr.prof,
		ProfileSHA: tr.profSHA, Plan: tr.plan, Owner: tr.in.Account})
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	s.chibiStartOrchestrator(acc.rt)
	writeChibiJSON(w, http.StatusAccepted, map[string]any{"run": run})
}

// handleChibiProjectRuns: GET lists the runs of a project, POST starts one.
func handleChibiProjectRuns(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		handleChibiRunList(s, w, r, id)
	case http.MethodPost:
		handleChibiRunStart(s, w, r, id)
	default:
		writeChibiMethodError(w, "GET, POST")
	}
}

// handleChibiRunCancel is a stop operation: it works on a read-only desktop. The orchestrator only marks the run; the
// next step tries the remote cancel once.
func handleChibiRunCancel(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopStop, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	run, err := acc.rt.orch.Cancel(r.Context(), runID)
	if errors.Is(err, chibisensei.ErrNotFound) {
		chibiRunNotFound(w)
		return
	}
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	if !chibisensei.RunTerminal(run.State) {
		s.chibiStartOrchestrator(acc.rt)
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"run": run})
}

// handleChibiRunDelete deletes a finished run's rows, then its local files (RunDir, ModelDir; base GGUFs stay). Kaggle is
// not touched.
func handleChibiRunDelete(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	err := acc.rt.store.DeleteRun(r.Context(), runID)
	if errors.Is(err, chibisensei.ErrNotFound) {
		chibiRunNotFound(w)
		return
	}
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	if err := chibisensei.RemoveRunFiles(acc.rt.dataDir, runID); err != nil {
		slog.Warn("[ChibiSensei] Run files could not be removed", "run", runID, "error", err)
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] 3. Create `internal/server/chibi_sensei_run_views.go`. Every read route checks the method (GET; the file route also accepts HEAD), calls `chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)` and loads the run with `chibiLoadRun` (404 `run_not_found`). The helper, one complete read route and the file route:

```go
package server

import (
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"aurago/internal/chibisensei"
)

func chibiLoadRun(w http.ResponseWriter, r *http.Request, acc *chibiSenseiAccess, runID string) (chibisensei.Run, bool) {
	run, err := acc.rt.store.GetRun(r.Context(), runID)
	if errors.Is(err, chibisensei.ErrNotFound) {
		chibiRunNotFound(w)
		return chibisensei.Run{}, false
	}
	if err != nil {
		chibiDomainError(w, err)
		return chibisensei.Run{}, false
	}
	return run, true
}

// handleChibiRunMetrics answers GET /runs/{id}/metrics?since_step=N (inclusive, A25).
func handleChibiRunMetrics(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	since, ok := chibiQueryInt(w, r, "since_step", 0)
	if !ok {
		return
	}
	run, ok := chibiLoadRun(w, r, acc, runID)
	if !ok {
		return
	}
	points, err := acc.rt.store.ListMetrics(r.Context(), runID, max(since, 0))
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	maxSteps := 0
	if st := run.Remote.Stage; st != nil {
		maxSteps = st.MaxSteps
	}
	if maxSteps == 0 && run.Remote.Result != nil {
		maxSteps = run.Remote.Result.Steps
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"metrics": points, "max_steps": maxSteps, "live": run.Remote.Live})
}

const (
	chibiArtifactMinRate    = 256 << 10 // bytes per second a download is given time for
	chibiArtifactBaseWindow = 10 * time.Minute
	chibiArtifactMaxWindow  = 4 * time.Hour
)

// chibiArtifactWriteWindow is the write deadline of one artifact download: 10 min plus the size at 256 KiB/s, at most
// 4 h. It replaces the listener's absolute write timeout (40 min) for this response only, as the Video Studio upload
// widens its window (video_studio_service.go:288-316); httpstream renews deadlines only for SSE and media.
func chibiArtifactWriteWindow(size int64) time.Duration {
	return min(chibiArtifactBaseWindow+time.Duration(size/chibiArtifactMinRate)*time.Second, chibiArtifactMaxWindow)
}

// handleChibiArtifactFile serves GET /runs/{id}/artifacts/{aid}/file as an attachment (Range, HEAD) through os.Root.
func handleChibiArtifactFile(s *Server, w http.ResponseWriter, r *http.Request, runID, artifactID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeChibiMethodError(w, "GET, HEAD")
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	a, err := acc.rt.store.GetArtifact(r.Context(), artifactID)
	if errors.Is(err, chibisensei.ErrNotFound) || (err == nil && a.RunID != runID) {
		chibiArtifactNotFound(w)
		return
	}
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	f, err := chibisensei.OpenArtifact(acc.rt.dataDir, a)
	if errors.Is(err, fs.ErrNotExist) {
		chibiArtifactNotFound(w)
		return
	}
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		chibiArtifactNotFound(w)
		return
	}
	name := "chibi-" + chibisensei.Run12(runID) + "-" + a.Name
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(chibiArtifactWriteWindow(info.Size()))); err != nil &&
		!errors.Is(err, http.ErrNotSupported) {
		slog.Warn("[ChibiSensei] The download deadline could not be extended", "error", err)
	}
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, name, info.ModTime(), f)
}
```

  The remaining read routes in the same file, with exact signatures and responses:
  - `func handleChibiRunList(s *Server, w http.ResponseWriter, r *http.Request, id string)`:
    - `GetProject` first; `ErrNotFound` gives `chibiDomainError` 404 `not_found`.
    - `limit` comes from `chibiQueryInt(w, r, "limit", 20)` and goes into `ListRuns`.
    - Answer `{"runs":[RunSummary]}`, built from `run.Summary()` into a non-nil `[]chibisensei.RunSummary{}`.
  - `func handleChibiRun(s, w, r, runID string)`: `{"run": run}` (`Run.MarshalJSON` is the full view).
  - `func handleChibiRunEvents(s, w, r, runID string)`: `after` from `chibiQueryInt(w, r, "after", 0)` (cast to `int64`), `limit` default 200; answer `{"events": ListRunEvents(ctx, runID, after, limit)}`.
  - `func handleChibiRunArtifacts(s, w, r, runID string)`: `{"artifacts": ListArtifacts(ctx, runID)}`. The JSON hides run, project and path (A29).
  - `func handleChibiRunReport(s, w, r, runID string)`: `GetReport(ctx, runID, "auto")`, then `{"report": body}` or `{"report": nil}`.
  - `func handleChibiRunLog(s, w, r, runID string)`: `b, err := chibisensei.ReadLogTail(acc.rt.dataDir, runID)`, then `{"tail": string(b), "bytes": len(b)}`. The text is already scrubbed and at most 64 KiB.

- [ ] 4. Router (`chibi_sensei_handlers.go`). The full set of new patterns:
  - `POST /consents`
  - `POST /preflight/enable`
  - `GET /projects/{id}/preflight`
  - `GET|POST /projects/{id}/runs`
  - `POST /projects/{id}/runs/prepare`
  - `GET /runs/{runId}`
  - `POST /runs/{runId}/cancel`
  - `POST /runs/{runId}/delete`
  - `GET /runs/{runId}/metrics`, `/events`, `/artifacts`, `/report`, `/log`
  - `GET|HEAD /runs/{runId}/artifacts/{artifactId}/file`

  Changes:
  - In `handleChibiSensei`, before `default:`:

```go
		case len(parts) >= 2 && parts[0] == "runs":
			if !chibisensei.ValidID(parts[1]) {
				chibiRunNotFound(w)
				return
			}
			handleChibiRunRoute(s, w, r, parts[1], parts[2:])
```

  - In `handleChibiProjectRoute`, in the `len(rest) == 1` switch, add `case "runs": handleChibiProjectRuns(s, w, r, id)`. Before `default:` (line 146), add:

```go
	case len(rest) == 2 && rest[0] == "runs" && rest[1] == "prepare":
		handleChibiRunPrepare(s, w, r, id)
```

  - Add after `handleChibiProjectRoute`:

```go
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
		default:
			chibiNotFound(w)
		}
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
```

- [ ] 5. SSE (`chibi_sensei_jobs.go`):
  - Change the comment of `handleChibiEvents` to: "…on connect one "job" event per unfinished job (oldest first), the current "rows" revision and, when the project has a non-terminal run, that run as a "run" event (RunSummary); then every hub event (job, rows, checks, run, metric, stage)…".
  - Replace line 149 (`if !send("rows", …) || !flush() {`) and its `return` block with:

```go
	if !send("rows", map[string]any{"data_rev": project.DataRev}) {
		return
	}
	if runs, err := acc.rt.store.ActiveRuns(ctx); err == nil {
		for _, run := range runs {
			if run.ProjectID == id && !send("run", run.Summary()) {
				return
			}
		}
	}
	if !flush() {
		return
	}
```

  Hub events `run`, `metric` and `stage` need no code: the loop forwards every hub event by name.

- [ ] 6. Project delete (`chibi_sensei_projects.go`):
  - Comment: "…The checks (archive, name, no active run) run before the jobs are cancelled, so a refused delete leaves the running jobs alone; after the store delete the files of every run go."
  - Replace lines 229-234 (from `acc.rt.jobs.CancelProject(id)` to the end) with:

```go
	ctx := r.Context()
	active, err := acc.rt.store.ActiveRuns(ctx)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	for _, run := range active {
		if run.ProjectID == id {
			chibiDomainError(w, chibisensei.ErrRunActive)
			return
		}
	}
	runIDs, err := acc.rt.store.ListRunIDs(ctx, id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	acc.rt.jobs.CancelProject(id)
	if err := acc.rt.store.DeleteProject(ctx, id, req.ConfirmName); err != nil {
		chibiDomainError(w, err)
		return
	}
	for _, runID := range runIDs {
		if err := chibisensei.RemoveRunFiles(acc.rt.dataDir, runID); err != nil {
			slog.Warn("[ChibiSensei] Run files of a deleted project could not be removed", "run", runID, "error", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
```

  Add the import `"log/slog"`.

- [ ] 7. `desktopRequestOperation` (`desktop_operation.go`). This function is HIGH risk: every desktop route depends on it. Exactly one new rule, inserted after line 46:

```diff
             (len(parts) == 6 && parts[0] == "api" && parts[1] == "desktop" && parts[2] == "chibi-sensei" && parts[3] == "jobs" && parts[5] == "cancel") ||
+			(len(parts) == 6 && parts[0] == "api" && parts[1] == "desktop" && parts[2] == "chibi-sensei" && parts[3] == "runs" && parts[5] == "cancel") ||
             (len(parts) == 6 && parts[0] == "api" && parts[1] == "desktop" && parts[2] == "personal-radio" && parts[3] == "stations" && (parts[5] == "stop" || parts[5] == "pause")) ||
```

  The test that pins the classification table is `TestDesktopReadonlyCleanupRoutes` (`desktop_audit_lifetime_test.go:12-42`). Extend both of its lists:

```diff
-	for _, path := range []string{"/api/game-maker/jobs/job/cancel", "/api/desktop/personal-radio/stations/id/stop", "/api/desktop/personal-radio/stations/id/pause", "/api/desktop/looper/stop", "/api/desktop/looper/pause"} {
+	for _, path := range []string{"/api/game-maker/jobs/job/cancel", "/api/desktop/personal-radio/stations/id/stop", "/api/desktop/personal-radio/stations/id/pause", "/api/desktop/looper/stop", "/api/desktop/looper/pause", "/api/desktop/chibi-sensei/runs/r/cancel"} {
@@
-	for _, path := range []string{"/api/game-maker/jobs/job/cancel/extra", "/api/desktop/personal-radio/stations/id/resume"} {
+	for _, path := range []string{"/api/game-maker/jobs/job/cancel/extra", "/api/desktop/personal-radio/stations/id/resume", "/api/desktop/chibi-sensei/runs/r/cancel/extra", "/api/desktop/chibi-sensei/runs/r/delete", "/api/desktop/chibi-sensei/projects/p/runs"} {
```

- [ ] 8. Extend `TestChibiSenseiReadOnlyRefusesWrites` (`chibi_sensei_api_test.go`):
  - Add to `writes` (each must answer 403 `desktop_readonly`):

```go
		{http.MethodPost, base + "/runs/prepare", `{"preset":"quick"}`},
		{http.MethodPost, base + "/runs", `{"plan_token":"x"}`},
		{http.MethodPost, "/consents", `{"profile_id":"qwen35-0p8b"}`},
		{http.MethodPost, "/preflight/enable", `{"flags":["kaggle.enabled"]}`},
		{http.MethodPost, "/runs/" + chibiFakeID + "/delete", ``},
```

  - Add `base + "/preflight?preset=quick"` and `base + "/runs"` to the GET list (200).
  - After the job cancel, add:

```go
	// A run cancel is a stop operation too.
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runs/"+chibiFakeID+"/cancel", nil), http.StatusNotFound, "run_not_found")
```

- [ ] 9. Create `internal/server/chibi_sensei_runs_test.go` (imports: `bufio`, `context`, `crypto/sha256`, `encoding/hex`, `encoding/json`, `errors`, `io/fs`, `net/http`, `net/http/httptest`, `os`, `path/filepath`, `strings`, `testing`, `time`, `aurago/internal/chibisensei`, `github.com/google/uuid`).

```go
// chibiPrepare runs prepare and returns the plan token.
func chibiPrepare(t *testing.T, s *Server, token, pid string) string {
	t.Helper()
	w := chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/runs/prepare", map[string]any{"preset": "quick"})
	chibiWant(t, w, http.StatusOK, "")
	tok, _ := chibiBody(t, w)["plan_token"].(string)
	if tok == "" {
		t.Fatalf("prepare: %s", w.Body.String())
	}
	return tok
}

func TestChibiSenseiRunPrepareAndStart(t *testing.T) {
	s, readToken, token := chibiAPIFixture(t)
	chibiUseStubKaggle(t, s)
	chibiReadyKaggle(t, s)
	pid := chibiReadyProject(t, s, token)
	base := "/projects/" + pid

	chibiWant(t, chibiDo(t, s, readToken, http.MethodPost, base+"/runs/prepare", map[string]any{"preset": "quick"}), http.StatusForbidden, "")
	w := chibiDo(t, s, token, http.MethodPost, base+"/runs/prepare", map[string]any{"preset": "quick"})
	chibiWant(t, w, http.StatusOK, "")
	var prep struct {
		PlanToken    string           `json:"plan_token"`
		ExpiresInS   int              `json:"expires_in_s"`
		RunID        string           `json:"run_id"`
		Plan         chibisensei.Plan `json:"plan"`
		Destinations struct {
			Owner, Dataset, Notebook string
			MachineShape             string `json:"machine_shape"`
			Private                  bool
		} `json:"destinations"`
		Data struct {
			Rows  chibisensei.PlanSplit `json:"rows"`
			Bytes int64                 `json:"bytes"`
		} `json:"data"`
		License struct {
			RequiresConsent bool `json:"requires_consent"`
			Accepted        bool `json:"accepted"`
		} `json:"license"`
		Time struct {
			Quota *float64 `json:"quota_hours_remaining"`
		} `json:"time"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &prep); err != nil {
		t.Fatal(err)
	}
	if prep.PlanToken == "" || prep.ExpiresInS != 900 || !chibisensei.ValidID(prep.RunID) ||
		prep.Destinations.Owner != "tester" || prep.Destinations.Dataset != "tester/"+chibisensei.DatasetSlug(prep.RunID) ||
		prep.Destinations.Notebook != "tester/"+chibisensei.NotebookSlug(pid, prep.RunID) || !prep.Destinations.Private ||
		prep.Data.Rows != prep.Plan.Split || prep.Data.Bytes <= 0 || prep.License.RequiresConsent || !prep.License.Accepted ||
		prep.Time.Quota == nil || *prep.Time.Quota != 30 {
		t.Fatalf("prepare = %s", w.Body.String())
	}

	// A row change after prepare expires the plan.
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/rows", map[string]any{"rows": chibiPairs(1, false)}), http.StatusCreated, "")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/runs", map[string]any{"plan_token": prep.PlanToken}), http.StatusConflict, "plan_expired")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/checks/run", nil), http.StatusOK, "")

	tok1, tok2 := chibiPrepare(t, s, token, pid), chibiPrepare(t, s, token, pid)
	w = chibiDo(t, s, token, http.MethodPost, base+"/runs", map[string]any{"plan_token": tok1})
	chibiWant(t, w, http.StatusAccepted, "")
	var started struct {
		Run chibisensei.RunView `json:"run"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started.Run.State != chibisensei.RunQueued || started.Run.ProjectID != pid || started.Run.LastState != chibisensei.RunQueued {
		t.Fatalf("started run = %s", w.Body.String())
	}
	// A token works once; a second prepared start meets the active run.
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/runs", map[string]any{"plan_token": tok1}), http.StatusConflict, "plan_expired")
	w = chibiDo(t, s, token, http.MethodPost, base+"/runs", map[string]any{"plan_token": tok2})
	chibiWant(t, w, http.StatusConflict, "run_active")
	if !strings.Contains(w.Body.String(), `"missing":["run_active"]`) {
		t.Fatalf("run_active refusal = %s", w.Body.String())
	}
	project := chibiBody(t, chibiDo(t, s, token, http.MethodGet, base, nil))["project"].(map[string]any)
	if steps := project["steps"].(map[string]any); steps["training"] != "in_progress" {
		t.Fatalf("steps = %v", steps)
	}

	// Read-only desktop: prepare and start are refused, cancel is not.
	cfg := *s.ConfigSnapshot()
	cfg.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&cfg)
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/runs/prepare", map[string]any{"preset": "quick"}), http.StatusForbidden, "desktop_readonly")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/runs", map[string]any{"plan_token": "x"}), http.StatusForbidden, "desktop_readonly")
	w = chibiDo(t, s, token, http.MethodPost, "/runs/"+started.Run.ID+"/cancel", nil)
	chibiWant(t, w, http.StatusOK, "")
	var cancelled struct {
		Run chibisensei.RunView `json:"run"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cancelled); err != nil || !cancelled.Run.CancelRequested || cancelled.Run.Cancel == nil {
		t.Fatalf("cancel = %v %s", err, w.Body.String())
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runs/"+chibiFakeID, nil), http.StatusNotFound, "run_not_found")
}
```

  Also `TestChibiSenseiRunPreflightRefusal`:
  - Kaggle stays disabled; use `chibiUseStubKaggle` and `chibiReadyProject`.
  - `prepare` gives 409 `preflight_failed`, and `missing` contains `"kaggle_enabled"`.

  Helper and finished-run test (complete):

```go
// chibiStoredRun creates a finished run of pid directly in the store, with metrics, one artifact file, a report and a
// log tail (B4 layout: models/<run>/model-q8_0.gguf, runs/<run>/out/log-tail.txt).
func chibiStoredRun(t *testing.T, rt *chibiSenseiRuntime, pid string) string {
	t.Helper()
	ctx := context.Background()
	id := uuid.NewString()
	if err := rt.store.CreateRun(ctx, chibisensei.Run{ID: id, ProjectID: pid, ProfileID: "qwen35-0p8b", Preset: "quick"}); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.UpdateRun(ctx, id, chibisensei.NewStateRunEvent(chibisensei.RunSucceeded), func(r *chibisensei.Run) error {
		r.State, r.Remote.LastState, r.Remote.Finished = chibisensei.RunSucceeded, chibisensei.RunCleaning, "2026-11-23T10:00:00Z"
		r.Remote.Stage = &chibisensei.RunStage{Name: "manifest", State: "done", Step: 20, MaxSteps: 20, Epoch: 1}
		r.Remote.Result = &chibisensei.RunResult{Steps: 20, Epochs: 1, StopReason: "done", GPU: "Tesla T4"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := rt.store.UpsertMetrics(ctx, id, []chibisensei.Metric{
		{Step: 10, Kind: "train", Loss: 2, TS: "2026-11-23T09:50:00Z"},
		{Step: 20, Kind: "train", Loss: 1, TS: "2026-11-23T09:55:00Z"}}); err != nil {
		t.Fatal(err)
	}
	model := []byte("GGUF-test")
	if err := os.MkdirAll(chibisensei.ModelDir(rt.dataDir, id), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chibisensei.ModelDir(rt.dataDir, id), "model-q8_0.gguf"), model, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(model)
	if err := rt.store.AddArtifact(ctx, chibisensei.Artifact{RunID: id, ProjectID: pid, Kind: "gguf_tuned",
		RelPath: "models/" + id + "/model-q8_0.gguf", Bytes: int64(len(model)), SHA256: hex.EncodeToString(sum[:])}); err != nil {
		t.Fatal(err)
	}
	if err := rt.store.SaveReport(ctx, id, "auto", chibisensei.Report{RunID: id, Kind: "auto", Samples: 20, SmallSample: true}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(chibisensei.RunDir(rt.dataDir, id), "out")
	if err := os.MkdirAll(out, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "log-tail.txt"), []byte("step 20 done\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return id
}
```

  `TestChibiSenseiRunReadsAndDelete`:
  1. `chibiAPIFixture`, `chibiUseStubKaggle`, then a qwen35 project named `"Bot"`. Take `rt, release, _ := s.acquireChibiSensei()` with `defer release()`; `id := chibiStoredRun(t, rt, pid)`.
  2. Then assert:
     - `GET /projects/{pid}/runs`: one run with that `id` and no `hyper` key;
     - `GET /runs/{id}`: `run.state == "succeeded"`, `run.hyper` present;
     - `GET /runs/{id}/metrics?since_step=15`: one point (step 20), `max_steps == 20`, `live == "unknown"`;
     - `GET /runs/{id}/events`: at least 2 events, `events[0].kind == "state"`;
     - `GET /runs/{id}/artifacts`: one artifact, `name == "model-q8_0.gguf"`, no `rel_path` key;
     - `GET .../artifacts/{aid}/file`: 200, body `GGUF-test`, `Content-Disposition` contains `attachment` and `chibi-`+`Run12(id)`;
     - `GET .../artifacts/{chibiFakeID}/file`: 404 `artifact_not_found`;
     - `GET /runs/{id}/report`: `report.samples == 20`;
     - `GET /runs/{id}/log`: `tail == "step 20 done\n"`, `bytes == 13`.
  3. `POST /runs/{id}/delete` gives 204. After it:
     - `os.Stat(chibisensei.RunDir(rt.dataDir, id))` and `ModelDir` report `fs.ErrNotExist`;
     - `GET /runs/{id}` gives 404 `run_not_found`.
  4. A second run `other` (`CreateRun`, queued) plus `os.MkdirAll(filepath.Join(chibisensei.RunDir(rt.dataDir, other), "data"), 0o700)`:
     - `POST /runs/{other}/delete` gives 409 `run_not_terminal`;
     - archive the project (`{"archived":true}`); `POST /projects/{pid}/delete` `{"confirm_name":"Bot"}` gives 409 `run_active`;
     - end `other` with `UpdateRun(... r.State = chibisensei.RunCancelled ...)`; project delete gives 204; `RunDir(other)` is gone.

  `TestChibiSenseiEventsSendActiveRun` (complete):

```go
func TestChibiSenseiEventsSendActiveRun(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	chibiUseStubKaggle(t, s)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant", "profile_id": "qwen35-0p8b"})
	rt, release, err := s.acquireChibiSensei()
	if err != nil {
		t.Fatal(err)
	}
	runID := uuid.NewString()
	err = rt.store.CreateRun(context.Background(), chibisensei.Run{ID: runID, ProjectID: pid, ProfileID: "qwen35-0p8b", Preset: "quick"})
	release()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handleChibiSensei(s))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+chibiSenseiAPIBase+"/projects/"+pid+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	sawRows, event := false, ""
	for sc.Scan() {
		line := sc.Text()
		if name, ok := strings.CutPrefix(line, "event: "); ok {
			event = name
			sawRows = sawRows || name == "rows"
			continue
		}
		if event == "run" && strings.HasPrefix(line, "data: ") {
			if !sawRows || !strings.Contains(line, runID) {
				t.Fatalf("run event before rows or for another run: %s", line)
			}
			return
		}
	}
	t.Fatalf("the stream ended without a run event: %v", sc.Err())
}
```

- [ ] 10. Browser smoke (`chibi_sensei_app_browser_test.go`). After step 5 (line 198), insert the block below and renumber the old steps 6 and 7 to 7 and 8:

```go
	// 6. Training step: the preflight answers with Kaggle switched off; nothing is started.
	csnbClick(t, page, "the training step opens", `[data-csn="step"][data-step="training"]`)
	csnbWait(t, page, "the preflight has loaded", `()=>!!document.querySelector('[data-csn="preflight"][aria-busy="false"]')`)
	csnbWait(t, page, "the preflight summary says blocked", `()=>!!document.querySelector('[data-csn="preflight-summary"][data-state="blocked"]')`)
	csnbWait(t, page, "Kaggle is reported as switched off", `()=>!!document.querySelector('[data-csn="preflight-item"][data-id="kaggle_enabled"][data-ok="0"][data-level="block"]')`)
	csnbWait(t, page, "the one-row dataset is reported as not ready", `()=>!!document.querySelector('[data-csn="preflight-item"][data-id="dataset_ready"][data-ok="0"]')`)
	csnbWait(t, page, "the primary action is disabled with a hint", `()=>{const b=document.querySelector('[data-csn="primary"]');const h=document.querySelector('[data-csn="primary-hint"]');return !!b&&b.disabled&&!!h&&h.textContent.trim()!=='';}`)
	csnbWait(t, page, "the enable button or the ask text is shown", `()=>!!document.querySelector('[data-csn="preflight-enable"],[data-csn="preflight-ask"]')`)
	csnbWait(t, page, "three preset cards are shown", `()=>document.querySelectorAll('[data-csn="preset-card"]').length===3`)
```

  At the end of the test, after the projects check, add:

```go
	// 9. The smoke never started a run.
	res, err = http.Get(origin + chibiSenseiAPIBase + "/projects/" + list.Projects[0].ID + "/runs")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"runs":[]`) {
		t.Fatalf("GET /runs = %d %s, want no run", res.StatusCode, body)
	}
```

  The smoke uses the real adapter. With Kaggle disabled the preflight never opens the Kaggle service (`KaggleReachable` is false and `kaggleEnv` skips `Status`), so no request fails.

- [ ] 11. Verify:

```bash
gofmt -l internal/server
git diff --check
go build ./...
go test ./internal/server -count=1 -run '(?i)(ChibiSensei|DesktopReadonlyCleanupRoutes)' -timeout 10m
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestChibiSenseiAppRealServerBrowser$' -count=1 -timeout 15m
```

  Expected:
  - `--- PASS` for `TestChibiSenseiRunPrepareAndStart`, `TestChibiSenseiRunPreflightRefusal`, `TestChibiSenseiRunReadsAndDelete`, `TestChibiSenseiEventsSendActiveRun`, `TestChibiSenseiReadOnlyRefusesWrites` and `TestDesktopReadonlyCleanupRoutes`;
  - all B5.1/B5.2 tests and the P1 Chibi tests stay green; `ok aurago/internal/server`;
  - the smoke passes when F2 to F4 have landed, or skips without Chrome. If F-batches are missing, report it and do not adapt the selectors.

---

#### Task B5.4: Docs and DOX

**Files**
- Modify:
  - `internal/chibisensei/AGENTS.md`, `internal/server/AGENTS.md:314-320`, `internal/kaggle/AGENTS.md` (Ownership), `AGENTS.md` (Child DOX Index, Chibi Sensei row)
  - `documentation/chibi-sensei.md:3`, `:47-49`, `:78-80`, `:100-102` and a new section before `## Storage`
  - `documentation/configuration.md:611`
  - `documentation/manuals/en/07-configuration.md:1344`, `documentation/manuals/de/07-configuration.md:906`
  - `documentation/manuals/en/21-api-reference.md:1636-1651`, `documentation/manuals/de/21-api-reference.md:1645-1660`

**Existing symbols modified:** none

**Steps**

- [ ] 1. `internal/chibisensei/AGENTS.md`.
  - **Purpose**: replace the paragraph with:

    > Domain package of the Chibi Sensei desktop app (guided studio for training small language models): the SQLite store, rows, history with undo, checks, split, presets, plan, profiles, imports, documents, export, the job runner, the event hub, the prompts, schemas and parsers of generation and cleaning, and the Kaggle training runs (trainer script and lock, job spec, notebook, snapshot, log and output parsers, run store, consents, plan tokens, preflight evaluation, report scoring, the `KaggleDriver` interface and the run orchestrator). It never imports `internal/server`, `internal/desktop` or `internal/llm`; it may import `internal/kaggle` and `internal/config`. The model call reaches it only through values the server hands in (`wrap func(string) string` for untrusted text, parsed answers as strings), and Kaggle only through the `KaggleDriver` the server implements (`internal/server/chibi_sensei_kaggle.go`). Spec: `docs/superpowers/specs/2026-11-22-chibi-sensei-design.md`; plans: `docs/superpowers/plans/2026-11-22-chibi-sensei-p1*.md` and the P2 plan files beside them.

  - **Ownership**: append two bullets.
    - Training files:

      > Training (P2): `trainer/` (`chibi_train.py` and `requirements.lock`, embedded by `trainer/embed.go`; lock keys `pip` (repeatable, exact `name==version`), `transformers`, `llama_cpp_tag`, `llama_cpp_url`, `llama_cpp_sha256`, each other key once); `jobspec.go` (`job.json` v1), `notebook.go`, `snapshot.go` (split files, marker, `Run12`, `DatasetSlug`, `NotebookSlug`), `logparse.go`, `outputs.go`; `runs.go` (run model, states, API view), `runstore.go`, `consents.go` (consents and settings), `plantoken.go`, `preflight.go`, `quota.go`, `report.go`, `driver.go` (`KaggleDriver`, request IDs, job statuses); `reconcile*.go` (orchestrator), `artifacts.go` (storage layout, verified copies, `base_gguf`), `gguf.go`; `testdata/` (goldens, proof data); tests `fakekaggle_test.go`, `reconcile_test.go`, `reconcile_restart_test.go`, `proofbundle_test.go`.

    - Storage directory:

      > `data/chibi_sensei/` with `runs/<run>/` (snapshot, notebook, `out/` small outputs and `log-tail.txt`), `models/<run>/` (GGUF, adapter) and `base/` (shared base GGUFs). Not part of AuraGo backups; only `chibi_sensei.db` is.

  - **Local Contracts**:
    - Replace the last bullet ("Profiles stay `experimental` until a P2 proof run.") with:

      > A profile is `experimental` with a non-empty `unverified` list until its Kaggle proof run passed (spec "Testing", manual proof run); `verified` needs an empty `unverified` list and the `verified` block (`ValidateProfile`).

    - Then append:
      - Trainer contract:

        > Trainer contract: stages are `TrainerStages` (boot, env_check, data, install, load_base, converter, base_eval, train, tuned_eval, save, gguf, manifest); the parsers reject other names. `@@chibi` lines carry `seq` except the wrapper's exit line; metric lines carry `kind`. Outputs are flat files in `/kaggle/working` named by the `Out*` constants (`chibi-status.json`, `chibi-manifest.json`, `chibi-metrics.jsonl`, `chibi-eval_samples.jsonl`, `chibi-model-q8_0.gguf`, `chibi-base-q8_0.gguf`, `chibi-adapter-model.safetensors`, `chibi-adapter-config.json`); each manifest kind at most once (`ManifestKindFile`). Dataset files are `r<run12>-{train,val,eval}.jsonl` and `r<run12>-marker.json`; a split without rows has no file. No secret enters `job.json`, the notebook, logs or events; Kaggle log text is scrubbed before it is stored.

      - Deterministic outputs:

        > `job.json` and the notebook are deterministic bytes. A change to their builders, the trainer, the lock or a profile regenerates the goldens in the same change. `.gitattributes` keeps LF for `internal/chibisensei/trainer/**` and `internal/chibisensei/testdata/**` (the script hash and the goldens depend on it).

      - Runs:

        > Runs: one non-terminal run per installation (`CreateRun` → `ErrRunActive`); `DeleteRun` only for terminal runs (`ErrRunNotTerminal`); `DeleteProject` refuses a project with a non-terminal run. `UpdateRun` loads, changes, stores and appends at most one event in one transaction (an empty kind writes none); publish after the commit. Data without a column lives in `remote_json`; absent times are `""`; `last_state` is the last non-terminal state. `run_events.data` shapes: `state {to}`, `stage {stage,state}`, `warn {code,message}`, `error {stage,code,message}`, `kaggle {step,job_id}`, `log {text}`. `since_step` is inclusive and a non-finite loss is never stored. Artifact JSON hides run, project and path; `name` is the base name; artifact IDs are UUID v5 of run and kind. The Kaggle project id is the setting `kaggle_project_id`.

      - Plan tokens:

        > Plan tokens live in memory (15 min, at most 64, single use) and bind project, run ID, preset and `PlanHash` (plan with `data_rev`, profile SHA, script SHA, owner, machine shape, delete flag).

      - Preflight:

        > Preflight evaluation is pure: the server collects `PreflightInput`; `KaggleReachable` gates the connection test and the quota read; item IDs, order and args follow the HTTP contract. `PreflightFlagPatch` is the only mapping from flag names to config values. `ParseGPUQuota` stays provisional until a proof run records the real answer.

      - Report:

        > Report: `loss` and `perplexity` values may be null; a confusion matrix has one row per label and one extra "other" column; samples whose expected answer is no label are left out of `label_accuracy`.

      - Orchestrator:

        > Orchestrator: one goroutine owns the active run; `Step` is the unit of work, at most one Kaggle mutation and one transition, each an `UpdateRun` with hub events after the commit. Request IDs are `KaggleRequestID(run, step, attempt)`; `unknown` is never replayed (uploads and downloads take a new attempt, `create_dataset` is decided by `get_dataset`, `save_kernel` by `get_kernel_session_status`: answers → untracked, 404 → `kaggle_submit_unknown`); at most 3 attempts. A driver refusal makes the run wait with one `warn` per code and fails it after 6 h before submission or while downloading; cleaning never waits and records leftovers. The deadline is remote start + session + 20 min, decided by one final read. Every run passes `cleaning`. A cancel before submission stops locally; after submission one remote cancel is tried and tracking goes on. Downloads are checked against `max_transfer_mb`, the storage cap and free disk, copied through `os.Root` with size and sha256 from the manifest, under names AuraGo generates. The log is read whole (16 MiB); only the scrubbed last 64 KiB is stored. `ReadGGUFInfo` reads only the header with fixed caps. Base GGUFs live in `base/` with a `base_gguf` row and are never removed by `RemoveRunFiles`.

  - **Work Guidance**: append:

    > A new run failure or warning code gets a UI message key and a row in the failure table of `documentation/chibi-sensei.md`. The server owns the HTTP shapes; keep `RunView`, `PreflightItem` and `Report` JSON in step with the HTTP contract.

  - **Verification**: replace with:

    ```
    go test ./internal/chibisensei/... -count=1
    go test ./internal/chibisensei/ -count=1 -run '(?i)Orchestrator'        # state machine and restart table against fakeKaggle
    go test ./internal/chibisensei/ -count=1 -run '(?i)(Golden|ProofData)' -update   # rewrite goldens and proof data; review the diff
    CHIBI_PROOF_OUT=<dir> CHIBI_PROOF_PROFILE=<profile id> go test ./internal/chibisensei/ -count=1 -run '^TestProofBundle$'
    go test ./internal/chibisensei/trainer -count=1
    ```

    Followed by the sentence: "`TestProofBundle` writes the data files, marker, `job.json` and notebook of one manual proof run (optional `CHIBI_PROOF_OWNER`, `CHIBI_PROOF_MACHINE_SHAPE`, `CHIBI_PROOF_RECIPE`, `CHIBI_PROOF_PRESET`, `CHIBI_PROOF_TIMEOUT_MINUTES`); it is skipped without `CHIBI_PROOF_OUT`. The trainer tests (`--selftest`, `--dry-run` with `CHIBI_FAKE_ENV=1`) need Python 3.10+ as `python3` or `python` on PATH and skip otherwise."

- [ ] 2. `internal/server/AGENTS.md`, section "### Chibi Sensei":
  - In the first bullet (line 316), replace "and a job cancel is `desktopStop`" with "and a job or run cancel is `desktopStop`".
  - Insert after line 319:

    > - Training (P2): `chibi_sensei_kaggle.go` implements `chibisensei.KaggleDriver` over `ensureKaggleServices()` with the live `kaggleSnapshot().Access`. Mutations (`Execute` of non-read operations, `EnsureProject`, `WriteFile`) and `ReadKernelLog` refuse a read-only desktop with `chibisensei.ErrDesktopReadOnly`; mutations pass `kaggle.CheckGrants` before `Service.Execute`, so `BeginJob` never admits a failed row for a refused attempt. Reads, `OpenFile`, `RemoveFile` (only files Chibi staged) and `Cancel` have no desktop gate; the Kaggle project id is the Chibi setting `kaggle_project_id`. Each runtime owns the orchestrator, the plan tokens and a 20 s cache of the Kaggle reads (connection, GPU quota, compute jobs; keyed by credentials and access, cleared by the enable route, bypassed by prepare and start). `openChibiRuntime` starts the orchestrator only when a non-terminal run exists, start and cancel start it otherwise; a retire closes it before the job runner and the store, and a data-directory change pauses an active run until that directory is active again. Routes: `GET projects/{id}/preflight`; `POST preflight/enable` (admin, flag names only, `PreflightFlagPatch` through `applyConfigPatch` under `CfgSaveMu`, logs flag names); `POST consents`; `POST projects/{id}/runs/prepare` (fresh preflight, data bytes, plan token); `POST projects/{id}/runs` (consume, recompute, 409 `plan_expired`, `preflight_failed`, `run_active` or `license_consent_required` with `missing`, then `Submit`); `GET projects/{id}/runs`; `runs/{id}` with `metrics`, `events`, `artifacts`, `artifacts/{aid}/file`, `report`, `log`, `cancel` (`desktopStop`, also in `desktopRequestOperation`) and `delete` (terminal runs, then `RemoveRunFiles`). The new POST routes require a same-origin `Origin` for non-bearer callers (`csrf_check_failed`). Artifact files use `http.ServeContent` (Range) with a write deadline of 10 min plus the size at 256 KiB/s, at most 4 h. The event stream sends the project's active run after the first `rows` event and forwards `run`, `metric` and `stage`. A project delete is refused while one of its runs is active and removes every run's files after the store delete. Tests replace the driver with `Server.chibiSenseiKaggleOverride` (read in `openChibiRuntime`), keep the loop off with `chibiSenseiNoReconcile` (read in `chibiStartOrchestrator`) and fake free disk with `chibiFreeDiskOverride`.

  - Replace the Verify line with:

    > - Verify `go test ./internal/server -run '(?i)(ChibiSensei|DesktopReadonlyCleanupRoutes)' -count=1 -timeout 10m` and `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestChibiSenseiAppRealServerBrowser$' -count=1 -timeout 15m`.

- [ ] 3. `internal/kaggle/AGENTS.md`, Ownership: append

  > In-process caller: Chibi Sensei drives Kaggle only through `internal/server/chibi_sensei_kaggle.go` (its `KaggleDriver`), with the server's snapshot access. It gates mutations before `Execute`, uses `RemoveFile` only for files it staged and `ReadKernelLog` only for its own run notebooks; see `../chibisensei/AGENTS.md`.

- [ ] 4. Root `AGENTS.md`, Child DOX Index. Replace the Chibi Sensei row with:

  ```
  | Chibi Sensei | `internal/chibisensei/AGENTS.md`, `internal/server/AGENTS.md`, `internal/kaggle/AGENTS.md`, `internal/desktop/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` | Small-model training studio: dataset store, checks, split, plan, profiles, imports/exports, one-shot generation and cleaning; training on Kaggle (trainer script and lock, job spec and notebook, run store and orchestrator over the in-process Kaggle driver, preflight with admin enabling, plan tokens, license consents, artifacts and report cards); `/api/desktop/chibi-sensei`, config section and `documentation/chibi-sensei.md`. |
  ```

- [ ] 5. `documentation/chibi-sensei.md`.
  - **Line 3**: replace with:

    > Chibi Sensei is the Virtual Desktop app that guides you through building a training dataset for a small language model and training the model on Kaggle. You create a project, collect and clean rows, check the dataset, look at the split and the training plan, export the result as JSONL, and train on a free Kaggle GPU; the result is a GGUF file you can download. Testing and using the model inside AuraGo come in a later phase.

  - **Line 49**: after "…until measured values from real runs exist." add:

    > The plan also tells whether a run must convert the base model as well (`need_base_gguf`); once a base GGUF of the same revision and converter is stored, later runs skip it.

  - **Insert before `## Storage` (line 78)**:

    ```markdown
    ## Training on Kaggle

    Training runs on a GPU in your own Kaggle account. AuraGo prepares the data and a notebook, starts it, follows it and downloads the result.

    ### What you need

    - A Kaggle account with phone verification (Kaggle gives GPU and internet to notebooks only then) and its API token saved under Config → Kaggle (it goes into the Vault).
    - The Kaggle integration switched on with these permissions: `kaggle.enabled`, not `kaggle.read_only`, `allow_create`, `allow_remote_execute`, `allow_gpu`, `allow_internet`, `allow_download`, and the GPU of `chibi_sensei.training.machine_shape` listed in `kaggle.allowed_machine_shapes`. `allow_delete` is optional; without it the uploaded data stays on Kaggle.
    - AuraGo's network and filesystem-write permissions, a writable desktop and the Vault.
    - A ready dataset: the latest check run belongs to the current rows and found nothing blocking.
    - Acceptance of the base model's license where it requires one (Gemma, LFM).
    - Enough weekly GPU quota and no other Kaggle GPU run of AuraGo (the agent's or the Kaggle app's); `kaggle.max_remote_jobs` (default 1) is the limit. A job in state `unknown` keeps its slot until an administrator releases it in the Kaggle app.

    ### Preflight and the enable button

    The Training step lists every requirement, grouped by area. A red item blocks the start, a note only warns. An administrator sees **Enable for Chibi Sensei**: a dialog lists each Kaggle setting it will switch on, and confirming saves exactly those to `config.yaml` (Kaggle on, read-only off, the permissions on, the machine shape added to the allow-list). Others see what to ask the administrator for. Nothing is switched on implicitly. The connection test, the quota and the job list are read at most every 20 seconds; a start always reads them fresh.

    ### What leaves the machine

    Data leaves only after you confirm the start dialog. It names the private dataset `<you>/chibi-data-<run>` and the private notebook `<you>/chibi-<project>-<run>`, the row count and size, the license, the estimated time, the time limit, the remaining quota and what happens afterwards. The confirmation is valid for 15 minutes and for exactly that plan: if rows, settings or the profile change, prepare the start again.

    - To the dataset: the frozen train, validation and evaluation files and a small marker file.
    - To the notebook: the trainer script and `job.json` (profile, hyper-parameters, pinned versions). No AuraGo secret, host path or Hugging Face token.
    - The notebook itself downloads the base model from Hugging Face at a pinned revision, pinned pip packages and the pinned llama.cpp converter (checked by sha256).

    ### During the run

    The monitor shows the run's phase (prepare, upload, start, train, download, cleanup) and the trainer's stages. When Kaggle serves the notebook log while it runs, AuraGo reads it every minute and draws the loss chart live; otherwise the numbers appear when the run ends. AuraGo's Kaggle operations show up in the Kaggle app's job list under `chibi-` names. A restart of AuraGo resumes the run; nothing is sent twice.

    ### Cancel

    - Before the notebook started, the run stops locally and the uploaded data is cleaned up.
    - After that, AuraGo tries once to stop the session. Kaggle normally does not allow that through the API, so the run shows **Stop on Kaggle** with the notebook link. GPU time ends when you stop the notebook there, and at the latest at the run's time limit. AuraGo keeps following the notebook, ends the run as cancelled and downloads nothing but the log tail.
    - Cancel also works while the desktop is read-only.

    ### Results

    A successful run stores `data/chibi_sensei/models/<run>/model-q8_0.gguf` (q8_0) and, for LoRA, the adapter, and shows a report card (loss before and after, and per task exact match, label accuracy or perplexity on the evaluation rows). The run's files list offers each file as a download. Deleting a finished run removes its files and records; it never touches Kaggle. With `delete_remote_after_run` and `allow_delete`, AuraGo deletes the Kaggle dataset and notebook after the run; otherwise, or when a delete fails, the run lists them as leftovers to delete on Kaggle.

    ### Settings

    | Key | Default | Meaning |
    |---|---|---|
    | `chibi_sensei.training.machine_shape` | `NvidiaTeslaT4` | Kaggle GPU (`NvidiaTeslaT4` or `NvidiaL4`); must also be in `kaggle.allowed_machine_shapes` |
    | `chibi_sensei.training.delete_remote_after_run` | `true` | delete dataset and notebook on Kaggle after the run (needs `kaggle.allow_delete`) |
    | `kaggle.remote_timeout_minutes` | `60` | upper limit of a run's session; the preflight warns below 1.5 × estimate + 15 |
    | `kaggle.max_transfer_mb`, `kaggle.max_storage_mb` | `2048`, `10240` | largest single download, and the Kaggle staging storage |
    | `kaggle.max_remote_jobs` | `1` | Kaggle GPU runs at the same time (all of AuraGo) |
    | `chibi_sensei.storage.max_gb` | `20` | database plus `data/chibi_sensei/` |

    ### When a run fails

    | Code | Meaning | What to do |
    |---|---|---|
    | `env_no_gpu`, `env_unsupported_gpu` | the notebook got no usable GPU | check the machine shape and the allow-list; try again later |
    | `env_no_internet` | the notebook had no internet | verify the Kaggle account by phone |
    | `install_failed`, `base_download_failed` | pip or the model download failed | usually temporary: start again |
    | `data_changed`, `data_mismatch` | the data changed after the start | start again |
    | `oom` | GPU memory ran out | preset `quick` or a smaller `max_seq_len` |
    | `train_diverged` | the loss became NaN | lower the learning rate |
    | `lora_targets_missing`, `merge_failed`, `gguf_failed`, `gguf_arch_mismatch` | the model could not be adapted, saved or converted | report it with the log tail; try another profile |
    | `kaggle_submit_unknown` | AuraGo cannot tell whether the notebook started | look at the notebook on Kaggle and stop it if it runs |
    | `remote_timeout` | the session passed its time limit | stop it on Kaggle; raise `kaggle.remote_timeout_minutes` or use `quick` |
    | `status_missing`, `manifest_invalid`, `trainer_failed` | the run left no usable result | read the log tail |
    | `download_failed`, `checksum_mismatch`, `artifact_too_large`, `storage_full` | a result could not be copied | check free disk, `kaggle.max_transfer_mb` and `chibi_sensei.storage.max_gb`; start again |
    | `upload_failed`, `dataset_failed`, `dataset_timeout`, `submit_failed`, `kaggle_slot_busy` | Kaggle refused a step | check the Kaggle app's job list and the quota; wait for other GPU runs |
    | `kaggle_unavailable`, `desktop_readonly` | Kaggle or the read-only desktop refused AuraGo for six hours | fix the Kaggle settings or make the desktop writable |
    | `profile_changed`, `packaging_failed`, `internal_error` | AuraGo could not build or continue the run | start again; report it with the log |
    | `cancelled_by_user` | you cancelled | – |

    Warnings keep the run successful: `time_budget` (training stopped at the time budget; the partial result is kept), `base_gguf_skipped` and `base_gguf_invalid` (no base GGUF this time).
    ```

  - **`## Storage` (line 80)**: replace the paragraph with:

    > The dataset lives in `data/chibi_sensei.db` (SQLite), which is part of the backup inventory. Training runs add files under `data/chibi_sensei/`: `runs/<run>/` (the frozen split files, the notebook sent to Kaggle, status, manifest, metrics, evaluation samples and the log tail), `models/<run>/` (GGUF and adapter) and `base/` (one GGUF per base model, revision and converter, shared by all runs). `chibi_sensei.storage.max_gb` (default 20) caps the database (with its write-ahead log) and these files together; the preflight checks free disk and the cap against 1.2 × the expected download. **`data/chibi_sensei/` is not part of AuraGo backups: copy a model you want to keep.** Deleting a project removes its rows, history, documents, checks, jobs, suggestions and runs with their files, and is refused while one of its runs is active. Deleting a finished run removes its files and records; base GGUFs stay. Neither touches Kaggle.

  - **`## Privacy and security` (line 102)**: replace "In P1 nothing leaves the machine except the calls to the configured generator route during generation and cleaning." with "Without a training run nothing leaves the machine except the calls to the configured generator route during generation and cleaning." Keep the rest of the paragraph, and append:

    > A training run sends the frozen split files to a private Kaggle dataset and the trainer to a private Kaggle notebook, only after you confirmed the start (see [Training on Kaggle](#training-on-kaggle)). The Kaggle token stays in the Vault; no secret, host path or Hugging Face token enters `job.json`, the notebook, the logs or the events, and Kaggle log text is scrubbed before AuraGo stores it. Result files get names AuraGo generates and are kept only when size and sha256 match the run's manifest.

- [ ] 6. Configuration texts:
  - `documentation/configuration.md:611` becomes:

    > The Chibi Sensei training studio: dataset studio and training runs on Kaggle. Generation and cleaning call the configured generator route; training needs the Kaggle integration and the `training` settings; see [Chibi Sensei](chibi-sensei.md#training-on-kaggle).

  - `documentation/manuals/en/07-configuration.md:1344` becomes:

    > `chibi_sensei` controls the **Chibi Sensei** desktop app, a guided studio for fine-tuning small models: projects, rows, imports, checks, JSONL export and training runs on Kaggle. Generation and cleaning call the configured generator route; training uses the Kaggle integration (`kaggle.*`, including `allowed_machine_shapes`); see the [Chibi Sensei guide](../../chibi-sensei.md#training-on-kaggle).

  - `documentation/manuals/de/07-configuration.md:906` becomes:

    > `chibi_sensei` steuert die Desktop-App **Chibi Sensei**, ein geführtes Studio zum Feintuning kleiner Modelle: Projekte, Zeilen, Importe, Prüfungen, JSONL-Export und Trainingsläufe auf Kaggle. Generieren und Bereinigen rufen die konfigurierte Generator-Route auf; das Training nutzt die Kaggle-Integration (`kaggle.*`, auch `allowed_machine_shapes`); siehe das [Chibi-Sensei-Handbuch](../../chibi-sensei.md#training-on-kaggle).

- [ ] 7. API references.
  - **English** (`21-api-reference.md`):
    - Line 1636: replace "The Chibi Sensei training studio (datasets for small language models). Reads need `desktop:read`, writes `desktop:write`; a job cancel is a stop operation…" with "The Chibi Sensei training studio (datasets for small language models and their training on Kaggle). Reads need `desktop:read`, writes `desktop:write`; a job or run cancel is a stop operation…". Keep the rest.
    - Change the events line to `# server-sent events: job, rows, checks, run, metric, stage`.
    - Insert before the closing fence:

      ```
      GET  /api/desktop/chibi-sensei/projects/{id}/preflight?preset=   # training requirements; fix.flags names what an admin can enable
      POST /api/desktop/chibi-sensei/preflight/enable                   # admin; {"flags":[...]} switches Kaggle settings on by name
      POST /api/desktop/chibi-sensei/consents                           # 201; {"profile_id"} records the base-model license acceptance
      POST /api/desktop/chibi-sensei/projects/{id}/runs/prepare         # {"preset"}; confirmation data and a 15-minute plan token
      POST /api/desktop/chibi-sensei/projects/{id}/runs                 # 202; {"plan_token"} starts the Kaggle run; GET lists runs
      GET  /api/desktop/chibi-sensei/runs/{runId}                       # one run; .../metrics, .../events, .../artifacts, .../report, .../log
      GET  /api/desktop/chibi-sensei/runs/{runId}/artifacts/{aid}/file  # the file as an attachment (Range supported)
      POST /api/desktop/chibi-sensei/runs/{runId}/cancel                # stop operation, also on a read-only desktop
      POST /api/desktop/chibi-sensei/runs/{runId}/delete                # 204; finished runs only; local files and rows, not Kaggle
      ```

    - Append to the errors paragraph:

      > Training answers 409 `preflight_failed`, `run_active` or `license_consent_required` with `missing` (the failing requirement IDs), 409 `plan_expired` when the plan token is unknown, used, older than 15 minutes or the plan changed, 409 `run_not_terminal`, 404 `run_not_found` and `artifact_not_found`, and 403 `admin_required`. The training POST routes require a same-origin `Origin` header from browsers (`csrf_check_failed`).

  - **German** (`21-api-reference.md`):
    - Line 1645: "…(Datensätze für kleine Sprachmodelle und ihr Training auf Kaggle)…; ein Job- oder Lauf-Abbruch ist eine Stopp-Operation…".
    - Events line: `# Server-Sent Events: job, rows, checks, run, metric, stage`.
    - Lines before the fence:

      ```
      GET  /api/desktop/chibi-sensei/projects/{id}/preflight?preset=   # Voraussetzungen fürs Training; fix.flags nennt, was ein Admin einschalten kann
      POST /api/desktop/chibi-sensei/preflight/enable                   # nur Admin; {"flags":[...]} schaltet Kaggle-Einstellungen per Name ein
      POST /api/desktop/chibi-sensei/consents                           # 201; {"profile_id"} speichert die Zustimmung zur Lizenz des Basismodells
      POST /api/desktop/chibi-sensei/projects/{id}/runs/prepare         # {"preset"}; Daten für die Bestätigung und ein Plan-Token (15 Minuten)
      POST /api/desktop/chibi-sensei/projects/{id}/runs                 # 202; {"plan_token"} startet den Kaggle-Lauf; GET listet die Läufe
      GET  /api/desktop/chibi-sensei/runs/{runId}                       # ein Lauf; .../metrics, .../events, .../artifacts, .../report, .../log
      GET  /api/desktop/chibi-sensei/runs/{runId}/artifacts/{aid}/file  # die Datei als Download (Range möglich)
      POST /api/desktop/chibi-sensei/runs/{runId}/cancel                # Stopp-Operation, auch auf einem schreibgeschützten Desktop
      POST /api/desktop/chibi-sensei/runs/{runId}/delete                # 204; nur beendete Läufe; lokale Dateien und Zeilen, nicht Kaggle
      ```

    - Append to the errors paragraph:

      > Das Training antwortet mit 409 `preflight_failed`, `run_active` oder `license_consent_required` samt `missing` (die IDs der fehlenden Voraussetzungen), mit 409 `plan_expired`, wenn das Plan-Token unbekannt, schon benutzt oder älter als 15 Minuten ist oder sich der Plan geändert hat, mit 409 `run_not_terminal`, 404 `run_not_found` und `artifact_not_found` sowie 403 `admin_required`. Die POST-Routen des Trainings verlangen von Browsern einen passenden `Origin`-Header (`csrf_check_failed`).

- [ ] 8. DOX closeout for the report:
  - Changed guides: `internal/chibisensei/AGENTS.md`, `internal/server/AGENTS.md`, `internal/kaggle/AGENTS.md`, root `AGENTS.md`.
  - Intentionally unchanged:
    - `internal/desktop/AGENTS.md`: no desktop-package change;
    - `internal/httpstream/AGENTS.md`: the package is unchanged and the artifact route keeps a finite deadline;
    - `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `scripts/AGENTS.md`: F4 owns them;
    - `internal/config` (no guide): B1 documented its fields.
  - Remind the coordinator that the four `AGENTS.md` files are ignored by `.gitignore` but tracked, and must be staged with `git add -f`.

- [ ] 9. Verify:

```bash
git diff --check
grep -n "P1 does not train" documentation/chibi-sensei.md || echo "P1 statement removed"
grep -n "In P1 nothing leaves" documentation/chibi-sensei.md || echo "privacy statement updated"
go build ./...
go test ./internal/server -count=1 -run '(?i)(ChibiSensei|DesktopReadonlyCleanupRoutes)' -timeout 10m
go test ./internal/chibisensei/... -count=1
```

  Expected: no `git diff --check` output, both `echo` lines printed, a clean build, and `ok` for `aurago/internal/server`, `aurago/internal/chibisensei` and `aurago/internal/chibisensei/trainer`.
