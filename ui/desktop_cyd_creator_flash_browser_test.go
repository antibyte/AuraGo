package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopCYDCreatorFlashBrowser drives the plan 5 modules in the real desktop shell against the
// shell fixture plus testdata/cyd-creator-flash-fixture.js (fake navigator.serial and esptool-js):
// import on the start screen (a refused archive with its reason, then a new project), the desktop
// serial switch (D2), the flash flow with the AuraGo link (device URL refusal, image check before the
// mint, baud fallback, full install), the monitor taking the port over, backtrace decoding, "Attach
// log", a second flash that keeps the board's settings (D1), the OTA dialog with live updates, the
// rollback hint, rename, update target, read-only view and removal, a flash without the link (erased
// config partition), export and the fallback without Web Serial.
func TestDesktopCYDCreatorFlashBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startCYDCreatorSmoke(t, "/testdata/cyd-creator-flash-fixture.js")
	page := s.page
	s.wait(`()=>{const P=window.CYDCreatorParts||{};return !!P.flasher && !!P.devices && !!P.transfer && !!P.monitor}`)
	page.MustEval(`async()=>{await cydFlashFixture.ready;cydFlashFixture.install();}`)
	policy := func(detail string) {
		page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:` + detail + `}))`)
	}
	const toast = `text=>[...document.querySelectorAll('.vd-toast')].some(x=>x.textContent.includes(text))`

	// Import ZIP: a foreign archive is refused with its reason and path; an export opens as a new project.
	s.wait(`()=>{const b=document.querySelector('[data-cc-action="import"]');return !!b && !b.hidden && !b.disabled}`)
	page.MustElement(`[data-cc-action="import"]`).MustClick()
	s.wait(`()=>cydFlashFixture.state.imports[0]==='fremd.zip' && (` + toast + `)(t('cyd_creator.transfer.import_file_type',{path:'secrets/wlan.txt'})) && !!document.querySelector('.cc-home')`)
	s.noRawKeys("import refused")
	page.MustElement(`[data-cc-action="import"]`).MustClick()
	s.wait(`()=>cydFlashFixture.state.imports[1]==='wetter-import.zip' && document.querySelector('[data-cc-project-title]')?.textContent==='Importiert'`)
	s.noRawKeys("imported project")
	page.MustElement(`[data-cc-action="home"]`).MustClick()
	s.wait(`()=>[...document.querySelectorAll('.cc-project-open')].some(b=>b.textContent.includes('Wetterstation'))`)
	page.MustEval(`()=>[...document.querySelectorAll('.cc-project-open')].find(b=>b.textContent.includes('Wetterstation')).click()`)
	s.wait(`()=>document.querySelector('[data-cc-project-title]')?.textContent==='Wetterstation'`)
	s.wait(`()=>{const q=a=>document.querySelector('[data-cc-action="'+a+'"]');return !q('flash').hidden && !q('ota').hidden && !q('export').hidden && !!document.querySelector('[data-cc-dock-tab="monitor"]')}`)

	// D2: while the desktop's serial switch is off the monitor cannot connect and the flash dialog
	// says where to turn it on; the policy event enables both while the dialog stays open.
	page.MustElement(`[data-cc-dock-tab="monitor"]`).MustClick()
	s.wait(`()=>{const c=document.querySelector('[data-cc-monitor-action="connect"]'),n=document.querySelector('[data-cc-monitor-notice]');return !!c && c.disabled && !n.hidden && n.textContent===t('cyd_creator.flash.unsupported_disabled')}`)
	page.MustElement(`[data-cc-action="flash"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-cc-flash-unsupported]')?.dataset.ccFlashUnsupported==='disabled' && document.querySelector('.cc-dialog [data-cc-dialog-action="flash"]').hidden && !document.querySelector('[data-cc-flash-mode]')`)
	s.noRawKeys("flash disabled")
	policy(`{serial_browser_enabled:true,readonly:false}`)
	s.wait(`()=>!document.querySelector('[data-cc-flash-unsupported]') && !document.querySelector('.cc-dialog [data-cc-dialog-action="flash"]').hidden && document.querySelector('[data-cc-flash-mode]')?.value==='auto' && !document.querySelector('[data-cc-monitor-action="connect"]').disabled && document.querySelector('[data-cc-monitor-notice]').hidden`)

	// The newest green build is preselected and has the AuraGo link.
	s.wait(`()=>document.querySelectorAll('[data-cc-flash-build] option').length===2 && document.querySelector('[data-cc-flash-build]').value==='ccb_green_000002' && document.querySelector('[data-cc-flash-link]')?.dataset.ccFlashLink==='on'`)
	s.noRawKeys("flash dialog")
	s.shot("flash-dialog")

	// D4: the server cannot give the board an address. The images were downloaded and checked first;
	// nothing was written.
	page.MustEval(`()=>{cydFlashFixture.state.mintFail={reason:'device_url',hint:'cyd_creator.ota.public_url'};}`)
	page.MustElement(`.cc-dialog [data-cc-dialog-action="flash"]`).MustClick()
	s.wait(`()=>{const e=document.querySelector('.cc-flasher-dialog .cc-dialog-error');return !e.hidden && e.textContent===t('cyd_creator.flash.error_device_url') && document.querySelector('[data-cc-flash-phase]').dataset.ccFlashPhase==='failed' && cydFlashFixture.state.esptool.attempts.length===0}`)
	s.wait(`()=>JSON.stringify(cydFlashFixture.state.calls)===JSON.stringify(['artifact:ccb_green_000002/merged.bin','artifact:ccb_green_000002/firmware.bin','mint:ccb_green_000002'])`)

	// Flash with the AuraGo link on a board this project never flashed: Automatic is a full install
	// (merged.bin at 0x0, the device blob at the board's config offset); 921600 baud fails once,
	// 460800 succeeds; esptool stays in the stub and the monitor resets the board.
	page.MustElement(`.cc-dialog [data-cc-dialog-action="flash"]`).MustClick()
	s.wait(`()=>{const c=document.querySelector('[data-cc-flash-mode-chosen]');return document.querySelector('[data-cc-flash-phase]')?.dataset.ccFlashPhase==='done' && document.querySelector('[data-cc-flash-progress]').value===100 && !c.hidden && c.textContent===t('cyd_creator.flash.mode_chosen_full')}`)
	s.wait(`()=>{const e=cydFlashFixture.state.esptool;return JSON.stringify(e.attempts)==='[921600,460800]' && e.writes.length===1 && e.writes[0].baud===460800 && JSON.stringify(e.writes[0].files)===JSON.stringify([[0,36864,'ccb_green_000002/merged.bin'],[3997696,4096,'blob1']]) && e.after.at(-1)==='no_reset_stub'}`)
	s.wait(`()=>cydFlashFixture.state.mints.length===1 && cydFlashFixture.state.mints[0].build_id==='ccb_green_000002' && cydFlashFixture.state.deletes.length===0`)
	s.noRawKeys("flash done")
	s.shot("flash-done")
	s.wait(`()=>document.querySelector('[data-cc-dock-tab="monitor"]').getAttribute('aria-selected')==='true' && document.querySelector('[data-cc-monitor-status]')?.dataset.ccMonitorStatus==='connected'`)
	s.wait(`()=>!!document.querySelector('.cc-monitor-line.is-backtrace [data-cc-monitor-decode]') && cydFlashFixture.state.port.signals.some(x=>x.requestToSend)`)
	page.MustElement(`.cc-dialog [data-cc-dialog-action="close"]`).MustClick()
	s.wait(`()=>!document.querySelector('.cc-dialog')`)

	// Backtrace decoding uses the build that was just flashed.
	page.MustElement(`.cc-monitor-line.is-backtrace [data-cc-monitor-decode]`).MustClick()
	s.wait(`()=>{const d=cydFlashFixture.state.decodes[0];return !!d && d.build==='ccb_green_000002' && d.addresses.join()==='0x400d1234,0x400d5678,0x40088e2d' && document.querySelector('[data-cc-monitor-frames]')?.textContent.includes('src/app/app_main.cpp:42')}`)
	s.noRawKeys("monitor")
	s.shot("monitor-backtrace")
	page.MustElement(`[data-cc-monitor-action="attach"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-cc-panel="agent"]').getAttribute('aria-selected')==='true' && !!document.querySelector('[data-cc-attachment="log"]')`)
	s.shot("attach-log")
	s.failOnPageErrors("flash and monitor")

	// D1: the same board again. Automatic keeps the settings (this browser's board record and the
	// partition table read back from the board match): firmware.bin at app0, otadata erased (8 KiB
	// 0xFF at 0xE000) and the new device's blob; the board's previous device is revoked.
	page.MustElement(`[data-cc-action="flash"]`).MustClick()
	s.wait(`()=>{const b=document.querySelector('.cc-dialog [data-cc-dialog-action="flash"]');return !!b && !b.hidden && document.querySelector('[data-cc-flash-mode]')?.value==='auto'}`)
	page.MustElement(`.cc-dialog [data-cc-dialog-action="flash"]`).MustClick()
	s.wait(`()=>{const c=document.querySelector('[data-cc-flash-mode-chosen]');return document.querySelector('[data-cc-flash-phase]')?.dataset.ccFlashPhase==='done' && !c.hidden && c.textContent===t('cyd_creator.flash.mode_chosen_keep')}`)
	s.wait(`()=>{const st=cydFlashFixture.state,w=st.esptool.writes;return w.length===2 && w[1].baud===921600 && JSON.stringify(w[1].files)===JSON.stringify([[65536,8192,'ccb_green_000002/firmware.bin'],[57344,8192,'ff'],[3997696,4096,'blob2']]) && st.mints.length===2 && JSON.stringify(st.deletes)==='["ccd_000000000001"]'}`)
	s.wait(`()=>document.querySelector('[data-cc-monitor-status]')?.dataset.ccMonitorStatus==='connected'`)
	page.MustElement(`.cc-dialog [data-cc-dialog-action="close"]`).MustClick()
	s.wait(`()=>!document.querySelector('.cc-dialog')`)
	s.failOnPageErrors("keep settings")

	// OTA dialog: the minted device, a live device_changed update, the rollback hint, rename and a new target.
	page.MustEval(`()=>{cydFixture.inst().caps.ota={enabled:true,public_url:'http://192.168.1.5:8088',insecure:true};}`)
	page.MustElement(`[data-cc-action="ota"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('[data-cc-device]').length===1 && document.querySelector('[data-cc-device]').dataset.ccDevice==='ccd_000000000002' && document.querySelector('[data-cc-device-state]')?.dataset.ccDeviceState==='pending' && !!document.querySelector('[data-cc-devices-insecure]')`)
	page.MustEval(`()=>cydFlashFixture.deviceChanged('ccd_000000000002',{status:'active',online:true,ip:'192.168.1.77',rssi:-58,last_seen:new Date().toISOString(),template_version:'1.0.0',ota_state:'idle'})`)
	s.wait(`()=>document.querySelector('[data-cc-device-state]')?.dataset.ccDeviceState==='online' && document.querySelector('.cc-device-facts').textContent.includes('192.168.1.77')`)
	page.MustEval(`()=>cydFlashFixture.deviceChanged('ccd_000000000002',{ota_state:'rolled_back',last_error:'rolled_back: rolled back from ccb_green_000001'})`)
	s.wait(`()=>{const e=document.querySelector('[data-cc-device-error]')?.textContent||'';return e.startsWith(t('cyd_creator.devices.error_rolled_back')+' (rolled back from ccb_green_000001)') && e.endsWith(t('cyd_creator.devices.rollback_hint'))}`)
	s.noRawKeys("devices")
	s.shot("devices")
	page.MustElement(`[data-cc-device-action="rename"]`).MustClick()
	field := page.MustElement(`[data-cc-device-name-input]`)
	field.MustSelectAllText().MustInput("Küche")
	page.Keyboard.MustType(input.Enter)
	s.wait(`()=>cydFlashFixture.state.patches.some(p=>p.name==='Küche') && document.querySelector('[data-cc-device-name]')?.textContent==='Küche'`)
	page.MustEval(`()=>{const sel=document.querySelector('[data-cc-device-target]');sel.value='ccb_green_000001';sel.dispatchEvent(new Event('change'));}`)
	page.MustElement(`[data-cc-device-action="update"]`).MustClick()
	s.wait(`()=>cydFlashFixture.state.patches.some(p=>p.target_build_id==='ccb_green_000001') && document.querySelector('.cc-device-facts').textContent.includes(t('cyd_creator.devices.target'))`)
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('.cc-dialog')`)

	// A read-only desktop shows the list without actions.
	policy(`{readonly:true}`)
	page.MustElement(`[data-cc-action="ota"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-cc-devices-readonly]') && document.querySelectorAll('[data-cc-device]').length===1 && document.querySelectorAll('[data-cc-device-action]').length===0`)
	s.shot("devices-readonly")
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('.cc-dialog')`)
	policy(`{readonly:false}`)

	// Remove asks first, then revokes.
	page.MustElement(`[data-cc-action="ota"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-cc-device-action="remove"]')`)
	page.MustElement(`[data-cc-device-action="remove"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.cc-dialog').length===2`)
	s.noRawKeys("remove confirm")
	page.MustEval(`()=>[...document.querySelectorAll('.cc-dialog [data-cc-dialog-action="ok"]')].at(-1).click()`)
	s.wait(`()=>JSON.stringify(cydFlashFixture.state.deletes)==='["ccd_000000000001","ccd_000000000002"]' && !!document.querySelector('[data-cc-devices-empty]')`)
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('.cc-dialog')`)

	// A build without the AuraGo link, full install, monitor off: nothing is minted, the config
	// partition is erased (4 KiB 0xFF) so no earlier token survives, and esptool resets the board.
	page.MustElement(`[data-cc-action="flash"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-cc-flash-mode]') && !document.querySelector('.cc-dialog [data-cc-dialog-action="flash"]').hidden`)
	page.MustEval(`()=>{const set=(q,v)=>{const n=document.querySelector(q);n.value=v;n.dispatchEvent(new Event('change'));};set('[data-cc-flash-build]','ccb_green_000001');set('[data-cc-flash-mode]','full');document.querySelector('[data-cc-flash-monitor]').checked=false;}`)
	page.MustElement(`.cc-dialog [data-cc-dialog-action="flash"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-cc-flash-phase]')?.dataset.ccFlashPhase==='done' && document.querySelector('[data-cc-flash-mode-chosen]').textContent===t('cyd_creator.flash.mode_chosen_full')`)
	s.wait(`()=>{const st=cydFlashFixture.state,w=st.esptool.writes;return w.length===3 && JSON.stringify(w[2].files)===JSON.stringify([[0,36864,'ccb_green_000001/merged.bin'],[3997696,4096,'ff']]) && st.mints.length===2 && st.calls.at(-1)==='artifact:ccb_green_000001/merged.bin' && st.esptool.after.at(-1)==='hard_reset'}`)
	page.MustElement(`.cc-dialog [data-cc-dialog-action="close"]`).MustClick()
	s.wait(`()=>!document.querySelector('.cc-dialog')`)
	s.failOnPageErrors("devices")

	// Export downloads the ZIP of GET /projects/{id}/export under the server's file name.
	page.MustElement(`[data-cc-action="export"]`).MustClick()
	s.wait(`()=>{const d=cydFlashFixture.state.downloads;return cydFlashFixture.state.exports===1 && d.length===1 && d[0].name==='wetterstation.zip' && d[0].size===22 && (` + toast + `)(t('cyd_creator.transfer.exported',{name:'wetterstation.zip'}))}`)

	// Without Web Serial the dialog explains why and only offers the image download.
	page.MustEval(`()=>cydFlashFixture.setSerial(false)`)
	page.MustElement(`[data-cc-action="flash"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-cc-flash-unsupported]')?.dataset.ccFlashUnsupported==='unsupported' && document.querySelector('.cc-dialog [data-cc-dialog-action="flash"]').hidden && (document.querySelector('[data-cc-flash-action="download"]')?.getAttribute('href')||'').endsWith('/builds/ccb_green_000002/artifacts/merged.bin')`)
	s.noRawKeys("no web serial")
	s.shot("flash-unsupported")
	page.Keyboard.MustType(input.Escape)
	s.failOnPageErrors("final")
}
