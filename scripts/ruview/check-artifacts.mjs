import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {validateProfile,revision} from '../../ui/js/ruview/protocol.mjs';
const c=JSON.parse(await readFile('internal/ruview/artifacts.json'));
const resources=process.argv.find(a=>a.startsWith('--resources='))?.slice(12)||'ui';
if(c.revision!==revision||c.version!=='v3122'||c.firmware_version!=='0.8.12'||!/@sha256:[a-f0-9]{64}$/.test(c.docker_image))throw Error('Artifact provenance mismatch');
if(c.firmware.length!==6||new Set(c.firmware.map(p=>p.id)).size!==6)throw Error('Expected all six firmware profiles');
async function check(file,p){const data=await readFile(p);if(data.length!==file.size||createHash('sha256').update(data).digest('hex')!==file.sha256)throw Error(`Integrity mismatch: ${p}`);}
for(const profile of c.firmware){validateProfile(profile);for(const file of profile.parts)await check(file,`${resources}/js/vendor/ruview-firmware/${revision}/${profile.id}/${file.name}`);}
for(const runtime of c.native)for(const file of runtime.files){if(!file.resource)throw Error('Expected bundled native runtime');await check(file,resources+'/'+file.resource);}
if(process.argv.includes('--release')){
 for(const platform of ['windows-amd64','darwin-amd64','darwin-arm64'])if(!c.native.some(n=>n.platform===platform))throw Error(`Missing release package: ${platform}`);
 for(const platform of ['linux/amd64','linux/arm64'])if(!c.docker_platforms.includes(platform))throw Error(`Missing release image platform: ${platform}`);
}
console.log(`Verified ${c.firmware.length} firmware profiles and ${c.native.length} native packages; Docker ${c.docker_platforms.join(', ')}`);
