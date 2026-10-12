const encoder = new TextEncoder();
export const revision = 'a71634ae8a606bba5463489450c69e8b42de114e';
export function nonce() { return Array.from(crypto.getRandomValues(new Uint8Array(16)),b=>b.toString(16).padStart(2,'0')).join(''); }
export function privateIPv4(value) {
    if (!/^(?:0|[1-9]\d{0,2})(?:\.(?:0|[1-9]\d{0,2})){3}$/.test(value)) return false;
    const p=value.split('.').map(Number);return p.every(n=>n<=255)&&(p[0]===10||(p[0]===172&&p[1]>=16&&p[1]<=31)||(p[0]===192&&p[1]===168));
}
export function validateConfig(c) {
    if (!Number.isInteger(c.node_id)||c.node_id<1||c.node_id>255||!privateIPv4(c.target_ip)||!Number.isInteger(c.target_port)||c.target_port<1||c.target_port>65535) throw Error('invalid_config');
    if (!c.keep) {
        if (typeof c.ssid!=='string'||typeof c.password!=='string'||/[\x00-\x1f\x7f]/.test(c.ssid+c.password)) throw Error('invalid_wifi');
        const ssid=encoder.encode(c.ssid).length,password=encoder.encode(c.password).length;
        if(ssid<1||ssid>32||password<8||password>63)throw Error('invalid_wifi');
    }
    return c;
}
function b64(value){let s='';for(const b of encoder.encode(value))s+=String.fromCharCode(b);return btoa(s);}
export function configCommand(c,n){validateConfig(c);if(!/^[a-f0-9]{32}$/.test(n))throw Error('invalid_nonce');return `RUVIEW_CONFIG_V1 ${n} ${c.node_id} ${c.target_ip} ${c.target_port} ${c.keep?'KEEP':b64(c.ssid)} ${c.keep?'KEEP':b64(c.password)}\n`;}
export function parseReply(line,expectedNonce,prefix='RUVIEW_HELLO_OK_V1'){
    if(line.length>1024||!line.startsWith(prefix+' '))return null;
    const parts=line.trim().split(/\s+/),values={};
    for(const item of parts.slice(1)){const i=item.indexOf('=');if(i<1||Object.hasOwn(values,item.slice(0,i)))throw Error('invalid_reply');values[item.slice(0,i)]=item.slice(i+1);}
    if(values.nonce!==expectedNonce)throw Error('invalid_nonce');
    if(prefix==='RUVIEW_HELLO_OK_V1'&&values.version!=='0.8.12')throw Error('invalid_firmware');
    if(prefix==='RUVIEW_HELLO_OK_V1'&&(!['esp32s3','esp32c6'].includes(values.chip)||!/^\d+\.\d+\.\d+$/.test(values.version)||!/^[a-f0-9]{16}$/i.test(values.device_digest)))throw Error('invalid_reply');
    return values;
}
export async function sha256(bytes){return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),b=>b.toString(16).padStart(2,'0')).join('');}
export function validateProfile(profile){
    const expected=`${profile.chip}-${profile.flash_bytes/1048576}mb-${profile.console}`;
    if(profile.id!==expected)throw Error('invalid_firmware');
    if(!/^esp32s[3]-[48]mb-(uart|cdc)$|^esp32c6-4mb-(uart|cdc)$/.test(profile.id)||!['esp32s3','esp32c6'].includes(profile.chip)||![4194304,8388608].includes(profile.flash_bytes)||!['uart','cdc'].includes(profile.console)||profile.parts.length!==4)throw Error('invalid_firmware');
    let end=0;for(const p of [...profile.parts].sort((a,b)=>a.offset-b.offset)){if(!/^[a-z0-9_.-]+\.bin$/.test(p.name)||!Number.isSafeInteger(p.offset)||p.offset<end||p.offset%4096||!Number.isSafeInteger(p.size)||p.size<=0||p.offset+p.size>profile.flash_bytes||!/^[a-f0-9]{64}$/.test(p.sha256))throw Error('invalid_firmware');if(p.offset<0xf000&&p.offset+p.size>0x9000)throw Error('invalid_firmware');end=p.offset+p.size;}
    const offsets=profile.parts.map(p=>p.offset).sort((a,b)=>a-b);
    if(offsets.join(',')!=='0,32768,61440,131072'||!profile.parts.some(p=>p.offset===0x8000&&p.sha256===profile.partition_sha256))throw Error('invalid_firmware');return profile;
}
export class Onboarding {
    constructor(port){this.port=port;this.reader=null;this.writer=null;this.buffer='';this.closed=false;}
    async open(){await this.port.open({baudRate:115200});this.reader=this.port.readable.getReader();this.writer=this.port.writable.getWriter();}
    async exchange(command,n,prefix){
        if(this.closed)throw Error('serial_closed');
        const expires=Date.now()+8000;await this.writer.write(encoder.encode(command));
        while(Date.now()<expires){
            const index=this.buffer.indexOf('\n');if(index>=0){const line=this.buffer.slice(0,index).trim();this.buffer=this.buffer.slice(index+1);if(line.startsWith('RUVIEW_CONFIG_ERR_V1'))throw Error('config_rejected');const value=parseReply(line,n,prefix);if(value)return value;continue;}
            let timer;try{const result=await Promise.race([this.reader.read(),new Promise((_,reject)=>{timer=setTimeout(()=>{this.reader.cancel().catch(()=>{});reject(Error('serial_timeout'));},Math.max(1,expires-Date.now()));})]);if(result.done)throw Error('serial_disconnected');this.buffer+=new TextDecoder().decode(result.value);if(this.buffer.length>8192)throw Error('serial_overflow');}finally{clearTimeout(timer);}
        }throw Error('serial_timeout');
    }
    async hello(){const n=nonce();return this.exchange(`RUVIEW_HELLO_V1 ${n}\n`,n,'RUVIEW_HELLO_OK_V1');}
    async configure(c){const h=await this.hello();const ack=await this.exchange(configCommand(c,h.nonce),h.nonce,'RUVIEW_CONFIG_OK_V1');if(ack.node_id!==String(c.node_id)||ack.rebooting!=='1')throw Error('invalid_reply');return h;}
    async close(){this.closed=true;try{await this.reader?.cancel();}catch(_){}try{this.reader?.releaseLock();this.writer?.releaseLock();await this.port.close();}catch(_){}this.buffer='';}
}
