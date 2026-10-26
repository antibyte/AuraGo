import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';

const rerender = page => page.evaluate(() => render());
// The text a control's aria-describedby points to (null when it has none).
export const describedText = (page, selector) => page.evaluate(selector => {
    const id = document.querySelector(selector)?.getAttribute('aria-describedby');
    return id ? document.getElementById(id)?.textContent ?? null : null;
}, selector);
const iso = ms => new Date(Date.now() + ms).toISOString();
const IDLE = {ready: false, fresh: false, data_state: 'unchecked', last_frame_age_ms: null, observed_at: null, last_fresh_at: null};
const dict = lang => readFile(`ui/lang/ruview/${lang}.json`, 'utf8').then(text => JSON.parse(text));

// The first status answer has been applied: the headline left "loading".
export const resolved = page => page.locator('[data-ruview-headline][data-state]:not([data-state=loading])').waitFor();
export const waitState = (page, id, timeout = 15000) => page.waitForFunction(id => document.querySelector('[data-ruview-headline]')?.dataset.state === id, id, {timeout});

// Opens a view the way a user does: the tab when it is shown, the view menu below 720 px.
export async function openView(page, view) {
    await page.waitForFunction(view => !!document.querySelector(`[data-ruview-tablist] [data-view="${view}"]:not([hidden])`), view);
    const tab = page.locator(`[data-ruview-tablist] [data-view="${view}"]`);
    if (await tab.isVisible()) await tab.click();
    else {
        await page.locator('[data-ruview-view-menu]').click();
        await page.locator(`[data-view-option="${view}"]`).click();
    }
    await page.locator(`[data-panel="${view}"]:not([hidden])`).waitFor();
}
// Opens one original page in the Lab; a page that is already open is closed first.
export async function openLab(page, name) {
    await openView(page, 'lab');
    const back = page.locator('[data-lab-close="back"]');
    if (await back.isVisible()) await back.click();
    await page.locator(`[data-lab-open="${name}"]`).click();
    await page.locator('iframe').waitFor();
}

export async function checkDevices(page, fixture) {
    let release;
    fixture.hold = new Promise(resolve => { release = resolve; });
    fixture.statusFail = true;
    const feedback = () => page.locator('[data-ruview-boards-feedback]');
    try {
        const age = Number(await page.locator('[data-age]').getAttribute('data-age-ms'));
        await page.waitForFunction(age => Number(document.querySelector('[data-age]')?.dataset.ageMs) > age + 500, age);
        await page.waitForFunction(() => Number(document.querySelector('[data-age]')?.dataset.ageMs) > 10000, {}, {timeout: 16000});
        assert.match(await page.locator('[data-fresh]').innerText(), /Veraltet/i);
        assert.equal(fixture.max, 1, 'device polling must not overlap');
        fixture.fail = true; release(); fixture.hold = null;
        await page.waitForFunction(() => document.querySelector('[data-ruview-boards-feedback]')?.textContent.includes('Aktualisierung fehlgeschlagen'));
        assert.equal(await page.locator('.ruview-node').count(), 1, 'failed refresh retains old values');
        fixture.fail = false; fixture.statusFail = false;
        await page.waitForFunction(() => document.querySelector('[data-ruview-boards-feedback]')?.textContent === '');
        fixture.hold = new Promise(resolve => { release = resolve; });
        await page.waitForRequest(request => request.url().endsWith('/ruview/nodes'));
        await openView(page, 'status');
        await openView(page, 'boards'); await page.waitForTimeout(4200);
        assert.equal(fixture.max, 1, 'returning to boards must not overlap a previous request');
        release(); fixture.hold = null;
        await page.waitForFunction(() => Number(document.querySelector('[data-age]')?.dataset.ageMs) < 1000);
        fixture.offline = true;
        await page.waitForFunction(() => document.querySelector('[data-fresh]')?.textContent === 'Keine aktuellen Sensordaten');
        assert.equal(await page.locator('.ruview-node').count(), 1, 'empty upstream after restart retains known boards');
        assert.match(await page.locator('.ruview-node').innerText(), /-51/, 'last RSSI remains explicitly offline');
        fixture.unavailable = true;
        await page.waitForFunction(() => document.querySelector('[data-ruview-boards-feedback]')?.textContent.includes('Aktualisierung fehlgeschlagen'));
        assert.equal(await page.locator('.ruview-node').count(), 1, 'unreachable upstream retains known boards');
        await rerender(page); await openView(page, 'boards'); await page.locator('.ruview-node').waitFor();
        assert.equal(await page.locator('[data-age]').innerText(), '—', 'reopening restores identity, never fake historical measurements');
        fixture.offline = false; fixture.unavailable = false;
        await page.waitForFunction(() => document.querySelector('[data-fresh]')?.textContent === 'Frische Sensordaten empfangen');
        fixture.offline = true; fixture.scope = 'different-receiver';
        await page.waitForFunction(() => document.querySelector('[data-age]')?.textContent === '—');
        assert.ok(!(await page.locator('.ruview-node').innerText()).includes('-51'), 'another receiver must not inherit previous measurements');
        fixture.offline = false; fixture.scope = 'fixture';
        await page.waitForFunction(() => document.querySelector('[data-fresh]')?.textContent === 'Frische Sensordaten empfangen');
        await page.evaluate(() => Object.defineProperty(document, 'hidden', {configurable: true, value: true}));
        const reads = fixture.reads; await page.waitForTimeout(4200);
        assert.equal(fixture.reads, reads, 'hidden document pauses polling');
        await page.evaluate(() => delete document.hidden);
        await page.waitForFunction(() => document.querySelector('[data-fresh]')?.textContent === 'Frische Sensordaten empfangen');
        await page.locator('#app').evaluate(el => el.style.display = 'none');
        const hiddenReads = fixture.reads; await page.waitForTimeout(4200);
        assert.equal(fixture.reads, hiddenReads, 'hidden desktop window pauses polling');
        await page.locator('#app').evaluate(el => el.style.display = '');
        assert.equal(await feedback().innerText(), '');
        // D6: Status never dials an external service; a managed service has its board list read for the count.
        await openView(page, 'status');
        const externalReads = fixture.reads; await page.waitForTimeout(4200);
        assert.equal(fixture.reads, externalReads, 'Status does not read the board list of an external service');
        fixture.statusExtra = {mode: 'docker', runtime: 'running'};
        const managedReads = fixture.reads;
        for (let waited = 0; fixture.reads === managedReads && waited < 9000; waited += 250) await page.waitForTimeout(250);
        assert.ok(fixture.reads > managedReads, 'Status reads the board list of a managed esp32 service');
        assert.equal(fixture.max, 1, 'status refresh does not overlap');
        fixture.statusExtra = null;
        await openView(page, 'boards'); await page.locator('.ruview-node').waitFor();
        await page.evaluate(() => RuViewApp.dispose('test'));
        const closedReads = fixture.reads, closedAge = await page.locator('[data-age]').innerText(); await page.waitForTimeout(4200);
        assert.equal(fixture.reads, closedReads); assert.equal(await page.locator('[data-age]').innerText(), closedAge, 'disposed app has no age timer');
        await rerender(page); await openView(page, 'boards'); await page.locator('.ruview-node').waitFor();
    } finally {
        release(); fixture.hold = null; fixture.fail = false; fixture.statusFail = false; fixture.offline = false; fixture.unavailable = false;
        fixture.scope = 'fixture'; fixture.statusExtra = null; await page.evaluate(() => delete document.hidden);
    }
}

// Status wording for states that are not failures (German locale, the app already open).
export async function checkStatusStates(page, fixture, ctl) {
    const text = await dict('de');
    try {
        // A source without boards: no Boards tab, an explanation in Details, and no receiver or UDP step.
        ctl.source = 'wifi'; fixture.liveState = 'not_applicable'; fixture.statusExtra = {mode: 'docker', runtime: 'not_started', ...IDLE};
        await rerender(page); await waitState(page, 'stopped');
        assert.equal(await page.locator('[data-ruview-tablist] [data-view="boards"]:not([hidden])').count(), 0, 'Wi-Fi has no Boards tab');
        assert.equal(await page.locator('[data-view-option="boards"]:not([hidden])').count(), 0);
        assert.ok((await page.locator('[data-detail=source]').textContent()).includes(text['ruview.devices_not_applicable']), 'Details explain why there are no boards');
        assert.equal(await page.locator('[data-step=receiver]').count(), 0, 'a source without UDP has no receiver step');
        assert.equal(await page.locator('[data-step=service]').getAttribute('data-step-state'), 'current');
        assert.ok(!(await page.locator('#app').innerText()).includes('Aktualisierung fehlgeschlagen'));
        ctl.source = 'esp32'; fixture.liveState = '';

        // M30: a stopped managed service has udp_ready=false by construction, so the service is the step to fix, not UDP.
        fixture.statusExtra = {mode: 'docker', runtime: 'not_started', udp_scope: 'managed', udp_enabled: true, udp_ready: false, receiver_configured: true, ...IDLE};
        await rerender(page); await waitState(page, 'stopped');
        assert.equal(await page.locator('[data-step=service]').getAttribute('data-step-state'), 'current');
        assert.notEqual(await page.locator('[data-step=receiver]').getAttribute('data-step-state'), 'done', 'UDP enabled is not UDP listening');
        assert.equal(await page.locator('[data-step=receiver]').getAttribute('data-step-state'), 'todo');
        // While the service starts, the setup step's start button is blocked and its row says why.
        fixture.statusExtra = {mode: 'docker', runtime: 'starting', phase: 'creating', phase_since: iso(-5000), ...IDLE};
        await rerender(page); await waitState(page, 'starting');
        const fix = page.locator('[data-step=service] [data-step-fix]');
        assert.equal(await fix.getAttribute('aria-disabled'), 'true');
        assert.equal(await describedText(page, '[data-step=service] [data-step-fix]'), text['ruview.reason_wait_start']);
        fixture.statusExtra = null;

        // A disabled integration: "off", no Boards or Lab, and the board list is never asked for.
        fixture.disabled = true;
        await rerender(page); await waitState(page, 'off');
        for (const view of ['boards', 'lab']) assert.equal(await page.locator(`[data-ruview-tablist] [data-view="${view}"]:not([hidden])`).count(), 0, `${view} is hidden while RuView is off`);
        // The check is blocked while RuView is off, and the menu item says why.
        await page.locator('[data-ruview-more]').click();
        assert.equal(await page.locator('[data-ruview-menu] [data-action=test]').getAttribute('aria-disabled'), 'true');
        assert.equal(await describedText(page, '[data-ruview-menu] [data-action=test]'), text['ruview.enable_first']);
        assert.equal(await page.locator('[data-ruview-menu] [data-action=test] .ruview-menuitem-reason').isVisible(), true);
        await page.keyboard.press('Escape');
        const reads = fixture.reads; await page.waitForTimeout(4200);
        assert.equal(fixture.reads, reads, 'a disabled integration reads nothing');
        assert.ok(!(await page.locator('#app').innerText()).includes('Aktualisierung fehlgeschlagen'));
        fixture.disabled = false;

        // The first status request failing says "unavailable", never "turned off".
        fixture.statusFail = true;
        await rerender(page); await waitState(page, 'status_failed');
        assert.equal(await page.locator('[data-ruview-headline-text]').innerText(), text['ruview.headline_status_failed']);
        assert.ok(!(await page.locator('#app').innerText()).includes(text['ruview.headline_off']), 'an unknown status must not read as turned off');
        fixture.statusFail = false;
        await waitState(page, 'live');
    } finally { ctl.source = 'esp32'; fixture.liveState = ''; fixture.disabled = false; fixture.statusExtra = null; fixture.statusFail = false; }
}

// Check 1: every state shows its headline and next step without scrolling (1100 x 760, German).
const STATE_FIXTURES = [
    ['off', () => ({enabled: false, ...IDLE, data_state: 'disabled'})],
    ['installing', () => ({mode: 'native', runtime: 'not_started', phase: 'downloading', phase_since: iso(-5000), ...IDLE})],
    ['readonly_blocked', () => ({mode: 'docker', readonly: true, runtime: 'not_started', ...IDLE})],
    ['not_installed', () => ({mode: 'native', runtime: 'not_started', error_code: 'not_installed', ...IDLE})],
    ['stopped', () => ({mode: 'docker', runtime: 'stopped', stop_reason: 'user', stopped_at: iso(-60000), ...IDLE})],
    ['starting_pull', () => ({mode: 'docker', runtime: 'starting', phase: 'pulling_image', phase_since: iso(-5000), ...IDLE})],
    ['starting', () => ({mode: 'docker', runtime: 'starting', phase: 'creating', phase_since: iso(-5000), ...IDLE})],
    ['stopping', () => ({mode: 'docker', runtime: 'stopping', ...IDLE})],
    ['stop_failed', () => ({mode: 'docker', runtime: 'stop_unconfirmed', error_code: 'stop_failed', ...IDLE})],
    ['error', () => ({mode: 'docker', runtime: 'not_started', error_code: 'image_denied', error_detail: 'HTTP 403: registry denied', ...IDLE})],
    ['external_unchecked', () => ({observed_at: null, ready: false, fresh: false, last_frame_age_ms: null})],
    ['external_check_old', () => ({observed_at: iso(-120000)})],
    ['demo', () => ({source: 'simulated', simulated: true, data_state: 'synthetic', fresh: false})],
    ['waiting', () => ({waiting_for_frames: true, fresh: false, last_frame_age_ms: null})],
    ['disconnected', () => ({data_state: 'disconnected', fresh: false, last_frame_age_ms: null})],
    ['live', () => ({})],
    ['stale', () => ({data_state: 'stale', fresh: false, last_frame_age_ms: 20000})]
];
export async function checkStateFixtures(page, fixture) {
    const measure = () => page.evaluate(() => {
        const next = document.querySelector('[data-ruview-next]'), headline = document.querySelector('[data-ruview-headline]').getBoundingClientRect();
        return {
            state: document.querySelector('[data-ruview-headline]').dataset.state, scrollTop: document.querySelector('.ruview-body').scrollTop,
            barBottom: document.querySelector('.ruview-commandbar').getBoundingClientRect().bottom, headlineTop: headline.top, headlineBottom: headline.bottom,
            nextBottom: next && !next.hidden ? next.getBoundingClientRect().bottom : null, viewport: innerHeight
        };
    });
    try {
        await page.setViewportSize({width: 1100, height: 760});
        fixture.statusExtra = null; fixture.statusFail = false;
        await rerender(page); await waitState(page, 'live');
        // Row 2: the status fails after a first success and the last known state is kept.
        fixture.statusFail = true;
        await waitState(page, 'status_failed');
        let m = await measure();
        assert.ok(m.scrollTop === 0 && m.headlineTop >= m.barBottom - 1 && m.headlineBottom <= m.viewport, 'status_failed fits');
        assert.notEqual(m.nextBottom, null, 'status_failed offers Try again');
        fixture.statusFail = false;
        for (const [id, make] of STATE_FIXTURES) {
            fixture.statusExtra = make();
            await rerender(page); await waitState(page, id);
            m = await measure();
            assert.equal(m.state, id);
            assert.equal(m.scrollTop, 0, `${id}: the body is not scrolled`);
            assert.ok(m.headlineTop >= m.barBottom - 1 && m.headlineBottom <= m.viewport, `${id}: the headline is visible without scrolling`);
            assert.ok(m.nextBottom === null || m.nextBottom <= m.viewport, `${id}: the next step is visible without scrolling`);
        }
    } finally { fixture.statusExtra = null; fixture.statusFail = false; }
}

// Check 2: the command bar is one 44 px row at 560 px, on Status and on Boards.
export async function checkCommandBar(page) {
    await page.setViewportSize({width: 560, height: 760});
    try {
        await rerender(page); await resolved(page);
        for (const view of ['status', 'boards']) {
            await openView(page, view);
            const bar = await page.evaluate(() => {
                const element = document.querySelector('.ruview-commandbar');
                const tops = [...element.children].filter(child => child.getClientRects().length > 0).map(child => child.getBoundingClientRect().top);
                return {height: element.offsetHeight, overflow: element.scrollWidth > element.clientWidth, tops};
            });
            assert.equal(bar.height, 44, `${view}: the command bar keeps its height`);
            assert.equal(bar.overflow, false, `${view}: the command bar does not overflow`);
            assert.ok(bar.tops.length >= 3 && bar.tops.every(top => Math.abs(top - bar.tops[0]) <= 1), `${view}: the command bar does not wrap (${bar.tops})`);
        }
    } finally { await page.setViewportSize({width: 1100, height: 760}); }
}

// Check 3: focus survives two polls on Status and on Boards.
export async function checkFocusKept(page, fixture) {
    const survive = async selector => {
        await page.locator(selector).focus();
        await page.evaluate(selector => { document.querySelector(selector).dataset.focusProbe = '1'; }, selector);
        const reads = fixture.statusReads;
        for (let waited = 0; fixture.statusReads < reads + 2 && waited < 12000; waited += 250) await page.waitForTimeout(250);
        assert.ok(fixture.statusReads >= reads + 2, 'two polls ran');
        await page.waitForTimeout(100);
        assert.equal(await page.evaluate(() => document.activeElement?.dataset.focusProbe), '1', `${selector} keeps focus across polls`);
    };
    fixture.statusExtra = null;
    await rerender(page); await waitState(page, 'live');
    await survive('[data-ruview-next]');
    await openView(page, 'boards'); await page.locator('button[data-node="7"]').waitFor();
    await survive('button[data-node="7"]');
    await openView(page, 'status');
}

// Check 4: the live region is silent on routine polls and announces a changed state exactly once.
export async function checkLiveRegion(page, fixture, ctl) {
    await rerender(page); await waitState(page, 'live');
    await page.evaluate(() => {
        window.__liveChanges = 0;
        new MutationObserver(records => { window.__liveChanges += records.length; }).observe(document.querySelector('[data-ruview-live]'), {childList: true, characterData: true, subtree: true});
    });
    const reads = fixture.statusReads;
    for (let waited = 0; fixture.statusReads < reads + 2 && waited < 12000; waited += 250) await page.waitForTimeout(250);
    await page.waitForTimeout(300);
    assert.equal(await page.evaluate(() => window.__liveChanges), 0, 'routine polls do not announce');
    ctl.offline = true;
    await page.waitForFunction(() => window.__liveChanges > 0, {}, {timeout: 12000});
    await page.waitForTimeout(500);
    try {
        assert.equal(await page.evaluate(() => window.__liveChanges), 1, 'the change is announced once');
        const [announced, headline] = await page.evaluate(() => [document.querySelector('[data-ruview-live]').textContent, document.querySelector('[data-ruview-headline-text]').textContent]);
        assert.equal(announced, headline, 'the announcement is the new headline');
    } finally { ctl.offline = false; }
}

// Check 5: a Docker image denial is the headline cause; the raw text stays inert under Technical details.
export async function checkImageDenied(page, fixture) {
    const detail = 'HTTP 403: registry denied <img src=x onerror="window.ruviewInjected=1">';
    const text = await dict('de');
    try {
        fixture.statusExtra = {mode: 'docker', runtime: 'not_started', error_code: 'image_denied', error_detail: detail, ...IDLE};
        await rerender(page); await waitState(page, 'error');
        const headline = await page.locator('[data-ruview-headline-text]').innerText();
        assert.equal(headline, text['ruview.err_image_denied']);
        assert.ok(!headline.includes('registry'), 'the headline names the cause, not the raw text');
        assert.equal(await page.locator('[data-ruview-technical-text]').textContent(), detail, 'the raw text is shown as text');
        assert.equal(await page.locator('#app img').count(), 0);
        assert.equal(await page.evaluate(() => !window.ruviewInjected), true);
        assert.equal(await page.locator('[data-ruview-next]').getAttribute('data-action'), 'runtime/start');
    } finally { fixture.statusExtra = null; }
}

// Check 6 (D4): "Allow changes" saves exactly one field, and only after the user confirms.
export async function checkAllowChanges(page, fixture, ctl) {
    const text = await dict('de');
    const answer = value => page.evaluate(value => { window.__ruviewAnswer = value; window.__ruviewModals = []; }, value);
    try {
        ctl.mode = 'docker'; ctl.readonly = true;
        fixture.statusExtra = {runtime: 'not_started', ...IDLE};
        await rerender(page); await waitState(page, 'readonly_blocked');
        const next = page.locator('[data-ruview-next][data-action=allow-changes]');
        // The desktop modal answers false for Cancel, Escape and a click outside; only its 'confirm' choice is a yes.
        for (const no of [false, null, 'cancel']) {
            await answer(no);
            await next.click(); await page.waitForTimeout(400);
            assert.equal(ctl.configPuts.length, 0, `declining the dialog (${JSON.stringify(no)}) saves nothing`);
        }
        const asked = await page.evaluate(() => window.__ruviewModals);
        assert.deepEqual(asked, [{title: text['ruview.allow_changes_title'], message: text['ruview.allow_changes_body'], choices: [{value: 'confirm', label: text['ruview.allow_changes_confirm']}]}], 'the modal offers one confirm choice');

        // The save is refused: the cause is shown, announced once, nothing is retried, and the state stays read-only.
        ctl.putStatus = 403;
        await answer('confirm');
        await next.click();
        await page.waitForFunction(() => { const block = document.querySelector('[data-ruview-action-error]'); return block && !block.hidden; });
        const failed = await page.locator('[data-ruview-action-error]').textContent();
        assert.ok(failed.includes(text['ruview.allow_changes_failed']), 'the failure is named');
        assert.ok(failed.includes(text['ruview.err_forbidden']), 'the cause is translated');
        await page.waitForFunction(failed => document.querySelector('[data-ruview-live]').textContent.includes(failed), text['ruview.allow_changes_failed']);
        assert.equal(await page.locator('[data-ruview-headline]').getAttribute('data-state'), 'readonly_blocked');
        await page.waitForTimeout(3600);
        assert.equal(ctl.configPuts.length, 1, 'a refused save is not repeated');
        assert.equal(await page.locator('[data-ruview-action-error]').isVisible(), true, 'the failure outlives a poll');

        // The save works: one more PUT with exactly one field, and the headline moves on.
        ctl.putStatus = 0; ctl.configPuts.length = 0;
        await answer('confirm');
        await next.click();
        await waitState(page, 'stopped');
        assert.equal(ctl.configPuts.length, 1, 'confirming saves once');
        assert.deepEqual(JSON.parse(ctl.configPuts[0].body), {ruview: {readonly: false}});
        assert.equal(ctl.configPuts[0].type, 'application/json');
        assert.equal(await page.locator('[data-ruview-action-error]').isVisible(), false, 'the failure is gone after a new action');
    } finally { ctl.putStatus = 0; ctl.mode = 'external'; ctl.readonly = false; fixture.statusExtra = null; ctl.configPuts.length = 0; await answer('confirm'); }
}

export async function checkPolicy(frame, setReadonly) {
    await frame.evaluate(() => {
        const host = document.createElement('div'); host.id = 'policy-fixture';
        for (const [id, action, disabled] of [['start', 'training_start', true], ['stop', 'training_stop', false], ['load', 'model_load', false]]) {
            const button = document.createElement('button'); button.id = 'policy-' + id; button.dataset.ruviewAction = action; button.disabled = disabled; button.textContent = 'Localized label'; host.append(button);
        }
        document.body.append(host);
    });
    try {
        setReadonly(true);
        await frame.waitForFunction(() => document.getElementById('policy-start').disabled && !document.getElementById('policy-stop').disabled);
        assert.equal(await frame.locator('#policy-load').isDisabled(), true);
        assert.equal(await frame.evaluate(async () => { try { await fetch('/api/v1/recording/start', {method: 'POST'}); return false } catch { return true } }), true);
        assert.equal(await frame.evaluate(async () => { try { await fetch('/api/v1/recording/rec_fixture', {method: 'DELETE'}); return false } catch { return true } }), true);
        assert.equal(await frame.evaluate(async () => (await fetch('/api/v1/recording/stop', {method: 'POST'})).ok), true);
        assert.equal(await frame.evaluate(async () => (await fetch('/api/v1/train/stop', {method: 'POST'})).ok), true);
        setReadonly(false); await frame.waitForTimeout(3400);
        assert.equal(await frame.locator('#policy-start').isDisabled(), true, 'permission changes preserve business-disabled training');
        await frame.evaluate(() => document.getElementById('policy-start').disabled = false);
        assert.equal(await frame.locator('#policy-start').isDisabled(), false);
        setReadonly(true); await frame.waitForFunction(() => document.getElementById('policy-start').disabled);
        await frame.evaluate(() => document.getElementById('policy-start').disabled = false);
        assert.equal(await frame.locator('#policy-start').isDisabled(), true, 'component update cannot override read-only');
    } finally { setReadonly(false); await frame.evaluate(() => document.getElementById('policy-fixture').remove()); }
}
