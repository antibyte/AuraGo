(function () {
    'use strict';
    const MODES = ['notes', 'sound', 'performance', 'strudel', 'mix'];
    const MODE_NAME = { notes: 'arrange', sound: 'sound', performance: 'perform', strudel: 'strudel', mix: 'mix' };
    function create(root, options) {
        const { tr, signal } = options, key = 'aurago:synth-studio:view:2';
        const LIMITS = { height: [160, 900] };
        let view = { panel: 'notes', height: 0, library: false };
        try { const saved = JSON.parse(localStorage.getItem(key)); if (saved) view = { ...view, ...saved }; } catch (_) { /* private mode */ }
        const clamp = (value, min, max) => Math.max(min, Math.min(max, Math.round(Number(value) || min)));
        view.height = Math.max(0, Math.min(LIMITS.height[1], Math.round(Number(view.height) || 0)));
        view.library = view.library === true;
        if (!MODES.includes(view.panel)) view.panel = 'notes';
        let hasClip = false, forcedMaster = false;

        const host = root.querySelector('[data-editor-host]'), note = host.querySelector('[data-note-editor]');
        const bar = document.createElement('nav');
        bar.className = 'ss-modes'; bar.setAttribute('role', 'tablist'); bar.setAttribute('aria-label', tr('modes'));
        for (const name of MODES) {
            const button = document.createElement('button');
            button.type = 'button'; button.dataset.workTab = name; button.dataset.mode = MODE_NAME[name];
            button.setAttribute('role', 'tab');
            const mark = document.createElement('span'); mark.className = 'ss-mode-mark'; mark.setAttribute('aria-hidden', 'true');
            const label = document.createElement('span'); label.textContent = tr('mode_' + MODE_NAME[name]);
            button.append(mark, label); bar.append(button);
        }
        root.insertBefore(bar, root.querySelector('[data-master-panel]') || root.querySelector('.ss-notice'));

        const grip = document.createElement('div'); grip.className = 'ss-workbench-grip'; grip.tabIndex = 0;
        grip.setAttribute('role', 'separator'); grip.setAttribute('aria-orientation', 'horizontal'); grip.setAttribute('aria-label', tr('resize_layout'));
        grip.setAttribute('aria-valuemin', String(LIMITS.height[0])); grip.setAttribute('aria-valuemax', String(LIMITS.height[1]));
        host.insertBefore(grip, note);
        note.dataset.workPanel = 'notes';
        const sound = document.createElement('section'); sound.className = 'ss-sound-workbench'; sound.dataset.workPanel = 'sound';
        sound.append(root.querySelector('[data-inspector]')); host.append(sound);
        const performance = document.createElement('section'); performance.className = 'ss-performance-workbench'; performance.dataset.workPanel = 'performance'; host.append(performance);
        const strudel = document.createElement('section'); strudel.className = 'ss-strudel-workbench'; strudel.dataset.workPanel = 'strudel'; host.append(strudel);

        const save = () => { try { localStorage.setItem(key, JSON.stringify(view)); } catch (_) { /* private mode */ } };
        function workbenchHeight() {
            const available = host.clientHeight || 0;
            const max = Math.max(LIMITS.height[0], available - 170);
            if (view.height > 0) return Math.min(view.height, max);
            return Math.round(Math.min(Math.max(available * .45, LIMITS.height[0]), max));
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
            root.classList.toggle('ss-has-clip', hasClip);
            applyLayout();
            bar.querySelectorAll('button').forEach(b => { const on = b.dataset.workTab === view.panel; b.setAttribute('aria-selected', String(on)); b.tabIndex = on ? 0 : -1; });
            host.querySelectorAll('[data-work-panel]').forEach(p => {
                const shown = p.dataset.workPanel === view.panel && (p.dataset.workPanel !== 'notes' || hasClip);
                p.hidden = !shown; p.setAttribute('role', 'tabpanel');
            });
            grip.hidden = !(view.panel === 'notes' && hasClip);
            const master = root.querySelector('[data-master-panel]');
            if (master) {
                if (view.panel === 'mix') {
                    if (master.hidden) { master.hidden = false; forcedMaster = true; }
                    if (started && typeof options.onMix === 'function') options.onMix();
                } else if (forcedMaster) { master.hidden = true; forcedMaster = false; }
            }
            root.querySelectorAll('[data-action="library"]').forEach(b => b.setAttribute('aria-expanded', String(view.library)));
        }
        function panel(name) { if (!MODES.includes(name)) return; view.panel = name; render(); save(); }
        bar.addEventListener('click', e => { const b = e.target.closest('[data-work-tab]'); if (b) panel(b.dataset.workTab); }, { signal });
        bar.addEventListener('keydown', e => {
            if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return;
            e.preventDefault();
            let i = MODES.indexOf(view.panel); if (i < 0) i = 0;
            i = e.key === 'Home' ? 0 : e.key === 'End' ? MODES.length - 1 : (i + (e.key === 'ArrowRight' ? 1 : MODES.length - 1)) % MODES.length;
            panel(MODES[i]); bar.querySelector('[data-work-tab="' + MODES[i] + '"]').focus();
        }, { signal });

        // The note-editor height grip: drag, arrow keys, double click = default.
        let drag = null;
        const setHeight = value => { view.height = clamp(value, LIMITS.height[0], LIMITS.height[1]); render(); };
        grip.addEventListener('pointerdown', e => {
            if (e.button !== 0) return;
            drag = { origin: e.clientY, value: workbenchHeight() };
            grip.setPointerCapture(e.pointerId); grip.classList.add('is-dragging'); e.preventDefault();
        }, { signal });
        grip.addEventListener('pointermove', e => { if (drag) setHeight(drag.value - (e.clientY - drag.origin)); }, { signal });
        grip.addEventListener('pointerup', () => { if (!drag) return; drag = null; grip.classList.remove('is-dragging'); save(); }, { signal });
        grip.addEventListener('pointercancel', () => { if (!drag) return; setHeight(drag.value); drag = null; grip.classList.remove('is-dragging'); }, { signal });
        grip.addEventListener('dblclick', () => { view.height = 0; render(); save(); }, { signal });
        grip.addEventListener('keydown', e => {
            if (!['ArrowUp', 'ArrowDown'].includes(e.key)) return;
            e.preventDefault();
            setHeight(workbenchHeight() + (e.key === 'ArrowUp' ? 1 : -1) * 20 * (e.shiftKey ? 3 : 1)); save();
        }, { signal });

        const welcome = document.createElement('div'); welcome.className = 'ss-welcome';
        const title = document.createElement('strong'); title.textContent = tr('start_loop'); welcome.append(title);
        for (const name of ['electronic', 'chill', 'retro']) { const b = document.createElement('button'); b.dataset.template = name; b.textContent = tr('template_' + name); b.type = 'button'; welcome.append(b); }
        for (const action of ['new', 'open']) { const b = document.createElement('button'); b.dataset.action = action; b.type = 'button'; b.textContent = tr(action); welcome.append(b); }
        if (typeof options.onTour === 'function') { const b = document.createElement('button'); b.dataset.action = 'tour'; b.type = 'button'; b.textContent = tr('tour'); welcome.append(b); }
        root.querySelector('.ss-main').prepend(welcome);
        // The app builds its mixer after this module; its own first refresh() fills the master panel.
        let started = false;
        render();
        started = true;
        if (typeof ResizeObserver === 'function') {
            const observer = new ResizeObserver(() => applyLayout());
            observer.observe(host);
            if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', () => observer.disconnect(), { once: true });
        }
        return {
            panel, performance, sound, strudel, welcome,
            setClip(value) { const next = !!value; if (next !== hasClip) { hasClip = next; render(); } },
            toggleLibrary() { view.library = !view.library; render(); save(); },
            refresh() { welcome.hidden = options.project().tracks.length > 0; welcome.querySelectorAll('[data-template]').forEach(b => b.disabled = options.blocked()); }
        };
    }
    window.SynthStudioWorkspace = { create };
})();
