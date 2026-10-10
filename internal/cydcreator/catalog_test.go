package cydcreator

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// describeProps renders props like the overview table: name:kind(range)=default,
// enums as name:enum(a,b)=default.
func describeProps(props []PropSpec) string {
	parts := make([]string, len(props))
	for i, p := range props {
		kind := p.Kind
		switch {
		case p.Kind == PropEnum:
			kind += "(" + strings.Join(p.Options, ",") + ")"
		case p.Min != nil && p.Max != nil:
			kind += fmt.Sprintf("(%d..%d)", *p.Min, *p.Max)
		}
		def, _ := json.Marshal(p.Default)
		parts[i] = p.Name + ":" + kind + "=" + string(def)
	}
	return strings.Join(parts, " ")
}

func TestWidgetCatalogMatchesContract(t *testing.T) {
	type row struct {
		typ, group string
		w, h       int
		container  bool
		events     string
		props      string
	}
	want := []row{
		{"label", "basic", 120, 24, false, "clicked,long_pressed",
			`text:text="Label" font:font="" size:font_size=0 align:enum(left,center,right)="left" long_mode:enum(wrap,dots,scroll,clip)="wrap"`},
		{"button", "basic", 120, 40, false, "clicked,long_pressed",
			`text:string="Button" symbol:symbol="" font:font="" size:font_size=0`},
		{"image", "basic", 64, 64, false, "clicked,long_pressed", `asset:asset=""`},
		{"panel", "basic", 160, 100, true, "clicked,long_pressed", `scrollable:bool=false`},
		{"line", "basic", 120, 8, false, "",
			`orientation:enum(horizontal,vertical)="horizontal" thickness:int(1..16)=2`},
		{"switch", "input", 56, 28, false, "value_changed", `checked:bool=false`},
		{"checkbox", "input", 140, 28, false, "value_changed", `text:string="Option" checked:bool=false`},
		{"slider", "input", 160, 24, false, "value_changed", `min:int=0 max:int=100 value:int=50`},
		{"arc", "input", 120, 120, false, "value_changed", `min:int=0 max:int=100 value:int=50`},
		{"dropdown", "input", 140, 40, false, "value_changed", `options:list=["Eins","Zwei","Drei"] selected:int=0`},
		{"roller", "input", 120, 100, false, "value_changed",
			`options:list=["Eins","Zwei","Drei"] selected:int=0 visible_rows:int(1..7)=3`},
		{"textarea", "input", 200, 40, false, "value_changed,ready",
			`text:string="" placeholder:string="" one_line:bool=true password:bool=false max_length:int(1..256)=64 keyboard:enum(text,number)="text"`},
		{"spinbox", "input", 140, 40, false, "value_changed",
			`min:int=0 max:int=100 value:int=0 step:int(1..1000)=1 digits:int(1..8)=3 decimals:int(0..3)=0`},
		{"keypad", "input", 200, 160, false, "value_changed",
			`layout:enum(numeric,numeric_ok,custom)="numeric" keys:list=[]`},
		{"bar", "display", 160, 20, false, "", `min:int=0 max:int=100 value:int=30`},
		{"spinner", "display", 48, 48, false, "", ``},
		{"led", "display", 24, 24, false, "", `color:color="" brightness:int(0..255)=255 on:bool=true`},
		{"chart", "display", 200, 120, false, "",
			`kind:enum(line,bar)="line" points:int(2..100)=10 min:int=0 max:int=100 series:list=["Wert"]`},
		{"gauge", "display", 140, 140, false, "",
			`min:int=0 max:int=100 value:int=40 ticks:int(2..41)=11 label_every:int(1..10)=2`},
		{"table", "display", 200, 120, false, "", `columns:int(1..8)=2 rows:int(1..32)=3 cells:list=[]`},
		{"qrcode", "display", 100, 100, false, "", `text:string="https://example.com"`},
		{"tabview", "structure", 300, 200, true, "value_changed",
			`tabs:list=["Tab 1","Tab 2"] bar_position:enum(top,bottom,left,right)="top" bar_size:int(20..80)=40`},
		{"list", "structure", 200, 160, false, "clicked", `items:list=["Eintrag 1","Eintrag 2"]`},
		{"msgbox", "structure", 240, 140, false, "clicked", `title:string="Hinweis" text:text="" buttons:list=["OK"]`},
		{"canvas", "structure", 100, 100, false, "", ``},
		{"statusbar", "composite", 0, 24, false, "long_pressed",
			`title:string="" show_clock:bool=true show_wifi:bool=true`},
	}
	got := WidgetCatalog()
	if len(got) != len(want) {
		t.Fatalf("catalog has %d types, want %d", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		gotRow := row{g.Type, g.Group, g.DefaultW, g.DefaultH, g.Container, strings.Join(g.Events, ","), describeProps(g.Props)}
		if gotRow != w {
			t.Errorf("catalog[%d]:\n got %+v\nwant %+v", i, gotRow, w)
		}
		if g.Events == nil || g.Props == nil {
			t.Errorf("%s: events/props must be empty slices, not nil (JSON [])", g.Type)
		}
		for _, p := range g.Props {
			switch p.Kind {
			case PropString, PropText:
				if p.MaxLen != MaxTextBytes {
					t.Errorf("%s.%s: MaxLen %d, want %d", g.Type, p.Name, p.MaxLen, MaxTextBytes)
				}
			case PropList:
				if p.MaxLen < 1 {
					t.Errorf("%s.%s: list needs an item limit", g.Type, p.Name)
				}
			}
		}
	}
}

func TestWidgetCatalogListLimits(t *testing.T) {
	want := map[string]int{"dropdown.options": 64, "roller.options": 64, "keypad.keys": 64, "chart.series": 8,
		"table.cells": 256, "tabview.tabs": 8, "list.items": 64, "msgbox.buttons": 4}
	for _, s := range WidgetCatalog() {
		for _, p := range s.Props {
			if p.Kind != PropList {
				continue
			}
			key := s.Type + "." + p.Name
			if p.MaxLen != want[key] {
				t.Errorf("%s MaxLen = %d, want %d", key, p.MaxLen, want[key])
			}
			delete(want, key)
		}
	}
	if len(want) > 0 {
		t.Errorf("list props missing: %v", want)
	}
}

func TestWidgetCatalogCopies(t *testing.T) {
	a := WidgetCatalog()
	a[0].Props[0].Name = "mutated"
	a[9].Props[0].Default.([]string)[0] = "mutated"
	a[0].Props[3].Options[0] = "mutated" // label.align
	*a[4].Props[1].Min = 99              // line.thickness
	spec, ok := WidgetSpecFor("label")
	if !ok || spec.Props[0].Name != "text" {
		t.Fatal("WidgetCatalog leaked its cache")
	}
	if spec.Props[3].Options[0] != "left" {
		t.Fatal("WidgetCatalog leaked enum options")
	}
	if line, _ := WidgetSpecFor("line"); *line.Props[1].Min != 1 {
		t.Fatal("WidgetCatalog leaked int bounds")
	}
	spec.Events[0] = "mutated"
	again, _ := WidgetSpecFor("label")
	if again.Events[0] != "clicked" {
		t.Fatal("WidgetSpecFor leaked its cache")
	}
	dd, _ := WidgetSpecFor("dropdown")
	if dd.Props[0].Default.([]string)[0] != "Eins" {
		t.Fatal("list defaults leaked")
	}
	if _, ok := WidgetSpecFor("slider2"); ok {
		t.Fatal("unknown type found")
	}
	encoded, err := json.Marshal(WidgetCatalog())
	if err != nil || !strings.Contains(string(encoded), `"type":"statusbar","group":"composite","default_w":0`) {
		t.Fatalf("catalog JSON: %v %.200s", err, encoded)
	}
}

func TestFontsAndSymbols(t *testing.T) {
	if !reflect.DeepEqual(FontFamilies, []string{"montserrat", "inter", "jetbrains_mono"}) {
		t.Errorf("FontFamilies = %v", FontFamilies)
	}
	if !reflect.DeepEqual(FontSizes, []int{12, 14, 16, 20, 24, 28, 32, 40, 48}) {
		t.Errorf("FontSizes = %v", FontSizes)
	}
	want := "audio video list ok close power settings home download drive refresh mute volume_mid volume_max " +
		"image tint prev play pause stop next eject left right plus minus eye_open eye_close warning shuffle up " +
		"down loop directory upload call cut copy save bars envelope charge paste bell keyboard gps file wifi " +
		"battery_full battery_3 battery_2 battery_1 battery_empty usb bluetooth trash edit backspace sd_card new_line"
	if got := strings.Join(Symbols, " "); got != want {
		t.Errorf("Symbols =\n%s\nwant\n%s", got, want)
	}
	re := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	seen := map[string]bool{}
	for _, s := range Symbols {
		if !re.MatchString(s) || seen[s] {
			t.Errorf("symbol %q invalid or duplicate", s)
		}
		seen[s] = true
	}
	if len(Symbols) != 60 {
		t.Errorf("%d symbols, want 60", len(Symbols))
	}
}

func TestLibrariesPinned(t *testing.T) {
	want := []Library{
		{"arduinojson", "ArduinoJson", "7.4.3", "bblanchon/ArduinoJson@7.4.3", nil},
		{"pubsubclient", "PubSubClient", "2.8", "knolleary/PubSubClient@2.8", nil},
		{"onewire", "OneWire", "2.3.8", "paulstoffregen/OneWire@2.3.8", nil},
		{"dallastemperature", "DallasTemperature", "4.0.6", "milesburton/DallasTemperature@4.0.6",
			[]string{"paulstoffregen/OneWire@2.3.8"}},
		{"dht", "DHT sensor library", "1.4.7", "adafruit/DHT sensor library@1.4.7",
			[]string{"adafruit/Adafruit Unified Sensor@1.1.15"}},
		{"bme280", "Adafruit BME280 Library", "2.3.0", "adafruit/Adafruit BME280 Library@2.3.0",
			[]string{"adafruit/Adafruit BusIO@1.17.4", "adafruit/Adafruit Unified Sensor@1.1.15"}},
		{"bmp280", "Adafruit BMP280 Library", "3.0.0", "adafruit/Adafruit BMP280 Library@3.0.0",
			[]string{"adafruit/Adafruit BusIO@1.17.4", "adafruit/Adafruit Unified Sensor@1.1.15"}},
		{"neopixel", "Adafruit NeoPixel", "1.15.5", "adafruit/Adafruit NeoPixel@1.15.5", nil},
	}
	got := Libraries()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Libraries() =\n%+v\nwant\n%+v", got, want)
	}
	for _, l := range got {
		if !strings.HasSuffix(l.PIOSpec, "@"+l.Version) || strings.ContainsAny(l.PIOSpec, "^~<>") {
			t.Errorf("%s: PIOSpec %q must pin exactly version %s", l.ID, l.PIOSpec, l.Version)
		}
	}
	if _, ok := libraryByID("bme280"); !ok {
		t.Fatal("libraryByID(bme280) not found")
	}
	if _, ok := libraryByID("ArduinoJson"); ok {
		t.Fatal("library ids are lowercase")
	}
}
