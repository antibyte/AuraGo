// Runs ui/js/dashboard/dash-summary.js in a Node vm context with fake timers, a fake document and a stubbed
// DashAPI. Checks the pure badge and pill helpers, the sidebar badges, the coalesced 2 s SSE refresh, the 60 s
// poll while visible, the pause while hidden, the catch-up refresh on becoming visible and failure handling.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const file = path.join(__dirname, '..', 'js', 'dashboard', 'dash-summary.js');
const source = fs.readFileSync(file, 'utf8');
const json = value => JSON.stringify(value);

function fakeElement() {
    const attrs = {};
    const classes = new Set();
    return {
        textContent: '',
        hidden: true,
        attrs,
        classes,
        setAttribute(name, value) { attrs[name] = String(value); },
        removeAttribute(name) { delete attrs[name]; },
        classList: { toggle(name, on) { if (on) classes.add(name); else classes.delete(name); } },
    };
}

function load(answers) {
    const timers = [];
    let nextTimer = 1;
    const clock = { now: 1000 };
    const calls = [];
    const listeners = {};
    const badges = { overview: fakeElement(), problems: fakeElement(), automations: fakeElement() };
    const document = {
        visibilityState: 'visible',
        addEventListener: (type, fn) => { listeners[type] = fn; },
        querySelector: selector => {
            const match = /^\[data-nav-badge="([a-z]+)"\]$/.exec(selector);
            return match ? badges[match[1]] || null : null;
        },
    };
    const ctx = {
        console,
        document,
        t: (key, params) => (params && params.n !== undefined ? key + '|' + params.n : key),
        setTimeout: (fn, ms) => {
            const id = nextTimer++;
            timers.push({ id, at: clock.now + Math.max(0, Number(ms) || 0), fn });
            return id;
        },
        clearTimeout: id => {
            const at = timers.findIndex(timer => timer.id === id);
            if (at !== -1) timers.splice(at, 1);
        },
        DashAPI: {
            getWithStatus: (url, options) => {
                calls.push({ url, ttl: options && options.ttl, force: !!(options && options.force), at: clock.now });
                return Promise.resolve(answers.length > 1 ? answers.shift() : answers[0]);
            },
        },
    };
    ctx.window = ctx;
    vm.createContext(ctx);
    vm.runInContext(source, ctx, { filename: file });
    vm.runInContext('Date', ctx).now = () => clock.now;
    const flush = () => new Promise(resolve => setImmediate(resolve));
    async function advance(ms) {
        const end = clock.now + ms;
        for (;;) {
            timers.sort((a, b) => a.at - b.at || a.id - b.id);
            if (!timers.length || timers[0].at > end) break;
            const timer = timers.shift();
            clock.now = timer.at;
            timer.fn();
            await flush();
        }
        clock.now = end;
        await flush();
    }
    function setVisibility(state) {
        document.visibilityState = state;
        listeners.visibilitychange();
    }
    return { feed: ctx.DashSummary, calls, badges, advance, flush, setVisibility };
}

const row = (severity, code) => ({ id: code, severity, code, params: {}, section: 'problems', count: 1, at: '' });
const ok = data => ({ ok: true, data });
const mixed = { overall: 'critical', attention: [row('critical', 'disk_high'), row('warning', 'cron_errors'), row('warning', 'missions_failed'), row('info', 'memory_conflicts')], attention_more: 0, badges: { problems: 3, automations: 0 } };
const full = { overall: 'warning', attention: Array.from({ length: 8 }, (_, i) => row('warning', 'w' + i)), attention_more: 2, badges: { problems: 0, automations: 5 } };
const infoTail = { overall: 'critical', attention: [row('critical', 'c')].concat(Array.from({ length: 7 }, (_, i) => row('info', 'i' + i))), attention_more: 4, badges: {} };

async function main() {
    {
        const { feed } = load([ok(mixed)]);
        assert.ok(feed, 'dash-summary.js must define window.DashSummary without touching the DOM at load time');
        assert.equal(feed.overviewBadgeText(mixed), '3', 'critical + warning rows');
        assert.equal(feed.overviewBadgeText(full), '8+', 'rows were cut off after a warning row');
        assert.equal(feed.overviewBadgeText(infoTail), '1', 'cut-off info rows do not count');
        assert.equal(feed.overviewBadgeText({ overall: 'ok', attention: [], attention_more: 0 }), '');
        assert.equal(feed.overviewBadgeText(null), '');
        assert.equal(json(feed.badgeTexts(mixed)), json({ overview: '3', problems: '3', automations: '' }));
        const tr = (key, params) => key.replace('dashboard.overview.', '') + (params ? ':' + params.n : '');
        assert.equal(feed.overallText(mixed, tr), 'pill_critical:1, pill_warning:2');
        assert.equal(feed.overallText(full, tr), 'pill_warning:8+');
        assert.equal(feed.overallText({ overall: 'ok', attention: [row('info', 'x')] }, tr), 'pill_ok', 'info rows keep ok');
        assert.equal(feed.overallText(null, tr), 'pill_unavailable');
    }
    {
        const env = load([ok(mixed), ok(full)]);
        env.feed.init();
        await env.flush();
        assert.equal(env.calls.length, 1, 'init refreshes once');
        assert.equal(json(env.calls[0]), json({ url: '/api/dashboard/summary', ttl: 15000, force: false, at: 1000 }));
        assert.equal(env.feed.latest().overall, 'critical');
        assert.equal(env.badges.problems.textContent, '3');
        assert.equal(env.badges.problems.hidden, false);
        assert.equal(env.badges.problems.attrs['aria-label'], 'dashboard.overview.badge_problems|3');
        assert.equal(env.badges.automations.hidden, true, 'a zero badge stays hidden');
        assert.equal(env.badges.overview.textContent, '3');
        assert.equal(env.badges.overview.classes.has('is-warning'), false, 'a critical row makes the overview badge red');

        const seen = [];
        env.feed.onUpdate(summary => seen.push(summary.overall));
        env.feed.scheduleRefresh();
        await env.advance(500);
        env.feed.scheduleRefresh();
        env.feed.scheduleRefresh();
        await env.advance(1499);
        assert.equal(env.calls.length, 1, 'no refresh before 2 s');
        await env.advance(1);
        assert.equal(env.calls.length, 2, 'a burst of SSE triggers costs one refresh');
        assert.equal(env.calls[1].force, true, 'SSE refreshes bypass the 15 s cache');
        assert.equal(json(seen), '["warning"]', 'listeners get the new summary');
        assert.equal(env.badges.overview.textContent, '8+');
        assert.equal(env.badges.overview.classes.has('is-warning'), true);
        assert.equal(env.badges.automations.textContent, '5');

        await env.advance(59999);
        assert.equal(env.calls.length, 2, 'no poll before 60 s');
        await env.advance(1);
        assert.equal(env.calls.length, 3, 'polls 60 s after the last fetch while visible');

        env.setVisibility('hidden');
        await env.advance(180000);
        assert.equal(env.calls.length, 3, 'no polling while hidden');
        env.feed.scheduleRefresh();
        await env.advance(5000);
        assert.equal(env.calls.length, 3, 'SSE triggers wait while hidden');
        env.setVisibility('visible');
        await env.flush();
        assert.equal(env.calls.length, 4, 'catch-up refresh when the page becomes visible again');

        await env.advance(10000);
        env.setVisibility('hidden');
        env.setVisibility('visible');
        await env.flush();
        assert.equal(env.calls.length, 4, 'no extra refresh when the last fetch is younger than 60 s');
        await env.advance(50000);
        assert.equal(env.calls.length, 5, 'the poll resumes with the remaining time');
    }
    {
        const env = load([ok(mixed), { ok: false, status: 500 }]);
        env.feed.init();
        await env.flush();
        const result = await env.feed.refresh({ force: true });
        assert.equal(json(result), json({ ok: false, status: 500 }));
        assert.equal(env.feed.latest().overall, 'critical', 'a failed refresh keeps the last summary');
    }
    console.log('DashSummary: badges, pill, debounce, 60 s poll, visibility and failure handling pass.');
}

main().catch(error => { console.error(error); process.exitCode = 1; });
