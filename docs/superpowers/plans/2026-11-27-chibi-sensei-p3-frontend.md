# Chibi Sensei P3 (test and compare): frontend plan (F1 to F3)

Index with conventions, the coordinator contract and its amendments: `2026-11-27-chibi-sensei-p3.md`. Where this file and the amendments differ, the amendments win.

Frontend half of the Chibi Sensei P3 plan (step 4 "Test"): batches F1, F2 and F3. It follows the conventions of the P2 plan index (`docs/superpowers/plans/2026-11-23-chibi-sensei-p2.md`, section "Conventions for every batch"). Worktree: `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\xenodochial-blackwell-7e567d`. All line numbers were checked against the files on 2026-11-27.

## Contract changes

1. **The loader entry and the module-list tests move from F3 to F2.** These are `ui/js/desktop/core/module-loader.js:43`, `ui/desktop_chibi_sensei_guards_test.go:49-50` (12 → 14), `ui/desktop_chibi_sensei_registration_test.go:13` and `ui/i18n_lint_test.go:1113-1124` (two new rows). Reason: once `chibi-sensei-test.js` and `chibi-sensei-chat.js` exist, `TestChibiSenseiSourcesAreSafeAndSmall` stops at its `t.Fatalf` on the script count. That fatal stop happens before the innerHTML, template-string, network and line-budget checks, so none of those checks would run on the new modules in F2. `TestChibiSenseiLoaderListsEveryScript` fails as well. P2 made the same move (P2 frontend, contract change 4).
2. **F1 also edits `ui/config_chibi_sensei_test.go`** (new markers for the runtime group). The table gives `ui/*_test.go` to F3, but the F1 brief names this file.
3. **F1 writes 9 keys into all 16 `ui/lang/chibi_sensei/*.json` files.** These are the six `chibi_sensei.error.*` texts of the new `ERROR_CODES`, plus `primary.no_succeeded_run`, `tip.test_ready` and `tip.test_done`. Reason: `TestChibiSenseiAppUsesOnlyKeysItHas` checks the `ERROR_CODES` family and every literal key, and the locale test checks all 16 languages. Without the keys F1 cannot finish green. The F1 implementer translates the 14 other languages for these 9 keys.
4. **F1 edits two functions of the composition root**, `recompute` (`chibi-sensei.js:372`) and `runPrimary` (`:385`). The root lies inside F2's glob, but the F1 brief puts the Test primary action in F1. F2 edits the root later, so the edits are sequential.
5. **`scripts/test-chibi-sensei-model.mjs` is split across batches by content.**
   - F1 replaces the `test`/`use` loop (lines 63-66), adds the primary and tip checks, and adds the stream-parser and `chatStream` checks. It loads `chibi-sensei-api.js`.
   - F2 loads `chibi-sensei-chat.js` and `chibi-sensei-test.js` and adds the reducer and helper checks.
   - F3 changes nothing in the script; it updates the Chibi line of `scripts/AGENTS.md`.
   - Reason: each batch has to verify its own pure logic. F1's change to `primaryFor('test')` breaks lines 63-66 the moment it lands.
6. **The selector `chat-answer` gains the `data-state` value `stopped`.** It also gets `data-phase="waiting|loading|generating"` while `data-state="streaming"`. Both are additive; the smoke still waits for `done`. New selectors the smoke may use (additive): `test-run`, `test-panel`, `test-empty`, `samples-filter`, `samples-pager`, `sample-try`, `runtime-stop`, `runtime-card`, `runtime-progress`, `chat-system`, `chat-new`, `chat-error`, `chat-live`, `chat-readonly`.
7. **The run module's report code is lifted, not copied.** `fillSuccess` and `confusionBox` (`chibi-sensei-run.js:549-592`) live inside the `createArea` closure today and are not exported. D16 asks for reuse, so F2 moves both to module scope as `fillReport(t, box, run, report)` and `confusionBox(t, acc)` and exports `fillReport`. Behaviour stays the same.
8. **Loader order:** `… run, chat, test, root`. The test module uses `P.chat` at create time only, so the order is for reading, not for correctness.
9. **`report/seen` is never sent from a read-only desktop.** It is a POST, and D10 does not list it as a stop operation. A `desktop_readonly` answer to it would run `inst.handleError` and force the whole window read-only. So on a read-only desktop the Test step stays `ready` until a writable visit.

## Conventions for the three batches

- These apply as in P2:
  - implementers do not commit, stage, stash or switch branches;
  - `git diff --check` runs before the tests;
  - `node --check` runs on every changed JS file;
  - text is real UTF-8 (no `\u` escapes).
- UI rules:
  - `textContent` only; no `innerHTML`, no backticks (also not in comments), no browser dialogs, no storage;
  - every class starts with `csn-`; there are no external URLs;
  - `fetch` and `ReadableStream` code lives only in `chibi-sensei-api.js`;
  - each module stays under 1100 lines.
- **Locale keys are always written as full single-quoted literals** (`'chibi_sensei.x.y'`), never built by concatenation. Families go through literal maps, because the guard test skips keys that end in `.` or `_`.
- Report format: files created, files modified, the exact commands with their PASS/FAIL lines, deviations with the reason, open items.
- Scratch files go only to `C:\disposable\chibi-sensei\p3\`.
- No batch creates an `AGENTS.md` file or a file under `docs/`.

## File map

| File | Batch | Change |
|---|---|---|
| `ui/cfg/chibi_sensei.js` | F1 | runtime defaults, backend list, "Local testing" group, header, later note |
| `ui/js/config/catalog.js:150` | F1 | two number rules |
| `ui/lang/config/chibi_sensei/*.json` (16) | F1 | 13 new keys, 2 changed |
| `ui/config_chibi_sensei_test.go` | F1 | runtime markers |
| `ui/lang/meta.json` | — | no change: the backend select builds its options inline, like the machine-shape select; `meta.json` only carries provider-ref metadata |
| `chibi-sensei-core.js` | F1 | 6 error codes, constants |
| `chibi-sensei-api.js` | F1 | `createSseParser`, 7 methods |
| `chibi-sensei-steps.js` | F1 | Test primary, two tips |
| `chibi-sensei.js` | F1 (`recompute`, `runPrimary`), F2 (`enterStep`) | |
| `ui/lang/chibi_sensei/*.json` (16) | F1 | 9 keys |
| `scripts/test-chibi-sensei-model.mjs` | F1, F2 | checks |
| `chibi-sensei-chat.js` (new, ≈900 lines) | F2 | runtime strip, install card, chat, comparison; pure reducer |
| `chibi-sensei-test.js` (new, ≈600 lines) | F2 | step view, run picker, tabs, report card, examples |
| `chibi-sensei-run.js`, `chibi-sensei-training.js` | F2 | `fillReport` lift; `LATER_KEYS` |
| `ui/css/desktop-app-chibi-sensei.css` (406 → ≈475 lines) | F2 | Test step styles |
| `ui/lang/chibi_sensei/en.json`, `de.json` | F2 | ≈118 new keys, 1 removed |
| `module-loader.js:43` and three `ui/*_test.go` lists | F2 | see contract change 1 |
| `ui/lang/chibi_sensei/{cs,da,el,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | F3 | translations |
| `ui/desktop_chibi_sensei_i18n_test.go` | F3 | exception lists |
| `ui/AGENTS.md:140`, `ui/js/desktop/apps/AGENTS.md:169-177`, `scripts/AGENTS.md` (Chibi line) | F3 | DOX |

`APP` below means `ui/js/desktop/apps/chibi-sensei/`.

---

## F1: config group, API methods with the stream reader, constants, Test primary action

**Goal:** the runtime settings are editable in the Config UI. The API module can read, install and stop the runtime, read the test view and the examples, mark a report as seen and stream a chat. The Test step has its primary action.

**Files**
- Modify:
  - `ui/cfg/chibi_sensei.js`
  - `ui/js/config/catalog.js`
  - `ui/lang/config/chibi_sensei/*.json` (16)
  - `ui/config_chibi_sensei_test.go`
  - `APP/chibi-sensei-core.js`, `APP/chibi-sensei-api.js`, `APP/chibi-sensei-steps.js`, `APP/chibi-sensei.js`
  - `ui/lang/chibi_sensei/*.json` (16; 9 keys)
  - `scripts/test-chibi-sensei-model.mjs`

### Steps

- [ ] **1. `ui/cfg/chibi_sensei.js`**
  - Header: "(phases P1 and P2)" becomes "(phases P1 to P3)". "Agent and runtime keys stay hidden until their phases ship." becomes "Agent keys stay hidden until their phase ships."
  - Add to `CHIBI_SENSEI_DEFAULTS` (line 9) after `training`:
    ```js
    runtime: Object.freeze({ enabled: true, backend: 'auto', idle_minutes: 10, context_size: 4096 })
    ```
  - Add after line 18:
    ```js
    const CHIBI_SENSEI_RUNTIME_BACKENDS = Object.freeze(['auto', 'cpu', 'vulkan', 'cuda']);
    ```
  - Push this group after the training group (after line 86):
    ```js
    groups.push({ title: t('config.chibi_sensei.runtime_title'), fields: [
        chibiCfgToggleRow('config.chibi_sensei.runtime_enabled_label', 'config.chibi_sensei.runtime_enabled_help',
            data.runtime.enabled === true, 'chibi_sensei.runtime.enabled', 'toggleBool(this)'),
        form.select({ label: t('config.chibi_sensei.runtime_backend_label'), help: tOr('config.chibi_sensei.runtime_backend_help', ''),
            path: 'chibi_sensei.runtime.backend', value: data.runtime.backend,
            options: CHIBI_SENSEI_RUNTIME_BACKENDS.map(value => ({ value, label: t('config.chibi_sensei.runtime_backend_' + value) })) }),
        chibiCfgNumber('config.chibi_sensei.runtime_idle_minutes_label', 'config.chibi_sensei.runtime_idle_minutes_help', 'chibi_sensei.runtime.idle_minutes', data.runtime.idle_minutes, 1, 120),
        chibiCfgNumber('config.chibi_sensei.runtime_context_size_label', 'config.chibi_sensei.runtime_context_size_help', 'chibi_sensei.runtime.context_size', data.runtime.context_size, 1024, 32768)
    ] });
    ```
  - No `step` on the context field: a hand-written 5000 must not make the browser refuse the form; the server checks the range.
  - `chibiCfgFill` fills the missing boolean `enabled` with `true`.
- [ ] **2. `ui/js/config/catalog.js:150`.** Append after the `max_daily_rows` rule, with a comma on the line before:
  ```js
  'chibi_sensei.runtime.idle_minutes': { type: 'number', min: 1, max: 120, integer: true, changedOnly: true },
  'chibi_sensei.runtime.context_size': { type: 'number', min: 1024, max: 32768, integer: true, changedOnly: true }
  ```
- [ ] **3. Config locales.** Insert the new keys before `config.chibi_sensei.later_note`, in the same order in all 16 files. The implementer translates the 14 other languages: real translations; "CUDA", "Vulkan", "NVIDIA" and "llama.cpp" stay as they are.

| Key | English | German |
|---|---|---|
| `config.chibi_sensei.runtime_title` | Local testing | Lokales Testen |
| `config.chibi_sensei.runtime_enabled_label` | Allow local testing | Lokales Testen erlauben |
| `config.chibi_sensei.runtime_enabled_help` | Lets the Test step chat with trained models on this server. The runtime (llama.cpp) is downloaded only after an administrator confirms it in the app. | Erlaubt dem Schritt Testen, auf diesem Server mit trainierten Modellen zu chatten. Die Laufzeit (llama.cpp) wird erst heruntergeladen, wenn jemand mit Administratorrechten das in der App bestätigt. |
| `config.chibi_sensei.runtime_backend_label` | Where the models run | Wo die Modelle laufen |
| `config.chibi_sensei.runtime_backend_help` | Automatic tries an NVIDIA graphics card, then another graphics card through Vulkan, then the processor. If a graphics card fails, AuraGo falls back to the processor. | Automatisch versucht zuerst eine NVIDIA-Grafikkarte, dann eine andere Grafikkarte über Vulkan, dann den Prozessor. Klappt es mit der Grafikkarte nicht, weicht AuraGo auf den Prozessor aus. |
| `config.chibi_sensei.runtime_backend_auto` | Automatic | Automatisch |
| `config.chibi_sensei.runtime_backend_cpu` | Processor (CPU) | Prozessor (CPU) |
| `config.chibi_sensei.runtime_backend_vulkan` | Graphics card through Vulkan | Grafikkarte über Vulkan |
| `config.chibi_sensei.runtime_backend_cuda` | NVIDIA graphics card (CUDA) | NVIDIA-Grafikkarte (CUDA) |
| `config.chibi_sensei.runtime_idle_minutes_label` | Stop idle models after (minutes) | Unbenutzte Modelle stoppen nach (Minuten) |
| `config.chibi_sensei.runtime_idle_minutes_help` | A model that has not answered for this long leaves memory. The next message loads it again. | Ein Modell, das so lange nicht geantwortet hat, verlässt den Speicher. Die nächste Nachricht lädt es wieder. |
| `config.chibi_sensei.runtime_context_size_label` | Chat memory (tokens) | Chat-Gedächtnis (Tokens) |
| `config.chibi_sensei.runtime_context_size_help` | How much of a chat a model sees at once. More needs more working memory; 4096 fits most tests. | Wie viel von einem Chat ein Modell auf einmal sieht. Mehr braucht mehr Arbeitsspeicher; 4096 reicht für die meisten Tests. |
| `help.chibi_sensei.enabled` (changed) | Shows the Chibi Sensei app on the desktop. The dataset studio and testing work locally; training runs on Kaggle and needs the Kaggle permissions. | Zeigt die Chibi-Sensei-App auf dem Desktop. Datensatz-Studio und Testen arbeiten lokal; das Training läuft auf Kaggle und braucht die Kaggle-Berechtigungen. |
| `config.chibi_sensei.later_note` (changed) | The Sensei chat gets its settings in a later version. | Der Sensei-Chat bekommt seine Einstellungen in einer späteren Version. |

- [ ] **4. `ui/config_chibi_sensei_test.go`**
  - Add to the module markers (line 33): `"'chibi_sensei.runtime.enabled'"`, `"'chibi_sensei.runtime.backend'"`, `"CHIBI_SENSEI_RUNTIME_BACKENDS"`.
  - Add to the ranges (line 46): `"'chibi_sensei.runtime.idle_minutes', data.runtime.idle_minutes, 1, 120"`, `"'chibi_sensei.runtime.context_size', data.runtime.context_size, 1024, 32768"`.
  - Add both catalog rule strings of step 2 to the catalog markers.
  - Add `"config.chibi_sensei.runtime_title"` and `"config.chibi_sensei.runtime_backend_help"` to the English key list (line 77).
- [ ] **5. `chibi-sensei-core.js`**
  - Append to `ERROR_CODES` (line 277): `'runtime_unavailable', 'runtime_not_installed', 'runtime_busy', 'model_missing', 'run_not_succeeded', 'samples_missing'`.
  - Add before `P.el = el;` (line 397) and export each under the same name on `P`:
    ```js
    // ---- test and chat (P3) ------------------------------------------------------------------------------------------------------
    const CHAT_TARGETS = ['tuned', 'base'];
    const RUNTIME_STATES = ['disabled', 'unsupported', 'not_installed', 'installing', 'ready', 'failed'];
    const SAMPLE_FILTERS = ['all', 'different', 'wrong'];
    // Limits of POST /runtime/chat; GET /meta "chat" overrides them when present.
    const CHAT_LIMITS = Object.freeze({ max_targets: 2, max_turns: 20, max_chars: 16000, max_tokens_cap: 1024 });
    function chatLimits(meta) {
        const m = meta && meta.chat && typeof meta.chat === 'object' ? meta.chat : {};
        const pick = (name) => (Number.isInteger(m[name]) && m[name] > 0 ? m[name] : CHAT_LIMITS[name]);
        return { max_targets: pick('max_targets'), max_turns: pick('max_turns'), max_chars: pick('max_chars'), max_tokens_cap: pick('max_tokens_cap') };
    }
    // Characters as the server counts them (Unicode code points, Go's utf8.RuneCountInString).
    function charCount(text) { return Array.from(String(text || '')).length; }
    ```
- [ ] **6. `chibi-sensei-api.js`: the stream parser.** Add it at module scope after `qs` (line 63) and publish it as `P.createSseParser = createSseParser;` next to line 235. It is pure: no DOM, no fetch, no globals except the decoder that is passed in.
  ```js
  const SSE_MAX_BUFFER = 1 << 20;
  // createSseParser(decoder) parses text/event-stream incrementally. push(chunk) takes a Uint8Array (decoded with
  // decoder.decode(chunk, {stream: true}), so a multi-byte character split across chunks survives) or a string, and returns the
  // events completed so far as [{event, data}]. end() flushes the decoder and returns what is left. Lines end with \n, \r\n or \r;
  // a \r at the end of a chunk waits for the next chunk. Comment lines (":") are skipped; id, retry and unknown fields are ignored;
  // several data lines join with \n; an event without data is not dispatched, and an event without its blank line at the end of
  // the stream is dropped (as in the HTML standard). A line longer than 1 MiB throws {code: 'bad_response'}.
  function createSseParser(decoder) {
      let buffer = '', name = '', data = [];
      const out = [];
      function line(text) {
          if (text === '') {
              if (data.length) out.push({ event: name || 'message', data: data.join('\n') });
              name = ''; data = [];
              return;
          }
          if (text.charAt(0) === ':') return;
          const colon = text.indexOf(':');
          const field = colon < 0 ? text : text.slice(0, colon);
          let value = colon < 0 ? '' : text.slice(colon + 1);
          if (value.charAt(0) === ' ') value = value.slice(1);
          if (field === 'event') name = value;
          else if (field === 'data') data.push(value);
      }
      function drain(final) {
          let start = 0;
          for (let i = 0; i < buffer.length; i++) {
              const ch = buffer.charAt(i);
              if (ch !== '\n' && ch !== '\r') continue;
              if (ch === '\r' && i === buffer.length - 1 && !final) break;
              line(buffer.slice(start, i));
              if (ch === '\r' && buffer.charAt(i + 1) === '\n') i++;
              start = i + 1;
          }
          buffer = buffer.slice(start);
          if (buffer.length > SSE_MAX_BUFFER) throw Object.assign(new Error('event stream line too long'), { code: 'bad_response' });
      }
      return {
          push(chunk) {
              buffer += typeof chunk === 'string' ? chunk : (decoder ? decoder.decode(chunk, { stream: true }) : '');
              drain(false);
              return out.splice(0);
          },
          end() {
              if (decoder) buffer += decoder.decode();
              drain(true);
              buffer = ''; name = ''; data = [];
              return out.splice(0);
          }
      };
  }
  ```
- [ ] **7. `chibi-sensei-api.js`: methods.** Add them after `deleteRun` (line 190):
  ```js
  // Runtime = {state, admin, build, backend:{requested,selected,fallback}, download:{bytes,done_bytes,host,error_code}, instances, limits, notice_code}
  async runtime() { return (await json('GET', '/runtime')).runtime || null; },
  async installRuntime() { return (await send('POST', '/runtime/install', {})).runtime || null; },
  async stopRuntime() { return (await send('POST', '/runtime/stop', {})).runtime || null; },
  // {run, report, report_seen, samples_total, models:{tuned,base}, chat:{system_role, sampling, max_tokens}}
  runTest(runId) { return json('GET', '/runs/' + enc(runId) + '/test'); },
  // {samples, total, offset, limit}; query {offset, limit, filter}
  runSamples(runId, query) { return json('GET', '/runs/' + enc(runId) + '/samples' + qs(query)); },
  reportSeen(runId) { return send('POST', '/runs/' + enc(runId) + '/report/seen', {}); },
  chatStream(body, handlers) { return chatStream(body, handlers); },
  ```
  Define `chatStream` inside `createApi`, before `const api`, so it can use `request`, `jsonHeaders` and `lifetime`:
  ```js
  const CHAT_EVENTS = ['status', 'delta', 'done', 'error'];
  // chatStream(body, {onEvent, onClose}) -> {abort()}. POST /runtime/chat. onEvent({type, target, ...data}) for each status,
  // delta, done or error event of a known target; onClose({ended, aborted, error}) exactly once: ended = the server sent "end";
  // aborted = abort() or the window closed; error = the normal error shape (a JSON error answer before the stream, a 200 that
  // is not an event stream: bad_response, a stream that broke off: network or connection_lost).
  function chatStream(body, handlers) {
      const h = handlers || {};
      const targets = P.CHAT_TARGETS || ['tuned', 'base'];
      const ctrl = new AbortController();
      const relay = () => ctrl.abort();
      if (lifetime.signal.aborted) ctrl.abort(); else lifetime.signal.addEventListener('abort', relay);
      let closed = false, ended = false;
      const close = result => {
          if (closed) return;
          closed = true;
          lifetime.signal.removeEventListener('abort', relay);
          if (h.onClose) h.onClose(result);
      };
      const take = events => events.forEach(ev => {
          let data = null;
          try { data = JSON.parse(ev.data); } catch (_) { data = null; }
          if (!data || typeof data !== 'object') return;
          if (ev.event === 'end') { ended = true; return; }
          if (CHAT_EVENTS.indexOf(ev.event) < 0 || targets.indexOf(data.target) < 0) return;
          if (h.onEvent) h.onEvent(Object.assign({}, data, { type: ev.event }));
      });
      (async () => {
          try {
              const response = await request('POST', '/runtime/chat', { headers: Object.assign(jsonHeaders(), { Accept: 'text/event-stream' }), body: JSON.stringify(body || {}), signal: ctrl.signal });
              const type = String(response.headers.get('content-type') || '');
              if (type.indexOf('text/event-stream') !== 0 || !response.body || typeof response.body.getReader !== 'function') {
                  throw P.errors.normalize({ code: 'bad_response', status: response.status });
              }
              const reader = response.body.getReader();
              const parser = createSseParser(new TextDecoder('utf-8'));
              while (!ended) {
                  const step = await reader.read();
                  if (step.done) { take(parser.end()); break; }
                  take(parser.push(step.value));
              }
              if (ended) reader.cancel().catch(() => {});
              close({ ended, aborted: false, error: ended ? null : P.errors.normalize({ code: 'connection_lost', status: 200 }) });
          } catch (err) {
              const aborted = ctrl.signal.aborted || !!(err && (err.aborted || err.name === 'AbortError'));
              close({ ended, aborted, error: aborted ? null : (err && err.csn ? err : P.errors.normalize({ code: err && err.code, message: String(err && err.message || err) })) });
          }
      })();
      return { abort() { ctrl.abort(); } };
  }
  ```
  `request` already turns a non-ok answer into `P.errors.fromResponse(status, body)`, which gives the normal error shape (contract: 422, 404, 409, 503). Update the header comment: "…the project event stream is an EventSource…; the chat answer is a streamed POST read with a ReadableStream reader."
- [ ] **8. `chibi-sensei-steps.js`**
  - Replace line 84 with:
    ```js
    if (step === 'test') {
        // A succeeded run makes the chat possible; the server's step state and the run list both count (the list holds 20 runs).
        const has = s.test === 'ready' || s.test === 'done' || !!o.hasSucceeded;
        let reason = '';
        if (!has) reason = 'chibi_sensei.primary.no_succeeded_run';
        else if (o.readonly) reason = 'chibi_sensei.readonly_banner';
        return { labelKey: 'chibi_sensei.primary.chat_model', action: 'open_chat', disabled: !!reason, reasonKey: reason };
    }
    ```
    The `later` helper stays for `use`.
  - `TIPS` (line 93): add `test_ready: 'chibi_sensei.tip.test_ready', test_done: 'chibi_sensei.tip.test_done'`.
  - Update the comment at line 61: opts gain `hasSucceeded`.
- [ ] **9. `chibi-sensei.js`**
  - `recompute` (line 375-378): add `hasSucceeded: inst.runs.some(r => r && r.state === 'succeeded')` to the opts.
  - `runPrimary` (line 385): add before the closing brace of the if-chain:
    ```js
    } else if (id === 'open_chat') {
        if (inst.step !== 'test') { const pending = showStep(inst, 'test'); if (pending) await pending; }
        const view = inst.parts.view;
        if (!inst.disposed && inst.step === 'test' && view && typeof view.showView === 'function') view.showView('chat', { focus: true });
    ```
    Update the comment at line 384. Until F2 lands, `showView` does not exist and the branch only switches the step.
- [ ] **10. `ui/lang/chibi_sensei/*.json` (all 16).** Put the error keys after `chibi_sensei.error.license_consent_required`, `primary.no_succeeded_run` after `primary.preflight_blocked`, and the tips after `tip.training_blocked`. Use the same position in every file.

| Key | English | German |
|---|---|---|
| `chibi_sensei.error.runtime_unavailable` | Local testing is not available: it is switched off, or this host has no runtime for it. | Lokales Testen ist nicht verfügbar: Es ist ausgeschaltet, oder für diesen Rechner gibt es keine Laufzeit dafür. |
| `chibi_sensei.error.runtime_not_installed` | The local test runtime is not installed yet. | Die lokale Test-Laufzeit ist noch nicht installiert. |
| `chibi_sensei.error.runtime_busy` | Both test slots are in use, or the runtime is being installed. Try again in a moment, or stop the models. | Beide Testplätze sind belegt, oder die Laufzeit wird gerade installiert. Versuch es gleich noch einmal oder stoppe die Modelle. |
| `chibi_sensei.error.model_missing` | A model file of this run is missing, so it cannot answer. | Eine Modelldatei dieses Laufs fehlt, deshalb kann es nicht antworten. |
| `chibi_sensei.error.run_not_succeeded` | Only a successfully finished run can be tested. | Testen kannst Du nur einen erfolgreich beendeten Lauf. |
| `chibi_sensei.error.samples_missing` | This run has no saved evaluation examples. | Dieser Lauf hat keine gespeicherten Auswertungsbeispiele. |
| `chibi_sensei.primary.no_succeeded_run` | Train a model successfully first. | Trainiere zuerst erfolgreich ein Modell. |
| `chibi_sensei.tip.test_ready` | Start with the report card, then look at the examples and chat with your model. | Fang mit dem Zeugnis an, schau Dir dann die Beispiele an und chatte mit Deinem Modell. |
| `chibi_sensei.tip.test_done` | Ask your model what your users would ask. Compare it with the base model to see what training changed. | Frag Dein Modell, was Deine Nutzer fragen würden. Vergleich es mit dem Basismodell, um zu sehen, was das Training verändert hat. |

- [ ] **11. `scripts/test-chibi-sensei-model.mjs`**
  - `files` (line 9): insert `'chibi-sensei-api.js'` after core.
  - Context (line 18): add `TextDecoder, TextEncoder, Uint8Array, AbortController, Promise`.
  - Replace lines 63-66 with these checks:
    - `primary('test', {test:'todo'}, {})`: disabled, action `open_chat`, reason `primary.no_succeeded_run`.
    - `{test:'ready'}`: enabled.
    - `{test:'todo'}` with `{hasSucceeded:true}`: enabled.
    - `{test:'done'}` with `{readonly:true}`: disabled with `readonly_banner`.
    - `primary('use')`: still `primary.later`.
  - Tips: `tipFor('test','ready')` is `tip.test_ready`, `('test','done')` is `tip.test_done`, `('test','weird')` is `tip.test_todo`.
  - Parser checks (`const S = () => P.createSseParser(new TextDecoder())`):
    - a delta split inside its JSON over two pushes gives one event with the whole data;
    - for every split point of the UTF-8 bytes of `'event: delta\ndata: {"text":"ä€😀"}\n\n'`, two pushes give `data` byte-identical to the original;
    - `': ping\n\n'` gives no event; `': ping\nevent: end\ndata: {}\n\n'` gives one `end`;
    - `'event: done\r\ndata: {}\r\n\r\n'` gives one event;
    - a `\r` at the end of one push and `\n` at the start of the next give one line, with no extra dispatch;
    - two `data:` lines join with `\n`;
    - `'data:x\n\n'` gives `message`/`x`;
    - `event: x` without data gives nothing;
    - an unterminated event is dropped by `end()`.
  - `chatStream` checks (top-level `await`; `P.createApi({ fetch })` with a fake):
    - (a) a 200 `text/event-stream` body whose reader yields three `Uint8Array` chunks (status, delta, done for `tuned`, then `end`): `onEvent` types `['status','delta','done']`, `onClose({ended:true, aborted:false, error:null})`;
    - (b) a 409 JSON `{"code":"runtime_busy"}`: `onClose` with `error.code === 'runtime_busy'`, no events;
    - (c) a 200 `application/json`: `error.code === 'bad_response'`;
    - (d) a stream that ends without `end`: `error.code === 'connection_lost'`;
    - (e) an event with target `provider` is not forwarded.
  - Wait for `onClose` through a promise. Keep the final console line.
- [ ] **12. Verify**
  ```bash
  git diff --check
  node --check ui/cfg/chibi_sensei.js && node --check ui/js/config/catalog.js
  for f in core api steps; do node --check ui/js/desktop/apps/chibi-sensei/chibi-sensei-$f.js; done; node --check ui/js/desktop/apps/chibi-sensei/chibi-sensei.js
  node -e "for (const f of process.argv.slice(1)) JSON.parse(require('fs').readFileSync(f,'utf8'))" ui/lang/config/chibi_sensei/*.json ui/lang/chibi_sensei/*.json
  node scripts/test-chibi-sensei-model.mjs
  go build ./...
  go test ./ui -run '(?i)(ChibiSensei|Translations_|Frontend_)' -count=1
  ```
  Expected: no `node` output except `chibi-sensei model: ok (N checks)`, and `ok aurago/ui`.

**Done when** every command above passes and the config page shows the "Local testing" group with four controls.

---

## F2: test module, chat module, routing, CSS, English and German

**Goal:**
- the Test step shows the run picker and the three views (report card, examples, chat);
- the chat streams one or two answers, with an install card, a runtime strip and advanced sampling;
- every user state has a text.

**Files**
- Create: `APP/chibi-sensei-chat.js`, `APP/chibi-sensei-test.js`.
- Modify:
  - `APP/chibi-sensei.js` (`enterStep`), `APP/chibi-sensei-run.js`, `APP/chibi-sensei-training.js`
  - `ui/css/desktop-app-chibi-sensei.css`
  - `ui/lang/chibi_sensei/en.json`, `de.json`
  - `ui/js/desktop/core/module-loader.js`, `ui/desktop_chibi_sensei_guards_test.go`, `ui/desktop_chibi_sensei_registration_test.go`, `ui/i18n_lint_test.go`
  - `scripts/test-chibi-sensei-model.mjs`
- Scratch: write `C:\disposable\chibi-sensei\p3\f3-en-changed.json` (`{}`, since F2 changes no English value) for F3.

### Steps

- [ ] **1. `chibi-sensei-run.js`**
  - Move `fillSuccess` (lines 549-576) and `confusionBox` (579-592) out of `createArea` to module scope, after `orMissing` (line 209), as `fillReport(t, box, run, report)` and `confusionBox(t, acc)`.
  - `fillReport` makes its own `line(text, name)` helper (same body as line 536). It reads `run.result`, `run.warning_code` and `report`, exactly like today.
  - In `paintResult` (line 537) call `fillReport(t, box, detail.run, detail.report)`.
  - Export `fillReport` in `P.run` (line 706).
  - The DOM and the `data-csn` names stay byte-identical (the P2 browser smoke reads `run-result` and `run-warning`).
- [ ] **2. `chibi-sensei-training.js`**
  - Line 17 becomes `const LATER_KEYS = { use: 'chibi_sensei.later.use_text' };`.
  - Line 896 falls back to `LATER_KEYS.use`.
  - Update the comment of `createLater`: Use only.
- [ ] **3. `chibi-sensei.js` `enterStep`.** Line 349 becomes:
  ```js
  else if (id === 'test') { if (P.need(inst, 'test')) view = P.test.create(inst); }
  else if (P.need(inst, 'training')) view = P.training.createLater(inst, id);
  ```
  Update the header comment ("…routing of the five steps, Test through P.test…").
- [ ] **4. `chibi-sensei-chat.js`: the pure part.** It touches no DOM at load. Publish:
  ```js
  P.chat = { createArea, initialState, reduce, conversation, canSend, requestBody, validateSampling, runtimeView, answerView, usable, STREAM_ERROR_KEYS }
  ```
  - State: `{ turns: [], draft: '', system: '', compare: false, compareFrom: -1, streaming: false, seq: 0, notice: null }`.
  - Turn: `{ id, user, started: false, answers: { tuned: Answer, base: Answer|null } }`.
  - Answer: `{ state: 'waiting'|'loading'|'streaming'|'done'|'error'|'stopped', text: '', stats: null, code: '' }`.
  - `done`, `error` and `stopped` are final states.
  - `reduce(state, action)` returns a new object, or the same object when nothing changes:

| Action | Rule |
|---|---|
| `{type:'draft', text}` / `{type:'system', text}` | set the field |
| `{type:'compare', on}` | `compare = !!on`; when it turns on with turns present, `compareFrom = turns.length` |
| `{type:'send', text}` | ignored while `streaming` or when `text.trim()` is empty. Pushes `{id: seq + 1, user: text.trim(), answers: {tuned: waiting, base: compare ? waiting : null}}`; `seq++`, `streaming = true`, `draft = ''`, `notice = null` |
| `{type:'event', ev}` | only while `streaming`, on the last turn, for a target whose answer is not null and not final; `started = true`. `status`: `loading` when `ev.state === 'loading'`, else `streaming`. `delta`: `text += ev.text` (strings only), state `streaming`. `done`: state `done`, `stats = {finish_reason, prompt_tokens, completion_tokens, ms, tok_s, backend}`. `error`: `ev.code === 'cancelled'` gives `stopped`, otherwise `error` with `code` = the code when it is a key of `STREAM_ERROR_KEYS`, else `unknown` |
| `{type:'close', ended, aborted, error}` | `streaming = false`. If the last turn was not `started` and `error` is set (an error before the stream): the turn is removed, `draft` becomes its `user` text when `draft` is empty, and `notice = error`. Otherwise every non-final answer of the last turn becomes `stopped` when `aborted`, else `error` with code `connection_lost` |
| `{type:'new_chat'}` | ignored while `streaming`; `turns = []`, `compareFrom = -1`, `notice = null` (system, compare and draft stay) |
| `{type:'notice', value}` | set `notice` |

  Exact helpers:
  ```js
  function usable(a) { return !!a && (a.state === 'done' || a.state === 'stopped') && a.text.trim() !== ''; }
  // The target's own conversation of finished turns: a turn the target has no usable answer for is left out completely, so roles
  // keep alternating and every request ends with the new user message.
  function conversation(state, target) {
      const out = [];
      const system = String(state.system || '').trim();
      if (system) out.push({ role: 'system', content: system });
      state.turns.forEach(turn => {
          const a = turn.answers[target];
          if (usable(a)) out.push({ role: 'user', content: turn.user }, { role: 'assistant', content: a.text });
      });
      return out;
  }
  function targetsOf(state) { return state.compare ? ['tuned', 'base'] : ['tuned']; }
  // canSend -> {ok, reasonKey, vars}
  function canSend(state, limits, text) {
      if (state.streaming) return { ok: false, reasonKey: 'chibi_sensei.chat.busy' };
      const msg = String(text || '').trim();
      if (!msg) return { ok: false, reasonKey: '' };
      for (const target of targetsOf(state)) {
          const msgs = conversation(state, target).concat([{ role: 'user', content: msg }]);
          if (msgs.filter(m => m.role === 'user').length > limits.max_turns) return { ok: false, reasonKey: 'chibi_sensei.chat.limit_turns', vars: { max: limits.max_turns } };
          if (msgs.reduce((n, m) => n + P.charCount(m.content), 0) > limits.max_chars) return { ok: false, reasonKey: 'chibi_sensei.chat.limit_chars', vars: { max: limits.max_chars } };
      }
      return { ok: true, reasonKey: '' };
  }
  function requestBody(state, runId, text, sampling) {
      const msg = String(text || '').trim();
      const body = { run_id: runId, targets: targetsOf(state).map(kind => ({ kind, messages: conversation(state, kind).concat([{ role: 'user', content: msg }]) })) };
      if (sampling && Object.keys(sampling).length) body.sampling = sampling;
      return body;
  }
  ```
  The view calls `canSend`, then `requestBody`, then dispatches `send`, then calls `api.chatStream`. 64 messages per conversation can never be reached: 1 + 2 × 20 = 41.
  - `validateSampling(draft, cap)` returns `{sampling, issues: [{field, min, max}]}`. It copies the pattern of `P.training.validateOverrides` (`chibi-sensei-training.js`):
    - empty fields are left out; a decimal comma is accepted;
    - `temperature` is a number from 0 to 2;
    - `top_p` is a number above 0 and at most 1 (the issue shows `min: 0.01`);
    - `top_k` is an integer from 0 to 200;
    - `max_tokens` is an integer from 1 to `cap`;
    - `seed` is never sent.
  - `runtimeView(rt, admin, readonly)` returns `{state, card, composer, canStop, installEnabled, installReasonKey}`:
    - `state`: `loading` while `rt === undefined`, `error` while `rt === null`, else `rt.state`;
    - `card`: `install` (admin, `not_installed`), `ask` (non-admin, `not_installed`), `installing`, `failed`, `disabled`, `unsupported`, or `null` when ready;
    - `composer = state === 'ready'`;
    - `canStop = rt && rt.instances.length > 0`;
    - `installEnabled = admin && !readonly`; the reason is `readonly_banner` when read-only.
  - `answerView(a)` returns `{state, phase}` for the DOM: waiting, loading and streaming give `state: 'streaming'` with `phase` `waiting`, `loading` or `generating`; `done` gives `done`; `stopped` gives `stopped`; `error` gives `error`.
  - Literal key maps (module constants):
    - `STREAM_ERROR_KEYS`: `start_failed`, `generation_failed`, `context_exceeded`, `runtime_busy`, `cancelled`, `connection_lost`, `unknown` → `chibi_sensei.chat.error_<code>`;
    - `RUNTIME_STATE_KEYS`: the six states plus `loading`, `error` → `chibi_sensei.runtime.state_*`;
    - `BACKEND_KEYS`: `cpu`, `vulkan`, `cuda`, `metal`, `auto` → `chibi_sensei.runtime.backend_*`;
    - `INSTALL_FAIL_KEYS`: `download_failed`, `checksum_mismatch`, `disk_full`, `unknown` → `chibi_sensei.runtime.failed_*`.
- [ ] **5. `chibi-sensei-chat.js`: `createArea(inst, ctx)`.**
  - Input: `ctx = { runId, test /* GET /runs/{id}/test body */ }`. Output: `{ el, setVisible(bool), focus(), setPrompt({system, user}), dispose() }`.
  - Lifecycle copied from `P.run.createArea` (`chibi-sensei-run.js:211`): all nodes are built up front, `offs` holds the bus unsubscribes, `disposed` guards async work, `dispose()` clears timers and cancels the animation frame.
  - DOM (exact `data-csn` names; the smoke reads the starred ones):
    ```
    div.csn-chat[data-csn=chat]
      div.csn-runtime[data-csn=runtime-strip*][data-state=<runtimeView.state>]
        span.csn-runtime-state[role=status]            runtime.state_* (+ " · " runtime.backend with BACKEND_KEYS[selected||requested])
        span.csn-muted                                 runtime.models_loaded {count,max} · runtime.idle_note {minutes}   (only when instances exist)
        button.csn-btn[data-csn=runtime-stop]          runtime.stop (hidden unless canStop; allowed read-only)
      p.csn-note[data-tone=warn][data-csn=runtime-notice]   runtime.gpu_fallback, when notice_code === 'gpu_fallback'
      div.csn-note[data-csn=runtime-card][data-card]   one of the cards below; hidden when card is null
        p (text)  progress.csn-progressbar[data-csn=runtime-progress][max][value] (installing)  p.csn-hint (progress text)
        button.csn-btn.csn-btn-primary[data-csn=runtime-install*]  runtime.install, or chibi_sensei.retry on the failed card
        button.csn-btn[data-csn=runtime-settings]      runtime.open_settings (disabled card, admin only; P.openLink('/config#chibi_sensei'))
      p.csn-note[data-csn=chat-readonly]               chat.readonly (read-only only; then chat-body is hidden)
      p.csn-note[data-tone=warn][data-csn=chat-tuned-missing]   test.tuned_missing or test.tuned_invalid when models.tuned is not available
      div.csn-chat-body[data-csn=chat-body]            shown only when composer && !readonly && models.tuned.available
        div.csn-chat-scroll[role=region][tabindex=0][aria-label=chat.history]
          p.csn-muted[data-csn=chat-empty]             chat.empty (no turns)
          div.csn-chat-columns[data-compare=0|1]
            section.csn-chat-column[data-csn=chat-column*][data-target=tuned][aria-labelledby]  h4 chat.column_tuned
              ol.csn-chat-turns > li.csn-chat-turn
                div.csn-chat-user  (span.csn-sr-only chat.you, then the text)
                div.csn-chat-answer[data-csn=chat-answer*][data-target][data-state][data-phase][aria-busy]
                  p.csn-chat-text    p.csn-chat-stats    p.csn-error (target error text)
            section … [data-target=base], h4 run.confusion_base; hidden while data-compare=0; a turn without a base answer shows p.csn-muted chat.not_compared
        div.csn-composer
          label.csn-field > span.csn-label chat.system_label + textarea.csn-textarea[rows=2][data-csn=chat-system] + span.csn-hint chat.system_help
          label.csn-field > span.csn-label chat.input_label + textarea.csn-textarea[rows=3][data-csn=chat-input*][placeholder=chat.input_placeholder]
          div.csn-composer-bar
            label.csn-check-option > input[type=checkbox][role=switch][data-csn=chat-compare*] + span chat.compare
            span.csn-hint[data-csn=chat-compare-reason]   test.base_* reason, or chat.compare_fresh after a mid-chat switch-on
            span.csn-spacer  span.csn-hint[data-csn=chat-chars] chat.chars {count,max}
            button.csn-btn[data-csn=chat-new] chat.new   button.csn-btn[data-csn=chat-stop*] chat.stop   button.csn-btn.csn-btn-primary[data-csn=chat-send*] chat.send
          p.csn-hint[data-csn=chat-reason]   canSend reason
          p.csn-error[role=alert][data-csn=chat-error]   inst.handleError(notice)
          details.csn-advanced[data-csn=chat-advanced] > summary chat.advanced, p.csn-hint chat.advanced_help,
            div.csn-grid2 (4 × label.csn-field: chat.temperature | chat.top_p | chat.top_k | chat.max_tokens{max}; input.csn-input[inputmode=decimal]
            [data-csn=chat-temperature|chat-top-p|chat-top-k|chat-max-tokens][placeholder=default]; span.csn-hint chat.default_value{value}),
            button.csn-btn[data-csn=chat-reset] chat.reset
          p.csn-muted chat.not_saved
      div.csn-sr-only[role=status][aria-live=polite][aria-atomic=true][data-csn=chat-live]
    ```
  - Runtime:
    - `GET /runtime` runs on the first `setVisible(true)`, after every `close` of a stream and after a stop.
    - While `state === 'installing'` a 1000 ms `setTimeout` loop calls `api.runtime()`.
    - While the area is visible and instances exist, a 30 s refresh shows the idle stop.
    - Every timer stops when the area is hidden or disposed.
    - When the state turns `installing` → `ready`: `inst.notify(t('chibi_sensei.runtime.installed'))`; if the focused element was removed (the Install button), focus `chat-input`.
  - Install:
    - `inst.askConfirm(t('…runtime.install_confirm_title'), t('…runtime.install_confirm_text', {build, backend, size: P.format.bytes(download.bytes), host}))`, then `api.installRuntime()`, then start the poll.
    - A failure shows `inst.handleError(err)` in `chat-error`. `runtime_unavailable` refreshes the status.
    - Cards with their texts:
      - `install`: `runtime.state_not_installed` + `runtime.install_text`;
      - `ask`: `runtime.state_not_installed` + `runtime.install_ask`;
      - `installing`: `runtime.state_installing` + `runtime.progress {done,total}`;
      - `failed`: `runtime.state_failed` + `INSTALL_FAIL_KEYS[error_code]`;
      - `disabled`: `runtime.state_disabled` + `runtime.disabled_admin` or `runtime.disabled_user`;
      - `unsupported`: `runtime.state_unsupported`.
  - Stop: `api.stopRuntime()` (after aborting a running stream) refreshes the strip and notifies `runtime.stopped`.
  - Send:
    - `validateSampling` first: an issue marks the field `aria-invalid`, opens the `details` and refuses.
    - Then `canSend`, then `requestBody(state, ctx.runId, draft, sampling)`, then dispatch `send`, then `stream = inst.api.chatStream(body, { onEvent: ev => dispatch({type:'event', ev}), onClose: r => dispatch(Object.assign({type:'close'}, r)) })`.
  - Compare switch:
    - disabled (`aria-disabled`, `disabled`) and forced off when `ctx.test.models.base.available` is false;
    - the reason comes from `BASE_REASON_KEYS` of the test module (`P.test.baseReasonKey(reason)`);
    - locked while streaming.
  - Painting:
    - `paintTurns()` rebuilds the turn lists only when the turn count or `compare` changes, keyed by turn id;
    - otherwise `paintAnswer(turn, target)` sets `textContent`, `data-state`, `data-phase`, `aria-busy`, the stats line (`chat.stats {tokens: format.number(completion_tokens), speed: format.decimal(tok_s, 1), backend}`) and `chat.cut_length` when `finish_reason === 'length'`;
    - the waiting and loading texts are `chat.waiting` and `chat.loading`;
    - deltas repaint at most once per `requestAnimationFrame`.
  - Keyboard and focus:
    - in `chat-input`, Enter without Shift and without `event.isComposing` sends;
    - Shift+Enter inserts a new line;
    - Escape while streaming stops;
    - after Send, focus stays in the input (now empty);
    - Stop aborts, then focuses `chat-input`;
    - when a stream ends while `chat-stop` has focus (it becomes disabled), focus moves to `chat-input`;
    - Send is disabled while streaming or `!canSend.ok`; Stop is disabled when not streaming.
  - Announcements (`chat-live`, never per token):
    - on the first event of a turn: `chat.announce_start` or `chat.announce_start_both`;
    - on close: `chat.announce_done` or `chat.announce_done_both`, `chat.announce_stopped` when aborted, `chat.announce_error` when any answer failed.
  - Auto-scroll:
    - `pinned = el.scrollHeight - el.scrollTop - el.clientHeight <= 24`, updated by a passive `scroll` listener;
    - Send sets `pinned = true`;
    - after each paint, `scrollTop = scrollHeight` when pinned (instant, no smooth scrolling, so reduced motion needs nothing more).
  - Read-only: the bus `readonly` event aborts a running stream, hides `chat-body` and shows `chat-readonly`; the strip and Stop stay.
  - Error answers from the chat:
    - `runtime_not_installed` refreshes the strip;
    - `desktop_readonly` goes through `inst.handleError`, which flips the window read-only.
- [ ] **6. `chibi-sensei-test.js`.** Publish:
  ```js
  P.test = { create, succeededRuns, smokeLine, modelLine, baseReasonKey, promptOf, VIEWS }
  ```
  `VIEWS = ['report','samples','chat']`.
  - Pure helpers:
    - `succeededRuns(runs)` returns the succeeded runs, newest first by `finished || created`.
    - `smokeLine(smoke)` returns `null` for null; `{key: 'chibi_sensei.test.smoke_ok', vars: {time}, tone: ''}` for ok; `test.smoke_failed` (tone warn) for failed; for skipped, `SMOKE_SKIP_KEYS[code] || 'chibi_sensei.test.smoke_skipped'`. `SMOKE_SKIP_KEYS`: `runtime_not_installed` → `smoke_skipped_installed`, `runtime_busy` → `smoke_skipped_busy`, `runtime_disabled` → `smoke_skipped_disabled`.
    - `modelLine(kind, model)` returns `{key, vars:{size}, tone}` from literal maps: tuned `test.tuned_ready` / `test.tuned_missing` / `test.tuned_invalid`; base `test.base_ready` / `test.base_missing` / `test.base_skipped` / `test.base_invalid`. An unknown reason counts as `missing`.
    - `baseReasonKey(reason)` is the base map without the ready entry.
    - `promptOf(sample)` returns `{system, user}`: `system` is the first message when its role is `system`; `user` is the last `user` message; for `prompt_text`, `user = prompt_text` and `system = ''`.
  - `create(inst)` returns `{ el, focus(), showView(name, {focus}), dispose() }`. DOM:
    ```
    section.csn-test[data-csn=test*]
      div.csn-test-head > h2 test.title, label.csn-field[data-csn=test-run-field] (hidden unless 2+ succeeded runs)
           > span.csn-label test.run_label + select.csn-select[data-csn=test-run] (option text test.run_option {date: format.dateTime(finished), preset})
      p.csn-muted test.intro
      div.csn-empty[data-csn=test-empty]  (no succeeded run: mascot 88 idle, h3 test.empty_title, p test.empty_text,
           button.csn-btn.csn-btn-primary[data-csn=test-goto-training] test.goto_training -> inst.showStep('training'))
      div.csn-tabs[role=tablist][aria-label=test.views_label]
        button.csn-tab[role=tab][id][aria-controls][aria-selected][tabindex=0|-1][data-csn=test-view*][data-view=report|samples|chat]
      div.csn-tabpanel[role=tabpanel][id][aria-labelledby][tabindex=0][data-csn=test-panel][data-view]  × 3 (kept mounted, hidden when inactive)
      p.csn-error[role=alert][data-csn=test-error]
    report panel:  section.csn-training-section[data-csn=report-card*] > h3 test.view_report,
                   div.csn-result[data-outcome=succeeded] (P.run.fillReport, or p test.report_none when report is null),
                   p.csn-note[data-csn=model-tuned][data-tone], p.csn-note[data-csn=model-base][data-tone], p.csn-note[data-csn=report-smoke][data-tone]
    samples panel: section.csn-training-section[data-csn=samples] > div.csn-section-head (h3 test.samples_title, label + select[data-csn=samples-filter]),
                   p.csn-muted test.samples_help, ol.csn-examples > li.csn-example[data-csn=sample*][data-index]
                     div.csn-example-head (h4 test.sample_title {n}, span.csn-match[data-match=1|0] × 0-2 with run.confusion_base/tuned + test.match_yes/no,
                          button.csn-btn[data-csn=sample-try][aria-label=test.try_chat_label {n}] test.try_chat)
                     h5 test.prompt + messages (p > strong editor.role_* + span text) or p.csn-diff-text prompt_text
                     h5 test.expected + p.csn-diff-text
                     div.csn-diff > 2 × div.csn-diff-col (h5 run.confusion_base|tuned, p.csn-diff-text or p.csn-muted test.no_answer, p.csn-muted test.answer_time {time})
                     p.csn-note test.truncated (when truncated)
                   p.csn-muted test.samples_empty | error.samples_missing;  div.csn-pager[data-csn=samples-pager] (table.prev, span table.page, table.next)
    chat panel:    P.chat.createArea(inst, {runId, test}).el, created when the chat view is first shown
    ```
  - Tabs: a real WAI-ARIA tab pattern with automatic activation.
    - ArrowLeft and ArrowRight move and activate (with wrap); Home and End jump.
    - Only the active tab has `tabindex=0`.
    - Tab and panel ids come from a module counter: `'csn-test-' + n + '-' + view`.
    - `showView(name, {focus})` activates the view. With `focus`, the chat view focuses `chat-input` when the composer is shown, else its panel.
  - Data:
    - On create and on bus `runs`/`run`: recompute the succeeded runs and keep the selection while it is still listed, else select the newest.
    - On a selection change: dispose the chat area (this aborts a running stream), clear the examples, then `api.runTest(id)` with a sequence guard.
    - A 409 `run_not_succeeded` calls `inst.reloadRuns()`. Other errors go to `test-error` through `inst.handleError`.
  - Report seen: when the report view is active, the data is loaded, `report_seen === false`, `!inst.readonly`, and the run is not yet in the module's `seen` set:
    - call `api.reportSeen(id)`;
    - on success: `inst.applyRun({ id, report_seen: true })`, then `if (inst.refresh) inst.refresh()`;
    - a failure is ignored silently (never `handleError`, see contract change 9).
  - Examples:
    - loaded when the samples view is first shown, then on a filter or page change;
    - `limit = 20`;
    - the filter `wrong` is offered only when the report has `exact_match` or `label_accuracy`;
    - Try in chat is disabled (with `title`) when read-only or the tuned model is unavailable;
    - Try in chat shows the chat view, calls `chatArea.setPrompt(promptOf(sample))`, notifies `test.try_chat_done` and focuses `chat-input`.
  - Bus: `readonly` repaints the try buttons (the chat area listens itself). `dispose()` disposes the chat area and unsubscribes.
- [ ] **7. CSS.** Append after line 406. Only tokens, every selector names a `.csn-` class, motion stays behind the gate. The new views reuse `csn-training-section`, `csn-result`, `csn-note`, `csn-diff*`, `csn-pager`, `csn-empty`, `csn-check-option`, `csn-grid2`, `csn-advanced`, `csn-progressbar`, `csn-textarea`, `csn-field`. They do not use the four classes without rules (`csn-state-inline`, `csn-job-result`, `csn-run-title`, `csn-run-history`), so no rule is added for those.
  ```css
  /* Test step (P3): head, tabs, examples, runtime strip, chat columns and composer */
  .csn-test { display: grid; gap: var(--ds-space-3); align-content: start; min-width: 0; }
  .csn-test > p { margin: 0; }
  .csn-test-head { display: flex; flex-wrap: wrap; align-items: flex-end; gap: var(--ds-space-2) var(--ds-space-3); }
  .csn-test-head h2 { margin: 0 auto 0 0; color: var(--vd-mat-heading-color, var(--vd-text)); font-family: var(--vd-mat-heading-font, inherit); font-size: 18px; font-weight: var(--vd-mat-heading-weight, 600); letter-spacing: var(--vd-mat-heading-tracking, normal); text-transform: var(--vd-mat-heading-transform, none); }
  .csn-test-head .csn-field { margin: 0; min-width: 220px; }
  .csn-tabs { display: inline-flex; flex-wrap: wrap; justify-self: start; gap: 2px; padding: 2px; border: 1px solid var(--csn-line); border-radius: var(--vd-mat-btn-radius, var(--ds-radius-sm)); background: var(--vd-mat-well, var(--vd-theme-control-bg)); }
  .csn-tab { min-height: 30px; padding: 0 14px; border: 1px solid transparent; border-radius: var(--vd-mat-btn-radius, var(--ds-radius-sm)); background: transparent; font-weight: 600; cursor: pointer; }
  .csn-tab:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
  .csn-tab[aria-selected="true"] { border-color: var(--vd-mat-selected-color, var(--vd-theme-accent-border)); background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); box-shadow: var(--vd-mat-selected-edge, none); color: var(--vd-mat-selected-text, inherit); }
  .csn-tabpanel { display: grid; gap: var(--ds-space-3); align-content: start; min-width: 0; }
  .csn-examples { display: grid; gap: var(--ds-space-2); margin: 0; padding: 0; list-style: none; }
  .csn-example { display: grid; gap: var(--ds-space-1); padding: var(--ds-space-2) var(--ds-space-3); border: 1px solid var(--csn-line); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-well, var(--vd-theme-control-bg)); }
  .csn-example h4, .csn-example h5, .csn-example p { margin: 0; }
  .csn-example h5 { color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12px; }
  .csn-example-head { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); }
  .csn-example-head h4 { margin-right: auto; font-size: 13px; }
  .csn-match { padding: 0 8px; border: 1px solid var(--csn-line); border-radius: var(--ds-radius-full); font-size: 12px; white-space: nowrap; }
  .csn-match[data-match="1"] { border-color: var(--csn-ok); color: var(--csn-ok-text); }
  .csn-match[data-match="0"] { border-color: var(--csn-bad); color: var(--csn-bad-text); }
  .csn-runtime { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2) var(--ds-space-3); padding: 6px var(--ds-space-3); border: 1px solid var(--csn-line); border-left: 3px solid var(--csn-line); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-well, var(--vd-theme-control-bg)); }
  .csn-runtime[data-state="ready"] { border-left-color: var(--csn-ok); }
  .csn-runtime[data-state="installing"], .csn-runtime[data-state="loading"] { border-left-color: var(--vd-accent); }
  .csn-runtime[data-state="failed"], .csn-runtime[data-state="error"] { border-left-color: var(--csn-bad); }
  .csn-runtime[data-state="disabled"], .csn-runtime[data-state="unsupported"], .csn-runtime[data-state="not_installed"] { border-left-color: var(--csn-warn); }
  .csn-runtime-state { margin-right: auto; font-weight: 600; }
  .csn-chat, .csn-chat-body, .csn-composer { display: grid; gap: var(--ds-space-2); min-width: 0; }
  .csn-chat-scroll { min-height: 160px; max-height: min(56vh, 640px); overflow: auto; overscroll-behavior: contain; padding: var(--ds-space-2); border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); }
  .csn-chat-columns { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--ds-space-3); }
  .csn-chat-columns[data-compare="1"] { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .csn-chat-column { display: grid; align-content: start; gap: var(--ds-space-2); min-width: 0; }
  .csn-chat-column h4 { margin: 0; color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12px; letter-spacing: var(--vd-mat-label-tracking, normal); text-transform: var(--vd-mat-label-transform, none); }
  .csn-chat-turns { display: grid; gap: var(--ds-space-2); margin: 0; padding: 0; list-style: none; }
  .csn-chat-turn { display: grid; gap: 4px; }
  .csn-chat-user, .csn-chat-answer { padding: 6px var(--ds-space-2); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); overflow-wrap: anywhere; white-space: pre-wrap; }
  .csn-chat-user { justify-self: end; max-width: 92%; background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); color: var(--vd-mat-selected-text, inherit); }
  .csn-chat-answer { border: 1px solid var(--csn-line); background: var(--vd-mat-well, var(--vd-theme-control-bg)); }
  .csn-chat-answer[data-state="error"] { border-color: var(--csn-bad); }
  .csn-chat-answer[data-state="stopped"] { border-color: var(--csn-warn); }
  .csn-chat-text, .csn-chat-stats { margin: 0; }
  .csn-chat-answer[data-state="streaming"] .csn-chat-text::after { content: "▍"; content: "▍" / ""; margin-left: 1px; color: var(--vd-accent); }
  .csn-chat-stats { color: var(--vd-theme-muted); font-size: 12px; font-variant-numeric: tabular-nums; white-space: normal; }
  .csn-composer .csn-textarea { min-height: 64px; }
  .csn-composer-bar { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); }
  .csn-sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip-path: inset(50%); border: 0; white-space: nowrap; }
  @container csn (max-width: 600px) {
      .csn-chat-columns[data-compare="1"] { grid-template-columns: minmax(0, 1fr); }
      .csn-tabs { display: flex; justify-self: stretch; }
      .csn-tab { flex: 1 1 0; }
  }
  @media (pointer: coarse) {
      .csn-app .csn-tab { min-height: 40px; }
  }
  @media (prefers-reduced-motion: no-preference) {
      body:not([data-animations="false"]) .csn-app .csn-tab { transition: background-color .15s ease, border-color .15s ease; }
      body:not([data-animations="false"]) .csn-app .csn-chat-answer[data-state="streaming"] .csn-chat-text::after { animation: csn-blink 1s steps(2, start) infinite; }
  }
  @keyframes csn-blink { to { visibility: hidden; } }
  ```
  - With reduced motion, or with animations switched off on the desktop, the streaming cursor stays visible and does not blink.
  - At narrow widths (window container at most 600 px) the two columns stack: the tuned conversation first, then the base conversation.
- [ ] **8. `en.json` and `de.json`.**
  - Remove `chibi_sensei.later.test_text`; it is unused after step 2. It must also leave the 14 other locales (F3).
  - No other existing key changes. `tip.test_todo`, `later.title`, `later.use_text`, `primary.later` and `panel.chat_later` stay: they still describe Use and the Sensei chat (P4).
  - Add the keys below after `chibi_sensei.tip.test_done`, in this order:

| Key (`chibi_sensei.` omitted) | English | German |
|---|---|---|
| `test.title` | Test your model | Teste Dein Modell |
| `test.intro` | See what training changed: the numbers, real examples and a chat. | Sieh, was das Training verändert hat: die Zahlen, echte Beispiele und ein Chat. |
| `test.empty_title` | No trained model yet | Noch kein trainiertes Modell |
| `test.empty_text` | As soon as a training run has finished successfully, you can test the model here. | Sobald ein Trainingslauf erfolgreich beendet ist, kannst Du das Modell hier testen. |
| `test.goto_training` | Go to training | Zum Training |
| `test.run_label` | Training run | Trainingslauf |
| `test.run_option` | Run from {date} ({preset}) | Lauf vom {date} ({preset}) |
| `test.views_label` | Test views | Testansichten |
| `test.view_report` | Report card | Zeugnis |
| `test.view_samples` | Examples | Beispiele |
| `test.view_chat` | Chat | Chatten |
| `test.loading` | Loading the test data… | Die Testdaten werden geladen… |
| `test.report_none` | This run has no report numbers. The examples and the chat still work. | Für diesen Lauf gibt es keine Zeugniszahlen. Beispiele und Chat funktionieren trotzdem. |
| `test.tuned_ready` | Your model: ready to chat ({size}). | Dein Modell: bereit zum Chatten ({size}). |
| `test.tuned_missing` | The file of your trained model is missing. Chatting is not possible for this run. | Die Datei Deines trainierten Modells fehlt. Chatten ist für diesen Lauf nicht möglich. |
| `test.tuned_invalid` | The file of your trained model is damaged. Chatting is not possible for this run. | Die Datei Deines trainierten Modells ist beschädigt. Chatten ist für diesen Lauf nicht möglich. |
| `test.base_ready` | Base model for comparing: ready ({size}). | Basismodell zum Vergleichen: bereit ({size}). |
| `test.base_missing` | The base model file is missing, so comparing is not possible. The next training run with this base model creates it again. | Die Datei des Basismodells fehlt, deshalb ist kein Vergleich möglich. Der nächste Trainingslauf mit diesem Basismodell legt sie neu an. |
| `test.base_skipped` | This run has no base model for comparing: there was no time left to convert it. | Dieser Lauf hat kein Basismodell zum Vergleichen: Für die Umwandlung blieb keine Zeit. |
| `test.base_invalid` | The base model file is damaged, so comparing is not possible. The next training run with this base model creates it again. | Die Datei des Basismodells ist beschädigt, deshalb ist kein Vergleich möglich. Der nächste Trainingslauf mit diesem Basismodell legt sie neu an. |
| `test.smoke_ok` | Quick check after training: your model answered in {time}. | Kurzer Check nach dem Training: Dein Modell hat in {time} geantwortet. |
| `test.smoke_failed` | Quick check after training: your model did not answer. Try the chat; if it fails there too, train again. | Kurzer Check nach dem Training: Dein Modell hat nicht geantwortet. Probier den Chat aus; klappt es dort auch nicht, trainiere neu. |
| `test.smoke_skipped_installed` | Quick check skipped: the local test runtime was not installed yet. | Kurzer Check übersprungen: Die lokale Test-Laufzeit war noch nicht installiert. |
| `test.smoke_skipped_busy` | Quick check skipped: the local test runtime was busy. | Kurzer Check übersprungen: Die lokale Test-Laufzeit war belegt. |
| `test.smoke_skipped_disabled` | Quick check skipped: local testing was switched off. | Kurzer Check übersprungen: Lokales Testen war ausgeschaltet. |
| `test.smoke_skipped` | Quick check skipped. | Kurzer Check übersprungen. |
| `test.samples_title` | Evaluation examples | Auswertungsbeispiele |
| `test.samples_help` | These prompts were held back from training. Both models answered them after the run. | Diese Eingaben waren nicht im Training. Beide Modelle haben sie nach dem Lauf beantwortet. |
| `test.filter_label` | Show | Anzeigen |
| `test.filter_all` | All examples | Alle Beispiele |
| `test.filter_different` | Where the answers differ | Wo sich die Antworten unterscheiden |
| `test.filter_wrong` | Where your model is wrong | Wo Dein Modell danebenliegt |
| `test.samples_empty` | No example matches this filter. | Kein Beispiel passt zu diesem Filter. |
| `test.sample_title` | Example {n} | Beispiel {n} |
| `test.prompt` | Prompt | Eingabe |
| `test.expected` | Expected answer | Erwartete Antwort |
| `test.no_answer` | No answer | Keine Antwort |
| `test.answer_time` | Answered in {time} | Antwort in {time} |
| `test.match_yes` | matches | stimmt |
| `test.match_no` | does not match | stimmt nicht |
| `test.truncated` | This example was shortened for the evaluation. | Dieses Beispiel wurde für die Auswertung gekürzt. |
| `test.try_chat` | Try in chat | Im Chat ausprobieren |
| `test.try_chat_label` | Try example {n} in the chat | Beispiel {n} im Chat ausprobieren |
| `test.try_chat_done` | The example is in the message box. Edit it or press Send. | Das Beispiel steht im Nachrichtenfeld. Ändere es oder drück auf Senden. |
| `runtime.state_loading` | Checking the local test runtime… | Die lokale Test-Laufzeit wird geprüft… |
| `runtime.state_error` | The state of the local test runtime could not be read. | Der Zustand der lokalen Test-Laufzeit ließ sich nicht lesen. |
| `runtime.state_disabled` | Local testing is switched off in the settings. | Lokales Testen ist in den Einstellungen ausgeschaltet. |
| `runtime.disabled_admin` | Switch it on under Settings, Chibi Sensei, Local testing. | Schalte es unter Einstellungen, Chibi Sensei, Lokales Testen ein. |
| `runtime.disabled_user` | Ask an administrator to switch on local testing. | Bitte jemanden mit Administratorrechten, lokales Testen einzuschalten. |
| `runtime.open_settings` | Open the settings | Einstellungen öffnen |
| `runtime.state_unsupported` | Local testing is not available on this host. The report card and the examples still work. | Lokales Testen ist auf diesem Rechner nicht verfügbar. Zeugnis und Beispiele funktionieren trotzdem. |
| `runtime.state_not_installed` | The local test runtime is not installed yet. | Die lokale Test-Laufzeit ist noch nicht installiert. |
| `runtime.install_text` | To chat with your model, AuraGo needs llama.cpp {build} for {backend}: a one-time download of {size} from {host}. | Zum Chatten braucht AuraGo llama.cpp {build} für {backend}: einmalig {size} von {host}. |
| `runtime.install` | Install | Installieren |
| `runtime.install_confirm_title` | Install the local test runtime? | Lokale Test-Laufzeit installieren? |
| `runtime.install_confirm_text` | AuraGo downloads llama.cpp {build} for {backend} ({size}) once from {host}, checks the files and keeps them in its data folder. Your models stay on this server. | AuraGo lädt llama.cpp {build} für {backend} ({size}) einmalig von {host} herunter, prüft die Dateien und legt sie in seinem Datenordner ab. Deine Modelle bleiben auf diesem Server. |
| `runtime.install_ask` | Ask an administrator to install the local test runtime in Chibi Sensei. | Bitte jemanden mit Administratorrechten, die lokale Test-Laufzeit in Chibi Sensei zu installieren. |
| `runtime.state_installing` | Installing the local test runtime… | Die lokale Test-Laufzeit wird installiert… |
| `runtime.progress` | {done} of {total} downloaded | {done} von {total} heruntergeladen |
| `runtime.installed` | The local test runtime is installed. | Die lokale Test-Laufzeit ist installiert. |
| `runtime.state_failed` | Installing the local test runtime failed. | Die Installation der lokalen Test-Laufzeit ist fehlgeschlagen. |
| `runtime.failed_download_failed` | The download broke off. Check the internet connection and try again. | Der Download ist abgebrochen. Prüf die Internetverbindung und versuch es noch einmal. |
| `runtime.failed_checksum_mismatch` | The downloaded file did not match its checksum and was discarded. Try again later. | Die heruntergeladene Datei passte nicht zu ihrer Prüfsumme und wurde verworfen. Versuch es später noch einmal. |
| `runtime.failed_disk_full` | There is not enough disk space for the runtime. | Für die Laufzeit ist nicht genug Speicherplatz frei. |
| `runtime.failed_unknown` | Something went wrong during the download. | Beim Download ist etwas schiefgegangen. |
| `runtime.state_ready` | Local test runtime ready | Lokale Test-Laufzeit bereit |
| `runtime.backend` | runs on {backend} | läuft auf {backend} |
| `runtime.backend_cpu` | the processor (CPU) | dem Prozessor (CPU) |
| `runtime.backend_vulkan` | the graphics card (Vulkan) | der Grafikkarte (Vulkan) |
| `runtime.backend_cuda` | the NVIDIA graphics card (CUDA) | der NVIDIA-Grafikkarte (CUDA) |
| `runtime.backend_metal` | the Apple graphics (Metal) | der Apple-Grafik (Metal) |
| `runtime.backend_auto` | the best available hardware | der besten verfügbaren Hardware |
| `runtime.gpu_fallback` | The graphics card could not be used, so the models run on the processor. Answers take longer. | Die Grafikkarte ließ sich nicht nutzen, deshalb laufen die Modelle auf dem Prozessor. Antworten dauern länger. |
| `runtime.models_loaded` | Models in memory: {count} of {max} | Modelle im Speicher: {count} von {max} |
| `runtime.idle_note` | Idle models stop by themselves after {minutes} minutes. | Unbenutzte Modelle stoppen nach {minutes} Minuten von selbst. |
| `runtime.stop` | Stop models | Modelle stoppen |
| `runtime.stopped` | The models were stopped. | Die Modelle wurden gestoppt. |
| `chat.readonly` | This desktop is read-only. You can read the report card and the examples, but chatting is switched off. | Dieser Desktop ist schreibgeschützt. Zeugnis und Beispiele kannst Du lesen, Chatten ist aber abgeschaltet. |
| `chat.history` | Conversation | Gesprächsverlauf |
| `chat.system_label` | System prompt (optional) | Systemanweisung (optional) |
| `chat.system_help` | Instructions both models get before the chat, as in your examples. | Anweisungen, die beide Modelle vor dem Chat bekommen, wie in Deinen Beispielen. |
| `chat.input_label` | Your message | Deine Nachricht |
| `chat.input_placeholder` | Write a message. Enter sends, Shift+Enter starts a new line. | Schreib eine Nachricht. Enter sendet, Umschalt+Enter beginnt eine neue Zeile. |
| `chat.send` | Send | Senden |
| `chat.stop` | Stop | Stopp |
| `chat.new` | New chat | Neuer Chat |
| `chat.not_saved` | The chat is not saved. It ends when you leave this step or pick another run. | Der Chat wird nicht gespeichert. Er endet, wenn Du diesen Schritt verlässt oder einen anderen Lauf wählst. |
| `chat.compare` | Compare with the base model | Mit dem Basismodell vergleichen |
| `chat.compare_fresh` | The base model joins from your next message on. | Das Basismodell ist ab Deiner nächsten Nachricht dabei. |
| `chat.column_tuned` | Your model | Dein Modell |
| `chat.you` | You: | Du: |
| `chat.empty` | Ask your model something, or take an example from the Examples view. | Frag Dein Modell etwas oder nimm ein Beispiel aus der Ansicht Beispiele. |
| `chat.not_compared` | Not compared | Nicht verglichen |
| `chat.waiting` | Waiting for the model… | Warte auf das Modell… |
| `chat.loading` | Loading the model. The first answer takes longer. | Das Modell wird geladen. Die erste Antwort dauert länger. |
| `chat.announce_start` | The answer is being written. | Die Antwort wird geschrieben. |
| `chat.announce_start_both` | Both models are answering. | Beide Modelle antworten. |
| `chat.announce_done` | The answer is complete. | Die Antwort ist fertig. |
| `chat.announce_done_both` | Both answers are complete. | Beide Antworten sind fertig. |
| `chat.announce_stopped` | Stopped. | Gestoppt. |
| `chat.announce_error` | An answer failed. | Eine Antwort ist fehlgeschlagen. |
| `chat.stats` | {tokens} tokens · {speed} tokens per second · {backend} | {tokens} Tokens · {speed} Tokens pro Sekunde · {backend} |
| `chat.cut_length` | The answer reached the length limit. | Die Antwort hat die Längengrenze erreicht. |
| `chat.error_start_failed` | The model could not be started. | Das Modell ließ sich nicht starten. |
| `chat.error_generation_failed` | The model stopped with an error while answering. | Das Modell ist beim Antworten mit einem Fehler stehen geblieben. |
| `chat.error_context_exceeded` | This chat is too long for the model's memory. Start a new chat. | Dieser Chat ist zu lang für das Gedächtnis des Modells. Beginne einen neuen Chat. |
| `chat.error_runtime_busy` | Both test slots are in use by other models. Try again in a moment. | Beide Testplätze sind von anderen Modellen belegt. Versuch es gleich noch einmal. |
| `chat.error_cancelled` | Stopped by you. | Von Dir gestoppt. |
| `chat.error_connection_lost` | The connection broke off. The answer may be incomplete. | Die Verbindung ist abgebrochen. Die Antwort ist vielleicht unvollständig. |
| `chat.error_unknown` | Something went wrong with this answer. | Bei dieser Antwort ist etwas schiefgegangen. |
| `chat.limit_turns` | This chat has reached {max} messages. Start a new chat. | Dieser Chat hat {max} Nachrichten erreicht. Beginne einen neuen Chat. |
| `chat.limit_chars` | This chat is longer than {max} characters. Start a new chat or shorten your message. | Dieser Chat ist länger als {max} Zeichen. Beginne einen neuen Chat oder kürze Deine Nachricht. |
| `chat.chars` | {count} of {max} characters | {count} von {max} Zeichen |
| `chat.busy` | Wait until the answer is complete, or press Stop. | Warte, bis die Antwort fertig ist, oder drück auf Stopp. |
| `chat.advanced` | Advanced: answer settings | Erweitert: Antworteinstellungen |
| `chat.advanced_help` | Empty fields use the values the model was trained with. | Leere Felder nutzen die Werte, mit denen das Modell trainiert wurde. |
| `chat.temperature` | Temperature (0 to 2) | Temperatur (0 bis 2) |
| `chat.top_p` | Top-p (above 0 up to 1) | Top-p (über 0 bis 1) |
| `chat.top_k` | Top-k (0 to 200) | Top-k (0 bis 200) |
| `chat.max_tokens` | Longest answer in tokens (1 to {max}) | Längste Antwort in Tokens (1 bis {max}) |
| `chat.default_value` | Default: {value} | Standard: {value} |
| `chat.reset` | Reset to model defaults | Auf Modellwerte zurücksetzen |
| `chat.invalid` | Enter a number from {min} to {max}. | Gib eine Zahl von {min} bis {max} ein. |

  - Reused existing keys:
    - `primary.chat_model` is the chat heading;
    - `run.confusion_base` / `run.confusion_tuned` are the answer column titles;
    - `editor.role_*` and `review.role_text` label the prompt messages;
    - `table.prev`, `table.next` and `table.page` drive the pager;
    - `retry`, `loading` and `readonly_banner` as before.
  - States and their texts:

| State | Text |
|---|---|
| no succeeded run yet | `test.empty_*` |
| run without base model | `test.base_missing` / `test.base_skipped` / `test.base_invalid` |
| runtime disabled | `runtime.state_disabled` plus `runtime.disabled_admin` or `runtime.disabled_user` |
| platform unsupported | `runtime.state_unsupported` |
| not installed | `runtime.install_text` (admin) / `runtime.install_ask` (everyone else) |
| installing with progress | `runtime.state_installing` plus `runtime.progress` |
| install failed | `runtime.failed_*` |
| model loading | `chat.loading` |
| generating | streaming cursor plus `chat.announce_start*` |
| stopped by the user | `chat.error_cancelled` |
| error per target | `chat.error_*` |
| both slots busy | `error.runtime_busy` / `chat.error_runtime_busy` |
| context exceeded | `chat.error_context_exceeded` |
| GPU fallback | `runtime.gpu_fallback` |
| read-only desktop | `chat.readonly` |
| smoke result line | `test.smoke_ok` / `test.smoke_failed` / `test.smoke_skipped*` |

- [ ] **9. Lists and loader (contract change 1).**
  - `module-loader.js:43`: insert `'/js/desktop/apps/chibi-sensei/chibi-sensei-chat.js', '/js/desktop/apps/chibi-sensei/chibi-sensei-test.js'` after `-run.js`. No `npm run build:ui`: the module loader is not part of `main.bundle.js` (`scripts/build-ui-bundles.js` does not read it, and the bundle names no Chibi script). `npm run check:ui` confirms this.
  - `ui/desktop_chibi_sensei_guards_test.go:49-50`: `12` becomes `14` (twice).
  - `ui/desktop_chibi_sensei_registration_test.go:13`: append `"chat", "test"`.
  - `ui/i18n_lint_test.go`: after line 1123 add `"js/desktop/apps/chibi-sensei/chibi-sensei-chat.js": {name: "t", prefix: "chibi_sensei."},` and the same for `-test.js`. Run `gofmt` to align the map.
- [ ] **10. Model test.** Append `'chibi-sensei-chat.js', 'chibi-sensei-test.js'` to `files` and add these checks:
  - reducer:
    - `send` with compare off gives no base answer; with compare on, both answers are `waiting`;
    - `send` while streaming changes nothing;
    - `status` loading gives `loading`; two deltas concatenate; `done` stores the stats;
    - error `cancelled` gives `stopped`; a strange code gives `error` with `unknown`;
    - `close` not ended turns in-flight answers into `connection_lost`; `close` aborted gives `stopped`;
    - `close` with an error and no event removes the turn, restores the draft and sets `notice`;
    - `new_chat` while streaming is ignored; `compare` on mid-chat sets `compareFrom`;
  - `conversation`:
    - leading system prompt;
    - a turn with a failed tuned answer is left out for tuned;
    - base skips turns without a base answer;
    - a stopped answer with text is kept;
  - `canSend`:
    - empty text gives no reason;
    - 20 usable turns give `chat.limit_turns`;
    - 16,001 code points (`'😀'.repeat(16001)`) give `chat.limit_chars`, while 16,000 pass;
  - `requestBody`: the targets order, the last message is user, `sampling` is left out when empty;
  - `validateSampling`: `'0,7'` gives 0.7; empty fields are left out; `top_p: '0'`, `top_k: '201'` and `max_tokens: '1025'` (cap 1024) are issues; `top_k: '2.5'` is an issue;
  - `runtimeView`: admin and non-admin on `not_installed`, `ready` with composer, `installing`, `disabled`, read-only install reason, `undefined` gives `loading`, `null` gives `error`;
  - `answerView`: the three phases map to `streaming`;
  - test helpers:
    - `succeededRuns` filters and sorts;
    - `smokeLine` for null, ok, failed and the three skip codes plus an unknown one;
    - `modelLine` for the three base reasons and an unknown reason;
    - `promptOf` for messages with system, a multi-turn prompt (last user) and a text prompt.
- [ ] **11. Verify**
  ```bash
  git diff --check
  for f in chat test run training; do node --check ui/js/desktop/apps/chibi-sensei/chibi-sensei-$f.js; done; node --check ui/js/desktop/apps/chibi-sensei/chibi-sensei.js; node --check ui/js/desktop/core/module-loader.js
  node -e "for (const f of process.argv.slice(1)) JSON.parse(require('fs').readFileSync(f,'utf8'))" ui/lang/chibi_sensei/en.json ui/lang/chibi_sensei/de.json
  node scripts/test-chibi-sensei-model.mjs
  go build ./...
  go test ./ui -run '(?i)(ChibiSensei|Translations_|Frontend_|DesktopAppMaterials)' -skip '(?i)(TestChibiSenseiLocalesCoverAllSixteenLanguages|TestTranslations_AllKeysPresentInAllLanguages)' -count=1
  go test ./ui -run '(?i)(TestChibiSenseiLocalesCoverAllSixteenLanguages|TestTranslations_AllKeysPresentInAllLanguages)' -count=1
  ```
  Expected: the first `go test` prints `ok`. The second fails by design, and only with lines that name a new key missing from one of the 14 languages, or `chibi_sensei.later.test_text` present where English lacks it. `de` must not appear in either kind of line.

**Done when** the commands give the expected results and `chibi-sensei-chat.js` and `chibi-sensei-test.js` are each under 1100 lines.

---

## F3: translations, exception lists, guides

**Goal:** all 16 languages carry the F2 keys and the removal; the test lists accept genuine shared words; the guides describe P3.

**Files**
- Modify:
  - `ui/lang/chibi_sensei/{cs,da,el,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`
  - `ui/desktop_chibi_sensei_i18n_test.go`
  - `ui/AGENTS.md:140`, `ui/js/desktop/apps/AGENTS.md:169-177`, `scripts/AGENTS.md` (Chibi Sensei verification line)
- Scratch:
  - `C:\disposable\chibi-sensei\p3\apply-locale-p3.mjs`
  - `C:\disposable\chibi-sensei\p3\f3-en-new.json`
  - `C:\disposable\chibi-sensei\p3\tr\<lang>.json` (14)

### Steps

- [ ] **1. Fragments.** From the worktree root:
  ```bash
  node -e "const fs=require('fs'),r=p=>JSON.parse(fs.readFileSync(p,'utf8'));const en=r('ui/lang/chibi_sensei/en.json'),cs=r('ui/lang/chibi_sensei/cs.json');const o={};for(const k of Object.keys(en))if(!(k in cs))o[k]=en[k];fs.writeFileSync('C:/disposable/chibi-sensei/p3/f3-en-new.json',JSON.stringify(o,null,2)+'\n');console.log(Object.keys(o).length, Object.keys(cs).filter(k=>!(k in en)))"
  ```
  Expected: the number of F2 keys (≈118) and `[ 'chibi_sensei.later.test_text' ]`.
- [ ] **2. Merge script.** Copy `C:\disposable\chibi-sensei\p2\apply-locale-p2.mjs` to `C:\disposable\chibi-sensei\p3\apply-locale-p3.mjs` with these changes:
  - `here = 'C:/disposable/chibi-sensei/p3'`;
  - the inputs are `f3-en-new.json` and `f3-en-changed.json`;
  - before writing, compute `dropped = keys of the locale file that en.json lacks`, and fail (writing nothing) unless `dropped` is exactly `['chibi_sensei.later.test_text']` or empty;
  - keep the existing checks: exact key set, same `{placeholder}` names, no empty value, no U+FFFD, no literal `\u` escape;
  - print `equal_to_english` as before.
- [ ] **3. Translate.**
  - Write one flat JSON fragment per language to `C:\disposable\chibi-sensei\p3\tr\<lang>.json`, holding exactly the keys of `f3-en-new.json`.
  - Before writing, read ten existing entries of that locale and use the same register (the informal address where the language has one) and the same terms as the existing keys: the words for report, base model, trained model, examples, run.
  - Keep `{placeholders}`, "llama.cpp", "CUDA", "Vulkan", "Metal", "NVIDIA", "Apple", "AuraGo" and "Chibi Sensei".
  - Keyboard names follow the platform convention of the language ("Enter", "Shift" or the local word).
  - Then run 14 times: `node C:/disposable/chibi-sensei/p3/apply-locale-p3.mjs . <lang> C:/disposable/chibi-sensei/p3/tr/<lang>.json`. Each run prints one JSON line and writes the file in English key order.
- [ ] **4. Exception lists in `ui/desktop_chibi_sensei_i18n_test.go`.**
  - Add `"chibi_sensei.test.view_chat": true` to the global `same` map (line 48-55), with the comment "Chat is the loan word in most languages".
  - Add a per-language entry to `sameIn` only for a key that `equal_to_english` reported, and only where the translation genuinely is the English word. Expected candidates: `chibi_sensei.test.prompt` for `pl`, `pt`, `sv`, like `table.col_prompt` today; possibly `chibi_sensei.chat.stop` or `runtime.install` in a language that borrows the word.
  - Every other key in `equal_to_english` must be translated, not listed.
- [ ] **5. Guides.**
  - `ui/AGENTS.md:140`: add after the P2 sentence: "P3 adds the Test step (report card, evaluation examples, chat with the trained model and a side-by-side comparison with the base model in a local llama.cpp runtime the server supervises)."
  - `ui/js/desktop/apps/AGENTS.md`:
    - line 171: "twelve classic scripts" becomes "fourteen". Before the root, add `-chat.js` (runtime strip, admin install with confirmation, streamed chat of one or two targets with a pure reducer, own conversation per target, 20 turns and 16,000 code points per conversation, advanced sampling) and `-test.js` (Test step: run picker, tab pattern with report card from `P.run.fillReport`, paged examples with match marks and "Try in chat", report-seen only on a writable desktop). `-api.js` also reads the streamed chat POST through `createSseParser`. `-run.js` exports `fillReport`.
    - line 173: "test and use stay `todo`" becomes "test follows the server (`ready` with a succeeded run, `done` after the report card was opened); the Test primary action opens the chat and needs a succeeded run; use stays `todo`".
    - Add one bullet after line 175: "Test: model output is untrusted and reaches the page only as `textContent`; nothing is downloaded until an administrator confirms the install dialog (source host, backend, size); a read-only desktop keeps the report card, the examples and Stop models, hides the composer and never sends `report/seen`; the tab pattern, the polite live region (start and end of an answer, never tokens), Enter/Shift+Enter, focus after Send and Stop, and the scroll pin are the accessibility contract."
    - line 176: "(P1 fields and the training group)" becomes "(P1 fields, the training group and the local-testing group)".
  - `scripts/AGENTS.md`, Chibi Sensei line: add "the event-stream parser and `chatStream`, the chat reducer and limits, the runtime view and the Test-step helpers".
- [ ] **6. Verify**
  ```bash
  git diff --check
  node -e "for (const f of process.argv.slice(1)) JSON.parse(require('fs').readFileSync(f,'utf8'))" ui/lang/chibi_sensei/*.json
  node scripts/test-chibi-sensei-model.mjs
  go build ./...
  go test ./ui -run '(?i)(ChibiSensei|Translations_|Frontend_|DesktopAppMaterials|ConfigChibiSensei)' -count=1
  go test ./ui -count=1
  npm run check:ui
  ```
  Expected: `ok aurago/ui` twice (the browser tests skip without the opt-in), and no drift from `check:ui`. When B4 has landed, the coordinator runs `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run TestChibiSenseiAppRealServerBrowser -count=1 -timeout 15m`.

**Done when** all 16 locales hold the same key set (en has no `later.test_text`), every listed command passes and the three guides name the P3 modules.

---

## Existing functions changed

| Function / symbol | File:line | Batch | Kind of change |
|---|---|---|---|
| `CHIBI_SENSEI_DEFAULTS` | `ui/cfg/chibi_sensei.js:9` | F1 | adds `runtime` defaults |
| `renderChibiSenseiSection` | `ui/cfg/chibi_sensei.js:50` | F1 | adds the local-testing group |
| number rules object | `ui/js/config/catalog.js:145-150` | F1 | two entries |
| `TestConfigChibiSenseiSection` | `ui/config_chibi_sensei_test.go:11` | F1 | new markers |
| `ERROR_CODES` | `APP/chibi-sensei-core.js:277` | F1 | 6 codes |
| core exports | `APP/chibi-sensei-core.js:397-425` | F1 | `CHAT_TARGETS`, `RUNTIME_STATES`, `SAMPLE_FILTERS`, `CHAT_LIMITS`, `chatLimits`, `charCount` |
| `createApi` | `APP/chibi-sensei-api.js:68` | F1 | 7 methods, inner `chatStream`; new module-level `createSseParser` (export near :235) |
| `primaryFor` | `APP/chibi-sensei-steps.js:62` (branch :84) | F1 | Test returns `open_chat`; new opt `hasSucceeded` |
| `TIPS` | `APP/chibi-sensei-steps.js:88-94` | F1 | `test_ready`, `test_done` |
| `recompute` | `APP/chibi-sensei.js:372` | F1 | passes `hasSucceeded` |
| `runPrimary` | `APP/chibi-sensei.js:385` | F1 | `open_chat` branch |
| `enterStep` | `APP/chibi-sensei.js:339` (line 349) | F2 | routes `test` to `P.test.create` |
| `createArea` (`fillSuccess` :549, `confusionBox` :579, `paintResult` :528) | `APP/chibi-sensei-run.js:211` | F2 | lifted to module-level `fillReport` and `confusionBox`, output unchanged |
| `P.run` export | `APP/chibi-sensei-run.js:706` | F2 | adds `fillReport` |
| `LATER_KEYS`, `createLater` | `APP/chibi-sensei-training.js:17`, `:891` | F2 | Use only |
| chibi-sensei loader entry | `ui/js/desktop/core/module-loader.js:43` | F2 | two scripts |
| `TestChibiSenseiSourcesAreSafeAndSmall` | `ui/desktop_chibi_sensei_guards_test.go:49-50` | F2 | count 14 |
| `chibiSenseiModules` | `ui/desktop_chibi_sensei_registration_test.go:13` | F2 | `chat`, `test` |
| `desktopLocalI18nHelpers` | `ui/i18n_lint_test.go:1113-1124` | F2 | two rows |
| `TestChibiSenseiTranslationsAreTranslated` | `ui/desktop_chibi_sensei_i18n_test.go:48-78` | F3 | exception entries |

## Open risks

1. **How the server counts characters.** The client counts code points. If B4 counts bytes or UTF-16 units, a conversation near 16,000 can pass the client check and get a 422. The 422 is shown, so it is not silent, but the counts should match B4's validator.
2. **Proxy buffering of a streamed POST.** The `X-Accel-Buffering: no` header covers nginx. The managed Caddy proxy or another reverse proxy may still buffer or compress, and then answers would appear all at once at the end. That needs checking on `aurago-test` behind the proxy.
3. **Stream framing.** The parser follows the HTML standard, and the contract leaves the real llama.cpp framing to B2. The AuraGo-side framing (`event:` and `data:` per line, `: ping`) must come from B4 exactly as the contract shows, because the client drops unknown events silently.
4. **`report/seen` on a read-only desktop.** It is never sent there (contract change 9), so the Test step stays `ready` on read-only desktops. If B4 classifies the route as read-class, the skip can be removed.
5. **Comparison columns do not line up.** In compare mode the two conversations are separate columns in one scroll area, so long answers push one column's turns below the other's. This is accepted for P3; a turn-row layout would need a different `chat-column` contract.
6. **Translation volume.** ≈118 new keys × 14 languages in F3, plus 9 keys and 13 config keys in F1. The merge script catches key and placeholder errors, but not quality; the combined review should spot-check de-adjacent languages (nl, da, no, sv) for English copies.
7. **Chat ends silently on leave.** Leaving the step or picking another run while an answer streams aborts it without asking. `chat.not_saved` says so up front; a confirmation would add a second dirty guard to the root.
