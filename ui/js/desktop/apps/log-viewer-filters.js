(function () {
    'use strict';

    const LEVELS = ['DEBUG', 'INFO', 'WARN', 'ERROR'];
    const STORAGE_PREFIX = 'aurago.desktop.log_viewer.filters.';

    function cloneLevels(source) {
        const levels = {};
        LEVELS.forEach((level) => {
            levels[level] = !source || source[level] !== false;
        });
        return levels;
    }

    function compileRegex(query) {
        try {
            return new RegExp(query, 'i');
        } catch (_) {
            return null;
        }
    }

    function recordText(record) {
        if (!record) return '';
        const attrs = record.attrs && typeof record.attrs === 'object'
            ? Object.keys(record.attrs).map((key) => key + '=' + record.attrs[key]).join(' ')
            : '';
        return [record.raw, record.msg, record.level, record.time, attrs].join(' ');
    }

    function create(initial) {
        const state = {
            levels: cloneLevels(initial && initial.levels),
            query: String((initial && initial.query) || ''),
            regex: !!(initial && initial.regex)
        };

        function apply(records) {
            const list = Array.isArray(records) ? records : [];
            const query = state.query.trim();
            const matcher = state.regex && query ? compileRegex(query) : null;
            const needle = query.toLowerCase();
            return list.filter((record) => {
                const level = String(record && record.level || '').toUpperCase();
                if (level && state.levels[level] === false) return false;
                if (!level && state.levels.INFO === false) return false;
                if (!query) return true;
                const hay = recordText(record);
                if (state.regex) return !!(matcher && matcher.test(hay));
                return hay.toLowerCase().indexOf(needle) !== -1;
            });
        }

        function toggleLevel(level) {
            const key = String(level || '').toUpperCase();
            if (!Object.prototype.hasOwnProperty.call(state.levels, key)) return state.levels[key];
            state.levels[key] = !state.levels[key];
            return state.levels[key];
        }

        function setQuery(query) {
            state.query = String(query == null ? '' : query);
            return state.query;
        }

        function setRegex(enabled) {
            state.regex = !!enabled;
            return state.regex;
        }

        function serialize() {
            return {
                levels: cloneLevels(state.levels),
                query: state.query,
                regex: state.regex
            };
        }

        function deserialize(data) {
            if (!data || typeof data !== 'object') return serialize();
            state.levels = cloneLevels(data.levels);
            state.query = String(data.query || '');
            state.regex = !!data.regex;
            return serialize();
        }

        function persist(windowId) {
            if (!windowId) return;
            try {
                localStorage.setItem(STORAGE_PREFIX + windowId, JSON.stringify(serialize()));
            } catch (_) {}
        }

        function restore(windowId) {
            if (!windowId) return serialize();
            try {
                const raw = localStorage.getItem(STORAGE_PREFIX + windowId);
                if (!raw) return serialize();
                return deserialize(JSON.parse(raw));
            } catch (_) {
                return serialize();
            }
        }

        return {
            levels: state.levels,
            apply,
            toggleLevel,
            setQuery,
            setRegex,
            serialize,
            deserialize,
            persist,
            restore,
            get query() { return state.query; },
            get regex() { return state.regex; },
            LEVELS
        };
    }

    window.LogViewerFilters = { create, LEVELS, STORAGE_PREFIX };
}());
