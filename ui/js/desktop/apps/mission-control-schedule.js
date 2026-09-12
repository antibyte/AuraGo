// Mission Control – pure cron schedule helpers (no DOM, no globals besides the export).
// Loaded before the other Mission Control modules; safe to run in Node for tests.
(function () {
    'use strict';

    const PRESETS = [
        { value: '*/5 * * * *', labelKey: 'missions.cron_preset_every_5min' },
        { value: '*/15 * * * *', labelKey: 'missions.cron_preset_every_15min' },
        { value: '*/30 * * * *', labelKey: 'missions.cron_preset_every_30min' },
        { value: '0 * * * *', labelKey: 'missions.cron_preset_every_hour' },
        { value: '0 */6 * * *', labelKey: 'missions.cron_preset_every_6hours' },
        { value: '0 0 * * *', labelKey: 'missions.cron_preset_daily_midnight' },
        { value: '0 9 * * *', labelKey: 'missions.cron_preset_daily_9am' },
        { value: '0 9 * * 1', labelKey: 'missions.cron_preset_weekly_monday' },
        { value: '0 0 1 * *', labelKey: 'missions.cron_preset_monthly_first' }
    ];
    const QUICK_PRESETS = ['0 * * * *', '0 9 * * *', '0 9 * * 1', '0 0 1 * *'];
    const MINUTE_STEPS = [5, 10, 15, 30];
    const HOUR_STEPS = [2, 3, 4, 6, 8, 12];
    const DESCRIPTORS = { '@hourly': 'hourly', '@daily': 'daily', '@midnight': 'daily', '@weekly': 'weekly', '@monthly': 'monthly', '@yearly': 'yearly', '@annually': 'yearly' };
    const NUM = /^\d{1,2}$/;
    const FIELD = /^[0-9A-Za-z*\/,\-?#LW]+$/;
    const STEP = /^\*\/(\d{1,2})$/;
    const WEEKDAY_FALLBACK = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

    function defaultState() {
        return { mode: 'daily', every: 15, minute: 0, hour: 9, weekdays: [1], dayOfMonth: 1, cron: '0 9 * * *' };
    }

    function num(value) { return NUM.test(value) ? Number(value) : NaN; }

    function clamp(value, lo, hi) {
        const n = Number(value);
        if (Number.isNaN(n)) return lo;
        return Math.min(hi, Math.max(lo, Math.trunc(n)));
    }

    function custom(state, raw) { state.mode = 'custom'; state.cron = raw; return state; }

    function parse(expr) {
        const state = defaultState();
        const raw = String(expr || '').trim();
        if (!raw) return state;
        state.cron = raw;
        const fields = raw.split(/\s+/);
        if (fields.length !== 5) return custom(state, raw);
        const [min, hour, dom, mon, dow] = fields;
        if (mon !== '*') return custom(state, raw);
        const stepMin = STEP.exec(min);
        if (stepMin) {
            if (hour !== '*' || dom !== '*' || dow !== '*') return custom(state, raw);
            const every = Number(stepMin[1]);
            return MINUTE_STEPS.includes(every) ? Object.assign(state, { mode: 'minutes', every }) : custom(state, raw);
        }
        const stepHour = STEP.exec(hour);
        if (stepHour) {
            if (min !== '0' || dom !== '*' || dow !== '*') return custom(state, raw);
            const every = Number(stepHour[1]);
            return HOUR_STEPS.includes(every) ? Object.assign(state, { mode: 'hours', every }) : custom(state, raw);
        }
        const minute = num(min);
        if (Number.isNaN(minute) || minute > 59) return custom(state, raw);
        if (hour === '*' && dom === '*' && dow === '*') return Object.assign(state, { mode: 'hourly', minute });
        const h = num(hour);
        if (Number.isNaN(h) || h > 23) return custom(state, raw);
        if (dom === '*' && dow === '*') return Object.assign(state, { mode: 'daily', minute, hour: h });
        if (dom === '*') {
            const days = dow.split(',').map(num);
            if (days.some(d => Number.isNaN(d) || d > 7)) return custom(state, raw);
            const weekdays = Array.from(new Set(days.map(d => (d === 7 ? 0 : d)))).sort((a, b) => a - b);
            return Object.assign(state, { mode: 'weekly', minute, hour: h, weekdays });
        }
        if (dow === '*') {
            const day = num(dom);
            if (Number.isNaN(day) || day < 1 || day > 31) return custom(state, raw);
            return Object.assign(state, { mode: 'monthly', minute, hour: h, dayOfMonth: day });
        }
        return custom(state, raw);
    }

    function build(state) {
        const s = Object.assign(defaultState(), state || {});
        const minute = clamp(s.minute, 0, 59);
        const hour = clamp(s.hour, 0, 23);
        switch (s.mode) {
            case 'minutes': return `*/${MINUTE_STEPS.includes(Number(s.every)) ? Number(s.every) : 15} * * * *`;
            case 'hours': return `0 */${HOUR_STEPS.includes(Number(s.every)) ? Number(s.every) : 6} * * *`;
            case 'hourly': return `${minute} * * * *`;
            case 'daily': return `${minute} ${hour} * * *`;
            case 'weekly': {
                const days = Array.from(new Set((s.weekdays || []).map(Number).filter(d => d >= 0 && d <= 6))).sort((a, b) => a - b);
                return `${minute} ${hour} * * ${days.length ? days.join(',') : '1'}`;
            }
            case 'monthly': return `${minute} ${hour} ${clamp(s.dayOfMonth, 1, 31)} * *`;
            default: return String(s.cron || '').trim();
        }
    }

    function validate(expr) {
        const raw = String(expr || '').trim();
        if (!raw) return false;
        if (raw.startsWith('@')) {
            return /^@(hourly|daily|midnight|weekly|monthly|yearly|annually)$/i.test(raw)
                || /^@every\s+(\d+(ns|us|µs|ms|s|m|h))+$/i.test(raw);
        }
        const fields = raw.split(/\s+/);
        if (fields.length !== 5 && fields.length !== 6) return false;
        return fields.every(field => FIELD.test(field));
    }

    function isSupported(expr) { return parse(expr).mode !== 'custom'; }

    function pad2(n) { return String(n).padStart(2, '0'); }

    function formatTime(hour, minute, lang) {
        try {
            return new Intl.DateTimeFormat(lang || undefined, { hour: '2-digit', minute: '2-digit' }).format(new Date(2000, 0, 1, hour, minute));
        } catch (_) {
            return `${pad2(hour)}:${pad2(minute)}`;
        }
    }

    function weekdayNames(days, lang) {
        let fmt = null;
        try { fmt = new Intl.DateTimeFormat(lang || undefined, { weekday: 'short' }); } catch (_) { fmt = null; }
        // 2023-01-01 is a Sunday, so day index d maps to January 1 + d.
        return (days || []).map(d => (fmt ? fmt.format(new Date(2023, 0, 1 + d)) : WEEKDAY_FALLBACK[d] || String(d)));
    }

    function describe(expr, t, lang) {
        const raw = String(expr || '').trim();
        if (!raw) return '';
        if (raw.startsWith('@')) {
            const name = DESCRIPTORS[raw.toLowerCase()];
            if (name) return t('desktop.mc_schedule_descriptor_' + name);
            const every = /^@every\s+(\S+)$/i.exec(raw);
            if (every) return t('desktop.mc_schedule_every_duration', { duration: every[1] });
            return t('desktop.mc_schedule_custom_desc');
        }
        const s = parse(raw);
        const time = () => formatTime(s.hour, s.minute, lang);
        switch (s.mode) {
            case 'minutes': return t('desktop.mc_schedule_every_minutes', { count: s.every });
            case 'hours': return t('desktop.mc_schedule_every_hours', { count: s.every });
            case 'hourly': return t('desktop.mc_schedule_hourly_at', { minute: pad2(s.minute) });
            case 'daily': return t('desktop.mc_schedule_daily_at', { time: time() });
            case 'weekly':
                if (s.weekdays.length === 7) return t('desktop.mc_schedule_daily_at', { time: time() });
                return t('desktop.mc_schedule_weekly_at', { days: weekdayNames(s.weekdays, lang).join(', '), time: time() });
            case 'monthly': return t('desktop.mc_schedule_monthly_at', { day: s.dayOfMonth, time: time() });
            default: return t('desktop.mc_schedule_custom_desc');
        }
    }

    window.MissionControlSchedule = { PRESETS, QUICK_PRESETS, MINUTE_STEPS, HOUR_STEPS, parse, build, describe, validate, isSupported, formatTime, weekdayNames };
})();
