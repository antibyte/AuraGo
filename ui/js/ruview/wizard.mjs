const resourceQuery = new URL(import.meta.url).search;
const [{ Onboarding, privateIPv4, validateConfig, revision }, { flash, serialAvailable }] = await Promise.all([
    import('./protocol.mjs' + resourceQuery), import('./flash.mjs' + resourceQuery)
]);

export async function openWizard({base='/api/ruview/',t,receiverIP='',port}={}){
    const tr=key=>t('ruview.'+key),dialog=document.createElement('dialog');dialog.className='ruview-wizard';dialog.setAttribute('aria-label','RuView · ESP32');
    const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    let stopped=false,busy=false,serial=null,identity=null,expected=null,controller=new AbortController();
    dialog.innerHTML=`<form method="dialog"><header><h2>RuView · ESP32</h2><button aria-label="${esc(tr('close'))}" value="close">×</button></header></form>
        <ol class="ruview-steps"><li>${esc(tr('firmware'))}</li><li>${esc(tr('configure'))}</li><li>${esc(tr('verify'))}</li></ol>
        <p>${esc(tr('prerequisites'))}</p><p class="ruview-feedback" role="status" aria-live="polite"></p>
        <section><h3>${esc(tr('step_firmware'))}</h3><label>${esc(tr('profile'))}<select name="profile" aria-describedby="ruview-profile-help"></select></label><p id="ruview-profile-help" class="ruview-help">${esc(tr('profile_help'))}</p><label class="ruview-check"><input type="checkbox" name="erase"><span>${esc(tr('erase'))}</span></label><button type="button" data-action="flash">${esc(tr('flash'))}</button><div class="ruview-flash-progress" hidden><progress max="100" aria-label="${esc(tr('firmware'))}" aria-describedby="ruview-flash-status"></progress><output aria-live="off"></output></div><p id="ruview-flash-status" class="ruview-feedback" role="status" aria-live="polite"></p><button type="button" data-flash-next hidden>${esc(tr('flash_continue'))}</button></section>
        <section><h3>${esc(tr('step_configure'))}</h3><p class="ruview-help">${esc(tr('step_configure_help'))}</p><label>${esc(tr('ssid'))}<input name="ssid" autocomplete="off" spellcheck="false"></label><label>${esc(tr('password'))}<input name="password" type="password" autocomplete="new-password"></label><label class="ruview-check"><input type="checkbox" name="keep"><span>${esc(tr('keep_wifi'))}</span></label>
        <p id="ruview-receiver-help" class="ruview-help">${esc(tr('receiver_purpose'))}</p>
        <div class="ruview-fields"><label>${esc(tr('node'))}<input name="node" type="number" min="1" max="255" value="1"></label><label>${esc(tr('receiver'))}<input name="receiver" value="${esc(receiverIP)}" inputmode="decimal" autocomplete="off" spellcheck="false" aria-describedby="ruview-receiver-help ruview-receiver-lookup"></label><label>${esc(tr('udp_port'))}<input name="port" type="number" min="1" max="65535" value="${esc(port??5005)}"></label></div>
        <div class="ruview-receiver-suggestions" hidden><label>${esc(tr('receiver_candidates'))}<select name="receiver_choice"></select></label><button type="button" data-use-receiver>${esc(tr('use_receiver').replace('{ip}',''))}</button></div>
        <p id="ruview-receiver-lookup" class="ruview-help" aria-live="polite">${esc(tr('receiver_loading'))}</p>
        <p class="ruview-udp-warning ruview-feedback" hidden>${esc(tr('receiver_udp_disabled'))} <a href="/config#ruview" target="_blank" rel="noopener">${esc(tr('settings'))}</a></p>
        <div class="ruview-actions"><button type="button" data-action="configure">${esc(tr('configure'))}</button><button type="button" data-action="readback" aria-describedby="ruview-readback-reason">${esc(tr('readback'))}</button></div><p id="ruview-readback-reason" class="ruview-help">${esc(tr('configure_first'))}</p><p class="ruview-feedback" role="status" aria-live="polite"></p></section>
        <section><h3>${esc(tr('step_verify'))}</h3><button type="button" data-action="verify" aria-describedby="ruview-verify-reason">${esc(tr('verify'))}</button><p id="ruview-verify-reason" class="ruview-help">${esc(tr('confirm_first'))}</p><p class="ruview-feedback" role="status" aria-live="polite"></p><p class="ruview-help">${esc(tr('privacy'))}</p></section>`;
    (document.fullscreenElement||document.body).append(dialog);dialog.showModal();
    const get=n=>dialog.querySelector(`[name="${n}"]`),message=dialog.querySelector('[role="status"]'),buttons=[...dialog.querySelectorAll('[data-action]')];
    const flashProgress=dialog.querySelector('.ruview-flash-progress'),progress=flashProgress.querySelector('progress'),percentage=flashProgress.querySelector('output'),next=dialog.querySelector('[data-flash-next]');
    let activeMessage=message;
    next.addEventListener('click',()=>{if(busy||stopped)return;const target=get(get('keep').checked?'receiver':'ssid');target.focus({preventScroll:true});target.closest('section').scrollIntoView({block:'start'});});
    buttons.forEach(button=>button.disabled=true);
    for(const name of ['receiver','port'])get(name).addEventListener('input',()=>get(name).dataset.edited='true');
    get('keep').addEventListener('change',()=>{for(const name of ['ssid','password'])get(name).disabled=get('keep').checked;});
    const useReceiver=dialog.querySelector('[data-use-receiver]');
    const updateReceiverChoice=()=>{const ip=get('receiver_choice').value;useReceiver.disabled=!privateIPv4(ip);useReceiver.textContent=tr('use_receiver').replace('{ip}',ip);};
    get('receiver_choice').addEventListener('change',updateReceiverChoice);
    useReceiver.addEventListener('click',()=>{const ip=get('receiver_choice').value;if(!privateIPv4(ip))return;get('receiver').value=ip;get('receiver').dataset.edited='true';get('receiver').focus();});
    const feedback=(key,detail='')=>{activeMessage.textContent=tr(key)+(detail?' · '+detail:'');activeMessage.dataset.state=key==='error'?'error':['flashed','reconnect','configured','received'].includes(key)?'success':'';if(activeMessage!==message)activeMessage.scrollIntoView({block:'nearest'});};
    const flashPhase=phase=>{
        if(stopped)return;
        flashProgress.hidden=false;percentage.textContent='';
        if(phase==='writing'){progress.value=0;percentage.textContent='0 %';}else progress.removeAttribute('value');
        feedback('flash_'+phase);
    };
    const explain=error=>{
        if(error.fromAPI)return error.message;
        if(error.message==='wrong_flash')return tr('flash_size_mismatch').replace('{detected}',error.detectedFlash).replace('{expected}',error.expectedFlash);
        const code=({NotFoundError:'cancelled',AbortError:'cancelled',SecurityError:'serial_unsupported',InvalidStateError:'serial_timeout',NetworkError:'serial_disconnected'})[error.name]||error.message;
        const key='ruview.'+code,value=t(key);
        if(value!==key)return value;
        const detail=String(error.message||error.name||'').slice(0,240);
        return tr(error.ruviewStage||'verification_failed')+(detail?' · '+detail:'');
    };
    const completed=index=>{const step=dialog.querySelectorAll('.ruview-steps li')[index];step.classList.add('complete');step.setAttribute('aria-label',step.textContent+' ✓');};
    const cleanup=()=>{stopped=true;controller.abort();serial?.close();get('ssid').value='';get('password').value='';identity=null;expected=null;clearInterval(policyTimer);window.removeEventListener('aurago:auth-ended',close);window.removeEventListener('aurago:desktop-policy',policy);dialog.remove();};
    const close=()=>dialog.close();const policy=e=>{if(e.detail?.readonly||e.detail?.read_only)close();};
    dialog.addEventListener('close',cleanup,{once:true});window.addEventListener('aurago:auth-ended',close);window.addEventListener('aurago:desktop-policy',policy);
    async function request(route,method='GET') {const response=await fetch(base+route,{method,signal:controller.signal});const body=await response.json().catch(()=>({}));if(!response.ok){const error=Error(body.error||body.message||('HTTP '+response.status));error.fromAPI=true;error.httpStatus=response.status;throw error;}return body;}
    async function allowed(){const status=await request('status');if(typeof status.enabled!=='boolean'||typeof status.readonly!=='boolean')throw Error('invalid_reply');if(!status.enabled||status.readonly)throw Error('readonly');return status;}
    async function loadBoardSetup(){
        const hint=dialog.querySelector('#ruview-receiver-lookup');
        try{
            const info=await request('board-setup');if(stopped)return;
            if(!receiverIP&&!get('receiver').dataset.edited&&privateIPv4(info.receiver_ip))get('receiver').value=info.receiver_ip;
            if(port==null&&!get('port').dataset.edited&&Number.isInteger(info.udp_port)&&info.udp_port>0&&info.udp_port<=65535)get('port').value=info.udp_port;
            const candidates=[...new Set((info.candidates||[]).filter(ip=>typeof ip==='string'&&privateIPv4(ip)))];
            get('receiver_choice').innerHTML=(candidates.length>1?`<option value="">${esc(tr('receiver_choose'))}</option>`:'')+candidates.map(ip=>`<option value="${esc(ip)}">${esc(ip)}</option>`).join('');
            dialog.querySelector('.ruview-receiver-suggestions').hidden=!candidates.length;updateReceiverChoice();
            hint.textContent=tr(candidates.length?'receiver_detected_help':'receiver_manual_help');
            dialog.querySelector('.ruview-udp-warning').hidden=info.mode==='external'||info.udp_enabled!==false;
        }catch(error){if(!stopped)hint.textContent=tr('receiver_lookup_failed')+' '+tr('receiver_manual_help');}
    }
    const policyTimer=setInterval(()=>allowed().then(()=>{
        if(!stopped&&message.textContent===tr('connection_interrupted'))message.textContent=busy?'':tr('ready');
    }).catch(error=>{
        if(stopped)return;
        if(error.message==='readonly'||[401,403].includes(error.httpStatus))close();
        else message.textContent=tr('connection_interrupted');
    }),3000);
    let catalog;
    try{await allowed();if(stopped)return close;await Promise.all([loadBoardSetup(),request('firmware').then(value=>catalog=value)]);if(stopped)return close;get('profile').innerHTML=catalog.firmware.map(p=>`<option value="${esc(p.id)}">${esc(p.id)}</option>`).join('');if(!serialAvailable())throw Error('serial_unsupported');feedback(catalog.firmware.length?'ready':'firmware_missing');}
    catch(error){feedback('error',explain(error));buttons.forEach(b=>b.disabled=true);return close;}
    const enableButtons=()=>{
        for(const button of buttons){
            const action=button.dataset.action;
            button.disabled=(action==='flash'&&!catalog.firmware.length)||(action==='readback'&&(!expected||!identity))||(action==='verify'&&!expected?.confirmed);
        }
        dialog.querySelector('#ruview-readback-reason').hidden=!!expected&&!!identity;
        dialog.querySelector('#ruview-verify-reason').hidden=!!expected?.confirmed;
    };
    enableButtons();
    dialog.addEventListener('click',async event=>{
        const button=event.target.closest('[data-action]');if(!button||button.disabled||busy)return;
        activeMessage=button.closest('section').querySelector('[role="status"]');
        const action=button.dataset.action;
        const selection=action==='flash'?{id:get('profile').value,erase:get('erase').checked}:null;
        if(message.textContent===tr('ready'))message.textContent='';
        busy=true;buttons.forEach(b=>b.disabled=true);next.disabled=true;button.setAttribute('aria-busy','true');feedback('working');
        if(action==='flash'){next.hidden=true;flashProgress.hidden=false;progress.removeAttribute('value');percentage.textContent='';}
        try{
            // requestPort remains in the trusted click stack, before network awaits.
            let selected;
            if(action==='flash'||action==='configure'||action==='readback')selected=await navigator.serial.requestPort();
            await allowed();if(stopped)return;feedback('working');
            if(action==='flash'){
                identity=null;expected=null;dialog.querySelectorAll('.ruview-steps li').forEach(step=>{step.classList.remove('complete');step.removeAttribute('aria-label');});
                // Long-lived dialogs must not keep metadata from before a server repair.
                const current=await request('firmware');if(stopped)return;
                const profile=current?.revision===revision&&Array.isArray(current.firmware)?current.firmware.find(p=>p?.id===selection.id):null;
                if(!profile)throw Error('invalid_firmware');
                await flash(selected,profile,{erase:selection.erase,signal:controller.signal,onPhase:flashPhase,onProgress:n=>{if(stopped)return;progress.value=n;percentage.textContent=n+' %';}});
                if(stopped)return;
                progress.value=100;percentage.textContent='100 %';completed(0);feedback('flashed');next.hidden=false;next.disabled=false;next.focus({preventScroll:true});next.scrollIntoView({block:'nearest'});
            }else if(action==='configure'){
                const settings=validateConfig({ssid:get('ssid').value,password:get('password').value,keep:get('keep').checked,node_id:Number(get('node').value),target_ip:get('receiver').value.trim(),target_port:Number(get('port').value)});
                identity=null;expected=null;dialog.querySelectorAll('.ruview-steps li').forEach((step,index)=>{if(index>0){step.classList.remove('complete');step.removeAttribute('aria-label');}});
                serial=new Onboarding(selected);try{await serial.open();identity=await serial.configure(settings);if(stopped)return;expected={node_id:settings.node_id,target_ip:settings.target_ip,target_port:settings.target_port};get('ssid').value='';get('password').value='';feedback('reconnect');}finally{settings.ssid='';settings.password='';await serial.close();serial=null;}
            }else if(action==='readback'){
                if(!expected||!identity)throw Error('configure_first');serial=new Onboarding(selected);try{await serial.open();const h=await serial.hello();if(h.device_digest!==identity.device_digest||h.chip!==identity.chip||h.node_id!==String(expected.node_id)||h.target_ip!==expected.target_ip||h.target_port!==String(expected.target_port)||h.configured!=='1')throw Error('readback_failed');expected.confirmed=true;completed(1);feedback('configured');}finally{await serial.close();serial=null;}
            }else if(action==='verify'){
                if(!expected?.confirmed)throw Error('configure_first');const status=await request('test','POST');const data=await request('proxy/api/v1/nodes');const nodes=Array.isArray(data)?data:(data.nodes||[]);const node=nodes.find(n=>Number(n.node_id??n.id)===expected.node_id);if(!status.fresh||status.simulated||status.signal!=='CSI'||!node||node.csi_status!=='active'||!Number.isFinite(node.csi_last_seen_ms)||node.csi_last_seen_ms>10000||node.csi_sequence==null)throw Error('no_frames');completed(2);feedback('received');
            }
        }catch(error){if(!stopped){if(action==='flash')flashProgress.hidden=true;feedback('error',explain(error));}}
        finally{busy=false;button.setAttribute('aria-busy','false');next.disabled=false;if(!stopped)enableButtons();}
    });
    return close;
}
