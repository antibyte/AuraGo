(function () {
    'use strict';

    // Preview helpers for Game Maker Studio: loading overlay, stale badge,
    // fullscreen, and opening the sandboxed preview in a new tab.


    function handleMessage(state, event) {
        if (!state.frame || event.source !== state.frame.contentWindow) return;
        const data = event.data;
        if (data?.source === 'aurago-voxel') { handlePlayState(state, event); return; }
        if (!data || typeof data !== 'object' || data.channel !== state.channelID || data.source !== 'aurago-game') return;
        if (!state.project || state.previewProjectID !== state.project.id) return;
        if (data.type === 'ready') { setSceneDebug(state, state.sceneDebug); clearLoading(state); return; }
        if (!['runtime_error', 'resource_error', 'diagnostic'].includes(data.type)) return;
        const message = String(data.message || data.type).slice(0, 2000);
        const key = data.type + ':' + message;
        if (state.previewReported.has(key) || state.previewReported.size >= 200) return;
        state.previewReported.add(key);
        state.previewDiagnostics.push({ level: 'runtime', message });
        state.addDiagnostic({ level: 'runtime', message });
    }

    // Only the current opaque iframe can use its parent's narrowly scoped save grant.
    async function handlePlayState(state, event) {
        const data = event.data, grant = state.previewGrant, frame = state.frame, project = state.project;
        if (state.disposed || !frame || event.source !== frame.contentWindow || data?.source !== 'aurago-voxel'
            || data.type !== 'play_state' || data.channel !== state.channelID || project?.variant !== 'voxel'
            || state.previewProjectID !== project.id || !grant || !['load','save','reset'].includes(data.operation)
            || typeof data.request !== 'string' || data.request.length > 96 || !data.request.length) return;
        const reply = result => frame.contentWindow?.postMessage({source:'aurago-voxel-host',type:'play_state',channel:data.channel,request:data.request,...result}, '*');
        if (!grant.play_token || grant.validation_id) {
            reply(data.operation === 'load' ? {result:{temporary:true,version:0,state:null}} : {error:'saveError'}); return;
        }
        // A conflict returned by a load GET means the revision-bound grant is
        // stale. Keep that distinct from a save CAS conflict: the runtime's
        // explicit Load latest action can safely resolve the latter in place.
        if (data.operation === 'load' && state.playStateRevisionInvalid) {
            reply({error:'conflict'});
            try { await state.resolvePlayStateRevisionConflict?.(); } catch (_) {}
            return;
        }
        if (grant.revision !== project.current_revision) {
            if (data.operation === 'load') {
                state.playStateRevisionInvalid = true;
                reply({error:'conflict'});
                try { await state.resolvePlayStateRevisionConflict?.(); } catch (_) {}
            } else {
                reply({error:'conflict'});
            }
            return;
        }
        let size;
        try { size = new TextEncoder().encode(JSON.stringify({version:data.version,state:data.payload})).length; } catch (_) { reply({error:'saveError'}); return; }
        if (!Number.isSafeInteger(data.version) || data.version < 0 || state.playStateBusy || size > 4*1024*1024) { reply({error:'saveError'}); return; }
        state.playStateBusy = true;
        let recoverRevisionConflict = false;
        try {
            const result = await state.api.playState(project.id, grant.play_token, data.operation, data.version, data.payload);
            if (!state.disposed && state.frame === frame && state.previewGrant === grant) {
                if (data.operation === 'load') state.playStateRevisionInvalid = false;
                reply({result});
            }
        } catch (error) {
            if (!state.disposed && state.frame === frame && state.previewGrant === grant) {
                if (data.operation === 'load' && error.code === 'conflict') {
                    state.playStateRevisionInvalid = true;
                    reply({error:'conflict'});
                    recoverRevisionConflict = true;
                } else {
                    reply({error:error.code === 'conflict' ? 'conflict' : 'saveError'});
                }
            }
        } finally {
            if (state.frame === frame && state.previewGrant === grant) state.playStateBusy = false;
        }
        if (recoverRevisionConflict && !state.disposed && state.frame === frame && state.previewGrant === grant) {
            try { await state.resolvePlayStateRevisionConflict?.(); } catch (_) {}
        }
    }

    function flush(state) {
        if (!state.frame || state.project?.variant !== 'voxel' || !state.previewGrant?.play_token) return Promise.resolve(true);
        const frame = state.frame, channel = state.channelID;
        return new Promise(resolve => {
            const timer = setTimeout(() => done(false), 11000);
            function done(ok) { clearTimeout(timer); window.removeEventListener('message', reply); resolve(ok); }
            function reply(event) { if (event.source === frame.contentWindow && event.data?.source === 'aurago-voxel'
                && event.data.type === 'flushed' && event.data.channel === channel) done(event.data.ok === true); }
            window.addEventListener('message', reply);
            frame.contentWindow?.postMessage({source:'aurago-voxel-host',type:'flush',channel}, '*');
        });
    }

    function showAgentCapture(state, capture) {
        if (!capture || typeof capture.url !== 'string' || !state.project) return;
        const prefix = '/api/game-maker/projects/' + encodeURIComponent(state.project.id) + '/agent-captures/';
        if (!capture.url.startsWith(prefix)) return;
        const panel = state.container.querySelector('[data-gm-visual]');
        if (!panel) return;
        state.agentBuildID = capture.build_id;
        const image = document.createElement('img');
        image.alt = state.context.t('game_maker.agent_view');
        image.dataset.captureId = capture.id;
        image.title = image.alt + ' · ' + capture.build_id;
        image.src = capture.url;
        image.addEventListener('error', () => image.remove(), { once: true });
        panel.querySelector('[data-gm-visual-status]').textContent = image.alt;
        panel.hidden = false;
        panel.append(image);
        while (panel.querySelectorAll('img').length > 3) panel.querySelector('img').remove();
    }

    function cancelVisual(state) {
        // Agent images belong to the job and survive interactive iframe reloads.
        if (state.disposed) state.container.querySelector('[data-gm-visual]')?.replaceChildren();
    }

    function setSceneDebug(state, enabled) {
        state.sceneDebug = enabled === true;
        state.container.querySelector('[data-gm-action="scene_debug"]')?.setAttribute('aria-pressed', String(state.sceneDebug));
        if (!state.disposed && state.frame && state.channelID) state.frame.contentWindow.postMessage({ source: 'aurago-studio', type: 'scene-debug', channel: state.channelID, enabled: state.sceneDebug }, '*');
    }

    function showLoading(state, shellEl, frame) {
        cancelVisual(state);
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
        if (state.project.variant === 'voxel') { window.open(state.api.playURL(state.project.id), '_blank', 'noopener'); return; }
        // The tab is opened while the click still counts as a user gesture.
        // Waiting for the grant first would spend the gesture and the browser
        // would block the window without a word.
        const tab = window.open('', '_blank');
        if (tab) tab.opener = null;
        try {
            const grant = await state.api.previewGrant(state.project.id);
            if (state.disposed) { tab?.close(); return; }
            if (!tab) { state.fail(new Error(state.context.t('game_maker.tab_blocked'))); return; }
            tab.location.replace(grant.url);
        } catch (error) {
            tab?.close();
            state.fail(error);
        }
    }

    window.GameMakerStudioPreview = { flush, handlePlayState, showAgentCapture, cancelVisual, handleMessage, setSceneDebug, showLoading, clearLoading, updateStaleBadge, toggleFullscreen, openTab };
})();
