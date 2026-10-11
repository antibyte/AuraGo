import {readFile,writeFile,mkdir,copyFile} from 'node:fs/promises';
import path from 'node:path';
import {createHash} from 'node:crypto';
const dir=process.argv.find(a=>a.startsWith('--package='))?.slice(10);
if(!dir)throw Error('Require --package=<reviewed native artifact directory>');
const catalogPath='internal/ruview/artifacts.json',catalog=JSON.parse(await readFile(catalogPath));
const entry=JSON.parse(await readFile(path.join(dir,'manifest.json')));
if(entry.revision!==catalog.revision||!['windows-amd64','darwin-amd64','darwin-arm64'].includes(entry.platform))throw Error('Unsupported package revision/platform');
const names=entry.platform==='windows-amd64'?['sensing-server.exe']:['mac_wifi','sensing-server'];
if(entry.files.map(f=>f.name).sort().join(',')!==names.sort().join(','))throw Error('Unexpected native package contents');
for(const file of entry.files){const data=await readFile(path.join(dir,file.name));if(data.length!==file.size||createHash('sha256').update(data).digest('hex')!==file.sha256)throw Error('Native package integrity mismatch');}
// Resource packs admit opaque .bin files; executable names are restored only on explicit installation.
for(const file of entry.files){file.resource=`js/vendor/ruview-runtime/${catalog.revision}/${entry.platform}/${file.name}.bin`;await mkdir(path.dirname('ui/'+file.resource),{recursive:true});await copyFile(path.join(dir,file.name),'ui/'+file.resource);}
catalog.native=catalog.native.filter(n=>n.platform!==entry.platform);catalog.native.push({platform:entry.platform,files:entry.files});catalog.native.sort((a,b)=>a.platform.localeCompare(b.platform));
await writeFile(catalogPath,JSON.stringify(catalog,null,2)+'\n');
console.log(`Imported verified ${entry.platform} package`);
