import * as THREE from 'three';

// Memory hologram: short sampled artifacts projected above the archive roof. Text is drawn
// with canvas fillText only; the scene never interprets it as markup or instructions.
const FONT = 'Geist, "Segoe UI", system-ui, sans-serif';
const CONE_HEIGHT = 13, HASH = s => { let h = 2166136261; for (let i = 0; i < s.length; i++) h = Math.imul(h ^ s.charCodeAt(i), 16777619); return (h >>> 0).toString(16).padStart(8, '0'); };
const PANEL_SHADER = {
  vertex: 'varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}',
  fragment: `uniform sampler2D map;uniform float time,glitch,fade,gain;uniform vec3 tint;varying vec2 vUv;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){vec2 uv=vUv;float row=floor(uv.y*44.0),frame=floor(time*24.0);
float g=glitch*step(0.7,hash(row+frame*0.37));uv.x+=(hash(row*3.1+frame)-0.5)*0.14*g;
float split=0.0025+glitch*0.02;vec4 c=texture2D(map,uv);
float r=texture2D(map,uv+vec2(split,0.0)).r,b=texture2D(map,uv-vec2(split,0.0)).b;
float a=max(c.a,max(texture2D(map,uv+vec2(split,0.0)).a,texture2D(map,uv-vec2(split,0.0)).a));
float lines=0.84+0.16*sin(uv.y*420.0-time*9.0);float flicker=0.95+0.05*sin(time*31.0)*sin(time*7.3);
float bx=(fract(uv.y*0.55-time*0.11)-0.5)*22.0;float band=0.35*exp(-bx*bx);
float edge=smoothstep(0.0,0.05,uv.x)*smoothstep(0.0,0.05,1.0-uv.x)*smoothstep(0.0,0.09,uv.y)*smoothstep(0.0,0.09,1.0-uv.y);
vec3 col=vec3(r,c.g,b)*tint*(lines*flicker+band)*gain;gl_FragColor=vec4(col*a*fade*edge,1.0);}`,
};
const CONE_SHADER = {
  vertex: 'varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}',
  fragment: `uniform float time,strength;uniform vec3 base,top;varying vec2 vUv;varying vec3 vNormal,vView;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){float h=vUv.y;float rim=pow(clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0),1.5);
float fall=pow(clamp(1.0-h,0.0,1.0),1.35)*0.75+0.25*clamp(1.0-h,0.0,1.0);float scan=0.7+0.3*sin(h*64.0-time*5.5);
float noise=0.85+0.15*hash(floor(h*96.0)+floor(time*18.0));float shimmer=0.8+0.2*sin(vUv.x*40.0+time*2.0);
float a=(0.28+0.72*rim)*fall*scan*noise*shimmer*strength;gl_FragColor=vec4(mix(base,top,h)*a,1.0);}`,
};
const GLOW_SHADER = {
  vertex: PANEL_SHADER.vertex,
  fragment: `uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float r=length(vUv-0.5)*2.0;float rings=smoothstep(0.35,1.0,sin(r*16.0-time*2.6));
float core=exp(-r*r*7.0);float a=(core*1.2+pow(max(0.0,1.0-r),1.8)*(0.25+0.75*rings)*0.6)*strength*(1.0-smoothstep(0.85,1.0,r));
gl_FragColor=vec4(color*a,1.0);}`,
};
const RING_SHADER = {
  vertex: PANEL_SHADER.vertex,
  fragment: `uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float dash=step(0.45,fract(vUv.x*28.0+time*0.35));float pulse=0.75+0.25*sin(vUv.x*6.2831*3.0-time*4.0);
gl_FragColor=vec4(color*dash*pulse*strength,1.0);}`,
};
const MOTE_SHADER = {
  vertex: `attribute float seed;uniform float time,pointScale;varying float vLife;
void main(){float speed=0.55+seed*0.9;float life=fract(seed*3.17+time*speed/13.0);float y=life*13.0;
float ang=seed*6.2831+time*(0.35+seed*0.4)+life*2.2;float rad=mix(1.8,6.8,life)*(0.3+0.7*fract(seed*7.13));
vec4 mv=modelViewMatrix*vec4(cos(ang)*rad,y,sin(ang)*rad,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.09+0.16*fract(seed*3.7))*pointScale/max(1.0,-mv.z);vLife=life;}`,
  fragment: `uniform vec3 color;uniform float strength;varying float vLife;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,2.0)*sin(vLife*3.1416)*strength;gl_FragColor=vec4(color*a,1.0);}`,
};
function additive(shader, uniforms, extra = {}) {
  return new THREE.ShaderMaterial({ uniforms, vertexShader: shader.vertex, fragmentShader: shader.fragment,
    transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide, ...extra });
}
export function sanitizeArtifacts(list, limit = 24) {
  const out = [], seen = new Set();
  for (const item of Array.isArray(list) ? list : []) {
    if (typeof item !== 'string') continue;
    const text = item.replace(/[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u2028-\u202e\u2066-\u2069]/g, ' ').replace(/\s+/g, ' ').trim().slice(0, 140);
    if (text.length < 4 || seen.has(text)) continue;
    seen.add(text); out.push(text);
    if (out.length >= limit) break;
  }
  return out;
}
function wrapText(ctx, text, maxWidth, maxLines) {
  const lines = [], fits = s => ctx.measureText(s).width <= maxWidth;
  let line = '';
  for (const word of text.split(' ')) {
    if (lines.length >= maxLines) break;
    const candidate = line ? line + ' ' + word : word;
    if (fits(candidate)) { line = candidate; continue; }
    if (line) { lines.push(line); line = ''; if (lines.length >= maxLines) break; }
    let rest = word;
    while (!fits(rest) && lines.length < maxLines) {
      let cut = rest.length - 1;
      while (cut > 1 && !fits(rest.slice(0, cut))) cut--;
      lines.push(rest.slice(0, cut)); rest = rest.slice(cut);
    }
    line = rest;
  }
  if (line && lines.length < maxLines) { lines.push(line); line = ''; }
  const shown = lines.join('').replace(/\s/g, '').length, total = text.replace(/\s/g, '').length;
  if (shown < total && lines.length) {
    let last = lines[lines.length - 1].replace(/[\s,.;:]+$/, '');
    while (last.length && !fits(last + '…')) last = last.slice(0, -1);
    lines[lines.length - 1] = last + '…';
  }
  return lines.length ? lines : [''];
}
export function createMemoryHologram(scene, district, options = {}) {
  const roof = options.roof ?? 21, label = String(options.label || 'MEMORY').toUpperCase();
  const group = new THREE.Group(); group.name = 'memory-hologram';
  group.position.set(district.x, roof, district.z); scene.add(group);
  const geometries = [], materials = [], textures = [];
  const own = (g, m) => { geometries.push(g); materials.push(m); return new THREE.Mesh(g, m); };
  const cyan = new THREE.Color(0x63dcff), violet = new THREE.Color(0xa78bff), pale = new THREE.Color(0xd8f5ff);
  const shared = { time: { value: 0 } };
  let reducedMode = false, switches = 0, texts = [], queue = [], cursor = 0, source = 'none', disposed = false;
  // Emitter, cone, rings and projector core.
  const glow = own(new THREE.PlaneGeometry(11, 11), additive(GLOW_SHADER, { time: shared.time, strength: { value: .9 }, color: { value: cyan } }));
  glow.rotation.x = -Math.PI / 2; glow.position.y = .18; group.add(glow);
  const cone = own(new THREE.CylinderGeometry(7.4, 2.3, CONE_HEIGHT, 56, 1, true),
    additive(CONE_SHADER, { time: shared.time, strength: { value: .32 }, base: { value: cyan }, top: { value: violet } }));
  cone.position.y = CONE_HEIGHT / 2 + .3; group.add(cone);
  const ringTop = own(new THREE.TorusGeometry(7.5, .055, 6, 128), additive(RING_SHADER, { time: shared.time, strength: { value: 1.6 }, color: { value: cyan } }));
  ringTop.rotation.x = Math.PI / 2; ringTop.position.y = CONE_HEIGHT + .3; group.add(ringTop);
  const ringBase = own(new THREE.TorusGeometry(2.6, .05, 6, 96), additive(RING_SHADER, { time: shared.time, strength: { value: 2 }, color: { value: pale } }));
  ringBase.rotation.x = Math.PI / 2; ringBase.position.y = .55; group.add(ringBase);
  const coreGeometry = new THREE.IcosahedronGeometry(1.35, 1), wire = new THREE.WireframeGeometry(coreGeometry);
  const coreMaterial = new THREE.LineBasicMaterial({ color: 0x9be9ff, transparent: true, opacity: .75, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false });
  const core = new THREE.LineSegments(wire, coreMaterial); core.position.y = 4.6; group.add(core);
  geometries.push(coreGeometry, wire); materials.push(coreMaterial);
  const fillMaterial = new THREE.MeshBasicMaterial({ color: 0x3fb7ff, transparent: true, opacity: .12, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false });
  const fill = own(new THREE.IcosahedronGeometry(1.1, 1), fillMaterial); core.add(fill);
  // GPU-driven motes rising inside the cone.
  const count = 160, seeds = new Float32Array(count);
  for (let i = 0; i < count; i++) seeds[i] = (i * .618033988749895) % 1;
  const moteGeometry = new THREE.BufferGeometry();
  moteGeometry.setAttribute('position', new THREE.BufferAttribute(new Float32Array(count * 3), 3));
  moteGeometry.setAttribute('seed', new THREE.BufferAttribute(seeds, 1));
  moteGeometry.boundingSphere = new THREE.Sphere(new THREE.Vector3(0, CONE_HEIGHT / 2, 0), 12);
  const moteMaterial = additive(MOTE_SHADER, { time: shared.time, pointScale: { value: 800 }, color: { value: pale }, strength: { value: .9 } });
  const motes = new THREE.Points(moteGeometry, moteMaterial); motes.frustumCulled = false; group.add(motes);
  geometries.push(moteGeometry); materials.push(moteMaterial);
  const light = new THREE.PointLight(0x62d8ff, 26, 46, 2); light.position.y = 6; group.add(light);
  // Text panels: one billboard main panel, two orbiting satellites with shorter fragments.
  function panel(width, height, cw, ch) {
    const canvas = document.createElement('canvas'); canvas.width = cw; canvas.height = ch;
    const context = canvas.getContext('2d');
    const texture = new THREE.CanvasTexture(canvas); texture.colorSpace = THREE.SRGBColorSpace; texture.generateMipmaps = false; texture.minFilter = THREE.LinearFilter;
    textures.push(texture);
    const material = additive(PANEL_SHADER, { map: { value: texture }, time: shared.time, glitch: { value: 0 }, fade: { value: 0 }, gain: { value: 1.35 }, tint: { value: new THREE.Color(0xffffff) } }, { side: THREE.FrontSide });
    const mesh = own(new THREE.PlaneGeometry(width, height), material);
    return { canvas, context, texture, material, mesh, text: '', targetFade: 1, next: 0 };
  }
  const main = panel(17.5, 8.55, 1024, 500), satellites = [panel(8.6, 2.1, 640, 156), panel(8.6, 2.1, 640, 156)];
  main.mesh.position.y = 10.4; group.add(main.mesh);
  satellites.forEach((s, i) => { s.mesh.position.y = i ? 14.4 : 5.9; s.orbit = i ? -.22 : .27; s.angle = i * 2.3; group.add(s.mesh); });
  function drawMain(text, index, total) {
    const { context: c, canvas } = main, w = canvas.width, h = canvas.height;
    c.clearRect(0, 0, w, h);
    c.fillStyle = 'rgba(48,150,214,0.14)'; c.beginPath(); c.roundRect(14, 14, w - 28, h - 28, 22); c.fill();
    c.strokeStyle = 'rgba(150,230,255,0.85)'; c.lineWidth = 3;
    for (const [x, y, dx, dy] of [[18, 18, 1, 1], [w - 18, 18, -1, 1], [18, h - 18, 1, -1], [w - 18, h - 18, -1, -1]]) {
      c.beginPath(); c.moveTo(x, y + dy * 42); c.lineTo(x, y); c.lineTo(x + dx * 42, y); c.stroke();
    }
    c.fillStyle = 'rgba(150,230,255,0.55)'; c.fillRect(48, 108, w - 96, 2);
    c.font = '600 27px ' + FONT; c.textBaseline = 'middle'; c.fillStyle = 'rgba(160,232,255,0.92)'; c.textAlign = 'left';
    c.fillText('▌ ' + label + (total ? '  ·  ' + String(index + 1).padStart(2, '0') + ' / ' + String(total).padStart(2, '0') : ''), 50, 72);
    c.textAlign = 'right'; c.font = '500 25px ' + FONT; c.fillStyle = 'rgba(190,150,255,0.85)';
    c.fillText('0x' + HASH(text || label).toUpperCase(), w - 52, 72);
    c.textAlign = 'left'; c.shadowColor = 'rgba(120,225,255,0.9)'; c.shadowBlur = 16;
    if (text) {
      c.font = '600 54px ' + FONT; c.fillStyle = 'rgba(232,250,255,0.97)';
      const lines = wrapText(c, text, w - 110, 4);
      lines.forEach((line, i) => c.fillText(line, 54, 172 + i * 72));
    } else {
      c.font = '600 40px ' + FONT; c.fillStyle = 'rgba(180,235,255,0.7)';
      c.fillText('▮ ▮ ▯ ▮ ▯ ▯ ▮ ▯ ▮ ▮ ▯ ▮', 54, 250);
    }
    c.shadowBlur = 0; main.texture.needsUpdate = true;
  }
  function drawSatellite(s, text) {
    const { context: c, canvas } = s, w = canvas.width, h = canvas.height;
    c.clearRect(0, 0, w, h);
    c.fillStyle = 'rgba(48,150,214,0.12)'; c.beginPath(); c.roundRect(6, 6, w - 12, h - 12, 14); c.fill();
    c.fillStyle = 'rgba(190,150,255,0.8)'; c.fillRect(20, 26, 6, h - 52);
    c.font = '600 42px ' + FONT; c.textBaseline = 'middle'; c.textAlign = 'left';
    c.shadowColor = 'rgba(160,140,255,0.9)'; c.shadowBlur = 12; c.fillStyle = 'rgba(236,240,255,0.95)';
    c.fillText(text ? wrapText(c, text, w - 70, 1)[0] : '▯ ▮ ▯ ▮ ▯', 42, h / 2);
    c.shadowBlur = 0; s.texture.needsUpdate = true;
  }
  function nextText() {
    if (!texts.length) return '';
    if (cursor >= queue.length) { queue = texts.map((_, i) => i).sort(() => Math.random() - .5); cursor = 0; }
    return texts[queue[cursor++]];
  }
  let mainTimer = .8, satelliteTimers = [3.5, 6.5];
  function showMain(animated) {
    const text = nextText(); main.text = text; switches++;
    drawMain(text, texts.indexOf(text), texts.length);
    main.material.uniforms.glitch.value = animated ? 1 : 0;
    main.material.uniforms.fade.value = animated ? .35 : 1;
    mainTimer = reducedMode ? 12 : 4.5 + Math.random() * 3;
  }
  function showSatellite(i, animated) {
    const s = satellites[i], text = nextText(); s.text = text;
    drawSatellite(s, text.length > 46 ? text.slice(0, 44).replace(/\s+\S*$/, '') + '…' : text);
    s.material.uniforms.glitch.value = animated ? .7 : 0;
    s.material.uniforms.fade.value = animated ? .3 : 1;
    satelliteTimers[i] = reducedMode ? 15 : 6 + Math.random() * 4;
  }
  drawMain('', 0, 0); satellites.forEach(s => drawSatellite(s, ''));
  document.fonts?.ready?.then(() => { if (!disposed) { drawMain(main.text, texts.indexOf(main.text), texts.length); satellites.forEach(s => drawSatellite(s, s.text)); } });
  const tmp = new THREE.Vector3();
  return {
    group,
    setTexts(list, from = 'live') {
      const clean = sanitizeArtifacts(list);
      const changed = clean.length !== texts.length || clean.some((t, i) => t !== texts[i]);
      texts = clean; source = clean.length ? from : 'none';
      if (changed) { queue = []; cursor = 0; if (!main.text || !texts.includes(main.text)) mainTimer = Math.min(mainTimer, .6); }
    },
    setPointScale(value) { moteMaterial.uniforms.pointScale.value = value; },
    setReducedMotion(value) { reducedMode = !!value; },
    update(dt, camera, animated) {
      if (disposed) return;
      const live = animated && !reducedMode, step = live ? Math.min(.1, Math.max(0, dt)) : 0;
      shared.time.value += step;
      mainTimer -= dt; if (mainTimer <= 0) showMain(live);
      satelliteTimers.forEach((t, i) => { satelliteTimers[i] = t - dt; if (satelliteTimers[i] <= 0) showSatellite(i, live); });
      for (const p of [main, ...satellites]) {
        const u = p.material.uniforms;
        u.glitch.value = Math.max(0, u.glitch.value - dt * 2.4);
        u.fade.value = Math.min(1, u.fade.value + dt * 1.8);
        p.mesh.quaternion.copy(camera.quaternion);
      }
      satellites.forEach(s => {
        s.angle += s.orbit * step;
        s.mesh.position.x = Math.cos(s.angle) * 6.4; s.mesh.position.z = Math.sin(s.angle) * 6.4;
        // Fragments facing away from the camera dim so the carousel never shows mirrored text.
        tmp.copy(s.mesh.position).add(group.position).sub(camera.position).normalize();
        const facing = -(tmp.x * Math.cos(s.angle) + tmp.z * Math.sin(s.angle));
        s.material.uniforms.gain.value = 1.35 * THREE.MathUtils.clamp(.35 + facing * .9, .15, 1.2);
      });
      main.mesh.position.y = 9.6 + Math.sin(shared.time.value * .9) * .22;
      core.rotation.y += step * .7; core.rotation.x += step * .31; fill.rotation.y -= step * 1.1;
      ringTop.rotation.z += step * .18; ringBase.rotation.z -= step * .42;
      light.intensity = 26 + Math.sin(shared.time.value * 2.1) * 6 + main.material.uniforms.glitch.value * 22;
    },
    stats: () => ({ artifacts: texts.length, source, switches, shown: main.text ? texts.indexOf(main.text) : -1 }),
    dispose() {
      if (disposed) return; disposed = true; group.removeFromParent();
      geometries.forEach(g => g.dispose()); materials.forEach(m => m.dispose()); textures.forEach(t => t.dispose()); light.dispose();
    },
  };
}
