import fs from 'node:fs';
const all = JSON.parse(fs.readFileSync('out/cmaps.json','utf8'));
const need = []; for (let c=0x391;c<=0x3A9;c++) if(c!==0x3A2) need.push(c); for (let c=0x3B1;c<=0x3C9;c++) need.push(c); need.push(0x386,0x388,0x389,0x38A,0x38C,0x38E,0x38F,0x3AC,0x3AD,0x3AE,0x3AF,0x3CC,0x3CD,0x3CE,0x3CA,0x3CB,0x390,0x3B0,0x3C2);
for (const k of ['ui/fonts/Geist-Regular.woff2','ui/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1pL7SUc.woff2','ui/fonts/qr/NotoSansCJKsc.otf','ui/fonts/qr/LiberationSerif-Regular.ttf','ui/fonts/JetBrainsMono-SemiBold.woff2']) {
  const s = new Set(all[k]); const miss = need.filter(c=>!s.has(c));
  console.log(k, 'core greek missing', miss.length, '/', need.length, miss.slice(0,12).map(c=>c.toString(16)).join(','));
}
