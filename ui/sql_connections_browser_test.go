package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLManagedImportBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshOrigin(t) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => {
		const original = window.fetch;
		window.sqlRequests = [];
		window.fetch = async (url,opts={}) => {
			if (!String(url).startsWith('/api/sql-connections')) return original(url,opts);
			window.sqlRequests.push({url:String(url),method:opts.method || 'GET',body:opts.body instanceof File ? opts.body.size : opts.body});
			if (String(url).endsWith('/import')) return new Response('{"status":"imported"}',{headers:{'Content-Type':'application/json'}});
			return new Response(opts.method ? '{"id":"sql-fixture"}' : '[]',{headers:{'Content-Type':'application/json'}});
		};
		await selectSection('sql_connections',{scrollBehavior:'auto'});
		sqlConnShowModal();
		const tls=document.getElementById('sqlconn-field-ssl');
		window.sqlTLSState=tls.value+'|'+Array.from(tls.options,o=>o.value).join(',');
		document.getElementById('sqlconn-field-driver').value='sqlite';sqlConnDriverChanged();
		document.getElementById('sqlconn-field-name').value='Imported database';
		const transfer=new DataTransfer();transfer.items.add(new File(['SQLite fixture'],'backup.sqlite'));
		document.getElementById('sqlconn-import-file').files=transfer.files;
	}`)
	if got := page.MustEval(`() => window.sqlTLSState`).Str(); got != "require|disable,require,verify-ca,verify-full" {
		t.Fatalf("new connection TLS select = %q, want require pre-selected and no implicit default", got)
	}
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"dark", "light"} {
			page.MustSetViewport(width, 900, 1, false)
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme}`, theme)
			if !page.MustEval(`() => document.getElementById('sqlconn-field-database').readOnly && !document.getElementById('sqlconn-import-row').classList.contains('is-hidden')`).Bool() {
				t.Fatal("SQLite UI permits raw paths or hides import")
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("sql-import-%d-%s.png", width, theme)), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	page.MustEval(`async () => { await sqlConnSave(); }`)
	if !page.MustEval(`() => sqlRequests.filter(r=>r.method==='POST').length===2 && sqlRequests.some(r=>r.url==='/api/sql-connections/sql-fixture/import' && r.body===14)`).Bool() {
		t.Fatal("save did not create and import the selected backup")
	}
}
