import {test} from 'node:test';
import assert from 'node:assert/strict';
import {CsiSimulator} from '../../ui/js/vendor/ruview/pose-fusion/js/csi-simulator.js';
test('Pose-Fusion requires real CSI and expires live frames',()=>{
 const c=new CsiSimulator();assert.equal(c.isLive,false);
 c._handleJsonFrame({source:'simulated',nodes:[{amplitude:[1,2,3]}]});assert.equal(c.isLive,false);
 c._handleJsonFrame({source:'wifi',nodes:[{amplitude:[1,2,3]}]});assert.equal(c.isLive,false);
 c._handleJsonFrame({source:'esp32',nodes:[{amplitude:[1,2,3]}]});assert.equal(c.isLive,true);
 c.auragoFrameAt=performance.now()-10001;assert.equal(c.isLive,false);
 c.disconnect();assert.equal(c.isLive,false);
});
