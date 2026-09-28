// MIT. Text-only authored labels, native controls, and bounded touch actions.
const keys=['inventory','craft','close','mine','place','jump','health','saved','saving','saveError','conflict','reload','reset','respawn','help','loading','creative','survival','full','needTool','goals','complete','local','transient','empty'];
const translations={
 en:['Inventory','Craft','Close','Mine / attack','Place','Jump','Health','Saved','Saving…','Save failed','A newer save exists on another device','Load latest','Reset world','Respawn','WASD: move · Mouse / drag: look · Space: jump\nLeft click: mine / attack · Right click: place\n1–9: select · I: inventory / crafting · Esc: pause','Loading world…','Creative','Survival','Inventory full','A stronger tool is required','Goals','Complete','Saved on this device','Temporary preview','Empty'],
 de:['Inventar','Herstellen','Schließen','Abbauen / angreifen','Platzieren','Springen','Gesundheit','Gespeichert','Wird gespeichert…','Speichern fehlgeschlagen','Auf einem anderen Gerät liegt ein neuerer Spielstand vor','Neuesten Stand laden','Welt zurücksetzen','Wieder erscheinen','WASD: bewegen · Maus / ziehen: umsehen · Leertaste: springen\nLinksklick: abbauen / angreifen · Rechtsklick: platzieren\n1–9: auswählen · I: Inventar / Herstellung · Esc: Pause','Welt wird geladen…','Kreativ','Überleben','Inventar voll','Du brauchst ein stärkeres Werkzeug','Ziele','Abgeschlossen','Auf diesem Gerät gespeichert','Flüchtige Vorschau','Leer'],
 cs:['Inventář','Vyrobit','Zavřít','Těžit / útočit','Umístit','Skok','Zdraví','Uloženo','Ukládání…','Uložení selhalo','Na jiném zařízení je novější hra','Načíst nejnovější','Obnovit svět','Znovu se objevit','WASD: pohyb · Myš / tažení: pohled · Mezerník: skok\nLevé tlačítko: těžit / útočit · Pravé: umístit\n1–9: výběr · I: inventář / výroba · Esc: pauza','Načítání světa…','Kreativní','Přežití','Inventář je plný','Potřebuješ silnější nástroj','Cíle','Hotovo','Uloženo v tomto zařízení','Dočasný náhled','Prázdné'],
 da:['Inventar','Fremstil','Luk','Bryd / angrib','Placér','Hop','Helbred','Gemt','Gemmer…','Kunne ikke gemme','En nyere gemt verden findes på en anden enhed','Indlæs nyeste','Nulstil verden','Genopstå','WASD: bevægelse · Mus / træk: se omkring · Mellemrum: hop\nVenstreklik: bryd / angrib · Højreklik: placér\n1–9: vælg · I: inventar / fremstilling · Esc: pause','Indlæser verden…','Kreativ','Overlevelse','Inventaret er fuldt','Et stærkere værktøj kræves','Mål','Fuldført','Gemt på denne enhed','Midlertidig forhåndsvisning','Tom'],
 el:['Αποθήκη','Κατασκευή','Κλείσιμο','Εξόρυξη / επίθεση','Τοποθέτηση','Άλμα','Υγεία','Αποθηκεύτηκε','Αποθήκευση…','Αποτυχία αποθήκευσης','Υπάρχει νεότερη αποθήκευση σε άλλη συσκευή','Φόρτωση νεότερης','Επαναφορά κόσμου','Επανεμφάνιση','WASD: κίνηση · Ποντίκι / σύρσιμο: θέα · Κενό: άλμα\nΑριστερό κλικ: εξόρυξη / επίθεση · Δεξί: τοποθέτηση\n1–9: επιλογή · I: αποθήκη / κατασκευή · Esc: παύση','Φόρτωση κόσμου…','Δημιουργικό','Επιβίωση','Η αποθήκη είναι πλήρης','Απαιτείται ισχυρότερο εργαλείο','Στόχοι','Ολοκληρώθηκε','Αποθηκεύτηκε σε αυτή τη συσκευή','Προσωρινή προεπισκόπηση','Κενό'],
 es:['Inventario','Fabricar','Cerrar','Minar / atacar','Colocar','Saltar','Salud','Guardado','Guardando…','No se pudo guardar','Hay una partida más reciente en otro dispositivo','Cargar última','Reiniciar mundo','Reaparecer','WASD: mover · Ratón / arrastrar: mirar · Espacio: saltar\nClic izquierdo: minar / atacar · Derecho: colocar\n1–9: elegir · I: inventario / fabricación · Esc: pausa','Cargando mundo…','Creativo','Supervivencia','Inventario lleno','Necesitas una herramienta mejor','Objetivos','Completado','Guardado en este dispositivo','Vista previa temporal','Vacío'],
 fr:['Inventaire','Fabriquer','Fermer','Miner / attaquer','Placer','Sauter','Santé','Enregistré','Enregistrement…','Échec de sauvegarde','Une sauvegarde plus récente existe sur un autre appareil','Charger la dernière','Réinitialiser le monde','Réapparaître','WASD : déplacement · Souris / glisser : regarder · Espace : sauter\nClic gauche : miner / attaquer · Droit : placer\n1–9 : choisir · I : inventaire / fabrication · Échap : pause','Chargement du monde…','Créatif','Survie','Inventaire plein','Un outil plus puissant est nécessaire','Objectifs','Terminé','Enregistré sur cet appareil','Aperçu temporaire','Vide'],
 hi:['सामान','बनाएँ','बंद करें','खनन / हमला','रखें','कूदें','स्वास्थ्य','सहेजा गया','सहेज रहे हैं…','सहेजना विफल','दूसरे उपकरण पर नया सहेजा हुआ खेल है','नवीनतम लोड करें','दुनिया रीसेट करें','फिर प्रकट हों','WASD: चलें · माउस / खींचें: देखें · स्पेस: कूदें\nबायाँ क्लिक: खनन / हमला · दायाँ: रखें\n1–9: चुनें · I: सामान / निर्माण · Esc: विराम','दुनिया लोड हो रही है…','रचनात्मक','जीवित रहना','सामान भरा है','अधिक शक्तिशाली औज़ार चाहिए','लक्ष्य','पूरा','इस उपकरण पर सहेजा गया','अस्थायी पूर्वावलोकन','खाली'],
 it:['Inventario','Crea','Chiudi','Scava / attacca','Piazza','Salta','Salute','Salvato','Salvataggio…','Salvataggio fallito','Esiste un salvataggio più recente su un altro dispositivo','Carica ultimo','Reimposta mondo','Rinasci','WASD: muovi · Mouse / trascina: guarda · Spazio: salta\nClic sinistro: scava / attacca · Destro: piazza\n1–9: scegli · I: inventario / creazione · Esc: pausa','Caricamento del mondo…','Creativa','Sopravvivenza','Inventario pieno','Serve un attrezzo migliore','Obiettivi','Completato','Salvato su questo dispositivo','Anteprima temporanea','Vuoto'],
 ja:['持ち物','作成','閉じる','採掘 / 攻撃','設置','ジャンプ','体力','保存済み','保存中…','保存に失敗しました','別の端末に新しいセーブデータがあります','最新を読み込む','ワールドをリセット','復活','WASD：移動 · マウス / ドラッグ：視点 · Space：ジャンプ\n左クリック：採掘 / 攻撃 · 右クリック：設置\n1–9：選択 · I：持ち物 / 作成 · Esc：一時停止','ワールドを読み込み中…','クリエイティブ','サバイバル','持ち物がいっぱいです','より強い道具が必要です','目標','完了','この端末に保存','一時プレビュー','空'],
 nl:['Inventaris','Maken','Sluiten','Delven / aanvallen','Plaatsen','Springen','Gezondheid','Opgeslagen','Opslaan…','Opslaan mislukt','Een ander apparaat heeft een nieuwere opslag','Nieuwste laden','Wereld resetten','Terugkeren','WASD: bewegen · Muis / slepen: kijken · Spatie: springen\nLinksklik: delven / aanvallen · Rechtsklik: plaatsen\n1–9: kiezen · I: inventaris / maken · Esc: pauze','Wereld laden…','Creatief','Overleven','Inventaris vol','Je hebt sterker gereedschap nodig','Doelen','Voltooid','Opgeslagen op dit apparaat','Tijdelijk voorbeeld','Leeg'],
 no:['Inventar','Lag','Lukk','Grav / angrip','Plasser','Hopp','Helse','Lagret','Lagrer…','Lagring mislyktes','En nyere lagring finnes på en annen enhet','Last inn nyeste','Tilbakestill verden','Gjenoppstå','WASD: bevegelse · Mus / dra: se rundt · Mellomrom: hopp\nVenstreklikk: grav / angrip · Høyreklikk: plasser\n1–9: velg · I: inventar / laging · Esc: pause','Laster verden…','Kreativ','Overlevelse','Inventaret er fullt','Et sterkere verktøy kreves','Mål','Fullført','Lagret på denne enheten','Midlertidig forhåndsvisning','Tom'],
 pl:['Ekwipunek','Wytwórz','Zamknij','Kop / atakuj','Postaw','Skok','Zdrowie','Zapisano','Zapisywanie…','Zapis nie powiódł się','Na innym urządzeniu jest nowszy zapis','Wczytaj najnowszy','Zresetuj świat','Odrodzenie','WASD: ruch · Mysz / przeciąganie: widok · Spacja: skok\nLewy przycisk: kop / atakuj · Prawy: postaw\n1–9: wybór · I: ekwipunek / wytwarzanie · Esc: pauza','Ładowanie świata…','Kreatywny','Przetrwanie','Ekwipunek pełny','Potrzebujesz lepszego narzędzia','Cele','Ukończono','Zapisano na tym urządzeniu','Podgląd tymczasowy','Puste'],
 pt:['Inventário','Criar','Fechar','Minerar / atacar','Colocar','Saltar','Saúde','Guardado','A guardar…','Falha ao guardar','Há um jogo mais recente noutro dispositivo','Carregar mais recente','Repor mundo','Renascer','WASD: mover · Rato / arrastar: olhar · Espaço: saltar\nClique esquerdo: minerar / atacar · Direito: colocar\n1–9: escolher · I: inventário / criação · Esc: pausa','A carregar mundo…','Criativo','Sobrevivência','Inventário cheio','É necessária uma ferramenta melhor','Objetivos','Concluído','Guardado neste dispositivo','Pré-visualização temporária','Vazio'],
 sv:['Inventarium','Tillverka','Stäng','Bryt / angrip','Placera','Hoppa','Hälsa','Sparat','Sparar…','Det gick inte att spara','En nyare sparning finns på en annan enhet','Läs in senaste','Återställ värld','Återuppstå','WASD: rörelse · Mus / dra: titta · Mellanslag: hoppa\nVänsterklick: bryt / angrip · Högerklick: placera\n1–9: välj · I: inventarium / tillverkning · Esc: paus','Läser in världen…','Kreativ','Överlevnad','Inventariet är fullt','Ett starkare verktyg krävs','Mål','Slutfört','Sparat på denna enhet','Tillfällig förhandsvisning','Tom'],
 zh:['物品栏','制作','关闭','挖掘 / 攻击','放置','跳跃','生命','已保存','正在保存…','保存失败','其他设备上有更新的存档','加载最新存档','重置世界','重生','WASD：移动 · 鼠标 / 拖动：视角 · 空格：跳跃\n左键：挖掘 / 攻击 · 右键：放置\n1–9：选择 · I：物品栏 / 制作 · Esc：暂停','正在加载世界…','创造','生存','物品栏已满','需要更强的工具','目标','已完成','已保存在此设备','临时预览','空'],
};
export function voxelLabels(locale=(navigator.language||'en').split('-')[0]){return Object.fromEntries(keys.map((k,i)=>[k,(translations[locale]||translations.en)[i]]));}
export function createVoxelUI({root,getGame,key,toggle,craft,select,swap,supply,reload,reset}) {
  const t=voxelLabels(),controller=new AbortController(),signal=controller.signal;
  const layer=document.createElement('div');layer.dataset.voxelUi='';
  const style=document.createElement('style');style.textContent=`
    [data-voxel-ui]{position:absolute;inset:0;pointer-events:none;color:#f2f6fa;font:14px/1.4 system-ui;z-index:1045}
    [data-voxel-ui] [hidden]{display:none!important}[data-voxel-ui] button{pointer-events:auto;color:inherit;font:inherit;background:#152536ee;border:1px solid #bed0db66;border-radius:8px;min-height:44px;padding:6px 10px;cursor:pointer;touch-action:none}
    [data-voxel-ui] button:focus-visible{outline:3px solid #9cf}[data-voxel-ui] button:disabled{opacity:.45;cursor:default}
    [data-voxel-hotbar]{position:absolute;bottom:12px;left:50%;transform:translateX(-50%);display:flex;gap:3px;max-width:96%;overflow-x:auto;pointer-events:auto}
    [data-voxel-hotbar] button{box-sizing:border-box;flex:0 0 48px;height:48px;position:relative;font-size:11px;padding:4px;touch-action:pan-x}
    [data-voxel-icon]{display:block;margin:auto;width:25px;height:25px;font:23px/25px system-ui;border-radius:3px;box-shadow:inset -4px -4px #0002,inset 3px 3px #fff2}
    [data-voxel-number]{position:absolute;top:1px;left:3px;font-size:10px;text-shadow:0 1px 2px #000}
    [data-voxel-count]{position:absolute;bottom:1px;right:3px;font-size:10px;text-shadow:0 1px 2px #000}
    [data-voxel-hotbar] [aria-pressed=true]{outline:2px solid #ffe09b;background:#3e5245}
    [data-voxel-hud]{position:absolute;top:12px;left:12px;max-width:70%;display:grid;justify-items:start;gap:6px}
    [data-voxel-status]{padding:7px 11px;border-radius:8px;background:#142433ce}
    [data-voxel-inventory]{position:absolute;inset:0;display:grid;place-items:center;background:#09131dbb;pointer-events:auto}
    [data-voxel-card]{box-sizing:border-box;width:min(660px,94%);max-height:92%;overflow:auto;background:#122231;border:1px solid #cedae455;border-radius:16px;padding:16px}
    [data-voxel-slots]{display:grid;grid-template-columns:repeat(9,minmax(0,1fr));gap:4px}[data-voxel-slots] button{font-size:11px;padding:3px;overflow-wrap:anywhere}
    [data-voxel-recipes]{display:flex;flex-direction:column;gap:6px;margin-top:14px}[data-voxel-recipes] button{text-align:left}
    [data-voxel-actions]{position:absolute;right:14px;bottom:80px;display:grid;grid-template-columns:1fr 1fr;gap:6px;max-width:45%}
    [data-voxel-save]{background:#142433ed;border-radius:8px;padding:6px;max-width:440px;pointer-events:auto}
    [data-voxel-crosshair]{position:absolute;left:50%;top:50%;transform:translate(-50%,-50%);font:24px monospace;text-shadow:0 1px 3px #000}
    #game-root:has([data-voxel-ui]) [data-player-stick]{bottom:max(70px,env(safe-area-inset-bottom))}
    @media(max-width:600px){[data-voxel-hud]{top:72px;max-width:calc(100% - 88px)}[data-voxel-slots]{grid-template-columns:repeat(6,minmax(0,1fr))}}
    @media(pointer:coarse){[data-voxel-hotbar]{bottom:4px}[data-voxel-hotbar] button{flex-basis:44px}}`;
  layer.append(style);
  const el=(tag,attr,parent=layer)=>{const e=document.createElement(tag);e.setAttribute('data-voxel-'+attr,'');parent.append(e);return e;};
  const button=(text,fn,parent)=>{const b=document.createElement('button');b.type='button';b.textContent=text;b.addEventListener('click',fn,{signal});parent.append(b);return b;};
  const hud=el('div','hud'),status=el('div','status',hud),crosshair=el('div','crosshair');crosshair.textContent='+';
  const hotbar=el('div','hotbar');hotbar.setAttribute('role','group');hotbar.setAttribute('aria-label',t.inventory);
  const slots=Array.from({length:9},(_,i)=>{const b=button('',()=>select(i),hotbar);el('span','number',b).textContent=String(i+1);el('span','icon',b);el('span','count',b);return b;});
  const inventory=el('section','inventory');inventory.hidden=true;inventory.setAttribute('role','dialog');inventory.setAttribute('aria-modal','true');inventory.setAttribute('aria-label',t.inventory);
  const card=el('div','card',inventory),heading=document.createElement('h2');heading.textContent=t.inventory;card.append(heading);
  const close=button(t.close,toggle,card),grid=el('div','slots',card);
  const gridSlots=Array.from({length:36},(_,i)=>button('',()=>swap(i),grid)),recipes=el('div','recipes',card);
  for(const r of getGame().definition.recipes||[]){const b=button('',()=>craft(r.id),recipes);b.dataset.recipe=r.id;}
  if(getGame().creative){const palette=el('div','palette',card);palette.setAttribute('role','group');palette.setAttribute('aria-label',t.creative);for(const item of getGame().definition.items)button(item.name,()=>supply(item.id),palette);}
  button(t.reload,reload,card);button(t.reset,()=>{if(confirmReset.hidden){confirmReset.hidden=false;return;}},card);
  const confirmReset=button(t.reset+'?',reset,card);confirmReset.hidden=true;
  const actions=el('div','actions');const held=new Map();let touch=matchMedia('(pointer:coarse)').matches;
  function release(id){const name=held.get(id);held.delete(id);if(name&&![...held.values()].includes(name))key(name,false);}
  for(const [name,label] of [['primary',t.mine],['place',t.place],['jump',t.jump],['inventory',t.inventory]]){
    const b=button(label,()=>{},actions);b.dataset.voxelAction=name;
    b.addEventListener('pointerdown',e=>{e.preventDefault();b.setPointerCapture(e.pointerId);held.set(e.pointerId,name);key(name,true);},{signal});
    for(const type of ['pointerup','pointercancel','lostpointercapture'])b.addEventListener(type,e=>release(e.pointerId),{signal});
  }
  window.addEventListener('pointerdown',e=>{touch=e.pointerType==='touch'||e.pointerType==='pen';},{capture:true,signal});
  const save=el('div','save',hud);save.setAttribute('role','status');save.hidden=true;
  let opened=false,lastRevision=-1,lastFocus=null;
  inventory.addEventListener('keydown',e=>{if(e.key!=='Tab')return;const buttons=[...inventory.querySelectorAll('button')].filter(b=>!b.disabled&&!b.hidden);const first=buttons[0],last=buttons.at(-1);if(e.shiftKey&&document.activeElement===first){e.preventDefault();last.focus();}else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first.focus();}},{signal});
  function slotText(s,i){return `${i<9?i+1+' ':''}${s?(getGame().items.get(s.item)?.name||s.item)+' ×'+s.count:t.empty}`;}
  const api={labels:t,
    open(value){opened=value;inventory.hidden=!value;confirmReset.hidden=true;api.release();if(value){lastFocus=document.activeElement;lastRevision=-1;api.update(false);close.focus();}else lastFocus?.focus?.();},
    update(blocked){const g=getGame();hotbar.hidden=crosshair.hidden=status.hidden=blocked||opened;actions.hidden=blocked||opened||!touch;
      let text=g.creative?t.creative:`${t.health} ${Math.ceil(g.player.health)} / 100`;
      if(g.held())text+=' · '+g.held().name;
      if(g.definition.goals?.length)text+=' · '+g.definition.goals.map(goal=>`${goal.id}: ${Math.min(goal.count,g.progress[goal.kind+(goal.item?':'+goal.item:'')]||0)}/${goal.count}`).join(' · ');
      if(status.textContent!==text)status.textContent=text;
      slots.forEach((b,i)=>{const slot=g.inventory[i],text=slotText(slot,i),selected=String(i===g.selected);if(b.title!==text){b.title=text;b.setAttribute('aria-label',text);const item=g.items.get(slot?.item),icon=b.querySelector('[data-voxel-icon]');icon.style.backgroundColor=item?.block?g.world.blocks.get(item.block).color:'transparent';icon.style.opacity=item?'1':'.2';icon.textContent=item&&!item.block?(item.tier?'⛏':'◆'):'';b.querySelector('[data-voxel-count]').textContent=slot?String(slot.count):'';}if(b.getAttribute('aria-pressed')!==selected)b.setAttribute('aria-pressed',selected);});
      if(opened&&lastRevision!==g.revision){lastRevision=g.revision;gridSlots.forEach((b,i)=>{b.textContent=slotText(g.inventory[i],i);});
        for(const b of recipes.children){const r=g.recipes.get(b.dataset.recipe);b.textContent=t.craft+': '+g.items.get(r.item).name+' ×'+r.count+' — '+Object.entries(r.ingredients).map(([id,n])=>`${g.items.get(id).name} ${g.count(id)}/${n}`).join(', ');b.disabled=!g.creative&&Object.entries(r.ingredients).some(([id,n])=>g.count(id)<n);}}
    },
    saveStatus(kind){save.hidden=!kind;save.textContent=t[kind]||t.saveError;save.setAttribute('role',['saveError','conflict'].includes(kind)?'alert':'status');},
    release(){for(const id of [...held.keys()])release(id);},
    dispose(){api.release();controller.abort();layer.remove();},
  };
  root.append(layer);return api;
}
