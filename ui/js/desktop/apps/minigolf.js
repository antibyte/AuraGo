(function () {
    'use strict';
    const instances = new Map();
    const assetURL = path => window.AuraLazyAssets?.versionedURL(path) || path + '?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');
    function dispose(id) {
        const entry = instances.get(id);
        if (!entry) return;
        entry.cancelled = true;
        entry.game?.dispose();
        instances.delete(id);
    }
    async function render(host, id, context = {}) {
        dispose(id);
        const entry = { cancelled: false, game: null };
        instances.set(id, entry);
        const message = document.createElement('p');
        message.className = 'mg-loading';
        message.textContent = context.t?.('minigolf.loading') || 'AuraGolf';
        host.replaceChildren(message);
        try {
            const module = await import(assetURL('/js/vendor/minigolf/game.esm.js'));
            if (entry.cancelled) return;
            entry.game = module.mountGame(host, { ...context, assetURL });
        } catch (_) {
            if (entry.cancelled) return;
            message.textContent = context.t?.('minigolf.load_error') || 'AuraGolf';
            const retry = document.createElement('button');
            retry.textContent = context.t?.('minigolf.retry') || 'Retry';
            retry.addEventListener('click', () => render(host, id, context), { once: true });
            host.replaceChildren(message, retry);
        }
    }
    window.MinigolfApp = { render, dispose };
})();
