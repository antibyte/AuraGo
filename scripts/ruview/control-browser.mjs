import assert from 'node:assert/strict';

// Render real upstream components against the pinned response contracts.
export async function checkControls(frame) {
    await frame.evaluate(async () => {
        const {apiService}=await import('./services/api.service.js');
        const {trainingService}=await import('./services/training.service.js');
        const {default:TrainingPanel}=await import('./components/TrainingPanel.js');
        const original={get:apiService.get,post:apiService.post,delete:apiService.delete};
        const state={requests:[],failRecording:false,failRead:false};
        apiService.get=async path=>{
            if(state.failRead)throw Object.assign(Error('offline'),{status:503});
            if(path==='/api/v1/recording/list')return {recordings:[{id:'rec_fixture',name:'Fixture recording'}]};
            if(path==='/api/v1/train/status')return {active:false,enabled:true};
            if(path==='/api/v1/models')return {models:[{id:'model_fixture',name:'Fixture model'}]};
            return original.get.call(apiService,path);
        };
        apiService.post=async(path,body)=>{
            state.requests.push({path,body});
            if(path.includes('/recording/'))return state.failRecording?{success:false,error:'Recording denied'}:{success:true,recording_id:'rec_fixture'};
            return {status:path.endsWith('/stop')?'stopping':'started'};
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
        await host.getByRole('button',{name:'Start Recording',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.trainingService.recordingState==='recording');
        assert.equal(await host.getByRole('button',{name:'Start Recording',exact:true}).isDisabled(),true);
        await host.getByRole('button',{name:'Stop Recording',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.trainingService.recordingState==='idle');
        await frame.evaluate(()=>{window.ruviewControlTest.state.failRecording=true;});
        await host.getByRole('button',{name:'Start Recording',exact:true}).click();
        await frame.waitForFunction(()=>window.ruviewControlTest.trainingService.recordingState==='unknown');
        assert.match(await host.innerText(),/Recording denied/);
        await frame.evaluate(async()=>{window.ruviewControlTest.state.failRead=true;await window.ruviewControlTest.panel.refresh();});
        assert.match(await host.innerText(),/Status unknown/);assert.match(await host.innerText(),/Previously loaded data may be stale/);
        assert.equal(await host.getByRole('button',{name:'Start Training',exact:true}).isDisabled(),true);
        assert.equal(await host.getByRole('button',{name:'Stop Recording',exact:true}).isEnabled(),true);
        assert.equal(await host.getByRole('button',{name:'Stop Training',exact:true}).isEnabled(),true);
        // Field labels must name every numeric/text/select input in the real DOM.
        assert.equal(await host.locator('input:not([type=checkbox]),select').evaluateAll(inputs=>inputs.every(input=>input.labels?.length>0)),true);
    } finally { await frame.evaluate(()=>window.ruviewControlTest?.restore()); }
}
