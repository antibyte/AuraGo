package gamemaker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type gameManifest struct {
	Name        string `json:"name"`
	Dimension   string `json:"dimension"`
	Engine      string `json:"engine"`
	Version     string `json:"engine_version"`
	Entry       string `json:"entry"`
	Description string `json:"description"`
}

func WriteScaffold(projectDir string, project Project) error {
	for _, dir := range []string{"src", "assets", "dist"} {
		if err := os.MkdirAll(filepath.Join(projectDir, dir), 0o750); err != nil {
			return fmt.Errorf("create game scaffold directory: %w", err)
		}
	}
	if err := installRuntime(projectDir, project.Dimension); err != nil {
		return err
	}
	engine := "phaser"
	version := PhaserVersion
	source := phaserScaffold
	if project.Dimension == "3d" {
		engine = "three"
		version = ThreeVersion
		source = threeScaffold
	}
	manifest, _ := json.MarshalIndent(gameManifest{
		Name:        project.Name,
		Dimension:   project.Dimension,
		Engine:      engine,
		Version:     version,
		Entry:       "src/main.ts",
		Description: project.Description,
	}, "", "  ")
	files := map[string][]byte{
		"game.json":   append(manifest, '\n'),
		"index.html":  []byte(scaffoldHTML(project.Dimension, project.Name)),
		"src/main.ts": []byte(source),
	}
	for name, data := range files {
		path := filepath.Join(projectDir, filepath.FromSlash(name))
		if err := os.WriteFile(path, data, 0o640); err != nil {
			return fmt.Errorf("write game scaffold %s: %w", name, err)
		}
	}
	return nil
}

func scaffoldHTML(dimension, title string) string {
	runtimeScript := `<script src="vendor/phaser-4.2.1.min.js"></script>`
	if dimension == "3d" {
		runtimeScript = ""
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>%s</title>
  <style>
    html,body,#game-root{width:100%%;height:100%%;margin:0;overflow:hidden;background:#081018;color:#fff}
    body{font:14px system-ui,sans-serif} canvas{display:block;width:100%%;height:100%%}
    #hud{position:fixed;inset:14px auto auto 14px;z-index:2;padding:8px 12px;border:1px solid #ffffff30;
      border-radius:9px;background:#07101ccc;backdrop-filter:blur(10px)}
  </style>
</head>
<body>
  <div id="game-root"></div><div id="hud">Loading…</div>
  %s
  <script type="module" src="dist/game.js"></script>
</body>
</html>
`, htmlEscape(title), runtimeScript)
}

func htmlEscape(value string) string {
	out := ""
	for _, r := range value {
		switch r {
		case '&':
			out += "&amp;"
		case '<':
			out += "&lt;"
		case '>':
			out += "&gt;"
		case '"':
			out += "&quot;"
		default:
			out += string(r)
		}
	}
	return out
}

const diagnosticsPrelude = `
type DiagnosticLevel = "ready" | "scene" | "fps" | "resource_error" | "runtime_error";
const channel = new URLSearchParams(location.hash.replace(/^#/, "")).get("gm-channel") || "";
function diagnostic(type: DiagnosticLevel, detail: Record<string, unknown> = {}) {
  if (!channel) return;
  parent.postMessage({ source: "aurago-game", type, channel, ...detail }, location.origin);
}
window.addEventListener("error", (event) => diagnostic("runtime_error", { message: event.message }));
window.addEventListener("unhandledrejection", (event) => diagnostic("runtime_error", { message: String(event.reason) }));
(window as any).__AURAGO_GAME_DIAGNOSTICS__ = { diagnostic };
`

const phaserScaffold = diagnosticsPrelude + `
declare const Phaser: any;
const hud = document.getElementById("hud")!;
let score = 0;
class MainScene extends Phaser.Scene {
  player: any; cursors: any; stars: any; hazards: any;
  constructor() { super("main"); }
  create() {
    const { width, height } = this.scale;
    this.add.rectangle(width/2, height/2, width, height, 0x081018);
    for (let i=0;i<70;i++) this.add.circle(Math.random()*width,Math.random()*height,Math.random()*2+0.5,0xa7d8ff,0.7);
    this.player = this.add.circle(width/2,height-80,18,0x5eead4);
    this.physics.add.existing(this.player); this.player.body.setCollideWorldBounds(true);
    this.cursors = this.input.keyboard.createCursorKeys();
    this.stars = this.physics.add.group();
    this.hazards = this.physics.add.group();
    for (let i=0;i<7;i++) this.spawnStar();
    for (let i=0;i<4;i++) this.spawnHazard();
    this.physics.add.overlap(this.player,this.stars,(_:any, star:any)=>{star.destroy();score++;this.spawnStar();});
    this.physics.add.overlap(this.player,this.hazards,()=>{score=Math.max(0,score-2);this.player.setFillStyle(0xfb7185);this.time.delayedCall(160,()=>this.player.setFillStyle(0x5eead4));});
    diagnostic("ready",{canvas:true}); diagnostic("scene",{name:"main"});
  }
  spawnStar(){const o=this.add.circle(40+Math.random()*(this.scale.width-80),40+Math.random()*(this.scale.height-160),10,0xfacc15);this.physics.add.existing(o);this.stars.add(o);}
  spawnHazard(){const o=this.add.rectangle(40+Math.random()*(this.scale.width-80),60+Math.random()*(this.scale.height-220),24,24,0xa855f7);this.physics.add.existing(o);o.body.setVelocity((Math.random()-.5)*180,(Math.random()-.5)*180);o.body.setBounce(1).setCollideWorldBounds(true);this.hazards.add(o);}
  update(){
    const speed=260; this.player.body.setVelocity(0);
    if(this.cursors.left.isDown)this.player.body.setVelocityX(-speed);
    if(this.cursors.right.isDown)this.player.body.setVelocityX(speed);
    if(this.cursors.up.isDown)this.player.body.setVelocityY(-speed);
    if(this.cursors.down.isDown)this.player.body.setVelocityY(speed);
    hud.textContent="Score "+score+" · Arrow keys to move";
  }
}
new Phaser.Game({type:Phaser.AUTO,parent:"game-root",width:960,height:540,backgroundColor:"#081018",
  physics:{default:"arcade",arcade:{debug:false}},scene:[MainScene],scale:{mode:Phaser.Scale.RESIZE,autoCenter:Phaser.Scale.CENTER_BOTH}});
`

const threeScaffold = diagnosticsPrelude + `
import * as THREE from "../vendor/three-0.185.1.module.min.js";
const root=document.getElementById("game-root")!, hud=document.getElementById("hud")!;
const scene=new THREE.Scene(); scene.background=new THREE.Color(0x07111f);
const camera=new THREE.PerspectiveCamera(60,innerWidth/innerHeight,.1,100); camera.position.set(0,6,9); camera.lookAt(0,0,0);
const renderer=new THREE.WebGLRenderer({antialias:true}); renderer.setPixelRatio(Math.min(devicePixelRatio,2)); root.appendChild(renderer.domElement);
scene.add(new THREE.HemisphereLight(0xa7d8ff,0x101020,2.2));
const light=new THREE.DirectionalLight(0xffffff,3);light.position.set(4,8,5);scene.add(light);
const floor=new THREE.Mesh(new THREE.PlaneGeometry(14,14),new THREE.MeshStandardMaterial({color:0x10243d,roughness:.85}));
floor.rotation.x=-Math.PI/2;scene.add(floor);
const player=new THREE.Mesh(new THREE.BoxGeometry(.8,.8,.8),new THREE.MeshStandardMaterial({color:0x5eead4,metalness:.2,roughness:.3}));
player.position.y=.45;scene.add(player);
const pickups:Array<any>=[], hazards:Array<any>=[]; let score=0;
function spawn(group:Array<any>,color:number){
 const mesh=new THREE.Mesh(new THREE.SphereGeometry(.3,16,12),new THREE.MeshStandardMaterial({color,emissive:color,emissiveIntensity:.25}));
 mesh.position.set((Math.random()-.5)*11,.35,(Math.random()-.5)*11);scene.add(mesh);group.push(mesh);
}
for(let i=0;i<8;i++)spawn(pickups,0xfacc15);for(let i=0;i<5;i++)spawn(hazards,0xfb7185);
const keys=new Set<string>();addEventListener("keydown",e=>keys.add(e.key.toLowerCase()));addEventListener("keyup",e=>keys.delete(e.key.toLowerCase()));
function resize(){renderer.setSize(innerWidth,innerHeight);camera.aspect=innerWidth/innerHeight;camera.updateProjectionMatrix()}addEventListener("resize",resize);resize();
let last=performance.now(),frames=0,fpsAt=last;diagnostic("ready",{canvas:true});diagnostic("scene",{name:"arena"});
function loop(now:number){
 requestAnimationFrame(loop);const dt=Math.min((now-last)/1000,.05);last=now;const speed=5*dt;
 if(keys.has("a")||keys.has("arrowleft"))player.position.x-=speed;if(keys.has("d")||keys.has("arrowright"))player.position.x+=speed;
 if(keys.has("w")||keys.has("arrowup"))player.position.z-=speed;if(keys.has("s")||keys.has("arrowdown"))player.position.z+=speed;
 player.position.x=THREE.MathUtils.clamp(player.position.x,-6,6);player.position.z=THREE.MathUtils.clamp(player.position.z,-6,6);
 pickups.forEach(p=>{p.rotation.y+=dt;if(p.position.distanceTo(player.position)<.7){score++;p.position.set((Math.random()-.5)*11,.35,(Math.random()-.5)*11)}});
 hazards.forEach((h,i)=>{h.position.x+=Math.sin(now*.001+i)*dt;if(h.position.distanceTo(player.position)<.7){score=Math.max(0,score-1);h.position.set((Math.random()-.5)*11,.35,(Math.random()-.5)*11)}});
 hud.textContent="Score "+score+" · WASD / arrow keys";renderer.render(scene,camera);
 frames++;if(now-fpsAt>1000){diagnostic("fps",{value:frames});frames=0;fpsAt=now}
}requestAnimationFrame(loop);
`
