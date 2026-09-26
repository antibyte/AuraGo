// MIT. Shared player onboarding and touch input; no game loop or gameplay state.
const startedRoots = new WeakSet();
const translations = {
  en: ['Start game','Paused','Continue','Restart','Pause','Move','Action','Jump','Fire','Interact','Launch','Boost','Reload','Descend','Ascend','Tap a cell','Drag to look','Left stick: move · Right side: actions'],
  de: ['Spiel starten','Pause','Fortsetzen','Neu starten','Pausieren','Bewegen','Aktion','Springen','Feuern','Interagieren','Starten','Schub','Nachladen','Sinken','Steigen','Tippe auf ein Feld','Ziehen zum Umsehen','Linker Stick: bewegen · Rechts: Aktionen'],
  cs: ['Spustit hru','Pozastaveno','Pokračovat','Restartovat','Pozastavit','Pohyb','Akce','Skok','Střelba','Interakce','Vypustit','Zrychlit','Přebít','Klesat','Stoupat','Klepni na políčko','Rozhlížej se tažením','Levý ovladač: pohyb · Vpravo: akce'],
  da: ['Start spil','På pause','Fortsæt','Start igen','Pause','Bevægelse','Handling','Hop','Skyd','Interagér','Affyr','Boost','Genlad','Ned','Op','Tryk på et felt','Træk for at se dig omkring','Venstre styrepind: bevægelse · Højre: handlinger'],
  el: ['Έναρξη παιχνιδιού','Σε παύση','Συνέχεια','Επανεκκίνηση','Παύση','Κίνηση','Ενέργεια','Άλμα','Πυρ','Αλληλεπίδραση','Εκτόξευση','Ώθηση','Επαναγέμιση','Κάθοδος','Άνοδος','Πάτησε ένα κελί','Σύρε για να κοιτάξεις','Αριστερός μοχλός: κίνηση · Δεξιά: ενέργειες'],
  es: ['Iniciar juego','En pausa','Continuar','Reiniciar','Pausar','Mover','Acción','Saltar','Disparar','Interactuar','Lanzar','Impulso','Recargar','Descender','Ascender','Toca una casilla','Arrastra para mirar','Palanca izquierda: mover · Derecha: acciones'],
  fr: ['Lancer le jeu','En pause','Continuer','Recommencer','Pause','Déplacement','Action','Sauter','Tirer','Interagir','Lancer','Accélérer','Recharger','Descendre','Monter','Touche une case','Glisse pour regarder','Stick gauche : déplacement · À droite : actions'],
  hi: ['खेल शुरू करें','विराम','जारी रखें','फिर शुरू करें','रोकें','चलें','क्रिया','कूदें','गोली चलाएँ','बातचीत करें','छोड़ें','तेज़ करें','फिर भरें','नीचे जाएँ','ऊपर जाएँ','किसी खाने पर टैप करें','देखने के लिए खींचें','बायाँ स्टिक: चलना · दाईं ओर: क्रियाएँ'],
  it: ['Avvia gioco','In pausa','Continua','Ricomincia','Pausa','Movimento','Azione','Salta','Spara','Interagisci','Lancia','Spinta','Ricarica','Scendi','Sali','Tocca una casella','Trascina per guardarti intorno','Levetta sinistra: movimento · A destra: azioni'],
  ja: ['ゲーム開始','一時停止中','続ける','やり直す','一時停止','移動','アクション','ジャンプ','射撃','調べる','発射','加速','リロード','下降','上昇','マスをタップ','ドラッグで視点移動','左スティック：移動 · 右側：アクション'],
  nl: ['Spel starten','Gepauzeerd','Doorgaan','Opnieuw starten','Pauzeren','Bewegen','Actie','Springen','Schieten','Interactie','Lanceren','Versnellen','Herladen','Dalen','Stijgen','Tik op een vak','Sleep om rond te kijken','Linkerstick: bewegen · Rechts: acties'],
  no: ['Start spill','På pause','Fortsett','Start på nytt','Pause','Bevegelse','Handling','Hopp','Skyt','Samhandle','Avfyr','Boost','Lad om','Ned','Opp','Trykk på et felt','Dra for å se deg rundt','Venstre spak: bevegelse · Høyre: handlinger'],
  pl: ['Rozpocznij grę','Wstrzymano','Kontynuuj','Zacznij od nowa','Pauza','Ruch','Akcja','Skok','Strzał','Interakcja','Wystrzel','Przyspiesz','Przeładuj','W dół','W górę','Dotknij pola','Przeciągnij, aby się rozejrzeć','Lewy drążek: ruch · Po prawej: akcje'],
  pt: ['Iniciar jogo','Em pausa','Continuar','Recomeçar','Pausar','Mover','Ação','Saltar','Disparar','Interagir','Lançar','Impulso','Recarregar','Descer','Subir','Toca numa casa','Arrasta para olhar','Manípulo esquerdo: mover · Direita: ações'],
  sv: ['Starta spel','Pausat','Fortsätt','Börja om','Pausa','Förflyttning','Handling','Hoppa','Skjut','Interagera','Avfyra','Öka fart','Ladda om','Nedåt','Uppåt','Tryck på en ruta','Dra för att se dig omkring','Vänster spak: förflyttning · Höger: handlingar'],
  zh: ['开始游戏','已暂停','继续','重新开始','暂停','移动','操作','跳跃','射击','互动','发射','加速','换弹','下降','上升','轻触格子','拖动以环顾','左摇杆：移动 · 右侧：操作'],
};
const labelKeys = ['start','paused','resume','restart','pause','move','action','jump','fire','interact','launch','boost','reload','descend','ascend','board','look','touch'];

export function createPlayerUI({root, objective = '', mode = 'minimal', movement, action, instructions, key = () => {}, change = () => {}, restart = () => {}, clear = () => {}}) {
  // Export scaffolds carry lang=en; player chrome follows the player's browser.
  const locale = (navigator.language || document.documentElement.lang || 'en').split('-')[0];
  const labels = Object.fromEntries(labelKeys.map((name,i) => [name,(translations[locale] || translations.en)[i]]));
  const controller = new AbortController(), signal = controller.signal;
  const coarse = matchMedia('(pointer: coarse)');
  let touch = coarse.matches, started = startedRoots.has(root), paused = false, active = true, ended = false;
  let stickPointer = null;
  const held = new Map();
  movement ??= mode === 'board' ? 'none' : ['platformer','blocks'].includes(mode) ? 'horizontal' : 'full';
  action ??= ({board:false,platformer:'jump',blocks:'launch',shooter:'fire',fps:'fire',space:'fire',topdown:'interact',exploration:'boost',transport:'boost',flight:'boost'})[mode] ?? 'action';
  const layer = document.createElement('div');
  layer.dataset.playerUi = 'true';
  const style = document.createElement('style');
  style.textContent = `
    [data-player-ui]{position:absolute;inset:0;z-index:1040;pointer-events:none;color:#f3f7ff;font:15px/1.5 system-ui}
    [data-player-ui] [hidden]{display:none!important}
    [data-player-ui] button{font:inherit;color:inherit;background:#16283bea;border:1px solid #bcd5ef66;border-radius:12px;min-width:48px;min-height:48px;padding:10px 16px;cursor:pointer;touch-action:none;pointer-events:auto}
    [data-player-ui] button:focus-visible{outline:3px solid #a5e8ff;outline-offset:3px}
    [data-player-panel]{position:absolute;inset:0;display:grid;place-items:center;padding:16px;box-sizing:border-box;background:#07111aa8;pointer-events:auto;touch-action:manipulation}
    [data-player-card]{width:min(440px,100%);max-height:100%;box-sizing:border-box;overflow:auto;padding:clamp(16px,4vw,28px);border:1px solid #bbd6f344;border-radius:18px;background:#101c2bf5;box-shadow:0 18px 60px #0007}
    [data-player-card] h2{font-size:24px;margin:0 0 12px}[data-player-card] p{margin:0 0 18px;white-space:pre-line;overflow-wrap:anywhere}
    [data-player-help]{color:#c1d3e8;font-size:14px}[data-player-menu]{display:flex;gap:10px;flex-wrap:wrap}
    [data-player-pause]{position:absolute;right:max(12px,env(safe-area-inset-right));top:72px;padding:6px!important}
    [data-player-touch]{position:absolute;inset:0;pointer-events:none}
    [data-player-stick]{position:absolute;left:max(16px,env(safe-area-inset-left));bottom:max(16px,env(safe-area-inset-bottom));width:112px;height:112px;border-radius:50%;background:#14243899;border:1px solid #cce5ff66;pointer-events:auto;touch-action:none;user-select:none;display:grid;place-items:center}
    [data-player-stick] span{display:grid;place-items:center;width:48px;height:48px;border-radius:50%;background:#d1eaff66;font-size:22px;pointer-events:none}
    [data-player-actions]{position:absolute;right:max(16px,env(safe-area-inset-right));bottom:max(16px,env(safe-area-inset-bottom));display:grid;grid-template-columns:repeat(2,minmax(48px,auto));gap:8px;max-width:calc(100% - 164px)}
    [data-player-actions] button{padding:10px;overflow-wrap:anywhere}[data-player-actions] button:first-child{grid-column:1/-1}
    @media(max-width:650px){.aurago-player-hud{top:72px!important;max-width:calc(100% - 84px)!important}}
    @media(max-height:420px){[data-player-stick]{width:96px;height:96px}[data-player-actions]{gap:4px}}
  `;
  const panel = document.createElement('section');
  panel.dataset.playerPanel = 'intro'; panel.setAttribute('role','dialog'); panel.setAttribute('aria-modal','true');
  const card = document.createElement('div'); card.dataset.playerCard = '';
  const title = document.createElement('h2'), goal = document.createElement('p'), help = document.createElement('p'), menu = document.createElement('div');
  help.dataset.playerHelp = ''; menu.dataset.playerMenu = ''; goal.textContent = String(objective);
  const button = (label, fn, parent) => {const b=document.createElement('button');b.type='button';b.textContent=label;b.addEventListener('click',fn,{signal});parent.append(b);return b;};
  const play = button(labels.start, () => resume(), menu); play.dataset.playerStart = '';
  const retry = button(labels.restart, () => {resume();restart();}, menu);
  const pause = button('Ⅱ', () => togglePause(), layer); pause.dataset.playerPause = ''; pause.setAttribute('aria-label',labels.pause); pause.title=labels.pause;
  card.append(title,goal,help,menu);panel.append(card);
  const controls=document.createElement('div');controls.dataset.playerTouch='';
  const stick=document.createElement('div');stick.dataset.playerStick='';stick.setAttribute('role','group');stick.setAttribute('aria-label',labels.move);
  const knob=document.createElement('span');knob.textContent=movement==='horizontal'?'↔':'✥';stick.append(knob);
  const actions=document.createElement('div');actions.dataset.playerActions='';controls.append(stick,actions);
  const actionLabel = labels[action] || String(action || '');
  function hold(id, names) {
    const before=new Set([...held.values()].flat());
    if(names.length)held.set(id,names);else held.delete(id);
    const after=new Set([...held.values()].flat());
    for(const name of before)if(!after.has(name))key(name,false);
    for(const name of after)if(!before.has(name))key(name,true);
  }
  function release(clearKeys = true) {for(const id of [...held.keys()])hold(id,[]);stickPointer=null;knob.style.transform='';if(clearKeys)clear();}
  function bindAction(name, label) {
    const b=button(label,()=>{},actions);b.dataset.playerKey=name;b.setAttribute('aria-label',label);
    b.addEventListener('pointerdown',e=>{if(!started||paused||!active||ended)return;e.preventDefault();b.setPointerCapture(e.pointerId);hold(e.pointerId,[name]);},{signal});
    for(const event of ['pointerup','pointercancel','lostpointercapture'])b.addEventListener(event,e=>hold(e.pointerId,[]),{signal});
  }
  if(action)bindAction('SPACE',actionLabel);
  if(mode==='fps')bindAction('F',labels.reload);
  if(['flight','space'].includes(mode)){bindAction('Q','↓ '+labels.descend);bindAction('E','↑ '+labels.ascend);}
  function moveStick(e) {
    if(e.pointerId!==stickPointer)return;
    const r=stick.getBoundingClientRect(),x=(e.clientX-r.left-r.width/2)/(r.width/2),y=movement==='horizontal'?0:(e.clientY-r.top-r.height/2)/(r.height/2);
    const length=Math.max(1,Math.hypot(x,y));knob.style.transform=`translate(${x/length*30}px,${y/length*30}px)`;
    const names=[];if(x<-.25)names.push('LEFT');if(x>.25)names.push('RIGHT');if(y<-.25)names.push('UP');if(y>.25)names.push('DOWN');hold(e.pointerId,names);
  }
  stick.addEventListener('pointerdown',e=>{if(stickPointer!==null||!started||paused||!active||ended)return;e.preventDefault();stickPointer=e.pointerId;stick.setPointerCapture(e.pointerId);moveStick(e);},{signal});
  stick.addEventListener('pointermove',moveStick,{signal});
  for(const event of ['pointerup','pointercancel','lostpointercapture'])stick.addEventListener(event,e=>{if(e.pointerId!==stickPointer)return;hold(e.pointerId,[]);stickPointer=null;knob.style.transform='';},{signal});
  function render() {
    panel.hidden=started&&!paused;panel.dataset.playerPanel=started?'pause':'intro';
    title.textContent=started?labels.paused:labels.start;panel.setAttribute('aria-label',title.textContent);
    play.textContent=started?labels.resume:labels.start;retry.hidden=!started;
    const moveKeys=mode==='board'?'←↑↓→':mode==='fps'?'WASD':movement==='horizontal'?'A/D / ←→':'WASD / ←↑↓→';
    help.textContent=instructions || (touch ? mode==='board'?labels.board:labels.touch+(mode==='fps'?'\n'+labels.look:'') :
      `${movement!=='none'||mode==='board'?moveKeys+': '+labels.move:''}${action||mode==='board'?` · Space: ${actionLabel||labels.action}`:''}${mode==='board'?' · '+labels.board:''}${mode==='fps'?'\n'+labels.look+' · ←↑↓→ · F: '+labels.reload:''}${['flight','space'].includes(mode)?' · Q/E: '+labels.descend+'/'+labels.ascend:''}\nP: ${labels.pause} · R: ${labels.restart}`);
    controls.hidden=!touch||!started||paused||!active||ended;
    stick.hidden=movement==='none';pause.hidden=!started||paused||!active||ended;
  }
  function resume() {
    if(!active)return;started=true;startedRoots.add(root);paused=false;release();render();change(false);
    root.querySelector('canvas')?.focus();
  }
  function togglePause() {if(!started||ended||!active)return;paused=!paused;release();render();change(paused);if(paused)play.focus({preventScroll:true});}
  function selectTouch(value) {if(touch===value)return;touch=value;release();render();}
  window.addEventListener('pointerdown',e=>selectTouch(e.pointerType==='touch'||e.pointerType==='pen'),{signal,capture:true});
  window.addEventListener('keydown',e=>{if(e.key!=='Enter'||e.repeat||started&&!paused||e.target instanceof HTMLButtonElement)return;e.preventDefault();e.stopImmediatePropagation();resume();},{signal,capture:true});
  coarse.addEventListener('change',e=>selectTouch(e.matches),{signal});
  window.addEventListener('blur',release,{signal});
  document.addEventListener('visibilitychange',()=>{if(document.hidden)release();},{signal});
  layer.append(style,panel,controls);root.append(layer);render();
  change(!started);
  return {
    get blocked(){return !started||paused;}, get started(){return started;},
    togglePause,
    // Finishing releases touch holds without discarding a queued R/Enter key.
    sync(visible, finished=false){if(active===visible&&ended===finished)return;active=visible;ended=finished;if(!active||ended)release(!active);render();},
    reset(){paused=false;ended=false;release();render();change(!started);},
    dispose(){release();controller.abort();layer.remove();},
  };
}
