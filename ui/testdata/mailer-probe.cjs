const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.resolve(__dirname, '..');
const de = JSON.parse(fs.readFileSync(path.join(root, 'lang/mailer/de.json'), 'utf8'));
const statuses = [];
const button = direction => ({
    dataset: { testAccount: 'email:fixture', direction }, disabled: false,
    setAttribute(name, value) { this[name] = value; }
});
const buttons = [button('receive'), button('send')];
const inst = {
    ctx: {}, state: { readonly: false, accounts: [{ id: 'email:fixture', capabilities: ['read', 'send'] }] },
    host: { querySelectorAll: () => buttons }, api: {}
};
const context = { window: {} };
vm.createContext(context);
vm.runInContext(fs.readFileSync(path.join(root, 'js/desktop/apps/mailer-settings.js'), 'utf8'), context);
const settings = context.window.MailerSettings.create(inst, {
    t: (_, key) => de['mailer.' + key],
    setStatus: (_, message) => statuses.push(message),
    showError: () => statuses.push('generic error')
});

(async () => {
    let finish, calls = 0;
    inst.api.testAccount = () => { calls++; return new Promise(resolve => { finish = resolve; }); };
    const pending = settings.testAccount('email:fixture', 'receive');
    assert(buttons.every(item => item.disabled && item['aria-busy'] === 'true'));
    await settings.testAccount('email:fixture', 'send');
    assert.equal(calls, 1, 'pending probe must refuse duplicate work');
    finish({ receive: 'ok' });
    await pending;
    assert(buttons.every(item => !item.disabled && item['aria-busy'] === 'false'));

    inst.api.testAccount = async () => {
        const error = new Error('mail_test_busy');
        error.status = 429;
        error.body = { code: 'mail_test_busy' };
        throw error;
    };
    await settings.testAccount('email:fixture', 'send');
    assert.equal(statuses.at(-1), de['mailer.test_busy']);
    assert.match(statuses.at(-1), /warte/i);
    assert(buttons.every(item => !item.disabled));

    inst.api.testAccount = () => new Promise(resolve => { finish = resolve; });
    const readonlyPending = settings.testAccount('email:fixture', 'receive');
    inst.state.readonly = true;
    finish({ receive: 'ok' });
    await readonlyPending;
    assert(buttons.every(item => item.disabled), 'completion must preserve a new readonly policy');
    inst.state.readonly = false;
    const beforeDispose = statuses.length;
    const disposedPending = settings.testAccount('email:fixture', 'receive');
    inst.disposed = true;
    finish({ receive: 'ok' });
    await disposedPending;
    assert.equal(statuses.length, beforeDispose + 1, 'disposed window must ignore the result');
    console.log('Mailer probe feedback, pending controls, readonly and disposal passed.');
})().catch(error => { console.error(error); process.exitCode = 1; });
