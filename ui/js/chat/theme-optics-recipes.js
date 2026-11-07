/**
 * Theme Optics — chat page recipes.
 *
 * Every chat theme gets an optical backdrop: the light behaviour that belongs
 * to that theme. The recipes are intentionally declarative — a palette, an
 * optics kind, a speed, a strength and a blend mode — so the shared engine
 * stays generic and a new theme only needs one recipe, not a new shader.
 *
 * Two rules shape the table:
 *
 *   1. A theme that already owns a full scene (ocean, sandstorm, galaxy,
 *      threedee, ...) gets optics that COMPLEMENT that scene instead of
 *      repeating it: ocean shows marine snow, not a second set of caustics.
 *
 *   2. The Standard theme ("dark") gets no backdrop at all. Its stylesheet
 *      states the contract outright — "no noise, orbs, tint or logo" — and the
 *      neutral look is deliberate, so the standard chat stays plain.
 */
(function () {
    'use strict';

    const RECIPES = {
        // Light family: airy, paper-like washes.
        'light': {
            optics: 'aurora',
            palette: { primary: '#e8eef6', secondary: '#f2e6dc', accent: '#c98a5e' },
            intensity: 0.26,
            speed: 0.4,
            blend: 'multiply'
        },
        'retro-crt': {
            optics: 'lattice',
            palette: { primary: '#0d1a0b', secondary: '#39ff5a', accent: '#9dffbe' },
            intensity: 0.5,
            speed: 0.75,
            blend: 'screen'
        },
        '8bit': {
            optics: 'lattice',
            palette: { primary: '#1b1030', secondary: '#7b4dff', accent: '#ffd23f' },
            intensity: 0.46,
            speed: 0.6,
            blend: 'screen'
        },
        'cyberwar': {
            optics: 'rain',
            palette: { primary: '#041018', secondary: '#22e0ff', accent: '#ff3b6b' },
            intensity: 0.55,
            speed: 1.15,
            blend: 'screen'
        },
        'lollipop': {
            optics: 'facet',
            palette: { primary: '#fff0f6', secondary: '#ffb3d1', accent: '#ffe27a' },
            intensity: 0.34,
            speed: 0.5,
            blend: 'multiply'
        },
        'dark-sun': {
            optics: 'aurora',
            palette: { primary: '#2a0d12', secondary: '#ff6a1f', accent: '#ffc46b' },
            intensity: 0.34,
            speed: 0.28,
            blend: 'screen'
        },
        'ocean': {
            optics: 'motes',
            palette: { primary: '#04283c', secondary: '#1fb6c9', accent: '#9ff2e4' },
            intensity: 0.5,
            speed: 0.4,
            blend: 'screen'
        },
        'sandstorm': {
            optics: 'facet',
            palette: { primary: '#3a2a15', secondary: '#e0a45c', accent: '#f6d08a' },
            intensity: 0.42,
            speed: 0.45,
            blend: 'screen'
        },
        'papyrus': {
            optics: 'motes',
            palette: { primary: '#f3e7cd', secondary: '#e2bd72', accent: '#2f7f73' },
            intensity: 0.34,
            speed: 0.3,
            blend: 'multiply'
        },
        'threedee': {
            optics: 'facet',
            palette: { primary: '#101a2e', secondary: '#4d7cff', accent: '#8be9fd' },
            intensity: 0.4,
            speed: 0.55,
            blend: 'screen'
        },
        'galaxy': {
            optics: 'plasma',
            palette: { primary: '#060a1e', secondary: '#6a5cff', accent: '#ffd166' },
            intensity: 0.44,
            speed: 0.4,
            blend: 'screen'
        },
        'black-matrix': {
            optics: 'plasma',
            palette: { primary: '#020804', secondary: '#3ef07a', accent: '#d8ffe6' },
            intensity: 0.42,
            speed: 0.7,
            blend: 'screen'
        }
    };

    function recipeFor(theme) {
        const key = String(theme || '').trim();
        return RECIPES[key] || null;
    }

    window.AuraChatThemeOptics = { recipes: RECIPES, recipeFor };
})();
