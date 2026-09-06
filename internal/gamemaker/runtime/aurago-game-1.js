// AuraGo sprite/input helpers v1. Angles are radians; physics is owned by the game.
const usage = new WeakMap();
const keyFor = meta => `${meta.id}@${meta.version}`;
const animKey = (meta, id) => `${keyFor(meta)}:${id}`;
function need(value, message) { if (!value) throw new Error(`Game assets: ${message}`); return value; }
function asset(meta, id) { return need(meta.assets.find(a => a.id === id), `${meta.id}: unknown asset ${id}`); }

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
  const sprite = scene.add.sprite(x, y, keyFor(meta), a.frames[0]).setOrigin(a.origin.x, a.origin.y);
  usage.set(sprite, { meta, a, direction: a.direction, action: a.action || '', assembly: false });
  return sprite;
}
export function createAssembly(scene, meta, id, x, y) {
  const a = need((meta.assemblies || []).find(a => a.id === id), `${meta.id}: unknown assembly ${id}`);
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
  const keys = scene.input.keyboard.addKeys('LEFT,RIGHT,UP,DOWN,W,A,S,D,SPACE,R,ESC,ENTER');
  scene.events.once('shutdown', () => {
    for (const key of Object.values(keys)) scene.input.keyboard.removeKey(key, true);
  });
  return {
    keys,
    vector: () => ({ x: Number(keys.RIGHT.isDown || keys.D.isDown) - Number(keys.LEFT.isDown || keys.A.isDown), y: Number(keys.DOWN.isDown || keys.S.isDown) - Number(keys.UP.isDown || keys.W.isDown) }),
    pressed: name => { const key = need(keys[name], `unknown input ${name}`); return globalThis.Phaser.Input.Keyboard.JustDown(key); }
  };
}

// Bind observable state, not a success verdict. The server compares snapshots.
// state: { score, actions, hits, spawns, turns, ticks, ended }; player is the live object.
export function bindGameTest(scene, state, player) {
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
    if (object.list) object.list.forEach(inspect);
  };
  scene.children.list.forEach(inspect);
  return { assets_used: used, invalid_assets: invalid };
}
