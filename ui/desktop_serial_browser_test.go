package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

func TestQuickConnectSerialBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)

	t.Run("chooser cancellation and denial", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		result := page.MustEval(`async()=>{
            const app=serialTest.app;
            serialTest.requestPort=()=>Promise.reject(new DOMException('cancelled','NotFoundError'));
            await app.connectProfile('browser');
            const cancelled=document.querySelector('[data-serial-status]');
            const cancelState=cancelled.dataset.state, cancelText=cancelled.textContent;
            serialTest.cancelSnapshot={state:cancelState,text:cancelText};
            serialTest.requestPort=()=>Promise.reject(new DOMException('denied','NotAllowedError'));
            await app.connectProfile('browser');
            return {
                cancelState, cancelText,
                deniedState:cancelled.dataset.state,deniedText:cancelled.textContent,
                opens:serialTest.ports.reduce((n,p)=>n+p.openCount,0),
                starts:serialTest.sessionStarts,
                filters:serialTest.lastChooserOptions?.filters || []
            };
		}`)
		if result.Get("deniedState").Str() != "error" || result.Get("opens").Int() != 0 || result.Get("starts").Int() != 0 {
			t.Fatalf("chooser denial opened a port or started a session: %s", result.JSON("", ""))
		}
		filters := result.Get("filters").Get("0")
		if filters.Get("usbVendorId").Int() != 0x10C4 || filters.Get("usbProductId").Int() != 0xEA60 {
			t.Fatalf("saved USB filter was not sent to the chooser: %s", result.JSON("", ""))
		}
		if result.Get("deniedText").Str() != "desktop.qc_serial_browser_permission_denied" {
			t.Fatalf("permission denial did not show the open error: %s", result.JSON("", ""))
		}
		cancel := result.Get("cancelText").Str()
		if cancel != "desktop.qc_serial_cancelled" || result.Get("cancelState").Str() != "ready" {
			t.Fatalf("chooser cancellation was not reported as a recoverable state: %s", result.JSON("", ""))
		}
	})

	t.Run("binary receive, strict hex send, line endings and export", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		result := page.MustEval(`async()=>{
            const app=serialTest.app, port=serialTest.makePort();
            serialTest.requestPort=()=>Promise.resolve(port);
            await app.connectProfile('browser');
            await serialTest.wait(()=>app.connected && document.querySelector('[data-serial-status]')?.dataset.state==='connected');
            const open=port.openOptions;
            const initialSignals=port.signals.slice();
            const requestsBeforeRX=serialTest.requests.length, startsBeforeRX=serialTest.sessionStarts;
            const rxMode=document.querySelector('[data-serial-rx-mode]');let terminal=serialTest.terminals.at(-1);
            rxMode.value='text';rxMode.dispatchEvent(new Event('change',{bubbles:true}));
            terminal=serialTest.terminals.at(-1);
            const writesBeforeRX=terminal.writeCalls;
            port.inject([0xE2,0x82]);
            await serialTest.wait(()=>terminal.writeCalls>writesBeforeRX);
            port.inject([0xAC]);
            await serialTest.wait(()=>terminal.display.includes('€'));
            const splitUTF8Display=terminal.display;
            rxMode.value='hex';rxMode.dispatchEvent(new Event('change',{bubbles:true}));
            terminal=serialTest.terminals.at(-1);
            port.inject([0,0x80,0xFF]);
            await serialTest.wait(()=>serialTest.terminals.at(-1)?.display.includes('00 80 FF'));
            const rxDidNotRenew=requestsBeforeRX===serialTest.requests.length&&startsBeforeRX===serialTest.sessionStarts;

            const mode=document.querySelector('[data-serial-send-mode]');
            const input=document.querySelector('[data-serial-input]');
            const form=document.querySelector('[data-serial-form]');
            mode.value='hex';mode.dispatchEvent(new Event('change',{bubbles:true}));
            input.value='AA 0G';form.requestSubmit();
            await Promise.resolve();
            const invalidWrites=port.writes.length;
            input.value='AA 00 ff';form.requestSubmit();
            await serialTest.wait(()=>port.writes.length===1);
            mode.value='text';mode.dispatchEvent(new Event('change',{bubbles:true}));
            document.querySelector('[data-serial-line-ending]').value='crlf';
            input.value='hello';form.requestSubmit();
            await serialTest.wait(()=>port.writes.length===2);

            const originalCreate=URL.createObjectURL, originalClick=HTMLAnchorElement.prototype.click;
            URL.createObjectURL=blob=>{serialTest.exportBlob=blob;return 'blob:serial-test';};
            HTMLAnchorElement.prototype.click=function(){serialTest.download=this.download;};
            try { document.querySelector('[data-serial-export]').click(); }
            finally { URL.createObjectURL=originalCreate;HTMLAnchorElement.prototype.click=originalClick; }
            const exported=await serialTest.exportBlob.text();
            return {open,initialSignals,splitUTF8Display,display:serialTest.terminals.at(-1).display,invalidWrites,writes:port.writes.map(x=>Array.from(x)),exported,download:serialTest.download,rxDidNotRenew};
        }`)
		if result.Get("invalidWrites").Int() != 0 {
			t.Fatalf("invalid hex reached the port: %s", result.JSON("", ""))
		}
		writes := result.Get("writes")
		if len(writes.Arr()) != 2 || writes.Get("0").JSON("", "") != "[170,0,255]" || writes.Get("1").JSON("", "") != "[104,101,108,108,111,13,10]" {
			t.Fatalf("incorrect raw TX bytes or CRLF ending: %s", result.JSON("", ""))
		}
		if !strings.Contains(result.Get("splitUTF8Display").Str(), "€") || !strings.Contains(result.Get("display").Str(), "00 80 FF") {
			t.Fatalf("split UTF-8 or raw binary receive was corrupted: %s", result.JSON("", ""))
		}
		if !result.Get("rxDidNotRenew").Bool() {
			t.Fatal("receive bytes caused an API request or new serial session")
		}
		if !strings.Contains(result.Get("exported").Str(), "\tRX\tE2 82") || !strings.Contains(result.Get("exported").Str(), "\tRX\tAC") || !strings.Contains(result.Get("exported").Str(), "\tRX\t00 80 FF") || !strings.Contains(result.Get("exported").Str(), "\tTX\tAA 00 FF") || !strings.Contains(result.Get("exported").Str(), "\tTX\t68 65 6C 6C 6F 0D 0A") {
			t.Fatalf("capture export lost a timestamped direction or exact bytes")
		}
		if !strings.Contains(result.Get("exported").Str(), "T") || !strings.HasPrefix(result.Get("download").Str(), "serial-capture-") {
			t.Fatalf("capture timestamp or download name missing")
		}
		if len(result.Get("initialSignals").Arr()) == 0 {
			t.Fatalf("opening the port did not explicitly initialize DTR/RTS low: %s", result.JSON("", ""))
		}
		firstSignals := result.Get("initialSignals").Get("0")
		if firstSignals.Get("dataTerminalReady").Bool() || firstSignals.Get("requestToSend").Bool() {
			t.Fatalf("initial modem signals were not both false: %s", result.JSON("", ""))
		}
	})

	t.Run("failed TX is not exported", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		ok := page.MustEval(`async()=>{
            const app=serialTest.app,port=serialTest.makePort({failWrite:true});
            serialTest.requestPort=()=>Promise.resolve(port);await app.connectProfile('browser');
            await serialTest.wait(()=>app.connected);
            const input=document.querySelector('[data-serial-input]');input.value='fail';
            document.querySelector('[data-serial-form]').requestSubmit();
            await serialTest.wait(()=>document.querySelector('[data-serial-status]').dataset.state==='error');
            const originalCreate=URL.createObjectURL,originalClick=HTMLAnchorElement.prototype.click;
            URL.createObjectURL=blob=>{serialTest.exportBlob=blob;return 'blob:serial-test';};
            HTMLAnchorElement.prototype.click=function(){};
            try{document.querySelector('[data-serial-export]').click();}finally{URL.createObjectURL=originalCreate;HTMLAnchorElement.prototype.click=originalClick;}
            return !(await serialTest.exportBlob.text()).includes('\tTX\t66 61 69 6C');
        }`).Bool()
		if !ok {
			t.Fatal("capture exported TX bytes after the serial write failed")
		}
	})

	t.Run("terminal input readiness and RX replies do not renew idle activity", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		result := page.MustEval(`async()=>{
            const app=serialTest.app,port=serialTest.makePort();let terminal=serialTest.terminals.at(-1);
            serialTest.requestPort=()=>Promise.resolve(port);
            terminal.emitData('before-open');
            const beforeOpen=port.writes.length;
            await app.connectProfile('browser');await serialTest.wait(()=>app.connected);
            terminal=serialTest.terminals.at(-1);
            const timersOnOpen=serialTest.timerSchedules.length;
            terminal.replyOnNextWrite=true;port.inject([0x1B,0x5B,0x36,0x6E]);
            await serialTest.wait(()=>port.writes.length===1);
            const timersAfterReply=serialTest.timerSchedules.length;
            terminal.emitData('keyboard');await serialTest.wait(()=>port.writes.length===2);
            const timersAfterTerminalData=serialTest.timerSchedules.length;
            const input=document.querySelector('[data-serial-input]');input.value='user';document.querySelector('[data-serial-form]').requestSubmit();
            await serialTest.wait(()=>port.writes.length===3);
            return {beforeOpen,timersOnOpen,timersAfterReply,timersAfterTerminalData,timersAfterSend:serialTest.timerSchedules.length,writes:port.writes.map(bytes=>Array.from(bytes))};
        }`)
		if result.Get("beforeOpen").Int() != 0 || result.Get("timersAfterReply").Int() != result.Get("timersOnOpen").Int() || result.Get("timersAfterTerminalData").Int() != result.Get("timersAfterReply").Int() || result.Get("timersAfterSend").Int() <= result.Get("timersAfterTerminalData").Int() {
			t.Fatalf("terminal readiness or trusted-activity timer contract failed: %s", result.JSON("", ""))
		}
		if result.Get("writes").Get("0").JSON("", "") != "[27,91,49,59,49,82]" || result.Get("writes").Get("1").JSON("", "") != "[107,101,121,98,111,97,114,100]" {
			t.Fatalf("terminal keyboard or emulator response did not reach the raw serial writer: %s", result.JSON("", ""))
		}
		if result.Get("writes").Get("2").JSON("", "") != "[117,115,101,114,13]" {
			t.Fatalf("explicit form send did not preserve the selected line ending: %s", result.JSON("", ""))
		}
	})

	t.Run("two Quick Connect serial windows own independent ports", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		result := page.MustEval(`async()=>{
            const firstApp=serialTest.app,first=serialTest.makePort(),second=serialTest.makePort();
            serialTest.requestPort=()=>Promise.resolve(first);await firstApp.connectProfile('browser');await serialTest.wait(()=>first.openCount===1);
            const list=document.createElement('aside'),content=document.createElement('main');document.querySelector('#qc').append(list,content);
            const secondApp=serialTest.makeApp(list,content);secondApp.render();await serialTest.wait(()=>list.querySelector('[data-serial-connect="browser"]'));
            serialTest.requestPort=()=>Promise.resolve(second);await secondApp.connectProfile('browser');await serialTest.wait(()=>second.openCount===1);
            await firstApp.disconnect('manual');await serialTest.wait(()=>first.closeCount===1);
            const secondSurvived=secondApp.connected&&second.closeCount===0&&content.querySelector('[data-serial-status]')?.dataset.state==='connected';
            await secondApp.disconnect('manual');await serialTest.wait(()=>second.closeCount===1);
            return {firstClose:first.closeCount,secondClose:second.closeCount,secondSurvived,starts:serialTest.sessionStarts,ends:serialTest.sessionEnds};
        }`)
		if result.Get("firstClose").Int() != 1 || result.Get("secondClose").Int() != 1 || !result.Get("secondSurvived").Bool() || result.Get("starts").Int() != 2 || result.Get("ends").Int() != 2 {
			t.Fatalf("disposing one Quick Connect window affected another serial port owner: %s", result.JSON("", ""))
		}
	})

	t.Run("capture byte and record limits", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		limit := page.MustEval(`async()=>{
            const app=serialTest.app,port=serialTest.makePort();serialTest.requestPort=()=>Promise.resolve(port);
            await app.connectProfile('browser');await serialTest.wait(()=>app.connected);
            for(let i=1;i<=11;i++)port.inject(new Uint8Array(1024*1024).fill(i));
            await serialTest.wait(()=>serialTest.terminals.at(-1)?.writeCalls>=11);
            const originalCreate=URL.createObjectURL,originalClick=HTMLAnchorElement.prototype.click;
            URL.createObjectURL=blob=>{serialTest.exportBlob=blob;return 'blob:serial-test';};HTMLAnchorElement.prototype.click=function(){};
            try{document.querySelector('[data-serial-export]').click();}finally{URL.createObjectURL=originalCreate;HTMLAnchorElement.prototype.click=originalClick;}
            const text=await serialTest.exportBlob.text(),lines=text.split('\n').filter(line=>/^\d{4}-/.test(line));
            return {marker:text.startsWith('# Older capture data was dropped.\n'),count:lines.length,first:lines[0]?.slice(0,60),last:lines.at(-1)?.slice(-12),hexChars:lines.reduce((n,line)=>n+line.length,0)};
        }`)
		if !limit.Get("marker").Bool() || limit.Get("count").Int() != 9 || !strings.Contains(limit.Get("first").Str(), "\tRX\t03 03 03") || !strings.HasSuffix(limit.Get("last").Str(), "0B 0B 0B") {
			t.Fatalf("capture did not retain bounded bytes with a truncation marker: %s", limit.JSON("", ""))
		}
		if limit.Get("hexChars").Int() > 10*1024*1024*3+1000 {
			t.Fatalf("capture exceeded the 10 MiB byte cap: %s", limit.JSON("", ""))
		}
		count := page.MustEval(`async()=>{
            document.querySelector('[data-serial-clear]').click();
            // Recreate a fresh capture through the same live stream and exceed the record limit.
            const port=serialTest.ports[0];
            for(let i=0;i<10001;i++)port.inject([0xA5]);
            await serialTest.wait(()=>serialTest.terminals.at(-1)?.writeCalls>=10001);
            const originalCreate=URL.createObjectURL,originalClick=HTMLAnchorElement.prototype.click;
            URL.createObjectURL=blob=>{serialTest.exportBlob=blob;return 'blob:serial-test';};HTMLAnchorElement.prototype.click=function(){};
            try{document.querySelector('[data-serial-export]').click();}finally{URL.createObjectURL=originalCreate;HTMLAnchorElement.prototype.click=originalClick;}
            const text=await serialTest.exportBlob.text(),lines=text.split('\n').filter(line=>/^\d{4}-/.test(line));
            return {marker:text.startsWith('# Older capture data was dropped.\n'),count:lines.length,example:lines[0]};
        }`)
		if !count.Get("marker").Bool() || count.Get("count").Int() != 10000 || !strings.Contains(count.Get("example").Str(), "\tRX\tA5") {
			t.Fatalf("capture record limit/export metadata failed: %s", count.JSON("", ""))
		}
	})

	t.Run("DTR RTS and 250ms break", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		result := page.MustEval(`async()=>{
            const app=serialTest.app,port=serialTest.makePort();serialTest.requestPort=()=>Promise.resolve(port);
            await app.connectProfile('browser');await serialTest.wait(()=>app.connected);
            const dtr=document.querySelector('[data-serial-dtr]'),rts=document.querySelector('[data-serial-rts]');
            dtr.checked=true;dtr.dispatchEvent(new Event('change',{bubbles:true}));
            rts.checked=true;rts.dispatchEvent(new Event('change',{bubbles:true}));
            await serialTest.wait(()=>port.signals.some(s=>s.dataTerminalReady===true)&&port.signals.some(s=>s.requestToSend===true));
            const button=document.querySelector('[data-serial-break]');button.click();
            await serialTest.wait(()=>port.signals.some(s=>s.break===true));
            const started=port.signalTimes.find(x=>x.value.break===true).at;
            await serialTest.wait(()=>port.signals.some(s=>s.break===false),1000);
            const ended=port.signalTimes.find(x=>x.value.break===false).at;
            return {signals:port.signals,elapsed:ended-started};
        }`)
		if result.Get("elapsed").Int() < 240 || result.Get("elapsed").Int() > 1000 {
			t.Fatalf("Break was not held for 250 ms: %s", result.JSON("", ""))
		}
		if !strings.Contains(result.JSON("", ""), `"dataTerminalReady":true`) || !strings.Contains(result.JSON("", ""), `"requestToSend":true`) {
			t.Fatalf("DTR/RTS controls were not forwarded: %s", result.JSON("", ""))
		}
	})

	t.Run("picker revocation, delayed open and teardown", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		result := page.MustEval(`async()=>{
            const app=serialTest.app,port=serialTest.makePort();let resolvePick;
            serialTest.requestPort=()=>new Promise(resolve=>resolvePick=resolve);
            const pending=app.connectProfile('browser');await serialTest.wait(()=>!!resolvePick);
            document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{serial_browser_enabled:false}}));
            document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{serial_browser_enabled:true}}));
            resolvePick(port);await pending;await new Promise(r=>setTimeout(r,30));
            const latePickOpens=port.openCount;

            let releaseOpen;const late=serialTest.makePort({open:()=>new Promise(resolve=>releaseOpen=resolve)});
            serialTest.requestPort=()=>Promise.resolve(late);
            const opening=app.connectProfile('browser');await serialTest.wait(()=>!!releaseOpen);
            app.dispose();releaseOpen();await opening;await new Promise(r=>setTimeout(r,30));
            return {latePickOpens,lateOpenCloses:late.closeCount,connected:app.connected,starts:serialTest.sessionStarts,ends:serialTest.sessionEnds,cleanup:serialTest.sessionEndReasons};
        }`)
		if result.Get("latePickOpens").Int() != 0 || result.Get("lateOpenCloses").Int() != 1 || result.Get("connected").Bool() {
			t.Fatalf("revoked picker or disposed pending open revived serial I/O: %s", result.JSON("", ""))
		}
		if result.Get("starts").Int() != result.Get("ends").Int() || result.Get("ends").Int() != 1 || !strings.Contains(result.Get("cleanup").Str(), "dispose") {
			t.Fatalf("session owner lifecycle did not close exactly once: %s", result.JSON("", ""))
		}
	})

	t.Run("policy, auth expiry and late close do not touch newer connection", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		result := page.MustEval(`async()=>{
            const app=serialTest.app,first=serialTest.makePort({deferClose:true}),second=serialTest.makePort();
            serialTest.requestPort=()=>Promise.resolve(first);await app.connectProfile('browser');await serialTest.wait(()=>app.connected);
            const disconnecting=app.disconnect('manual');await serialTest.wait(()=>!!first.releaseClose);
            const list=document.createElement('aside'),content=document.createElement('main');document.querySelector('#qc').append(list,content);
            const newer=serialTest.makeApp(list,content);newer.render();await serialTest.wait(()=>list.querySelector('[data-serial-connect="browser"]'));
            serialTest.requestPort=()=>Promise.resolve(second);await newer.connectProfile('browser');await serialTest.wait(()=>newer.connected&&second.openCount===1);
            first.releaseClose();await disconnecting;await new Promise(r=>setTimeout(r,20));
            const staleStatus=content.querySelector('[data-serial-status]').dataset.state,secondStayedConnected=newer.connected&&second.closeCount===0;
            document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}));
            await serialTest.wait(()=>!app.connected&&!newer.connected);
            document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:false,serial_browser_enabled:true}}));
            const third=serialTest.makePort();serialTest.requestPort=()=>Promise.resolve(third);
            await app.connectProfile('browser');await serialTest.wait(()=>app.connected);
            document.dispatchEvent(new Event('aurago:auth-ended'));await serialTest.wait(()=>!app.connected);
            const deniedAfterAuth=serialTest.makePort();serialTest.requestPort=()=>Promise.resolve(deniedAfterAuth);await app.connectProfile('browser');await new Promise(r=>setTimeout(r,20));
            return {staleStatus,secondStayedConnected,closeCounts:[first.closeCount,second.closeCount,third.closeCount,deniedAfterAuth.openCount],reasons:serialTest.sessionEndReasons,starts:serialTest.sessionStarts,ends:serialTest.sessionEnds};
        }`)
		if result.Get("staleStatus").Str() != "connected" || !result.Get("secondStayedConnected").Bool() {
			t.Fatalf("late port close overwrote the newer connection status: %s", result.JSON("", ""))
		}
		if result.Get("starts").Int() != result.Get("ends").Int() || !strings.Contains(result.Get("reasons").Str(), "policy") || !strings.Contains(result.Get("reasons").Str(), "auth") || result.Get("closeCounts").Get("3").Int() != 0 {
			t.Fatalf("readonly or auth-ended failed to terminate session ownership: %s", result.JSON("", ""))
		}
	})

	t.Run("late break completion cannot cancel a newer break", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		ok := page.MustEval(`async()=>{
            const app=serialTest.app,first=serialTest.makePort({deferBreak:true}),second=serialTest.makePort();
            serialTest.requestPort=()=>Promise.resolve(first);
            await app.connectProfile('browser');await serialTest.wait(()=>app.connected);
            document.querySelector('[data-serial-break]').click();await serialTest.wait(()=>!!first.releaseBreak);
            const stopping=app.disconnect('manual');
            const list=document.createElement('aside'),content=document.createElement('main');list.id='second-list';content.id='second-content';document.body.append(list,content);
            const other=serialTest.makeApp(list,content);other.render();serialTest.requestPort=()=>Promise.resolve(second);
            await serialTest.wait(()=>list.querySelector('[data-serial-connect="browser"]'));
            await other.connectProfile('browser');await serialTest.wait(()=>other.connected&&second.openCount===1);
            content.querySelector('[data-serial-break]').click();await serialTest.wait(()=>second.signals.some(s=>s.break===true));
            await new Promise(r=>setTimeout(r,150));first.releaseBreak();await stopping;
            await serialTest.wait(()=>second.signals.some(s=>s.break===false),1000);
            return first.closeCount===1&&second.closeCount===0&&second.signals.filter(s=>s.break===true).length===1&&second.signals.some(s=>s.break===false);
        }`).Bool()
		if !ok {
			t.Fatal("late Break completion cancelled the current connection's Break timer")
		}
	})

	t.Run("host WebSocket ordering and binary payloads", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		result := page.MustEval(`async()=>{
            const app=serialTest.app;await app.connectProfile('host');
            const socket=serialTest.sockets.at(-1);socket.open();
            await serialTest.wait(()=>socket.sent.length===1);
            const open=JSON.parse(socket.sent[0]);
            socket.message(JSON.stringify({type:'connected'}));
            const mode=document.querySelector('[data-serial-send-mode]'),input=document.querySelector('[data-serial-input]');
            mode.value='hex';mode.dispatchEvent(new Event('change',{bubbles:true}));input.value='01 80 FF';
            document.querySelector('[data-serial-form]').requestSubmit();await serialTest.wait(()=>socket.sent.length===2);
            const rx=new Uint8Array([0,128,255]);socket.message(rx.buffer);
            await serialTest.wait(()=>serialTest.terminals.at(-1)?.writeCalls>0);
            socket.message(JSON.stringify({type:'disconnected'}));await serialTest.wait(()=>!app.connected);
            return {open,sent:socket.sent.map(item=>typeof item==='string'?item:Array.from(new Uint8Array(item))),status:document.querySelector('[data-serial-status]').dataset.state};
        }`)
		if result.Get("open").Get("type").Str() != "open" || result.Get("open").Get("port").Str() != "COM3" || len(result.Get("sent").Arr()) != 3 || result.Get("sent").Get("1").JSON("", "") != "[1,128,255]" || !strings.Contains(result.Get("sent").Get("2").Str(), `"type":"disconnect"`) {
			t.Fatalf("host open/binary WebSocket ordering failed: %s", result.JSON("", ""))
		}
		if result.Get("status").Str() != "error" {
			t.Fatalf("remote disconnect did not report a lost serial connection: %s", result.JSON("", ""))
		}
	})
}

func TestQuickConnectSerialSwitchesThroughExistingDesktopSessionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := newQuickConnectShellPage(t, browser, "standard", 1200, 800)
	defer page.Close()

	result := page.MustEval(`async()=>{
        await serialShell.wait(()=>document.querySelector('[data-device-id="ssh-lab"]')&&document.querySelector('[data-serial-connect="browser"]'));
        document.querySelector('[data-serial-connect="browser"]').click();
        await serialShell.wait(()=>serialShell.ports[0]?.openCount===1&&document.querySelector('[data-serial-status]')?.dataset.state==='connected');
        const serialVisible=!document.querySelector('[data-serial-content]').hidden;
        document.querySelector('[data-device-id="ssh-lab"]').click();
        await serialShell.wait(()=>serialShell.sockets.length===1&&document.querySelector('[data-qc-tabs]').hidden===false);
        const afterSSH={serialVisible:!document.querySelector('[data-serial-content]').hidden,serialClosed:serialShell.ports[0].closeCount,sshURL:serialShell.sockets[0].url,tabVisible:!document.querySelector('[data-tab-content]').hidden};
        document.querySelector('[data-tab="files"]').click();
        await serialShell.wait(()=>!!document.querySelector('.vd-qc-sftp-panel'));
        document.querySelector('[data-device-id="vnc-lab"]').click();
        await serialShell.wait(()=>serialShell.rfbs.length===1&&!!document.querySelector('.vd-qc-vnc-session'));
        const afterVNC={vncURL:serialShell.rfbs[0].url,sshClosed:serialShell.sockets[0].closeCount,terminalDisposed:serialShell.terminals[0].disposed,sftpDisposed:serialShell.sftpDisposals,serialVisible:!document.querySelector('[data-serial-content]').hidden,tabsFilesHidden:document.querySelector('[data-tab="files"]').hidden};
        serialShell.cleanup();
        return {afterSSH,afterVNC,vncDisposed:serialShell.rfbs[0].disconnectCount,cleanupRan:serialShell.cleaned};
    }`)
	afterSSH := result.Get("afterSSH")
	if afterSSH.Get("serialVisible").Bool() || afterSSH.Get("serialClosed").Int() != 1 || !strings.Contains(afterSSH.Get("sshURL").Str(), "/api/desktop/ssh?device_id=ssh-lab") || !afterSSH.Get("tabVisible").Bool() {
		t.Fatalf("switching from Serial to SSH did not close Serial and restore the existing terminal shell: %s", result.JSON("", ""))
	}
	afterVNC := result.Get("afterVNC")
	if !strings.Contains(afterVNC.Get("vncURL").Str(), "/api/desktop/vnc?device_id=vnc-lab") || afterVNC.Get("sshClosed").Int() != 1 || !afterVNC.Get("terminalDisposed").Bool() || afterVNC.Get("sftpDisposed").Int() < 1 || afterVNC.Get("serialVisible").Bool() || !afterVNC.Get("tabsFilesHidden").Bool() {
		t.Fatalf("switching from SSH files to VNC did not clean the existing remote session: %s", result.JSON("", ""))
	}
	if !result.Get("cleanupRan").Bool() || result.Get("vncDisposed").Int() != 1 {
		t.Fatalf("Quick Connect window cleanup did not dispose the active VNC session: %s", result.JSON("", ""))
	}
}

func TestQuickConnectSerialVisualBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	artifactDir := filepath.Join("..", "reports", "quickconnect-serial")
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity"} {
		for _, viewport := range []struct {
			name   string
			width  int
			height int
		}{{"wide", 1240, 820}, {"narrow", 420, 780}} {
			t.Run(theme+"/"+viewport.name, func(t *testing.T) {
				page := newQuickConnectShellPage(t, browser, theme, viewport.width, viewport.height)
				defer page.Close()
				page.MustWait(`()=>!!document.querySelector('[data-qc-filter="serial"]')`)
				page.MustElement(`[data-qc-filter="serial"]`).MustClick()
				page.MustWait(`()=>!!document.querySelector('[data-serial-connect="browser"]')`)
				page.MustElement(`[data-serial-connect="browser"]`).MustClick()
				page.MustWait(`()=>serialShell.ports[0]?.openCount===1&&document.querySelector('[data-serial-status]')?.dataset.state==='connected'&&!!document.querySelector('[data-serial-content]:not([hidden]) [data-qc-serial-app]')`)
				if rawKeys := page.MustEval(`()=>document.body.innerText.includes('desktop.qc_')`).Bool(); rawKeys {
					t.Fatal("Quick Connect visual fixture rendered untranslated locale keys")
				}
				geometry := page.MustEval(`()=>{const rect=el=>{const r=el.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,display:getComputedStyle(el).display,hidden:el.hidden}};const content=document.querySelector('[data-serial-content]'),config=document.querySelector('[data-serial-editor]'),form=config.querySelector('[data-serial-profile-form]'),terminal=document.querySelector('[data-serial-terminal]'),send=document.querySelector('[data-serial-form]'),inside=(child,parent)=>{const c=child.getBoundingClientRect(),p=parent.getBoundingClientRect();return c.width>0&&c.height>0&&c.top>=p.top-1&&c.bottom<=p.bottom+1};return {width:document.documentElement.clientWidth,scrollWidth:document.documentElement.scrollWidth,root:rect(document.querySelector('.vd-quick-connect')),content:rect(content),serialScroll:content.scrollWidth,config:rect(config),settingsScrollable:config.scrollHeight>config.clientHeight,profileFields:form.querySelectorAll('input,select').length,lineEndingAccessible:!!form.querySelector('[name="line_ending"]'),terminal:rect(terminal),consoleVisible:terminal.getBoundingClientRect().height>=120&&inside(terminal,content),send:rect(send),sendVisible:inside(send,content),sendControlsVisible:Array.from(send.querySelectorAll('select,input,button')).every(control=>inside(control,content))}}`)
				if geometry.Get("scrollWidth").Int() > geometry.Get("width").Int()+1 {
					t.Fatalf("serial app overflows %s at %dpx: %s", viewport.name, viewport.width, geometry.JSON("", ""))
				}
				if geometry.Get("serialScroll").Int() > geometry.Get("content").Get("width").Int()+1 {
					t.Fatalf("serial content overflows its Quick Connect shell at %s %dpx: %s", viewport.name, viewport.width, geometry.JSON("", ""))
				}
				if viewport.width <= 680 && (!geometry.Get("settingsScrollable").Bool() || geometry.Get("profileFields").Int() < 10 || !geometry.Get("lineEndingAccessible").Bool() || !geometry.Get("consoleVisible").Bool() || !geometry.Get("sendVisible").Bool() || !geometry.Get("sendControlsVisible").Bool()) {
					t.Fatalf("narrow serial layout must keep settings scrollable while console and every send control remain visible: %s", geometry.JSON("", ""))
				}
				path := filepath.Join(artifactDir, "quickconnect-serial-"+theme+"-"+viewport.name+".png")
				if err := os.WriteFile(path, page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func newQuickConnectSerialPage(t *testing.T, browser *rod.Browser, theme string, width, height int) *rod.Page {
	t.Helper()
	css := readDesktopAssetText(t, "css/desktop-app-quick-connect.css")
	module := readQuickConnectSerialSources(t)
	page := browser.MustPage().Timeout(45 * time.Second)
	page.MustSetViewport(width, height, 1, false)
	page.MustSetDocumentContent(`<!doctype html><html><head><meta charset="utf-8"><style>` + css + `</style><style>
html,body{margin:0;width:100%;height:100%;font-family:system-ui,sans-serif;overflow:hidden}
body{--vd-theme-app-bg:#eef1f6;--vd-theme-panel-bg:#fff;--vd-theme-panel-bg-strong:#e8ebf1;--vd-theme-chrome-bg:#f7f8fb;--vd-theme-border:#d2d7e0;--vd-theme-border-strong:#b3bac6;--vd-text:#1a1e26;--vd-theme-muted:#667080;--vd-bg:#fff;--vd-accent:#7655c8;--vd-theme-accent-soft:rgba(118,85,200,.14);--vd-control-bg:rgba(0,0,0,.04);--ds-space-2:8px;--ds-space-3:12px;--ds-space-6:24px;--ds-radius-xs:4px;--ds-radius-sm:6px;--ds-radius-md:10px;--ds-radius-full:999px;--ds-type-size-xs:11px;--ds-type-size-sm:13px;--ds-type-size-md:14px;--ds-type-size-lg:17px;--ds-type-size-xl:20px;--ds-motion-duration-fast:120ms;--ds-motion-ease-standard:ease;--ds-focus-ring-width:2px;--ds-focus-ring-offset:2px;color:var(--vd-text);background:var(--vd-theme-app-bg)}
body[data-theme="fruity"]{--vd-theme-app-bg:#f8f3ee;--vd-theme-panel-bg:#fffaf5;--vd-theme-panel-bg-strong:#f1e7df;--vd-theme-chrome-bg:#fff7f0;--vd-theme-border:#e8d9cc;--vd-theme-border-strong:#d0b9a8;--vd-text:#30251f;--vd-theme-muted:#806f64;--vd-bg:#fffaf5;--vd-accent:#e36a91;--vd-theme-accent-soft:rgba(227,106,145,.14);--vd-control-bg:rgba(74,43,27,.05)}
#qc{height:100%;width:100%;min-width:0;display:flex}.vd-qc-sidebar{min-width:190px;width:230px;max-width:38%;overflow:auto}.vd-qc-terminal-area{flex:1;min-width:0;overflow:hidden}.vd-qc-serial-host{display:flex;flex:1;min-width:0;min-height:0;overflow:auto}
@media(max-width:680px){#qc{flex-direction:column}.vd-qc-sidebar{width:auto;max-width:none;min-width:0;height:130px;flex:none}.vd-qc-terminal-area{flex:1;min-height:0}}
</style></head><body data-theme="` + theme + `"><div id="qc" class="vd-quick-connect"><aside id="list" class="vd-qc-sidebar"></aside><main id="content" class="vd-qc-terminal-area vd-qc-serial-host"></main></div><script>
window.__errors=[];window.addEventListener('error',e=>__errors.push(e.error?.stack||e.message));
window.esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
window.iconMarkup=()=>'';
window.serialTest={
 profiles:[
  {id:'browser',name:'Loopback adapter',source:'browser',port:'',usb_vendor_id:0x10C4,usb_product_id:0xEA60,options:{baud_rate:115200,data_bits:8,stop_bits:1,parity:'none',flow_control:'none',local_echo:false,line_ending:'cr',dtr:false,rts:false}},
  {id:'browser-second',name:'Second adapter',source:'browser',port:'',options:{baud_rate:9600,data_bits:8,stop_bits:1,parity:'none',flow_control:'none',local_echo:false,line_ending:'none',dtr:false,rts:false}},
  {id:'host',name:'Server COM port',source:'host',port:'COM3',options:{baud_rate:57600,data_bits:8,stop_bits:1,parity:'none',flow_control:'none',local_echo:false,line_ending:'none',dtr:false,rts:false}}
 ],ports:[],sockets:[],terminals:[],requests:[],timerSchedules:[],ownerWindow:null,sessionStarts:0,sessionEnds:0,sessionEndReasons:[],
 wait:async(predicate,timeout=5000)=>{const end=Date.now()+timeout;while(Date.now()<end){if(predicate())return;await new Promise(r=>setTimeout(r,5));}throw Error('timed out waiting for browser fixture condition: '+JSON.stringify({connected:serialTest.app?.connected,status:document.querySelector('[data-serial-status]')?.dataset.state,ports:serialTest.ports.map(p=>({opens:p.openCount,closes:p.closeCount,reads:p.readCalls,writes:p.writes.length,locked:p.readable.locked})),terminals:serialTest.terminals.map(t=>({calls:t.writeCalls,display:t.display.slice(-40)})),errors:window.__errors}));},
 makePort(options={}){let controller;const port={openCount:0,closeCount:0,readCalls:0,writes:[],signals:[],signalTimes:[],openOptions:null,readable:new ReadableStream({start(c){controller=c;}}),
  writable:new WritableStream({write(chunk){const bytes=Uint8Array.from(chunk);if(options.failWrite)throw Error('simulated_write_failure');port.writes.push(bytes);},abort(){}}),
  open(config){port.openCount++;port.openOptions=config;if(options.open)return options.open();return Promise.resolve();},
  close(){port.closeCount++;const done=()=>{try{controller?.close();}catch(_){}};if(options.deferClose)return new Promise(resolve=>port.releaseClose=()=>{done();resolve();});done();return Promise.resolve();},
  setSignals(value){port.signals.push({...value});port.signalTimes.push({value:{...value},at:Date.now()});if(options.deferBreak&&value.break===true&&!port.releaseBreak)return new Promise(resolve=>port.releaseBreak=resolve);return Promise.resolve();},
  inject(value){controller.enqueue(value instanceof Uint8Array?value:new Uint8Array(value));}
 };const getReader=port.readable.getReader.bind(port.readable);port.readable.getReader=(...args)=>{const reader=getReader(...args),read=reader.read.bind(reader);reader.read=()=>{port.readCalls++;return read();};return reader;};serialTest.ports.push(port);return port;}
};
serialTest.ownerWindow={isSecureContext:true,location:window.location,setTimeout:(fn,delay)=>{const id=window.setTimeout(fn,delay);serialTest.timerSchedules.push({id,delay});return id;},clearTimeout:id=>window.clearTimeout(id)};
class TestTerminal{constructor(){this.writes=[];this.display='';this.writeCalls=0;this.disposed=false;this.decoder=new TextDecoder();serialTest.terminals.push(this);}loadAddon(){}open(){}onData(handler){this.inputHandler=handler;return {dispose(){}};}emitData(value){this.inputHandler?.(value);}onResize(){return {dispose(){}};}write(value,done){this.writeCalls++;this.writes.push(value);const text=typeof value==='string'?value:this.decoder.decode(value,{stream:true});this.display+=text.slice(0,4096);if(this.replyOnNextWrite&&text.includes(String.fromCharCode(27)+'[6n')){this.replyOnNextWrite=false;queueMicrotask(()=>this.inputHandler?.(String.fromCharCode(27)+'[1;1R'));}done?.();}clear(){this.display='';}dispose(){this.disposed=true;}}
class TestFitAddon{fit(){}}
class TestSocket extends EventTarget{static OPEN=1;constructor(url){super();this.url=url;this.readyState=0;this.bufferedAmount=0;this.sent=[];serialTest.sockets.push(this);}open(){this.readyState=1;this.dispatchEvent(new Event('open'));}send(data){this.sent.push(typeof data==='string'?data:data.slice(0));}message(data){this.dispatchEvent(new MessageEvent('message',{data}));}close(){if(this.readyState===3)return;this.readyState=3;this.dispatchEvent(new Event('close'));}}
window.TestTerminal=TestTerminal;window.TestFitAddon=TestFitAddon;window.TestSocket=TestSocket;
</script>`)
	if err := page.AddScriptTag("", normalizeAssetText([]byte(module))); err != nil {
		t.Fatalf("load Quick Connect serial module: %v", err)
	}
	if !page.MustEval(`()=>!!window.QuickConnectSerial`).Bool() {
		t.Fatalf("Quick Connect serial module did not initialize: %s; errors: %s", strings.TrimSpace(module[:min(len(module), 300)]), page.MustEval(`()=>window.__errors`).JSON("", ""))
	}
	page.MustEval(`()=>{
        const bootstrap={enabled:true,readonly:false,serial_browser_enabled:true,serial_host_enabled:true,settings:{'quick_connect.serial_profiles':JSON.stringify({version:1,profiles:serialTest.profiles})}};
        const context={state:{bootstrap}};
        const api=async(url,options={})=>{serialTest.requests.push([url,options]);if(url==='/api/desktop/settings')return {settings:{'quick_connect.serial_profiles':JSON.stringify({version:1,profiles:serialTest.profiles})}};if(url==='/api/desktop/serial/ports')return {ports:[{name:'COM3',busy:false}]};return {};};
        serialTest.requestPort=async(options)=>{serialTest.lastChooserOptions=options;return serialTest.makePort();};
        serialTest.context=context;serialTest.api=api;serialTest.navigator={serial:{requestPort:options=>{serialTest.lastChooserOptions=options;return serialTest.requestPort(options);}}};
        serialTest.onSessionStart=()=>{serialTest.sessionStarts++;return ()=>true;};serialTest.onSessionEnd=(_owner,reason)=>{serialTest.sessionEnds++;serialTest.sessionEndReasons.push(reason);};
        serialTest.makeApp=(list,content)=>QuickConnectSerial.create({list,content,context:serialTest.context,api:serialTest.api,t:key=>key,navigator:serialTest.navigator,window:serialTest.ownerWindow,WebSocket:TestSocket,Terminal:TestTerminal,FitAddon:{FitAddon:TestFitAddon},onSessionStart:serialTest.onSessionStart,onSessionEnd:serialTest.onSessionEnd});
        serialTest.app=serialTest.makeApp(document.querySelector('#list'),document.querySelector('#content'));
        serialTest.app.render();
    }`)
	page.MustWait(`()=>window.serialTest?.app && document.querySelector('[data-qc-serial-app]') && document.querySelectorAll('[data-serial-profile]').length===3`)
	return page
}

func newQuickConnectShellPage(t *testing.T, browser *rod.Browser, theme string, width, height int) *rod.Page {
	t.Helper()
	tokens := readDesktopAssetText(t, "css/tokens.css")
	shellCSS := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	quickConnectCSS := readDesktopAssetText(t, "css/desktop-app-quick-connect.css")
	localeSource := readDesktopAssetText(t, "lang/desktop/en.json")
	translations := map[string]string{}
	if err := json.Unmarshal([]byte(localeSource), &translations); err != nil {
		t.Fatalf("decode Desktop English locale: %v", err)
	}
	localeJSON, err := json.Marshal(translations)
	if err != nil {
		t.Fatalf("encode Desktop English locale for browser: %v", err)
	}
	page := browser.MustPage().Timeout(45 * time.Second)
	page.MustSetViewport(width, height, 1, false)
	page.MustSetDocumentContent(`<!doctype html><html><head><meta charset="utf-8"><style>` + tokens + "\n" + shellCSS + "\n" + quickConnectCSS + `</style><style>
html,body,#shell,#qc{box-sizing:border-box;margin:0;width:100%;height:100%;min-width:0}body{overflow:hidden}
</style></head><body class="desktop-body" data-theme="` + theme + `" data-fruity-mode="light" data-animations="false"><div id="shell"><div id="qc"></div></div><script>
window.__shellErrors=[];window.addEventListener('error',e=>__shellErrors.push(e.error?.stack||e.message));
try{Object.defineProperty(window,'isSecureContext',{value:true,configurable:true});}catch(_){}
window.__desktopEnglish=` + string(localeJSON) + `;window.t=key=>window.__desktopEnglish[key]||key;
window.esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));window.iconMarkup=()=>'';window.state={bootstrap:{enabled:true,readonly:false,serial_browser_enabled:true,serial_host_enabled:true,settings:{'quick_connect.serial_profiles':JSON.stringify({version:1,profiles:[{id:'browser',name:'USB adapter',source:'browser',port:'',options:{baud_rate:115200,data_bits:8,stop_bits:1,parity:'none',flow_control:'none',local_echo:false,line_ending:'cr',dtr:false,rts:false}}]})}}};
window.serialShell={ports:[],sockets:[],terminals:[],rfbs:[],requests:[],sessionEnds:0,sftpDisposals:0,cleaned:false,wait:async(predicate,timeout=5000)=>{const end=Date.now()+timeout;while(Date.now()<end){if(predicate())return;await new Promise(r=>setTimeout(r,5));}throw Error('shell fixture timeout '+JSON.stringify({connected:serialShell.app?.connected,devices:Array.from(document.querySelectorAll('[data-device-id]')).map(x=>x.dataset.deviceId),status:document.querySelector('[data-serial-status]')?.dataset.state,ports:serialShell.ports.map(p=>[p.openCount,p.closeCount]),sockets:serialShell.sockets.map(s=>s.url),errors:window.__shellErrors}));},makePort(){let controller;const p={openCount:0,closeCount:0,writes:[],signals:[],readable:new ReadableStream({start(c){controller=c;}}),writable:new WritableStream({write(b){p.writes.push(Uint8Array.from(b));}}),open(){p.openCount++;return Promise.resolve();},close(){p.closeCount++;try{controller.close();}catch(_){}return Promise.resolve();},setSignals(v){p.signals.push(v);return Promise.resolve();}};serialShell.ports.push(p);return p;}};
Object.defineProperty(navigator,'serial',{configurable:true,value:{requestPort:()=>Promise.resolve(serialShell.makePort())}});
window.api=async(url,options={})=>{serialShell.requests.push([url,options]);if(url==='/api/desktop/integrations/devices')return {devices:[{id:'ssh-lab',name:'SSH lab',type:'server',protocol:'ssh',ip_address:'192.0.2.10',port:22},{id:'vnc-lab',name:'VNC lab',type:'server',protocol:'vnc',ip_address:'192.0.2.11',port:5900}]};if(url==='/api/desktop/integrations/credentials')return [];if(url==='/api/desktop/settings')return {settings:{'quick_connect.serial_profiles':JSON.stringify({version:1,profiles:state.bootstrap.settings['quick_connect.serial_profiles']&&JSON.parse(state.bootstrap.settings['quick_connect.serial_profiles']).profiles})}};if(url==='/api/desktop/serial/ports')return {ports:[]};return {};};
window.contentEl=id=>document.getElementById(id);window.wireContextMenuBoundary=()=>{};window.registerWindowCleanup=(_id,fn)=>{serialShell.cleanup=()=>{fn();serialShell.cleaned=true;};};window.setWindowMenus=()=>{};window.closeContextMenu=()=>{};window.showContextMenu=()=>{};window.showDesktopNotification=()=>{};window.createSFTPNavigator=()=>({path:'/',entries:[],load:async function(path){this.path=path;return {status:'ok'};},dispose(){serialShell.sftpDisposals++;}});window.ResizeObserver=class{observe(){}disconnect(){}};
class ShellTerminal{constructor(){this.disposed=false;this.cols=80;this.rows=24;serialShell.terminals.push(this);}loadAddon(){}open(){}onData(){return {dispose(){}};}onResize(){return {dispose(){}};}write(){}dispose(){this.disposed=true;}}class ShellFitAddon{fit(){}}window.Terminal=ShellTerminal;window.FitAddon={FitAddon:ShellFitAddon};
class ShellSocket extends EventTarget{static OPEN=1;constructor(url){super();this.url=url;this.readyState=0;this.closeCount=0;serialShell.sockets.push(this);}send(){}close(){if(this.readyState===3)return;this.readyState=3;this.closeCount++;this.onclose?.(new Event('close'));}}window.WebSocket=ShellSocket;
window.RFB=class extends EventTarget{constructor(_el,url){super();this.url=url;this.disconnectCount=0;serialShell.rfbs.push(this);}disconnect(){this.disconnectCount++;this.dispatchEvent(new Event('disconnect'));}};
</script></body></html>`)
	module := readQuickConnectSerialSources(t)
	if err := page.AddScriptTag("", normalizeAssetText([]byte(module))); err != nil {
		t.Fatalf("load Quick Connect serial module: %v", err)
	}
	launchpad := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js") + "\nwindow.__renderQuickConnect=renderQuickConnect;"
	if err := page.AddScriptTag("", normalizeAssetText([]byte(launchpad))); err != nil {
		t.Fatalf("load Quick Connect shell source: %v; browser errors: %s", err, page.MustEval(`()=>window.__shellErrors`).JSON("", ""))
	}
	page.MustEval(`()=>{serialShell.app=window.__renderQuickConnect('qc');}`)
	return page
}
