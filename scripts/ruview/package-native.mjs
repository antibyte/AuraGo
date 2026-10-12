import {readFile,writeFile,copyFile,mkdir} from 'node:fs/promises';
import path from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';

const source=process.argv.find(a=>a.startsWith('--source='))?.slice(9);
const platform=process.argv.find(a=>a.startsWith('--platform='))?.slice(11);
const out=process.argv.find(a=>a.startsWith('--out='))?.slice(6);
const revision='a71634ae8a606bba5463489450c69e8b42de114e';
if(!source||!out||!['windows-amd64','darwin-amd64','darwin-arm64'].includes(platform))throw Error('Require --source, --out, --platform');
if(execFileSync('git',['-C',source,'rev-parse','HEAD'],{encoding:'utf8'}).trim()!==revision)throw Error('Source revision mismatch');
const names=platform.startsWith('windows')?['sensing-server.exe']:['sensing-server','mac_wifi'];
await mkdir(out,{recursive:true});const files=[];
for(const name of names){const sourceFile=path.join(source,'v2/target/release',name),data=await readFile(sourceFile);await copyFile(sourceFile,path.join(out,name));files.push({name,size:data.length,sha256:createHash('sha256').update(data).digest('hex')});}
await copyFile(path.join(source,'LICENSE'),path.join(out,'LICENSE-RuView.txt'));
await writeFile(path.join(out,'manifest.json'),JSON.stringify({revision,platform,files},null,2)+'\n');
console.log(`${platform}: packaged ${files.length} verified binaries`);
