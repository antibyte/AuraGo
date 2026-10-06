package server

import "bytes"

const desktopSDKChannelMarker = "data-aurago-desktop-sdk-channel"

// desktopSDKChannelBootstrapScript installs the first-party half of the SDK
// MessageChannel handshake. The capability arrives in the iframe fragment,
// which browsers never send in HTTP requests or Referer headers. This script
// is injected only into rooted Desktop app/widget HTML entry documents.
const desktopSDKChannelBootstrapScript = `<script data-aurago-desktop-sdk-channel>(function(){
if(window.__AURAGO_DESKTOP_SDK_CHANNEL__)return;
var fragment=new URLSearchParams((window.location.hash||'').slice(1));
var capability=fragment.get('__aurago_sdk_channel')||'';
var originalHash=fragment.get('__aurago_sdk_original_hash')||'';
if(!/^[0-9a-f]{64}$/.test(capability))return;
try{history.replaceState(history.state,'',location.pathname+location.search+originalHash);}catch(_){return;}
var activePort=null,listeners=[];
var bridge=Object.freeze({onPort:function(handler){if(typeof handler!=='function')return function(){};listeners.push(handler);if(activePort)handler(activePort);return function(){listeners=listeners.filter(function(item){return item!==handler;});};}});
try{Object.defineProperty(window,'__AURAGO_DESKTOP_SDK_CHANNEL__',{value:bridge,configurable:false,writable:false});}catch(_){return;}
window.addEventListener('message',function(event){
if(!event||event.source!==window.parent||!event.origin||event.origin==='null')return;
var message=event.data;
if(!message||message.type!=='aurago.desktop.channel.challenge'||!Number.isSafeInteger(message.challenge))return;
if(typeof MessageChannel!=='function')return;
var channel=new MessageChannel();
if(activePort)try{activePort.close();}catch(_){}
activePort=channel.port1;
try{event.source.postMessage({type:'aurago.desktop.channel.handshake',capability:capability,challenge:message.challenge},event.origin,[channel.port2]);}
catch(_){activePort.close();activePort=null;return;}
listeners.slice().forEach(function(handler){try{handler(activePort);}catch(_){}});
});
})();</script>`

// injectDesktopSDKChannelHTML adds the bootstrap before application scripts.
// Callers must first prove that content is a rooted Desktop app/widget HTML
// document; the bootstrap deliberately does not run on arbitrary HTML files.
func injectDesktopSDKChannelHTML(content []byte) []byte {
	if len(content) == 0 || bytes.Contains(content, []byte(desktopSDKChannelMarker)) {
		return content
	}
	script := []byte(desktopSDKChannelBootstrapScript)
	lower := bytes.ToLower(content)
	firstScript := bytes.Index(lower, []byte("<script"))
	if idx := bytes.Index(lower, []byte("<head")); idx >= 0 {
		if end := bytes.IndexByte(content[idx:], '>'); end >= 0 {
			insertAt := idx + end + 1
			if firstScript >= 0 && firstScript < insertAt {
				insertAt = firstScript
			}
			return insertDesktopSDKChannelScript(content, insertAt, script)
		}
	}
	if firstScript >= 0 {
		return insertDesktopSDKChannelScript(content, firstScript, script)
	}
	return append(append([]byte(nil), script...), content...)
}

func insertDesktopSDKChannelScript(content []byte, at int, script []byte) []byte {
	out := make([]byte, 0, len(content)+len(script))
	out = append(out, content[:at]...)
	out = append(out, script...)
	out = append(out, content[at:]...)
	return out
}
