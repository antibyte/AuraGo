(function () {
    'use strict';
    function create(root, options) {
        const {tr, signal} = options, key = 'aurago:synth-studio:view:1';
        let view = { panel: 'notes', height: 290, library: true, compactLibrary:false };
        try { const saved = JSON.parse(localStorage.getItem(key)); if (saved) view = { ...view, ...saved }; } catch (_) {}
        view.height = Math.max(180, Math.min(560, Number(view.height) || 290));
        if (!['notes','sound','performance'].includes(view.panel)) view.panel = 'notes';
        const host = root.querySelector('[data-editor-host]'), note = host.querySelector('[data-note-editor]');
        const tabs = document.createElement('div'); tabs.className = 'ss-workbench-tabs'; tabs.setAttribute('role','tablist');
        for (const name of ['notes','sound','performance']) {
            const button = document.createElement('button'); button.type = 'button'; button.dataset.workTab = name;
            button.textContent = tr('workspace_' + name); button.setAttribute('role','tab'); tabs.append(button);
        }
        const grip = document.createElement('div'); grip.className = 'ss-workbench-grip'; grip.tabIndex = 0;
        grip.setAttribute('role','separator'); grip.setAttribute('aria-orientation','horizontal'); grip.setAttribute('aria-label',tr('resize'));
        grip.setAttribute('aria-valuemin','180'); grip.setAttribute('aria-valuemax','560');
        host.insertBefore(grip,note); host.insertBefore(tabs,note);
        note.dataset.workPanel = 'notes';
        const sound = document.createElement('section'); sound.className = 'ss-sound-workbench'; sound.dataset.workPanel = 'sound';
        sound.append(root.querySelector('[data-inspector]')); host.append(sound);
        const performance = document.createElement('section'); performance.className = 'ss-performance-workbench'; performance.dataset.workPanel = 'performance'; host.append(performance);
        const save = () => { try { localStorage.setItem(key, JSON.stringify(view)); } catch (_) {} };
        // On a wide desktop the notes panel and the sound workbench sit next to
        // each other, so the instrument and its effect chain stay in view.
        const wide=()=>root.clientWidth>=1400;
        let lastWide=wide();
        function render() {
            const isWide=wide();lastWide=isWide;
            // The wide workbench hides the sound tab. A selection that still
            // points at it would leave the tab list without a focusable tab.
            const active=isWide&&view.panel==='sound'?'notes':view.panel;
            root.dataset.workPanel = view.panel;root.classList.toggle('ss-library-open',view.compactLibrary===true); root.classList.toggle('ss-library-collapsed',!view.library);
            root.classList.toggle('ss-wide',isWide);
            root.style.setProperty('--ss-workbench-height',view.height+'px');
            grip.setAttribute('aria-valuenow',String(view.height));
            tabs.querySelectorAll('button').forEach(b => { b.setAttribute('aria-selected',String(b.dataset.workTab===active)); b.tabIndex=b.dataset.workTab===active?0:-1; });
            const visible=name=>isWide?(name==='sound'?true:name==='notes'?view.panel!=='performance':view.panel==='performance'):name===view.panel;
            host.querySelectorAll('[data-work-panel]').forEach(p=>{p.hidden=!visible(p.dataset.workPanel);p.setAttribute('role','tabpanel');});
            root.querySelectorAll('[data-action="library"]').forEach(b=>b.setAttribute('aria-expanded',String(root.clientWidth<=540?!!view.compactLibrary:!!view.library)));
        }
        function panel(name) { view.panel=name; render(); save(); }
        tabs.addEventListener('click',e=>{const b=e.target.closest('[data-work-tab]');if(b)panel(b.dataset.workTab);},{signal});
        tabs.addEventListener('keydown',e=>{if(!['ArrowLeft','ArrowRight','Home','End'].includes(e.key))return;e.preventDefault();
            // The wide workbench hides the sound tab, so it must not stay in the
            // rotation: focusing a hidden button would leave the focus behind.
            const names=wide()?['notes','performance']:['notes','sound','performance'];
            let i=names.indexOf(view.panel);if(i<0)i=0;
            i=e.key==='Home'?0:e.key==='End'?names.length-1:(i+(e.key==='ArrowRight'?1:names.length-1))%names.length;
            panel(names[i]);tabs.querySelector('[data-work-tab="'+names[i]+'"]').focus();},{signal});
        let resize = null;
        grip.addEventListener('pointerdown',e=>{if(e.button!==0)return;resize={y:e.clientY,height:view.height};grip.setPointerCapture(e.pointerId);e.preventDefault();},{signal});
        grip.addEventListener('pointermove',e=>{if(resize){view.height=Math.max(180,Math.min(560,resize.height+resize.y-e.clientY));render();}},{signal});
        grip.addEventListener('pointerup',()=>{resize=null;save();},{signal});
        grip.addEventListener('pointercancel',()=>{if(resize)view.height=resize.height;resize=null;render();},{signal});
        grip.addEventListener('keydown',e=>{if(!['ArrowUp','ArrowDown'].includes(e.key))return;e.preventDefault();view.height=Math.max(180,Math.min(560,view.height+(e.key==='ArrowUp'?20:-20)));render();save();},{signal});
        const welcome = document.createElement('div'); welcome.className='ss-welcome';
        const title=document.createElement('strong');title.textContent=tr('start_loop');welcome.append(title);
        for(const name of ['electronic','chill','retro']) {const b=document.createElement('button');b.dataset.template=name;b.textContent=tr('template_'+name);b.type='button';welcome.append(b);}
        for(const action of ['new','open']){const b=document.createElement('button');b.dataset.action=action;b.type='button';b.textContent=tr(action);welcome.append(b);}
        if(typeof options.onTour==='function'){const b=document.createElement('button');b.dataset.action='tour';b.type='button';b.textContent=tr('tour');welcome.append(b);}
        root.querySelector('.ss-main').prepend(welcome);
        render();
        // The wide workbench depends on the window width. Without an observer the
        // layout would keep the old mode until the user switches a tab.
        if(typeof ResizeObserver==='function'){
            const observer=new ResizeObserver(()=>{if(wide()!==lastWide)render();});
            observer.observe(root);
            if(signal&&typeof signal.addEventListener==='function')signal.addEventListener('abort',()=>observer.disconnect(),{once:true});
        }
        return { panel, performance, sound, welcome, toggleLibrary(){if(root.clientWidth<=540)view.compactLibrary=!view.compactLibrary;else view.library=!view.library;render();save();}, refresh(){welcome.hidden=options.project().tracks.length>0;welcome.querySelectorAll('[data-template]').forEach(b=>b.disabled=options.blocked());} };
    }
    window.SynthStudioWorkspace={create};
})();
