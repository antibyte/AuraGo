/**
 * Theme Optics — virtual desktop agent chat recipes.
 *
 * The desktop shell has its own theme set (body[data-theme]), separate from the
 * chat page themes. Every desktop theme gets its own light behaviour inside the
 * Agent Chat window, so the window feels like it belongs to the shell.
 *
 * "standard" and "fruity" stay close to neutral: the shell promises those two
 * a quiet look, and the neutrality check compares them against HEAD.
 */
(function () {
    'use strict';

    const RECIPES = {
        standard: {
            optics: 'aurora',
            palette: { primary: '#151b24', secondary: '#1e2734', accent: '#4a90c2' },
            intensity: 0.14,
            speed: 0.3,
            blend: 'screen'
        },
        fruity: {
            optics: 'aurora',
            palette: { primary: '#fdf3ee', secondary: '#ffe0e6', accent: '#ff8fa3' },
            intensity: 0.26,
            speed: 0.35,
            blend: 'multiply'
        },
        amiga: {
            optics: 'lattice',
            palette: { primary: '#0b3d91', secondary: '#66ccff', accent: '#ff9d00' },
            intensity: 0.6,
            speed: 0.55,
            blend: 'screen'
        },
        cyberpunk: {
            optics: 'rain',
            palette: { primary: '#0a0018', secondary: '#ff2bd6', accent: '#00f0ff' },
            intensity: 0.68,
            speed: 1.1,
            blend: 'screen'
        },
        black_gloss: {
            optics: 'facet',
            palette: { primary: '#08080a', secondary: '#2b2b32', accent: '#c8b28a' },
            intensity: 0.42,
            speed: 0.4,
            blend: 'screen'
        },
        crazy: {
            optics: 'plasma',
            palette: { primary: '#1b0033', secondary: '#ff00a8', accent: '#00ffb3' },
            intensity: 0.72,
            speed: 1.25,
            blend: 'screen'
        },
        lollipop: {
            optics: 'motes',
            palette: { primary: '#fff5fa', secondary: '#ffc0dd', accent: '#8be0ff' },
            intensity: 0.44,
            speed: 0.45,
            blend: 'multiply'
        },
        organic: {
            optics: 'caustic',
            palette: { primary: '#0d1a10', secondary: '#3f7d3a', accent: '#c7e07a' },
            intensity: 0.58,
            speed: 0.5,
            blend: 'screen'
        },
        electronic_dreams: {
            optics: 'plasma',
            palette: { primary: '#0b0620', secondary: '#7b5cff', accent: '#ff7ad9' },
            intensity: 0.66,
            speed: 0.9,
            blend: 'screen'
        },
        windows_xp: {
            optics: 'aurora',
            palette: { primary: '#dceaf7', secondary: '#a8d4f0', accent: '#f26b2c' },
            intensity: 0.3,
            speed: 0.4,
            blend: 'multiply'
        },
        green_slime: {
            optics: 'caustic',
            palette: { primary: '#07160a', secondary: '#39d353', accent: '#c6ff6b' },
            intensity: 0.64,
            speed: 0.6,
            blend: 'screen'
        },
        voxel_world: {
            optics: 'facet',
            palette: { primary: '#1b2a1a', secondary: '#7cb342', accent: '#f0f4c3' },
            intensity: 0.58,
            speed: 0.5,
            blend: 'screen'
        },
        orbit: {
            optics: 'motes',
            palette: { primary: '#04060f', secondary: '#4d7cff', accent: '#ffe08a' },
            intensity: 0.6,
            speed: 0.45,
            blend: 'screen'
        },
        minimal: {
            optics: 'aurora',
            palette: { primary: '#f7f7f8', secondary: '#eceef2', accent: '#9aa3b2' },
            intensity: 0.2,
            speed: 0.3,
            blend: 'multiply'
        }
    };

    function recipeFor(theme) {
        const key = String(theme || '').trim();
        return RECIPES[key] || null;
    }

    window.AuraDesktopChatOpticsRecipes = { recipes: RECIPES, recipeFor };
})();
