package ui

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"sync"
	"testing"
	"time"
)

func customWallpaperBrowserPNG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 24, 16))
	for y := 0; y < canvas.Bounds().Dy(); y++ {
		for x := 0; x < canvas.Bounds().Dx(); x++ {
			canvas.SetRGBA(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 12), B: 0x66, A: 0xff})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		t.Fatalf("encode custom wallpaper fixture: %v", err)
	}
	return encoded.Bytes()
}

func TestDesktopCustomWallpaperBrowser(t *testing.T) {
	oldStarted := make(chan struct{})
	releaseOld := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseOld) }) }
	defer release()
	imageBytes := customWallpaperBrowserPNG(t)
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/wallpaper": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Query().Get("path") {
			case "Pictures/old.png":
				close(oldStarted)
				<-releaseOld
			case "Pictures/missing.png":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": "wallpaper_not_found"})
				return
			case "Pictures/invalid.png":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnsupportedMediaType)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": "wallpaper_format_mismatch"})
				return
			case "Pictures/current.png":
			default:
				http.Error(w, "unknown wallpaper fixture", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "private, no-store")
			_, _ = w.Write(imageBytes)
		}),
	})

	page.MustWait(`()=>!!window.AuraDesktopCustomWallpaper`)
	page.MustEval(`()=>{
        const nativeFetch=window.fetch.bind(window);
        window.fetch=(url,options={})=>{
            const path=new URL(url,location.href).searchParams.get('path');
            // Let this in-flight response arrive despite the runtime aborting it,
            // so the generation guard is exercised against a late network result.
            return nativeFetch(url,path==='Pictures/old.png'?{...options,signal:undefined}:options);
        };
        const body=document.body,wallpaper=window.AuraDesktopCustomWallpaper;
        body.dataset.wallpaper='file:Pictures/old.png';
        window.oldWallpaperLoad=wallpaper.apply(body,body.dataset.wallpaper,'cover',{
            isCurrent:value=>body.dataset.wallpaper===value
        });
    }`)
	select {
	case <-oldStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("old wallpaper request did not start")
	}

	page.MustEval(`async()=>{
        const body=document.body,wallpaper=window.AuraDesktopCustomWallpaper;
        body.dataset.wallpaper='file:Pictures/current.png';
        window.currentWallpaperLoaded=await wallpaper.apply(body,body.dataset.wallpaper,'contain',{
            isCurrent:value=>body.dataset.wallpaper===value
        });
    }`)
	if !page.MustEval(`()=>window.currentWallpaperLoaded===true&&document.body.dataset.customWallpaperState==='ready'
        &&document.body.style.getPropertyValue('--vd-custom-wallpaper-fit')==='contain'
        &&document.body.style.getPropertyValue('--vd-custom-wallpaper-image').includes('blob:')`).Bool() {
		t.Fatal("a real image response should decode, apply, and use the selected contain fit")
	}

	release()
	page.MustEval(`async()=>{window.oldWallpaperResult=await window.oldWallpaperLoad;}`)
	if !page.MustEval(`()=>window.oldWallpaperResult===false&&document.body.dataset.wallpaper==='file:Pictures/current.png'
        &&document.body.style.getPropertyValue('--vd-custom-wallpaper-fit')==='contain'`).Bool() {
		t.Fatal("a late response for the previous workspace wallpaper replaced the current image")
	}

	page.MustEval(`async()=>{
        const body=document.body,wallpaper=window.AuraDesktopCustomWallpaper;
        body.dataset.wallpaper='file:Pictures/missing.png';
        window.missingWallpaperResult=await wallpaper.apply(body,body.dataset.wallpaper,'cover',{
            isCurrent:value=>body.dataset.wallpaper===value
        });
    }`)
	if !page.MustEval(`()=>window.missingWallpaperResult===false&&document.body.dataset.customWallpaperState==='missing'
        &&!document.body.style.getPropertyValue('--vd-custom-wallpaper-image')`).Bool() {
		t.Fatal("missing files should retain their selection while clearing the image and reporting missing status")
	}

	page.MustEval(`async()=>{
        const body=document.body,wallpaper=window.AuraDesktopCustomWallpaper;
        body.dataset.wallpaper='file:Pictures/invalid.png';
        window.invalidWallpaperResult=await wallpaper.apply(body,body.dataset.wallpaper,'cover',{
            isCurrent:value=>body.dataset.wallpaper===value
        });
    }`)
	if !page.MustEval(`()=>window.invalidWallpaperResult===false&&document.body.dataset.customWallpaperState==='invalid'
        &&!document.body.style.getPropertyValue('--vd-custom-wallpaper-image')`).Bool() {
		t.Fatal("invalid files should retain their selection while clearing the image and reporting invalid status")
	}
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatal(got)
	}
}
