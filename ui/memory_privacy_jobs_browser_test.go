package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

var memoryPrivacyLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

var memoryPrivacyTranslationKeys = []string{
	"dashboard.memory_privacy_title",
	"dashboard.memory_privacy_description",
	"dashboard.memory_privacy_refresh",
	"dashboard.memory_privacy_loading",
	"dashboard.memory_privacy_empty",
	"dashboard.memory_privacy_load_error",
	"dashboard.memory_privacy_job",
	"dashboard.memory_privacy_in_progress",
	"dashboard.memory_privacy_retry_wait",
	"dashboard.memory_privacy_reconcile_pending",
	"dashboard.memory_privacy_reconcile_retry",
	"dashboard.memory_privacy_reconcile_queued",
	"dashboard.memory_privacy_retained_review",
	"dashboard.memory_privacy_completed",
	"dashboard.memory_privacy_aborted",
	"dashboard.memory_privacy_created",
	"dashboard.memory_privacy_updated",
	"dashboard.memory_privacy_counts",
	"dashboard.memory_privacy_attempts_next_retry",
	"dashboard.memory_privacy_error_code",
	"dashboard.memory_privacy_details",
	"dashboard.memory_privacy_hide_details",
	"dashboard.memory_privacy_details_error",
	"dashboard.memory_privacy_retry",
	"dashboard.memory_privacy_retrying",
	"dashboard.memory_privacy_retry_queued",
	"dashboard.memory_privacy_retry_error",
	"dashboard.memory_privacy_not_retryable",
	"dashboard.memory_privacy_unknown",
	"dashboard.memory_privacy_no_retry_time",
}

func TestMemoryPrivacyJobsTranslations(t *testing.T) {
	for _, locale := range memoryPrivacyLocales {
		t.Run(locale, func(t *testing.T) {
			data, err := fs.ReadFile(Content, "lang/dashboard/"+locale+".json")
			if err != nil {
				t.Fatal(err)
			}
			var messages map[string]string
			if err := json.Unmarshal(data, &messages); err != nil {
				t.Fatal(err)
			}
			for _, key := range memoryPrivacyTranslationKeys {
				if strings.TrimSpace(messages[key]) == "" {
					t.Errorf("%s is missing from %s dashboard locale", key, locale)
				}
			}
		})
	}
}

func TestMemoryPrivacyJobsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("headless browser smoke test requires Chrome or Edge")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head>
<link rel="stylesheet" href="/css/dashboard.css"><link rel="stylesheet" href="/css/dashboard-shell.css">
</head><body><main id="sec-memory" data-section="memory">
<div id="memory-privacy-deletions">
<div id="memory-privacy-title"></div><button id="memory-privacy-refresh" onclick="loadMemoryPrivacyDeletionJobs()"></button>
<div id="memory-privacy-status" role="status" aria-live="polite"></div><div id="memory-privacy-job-list"></div>
</div></main></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launchCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Leakless' PID handshake can wait forever on Windows when the browser
	// cannot start. This smoke test owns the process and cleans it up directly.
	launch := launcher.New().Context(launchCtx).Bin(bin).Headless(true).NoSandbox(true).Leakless(false).Set("disable-gpu").Set("disable-dev-shm-usage")
	controlURL, err := launch.Launch()
	if err != nil {
		t.Skipf("headless browser smoke test skipped; browser launch failed: %v", err)
	}
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		t.Fatalf("connect headless browser for privacy jobs: %v", err)
	}
	t.Cleanup(func() {
		_ = browser.Close()
		launch.Kill()
		launch.Cleanup()
	})

	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			page := browser.MustPage(srv.URL + "/fixture").Timeout(30 * time.Second)
			defer page.Close()
			page.MustWaitLoad()
			result := page.MustEval(`async(theme)=>{
 document.documentElement.dataset.theme=theme;
 const source=await(await fetch('/js/dashboard/sections/memory.js')).text();
 const until=async fn=>{for(let i=0;i<300;i++){if(fn())return;await new Promise(r=>setTimeout(r,10))}throw Error('condition timed out')};
 const retryJob={deletion_job_id:'opaque-job-1',kind:'chat',deletion_status:'retry_wait',transcript_deleted:true,created_at:'2026-10-18T10:00:00Z',updated_at:'2026-10-18T10:01:00Z',next_retry_at:'2026-10-18T10:05:00Z',attempt_count:2,pending_count:3,deleted_count:4,retained_count:1,error_code:'<img data-privacy-xss src=x onerror=alert(1)>'};
 const reviewJob={deletion_job_id:'opaque-job-2',kind:'chat',deletion_status:'retained_review',transcript_deleted:true,created_at:'2026-10-18T09:00:00Z',updated_at:'2026-10-18T09:01:00Z',next_retry_at:'',attempt_count:1,pending_count:0,deleted_count:2,retained_count:1,error_code:'target_requires_review'};
 const reconcileJob={deletion_job_id:'opaque-job-3',kind:'contact',deletion_status:'reconcile_pending',transcript_deleted:false,created_at:'2026-10-18T08:00:00Z',updated_at:'2026-10-18T08:01:00Z',next_retry_at:'',attempt_count:1,pending_count:2,deleted_count:0,retained_count:0,error_code:'contact_delete_outcome_unknown'};
 const conflictJob={deletion_job_id:'opaque-job-4',kind:'chat',deletion_status:'retry_wait',transcript_deleted:true,created_at:'2026-10-18T07:00:00Z',updated_at:'2026-10-18T07:01:00Z',next_retry_at:'2026-10-18T07:05:00Z',attempt_count:1,pending_count:1,deleted_count:0,retained_count:0,error_code:'already_processing'};
 let failList=false;
 const calls=[];
 const labels={
  'dashboard.memory_privacy_loading':'Loading deletion jobs…',
  'dashboard.memory_privacy_empty':'No privacy deletion jobs.',
  'dashboard.memory_privacy_load_error':'Could not load deletion jobs.',
  'dashboard.memory_privacy_job':'Deletion job',
  'dashboard.memory_privacy_in_progress':'In progress',
  'dashboard.memory_privacy_retry_wait':'Waiting to retry',
  'dashboard.memory_privacy_reconcile_pending':'Contact status needs checking',
  'dashboard.memory_privacy_reconcile_retry':'Check contact status',
  'dashboard.memory_privacy_reconcile_queued':'Contact status check queued. Review the job state after it updates.',
  'dashboard.memory_privacy_retained_review':'Retained for review',
  'dashboard.memory_privacy_completed':'Completed',
  'dashboard.memory_privacy_aborted':'Aborted',
  'dashboard.memory_privacy_created':'Created: {{date}}',
  'dashboard.memory_privacy_updated':'Updated: {{date}}',
  'dashboard.memory_privacy_counts':'{{pending}} pending · {{deleted}} deleted · {{retained}} retained',
  'dashboard.memory_privacy_attempts_next_retry':'Attempts: {{count}} · Next retry: {{date}}',
  'dashboard.memory_privacy_error_code':'Last error: {{code}}',
  'dashboard.memory_privacy_details':'Details',
  'dashboard.memory_privacy_hide_details':'Hide details',
  'dashboard.memory_privacy_details_error':'Could not load job details.',
  'dashboard.memory_privacy_retry':'Retry deletion',
  'dashboard.memory_privacy_retrying':'Scheduling retry…',
  'dashboard.memory_privacy_retry_queued':'Retry queued.',
  'dashboard.memory_privacy_retry_error':'Could not retry deletion.',
  'dashboard.memory_privacy_not_retryable':'This job is no longer retryable.',
  'dashboard.memory_privacy_unknown':'Unknown',
  'dashboard.memory_privacy_no_retry_time':'Not scheduled'
 };
 window.t=(key,args={})=>(labels[key]||key).replace(/\{\{(\w+)\}\}/g,(_,name)=>String(args[name]??''));
 window.DashSections={register:()=>{}};
 (0,eval)(source);
 window.fetch=async(url,options={})=>{
   url=String(url);calls.push({url,method:options.method||'GET',credentials:options.credentials});
   if(url==='/api/privacy-deletions')return failList
     ? {ok:false,status:500,json:async()=>({status:'error',error:'temporarily_unavailable'})}
     : {ok:true,status:200,json:async()=>({status:'ok',jobs:[retryJob,reviewJob,reconcileJob,conflictJob]})};
   if(url==='/api/privacy-deletions/opaque-job-1')return {ok:true,status:200,json:async()=>({status:'ok',job:{...retryJob,updated_at:'2026-10-18T10:02:00Z'}})};
   if(url==='/api/privacy-deletions/opaque-job-1/retry'&&options.method==='POST'){
     retryJob.deletion_status='pending';retryJob.next_retry_at='';
     return {ok:true,status:200,json:async()=>({status:'ok',...retryJob})};
   }
   if(url==='/api/privacy-deletions/opaque-job-3/retry'&&options.method==='POST'){
     reconcileJob.deletion_status='pending';
     return {ok:true,status:202,json:async()=>({status:'ok',...reconcileJob})};
   }
   if(url==='/api/privacy-deletions/opaque-job-4/retry'&&options.method==='POST'){
     failList=true;
     return {ok:false,status:409,json:async()=>({status:'error',error:'deletion_job_not_retryable'})};
   }
   throw Error('unexpected request '+url);
 };
 await window.loadMemoryPrivacyDeletionJobs();
 await until(()=>document.querySelectorAll('#memory-privacy-job-list article').length===4);
 const body=document.querySelector('#memory-privacy-job-list').textContent;
 if(body.includes('opaque-job-1')||body.includes('opaque-job-2')||body.includes('opaque-job-3')||body.includes('opaque-job-4'))throw Error('job IDs must not be displayed');
 if(body.includes('Private chat')||body.includes('message body'))throw Error('chat targets or content must not be displayed');
 if(document.querySelector('#memory-privacy-job-list img[data-privacy-xss]'))throw Error('error code was interpreted as HTML');
 if(!body.includes('<img data-privacy-xss src=x onerror=alert(1)>'))throw Error('error code should remain visible as text');
 const rows=[...document.querySelectorAll('#memory-privacy-job-list article')];
 const retryRow=rows.find(row=>row.textContent.includes('Waiting to retry'));
 const reviewRow=rows.find(row=>row.textContent.includes('Retained for review'));
 const reconcileRow=rows.find(row=>row.textContent.includes('Contact status needs checking'));
 let conflictRow=rows.find(row=>row.textContent.includes('already_processing'));
 if(!retryRow||!reviewRow||!reconcileRow||!conflictRow)throw Error('job statuses were not rendered');
 if(retryRow.querySelectorAll('button').length!==2)throw Error('retry_wait job should expose details and retry');
 if(reviewRow.querySelectorAll('button').length!==1)throw Error('non-retryable job must not expose retry');
 if(reconcileRow.querySelectorAll('button').length!==2||!reconcileRow.textContent.includes('Check contact status'))throw Error('reconcile_pending job should expose a contact status check');
 const detailButton=retryRow.querySelector('button');detailButton.click();
 await until(()=>calls.some(call=>call.url==='/api/privacy-deletions/opaque-job-1'));
 await until(()=>retryRow.querySelector('.memory-curator-preview')?.textContent.includes('2026'));
 if(calls.find(call=>call.url==='/api/privacy-deletions/opaque-job-1').credentials!=='same-origin')throw Error('details request must use same-origin credentials');
 const retryButton=retryRow.querySelectorAll('button')[1];retryButton.click();
 await until(()=>calls.some(call=>call.url==='/api/privacy-deletions/opaque-job-1/retry'));
	await until(()=>document.querySelector('#memory-privacy-job-list article[data-memory-privacy-status="pending"]')!==null);
 const retryCall=calls.find(call=>call.url==='/api/privacy-deletions/opaque-job-1/retry');
 if(retryCall.method!=='POST'||retryCall.credentials!=='same-origin')throw Error('retry must be authenticated same-origin POST');
	await until(()=>document.querySelector('#memory-privacy-status').textContent==='Retry queued.');
 document.querySelector('#memory-privacy-job-list article[data-memory-privacy-status="reconcile_pending"] [data-memory-privacy-retry]').click();
 await until(()=>calls.some(call=>call.url==='/api/privacy-deletions/opaque-job-3/retry'));
 await until(()=>document.querySelectorAll('#memory-privacy-job-list article[data-memory-privacy-status="pending"]').length===2);
 if(document.querySelector('#memory-privacy-status').textContent!=='Contact status check queued. Review the job state after it updates.')throw Error('contact recheck queue was not announced');
 conflictRow=[...document.querySelectorAll('#memory-privacy-job-list article[data-memory-privacy-status="retry_wait"]')].find(row=>row.textContent.includes('already_processing'));
 if(!conflictRow?.textContent.includes('already_processing'))throw Error('conflict job disappeared after refresh');
 const conflictRetry=conflictRow.querySelector('[data-memory-privacy-retry]');conflictRetry.click();
 await until(()=>calls.some(call=>call.url==='/api/privacy-deletions/opaque-job-4/retry'));
 await until(()=>document.querySelector('#memory-privacy-status').textContent==='This job is no longer retryable.');
 if(!conflictRetry.isConnected||conflictRetry.disabled)throw Error('failed refresh left the retry action disabled');
 document.querySelector('#memory-privacy-refresh').click();
 await until(()=>calls.filter(call=>call.url==='/api/privacy-deletions').length>=5);
 return true;
}`, theme)
			if !result.Bool() {
				t.Fatalf("browser result = %s", result.String())
			}
		})
	}
}
