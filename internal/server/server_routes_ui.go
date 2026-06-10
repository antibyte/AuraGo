package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/tools"
	"aurago/ui"
)

const desktopWidgetWorkspaceCSP = "sandbox allow-scripts allow-forms allow-modals; default-src 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' data: blob:; font-src 'self'; connect-src 'self' https://api.open-meteo.com https://geocoding-api.open-meteo.com; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'"
const desktopAppWorkspaceCSP = "sandbox allow-scripts allow-forms allow-modals; default-src 'none'; script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; media-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'"
const desktopWidgetAutoResizeMarker = "data-aurago-widget-auto-resize"
const desktopAppKeyBridgeMarker = "data-aurago-app-key-bridge"

var desktopPrinterCameraProxyPattern = regexp.MustCompile(`/api/3d-printers/[^"'<>\\\s)]+/camera/stream(?:\?[^"'<>\\\s)]*)?`)
var desktopAppResourceAttrPattern = regexp.MustCompile(`\b(src|href)=(["'])([^"']+)(["'])`)
var desktopAppExternalScriptPattern = regexp.MustCompile(`(?is)<script\b[^>]*\bsrc=["']([^"']+)["'][^>]*>\s*</script>`)

// uiBuildVersion is set once at server start and used as a cache-busting
// query parameter for all embedded static assets. Formatted as a compact
// timestamp (e.g. "20260509T143012a").
var uiBuildVersion string

func init() {
	uiBuildVersion = formatUIBuildVersion(time.Now())
}

func formatUIBuildVersion(now time.Time) string {
	return now.Format("20060102T150405") + "a"
}

// uiTemplateData returns the common template data map shared by all HTML pages.
func uiTemplateData(lang string) map[string]interface{} {
	return map[string]interface{}{
		"Lang":         lang,
		"I18N":         getI18NJSON(lang),
		"BuildVersion": uiBuildVersion,
	}
}

// notFoundResponseWriter wraps http.ResponseWriter to detect 404 responses
// from the static file server so we can serve a branded 404 page instead.
type notFoundResponseWriter struct {
	http.ResponseWriter
	path     string
	notFound bool
}

func (w *notFoundResponseWriter) WriteHeader(code int) {
	if code == http.StatusNotFound {
		w.notFound = true
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *notFoundResponseWriter) Write(b []byte) (int, error) {
	if w.notFound {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}

const desktopWidgetAutoResizeScript = `<script data-aurago-widget-auto-resize>(function(){
if(window.__auragoWidgetAutoResize)return;
window.__auragoWidgetAutoResize=true;
var params=new URLSearchParams(location.search);
if(!params.get('widget_id')||!window.parent||window.parent===window)return;
var frame=0,lastResizePayload=null,lastResizePostAt=0;
function measure(){
var doc=document.documentElement,body=document.body,width=0,height=0,sx=window.scrollX||0,sy=window.scrollY||0;
var viewportWidth=window.innerWidth||doc.clientWidth||(body&&body.clientWidth)||0;
var viewportHeight=window.innerHeight||doc.clientHeight||(body&&body.clientHeight)||0;
function include(node){
if(!node)return;
var rect=typeof node.getBoundingClientRect==='function'?node.getBoundingClientRect():null;
var left=rect?rect.left+sx:0,top=rect?rect.top+sy:0;
width=Math.max(width,node.scrollWidth||0,node.offsetWidth||0,node.clientWidth||0,rect?rect.right+sx:0,left+(node.scrollWidth||0));
if(node===doc||node===body)return;
var nodeHeight=Math.max(node.clientHeight||0,node.offsetHeight||0,rect?rect.height:0);
var fillsViewport=viewportHeight>0&&nodeHeight>=viewportHeight-2&&(node.scrollHeight||0)<=nodeHeight+2&&node.children&&node.children.length;
if(fillsViewport)return;
height=Math.max(height,node.scrollHeight||0,node.offsetHeight||0,node.clientHeight||0,rect?rect.bottom+sy:0,top+(node.scrollHeight||0));
}
include(doc);
include(body);
if(body)body.querySelectorAll('*').forEach(include);
var documentScrollHeight=Math.max(doc.scrollHeight||0,body?body.scrollHeight||0:0,body?body.offsetHeight||0:0);
var contentOverflowsViewport=viewportHeight>0&&documentScrollHeight>viewportHeight+2;
if(contentOverflowsViewport)height=Math.max(height,documentScrollHeight);
return{width:Math.ceil(width),height:Math.ceil(Math.max(height,1)),viewportWidth:Math.ceil(viewportWidth),viewportHeight:Math.ceil(viewportHeight)};
}
function shouldPostResize(next){
if(!next)return false;
var now=Date.now();
if(lastResizePayload&&now-lastResizePostAt<250)return false;
if(!lastResizePayload){lastResizePayload=next;lastResizePostAt=now;return true;}
var changed=Math.abs(next.width-lastResizePayload.width)>2||Math.abs(next.height-lastResizePayload.height)>2;
if(changed){lastResizePayload=next;lastResizePostAt=now;}
return changed;
}
function send(){
if(frame)cancelAnimationFrame(frame);
frame=requestAnimationFrame(function(){
frame=0;
var payload=measure();
if(!shouldPostResize(payload))return;
window.parent.postMessage({type:'aurago.desktop.request',action:'desktop:widget:resize',payload:payload},'*');
});
}
function start(){
send();
if(window.ResizeObserver){
var ro=new ResizeObserver(send);
if(document.documentElement)ro.observe(document.documentElement);
if(document.body)ro.observe(document.body);
}
if(window.MutationObserver&&document.body)new MutationObserver(send).observe(document.body,{childList:true,subtree:true,attributes:true,characterData:true});
window.addEventListener('load',send,{once:true});
window.addEventListener('resize',send);
if(document.fonts&&document.fonts.ready)document.fonts.ready.then(send).catch(function(){});
[100,500,1500].forEach(function(ms){setTimeout(send,ms);});
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start,{once:true});else start();
})();</script>`

const desktopAppKeyBridgeScript = `<script data-aurago-app-key-bridge>(function(){
if(window.__auragoAppKeyBridge)return;
window.__auragoAppKeyBridge=true;
function installStorageShim(name){
try{var nativeStore=window[name],probe='__aurago_storage_probe__';nativeStore.setItem(probe,'1');nativeStore.removeItem(probe);return;}catch(_){}
var data=Object.create(null);
var shim={get length(){return Object.keys(data).length;},key:function(index){var keys=Object.keys(data);return keys[index]||null;},getItem:function(key){key=String(key);return Object.prototype.hasOwnProperty.call(data,key)?data[key]:null;},setItem:function(key,value){data[String(key)]=String(value);},removeItem:function(key){delete data[String(key)];},clear:function(){data=Object.create(null);}};
try{Object.defineProperty(window,name,{value:shim,configurable:true});}catch(_){try{window[name]=shim;}catch(__){}}
}
installStorageShim('localStorage');
installStorageShim('sessionStorage');
var keyboardHandlers={keydown:[],keyup:[]};
var originalWindowAddEventListener=window.addEventListener.bind(window);
window.addEventListener=function(type,handler,options){
if((type==='keydown'||type==='keyup')&&typeof handler==='function')keyboardHandlers[type].push(handler);
return originalWindowAddEventListener(type,handler,options);
};
window.addEventListener('message',function(event){
var msg=event&&event.data;
if(!msg||msg.type!=='aurago.desktop.key-event')return;
var eventType=msg.eventType==='keyup'?'keyup':'keydown';
var key=String(msg.key||''),code=String(msg.code||'');
if(!code&&(key===' '||key==='Spacebar'))code='Space';
if(!key&&code==='Space')key=' ';
var prevented=false;
var eventObject={key:key,code:code,location:Number(msg.location)||0,ctrlKey:!!msg.ctrlKey,shiftKey:!!msg.shiftKey,altKey:!!msg.altKey,metaKey:!!msg.metaKey,repeat:!!msg.repeat,type:eventType,defaultPrevented:false,preventDefault:function(){prevented=true;this.defaultPrevented=true;},stopPropagation:function(){}};
keyboardHandlers[eventType].slice().forEach(function(handler){try{handler.call(window,eventObject);}catch(_){}});
var init={key:key,code:code,location:eventObject.location,ctrlKey:eventObject.ctrlKey,shiftKey:eventObject.shiftKey,altKey:eventObject.altKey,metaKey:eventObject.metaKey,repeat:eventObject.repeat,bubbles:true,cancelable:true};
function dispatch(target){
if(!target||typeof target.dispatchEvent!=='function')return;
try{var keyEvent=new KeyboardEvent(eventType,init);target.dispatchEvent(keyEvent);}catch(_){}
}
var active=document.activeElement;
if(active&&active!==document.body&&active!==document.documentElement)dispatch(active);
try{document.querySelectorAll('canvas,[tabindex]').forEach(dispatch);}catch(_){}
document.dispatchEvent(new KeyboardEvent(eventType,init));
window.dispatchEvent(new KeyboardEvent(eventType,init));
});
})();</script>`

func shouldInjectDesktopWidgetAutoResize(r *http.Request) bool {
	if r == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return false
	}
	if strings.TrimSpace(r.URL.Query().Get("widget_id")) == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(r.URL.Path))
	return ext == ".html" || ext == ".htm"
}

func injectDesktopWidgetAutoResizeHTML(content []byte) []byte {
	if len(content) == 0 || bytes.Contains(content, []byte(desktopWidgetAutoResizeMarker)) {
		return content
	}
	lower := bytes.ToLower(content)
	if idx := bytes.LastIndex(lower, []byte("</body>")); idx >= 0 {
		out := make([]byte, 0, len(content)+len(desktopWidgetAutoResizeScript))
		out = append(out, content[:idx]...)
		out = append(out, desktopWidgetAutoResizeScript...)
		out = append(out, content[idx:]...)
		return out
	}
	out := make([]byte, 0, len(content)+len(desktopWidgetAutoResizeScript))
	out = append(out, content...)
	out = append(out, desktopWidgetAutoResizeScript...)
	return out
}

func injectDesktopAppKeyBridgeHTML(content []byte) []byte {
	if len(content) == 0 || bytes.Contains(content, []byte(desktopAppKeyBridgeMarker)) {
		return content
	}
	lower := bytes.ToLower(content)
	if idx := bytes.Index(lower, []byte("<head")); idx >= 0 {
		if end := bytes.IndexByte(content[idx:], '>'); end >= 0 {
			insertAt := idx + end + 1
			out := make([]byte, 0, len(content)+len(desktopAppKeyBridgeScript))
			out = append(out, content[:insertAt]...)
			out = append(out, desktopAppKeyBridgeScript...)
			out = append(out, content[insertAt:]...)
			return out
		}
	}
	if idx := bytes.Index(lower, []byte("<script")); idx >= 0 {
		out := make([]byte, 0, len(content)+len(desktopAppKeyBridgeScript))
		out = append(out, content[:idx]...)
		out = append(out, desktopAppKeyBridgeScript...)
		out = append(out, content[idx:]...)
		return out
	}
	if idx := bytes.Index(lower, []byte("<body")); idx >= 0 {
		if end := bytes.IndexByte(content[idx:], '>'); end >= 0 {
			insertAt := idx + end + 1
			out := make([]byte, 0, len(content)+len(desktopAppKeyBridgeScript))
			out = append(out, content[:insertAt]...)
			out = append(out, desktopAppKeyBridgeScript...)
			out = append(out, content[insertAt:]...)
			return out
		}
	}
	out := make([]byte, 0, len(content)+len(desktopAppKeyBridgeScript))
	out = append(out, desktopAppKeyBridgeScript...)
	out = append(out, content...)
	return out
}

func serveDesktopWidgetAutoResizeHTML(w http.ResponseWriter, r *http.Request, desktopDir string, cfg *config.Config) bool {
	if !shouldInjectDesktopWidgetAutoResize(r) {
		return false
	}
	relPath, err := normalizeDesktopEmbedPath(strings.TrimPrefix(r.URL.Path, "/files/desktop/"))
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	fullPath := filepath.Join(desktopDir, filepath.FromSlash(relPath))
	rootAbs, rootErr := filepath.Abs(desktopDir)
	fullAbs, fullErr := filepath.Abs(fullPath)
	if rootErr != nil || fullErr != nil {
		http.NotFound(w, r)
		return true
	}
	relToRoot, relErr := filepath.Rel(rootAbs, fullAbs)
	if relErr != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(os.PathSeparator)) || filepath.IsAbs(relToRoot) {
		http.NotFound(w, r)
		return true
	}
	info, err := os.Stat(fullAbs)
	if err != nil || info.IsDir() {
		return false
	}
	content, err := os.ReadFile(fullAbs)
	if err != nil {
		return false
	}
	content = prepareDesktopHTMLContentForEmbed(content, cfg, r.URL.Query().Get(desktopEmbedTokenParam))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", desktopWidgetWorkspaceCSP)
	http.ServeContent(w, r, filepath.Base(fullAbs), info.ModTime(), bytes.NewReader(injectDesktopWidgetAutoResizeHTML(content)))
	return true
}

func desktopWorkspaceCSPForPath(requestPath string) string {
	rel := strings.TrimPrefix(requestPath, "/files/desktop/")
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	if strings.HasPrefix(rel, "Apps/") {
		return desktopAppWorkspaceCSP
	}
	return desktopWidgetWorkspaceCSP
}

func desktopRequestOrigin(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}
	scheme := "https"
	if r.TLS == nil {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
			scheme = strings.ToLower(strings.TrimSpace(strings.Split(forwarded, ",")[0]))
		} else if strings.EqualFold(r.URL.Scheme, "http") {
			scheme = "http"
		}
	}
	return scheme + "://" + host
}

func desktopAppWorkspaceCSPForRequest(r *http.Request) string {
	origin := desktopRequestOrigin(r)
	if origin == "" {
		return desktopAppWorkspaceCSP
	}
	csp := desktopAppWorkspaceCSP
	for _, directive := range []string{"script-src", "style-src", "font-src", "worker-src"} {
		csp = strings.Replace(csp, directive+" ", directive+" "+origin+" ", 1)
	}
	return csp
}

func desktopWorkspaceCSPForRequest(r *http.Request) string {
	if r == nil {
		return desktopWidgetWorkspaceCSP
	}
	requestPath := r.URL.Path
	rel := strings.TrimPrefix(requestPath, "/files/desktop/")
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	if strings.HasPrefix(rel, "Apps/") {
		return desktopAppWorkspaceCSPForRequest(r)
	}
	return desktopWidgetWorkspaceCSP
}

func setDesktopFileResponseHeaders(w http.ResponseWriter, r *http.Request) {
	requestPath := ""
	if r != nil && r.URL != nil {
		requestPath = r.URL.Path
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Security-Policy", desktopWorkspaceCSPForRequest(r))
	if !shouldServeDesktopFileInline(requestPath) {
		filename := filepath.Base(strings.TrimPrefix(requestPath, "/files/desktop/"))
		if filename == "." || filename == string(filepath.Separator) || strings.TrimSpace(filename) == "" {
			filename = "download"
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	}
}

func shouldServeDesktopFileInline(requestPath string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(requestPath)))
	switch ext {
	case ".html", ".htm", ".css", ".js", ".mjs", ".json", ".webmanifest",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".ico",
		".mp3", ".wav", ".ogg", ".m4a", ".mp4", ".m4v", ".webm", ".mov", ".ogv",
		".woff", ".woff2", ".ttf", ".otf", ".wasm", ".glb":
		return true
	default:
		return false
	}
}

func serveDesktopExactIndexFile(w http.ResponseWriter, r *http.Request, desktopDir string, cfg *config.Config) bool {
	relPath, err := normalizeDesktopEmbedPath(strings.TrimPrefix(r.URL.Path, "/files/desktop/"))
	if err != nil || !strings.EqualFold(filepath.Base(relPath), "index.html") {
		return false
	}
	fullPath := filepath.Join(desktopDir, filepath.FromSlash(relPath))
	rootAbs, rootErr := filepath.Abs(desktopDir)
	fullAbs, fullErr := filepath.Abs(fullPath)
	if rootErr != nil || fullErr != nil {
		http.NotFound(w, r)
		return true
	}
	relToRoot, relErr := filepath.Rel(rootAbs, fullAbs)
	if relErr != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(os.PathSeparator)) || filepath.IsAbs(relToRoot) {
		http.NotFound(w, r)
		return true
	}
	info, err := os.Stat(fullAbs)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return true
	}
	content, err := os.ReadFile(fullAbs)
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	content = inlineDesktopAppSiblingScripts(content, fullAbs)
	embedToken := r.URL.Query().Get(desktopEmbedTokenParam)
	content = prepareDesktopHTMLContentForEmbed(content, cfg, embedToken)
	content = rewriteDesktopAppResourceURLs(content, info.ModTime(), embedToken)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Security-Policy", desktopAppWorkspaceCSPForRequest(r))
	http.ServeContent(w, r, filepath.Base(fullAbs), info.ModTime(), bytes.NewReader(injectDesktopAppKeyBridgeHTML(content)))
	return true
}

func inlineDesktopAppSiblingScripts(content []byte, indexFilePath string) []byte {
	if len(content) == 0 || !bytes.Contains(content, []byte(`<script`)) {
		return content
	}
	indexDir := filepath.Dir(indexFilePath)
	indexDirAbs, err := filepath.Abs(indexDir)
	if err != nil {
		return content
	}
	return desktopAppExternalScriptPattern.ReplaceAllFunc(content, func(match []byte) []byte {
		parts := desktopAppExternalScriptPattern.FindSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		src := strings.TrimSpace(string(parts[1]))
		if !shouldInlineDesktopAppSiblingAsset(src) {
			return match
		}
		assetPath := filepath.Join(indexDir, filepath.FromSlash(src))
		assetAbs, err := filepath.Abs(assetPath)
		if err != nil || !desktopPathWithinRoot(indexDirAbs, assetAbs) {
			return match
		}
		data, err := os.ReadFile(assetAbs)
		if err != nil || len(data) == 0 {
			return match
		}
		out := make([]byte, 0, len(data)+16)
		out = append(out, "<script>\n"...)
		out = append(out, data...)
		out = append(out, "\n</script>"...)
		return out
	})
}

func shouldInlineDesktopAppSiblingAsset(src string) bool {
	if src == "" || strings.Contains(src, "://") || strings.HasPrefix(src, "//") || strings.HasPrefix(src, "/") {
		return false
	}
	if strings.Contains(src, "..") || strings.Contains(src, `\`) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(src))
	return ext == ".js" || ext == ".mjs"
}

func desktopPathWithinRoot(rootAbs, candidateAbs string) bool {
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return false
	}
	return true
}

func rewriteDesktopAppResourceURLs(content []byte, modTime time.Time, embedToken string) []byte {
	if len(content) == 0 {
		return content
	}
	version := fmt.Sprintf("%d", modTime.UnixNano())
	embedToken = strings.TrimSpace(embedToken)
	rewritten := desktopAppResourceAttrPattern.ReplaceAllStringFunc(string(content), func(match string) string {
		parts := desktopAppResourceAttrPattern.FindStringSubmatch(match)
		if len(parts) != 5 {
			return match
		}
		value := strings.TrimSpace(parts[3])
		if !shouldCacheBustDesktopAppResource(value) {
			return match
		}
		separator := "?"
		if strings.Contains(value, "?") {
			separator = "&"
		}
		value = value + separator + "desktop_v=" + url.QueryEscape(version)
		if embedToken != "" {
			value += "&desktop_token=" + url.QueryEscape(embedToken)
		}
		return parts[1] + "=" + parts[2] + value + parts[4]
	})
	return []byte(rewritten)
}

func shouldCacheBustDesktopAppResource(value string) bool {
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(value, "#") || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return false
	}
	for _, prefix := range []string{"http://", "https://", "data:", "blob:", "mailto:", "tel:", "javascript:", "about:"} {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}

func prepareDesktopHTMLContentForEmbed(content []byte, cfg *config.Config, embedToken string) []byte {
	rewritten := tools.RewriteVirtualDesktopPrinterCameraURLs(cfg, string(content))
	rewritten = appendDesktopTokenToPrinterCameraProxies(rewritten, embedToken)
	return []byte(rewritten)
}

func appendDesktopTokenToPrinterCameraProxies(content, embedToken string) string {
	embedToken = strings.TrimSpace(embedToken)
	if content == "" || embedToken == "" {
		return content
	}
	encodedToken := url.QueryEscape(embedToken)
	return desktopPrinterCameraProxyPattern.ReplaceAllStringFunc(content, func(match string) string {
		if strings.Contains(match, desktopEmbedTokenParam+"=") {
			return match
		}
		separator := "?"
		if strings.Contains(match, "?") {
			separator = "&"
		}
		return match + separator + desktopEmbedTokenParam + "=" + encodedToken
	})
}

func (s *Server) registerUIRoutes(mux *http.ServeMux, shutdownCh chan struct{}) (*http.Server, error) {
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
	_ = mime.AddExtensionType(".js", "application/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".mjs", "application/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".glb", "model/gltf-binary")
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".woff", "font/woff")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	_ = mime.AddExtensionType(".mp4", "video/mp4")
	_ = mime.AddExtensionType(".m4v", "video/mp4")
	_ = mime.AddExtensionType(".webm", "video/webm")
	_ = mime.AddExtensionType(".mov", "video/quicktime")
	_ = mime.AddExtensionType(".ogv", "video/ogg")

	// Serve the embedded Web UI at root via html/template for i18n injection
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to create UI filesystem: %w", err)
	}

	// Load i18n translations from embedded ui/lang/ directory
	loadI18N(uiFS, s.Logger)

	tmpl, err := template.ParseFS(uiFS, "index.html")
	if err != nil {
		s.Logger.Error("Failed to parse UI template", "error", err)
	}

	// Config page (separate template, guarded by WebConfig.Enabled)
	if s.Cfg.WebConfig.Enabled {
		cfgTmpl, cfgErr := template.ParseFS(uiFS, "config.html")
		if cfgErr != nil {
			s.Logger.Error("Failed to parse config UI template", "error", cfgErr)
		}
		mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
			if cfgTmpl == nil {
				http.Error(w, "Config template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			data["I18NMeta"] = getI18NMetaJSON()
			if err := cfgTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute config template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		// Serve the help texts JSON
		mux.HandleFunc("/config_help.json", func(w http.ResponseWriter, r *http.Request) {
			helpData, err := fs.ReadFile(uiFS, "config_help.json")
			if err != nil {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(helpData)
		})
		s.Logger.Info("Config UI enabled at /config")

		// Dashboard page (separate template, guarded by WebConfig.Enabled)
		dashTmpl, dashErr := template.ParseFS(uiFS, "dashboard.html")
		if dashErr != nil {
			s.Logger.Error("Failed to parse dashboard UI template", "error", dashErr)
		}
		mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			if dashTmpl == nil {
				http.Error(w, "Dashboard template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			if err := dashTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute dashboard template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Dashboard UI enabled at /dashboard")

		desktopTmpl, desktopErr := template.ParseFS(uiFS, "desktop.html")
		if desktopErr != nil {
			s.Logger.Error("Failed to parse desktop UI template", "error", desktopErr)
		}
		mux.HandleFunc("/desktop", func(w http.ResponseWriter, r *http.Request) {
			if desktopTmpl == nil {
				http.Error(w, "Desktop template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			if err := desktopTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute desktop template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Virtual Desktop UI enabled at /desktop")

		plansTmpl, plansErr := template.ParseFS(uiFS, "plans.html")
		if plansErr != nil {
			s.Logger.Error("Failed to parse plans UI template", "error", plansErr)
		}
		mux.HandleFunc("/plans", func(w http.ResponseWriter, r *http.Request) {
			if plansTmpl == nil {
				http.Error(w, "Plans template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			if err := plansTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute plans template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Plans UI enabled at /plans")

		// Mission Control page (legacy v1)
		mux.HandleFunc("/missions", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/missions/v2", http.StatusMovedPermanently)
		})
		s.Logger.Info("Mission Control UI /missions redirects to /missions/v2")

		// Mission Control V2 page (enhanced with triggers)
		missionV2Tmpl, missionV2Err := template.ParseFS(uiFS, "missions_v2.html")
		if missionV2Err != nil {
			s.Logger.Error("Failed to parse mission V2 UI template", "error", missionV2Err)
		}
		mux.HandleFunc("/missions/v2", func(w http.ResponseWriter, r *http.Request) {
			if missionV2Tmpl == nil {
				http.Error(w, "Mission V2 template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			if err := missionV2Tmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute mission V2 template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Mission Control V2 UI enabled at /missions/v2")

		// Cheat Sheet Editor page
		cheatsheetTmpl, cheatsheetErr := template.ParseFS(uiFS, "cheatsheets.html")
		if cheatsheetErr != nil {
			s.Logger.Error("Failed to parse cheatsheet UI template", "error", cheatsheetErr)
		}
		mux.HandleFunc("/cheatsheets", func(w http.ResponseWriter, r *http.Request) {
			if cheatsheetTmpl == nil {
				http.Error(w, "Cheatsheet template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			if err := cheatsheetTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute cheatsheet template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Cheat Sheet Editor UI enabled at /cheatsheets")

		// ── Media View Page (replaces Gallery) ──
		mediaTmpl, mediaTmplErr := template.ParseFS(uiFS, "media.html")
		if mediaTmplErr != nil {
			s.Logger.Error("Failed to parse media UI template", "error", mediaTmplErr)
		}
		serveMediaPage := func(w http.ResponseWriter, r *http.Request) {
			if mediaTmpl == nil {
				http.Error(w, "Media template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			if err := mediaTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute media template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		}
		mux.HandleFunc("/media", serveMediaPage)
		// Legacy /gallery redirect for backward compatibility
		mux.HandleFunc("/gallery", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/media", http.StatusMovedPermanently)
		})
		s.Logger.Info("Media View UI enabled at /media (/gallery redirects here)")

		// ── Knowledge Center Page ──
		knowledgeTmpl, knowledgeTmplErr := template.ParseFS(uiFS, "knowledge.html")
		if knowledgeTmplErr != nil {
			s.Logger.Error("Failed to parse knowledge UI template", "error", knowledgeTmplErr)
		}
		mux.HandleFunc("/knowledge", func(w http.ResponseWriter, r *http.Request) {
			if knowledgeTmpl == nil {
				http.Error(w, "Knowledge template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			if err := knowledgeTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute knowledge template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Knowledge Center UI enabled at /knowledge")

		// ── Containers Page ──
		containersTmpl, containersTmplErr := template.ParseFS(uiFS, "containers.html")
		if containersTmplErr != nil {
			s.Logger.Error("Failed to parse containers UI template", "error", containersTmplErr)
		}
		mux.HandleFunc("/containers", func(w http.ResponseWriter, r *http.Request) {
			if containersTmpl == nil {
				http.Error(w, "Containers template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			if err := containersTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute containers template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Containers UI enabled at /containers")

		// ── TrueNAS Storage Page ──
		truenasTmpl, truenasTmplErr := template.ParseFS(uiFS, "truenas.html")
		if truenasTmplErr != nil {
			s.Logger.Error("Failed to parse TrueNAS UI template", "error", truenasTmplErr)
		}
		mux.HandleFunc("/truenas", func(w http.ResponseWriter, r *http.Request) {
			if truenasTmpl == nil {
				http.Error(w, "TrueNAS template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			if err := truenasTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute TrueNAS template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("TrueNAS Storage UI enabled at /truenas")

		// ── Skills Manager Page ──
		skillsTmpl, skillsTmplErr := template.ParseFS(uiFS, "skills.html")
		if skillsTmplErr != nil {
			s.Logger.Error("Failed to parse Skills UI template", "error", skillsTmplErr)
		}
		mux.HandleFunc("/skills", func(w http.ResponseWriter, r *http.Request) {
			if skillsTmpl == nil {
				http.Error(w, "Skills template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := uiTemplateData(lang)
			if err := skillsTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute Skills template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Skills Manager UI enabled at /skills")
	}

	// Invasion Control UI page (always registered — same pattern as /setup)
	invasionTmpl, invasionErr := template.ParseFS(uiFS, "invasion_control.html")
	if invasionErr != nil {
		s.Logger.Error("Failed to parse invasion control UI template", "error", invasionErr)
	}
	mux.HandleFunc("/invasion", func(w http.ResponseWriter, r *http.Request) {
		if invasionTmpl == nil {
			http.Error(w, "Invasion Control template error", http.StatusInternalServerError)
			return
		}
		lang := normalizeLang(s.Cfg.Server.UILanguage)
		data := uiTemplateData(lang)
		if err := invasionTmpl.Execute(w, data); err != nil {
			s.Logger.Error("Failed to execute invasion control template", "error", err)
			http.Error(w, "Template render error", http.StatusInternalServerError)
		}
	})
	s.Logger.Info("Invasion Control UI registered at /invasion")

	// Quick Setup wizard page (always available — parsed outside WebConfig guard)
	setupTmpl, setupErr := template.ParseFS(uiFS, "setup.html")
	if setupErr != nil {
		s.Logger.Error("Failed to parse setup UI template", "error", setupErr)
	}
	mux.HandleFunc("/setup", func(w http.ResponseWriter, r *http.Request) {
		if setupTmpl == nil {
			http.Error(w, "Setup template error", http.StatusInternalServerError)
			return
		}
		lang := normalizeLang(s.Cfg.Server.UILanguage)
		data := uiTemplateData(lang)
		if err := setupTmpl.Execute(w, data); err != nil {
			s.Logger.Error("Failed to execute setup template", "error", err)
			http.Error(w, "Template render error", http.StatusInternalServerError)
		}
	})

	// Auth login / logout pages (registered here so they can use uiFS)
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleAuthLoginPage(s, uiFS)(w, r)
		case http.MethodPost:
			handleAuthLogin(s)(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/auth/logout", handleAuthLogout(s))

	// 404 page template
	notFoundTmpl, notFoundErr := template.ParseFS(uiFS, "404.html")
	if notFoundErr != nil {
		s.Logger.Error("Failed to parse 404 UI template", "error", notFoundErr)
	}

	staticHandler := http.FileServer(http.FS(uiFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Redirect to setup wizard if LLM is not configured (first start)
			s.CfgMu.RLock()
			showSetup := needsSetup(s.Cfg)
			s.CfgMu.RUnlock()

			if showSetup && r.URL.Query().Get("skip_setup") != "1" {
				http.Redirect(w, r, "/setup", http.StatusTemporaryRedirect)
				return
			}

			if tmpl != nil {
				lang := normalizeLang(s.Cfg.Server.UILanguage)
				data := uiTemplateData(lang)
				data["ShowToolResults"] = s.Cfg.Agent.ShowToolResults
				data["DebugMode"] = agent.GetDebugMode()
				data["PersonalityEnabled"] = s.Cfg.Personality.Engine
				if err := tmpl.Execute(w, data); err != nil {
					s.Logger.Error("Failed to execute UI template", "error", err)
					http.Error(w, "Template render error", http.StatusInternalServerError)
					return
				}
			} else {
				http.Error(w, "Template error", http.StatusInternalServerError)
			}
			return
		}
		// Serve static assets from embedded UI FS, fall through to 404 for HTML requests
		nfw := &notFoundResponseWriter{ResponseWriter: w, path: r.URL.Path}
		staticHandler.ServeHTTP(nfw, r)
		if nfw.notFound {
			// Static file not found — serve branded 404 page for HTML requests
			accept := r.Header.Get("Accept")
			if strings.Contains(accept, "text/html") && notFoundTmpl != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				lang := normalizeLang(s.Cfg.Server.UILanguage)
				data := uiTemplateData(lang)
				if err := notFoundTmpl.Execute(w, data); err != nil {
					s.Logger.Error("Failed to execute 404 template", "error", err)
				}
			} else {
				http.NotFound(w, r)
			}
		}
	})

	// Serve generated documents from the document_creator output directory
	docDir := s.Cfg.Tools.DocumentCreator.OutputDir
	if docDir == "" {
		docDir = filepath.Join(s.Cfg.Directories.DataDir, "documents")
	}
	os.MkdirAll(docDir, 0755) // ensure directory exists
	docHandler := http.StripPrefix("/files/documents/", http.FileServer(neuteredFileSystem{http.Dir(docDir)}))
	mux.HandleFunc("/files/documents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		filename := filepath.Base(r.URL.Path)
		// Allow inline display when ?inline=1 is set (e.g. PDF preview)
		if r.URL.Query().Get("inline") == "1" {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
		} else {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		}
		docHandler.ServeHTTP(w, r)
	})

	// Serve agent audio files from data/audio directory
	audioDir := filepath.Join(s.Cfg.Directories.DataDir, "audio")
	os.MkdirAll(audioDir, 0755) // ensure directory exists
	audioHandler := http.StripPrefix("/files/audio/", http.FileServer(neuteredFileSystem{http.Dir(audioDir)}))
	mux.HandleFunc("/files/audio/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		audioHandler.ServeHTTP(w, r)
	})

	// Serve generated images from data directory
	genImgDir := filepath.Join(s.Cfg.Directories.DataDir, "generated_images")
	genImgHandler := http.StripPrefix("/files/generated_images/", http.FileServer(neuteredFileSystem{http.Dir(genImgDir)}))
	mux.HandleFunc("/files/generated_images/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		genImgHandler.ServeHTTP(w, r)
	})

	// Serve generated videos from data directory
	genVideoDir := filepath.Join(s.Cfg.Directories.DataDir, "generated_videos")
	os.MkdirAll(genVideoDir, 0755)
	genVideoHandler := http.StripPrefix("/files/generated_videos/", http.FileServer(neuteredFileSystem{http.Dir(genVideoDir)}))
	mux.HandleFunc("/files/generated_videos/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		genVideoHandler.ServeHTTP(w, r)
	})

	// Serve launchpad icons from data directory
	launchpadIconDir := filepath.Join(s.Cfg.Directories.DataDir, "launchpad_icons")
	os.MkdirAll(launchpadIconDir, 0755)
	launchpadIconHandler := http.StripPrefix("/files/launchpad_icons/", http.FileServer(neuteredFileSystem{http.Dir(launchpadIconDir)}))
	mux.HandleFunc("/files/launchpad_icons/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		launchpadIconHandler.ServeHTTP(w, r)
	})

	// Serve stored Frigate snapshots, frames, and clips from data/frigate_media.
	frigateMediaDir := filepath.Join(s.Cfg.Directories.DataDir, "frigate_media")
	os.MkdirAll(frigateMediaDir, 0755)
	frigateMediaHandler := http.StripPrefix("/files/frigate_media/", http.FileServer(neuteredFileSystem{http.Dir(frigateMediaDir)}))
	mux.HandleFunc("/files/frigate_media/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		frigateMediaHandler.ServeHTTP(w, r)
	})

	// Serve stored 3D printer snapshots from data/3d_printer_media.
	threeDPrinterMediaDir := filepath.Join(s.Cfg.Directories.DataDir, "3d_printer_media")
	os.MkdirAll(threeDPrinterMediaDir, 0755)
	threeDPrinterMediaHandler := http.StripPrefix("/files/3d_printer_media/", http.FileServer(neuteredFileSystem{http.Dir(threeDPrinterMediaDir)}))
	mux.HandleFunc("/files/3d_printer_media/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		threeDPrinterMediaHandler.ServeHTTP(w, r)
	})

	// Serve yt-dlp downloads from the configured video_download directory
	downloadsDir, err := tools.ResolveVideoDownloadDir(s.Cfg)
	if err != nil {
		downloadsDir = filepath.Join(s.Cfg.Directories.DataDir, "downloads")
	}
	os.MkdirAll(downloadsDir, 0755)
	downloadsHandler := http.StripPrefix("/files/downloads/", http.FileServer(neuteredFileSystem{http.Dir(downloadsDir)}))
	mux.HandleFunc("/files/downloads/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		downloadsHandler.ServeHTTP(w, r)
	})

	// Serve static files securely from the workspace directory
	desktopDir := s.Cfg.VirtualDesktop.WorkspaceDir
	if strings.TrimSpace(desktopDir) == "" {
		desktopDir = filepath.Join(s.Cfg.Directories.WorkspaceDir, "virtual_desktop")
	}
	os.MkdirAll(desktopDir, 0755)
	desktopFileHandler := http.StripPrefix("/files/desktop/", http.FileServer(neuteredFileSystem{http.Dir(desktopDir)}))
	mux.HandleFunc("/files/desktop/", func(w http.ResponseWriter, r *http.Request) {
		if !s.Cfg.VirtualDesktop.Enabled {
			http.NotFound(w, r)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			s.Logger.Warn("unauthorized access attempt to desktop files", "path", r.URL.Path, "remote_addr", r.RemoteAddr)
			return
		}
		relPath, err := normalizeDesktopEmbedPath(strings.TrimPrefix(r.URL.Path, "/files/desktop/"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if svc, _, svcErr := s.getDesktopService(r.Context()); svcErr != nil {
			s.Logger.Warn("desktop service unavailable for file integrity check", "path", r.URL.Path, "error", svcErr)
			http.NotFound(w, r)
			return
		} else if ok, reason, verifyErr := svc.VerifyGeneratedAssetIntegrity(r.Context(), relPath); verifyErr != nil {
			s.Logger.Warn("desktop file integrity check failed", "path", r.URL.Path, "error", verifyErr)
			http.NotFound(w, r)
			return
		} else if !ok {
			http.Error(w, fmt.Sprintf("desktop asset integrity check failed: %s", reason), http.StatusConflict)
			return
		}
		setDesktopFileResponseHeaders(w, r)
		if serveDesktopWidgetAutoResizeHTML(w, r, desktopDir, s.Cfg) {
			return
		}
		if serveDesktopExactIndexFile(w, r, desktopDir, s.Cfg) {
			return
		}
		desktopFileHandler.ServeHTTP(w, r)
	})

	fsHandler := http.StripPrefix("/files/", http.FileServer(neuteredFileSystem{http.Dir(s.Cfg.Directories.WorkspaceDir)}))
	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if isActiveContentExtension(r.URL.Path) {
			filename := filepath.Base(r.URL.Path)
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		}
		fsHandler.ServeHTTP(w, r)
	})

	// Serve TTS audio files from data/tts/ on the main server
	ttsDir := tools.TTSAudioDir(s.Cfg.Directories.DataDir)
	os.MkdirAll(ttsDir, 0755)
	mainTTSHandler := http.StripPrefix("/tts/", http.FileServer(http.Dir(ttsDir)))
	mux.HandleFunc("/tts/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wav") {
			w.Header().Set("Content-Type", "audio/wav")
		} else {
			w.Header().Set("Content-Type", "audio/mpeg")
		}
		mainTTSHandler.ServeHTTP(w, r)
	})

	// Phase X: Dedicated TTS Server for Chromecast
	// Declared outside the if-block so the graceful shutdown goroutine can close it.
	var ttsServer *http.Server
	if s.Cfg.Chromecast.Enabled && s.Cfg.Chromecast.TTSPort > 0 {
		ccTTSDir := tools.TTSAudioDir(s.Cfg.Directories.DataDir)
		ttsMux := http.NewServeMux()
		ttsFsHandler := http.StripPrefix("/tts/", http.FileServer(http.Dir(ccTTSDir)))
		ttsMux.HandleFunc("/tts/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".wav") {
				w.Header().Set("Content-Type", "audio/wav")
			} else {
				w.Header().Set("Content-Type", "audio/mpeg")
			}
			ttsFsHandler.ServeHTTP(w, r)
		})

		// Bind TTS to the configured server host so it doesn't accidentally
		// listen on all interfaces when the server is internet-facing.
		// Chromecasts reach it on the LAN IP the operator put in server.host.
		ttsHost := s.Cfg.Server.Host
		if ttsHost == "" {
			ttsHost = "0.0.0.0"
		}
		ttsServer = &http.Server{
			Addr:    fmt.Sprintf("%s:%d", ttsHost, s.Cfg.Chromecast.TTSPort),
			Handler: ttsMux,
		}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.Logger.Error("[TTS Server] Goroutine panic recovered", "error", r)
				}
			}()
			s.Logger.Info("Starting Dedicated TTS Server", "host", ttsHost, "port", s.Cfg.Chromecast.TTSPort)
			if err := ttsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.Logger.Warn("Dedicated TTS Server failed (Chromecast audio will not be available)", "error", err)
			}
		}()
	}

	return ttsServer, nil
}
