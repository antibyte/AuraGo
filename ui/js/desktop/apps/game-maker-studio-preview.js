(function () {
    'use strict';

    // Preview helpers for Game Maker Studio: loading overlay, stale badge,
    // fullscreen, and opening the sandboxed preview in a new tab.

    function handleMessage(state, event) {
        if (!state.frame || event.source !== state.frame.contentWindow) return;
        const data = event.data;
        if (!data || typeof data !== 'object' || data.channel !== state.channelID || data.source !== 'aurago-game') return;
        const allowed = new Set(['ready', 'runtime_error', 'resource_error', 'diagnostic', 'gameplay']);
        if (!allowed.has(data.type)) return;
        // Game-authored ready calls may precede rendering or describe a canvas
        // outside the viewport. Only the server boot's layout check qualifies.
        if (data.type === 'ready' && (data.boot !== true || data.visible !== true)) return;
        if (!state.project || state.previewProjectID !== state.project.id) return;
        if (data.type === 'gameplay') {
            if (!state.previewGrant?.validation_id || state.previewReported.has('gameplay')) return;
            if (!Array.isArray(data.observations) || data.observations.length > 16) return;
            const images = Array.isArray(data.images) ? data.images.filter(image => typeof image === 'string' && image.length <= 700000).slice(0, 2) : [];
            const payload = { token: state.previewGrant.token, type: 'gameplay', observations: data.observations, images };
            if (JSON.stringify(payload).length > 1500000) return;
            state.previewReported.add('gameplay');
            const grant = state.previewGrant;
            state.api.reportPreview(state.previewProjectID, payload).catch(error => {
                if (!state.disposed && state.previewGrant === grant) state.addDiagnostic({ level: 'error', message: error.message || String(error) });
            });
            return;
        }
        const message = data.type === 'ready' ? '' : String(data.message || data.type).slice(0, 1000);
        const key = data.type + ':' + message;
        if (state.previewReported.has(key) || state.previewReported.size >= 21) return;
        state.previewReported.add(key);
        if (state.previewGrant && state.previewGrant.validation_id) {
            const frame = state.frame;
            const grant = state.previewGrant;
            state.api.reportPreview(state.previewProjectID, {
                token: state.previewGrant.token, type: data.type, message, canvas_visible: data.type === 'ready'
            }).catch(error => {
                if (!state.disposed && state.previewGrant === grant && state.frame === frame && state.previewProjectID === state.project.id) {
                    state.addDiagnostic({ level: 'error', message: error.message || String(error) });
                }
            });
        }
        if (data.type === 'ready') {
            if (state.previewGrant?.scenarios?.length) {
                state.frame.contentWindow.postMessage({ source: 'aurago-studio', type: 'run-tests', channel: state.channelID, scenarios: state.previewGrant.scenarios }, '*');
            }
            clearLoading(state);
            return;
        }
        state.previewDiagnostics.push({ level: 'runtime', message });
        state.addDiagnostic({
            level: 'runtime',
            message
        });
    }

    function showLoading(state, shellEl, frame) {
        clearLoading(state);
        const overlay = document.createElement('div');
        overlay.className = 'gm-preview-loading';
        overlay.setAttribute('data-gm-preview-loading', 'true');
        overlay.innerHTML = `<span class="gm-job-spinner" aria-hidden="true"></span>
            <span>${state.context.esc(state.context.t('game_maker.preview_loading'))}</span>`;
        shellEl.appendChild(overlay);
        let timer = null;
        let settled = false;
        const clear = () => {
            if (settled) return;
            settled = true;
            overlay.remove();
            if (timer) clearTimeout(timer);
            if (state.previewLoadClear === clear) {
                state.previewLoadTimer = null;
                state.previewLoadClear = null;
            }
        };
        // Prefer the game's ready diagnostic; fall back to iframe load so a
        // hung/missing canvas does not leave the studio overlay forever.
        frame.addEventListener('load', () => setTimeout(clear, 600), { once: true });
        state.previewLoadClear = clear;
        timer = setTimeout(() => {
            const current = state.previewLoadClear === clear;
            clear();
            if (current) {
                state.addDiagnostic({ level: 'info', message: state.context.t('game_maker.preview_timeout') });
            }
        }, 12000);
        state.previewLoadTimer = timer;
    }

    function clearLoading(state) {
        if (state.previewLoadClear) {
            state.previewLoadClear();
            return;
        }
        if (state.previewLoadTimer) clearTimeout(state.previewLoadTimer);
        state.previewLoadTimer = null;
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

    window.GameMakerStudioPreview = { handleMessage, showLoading, clearLoading, updateStaleBadge, toggleFullscreen, openTab };
})();
