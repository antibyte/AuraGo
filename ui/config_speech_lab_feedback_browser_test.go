package ui

import (
	"testing"
	"time"
)

func TestConfigSpeechLabFeedbackBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	page = page.Timeout(30 * time.Second)
	page.MustWaitLoad()
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
        const originalFetch = window.fetch;
        const json = (body, status = 200) => new Response(JSON.stringify(body), {
            status, headers: {'Content-Type': 'application/json'}
        });
        window.speechRequests = [];
        window.speechDeployment = {managed:true, state:'ready', bundle:'stable', requested_bundle:'stable', progress:100};
        window.showConfirm = async () => true;
        window.fetch = (url, options = {}) => {
            const path = String(url);
            if (path === '/api/config' && options.method === 'PUT' && window.speechHoldConfig) {
                return new Promise((resolve, reject) => {window.failSpeechConfig = reject;});
            }
            if (!path.startsWith('/api/speech-lab/')) return originalFetch(url, options);
            speechRequests.push({path, method:options.method || 'GET'});
            if (path === '/api/speech-lab/status') {
                if (window.speechStatusFails) return Promise.resolve(json({message:'status unavailable'}, 503));
                if (window.speechHoldStatus) {
                    return new Promise(resolve => {window.releaseSpeechStatus = () => {
                        window.speechHoldStatus = false;
                        resolve(json({enabled:true, ready:true, asr_id:'asr', tts_id:'tts', voice:'voice', deployment:speechDeployment}));
                    };});
                }
                return Promise.resolve(json({enabled:true, ready:true, asr_id:'asr', tts_id:'tts', voice:'voice', deployment:speechDeployment}));
            }
            if (path === '/api/speech-lab/capability') {
                return Promise.resolve(window.speechCapabilityFails ? json({message:'capability unavailable'}, 503) : json({tier:'local'}));
            }
            if (path === '/api/speech-lab/catalog') return Promise.resolve(json({backends:[
                {id:'asr', name:'ASR', stage:'asr', available:true, stable:true},
                {id:'confucius4-r2t2', name:'Confucius4-R2T2 GGUF', stage:'asr', available:false,
                 variants:[{id:'confucius4-r2t2-vulkan-windows', stable:true}]},
                {id:'tts', name:'TTS', stage:'tts', available:true, stable:true, voices:['voice']}
            ]}));
            if (path.startsWith('/api/speech-lab/suggestions')) return Promise.resolve(json({suggested_pairs:[]}));
            if (path === '/api/speech-lab/deployment/update') {
                return new Promise((resolve, reject) => {
                    window.finishSpeechUpdate = () => resolve(json({ok:true, deployment:speechDeployment}));
                    window.failSpeechUpdate = reject;
                });
            }
            if (path === '/api/speech-lab/stack') {
                return new Promise(resolve => {window.finishSpeechStack = () => resolve(json({message:'Stack switched'}));});
            }
            if (path === '/api/speech-lab/deployment' && options.method === 'DELETE') {
                speechDeployment.state = 'disabled';
                return Promise.resolve(json({ok:true, deployment:speechDeployment}));
            }
            return Promise.resolve(json({message:'Unexpected Speech Lab request'}, 404));
        };
    }`)
	page.MustEval(`async () => { await selectSection('speech_lab'); resetDirtySnapshot(); }`)
	if !page.MustEval(`() => {
        const option = document.querySelector('#speech-lab-asr option[value="confucius4-r2t2"]');
        return !!option && option.disabled && option.textContent.includes(t('config.speech_lab.not_ready')) &&
            document.getElementById('speech-lab-asr').value === 'asr';
    }`).Bool() {
		t.Fatal("stable but unavailable Confucius ASR must remain visible without becoming selectable")
	}
	if !page.MustEval(`() => document.getElementById('speech-lab-action-status').hidden`).Bool() {
		t.Fatal("initial status load should not announce a user action")
	}

	page.MustEval(`() => {window.speechHoldStatus = true; document.getElementById('speech-lab-refresh').click();}`)
	waitForJSBool(t, page, `() => !!window.releaseSpeechStatus && document.getElementById('speech-lab-refresh').disabled && !!document.querySelector('#speech-lab-action-status progress')`)
	page.MustEval(`() => releaseSpeechStatus()`)
	waitForJSBool(t, page, `() => {const status=document.getElementById('speech-lab-action-status');return !document.getElementById('speech-lab-refresh').disabled && status.textContent===t('config.speech_lab.refresh_done') && !status.querySelector('progress');}`)

	page.MustEval(`() => {window.speechCapabilityFails=true; document.getElementById('speech-lab-refresh').click();}`)
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent===t('config.speech_lab.refresh_partial') && !document.getElementById('speech-lab-refresh').disabled`)
	page.MustEval(`() => {window.speechStatusFails=true; document.getElementById('speech-lab-refresh').click();}`)
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent===t('config.speech_lab.unreachable') && document.getElementById('speech-lab-refresh').disabled===false && [...document.querySelectorAll('#speech-lab-deployment button')].every(button=>button.disabled)`)
	page.MustEval(`() => {window.speechStatusFails=false; document.getElementById('speech-lab-refresh').click();}`)
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent===t('config.speech_lab.refresh_partial') && [...document.querySelectorAll('#speech-lab-deployment button')].some(button=>!button.disabled)`)
	page.MustSetViewport(390, 800, 1, true)
	page.MustEval(`() => {window.speechCapabilityFails=false; speechDeployment.state='pulling'; [...document.querySelectorAll('#speech-lab-deployment button')].find(button=>button.textContent===t('config.speech_lab.deployment_update')).click();}`)
	waitForJSBool(t, page, `() => !!window.finishSpeechUpdate && !!document.querySelector('#speech-lab-action-status progress') && [...document.querySelectorAll('#speech-lab-deployment button')].every(button=>button.disabled)`)
	if page.MustEval(`() => document.documentElement.scrollWidth>innerWidth+1 || document.getElementById('content').scrollWidth>document.getElementById('content').clientWidth+1`).Bool() {
		t.Fatal("pending Speech Lab action overflows the narrow viewport")
	}
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent.includes(t('config.speech_lab.phase_pulling'))`)
	page.MustEval(`() => {speechDeployment.state='starting'}`)
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent.includes(t('config.speech_lab.phase_starting'))`)
	page.MustEval(`() => {speechDeployment.state='ready'; finishSpeechUpdate();}`)
	waitForJSBool(t, page, `() => {const status=document.getElementById('speech-lab-action-status');return status.textContent===t('config.speech_lab.action_done').replace('{action}',t('config.speech_lab.deployment_update')) && !status.querySelector('progress') && [...document.querySelectorAll('#speech-lab-deployment button')].some(button=>!button.disabled);}`)
	if !page.MustEval(`() => speechRequests.filter(request=>request.path==='/api/speech-lab/deployment/update').length===1`).Bool() {
		t.Fatal("update was submitted more than once")
	}

	page.MustEval(`() => {speechDeployment.state='pulling'; [...document.querySelectorAll('#speech-lab-deployment button')].find(button=>button.textContent===t('config.speech_lab.deployment_update')).click();}`)
	waitForJSBool(t, page, `() => !!window.failSpeechUpdate && [...document.querySelectorAll('#speech-lab-deployment button')].every(button=>button.disabled)`)
	page.MustEval(`() => {speechDeployment.state='ready'; failSpeechUpdate(new Error('offline'));}`)
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent.includes('offline') && [...document.querySelectorAll('#speech-lab-deployment button')].some(button=>!button.disabled)`)

	page.MustEval(`() => document.querySelector('.speech-lab-stack-actions button').click()`)
	waitForJSBool(t, page, `() => !!window.finishSpeechStack && document.querySelector('.speech-lab-stack-actions button').disabled && !!document.querySelector('#speech-lab-action-status progress')`)
	page.MustEval(`() => finishSpeechStack()`)
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent==='Stack switched' && !document.querySelector('.speech-lab-stack-actions button').disabled`)

	page.MustEval(`() => [...document.querySelectorAll('#speech-lab-deployment button')].find(button=>button.textContent===t('config.speech_lab.deployment_remove')).click()`)
	waitForJSBool(t, page, `() => speechRequests.some(request=>request.path==='/api/speech-lab/deployment' && request.method==='DELETE') && document.getElementById('speech-lab-action-status').textContent.includes(t('config.speech_lab.deployment_remove'))`)
	if page.MustEval(`() => speechRequests.some(request=>request.path==='/api/speech-lab/deployment/remove')`).Bool() {
		t.Fatal("remove used the nonexistent /remove route")
	}

	page.MustEval(`() => {
        window.speechHoldConfig = true;
        const profile = document.querySelector('#speech-lab-deployment select[data-path="speech_lab.deployment.gpu_backend"]');
        profile.value = 'cpu';
        profile.dispatchEvent(new Event('change', {bubbles:true}));
        document.querySelector('.btn-speech-lab-apply').click();
    }`)
	waitForJSBool(t, page, `() => !!window.failSpeechConfig && document.getElementById('speech-lab-action-status').textContent===t('config.speech_lab.hardware_saving') && document.querySelector('.btn-speech-lab-apply').disabled`)
	page.MustEval(`() => failSpeechConfig(new Error('offline'))`)
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent===t('config.speech_lab.hardware_save_failed') && !document.getElementById('speech-lab-refresh').disabled && document.querySelector('#speech-lab-deployment select[data-path="speech_lab.deployment.gpu_backend"]').value==='cpu' && AuraConfigState.dirtyPaths().includes('speech_lab.deployment.gpu_backend')`)
}
