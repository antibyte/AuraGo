# Part D — storyboard panel (UI)

Scope: Video Studio storyboard panel (brief §11, spec §6). Vanilla JS, two new classic scripts (`window.VideoStudioPlanBoard`, `window.VideoStudioPlan`), the header switch in the existing shell, a CSS section, the script lists, 122 `videoStudio.plan.*` strings in all 16 desktop locales, one strings test, one browser tour, and the AGENTS.md contract lines. No Go server code. The part uses only the API shapes in brief §6/§7 and the status fields `status.plan` and `status.generation.profile`.

Findings that shaped the design (all checked against the code):
- The shell's host listeners (`handleClick`, `handleChange`, `handleInput` in `ui/js/desktop/apps/video-studio.js`) react to `data-action`, `data-field`, `data-transition`, `data-position`, `data-fit`, `data-text-*`, `data-sticker` and others. So every board control uses `data-plan-*` attributes only.
- `P.request` (`video-studio-core.js`) throws on every non-2xx status, including 304. The plan code catches `error.status === 304`.
- Load order is fixed by `TestVideoStudioNavigationLoadOrder` (`ui/desktop_video_studio_navigation_test.go`). It checks `module-loader.js`, the UI fixture (`ui/desktop_video_studio_browser_test.go`) and the server fixture (`internal/server/video_studio_browser_test.go`). AGENTS.md says "lists only get insertions". The camera-handoff fixtures keep their own lists, so the shell calls the plan globals behind `window.VideoStudioPlan ? … : …` guards.
- `TestVideoStudioCountStringsHaveSingulars` (`ui/desktop_video_studio_plurals_test.go`) requires a `_one` key in all 16 locales for every `videoStudio.*` string that contains `{{count}}`. It also requires `trCount` for count strings. The plan uses `P.trCount` for `plan.scenes` and `plan.blocking` and ships `_one` variants.
- `npm run check:ui` only checks the generated bundles (`build-ui-bundles.js --check`, `build-chat-ocean-life.js --check`). Video Studio scripts and desktop strings are not bundled, so it is a sanity run here. The strings get their own Go test.
- Locale files are flat `"key": "value"` maps with LF line endings and two-space indent. Feature keys are grouped, and in all 16 files the Video Studio group ends with `"videoStudio.sourceAspectExtreme"`. New keys go right after the last `  "videoStudio.` line.
- Phone toolbar budget: at 390 px the toolbar already uses about 374 px. At 600 px or less the switch therefore takes its own toolbar row (`flex-wrap`). Groups with `flex: 1 1 0` never wrap, so layouts without the switch do not change.
- The existing UI browser fixture's status has no `plan` key, so the switch stays hidden there and its selectors are untouched. The server fixture (after Part B) shows the switch at 1280 px. It uses no `data-action` and no other selector clashes.

GitNexus (CLAUDE.md): run `impact` before editing the shell functions (D1 step 0) and `detect_changes` before every commit, from the main checkout. Worktrees cannot be indexed. JS callers go through `P.*` wrappers, so expect `risk: UNKNOWN` and confirm with a text search as the step says.

---

### Task D1: Storyboard panel scripts, header switch, CSS and script lists

**Files:**
- Create: `ui/js/desktop/apps/video-studio-plan-board.js` (`window.VideoStudioPlanBoard`)
- Create: `ui/js/desktop/apps/video-studio-plan.js` (`window.VideoStudioPlan`)
- Modify: `ui/js/desktop/apps/video-studio.js` (`dispose`, `layoutMarkup` header + centre column, `appRender` install + close guard, `renderToolbar`, `handleKeys`)
- Modify: `ui/css/desktop-app-video-studio.css` (append section `/* === Plan mode (storyboard) === */`)
- Modify: `ui/js/desktop/core/module-loader.js` (`'video-studio'` scripts list, ~line 338)
- Modify: `ui/desktop_video_studio_browser_test.go` (fixture script tags, line ~67)
- Modify: `internal/server/video_studio_browser_test.go` (fixture script tags, lines ~106–107; test file only)
- Test: `ui/desktop_video_studio_navigation_test.go` (`TestVideoStudioNavigationLoadOrder`)

- [ ] **Step 0: Impact before editing (main checkout `C:\Users\Andi\Documents\repo\AuraGo`)**

```
node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "renderToolbar" --direction upstream --repo .
node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "handleKeys" --direction upstream --repo .
node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "appRender" --direction upstream --repo .
node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "layoutMarkup" --direction upstream --repo .
node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "dispose" --direction upstream --repo .
```
Report callers, processes and risk. Warn on HIGH/CRITICAL. If the verdict is `UNKNOWN`, confirm the callers with a text search (`P.renderToolbar`, `renderToolbar(s)`, `handleKeys(s, event)`). Every edit below only adds guarded lines, so existing behaviour stays the same while the plan globals are absent.

- [ ] **Step 1: Write the failing test.** In `ui/desktop_video_studio_navigation_test.go`, inside `TestVideoStudioNavigationLoadOrder`, replace:

**Old:**
```go
"video-studio-menus.js", "video-studio.js"}
```
**New:**
```go
"video-studio-menus.js", "video-studio-plan-board.js", "video-studio-plan.js", "video-studio.js"}
```

- [ ] **Step 2: Run it.**
`go test ./ui -run '(?i)TestVideoStudioNavigationLoadOrder' -count=1`
Expected: FAIL, including `module loader does not load video-studio-plan-board.js`, `ui browser fixture does not load video-studio-plan.js`, `server browser fixture does not load video-studio-plan-board.js`.

- [ ] **Step 3: Create `ui/js/desktop/apps/video-studio-plan-board.js`:**

```js
(function () {
    'use strict';

    // Plan mode storyboard (DOM only). VideoStudioPlan (video-studio-plan.js) owns state, requests and edits and hands
    // this file a view (VideoStudioPlan.view(s)): the board shell in the centre column, the loading, error, unsupported
    // and empty states, the header with the idea and the agent status line, the plan-level fields (style, characters,
    // music, clip sound), one card per scene with its transition chip, the inline issues, the bottom bar and the start
    // and revise dialogs. Controls carry data-plan-* attributes only: the shell's click, input and change handlers react
    // to data-action, data-field, data-transition and similar names, so none of those appear here.
    const P = window.VideoStudioParts;
    const tr = (s, key, fallback) => P.tr(s, key, fallback);
    const esc = (s, value) => P.esc(s, value);
    const icon = (name, size, cls) => P.icon(name, size, cls);
    const fill = (text, vars) => P.fill(text, vars);
    const label = (s, key, fallback) => esc(s, tr(s, key, fallback));
    const own = (map, key) => Object.prototype.hasOwnProperty.call(map, key);

    const TARGETS = Array.from({ length: 23 }, (_, index) => 10 + index * 5);
    const MAX_SCENES = 12;
    const MAX_CHARACTERS = 6;
    const REASONS = {
        provider_images_public_url_only: 'This video provider accepts start images only as public web addresses.',
        provider_unsupported: 'The configured video provider does not support plan mode.',
        generation_unavailable: 'Video generation is not set up.'
    };
    const PHASES = { outline: 'Scene outline', details: 'Scene details', check: 'Checking the plan', revise: 'Revising a scene' };
    const KINDS = { cut: 'Cut', continue: 'Seamless', bridge: 'Bridge', blend: 'Blend' };
    const SOURCES = { generated: 'Generated image', library: 'Library image', none: 'Text only', chain: 'Continues from scene {{number}}', bridge: 'Shared frame with scene {{number}}' };
    const CLIP_AUDIO = { keep: 'Keep', duck: 'Lower', mute: 'Mute' };
    // Where an issue shows: the slot of its code in the scene card (issues with a scene_id) or in the plan section. A slot
    // that is not on screen (the first card has no transition chip) falls back to the card, plan issues to the header.
    const CODE_SLOTS = {
        title_missing: 'title', title_too_long: 'title', prompt_missing: 'video_prompt', prompt_too_long: 'video_prompt',
        negative_prompt_too_long: 'negative_prompt', duration_unsupported: 'timing', use_seconds_invalid: 'timing',
        transition_unknown: 'transition', transition_unsupported: 'transition', first_scene_transition: 'transition',
        chain_too_long: 'transition', blend_type_unknown: 'transition', blend_type_unavailable: 'transition',
        blend_overlap_invalid: 'transition', keyframe_source_invalid: 'keyframe', keyframe_prompt_missing: 'keyframe',
        keyframe_asset_invalid: 'keyframe', narration_unavailable: 'narration', narration_too_long: 'narration',
        style_too_long: 'style', too_many_characters: 'characters', character_invalid: 'characters',
        character_reference_invalid: 'characters', music_unavailable: 'music', clip_audio_invalid: 'clip_audio'
    };
    const PLAN_SLOTS = new Set(['style', 'characters', 'music', 'clip_audio']);

    function num(value) {
        const n = Math.round((Number(value) || 0) * 100) / 100;
        try { return n.toLocaleString(P.uiLocale(), { maximumFractionDigits: 1 }); } catch (_) { return String(n); }
    }
    function secondsLabel(s, value) { return fill(tr(s, 'plan.secondsValue', '{{value}} s'), { value: num(value) }); }
    function durationText(s, scene) { return fill(tr(s, 'plan.duration', '{{use}} of {{generate}} s'), { use: num(scene.use_seconds), generate: num(scene.generate_seconds) }); }
    function option(s, value, text, selected, disabled) { return `<option value="${esc(s, value)}"${selected ? ' selected' : ''}${disabled ? ' disabled' : ''}>${esc(s, text)}</option>`; }
    function formId(s) { return 'vs-plan-create-' + s.id; }

    function shellMarkup(s) {
        return `<section class="vs-plan" data-plan-board aria-label="${label(s, 'plan.board', 'Storyboard')}" hidden></section>`;
    }
    // Full redraw. The focused field (data-plan-key), its caret and the scroll position survive.
    function render(s, view) {
        const host = s.q('[data-plan-board]');
        if (!host) return;
        const active = document.activeElement;
        const key = active && host.contains(active) && active.dataset ? active.dataset.planKey || '' : '';
        let selection = null;
        if (key) { try { selection = typeof active.selectionStart === 'number' ? [active.selectionStart, active.selectionEnd] : null; } catch (_) { selection = null; } }
        const scroller = host.querySelector('.vs-plan-scroll');
        const top = scroller ? scroller.scrollTop : 0;
        host.innerHTML = `<div class="vs-plan-scroll"><div class="vs-plan-inner">${body(s, view)}</div></div>${bar(s, view)}`;
        const next = host.querySelector('.vs-plan-scroll');
        if (next) next.scrollTop = top;
        if (key) {
            const el = host.querySelector(`[data-plan-key="${CSS.escape(key)}"]`);
            if (el && !el.disabled) {
                el.focus({ preventScroll: true });
                if (selection && typeof el.setSelectionRange === 'function') { try { el.setSelectionRange(selection[0], selection[1]); } catch (_) { /* not a text field */ } }
            }
        }
        renderIssues(s, view);
    }
    function body(s, view) {
        if (view.phase === 'loading') return `<div class="vs-plan-state" role="status">${icon('spinner', 18, 'vs-icon vs-spin')}<span>${label(s, 'plan.loading', 'Loading the plan…')}</span></div>`;
        if (view.phase === 'error') return `<div class="vs-plan-state" role="alert">${icon('alert', 18)}<span>${label(s, 'plan.loadFailed', 'The plan could not be loaded.')}</span><button type="button" class="vs-chip-button" data-plan-action="retry">${label(s, 'plan.retry', 'Try again')}</button></div>`;
        if (view.phase === 'unsupported') return unsupported(s, view);
        if (view.phase === 'empty') return empty(s, view);
        if (view.phase === 'planning' && !(view.plan.scenes || []).length) return head(s, view);
        return head(s, view) + settings(s, view) + scenes(s, view);
    }
    function unsupported(s, view) {
        const reason = String(view.availability.reason || '');
        const text = own(REASONS, reason) ? tr(s, 'plan.reason.' + reason, REASONS[reason]) : '';
        return `<div class="vs-plan-empty" data-plan-unsupported><span class="vs-modal-icon">${icon('alert', 22)}</span><h2>${label(s, 'plan.unsupportedTitle', 'Plan mode is not available')}</h2>${text ? `<p>${esc(s, text)}</p>` : ''}<p class="vs-hint">${label(s, 'plan.unsupportedHint', 'Choose a supported video provider in Settings > Video Generation.')}</p></div>`;
    }
    function empty(s, view) {
        const intro = `<span class="vs-modal-icon">${icon('sparkle', 22)}</span><h2>${label(s, 'plan.emptyTitle', 'Plan a clip with the agent')}</h2>`;
        // A damaged plan.json (Part B: GET 409 plan_invalid) shows this state with a notice; its create sends replace.
        const damaged = view.damaged ? `<p class="vs-plan-issue is-blocking" data-plan-damaged>${icon('alert', 13)}<span>${label(s, 'plan.damaged', 'The stored plan is damaged. Create a new plan to replace it.')}</span></p>` : '';
        if (view.readonly) return `<div class="vs-plan-empty" data-plan-empty>${intro}${damaged || `<p>${label(s, 'plan.emptyReadonly', 'This project has no plan yet.')}</p>`}<p class="vs-hint">${label(s, 'plan.readonlyHint', 'Read-only: the storyboard can be viewed but not changed.')}</p></div>`;
        return `<form class="vs-plan-empty" id="${esc(s, formId(s))}" data-plan-empty data-plan-create-form>${intro}${damaged}<p>${label(s, 'plan.emptyCopy', 'Describe your idea. The agent plans the scenes as a storyboard. Nothing is generated or paid for yet.')}</p>${ideaFields(s, '', 30)}${view.profileLine ? `<p class="vs-hint">${icon('info', 13)} ${esc(s, view.profileLine)}</p>` : ''}</form>`;
    }
    function ideaFields(s, idea, target) {
        const values = TARGETS.includes(target) ? TARGETS : TARGETS.concat(target).sort((a, b) => a - b);
        return `<label class="vs-field vs-field-wide"><span>${label(s, 'plan.idea', 'Idea')}</span><textarea data-plan-idea rows="5" maxlength="4000" required placeholder="${label(s, 'plan.ideaHint', '30 seconds: a fisherman heads out at dawn and the day wakes over the bay')}">${esc(s, idea)}</textarea></label>`
            + `<label class="vs-field vs-field-wide"><span>${label(s, 'plan.target', 'Target length')}</span><select data-plan-target>${values.map(value => option(s, value, secondsLabel(s, value), value === target)).join('')}</select></label>`;
    }
    function head(s, view) {
        const plan = view.plan, agent = view.agent;
        const profile = view.profileLine ? `<small>${esc(s, view.profileLine)}</small>` : '';
        let status;
        if (view.phase === 'planning') {
            // A job that just started has no phase yet (Part C's Start): a revise job has one phase, create starts with the outline.
            const phase = agent && own(PHASES, agent.phase) ? agent.phase : agent && agent.mode === 'revise' ? 'revise' : 'outline';
            const stop = view.readonly ? '' : `<button type="button" class="vs-chip-button vs-plan-stop" data-plan-action="stop">${icon('close', 14)}<span>${label(s, 'plan.stop', 'Stop')}</span></button>`;
            status = `${icon('spinner', 15, 'vs-icon vs-spin')}<strong>${label(s, 'plan.planning', 'Planning')}</strong><span data-plan-phase="${esc(s, phase)}">${label(s, 'plan.phase.' + phase, PHASES[phase])}</span>${profile}${stop}`;
        } else {
            const stopped = !!agent && (agent.state === 'stopped' || agent.error === 'stopped');
            const failed = !!agent && agent.state === 'failed';
            const ended = stopped ? label(s, 'plan.agentStopped', 'Planning was stopped. The plan keeps what was planned so far.') : failed ? label(s, 'plan.agentFailed', 'Planning ended early. The plan keeps what was planned so far.') : '';
            status = `${icon('sparkle', 15)}${profile}${ended ? `<span class="vs-plan-ended">${ended}</span>` : ''}`;
        }
        return `<header class="vs-plan-head"><div class="vs-plan-idea"><p class="vs-eyebrow">${label(s, 'plan.idea', 'Idea')}</p><p>${esc(s, plan.idea || '')}</p></div><div class="vs-plan-agent" data-plan-agent role="status">${status}</div>${view.readonly ? `<p class="vs-hint">${label(s, 'plan.readonlyHint', 'Read-only: the storyboard can be viewed but not changed.')}</p>` : ''}<div data-plan-issues="plan:general"></div></header>`;
    }
    function settings(s, view) {
        const plan = view.plan, dis = view.locked ? ' disabled' : '';
        const characters = plan.characters || [];
        const name = label(s, 'plan.characterName', 'Name');
        const look = label(s, 'plan.characterDescription', 'Look and clothing');
        const remove = label(s, 'plan.removeCharacter', 'Remove character');
        const rows = characters.map((character, index) => `<div class="vs-plan-character"><input type="text" data-plan-field="character.name" data-plan-index="${index}" data-plan-key="character:${index}:name" maxlength="80" value="${esc(s, character.name || '')}" placeholder="${name}" aria-label="${name}"${dis}><textarea data-plan-field="character.description" data-plan-index="${index}" data-plan-key="character:${index}:description" rows="2" maxlength="600" placeholder="${look}" aria-label="${look}"${dis}>${esc(s, character.description || '')}</textarea>${view.locked ? '' : `<button type="button" class="vs-icon-button" data-plan-action="remove-character" data-plan-index="${index}" aria-label="${remove}" title="${remove}">${icon('trash', 15)}</button>`}</div>`).join('');
        const add = view.locked ? '' : `<button type="button" class="vs-chip-button" data-plan-action="add-character"${characters.length >= MAX_CHARACTERS ? ' disabled' : ''}>${icon('plus', 14)}<span>${label(s, 'plan.addCharacter', 'Add character')}</span></button>`;
        const music = plan.audio.music || {};
        const musicField = view.availability.music || music.prompt ? `<label class="vs-field"><span>${label(s, 'plan.music', 'Music prompt')}</span><textarea data-plan-field="music.prompt" data-plan-key="plan:music" rows="2" maxlength="2000"${dis}>${esc(s, music.prompt || '')}</textarea></label><div data-plan-issues="plan:music"></div>` : '';
        const clip = own(CLIP_AUDIO, plan.audio.clip_audio) ? plan.audio.clip_audio : 'duck';
        return `<details class="vs-plan-settings" data-plan-settings${view.settingsOpen ? ' open' : ''}><summary>${icon('sliders', 15)}<span>${label(s, 'plan.planSettings', 'Style and sound')}</span></summary>`
            + `<label class="vs-field"><span>${label(s, 'plan.style', 'Style')}</span><textarea data-plan-field="style" data-plan-key="plan:style" rows="3" maxlength="2000"${dis}>${esc(s, plan.style || '')}</textarea></label><div data-plan-issues="plan:style"></div>`
            + `<div class="vs-plan-characters"><p class="vs-plan-label">${label(s, 'plan.characters', 'Characters')}</p>${rows}${add}<div data-plan-issues="plan:characters"></div></div>`
            + musicField
            + `<label class="vs-field"><span>${label(s, 'plan.clipAudio', 'Clip sound')}</span><select data-plan-field="clip_audio" data-plan-key="plan:clip_audio"${dis}>${Object.keys(CLIP_AUDIO).map(value => option(s, value, tr(s, 'plan.clipAudio.' + value, CLIP_AUDIO[value]), value === clip)).join('')}</select></label><div data-plan-issues="plan:clip_audio"></div></details>`;
    }
    function scenes(s, view) {
        const list = view.plan.scenes || [];
        if (!list.length) return `<div class="vs-plan-none"><p class="vs-hint">${label(s, 'plan.noScenes', 'This plan has no scenes yet.')}</p>${view.locked ? '' : `<button type="button" class="vs-chip-button" data-plan-action="add-scene">${icon('plus', 14)}<span>${label(s, 'plan.addScene', 'Add scene')}</span></button>`}</div>`;
        return `<ol class="vs-plan-scenes" data-plan-scenes>${list.map((scene, index) => `<li class="vs-plan-item">${index ? transition(s, view, scene) : ''}${card(s, view, scene, index)}</li>`).join('')}</ol>`;
    }
    // The chip before every scene but the first: the kinds the profile allows (plus a stored one it does not, so its
    // issue stays readable); a blend adds the type and the overlap.
    function transition(s, view, scene) {
        const id = esc(s, scene.id), dis = view.locked ? ' disabled' : '';
        const value = scene.transition_in || {};
        const kind = value.kind || 'cut';
        const kinds = view.kinds.includes(kind) ? view.kinds : view.kinds.concat(kind);
        const name = label(s, 'plan.transition', 'Transition');
        let blend = '';
        if (kind === 'blend') {
            const types = !value.type || view.blendTypes.some(item => item.id === value.type) ? view.blendTypes : view.blendTypes.concat({ id: value.type, label: value.type });
            const typeName = label(s, 'plan.blendType', 'Blend type');
            blend = `<select data-plan-field="transition.type" data-plan-scene="${id}" data-plan-key="${id}:transition.type" aria-label="${typeName}" title="${typeName}"${dis}>${types.map(item => option(s, item.id, item.label, item.id === value.type)).join('')}</select>`
                + `<label class="vs-plan-inline"><span>${label(s, 'plan.blendSeconds', 'Overlap (s)')}</span><input type="number" min="0.5" max="2" step="0.1" data-plan-field="transition.seconds" data-plan-scene="${id}" data-plan-key="${id}:transition.seconds" value="${esc(s, Number(value.seconds) || '')}"${dis}></label>`;
        }
        return `<div class="vs-plan-transition" data-plan-transition="${id}">${icon('transition', 15)}<select data-plan-field="transition.kind" data-plan-scene="${id}" data-plan-key="${id}:transition.kind" aria-label="${name}" title="${name}"${dis}>${kinds.map(item => option(s, item, tr(s, 'plan.kind.' + item, own(KINDS, item) ? KINDS[item] : item), item === kind)).join('')}</select>${blend}<div data-plan-issues="${id}:transition"></div></div>`;
    }
    function card(s, view, scene, index) {
        const id = esc(s, scene.id), dis = view.locked ? ' disabled' : '';
        const attrs = field => `data-plan-field="${field}" data-plan-scene="${id}" data-plan-key="${id}:${field}"`;
        const count = (view.plan.scenes || []).length;
        const generate = Number(scene.generate_seconds) || 0;
        const durations = !generate || view.durations.includes(generate) ? view.durations : view.durations.concat(generate).sort((a, b) => a - b);
        const keyframe = scene.keyframe || {};
        const narration = scene.narration || {};
        const kind = (scene.transition_in && scene.transition_in.kind) || 'cut';
        const joined = index > 0 && (kind === 'continue' || kind === 'bridge');
        const title = label(s, 'plan.title', 'Title');
        const number = esc(s, fill(tr(s, 'plan.sceneNumber', 'Scene {{number}}'), { number: index + 1 }));
        const negative = label(s, 'plan.negativePrompt', 'Negative prompt');
        const iconAction = (action, iconName, key, fallback, disabled, iconCls, cls) => `<button type="button"${cls ? ` class="${cls}"` : ''} data-plan-action="${action}" data-plan-scene="${id}" aria-label="${label(s, key, fallback)}" title="${label(s, key, fallback)}"${disabled ? ' disabled' : ''}>${icon(iconName, 14, iconCls)}</button>`;
        const actions = view.locked ? '' : `<div class="vs-clip-actions vs-plan-actions">`
            + `<button type="button" data-plan-action="revise" data-plan-scene="${id}">${icon('sparkle', 14)}<span>${label(s, 'plan.revise', 'Revise scene')}</span></button>`
            + `<button type="button" data-plan-action="text-only" data-plan-scene="${id}"${keyframe.source === 'none' || joined ? ' disabled' : ''}>${icon('text', 14)}<span>${label(s, 'plan.source.none', 'Text only')}</span></button>`
            + iconAction('up', 'chevronDown', 'plan.moveUp', 'Move up', index === 0, 'vs-icon vs-plan-up')
            + iconAction('down', 'chevronDown', 'plan.moveDown', 'Move down', index === count - 1)
            + iconAction('add-after', 'plus', 'plan.addScene', 'Add scene', count >= MAX_SCENES)
            + iconAction('remove', 'trash', 'plan.removeScene', 'Remove scene', false, undefined, 'vs-danger')
            + `</div>`;
        const narrationField = view.availability.tts || narration.text ? `<label class="vs-field"><span>${label(s, 'plan.narration', 'Narration')}</span><textarea ${attrs('narration.text')} rows="2" maxlength="600"${dis}>${esc(s, narration.text || '')}</textarea></label><div data-plan-issues="${id}:narration"></div>` : '';
        return `<article class="vs-plan-card" data-plan-card="${id}" aria-label="${number}">`
            + `<div class="vs-plan-card-head"><span class="vs-plan-num" aria-hidden="true">${index + 1}</span><input type="text" ${attrs('title')} maxlength="120" value="${esc(s, scene.title || '')}" placeholder="${title}" aria-label="${title}"${dis}><span class="vs-plan-chip" data-plan-duration="${id}">${esc(s, durationText(s, scene))}</span></div><div data-plan-issues="${id}:title"></div>`
            + `<label class="vs-field"><span>${label(s, 'plan.videoPrompt', 'Video prompt')}</span><textarea ${attrs('video_prompt')} rows="3" maxlength="${view.maxPrompt}"${dis}>${esc(s, scene.video_prompt || '')}</textarea></label><div data-plan-issues="${id}:video_prompt"></div>`
            + `<details class="vs-plan-neg" data-plan-negative="${id}"${view.openNegative.has(scene.id) ? ' open' : ''}><summary>${negative}</summary><textarea ${attrs('negative_prompt')} rows="2" maxlength="${view.maxPrompt}" aria-label="${negative}"${dis}>${esc(s, scene.negative_prompt || '')}</textarea></details><div data-plan-issues="${id}:negative_prompt"></div>`
            + `<div class="vs-plan-grid"><label class="vs-field"><span>${label(s, 'plan.generateSeconds', 'Generate')}</span><select ${attrs('generate_seconds')}${dis}>${generate ? '' : option(s, '', '–', true, true)}${durations.map(value => option(s, value, secondsLabel(s, value), value === generate)).join('')}</select></label><label class="vs-field"><span>${label(s, 'plan.useSeconds', 'Use (s)')}</span><input type="number" min="0.5" step="0.5"${generate ? ` max="${generate}"` : ''} ${attrs('use_seconds')} value="${esc(s, Number(scene.use_seconds) || '')}"${dis}></label></div><div data-plan-issues="${id}:timing"></div>`
            + keyframeField(s, view, scene, index, dis) + `<div data-plan-issues="${id}:keyframe"></div>`
            + narrationField
            + `<div data-plan-issues="${id}:card"></div>${actions}</article>`;
    }
    // Start image: a seamless or bridge scene shows its source read-only ("continues from scene n"); otherwise a choice of
    // generated (with image generation), library (with project images) and text only, plus a stored value the choice
    // lacks so its issue stays readable.
    function keyframeField(s, view, scene, index, dis) {
        const id = esc(s, scene.id);
        const keyframe = scene.keyframe || {};
        const kind = (scene.transition_in && scene.transition_in.kind) || 'cut';
        const name = label(s, 'plan.keyframe', 'Start image');
        const promptField = `<label class="vs-field"><span>${label(s, 'plan.imagePrompt', 'Image prompt')}</span><textarea data-plan-field="keyframe.image_prompt" data-plan-scene="${id}" data-plan-key="${id}:keyframe.image_prompt" rows="2" maxlength="${view.maxPrompt}"${dis}>${esc(s, keyframe.image_prompt || '')}</textarea></label>`;
        if (index > 0 && (kind === 'continue' || kind === 'bridge')) {
            const source = kind === 'continue' ? 'chain' : 'bridge';
            return `<div class="vs-field"><span>${name}</span><p class="vs-plan-fixed" data-plan-source="${source}">${icon('link', 14)} ${esc(s, fill(tr(s, 'plan.source.' + source, SOURCES[source]), { number: index }))}</p></div>${source === 'bridge' ? promptField : ''}`;
        }
        const chosen = keyframe.source || 'none';
        const sources = ['generated', 'library', 'none'].filter(value => value === chosen || value === 'none' || (value === 'generated' && view.availability.image_generation) || (value === 'library' && view.imageAssets.length > 0));
        if (!sources.includes(chosen)) sources.push(chosen);
        let extra = '';
        if (chosen === 'generated') extra = promptField;
        else if (chosen === 'library') {
            const assets = !keyframe.asset_id || view.imageAssets.some(asset => asset.id === keyframe.asset_id) ? view.imageAssets : view.imageAssets.concat({ id: keyframe.asset_id, name: keyframe.asset_id });
            extra = `<label class="vs-field"><span>${label(s, 'plan.libraryImage', 'Image')}</span><select data-plan-field="keyframe.asset_id" data-plan-scene="${id}" data-plan-key="${id}:keyframe.asset_id"${dis}>${option(s, '', tr(s, 'plan.chooseImage', 'Choose an image'), !keyframe.asset_id)}${assets.map(asset => option(s, asset.id, asset.name, asset.id === keyframe.asset_id)).join('')}</select></label>`;
        }
        return `<label class="vs-field"><span>${name}</span><select data-plan-field="keyframe.source" data-plan-scene="${id}" data-plan-key="${id}:keyframe.source"${dis}>${sources.map(value => option(s, value, fill(tr(s, 'plan.source.' + value, own(SOURCES, value) ? SOURCES[value] : value), { number: index }), value === chosen)).join('')}</select></label>${extra}`;
    }
    function bar(s, view) {
        if (view.phase === 'empty') return view.readonly ? '' : `<footer class="vs-plan-bar" data-plan-bar><div class="vs-plan-facts"></div><button type="submit" class="vs-primary" form="${esc(s, formId(s))}" data-plan-submit>${icon('sparkle', 16)}<span>${label(s, 'plan.create', 'Create plan')}</span></button></footer>`;
        if (!view.plan || (view.phase !== 'review' && view.phase !== 'planning')) return '';
        const replan = view.locked ? '' : `<button type="button" class="vs-primary" data-plan-action="replan">${icon('sparkle', 16)}<span>${label(s, 'plan.replan', 'Plan again')}</span></button>`;
        return `<footer class="vs-plan-bar" data-plan-bar><div class="vs-plan-facts" data-plan-facts>${facts(s, view)}</div>${replan}</footer>`;
    }
    function facts(s, view) {
        const plan = view.plan;
        const blocking = (view.issues || []).filter(issue => issue.blocking).length;
        const total = fill(tr(s, 'plan.total', '{{total}} s of {{target}} s'), { total: num(view.total), target: num(plan.target_seconds) });
        const count = P.trCount(s, 'plan.scenes', (plan.scenes || []).length, '1 scene', '{{count}} scenes');
        const problems = blocking ? P.trCount(s, 'plan.blocking', blocking, '1 problem to fix', '{{count}} problems to fix') : tr(s, 'plan.noBlocking', 'No problems');
        const saved = view.locked ? '' : view.saving ? tr(s, 'saving', 'Saving…') : view.dirty ? '' : tr(s, 'saved', 'All changes saved');
        return `<span data-plan-total>${icon('clock', 14)} ${esc(s, total)}</span><span data-plan-count>${icon('film', 14)} ${esc(s, count)}</span><span data-plan-blocking class="${blocking ? 'is-blocking' : 'is-ok'}">${icon(blocking ? 'alert' : 'check', 14)} ${esc(s, problems)}</span>${saved ? `<span data-plan-save>${esc(s, saved)}</span>` : ''}`;
    }
    // Live parts only (bottom bar, duration chips): an edit never redraws the fields, so the caret stays.
    function refresh(s, view) {
        const host = s.q('[data-plan-board]');
        if (!host || !view.plan) return;
        const box = host.querySelector('[data-plan-facts]');
        if (box) box.innerHTML = facts(s, view);
        (view.plan.scenes || []).forEach(scene => {
            const chip = host.querySelector(`[data-plan-duration="${CSS.escape(String(scene.id))}"]`);
            if (chip) chip.textContent = durationText(s, scene);
        });
    }
    function renderIssues(s, view) {
        const host = s.q('[data-plan-board]');
        if (!host) return;
        host.querySelectorAll('[data-plan-issues]').forEach(slot => slot.replaceChildren());
        if (view.plan) {
            const ids = new Set((view.plan.scenes || []).map(scene => scene.id));
            (view.issues || []).forEach(issue => {
                const code = String(issue.code || '');
                const field = String(issue.field || '');
                // prompt_too_long also names the image prompt (scenes[i].keyframe.image_prompt, 0-based) and the music
                // prompt (audio.music.prompt); the validator's field tells them apart.
                const slot = code === 'prompt_too_long' && field.endsWith('.keyframe.image_prompt') ? 'keyframe' : code === 'prompt_too_long' && field === 'audio.music.prompt' ? 'music' : own(CODE_SLOTS, code) ? CODE_SLOTS[code] : '';
                const scene = issue.scene_id && ids.has(issue.scene_id) ? issue.scene_id : '';
                const keys = scene ? [scene + ':' + (slot || 'card'), scene + ':card'] : ['plan:' + (PLAN_SLOTS.has(slot) ? slot : 'general'), 'plan:general'];
                const target = keys.map(key => host.querySelector(`[data-plan-issues="${CSS.escape(key)}"]`)).find(Boolean);
                if (!target) return;
                const text = tr(s, 'plan.issue.' + code, issue.message || code);
                target.insertAdjacentHTML('beforeend', `<p class="vs-plan-issue ${issue.blocking ? 'is-blocking' : 'is-warning'}" data-plan-issue-code="${esc(s, code)}">${icon(issue.blocking ? 'alert' : 'info', 13)}<span>${esc(s, text)}</span></p>`);
            });
        }
        refresh(s, view);
    }
    // "Plan again": the dialog is the confirmation; it starts from the current idea and target.
    function openStart(s, view, submit) {
        const plan = view.plan || {};
        const id = esc(s, 'vs-plan-start-' + s.id);
        const host = P.openModal(s, `<form class="vs-modal vs-plan-modal" data-plan-start-form role="dialog" aria-modal="true" aria-labelledby="${id}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${label(s, 'close', 'Close')}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('sparkle', 22)}</span><h2 id="${id}">${label(s, 'plan.replanTitle', 'Plan again?')}</h2><p>${label(s, 'plan.replanCopy', 'The agent plans a new storyboard. It replaces the current plan and your edits.')}</p>${ideaFields(s, plan.idea || '', Math.round(Number(plan.target_seconds)) || 30)}<div class="vs-modal-actions"><button type="button" data-modal-close>${label(s, 'cancel', 'Cancel')}</button><button type="submit" class="vs-primary">${icon('sparkle', 16)}<span>${label(s, 'plan.replan', 'Plan again')}</span></button></div></form>`);
        const form = host.querySelector('[data-plan-start-form]');
        form.addEventListener('submit', event => {
            event.preventDefault();
            const idea = form.querySelector('[data-plan-idea]').value.trim();
            const target = Number(form.querySelector('[data-plan-target]').value) || 30;
            if (!idea) return;
            host.replaceChildren();
            s.app.focus({ preventScroll: true });
            submit({ idea, target_seconds: target });
        });
    }
    function openRevise(s, view, sceneId, submit) {
        const index = ((view.plan && view.plan.scenes) || []).findIndex(scene => scene.id === sceneId);
        if (index < 0) return;
        const id = esc(s, 'vs-plan-revise-' + s.id);
        const host = P.openModal(s, `<form class="vs-modal vs-plan-modal" data-plan-revise-form role="dialog" aria-modal="true" aria-labelledby="${id}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${label(s, 'close', 'Close')}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('sparkle', 22)}</span><h2 id="${id}">${esc(s, fill(tr(s, 'plan.reviseTitle', 'Revise scene {{number}}'), { number: index + 1 }))}</h2><p>${label(s, 'plan.reviseCopy', 'Tell the agent what to change. It changes only this scene and its transitions.')}</p><label class="vs-field vs-field-wide"><span>${label(s, 'plan.reviseNote', 'What should change?')}</span><textarea data-plan-note rows="4" maxlength="1000" required></textarea></label><div class="vs-modal-actions"><button type="button" data-modal-close>${label(s, 'cancel', 'Cancel')}</button><button type="submit" class="vs-primary">${icon('sparkle', 16)}<span>${label(s, 'plan.revise', 'Revise scene')}</span></button></div></form>`);
        const form = host.querySelector('[data-plan-revise-form]');
        form.addEventListener('submit', event => {
            event.preventDefault();
            const note = form.querySelector('[data-plan-note]').value.trim();
            if (!note) return;
            host.replaceChildren();
            s.app.focus({ preventScroll: true });
            submit({ scene_id: sceneId, note });
        });
    }

    window.VideoStudioPlanBoard = { shellMarkup, render, refresh, renderIssues, openStart, openRevise };
})();
```

- [ ] **Step 4: Create `ui/js/desktop/apps/video-studio-plan.js`:**

```js
(function () {
    'use strict';

    // Plan mode (agent-planned clips, slice 1): the Edit | Plan switch in the toolbar, the plan of the open project
    // (GET and PUT .../projects/{id}/plan with ETags), the planning agent (POST .../plan/agent and .../plan/agent/stop),
    // polling every 2 s while the agent plans, and the storyboard edits (one PUT with If-Match 800 ms after the last
    // edit). video-studio-plan-board.js draws the board from view(s). State: s.planMode (per window session, false =
    // edit) and s.planState (the open project's plan; a project change starts a new one, the old one saves first).
    const P = window.VideoStudioParts;
    const B = () => window.VideoStudioPlanBoard;
    const POLL_MS = 2000;
    const SAVE_MS = 800;
    const MAX_SCENES = 12;
    const MAX_CHARACTERS = 6;
    const AGENT_ERRORS = {
        plan_daily_limit: ['plan.errDailyLimit', "Today's planning limit is reached. Try again tomorrow."],
        plan_agent_running: ['plan.agentBusy', 'The agent is still working on this plan.'],
        plan_agent_unavailable: ['plan.errAgentUnavailable', 'No language model is available for planning.'],
        plan_unsupported_provider: ['plan.issue.plan_unsupported_provider', 'The current video provider does not support plan mode.'],
        // Part B start refusals the user can act on; the rest fall back to plan.errStart.
        plan_exists: ['plan.changedElsewhere', 'The plan was changed elsewhere. The latest version is shown.'],
        plan_not_found: ['plan.changedElsewhere', 'The plan was changed elsewhere. The latest version is shown.'],
        scene_not_found: ['plan.changedElsewhere', 'The plan was changed elsewhere. The latest version is shown.'],
        plan_invalid: ['plan.damaged', 'The stored plan is damaged. Create a new plan to replace it.'],
        project_size_limit: ['projectStorageFull', 'This project has reached its storage limit. Remove media or use another project.']
    };
    // After these refusals the stored plan differs from the board (another window planned, changed or damaged it): reload.
    const RELOAD_AFTER_START = new Set(['plan_agent_running', 'plan_exists', 'plan_not_found', 'scene_not_found', 'plan_invalid']);
    const own = (map, key) => Object.prototype.hasOwnProperty.call(map, key);
    const planURL = ps => P.API + '/projects/' + encodeURIComponent(ps.projectId) + '/plan';
    const current = (s, ps) => !s.disposed && s.planState === ps;

    function uiLanguage() {
        const tag = String(P.uiLocale() || 'en').toLowerCase().split(/[-_]/)[0];
        return /^[a-z]{2,3}$/.test(tag) ? tag : 'en';
    }
    function canPlan(s) {
        return !!(s.app && s.project && s.projectId && !s.featureDisabled && s.status && s.status.plan && s.status.plan.enabled);
    }
    function switchMarkup(s) {
        const text = (key, fallback) => P.esc(s, P.tr(s, key, fallback));
        return `<div class="vs-segmented vs-mode-switch" data-plan-switch role="group" aria-label="${text('plan.modeLabel', 'Mode')}" hidden><button type="button" data-plan-mode="edit" aria-pressed="true">${text('plan.modeEdit', 'Edit')}</button><button type="button" data-plan-mode="plan" aria-pressed="false">${text('plan.modePlan', 'Plan')}</button></div>`;
    }
    function install(s) {
        s.planMode = false;
        s.planState = null;
        const signal = s.abort.signal;
        s.host.addEventListener('click', event => onClick(s, event), { signal });
        s.host.addEventListener('input', event => onInput(s, event), { signal });
        s.host.addEventListener('change', event => onChange(s, event), { signal });
        s.host.addEventListener('submit', event => onSubmit(s, event), { signal });
    }
    function freshState(s) {
        return { projectId: s.projectId, data: null, etag: '', missing: false, damaged: false, error: false, loading: false, revision: 0, savedRevision: 0, saving: false, again: false, saveTimer: 0, savePromise: null, pollTimer: 0, settingsOpen: true, openNegative: new Set() };
    }
    function state(s) {
        if (!s.planState || s.planState.projectId !== s.projectId) {
            retire(s);
            s.planState = freshState(s);
        }
        return s.planState;
    }
    // Drops the previous project's plan; a pending edit is still saved (the request names its own project).
    function retire(s) {
        const ps = s.planState;
        if (!ps) return;
        clearTimeout(ps.pollTimer);
        ps.pollTimer = 0;
        s.planState = null;
        if (ps.saveTimer || ps.revision !== ps.savedRevision) save(s, ps);
    }
    // Called from renderToolbar: the switch shows with an open project while status.plan.enabled.
    function sync(s) {
        if (!s.app || s.disposed) return;
        const ok = canPlan(s);
        const toggle = s.q('[data-plan-switch]');
        if (toggle) toggle.hidden = !ok;
        if (!ok) { if (s.planMode) setMode(s, false); return; }
        if (s.planMode && (!s.planState || s.planState.projectId !== s.projectId)) { state(s); render(s); load(s); }
    }
    function setMode(s, on) {
        const next = !!on && canPlan(s);
        if (next !== !!s.planMode) {
            s.planMode = next;
            s.app.classList.toggle('vs-plan-mode', next);
            const board = s.q('[data-plan-board]');
            if (board) board.hidden = !next;
            if (next) {
                if (s.preview && s.preview.isPlaying()) s.preview.pause();
                if (s.sourcePreview && typeof s.sourcePreview.pause === 'function') s.sourcePreview.pause();
                if (s.app.classList.contains('vs-show-inspector') && P.setInspectorOpen) P.setInspectorOpen(s, false, false);
                state(s);
                render(s);
                load(s);
            } else {
                const ps = s.planState;
                if (ps) { clearTimeout(ps.pollTimer); ps.pollTimer = 0; flush(s); }
                // The stage and the timeline were hidden: draw them at their real size again.
                if (s.project) P.renderUI(s);
            }
        }
        const toggle = s.q('[data-plan-switch]');
        if (toggle) toggle.querySelectorAll('[data-plan-mode]').forEach(button => button.setAttribute('aria-pressed', String((button.dataset.planMode === 'plan') === next)));
        return next;
    }
    function render(s) { if (s.planMode && B()) B().render(s, view(s)); }
    // The agent works on the plan: the board is read-only and polls.
    function planning(ps) {
        const data = ps && ps.data;
        return !!data && !!data.plan && (data.plan.status === 'planning' || !!(data.agent && data.agent.state === 'running'));
    }
    // Shape guard for drawing only; the server normalizes the stored plan.
    function normalize(plan) {
        plan.scenes = Array.isArray(plan.scenes) ? plan.scenes : [];
        plan.characters = Array.isArray(plan.characters) ? plan.characters : [];
        plan.audio = plan.audio && typeof plan.audio === 'object' ? plan.audio : {};
        plan.audio.music = plan.audio.music && typeof plan.audio.music === 'object' ? plan.audio.music : { instrumental: true };
        plan.scenes.forEach(scene => {
            if (!scene.transition_in || typeof scene.transition_in !== 'object') scene.transition_in = { kind: 'cut' };
            if (!scene.keyframe || typeof scene.keyframe !== 'object') scene.keyframe = { source: 'none' };
            if (!scene.narration || typeof scene.narration !== 'object') scene.narration = {};
        });
        return plan;
    }
    function adopt(ps, body, etag) {
        if (!body || !body.plan || typeof body.plan !== 'object') { ps.error = !ps.data; return; }
        ps.data = { plan: normalize(body.plan), issues: Array.isArray(body.issues) ? body.issues : [], profile: body.profile || null, availability: body.availability || null, agent: body.agent || null };
        ps.etag = etag || body.etag || '';
        ps.missing = false;
        ps.damaged = false;
        ps.error = false;
        clearTimeout(ps.saveTimer);
        ps.saveTimer = 0;
        ps.savedRevision = ps.revision;
    }
    async function load(s, conditional) {
        const ps = state(s);
        if (ps.loading) return;
        ps.loading = true;
        let changed = true;
        try {
            const headers = conditional && ps.etag ? { 'If-None-Match': ps.etag } : {};
            const response = await P.request(planURL(ps), { headers, signal: s.abort.signal });
            if (!current(s, ps)) return;
            adopt(ps, response.body, response.etag);
        } catch (error) {
            if (!current(s, ps)) return;
            if (error.status === 304) changed = false;
            else if (error.status === 404) { ps.data = null; ps.etag = ''; ps.missing = true; ps.damaged = false; ps.error = false; }
            // A damaged plan.json (Part B: 409 plan_invalid): the empty state with a notice; its create replaces the file.
            else if (error.status === 409 && error.message === 'plan_invalid') { ps.data = null; ps.etag = ''; ps.missing = true; ps.damaged = true; ps.error = false; }
            else ps.error = !ps.data;
        } finally {
            ps.loading = false;
        }
        if (changed) render(s);
        schedulePoll(s);
    }
    function schedulePoll(s) {
        const ps = s.planState;
        if (!ps) return;
        clearTimeout(ps.pollTimer);
        ps.pollTimer = 0;
        if (s.disposed || !s.planMode || !planning(ps)) return;
        ps.pollTimer = window.setTimeout(() => { ps.pollTimer = 0; if (current(s, ps)) load(s, true); }, POLL_MS);
    }
    function queueSave(s, ps) {
        clearTimeout(ps.saveTimer);
        ps.saveTimer = window.setTimeout(() => { ps.saveTimer = 0; save(s, ps); }, SAVE_MS);
    }
    // One PUT at a time; an edit made while it runs saves after it. Never rejects.
    function save(s, ps) {
        clearTimeout(ps.saveTimer);
        ps.saveTimer = 0;
        if (ps.savePromise) { ps.again = true; return ps.savePromise; }
        if (!ps.data || s.readonly || planning(ps)) { ps.savedRevision = ps.revision; return Promise.resolve(true); }
        if (ps.revision === ps.savedRevision) return Promise.resolve(true);
        const revision = ps.revision;
        const body = JSON.stringify(ps.data.plan);
        ps.saving = true;
        if (current(s, ps) && s.planMode && B()) B().refresh(s, view(s));
        ps.savePromise = (async () => {
            let ok = false, reload = false;
            try {
                const response = await P.request(planURL(ps), { method: 'PUT', headers: { 'Content-Type': 'application/json', 'If-Match': ps.etag }, body });
                const fresh = response.body || {};
                ps.etag = response.etag || ps.etag;
                ps.savedRevision = revision;
                if (Array.isArray(fresh.issues)) ps.data.issues = fresh.issues;
                if (fresh.profile) ps.data.profile = fresh.profile;
                if (fresh.availability) ps.data.availability = fresh.availability;
                if ('agent' in fresh) ps.data.agent = fresh.agent;
                // Nothing changed while the request ran: the server's copy becomes the model; the fields stay as typed.
                if (fresh.plan && ps.revision === revision) ps.data.plan = normalize(fresh.plan);
                ok = true;
            } catch (error) {
                if (!current(s, ps)) return false;
                const busy = error.status === 409 && error.message === 'plan_agent_running';
                // plan_not_found / plan_invalid: removed or damaged elsewhere; the reload shows the empty or damaged state.
                if (busy || error.status === 412 || error.status === 428 || error.message === 'file_conflict' || error.message === 'plan_not_found' || error.message === 'plan_invalid') {
                    P.showNotice(s, busy ? 'plan.agentBusy' : 'plan.changedElsewhere', busy ? 'The agent is still working on this plan.' : 'The plan was changed elsewhere. The latest version is shown.', true);
                    ps.savedRevision = ps.revision;
                    reload = true;
                } else if (error.message === 'project_size_limit') {
                    P.showNotice(s, 'projectStorageFull', 'This project has reached its storage limit. Remove media or use another project.', true);
                } else {
                    P.showNotice(s, error.status === 403 ? 'readonly' : 'saveFailed', error.status === 403 ? 'Read-only mode prevents editing.' : 'Save failed', true);
                }
            } finally {
                ps.saving = false;
                ps.savePromise = null;
                const again = ps.again;
                ps.again = false;
                if (current(s, ps)) {
                    if (reload) load(s);
                    else if (ok && (again || ps.revision !== ps.savedRevision)) queueSave(s, ps);
                    if (s.planMode && B()) B().renderIssues(s, view(s));
                }
            }
            return ok;
        })();
        return ps.savePromise;
    }
    // Saves a pending edit now (leaving plan mode, closing the window, before the agent starts).
    async function flush(s) {
        const ps = s.planState;
        for (let round = 0; ps && round < 5; round++) {
            if (!ps.savePromise && !ps.saveTimer && ps.revision === ps.savedRevision) return true;
            if (!(await save(s, ps))) return false;
        }
        return !ps || ps.revision === ps.savedRevision;
    }
    async function startAgent(s, body) {
        const ps = state(s);
        if (s.readonly || planning(ps)) return false;
        await flush(s);
        if (!current(s, ps)) return false;
        let response;
        try {
            response = await P.request(planURL(ps) + '/agent', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal: s.abort.signal });
        } catch (error) {
            if (current(s, ps)) {
                const [key, fallback] = own(AGENT_ERRORS, error.message) ? AGENT_ERRORS[error.message] : error.status === 403 ? ['readonly', 'Read-only mode prevents editing.'] : ['plan.errStart', 'Planning could not be started.'];
                P.showNotice(s, key, fallback, true);
                if (RELOAD_AFTER_START.has(error.message)) load(s);
            }
            return false;
        }
        if (!current(s, ps)) return false;
        const agent = (response.body && response.body.agent) || { state: 'running', mode: body.mode, phase: body.mode === 'revise' ? 'revise' : 'outline' };
        if (body.mode === 'create') {
            // The server wrote a new plan.json; until the first read the board shows the idea and the status line.
            ps.data = { plan: normalize({ status: 'planning', idea: body.idea, target_seconds: body.target_seconds, profile: {} }), issues: [], profile: null, availability: null, agent };
            ps.etag = '';
            ps.missing = false;
            ps.damaged = false;
        } else if (ps.data) ps.data.agent = agent;
        render(s);
        await load(s);
        return true;
    }
    async function stopAgent(s) {
        const ps = state(s);
        if (s.readonly) return false;
        try { await P.request(planURL(ps) + '/agent/stop', { method: 'POST', signal: s.abort.signal }); } catch (_) { /* the next read shows the real state */ }
        if (current(s, ps)) await load(s);
        return true;
    }
    // Σ use_seconds − the blend overlaps (validator rule, brief §4).
    function totalSeconds(plan) {
        let total = 0;
        ((plan && plan.scenes) || []).forEach((scene, index) => {
            total += Number(scene.use_seconds) || 0;
            const transition = scene.transition_in;
            if (index > 0 && transition && transition.kind === 'blend') total -= Number(transition.seconds) || 0;
        });
        return Math.round(total * 100) / 100;
    }
    function blendTypes(s) {
        const fx = window.VideoStudioEffects;
        if (!fx || typeof fx.transitions !== 'function') return [];
        return fx.transitions().filter(spec => spec.available).map(spec => ({ id: spec.id, label: P.tr(s, spec.label_key, spec.id) }));
    }
    function imageAssets(s) {
        const media = window.VideoStudioMedia;
        return ((s.project && s.project.assets) || []).filter(asset => asset.kind === 'image' && !(media && media.isArtwork(asset))).map(asset => ({ id: asset.id, name: P.assetDisplayName(s, asset) || asset.id }));
    }
    function view(s) {
        const ps = s.planState || {};
        const data = ps.data || null;
        const status = s.status || {};
        const generation = status.generation || {};
        const availability = Object.assign({ enabled: false, supported: true, image_generation: false, tts: false, music: false }, status.plan || {}, (data && data.availability) || {});
        const profile = (data && data.profile) || generation.profile || {};
        const plan = data ? data.plan : null;
        let phase = 'loading';
        if (plan) phase = planning(ps) ? 'planning' : 'review';
        else if (ps.missing) phase = availability.supported === false ? 'unsupported' : 'empty';
        else if (ps.error) phase = 'error';
        const blends = blendTypes(s);
        const durations = ((profile.durations_seconds && profile.durations_seconds.length ? profile.durations_seconds : generation.durations_seconds) || []).map(Number).filter(value => value > 0);
        // A bridge needs first and last frame and a generated start image (the board edits its image prompt; validator and
        // Part C's agent rule).
        const kinds = ['cut'].concat(profile.first_frame ? ['continue'] : [], profile.first_frame && profile.last_frame && availability.image_generation ? ['bridge'] : [], blends.length ? ['blend'] : []);
        const ref = plan && plan.profile && plan.profile.provider ? plan.profile : profile;
        return {
            phase, plan, availability, profile, durations, kinds, blendTypes: blends,
            issues: (data && data.issues) || [], agent: (data && data.agent) || null,
            readonly: !!s.readonly, locked: !!s.readonly || phase !== 'review',
            saving: !!ps.saving, dirty: ps.revision !== ps.savedRevision,
            total: totalSeconds(plan), maxPrompt: Number(profile.max_prompt_chars) || 2000, imageAssets: imageAssets(s),
            profileLine: [ref.provider, ref.model, ref.resolution].filter(Boolean).join(' · '),
            settingsOpen: ps.settingsOpen !== false, openNegative: ps.openNegative || new Set(), damaged: !!ps.damaged
        };
    }
    function newScene(s, plan) {
        const shown = view(s);
        const used = new Set(plan.scenes.map(scene => scene.id));
        let n = plan.scenes.length + 1;
        while (used.has('s' + n)) n++;
        const seconds = shown.durations[0] || 6;
        return { id: 's' + n, title: '', video_prompt: '', generate_seconds: seconds, use_seconds: seconds, transition_in: { kind: 'cut' }, keyframe: { source: shown.availability.image_generation ? 'generated' : 'none' }, narration: {}, state: 'pending' };
    }
    // The first scene has no incoming transition: a cut, and a seamless or bridge start image has nothing to join.
    function fixFirst(plan) {
        const first = plan.scenes[0];
        if (!first) return;
        first.transition_in = { kind: 'cut' };
        if (first.keyframe && (first.keyframe.source === 'chain' || first.keyframe.source === 'bridge')) first.keyframe = Object.assign({}, first.keyframe, { source: 'none' });
    }
    // A seamless scene starts from the previous clip (source chain), a bridge from the shared frame (source bridge).
    function setKind(scene, kind, blends) {
        const before = scene.transition_in || {};
        const next = { kind };
        if (kind === 'blend') { next.type = before.type || (blends[0] && blends[0].id) || 'dissolve'; next.seconds = Number(before.seconds) || 0.8; }
        scene.transition_in = next;
        const source = scene.keyframe && scene.keyframe.source;
        if (kind === 'continue') scene.keyframe = Object.assign({}, scene.keyframe, { source: 'chain' });
        else if (kind === 'bridge') scene.keyframe = Object.assign({}, scene.keyframe, { source: 'bridge' });
        else if (source === 'chain' || source === 'bridge') scene.keyframe = Object.assign({}, scene.keyframe, { source: 'none' });
    }
    // Writes one field into the model: 'save' (no redraw), 'render' (the card changes shape) or false.
    function write(s, el) {
        const ps = s.planState, plan = ps && ps.data && ps.data.plan;
        if (!plan || s.readonly || planning(ps)) return false;
        const field = el.dataset.planField, value = el.value;
        const number = () => { const n = Number(value); return value !== '' && Number.isFinite(n) ? n : null; };
        if (el.dataset.planScene) {
            const scene = plan.scenes.find(item => item.id === el.dataset.planScene);
            if (!scene) return false;
            if (field === 'title' || field === 'video_prompt' || field === 'negative_prompt') { scene[field] = value; return 'save'; }
            if (field === 'generate_seconds') {
                const n = number();
                if (n == null) return false;
                scene.generate_seconds = n;
                if (!(Number(scene.use_seconds) > 0) || Number(scene.use_seconds) > n) scene.use_seconds = n;
                return 'render';
            }
            if (field === 'use_seconds') { const n = number(); if (n == null) return false; scene.use_seconds = n; return 'save'; }
            if (field === 'keyframe.source') { scene.keyframe = Object.assign({}, scene.keyframe, { source: value }); return 'render'; }
            if (field === 'keyframe.image_prompt') { scene.keyframe = Object.assign({}, scene.keyframe, { image_prompt: value }); return 'save'; }
            if (field === 'keyframe.asset_id') { scene.keyframe = Object.assign({}, scene.keyframe, { asset_id: value }); return 'save'; }
            if (field === 'narration.text') { scene.narration = Object.assign({}, scene.narration, { text: value }); return 'save'; }
            if (field === 'transition.kind') { setKind(scene, value, blendTypes(s)); return 'render'; }
            if (field === 'transition.type') { scene.transition_in = Object.assign({}, scene.transition_in, { type: value }); return 'save'; }
            if (field === 'transition.seconds') { const n = number(); if (n == null) return false; scene.transition_in = Object.assign({}, scene.transition_in, { seconds: n }); return 'save'; }
            return false;
        }
        const index = Number(el.dataset.planIndex);
        if (field === 'style') { plan.style = value; return 'save'; }
        if (field === 'music.prompt') { plan.audio.music = Object.assign({}, plan.audio.music, { prompt: value }); return 'save'; }
        if (field === 'clip_audio') { plan.audio.clip_audio = value; return 'save'; }
        if ((field === 'character.name' || field === 'character.description') && plan.characters[index]) { plan.characters[index][field.slice(10)] = value; return 'save'; }
        return false;
    }
    // Structural edits from card and section buttons; true when the plan changed.
    function edit(s, action, sceneId, index) {
        const ps = s.planState, plan = ps && ps.data && ps.data.plan;
        if (!plan) return false;
        const scenes = plan.scenes;
        if (action === 'add-character') { if (plan.characters.length >= MAX_CHARACTERS) return false; plan.characters.push({ name: '', description: '' }); return true; }
        if (action === 'remove-character') { if (!(index >= 0 && index < plan.characters.length)) return false; plan.characters.splice(index, 1); return true; }
        if (action === 'add-scene') { if (scenes.length >= MAX_SCENES) return false; scenes.push(newScene(s, plan)); fixFirst(plan); return true; }
        const at = scenes.findIndex(scene => scene.id === sceneId);
        if (at < 0) return false;
        if (action === 'text-only') { scenes[at].keyframe = Object.assign({}, scenes[at].keyframe, { source: 'none' }); return true; }
        if (action === 'remove') { scenes.splice(at, 1); fixFirst(plan); return true; }
        if (action === 'up' || action === 'down') {
            const to = at + (action === 'up' ? -1 : 1);
            if (to < 0 || to >= scenes.length) return false;
            [scenes[at], scenes[to]] = [scenes[to], scenes[at]];
            fixFirst(plan);
            return true;
        }
        if (action === 'add-after') { if (scenes.length >= MAX_SCENES) return false; scenes.splice(at + 1, 0, newScene(s, plan)); return true; }
        return false;
    }
    function edited(s) {
        const ps = s.planState;
        if (!ps) return;
        ps.revision++;
        queueSave(s, ps);
        if (B()) B().refresh(s, view(s));
    }
    function boardField(s, target) {
        if (!target || !target.dataset || !target.dataset.planField) return null;
        const board = s.q('[data-plan-board]');
        return board && board.contains(target) ? target : null;
    }
    function onClick(s, event) {
        const target = event.target;
        if (!target || typeof target.closest !== 'function') return;
        const modeButton = target.closest('[data-plan-switch] [data-plan-mode]');
        if (modeButton) { setMode(s, modeButton.dataset.planMode === 'plan'); return; }
        const board = s.q('[data-plan-board]');
        const ps = s.planState;
        if (!board || !board.contains(target) || !ps) return;
        const summary = target.closest('summary');
        if (summary) {
            // The open state survives redraws; details.open is still the old state during the click.
            const details = summary.parentElement;
            if (details && details.hasAttribute('data-plan-settings')) ps.settingsOpen = !details.open;
            else if (details && details.dataset.planNegative) { if (details.open) ps.openNegative.delete(details.dataset.planNegative); else ps.openNegative.add(details.dataset.planNegative); }
            return;
        }
        const button = target.closest('[data-plan-action]');
        if (!button || button.disabled) return;
        const action = button.dataset.planAction;
        if (action === 'retry') { load(s); return; }
        if (s.readonly) return;
        if (action === 'stop') { stopAgent(s); return; }
        const shown = view(s);
        if (shown.phase !== 'review' || !B()) return;
        if (action === 'replan') { B().openStart(s, shown, values => startAgent(s, Object.assign({ mode: 'create', language: uiLanguage(), replace: true }, values))); return; }
        if (action === 'revise') { B().openRevise(s, shown, button.dataset.planScene || '', values => startAgent(s, Object.assign({ mode: 'revise' }, values))); return; }
        if (edit(s, action, button.dataset.planScene || '', Number(button.dataset.planIndex))) { render(s); edited(s); }
    }
    // Text and number fields write on input without a redraw (the caret stays); selects write on change.
    function onInput(s, event) {
        const el = boardField(s, event.target);
        if (!el || el.tagName === 'SELECT') return;
        if (write(s, el)) edited(s);
    }
    function onChange(s, event) {
        const el = boardField(s, event.target);
        if (!el || el.tagName !== 'SELECT') return;
        const result = write(s, el);
        if (!result) return;
        if (result === 'render') render(s);
        edited(s);
    }
    function onSubmit(s, event) {
        const form = event.target && typeof event.target.closest === 'function' ? event.target.closest('[data-plan-create-form]') : null;
        if (!form) return;
        event.preventDefault();
        const idea = form.querySelector('[data-plan-idea]').value.trim();
        const target = Number(form.querySelector('[data-plan-target]').value) || 30;
        if (!idea || s.readonly) return;
        // Over a damaged plan.json the create must replace it (Part B answers plan_exists otherwise).
        startAgent(s, Object.assign({ mode: 'create', idea, target_seconds: target, language: uiLanguage() }, s.planState && s.planState.damaged ? { replace: true } : {}));
    }
    function dispose(s) {
        const ps = s.planState;
        if (!ps) return;
        clearTimeout(ps.pollTimer);
        clearTimeout(ps.saveTimer);
        ps.pollTimer = 0;
        ps.saveTimer = 0;
    }

    window.VideoStudioPlan = { install, sync, setMode, switchMarkup, view, load, flush, startAgent, stopAgent, dispose, totalSeconds };
})();
```

- [ ] **Step 5: Edit `ui/js/desktop/apps/video-studio.js`** (seven insertions, all guarded).

(a) `dispose`. **Old:**
```js
        clearInterval(s.jobsTimer);
```
**New:**
```js
        clearInterval(s.jobsTimer);
        if (window.VideoStudioPlan) window.VideoStudioPlan.dispose(s);
```

(b) `layoutMarkup`, header (Old is the start of the `vs-tb-edit` line). **Old:**
```
 <div class="vs-tb-group vs-tb-edit"><button type="button" class="vs-icon-button" data-action="undo"
```
**New:**
```
 ${window.VideoStudioPlan ? window.VideoStudioPlan.switchMarkup(s) : ''}
 <div class="vs-tb-group vs-tb-edit"><button type="button" class="vs-icon-button" data-action="undo"
```

(c) `layoutMarkup`, centre column. **Old:**
```
  </div>
 </section>
 </main>
```
**New:**
```
  </div>
 </section>
 ${window.VideoStudioPlanBoard ? window.VideoStudioPlanBoard.shellMarkup(s) : ''}
 </main>
```

(d) `appRender`, install. **Old:**
```js
        P.wireSource(s);
        if (typeof MutationObserver === 'function') {
```
**New:**
```js
        P.wireSource(s);
        // Plan mode (video-studio-plan.js): the Edit | Plan switch and the storyboard listeners.
        if (window.VideoStudioPlan) window.VideoStudioPlan.install(s);
        if (typeof MutationObserver === 'function') {
```

(e) `appRender`, window close guard. **Old:**
```js
        if (typeof ctx.setWindowBeforeClose === 'function') ctx.setWindowBeforeClose(id, async () => {
            if (s.readonly) return true;
```
**New:**
```js
        if (typeof ctx.setWindowBeforeClose === 'function') ctx.setWindowBeforeClose(id, async () => {
            if (s.readonly) return true;
            if (window.VideoStudioPlan) await window.VideoStudioPlan.flush(s);
```

(f) `renderToolbar`. **Old:**
```js
        if (P.syncRangeRender) P.syncRangeRender(s);
        renderJobsButton(s);
    }
```
**New:**
```js
        if (P.syncRangeRender) P.syncRangeRender(s);
        renderJobsButton(s);
        if (window.VideoStudioPlan) window.VideoStudioPlan.sync(s);
    }
```

(g) `handleKeys` (the first of the two identical modal checks; the comment line makes it unique). **Old:**
```js
        if (s.q('[data-modal-host] .vs-modal, [data-conflict-host] .vs-modal')) return;
        // The source position slider is an input that takes no text: in source mode it passes everything but its own
```
**New:**
```js
        if (s.q('[data-modal-host] .vs-modal, [data-conflict-host] .vs-modal')) return;
        // Plan mode shows the storyboard instead of the stage and the timeline: no editor shortcut applies there.
        if (s.planMode) return;
        // The source position slider is an input that takes no text: in source mode it passes everything but its own
```

- [ ] **Step 6: Append to the end of `ui/css/desktop-app-video-studio.css`** (after the last rule, the colour section's `::-moz-range-thumb` rule):

```css

/* === Plan mode (storyboard) === */
/* video-studio-plan.js and video-studio-plan-board.js. The Edit | Plan switch sits in the toolbar between the project
   and the edit groups and shows only with an open project while status.plan.enabled. .vs-plan-mode on .vs-app hides
   the monitor bar, the stage, the transport, the source monitor, the scopes, the timeline, the inspector and the edit
   group (undo, redo, format act on the timeline); the storyboard fills the centre column and the library stays. At
   600 px or less the switch takes its own toolbar row (the 390 px toolbar has no room left). */
.vs-app .vs-mode-switch { flex: 0 0 auto; margin-top: 0; }
.vs-app .vs-mode-switch button { min-height: 28px; white-space: nowrap; }
.vs-plan-mode .vs-center > :not(.vs-plan), .vs-plan-mode > .vs-resize, .vs-plan-mode > .vs-timeline-panel,
.vs-plan-mode .vs-inspector, .vs-plan-mode .vs-inspector-scrim, .vs-plan-mode .vs-inspector-toggle, .vs-plan-mode .vs-tb-edit { display: none !important; }
.vs-plan-mode .vs-workspace { grid-template-columns: var(--vs-lib-w) minmax(0, 1fr); }
.vs-plan-mode .vs-center { background: var(--vs-canvas); }
.vs-plan-mode .vs-toast { bottom: 64px; }
.vs-plan { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; }
.vs-plan-scroll { flex: 1 1 auto; min-height: 0; overflow: auto; padding: 16px; }
.vs-plan-inner { display: grid; gap: 12px; width: 100%; max-width: 860px; min-width: 0; margin: 0 auto; }
.vs-plan-state { display: flex; flex-wrap: wrap; align-items: center; justify-content: center; gap: 10px; padding: 32px 16px; color: var(--vs-muted); }
.vs-plan-empty { display: grid; gap: 4px; width: 100%; max-width: 560px; min-width: 0; margin: 16px auto; padding: 20px; border: 1px solid var(--vs-border); border-radius: var(--vs-card-radius); background: var(--vs-card-flat); }
.vs-plan-empty h2 { margin: 2px 0; font-family: var(--vs-head-font); font-size: 19px; color: var(--vs-head-color); }
.vs-plan-empty > p { margin: 0; color: var(--vs-muted); line-height: 1.5; }
.vs-plan-head, .vs-plan-settings, .vs-plan-card { min-width: 0; padding: 12px 14px; border: 1px solid var(--vs-border); border-radius: var(--vs-card-radius); background: var(--vs-card-flat); }
.vs-plan-head { display: grid; gap: 8px; }
.vs-plan-idea > p:last-child { margin: 2px 0 0; overflow-wrap: anywhere; white-space: pre-wrap; }
.vs-plan-agent { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; min-width: 0; font-size: 12.5px; color: var(--vs-muted); }
.vs-plan-agent strong { color: var(--vs-text); }
.vs-plan-agent small { min-width: 0; overflow-wrap: anywhere; }
.vs-plan-agent .vs-plan-stop { margin-left: auto; }
.vs-plan-ended { flex: 1 1 100%; color: color-mix(in srgb, var(--vs-warn) 70%, var(--vs-text)); }
.vs-plan-settings > summary { display: flex; align-items: center; gap: 7px; cursor: pointer; font-weight: 650; }
.vs-plan-label { margin: 10px 0 0; font-size: 12px; color: var(--vs-muted); }
.vs-plan-character { display: grid; grid-template-columns: minmax(0, 180px) minmax(0, 1fr) auto; gap: 6px; align-items: start; margin-top: 6px; }
.vs-plan-character textarea { min-height: 38px; resize: vertical; }
.vs-plan-characters > .vs-chip-button, .vs-plan-none .vs-chip-button { margin-top: 8px; }
.vs-plan-scenes { display: grid; margin: 0; padding: 0; list-style: none; }
.vs-plan-item { display: grid; min-width: 0; }
.vs-plan-card-head { display: flex; align-items: center; gap: 8px; min-width: 0; }
.vs-plan-num { flex: none; display: grid; place-items: center; width: 26px; height: 26px; border-radius: 50%; background: var(--vs-accent-soft); color: var(--vs-accent); font-size: 12px; font-weight: 700; }
.vs-plan-card-head input { flex: 1 1 auto; min-width: 0; font-weight: 650; }
.vs-plan-chip { flex: none; padding: 2px 8px; border-radius: 999px; background: var(--vs-inset); color: var(--vs-muted); font-size: 11.5px; font-variant-numeric: tabular-nums; white-space: nowrap; }
.vs-plan-card .vs-field, .vs-plan-settings .vs-field { margin-top: 8px; }
.vs-plan-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.vs-plan-neg { margin-top: 8px; font-size: 12px; color: var(--vs-muted); }
.vs-plan-neg > summary { cursor: pointer; }
.vs-plan-neg textarea { width: 100%; margin-top: 6px; color: var(--vs-text); }
.vs-plan-fixed { margin: 0; padding: 7px 10px; border-radius: var(--vs-field-radius); background: var(--vs-inset); color: var(--vs-text); }
.vs-plan-transition { display: flex; flex-wrap: wrap; align-items: center; justify-content: center; gap: 6px; padding: 6px 0; color: var(--vs-muted); font-size: 12px; }
.vs-plan-transition select, .vs-plan-transition input { max-width: 100%; padding: 4px 8px; font-size: 12px; }
.vs-plan-transition input { width: 72px; }
.vs-plan-inline { display: inline-flex; align-items: center; gap: 6px; }
.vs-plan-transition > [data-plan-issues] { flex: 1 1 100%; }
.vs-plan-transition .vs-plan-issue { justify-content: center; }
.vs-plan-issue { display: flex; align-items: flex-start; gap: 6px; margin: 6px 0 0; font-size: 12px; line-height: 1.4; color: color-mix(in srgb, var(--vs-warn) 70%, var(--vs-text)); }
.vs-plan-issue.is-blocking { color: color-mix(in srgb, var(--vs-danger) 78%, var(--vs-text)); }
.vs-plan-issue span { min-width: 0; overflow-wrap: anywhere; }
.vs-plan [data-plan-issues]:empty { display: none; }
.vs-plan-actions { margin-top: 10px; }
.vs-plan-up { transform: rotate(180deg); }
.vs-plan-bar { flex: 0 0 auto; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 12px; padding: 8px 14px; border-top: 1px solid var(--vs-border); background: var(--vs-bar); box-shadow: var(--vs-bar-edge); }
.vs-plan-facts { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 14px; min-width: 0; font-size: 12.5px; color: var(--vs-muted); font-variant-numeric: tabular-nums; }
.vs-plan-facts > span { display: inline-flex; align-items: center; gap: 5px; }
.vs-plan-facts .is-blocking { color: color-mix(in srgb, var(--vs-danger) 78%, var(--vs-text)); }
.vs-plan-facts .is-ok .vs-icon { color: var(--vs-ok); }
.vs-plan-bar .vs-primary { margin-left: auto; }
@container vs-app (max-width: 650px) {
    .vs-plan-mode .vs-workspace { grid-template-columns: minmax(0, 1fr); }
}
@container vs-app (max-width: 600px) {
    .vs-toolbar { flex-wrap: wrap; row-gap: 6px; }
    .vs-app .vs-mode-switch { order: 9; flex: 1 1 100%; }
    .vs-app .vs-mode-switch button { flex: 1 1 0; }
}
@container vs-center (max-width: 600px) {
    .vs-plan-scroll { padding: 10px; }
    .vs-plan-empty { margin: 4px auto; padding: 14px; }
    .vs-plan-character { grid-template-columns: minmax(0, 1fr) auto; }
    .vs-plan-character textarea { grid-column: 1 / -1; grid-row: 2; }
}
@container vs-center (max-width: 420px) {
    .vs-plan-grid { grid-template-columns: minmax(0, 1fr); }
}
```

- [ ] **Step 7: Script lists.**

`ui/js/desktop/core/module-loader.js` (`'video-studio'` entry). **Old:**
```js
'/js/desktop/apps/video-studio-menus.js', '/js/desktop/apps/video-studio.js']
```
**New:**
```js
'/js/desktop/apps/video-studio-menus.js', '/js/desktop/apps/video-studio-plan-board.js', '/js/desktop/apps/video-studio-plan.js', '/js/desktop/apps/video-studio.js']
```

`ui/desktop_video_studio_browser_test.go` (fixture HTML, line ~67). **Old:**
```
<script src="/js/desktop/apps/video-studio-menus.js"></script><script src="/js/desktop/apps/video-studio.js"></script>
```
**New:**
```
<script src="/js/desktop/apps/video-studio-menus.js"></script><script src="/js/desktop/apps/video-studio-plan-board.js"></script><script src="/js/desktop/apps/video-studio-plan.js"></script><script src="/js/desktop/apps/video-studio.js"></script>
```

`internal/server/video_studio_browser_test.go` (fixture HTML). **Old:**
```
<script src="/js/desktop/apps/video-studio-menus.js"></script>
<script src="/js/desktop/apps/video-studio.js"></script>
```
**New:**
```
<script src="/js/desktop/apps/video-studio-menus.js"></script>
<script src="/js/desktop/apps/video-studio-plan-board.js"></script>
<script src="/js/desktop/apps/video-studio-plan.js"></script>
<script src="/js/desktop/apps/video-studio.js"></script>
```

- [ ] **Step 8: Run.**
```
go test ./ui -run '(?i)(TestVideoStudioNavigationLoadOrder|TestVideoStudioCountStringsHaveSingulars|TestVideoStudioTrCount|TestVideoStudioTransitionPicker)' -count=1
node --check ui/js/desktop/apps/video-studio-plan-board.js
node --check ui/js/desktop/apps/video-studio-plan.js
npm run check:ui
```
Expected: `ok  	aurago/ui`. Both `node --check` runs print nothing. `check:ui` reports the bundles unchanged. If `TestVideoStudioTransitionPicker` is not the real name, drop it: the point is that the source-order assertions in `ui/desktop_video_studio_transitions_test.go` (picker Escape before drawer Escape in `handleKeys`) still pass, and the new guard sits after both.

- [ ] **Step 9: Commit** (run `detect_changes --scope all` from the main checkout first, as CLAUDE.md requires)
```
git add ui/js/desktop/apps/video-studio-plan-board.js ui/js/desktop/apps/video-studio-plan.js ui/js/desktop/apps/video-studio.js ui/css/desktop-app-video-studio.css ui/js/desktop/core/module-loader.js ui/desktop_video_studio_browser_test.go ui/desktop_video_studio_navigation_test.go internal/server/video_studio_browser_test.go
git commit -m "feat(video-studio): storyboard panel with the Edit | Plan switch

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task D2: Plan-mode strings in all sixteen desktop locales

**Files:**
- Create (outside the repo): `../vsplan1-scripts/add-plan-strings.mjs`
- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`. 122 lines are inserted after the last `  "videoStudio.` line of each file.
- Create: `ui/desktop_video_studio_plan_strings_test.go`

Checked: no test enforces sorted keys or global key parity in `lang/desktop`. `TestDesktopUsedI18nKeysExistInAllLanguages` ignores the `videoStudio`/`plan` prefixes. Two Video Studio rules apply: the `_one` rule (`TestVideoStudioCountStringsHaveSingulars`) and the new test below.

- [ ] **Step 1: Write the failing test** `ui/desktop_video_studio_plan_strings_test.go`:

```go
package ui

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The validator's issue codes (brief §4); the board reads videoStudio.plan.issue.<code>.
var videoStudioPlanIssueCodes = []string{
	"no_scenes", "too_many_scenes", "target_seconds_invalid", "total_length_mismatch", "total_length_too_long",
	"idea_too_long", "style_too_long", "too_many_characters", "character_invalid", "character_reference_invalid",
	"title_missing", "title_too_long", "prompt_missing", "prompt_too_long", "negative_prompt_too_long",
	"duration_unsupported", "use_seconds_invalid", "transition_unknown", "transition_unsupported",
	"first_scene_transition", "chain_too_long", "blend_type_unknown", "blend_type_unavailable", "blend_overlap_invalid",
	"keyframe_source_invalid", "keyframe_prompt_missing", "keyframe_asset_invalid", "narration_unavailable",
	"narration_too_long", "music_unavailable", "clip_audio_invalid", "plan_unsupported_provider", "profile_changed",
}

// Every plan-mode string the storyboard reads (each 'plan.*' literal in the two plan scripts, the _one form of the
// trCount keys, the dynamic families and one issue text per validator code) exists in all sixteen desktop locales
// with the English placeholders.
func TestDesktopVideoStudioPlanStrings(t *testing.T) {
	literal := regexp.MustCompile(`'(plan\.[A-Za-z0-9_.]*[A-Za-z0-9_])'`)
	counted := regexp.MustCompile(`trCount\(s, '(plan\.[A-Za-z0-9_.]*[A-Za-z0-9_])'`)
	keys := map[string]bool{}
	for _, file := range []string{"js/desktop/apps/video-studio-plan.js", "js/desktop/apps/video-studio-plan-board.js"} {
		source := readDesktopAssetText(t, file)
		for _, match := range literal.FindAllStringSubmatch(source, -1) {
			keys[match[1]] = true
		}
		for _, match := range counted.FindAllStringSubmatch(source, -1) {
			keys[match[1]+"_one"] = true
		}
	}
	if len(keys) < 60 {
		t.Fatalf("found only %d plan keys in the plan scripts", len(keys))
	}
	families := map[string][]string{
		"phase":     {"outline", "details", "check", "revise"},
		"kind":      {"cut", "continue", "bridge", "blend"},
		"source":    {"generated", "library", "none", "chain", "bridge"},
		"clipAudio": {"keep", "duck", "mute"},
		"reason":    {"provider_images_public_url_only", "provider_unsupported", "generation_unavailable"},
		"issue":     videoStudioPlanIssueCodes,
	}
	for family, names := range families {
		for _, name := range names {
			keys["plan."+family+"."+name] = true
		}
	}
	placeholder := regexp.MustCompile(`\{\{[a-z]+\}\}`)
	placeholders := func(text string) string {
		found := placeholder.FindAllString(text, -1)
		sort.Strings(found)
		return strings.Join(found, ",")
	}
	english := readDesktopLocale(t, "en")
	for _, lang := range videoStudioLocales {
		words := readDesktopLocale(t, lang)
		for key := range keys {
			full := "videoStudio." + key
			value := strings.TrimSpace(words[full])
			if value == "" {
				t.Errorf("%s lacks %s", lang, full)
				continue
			}
			if got, want := placeholders(value), placeholders(english[full]); got != want {
				t.Errorf("%s %s has placeholders %q, English has %q", lang, full, got, want)
			}
		}
	}
}
```

- [ ] **Step 2: Run it.**
`go test ./ui -run '(?i)DesktopVideoStudioPlanStrings' -count=1`
Expected: FAIL with lines like `cs lacks videoStudio.plan.modeLabel` (and so on for every locale).

- [ ] **Step 3: Write `../vsplan1-scripts/add-plan-strings.mjs`** (outside the repo; create the folder next to the worktree). The order is `cs da de el en es fr hi it ja nl no pl pt sv zh`. The script uses no backslash escapes, so the Write tool cannot mangle it.

```js
// Inserts the Video Studio plan-mode strings (videoStudio.plan.*) into the 16 desktop locale files.
// Usage from the worktree root: node ../vsplan1-scripts/add-plan-strings.mjs ui/lang/desktop
// New keys go right after the last "videoStudio." line of each file (the Video Studio group), in table order.
// Existing keys are left alone, so a second run adds nothing. LF line endings are kept; CRLF files are refused.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const dir = process.argv[2] || 'ui/lang/desktop';
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const NL = String.fromCharCode(10);
const CR = String.fromCharCode(13);
const T = {
  "plan.modeLabel": ["Režim", "Tilstand", "Modus", "Λειτουργία", "Mode", "Modo", "Mode", "मोड", "Modalità", "モード", "Modus", "Modus", "Tryb", "Modo", "Läge", "模式"],
  "plan.modeEdit": ["Úpravy", "Rediger", "Bearbeiten", "Επεξεργασία", "Edit", "Editar", "Édition", "संपादन", "Modifica", "編集", "Bewerken", "Rediger", "Edycja", "Edição", "Redigera", "编辑"],
  "plan.modePlan": ["Plán", "Plan", "Plan", "Σχέδιο", "Plan", "Plan", "Plan", "योजना", "Piano", "プラン", "Plan", "Plan", "Plan", "Plano", "Plan", "规划"],
  "plan.board": ["Storyboard", "Storyboard", "Storyboard", "Storyboard", "Storyboard", "Guion gráfico", "Storyboard", "स्टोरीबोर्ड", "Storyboard", "絵コンテ", "Storyboard", "Storyboard", "Scenorys", "Storyboard", "Storyboard", "故事板"],
  "plan.loading": ["Načítám plán…", "Indlæser planen…", "Plan wird geladen…", "Φόρτωση του σχεδίου…", "Loading the plan…", "Cargando el plan…", "Chargement du plan…", "योजना लोड हो रही है…", "Caricamento del piano…", "プランを読み込んでいます…", "Plan laden…", "Laster inn planen…", "Wczytywanie planu…", "A carregar o plano…", "Läser in planen…", "正在加载规划…"],
  "plan.loadFailed": ["Plán se nepodařilo načíst.", "Planen kunne ikke indlæses.", "Der Plan konnte nicht geladen werden.", "Δεν ήταν δυνατή η φόρτωση του σχεδίου.", "The plan could not be loaded.", "No se ha podido cargar el plan.", "Le plan n’a pas pu être chargé.", "योजना लोड नहीं हो सकी।", "Impossibile caricare il piano.", "プランを読み込めませんでした。", "Het plan kon niet worden geladen.", "Planen kunne ikke lastes inn.", "Nie udało się wczytać planu.", "Não foi possível carregar o plano.", "Planen kunde inte läsas in.", "无法加载规划。"],
  "plan.retry": ["Zkusit znovu", "Prøv igen", "Erneut versuchen", "Δοκίμασε ξανά", "Try again", "Reintentar", "Réessayer", "फिर से कोशिश करें", "Riprova", "再試行", "Opnieuw proberen", "Prøv igjen", "Spróbuj ponownie", "Tentar novamente", "Försök igen", "重试"],
  "plan.unsupportedTitle": ["Režim plánu není k dispozici", "Plantilstand er ikke tilgængelig", "Planmodus ist nicht verfügbar", "Η λειτουργία σχεδίου δεν είναι διαθέσιμη", "Plan mode is not available", "El modo plan no está disponible", "Le mode plan n’est pas disponible", "योजना मोड उपलब्ध नहीं है", "La modalità piano non è disponibile", "プランモードは利用できません", "Planmodus is niet beschikbaar", "Planmodus er ikke tilgjengelig", "Tryb planu jest niedostępny", "O modo de plano não está disponível", "Planläget är inte tillgängligt", "规划模式不可用"],
  "plan.unsupportedHint": ["Vyber podporovaného poskytovatele videa v Nastavení > Generování videa.", "Vælg en understøttet videoudbyder under Indstillinger > Videogenerering.", "Wähle unter Einstellungen > Videogenerierung einen unterstützten Videoanbieter.", "Επίλεξε έναν υποστηριζόμενο πάροχο βίντεο στις Ρυθμίσεις > Δημιουργία βίντεο.", "Choose a supported video provider in Settings > Video Generation.", "Elige un proveedor de vídeo compatible en Ajustes > Generación de vídeo.", "Choisis un fournisseur vidéo pris en charge dans Réglages > Génération vidéo.", "सेटिंग्स > वीडियो जनरेशन में कोई समर्थित वीडियो प्रदाता चुनें।", "Scegli un fornitore video supportato in Impostazioni > Generazione video.", "設定 > 動画生成 で対応している動画プロバイダーを選んでください。", "Kies een ondersteunde videoprovider in Instellingen > Videogeneratie.", "Velg en støttet videoleverandør under Innstillinger > Videogenerering.", "Wybierz obsługiwanego dostawcę wideo w Ustawienia > Generowanie wideo.", "Escolha um fornecedor de vídeo suportado em Definições > Geração de vídeo.", "Välj en videoleverantör som stöds under Inställningar > Videogenerering.", "请在“设置 > 视频生成”中选择受支持的视频提供商。"],
  "plan.reason.provider_images_public_url_only": ["Tento poskytovatel videa přijímá úvodní obrázky jen jako veřejné webové adresy.", "Denne videoudbyder accepterer kun startbilleder som offentlige webadresser.", "Dieser Videoanbieter nimmt Startbilder nur als öffentliche Webadressen an.", "Αυτός ο πάροχος βίντεο δέχεται εικόνες έναρξης μόνο ως δημόσιες διευθύνσεις web.", "This video provider accepts start images only as public web addresses.", "Este proveedor de vídeo solo acepta imágenes iniciales como direcciones web públicas.", "Ce fournisseur vidéo n’accepte les images de départ que sous forme d’adresses web publiques.", "यह वीडियो प्रदाता शुरुआती इमेज केवल सार्वजनिक वेब पतों के रूप में स्वीकार करता है।", "Questo fornitore video accetta immagini iniziali solo come indirizzi web pubblici.", "この動画プロバイダーは開始画像を公開 Web アドレスとしてしか受け付けません。", "Deze videoprovider accepteert startafbeeldingen alleen als openbare webadressen.", "Denne videoleverandøren godtar bare startbilder som offentlige nettadresser.", "Ten dostawca wideo przyjmuje obrazy startowe tylko jako publiczne adresy internetowe.", "Este fornecedor de vídeo só aceita imagens iniciais como endereços web públicos.", "Den här videoleverantören tar bara emot startbilder som offentliga webbadresser.", "此视频提供商只接受公开网址形式的起始图片。"],
  "plan.reason.provider_unsupported": ["Nastavený poskytovatel videa nepodporuje režim plánu.", "Den valgte videoudbyder understøtter ikke plantilstand.", "Der eingestellte Videoanbieter unterstützt den Planmodus nicht.", "Ο ρυθμισμένος πάροχος βίντεο δεν υποστηρίζει τη λειτουργία σχεδίου.", "The configured video provider does not support plan mode.", "El proveedor de vídeo configurado no admite el modo plan.", "Le fournisseur vidéo configuré ne prend pas en charge le mode plan.", "कॉन्फ़िगर किया गया वीडियो प्रदाता योजना मोड का समर्थन नहीं करता।", "Il fornitore video configurato non supporta la modalità piano.", "設定されている動画プロバイダーはプランモードに対応していません。", "De ingestelde videoprovider ondersteunt de planmodus niet.", "Den valgte videoleverandøren støtter ikke planmodus.", "Skonfigurowany dostawca wideo nie obsługuje trybu planu.", "O fornecedor de vídeo configurado não suporta o modo de plano.", "Den inställda videoleverantören stöder inte planläget.", "所配置的视频提供商不支持规划模式。"],
  "plan.reason.generation_unavailable": ["Generování videa není nastavené.", "Videogenerering er ikke sat op.", "Die Videogenerierung ist nicht eingerichtet.", "Η δημιουργία βίντεο δεν έχει ρυθμιστεί.", "Video generation is not set up.", "La generación de vídeo no está configurada.", "La génération vidéo n’est pas configurée.", "वीडियो जनरेशन सेट अप नहीं है।", "La generazione video non è configurata.", "動画生成が設定されていません。", "Videogeneratie is niet ingesteld.", "Videogenerering er ikke satt opp.", "Generowanie wideo nie jest skonfigurowane.", "A geração de vídeo não está configurada.", "Videogenerering är inte konfigurerad.", "尚未设置视频生成。"],
  "plan.emptyTitle": ["Naplánuj klip s agentem", "Planlæg et klip med agenten", "Plane einen Clip mit dem Agenten", "Σχεδίασε ένα κλιπ με τον πράκτορα", "Plan a clip with the agent", "Planifica un clip con el agente", "Planifie un clip avec l’agent", "एजेंट के साथ क्लिप की योजना बनाएँ", "Pianifica una clip con l’agente", "エージェントとクリップを計画", "Plan een clip met de agent", "Planlegg et klipp med agenten", "Zaplanuj klip z agentem", "Planeie um clipe com o agente", "Planera ett klipp med agenten", "与智能体一起规划短片"],
  "plan.emptyCopy": ["Popiš svůj nápad. Agent naplánuje scény jako storyboard. Zatím se nic negeneruje ani neplatí.", "Beskriv din idé. Agenten planlægger scenerne som et storyboard. Der bliver endnu ikke genereret eller betalt for noget.", "Beschreibe deine Idee. Der Agent plant die Szenen als Storyboard. Noch wird nichts erzeugt oder bezahlt.", "Περίγραψε την ιδέα σου. Ο πράκτορας σχεδιάζει τις σκηνές ως storyboard. Τίποτα δεν δημιουργείται ούτε χρεώνεται ακόμη.", "Describe your idea. The agent plans the scenes as a storyboard. Nothing is generated or paid for yet.", "Describe tu idea. El agente planifica las escenas como un guion gráfico. Todavía no se genera ni se paga nada.", "Décris ton idée. L’agent planifie les scènes sous forme de storyboard. Rien n’est encore généré ni payé.", "अपना आइडिया लिखें। एजेंट दृश्यों की योजना स्टोरीबोर्ड के रूप में बनाता है। अभी कुछ भी जनरेट या भुगतान नहीं होता।", "Descrivi la tua idea. L’agente pianifica le scene come storyboard. Per ora non viene generato né pagato nulla.", "アイデアを書いてください。エージェントがシーンを絵コンテとして計画します。この段階では何も生成されず、料金も発生しません。", "Beschrijf je idee. De agent plant de scènes als storyboard. Er wordt nog niets gegenereerd of betaald.", "Beskriv ideen din. Agenten planlegger scenene som et storyboard. Ingenting blir generert eller betalt for ennå.", "Opisz swój pomysł. Agent zaplanuje sceny jako scenorys. Na razie nic nie jest generowane ani opłacane.", "Descreva a sua ideia. O agente planeia as cenas como um storyboard. Ainda nada é gerado nem pago.", "Beskriv din idé. Agenten planerar scenerna som en storyboard. Ingenting genereras eller betalas ännu.", "描述你的创意。智能体会把场景规划成故事板。此时不会生成任何内容，也不会产生费用。"],
  "plan.emptyReadonly": ["Tento projekt zatím nemá plán.", "Dette projekt har endnu ingen plan.", "Dieses Projekt hat noch keinen Plan.", "Αυτό το έργο δεν έχει ακόμη σχέδιο.", "This project has no plan yet.", "Este proyecto aún no tiene plan.", "Ce projet n’a pas encore de plan.", "इस प्रोजेक्ट की अभी कोई योजना नहीं है।", "Questo progetto non ha ancora un piano.", "このプロジェクトにはまだプランがありません。", "Dit project heeft nog geen plan.", "Dette prosjektet har ingen plan ennå.", "Ten projekt nie ma jeszcze planu.", "Este projeto ainda não tem plano.", "Det här projektet har ingen plan än.", "此项目还没有规划。"],
  "plan.damaged": ["Uložený plán je poškozený. Vytvoř nový plán, který ho nahradí.", "Den gemte plan er beskadiget. Opret en ny plan for at erstatte den.", "Der gespeicherte Plan ist beschädigt. Erstelle einen neuen Plan, um ihn zu ersetzen.", "Το αποθηκευμένο σχέδιο είναι κατεστραμμένο. Δημιούργησε νέο σχέδιο για να το αντικαταστήσεις.", "The stored plan is damaged. Create a new plan to replace it.", "El plan guardado está dañado. Crea un plan nuevo para sustituirlo.", "Le plan enregistré est endommagé. Crée un nouveau plan pour le remplacer.", "सहेजी गई योजना खराब हो गई है। उसकी जगह नई योजना बनाएँ।", "Il piano salvato è danneggiato. Crea un nuovo piano per sostituirlo.", "保存されたプランが壊れています。新しいプランを作成して置き換えてください。", "Het opgeslagen plan is beschadigd. Maak een nieuw plan om het te vervangen.", "Den lagrede planen er skadet. Lag en ny plan for å erstatte den.", "Zapisany plan jest uszkodzony. Utwórz nowy plan, aby go zastąpić.", "O plano guardado está danificado. Crie um novo plano para o substituir.", "Den sparade planen är skadad. Skapa en ny plan för att ersätta den.", "已保存的规划已损坏。请创建新的规划来替换它。"],
  "plan.idea": ["Nápad", "Idé", "Idee", "Ιδέα", "Idea", "Idea", "Idée", "आइडिया", "Idea", "アイデア", "Idee", "Idé", "Pomysł", "Ideia", "Idé", "创意"],
  "plan.ideaHint": ["30 sekund: rybář vyráží za úsvitu a nad zátokou se probouzí den", "30 sekunder: en fisker sejler ud ved daggry, og dagen vågner over bugten", "30 Sekunden: Ein Fischer fährt im Morgengrauen hinaus, und über der Bucht erwacht der Tag", "30 δευτερόλεπτα: ένας ψαράς βγαίνει την αυγή και η μέρα ξυπνά πάνω από τον κόλπο", "30 seconds: a fisherman heads out at dawn and the day wakes over the bay", "30 segundos: un pescador sale al amanecer y el día despierta sobre la bahía", "30 secondes : un pêcheur part à l’aube et le jour se lève sur la baie", "30 सेकंड: एक मछुआरा भोर में निकलता है और खाड़ी पर दिन जागता है", "30 secondi: un pescatore esce all’alba e il giorno si sveglia sulla baia", "30 秒：夜明けに漁師が船を出し、湾の上で一日が目覚める", "30 seconden: een visser vaart bij dageraad uit en boven de baai ontwaakt de dag", "30 sekunder: en fisker drar ut ved daggry, og dagen våkner over bukta", "30 sekund: rybak wypływa o świcie, a nad zatoką budzi się dzień", "30 segundos: um pescador sai ao amanhecer e o dia desperta sobre a baía", "30 sekunder: en fiskare ger sig ut i gryningen och dagen vaknar över viken", "30 秒：渔夫在黎明出海，海湾上的白昼渐渐苏醒"],
  "plan.target": ["Cílová délka", "Ønsket længde", "Ziellänge", "Επιθυμητή διάρκεια", "Target length", "Duración objetivo", "Durée cible", "लक्षित अवधि", "Durata obiettivo", "目標の長さ", "Beoogde lengte", "Ønsket lengde", "Docelowa długość", "Duração pretendida", "Önskad längd", "目标时长"],
  "plan.secondsValue": ["{{value}} s", "{{value}} s", "{{value}} s", "{{value}} s", "{{value}} s", "{{value}} s", "{{value}} s", "{{value}} सेकंड", "{{value}} s", "{{value}} 秒", "{{value}} s", "{{value}} s", "{{value}} s", "{{value}} s", "{{value}} s", "{{value}} 秒"],
  "plan.create": ["Vytvořit plán", "Opret plan", "Plan erstellen", "Δημιουργία σχεδίου", "Create plan", "Crear plan", "Créer le plan", "योजना बनाएँ", "Crea piano", "プランを作成", "Plan maken", "Lag plan", "Utwórz plan", "Criar plano", "Skapa plan", "创建规划"],
  "plan.replan": ["Naplánovat znovu", "Planlæg igen", "Neu planen", "Νέος σχεδιασμός", "Plan again", "Volver a planificar", "Replanifier", "फिर से योजना बनाएँ", "Ripianifica", "再計画", "Opnieuw plannen", "Planlegg på nytt", "Zaplanuj ponownie", "Planear de novo", "Planera om", "重新规划"],
  "plan.replanTitle": ["Naplánovat znovu?", "Planlæg igen?", "Neu planen?", "Νέος σχεδιασμός;", "Plan again?", "¿Volver a planificar?", "Replanifier ?", "फिर से योजना बनाएँ?", "Ripianificare?", "再計画しますか？", "Opnieuw plannen?", "Planlegge på nytt?", "Zaplanować ponownie?", "Planear de novo?", "Planera om?", "重新规划？"],
  "plan.replanCopy": ["Agent naplánuje nový storyboard. Nahradí současný plán i tvoje úpravy.", "Agenten planlægger et nyt storyboard. Det erstatter den nuværende plan og dine ændringer.", "Der Agent plant ein neues Storyboard. Es ersetzt den aktuellen Plan und deine Änderungen.", "Ο πράκτορας σχεδιάζει νέο storyboard. Αντικαθιστά το τρέχον σχέδιο και τις αλλαγές σου.", "The agent plans a new storyboard. It replaces the current plan and your edits.", "El agente planifica un nuevo guion gráfico. Sustituye el plan actual y tus cambios.", "L’agent planifie un nouveau storyboard. Il remplace le plan actuel et tes modifications.", "एजेंट एक नया स्टोरीबोर्ड बनाता है। यह मौजूदा योजना और आपके बदलावों की जगह लेता है।", "L’agente pianifica un nuovo storyboard. Sostituisce il piano attuale e le tue modifiche.", "エージェントが新しい絵コンテを計画します。現在のプランと編集内容は置き換えられます。", "De agent plant een nieuw storyboard. Het vervangt het huidige plan en je wijzigingen.", "Agenten planlegger et nytt storyboard. Det erstatter den nåværende planen og endringene dine.", "Agent zaplanuje nowy scenorys. Zastąpi on obecny plan i twoje zmiany.", "O agente planeia um novo storyboard. Substitui o plano atual e as suas alterações.", "Agenten planerar en ny storyboard. Den ersätter den nuvarande planen och dina ändringar.", "智能体会规划新的故事板，并替换当前规划和你的修改。"],
  "plan.stop": ["Zastavit", "Stop", "Stoppen", "Διακοπή", "Stop", "Detener", "Arrêter", "रोकें", "Interrompi", "停止", "Stoppen", "Stopp", "Zatrzymaj", "Parar", "Stoppa", "停止"],
  "plan.planning": ["Plánuji", "Planlægger", "Plant", "Σχεδιασμός", "Planning", "Planificando", "Planification", "योजना बन रही है", "Pianificazione", "計画中", "Bezig met plannen", "Planlegger", "Planowanie", "A planear", "Planerar", "规划中"],
  "plan.phase.outline": ["Osnova scén", "Sceneoversigt", "Szenenübersicht", "Περίγραμμα σκηνών", "Scene outline", "Esquema de escenas", "Plan des scènes", "दृश्यों की रूपरेखा", "Scaletta delle scene", "シーン構成", "Scèneoverzicht", "Sceneoversikt", "Zarys scen", "Esboço das cenas", "Scenöversikt", "场景大纲"],
  "plan.phase.details": ["Podrobnosti scén", "Scenedetaljer", "Szenendetails", "Λεπτομέρειες σκηνών", "Scene details", "Detalles de las escenas", "Détails des scènes", "दृश्यों का विवरण", "Dettagli delle scene", "シーンの詳細", "Scènedetails", "Scenedetaljer", "Szczegóły scen", "Detalhes das cenas", "Scendetaljer", "场景细节"],
  "plan.phase.check": ["Kontrola plánu", "Tjekker planen", "Plan wird geprüft", "Έλεγχος του σχεδίου", "Checking the plan", "Comprobando el plan", "Vérification du plan", "योजना की जाँच", "Verifica del piano", "プランを確認中", "Plan controleren", "Kontrollerer planen", "Sprawdzanie planu", "A verificar o plano", "Kontrollerar planen", "检查规划"],
  "plan.phase.revise": ["Úprava scény", "Reviderer en scene", "Szene wird überarbeitet", "Αναθεώρηση σκηνής", "Revising a scene", "Revisando una escena", "Révision d’une scène", "दृश्य में संशोधन", "Revisione di una scena", "シーンを修正中", "Scène herzien", "Reviderer en scene", "Poprawianie sceny", "A rever uma cena", "Reviderar en scen", "修改场景"],
  "plan.agentStopped": ["Plánování bylo zastaveno. Plán si ponechá to, co bylo dosud naplánováno.", "Planlægningen blev stoppet. Planen beholder det, der er planlagt indtil nu.", "Die Planung wurde gestoppt. Der Plan behält, was bisher geplant wurde.", "Ο σχεδιασμός διακόπηκε. Το σχέδιο κρατά ό,τι σχεδιάστηκε μέχρι τώρα.", "Planning was stopped. The plan keeps what was planned so far.", "Se detuvo la planificación. El plan conserva lo planificado hasta ahora.", "La planification a été arrêtée. Le plan garde ce qui a déjà été planifié.", "योजना बनाना रोक दिया गया। अब तक जो योजना बनी है, वह रहती है।", "La pianificazione è stata interrotta. Il piano conserva quanto pianificato finora.", "計画を停止しました。ここまでの計画内容はプランに残ります。", "Het plannen is gestopt. Het plan houdt wat tot nu toe gepland is.", "Planleggingen ble stoppet. Planen beholder det som er planlagt så langt.", "Planowanie zostało zatrzymane. Plan zachowuje to, co dotąd zaplanowano.", "O planeamento foi parado. O plano mantém o que foi planeado até agora.", "Planeringen stoppades. Planen behåller det som har planerats hittills.", "规划已停止。规划会保留目前已规划的内容。"],
  "plan.agentFailed": ["Plánování skončilo předčasně. Plán si ponechá to, co bylo dosud naplánováno.", "Planlægningen sluttede for tidligt. Planen beholder det, der er planlagt indtil nu.", "Die Planung endete vorzeitig. Der Plan behält, was bisher geplant wurde.", "Ο σχεδιασμός τελείωσε πρόωρα. Το σχέδιο κρατά ό,τι σχεδιάστηκε μέχρι τώρα.", "Planning ended early. The plan keeps what was planned so far.", "La planificación terminó antes de tiempo. El plan conserva lo planificado hasta ahora.", "La planification s’est terminée plus tôt que prévu. Le plan garde ce qui a déjà été planifié.", "योजना बनाना समय से पहले रुक गया। अब तक जो योजना बनी है, वह रहती है।", "La pianificazione è terminata in anticipo. Il piano conserva quanto pianificato finora.", "計画が途中で終了しました。ここまでの計画内容はプランに残ります。", "Het plannen stopte voortijdig. Het plan houdt wat tot nu toe gepland is.", "Planleggingen stoppet for tidlig. Planen beholder det som er planlagt så langt.", "Planowanie zakończyło się przedwcześnie. Plan zachowuje to, co dotąd zaplanowano.", "O planeamento terminou mais cedo. O plano mantém o que foi planeado até agora.", "Planeringen avslutades i förtid. Planen behåller det som har planerats hittills.", "规划提前结束。规划会保留目前已规划的内容。"],
  "plan.readonlyHint": ["Jen pro čtení: storyboard lze prohlížet, ale ne měnit.", "Skrivebeskyttet: storyboardet kan ses, men ikke ændres.", "Schreibgeschützt: Das Storyboard kann angesehen, aber nicht geändert werden.", "Μόνο ανάγνωση: το storyboard μπορεί να προβληθεί αλλά όχι να αλλάξει.", "Read-only: the storyboard can be viewed but not changed.", "Solo lectura: el guion gráfico se puede ver, pero no cambiar.", "Lecture seule : le storyboard peut être consulté, mais pas modifié.", "केवल पढ़ने के लिए: स्टोरीबोर्ड देखा जा सकता है, बदला नहीं जा सकता।", "Sola lettura: lo storyboard si può vedere ma non modificare.", "読み取り専用：絵コンテは表示できますが、変更できません。", "Alleen-lezen: het storyboard kan worden bekeken, maar niet gewijzigd.", "Skrivebeskyttet: storyboardet kan vises, men ikke endres.", "Tylko do odczytu: scenorys można przeglądać, ale nie zmieniać.", "Só de leitura: o storyboard pode ser visto, mas não alterado.", "Skrivskyddat: storyboarden kan visas men inte ändras.", "只读：可以查看故事板，但不能修改。"],
  "plan.planSettings": ["Styl a zvuk", "Stil og lyd", "Stil und Ton", "Ύφος και ήχος", "Style and sound", "Estilo y sonido", "Style et son", "शैली और ध्वनि", "Stile e suono", "スタイルとサウンド", "Stijl en geluid", "Stil og lyd", "Styl i dźwięk", "Estilo e som", "Stil och ljud", "风格与声音"],
  "plan.style": ["Styl", "Stil", "Stil", "Ύφος", "Style", "Estilo", "Style", "शैली", "Stile", "スタイル", "Stijl", "Stil", "Styl", "Estilo", "Stil", "风格"],
  "plan.characters": ["Postavy", "Personer", "Figuren", "Χαρακτήρες", "Characters", "Personajes", "Personnages", "पात्र", "Personaggi", "登場人物", "Personages", "Personer", "Postacie", "Personagens", "Karaktärer", "角色"],
  "plan.characterName": ["Jméno", "Navn", "Name", "Όνομα", "Name", "Nombre", "Nom", "नाम", "Nome", "名前", "Naam", "Navn", "Imię", "Nome", "Namn", "名称"],
  "plan.characterDescription": ["Vzhled a oblečení", "Udseende og tøj", "Aussehen und Kleidung", "Εμφάνιση και ρούχα", "Look and clothing", "Aspecto y ropa", "Apparence et vêtements", "रूप और कपड़े", "Aspetto e abiti", "外見と服装", "Uiterlijk en kleding", "Utseende og klær", "Wygląd i ubiór", "Aspeto e roupa", "Utseende och kläder", "外貌与服装"],
  "plan.addCharacter": ["Přidat postavu", "Tilføj person", "Figur hinzufügen", "Προσθήκη χαρακτήρα", "Add character", "Añadir personaje", "Ajouter un personnage", "पात्र जोड़ें", "Aggiungi personaggio", "登場人物を追加", "Personage toevoegen", "Legg til person", "Dodaj postać", "Adicionar personagem", "Lägg till karaktär", "添加角色"],
  "plan.removeCharacter": ["Odebrat postavu", "Fjern person", "Figur entfernen", "Αφαίρεση χαρακτήρα", "Remove character", "Quitar personaje", "Retirer le personnage", "पात्र हटाएँ", "Rimuovi personaggio", "登場人物を削除", "Personage verwijderen", "Fjern person", "Usuń postać", "Remover personagem", "Ta bort karaktär", "移除角色"],
  "plan.music": ["Zadání hudby", "Musikprompt", "Musik-Prompt", "Περιγραφή μουσικής", "Music prompt", "Indicación de música", "Prompt musical", "संगीत प्रॉम्प्ट", "Prompt musicale", "音楽プロンプト", "Muziekprompt", "Musikkprompt", "Opis muzyki", "Descrição da música", "Musikprompt", "音乐提示词"],
  "plan.clipAudio": ["Zvuk klipů", "Klippenes lyd", "Clip-Ton", "Ήχος κλιπ", "Clip sound", "Sonido de los clips", "Son des clips", "क्लिप की ध्वनि", "Audio delle clip", "クリップの音声", "Clipgeluid", "Klippenes lyd", "Dźwięk klipów", "Som dos clipes", "Klippens ljud", "片段声音"],
  "plan.clipAudio.keep": ["Ponechat", "Behold", "Behalten", "Διατήρηση", "Keep", "Mantener", "Conserver", "रखें", "Mantieni", "そのまま", "Behouden", "Behold", "Zachowaj", "Manter", "Behåll", "保留"],
  "plan.clipAudio.duck": ["Zeslabit", "Dæmp", "Absenken", "Χαμήλωμα", "Lower", "Bajar", "Baisser", "धीमा करें", "Abbassa", "下げる", "Zachter", "Demp", "Przycisz", "Baixar", "Sänk", "降低"],
  "plan.clipAudio.mute": ["Vypnout", "Slå fra", "Stumm", "Σίγαση", "Mute", "Silenciar", "Couper", "म्यूट", "Disattiva", "ミュート", "Dempen", "Slå av", "Wycisz", "Silenciar", "Stäng av", "静音"],
  "plan.sceneNumber": ["Scéna {{number}}", "Scene {{number}}", "Szene {{number}}", "Σκηνή {{number}}", "Scene {{number}}", "Escena {{number}}", "Scène {{number}}", "दृश्य {{number}}", "Scena {{number}}", "シーン {{number}}", "Scène {{number}}", "Scene {{number}}", "Scena {{number}}", "Cena {{number}}", "Scen {{number}}", "场景 {{number}}"],
  "plan.title": ["Název", "Titel", "Titel", "Τίτλος", "Title", "Título", "Titre", "शीर्षक", "Titolo", "タイトル", "Titel", "Tittel", "Tytuł", "Título", "Titel", "标题"],
  "plan.videoPrompt": ["Zadání videa", "Videoprompt", "Video-Prompt", "Περιγραφή βίντεο", "Video prompt", "Indicación de vídeo", "Prompt vidéo", "वीडियो प्रॉम्प्ट", "Prompt video", "動画プロンプト", "Videoprompt", "Videoprompt", "Opis wideo", "Descrição do vídeo", "Videoprompt", "视频提示词"],
  "plan.negativePrompt": ["Negativní zadání", "Negativ prompt", "Negativ-Prompt", "Αρνητική περιγραφή", "Negative prompt", "Indicación negativa", "Prompt négatif", "नकारात्मक प्रॉम्प्ट", "Prompt negativo", "ネガティブプロンプト", "Negatieve prompt", "Negativ prompt", "Opis negatywny", "Descrição negativa", "Negativ prompt", "反向提示词"],
  "plan.generateSeconds": ["Generovat", "Generér", "Erzeugen", "Δημιουργία", "Generate", "Generar", "Générer", "जनरेट", "Genera", "生成", "Genereren", "Generer", "Generuj", "Gerar", "Generera", "生成"],
  "plan.useSeconds": ["Použít (s)", "Brug (s)", "Verwenden (s)", "Χρήση (s)", "Use (s)", "Usar (s)", "Utiliser (s)", "उपयोग (से.)", "Usa (s)", "使用（秒）", "Gebruiken (s)", "Bruk (s)", "Użyj (s)", "Usar (s)", "Använd (s)", "使用（秒）"],
  "plan.duration": ["{{use}} z {{generate}} s", "{{use}} af {{generate}} s", "{{use}} von {{generate}} s", "{{use}} από {{generate}} s", "{{use}} of {{generate}} s", "{{use}} de {{generate}} s", "{{use}} sur {{generate}} s", "{{generate}} में से {{use}} से.", "{{use}} di {{generate}} s", "{{generate}} 秒中 {{use}} 秒", "{{use}} van {{generate}} s", "{{use}} av {{generate}} s", "{{use}} z {{generate}} s", "{{use}} de {{generate}} s", "{{use}} av {{generate}} s", "{{use}} / {{generate}} 秒"],
  "plan.keyframe": ["Úvodní obrázek", "Startbillede", "Startbild", "Εικόνα έναρξης", "Start image", "Imagen inicial", "Image de départ", "शुरुआती इमेज", "Immagine iniziale", "開始画像", "Startafbeelding", "Startbilde", "Obraz startowy", "Imagem inicial", "Startbild", "起始图片"],
  "plan.source.generated": ["Generovaný obrázek", "Genereret billede", "Erzeugtes Bild", "Δημιουργημένη εικόνα", "Generated image", "Imagen generada", "Image générée", "जनरेट की गई इमेज", "Immagine generata", "生成画像", "Gegenereerde afbeelding", "Generert bilde", "Wygenerowany obraz", "Imagem gerada", "Genererad bild", "生成的图片"],
  "plan.source.library": ["Obrázek z knihovny", "Billede fra biblioteket", "Bild aus der Mediathek", "Εικόνα από τη βιβλιοθήκη", "Library image", "Imagen de la biblioteca", "Image de la médiathèque", "लाइब्रेरी की इमेज", "Immagine dalla libreria", "ライブラリの画像", "Afbeelding uit de bibliotheek", "Bilde fra biblioteket", "Obraz z biblioteki", "Imagem da biblioteca", "Bild från biblioteket", "媒体库图片"],
  "plan.source.none": ["Jen text", "Kun tekst", "Nur Text", "Μόνο κείμενο", "Text only", "Solo texto", "Texte seul", "केवल टेक्स्ट", "Solo testo", "テキストのみ", "Alleen tekst", "Bare tekst", "Tylko tekst", "Só texto", "Endast text", "仅文本"],
  "plan.source.chain": ["Navazuje na scénu {{number}}", "Fortsætter fra scene {{number}}", "Setzt Szene {{number}} fort", "Συνεχίζει από τη σκηνή {{number}}", "Continues from scene {{number}}", "Continúa la escena {{number}}", "Prolonge la scène {{number}}", "दृश्य {{number}} से जारी", "Prosegue dalla scena {{number}}", "シーン {{number}} から続く", "Gaat verder vanaf scène {{number}}", "Fortsetter fra scene {{number}}", "Kontynuuje scenę {{number}}", "Continua a cena {{number}}", "Fortsätter från scen {{number}}", "延续场景 {{number}}"],
  "plan.source.bridge": ["Společný snímek se scénou {{number}}", "Fælles billede med scene {{number}}", "Gemeinsames Bild mit Szene {{number}}", "Κοινό καρέ με τη σκηνή {{number}}", "Shared frame with scene {{number}}", "Fotograma compartido con la escena {{number}}", "Image partagée avec la scène {{number}}", "दृश्य {{number}} के साथ साझा फ़्रेम", "Fotogramma condiviso con la scena {{number}}", "シーン {{number}} と共有するフレーム", "Gedeeld beeld met scène {{number}}", "Felles bilde med scene {{number}}", "Klatka wspólna ze sceną {{number}}", "Fotograma partilhado com a cena {{number}}", "Delad bildruta med scen {{number}}", "与场景 {{number}} 共用的画面"],
  "plan.imagePrompt": ["Zadání obrázku", "Billedprompt", "Bild-Prompt", "Περιγραφή εικόνας", "Image prompt", "Indicación de imagen", "Prompt d’image", "इमेज प्रॉम्प्ट", "Prompt immagine", "画像プロンプト", "Afbeeldingsprompt", "Bildeprompt", "Opis obrazu", "Descrição da imagem", "Bildprompt", "图片提示词"],
  "plan.libraryImage": ["Obrázek", "Billede", "Bild", "Εικόνα", "Image", "Imagen", "Image", "इमेज", "Immagine", "画像", "Afbeelding", "Bilde", "Obraz", "Imagem", "Bild", "图片"],
  "plan.chooseImage": ["Vyber obrázek", "Vælg et billede", "Bild wählen", "Επίλεξε εικόνα", "Choose an image", "Elige una imagen", "Choisis une image", "इमेज चुनें", "Scegli un’immagine", "画像を選択", "Kies een afbeelding", "Velg et bilde", "Wybierz obraz", "Escolha uma imagem", "Välj en bild", "选择图片"],
  "plan.narration": ["Komentář", "Speak", "Sprechertext", "Αφήγηση", "Narration", "Narración", "Narration", "वर्णन", "Narrazione", "ナレーション", "Voice-over", "Fortellerstemme", "Narracja", "Narração", "Berättarröst", "旁白"],
  "plan.transition": ["Přechod", "Overgang", "Übergang", "Μετάβαση", "Transition", "Transición", "Transition", "ट्रांज़िशन", "Transizione", "トランジション", "Overgang", "Overgang", "Przejście", "Transição", "Övergång", "转场"],
  "plan.kind.cut": ["Střih", "Klip", "Schnitt", "Κοπή", "Cut", "Corte", "Coupe", "कट", "Stacco", "カット", "Knip", "Klipp", "Cięcie", "Corte", "Klipp", "硬切"],
  "plan.kind.continue": ["Plynule", "Glidende", "Nahtlos", "Χωρίς ραφή", "Seamless", "Continuo", "Continu", "निर्बाध", "Continuo", "シームレス", "Naadloos", "Sømløs", "Płynnie", "Contínuo", "Sömlös", "无缝"],
  "plan.kind.bridge": ["Most", "Bro", "Brücke", "Γέφυρα", "Bridge", "Puente", "Pont", "ब्रिज", "Ponte", "ブリッジ", "Brug", "Bro", "Most", "Ponte", "Brygga", "桥接"],
  "plan.kind.blend": ["Prolínání", "Overtoning", "Überblendung", "Μίξη", "Blend", "Fundido", "Fondu", "ब्लेंड", "Dissolvenza", "ブレンド", "Overvloeien", "Overtoning", "Przenikanie", "Fusão", "Övertoning", "混合过渡"],
  "plan.blendType": ["Typ prolínání", "Overtoningstype", "Art der Überblendung", "Τύπος μίξης", "Blend type", "Tipo de fundido", "Type de fondu", "ब्लेंड का प्रकार", "Tipo di dissolvenza", "ブレンドの種類", "Soort overgang", "Overtoningstype", "Rodzaj przenikania", "Tipo de fusão", "Typ av övertoning", "过渡类型"],
  "plan.blendSeconds": ["Překryv (s)", "Overlap (s)", "Überlappung (s)", "Επικάλυψη (s)", "Overlap (s)", "Solapamiento (s)", "Chevauchement (s)", "ओवरलैप (से.)", "Sovrapposizione (s)", "重なり（秒）", "Overlap (s)", "Overlapp (s)", "Nakładanie (s)", "Sobreposição (s)", "Överlapp (s)", "重叠（秒）"],
  "plan.revise": ["Upravit scénu", "Revidér scene", "Szene überarbeiten", "Αναθεώρηση σκηνής", "Revise scene", "Revisar escena", "Réviser la scène", "दृश्य संशोधित करें", "Rivedi scena", "シーンを修正", "Scène herzien", "Revider scene", "Popraw scenę", "Rever cena", "Revidera scen", "修改场景"],
  "plan.reviseTitle": ["Upravit scénu {{number}}", "Revidér scene {{number}}", "Szene {{number}} überarbeiten", "Αναθεώρηση σκηνής {{number}}", "Revise scene {{number}}", "Revisar la escena {{number}}", "Réviser la scène {{number}}", "दृश्य {{number}} संशोधित करें", "Rivedi la scena {{number}}", "シーン {{number}} を修正", "Scène {{number}} herzien", "Revider scene {{number}}", "Popraw scenę {{number}}", "Rever a cena {{number}}", "Revidera scen {{number}}", "修改场景 {{number}}"],
  "plan.reviseCopy": ["Napiš agentovi, co má změnit. Změní jen tuto scénu a její přechody.", "Fortæl agenten, hvad der skal ændres. Den ændrer kun denne scene og dens overgange.", "Sag dem Agenten, was er ändern soll. Er ändert nur diese Szene und ihre Übergänge.", "Πες στον πράκτορα τι να αλλάξει. Αλλάζει μόνο αυτή τη σκηνή και τις μεταβάσεις της.", "Tell the agent what to change. It changes only this scene and its transitions.", "Dile al agente qué debe cambiar. Solo cambia esta escena y sus transiciones.", "Dis à l’agent ce qu’il doit changer. Il ne modifie que cette scène et ses transitions.", "एजेंट को बताएँ कि क्या बदलना है। वह केवल यह दृश्य और इसके ट्रांज़िशन बदलता है।", "Di’ all’agente cosa cambiare. Modifica solo questa scena e le sue transizioni.", "変更内容をエージェントに伝えてください。変更されるのはこのシーンとそのトランジションだけです。", "Vertel de agent wat er moet veranderen. Hij wijzigt alleen deze scène en haar overgangen.", "Fortell agenten hva som skal endres. Den endrer bare denne scenen og overgangene dens.", "Napisz agentowi, co ma zmienić. Zmieni tylko tę scenę i jej przejścia.", "Diga ao agente o que mudar. Ele muda apenas esta cena e as suas transições.", "Berätta för agenten vad som ska ändras. Den ändrar bara den här scenen och dess övergångar.", "告诉智能体要修改什么。它只会修改此场景及其转场。"],
  "plan.reviseNote": ["Co se má změnit?", "Hvad skal ændres?", "Was soll sich ändern?", "Τι πρέπει να αλλάξει;", "What should change?", "¿Qué debe cambiar?", "Que faut-il changer ?", "क्या बदलना चाहिए?", "Cosa deve cambiare?", "何を変更しますか？", "Wat moet er veranderen?", "Hva skal endres?", "Co ma się zmienić?", "O que deve mudar?", "Vad ska ändras?", "需要修改什么？"],
  "plan.moveUp": ["Posunout nahoru", "Flyt op", "Nach oben", "Μετακίνηση πάνω", "Move up", "Subir", "Monter", "ऊपर ले जाएँ", "Sposta su", "上へ移動", "Omhoog", "Flytt opp", "Przesuń w górę", "Mover para cima", "Flytta upp", "上移"],
  "plan.moveDown": ["Posunout dolů", "Flyt ned", "Nach unten", "Μετακίνηση κάτω", "Move down", "Bajar", "Descendre", "नीचे ले जाएँ", "Sposta giù", "下へ移動", "Omlaag", "Flytt ned", "Przesuń w dół", "Mover para baixo", "Flytta ned", "下移"],
  "plan.addScene": ["Přidat scénu", "Tilføj scene", "Szene hinzufügen", "Προσθήκη σκηνής", "Add scene", "Añadir escena", "Ajouter une scène", "दृश्य जोड़ें", "Aggiungi scena", "シーンを追加", "Scène toevoegen", "Legg til scene", "Dodaj scenę", "Adicionar cena", "Lägg till scen", "添加场景"],
  "plan.removeScene": ["Odebrat scénu", "Fjern scene", "Szene entfernen", "Αφαίρεση σκηνής", "Remove scene", "Quitar escena", "Retirer la scène", "दृश्य हटाएँ", "Rimuovi scena", "シーンを削除", "Scène verwijderen", "Fjern scene", "Usuń scenę", "Remover cena", "Ta bort scen", "移除场景"],
  "plan.noScenes": ["Tento plán zatím nemá žádné scény.", "Denne plan har endnu ingen scener.", "Dieser Plan hat noch keine Szenen.", "Αυτό το σχέδιο δεν έχει ακόμη σκηνές.", "This plan has no scenes yet.", "Este plan aún no tiene escenas.", "Ce plan n’a pas encore de scènes.", "इस योजना में अभी कोई दृश्य नहीं है।", "Questo piano non ha ancora scene.", "このプランにはまだシーンがありません。", "Dit plan heeft nog geen scènes.", "Denne planen har ingen scener ennå.", "Ten plan nie ma jeszcze scen.", "Este plano ainda não tem cenas.", "Den här planen har inga scener än.", "此规划还没有场景。"],
  "plan.total": ["{{total}} s z {{target}} s", "{{total}} s af {{target}} s", "{{total}} s von {{target}} s", "{{total}} s από {{target}} s", "{{total}} s of {{target}} s", "{{total}} s de {{target}} s", "{{total}} s sur {{target}} s", "{{target}} से. में से {{total}} से.", "{{total}} s di {{target}} s", "{{target}} 秒中 {{total}} 秒", "{{total}} s van {{target}} s", "{{total}} s av {{target}} s", "{{total}} s z {{target}} s", "{{total}} s de {{target}} s", "{{total}} s av {{target}} s", "{{total}} 秒 / {{target}} 秒"],
  "plan.scenes": ["Scény: {{count}}", "{{count}} scener", "{{count}} Szenen", "{{count}} σκηνές", "{{count}} scenes", "{{count}} escenas", "{{count}} scènes", "{{count}} दृश्य", "{{count}} scene", "{{count}} シーン", "{{count}} scènes", "{{count}} scener", "Sceny: {{count}}", "{{count}} cenas", "{{count}} scener", "{{count}} 个场景"],
  "plan.scenes_one": ["1 scéna", "1 scene", "1 Szene", "1 σκηνή", "1 scene", "1 escena", "1 scène", "1 दृश्य", "1 scena", "1 シーン", "1 scène", "1 scene", "1 scena", "1 cena", "1 scen", "1 个场景"],
  "plan.blocking": ["K opravě: {{count}}", "{{count}} problemer skal rettes", "{{count}} Probleme zu beheben", "{{count}} προβλήματα προς διόρθωση", "{{count}} problems to fix", "{{count}} problemas por corregir", "{{count}} problèmes à corriger", "ठीक करने के लिए {{count}} समस्याएँ", "{{count}} problemi da correggere", "修正が必要な問題：{{count}} 件", "{{count}} problemen op te lossen", "{{count}} problemer må rettes", "Do poprawy: {{count}}", "{{count}} problemas por corrigir", "{{count}} problem att åtgärda", "{{count}} 个问题需要修正"],
  "plan.blocking_one": ["1 problém k opravě", "1 problem skal rettes", "1 Problem zu beheben", "1 πρόβλημα προς διόρθωση", "1 problem to fix", "1 problema por corregir", "1 problème à corriger", "ठीक करने के लिए 1 समस्या", "1 problema da correggere", "修正が必要な問題：1 件", "1 probleem op te lossen", "1 problem må rettes", "1 problem do poprawy", "1 problema por corrigir", "1 problem att åtgärda", "1 个问题需要修正"],
  "plan.noBlocking": ["Bez problémů", "Ingen problemer", "Keine Probleme", "Κανένα πρόβλημα", "No problems", "Sin problemas", "Aucun problème", "कोई समस्या नहीं", "Nessun problema", "問題なし", "Geen problemen", "Ingen problemer", "Brak problemów", "Sem problemas", "Inga problem", "没有问题"],
  "plan.changedElsewhere": ["Plán byl změněn jinde. Zobrazuje se nejnovější verze.", "Planen blev ændret et andet sted. Den nyeste version vises.", "Der Plan wurde an anderer Stelle geändert. Die neueste Fassung wird angezeigt.", "Το σχέδιο άλλαξε από αλλού. Εμφανίζεται η πιο πρόσφατη έκδοση.", "The plan was changed elsewhere. The latest version is shown.", "El plan se cambió en otro lugar. Se muestra la versión más reciente.", "Le plan a été modifié ailleurs. La dernière version est affichée.", "योजना कहीं और बदली गई। नवीनतम संस्करण दिखाया गया है।", "Il piano è stato modificato altrove. Viene mostrata la versione più recente.", "プランが別の場所で変更されました。最新の版を表示しています。", "Het plan is elders gewijzigd. De nieuwste versie wordt getoond.", "Planen ble endret et annet sted. Den nyeste versjonen vises.", "Plan został zmieniony gdzie indziej. Wyświetlana jest najnowsza wersja.", "O plano foi alterado noutro local. É mostrada a versão mais recente.", "Planen ändrades någon annanstans. Den senaste versionen visas.", "规划已在别处被修改。现在显示的是最新版本。"],
  "plan.agentBusy": ["Agent na tomto plánu ještě pracuje.", "Agenten arbejder stadig på denne plan.", "Der Agent arbeitet noch an diesem Plan.", "Ο πράκτορας δουλεύει ακόμη σε αυτό το σχέδιο.", "The agent is still working on this plan.", "El agente aún está trabajando en este plan.", "L’agent travaille encore sur ce plan.", "एजेंट अभी भी इस योजना पर काम कर रहा है।", "L’agente sta ancora lavorando a questo piano.", "エージェントはまだこのプランを作業中です。", "De agent werkt nog aan dit plan.", "Agenten jobber fortsatt med denne planen.", "Agent nadal pracuje nad tym planem.", "O agente ainda está a trabalhar neste plano.", "Agenten arbetar fortfarande med den här planen.", "智能体仍在处理此规划。"],
  "plan.errStart": ["Plánování se nepodařilo spustit.", "Planlægningen kunne ikke startes.", "Die Planung konnte nicht gestartet werden.", "Δεν ήταν δυνατή η έναρξη του σχεδιασμού.", "Planning could not be started.", "No se ha podido iniciar la planificación.", "La planification n’a pas pu démarrer.", "योजना बनाना शुरू नहीं हो सका।", "Impossibile avviare la pianificazione.", "計画を開始できませんでした。", "Het plannen kon niet worden gestart.", "Planleggingen kunne ikke startes.", "Nie udało się rozpocząć planowania.", "Não foi possível iniciar o planeamento.", "Planeringen kunde inte startas.", "无法开始规划。"],
  "plan.errDailyLimit": ["Dnešní limit plánování je vyčerpán. Zkus to zítra.", "Dagens grænse for planlægning er nået. Prøv igen i morgen.", "Das heutige Planungslimit ist erreicht. Versuche es morgen wieder.", "Συμπληρώθηκε το σημερινό όριο σχεδιασμών. Δοκίμασε ξανά αύριο.", "Today's planning limit is reached. Try again tomorrow.", "Se ha alcanzado el límite de planificaciones de hoy. Inténtalo mañana.", "La limite de planifications du jour est atteinte. Réessaie demain.", "आज की योजना सीमा पूरी हो गई है। कल फिर कोशिश करें।", "Il limite di pianificazioni di oggi è stato raggiunto. Riprova domani.", "本日の計画回数の上限に達しました。明日もう一度お試しください。", "De planlimiet van vandaag is bereikt. Probeer het morgen opnieuw.", "Dagens grense for planlegging er nådd. Prøv igjen i morgen.", "Dzisiejszy limit planowania został osiągnięty. Spróbuj jutro.", "O limite de planeamentos de hoje foi atingido. Tente novamente amanhã.", "Dagens gräns för planering är nådd. Försök igen i morgon.", "今天的规划次数已达上限。请明天再试。"],
  "plan.errAgentUnavailable": ["Pro plánování není k dispozici žádný jazykový model.", "Der er ingen sprogmodel til rådighed til planlægning.", "Für die Planung ist kein Sprachmodell verfügbar.", "Δεν υπάρχει διαθέσιμο γλωσσικό μοντέλο για σχεδιασμό.", "No language model is available for planning.", "No hay ningún modelo de lenguaje disponible para planificar.", "Aucun modèle de langage n’est disponible pour la planification.", "योजना के लिए कोई भाषा मॉडल उपलब्ध नहीं है।", "Nessun modello linguistico disponibile per la pianificazione.", "計画に使える言語モデルがありません。", "Er is geen taalmodel beschikbaar om te plannen.", "Ingen språkmodell er tilgjengelig for planlegging.", "Brak dostępnego modelu językowego do planowania.", "Não há nenhum modelo de linguagem disponível para o planeamento.", "Ingen språkmodell är tillgänglig för planering.", "没有可用于规划的语言模型。"],
  "plan.issue.no_scenes": ["Plán nemá žádné scény.", "Planen har ingen scener.", "Der Plan hat keine Szenen.", "Το σχέδιο δεν έχει σκηνές.", "The plan has no scenes.", "El plan no tiene escenas.", "Le plan n’a aucune scène.", "योजना में कोई दृश्य नहीं है।", "Il piano non ha scene.", "プランにシーンがありません。", "Het plan heeft geen scènes.", "Planen har ingen scener.", "Plan nie ma scen.", "O plano não tem cenas.", "Planen har inga scener.", "规划中没有场景。"],
  "plan.issue.too_many_scenes": ["Plán může mít nejvýše 12 scén.", "En plan kan højst have 12 scener.", "Ein Plan kann höchstens 12 Szenen haben.", "Ένα σχέδιο μπορεί να έχει έως 12 σκηνές.", "A plan can have at most 12 scenes.", "Un plan puede tener como máximo 12 escenas.", "Un plan peut contenir au maximum 12 scènes.", "एक योजना में अधिकतम 12 दृश्य हो सकते हैं।", "Un piano può avere al massimo 12 scene.", "プランのシーンは最大 12 個です。", "Een plan kan hooguit 12 scènes hebben.", "En plan kan ha høyst 12 scener.", "Plan może mieć najwyżej 12 scen.", "Um plano pode ter no máximo 12 cenas.", "En plan kan ha högst 12 scener.", "一个规划最多可有 12 个场景。"],
  "plan.issue.target_seconds_invalid": ["Cílová délka musí být mezi 5 a 120 sekundami.", "Den ønskede længde skal være mellem 5 og 120 sekunder.", "Die Ziellänge muss zwischen 5 und 120 Sekunden liegen.", "Η επιθυμητή διάρκεια πρέπει να είναι από 5 έως 120 δευτερόλεπτα.", "The target length must be between 5 and 120 seconds.", "La duración objetivo debe estar entre 5 y 120 segundos.", "La durée cible doit être comprise entre 5 et 120 secondes.", "लक्षित अवधि 5 से 120 सेकंड के बीच होनी चाहिए।", "La durata obiettivo deve essere tra 5 e 120 secondi.", "目標の長さは 5〜120 秒にしてください。", "De beoogde lengte moet tussen 5 en 120 seconden liggen.", "Ønsket lengde må være mellom 5 og 120 sekunder.", "Docelowa długość musi wynosić od 5 do 120 sekund.", "A duração pretendida tem de estar entre 5 e 120 segundos.", "Önskad längd måste vara mellan 5 och 120 sekunder.", "目标时长必须在 5 到 120 秒之间。"],
  "plan.issue.total_length_mismatch": ["Celková délka se od cíle liší o více než 15 %.", "Den samlede længde afviger mere end 15 % fra målet.", "Die Gesamtlänge weicht um mehr als 15 % vom Ziel ab.", "Η συνολική διάρκεια απέχει πάνω από 15 % από τον στόχο.", "The total length is more than 15 % away from the target.", "La duración total se aleja más de un 15 % del objetivo.", "La durée totale s’écarte de plus de 15 % de la cible.", "कुल अवधि लक्ष्य से 15 % से अधिक अलग है।", "La durata totale si discosta di oltre il 15 % dall’obiettivo.", "合計の長さが目標から 15 % 以上ずれています。", "De totale lengte wijkt meer dan 15 % af van het doel.", "Den samlede lengden avviker mer enn 15 % fra målet.", "Łączna długość różni się od docelowej o ponad 15 %.", "A duração total afasta-se mais de 15 % do objetivo.", "Den totala längden avviker mer än 15 % från målet.", "总时长与目标相差超过 15%。"],
  "plan.issue.total_length_too_long": ["Celková délka přesahuje 120 sekund.", "Den samlede længde er over 120 sekunder.", "Die Gesamtlänge liegt über 120 Sekunden.", "Η συνολική διάρκεια ξεπερνά τα 120 δευτερόλεπτα.", "The total length is over 120 seconds.", "La duración total supera los 120 segundos.", "La durée totale dépasse 120 secondes.", "कुल अवधि 120 सेकंड से अधिक है।", "La durata totale supera i 120 secondi.", "合計の長さが 120 秒を超えています。", "De totale lengte is meer dan 120 seconden.", "Den samlede lengden er over 120 sekunder.", "Łączna długość przekracza 120 sekund.", "A duração total ultrapassa 120 segundos.", "Den totala längden är över 120 sekunder.", "总时长超过 120 秒。"],
  "plan.issue.idea_too_long": ["Nápad je delší než 4000 znaků.", "Idéen er længere end 4000 tegn.", "Die Idee ist länger als 4000 Zeichen.", "Η ιδέα ξεπερνά τους 4000 χαρακτήρες.", "The idea is longer than 4000 characters.", "La idea supera los 4000 caracteres.", "L’idée dépasse 4000 caractères.", "आइडिया 4000 अक्षरों से लंबा है।", "L’idea supera i 4000 caratteri.", "アイデアが 4000 文字を超えています。", "Het idee is langer dan 4000 tekens.", "Ideen er lengre enn 4000 tegn.", "Pomysł ma ponad 4000 znaków.", "A ideia tem mais de 4000 caracteres.", "Idén är längre än 4000 tecken.", "创意超过 4000 个字符。"],
  "plan.issue.style_too_long": ["Styl je delší než 2000 znaků.", "Stilen er længere end 2000 tegn.", "Der Stil ist länger als 2000 Zeichen.", "Το ύφος ξεπερνά τους 2000 χαρακτήρες.", "The style is longer than 2000 characters.", "El estilo supera los 2000 caracteres.", "Le style dépasse 2000 caractères.", "शैली 2000 अक्षरों से लंबी है।", "Lo stile supera i 2000 caratteri.", "スタイルが 2000 文字を超えています。", "De stijl is langer dan 2000 tekens.", "Stilen er lengre enn 2000 tegn.", "Styl ma ponad 2000 znaków.", "O estilo tem mais de 2000 caracteres.", "Stilen är längre än 2000 tecken.", "风格超过 2000 个字符。"],
  "plan.issue.too_many_characters": ["Plán může mít nejvýše 6 postav.", "En plan kan højst have 6 personer.", "Ein Plan kann höchstens 6 Figuren haben.", "Ένα σχέδιο μπορεί να έχει έως 6 χαρακτήρες.", "A plan can have at most 6 characters.", "Un plan puede tener como máximo 6 personajes.", "Un plan peut contenir au maximum 6 personnages.", "एक योजना में अधिकतम 6 पात्र हो सकते हैं।", "Un piano può avere al massimo 6 personaggi.", "登場人物は最大 6 人です。", "Een plan kan hooguit 6 personages hebben.", "En plan kan ha høyst 6 personer.", "Plan może mieć najwyżej 6 postaci.", "Um plano pode ter no máximo 6 personagens.", "En plan kan ha högst 6 karaktärer.", "一个规划最多可有 6 个角色。"],
  "plan.issue.character_invalid": ["Postava nemá jméno, má jméno použité dvakrát nebo příliš dlouhý text.", "En person mangler navn, har et navn, der bruges to gange, eller en for lang tekst.", "Eine Figur hat keinen Namen, einen doppelten Namen oder einen zu langen Text.", "Ένας χαρακτήρας δεν έχει όνομα, έχει όνομα που χρησιμοποιείται δύο φορές ή πολύ μεγάλο κείμενο.", "A character has no name, a name used twice or a text that is too long.", "Un personaje no tiene nombre, repite un nombre o tiene un texto demasiado largo.", "Un personnage n’a pas de nom, porte un nom déjà utilisé ou a un texte trop long.", "किसी पात्र का नाम नहीं है, उसका नाम दो बार इस्तेमाल हुआ है या कोई टेक्स्ट बहुत लंबा है।", "Un personaggio non ha nome, ha un nome usato due volte o un testo troppo lungo.", "名前がない、名前が重複している、またはテキストが長すぎる登場人物があります。", "Een personage heeft geen naam, een dubbele naam of een te lange tekst.", "En person mangler navn, har et navn som brukes to ganger, eller en for lang tekst.", "Postać nie ma imienia, ma powtórzone imię lub za długi tekst.", "Uma personagem não tem nome, tem um nome repetido ou um texto demasiado longo.", "En karaktär saknar namn, har ett namn som används två gånger eller en för lång text.", "有角色缺少名称、名称重复或文字过长。"],
  "plan.issue.character_reference_invalid": ["Referenční obrázek postavy není v tomto projektu.", "Referencebilledet for en person findes ikke i dette projekt.", "Das Referenzbild einer Figur gehört nicht zu diesem Projekt.", "Η εικόνα αναφοράς ενός χαρακτήρα δεν υπάρχει σε αυτό το έργο.", "The reference image of a character is not in this project.", "La imagen de referencia de un personaje no está en este proyecto.", "L’image de référence d’un personnage n’est pas dans ce projet.", "किसी पात्र की संदर्भ इमेज इस प्रोजेक्ट में नहीं है।", "L’immagine di riferimento di un personaggio non è in questo progetto.", "登場人物の参照画像がこのプロジェクトにありません。", "De referentieafbeelding van een personage staat niet in dit project.", "Referansebildet til en person finnes ikke i dette prosjektet.", "Obraz referencyjny postaci nie należy do tego projektu.", "A imagem de referência de uma personagem não está neste projeto.", "Referensbilden för en karaktär finns inte i det här projektet.", "某个角色的参考图片不在此项目中。"],
  "plan.issue.title_missing": ["Scéna potřebuje název.", "Scenen skal have en titel.", "Die Szene braucht einen Titel.", "Η σκηνή χρειάζεται τίτλο.", "The scene needs a title.", "La escena necesita un título.", "La scène a besoin d’un titre.", "दृश्य का शीर्षक ज़रूरी है।", "La scena ha bisogno di un titolo.", "シーンにタイトルが必要です。", "De scène heeft een titel nodig.", "Scenen trenger en tittel.", "Scena potrzebuje tytułu.", "A cena precisa de um título.", "Scenen behöver en titel.", "场景需要标题。"],
  "plan.issue.title_too_long": ["Název je delší než 120 znaků.", "Titlen er længere end 120 tegn.", "Der Titel ist länger als 120 Zeichen.", "Ο τίτλος ξεπερνά τους 120 χαρακτήρες.", "The title is longer than 120 characters.", "El título supera los 120 caracteres.", "Le titre dépasse 120 caractères.", "शीर्षक 120 अक्षरों से लंबा है।", "Il titolo supera i 120 caratteri.", "タイトルが 120 文字を超えています。", "De titel is langer dan 120 tekens.", "Tittelen er lengre enn 120 tegn.", "Tytuł ma ponad 120 znaków.", "O título tem mais de 120 caracteres.", "Titeln är längre än 120 tecken.", "标题超过 120 个字符。"],
  "plan.issue.prompt_missing": ["Zadání videa je prázdné.", "Videoprompten er tom.", "Der Video-Prompt ist leer.", "Η περιγραφή βίντεο είναι κενή.", "The video prompt is empty.", "La indicación de vídeo está vacía.", "Le prompt vidéo est vide.", "वीडियो प्रॉम्प्ट खाली है।", "Il prompt video è vuoto.", "動画プロンプトが空です。", "De videoprompt is leeg.", "Videoprompten er tom.", "Opis wideo jest pusty.", "A descrição do vídeo está vazia.", "Videoprompten är tom.", "视频提示词为空。"],
  "plan.issue.prompt_too_long": ["Zadání je příliš dlouhé.", "Prompten er for lang.", "Der Prompt ist zu lang.", "Η περιγραφή είναι πολύ μεγάλη.", "This prompt is too long.", "La indicación es demasiado larga.", "Le prompt est trop long.", "प्रॉम्प्ट बहुत लंबा है।", "Il prompt è troppo lungo.", "プロンプトが長すぎます。", "De prompt is te lang.", "Prompten er for lang.", "Opis jest za długi.", "A descrição é demasiado longa.", "Prompten är för lång.", "提示词太长。"],
  "plan.issue.negative_prompt_too_long": ["Negativní zadání je příliš dlouhé.", "Den negative prompt er for lang.", "Der Negativ-Prompt ist zu lang.", "Η αρνητική περιγραφή είναι πολύ μεγάλη.", "The negative prompt is too long.", "La indicación negativa es demasiado larga.", "Le prompt négatif est trop long.", "नकारात्मक प्रॉम्प्ट बहुत लंबा है।", "Il prompt negativo è troppo lungo.", "ネガティブプロンプトが長すぎます。", "De negatieve prompt is te lang.", "Den negative prompten er for lang.", "Opis negatywny jest za długi.", "A descrição negativa é demasiado longa.", "Den negativa prompten är för lång.", "反向提示词太长。"],
  "plan.issue.duration_unsupported": ["Poskytovatel nedokáže vygenerovat tuto délku.", "Udbyderen kan ikke generere denne længde.", "Der Anbieter kann diese Länge nicht erzeugen.", "Ο πάροχος δεν μπορεί να δημιουργήσει αυτή τη διάρκεια.", "The provider cannot generate this length.", "El proveedor no puede generar esta duración.", "Le fournisseur ne peut pas générer cette durée.", "प्रदाता यह अवधि जनरेट नहीं कर सकता।", "Il fornitore non può generare questa durata.", "プロバイダーはこの長さを生成できません。", "De provider kan deze lengte niet genereren.", "Leverandøren kan ikke generere denne lengden.", "Dostawca nie może wygenerować takiej długości.", "O fornecedor não consegue gerar esta duração.", "Leverantören kan inte generera den här längden.", "提供商无法生成此时长。"],
  "plan.issue.use_seconds_invalid": ["Použitá délka musí být větší než 0 a nejvýše rovna vygenerované délce.", "Den brugte længde skal være over 0 og højst den genererede længde.", "Die verwendete Länge muss über 0 liegen und darf die erzeugte Länge nicht überschreiten.", "Η διάρκεια χρήσης πρέπει να είναι πάνω από 0 και έως τη διάρκεια που δημιουργείται.", "The used length must be above 0 and at most the generated length.", "La duración usada debe ser mayor que 0 y como máximo la generada.", "La durée utilisée doit être supérieure à 0 et au plus égale à la durée générée.", "उपयोग की गई अवधि 0 से अधिक और जनरेट की गई अवधि से अधिक नहीं होनी चाहिए।", "La durata usata deve essere maggiore di 0 e al massimo pari a quella generata.", "使用する長さは 0 より大きく、生成する長さ以下にしてください。", "De gebruikte lengte moet groter dan 0 zijn en mag de gegenereerde lengte niet overschrijden.", "Den brukte lengden må være over 0 og høyst den genererte lengden.", "Użyta długość musi być większa od 0 i nie większa niż wygenerowana.", "A duração usada tem de ser superior a 0 e no máximo igual à gerada.", "Den använda längden måste vara över 0 och högst den genererade längden.", "使用时长必须大于 0，且不超过生成时长。"],
  "plan.issue.transition_unknown": ["Neznámý přechod.", "Ukendt overgang.", "Unbekannter Übergang.", "Άγνωστη μετάβαση.", "Unknown transition.", "Transición desconocida.", "Transition inconnue.", "अज्ञात ट्रांज़िशन।", "Transizione sconosciuta.", "不明なトランジションです。", "Onbekende overgang.", "Ukjent overgang.", "Nieznane przejście.", "Transição desconhecida.", "Okänd övergång.", "未知的转场。"],
  "plan.issue.transition_unsupported": ["Poskytovatel tento přechod nepodporuje.", "Udbyderen understøtter ikke denne overgang.", "Der Anbieter unterstützt diesen Übergang nicht.", "Ο πάροχος δεν υποστηρίζει αυτή τη μετάβαση.", "The provider does not support this transition.", "El proveedor no admite esta transición.", "Le fournisseur ne prend pas en charge cette transition.", "प्रदाता इस ट्रांज़िशन का समर्थन नहीं करता।", "Il fornitore non supporta questa transizione.", "プロバイダーはこのトランジションに対応していません。", "De provider ondersteunt deze overgang niet.", "Leverandøren støtter ikke denne overgangen.", "Dostawca nie obsługuje tego przejścia.", "O fornecedor não suporta esta transição.", "Leverantören stöder inte den här övergången.", "提供商不支持此转场。"],
  "plan.issue.first_scene_transition": ["První scéna musí začínat střihem.", "Den første scene skal starte med et klip.", "Die erste Szene muss mit einem Schnitt beginnen.", "Η πρώτη σκηνή πρέπει να ξεκινά με κοπή.", "The first scene must start with a cut.", "La primera escena debe empezar con un corte.", "La première scène doit commencer par une coupe.", "पहला दृश्य कट से शुरू होना चाहिए।", "La prima scena deve iniziare con uno stacco.", "最初のシーンはカットで始めてください。", "De eerste scène moet met een knip beginnen.", "Den første scenen må starte med et klipp.", "Pierwsza scena musi zaczynać się cięciem.", "A primeira cena tem de começar com um corte.", "Den första scenen måste börja med ett klipp.", "第一个场景必须以硬切开始。"],
  "plan.issue.chain_too_long": ["Plynule lze za sebou spojit nejvýše 4 scény.", "Højst 4 scener kan sættes glidende sammen i træk.", "Höchstens 4 Szenen können nahtlos hintereinander verbunden werden.", "Έως 4 σκηνές μπορούν να ενωθούν χωρίς ραφή στη σειρά.", "At most 4 scenes can be joined seamlessly in a row.", "Como máximo 4 escenas seguidas pueden unirse de forma continua.", "Au maximum 4 scènes peuvent s’enchaîner en continu.", "लगातार अधिकतम 4 दृश्य निर्बाध रूप से जोड़े जा सकते हैं।", "Al massimo 4 scene di fila possono essere unite senza stacchi.", "シームレスにつなげられるのは連続 4 シーンまでです。", "Hooguit 4 scènes kunnen naadloos achter elkaar worden verbonden.", "Høyst 4 scener kan settes sømløst sammen etter hverandre.", "Płynnie można połączyć najwyżej 4 sceny z rzędu.", "No máximo 4 cenas seguidas podem ser unidas de forma contínua.", "Högst 4 scener i rad kan fogas ihop sömlöst.", "最多只能连续无缝衔接 4 个场景。"],
  "plan.issue.blend_type_unknown": ["Neznámý typ prolínání.", "Ukendt overtoningstype.", "Unbekannte Art der Überblendung.", "Άγνωστος τύπος μίξης.", "Unknown blend type.", "Tipo de fundido desconocido.", "Type de fondu inconnu.", "अज्ञात ब्लेंड प्रकार।", "Tipo di dissolvenza sconosciuto.", "不明なブレンドの種類です。", "Onbekende soort overgang.", "Ukjent overtoningstype.", "Nieznany rodzaj przenikania.", "Tipo de fusão desconhecido.", "Okänd typ av övertoning.", "未知的过渡类型。"],
  "plan.issue.blend_type_unavailable": ["Tento typ prolínání není na tomto serveru k dispozici.", "Denne overtoningstype er ikke tilgængelig på denne server.", "Diese Art der Überblendung ist auf diesem Server nicht verfügbar.", "Αυτός ο τύπος μίξης δεν είναι διαθέσιμος σε αυτόν τον διακομιστή.", "This blend type is not available on this server.", "Este tipo de fundido no está disponible en este servidor.", "Ce type de fondu n’est pas disponible sur ce serveur.", "यह ब्लेंड प्रकार इस सर्वर पर उपलब्ध नहीं है।", "Questo tipo di dissolvenza non è disponibile su questo server.", "このブレンドの種類はこのサーバーでは使えません。", "Deze soort overgang is niet beschikbaar op deze server.", "Denne overtoningstypen er ikke tilgjengelig på denne serveren.", "Ten rodzaj przenikania jest niedostępny na tym serwerze.", "Este tipo de fusão não está disponível neste servidor.", "Den här typen av övertoning finns inte på den här servern.", "此服务器不提供该过渡类型。"],
  "plan.issue.blend_overlap_invalid": ["Překryv musí být alespoň 0,5 s a vejít se do obou scén.", "Overlappet skal være mindst 0,5 s og passe til begge scener.", "Die Überlappung muss mindestens 0,5 s betragen und in beide Szenen passen.", "Η επικάλυψη πρέπει να είναι τουλάχιστον 0,5 s και να χωρά και στις δύο σκηνές.", "The overlap must be at least 0.5 s and fit both scenes.", "El solapamiento debe ser de al menos 0,5 s y caber en ambas escenas.", "Le chevauchement doit durer au moins 0,5 s et tenir dans les deux scènes.", "ओवरलैप कम से कम 0.5 से. होना चाहिए और दोनों दृश्यों में समाना चाहिए।", "La sovrapposizione deve essere di almeno 0,5 s e rientrare in entrambe le scene.", "重なりは 0.5 秒以上で、両方のシーンに収まる必要があります。", "De overlap moet minstens 0,5 s zijn en in beide scènes passen.", "Overlappen må være minst 0,5 s og passe i begge scenene.", "Nakładanie musi trwać co najmniej 0,5 s i mieścić się w obu scenach.", "A sobreposição tem de ter pelo menos 0,5 s e caber nas duas cenas.", "Överlappet måste vara minst 0,5 s och rymmas i båda scenerna.", "重叠至少需要 0.5 秒，并且要容纳在两个场景内。"],
  "plan.issue.keyframe_source_invalid": ["Tento zdroj úvodního obrázku ke scéně nesedí.", "Denne kilde til startbilledet passer ikke til scenen.", "Diese Quelle für das Startbild passt nicht zur Szene.", "Αυτή η πηγή εικόνας έναρξης δεν ταιριάζει στη σκηνή.", "This start image source does not fit the scene.", "Este origen de la imagen inicial no encaja con la escena.", "Cette source d’image de départ ne convient pas à la scène.", "शुरुआती इमेज का यह स्रोत इस दृश्य के अनुकूल नहीं है।", "Questa origine dell’immagine iniziale non è adatta alla scena.", "この開始画像の取得方法はシーンに合いません。", "Deze bron voor de startafbeelding past niet bij de scène.", "Denne kilden til startbildet passer ikke til scenen.", "To źródło obrazu startowego nie pasuje do sceny.", "Esta origem da imagem inicial não se adequa à cena.", "Den här källan för startbilden passar inte scenen.", "此起始图片来源不适合该场景。"],
  "plan.issue.keyframe_prompt_missing": ["Zadání úvodního obrázku je prázdné.", "Billedprompten til startbilledet er tom.", "Der Bild-Prompt für das Startbild ist leer.", "Η περιγραφή της εικόνας έναρξης είναι κενή.", "The image prompt for the start image is empty.", "La indicación de la imagen inicial está vacía.", "Le prompt de l’image de départ est vide.", "शुरुआती इमेज का प्रॉम्प्ट खाली है।", "Il prompt dell’immagine iniziale è vuoto.", "開始画像の画像プロンプトが空です。", "De prompt voor de startafbeelding is leeg.", "Bildeprompten for startbildet er tom.", "Opis obrazu startowego jest pusty.", "A descrição da imagem inicial está vazia.", "Bildprompten för startbilden är tom.", "起始图片的提示词为空。"],
  "plan.issue.keyframe_asset_invalid": ["Vyber obrázek z knihovny tohoto projektu.", "Vælg et billede fra dette projekts bibliotek.", "Wähle ein Bild aus der Mediathek dieses Projekts.", "Επίλεξε μια εικόνα από τη βιβλιοθήκη αυτού του έργου.", "Choose an image from the library of this project.", "Elige una imagen de la biblioteca de este proyecto.", "Choisis une image de la médiathèque de ce projet.", "इस प्रोजेक्ट की लाइब्रेरी से कोई इमेज चुनें।", "Scegli un’immagine dalla libreria di questo progetto.", "このプロジェクトのライブラリから画像を選んでください。", "Kies een afbeelding uit de bibliotheek van dit project.", "Velg et bilde fra biblioteket til dette prosjektet.", "Wybierz obraz z biblioteki tego projektu.", "Escolha uma imagem da biblioteca deste projeto.", "Välj en bild från det här projektets bibliotek.", "请从此项目的媒体库中选择图片。"],
  "plan.issue.narration_unavailable": ["Komentář potřebuje převod textu na řeč, který není nastavený.", "Speak kræver tekst til tale, som ikke er sat op.", "Sprechertext braucht Sprachausgabe, die nicht eingerichtet ist.", "Η αφήγηση χρειάζεται μετατροπή κειμένου σε ομιλία, που δεν έχει ρυθμιστεί.", "Narration needs text to speech, which is not set up.", "La narración necesita texto a voz, que no está configurado.", "La narration nécessite la synthèse vocale, qui n’est pas configurée.", "वर्णन के लिए टेक्स्ट-टू-स्पीच चाहिए, जो सेट अप नहीं है।", "La narrazione richiede la sintesi vocale, che non è configurata.", "ナレーションには音声合成が必要ですが、設定されていません。", "Voice-over heeft tekst-naar-spraak nodig, en dat is niet ingesteld.", "Fortellerstemme krever tekst til tale, som ikke er satt opp.", "Narracja wymaga zamiany tekstu na mowę, która nie jest skonfigurowana.", "A narração precisa de texto para voz, que não está configurado.", "Berättarröst kräver text till tal, som inte är konfigurerat.", "旁白需要文本转语音，但尚未设置。"],
  "plan.issue.narration_too_long": ["Komentář je pro scénu příliš dlouhý.", "Speaken er for lang til scenen.", "Der Sprechertext ist für die Szene zu lang.", "Η αφήγηση είναι πολύ μεγάλη για τη σκηνή.", "The narration is too long for the scene.", "La narración es demasiado larga para la escena.", "La narration est trop longue pour la scène.", "दृश्य के लिए वर्णन बहुत लंबा है।", "La narrazione è troppo lunga per la scena.", "ナレーションがシーンに対して長すぎます。", "De voice-over is te lang voor de scène.", "Fortellerstemmen er for lang for scenen.", "Narracja jest za długa dla tej sceny.", "A narração é demasiado longa para a cena.", "Berättarrösten är för lång för scenen.", "旁白对该场景来说太长。"],
  "plan.issue.music_unavailable": ["Hudba potřebuje generování hudby, které není nastavené.", "Musik kræver musikgenerering, som ikke er sat op.", "Musik braucht die Musikgenerierung, die nicht eingerichtet ist.", "Η μουσική χρειάζεται δημιουργία μουσικής, που δεν έχει ρυθμιστεί.", "Music needs music generation, which is not set up.", "La música necesita generación de música, que no está configurada.", "La musique nécessite la génération musicale, qui n’est pas configurée.", "संगीत के लिए म्यूज़िक जनरेशन चाहिए, जो सेट अप नहीं है।", "La musica richiede la generazione musicale, che non è configurata.", "音楽には音楽生成が必要ですが、設定されていません。", "Muziek heeft muziekgeneratie nodig, en dat is niet ingesteld.", "Musikk krever musikkgenerering, som ikke er satt opp.", "Muzyka wymaga generowania muzyki, które nie jest skonfigurowane.", "A música precisa de geração de música, que não está configurada.", "Musik kräver musikgenerering, som inte är konfigurerad.", "音乐需要音乐生成，但尚未设置。"],
  "plan.issue.clip_audio_invalid": ["Neznámé nastavení zvuku klipů.", "Ukendt indstilling for klippenes lyd.", "Unbekannte Einstellung für den Clip-Ton.", "Άγνωστη ρύθμιση ήχου κλιπ.", "Unknown clip sound setting.", "Ajuste de sonido de los clips desconocido.", "Réglage du son des clips inconnu.", "क्लिप ध्वनि की अज्ञात सेटिंग।", "Impostazione dell’audio delle clip sconosciuta.", "不明なクリップ音声の設定です。", "Onbekende instelling voor clipgeluid.", "Ukjent innstilling for klippenes lyd.", "Nieznane ustawienie dźwięku klipów.", "Definição de som dos clipes desconhecida.", "Okänd inställning för klippens ljud.", "未知的片段声音设置。"],
  "plan.issue.plan_unsupported_provider": ["Současný poskytovatel videa nepodporuje režim plánu.", "Den nuværende videoudbyder understøtter ikke plantilstand.", "Der aktuelle Videoanbieter unterstützt den Planmodus nicht.", "Ο τρέχων πάροχος βίντεο δεν υποστηρίζει τη λειτουργία σχεδίου.", "The current video provider does not support plan mode.", "El proveedor de vídeo actual no admite el modo plan.", "Le fournisseur vidéo actuel ne prend pas en charge le mode plan.", "मौजूदा वीडियो प्रदाता योजना मोड का समर्थन नहीं करता।", "Il fornitore video attuale non supporta la modalità piano.", "現在の動画プロバイダーはプランモードに対応していません。", "De huidige videoprovider ondersteunt de planmodus niet.", "Den nåværende videoleverandøren støtter ikke planmodus.", "Obecny dostawca wideo nie obsługuje trybu planu.", "O fornecedor de vídeo atual não suporta o modo de plano.", "Den nuvarande videoleverantören stöder inte planläget.", "当前视频提供商不支持规划模式。"],
  "plan.issue.profile_changed": ["Nastavení videa se od plánování změnilo. Zkontroluj scény znovu.", "Videoindstillingerne er ændret siden planlægningen. Tjek scenerne igen.", "Die Videoeinstellungen haben sich seit der Planung geändert. Prüfe die Szenen erneut.", "Οι ρυθμίσεις βίντεο άλλαξαν από τον σχεδιασμό. Έλεγξε ξανά τις σκηνές.", "The video settings changed since planning. Check the scenes again.", "Los ajustes de vídeo cambiaron desde la planificación. Revisa de nuevo las escenas.", "Les réglages vidéo ont changé depuis la planification. Vérifie à nouveau les scènes.", "योजना बनने के बाद वीडियो सेटिंग्स बदल गई हैं। दृश्यों को फिर से जाँचें।", "Le impostazioni video sono cambiate dalla pianificazione. Ricontrolla le scene.", "計画後に動画の設定が変わりました。シーンをもう一度確認してください。", "De video-instellingen zijn sinds het plannen gewijzigd. Controleer de scènes opnieuw.", "Videoinnstillingene er endret siden planleggingen. Sjekk scenene på nytt.", "Ustawienia wideo zmieniły się od czasu planowania. Sprawdź sceny ponownie.", "As definições de vídeo mudaram desde o planeamento. Verifique novamente as cenas.", "Videoinställningarna har ändrats sedan planeringen. Kontrollera scenerna igen.", "规划后视频设置已更改。请重新检查各个场景。"]
};

const rows = Object.entries(T);
for (const [key, values] of rows) {
  if (!Array.isArray(values) || values.length !== LANGS.length || values.some(value => typeof value !== 'string' || !value.trim())) throw new Error('bad row ' + key);
}
if (rows.length !== 122) throw new Error('expected 122 keys, have ' + rows.length);
LANGS.forEach((lang, index) => {
  const file = join(dir, lang + '.json');
  const text = readFileSync(file, 'utf8');
  if (text.includes(CR)) throw new Error(file + ' has CRLF line endings');
  const existing = JSON.parse(text);
  const lines = text.split(NL);
  let anchor = -1;
  lines.forEach((line, i) => { if (line.startsWith('  "videoStudio.')) anchor = i; });
  if (anchor < 0) throw new Error(file + ': no videoStudio group');
  const added = rows.filter(([key]) => !Object.prototype.hasOwnProperty.call(existing, 'videoStudio.' + key)).map(([key, values]) => '  ' + JSON.stringify('videoStudio.' + key) + ': ' + JSON.stringify(values[index]));
  if (!added.length) { console.log(lang + ': nothing to add'); return; }
  const anchorIsLast = !lines[anchor].trimEnd().endsWith(',');
  if (anchorIsLast) lines[anchor] = lines[anchor].trimEnd() + ',';
  const insert = added.map((line, i) => (i === added.length - 1 && anchorIsLast ? line : line + ','));
  lines.splice(anchor + 1, 0, ...insert);
  const out = lines.join(NL);
  JSON.parse(out);
  writeFileSync(file, out);
  console.log(lang + ': added ' + added.length);
});
```

- [ ] **Step 4: Run the script**, then the tests.
```
node ../vsplan1-scripts/add-plan-strings.mjs ui/lang/desktop
go test ./ui -run '(?i)(DesktopVideoStudioPlanStrings|TestVideoStudioCountStringsHaveSingulars)' -count=1
git diff --stat -- ui/lang/desktop
```
Expected: the script prints `cs: added 122` … `zh: added 122`. The tests print `ok  	aurago/ui`. The diff stat shows 16 files with 122 insertions each and no deletions. Run `git diff --check -- ui/lang/desktop` and confirm the files stay LF (`file ui/lang/desktop/en.json` shows no CRLF).

- [ ] **Step 5: Commit** (run `detect_changes` first)
```
git add ui/lang/desktop ui/desktop_video_studio_plan_strings_test.go
git commit -m "feat(video-studio): plan mode strings in all sixteen desktop locales

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task D3: Plan mode browser tour

**Files:**
- Create: `ui/desktop_video_studio_plan_browser_test.go`

How the existing tests work: `ui/desktop_video_studio_browser_test.go` serves `ui.Content` (an `os.DirFS` of `ui/`) through `httptest`. It writes a `/fixture` page that loads the scripts in loader order, overrides `window.fetch` with a fake Video Studio API, and mounts `VideoStudioApp.render`. It is gated by `requirePrecisionBrowserSmoke` (`AURAGO_RUN_BROWSER_SMOKE=1`) and runs in `newSmokeBrowser`. This tour follows the same pattern with a fake plan API: GET 404 → POST 202 → GET planning (`"p1"`) → poll with `If-None-Match: "p1"` → review (`"p2"`) → PUT `If-Match: "p2"`. The read-only pass seeds the plan with `?plan=review`. No real server and no agent stub are needed.

- [ ] **Step 1: Write the test** `ui/desktop_video_studio_plan_browser_test.go`:

```go
package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Loader order (module-loader.js): the plan scripts sit between the menus and the shell.
var videoStudioPlanFixtureScripts = []string{
	"shared/sticker-catalog.js", "shared/sticker-library.js", "apps/video-studio-icons.js", "apps/video-studio-time.js",
	"apps/video-studio-effects.js", "apps/video-studio-picture.js", "apps/video-studio-anim.js", "apps/video-studio-scope-math.js",
	"apps/video-studio-media.js", "apps/video-studio-gl.js", "apps/video-studio-shaders.js", "apps/video-studio-compositor.js",
	"apps/video-studio-painter2d.js", "apps/video-studio-preview.js", "apps/video-studio-timeline.js", "apps/video-studio-ruler.js",
	"apps/video-studio-edit.js", "apps/video-studio-keyframe-edit.js", "apps/video-studio-navigation.js", "apps/video-studio-source-edit.js",
	"apps/video-studio-folders.js", "apps/video-studio-inspector.js", "apps/video-studio-colorwheel.js", "apps/video-studio-core.js",
	"apps/video-studio-library.js", "apps/video-studio-effects-library.js", "apps/video-studio-artwork.js", "apps/video-studio-jobs.js",
	"apps/video-studio-stage.js", "apps/video-studio-mask.js", "apps/video-studio-editing.js", "apps/video-studio-keyframes.js",
	"apps/video-studio-inspector-effects.js", "apps/video-studio-source.js", "apps/video-studio-tracks.js", "apps/video-studio-markers.js",
	"apps/video-studio-transport.js", "apps/video-studio-rangerender.js", "apps/video-studio-scopes.js", "apps/video-studio-derive.js",
	"apps/video-studio-projects.js", "apps/video-studio-mediabin.js", "apps/video-studio-menus.js",
	"apps/video-studio-plan-board.js", "apps/video-studio-plan.js", "apps/video-studio.js",
}

// The fake API: status with plan availability and a generation profile, one project, and the plan routes of brief §7.
// After a create, the first read answers "planning" (ETag "p1"); the next read answers the plan in review ("p2").
// ?plan=review seeds the reviewed plan, ?readonly makes the studio read-only.
const videoStudioPlanFixtureJS = `
window.fixtureErrors=[];window.addEventListener('error',e=>fixtureErrors.push(String(e.message)));window.addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
const params=new URLSearchParams(location.search);
window.fixtureLocales=__LOCALES__;window.fixtureCalls=[];window.fixturePlanGets=[];window.fixturePlanPuts=[];window.fixtureAgentBodies=[];
window.fixtureReviewPlan=__REVIEW__;window.fixtureReviewIssues=__ISSUES__;
const profile=__PROFILE__,availability={enabled:true,supported:true,image_generation:true,tts:true,music:true},project=__PROJECT__;
window.fixturePlan=null;window.fixturePlanTag='';window.fixtureIssues=[];window.fixtureAgent=null;window.fixtureGetsAfterStart=null;
if(params.get('plan')==='review'){window.fixturePlan=structuredClone(window.fixtureReviewPlan);window.fixtureIssues=structuredClone(window.fixtureReviewIssues);window.fixturePlanTag='"p2"';}
const json=(data,status,headers)=>new Response(JSON.stringify(data),{status:status||200,headers:Object.assign({'Content-Type':'application/json'},headers||{})});
const planBody=()=>({plan:window.fixturePlan,etag:window.fixturePlanTag,issues:window.fixtureIssues,profile,availability,agent:window.fixtureAgent});
const base='/api/desktop/video-studio',planPath=base+'/projects/p1/plan',originalFetch=window.fetch;
window.fetch=async(url,options={})=>{const path=String(url),method=String(options.method||'GET').toUpperCase(),headers=new Headers(options.headers||{});window.fixtureCalls.push([method,path]);
 if(path===base+'/status')return json({enabled:true,desktop_enabled:true,ffmpeg_ready:true,read_only:params.has('readonly'),limits:{max_duration_frames:18000,max_duration_seconds:3600,frame_rates:[{fps:24,ntsc:true},{fps:25},{fps:30},{fps:60}],canvas_sizes:[{width:1280,height:720},{width:720,height:1280},{width:720,height:720}]},generation:{enabled:true,configured:true,provider:'minimax',model:'MiniMax-Hailuo-02',durations_seconds:[6,10],image_modes:['first_frame'],profile},plan:availability});
 if(path===base+'/projects'&&method==='GET')return json({projects:[{id:'p1',project:{name:'Plan fixture'},desktop_path:'Documents/Video Studio/p1/project.json'}]});
 if(path===base+'/projects/p1'&&method==='GET')return json({id:'p1',project,desktop_path:'Documents/Video Studio/p1/project.json'},200,{ETag:'"v1"'});
 if(path===base+'/jobs?project_id=p1')return json({jobs:[]});
 if(path===planPath&&method==='GET'){window.fixturePlanGets.push(headers.get('If-None-Match')||'');
  if(window.fixtureGetsAfterStart!==null&&++window.fixtureGetsAfterStart>=2){window.fixturePlan=structuredClone(window.fixtureReviewPlan);window.fixtureIssues=structuredClone(window.fixtureReviewIssues);window.fixtureAgent={state:'done',mode:'create',phase:'check',tokens:1800,started_at:'2026-11-24T09:00:00Z',finished_at:'2026-11-24T09:01:00Z'};window.fixturePlanTag='"p2"';window.fixtureGetsAfterStart=null;}
  if(!window.fixturePlan)return json({error:'plan_not_found',code:'plan_not_found',message:'This project has no plan yet.'},404);
  // Part B's 304 rule: only for a settled plan (no job status known, stored status not planning).
  if(headers.get('If-None-Match')===window.fixturePlanTag&&!window.fixtureAgent&&window.fixturePlan.status!=='planning')return new Response(null,{status:304,headers:{ETag:window.fixturePlanTag}});
  return json(planBody(),200,{ETag:window.fixturePlanTag});}
 if(path===planPath&&method==='PUT'){const ifMatch=headers.get('If-Match')||'';window.fixturePlanPuts.push({ifMatch,body:JSON.parse(options.body)});
  if(!ifMatch)return json({error:'precondition_required',code:'precondition_required',message:'Save requires the current plan ETag in If-Match.'},428);
  if(window.fixtureAgent&&window.fixtureAgent.state==='running')return json({error:'plan_agent_running',code:'plan_agent_running',message:'The planning agent is working on this plan.'},409);
  if(!window.fixturePlan)return json({error:'plan_not_found',code:'plan_not_found',message:'This project has no plan yet.'},404);
  if(ifMatch!==window.fixturePlanTag)return json({error:'file_conflict',code:'file_conflict',conflict:{code:'file_conflict',path:'Documents/Video Studio/p1/plan.json',version:window.fixturePlanTag}},412);
  const stored=window.fixturePlan;window.fixturePlan=Object.assign(JSON.parse(options.body),{id:stored.id,version:stored.version,profile:stored.profile,status:stored.status==='planning'?'review':stored.status,cost:stored.cost});window.fixturePlanTag='"p'+(window.fixturePlanPuts.length+2)+'"';return json(planBody(),200,{ETag:window.fixturePlanTag});}
 if(path===planPath+'/agent'&&method==='POST'){const body=JSON.parse(options.body);window.fixtureAgentBodies.push(body);
  if(window.fixtureAgent&&window.fixtureAgent.state==='running')return json({error:'plan_agent_running',code:'plan_agent_running',message:'The planning agent is already working on this project.'},409);
  if(body.mode==='create'&&window.fixturePlan&&!body.replace)return json({error:'plan_exists',code:'plan_exists',message:'This project already has a plan. Confirm to replace it.'},409);
  if(body.mode==='create')window.fixturePlan={version:1,id:'plan_fixture',status:'planning',idea:body.idea,target_seconds:body.target_seconds,language:body.language,profile:{provider:'minimax',model:'MiniMax-Hailuo-02',resolution:'768P'},style:'',characters:[],scenes:[],audio:{music:{instrumental:true},clip_audio:'duck'},cost:{estimate:0,spent:0}};
  window.fixtureAgent={state:'running',mode:body.mode,phase:'outline',tokens:0,started_at:'2026-11-24T09:00:00Z',finished_at:'0001-01-01T00:00:00Z'};window.fixtureIssues=[];window.fixturePlanTag='"p1"';window.fixtureGetsAfterStart=0;
  // Part C's Start answers before the job has a phase.
  return json({agent:{state:'running',mode:body.mode,tokens:0,started_at:'2026-11-24T09:00:00Z',finished_at:'0001-01-01T00:00:00Z'}},202);}
 if(path===planPath+'/agent/stop'&&method==='POST')return json({stopped:!!(window.fixtureAgent&&window.fixtureAgent.state==='running')});
 return originalFetch(url,options);};
document.documentElement.lang='en';
const t=key=>window.fixtureLocales[key]||key;
const ctx={t,esc:v=>String(v).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])),iconMarkup:()=>'',setWindowBeforeClose:()=>{},setWindowMenus:()=>{},clearWindowMenus:()=>{},promptDialog:async()=>false,confirmDialog:async()=>false};
window.VideoStudioApp.render(document.getElementById('studio'),'fixture',ctx);
`

func videoStudioPlanFixture(t *testing.T) *httptest.Server {
	t.Helper()
	var poster bytes.Buffer
	art := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			art.Set(x, y, color.RGBA{R: 40, G: uint8(60 + y), B: 120, A: 255})
		}
	}
	if err := png.Encode(&poster, art); err != nil {
		t.Fatal(err)
	}
	locales := map[string]string{}
	for key, value := range readDesktopLocale(t, "en") {
		if strings.HasPrefix(key, "videoStudio.") || strings.HasPrefix(key, "stickers.") {
			locales[key] = value
		}
	}
	profile := map[string]any{"plan_supported": true, "provider": "minimax", "model": "MiniMax-Hailuo-02", "resolution": "768P", "durations_seconds": []int{6, 10}, "aspect_ratios": []string{}, "aspect_from_first_frame": true, "first_frame": true, "last_frame": true, "native_audio": false, "max_prompt_chars": 2000, "cost_per_clip_estimate": 0.2, "parallel": 2}
	scene := func(id, title, prompt string, use float64, transition, keyframe, narration map[string]any) map[string]any {
		return map[string]any{"id": id, "title": title, "video_prompt": prompt, "generate_seconds": 10, "use_seconds": use, "transition_in": transition, "keyframe": keyframe, "narration": narration, "state": "pending"}
	}
	// Total 9.5 + 10 + 10 - 0.8 (blend) = 28.7 s; scene 2 has no video prompt (one blocking issue).
	reviewPlan := map[string]any{
		"version": 1, "id": "plan_fixture", "status": "review", "idea": "A fisherman heads out at dawn", "target_seconds": 30, "language": "en",
		"profile": map[string]any{"provider": "minimax", "model": "MiniMax-Hailuo-02", "resolution": "768P"},
		"style":   "Soft morning light, slow camera moves", "characters": []any{map[string]any{"name": "Fisher", "description": "Old man in a yellow raincoat"}},
		"scenes": []any{
			scene("s1", "Harbour in fog", "Slow push-in over a foggy harbour at dawn", 9.5, map[string]any{"kind": "cut"}, map[string]any{"source": "generated", "image_prompt": "A foggy harbour at dawn, wooden boats"}, map[string]any{"text": "The bay is still asleep."}),
			scene("s2", "The boat leaves", "", 10, map[string]any{"kind": "continue"}, map[string]any{"source": "chain"}, map[string]any{}),
			scene("s3", "Sun over the bay", "Wide shot, the sun rises over the bay", 10, map[string]any{"kind": "blend", "type": "dissolve", "seconds": 0.8}, map[string]any{"source": "none"}, map[string]any{}),
		},
		"audio": map[string]any{"voice": "", "music": map[string]any{"prompt": "Calm acoustic guitar", "instrumental": true}, "clip_audio": "duck"},
		"cost":  map[string]any{"estimate": 0, "spent": 0}, "updated_at": "2026-11-24T09:00:00Z",
	}
	reviewIssues := []any{map[string]any{"code": "prompt_missing", "scene_id": "s2", "field": "scenes[1].video_prompt", "message": "The video prompt is empty.", "blocking": true}}
	project := map[string]any{
		"version": 1, "name": "Plan fixture", "width": 1280, "height": 720, "fps": 30,
		"assets": []any{map[string]any{"id": "art1", "name": "Coast.png", "path": "media/art1_coast.png", "kind": "image", "duration_frames": 18000, "width": 64, "height": 36, "has_audio": false}},
		"tracks": []any{
			map[string]any{"id": "v1", "kind": "video", "name": "Video 1", "muted": false, "hidden": false, "locked": false, "clips": []any{}},
			map[string]any{"id": "a1", "kind": "audio", "name": "Audio 1", "muted": false, "hidden": false, "locked": false, "clips": []any{}},
			map[string]any{"id": "o1", "kind": "overlay", "name": "Overlay 1", "muted": false, "hidden": false, "locked": false, "clips": []any{map[string]any{
				"id": "c1", "asset_id": "art1", "start": 0, "offset": 0, "duration": 150, "x": 0, "y": 0, "width": 1, "height": 1, "rotation": 0, "opacity": 1, "volume": 1,
				"fade_in": 0, "fade_out": 0, "fit": "contain", "text": "", "text_style": nil, "transition": nil,
			}}},
		},
	}
	var scripts strings.Builder
	for _, script := range videoStudioPlanFixtureScripts {
		scripts.WriteString(`<script src="/js/desktop/` + script + `"></script>`)
	}
	script := strings.NewReplacer("__LOCALES__", mustJSON(t, locales), "__REVIEW__", mustJSON(t, reviewPlan), "__ISSUES__", mustJSON(t, reviewIssues), "__PROFILE__", mustJSON(t, profile), "__PROJECT__", mustJSON(t, project)).Replace(videoStudioPlanFixtureJS)
	html := `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/css/desktop-shell-overrides.css"><link rel="stylesheet" href="/css/desktop-app-video-studio.css"><link rel="stylesheet" href="/css/desktop-stickers.css"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden}.desktop-body{font-family:system-ui,sans-serif;color:var(--vd-text,#edf0f5);background:var(--vd-theme-app-bg)}.vd-window-content{height:100%;width:100%;display:flex}#studio{flex:1;min-width:0;min-height:0}.vs-app{flex:1}</style></head><body class="desktop-body" data-theme="standard"><main class="vd-window-content"><div id="studio"></div></main>` + scripts.String() + `<script>` + script + `</script></body></html>`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, html)
	})
	mux.HandleFunc("/api/desktop/video-studio/projects/p1/media/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(poster.Bytes())
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// One tour: open a project, switch to Plan, empty state, start a plan (202, planning, poll, review), an issue next
// to its field, a title edit saved with If-Match, phone width without sideways scrolling, back to Edit, read-only.
func TestDesktopVideoStudioPlanBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	server := videoStudioPlanFixture(t)
	page := newSmokeBrowser(t).MustPage().Timeout(60 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(server.URL + "/fixture")
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>{const sw=document.querySelector('[data-plan-switch]');return !!sw&&!sw.hidden&&!!document.querySelector('.vs-clip')}`)

	page.MustElement(`[data-plan-mode="plan"]`).MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-plan-create-form] [data-plan-idea]')`)
	layout := page.MustEval(`()=>{const shown=s=>{const el=document.querySelector(s);return !!el&&el.getClientRects().length>0};return JSON.stringify({plan:document.querySelector('.vs-app').classList.contains('vs-plan-mode'),board:shown('[data-plan-board]'),stage:shown('.vs-preview-stage'),timeline:shown('.vs-timeline-panel'),library:shown('.vs-library'),pressed:document.querySelector('[data-plan-mode="plan"]').getAttribute('aria-pressed')})}`).Str()
	if want := `{"plan":true,"board":true,"stage":false,"timeline":false,"library":true,"pressed":"true"}`; layout != want {
		t.Fatalf("plan mode layout: got %s, want %s", layout, want)
	}

	page.MustElement(`[data-plan-idea]`).MustInput("A fisherman heads out at dawn")
	page.MustElement(`[data-plan-submit]`).MustClick()
	waitForJSBool(t, page, `()=>window.fixtureAgentBodies.length===1`)
	if got := page.MustEval(`()=>{const b=window.fixtureAgentBodies[0];return JSON.stringify([b.mode,b.idea,b.target_seconds,b.language,'replace' in b])}`).Str(); got != `["create","A fisherman heads out at dawn",30,"en",false]` {
		t.Fatalf("create request body: %s", got)
	}
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-plan-agent] [data-plan-action="stop"]')`)
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-plan-card="s3"]')`)
	if got := page.MustEval(`()=>JSON.stringify(window.fixturePlanGets)`).Str(); !strings.Contains(got, `"\"p1\""`) {
		t.Fatalf("the poll did not send If-None-Match: %s", got)
	}

	review := page.MustEval(`()=>{const en=window.fixtureLocales,issue=document.querySelector('[data-plan-issues="s2:video_prompt"] [data-plan-issue-code="prompt_missing"]');return JSON.stringify({issue:!!issue&&issue.textContent.trim()===en['videoStudio.plan.issue.prompt_missing'],blocking:document.querySelector('[data-plan-blocking]').textContent.trim()===en['videoStudio.plan.blocking_one'],total:document.querySelector('[data-plan-total]').textContent.trim(),chained:!!document.querySelector('[data-plan-card="s2"] [data-plan-source="chain"]'),stop:!!document.querySelector('[data-plan-action="stop"]')})}`).Str()
	if want := `{"issue":true,"blocking":true,"total":"28.7 s of 30 s","chained":true,"stop":false}`; review != want {
		t.Fatalf("review board: got %s, want %s", review, want)
	}

	page.MustEval(`()=>{const input=document.querySelector('[data-plan-card="s1"] [data-plan-field="title"]');input.focus();input.value='Harbour at dawn';input.dispatchEvent(new Event('input',{bubbles:true}))}`)
	waitForJSBool(t, page, `()=>window.fixturePlanPuts.length===1`)
	if got := page.MustEval(`()=>{const put=window.fixturePlanPuts[0];return JSON.stringify([put.ifMatch,put.body.scenes[0].title,put.body.scenes.length,document.activeElement===document.querySelector('[data-plan-card="s1"] [data-plan-field="title"]')])}`).Str(); got != `["\"p2\"","Harbour at dawn",3,true]` {
		t.Fatalf("title save: %s", got)
	}
	if errs := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).Str(); errs != "[]" {
		t.Fatalf("page errors: %s", errs)
	}

	page.MustSetViewport(390, 844, 1, false)
	waitForJSBool(t, page, `()=>document.querySelector('.vs-app').getBoundingClientRect().width<=390`)
	phone := page.MustEval(`()=>{const q=s=>document.querySelector(s),box=el=>el.getBoundingClientRect(),app=q('.vs-app'),scroll=q('.vs-plan-scroll'),area=box(scroll),cards=[...document.querySelectorAll('[data-plan-card]')].map(box),sw=box(q('[data-plan-switch]')),problems=[];
		if(app.scrollWidth>app.clientWidth+1)problems.push('the app scrolls sideways');
		if(scroll.scrollWidth>scroll.clientWidth+1)problems.push('the board scrolls sideways');
		cards.forEach((r,i)=>{if(r.left<area.left-1||r.right>area.right+1)problems.push('card '+(i+1)+' is wider than the board');if(i&&r.top<cards[i-1].bottom)problems.push('card '+(i+1)+' is not below card '+i);});
		if(sw.width===0||sw.left<0||sw.right>window.innerWidth+1)problems.push('the switch is off screen');
		return problems.length?JSON.stringify({problems,area,scroll:[scroll.scrollWidth,scroll.clientWidth],app:[app.scrollWidth,app.clientWidth]}):'ok'}`).Str()
	if phone != "ok" {
		t.Fatalf("phone plan board: %s", phone)
	}

	page.MustSetViewport(1280, 900, 1, false)
	page.MustElement(`[data-plan-mode="edit"]`).MustClick()
	waitForJSBool(t, page, `()=>!document.querySelector('.vs-app').classList.contains('vs-plan-mode')&&document.querySelector('.vs-preview-stage').getClientRects().length>0&&document.querySelector('[data-plan-board]').hidden`)

	page.MustNavigate(server.URL + "/fixture?readonly&plan=review")
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>{const sw=document.querySelector('[data-plan-switch]');return !!sw&&!sw.hidden}`)
	page.MustElement(`[data-plan-mode="plan"]`).MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-plan-card="s1"]')`)
	page.MustEval(`()=>{const input=document.querySelector('[data-plan-card="s1"] [data-plan-field="title"]');input.value='Changed';input.dispatchEvent(new Event('input',{bubbles:true}))}`)
	time.Sleep(1200 * time.Millisecond)
	readonly := page.MustEval(`()=>{const board=document.querySelector('[data-plan-board]'),fields=[...board.querySelectorAll('[data-plan-field]')];return JSON.stringify({actions:board.querySelectorAll('[data-plan-action],[data-plan-submit]').length,fields:fields.length>10,editable:fields.filter(el=>!el.disabled).length,puts:window.fixturePlanPuts.length,errors:window.fixtureErrors})}`).Str()
	if want := `{"actions":0,"fields":true,"editable":0,"puts":0,"errors":[]}`; readonly != want {
		t.Fatalf("read-only board: got %s, want %s", readonly, want)
	}
}
```

- [ ] **Step 2: Run it.**
`AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(?i)DesktopVideoStudioPlan' -count=1 -timeout 15m`
Expected: PASS for `TestDesktopVideoStudioPlanStrings` and `TestDesktopVideoStudioPlanBrowser`. The tour covers D1 and D2 end to end and is written last because it needs both. A failure is a bug in the D1 scripts or CSS, not in the fixture. Without `AURAGO_RUN_BROWSER_SMOKE=1` the tour reports SKIP.

- [ ] **Step 3: Commit** (run `detect_changes` first)
```
git add ui/desktop_video_studio_plan_browser_test.go
git commit -m "test(video-studio): plan mode browser tour

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task D4: Plan mode lines in the Video Studio contract

**Files:**
- Modify: `ui/js/desktop/apps/AGENTS.md` (`### Video Studio contract`: the load-order sentence, plus a new bullet after the empty-project bullet)

- [ ] **Step 1: Edit the load list. Old:**
```
  `-menus.js` (context menus), and `video-studio.js` last (42 Video Studio scripts after the two shared
  sticker scripts, so 44 in all; the same order in `module-loader.js`, both browser fixtures and
  `TestVideoStudioNavigationLoadOrder`; lists only get insertions). The parts share
```
**New:**
```
  `-menus.js` (context menus), `-plan-board.js` (plan mode storyboard DOM) and `-plan.js` (plan mode state, requests
  and the Edit | Plan switch), and `video-studio.js` last (44 Video Studio scripts after the two shared
  sticker scripts, so 46 in all; the same order in `module-loader.js`, both browser fixtures and
  `TestVideoStudioNavigationLoadOrder`; lists only get insertions). The parts share
```

- [ ] **Step 2: Insert after the line**
```
  asserts the card is visibly hit-testable at 1280x800 and 1910x760 in both themes.
```
this block:
```
- Plan mode (agent-planned clips, slice 1; spec `docs/superpowers/specs/2026-11-23-video-studio-plan-mode-design.md`
  §6) adds two globals, not `VideoStudioParts` members. `VideoStudioPlan` (`video-studio-plan.js`): `install(s)` (from
  `appRender`; host listeners with `s.abort.signal`), `sync(s)` (from `renderToolbar`: shows `[data-plan-switch]` only
  with an open project and `status.plan.enabled`, leaves plan mode otherwise, reloads on a project change),
  `setMode(s, on)`, `switchMarkup(s)`, `view(s)`, `load(s, conditional)`, `flush(s)` (the window close guard awaits
  it), `startAgent(s, body)`, `stopAgent(s)`, `dispose(s)` and the pure `totalSeconds(plan)` (Σ `use_seconds` minus the
  blend overlaps). `VideoStudioPlanBoard` (`video-studio-plan-board.js`, DOM only, draws from `view(s)`):
  `shellMarkup(s)`, `render(s, view)` (keeps the focused `[data-plan-key]` field, its caret and the scroll position),
  `refresh(s, view)` (bottom bar and duration chips only), `renderIssues(s, view)`, `openStart`, `openRevise`.
  - Mode: `s.planMode` is per window session (default edit, never stored). `.vs-plan-mode` on `.vs-app` hides the
    monitor bar, stage, transport, source monitor, scopes, timeline, inspector and the edit group (undo, redo,
    format); the board `[data-plan-board]` fills `.vs-center` and the library stays. `handleKeys` ignores editor
    shortcuts in plan mode (dialogs and Escape still work). At 600 px or less the switch takes its own toolbar row.
  - Requests: GET `…/projects/{id}/plan` on entering plan mode (404 = empty state, or the unsupported state when
    `status.plan.supported` is false; 409 `plan_invalid` = the empty state with a damaged-plan notice, whose create
    sends `replace: true`). While `plan.status` is `planning` or `agent.state` is `running` the board is
    read-only and polls every 2 s with `If-None-Match` (304 keeps the board as drawn). POST `…/plan/agent`
    `{mode:'create', idea, target_seconds, language}` (`language` = the UI language; "Plan again" adds
    `replace: true` after its dialog) or `{mode:'revise', scene_id, note}`; POST `…/plan/agent/stop`.
  - Saving: text and number fields write the model on `input` without a redraw; selects write on `change` and redraw
    when the card changes shape. One PUT with `If-Match` 800 ms after the last edit (one in flight; a newer edit saves
    after it). A 412/428 or `plan_agent_running` reloads the plan with a notice. Issues from every response render
    next to their field (by code, then `scene_id`) and as the blocking count in the bottom bar, text
    `tr(s, 'plan.issue.' + code, message)`. Read-only shows the board with every field disabled and no actions.
  - Every board control uses `data-plan-*` attributes only (the shell's handlers react to `data-action`,
    `data-field`, `data-transition` and similar). Strings are `videoStudio.plan.*` in all sixteen desktop locales.
    Checks: `go test ./ui -run '(?i)DesktopVideoStudioPlan' -count=1` (strings) and
    `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(?i)DesktopVideoStudioPlan' -count=1 -timeout 15m` (tour).
```

- [ ] **Step 3: Check that git sees the file.** `git status --short ui/js/desktop/apps/AGENTS.md` must list it. If it does not, run `git ls-files -v ui/js/desktop/apps/AGENTS.md`. A leading `h` means assume-unchanged; fix it with `git update-index --no-assume-unchanged ui/js/desktop/apps/AGENTS.md`.

- [ ] **Step 4: Commit**
```
git add ui/js/desktop/apps/AGENTS.md
git commit -m "docs(video-studio): plan mode contract in the desktop apps AGENTS.md

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

At the end of the batch, run the existing suites from brief §12. `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(?i)DesktopVideoStudio' -count=1 -timeout 30m` covers the changed UI fixture script list. The server fixture list is covered by `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run VideoStudioBrowser -count=1 -timeout 20m`.

## Contract notes

1. **Switch position:** the switch is a direct child of `<header class="vs-toolbar">`, between the project and edit groups. At 600 px or less it takes a second toolbar row, because the 390 px toolbar has about 16 px left. Brief §11 only says "header".
2. **What plan mode hides beyond the brief:** besides the preview and timeline areas, plan mode also hides the inspector, its toggle and scrim, and the edit group (undo, redo, format), since all of them act on the hidden timeline. `handleKeys` ignores editor shortcuts in plan mode for the same reason.
3. **POST bodies:** create sends `{mode:'create', idea, target_seconds, language}`, plus `replace:true` only for "Plan again". Revise sends `{mode:'revise', scene_id, note}` with no `language`. Part B must accept a missing `replace` as false. There is no `Idempotency-Key`, because §7 defines none.
4. **Start and save errors:** the UI translates `plan_daily_limit`, `plan_agent_running`, `plan_agent_unavailable` and `plan_unsupported_provider` (the last one reuses `plan.issue.plan_unsupported_provider`). `plan_exists`, `plan_not_found` and `scene_not_found` show `plan.changedElsewhere`, `plan_invalid` shows `plan.damaged`, and these plus `plan_agent_running` reload the plan; `project_size_limit` reuses `videoStudio.projectStorageFull` and 403 reuses `videoStudio.readonly`. `plan_disabled` (the switch is hidden while plan mode is off), the `invalid_*` codes (the forms check the same limits), `plan_agent_start_failed`, `plan_read_failed`, `plan_save_failed`, `project_not_found` and `configuration_changed` fall back to the generic "Planning could not be started." / "Save failed". GET 409 `plan_invalid` shows the empty state with `plan.damaged`, and its create sends `replace: true` (Part B refuses a create over a damaged file without it). A PUT answered 404 `plan_not_found` or 409 `plan_invalid` reloads like a 412.
5. **Issue placement:** issues are placed by `code` (a fixed table) and `scene_id`. `field` is read only for `prompt_too_long`, which Part A also reports for `scenes[i].keyframe.image_prompt` and `audio.music.prompt` (0-based indexes, like every A3 path); its text is therefore generic. Plan-level codes and codes with no slot on screen go to the plan header.
6. **"Planning" means `plan.status === 'planning' || agent.state === 'running'`:** a revise job is read-only and polled even if Part C keeps `status: review` during revise. Part B's GET should keep returning `agent` while a job runs.
7. **304 and live phase text:** `P.request` throws on 304; the UI catches status 304. Part B answers 304 only when no job status (live or last finished) is known for the project and the stored status is not `planning`, so every poll while the agent plans gets the full body and the phase line stays live. The D3 fixture follows the same rule.
8. **Profile source:** the board prefers `profile` and `availability` from the plan GET/PUT. It falls back to `status.generation.profile` and `status.plan`, then to `status.generation.durations_seconds` for durations. Allowed blend types are `VideoStudioEffects.transitions()` filtered by `available`, the same data the validator's known − unavailable list is built from.
9. **What the PUT sends:** the UI PUTs the whole plan as edited, including server-owned fields; the server keeps its own values per §7. New scenes get the first free id `sN`, `state: 'pending'` and a keyframe source of `generated` (with image generation) or `none`.
10. **`npm run check:ui` does not check this part:** it validates neither desktop app scripts nor locale strings. The strings are guarded by the new `TestDesktopVideoStudioPlanStrings` in `ui/desktop_video_studio_plan_strings_test.go`, a second test file the brief did not name. Its test name matches the brief's `-run '(?i)DesktopVideoStudioPlan'` filter.
11. **Test-only edit in `internal/server`:** per the AGENTS contract ("the same order in module-loader.js, both browser fixtures"), D1 inserts the two scripts into the script tags of `internal/server/video_studio_browser_test.go`. No server code changes.
12. **Unverified anchors:** `ui/js/desktop/core/module-loader.js`, `ui/js/desktop/apps/AGENTS.md` and the 16 locale files have uncommitted changes from another session in the main checkout. The Old blocks and anchors used here come from the Video Studio parts of those files, which that session did not touch, but they could not be checked against `main` itself. The locale script anchors on the last `videoStudio.` line, so it works either way.
13. **Keyframe source and the server's Normalize (controller DECISION):** the board derives `keyframe.source` from the transition exactly like Part A's Normalize (continue → `chain`, bridge → `bridge`, a chain/bridge source on any other transition → `none`, the first scene is a cut). It shows that source read-only on joined scenes and never sends a conflicting value. A PUT answer (the normalized plan) replaces the model only when no edit was typed while the PUT ran. Otherwise the local model with the newer edits stays, the next PUT follows at once, and its answer is adopted. Bridge is offered only with first frame, last frame and image generation. That is the only case in which the validator can accept a bridge start image the board can edit (the image prompt).
