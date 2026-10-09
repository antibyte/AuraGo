// Behaviour checks for ui/cfg/local_wikipedia.js against a small fake DOM:
// status error wording, loading, poll errors, focus, the live region and the
// install questions. Run with `npm run test:local-wikipedia-config`.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const bundle = JSON.parse(readFileSync(path.join(root, 'ui/lang/config/local_wikipedia/en.json'), 'utf8'));
const source = readFileSync(path.join(root, 'ui/cfg/local_wikipedia.js'), 'utf8').replace(/\r\n?/g, '\n');

// A status area whose buttons and state banner can be queried and focused.
function makeRuntime(document) {
  const runtime = {
    nodes: [],
    html: '',
    get innerHTML() { return this.html; },
    set innerHTML(value) {
      this.html = value;
      this.nodes = [];
      for (const match of value.matchAll(/<button[^>]*data-lw-action="([a-z]+)"[^>]*>/g)) {
        this.nodes.push(makeNode(document, 'button', match[1], /\sdisabled(\s|>)/.test(match[0])));
      }
      if (value.includes('id="lw-state"')) this.nodes.push(makeNode(document, 'state', '', false));
    },
    contains(node) { return this.nodes.includes(node); },
    querySelector(selector) {
      const action = selector.match(/^\[data-lw-action="([a-z]+)"\]:not\(\[disabled\]\)$/);
      if (action) return this.nodes.find(node => node.kind === 'button' && node.dataset.lwAction === action[1] && !node.disabled) || null;
      if (selector === '#lw-state') return this.nodes.find(node => node.kind === 'state') || null;
      if (selector === '[data-lw-action]:not([disabled])') return this.nodes.find(node => node.kind === 'button' && !node.disabled) || null;
      return null;
    },
  };
  return runtime;
}

function makeNode(document, kind, action, disabled) {
  return {
    kind,
    disabled,
    id: kind === 'state' ? 'lw-state' : '',
    dataset: kind === 'button' ? { lwAction: action } : {},
    focus() { document.activeElement = this; },
  };
}

function makeContext() {
  const state = { dirty: false, responses: [], confirmAnswers: [] };
  const calls = [];
  const confirms = [];
  const document = {
    activeElement: null,
    elements: {},
    getElementById(id) { return this.elements[id] || null; },
    querySelector(selector) {
      if (selector.includes('language')) return { value: '', options: [], addEventListener() {} };
      if (selector.includes('variant')) return { value: 'nopic', options: [], addEventListener() {} };
      return null;
    },
    addEventListener() {},
  };
  const runtime = makeRuntime(document);
  const announce = { writes: 0, text: '', set textContent(value) { this.writes += 1; this.text = value; }, get textContent() { return this.text; } };
  document.elements['lw-runtime'] = runtime;
  document.elements['lw-announce'] = announce;
  document.elements.content = { innerHTML: '' };
  const sandbox = {
    console,
    setTimeout: () => 1,
    clearTimeout() {},
    lang: 'en',
    configData: { local_wikipedia: { enabled: true, variant: 'nopic', language: '' } },
    escapeHtml: value => String(value).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
    escapeAttr: value => String(value).replace(/&/g, '&amp;').replace(/"/g, '&quot;'),
    t(key, params) {
      let text = bundle[key] || key;
      if (params) Object.entries(params).forEach(([name, value]) => { text = text.replaceAll('{' + name + '}', value); });
      return text;
    },
    hasUnsavedConfigChanges: () => state.dirty,
    isDockerRuntime: () => false,
    attachChangeListeners() {},
    toggleBool() {},
    showConfirm: async (title, message) => { confirms.push({ title, message }); return state.confirmAnswers.length ? state.confirmAnswers.shift() : true; },
    document,
    window: { addEventListener() {}, setTimeout: () => 1 },
    fetch: async (url, options) => {
      calls.push({ url, body: options && options.body ? JSON.parse(options.body) : null });
      const next = state.responses.shift();
      if (!next) throw new Error('unexpected fetch ' + url);
      if (typeof next === 'function') return next();
      return { ok: next.status < 400, status: next.status, json: async () => next.body };
    },
  };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  return {
    sandbox, runtime, announce, calls, confirms, state, document,
    run: code => vm.runInContext(code, sandbox),
    setStatus(status) { vm.runInContext('_lwStatus = ' + JSON.stringify(status), sandbox); },
    html() { return vm.runInContext('localWikiRuntimeHTML()', sandbox); },
  };
}

const edition = { language: 'de', variant: 'nopic', date: '2026-10', size: 18585337585, article_count: 3000000 };
const ready = {
  state: 'ready', edition, readable: true, fulltext: true, selection_matches_installed: true, loading: false,
  selection: { language: 'de', variant: 'nopic' }, free_bytes: 500e9, data_dir: '/data/wikipedia',
  operation_in_progress: false, system_language: 'de', languages: [],
};
const SERVER_ENGLISH = 'Server recommendation that must never be shown.';

// The server's English recommendation is never shown; the wording is derived locally.
{
  const ctx = makeContext();
  // Installed edition unreadable: delete it.
  ctx.setStatus({ ...ready, state: 'error', readable: false, fulltext: false, error_code: 'zim_unreadable', recommendation: SERVER_ENGLISH });
  let html = ctx.html();
  assert.ok(html.includes(bundle['config.local_wikipedia.error_zim_unreadable']));
  assert.ok(!html.includes(SERVER_ENGLISH));
  assert.ok(html.includes(bundle['config.local_wikipedia.unreadable']));
  assert.ok(html.includes('data-lw-action="delete"'));
  assert.ok(!html.includes('data-lw-action="install"') && !html.includes('data-lw-action="check"'));

  // A downloaded file that cannot be read while the installed edition keeps serving.
  ctx.setStatus({ ...ready, error_code: 'zim_unreadable', recommendation: SERVER_ENGLISH });
  html = ctx.html();
  assert.ok(html.includes(bundle['config.local_wikipedia.error_download_unreadable']));
  assert.ok(html.includes(bundle['config.local_wikipedia.update_failed']));
  assert.ok(!html.includes(bundle['config.local_wikipedia.error_zim_unreadable']));
  assert.ok(!html.includes(SERVER_ENGLISH));

  // A download nobody could read when nothing is installed.
  ctx.setStatus({ state: 'error', readable: false, selection_matches_installed: false, error_code: 'zim_unreadable', recommendation: SERVER_ENGLISH });
  html = ctx.html();
  assert.ok(html.includes(bundle['config.local_wikipedia.error_download_unreadable']));
  assert.ok(!html.includes(bundle['config.local_wikipedia.update_failed']));
  assert.ok(html.includes('data-lw-action="install"'));

  // Other codes use the per-code text; a failed update says the edition keeps working.
  ctx.setStatus({ ...ready, state: 'interrupted', error_code: 'download_failed', recommendation: SERVER_ENGLISH });
  html = ctx.html();
  assert.ok(html.includes(bundle['config.local_wikipedia.update_failed'] + ' ' + bundle['config.local_wikipedia.error_download_failed']));
  assert.ok(html.includes('data-lw-action="resume"'));
  ctx.setStatus({ ...ready, state: 'interrupted', error_code: 'insufficient_disk_space', required_bytes: 20e9, free_bytes: 5e9 });
  assert.ok(ctx.html().includes('20 GB needed, 5 GB available'));
  ctx.setStatus({ ...ready, error_code: 'fulltext_unsupported', fulltext: false });
  html = ctx.html();
  assert.ok(html.includes(bundle['config.local_wikipedia.error_fulltext_unsupported']));
  assert.ok(!html.includes(bundle['config.local_wikipedia.update_failed']));
  ctx.setStatus({ ...ready, error_code: 'data_dir_invalid' });
  assert.ok(!ctx.html().includes(bundle['config.local_wikipedia.update_failed']));
  ctx.setStatus({ ...ready, error_code: 'checksum_mismatch' });
  assert.ok(ctx.html().includes(bundle['config.local_wikipedia.update_failed'] + ' ' + bundle['config.local_wikipedia.error_checksum_mismatch']));
}

// loading: neither the not-installed nor the busy view; polling goes on.
{
  const ctx = makeContext();
  ctx.setStatus({ state: 'not_installed', readable: false, loading: true, error_code: 'busy', selection_matches_installed: false });
  const html = ctx.html();
  assert.ok(html.includes(bundle['config.local_wikipedia.loading_edition']));
  assert.ok(!html.includes(bundle['config.local_wikipedia.state_not_installed']));
  assert.ok(!html.includes(bundle['config.local_wikipedia.error_busy']));
  assert.ok(!html.includes('data-lw-action'));
  let scheduled = 0;
  ctx.sandbox.setTimeout = () => { scheduled += 1; return 7; };
  ctx.run('localWikiSchedulePolling()');
  assert.equal(scheduled, 1, 'a loading status must be polled');
  ctx.setStatus({ ...ready });
  ctx.run('localWikiSchedulePolling()');
  assert.equal(scheduled, 1, 'an idle status must not be polled');
}

// A failed poll is kept apart from the action messages and cleared by the next success.
{
  const ctx = makeContext();
  ctx.state.responses.push({ status: 503, body: { error: 'localwiki_unavailable' } });
  await ctx.run('localWikiRefreshStatus()');
  assert.equal(ctx.run('_lwPollError').startsWith(bundle['config.local_wikipedia.error_prefix']), true);
  assert.equal(ctx.run('_lwMessage'), null);
  assert.ok(ctx.runtime.innerHTML.includes('data-lw-action="reload"'));
  ctx.state.responses.push({ status: 200, body: { ...ready } });
  ctx.state.responses.push({ status: 200, body: { language: 'de', fulltext: true, variants: { nopic: { date: '2026-10', size: 1e9 } } } });
  await ctx.run('localWikiRefreshStatus()');
  assert.equal(ctx.run('_lwPollError'), '');
  assert.ok(!ctx.runtime.innerHTML.includes('data-lw-action="reload"'));
  assert.ok(ctx.runtime.innerHTML.includes(bundle['config.local_wikipedia.state_ready']));
  // A failure while a status is known keeps the status and shows the error beside it.
  ctx.state.responses.push(() => { throw new Error('network down'); });
  await ctx.run('localWikiRefreshStatus()');
  assert.ok(ctx.runtime.innerHTML.includes('network down'));
  assert.ok(ctx.runtime.innerHTML.includes(bundle['config.local_wikipedia.state_ready']));
}

// A status answer that arrives after leaving the section starts no catalog fetch.
{
  const ctx = makeContext();
  ctx.state.responses.push(() => {
    delete ctx.document.elements['lw-runtime'];
    return { ok: true, status: 200, json: async () => ({ ...ready }) };
  });
  await ctx.run('localWikiRefreshStatus()');
  assert.deepEqual(ctx.calls.map(call => call.url), ['/api/local-wikipedia/status']);
  assert.equal(ctx.run('_lwStatus'), null, 'a late answer is dropped');
}

// Focus: kept on the button while it exists, else the first enabled action, else the state banner.
{
  const ctx = makeContext();
  ctx.setStatus({ state: 'not_installed', readable: false, selection_matches_installed: false, free_bytes: 1e9 });
  ctx.run('localWikiUpdateRuntimeDOM()');
  const install = ctx.runtime.nodes.find(node => node.dataset.lwAction === 'install');
  install.focus();
  ctx.state.dirty = true; // Install is disabled now.
  ctx.run('localWikiUpdateRuntimeDOM()');
  assert.equal(ctx.document.activeElement.kind, 'state', 'no enabled action left: focus the state banner');

  ctx.setStatus({ ...ready });
  ctx.state.dirty = false;
  ctx.run('localWikiUpdateRuntimeDOM()');
  ctx.runtime.nodes.find(node => node.dataset.lwAction === 'check').focus();
  ctx.setStatus({ ...ready, update_available: { date: '2026-11', size: 1 } });
  ctx.run('localWikiUpdateRuntimeDOM()');
  assert.equal(ctx.document.activeElement.dataset.lwAction, 'check', 'the same button keeps the focus');

  ctx.setStatus({ ...ready, state: 'downloading', operation_in_progress: true, bytes_done: 1, bytes_total: 2 });
  ctx.run('localWikiUpdateRuntimeDOM()');
  assert.equal(ctx.document.activeElement.dataset.lwAction, 'cancel', 'the first enabled action takes over');

  // Focus outside the status area is left alone.
  const outside = { dataset: {}, id: 'other', focus() { ctx.document.activeElement = this; } };
  outside.focus();
  ctx.setStatus({ ...ready });
  ctx.run('localWikiUpdateRuntimeDOM()');
  assert.equal(ctx.document.activeElement, outside);
}

// One persistent live region: it announces changes once, never progress, and no banner is a live region.
{
  const ctx = makeContext();
  ctx.setStatus({ ...ready });
  ctx.run('localWikiUpdateRuntimeDOM()');
  assert.equal(ctx.announce.text, bundle['config.local_wikipedia.state_ready']);
  const writes = ctx.announce.writes;
  ctx.run('localWikiUpdateRuntimeDOM()');
  assert.equal(ctx.announce.writes, writes, 'unchanged text is not written again');
  ctx.setStatus({ ...ready, state: 'downloading', operation_in_progress: true, bytes_done: 1, bytes_total: 4 });
  ctx.run('localWikiUpdateRuntimeDOM()');
  const downloading = ctx.announce.writes;
  ctx.setStatus({ ...ready, state: 'downloading', operation_in_progress: true, bytes_done: 2, bytes_total: 4 });
  ctx.run('localWikiUpdateRuntimeDOM()');
  assert.equal(ctx.announce.writes, downloading, 'progress is not announced');
  ctx.setStatus({ ...ready, error_code: 'download_failed' });
  ctx.run('localWikiUpdateRuntimeDOM()');
  assert.ok(ctx.announce.text.includes(bundle['config.local_wikipedia.error_download_failed']));
  assert.ok(!/role="(alert|status)"|aria-live/.test(ctx.runtime.innerHTML), 'the status area holds no live region');
}

// Delete waits for saved changes; Install questions.
{
  const ctx = makeContext();
  ctx.setStatus({ ...ready });
  ctx.state.dirty = true;
  const html = ctx.html();
  assert.match(html, /data-lw-action="delete"[^>]* disabled/);
  assert.match(html, /data-lw-action="check"[^>]* disabled/);
  await ctx.run('localWikiDelete()');
  assert.equal(ctx.confirms.length, 0);
  assert.equal(ctx.calls.length, 0);
  ctx.state.dirty = false;
  assert.doesNotMatch(ctx.html(), /data-lw-action="delete"[^>]* disabled/);
}
{
  const ctx = makeContext();
  ctx.setStatus({ state: 'not_installed', selection_matches_installed: false, free_bytes: -1 });
  ctx.run('_lwCatalog = ' + JSON.stringify({ language: 'de', fulltext: true, variants: { nopic: { date: '2026-10', size: 18585337585 } } }));
  ctx.state.responses.push(
    { status: 409, body: { error: 'free_space_unknown', error_code: 'free_space_unknown' } },
    { status: 202, body: { status: 'accepted' } },
    { status: 200, body: { ...ready, state: 'downloading', operation_in_progress: true } },
  );
  await ctx.run("localWikiInstall('install')");
  const posts = ctx.calls.filter(call => call.url.endsWith('/install'));
  assert.deepEqual(posts.map(post => post.body), [
    { replace_mode: 'keep_old', confirm_unknown_space: false },
    { replace_mode: 'keep_old', confirm_unknown_space: true },
  ]);
  assert.equal(ctx.confirms.length, 2);
}
{
  const ctx = makeContext();
  ctx.setStatus({ ...ready, selection_matches_installed: false });
  ctx.state.responses.push(
    { status: 422, body: { error: 'insufficient_disk_space', error_code: 'insufficient_disk_space', required_bytes: 20e9, free_bytes: 5e9, can_delete_old: true } },
    { status: 202, body: { status: 'accepted' } },
    { status: 200, body: { ...ready, state: 'downloading', operation_in_progress: true } },
  );
  await ctx.run("localWikiInstall('install')");
  assert.deepEqual(ctx.calls.filter(call => call.url.endsWith('/install')).map(call => call.body.replace_mode), ['keep_old', 'delete_old_first']);
  assert.ok(ctx.confirms[1].message.includes('20 GB are needed, but only 5 GB are free'));
}
{
  const ctx = makeContext();
  ctx.setStatus({ state: 'not_installed', selection_matches_installed: false });
  ctx.state.responses.push(
    { status: 422, body: { error: 'insufficient_disk_space', error_code: 'insufficient_disk_space', required_bytes: 20e9, free_bytes: 5e9, can_delete_old: false } },
    { status: 200, body: { state: 'not_installed', selection_matches_installed: false, free_bytes: 5e9, data_dir: '/d' } },
  );
  await ctx.run("localWikiInstall('install')");
  assert.equal(ctx.confirms.length, 1);
  assert.ok(ctx.runtime.innerHTML.includes('20 GB needed, 5 GB available'));
}
for (const [code, key, status] of [
  ['busy', 'error_busy', 409], ['already_installed', 'error_already_installed', 409], ['disabled', 'error_disabled', 409],
  ['data_dir_invalid', 'error_data_dir_invalid', 422],
]) {
  const ctx = makeContext();
  ctx.setStatus({ state: 'not_installed', selection_matches_installed: false });
  ctx.state.responses.push(
    { status, body: { error: code, error_code: code } },
    { status: 200, body: { state: 'not_installed', selection_matches_installed: false, free_bytes: 5e9, data_dir: '/d' } },
  );
  await ctx.run("localWikiInstall('resume')");
  assert.ok(ctx.runtime.innerHTML.includes(bundle['config.local_wikipedia.' + key]), code);
}

// Server text is escaped wherever it is rendered.
{
  const ctx = makeContext();
  ctx.setStatus({ ...ready, edition: { ...edition, language: '<img src=x>' } });
  assert.ok(!ctx.html().includes('<img'));
}

console.log('local wikipedia config checks passed');
