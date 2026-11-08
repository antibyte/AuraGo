import { TABLES, ASSET_ROOT, tableOf } from './tables.js';

function el(tag, text, className) { const node=document.createElement(tag);if(text!==undefined)node.textContent=text;if(className)node.className=className;return node; }
function button(label, action, primary=false) { const node=el('button',label,primary?'pb-button pb-primary':'pb-button');node.type='button';node.dataset.action=action;return node; }
function label(text, control) { const node=el('label',undefined,'pb-field');node.append(el('span',text),control);return node; }

export function createView(host, tr, assetURL) {
    const root=el('section',undefined,'pb-app');root.tabIndex=0;root.setAttribute('aria-label','AuraPinball');
    const stage=el('div',undefined,'pb-stage'),chrome=el('header',undefined,'pb-toolbar'),brand=el('strong','AURA / PINBALL','pb-brand');
    const camera=el('select');camera.dataset.setting='camera';camera.setAttribute('aria-label',tr('camera'));
    for(const key of ['play','top','showcase']){const option=el('option',tr(key));option.value=key;camera.append(option);}
    chrome.append(brand,camera,button(tr('fullscreen'),'fullscreen'),button(tr('pause_action'),'pause'));
    const hud=el('div',undefined,'pb-hud'),scoreboard=el('div',undefined,'pb-scoreboard'),missions=el('aside',undefined,'pb-missions'),bottom=el('div',undefined,'pb-bottom');
    const ball=el('span'),locks=el('span'),charge=el('progress');charge.max=1;charge.value=0;charge.setAttribute('aria-label',tr('plunger'));
    bottom.append(ball,locks,charge);hud.append(scoreboard,missions,bottom);
    const status=el('div',undefined,'pb-status');status.setAttribute('role','status');status.setAttribute('aria-live','polite');
    const overlay=el('div',undefined,'pb-overlay');root.append(stage,chrome,hud,status,overlay);host.replaceChildren(root);
    let kind='loading',serial=-1,currentGame=null;
    const number=new Intl.NumberFormat(document.documentElement.lang||'de');
    function panel(next,data={}) {
        kind=next;root.dataset.panel=next;overlay.replaceChildren();overlay.hidden=!next;hud.hidden=!data.game;
        chrome.hidden=next==='loading'||next==='error';
        if(!next){root.focus({preventScroll:true});return;}
        const card=el('div',undefined,'pb-panel');card.setAttribute('role',next==='lobby'?'region':'dialog');card.setAttribute('aria-label',next==='lobby'?'AuraPinball':tr(next==='binding'?'settings':next==='error'?'load_error':next));overlay.append(card);
        const heading=el('h1',next==='lobby'?'AuraPinball':tr(next==='error'?(data.error||'load_error'):next==='binding'?'settings':next));card.append(heading);
        if(next==='loading'){card.append(el('p',tr('loading')));return;}
        if(next==='error'){card.append(button(tr('retry'),'retry',true));return;}
        if(next==='lobby'){
            card.classList.add('pb-lobby');card.append(el('p',tr('tagline'),'pb-tagline'));
            const choices=el('div',undefined,'pb-tables');
            for(const table of TABLES){
                const choice=button('', 'table-'+table.id);choice.classList.add('pb-table');choice.setAttribute('aria-pressed',String(data.table===table.id));
                const image=el('img');image.src=assetURL(ASSET_ROOT+table.id+'-concept.webp');image.alt='';image.width=200;image.height=240;
                choice.append(image,el('strong',table.title),el('span',tr(table.id+'_description')));choices.append(choice);
            }
            card.append(choices);
            const setup=el('div',undefined,'pb-setup'),count=el('select');count.dataset.field='players';
            for(let n=1;n<=4;n++){const option=el('option',String(n));option.value=n;count.append(option);}
            count.value=String(data.count||1);setup.append(label(tr('players'),count));
            for(let i=0;i<(data.count||1);i++){const input=el('input');input.maxLength=48;input.dataset.player=i;input.value=data.names?.[i]||tr('player').replace('{n}',String(i+1));input.setAttribute('aria-label',tr('player').replace('{n}',String(i+1)));setup.append(input);}
            card.append(setup);
            const actions=el('div',undefined,'pb-actions');for(const [key,act] of [['start','start'],['training','training'],['scores','scores'],['settings','settings'],['help','help']])actions.append(button(tr(key),act,key==='start'));card.append(actions);
        }else if(next==='pause'){
            card.append(el('p',tr(data.reason||'paused_hint')));
            for(const key of ['resume','settings','help','restart','quit'])card.append(button(tr(key),key,key==='resume'));
        }else if(next==='showcase'){
            card.classList.add('pb-showcase');card.append(el('p',tr('showcase_hint')),button(tr('resume'),'resume',true));
        }else if(next==='help'){
            card.append(el('p',tr('help_text'),'pb-help-text'),el('p',tr('missions_hint'),'pb-help-text'));
            const credits=el('p',undefined,'pb-credits');credits.append(el('span','Audio: Kenney · CC0 1.0. '));
            const source=el('a','Kenney Impact Sounds');source.href='https://kenney.nl/assets/impact-sounds';source.target='_blank';source.rel='noopener noreferrer';credits.append(source);card.append(credits,button(tr('back'),'back',true));
        }else if(next==='settings'){
            const quality=el('select');quality.dataset.setting='quality';
            for(const key of ['low','medium','high']){const option=el('option',tr(key));option.value=key;quality.append(option);}quality.value=data.settings.quality;card.append(label(tr('quality'),quality));
            for(const key of ['effects','ambience']){const input=el('input');input.type='range';input.min=0;input.max=1;input.step=.01;input.value=data.settings[key];input.dataset.setting=key;card.append(label(tr(key),input));}
            const reduced=el('input');reduced.type='checkbox';reduced.checked=data.settings.reduced;reduced.dataset.setting='reduced';card.append(label(tr('reduced'),reduced));
            const bindings=el('div',undefined,'pb-bindings');
            for(const device of ['keys','pad']){bindings.append(el('h3',tr(device)));
                for(const [key,code] of Object.entries(data.settings[device])){const row=el('div',undefined,'pb-binding-row');row.append(el('span',tr(key==='pause'?'pause_action':key)),button(device==='keys'?code:tr('button').replace('{n}',String(code+1)),'bind-'+device+'-'+key));bindings.append(row);}}
            card.append(bindings,button(tr('back'),'back',true));
        }else if(next==='binding'){
            card.append(el('p',tr(data.device==='keys'?'press_key':'press_pad')),button(tr('cancel'),'cancel_binding'));
        }else if(next==='scores'||next==='game_over'){
            const rows=next==='game_over'?data.game.players:[...(data.scores?.[data.table]||[])];
            card.append(el('h2',tableOf(data.table).title));
            const list=el('ol',undefined,'pb-score-list');
            for(const row of rows){const li=el('li');li.append(el('span',row.name),el('strong',number.format(row.score)));list.append(li);}
            if(!rows.length)card.append(el('p',tr('no_scores')));else card.append(list);
            card.append(button(tr(next==='game_over'?'quit':'back'),next==='game_over'?'lobby':'back',true));
        }else if(next==='confirm_exit'){
            card.append(el('p',tr('exit_text')),button(tr('cancel'),'back'),button(tr(data.restart?'restart':'quit'),data.restart?'confirm_restart':'lobby',true));
        }
        requestAnimationFrame(()=>{if(overlay.contains(card))card.querySelector('button,input,select')?.focus({preventScroll:true});});
    }
    function updateHUD(game,snapshot){
        if(!game)return;currentGame=game;const p=game.players[game.turn],table=tableOf(game.table);
        if(scoreboard.children.length!==game.players.length){scoreboard.replaceChildren(...game.players.map(()=>{const node=el('div',undefined,'pb-player-score');node.append(el('span'),el('strong'));return node;}));}
        game.players.forEach((player,index)=>{const node=scoreboard.children[index];node.dataset.active=String(index===game.turn);node.firstChild.textContent=player.name;node.lastChild.textContent=number.format(player.score);});
        ball.textContent=tr('ball')+' '+(game.training?'∞':Math.min(3,p.played+1)+' / 3');locks.textContent=tr('locks')+' '+p.locks+' / 3';charge.value=snapshot?.pull||0;
        const signature=game.table+':'+p.progress.join(',')+':'+p.finale+':'+game.multiball;
        if(missions.dataset.signature!==signature){missions.dataset.signature=signature;missions.replaceChildren(el('h2',tr('missions')));
            table.goals.forEach((goal,i)=>{const row=el('div',undefined,'pb-mission');row.dataset.complete=String(p.completed[i]);const progress=el('progress');progress.max=goal.count;progress.value=p.progress[i];progress.setAttribute('aria-label',tr(table.missions[i]));row.append(el('span',tr(table.missions[i])),progress);missions.append(row);});
            if(p.finale)missions.append(el('strong',tr(table.finale),'pb-finale'));
        }
        if(serial!==game.noticeSerial){serial=game.noticeSerial;status.textContent=tr(game.notice==='training'?'training_notice':game.notice);status.dataset.event=game.notice;}
    }
    return {root,stage,panel,updateHUD,kind:()=>kind,notice(key){status.textContent=tr(key);},names:()=>[...root.querySelectorAll('[data-player]')].map(input=>input.value),focus:()=>root.focus({preventScroll:true}),dispose(){root.remove();}};
}
