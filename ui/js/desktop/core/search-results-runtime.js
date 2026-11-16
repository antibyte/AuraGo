(function () {
    'use strict';

    const DEFAULT_LIMIT = 50;
    const MAX_LIMIT = 100;
    const MAX_OFFSET = 1000;
    const TYPE_ORDER = Object.freeze({ app: 0, setting: 1, action: 2, calculator: 3, file: 4 });

    function normalizedText(value) {
        return String(value == null ? '' : value).normalize('NFKC').toLocaleLowerCase().trim();
    }

    function queryTerms(query) {
        return normalizedText(query).split(/\s+/).filter(Boolean);
    }

    function itemSearchText(item) {
        return normalizedText([
            item.title,
            item.subtitle,
            item.description,
            item.category,
            item.settingKey,
            item.expression,
            item.searchText,
            Array.isArray(item.keywords) ? item.keywords.join(' ') : item.keywords
        ].filter(Boolean).join(' '));
    }

    function searchScore(item, terms, query) {
        if (!terms.length) return 0;
        const title = normalizedText(item.title);
        const haystack = itemSearchText(item);
        if (!terms.every(term => haystack.includes(term))) return -1;
        if (title.startsWith(query)) return 0;
        if (title.includes(query)) return 1;
        if (haystack.includes(query)) return 2;
        return 3;
    }

    function sourceItems(source, query, context) {
        if (typeof source === 'function') {
            const items = source(query, context);
            return Array.isArray(items) ? items : [];
        }
        return Array.isArray(source) ? source : [];
    }

    function arithmeticResult(query, sources) {
        const expression = String(query || '').trim();
        if (expression.length < 3 || expression.length > 80 || !/\d/.test(expression) || !/[+*/%^()!-]/.test(expression)) return null;
        if (!/^[\d\s.+\-*/%^()!a-z]+$/i.test(expression)) return null;
        const calculator = window.AuraCalculatorExpression;
        if (!calculator || typeof calculator.evaluate !== 'function') return null;
        try {
            const value = calculator.evaluate(expression);
            const rendered = String(value);
            const t = typeof sources.t === 'function' ? sources.t : key => key;
            return {
                id: 'calculator-' + expression,
                type: 'calculator',
                title: expression + ' = ' + rendered,
                subtitle: t('desktop.spotlight_calculator'),
                expression,
                value: rendered,
                searchText: expression,
                open: typeof sources.openCalculator === 'function' ? () => sources.openCalculator(expression) : null
            };
        } catch (_) {
            return null;
        }
    }

    function localResults(query, sources, options) {
        const source = sources || {};
        const terms = queryTerms(query);
        const normalizedQuery = normalizedText(query);
        const groups = [
            ['app', source.apps],
            ['setting', source.settings],
            ['action', source.actions],
            ['calculator', source.calculator],
            ['file', source.files]
        ];
        const ranked = [];
        let order = 0;
        groups.forEach(([type, entries]) => {
            sourceItems(entries, query, source).forEach(entry => {
                if (!entry || typeof entry !== 'object') return;
                const item = Object.assign({}, entry, {
                    id: String(entry.id || (type + '-' + order)),
                    type: entry.type || type,
                    title: String(entry.title || ''),
                    subtitle: entry.subtitle == null ? '' : String(entry.subtitle)
                });
                if (!item.title) return;
                const score = searchScore(item, terms, normalizedQuery);
                if (score >= 0) ranked.push({ item, score, order: order++ });
            });
        });
        const calc = arithmeticResult(query, source);
        if (calc) {
            const score = searchScore(calc, terms, normalizedQuery);
            if (score >= 0) ranked.push({ item: calc, score: Math.min(score, 1), order: order++ });
        }
        ranked.sort((a, b) => a.score - b.score ||
            (TYPE_ORDER[a.item.type] ?? 10) - (TYPE_ORDER[b.item.type] ?? 10) ||
            a.order - b.order || a.item.id.localeCompare(b.item.id));
        const limit = Number.isInteger(options && options.limit) ? Math.max(0, options.limit) : 100;
        return ranked.slice(0, limit).map(entry => entry.item);
    }

    function makeSearchURL(query, options) {
        const params = new URLSearchParams();
        params.set('query', String(query || '').trim());
        params.set('limit', String(options.limit));
        params.set('offset', String(options.offset));
        if (options.path) params.set('path', String(options.path));
        const filters = options.filters || {};
        const type = String(filters.type || filters.filter || 'all');
        if (type) params.set('type', type);
        if (filters.modifiedAfter) params.set('modified_after', String(filters.modifiedAfter));
        if (filters.modifiedBefore) params.set('modified_before', String(filters.modifiedBefore));
        if (filters.minSize != null && filters.minSize !== '') params.set('min_size', String(filters.minSize));
        if (filters.maxSize != null && filters.maxSize !== '') params.set('max_size', String(filters.maxSize));
        return '/api/desktop/search?' + params.toString();
    }

    function hasActiveFileFilters(filters) {
        const active = filters && typeof filters === 'object' ? filters : {};
        const type = String(active.type || active.filter || 'all').toLowerCase();
        return type !== '' && type !== 'all' || !!(active.modifiedAfter || active.modifiedBefore) ||
            active.minSize != null && active.minSize !== '' || active.maxSize != null && active.maxSize !== '';
    }

    async function defaultRequest(url, init) {
        const response = await window.fetch(url, Object.assign({ credentials: 'same-origin' }, init || {}));
        if (!response.ok) throw new Error('Desktop search failed: ' + response.status);
        return response.json();
    }

    function createProvider(options) {
        const config = options || {};
        return Object.freeze({
            local(query, sources, localOptions) {
                return localResults(query, sources || config.sources, localOptions);
            },
            async files(query, requestOptions) {
                const request = requestOptions || {};
                const text = String(query || '').trim();
                const filters = request.filters || {};
                if (text.length < 2 && !hasActiveFileFilters(filters)) return { items: [], hasMore: false, truncated: false, nextOffset: 0 };
                const limit = Math.min(MAX_LIMIT, Math.max(1, Number.isInteger(request.limit) ? request.limit : DEFAULT_LIMIT));
                const offset = Math.min(MAX_OFFSET, Math.max(0, Number.isInteger(request.offset) ? request.offset : 0));
                const api = typeof request.api === 'function' ? request.api : typeof config.api === 'function' ? config.api : defaultRequest;
                const response = await api(makeSearchURL(text, {
                    limit,
                    offset,
                    path: request.path,
                    filters
                }), { signal: request.signal });
                const files = Array.isArray(response && response.files) ? response.files : Array.isArray(response && response.results) ? response.results : [];
                const openFile = request.openFile || config.openFile;
                return {
                    items: files.map(file => ({
                        id: 'file-' + String(file.path || file.name || ''),
                        type: 'file',
                        title: String(file.name || file.path || ''),
                        subtitle: String(file.path || ''),
                        path: String(file.path || ''),
                        entry: file,
                        searchText: String(file.path || ''),
                        open: typeof openFile === 'function' ? () => openFile(file.path, file) : null
                    })),
                    hasMore: Boolean(response && (response.has_more ?? response.hasMore)),
                    truncated: Boolean(response && response.truncated),
                    nextOffset: Number.isInteger(response && response.next_offset) ? response.next_offset : Number.isInteger(response && response.nextOffset) ? response.nextOffset : offset + files.length
                };
            }
        });
    }

    const defaultProvider = createProvider();
    window.AuraDesktopSearch = Object.freeze({
        createProvider,
        local: (query, sources, options) => defaultProvider.local(query, sources, options),
        files: (query, options) => defaultProvider.files(query, options),
        makeSearchURL,
        normalizedText
    });
}());
