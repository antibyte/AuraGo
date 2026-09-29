package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Exercise the shipped auth functions with real trusted input and a controlled
// clock, so the ten-minute expiry does not require a ten-minute browser test.
func TestAuthSessionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	source := readDesktopAssetText(t, "js/shared/shared-core.js")
	start := strings.Index(source, "window.AuraAuth =")
	end := strings.Index(source, "function showToast(")
	if start < 0 || end <= start {
		t.Fatal("shared authentication runtime missing")
	}
	authSource := source[start:end]
	mux := http.NewServeMux()
	mux.HandleFunc("/auth-runtime.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, authSource)
	})
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<!doctype html><title>Logged out</title>")
	})
	mux.HandleFunc("/desktop", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><input id="activity" aria-label="Input"><script>
const realTimeout = window.setTimeout.bind(window);
const timers = new Map();
let now = 1000000, timerID = 0;
Date.now = () => now;
window.setTimeout = (fn, delay = 0) => { const id = ++timerID; timers.set(id, {fn, at: now + delay}); return id; };
window.clearTimeout = id => timers.delete(id);
window.authTest = {
    expiry: now + 60000, calls: [], hidden: false, delayRenewal: false, failStatus: true,
    async flush() { for (let i = 0; i < 3; i++) await new Promise(r => realTimeout(r, 0)); },
    async tick(ms) {
        now += ms;
        for (const [id, timer] of [...timers]) {
            if (timer.at <= now && timers.delete(id)) await timer.fn();
        }
        await this.flush();
    },
    renewOtherTab() { this.expiry = now + 600000; },
    remaining() { return this.expiry - now; }
};
Object.defineProperty(document, 'visibilityState', {get: () => authTest.hidden ? 'hidden' : 'visible'});
window.fetch = async (url, init = {}) => {
    const path = new URL(url, location.origin).pathname;
    authTest.calls.push(path);
    if (path === '/api/auth/status' && authTest.failStatus) {
        authTest.failStatus = false;
        throw Error('temporary connection failure');
    }
    if (path === '/api/auth/logout') {
        authTest.expiry = 0;
        sessionStorage.setItem('logoutOrder', JSON.stringify(authTest.calls));
        return new Response(JSON.stringify({ok: true, redirect: '/auth/login'}));
    }
    if (path === '/api/auth/activity') {
        if (init.method !== 'POST' || init.credentials !== 'same-origin' || init.cache !== 'no-store') throw Error('unsafe activity request');
        if (authTest.delayRenewal) await new Promise(resolve => { authTest.releaseRenewal = resolve; });
        if (now >= authTest.expiry) return new Response('{}', {status: 401});
        authTest.expiry = Math.max(authTest.expiry, now + 600000);
    }
    return new Response(JSON.stringify({enabled: true, authenticated: now < authTest.expiry,
        expires_in_seconds: Math.max(0, (authTest.expiry - now) / 1000)}));
};
</script><script src="/auth-runtime.js"></script><script>checkAuth().then(() => authTest.ready = true);</script>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(40 * time.Second)
	defer page.Close()
	page.MustNavigate(srv.URL + "/desktop").MustWaitLoad()
	page.MustWait(`() => window.authTest?.ready`)
	page.MustEval(`async () => {
        for (const type of ['pointerdown', 'pointermove', 'keydown', 'input', 'wheel'])
            window.dispatchEvent(new Event(type));
        await authTest.flush();
        if (authTest.calls.includes('/api/auth/activity')) throw Error('synthetic input renewed session');
        await authTest.tick(30000); // Recover from the first failed status request.
    }`)
	page.MustElement("#activity").MustClick().MustInput("trusted typing")
	page.MustWait(`() => authTest.remaining() === 600000`)
	page.MustEval(`async () => {
        await authTest.flush();
        if (authTest.calls.filter(p => p === '/api/auth/activity').length !== 1) throw Error('input was not throttled');
        await authTest.tick(31000);
        authTest.hidden = true;
    }`)
	page.MustElement("#activity").MustInput("hidden input")
	page.MustEval(`async () => {
        await authTest.flush();
        if (authTest.calls.filter(p => p === '/api/auth/activity').length !== 1) throw Error('hidden input renewed session');
        authTest.hidden = false;
        document.dispatchEvent(new Event('visibilitychange'));
        await authTest.flush();
        await fetch('/api/poll');
        await authTest.tick(100000);
        if (authTest.calls.filter(p => p === '/api/auth/activity').length !== 1) throw Error('background traffic renewed session');
        // Pass the original login deadline while the renewed cookie remains valid.
        if (location.pathname !== '/desktop') throw Error('active session was logged out');
    }`)
	page.MustElement("#activity").MustInput("still active")
	page.MustWait(`() => authTest.calls.filter(p => p === '/api/auth/activity').length === 2 && authTest.remaining() === 600000`)
	page.MustEval(`async () => {
        await authTest.flush();
        await authTest.tick(599000);
        authTest.renewOtherTab();
        await authTest.tick(2000);
        if (location.pathname !== '/desktop') throw Error('ignored renewal in another tab');
        if (authTest.calls.filter(p => p === '/api/auth/activity').length !== 2) throw Error('idle timer renewed session');
    }`)
	page.MustEval(`() => { authTest.tick(600000); }`)
	page.MustWait(`() => location.pathname === '/auth/login'`)

	// A pending renewal must finish before explicit logout clears the cookie.
	page.MustNavigate(srv.URL + "/desktop").MustWaitLoad()
	page.MustWait(`() => window.authTest?.ready`)
	page.MustEval(`async () => { await authTest.tick(30000); authTest.delayRenewal = true; }`)
	page.MustElement("#activity").MustClick()
	page.MustWait(`() => !!authTest.releaseRenewal`)
	page.MustEval(`() => {
        performLogout();
        if (authTest.calls.includes('/api/auth/logout')) throw Error('logout raced pending renewal');
    }`)
	page.MustElement("#activity").MustInput("logout in progress")
	page.MustEval(`() => {
        if (authTest.calls.filter(p => p === '/api/auth/activity').length !== 1) throw Error('renewed during logout');
        authTest.releaseRenewal();
    }`)
	page.MustWait(`() => location.pathname === '/auth/login'`)
	page.MustEval(`() => {
        const calls = JSON.parse(sessionStorage.getItem('logoutOrder'));
        if (calls.at(-1) !== '/api/auth/logout' || calls.filter(p => p === '/api/auth/activity').length !== 1)
            throw Error('logout did not follow the single renewal');
    }`)
}
