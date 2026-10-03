package ui

import (
	"testing"

	"github.com/go-rod/rod"
)

// Reuse the encrypted records and HTTP fixture from TestDesktopTresorBrowser.
func checkTresorFailureRecovery(t *testing.T, page *rod.Page) {
	t.Helper()
	result := page.MustEval(`async () => {
        const until = async (predicate, label) => {
            for (let i = 0; i < 150; i++) {
                if (predicate()) return;
                await new Promise(resolve => setTimeout(resolve, 50));
            }
            throw Error(label + ': ' + document.querySelector('[data-status]')?.textContent);
        };
        const assert = (ok, message) => { if (!ok) throw Error(message); };
        const idle = () => until(() => document.querySelector('.tresor-app')?.getAttribute('aria-busy') === 'false', 'idle');
        const click = action => document.querySelector('[data-action=' + action + ']').click();
        const input = (selector, value) => {
            const field = document.querySelector(selector);
            field.value = value;
            field.dispatchEvent(new Event('input', { bubbles: true }));
        };
        const submit = selector => document.querySelector(selector).dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        const select = async id => { document.querySelector('[data-id="' + id + '"]').click(); await idle(); };
        const save = async () => { click('save'); await idle(); };
        const nativeFetch = window.fetch;
        const password = 'a strong vault password 123';
        const c = await import('/js/desktop/apps/tresor-crypto.js');
        const { header } = await (await nativeFetch('/api/desktop/tresor')).json();
        const master = await c.unlockWithPassword(header, password);
        const read = async id => {
            const row = await (await nativeFetch('/api/desktop/tresor/items/' + id)).json();
            const body = await c.open(master, c.fromBase64(row.body), c.recordContext(id, 'body'));
            try { return new TextDecoder().decode(body); } finally { body.fill(0); }
        };
        const a = [...document.querySelectorAll('.tresor-item')].find(x => x.textContent.includes('Secret Plan')).dataset.id;
        const unlock = async () => {
            input('[name=password]', password); submit('[data-form=unlock]');
            await until(() => document.querySelector('.tresor-workspace'), 'unlock'); await idle();
        };
        const reset = async () => {
            input('[data-title]', 'Secret Plan'); input('[data-note]', 'Private content'); await save();
        };
        try {
            await select(a);
            // Opening and closing a dialog must preserve both draft fields.
            input('[data-title]', 'Unsaved title'); input('[data-note]', 'Unsaved text');
            click('password'); await idle();
            assert(document.querySelector('[data-title]').value === 'Unsaved title', 'password dialog lost title');
            click('cancel'); await idle();
            assert(document.querySelector('[data-title]').value === 'Unsaved title', 'cancel lost title');
            assert(document.querySelector('[data-note]').value === 'Unsaved text', 'cancel lost note');
            await reset();

            click('new-note'); await idle();
            const b = document.querySelector('.tresor-item.is-selected').dataset.id;
            input('[data-title]', 'Other note'); input('[data-note]', 'Other content'); await save();
            await select(a);
            for (const failure of ['http', 'ciphertext']) {
                window.fetch = async (url, options) => {
                    if (String(url).endsWith('/items/' + b) && (!options?.method || options.method === 'GET')) {
                        if (failure === 'http') return new Response('{}', { status: 503 });
                        const row = await (await nativeFetch(url, options)).json();
                        const bytes = c.fromBase64(row.body); bytes[bytes.length - 1] ^= 1;
                        return new Response(JSON.stringify({ ...row, body: c.toBase64(bytes) }), { status: 200 });
                    }
                    return nativeFetch(url, options);
                };
                await select(b);
                assert(document.querySelector('.tresor-item.is-selected').dataset.id === a, failure + ' changed selection');
                assert(document.querySelector('[data-title]').value === 'Secret Plan', failure + ' changed editor');
                window.fetch = nativeFetch;
                input('[data-note]', 'Updated original'); await save();
                assert(await read(a) === 'Updated original', failure + ' did not save the original');
                assert(await read(b) === 'Other content', failure + ' overwrote the other record');
                await reset();
            }
            await select(b); click('delete'); await idle(); click('confirm-delete'); await idle();
            await select(a);

            // Persisted ciphertext drafts survive unconditional security locking.
            for (const trigger of ['idle', 'manual', 'pending']) {
                let release;
                let putStarted = false;
                window.fetch = async (url, options) => {
                    if (String(url).includes('/tresor/items/') && options?.method === 'PUT') {
                        putStarted = true;
                        if (trigger === 'pending') await new Promise(resolve => { release = resolve; });
                        return new Response('{}', { status: 503 });
                    }
                    return nativeFetch(url, options);
                };
                input('[data-title]', 'Recover ' + trigger); input('[data-note]', 'Draft ' + trigger);
                const draftKey = 'aurago:tresor:draft:' + a;
                await until(() => !!localStorage.getItem(draftKey), 'encrypted draft');
                const encrypted = localStorage.getItem(draftKey);
                assert(!encrypted.includes('Draft ') && !encrypted.includes('Recover '), 'plaintext draft persisted');
                if (trigger === 'pending') click('save');
                await until(() => putStarted, 'save attempt');
                if (trigger === 'idle') window.expireTresor(); else click('lock');
                await until(() => document.querySelector('[data-form=unlock]'), trigger + ' lock');
                assert(!document.querySelector('[data-note]'), trigger + ' left plaintext editor');
                assert(!document.querySelector('[data-form=unlock] button').disabled, trigger + ' left unlock busy');
                window.fetch = nativeFetch;
                // Unlock and edit while the old session's write is still pending.
                await unlock(); await select(a);
                assert(document.querySelector('[data-title]').value === 'Recover ' + trigger, 'draft title not recovered');
                assert(document.querySelector('[data-note]').value === 'Draft ' + trigger, 'draft text not recovered');
                if (release) {
                    release();
                    await new Promise(resolve => setTimeout(resolve, 50));
                    assert(document.querySelector('.tresor-workspace'), 'late write changed unlocked view');
                }
                await save();
                assert(await read(a) === 'Draft ' + trigger, 'recovered draft failed to save');
                await reset();
            }
            // A failed list request from an expired unlock cannot clear a new key.
            click('lock');
            let releaseRead;
            window.fetch = async (url, options) => {
                if (String(url).endsWith('/tresor/items')) {
                    await new Promise(resolve => { releaseRead = resolve; });
                    return new Response('{}', { status: 503 });
                }
                return nativeFetch(url, options);
            };
            input('[name=password]', password); submit('[data-form=unlock]');
            await until(() => !!releaseRead, 'pending unlock');
            window.expireTresor();
            window.fetch = nativeFetch;
            await unlock(); await select(a);
            releaseRead();
            await new Promise(resolve => setTimeout(resolve, 50));
            assert(document.querySelector('[data-note]')?.value === 'Private content', 'stale unlock cleared new session');
            // Page lifecycle locking also clears the DOM kept by a cached page.
            window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true }));
            assert(document.querySelector('[data-form=unlock]') && !document.querySelector('[data-note]'), 'pagehide retained plaintext');
            await unlock(); await select(a);
            return window.errors;
        } finally {
            window.fetch = nativeFetch;
            master.fill(0);
        }
    }`)
	if len(result.Arr()) != 0 {
		t.Fatalf("Tresor failure recovery browser errors: %s", result.JSON("", ""))
	}
}
