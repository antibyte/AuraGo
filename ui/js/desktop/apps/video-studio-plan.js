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
