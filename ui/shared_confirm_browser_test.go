package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
)

// sharedConfirmFixture serves the real shared CSS/JS plus two small pages: a
// Precision workspace page with a page modal, and a chat-like page that only
// carries chat's own #modal-overlay markup (taken from index.html).
func sharedConfirmFixture(t *testing.T) string {
	t.Helper()

	i18n := normalizeAssetText(mustReadUIFile(t, "lang/common/en.json"))
	core := strings.Split(normalizeAssetText(mustReadUIFile(t, "js/shared/shared-core.js")), "// Auto-initialize on DOM ready")[0]

	index := string(mustReadUIFile(t, "index.html"))
	chatModal := regexp.MustCompile(`(?s)<div id="modal-overlay".*?id="modal-confirm"[^>]*></button>\s*</div>\s*</div>\s*</div>`).FindString(index)
	if chatModal == "" {
		t.Fatal("chat #modal-overlay markup not found in index.html")
	}

	head := `<!doctype html><html lang="en" data-theme="dark"><head><meta charset="utf-8">
<link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-utilities.css">
<link rel="stylesheet" href="/shared-components.css">
<link rel="stylesheet" href="/css/precision-workspace.css"><link rel="stylesheet" href="/css/precision-pages.css">
<script>var I18N = ` + i18n + `;</script><script>` + core + `
window.addEventListener('DOMContentLoaded', () => initModals());
</script></head>`
	pwPage := head + `<body class="pw-page pw-operational-page" data-workspace-page="fixture">
<button id="opener" type="button">Open</button><button id="other" type="button">Other</button>
<div id="page-modal" class="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="page-modal-title">
<div class="modal"><div class="modal-header"><h2 id="page-modal-title">Page dialog</h2>
<button type="button" class="modal-close" onclick="closeModal('page-modal')">&times;</button></div>
<p>Body</p><div class="modal-actions"><button class="btn btn-secondary" type="button">Close</button></div></div></div>
</body></html>`
	chatPage := head + `<body>` + chatModal + `<button id="opener" type="button">Open</button></body></html>`

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture/pw", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(pwPage))
	})
	mux.HandleFunc("/fixture/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(chatPage))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

const confirmContrastHelper = `
window.__lum = (color) => {
  const c = document.createElement('canvas'); c.width = c.height = 1;
  const x = c.getContext('2d', { willReadFrequently: true }); x.clearRect(0,0,1,1); x.fillStyle = color; x.fillRect(0,0,1,1);
  const d = x.getImageData(0,0,1,1).data;
  const f = v => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); };
  return { l: 0.2126 * f(d[0]) + 0.7152 * f(d[1]) + 0.0722 * f(d[2]), a: d[3] };
};
window.__ratio = (a, b) => { const x = __lum(a).l, y = __lum(b).l; return (Math.max(x,y) + 0.05) / (Math.min(x,y) + 0.05); };
`

func TestSharedConfirmDialogBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("requires browser smoke flag")
	}
	origin := sharedConfirmFixture(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(origin + "/fixture/pw").Timeout(120 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustEval(`() => {` + confirmContrastHelper + `}`)

	open := func(expr string) {
		t.Helper()
		page.MustEval(`() => { window.res = null; window.p = (` + expr + `); window.p.then(v => { window.res = v; }); }`)
		waitForJSBool(t, page, `() => document.getElementById('shared-modal-overlay')?.style.display === 'flex'`)
		// initial focus lands on the next tick
		waitForJSBool(t, page, `() => document.getElementById('shared-modal-overlay').contains(document.activeElement)`)
		time.Sleep(60 * time.Millisecond)
	}
	closed := func() {
		t.Helper()
		waitForJSBool(t, page, `() => window.res !== null`)
		waitForJSBool(t, page, `() => document.getElementById('shared-modal-overlay').style.display === 'none' && !document.getElementById('shared-modal-overlay').classList.contains('active')`)
	}
	result := func() string {
		return page.MustEval(`() => JSON.stringify(typeof window.res === 'boolean' ? { confirmed: window.res } : window.res)`).String()
	}
	activeID := func() string {
		return page.MustEval(`() => document.activeElement && document.activeElement.id || ''`).String()
	}

	t.Run("normal tier focuses confirm and resolves true", func(t *testing.T) {
		open(`showConfirm('Title', 'Message')`)
		if activeID() != "shared-modal-confirm" {
			t.Fatalf("initial focus = %q, want confirm", activeID())
		}
		if page.MustEval(`() => document.getElementById('shared-modal-confirm').classList.contains('btn-danger')`).Bool() {
			t.Fatal("normal tier must not use the danger button")
		}
		page.MustEval(`() => { const o = document.getElementById('shared-modal-overlay'); if (o.getAttribute('role') !== 'dialog' || o.getAttribute('aria-modal') !== 'true' || o.getAttribute('aria-labelledby') !== 'shared-modal-title' || document.querySelector('#shared-modal-title').tagName !== 'H2') throw Error('dialog semantics'); }`)
		page.MustElement("#shared-modal-confirm").MustClick()
		closed()
		if got := result(); got != `{"confirmed":true}` {
			t.Fatalf("result = %s", got)
		}
	})

	t.Run("alert hides cancel and resolves", func(t *testing.T) {
		open(`showAlert('Hello')`)
		if page.MustEval(`() => getComputedStyle(document.getElementById('shared-modal-cancel')).display`).String() != "none" {
			t.Fatal("alert shows a cancel button")
		}
		page.MustElement("#shared-modal-confirm").MustClick()
		closed()
	})

	t.Run("danger tier starts on cancel", func(t *testing.T) {
		open(`showConfirm('Delete', 'Really?', { danger: true, confirmText: 'Delete', cancelText: 'Keep' })`)
		if activeID() != "shared-modal-cancel" {
			t.Fatalf("danger initial focus = %q, want cancel", activeID())
		}
		if !page.MustEval(`() => document.getElementById('shared-modal-confirm').classList.contains('btn-danger') && document.getElementById('shared-modal-confirm').textContent === 'Delete'`).Bool() {
			t.Fatal("danger tier lacks the danger button or custom text")
		}
		page.Keyboard.MustType(input.Escape)
		closed()
		if got := result(); got != `{"confirmed":false}` {
			t.Fatalf("Esc result = %s", got)
		}
	})

	t.Run("Enter on a focused Cancel does not confirm", func(t *testing.T) {
		open(`showConfirm('Title', 'Message')`)
		page.MustEval(`() => document.getElementById('shared-modal-cancel').focus()`)
		page.Keyboard.MustType(input.Enter)
		closed()
		if got := result(); got != `{"confirmed":false}` {
			t.Fatalf("Enter on Cancel result = %s, want false", got)
		}
	})

	t.Run("type to confirm", func(t *testing.T) {
		open(`showConfirmDialog({ title: 'Delete nest', message: 'Permanent', requireText: 'nest-1' })`)
		if activeID() != "shared-modal-input" {
			t.Fatalf("type-to-confirm initial focus = %q, want the input", activeID())
		}
		disabled := func() bool {
			return page.MustEval(`() => document.getElementById('shared-modal-confirm').disabled`).Bool()
		}
		if !disabled() {
			t.Fatal("confirm must start disabled")
		}
		page.Keyboard.MustType(input.KeyN, input.KeyE, input.KeyS, input.KeyT)
		if !disabled() {
			t.Fatal("partial text enabled confirm")
		}
		page.MustEval(`() => { const i = document.getElementById('shared-modal-input'); i.value = '  nest-1 '; i.dispatchEvent(new Event('input', { bubbles: true })); }`)
		if disabled() {
			t.Fatal("trimmed exact match did not enable confirm")
		}
		page.MustEval(`() => { const i = document.getElementById('shared-modal-input'); i.value = 'Nest-1'; i.dispatchEvent(new Event('input', { bubbles: true })); }`)
		if !disabled() {
			t.Fatal("case mismatch enabled confirm")
		}
		page.MustEval(`() => { const i = document.getElementById('shared-modal-input'); i.value = 'nest-1'; i.dispatchEvent(new Event('input', { bubbles: true })); }`)
		page.MustElement("#shared-modal-confirm").MustClick()
		closed()
		if got := result(); got != `{"confirmed":true,"checked":false}` {
			t.Fatalf("result = %s", got)
		}
	})

	t.Run("checkbox", func(t *testing.T) {
		open(`showConfirmDialog({ title: 'Delete skill', message: 'Sure?', danger: true, checkbox: { label: 'Delete files', checked: true, hint: 'Removes the folder' } })`)
		if !page.MustEval(`() => document.getElementById('shared-modal-check').checked`).Bool() {
			t.Fatal("checkbox should start checked")
		}
		page.MustElement("#shared-modal-check").MustClick()
		page.MustElement("#shared-modal-confirm").MustClick()
		closed()
		if got := result(); got != `{"confirmed":true,"checked":false}` {
			t.Fatalf("result = %s", got)
		}
		open(`showConfirmDialog({ title: 'Again', message: 'Sure?' })`)
		if page.MustEval(`() => !!document.getElementById('shared-modal-check') || !document.querySelector('[data-confirm-extra]').hidden`).Bool() {
			t.Fatal("checkbox leaked into the next dialog")
		}
		page.MustElement("#shared-modal-cancel").MustClick()
		closed()
	})

	t.Run("busy onConfirm keeps the dialog open until it succeeds", func(t *testing.T) {
		page.MustEval(`() => { window.attempts = 0; window.allow = false; }`)
		open(`showConfirmDialog({ title: 'Busy', message: 'Working', onConfirm: async () => { attempts++; await new Promise(r => setTimeout(r, 250)); return allow; } })`)
		page.MustElement("#shared-modal-confirm").MustClick()
		waitForJSBool(t, page, `() => document.getElementById('shared-modal-confirm').getAttribute('aria-busy') === 'true' && document.getElementById('shared-modal-confirm').disabled`)
		page.Keyboard.MustType(input.Escape) // ignored while busy
		if !page.MustEval(`() => document.getElementById('shared-modal-overlay').classList.contains('active') && window.res === null`).Bool() {
			t.Fatal("Esc closed a busy dialog")
		}
		waitForJSBool(t, page, `() => !document.getElementById('shared-modal-confirm').disabled && !document.getElementById('shared-modal-confirm').hasAttribute('aria-busy')`)
		if !page.MustEval(`() => window.res === null && document.getElementById('shared-modal-overlay').classList.contains('active') && attempts === 1`).Bool() {
			t.Fatal("false from onConfirm must keep the dialog open")
		}
		page.MustEval(`() => { window.allow = true; }`)
		page.MustElement("#shared-modal-confirm").MustClick()
		closed()
		if got := result(); got != `{"confirmed":true,"checked":false}` {
			t.Fatalf("result = %s", got)
		}
	})

	t.Run("onConfirm throwing keeps the dialog open", func(t *testing.T) {
		open(`showConfirmDialog({ title: 'Throws', message: 'x', onConfirm: async () => { throw new Error('nope'); } })`)
		page.MustElement("#shared-modal-confirm").MustClick()
		waitForJSBool(t, page, `() => !document.getElementById('shared-modal-confirm').disabled`)
		if !page.MustEval(`() => window.res === null && document.getElementById('shared-modal-overlay').classList.contains('active')`).Bool() {
			t.Fatal("a throwing onConfirm closed the dialog")
		}
		page.MustElement("#shared-modal-cancel").MustClick()
		closed()
	})

	t.Run("backdrop and X resolve false", func(t *testing.T) {
		open(`showConfirm('Title', 'Message')`)
		page.Mouse.MustMoveTo(4, 4)
		page.Mouse.MustClick("left")
		closed()
		if got := result(); got != `{"confirmed":false}` {
			t.Fatalf("backdrop result = %s", got)
		}
		open(`showConfirm('Title', 'Message')`)
		page.MustElement("#shared-modal-overlay .modal-close").MustClick()
		closed()
		if got := result(); got != `{"confirmed":false}` {
			t.Fatalf("X result = %s", got)
		}
	})

	t.Run("focus returns to the opener", func(t *testing.T) {
		page.MustEval(`() => { document.getElementById('opener').onclick = () => { window.res = null; showConfirm('T', 'M').then(v => { window.res = v; }); }; }`)
		page.MustElement("#opener").MustClick()
		waitForJSBool(t, page, `() => document.getElementById('shared-modal-overlay')?.style.display === 'flex' && document.getElementById('shared-modal-overlay').contains(document.activeElement)`)
		page.Keyboard.MustType(input.Escape)
		closed()
		waitForJSBool(t, page, `() => document.activeElement && document.activeElement.id === 'opener'`)
	})

	t.Run("confirm sits above a page modal and Esc closes only the confirm", func(t *testing.T) {
		page.MustEval(`() => { window.reasons = []; openModal('page-modal', { onClose: r => reasons.push(r) }); }`)
		open(`showConfirm('Title', 'Message')`)
		z := page.MustEval(`() => [Number(getComputedStyle(document.getElementById('shared-modal-overlay')).zIndex), Number(getComputedStyle(document.getElementById('page-modal')).zIndex)]`).Arr()
		if z[0].Int() <= z[1].Int() {
			t.Fatalf("confirm z-index %d must exceed page modal z-index %d", z[0].Int(), z[1].Int())
		}
		if !page.MustEval(`() => { const b = document.getElementById('shared-modal-confirm').getBoundingClientRect(); return document.getElementById('shared-modal-overlay').contains(document.elementFromPoint(b.left + b.width / 2, b.top + b.height / 2)); }`).Bool() {
			t.Fatal("page modal covers the confirm dialog")
		}
		page.Keyboard.MustType(input.Escape)
		closed()
		if !page.MustEval(`() => document.getElementById('page-modal').classList.contains('active') && reasons.length === 0`).Bool() {
			t.Fatal("Esc on the confirm closed the page modal as well")
		}
		page.Keyboard.MustType(input.Escape)
		waitForJSBool(t, page, `() => !document.getElementById('page-modal').classList.contains('active') && reasons.length === 1 && reasons[0] === 'esc'`)
		// API close reports 'api', backdrop reports 'backdrop', and modal:close fires.
		page.MustEval(`() => { window.closeEvents = 0; document.getElementById('page-modal').addEventListener('modal:close', () => closeEvents++); openModal('page-modal', { onClose: r => reasons.push(r) }); closeModal('page-modal'); }`)
		if !page.MustEval(`() => reasons[1] === 'api' && closeEvents === 1 && !document.getElementById('page-modal').classList.contains('active')`).Bool() {
			t.Fatal("closeModal did not report 'api' / fire modal:close")
		}
		// openModal(id) without options stays the old call shape.
		page.MustEval(`() => { openModal('page-modal'); if (!document.getElementById('page-modal').classList.contains('active')) throw Error('openModal(id)'); closeModal('page-modal'); }`)
	})

	t.Run("queue keeps dialogs in order", func(t *testing.T) {
		page.MustEval(`() => { window.results = []; showModal('one', 'first', true).then(v => results.push(['one', v])); showModal('two', 'second', true).then(v => results.push(['two', v])); }`)
		page.MustElement("#shared-modal-confirm").MustClick()
		page.MustWait(`() => window.results.length === 1 && document.getElementById('shared-modal-title').textContent === 'two'`)
		page.MustElement("#shared-modal-cancel").MustClick()
		page.MustWait(`() => window.results.length === 2`)
		if !page.MustEval(`() => results[0][1] === true && results[1][1] === false`).Bool() {
			t.Fatal("queued dialogs share a result")
		}
	})

	for _, theme := range []string{"dark", "light"} {
		theme := theme
		t.Run("contrast "+theme, func(t *testing.T) {
			page.MustEval(`theme => { document.documentElement.dataset.theme = theme; }`, theme)
			for _, tier := range []string{"{}", "{ danger: true }"} {
				open(fmt.Sprintf(`showConfirm('Title', 'Message text', %s)`, tier))
				got := page.MustEval(`() => {
  const bg = getComputedStyle(document.querySelector('#shared-modal-overlay .modal')).backgroundColor;
  const pair = (el) => __ratio(getComputedStyle(el).color, bg);
  const btn = document.getElementById('shared-modal-confirm');
  const btnBg = getComputedStyle(btn).backgroundColor;
  return { title: pair(document.getElementById('shared-modal-title')), message: pair(document.getElementById('shared-modal-message')),
    cancel: pair(document.getElementById('shared-modal-cancel')),
    confirm: __lum(btnBg).a > 0 ? __ratio(getComputedStyle(btn).color, btnBg) : 21 };
 }`)
				for key, min := range map[string]float64{"title": 4.5, "message": 4.5, "cancel": 4.5, "confirm": 3} {
					if v := got.Get(key).Num(); v < min {
						t.Errorf("%s %s contrast %.2f < %.1f", theme, key, v, min)
					}
				}
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					_ = os.MkdirAll(dir, 0o755)
					_ = os.WriteFile(fmt.Sprintf("%s/shared-confirm-%s-%d.png", dir, theme, len(tier)), page.MustScreenshot(), 0o644)
				}
				page.MustElement("#shared-modal-cancel").MustClick()
				closed()
			}
		})
	}
}

func TestSharedConfirmChatOverlayBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("requires browser smoke flag")
	}
	origin := sharedConfirmFixture(t)
	page := newSmokeBrowser(t).MustPage(origin + "/fixture/chat").Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()

	wait := func(expr string) {
		t.Helper()
		waitForJSBool(t, page, expr)
	}
	// The chat overlay is reused: no second overlay is created.
	page.MustEval(`() => { window.res = null; showConfirm('Chat title', 'Chat message').then(v => { window.res = v; }); }`)
	wait(`() => document.getElementById('modal-overlay').style.display === 'flex' && document.getElementById('modal-overlay').classList.contains('active')`)
	if !page.MustEval(`() => !document.getElementById('shared-modal-overlay') && document.getElementById('modal-title').textContent === 'Chat title' && document.getElementById('modal-message').textContent === 'Chat message' && document.getElementById('modal-confirm').textContent === 'Yes'`).Bool() {
		t.Fatal("chat overlay was not filled")
	}
	page.MustElement("#modal-confirm").MustClick()
	wait(`() => window.res === true && document.getElementById('modal-overlay').style.display === 'none'`)

	// Rich options work on the chat overlay and leave no residue.
	page.MustEval(`() => { window.res = null; showConfirmDialog({ title: 'Del', message: 'Gone', danger: true, requireText: 'go', checkbox: { label: 'Files' } }).then(v => { window.res = v; }); }`)
	wait(`() => document.getElementById('modal-overlay').classList.contains('active') && !!document.getElementById('shared-modal-input')`)
	if !page.MustEval(`() => document.getElementById('modal-confirm').disabled && document.getElementById('modal-confirm').classList.contains('btn-danger')`).Bool() {
		t.Fatal("type-to-confirm did not disable the chat confirm button")
	}
	page.MustElement("#shared-modal-input").MustInput("go")
	page.MustElement("#modal-confirm").MustClick()
	wait(`() => window.res && window.res.confirmed === true && window.res.checked === false`)
	if !page.MustEval(`() => !document.getElementById('shared-modal-input') && !document.querySelector('[data-confirm-injected]') && !document.getElementById('modal-confirm').classList.contains('btn-danger')`).Bool() {
		t.Fatal("chat overlay kept extra content after closing")
	}

	// Esc closes it with false and a plain alert hides Cancel.
	page.MustEval(`() => { window.res = null; showAlert('Heads up').then(v => { window.res = v; }); }`)
	wait(`() => document.getElementById('modal-overlay').classList.contains('active')`)
	if page.MustEval(`() => getComputedStyle(document.getElementById('modal-cancel')).display`).String() != "none" {
		t.Fatal("chat alert shows Cancel")
	}
	page.Keyboard.MustType(input.Escape)
	wait(`() => window.res !== null && !document.getElementById('modal-overlay').classList.contains('active')`)
}
