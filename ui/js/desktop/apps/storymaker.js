// Storymaker window: the composition root. It owns the per-window instance (the state the other modules read and the bus they
// listen to), the start screen and the editor layout (toolbar, scene strip, stage with the player iframe and the scrubber,
// inspector and library, code tab, issues panel, banners), the save pipeline (edits are debounced, saved with If-Match and
// answered with the server's story; a conflict offers reload or overwrite), the player loading and thumbnails, the project
// event stream and the window disposal. Loaded last; publishes window.StorymakerApp = {render, dispose}. Classic script.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const instances = new Map();
    const SAVE_DELAY_MS = 700;
    const PREVIEW_DELAY_MS = 250;
    const THUMB_WIDTH = 192;
    const LIVE_CLEAR_MS = 5000;

    function createInstance(host, windowId, ctx) {
        const c = ctx || {};
        const inst = {
            windowId, host, ctx: c,
            t: (key, vars) => (typeof c.t === 'function' ? c.t(key, vars) : key),
            bus: S.createBus(),
            bag: S.bag(),
            api: S.createApi(),
            status: null,
            ctxReadonly: !!c.readonly,
            forcedReadonly: false,
            projectId: '', project: null, story: null, etag: '', issues: [], draftIssues: [], codeStatus: [], playerErrors: [],
            report: null, frame: 0, playing: false, selected: 0, tab: 'stage', sideTab: 'scene', saveState: 'saved',
            catalog: null, thumbs: new Map(), dirty: false, disposed: false, loadSeq: 0, thumbSeq: 0, conflict: false,
            wantFrame: null, seekBusy: false, seekSent: -1, seekTimer: 0,
            view: 'loading', parts: {}, root: null, content: null, liveTimer: 0
        };
        inst.assets = S.createAssets(inst.api);
        Object.defineProperty(inst, 'readonly', { get() { return inst.ctxReadonly || inst.forcedReadonly || !!(inst.status && inst.status.read_only); } });
        inst.domId = suffix => 'stm-' + String(windowId).replace(/[^A-Za-z0-9_-]/g, '_') + '-' + suffix;
        return inst;
    }

    // ---- small shared pieces ---------------------------------------------------------------------------------------------------

    function notify(inst, text, kind) {
        if (inst.disposed || !text) return;
        const live = inst.parts.live;
        if (!live) return;
        live.textContent = text;
        live.dataset.kind = kind || 'info';
        clearTimeout(inst.liveTimer);
        inst.liveTimer = setTimeout(() => { live.textContent = ''; }, LIVE_CLEAR_MS);
    }

    function stateCard(inst, titleKey, textKey, actions) {
        const { el, icon } = S;
        const t = inst.t;
        return el('div', { class: 'stm-state', role: 'status', dataset: { stm: 'state', state: titleKey } },
            icon('lock', 'stm-state-icon'),
            el('h2', { text: t(titleKey) }),
            el('p', { text: t(textKey) }),
            actions ? el('div', { class: 'stm-actions' }, actions) : null);
    }

    // The view area swaps (start screen, editor, state cards); the live region next to it stays.
    function showRoot(inst, node) {
        S.clear(inst.content);
        inst.content.appendChild(node);
    }

    function retryButton(inst, action) {
        return S.el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'retry' }, onClick: action }, S.icon('refresh'), S.el('span', { text: inst.t('storymaker.retry') }));
    }

    // Which state card an error ends in for the start: switched off, signed out, not allowed, anything else.
    function startFailure(inst, err) {
        const e = S.errors.normalize(err);
        if (e.code === 'storymaker_disabled') return stateCard(inst, 'storymaker.state.disabled_title', 'storymaker.state.disabled_text');
        if (e.status === 401 || e.code === 'unauthorized') return stateCard(inst, 'storymaker.state.unauthorized_title', 'storymaker.state.unauthorized_text');
        if (e.code === 'desktop_scope_required') return stateCard(inst, 'storymaker.state.scope_title', 'storymaker.state.scope_text');
        const card = stateCard(inst, 'storymaker.state.load_failed_title', 'storymaker.state.load_failed_text', [retryButton(inst, () => start(inst))]);
        if (e.message) card.appendChild(S.el('code', { class: 'stm-cap-issue', text: e.message.slice(0, 300) }));
        return card;
    }

    // ---- start screen ----------------------------------------------------------------------------------------------------------

    async function start(inst) {
        inst.view = 'loading';
        showRoot(inst, S.el('p', { class: 'stm-muted stm-loading', role: 'status', text: inst.t('storymaker.loading') }));
        try {
            inst.status = await inst.api.status();
            if (inst.disposed) return;
            if (inst.status && inst.status.enabled === false) { showRoot(inst, stateCard(inst, 'storymaker.state.disabled_title', 'storymaker.state.disabled_text')); return; }
            inst.bus.emit('status');
            await showProjects(inst);
        } catch (err) {
            if (inst.disposed || (err && err.aborted)) return;
            showRoot(inst, startFailure(inst, err));
        }
    }

    async function showProjects(inst) {
        closeEditor(inst);
        inst.view = 'projects';
        const view = S.createProjects({
            t: inst.t, domId: inst.domId, get readonly() { return inst.readonly; },
            createProject: spec => createProject(inst, spec),
            openProject: id => openProject(inst, id),
            deleteProject: project => deleteProject(inst, project),
            confirm: (title, message) => askConfirm(inst, title, message)
        });
        inst.parts.projects = view;
        const shell = S.el('div', { class: 'stm-start' });
        banners(inst, shell);
        shell.appendChild(view.el);
        showRoot(inst, shell);
        try {
            view.setProjects(await inst.api.listProjects());
        } catch (err) {
            if (inst.disposed || (err && err.aborted)) return;
            view.setProjects([]);
            view.setError(S.errors.describe(inst.t, err));
        }
        view.focus();
    }

    // The banner area of the start screen: read-only and the missing export parts.
    function banners(inst, host) {
        const { el, icon } = S;
        const t = inst.t;
        const area = el('div', { class: 'stm-banners', dataset: { stm: 'banners' } });
        const add = (key, tone, text) => area.appendChild(el('div', { class: 'stm-banner', role: 'status', dataset: { banner: key, tone } }, icon(tone === 'bad' ? 'alert' : 'info'), el('span', { text })));
        if (inst.readonly) add('readonly', 'info', t('storymaker.banner.readonly'));
        const status = inst.status || {};
        const ffmpegOk = !!(status.ffmpeg && status.ffmpeg.ready), browserOk = !!(status.browser && status.browser.ready);
        if (!ffmpegOk || !browserOk) {
            const missing = [!ffmpegOk ? t('storymaker.export.cap_ffmpeg') : '', !browserOk ? t('storymaker.export.cap_browser') : ''].filter(Boolean).join(', ');
            add('export_unavailable', 'info', t('storymaker.banner.export_unavailable', { missing }));
        }
        host.appendChild(area);
        return area;
    }

    async function createProject(inst, spec) {
        const result = await inst.api.createProject(spec);
        if (inst.disposed) return;
        await openProject(inst, result.project.id, result);
    }

    async function deleteProject(inst, project) {
        await inst.api.deleteProject(project.id);
        if (inst.disposed) return;
        notify(inst, inst.t('storymaker.projects.deleted', { title: project.title || project.id }));
        const view = inst.parts.projects;
        if (view) view.setProjects(await inst.api.listProjects());
    }

    async function askConfirm(inst, title, message) {
        if (typeof inst.ctx.confirmDialog !== 'function') return false;
        try { return !!(await inst.ctx.confirmDialog(title, message)); } catch (_) { return false; }
    }

    // ---- editor -----------------------------------------------------------------------------------------------------------------

    function closeEditor(inst) {
        clearTimeout(inst.saveTimer);
        clearTimeout(inst.previewTimer);
        clearTimeout(inst.seekTimer);
        inst.wantFrame = null;
        inst.seekBusy = false;
        inst.loadSeq++;
        inst.thumbSeq++;
        if (inst.events) { inst.events.close(); inst.events = null; }
        Object.keys(inst.parts).forEach(name => {
            const part = inst.parts[name];
            if (part && typeof part.dispose === 'function') part.dispose();
        });
        inst.parts = { live: inst.parts.live };
        inst.thumbs.forEach(bitmap => { if (bitmap && typeof bitmap.close === 'function') bitmap.close(); });
        inst.thumbs.clear();
        if (inst.player) inst.player.dispose();
        inst.player = null;
        inst.projectId = '';
        inst.story = null;
        inst.report = null;
        inst.dirty = false;
        inst.conflict = false;
        inst.playerErrors = [];
        inst.draftIssues = [];
        inst.bus.clear();
    }

    // true when the project is open; a failure is shown on the start screen (or as a state card) and answers false
    async function openProject(inst, id, fresh) {
        try {
            const data = fresh || await inst.api.getProject(id);
            if (inst.disposed) return;
            closeEditor(inst);
            inst.view = 'editor';
            inst.projectId = data.project.id;
            inst.project = data.project;
            inst.story = data.story;
            inst.etag = data.etag || '';
            inst.issues = Array.isArray(data.issues) ? data.issues : [];
            inst.codeStatus = Array.isArray(data.code_scenes) ? data.code_scenes : [];
            inst.selected = 0;
            inst.frame = 0;
            inst.playing = false;
            inst.tab = 'stage';
            inst.saveState = 'saved';
            buildEditor(inst);
            inst.events = inst.api.events(inst.projectId, job => inst.bus.emit('job', job));
            reloadPlayer(inst, 0);
            return true;
        } catch (err) {
            if (inst.disposed || (err && err.aborted)) return false;
            const view = inst.parts.projects;
            if (view) view.setError(S.errors.describe(inst.t, err)); else showRoot(inst, startFailure(inst, err));
            return false;
        }
    }

    function buildEditor(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const bus = inst.bus;
        Object.assign(inst, {
            notify: (text, kind) => notify(inst, text, kind),
            confirm: (title, message) => askConfirm(inst, title, message),
            editScene: (sceneId, fn, options) => editScene(inst, sceneId, fn, options),
            select: (index, options) => select(inst, index, options),
            addScene: kind => addScene(inst, kind),
            moveScene: (from, to) => moveScene(inst, from, to),
            deleteScene: index => deleteScene(inst, index),
            openCode: sceneId => openCode(inst, sceneId),
            addActor: entry => addActor(inst, entry),
            addProp: entry => addProp(inst, entry),
            setEnvironment: id => { const scene = currentScene(inst); if (scene) editScene(inst, scene.id, draft => { draft.environment = id; }, { immediate: true }); },
            actorInScene: entry => actorInScene(inst, entry),
            play: () => { if (inst.player) inst.player.play(); },
            pause: () => { if (inst.player) inst.player.pause(); },
            seek: frame => seek(inst, frame),
            flushEdits: () => flushEdits(inst),
            saveStoryText: text => saveStoryText(inst, text),
            saveCodeScene: (sceneId, text, etag) => saveCodeScene(inst, sceneId, text, etag),
            showIssues: () => toggleIssues(inst, true)
        });

        const aspect = inst.story.format && inst.story.format.aspect ? inst.story.format.aspect : '16:9';
        const stageFrame = el('div', { class: 'stm-frame', dataset: { stm: 'stage-frame' } });
        const stageNote = el('div', { class: 'stm-stage-note', role: 'status', dataset: { stm: 'stage-note' }, hidden: true });
        const stage = el('div', { class: 'stm-stage', style: { '--stm-ar': String(S.model.aspectRatio(aspect)) } }, stageFrame, stageNote);

        inst.player = S.createPlayer({
            container: stageFrame, src: inst.api.playerUrl(), title: t('storymaker.preview'),
            resolve: keys => inst.assets.resolve(keys, inst.story, inst.projectId)
        });
        bindPlayer(inst, stageNote);

        const strip = S.createStrip(inst);
        const transport = S.createTransport(inst);
        const inspector = S.createInspector(inst);
        const library = S.createLibrary(inst);
        const code = S.createCode(inst);
        const exporter = S.createExport(inst);
        code.el.hidden = true;
        inst.parts.strip = strip; inst.parts.transport = transport; inst.parts.inspector = inspector; inst.parts.library = library; inst.parts.code = code; inst.parts.exporter = exporter;

        const backButton = el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'back' }, onClick: () => leaveProject(inst) }, icon('left'), el('span', { text: t('storymaker.back_to_projects') }));
        const title = el('h2', { class: 'stm-title', dataset: { stm: 'project-title' }, text: inst.project.title || inst.projectId });
        const format = el('span', { class: 'stm-badge', dataset: { stm: 'format' }, text: aspect + ' · ' + ((inst.story.format && inst.story.format.fps) || 30) + ' fps' });
        const saveLabel = el('span', { class: 'stm-save', role: 'status', dataset: { stm: 'save-state' } });
        const tabButtons = [['stage', 'storymaker.tab_stage', 'film'], ['code', 'storymaker.tab_code', 'code']].map(([name, key, glyph]) => el('button', {
            type: 'button', class: 'stm-tab', role: 'tab', dataset: { stm: 'tab', tab: name }, ariaSelected: String(name === inst.tab),
            onClick: () => setTab(inst, name)
        }, icon(glyph), el('span', { text: t(key) })));
        const issuesButton = el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-issues-btn', dataset: { stm: 'issues-toggle' }, ariaExpanded: 'false', onClick: () => toggleIssues(inst) }, icon('alert'), el('span', { class: 'stm-issues-count', dataset: { stm: 'issues-count' } }));
        const bar = el('header', { class: 'stm-bar' }, backButton, el('div', { class: 'stm-title-group' }, title, format), saveLabel,
            el('div', { class: 'stm-tabs', role: 'tablist', ariaLabel: t('storymaker.title') }, tabButtons),
            el('span', { class: 'stm-spacer' }, ''), issuesButton, exporter.el);

        const issuesPanel = el('section', { class: 'stm-issues', hidden: true, ariaLabel: t('storymaker.issues.title'), dataset: { stm: 'issues' } });
        const sideTabs = [['scene', 'storymaker.side_scene', 'sliders'], ['library', 'storymaker.side_library', 'box']].map(([name, key, glyph]) => el('button', {
            type: 'button', class: 'stm-tab', role: 'tab', dataset: { stm: 'side-tab', tab: name }, ariaSelected: String(name === inst.sideTab),
            onClick: () => setSideTab(inst, name)
        }, icon(glyph), el('span', { text: t(key) })));
        const side = el('aside', { class: 'stm-side', dataset: { stm: 'side' } }, el('div', { class: 'stm-tabs stm-tabs-small', role: 'tablist', ariaLabel: t('storymaker.inspector.title') }, sideTabs), inspector.el, library.el);
        const center = el('main', { class: 'stm-center' }, el('div', { class: 'stm-stage-wrap', dataset: { stm: 'stage-wrap' } }, stage, transport.el), code.el, issuesPanel);
        const body = el('div', { class: 'stm-body' }, strip.el, center, side);
        const bannerArea = el('div', { class: 'stm-banners', dataset: { stm: 'banners' } });
        const app = el('div', { class: 'stm-editor-app', dataset: { tab: inst.tab, sidetab: inst.sideTab } }, bar, bannerArea, body);
        Object.assign(inst.parts, { app, bannerArea, issuesPanel, issuesButton, saveLabel, tabButtons, sideTabs, stageNote, title, format });
        showRoot(inst, app);
        app.addEventListener('keydown', event => {
            if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 's' && !app.querySelector('[data-stm="code"]:not([hidden])')) {
                event.preventDefault();
                flushEdits(inst);
            }
        });

        const catalogIndex = inst.assets.index();
        if (catalogIndex) { inst.catalog = catalogIndex; }
        inst.assets.loadCatalog().then(() => { inst.catalog = inst.assets.index(); bus.emit('catalog'); bus.emit('story'); }, err => { if (!inst.disposed) notify(inst, S.errors.describe(t, err), 'error'); });

        bus.on('issues', () => paintIssues(inst));
        bus.on('report', () => paintIssues(inst));
        bus.on('status', () => { paintBanners(inst); });
        bus.on('readonly', () => { paintBanners(inst); paintSave(inst); });
        bus.on('story', () => { paintSave(inst); });
        bus.on('conflict', () => { inst.conflict = true; paintBanners(inst); paintSave(inst); });
        paintBanners(inst);
        paintSave(inst);
        paintIssues(inst);
    }

    // ---- player ---------------------------------------------------------------------------------------------------------------

    function bindPlayer(inst, note) {
        const t = inst.t;
        const showNote = text => { note.textContent = text || ''; note.hidden = !text; };
        inst.player.on('loading', () => { inst.compiling = true; showNote(t('storymaker.stage.compiling')); });
        inst.player.on('compiled', report => {
            inst.report = report;
            inst.compiling = false;
            inst.playerErrors = [];
            inst.thumbsDue = true;
            showNote('');
            if (inst.frame > report.frame_count - 1) inst.frame = Math.max(0, report.frame_count - 1);
            inst.bus.emit('report');
        });
        inst.player.on('time', ({ frame, playing }) => {
            inst.playing = playing;
            // A paused host answers every seek with a time message. The next wanted frame goes out once the last one is answered,
            // and until then the playhead stays where the user put it (an older answer must not move it back).
            if (!playing && inst.seekBusy && frame === inst.seekSent) {
                inst.seekBusy = false;
                clearTimeout(inst.seekTimer);
                pumpSeek(inst);
            }
            if (playing || (!inst.seekBusy && inst.wantFrame === null)) inst.frame = frame;
            // The first time message after a compile is the "ready to play" signal: the assets are in, so scenes can be drawn.
            if (inst.thumbsDue) { inst.thumbsDue = false; requestThumbs(inst); }
            if (playing && inst.report) {
                const index = S.model.sceneAtFrame(inst.report, frame);
                if (index >= 0 && index !== inst.selected && Date.now() - (inst.selectedAt || 0) > 600) select(inst, index, { seek: false });
            }
            inst.bus.emit('time');
        });
        inst.player.on('error', error => {
            inst.playerErrors.push(error);
            if (inst.playerErrors.length > 20) inst.playerErrors.shift();
            if (error.code === 'bad_message') return;
            showNote(error.code === 'code_error' || error.code === 'deadline' ? t('storymaker.stage.code_error') : t('storymaker.stage.player_error'));
            inst.bus.emit('issues');
        });
        inst.player.on('unavailable', () => {
            inst.playerUnavailable = true;
            showNote(t('storymaker.stage.unavailable'));
            paintBanners(inst);
        });
        inst.player.on('ready', () => { inst.playerUnavailable = false; paintBanners(inst); });
    }

    async function reloadPlayer(inst, resume) {
        if (inst.disposed || !inst.player || !inst.story) return;
        const seq = ++inst.loadSeq;
        try {
            const prepared = await inst.assets.prepare(inst.story, inst.projectId);
            if (inst.disposed || seq !== inst.loadSeq) return;
            if (!inst.catalog) { inst.catalog = inst.assets.index(); inst.bus.emit('catalog'); }
            const format = inst.story.format || {};
            const aspect = ['16:9', '9:16', '1:1'].indexOf(format.aspect) >= 0 ? format.aspect : '16:9';
            const fps = [24, 25, 30].indexOf(format.fps) >= 0 ? format.fps : 30;
            inst.player.load({
                story: inst.story, catalog: prepared.catalog, audio_info: {}, assets: prepared.assets, fonts: prepared.fonts,
                code_bundles: prepared.code_bundles, format: { aspect, fps, width: S.model.previewWidth(aspect) }
            }, resume === undefined ? inst.frame : resume);
        } catch (err) {
            if (inst.disposed || seq !== inst.loadSeq || (err && err.aborted)) return;
            const note = inst.parts.stageNote;
            if (note) { note.textContent = S.errors.describe(inst.t, err); note.hidden = false; }
        }
    }

    function schedulePreview(inst) {
        clearTimeout(inst.previewTimer);
        inst.previewTimer = setTimeout(() => reloadPlayer(inst), PREVIEW_DELAY_MS);
    }

    async function requestThumbs(inst) {
        const token = ++inst.thumbSeq;
        const report = inst.report;
        if (!report || !inst.player) return;
        const ids = report.scenes.map(scene => scene.id);
        const scenes = S.model.scenesOf(inst.story);
        const first = scenes[inst.selected] ? scenes[inst.selected].id : '';
        const order = ids.slice().sort((a, b) => (a === first ? -1 : 0) - (b === first ? -1 : 0));
        inst.thumbs.forEach((bitmap, id) => {
            if (ids.indexOf(id) < 0) { if (bitmap && typeof bitmap.close === 'function') bitmap.close(); inst.thumbs.delete(id); }
        });
        for (const id of order) {
            if (inst.disposed || token !== inst.thumbSeq) return;
            try {
                const bitmap = await inst.player.thumb(id, THUMB_WIDTH);
                if (inst.disposed || token !== inst.thumbSeq) { if (bitmap && typeof bitmap.close === 'function') bitmap.close(); return; }
                const old = inst.thumbs.get(id);
                inst.thumbs.set(id, bitmap);
                if (old && old !== bitmap && typeof old.close === 'function') old.close();
                inst.bus.emit('thumb', id);
            } catch (_) {
                if (inst.disposed || token !== inst.thumbSeq) return;
            }
        }
    }

    // requestSeek moves the playhead at once and sends only the newest wanted frame to the player, one seek at a time: the host renders
    // seeks concurrently, so two in flight could be answered in the wrong order and leave the picture on the older frame.
    function requestSeek(inst, frame) {
        if (!inst.player || !inst.report) return -1;
        const target = S.clamp(Math.floor(frame), 0, Math.max(0, inst.report.frame_count - 1));
        inst.frame = target;
        inst.wantFrame = target;
        pumpSeek(inst);
        return target;
    }

    function pumpSeek(inst) {
        if (inst.disposed || !inst.player || inst.seekBusy || inst.wantFrame === null) return;
        const target = inst.wantFrame;
        inst.wantFrame = null;
        inst.player.seek(target);
        if (inst.playing) return; // a playing host takes the new start frame without an answer
        inst.seekBusy = true;
        inst.seekSent = target;
        clearTimeout(inst.seekTimer);
        inst.seekTimer = setTimeout(() => { inst.seekBusy = false; pumpSeek(inst); }, 600);
    }

    function seek(inst, frame) {
        const target = requestSeek(inst, frame);
        if (target < 0) return;
        const index = S.model.sceneAtFrame(inst.report, target);
        if (index >= 0 && index !== inst.selected) select(inst, index, { seek: false });
        inst.bus.emit('time');
    }

    // ---- selection and structure ------------------------------------------------------------------------------------------------

    function currentScene(inst) {
        return S.model.scenesOf(inst.story)[inst.selected] || null;
    }

    function select(inst, index, options) {
        const total = S.model.scenesOf(inst.story).length;
        if (!total) return;
        const next = S.clamp(index, 0, total - 1);
        const changed = next !== inst.selected;
        inst.selected = next;
        inst.selectedAt = Date.now();
        if (changed) inst.bus.emit('selection');
        if (options && options.seek && inst.report) {
            requestSeek(inst, S.model.frameForScene(inst.report, next));
            inst.bus.emit('time');
        }
    }

    function actorInScene(inst, entry) {
        const scene = currentScene(inst);
        if (!scene || !Array.isArray(scene.actors)) return false;
        const member = (inst.story.cast || []).find(candidate => candidate && candidate.character === entry.id);
        return !!member && scene.actors.some(actor => actor && actor.cast === member.id);
    }

    function addActor(inst, entry) {
        const scene = currentScene(inst);
        if (!scene || inst.readonly) return;
        if (actorInScene(inst, entry)) { notify(inst, inst.t('storymaker.library.already_in_scene')); return; }
        const draft = S.model.cloneStory(inst.story);
        const result = S.model.insertActor(draft, inst.selected, entry, S.catalogName(inst.t, entry));
        if (!result.ok) { notify(inst, inst.t('storymaker.library.already_in_scene')); return; }
        commit(inst, draft, true);
        notify(inst, inst.t('storymaker.library.added_actor', { name: S.catalogName(inst.t, entry) }));
    }

    function addProp(inst, entry) {
        const scene = currentScene(inst);
        if (!scene || inst.readonly) return;
        const draft = S.model.cloneStory(inst.story);
        const env = inst.catalog && inst.catalog.byId ? inst.catalog.byId[scene.environment] : null;
        if (!S.model.insertProp(draft, inst.selected, entry, env).ok) return;
        commit(inst, draft, true);
        notify(inst, inst.t('storymaker.library.added_prop', { name: S.catalogName(inst.t, entry) }));
    }

    async function addScene(inst, kind) {
        if (inst.readonly) return;
        const draft = S.model.cloneStory(inst.story);
        const index = S.model.addScene(draft, inst.selected, { kind });
        const id = draft.scenes[index].id;
        if (kind === 'code') {
            try {
                await inst.api.putCode(inst.projectId, id, S.codeTemplate, null);
            } catch (err) {
                notify(inst, S.errors.describe(inst.t, err), 'error');
                return;
            }
        }
        if (inst.disposed) return;
        commit(inst, draft, true);
        inst.selected = index;
        inst.bus.emit('selection');
        if (kind === 'code') openCode(inst, id);
    }

    function moveScene(inst, from, to) {
        if (inst.readonly) return;
        const draft = S.model.cloneStory(inst.story);
        const index = S.model.moveScene(draft, from, to);
        if (index < 0) return;
        commit(inst, draft, true);
        inst.selected = index;
        inst.bus.emit('selection');
    }

    async function deleteScene(inst, index) {
        const scene = S.model.scenesOf(inst.story)[index];
        if (!scene || inst.readonly) return;
        const ok = await askConfirm(inst, inst.t('storymaker.inspector.delete_scene'), inst.t('storymaker.inspector.delete_scene_text', { scene: scene.id }));
        if (!ok || inst.disposed) return;
        const draft = S.model.cloneStory(inst.story);
        const next = S.model.removeScene(draft, index);
        if (next < 0) return;
        if (inst.parts.code && scene.kind === 'code') inst.parts.code.forget(scene.id);
        commit(inst, draft, true);
        inst.selected = Math.min(next, draft.scenes.length - 1);
        inst.bus.emit('selection');
    }

    // ---- tabs, issues, banners --------------------------------------------------------------------------------------------------

    async function setTab(inst, name) {
        if (name === inst.tab) return;
        if (name === 'code') {
            if (inst.player) inst.player.pause();
            await flushEdits(inst);
        }
        inst.tab = name;
        const app = inst.parts.app;
        app.dataset.tab = name;
        inst.parts.tabButtons.forEach(button => button.setAttribute('aria-selected', String(button.dataset.tab === name)));
        app.querySelector('[data-stm="stage-wrap"]').hidden = name === 'code';
        inst.parts.code.el.hidden = name !== 'code';
        if (name === 'code') await inst.parts.code.show(inst.codeFile || 'story'); else inst.parts.code.hide();
        inst.bus.emit('tab');
    }

    async function openCode(inst, sceneId) {
        inst.codeFile = sceneId ? 'scene:' + sceneId : 'story';
        if (inst.tab === 'code') await inst.parts.code.show(inst.codeFile); else await setTab(inst, 'code');
    }

    function setSideTab(inst, name) {
        inst.sideTab = name;
        inst.parts.app.dataset.sidetab = name;
        inst.parts.sideTabs.forEach(button => button.setAttribute('aria-selected', String(button.dataset.tab === name)));
    }

    function allIssues(inst) {
        const report = inst.report || {};
        return S.model.visibleIssues(S.model.mergeIssues(inst.issues, report.issues, report.unverified, codeRuntimeIssues(inst)));
    }

    // Errors of code scenes found at run time (the bundle exists but its frame or build failed) join the list.
    function codeRuntimeIssues(inst) {
        return inst.playerErrors.filter(error => error.scene_id && (error.code === 'code_error' || error.code === 'deadline')).map(error => ({
            severity: 'error', code: error.code, path: '', scene_id: error.scene_id, message: error.message + (error.line ? ' (' + error.line + (error.column ? ':' + error.column : '') + ')' : '')
        }));
    }

    function toggleIssues(inst, force) {
        const panel = inst.parts.issuesPanel;
        const open = typeof force === 'boolean' ? force : panel.hidden;
        if (open && inst.tab === 'code') { setTab(inst, 'stage'); }
        panel.hidden = !open;
        inst.parts.issuesButton.setAttribute('aria-expanded', String(open));
        if (open) paintIssues(inst);
    }

    function focusIssue(inst, issue) {
        const index = S.model.sceneIndexOfIssue(issue, inst.story);
        if (index >= 0) select(inst, index, { seek: true });
    }

    function paintIssues(inst) {
        if (inst.disposed || !inst.parts.issuesPanel) return;
        const { el, icon } = S;
        const t = inst.t;
        const list = allIssues(inst);
        const counts = S.model.issueCounts(list);
        const button = inst.parts.issuesButton;
        // The badge counts what needs a look (errors and warnings); the notes of severity minor are in the list only.
        const total = counts.error + counts.warning;
        button.querySelector('[data-stm="issues-count"]').textContent = String(total);
        button.dataset.tone = counts.error ? 'bad' : counts.warning ? 'warn' : 'ok';
        button.setAttribute('aria-label', t('storymaker.issues.count', { count: total }));
        const panel = inst.parts.issuesPanel;
        if (panel.hidden) return;
        S.clear(panel);
        panel.appendChild(el('div', { class: 'stm-issues-head' }, el('h3', { text: t('storymaker.issues.title') }),
            el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', ariaLabel: t('storymaker.close'), title: t('storymaker.close'), onClick: () => toggleIssues(inst, false) }, icon('close'))));
        if (!list.length) { panel.appendChild(el('p', { class: 'stm-muted', text: t('storymaker.issues.none') })); return; }
        const rows = el('ul', { class: 'stm-issue-list' });
        list.slice(0, 200).forEach(issue => {
            const index = S.model.sceneIndexOfIssue(issue, inst.story);
            rows.appendChild(el('li', { class: 'stm-issue', dataset: { severity: issue.severity, code: issue.code } },
                el('button', { type: 'button', class: 'stm-issue-go', onClick: () => focusIssue(inst, issue) },
                    icon(issue.severity === 'error' ? 'alert' : 'info'),
                    el('span', { class: 'stm-issue-body' },
                        el('strong', { text: S.errors.issueTitle(t, issue.code) }),
                        index >= 0 ? el('span', { class: 'stm-badge', text: t('storymaker.strip.scene_n', { n: index + 1 }) }) : null,
                        el('span', { class: 'stm-issue-text', text: issue.message }),
                        issue.fix ? el('span', { class: 'stm-issue-fix', text: issue.fix }) : null)),
                issue.path ? el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'issue-code' }, title: t('storymaker.issues.show_in_code'), onClick: () => showIssueInCode(inst, issue) }, icon('code')) : null));
        });
        panel.appendChild(rows);
    }

    async function showIssueInCode(inst, issue) {
        inst.parts.issuesPanel.hidden = true;
        inst.codeFile = 'story';
        await setTab(inst, 'code');
        await inst.parts.code.show('story');
        return inst.parts.code.gotoPath(issue);
    }

    function paintBanners(inst) {
        if (inst.disposed || !inst.parts.bannerArea) return;
        const { el, icon } = S;
        const t = inst.t;
        const area = inst.parts.bannerArea;
        S.clear(area);
        const add = (key, tone, text, actions) => area.appendChild(el('div', { class: 'stm-banner', role: 'status', dataset: { banner: key, tone } }, icon(tone === 'bad' ? 'alert' : 'info'), el('span', { text }), actions || null));
        if (inst.readonly) add('readonly', 'info', t('storymaker.banner.readonly'));
        if (inst.conflict) {
            add('conflict', 'bad', t('storymaker.banner.conflict'), el('span', { class: 'stm-actions' },
                el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'conflict-reload' }, onClick: () => reloadProject(inst) }, el('span', { text: t('storymaker.banner.conflict_reload') })),
                el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'conflict-overwrite' }, disabled: inst.readonly || null, onClick: () => overwrite(inst) }, el('span', { text: t('storymaker.banner.conflict_overwrite') }))));
        }
        if (inst.playerUnavailable) add('player_unavailable', 'bad', t('storymaker.banner.player_unavailable'), retryButton(inst, () => reloadPlayerFrame(inst)));
        const status = inst.status || {};
        const ffmpegOk = !!(status.ffmpeg && status.ffmpeg.ready), browserOk = !!(status.browser && status.browser.ready);
        if (!ffmpegOk || !browserOk) {
            const missing = [!ffmpegOk ? t('storymaker.export.cap_ffmpeg') : '', !browserOk ? t('storymaker.export.cap_browser') : ''].filter(Boolean).join(', ');
            add('export_unavailable', 'info', t('storymaker.banner.export_unavailable', { missing }));
        }
    }

    function reloadPlayerFrame(inst) {
        if (!inst.player) return;
        inst.playerUnavailable = false;
        inst.player.iframe.src = inst.api.playerUrl();
        paintBanners(inst);
        reloadPlayer(inst, inst.frame);
    }

    function paintSave(inst) {
        if (inst.disposed || !inst.parts.saveLabel) return;
        const t = inst.t;
        let key = 'storymaker.save_saved';
        let state = 'saved';
        if (inst.conflict) { key = 'storymaker.save_conflict'; state = 'conflict'; }
        else if (inst.saveState === 'error') { key = 'storymaker.save_error'; state = 'error'; }
        else if (inst.saveState === 'offline') { key = 'storymaker.save_offline'; state = 'offline'; }
        else if (inst.saveState === 'saving') { key = 'storymaker.save_saving'; state = 'saving'; }
        else if (inst.dirty) { key = 'storymaker.save_dirty'; state = 'dirty'; }
        else if (inst.readonly) { key = 'storymaker.save_readonly'; state = 'readonly'; }
        inst.parts.saveLabel.textContent = t(key);
        inst.parts.saveLabel.dataset.state = state;
    }

    // ---- saving ---------------------------------------------------------------------------------------------------------------

    function editScene(inst, sceneId, fn, options) {
        if (inst.readonly || !inst.story) return false;
        const draft = S.model.cloneStory(inst.story);
        const scene = S.model.scenesOf(draft).find(candidate => candidate && candidate.id === sceneId);
        if (!scene) return false;
        fn(scene, draft);
        commit(inst, draft, !!(options && options.immediate));
        return true;
    }

    function commit(inst, draft, immediate) {
        inst.story = draft;
        inst.dirty = true;
        inst.saveState = 'dirty';
        inst.bus.emit('story');
        clearTimeout(inst.saveTimer);
        if (immediate) saveNow(inst);
        else inst.saveTimer = setTimeout(() => saveNow(inst), SAVE_DELAY_MS);
    }

    function saveNow(inst) {
        clearTimeout(inst.saveTimer);
        if (inst.saving) { inst.saveAgain = true; return inst.saving; }
        inst.saving = runSave(inst).finally(() => { inst.saving = null; });
        return inst.saving;
    }

    async function runSave(inst) {
        if (inst.readonly || inst.conflict) return false;
        do {
            inst.saveAgain = false;
            if (!inst.dirty) break;
            inst.saveState = 'saving';
            paintSave(inst);
            const sent = inst.story;
            const text = JSON.stringify(sent);
            try {
                const result = await inst.api.saveStory(inst.projectId, text, inst.etag);
                if (inst.disposed) return false;
                inst.etag = result.etag || inst.etag;
                inst.issues = Array.isArray(result.issues) ? result.issues : [];
                inst.draftIssues = [];
                if (inst.story === sent) {
                    if (result.story && JSON.stringify(result.story) !== text) inst.story = result.story;
                    inst.dirty = false;
                } else {
                    inst.saveAgain = true;
                }
                inst.saveState = 'saved';
                afterStorySaved(inst);
            } catch (err) {
                if (inst.disposed || (err && err.aborted)) return false;
                handleSaveError(inst, err);
                return false;
            }
        } while (inst.saveAgain);
        paintSave(inst);
        return true;
    }

    function afterStorySaved(inst) {
        const scenes = S.model.scenesOf(inst.story);
        inst.selected = S.clamp(inst.selected, 0, Math.max(0, scenes.length - 1));
        inst.bus.emit('story');
        inst.bus.emit('issues');
        if (inst.parts.code) inst.parts.code.storyChanged();
        schedulePreview(inst);
        paintSave(inst);
    }

    function handleSaveError(inst, err) {
        const e = S.errors.normalize(err);
        if (e.status === 412) { inst.conflict = true; inst.saveState = 'error'; }
        else if (e.code === 'desktop_readonly') { inst.forcedReadonly = true; inst.saveState = 'error'; inst.bus.emit('readonly'); }
        else if (e.status === 422) { inst.saveState = 'error'; inst.issues = e.issues.length ? e.issues : inst.issues; inst.bus.emit('issues'); }
        else inst.saveState = e.network ? 'offline' : 'error';
        notify(inst, S.errors.describe(inst.t, e), 'error');
        paintBanners(inst);
        paintSave(inst);
    }

    // Everything typed so far is on the server (or the reason it is not has been shown): true when the story is saved.
    async function flushEdits(inst) {
        clearTimeout(inst.saveTimer);
        if (inst.saving) await inst.saving;
        if (inst.dirty && !inst.conflict) await saveNow(inst);
        return !inst.dirty && !inst.conflict;
    }

    async function saveStoryText(inst, text) {
        await flushEdits(inst);
        if (inst.conflict) return { ok: false };
        try {
            const result = await inst.api.saveStory(inst.projectId, text, inst.etag);
            inst.etag = result.etag || inst.etag;
            inst.story = result.story || inst.story;
            inst.issues = Array.isArray(result.issues) ? result.issues : [];
            inst.draftIssues = [];
            inst.dirty = false;
            inst.saveState = 'saved';
            inst.catalogMismatch = false;
            afterStorySaved(inst);
            notify(inst, inst.t('storymaker.code.saved'));
            return { ok: true };
        } catch (err) {
            const e = S.errors.normalize(err);
            if (e.status === 422) {
                inst.draftIssues = e.issues;
                inst.bus.emit('issues');
                notify(inst, inst.t('storymaker.error.invalid_story'), 'error');
                return { ok: false, issues: e.issues };
            }
            handleSaveError(inst, e);
            return { ok: false };
        }
    }

    async function saveCodeScene(inst, sceneId, text, etag) {
        const result = await inst.api.putCode(inst.projectId, sceneId, text, etag);
        const entry = { scene_id: sceneId, ok: !!result.ok, errors: Array.isArray(result.errors) ? result.errors : [] };
        inst.codeStatus = inst.codeStatus.filter(item => item.scene_id !== sceneId).concat([entry]);
        refreshIssues(inst);
        if (result.ok) { schedulePreview(inst); notify(inst, inst.t('storymaker.code.bundle_ok')); } else notify(inst, inst.t('storymaker.code.bundle_failed'), 'error');
        inst.bus.emit('story');
        return result;
    }

    async function refreshIssues(inst) {
        try {
            const data = await inst.api.getProject(inst.projectId);
            if (inst.disposed || !inst.story) return;
            inst.issues = Array.isArray(data.issues) ? data.issues : inst.issues;
            inst.codeStatus = Array.isArray(data.code_scenes) ? data.code_scenes : inst.codeStatus;
            inst.bus.emit('issues');
        } catch (_) { /* the list stays as it was */ }
    }

    async function reloadProject(inst) {
        try {
            const data = await inst.api.getProject(inst.projectId);
            if (inst.disposed) return;
            inst.story = data.story;
            inst.etag = data.etag || '';
            inst.issues = Array.isArray(data.issues) ? data.issues : [];
            inst.codeStatus = Array.isArray(data.code_scenes) ? data.code_scenes : [];
            inst.draftIssues = [];
            inst.dirty = false;
            inst.conflict = false;
            inst.saveState = 'saved';
            inst.selected = S.clamp(inst.selected, 0, Math.max(0, S.model.scenesOf(inst.story).length - 1));
            paintBanners(inst);
            afterStorySaved(inst);
        } catch (err) {
            if (!inst.disposed && !(err && err.aborted)) notify(inst, S.errors.describe(inst.t, err), 'error');
        }
    }

    // Keep my version: take the etag of the stored story and save mine over it.
    async function overwrite(inst) {
        try {
            const data = await inst.api.getProject(inst.projectId);
            if (inst.disposed) return;
            inst.etag = data.etag || '';
            inst.conflict = false;
            inst.dirty = true;
            paintBanners(inst);
            await saveNow(inst);
        } catch (err) {
            if (!inst.disposed && !(err && err.aborted)) notify(inst, S.errors.describe(inst.t, err), 'error');
        }
    }

    async function leaveProject(inst) {
        const saved = await flushEdits(inst);
        if (!saved && inst.conflict) {
            const leave = await askConfirm(inst, inst.t('storymaker.leave_title'), inst.t('storymaker.leave_text'));
            if (!leave) return;
        }
        if (inst.parts.code && inst.parts.code.dirty()) {
            const leave = await askConfirm(inst, inst.t('storymaker.leave_title'), inst.t('storymaker.leave_code_text'));
            if (!leave) return;
        }
        if (inst.disposed) return;
        await showProjects(inst);
    }

    // The window is closing: save what is typed; unsaved code or a conflict asks first.
    async function beforeClose(inst) {
        if (inst.view !== 'editor') return true;
        const saved = await flushEdits(inst);
        const codeDirty = !!(inst.parts.code && inst.parts.code.dirty());
        if (saved && !codeDirty) return true;
        return askConfirm(inst, inst.t('storymaker.leave_title'), codeDirty ? inst.t('storymaker.leave_code_text') : inst.t('storymaker.leave_text'));
    }

    // ---- window -----------------------------------------------------------------------------------------------------------------

    function render(host, windowId, ctx) {
        dispose(windowId);
        const inst = createInstance(host, windowId, ctx);
        instances.set(windowId, inst);
        inst.content = S.el('div', { class: 'stm-view' });
        inst.parts.live = S.el('div', { class: 'stm-live', role: 'status', ariaLive: 'polite', dataset: { stm: 'live' } });
        inst.root = S.el('div', { class: 'stm-app', dataset: { stm: 'app' } }, inst.content, inst.parts.live);
        S.clear(host);
        host.appendChild(inst.root);
        inst.bag.on(document, 'aurago:desktop-policy', event => {
            const detail = event && event.detail;
            if (!detail || typeof detail.readonly !== 'boolean') return;
            inst.ctxReadonly = detail.readonly;
            if (!detail.readonly) inst.forcedReadonly = false;
            inst.bus.emit('readonly');
            if (inst.parts.projects) inst.parts.projects.refresh();
        });
        if (typeof inst.ctx.setWindowBeforeClose === 'function') inst.ctx.setWindowBeforeClose(windowId, () => beforeClose(inst));
        start(inst);
        return inst;
    }

    function dispose(windowId) {
        const inst = instances.get(windowId);
        if (!inst) return;
        instances.delete(windowId);
        inst.disposed = true;
        clearTimeout(inst.liveTimer);
        inst.api.abort();
        closeEditor(inst);
        inst.bag.dispose();
    }

    window.StorymakerApp = { render, dispose };
})();
