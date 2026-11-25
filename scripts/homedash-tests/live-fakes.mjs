// Fakes for the HomeDash live-data tests: a manual clock with timers, and EventSource objects the test drives.
export function fakeClock(start = Date.UTC(2026, 10, 25, 10)) {
  let now = start, seq = 0; const timers = new Map();
  const flush = async () => { for (let i = 0; i < 25; i++) await Promise.resolve(); };
  return {
    now: () => now, flush,
    setTimeout(fn, ms) { const id = ++seq; timers.set(id, { at: now + Math.max(0, Number(ms) || 0), fn }); return id; },
    clearTimeout(id) { timers.delete(id); },
    delays: () => [...timers.values()].map(x => x.at - now).sort((a, b) => a - b),
    async advance(ms) {
      const end = now + ms; await flush();
      for (;;) {
        let pick = null;
        for (const [id, x] of timers) if (x.at <= end && (!pick || x.at < pick[1].at || (x.at === pick[1].at && id < pick[0]))) pick = [id, x];
        if (!pick) break;
        timers.delete(pick[0]); now = pick[1].at; pick[1].fn(); await flush();
      }
      now = end; await flush();
    } };
}
export function fakeSources() {
  const list = [];
  class FakeEventSource {
    constructor(url) { this.url = url; this.closed = false; this.listeners = new Map(); list.push(this); }
    addEventListener(type, fn) { if (!this.listeners.has(type)) this.listeners.set(type, []); this.listeners.get(type).push(fn); }
    emit(type, data) { for (const fn of this.listeners.get(type) || []) fn({ type, data: typeof data === 'string' ? data : JSON.stringify(data) }); }
    fail() { for (const fn of this.listeners.get('error') || []) fn({ type: 'error' }); }
    close() { this.closed = true; }
  }
  return { list, open: url => new FakeEventSource(url), last: () => list[list.length - 1] };
}
export function fakeStreamApi(sources) {
  const puts = []; let answer = ids => ({ status: 'ok', accepted: ids, rejected: [] });
  return { base: '/api/desktop/homedash', puts, openStream: () => sources.open('/api/desktop/homedash/stream'),
    putSubscriptions(streamId, ids) { const list = Array.from(ids); puts.push({ streamId, ids: list }); const a = answer(list); return a instanceof Error ? Promise.reject(a) : Promise.resolve(a); },
    setAnswer(fn) { answer = fn; } };
}
export const HELLO = { stream_id: 's1', protocol: 1, heartbeat_ms: 15000, limits: { entities: 300 }, server_time: '2026-11-25T10:00:00Z', principal: 'session' };
