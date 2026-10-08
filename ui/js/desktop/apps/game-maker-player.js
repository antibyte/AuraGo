(async function () {
    'use strict';
    const data = JSON.parse(document.getElementById('game-player-data').textContent);
    const status = document.getElementById('status');
    const label = key => data.i18n['game_maker.'+key] || data.i18n.game_maker?.[key];
    const api = window.GameMakerStudioAPI.create(async (url, options) => {
        const response = await fetch(url, {...options, credentials:'same-origin', cache:'no-store'});
        if (!response.ok) {
            const error = new Error('Game player unavailable');
            error.status = response.status;
            throw error;
        }
        return response.json();
    });
    status.textContent = label('preview_loading') || '…';
    try {
        const {project} = await api.getProject(data.projectID);
        const grant = await api.previewGrant(data.projectID);
        const frame = document.createElement('iframe');
        frame.title = project.name;
        frame.setAttribute('sandbox','allow-scripts allow-pointer-lock');
        frame.setAttribute('allowfullscreen',''); frame.referrerPolicy = 'no-referrer';
        const state = {api,project,previewProjectID:project.id,previewGrant:grant,frame,channelID:crypto.getRandomValues(new Uint32Array(4)).join('-'),
            // A revision-bound GET conflict can happen during initial loading
            // or after the visible Load latest action. Refresh the trusted host
            // for a new grant; same-revision CAS recovery stays in the iframe.
            resolvePlayStateRevisionConflict:()=>window.location.reload()};
        window.addEventListener('message', event => window.GameMakerStudioPreview.handlePlayState(state,event));
        document.addEventListener('visibilitychange', () => frame.contentWindow?.postMessage({type:'aurago:game:active',active:!document.hidden},'*'));
        window.addEventListener('pagehide', () => window.GameMakerStudioPreview.flush(state));
        frame.addEventListener('load', () => { status.hidden = true; });
        document.querySelector('main').append(frame);
        frame.src = grant.url + '#gm-channel=' + encodeURIComponent(state.channelID);
    } catch (error) {
        status.setAttribute('role','alert');
        const timedOut = error?.name === 'TimeoutError' || error?.status === 408 || error?.status === 504;
        status.textContent = label(timedOut ? 'preview_timeout' : 'modules_load_failed') || '';
    }
})();
