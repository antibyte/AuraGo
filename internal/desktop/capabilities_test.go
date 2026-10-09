package desktop

import (
	"context"
	"testing"
)

type staticCapabilities map[string]bool

func (c staticCapabilities) HasCapability(name string) bool { return c[name] }

func TestFilterAvailableAppsFailsClosedWithoutProvider(t *testing.T) {
	apps := []AppManifest{{ID: "plain"}, {ID: "radio-hw", Requires: []string{"radio"}}}
	if got := FilterAvailableApps(apps, nil); len(got) != 1 || got[0].ID != "plain" {
		t.Fatalf("without provider = %+v, want only plain", got)
	}
	if got := FilterAvailableApps(apps, staticCapabilities{"radio": true}); len(got) != 2 {
		t.Fatalf("with radio capability = %+v, want both apps", got)
	}
}

func TestBuiltinBluetoothAppRequiresBluetooth(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "bluetooth")
	if app.Entry != "builtin://bluetooth" || app.Icon != "bluetooth" {
		t.Fatalf("bluetooth manifest entry/icon = %q/%q", app.Entry, app.Icon)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "bluetooth" {
		t.Fatalf("bluetooth requires = %v, want [bluetooth]", app.Requires)
	}
}

func TestBootstrapHidesBluetoothUntilCapabilityIsPresent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := testService(t)
	hasBluetooth := func() bool {
		bootstrap, err := svc.Bootstrap(ctx)
		if err != nil {
			t.Fatalf("Bootstrap: %v", err)
		}
		for _, app := range bootstrap.BuiltinApps {
			if app.ID == "bluetooth" {
				return true
			}
		}
		return false
	}
	if hasBluetooth() {
		t.Fatal("bluetooth must stay hidden without a capability provider")
	}
	capabilities := staticCapabilities{"bluetooth": true}
	svc.SetCapabilityProvider(capabilities)
	if !hasBluetooth() {
		t.Fatal("bluetooth must appear once the capability is present")
	}
	capabilities["bluetooth"] = false
	if hasBluetooth() {
		t.Fatal("bluetooth must disappear when the capability goes away")
	}
}

func TestBootstrapOmitsButKeepsShortcutsToHiddenApps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := testService(t)
	capabilities := staticCapabilities{"bluetooth": true}
	svc.SetCapabilityProvider(capabilities)
	if err := svc.AddDesktopAppShortcut(ctx, "bluetooth", SourceUser); err != nil {
		t.Fatalf("AddDesktopAppShortcut: %v", err)
	}
	hasShortcut := func() bool {
		bootstrap, err := svc.Bootstrap(ctx)
		if err != nil {
			t.Fatalf("Bootstrap: %v", err)
		}
		for _, shortcut := range bootstrap.Shortcuts {
			if shortcut.TargetType == ShortcutTargetApp && shortcut.TargetID == "bluetooth" {
				return true
			}
		}
		return false
	}
	if !hasShortcut() {
		t.Fatal("shortcut missing while bluetooth is available")
	}
	capabilities["bluetooth"] = false
	if hasShortcut() {
		t.Fatal("shortcut to a hidden app must be omitted")
	}
	capabilities["bluetooth"] = true
	if !hasShortcut() {
		t.Fatal("shortcut must return with the capability; it must not be deleted")
	}
}

func TestFindAppTreatsUnavailableBuiltinAsMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := testService(t)
	if _, ok, err := svc.findApp(ctx, "bluetooth"); err != nil || ok {
		t.Fatalf("findApp without capability = ok:%v err:%v, want not found", ok, err)
	}
	svc.SetCapabilityProvider(staticCapabilities{"bluetooth": true})
	if _, ok, err := svc.findApp(ctx, "bluetooth"); err != nil || !ok {
		t.Fatalf("findApp with capability = ok:%v err:%v, want found", ok, err)
	}
}

func TestBuiltinEasyDragAppRequiresFlows(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "easydrag")
	if app.Entry != "builtin://easydrag" || app.Icon != "easydrag" || app.Runtime != BuiltinRuntime {
		t.Fatalf("easydrag manifest = %+v", app)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "flows" {
		t.Fatalf("easydrag requires = %v, want [flows]", app.Requires)
	}
	if app.Metadata["open_maximized"] != "true" || !app.DockVisible || !app.StartVisible || app.Deletable {
		t.Fatalf("easydrag must be a visible, maximized first-party app: %+v", app)
	}
}

func TestBootstrapHidesEasyDragUntilFlowsArePresent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := testService(t)
	hasEasyDrag := func() bool {
		bootstrap, err := svc.Bootstrap(ctx)
		if err != nil {
			t.Fatalf("Bootstrap: %v", err)
		}
		for _, app := range bootstrap.BuiltinApps {
			if app.ID == "easydrag" {
				return true
			}
		}
		return false
	}
	if hasEasyDrag() {
		t.Fatal("easydrag must stay hidden without the flows capability")
	}
	svc.SetCapabilityProvider(staticCapabilities{"flows": true})
	if !hasEasyDrag() {
		t.Fatal("easydrag must appear once flows are available")
	}
}

func TestBuiltinLocalWikipediaAppRequiresCapability(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "local-wikipedia")
	if app.Entry != "builtin://local-wikipedia" || app.Name != "Wikipedia" || app.Icon != "book" || app.Category != "office" {
		t.Fatalf("local-wikipedia manifest = %+v", app)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "local_wikipedia" {
		t.Fatalf("local-wikipedia requires = %v, want [local_wikipedia]", app.Requires)
	}
	if !app.StartVisible || !app.Builtin {
		t.Fatalf("local-wikipedia must be a visible builtin: %+v", app)
	}
	iconKnown := false
	for _, name := range desktopPreferredIconNames {
		if name == app.Icon {
			iconKnown = true
		}
	}
	if !iconKnown {
		t.Fatalf("icon %q is not a preferred desktop icon", app.Icon)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 0 {
		t.Fatalf("without the capability the app must stay hidden: %+v", got)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{"local_wikipedia": true}); len(got) != 1 {
		t.Fatalf("with the capability the app must show: %+v", got)
	}
}
