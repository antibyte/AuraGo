    let desktopMediaKeysWired = false;

    function webampMusicActive() {
        return !!(state.webampMusic && state.webampMusic.instance);
    }

    function dispatchWebampMediaAction(type) {
        const current = state.webampMusic;
        if (!current || !current.instance) return false;
        const store = current.instance.store;
        if (store && typeof store.dispatch === 'function') {
            try {
                store.dispatch({ type: type });
                return true;
            } catch (_) { /* fall through */ }
        }
        const instance = current.instance;
        const map = {
            PLAY: 'play',
            PAUSE: 'pause',
            NEXT: 'nextTrack',
            PREVIOUS: 'previousTrack',
            STOP: 'stop'
        };
        const method = map[type];
        if (method && typeof instance[method] === 'function') {
            try {
                instance[method]();
                return true;
            } catch (_) { /* ignore */ }
        }
        return false;
    }

    function updateWebampMediaSessionMetadata() {
        if (!('mediaSession' in navigator) || !webampMusicActive()) return;
        try {
            navigator.mediaSession.metadata = new MediaMetadata({
                title: t('desktop.app_music_player'),
                artist: 'AuraGo',
                album: t('desktop.winamp_tracks')
            });
        } catch (_) { /* ignore metadata errors */ }
    }

    function bindDesktopMediaSessionAction(action, handler) {
        if (!('mediaSession' in navigator)) return;
        try {
            navigator.mediaSession.setActionHandler(action, handler);
        } catch (_) { /* unsupported action */ }
    }

    function clearDesktopMediaSessionHandlers() {
        if (!('mediaSession' in navigator)) return;
        ['play', 'pause', 'previoustrack', 'nexttrack', 'stop'].forEach(action => {
            try {
                navigator.mediaSession.setActionHandler(action, null);
            } catch (_) { /* ignore */ }
        });
        try {
            navigator.mediaSession.metadata = null;
        } catch (_) { /* ignore */ }
    }

    function refreshDesktopMediaSessionHandlers() {
        if (!('mediaSession' in navigator)) return;
        if (!webampMusicActive()) {
            clearDesktopMediaSessionHandlers();
            return;
        }
        bindDesktopMediaSessionAction('play', () => { dispatchWebampMediaAction('PLAY'); });
        bindDesktopMediaSessionAction('pause', () => { dispatchWebampMediaAction('PAUSE'); });
        bindDesktopMediaSessionAction('stop', () => { dispatchWebampMediaAction('STOP'); });
        bindDesktopMediaSessionAction('previoustrack', () => { dispatchWebampMediaAction('PREVIOUS'); });
        bindDesktopMediaSessionAction('nexttrack', () => { dispatchWebampMediaAction('NEXT'); });
        updateWebampMediaSessionMetadata();
    }

    function handleDesktopMediaKeydown(event) {
        if (!webampMusicActive() || isEditableTarget(event.target)) return false;
        let type = '';
        switch (event.code) {
        case 'MediaPlayPause':
            type = 'PLAY';
            break;
        case 'MediaPause':
            type = 'PAUSE';
            break;
        case 'MediaTrackNext':
            type = 'NEXT';
            break;
        case 'MediaTrackPrevious':
            type = 'PREVIOUS';
            break;
        case 'MediaStop':
            type = 'STOP';
            break;
        default:
            return false;
        }
        event.preventDefault();
        dispatchWebampMediaAction(type === 'PLAY' ? 'PLAY' : type);
        return true;
    }

    function initDesktopMediaKeysRuntime() {
        if (desktopMediaKeysWired) return;
        desktopMediaKeysWired = true;
        refreshDesktopMediaSessionHandlers();
    }

    function notifyWebampMediaSessionChanged() {
        refreshDesktopMediaSessionHandlers();
    }

    function notifyWebampMediaSessionStopped() {
        clearDesktopMediaSessionHandlers();
    }
