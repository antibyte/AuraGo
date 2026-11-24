(function () {
    'use strict';
    const MODES = ['notes', 'sound', 'performance', 'mix', 'strudel', 'rb338'];
    const MODE_NAME = { notes: 'arrange', sound: 'sound', performance: 'perform', mix: 'mix', strudel: 'strudel', rb338: 'rb338' };
    const PERFORM_TABS = ['stage', 'compose'];

    function create(root, options) {
        const { tr, signal } = options, key = 'aurago:synth-studio:view:2';
        const LIMITS = { height: [160, 900] };
        let view = { panel: 'notes', height: 0, library: false, quickStrip: false, performTab: 'stage', noteExpanded: false };
        try {
            const saved = JSON.parse(localStorage.getItem(key));
            if (saved && typeof saved === 'object') view = { ...view, ...saved };
        } catch (_) { /* private mode */ }
        const clamp = (value, min, max) => Math.max(min, Math.min(max, Math.round(Number(value) || min)));
        view.height = Math.max(0, Math.min(LIMITS.height[1], Math.round(Number(view.height) || 0)));
        view.library = view.library === true;
        view.quickStrip = view.quickStrip === true;
        view.noteExpanded = view.noteExpanded === true;
        if (!MODES.includes(view.panel)) view.panel = 'notes';
        if (!PERFORM_TABS.includes(view.performTab)) view.performTab = 'stage';

        let hasClip = false, forcedMaster = false;

        const host = root.querySelector('[data-editor-host]'), note = host.querySelector('[data-note-editor]');
        const bar = document.createElement('nav');
        bar.className = 'ss-modes';
        bar.setAttribute('role', 'tablist');
        bar.setAttribute('aria-label', tr('modes'));
        for (const name of MODES) {
            const button = document.createElement('button');
            button.type = 'button';
            button.dataset.workTab = name;
            button.dataset.mode = MODE_NAME[name];
            button.setAttribute('role', 'tab');
            const mark = document.createElement('span');
            mark.className = 'ss-mode-mark';
            mark.setAttribute('aria-hidden', 'true');
            const label = document.createElement('span');
            label.textContent = tr('mode_' + MODE_NAME[name]);
            button.append(mark, label);
            bar.append(button);
        }
        const modesSlot = root.querySelector('[data-modes-slot]');
        if (modesSlot) modesSlot.append(bar);
        else root.insertBefore(bar, root.querySelector('[data-master-panel]') || root.querySelector('.ss-notice'));

        const grip = document.createElement('div');
        grip.className = 'ss-workbench-grip';
        grip.tabIndex = 0;
        grip.setAttribute('role', 'separator');
        grip.setAttribute('aria-orientation', 'horizontal');
        grip.setAttribute('aria-label', tr('resize_layout'));
        grip.setAttribute('aria-valuemin', String(LIMITS.height[0]));
        grip.setAttribute('aria-valuemax', String(LIMITS.height[1]));
        host.insertBefore(grip, note);
        note.dataset.workPanel = 'notes';

        const sound = document.createElement('section');
        sound.className = 'ss-sound-workbench';
        sound.dataset.workPanel = 'sound';
        const soundBody = document.createElement('div');
        soundBody.className = 'ss-sound-body';
        const soundRail = document.createElement('aside');
        soundRail.className = 'ss-sound-rail';
        soundRail.dataset.soundRail = '';
        soundRail.setAttribute('aria-label', tr('tracks'));
        const inspector = root.querySelector('[data-inspector]');
        soundBody.append(soundRail, inspector);
        sound.append(soundBody);
        host.append(sound);

        const performance = document.createElement('section');
        performance.className = 'ss-performance-workbench';
        performance.dataset.workPanel = 'performance';
        const performBar = document.createElement('div');
        performBar.className = 'ss-perform-bar';
        const performTabs = document.createElement('div');
        performTabs.className = 'ss-perform-tabs';
        performTabs.setAttribute('role', 'tablist');
        performTabs.setAttribute('aria-label', tr('mode_perform'));
        for (const tabId of PERFORM_TABS) {
            const tabBtn = document.createElement('button');
            tabBtn.type = 'button';
            tabBtn.dataset.performTab = tabId;
            tabBtn.setAttribute('role', 'tab');
            tabBtn.textContent = tr(tabId === 'stage' ? 'perform_stage' : 'perform_compose');
            performTabs.append(tabBtn);
        }
        const performHarmony = document.createElement('div');
        performHarmony.className = 'ss-perform-harmony';
        performHarmony.dataset.performHarmony = '';
        performBar.append(performTabs, performHarmony);

        const performPanels = document.createElement('div');
        performPanels.className = 'ss-perform-panels';
        const performStage = document.createElement('div');
        performStage.className = 'ss-perform-panel ss-stage';
        performStage.dataset.performPanel = 'stage';
        const performCompose = document.createElement('div');
        performCompose.className = 'ss-perform-panel';
        performCompose.dataset.performPanel = 'compose';
        performPanels.append(performStage, performCompose);
        performance.append(performBar, performPanels);
        host.append(performance);

        const mixer = document.createElement('section');
        mixer.className = 'ss-mixer-workbench';
        mixer.dataset.workPanel = 'mix';
        host.append(mixer);

        const strudel = document.createElement('section');
        strudel.className = 'ss-strudel-workbench';
        strudel.dataset.workPanel = 'strudel';
        host.append(strudel);

        const rb338 = document.createElement('section');
        rb338.className = 'ss-rb338-workbench';
        rb338.dataset.rb338Panel = '';
        rb338.hidden = true;
        root.insertBefore(rb338, root.querySelector('[data-midi-panel]') || root.querySelector('.ss-footer'));
        let rbActive = false;

        const save = () => { try { localStorage.setItem(key, JSON.stringify(view)); } catch (_) { /* private mode */ } };

        function workbenchHeight() {
            const available = host.clientHeight || 0;
            const max = Math.max(LIMITS.height[0], available - 98);
            if (view.noteExpanded && view.panel === 'notes' && hasClip) return max;
            if (view.height > 0) return Math.min(view.height, max);
            return Math.round(Math.min(Math.max(available * 0.46, LIMITS.height[0]), max));
        }

        function applyLayout() {
            const height = workbenchHeight();
            root.style.setProperty('--ss-workbench-height', height + 'px');
            grip.setAttribute('aria-valuenow', String(height));
        }

        function render() {
            const mode = MODE_NAME[view.panel];
            root.dataset.mode = mode;
            root.classList.toggle('ss-library-open', view.library);
            root.classList.toggle('ss-quick-open', view.quickStrip && view.panel === 'notes');
            root.classList.toggle('ss-has-clip', hasClip);
            root.classList.toggle('ss-note-expanded', view.noteExpanded);
            performance.dataset.activePerformTab = view.performTab;
            applyLayout();

            bar.querySelectorAll('button').forEach(b => {
                const on = b.dataset.workTab === view.panel;
                b.setAttribute('aria-selected', String(on));
                b.tabIndex = on ? 0 : -1;
            });

            performTabs.querySelectorAll('button').forEach(b => {
                const on = b.dataset.performTab === view.performTab;
                b.setAttribute('aria-selected', String(on));
                b.tabIndex = on ? 0 : -1;
            });
            performStage.hidden = view.performTab !== 'stage';
            performCompose.hidden = view.performTab !== 'compose';

            host.querySelectorAll('[data-work-panel]').forEach(p => {
                const shown = p.dataset.workPanel === view.panel && (p.dataset.workPanel !== 'notes' || hasClip);
                p.hidden = !shown;
                p.setAttribute('role', 'tabpanel');
            });
            grip.hidden = !(view.panel === 'notes' && hasClip);

            const quickStrip = root.querySelector('[data-quick-strip]');
            if (quickStrip) quickStrip.hidden = !(view.panel === 'notes' && view.quickStrip);
            root.querySelectorAll('[data-action="quickStrip"]').forEach(b => b.setAttribute('aria-pressed', String(view.quickStrip)));

            const master = root.querySelector('[data-master-panel]');
            if (master) {
                if (view.panel === 'mix') {
                    master.classList.add('is-docked');
                    if (master.hidden) { master.hidden = false; forcedMaster = true; }
                    if (started && typeof options.onMix === 'function') options.onMix();
                } else {
                    master.classList.remove('is-docked');
                    if (forcedMaster) { master.hidden = true; forcedMaster = false; }
                }
                const toggle = root.querySelector('[data-action="master"]');
                if (toggle) toggle.setAttribute('aria-expanded', String(!master.hidden));
            }

            const rbOn = view.panel === 'rb338';
            rb338.hidden = !rbOn;
            if (rbOn !== rbActive) {
                rbActive = rbOn;
                if (started && typeof options.onRb338 === 'function') options.onRb338(rbOn);
            }
            root.querySelectorAll('[data-action="library"]').forEach(b => b.setAttribute('aria-expanded', String(view.library)));
        }

        function panel(name) {
            if (!MODES.includes(name)) return;
            view.panel = name;
            render();
            save();
            if (typeof options.onPanel === 'function') options.onPanel(name);
        }

        function performTab(name) {
            if (!PERFORM_TABS.includes(name)) return;
            view.performTab = name;
            render();
            save();
            if (typeof options.onPerformTab === 'function') options.onPerformTab(name);
        }

        bar.addEventListener('click', e => {
            const b = e.target.closest('[data-work-tab]');
            if (b) panel(b.dataset.workTab);
        }, { signal });

        bar.addEventListener('keydown', e => {
            if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return;
            e.preventDefault();
            let i = MODES.indexOf(view.panel);
            if (i < 0) i = 0;
            i = e.key === 'Home' ? 0 : e.key === 'End' ? MODES.length - 1 : (i + (e.key === 'ArrowRight' ? 1 : MODES.length - 1)) % MODES.length;
            panel(MODES[i]);
            bar.querySelector('[data-work-tab="' + MODES[i] + '"]').focus();
        }, { signal });

        performTabs.addEventListener('click', e => {
            const b = e.target.closest('[data-perform-tab]');
            if (b) performTab(b.dataset.performTab);
        }, { signal });

        performTabs.addEventListener('keydown', e => {
            if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return;
            e.preventDefault();
            let i = PERFORM_TABS.indexOf(view.performTab);
            if (i < 0) i = 0;
            i = e.key === 'Home' ? 0 : e.key === 'End' ? PERFORM_TABS.length - 1 : (i + (e.key === 'ArrowRight' ? 1 : PERFORM_TABS.length - 1)) % PERFORM_TABS.length;
            performTab(PERFORM_TABS[i]);
            performTabs.querySelector('[data-perform-tab="' + PERFORM_TABS[i] + '"]').focus();
        }, { signal });

        // Note-editor height grip: drag, arrow keys, double click = default.
        let drag = null;
        const setHeight = value => {
            view.noteExpanded = false;
            view.height = clamp(value, LIMITS.height[0], LIMITS.height[1]);
            render();
        };
        grip.addEventListener('pointerdown', e => {
            if (e.button !== 0) return;
            drag = { origin: e.clientY, value: workbenchHeight() };
            grip.setPointerCapture(e.pointerId);
            grip.classList.add('is-dragging');
            e.preventDefault();
        }, { signal });
        grip.addEventListener('pointermove', e => { if (drag) setHeight(drag.value - (e.clientY - drag.origin)); }, { signal });
        grip.addEventListener('pointerup', () => { if (!drag) return; drag = null; grip.classList.remove('is-dragging'); save(); }, { signal });
        grip.addEventListener('pointercancel', () => { if (!drag) return; setHeight(drag.value); drag = null; grip.classList.remove('is-dragging'); }, { signal });
        grip.addEventListener('dblclick', () => { view.noteExpanded = false; view.height = 0; render(); save(); }, { signal });
        grip.addEventListener('keydown', e => {
            if (!['ArrowUp', 'ArrowDown'].includes(e.key)) return;
            e.preventDefault();
            setHeight(workbenchHeight() + (e.key === 'ArrowUp' ? 1 : -1) * 20 * (e.shiftKey ? 3 : 1));
            save();
        }, { signal });

        const welcome = document.createElement('div');
        welcome.className = 'ss-welcome';
        const title = document.createElement('strong');
        title.textContent = tr('start_loop');
        welcome.append(title);
        for (const name of ['electronic', 'chill', 'retro']) {
            const b = document.createElement('button');
            b.dataset.template = name;
            b.textContent = tr('template_' + name);
            b.type = 'button';
            welcome.append(b);
        }
        for (const action of ['new', 'open']) {
            const b = document.createElement('button');
            b.dataset.action = action;
            b.type = 'button';
            b.textContent = tr(action);
            welcome.append(b);
        }
        if (typeof options.onTour === 'function') {
            const b = document.createElement('button');
            b.dataset.action = 'tour';
            b.type = 'button';
            b.textContent = tr('tour');
            welcome.append(b);
        }
        root.querySelector('.ss-main').prepend(welcome);

        let started = false;
        render();
        started = true;

        if (typeof ResizeObserver === 'function') {
            const observer = new ResizeObserver(() => applyLayout());
            observer.observe(host);
            if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', () => observer.disconnect(), { once: true });
        }

        return {
            panel,
            currentPanel: () => view.panel,
            performTab,
            getPerformTab: () => view.performTab,
            performance,
            performHarmony,
            performStage,
            performCompose,
            sound,
            soundRail,
            mixer,
            strudel,
            rb338,
            welcome,
            isRb338: () => rbActive,
            setClip(value) { const next = !!value; if (next !== hasClip) { hasClip = next; render(); } },
            toggleLibrary(force) { view.library = typeof force === 'boolean' ? force : !view.library; render(); save(); },
            toggleQuickStrip(force) { view.quickStrip = typeof force === 'boolean' ? force : !view.quickStrip; render(); save(); },
            toggleNoteExpanded(force) { view.noteExpanded = typeof force === 'boolean' ? force : !view.noteExpanded; render(); save(); },
            isNoteExpanded: () => view.noteExpanded,
            refresh() {
                welcome.hidden = options.project().tracks.length > 0;
                welcome.querySelectorAll('[data-template]').forEach(b => b.disabled = options.blocked());
            }
        };
    }

    window.SynthStudioWorkspace = { create };
})();
