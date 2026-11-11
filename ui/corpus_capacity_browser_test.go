package ui

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCorpusLimitConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(40 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async()=>{configData.embeddings.provider='disabled';AuraConfigState.init(configData);await selectSection('embeddings',{scrollBehavior:'auto'});}`)
	selector := `[data-path="embeddings.corpus_max_size_mb"]`
	if !page.MustEval(`()=>{const el=document.querySelector('[data-path="embeddings.corpus_max_size_mb"]');return el && !el.disabled && el.type==='number' && el.value==='1024' && el.min==='1' && el.max==='1048576' && el.closest('.field-group').textContent.includes('Neustart');}`).Bool() {
		t.Fatal("disabled-provider limit field missing its default, bounds or restart help")
	}
	page.MustElement(selector).MustSelectAllText().MustInput("2048")
	page.MustEval(`async()=>{await saveConfig();await selectSection('embeddings',{scrollBehavior:'auto'});}`)
	if !page.MustEval(`async()=>{const saved=await(await fetch('/api/config')).json();return saved.embeddings.corpus_max_size_mb===2048 && document.querySelector('[data-path="embeddings.corpus_max_size_mb"]').value==='2048';}`).Bool() {
		t.Fatal("limit did not survive save/reopen")
	}
	page.MustElement(selector).MustSelectAllText().MustInput("0")
	if page.MustEval(`()=>document.querySelector('[data-path="embeddings.corpus_max_size_mb"]').checkValidity()`).Bool() {
		t.Fatal("invalid limit allowed")
	}
	page.MustEval(`()=>{AuraConfigState.discard();resetDirtySnapshot();}`)
}

func TestCorpusLimitLocales(t *testing.T) {
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		for section, keys := range map[string][]string{
			"config":    {"config.embeddings.corpus_max_size_mb_label"},
			"help":      {"help.embeddings.corpus_max_size_mb"},
			"dashboard": {"dashboard.memory_corpus_limit", "dashboard.memory_corpus_utilization", "dashboard.memory_corpus_wal", "dashboard.memory_corpus_disk_free", "dashboard.memory_corpus_reserve", "dashboard.memory_corpus_warning", "dashboard.memory_corpus_paused", "dashboard.corpus_limit_exceeded", "dashboard.corpus_disk_reserve", "dashboard.corpus_disk_status_unavailable", "dashboard.corpus_storage_full"},
		} {
			var texts map[string]string
			if err := json.Unmarshal(mustReadUIFile(t, "lang/"+section+"/"+lang+".json"), &texts); err != nil {
				t.Fatal(err)
			}
			for _, key := range keys {
				if texts[key] == "" {
					t.Errorf("%s: missing %s", lang, key)
				}
			}
		}
	}
}

func TestCorpusCapacityDashboardBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(newPrecisionSmokeOrigin(t))
	defer page.MustClose()
	page.MustSetDocumentContent(`<!doctype html><html><body><div id="memory-health-summary"></div></body></html>`)
	var texts map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/dashboard/de.json"), &texts); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`texts=>{window.t=key=>texts[key]||key;window.esc=value=>String(value);window.DashSections={register(){}};}`, texts)
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/dashboard/sections/memory.js"))); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{renderMemoryHealth({memory_health:{corpus:{available:true,documents:10,dimensions:[3],used_bytes:900*1048576,limit_bytes:1024*1048576,utilization_percent:88,wal_bytes:1048576,disk_free_bytes:400*1048576,disk_reserve_bytes:256*1048576,vector_payload_bytes:120,capacity_warning:true,pause_reason:'corpus_limit_exceeded'}}});}`)
	if !page.MustEval(`()=>{const text=document.getElementById('memory-health-summary').textContent;return ['Korpuslimit','Auslastung','Temporäre WAL-Datei','Freier Plattenplatz','256','80 %','Korpus über dem Limit'].every(s=>text.includes(s));}`).Bool() {
		t.Fatal("capacity, reserve or remedy missing")
	}
}
