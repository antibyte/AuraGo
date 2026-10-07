package ui

import "testing"

func TestQuickConnectSerialProfileEditorBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	t.Run("browser policy denies the chooser without disabling host access", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		blocked := page.MustEval(`async()=>{
            Object.defineProperty(document,'permissionsPolicy',{configurable:true,value:{allowsFeature:feature=>feature!=='serial'}});
            let choices=0;
            serialTest.requestPort=()=>{choices++;return Promise.resolve(serialTest.makePort());};
            serialTest.app.render();
            await serialTest.app.connectProfile('browser');
            return choices===0&&!serialTest.app.connected&&document.querySelector('[data-serial-profile="browser"]').textContent.includes('desktop.qc_serial_browser_permission_denied')&&!document.querySelector('[data-serial-connect="host"]').disabled&&document.querySelector('[data-serial-status]').textContent==='desktop.qc_serial_browser_permission_denied';
        }`).Bool()
		if !blocked {
			t.Fatal("browser policy denial did not block the chooser independently of host access")
		}
	})
	t.Run("new window cannot connect during logout", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()
		blocked := page.MustEval(`async()=>{
            serialTest.app.dispose();
            window._logoutInProgress=true;
            let choices=0;
            serialTest.requestPort=()=>{choices++;return Promise.resolve(serialTest.makePort());};
            const app=serialTest.makeApp(document.querySelector('#list'),document.querySelector('#content'));
            app.render();
            await serialTest.wait(()=>document.querySelectorAll('[data-serial-profile]').length===3);
            await app.connectProfile('browser');
            const blocked=choices===0&&!app.connected&&document.querySelector('[data-serial-connect]').disabled;
            app.dispose();return blocked;
        }`).Bool()
		if !blocked {
			t.Fatal("a newly opened Quick Connect window requested a port during logout")
		}
	})

	t.Run("saved options reach the next connection", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()

		result := page.MustEval(`async()=>{
            const app=serialTest.app;
            document.querySelector('[data-serial-profile="browser"]').click();
            const form=document.querySelector('[data-serial-profile-form]');
            form.elements.name.value='Edited browser profile';
            form.elements.baud_preset.value='custom';
            form.elements.baud_preset.dispatchEvent(new Event('change',{bubbles:true}));
            form.elements.baud_custom.value='230400';
            form.elements.dtr.checked=true;
            form.elements.rts.checked=true;
            form.elements.line_ending.value='lf';
            form.requestSubmit();
            await serialTest.wait(()=>serialTest.requests.some(([url,options])=>url==='/api/desktop/settings'&&options.method==='PUT'));
            await serialTest.wait(()=>form.querySelector('[data-serial-form-status]')?.textContent==='desktop.qc_serial_saved');

            const request=serialTest.requests.findLast(([url,options])=>url==='/api/desktop/settings'&&options.method==='PUT');
            const payload=JSON.parse(request[1].body),stored=JSON.parse(payload.value).profiles.find(profile=>profile.id==='browser');
            const port=serialTest.makePort();serialTest.requestPort=()=>Promise.resolve(port);
            await app.connectProfile('browser');
            await serialTest.wait(()=>app.connected&&document.querySelector('[data-serial-status]')?.dataset.state==='connected');
            const ending=document.querySelector('[data-serial-line-ending]');
            const input=document.querySelector('[data-serial-input]');input.value='saved ending';
            document.querySelector('[data-serial-form]').requestSubmit();
            await serialTest.wait(()=>port.writes.length===1);
            return {stored,open:port.openOptions,signals:port.signals,consoleEnding:ending.value,write:Array.from(port.writes[0])};
        }`)
		stored := result.Get("stored")
		options := stored.Get("options")
		if stored.Get("name").Str() != "Edited browser profile" || options.Get("baud_rate").Int() != 230400 || !options.Get("dtr").Bool() || !options.Get("rts").Bool() || options.Get("line_ending").Str() != "lf" {
			t.Fatalf("profile changes were not persisted in the settings payload: %s", result.JSON("", ""))
		}
		if result.Get("open").Get("baudRate").Int() != 230400 {
			t.Fatalf("custom baud rate did not reach the Web Serial port: %s", result.JSON("", ""))
		}
		if len(result.Get("signals").Arr()) == 0 || !result.Get("signals").Get("0").Get("dataTerminalReady").Bool() || !result.Get("signals").Get("0").Get("requestToSend").Bool() {
			t.Fatalf("saved DTR/RTS state was not applied on connect: %s", result.JSON("", ""))
		}
		if result.Get("consoleEnding").Str() != "lf" || result.Get("write").JSON("", "") != "[115,97,118,101,100,32,101,110,100,105,110,103,10]" {
			t.Fatalf("saved line ending did not reach the console after reconnect: %s", result.JSON("", ""))
		}
	})

	t.Run("delayed port refresh preserves the unsaved editor draft", func(t *testing.T) {
		page := newQuickConnectSerialPage(t, browser, "standard", 1200, 760)
		defer page.Close()

		result := page.MustEval(`async()=>{
            serialTest.app.dispose();
            const list=document.createElement('aside'),content=document.createElement('main');
            list.id='serial-draft-list';content.id='serial-draft-content';document.body.append(list,content);
            serialTest.delayPortRefresh=false;serialTest.portRefreshCalls=0;serialTest.portRefreshCompleted=0;
            serialTest.api=async(url,options={})=>{
                serialTest.requests.push([url,options]);
                if(url==='/api/desktop/serial/ports'){
                    serialTest.portRefreshCalls++;
                    const result=serialTest.delayPortRefresh
                        ? await new Promise(resolve=>serialTest.releasePorts=()=>resolve({ports:[{name:'COM3',busy:false}]}))
                        : {ports:[{name:'COM3',busy:false}]};
                    serialTest.portRefreshCompleted++;return result;
                }
                if(url==='/api/desktop/settings')return {settings:{'quick_connect.serial_profiles':JSON.stringify({version:1,profiles:serialTest.profiles})}};
                return {};
            };
            const app=serialTest.makeApp(list,content);app.render();
            await serialTest.wait(()=>serialTest.portRefreshCompleted===1);
            list.querySelector('[data-serial-profile="browser"]').click();
            const form=content.querySelector('[data-serial-profile-form]');
            form.elements.name.value='Unsaved host draft';
            form.elements.source.value='host';form.elements.source.dispatchEvent(new Event('change',{bubbles:true}));
            await serialTest.wait(()=>form.elements.port.querySelector('option[value="COM3"]:not([disabled])'));
            form.elements.port.value='COM3';
            form.elements.baud_preset.value='custom';form.elements.baud_preset.dispatchEvent(new Event('change',{bubbles:true}));
            form.elements.baud_custom.value='460800';
            form.elements.dtr.checked=true;form.elements.rts.checked=true;form.elements.line_ending.value='crlf';
            serialTest.delayPortRefresh=true;
            form.querySelector('[data-serial-refresh-ports]').click();
            await serialTest.wait(()=>typeof serialTest.releasePorts==='function');
            const during={name:form.elements.name.value,source:form.elements.source.value,baud:form.elements.baud_custom.value,dtr:form.elements.dtr.checked,rts:form.elements.rts.checked,ending:form.elements.line_ending.value};
            serialTest.releasePorts();
            await serialTest.wait(()=>serialTest.portRefreshCompleted===2);
            const current=content.querySelector('[data-serial-profile-form]');
            const after={name:current.elements.name.value,source:current.elements.source.value,baud:current.elements.baud_custom.value,dtr:current.elements.dtr.checked,rts:current.elements.rts.checked,ending:current.elements.line_ending.value,port:current.elements.port.value};
            app.dispose();return {during,after};
        }`)
		for _, stage := range []string{"during", "after"} {
			got := result.Get(stage)
			if got.Get("name").Str() != "Unsaved host draft" || got.Get("source").Str() != "host" || got.Get("baud").Str() != "460800" || !got.Get("dtr").Bool() || !got.Get("rts").Bool() || got.Get("ending").Str() != "crlf" {
				t.Fatalf("%s port refresh changed the unsaved editor draft: %s", stage, result.JSON("", ""))
			}
		}
		if result.Get("after").Get("port").Str() != "COM3" {
			t.Fatalf("refreshed host port selection was not retained: %s", result.JSON("", ""))
		}
	})
}
