const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const detail = {innerHTML: ''};
const state = {
 configData: {composio: {user_id:'owner',toolkits:[{slug:'github',enabled:true}]}},
 t: key => key,
 escapeAttr: value => String(value).replaceAll('&','&amp;').replaceAll('"','&quot;').replaceAll('<','&lt;'),
 document: {getElementById: () => detail},
 window: {addEventListener(){}},
 cfgActionResult: {html: () => ''},
};
vm.createContext(state);
vm.runInContext(fs.readFileSync(path.join(__dirname,'../cfg/composio.js'),'utf8'),state);
vm.runInContext(`
composioState.selectedSlug='github';
composioState.accounts=[
 {id:'good',user_id:'owner',toolkit_slug:'github',status:'ACTIVE'},
 {id:'foreign',user_id:'other',toolkit_slug:'github',status:'ACTIVE'},
 {id:'expired',user_id:'owner',toolkit_slug:'github',status:'EXPIRED'},
 {id:'wrong-kit',user_id:'owner',toolkit_slug:'gmail',status:'ACTIVE'},
];
composioRenderDetail();
`,state);
assert.match(detail.innerHTML, /value="good"/);
for(const id of ['foreign','expired','wrong-kit']) assert.ok(!detail.innerHTML.includes('value="'+id+'"'));
vm.runInContext("composioSelectAccount('good')",state);
assert.equal(state.configData.composio.toolkits[0].preferred_connected_account_id,'good');
vm.runInContext("composioSelectAccount('')",state);
assert.equal(state.configData.composio.toolkits[0].preferred_connected_account_id,'');
for(const locale of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']) {
 const data=JSON.parse(fs.readFileSync(path.join(__dirname, '../lang/config/composio',locale+'.json'),'utf8'));
 for(const key of ['account_binding','account_binding_help','unique_account']) assert.ok(data['config.composio.'+key]);
}
console.log('Composio account binding and 16 locales passed');
