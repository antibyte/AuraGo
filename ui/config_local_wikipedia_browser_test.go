package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
)

// localWikipediaFakeAdminAPI replaces fetch for /api/local-wikipedia/* with an
// in-page fake of the admin API. window.lwStatus is the status it serves (reset
// it from the test and call localWikiRefreshStatus()), window.lwCalls records
// every request. The install answers 422 insufficient_disk_space with
// can_delete_old until it is asked for delete_old_first, like the real server
// does for an update that does not fit next to the installed edition.
const localWikipediaFakeAdminAPI = `() => {
    const originalFetch = window.fetch;
    window.lwCalls = [];
    window.lwReady = {state:'ready', readable:true, loading:false, progress:0, bytes_done:0, bytes_total:0, rate:0, eta_seconds:0,
        edition:{language:'de', variant:'nopic', date:'2026-09', name:'wikipedia_de_all_nopic_2026-09', file_name:'wikipedia_de_all_nopic_2026-09.zim', size:18000000000, article_count:5100000},
        selection:{language:'de', variant:'nopic'}, selection_matches_installed:true,
        update_available:{date:'2026-10', size:18585337856}, fulltext:true, free_bytes:2000000000, required_bytes:0,
        data_dir:'/data/wikipedia', data_dir_locked:false, operation_in_progress:false, system_language:'de',
        languages:[{code:'de', name:'Deutsch', fulltext:true}, {code:'ja', name:'日本語', fulltext:false}]};
    window.lwStatus = Object.assign({}, window.lwReady);
    const json = (body, status) => Promise.resolve(new Response(JSON.stringify(body), {status: status || 200, headers: {'Content-Type': 'application/json'}}));
    window.fetch = (url, options) => {
        options = options || {};
        const path = String(url);
        if (!path.startsWith('/api/local-wikipedia/')) return originalFetch(url, options);
        lwCalls.push({path: path, method: options.method || 'GET', body: options.body ? JSON.parse(options.body) : null});
        if (path === '/api/local-wikipedia/status') return json(lwStatus);
        if (path.startsWith('/api/local-wikipedia/catalog')) return json({language:'de', fulltext:true, variants:{
            nopic:{name:'wikipedia_de_all_nopic_2026-10', date:'2026-10', size:18585337856, article_count:5153780, meta4_url:'https://download.kiwix.org/a.meta4'},
            maxi:{name:'wikipedia_de_all_maxi_2026-01', date:'2026-01', size:52392226816, article_count:5041970, meta4_url:'https://download.kiwix.org/b.meta4'}}});
        if (path === '/api/local-wikipedia/install') {
            const body = JSON.parse(options.body);
            if (body.replace_mode !== 'delete_old_first') {
                return json({error:'insufficient_disk_space', error_code:'insufficient_disk_space', required_bytes:19659079680, free_bytes:2000000000, can_delete_old:true}, 422);
            }
            lwStatus = Object.assign({}, lwStatus, {state:'downloading', operation_in_progress:true, readable:false, edition:null, update_available:null,
                bytes_done:9292668792, bytes_total:18585337585, progress:0.5, rate:25000000, eta_seconds:372});
            return json({status:'accepted'}, 202);
        }
        return json({error:'unexpected'}, 500);
    };
}`

// openLocalWikipediaConfigPage opens the config page of the fixture server with
// the fake admin API installed.
func openLocalWikipediaConfigPage(t *testing.T, locale string, populated bool) *rod.Page {
	t.Helper()
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, locale, populated) + "/config#overview").Timeout(60 * time.Second)
	page.MustWaitLoad()
	t.Cleanup(func() { page.MustClose() })
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(localWikipediaFakeAdminAPI)
	return page
}

// setLocalWikipediaStatus serves the ready edition with the JSON patch applied
// and polls the status once, as the section's own poll would.
func setLocalWikipediaStatus(page *rod.Page, patch string) {
	page.MustEval(`async patch => {
        lwStatus = Object.assign({}, lwReady, JSON.parse(patch));
        await localWikiRefreshStatus();
    }`, patch)
}

// TestConfigLocalWikipediaBrowser drives the section against a fake admin API:
// the save-first gate, live sizes, title-search-only marking, the update
// confirmation, the "delete old edition first" retry, the progress view and the
// single persistent live region. The fake marks ja as title-only only to
// exercise the suffix: with slice 2's analyzer every offered language (ja and zh
// included) reports fulltext: true.
func TestConfigLocalWikipediaBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := openLocalWikipediaConfigPage(t, "de", false)
	page.MustEval(`async () => {
        await selectSection('local_wikipedia');
        resetDirtySnapshot();
    }`)
	// The status arrives asynchronously; the edition is installed, so the update
	// button is offered, but nothing may start while the integration is off.
	waitForJSBool(t, page, `() => !!document.querySelector('[data-lw-action="update"]')`)
	if !page.MustEval(`() => document.querySelector('[data-lw-action="update"]').disabled &&
        document.getElementById('lw-runtime').textContent.includes(t('config.local_wikipedia.enable_first'))`).Bool() {
		t.Fatal("update is offered although Local Wikipedia is disabled")
	}
	if !page.MustEval(`() => {
        const live = document.querySelectorAll('#content [aria-live], #content [role="status"], #content [role="alert"]');
        const region = document.getElementById('lw-announce');
        window.lwAnnounceElement = region;
        return live.length === 1 && live[0] === region && region.getAttribute('role') === 'status' && region.getAttribute('aria-live') === 'polite' &&
            region.textContent === t('config.local_wikipedia.state_ready');
    }`).Bool() {
		t.Fatal("the section must have exactly one live region announcing the state")
	}

	// Enabling is a pending change: until it is saved, nothing starts.
	page.MustEval(`() => document.querySelector('[data-path="local_wikipedia.enabled"]').click()`)
	waitForJSBool(t, page, `() => document.getElementById('lw-runtime').textContent.includes(t('config.local_wikipedia.save_first')) && document.querySelector('[data-lw-action="update"]').disabled`)
	if !page.MustEval(`async () => await saveConfig()`).Bool() {
		t.Fatal("enabling Local Wikipedia could not be saved")
	}
	waitForJSBool(t, page, `() => { const update = document.querySelector('[data-lw-action="update"]'); const nopic = document.querySelector('[data-path="local_wikipedia.variant"] option[value="nopic"]');
        return !!update && !update.disabled && !!nopic && nopic.textContent.includes('18,6'); }`)
	if !page.MustEval(`() => document.querySelector('[data-path="local_wikipedia.language"] option[value="ja"]').textContent.includes(t('config.local_wikipedia.title_only')) &&
        document.querySelector('[data-path="local_wikipedia.language"] option[value=""]').textContent.includes('Deutsch')`).Bool() {
		t.Fatal("language dropdown does not show the system language and the title-search-only marker")
	}

	page.MustEval(`() => document.querySelector('[data-lw-action="update"]').click()`)
	waitForJSBool(t, page, `() => { const overlay = document.getElementById('shared-modal-overlay'); return !!overlay && overlay.classList.contains('active') && document.getElementById('shared-modal-title').textContent === t('config.local_wikipedia.confirm_update_title'); }`)
	page.MustEval(`() => document.getElementById('shared-modal-confirm').click()`)
	waitForJSBool(t, page, `() => lwCalls.filter(call => call.path === '/api/local-wikipedia/install').length === 1 && document.getElementById('shared-modal-title').textContent === t('config.local_wikipedia.confirm_delete_old_title') && document.getElementById('shared-modal-overlay').classList.contains('active')`)
	page.MustEval(`() => document.getElementById('shared-modal-confirm').click()`)
	waitForJSBool(t, page, `() => { const installs = lwCalls.filter(call => call.path === '/api/local-wikipedia/install'); return installs.length === 2 && installs[0].body.replace_mode === 'keep_old' && installs[1].body.replace_mode === 'delete_old_first'; }`)
	waitForJSBool(t, page, `() => { const bar = document.getElementById('lw-progress'); return !!bar && Number(bar.value) === 500 && !!document.querySelector('[data-lw-action="cancel"]') && !document.querySelector('[data-lw-action="update"]'); }`)

	// The live region survived every re-render and announces the state, not the progress.
	if !page.MustEval(`() => document.getElementById('lw-announce') === lwAnnounceElement &&
        lwAnnounceElement.textContent === t('config.local_wikipedia.state_downloading')`).Bool() {
		t.Fatalf("live region replaced or wrong: %s", page.MustEval(`() => document.getElementById('lw-announce').textContent`).String())
	}
	// Status polls after the install answer the 2 s timer; an unchanged status does not rebuild the area.
	page.MustEval(`() => { window.lwProgressElement = document.getElementById('lw-progress'); }`)
	page.MustEval(`async () => { await localWikiRefreshStatus(); }`)
	if !page.MustEval(`() => document.getElementById('lw-progress') === lwProgressElement`).Bool() {
		t.Fatal("an unchanged status rebuilt the status area")
	}

	for _, width := range []int{390, 768, 1440, 1920} {
		page.MustSetViewport(width, 900, 1, width == 390)
		page.MustEval(`() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`)
		if page.MustEval(`() => document.documentElement.scrollWidth > innerWidth + 1 || document.getElementById('content').scrollWidth > document.getElementById('content').clientWidth + 1`).Bool() {
			t.Fatalf("Local Wikipedia section overflows a %d px viewport", width)
		}
	}
}

// TestConfigLocalWikipediaStatusesBrowser renders the statuses the manager
// reports today: the first background load, an installed edition that cannot be
// read, a failed update next to a served edition, an interrupted download, the
// full-text hint and Docker's fixed storage directory. The wording always comes
// from the status fields and the section's own translations; the server's
// English recommendation is never shown.
func TestConfigLocalWikipediaStatusesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	// The populated fixture enables every section, Local Wikipedia included.
	page := openLocalWikipediaConfigPage(t, "de", true)
	page.MustEval(`async () => {
        await selectSection('local_wikipedia');
        resetDirtySnapshot();
    }`)
	waitForJSBool(t, page, `() => !!document.querySelector('[data-lw-action="update"]') && !document.querySelector('[data-lw-action="update"]').disabled`)

	runtimeHas := func(t *testing.T, label, expression string) {
		t.Helper()
		if !page.MustEval(`() => { const text = document.getElementById('lw-runtime').textContent; const has = value => text.includes(value); return ` + expression + `; }`).Bool() {
			t.Fatalf("%s: %s", label, page.MustEval(`() => document.getElementById('lw-runtime').textContent`).String())
		}
	}
	const sentinel = "SERVER ENGLISH RECOMMENDATION"

	t.Run("first load is neither empty nor busy", func(t *testing.T) {
		setLocalWikipediaStatus(page, `{"state":"not_installed","readable":false,"loading":true,"edition":null,"update_available":null,"error_code":"busy","recommendation":"`+sentinel+`","selection_matches_installed":false}`)
		runtimeHas(t, "loading view", `has(t('config.local_wikipedia.loading_edition')) && !has(t('config.local_wikipedia.state_not_installed')) && !has(t('config.local_wikipedia.error_busy')) && !has('`+sentinel+`')`)
		if page.MustEval(`() => !!document.querySelector('#lw-runtime [data-lw-action]')`).Bool() {
			t.Fatal("no action may be offered while the storage directory is being loaded")
		}
		if !page.MustEval(`() => document.getElementById('lw-announce').textContent === t('config.local_wikipedia.loading_edition')`).Bool() {
			t.Fatal("the live region does not announce the loading state")
		}
		// The page polls on its own until loading is over.
		page.MustEval(`() => { lwStatus = Object.assign({}, lwReady, {update_available: null}); }`)
		waitForJSBool(t, page, `() => document.getElementById('lw-runtime').textContent.includes(t('config.local_wikipedia.state_ready'))`)
	})

	t.Run("installed edition that cannot be opened", func(t *testing.T) {
		setLocalWikipediaStatus(page, `{"state":"error","readable":false,"fulltext":false,"update_available":null,"error_code":"zim_unreadable","recommendation":"`+sentinel+`"}`)
		runtimeHas(t, "unreadable edition", `has(t('config.local_wikipedia.error_zim_unreadable')) && has(t('config.local_wikipedia.unreadable')) && !has(t('config.local_wikipedia.update_failed')) && !has('`+sentinel+`')`)
		if !page.MustEval(`() => !!document.querySelector('[data-lw-action="delete"]') && !document.querySelector('[data-lw-action="delete"]').disabled &&
            !document.querySelector('[data-lw-action="check"]') && !document.querySelector('[data-lw-action="install"]') && !document.querySelector('[data-lw-action="update"]')`).Bool() {
			t.Fatal("an unreadable edition can only be deleted")
		}
	})

	t.Run("failed update keeps the served edition", func(t *testing.T) {
		setLocalWikipediaStatus(page, `{"state":"ready","error_code":"download_failed","recommendation":"`+sentinel+`"}`)
		runtimeHas(t, "failed update", `has(t('config.local_wikipedia.state_ready')) && has(t('config.local_wikipedia.update_failed')) && has(t('config.local_wikipedia.error_download_failed')) && !has('`+sentinel+`')`)
		if !page.MustEval(`() => !!document.querySelector('[data-lw-action="check"]') && !document.querySelector('[data-lw-action="check"]').disabled`).Bool() {
			t.Fatal("the served edition can still be checked for updates")
		}
	})

	t.Run("downloaded file that cannot be read", func(t *testing.T) {
		setLocalWikipediaStatus(page, `{"state":"ready","error_code":"zim_unreadable","update_available":null}`)
		runtimeHas(t, "unreadable download", `has(t('config.local_wikipedia.error_download_unreadable')) && has(t('config.local_wikipedia.update_failed')) && !has(t('config.local_wikipedia.error_zim_unreadable'))`)
	})

	t.Run("interrupted download offers resume", func(t *testing.T) {
		setLocalWikipediaStatus(page, `{"state":"interrupted","error_code":"insufficient_disk_space","required_bytes":20000000000,"free_bytes":5000000000,"update_available":null}`)
		runtimeHas(t, "interrupted", `has(t('config.local_wikipedia.state_interrupted')) && has(localWikiErrorText('insufficient_disk_space', {required_bytes: 20000000000, free_bytes: 5000000000}))`)
		if !page.MustEval(`() => !!document.querySelector('[data-lw-action="resume"]') && !document.querySelector('[data-lw-action="resume"]').disabled && !document.querySelector('[data-lw-action="cancel"]')`).Bool() {
			t.Fatal("an interrupted download offers resume")
		}
	})

	t.Run("full-text hint is information", func(t *testing.T) {
		setLocalWikipediaStatus(page, `{"state":"ready","fulltext":false,"error_code":"fulltext_unsupported","update_available":null}`)
		runtimeHas(t, "title search only", `has(t('config.local_wikipedia.error_fulltext_unsupported')) && !has(t('config.local_wikipedia.update_failed')) && has(t('config.local_wikipedia.fulltext_off'))`)
		if page.MustEval(`() => !!document.querySelector('#lw-runtime .cfg-note-banner-warning') && document.querySelector('#lw-runtime .cfg-note-banner-warning').textContent.includes(t('config.local_wikipedia.error_fulltext_unsupported'))`).Bool() {
			t.Fatal("the full-text hint is not a warning")
		}
	})

	t.Run("a poll keeps the focus on the same button", func(t *testing.T) {
		setLocalWikipediaStatus(page, `{"update_available":null}`)
		page.MustEval(`() => document.querySelector('[data-lw-action="check"]').focus()`)
		setLocalWikipediaStatus(page, `{"update_available":{"date":"2026-11","size":18700000000}}`)
		if !page.MustEval(`() => !!document.querySelector('[data-lw-action="update"]') && document.activeElement && document.activeElement.dataset.lwAction === 'check' && document.getElementById('lw-runtime').contains(document.activeElement)`).Bool() {
			t.Fatal("re-rendering the status area lost the focus")
		}
		// Focus outside the status area is left alone.
		page.MustEval(`() => document.querySelector('[data-path="local_wikipedia.data_dir"]').focus()`)
		setLocalWikipediaStatus(page, `{"update_available":null}`)
		if !page.MustEval(`() => document.activeElement && document.activeElement.dataset.path === 'local_wikipedia.data_dir'`).Bool() {
			t.Fatal("a status refresh moved the focus out of the settings")
		}
	})

	t.Run("docker fixes the storage directory", func(t *testing.T) {
		page.MustEval(`async () => {
            lwStatus = Object.assign({}, lwReady, {data_dir_locked: true, data_dir: '/app/data/wikipedia', update_available: null});
            await localWikiRefreshStatus();
            renderLocalWikipediaSection();
            await localWikiRefreshStatus();
        }`)
		if !page.MustEval(`() => { const input = document.getElementById('lw-data-dir-readonly'); return !!input && input.disabled && input.value === '/app/data/wikipedia' &&
            !document.querySelector('[data-path="local_wikipedia.data_dir"]') && document.getElementById('lw-data-dir-locked').textContent.includes('/app/data/wikipedia'); }`).Bool() {
			t.Fatal("Docker's storage directory is not shown read-only")
		}
	})
}
