package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/localwiki"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestValidateLocalWikipediaSettings(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	s := &Server{Cfg: cfg}
	system := "/etc/wikipedia"
	if runtime.GOOS == "windows" {
		system = `C:\Windows\wikipedia`
	}
	for name, tc := range map[string]struct {
		dir     string
		docker  bool
		wantErr bool
	}{
		"empty uses the default": {dir: ""},
		"absolute directory":     {dir: filepath.Join(t.TempDir(), "wiki")},
		"relative directory":     {dir: "wiki", wantErr: true},
		"data directory root":    {dir: cfg.Directories.DataDir, wantErr: true},
		"system directory":       {dir: system, wantErr: true},
		"docker ignores the dir": {dir: "wiki", docker: true},
	} {
		err := validateLocalWikipediaSettings(s, config.LocalWikipediaConfig{DataDir: tc.dir}, config.Runtime{IsDocker: tc.docker})
		if (err != nil) != tc.wantErr {
			t.Fatalf("%s: validateLocalWikipediaSettings = %v, wantErr %v", name, err, tc.wantErr)
		}
	}
}

// Common installs keep AuraGo's data directory inside a tree the denylist
// refuses; the default <data_dir>/wikipedia and other folders below it must
// stay usable, while the data root itself, its parents, siblings and the
// spellings Windows reads as the root stay refused.
func TestLocalWikipediaSensitivePathAllowsDirectoriesBelowTheDataDir(t *testing.T) {
	type check struct {
		dir       string
		sensitive bool
	}
	cases := map[string][]check{}
	if runtime.GOOS == "windows" {
		cases[`C:\ProgramData\AuraGo\data`] = []check{
			{`C:\ProgramData\AuraGo\data\wikipedia`, false},
			{`C:\programdata\aurago\DATA\wikipedia`, false},
			{`C:\ProgramData\AuraGo\data\nested\wiki`, false},
			{`C:\ProgramData\AuraGo\data`, true},
			{`C:\ProgramData\AuraGo\data\`, true},
			{`c:\programdata\aurago\data`, true},
			{`C:\ProgramData\AuraGo\data\wiki\..`, true},
			{`C:\ProgramData\AuraGo\data\..\other`, true},
			{`C:\ProgramData\AuraGo`, true},
			{`C:\ProgramData\AuraGo\data2\wikipedia`, true},
			{`C:\ProgramData\AuraGo\data\. `, true},
			{`C:\ProgramData\AuraGo\data\wiki.`, true},
			{`C:\ProgramData\AuraGo\data\.::$INDEX_ALLOCATION`, true},
			{`C:\Windows\wikipedia`, true},
			{`D:\wiki`, false},
		}
		// A data directory directly below a volume root grants nothing.
		cases[`C:\Windows`] = []check{{`C:\Windows\wikipedia`, true}}
		cases[`C:\`] = []check{{`C:\Windows\wikipedia`, true}}
	} else {
		for _, root := range []string{
			"/root/aurago/data",
			"/usr/local/aurago/data",
			"/Users/aura/Library/Application Support/aurago/data",
		} {
			cases[root] = []check{
				{root + "/wikipedia", false},
				{root + "/nested/wiki", false},
				{root, true},
				{root + "/", true},
				{root + "/wiki/..", true},
				{root + "/../other", true},
				{root + "2/wikipedia", true},
				{root + "/wiki.", true},
				{"/etc/wikipedia", true},
				{"/srv/wiki", false},
			}
		}
		cases["/usr"] = []check{{"/usr/bin", true}}
		cases["/"] = []check{{"/etc/wiki", true}}
	}
	for root, checks := range cases {
		sensitive := localWikipediaSensitivePath(root)
		for _, c := range checks {
			if got := sensitive(c.dir); got != c.sensitive {
				t.Errorf("data_dir %q: sensitive(%q) = %v, want %v", root, c.dir, got, c.sensitive)
			}
		}
	}
	if got := localWikipediaSensitivePath("")(filepath.Join(t.TempDir(), "wiki")); got {
		t.Error("an empty data_dir refused an ordinary directory")
	}
}

// The data directory also counts in its resolved form, so the manager's
// check of a resolved storage path still sees the data root.
func TestLocalWikipediaSensitivePathKnowsTheResolvedDataDir(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real", "data")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	linkDirForTest(t, filepath.Join(base, "real"), link)
	resolved, err := localwiki.ResolveDirectory(real)
	if err != nil {
		t.Fatal(err)
	}
	sensitive := localWikipediaSensitivePath(filepath.Join(link, "data"))
	if !sensitive(resolved) {
		t.Fatalf("the resolved data root %s was not refused", resolved)
	}
	if !sensitive(filepath.Join(link, "data")) {
		t.Fatal("the data root as configured was not refused")
	}
	if sensitive(filepath.Join(resolved, "wikipedia")) {
		t.Fatal("a directory below the resolved data root was refused")
	}
}

func TestValidateLocalWikipediaSaveChecksOnlyChangedValues(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	s := &Server{Cfg: cfg}
	system := "/etc/wikipedia"
	if runtime.GOOS == "windows" {
		system = `C:\Windows\wikipedia`
	}
	invalid := config.LocalWikipediaConfig{Language: "xx", Variant: "mini", DataDir: system}
	if err := validateLocalWikipediaSave(s, invalid, invalid, config.Runtime{}); err != nil {
		t.Fatalf("unchanged hand-edited values blocked the save: %v", err)
	}
	unchangedDifferentCase := config.LocalWikipediaConfig{Language: " XX ", Variant: "MINI", DataDir: system + " "}
	if err := validateLocalWikipediaSave(s, invalid, unchangedDifferentCase, config.Runtime{}); err != nil {
		t.Fatalf("an unchanged value with different spacing blocked the save: %v", err)
	}
	for name, candidate := range map[string]config.LocalWikipediaConfig{
		"new language": {Language: "yy", Variant: "mini", DataDir: system},
		"new variant":  {Language: "xx", Variant: "midi", DataDir: system},
		"new data dir": {Language: "xx", Variant: "mini", DataDir: "relative/wiki"},
	} {
		if err := validateLocalWikipediaSave(s, invalid, candidate, config.Runtime{}); err == nil {
			t.Fatalf("%s: a changed invalid value was accepted", name)
		}
	}
	valid := config.LocalWikipediaConfig{Language: "de", Variant: "maxi", DataDir: filepath.Join(cfg.Directories.DataDir, "wikipedia")}
	if err := validateLocalWikipediaSave(s, invalid, valid, config.Runtime{}); err != nil {
		t.Fatalf("valid new values were refused: %v", err)
	}
	if got := localWikipediaSectionFromRaw(map[string]interface{}{
		"local_wikipedia": map[string]interface{}{"language": "xx", "data_dir": system},
	}); got.Language != "xx" || got.DataDir != system {
		t.Fatalf("decoded raw section = %+v", got)
	}
	if got := localWikipediaSectionFromRaw(map[string]interface{}{}); got != (config.LocalWikipediaConfig{}) {
		t.Fatalf("missing raw section = %+v, want the zero value", got)
	}
}

// A config.yaml whose local_wikipedia values were edited by hand (or became
// invalid) must not make every other config save fail with 400; a save that
// changes them to invalid values still does.
func TestHandleUpdateConfigKeepsSavingWithAnInvalidUnchangedLocalWikipediaSection(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	system := "/etc/wikipedia"
	if runtime.GOOS == "windows" {
		system = `C:\Windows\wikipedia`
	}
	raw := "agent:\n  system_language: Deutsch\nlocal_wikipedia:\n  enabled: true\n  language: xx\n  variant: mini\n  data_dir: '" + system + "'\n"
	if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("57", 32), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.ConfigPath = configPath
	manager := localwiki.NewManager(localwiki.Deps{})
	manager.Configure(localwiki.SettingsFromConfig(loaded))
	s := &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault, LocalWiki: manager}
	save := func(payload string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(payload)))
		return recorder
	}
	systemJSON, _ := json.Marshal(system)
	for _, payload := range []string{
		`{"local_wikipedia":{"update_check":false}}`,
		`{"local_wikipedia":{"enabled":true,"language":"xx","variant":"mini","data_dir":` + string(systemJSON) + `}}`,
		`{"agent":{"system_language":"English"}}`,
	} {
		if got := save(payload); got.Code != http.StatusOK {
			t.Fatalf("save %s = %d %s, want 200", payload, got.Code, got.Body.String())
		}
	}
	for _, payload := range []string{
		`{"local_wikipedia":{"language":"yy"}}`,
		`{"local_wikipedia":{"variant":"midi"}}`,
		`{"local_wikipedia":{"data_dir":"relative/wiki"}}`,
	} {
		if got := save(payload); got.Code != http.StatusBadRequest {
			t.Fatalf("save %s = %d, want 400", payload, got.Code)
		}
	}
}

func TestInjectLocalWikipediaDefaults(t *testing.T) {
	cfg := &config.Config{}
	cfg.LocalWikipedia = config.LocalWikipediaConfig{AgentAccess: true, Variant: "nopic", UpdateCheck: true}
	raw := map[string]interface{}{}
	injectLocalWikipediaDefaults(raw, cfg)
	section := raw["local_wikipedia"].(map[string]interface{})
	if section["enabled"] != false || section["agent_access"] != true || section["update_check"] != true ||
		section["variant"] != "nopic" || section["language"] != "" || section["data_dir"] != "" {
		t.Fatalf("injected defaults = %#v", section)
	}
	raw = map[string]interface{}{"local_wikipedia": map[string]interface{}{"agent_access": false, "variant": "maxi"}}
	injectLocalWikipediaDefaults(raw, cfg)
	section = raw["local_wikipedia"].(map[string]interface{})
	if section["agent_access"] != false || section["variant"] != "maxi" {
		t.Fatalf("explicit values were overwritten: %#v", section)
	}
}

func TestNewServerFromOptionsConstructsLocalWikipediaManager(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Agent.SystemLanguage = "Deutsch"
	t.Cleanup(func() { tools.SetLocalWikipediaSource(nil) })
	srv := newServerFromOptions(StartOptions{Cfg: cfg, Logger: slog.Default(), AccessLogger: slog.Default(), ShutdownCh: make(chan struct{})})
	if srv.LocalWiki == nil {
		t.Fatal("newServerFromOptions did not construct the Local Wikipedia manager")
	}
	if tools.PublishedLocalWikipediaManager() != srv.LocalWiki {
		t.Fatal("newServerFromOptions did not publish its Local Wikipedia manager to the tool")
	}
	settings := srv.LocalWiki.Settings()
	if settings.DataDir != filepath.Join(cfg.Directories.DataDir, "wikipedia") || settings.Language != "de" {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestHandleUpdateConfigReconfiguresLocalWikipedia(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("agent:\n  system_language: Deutsch\nlocal_wikipedia:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("57", 32), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.ConfigPath = configPath
	manager := localwiki.NewManager(localwiki.Deps{})
	manager.Configure(localwiki.SettingsFromConfig(loaded))
	s := &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault, LocalWiki: manager}
	save := func(payload string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(payload)))
		return recorder
	}

	if got := save(`{"local_wikipedia":{"enabled":true,"language":"fr","variant":"maxi"}}`); got.Code != http.StatusOK {
		t.Fatalf("save = %d %s", got.Code, got.Body.String())
	}
	settings := manager.Settings()
	if !settings.Enabled || settings.Language != "fr" || settings.Variant != localwiki.VariantMaxi || settings.SystemLanguage != "de" {
		t.Fatalf("manager not reconfigured: %+v", settings)
	}
	for _, payload := range []string{
		`{"local_wikipedia":{"language":"xx"}}`,
		`{"local_wikipedia":{"variant":"mini"}}`,
		`{"local_wikipedia":{"data_dir":"relative/wiki"}}`,
	} {
		if got := save(payload); got.Code != http.StatusBadRequest {
			t.Fatalf("save %s = %d, want 400", payload, got.Code)
		}
	}
}

// Overlapping syncs apply their snapshots in order. The hook holds the first
// sync after it read the older snapshot and before it configures the manager;
// a second sync, started after a newer snapshot was published, must wait for
// it and then apply the newer one. Without localWikiSyncMu the second sync
// would configure the newer settings first and the first would revert them.
func TestSyncLocalWikipediaSettingsAppliesSnapshotsInOrder(t *testing.T) {
	base := &config.Config{}
	base.Directories.DataDir = t.TempDir()
	base.Agent.SystemLanguage = "Deutsch"
	manager := localwiki.NewManager(localwiki.Deps{})
	manager.Configure(localwiki.SettingsFromConfig(base))
	s := &Server{Cfg: base, LocalWiki: manager}
	s.initConfigSnapshot()
	older, newer := base.Clone(), base.Clone()
	older.LocalWikipedia.Language = "fr"
	newer.LocalWikipedia.Language = "it"

	reached := make(chan struct{})
	proceed := make(chan struct{})
	release := sync.OnceFunc(func() { close(proceed) })
	defer release()
	var calls atomic.Int32
	s.localWikiBeforeConfigure = func() {
		if calls.Add(1) == 1 {
			close(reached)
			<-proceed
		}
	}
	s.cfgSnapshot.Store(older)
	first := make(chan struct{})
	go func() {
		s.syncLocalWikipediaSettings()
		close(first)
	}()
	<-reached
	s.cfgSnapshot.Store(newer)
	second := make(chan struct{})
	go func() {
		s.syncLocalWikipediaSettings()
		close(second)
	}()
	select {
	case <-second:
		t.Fatal("a second sync configured the manager while the first one was applying its snapshot")
	case <-time.After(100 * time.Millisecond):
	}
	if got := manager.Settings().Language; got != "de" {
		t.Fatalf("manager language while the first sync is held = %q, want the startup %q", got, "de")
	}
	release()
	<-first
	<-second
	if got := manager.Settings().Language; got != "it" {
		t.Fatalf("manager language = %q, want the newer snapshot's %q", got, "it")
	}
}

// Every config publisher reaches the manager, not only the config save: a
// system language changed elsewhere (the setup wizard) applies at once.
func TestReplaceConfigSnapshotConfiguresLocalWikipedia(t *testing.T) {
	base := &config.Config{}
	base.Directories.DataDir = t.TempDir()
	base.Agent.SystemLanguage = "Deutsch"
	manager := localwiki.NewManager(localwiki.Deps{})
	manager.Configure(localwiki.SettingsFromConfig(base))
	s := &Server{Cfg: base, Logger: slog.Default(), LocalWiki: manager}
	s.initConfigSnapshot()
	next := base.Clone()
	next.Agent.SystemLanguage = "Français"
	s.replaceConfigSnapshot(next)
	if settings := manager.Settings(); settings.SystemLanguage != "fr" || settings.Language != "fr" {
		t.Fatalf("settings after a system language change = %+v", settings)
	}
}

// Admin requests that run while a config save is publishing do not touch the
// manager's settings; only the save configures it. The hook holds the save
// after it published its snapshot and right before it configures the manager.
// Requests used to re-configure the manager from whichever snapshot they had
// read, which could revert a save that configured it in between.
func TestLocalWikipediaAdminRequestsDuringASaveKeepTheSavedSettings(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("agent:\n  system_language: Deutsch\nlocal_wikipedia:\n  enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("57", 32), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.ConfigPath = configPath
	manager := localwiki.NewManager(localwiki.Deps{})
	manager.Configure(localwiki.SettingsFromConfig(loaded))
	s := &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault, LocalWiki: manager}
	s.initConfigSnapshot()

	reached := make(chan struct{})
	proceed := make(chan struct{})
	release := sync.OnceFunc(func() { close(proceed) })
	defer release()
	var calls atomic.Int32
	s.localWikiBeforeConfigure = func() {
		if calls.Add(1) == 1 {
			close(reached)
			<-proceed
		}
	}
	saved := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		recorder := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config",
			strings.NewReader(`{"local_wikipedia":{"language":"fr"}}`)))
		saved <- recorder
	}()
	<-reached
	if got := s.ConfigSnapshot().LocalWikipedia.Language; got != "fr" {
		t.Fatalf("the save has not published its snapshot yet: language %q", got)
	}
	before := manager.Settings()
	// The handlers are called directly: requireAdmin would wait for the
	// config lock the save holds.
	for _, tc := range []struct {
		method, route string
		handler       http.HandlerFunc
	}{
		{http.MethodGet, "status", handleLocalWikipediaStatus(s)},
		{http.MethodPost, "check-update", handleLocalWikipediaCheckUpdate(s)},
		{http.MethodPost, "cancel", handleLocalWikipediaCancel(s)},
		{http.MethodPost, "delete", handleLocalWikipediaDelete(s)},
	} {
		request := httptest.NewRequest(tc.method, "/api/local-wikipedia/"+tc.route, nil)
		request.Header.Set("Origin", "http://example.com")
		recorder := httptest.NewRecorder()
		tc.handler.ServeHTTP(recorder, request)
		if recorder.Code >= http.StatusInternalServerError {
			t.Fatalf("%s during the save = %d %s", tc.route, recorder.Code, recorder.Body.String())
		}
		if got := manager.Settings(); got != before {
			t.Fatalf("the %s request re-configured the manager during the save: %+v, before %+v", tc.route, got, before)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("configure calls during the save = %d, want 1 (the save)", got)
	}
	release()
	if got := <-saved; got.Code != http.StatusOK {
		t.Fatalf("save = %d %s", got.Code, got.Body.String())
	}
	if got := manager.Settings().Language; got != "fr" {
		t.Fatalf("manager language after the save = %q, want %q", got, "fr")
	}
	recorder := httptest.NewRecorder()
	handleLocalWikipediaStatus(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/local-wikipedia/status", nil))
	var status localwiki.Status
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil || status.Selection.Language != "fr" {
		t.Fatalf("status after the save = %d %s (%v)", recorder.Code, recorder.Body.String(), err)
	}
}
