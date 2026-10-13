import { validateProfile, sha256, revision } from './protocol.mjs';

export function serialAvailable(){return isSecureContext&&!!navigator.serial&&(!document.permissionsPolicy?.allowsFeature||document.permissionsPolicy.allowsFeature('serial'))&&(!document.featurePolicy?.allowsFeature||document.featurePolicy.allowsFeature('serial'));}
export function serialVariant(info){if(info.usbVendorId===0x303a&&[0x1001,0x1002,0x1003,0x0002,0x0003].includes(info.usbProductId))return 'cdc';if((info.usbVendorId===0x10c4&&info.usbProductId===0xea60)||(info.usbVendorId===0x1a86&&[0x7523,0x55d4,0x55d3].includes(info.usbProductId))||(info.usbVendorId===0x0403&&info.usbProductId===0x6001))return 'uart';throw Error('unknown_usb');}
export function validateBoard(profile,chipName,flashID){
    const chip=chipName.toLowerCase().replaceAll('-',''),bytes=2**((flashID>>>16)&255);
    if(chip!==profile.chip)throw Error('wrong_chip');
    if(bytes!==profile.flash_bytes){const error=Error('wrong_flash');error.detectedFlash=bytes/1048576;error.expectedFlash=profile.flash_bytes/1048576;throw error;}
}

export async function flash(port,profile,{erase=false,signal,onProgress=()=>{}}={}){
    validateProfile(profile);if(serialVariant(port.getInfo())!==profile.console)throw Error('wrong_console');
    const files=[];
    for(const part of profile.parts){const response=await fetch(`/js/vendor/ruview-firmware/${revision}/${profile.id}/${part.name}`,{signal});if(!response.ok)throw Error('firmware_missing');const data=new Uint8Array(await response.arrayBuffer());if(data.length!==part.size||await sha256(data)!==part.sha256)throw Error('checksum_failed');files.push({data,address:part.offset});}
    const { ESPLoader, Transport } = await import('../vendor/ruview-esptool.js');
    const transport=new Transport(port,false),loader=new ESPLoader({transport,baudrate:115200,terminal:{clean(){},writeLine(){},write(){}}});
    loader.WRITE_BLOCK_ATTEMPTS=1;
    let stage='bootloader_failed';
    const abort=()=>{transport.disconnect().catch(()=>{});};signal?.addEventListener('abort',abort,{once:true});
    try{
        await loader.main();if(signal?.aborted)throw Error('cancelled');
        stage='flash_failed';
        validateBoard(profile,loader.chip.CHIP_NAME,await loader.readFlashId());
        const table=profile.parts.find(p=>p.offset===0x8000);const previous=await loader.readFlash(table.offset,table.size);
        if(await sha256(previous)!==profile.partition_sha256&&!erase)throw Error('erase_required');
        if(erase)await loader.eraseFlash();
        await loader.writeFlash({fileArray:files,flashSize:'keep',flashMode:'keep',flashFreq:'keep',eraseAll:false,compress:true,reportProgress:(i,n,total)=>onProgress(Math.round((i+n/total)/files.length*100))});
        for(let i=0;i<files.length;i++){if(signal?.aborted)throw Error('cancelled');if(await sha256(await loader.readFlash(files[i].address,files[i].data.length))!==profile.parts[i].sha256)throw Error('write_verify_failed');}
        await loader.after('hard_reset');
    }catch(error){error.ruviewStage=stage;throw error;}
    finally{signal?.removeEventListener('abort',abort);try{await transport.disconnect();}catch(_){}}
}
