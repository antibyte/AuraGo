import {test} from 'node:test';
import assert from 'node:assert/strict';
import {nonce,configCommand,parseReply,privateIPv4,validateConfig,validateProfile,sha256,Onboarding} from '../../ui/js/ruview/protocol.mjs';
import {serialVariant,validateBoard,flash} from '../../ui/js/ruview/flash.mjs';

const profile=()=>({id:'esp32s3-4mb-uart',chip:'esp32s3',console:'uart',flash_bytes:4194304,partition_sha256:'a'.repeat(64),parts:[0,0x8000,0xf000,0x20000].map((offset,i)=>({name:`part${i}.bin`,offset,size:4096,sha256:'a'.repeat(64)}))});
test('UTF-8 credentials, private IPv4 and explicit KEEP',()=>{
 const c={node_id:1,target_ip:'192.168.1.2',target_port:5005,ssid:'ä'.repeat(16),password:'12345678'};
 assert.equal(validateConfig(c),c);assert.throws(()=>validateConfig({...c,ssid:c.ssid+'a'}));assert.throws(()=>validateConfig({...c,password:'é'.repeat(32)}));
 for(const ip of ['127.0.0.1','169.254.1.1','8.8.8.8','10.256.0.1','010.0.0.1','::1','172.32.0.1'])assert.equal(privateIPv4(ip),false);
 assert.match(configCommand({...c,keep:true},nonce()),/ KEEP KEEP\n$/);assert.throws(()=>configCommand(c,'bad'));
});
test('nonce and identity parsing rejects stale or ambiguous replies',()=>{
 const n=nonce(),line=`RUVIEW_HELLO_OK_V1 nonce=${n} chip=esp32s3 version=0.8.12 device_digest=abcdef0123456789 configured=1`;
 assert.equal(parseReply(line,n).chip,'esp32s3');assert.throws(()=>parseReply(line,nonce()));assert.throws(()=>parseReply(line+' nonce='+n,n));assert.throws(()=>parseReply(line.replace('esp32s3','esp8266'),n));assert.equal(parseReply('boot log',n),null);
});
test('catalog offsets and board identity fail before writing',()=>{
 assert.equal(validateProfile(profile()).chip,'esp32s3');
 for(const change of [p=>p.parts[1].offset=0x9000,p=>p.parts[2].offset=0x8000,p=>p.parts[3].size=4194304,p=>p.parts[0].sha256='bad',p=>p.flash_bytes=8388608,p=>p.parts[3].offset=0x10000]){const p=profile();change(p);assert.throws(()=>validateProfile(p));}
 validateBoard(profile(),'ESP32-S3',22<<16);assert.throws(()=>validateBoard(profile(),'ESP32-C6',22<<16),/wrong_chip/);assert.throws(()=>validateBoard(profile(),'ESP32-S3',21<<16),/wrong_flash/);
 assert.equal(serialVariant({usbVendorId:0x303a,usbProductId:0x1001}),'cdc');assert.throws(()=>serialVariant({usbVendorId:123,usbProductId:456}));
});
test('damaged firmware never opens the serial transport',async()=>{
 const previous=globalThis.fetch;globalThis.fetch=async()=>new Response(new Uint8Array(4096));
 try{await assert.rejects(flash({getInfo:()=>({usbVendorId:0x10c4,usbProductId:0xea60}),open:()=>assert.fail('opened damaged firmware')},profile()),/checksum_failed/);}finally{globalThis.fetch=previous;}
});
test('N16R8 uses a 16 MB S3 profile and capacity mismatches report both sizes',()=>{
 for(const console of ['uart','cdc']){
  const p={...profile(),id:`esp32s3-16mb-${console}`,console,flash_bytes:16777216};
  assert.equal(validateProfile(p),p);validateBoard(p,'ESP32-S3',24<<16);
  for(const capacity of [22,23,25])assert.throws(()=>validateBoard(p,'ESP32-S3',capacity<<16),error=>error.message==='wrong_flash'&&error.detectedFlash===2**capacity/1048576&&error.expectedFlash===16);
  assert.throws(()=>validateProfile({...p,id:`esp32c6-16mb-${console}`,chip:'esp32c6'}),/invalid_firmware/);
 }
 assert.throws(()=>validateBoard({...profile(),flash_bytes:8388608},'ESP32-S3',24<<16),error=>error.detectedFlash===16&&error.expectedFlash===8);
});
test('serial handshake and configuration are confirmed, secrets stay in writes',async()=>{
 let controller,closed=false,writes=[];
 const port={open:async({baudRate})=>assert.equal(baudRate,115200),close:async()=>{closed=true;},
 readable:new ReadableStream({start(c){controller=c;}}),writable:new WritableStream({write(bytes){
  const line=new TextDecoder().decode(bytes);writes.push(line);const fields=line.trim().split(' '),n=fields[1];
  controller.enqueue(new TextEncoder().encode(fields[0]==='RUVIEW_HELLO_V1'?`RUVIEW_HELLO_OK_V1 nonce=${n} chip=esp32c6 version=0.8.12 device_digest=abcdef0123456789\n`:`RUVIEW_CONFIG_OK_V1 nonce=${n} node_id=7 rebooting=1\n`));
 }})};
 const serial=new Onboarding(port);await serial.open();const h=await serial.configure({node_id:7,target_ip:'10.0.0.2',target_port:5005,ssid:'test',password:'example123'});assert.equal(h.chip,'esp32c6');await serial.close();assert.ok(closed);assert.equal(serial.buffer,'');assert.equal(writes.length,2);assert.ok(!writes[1].includes('example123'));
});
test('disconnect releases the port; no retry',async()=>{
 let writes=0,closes=0;const p={open:async()=>{},close:async()=>{closes++;},readable:new ReadableStream({start(c){c.close();}}),writable:new WritableStream({write(){writes++;}})};
 const serial=new Onboarding(p);await serial.open();await assert.rejects(serial.hello(),/serial_disconnected/);await serial.close();assert.equal(writes,1);assert.equal(closes,1);
});
test('SHA-256 is checked over exact bytes',async()=>assert.equal(await sha256(new TextEncoder().encode('abc')),'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad'));

test('flashing preserves NVS, requires erase for layout changes, and never retries a write',async()=>{
 const {ESPLoader,Transport}=await import('../../ui/js/vendor/ruview-esptool.js');
 const p=profile(),images=await Promise.all(p.parts.map(async(part,i)=>{const data=new Uint8Array(part.size).fill(i+1);part.sha256=await sha256(data);return data;}));p.partition_sha256=p.parts[1].sha256;
 const fetchBefore=globalThis.fetch,restore=[];let writes=0,erases=0,disconnects=0,incompatible=false,fail=false,bootFailure=false,capacity=22;
 function patch(object,key,value){const old=object[key];restore.push(()=>object[key]=old);object[key]=value;}
 patch(ESPLoader.prototype,'main',async function(){if(bootFailure)throw Error('fixture_sync_failed');this.chip={CHIP_NAME:'ESP32-S3'};});
 patch(ESPLoader.prototype,'readFlashId',async()=>capacity<<16);
 patch(ESPLoader.prototype,'readFlash',async(address)=>incompatible&&address===0x8000&&writes===0?new Uint8Array(4096):images[p.parts.findIndex(part=>part.offset===address)]);
 patch(ESPLoader.prototype,'eraseFlash',async()=>{erases++;});
 patch(ESPLoader.prototype,'writeFlash',async function(options){writes++;assert.equal(this.WRITE_BLOCK_ATTEMPTS,1);assert.equal(options.eraseAll,false);assert.deepEqual(options.fileArray.map(f=>f.address),[0,0x8000,0xf000,0x20000]);if(fail)throw Error('uncertain_write');});
 patch(ESPLoader.prototype,'after',async()=>{});
 patch(Transport.prototype,'disconnect',async()=>{disconnects++;});
 globalThis.fetch=async(url)=>new Response(images[p.parts.findIndex(part=>url.endsWith('/'+part.name))]);
 const port={getInfo:()=>({usbVendorId:0x10c4,usbProductId:0xea60})};
 try{
  await flash(port,p);assert.equal(writes,1);assert.equal(erases,0);assert.equal(disconnects,1);
  writes=0;incompatible=true;await assert.rejects(flash(port,p),/erase_required/);assert.equal(writes,0);assert.equal(erases,0);
  await flash(port,p,{erase:true});assert.equal(erases,1);assert.equal(writes,1);
  writes=0;incompatible=false;fail=true;await assert.rejects(flash(port,p),error=>error.message==='uncertain_write'&&error.ruviewStage==='flash_failed');assert.equal(writes,1);
  writes=0;fail=false;capacity=24;await assert.rejects(flash(port,p,{erase:true}),/wrong_flash/);assert.equal(writes,0);assert.equal(erases,1,'capacity rejection must precede erase');
  bootFailure=true;await assert.rejects(flash(port,p),error=>error.message==='fixture_sync_failed'&&error.ruviewStage==='bootloader_failed');assert.equal(writes,0);
 }finally{globalThis.fetch=fetchBefore;for(const undo of restore.reverse())undo();}
});
