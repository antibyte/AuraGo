package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestConfigEmbeddingReindexStatusBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => {await selectSection('embeddings',{scrollBehavior:'auto'});window.clearTimeout(embeddingsRuntimeRefreshTimer);}`)
	for _, provider := range []string{"local-granite", "openai"} {
		page.MustEval(`provider=>{document.querySelector('[data-path="embeddings.provider"]').value=provider;renderEmbeddingsRuntimeStatus({provider,state:'ready',index_pending:4,index_blocked:2,index_error:'source_missing'});}`, provider)
		if !page.MustEval(`()=>{const el=document.getElementById('emb-index-status');return el&&el.getBoundingClientRect().height>0&&el.textContent.includes('4')&&el.textContent.includes('2')&&el.textContent.includes('original image or audio')&&el.textContent.includes('preserved')&&!el.textContent.includes('config.embeddings.');}`).Bool() {
			t.Fatalf("missing visible safe reindex state for %s", provider)
		}
	}
	page.MustEval(`()=>renderEmbeddingsRuntimeStatus({provider:'openai',state:'ready'})`)
	if text := page.MustEval(`()=>document.getElementById('emb-index-status').textContent`).String(); text != "" {
		t.Fatalf("stale reindex state: %q", text)
	}
}

func TestEmbeddingReindexLocales(t *testing.T) {
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		data := mustReadUIFile(t, fmt.Sprintf("lang/config/misc/%s.json", lang))
		var values map[string]string
		if err := json.Unmarshal([]byte(data), &values); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"index_pending", "index_blocked", "index_error", "reset_point_memories", "reset_continue"} {
			if values["config.embeddings."+key] == "" {
				t.Fatalf("%s missing %s", lang, key)
			}
		}
		for _, key := range []string{"index_pending", "index_blocked"} {
			if !strings.Contains(values["config.embeddings."+key], "{count}") {
				t.Fatalf("%s missing count in %s", lang, key)
			}
		}
	}
}
