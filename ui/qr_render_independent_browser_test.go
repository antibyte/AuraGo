package ui

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"aurago/internal/qrcode"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Chromium rasterizes the delivered PNG/JPEG/SVG and jsQR independently decodes
// them; neither rendering nor decoding reuses the Go acceptance implementation.
func TestQRCodeIndependentDecoderBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	dependency := os.Getenv("AURAGO_QR_TEST_NODE_MODULES")
	if dependency == "" {
		dependency = "../node_modules"
	}
	decoder, err := os.ReadFile(filepath.Join(dependency, "jsqr", "dist", "jsQR.js"))
	if err != nil {
		t.Fatal("run npm ci for pinned jsQR test dependency: ", err)
	}
	const payload = "Grüße 東京 中文 مرحبا नमस्ते 😀"
	fontProvider := func(ctx context.Context, id string) ([]byte, error) {
		for _, font := range qrcode.Presets().Fonts {
			if font.ID == id && font.Resource != "" {
				return os.ReadFile(filepath.Join("fonts", "qr", font.Resource))
			}
		}
		return nil, fmt.Errorf("unknown test font")
	}
	logo := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			logo.Set(x, y, color.NRGBA{R: 180, G: 40, B: 30, A: 255})
		}
	}
	var logoBytes bytes.Buffer
	if err = png.Encode(&logoBytes, logo); err != nil {
		t.Fatal(err)
	}
	var results []qrcode.Result
	var expected []string
	for _, shape := range []string{"square", "rounded", "extra-rounded", "classy", "classy-rounded", "dots"} {
		for _, format := range []string{"png", "jpeg", "svg"} {
			spec := qrcode.QRSpec{SchemaVersion: 1, Content: qrcode.Content{Type: "text", Text: payload}, Style: qrcode.Style{Size: 640, ErrorCorrection: "H", ModuleShape: shape}, Logo: &qrcode.Logo{Image: qrcode.ImageResource{MIME: "image/png", Data: logoBytes.Bytes()}, SizePercent: 10, Margin: 3, HideBackgroundModules: true}, Frame: &qrcode.Frame{Position: "bottom", Text: payload, FontSize: 22, Font: "sans"}, Output: qrcode.Output{Format: format}}
			result, err := qrcode.Render(context.Background(), spec, qrcode.RenderOptions{FontProvider: fontProvider})
			if err != nil {
				t.Fatalf("%s/%s: %v", shape, format, err)
			}
			results = append(results, result)
			expected = append(expected, payload)
		}
	}
	contact := qrcode.Content{Type: "vcard", VCard: &qrcode.VCardContent{Version: "2.1", FirstName: "Jörg", LastName: "Beispiel", Phone: "+49301234567", Email: "jorg@example.org", Organization: "A; B, GmbH", Note: "line one\nEND:VCARD\nline two", Address: &qrcode.AddressContent{Street: "Straße 1", City: "Berlin", Country: "Deutschland"}}}
	dense, err := qrcode.Render(context.Background(), qrcode.QRSpec{Content: contact, Style: qrcode.Style{Size: 512}, Output: qrcode.Output{Format: "png"}})
	if err != nil {
		t.Fatal(err)
	}
	densePayload, err := qrcode.Payload(contact)
	if err != nil {
		t.Fatal(err)
	}
	results = append(results, dense)
	expected = append(expected, densePayload)
	mux := http.NewServeMux()
	mux.HandleFunc("/jsqr.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Write(decoder)
	})
	mux.HandleFunc("/qr/", func(w http.ResponseWriter, r *http.Request) {
		i, err := strconv.Atoi(r.URL.Path[4:])
		if err != nil || i < 0 || i >= len(results) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", results[i].Metadata.MIME)
		w.Write(results[i].Data)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><script src="/jsqr.js"></script><canvas></canvas>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL).Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	got := page.MustEval(`async(count,expected)=>{for(let i=0;i<count;i++){const blob=await(await fetch('/qr/'+i)).blob();const url=URL.createObjectURL(blob);try{const img=new Image();img.src=url;await img.decode();const canvas=document.querySelector('canvas');canvas.width=img.naturalWidth;canvas.height=img.naturalHeight;const ctx=canvas.getContext('2d');ctx.fillStyle='white';ctx.fillRect(0,0,canvas.width,canvas.height);ctx.drawImage(img,0,0);const pixels=ctx.getImageData(0,0,canvas.width,canvas.height);const result=jsQR(pixels.data,pixels.width,pixels.height);if(!result||result.data!==expected[i])throw Error('Independent decoder failed export '+i+' '+blob.type);}finally{URL.revokeObjectURL(url)}}return count}`, len(results), expected).Int()
	if got != len(results) {
		t.Fatalf("checked %d/%d artifacts", got, len(results))
	}
}
