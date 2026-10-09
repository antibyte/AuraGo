package localwiki

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestManagerCatalogIsCachedForSixHours(t *testing.T) {
	env := newTestEnv(t)
	env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(1000))
	env.kiwix.addEdition("wikipedia_de_all_maxi_2026-01", testPayload(3000))
	env.kiwix.addEdition("wikipedia_nb_all_nopic_2026-09", testPayload(500))
	ctx := context.Background()

	info, err := env.manager.Catalog(ctx, "de")
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	spec, _ := lookupLanguage("de")
	if info.Language != "de" || info.Fulltext != fulltextSupported(spec) || len(info.Variants) != 2 ||
		info.Variants[VariantNoPic].Name != "wikipedia_de_all_nopic_2026-10" || info.Variants[VariantMaxi].Size != 3000 {
		t.Fatalf("catalog = %+v", info)
	}
	info.Variants[VariantNoPic] = CatalogEdition{}
	if again, _ := env.manager.Catalog(ctx, "de"); again.Variants[VariantNoPic].Name == "" || env.kiwix.catalogCount() != 1 {
		t.Fatalf("second call must come from the cache (requests %d)", env.kiwix.catalogCount())
	}
	env.clock.Advance(7 * time.Hour)
	if _, err := env.manager.Catalog(ctx, "de"); err != nil || env.kiwix.catalogCount() != 2 {
		t.Fatalf("expired cache not refreshed: %v, %d", err, env.kiwix.catalogCount())
	}
	norwegian, err := env.manager.Catalog(ctx, "no")
	if err != nil || norwegian.Language != "no" || norwegian.Variants[VariantNoPic].Name != "wikipedia_nb_all_nopic_2026-09" {
		t.Fatalf("Norwegian catalog = %+v, %v", norwegian, err)
	}
	if _, err := env.manager.Catalog(ctx, "nb"); !errors.Is(err, ErrUnknownLanguage) {
		t.Fatalf("Catalog(nb) = %v, want ErrUnknownLanguage", err)
	}
	env.kiwix.setCatalogStatus(http.StatusInternalServerError)
	if _, err := env.manager.catalog(ctx, "de", true); !errors.Is(err, ErrCatalogUnreachable) {
		t.Fatalf("unreachable catalog = %v", err)
	}
}

func TestManagerUpdateCheck(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.start()
	ctx := context.Background()

	env.manager.maybeCheckUpdate(ctx)
	if env.kiwix.catalogCount() != 0 {
		t.Fatal("the daily check ran although the last check is not a day old")
	}
	env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(5000))
	env.clock.Advance(25 * time.Hour)
	env.manager.maybeCheckUpdate(ctx)
	status := env.manager.Status()
	if env.kiwix.catalogCount() != 1 || status.UpdateAvailable == nil || *status.UpdateAvailable != (UpdateInfo{Date: "2026-10", Size: 5000}) {
		t.Fatalf("update not reported: %+v (requests %d)", status.UpdateAvailable, env.kiwix.catalogCount())
	}
	st, err := readState(env.dir)
	if err != nil || !st.LastUpdateCheck.Equal(env.clock.Now()) {
		t.Fatalf("last check not persisted: %+v, %v", st, err)
	}
	env.manager.maybeCheckUpdate(ctx)
	if env.kiwix.catalogCount() != 1 {
		t.Fatal("a second check ran within the same day")
	}

	settings := env.settings()
	settings.UpdateCheck = false
	env.manager.Configure(settings)
	env.clock.Advance(25 * time.Hour)
	env.manager.maybeCheckUpdate(ctx)
	if env.kiwix.catalogCount() != 1 {
		t.Fatal("update_check: false must stop the daily check")
	}
	if err := env.manager.CheckUpdate(ctx); err != nil || env.kiwix.catalogCount() != 2 {
		t.Fatalf("explicit CheckUpdate: %v (requests %d)", err, env.kiwix.catalogCount())
	}
	env.kiwix.setCatalogStatus(http.StatusBadGateway)
	if err := env.manager.CheckUpdate(ctx); !errors.Is(err, ErrCatalogUnreachable) {
		t.Fatalf("CheckUpdate with a broken catalog = %v", err)
	}
	if env.manager.Status().State != StateReady {
		t.Fatal("a failed update check must not affect the installed edition")
	}
}

func TestManagerLoopRunsTheDailyUpdateCheck(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(5000))
	env.clock.Advance(25 * time.Hour)
	env.manager.firstCheck = 10 * time.Millisecond
	env.start()
	status := env.waitFor("the update found by the background loop", func(s Status) bool { return s.UpdateAvailable != nil })
	if *status.UpdateAvailable != (UpdateInfo{Date: "2026-10", Size: 5000}) {
		t.Fatalf("update = %+v", status.UpdateAvailable)
	}
}
