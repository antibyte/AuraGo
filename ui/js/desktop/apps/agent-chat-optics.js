/**
 * Theme Optics — virtual desktop agent chat controller.
 *
 * The Agent Chat window lives inside the desktop shell, so it follows the
 * desktop theme (body[data-theme]), not the chat page themes. This controller
 * mounts one WebGL canvas into the window's log area and swaps the recipe when
 * the shell switches theme.
 *
 * The canvas sits behind the messages: the log has a background of its own, so
 * the optics read as light in the room rather than on top of the text.
 */
(function () {
    'use strict';

    const CANVAS_ID = 'aurago-desktop-chat-optics';

    function activeTheme() {
        return document.body && document.body.dataset ? (document.body.dataset.theme || '') : '';
    }

    function recipeFor(theme) {
        const recipes = window.AuraDesktopChatOpticsRecipes;
        if (!recipes || typeof recipes.recipeFor !== 'function') return null;
        return recipes.recipeFor(theme);
    }

    /**
     * Mounts the optics canvas into a chat window host.
     * Returns a handle with dispose(); safe to call when WebGL is missing.
     */
    function mount(host) {
        if (!host || typeof host.appendChild !== 'function') return null;
        if (!window.AuraThemeOptics || typeof window.AuraThemeOptics.createRenderer !== 'function') return null;

        const log = host.querySelector('.vd-chat-log') || host;
        const renderer = window.AuraThemeOptics.createRenderer({
            container: log,
            canvasId: CANVAS_ID,
            className: 'vd-chat-optics-canvas',
            maxDpr: 1.25,
            pinScroll: true
        });
        if (!renderer) return null;

        renderer.canvas.classList.add('vd-chat-optics-canvas');

        let current = '';
        function sync() {
            const theme = activeTheme();
            if (theme === current) return;
            current = theme;
            const recipe = recipeFor(theme);
            if (!recipe) {
                renderer.applyRecipe(null);
                renderer.stop();
                return;
            }
            renderer.applyRecipe(recipe);
            renderer.start();
        }

        sync();

        // The shell switches themes by writing body[data-theme].
        const observer = new MutationObserver(() => sync());
        observer.observe(document.body, { attributes: true, attributeFilter: ['data-theme'] });

        return {
            dispose() {
                observer.disconnect();
                renderer.dispose();
            }
        };
    }

    window.AuraDesktopChatOptics = { mount };
})();
