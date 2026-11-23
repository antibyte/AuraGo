(function () {
    'use strict';
    const instances = new Map(), base = '/api/desktop/newspaper';
    const sections = ['regional','national','international','politics','economy','culture','technology','science','environment','health','sport'];
    const languages = ['de','en','cs','da','el','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh'];
    const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const safeURL = value => { try { const u = new URL(value); return ['http:','https:'].includes(u.protocol) && !u.username && !u.password ? u.href : ''; } catch (_) { return ''; } };
    let hyphenOK = false;
    const clone = value => JSON.parse(JSON.stringify(value));
    function render(host, windowId, context) {
        dispose(windowId);
        const ctx = context || {}, controller = new AbortController();
        const tr = key => { const id = 'newspaper.' + key, value = ctx.t?.(id); return value && value !== id ? value : key.replaceAll('_',' '); };
        const st = {host, controller, view:'today', edition:null, editionId:'', followLatest:true, deliveries:[], articleId:'', profile:null, draft:null, rssInput:'', rssSection:'', confirmCode:'', caps:{}, budgetPreview:null, budgetPreviewSignature:'', budgetPreviewVersion:0, budgetPreviewPending:false, budgetPreviewError:false, previewTimer:null, archive:[], run:null, events:[], runSources:[], runSourcesRun:'', search:'', filter:'', frontScroll:0, busy:false, error:'', notice:'', disposed:false, timer:null};
        const channels = ['email','telegram','whatsapp'];
        const channelName = id => id === 'email' ? tr('email') : id === 'telegram' ? 'Telegram' : 'WhatsApp';
        const layout = document.createElement('div'), frame = document.createElement('div'), dock = document.createElement('div');
        layout.className='np-reader-layout'; frame.className='np-reader-body'; dock.className='np-player'; dock.hidden=true;
        const player = document.createElement('audio'), playerTitle = document.createElement('strong');
        player.controls=true; player.preload='none'; dock.append(playerTitle,player); layout.append(frame,dock); host.replaceChildren(layout);
        Object.assign(st,{player,audio:{status:'missing'},casts:[],creation:{},sendFormats:{},castWhen:''});
        instances.set(windowId, st);
        window.AuraDesktopEmail?.attach(windowId, '.np-toolbar > div', () => {
            const edition=st.edition;
            return {key:edition?.id+':'+edition?.revision,signal:controller.signal,disabled:st.disposed||!edition,
                subject:edition?.title,formats:['pdf'],
                createFiles:(_format,signal)=>window.AuraDesktopEmail.files([{url:base+'/editions/'+encodeURIComponent(edition.id)+'/export?format=pdf',name:'newspaper-'+edition.local_date+'-r'+edition.revision+'.pdf'}],signal)};
        });
        st.toolsOpen = null;
        const readOnly = () => ctx.readonly || !st.caps.enabled || st.caps.read_only;
        const date = value => { try { return new Intl.DateTimeFormat(document.documentElement.lang || 'de',{dateStyle:'long'}).format(new Date(value)); } catch (_) { return value; } };
        const time = value => { try { return new Intl.DateTimeFormat(document.documentElement.lang || 'de',{dateStyle:'short',timeStyle:'short'}).format(new Date(value)); } catch (_) { return value; } };
        const request = (path, options = {}) => { const opts = {...options,signal:controller.signal}; if (opts.body) {opts.headers = {...opts.headers,'Content-Type':'application/json'};opts.body=JSON.stringify(opts.body);} return ctx.api(base+path,opts); };
        const challengeError = err => {
            const code=err.body?.code;
            if(code==='agentmail_rejected')return tr('agentmail_rejected').replace('{status}',String(Number(err.body.provider_status)||'?'));
            if(code==='agentmail_bounce_blocked')return tr('agentmail_bounce_blocked');
            if(['email_send_safe','email_send_uncertain','challenge_pending'].includes(code))return tr(code);
            return err.message||tr('error');
        };
        const dateLabel = value => date(value+'T12:00:00');
        const fullDate = value => { try { return new Intl.DateTimeFormat(document.documentElement.lang || 'de',{dateStyle:'full'}).format(new Date(value)); } catch (_) { return value; } };
        const sectionLabel = story => tr('section_'+story.section);
        const languageName = value => { try { return new Intl.DisplayNames([document.documentElement.lang || 'de'], {type:'language'}).of(value) || value; } catch (_) { return value; } };
        const focusAttrs = ['name','data-action','data-view','data-send','data-add','data-remove','data-story','data-edition','id','class'];
        function focusToken(element) {
            if(!(element instanceof HTMLElement)||!host.contains(element))return '';
            for(const attr of focusAttrs){
                const value=element.getAttribute(attr);
                if(!value)continue;
                const peers=[...host.querySelectorAll('['+attr+']')].filter(node=>node.getAttribute(attr)===value);
                return attr+'='+value+'#'+Math.max(0,peers.indexOf(element));
            }
            return '';
        }
        function resolveFocus(token) {
            if(!token)return null;
            const cut=token.lastIndexOf('#'),head=token.slice(0,cut),index=Number(token.slice(cut+1))||0;
            const eq=head.indexOf('='),attr=head.slice(0,eq),value=head.slice(eq+1);
            const peers=[...host.querySelectorAll('['+attr+']')].filter(node=>node.getAttribute(attr)===value);
            return peers[index]||null;
        }
        function selectionOf(element) {
            if(!(element instanceof HTMLInputElement)&&!(element instanceof HTMLTextAreaElement))return null;
            try{const start=element.selectionStart,end=element.selectionEnd;return typeof start==='number'&&typeof end==='number'?[start,end]:null;}catch(_){return null;}
        }
        const sameList = (a,b) => { const left=[...(a||[])].map(value=>String(value)).sort(),right=[...(b||[])].map(value=>String(value)).sort(); return left.length===right.length&&left.every((value,index)=>value===right[index]); };
        const sameFeeds = (a,b) => { const norm=list=>[...(list||[])].map(feed=>[String(feed?.section||''),String(feed?.url||'')]).sort((x,y)=>String(x[0]+x[1]).localeCompare(String(y[0]+y[1]))); return JSON.stringify(norm(a))===JSON.stringify(norm(b)); };
        function preferencesDirty() {
            const draft=st.draft,profile=st.profile;
            if(!draft||!profile)return false;
            for(const key of ['name','language','country','region','city','time_zone','ready_time','length'])if(String(draft[key]??'')!==String(profile[key]??''))return true;
            if(!sameList(draft.sections,profile.sections))return true;
            if(!sameList(draft.interests,profile.interests))return true;
            if(!sameList(draft.exclusions,profile.exclusions))return true;
            if(!sameFeeds(draft.rss_feeds,profile.rss_feeds))return true;
            if(Boolean(draft.daily)!==Boolean(profile.daily))return true;
            if(String(draft.email_account_id||'')!==String(profile.email_account_id||''))return true;
            if(String(draft.email_to||'')!==String(profile.email_to||''))return true;
            if(Boolean(draft.email_daily)!==Boolean(profile.email_daily))return true;
            if(Boolean(draft.telegram_daily)!==Boolean(profile.telegram_daily))return true;
            if(Boolean(draft.whatsapp_daily)!==Boolean(profile.whatsapp_daily))return true;
            if(!!draft.audio_daily!==!!profile.audio_daily || !!draft.cast_daily!==!!profile.cast_daily || (draft.cast_device||'')!==(profile.cast_device||'') || (draft.cast_time||'')!==(profile.cast_time||''))return true;
            return channels.some(k=>(draft.formats?.[k]||'edition')!==(profile.formats?.[k]||'edition'));
        }
        function updateDirtyHint() {
            const node=host.querySelector('.np-dirty');
            if(node)node.hidden=!preferencesDirty();
        }
        const deliveryState = status => String(status||'').replace(/[^a-z_]/gi,'');
        function masthead(e, compact=false, kicker='') {
            const title=e?.title || st.profile?.name || 'Newspaper', place=e?.place || [st.profile?.city,st.profile?.region,st.profile?.country].filter(Boolean).join(', ');
            const day=e?fullDate(e.local_date+'T12:00:00'):fullDate(new Date());
            if(compact)return `<header class="np-masthead np-compact"><div class="np-edition-line"><span>${esc(kicker||place||tr('your_edition'))}</span><b>${esc(title)}</b><span>${esc(day)}</span></div></header>`;
            const ears=e?[`<p class="np-ear np-ear-start"><b>${(e.stories||[]).length} ${esc(tr('stories'))}</b><span>${(e.sources||[]).length} ${esc(tr('sources'))}</span></p>`,`<p class="np-ear np-ear-end"><b>${esc(tr('checked_sources'))}</b><span>${esc(tr('issue'))} ${e.revision}</span></p>`]:['',''];
            return `<header class="np-masthead"><div class="np-nameplate">${ears[0]}<h1>${esc(title)}</h1>${ears[1]}</div><p class="np-motto">${esc(tr('personal_daily'))}</p><div class="np-edition-line"><span>${esc(place||tr('your_edition'))}</span><span>${esc(day)}</span><span>${e?esc(tr('issue'))+' '+e.revision:esc(tr('personal_daily'))}</span></div></header>`;
        }
        function banner() {
            const value=st.error||st.notice||(!st.caps.enabled?tr('disabled'):st.caps.read_only||ctx.readonly?tr('read_only'):!st.caps.research_ready?tr('research_unavailable'):'');
            return value?`<div class="np-banner ${st.error?'np-error':''}" role="${st.error?'alert':'status'}">${esc(value)}${st.error||st.notice?`<button type="button" data-action="dismiss" aria-label="${esc(tr('dismiss'))}">×</button>`:''}</div>`:'';
        }
        function shell(content) {
            const tabs=[['today','today'],['archive','archive'],['preferences','preferences']];
            return `<div class="np-app${hyphenOK?' np-justify':''}"><header class="np-chrome"><div class="np-brand"><i aria-hidden="true"></i><b>Newspaper</b><small>${esc(tr('personal_daily'))}</small></div><nav aria-label="${esc(tr('views'))}">${tabs.map(([view,label])=>`<button type="button" data-view="${view}" aria-current="${st.view===view||view==='today'&&st.view==='article'?'page':'false'}">${esc(tr(label))}</button>`).join('')}</nav></header>${banner()}<main class="np-scroll" tabindex="0">${st.caps.research_ready?'':researchTools()}${content}${st.caps.research_ready?researchTools():''}</main></div>`;
        }
        function researchTools() {
            const tools=(st.caps.research_tools||[]).filter(tool=>['brave_search','ddg_search','rss','web_scraper'].includes(tool.id));
            if(!tools.length)return '';
            const message=reason=>tr('reason_'+(['ready','disabled','key_missing','network_disabled','feeds_missing','scraper_disabled','read_only','model_missing','skill_missing','rate_limited','quota_exhausted','access_denied'].includes(reason)?reason:'failed'));
            return `<details class="np-tools" ${(st.toolsOpen??!st.caps.research_ready)?'open':''}><summary>${esc(tr('research_tools'))}</summary>${st.caps.research_reason?`<p>${esc(message(st.caps.research_reason))}</p>`:''}<ul>${tools.map(tool=>`<li><b>${esc(tr('tool_'+tool.id))}</b><span>${esc(message(tool.reason||tool.state))}${tool.last_error?`<small>${esc(tr('last_attempt').replace('{result}',message(tool.last_error)))}</small>`:''}</span></li>`).join('')}</ul></details>`;
        }
        const runDone = () => !!st.run?.research && !['running','queued','failed','cancelled','interrupted','partial'].includes(st.run.status);
        function progress() {
            const r=st.run;if(!r)return '';const sources=st.runSources.length?`<details class="np-run-sources"><summary>${esc(tr('sources_context'))} · ${st.runSources.length}</summary><ol>${st.runSources.map(source=>`<li>${safeURL(source.url)?`<a href="${esc(safeURL(source.url))}" target="_blank" rel="noopener noreferrer">${esc(source.title||source.publisher)}</a>`:esc(source.title||source.publisher)}</li>`).join('')}</ol></details>`:'';
            const research=r.research,counts=research?tr('research_counts').replace('{candidates}',Number(research.candidates)||0).replace('{pages}',Number(research.read)||0).replace('{articles}',Number(research.accepted)||0):tr('progress_counts').replace('{sources}',r.sources||0).replace('{stories}',r.stories||0);
            const budget=research?.budget;
            const coverage=research?.coverage&&typeof research.coverage==='object'?research.coverage:null;
            const topicCount=Number(budget?.topics)||0;
            const covered=coverage?Object.values(coverage).filter(value=>Number(value)>0).length:0;
            const accepted=Number(research?.accepted)||0;
            const storyLimit=Number(budget?.stories)||0;
            const coverageCounts=budget&&coverage?tr('research_progress').replace('{accepted}',accepted).replace('{stories}',storyLimit).replace('{covered}',covered).replace('{topics}',topicCount):counts;
            const activity=research?tr('research_activity').replace('{overviews}',Number(research.overviews)||0).replace('{searches}',Number(research.searches)||0).replace('{pages}',Number(research.pages)||0).replace('{read}',Number(research.read)||0).replace('{candidates}',Number(research.candidates)||0):'';
            const gaps=research?.gaps?.length?`<p>${esc(tr('research_gaps').replace('{topics}',research.gaps.map(topic=>sections.includes(topic)?tr('section_'+topic):topic).join(', ')))}</p>`:'';
            if(!['running','queued'].includes(r.status))return ['failed','cancelled','interrupted','partial'].includes(r.status)?`<section class="np-progress np-failed" aria-live="polite"><b>${esc(tr(r.status))}</b><span>${esc(r.reason||tr('try_again'))}</span><p>${esc(coverageCounts)}</p>${activity?`<p>${esc(activity)}</p>`:''}${gaps}${sources}</section>`:research?`<section class="np-progress np-progress-done"><p class="np-progress-lead">${esc(coverageCounts)}</p>${activity?`<p class="np-progress-activity">${esc(activity)}</p>`:''}${gaps}</section>`:'';
            return `<section class="np-progress" aria-live="polite"><div><span class="np-pulse" aria-hidden="true"></span><b>${esc(tr('preparing'))}</b><span>${esc(tr(r.phase||'finding'))}</span></div><p>${esc(coverageCounts)}</p>${activity?`<p>${esc(activity)}</p>`:''}<div class="np-progress-actions"><small>${esc(tr('close_safe'))}</small><button type="button" data-action="stop" ${readOnly()?'disabled':''}>${esc(tr('stop'))}</button></div>${st.events.length?`<ol>${st.events.slice(-3).map(ev=>`<li><time>${esc(time(ev.at))}</time> ${esc(ev.text)}</li>`).join('')}</ol>`:''}${sources}</section>`;
        }
        function teaser(story,variant='flow') {
            const count=(story.source_ids||[]).length,sourceLabel=story.single_source?tr('single_source_short'):count+' '+tr('sources');
            const paragraphs=story.paragraphs||[],excerpt=index=>paragraphs[index]?.text?`<p class="np-excerpt">${esc(paragraphs[index].text)}</p>`:'';
            const button=`<button type="button" data-story="${esc(story.id)}">${esc(story.headline)}</button>`;
            if(variant==='brief')return `<article class="np-teaser np-brief"><p class="np-brief-text">${button} ${esc(story.deck)}</p><small class="np-teaser-meta">${esc(sourceLabel)}</small></article>`;
            const eyebrow=variant==='flow'?'':`<span class="np-eyebrow">${esc(sectionLabel(story))}</span>`;
            let leadCount=0;if(variant==='lead'){let total=0;while(leadCount<Math.min(4,paragraphs.length)&&(leadCount===0||total+String(paragraphs[leadCount]?.text||'').length<1100)){total+=String(paragraphs[leadCount]?.text||'').length;leadCount++;}}
            const body=variant==='lead'?`<div class="np-lead-text">${Array.from({length:leadCount},(_,i)=>excerpt(i)).join('')}</div>`:excerpt(0);
            return `<article class="np-teaser np-${variant}">${eyebrow}<h3 class="np-teaser-head">${button}</h3><p class="np-deck">${esc(story.deck)}</p>${body}<small class="np-teaser-meta">${esc(sourceLabel)} · <span class="np-more-link">${esc(tr('continue_reading'))} <span class="np-teaser-go" aria-hidden="true">→</span></span></small></article>`;
        }
        function formatSelect(channel,scope,value,allowOff=false) {
            const options = allowOff ? ['off','edition','audio','both'] : ['edition','audio','both'];
            return `<label class="np-format"><span>${esc(channelName(channel))}</span><select name="${scope}_${channel}" data-format-scope="${scope}" data-channel="${channel}">${options.map(id=>`<option value="${id}" ${value===id?'selected':''}>${esc(tr('format_'+id))}</option>`).join('')}</select></label>`;
        }
        function creationOptions() {
            const p=st.profile || {}, c=st.creation;
            return `<details class="np-audio-options"><summary>${esc(tr('audio_and_delivery'))}</summary><div class="np-audio-fields">
                <label><input type="checkbox" name="create_audio" ${(c.audio??p.audio_daily)?'checked':''}> ${esc(tr('generate_audio'))}</label>
                ${channels.filter(k=>st.caps[k+'_ready']).map(k=>formatSelect(k,'create',c[k]??(p[k+'_daily']?(p.formats?.[k]||'edition'):'off'),true)).join('')}
                </div></details>`;
        }
        function creationBody() {
            const channelsForRun={};
            for(const channel of channels) if(st.creation[channel]!==undefined) channelsForRun[channel]=st.creation[channel];
            return {audio:st.creation.audio,channels:channelsForRun};
        }
        function audioPreferences(p) {
            const devices=st.caps.cast_devices||[], saved=p.cast_device;
            return `<section class="np-pref-section np-audio-prefs"><div class="np-step">04</div><div><h3>${esc(tr('audio_and_delivery'))}</h3><label class="np-check"><input type="checkbox" name="audio_daily" ${p.audio_daily?'checked':''}>${esc(tr('generate_audio'))}</label>
                <div class="np-audio-fields">${channels.filter(k=>st.caps[k+'_allowed']).map(k=>formatSelect(k,'pref',p.formats?.[k]||'edition')).join('')}</div>
                ${st.caps.cast_allowed?`<div class="np-audio-fields"><label>Chromecast<select name="cast_device"><option value="">—</option>${saved&&!devices.some(d=>d.id===saved)?`<option selected value="${esc(saved)}">${esc(tr('cast_unavailable'))}</option>`:''}${devices.map(d=>`<option value="${esc(d.id)}" ${d.id===saved?'selected':''}>${esc(d.name)}</option>`).join('')}</select></label><label>${esc(tr('cast_time'))} · ${esc(p.time_zone)}<input type="time" name="cast_time" value="${esc(p.cast_time||'')}"></label></div><label class="np-check"><input type="checkbox" name="cast_daily" ${p.cast_daily?'checked':''}><span><b>${esc(tr('cast_daily'))}</b><small>${esc(tr('cast_policy'))}</small></span></label>`:''}
                </div></section>`;
        }
        function captureAudioPreferences(data,p) {
            p.audio_daily=data.has('audio_daily');
            p.formats={...p.formats};
            for(const k of channels) if(data.has('pref_'+k))p.formats[k]=String(data.get('pref_'+k));
            if(data.has('cast_device')){
                p.cast_device=String(data.get('cast_device'));p.cast_time=String(data.get('cast_time')||'');p.cast_daily=data.has('cast_daily');
            }
        }
        function sendInProgress(channel) {
            return st.deliveries.some(d=>d.channel===channel && ['preparing','sending'].includes(d.status));
        }
        function audioPanel() {
            if(!('audio_ready' in st.caps))return '';
            const state=st.audio.status, pending=state==='generating', ready=state==='ready';
            const showCast=st.caps.cast_allowed && st.profile?.cast_device;
            let reason='';
            if(!st.caps.audio_ready || st.audio.reason==='tts_unavailable') reason='audio_unavailable';
            else if(state==='failed') reason=['script_failed','script_budget_blocked','script_model_failed','tts_failed'].includes(st.audio.reason)?st.audio.reason:'audio_failed';
            return `<section class="np-broadcast"><b class="np-desk-label">${esc(tr('audio_edition'))}</b>
                ${ready?`<button type="button" class="np-primary" data-action="listen">▶ ${esc(tr('listen'))}</button>`:`<button type="button" data-action="audio" ${readOnly()||pending||!st.caps.audio_ready?'disabled':''}>${esc(tr(pending?'audio_generating':'generate_audio'))}</button>`}
                ${!ready&&reason?`<small role="status">${esc(tr(reason))}</small>`:''}
                ${showCast?`<div class="np-cast"><button type="button" data-action="cast-now" ${readOnly()?'disabled':''}>${esc(tr('cast_now'))}</button><label>${esc(tr('cast_time'))} · ${esc(st.profile.time_zone)}<input type="datetime-local" name="cast_when" value="${esc(st.castWhen)}" ${readOnly()?'disabled':''}></label><button type="button" data-action="cast-later" ${readOnly()?'disabled':''}>${esc(tr('cast_once'))}</button></div>`:''}
                ${st.casts.slice(0,3).map(b=>`<small class="np-cast-status">Chromecast · ${esc(new Intl.DateTimeFormat(document.documentElement.lang||'de',{dateStyle:'short',timeStyle:'short',timeZone:st.profile.time_zone}).format(new Date(b.due)))} · ${esc(tr('cast_'+b.status))}</small>`).join('')}</section>`;
        }
        function updatePlayer(){
            const id=st.audio.status==='ready'?st.edition?.id:'';
            if(player.dataset.edition!==(id||'')){
                player.pause();player.removeAttribute('src');
                if(id)player.src=base+'/editions/'+encodeURIComponent(id)+'/audio/file';
                player.load();player.dataset.edition=id||'';
            }
            dock.hidden=!id;
            if(id){playerTitle.textContent=tr('audio_edition')+' · '+st.edition.local_date;player.setAttribute('aria-label',playerTitle.textContent);}
        }
        function revisionForm() {
            if(st.edition?.local_date!==st.caps.local_date)return '';
            const blocked=readOnly()?tr('reason_read_only'):!st.caps.research_ready?tr('research_unavailable'):st.run?.status==='running'?tr('running'):'';
            if(blocked)return `<span class="np-revision-blocked" title="${esc(blocked)}"><button type="button" disabled aria-label="${esc(tr('new_revision'))}: ${esc(blocked)}">${esc(tr('new_revision'))}</button></span>`;
            return `<details class="np-revision"><summary>${esc(tr('new_revision'))}</summary><div class="np-revision-panel"><p>${esc(tr('revision_confirm'))}</p><label>${esc(tr('correction'))}<input name="correction_note" maxlength="300" placeholder="${esc(tr('correction_help'))}"></label>${creationOptions()}<button type="button" data-action="revision-submit" class="np-primary">${esc(tr('new_revision'))}</button></div></details>`;
        }
        function dailyCastStatus(){
            const b=st.caps.daily_cast;
            return b?`<p class="np-cast-status">Chromecast · ${esc(st.profile.cast_time)} · ${esc(tr('cast_'+b.status))}</p>`:'';
        }
        function shareCard() {
            return `<section class="np-share"><h2>${esc(tr('share_edition'))}</h2><p>${esc(tr('delivery_intro'))}</p><div class="np-actions">${st.caps.email_ready?formatSelect('email','send',st.sendFormats.email||st.profile?.formats?.email||'edition'):''}<button type="button" data-send="email" ${readOnly()||sendInProgress('email')||!st.caps.email_ready?'disabled':''}>${esc(tr('send_email'))}</button>${st.caps.telegram_ready?formatSelect('telegram','send',st.sendFormats.telegram||st.profile?.formats?.telegram||'edition'):''}<button type="button" data-send="telegram" ${readOnly()||sendInProgress('telegram')||!st.caps.telegram_ready?'disabled':''}>${esc(tr('send_telegram'))}</button>${st.caps.whatsapp_ready?formatSelect('whatsapp','send',st.sendFormats.whatsapp||st.profile?.formats?.whatsapp||'edition'):''}<button type="button" data-send="whatsapp" ${readOnly()||sendInProgress('whatsapp')||!st.caps.whatsapp_ready?'disabled':''}>${esc(tr('send_whatsapp'))}</button></div>${!st.caps.email_ready||!st.caps.telegram_ready||!st.caps.whatsapp_ready?`<small>${esc(tr('delivery_setup'))}</small>`:''}${st.deliveries.length?`<div class="np-delivery-history"><b>${esc(tr('delivery_history'))}</b>${st.deliveries.slice(0,4).map(d=>`<span class="np-delivery np-delivery-${esc(deliveryState(d.status))}"><b>${esc(({email:tr('email'),telegram:'Telegram',whatsapp:'WhatsApp'})[d.channel]||d.channel)}</b><span>${esc(tr('format_'+(d.format||'edition')))} · ${esc(tr('delivery_'+d.status))}</span>${d.status==='audio_pending'?`<button type="button" data-send="${esc(d.channel)}" data-audio-only="1" ${readOnly()||sendInProgress(d.channel)?'disabled':''}>${esc(tr('resend_audio'))}</button>`:''}<time>${esc(time(d.updated_at))}</time></span>`).join('')}</div>`:''}</section>`;
        }
        function footerLine(e) {
            return `<div class="np-footer-line"><span>${esc(e.title)} · ${esc(fullDate(e.local_date+'T12:00:00'))} · ${esc(tr('issue'))} ${e.revision}</span><span>${esc(tr('checked_sources'))}</span></div>`;
        }
        function moreNews(stories) {
            const groups=new Map();
            for(const story of stories){if(!groups.has(story.section))groups.set(story.section,[]);groups.get(story.section).push(story);}
            return `<section class="np-more" aria-label="${esc(tr('more_news'))}"><h2 class="np-band"><span>${esc(tr('more_news'))}</span></h2><div class="np-more-flow">${[...groups.values()].map(list=>`<div class="np-ressort"><h3 class="np-ressort-head">${esc(sectionLabel(list[0]))}</h3>${list.map(x=>teaser(x,'flow')).join('')}</div>`).join('')}</div></section>`;
        }
        function today() {
            const e=st.edition,done=runDone();
            if(!e)return `${progress()}${dailyCastStatus()}<div class="np-paper" lang="${esc(st.profile?.language||'')}">${masthead(null)}<section class="np-empty"><span class="np-eyebrow">${esc(tr('first_edition'))}</span><h2>${esc(tr('empty_title'))}</h2><p>${esc(tr('empty_text'))}</p><div class="np-actions"><button type="button" class="np-primary" data-action="create" ${readOnly()||!st.caps.research_ready||st.run?.status==='running'?'disabled':''}>${esc(tr('create_today'))}</button><button type="button" data-view="preferences">${esc(tr('set_preferences'))}</button></div>${creationOptions()}</section></div>`;
            const stories=e.stories||[],newDay=st.followLatest&&st.caps.local_date&&!st.archive.some(item=>item.local_date===st.caps.local_date);
            const desk=`<div class="np-desk"><div class="np-toolbar"><span class="np-toolbar-meta"><span>${esc(tr('published'))} <time>${esc(time(e.created_at))}</time></span><i aria-hidden="true">·</i><span>${stories.length} ${esc(tr('stories'))}</span><i aria-hidden="true">·</i><span>${e.sources.length} ${esc(tr('sources'))}</span></span><div>${revisionForm()}<button type="button" data-action="pdf">${esc(tr('download_pdf'))}</button></div></div>${audioPanel()}</div>`;
            const newDayCard=newDay?`<section class="np-new-day"><strong>${esc(tr('today'))} · ${esc(dateLabel(st.caps.local_date))}</strong><button type="button" class="np-primary" data-action="create" ${readOnly()||!st.caps.research_ready||st.run?.status==='running'?'disabled':''}>${esc(tr('create_today'))}</button>${creationOptions()}${dailyCastStatus()}</section>`:'';
            const colophon=done?`<div class="np-colophon"><b>${esc(tr('about_edition'))}</b>${progress()}</div>`:'';
            return `${desk}${newDayCard}${done?'':progress()}<div class="np-paper" lang="${esc(e.language||'')}">${masthead(e)}${e.partial?`<p class="np-partial">${esc(tr('partial'))}</p>`:''}${(e.corrections||[]).map(note=>`<p class="np-correction"><b>${esc(tr('correction'))}:</b> ${esc(note)}</p>`).join('')}<div class="np-front"><section class="np-front-lead">${stories[0]?teaser(stories[0],'lead'):''}</section><section class="np-front-side">${stories.slice(1,3).map(x=>teaser(x,'side')).join('')}</section><aside class="np-front-brief"><h2>${esc(tr('in_brief'))}</h2>${stories.slice(3,6).map(x=>teaser(x,'brief')).join('')}</aside></div>${stories.length>6?moreNews(stories.slice(6)):''}<footer class="np-footer">${footerLine(e)}${colophon}</footer></div>${shareCard()}`;
        }
        function article() {
            const e=st.edition, story=e?.stories?.find(x=>x.id===st.articleId);if(!story)return today();
            const sources=new Map(e.sources.map(s=>[s.id,s]));
            const cited=(story.source_ids||[]).map(id=>sources.get(id)).filter(Boolean);
            const numbers=new Map(cited.map((source,index)=>[source.id,index+1]));
            const marker=id=>{const source=sources.get(id),index=numbers.get(id);if(!source||!index)return '';return `<a class="np-ref" href="#np-sources" data-source="${esc(id)}" title="${esc(source.publisher||'')}" aria-label="${esc(source.publisher||tr('source'))}">[${index}]</a>`;};
            const position=e.stories.indexOf(story),prev=e.stories[position-1],next=e.stories[position+1];
            const publishers=[...new Set(cited.map(s=>s.publisher).filter(Boolean))].join(', ');
            const nav=(target,label)=>target?`<button type="button" data-story="${esc(target.id)}" aria-label="${esc(tr(label))}">${label==='previous_article'?'‹ '+esc(tr(label)):esc(tr(label))+' ›'}</button>`:`<button type="button" disabled aria-label="${esc(tr(label))}">${label==='previous_article'?'‹ '+esc(tr(label)):esc(tr(label))+' ›'}</button>`;
            return `<div class="np-desk np-article-desk"><button type="button" class="np-back" data-view="today">← ${esc(tr('back_front'))}</button><span class="np-article-position">${esc(tr('article_position').replace('{current}',position+1).replace('{total}',e.stories.length))}</span><nav class="np-article-nav" aria-label="${esc(tr('views'))}">${nav(prev,'previous_article')}${nav(next,'next_article')}</nav></div><div class="np-paper np-article-paper" lang="${esc(e.language||'')}">${masthead(e,true,sectionLabel(story))}<article class="np-article" dir="auto"><p class="np-article-overline"><span>${esc(sectionLabel(story))}</span></p><h2 tabindex="-1">${esc(story.headline)}</h2><p class="np-article-deck">${esc(story.deck)}</p>${publishers?`<p class="np-byline">${esc(tr('based_on').replace('{publishers}',publishers))}</p>`:''}${story.single_source?`<p class="np-disclosure">${esc(tr('single_source'))}</p>`:''}<div class="np-copy">${(story.paragraphs||[]).map(p=>`<p>${esc(p.text)} ${(p.source_ids||[]).map(marker).join(' ')}</p>`).join('')}</div><aside class="np-source-panel" id="np-sources"><h3>${esc(tr('sources_context'))}</h3><p>${esc(tr('source_note'))}</p>${cited.map(s=>`<details class="np-source" data-source-id="${esc(s.id)}"><summary><span class="np-source-num">${numbers.get(s.id)}</span><b>${esc(s.publisher)}</b><span>${esc(s.title)}</span></summary><p>${s.published_at?esc(tr('source_published'))+' '+esc(time(s.published_at)):esc(tr('date_unknown'))} · ${esc(tr('retrieved'))} ${esc(time(s.retrieved_at))}</p><blockquote>${esc(s.excerpt)}</blockquote>${safeURL(s.url)?`<a href="${esc(safeURL(s.url))}" target="_blank" rel="noopener noreferrer">${esc(tr('open_original'))} ↗</a>`:''}</details>`).join('')}</aside></article><footer class="np-footer">${footerLine(e)}</footer></div>`;
        }
        function archive() {
            const term=st.search.toLocaleLowerCase(),items=st.archive.filter(item=>(!st.filter||(item.sections||[]).includes(st.filter))&&(!term||(item.headlines||[item.lead]).some(s=>String(s).toLocaleLowerCase().includes(term))));
            return `<div class="np-paper np-archive">${masthead(st.edition,true,tr('archive'))}<div class="np-section-heading"><span class="np-eyebrow">${esc(tr('all_editions'))}</span><h2>${esc(tr('archive_title'))}</h2><p>${esc(tr('archive_intro'))}</p></div><div class="np-archive-tools"><label>${esc(tr('search_headlines'))}<input type="search" name="archive_search" value="${esc(st.search)}" placeholder="${esc(tr('search_placeholder'))}"></label><label>${esc(tr('section'))}<select name="archive_filter"><option value="">${esc(tr('all_sections'))}</option>${sections.concat('interests').map(x=>`<option value="${x}" ${st.filter===x?'selected':''}>${esc(tr('section_'+x))}</option>`).join('')}</select></label></div><p class="np-archive-count">${esc(tr('archive_count').replace('{count}',String(items.length)))}</p><div class="np-archive-list">${items.map(item=>`<button type="button" data-edition="${esc(item.id)}"><span class="np-archive-date">${esc(fullDate(item.local_date+'T12:00:00'))}<small>${esc(tr('issue'))} ${item.revision}${item.partial?' · '+esc(tr('partial_short')):''}</small></span><strong>${esc(item.lead)}</strong><span>${item.stories} ${esc(tr('stories'))} →</span></button>`).join('')||`<p class="np-archive-empty">${esc(tr('no_archive_results'))}</p>`}</div></div>`;
        }
        function chips(kind,values) {
            return `<div class="np-chips">${values.map((value,i)=>`<span>${esc(value)}<button type="button" data-remove="${kind}" data-index="${i}" aria-label="${esc(tr('remove'))}: ${esc(value)}" ${readOnly()?'disabled':''}>×</button></span>`).join('')}</div><div class="np-add"><input name="${kind}_input" maxlength="120" placeholder="${esc(tr(kind==='interests'?'interest_placeholder':'exclusion_placeholder'))}" ${readOnly()?'disabled':''}><button type="button" data-add="${kind}" ${readOnly()?'disabled':''}>${esc(tr('add'))}</button></div>`;
        }
        function rssFields(p) {
            const feeds=p.rss_feeds||[],choices=(p.sections||[]).concat((p.interests||[]).length?['interests']:[]);
            return `<div class="np-rss"><h4>${esc(tr('rss_feeds'))} <small>${feeds.length}/10</small></h4><p>${esc(tr('rss_help'))}</p><div class="np-chips">${feeds.map((feed,i)=>`<span><b>${esc(tr('section_'+feed.section))}</b> · ${esc(feed.url)}<button type="button" data-remove="rss_feeds" data-index="${i}" aria-label="${esc(tr('remove'))}: ${esc(feed.url)}" ${readOnly()?'disabled':''}>×</button></span>`).join('')}</div><div class="np-add"><input type="text" inputmode="url" name="rss_url" maxlength="2048" value="${esc(st.rssInput)}" placeholder="https://example.org/feed.xml" aria-label="${esc(tr('rss_feeds'))}" ${readOnly()?'disabled':''}><select name="rss_section" aria-label="${esc(tr('section'))}">${choices.map(section=>`<option value="${esc(section)}" ${st.rssSection===section?'selected':''}>${esc(tr('section_'+section))}</option>`).join('')}</select><button type="button" data-action="add-rss" ${readOnly()||!choices.length||feeds.length>=10?'disabled':''}>${esc(tr('add'))}</button></div><p class="np-rss-error" role="alert"></p></div>`;
        }
        function budgetPanel() {
            const response=st.budgetPreview||{effective_budget:st.caps.effective_budget,monetary_budget_enabled:st.caps.monetary_budget_enabled};
            const budget=response?.effective_budget;
            const mode=(budget?.mode||st.caps.budget_mode)==='auto'?'auto':'fixed';
            const values=budget&&typeof budget==='object'?`<p>${esc(tr('effective_budget_summary').replace('{topics}',Number(budget.topics)||0).replace('{pages}',Number(budget.pages)||0).replace('{searches}',Number(budget.searches)||0).replace('{overviews}',Number(budget.overviews)||0).replace('{minutes}',Number(budget.minutes)||0).replace('{candidates}',Number(budget.candidates)||0).replace('{stories}',Number(budget.stories)||0).replace('{editor_calls}',Number(budget.editor_calls)||0))}</p>`:`<p class="np-state">${esc(st.budgetPreviewError?tr('budget_preview_failed'):tr('budget_preview_loading'))}</p>`;
            const money=response?.monetary_budget_enabled===true?tr('monetary_budget_enabled'):tr('monetary_budget_disabled');
            const shared=mode==='fixed'&&budget?.shared_pages===true?`<p>${esc(tr('fixed_budget_note').replace('{pages}',Number(budget.pages)||0))}</p>`:'';
            return `<section class="np-pref-section np-budget-panel"><div class="np-step">05</div><div><h3>${esc(tr('budget_title'))}</h3><p>${esc(tr('budget_intro'))}</p><p class="np-state">${esc(tr('budget_mode'))}: ${esc(tr('budget_'+mode))}</p>${values}${st.budgetPreviewError?`<p class="np-state" role="status">${esc(tr('budget_preview_failed'))}</p>`:''}${st.budgetPreviewPending?`<p role="status">${esc(tr('budget_preview_loading'))}</p>`:''}<p>${esc(money)}</p>${shared}</div></section>`;
        }
        function queueBudgetPreview(profile) {
            if(!profile||!st.caps)return;
            const mode=st.caps.budget_mode||st.caps.effective_budget?.mode||'fixed';
            const signature=JSON.stringify({mode,sections:profile.sections||[],interests:profile.interests||[],length:profile.length||''});
            if(signature===st.budgetPreviewSignature)return;
            st.budgetPreviewSignature=signature;
            const version=++st.budgetPreviewVersion;
            clearTimeout(st.previewTimer);
            st.budgetPreviewPending=true;
            st.budgetPreviewError=false;
            if(st.view==='preferences')updateBudgetPanel();
            st.previewTimer=setTimeout(async()=>{
                try {
                    const result=await request('/budget-preview',{method:'POST',body:{profile:{sections:profile.sections||[],interests:profile.interests||[],length:profile.length||''},budget_mode:mode}});
                    if(st.disposed||version!==st.budgetPreviewVersion)return;
                    st.budgetPreview=result;
                    st.budgetPreviewPending=false;
                    if(st.view==='preferences')updateBudgetPanel();
                } catch(error) {
                    if(st.disposed||version!==st.budgetPreviewVersion||error.name==='AbortError')return;
                    st.budgetPreviewPending=false;
                    st.budgetPreviewError=true;
                    st.budgetPreviewSignature='';
                    if(st.view==='preferences')updateBudgetPanel();
                }
            },140);
        }
        function capturePreferences() {
            const form=host.querySelector('.np-prefs');if(!form||!st.draft||readOnly())return;
            const data=new FormData(form);
            st.draft.sections=data.getAll('sections');
            for(const key of ['name','language','country','region','city','time_zone','ready_time','length'])st.draft[key]=String(data.get(key)||'');
            st.draft.daily=data.has('daily');
            captureAudioPreferences(data,st.draft);
            if(st.caps.email_allowed){st.draft.email_account_id=String(data.get('email_account_id')||'');st.draft.email_to=String(data.get('email_to')||'');st.draft.email_daily=data.has('email_daily');}
            if(st.caps.telegram_ready)st.draft.telegram_daily=data.has('telegram_daily');
            if(st.caps.whatsapp_ready)st.draft.whatsapp_daily=data.has('whatsapp_daily');
            st.rssInput=String(data.get('rss_url')||'');st.rssSection=String(data.get('rss_section')||'');
        }
        async function saveEmailDestinationForChallenge() {
            capturePreferences();
            const account=String(st.draft?.email_account_id||'');
            const address=String(st.draft?.email_to||'').trim();
            const input=host.querySelector('.np-prefs [name=email_to]');
            if(!account||!st.caps.email_accounts?.some(item=>item.id===account)||!address||!input?.checkValidity())throw new Error(tr('email_setup_required'));
            if(account===st.profile.email_account_id&&address===st.profile.email_to)return;
            const next=clone(st.profile);
            next.email_account_id=account;next.email_to=address;next.email_daily=false;
            const saved=await request('/profile',{method:'PUT',headers:{'If-Match':'"'+st.profile.version+'"'},body:next});
            st.profile=saved;
            for(const key of ['email_account_id','email_to','email_verified','email_daily'])st.draft[key]=saved[key];
        }
        function syncChallengeAction() {
            const form=host.querySelector('.np-prefs'),button=host.querySelector('[data-action=email-challenge]');
            if(!form||!button)return;
            const data=new FormData(form);
            button.disabled=readOnly()||!st.caps.email_allowed||!data.get('email_account_id')||!String(data.get('email_to')||'').trim();
        }
        function preferences() {
            const p=st.draft||st.profile;if(!p)return `<div class="np-paper">${esc(tr('loading'))}</div>`;
            return `<div class="np-paper np-preferences">${masthead(st.edition,true,tr('preferences'))}<div class="np-section-heading"><span class="np-eyebrow">${esc(tr('your_newspaper'))}</span><h2>${esc(tr('preferences_title'))}</h2><p>${esc(tr('preferences_intro'))}</p></div><form class="np-prefs"><fieldset ${readOnly()?'disabled':''}><section class="np-pref-section"><div class="np-step">01</div><div><h3>${esc(tr('interests_title'))}</h3><p>${esc(tr('interests_intro'))}</p><div class="np-section-grid">${sections.map(x=>`<label><input type="checkbox" name="sections" value="${x}" ${(p.sections||[]).includes(x)?'checked':''}><span>${esc(tr('section_'+x))}</span></label>`).join('')}</div><label class="np-field"><span>${esc(tr('free_interests'))}</span>${chips('interests',p.interests||[])}</label><label class="np-field"><span>${esc(tr('exclusions'))}</span>${chips('exclusions',p.exclusions||[])}</label>${rssFields(p)}</div></section><section class="np-pref-section"><div class="np-step">02</div><div><h3>${esc(tr('edition_title'))}</h3><p>${esc(tr('edition_intro'))}</p><div class="np-form-grid"><label>${esc(tr('publication_name'))}<input name="name" maxlength="70" value="${esc(p.name)}" required></label><label>${esc(tr('language'))}<select name="language">${languages.map(x=>`<option value="${x}" ${p.language===x?'selected':''}>${esc(languageName(x))}</option>`).join('')}</select></label><label>${esc(tr('country'))}<input name="country" maxlength="100" value="${esc(p.country)}" placeholder="DE"></label><label>${esc(tr('region'))}<input name="region" maxlength="100" value="${esc(p.region)}"></label><label>${esc(tr('city'))}<input name="city" maxlength="100" value="${esc(p.city)}"></label><label>${esc(tr('time_zone'))}<input name="time_zone" maxlength="80" value="${esc(p.time_zone)}" required></label><label>${esc(tr('ready_time'))}<input type="time" name="ready_time" value="${esc(p.ready_time)}" required></label><label>${esc(tr('reading_length'))}<select name="length">${['brief','standard','in_depth'].map(x=>`<option value="${x}" ${p.length===x?'selected':''}>${esc(tr('length_'+x))}</option>`).join('')}</select></label></div><label class="np-check"><input type="checkbox" name="daily" ${p.daily?'checked':''}><span><b>${esc(tr('daily'))}</b><small>${esc(tr('daily_help'))}</small></span></label></div></section><section class="np-pref-section"><div class="np-step">03</div><div><h3>${esc(tr('delivery_title'))}</h3><p>${esc(tr('delivery_intro'))}</p><div class="np-channels"><div><h4>${esc(tr('email'))}</h4><p>${esc(st.caps.email_allowed?tr('email_help'):tr('email_disabled'))}</p><label>${esc(tr('sending_account'))}<select name="email_account_id" ${!st.caps.email_allowed?'disabled':''}><option value="">${esc(tr('choose_account'))}</option>${(st.caps.email_accounts||[]).map(a=>`<option value="${esc(a.id)}" ${p.email_account_id===a.id?'selected':''}>${esc(a.name||a.id)}</option>`).join('')}</select></label><label>${esc(tr('email_address'))}<input type="email" name="email_to" value="${esc(p.email_to)}" maxlength="254" ${!st.caps.email_allowed?'disabled':''}></label><p class="np-state">${esc(p.email_verified?tr('email_verified'):tr('email_unverified'))}</p><label class="np-check"><input type="checkbox" name="email_daily" ${p.email_daily?'checked':''} ${!st.caps.email_allowed||!p.email_verified?'disabled':''}><span>${esc(tr('email_daily'))}</span></label></div><div><h4>Telegram</h4><p>${esc(st.caps.telegram_ready?tr('telegram_ready'):tr('telegram_unavailable'))}</p><label class="np-check"><input type="checkbox" name="telegram_daily" ${p.telegram_daily?'checked':''} ${!st.caps.telegram_ready?'disabled':''}><span>${esc(tr('telegram_daily'))}</span></label></div><div><h4>WhatsApp</h4><p>${esc(st.caps.whatsapp_ready?tr('whatsapp_ready'):tr('whatsapp_unavailable'))}</p><label class="np-check"><input type="checkbox" name="whatsapp_daily" ${p.whatsapp_daily?'checked':''} ${!st.caps.whatsapp_ready?'disabled':''}><span>${esc(tr('whatsapp_daily'))}</span></label></div></div></div></section><div class="np-save"><button type="submit" class="np-primary">${esc(tr('save'))}</button><span>${esc(tr('save_help'))}</span><span class="np-dirty" role="status" ${preferencesDirty()?'':'hidden'}>${esc(tr('unsaved_changes'))}</span></div></fieldset></form><section class="np-verify"><h3>${esc(tr('verify_delivery'))}</h3><p>${esc(tr('verify_help'))}</p><div class="np-verify-row"><label class="np-verify-code"><span>${esc(tr('confirmation_code'))}</span><input name="confirmation_code" maxlength="10" autocomplete="one-time-code" value="${esc(st.confirmCode)}"></label><div class="np-actions"><button type="button" data-action="email-challenge" ${readOnly()||!st.caps.email_allowed||!p.email_to||!p.email_account_id?'disabled':''}>${esc(tr('send_code'))}</button><button type="button" data-action="email-confirm" ${readOnly()||!st.caps.email_allowed?'disabled':''}>${esc(tr('confirm'))}</button><button type="button" data-action="telegram-test" ${readOnly()||!st.caps.telegram_ready?'disabled':''}>${esc(tr('send_telegram_test'))}</button><button type="button" data-action="whatsapp-test" ${readOnly()||!st.caps.whatsapp_ready?'disabled':''}>${esc(tr('send_whatsapp_test'))}</button></div></div></section></div>`;
        }
        function probeHyphens(lang) {
            if(hyphenOK)return;
            const probe=document.createElement('div');probe.lang=lang||'de';probe.textContent='Bundestagsabgeordnete';
            probe.style.cssText='position:absolute;left:0;top:0;width:60px;font:16px serif;hyphens:auto;-webkit-hyphens:auto;visibility:hidden';
            host.appendChild(probe);hyphenOK=probe.scrollWidth<=62;probe.remove();
        }
        function draw() {
            if(st.disposed)return;
            probeHyphens(st.edition?.language);
            const preferencesScroll=st.view==='preferences'&&frame.querySelector('.np-preferences')?frame.querySelector('.np-scroll').scrollTop:null;
            const active=document.activeElement;
            const token=focusToken(active),selection=selectionOf(active);
            frame.innerHTML=shell(st.view==='article'?article():st.view==='archive'?archive():st.view==='preferences'?preferences():today());
            if(st.view==='preferences')st.host.querySelector('.np-prefs .np-pref-section:last-of-type')?.insertAdjacentHTML('afterend',audioPreferences(st.draft||st.profile)+budgetPanel());
            updatePlayer();
            if(preferencesScroll!==null)frame.querySelector('.np-scroll').scrollTop=preferencesScroll;
            if(st.view==='today')st.host.querySelector('.np-scroll').scrollTop=st.frontScroll;
            if(st.view==='article'&&st.edition)st.host.querySelector('.np-scroll').scrollTop=Number(localStorage.getItem('aurago.newspaper.read.'+st.edition.id+'.'+st.articleId)||0);
            const restored=resolveFocus(token);
            if(restored){restored.focus({preventScroll:true});if(selection){try{restored.setSelectionRange(selection[0],selection[1]);}catch(_){}}}
        }
        function updateBudgetPanel() {
            const panel=st.host.querySelector('.np-budget-panel');if(!panel)return;
            const wrapper=document.createElement('div');wrapper.innerHTML=budgetPanel();
            panel.replaceWith(wrapper.firstElementChild);
        }
        async function refresh(redraw=true) {
            const [caps,profile,list]=await Promise.all([request('/capabilities'),request('/profile'),request('/editions')]);if(st.disposed)return;
            st.caps=caps;st.profile=profile;st.archive=list.editions||[];st.run=list.latest_run||null;if(!st.draft||st.view!=='preferences')st.draft=clone(profile);queueBudgetPreview(st.draft||profile);
            const wanted=st.followLatest?st.archive[0]?.id:st.editionId;if(wanted&&st.edition?.id!==wanted){st.edition=await request('/editions/'+encodeURIComponent(wanted));st.editionId=wanted;}
            if(wanted && 'audio_ready' in caps){
                const info=await request('/editions/'+encodeURIComponent(wanted)+'/audio');
                if(st.edition?.id===wanted){st.audio=info.audio||{status:'missing'};st.casts=info.casts||[];}
            }
            if(wanted)st.deliveries=(await request('/editions/'+encodeURIComponent(wanted)+'/deliveries')).deliveries||[];
            if(st.run?.status==='running'||(['failed','cancelled','interrupted'].includes(st.run?.status)&&st.runSourcesRun!==st.run.id)){const data=await request('/editions/'+encodeURIComponent(st.run.id)+'/events?after='+(st.events.at(-1)?.id||0));st.events=[...st.events,...(data.events||[])].slice(-30);st.runSources=data.sources||[];st.runSourcesRun=st.run.id;}else if(st.run?.status==='published'){st.events=[];st.runSources=[];st.runSourcesRun='';}
            if(!st.disposed&&redraw)draw();
        }
        async function poll() {
            if(st.disposed)return;
            if(st.busy){st.timer=setTimeout(poll,5000);return;}
            try{const audioBefore=JSON.stringify([st.audio,st.casts,st.deliveries]),old=st.run?.updated_at,latest=st.archive[0]?.id,day=st.caps.local_date,castStatus=st.caps.daily_cast?.status;await refresh(false);if(st.view==='today'&&(old!==st.run?.updated_at||latest!==st.archive[0]?.id||day!==st.caps.local_date||castStatus!==st.caps.daily_cast?.status||audioBefore!==JSON.stringify([st.audio,st.casts,st.deliveries]))){st.frontScroll=host.querySelector('.np-scroll')?.scrollTop||0;draw();}}catch(err){if(err.name!=='AbortError'){st.error=err.message||tr('error');draw();}}
            if(!st.disposed)st.timer=setTimeout(poll,5000);
        }
        async function act(fn){if(st.busy)return;st.busy=true;frame.setAttribute('aria-busy','true');st.error='';st.notice='';try{await fn();await refresh(false);draw();}catch(err){if(err.name!=='AbortError'){try{await refresh(false);}catch(_){}st.error=err.message||tr('error');draw();}}finally{st.busy=false;frame.removeAttribute('aria-busy');}}
        host.addEventListener('toggle',ev=>{if(ev.target.matches('.np-tools'))st.toolsOpen=ev.target.open;},{capture:true,signal:controller.signal});
        host.addEventListener('scroll',ev=>{if(!ev.target.matches('.np-scroll'))return;if(st.view==='today')st.frontScroll=ev.target.scrollTop;if(st.view==='article'&&st.edition)localStorage.setItem('aurago.newspaper.read.'+st.edition.id+'.'+st.articleId,String(ev.target.scrollTop));},{capture:true,signal:controller.signal});
        host.addEventListener('input',ev=>{if(ev.target.name==='archive_search'){st.search=ev.target.value;draw();return;}if(ev.target.name==='rss_url'){host.querySelector('.np-rss-error').textContent='';ev.target.removeAttribute('aria-invalid');}else if(ev.target.name==='email_to')syncChallengeAction();if(ev.target.name==='confirmation_code')st.confirmCode=ev.target.value;if(ev.target.closest('.np-prefs')){capturePreferences();updateDirtyHint();}},{signal:controller.signal});
        host.addEventListener('change',ev=>{if(ev.target.name==='archive_filter'){st.filter=ev.target.value;draw();}else if(ev.target.name==='email_account_id')syncChallengeAction();else if(ev.target.name==='sections'){capturePreferences();const select=host.querySelector('[name=rss_section]');if(select){const chosen=select.value,choices=st.draft.sections.concat((st.draft.interests||[]).length?['interests']:[]);select.replaceChildren(...choices.map(section=>{const option=document.createElement('option');option.value=section;option.textContent=tr('section_'+section);return option;}));if(choices.includes(chosen))select.value=chosen;st.rssSection=select.value;host.querySelector('[data-action=add-rss]').disabled=readOnly()||!choices.length;}}},{signal:controller.signal});
        host.addEventListener('change',ev=>{if(ev.target.name==='sections'||ev.target.name==='length'){capturePreferences();queueBudgetPreview(st.draft);}},{signal:controller.signal});
        host.addEventListener('change',ev=>{if(ev.target.closest('.np-prefs')){capturePreferences();updateDirtyHint();}},{signal:controller.signal});
        host.addEventListener('input',ev=>{
            if(ev.target.name==='cast_when')st.castWhen=ev.target.value;
            if(ev.target.name==='create_audio')st.creation.audio=ev.target.checked;
            const {formatScope,channel}=ev.target.dataset;
            if(formatScope==='send')st.sendFormats[channel]=ev.target.value;
            if(formatScope==='create')st.creation[channel]=ev.target.value;
        },{signal:controller.signal});
        function backToFront(){const id=st.articleId;st.view='today';st.error='';draw();(host.querySelector('.np-front [data-story="'+CSS.escape(id)+'"], .np-more [data-story="'+CSS.escape(id)+'"]')||host.querySelector('.np-scroll'))?.focus({preventScroll:true});}
        function stepArticle(delta){const list=st.edition?.stories||[],target=list[list.findIndex(x=>x.id===st.articleId)+delta];if(!target)return;st.articleId=target.id;draw();host.querySelector('.np-article h2')?.focus({preventScroll:true});}
        host.addEventListener('keydown',ev=>{if(st.view!=='article')return;if(ev.key==='Escape'){ev.preventDefault();backToFront();return;}if(ev.altKey&&(ev.key==='ArrowLeft'||ev.key==='ArrowRight')&&!ev.target.closest?.('input,select,textarea')){ev.preventDefault();stepArticle(ev.key==='ArrowLeft'?-1:1);}},{signal:controller.signal});
        host.addEventListener('submit',ev=>{if(!ev.target.matches('.np-prefs'))return;ev.preventDefault();if(readOnly())return;const data=new FormData(ev.target),next=clone(st.profile);next.sections=data.getAll('sections');next.interests=st.draft.interests;next.exclusions=st.draft.exclusions;next.rss_feeds=st.draft.rss_feeds||[];for(const key of ['name','language','country','region','city','time_zone','ready_time','length'])next[key]=String(data.get(key)||'');if(st.caps.email_allowed){next.email_account_id=String(data.get('email_account_id')||'');next.email_to=String(data.get('email_to')||'');next.email_daily=data.has('email_daily');}next.daily=data.has('daily');captureAudioPreferences(data,next);if(st.caps.telegram_ready)next.telegram_daily=data.has('telegram_daily');if(st.caps.whatsapp_ready)next.whatsapp_daily=data.has('whatsapp_daily');act(async()=>{const saved=await request('/profile',{method:'PUT',headers:{'If-Match':'"'+st.profile.version+'"'},body:next});st.profile=saved;st.draft=clone(saved);st.notice=tr('saved');});},{signal:controller.signal});
        host.addEventListener('click',ev=>{
            const el=ev.target.closest('button,a[data-source]')||ev.target.closest('.np-teaser')?.querySelector('[data-story]');if(!el||el.disabled)return;
            if(el.dataset.view){if(el.dataset.view==='today'&&st.view==='article'&&!el.closest('.np-chrome')){backToFront();return;}if(st.view==='today')st.frontScroll=host.querySelector('.np-scroll')?.scrollTop||0;if(el.dataset.view==='today'&&el.closest('.np-chrome')){st.followLatest=true;st.editionId='';st.frontScroll=0;st.view='today';act(async()=>{});return;}st.view=el.dataset.view;st.error='';draw();return;}
            if(el.dataset.story){if(st.view==='today')st.frontScroll=host.querySelector('.np-scroll')?.scrollTop||0;st.articleId=el.dataset.story;st.view='article';draw();(host.querySelector('.np-article h2')||host.querySelector('.np-scroll'))?.focus({preventScroll:true});return;}
            if(el.dataset.source){ev.preventDefault();const node=[...host.querySelectorAll('.np-source')].find(x=>x.dataset.sourceId===el.dataset.source);if(node){node.open=true;node.scrollIntoView({block:'nearest'});node.querySelector('summary')?.focus({preventScroll:true});}return;}
            if(el.dataset.edition){act(async()=>{st.editionId=el.dataset.edition;st.followLatest=false;st.edition=await request('/editions/'+encodeURIComponent(st.editionId));st.deliveries=(await request('/editions/'+encodeURIComponent(st.editionId)+'/deliveries')).deliveries||[];st.view='today';st.frontScroll=0;});return;}
            if(el.dataset.remove){const kind=String(el.dataset.remove||'').replace(/[^\w-]/g,''),index=Number(el.dataset.index);capturePreferences();(st.draft[kind]||[]).splice(index,1);queueBudgetPreview(st.draft);draw();const remaining=host.querySelectorAll('[data-remove="'+kind+'"]');remaining[Math.min(index,remaining.length-1)]?.focus({preventScroll:true});return;}
            if(el.dataset.add){const kind=el.dataset.add,input=host.querySelector('[name='+kind+'_input]'),value=input?.value.trim(),list=st.draft[kind]||(st.draft[kind]=[]);if(value&&list.length<20&&!list.some(x=>x.toLowerCase()===value.toLowerCase())){capturePreferences();list.push(value);queueBudgetPreview(st.draft);draw();host.querySelector('[name='+kind+'_input]')?.focus();}return;}
            if(el.dataset.send){const format=el.dataset.audioOnly?'audio':st.sendFormats[el.dataset.send]||st.profile?.formats?.[el.dataset.send]||'edition';act(async()=>{const last=st.deliveries.find(d=>d.channel===el.dataset.send);if(['uncertain','sending'].includes(last?.status)&&ctx.confirmDialog&&!(await ctx.confirmDialog(tr('delivery_uncertain'),tr('resend_confirm'))))return;const key=crypto.randomUUID();const result=await request('/editions/'+encodeURIComponent(st.edition.id)+'/deliver',{method:'POST',headers:{'Idempotency-Key':key},body:{channel:el.dataset.send,kind:'manual',format}});st.notice=tr('delivery_'+result.status);st.deliveries=(await request('/editions/'+encodeURIComponent(st.edition.id)+'/deliveries')).deliveries||[];});return;}
            const action=el.dataset.action;
            if(action==='listen'){player.play().catch(()=>{st.error=tr('audio_failed');draw();});return;}
            if(action==='audio'){act(async()=>{st.audio=await request('/editions/'+encodeURIComponent(st.edition.id)+'/audio',{method:'POST',body:{}});});return;}
            if(action==='cast-now'||action==='cast-later'){
                const input=host.querySelector('[name=cast_when]');
                if(action==='cast-later'&&!input?.value){if(input){input.required=true;input.reportValidity();}return;}
                const [date,time]=(action==='cast-later'?input.value:'').split('T');
                act(async()=>{if(st.casts[0]?.status==='uncertain'&&ctx.confirmDialog&&!(await ctx.confirmDialog(tr('cast_uncertain'),tr('resend_confirm'))))return;await request('/editions/'+encodeURIComponent(st.edition.id)+'/cast',{method:'POST',headers:{'Idempotency-Key':crypto.randomUUID()},body:{date:date||'',time:time||''}});st.notice=tr('cast_pending');});return;
            }
            if(action==='add-rss'){const input=host.querySelector('[name=rss_url]'),url=input?.value.trim(),section=host.querySelector('[name=rss_section]')?.value,feeds=st.draft.rss_feeds||(st.draft.rss_feeds=[]);if(!url){input?.focus();return;}const valid=safeURL(url)&&!new URL(url).hash;if(valid&&section&&feeds.length<10&&!feeds.some(x=>x.url.toLowerCase()===url.toLowerCase())){capturePreferences();feeds.push({url,section});st.rssInput='';draw();}else{input.setAttribute('aria-invalid','true');host.querySelector('.np-rss-error').textContent=tr('rss_invalid');input.focus();}return;}
            if(action==='dismiss'){st.error='';st.notice='';draw();return;}
            if(action==='create'||action==='revision-submit'){const correction=action==='revision-submit'?host.querySelector('[name=correction_note]')?.value.trim()||'':'';act(async()=>{st.run=await request('/editions',{method:'POST',body:{new_revision:action==='revision-submit',correction_note:correction,...creationBody()}});st.creation={};st.view='today';st.notice=tr('started');});return;}
            if(action==='stop'){act(async()=>{await request('/editions/'+encodeURIComponent(st.run.id)+'/stop',{method:'POST',body:{}});st.notice=tr('stopping');});return;}
            if(action==='email-challenge'){act(async()=>{await saveEmailDestinationForChallenge();try{await request('/email/challenge',{method:'POST',body:{}});}catch(err){throw new Error(challengeError(err));}st.notice=tr('code_sent');});return;}
            if(action==='email-confirm'){const code=host.querySelector('[name=confirmation_code]')?.value.trim();act(async()=>{st.profile=await request('/email/confirm',{method:'POST',body:{code}});st.draft=clone(st.profile);st.notice=tr('email_verified');});return;}
            if(action==='telegram-test'){act(async()=>{await request('/telegram/test',{method:'POST',body:{}});st.notice=tr('test_sent');});return;}
            if(action==='whatsapp-test'){act(async()=>{await request('/whatsapp/test',{method:'POST',body:{}});st.notice=tr('test_sent');});return;}
            if(action==='pdf'){act(async()=>{const resp=await fetch(base+'/editions/'+encodeURIComponent(st.edition.id)+'/export?format=pdf',{credentials:'same-origin',signal:controller.signal});if(!resp.ok){const p=await resp.json();throw new Error(p.error||tr('error'));}const blob=await resp.blob(),url=URL.createObjectURL(blob),link=document.createElement('a');link.href=url;link.download='newspaper-'+st.edition.local_date+'-r'+st.edition.revision+'.pdf';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);});}
        },{signal:controller.signal});
        (async()=>{try{await refresh();poll();}catch(err){st.error=err.message||tr('error');draw();}})();
    }
    function dispose(id){const st=instances.get(id);if(!st)return;st.disposed=true;st.player.pause();st.player.removeAttribute('src');st.player.load();clearTimeout(st.timer);clearTimeout(st.previewTimer);st.controller.abort();instances.delete(id);}
    window.NewspaperApp={render,dispose};
})();
