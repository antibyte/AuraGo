package ui

import (
	"os/exec"
	"testing"
)

func TestVideoStudioCanonicalProjectV2(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio canonical form checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-core.js');
const P = window.VideoStudioParts;
const asset = {id:'a',name:'a.mp4',path:'media/a',kind:'video',duration_frames:30,width:64,height:36,has_audio:true,duration_us:1000000,video_duration_us:1000000,source_fps:'30/1',codec:'h264',size_bytes:99,media_url:'/m'};
const project = {version:2,name:'x',width:1280,height:720,fps:30,ntsc:false,assets:[asset],tracks:[]};
const c = P.canonicalProject(project);
assert.equal('ntsc' in c, false, 'false ntsc is omitted like Go omitempty');
assert.deepEqual(c.assets[0], {id:'a',name:'a.mp4',path:'media/a',kind:'video',duration_frames:30,width:64,height:36,has_audio:true,duration_us:1000000,video_duration_us:1000000,source_fps:'30/1',codec:'h264',size_bytes:99});
assert.deepEqual(Object.keys(c.assets[0]), ['id','name','path','kind','duration_frames','duration_us','video_duration_us','width','height','has_audio','source_fps','codec','size_bytes'], 'asset keys follow the Go struct order');
const serverOrdered = {id:'a',name:'a.mp4',path:'media/a',kind:'video',duration_frames:30,duration_us:1000000,video_duration_us:1000000,width:64,height:36,has_audio:true,source_fps:'30/1',codec:'h264',size_bytes:99};
assert.equal(JSON.stringify(c.assets[0]), JSON.stringify(serverOrdered), 'a canonical asset stringifies like the server copy');
assert.equal(P.canonicalProject(Object.assign({}, project, {ntsc:true})).ntsc, true);
const s = {projectId:'p'};
assert.equal(P.hydrateProject(s, {version:1,name:'old',width:1280,height:720,fps:30,assets:[],tracks:[]}).version, 2, 'hydrating a 30 fps version 1 copy upgrades it like the server');
assert.equal(P.hydrateProject(s, {version:1,fps:30,ntsc:true,assets:[],tracks:[]}).version, 1, 'an invalid version 1 copy is not upgraded');
assert.equal(P.hydrateProject(s, {version:2,fps:25,assets:[],tracks:[]}).version, 2);
const image = {id:'b',name:'b.png',path:'media/b',kind:'image',duration_frames:0,width:1,height:1,has_audio:false};
const bare = P.canonicalProject(Object.assign({}, project, {assets:[image]}));
assert.deepEqual(Object.keys(bare.assets[0]), ['id','name','path','kind','duration_frames','width','height','has_audio'], 'absent v2 fields stay absent');
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("canonical form checks: %v\n%s", err, output)
	}
}
