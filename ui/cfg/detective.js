// Server-owned research profiles. Existing runs keep their accepted budgets.
function renderDetectiveSection(section) {
    const data = configData.detective || (configData.detective = {enabled:true,readonly:false});
    const defaults = {quick:{seconds:300,tools:40,iterations:60,tokens:0},normal:{seconds:900,tools:100,iterations:140,tokens:0},maximum:{seconds:3600,tools:250,iterations:320,tokens:0}};
    const label = key => escapeHtml(t('config.detective.'+key));
    let html = '<div class="cfg-section active"><div class="section-header">Detective</div><div class="section-desc">'+escapeHtml(section.desc)+'</div>';
    for (const key of ['enabled','readonly']) {
        html += '<div class="field-group"><div class="field-label">'+label(key)+'</div><div class="toggle '+(data[key]?'on':'')+'" data-path="detective.'+key+'" onclick="toggleBool(this)"></div></div>';
    }
    html += '<div class="cfg-note-banner">'+label('restart')+'</div>';
    for (const effort of ['quick','normal','maximum']) {
        const p = {...defaults[effort],...(data.profiles?.[effort]||{})};
        html += '<h3>'+label(effort)+'</h3><div class="field-grid two-cols">';
        for (const [key,min,max] of [['seconds',30,7200],['tools',1,500],['iterations',1,750],['tokens',0,10000000]]) {
            html += '<div class="field-group"><div class="field-label">'+label(key)+'</div><input class="field-input" type="number" min="'+min+'" max="'+max+'" value="'+Number(p[key])+'" data-path="detective.profiles.'+effort+'.'+key+'"></div>';
        }
        html += '</div>';
    }
    document.getElementById('content').innerHTML = html+'</div>';
    attachChangeListeners();
}
