(function () {
    'use strict';

    const SUNRISE = 6.5;
    const SUNSET = 19.5;
    const DEG = Math.PI / 180;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function rgb(hex) {
        const n = parseInt(hex.slice(1), 16);
        return [(n >> 16 & 255) / 255, (n >> 8 & 255) / 255, (n & 255) / 255];
    }

    // hour, skyTop, skyHorizon, skyBottom, fogColor, keyColor, keyI, hemiSky, hemiGround, hemiI, cloudTint, night, stars
    const KEYS = [
        [0, '#08102b', '#1e2f63', '#141c38', '#1b2648', '#a9bcff', .55, '#3b4f8c', '#121622', .55, '#5d6c9c', 1, 1],
        [4.8, '#08102b', '#1e2f63', '#141c38', '#1b2648', '#a9bcff', .55, '#3b4f8c', '#121622', .55, '#5d6c9c', 1, 1],
        [6.0, '#36559c', '#ffb08a', '#d9a3a0', '#eab4a2', '#ffb47e', 1.5, '#c6b4d8', '#5a4636', .85, '#ffd4c0', .25, .2],
        [7.6, '#4f8fe0', '#cfe7f6', '#e6eef2', '#d9e9f3', '#fff0d8', 2.6, '#bcd8f5', '#6b5a40', 1.0, '#ffffff', 0, 0],
        [12, '#3b84e6', '#bfe2ff', '#eaf3f8', '#cfe6f8', '#fff6e6', 3.0, '#c4e0ff', '#6e6042', 1.05, '#ffffff', 0, 0],
        [16.8, '#4b88d8', '#ffe6bf', '#f1ece2', '#f3e2c8', '#ffe4b4', 2.6, '#cfdcf0', '#6e5a3c', .95, '#fff4e4', 0, 0],
        [18.8, '#4a5ea6', '#ff9a5a', '#f0b089', '#f2a87c', '#ffa04f', 2.3, '#e2aa90', '#4b3628', .8, '#ffc69c', .1, 0],
        [19.9, '#272c66', '#c2688a', '#6d4f7e', '#84588a', '#dc8c9c', .8, '#6b5b9c', '#201829', .6, '#b07ca2', .6, .4],
        [21.2, '#08102b', '#1e2f63', '#141c38', '#1b2648', '#a9bcff', .55, '#3b4f8c', '#121622', .55, '#5d6c9c', 1, 1]
    ].map(r => ({
        hour: r[0], skyTop: rgb(r[1]), skyHorizon: rgb(r[2]), skyBottom: rgb(r[3]), fogColor: rgb(r[4]),
        keyColor: rgb(r[5]), keyIntensity: r[6], hemiSky: rgb(r[7]), hemiGround: rgb(r[8]), hemiIntensity: r[9],
        cloudTint: rgb(r[10]), night: r[11], stars: r[12]
    }));
    KEYS.push(Object.assign({}, KEYS[0], { hour: 24 }));

    const COLOR_FIELDS = ['skyTop', 'skyHorizon', 'skyBottom', 'fogColor', 'keyColor', 'hemiSky', 'hemiGround', 'cloudTint'];
    const NUMBER_FIELDS = ['keyIntensity', 'hemiIntensity', 'night', 'stars'];

    function normalize(v) {
        const len = Math.hypot(v[0], v[1], v[2]) || 1;
        return [v[0] / len, v[1] / len, v[2] / len];
    }

    function phaseFor(hour) {
        if (hour >= 21.2 || hour < 5.4) return 'night';
        if (hour < 7.0) return 'dawn';
        if (hour < 18.3) return 'day';
        return 'dusk';
    }

    function sunPath(hour) {
        let elevation;
        let azimuth;
        if (hour >= SUNRISE && hour < SUNSET) {
            const dayT = (hour - SUNRISE) / (SUNSET - SUNRISE);
            elevation = 62 * Math.sin(Math.PI * dayT);
            azimuth = Math.PI * dayT;
        } else {
            const nightT = ((hour - SUNSET + 24) % 24) / 11;
            elevation = -40 * Math.sin(Math.PI * nightT);
            azimuth = Math.PI + Math.PI * nightT;
        }
        const el = elevation * DEG;
        return {
            elevation,
            dir: [Math.cos(azimuth) * Math.cos(el), Math.sin(el), Math.sin(azimuth) * Math.cos(el)]
        };
    }

    function at(hourIn) {
        const raw = Number.isFinite(Number(hourIn)) ? Number(hourIn) : 12;
        const hour = ((raw % 24) + 24) % 24;
        let i = 0;
        while (i < KEYS.length - 2 && hour >= KEYS[i + 1].hour) i++;
        const a = KEYS[i];
        const b = KEYS[i + 1];
        const s = clamp((hour - a.hour) / (b.hour - a.hour), 0, 1);
        const t = s * s * (3 - 2 * s);
        const out = { hour };
        for (const key of COLOR_FIELDS) out[key] = [0, 1, 2].map(c => a[key][c] + (b[key][c] - a[key][c]) * t);
        for (const key of NUMBER_FIELDS) out[key] = a[key] + (b[key] - a[key]) * t;

        const sun = sunPath(hour);
        const sunDir = sun.dir;
        const moonDir = normalize([-sunDir[0], -sunDir[1], .35 + .5 * Math.abs(sunDir[2])]);
        out.phase = phaseFor(hour);
        out.sunDir = sunDir;
        out.moonDir = moonDir;
        out.keyDir = sunDir[1] > -.05 ? sunDir.slice() : moonDir.slice();
        out.elevation = sun.elevation;
        out.sunVisible = clamp(sun.elevation / 4 + 1, 0, 1);
        out.moonVisible = clamp(-sun.elevation / 4, 0, 1);
        out.rainbow = out.phase === 'day'
            ? clamp((sun.elevation - 8) / 6, 0, 1) * clamp((45 - sun.elevation) / 8, 0, 1)
            : 0;
        return out;
    }

    function hourFromDate(date) {
        const d = date || new Date();
        return d.getHours() + d.getMinutes() / 60 + d.getSeconds() / 3600;
    }

    window.AuraVoxelSky = Object.freeze({ at, hourFromDate });
})();
