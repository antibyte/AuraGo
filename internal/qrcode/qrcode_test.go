package qrcode

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func basicSpec() QRSpec {
	return QRSpec{SchemaVersion: 1, Content: Content{Type: "text", Text: "Grüße 東京 中文 مرحبا नमस्ते 😀"}, Style: Style{Size: 512}, Output: Output{Format: "png"}}
}
func testFonts(ctx context.Context, id string) ([]byte, error) {
	for _, f := range Presets().Fonts {
		if f.ID == id && f.Resource != "" {
			return os.ReadFile(filepath.Join("..", "..", "ui", "fonts", "qr", f.Resource))
		}
	}
	return nil, errors.New("missing test font")
}
func testLogo(t *testing.T) ImageResource {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 32, 24))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.NRGBA{R: 170, G: 61, B: 30, A: 255}), image.Point{}, draw.Src)
	draw.Draw(im, image.Rect(12, 4, 20, 20), image.NewUniform(color.White), image.Point{}, draw.Src)
	raw, e := encodePNG(im, MaxResourceBytes)
	if e != nil {
		t.Fatal(e)
	}
	return ImageResource{MIME: "image/png", Data: raw}
}
func assertRenderRoundtrip(t *testing.T, s QRSpec, opts ...RenderOptions) Result {
	t.Helper()
	normalized, _, err := Normalize(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	s = normalized
	want, e := Payload(s.Content)
	if e != nil {
		t.Fatal(e)
	}
	r, e := Render(context.Background(), s, opts...)
	if e != nil {
		t.Fatal(e)
	}
	if !r.Metadata.Verified || len(r.Data) == 0 {
		t.Fatal("unverified or empty result")
	}
	if r.Metadata.Extension == "svg" {
		im, e := rasterizeSVG(context.Background(), r.Data, true)
		if e != nil {
			t.Fatal(e)
		}
		if e = verifyImage(context.Background(), im, want); e != nil {
			t.Fatal(e)
		}
	} else if strings.HasPrefix(r.Metadata.MIME, "image/") {
		got, e := Decode(context.Background(), r.Data)
		if e != nil || got.Text != want {
			t.Fatalf("decoded mismatch/error: %v", e)
		}
	}
	return r
}

func TestRenderUTF8AndCorrectionLevels(t *testing.T) {
	for _, level := range []string{"L", "M", "Q", "H"} {
		for _, format := range []string{"png", "jpeg", "svg"} {
			t.Run(level+"/"+format, func(t *testing.T) {
				s := basicSpec()
				s.Style.ErrorCorrection = level
				s.Output.Format = format
				assertRenderRoundtrip(t, s)
			})
		}
	}
}
func TestRenderAllShapesLogoAndFrames(t *testing.T) {
	catalog := Presets()
	for i, shape := range catalog.ModuleShapes {
		for j, finder := range catalog.FinderShapes {
			t.Run(shape+"/"+finder, func(t *testing.T) {
				s := basicSpec()
				s.Style.Size = 640
				s.Style.ModuleShape = shape
				s.Style.FinderShape = finder
				s.Style.CenterShape = catalog.CenterShapes[(i+j)%len(catalog.CenterShapes)]
				s.Style.ModuleColor = "#12334b"
				s.Style.FinderColor = "#132e24"
				s.Style.CenterColor = "#241d39"
				s.Logo = &Logo{Image: testLogo(t), SizePercent: 12, Margin: 3, HideBackgroundModules: true}
				s.Frame = &Frame{Position: catalog.FramePositions[(i+j)%4], Text: "Grüße 東京 中文\nمرحبا नमस्ते 😀", Font: "sans", FontSize: 20, SideWidth: 200, BorderWidth: 2, BorderRadius: 14, Padding: 16}
				if i%2 == 0 {
					s.Output.Format = "svg"
				} else {
					s.Output.Format = "jpeg"
				}
				r := assertRenderRoundtrip(t, s, RenderOptions{FontProvider: testFonts})
				if r.Metadata.Width == r.Metadata.Height {
					t.Fatal("framed output should be rectangular")
				}
			})
		}
	}
}
func TestRenderTransparencyAndBackgroundImage(t *testing.T) {
	s := basicSpec()
	s.Style.BackgroundColor = "#ffffff00"
	s.Style.BackgroundRadius = 20
	s.Logo = &Logo{Image: testLogo(t), SizePercent: 10, HideBackgroundModules: false}
	r := assertRenderRoundtrip(t, s)
	im, _, e := decodeRaster(context.Background(), r.Data)
	if e != nil {
		t.Fatal(e)
	}
	_, _, _, alpha := im.At(0, 0).RGBA()
	if alpha != 0 {
		t.Fatal("PNG alpha was flattened")
	}
	s.Frame = &Frame{Text: "QR code", BackgroundColor: "#fff", BackgroundImage: ptrResource(testLogo(t)), BorderWidth: 2, BorderRadius: 10}
	s.Output.Format = "svg"
	assertRenderRoundtrip(t, s)
	s.Output.Format = "jpeg"
	s.Output.JPEGBackground = "#ffffff"
	assertRenderRoundtrip(t, s)
}
func ptrResource(r ImageResource) *ImageResource { return &r }
func TestRenderLocalFontCatalogAndWrapping(t *testing.T) {
	for _, f := range Presets().Fonts {
		t.Run(f.ID, func(t *testing.T) {
			s := basicSpec()
			s.Frame = &Frame{Text: "Grüße 東京 中文 مرحبا नमस्ते 😀\n" + strings.Repeat("A long caption wraps safely. ", 7), Font: f.ID, Position: "bottom", FontSize: 16}
			assertRenderRoundtrip(t, s, RenderOptions{FontProvider: testFonts})
		})
	}
	s := basicSpec()
	s.Frame = &Frame{Text: "東京"}
	if _, e := Render(context.Background(), s); e == nil {
		t.Fatal("missing local font was silently replaced")
	}
}
func TestFontResourcesHavePinnedProvenance(t *testing.T) {
	raw, e := os.ReadFile(filepath.Join("..", "..", "ui", "fonts", "qr", "manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var m struct {
		Fonts []struct {
			ID, File, SHA256, Licence, URL string
			Bytes                          int
		}
	}
	if e = json.Unmarshal(raw, &m); e != nil {
		t.Fatal(e)
	}
	for _, f := range m.Fonts {
		b, e := os.ReadFile(filepath.Join("..", "..", "ui", "fonts", "qr", f.File))
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(b)
		if len(b) != f.Bytes || hex.EncodeToString(sum[:]) != f.SHA256 || !strings.HasPrefix(f.URL, "https://") {
			t.Fatalf("bad provenance for %s", f.ID)
		}
		license, e := os.ReadFile(filepath.Join("..", "..", "ui", "fonts", "qr", f.Licence))
		if e != nil || !bytes.Contains(license, []byte("SIL OPEN FONT LICENSE")) {
			t.Fatalf("missing OFL for %s", f.ID)
		}
	}
}

func TestPayloadAllTemplates(t *testing.T) {
	templates := []Content{{Type: "text", Text: "Grüße 東京 中文 مرحبا नमस्ते 😀"}, {Type: "url", URL: "https://example.org/a?x=1&text=Grüße#part"}, {Type: "email", Email: &EmailContent{To: []string{"a+tag@example.org", "b@example.org"}, Subject: "Grüße & news?", Body: "Line one\nLine two", CC: []string{"c@example.org"}, BCC: []string{"d@example.org"}}}, {Type: "telephone", Telephone: &TelephoneContent{Number: "+49 (30) 1234567"}}, {Type: "sms", SMS: &SMSContent{Number: "+49301234567", Body: "Grüße & test"}}, {Type: "wifi", WiFi: &WiFiContent{SSID: "Net;\\\"名", Password: "password;123", Security: "WPA", Hidden: true}}, {Type: "location", Location: &LocationContent{Latitude: 52.52, Longitude: 13.405}}, {Type: "calendar", Calendar: &CalendarContent{Title: "Grüße; planning", Start: "2026-10-26T10:00", End: "2026-10-26T11:00", Timezone: "Europe/Berlin", Description: "one\ntwo", Location: "Berlin"}}, {Type: "epc", EPC: &EPCContent{Version: "002", Name: "Grüße GmbH", IBAN: "DE89 3704 0044 0532 0130 00", Amount: "12.3", Reference: "RF18539007547034"}}}
	for _, version := range []string{"2.1", "3.0", "4.0"} {
		templates = append(templates, Content{Type: "vcard", VCard: &VCardContent{Version: version, FirstName: "Jörg", LastName: "Beispiel", Phone: "+49301234567", Email: "jorg@example.org", Organization: "A; B, GmbH", Note: "line one\nEND:VCARD\nline two", Address: &AddressContent{Street: "Straße 1", City: "Berlin", Country: "Deutschland"}}})
	}
	for _, content := range templates {
		label := content.Type
		if content.VCard != nil {
			label += "/" + content.VCard.Version
		}
		t.Run(label, func(t *testing.T) {
			s := basicSpec()
			s.Content = content
			result := assertRenderRoundtrip(t, s)
			scan, err := Decode(context.Background(), result.Data)
			if err != nil || scan.Type != content.Type || scan.Content == nil {
				t.Fatalf("typed classification failed: %s %v", scan.Type, err)
			}
		})
	}
}
func TestPayloadEscapingAndEPCStandards(t *testing.T) {
	mail, e := Payload(Content{Type: "email", Email: &EmailContent{To: []string{"a@example.org"}, Body: "a\nb", Subject: "a&b"}})
	if e != nil || !strings.Contains(mail, "body=a%0D%0Ab") || !strings.Contains(mail, "subject=a%26b") {
		t.Fatal("mailto escaping")
	}
	wifi, e := Payload(Content{Type: "wifi", WiFi: &WiFiContent{SSID: "ABCD", Security: "nopass"}})
	if e != nil || wifi != "WIFI:T:nopass;S:\"ABCD\";;" {
		t.Fatal("WiFi hex SSID quoting")
	}
	c := Content{Type: "epc", EPC: &EPCContent{Version: "001", BIC: "COBADEFFXXX", Name: "Example", IBAN: "DE89370400440532013000", Amount: "999999999.99"}}
	p, e := Payload(c)
	if e != nil || !strings.Contains(p, "EUR999999999.99") || strings.HasSuffix(p, "\n") {
		t.Fatal("EPC exact decimal or terminator")
	}
	s := basicSpec()
	s.Content = c
	s.Style.ErrorCorrection = "H"
	if _, _, e = Normalize(context.Background(), s); e == nil {
		t.Fatal("EPC correction was silently changed")
	}
	for _, amount := range []string{"0", "0.00", "-1", "1,23", "1e3", "01.20", "1.234", "1000000000"} {
		v := *c.EPC
		v.Amount = amount
		if _, e := Payload(Content{Type: "epc", EPC: &v}); e == nil {
			t.Fatalf("accepted invalid amount %q", amount)
		}
	}
	v := *c.EPC
	v.IBAN = "DE89370400440532013001"
	if _, e := Payload(Content{Type: "epc", EPC: &v}); e == nil {
		t.Fatal("IBAN checksum ignored")
	}
	v = *c.EPC
	v.Reference = "RF00539007547034"
	if _, e := Payload(Content{Type: "epc", EPC: &v}); e == nil {
		t.Fatal("RF checksum ignored")
	}
}
func TestPayloadCalendarTimezoneAndInjection(t *testing.T) {
	base := CalendarContent{CreatedAt: "2026-10-26T08:00:00Z", Title: "Title\nEND:VEVENT\nURL:javascript:bad", Start: "2026-10-26T10:00", End: "2026-10-26T11:00", Timezone: "Europe/Berlin"}
	p, e := Payload(Content{Type: "calendar", Calendar: &base})
	if e != nil || strings.Count(p, "\r\nEND:VEVENT") != 1 || !strings.Contains(p, "DTSTART:20261026T090000Z") {
		t.Fatal("calendar escaping/timezone")
	}
	for _, start := range []string{"2026-03-29T02:30", "2026-10-25T02:30"} {
		c := base
		c.Start = start
		c.End = "2026-10-26T11:00"
		if _, e := Payload(Content{Type: "calendar", Calendar: &c}); e == nil {
			t.Fatalf("invalid/ambiguous DST time accepted: %s", start)
		}
	}
	c := base
	c.AllDay = true
	c.Start = "2026-10-26"
	c.End = "2026-10-27"
	p, e = Payload(Content{Type: "calendar", Calendar: &c})
	if e != nil || !strings.Contains(p, "DTEND;VALUE=DATE:20261027") {
		t.Fatal("all-day exclusive end")
	}
}
func TestPayloadRejectsConflictsAndInjection(t *testing.T) {
	cases := []Content{{Type: "text", Text: "secret", URL: "https://example.org"}, {Type: "url", URL: "javascript:bad"}, {Type: "email", Email: &EmailContent{To: []string{"a@example.org\nBcc:bad@example.org"}}}, {Type: "email", Email: &EmailContent{To: []string{"a@example.org"}, Subject: "x\r\nBcc:bad@example.org"}}, {Type: "wifi", WiFi: &WiFiContent{SSID: "x", Security: "nopass", Password: "secret"}}, {Type: "epc", EPC: &EPCContent{Name: "x\nSCT", IBAN: "DE89370400440532013000"}}}
	for _, c := range cases {
		if _, e := Payload(c); e == nil {
			t.Fatalf("invalid content %s was accepted", c.Type)
		}
	}
}

func TestConfigRoundtripAndStrictLimits(t *testing.T) {
	s := basicSpec()
	s.Logo = &Logo{Image: testLogo(t), SizePercent: 12}
	s.Frame = &Frame{Text: "Caption", BackgroundImage: ptrResource(testLogo(t))}
	raw, e := ExportConfig(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	loaded, e := ParseConfig(context.Background(), raw)
	if e != nil {
		t.Fatal(e)
	}
	second, e := ExportConfig(context.Background(), loaded)
	if e != nil || !bytes.Equal(raw, second) {
		t.Fatal("normalized config did not reproduce")
	}
	for _, raw := range []string{`{"schema_version":1,"schema_version":1}`, `{"schema_version":1,"content":{"type":"text","text":"x"},"surprise":true}`, `{"schema_version":2}`, `{"schema_version":1} {}`, strings.Repeat("[", 20) + "0" + strings.Repeat("]", 20)} {
		if _, e := ParseConfig(context.Background(), []byte(raw)); e == nil {
			t.Fatal("unsafe config accepted")
		}
	}
	s = basicSpec()
	s.Style.Size = 4097
	if _, e := Render(context.Background(), s); e == nil {
		t.Fatal("output bound ignored")
	}
	s = basicSpec()
	s.Content.Text = strings.Repeat("x", 8000)
	if _, e := Render(context.Background(), s); e == nil {
		t.Fatal("capacity ignored")
	}
	s = basicSpec()
	s.Style.ModuleColor = "#ffffff"
	if r, e := Render(context.Background(), s); e == nil || len(r.Data) != 0 {
		t.Fatal("unreadable QR succeeded")
	}
}
func TestRenderTextExports(t *testing.T) {
	for _, format := range []string{"ascii", "unicode", "unicode_half", "txt", "markdown"} {
		s := basicSpec()
		s.Output.Format = format
		r, e := Render(context.Background(), s)
		if e != nil || !r.Metadata.Verified || !strings.Contains(string(r.Data), "\n") {
			t.Fatalf("text output %s: %v", format, e)
		}
		if format == "ascii" && bytes.ContainsAny(r.Data, "█▀▄") {
			t.Fatal("ASCII contains Unicode art")
		}
		if format == "markdown" && !bytes.HasPrefix(r.Data, []byte("```text\n")) {
			t.Fatal("missing Markdown fence")
		}
	}
}
func TestNormalizeImageRestrictedSVGAndLimits(t *testing.T) {
	valid := `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="48" viewBox="0 0 64 48"><g transform="translate(2 2)"><path fill="#17324d" d="M0 0h60v44h-60z"/><circle cx="30" cy="22" r="10" fill="#ffffff"/><path fill="#000000" d="M10 10 A8 6 20 0 1 30 20 L20 30Z"/></g></svg>`
	resource, e := NormalizeImage(context.Background(), "image/svg+xml", []byte(valid))
	if e != nil || resource.MIME != "image/png" {
		t.Fatal(e)
	}
	for _, bad := range []string{`<svg xmlns="http://www.w3.org/2000/svg" onload="bad()"/>`, `<svg><script>bad()</script></svg>`, `<svg><foreignObject/></svg>`, `<svg><image href="http://example.org/x"/></svg>`, `<!DOCTYPE svg [<!ENTITY x "bad">]><svg>&x;</svg>`, `<svg width="999999999" height="99999999"/>`, `<svg width="10" height="10"><path d="M NaN 0"/></svg>`, `<svg width="10" height="10"><path style="fill:url(http://example.org)"/></svg>`, `<svg width="10" height="10"><use href="#x"/></svg>`} {
		if _, e := NormalizeImage(context.Background(), "image/svg+xml", []byte(bad)); e == nil {
			t.Fatal("unsafe SVG accepted")
		}
	}
	if _, e := NormalizeImage(context.Background(), "image/svg+xml", testLogo(t).Data); e == nil {
		t.Fatal("MIME mismatch accepted")
	}
	if _, e := Decode(context.Background(), make([]byte, MaxResourceBytes+1)); e == nil {
		t.Fatal("scan byte limit ignored")
	}
}
func TestBatchCSVZIPPreviewAndFailures(t *testing.T) {
	s := basicSpec()
	s.Content = Content{Type: "url", URL: "https://example.org"}
	data := []byte("url,caption,filename,font\r\nhttps://example.org/1,\"line one\nline two\",first,sans\r\nhttps://example.org/2,Grüße,second,mono\r\n")
	preview, e := ValidateBatch(context.Background(), data, s, BatchOptions{ContentType: "url"})
	if e != nil || len(preview.Errors) > 0 || len(preview.Entries) != 2 || len(preview.Data) > 0 || preview.Entries[0].Preview == nil {
		t.Fatalf("preview: %+v %v", preview, e)
	}
	batch, e := Batch(context.Background(), data, s, BatchOptions{ContentType: "url"})
	if e != nil || len(batch.Entries) != 2 || len(batch.Data) == 0 {
		t.Fatal(e)
	}
	archive, e := zip.NewReader(bytes.NewReader(batch.Data), int64(len(batch.Data)))
	if e != nil || len(archive.File) != 2 {
		t.Fatal(e)
	}
	for _, entry := range archive.File {
		f, e := entry.Open()
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		_, e = b.ReadFrom(f)
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
		if _, e := Decode(context.Background(), b.Bytes()); e != nil {
			t.Fatal(e)
		}
	}
	for _, bad := range []string{"url,filename\nhttps://example.org,../escape\n", "url,filename\nhttps://example.org,CON\n", "url,filename\nhttps://example.org,Dup\nhttps://example.org,dup\n", "url\nnot a url\n"} {
		result, e := Batch(context.Background(), []byte(bad), s, BatchOptions{ContentType: "url"})
		if !errors.Is(e, ErrBatchInvalid) || len(result.Data) != 0 || len(result.Errors) == 0 {
			t.Fatal("invalid batch produced an archive")
		}
	}
	cards := []byte("first_name,last_name,email,filename\nJörg,Beispiel,jorg@example.org,contact\n")
	if _, e := Batch(context.Background(), cards, s, BatchOptions{ContentType: "vcard"}); e != nil {
		t.Fatal(e)
	}
}
func TestCancellationAndNoSensitiveErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := basicSpec()
	if _, e := Render(ctx, s); !errors.Is(e, context.Canceled) {
		t.Fatal("render ignored cancellation")
	}
	if _, e := Batch(ctx, []byte("text\nx\n"), s, BatchOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal("batch ignored cancellation")
	}
	secret := "sensitive-password-123"
	s.Content = Content{Type: "wifi", WiFi: &WiFiContent{SSID: "test", Password: secret, Security: "invalid"}}
	_, e := Render(context.Background(), s)
	if e == nil || strings.Contains(e.Error(), secret) {
		t.Fatal("error leaked payload")
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, e := Decode(ctx, testLogo(t).Data); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal("scan ignored deadline")
	}
}

func FuzzParseConfig(f *testing.F) {
	f.Add([]byte(`{"schema_version":1,"content":{"type":"text","text":"hello"}}`))
	f.Add([]byte(`{"schema_version":1,"schema_version":2}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = ParseConfig(context.Background(), b)
	})
}
func FuzzPayload(f *testing.F) {
	f.Add("Grüße 東京", "WPA")
	f.Add("x;\\\n", "nopass")
	f.Fuzz(func(t *testing.T, text, security string) {
		if len(text) > 32768 || len(security) > 128 {
			t.Skip()
		}
		_, _ = Payload(Content{Type: "text", Text: text})
		_, _ = Payload(Content{Type: "wifi", WiFi: &WiFiContent{SSID: text, Security: security}})
	})
}
func FuzzBoundedCSV(f *testing.F) {
	f.Add([]byte("text,filename\nhello,a\n"))
	f.Add([]byte("url\n\"bad quote\n"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _, _ = parseCSVRows(context.Background(), b, basicSpec(), BatchOptions{ContentType: "text"})
	})
}

func TestEPCNonEEABICAndLegacyVCardEncoding(t *testing.T) {
	c := Content{Type: "epc", EPC: &EPCContent{Version: "002", Name: "Example", IBAN: "CH9300762011623852957", Amount: "1"}}
	if _, err := Payload(c); err == nil {
		t.Fatal("non-EEA account requires BIC")
	}
	c.EPC.BIC = "POFICHBEXXX"
	if _, err := Payload(c); err != nil {
		t.Fatal(err)
	}
	card := Content{Type: "vcard", VCard: &VCardContent{Version: "2.1", FirstName: "Jörg", LastName: "Example", Note: "one\ntwo"}}
	payload, err := Payload(card)
	if err != nil || !strings.Contains(payload, "CHARSET=UTF-8;ENCODING=QUOTED-PRINTABLE:") || !strings.Contains(payload, "J=C3=B6rg") || !strings.Contains(payload, "one=0D=0Atwo") {
		t.Fatal("legacy vCard encoding")
	}
	scan := Classify(payload)
	if scan.Content == nil || scan.Content.VCard.FirstName != "Jörg" || scan.Content.VCard.Note != "one\r\ntwo" {
		t.Fatal("legacy vCard fields did not decode")
	}
}
func TestSVGDownscaleAndArchiveReservedNames(t *testing.T) {
	raw := []byte(`<svg width="4096" height="1024"><rect x="2048" width="2048" height="1024" fill="#000000"/></svg>`)
	resource, err := NormalizeImage(context.Background(), "image/svg+xml", raw)
	if err != nil {
		t.Fatal(err)
	}
	im, _, err := decodeRaster(context.Background(), resource.Data)
	if err != nil || im.Bounds().Dx() != 2048 || im.Bounds().Dy() != 512 {
		t.Fatal("SVG normalization bounds")
	}
	_, _, _, a := im.At(1536, 200).RGBA()
	if a == 0 {
		t.Fatal("SVG geometry did not scale with its viewport")
	}
	for _, name := range []string{"CON .png", "LPT¹", "normal\u202etxt", "../escape"} {
		if _, err := safeArchiveName(name, "png"); err == nil {
			t.Fatal("unsafe archive name accepted")
		}
	}
	for _, svg := range []string{`<svg width="10" height="10" id="" id="x"/>`, `<svg width="10" height="10"><g opacity="0.5"><rect width="10" height="10"/></g></svg>`} {
		if _, err := NormalizeImage(context.Background(), "image/svg+xml", []byte(svg)); err == nil {
			t.Fatal("unsupported SVG attributes accepted")
		}
	}
}
func FuzzClassify(f *testing.F) {
	f.Add("WIFI:T:WPA;S:Network;P:password;;")
	f.Add("BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Name\r\nEND:VCARD")
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 32768 {
			t.Skip()
		}
		_ = Classify(s)
	})
}

func TestPhoneSubscribersAndMultipleVCardNumbers(t *testing.T) {
	for _, number := range []string{"+1", "030 1234567", "7042;phone-context=pbx.example.org;ext=9", "863-1234;phone-context=+1-914-555", "+49(30)12345;ext=(12)", "*123#;phone-context=pbx.example.org"} {
		for _, kind := range []string{"telephone", "sms"} {
			s := basicSpec()
			if kind == "telephone" {
				s.Content = Content{Type: kind, Telephone: &TelephoneContent{Number: number}}
			} else {
				s.Content = Content{Type: kind, SMS: &SMSContent{Number: number, Body: "Hello & Grüße"}}
			}
			rendered := assertRenderRoundtrip(t, s)
			scan, err := Decode(context.Background(), rendered.Data)
			if err != nil || scan.Content == nil || scan.Type != kind {
				t.Fatalf("subscriber roundtrip %s: %v", kind, err)
			}
		}
	}
	for _, bad := range []string{"+", "+1?body=bad", "123;phone-context=evil/host", "123;ext=1;EXT=2", "123;phone-context=bad_domain", "123;isub=unsupported", "123\nEND:VCARD", "123;phone-context=example.org;other=x"} {
		if err := validPhone(bad); err == nil {
			t.Fatal("invalid phone subscriber accepted")
		}
	}
	s := basicSpec()
	s.Content = Content{Type: "telephone", Telephone: &TelephoneContent{Number: "030123"}}
	_, diagnostics, err := Normalize(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range diagnostics {
		found = found || d.Code == "local_phone"
	}
	if !found {
		t.Fatal("missing local phone warning")
	}
	for _, version := range []string{"2.1", "3.0", "4.0"} {
		v := VCardContent{Version: version, FirstName: "Jörg", LastName: "Example", Phone: "+49301234567", PhoneWork: "7042;phone-context=pbx.example.org;ext=9", PhonePrivate: "03012345", PhoneMobile: "+49170123456"}
		payload, err := Payload(Content{Type: "vcard", VCard: &v})
		if err != nil {
			t.Fatal(err)
		}
		scan := Classify(payload)
		if scan.Content == nil {
			t.Fatal("multiple TEL properties were lost")
		}
		got := scan.Content.VCard
		if got.Phone != compactPhone(v.Phone) || got.PhoneWork != compactPhone(v.PhoneWork) || got.PhonePrivate != compactPhone(v.PhonePrivate) || got.PhoneMobile != compactPhone(v.PhoneMobile) {
			t.Fatal("vCard TEL kinds did not roundtrip")
		}
		if version == "4.0" && !strings.Contains(payload, "TEL;TYPE=home,voice;VALUE=text:03012345") {
			t.Fatal("vCard4 local text number is not version-correct")
		}
	}
	batch, err := Batch(context.Background(), []byte("first_name,phone_work,phone_private,phone_mobile\nJörg,+4930123456,03012345,+49170123456\n"), basicSpec(), BatchOptions{ContentType: "vcard"})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Entries) != 1 {
		t.Fatal("vCard CSV phones")
	}
}
func TestScannerPreservesLiteralEqualsAndColon(t *testing.T) {
	scan := Classify("BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Name\r\nNOTE:Value=\r\n continued\r\nEND:VCARD")
	if scan.Content == nil || scan.Content.VCard.Note != "Value=continued" {
		t.Fatal("vCard folding removed a literal equals")
	}
	sms := Classify("SMSTO:030123:one:two")
	if sms.Content == nil || sms.Content.SMS.Body != "one:two" {
		t.Fatal("SMS colons lost")
	}
}

func TestConcurrentFontRenderAndCancelledAdmission(t *testing.T) {
	var wg sync.WaitGroup
	failures := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := basicSpec()
			s.Frame = &Frame{Text: "Grüße 東京 مرحبا नमस्ते 😀", FontSize: 16}
			r, err := Render(context.Background(), s, RenderOptions{FontProvider: testFonts})
			if err != nil {
				failures <- err
			} else if !r.Metadata.Verified {
				failures <- errors.New("unverified concurrent output")
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	for i := 0; i < cap(renderAdmission); i++ {
		renderAdmission <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Render(ctx, basicSpec())
	for i := 0; i < cap(renderAdmission); i++ {
		<-renderAdmission
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatal("waiting renderer ignored cancellation")
	}
}

func TestExplicitZeroPaddingAndFrameRadiusBounds(t *testing.T) {
	s := basicSpec()
	s.Frame = &Frame{Text: "Caption", Padding: 0}
	normalized, _, err := Normalize(context.Background(), s)
	if err != nil || normalized.Frame.Padding != 0 {
		t.Fatal("explicit zero frame padding was changed")
	}
	assertRenderRoundtrip(t, s)
	s.Frame.BorderRadius = 2048
	if _, err := Render(context.Background(), s); err == nil {
		t.Fatal("oversized frame radius was silently clamped")
	}
}

func TestCalendarCreationTimeIsStableAndExported(t *testing.T) {
	s := basicSpec()
	s.Content = Content{Type: "calendar", Calendar: &CalendarContent{Title: "Event", Start: "2026-10-27T09:00:00Z", End: "2026-10-27T10:00:00Z"}}
	if _, err := Payload(s.Content); err == nil {
		t.Fatal("uninitialized calendar payload accepted")
	}
	normalized, _, err := Normalize(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if s.Content.Calendar.CreatedAt != "" || normalized.Content.Calendar.CreatedAt == "" {
		t.Fatal("normalization mutated the caller or omitted creation time")
	}
	payload, err := Payload(normalized.Content)
	if err != nil {
		t.Fatal(err)
	}
	again, _, err := Normalize(context.Background(), normalized)
	if err != nil || again.Content.Calendar.CreatedAt != normalized.Content.Calendar.CreatedAt {
		t.Fatal("creation time changed on repeat normalization")
	}
	raw, err := ExportConfig(context.Background(), normalized)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := ParseConfig(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	reproduced, err := Payload(loaded.Content)
	if err != nil || reproduced != payload {
		t.Fatal("calendar export changed the encoded payload")
	}
	scan := Classify(payload)
	if scan.Content == nil || scan.Content.Calendar.CreatedAt != normalized.Content.Calendar.CreatedAt {
		t.Fatal("scanner lost creation timestamp")
	}
	assertRenderRoundtrip(t, loaded)
}
