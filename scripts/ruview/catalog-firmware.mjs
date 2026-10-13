import { readFile, writeFile, mkdir, copyFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { validateProfile } from '../../ui/js/ruview/protocol.mjs';

const root = process.argv.find(a => a.startsWith('--build='))?.slice(8);
if (!root) throw Error('Pass --build=<firmware/out>');
const catalogPath = 'internal/ruview/artifacts.json';
const catalog = JSON.parse(await readFile(catalogPath));
const profiles = [];
const sha256 = data => createHash('sha256').update(data).digest('hex');
for (const [chip, size] of [['esp32s3',4],['esp32s3',8],['esp32s3',16],['esp32c6',4]]) {
  for (const console of ['uart','cdc']) {
    const id = `${chip}-${size}mb-${console}`, dir = path.join(root,id,'build');
    const args = JSON.parse(await readFile(path.join(dir,'flasher_args.json')));
    const config = await readFile(path.join(root,id,'sdkconfig'),'utf8');
    if (args.extra_esptool_args.chip !== chip || args.flash_settings.flash_size !== `${size}MB` || !config.includes(`CONFIG_IDF_TARGET="${chip}"`)) throw Error(`${id}: build target mismatch`);
    if (!config.includes(console === 'cdc' ? 'CONFIG_ESP_CONSOLE_USB_SERIAL_JTAG=y' : 'CONFIG_ESP_CONSOLE_UART_DEFAULT=y')) throw Error(`${id}: console mismatch`);
    const description = JSON.parse(await readFile(path.join(dir,'project_description.json')));
    if (description.project_version !== catalog.firmware_version) throw Error(`${id}: firmware version mismatch`);
    const dest = `ui/js/vendor/ruview-firmware/${catalog.revision}/${id}`;
    await mkdir(dest,{recursive:true});
    const parts=[];
    for (const [offset, name] of Object.entries(args.flash_files)) {
      const file = path.join(dir,name), data = await readFile(file);
      const entry = { name:path.basename(name), offset:Number(offset), size:data.length, sha256:sha256(data) };
      parts.push(entry); await copyFile(file,path.join(dest,entry.name));
    }
    parts.sort((a,b)=>a.offset-b.offset);
    const profile = { id, chip, flash_bytes:size*1048576, console, partition_sha256:parts.find(p=>p.offset===0x8000)?.sha256, parts };
    validateProfile(profile); profiles.push(profile);
    await writeFile(`${dest}/build-metadata.json`,JSON.stringify({revision:catalog.revision,version:description.project_version,idf_version:description.idf_ver,flash_args:args},null,2)+'\n');
  }
}
catalog.firmware = profiles;
catalog.firmware_builder = 'espressif/idf@sha256:f1e9f69dc052b9afc7801ca884e0ef40c17e014bb05ce73d9c09d29290bd17fb';
await writeFile(catalogPath,JSON.stringify(catalog,null,2)+'\n');
console.log(`Catalogued ${profiles.length} verified firmware profiles`);
