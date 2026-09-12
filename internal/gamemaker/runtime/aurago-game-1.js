// AuraGo sprite/input helpers v1. Angles are radians; physics is owned by the game.
const usage = new WeakMap();
const visualBounds = new WeakMap();
const keyFor = meta => `${meta.id}@${meta.version}`;
const animKey = (meta, id) => `${keyFor(meta)}:${id}`;
function need(value, message) { if (!value) throw new Error(`Game assets: ${message}`); return value; }
function asset(meta, id) { return need(meta.assets.find(a => a.id === id), `${meta.id}: unknown asset ${id}`); }
function requireFrame(scene, meta, id, frame) {
  need(scene.textures.exists(keyFor(meta)), `${meta.id}/${id}: texture ${keyFor(meta)} is not loaded. Call preloadPack(this, meta, 'assets/builtin/${meta.id}/${meta.version}/sheet.png') inside preload(); create assets in setup()/create() after loading.`);
  const texture = scene.textures.get(keyFor(meta));
  need(Number.isInteger(frame) && texture.has(frame), `${meta.id}/${id}: missing numeric frame ${frame}; use preloadPack and the exact metadata frame.`);
}

export function preloadPack(scene, meta, imagePath) {
  need(meta.frame_width === 64 && meta.frame_height === 64 && meta.frames.length === 100, `${meta.id}: expected 100 frames of 64x64`);
  if (!scene.textures.exists(keyFor(meta))) scene.load.spritesheet(keyFor(meta), imagePath, { frameWidth: 64, frameHeight: 64 });
}
export function registerAnimations(scene, meta) {
	need(scene.textures.exists(keyFor(meta)), `${meta.id}: preloadPack must complete first`);
  for (const a of meta.animations) {
    need(a.frames.length && a.frames.every(i => Number.isInteger(i) && scene.textures.get(keyFor(meta)).has(i)), `${meta.id}/${a.id}: missing frame`);
    const key = animKey(meta, a.id);
    if (!scene.anims.exists(key)) scene.anims.create({ key, sortFrames: false, frames: a.frames.map(frame => ({ key: keyFor(meta), frame })), frameRate: a.frame_rate, repeat: a.repeat, yoyo: !!a.yoyo });
  }
}
export function createAsset(scene, meta, id, x, y) {
  const a = asset(meta, id);
  need(!a.assembly_part, `${id} is a fragment; use createAssembly`);
  requireFrame(scene, meta, id, a.frames[0]);
  const sprite = scene.add.sprite(x, y, keyFor(meta), a.frames[0]).setOrigin(a.origin.x, a.origin.y);
  usage.set(sprite, { meta, a, direction: a.direction, action: a.action || '', assembly: false });
  return sprite;
}
export function createAssembly(scene, meta, id, x, y) {
  const a = need((meta.assemblies || []).find(a => a.id === id), `${meta.id}: unknown assembly ${id}`);
  for (const p of a.parts) requireFrame(scene, meta, `${id}/${p.asset_id}`, p.frame);
  const container = scene.add.container(x, y).setSize(a.width, a.height);
  const parts = a.parts.map(p => {
    const sprite = scene.add.sprite(p.x - a.width * a.origin.x, p.y - a.height * a.origin.y, keyFor(meta), p.frame).setOrigin(0, 0);
    if (p.animation_id) sprite.play(animKey(meta, p.animation_id));
    container.add(sprite);
    return { sprite, part: p };
  });
  usage.set(container, { meta, a, parts, direction: a.direction, action: 'move', assembly: true });
  return container;
}

// Fit centered template art to a separate collision proxy, keeping pixel aspect.
// Return the anchor offset; do not change the pack's origin or physics geometry.
export function fitVisual(object, width, height) {
  const u = need(usage.get(object), 'fitVisual requires a library asset');
  need(width > 0 && height > 0 && Number.isFinite(width + height), 'fitVisual needs positive finite dimensions');
  let b = visualBounds.get(u.a);
  if (!b) {
    b = {x:0,y:0,w:object.width,h:object.height};
    if (!u.assembly) {
      const frame=object.frame, canvas=document.createElement('canvas');
      canvas.width=frame.width;canvas.height=frame.height;
      const ctx=canvas.getContext('2d', {willReadFrequently:true});
      ctx.drawImage(frame.source.image,frame.cutX,frame.cutY,frame.width,frame.height,0,0,frame.width,frame.height);
      const pixels=ctx.getImageData(0,0,frame.width,frame.height).data;
      let left=frame.width,top=frame.height,right=-1,bottom=-1;
      for(let y=0;y<frame.height;y++)for(let x=0;x<frame.width;x++)if(pixels[(y*frame.width+x)*4+3]){left=Math.min(left,x);right=Math.max(right,x);top=Math.min(top,y);bottom=Math.max(bottom,y);}
      need(right>=left, `${u.meta.id}/${u.a.id}: empty sprite frame`);
      b={x:left,y:top,w:right-left+1,h:bottom-top+1};
    }
    visualBounds.set(u.a,b);
  }
  const scale=Math.min(width/b.w,height/b.h);
  object.setScale(scale);
  const offset={x:(object.width*u.a.origin.x-b.x-b.w/2)*scale,y:(object.height*u.a.origin.y-b.y-b.h/2)*scale,width:b.w*scale,height:b.h*scale};
  object.setPosition(object.x+offset.x,object.y+offset.y);
  return offset;
}
export function playAction(object, action) {
  const u = need(usage.get(object), 'object was not created by an asset helper');
  if (u.assembly) {
    need(action === 'move' || action === 'idle', `${u.a.id}: assembly supports move/idle only`);
    for (const { sprite, part } of u.parts) {
      if (!part.animation_id) continue;
      if (action === 'idle') { sprite.anims.stop(); sprite.setFrame(part.frame); }
      else sprite.play(animKey(u.meta, part.animation_id), true);
    }
    u.action = action;
    return object;
  }
  const candidates = u.meta.animations.filter(a => (a.entity ? a.entity === u.a.entity : a.asset_id === u.a.id) && a.action === action);
  const animation = candidates.find(a => a.direction === u.direction) || candidates.find(a => a.direction === u.a.direction && u.a.transform.mode !== 'directional');
  need(animation, `${u.a.id}: action ${action}/${u.direction} unavailable; available: ${u.meta.animations.filter(a => a.entity === u.a.entity).map(a => `${a.action}/${a.direction}`).join(', ') || 'none'}`);
  object.play(animKey(u.meta, animation.id), true);
  u.action = action;
  return object;
}
export function setFacing(object, dx, dy) {
  need(Number.isFinite(dx) && Number.isFinite(dy), 'direction vector must be finite');
  if (dx === 0 && dy === 0) return object; // Keep the last idle direction.
  const u = need(usage.get(object), 'object was not created by an asset helper');
  const rule = need(u.a.transform, `${u.a.id}: transform metadata missing; import current pack version`);
  const direction = Math.abs(dx) >= Math.abs(dy) ? (dx < 0 ? 'left' : 'right') : (dy < 0 ? 'up' : 'down');
  if (rule.mode === 'rotate') object.setRotation(Math.atan2(dy, dx) - rule.forward_radians);
  else if (rule.mode === 'directional') {
    u.direction = direction;
    if (u.action) playAction(object, u.action);
    else {
      const variant = need(u.meta.assets.find(a => a.entity === u.a.entity && a.direction === direction), `${u.a.id}: no ${direction} view`);
      object.setFrame(variant.frames[0]);
    }
  } else if (rule.flip_x && (direction === 'left' || direction === 'right')) {
    const flip = direction !== u.a.direction;
    if (u.assembly) object.setScale((flip ? -1 : 1) * Math.abs(object.scaleX), object.scaleY);
    else {
      object.setFlipX(flip);
      // Phaser flips around the texture center, so mirror the anchor as well.
      object.setOrigin(flip ? 1 - u.a.origin.x : u.a.origin.x, u.a.origin.y);
    }
  } else need(direction === u.a.direction, `${u.a.id}: ${direction} forbidden by transform metadata`);
  u.direction = direction;
  return object;
}

// Keys belong to the scene. No persistent window listeners survive restart.
export function createInputs(scene) {
  const keys = scene.input.keyboard.addKeys('LEFT,RIGHT,UP,DOWN,W,A,S,D,SPACE,R,P,ESC,ENTER');
  scene.events.once('shutdown', () => {
    for (const key of Object.values(keys)) scene.input.keyboard.removeKey(key, true);
  });
  const held=new Set(), edges=new Set();
  const touch=document.createElement('div');
  touch.className='aurago-game-touch';
  const style=document.createElement('style');
  style.textContent='.aurago-game-touch{display:none;position:absolute;left:8px;right:8px;bottom:max(8px,env(safe-area-inset-bottom));gap:5px;z-index:1000;pointer-events:none;flex-wrap:wrap}.aurago-game-touch button{pointer-events:auto;touch-action:none;min-width:44px;min-height:44px;border:1px solid #8eafc666;border-radius:8px;background:#152635e8;color:#fff;font:14px system-ui}@media(pointer:coarse),(max-width:650px){.aurago-game-touch{display:flex}}';
  touch.append(style);
  const cleanup=new AbortController();
  for(const [label,name] of [['◀','LEFT'],['▲','UP'],['▼','DOWN'],['▶','RIGHT'],['Action','SPACE'],['Pause','P'],['Restart','R']]) {
    const button=document.createElement('button');button.textContent=label;button.setAttribute('aria-label',name);
    button.addEventListener('pointerdown',event=>{event.preventDefault();button.setPointerCapture(event.pointerId);held.add(name);edges.add(name)},{signal:cleanup.signal});
    const release=()=>held.delete(name);
    button.addEventListener('pointerup',release,{signal:cleanup.signal});button.addEventListener('pointercancel',release,{signal:cleanup.signal});touch.append(button);
  }
  (document.getElementById('game-root')||document.body).append(touch);
  window.addEventListener('blur',()=>{held.clear();edges.clear()},{signal:cleanup.signal});
  scene.events.once('shutdown',()=>{cleanup.abort();touch.remove();held.clear();edges.clear()});
  const down=(...names)=>names.some(name=>keys[name].isDown||held.has(name));
  return {
    keys, down,
    vector: () => ({x:Number(down('RIGHT','D'))-Number(down('LEFT','A')),y:Number(down('DOWN','S'))-Number(down('UP','W'))}),
    pressed: name => {const key=need(keys[name],`unknown input ${name}`),edge=edges.delete(name);return globalThis.Phaser.Input.Keyboard.JustDown(key)||edge}
  };
}

// Bind observable state, not a success verdict. The server compares snapshots.
// state: { score, actions, hits, spawns, turns, ticks, ended }; player is the live object.
export function bindGameTest(scene, state, player) {
  if (!scene?.sys || !state || !player?.active || player.scene !== scene) {
    throw new Error('bindGameTest(scene,state,player): player must be a live object in this scene. In GameScene.setup() assign this.player = this.paddle (or the controlled object) before returning. Preserve common.ts create/update and rebind after every scene restart.');
  }
  const binding = { scene, state, player };
  globalThis.__AURAGO_GAME_TEST__ = binding;
  scene.events.once('shutdown', () => { if (globalThis.__AURAGO_GAME_TEST__ === binding) delete globalThis.__AURAGO_GAME_TEST__; });
}

export function inspectAssets(scene) {
  let invalid = 0, used = 0;
  const inspect = object => {
    const u = usage.get(object);
    if (u) {
      used++;
      if (u.assembly) {
        for (const { sprite, part } of u.parts) {
          if (Math.abs(sprite.x - (part.x - u.a.width * u.a.origin.x)) > .01 || Math.abs(sprite.y - (part.y - u.a.height * u.a.origin.y)) > .01 || sprite.body) invalid++;
        }
      } else if (!object.frame || object.frame.width !== 64 || object.frame.height !== 64) invalid++;
      if (u.a.transform?.mode !== 'rotate' && object.rotation !== 0) invalid++;
      if (!u.a.transform?.flip_x && object.flipX || !u.a.transform?.flip_y && object.flipY) invalid++;
    }
    if (object.texture?.key?.includes('@') && object.frame?.name === '__BASE') invalid++;
    if (object.texture?.key === '__MISSING') invalid++;
    if (object.list) object.list.forEach(inspect);
  };
  scene.children.list.forEach(inspect);
  return { assets_used: used, invalid_assets: invalid };
}
