import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
const context={window:{},Intl,Date,structuredClone,crypto:globalThis.crypto};vm.runInNewContext(fs.readFileSync('ui/js/desktop/apps/sheets-data.js','utf8'),context);
const d=context.window.SheetsData;
assert.equal(d.parseInput('1.234,50','de').v,1234.5);
assert.equal(d.parseInput('10.09.2026','de').v,d.parseInput('2026-09-10','en').v);
assert.equal(d.parseInput('1,234.50','en').v,1234.5);
assert.equal(d.parseInput('00123','de').v,'00123');
assert.equal(d.parseInput('01/01/1900','en').v,1);
assert.equal(d.parseInput('12,5%','de').v,.125);
assert.equal(d.parseInput('=AVG(A1:A3)+"AVG("','de').f,'=AVERAGE(A1:A3)+"AVG("');
assert.equal(d.parseInput('31.02.2026','de').t,1);
assert.deepEqual(JSON.parse(JSON.stringify(d.parseCSV('a;"two;values"\r\n"quoted ""word""";00123',';'))),[['a','two;values'],['quoted "word"','00123']]);
assert.throws(()=>d.parseCSV('"unclosed'));
const en=JSON.parse(fs.readFileSync('ui/lang/desktop/en.json','utf8'));
const keys=Object.keys(en).filter(key=>key.startsWith('desktop.sheets_'));
for(const file of fs.readdirSync('ui/lang/desktop').filter(name=>name.endsWith('.json'))){
    const labels=JSON.parse(fs.readFileSync('ui/lang/desktop/'+file,'utf8'));
    for(const key of keys){assert.ok(labels[key],file+': '+key);assert.ok(!labels[key].includes('\uFFFD'),file+': invalid Unicode');}
}
const filter=(await import('@univerjs/sheets-filter-ui/locale/en-US')).default;
for(const locale of ['cs','da','el','hi','nl','no','sv']){
    const labels=JSON.parse(fs.readFileSync('ui/lang/desktop/'+locale+'.json','utf8'));
    const tr=key=>['sheets_','writer_',''].map(prefix=>labels['desktop.'+prefix+key]).find(Boolean)||key;
    const translated=d.nativeLocale(filter,tr,locale)['sheets-filter-ui'];
    assert.equal(translated.panel['select-all'],labels['desktop.fm.select_all']);
    assert.equal(translated.date[1],new Intl.DateTimeFormat(locale,{month:'long'}).format(new Date(2000,0,1)));
}
console.log('Sheets localized inputs, formula alias, CSV and 16 locale checks passed');
