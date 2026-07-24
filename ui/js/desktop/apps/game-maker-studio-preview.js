(function () {
    'use strict';

    // Preview helpers for Game Maker Studio: loading overlay, stale badge,
    // fullscreen, and opening the sandboxed preview in a new tab.

    function showLoading(state, shellEl, frame) {
        const overlay = document.createElement('div');
        overlay.className = 'gm-preview-loading';
        overlay.innerHTML = `<span class="gm-job-spinner" aria-hidden="true"></span>
            <span>${state.context.esc(state.context.t('game_maker.preview_loading'))}</span>`;
        shellEl.appendChild(overlay);
        const clear = () => {
            overlay.remove();
            clearLoading(state);
        };
        frame.addEventListener('load', () => setTimeout(clear, 400), { once: true });
        state.previewLoadClear = clear;
        state.previewLoadTimer = setTimeout(() => {
            clear();
            state.addDiagnostic({ level: 'info', message: state.context.t('game_maker.preview_timeout') });
        }, 15000);
    }

    function clearLoading(state) {
        if (state.previewLoadTimer) clearTimeout(state.previewLoadTimer);
        state.previewLoadTimer = null;
        state.previewLoadClear = null;
    }

    function updateStaleBadge(state) {
        const shellEl = state.container.querySelector('[data-gm-preview]');
        if (!shellEl) return;
        let badge = shellEl.querySelector('.gm-preview-stale');
        const stale = Boolean(state.jobActive && state.previewStale && state.frame);
        if (stale && !badge) {
            badge = document.createElement('span');
            badge.className = 'gm-preview-stale';
            shellEl.appendChild(badge);
        }
        if (badge) {
            badge.hidden = !stale;
            badge.textContent = state.context.t('game_maker.preview_stale');
        }
    }

    function toggleFullscreen(state) {
        const shellEl = state.container.querySelector('[data-gm-preview]');
        if (!shellEl || !shellEl.requestFullscreen) return;
        if (document.fullscreenElement) document.exitFullscreen();
        else shellEl.requestFullscreen();
    }

    async function openTab(state) {
        if (!state.project) return;
        try {
            const grant = await state.api.previewGrant(state.project.id);
            if (state.disposed) return;
            window.open(grant.url, '_blank', 'noopener');
        } catch (error) {
            state.fail(error);
        }
    }

    window.GameMakerStudioPreview = { showLoading, clearLoading, updateStaleBadge, toggleFullscreen, openTab };
})();
