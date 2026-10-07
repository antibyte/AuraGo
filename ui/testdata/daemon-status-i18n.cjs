const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

const ui = path.join(__dirname, '..');
const source = fs.readFileSync(path.join(ui, 'cfg/daemon_skills.js'), 'utf8');
const translate = fs.readFileSync(path.join(ui, 'js/shared/shared-core.js'), 'utf8')
    .match(/function t\(k, p\) \{[\s\S]*?\r?\n\}/)[0];

async function main() {
    for (const locale of fs.readdirSync(path.join(ui, 'lang/config/daemon_skills'))) {
        const dict = JSON.parse(fs.readFileSync(path.join(ui, 'lang/config/daemon_skills', locale), 'utf8'));
        const title = { textContent: '' }, grid = { innerHTML: '' }, content = { innerHTML: '' };
        const ctx = vm.createContext({
            I18N: dict, configData: {}, esc: String, attachChangeListeners() {},
            document: { getElementById: id => ({ content, 'daemon-status-title': title, 'daemon-status-grid': grid })[id] },
            fetch: () => new Promise(() => {}),
        });
        vm.runInContext(translate + '\n' + source, ctx);
        const heading = count => ctx.t('config.daemon_skills.status_title', { count });
        ctx.renderDaemonSkillsSection({ label: '', desc: '' });
        assert.ok(content.innerHTML.includes('id="daemon-status-title">' + heading('…')), locale);
        assert.doesNotMatch(content.innerHTML, /%d|\{\{count\}\}/, locale);

        for (const daemons of [null, [], [{ name: 'alpha', status: 'running' }, { name: 'beta', status: 'stopped' }]]) {
            ctx.fetch = async () => ({ ok: true, json: async () => ({ status: 'ok', daemons }) });
            await ctx.loadDaemonStatus();
            assert.equal(title.textContent, heading(daemons?.length ? 1 : 0), locale);
            assert.ok(grid.innerHTML.includes(daemons?.length ? 'alpha' : dict['config.daemon_skills.no_daemons']), locale);
        }

        for (const response of [
            { ok: false, data: { status: 'ok', daemons: [] } },
            { ok: true, data: { status: 'error' } },
            { ok: true, data: { status: 'ok' } },
            { ok: true, data: { status: 'ok', daemons: {} } },
            null,
        ]) {
            ctx.fetch = async () => {
                if (!response) throw new Error('offline');
                return { ok: response.ok, json: async () => response.data };
            };
            await ctx.loadDaemonStatus();
            assert.equal(title.textContent, heading('—'), locale);
            assert.ok(grid.innerHTML.includes(dict['config.daemon_skills.load_error']), locale);
            assert.ok(!grid.innerHTML.includes(dict['config.daemon_skills.no_daemons']), locale);
        }
    }
    console.log('Daemon status: loading, counts, empty results and failures pass in all 16 locales.');
}

main().catch(error => { console.error(error); process.exitCode = 1; });
