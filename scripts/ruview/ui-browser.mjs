import assert from 'node:assert/strict';

export async function checkDevices(page,fixture) {
    let release;
    fixture.hold=new Promise(resolve=>{release=resolve;});
    fixture.statusFail=true;
    try {
        const age=Number(await page.locator('[data-age]').getAttribute('data-age-ms'));
        await page.waitForFunction(age=>Number(document.querySelector('[data-age]')?.dataset.ageMs)>age+500,age);
        await page.waitForFunction(()=>Number(document.querySelector('[data-age]')?.dataset.ageMs)>10000,{},{timeout:16000});
        assert.match(await page.locator('[data-fresh]').innerText(),/Veraltet/i);
        assert.equal(fixture.max,1,'device polling must not overlap');
        fixture.fail=true;release();fixture.hold=null;
        await page.waitForFunction(()=>document.querySelector('.ruview-feedback')?.textContent.includes('Aktualisierung fehlgeschlagen'));
        assert.equal(await page.locator('.ruview-node').count(),1,'failed refresh retains old values');
        fixture.fail=false;fixture.statusFail=false;
        await page.waitForFunction(()=>document.querySelector('.ruview-feedback')?.textContent==='');
        fixture.hold=new Promise(resolve=>{release=resolve;});
        await page.waitForRequest(request=>request.url().endsWith('/ruview/nodes'));
        await page.locator('[data-tab=overview]').click();await page.locator('[data-action=test]').waitFor();
        await page.locator('[data-tab=devices]').click();await page.waitForTimeout(4200);
        assert.equal(fixture.max,1,'returning to devices must not overlap a previous request');
        release();fixture.hold=null;
        await page.waitForFunction(()=>Number(document.querySelector('[data-age]')?.dataset.ageMs)<1000);
        fixture.offline=true;
        await page.waitForFunction(()=>document.querySelector('[data-fresh]')?.textContent==='Keine aktuellen Sensordaten');
        assert.equal(await page.locator('.ruview-node').count(),1,'empty upstream after restart retains known boards');
        assert.match(await page.locator('.ruview-node').innerText(),/-51/,'last RSSI remains explicitly offline');
        fixture.unavailable=true;
        await page.waitForFunction(()=>document.querySelector('.ruview-feedback')?.textContent.includes('Aktualisierung fehlgeschlagen'));
        assert.equal(await page.locator('.ruview-node').count(),1,'unreachable upstream retains known boards');
        await page.evaluate(()=>render());await page.locator('[data-action=test]').waitFor();
        await page.locator('[data-tab=devices]').click();await page.locator('.ruview-node').waitFor();
        assert.equal(await page.locator('[data-age]').innerText(),'—','reopening restores identity, never fake historical measurements');
        fixture.offline=false;fixture.unavailable=false;
        await page.waitForFunction(()=>document.querySelector('[data-fresh]')?.textContent==='Frische Sensordaten empfangen');
        fixture.offline=true;fixture.scope='different-receiver';
        await page.waitForFunction(()=>document.querySelector('[data-age]')?.textContent==='—');
        assert.ok(!(await page.locator('.ruview-node').innerText()).includes('-51'),'another receiver must not inherit previous measurements');
        fixture.offline=false;fixture.scope='fixture';
        await page.waitForFunction(()=>document.querySelector('[data-fresh]')?.textContent==='Frische Sensordaten empfangen');
        await page.evaluate(()=>Object.defineProperty(document,'hidden',{configurable:true,value:true}));
        const reads=fixture.reads;await page.waitForTimeout(4200);
        assert.equal(fixture.reads,reads,'hidden document pauses polling');
        await page.evaluate(()=>delete document.hidden);
        await page.waitForFunction(()=>document.querySelector('[data-fresh]')?.textContent==='Frische Sensordaten empfangen');
        await page.locator('#app').evaluate(el=>el.style.display='none');
        const hiddenReads=fixture.reads;await page.waitForTimeout(4200);
        assert.equal(fixture.reads,hiddenReads,'hidden desktop window pauses polling');
        await page.locator('#app').evaluate(el=>el.style.display='');
        await page.locator('[data-tab=overview]').click();await page.locator('[data-action=test]').waitFor();
        const leftReads=fixture.reads;await page.waitForTimeout(4200);
        assert.ok(fixture.reads>leftReads,'overview checklist refreshes known boards');
        assert.equal(fixture.max,1,'overview refresh does not overlap');
        await page.locator('[data-tab=devices]').click();await page.locator('.ruview-node').waitFor();
        await page.evaluate(()=>RuViewApp.dispose('test'));
        const closedReads=fixture.reads,closedAge=await page.locator('[data-age]').innerText();await page.waitForTimeout(4200);
        assert.equal(fixture.reads,closedReads);assert.equal(await page.locator('[data-age]').innerText(),closedAge,'disposed app has no age timer');
        await page.evaluate(()=>render());await page.locator('[data-action=test]').waitFor();
        await page.locator('[data-tab=devices]').click();await page.locator('.ruview-node').waitFor();
    } finally {release();fixture.hold=null;fixture.fail=false;fixture.statusFail=false;fixture.offline=false;fixture.unavailable=false;fixture.scope='fixture';await page.evaluate(()=>delete document.hidden);}
}

// Overview/Devices wording for states that are not failures (German locale, desktop overview already open).
export async function checkReceptionStates(page,fixture,setSource) {
    const summary=()=>page.locator('.ruview-summary article');
    try {
        // The 6th card must carry its own translated title, not a missing key or the generic fallback.
        assert.equal(await summary().nth(5).evaluate(card=>card.firstChild.textContent),'Aufnahme');

        // Wi-Fi/demo sources have no board list: neutral checks, no "refresh failed".
        setSource('wifi');fixture.liveState='not_applicable';
        await page.evaluate(()=>render());await page.locator('.ruview-check').waitFor();
        assert.equal(await page.locator('.ruview-check [data-state="attention"]').count(),0,'a source without boards has nothing that needs attention');
        assert.equal(await page.locator('.ruview-check li').nth(3).getAttribute('data-state'),'neutral');
        await page.locator('[data-tab=devices]').click();await page.locator('.ruview-node').waitFor();
        await page.waitForFunction(()=>document.querySelector('.ruview-feedback')?.textContent==='Die Board-Liste wird nur mit ESP32-Boards verwendet.');
        assert.ok(!(await page.locator('#app').innerText()).includes('Aktualisierung fehlgeschlagen'));
        setSource('esp32');fixture.liveState='';

        // A disabled integration refuses the board list by design (403): unchecked, never "needs attention" or "refresh failed".
        fixture.disabled=true;
        await page.evaluate(()=>render());await page.locator('.ruview-check').waitFor();
        assert.equal(await page.locator('.ruview-check [data-state="attention"]').count(),0,'a disabled integration has nothing that needs attention');
        assert.equal(await page.locator('.ruview-check li').nth(3).getAttribute('data-state'),'unchecked');
        assert.equal(await page.locator('.ruview-check li').nth(4).getAttribute('data-state'),'unchecked');
        await page.locator('[data-tab=devices]').click();
        await page.waitForFunction(()=>document.querySelector('.ruview-feedback')?.textContent==='Deaktiviert');
        assert.ok(!(await page.locator('#app').innerText()).includes('Aktualisierung fehlgeschlagen'));
        fixture.disabled=false;

        // The first status request failing says "unavailable", never "disabled".
        fixture.statusFail=true;
        await page.evaluate(()=>render());await page.locator('.ruview-summary').waitFor();
        assert.equal(await page.locator('.ruview-summary article strong').first().innerText(),'Status nicht abrufbar');
        assert.ok(!(await page.locator('#app').innerText()).includes('Deaktiviert'),'an unknown status must not read as disabled');
        assert.equal(await page.locator('#ruview-action-reason').innerText(),'Status nicht abrufbar');
        fixture.statusFail=false;
        await page.waitForFunction(()=>document.querySelector('.ruview-summary article strong')?.textContent==='Bereit');
    } finally {setSource('esp32');fixture.liveState='';fixture.disabled=false;fixture.statusFail=false;}
}

export async function checkPolicy(frame,setReadonly) {
    await frame.evaluate(()=>{
        const host=document.createElement('div');host.id='policy-fixture';
        for(const [id,action,disabled] of [['start','training_start',true],['stop','training_stop',false],['load','model_load',false]]){
            const button=document.createElement('button');button.id='policy-'+id;button.dataset.ruviewAction=action;button.disabled=disabled;button.textContent='Localized label';host.append(button);
        }
        document.body.append(host);
    });
    try {
        setReadonly(true);
        await frame.waitForFunction(()=>document.getElementById('policy-start').disabled&&!document.getElementById('policy-stop').disabled);
        assert.equal(await frame.locator('#policy-load').isDisabled(),true);
        assert.equal(await frame.evaluate(async()=>{try{await fetch('/api/v1/recording/start',{method:'POST'});return false}catch{return true}}),true);
        assert.equal(await frame.evaluate(async()=>{try{await fetch('/api/v1/recording/rec_fixture',{method:'DELETE'});return false}catch{return true}}),true);
        assert.equal(await frame.evaluate(async()=>(await fetch('/api/v1/recording/stop',{method:'POST'})).ok),true);
        assert.equal(await frame.evaluate(async()=>(await fetch('/api/v1/train/stop',{method:'POST'})).ok),true);
        setReadonly(false);await frame.waitForTimeout(3400);
        assert.equal(await frame.locator('#policy-start').isDisabled(),true,'permission changes preserve business-disabled training');
        await frame.evaluate(()=>document.getElementById('policy-start').disabled=false);
        assert.equal(await frame.locator('#policy-start').isDisabled(),false);
        setReadonly(true);await frame.waitForFunction(()=>document.getElementById('policy-start').disabled);
        await frame.evaluate(()=>document.getElementById('policy-start').disabled=false);
        assert.equal(await frame.locator('#policy-start').isDisabled(),true,'component update cannot override read-only');
    } finally {setReadonly(false);await frame.evaluate(()=>document.getElementById('policy-fixture').remove());}
}
