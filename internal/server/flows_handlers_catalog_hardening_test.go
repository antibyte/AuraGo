package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/i18n"
	"aurago/internal/tools"
	"aurago/internal/webhooks"
	"aurago/ui"
)

// c19NodeTypes calls GET /node-types with an optional If-None-Match header.
func c19NodeTypes(t *testing.T, s *Server, token, lang, ifNoneMatch string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/desktop/flows/node-types?lang="+lang, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	if ifNoneMatch != "" {
		r.Header.Set("If-None-Match", ifNoneMatch)
	}
	w := httptest.NewRecorder()
	s.handleFlows(w, r)
	return w
}

// c19Builds returns how many palette answers the server built.
func c19Builds(s *Server) int {
	s.flowNodeTypesCache.mu.Lock()
	defer s.flowNodeTypesCache.mu.Unlock()
	return s.flowNodeTypesCache.builds
}

var c19I18nOnce sync.Once

// c19LoadTranslations loads the real UI translations once, so answers differ per language.
func c19LoadTranslations() {
	c19I18nOnce.Do(func() { i18n.Load(ui.Content, slog.New(slog.NewTextHandler(io.Discard, nil))) })
}

// c19Gzip compresses data as the gzip middleware does.
func c19Gzip(t *testing.T, data []byte) int {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Len()
}

// A1/A2: the palette answer is built once per language and key, carries an ETag and
// "no-cache" (not flowsJSON's no-store), and a registry change rebuilds it.
func TestC19NodeTypesAreCachedPerLanguageAndGeneration(t *testing.T) {
	c19LoadTranslations()
	s, token := newFlowsTestServer(t)
	first := c19NodeTypes(t, s, token, "en", "")
	if first.Code != http.StatusOK {
		t.Fatalf("node-types = %d %s", first.Code, first.Body.String())
	}
	etag := first.Header().Get("ETag")
	if !strings.HasPrefix(etag, `W/"`) || len(etag) != len(`W/""`)+32 {
		t.Fatalf("ETag = %q, want W/ and 32 hex digits", etag)
	}
	if cc := first.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") || strings.Contains(cc, "no-store") {
		t.Fatalf("Cache-Control = %q, want no-cache without no-store", cc)
	}
	if ct := first.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q", ct)
	}
	if c19Builds(s) != 1 {
		t.Fatalf("builds after the first request = %d", c19Builds(s))
	}
	second := c19NodeTypes(t, s, token, "en", "")
	if c19Builds(s) != 1 || second.Header().Get("ETag") != etag || !bytes.Equal(second.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("the second request was no cache hit: builds %d, ETag %q", c19Builds(s), second.Header().Get("ETag"))
	}
	de := c19NodeTypes(t, s, token, "de", "")
	if c19Builds(s) != 2 || de.Header().Get("ETag") == etag {
		t.Fatalf("German: builds %d, ETag %q (English %q)", c19Builds(s), de.Header().Get("ETag"), etag)
	}
	c19NodeTypes(t, s, token, "en", "")
	c19NodeTypes(t, s, token, "deutsch", "") // normalised to de
	if c19Builds(s) != 2 {
		t.Fatalf("both languages must stay cached; builds = %d", c19Builds(s))
	}

	s.Flows.Registry().Replace(&flows.NodeDef{Type: "c19.extra", Category: "logic", LabelKey: "c19.extra"})
	changed := c19NodeTypes(t, s, token, "en", etag)
	if changed.Code != http.StatusOK || c19Builds(s) != 3 || changed.Header().Get("ETag") == etag ||
		!strings.Contains(changed.Body.String(), `"c19.extra"`) {
		t.Fatalf("after a registry change: %d, builds %d, ETag %q", changed.Code, c19Builds(s), changed.Header().Get("ETag"))
	}
	if again := c19NodeTypes(t, s, token, "de", ""); c19Builds(s) != 4 || !strings.Contains(again.Body.String(), `"c19.extra"`) {
		t.Fatalf("a registry change must drop every language; builds %d", c19Builds(s))
	}
}

// A1: If-None-Match is compared weakly; a match is 304 without a body.
func TestC19NodeTypesAnswerIfNoneMatch(t *testing.T) {
	s, token := newFlowsTestServer(t)
	etag := c19NodeTypes(t, s, token, "en", "").Header().Get("ETag")
	strong := strings.TrimPrefix(etag, "W/")
	for _, header := range []string{etag, strong, `W/"0000", ` + etag, "*"} {
		w := c19NodeTypes(t, s, token, "en", header)
		if w.Code != http.StatusNotModified || w.Body.Len() != 0 || w.Header().Get("ETag") != etag {
			t.Fatalf("If-None-Match %q = %d, %d bytes, ETag %q", header, w.Code, w.Body.Len(), w.Header().Get("ETag"))
		}
		if cc := w.Header().Get("Cache-Control"); strings.Contains(cc, "no-store") {
			t.Fatalf("304 Cache-Control = %q", cc)
		}
	}
	for _, header := range []string{`W/"0000"`, `"` + strings.Repeat("0", 32) + `"`, " "} {
		if w := c19NodeTypes(t, s, token, "en", header); w.Code != http.StatusOK || w.Body.Len() == 0 {
			t.Fatalf("If-None-Match %q = %d, %d bytes", header, w.Code, w.Body.Len())
		}
	}
	if c19Builds(s) != 1 {
		t.Fatalf("revalidation must not rebuild; builds = %d", c19Builds(s))
	}
}

// A2: the cache key is (generation, configuration snapshot, language), stale puts are not
// kept, and the languages are bounded.
func TestC19NodeTypesCacheKey(t *testing.T) {
	var c flowNodeTypesCache
	cfgA, cfgB := &config.Config{}, &config.Config{}
	a := flowNodeTypesAnswer{body: []byte("a"), etag: `W/"a"`}
	c.put(1, cfgA, cfgA, "en", a)
	if got, ok := c.get(1, cfgA, "en"); !ok || got.etag != a.etag {
		t.Fatal("a put must be found")
	}
	for _, miss := range []struct {
		gen  uint64
		cfg  *config.Config
		lang string
	}{{2, cfgA, "en"}, {1, cfgB, "en"}, {1, cfgA, "de"}} {
		if _, ok := c.get(miss.gen, miss.cfg, miss.lang); ok {
			t.Fatalf("get(%d, %p, %s) must miss", miss.gen, miss.cfg, miss.lang)
		}
	}
	c.put(1, cfgB, cfgA, "de", a) // built for a configuration that is no longer current
	if _, ok := c.get(1, cfgB, "de"); ok {
		t.Fatal("an answer for a replaced configuration must not be kept")
	}
	if _, ok := c.get(1, cfgA, "en"); !ok {
		t.Fatal("a refused put must not drop the cached answers")
	}
	c.put(3, cfgA, cfgA, "en", a)
	c.put(2, cfgA, cfgA, "de", a) // a slow request of an older generation
	if _, ok := c.get(3, cfgA, "en"); !ok {
		t.Fatal("an older generation must not replace a newer one")
	}
	if _, ok := c.get(2, cfgA, "de"); ok {
		t.Fatal("an older generation must not be kept")
	}
	for i := range flowNodeTypesCacheLangs + 5 {
		c.put(3, cfgA, cfgA, string(rune('a'+i))+"x", a)
	}
	c.mu.Lock()
	n := len(c.answers)
	c.mu.Unlock()
	if n > flowNodeTypesCacheLangs {
		t.Fatalf("%d languages cached, the bound is %d", n, flowNodeTypesCacheLangs)
	}
	c.put(4, cfgB, cfgB, "en", a)
	if _, ok := c.get(3, cfgA, "en"); ok {
		t.Fatal("a new key must drop the old answers")
	}
}

// A1: the gzip middleware compresses the answer by its content type and keeps the ETag;
// a 304 passes uncompressed.
func TestC19NodeTypesAreGzipped(t *testing.T) {
	s, token := newFlowsTestServer(t)
	handler := gzipMiddleware(http.HandlerFunc(s.handleFlows))
	plain := c19NodeTypes(t, s, token, "en", "")
	call := func(ifNoneMatch string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/desktop/flows/node-types?lang=en", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Accept-Encoding", "gzip, deflate")
		if ifNoneMatch != "" {
			r.Header.Set("If-None-Match", ifNoneMatch)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	w := call("")
	if w.Code != http.StatusOK || w.Header().Get("Content-Encoding") != "gzip" || w.Header().Get("ETag") != plain.Header().Get("ETag") {
		t.Fatalf("gzip answer = %d, encoding %q, ETag %q", w.Code, w.Header().Get("Content-Encoding"), w.Header().Get("ETag"))
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(zr)
	if err != nil || !bytes.Equal(body, plain.Body.Bytes()) {
		t.Fatalf("the decompressed answer differs from the plain one (%v)", err)
	}
	notModified := call(plain.Header().Get("ETag"))
	if notModified.Code != http.StatusNotModified || notModified.Body.Len() != 0 || notModified.Header().Get("Content-Encoding") != "" {
		t.Fatalf("gzip 304 = %d, %d bytes, encoding %q", notModified.Code, notModified.Body.Len(), notModified.Header().Get("Content-Encoding"))
	}
}

// A1: the real answer sizes, for the test server and for the full tool configuration of
// 1c-12 (every integration on), with real translations.
func TestC19NodeTypesPayloadSize(t *testing.T) {
	c19LoadTranslations()
	s, token := newFlowsTestServer(t)
	for lang, category := range map[string]string{"en": `"Triggers"`, "de": `"Auslöser"`} {
		w := c19NodeTypes(t, s, token, lang, "")
		if !strings.Contains(w.Body.String(), category) {
			t.Fatalf("the %s answer is not translated (no %s)", lang, category)
		}
		var body struct {
			NodeTypes []map[string]any `json:"node_types"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		t.Logf("test server, %s: %d node types, %d bytes, %d bytes gzipped", lang, len(body.NodeTypes), w.Body.Len(), c19Gzip(t, w.Body.Bytes()))
	}

	full := &Server{Cfg: c12FullToolConfig(t), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	env := newFlowCatalogEnv(full)
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, env); err != nil {
		t.Fatal(err)
	}
	env.refreshRegistry(reg, full.ConfigSnapshot())
	for _, lang := range []string{"en", "de"} {
		answer, err := flowNodeTypesBody(reg, lang)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(answer.body), `"categories"`) {
			t.Fatal("the answer has no categories")
		}
		t.Logf("full configuration, %s: %d node types, %d bytes, %d bytes gzipped", lang, len(reg.All()), len(answer.body), c19Gzip(t, answer.body))
	}
}

// A2: concurrent palette requests while the registry changes always get a complete answer
// (run under -race on aurago-test).
func TestC19NodeTypesConcurrentRequests(t *testing.T) {
	s, token := newFlowsTestServer(t)
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 4 {
				lang := []string{"en", "de"}[(i+j)%2]
				w := c19NodeTypes(t, s, token, lang, "")
				if w.Code != http.StatusOK || !json.Valid(w.Body.Bytes()) {
					errs <- w.Body.String()
				}
			}
		}()
	}
	for i := range 4 {
		s.Flows.Registry().Replace(&flows.NodeDef{Type: "c19.concurrent", Category: "logic", LabelKey: "c19", Version: i + 1})
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("a concurrent request failed: %.200s", e)
	}
	final := c19NodeTypes(t, s, token, "en", "")
	if !strings.Contains(final.Body.String(), `"c19.concurrent"`) {
		t.Fatal("the answer after the changes misses the last definition")
	}
}

// c19HAToken is the Home Assistant token of the tests (long enough for the scrubber).
const c19HAToken = "c19-home-assistant-token-0123456789"

// c19UseHomeAssistant points the configuration at a fake Home Assistant.
func c19UseHomeAssistant(s *Server, url string) {
	s.Cfg.HomeAssistant.Enabled = true
	s.Cfg.HomeAssistant.URL = url
	s.Cfg.HomeAssistant.AccessToken = c19HAToken
}

// c19OptionValues returns the values of an options answer.
func c19OptionValues(opts []flowOption) []string {
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		out = append(out, o.Value)
	}
	return out
}

// B4: FLOW_OPTIONS_UNAVAILABLE carries Home Assistant's error text scrubbed and bounded.
func TestC19OptionErrorsAreBoundedAndScrubbed(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "invalid token "+c19HAToken+" "+strings.Repeat("x", 10<<10))
	}))
	defer ha.Close()
	c19UseHomeAssistant(s, ha.URL)
	w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/node-types/home.assistant/options/entity", token, "")
	body := flowsBody(t, w)
	msg, _ := body["error"].(string)
	if w.Code != http.StatusBadGateway || body["code"] != "FLOW_OPTIONS_UNAVAILABLE" || !strings.Contains(msg, "401") {
		t.Fatalf("answer = %d %.300s", w.Code, w.Body.String())
	}
	if n := utf8.RuneCountInString(msg); n > flowOptionsErrorRunes || !strings.HasSuffix(msg, "…") {
		t.Fatalf("the reason has %d runes, the bound is %d", n, flowOptionsErrorRunes)
	}
	if strings.Contains(w.Body.String(), c19HAToken) {
		t.Fatal("the Home Assistant token reached the answer")
	}
}

// B5: Home Assistant entities are cut at flowHAEntitiesMax (with "truncated"), bounded per
// label and hint, reused for flowHAEntitiesTTL per configuration snapshot, and an error is
// not reused.
func TestC19HomeAssistantEntitiesAreCachedBoundedAndTruncated(t *testing.T) {
	s, token := newFlowsTestServer(t)
	var hits, count atomic.Int32
	var failing atomic.Bool
	count.Store(flowHAEntitiesMax + 5)
	long := strings.Repeat("Ä", 130)
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/api/states" || r.Header.Get("Authorization") != "Bearer "+c19HAToken {
			http.NotFound(w, r)
			return
		}
		if failing.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		states := []map[string]any{}
		for i := int32(0); i < count.Load(); i++ {
			// Reverse order: the answer must sort by entity id.
			id := fmt.Sprintf("sensor.c19_%04d", count.Load()-1-i)
			states = append(states, map[string]any{"entity_id": id, "state": long, "attributes": map[string]any{"friendly_name": long}})
		}
		_ = json.NewEncoder(w).Encode(states)
	}))
	defer ha.Close()
	c19UseHomeAssistant(s, ha.URL)
	now := time.Now()
	s.flowHACache.now = func() time.Time { return now }
	get := func(node string) (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/node-types/"+node+"/options/entity", token, "")
		return w, flowsBody(t, w)
	}

	w, body := get("trigger.ha_state")
	opts, _ := body["options"].([]any)
	if w.Code != http.StatusOK || len(opts) != flowHAEntitiesMax || body["truncated"] != true || hits.Load() != 1 {
		t.Fatalf("first answer = %d, %d options, truncated %v, %d requests", w.Code, len(opts), body["truncated"], hits.Load())
	}
	first, _ := opts[0].(map[string]any)
	if first["value"] != "sensor.c19_0000" {
		t.Fatalf("the first option is %v, want the smallest entity id", first["value"])
	}
	for _, raw := range opts {
		o, _ := raw.(map[string]any)
		label, _ := o["label"].(string)
		hint, _ := o["hint"].(string)
		if utf8.RuneCountInString(label) > flowOptionTextRunes || utf8.RuneCountInString(hint) > flowOptionTextRunes ||
			!strings.HasSuffix(label, "…") {
			t.Fatalf("option not bounded: label %d runes, hint %d runes", utf8.RuneCountInString(label), utf8.RuneCountInString(hint))
		}
	}

	if w, _ := get("home.assistant"); w.Code != http.StatusOK || hits.Load() != 1 {
		t.Fatalf("a second select within the TTL asked Home Assistant again (%d requests)", hits.Load())
	}
	now = now.Add(flowHAEntitiesTTL)
	if get("home.assistant"); hits.Load() != 2 {
		t.Fatalf("an expired list was reused (%d requests)", hits.Load())
	}
	next := *s.Cfg
	s.cfgSnapshot.Store(&next)
	if get("home.assistant"); hits.Load() != 3 {
		t.Fatalf("a new configuration snapshot reused the old list (%d requests)", hits.Load())
	}

	failing.Store(true)
	now = now.Add(flowHAEntitiesTTL)
	if w, _ := get("home.assistant"); w.Code != http.StatusBadGateway || hits.Load() != 4 {
		t.Fatalf("Home Assistant failing = %d, %d requests", w.Code, hits.Load())
	}
	failing.Store(false)
	count.Store(3)
	if w, _ := get("home.assistant"); w.Code != http.StatusBadGateway || hits.Load() != 4 {
		t.Fatalf("an error within flowHAEntitiesErrorTTL must be reused: %d, %d requests", w.Code, hits.Load())
	}
	now = now.Add(flowHAEntitiesErrorTTL)
	w, body = get("home.assistant")
	if opts, _ := body["options"].([]any); w.Code != http.StatusOK || hits.Load() != 5 || len(opts) != 3 {
		t.Fatalf("after an error = %d, %d requests, %d options", w.Code, hits.Load(), len(opts))
	}
	if _, cut := body["truncated"]; cut {
		t.Fatal("a complete list must not say truncated")
	}
}

// B7: AI model options list only providers that can answer an ai.step, and the default
// names the model the default route uses.
// c19ProviderFixtures are provider entries of every kind the AI model options must sort
// out: chat providers, media types, missing models and account ids, the reserved local
// provider, unknown types, a blank and a padded id, and OAuth2.
func c19ProviderFixtures() []config.ProviderEntry {
	return []config.ProviderEntry{
		{ID: "main", Name: "Main", Type: "openai", Model: "gpt-c19"},
		{ID: "images", Type: "stability", Model: "sd3"},
		{ID: "eyes", Type: "vision", Model: "v1"},
		{ID: "art", Type: "agnes", Model: "agnes-image-1"},
		{ID: "chatty", Type: "agnes", Model: "agnes-chat"},
		{ID: "nomodel", Type: "openai"},
		{ID: "generic", Model: "local-model"},
		{ID: "generic_nomodel"},
		{ID: "cf", Type: "workers-ai", Model: "@cf/a"},
		{ID: "cf2", Type: "workers-ai", Model: "@cf/b", AccountID: "acc"},
		{ID: config.LocalLLMProviderID, Type: "openai", Model: "qwen"},
		{ID: "mystery", Type: "no-such-type", Model: "m"},
		{ID: " ", Type: "openai", Model: "m"},
		{ID: " padded", Type: "openai", Model: "m"},
		{ID: "oauth", Type: "openai", Model: "m", AuthType: "oauth2"},
		{ID: "keyless", Type: "custom", BaseURL: "http://127.0.0.1:1", Model: "m"},
	}
}

func TestC19AIModelOptionsAreChatProviders(t *testing.T) {
	c19LoadTranslations()
	s, _ := newFlowsTestServer(t)
	s.Cfg.LLM.Model = "main-llm"
	s.Cfg.Providers = c19ProviderFixtures()
	ctx := context.Background()
	opts, err := s.flowOptions(ctx, "ai_models", "en")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c19OptionValues(opts), []string{"", "main", "chatty", "generic", "cf2", "oauth", "keyless"}; !slices.Equal(got, want) {
		t.Fatalf("AI model options = %v, want %v", got, want)
	}
	defaultLabel := i18n.T("en", "easydrag.option.ai_model_default")
	if opts[0].Label != defaultLabel+" (main-llm)" {
		t.Fatalf("default without flows.ai_provider = %q", opts[0].Label)
	}
	s.Cfg.Flows.AIProvider = "main"
	if opts, _ := s.flowOptions(ctx, "ai_models", "en"); opts[0].Label != defaultLabel+" (gpt-c19)" {
		t.Fatalf("default with flows.ai_provider = %q", opts[0].Label)
	}
	s.Cfg.Flows.AIProvider = "gone"
	if opts, _ := s.flowOptions(ctx, "ai_models", "en"); opts[0].Label != defaultLabel {
		t.Fatalf("default with an unknown flows.ai_provider = %q", opts[0].Label)
	}
}

// B8: notification channel options are channels tools.SendNotification sends to; a
// read-only Discord, or one without a default channel, is not offered.
func TestC19NotificationChannelOptions(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	accepted := []string{string(tools.ChannelAll), string(tools.ChannelPush), string(tools.ChannelTelegram), string(tools.ChannelDiscord),
		string(tools.ChannelNtfy), string(tools.ChannelPushover), string(tools.ChannelTelnyx), string(tools.ChannelCYD)}
	channels := func() []string {
		t.Helper()
		opts, err := s.flowOptions(context.Background(), "notification_channels", "en")
		if err != nil {
			t.Fatal(err)
		}
		values := c19OptionValues(opts)
		for _, v := range values {
			if !slices.Contains(accepted, v) {
				t.Fatalf("channel %q is no channel SendNotification accepts", v)
			}
		}
		return values
	}
	s.Cfg.Telegram.BotToken, s.Cfg.Telegram.UserID = "1:x", 7
	s.Cfg.Notifications.Ntfy.Enabled, s.Cfg.Notifications.Pushover.Enabled = true, true
	s.Cfg.Discord.Enabled, s.Cfg.Discord.ReadOnly, s.Cfg.Discord.DefaultChannelID = true, true, "123"
	if got := channels(); slices.Contains(got, "discord") || !slices.Equal(got, []string{"all", "push", "telegram", "ntfy", "pushover"}) {
		t.Fatalf("read-only Discord: %v", got)
	}
	s.Cfg.Discord.ReadOnly, s.Cfg.Discord.DefaultChannelID = false, " "
	if got := channels(); slices.Contains(got, "discord") {
		t.Fatalf("Discord without a default channel: %v", got)
	}
	s.Cfg.Discord.DefaultChannelID = "123"
	if got := channels(); !slices.Contains(got, "discord") {
		t.Fatalf("usable Discord: %v", got)
	}
}

// B9: webhook options list every webhook, as Mission Control's picker does; a disabled one
// says so in its hint.
func TestC19WebhookOptionsMarkDisabledWebhooks(t *testing.T) {
	c19LoadTranslations()
	s, _ := newFlowsTestServer(t)
	dir := t.TempDir()
	mgr, err := webhooks.NewManager(filepath.Join(dir, "webhooks.json"), filepath.Join(dir, "webhook_log.json"))
	if err != nil {
		t.Fatal(err)
	}
	on, err := mgr.Create(webhooks.Webhook{Name: "Door", Slug: "c19-door", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	off, err := mgr.Create(webhooks.Webhook{Name: "Old", Slug: "c19-old", Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	s.WebhookManager = mgr
	opts, err := s.flowOptions(context.Background(), "webhooks", "de")
	if err != nil {
		t.Fatal(err)
	}
	hints := map[string]string{}
	for _, o := range opts {
		hints[o.Value] = o.Hint
	}
	if len(opts) != 2 || hints[on.ID] != "c19-door" || hints[off.ID] != "c19-old · "+i18n.T("de", "easydrag.option.webhook_disabled") {
		t.Fatalf("webhook options = %+v", opts)
	}
	if hints[off.ID] == "c19-old · easydrag.option.webhook_disabled" {
		t.Fatal("the disabled hint is not translated")
	}
}

// B6: mission options name their kind; an older agent mission without an execution type
// is "agent". Labels are bounded (the agent names missions).
func TestC19MissionOptionsNameTheirKind(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("m", 300)
	missions := `[{"id":"m_legacy","name":"Legacy","prompt":"x","enabled":true},
 {"id":"m_sched","name":"` + long + `","prompt":"y","execution_type":"scheduled","enabled":true},
 {"id":"m_flow","name":"Flow","execution_type":"flow","flow_id":"flow_c19","enabled":true}]`
	if err := os.WriteFile(filepath.Join(dir, "missions_v2.json"), []byte(missions), 0o600); err != nil {
		t.Fatal(err)
	}
	mm := tools.NewMissionManagerV2(dir, nil)
	if err := mm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mm.Stop)
	s := &Server{Cfg: &config.Config{}, MissionManagerV2: mm}
	opts, err := s.flowOptions(context.Background(), "missions", "en")
	if err != nil {
		t.Fatal(err)
	}
	hints := map[string]string{}
	for _, o := range opts {
		hints[o.Value] = o.Hint
		if utf8.RuneCountInString(o.Label) > flowOptionTextRunes {
			t.Fatalf("label of %s has %d runes", o.Value, utf8.RuneCountInString(o.Label))
		}
	}
	if want := map[string]string{"m_legacy": "agent", "m_sched": "scheduled", "m_flow": "flow"}; !reflect.DeepEqual(hints, want) {
		t.Fatalf("mission hints = %v, want %v", hints, want)
	}
}

// B10: validate works in read-only mode behind the desktop permission and the origin
// check, takes only draft or publish (empty means draft) and maps a document refusal.
func TestC19ValidateRules(t *testing.T) {
	s, token := newFlowsTestServer(t)
	const doc = `{"schema":1,"name":"","nodes":[],"edges":[]}`
	validate := func(tok, body string) *httptest.ResponseRecorder {
		t.Helper()
		return flowsCall(t, s, http.MethodPost, "/api/desktop/flows/validate", tok, body)
	}
	if w := validate(token, `{"doc":`+doc+`,"mode":"live"}`); w.Code != http.StatusBadRequest || flowsBody(t, w)["code"] != "FLOW_BAD_REQUEST" {
		t.Fatalf("unknown mode = %d %s", w.Code, w.Body.String())
	}
	empty := flowsBody(t, validate(token, `{"doc":`+doc+`}`))
	draft := flowsBody(t, validate(token, `{"doc":`+doc+`,"mode":"draft"}`))
	publish := flowsBody(t, validate(token, `{"doc":`+doc+`,"mode":"publish"}`))
	if !reflect.DeepEqual(empty, draft) || reflect.DeepEqual(draft, publish) {
		t.Fatalf("empty mode = %v, draft = %v, publish = %v", empty, draft, publish)
	}
	big := `{"doc":{"schema":1,"name":"x","nodes":[],"edges":[],"description":"` + strings.Repeat("a", flows.MaxDocumentBytes) + `"}}`
	if w := validate(token, big); w.Code != http.StatusRequestEntityTooLarge || flowsBody(t, w)["code"] != "FLOW_TOO_LARGE" {
		t.Fatalf("oversized document = %d %.200s", w.Code, w.Body.String())
	}

	for _, readOnly := range []func(bool){
		func(on bool) { s.Cfg.Tools.Missions.ReadOnly = on },
		func(on bool) { s.Cfg.VirtualDesktop.ReadOnly = on },
	} {
		readOnly(true)
		if w := validate(token, `{"doc":`+doc+`}`); w.Code != http.StatusOK {
			t.Fatalf("validate while read-only = %d %s", w.Code, w.Body.String())
		}
		if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"name":"x"}`); w.Code != http.StatusForbidden {
			t.Fatalf("create while read-only = %d", w.Code)
		}
		readOnly(false)
	}

	s.Cfg.Tools.Missions.ReadOnly = true
	readToken, _, err := s.TokenManager.Create("c19 read", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := validate(readToken, `{"doc":`+doc+`}`); w.Code != http.StatusForbidden {
		t.Fatalf("validate with a read-only token = %d", w.Code)
	}
	session := func(origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/desktop/flows/validate", strings.NewReader(`{"doc":`+doc+`}`))
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		s.handleFlows(w, r)
		return w
	}
	if w := session("https://evil.example"); w.Code != http.StatusForbidden || flowsBody(t, w)["code"] != "FLOW_PERMISSION_DENIED" {
		t.Fatalf("cross-origin session validate = %d %s", w.Code, w.Body.String())
	}
	if w := session("http://example.com"); w.Code != http.StatusOK {
		t.Fatalf("same-origin session validate = %d %s", w.Code, w.Body.String())
	}
}

// B11: unknown catalog routes are a clean 404 FLOW_NOT_FOUND.
func TestC19UnknownCatalogRoutes(t *testing.T) {
	s, token := newFlowsTestServer(t)
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/desktop/flows/node-types/notify.push/options/channel/extra"},
		{http.MethodGet, "/api/desktop/flows/node-types/notify.push"},
		{http.MethodGet, "/api/desktop/flows/node-types/notify.push/fields/channel"},
		{http.MethodGet, "/api/desktop/flows/templates/x"},
		{http.MethodPost, "/api/desktop/flows/templates/publish"},
		{http.MethodPost, "/api/desktop/flows/validate/x"},
	} {
		w := flowsCall(t, s, c.method, c.path, token, "")
		if w.Code != http.StatusNotFound || flowsBody(t, w)["code"] != "FLOW_NOT_FOUND" {
			t.Fatalf("%s %s = %d %s", c.method, c.path, w.Code, w.Body.String())
		}
	}
	if w := flowsCall(t, s, http.MethodDelete, "/api/desktop/flows/templates", token, ""); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE templates = %d", w.Code)
	}
}

// c19Secrets is a vault that has a credential for every provider: an API key, and a
// valid OAuth2 token.
type c19Secrets struct{}

func (c19Secrets) ReadSecret(key string) (string, error) {
	if strings.HasPrefix(key, "oauth_") {
		return `{"access_token":"c19-access-token","expiry":"2999-01-01T00:00:00Z"}`, nil
	}
	return "c19-api-key", nil
}

// Review item 1: the AI model options offer exactly the providers a run can use once the
// credentials are there; flowChatProvider and flowProviderEntry share one entry check.
func TestC19AIModelOptionsMatchTheRunTimeCheck(t *testing.T) {
	cfg := &config.Config{Providers: c19ProviderFixtures()}
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		_, err := flowProviderEntry(cfg, p.ID, c19Secrets{})
		if offered, usable := flowChatProvider(p), err == nil; offered != usable {
			t.Errorf("provider %q (type %q): offered %v, but the run-time check says %v (%v)", p.ID, p.Type, offered, usable, err)
		}
	}
}

// Review item 2: the HA cache keeps an answer only for the current configuration.
func TestC19HACacheKeepsOnlyTheCurrentConfiguration(t *testing.T) {
	var c flowHACache
	cfgA, cfgB := &config.Config{}, &config.Config{}
	list := flowOptionList{options: []flowOption{{Value: "a"}}}
	c.put(cfgB, cfgA, list, nil)
	if _, _, ok := c.get(cfgB); ok {
		t.Fatal("an answer for a replaced configuration must not be kept")
	}
	c.put(cfgA, cfgA, list, nil)
	c.put(cfgB, cfgA, flowOptionList{}, nil)
	if got, _, ok := c.get(cfgA); !ok || len(got.options) != 1 {
		t.Fatal("a refused put must not replace the current answer")
	}
}

// Review item 3: requests that miss the HA cache at the same time share one Home
// Assistant request.
func TestC19HomeAssistantFetchesAreCoalesced(t *testing.T) {
	s, token := newFlowsTestServer(t)
	var hits atomic.Int32
	release := make(chan struct{})
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-release
		_ = json.NewEncoder(w).Encode([]map[string]any{{"entity_id": "light.c19", "state": "on"}})
	}))
	defer ha.Close()
	var releaseOnce sync.Once
	open := func() { releaseOnce.Do(func() { close(release) }) }
	defer open() // before ha.Close, so no handler stays blocked
	c19UseHomeAssistant(s, ha.URL)

	const callers = 8
	codes := make(chan int, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- flowsCall(t, s, http.MethodGet, "/api/desktop/flows/node-types/home.assistant/options/entity", token, "").Code
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for hits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond) // the other callers reach the shared fetch meanwhile
	open()
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != http.StatusOK {
			t.Fatalf("a caller got %d", code)
		}
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("%d concurrent cold requests made %d Home Assistant requests, want 1", callers, n)
	}
}

// Review item 7: HEAD /node-types is GET without the body, revalidation included.
func TestC19NodeTypesAnswerHead(t *testing.T) {
	s, token := newFlowsTestServer(t)
	get := c19NodeTypes(t, s, token, "en", "")
	head := func(path, ifNoneMatch string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodHead, path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		if ifNoneMatch != "" {
			r.Header.Set("If-None-Match", ifNoneMatch)
		}
		w := httptest.NewRecorder()
		s.handleFlows(w, r)
		return w
	}
	w := head("/api/desktop/flows/node-types?lang=en", "")
	if w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get("ETag") != get.Header().Get("ETag") ||
		w.Header().Get("Content-Length") != fmt.Sprint(get.Body.Len()) || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("HEAD = %d, %d bytes, headers %v", w.Code, w.Body.Len(), w.Header())
	}
	if w := head("/api/desktop/flows/node-types?lang=en", get.Header().Get("ETag")); w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("HEAD with a matching If-None-Match = %d", w.Code)
	}
	if w := head("/api/desktop/flows/node-types/notify.push/options/channel", ""); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("HEAD on an options route = %d", w.Code)
	}
	if c19Builds(s) != 1 {
		t.Fatalf("HEAD must use the cache; builds = %d", c19Builds(s))
	}
}

// Review item 8: a new configuration snapshot rebuilds the palette once.
func TestC19NodeTypesRebuildOnceAfterAConfigSwap(t *testing.T) {
	s, token := newFlowsTestServer(t)
	first := c19NodeTypes(t, s, token, "en", "")
	before := c19Builds(s)
	next := *s.ConfigSnapshot()
	s.cfgSnapshot.Store(&next)
	if w := c19NodeTypes(t, s, token, "en", first.Header().Get("ETag")); w.Code != http.StatusNotModified || c19Builds(s) != before+1 {
		t.Fatalf("after a config swap = %d, builds %d (before %d); want one rebuild with the same content", w.Code, c19Builds(s), before)
	}
	if c19NodeTypes(t, s, token, "en", ""); c19Builds(s) != before+1 {
		t.Fatalf("the rebuilt answer was not cached; builds %d", c19Builds(s))
	}
}
