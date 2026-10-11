import { Onboarding, validateConfig } from './protocol.mjs';
import { flash, serialAvailable } from './flash.mjs';

export async function openWizard({base='/api/ruview/',t,receiverIP='',port=5005}={}){
    const tr=key=>t('ruview.'+key),dialog=document.createElement('dialog');dialog.className='ruview-wizard';
    const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    let stopped=false,busy=false,serial=null,identity=null,expected=null,controller=new AbortController();
    dialog.innerHTML=`<form method="dialog"><header><h2>RuView · ESP32</h2><button aria-label="${esc(tr('close'))}" value="close">×</button></header></form>
        <ol class="ruview-steps"><li>${esc(tr('firmware'))}</li><li>${esc(tr('configure'))}</li><li>${esc(tr('verify'))}</li></ol>
        <p>${esc(tr('prerequisites'))}</p><p class="ruview-feedback" role="status" aria-live="polite"></p>
        <section><label>${esc(tr('profile'))}<select name="profile"></select></label><label class="ruview-check"><input type="checkbox" name="erase">${esc(tr('erase'))}</label><button type="button" data-action="flash">${esc(tr('flash'))}</button><progress max="100" value="0"></progress></section>
        <section><label>${esc(tr('ssid'))}<input name="ssid" autocomplete="off" spellcheck="false"></label><label>${esc(tr('password'))}<input name="password" type="password" autocomplete="new-password"></label><label class="ruview-check"><input type="checkbox" name="keep">${esc(tr('keep_wifi'))}</label><div class="ruview-fields"><label>${esc(tr('node'))}<input name="node" type="number" min="1" max="255" value="1"></label><label>${esc(tr('receiver'))}<input name="receiver" value="${esc(receiverIP)}" placeholder="192.168.1.10"></label><label>UDP<input name="port" type="number" min="1" max="65535" value="${port}"></label></div><button type="button" data-action="configure">${esc(tr('configure'))}</button><button type="button" data-action="readback">${esc(tr('readback'))}</button></section>
        <section><button type="button" data-action="verify">${esc(tr('verify'))}</button><p>${esc(tr('privacy'))}</p></section>`;
    (document.fullscreenElement||document.body).append(dialog);dialog.showModal();
    const get=n=>dialog.querySelector(`[name="${n}"]`),message=dialog.querySelector('[role="status"]'),buttons=[...dialog.querySelectorAll('[data-action]')];
    const feedback=(key,detail='')=>{message.textContent=tr(key)+(detail?' · '+detail:'');};
    const explain=error=>{const code=({NotFoundError:'cancelled',SecurityError:'serial_unsupported',InvalidStateError:'serial_timeout',NetworkError:'serial_disconnected'})[error.name]||error.message;const key='ruview.'+code,value=t(key);return value===key?tr('verification_failed'):value;};
    const completed=index=>{const step=dialog.querySelectorAll('.ruview-steps li')[index];step.classList.add('complete');step.setAttribute('aria-label',step.textContent+' ✓');};
    const cleanup=()=>{stopped=true;controller.abort();serial?.close();get('ssid').value='';get('password').value='';identity=null;expected=null;clearInterval(policyTimer);window.removeEventListener('aurago:auth-ended',close);window.removeEventListener('aurago:desktop-policy',policy);dialog.remove();};
    const close=()=>dialog.close();const policy=e=>{if(e.detail?.readonly||e.detail?.read_only)close();};
    dialog.addEventListener('close',cleanup,{once:true});window.addEventListener('aurago:auth-ended',close);window.addEventListener('aurago:desktop-policy',policy);
    async function request(route,method='GET') {const response=await fetch(base+route,{method,signal:controller.signal});const body=await response.json();if(!response.ok)throw Error(body.message||'request_failed');return body;}
    async function allowed(){const status=await request('status');if(!status.enabled||status.readonly)throw Error('readonly');return status;}
    const policyTimer=setInterval(()=>allowed().catch(()=>{if(!stopped)close();}),3000);
    let catalog;
    try{await allowed();catalog=await request('firmware');get('profile').innerHTML=catalog.firmware.map(p=>`<option value="${esc(p.id)}">${esc(p.id)}</option>`).join('');if(!serialAvailable())throw Error('serial_unsupported');feedback(catalog.firmware.length?'ready':'firmware_missing');}
    catch(error){feedback('error',explain(error));buttons.forEach(b=>b.disabled=true);return close;}
    const enableButtons=()=>buttons.forEach(b=>b.disabled=b.dataset.action==='flash'&&!catalog.firmware.length);
    enableButtons();
    dialog.addEventListener('click',async event=>{
        const button=event.target.closest('[data-action]');if(!button||busy)return;
        busy=true;buttons.forEach(b=>b.disabled=true);
        try{
            // requestPort remains in the trusted click stack, before network awaits.
            const action=button.dataset.action;let selected;
            if(action==='flash'||action==='configure'||action==='readback')selected=await navigator.serial.requestPort();
            await allowed();if(stopped)return;feedback('working');
            if(action==='flash'){
                const profile=catalog.firmware.find(p=>p.id===get('profile').value);
                await flash(selected,profile,{erase:get('erase').checked,signal:controller.signal,onProgress:n=>dialog.querySelector('progress').value=n});completed(0);feedback('flashed');
            }else if(action==='configure'){
                const settings=validateConfig({ssid:get('ssid').value,password:get('password').value,keep:get('keep').checked,node_id:Number(get('node').value),target_ip:get('receiver').value.trim(),target_port:Number(get('port').value)});
                serial=new Onboarding(selected);try{await serial.open();identity=await serial.configure(settings);expected={node_id:settings.node_id,target_ip:settings.target_ip,target_port:settings.target_port};feedback('reconnect');}finally{settings.ssid='';settings.password='';get('ssid').value='';get('password').value='';await serial.close();serial=null;}
            }else if(action==='readback'){
                if(!expected||!identity)throw Error('configure_first');serial=new Onboarding(selected);try{await serial.open();const h=await serial.hello();if(h.device_digest!==identity.device_digest||h.chip!==identity.chip||h.node_id!==String(expected.node_id)||h.target_ip!==expected.target_ip||h.target_port!==String(expected.target_port)||h.configured!=='1')throw Error('readback_failed');expected.confirmed=true;completed(1);feedback('configured');}finally{await serial.close();serial=null;}
            }else if(action==='verify'){
                if(!expected?.confirmed)throw Error('configure_first');const status=await request('test','POST');const data=await request('proxy/api/v1/nodes');const nodes=Array.isArray(data)?data:(data.nodes||[]);const node=nodes.find(n=>Number(n.node_id??n.id)===expected.node_id);if(!status.fresh||status.simulated||status.signal!=='CSI'||!node||node.csi_status!=='active'||!Number.isFinite(node.csi_last_seen_ms)||node.csi_last_seen_ms>10000||node.csi_sequence==null)throw Error('no_frames');completed(2);feedback('received');
            }
        }catch(error){if(!stopped)feedback('error',explain(error));}
        finally{get('ssid').value='';get('password').value='';busy=false;if(!stopped)enableButtons();}
    });
    return close;
}
