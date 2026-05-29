(function () {
    'use strict';

    function normalizeStreamEvent(data) {
        const event = data && (data.event || data.type) || '';
        return Object.assign({}, data || {}, { event, type: event });
    }

    async function readFetchEventStream(response, handlers = {}) {
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        try {
            while (true) {
                const { done, value } = await reader.read();
                if (done) break;
                buffer += decoder.decode(value, { stream: true });
                const lines = buffer.split('\n');
                buffer = lines.pop() || '';
                for (const line of lines) {
                    if (!line.startsWith('data: ')) continue;
                    const raw = line.slice(6).trim();
                    if (raw === '[DONE]') {
                        if (typeof handlers.onDone === 'function') handlers.onDone();
                        try { await reader.cancel(); } catch (_) {}
                        return;
                    }
                    try {
                        const parsed = JSON.parse(raw);
                        if (typeof handlers.onEvent === 'function') {
                            handlers.onEvent(normalizeStreamEvent(parsed));
                        }
                    } catch (err) {
                        if (typeof handlers.onParseError === 'function') handlers.onParseError(err);
                    }
                }
            }
            if (typeof handlers.onDone === 'function') handlers.onDone();
        } catch (err) {
            if (typeof handlers.onError === 'function') handlers.onError(err);
            else throw err;
        }
    }

    window.AuraChatStreamParser = {
        normalizeStreamEvent,
        readFetchEventStream
    };
})();
