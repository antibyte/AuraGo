// CYD Creator browser fixture: an in-memory /api/desktop/cyd-creator backend with the shapes of the
// plan overview (docs/superpowers/plans/2026-10-10-cyd-creator-overview.md, "HTTP API" and "SSE"):
// projects, layout with If-Match versions, files with If-Match sha, builds with streamed build_log,
// agent jobs (planning → plan confirm → coding → done, the project locked from the start like the
// plan-3 server), messages and revisions, the publication-review fence (CYD_BUSY publication_review)
// and a scripted EventSource for GET /projects/{id}/events. Loaded after aurora-fixture.js by
// TestDesktopCYDCreatorShellBrowser; a request no route takes lands in fixtureErrors.
(function () {
    'use strict';

    const previousFetch = window.fetch.bind(window);
    const BASE = '/api/desktop/cyd-creator';
    const JSON_HEADERS = { 'Content-Type': 'application/json; charset=utf-8' };
    const reply = (data, status, headers) => new Response(JSON.stringify(data), { status: status || 200, headers: Object.assign({}, JSON_HEADERS, headers || {}) });
    const empty = status => new Response(null, { status: status || 204 });
    const fail = (status, code, error, details) => reply({ error: error || code, code, details: details || {} }, status);
    // A project fenced after an interrupted publication refuses every write like the plan-2 server.
    const fenced = () => fail(409, 'CYD_BUSY', 'an interrupted publication of this project needs review; restart AuraGo or see the server log', { reason: 'publication_review' });
    const iso = offset => new Date(Date.now() + (offset || 0)).toISOString();
    const clone = v => JSON.parse(JSON.stringify(v));
    const wait = ms => new Promise(r => setTimeout(r, ms));
    let idSeq = 0;
    const nextID = prefix => prefix + '_' + String(++idSeq).padStart(12, '0');

    function sha(text) {
        let h = 5381;
        const s = String(text);
        for (let i = 0; i < s.length; i++) h = ((h << 5) + h + s.charCodeAt(i)) >>> 0;
        return h.toString(16).padStart(8, '0') + String(s.length);
    }

    function unrouted(method, path) {
        const message = 'cyd-creator-fixture: no route for ' + method + ' ' + path;
        if (Array.isArray(window.fixtureErrors)) window.fixtureErrors.push(message);
        return fail(404, 'CYD_NOT_FOUND', message);
    }

    const tokens = (bg, surface, text, accent) => ({ bg, surface, surface_alt: surface, text, text_muted: text, accent, on_accent: '#ffffff', success: '#2e9e5b', warning: '#d99a1e', danger: '#d64545', border: '#444444', focus: accent, border_width: 1, radius: 6 });
    const caps = {
        enabled: true,
        readonly: false,
        template_version: '1.0.0',
        toolchain: { docker_available: true, image: 'ghcr.io/antibyte/aurago-cyd-builder:1', present: true, pulling: false, progress: 0, error: '' },
        active_job: null,
        build_queue: 0,
        boards: [
            { id: 'esp32-2432s028r', label: 'CYD 2.8" (ESP32-2432S028R, micro-USB)', status: 'verified', chip: 'esp32', flash_mb: 4, psram: false, display: { driver: 'ILI9341', bus: 'spi', width: 320, height: 240, native_orientation: 'landscape' } },
            { id: 'esp32-3248s035c', label: 'CYD 3.5" (ESP32-3248S035C, kapazitiv)', status: 'community', chip: 'esp32', flash_mb: 4, psram: false, display: { driver: 'ST7796', bus: 'spi', width: 480, height: 320, native_orientation: 'landscape' } }
        ],
        themes: [
            { id: 'clear', font: 'montserrat', font_size: 16, button_style: 'filled', tokens: tokens('#f5f7fb', '#ffffff', '#1d2433', '#2f6fed') },
            { id: 'night', font: 'montserrat', font_size: 16, button_style: 'filled', tokens: tokens('#0f1626', '#18233a', '#e6edf7', '#2ec5d3') },
            { id: 'terminal', font: 'jetbrains_mono', font_size: 16, button_style: 'bracket', tokens: tokens('#050805', '#0b120b', '#5cff7a', '#5cff7a') },
            { id: 'sandstorm', font: 'montserrat', font_size: 16, button_style: 'filled', tokens: tokens('#f3e5c8', '#fbf1dc', '#4a3422', '#c4643a') },
            { id: 'dark_sun', font: 'montserrat', font_size: 16, button_style: 'outline', tokens: tokens('#0b0a0e', '#16141c', '#f4e7c5', '#f2b33d') },
            { id: 'synthwave', font: 'montserrat', font_size: 16, button_style: 'filled', tokens: tokens('#1b0d2e', '#2a1446', '#f7e8ff', '#ff3fb4') },
            { id: 'high_contrast', font: 'montserrat', font_size: 20, button_style: 'outline', tokens: tokens('#000000', '#000000', '#ffffff', '#ffe600') }
        ],
        widgets: [],
        fonts: { families: ['montserrat', 'inter', 'jetbrains_mono'], sizes: [12, 14, 16, 20, 24, 28, 32, 40, 48] },
        symbols: ['ok', 'close', 'home', 'wifi'],
        libraries: [{ id: 'arduinojson', name: 'ArduinoJson', version: '7.2.1', pio_spec: 'bblanchon/ArduinoJson@7.2.1' }],
        languages: ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'],
        ota: { enabled: true, public_url: 'https://aurago.test', insecure: false },
        limits: { max_screens: 16, max_widgets_per_screen: 200, max_app_files: 64, max_app_file_bytes: 262144, asset_budget_bytes: 262144 }
    };

    const APP_MAIN = '#include <cyd.h>\n#include "../generated/ui.h"\n\nvoid app_setup() {\n}\n\nvoid app_loop() {\n}\n';
    const GENERATED = {
        'src/generated/ui.h': '#pragma once\n#include <lvgl.h>\nvoid ui_init(void);\n',
        'src/generated/ui.cpp': '#include "ui.h"\nvoid ui_init(void) {}\n',
        'platformio.ini': '[env:esp32-2432s028r]\nplatform = espressif32\n',
        'lib/cyd_template/src/cyd.h': '#pragma once\nnamespace cyd {}\n'
    };

    const state = {
        caps,
        projects: new Map(),
        layouts: new Map(),
        files: new Map(),
        builds: new Map(),
        logs: new Map(),
        jobs: new Map(),
        messages: new Map(),
        revisions: new Map(),
        events: new Map(),
        listeners: new Map(),
        streams: [],
        seq: 0,
        requests: [],
        layoutPuts: [],
        filePuts: [],
        jobBodies: [],
        heldJob: null,
        buildOutcome: 'failed',
        // streamDown: 'unauthorized' answers every request with the desktop auth layer's 401 and makes
        // every event stream fail (breakStream); streamErrors counts the failed streams.
        streamDown: '',
        streamErrors: 0
    };

    function layoutFor(name, board, theme, language, timezone) {
        return {
            format: 1, template_version: '1.0.0', name, board, orientation: 'landscape', theme, language, timezone,
            settings: { wifi: 'optional', aurago_link: false, libraries: [], backlight: 100, statusbar_clock: false },
            start_screen: 's_000000000001',
            screens: [{ id: 's_000000000001', name: 'home', title: 'Start', transition: 'none', style: {}, widgets: [] }],
            assets: []
        };
    }

    function addProject(body) {
        const id = nextID('ccp');
        const now = iso();
        const project = {
            id, slug: String(body.name).toLowerCase().replace(/[^a-z0-9]+/g, '-'), name: body.name, board: body.board, theme: body.theme,
            version: 1, locked_by_job: '', has_draft: false, publish_review: false, last_build: null, created_at: now, updated_at: now
        };
        state.projects.set(id, project);
        state.layouts.set(id, { version: 1, project: layoutFor(body.name, body.board, body.theme, body.language || 'de', body.timezone || 'UTC0') });
        const files = new Map();
        files.set('src/app/app_main.cpp', { text: APP_MAIN, editable: true });
        Object.keys(GENERATED).forEach(path => files.set(path, { text: GENERATED[path], editable: false }));
        state.files.set(id, files);
        state.messages.set(id, []);
        state.revisions.set(id, [
            { id: nextID('ccr'), label: 'Projekt angelegt', reason: 'import', job_id: '', build_id: '', created_at: iso(-60000) }
        ]);
        state.events.set(id, []);
        return project;
    }

    function fileInfo(path, entry) {
        return { path, size: entry.text.length, sha256: sha(entry.text), editable: entry.editable, generated: !entry.editable };
    }

    // ── events ──
    // emit stores a persisted event (seq > 0, sent with an id line) or sends an ephemeral one
    // (persist false: seq 0, no id line, like build_log and stream_ready) to the open streams.
    function emit(projectId, type, data, persist) {
        const ev = { type, data: clone(data), seq: persist === false ? 0 : ++state.seq };
        if (ev.seq) state.events.get(projectId).push(ev);
        (state.listeners.get(projectId) || new Set()).forEach(fn => fn(ev));
    }

    class FixtureEventSource {
        constructor(url) {
            this.url = String(url);
            this.listeners = {};
            this.closed = false;
            this.ready = false;
            this.onerror = null;
            // A browser EventSource keeps lastEventId for an event without an id line.
            this.lastEventId = '';
            state.streams.push(this);
            setTimeout(() => this.open(), 10);
        }
        addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
        removeEventListener(type, fn) { this.listeners[type] = (this.listeners[type] || []).filter(f => f !== fn); }
        close() {
            this.closed = true;
            if (this.unsubscribe) this.unsubscribe();
        }
        dispatch(ev) {
            if (this.closed) return;
            if (ev.seq) this.lastEventId = String(ev.seq);
            const event = { type: ev.type, data: JSON.stringify(ev.data), lastEventId: this.lastEventId };
            (this.listeners[ev.type] || []).slice().forEach(fn => fn(event));
        }
        fail() {
            if (this.closed) return;
            state.streamErrors++;
            if (this.unsubscribe) this.unsubscribe();
            if (typeof this.onerror === 'function') this.onerror({ type: 'error' });
        }
        open() {
            if (this.closed) return;
            const u = new URL(this.url, location.origin);
            const match = /^\/api\/desktop\/cyd-creator\/projects\/([^/]+)\/events$/.exec(u.pathname);
            if (!match || !state.projects.has(decodeURIComponent(match[1]))) {
                unrouted('GET', u.pathname + u.search);
                if (typeof this.onerror === 'function') this.onerror({ type: 'error' });
                return;
            }
            if (state.streamDown) { this.fail(); return; }
            const projectId = decodeURIComponent(match[1]);
            const events = state.events.get(projectId);
            const newest = events.length ? events[events.length - 1].seq : 0;
            // Like the plan-2 handler: a resume point above the newest event cannot be continued
            // (reload), stream_ready carries the seq the live events continue after.
            let after = Number(u.searchParams.get('after')) || 0;
            let reload = false;
            if (after > newest) { after = newest; reload = true; }
            events.filter(ev => ev.seq > after).forEach(ev => { this.dispatch(ev); after = ev.seq; });
            this.dispatch({ type: 'stream_ready', data: reload ? { reload: true, after } : { after }, seq: 0 });
            this.ready = true;
            const listener = ev => this.dispatch(ev);
            if (!state.listeners.has(projectId)) state.listeners.set(projectId, new Set());
            state.listeners.get(projectId).add(listener);
            this.unsubscribe = () => state.listeners.get(projectId).delete(listener);
        }
    }
    window.EventSource = FixtureEventSource;

    // ── builds ──
    async function runBuild(build) {
        const project = state.projects.get(build.project_id);
        await wait(40);
        build.status = 'running';
        build.started_at = iso();
        emit(build.project_id, 'build_status', { build });
        const lines = ['Processing esp32-2432s028r (platform: espressif32)', 'Compiling .pio/build/src/app/app_main.cpp.o'];
        const failed = state.buildOutcome === 'failed';
        if (failed) lines.push("src/app/app_main.cpp:5:3: error: 'temperatur' was not declared in this scope");
        lines.push(failed ? '*** [.pio/build/firmware.elf] Error 1' : 'Linking .pio/build/firmware.elf', failed ? 'FAILED' : 'SUCCESS');
        for (const line of lines) {
            await wait(30);
            state.logs.set(build.id, (state.logs.get(build.id) || []).concat(line));
            emit(build.project_id, 'build_log', { build_id: build.id, lines: [line] }, false);
        }
        await wait(30);
        build.finished_at = iso();
        build.usage = { flash_used: 1210000, flash_total: 1966080, ram_used: 81234, ram_total: 327680, duration_ms: 4100, warnings: 0 };
        if (failed) {
            build.status = 'failed';
            build.error_code = 'CYD_BUILD_FAILED';
            build.diagnostics = [{ file: 'src/app/app_main.cpp', line: 5, column: 3, severity: 'error', message: "'temperatur' was not declared in this scope", generated: false }];
        } else {
            build.status = 'succeeded';
            build.artifacts = [{ name: 'merged.bin', size: 1300000, sha256: 'ab' }, { name: 'firmware.bin', size: 1234567, sha256: 'cd' }];
        }
        project.last_build = clone(build);
        emit(build.project_id, 'build_status', { build });
    }

    // ── jobs ──
    function message(projectId, jobId, role, text, attachments) {
        const m = { id: nextID('ccm'), job_id: jobId, role, text, attachments: attachments || [], created_at: iso() };
        state.messages.get(projectId).push(m);
        emit(projectId, 'job_message', { message: m });
        return m;
    }
    function setJob(job, patch) {
        Object.assign(job, patch);
        emit(job.project_id, 'job_status', { job });
    }
    function setLock(projectId, jobId) {
        state.projects.get(projectId).locked_by_job = jobId;
        emit(projectId, 'lock_changed', { locked_by_job: jobId });
    }

    // runJob: a new project plans first and waits for the confirmation with the plan (its summary is
    // also the agent's chat message); change and generate_code start coding right away.
    async function runJob(job) {
        await wait(60);
        if (job.mode === 'new') {
            setJob(job, { status: 'running', phase: 'planning', started_at: job.started_at || iso() });
            await wait(60);
            emit(job.project_id, 'job_step', { job_id: job.id, kind: 'info', text: 'Plan drafted' });
            setJob(job, {
                status: 'awaiting_confirmation', phase: 'planning',
                plan: { summary: 'Ein Thermometer mit großer Temperaturanzeige und Verlauf.', screens: ['home', 'verlauf'], widgets: ['lbl_temp', 'chart_verlauf'], data_sources: ['Open-Meteo'], wifi: 'required', aurago_link: false, libraries: ['arduinojson'] }
            });
            message(job.project_id, job.id, 'agent', job.plan.summary);
            return;
        }
        await work(job, 'coding');
    }

    // work runs a job from startPhase (layout after a confirmed plan, else coding) and holds it in
    // coding until cydFixture.finishJob().
    async function work(job, startPhase) {
        setJob(job, { status: 'running', phase: startPhase, started_at: job.started_at || iso() });
        await wait(60);
        if (startPhase === 'layout') {
            emit(job.project_id, 'job_step', { job_id: job.id, kind: 'layout', text: '2 widgets added' });
            setJob(job, { phase: 'coding' });
        }
        emit(job.project_id, 'job_step', { job_id: job.id, kind: 'file', text: 'src/app/app_main.cpp written' });
        state.heldJob = job;
    }

    // finishJob builds and publishes the held job in the plan-3 order: project_changed (layout and
    // files, origin job:<id>), lock_changed, job_status succeeded/done, then the agent's summary.
    async function finishJob() {
        const job = state.heldJob;
        if (!job) return false;
        state.heldJob = null;
        setJob(job, { phase: 'building' });
        await wait(40);
        const projectId = job.project_id;
        const project = state.projects.get(projectId);
        const layout = state.layouts.get(projectId);
        layout.version += 1;
        layout.project.screens[0].title = 'Thermometer';
        project.version = layout.version;
        project.locked_by_job = '';
        project.has_draft = false;
        state.revisions.get(projectId).unshift({ id: nextID('ccr'), label: 'Job abgeschlossen', reason: 'job', job_id: job.id, build_id: '', created_at: iso() });
        const origin = 'job:' + job.id;
        emit(projectId, 'project_changed', { version: layout.version, origin, what: 'layout' });
        emit(projectId, 'project_changed', { version: layout.version, origin, what: 'files' });
        emit(projectId, 'lock_changed', { locked_by_job: '' });
        setJob(job, { status: 'succeeded', phase: 'done', finished_at: iso() });
        message(projectId, job.id, 'agent', 'Fertig: Die Temperatur steht groß auf dem Startscreen.');
        return true;
    }

    // ── routes ──
    async function route(method, path, init) {
        const u = new URL(path, location.origin);
        const parts = u.pathname.slice(BASE.length).split('/').filter(Boolean).map(decodeURIComponent);
        const headers = init.headers || {};
        const ifMatch = headers['If-Match'] || headers['if-match'] || '';
        const body = () => { try { return JSON.parse(init.body || '{}'); } catch (_) { return {}; } };
        const [head, id, sub] = parts;

        // An expired session: AuraGo's auth layer answers {"error":"unauthorized"} without a code.
        if (state.streamDown === 'unauthorized') return reply({ error: 'unauthorized' }, 401);
        if (head === 'capabilities' && method === 'GET') return reply(state.caps);
        if (head === 'toolchain' && id === 'pull' && method === 'POST') return reply({ toolchain: state.caps.toolchain }, 202);
        if (head === 'projects' && !id) {
            if (method === 'GET') return reply({ projects: Array.from(state.projects.values()).map(clone) });
            if (method === 'POST') return reply({ project: clone(addProject(body())) }, 201);
        }
        if (head === 'projects' && id) {
            const project = state.projects.get(id);
            if (!project) return fail(404, 'CYD_NOT_FOUND', 'project not found');
            const locked = !!project.locked_by_job;
            if (!sub) {
                if (method === 'GET') return reply({ project: clone(project) });
                if (method === 'PATCH') { project.name = body().name || project.name; return reply({ project: clone(project) }); }
                if (method === 'DELETE') { if (locked) return fail(423, 'CYD_PROJECT_LOCKED', 'locked'); state.projects.delete(id); return empty(204); }
            }
            if (sub === 'layout') {
                const layout = state.layouts.get(id);
                if (method === 'GET') return reply({ version: layout.version, project: clone(layout.project), issues: [] });
                if (method === 'PUT') {
                    if (project.publish_review) return fenced();
                    if (locked) return fail(423, 'CYD_PROJECT_LOCKED', 'locked');
                    if (String(ifMatch) !== String(layout.version)) return fail(409, 'CYD_PROJECT_CONFLICT', 'stale layout', { version: layout.version });
                    layout.version += 1;
                    layout.project = body().project;
                    project.version = layout.version;
                    state.layoutPuts.push({ version: layout.version, project: clone(layout.project) });
                    emit(id, 'project_changed', { version: layout.version, origin: 'user', what: 'layout' });
                    return reply({ version: layout.version, issues: [{ severity: 'warning', code: 'PARTLY_OUT_OF_BOUNDS', message: 'widget lbl_demo is partly outside the screen', screen: 'home', widget: 'lbl_demo' }] });
                }
            }
            if (sub === 'files') {
                const files = state.files.get(id);
                const filePath = parts.slice(3).join('/');
                if (!filePath && method === 'GET') return reply({ files: Array.from(files.entries()).map(([p, e]) => fileInfo(p, e)) });
                const entry = files.get(filePath);
                if (method === 'GET') {
                    if (!entry) return fail(404, 'CYD_NOT_FOUND', 'file not found');
                    return new Response(entry.text, { status: 200, headers: { 'Content-Type': 'text/plain; charset=utf-8', 'X-CYD-SHA256': sha(entry.text) } });
                }
                if (method === 'PUT') {
                    if (entry && !entry.editable) return fail(403, 'CYD_FILE_READONLY', 'generated file');
                    if (project.publish_review) return fenced();
                    if (locked) return fail(423, 'CYD_PROJECT_LOCKED', 'locked');
                    if (entry ? ifMatch !== sha(entry.text) : ifMatch !== '*') return fail(409, 'CYD_FILE_CONFLICT', 'stale file');
                    const text = String(init.body || '');
                    files.set(filePath, { text, editable: true });
                    state.filePuts.push({ path: filePath, ifMatch, text });
                    emit(id, 'project_changed', { version: project.version, origin: 'user', what: 'files' });
                    return reply({ sha256: sha(text) });
                }
                if (method === 'DELETE') {
                    if (!entry) return fail(404, 'CYD_NOT_FOUND', 'file not found');
                    if (!entry.editable) return fail(403, 'CYD_FILE_READONLY', 'generated file');
                    if (project.publish_review) return fenced();
                    if (ifMatch !== sha(entry.text)) return fail(409, 'CYD_FILE_CONFLICT', 'stale file');
                    files.delete(filePath);
                    emit(id, 'project_changed', { version: project.version, origin: 'user', what: 'files' });
                    return empty(204);
                }
            }
            if (sub === 'builds') {
                if (method === 'GET') return reply({ builds: Array.from(state.builds.values()).filter(b => b.project_id === id).map(clone) });
                if (method === 'POST') {
                    if (project.publish_review) return fenced();
                    if (locked) return fail(423, 'CYD_PROJECT_LOCKED', 'locked');
                    const build = { id: nextID('ccb'), project_id: id, job_id: '', origin: 'user', status: 'queued', board: project.board, project_version: project.version, error_code: '', diagnostics: [], usage: {}, artifacts: [], created_at: iso(), started_at: '', finished_at: '' };
                    state.builds.set(build.id, build);
                    emit(id, 'build_status', { build });
                    runBuild(build);
                    return reply({ build: clone(build) }, 202);
                }
            }
            if (sub === 'jobs') {
                if (method === 'GET') return reply({ jobs: Array.from(state.jobs.values()).filter(j => j.project_id === id).reverse().map(clone) });
                if (method === 'POST') {
                    const b = body();
                    state.jobBodies.push(b);
                    if (project.publish_review) return fenced();
                    if (locked) return fail(409, 'CYD_BUSY', 'the project is locked by another job', { job_id: project.locked_by_job });
                    if (Array.from(state.jobs.values()).some(j => ['queued', 'running', 'awaiting_confirmation'].includes(j.status))) return fail(409, 'CYD_BUSY', 'busy');
                    // error_reason is omitted while empty (omitempty on the server).
                    const job = { id: nextID('ccj'), project_id: id, mode: b.mode, status: 'queued', phase: 'queued', repair_attempt: 0, plan: null, error_code: '', error_message: '', build_id: '', resume_of: '', created_at: iso(), started_at: '', finished_at: '' };
                    state.jobs.set(job.id, job);
                    // The plan-3 server locks the project when the job starts (also while it plans).
                    setLock(id, job.id);
                    if (b.prompt) message(id, job.id, 'user', b.prompt, b.attachments);
                    emit(id, 'job_status', { job });
                    runJob(job);
                    return reply({ job: clone(job) }, 202);
                }
            }
            if (sub === 'messages' && method === 'GET') return reply({ messages: clone(state.messages.get(id)) });
            if (sub === 'revisions' && method === 'GET') return reply({ revisions: clone(state.revisions.get(id)) });
            if (sub === 'draft' && parts[3] === 'discard' && method === 'POST') { project.has_draft = false; return empty(204); }
        }
        if (head === 'builds' && id) {
            const build = state.builds.get(id);
            if (!build) return fail(404, 'CYD_NOT_FOUND', 'build not found');
            if (!sub && method === 'GET') return reply({ build: clone(build) });
            if (sub === 'log' && method === 'GET') return new Response((state.logs.get(id) || []).join('\n'), { status: 200, headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
            if (sub === 'cancel' && method === 'POST') return reply({}, 202);
        }
        if (head === 'jobs' && id) {
            const job = state.jobs.get(id);
            if (!job) return fail(404, 'CYD_JOB_NOT_FOUND', 'job not found');
            if (!sub && method === 'GET') return reply({ job: clone(job) });
            if (sub === 'confirm' && method === 'POST') {
                if (job.status !== 'awaiting_confirmation') return fail(409, 'CYD_JOB_STATE', 'not awaiting confirmation');
                if (state.projects.get(job.project_id).publish_review) return fenced();
                const b = body();
                if (b.prompt) {
                    setJob(job, { status: 'running', phase: 'planning', plan: null });
                    message(job.project_id, job.id, 'user', b.prompt);
                    setTimeout(() => runJob(job), 30);
                } else {
                    setJob(job, { status: 'running' });
                    setTimeout(() => work(job, 'layout'), 30);
                }
                return reply({ job: clone(job) });
            }
            if (sub === 'cancel' && method === 'POST') {
                if (state.projects.get(job.project_id).locked_by_job === job.id) setLock(job.project_id, '');
                setJob(job, { status: 'cancelled', finished_at: iso() });
                state.heldJob = null;
                return reply({}, 202);
            }
        }
        if (head === 'revisions' && id) {
            if (sub === 'diff' && method === 'GET') return reply({ layout_changed: true, files: [{ path: 'src/app/app_main.cpp', status: 'modified', diff: '@@ -4,2 +4,3 @@\n void app_setup() {\n+  cyd::log::info("start");\n }' }] });
            if (sub === 'restore' && method === 'POST') return reply({ version: 1, revision: { id: nextID('ccr'), label: 'Wiederhergestellt', reason: 'restore', job_id: '', build_id: '', created_at: iso() } });
        }
        return unrouted(method, u.pathname);
    }

    window.fetch = async (url, options) => {
        const path = String(url);
        const init = options || {};
        if (!path.startsWith(BASE + '/')) return previousFetch(url, init);
        const method = String(init.method || 'GET').toUpperCase();
        state.requests.push(method + ' ' + path.split('?')[0]);
        return route(method, path, init);
    };

    window.cydFixture = {
        state,
        sha,
        addProject,
        emit,
        finishJob,
        inst: () => {
            const all = window.CYDCreatorApp ? Array.from(window.CYDCreatorApp.instances.values()) : [];
            return all[all.length - 1] || null;
        },
        // touchFile changes a file server-side (another window, the agent) so the next save conflicts.
        touchFile(projectId, path, text) {
            state.files.get(projectId).set(path, { text, editable: true });
        },
        // fence sets or clears the publication-review fence. Clearing it announces a meta change, so
        // the window re-reads the summary.
        fence(projectId, on) {
            const project = state.projects.get(projectId);
            project.publish_review = !!on;
            if (!on) emit(projectId, 'project_changed', { version: project.version, origin: 'user', what: 'meta' });
        },
        // breakStream fails the open event streams and every new one (reason 'unauthorized' also
        // answers every request with 401) until state.streamDown is cleared.
        breakStream(reason) {
            state.streamDown = reason || 'unauthorized';
            state.streams.filter(es => !es.closed).forEach(es => es.fail());
        }
    };

    addProject({ name: 'Wetterstation', board: 'esp32-2432s028r', theme: 'night', language: 'de', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3' });
})();
