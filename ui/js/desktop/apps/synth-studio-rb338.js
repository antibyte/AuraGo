// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors.
// RB-338 rack panel for Synth Studio: DOM, skin hooks, wire sending and snapshot follow.
// Port of the OpenRB UI behaviour (orb-ui model.rs / link.rs); the engine lives in the host.
(function (root, factory) {
    const api = factory(root);
    if (typeof module === 'object' && module.exports) module.exports = api;
    else root.SynthStudioRB338 = api;
})(typeof globalThis !== 'undefined' ? globalThis : window, function (root) {
    'use strict';

    const getModel = () => root.SynthStudioRB338Model || (typeof require === 'function' ? require('./synth-studio-rb338-model.js') : null);
    const escFallback = s => String(s).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch]);
    const clamp = (x, lo, hi) => Math.max(lo, Math.min(hi, x));
    const pad = (n, w) => String(Math.max(0, Math.trunc(n))).padStart(w, '0');

    const NOTE_NAMES = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B', "C'"];
    const BANKS = ['A', 'B', 'C', 'D'];
    // Instrument legends (A6): the drum rows' selectable instruments, in instrument-number order.
    const LEGEND_808 = ['AC', 'BD', 'SD', 'LT/LC', 'MT/MC', 'HT/HC', 'RS/CL', 'CP/MA', 'CB', 'CY', 'OH', 'CH'];
    const LEGEND_909 = ['AC', 'BD', 'SD', 'LT', 'MT', 'HT', 'RS', 'CP', 'CH', 'OH', 'CC', 'RC'];
    // Knob groups per drum machine: [group silkscreen, [control name suffixes]].
    const GROUPS_808 = [
        ['AC', ['ac_level']], ['BD', ['bd_level', 'bd_tone', 'bd_decay']], ['SD', ['sd_level', 'sd_tone', 'sd_snappy']],
        ['LT/LC', ['lt_level', 'lt_tuning', 'slot3_switch']], ['MT/MC', ['mt_level', 'mt_tuning', 'slot4_switch']],
        ['HT/HC', ['ht_level', 'ht_tuning', 'slot5_switch']], ['RS/CL', ['rs_level', 'slot6_switch']],
        ['CP/MA', ['cp_level', 'slot7_switch']], ['CB', ['cb_level']], ['CY', ['cy_level', 'cy_tone', 'cy_decay']],
        ['OH', ['oh_level', 'oh_decay']], ['CH', ['ch_level']]
    ];
    const GROUPS_909 = [
        ['AC', ['ac_level']], ['BD', ['bd_level', 'bd_tune', 'bd_attack', 'bd_decay']], ['SD', ['sd_level', 'sd_tune', 'sd_tone', 'sd_snappy']],
        ['LT', ['lt_level', 'lt_tune', 'lt_decay']], ['MT', ['mt_level', 'mt_tune', 'mt_decay']], ['HT', ['ht_level', 'ht_tune', 'ht_decay']],
        ['RS', ['rs_level']], ['CP', ['cp_level']], ['HH', ['hh_level']], ['CH', ['ch_decay']], ['OH', ['oh_decay']],
        ['CC', ['cc_level', 'cc_tune']], ['RC', ['rc_level', 'rc_tune']]
    ];
    const ICON = {
        play: '<path d="M4 2.5v11l9-5.5z"/>', stop: '<path d="M3.5 3.5h9v9h-9z"/>',
        rew: '<path d="M8.5 3v10L2 8zM14.5 3v10L8 8z"/>', ff: '<path d="M7.5 3v10L14 8zM1.5 3v10L8 8z"/>',
        rec: '<circle cx="8" cy="8" r="4.5"/>'
    };
    // Display repeat buttons that map to transport arrows (model.transportPress names).
    const TRANSPORT_ARROWS = {
        'transport.tempo': ['tempo_up', 'tempo_down'], 'transport.position': ['bar_up', 'bar_down'],
        'transport.loop_start': ['loop_start_up', 'loop_start_down'], 'transport.loop_end': ['loop_length_up', 'loop_length_down']
    };
    const SONG_ONLY = ['rewind', 'ff', 'record', 'bar_up', 'bar_down', 'loop_start_up', 'loop_start_down', 'loop_length_up', 'loop_length_down'];
    const MIXER_SNAP_KEYS = ['mute', 'level', 'pan', 'send', 'dist'];

    // ---- markup ----------------------------------------------------------------------------------
    function rackMarkup(M, tr, E) {
        const ROWS = M.ROW_LAYOUT;
        const svg = name => '<svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">' + ICON[name] + '</svg>';
        const ctl = (id, sil, who, o) => {
            o = o || {};
            const kind = o.kind || 'knob', def = o.def == null ? 64 : o.def;
            return '<label class="ss-rb-ctl ss-rb-' + kind + (o.cls ? ' ' + o.cls : '') + '"><span class="ss-rb-sil">' + E(sil) + '</span>' +
                '<input type="range" min="0" max="127" step="1" value="' + def + '" data-ss-kind="' + kind + '" data-ss-default="' + def + '" data-rb="' + id +
                '" aria-label="' + E((who + ' ' + (o.aria || sil)).trim()) + '"></label>';
        };
        const key = (id, sil, who, o) => {
            o = o || {};
            return '<button type="button" class="ss-rb-key' + (o.cls ? ' ' + o.cls : '') + '" data-rb="' + id + '"' + (o.value !== undefined ? ' data-rb-value="' + o.value + '"' : '') +
                (o.tone ? ' data-tone="' + o.tone + '"' : '') + ' aria-pressed="false" aria-label="' + E((who + ' ' + (o.aria || sil)).trim()) + '"><i class="ss-rb-led" aria-hidden="true"></i><span>' + E(sil) + '</span></button>';
        };
        const seg = (id, sil, who, labels, o) => {
            o = o || {};
            return '<span class="ss-rb-seg' + (o.cls ? ' ' + o.cls : '') + '" role="group" data-rb="' + id + '" aria-label="' + E((who + ' ' + (o.aria || sil)).trim()) + '">' +
                labels.map((l, i) => '<button type="button" data-rb-value="' + i + '" aria-pressed="false" aria-label="' + E((who + ' ' + l).trim()) + '">' + E(l) + '</button>').join('') + '</span>';
        };
        const meter = (id, kind) => {
            const tones = kind === 'v' ? 'ggggggaarr' : kind === 'h' ? 'ggar' : 'gggaaarrr';
            return '<span class="ss-led-meter ss-rb-meter" data-rb-meter="' + id + '" data-kind="' + kind + '" aria-hidden="true">' +
                tones.split('').map(t => '<i class="ss-led-seg"' + (t === 'a' ? ' data-tone="a"' : t === 'r' ? ' data-tone="r"' : '') + '></i>').join('') + '</span>';
        };
        const lcd = (id, sil, aria, o) => {
            o = o || {};
            const btn = (dir, name) => '<button type="button" class="ss-rb-ud" data-rb-repeat="' + id + '" data-dir="' + dir + '" aria-label="' + E(aria + ' ' + name) + '"><b class="ss-rb-tri' + (dir > 0 ? ' is-up' : '') + '"></b></button>';
            return '<span class="ss-rb-lcdbox' + (o.cls ? ' ' + o.cls : '') + '"><span class="ss-rb-sil">' + E(sil) + '</span><span class="ss-rb-lcdrow">' +
                '<output class="ss-rb-lcd" data-rb-out="' + id + '" aria-label="' + E(aria) + '">' + (o.init || '000') + '</output>' +
                '<span class="ss-rb-updown">' + btn(1, '+') + btn(-1, '-') + '</span></span></span>';
        };
        const title = (text, cls) => '<h4 class="ss-rb-title' + (cls ? ' ' + cls : '') + '">' + E(text) + '</h4>';
        const act = (a, label, aria, extra) => '<button type="button" class="ss-rb-key" data-rb-act="' + a + '"' + (extra || '') + ' aria-pressed="false" aria-label="' + E(aria) + '"><i class="ss-rb-led" aria-hidden="true"></i><span>' + E(label) + '</span></button>';

        // ---- toolbar -------------------------------------------------------------------------
        const tbtn = (action, key, cls) => '<button type="button" class="ss-rb-tool' + (cls ? ' ' + cls : '') + '" data-action="' + action + '" disabled><span>' + E(tr(key)) + '</span></button>';
        const toolbar = '<div class="ss-rb-toolbar" role="toolbar" aria-label="RB-338">' +
            tbtn('rb-new', 'rb_new') + tbtn('rb-open', 'rb_open') + tbtn('rb-save', 'rb_save') + tbtn('rb-export', 'rb_export_wav') + tbtn('rb-send-arrange', 'rb_send_arrange', 'is-primary') +
            '<span class="ss-rb-toolgap"></span>' +
            '<button type="button" class="ss-rb-tool" data-action="rb-keys" aria-pressed="false" disabled><i class="ss-rb-led" aria-hidden="true"></i><span>' + E(tr('rb_keys')) + '</span></button>' +
            '<button type="button" class="ss-rb-tool" data-action="rb-vintage" data-rb="song.vintage" aria-pressed="false"><i class="ss-rb-led" aria-hidden="true"></i><span>' + E(tr('rb_vintage')) + '</span></button>' +
            '</div>';

        // ---- transport -----------------------------------------------------------------------
        const tbutton = (name, icon, aria, cls) => '<button type="button" class="ss-rb-tbtn' + (cls ? ' ' + cls : '') + '" data-rb-act="transport" data-name="' + name + '" aria-label="' + E(aria) + '"' + (cls && cls.indexOf('is-toggle') >= 0 ? ' aria-pressed="false"' : '') + '>' + svg(icon) + '</button>';
        const transport = '<div class="ss-rb-plate ss-rb-transport" role="group" aria-label="TRANSPORT">' +
            '<button type="button" class="ss-rb-logo" data-rb-act="logo" aria-label="RB-338"><b>RB-338</b><span>Rebirth</span></button>' +
            '<div class="ss-rb-mod"><span class="ss-rb-sil">MOD</span><output class="ss-rb-lcd ss-rb-modname" data-rb-out="mod">Standard ReBirth</output><output class="ss-rb-modtitle" data-rb-out="title"></output></div>' +
            lcd('transport.tempo', 'TEMPO', tr('rb_tempo')) +
            ctl('transport.shuffle', 'SHUFFLE', '', { aria: tr('rb_shuffle'), cls: 'is-small', def: 0 }) +
            seg('transport.mode', 'MODE', '', [tr('rb_pattern_mode'), tr('rb_song_mode')], { cls: 'is-wide' }) +
            '<div class="ss-rb-tbtns">' + tbutton('rewind', 'rew', tr('rb_rewind')) + tbutton('stop', 'stop', tr('rb_stop')) + tbutton('play', 'play', tr('rb_play'), 'is-play is-toggle') + tbutton('ff', 'ff', tr('rb_forward')) +
            tbutton('record', 'rec', tr('rb_record'), 'is-rec is-toggle') + '</div>' +
            lcd('transport.position', 'BAR', tr('rb_bar')) + lcd('transport.loop_start', 'LOOP START', tr('rb_loop_start')) + lcd('transport.loop_end', 'LOOP LENGTH', tr('rb_loop_length')) +
            key('transport.loop', 'LOOP', '', { aria: tr('rb_loop'), cls: 'is-loop' }) + '</div>';

        // ---- rows ----------------------------------------------------------------------------
        const patternPlate = r => {
            const R = ROWS[r], sec = R.section, n = r + 1;
            return '<div class="ss-rb-plate ss-rb-pattern" role="group" aria-label="' + E(R.name + ' PATTERN') + '">' + title('PATTERN') +
                '<div class="ss-rb-prow">' + key(sec + '.on', 'ON', R.name, { aria: tr('rb_row_on'), cls: 'is-on' }) +
                '<span class="ss-rb-banks" role="group" aria-label="' + E(R.name + ' ' + tr('rb_bank')) + '">' +
                BANKS.map((b, i) => '<button type="button" data-rb-act="bank" data-row="' + r + '" data-bank="' + i + '" aria-pressed="false" aria-label="' + E(R.name + ' ' + tr('rb_bank') + ' ' + b) + '">' + b + '</button>').join('') + '</span></div>' +
                '<span class="ss-rb-pbtns" role="group" data-rb="' + sec + '.pattern" aria-label="' + E(R.name + ' PATTERN') + '">' +
                [1, 2, 3, 4, 5, 6, 7, 8].map(b => '<button type="button" data-rb-value="' + b + '" aria-pressed="false" aria-label="' + E(R.name + ' PATTERN ' + b) + '">' + b + '</button>').join('') + '</span>' +
                '<div class="ss-rb-prow">' + key(sec + '.shuffle', 'SHUFFLE', R.name) + lcd(sec + '.steps', 'STEPS', R.name + ' STEPS', { init: '16', cls: 'is-steps' }) + '</div></div>';
        };
        const stepStrip = (r, drum) => {
            const R = ROWS[r];
            let h = '<div class="ss-rb-steps' + (drum ? ' is-drum' : '') + '" role="group" aria-label="' + E(R.name + ' STEPS') + '" data-row="' + r + '">';
            for (let i = 0; i < 16; i++) {
                h += '<button type="button" class="ss-rb-step" data-rb-act="step" data-row="' + r + '" data-step="' + i + '" data-g="' + (i >> 2) + '" aria-pressed="false" aria-label="' + E(R.name + ' STEP ' + (i + 1)) + '"><i class="ss-rb-led" aria-hidden="true"></i><span>' + (i + 1) + '</span></button>';
            }
            return h + '</div>';
        };
        const device303 = r => {
            const R = ROWS[r], p = R.prefix;
            const noteKeys = NOTE_NAMES.map((nm, i) => '<button type="button" class="ss-rb-note" data-rb-act="note" data-row="' + r + '" data-note="' + i + '"' + (nm.length === 2 && nm[1] === '#' ? ' data-black' : '') + ' aria-pressed="false" aria-label="' + E(R.name + ' NOTE ' + nm) + '">' + E(nm) + '</button>').join('');
            const flag = (f, sil) => act('flag', sil, R.name + ' ' + sil, ' data-row="' + r + '" data-flag="' + f + '"');
            return '<div class="ss-rb-plate ss-rb-device ss-rb-d303" role="group" aria-label="' + E(R.name) + '">' + title(R.name) +
                '<div class="ss-rb-knobs">' + ctl(p + '.tune', 'TUNE', R.name) + ctl(p + '.cutoff', 'CUTOFF', R.name) + ctl(p + '.reso', 'RESO', R.name) +
                ctl(p + '.envmod', 'ENV MOD', R.name) + ctl(p + '.decay', 'DECAY', R.name) + ctl(p + '.accent', 'ACCENT', R.name) +
                seg(p + '.wave', 'WAVEFORM', R.name, ['SAW', 'SQR'], { cls: 'is-wave' }) + '</div>' +
                stepStrip(r, false) +
                '<div class="ss-rb-editor" data-row="' + r + '">' +
                '<span class="ss-rb-lcdbox is-edit"><span class="ss-rb-sil">STEP</span><span class="ss-rb-lcdrow"><output class="ss-rb-lcd" data-rb-out="edit:' + r + '" aria-label="' + E(R.name + ' EDIT STEP') + '">01</output></span></span>' +
                '<div class="ss-rb-notes" role="group" aria-label="' + E(R.name + ' NOTES') + '">' + noteKeys + '</div>' +
                '<div class="ss-rb-flags">' + flag('down', 'DOWN') + flag('up', 'UP') + flag('accent', 'ACCENT') + flag('slide', 'SLIDE') + '</div>' +
                '<div class="ss-rb-edits">' +
                '<button type="button" class="ss-rb-key is-notepause" data-rb-act="note_pause" data-row="' + r + '" aria-label="' + E(R.name + ' NOTE/PAUSE') + '"><i class="ss-rb-led is-note" aria-hidden="true"></i><span>NOTE</span><i class="ss-rb-led is-rest" aria-hidden="true"></i><span>REST</span></button>' +
                act('back', 'BACK', R.name + ' BACK', ' data-row="' + r + '"') + act('step_next', 'STEP', R.name + ' STEP', ' data-row="' + r + '"') +
                act('pitch', 'PITCH MODE', R.name + ' PITCH MODE', ' data-row="' + r + '"') + act('clear', 'CLEAR', R.name + ' CLEAR', ' data-row="' + r + '"') +
                '</div></div></div>';
        };
        const deviceDrum = r => {
            const R = ROWS[r], p = R.prefix, is808 = r === 2;
            const groups = is808 ? GROUPS_808 : GROUPS_909, legend = is808 ? LEGEND_808 : LEGEND_909;
            const grp = groups.map(([name, names]) => {
                let inner = '';
                for (const n of names) {
                    const c = M.control(p + '.' + n);
                    if (!c) continue;
                    if (c.kind === 'switch') inner += seg(c.id, c.label, R.name, c.label.split('/'), { cls: 'is-slot' });
                    else inner += ctl(c.id, c.label.replace(/^\S+\s*/, '') || 'LEVEL', R.name, { aria: c.label, cls: c.level && is808 ? 'is-level' : '' });
                }
                return '<div class="ss-rb-grp" role="group" aria-label="' + E(R.name + ' ' + name) + '"><span class="ss-rb-grptitle">' + E(name) + '</span><div class="ss-rb-grpknobs">' + inner + '</div></div>';
            }).join('');
            const flam = is808 ? '' : key(p + '.flam', 'FLAM', R.name, { cls: 'is-flam' });
            const leg = legend.map((l, i) => '<button type="button" data-rb-act="inst" data-row="' + r + '" data-inst="' + i + '" aria-pressed="false" aria-label="' + E(R.name + ' ' + l) + '">' + E(l) + '</button>').join('');
            return '<div class="ss-rb-plate ss-rb-device ss-rb-ddrum ' + (is808 ? 'is-808' : 'is-909') + '" role="group" aria-label="' + E(R.name) + '">' + title(R.name) +
                '<div class="ss-rb-groups">' + grp + '</div>' +
                '<div class="ss-rb-legendrow"><span class="ss-rb-legend" role="group" aria-label="' + E(R.name + ' INSTRUMENT') + '">' + leg + '</span>' + flam + '</div>' +
                stepStrip(r, true) + '</div>';
        };
        const channel = r => {
            const R = ROWS[r], c = r + 1, id = 'mixer' + c;
            return '<div class="ss-rb-plate ss-rb-channel" role="group" aria-label="' + E(R.name + ' MIXER') + '">' + title('CH ' + c, 'is-small') +
                '<div class="ss-rb-chtop">' + key(id + '.mute', 'MUTE', R.name, { cls: 'is-mute', tone: 'red' }) + key(id + '.dist', 'DIST', R.name) + '</div>' +
                '<div class="ss-rb-chfader">' + ctl(id + '.level', 'LEVEL', R.name, { kind: 'vfader', def: 100, cls: 'is-mixer' }) + meter(id + '.meter', 'v') + '</div>' +
                '<div class="ss-rb-chknobs">' + ctl(id + '.pan', 'PAN', R.name, { cls: 'is-small' }) + ctl(id + '.delay', 'DELAY', R.name, { cls: 'is-small', def: 0 }) + '</div>' +
                '<div class="ss-rb-chfx">' + key('mixer.pcf_id', 'PCF', R.name, { value: c, cls: 'is-radio' }) + key('mixer.comp_id', 'COMP', R.name, { value: c + 1, cls: 'is-radio' }) + '</div></div>';
        };
        const rows = ROWS.map((R, r) => '<div class="ss-rb-row" data-row="' + r + '" data-kind="' + (r < 2 ? '303' : r === 2 ? '808' : '909') + '">' +
            patternPlate(r) + (r < 2 ? device303(r) : deviceDrum(r)) + channel(r) + '</div>').join('');

        // ---- effects column ------------------------------------------------------------------
        const fxHead = (name, id) => '<div class="ss-rb-fxhead">' + title(name) + meter(id + '.meter', 'h') + '</div>';
        const master = '<div class="ss-rb-plate ss-rb-fx ss-rb-master" role="group" aria-label="MASTER">' + title('MASTER') +
            '<div class="ss-rb-masterbody">' + meter('master.meter_l', 'v') + ctl('master.fader', 'LEVEL', 'MASTER', { kind: 'vfader', def: 100, cls: 'is-mixer' }) + meter('master.meter_r', 'v') + '</div>' +
            key('mixer.comp_id', 'COMP', 'MASTER', { value: 1, cls: 'is-radio' }) + '</div>';
        const pcf = '<div class="ss-rb-plate ss-rb-fx ss-rb-pcf" role="group" aria-label="PCF">' + fxHead('PCF', 'pcf') +
            '<div class="ss-rb-fxrow">' + key('pcf.on', 'ON', 'PCF', { cls: 'is-on' }) + seg('pcf.mode', 'BP/LP', 'PCF', ['BP', 'LP']) + lcd('pcf.pattern', 'WAVE', 'PCF WAVE', { init: '00', cls: 'is-steps' }) + '</div>' +
            '<div class="ss-rb-faders">' + ctl('pcf.freq', 'FREQ', 'PCF', { kind: 'vfader', cls: 'is-pcf' }) + ctl('pcf.q', 'Q', 'PCF', { kind: 'vfader', cls: 'is-pcf' }) +
            ctl('pcf.amt', 'AMT', 'PCF', { kind: 'vfader', cls: 'is-pcf' }) + ctl('pcf.dec', 'DEC', 'PCF', { kind: 'vfader', cls: 'is-pcf' }) + '</div></div>';
        const delay = '<div class="ss-rb-plate ss-rb-fx ss-rb-delay" role="group" aria-label="DELAY">' + fxHead('DELAY', 'delay') +
            '<div class="ss-rb-fxrow">' + key('delay.on', 'ON', 'DELAY', { cls: 'is-on' }) + lcd('delay.steps', 'STEPS', 'DELAY STEPS', { init: '00', cls: 'is-steps' }) + seg('delay.triplet', 'TRIPLET/16TH', 'DELAY', ['TRIP', '16TH']) + '</div>' +
            '<div class="ss-rb-knobs">' + ctl('delay.feedback', 'F.BACK', 'DELAY') + ctl('delay.pan', 'PAN', 'DELAY') + '</div></div>';
        const dist = '<div class="ss-rb-plate ss-rb-fx ss-rb-dist" role="group" aria-label="DIST">' + fxHead('DIST', 'dist') +
            '<div class="ss-rb-fxrow">' + key('dist.on', 'ON', 'DIST', { cls: 'is-on' }) + '</div>' +
            '<div class="ss-rb-knobs">' + ctl('dist.amount', 'AMOUNT', 'DIST') + ctl('dist.shape', 'SHAPE', 'DIST') + '</div></div>';
        const comp = '<div class="ss-rb-plate ss-rb-fx ss-rb-comp" role="group" aria-label="COMP">' + fxHead('COMP', 'comp') +
            '<div class="ss-rb-fxrow">' + key('comp.on', 'ON', 'COMP', { cls: 'is-on' }) + '</div>' +
            '<div class="ss-rb-knobs">' + ctl('comp.ratio', 'RATIO', 'COMP') + ctl('comp.threshold', 'THRES', 'COMP') + '</div>' +
            '<div class="ss-rb-grrow"><span class="ss-rb-sil">GR</span>' + meter('comp.gr', 'g') + '</div></div>';

        const foot = '<footer class="ss-rb-foot"><span>' + E(tr('rb_license')) + '</span> <a href="/js/vendor/rb338/LICENSES.md" target="_blank" rel="noopener noreferrer">GPL-3.0</a> ' +
            '<a href="/js/vendor/rb338/sources.zip" download>' + E(tr('rb_source')) + '</a></footer>';
        return '<div class="ss-rb-loading" role="status"><i class="ss-rb-led is-on" aria-hidden="true"></i><span data-rb-loadtext>' + E(tr('rb_loading')) + '</span></div>' +
            '<div class="ss-rb-rack">' + toolbar + transport + rows + '<div class="ss-rb-fxcol" role="group" aria-label="EFFECTS">' + master + pcf + delay + dist + comp + '</div>' + foot + '</div>';
    }

    // ---- panel ----------------------------------------------------------------------------------
    function create(section, options) {
        const M = getModel();
        const o = options || {};
        const tr = typeof o.tr === 'function' ? o.tr : k => k;
        const E = typeof o.esc === 'function' ? o.esc : escFallback;
        const signal = o.signal;
        const lock = () => (typeof o.blocked === 'function' ? !!o.blocked() : !!o.blocked);
        const say = text => { if (typeof o.notice === 'function') o.notice(text); };
        const fmt = (key, vars) => String(tr(key)).replace(/\{(\w+)\}/g, (m, k) => (vars && vars[k] != null ? vars[k] : m));
        const ac = new AbortController();
        const sig = ac.signal;
        if (signal && typeof signal.addEventListener === 'function') {
            if (signal.aborted) ac.abort();
            else signal.addEventListener('abort', () => dispose(), { once: true });
        }

        // state
        let host = null, hostState = 'idle', active = false, disposed = false, rafId = 0;
        let snap = null;
        let seq = 0, lastSent = 0, flushToken = 0;
        let transport = M.createTransport();
        let mirror = new Uint8Array(M.PATTERNS_SIZE);
        let info = null, focusRow = 0, modified = false;
        const outbox = [];
        const held = new Set();
        const vals = Object.create(null);
        const rows = [0, 1, 2, 3].map(() => ({ bank: 0, inst: 1, editStep: 0, pitch: false, lastPattern: -1 }));
        const actions = Object.create(null); // toolbar data-action -> handler (T9/T11 fill these)
        const behaviours = Object.create(null); // data-rb-act -> (button, event) (T8 fills these)
        const hooks = {}; // ready/loaded/saved/crash/modified/activate/leave: later tasks subscribe
        const bindings = { ranges: [], pressed: [], outs: [], meters: [], songOnly: [], steps: [] };

        if (!section) return { activate() {}, stop() {}, isPlaying: () => false, dispose() {} };
        section.classList.add('ss-rb338-workbench');
        section.dataset.rbState = 'idle';
        section.innerHTML = rackMarkup(M, tr, E);
        const $ = (sel, scope) => (scope || section).querySelector(sel);
        const $$ = (sel, scope) => Array.from((scope || section).querySelectorAll(sel));
        const Controls = () => root.SynthStudioControls;
        if (Controls() && Controls().upgradeAll) Controls().upgradeAll(section);

        // ---- values ----------------------------------------------------------------------------
        const rowOf = id => { const m = /^pattern(\d)\./.exec(id); return m ? Number(m[1]) - 1 : -1; };
        const curPattern = r => vals[M.ROW_LAYOUT[r].section + '.pattern'] || 0;
        const patternField = (r, field) => mirror[M.patternOffset(r, curPattern(r)) + field];
        function getVal(id) {
            switch (id) {
            case 'transport.mode': return +transport.songMode;
            case 'transport.loop': return +transport.looping;
            case 'transport.record': return +transport.recording;
            case 'transport.tempo': return transport.tempo;
            case 'transport.position': return transport.position;
            case 'transport.loop_start': return transport.loopStart;
            case 'transport.loop_end': return transport.loopEnd;
            default: break;
            }
            const m = /^pattern(\d)\.(shuffle|steps)$/.exec(id);
            if (m) return patternField(Number(m[1]) - 1, m[2] === 'shuffle' ? 0 : 1);
            return vals[id];
        }
        function readSnap(s, c) {
            if (c.msg === 'param') { const a = s.ctrl[M.CTRL_KEYS[c.dev]]; return a ? a[c.ctrl] : undefined; }
            if (c.msg === 'mixerGlobal') return c.ctrl === 0 ? s.mixer.master : c.ctrl === 1 ? s.mixer.compId : s.mixer.pcfId;
            if (c.msg === 'mixerChannel') return s.mixer.channels[c.ch][MIXER_SNAP_KEYS[c.k]];
            return undefined;
        }
        function adopt(s) {
            for (const c of M.CONTROLS) {
                if (c.readOnly || c.uiOnly || held.has(c.id)) continue;
                if (c.msg === 'param' || c.msg === 'mixerGlobal' || c.msg === 'mixerChannel') {
                    const v = readSnap(s, c);
                    if (v === undefined) continue;
                    if (/^pattern\d\.pattern$/.test(c.id)) {
                        const r = rowOf(c.id);
                        if (v !== vals[c.id]) rows[r].bank = v >> 3;
                    }
                    vals[c.id] = v;
                }
            }
            vals['transport.shuffle'] = s.shuffle;
            vals['song.vintage'] = +s.vintage;
            transport = Object.assign({}, transport, {
                tempo: s.tempo, position: Math.max(0, s.tick), loopStart: s.loopStart, loopEnd: s.loopEnd,
                songMode: s.songMode, playing: s.playing, recording: s.recording, looping: s.loopOn
            });
        }
        // The adopt rule (A7): only once the engine applied everything we sent.
        function ingest() {
            if (!snap) return;
            if (snap.appliedSeq >= lastSent && outbox.length === 0) adopt(snap);
        }

        // ---- sending ---------------------------------------------------------------------------
        function ready() { return host && hostState === 'ready'; }
        function queue(msg, key) {
            if (key) {
                const i = outbox.findIndex(e => e.key === key);
                if (i >= 0) { outbox[i].msg = msg; return; }
            }
            outbox.push({ msg, key });
        }
        function flush() {
            flushToken++;
            if (!outbox.length) return;
            const msgs = outbox.splice(0).map(e => e.msg);
            if (!ready()) return;
            lastSent = ++seq;
            try { host.send(M.batch(msgs), lastSent); } catch (err) { fail('rb_crashed'); }
        }
        function schedule() {
            const token = ++flushToken;
            const go = () => { if (token === flushToken) flush(); };
            if (typeof requestAnimationFrame === 'function') requestAnimationFrame(go);
            setTimeout(go, 48);
        }
        function emit(msg, key) {
            if (!ready()) return;
            queue(msg, key);
            if (key) schedule(); else flush();
        }
        function emitMany(msgs) {
            if (!ready()) return;
            msgs.forEach(m => queue(m));
            flush();
        }
        function markModified() { if (!modified) { modified = true; if (hooks.modified) hooks.modified(); } }

        function setControl(id, value) {
            if (lock()) return;
            const c = M.control(id);
            if (!c || c.readOnly) return;
            const r = rowOf(id);
            if (c.msg === 'patternField') mirror[M.patternOffset(r, curPattern(r)) + c.field] = value;
            else vals[id] = value;
            if (c.uiOnly) return;
            const msg = M.encodeControl(id, value, r >= 0 ? curPattern(r) : 0);
            if (!msg) return;
            emit(msg, c.kind === 'knob' || c.kind === 'vfader' ? 'k:' + id : undefined);
            markModified();
        }
        function press(name) {
            if (!ready()) return;
            if (lock() && name !== 'stop') return;
            if (name === 'play') {
                if (typeof o.beforePlay === 'function') o.beforePlay();
                wake();
            }
            const r = M.transportPress(transport, name);
            transport = r.transport;
            emitMany(r.messages);
            if (name !== 'play' && name !== 'stop' && /^(loop|mode|tempo_)/.test(name)) markModified();
        }
        function toggle(id) {
            if (id === 'transport.loop') return press('loop');
            if (id === 'transport.record') return press('record');
            if (id === 'tr909.flam') { vals[id] = vals[id] ? 0 : 1; return undefined; }
            return setControl(id, getVal(id) ? 0 : 1);
        }
        function bump(id, dir) {
            if (TRANSPORT_ARROWS[id]) return press(TRANSPORT_ARROWS[id][dir > 0 ? 0 : 1]);
            const c = M.control(id);
            if (!c) return undefined;
            const cur = getVal(id) | 0, next = clamp(cur + dir, c.min, c.max);
            if (next !== cur) setControl(id, next);
            return undefined;
        }

        // ---- host ------------------------------------------------------------------------------
        function fail(key, detail) {
            hostState = 'error';
            section.dataset.rbState = 'error';
            section.removeAttribute('data-rb-ready');
            const t = $('[data-rb-loadtext]');
            if (t) t.textContent = tr(key);
            say(detail ? tr(key) + ' ' + detail : tr(key));
            if (hooks.crash) hooks.crash(key, detail);
        }
        const bytesOf = v => (v == null ? null : v instanceof Uint8Array ? v : ArrayBuffer.isView(v) ? new Uint8Array(v.buffer, v.byteOffset, v.byteLength) : new Uint8Array(v));
        function setMirror(patterns) {
            const b = bytesOf(patterns);
            if (b && b.length >= M.PATTERNS_SIZE) mirror = Uint8Array.from(b.subarray(0, M.PATTERNS_SIZE));
        }
        function setInfo(i) {
            if (typeof i === 'string') { try { i = JSON.parse(i); } catch (_) { i = null; } }
            info = i && typeof i === 'object' ? i : null;
            const name = info && info.mod ? String(info.mod) : '';
            if (name && !/^standard/i.test(name)) say(fmt('rb_mod_missing', { mod: name }));
        }
        function applyDefaults(d) {
            const u = bytesOf(d);
            if (!u) return;
            const base = u.length >= M.SNAPSHOT_SIZE ? 0 : 64;
            for (const b of bindings.ranges) {
                const c = M.control(b.id);
                const v = c && c.off != null ? u[c.off - base] : undefined;
                if (v !== undefined) b.input.dataset.ssDefault = String(v);
            }
        }
        function onReady(msg) {
            if (disposed) return;
            const m = msg || {};
            setMirror(m.patterns);
            setInfo(m.info);
            applyDefaults(m.defaults);
            hostState = 'ready';
            section.dataset.rbState = 'ready';
            section.setAttribute('data-rb-ready', '');
            if (hooks.ready) hooks.ready(m);
            ingest();
            if (active) { renderAll(); wake(); }
        }
        function onSnapshot(buf) {
            if (disposed) return;
            try { snap = M.decodeSnapshot(buf); } catch (_) { return; }
            ingest();
        }
        function onLoaded(msg) {
            if (disposed) return;
            if (msg && msg.ok !== false) {
                setMirror(msg.patterns);
                setInfo(msg.info);
                rows.forEach(r => { r.editStep = 0; r.pitch = false; });
                modified = false;
            }
            if (hooks.loaded) hooks.loaded(msg);
        }
        function onSaved(msg) { if (!disposed && hooks.saved) hooks.saved(msg); }
        function onCrash(msg) { if (!disposed) fail('rb_crashed', msg ? String(msg.message || msg) : ''); }
        // Creates the live host; the AudioContext inside must be made in the user's gesture.
        function ensureHost() {
            if (host || hostState === 'error') return;
            const Host = root.SynthStudioRB338Host;
            if (!Host || typeof Host.live !== 'function') { fail('rb_load_failed'); return; }
            hostState = 'loading';
            section.dataset.rbState = 'loading';
            try {
                host = Host.live({ onReady, onSnapshot, onCrash, onLoaded, onSaved });
            } catch (err) {
                host = null;
                fail('rb_load_failed', err && err.message ? err.message : '');
            }
        }
        function wake() {
            if (!host || typeof host.resume !== 'function') return;
            let p;
            try { p = host.resume(); } catch (_) { return; }
            const check = () => { if (host && host.context && host.context.state === 'suspended') say(tr('rb_audio_blocked')); };
            if (p && typeof p.then === 'function') p.then(check, check);
        }

        // ---- bindings --------------------------------------------------------------------------
        const setPressed = (el, on) => { if (el._p !== on) { el._p = on; el.setAttribute('aria-pressed', on ? 'true' : 'false'); } };
        const setText = (el, text) => { if (el._t !== text) { el._t = text; el.textContent = text; } };
        const addPressed = (el, test) => bindings.pressed.push({ el, test });

        function bind() {
            $$('input[type=range][data-rb]').forEach(input => bindings.ranges.push({ id: input.dataset.rb, input, wrap: input.parentElement }));
            // toggles, radios and group buttons
            $$('button[data-rb]').forEach(b => {
                const id = b.dataset.rb;
                if (b.dataset.rbValue !== undefined) { const v = Number(b.dataset.rbValue); addPressed(b, () => getVal(id) === v); }
                else addPressed(b, () => !!getVal(id));
            });
            $$('[role=group][data-rb]').forEach(g => {
                const id = g.dataset.rb;
                const r = rowOf(id);
                $$('button[data-rb-value]', g).forEach(b => {
                    const v = Number(b.dataset.rbValue);
                    if (/\.pattern$/.test(id)) addPressed(b, () => getVal(id) === M.patternIndex(rows[r].bank, v));
                    else addPressed(b, () => M.switchPosition(id, getVal(id) | 0) === v);
                });
            });
            $$('[data-rb-act=bank]').forEach(b => { const r = Number(b.dataset.row), k = Number(b.dataset.bank); addPressed(b, () => rows[r].bank === k); });
            $$('[data-rb-act=inst]').forEach(b => { const r = Number(b.dataset.row), k = Number(b.dataset.inst); addPressed(b, () => rows[r].inst === k); });
            $$('[data-rb-act=transport][data-name=play]').forEach(b => addPressed(b, () => transport.playing));
            $$('[data-rb-act=transport][data-name=record]').forEach(b => addPressed(b, () => transport.recording));
            $$('[data-rb-act=pitch]').forEach(b => { const r = Number(b.dataset.row); addPressed(b, () => rows[r].pitch); });
            // 303 editor LEDs read the step under the edit cursor from the pattern mirror
            const stepOf = r => M.patternOffset(r, curPattern(r)) + M.stepIndex(r, rows[r].editStep, 0);
            $$('[data-rb-act=note]').forEach(b => { const r = Number(b.dataset.row), n = Number(b.dataset.note); addPressed(b, () => mirror[stepOf(r) + 5] === 0 && mirror[stepOf(r)] === n); });
            const FLAG_FIELD = { slide: 1, accent: 2, up: 3, down: 4 };
            $$('[data-rb-act=flag]').forEach(b => { const r = Number(b.dataset.row), f = FLAG_FIELD[b.dataset.flag]; addPressed(b, () => mirror[stepOf(r) + f] !== 0); });
            $$('[data-rb-act=note_pause]').forEach(b => { const r = Number(b.dataset.row); addPressed(b, () => mirror[stepOf(r) + 5] === 0); });
            // displays
            $$('output[data-rb-out]').forEach(out => bindings.outs.push({ el: out, id: out.dataset.rbOut }));
            // meters
            $$('[data-rb-meter]').forEach(el => bindings.meters.push({ el, id: el.dataset.rbMeter, kind: el.dataset.kind, segs: Array.from(el.children), lit: -1 }));
            // song-mode only controls
            $$('[data-rb-act=transport]').forEach(b => { if (SONG_ONLY.indexOf(b.dataset.name) >= 0) bindings.songOnly.push(b); });
            $$('[data-rb-repeat]').forEach(b => { if (/^transport\.(position|loop_start|loop_end)$/.test(b.dataset.rbRepeat)) bindings.songOnly.push(b); });
            // step buttons
            $$('.ss-rb-step').forEach(b => bindings.steps.push({ el: b, row: Number(b.dataset.row), step: Number(b.dataset.step) }));
        }
        bind();

        // ---- rendering -------------------------------------------------------------------------
        function stepState(r, i) {
            const p = curPattern(r), base = M.patternOffset(r, p);
            const len = clamp(mirror[base + 1] || 16, 1, 16);
            const so = base + M.stepIndex(r, i, 0);
            let s = i >= len ? 'out' : '';
            if (r < 2) {
                if (mirror[so + 5] === 0) s += ' on' + (mirror[so + 2] ? ' acc' : '') + (mirror[so + 1] ? ' sl' : '');
                if (i === rows[r].editStep) s += ' edit';
            } else {
                const v = mirror[so + rows[r].inst];
                if (v) s += ' on v' + Math.min(v, 3);
            }
            if (snap && snap.playing && snap.playStep[r] === i && snap.playingPattern[r] === p) s += ' run';
            return s;
        }
        function outText(id) {
            if (id === 'transport.tempo') return pad(M.tempoDisplay(transport), 3);
            if (id === 'transport.position') return pad(M.barDisplay(transport), 3);
            if (id === 'transport.loop_start') return pad(M.loopStartDisplay(transport), 3);
            if (id === 'transport.loop_end') return pad(M.loopLengthDisplay(transport), 3);
            if (id === 'mod') return 'Standard ReBirth';
            if (id === 'title') return info && info.title ? String(info.title) : '';
            if (id.startsWith('edit:')) return pad(rows[Number(id.slice(5))].editStep + 1, 2);
            const v = getVal(id);
            return v == null ? '--' : pad(v, 2);
        }
        function meterByte(id) {
            if (!snap) return 0;
            let m = /^mixer(\d)\.meter$/.exec(id);
            if (m) return snap.channelMeters[Number(m[1]) - 1];
            if (id === 'master.meter_l') return snap.masterMeters[0];
            if (id === 'master.meter_r') return snap.masterMeters[1];
            if (id === 'comp.gr') return snap.compGain;
            m = /^(\w+)\.meter$/.exec(id);
            return m && snap.effectMeters[m[1]] != null ? snap.effectMeters[m[1]] : 0;
        }
        function renderMeters() {
            for (const m of bindings.meters) {
                const b = meterByte(m.id);
                const n = m.kind === 'v' ? M.meterState(b) : m.kind === 'h' ? M.headerMeterSegments(M.meterState(b)) : M.grLit(b);
                if (n === m.lit) continue;
                m.lit = n;
                m.segs.forEach((seg, i) => seg.classList.toggle('is-on', i < n));
            }
        }
        function renderAll() {
            ingest();
            const lockNow = lock();
            if (section._l !== lockNow) { section._l = lockNow; section.toggleAttribute('data-rb-locked', lockNow); }
            const mode = transport.songMode ? 'song' : 'pattern';
            if (section.dataset.rbMode !== mode) section.dataset.rbMode = mode;
            if (snap) {
                const f = snap.faulted ? '1' : '0';
                if (section._f !== f) { section._f = f; section.toggleAttribute('data-rb-fault', snap.faulted); }
            }
            for (const b of bindings.ranges) {
                const v = vals[b.id];
                if (v === undefined || held.has(b.id) || Number(b.input.value) === v) continue;
                b.input.value = String(v);
                if (Controls() && Controls().sync) Controls().sync(b.wrap);
            }
            for (const b of bindings.pressed) setPressed(b.el, !!b.test());
            for (const out of bindings.outs) setText(out.el, outText(out.id));
            const songMode = transport.songMode;
            for (const b of bindings.songOnly) { const d = !songMode; if (b._d !== d) { b._d = d; b.disabled = d; } }
            for (const s of bindings.steps) {
                const st = stepState(s.row, s.step);
                if (s.el._s === st) continue;
                s.el._s = st;
                s.el.dataset.s = st;
                setPressed(s.el, / on/.test(' ' + st));
            }
            const rowEls = section._rows || (section._rows = $$('.ss-rb-row'));
            rowEls.forEach((el, i) => { const f = i === focusRow; if (el._f !== f) { el._f = f; el.classList.toggle('is-focus', f); } });
            renderMeters();
        }
        function frame() {
            rafId = 0;
            if (!active || disposed) return;
            renderAll();
            rafId = requestAnimationFrame(frame);
        }
        function startLoop() { if (!rafId && typeof requestAnimationFrame === 'function') rafId = requestAnimationFrame(frame); }
        function stopLoop() { if (rafId) { cancelAnimationFrame(rafId); rafId = 0; } }

        // ---- events ----------------------------------------------------------------------------
        const on = (target, type, fn, opts) => target.addEventListener(type, fn, Object.assign({ signal: sig }, opts));
        // knobs and faders (the Controls upgrade dispatches input/change on the native range)
        on(section, 'input', e => {
            const t = e.target;
            if (!t || t.type !== 'range' || !t.dataset.rb) return;
            held.add(t.dataset.rb);
            setControl(t.dataset.rb, Number(t.value));
        });
        on(section, 'change', e => { const t = e.target; if (t && t.type === 'range' && t.dataset.rb) held.delete(t.dataset.rb); });
        on(section, 'pointerdown', e => {
            const t = e.target;
            if (t && t.type === 'range' && t.dataset && t.dataset.rb) held.add(t.dataset.rb);
            const row = t && t.closest ? t.closest('.ss-rb-row') : null;
            if (row) focusRow = Number(row.dataset.row);
        });
        on(window, 'pointerup', () => held.clear());
        on(window, 'pointercancel', () => held.clear());

        // repeat buttons: first step on press, then repeatExtraSteps while held
        let repeat = null;
        function stopRepeat() { if (repeat) { clearInterval(repeat.timer); repeat = null; } }
        on(section, 'pointerdown', e => {
            const b = e.target.closest ? e.target.closest('[data-rb-repeat]') : null;
            if (!b || b.disabled || e.button !== 0) return;
            stopRepeat();
            const id = b.dataset.rbRepeat, dir = Number(b.dataset.dir);
            bump(id, dir);
            const t0 = performance.now();
            repeat = { timer: setInterval(() => {
                const extra = M.repeatExtraSteps(performance.now() - t0);
                while (repeat && repeat.done < extra) { repeat.done++; bump(id, dir); }
            }, 20), done: 0 };
        });
        on(window, 'pointerup', stopRepeat);
        on(window, 'pointercancel', stopRepeat);
        on(window, 'blur', stopRepeat);

        function runAct(name, b, e) {
            if (name === 'transport') return press(b.dataset.name);
            if (name === 'bank') { rows[Number(b.dataset.row)].bank = Number(b.dataset.bank); return undefined; }
            if (name === 'inst') { rows[Number(b.dataset.row)].inst = Number(b.dataset.inst); return undefined; }
            if (name === 'logo') { if (host && typeof host.startup === 'function') { if (typeof o.beforePlay === 'function') o.beforePlay(); wake(); host.startup(); } return undefined; }
            if (name === 'step' && Number(b.dataset.row) < 2) { rows[Number(b.dataset.row)].editStep = Number(b.dataset.step); return undefined; }
            if (behaviours[name]) return behaviours[name](b, e);
            return undefined;
        }
        on(section, 'click', e => {
            const b = e.target.closest ? e.target.closest('button') : null;
            if (!b || !section.contains(b) || b.disabled) return;
            if (b.dataset.rbRepeat !== undefined) { if (e.detail === 0) bump(b.dataset.rbRepeat, Number(b.dataset.dir)); return; }
            if (b.dataset.rbAct) { runAct(b.dataset.rbAct, b, e); return; }
            if (b.dataset.rb !== undefined) {
                if (b.dataset.rbValue === undefined) toggle(b.dataset.rb);
                else { const id = b.dataset.rb, v = Number(b.dataset.rbValue); setControl(id, getVal(id) === v ? 0 : v); }
                return;
            }
            const holder = b.closest('[data-rb]');
            if (holder && b.dataset.rbValue !== undefined) {
                const id = holder.dataset.rb, v = Number(b.dataset.rbValue);
                if (id === 'transport.mode') { if (+transport.songMode !== v) press('mode'); }
                else if (/\.pattern$/.test(id)) setControl(id, M.patternIndex(rows[rowOf(id)].bank, v));
                else setControl(id, v);
                return;
            }
            const action = b.dataset.action;
            if (action && actions[action]) actions[action](b, e);
        });

        // ---- api -------------------------------------------------------------------------------
        function setAction(name, fn) {
            actions[name] = fn;
            const b = $('[data-action="' + name + '"]');
            if (b) b.disabled = !fn;
        }
        function activate(next) {
            if (disposed) return;
            const was = active;
            active = !!next;
            section.dataset.rbActive = active ? 'true' : 'false';
            if (active) {
                if (!host) ensureHost(); else wake();
                renderAll();
                startLoop();
            } else {
                stopLoop();
                if (was && hooks.leave) hooks.leave();
                if (host && !transport.playing && typeof host.suspend === 'function') { try { host.suspend(); } catch (_) { /* closing */ } }
            }
            if (hooks.activate) hooks.activate(active);
        }
        function stop() { if (ready() && transport.playing) press('stop'); }
        function dispose() {
            if (disposed) return;
            disposed = true;
            if (hooks.leave) hooks.leave();
            stopLoop();
            stopRepeat();
            ac.abort();
            if (host && typeof host.close === 'function') { try { host.close(); } catch (_) { /* already closed */ } }
            host = null;
        }
        const api = { activate, stop, isPlaying: () => !!(transport.playing || (snap && snap.playing)), dispose };
        // Internals for the later tasks (T8 behaviours/keys, T9 documents, T11 arrange): same file, no public contract.
        api.internals = { M, tr, E, section, rows, vals, mirror: () => mirror, setMirror, getVal, setControl, press, bump, toggle, emit, emitMany, flush, curPattern,
            transport: () => transport, snapshot: () => snap, host: () => host, state: () => hostState, info: () => info, setInfo, behaviours, actions, setAction, hooks,
            focusRow: () => focusRow, setFocusRow: r => { focusRow = r; }, markModified, isModified: () => modified, clearModified: () => { modified = false; }, renderAll, say, fmt, lock };
        return api;
    }

    return { create, rackMarkup };
});
