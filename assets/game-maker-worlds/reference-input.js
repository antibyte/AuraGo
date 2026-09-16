// MIT. One input owner, released with the scene/game. No synthetic game events.
export function referenceInput(onCommand){
 const abort=new AbortController(),keys=new Set();
 const down=key=>{if(!keys.has(key)&&['KeyP','KeyR'].includes(key))onCommand(key);keys.add(key)},up=key=>keys.delete(key);
 window.addEventListener('keydown',e=>{if(['Space','ArrowUp','ArrowDown','ArrowLeft','ArrowRight'].includes(e.code))e.preventDefault();down(e.code)},{signal:abort.signal});
 window.addEventListener('keyup',e=>up(e.code),{signal:abort.signal});
 window.addEventListener('blur',()=>keys.clear(),{signal:abort.signal});
 for(const button of document.querySelectorAll('[data-key]')){
  button.addEventListener('pointerdown',e=>{button.setPointerCapture(e.pointerId);down(button.dataset.key)},{signal:abort.signal});
  for(const event of ['pointerup','pointercancel','lostpointercapture'])button.addEventListener(event,()=>up(button.dataset.key),{signal:abort.signal});
 }
 return {keys,axis:()=>[(+keys.has('ArrowRight')||+keys.has('KeyD'))-(+keys.has('ArrowLeft')||+keys.has('KeyA')),(+keys.has('ArrowDown')||+keys.has('KeyS'))-(+keys.has('ArrowUp')||+keys.has('KeyW'))],dispose(){abort.abort();keys.clear()}};
}
