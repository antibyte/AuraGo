package ui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/go-rod/rod"
	"github.com/makiuchi-d/gozxing"
	qr "github.com/makiuchi-d/gozxing/qrcode"
)

func TestDesktopSheetsImagesBrowser(t *testing.T) {
	const payload = "https://example.org/AuraGo-office-qr"
	matrix, err := qr.NewQRCodeWriter().EncodeWithoutHint(payload, gozxing.BarcodeFormat_QR_CODE, 210, 210)
	if err != nil {
		t.Fatal(err)
	}
	pixels := image.NewGray(image.Rect(0, 0, matrix.GetWidth(), matrix.GetHeight()))
	for y := 0; y < matrix.GetHeight(); y++ {
		for x := 0; x < matrix.GetWidth(); x++ {
			pixels.Set(x, y, color.White)
			if matrix.Get(x, y) {
				pixels.Set(x, y, color.Black)
			}
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, pixels); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("testdata/desktop-sheets-images.js")
	if err != nil {
		t.Fatal(err)
	}
	testSheetsAppBrowser(t, func(t *testing.T, page *rod.Page) {
		data, err := base64.StdEncoding.DecodeString(page.MustEval(string(source), base64.StdEncoding.EncodeToString(encoded.Bytes())).Str())
		if err != nil {
			t.Fatal(err)
		}
		decodedImage, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		bitmap, err := gozxing.NewBinaryBitmapFromImage(decodedImage)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := qr.NewQRCodeReader().DecodeWithoutHints(bitmap)
		if err != nil || decoded.GetText() != payload {
			t.Fatalf("saved/reopened XLSX QR pixels did not decode: %v", err)
		}
	})
}

func TestDesktopSheetsQRDialogBrowser(t *testing.T) {
	testSheetsAppBrowser(t, func(t *testing.T, page *rod.Page) {
		encoded := page.MustEval(`async()=>{
 const until=async condition=>{for(let i=0;i<400;i++){if(condition())return;await new Promise(resolve=>setTimeout(resolve,25));}throw Error('Real QR dialog timed out');};
 const app=SheetsApp.instances.get('test');if(!app.session)throw Error('No workbook');app.session.suspend();
 app.book.getActiveSheet().setActiveRange(app.book.getActiveSheet().getRange('C5'));
 await app.act('qr');await until(()=>document.querySelector('dialog [data-field="content.text"]'));
 const input=document.querySelector('dialog [data-field="content.text"]');input.value='Real Tabellen QR Grüße 東京';input.dispatchEvent(new Event('input',{bubbles:true}));
 await until(()=>!document.querySelector('dialog [data-action="insert"]').disabled);
 document.querySelector('dialog [data-action="insert"]').click();await until(()=>!document.querySelector('dialog')&&app.state.aux.images.length===1);
 if(app.state.aux.images[0].anchor!=='C5'||app.state.aux.images[0].width!==189)throw Error('QR dialog lost captured cell or physical dimensions');
 await app.api.undo();if(app.state.aux.images.length!==0)throw Error('Real QR insertion undo');await app.api.redo();
 app.session.resume();await app.state.save();await app.state.load(app.path);
 if(app.state.aux.images.length!==1||app.state.aux.images[0].anchor!=='C5')throw Error('Real QR insertion save/reopen');
 const pixels=app.state.aux.images[0].data;SheetsApp.dispose('test');return pixels;
 }`).Str()
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		pixels, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		bitmap, err := gozxing.NewBinaryBitmapFromImage(pixels)
		if err != nil {
			t.Fatal(err)
		}
		result, err := qr.NewQRCodeReader().DecodeWithoutHints(bitmap)
		if err != nil || result.GetText() != "Real Tabellen QR Grüße 東京" {
			t.Fatalf("real QR dialog XLSX embedded pixels: %v", err)
		}
	})
}
