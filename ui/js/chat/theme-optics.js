/**
 * Theme Optics — chat page controller.
 *
 * Mounts one shared WebGL canvas behind the chat content and feeds it the
 * recipe of the active theme. The canvas lives in #chat-box, so it scrolls with
 * the conversation and stays behind the bubbles.
 *
 * Switching themes swaps the recipe in place; it never tears down the WebGL
 * context, so a theme switch stays cheap.
 */
(function () {
    'use strict';

    const CANVAS_ID = 'aurago-chat-optics';
    const CHAT_BOX_ID = 'chat-box';

    let renderer = null;
    let currentTheme = '';

    function activeTheme() {
        return document.documentElement.getAttribute('data-theme') || 'dark';
    }

    function chatBox() {
        return document.getElementById(CHAT_BOX_ID) || document.getElementById('chat-content');
    }

    function recipeFor(theme) {
        const recipes = window.AuraChatThemeOptics;
        if (!recipes || typeof recipes.recipeFor !== 'function') return null;
        return recipes.recipeFor(theme);
    }

    function ensureRenderer() {
        if (renderer) return renderer;
        if (!window.AuraThemeOptics || typeof window.AuraThemeOptics.createRenderer !== 'function') return null;
        const host = chatBox();
        if (!host) return null;
        renderer = window.AuraThemeOptics.createRenderer({
            container: host,
            canvasId: CANVAS_ID,
            maxDpr: 1.5,
            pinScroll: true
        });
        return renderer;
    }

    function sync() {
        const theme = activeTheme();
        currentTheme = theme;
        const recipe = recipeFor(theme);

        if (!recipe) {
            // The standard theme promises a plain surface, so it gets no canvas
            // at all — not a hidden one. The renderer and its WebGL context go
            // away with it.
            if (renderer) {
                renderer.dispose();
                renderer = null;
            }
            return;
        }

        const instance = ensureRenderer();
        if (!instance) return;
        instance.applyRecipe(recipe);
        instance.start();
    }

    function init() {
        sync();
        window.addEventListener('aurago:themechange', sync);
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init, { once: true });
    } else {
        init();
    }

    window.AuraChatThemeOpticsRuntime = {
        sync,
        getTheme: () => currentTheme,
        dispose() {
            if (renderer) renderer.dispose();
            renderer = null;
        }
    };
})();
