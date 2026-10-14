import assert from 'node:assert/strict';

// Render real upstream components against the pinned response contracts.
export async function checkControls(frame) {
    await frame.evaluate(async () => {
        const {apiService}=await import('./services/api.service.js');
        const {trainingService}=await import('./services/training.service.js');
        const {default:TrainingPanel}=await import('./components/TrainingPanel.js');
        const original={get:apiService.get,post:apiService.post,delete:apiService.delete};
        const state={requests:[],failRecording:false,failRead:false,readStatus:503,enabled:true,empty:false,failTraining:false};
        apiService.get=async path=>{
            if(state.failRead)throw Object.assign(Error('offline'),{status:state.readStatus});
            if(path==='/api/v1/recording/list')return {recordings:state.empty?[]:[{id:'rec_fixture',name:'Fixture recording'}]};
            if(path==='/api/v1/train/status')return {active:false,enabled:state.enabled};
            if(path==='/api/v1/models')return {models:state.empty?[]:[{id:'model_fixture',name:'Fixture model'}]};
            if(path==='/api/v1/models/active')return null;
            if(path==='/api/v1/models/lora/profiles')return {profiles:[]};
            return original.get.call(apiService,path);
        };
        apiService.post=async(path,body)=>{
            state.requests.push({path,body});
            if(path.includes('/recording/'))return state.failRecording?{success:false,error:'Recording denied'}:{success:true,recording_id:'rec_fixture'};
            return state.failTraining?{status:'error',message:'Training denied'}:{status:path.endsWith('/stop')?'stopping':'started'};
        };
        apiService.delete=async()=>({success:false,error:'Delete denied'});
        const host=document.createElement('div');host.id='audit-controls';host.style.cssText='position:fixed;inset:0;overflow:auto;z-index:999999;background:#111827';document.body.appendChild(host);
        const panel=new TrainingPanel(host);
        window.ruviewControlTest={...state,state,panel,host,trainingService,restore(){panel.destroy();host.remove();Object.assign(apiService,original);delete window.ruviewControlTest;}};
    });
    const host=frame.locator('#audit-controls');
    try {
        await host.getByLabel('Training type').waitFor();
        await frame.waitForFunction(()=>window.ruviewControlTest.panel.state.statusLoaded);
        assert.match(await host.innerText(),/Recording status unknown/);
        await frame.waitForFunction(()=>!document.querySelector('#audit-controls [data-ruview-action="recording_stop"]').disabled);
        await host.getByRole('button',{name:'Stop Recording',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.trainingService.recordingState==='idle');
        await frame.evaluate(()=>{window.ruviewControlTest.state.requests.length=0;});
        await host.getByLabel('Fixture recording').check();
        await host.getByLabel('Epochs',{exact:true}).fill('17');
        await host.getByLabel('Early Stop Patience').fill('6');
        await host.getByRole('button',{name:'Start Training',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.state.requests.length===1);
        assert.equal((await frame.evaluate(()=>window.ruviewControlTest.state.requests[0])).body.config.early_stopping_patience,6);
        await host.getByLabel('Training type').selectOption('startPretraining');
        assert.equal(await host.getByLabel('Batch Size').count(),0);
        await host.getByRole('button',{name:'Start Training',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.state.requests.length===2);
        const pretrain=await frame.evaluate(()=>window.ruviewControlTest.state.requests[1]);
        assert.equal(pretrain.body.epochs,17);assert.equal(pretrain.body.config,undefined);
        await host.getByLabel('Training type').selectOption('startLoraTraining');
        await host.getByLabel('Base Model').selectOption('model_fixture');
        assert.equal(await host.getByLabel('Learning Rate').count(),0);
        await host.getByLabel('LoRA Profile').fill('room_fixture');
        await host.getByRole('button',{name:'Start Training',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.state.requests.length===3);
        const lora=await frame.evaluate(()=>window.ruviewControlTest.state.requests[2]);
        assert.equal(lora.body.base_model_id,'model_fixture');assert.equal(lora.body.profile_name,'room_fixture');assert.equal(lora.body.rank,8);
        await host.getByLabel('LoRA Profile').focus();
        await frame.evaluate(()=>window.ruviewControlTest.panel.render());
        assert.equal(await host.getByLabel('LoRA Profile').evaluate(el=>el===document.activeElement),true);
        await frame.evaluate(()=>{window.ruviewControlTest.state.failTraining=true;});
        await host.getByRole('button',{name:'Start Training',exact:true}).click();
        await frame.waitForFunction(()=>document.querySelector('#audit-controls [role=alert]')?.textContent.includes('Training denied'));
        await frame.evaluate(async()=>{window.ruviewControlTest.state.enabled=false;await window.ruviewControlTest.panel.refresh();});
        assert.equal(await host.getByRole('button',{name:'Start Training',exact:true}).isDisabled(),true);
        await host.getByRole('button',{name:'Start Recording',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.trainingService.recordingState==='recording');
        assert.equal(await host.getByRole('button',{name:'Start Recording',exact:true}).isDisabled(),true);
        assert.equal(await frame.locator('#training-status-badge').innerText(),'Recording','Live and Training share recording state');
        await host.getByRole('button',{name:'Stop Recording',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.trainingService.recordingState==='idle');
        await frame.evaluate(()=>{window.ruviewControlTest.state.failRecording=true;});
        await host.getByRole('button',{name:'Start Recording',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.trainingService.recordingState==='unknown');
        assert.match(await host.innerText(),/Recording denied/);
        await frame.evaluate(()=>{window.ruviewControlTest.state.failRecording=false;});
        await host.getByRole('button',{name:'Stop Recording',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.trainingService.recordingState==='idle');
        await host.getByRole('button',{name:'Delete',exact:true}).click();
        await frame.waitForFunction(()=>document.querySelector('#audit-controls [role=alert]')?.textContent.includes('Delete denied'));
        await frame.evaluate(async()=>{window.ruviewControlTest.state.failRead=true;await window.ruviewControlTest.panel.refresh();});
        assert.match(await host.innerText(),/Status unknown/);assert.match(await host.innerText(),/Previously loaded data may be stale/);
        assert.equal(await host.getByRole('button',{name:'Start Training',exact:true}).isDisabled(),true);
        assert.equal(await host.getByRole('button',{name:'Stop Recording',exact:true}).isEnabled(),true);
        assert.equal(await host.getByRole('button',{name:'Stop Training',exact:true}).isEnabled(),true);
        // Field labels must name every numeric/text/select input in the real DOM.
        assert.equal(await host.locator('input,select').evaluateAll(inputs=>inputs.every(input=>input.id&&input.labels?.length>0)&&new Set(inputs.map(input=>input.id)).size===inputs.length),true);
        await host.screenshot({path:'reports/ruview-browser/training-stale.png'});
        await frame.evaluate(async()=>{const test=window.ruviewControlTest;test.state.readStatus=404;await test.panel.refresh();});
        assert.match(await host.innerText(),/Not supported by this service/);
        await frame.evaluate(async()=>{
            const test=window.ruviewControlTest;test.state.failRead=false;test.state.empty=true;await test.panel.refresh();
        });
        assert.match(await host.innerText(),/Start recording CSI data/);
        await frame.evaluate(async()=>{
            const test=window.ruviewControlTest;test.panel.destroy();test.state.empty=false;
            const {default:ModelPanel}=await import('./components/ModelPanel.js');test.panel=new ModelPanel(test.host);
        });
        await frame.waitForFunction(()=>window.ruviewControlTest.panel.state.modelsLoaded);
        assert.equal(await host.getByRole('button',{name:'Load',exact:true}).isDisabled(),true,'actual short model button uses capabilities');
        await host.getByRole('button',{name:'Delete',exact:true}).click();
        await frame.waitForFunction(()=>document.querySelector('#audit-controls [role=alert]')?.textContent.includes('Delete denied'));
        await frame.evaluate(async()=>{const test=window.ruviewControlTest;test.state.failRead=true;test.state.readStatus=503;await test.panel.refresh();});
        assert.match(await host.innerText(),/previously loaded data may be stale/i);
        assert.match(await host.innerText(),/model_fixture/);
        assert.equal(await host.getByRole('button',{name:'Delete',exact:true}).isDisabled(),true);
        await frame.evaluate(async()=>{const test=window.ruviewControlTest;test.state.readStatus=404;await test.panel.refresh();});
        assert.match(await host.innerText(),/Not supported by this service/);
        await frame.evaluate(async()=>{const test=window.ruviewControlTest;test.state.failRead=false;test.state.empty=true;await test.panel.refresh();});
        assert.match(await host.innerText(),/No .rvf models found/);
    } finally { await frame.evaluate(()=>window.ruviewControlTest?.restore()); }
}
