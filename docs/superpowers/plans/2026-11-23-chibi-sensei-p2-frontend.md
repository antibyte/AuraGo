# Chibi Sensei P2 frontend plan

## Contract changes

1. **`GET /meta` profiles gain `methods` (string array) and `max_seq_cap` (int).** The overrides form needs them: it shows a method choice only where a profile allows two methods (Gemma: `["full","lora"]`), and it lists only the sequence buckets up to the cap. `ProfileSummary` in `internal/chibisensei/profiles.go:112-129` has neither field today. The fields are additive; the owner is the batch that touches `profiles.go` (B2), plus the meta wiring (B5). If they are missing, the UI hides the method select and offers the buckets up to `min(4096, context_max)`. The server's 422 for `overrides.*` then catches any wrong value, and it is shown inline.
2. **`Run` and `RunSummary` gain `last_state`**: the last non-terminal state the run was in. It equals `state` while the run is active. The timeline needs it to mark the phase where a failed or cancelled run stopped (B3 keeps it in `remote_json`; B4 sets it on every transition into a non-terminal state). Without it, the UI guesses from `error_code`, `stage`, `remote_started` and `remote.dataset`/`remote.notebook` (`reachedPhase` in F3).
3. **`run_events.data` fields the UI reads** (B3/B4 should emit exactly these):
   - `state` → `{to}`
   - `stage` → `{stage, state}`
   - `warn` → `{code, message}`
   - `error` → `{stage, code, message}`
   - `kaggle` → `{step, job_id}`
   - `log` → `{text}`

   Any other shape is skipped, never shown raw.
4. **Path ownership shifts so that every batch can be verified:**
   - F2 also edits `scripts/test-chibi-sensei-model.mjs`. The new `primaryFor('training')` breaks its lines 63-66 and 70.
   - F3 also edits these, because `TestChibiSenseiSourcesAreSafeAndSmall` (`len(sources) != 10`) and `TestChibiSenseiLoaderListsEveryScript` fail the moment `chibi-sensei-run.js` and `chibi-sensei-chart.js` exist:
     - `scripts/test-chibi-sensei-model.mjs`
     - `ui/js/desktop/core/module-loader.js`
     - `ui/desktop_chibi_sensei_guards_test.go`
     - `ui/desktop_chibi_sensei_registration_test.go`
     - `ui/i18n_lint_test.go` (`desktopLocalI18nHelpers`)
   - F4 keeps the 14 locales, the config and Kaggle test markers, the guides and the full final run.
5. **Two tests fail by design from F2 until F4 adds the 14 locales:** `TestChibiSenseiLocalesCoverAllSixteenLanguages` and `TestTranslations_AllKeysPresentInAllLanguages`. F2 and F3 run the UI tests with `-skip` for both, then run them once more and expect only "lacks <new key>" lines.
6. **Artifact downloads** are plain `<a download href="/api/desktop/chibi-sensei/runs/{id}/artifacts/{aid}/file">` anchors. They are same-origin, need no fetch, and work on a read-only desktop. If the backend plan leaves this route out (the contract allows that), the coordinator tells F3, which then shows file names and sizes without the anchors.
7. **Fact-sheet correction:** `desktopLocalI18nHelpers` is real (`ui/i18n_lint_test.go:1113-1122`); the fact sheet only searched `ui/js`. The guide sentence in `ui/js/desktop/apps/AGENTS.md:115` is right and stays. The two new modules must be registered there (F3).

## File map

| File | Batch | Responsibility |
|---|---|---|
| `ui/cfg/kaggle.js` (+18 lines) | F1 | machine-shape checkbox list and hidden array field |
| `ui/cfg/chibi_sensei.js` (+20) | F1 | Training group: machine-shape select, delete toggle, defaults |
| `ui/lang/kaggle/*.json` (16) | F1 | 4 new keys |
| `ui/lang/config/chibi_sensei/*.json` (16) | F1 | 7 new keys, 2 changed |
| `ui/js/desktop/apps/chibi-sensei/chibi-sensei-core.js` (352 → ≈450) | F2 | run state helpers, `format.bytes/duration/decimal`, `safeLink/openLink/linkRow`, 9 error codes |
| `ui/js/desktop/apps/chibi-sensei/chibi-sensei-api.js` (203 → ≈250) | F2 | one method per new route, `artifactURL`, SSE `run`/`metric`/`stage` |
| `ui/js/desktop/apps/chibi-sensei/chibi-sensei-mascot.js` (73 → 75) | F2 | `moodFor` run inputs |
| `ui/js/desktop/apps/chibi-sensei/chibi-sensei-steps.js` (187 → ≈215) | F2 | training primary action, tips, panel mood |
| `ui/js/desktop/apps/chibi-sensei/chibi-sensei-training.js` (162 → ≈950) | F2 (F3 mounts the run area) | preflight, enable dialog, presets, overrides, license, confirmation |
| `ui/js/desktop/apps/chibi-sensei/chibi-sensei.js` (520 → ≈610) | F2, F3 | run list state, `applyRun`, primary actions; F3: SSE fan-out, run poll |
| `ui/js/desktop/apps/chibi-sensei/chibi-sensei-chart.js` (new, ≈200) | F3 | pure SVG loss chart, `layout` |
| `ui/js/desktop/apps/chibi-sensei/chibi-sensei-run.js` (new, ≈820) | F3 | run monitor, result, files, leftovers, history |
| `ui/css/desktop-app-chibi-sensei.css` (300 → ≈420) | F2, F3 | styles |
| `ui/lang/chibi_sensei/en.json`, `de.json` | F2, F3 | keys of the fragments in Task F4 |
| `scripts/test-chibi-sensei-model.mjs` | F2, F3 | pure-logic checks |
| `ui/js/desktop/core/module-loader.js:39` | F3 | two new scripts |
| `ui/desktop_chibi_sensei_guards_test.go:49-50`, `ui/desktop_chibi_sensei_registration_test.go:13`, `ui/i18n_lint_test.go:1121` | F3 | module count, order, helper registry |
| `ui/lang/chibi_sensei/{cs,da,el,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | F4 | translations |
| `ui/desktop_chibi_sensei_i18n_test.go`, `ui/config_chibi_sensei_test.go`, `ui/kaggle_locales_test.go` | F4 | `same` list, config and Kaggle markers |
| `ui/AGENTS.md:130`, `ui/js/desktop/apps/AGENTS.md:111-116`, `scripts/AGENTS.md` (Chibi line) | F4 | DOX |

Conventions for all four tasks:
- Work only in the worktree.
- Go commands run in Git Bash from the worktree root.
- Write real UTF-8 text (no `\u` escapes).
- Follow the UI rules of the brief:
  - `textContent` only, no `innerHTML`, no backticks, no browser dialogs, no storage;
  - classes start with `csn-`, and a class name never contains `consent`, `cookie`, `ad-` or `sponsor`;
  - `fetch` and `EventSource` only in `-api.js`;
  - no `http(s)://host` literal in any app source, comments included (`TestChibiSenseiAppLoadsNothingExternalAndAvoidsFilteredClasses`).
- Before testing, run `git diff --check`.

---

### Task F1: Config forms

**Files**
- Modify:
  - `ui/cfg/kaggle.js:1` (constant), `:20` (field after `allowed_owners`), `:26` (wiring after `attachChangeListeners()`)
  - `ui/cfg/chibi_sensei.js:1-5` (header), `:9-15` (defaults), `:68-77` (new group before the `after` note)
  - `ui/lang/kaggle/*.json` (16)
  - `ui/lang/config/chibi_sensei/*.json` (16)
- `ui/js/config/catalog.js` stays unchanged. There is no numeric rule to add; the server normalizes both values.

**Existing symbols modified:** `renderKaggleSection`, `renderChibiSenseiSection`, `CHIBI_SENSEI_DEFAULTS`.

**Steps**
- [ ] 1. `ui/cfg/kaggle.js`. Insert after line 1:
```js
// The machine shapes a Kaggle run may request; equal to config.KaggleMachineShapes (NormalizeKaggleConfig keeps only these).
const KAGGLE_MACHINE_SHAPES = ['NvidiaTeslaT4', 'NvidiaL4'];
```
  Insert after the `allowed_owners` field (the `html +=` statement ending at line 20), before the numbers grid. This follows the precedent of `cfg/newspaper.js:22-40`: native checkboxes carry `data-cfg-choice` and no `data-path`, and one hidden `data-type="array"` input carries the saved value. `state.js` `controlValue` (:167-172) splits it on commas.
```js
    const shapes = Array.isArray(cfg.allowed_machine_shapes) ? cfg.allowed_machine_shapes.filter(shape => KAGGLE_MACHINE_SHAPES.includes(shape)) : [];
    html += '<div class="field-group" role="group" aria-labelledby="kaggle-shapes-label" aria-describedby="kaggle-shapes-help"><div class="field-label" id="kaggle-shapes-label">' + label('allowed_machine_shapes') + '</div><span class="field-help" id="kaggle-shapes-help">' + label('allowed_machine_shapes_help') + '</span><div class="cfg-choice-items">';
    for (const shape of KAGGLE_MACHINE_SHAPES) html += '<label class="cfg-choice-item"><input type="checkbox" data-cfg-choice data-kaggle-shape="' + shape + '"' + (shapes.includes(shape) ? ' checked' : '') + '><span>' + label('machine_shape_' + shape) + '</span></label>';
    html += '</div><input class="field-input is-hidden" type="text" data-path="kaggle.allowed_machine_shapes" data-type="array" value="' + escapeAttr(shapes.join(', ')) + '" tabindex="-1" aria-hidden="true"></div>';
```
  Insert after `attachChangeListeners();` (line 26):
```js
    const shapeInputs = [...root.querySelectorAll('[data-kaggle-shape]')];
    const shapeField = root.querySelector('[data-path="kaggle.allowed_machine_shapes"]');
    for (const input of shapeInputs) input.addEventListener('change', () => {
        shapeField.value = shapeInputs.filter(box => box.checked).map(box => box.dataset.kaggleShape).join(', ');
        shapeField.dispatchEvent(new Event('input', { bubbles: true }));
        setDirty(true);
    });
```
  - The dispatched event lets `AuraConfigState`'s document listener (`state.js:282-293`) sync the draft.
  - `markDirty` ignores untrusted events (`main.js:3237-3241`), so `setDirty(true)` marks the draft dirty the same way `chibiCfgToggleEnabled` does.
- [ ] 2. `ui/cfg/chibi_sensei.js`:
  - Header: replace "(phase P1)" with "(phases P1 and P2)" and "Agent, training and runtime keys stay hidden" with "Agent and runtime keys stay hidden".
  - Defaults: add to `CHIBI_SENSEI_DEFAULTS` after `generator`:
```js
    training: Object.freeze({ machine_shape: 'NvidiaTeslaT4', delete_remote_after_run: true })
```
  - Add after the defaults: `const CHIBI_SENSEI_MACHINE_SHAPES = Object.freeze(['NvidiaTeslaT4', 'NvidiaL4']);`
  - In the `else` branch, after the generator group push:
```js
        groups.push({ title: t('config.chibi_sensei.training_title'), fields: [
            form.select({ label: t('config.chibi_sensei.training_machine_shape_label'), help: tOr('config.chibi_sensei.training_machine_shape_help', ''),
                path: 'chibi_sensei.training.machine_shape', value: data.training.machine_shape,
                options: CHIBI_SENSEI_MACHINE_SHAPES.map(value => ({ value, label: t('config.chibi_sensei.machine_shape_' + value) })) }),
            chibiCfgToggleRow('config.chibi_sensei.training_delete_remote_label', 'config.chibi_sensei.training_delete_remote_help',
                data.training.delete_remote_after_run === true, 'chibi_sensei.training.delete_remote_after_run', 'toggleBool(this)')
        ] });
```
  `chibiCfgFill` already fills a missing boolean with the default `true`.
- [ ] 3. Locales: add the keys below, after `config.kaggle.remote_timeout_help` in `lang/kaggle/*.json` and before `config.chibi_sensei.later_note` in `lang/config/chibi_sensei/*.json`, with the same key order in all 16 files. The F1 implementer translates the 14 other languages from the English text: real translations with the same placeholders (none here). GPU product names stay, the words around them are translated.

  `ui/lang/kaggle/en.json`:
```json
  "config.kaggle.allowed_machine_shapes": "Allowed GPU types",
  "config.kaggle.allowed_machine_shapes_help": "Kaggle runs may request only the GPU types ticked here. With none ticked, a run that asks for a specific GPU type is refused.",
  "config.kaggle.machine_shape_NvidiaTeslaT4": "NVIDIA T4 graphics card (16 GB)",
  "config.kaggle.machine_shape_NvidiaL4": "NVIDIA L4 graphics card (24 GB)"
```
  `ui/lang/kaggle/de.json`:
```json
  "config.kaggle.allowed_machine_shapes": "Erlaubte GPU-Typen",
  "config.kaggle.allowed_machine_shapes_help": "Kaggle-Läufe dürfen nur die hier angehakten GPU-Typen anfordern. Ist keiner angehakt, wird ein Lauf abgelehnt, der einen bestimmten GPU-Typ verlangt.",
  "config.kaggle.machine_shape_NvidiaTeslaT4": "NVIDIA-T4-Grafikkarte (16 GB)",
  "config.kaggle.machine_shape_NvidiaL4": "NVIDIA-L4-Grafikkarte (24 GB)"
```
  `ui/lang/config/chibi_sensei/en.json` (new keys, plus two changed values):
```json
  "config.chibi_sensei.training_title": "Training on Kaggle",
  "config.chibi_sensei.training_machine_shape_label": "GPU type",
  "config.chibi_sensei.training_machine_shape_help": "The graphics card Kaggle uses for training. It must also be ticked under “Allowed GPU types” in the Kaggle settings.",
  "config.chibi_sensei.machine_shape_NvidiaTeslaT4": "NVIDIA T4 graphics card (16 GB)",
  "config.chibi_sensei.machine_shape_NvidiaL4": "NVIDIA L4 graphics card (24 GB)",
  "config.chibi_sensei.training_delete_remote_label": "Delete the Kaggle copies after a run",
  "config.chibi_sensei.training_delete_remote_help": "After a run AuraGo deletes the uploaded dataset and the notebook on Kaggle. This needs the delete permission in the Kaggle settings; without it both stay on Kaggle and the run lists them.",
  "help.chibi_sensei.enabled": "Shows the Chibi Sensei app on the desktop. The dataset studio works locally; training runs on Kaggle and needs the Kaggle permissions.",
  "config.chibi_sensei.later_note": "Local testing and the Sensei chat get their settings in later versions."
```
  `de.json`:
```json
  "config.chibi_sensei.training_title": "Training auf Kaggle",
  "config.chibi_sensei.training_machine_shape_label": "GPU-Typ",
  "config.chibi_sensei.training_machine_shape_help": "Die Grafikkarte, die Kaggle fürs Training nutzt. Sie muss auch in den Kaggle-Einstellungen unter „Erlaubte GPU-Typen“ angehakt sein.",
  "config.chibi_sensei.machine_shape_NvidiaTeslaT4": "NVIDIA-T4-Grafikkarte (16 GB)",
  "config.chibi_sensei.machine_shape_NvidiaL4": "NVIDIA-L4-Grafikkarte (24 GB)",
  "config.chibi_sensei.training_delete_remote_label": "Kaggle-Kopien nach einem Lauf löschen",
  "config.chibi_sensei.training_delete_remote_help": "Nach einem Lauf löscht AuraGo den hochgeladenen Datensatz und das Notebook auf Kaggle. Dafür braucht AuraGo in den Kaggle-Einstellungen die Erlaubnis zum Löschen; sonst bleiben beide auf Kaggle und der Lauf listet sie auf.",
  "help.chibi_sensei.enabled": "Zeigt die Chibi-Sensei-App auf dem Desktop. Das Datensatz-Studio arbeitet lokal; das Training läuft auf Kaggle und braucht die Kaggle-Berechtigungen.",
  "config.chibi_sensei.later_note": "Lokales Testen und der Sensei-Chat bekommen ihre Einstellungen in späteren Versionen."
```
- [ ] 4. Tests that pin these files:
  - `ui/config_chibi_sensei_test.go` (`TestConfigChibiSenseiSection`) checks the P1 markers, key parity in all 16 config locales and German "Du". F1 keeps every existing marker string intact, in particular `window.AuraConfigState.set('chibi_sensei.enabled', next)` and the exact range call strings.
  - `ui/kaggle_locales_test.go` (`TestKaggleLocalesComplete`) requires equal key counts in all 16 Kaggle locales.
  - `ui/kaggle_limits_test.go` is untouched (numbers only).
  - `TestTranslations_ConfigCfgJSKeysExist` and `TestTranslations_AllKeysPresentInAllLanguages` cover the new literal `t('config.…')` keys.
  - New markers for both files come in F4.
- [ ] 5. Verify:
```bash
node --check ui/cfg/kaggle.js && node --check ui/cfg/chibi_sensei.js
node -e "for (const f of process.argv.slice(1)) JSON.parse(require('fs').readFileSync(f,'utf8'))" ui/lang/kaggle/*.json ui/lang/config/chibi_sensei/*.json
go build ./...
go test ./ui -run '(?i)(Kaggle|ConfigChibiSensei|Translations_)' -count=1
```
  Expected: no output from `node`; `ok aurago/ui` (the browser tests skip without the opt-in).

---

### Task F2: Training step, part 1

**Files**
- Modify:
  - `chibi-sensei-core.js:194-222` (format), `:256` (ERROR_CODES), `:330-351` (exports)
  - `chibi-sensei-api.js:169-196`
  - `chibi-sensei-mascot.js:61-70`
  - `chibi-sensei-steps.js:62-84`, `:156-176`
  - `chibi-sensei-training.js` (rewrite of `create`; `createLater` unchanged)
  - `chibi-sensei.js:14-46`, `:173-236`, `:361-387`
  - `ui/css/desktop-app-chibi-sensei.css` (append)
  - `ui/lang/chibi_sensei/en.json`, `de.json` (families marked F2 in Task F4)
  - `scripts/test-chibi-sensei-model.mjs`

**Existing symbols modified:** `format`, `ERROR_CODES`, `createApi` (api object, `events`), `moodFor`, `primaryFor`, `TIPS`, `createPanel`, `create` (training), `createInstance`, `closeProject`, `openProject`, `recompute`, `runPrimary`.

**Steps**
- [ ] 1. `chibi-sensei-core.js`:
  - Add three formatters to `format`. They use Intl units, so no unit keys are needed.
  - Add run helpers and the link helpers before `P.el = el;`, and export them.
  - Append to `ERROR_CODES`: `'preflight_failed', 'plan_expired', 'run_active', 'run_not_terminal', 'run_not_found', 'artifact_not_found', 'admin_required', 'kaggle_unavailable', 'license_consent_required'`. `TestChibiSenseiAppUsesOnlyKeysItHas` then requires `chibi_sensei.error.<code>`.
```js
        // seconds as "12 s", "4 min" or "1 h 5 min" in the user's language
        duration(seconds) {
            const s = Math.max(0, Math.round(Number(seconds) || 0));
            const unit = (value, name) => {
                try { return new Intl.NumberFormat(locale(), { style: 'unit', unit: name, unitDisplay: 'short' }).format(value); } catch (_) { return String(value) + ' ' + name; }
            };
            if (s < 60) return unit(s, 'second');
            const m = Math.floor(s / 60);
            return m < 60 ? unit(m, 'minute') : unit(Math.floor(m / 60), 'hour') + ' ' + unit(m % 60, 'minute');
        },
        bytes(n) {
            const units = ['byte', 'kilobyte', 'megabyte', 'gigabyte'];
            let x = Math.max(0, Number(n) || 0), i = 0;
            while (x >= 1024 && i < units.length - 1) { x /= 1024; i++; }
            try { return new Intl.NumberFormat(locale(), { style: 'unit', unit: units[i], unitDisplay: 'short', maximumFractionDigits: i ? 1 : 0 }).format(x); } catch (_) { return Math.round(x) + ' ' + units[i]; }
        },
        decimal(n, digits) {
            const v = Number(n);
            if (!Number.isFinite(v)) return '0';
            try { return new Intl.NumberFormat(locale(), { maximumFractionDigits: digits == null ? 3 : digits }).format(v); } catch (_) { return String(v); }
        }
```
```js
    // ---- runs ------------------------------------------------------------------------------------------------------------------
    const RUN_STATES = ['queued', 'packaging', 'uploading', 'creating_dataset', 'waiting_dataset', 'starting', 'running', 'downloading', 'verifying', 'cleaning', 'succeeded', 'failed', 'cancelled'];
    const RUN_TERMINAL = ['succeeded', 'failed', 'cancelled'];
    function runTerminal(state) { return RUN_TERMINAL.indexOf(state) >= 0; }
    // runs are newest first; an unknown state is neither active nor terminal, so it never keeps a poll alive.
    function activeRun(runs) {
        return (Array.isArray(runs) ? runs : []).find(r => r && RUN_STATES.indexOf(r.state) >= 0 && !runTerminal(r.state)) || null;
    }
    // upsertRun merges a (possibly partial) run into the list: fields the update lacks are kept, the list is newest first.
    function upsertRun(runs, run, limit) {
        const list = Array.isArray(runs) ? runs : [];
        const prev = list.find(r => r && r.id === run.id);
        const next = list.filter(r => r && r.id !== run.id);
        next.push(prev ? Object.assign({}, prev, run) : run);
        next.sort((a, b) => String(b.created || '').localeCompare(String(a.created || '')));
        return next.slice(0, limit || 20);
    }

    // ---- links: never an embedded outside resource; opened on demand in a new tab without referrer -----------------------
    function safeLink(url) {
        const s = String(url || '').trim();
        if (s.charAt(0) === '/' && s.charAt(1) !== '/' && s.charAt(1) !== '\\') return s;
        try { const u = new URL(s); return u.protocol === 'https:' ? u.href : ''; } catch (_) { return ''; }
    }
    function openLink(url) {
        const safe = safeLink(url);
        if (safe) window.open(safe, '_blank', 'noopener,noreferrer');
        return !!safe;
    }
    // The address as selectable read-only text, a copy button and an open button (disabled for anything but https or a path).
    function linkRow(t, url, name) {
        const safe = safeLink(url);
        const input = el('input', { class: 'csn-input csn-link-text', type: 'text', readonly: true, value: String(url || ''), ariaLabel: t('chibi_sensei.link.address'), dataset: { csn: name + '-url' } });
        const note = el('span', { class: 'csn-hint', role: 'status', dataset: { csn: name + '-copied' } });
        const copy = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: name + '-copy' }, text: t('chibi_sensei.link.copy'), onClick: async () => {
            let ok = false;
            try { if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') { await navigator.clipboard.writeText(input.value); ok = true; } } catch (_) { ok = false; }
            if (!ok) { input.focus(); input.select(); }
            note.textContent = t(ok ? 'chibi_sensei.link.copied' : 'chibi_sensei.link.copy_failed');
        } });
        const open = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: name + '-open' }, text: t('chibi_sensei.link.open'), onClick: () => openLink(url) });
        open.disabled = !safe;
        open.title = safe ? '' : t('chibi_sensei.link.blocked');
        return el('div', { class: 'csn-linkrow', dataset: { csn: name } }, input, copy, open, note);
    }
```
  Exports: `P.RUN_STATES = RUN_STATES; P.runTerminal = runTerminal; P.activeRun = activeRun; P.upsertRun = upsertRun; P.safeLink = safeLink; P.openLink = openLink; P.linkRow = linkRow;`.
- [ ] 2. `chibi-sensei-api.js`. Add these methods after `plan`:
```js
            // {ready, admin, preset, checked_at, items, fix}
            preflight(id, preset) { return json('GET', project(id) + '/preflight' + qs({ preset })); },
            // {applied}; the server maps each flag name to its value
            enablePreflight(flags) { return send('POST', '/preflight/enable', { flags }); },
            async consent(profileId) { return (await send('POST', '/consents', { profile_id: profileId })).consent; },
            // {plan_token, expires_in_s, run_id, plan, destinations, data, license, time, cleanup}
            prepareRun(id, preset) { return send('POST', project(id) + '/runs/prepare', { preset }); },
            async startRun(id, planToken) { return (await send('POST', project(id) + '/runs', { plan_token: planToken })).run; },
            async listRuns(id, limit) { return list(await json('GET', project(id) + '/runs' + qs({ limit })), 'runs'); },
            async getRun(runId) { return (await json('GET', '/runs/' + enc(runId))).run; },
            async cancelRun(runId) { return (await send('POST', '/runs/' + enc(runId) + '/cancel', {})).run; },
            // {metrics, max_steps, live}
            runMetrics(runId, sinceStep) { return json('GET', '/runs/' + enc(runId) + '/metrics' + qs({ since_step: sinceStep || 0 })); },
            async runEvents(runId, after, limit) { return list(await json('GET', '/runs/' + enc(runId) + '/events' + qs({ after: after || 0, limit })), 'events'); },
            async runArtifacts(runId) { return list(await json('GET', '/runs/' + enc(runId) + '/artifacts'), 'artifacts'); },
            artifactURL(runId, artifactId) { return BASE + '/runs/' + enc(runId) + '/artifacts/' + enc(artifactId) + '/file'; },
            async runReport(runId) { return (await json('GET', '/runs/' + enc(runId) + '/report')).report || null; },
            // {tail, bytes}
            runLog(runId) { return json('GET', '/runs/' + enc(runId) + '/log'); },
            deleteRun(runId) { return send('POST', '/runs/' + enc(runId) + '/delete', {}); },
```
  In `events()`, before the `'open'` listener:
```js
                source.addEventListener('run', event => {
                    const body = parse(event);
                    const run = body && body.run ? body.run : body;
                    if (h.onRun && run && typeof run === 'object' && typeof run.id === 'string') h.onRun(run);
                });
                source.addEventListener('metric', event => {
                    const body = parse(event);
                    if (h.onMetric && body && typeof body.run_id === 'string' && Array.isArray(body.points)) h.onMetric(body);
                });
                source.addEventListener('stage', event => {
                    const body = parse(event);
                    if (h.onStage && body && typeof body.run_id === 'string') h.onStage(body);
                });
```
  Update the comment above `events(` to `{onJob, onRows, onChecks, onRun, onMetric, onStage, onState}`.
- [ ] 3. `chibi-sensei-mascot.js` `moodFor`:
  - New priority comment: `switched off > a job or run is working > blocked or failed (job or run) > run succeeded > step ready or done > idle`.
  - `if (x.jobsActive || x.checksRunning || x.runActive) return 'thinking';`
  - `if (x.blocked || x.jobFailed || x.runFailed) return 'worried';`
- [ ] 4. `chibi-sensei-steps.js`:
  - Replace the training branch of `primaryFor`. The opts comment becomes `{readonly, goalValid, checksRunning, rows, runActive, preflight, starting}`.
```js
        if (step === 'training') {
            // While a run is active the primary action shows it; a read-only desktop may still look at (and cancel) a run.
            if (o.runActive) return { labelKey: 'chibi_sensei.primary.show_run', action: 'show_run', disabled: false, reasonKey: '' };
            let reason = '';
            if (o.readonly) reason = 'chibi_sensei.readonly_banner';
            else if (!o.preflight) reason = 'chibi_sensei.primary.preflight_loading';
            else if (!o.preflight.ready) reason = 'chibi_sensei.primary.preflight_blocked';
            const label = o.starting ? 'chibi_sensei.primary.starting' : (s.training === 'done' ? 'chibi_sensei.primary.train_again' : 'chibi_sensei.primary.start_training');
            return { labelKey: label, action: 'start_training', disabled: !!reason || !!o.starting, reasonKey: reason };
        }
```
  - `TIPS` gains `training_ready: 'chibi_sensei.tip.training_ready', training_in_progress: 'chibi_sensei.tip.training_in_progress', training_done: 'chibi_sensei.tip.training_done', training_blocked: 'chibi_sensei.tip.training_blocked'`.
  - In `createPanel.paint`, replace the `mascot.setMood(...)` call:
```js
            if (mascot) {
                const jobsActive = Array.from(inst.jobs.values()).some(job => job && ACTIVE.indexOf(job.state) >= 0);
                const runs = Array.isArray(inst.runs) ? inst.runs : [];
                const active = P.activeRun(runs);
                const latest = runs[0] || null;
                const onTraining = step === 'training' && !active && !!latest;
                mascot.setMood(P.mascot.moodFor({
                    jobsActive, runActive: !!active, checksRunning: !!inst.checksRunning,
                    blocked: derived.data === 'blocked' && step === 'data', jobFailed: !!inst.lastJobFailed,
                    runFailed: onTraining && latest.state === 'failed', runSucceeded: onTraining && latest.state === 'succeeded',
                    stepState: derived[step]
                }));
            }
```
  Mood rules: an active run makes the mascot think on every step. On the Training step a failed latest run makes it worried and a succeeded one makes it celebrate. A cancelled run is the user's choice and leaves the step-state mood.
- [ ] 5. `chibi-sensei.js` (composition root):
  - **Instance fields:** in `createInstance`, add `runs: [], preflight: null, trainingStarting: false,` and the methods `inst.applyRun = run => applyRun(inst, run); inst.reloadRuns = () => reloadRuns(inst); inst.removeRun = id => removeRun(inst, id);`. Add the constants `const RUN_LIST_LIMIT = 20;` and `const RUN_END_KEYS = { succeeded: 'chibi_sensei.run.finished_notify', failed: 'chibi_sensei.run.failed_notify', cancelled: 'chibi_sensei.run.cancelled_notify' };`.
  - **Closing a project:** `closeProject` resets `inst.runs = []; inst.preflight = null; inst.trainingStarting = false;`.
  - **Opening a project:** `openProject` adds `tolerant(inst.api.listRuns(id, RUN_LIST_LIMIT))` as the sixth entry. It sets `inst.runs = Array.isArray(runs) ? runs.slice(0, RUN_LIST_LIMIT) : [];` next to the jobs.
  - **Recompute:** `recompute` passes `runActive: !!P.activeRun(inst.runs), preflight: inst.preflight, starting: inst.trainingStarting` into `primaryFor`.
  - **New functions:**
```js
    // A run from a start, an event or the poll. The step states come from the server, so a new or finished run refreshes the project.
    function applyRun(inst, run) {
        if (inst.disposed || !run || typeof run.id !== 'string' || !inst.project) return;
        if (run.project_id && run.project_id !== inst.project.id) return;
        const before = inst.runs.find(r => r.id === run.id) || null;
        inst.runs = P.upsertRun(inst.runs, run, RUN_LIST_LIMIT);
        const merged = inst.runs.find(r => r.id === run.id) || run;
        inst.bus.emit('run', merged);
        const ended = before && !P.runTerminal(before.state) && P.runTerminal(merged.state);
        if (ended) notify(inst, inst.t(RUN_END_KEYS[merged.state]), merged.state === 'failed' ? 'error' : 'info');
        if ((ended || !before) && inst.refresh) inst.refresh();
        recompute(inst);
        syncPoll(inst);
    }

    async function reloadRuns(inst) {
        if (inst.disposed || !inst.project) return;
        const seq = inst.projectSeq;
        try {
            const runs = await inst.api.listRuns(inst.project.id, RUN_LIST_LIMIT);
            if (inst.disposed || seq !== inst.projectSeq) return;
            inst.runs = runs.slice(0, RUN_LIST_LIMIT);
            inst.bus.emit('runs', inst.runs);
            recompute(inst);
            syncPoll(inst);
        } catch (_) { /* the list stays; the next event or poll brings the change */ }
    }

    function removeRun(inst, id) {
        inst.runs = inst.runs.filter(r => r.id !== id);
        inst.bus.emit('runs', inst.runs);
        recompute(inst);
        syncPoll(inst);
    }
```
  - **Polling:** in F2, `syncPoll` stays as it is (F3 adds the run).
  - **Primary actions:** in `runPrimary`, add:
```js
        } else if (id === 'start_training' || id === 'show_run') {
            const view = inst.parts.view;
            if (inst.step !== 'training') { showStep(inst, 'training'); return; }
            if (id === 'start_training' && view && typeof view.startTraining === 'function') await view.startTraining();
            else if (id === 'show_run' && view && typeof view.showRun === 'function') view.showRun();
        }
```
  - Update the file header comment: "…the 2.5 s poll of active jobs and of the active run, the run list…".
- [ ] 6. `chibi-sensei-training.js`, rewritten.

  **Header:**
```js
// Chibi Sensei training step: the preflight list (grouped requirements, block and note styling, refresh, the admin dialog that
// switches the missing Kaggle settings on by flag name, the text that tells others what to ask for), the preset cards, the
// advanced overrides, the license acceptance and the start: prepare, a confirmation built from the server's answer, then the
// start with the plan token (plan_expired prepares again in the same dialog). The run monitor is P.run (chibi-sensei-run.js),
// mounted when present. Also the "comes later" state of the Test and Use steps. Pure helpers are exported for the Node test.
// Classic script, publishes P.training.
```

  **Constants** (complete). Keep `PRESETS`, `PRESET_KEYS`, `LATER_KEYS`, `REFRESH_MS` and `lrText` of P1, then add:
```js
    const PREFLIGHT_REFRESH_MS = 5000;
    const GROUPS = ['data', 'kaggle', 'account', 'quota', 'storage', 'license'];
    const GROUP_KEYS = {
        data: 'chibi_sensei.preflight.group_data', kaggle: 'chibi_sensei.preflight.group_kaggle', account: 'chibi_sensei.preflight.group_account',
        quota: 'chibi_sensei.preflight.group_quota', storage: 'chibi_sensei.preflight.group_storage', license: 'chibi_sensei.preflight.group_license',
        other: 'chibi_sensei.preflight.group_other'
    };
    const LEVEL_KEYS = { block: 'chibi_sensei.preflight.level_block', warn: 'chibi_sensei.preflight.level_warn' };
    // label: the requirement in words (also used in the enable dialog); fix: what to do when it fails ('' = flag text or none)
    const ITEM_KEYS = {
        dataset_ready: { label: 'chibi_sensei.preflight.dataset_ready', fix: 'chibi_sensei.preflight.dataset_ready_fix' },
        run_active: { label: 'chibi_sensei.preflight.run_active', fix: 'chibi_sensei.preflight.run_active_fix' },
        kaggle_enabled: { label: 'chibi_sensei.preflight.kaggle_enabled', fix: '' },
        kaggle_not_readonly: { label: 'chibi_sensei.preflight.kaggle_not_readonly', fix: '' },
        kaggle_allow_create: { label: 'chibi_sensei.preflight.kaggle_allow_create', fix: '' },
        kaggle_allow_remote_execute: { label: 'chibi_sensei.preflight.kaggle_allow_remote_execute', fix: '' },
        kaggle_allow_gpu: { label: 'chibi_sensei.preflight.kaggle_allow_gpu', fix: '' },
        kaggle_allow_internet: { label: 'chibi_sensei.preflight.kaggle_allow_internet', fix: '' },
        kaggle_allow_download: { label: 'chibi_sensei.preflight.kaggle_allow_download', fix: '' },
        kaggle_machine_shape: { label: 'chibi_sensei.preflight.kaggle_machine_shape', fix: '' },
        kaggle_allow_delete: { label: 'chibi_sensei.preflight.kaggle_allow_delete', fix: 'chibi_sensei.preflight.kaggle_allow_delete_fix' },
        network_access: { label: 'chibi_sensei.preflight.network_access', fix: 'chibi_sensei.preflight.network_access_fix' },
        filesystem_write: { label: 'chibi_sensei.preflight.filesystem_write', fix: 'chibi_sensei.preflight.filesystem_write_fix' },
        kaggle_vault: { label: 'chibi_sensei.preflight.kaggle_vault', fix: 'chibi_sensei.preflight.kaggle_vault_fix' },
        kaggle_credentials: { label: 'chibi_sensei.preflight.kaggle_credentials', fix: 'chibi_sensei.preflight.kaggle_credentials_fix' },
        kaggle_connection: { label: 'chibi_sensei.preflight.kaggle_connection', fix: 'chibi_sensei.preflight.kaggle_connection_fix' },
        kaggle_timeout: { label: 'chibi_sensei.preflight.kaggle_timeout', fix: 'chibi_sensei.preflight.kaggle_timeout_fix' },
        kaggle_quota: { label: 'chibi_sensei.preflight.kaggle_quota', fix: 'chibi_sensei.preflight.kaggle_quota_fix' },
        kaggle_slot: { label: 'chibi_sensei.preflight.kaggle_slot', fix: 'chibi_sensei.preflight.kaggle_slot_fix' },
        kaggle_transfer: { label: 'chibi_sensei.preflight.kaggle_transfer', fix: 'chibi_sensei.preflight.kaggle_transfer_fix' },
        disk_space: { label: 'chibi_sensei.preflight.disk_space', fix: 'chibi_sensei.preflight.disk_space_fix' },
        license_consent: { label: 'chibi_sensei.preflight.license_consent', fix: 'chibi_sensei.preflight.license_consent_fix' }
    };
    const FLAG_ITEMS = {
        'kaggle.enabled': 'kaggle_enabled', 'kaggle.read_only': 'kaggle_not_readonly', 'kaggle.allow_create': 'kaggle_allow_create',
        'kaggle.allow_remote_execute': 'kaggle_allow_remote_execute', 'kaggle.allow_gpu': 'kaggle_allow_gpu', 'kaggle.allow_internet': 'kaggle_allow_internet',
        'kaggle.allow_download': 'kaggle_allow_download', 'kaggle.allowed_machine_shapes': 'kaggle_machine_shape', 'kaggle.allow_delete': 'kaggle_allow_delete'
    };
    const SHAPE_KEYS = { NvidiaTeslaT4: 'chibi_sensei.shape.NvidiaTeslaT4', NvidiaL4: 'chibi_sensei.shape.NvidiaL4' };
    const METHOD_KEYS = { lora: 'chibi_sensei.training.method_lora', full: 'chibi_sensei.training.method_full' };
    const SEQ_BUCKETS = [256, 512, 1024, 2048, 4096];
    const OVERRIDE_FIELDS = ['method', 'epochs', 'learning_rate', 'max_seq_len', 'lora_r', 'lora_alpha'];
    const BAD_KEYS = {
        method: 'chibi_sensei.overrides.bad_method', epochs: 'chibi_sensei.overrides.bad_epochs', learning_rate: 'chibi_sensei.overrides.bad_lr',
        max_seq_len: 'chibi_sensei.overrides.bad_seq', lora_r: 'chibi_sensei.overrides.bad_lora_r', lora_alpha: 'chibi_sensei.overrides.bad_lora_alpha'
    };
```

  **Pure helpers** (complete; no DOM):
```js
    // groupPreflight orders the items by group (GROUPS order, unknown groups last as "other") and counts what blocks.
    function groupPreflight(pf) {
        const items = pf && Array.isArray(pf.items) ? pf.items.filter(i => i && typeof i.id === 'string') : [];
        const byId = {}, groups = [];
        items.forEach(item => {
            const id = GROUPS.indexOf(item.group) >= 0 ? item.group : 'other';
            if (!byId[id]) { byId[id] = { id, items: [], failing: 0 }; groups.push(byId[id]); }
            byId[id].items.push(item);
            if (!item.ok) byId[id].failing++;
        });
        const rank = id => (GROUPS.indexOf(id) >= 0 ? GROUPS.indexOf(id) : GROUPS.length);
        groups.sort((a, b) => rank(a.id) - rank(b.id));
        const blocks = items.filter(i => !i.ok && i.level !== 'warn').length;
        const warns = items.filter(i => !i.ok && i.level === 'warn').length;
        return { groups, blocks, warns, ready: !!(pf && pf.ready) && blocks === 0 };
    }

    // The flags the enable dialog offers, from the server's fix list (deduped). Flags of blocking items are required and always
    // applied; a flag of a note (allow_delete) is optional and can be unticked. Required come first.
    function enableFlags(pf) {
        const fix = pf && pf.fix && Array.isArray(pf.fix.flags) ? pf.fix.flags : [];
        const items = pf && Array.isArray(pf.items) ? pf.items : [];
        const seen = new Set(), out = [];
        fix.forEach(flag => {
            if (typeof flag !== 'string' || !flag || seen.has(flag)) return;
            seen.add(flag);
            const item = items.find(i => i && i.flag === flag && !i.ok);
            out.push({ flag, id: item ? item.id : (FLAG_ITEMS[flag] || ''), optional: !!item && item.level === 'warn' });
        });
        return out.sort((a, b) => Number(a.optional) - Number(b.optional));
    }

    function profileMethods(profile) {
        const p = profile || {};
        return Array.isArray(p.methods) && p.methods.length ? p.methods.slice() : [p.method_default || 'lora'];
    }
    function seqCap(profile) {
        const p = profile || {};
        return Number(p.max_seq_cap) > 0 ? Number(p.max_seq_cap) : Math.min(4096, Number(p.context_max) || 4096);
    }
    function seqOptions(profile) { const cap = seqCap(profile); return SEQ_BUCKETS.filter(b => b <= cap); }

    // validateOverrides mirrors chibisensei.ValidateOverrides. draft holds the form's strings ('' = automatic). It returns only the
    // values that are set; LoRA fields count only when the effective method is LoRA.
    function validateOverrides(draft, profile) {
        const d = draft || {}, out = {}, issues = [];
        const raw = name => String(d[name] == null ? '' : d[name]).trim();
        const int = (name, min, max) => {
            const s = raw(name);
            if (!s) return;
            const n = Number(s);
            if (!/^\d+$/.test(s) || n < min || n > max) issues.push({ field: name, key: BAD_KEYS[name] }); else out[name] = n;
        };
        const method = raw('method');
        if (method) { if (profileMethods(profile).indexOf(method) >= 0) out.method = method; else issues.push({ field: 'method', key: BAD_KEYS.method }); }
        int('epochs', 1, 10);
        const lr = raw('learning_rate').replace(',', '.');
        if (lr) {
            const n = Number(lr);
            if (!Number.isFinite(n) || n < 1e-6 || n > 1e-3) issues.push({ field: 'learning_rate', key: BAD_KEYS.learning_rate }); else out.learning_rate = n;
        }
        const seq = raw('max_seq_len');
        if (seq) { const n = Number(seq); if (seqOptions(profile).indexOf(n) < 0) issues.push({ field: 'max_seq_len', key: BAD_KEYS.max_seq_len }); else out.max_seq_len = n; }
        if ((out.method || (profile && profile.method_default) || 'lora') === 'lora') { int('lora_r', 1, 256); int('lora_alpha', 1, 512); }
        return { overrides: out, issues };
    }

    function shapeText(t, shape) { return SHAPE_KEYS[shape] ? t(SHAPE_KEYS[shape]) : String(shape || ''); }
    function trainingShape(inst) { return (inst.meta && inst.meta.training && inst.meta.training.machine_shape) || 'NvidiaTeslaT4'; }
    function itemVars(inst, item) {
        const vars = { shape: shapeText(inst.t, trainingShape(inst)) };
        const args = item && item.args && typeof item.args === 'object' ? item.args : {};
        Object.keys(args).forEach(k => {
            const v = args[k];
            vars[k] = typeof v === 'number' ? P.format.number(v) : (Array.isArray(v) ? v.map(String).join(', ') : String(v == null ? '' : v));
        });
        return vars;
    }
    function itemLabel(t, item, vars) {
        const keys = ITEM_KEYS[item.id];
        return keys ? t(keys.label, vars) : P.errors.knownKey(t, item.message_key, 'chibi_sensei.preflight.unknown', vars);
    }
    function fixText(t, item, vars, admin) {
        if (item.id === 'kaggle_quota' && item.level === 'warn') return t('chibi_sensei.preflight.kaggle_quota_warn_fix', vars);
        const keys = ITEM_KEYS[item.id];
        if (keys && keys.fix) return t(keys.fix, vars);
        if (item.flag) return t(admin ? 'chibi_sensei.preflight.flag_fix_admin' : 'chibi_sensei.preflight.flag_fix_ask');
        return '';
    }
    function flagLabel(inst, f) {
        const id = f.id || FLAG_ITEMS[f.flag] || '';
        return itemLabel(inst.t, { id, message_key: '' }, itemVars(inst, null));
    }
```

  **Preflight panel** `createPreflight(inst, hooks)` (complete):
```js
    function createPreflight(inst, hooks) {
        const { el, icon } = P;
        const t = inst.t;
        const checked = el('span', { class: 'csn-hint', dataset: { csn: 'preflight-checked' } });
        const refresh = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'preflight-refresh' }, onClick: () => hooks.reload() },
            icon('refresh'), el('span', { text: t('chibi_sensei.preflight.refresh') }));
        const summary = el('p', { class: 'csn-preflight-summary', role: 'status', dataset: { csn: 'preflight-summary', state: 'loading' }, text: t('chibi_sensei.preflight.checking') });
        const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'preflight-error' } });
        error.hidden = true;
        const groupsBox = el('div', { class: 'csn-pf-groups' });
        const fixBox = el('div', { class: 'csn-pf-fixbar', dataset: { csn: 'preflight-fix' } });
        fixBox.hidden = true;
        const node = el('section', { class: 'csn-training-section csn-preflight', dataset: { csn: 'preflight' }, ariaLabel: t('chibi_sensei.preflight.title') },
            el('div', { class: 'csn-section-head' }, el('h3', { text: t('chibi_sensei.preflight.title') }), checked, refresh),
            el('p', { class: 'csn-muted', text: t('chibi_sensei.preflight.intro') }), summary, error, groupsBox, fixBox);
        let last = null;

        function setLoading(on) {
            node.setAttribute('aria-busy', on ? 'true' : 'false');
            refresh.disabled = !!on;
            if (on && !last) { summary.dataset.state = 'loading'; summary.textContent = t('chibi_sensei.preflight.checking'); }
        }

        function itemNode(it, admin) {
            const level = it.level === 'warn' ? 'warn' : 'block';
            const vars = itemVars(inst, it);
            const li = el('li', { class: 'csn-pf-item', dataset: { csn: 'preflight-item', id: it.id, level, ok: it.ok ? '1' : '0' } },
                icon(it.ok ? 'check' : (level === 'warn' ? 'info' : 'alert')),
                el('span', { class: 'csn-pf-text', text: itemLabel(t, it, vars) }),
                el('span', { class: 'csn-pf-state', text: t(it.ok ? 'chibi_sensei.preflight.level_ok' : LEVEL_KEYS[level]) }));
            if (!it.ok) {
                const fix = fixText(t, it, vars, admin);
                if (fix) li.appendChild(el('p', { class: 'csn-hint csn-pf-fix', text: fix }));
                if (it.link && P.safeLink(it.link)) li.appendChild(el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'preflight-link' }, text: t('chibi_sensei.preflight.open_link'), onClick: () => P.openLink(it.link) }));
            }
            return li;
        }

        function paintFix(pf, admin) {
            P.clear(fixBox);
            const flags = enableFlags(pf);
            fixBox.hidden = !flags.length;
            if (!flags.length) return;
            if (admin) {
                const button = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'preflight-enable' }, onClick: () => hooks.enable(pf) },
                    icon('check'), el('span', { text: t('chibi_sensei.enable.button') }));
                button.disabled = inst.readonly;
                button.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
                fixBox.appendChild(el('p', { class: 'csn-hint', text: t('chibi_sensei.enable.lead_admin') }));
                fixBox.appendChild(button);
                return;
            }
            fixBox.appendChild(el('div', { class: 'csn-note', dataset: { csn: 'preflight-ask' } },
                el('strong', { text: t('chibi_sensei.enable.ask_title') }), el('p', { text: t('chibi_sensei.enable.ask_text') }),
                el('ul', { class: 'csn-flag-list' }, flags.map(f => el('li', { text: flagLabel(inst, f) })))));
        }

        function paint(pf) {
            last = pf;
            error.hidden = true;
            const g = groupPreflight(pf);
            const admin = !!(pf && pf.admin);
            P.clear(groupsBox);
            g.groups.forEach(group => groupsBox.appendChild(el('section', { class: 'csn-pf-group', dataset: { csn: 'preflight-group', group: group.id, failing: String(group.failing) } },
                el('h4', { class: 'csn-pf-group-title', text: t(GROUP_KEYS[group.id] || GROUP_KEYS.other) }),
                group.id === 'quota' ? el('p', { class: 'csn-hint', text: t('chibi_sensei.preflight.quota_help') }) : null,
                el('ul', { class: 'csn-pf-list' }, group.items.map(it => itemNode(it, admin))))));
            summary.dataset.state = g.blocks ? 'blocked' : (g.warns ? 'warn' : 'ready');
            summary.textContent = g.blocks ? t('chibi_sensei.preflight.summary_blocked', { count: P.format.number(g.blocks) })
                : (g.warns ? t('chibi_sensei.preflight.summary_warn', { count: P.format.number(g.warns) }) : t('chibi_sensei.preflight.summary_ready'));
            checked.textContent = pf && pf.checked_at ? t('chibi_sensei.preflight.checked_at', { time: P.format.dateTime(pf.checked_at) }) : '';
            paintFix(pf, admin);
        }

        function fail(text) {
            error.textContent = text || '';
            error.hidden = !text;
            summary.dataset.state = 'error';
            summary.textContent = t('chibi_sensei.preflight.failed');
        }

        return { el: node, paint, setLoading, fail, repaint() { if (last) paint(last); } };
    }
```

  **Enable dialog** `openEnable(inst, pf, done)` (complete):
```js
    function openEnable(inst, pf, done) {
        const { el } = P;
        const t = inst.t;
        const flags = enableFlags(pf);
        if (!flags.length || inst.readonly || !P.table) return;
        const dlg = P.table.makeDialog(inst, t('chibi_sensei.enable.title'), 'enable-dialog', false);
        const required = flags.filter(f => !f.optional), optional = flags.filter(f => f.optional);
        const boxes = optional.map(f => el('input', { type: 'checkbox', checked: true, dataset: { csn: 'enable-flag', flag: f.flag } }));
        const err = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'enable-error' } });
        err.hidden = true;
        const cancel = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'enable-cancel' }, text: t('chibi_sensei.enable.cancel'), onClick: () => dlg.close() });
        const ok = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'enable-confirm' }, text: t('chibi_sensei.enable.confirm') });
        dlg.content.appendChild(el('p', { text: t('chibi_sensei.enable.intro') }));
        if (required.length) dlg.content.appendChild(el('div', {}, el('h4', { class: 'csn-subhead', text: t('chibi_sensei.enable.required') }),
            el('ul', { class: 'csn-flag-list', dataset: { csn: 'enable-required' } }, required.map(f => el('li', { dataset: { flag: f.flag }, text: flagLabel(inst, f) })))));
        if (optional.length) dlg.content.appendChild(el('div', { class: 'csn-checkgroup', dataset: { csn: 'enable-optional' } },
            el('h4', { class: 'csn-subhead', text: t('chibi_sensei.enable.optional') }),
            optional.map((f, i) => el('label', { class: 'csn-check-option' }, boxes[i], el('span', { text: flagLabel(inst, f) })))));
        dlg.content.appendChild(err);
        dlg.content.appendChild(el('div', { class: 'csn-actions csn-dialog-actions' }, cancel, ok));
        let busy = false;
        ok.addEventListener('click', async () => {
            if (busy) return;
            const chosen = required.map(f => f.flag).concat(boxes.filter(b => b.checked).map(b => b.dataset.flag));
            if (!chosen.length) { err.textContent = t('chibi_sensei.enable.none_selected'); err.hidden = false; return; }
            busy = true; ok.disabled = true; cancel.disabled = true; ok.textContent = t('chibi_sensei.pending'); err.hidden = true;
            try {
                const res = await inst.api.enablePreflight(chosen);
                if (inst.disposed) return;
                dlg.close();
                const n = Array.isArray(res && res.applied) ? res.applied.length : chosen.length;
                inst.notify(t('chibi_sensei.enable.applied', { count: P.format.number(n) }), 'info');
                done();
            } catch (e) {
                if (inst.disposed || (e && e.aborted)) return;
                err.textContent = inst.handleError(e); err.hidden = false;
            } finally {
                busy = false;
                if (!inst.disposed) { ok.disabled = false; cancel.disabled = false; ok.textContent = t('chibi_sensei.enable.confirm'); }
            }
        });
        dlg.open();
        ok.focus();
    }
```

  **License section** `createLicense(inst, hooks)`. Class `csn-training-section csn-license`, `data-csn="license"`. Never use "consent" in a class name (cookie-banner filters). `paint(profile)`:
  - Hidden unless `profile.license.requires_consent`.
  - Contents:
    - `h3` `consent.title`;
    - `p` `consent.text {model, license}` (model = `display_name || id`, license = `license.name`);
    - `span.csn-label` `consent.link_label`;
    - `P.linkRow(t, license.url, 'license-link')`.
  - When `license.accepted`, add `p.csn-ok-line[data-csn=license-accepted]` `consent.accepted {license}`.
  - Otherwise add:
    - `label.csn-check-option` with `input[type=checkbox][data-csn=license-check]` and `span` `consent.accept_label {license}`;
    - `button.csn-btn.csn-btn-primary[data-csn=license-accept]` `consent.accept`. It is disabled until the box is ticked and on a read-only desktop (title `readonly_banner`).
    - `p.csn-error[role=alert][data-csn=license-error]`.
  - Accept action:
    1. Show `pending` on the button while it runs.
    2. Call `inst.api.consent(profile.id)`.
    3. On success: `profile.license.accepted = true` (the window's meta copy), notify `consent.saved`, `paint(profile)`, then `hooks.accepted()` (reloads the preflight).
    4. On error: the inline message from `inst.handleError`.

  **Presets:**
  - Keep P1 `card()`, with the `plan-estimate` line text now from the changed `training.estimate`, and append:
    - `line(t('chibi_sensei.training.eval_samples', { count: num(plan.eval_samples) }), 'plan-eval')`, only when `plan.eval_samples > 0`;
    - `line(t('chibi_sensei.training.download_size', { mb: num(plan.artifacts_estimated_mb) }), 'plan-download')`, only when `> 0`.
  - The preset section: `section.csn-training-section[data-csn=presets]` with:
    - `h3` `training.preset_title`;
    - `p.csn-muted` `training.preset_hint`;
    - `p.csn-hint` `training.gpu {shape}` (`data-csn=plan-gpu`);
    - the grid;
    - `p.csn-hint` `training.timeout_help`;
    - the note.
  - The note appears only when the selected plan has `trainable === false`: `knownKey(t, reason_key, 'chibi_sensei.plan.training_later')`.
  - `choose(preset)` stays as P1 and calls `loadPreflight()` after a successful save.

  **Overrides** `createOverrides(inst, hooks)`, inside the existing `details.csn-advanced[data-csn=training-advanced]`:
  - Structure:
    - `summary` `training.advanced`;
    - `p.csn-muted` `training.advanced_text`;
    - `form.csn-overrides[data-csn=overrides][novalidate]` holding a `div.csn-grid2` of `label.csn-field` blocks, each with `span.csn-label`, the control, `span.csn-hint` (help) and `p.csn-error[data-csn=ov-<field>-error]`, which is the target of the control's `aria-describedby`; ids are `'csn-' + inst.windowId + '-ov-' + field`.
  - Fields:
    - method: `select.csn-select[data-csn=ov-method]`, only when `profileMethods(profile).length > 1`. Options are `''` → `overrides.method_auto {value: t(METHOD_KEYS[default])}` plus one option per method → `t(METHOD_KEYS[m])`.
    - epochs: `input.csn-input[type=number][min=1][max=10][step=1][data-csn=ov-epochs]` (`overrides.epochs`, `epochs_help`).
    - learning rate: `input[type=text][inputmode=decimal][data-csn=ov-learning_rate]` (`overrides.lr`, `lr_help`).
    - max length: `select[data-csn=ov-max_seq_len]` with `''` → `overrides.auto_option` plus `seqOptions(profile)` (`overrides.seq`, `seq_help`).
    - LoRA rank: `input[type=number][min=1][max=256][data-csn=ov-lora_r]` (`overrides.lora_r`, `lora_r_help`).
    - LoRA alpha: `input[type=number][min=1][max=512][data-csn=ov-lora_alpha]` (`overrides.lora_alpha`, `lora_alpha_help`).
  - Placeholders show the selected preset's plan value through `overrides.auto {value}` (epochs `hyper.epochs`, learning rate `lrText(hyper.learning_rate)`, rank and alpha from `hyper.lora`).
  - Both LoRA fields are hidden (and ignored) while the effective method (select value, else `profile.method_default`) is `full`.
  - Actions: `button[type=submit].csn-btn.csn-btn-primary[data-csn=overrides-save]` `overrides.save`; `button.csn-btn[data-csn=overrides-reset]` `overrides.reset`; `p.csn-error[role=alert][data-csn=overrides-error]`.
  - Values come from `project.overrides` and are refilled only while the form is not dirty (`input` sets dirty).
  - A read-only desktop disables every control.
  - Save code:
```js
        async function save(reset) {
            if (saving || inst.readonly || !inst.project) return;
            clearIssues();
            let next = {};
            if (!reset) {
                const result = validateOverrides(readDraft(), hooks.profile());
                if (result.issues.length) { showIssues(result.issues); return; }
                next = result.overrides;
            }
            saving = true; setBusy(true);
            const mine = inst.projectSeq;
            try {
                const updated = await inst.api.patchProject(inst.project.id, inst.project.rev, { overrides: next });
                if (mine !== inst.projectSeq || inst.disposed) return;
                dirty = false;
                hooks.saved(updated);
                inst.notify(t('chibi_sensei.overrides.saved'), 'info');
            } catch (err) {
                if (inst.disposed || (err && err.aborted)) return;
                const e = P.errors.normalize(err);
                const fields = e.issues.map(i => String(i && i.field || '').replace(/^overrides\./, '')).filter(f => BAD_KEYS[f]);
                if (e.status === 422 && fields.length) showIssues(fields.map(field => ({ field, key: BAD_KEYS[field] })));
                else formError(inst.handleError(err));
            } finally { saving = false; setBusy(false); }
        }
```
  - The helpers:
    - `readDraft()` returns the six control values as strings (`''` for missing controls).
    - `showIssues(list)` sets `aria-invalid="true"`, writes `t(issue.key)` into each field's error line, writes `overrides.fix_fields` into the form error and focuses the first invalid control.
    - `clearIssues()` reverses that.
    - `hooks.saved(updated)` sets `inst.project = updated`, emits `project` and calls `inst.recompute()`. An overrides change bumps `data_rev`, so the plans reload.

  **Start flow** (complete). These are functions inside `create`:
```js
        function setStarting(on) { inst.trainingStarting = !!on; inst.recompute(); }
        const currentPreset = () => (inst.project && PRESETS.indexOf(inst.project.preset) >= 0 ? inst.project.preset : 'balanced');

        async function startTraining() {
            if (disposed || inst.trainingStarting || inst.readonly || !inst.project || !inst.preflight || !inst.preflight.ready) return;
            setStarting(true);
            showError('');
            const id = inst.project.id, preset = currentPreset();
            let prep;
            try {
                prep = await inst.api.prepareRun(id, preset);
            } catch (err) {
                if (!disposed) setStarting(false);
                if (disposed || (err && err.aborted)) return;
                startFailed(err);
                return;
            }
            if (disposed || !inst.project || inst.project.id !== id) { setStarting(false); return; }
            openConfirm(id, preset, prep);
        }

        // A refusal that means the list is out of date reloads it (and the runs), and says why above the list.
        function startFailed(err) {
            showError(inst.handleError(err));
            const code = P.errors.normalize(err).code;
            if (code === 'preflight_failed' || code === 'license_consent_required' || code === 'run_active') loadPreflight();
            if (code === 'run_active') inst.reloadRuns();
        }

        function openConfirm(id, preset, first) {
            const dlg = P.table.makeDialog(inst, t('chibi_sensei.confirm.title'), 'confirm-dialog', true);
            const body = el('div', { dataset: { csn: 'confirm-body' } });
            const notice = el('p', { class: 'csn-note', role: 'status', dataset: { csn: 'confirm-expired' } });
            const err = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'confirm-error' } });
            notice.hidden = true; err.hidden = true;
            const cancel = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'confirm-cancel' }, text: t('chibi_sensei.confirm.cancel'), onClick: () => dlg.close() });
            const start = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'confirm-start' }, text: t('chibi_sensei.confirm.start') });
            dlg.content.appendChild(notice);
            dlg.content.appendChild(body);
            dlg.content.appendChild(err);
            dlg.content.appendChild(el('div', { class: 'csn-actions csn-dialog-actions' }, cancel, start));
            let prep = first, busy = false, done = false;
            const fill = p => { P.clear(body); body.appendChild(confirmDetails(inst, p)); };
            fill(prep);
            dlg.onClose = () => { if (!done && !disposed) setStarting(false); };
            start.addEventListener('click', async () => {
                if (busy) return;
                busy = true; start.disabled = true; cancel.disabled = true; start.textContent = t('chibi_sensei.pending'); err.hidden = true;
                let refocus = false;
                try {
                    const run = await inst.api.startRun(id, prep.plan_token);
                    if (disposed) return;
                    done = true;
                    dlg.close();
                    setStarting(false);
                    inst.applyRun(run);
                    inst.notify(t('chibi_sensei.confirm.started'), 'info');
                    showRun();
                } catch (e) {
                    if (disposed || (e && e.aborted)) return;
                    const code = P.errors.normalize(e).code;
                    if (code === 'plan_expired') {
                        // The token was unknown, expired or the plan changed: prepare again here and say so; the user confirms again.
                        try {
                            prep = await inst.api.prepareRun(id, preset);
                            if (disposed) return;
                            fill(prep);
                            notice.textContent = t('chibi_sensei.confirm.expired');
                            notice.hidden = false;
                            refocus = true;
                        } catch (e2) {
                            if (disposed || (e2 && e2.aborted)) return;
                            dlg.close();
                            startFailed(e2);
                        }
                    } else if (code === 'preflight_failed' || code === 'run_active' || code === 'license_consent_required') {
                        dlg.close();
                        startFailed(e);
                    } else {
                        err.textContent = inst.handleError(e); err.hidden = false;
                    }
                } finally {
                    busy = false;
                    if (!done && !disposed) {
                        start.disabled = false; cancel.disabled = false; start.textContent = t('chibi_sensei.confirm.start');
                        if (refocus) start.focus();
                    }
                }
            });
            dlg.open();
            start.focus();
        }
```
  `confirmDetails(inst, prep)` is module level and complete. Its blocks carry the `data-csn` hooks shown:
```js
    function confirmDetails(inst, prep) {
        const { el } = P;
        const t = inst.t, num = P.format.number;
        const d = prep || {}, data = d.data || {}, rows = data.rows || {}, dest = d.destinations || {};
        const lic = d.license || {}, time = d.time || {}, clean = d.cleanup || {};
        const p = (text, name) => el('p', { dataset: name ? { csn: name } : null, text });
        const block = (name, title, lines) => el('section', { class: 'csn-confirm-block', dataset: { csn: 'confirm-' + name } }, el('h4', { text: title }), lines);
        const session = P.format.minutes(time.session_timeout_minutes);
        const hours = time.quota_hours_remaining;
        const afterKey = !clean.delete_remote_after_run ? 'chibi_sensei.confirm.after_keep_setting'
            : (clean.allow_delete ? 'chibi_sensei.confirm.after_delete' : 'chibi_sensei.confirm.after_keep_policy');
        return el('div', { class: 'csn-confirm' },
            block('data', t('chibi_sensei.confirm.data_title'), [
                p(t('chibi_sensei.confirm.data_rows', { train: num(rows.train), val: num(rows.val), eval: num(rows.eval), size: P.format.bytes(data.bytes) }), 'confirm-rows'),
                p(t('chibi_sensei.confirm.data_private')),
                p(t('chibi_sensei.confirm.dataset', { name: String(dest.dataset || '') }), 'confirm-dataset'),
                p(t('chibi_sensei.confirm.notebook', { name: String(dest.notebook || '') }), 'confirm-notebook'),
                p(t('chibi_sensei.confirm.notebook_help')),
                p(t('chibi_sensei.training.gpu', { shape: shapeText(t, dest.machine_shape) }), 'confirm-gpu')]),
            block('license', t('chibi_sensei.confirm.license', { name: String(lic.name || lic.id || '') }), [lic.url ? P.linkRow(t, lic.url, 'confirm-license-link') : null]),
            block('time', t('chibi_sensei.confirm.time_title'), [
                p(t('chibi_sensei.confirm.estimate', { minutes: P.format.minutes(time.estimate_minutes) }), 'confirm-estimate'),
                p(t('chibi_sensei.confirm.session', { minutes: session }), 'confirm-session'),
                p(typeof hours === 'number' && Number.isFinite(hours) ? t('chibi_sensei.confirm.quota', { hours: P.format.decimal(hours, 1) }) : t('chibi_sensei.confirm.quota_unknown'), 'confirm-quota')]),
            block('stop', t('chibi_sensei.confirm.cancel_title'), [p(t('chibi_sensei.confirm.cancel_text', { minutes: session }), 'confirm-cancel-text')]),
            block('after', t('chibi_sensei.confirm.after_title'), [p(t(afterKey), 'confirm-after')]),
            p(t('chibi_sensei.confirm.valid', { minutes: P.format.minutes((Number(d.expires_in_s) || 900) / 60) }), 'confirm-valid'));
    }
```

  **`create(inst)` wiring.**
  - Order of `node`:
    - `h2.csn-step-title` `training.title`;
    - `p.csn-muted` `training.intro`;
    - `p.csn-error[role=alert][data-csn=training-error]`;
    - `div.csn-training-body[data-csn=training-top]`;
    - `div.csn-training-body` (`body`);
    - `div.csn-training-body[data-csn=training-bottom]`.
  - `layout()` clears `body`:
    - with no `profile_id`, it shows the P1 empty card with `training-to-goal`;
    - with no rows, it shows the `no_data` card;
    - otherwise it appends preflight, license, presets and overrides, in that order.
  - `arrange()` puts `runArea.el` (F3; `null` in F2) into `top` while `P.activeRun(inst.runs)` exists, else into `bottom`.
  - `refresh()` calls `layout()`. When that returns false it sets `inst.preflight = null` and calls `inst.recompute()`. Otherwise it calls `loadPreflight()` and loads the three plans as in P1. It records `loadedRev = inst.project.data_rev` and paints presets, overrides and license.
  - `loadPreflight()`:
```js
        async function loadPreflight() {
            if (disposed || !inst.project || !inst.project.profile_id) return;
            const mine = ++pfSeq, id = inst.project.id;
            pf.setLoading(true);
            try {
                const result = await inst.api.preflight(id, currentPreset());
                if (disposed || mine !== pfSeq || !inst.project || inst.project.id !== id) return;
                inst.preflight = result;
                pf.paint(result);
            } catch (err) {
                if (disposed || mine !== pfSeq || (err && err.aborted)) return;
                inst.preflight = null;
                pf.fail(inst.handleError(err));
            } finally {
                if (!disposed && mine === pfSeq) { pf.setLoading(false); inst.recompute(); }
            }
        }
```
  - `create` starts with `inst.preflight = null;`, so the primary action shows `preflight_loading` until the first answer.
  - Bus subscriptions:
    - `rows` and `checks` → `again` (P1 debounce, 1 s; refreshes the plans and the preflight);
    - `project` → `again()` when `project.data_rev !== loadedRev`, else repaint the presets;
    - `readonly` → repaint presets, overrides, license and `pf.repaint()`;
    - `run` and `runs` → `arrange()`, and when the run is terminal also `againPreflight()` (a separate `P.debounce(loadPreflight, PREFLIGHT_REFRESH_MS)`).
  - `showRun()` calls `runArea.focus()` when the run area exists.
  - Returns `{ el: node, refresh, startTraining, showRun, dispose }`. `dispose` cancels both debounces, drops the bus listeners, disposes `runArea`, and sets `inst.preflight = null; inst.trainingStarting = false`.
  - `createLater` is unchanged.
  - Publish `P.training = { create, createLater, groupPreflight, enableFlags, validateOverrides, seqOptions, PRESETS }`.
- [ ] 7. CSS. Append to `ui/css/desktop-app-chibi-sensei.css` (tokens only):
```css
/* Training step (P2): sections, preflight, enable dialog, license, overrides, confirmation, links */
.csn-training-section { display: grid; gap: var(--ds-space-2); padding: var(--ds-space-3); border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, none); }
.csn-training-section h3 { margin: 0; font-size: 15px; }
.csn-section-head { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); }
.csn-section-head h3 { margin-right: auto; }
.csn-preflight-summary { margin: 0; font-weight: 600; }
.csn-preflight-summary[data-state="ready"] { color: var(--csn-ok-text); }
.csn-preflight-summary[data-state="blocked"], .csn-preflight-summary[data-state="error"] { color: var(--csn-bad-text); }
.csn-preflight-summary[data-state="warn"] { color: var(--csn-warn-text); }
.csn-pf-groups { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: var(--ds-space-3); }
.csn-pf-group { display: grid; align-content: start; gap: var(--ds-space-1); min-width: 0; }
.csn-pf-group-title { margin: 0; color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12px; font-weight: 600; letter-spacing: var(--vd-mat-label-tracking, normal); text-transform: var(--vd-mat-label-transform, none); }
.csn-pf-list { display: grid; gap: 4px; margin: 0; padding: 0; list-style: none; }
.csn-pf-item { display: grid; grid-template-columns: 16px minmax(0, 1fr) auto; column-gap: var(--ds-space-2); row-gap: 2px; align-items: start; padding: 6px var(--ds-space-2); border: 1px solid var(--csn-line); border-left-width: 3px; border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-well, var(--vd-theme-control-bg)); }
.csn-pf-item[data-ok="1"] { border-left-color: var(--csn-ok); }
.csn-pf-item[data-ok="1"] .csn-icon { color: var(--csn-ok-text); }
.csn-pf-item[data-ok="0"][data-level="block"] { border-left-color: var(--csn-bad); }
.csn-pf-item[data-ok="0"][data-level="block"] :is(.csn-icon, .csn-pf-state) { color: var(--csn-bad-text); }
.csn-pf-item[data-ok="0"][data-level="warn"] { border-left-color: var(--csn-warn); }
.csn-pf-item[data-ok="0"][data-level="warn"] :is(.csn-icon, .csn-pf-state) { color: var(--csn-warn-text); }
.csn-pf-state { font-size: 12px; font-weight: 600; white-space: nowrap; }
.csn-pf-item .csn-pf-fix, .csn-pf-item .csn-btn { grid-column: 2 / -1; justify-self: start; }
.csn-pf-fixbar { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); }
.csn-flag-list { margin: 0; padding-left: 18px; }
.csn-linkrow { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); }
.csn-link-text { flex: 1 1 240px; font-family: var(--csn-mono); font-size: 12px; }
.csn-ok-line { margin: 0; color: var(--csn-ok-text); font-weight: 600; }
.csn-overrides { display: grid; gap: var(--ds-space-2); margin-top: var(--ds-space-2); }
.csn-app .csn-input[aria-invalid="true"], .csn-app .csn-select[aria-invalid="true"] { border-color: var(--csn-bad); }
.csn-confirm { display: grid; gap: var(--ds-space-3); }
.csn-confirm-block { display: grid; gap: 4px; }
.csn-confirm-block h4, .csn-confirm-block p { margin: 0; }
.csn-confirm-block h4 { font-size: 13px; }
.csn-note[data-tone="warn"] { border-left-color: var(--csn-warn); }
.csn-dialog-actions { justify-content: flex-end; margin-top: var(--ds-space-2); }
```
- [ ] 8. Locales: add every key of the families marked **F2** in Task F4 to `en.json` and `de.json`, and replace the changed values. Use one key per line and append each family at the end of its existing family block (new families at the end of the file).
- [ ] 9. `scripts/test-chibi-sensei-model.mjs`:
  - Line 9: append `'chibi-sensei-training.js'` to `files`.
  - Line 18: add `URL` to the vm context: `{ window, console, Intl, Map, Set, JSON, Math, Date, URL }`.
  - Lines 63-66: replace with:
```js
for (const step of ['test', 'use']) {
    p = primary(step, {}, {});
    ok(p.disabled && p.reasonKey === 'chibi_sensei.primary.later', step + ' comes later');
}
p = primary('training', { training: 'ready' }, { preflight: null });
ok(p.action === 'start_training' && p.disabled && p.reasonKey === 'chibi_sensei.primary.preflight_loading', 'training waits for the preflight');
p = primary('training', { training: 'ready' }, { preflight: { ready: false } });
ok(p.disabled && p.reasonKey === 'chibi_sensei.primary.preflight_blocked', 'a blocking preflight disables the start');
p = primary('training', { training: 'ready' }, { preflight: { ready: true } });
ok(!p.disabled && p.labelKey === 'chibi_sensei.primary.start_training', 'a ready preflight enables the start');
p = primary('training', { training: 'ready' }, { preflight: { ready: true }, readonly: true });
ok(p.disabled && p.reasonKey === 'chibi_sensei.readonly_banner', 'read-only cannot start');
eq(primary('training', { training: 'done' }, { preflight: { ready: true } }).labelKey, 'chibi_sensei.primary.train_again', 'after a success: train again');
p = primary('training', { training: 'in_progress' }, { runActive: true, readonly: true });
ok(p.action === 'show_run' && !p.disabled, 'an active run is shown, even read-only');
p = primary('training', {}, { preflight: { ready: true }, starting: true });
ok(p.disabled && p.labelKey === 'chibi_sensei.primary.starting', 'a start in flight disables the button');
```
  - Line 70: replace with:
    - `eq(P.steps.tipFor('training', 'ready'), 'chibi_sensei.tip.training_ready', 'tip: training ready');`
    - `eq(P.steps.tipFor('training', 'weird'), 'chibi_sensei.tip.training_todo', 'tip: unknown training state');`
  - After the `moodFor` block, add:
```js
eq(mood({ runActive: true, runFailed: true }), 'thinking', 'mood: an active run is work');
eq(mood({ runFailed: true }), 'worried', 'mood: failed run');
eq(mood({ runSucceeded: true, stepState: 'done' }), 'celebrating', 'mood: run succeeded');

// ---- runs, links ----------------------------------------------------------------------------------------------------------
ok(P.runTerminal('failed') && !P.runTerminal('running') && !P.runTerminal('weird'), 'runTerminal');
const runs = [{ id: 'b', state: 'running', created: '2026-11-23T10:00:00Z' }, { id: 'a', state: 'succeeded', created: '2026-11-22T10:00:00Z' }];
eq(P.activeRun(runs).id, 'b', 'activeRun');
eq(P.activeRun([{ id: 'x', state: 'weird' }]), null, 'an unknown state is not active');
const up = P.upsertRun(runs, { id: 'b', state: 'downloading' });
ok(up[0].id === 'b' && up[0].state === 'downloading' && up[0].created === '2026-11-23T10:00:00Z', 'upsert merges and keeps fields');
eq(P.upsertRun(runs, { id: 'c', state: 'queued', created: '2026-11-24T00:00:00Z' }).map(r => r.id), ['c', 'b', 'a'], 'newest first');
eq(P.upsertRun(runs, { id: 'c', created: '2026-11-24T00:00:00Z' }, 2).length, 2, 'limit');
eq([P.safeLink('/config#kaggle'), P.safeLink('//evil.test/x'), P.safeLink('javascript:void(0)'), P.safeLink('http://example.test/'), P.safeLink('https://example.test/a b')],
    ['/config#kaggle', '', '', '', 'https://example.test/a%20b'], 'safeLink');

// ---- preflight, enable flags, overrides -----------------------------------------------------------------------------------
const pf = { ready: false, items: [
    { id: 'kaggle_enabled', group: 'kaggle', ok: false, level: 'block', flag: 'kaggle.enabled' },
    { id: 'dataset_ready', group: 'data', ok: true, level: 'block' },
    { id: 'mystery', group: 'elsewhere', ok: false, level: 'warn' },
    { id: 'kaggle_allow_delete', group: 'kaggle', ok: false, level: 'warn', flag: 'kaggle.allow_delete' },
    { id: 'kaggle_quota', group: 'quota', ok: true, level: 'block' }], fix: { flags: ['kaggle.allow_delete', 'kaggle.enabled', 'kaggle.enabled'] } };
const g = plain(P.training.groupPreflight(pf));
eq(g.groups.map(x => x.id), ['data', 'kaggle', 'quota', 'other'], 'groups in fixed order, unknown last');
eq([g.blocks, g.warns, g.ready], [1, 2, false], 'preflight counts');
eq(plain(P.training.groupPreflight({ ready: true, items: [] })).ready, true, 'an empty ready list');
eq(P.training.enableFlags(pf), [{ flag: 'kaggle.enabled', id: 'kaggle_enabled', optional: false }, { flag: 'kaggle.allow_delete', id: 'kaggle_allow_delete', optional: true }], 'required first, deduped');
eq(P.training.enableFlags({ items: [], fix: null }), [], 'no fix');
const gemma = { method_default: 'full', methods: ['full', 'lora'], max_seq_cap: 2048 };
const qwen = { method_default: 'lora', methods: ['lora'], max_seq_cap: 2048 };
const v = (d, prof) => plain(P.training.validateOverrides(d, prof));
eq(v({}, qwen), { overrides: {}, issues: [] }, 'empty is automatic');
eq(v({ epochs: '3', learning_rate: '0,0002', max_seq_len: '1024', lora_r: '16', lora_alpha: '32' }, qwen).overrides,
    { epochs: 3, learning_rate: 0.0002, max_seq_len: 1024, lora_r: 16, lora_alpha: 32 }, 'valid values');
eq(v({ method: 'full' }, qwen).issues.map(i => i.field), ['method'], 'qwen has no full method');
eq(v({ epochs: '11', learning_rate: '0.01', max_seq_len: '4096', lora_r: '0', lora_alpha: '1.5' }, qwen).issues.map(i => i.field),
    ['epochs', 'learning_rate', 'max_seq_len', 'lora_r', 'lora_alpha'], 'out of range');
eq(v({ lora_r: '999' }, gemma).issues, [], 'gemma defaults to full: LoRA fields are ignored');
eq(v({ method: 'lora', lora_r: '999' }, gemma).issues.map(i => i.field), ['lora_r'], 'gemma with LoRA checks the rank');
eq(v({ max_seq_len: '300' }, { method_default: 'lora', context_max: 32768 }).issues.map(i => i.field), ['max_seq_len'], 'not a bucket');
```
- [ ] 10. Verify:
```bash
for f in core api mascot steps training; do node --check ui/js/desktop/apps/chibi-sensei/chibi-sensei-$f.js; done; node --check ui/js/desktop/apps/chibi-sensei/chibi-sensei.js
node -e "for (const f of process.argv.slice(1)) JSON.parse(require('fs').readFileSync(f,'utf8'))" ui/lang/chibi_sensei/en.json ui/lang/chibi_sensei/de.json
node scripts/test-chibi-sensei-model.mjs
git diff --check
go build ./...
go test ./ui -run '(?i)ChibiSensei' -skip '(?i)LocalesCoverAllSixteenLanguages' -count=1
go test ./ui -run '(?i)(ChibiSenseiLocalesCoverAllSixteenLanguages)' -count=1
```
  Expected:
  - `chibi-sensei model: ok (N checks)`;
  - `ok aurago/ui` for the skipped run;
  - the last command FAILS and lists only `lacks chibi_sensei.<key>` for the new keys in the 14 F4 languages (expected until F4).

---

### Task F3: Training step, part 2 (run monitor, chart, live wiring)

**Files**
- Create:
  - `ui/js/desktop/apps/chibi-sensei/chibi-sensei-chart.js` (≈200 lines)
  - `ui/js/desktop/apps/chibi-sensei/chibi-sensei-run.js` (≈820 lines)
- Modify:
  - `chibi-sensei.js:391-409` (`openEvents`), `:434-467` (`syncPoll`, `pollJobs`)
  - `chibi-sensei-training.js` (mount `P.run.createArea`)
  - `ui/css/desktop-app-chibi-sensei.css` (append)
  - `ui/lang/chibi_sensei/en.json`, `de.json` (F3 families)
  - `scripts/test-chibi-sensei-model.mjs`
  - `ui/js/desktop/core/module-loader.js:39`
  - `ui/desktop_chibi_sensei_guards_test.go:49-50`
  - `ui/desktop_chibi_sensei_registration_test.go:13`
  - `ui/i18n_lint_test.go:1121`

**Existing symbols modified:** `openEvents`, `syncPoll`, `pollJobs`, `create` (training), `DESKTOP_APP_ASSETS` (`chibi-sensei` entry), `chibiSenseiModules`, `TestChibiSenseiSourcesAreSafeAndSmall`, `desktopLocalI18nHelpers`.

**Steps**
- [ ] 1. `chibi-sensei-chart.js`, complete:
```js
// Chibi Sensei loss chart: the training loss as one SVG line and the validation loss as points, built with createElementNS, plus
// the pure layout() that turns metric points into SVG geometry (the Node model test covers it). Above 600 training points the
// line is averaged in buckets. No animation; a sentence under the chart says the same in words. Classic script, publishes P.chart.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const NS = 'http://www.w3.org/2000/svg';
    const MAX_POINTS = 600;
    const WIDTH = 560;
    const HEIGHT = 220;
    const PAD = { left: 48, right: 12, top: 12, bottom: 30 };

    function finite(v) { return typeof v === 'number' ? Number.isFinite(v) : (v !== null && v !== '' && v !== undefined && Number.isFinite(Number(v))); }
    const round1 = v => Math.round(v * 10) / 10;
    const minOf = list => list.reduce((m, v) => (v < m ? v : m), Infinity);
    const maxOf = list => list.reduce((m, v) => (v > m ? v : m), -Infinity);

    // downsample averages consecutive points in buckets so at most max remain; each bucket keeps its last step.
    function downsample(points, max) {
        const limit = max || MAX_POINTS;
        if (points.length <= limit) return points.slice();
        const size = Math.ceil(points.length / limit), out = [];
        for (let i = 0; i < points.length; i += size) {
            const bucket = points.slice(i, i + size);
            out.push({ step: bucket[bucket.length - 1].step, loss: bucket.reduce((a, p) => a + p.loss, 0) / bucket.length, kind: 'train' });
        }
        return out;
    }

    // layout(metrics, {maxSteps, width, height, maxPoints}) -> SVG geometry; no DOM. The summary uses the raw points.
    function layout(metrics, opts) {
        const o = opts || {};
        const width = o.width || WIDTH, height = o.height || HEIGHT;
        const plot = { left: PAD.left, top: PAD.top, right: width - PAD.right, bottom: height - PAD.bottom };
        const clean = (Array.isArray(metrics) ? metrics : [])
            .filter(m => m && finite(m.step) && finite(m.loss))
            .map(m => ({ step: Number(m.step), loss: Number(m.loss), kind: m.kind === 'eval' ? 'eval' : 'train' }))
            .sort((a, b) => a.step - b.step);
        const rawTrain = clean.filter(m => m.kind === 'train');
        const evals = clean.filter(m => m.kind === 'eval');
        const out = { width, height, plot, empty: true, path: '', evalPoints: [], xTicks: [], yTicks: [], summary: null, evalLast: null, trainCount: 0 };
        if (!rawTrain.length && !evals.length) return out;
        const train = downsample(rawTrain, o.maxPoints);
        const shown = train.concat(evals);
        const losses = shown.map(p => p.loss);
        const xMax = Math.max(1, finite(o.maxSteps) ? Number(o.maxSteps) : 0, maxOf(shown.map(p => p.step)));
        let yMin = minOf(losses), yMax = maxOf(losses);
        if (yMax - yMin < 1e-9) { yMin -= 0.5; yMax += 0.5; } else { const m = (yMax - yMin) * 0.08; yMin -= m; yMax += m; }
        if (yMin < 0 && minOf(losses) >= 0) yMin = 0;
        const w = plot.right - plot.left, h = plot.bottom - plot.top;
        const x = s => round1(plot.left + (s / xMax) * w);
        const y = v => round1(plot.top + (1 - (v - yMin) / (yMax - yMin)) * h);
        [0, 0.25, 0.5, 0.75, 1].forEach(f => { const s = Math.round(xMax * f); if (!out.xTicks.some(tk => tk.value === s)) out.xTicks.push({ x: x(s), value: s }); });
        out.yTicks = [0, 1, 2, 3].map(i => { const v = yMin + (yMax - yMin) * i / 3; return { y: y(v), value: v }; });
        out.empty = false;
        out.path = train.map((p, i) => (i ? 'L' : 'M') + x(p.step) + ' ' + y(p.loss)).join(' ');
        out.evalPoints = evals.map(p => ({ x: x(p.step), y: y(p.loss), step: p.step, loss: p.loss }));
        out.trainCount = train.length;
        if (rawTrain.length) out.summary = { first: rawTrain[0].loss, last: rawTrain[rawTrain.length - 1].loss, min: minOf(rawTrain.map(p => p.loss)), steps: rawTrain[rawTrain.length - 1].step };
        if (evals.length) out.evalLast = evals[evals.length - 1].loss;
        return out;
    }

    function summaryText(t, g) {
        const parts = [];
        const dec = v => P.format.decimal(v, 3);
        if (g.summary) parts.push(t('chibi_sensei.chart.summary', { first: dec(g.summary.first), last: dec(g.summary.last), steps: P.format.number(g.summary.steps), min: dec(g.summary.min) }));
        if (g.evalLast != null) parts.push(t('chibi_sensei.chart.summary_eval', { value: dec(g.evalLast) }));
        return parts.join(' ');
    }

    function svg(name, attrs, text) {
        const node = document.createElementNS(NS, name);
        Object.keys(attrs || {}).forEach(k => node.setAttribute(k, String(attrs[k])));
        if (text != null) node.textContent = String(text);
        return node;
    }

    // create(inst) -> {el, update(metrics, maxSteps, noteText), dispose}
    function create(inst) {
        const { el } = P;
        const t = inst.t;
        const box = svg('svg', { class: 'csn-chart-svg', viewBox: '0 0 ' + WIDTH + ' ' + HEIGHT, 'aria-hidden': 'true', focusable: 'false', 'data-csn': 'chart-svg' });
        const summary = el('p', { class: 'csn-chart-summary', dataset: { csn: 'chart-summary' } });
        const note = el('p', { class: 'csn-muted', dataset: { csn: 'chart-note' } });
        const node = el('figure', { class: 'csn-chart', dataset: { csn: 'chart', empty: '1' } },
            el('figcaption', { class: 'csn-chart-head' }, el('strong', { text: t('chibi_sensei.chart.title') }),
                el('span', { class: 'csn-chart-legend' },
                    el('span', { class: 'csn-chart-key', dataset: { kind: 'train' }, text: t('chibi_sensei.chart.train') }),
                    el('span', { class: 'csn-chart-key', dataset: { kind: 'eval' }, text: t('chibi_sensei.chart.eval') }))),
            box, summary, note);

        function update(metrics, maxSteps, noteText) {
            const g = layout(metrics, { maxSteps });
            while (box.firstChild) box.removeChild(box.firstChild);
            if (!g.empty) {
                g.yTicks.forEach(tk => {
                    box.appendChild(svg('line', { class: 'csn-chart-gridline', x1: g.plot.left, x2: g.plot.right, y1: tk.y, y2: tk.y }));
                    box.appendChild(svg('text', { class: 'csn-chart-axis', x: g.plot.left - 6, y: tk.y + 4, 'text-anchor': 'end' }, P.format.decimal(tk.value, 2)));
                });
                g.xTicks.forEach(tk => box.appendChild(svg('text', { class: 'csn-chart-axis', x: tk.x, y: g.plot.bottom + 16, 'text-anchor': 'middle' }, P.format.number(tk.value))));
                box.appendChild(svg('text', { class: 'csn-chart-axis', x: g.plot.right, y: g.height - 1, 'text-anchor': 'end' }, t('chibi_sensei.chart.axis_step')));
                if (g.path) box.appendChild(svg('path', { class: 'csn-chart-train', d: g.path }));
                if (g.trainCount === 1) { const m = /^M(\S+) (\S+)$/.exec(g.path); if (m) box.appendChild(svg('circle', { class: 'csn-chart-dot', cx: m[1], cy: m[2], r: 3 })); }
                g.evalPoints.forEach(p => box.appendChild(svg('circle', { class: 'csn-chart-eval', cx: p.x, cy: p.y, r: 3.5 })));
            }
            summary.textContent = summaryText(t, g);
            summary.hidden = !summary.textContent;
            note.textContent = noteText || (g.empty ? t('chibi_sensei.chart.empty') : '');
            note.hidden = !note.textContent;
            node.dataset.empty = g.empty ? '1' : '0';
        }

        return { el: node, update, dispose() {} };
    }

    P.chart = { create, layout, downsample, MAX_POINTS };
})();
```
- [ ] 2. `chibi-sensei-run.js`, part 1: header, constants and pure helpers (complete). Every key map stays literal for `TestChibiSenseiAppUsesOnlyKeysItHas`.
```js
// Chibi Sensei run monitor: the current run (state, elapsed time against the estimate, stage timeline over the run states and the
// trainer stages, live or post-run loss chart, log highlights, cancel with the plain promise, the "stop it on Kaggle" state with
// the notebook link, result card, files, leftovers, delete) and the run history of the project. Live data comes from the window's
// bus (run, runs, metric, stage); the root polls the active run every 2.5 s as a safety net. Pure helpers are exported for the
// Node test. Classic script, publishes P.run.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const PHASES = ['prepare', 'upload', 'start', 'train', 'download', 'cleanup'];
    const PHASE_OF = { queued: 'prepare', packaging: 'prepare', uploading: 'upload', creating_dataset: 'upload', waiting_dataset: 'upload', starting: 'start', running: 'train', downloading: 'download', verifying: 'download', cleaning: 'cleanup' };
    const STAGES = ['boot', 'env_check', 'install', 'data', 'load_base', 'base_eval', 'train', 'tuned_eval', 'save', 'gguf', 'manifest'];
    const FAIL_PHASE = { kaggle_submit_unknown: 'start', download_failed: 'download', checksum_mismatch: 'download', status_missing: 'download' };
    const TICK_MS = 1000, EVENTS_KEPT = 200, EVENTS_DEBOUNCE_MS = 1000, METRICS_DEBOUNCE_MS = 5000, CHART_DEBOUNCE_MS = 500;
```
  Then add these literal maps:
  - `PHASE_KEYS` (`chibi_sensei.phase.<id>`);
  - `PHASE_STATE_KEYS` (`done | active | failed | stopped | todo | skipped` → `chibi_sensei.phase_state.<s>`);
  - `STAGE_KEYS` (11 stages → `chibi_sensei.stage.<id>`, plus `unknown`);
  - `RUN_STATE_KEYS` (13 → `chibi_sensei.run_state.<s>`);
  - `STAGE_EVENT_KEYS` (`start | done | failed` → `chibi_sensei.event.stage_<s>`);
  - `FAIL_KEYS` (each of the 19 codes in the F4 fragment plus `unknown` → `{ title: 'chibi_sensei.fail.<code>', fix: 'chibi_sensei.fail.<code>_fix' }`);
  - `WARNING_KEYS` (`time_budget`, `base_gguf_skipped`, `unknown`);
  - `ARTIFACT_KEYS` (`gguf_tuned`, `gguf_base`, `adapter`, `adapter_config`, `metrics`, `eval_samples`, `status`, `log`, `other`);
  - `LEFTOVER_KEYS` (`dataset`, `notebook`, `other`);
  - `PRESET_NAME_KEYS` (`quick | balanced | thorough` → the existing `chibi_sensei.training.preset_<id>`).

  The helper functions:
```js
    // The phase the run is in now, or was in when it ended. last_state is the server's answer; without it the code, the trainer
    // stage and the remote facts decide.
    function reachedPhase(run) {
        const r = run || {};
        if (PHASE_OF[r.state]) return PHASE_OF[r.state];
        if (r.state === 'succeeded') return 'cleanup';
        if (PHASE_OF[r.last_state]) return PHASE_OF[r.last_state];
        if (FAIL_PHASE[r.error_code]) return FAIL_PHASE[r.error_code];
        if ((r.stage && STAGES.indexOf(r.stage.name) >= 0) || r.remote_started) return 'train';
        const remote = r.remote || {};
        if (remote.notebook) return 'start';
        if (remote.dataset) return 'upload';
        return 'prepare';
    }

    // timeline(run) -> {phase, phases: [{id, status}], stages: [{id, status}]}; status: done | active | failed | stopped | todo | skipped
    function timeline(run) {
        const r = run || {};
        const phase = reachedPhase(r);
        const at = PHASES.indexOf(phase), trainAt = PHASES.indexOf('train');
        const end = r.state === 'failed' ? 'failed' : (r.state === 'cancelled' ? 'stopped' : '');
        const later = end ? 'skipped' : 'todo';
        const phases = PHASES.map((id, i) => ({ id, status: r.state === 'succeeded' || i < at ? 'done' : (i > at ? later : (end || 'active')) }));
        const s = r.stage && STAGES.indexOf(r.stage.name) >= 0 ? r.stage : null;
        const idx = s ? STAGES.indexOf(s.name) : -1;
        const stages = STAGES.map((id, i) => {
            if (r.state === 'succeeded' || at > trainAt) return { id, status: 'done' };
            if (at < trainAt || idx < 0 || i > idx) return { id, status: later };
            if (i < idx || s.state === 'done') return { id, status: 'done' };
            if (s.state === 'failed') return { id, status: 'failed' };
            return { id, status: end || 'active' };
        });
        return { phase, phases, stages };
    }

    // mergeMetrics dedupes by kind and step (the later point wins) and sorts by step, training before validation.
    function mergeMetrics(list, points) {
        const map = new Map();
        (Array.isArray(list) ? list : []).concat(Array.isArray(points) ? points : []).forEach(m => {
            if (m && Number.isFinite(Number(m.step))) map.set((m.kind === 'eval' ? 'eval' : 'train') + ':' + Number(m.step), m);
        });
        return Array.from(map.values()).sort((a, b) => (Number(a.step) - Number(b.step)) || ((a.kind === 'eval') - (b.kind === 'eval')));
    }

    function elapsedSeconds(run, now) {
        const r = run || {};
        const from = Date.parse(r.remote_started || r.created || '');
        if (!Number.isFinite(from)) return 0;
        const to = P.runTerminal(r.state) ? Date.parse(r.finished || r.updated || '') : now;
        return Number.isFinite(to) ? Math.max(0, Math.round((to - from) / 1000)) : 0;
    }

    // liveNote: the key of the note under the chart while the run trains, '' when none applies.
    function liveNote(run, points) {
        const r = run || {};
        if (r.state !== 'running') return '';
        if (r.live === 'no') return 'chibi_sensei.run.live_off';
        if (r.live !== 'yes' && !points) return 'chibi_sensei.run.live_waiting';
        return '';
    }

    function stageName(t, id) { return t(STAGE_KEYS[id] || STAGE_KEYS.unknown); }

    // eventText: one log highlight in words, '' for a shape the UI does not know (never the raw payload).
    function eventText(t, ev) {
        const d = ev && ev.data && typeof ev.data === 'object' ? ev.data : {};
        const str = v => (typeof v === 'string' ? v.slice(0, 300) : '');
        switch (ev && ev.kind) {
        case 'state': { const s = str(d.to) || str(d.state); return RUN_STATE_KEYS[s] ? t('chibi_sensei.event.state', { state: t(RUN_STATE_KEYS[s]) }) : ''; }
        case 'stage': { const key = STAGE_EVENT_KEYS[str(d.state)]; return key ? t(key, { stage: stageName(t, str(d.stage)) }) : ''; }
        case 'warn': return t('chibi_sensei.event.warn', { text: WARNING_KEYS[str(d.code)] ? t(WARNING_KEYS[str(d.code)]) : (str(d.message) || t(WARNING_KEYS.unknown)) });
        case 'error': return t('chibi_sensei.event.error', { text: FAIL_KEYS[str(d.code)] ? t(FAIL_KEYS[str(d.code)].title) : (str(d.message) || t(FAIL_KEYS.unknown.title)) });
        case 'kaggle': return str(d.step) ? t('chibi_sensei.event.kaggle', { step: str(d.step) }) : '';
        case 'log': return str(d.text);
        default: return '';
        }
    }
```
- [ ] 3. `chibi-sensei-run.js`, part 2: `createArea(inst)` builds the DOM and the wiring (complete code for wiring, cancel and delete; painters specified).
  - **DOM** (classes and hooks):
    - `div.csn-runs[data-csn=runs]` holds two sections.
    - `section.csn-training-section.csn-run[data-csn=run-current]` (hidden when the project has no runs) contains, in this order:
      - `div.csn-run-head`: `h3.csn-run-title[tabindex=-1][data-csn=run-title]` and `span.csn-badge.csn-run-state[data-csn=run-state][data-state]`;
      - `p.csn-run-status[role=status][data-csn=run-status]` (rewritten only when the sentence changes);
      - `p.csn-run-meta[data-csn=run-time]` and `p.csn-run-meta[data-csn=run-progress]`;
      - `p.csn-note[data-csn=run-live-note]` and `p.csn-note[data-tone=warn][data-csn=run-longer]`;
      - `ol.csn-timeline[data-csn=run-timeline][aria-label=run.timeline]`, whose items are `li.csn-phase[data-phase][data-status]` with `span.csn-phase-name` and `span.csn-phase-state`;
      - `ol.csn-stages[data-csn=run-stages][aria-label=run.stages]`, whose items are `li.csn-stage[data-stage][data-status][title=state word]`; shown only when the phase reached is `train` or later;
      - `div.csn-actions[data-csn=run-actions]` with `run-cancel` and `run-delete`;
      - `p.csn-note[data-csn=run-cancel-note]` and `div.csn-note[data-tone=warn][data-csn=run-stop]`;
      - `div.csn-result[data-csn=run-result][data-outcome]`;
      - the chart (`P.chart.create(inst).el`, when present);
      - `div[data-csn=run-files]` and `div[data-csn=run-leftovers]`;
      - three `details.csn-advanced`: `run-events` (`ol.csn-events`, items `li.csn-event[data-kind]` with `span.csn-event-time` and `span`), `run-jobs` and `run-log` (`pre.csn-log[data-csn=run-log-text]`, loaded on the first `toggle`);
      - `p.csn-error[role=alert][data-csn=run-error]`.
    - `section.csn-training-section.csn-run-history[data-csn=run-history]`: `h3` `run.history_title` and `ol.csn-run-list`. Each item is `button.csn-run-item[data-csn=run-item][data-run][data-state][aria-pressed]` holding `span` date (`format.dateTime(created)`), `span` preset name and `span.csn-run-item-state` (state word). Its `aria-label` is `run.item_label {date, preset, state}`, and a click selects the run (`pinned = true`). With no runs the list shows `p.csn-muted` `run.history_empty`.
  - **Wiring:**
```js
    function createArea(inst) {
        const t = inst.t;
        let disposed = false, selectedId = '', pinned = false, detail = null, detailSeq = 0, cancelling = false, deleting = false;
        let chartTimer = 0, eventsTimer = 0, metricsTimer = 0;
        const chart = P.chart ? P.chart.create(inst) : null;
        // ... build the nodes listed above ...
        const tolerant = promise => promise.catch(err => { if (err && err.aborted) throw err; return null; });
        const stale = mine => disposed || mine !== detailSeq;

        function pickSelected() {
            const runs = inst.runs || [];
            const active = P.activeRun(runs);
            if (active && !(pinned && runs.some(r => r.id === selectedId))) return active.id;
            if (selectedId && runs.some(r => r.id === selectedId)) return selectedId;
            return runs.length ? runs[0].id : '';
        }

        function sync() {
            const next = pickSelected();
            paintHistory();
            if (next === selectedId) return;
            selectedId = next;
            if (next) loadDetail(next); else { detail = null; paintDetail(); }
        }

        async function loadDetail(id) {
            const mine = ++detailSeq;
            detail = { id, run: (inst.runs || []).find(r => r.id === id) || { id }, metrics: [], maxSteps: 0, live: '', events: [], artifacts: null, report: null };
            paintDetail();
            try {
                const [run, metrics, events] = await Promise.all([inst.api.getRun(id), tolerant(inst.api.runMetrics(id, 0)), tolerant(inst.api.runEvents(id, 0, EVENTS_KEPT))]);
                if (stale(mine)) return;
                detail.run = run;
                applyMetricsBody(metrics, true);
                detail.events = (events || []).slice(-EVENTS_KEPT);
                if (P.runTerminal(run.state)) await loadFinal(id, mine);
                if (!stale(mine)) paintDetail();
            } catch (err) {
                if (stale(mine) || (err && err.aborted)) return;
                const e = P.errors.normalize(err);
                if (e.status === 404 || e.code === 'run_not_found') { inst.removeRun(id); return; }
                showError(inst.handleError(err));
            }
        }

        async function loadFinal(id, mine) {
            const [artifacts, report] = await Promise.all([tolerant(inst.api.runArtifacts(id)), tolerant(inst.api.runReport(id))]);
            if (stale(mine)) return;
            detail.artifacts = artifacts || [];
            detail.report = report || null;
        }

        // The run just ended: the post-run metrics replace the live ones, then files, report and the last events.
        async function finishDetail(id) {
            const mine = detailSeq;
            try {
                const [run, metrics] = await Promise.all([inst.api.getRun(id), tolerant(inst.api.runMetrics(id, 0))]);
                if (stale(mine)) return;
                detail.run = run;
                applyMetricsBody(metrics, true);
                await loadFinal(id, mine);
                await loadEvents(id, mine);
            } catch (err) { if (!stale(mine) && !(err && err.aborted)) showError(inst.handleError(err)); }
            if (!stale(mine)) paintDetail();
        }

        function applyMetricsBody(body, replace) {
            if (!body || !detail) return;
            detail.metrics = mergeMetrics(replace ? [] : detail.metrics, body.metrics);
            if (Number(body.max_steps) > 0) detail.maxSteps = Number(body.max_steps);
            if (typeof body.live === 'string') detail.live = body.live;
        }

        async function loadEvents(id, mine) {
            if (!detail || detail.id !== id) return;
            const after = detail.events.length ? Number(detail.events[detail.events.length - 1].id) || 0 : 0;
            try {
                const list = await inst.api.runEvents(id, after, EVENTS_KEPT);
                if (stale(mine)) return;
                detail.events = detail.events.concat(list.filter(e => e && Number(e.id) > after)).slice(-EVENTS_KEPT);
                paintEvents();
            } catch (_) { /* the next stage event or the end of the run loads them */ }
        }

        // Without the event stream the chart would wait for the end: when the run reports a step past the last point, fetch the rest.
        function maybeFetchMetrics() {
            const r = detail && detail.run;
            if (!r || r.state !== 'running' || !r.stage || !(Number(r.stage.step) > 0)) return;
            const last = detail.metrics.length ? Number(detail.metrics[detail.metrics.length - 1].step) : 0;
            if (Number(r.stage.step) <= last || metricsTimer) return;
            const id = detail.id, mine = detailSeq;
            metricsTimer = setTimeout(async () => {
                metricsTimer = 0;
                try { const body = await inst.api.runMetrics(id, last); if (!stale(mine)) { applyMetricsBody(body, false); scheduleChart(); } } catch (_) { /* next time */ }
            }, METRICS_DEBOUNCE_MS);
        }

        function scheduleChart() {
            if (chartTimer) return;
            chartTimer = setTimeout(() => { chartTimer = 0; if (!disposed) paintChart(); }, CHART_DEBOUNCE_MS);
        }

        function onRun(run) {
            if (disposed || !run) return;
            sync();
            if (!detail || run.id !== detail.id) return;
            const was = P.runTerminal(detail.run && detail.run.state);
            detail.run = Object.assign({}, detail.run || {}, run);
            if (!was && P.runTerminal(detail.run.state)) { paintDetail(); finishDetail(detail.id); return; }
            maybeFetchMetrics();
            paintDetail();
        }
        function onMetric(body) {
            if (disposed || !detail || !body || body.run_id !== detail.id) return;
            detail.metrics = mergeMetrics(detail.metrics, body.points);
            if (detail.live !== 'no') detail.live = 'yes';
            scheduleChart();
        }
        function onStage(body) {
            if (disposed || !detail || !body || body.run_id !== detail.id) return;
            const id = detail.id, mine = detailSeq;
            clearTimeout(eventsTimer);
            eventsTimer = setTimeout(() => loadEvents(id, mine), EVENTS_DEBOUNCE_MS);
        }

        const offs = [inst.bus.on('run', onRun), inst.bus.on('runs', sync), inst.bus.on('metric', onMetric), inst.bus.on('stage', onStage), inst.bus.on('readonly', () => paintActions())];
        const ticker = setInterval(() => { if (!disposed && detail && detail.run && !P.runTerminal(detail.run.state)) paintTime(); }, TICK_MS);
        sync();

        return {
            el: node,
            focus() { if (!current.hidden) { current.scrollIntoView({ block: 'start' }); heading.focus({ preventScroll: true }); } },
            dispose() {
                disposed = true;
                clearInterval(ticker); clearTimeout(chartTimer); clearTimeout(eventsTimer); clearTimeout(metricsTimer);
                offs.forEach(off => { if (typeof off === 'function') off(); });
                if (chart) chart.dispose();
            }
        };
    }
```
  - **Cancel and delete** (complete). Cancel is not disabled on a read-only desktop (a stop operation); delete is.
```js
        async function cancelRun() {
            const run = detail && detail.run;
            if (!run || P.runTerminal(run.state) || run.cancel_requested || cancelling) return;
            const minutes = P.format.minutes(run.session_timeout_minutes || (run.cancel && run.cancel.time_limit_minutes));
            const ok = await inst.askConfirm(t('chibi_sensei.run.cancel_title'), t('chibi_sensei.run.cancel_text', { minutes }));
            if (!ok || disposed) return;
            cancelling = true; paintActions();
            try { const updated = await inst.api.cancelRun(run.id); if (!disposed) inst.applyRun(updated); }
            catch (err) { if (!disposed && !(err && err.aborted)) showError(inst.handleError(err)); }
            finally { cancelling = false; if (!disposed) paintActions(); }
        }

        async function deleteRun() {
            const run = detail && detail.run;
            if (!run || !P.runTerminal(run.state) || inst.readonly || deleting) return;
            const ok = await inst.askConfirm(t('chibi_sensei.run.delete_title'), t('chibi_sensei.run.delete_text'));
            if (!ok || disposed) return;
            deleting = true; paintActions();
            try { await inst.api.deleteRun(run.id); if (disposed) return; pinned = false; inst.removeRun(run.id); inst.notify(t('chibi_sensei.run.deleted'), 'info'); }
            catch (err) { if (!disposed && !(err && err.aborted)) showError(inst.handleError(err)); }
            finally { deleting = false; if (!disposed) paintActions(); }
        }
```
  - **Painters.** `paintDetail` calls all of them; each only sets `textContent`, attributes and `hidden`.
    - `paintHead`:
      - title: `run.title` while active, else `run.title_past {date}`;
      - badge: state word from `RUN_STATE_KEYS`;
      - status: the state sentence, plus ` ` and `run.status_kaggle {status}` when `run.remote.status` is set during `running`.
    - `paintTime` (also ticked every second):
      - `run.elapsed {time}` with `format.duration(elapsedSeconds(run, Date.now()))`, then ` · ` and `run.estimate {minutes}`;
      - `run-longer` shows `run.longer {minutes: session}` when not terminal and elapsed > 1.25 × `estimate_minutes` × 60;
      - progress: `run.progress {step, max, epoch}` (epoch via `format.decimal(epoch, 1)`) when `stage.name === 'train'` and `stage.max_steps > 0`, plus ` · ` and `run.eta {time}` when `stage.eta_s > 0`.
    - `paintTimeline`:
      - from `timeline(run)`: each `li` gets `data-status`, name `t(PHASE_KEYS[id])`, state `t(PHASE_STATE_KEYS[status])` and `aria-label = name + ', ' + stateWord`;
      - the active phase gets `aria-current="step"`;
      - stages likewise, shown when the phase index ≥ `train`.
    - `paintActions`:
      - `run-cancel` is shown while not terminal and not `cancel_requested`, disabled while `cancelling`, and shows the text `pending` while busy;
      - `run-delete` is shown when terminal, disabled while read-only (title `readonly_banner`) or deleting.
    - `paintCancel`:
      - when `cancel_requested` and not terminal:
        - the cancel note is `run.cancel_requested {minutes: cancel.time_limit_minutes}`;
        - when `cancel.remote_stopped === false && cancel.stop_url`, the `run-stop` box shows `h4` `run.stop_title`, `p` `run.stop_text` and `P.linkRow(t, run.cancel.stop_url, 'run-stop-link')`;
      - when no `remote.notebook` exists yet, the note is `run.stopped_local` instead.
    - `paintResult` (terminal only; `data-outcome` = state):
      - **succeeded:**
        - `h4` `run.result_title`;
        - `run.result_steps {steps, epochs}`;
        - `run.train_loss {value}`;
        - `run.eval_loss {base, tuned}` and `run.loss_help` (from `report.loss`, else `result.eval_loss_base/tuned`);
        - `run.exact {base, tuned, n}` (`format.share`) when `report.exact_match`;
        - `run.labels {…}` when `report.label_accuracy`;
        - `run.perplexity {base, tuned}` when `report.perplexity`;
        - `run.small_sample {n: report.samples}` when `report.small_sample`;
        - `run.gpu_used {name}` when `result.gpu`;
        - `warning_code` → `div.csn-note[data-tone=warn][data-csn=run-warning]` with `strong` `run.warning_title` and `p` `WARNING_KEYS[code] || unknown`.
      - **failed:** `h4` `run.failed_title`, `p[data-csn=run-fail]` `FAIL_KEYS[code].title`, `h4` `run.fix_title`, `p[data-csn=run-fix]` `FAIL_KEYS[code].fix`, and `code.csn-detail` with `run.error.slice(0, 300)` when set.
      - **cancelled:** `p` `run.cancelled_text`.
    - `paintChart`: `chart.update(detail.metrics, detail.maxSteps, liveNote(run, detail.metrics.length) ? t(liveNote(...)) : '')`.
    - `paintFiles`:
      - `h4` `run.files_title` and `ul.csn-files`;
      - each item is `li.csn-file[data-csn=artifact][data-kind]` with `span.csn-file-name` (name), `span.csn-muted` (`ARTIFACT_KEYS[kind] || other`), `span.csn-muted` (`format.bytes`) and `a.csn-btn[data-csn=artifact-download][download][href=inst.api.artifactURL(run.id, a.id)][aria-label=run.download_label {name}]` with text `run.download`;
      - `run.files_empty` when the list is empty.
    - `paintLeftovers` (when `run.leftovers.length`): `h4` `run.leftovers_title`, `p` `run.leftovers_text`, and one item per leftover with the kind word, `code` ref and `P.linkRow(t, url, 'leftover-' + i)`.
    - `paintEvents`: newest last; `li.csn-event[data-kind]` with time (`format.dateTime`) and `eventText`. Empty texts are skipped; `run.events_empty` when nothing is left.
    - `paintJobs`: `run.jobs_title` summary, then `p` `run.notebook_title` with `P.linkRow(t, remote.notebook_url, 'run-notebook')` when set, then `ul` of `run.job_line {step, id}` from `remote.kaggle_jobs` (the full run only).
    - `loadLog` on first open: `pending`, then `inst.api.runLog(id)` → `pre.textContent = body.tail || ''` or `run.log_empty`.
  - Publish `P.run = { createArea, timeline, reachedPhase, mergeMetrics, elapsedSeconds, liveNote, eventText, PHASES, STAGES }`.
- [ ] 4. Mount in `chibi-sensei-training.js`. `create` already holds `const runArea = P.run ? P.run.createArea(inst) : null;` and `arrange()` from F2. Confirm the area is mounted and that `dispose` disposes it. Nothing else changes.
- [ ] 5. `chibi-sensei.js` live wiring.
  - In `openEvents`, add after `onChecks`:
```js
            onRun: live(run => applyRun(inst, run)),
            onMetric: live(body => inst.bus.emit('metric', body)),
            onStage: live(body => {
                const prev = inst.runs.find(r => r.id === body.run_id);
                if (prev) applyRun(inst, { id: prev.id, stage: Object.assign({}, prev.stage || {}, { name: String(body.stage || ''), state: String(body.state || '') }) });
                inst.bus.emit('stage', body);
            })
```
  - `syncPoll`: `const any = Array.from(inst.jobs.values()).some(isActive) || !!P.activeRun(inst.runs);`
  - `pollJobs`: wrap the existing `try { … } catch` in `if (known.length) { … }`, then before the final `syncPoll` add `await pollRun(inst, seq);`. New function:
```js
    // The safety poll of the active run (the event stream is the fast path; a missed event is caught here within 2.5 s).
    async function pollRun(inst, seq) {
        const active = P.activeRun(inst.runs);
        if (!active || inst.disposed || seq !== inst.projectSeq) return;
        try {
            const run = await inst.api.getRun(active.id);
            if (inst.disposed || seq !== inst.projectSeq) return;
            applyRun(inst, run);
        } catch (err) {
            if (inst.disposed || seq !== inst.projectSeq || (err && err.aborted)) return;
            const e = P.errors.normalize(err);
            if (e.status === 404 || e.code === 'run_not_found') removeRun(inst, active.id);
        }
    }
```
  - Disposal is unchanged: `closeProject` closes the events, clears the poll timer and disposes the view, which disposes the run area and the chart.
- [ ] 6. CSS. Append (tokens only; the motion sits in a second reduced-motion gate):
```css
/* Run monitor (P2): head, timeline, stages, result, chart, files, events, log, history */
.csn-runs { display: grid; gap: var(--ds-space-3); }
.csn-run-head { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); }
.csn-run-head h3 { margin: 0 auto 0 0; }
.csn-run-state[data-state="succeeded"] { border-color: var(--csn-ok); color: var(--csn-ok-text); }
.csn-run-state[data-state="failed"] { border-color: var(--csn-bad); color: var(--csn-bad-text); }
.csn-run-status { margin: 0; font-weight: 600; }
.csn-run-meta { margin: 0; color: var(--vd-theme-muted); font-size: 13px; }
.csn-timeline { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 4px; margin: 0; padding: 0; list-style: none; }
.csn-phase { display: grid; gap: 4px; min-width: 0; font-size: 12px; }
.csn-phase::before { content: ""; display: block; height: 6px; border-radius: var(--ds-radius-full); background: var(--csn-line); }
.csn-phase[data-status="done"]::before { background: var(--csn-ok); }
.csn-phase[data-status="active"]::before { background: var(--vd-accent); }
.csn-phase[data-status="failed"]::before { background: var(--csn-bad); }
.csn-phase[data-status="stopped"]::before { background: var(--csn-warn); }
.csn-phase[data-status="skipped"] { opacity: .6; }
.csn-phase-name { font-weight: 600; overflow-wrap: anywhere; }
.csn-phase-state { color: var(--vd-theme-muted); }
.csn-stages { display: flex; flex-wrap: wrap; gap: 4px; margin: 0; padding: 0; list-style: none; }
.csn-stage { padding: 0 8px; border: 1px solid var(--csn-line); border-radius: var(--ds-radius-full); color: var(--vd-theme-muted); font-size: 12px; }
.csn-stage[data-status="done"] { border-color: var(--csn-ok); color: var(--csn-ok-text); }
.csn-stage[data-status="active"] { border-color: var(--vd-accent); color: var(--vd-text); font-weight: 600; }
.csn-stage[data-status="failed"] { border-color: var(--csn-bad); color: var(--csn-bad-text); }
.csn-result { display: grid; gap: 4px; padding: var(--ds-space-2) var(--ds-space-3); border: 1px solid var(--csn-line); border-left: 3px solid var(--csn-ok); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-well, var(--vd-theme-control-bg)); }
.csn-result:empty { display: none; }
.csn-result[data-outcome="failed"] { border-left-color: var(--csn-bad); }
.csn-result[data-outcome="cancelled"] { border-left-color: var(--csn-warn); }
.csn-result h4, .csn-result p { margin: 0; }
.csn-chart { display: grid; gap: 4px; margin: 0; }
.csn-chart-head { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-3); }
.csn-chart-svg { width: 100%; height: auto; max-height: 260px; }
.csn-chart[data-empty="1"] .csn-chart-svg, .csn-chart[data-empty="1"] .csn-chart-legend { display: none; }
.csn-chart-gridline { stroke: var(--csn-line); stroke-width: 1; }
.csn-chart-axis { fill: var(--vd-theme-muted); font-size: 11px; font-family: var(--vd-font-ui, system-ui, sans-serif); }
.csn-chart-train { fill: none; stroke: var(--vd-accent); stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
.csn-chart-dot { fill: var(--vd-accent); }
.csn-chart-eval { fill: var(--vd-mat-card, var(--vd-theme-panel-bg)); stroke: var(--csn-warn); stroke-width: 2; }
.csn-chart-legend { display: flex; flex-wrap: wrap; gap: var(--ds-space-3); font-size: 12px; }
.csn-chart-key::before { content: ""; display: inline-block; width: 14px; height: 3px; margin-right: 6px; vertical-align: middle; background: var(--vd-accent); }
.csn-chart-key[data-kind="eval"]::before { width: 8px; height: 8px; border: 2px solid var(--csn-warn); border-radius: var(--ds-radius-full); background: transparent; }
.csn-chart-summary { margin: 0; font-size: 13px; }
.csn-files { display: grid; gap: 4px; margin: 0; padding: 0; list-style: none; }
.csn-file { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); padding: 6px var(--ds-space-2); border: 1px solid var(--csn-line); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-well, var(--vd-theme-control-bg)); }
.csn-file-name { margin-right: auto; font-weight: 600; overflow-wrap: anywhere; }
a.csn-btn { text-decoration: none; }
.csn-events { display: grid; gap: 2px; margin: 0; padding: 0; list-style: none; font-size: 13px; }
.csn-event { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: var(--ds-space-2); }
.csn-event[data-kind="error"] { color: var(--csn-bad-text); }
.csn-event[data-kind="warn"] { color: var(--csn-warn-text); }
.csn-event-time { color: var(--vd-theme-muted); font-variant-numeric: tabular-nums; }
.csn-log { max-height: 320px; margin: 0; padding: var(--ds-space-2); overflow: auto; border: 1px solid var(--csn-line); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-well, var(--vd-theme-control-bg)); font: 12px/1.4 var(--csn-mono); white-space: pre-wrap; overflow-wrap: anywhere; }
.csn-run-list { display: grid; gap: 4px; margin: 0; padding: 0; list-style: none; }
.csn-run-item { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); width: 100%; padding: 6px var(--ds-space-2); border: 1px solid var(--csn-line); border-radius: var(--vd-mat-btn-radius, var(--ds-radius-sm)); background: var(--vd-mat-well, var(--vd-theme-control-bg)); text-align: left; cursor: pointer; }
.csn-run-item:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.csn-run-item[aria-pressed="true"] { border-color: var(--vd-mat-selected-color, var(--vd-theme-accent-border)); background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); color: var(--vd-mat-selected-text, inherit); }
.csn-run-item-state { margin-left: auto; font-size: 12px; font-weight: 600; }
.csn-run-item[data-state="failed"] .csn-run-item-state { color: var(--csn-bad-text); }
.csn-run-item[data-state="succeeded"] .csn-run-item-state { color: var(--csn-ok-text); }
@container csn (max-width: 760px) {
    .csn-timeline { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (prefers-reduced-motion: no-preference) {
    body:not([data-animations="false"]) .csn-app .csn-phase[data-status="active"]::before { animation: csn-pulse 1.6s ease-in-out infinite; }
}
```
- [ ] 7. Locales: add the families marked **F3** in Task F4 to `en.json` and `de.json`.
- [ ] 8. Registration edits (Contract change 4):
  - `module-loader.js:39`: insert `'/js/desktop/apps/chibi-sensei/chibi-sensei-chart.js', '/js/desktop/apps/chibi-sensei/chibi-sensei-run.js'` between `-training.js` and `chibi-sensei.js`.
  - `ui/desktop_chibi_sensei_registration_test.go:13`: `{…, "training", "chart", "run"}`.
  - `ui/desktop_chibi_sensei_guards_test.go:49-50`: `!= 12` and "want exactly 12".
  - `ui/i18n_lint_test.go` after line 1121: two entries `"js/desktop/apps/chibi-sensei/chibi-sensei-chart.js": {name: "t", prefix: "chibi_sensei."},` and the same for `-run.js`, then `gofmt -w ui/i18n_lint_test.go`.
  - `module-loader.js` is not part of `main.bundle.js` (no bundle contains `chibi-sensei-training.js`), so no rebuild is needed; `npm run check:ui` confirms it.
- [ ] 9. `scripts/test-chibi-sensei-model.mjs`: append `'chibi-sensei-chart.js', 'chibi-sensei-run.js'` to `files`, then add before the final `console.log`:
```js
// ---- run timeline, metrics, events ------------------------------------------------------------------------------------------
const R = P.run, C = P.chart;
const phases = run => plain(R.timeline(run).phases.map(x => x.status));
eq(phases({ state: 'uploading' }), ['done', 'active', 'todo', 'todo', 'todo', 'todo'], 'timeline: uploading');
const tl = plain(R.timeline({ state: 'running', stage: { name: 'train', state: 'start' } }));
eq(tl.phases.map(x => x.status), ['done', 'done', 'done', 'active', 'todo', 'todo'], 'timeline: running');
eq(tl.stages.slice(5, 8).map(x => x.status), ['done', 'active', 'todo'], 'timeline: stages around train');
eq(phases({ state: 'succeeded' }), ['done', 'done', 'done', 'done', 'done', 'done'], 'timeline: succeeded');
eq(phases({ state: 'failed', last_state: 'waiting_dataset' }), ['done', 'failed', 'skipped', 'skipped', 'skipped', 'skipped'], 'timeline: failed while waiting');
eq(phases({ state: 'cancelled', last_state: 'queued' }), ['stopped', 'skipped', 'skipped', 'skipped', 'skipped', 'skipped'], 'timeline: cancelled early');
eq(phases({ state: 'failed', error_code: 'download_failed' }), ['done', 'done', 'done', 'done', 'failed', 'skipped'], 'timeline: fallback by code');
const gg = plain(R.timeline({ state: 'failed', stage: { name: 'gguf', state: 'failed' } }));
ok(gg.phases[3].status === 'failed' && gg.stages[9].status === 'failed' && gg.stages[10].status === 'skipped', 'timeline: failed in gguf');
eq(plain(R.mergeMetrics([{ step: 20, kind: 'train', loss: 1 }], [{ step: 10, kind: 'train', loss: 2 }, { step: 20, kind: 'train', loss: 0.9 }, { step: 20, kind: 'eval', loss: 1.1 }]))
    .map(m => m.kind + m.step + ':' + m.loss), ['train10:2', 'train20:0.9', 'eval20:1.1'], 'mergeMetrics');
eq(R.elapsedSeconds({ state: 'running', created: '2026-11-23T10:00:00Z' }, Date.parse('2026-11-23T10:05:00Z')), 300, 'elapsed since creation');
eq(R.elapsedSeconds({ state: 'succeeded', remote_started: '2026-11-23T10:00:00Z', finished: '2026-11-23T10:01:00Z' }, 0), 60, 'elapsed of a finished run');
eq([R.liveNote({ state: 'running', live: 'no' }, 0), R.liveNote({ state: 'running', live: 'unknown' }, 0), R.liveNote({ state: 'running', live: 'yes' }, 5), R.liveNote({ state: 'succeeded', live: 'no' }, 0)],
    ['chibi_sensei.run.live_off', 'chibi_sensei.run.live_waiting', '', ''], 'liveNote');
const tt = (k, vars) => k + (vars ? JSON.stringify(vars) : '');
eq(R.eventText(tt, { kind: 'stage', data: { stage: 'train', state: 'start' } }), 'chibi_sensei.event.stage_start{"stage":"chibi_sensei.stage.train"}', 'event: stage');
eq(R.eventText(tt, { kind: 'error', data: { code: 'oom' } }), 'chibi_sensei.event.error{"text":"chibi_sensei.fail.oom"}', 'event: error');
eq(R.eventText(tt, { kind: 'mystery', data: { x: 1 } }), '', 'event: unknown kinds are skipped');

// ---- chart layout --------------------------------------------------------------------------------------------------------
ok(C.layout([]).empty, 'chart: empty');
const pts = Array.from({ length: 1000 }, (_, i) => ({ step: (i + 1) * 2, kind: 'train', loss: 3 - i / 1000 }));
const L = plain(C.layout(pts.concat([{ step: 1000, kind: 'eval', loss: 2.4 }]), { maxSteps: 2000 }));
eq(L.trainCount, 500, 'chart: downsampled in buckets of two');
ok(L.path.charAt(0) === 'M' && (L.path.match(/L/g) || []).length === 499, 'chart: one command per point');
ok(L.summary.first === 3 && L.summary.steps === 2000 && Math.abs(L.summary.last - 2.001) < 1e-9 && Math.abs(L.summary.min - 2.001) < 1e-9, 'chart: summary from raw points');
eq(L.xTicks.map(x => x.value), [0, 500, 1000, 1500, 2000], 'chart: x ticks');
eq(L.evalPoints.length, 1, 'chart: eval point');
ok(L.yTicks.every(tk => tk.y >= L.plot.top && tk.y <= L.plot.bottom), 'chart: y ticks inside the plot');
const flat = plain(C.layout([{ step: 1, kind: 'train', loss: 1 }, { step: 2, kind: 'train', loss: 1 }]));
ok(!flat.empty && flat.yTicks[0].value < 1 && flat.yTicks[3].value > 1, 'chart: a flat line gets a range');
ok(C.layout([{ step: 1, kind: 'train', loss: NaN }, { step: 'x', loss: 1 }]).empty, 'chart: non-numbers are dropped');
eq(plain(C.layout([{ step: 0, kind: 'train', loss: 1 }, { step: 2, kind: 'train', loss: 2 }], { maxSteps: 2 })).xTicks.map(x => x.value), [0, 1, 2], 'chart: small step counts dedupe ticks');
```
- [ ] 10. Verify:
```bash
for f in chart run training; do node --check ui/js/desktop/apps/chibi-sensei/chibi-sensei-$f.js; done; node --check ui/js/desktop/apps/chibi-sensei/chibi-sensei.js ui/js/desktop/core/module-loader.js
node -e "for (const f of process.argv.slice(1)) JSON.parse(require('fs').readFileSync(f,'utf8'))" ui/lang/chibi_sensei/en.json ui/lang/chibi_sensei/de.json
node scripts/test-chibi-sensei-model.mjs
npm run check:ui
gofmt -l ui
git diff --check
go build ./...
go test ./ui -run '(?i)(ChibiSensei|I18n|Frontend_)' -skip '(?i)LocalesCoverAllSixteenLanguages' -count=1
```
  Expected:
  - `chibi-sensei model: ok (N checks)`;
  - `check:ui` exits 0 (if it reports drift, run `npm run build:ui` and run it again);
  - `gofmt -l ui` prints nothing;
  - `ok aurago/ui`.
  - Every module stays under 1100 lines: `wc -l ui/js/desktop/apps/chibi-sensei/*.js`.

---

### Task F4: Locales, tests, loader check, guides

**Files**
- Modify:
  - `ui/lang/chibi_sensei/{cs,da,el,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`
  - `ui/desktop_chibi_sensei_i18n_test.go:48-69` (only if needed)
  - `ui/config_chibi_sensei_test.go:24-31`, `:76`
  - `ui/kaggle_locales_test.go:35` (and a new test)
  - `ui/AGENTS.md:130`
  - `ui/js/desktop/apps/AGENTS.md:111-116`
  - `scripts/AGENTS.md` (the Chibi Sensei verification line)

**Existing symbols modified:** `TestConfigChibiSenseiSection`, `TestKaggleLocalesComplete`, `TestChibiSenseiTranslationsAreTranslated` (`same`/`sameIn`, only if a translator keeps an English word).

**Steps**
- [ ] 1. **Locale fragments.** F2 writes the families marked F2, F3 those marked F3. "Changed" means an existing key with a new value: the F4 translators must re-translate those too. Placeholders are single braces.

  **English (`ui/lang/chibi_sensei/en.json`)**
```json
{
  "_F2_changed": "",
  "chibi_sensei.training.title": "Train your model",
  "chibi_sensei.training.intro": "Check what is still missing, pick a preset and start a run on your Kaggle account. AuraGo uploads your examples to a private Kaggle dataset, trains on a free Kaggle GPU and downloads the finished model.",
  "chibi_sensei.training.estimate": "About {minutes} min of GPU time (estimate)",
  "chibi_sensei.training.advanced_text": "Sensei computes these values from your data and the base model. Change them only if you know why; an empty field keeps the automatic value.",
  "chibi_sensei.plan.training_later": "Sensei cannot compute a plan for this project yet.",
  "chibi_sensei.later.test_text": "Chatting with your model and comparing it with the base model arrive in a later version. Until then you can download the model file in the Training step.",
  "chibi_sensei.tip.data_ready": "Your dataset passes the checks. Continue to training when you are ready.",
  "chibi_sensei.tip.training_todo": "Finish the dataset first: training can start once the checks pass.",

  "_F2_new": "",
  "chibi_sensei.pending": "One moment…",
  "chibi_sensei.training.preset_title": "Preset",
  "chibi_sensei.training.preset_hint": "The preset decides how long and how carefully Sensei trains. The selected one is used when you start.",
  "chibi_sensei.training.timeout_help": "The session limit is the longest time Kaggle lets one run use the GPU. Kaggle stops the run there at the latest.",
  "chibi_sensei.training.eval_samples": "Evaluation: {count} examples",
  "chibi_sensei.training.download_size": "Download: about {mb} MB",
  "chibi_sensei.training.gpu": "Graphics card: {shape}",
  "chibi_sensei.tip.training_ready": "Look at the list “Before you start”. When nothing blocks, pick a preset and press Start training.",
  "chibi_sensei.tip.training_in_progress": "Your model is training on Kaggle. You can close this window; AuraGo keeps watching the run.",
  "chibi_sensei.tip.training_done": "Your model is ready. Download the GGUF file from the run below.",
  "chibi_sensei.tip.training_blocked": "The last run did not finish. Its card says why and what to try next.",
  "chibi_sensei.primary.show_run": "Show the run",
  "chibi_sensei.primary.train_again": "Train again",
  "chibi_sensei.primary.starting": "Preparing…",
  "chibi_sensei.primary.preflight_loading": "Sensei is checking what the run needs.",
  "chibi_sensei.primary.preflight_blocked": "Something in the list “Before you start” blocks the start.",
  "chibi_sensei.error.preflight_failed": "Something the run needs is missing now. The list was updated; check it and try again.",
  "chibi_sensei.error.plan_expired": "The confirmation expired or the plan changed. Sensei prepared a fresh one.",
  "chibi_sensei.error.run_active": "Another training run is still active. Only one run at a time is possible.",
  "chibi_sensei.error.run_not_terminal": "This run is still active. Stop it before you delete it.",
  "chibi_sensei.error.run_not_found": "This run no longer exists.",
  "chibi_sensei.error.artifact_not_found": "This file no longer exists.",
  "chibi_sensei.error.admin_required": "Only an administrator can change these settings.",
  "chibi_sensei.error.kaggle_unavailable": "Kaggle cannot be reached right now. Try again in a moment.",
  "chibi_sensei.error.license_consent_required": "Accept the license of the base model first.",
  "chibi_sensei.run.finished_notify": "Training finished.",
  "chibi_sensei.run.failed_notify": "Training failed.",
  "chibi_sensei.run.cancelled_notify": "Training stopped.",
  "chibi_sensei.preflight.title": "Before you start",
  "chibi_sensei.preflight.intro": "Sensei checks everything the run needs. Items marked “Blocks the start” must be fixed first; notes are optional.",
  "chibi_sensei.preflight.refresh": "Check again",
  "chibi_sensei.preflight.checking": "Checking…",
  "chibi_sensei.preflight.checked_at": "Checked {time}",
  "chibi_sensei.preflight.failed": "Sensei could not check the requirements.",
  "chibi_sensei.preflight.summary_ready": "Everything the run needs is in place.",
  "chibi_sensei.preflight.summary_blocked": "Items that block the start: {count}",
  "chibi_sensei.preflight.summary_warn": "Nothing blocks the start. Notes: {count}",
  "chibi_sensei.preflight.level_ok": "Met",
  "chibi_sensei.preflight.level_block": "Blocks the start",
  "chibi_sensei.preflight.level_warn": "Note",
  "chibi_sensei.preflight.group_data": "Your data",
  "chibi_sensei.preflight.group_kaggle": "Kaggle settings in AuraGo",
  "chibi_sensei.preflight.group_account": "Kaggle account and this server",
  "chibi_sensei.preflight.group_quota": "GPU time",
  "chibi_sensei.preflight.group_storage": "Storage",
  "chibi_sensei.preflight.group_license": "License",
  "chibi_sensei.preflight.group_other": "Other",
  "chibi_sensei.preflight.quota_help": "Kaggle gives every account a free weekly allowance of GPU hours (the quota). Each run uses part of it.",
  "chibi_sensei.preflight.open_link": "Open settings",
  "chibi_sensei.preflight.unknown": "Another requirement",
  "chibi_sensei.preflight.flag_fix_admin": "You can switch this on with the button below.",
  "chibi_sensei.preflight.flag_fix_ask": "Ask an administrator to switch this on.",
  "chibi_sensei.preflight.dataset_ready": "The dataset passed the checks",
  "chibi_sensei.preflight.dataset_ready_fix": "Run the dataset check in the Data step and fix everything marked red.",
  "chibi_sensei.preflight.run_active": "No other training run is active",
  "chibi_sensei.preflight.run_active_fix": "Wait until the other run ends, or stop it.",
  "chibi_sensei.preflight.kaggle_enabled": "The Kaggle integration is switched on",
  "chibi_sensei.preflight.kaggle_not_readonly": "Kaggle is not in read-only mode",
  "chibi_sensei.preflight.kaggle_allow_create": "AuraGo may create datasets and notebooks on Kaggle",
  "chibi_sensei.preflight.kaggle_allow_remote_execute": "AuraGo may run notebooks on Kaggle",
  "chibi_sensei.preflight.kaggle_allow_gpu": "Kaggle runs may use a GPU",
  "chibi_sensei.preflight.kaggle_allow_internet": "Kaggle runs may use the internet (to download the base model)",
  "chibi_sensei.preflight.kaggle_allow_download": "AuraGo may download results from Kaggle",
  "chibi_sensei.preflight.kaggle_machine_shape": "The GPU type {shape} is allowed",
  "chibi_sensei.preflight.kaggle_allow_delete": "AuraGo may delete on Kaggle (to clean up after the run)",
  "chibi_sensei.preflight.kaggle_allow_delete_fix": "Without it, your uploaded data and the notebook stay on Kaggle after the run. You can delete them there yourself.",
  "chibi_sensei.preflight.network_access": "AuraGo may use the network",
  "chibi_sensei.preflight.network_access_fix": "An administrator must allow network requests for AuraGo.",
  "chibi_sensei.preflight.filesystem_write": "AuraGo may write files",
  "chibi_sensei.preflight.filesystem_write_fix": "An administrator must allow AuraGo to write files.",
  "chibi_sensei.preflight.kaggle_vault": "The secret store (Vault) is available",
  "chibi_sensei.preflight.kaggle_vault_fix": "The Vault is locked or missing. An administrator must set the master key.",
  "chibi_sensei.preflight.kaggle_credentials": "Your Kaggle access key is saved",
  "chibi_sensei.preflight.kaggle_credentials_fix": "Save your Kaggle API token in the Kaggle settings.",
  "chibi_sensei.preflight.kaggle_connection": "AuraGo can sign in to Kaggle",
  "chibi_sensei.preflight.kaggle_connection_fix": "AuraGo tries the sign-in once the Kaggle settings above are in order. If it still fails, check the token in the Kaggle settings and test the connection there.",
  "chibi_sensei.preflight.kaggle_timeout": "The Kaggle time limit fits this run",
  "chibi_sensei.preflight.kaggle_timeout_fix": "The Kaggle time limit is {minutes} min, but this run may need {needed} min. Raise “Kaggle timeout” in the Kaggle settings or choose the Quick preset.",
  "chibi_sensei.preflight.kaggle_quota": "Enough weekly GPU time is left",
  "chibi_sensei.preflight.kaggle_quota_fix": "Your remaining weekly GPU time is too short for this run. Choose the Quick preset or wait until Kaggle renews the quota.",
  "chibi_sensei.preflight.kaggle_quota_warn_fix": "Kaggle did not say how much GPU time is left. The run may stop early if your quota runs out.",
  "chibi_sensei.preflight.kaggle_slot": "No other Kaggle run is busy",
  "chibi_sensei.preflight.kaggle_slot_fix": "Kaggle runs still active or unresolved: {jobs}. Wait for them, or release them in the Kaggle app.",
  "chibi_sensei.preflight.kaggle_transfer": "Kaggle storage and the transfer limit fit the model",
  "chibi_sensei.preflight.kaggle_transfer_fix": "Raise the transfer or storage limit in the Kaggle settings.",
  "chibi_sensei.preflight.disk_space": "This server has room for the model",
  "chibi_sensei.preflight.disk_space_fix": "Free some space or raise the Chibi Sensei storage limit.",
  "chibi_sensei.preflight.license_consent": "The license of the base model is accepted",
  "chibi_sensei.preflight.license_consent_fix": "Read the license below and accept it.",
  "chibi_sensei.enable.button": "Enable for Chibi Sensei",
  "chibi_sensei.enable.lead_admin": "As an administrator you can switch the missing Kaggle settings on here.",
  "chibi_sensei.enable.title": "Change Kaggle settings",
  "chibi_sensei.enable.intro": "AuraGo will change these Kaggle settings for the whole installation, not only for Chibi Sensei. You can switch each of them off again in the Kaggle settings.",
  "chibi_sensei.enable.required": "Needed for training",
  "chibi_sensei.enable.optional": "Optional",
  "chibi_sensei.enable.confirm": "Change these settings",
  "chibi_sensei.enable.cancel": "Cancel",
  "chibi_sensei.enable.applied": "Kaggle settings changed: {count}",
  "chibi_sensei.enable.none_selected": "Choose at least one setting.",
  "chibi_sensei.enable.ask_title": "Ask an administrator",
  "chibi_sensei.enable.ask_text": "Only an administrator can change Kaggle settings. Ask for these:",
  "chibi_sensei.consent.title": "License of the base model",
  "chibi_sensei.consent.text": "{model} is published under the {license}. Training and using it means you accept its terms.",
  "chibi_sensei.consent.link_label": "License text",
  "chibi_sensei.consent.accept_label": "I have read the {license} and accept it.",
  "chibi_sensei.consent.accept": "Accept the license",
  "chibi_sensei.consent.accepted": "You accepted the {license}.",
  "chibi_sensei.consent.saved": "License accepted.",
  "chibi_sensei.link.address": "Link address",
  "chibi_sensei.link.copy": "Copy link",
  "chibi_sensei.link.copied": "Link copied.",
  "chibi_sensei.link.copy_failed": "Copying failed. The link is selected; copy it by hand.",
  "chibi_sensei.link.open": "Open in a new tab",
  "chibi_sensei.link.blocked": "This link cannot be opened from here.",
  "chibi_sensei.shape.NvidiaTeslaT4": "NVIDIA T4 graphics card (16 GB)",
  "chibi_sensei.shape.NvidiaL4": "NVIDIA L4 graphics card (24 GB)",
  "chibi_sensei.overrides.method": "Method",
  "chibi_sensei.overrides.method_auto": "Automatic ({value})",
  "chibi_sensei.overrides.epochs": "Epochs (1–10)",
  "chibi_sensei.overrides.epochs_help": "How often the model sees every example.",
  "chibi_sensei.overrides.lr": "Learning rate (0.000001–0.001)",
  "chibi_sensei.overrides.lr_help": "How big each learning step is. Too high makes training unstable.",
  "chibi_sensei.overrides.seq": "Maximum length (tokens)",
  "chibi_sensei.overrides.seq_help": "Longer examples are cut at this length.",
  "chibi_sensei.overrides.lora_r": "LoRA rank (1–256)",
  "chibi_sensei.overrides.lora_r_help": "LoRA trains a small add-on instead of the whole model; the rank is the size of that add-on.",
  "chibi_sensei.overrides.lora_alpha": "LoRA alpha (1–512)",
  "chibi_sensei.overrides.lora_alpha_help": "How strongly the add-on acts. Usually twice the rank.",
  "chibi_sensei.overrides.auto": "Automatic: {value}",
  "chibi_sensei.overrides.auto_option": "Automatic",
  "chibi_sensei.overrides.save": "Save values",
  "chibi_sensei.overrides.reset": "Back to automatic",
  "chibi_sensei.overrides.saved": "Training values saved.",
  "chibi_sensei.overrides.fix_fields": "Check the marked fields.",
  "chibi_sensei.overrides.bad_method": "This base model does not support this method.",
  "chibi_sensei.overrides.bad_epochs": "Enter a whole number from 1 to 10.",
  "chibi_sensei.overrides.bad_lr": "Enter a number from 0.000001 to 0.001.",
  "chibi_sensei.overrides.bad_seq": "Choose one of the listed lengths.",
  "chibi_sensei.overrides.bad_lora_r": "Enter a whole number from 1 to 256.",
  "chibi_sensei.overrides.bad_lora_alpha": "Enter a whole number from 1 to 512.",
  "chibi_sensei.confirm.title": "Start training on Kaggle?",
  "chibi_sensei.confirm.data_title": "What leaves this computer",
  "chibi_sensei.confirm.data_rows": "{train} training, {val} validation and {eval} evaluation examples ({size})",
  "chibi_sensei.confirm.data_private": "They go to a private dataset on your Kaggle account. Private means only you can see it; Kaggle does not publish it.",
  "chibi_sensei.confirm.dataset": "Kaggle dataset: {name}",
  "chibi_sensei.confirm.notebook": "Kaggle notebook: {name}",
  "chibi_sensei.confirm.notebook_help": "A private notebook on your account runs the training.",
  "chibi_sensei.confirm.license": "Base model license: {name}",
  "chibi_sensei.confirm.time_title": "Time and GPU quota",
  "chibi_sensei.confirm.estimate": "About {minutes} min of GPU time (estimate).",
  "chibi_sensei.confirm.session": "Session limit: {minutes} min. This is the longest time Kaggle lets the run use the GPU; Kaggle stops the run there at the latest.",
  "chibi_sensei.confirm.quota": "GPU time left this week: {hours} h. Kaggle gives every account a free weekly GPU allowance (the quota); this run uses part of it.",
  "chibi_sensei.confirm.quota_unknown": "Kaggle did not say how much GPU time is left this week.",
  "chibi_sensei.confirm.cancel_title": "If you stop the run",
  "chibi_sensei.confirm.cancel_text": "AuraGo stops waiting at once. GPU time ends when you stop the notebook on Kaggle, and at the latest at the run's time limit ({minutes} min).",
  "chibi_sensei.confirm.after_title": "After the run",
  "chibi_sensei.confirm.after_delete": "AuraGo deletes the dataset and the notebook on Kaggle. The finished model stays here.",
  "chibi_sensei.confirm.after_keep_policy": "Deleting on Kaggle is not allowed in the Kaggle settings, so the dataset and the notebook stay on your Kaggle account. The run lists them so you can delete them yourself.",
  "chibi_sensei.confirm.after_keep_setting": "The dataset and the notebook stay on your Kaggle account (Chibi Sensei setting). The run lists them.",
  "chibi_sensei.confirm.valid": "This confirmation is valid for {minutes} min.",
  "chibi_sensei.confirm.expired": "The confirmation had expired or the plan changed, so Sensei prepared a fresh one. Check the details and start again.",
  "chibi_sensei.confirm.start": "Start training",
  "chibi_sensei.confirm.cancel": "Cancel",
  "chibi_sensei.confirm.started": "Training started.",

  "_F3_new": "",
  "chibi_sensei.run_state.queued": "Waiting to start",
  "chibi_sensei.run_state.packaging": "Packing your examples",
  "chibi_sensei.run_state.uploading": "Uploading your examples to Kaggle",
  "chibi_sensei.run_state.creating_dataset": "Creating the private dataset on Kaggle",
  "chibi_sensei.run_state.waiting_dataset": "Waiting for Kaggle to prepare the dataset",
  "chibi_sensei.run_state.starting": "Starting the notebook on Kaggle",
  "chibi_sensei.run_state.running": "Training on Kaggle",
  "chibi_sensei.run_state.downloading": "Downloading the results",
  "chibi_sensei.run_state.verifying": "Checking the downloaded model",
  "chibi_sensei.run_state.cleaning": "Cleaning up on Kaggle",
  "chibi_sensei.run_state.succeeded": "Finished",
  "chibi_sensei.run_state.failed": "Failed",
  "chibi_sensei.run_state.cancelled": "Stopped",
  "chibi_sensei.phase.prepare": "Prepare",
  "chibi_sensei.phase.upload": "Upload",
  "chibi_sensei.phase.start": "Start on Kaggle",
  "chibi_sensei.phase.train": "Train",
  "chibi_sensei.phase.download": "Fetch the results",
  "chibi_sensei.phase.cleanup": "Clean up",
  "chibi_sensei.phase_state.done": "done",
  "chibi_sensei.phase_state.active": "in progress",
  "chibi_sensei.phase_state.failed": "failed",
  "chibi_sensei.phase_state.stopped": "stopped",
  "chibi_sensei.phase_state.todo": "waiting",
  "chibi_sensei.phase_state.skipped": "skipped",
  "chibi_sensei.stage.boot": "Start the script",
  "chibi_sensei.stage.env_check": "Check the GPU and the internet",
  "chibi_sensei.stage.install": "Install the tools",
  "chibi_sensei.stage.data": "Read your examples",
  "chibi_sensei.stage.load_base": "Download the base model",
  "chibi_sensei.stage.base_eval": "Test the base model",
  "chibi_sensei.stage.train": "Learn from your examples",
  "chibi_sensei.stage.tuned_eval": "Test the trained model",
  "chibi_sensei.stage.save": "Save the model",
  "chibi_sensei.stage.gguf": "Convert to GGUF",
  "chibi_sensei.stage.manifest": "List the files",
  "chibi_sensei.stage.unknown": "Other step",
  "chibi_sensei.run.title": "Current run",
  "chibi_sensei.run.title_past": "Run of {date}",
  "chibi_sensei.run.timeline": "Progress of the run",
  "chibi_sensei.run.stages": "Steps inside the notebook",
  "chibi_sensei.run.history_title": "Runs of this project",
  "chibi_sensei.run.history_empty": "No runs yet. Start one above.",
  "chibi_sensei.run.item_label": "Run of {date}, {preset}, {state}",
  "chibi_sensei.run.status_kaggle": "Kaggle reports: {status}",
  "chibi_sensei.run.elapsed": "Running for {time}",
  "chibi_sensei.run.estimate": "estimate about {minutes} min",
  "chibi_sensei.run.longer": "This takes longer than estimated. Kaggle stops it at the session limit ({minutes} min) at the latest.",
  "chibi_sensei.run.progress": "Step {step} of {max}, epoch {epoch}",
  "chibi_sensei.run.eta": "about {time} left",
  "chibi_sensei.run.live_waiting": "Waiting for the first numbers from Kaggle…",
  "chibi_sensei.run.live_off": "Live numbers are not available for this run; the chart appears when the run ends.",
  "chibi_sensei.run.cancel": "Stop run",
  "chibi_sensei.run.cancel_title": "Stop this run?",
  "chibi_sensei.run.cancel_text": "AuraGo stops waiting at once. GPU time ends when you stop the notebook on Kaggle, and at the latest at the run's time limit ({minutes} min).",
  "chibi_sensei.run.cancel_requested": "Stop requested. AuraGo no longer waits for this run. GPU time ends when the notebook stops on Kaggle, at the latest after {minutes} min.",
  "chibi_sensei.run.stop_title": "Stop it on Kaggle",
  "chibi_sensei.run.stop_text": "Kaggle did not accept the stop request from AuraGo. Open the notebook on Kaggle and stop the session there. AuraGo keeps watching until it ends.",
  "chibi_sensei.run.stopped_local": "Stopped before anything ran on Kaggle. Uploaded files are cleaned up.",
  "chibi_sensei.run.result_title": "Result",
  "chibi_sensei.run.result_steps": "{steps} training steps in {epochs} epochs",
  "chibi_sensei.run.train_loss": "Final training loss: {value}",
  "chibi_sensei.run.eval_loss": "Evaluation loss: {base} before, {tuned} after training",
  "chibi_sensei.run.loss_help": "The loss measures how far the model's answers are from your expected answers. Lower is better.",
  "chibi_sensei.run.exact": "Exact answers: {base} before, {tuned} after ({n} examples)",
  "chibi_sensei.run.labels": "Correct labels: {base} before, {tuned} after ({n} examples)",
  "chibi_sensei.run.perplexity": "Perplexity: {base} before, {tuned} after (lower is better)",
  "chibi_sensei.run.small_sample": "Only {n} evaluation examples: treat these numbers as a rough hint.",
  "chibi_sensei.run.gpu_used": "Graphics card used: {name}",
  "chibi_sensei.run.failed_title": "What went wrong",
  "chibi_sensei.run.fix_title": "What you can try",
  "chibi_sensei.run.cancelled_text": "This run was stopped.",
  "chibi_sensei.run.warning_title": "Note",
  "chibi_sensei.run.files_title": "Files",
  "chibi_sensei.run.files_empty": "This run has no files.",
  "chibi_sensei.run.download": "Download",
  "chibi_sensei.run.download_label": "Download {name}",
  "chibi_sensei.run.leftovers_title": "Left on Kaggle",
  "chibi_sensei.run.leftovers_text": "These stayed on your Kaggle account. Delete them on Kaggle when you no longer need them.",
  "chibi_sensei.run.delete": "Delete run",
  "chibi_sensei.run.delete_title": "Delete this run?",
  "chibi_sensei.run.delete_text": "This removes the run and its downloaded files from this computer. Nothing on Kaggle changes.",
  "chibi_sensei.run.deleted": "Run deleted.",
  "chibi_sensei.run.log_title": "Kaggle log (last 64 KB)",
  "chibi_sensei.run.log_empty": "Kaggle sent no log for this run.",
  "chibi_sensei.run.events_title": "What happened",
  "chibi_sensei.run.events_empty": "Nothing yet.",
  "chibi_sensei.run.jobs_title": "Kaggle details",
  "chibi_sensei.run.notebook_title": "Notebook on Kaggle",
  "chibi_sensei.run.job_line": "Step {step}, job {id}",
  "chibi_sensei.event.state": "Now: {state}",
  "chibi_sensei.event.stage_start": "Started: {stage}",
  "chibi_sensei.event.stage_done": "Done: {stage}",
  "chibi_sensei.event.stage_failed": "Failed: {stage}",
  "chibi_sensei.event.warn": "Note: {text}",
  "chibi_sensei.event.error": "Error: {text}",
  "chibi_sensei.event.kaggle": "Kaggle job for step {step}",
  "chibi_sensei.fail.env_no_gpu": "Kaggle gave the notebook no GPU.",
  "chibi_sensei.fail.env_no_gpu_fix": "Check that your Kaggle account is phone-verified and has GPU quota left, then start again.",
  "chibi_sensei.fail.env_unsupported_gpu": "Kaggle assigned a graphics card this model cannot use.",
  "chibi_sensei.fail.env_unsupported_gpu_fix": "Choose another GPU type in the Chibi Sensei settings, for example the T4, and start again.",
  "chibi_sensei.fail.env_no_internet": "The notebook had no internet access.",
  "chibi_sensei.fail.env_no_internet_fix": "Kaggle allows internet only for phone-verified accounts. Verify your phone number on Kaggle and start again.",
  "chibi_sensei.fail.install_failed": "Installing the training tools failed.",
  "chibi_sensei.fail.install_failed_fix": "This is often a short problem at the package server. Start again later.",
  "chibi_sensei.fail.data_mismatch": "The uploaded examples did not match what AuraGo sent.",
  "chibi_sensei.fail.data_mismatch_fix": "Start again; AuraGo uploads a fresh copy.",
  "chibi_sensei.fail.base_download_failed": "Downloading the base model failed.",
  "chibi_sensei.fail.base_download_failed_fix": "Hugging Face may be busy. Start again later.",
  "chibi_sensei.fail.oom": "The graphics card ran out of memory.",
  "chibi_sensei.fail.oom_fix": "Choose the Quick preset or a shorter maximum length under Advanced.",
  "chibi_sensei.fail.train_diverged": "Training became unstable: the loss was no longer a number.",
  "chibi_sensei.fail.train_diverged_fix": "Lower the learning rate under Advanced and start again.",
  "chibi_sensei.fail.merge_failed": "Saving the trained model failed.",
  "chibi_sensei.fail.merge_failed_fix": "Start again. If it fails again, choose another base model.",
  "chibi_sensei.fail.gguf_failed": "Converting the model to GGUF failed.",
  "chibi_sensei.fail.gguf_failed_fix": "Start again. The Kaggle log below shows details.",
  "chibi_sensei.fail.gguf_arch_mismatch": "The converted model has an unexpected architecture.",
  "chibi_sensei.fail.gguf_arch_mismatch_fix": "The converter does not support this base model yet. Choose another base model.",
  "chibi_sensei.fail.kaggle_submit_unknown": "It is unclear whether Kaggle started the notebook.",
  "chibi_sensei.fail.kaggle_submit_unknown_fix": "Look at your notebooks on Kaggle. Start again if nothing runs there.",
  "chibi_sensei.fail.remote_timeout": "The run did not finish within its time limit.",
  "chibi_sensei.fail.remote_timeout_fix": "Choose the Quick preset or raise the Kaggle timeout in the Kaggle settings.",
  "chibi_sensei.fail.status_missing": "The run ended without a result report.",
  "chibi_sensei.fail.status_missing_fix": "The Kaggle log below shows the last lines. Start again.",
  "chibi_sensei.fail.download_failed": "Downloading the results failed.",
  "chibi_sensei.fail.download_failed_fix": "Start again. If space is short, raise the storage limits.",
  "chibi_sensei.fail.checksum_mismatch": "A downloaded file was damaged.",
  "chibi_sensei.fail.checksum_mismatch_fix": "Start again; AuraGo keeps no damaged files.",
  "chibi_sensei.fail.lora_targets_missing": "The training found no layers to adapt in this base model.",
  "chibi_sensei.fail.lora_targets_missing_fix": "Choose another base model, or the full fine-tune method under Advanced if the model offers it.",
  "chibi_sensei.fail.cancelled_by_user": "You stopped this run.",
  "chibi_sensei.fail.cancelled_by_user_fix": "Start a new run whenever you like.",
  "chibi_sensei.fail.desktop_readonly": "The desktop switched to read-only mode, so the run was stopped.",
  "chibi_sensei.fail.desktop_readonly_fix": "Start again when the desktop allows changes.",
  "chibi_sensei.fail.unknown": "The run failed.",
  "chibi_sensei.fail.unknown_fix": "The events and the Kaggle log below show what happened.",
  "chibi_sensei.warning.time_budget": "Training stopped at its time budget. The model uses what it learned until then.",
  "chibi_sensei.warning.base_gguf_skipped": "No time was left to convert the base model for comparisons. Your trained model is complete.",
  "chibi_sensei.warning.unknown": "The run finished with a note.",
  "chibi_sensei.artifact.gguf_tuned": "Your model (GGUF)",
  "chibi_sensei.artifact.gguf_base": "Base model for comparing (GGUF)",
  "chibi_sensei.artifact.adapter": "LoRA adapter",
  "chibi_sensei.artifact.adapter_config": "Adapter settings",
  "chibi_sensei.artifact.metrics": "Training numbers",
  "chibi_sensei.artifact.eval_samples": "Evaluation answers",
  "chibi_sensei.artifact.status": "Run report",
  "chibi_sensei.artifact.log": "Kaggle log",
  "chibi_sensei.artifact.other": "Other file",
  "chibi_sensei.leftover.dataset": "Kaggle dataset",
  "chibi_sensei.leftover.notebook": "Kaggle notebook",
  "chibi_sensei.leftover.other": "Other Kaggle item",
  "chibi_sensei.chart.title": "Loss during training",
  "chibi_sensei.chart.train": "Training loss",
  "chibi_sensei.chart.eval": "Validation loss",
  "chibi_sensei.chart.axis_step": "Step",
  "chibi_sensei.chart.empty": "No numbers yet.",
  "chibi_sensei.chart.summary": "Training loss went from {first} to {last} in {steps} steps; lowest value {min}.",
  "chibi_sensei.chart.summary_eval": "Last validation loss: {value}."
}
```
  The `"_F2_changed"`, `"_F2_new"` and `"_F3_new"` lines are batch markers for this plan only. Never write them into a file.

  **German (`ui/lang/chibi_sensei/de.json`)**
```json
{
  "_F2_changed": "",
  "chibi_sensei.training.title": "Modell trainieren",
  "chibi_sensei.training.intro": "Prüfe, was noch fehlt, wähle eine Voreinstellung und starte einen Lauf auf Deinem Kaggle-Konto. AuraGo lädt Deine Beispiele in einen privaten Kaggle-Datensatz, trainiert auf einer kostenlosen Kaggle-GPU und lädt das fertige Modell herunter.",
  "chibi_sensei.training.estimate": "Etwa {minutes} Min. GPU-Zeit (Schätzung)",
  "chibi_sensei.training.advanced_text": "Sensei berechnet diese Werte aus Deinen Daten und dem Basismodell. Ändere sie nur, wenn Du weißt warum; ein leeres Feld behält den automatischen Wert.",
  "chibi_sensei.plan.training_later": "Sensei kann für dieses Projekt noch keinen Plan berechnen.",
  "chibi_sensei.later.test_text": "Mit Deinem Modell chatten und es mit dem Basismodell vergleichen kommt in einer späteren Version. Bis dahin kannst Du die Modelldatei im Schritt Training herunterladen.",
  "chibi_sensei.tip.data_ready": "Dein Datensatz besteht die Prüfungen. Mach mit dem Training weiter, wenn Du bereit bist.",
  "chibi_sensei.tip.training_todo": "Mach zuerst den Datensatz fertig: Das Training kann starten, sobald die Prüfungen bestanden sind.",

  "_F2_new": "",
  "chibi_sensei.pending": "Einen Moment …",
  "chibi_sensei.training.preset_title": "Voreinstellung",
  "chibi_sensei.training.preset_hint": "Die Voreinstellung bestimmt, wie lange und wie gründlich Sensei trainiert. Die ausgewählte gilt beim Start.",
  "chibi_sensei.training.timeout_help": "Das Sitzungslimit ist die längste Zeit, die Kaggle einem Lauf die GPU überlässt. Spätestens dann stoppt Kaggle den Lauf.",
  "chibi_sensei.training.eval_samples": "Auswertung: {count} Beispiele",
  "chibi_sensei.training.download_size": "Download: etwa {mb} MB",
  "chibi_sensei.training.gpu": "Grafikkarte: {shape}",
  "chibi_sensei.tip.training_ready": "Sieh Dir die Liste „Bevor es losgeht“ an. Wenn nichts blockiert, wähle eine Voreinstellung und drücke Training starten.",
  "chibi_sensei.tip.training_in_progress": "Dein Modell trainiert auf Kaggle. Du kannst dieses Fenster schließen; AuraGo beobachtet den Lauf weiter.",
  "chibi_sensei.tip.training_done": "Dein Modell ist fertig. Lade die GGUF-Datei unten beim Lauf herunter.",
  "chibi_sensei.tip.training_blocked": "Der letzte Lauf wurde nicht fertig. Seine Karte sagt, warum, und was Du als Nächstes versuchen kannst.",
  "chibi_sensei.primary.show_run": "Lauf anzeigen",
  "chibi_sensei.primary.train_again": "Erneut trainieren",
  "chibi_sensei.primary.starting": "Wird vorbereitet …",
  "chibi_sensei.primary.preflight_loading": "Sensei prüft, was der Lauf braucht.",
  "chibi_sensei.primary.preflight_blocked": "Etwas in der Liste „Bevor es losgeht“ verhindert den Start.",
  "chibi_sensei.error.preflight_failed": "Etwas, das der Lauf braucht, fehlt jetzt. Die Liste wurde aktualisiert; prüfe sie und versuche es erneut.",
  "chibi_sensei.error.plan_expired": "Die Bestätigung ist abgelaufen oder der Plan hat sich geändert. Sensei hat einen neuen vorbereitet.",
  "chibi_sensei.error.run_active": "Ein anderer Trainingslauf ist noch aktiv. Es ist nur ein Lauf gleichzeitig möglich.",
  "chibi_sensei.error.run_not_terminal": "Dieser Lauf ist noch aktiv. Stoppe ihn, bevor Du ihn löschst.",
  "chibi_sensei.error.run_not_found": "Diesen Lauf gibt es nicht mehr.",
  "chibi_sensei.error.artifact_not_found": "Diese Datei gibt es nicht mehr.",
  "chibi_sensei.error.admin_required": "Nur eine Administratorin oder ein Administrator kann diese Einstellungen ändern.",
  "chibi_sensei.error.kaggle_unavailable": "Kaggle ist gerade nicht erreichbar. Versuche es gleich noch einmal.",
  "chibi_sensei.error.license_consent_required": "Akzeptiere zuerst die Lizenz des Basismodells.",
  "chibi_sensei.run.finished_notify": "Training abgeschlossen.",
  "chibi_sensei.run.failed_notify": "Training fehlgeschlagen.",
  "chibi_sensei.run.cancelled_notify": "Training gestoppt.",
  "chibi_sensei.preflight.title": "Bevor es losgeht",
  "chibi_sensei.preflight.intro": "Sensei prüft alles, was der Lauf braucht. Punkte mit „Verhindert den Start“ musst Du zuerst lösen; Hinweise sind optional.",
  "chibi_sensei.preflight.refresh": "Erneut prüfen",
  "chibi_sensei.preflight.checking": "Wird geprüft …",
  "chibi_sensei.preflight.checked_at": "Geprüft {time}",
  "chibi_sensei.preflight.failed": "Sensei konnte die Voraussetzungen nicht prüfen.",
  "chibi_sensei.preflight.summary_ready": "Alles, was der Lauf braucht, ist bereit.",
  "chibi_sensei.preflight.summary_blocked": "Punkte, die den Start verhindern: {count}",
  "chibi_sensei.preflight.summary_warn": "Nichts verhindert den Start. Hinweise: {count}",
  "chibi_sensei.preflight.level_ok": "Erfüllt",
  "chibi_sensei.preflight.level_block": "Verhindert den Start",
  "chibi_sensei.preflight.level_warn": "Hinweis",
  "chibi_sensei.preflight.group_data": "Deine Daten",
  "chibi_sensei.preflight.group_kaggle": "Kaggle-Einstellungen in AuraGo",
  "chibi_sensei.preflight.group_account": "Kaggle-Konto und dieser Server",
  "chibi_sensei.preflight.group_quota": "GPU-Zeit",
  "chibi_sensei.preflight.group_storage": "Speicher",
  "chibi_sensei.preflight.group_license": "Lizenz",
  "chibi_sensei.preflight.group_other": "Sonstiges",
  "chibi_sensei.preflight.quota_help": "Kaggle gibt jedem Konto ein kostenloses wöchentliches Kontingent an GPU-Stunden (die Quota). Jeder Lauf verbraucht einen Teil davon.",
  "chibi_sensei.preflight.open_link": "Einstellungen öffnen",
  "chibi_sensei.preflight.unknown": "Eine weitere Voraussetzung",
  "chibi_sensei.preflight.flag_fix_admin": "Du kannst das mit der Schaltfläche unten einschalten.",
  "chibi_sensei.preflight.flag_fix_ask": "Bitte eine Administratorin oder einen Administrator, das einzuschalten.",
  "chibi_sensei.preflight.dataset_ready": "Der Datensatz hat die Prüfungen bestanden",
  "chibi_sensei.preflight.dataset_ready_fix": "Starte die Datensatzprüfung im Schritt Daten und behebe alles, was rot markiert ist.",
  "chibi_sensei.preflight.run_active": "Kein anderer Trainingslauf ist aktiv",
  "chibi_sensei.preflight.run_active_fix": "Warte, bis der andere Lauf endet, oder stoppe ihn.",
  "chibi_sensei.preflight.kaggle_enabled": "Die Kaggle-Integration ist eingeschaltet",
  "chibi_sensei.preflight.kaggle_not_readonly": "Kaggle ist nicht im Nur-Lesen-Modus",
  "chibi_sensei.preflight.kaggle_allow_create": "AuraGo darf auf Kaggle Datensätze und Notebooks anlegen",
  "chibi_sensei.preflight.kaggle_allow_remote_execute": "AuraGo darf Notebooks auf Kaggle ausführen",
  "chibi_sensei.preflight.kaggle_allow_gpu": "Kaggle-Läufe dürfen eine GPU nutzen",
  "chibi_sensei.preflight.kaggle_allow_internet": "Kaggle-Läufe dürfen das Internet nutzen (um das Basismodell zu laden)",
  "chibi_sensei.preflight.kaggle_allow_download": "AuraGo darf Ergebnisse von Kaggle herunterladen",
  "chibi_sensei.preflight.kaggle_machine_shape": "Der GPU-Typ {shape} ist erlaubt",
  "chibi_sensei.preflight.kaggle_allow_delete": "AuraGo darf auf Kaggle löschen (um nach dem Lauf aufzuräumen)",
  "chibi_sensei.preflight.kaggle_allow_delete_fix": "Ohne diese Erlaubnis bleiben Deine hochgeladenen Daten und das Notebook nach dem Lauf auf Kaggle. Du kannst sie dort selbst löschen.",
  "chibi_sensei.preflight.network_access": "AuraGo darf das Netzwerk nutzen",
  "chibi_sensei.preflight.network_access_fix": "Eine Administratorin oder ein Administrator muss Netzwerkanfragen für AuraGo erlauben.",
  "chibi_sensei.preflight.filesystem_write": "AuraGo darf Dateien schreiben",
  "chibi_sensei.preflight.filesystem_write_fix": "Eine Administratorin oder ein Administrator muss AuraGo das Schreiben von Dateien erlauben.",
  "chibi_sensei.preflight.kaggle_vault": "Der Geheimnisspeicher (Vault) ist verfügbar",
  "chibi_sensei.preflight.kaggle_vault_fix": "Der Vault ist gesperrt oder fehlt. Eine Administratorin oder ein Administrator muss den Hauptschlüssel setzen.",
  "chibi_sensei.preflight.kaggle_credentials": "Dein Kaggle-Zugangsschlüssel ist gespeichert",
  "chibi_sensei.preflight.kaggle_credentials_fix": "Speichere Dein Kaggle-API-Token in den Kaggle-Einstellungen.",
  "chibi_sensei.preflight.kaggle_connection": "AuraGo kann sich bei Kaggle anmelden",
  "chibi_sensei.preflight.kaggle_connection_fix": "AuraGo versucht die Anmeldung, sobald die Kaggle-Einstellungen oben stimmen. Klappt es dann immer noch nicht, prüfe das Token in den Kaggle-Einstellungen und teste dort die Verbindung.",
  "chibi_sensei.preflight.kaggle_timeout": "Das Kaggle-Zeitlimit reicht für diesen Lauf",
  "chibi_sensei.preflight.kaggle_timeout_fix": "Das Kaggle-Zeitlimit ist {minutes} Min., dieser Lauf braucht aber vielleicht {needed} Min. Erhöhe „Kaggle-Zeitlimit“ in den Kaggle-Einstellungen oder wähle die Voreinstellung Schnell.",
  "chibi_sensei.preflight.kaggle_quota": "Es ist genug wöchentliche GPU-Zeit übrig",
  "chibi_sensei.preflight.kaggle_quota_fix": "Deine restliche wöchentliche GPU-Zeit reicht für diesen Lauf nicht. Wähle die Voreinstellung Schnell oder warte, bis Kaggle die Quota erneuert.",
  "chibi_sensei.preflight.kaggle_quota_warn_fix": "Kaggle hat nicht gesagt, wie viel GPU-Zeit übrig ist. Der Lauf kann früher enden, wenn Deine Quota aufgebraucht ist.",
  "chibi_sensei.preflight.kaggle_slot": "Kein anderer Kaggle-Lauf ist beschäftigt",
  "chibi_sensei.preflight.kaggle_slot_fix": "Noch aktive oder ungeklärte Kaggle-Läufe: {jobs}. Warte auf sie oder gib sie in der Kaggle-App frei.",
  "chibi_sensei.preflight.kaggle_transfer": "Kaggle-Speicher und Übertragungslimit reichen für das Modell",
  "chibi_sensei.preflight.kaggle_transfer_fix": "Erhöhe das Übertragungs- oder Speicherlimit in den Kaggle-Einstellungen.",
  "chibi_sensei.preflight.disk_space": "Dieser Server hat Platz für das Modell",
  "chibi_sensei.preflight.disk_space_fix": "Schaffe Platz oder erhöhe das Speicherlimit von Chibi Sensei.",
  "chibi_sensei.preflight.license_consent": "Die Lizenz des Basismodells ist akzeptiert",
  "chibi_sensei.preflight.license_consent_fix": "Lies die Lizenz unten und akzeptiere sie.",
  "chibi_sensei.enable.button": "Für Chibi Sensei einschalten",
  "chibi_sensei.enable.lead_admin": "Als Administrator kannst Du die fehlenden Kaggle-Einstellungen hier einschalten.",
  "chibi_sensei.enable.title": "Kaggle-Einstellungen ändern",
  "chibi_sensei.enable.intro": "AuraGo ändert diese Kaggle-Einstellungen für die ganze Installation, nicht nur für Chibi Sensei. Du kannst jede davon in den Kaggle-Einstellungen wieder ausschalten.",
  "chibi_sensei.enable.required": "Fürs Training nötig",
  "chibi_sensei.enable.optional": "Optional",
  "chibi_sensei.enable.confirm": "Diese Einstellungen ändern",
  "chibi_sensei.enable.cancel": "Abbrechen",
  "chibi_sensei.enable.applied": "Geänderte Kaggle-Einstellungen: {count}",
  "chibi_sensei.enable.none_selected": "Wähle mindestens eine Einstellung.",
  "chibi_sensei.enable.ask_title": "Frag eine Administratorin oder einen Administrator",
  "chibi_sensei.enable.ask_text": "Nur Administratoren können Kaggle-Einstellungen ändern. Bitte um Folgendes:",
  "chibi_sensei.consent.title": "Lizenz des Basismodells",
  "chibi_sensei.consent.text": "{model} wird unter der {license} veröffentlicht. Wenn Du es trainierst und nutzt, akzeptierst Du ihre Bedingungen.",
  "chibi_sensei.consent.link_label": "Lizenztext",
  "chibi_sensei.consent.accept_label": "Ich habe die {license} gelesen und akzeptiere sie.",
  "chibi_sensei.consent.accept": "Lizenz akzeptieren",
  "chibi_sensei.consent.accepted": "Du hast die {license} akzeptiert.",
  "chibi_sensei.consent.saved": "Lizenz akzeptiert.",
  "chibi_sensei.link.address": "Linkadresse",
  "chibi_sensei.link.copy": "Link kopieren",
  "chibi_sensei.link.copied": "Link kopiert.",
  "chibi_sensei.link.copy_failed": "Kopieren hat nicht geklappt. Der Link ist markiert; kopiere ihn von Hand.",
  "chibi_sensei.link.open": "In neuem Tab öffnen",
  "chibi_sensei.link.blocked": "Dieser Link lässt sich von hier aus nicht öffnen.",
  "chibi_sensei.shape.NvidiaTeslaT4": "NVIDIA-T4-Grafikkarte (16 GB)",
  "chibi_sensei.shape.NvidiaL4": "NVIDIA-L4-Grafikkarte (24 GB)",
  "chibi_sensei.overrides.method": "Methode",
  "chibi_sensei.overrides.method_auto": "Automatisch ({value})",
  "chibi_sensei.overrides.epochs": "Epochen (1–10)",
  "chibi_sensei.overrides.epochs_help": "Wie oft das Modell jedes Beispiel sieht.",
  "chibi_sensei.overrides.lr": "Lernrate (0,000001–0,001)",
  "chibi_sensei.overrides.lr_help": "Wie groß jeder Lernschritt ist. Zu hoch macht das Training instabil.",
  "chibi_sensei.overrides.seq": "Maximale Länge (Tokens)",
  "chibi_sensei.overrides.seq_help": "Längere Beispiele werden bei dieser Länge abgeschnitten.",
  "chibi_sensei.overrides.lora_r": "LoRA-Rang (1–256)",
  "chibi_sensei.overrides.lora_r_help": "LoRA trainiert einen kleinen Zusatz statt des ganzen Modells; der Rang ist die Größe dieses Zusatzes.",
  "chibi_sensei.overrides.lora_alpha": "LoRA-Alpha (1–512)",
  "chibi_sensei.overrides.lora_alpha_help": "Wie stark der Zusatz wirkt. Meist das Doppelte des Rangs.",
  "chibi_sensei.overrides.auto": "Automatisch: {value}",
  "chibi_sensei.overrides.auto_option": "Automatisch",
  "chibi_sensei.overrides.save": "Werte speichern",
  "chibi_sensei.overrides.reset": "Zurück auf automatisch",
  "chibi_sensei.overrides.saved": "Trainingswerte gespeichert.",
  "chibi_sensei.overrides.fix_fields": "Prüfe die markierten Felder.",
  "chibi_sensei.overrides.bad_method": "Dieses Basismodell unterstützt diese Methode nicht.",
  "chibi_sensei.overrides.bad_epochs": "Gib eine ganze Zahl von 1 bis 10 ein.",
  "chibi_sensei.overrides.bad_lr": "Gib eine Zahl von 0,000001 bis 0,001 ein.",
  "chibi_sensei.overrides.bad_seq": "Wähle eine der angebotenen Längen.",
  "chibi_sensei.overrides.bad_lora_r": "Gib eine ganze Zahl von 1 bis 256 ein.",
  "chibi_sensei.overrides.bad_lora_alpha": "Gib eine ganze Zahl von 1 bis 512 ein.",
  "chibi_sensei.confirm.title": "Training auf Kaggle starten?",
  "chibi_sensei.confirm.data_title": "Was diesen Computer verlässt",
  "chibi_sensei.confirm.data_rows": "{train} Trainings-, {val} Validierungs- und {eval} Auswertungsbeispiele ({size})",
  "chibi_sensei.confirm.data_private": "Sie landen in einem privaten Datensatz auf Deinem Kaggle-Konto. Privat heißt: Nur Du siehst ihn; Kaggle veröffentlicht ihn nicht.",
  "chibi_sensei.confirm.dataset": "Kaggle-Datensatz: {name}",
  "chibi_sensei.confirm.notebook": "Kaggle-Notebook: {name}",
  "chibi_sensei.confirm.notebook_help": "Ein privates Notebook auf Deinem Konto führt das Training aus.",
  "chibi_sensei.confirm.license": "Lizenz des Basismodells: {name}",
  "chibi_sensei.confirm.time_title": "Zeit und GPU-Quota",
  "chibi_sensei.confirm.estimate": "Etwa {minutes} Min. GPU-Zeit (Schätzung).",
  "chibi_sensei.confirm.session": "Sitzungslimit: {minutes} Min. Das ist die längste Zeit, die Kaggle dem Lauf die GPU überlässt; spätestens dann stoppt Kaggle den Lauf.",
  "chibi_sensei.confirm.quota": "GPU-Zeit übrig in dieser Woche: {hours} Std. Kaggle gibt jedem Konto ein kostenloses wöchentliches GPU-Kontingent (die Quota); dieser Lauf verbraucht einen Teil davon.",
  "chibi_sensei.confirm.quota_unknown": "Kaggle hat nicht gesagt, wie viel GPU-Zeit in dieser Woche übrig ist.",
  "chibi_sensei.confirm.cancel_title": "Wenn Du den Lauf stoppst",
  "chibi_sensei.confirm.cancel_text": "AuraGo hört sofort auf zu warten. Die GPU-Zeit endet, wenn Du das Notebook auf Kaggle stoppst, spätestens aber beim Zeitlimit des Laufs ({minutes} Min.).",
  "chibi_sensei.confirm.after_title": "Nach dem Lauf",
  "chibi_sensei.confirm.after_delete": "AuraGo löscht den Datensatz und das Notebook auf Kaggle. Das fertige Modell bleibt hier.",
  "chibi_sensei.confirm.after_keep_policy": "Löschen auf Kaggle ist in den Kaggle-Einstellungen nicht erlaubt, deshalb bleiben Datensatz und Notebook auf Deinem Kaggle-Konto. Der Lauf listet sie auf, damit Du sie selbst löschen kannst.",
  "chibi_sensei.confirm.after_keep_setting": "Datensatz und Notebook bleiben auf Deinem Kaggle-Konto (Einstellung von Chibi Sensei). Der Lauf listet sie auf.",
  "chibi_sensei.confirm.valid": "Diese Bestätigung gilt {minutes} Min.",
  "chibi_sensei.confirm.expired": "Die Bestätigung war abgelaufen oder der Plan hat sich geändert, deshalb hat Sensei eine neue vorbereitet. Prüfe die Angaben und starte erneut.",
  "chibi_sensei.confirm.start": "Training starten",
  "chibi_sensei.confirm.cancel": "Abbrechen",
  "chibi_sensei.confirm.started": "Training gestartet.",

  "_F3_new": "",
  "chibi_sensei.run_state.queued": "Wartet auf den Start",
  "chibi_sensei.run_state.packaging": "Deine Beispiele werden gepackt",
  "chibi_sensei.run_state.uploading": "Deine Beispiele werden zu Kaggle hochgeladen",
  "chibi_sensei.run_state.creating_dataset": "Der private Datensatz wird auf Kaggle angelegt",
  "chibi_sensei.run_state.waiting_dataset": "Kaggle bereitet den Datensatz vor",
  "chibi_sensei.run_state.starting": "Das Notebook startet auf Kaggle",
  "chibi_sensei.run_state.running": "Training auf Kaggle",
  "chibi_sensei.run_state.downloading": "Die Ergebnisse werden heruntergeladen",
  "chibi_sensei.run_state.verifying": "Das heruntergeladene Modell wird geprüft",
  "chibi_sensei.run_state.cleaning": "Auf Kaggle wird aufgeräumt",
  "chibi_sensei.run_state.succeeded": "Fertig",
  "chibi_sensei.run_state.failed": "Fehlgeschlagen",
  "chibi_sensei.run_state.cancelled": "Gestoppt",
  "chibi_sensei.phase.prepare": "Vorbereiten",
  "chibi_sensei.phase.upload": "Hochladen",
  "chibi_sensei.phase.start": "Auf Kaggle starten",
  "chibi_sensei.phase.train": "Trainieren",
  "chibi_sensei.phase.download": "Ergebnisse holen",
  "chibi_sensei.phase.cleanup": "Aufräumen",
  "chibi_sensei.phase_state.done": "erledigt",
  "chibi_sensei.phase_state.active": "läuft",
  "chibi_sensei.phase_state.failed": "fehlgeschlagen",
  "chibi_sensei.phase_state.stopped": "gestoppt",
  "chibi_sensei.phase_state.todo": "wartet",
  "chibi_sensei.phase_state.skipped": "übersprungen",
  "chibi_sensei.stage.boot": "Skript starten",
  "chibi_sensei.stage.env_check": "GPU und Internet prüfen",
  "chibi_sensei.stage.install": "Werkzeuge installieren",
  "chibi_sensei.stage.data": "Deine Beispiele lesen",
  "chibi_sensei.stage.load_base": "Basismodell herunterladen",
  "chibi_sensei.stage.base_eval": "Basismodell testen",
  "chibi_sensei.stage.train": "Aus Deinen Beispielen lernen",
  "chibi_sensei.stage.tuned_eval": "Trainiertes Modell testen",
  "chibi_sensei.stage.save": "Modell speichern",
  "chibi_sensei.stage.gguf": "In GGUF umwandeln",
  "chibi_sensei.stage.manifest": "Dateien auflisten",
  "chibi_sensei.stage.unknown": "Anderer Schritt",
  "chibi_sensei.run.title": "Aktueller Lauf",
  "chibi_sensei.run.title_past": "Lauf vom {date}",
  "chibi_sensei.run.timeline": "Fortschritt des Laufs",
  "chibi_sensei.run.stages": "Schritte im Notebook",
  "chibi_sensei.run.history_title": "Läufe dieses Projekts",
  "chibi_sensei.run.history_empty": "Noch keine Läufe. Starte oben einen.",
  "chibi_sensei.run.item_label": "Lauf vom {date}, {preset}, {state}",
  "chibi_sensei.run.status_kaggle": "Kaggle meldet: {status}",
  "chibi_sensei.run.elapsed": "Läuft seit {time}",
  "chibi_sensei.run.estimate": "Schätzung etwa {minutes} Min.",
  "chibi_sensei.run.longer": "Das dauert länger als geschätzt. Kaggle stoppt den Lauf spätestens beim Sitzungslimit ({minutes} Min.).",
  "chibi_sensei.run.progress": "Schritt {step} von {max}, Epoche {epoch}",
  "chibi_sensei.run.eta": "noch etwa {time}",
  "chibi_sensei.run.live_waiting": "Warte auf die ersten Zahlen von Kaggle …",
  "chibi_sensei.run.live_off": "Live-Zahlen gibt es für diesen Lauf nicht; das Diagramm erscheint, wenn der Lauf endet.",
  "chibi_sensei.run.cancel": "Lauf stoppen",
  "chibi_sensei.run.cancel_title": "Diesen Lauf stoppen?",
  "chibi_sensei.run.cancel_text": "AuraGo hört sofort auf zu warten. Die GPU-Zeit endet, wenn Du das Notebook auf Kaggle stoppst, spätestens aber beim Zeitlimit des Laufs ({minutes} Min.).",
  "chibi_sensei.run.cancel_requested": "Stopp angefordert. AuraGo wartet nicht mehr auf diesen Lauf. Die GPU-Zeit endet, wenn das Notebook auf Kaggle stoppt, spätestens nach {minutes} Min.",
  "chibi_sensei.run.stop_title": "Auf Kaggle stoppen",
  "chibi_sensei.run.stop_text": "Kaggle hat die Stopp-Anfrage von AuraGo nicht angenommen. Öffne das Notebook auf Kaggle und stoppe die Sitzung dort. AuraGo beobachtet weiter, bis sie endet.",
  "chibi_sensei.run.stopped_local": "Gestoppt, bevor auf Kaggle etwas lief. Hochgeladene Dateien werden aufgeräumt.",
  "chibi_sensei.run.result_title": "Ergebnis",
  "chibi_sensei.run.result_steps": "{steps} Trainingsschritte in {epochs} Epochen",
  "chibi_sensei.run.train_loss": "Letzter Trainingsverlust: {value}",
  "chibi_sensei.run.eval_loss": "Auswertungsverlust: {base} vorher, {tuned} nach dem Training",
  "chibi_sensei.run.loss_help": "Der Verlust misst, wie weit die Antworten des Modells von Deinen erwarteten Antworten entfernt sind. Kleiner ist besser.",
  "chibi_sensei.run.exact": "Exakte Antworten: {base} vorher, {tuned} nachher ({n} Beispiele)",
  "chibi_sensei.run.labels": "Richtige Labels: {base} vorher, {tuned} nachher ({n} Beispiele)",
  "chibi_sensei.run.perplexity": "Perplexität: {base} vorher, {tuned} nachher (kleiner ist besser)",
  "chibi_sensei.run.small_sample": "Nur {n} Auswertungsbeispiele: Nimm diese Zahlen als groben Hinweis.",
  "chibi_sensei.run.gpu_used": "Genutzte Grafikkarte: {name}",
  "chibi_sensei.run.failed_title": "Was schiefging",
  "chibi_sensei.run.fix_title": "Was Du versuchen kannst",
  "chibi_sensei.run.cancelled_text": "Dieser Lauf wurde gestoppt.",
  "chibi_sensei.run.warning_title": "Hinweis",
  "chibi_sensei.run.files_title": "Dateien",
  "chibi_sensei.run.files_empty": "Dieser Lauf hat keine Dateien.",
  "chibi_sensei.run.download": "Herunterladen",
  "chibi_sensei.run.download_label": "{name} herunterladen",
  "chibi_sensei.run.leftovers_title": "Auf Kaggle geblieben",
  "chibi_sensei.run.leftovers_text": "Das ist auf Deinem Kaggle-Konto geblieben. Lösche es auf Kaggle, wenn Du es nicht mehr brauchst.",
  "chibi_sensei.run.delete": "Lauf löschen",
  "chibi_sensei.run.delete_title": "Diesen Lauf löschen?",
  "chibi_sensei.run.delete_text": "Das entfernt den Lauf und seine heruntergeladenen Dateien von diesem Computer. Auf Kaggle ändert sich nichts.",
  "chibi_sensei.run.deleted": "Lauf gelöscht.",
  "chibi_sensei.run.log_title": "Kaggle-Protokoll (letzte 64 KB)",
  "chibi_sensei.run.log_empty": "Kaggle hat für diesen Lauf kein Protokoll geschickt.",
  "chibi_sensei.run.events_title": "Was passiert ist",
  "chibi_sensei.run.events_empty": "Noch nichts.",
  "chibi_sensei.run.jobs_title": "Kaggle-Details",
  "chibi_sensei.run.notebook_title": "Notebook auf Kaggle",
  "chibi_sensei.run.job_line": "Schritt {step}, Auftrag {id}",
  "chibi_sensei.event.state": "Jetzt: {state}",
  "chibi_sensei.event.stage_start": "Gestartet: {stage}",
  "chibi_sensei.event.stage_done": "Erledigt: {stage}",
  "chibi_sensei.event.stage_failed": "Fehlgeschlagen: {stage}",
  "chibi_sensei.event.warn": "Hinweis: {text}",
  "chibi_sensei.event.error": "Fehler: {text}",
  "chibi_sensei.event.kaggle": "Kaggle-Auftrag für Schritt {step}",
  "chibi_sensei.fail.env_no_gpu": "Kaggle hat dem Notebook keine GPU gegeben.",
  "chibi_sensei.fail.env_no_gpu_fix": "Prüfe, ob Dein Kaggle-Konto per Telefon bestätigt ist und noch GPU-Quota hat, und starte dann erneut.",
  "chibi_sensei.fail.env_unsupported_gpu": "Kaggle hat eine Grafikkarte zugeteilt, die dieses Modell nicht nutzen kann.",
  "chibi_sensei.fail.env_unsupported_gpu_fix": "Wähle in den Chibi-Sensei-Einstellungen einen anderen GPU-Typ, zum Beispiel die T4, und starte erneut.",
  "chibi_sensei.fail.env_no_internet": "Das Notebook hatte keinen Internetzugang.",
  "chibi_sensei.fail.env_no_internet_fix": "Kaggle erlaubt Internet nur für per Telefon bestätigte Konten. Bestätige Deine Telefonnummer auf Kaggle und starte erneut.",
  "chibi_sensei.fail.install_failed": "Die Installation der Trainingswerkzeuge ist fehlgeschlagen.",
  "chibi_sensei.fail.install_failed_fix": "Das ist oft ein kurzes Problem beim Paketserver. Starte später erneut.",
  "chibi_sensei.fail.data_mismatch": "Die hochgeladenen Beispiele passten nicht zu dem, was AuraGo geschickt hat.",
  "chibi_sensei.fail.data_mismatch_fix": "Starte erneut; AuraGo lädt eine frische Kopie hoch.",
  "chibi_sensei.fail.base_download_failed": "Das Herunterladen des Basismodells ist fehlgeschlagen.",
  "chibi_sensei.fail.base_download_failed_fix": "Hugging Face ist vielleicht ausgelastet. Starte später erneut.",
  "chibi_sensei.fail.oom": "Der Grafikkarte ist der Speicher ausgegangen.",
  "chibi_sensei.fail.oom_fix": "Wähle die Voreinstellung Schnell oder unter Erweitert eine kürzere maximale Länge.",
  "chibi_sensei.fail.train_diverged": "Das Training wurde instabil: Der Verlust war keine Zahl mehr.",
  "chibi_sensei.fail.train_diverged_fix": "Senke unter Erweitert die Lernrate und starte erneut.",
  "chibi_sensei.fail.merge_failed": "Das Speichern des trainierten Modells ist fehlgeschlagen.",
  "chibi_sensei.fail.merge_failed_fix": "Starte erneut. Schlägt es wieder fehl, wähle ein anderes Basismodell.",
  "chibi_sensei.fail.gguf_failed": "Die Umwandlung des Modells in GGUF ist fehlgeschlagen.",
  "chibi_sensei.fail.gguf_failed_fix": "Starte erneut. Das Kaggle-Protokoll unten zeigt Einzelheiten.",
  "chibi_sensei.fail.gguf_arch_mismatch": "Das umgewandelte Modell hat eine unerwartete Architektur.",
  "chibi_sensei.fail.gguf_arch_mismatch_fix": "Der Konverter unterstützt dieses Basismodell noch nicht. Wähle ein anderes Basismodell.",
  "chibi_sensei.fail.kaggle_submit_unknown": "Es ist unklar, ob Kaggle das Notebook gestartet hat.",
  "chibi_sensei.fail.kaggle_submit_unknown_fix": "Sieh Dir Deine Notebooks auf Kaggle an. Starte erneut, wenn dort nichts läuft.",
  "chibi_sensei.fail.remote_timeout": "Der Lauf wurde nicht innerhalb seines Zeitlimits fertig.",
  "chibi_sensei.fail.remote_timeout_fix": "Wähle die Voreinstellung Schnell oder erhöhe das Kaggle-Zeitlimit in den Kaggle-Einstellungen.",
  "chibi_sensei.fail.status_missing": "Der Lauf endete ohne Ergebnisbericht.",
  "chibi_sensei.fail.status_missing_fix": "Das Kaggle-Protokoll unten zeigt die letzten Zeilen. Starte erneut.",
  "chibi_sensei.fail.download_failed": "Das Herunterladen der Ergebnisse ist fehlgeschlagen.",
  "chibi_sensei.fail.download_failed_fix": "Starte erneut. Fehlt Platz, erhöhe die Speicherlimits.",
  "chibi_sensei.fail.checksum_mismatch": "Eine heruntergeladene Datei war beschädigt.",
  "chibi_sensei.fail.checksum_mismatch_fix": "Starte erneut; AuraGo behält keine beschädigten Dateien.",
  "chibi_sensei.fail.lora_targets_missing": "Das Training hat in diesem Basismodell keine anpassbaren Schichten gefunden.",
  "chibi_sensei.fail.lora_targets_missing_fix": "Wähle ein anderes Basismodell oder unter Erweitert das volle Fine-Tuning, wenn das Modell es anbietet.",
  "chibi_sensei.fail.cancelled_by_user": "Du hast diesen Lauf gestoppt.",
  "chibi_sensei.fail.cancelled_by_user_fix": "Starte einen neuen Lauf, wann immer Du willst.",
  "chibi_sensei.fail.desktop_readonly": "Der Desktop ist in den Nur-Lesen-Modus gewechselt, deshalb wurde der Lauf gestoppt.",
  "chibi_sensei.fail.desktop_readonly_fix": "Starte erneut, sobald der Desktop Änderungen erlaubt.",
  "chibi_sensei.fail.unknown": "Der Lauf ist fehlgeschlagen.",
  "chibi_sensei.fail.unknown_fix": "Die Ereignisse und das Kaggle-Protokoll unten zeigen, was passiert ist.",
  "chibi_sensei.warning.time_budget": "Das Training hat beim Zeitbudget aufgehört. Das Modell nutzt, was es bis dahin gelernt hat.",
  "chibi_sensei.warning.base_gguf_skipped": "Für die Umwandlung des Basismodells zum Vergleichen blieb keine Zeit. Dein trainiertes Modell ist vollständig.",
  "chibi_sensei.warning.unknown": "Der Lauf endete mit einem Hinweis.",
  "chibi_sensei.artifact.gguf_tuned": "Dein Modell (GGUF)",
  "chibi_sensei.artifact.gguf_base": "Basismodell zum Vergleichen (GGUF)",
  "chibi_sensei.artifact.adapter": "LoRA-Adapter",
  "chibi_sensei.artifact.adapter_config": "Adapter-Einstellungen",
  "chibi_sensei.artifact.metrics": "Trainingszahlen",
  "chibi_sensei.artifact.eval_samples": "Antworten der Auswertung",
  "chibi_sensei.artifact.status": "Laufbericht",
  "chibi_sensei.artifact.log": "Kaggle-Protokoll",
  "chibi_sensei.artifact.other": "Andere Datei",
  "chibi_sensei.leftover.dataset": "Kaggle-Datensatz",
  "chibi_sensei.leftover.notebook": "Kaggle-Notebook",
  "chibi_sensei.leftover.other": "Anderes Kaggle-Element",
  "chibi_sensei.chart.title": "Verlust während des Trainings",
  "chibi_sensei.chart.train": "Trainingsverlust",
  "chibi_sensei.chart.eval": "Validierungsverlust",
  "chibi_sensei.chart.axis_step": "Schritt",
  "chibi_sensei.chart.empty": "Noch keine Zahlen.",
  "chibi_sensei.chart.summary": "Der Trainingsverlust ging in {steps} Schritten von {first} auf {last}; kleinster Wert {min}.",
  "chibi_sensei.chart.summary_eval": "Letzter Validierungsverlust: {value}."
}
```
  German and English are identical for `chibi_sensei.enable.optional` ("Optional"). Add `"chibi_sensei.enable.optional": true` to `sameIn["de"]` in `ui/desktop_chibi_sensei_i18n_test.go:59-65`.
- [ ] 2. **Instruction for the 14 other locales** (`cs, da, el, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh`), one implementer per language or group:
  - Add every key of both fragments except the `_F…` markers.
  - Replace the values of the 8 `_F2_changed` keys: the old translations are now wrong.
  - Use the same keys and exactly the same `{placeholder}` names as English, real UTF-8, and no English filler.
  - Translate the meaning, not word by word:
    - "session limit" = the longest time Kaggle lets one run use the GPU;
    - "quota" = the free weekly GPU allowance;
    - "private dataset" = only the owner can see it.
  - Keep product names: Kaggle, AuraGo, Sensei, GGUF, LoRA, NVIDIA, T4, L4, Hugging Face, Vault.
  - Use the informal address where the language has one (the German "Du" style).
  - Every value must differ from English except where the word really is the same in that language. Each such key goes into `sameIn[<lang>]` in `ui/desktop_chibi_sensei_i18n_test.go`; likely candidates are `enable.optional`, `artifact.adapter` and `run.download`. Never add new keys to the global `same` list; no new key is language-neutral.
  - Validate each file with the JSON parse command below.
- [ ] 3. **Test updates:**
  - `ui/config_chibi_sensei_test.go`: add to the module markers (lines 24-31) `"'chibi_sensei.training.machine_shape'", "'chibi_sensei.training.delete_remote_after_run'", "CHIBI_SENSEI_MACHINE_SHAPES"`. Add to the en key list (line 76) `"config.chibi_sensei.training_title", "config.chibi_sensei.training_delete_remote_help"`.
  - `ui/kaggle_locales_test.go:35`: add `"config.kaggle.allowed_machine_shapes_help"` to the untranslated-help list, and append:
```go
// The Kaggle form offers the two known machine shapes as a checkbox list saved through one hidden array field.
func TestKaggleConfigMachineShapeChoices(t *testing.T) {
	module := normalizeAssetText(mustReadUIFile(t, "cfg/kaggle.js"))
	for _, marker := range []string{
		"const KAGGLE_MACHINE_SHAPES = ['NvidiaTeslaT4', 'NvidiaL4'];",
		`data-path="kaggle.allowed_machine_shapes" data-type="array"`,
		"data-cfg-choice data-kaggle-shape=",
		"shapeField.dispatchEvent(new Event('input', { bubbles: true }));",
	} {
		if !strings.Contains(module, marker) {
			t.Fatalf("cfg/kaggle.js misses %q", marker)
		}
	}
}
```
  - Already done in F3: the registration module list, the guards count, the loader entry, `desktopLocalI18nHelpers` and the model script. F4 re-runs them.
- [ ] 4. **Browser smoke (B5 adds it to `internal/server/chibi_sensei_app_browser_test.go`).** After the existing data-step assertions in `TestChibiSenseiAppRealServerBrowser`:
  1. Click `[data-csn="step"][data-step="training"]` and wait for `[data-csn="preflight"][aria-busy="false"]`.
  2. Assert `[data-csn="preflight-summary"][data-state="blocked"]`.
  3. Assert `[data-csn="preflight-item"][data-id="kaggle_enabled"][data-ok="0"][data-level="block"]` exists, and so does `[data-csn="preflight-item"][data-id="dataset_ready"][data-ok="0"]` (the smoke dataset has one row, so `count_min` blocks).
  4. Assert `[data-csn="primary"]` is disabled and `[data-csn="primary-hint"]` is not empty.
  5. Assert one of `[data-csn="preflight-enable"]` (admin session) or `[data-csn="preflight-ask"]` is present. Do not click it.
  6. Assert `[data-csn="preset-card"]` count is 3.

  No request to `/api/desktop/chibi-sensei/` may fail (the preflight answers 200 with Kaggle disabled), and the test never calls `runs/prepare`.
- [ ] 5. **Guides:**
  - `ui/AGENTS.md:130`: replace "P1 ships projects, the Goal step and the dataset studio." with "P1 ships projects, the Goal step and the dataset studio; P2 adds Kaggle training (preflight with admin enabling, license acceptance, confirmation with plan token, run monitor with loss chart, downloads)." Extend the verify list with the browser smoke command.
  - `ui/js/desktop/apps/AGENTS.md:111`: "ten classic scripts" → "twelve classic scripts". Replace the `-training.js` item with: "`-training.js` (preflight grouped by area with block and note levels, the admin dialog that enables Kaggle settings by flag name, preset cards, advanced overrides mirroring `ValidateOverrides`, license acceptance, the confirmation built from `runs/prepare` with the plan token and a re-prepare on `plan_expired`; later states for Test and Use), `-chart.js` (pure SVG loss chart; Node-tested `layout`, buckets above 600 points), `-run.js` (run monitor: timeline over run states and trainer stages, live or post-run chart, log highlights, cancel and Stop-on-Kaggle, result, files, leftovers, history)".
  - Line 112: "2.5 s active-job poll" → "2.5 s poll of active jobs and of the active run".
  - Line 113: replace "training, test and use come from the server and are `todo` in P1" with "training, test and use come from the server; the training primary action starts a run only when the preflight is ready and shows the active run while one exists; test and use stay `todo`".
  - Insert after line 114: "- Training: Kaggle settings change only through `POST /preflight/enable` with flag names after a dialog that lists every flag; others see what to ask for. A start needs a `prepare` answer and its plan token; the confirmation names dataset, notebook, rows, bytes, license, estimate, session limit, quota, the cancel promise and the cleanup. Cancel stays available on a read-only desktop. Outside links (license, Kaggle) appear as selectable text with copy and open buttons limited to https or same-origin paths (`P.safeLink`), never as embedded resources; the license section's class is `csn-license` (cookie-banner filters hide `consent`)."
  - Line 115: keep the `desktopLocalI18nHelpers` sentence (verified at `ui/i18n_lint_test.go:1113`). "Config page: `cfg/chibi_sensei.js` (P1 fields only)" → "Config page: `cfg/chibi_sensei.js` (P1 fields and the training group); the machine-shape allow-list is in `cfg/kaggle.js`."
  - `scripts/AGENTS.md`, Chibi line: "…readiness, primary-action, table-reducer, mood, preflight grouping, overrides validation, run timeline and chart layout logic of the desktop app in a vm."
  - DOX: `ui/cfg` and `ui/lang` have no own guides; the root guide is unaffected.
- [ ] 6. **Verify:**
```bash
node -e "for (const f of process.argv.slice(1)) JSON.parse(require('fs').readFileSync(f,'utf8'))" ui/lang/chibi_sensei/*.json ui/lang/kaggle/*.json ui/lang/config/chibi_sensei/*.json
node scripts/test-chibi-sensei-model.mjs
npm run check:ui
gofmt -l ui
git diff --check
go build ./...
go test ./ui -run '(?i)(ChibiSensei|Kaggle|ConfigChibiSensei|Translations_|I18n|Frontend_)' -count=1
```
  Expected:
  - `chibi-sensei model: ok (N checks)`;
  - `check:ui` exits 0;
  - no `gofmt` output;
  - `ok aurago/ui`, with all locale tests green now, including `TestChibiSenseiLocalesCoverAllSixteenLanguages`, `TestChibiSenseiTranslationsAreTranslated`, `TestChibiSenseiGermanUsesDu` and `TestTranslations_AllKeysPresentInAllLanguages`.
  - After B5 lands the smoke: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run TestChibiSenseiAppRealServerBrowser -count=1 -timeout 15m` passes.
