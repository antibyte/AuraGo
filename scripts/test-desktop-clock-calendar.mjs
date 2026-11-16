import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

class FakeElement {
    constructor(document, id = '') {
        this.document = document;
        this.id = id;
        this.dataset = {};
        this.listeners = {};
        this.parentNode = null;
        this.attributes = {};
        this.hidden = false;
        this.innerHTML = '';
        this.isConnected = true;
    }
    addEventListener(type, handler) { this.listeners[type] = handler; }
    setAttribute(key, value) { this.attributes[key] = value; }
    focus() { this.document.activeElement = this; }
    contains(element) { for (let current = element; current; current = current.parentNode) if (current === this) return true; return false; }
    querySelector(selector) {
        if (!this.nodes) this.nodes = new Map();
        if (!this.nodes.has(selector)) {
            const node = new FakeElement(this.document);
            node.parentNode = this;
            const action = selector.match(/^\[data-clock-action="([^"]+)"\]$/);
            const day = selector.match(/^\[data-clock-day="([^"]+)"\]$/);
            if (action) node.dataset.clockAction = action[1];
            if (day) node.dataset.clockDay = day[1];
            this.nodes.set(selector, node);
        }
        return this.nodes.get(selector);
    }
    closest(selector) { return selector.includes('data-clock') ? this : null; }
}

const elements = new Map();
const document = {
    activeElement: null,
    documentElement: { lang: 'en' },
    listeners: {},
    body: { appendChild(element) { element.isConnected = true; element.parentNode = this; elements.set(element.id, element); } },
    createElement() { return new FakeElement(document); },
    getElementById(id) { return elements.get(id) || null; },
    addEventListener(type, handler) { this.listeners[type] = handler; },
    removeEventListener(type) { delete this.listeners[type]; }
};
const timeoutCallbacks = new Map();
let nextTimer = 0;
const window = {
    SYSTEM_LANG: 'en',
    innerHeight: 900,
    innerWidth: 1200,
    addEventListener() {},
    clearTimeout(id) { timeoutCallbacks.delete(id); },
    setTimeout(callback) { const id = ++nextTimer; timeoutCallbacks.set(id, callback); return id; }
};
const requests = [];
let nextResponse = null;
let openAppCalls = 0;
const context = vm.createContext({
    window,
    document,
    navigator: { language: 'en' },
    AbortController,
    setTimeout: window.setTimeout,
    clearTimeout: window.clearTimeout,
    t: key => key,
    esc: value => String(value).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;'),
    iconMarkup: () => '',
    api: (url, options = {}) => {
        const entry = { url, signal: options.signal };
        requests.push(entry);
        if (!nextResponse) return new Promise((resolve, reject) => { entry.resolve = resolve; entry.reject = reject; });
        const response = nextResponse;
        nextResponse = null;
        return response();
    },
    placeTrayPopover() {},
    openApp: name => { assert.equal(name, 'calendar'); openAppCalls++; }
});
const helperPath = fileURLToPath(new URL('../ui/js/desktop/apps/calendar-views.js', import.meta.url));
const runtimePath = fileURLToPath(new URL('../ui/js/desktop/core/clock-calendar-runtime.js', import.meta.url));
vm.runInContext(await readFile(helperPath, 'utf8'), context, { filename: helperPath });
vm.runInContext(await readFile(runtimePath, 'utf8'), context, { filename: runtimePath });

const leapMonth = context.calendarMonthCells(new Date(2024, 1, 15));
assert.ok(leapMonth.cells.some(day => context.isoDate(day) === '2024-02-29'), 'month grid includes leap day');
assert.equal(context.isoDate(context.calAddMonths(new Date(2024, 0, 31), 1)), '2024-02-29', 'month navigation clamps Jan 31 to leap day');
const dstDay = new Date(2024, 2, 10, 12);
const nextLocalDay = context.calAddDays(dstDay, 1);
assert.equal(nextLocalDay.getHours(), 12, 'local day movement preserves wall-clock time over DST');
assert.equal(context.isoDate(nextLocalDay), '2024-03-11');
const filterDate = new Date(2024, 1, 29);
assert.equal(context.calendarDayItems([
    { date_time: '2024-02-29T09:00:00' },
    { date_time: '2024-03-01T09:00:00' }
], filterDate).length, 1, 'day agenda filters in local calendar dates');

let panel;
const anchor = new FakeElement(document, 'clock-anchor');
document.activeElement = anchor;
context.openClockCalendarPopup(anchor);
panel = document.getElementById('vd-clock-popup');
assert.ok(panel && !panel.hidden);
assert.ok(panel.innerHTML.includes('desktop.loading'));
assert.equal(document.activeElement.dataset.clockAction, 'previous-month', 'opening focuses the first calendar navigation control');
assert.equal(requests.length, 1);
const aborted = requests[0];
context.closeClockCalendarPopup(true);
assert.equal(aborted.signal.aborted, true, 'closing aborts the in-flight appointment request');
assert.equal(document.activeElement, anchor);
aborted.resolve([{ id: 'late', title: 'Late result', date_time: '2026-01-01T10:00:00', status: 'upcoming' }]);
await new Promise(resolve => setImmediate(resolve));
assert.equal(panel.hidden, true);
assert.ok(!panel.innerHTML.includes('Late result'), 'late response cannot repopulate a closed popup');

const today = new Date();
const todayKey = context.isoDate(today);
const tomorrowKey = context.isoDate(context.calAddDays(today, 1));
context.openClockCalendarPopup(anchor);
const activeRequest = requests[1];
const outside = new FakeElement(document, 'outside');
document.activeElement = outside;
activeRequest.resolve([
    { id: 'today', title: 'Today appointment', date_time: `${todayKey}T09:00:00`, status: 'upcoming' },
    { id: 'tomorrow', title: 'Tomorrow appointment', date_time: `${tomorrowKey}T09:00:00`, status: 'upcoming' },
    { id: 'done', title: 'Completed appointment', date_time: `${todayKey}T11:00:00`, status: 'completed' },
    { id: 'cancelled', title: 'Cancelled appointment', date_time: `${todayKey}T12:00:00`, status: 'cancelled' }
]);
await new Promise(resolve => setImmediate(resolve));
assert.equal(document.activeElement, outside, 'async appointment completion never steals focus after it leaves the popup');
assert.ok(panel.innerHTML.includes('Today appointment'));
assert.ok(!panel.innerHTML.includes('Tomorrow appointment'), 'agenda shows only the selected local date');
assert.ok(!panel.innerHTML.includes('Completed appointment'));
assert.ok(!panel.innerHTML.includes('Cancelled appointment'));
const dayTarget = new FakeElement(document);
dayTarget.dataset.clockDay = tomorrowKey;
dayTarget.parentNode = panel;
document.activeElement = dayTarget;
panel.listeners.click({ target: dayTarget });
assert.ok(panel.innerHTML.includes('Tomorrow appointment'), 'selecting a calendar day updates its agenda');
const navTarget = new FakeElement(document);
navTarget.dataset.clockAction = 'next-month';
navTarget.parentNode = panel;
document.activeElement = navTarget;
panel.listeners.click({ target: navTarget });
assert.ok(panel.innerHTML.includes('Tomorrow appointment'), 'month navigation keeps the selected-day agenda visible');

context.closeClockCalendarPopup();
nextResponse = () => Promise.reject(new Error('temporary network failure'));
context.openClockCalendarPopup(anchor);
await new Promise(resolve => setImmediate(resolve));
assert.ok(panel.innerHTML.includes('desktop.clock_calendar_load_failed'), 'load failure is explicit');
assert.ok(!panel.innerHTML.includes('desktop.clock_no_events'), 'failed fetch is never presented as an empty calendar');
nextResponse = () => Promise.resolve([]);
const retry = new FakeElement(document);
retry.dataset.clockAction = 'retry';
panel.listeners.click({ target: retry });
await new Promise(resolve => setImmediate(resolve));
assert.ok(panel.innerHTML.includes('desktop.clock_no_events'), 'a successful retry can show an empty agenda');
assert.equal(requests.length, 4, 'retry performs one additional request');

const openCalendar = new FakeElement(document);
openCalendar.dataset.clockAction = 'open-calendar';
panel.listeners.click({ target: openCalendar });
assert.equal(openAppCalls, 1);
assert.equal(panel.hidden, true);

console.log('Desktop clock calendar checks passed.');
