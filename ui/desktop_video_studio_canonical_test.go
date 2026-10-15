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
assert.equal(P.canonicalProject(Object.assign({}, project, {ntsc:true})).ntsc, true);
const image = {id:'b',name:'b.png',path:'media/b',kind:'image',duration_frames:0,width:1,height:1,has_audio:false};
const bare = P.canonicalProject(Object.assign({}, project, {assets:[image]}));
assert.deepEqual(Object.keys(bare.assets[0]), ['id','name','path','kind','duration_frames','width','height','has_audio'], 'absent v2 fields stay absent');
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("canonical form checks: %v\n%s", err, output)
	}
}
