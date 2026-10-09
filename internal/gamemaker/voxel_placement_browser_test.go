package gamemaker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestVoxelPlacementWithLockedMouse(t *testing.T) {
	if os.Getenv("GAMEMAKER_VOXEL_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_VOXEL_BROWSER=1")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	for _, mode := range []string{"creative", "survival"} {
		t.Run(mode, func(t *testing.T) {
			s := newTestService(t)
			p, dir, v := voxelFixture(t, s)
			v.Mode, v.Terrain, v.Enemies = mode, "flat", nil
			raw, _ := json.Marshal(v)
			if err := os.WriteFile(filepath.Join(dir, "src", "voxel.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if build := buildDirectory(context.Background(), dir, 100, 32<<20); !build.OK {
				t.Fatal(build.Diagnostics)
			}
			publishExportFixture(t, s, p, dir)
			extracted := t.TempDir()
			for name, data := range readExportFixture(t, s, p) {
				dest, _, err := secureJoin(extracted, name, true)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dest, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, control := range []string{"locked_mouse", "unlocked_mouse", "keyboard", "touch"} {
				t.Run(control, func(t *testing.T) {
					// Each origin starts with empty browser storage and ordinary player input.
					server := httptest.NewServer(http.StripPrefix("/game/", http.FileServer(http.Dir(extracted))))
					defer server.Close()
					page := browser.MustPage("about:blank").Timeout(30 * time.Second)
					defer page.Close()
					page.MustSetViewport(1280, 720, 1, false)
					if control == "touch" {
						page.MustSetViewport(390, 844, 1, false)
						if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
							t.Fatal(err)
						}
					}
					page.MustEvalOnNewDocument(`window.voxel=()=>window.__AURAGO_GAME_TEST__?.observeVoxel();window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)))`)
					page.MustNavigate(server.URL + "/game/").MustWaitLoad()
					page.MustWait(`()=>voxel()?.ready`)
					page.Keyboard.MustType(input.Enter)
					page.MustWait(`()=>!voxel().paused`)
					if mode == "survival" {
						page.Keyboard.Press(input.KeyE)
						page.MustWait(`()=>voxel().metrics.mined>0`)
						page.Keyboard.Release(input.KeyE)
					}
					if control == "locked_mouse" {
						page.MustElement("canvas").MustClick()
						page.MustWait(`()=>document.pointerLockElement===document.querySelector('canvas')`)
					} else if control == "touch" {
						page.Touch.MustTap(190, 260)
						page.MustWait(`()=>!document.querySelector('[data-voxel-actions]').hidden`)
					} else {
						page.Mouse.MustMoveTo(640, 360)
					}
					// Aim at a floor cell outside the player's collision volume.
					page.Keyboard.Press(input.ArrowDown)
					page.MustWait(`()=>voxel().player.pitch<-.6`)
					page.Keyboard.Release(input.ArrowDown)
					page.MustEval(`()=>{const o=voxel();window.beforePlacement=o;window.placementCell=o.target.previous;}`)
					switch control {
					case "keyboard":
						page.Keyboard.MustType(input.KeyF)
					case "touch":
						point := page.MustEval(`()=>{const r=document.querySelector('[data-voxel-action="place"]').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}}`)
						page.Touch.MustTap(point.Get("x").Num(), point.Get("y").Num())
					default:
						page.Mouse.MustClick(proto.InputMouseButtonRight)
					}
					page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
					if !page.MustEval(`()=>{const o=window.__AURAGO_GAME_TEST__.observeVoxel([placementCell]),slot=beforePlacement.inventory[beforePlacement.selected],item=o.items.find(i=>i.id===slot.item),count=s=>s.inventory.reduce((n,e)=>n+(e?.item===item.id?e.count:0),0);return o.metrics.placed===beforePlacement.metrics.placed+1&&o.blocks.some(b=>b.cell.join(',')===placementCell.join(',')&&b.id===item.block)&&count(o)===count(beforePlacement)-(o.mode==='survival'?1:0)&&errors.length===0}`).Bool() {
						t.Fatal("placement did not change the block and inventory correctly:", page.MustEval(`()=>({errors,before:beforePlacement.metrics,after:voxel().metrics,target:voxel().target,locked:!!document.pointerLockElement})`).String())
					}
					if control == "locked_mouse" {
						page.MustEval(`()=>{window.minedBefore=voxel().metrics.mined;}`)
						page.Mouse.MustDown(proto.InputMouseButtonLeft)
						page.MustWait(`()=>voxel().metrics.mined>minedBefore||errors.length>0`)
						page.Mouse.MustUp(proto.InputMouseButtonLeft)
						if !page.MustEval(`()=>errors.length===0&&window.__AURAGO_GAME_TEST__.observeVoxel([placementCell]).blocks.some(b=>b.cell.join(',')===placementCell.join(',')&&b.id===0)`).Bool() {
							t.Fatal("locked left click could not mine the placed block:", page.MustEval(`()=>errors`).String())
						}
					}
				})
			}
		})
	}
}
