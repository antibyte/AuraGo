(function () {
    'use strict';
    function create(root, options) {
        const {tr, signal} = options, key = 'aurago:synth-studio:view:1';
        // The workbench height is stored in pixels; zero means "share the space
        // automatically". The library rail and the sound column keep the size
        // the user dragged them to.
        const LIMITS = { height: [160, 900], libraryWidth: [160, 460], panelWidth: [260, 640] };
        let view = { panel: 'notes', height: 0, library: true, compactLibrary:false, libraryWidth: 220, panelWidth: 360 };
        try { const saved = JSON.parse(localStorage.getItem(key)); if (saved) view = { ...view, ...saved }; } catch (_) {}
        const clamp = (value, min, max) => Math.max(min, Math.min(max, Math.round(Number(value) || min)));
        view.height = Math.max(0, Math.min(LIMITS.height[1], Math.round(Number(view.height) || 0)));
        view.libraryWidth = clamp(view.libraryWidth, LIMITS.libraryWidth[0], LIMITS.libraryWidth[1]);
        view.panelWidth = clamp(view.panelWidth, LIMITS.panelWidth[0], LIMITS.panelWidth[1]);
        if (!['notes','sound','performance','strudel'].includes(view.panel)) view.panel = 'notes';
        const host = root.querySelector('[data-editor-host]'), note = host.querySelector('[data-note-editor]');
        const tabs = document.createElement('div'); tabs.className = 'ss-workbench-tabs'; tabs.setAttribute('role','tablist');
        for (const name of ['notes','sound','performance','strudel']) {
            const button = document.createElement('button'); button.type = 'button'; button.dataset.workTab = name;
            button.textContent = tr('workspace_' + name); button.setAttribute('role','tab'); tabs.append(button);
        }
        const grip = document.createElement('div'); grip.className = 'ss-workbench-grip'; grip.tabIndex = 0;
        grip.setAttribute('role','separator'); grip.setAttribute('aria-orientation','horizontal'); grip.setAttribute('aria-label',tr('resize_layout'));
        grip.setAttribute('aria-valuemin',String(LIMITS.height[0])); grip.setAttribute('aria-valuemax',String(LIMITS.height[1]));
        host.insertBefore(grip,note); host.insertBefore(tabs,note);
        note.dataset.workPanel = 'notes';
        const sound = document.createElement('section'); sound.className = 'ss-sound-workbench'; sound.dataset.workPanel = 'sound';
        sound.append(root.querySelector('[data-inspector]')); host.append(sound);
        const performance = document.createElement('section'); performance.className = 'ss-performance-workbench'; performance.dataset.workPanel = 'performance'; host.append(performance);
        const strudel = document.createElement('section'); strudel.dataset.workPanel = 'strudel'; host.append(strudel);
        // Two draggable separators: the library rail on the left and the sound
        // column on the right. Both remember their size and can be keyboard
        // driven or reset with a double click.
        const libraryGrip = document.createElement('div'); libraryGrip.className = 'ss-splitter ss-splitter-library'; libraryGrip.tabIndex = 0;
        libraryGrip.setAttribute('role','separator'); libraryGrip.setAttribute('aria-orientation','vertical'); libraryGrip.setAttribute('aria-label',tr('resize_layout'));
        libraryGrip.setAttribute('aria-valuemin',String(LIMITS.libraryWidth[0])); libraryGrip.setAttribute('aria-valuemax',String(LIMITS.libraryWidth[1]));
        const workspaceEl = root.querySelector('.ss-workspace');
        workspaceEl.insertBefore(libraryGrip,workspaceEl.querySelector('.ss-main'));
        const panelGrip = document.createElement('div'); panelGrip.className = 'ss-splitter ss-splitter-panel'; panelGrip.tabIndex = 0;
        panelGrip.setAttribute('role','separator'); panelGrip.setAttribute('aria-orientation','vertical'); panelGrip.setAttribute('aria-label',tr('resize_layout'));
        panelGrip.setAttribute('aria-valuemin',String(LIMITS.panelWidth[0])); panelGrip.setAttribute('aria-valuemax',String(LIMITS.panelWidth[1]));
        host.insertBefore(panelGrip,sound);
        const save = () => { try { localStorage.setItem(key, JSON.stringify(view)); } catch (_) {} };
        // On a wide desktop the notes panel and the sound workbench sit next to
        // each other, so the instrument and its effect chain stay in view.
        const wide=()=>root.clientWidth>=1400;
        let lastWide=wide();
        // The workbench never eats the arrangement: whatever is left above the
        // minimum row height stays with the track list.
        function workbenchHeight(){
            const available=host.clientHeight||0;
            const max=Math.max(LIMITS.height[0],available-170);
            // The stacked layout shares one column between the arrangement and
            // the workbench. The arrangement keeps its minimum height there,
            // so the workbench may use everything above it.
            if(view.height>0)return Math.min(view.height,max);
            return Math.round(Math.min(Math.max(available*.45,LIMITS.height[0]),max));
        }
        function applyLayout(){
            const height=workbenchHeight();
            root.style.setProperty('--ss-workbench-height',height+'px');
            root.style.setProperty('--ss-library-width',view.libraryWidth+'px');
            root.style.setProperty('--ss-panel-width',view.panelWidth+'px');
            grip.setAttribute('aria-valuenow',String(height));
            libraryGrip.setAttribute('aria-valuenow',String(view.libraryWidth));
            panelGrip.setAttribute('aria-valuenow',String(view.panelWidth));
        }
        function render() {
            const isWide=wide();lastWide=isWide;
            // The wide workbench hides the sound tab. A selection that still
            // points at it would leave the tab list without a focusable tab.
            const active=isWide&&view.panel==='sound'?'notes':view.panel;
            root.dataset.workPanel = view.panel;root.classList.toggle('ss-library-open',view.compactLibrary===true); root.classList.toggle('ss-library-collapsed',!view.library);
            root.classList.toggle('ss-wide',isWide);
            applyLayout();
            tabs.querySelectorAll('button').forEach(b => { b.setAttribute('aria-selected',String(b.dataset.workTab===active)); b.tabIndex=b.dataset.workTab===active?0:-1; });
            const visible=name=>isWide?(name==='sound'||name===(view.panel==='sound'?'notes':view.panel)):name===view.panel;
            host.querySelectorAll('[data-work-panel]').forEach(p=>{p.hidden=!visible(p.dataset.workPanel);p.setAttribute('role','tabpanel');});
            root.querySelectorAll('[data-action="library"]').forEach(b=>b.setAttribute('aria-expanded',String(root.clientWidth<=540?!!view.compactLibrary:!!view.library)));
        }
        function panel(name) { view.panel=name; render(); save(); }
        tabs.addEventListener('click',e=>{const b=e.target.closest('[data-work-tab]');if(b)panel(b.dataset.workTab);},{signal});
        tabs.addEventListener('keydown',e=>{if(!['ArrowLeft','ArrowRight','Home','End'].includes(e.key))return;e.preventDefault();
            // The wide workbench hides the sound tab, so it must not stay in the
            // rotation: focusing a hidden button would leave the focus behind.
            const names=wide()?['notes','performance','strudel']:['notes','sound','performance','strudel'];
            let i=names.indexOf(view.panel);if(i<0)i=0;
            i=e.key==='Home'?0:e.key==='End'?names.length-1:(i+(e.key==='ArrowRight'?1:names.length-1))%names.length;
            panel(names[i]);tabs.querySelector('[data-work-tab="'+names[i]+'"]').focus();},{signal});
        // Every separator behaves the same way: drag with the mouse, nudge with
        // the arrow keys, double click to fall back to the default size.
        function makeSplitter(element,config){
            let drag=null;
            const read=config.read;
            element.addEventListener('pointerdown',e=>{
                if(e.button!==0)return;
                drag={origin:config.axis==='x'?e.clientX:e.clientY,value:read()};
                element.setPointerCapture(e.pointerId);element.classList.add('is-dragging');e.preventDefault();
            },{signal});
            element.addEventListener('pointermove',e=>{
                if(!drag)return;
                const delta=(config.axis==='x'?e.clientX:e.clientY)-drag.origin;
                config.write(drag.value+(config.invert?-delta:delta));
            },{signal});
            element.addEventListener('pointerup',()=>{if(!drag)return;drag=null;element.classList.remove('is-dragging');save();},{signal});
            element.addEventListener('pointercancel',()=>{if(!drag)return;config.write(drag.value);drag=null;element.classList.remove('is-dragging');render();},{signal});
            element.addEventListener('dblclick',()=>{config.write(config.reset());save();},{signal});
            element.addEventListener('keydown',e=>{
                const keys=config.axis==='x'?['ArrowLeft','ArrowRight']:['ArrowUp','ArrowDown'];
                if(!keys.includes(e.key))return;e.preventDefault();
                const step=(e.key===keys[0]?-1:1)*config.step*(e.shiftKey?3:1);
                config.write(read()+(config.invert?-step:step));save();
            },{signal});
        }
        makeSplitter(grip,{axis:'y',invert:true,step:20,read:workbenchHeight,write:value=>{view.height=clamp(value,LIMITS.height[0],LIMITS.height[1]);render();},reset:()=>0});
        makeSplitter(libraryGrip,{axis:'x',step:16,read:()=>view.libraryWidth,write:value=>{view.libraryWidth=clamp(value,LIMITS.libraryWidth[0],LIMITS.libraryWidth[1]);render();},reset:()=>220});
        makeSplitter(panelGrip,{axis:'x',invert:true,step:16,read:()=>view.panelWidth,write:value=>{view.panelWidth=clamp(value,LIMITS.panelWidth[0],LIMITS.panelWidth[1]);render();},reset:()=>360});
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
            const observer=new ResizeObserver(()=>{if(wide()!==lastWide)render();else applyLayout();});
            observer.observe(root);
            // The workbench height follows the space the editor host really
            // has. The host grows when the welcome block disappears, and that
            // never changes the size of the app root itself.
            observer.observe(host);
            if(signal&&typeof signal.addEventListener==='function')signal.addEventListener('abort',()=>observer.disconnect(),{once:true});
        }
        return { panel, performance, sound, strudel, welcome, toggleLibrary(){if(root.clientWidth<=540)view.compactLibrary=!view.compactLibrary;else view.library=!view.library;render();save();}, refresh(){welcome.hidden=options.project().tracks.length>0;welcome.querySelectorAll('[data-template]').forEach(b=>b.disabled=options.blocked());} };
    }
    window.SynthStudioWorkspace={create};
})();
