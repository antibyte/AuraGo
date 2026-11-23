(function () {
    'use strict';

    // Template variables and daily-note helpers for the Notes app.

    const DAILY_DIR = 'Documents/Notes/Daily';
    const TEMPLATE_DIR = 'Documents/Notes/Templates';

    const TOKEN_RE = /\[([^\]]*)\]|YYYY|YY|MMMM|MMM|MM|M|DD|D|dddd|ddd|HH|H|mm|ss/g;

    function pad(n) { return String(n).padStart(2, '0'); }

    function intl(date, lang, options) {
        try { return new Intl.DateTimeFormat(lang || 'en', options).format(date); }
        catch (_) { return new Intl.DateTimeFormat('en', options).format(date); }
    }

    function formatDate(date, format, lang) {
        const d = date instanceof Date ? date : new Date(date);
        return String(format == null ? '' : format).replace(TOKEN_RE, (token, literal) => {
            if (literal !== undefined) return literal;
            switch (token) {
                case 'YYYY': return String(d.getFullYear());
                case 'YY': return pad(d.getFullYear() % 100);
                case 'MMMM': return intl(d, lang, { month: 'long' });
                case 'MMM': return intl(d, lang, { month: 'short' });
                case 'MM': return pad(d.getMonth() + 1);
                case 'M': return String(d.getMonth() + 1);
                case 'DD': return pad(d.getDate());
                case 'D': return String(d.getDate());
                case 'dddd': return intl(d, lang, { weekday: 'long' });
                case 'ddd': return intl(d, lang, { weekday: 'short' });
                case 'HH': return pad(d.getHours());
                case 'H': return String(d.getHours());
                case 'mm': return pad(d.getMinutes());
                case 'ss': return pad(d.getSeconds());
                default: return token;
            }
        });
    }

    function expand(text, opts) {
        const o = opts || {};
        const title = o.title == null ? '' : String(o.title);
        const date = o.date || new Date();
        const lang = o.lang || 'en';
        return String(text == null ? '' : text).replace(/\{\{\s*(title|date|time)\s*(?::([^}]*))?\}\}/gi, (match, name, fmt) => {
            const n = name.toLowerCase();
            if (n === 'title') return fmt === undefined ? title : match;
            const format = fmt !== undefined && fmt.trim() ? fmt.trim() : (n === 'date' ? 'YYYY-MM-DD' : 'HH:mm');
            return formatDate(date, format, lang);
        });
    }

    function dailyPath(date) {
        return DAILY_DIR + '/' + formatDate(date, 'YYYY-MM-DD', 'en') + '.md';
    }

    function dailyDate(path) {
        const m = /^Documents\/Notes\/Daily\/(\d{4}-\d{2}-\d{2})\.md$/.exec(String(path || ''));
        return m ? m[1] : '';
    }

    function shiftDay(dateString, days) {
        const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(dateString || ''));
        if (!m) return '';
        // noon local time keeps the day stable across DST transitions
        const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]) + (Number(days) || 0), 12);
        return formatDate(d, 'YYYY-MM-DD', 'en');
    }

    function defaultDaily(date, lang) {
        return '# ' + intl(date, lang, { dateStyle: 'full' }) + '\n\n';
    }

    const api = { DAILY_DIR, TEMPLATE_DIR, formatDate, expand, dailyPath, dailyDate, shiftDay, defaultDaily };
    if (typeof window !== 'undefined') window.NotesTemplates = api;
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
})();
