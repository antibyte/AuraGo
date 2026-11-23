// Chibi Sensei mascot: a small SVG sensei built with createElementNS, six moods, and the pure moodFor that picks a mood from the
// window state. The mood faces are drawn, never fetched. The mascot is decorative (aria-hidden); the tip text next to it carries
// the meaning. Classic script, publishes P.mascot.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const NS = 'http://www.w3.org/2000/svg';
    const MOODS = ['idle', 'thinking', 'happy', 'worried', 'sleeping', 'celebrating'];
    const BASE = [
        ['path', 'csn-mascot-body', { d: 'M12 62c0-13 9-21 20-21s20 8 20 21z' }],
        ['path', 'csn-mascot-belt', { d: 'M17 53h30' }],
        ['circle', 'csn-mascot-hair', { cx: 32, cy: 9, r: 5 }],
        ['circle', 'csn-mascot-face', { cx: 32, cy: 26, r: 17 }],
        ['path', 'csn-mascot-band', { d: 'M15.5 20.5c10-5.5 23-5.5 33 0l-.8 4.6c-10-4.8-21.6-4.8-31.4 0z' }],
        ['path', 'csn-mascot-band', { d: 'M48 21.5l9-4-2.4 7.6z' }]
    ];
    const EYES = [['ellipse', 'csn-mascot-eye', { cx: 25, cy: 29, rx: 2.2, ry: 2.8 }], ['ellipse', 'csn-mascot-eye', { cx: 39, cy: 29, rx: 2.2, ry: 2.8 }]];
    const JOY = [['path', 'csn-mascot-line', { d: 'M22 30q3-4 6 0' }], ['path', 'csn-mascot-line', { d: 'M36 30q3-4 6 0' }]];
    const CHEEKS = [['circle', 'csn-mascot-cheek', { cx: 21.5, cy: 34, r: 2.6 }], ['circle', 'csn-mascot-cheek', { cx: 42.5, cy: 34, r: 2.6 }]];
    const FACES = {
        idle: EYES.concat(CHEEKS, [['path', 'csn-mascot-line', { d: 'M28.5 36q3.5 2.6 7 0' }]]),
        thinking: [['ellipse', 'csn-mascot-eye', { cx: 26.5, cy: 27.5, rx: 2.1, ry: 2.6 }], ['ellipse', 'csn-mascot-eye', { cx: 40.5, cy: 27.5, rx: 2.1, ry: 2.6 }], ['path', 'csn-mascot-line', { d: 'M30 37h5' }],
            ['circle', 'csn-mascot-spark', { cx: 52, cy: 12, r: 1.4 }], ['circle', 'csn-mascot-spark', { cx: 56, cy: 7.5, r: 1.8 }], ['circle', 'csn-mascot-spark', { cx: 60.5, cy: 3, r: 2.2 }]],
        happy: JOY.concat(CHEEKS, [['path', 'csn-mascot-line', { d: 'M26.5 35q5.5 5.5 11 0' }]]),
        worried: EYES.concat([['path', 'csn-mascot-line', { d: 'M21.5 23.5l6 1.8' }], ['path', 'csn-mascot-line', { d: 'M42.5 23.5l-6 1.8' }], ['path', 'csn-mascot-line', { d: 'M27 38q2.5-2.2 5 0t5 0' }]]),
        sleeping: [['path', 'csn-mascot-line', { d: 'M22 30h6' }], ['path', 'csn-mascot-line', { d: 'M36 30h6' }], ['circle', 'csn-mascot-line', { cx: 32, cy: 37, r: 1.6 }],
            ['path', 'csn-mascot-zzz', { d: 'M49 13h5l-5 5h5' }], ['path', 'csn-mascot-zzz', { d: 'M56 5h4l-4 4h4' }]],
        celebrating: JOY.concat(CHEEKS, [['path', 'csn-mascot-mouth', { d: 'M26 34.5q6 8 12 0z' }],
            ['rect', 'csn-mascot-spark', { x: 6, y: 8, width: 3, height: 3, transform: 'rotate(20 7.5 9.5)' }], ['rect', 'csn-mascot-spark', { x: 54, y: 14, width: 3, height: 3, transform: 'rotate(-15 55.5 15.5)' }],
            ['circle', 'csn-mascot-spark', { cx: 10, cy: 22, r: 1.5 }], ['circle', 'csn-mascot-spark', { cx: 58, cy: 30, r: 1.5 }]])
    };

    function shape(spec) {
        const node = document.createElementNS(NS, spec[0]);
        node.setAttribute('class', spec[1]);
        Object.keys(spec[2]).forEach(name => node.setAttribute(name, String(spec[2][name])));
        return node;
    }

    function create(options) {
        const o = options || {};
        const svg = document.createElementNS(NS, 'svg');
        svg.setAttribute('viewBox', '0 0 64 64'); svg.setAttribute('class', 'csn-mascot'); svg.setAttribute('aria-hidden', 'true'); svg.setAttribute('focusable', 'false');
        svg.setAttribute('width', String(o.size || 96)); svg.setAttribute('height', String(o.size || 96)); svg.setAttribute('data-csn', 'mascot');
        BASE.forEach(spec => svg.appendChild(shape(spec)));
        const face = document.createElementNS(NS, 'g'); face.setAttribute('class', 'csn-mascot-mood'); svg.appendChild(face);
        let current = '';
        function setMood(mood) {
            const next = MOODS.indexOf(mood) >= 0 ? mood : 'idle';
            if (next === current) return;
            current = next;
            while (face.firstChild) face.removeChild(face.firstChild);
            FACES[next].forEach(spec => face.appendChild(shape(spec)));
            svg.setAttribute('data-mood', next);
        }
        setMood(o.mood);
        return { el: svg, setMood, get mood() { return current; } };
    }

    // Priority: switched off > a job or run is working > blocked or failed (job or run) > run succeeded > step ready or done > idle.
    function moodFor(s) {
        const x = s || {};
        if (x.disabled) return 'sleeping';
        if (x.jobsActive || x.checksRunning || x.runActive) return 'thinking';
        if (x.blocked || x.jobFailed || x.runFailed) return 'worried';
        if (x.runSucceeded) return 'celebrating';
        if (x.stepState === 'ready' || x.stepState === 'done') return 'happy';
        return 'idle';
    }

    P.mascot = { create, moodFor, MOODS: MOODS.slice() };
})();
