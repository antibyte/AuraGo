import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const requests = [];
const host = { innerHTML: '', scrollTop: 0, querySelectorAll: () => [], querySelector: () => null };
const head = { innerHTML: '' };
const context = {
    AbortController, clearTimeout() {}, setTimeout: () => 1,
    document: { activeElement: null }, $: id => id === 'vd-start-apps' ? host : head,
    api() {}, t: key => key, esc: value => String(value), spotlightLocalSources: () => ({}),
    startEmptyMarkup: text => text,
    window: { AuraDesktopSearch: { createProvider: () => ({
        local: query => [{ id: query, title: 'local:' + query }],
        files: (query, options) => new Promise((resolve, reject) => requests.push({ query, options, resolve, reject }))
    }) } }
};
const source = await readFile(new URL('../ui/js/desktop/core/start-search-runtime.js', import.meta.url), 'utf8');
vm.runInNewContext(source + '\nglobalThis.search = renderStartSearch; globalThis.cancel = cancelStartSearch; globalThis.page = () => loadStartSearchPage(startSearchRequest);', context);
context.search('older');
assert.match(host.innerHTML, /local:older/, 'local hits render before a network request');
const older = context.page();
context.search('newer');
assert.equal(requests[0].options.signal.aborted, true);
const newer = context.page();
requests[1].resolve({ items: [{ id: 'new-file', title: 'new file', path: 'new' }], nextOffset: 50, hasMore: true });
await newer;
assert.match(host.innerHTML, /new file/);
assert.match(host.innerHTML, /data-search-more/);
requests[0].resolve({ items: [{ id: 'old-file', title: 'old file' }] });
await older;
assert.doesNotMatch(host.innerHTML, /old file/, 'late responses never replace a newer search');
const next = context.page();
assert.equal(requests[2].options.offset, 50);
requests[2].reject(new Error('unavailable'));
await next;
assert.match(host.innerHTML, /start_search_failed/);
assert.match(host.innerHTML, /new file/, 'a failed next page retains confirmed results');
const cancelled = context.page();
context.cancel();
requests[3].resolve({ items: [{ id: 'closed-file', title: 'closed file' }] });
await cancelled;
assert.doesNotMatch(host.innerHTML, /closed file/);
console.log('Start search immediate hits, independent aborts, late replies and paging checked');
