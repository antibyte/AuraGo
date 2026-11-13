package ui

import (
	"testing"
	"time"
)

func TestMailReceiptsConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(90 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
 const original = window.fetch.bind(window);
 const id = 'a'.repeat(64), rev = 'b'.repeat(64);
 window.receiptFixture = {reads:0,writes:[],confirmations:[],state:'unknown',revision:rev};
 window.showConfirm = async text => {receiptFixture.confirmations.push(text);return true;};
 window.fetch = async (input, options={}) => {
  const url = new URL(String(input), location.origin), f=receiptFixture;
  if (!url.pathname.startsWith('/api/mail-notifications/receipts')) return original(input,options);
  if (options.method==='POST') {
   f.writes.push(JSON.parse(options.body));
   if (f.conflict) return new Response('{}',{status:409});
   f.state='sent';f.revision='c'.repeat(64);
   throw new Error('synthetic lost acknowledgement');
  }
  f.reads++;
  const more=url.searchParams.has('cursor');
  const rows=more ? [{id:'d'.repeat(64),revision:rev,channel:'ntfy',status:'unknown',attempted_at:'2026-11-13T12:00:00Z'}] : [
   {id,revision:f.revision,channel:'telegram',status:f.state,attempted_at:'2026-11-13T12:00:00Z',expires_at:f.state==='sent'?'2026-12-13T12:00:00Z':undefined,subject:'MUST_NOT_RENDER',recipient:'MUST_NOT_RENDER'},
   {id:'e'.repeat(64),revision:rev,channel:'discord',status:'active',attempted_at:'2026-11-13T12:00:00Z'}];
  return new Response(JSON.stringify({receipts:rows,channels:[{channel:'telegram',count:2,limit:2000}],next_cursor:more?'':'fixture-cursor'}),{headers:{'Content-Type':'application/json'}});
 };
}`)
	page.MustEval(`async () => {await selectSection('notifications');}`)
	waitForJSBool(t, page, `() => document.querySelectorAll('#config-mail-receipts tbody tr').length===2`)
	if !page.MustEval(`() => {
 const root=document.getElementById('config-mail-receipts'), rows=root.querySelectorAll('tbody tr');
 return root.innerText.includes('Mail-Zustellbelege') && root.innerText.includes('2/2000') &&
 rows[0].innerText.includes('Zustellung unklar') && Array.from(rows[1].querySelectorAll('button')).every(b=>b.disabled) &&
 !root.innerText.includes('MUST_NOT_RENDER') && receiptFixture.writes.length===0;
}`).Bool() {
		t.Fatal("receipt metadata, active guards or privacy boundary failed")
	}
	page.MustElement("#config-mail-receipts tbody tr:first-child button:first-child").MustClick()
	waitForJSBool(t, page, `() => receiptFixture.reads>=2 && document.querySelector('#config-mail-receipts tbody tr').innerText.includes('Zugestellt')`)
	if !page.MustEval(`() => receiptFixture.writes.length===1 && receiptFixture.confirmations.length===1 &&
 Object.keys(receiptFixture.writes[0]).sort().join(',')==='resolution,revision' && receiptFixture.writes[0].resolution==='sent' &&
 document.getElementById('config-mail-receipts').innerText.includes('erneut gelesen')`).Bool() {
		t.Fatal("lost response was replayed or not reconciled")
	}
	page.MustEval(`() => {receiptFixture.conflict=true;}`)
	page.MustElement("#config-mail-receipts tbody tr:first-child button:last-child").MustClick()
	waitForJSBool(t, page, `() => receiptFixture.reads>=3 && !document.getElementById('config-mail-receipts').matches('[aria-busy=true]')`)
	if !page.MustEval(`() => receiptFixture.writes.length===2 && receiptFixture.writes[1].resolution==='not_sent' && receiptFixture.writes[1].revision==='c'.repeat(64)`).Bool() {
		t.Fatal("resolution did not bind the refreshed revision")
	}
	page.MustElement("#config-mail-receipts > button").MustClick()
	waitForJSBool(t, page, `() => document.querySelectorAll('#config-mail-receipts tbody tr').length===3`)
	if !page.MustEval(`() => document.querySelector('#config-mail-receipts > button').hidden && receiptFixture.writes.length===2`).Bool() {
		t.Fatal("paging triggered a mutation or retained a finished cursor")
	}
	page.MustEval(`async () => {await selectSection('overview');}`)
	if page.MustHas("#config-mail-receipts") {
		t.Fatal("receipt card survived section navigation")
	}
}
