(function () {
    'use strict';

    // Public progress only: model deltas, reasoning, tool arguments and diagnostics
    // never enter this decorative terminal. The normal status UI stays accessible.
    const phases = new Set(['queued', 'planning', 'building', 'validating', 'polishing', 'cancelling']);
    const tools = {
        game_maker_project: 'planning', game_maker_file: 'files',
        game_maker_asset: 'assets', game_maker_validate: 'validating',
        activate_agent_skill: 'skills'
    };

    function create(state) {
        const shell = state.container.querySelector('[data-gm-preview]');
        const placeholder = shell.innerHTML;
        const terminal = document.createElement('div');
        terminal.className = 'gm-build-terminal';
        terminal.setAttribute('aria-hidden', 'true');
        terminal.innerHTML = '<div class="gm-build-lines"></div><div class="gm-build-prompt"><span class="gm-build-cursor">▌</span><span></span></div>';
        const lines = terminal.firstElementChild;
        const prompt = terminal.lastElementChild.lastElementChild;
        const motion = matchMedia('(prefers-reduced-motion: reduce)');
        const reducedMotion = () => motion.matches || document.body.dataset.animations === 'false';
        let timer = null, disposed = false, visible = true, jobID = '', lastID = 0;
        let queue = [], current = null, lastText = '', lastPhase = '', sequence = 0, stoppedAt = '';
        const t = key => state.context.t('game_maker.' + key);
        const text = value => String(value || '').replace(/[\u0000-\u001f\u007f]/g, ' ').slice(0, 240);
        const elapsed = () => {
            const seconds = Math.max(0, Math.floor((Date.now() - (state.jobStartedAt || Date.now())) / 1000));
            return Math.floor(seconds / 60).toString().padStart(2, '0') + ':' + (seconds % 60).toString().padStart(2, '0');
        };
        const running = () => !disposed && !state.disposed && phases.has(state.job?.status);
        const clearTimer = () => { if (timer !== null) clearTimeout(timer); timer = null; };

        function clear() {
            clearTimer();
            queue = []; current = null; lastText = ''; lastPhase = ''; sequence = 0; lastID = 0; stoppedAt = '';
            lines.replaceChildren(); prompt.textContent = ''; terminal.remove();
        }

        function add(value) {
            const clean = text(value);
            if (!clean || clean === lastText) return;
            lastText = clean;
            queue.push(clean);
            queue = queue.slice(-32);
        }

        function tick() {
            timer = null;
            if (!running() || !visible || document.hidden || !terminal.isConnected) return;
            terminal.classList.toggle('is-still', reducedMotion());
            prompt.textContent = t(state.reconnecting ? 'status_reconnecting' : 'terminal_waiting') + ' · ' + elapsed();
            if (!current && queue.length) {
                const row = document.createElement('div');
                row.className = 'gm-build-line';
                row.dataset.line = (++sequence).toString().padStart(2, '0');
                lines.appendChild(row);
                while (lines.children.length > 24) lines.firstElementChild.remove();
                current = { row, chars: Array.from(queue.shift()), count: 0 };
            }
            if (current) {
                current.count = reducedMotion() ? current.chars.length : Math.min(current.chars.length, current.count + 3);
                current.row.textContent = current.chars.slice(0, current.count).join('');
                lines.scrollTop = lines.scrollHeight;
                if (current.count === current.chars.length) current = null;
            }
            timer = setTimeout(tick, current ? 36 : queue.length ? 380 : 1000);
        }

        function finish() {
            clearTimer();
            if (current) { current.row.textContent = current.chars.join(''); current = null; }
            for (const value of queue) {
                const row = document.createElement('div');
                row.className = 'gm-build-line'; row.dataset.line = (++sequence).toString().padStart(2, '0');
                row.textContent = value; lines.appendChild(row);
            }
            queue = [];
            while (lines.children.length > 24) lines.firstElementChild.remove();
            if (!stoppedAt) stoppedAt = elapsed();
            prompt.textContent = t('status_' + state.job.status) + ' · ' + stoppedAt;
            terminal.classList.add('is-still');
            lines.scrollTop = lines.scrollHeight;
        }

        function sync() {
            if (disposed) return;
            const nextJob = state.job?.id || '';
            if (nextJob !== jobID) { clear(); jobID = nextJob; }
            const phase = state.job?.phase || state.job?.status;
            if (running() && phase !== lastPhase && phases.has(phase)) {
                lastPhase = phase;
                add(t('terminal_' + phase));
            }
            const empty = shell.querySelector('.gm-preview-empty');
            const stopped = ['failed', 'cancelled'].includes(state.job?.status) && !!lastPhase;
            if ((!running() && !stopped) || !empty) {
                clearTimer(); terminal.remove(); shell.classList.remove('has-build-terminal');
                return;
            }
            if (!terminal.isConnected) empty.prepend(terminal);
            shell.classList.add('has-build-terminal');
            if (stopped) { finish(); return; }
            if (visible && !document.hidden && timer === null) tick();
        }

        function event(event) {
            if (disposed || (event.project_id && event.project_id !== state.project?.id)) return;
            sync();
            if (!running() || (event.job_id && event.job_id !== jobID)) return;
            const id = Number(event.id || 0);
            if (id && id <= lastID) return;
            if (id) lastID = id;
            const p = event.payload || {};
            switch (event.type) {
            case 'model_progress':
                if (['waiting', 'receiving', 'retrying', 'recovering'].includes(p.status)) add(t('terminal_model_' + p.status) + ' · ' + elapsed());
                break;
            case 'tool_call': add(t('terminal_' + (tools[p.tool] || 'tool'))); break;
            case 'file_changed': add(t('updated') + ' > ' + text(p.path)); break;
            case 'asset_changed': add(t('assets') + ' > ' + text(p.path || p.kind)); break;
            case 'skill_activation': add(t('skill_loaded')); break;
            case 'validation_result':
                for (const kind of ['gameplay', 'rules']) {
                    const status = p.result?.[kind + '_status'];
                    if (['passed', 'failed', 'unverified', 'unavailable'].includes(status)) add(t(kind + '_checks') + ' > ' + t('check_' + status));
                }
                break;
            case 'visual_progress':
                if (['capturing', 'analyzing', 'repairing'].includes(p.status)) add(t('visual_' + p.status));
                break;
            }
        }

        function reset() {
            clear(); jobID = '';
            shell.classList.remove('has-build-terminal');
            shell.innerHTML = placeholder;
        }

        const onVisibility = () => { clearTimer(); sync(); };
        const observer = new IntersectionObserver(entries => {
            visible = entries[0]?.isIntersecting === true;
            onVisibility();
        });
        observer.observe(shell);
        const resize = new ResizeObserver(() => { lines.scrollTop = lines.scrollHeight; });
        resize.observe(lines);
        document.addEventListener('visibilitychange', onVisibility);
        motion.addEventListener('change', onVisibility);
        return {
            sync, event, reset,
            dispose() {
                disposed = true; clear(); observer.disconnect(); resize.disconnect();
                shell.classList.remove('has-build-terminal');
                document.removeEventListener('visibilitychange', onVisibility);
                motion.removeEventListener('change', onVisibility);
            }
        };
    }

    window.GameMakerStudioActivity = { create };
})();
