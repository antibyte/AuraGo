(function webpackUniversalModuleDefinition(root, factory) {
	if(typeof exports === 'object' && typeof module === 'object')
		module.exports = factory();
	else if(typeof define === 'function' && define.amd)
		define([], factory);
	else if(typeof exports === 'object')
		exports["rive"] = factory();
	else
		root["rive"] = factory();
})(this, () => {
return /******/ (() => { // webpackBootstrap
/******/ 	"use strict";
/******/ 	var __webpack_modules__ = ([
/* 0 */,
/* 1 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   Animation: () => (/* reexport safe */ _Animation__WEBPACK_IMPORTED_MODULE_0__.Animation)
/* harmony export */ });
/* harmony import */ var _Animation__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(2);



/***/ }),
/* 2 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   Animation: () => (/* binding */ Animation)
/* harmony export */ });
/**
 * Represents an animation that can be played on an Artboard.
 * Wraps animations and instances from the runtime and keeps track of playback state.
 *
 * The `Animation` class manages the state and behavior of a single animation instance,
 * including its current time, loop count, and ability to scrub to a specific time.
 *
 * The class provides methods to advance the animation, apply its interpolated keyframe
 * values to the Artboard, and clean up the underlying animation instance when the
 * animation is no longer needed.
 */
var Animation = /** @class */ (function () {
    /**
     * Constructs a new animation
     * @constructor
     * @param {any} animation: runtime animation object
     * @param {any} instance: runtime animation instance object
     */
    function Animation(animation, artboard, runtime, playing) {
        this.animation = animation;
        this.artboard = artboard;
        this.playing = playing;
        this.loopCount = 0;
        /**
         * The time to which the animation should move to on the next render.
         * If not null, the animation will scrub to this time instead of advancing by the given time.
         */
        this.scrubTo = null;
        this.instance = new runtime.LinearAnimationInstance(animation, artboard);
    }
    Object.defineProperty(Animation.prototype, "name", {
        /**
         * Returns the animation's name
         */
        get: function () {
            return this.animation.name;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Animation.prototype, "time", {
        /**
         * Returns the animation's name
         */
        get: function () {
            return this.instance.time;
        },
        /**
         * Sets the animation's current time
         */
        set: function (value) {
            this.instance.time = value;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Animation.prototype, "loopValue", {
        /**
         * Returns the animation's loop type
         */
        get: function () {
            return this.animation.loopValue;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Animation.prototype, "needsScrub", {
        /**
         * Indicates whether the animation needs to be scrubbed.
         * @returns `true` if the animation needs to be scrubbed, `false` otherwise.
         */
        get: function () {
            return this.scrubTo !== null;
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Advances the animation by the give time. If the animation needs scrubbing,
     * time is ignored and the stored scrub value is used.
     * @param time the time to advance the animation by if no scrubbing required
     */
    Animation.prototype.advance = function (time) {
        if (this.scrubTo === null) {
            this.instance.advance(time);
        }
        else {
            this.instance.time = 0;
            this.instance.advance(this.scrubTo);
            this.scrubTo = null;
        }
    };
    /**
     * Apply interpolated keyframe values to the artboard. This should be called after calling
     * .advance() on an animation instance so that new values are applied to properties.
     *
     * Note: This does not advance the artboard, which updates all objects on the artboard
     * @param mix - Mix value for the animation from 0 to 1
     */
    Animation.prototype.apply = function (mix) {
        this.instance.apply(mix);
    };
    /**
     * Deletes the backing Wasm animation instance; once this is called, this
     * animation is no more.
     */
    Animation.prototype.cleanup = function () {
        this.instance.delete();
    };
    return Animation;
}());



/***/ }),
/* 3 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   RuntimeLoader: () => (/* binding */ RuntimeLoader)
/* harmony export */ });
/* harmony import */ var _rive_advanced_mjs__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(4);
/* harmony import */ var package_json__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(5);
var __assign = (undefined && undefined.__assign) || function () {
    __assign = Object.assign || function(t) {
        for (var s, i = 1, n = arguments.length; i < n; i++) {
            s = arguments[i];
            for (var p in s) if (Object.prototype.hasOwnProperty.call(s, p))
                t[p] = s[p];
        }
        return t;
    };
    return __assign.apply(this, arguments);
};


// Runtime singleton; use getInstance to provide a callback that returns the
// Rive runtime
var RuntimeLoader = /** @class */ (function () {
    // Class is never instantiated
    function RuntimeLoader() {
    }
    // Rejects all pending awaitInstance() promises and resets loading state so
    // the next call to getInstance() / awaitInstance() can retry with a new URL.
    RuntimeLoader.notifyError = function (error) {
        var _a;
        RuntimeLoader.isLoading = false;
        while (RuntimeLoader.errorCallbackQueue.length > 0) {
            (_a = RuntimeLoader.errorCallbackQueue.shift()) === null || _a === void 0 ? void 0 : _a(error);
        }
        RuntimeLoader.callBackQueue = [];
    };
    // Loads the runtime
    RuntimeLoader.loadRuntime = function () {
        // Capture the URL at call time so the catch closure always refers to the
        // URL this particular attempt used, even if wasmURL is mutated for a retry.
        var attemptedUrl = RuntimeLoader.wasmURL;
        var wasmBinary = RuntimeLoader.wasmBinary;
        if (RuntimeLoader.enablePerfMarks)
            performance.mark('rive:wasm-init:start');
        _rive_advanced_mjs__WEBPACK_IMPORTED_MODULE_0__["default"](__assign({ 
            // Loads Wasm bundle
            locateFile: function () { return attemptedUrl; } }, (wasmBinary ? { wasmBinary: wasmBinary } : {})))
            .then(function (rive) {
            var _a;
            if (RuntimeLoader.enablePerfMarks) {
                performance.mark('rive:wasm-init:end');
                performance.measure('rive:wasm-init', 'rive:wasm-init:start', 'rive:wasm-init:end');
            }
            RuntimeLoader.runtime = rive;
            RuntimeLoader.errorCallbackQueue = [];
            // Fire all the callbacks
            while (RuntimeLoader.callBackQueue.length > 0) {
                (_a = RuntimeLoader.callBackQueue.shift()) === null || _a === void 0 ? void 0 : _a(RuntimeLoader.runtime);
            }
        })
            .catch(function (error) {
            // Capture specific error details
            var errorDetails = {
                message: (error === null || error === void 0 ? void 0 : error.message) || "Unknown error",
                type: (error === null || error === void 0 ? void 0 : error.name) || "Error",
                // Some browsers may provide additional WebAssembly-specific details
                wasmError: error instanceof WebAssembly.CompileError ||
                    error instanceof WebAssembly.RuntimeError,
                originalError: error,
            };
            // Log detailed error for debugging
            console.debug("Rive WASM load error details:", errorDetails);
            // In case the primary URL fails, or the wasm was not supported, try the
            // fallback URL (a rive_fallback.wasm compiled for older architectures).
            // The fallback can be customised or disabled via setWasmFallbackUrl().
            // TODO: (Gordon): preemptively test browser support and load the correct wasm file. Then use the fallback only if the primary fails.
            var fallbackUrl = RuntimeLoader.wasmFallbackURL;
            var alreadyOnFallback = fallbackUrl !== null &&
                attemptedUrl.toLowerCase() === fallbackUrl.toLowerCase();
            if (fallbackUrl !== null && !alreadyOnFallback) {
                console.warn("Failed to load WASM from ".concat(attemptedUrl, " (").concat(errorDetails.message, "), trying fallback URL: ").concat(fallbackUrl));
                // Clear wasmBinary so the retry actually fetches via locateFile
                // instead of re-using the same (failing) in-memory binary.
                RuntimeLoader.wasmBinary = null;
                RuntimeLoader.setWasmUrl(fallbackUrl);
                RuntimeLoader.loadRuntime();
            }
            else {
                // When alreadyOnFallback is true, wasmURL has already been overwritten
                // with the fallback URL, so we can no longer recover the original
                // primary URL here. The primary URL was logged in the earlier warning.
                var triedUrls = alreadyOnFallback
                    ? "the configured WASM URL or its fallback (".concat(fallbackUrl, ")")
                    : attemptedUrl;
                var errorMessage = [
                    "Could not load Rive WASM file from ".concat(triedUrls, "."),
                    "Possible reasons:",
                    "- Network connection is down",
                    "- WebAssembly is not supported in this environment",
                    "- The WASM file is corrupted or incompatible",
                    "\nError details:",
                    "- Type: ".concat(errorDetails.type),
                    "- Message: ".concat(errorDetails.message),
                    "- WebAssembly-specific error: ".concat(errorDetails.wasmError),
                    "\nTo resolve, you may need to:",
                    "1. Check your network connection",
                    "2. Set a new WASM source via RuntimeLoader.setWasmUrl()",
                    "3. Call RuntimeLoader.awaitInstance() again",
                ].join("\n");
                console.error(errorMessage);
                RuntimeLoader.notifyError(new Error(errorMessage));
            }
        });
    };
    // Provides a runtime instance via a callback
    RuntimeLoader.getInstance = function (callback, onError) {
        // If it's not loading, start loading runtime
        if (!RuntimeLoader.isLoading) {
            RuntimeLoader.isLoading = true;
            RuntimeLoader.loadRuntime();
        }
        if (!RuntimeLoader.runtime) {
            RuntimeLoader.callBackQueue.push(callback);
            if (onError) {
                RuntimeLoader.errorCallbackQueue.push(onError);
            }
        }
        else {
            callback(RuntimeLoader.runtime);
        }
    };
    // Provides a runtime instance via a promise; rejects if WASM fails to load.
    RuntimeLoader.awaitInstance = function () {
        return new Promise(function (resolve, reject) {
            return RuntimeLoader.getInstance(resolve, reject);
        });
    };
    // Manually sets the wasm url
    RuntimeLoader.setWasmUrl = function (url) {
        RuntimeLoader.wasmURL = url;
    };
    // Gets the current wasm url
    RuntimeLoader.getWasmUrl = function () {
        return RuntimeLoader.wasmURL;
    };
    /**
     * Sets the URL used as a fallback when the primary WASM URL fails to load.
     * Pass `null` to disable the fallback entirely.
     *
     * Defaults to pulling from the jsdelivr CDN.
     */
    RuntimeLoader.setWasmFallbackUrl = function (url) {
        RuntimeLoader.wasmFallbackURL = url;
    };
    // Gets the current fallback wasm url (null means fallback is disabled)
    RuntimeLoader.getWasmFallbackUrl = function () {
        return RuntimeLoader.wasmFallbackURL;
    };
    // Manually sets the wasm binary or clears it with null
    RuntimeLoader.setWasmBinary = function (value) {
        if ((value instanceof ArrayBuffer) || value === null) {
            RuntimeLoader.wasmBinary = value;
            return;
        }
        console.error("setWasmBinary expects an ArrayBuffer or null");
    };
    // Gets the current wasm build as ArrayBuffer or null
    RuntimeLoader.getWasmBinary = function () {
        return RuntimeLoader.wasmBinary;
    };
    // Flag to indicate that loading has started/completed
    RuntimeLoader.isLoading = false;
    // List of callbacks for the runtime that come in while loading
    RuntimeLoader.callBackQueue = [];
    // Path to the Wasm file; default path works for testing only;
    // if embedded wasm is used then this is never used.
    RuntimeLoader.wasmURL = "https://unpkg.com/".concat(package_json__WEBPACK_IMPORTED_MODULE_1__.name, "@").concat(package_json__WEBPACK_IMPORTED_MODULE_1__.version, "/rive.wasm");
    // Fallback WASM URL tried when the primary URL fails. Set to null to disable
    // the fallback entirely. Defaults to pulling from the jsdelivr CDN.
    RuntimeLoader.wasmFallbackURL = "https://cdn.jsdelivr.net/npm/".concat(package_json__WEBPACK_IMPORTED_MODULE_1__.name, "@").concat(package_json__WEBPACK_IMPORTED_MODULE_1__.version, "/rive_fallback.wasm");
    RuntimeLoader.wasmBinary = null;
    // Error callbacks enqueued from .getInstance()
    RuntimeLoader.errorCallbackQueue = [];
    /**
     * When true, performance.mark / performance.measure entries are emitted for
     * WASM initialization.
     */
    RuntimeLoader.enablePerfMarks = false;
    return RuntimeLoader;
}());



/***/ }),
/* 4 */
/***/ ((__unused_webpack___webpack_module__, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   "default": () => (__WEBPACK_DEFAULT_EXPORT__)
/* harmony export */ });
var Rive=(()=>{var _scriptName=globalThis.document?.currentScript?.src;return async function(moduleArg={}){var moduleRtn;var D=moduleArg,ba=!!globalThis.window,ca=!!globalThis.WorkerGlobalScope;
function da(){function d(y){const B=t;l=e=0;t=new Map;B.forEach(F=>{try{F(y)}catch(I){console.error(I)}});this.Ib();x&&x.xc()}let e=0,l=0,t=new Map,x=null,z=null;this.requestAnimationFrame=function(y){e||=requestAnimationFrame(d.bind(this));const B=++l;t.set(B,y);return B};this.cancelAnimationFrame=function(y){t.delete(y);e&&0==t.size&&(cancelAnimationFrame(e),e=0)};this.tc=function(y){z&&(document.body.remove(z),z=null);y||(z=document.createElement("div"),z.style.backgroundColor="black",z.style.position=
"fixed",z.style.right=0,z.style.top=0,z.style.color="white",z.style.padding="4px",z.innerHTML="RIVE FPS",y=function(B){z.innerHTML="RIVE FPS "+B.toFixed(1)},document.body.appendChild(z));x=new function(){let B=0,F=0;this.xc=function(){var I=performance.now();F?(++B,I-=F,1E3<I&&(y(1E3*B/I),B=F=0)):(F=I,B=0)}}};this.qc=function(){z&&(document.body.remove(z),z=null);x=null};this.Ib=function(){}}
function ha(){console.assert(!0);const d=new Map;let e=-Infinity;this.push=function(l){l=l+255>>8;d.has(l)&&clearTimeout(d.get(l));d.set(l,setTimeout(function(){d.delete(l);0==d.length?e=-Infinity:l==e&&(e=Math.max(...d.keys()),console.assert(e<l))},1E3));e=Math.max(l,e);return e<<8}}const ia=D.onRuntimeInitialized;
D.onRuntimeInitialized=function(){ia&&ia();D.startFileScripts=function(y){const B=D.riveScripting;return y&&B&&y.startScripts?B.prepare().then(function(){y.startScripts();return y}):Promise.resolve(y)};let d=D.decodeAudio;D.decodeAudio=function(y,B,F=null){y=d(y,F??null);B(y)};let e=D.decodeFont;D.decodeFont=function(y,B,F=null){y=e(y,F??null);B(y)};const l=D.FileAsset.prototype.decode;D.FileAsset.prototype.decode=function(y,B){return l.call(this,y,B??null)};let t=D.setFallbackFontCb;D.setFallbackFontCallback=
"function"===typeof t?function(y){t(y)}:function(){console.warn("Module.setFallbackFontCallback called, but text support is not enabled in this build.")};const x=D.FileAssetLoader;D.ptrToAsset=y=>{let B=D.ptrToFileAsset(y);return B.isImage?D.ptrToImageAsset(y):B.isFont?D.ptrToFontAsset(y):B.isAudio?D.ptrToAudioAsset(y):B};D.CustomFileAssetLoader=x.extend("CustomFileAssetLoader",{__construct:function({loadContents:y}){this.__parent.__construct.call(this);this.cc=y},loadContents:function(y,B){y=D.ptrToAsset(y);
return this.cc(y,B)}});D.CDNFileAssetLoader=x.extend("CDNFileAssetLoader",{__construct:function(y){this.__parent.__construct.call(this);this.fc=y??null},loadContents:function(y){let B=D.ptrToAsset(y);y=B.cdnUuid;if(""===y)return!1;const F=this.fc??null;(function(I,u){var a=new XMLHttpRequest;a.responseType="arraybuffer";a.onreadystatechange=function(){4==a.readyState&&200==a.status&&u(a)};a.open("GET",I,!0);a.send(null)})(B.cdnBaseUrl+"/"+y,I=>{B.decode(new Uint8Array(I.response),F)});return!0}});
D.FallbackFileAssetLoader=x.extend("FallbackFileAssetLoader",{__construct:function(){this.__parent.__construct.call(this);this.Gb=[]},addLoader:function(y){this.Gb.push(y)},loadContents:function(y,B){for(let F of this.Gb)if(F.loadContents(y,B))return!0;return!1}});let z=D.computeAlignment;D.computeAlignment=function(y,B,F,I,u=1){return z.call(this,y,B,F,I,u)}};
(function(){function d(u,a){const c=u.kc,p=u.ra,q=u.qa,v=u.ab??{i:!1},f=(k,h)=>{k=new Uint8Array(a().buffer,k,h);const m=4096<h?u.gb(h):u.Na(h);if(0===m)throw Error(`could not stage ${h} bytes for the host`);u.ia().set(k,m);v.i&&(v.Aa+=h);return m},b=(k,h)=>{0!==k&&4096<h&&u.cb(k)},g=(k,h,m)=>{(new Uint8Array(a().buffer,h,m)).set(u.ia().subarray(k,k+m));v.i&&(v.ya+=m)},n=k=>h=>{const m=(new Uint32Array(a().buffer,h-4,1))[0],w=p();let r=0;try{r=f(h,m),c.re(k,r,m)}finally{b(r,m),q(w)}};return{env:{seed:()=>
Date.now()*Math.random(),emscripten_get_now:()=>performance.now(),emscripten_date_now:()=>Date.now(),trace:(k,h,m,w,r,C,G)=>{var N=console,U=N.log;const aa=a().buffer,fa=(new Uint32Array(aa,k-4,1))[0];k=(new TextDecoder("utf-16le")).decode(new Uint8Array(aa,k,fa));return U.call(N,"trace: "+k,...[m,w,r,C,G].slice(0,h))},"console.log":n(0),"console.debug":n(0),"console.info":n(0),"console.warn":n(0),"console.error":n(0),"console.time":n(1),"console.timeLog":n(2),"console.timeEnd":n(3)},rive_rt_v1:{log:(k,
h,m)=>{const w=p();let r=0;try{r=f(h,m),c.ei(k,r,m)}finally{b(r,m),q(w)}},mark_needs_update:c.fi,budget_exceeded:c.Zh,error:(k,h)=>{const m=p();let w=0;try{w=f(k,h),c.ci(w,h)}finally{b(w,h),q(m)}},now:()=>performance.now(),date_now:()=>Date.now(),debug_enter:c.$h,debug_line:c.bi,debug_leave:c.ai,utc_offset:c.gi,is_dst:c.di,zone_name:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=c.hi(k,r,m);g(r,h,m);return C}finally{b(r,m),q(w)}}},rive_data_v1:{view_model:c.pf,root_view_model:c.hf,global_view_model:(k,
h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Oe(k,r,m)}finally{b(r,m),q(w)}},global_view_model_names:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=c.Pe(k,r,m);g(r,h,m);return C}finally{b(r,m),q(w)}},context:c.De,context_parent:c.Ee,context_view_model:c.Ge,context_release:c.Fe,has_view_model:(k,h)=>{const m=p();let w=0;try{return w=f(k,h),c.Qe(w,h)}finally{b(w,h),q(m)}},new_view_model:(k,h,m,w)=>{const r=p();let C=0,G=0;try{return C=f(k,h),G=f(m,w),c.df(C,h,G,w)}finally{b(G,w),b(C,h),q(r)}},
vmi_release:c.Ef,vmi_number:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Cf(k,r,m)}finally{b(r,m),q(w)}},vmi_boolean:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.uf(k,r,m)}finally{b(r,m),q(w)}},vmi_string:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Ff(k,r,m)}finally{b(r,m),q(w)}},vmi_trigger:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Hf(k,r,m)}finally{b(r,m),q(w)}},vmi_color:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.vf(k,r,m)}finally{b(r,m),q(w)}},vmi_view_model:(k,
h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.If(k,r,m)}finally{b(r,m),q(w)}},vmi_property:(k,h,m,w,r)=>{const C=p();let G=0,N=0;try{G=f(h,m);N=f(w,4*r);const U=c.Df(k,G,m,N,r);g(N,w,4*r);return U}finally{b(N,4*r),b(G,m),q(C)}},vmi_instance:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Af(k,r,m)}finally{b(r,m),q(w)}},vmi_symbol_index:c.Gf,vmi_equal:c.xf,view_model_get:c.qf,vmi_list:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Bf(k,r,m)}finally{b(r,m),q(w)}},vmi_enum:(k,h,m)=>{const w=
p();let r=0;try{return r=f(h,m),c.wf(k,r,m)}finally{b(r,m),q(w)}},vmi_image:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.zf(k,r,m)}finally{b(r,m),q(w)}},vmi_font:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.yf(k,r,m)}finally{b(r,m),q(w)}},vmi_blob:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.tf(k,r,m)}finally{b(r,m),q(w)}},vmi_artboard:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.sf(k,r,m)}finally{b(r,m),q(w)}},image_get:c.Re,image_set:c.Se,font_get:c.Le,font_set:c.Ne,
font_release:c.Me,blob_present:c.xe,blob_get:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=c.ve(k,r,m);g(r,h,m);return C}finally{b(r,m),q(w)}},blob_name:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=c.we(k,r,m);g(r,h,m);return C}finally{b(r,m),q(w)}},blob_set:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m),c.ye(k,r,m)}finally{b(r,m),q(w)}},blob_clear:c.ue,artboard_get:c.se,artboard_set:c.te,enum_get:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=c.Ie(k,r,m);g(r,h,m);return C}finally{b(r,
m),q(w)}},enum_set:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m),c.Je(k,r,m)}finally{b(r,m),q(w)}},enum_values:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=c.Ke(k,r,m);g(r,h,m);return C}finally{b(r,m),q(w)}},prop_release:c.gf,trigger_fire:c.mf,list_length:c.We,list_push:c.Ye,list_pop:c.Xe,list_shift:c.bf,list_clear:c.Te,list_swap:c.cf,list_insert:c.Ve,list_remove:c.Ze,list_remove_at:c.af,list_remove_all_of:c.$e,list_get:c.Ue,view_model_set:c.rf,color_get:c.Be,color_set:c.Ce,number_get:c.ef,number_set:c.ff,
boolean_get:c.ze,boolean_set:c.Ae,string_get:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=c.kf(k,r,m);g(r,h,m);return C}finally{b(r,m),q(w)}},string_changed:c.jf,string_set:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m),c.lf(k,r,m)}finally{b(r,m),q(w)}},watch:c.Jf,unwatch:c.nf,convert_result:(k,h,m,w,r,C)=>{const G=p();let N=0;try{N=f(r,C),c.He(k,h,m,w,N,C)}finally{b(N,C),q(G)}}},rive_artboard_v1:{release:c.Bd,advance:c.Uc,draw:c.bd,instance:c.md,data:c.ad,width:c.Gd,height:c.ld,set_width:c.Fd,
set_height:c.Ed,frame_origin:c.jd,set_frame_origin:c.Dd,bounds:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m),c.$c(k,r,m),g(r,h,4*m)}finally{b(r,4*m),q(w)}},pointer_event:c.zd,scroll_event:c.Cd,gamepad_event:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.kd(k,r,m)}finally{b(r,m),q(w)}},animation:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Vc(k,r,m)}finally{b(r,m),q(w)}},animation_release:c.Yc,animation_duration:c.Xc,animation_advance:c.Wc,animation_set_time:c.Zc,add_to_path:c.Tc,node:(k,
h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.nd(k,r,m)}finally{b(r,m),q(w)}},node_release:c.ud,node_transform:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m),c.xd(k,r,m),g(r,h,4*m)}finally{b(r,4*m),q(w)}},node_set:c.vd,node_world_transform:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m),c.yd(k,r,m),g(r,h,4*m)}finally{b(r,4*m),q(w)}},node_set_world_transform:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m),c.wd(k,r,m)}finally{b(r,4*m),q(w)}},node_decompose:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m),c.pd(k,
r,m)}finally{b(r,4*m),q(w)}},node_path_verbs:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=c.td(k,r,m);g(r,h,m);return C}finally{b(r,m),q(w)}},node_path_points:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m);const C=c.sd(k,r,m);g(r,h,4*m);return C}finally{b(r,4*m),q(w)}},node_paint:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m);const C=c.qd(k,r,m);g(r,h,4*m);return C}finally{b(r,4*m),q(w)}},node_children:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m);const C=c.od(k,r,m);g(r,h,4*m);return C}finally{b(r,
4*m),q(w)}},node_parent:c.rd,property_key:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Ad(k,r,m)}finally{b(r,m),q(w)}},draw_visit:c.dd,draw_modulated:c.cd,drawable_draw:c.ed,drawable_value:(k,h,m,w)=>{const r=p();let C=0;try{C=f(m,4*w);const G=c.hd(k,h,C,w);g(C,m,4*w);return G}finally{b(C,4*w),q(r)}},drawable_string:(k,h,m,w)=>{const r=p();let C=0;try{C=f(m,w);const G=c.gd(k,h,C,w);g(C,m,w);return G}finally{b(C,w),q(r)}},drawable_properties:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=
c.fd(k,r,m);g(r,h,m);return C}finally{b(r,m),q(w)}}},rive_audio_v1:{source:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Zd(k,r,m)}finally{b(r,m),q(w)}},source_release:c.be,source_duration:c.ae,source_sample_rate:c.ce,source_channels:c.$d,play:c.Hd,play_at_time:c.Jd,play_in_time:c.Ld,play_at_frame:c.Id,play_in_frame:c.Kd,time:c.de,time_frame:c.ee,sample_rate:c.Md,sound_release:c.Qd,sound_play:c.Pd,sound_pause:c.Od,sound_resume:c.Rd,sound_stop:c.Vd,sound_seek:c.Sd,sound_seek_frame:c.Td,sound_completed:c.Nd,
sound_time:c.Wd,sound_time_frame:c.Xd,sound_volume:c.Yd,sound_set_volume:c.Ud},rive_path_v1:{"new":c.Kh,update:(k,h,m,w,r,C)=>{const G=p();let N=0,U=0;try{N=f(h,m),U=f(w,4*r),c.Nh(k,N,m,U,r,C)}finally{b(U,4*r),b(N,m),q(G)}},release:c.Mh,add:c.Ih,verbs:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=c.Oh(k,r,m);g(r,h,m);return C}finally{b(r,m),q(w)}},points:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m);const C=c.Lh(k,r,m);g(r,h,4*m);return C}finally{b(r,4*m),q(w)}},effect_result:(k,h,m,w)=>{const r=
p();let C=0,G=0;try{C=f(k,h),G=f(m,4*w),c.Jh(C,h,G,w)}finally{b(G,4*w),b(C,h),q(r)}}},rive_measure_v1:{path_new:(k,h,m,w)=>{const r=p();let C=0,G=0;try{return C=f(k,h),G=f(m,4*w),c.nh(C,h,G,w)}finally{b(G,4*w),b(C,h),q(r)}},contours_new:(k,h,m,w)=>{const r=p();let C=0,G=0;try{return C=f(k,h),G=f(m,4*w),c.ih(C,h,G,w)}finally{b(G,4*w),b(C,h),q(r)}},contour_next:c.hh,length:c.mh,is_closed:c.lh,pos_tan:(k,h,m,w)=>{const r=p();let C=0;try{C=f(m,4*w),c.oh(k,h,C,w),g(C,m,4*w)}finally{b(C,4*w),q(r)}},warp:(k,
h,m,w,r)=>{const C=p();let G=0;try{G=f(w,4*r),c.qh(k,h,m,G,r),g(G,w,4*r)}finally{b(G,4*r),q(C)}},extract:c.jh,extract_read:(k,h,m,w,r)=>{const C=p();let G=0,N=0;try{G=f(h,m);N=f(w,4*r);const U=c.kh(k,G,m,N,r);g(G,h,m);g(N,w,4*r);return U}finally{b(N,4*r),b(G,m),q(C)}},release:c.ph},rive_paint_v1:{"new":c.Ch,release:c.Dh,style:c.Gh,color:c.zh,thickness:c.Hh,join:c.Bh,cap:c.yh,blend_mode:c.xh,feather:c.Ah,shader:c.Eh,shader_transform:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m),c.Fh(k,r,m)}finally{b(r,
4*m),q(w)}}},rive_canvas_v1:{"new":c.ne,release:c.oe,width:c.qe,height:c.le,resize:c.pe,image:c.me,begin_frame:c.je,end_frame:c.ke},rive_gpu_v1:{features:(k,h)=>{const m=p();let w=0;try{w=f(k,4*h);const r=c.ug(w,h);g(w,k,4*h);return r}finally{b(w,4*h),q(m)}},canvas_new:c.rg,canvas_release:c.sg,canvas_color_view:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m);const C=c.pg(k,r,m);g(r,h,4*m);return C}finally{b(r,4*m),q(w)}},canvas_image:c.qg,canvas_resize:(k,h,m,w,r)=>{const C=p();let G=0;try{G=f(w,4*r);
const N=c.tg(k,h,m,G,r);g(G,w,4*r);return N}finally{b(G,4*r),q(C)}},target_view:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m);const C=c.Tg(k,r,m);g(r,h,4*m);return C}finally{b(r,4*m),q(w)}},pass_begin:(k,h,m,w)=>{const r=p();let C=0,G=0;try{return C=f(k,h),G=f(m,w),c.wg(C,h,G,w)}finally{b(G,w),b(C,h),q(r)}},pass_set_pipeline:c.Eg,pass_set_vertex_buffer:c.Hg,pass_set_index_buffer:c.Dg,pass_set_bind_group:(k,h,m,w,r)=>{const C=p();let G=0;try{G=f(w,r),c.Bg(k,h,m,G,r)}finally{b(G,r),q(C)}},pass_set_viewport:c.Ig,
pass_set_scissor:c.Fg,pass_set_stencil_reference:c.Gg,pass_set_blend_color:c.Cg,pass_draw:c.xg,pass_draw_indexed:c.yg,pass_finish:c.zg,pass_release:c.Ag,image_view:c.vg,buffer_new:(k,h,m,w,r)=>{const C=p();let G=0;try{return G=f(w,r),c.mg(k,h,m,G,r)}finally{b(G,r),q(C)}},buffer_update:(k,h,m,w)=>{const r=p();let C=0;try{C=f(m,w),c.og(k,h,C,w)}finally{b(C,w),q(r)}},buffer_release:c.ng,texture_new:(k,h)=>{const m=p();let w=0;try{return w=f(k,h),c.Ug(w,h)}finally{b(w,h),q(m)}},texture_upload:(k,h,m,
w,r)=>{const C=p();let G=0,N=0;try{G=f(h,m),N=f(w,r),c.Wg(k,G,m,N,r)}finally{b(N,r),b(G,m),q(C)}},texture_release:c.Vg,sampler_new:(k,h)=>{const m=p();let w=0;try{return w=f(k,h),c.Lg(w,h)}finally{b(w,h),q(m)}},sampler_release:c.Mg,texture_view_new:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Xg(k,r,m)}finally{b(r,m),q(w)}},texture_view_release:c.Yg,shader_target:c.Sg,shader_asset_bytes:(k,h,m,w,r)=>{const C=p();let G=0,N=0;try{G=f(h,m);N=f(w,r);const U=c.Ng(k,G,m,N,r);g(N,w,r);return U}finally{b(N,
r),b(G,m),q(C)}},shader_asset_id:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Og(k,r,m)}finally{b(r,m),q(w)}},shader_module_from_asset:(k,h,m,w)=>{const r=p();let C=0;try{return C=f(h,m),c.Pg(k,C,m,w)}finally{b(C,m),q(r)}},shader_module_new:(k,h,m,w)=>{const r=p();let C=0,G=0;try{return C=f(k,h),G=f(m,w),c.Qg(C,h,G,w)}finally{b(G,w),b(C,h),q(r)}},shader_module_release:c.Rg,bind_group_layout_new:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.ig(k,r,m)}finally{b(r,m),q(w)}},bind_group_layout_release:c.jg,
bind_group_layout_from_shader:(k,h,m,w)=>{const r=p();let C=0;try{return C=f(m,4*w),c.gg(k,h,C,w)}finally{b(C,4*w),q(r)}},bind_group_layout_from_shaders:(k,h,m,w,r)=>{const C=p();let G=0;try{return G=f(w,4*r),c.hg(k,h,m,G,r)}finally{b(G,4*r),q(C)}},bind_group_new:(k,h,m,w,r,C,G)=>{const N=p();let U=0,aa=0,fa=0;try{return U=f(h,m),aa=f(w,r),fa=f(C,G),c.kg(k,U,m,aa,r,fa,G)}finally{b(fa,G),b(aa,r),b(U,m),q(N)}},bind_group_release:c.lg,pipeline_new:(k,h,m,w)=>{const r=p();let C=0,G=0;try{return C=f(k,
h),G=f(m,w),c.Jg(C,h,G,w)}finally{b(G,w),b(C,h),q(r)}},pipeline_release:c.Kg},rive_mat4_v1:{multiply:(k,h,m,w,r,C)=>{const G=p();let N=0,U=0,aa=0;try{N=f(k,h),U=f(m,w),aa=f(r,C),c.gh(N,h,U,w,aa,C),g(N,k,h)}finally{b(aa,C),b(U,w),b(N,h),q(G)}},invert:(k,h,m,w)=>{const r=p();let C=0,G=0;try{C=f(k,h);G=f(m,w);const N=c.fh(C,h,G,w);g(C,k,h);return N}finally{b(G,w),b(C,h),q(r)}}},rive_buffer_v1:{"new":c.ge,update:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m),c.ie(k,r,m)}finally{b(r,m),q(w)}},release:c.he},
rive_mesh_instances_v1:{"new":c.rh,resize:c.th,update:(k,h,m,w)=>{const r=p();let C=0;try{C=f(m,w),c.uh(k,h,C,w)}finally{b(C,w),q(r)}},release:c.sh},rive_blob_v1:{asset_bytes:(k,h,m,w,r)=>{const C=p();let G=0,N=0;try{G=f(h,m);N=f(w,r);const U=c.fe(k,G,m,N,r);g(N,w,r);return U}finally{b(N,r),b(G,m),q(C)}}},rive_image_v1:{from_asset:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.ah(k,r,m)}finally{b(r,m),q(w)}},width:c.eh,height:c.bh,release:c.dh,decode:(k,h,m)=>{const w=p();let r=0;try{return r=
f(k,h),c.Zg(r,h,m)}finally{b(r,h),q(w)}},decode_cancel:c.$g},rive_font_v1:{from_asset:(k,h,m)=>{const w=p();let r=0;try{return r=f(h,m),c.Yf(k,r,m)}finally{b(r,m),q(w)}},decode:(k,h)=>{const m=p();let w=0;try{return w=f(k,h),c.Wf(w,h)}finally{b(w,h),q(m)}},release:c.dg,metrics:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m),c.cg(k,r,m),g(r,h,4*m)}finally{b(r,4*m),q(w)}},weight:c.eg,is_italic:c.bg,axis_count:c.Uf,axis:(k,h,m,w)=>{const r=p();let C=0;try{C=f(m,4*w),c.Tf(k,h,C,w),g(C,m,4*w)}finally{b(C,
4*w),q(r)}},axis_value:c.Vf,features:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m);const C=c.Xf(k,r,m);g(r,h,4*m);return C}finally{b(r,4*m),q(w)}},has_glyph:c.ag,with_options:(k,h,m,w,r)=>{const C=p();let G=0,N=0;try{return G=f(h,4*m),N=f(w,4*r),c.fg(k,G,m,N,r)}finally{b(N,4*r),b(G,4*m),q(C)}},glyph_verbs:(k,h,m,w)=>{const r=p();let C=0;try{C=f(m,w);const G=c.$f(k,h,C,w);g(C,m,w);return G}finally{b(C,w),q(r)}},glyph_points:(k,h,m,w)=>{const r=p();let C=0;try{C=f(m,4*w);const G=c.Zf(k,h,C,w);g(C,m,
4*w);return G}finally{b(C,4*w),q(r)}}},rive_text_v1:{"new":c.yi,release:c.zi,append:(k,h,m,w,r,C,G,N,U)=>{const aa=p();let fa=0;try{fa=f(h,m),c.ni(k,fa,m,w,r,C,G,N,U)}finally{b(fa,m),q(aa)}},clear:c.ri,layout:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m),c.vi(k,r,m)}finally{b(r,m),q(w)}},draw:c.si,bounds:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m),c.oi(k,r,m),g(r,h,4*m)}finally{b(r,4*m),q(w)}},length:c.wi,lines:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m);const C=c.xi(k,r,m);g(r,h,4*m);return C}finally{b(r,
4*m),q(w)}},runs:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m);const C=c.Ai(k,r,m);g(r,h,4*m);return C}finally{b(r,4*m),q(w)}},glyphs:(k,h,m)=>{const w=p();let r=0;try{r=f(h,4*m);const C=c.ti(k,r,m);g(r,h,4*m);return C}finally{b(r,4*m),q(w)}},hit_test:c.ui,caret:(k,h,m,w)=>{const r=p();let C=0;try{C=f(m,4*w);const G=c.pi(k,h,C,w);g(C,m,4*w);return G}finally{b(C,4*w),q(r)}},selection_rects:(k,h,m,w,r)=>{const C=p();let G=0;try{G=f(w,4*r);const N=c.Bi(k,h,m,G,r);g(G,w,4*r);return N}finally{b(G,4*r),
q(C)}}},rive_shader_v1:{linear:c.ii,radial:c.ki,release:c.li},rive_test_v1:{blob:(k,h,m,w)=>{const r=p();let C=0,G=0;try{C=f(k,h);G=f(m,w);const N=c.mi(C,h,G,w);g(G,m,w);return N}finally{b(G,w),b(C,h),q(r)}}},rive_transition_v1:{child_draw:c.Ci,child_width:c.Ei,child_height:c.Di},rive_renderer_v1:{save:c.Xh,restore:c.Wh,transform:c.Yh,draw_path:c.Th,clip_path:c.Ph,modulate_opacity:c.Vh,modulate_color:c.Uh,draw_image:c.Qh,draw_image_mesh:c.Rh,draw_image_mesh_instanced:c.Sh},rive_net_v1:{fetch:(k,h,
m)=>{const w=p();let r=0;try{return r=f(k,h),c.vh(r,h,m)}finally{b(r,h),q(w)}},fetch_cancel:c.wh},rive_file_v1:{decode:(k,h,m,w)=>{const r=p();let C=0,G=0;try{C=f(k,h);G=f(m,4*w);const N=c.Rf(C,h,G,w);g(G,m,4*w);return N}finally{b(G,4*w),b(C,h),q(r)}},release:c.Sf,artboard_count:c.Kf,artboard_name:(k,h,m,w)=>{const r=p();let C=0;try{C=f(m,w);const G=c.Lf(k,h,C,w);g(C,m,w);return G}finally{b(C,w),q(r)}},bindable:(k,h,m,w)=>{const r=p();let C=0;try{return C=f(h,m),c.Mf(k,C,m,w)}finally{b(C,m),q(r)}},
bindable_release:c.Qf,bindable_name:(k,h,m)=>{const w=p();let r=0;try{r=f(h,m);const C=c.Pf(k,r,m);g(r,h,m);return C}finally{b(r,m),q(w)}},bindable_data:c.Nf,bindable_equal:c.Of}}}function e(u,a){const c=a.ab??{i:!1},p=()=>{const f=Error(a.j);a.j=null;throw f;},q=()=>{throw Error("librive faulted in a nested script call");},v=()=>{throw Error("script VM was released");};return{ei:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(0)),fi:(f=>
b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(1)),Zh:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(2)),ci:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(3)),$h:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(4)),bi:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=
n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(5)),ai:(f=>()=>{0===a.g&&v();c.i&&c.l++;try{f(a.g)}catch(g){var b=g;a.h=!0;throw b;}a.h&&q();null!==a.j&&p()})(u(6)),gi:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(7)),di:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(8)),hi:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=
h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(9)),pf:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(10)),hf:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(11)),Oe:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(12)),Pe:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=
f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(13)),De:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(14)),Ee:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(15)),Ge:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(16)),Fe:(f=>b=>{0===a.g&&v();c.i&&c.l++;
try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(17)),Qe:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(18)),df:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(19)),Ef:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(20)),Cf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;
let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(21)),uf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(22)),Ff:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(23)),Hf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(24)),
vf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(25)),If:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(26)),Df:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;let m;try{m=f(a.g,b,g,n,k,h)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p();return m})(u(27)),Af:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=
h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(28)),Gf:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(29)),xf:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(30)),qf:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(31)),Bf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,
b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(32)),wf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(33)),zf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(34)),yf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(35)),tf:(f=>(b,
g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(36)),sf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(37)),Re:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(38)),Se:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(39)),
Le:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(40)),Ne:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(41)),Me:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(42)),xe:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(43)),ve:(f=>(b,g,n)=>{0===
a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(44)),we:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(45)),ye:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(46)),ue:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(47)),se:(f=>b=>{0===
a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(48)),te:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(49)),Ie:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(50)),Je:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(51)),Ke:(f=>(b,g,
n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(52)),gf:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(53)),mf:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(54)),We:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(55)),Ye:(f=>(b,g)=>{0===a.g&&v();
c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(56)),Xe:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(57)),bf:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(58)),Te:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(59)),cf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,
b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(60)),Ve:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(61)),Ze:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(62)),af:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(63)),$e:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=
!0,b;}a.h&&q();null!==a.j&&p()})(u(64)),Ue:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(65)),rf:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(66)),Be:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(67)),Ce:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;
}a.h&&q();null!==a.j&&p()})(u(68)),ef:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(69)),ff:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(70)),ze:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(71)),Ae:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==
a.j&&p()})(u(72)),kf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(73)),jf:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(74)),lf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(75)),Jf:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==
a.j&&p()})(u(76)),nf:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(77)),He:(f=>(b,g,n,k,h,m)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(78)),Bd:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(79)),Uc:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(80)),
bd:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(81)),md:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(82)),ad:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(83)),Gd:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(84)),
ld:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(85)),Fd:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(86)),Ed:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(87)),jd:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(88)),Dd:(f=>(b,g)=>
{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(89)),$c:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(90)),zd:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;let m;try{m=f(a.g,b,g,n,k,h)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p();return m})(u(91)),Cd:(f=>(b,g,n,k,h,m,w,r,C)=>{0===a.g&&v();c.i&&c.l++;let G;try{G=f(a.g,b,g,n,k,h,m,w,r,C)}catch(N){throw b=N,a.h=!0,b;}a.h&&q();null!==
a.j&&p();return G})(u(92)),kd:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(93)),Vc:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(94)),Yc:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(95)),Xc:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&
q();null!==a.j&&p();return g})(u(96)),Wc:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(97)),Zc:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(98)),Tc:(f=>(b,g,n,k,h,m,w,r)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m,w,r)}catch(C){throw b=C,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(99)),nd:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=
h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(100)),ud:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(101)),xd:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(102)),vd:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(103)),yd:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;
}a.h&&q();null!==a.j&&p()})(u(104)),wd:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(105)),pd:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(106)),td:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(107)),sd:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=
h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(108)),qd:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(109)),od:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(110)),rd:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(111)),Ad:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;
let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(112)),dd:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(113)),cd:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(114)),ed:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(115)),hd:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;
try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(116)),gd:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(117)),fd:(f=>void 0===f?l("rive_artboard_v1","drawable_properties"):(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(118)),Zd:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=
h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(119)),be:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(120)),ae:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(121)),ce:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(122)),$d:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,
a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(123)),Hd:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(124)),Jd:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(125)),Ld:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(126)),Id:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=
f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(127)),Kd:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(128)),de:(f=>()=>{0===a.g&&v();c.i&&c.l++;try{var b=f(a.g)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p();return b})(u(129)),ee:(f=>()=>{0===a.g&&v();c.i&&c.l++;try{var b=f(a.g)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p();return b})(u(130)),Md:(f=>()=>{0===a.g&&v();c.i&&
c.l++;try{var b=f(a.g)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p();return b})(u(131)),Qd:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(132)),Pd:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(133)),Od:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(134)),Rd:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=
!0,b;}a.h&&q();null!==a.j&&p()})(u(135)),Vd:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(136)),Sd:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(137)),Td:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(138)),Nd:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=
n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(139)),Wd:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(140)),Xd:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(141)),Yd:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(142)),Ud:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=
n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(143)),Kh:(f=>()=>{0===a.g&&v();c.i&&c.l++;try{var b=f(a.g)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p();return b})(u(144)),Nh:(f=>(b,g,n,k,h,m)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(145)),Mh:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(146)),Ih:(f=>(b,g,n,k,h,m,w,r)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m,w,r)}catch(C){throw b=
C,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(147)),Oh:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(148)),Lh:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(149)),Jh:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(150)),nh:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=
f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(151)),ih:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(152)),hh:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(153)),mh:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(154)),lh:(f=>b=>{0===
a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(155)),oh:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(156)),qh:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(157)),jh:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(158)),
kh:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;let m;try{m=f(a.g,b,g,n,k,h)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p();return m})(u(159)),ph:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(160)),Ch:(f=>()=>{0===a.g&&v();c.i&&c.l++;try{var b=f(a.g)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p();return b})(u(161)),Dh:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(162)),Gh:(f=>
(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(163)),zh:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(164)),Hh:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(165)),Bh:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(166)),yh:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;
try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(167)),xh:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(168)),Ah:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(169)),Eh:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(170)),Fh:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=
k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(171)),ne:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(172)),oe:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(173)),qe:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(174)),le:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=
!0,b;}a.h&&q();null!==a.j&&p();return g})(u(175)),pe:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(176)),me:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(177)),je:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(178)),ke:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=
g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(179)),ug:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(180)),rg:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(181)),sg:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(182)),pg:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=
h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(183)),qg:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(184)),tg:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;let m;try{m=f(a.g,b,g,n,k,h)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p();return m})(u(185)),Tg:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(186)),wg:(f=>(b,g,n,k)=>{0===a.g&&v();
c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(187)),Eg:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(188)),Hg:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(189)),Dg:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(190)),Bg:(f=>(b,g,n,k,h)=>
{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(191)),Ig:(f=>(b,g,n,k,h,m,w)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m,w)}catch(r){throw b=r,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(192)),Fg:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(193)),Gg:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(194)),Cg:(f=>
(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(195)),xg:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(196)),yg:(f=>(b,g,n,k,h,m)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(197)),zg:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(198)),Ag:(f=>
b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(199)),vg:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(200)),mg:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;let m;try{m=f(a.g,b,g,n,k,h)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p();return m})(u(201)),og:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&
p()})(u(202)),ng:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(203)),Ug:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(204)),Wg:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(205)),Vg:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(206)),
Lg:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(207)),Mg:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(208)),Xg:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(209)),Yg:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(210)),Sg:(f=>
()=>{0===a.g&&v();c.i&&c.l++;try{var b=f(a.g)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p();return b})(u(211)),Ng:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;let m;try{m=f(a.g,b,g,n,k,h)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p();return m})(u(212)),Og:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(213)),Pg:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,
b;}a.h&&q();null!==a.j&&p();return h})(u(214)),Qg:(f=>void 0===f?l("rive_gpu_v1","shader_module_new"):(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(215)),Rg:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(216)),ig:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(217)),jg:(f=>b=>{0===
a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(218)),gg:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(219)),hg:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;let m;try{m=f(a.g,b,g,n,k,h)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p();return m})(u(220)),kg:(f=>(b,g,n,k,h,m,w)=>{0===a.g&&v();c.i&&c.l++;let r;try{r=f(a.g,b,g,n,k,h,m,w)}catch(C){throw b=C,a.h=!0,b;}a.h&&
q();null!==a.j&&p();return r})(u(221)),lg:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(222)),Jg:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(223)),Kg:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(224)),gh:(f=>(b,g,n,k,h,m)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m)}catch(w){throw b=w,a.h=!0,
b;}a.h&&q();null!==a.j&&p()})(u(225)),fh:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(226)),ge:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(227)),ie:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(228)),he:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=
g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(229)),rh:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(230)),th:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(231)),uh:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(232)),sh:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&
q();null!==a.j&&p()})(u(233)),fe:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;let m;try{m=f(a.g,b,g,n,k,h)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p();return m})(u(234)),ah:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(235)),eh:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(236)),bh:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=
n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(237)),dh:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(238)),Zg:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(239)),$g:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(240)),Yf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=
h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(241)),Wf:(f=>void 0===f?l("rive_font_v1","decode"):(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(242)),dg:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(243)),cg:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(244)),eg:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;
try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(245)),bg:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(246)),Uf:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(247)),Tf:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(248)),Vf:(f=>(b,g)=>{0===a.g&&v();c.i&&
c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(249)),Xf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(250)),ag:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(251)),fg:(f=>(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;let m;try{m=f(a.g,b,g,n,k,h)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p();
return m})(u(252)),$f:(f=>void 0===f?l("rive_font_v1","glyph_verbs"):(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(253)),Zf:(f=>void 0===f?l("rive_font_v1","glyph_points"):(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(254)),yi:(f=>void 0===f?l("rive_text_v1","new"):()=>{0===a.g&&v();c.i&&c.l++;try{var b=f(a.g)}catch(g){throw b=g,a.h=
!0,b;}a.h&&q();null!==a.j&&p();return b})(u(255)),zi:(f=>void 0===f?l("rive_text_v1","release"):b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(256)),ni:(f=>void 0===f?l("rive_text_v1","append"):(b,g,n,k,h,m,w,r,C)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m,w,r,C)}catch(G){throw b=G,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(257)),ri:(f=>void 0===f?l("rive_text_v1","clear"):b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&
q();null!==a.j&&p()})(u(258)),vi:(f=>void 0===f?l("rive_text_v1","layout"):(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(259)),si:(f=>void 0===f?l("rive_text_v1","draw"):(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(260)),oi:(f=>void 0===f?l("rive_text_v1","bounds"):(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(261)),
wi:(f=>void 0===f?l("rive_text_v1","length"):b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(262)),xi:(f=>void 0===f?l("rive_text_v1","lines"):(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(263)),Ai:(f=>void 0===f?l("rive_text_v1","runs"):(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(264)),
ti:(f=>void 0===f?l("rive_text_v1","glyphs"):(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(265)),ui:(f=>void 0===f?l("rive_text_v1","hit_test"):(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(266)),pi:(f=>void 0===f?l("rive_text_v1","caret"):(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();
null!==a.j&&p();return h})(u(267)),Bi:(f=>void 0===f?l("rive_text_v1","selection_rects"):(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;let m;try{m=f(a.g,b,g,n,k,h)}catch(w){throw b=w,a.h=!0,b;}a.h&&q();null!==a.j&&p();return m})(u(268)),ii:(f=>(b,g,n,k,h,m,w)=>{0===a.g&&v();c.i&&c.l++;let r;try{r=f(a.g,b,g,n,k,h,m,w)}catch(C){throw b=C,a.h=!0,b;}a.h&&q();null!==a.j&&p();return r})(u(269)),ki:(f=>(b,g,n,k,h,m)=>{0===a.g&&v();c.i&&c.l++;let w;try{w=f(a.g,b,g,n,k,h,m)}catch(r){throw b=r,a.h=!0,b;}a.h&&q();null!==
a.j&&p();return w})(u(270)),li:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(271)),mi:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(272)),Ci:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(273)),Ei:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==
a.j&&p();return g})(u(274)),Di:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(275)),Xh:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(276)),Wh:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(277)),Yh:(f=>(b,g,n,k,h,m,w)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m,w)}catch(r){throw b=r,a.h=!0,b;}a.h&&q();null!==a.j&&
p()})(u(278)),Th:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(279)),Ph:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(280)),Vh:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(281)),Uh:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(282)),Qh:(f=>
(b,g,n,k,h)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(283)),Rh:(f=>(b,g,n,k,h,m,w,r)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m,w,r)}catch(C){throw b=C,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(284)),Sh:(f=>(b,g,n,k,h,m,w)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n,k,h,m,w)}catch(r){throw b=r,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(285)),vh:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=h,a.h=!0,b;}a.h&&
q();null!==a.j&&p();return k})(u(286)),wh:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(287)),Rf:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(288)),Sf:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(289)),Kf:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==
a.j&&p();return g})(u(290)),Lf:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(291)),Mf:(f=>(b,g,n,k)=>{0===a.g&&v();c.i&&c.l++;let h;try{h=f(a.g,b,g,n,k)}catch(m){throw b=m,a.h=!0,b;}a.h&&q();null!==a.j&&p();return h})(u(292)),Qf:(f=>b=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b)}catch(g){throw b=g,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(293)),Pf:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;let k;try{k=f(a.g,b,g,n)}catch(h){throw b=
h,a.h=!0,b;}a.h&&q();null!==a.j&&p();return k})(u(294)),Nf:(f=>b=>{0===a.g&&v();c.i&&c.l++;let g;try{g=f(a.g,b)}catch(n){throw b=n,a.h=!0,b;}a.h&&q();null!==a.j&&p();return g})(u(295)),Of:(f=>(b,g)=>{0===a.g&&v();c.i&&c.l++;let n;try{n=f(a.g,b,g)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p();return n})(u(296)),re:(f=>(b,g,n)=>{0===a.g&&v();c.i&&c.l++;try{f(a.g,b,g,n)}catch(k){throw b=k,a.h=!0,b;}a.h&&q();null!==a.j&&p()})(u(297))}}function l(u,a){return()=>{throw Error("failed to call unlinked import function "+
u+"."+a);}}function t(u){u=[0,...u,11];return new Uint8Array([0,97,115,109,1,0,0,0,1,5,1,96,0,1,123,3,2,1,0,10,u.length+2,1,u.length,...u])}function x(u,a){let c=0,p=0,q;do q=u[a++],c|=(q&127)<<p,p+=7;while(q&128);return[c>>>0,a]}function z(u){const a=[];do{const c=u&127;u>>>=7;a.push(u?c|128:c)}while(u);return a}function y(u){var a=[],c=-1;for(var p=8;p<u.length;){const [q,v]=x(u,p+1);8===u[p]?c=x(u,v)[0]:a.push({id:u[p],at:p,body:v,end:v+q});p=v+q}if(0>c||!a.some(q=>7===q.id))return u;p=[u.subarray(0,
8)];for(const q of a){if(7!==q.id){p.push(u.subarray(q.at,q.end));continue}const [v,f]=x(u,q.body);a=Array.from("__riveLiftedStart",n=>n.charCodeAt(0));a=[...z(a.length),...a,0,...z(c)];const b=z(v+1),g=u.subarray(f,q.end);p.push(Uint8Array.from([7,...z(b.length+g.length+a.length),...b]),g,Uint8Array.from(a))}u=new Uint8Array(p.reduce((q,v)=>q+v.length,0));c=0;for(const q of p)u.set(q,c),c+=q.length;return u}const B=[253,12,...Array(16).fill(0)],F=t([...B]),I=t([...B,...B,253,128,2]);(function(u){function a(h,
m){const w=d({kc:e(v.Fc,h.U),ia:v.ia,ra:v.ra,Na:v.Na,qa:v.qa,gb:v.gb,cb:v.cb,ab:b},()=>{if(null===h.memory)throw Error("script passed memory to the host while its module was still starting");return h.memory});for(const r of WebAssembly.Module.imports(m))if("function"===r.kind){let C,G,N,U;(N=(C=w)[G=r.module]??(C[G]={}))[U=r.name]??(N[U]=l(r.module,r.name))}return w}function c(h,m){const w=v.ra();for(const {name:r}of WebAssembly.Module.exports(m)){if(!r.startsWith("__rive"))continue;m=3*r.length+
1;const C=v.Na(m);v.Wb(r,C,m);v.exports._rive_web_vm_module_export(h.U.g,C)}v.qa(w)}function p(h,m){h.memory=m.exports.memory;h.$a=null;const w=v.exports._rive_web_vm_booting;w(h.U.g);const r=v.ra();try{m.exports.__riveLiftedStart?.()}catch(C){throw v.qa(r),h.U.j=null,C;}finally{w(0)}h.instance=m}async function q(h){try{const w=await WebAssembly.compile(y(h.$a));if(0!==h.U.g){c(h,w);var m=await WebAssembly.instantiate(w,a(h,w));0!==h.U.g&&p(h,m)}}catch(w){h.error=String(w?.message??w)}}let v=null;
const f=new Map,b={i:!1,l:0,Aa:0,ya:0,za:0,Ba:0},g=u.riveScriptingValidate??(h=>WebAssembly.validate(h));let n=null;const k={ic(h){v??=h},register(h,m){m=m.slice();var w=f.set;n??={Ob:g(F),Rc:g(I)};var r=n.Ob&&n.Rc||g(m)?null:n.Ob?"this browser lacks WebAssembly relaxed SIMD, so this file's scripts are off; bake with wasmSimd: on to support it":"this browser lacks WebAssembly SIMD, so this file's scripts are off; bake with wasmSimd: off to support it";w.call(f,h,{U:{g:h,j:null,h:!1,ab:b},$a:m,instance:null,
memory:null,ba:null,error:r,names:new Map})},release(h){const m=f.get(h);m&&(m.U.g=0,f.delete(h))},Oc(){const h=[];for(const m of f.values())if(null===m.instance&&null===m.error){let w;(w=m).ba??(w.ba=q(m));h.push(m.ba)}return Promise.all(h)},start(h){h=f.get(h);if(!h)return"script module was never registered";if(null===h.instance&&null===h.error){if(null!==h.ba)return"script module is still being prepared";try{const m=new WebAssembly.Module(y(h.$a));c(h,m);p(h,new WebAssembly.Instance(m,a(h,m)))}catch(m){h.error=
String(m?.message??m)}}return h.error},ba(h){h=f.get(h);return null===h?.instance&&null===h.error&&null!==h.ba?1:0},Dc(h,m){return"function"===typeof f.get(h)?.instance?.exports[v.nb(m)]?1:0},call(h,m,w,r,C,G,N){h=f.get(h);if(!h||null===h.instance)return 1;let U=h.names.get(m);void 0===U&&(U=h.instance.exports[v.nb(m)]??null,h.names.set(m,U));if("function"!==typeof U)return 1;try{m=r>>3;const aa=U(...v.Eb().subarray(m,m+w));v.Eb()[C>>3]="number"===typeof aa?aa:0;return 0}catch(aa){return h.U.j=null,
v.Wb(String(aa?.message??aa),G,N),h.U.h?3:2}},Pc(h,m){if(h=f.get(h))h.U.j=v.nb(m)},read(h,m,w,r){h=f.get(h)?.memory;if(!h||m+r>h.buffer.byteLength)return 0;v.ia().set(new Uint8Array(h.buffer,m,r),w);b.i&&(b.za+=r);return 1},write(h,m,w,r){h=f.get(h)?.memory;if(!h||m+r>h.buffer.byteLength)return 0;(new Uint8Array(h.buffer,m,r)).set(v.ia().subarray(w,w+r));b.i&&(b.Ba+=r);return 1},Ji(h,m){h=f.get(h)?.memory;if(!h||m>=h.buffer.byteLength)return-1;h=(new Uint8Array(h.buffer)).indexOf(0,m);return 0>h?
-1:h-m},Mc(h){return(h=f.get(h)?.memory)?h.buffer.byteLength>>>16:0},oc(h){b.i=!!h;b.l=0;b.Aa=0;b.ya=0;b.za=0;b.Ba=0},Gc(){return{hostCalls:b.l,bytesStaged:b.Aa,bytesCopiedOut:b.ya,bytesRead:b.za,bytesWritten:b.Ba}}};k.prepare=k.Oc;k.countHostCalls=k.oc;k.hostCallCounts=k.Gc;return u.riveScripting=k})(D)})();function ja(){}const H=new Int32Array(5);
function ka(d,e,l,t,x){d(e,l,H);d=Math.max(H[0],0);const z=Math.max(H[1],0);e=Math.min(H[2],Math.ceil(t*e));l=Math.min(H[3],Math.ceil(x*l));H[4]=d!==H[0]||z!==H[1]||e!==H[2]||l!==H[3]?1:0;H[0]=d;H[1]=z;H[2]=e;H[3]=l;return e>d&&l>z}function la(){var d=(d=na())?d.dc:0;return 0===d?0:d}
function oa(d,e,l,t,x,z,y,B,F,I,u){var a=la(),c=a-1,p=c-6;if(!(0<p))return I(),ja;let q=1,v=1;if(!ka(B,q,v,l,t))return I(),ja;const f=H[2]-H[0],b=H[3]-H[1];if(f>c||b>c)if(q*=Math.min(1,p/f),v*=Math.min(1,p/b),!ka(B,q,v,l,t))return I(),ja;l=H[0];t=H[1];c=H[2];p=H[3];B=0!==H[4];if(1>q||1>v)--l,--t,c+=1,p+=1;const g=c-l,n=p-t;d.s||(d.s=new D.DynamicRectanizer(a),d.s.reset(512,512));a=d.s.addRect(g+1,n+1);if(0>a&&(u(),a=d.s.addRect(g+1,n+1),0>a))return console.assert(!1,"draw does not fit in an empty atlas"),
I(),ja;const k=a&65535,h=a>>16;d.A.push({hc:[q*x[0],v*x[1],q*x[2],v*x[3],q*x[4]-l+k,v*x[5]-t+h],vb:k,wb:h,Mi:g,Ec:n,Hb:B,nc:F,Sc:()=>{I()}});const m=l/q,w=t/v,r=g/q,C=n/v,G=d.T;return function(){const N=G.canvas();N&&(e.save(),e.resetTransform(),e.globalCompositeOperation=z,e.globalAlpha=y,e.drawImage(N,k,h,g,n,m,w,r,C),e.restore())}}class pa{constructor(){this.T=new qa;this.s=null;this.A=[];this.Ha=new ha;this.W=new ha}}let ra=null,sa=!1,ta=!1,ua=null;
function va(d){const e=document.createElement("canvas");e.width=1;e.height=1;var l={alpha:!0,depth:d,stencil:d,antialias:d,premultipliedAlpha:!0,preserveDrawingBuffer:0,powerPreference:"high-performance",failIfMajorPerformanceCaveat:0,enableExtensionsByDefault:!1,explicitSwapControl:0,renderViaOffscreenBackBuffer:0};d=e.getContext("webgl2",l);if(!d)return null;e.addEventListener("webglcontextlost",function(){ta||(ta=!0,console.error("WebGL context lost. Draws that rely on the WebGL2 renderer will be dropped."))});
l=wa(d,l);xa(l);const t=D.makeWebGL2Renderer(e.width,e.height);if(!t)return ya(l),null;t.Ta=l;t.la=e;t.ac=d;t.ub=e.width;t.rb=e.height;t.bc=!!d.getExtension("WEBGL_shader_pixel_local_storage");t.dc=Math.min(d.getParameter(d.MAX_RENDERBUFFER_SIZE),d.getParameter(d.MAX_TEXTURE_SIZE));return t}
function na(){if(ta)return null;if(sa)return ra;sa=!0;var d=va(!0);if(!d)return console.log("No WebGL2 support. Draws that Canvas2D can't make will be dropped."),null;if(d.bc)var e=!0;else{e=d.ac;var l=e.getExtension("WEBGL_debug_renderer_info");if(l){var t=e.getParameter(l.UNMASKED_RENDERER_WEBGL);e=e.getParameter(l.UNMASKED_VENDOR_WEBGL).includes("Google")&&t.includes("ANGLE Metal Renderer")}else e=!1}return e&&(xa(d.Ta),d["delete"](),ya(d.Ta),d=va(!1),!d)?null:ra=d}
class za{constructor(d){this.ca=d;this.path=D.makeWebGL2Path();this.path.fillRule(d)}moveTo(d,e){this.path.moveTo(d,e)}lineTo(d,e){this.path.lineTo(d,e)}xb(d,e,l,t,x,z){this.path.cubicTo(d,e,l,t,x,z)}close(){this.path.close()}addPath(d,e,l,t,x,z,y){const B=new za(this.ca);for(const F of d)F(B);B.Da();this.path.addPath(B.path,e,l,t,x,z,y);B.dispose()}Da(){}Vb(){return!1}dispose(){this.path.unref();this.path=null}}class qa{constructor(){this.Pb=!0}canvas(){return ta||!ra?null:ra.la}}const Ba=new pa;
function Ca(d,e,l,t,x,z){const y=new DOMMatrix;y.a=d;y.b=e;y.c=l;y.d=t;y.e=x;y.f=z;return y}const Da="createConicGradient createImageData createLinearGradient createPattern createRadialGradient getContextAttributes getImageData getLineDash getTransform isContextLost isPointInPath isPointInStroke measureText".split(" "),Ea=D.onRuntimeInitialized;
D.onRuntimeInitialized=function(){function d(A){switch(A){case p.srcOver:return"source-over";case p.Ni:return"plus-lighter";case p.screen:return"screen";case p.overlay:return"overlay";case p.darken:return"darken";case p.lighten:return"lighten";case p.colorDodge:return"color-dodge";case p.colorBurn:return"color-burn";case p.hardLight:return"hard-light";case p.softLight:return"soft-light";case p.difference:return"difference";case p.exclusion:return"exclusion";case p.multiply:return"multiply";case p.hue:return"hue";
case p.saturation:return"saturation";case p.color:return"color";case p.luminosity:return"luminosity"}}function e(A){return"rgba("+((16711680&A)>>>16)+","+((65280&A)>>>8)+","+((255&A)>>>0)+","+((4278190080&A)>>>24)/255+")"}function l(A,E,L,J,K){return{Qb:A,Rb:E,Ab:L,Bb:J,Fb:K,lb:[],ha:null}}function t(A,E,L,J){if(E.fa===v&&!(0<E.ga))return null;L=1===L;return{ib:A.ob(L?E.Va:null),gradientTransform:L?E.ma:null,Ki:L?E.ga*E.Ya:E.ga,fillRule:A.ca===f?"evenodd":"nonzero",opacity:J,style:E.fa,color:E.Ra,
Db:E.R,lc:E.Qa,join:E.Wa,jc:E.Pa}}function x(A,E){return E.fa!==v||0<E.ga?{path:A.Yb(A.ca),hb:E.Zb()}:null}function z(){if(0!==g.A.length){var A=la();if(0!==A){var E=Math.min(g.Ha.push(g.s.drawWidth()),A),L=Math.min(g.W.push(g.s.drawHeight()),A),J=g.A;if(A=na()){xa(A.Ta);const K=A.la;if(K.width!==E||K.height!==L)K.width=E,K.height=L;if(A.ub!==E||A.rb!==L)A.resize(E,L),A.ub=E,A.rb=L;A.clear();ua||=new D.Mat2D;E=ua;for(const M of J)A.save(),M.Hb&&A.saveClipRect(M.vb,M.wb,M.vb+M.Mi,M.wb+M.Ec),J=M.hc,
E.xx=J[0],E.xy=J[1],E.yx=J[2],E.yy=J[3],E.tx=J[4],E.ty=J[5],A.transform(E),M.nc(A),M.Hb&&A.restoreClipRect(),A.restore();A.flush()}}}for(const K of G){for(const M of K.L)M();K.L=[]}G.clear();for(const K of g.A)K.Sc();g.A=[];g.s&&g.s.reset(512,512)}Ea&&Ea();var y=D.RenderPaintStyle;const B=D.FillRule,F=D.RenderPath,I=D.RenderPaint,u=D.Renderer,a=D.StrokeCap,c=D.StrokeJoin,p=D.BlendMode,q=y.fill,v=y.stroke,f=B.evenOdd,b=B.nonZero,g=Ba,n=g.T;var k=D.RenderImage.extend("CanvasRenderImage",{__construct:function({pa:A,
La:E}={}){this.__parent.__construct.call(this);this.pa=A;this.La=E;this.ec=this.na=this.va=this.sb=null},__destruct:function(){this.na&&(this.na.unref(),this.na=null);this.va&&URL.revokeObjectURL(this.va);this.__parent.__destruct.call(this)},decode:function(A){var E=this;E.La&&E.La(E);var L=new Image;E.va=URL.createObjectURL(new Blob([A],{type:"image/png"}));L.onload=function(){E.sb=L;E.na=D.adoptWebGL2Image(L,L.width,L.height);E.size(L.width,L.height);E.pa&&E.pa(E)};L.src=E.va}});class h{constructor(A){this.s=
A;this.path=new Path2D;this.A=null;this.W=!1}T(){null!==this.A&&(this.path.lineTo(this.A.x,this.A.y),this.A=null)}moveTo(A,E){this.T();let L=A,J=E;null!==this.s&&(A=this.s.transformPoint({x:A,y:E}),L=A.x,J=A.y);this.path.moveTo(L,J);this.A={x:L,y:J}}lineTo(A,E){this.A=null;null!==this.s?(A=this.s.transformPoint({x:A,y:E}),this.path.lineTo(A.x,A.y)):this.path.lineTo(A,E)}xb(A,E,L,J,K,M){this.A=null;null!==this.s?(A=this.s.transformPoint({x:A,y:E}),L=this.s.transformPoint({x:L,y:J}),K=this.s.transformPoint({x:K,
y:M}),this.path.bezierCurveTo(A.x,A.y,L.x,L.y,K.x,K.y)):this.path.bezierCurveTo(A,E,L,J,K,M)}close(){this.A=null;this.path.closePath()}addPath(A,E,L,J,K,M,R){this.T();const P=new h(null);for(const O of A)O(P);P.Da();A=Ca(E,L,J,K,M,R);null!==this.s&&(A=this.s.multiply(A));this.path.addPath(P.path,A)}Da(){null===this.A||this.W||(this.T(),this.W=!0)}Vb(){return this.W}dispose(){}}class m{constructor(A){this.Kc=A;this.W=this.T=0;this.A=null;this.s=A(null)}clear(){this.Ha(null)}Za(A,E){for(((null===E?
null!==this.A:null===this.A||this.A.a!==E.a||this.A.b!==E.b||this.A.c!==E.c||this.A.d!==E.d||this.A.e!==E.e||this.A.f!==E.f)||this.W<A.length&&(0<this.T||this.s.Vb()))&&this.Ha(E);this.W<A.length;)A[this.W++](this.s);this.s.Da();this.T++;return this.s.path}kb(A){null!==this.s&&A===this.s.path&&this.T--}dispose(){this.s.dispose();this.s=null}Ha(A){this.s.dispose();this.W=this.T=0;this.A=A;this.s=this.Kc(A)}}var w=F.extend("CanvasRenderPath",{__construct:function(){this.__parent.__construct.call(this);
this.da=[];this.wa=new m(A=>new h(A));this.xa=new m(A=>new h(A));this.ea=new Map;this.ca=b;this.Ua=!1},__destruct:function(){this.wa.dispose();this.xa.dispose();for(const A of this.ea.values())A.dispose();this.ea.clear();this.__parent.__destruct.call(this)},rewind:function(){this.da.length=0;this.wa.clear();this.xa.clear();for(const A of this.ea.values())A.clear();this.Ua=!1},addPath:function(A,E,L,J,K,M,R){const P=A.da.slice();this.ka(O=>O.addPath(P,E,L,J,K,M,R))},fillRule:function(A){this.ca=A},
moveTo:function(A,E){this.Ua=!0;this.ka(L=>L.moveTo(A,E))},lineTo:function(A,E){this.qb();this.ka(L=>L.lineTo(A,E))},cubicTo:function(A,E,L,J,K,M){this.qb();this.ka(R=>R.xb(A,E,L,J,K,M))},close:function(){this.ka(A=>A.close())},qb:function(){this.Ua||this.moveTo(0,0)},ka:function(A){this.da.push(A)},ob:function(A){A=null===A?this.wa.Za(this.da,null):this.xa.Za(this.da,A);A.ref&&A.ref();return A},Yb:function(A){var E=this.ea.get(A);void 0===E&&(E=new m(()=>new za(A)),this.ea.set(A,E));E=E.Za(this.da,
null);E.ref&&E.ref();return E},Xa:function(A){this.wa.kb(A);this.xa.kb(A);for(const E of this.ea.values())E.kb(A);A.unref&&A.unref()}});const r=l(0,0,0,0,!1);var C=I.extend("CanvasRenderPaint",{__construct:function(){this.__parent.__construct.call(this);this.fa=q;this.Ra=4278190080;this.ga=1;this.Wa=c.miter;this.Qa=a.butt;this.Sa=0;this.Pa=d(p.srcOver);this.Va=this.ma=this.R=null;this.Ya=1;this.K=0;this.Z=null;this.tb=-1},__destruct:function(){null!==this.Z&&(D.unrefWebGL2Paint(this.Z),this.Z=null);
this.__parent.__destruct.call(this)},color:function(A){this.Ra=A;this.R=null;this.K++},thickness:function(A){this.ga=Math.abs(A);this.K++},join:function(A){this.Wa=A;this.K++},cap:function(A){this.Qa=A;this.K++},feather:function(A){this.Sa=Math.abs(A);this.K++},style:function(A){this.fa=A;this.K++},blendMode:function(A){this.Pa=d(A);this.K++},clearGradient:function(){this.R=null;this.K++},linearGradient:function(A,E,L,J){this.R=l(A,E,L,J,!1);this.K++},radialGradient:function(A,E,L,J){this.R=l(A,E,
L,J,!0);this.K++},addStop:function(A,E){this.R.lb.push({color:A,stop:E});this.R.ha=null;this.K++},completeGradient:function(){},_setGradientTransform:function(A,E,L,J,K,M,R,P,O,T,V,ea,ma){1===A&&0===E&&0===L&&1===J&&0===K&&0===M?this.Va=this.ma=null:(this.ma=Ca(A,E,L,J,K,M),this.Va=Ca(R,P,O,T,V,ea));this.Ya=ma;this.K++},Zb:function(){if(this.tb!==this.K){null!==this.Z&&D.unrefWebGL2Paint(this.Z);const E=null!==this.R?this.R:r;let L=null,J=null;if(null!==this.R){var A=E.lb;L=new Uint32Array(A.length);
J=new Float32Array(A.length);for(let V=0,ea=A.length;V<ea;V++)L[V]=A[V].color,J[V]=A[V].stop}A=1;let K=0,M=0,R=1,P=0,O=0;const T=this.ma;null!==T&&(A=T.a,K=T.b,M=T.c,R=T.d,P=T.e,O=T.f);this.Z=D.makeWebGL2Paint(this.fa,this.Ra,this.ga,this.Wa,this.Qa,this.Sa,L,J,E.Fb,E.Qb,E.Rb,E.Ab,E.Bb,A,K,M,R,P,O);this.tb=this.K}D.refWebGL2Paint(this.Z);return this.Z}});const G=new Set,N=Object.prototype.hasOwnProperty;var U=D.CanvasRenderer=u.extend("Renderer",{__construct:function(A){this.__parent.__construct.call(this);
this.S=[1,0,0,1,0,0];this.J=[1];this.C=A.getContext("2d");this.la=A;this.L=[]},save:function(){this.S.push(...this.S.slice(this.S.length-6));this.J.push(this.J[this.J.length-1]);this.L.push(this.C.save.bind(this.C))},restore:function(){const A=this.S.length-6;if(6>A)throw"restore() called without matching save().";this.S.splice(A);this.J.pop();this.L.push(this.C.restore.bind(this.C))},transform:function(A,E,L,J,K,M){const R=this.S,P=R.length-6;R.splice(P,6,R[P]*A+R[P+2]*E,R[P+1]*A+R[P+3]*E,R[P]*L+
R[P+2]*J,R[P+1]*L+R[P+3]*J,R[P]*K+R[P+2]*M+R[P+4],R[P+1]*K+R[P+3]*M+R[P+5]);this.L.push(this.C.transform.bind(this.C,A,E,L,J,K,M))},rotate:function(A){const E=Math.sin(A);A=Math.cos(A);this.transform(A,E,-E,A,0,0)},modulateOpacity:function(A){this.J[this.J.length-1]*=A},_drawPath:function(A,E){const L=Math.max(0,this.J[this.J.length-1]),J=0!==E.Sa?n.Pb?2:0:null===E.R||null===E.ma?0:E.fa!==v||0<E.Ya?1:n.Pb?2:0;if(2===J)this.$b(A,E,L);else{var K=t(A,E,J,L);null!==K&&this.L.push(()=>{var M=this.C;const R=
M.globalCompositeOperation,P=M.globalAlpha;M.globalCompositeOperation=K.jc;M.globalAlpha=K.opacity;if(null!==K.Db){var O=K.Db;if(null===O.ha){var T=O.Qb;const ma=O.Rb;var V=O.Ab,ea=O.Bb;O.Fb?(V-=T,ea-=ma,O.ha=M.createRadialGradient(T,ma,0,T,ma,Math.sqrt(V*V+ea*ea))):O.ha=M.createLinearGradient(T,ma,V,ea);T=O.lb;for(let Aa=0,nb=T.length;Aa<nb;Aa++)O.ha.addColorStop(T[Aa].stop,e(T[Aa].color))}O=O.ha}else O=e(K.color);null!==K.gradientTransform&&(M.save(),M.transform(K.gradientTransform.a,K.gradientTransform.b,
K.gradientTransform.c,K.gradientTransform.d,K.gradientTransform.e,K.gradientTransform.f));switch(K.style){case v:M.strokeStyle=O;M.lineWidth=K.Ki;a:{switch(K.lc){case a.butt:O="butt";break a;case a.round:O="round";break a;case a.square:O="square";break a}O=void 0}M.lineCap=O;a:{switch(K.join){case c.miter:O="miter";break a;case c.round:O="round";break a;case c.bevel:O="bevel";break a}O=void 0}M.lineJoin=O;M.miterLimit=4;M.stroke(K.ib);break;case q:M.fillStyle=O,M.fill(K.ib,K.fillRule)}null!==K.gradientTransform&&
M.restore();M.globalCompositeOperation=R;M.globalAlpha=P;A.Xa(K.ib)})}},$b:function(A,E,L){const J=x(A,E);if(null!==J){var K=this.S.slice(this.S.length-6);E=oa(g,this.C,this.C.canvas.width,this.C.canvas.height,K,E.Pa,L,(M,R,P)=>D.webGL2PathPixelBounds(J.path,J.hb,M*K[0],R*K[1],M*K[2],R*K[3],M*K[4],R*K[5],P),M=>M.drawPath(J.path,J.hb),()=>{A.Xa(J.path);D.unrefWebGL2Paint(J.hb)},z);G.add(this);this.L.push(E)}},_drawRiveImage:function(A,E,L){var J=A.sb;if(J){var K=this.C,M=d(E),R=Math.max(0,L*this.J[this.J.length-
1]);this.L.push(function(){K.globalCompositeOperation=M;K.globalAlpha=R;K.drawImage(J,0,0);K.globalAlpha=1})}},_getMatrix:function(A){const E=this.S,L=E.length-6;for(let J=0;6>J;++J)A[J]=E[L+J]},_drawImageMesh:function(A,E,L,J,K,M,R,P){const O=A.na;O?this.pb(L,J,K,M,R,P,T=>D.drawWebGL2PendingMesh(T,E,O),()=>E.unref()):E.unref()},_drawImageMeshFromHeap:function(A,E,L,J,K,M,R,P,O,T,V,ea,ma){if(A.ec){try{var Aa=D.HEAPF32.slice(J>>2,(J>>2)+K);var nb=D.HEAPF32.slice(M>>2,(M>>2)+R);var Vc=D.HEAPU16.slice(P>>
1,(P>>1)+O)}catch(Ld){console.error("[Rive] _drawImageMesh: failed to read mesh data from WASM heap. Mesh skipped for this frame.");return}this.pb(E,L,T,V,ea,ma,{image:A,Ti:Aa,Si:nb,indices:Vc},()=>{})}},pb:function(A,E,L,J,K,M,R,P){A=oa(g,this.C,this.C.canvas.width,this.C.canvas.height,this.S.slice(this.S.length-6),d(A),Math.max(0,E*this.J[this.J.length-1]),(O,T,V)=>{V[0]=Math.floor(L*O);V[1]=Math.floor(J*T);V[2]=Math.ceil(K*O);V[3]=Math.ceil(M*T)},R,P,z);G.add(this);this.L.push(A)},_clipPath:function(A){const E=
A.ca===f?"evenodd":"nonzero",L=A.ob(null);this.L.push(()=>{this.C.clip(L,E);A.Xa(L)})},beginFrame:function(A=!0){G.add(this);A&&this.L.push(this.C.clearRect.bind(this.C,0,0,this.la.width,this.la.height))},clear:function(){this.beginFrame(!0)},flush:function(){},translate:function(A,E){this.transform(1,0,0,1,A,E)}});D.makeRenderer=function(A){const E=new U(A),L=E.C;let J=null,K=null;const M={attachSession:function(P){if(P&&J===P)return!0;if(!P||J||"function"!==typeof D.c2dDeferredClaim||"function"!==
typeof D.c2dDeferredRenderer||!D.c2dDeferredClaim(P))return!1;const O=D.c2dDeferredRenderer(P);if(!O)return!1;J=P;K=O;return!0},detachSession:function(){J&&D.c2dDeferredDetach(J);K=J=null},deferredActive:function(){return null!==K}},R={save:function(){D.c2dDeferredSave(J)},restore:function(){D.c2dDeferredRestore(J)},transform:function(P,O,T,V,ea,ma){D.c2dDeferredTransform(J,P,O,T,V,ea,ma)},translate:function(P,O){D.c2dDeferredTransform(J,1,0,0,1,P,O)},rotate:function(P){const O=Math.sin(P);P=Math.cos(P);
D.c2dDeferredTransform(J,P,O,-O,P,0,0)},align:function(P,O,T,V,ea){D.c2dDeferredAlign(J,P,O,T.minX,T.minY,T.maxX,T.maxY,V.minX,V.minY,V.maxX,V.maxY,void 0===ea?1:ea)},beginFrame:function(P=!0){E.beginFrame(P);D.c2dDeferredBeginFrame(J)},clear:function(){E.beginFrame(!0);D.c2dDeferredBeginFrame(J)},flush:function(){D.c2dDeferredReplay(J,E)}};return new Proxy(E,{get(P,O){if(N.call(M,O))return M[O];if("_deferredRecorder"===O)return K;if(null!==K&&N.call(R,O))return R[O];if("function"===typeof P[O])return function(...T){return P[O].apply(P,
T)};if("function"===typeof L[O]){if(-1<Da.indexOf(O))throw Error("RiveException: Method call to '"+O+"()' is not allowed, as the renderer cannot immediately pass through the return                 values of any canvas 2d context methods.");return function(...T){E.L.push(L[O].bind(L,...T))}}return P[O]},set(P,O,T){if(O in L)return E.L.push(()=>{L[O]=T}),!0}})};D.decodeImage=function(A,E){(new k({pa:E})).decode(A)};D.renderFactory={makeRenderPaint:function(){return new C},makeRenderPath:function(){return new w},
makeRenderImage:function(){let A=fa;return new k({La:()=>{A.total++},pa:()=>{A.loaded++;if(A.loaded===A.total){const E=A.ready;E&&(E(),A.ready=null)}}})}};let aa=D.load,fa=null;D.load=function(A,E,L=!0,J=null){const K=new D.FallbackFileAssetLoader;void 0!==E&&K.addLoader(E);L&&(E=new D.CDNFileAssetLoader(J),K.addLoader(E));return(new Promise(function(M){let R=null;fa={total:0,loaded:0,ready:function(){M(R)}};R=aa(A,K,J??null);const P=D.riveScripting;P&&P.prepare();0==fa.total&&M(R)})).then(D.startFileScripts)};
const Wc=D.Artboard.prototype.draw;D.Artboard.prototype.draw=function(A){Wc.call(this,A._deferredRecorder||A)};let Xc=D.RendererWrapper.prototype.align;D.RendererWrapper.prototype.align=function(A,E,L,J,K=1){Xc.call(this,A,E,L,J,K)};y=new da;D.requestAnimationFrame=y.requestAnimationFrame.bind(y);D.cancelAnimationFrame=y.cancelAnimationFrame.bind(y);D.enableFPSCounter=y.tc.bind(y);D.disableFPSCounter=y.qc;y.Ib=z;D.resolveAnimationFrame=z;D.cleanup=function(){g.s&&(g.s["delete"](),g.s=null)}};
var Fa="./this.program";ca&&(_scriptName=self.location.href);var Ga="",Ha,Ia;
if(ba||ca){try{Ga=(new URL(".",_scriptName)).href}catch{}ca&&(Ia=d=>{var e=new XMLHttpRequest;e.open("GET",d,!1);e.responseType="arraybuffer";e.send(null);return new Uint8Array(e.response)});Ha=async d=>{if(Ja(d))return new Promise((l,t)=>{var x=new XMLHttpRequest;x.open("GET",d,!0);x.responseType="arraybuffer";x.onload=()=>{200==x.status||0==x.status&&x.response?l(x.response):t(x.status)};x.onerror=t;x.send(null)});var e=await fetch(d,{credentials:"same-origin"});if(e.ok)return e.arrayBuffer();throw Error(e.status+
" : "+e.url);}}var Ka=console.log.bind(console),La=console.error.bind(console),Ma,Na=!1,Ja=d=>d.startsWith("file://"),Oa,Pa,Qa,Q,Ra,Sa,S,W,Ta,Ua,Va=!1;function Wa(){var d=Xa.buffer;D.HEAP8=Qa=new Int8Array(d);Ra=new Int16Array(d);D.HEAPU8=Q=new Uint8Array(d);D.HEAPU16=Sa=new Uint16Array(d);D.HEAP32=S=new Int32Array(d);D.HEAPU32=W=new Uint32Array(d);D.HEAPF32=Ta=new Float32Array(d);Ua=new Float64Array(d)}
function Ya(d){D.onAbort?.(d);d="Aborted("+d+")";La(d);Na=!0;d=new WebAssembly.RuntimeError(d+". Build with -sASSERTIONS for more info.");Pa?.(d);throw d;}var Za;async function $a(d){if(!Ma)try{var e=await Ha(d);return new Uint8Array(e)}catch{}if(d==Za&&Ma)d=new Uint8Array(Ma);else if(Ia)d=Ia(d);else throw"both async and sync fetching of the wasm failed";return d}
async function ab(d,e){try{var l=await $a(d);return await WebAssembly.instantiate(l,e)}catch(t){La(`failed to asynchronously prepare wasm: ${t}`),Ya(t)}}async function bb(d){var e=Za;if(!Ma&&!Ja(e))try{var l=fetch(e,{credentials:"same-origin"});return await WebAssembly.instantiateStreaming(l,d)}catch(t){La(`wasm streaming compile failed: ${t}`),La("falling back to ArrayBuffer instantiation")}return ab(e,d)}
var cb,db,eb=d=>{for(;0<d.length;)d.shift()(D)},fb=[],gb=[],hb=()=>{var d=D.preRun.shift();gb.push(d)},jb=d=>ib(d),lb=()=>kb(),mb=globalThis.TextDecoder&&new TextDecoder,ob=(d,e,l,t)=>{l=e+l;if(t)return l;for(;d[e]&&!(e>=l);)++e;return e},pb=(d,e=0,l,t)=>{l=ob(d,e,l,t);if(16<l-e&&d.buffer&&mb)return mb.decode(d.subarray(e,l));for(t="";e<l;){var x=d[e++];if(x&128){var z=d[e++]&63;if(192==(x&224))t+=String.fromCharCode((x&31)<<6|z);else{var y=d[e++]&63;x=224==(x&240)?(x&15)<<12|z<<6|y:(x&7)<<18|z<<
12|y<<6|d[e++]&63;65536>x?t+=String.fromCharCode(x):(x-=65536,t+=String.fromCharCode(55296|x>>10,56320|x&1023))}}else t+=String.fromCharCode(x)}return t},qb=(d,e)=>Object.defineProperty(e,"name",{value:d}),rb=[],sb=[0,1,,1,null,1,!0,1,!1,1],X=class extends Error{constructor(d){super(d);this.name="BindingError"}},tb=d=>{if(!d)throw new X(`Cannot use deleted val. handle = ${d}`);return sb[d]},ub=d=>{switch(d){case void 0:return 2;case null:return 4;case !0:return 6;case !1:return 8;default:const e=
rb.pop()||sb.length;sb[e]=d;sb[e+1]=1;return e}};class vb extends Error{}
var Y=d=>{for(var e="";;){var l=Q[d++];if(!l)return e;e+=String.fromCharCode(l)}},wb={},xb=(d,e)=>{if(void 0===e)throw new X("ptr should not be undefined");for(;d.G;)e=d.ta(e),d=d.G;return e},yb={},Bb=d=>{d=zb(d);var e=Y(d);Ab(d);return e},Cb=(d,e)=>{var l=yb[d];if(void 0===l)throw d=`${e} has unknown type ${Bb(d)}`,new X(d);return l},Db=()=>{},Eb=!1,Fb=d=>{if(!globalThis.FinalizationRegistry)return Fb=e=>e,d;Eb=new FinalizationRegistry(e=>{e=e.m;--e.count.value;0===e.count.value&&(e.H?e.O.X(e.H):
e.B.o.X(e.v))});Fb=e=>{var l=e.m;l.H&&Eb.register(e,{m:l},e);return e};Db=e=>{Eb.unregister(e)};return Fb(d)},Gb={},Hb=d=>{for(;d.length;){var e=d.pop();d.pop()(e)}};function Ib(d){return this.u(W[d>>2])}
var Jb={},Kb={},Lb=class extends Error{constructor(d){super(d);this.name="InternalError"}},Nb=(d,e,l)=>{function t(B){B=l(B);if(B.length!==d.length)throw new Lb("Mismatched type converter count");for(var F=0;F<d.length;++F)Mb(d[F],B[F])}d.forEach(B=>Kb[B]=e);var x=Array(e.length),z=[],y=0;for(let [B,F]of e.entries())yb.hasOwnProperty(F)?x[B]=yb[F]:(z.push(F),Jb.hasOwnProperty(F)||(Jb[F]=[]),Jb[F].push(()=>{x[B]=yb[F];++y;y===z.length&&t(x)}));0===z.length&&t(x)};
function Ob(d,e,l={}){var t=e.name;if(!d)throw new X(`type "${t}" must have a positive integer typeid pointer`);if(yb.hasOwnProperty(d)){if(l.Hc)return;throw new X(`Cannot register type '${t}' twice`);}yb[d]=e;delete Kb[d];Jb.hasOwnProperty(d)&&(e=Jb[d],delete Jb[d],e.forEach(x=>x()))}function Mb(d,e,l={}){return Ob(d,e,l)}var Pb=d=>{throw new X(d.m.B.o.name+" instance already deleted");},Qb=[];function Rb(){}
var Sb={},Tb=(d,e,l)=>{if(void 0===d[e].D){var t=d[e];d[e]=function(...x){if(!d[e].D.hasOwnProperty(x.length))throw new X(`Function '${l}' called with an invalid number of arguments (${x.length}) - expects one of (${d[e].D})!`);return d[e].D[x.length].apply(this,x)};d[e].D=[];d[e].D[t.Y]=t}},Ub=(d,e,l)=>{if(D.hasOwnProperty(d)){if(void 0===l||void 0!==D[d].D&&void 0!==D[d].D[l])throw new X(`Cannot register public name '${d}' twice`);Tb(D,d,d);if(D[d].D.hasOwnProperty(l))throw new X(`Cannot register multiple overloads of a function with the same number of arguments (${l})!`);
D[d].D[l]=e}else D[d]=e,D[d].Y=l},Vb=d=>{d=d.replace(/[^a-zA-Z0-9_]/g,"$");var e=d.charCodeAt(0);return 48<=e&&57>=e?`_${d}`:d};function Wb(d,e,l,t,x,z,y,B){this.name=d;this.constructor=e;this.V=l;this.X=t;this.G=x;this.yc=z;this.ta=y;this.rc=B;this.Kb=[]}
var Xb=(d,e,l)=>{for(;e!==l;){if(!e.ta)throw new X(`Expected null or instance of ${l.name}, got an instance of ${e.name}`);d=e.ta(d);e=e.G}return d},Yb=d=>{if(null===d)return"null";var e=typeof d;return"object"===e||"array"===e||"function"===e?d.toString():""+d};
function Zb(d,e){if(null===e){if(this.eb)throw new X(`null is not a valid ${this.name}`);return 0}if(!e.m)throw new X(`Cannot pass "${Yb(e)}" as a ${this.name}`);if(!e.m.v)throw new X(`Cannot pass deleted object as a pointer of type ${this.name}`);return Xb(e.m.v,e.m.B.o,this.o)}
function $b(d,e){if(null===e){if(this.eb)throw new X(`null is not a valid ${this.name}`);if(this.Ga){var l=this.jb();null!==d&&d.push(this.X,l);return l}return 0}if(!e||!e.m)throw new X(`Cannot pass "${Yb(e)}" as a ${this.name}`);if(!e.m.v)throw new X(`Cannot pass deleted object as a pointer of type ${this.name}`);if(!this.Fa&&e.m.B.Fa)throw new X(`Cannot convert argument of type ${e.m.O?e.m.O.name:e.m.B.name} to parameter type ${this.name}`);l=Xb(e.m.v,e.m.B.o,this.o);if(this.Ga){if(void 0===e.m.H)throw new X("Passing raw pointer to smart pointer is illegal");
switch(this.Ii){case 0:if(e.m.O===this)l=e.m.H;else throw new X(`Cannot convert argument of type ${e.m.O?e.m.O.name:e.m.B.name} to parameter type ${this.name}`);break;case 1:l=e.m.H;break;case 2:if(e.m.O===this)l=e.m.H;else{var t=e.clone();l=this.Qc(l,ub(()=>t["delete"]()));null!==d&&d.push(this.X,l)}break;default:throw new X("Unsupported sharing policy");}}return l}
function ac(d,e){if(null===e){if(this.eb)throw new X(`null is not a valid ${this.name}`);return 0}if(!e.m)throw new X(`Cannot pass "${Yb(e)}" as a ${this.name}`);if(!e.m.v)throw new X(`Cannot pass deleted object as a pointer of type ${this.name}`);if(e.m.B.Fa)throw new X(`Cannot convert argument of type ${e.m.B.name} to parameter type ${this.name}`);return Xb(e.m.v,e.m.B.o,this.o)}
var bc=(d,e,l)=>{if(e===l)return d;if(void 0===l.G)return null;d=bc(d,e,l.G);return null===d?null:l.rc(d)},cc=(d,e)=>{e=xb(d,e);return wb[e]},dc=(d,e)=>{if(!e.B||!e.v)throw new Lb("makeClassHandle requires ptr and ptrType");if(!!e.O!==!!e.H)throw new Lb("Both smartPtrType and smartPtr must be specified");e.count={value:1};return Fb(Object.create(d,{m:{value:e,writable:!0}}))};
function ec(d,e,l,t,x,z,y,B,F,I,u){this.name=d;this.o=e;this.eb=l;this.Fa=t;this.Ga=x;this.Nc=z;this.Ii=y;this.Mb=B;this.jb=F;this.Qc=I;this.X=u;x||void 0!==e.G?this.F=$b:(this.F=t?Zb:ac,this.M=null)}
var fc=(d,e,l)=>{if(!D.hasOwnProperty(d))throw new Lb("Replacing nonexistent public symbol");void 0!==D[d].D&&void 0!==l?D[d].D[l]=e:(D[d]=e,D[d].Y=l)},gc={},ic=(d,e,l=[])=>{d.includes("j")?(d=d.replace(/p/g,"i"),e=(0,gc[d])(e,...l)):e=hc.get(e)(...l);return e},jc=(d,e)=>(...l)=>ic(d,e,l),kc=(d,e)=>{d=Y(d);var l=d.includes("j")?jc(d,e):hc.get(e);if("function"!=typeof l)throw new X(`unknown function pointer with signature ${d}: ${e}`);return l};class lc extends Error{}
var mc=(d,e)=>{function l(z){x[z]||yb[z]||(Kb[z]?Kb[z].forEach(l):(t.push(z),x[z]=!0))}var t=[],x={};e.forEach(l);throw new lc(`${d}: `+t.map(Bb).join([", "]));};function nc(d){for(var e=1;e<d.length;++e)if(null!==d[e]&&void 0===d[e].M)return!0;return!1}
function oc(d,e,l,t,x){var z=e.length;if(2>z)throw new X("argTypes array size mismatch! Must at least get return value and 'this' types!");var y=null!==e[1]&&null!==l,B=nc(e),F=!e[0].Jc,I=z-2,u=Array(I),a=[],c=[];return qb(d,function(...p){c.length=0;a.length=y?2:1;a[0]=x;if(y){var q=e[1].F(c,this);a[1]=q}for(var v=0;v<I;++v)u[v]=e[v+2].F(c,p[v]),a.push(u[v]);p=t(...a);if(B)Hb(c);else for(v=y?1:2;v<e.length;v++){var f=1===v?q:u[v-2];null!==e[v].M&&e[v].M(f)}q=F?e[0].u(p):void 0;return q})}
var pc=(d,e)=>{for(var l=[],t=0;t<d;t++)l.push(W[e+4*t>>2]);return l},qc=d=>{d=d.trim();const e=d.indexOf("(");return-1===e?d:d.slice(0,e)},rc=(d,e,l)=>{if(!(d instanceof Object))throw new X(`${l} with invalid "this": ${d}`);if(!(d instanceof e.o.constructor))throw new X(`${l} incompatible with "this" of type ${d.constructor.name}`);if(!d.m.v)throw new X(`cannot call emscripten binding method ${l} on deleted object`);return Xb(d.m.v,d.m.B.o,e.o)},sc=d=>{9<d&&0===--sb[d+1]&&(sb[d]=void 0,rb.push(d))},
tc={name:"emscripten::val",u:d=>{var e=tb(d);sc(d);return e},F:(d,e)=>ub(e),N:Ib,M:null},uc=(d,e,l)=>{switch(e){case 1:return l?function(t){return this.u(Qa[t])}:function(t){return this.u(Q[t])};case 2:return l?function(t){return this.u(Ra[t>>1])}:function(t){return this.u(Sa[t>>1])};case 4:return l?function(t){return this.u(S[t>>2])}:function(t){return this.u(W[t>>2])};default:throw new TypeError(`invalid integer width (${e}): ${d}`);}},vc=(d,e)=>{switch(e){case 4:return function(l){return this.u(Ta[l>>
2])};case 8:return function(l){return this.u(Ua[l>>3])};default:throw new TypeError(`invalid float width (${e}): ${d}`);}},wc=(d,e,l)=>{switch(e){case 1:return l?t=>Qa[t]:t=>Q[t];case 2:return l?t=>Ra[t>>1]:t=>Sa[t>>1];case 4:return l?t=>S[t>>2]:t=>W[t>>2];default:throw new TypeError(`invalid integer width (${e}): ${d}`);}},xc=(d,e,l)=>{var t=Q;if(!(0<l))return 0;var x=e;l=e+l-1;for(var z=0;z<d.length;++z){var y=d.codePointAt(z);if(127>=y){if(e>=l)break;t[e++]=y}else if(2047>=y){if(e+1>=l)break;t[e++]=
192|y>>6;t[e++]=128|y&63}else if(65535>=y){if(e+2>=l)break;t[e++]=224|y>>12;t[e++]=128|y>>6&63;t[e++]=128|y&63}else{if(e+3>=l)break;t[e++]=240|y>>18;t[e++]=128|y>>12&63;t[e++]=128|y>>6&63;t[e++]=128|y&63;z++}}t[e]=0;return e-x},yc=d=>{for(var e=0,l=0;l<d.length;++l){var t=d.charCodeAt(l);127>=t?e++:2047>=t?e+=2:55296<=t&&57343>=t?(e+=4,++l):e+=3}return e},zc=globalThis.TextDecoder?new TextDecoder("utf-16le"):void 0,Ac=(d,e,l)=>{d>>=1;e=ob(Sa,d,e/2,l);if(16<e-d&&zc)return zc.decode(Sa.subarray(d,e));
for(l="";d<e;++d)l+=String.fromCharCode(Sa[d]);return l},Bc=(d,e,l)=>{l??=2147483647;if(2>l)return 0;l-=2;var t=e;l=l<2*d.length?l/2:d.length;for(var x=0;x<l;++x)Ra[e>>1]=d.charCodeAt(x),e+=2;Ra[e>>1]=0;return e-t},Cc=d=>2*d.length,Dc=(d,e,l)=>{var t="";d>>=2;for(var x=0;!(x>=e/4);x++){var z=W[d+x];if(!z&&!l)break;t+=String.fromCodePoint(z)}return t},Ec=(d,e,l)=>{l??=2147483647;if(4>l)return 0;var t=e;l=t+l-4;for(var x=0;x<d.length;++x){var z=d.codePointAt(x);65535<z&&x++;S[e>>2]=z;e+=4;if(e+4>l)break}S[e>>
2]=0;return e-t},Fc=d=>{for(var e=0,l=0;l<d.length;++l)65535<d.codePointAt(l)&&l++,e+=4;return e},Gc=[],Hc=d=>{var e=Gc.length;Gc.push(d);return e},Ic=(d,e)=>{for(var l=Array(d),t=0;t<d;++t)l[t]=Cb(W[e+4*t>>2],`parameter ${t}`);return l},Jc={},Kc=d=>{var e=Jc[d];return void 0===e?Y(d):e},Lc=[0,31,60,91,121,152,182,213,244,274,305,335],Mc=[0,31,59,90,120,151,181,212,243,273,304,334],Nc=[],Oc=d=>{d.Oi=d.getExtension("WEBGL_draw_instanced_base_vertex_base_instance")},Pc=d=>{d.Qi=d.getExtension("WEBGL_multi_draw_instanced_base_vertex_base_instance")},
Z,Qc=d=>{var e="EXT_color_buffer_float EXT_conservative_depth EXT_disjoint_timer_query_webgl2 EXT_texture_norm16 NV_shader_noperspective_interpolation WEBGL_clip_cull_distance EXT_clip_control EXT_color_buffer_half_float EXT_depth_clamp EXT_float_blend EXT_polygon_offset_clamp EXT_texture_compression_bptc EXT_texture_compression_rgtc EXT_texture_filter_anisotropic KHR_parallel_shader_compile OES_texture_float_linear WEBGL_blend_func_extended WEBGL_compressed_texture_astc WEBGL_compressed_texture_etc WEBGL_compressed_texture_etc1 WEBGL_compressed_texture_s3tc WEBGL_compressed_texture_s3tc_srgb WEBGL_debug_renderer_info WEBGL_debug_shaders WEBGL_lose_context WEBGL_multi_draw WEBGL_polygon_mode".split(" ");
return(d.getSupportedExtensions()||[]).filter(l=>e.includes(l))},Rc=1,Sc=[],Tc=[],Uc=[],Yc=[],Zc=[],$c=[],ad=[],bd=[],cd=[],dd={},ed=4,fd=0,gd=d=>{for(var e=Rc++,l=d.length;l<e;l++)d[l]=null;return e},jd=(d,e,l,t)=>{for(var x=0;x<d;x++){var z=Z[l](),y=z&&gd(t);z?(z.name=y,t[y]=z):hd||=1282;S[e+4*x>>2]=y}},wa=(d,e)=>{var l=gd(bd),t={handle:l,attributes:e,version:e.Pi,I:d};d.canvas&&(d.canvas.Xb=t);bd[l]=t;if("undefined"==typeof e.sc||e.sc)if((d=t)||(d=kd),!d.Ic){d.Ic=!0;e=d.I;e.Lc=e.getExtension("WEBGL_multi_draw");
e.vc=e.getExtension("EXT_polygon_offset_clamp");e.uc=e.getExtension("EXT_clip_control");e.Li=e.getExtension("WEBGL_polygon_mode");Oc(e);Pc(e);2<=d.version&&(e.zb=e.getExtension("EXT_disjoint_timer_query_webgl2"));if(2>d.version||!e.zb)e.zb=e.getExtension("EXT_disjoint_timer_query");for(var x of Qc(e))x.includes("lose_context")||x.includes("debug")||e.getExtension(x)}return l},xa=d=>{kd=bd[d];D.ctx=Z=kd?.I;return!(d&&!Z)},ya=d=>{kd===bd[d]&&(kd=null);"object"==typeof JSEvents&&JSEvents.Ri(bd[d].I.canvas);
bd[d]?.I.canvas&&(bd[d].I.canvas.Xb=void 0);bd[d]=null},hd,kd,ld={},nd=()=>{if(!md){var d={USER:"web_user",LOGNAME:"web_user",PATH:"/",PWD:"/",HOME:"/home/web_user",LANG:(globalThis.navigator?.language??"C").replace("-","_")+".UTF-8",_:Fa||"./this.program"},e;for(e in ld)void 0===ld[e]?delete d[e]:d[e]=ld[e];var l=[];for(e in d)l.push(`${e}=${d[e]}`);md=l}return md},md,od=[null,[],[]],pd=[],qd=()=>{var d=Qc(Z);return d=d.concat(d.map(e=>"GL_"+e))},rd=(d,e)=>{if(e){var l=void 0;switch(d){case 36346:l=
1;break;case 36344:return;case 34814:case 36345:l=0;break;case 34466:var t=Z.getParameter(34467);l=t?t.length:0;break;case 33309:if(2>kd.version){hd||=1282;return}l=qd().length;break;case 33307:case 33308:if(2>kd.version){hd||=1280;return}l=33307==d?3:0}if(void 0===l)switch(t=Z.getParameter(d),typeof t){case "number":l=t;break;case "boolean":l=t?1:0;break;case "string":hd||=1280;return;case "object":if(null===t)switch(d){case 34964:case 35725:case 34965:case 36006:case 36007:case 32873:case 34229:case 36662:case 36663:case 35053:case 35055:case 36010:case 35097:case 35869:case 32874:case 36389:case 35983:case 35368:case 34068:l=
0;break;default:hd||=1280;return}else{if(t instanceof Float32Array||t instanceof Uint32Array||t instanceof Int32Array||t instanceof Array){for(d=0;d<t.length;++d)S[e+4*d>>2]=t[d];return}try{l=t.name|0}catch(x){hd||=1280;La(`GL_INVALID_ENUM in glGet${0}v: Unknown object returned from WebGL getParameter(${d})! (error: ${x})`);return}}break;default:hd||=1280;La(`GL_INVALID_ENUM in glGet${0}v: Native code calling glGet${0}v(${d}) and it returns ${t} of type ${typeof t}!`);return}S[e>>2]=l}else hd||=1281},
td=d=>{var e=yc(d)+1,l=sd(e);l&&xc(d,l,e);return l},ud=d=>"]"==d.slice(-1)&&d.lastIndexOf("["),vd=d=>{d-=5120;return 0==d?Qa:1==d?Q:2==d?Ra:4==d?S:6==d?Ta:5==d||28922==d||28520==d||30779==d||30782==d?W:Sa},xd=d=>wd(d);
(()=>{let d=Rb.prototype;Object.assign(d,{isAliasOf:function(l){if(!(this instanceof Rb&&l instanceof Rb))return!1;var t=this.m.B.o,x=this.m.v;l.m=l.m;var z=l.m.B.o;for(l=l.m.v;t.G;)x=t.ta(x),t=t.G;for(;z.G;)l=z.ta(l),z=z.G;return t===z&&x===l},clone:function(){this.m.v||Pb(this);if(this.m.ja)return this.m.count.value+=1,this;var l=Fb,t=Object,x=t.create,z=Object.getPrototypeOf(this),y=this.m;l=l(x.call(t,z,{m:{value:{count:y.count,oa:y.oa,ja:y.ja,v:y.v,B:y.B,H:y.H,O:y.O}}}));l.m.count.value+=1;l.m.oa=
!1;return l},["delete"](){this.m.v||Pb(this);if(this.m.oa&&!this.m.ja)throw new X("Object already scheduled for deletion");Db(this);var l=this.m;--l.count.value;0===l.count.value&&(l.H?l.O.X(l.H):l.B.o.X(l.v));this.m.ja||(this.m.H=void 0,this.m.v=void 0)},isDeleted:function(){return!this.m.v},deleteLater:function(){this.m.v||Pb(this);if(this.m.oa&&!this.m.ja)throw new X("Object already scheduled for deletion");Qb.push(this);this.m.oa=!0;return this}});const e=Symbol.dispose;e&&(d[e]=d["delete"])})();
Object.assign(ec.prototype,{zc(d){this.Mb&&(d=this.Mb(d));return d},yb(d){this.X?.(d)},N:Ib,u:function(d){function e(){return this.Ga?dc(this.o.V,{B:this.Nc,v:l,O:this,H:d}):dc(this.o.V,{B:this,v:d})}var l=this.zc(d);if(!l)return this.yb(d),null;var t=cc(this.o,l);if(void 0!==t){if(0===t.m.count.value)return t.m.v=l,t.m.H=d,t.clone();t=t.clone();this.yb(d);return t}t=this.o.yc(l);t=Sb[t];if(!t)return e.call(this);t=this.Fa?t.mc:t.pointerType;var x=bc(l,this.o,t.o);return null===x?e.call(this):this.Ga?
dc(t.o.V,{B:t,v:x,O:this,H:d}):dc(t.o.V,{B:t,v:x})}});for(let d=0;32>d;++d)pd.push(Array(d));D.print&&(Ka=D.print);D.printErr&&(La=D.printErr);D.wasmBinary&&(Ma=D.wasmBinary);D.thisProgram&&(Fa=D.thisProgram);if(D.preInit)for("function"==typeof D.preInit&&(D.preInit=[D.preInit]);0<D.preInit.length;)D.preInit.shift()();
var Bd={332054:(d,e,l,t,x)=>{if("undefined"===typeof window||void 0===(window.AudioContext||window.webkitAudioContext))return 0;if("undefined"===typeof window.miniaudio){window.miniaudio={referenceCount:0};window.miniaudio.device_type={};window.miniaudio.device_type.playback=d;window.miniaudio.device_type.capture=e;window.miniaudio.device_type.duplex=l;window.miniaudio.device_state={};window.miniaudio.device_state.stopped=t;window.miniaudio.device_state.started=x;let z=window.miniaudio;z.devices=
[];z.track_device=function(y){for(var B=0;B<z.devices.length;++B)if(null==z.devices[B])return z.devices[B]=y,B;z.devices.push(y);return z.devices.length-1};z.untrack_device_by_index=function(y){for(z.devices[y]=null;0<z.devices.length;)if(null==z.devices[z.devices.length-1])z.devices.pop();else break};z.untrack_device=function(y){for(var B=0;B<z.devices.length;++B)if(z.devices[B]==y)return z.untrack_device_by_index(B)};z.get_device_by_index=function(y){return z.devices[y]};z.unlock_event_types=["touchend",
"click"];z.unlock=function(){for(var y=0;y<z.devices.length;++y){var B=z.devices[y];null!=B&&null!=B.P&&B.state===z.device_state.started&&B.P.resume().then(()=>{yd(B.Jb)},F=>{console.error("Failed to resume audiocontext",F)})}z.unlock_event_types.map(function(F){document.removeEventListener(F,z.unlock,!0)})};z.unlock_event_types.map(function(y){document.addEventListener(y,z.unlock,!0)})}window.miniaudio.referenceCount+=1;return 1},334232:()=>{"undefined"!==typeof window.miniaudio&&(window.miniaudio.unlock_event_types.map(function(d){document.removeEventListener(d,
window.miniaudio.unlock,!0)}),--window.miniaudio.referenceCount,0===window.miniaudio.referenceCount&&delete window.miniaudio)},334536:()=>void 0!==navigator.mediaDevices&&void 0!==navigator.mediaDevices.getUserMedia,334640:()=>{try{var d=new (window.AudioContext||window.webkitAudioContext),e=d.sampleRate;d.close();return e}catch(l){return 0}},334811:(d,e,l,t,x,z)=>{if("undefined"===typeof window.miniaudio)return-1;var y={},B={};d==window.miniaudio.device_type.playback&&0!=l&&(B.sampleRate=l);y.P=
new (window.AudioContext||window.webkitAudioContext)(B);y.P.suspend();y.state=window.miniaudio.device_state.stopped;l=0;d!=window.miniaudio.device_type.playback&&(l=e);y.$=y.P.createScriptProcessor(t,l,e);y.$.onaudioprocess=function(F){if(null==y.Ea||0==y.Ea.length)y.Ea=new Float32Array(Ta.buffer,x,t*e);if(d==window.miniaudio.device_type.capture||d==window.miniaudio.device_type.duplex){for(var I=0;I<e;I+=1)for(var u=F.inputBuffer.getChannelData(I),a=y.Ea,c=0;c<t;c+=1)a[c*e+I]=u[c];zd(z,t,x)}if(d==
window.miniaudio.device_type.playback||d==window.miniaudio.device_type.duplex)for(Ad(z,t,x),I=0;I<F.outputBuffer.numberOfChannels;++I)for(u=F.outputBuffer.getChannelData(I),a=y.Ea,c=0;c<t;c+=1)u[c]=a[c*e+I];else for(I=0;I<F.outputBuffer.numberOfChannels;++I)F.outputBuffer.getChannelData(I).fill(0)};d!=window.miniaudio.device_type.capture&&d!=window.miniaudio.device_type.duplex||navigator.mediaDevices.getUserMedia({audio:!0,video:!1}).then(function(F){y.Oa=y.P.createMediaStreamSource(F);y.Oa.connect(y.$);
y.$.connect(y.P.destination)}).catch(function(F){console.log("Failed to get user media: "+F)});d==window.miniaudio.device_type.playback&&y.$.connect(y.P.destination);y.Jb=z;return window.miniaudio.track_device(y)},337688:d=>window.miniaudio.get_device_by_index(d).P.sampleRate,337761:d=>{d=window.miniaudio.get_device_by_index(d);void 0!==d.$&&(d.$.onaudioprocess=function(){},d.$.disconnect(),d.$=void 0);void 0!==d.Oa&&(d.Oa.disconnect(),d.Oa=void 0);d.P.close();d.P=void 0;d.Jb=void 0},338161:d=>{window.miniaudio.untrack_device_by_index(d)},
338211:d=>{d=window.miniaudio.get_device_by_index(d);d.P.resume();d.state=window.miniaudio.device_state.started},338350:d=>{d=window.miniaudio.get_device_by_index(d);d.P.suspend();d.state=window.miniaudio.device_state.stopped}},Ab,sd,zb,Cd,Dd,Ed,Fd,yd,zd,Ad,Gd,Hd,ib,wd,kb,Xa,hc,Jd={__syscall_fcntl64:function(){return 0},__syscall_ioctl:function(){return 0},__syscall_openat:function(){},_abort_js:()=>Ya(""),_embind_create_inheriting_constructor:(d,e,l)=>{d=Y(d);e=Cb(e,"wrapper");l=tb(l);var t=e.o,
x=t.V,z=t.G.V,y=t.G.constructor;d=qb(d,function(...B){for(var F of t.G.Kb)if(this[F]===z[F])throw new vb(`Pure virtual function ${F} must be implemented in JavaScript`);Object.defineProperty(this,"__parent",{value:x});this.__construct(...B)});x.__construct=function(...B){if(this===x)throw new X("Pass correct 'this' to __construct");B=y.implement(this,...B);Db(B);var F=B.m;B.notifyOnDestruction();F.ja=!0;Object.defineProperties(this,{m:{value:F}});Fb(this);B=F.v;B=xb(t,B);if(wb.hasOwnProperty(B))throw new X(`Tried to register registered instance: ${B}`);
wb[B]=this};x.__destruct=function(){if(this===x)throw new X("Pass correct 'this' to __destruct");Db(this);var B=this.m.v;B=xb(t,B);if(wb.hasOwnProperty(B))delete wb[B];else throw new X(`Tried to unregister unregistered instance: ${B}`);};d.prototype=Object.create(x);Object.assign(d.prototype,l);return ub(d)},_embind_finalize_value_object:d=>{var e=Gb[d];delete Gb[d];var l=e.jb,t=e.X,x=e.Cb,z=x.map(y=>y.Cc).concat(x.map(y=>y.Gi));Nb([d],z,y=>{var B={},F,I;for([F,I]of x.entries()){const u=y[F],a=I.Ac,
c=I.Bc,p=y[F+x.length],q=I.Fi,v=I.Hi;B[I.wc]={read:f=>u.u(a(c,f)),write:(f,b)=>{var g=[];q(v,f,p.F(g,b));Hb(g)},optional:u.optional}}return[{name:e.name,u:u=>{var a={},c;for(c in B)a[c]=B[c].read(u);t(u);return a},F:(u,a)=>{for(var c in B)if(!(c in a||B[c].optional))throw new TypeError(`Missing field: "${c}"`);var p=l();for(c in B)B[c].write(p,a[c]);null!==u&&u.push(t,p);return p},N:Ib,M:t}]})},_embind_register_bigint:()=>{},_embind_register_bool:(d,e,l,t)=>{e=Y(e);Mb(d,{name:e,u:function(x){return!!x},
F:function(x,z){return z?l:t},N:function(x){return this.u(Q[x])},M:null})},_embind_register_class:(d,e,l,t,x,z,y,B,F,I,u,a,c)=>{u=Y(u);z=kc(x,z);B&&=kc(y,B);I&&=kc(F,I);c=kc(a,c);var p=Vb(u);Ub(p,function(){mc(`Cannot construct ${u} due to unbound types`,[t])});Nb([d,e,l],t?[t]:[],q=>{q=q[0];if(t){var v=q.o;var f=v.V}else f=Rb.prototype;q=qb(u,function(...k){if(Object.getPrototypeOf(this)!==b)throw new X(`Use 'new' to construct ${u}`);if(void 0===g.aa)throw new X(`${u} has no accessible constructor`);
var h=g.aa[k.length];if(void 0===h)throw new X(`Tried to invoke ctor of ${u} with invalid number of parameters (${k.length}) - expected (${Object.keys(g.aa).toString()}) parameters instead!`);return h.apply(this,k)});var b=Object.create(f,{constructor:{value:q}});q.prototype=b;var g=new Wb(u,q,b,c,v,z,B,I);if(g.G){var n;(n=g.G).ua??(n.ua=[]);g.G.ua.push(g)}v=new ec(u,g,!0,!1,!1);n=new ec(u+"*",g,!1,!1,!1);f=new ec(u+" const*",g,!1,!0,!1);Sb[d]={pointerType:n,mc:f};fc(p,q);return[v,n,f]})},_embind_register_class_class_function:(d,
e,l,t,x,z,y)=>{var B=pc(l,t);e=Y(e);e=qc(e);z=kc(x,z);Nb([],[d],F=>{function I(){mc(`Cannot call ${u} due to unbound types`,B)}F=F[0];var u=`${F.name}.${e}`;e.startsWith("@@")&&(e=Symbol[e.substring(2)]);var a=F.o.constructor;void 0===a[e]?(I.Y=l-1,a[e]=I):(Tb(a,e,u),a[e].D[l-1]=I);Nb([],B,c=>{c=oc(u,[c[0],null].concat(c.slice(1)),null,z,y);void 0===a[e].D?(c.Y=l-1,a[e]=c):a[e].D[l-1]=c;if(F.o.ua)for(const p of F.o.ua)p.constructor.hasOwnProperty(e)||(p.constructor[e]=c);return[]});return[]})},_embind_register_class_class_property:(d,
e,l,t,x,z,y,B)=>{e=Y(e);z=kc(x,z);Nb([],[d],F=>{F=F[0];var I=`${F.name}.${e}`,u={get(){mc(`Cannot access ${I} due to unbound types`,[l])},enumerable:!0,configurable:!0};u.set=B?()=>{mc(`Cannot access ${I} due to unbound types`,[l])}:()=>{throw new X(`${I} is a read-only property`);};Object.defineProperty(F.o.constructor,e,u);Nb([],[l],a=>{a=a[0];var c={get(){return a.u(z(t))},enumerable:!0};B&&(B=kc(y,B),c.set=p=>{var q=[];B(t,a.F(q,p));Hb(q)});Object.defineProperty(F.o.constructor,e,c);return[]});
return[]})},_embind_register_class_constructor:(d,e,l,t,x,z)=>{var y=pc(e,l);x=kc(t,x);Nb([],[d],B=>{B=B[0];var F=`constructor ${B.name}`;void 0===B.o.aa&&(B.o.aa=[]);if(void 0!==B.o.aa[e-1])throw new X(`Cannot register multiple constructors with identical number of parameters (${e-1}) for class '${B.name}'! Overload resolution is currently only performed using the parameter count, not actual type info!`);B.o.aa[e-1]=()=>{mc(`Cannot construct ${B.name} due to unbound types`,y)};Nb([],y,I=>{I.splice(1,
0,null);B.o.aa[e-1]=oc(F,I,null,x,z);return[]});return[]})},_embind_register_class_function:(d,e,l,t,x,z,y,B)=>{var F=pc(l,t);e=Y(e);e=qc(e);z=kc(x,z);Nb([],[d],I=>{function u(){mc(`Cannot call ${a} due to unbound types`,F)}I=I[0];var a=`${I.name}.${e}`;e.startsWith("@@")&&(e=Symbol[e.substring(2)]);B&&I.o.Kb.push(e);var c=I.o.V,p=c[e];void 0===p||void 0===p.D&&p.className!==I.name&&p.Y===l-2?(u.Y=l-2,u.className=I.name,c[e]=u):(Tb(c,e,a),c[e].D[l-2]=u);Nb([],F,q=>{q=oc(a,q,I,z,y);void 0===c[e].D?
(q.Y=l-2,c[e]=q):c[e].D[l-2]=q;return[]});return[]})},_embind_register_class_property:(d,e,l,t,x,z,y,B,F,I)=>{e=Y(e);x=kc(t,x);Nb([],[d],u=>{u=u[0];var a=`${u.name}.${e}`,c={get(){mc(`Cannot access ${a} due to unbound types`,[l,y])},enumerable:!0,configurable:!0};c.set=F?()=>mc(`Cannot access ${a} due to unbound types`,[l,y]):()=>{throw new X(a+" is a read-only property");};Object.defineProperty(u.o.V,e,c);Nb([],F?[l,y]:[l],p=>{var q=p[0],v={get(){var b=rc(this,u,a+" getter");return q.u(x(z,b))},
enumerable:!0};if(F){F=kc(B,F);var f=p[1];v.set=function(b){var g=rc(this,u,a+" setter"),n=[];F(I,g,f.F(n,b));Hb(n)}}Object.defineProperty(u.o.V,e,v);return[]});return[]})},_embind_register_emval:d=>Mb(d,tc),_embind_register_enum:(d,e,l,t,x)=>{e=Y(e);x=0===x?"object":1===x?"number":"string";switch(x){case "object":function y(){}y.values={};Mb(d,{name:e,constructor:y,valueType:x,u:function(B){return this.constructor.values[B]},F:(B,F)=>F.value,N:uc(e,l,t),M:null});Ub(e,y);break;case "number":var z=
{};Mb(d,{name:e,fb:z,valueType:x,u:B=>B,F:(B,F)=>F,N:uc(e,l,t),M:null});Ub(e,z);delete D[e].Y;break;case "string":z={},Mb(d,{name:e,Ub:{},Nb:{},fb:z,valueType:x,u:function(B){return this.Nb[B]},F:function(B,F){return this.Ub[F]},N:uc(e,l,t),M:null}),Ub(e,z),delete D[e].Y}},_embind_register_enum_value:(d,e,l)=>{var t=Cb(d,"enum");e=Y(e);switch(t.valueType){case "object":d=t.constructor;t=Object.create(t.constructor.prototype,{value:{value:l},constructor:{value:qb(`${t.name}_${e}`,function(){})}});
d.values[l]=t;d[e]=t;break;case "number":t.fb[e]=l;break;case "string":t.Ub[e]=l,t.Nb[l]=e,t.fb[e]=e}},_embind_register_float:(d,e,l)=>{e=Y(e);Mb(d,{name:e,u:t=>t,F:(t,x)=>x,N:vc(e,l),M:null})},_embind_register_function:(d,e,l,t,x,z)=>{var y=pc(e,l);d=Y(d);d=qc(d);x=kc(t,x);Ub(d,function(){mc(`Cannot call ${d} due to unbound types`,y)},e-1);Nb([],y,B=>{fc(d,oc(d,[B[0],null].concat(B.slice(1)),null,x,z),e-1);return[]})},_embind_register_integer:(d,e,l,t,x)=>{e=Y(e);let z=B=>B;if(0===t){var y=32-8*
l;z=B=>B<<y>>>y;x=z(x)}Mb(d,{name:e,u:z,F:(B,F)=>F,N:wc(e,l,0!==t),M:null})},_embind_register_memory_view:(d,e,l)=>{function t(z){return new x(Qa.buffer,W[z+4>>2],W[z>>2])}var x=[Int8Array,Uint8Array,Int16Array,Uint16Array,Int32Array,Uint32Array,Float32Array,Float64Array][e];l=Y(l);Mb(d,{name:l,u:t,N:t},{Hc:!0})},_embind_register_std_string:(d,e)=>{e=Y(e);Mb(d,{name:e,u(l){var t=(t=l+4)?pb(Q,t,W[l>>2],!0):"";Ab(l);return t},F(l,t){t instanceof ArrayBuffer&&(t=new Uint8Array(t));var x="string"==typeof t;
if(!(x||ArrayBuffer.isView(t)&&1==t.BYTES_PER_ELEMENT))throw new X("Cannot pass non-string to std::string");var z=x?yc(t):t.length;var y=sd(4+z+1),B=y+4;W[y>>2]=z;x?xc(t,B,z+1):Q.set(t,B);null!==l&&l.push(Ab,y);return y},N:Ib,M(l){Ab(l)}})},_embind_register_std_wstring:(d,e,l)=>{l=Y(l);if(2===e){var t=Ac;var x=Bc;var z=Cc}else t=Dc,x=Ec,z=Fc;Mb(d,{name:l,u:y=>{var B=t(y+4,W[y>>2]*e,!0);Ab(y);return B},F:(y,B)=>{if("string"!=typeof B)throw new X(`Cannot pass non-string to C++ string type ${l}`);var F=
z(B),I=sd(4+F+e);W[I>>2]=F/e;x(B,I+4,F+e);null!==y&&y.push(Ab,I);return I},N:Ib,M(y){Ab(y)}})},_embind_register_value_object:(d,e,l,t,x,z)=>{Gb[d]={name:Y(e),jb:kc(l,t),X:kc(x,z),Cb:[]}},_embind_register_value_object_field:(d,e,l,t,x,z,y,B,F,I)=>{Gb[d].Cb.push({wc:Y(e),Cc:l,Ac:kc(t,x),Bc:z,Gi:y,Fi:kc(B,F),Hi:I})},_embind_register_void:(d,e)=>{e=Y(e);Mb(d,{Jc:!0,name:e,u:()=>{},F:()=>{}})},_emscripten_throw_longjmp:()=>{throw Infinity;},_emval_create_invoker:(d,e,l)=>{var [t,...x]=Ic(d,e),z=t.F.bind(t),
y=x.map(F=>F.N.bind(F));d--;var B=Array(d);e=`methodCaller<(${x.map(F=>F.name)}) => ${t.name}>`;return Hc(qb(e,(F,I,u,a)=>{for(var c=0,p=0;p<d;++p)B[p]=y[p](a+c),c+=8;switch(l){case 0:var q=tb(F).apply(null,B);break;case 2:q=Reflect.construct(tb(F),B);break;case 3:q=B[0];break;case 1:q=tb(F)[Kc(I)](...B)}F=[];q=z(F,q);F.length&&(W[u>>2]=ub(F));return q}))},_emval_decref:sc,_emval_get_module_property:d=>{d=Kc(d);return ub(D[d])},_emval_get_property:(d,e)=>{d=tb(d);e=tb(e);return ub(d[e])},_emval_incref:d=>
{9<d&&(sb[d+1]+=1)},_emval_invoke:(d,e,l,t,x)=>Gc[d](e,l,t,x),_emval_new_array:()=>ub([]),_emval_new_cstring:d=>ub(Kc(d)),_emval_new_object:()=>ub({}),_emval_run_destructors:d=>{var e=tb(d);Hb(e);sc(d)},_emval_set_property:(d,e,l)=>{d=tb(d);e=tb(e);l=tb(l);d[e]=l},_gmtime_js:function(d,e,l){d=new Date(1E3*(e+2097152>>>0<4194305-!!d?(d>>>0)+4294967296*e:NaN));S[l>>2]=d.getUTCSeconds();S[l+4>>2]=d.getUTCMinutes();S[l+8>>2]=d.getUTCHours();S[l+12>>2]=d.getUTCDate();S[l+16>>2]=d.getUTCMonth();S[l+20>>
2]=d.getUTCFullYear()-1900;S[l+24>>2]=d.getUTCDay();S[l+28>>2]=(d.getTime()-Date.UTC(d.getUTCFullYear(),0,1,0,0,0,0))/864E5|0},_localtime_js:function(d,e,l){d=new Date(1E3*(e+2097152>>>0<4194305-!!d?(d>>>0)+4294967296*e:NaN));S[l>>2]=d.getSeconds();S[l+4>>2]=d.getMinutes();S[l+8>>2]=d.getHours();S[l+12>>2]=d.getDate();S[l+16>>2]=d.getMonth();S[l+20>>2]=d.getFullYear()-1900;S[l+24>>2]=d.getDay();e=d.getFullYear();S[l+28>>2]=(0!==e%4||0===e%100&&0!==e%400?Mc:Lc)[d.getMonth()]+d.getDate()-1|0;S[l+36>>
2]=-(60*d.getTimezoneOffset());e=(new Date(d.getFullYear(),6,1)).getTimezoneOffset();var t=(new Date(d.getFullYear(),0,1)).getTimezoneOffset();S[l+32>>2]=(e!=t&&d.getTimezoneOffset()==Math.min(t,e))|0},_timegm_js:function(d){var e=new Date(Date.UTC(S[d+20>>2]+1900,S[d+16>>2],S[d+12>>2],S[d+8>>2],S[d+4>>2],S[d>>2],0));S[d+24>>2]=e.getUTCDay();S[d+28>>2]=(e.getTime()-Date.UTC(e.getUTCFullYear(),0,1,0,0,0,0))/864E5|0;d=e.getTime()/1E3;Hd((cb=d,1<=+Math.abs(cb)?0<cb?+Math.floor(cb/4294967296)>>>0:~~+Math.ceil((cb-
+(~~cb>>>0))/4294967296)>>>0:0));return d>>>0},_tzset_js:(d,e,l,t)=>{var x=(new Date).getFullYear(),z=(new Date(x,0,1)).getTimezoneOffset();x=(new Date(x,6,1)).getTimezoneOffset();W[d>>2]=60*Math.max(z,x);S[e>>2]=Number(z!=x);e=y=>{var B=Math.abs(y);return`UTC${0<=y?"-":"+"}${String(Math.floor(B/60)).padStart(2,"0")}${String(B%60).padStart(2,"0")}`};d=e(z);e=e(x);x<z?(xc(d,l,17),xc(e,t,17)):(xc(d,t,17),xc(e,l,17))},beginPixelLocalStorageWEBGL:function(d,e,l){(d=bd[d].I.Ma)&&d.beginPixelLocalStorageWEBGL(new Uint32Array(Xa.buffer,
4*l,e))},clock_time_get:function(d,e,l,t){if(!(0<=d&&3>=d))return 28;d=Math.round(1E6*(0===d?Date.now():performance.now()));db=[d>>>0,(cb=d,1<=+Math.abs(cb)?0<cb?+Math.floor(cb/4294967296)>>>0:~~+Math.ceil((cb-+(~~cb>>>0))/4294967296)>>>0:0)];S[t>>2]=db[0];S[t+4>>2]=db[1];return 0},decode_image:function(d,e,l){var t=D.images;t||(t=new Map,D.images=t);var x=new Image;t.set(d,x);e=D.HEAP8.subarray(e,e+l);l=new Uint8Array(l);l.set(e);x.src=URL.createObjectURL(new Blob([l],{type:"image/png"}));x.onload=
function(){D._setWebImage(d,x.width,x.height)}},delete_image:function(d){var e=D.images;e&&e.get(d)&&e.delete(d)},emscripten_asm_const_int:(d,e,l)=>{Nc.length=0;for(var t;t=Q[e++];){var x=105!=t;x&=112!=t;l+=x&&l%8?4:0;Nc.push(112==t?W[l>>2]:105==t?S[l>>2]:Ua[l>>3]);l+=x?8:4}return Bd[d](...Nc)},emscripten_date_now:()=>Date.now(),emscripten_get_now:()=>performance.now(),emscripten_resize_heap:d=>{var e=Q.length;d>>>=0;if(2147483648<d)return!1;for(var l=1;4>=l;l*=2){var t=e*(1+.2/l);t=Math.min(t,d+
100663296);a:{t=(Math.min(2147483648,65536*Math.ceil(Math.max(d,t)/65536))-Xa.buffer.byteLength+65535)/65536|0;try{Xa.grow(t);Wa();var x=1;break a}catch(z){}x=void 0}if(x)return!0}return!1},emscripten_webgl_enable_extension:(d,e)=>{d=bd[d];e=e?pb(Q,e):"";e.startsWith("GL_")&&(e=e.slice(3));"WEBGL_draw_instanced_base_vertex_base_instance"==e&&Oc(Z);"WEBGL_multi_draw_instanced_base_vertex_base_instance"==e&&Pc(Z);"WEBGL_multi_draw"==e&&(Z.Lc=Z.getExtension("WEBGL_multi_draw"));"EXT_polygon_offset_clamp"==
e&&(Z.vc=Z.getExtension("EXT_polygon_offset_clamp"));"EXT_clip_control"==e&&(Z.uc=Z.getExtension("EXT_clip_control"));"WEBGL_polygon_mode"==e&&(Z.Li=Z.getExtension("WEBGL_polygon_mode"));return!!d.I.getExtension(e)},emscripten_webgl_get_current_context:()=>kd?kd.handle:0,emscripten_webgl_make_context_current:d=>xa(d)?0:-5,enable_WEBGL_provoking_vertex:function(d){d=bd[d].I;d.Lb=d.getExtension("WEBGL_provoking_vertex");return!!d.Lb},enable_WEBGL_shader_pixel_local_storage_coherent:function(d){d=bd[d].I;
const e=d.getExtension("WEBGL_shader_pixel_local_storage");if(e&&e.isCoherent()){if(5==e.framebufferTexturePixelLocalStorageWEBGL.length)return d.Ma=e,!0;console.warn("WEBGL_shader_pixel_local_storage is advertised, but a deprecated version has been detected. Disabling.")}return!1},endPixelLocalStorageWEBGL:function(d,e,l){(d=bd[d].I.Ma)&&d.endPixelLocalStorageWEBGL(new Uint32Array(Xa.buffer,4*l,e))},ensure_images_map:function(){D.images||(D.images=new Map)},environ_get:(d,e)=>{var l=0,t=0,x;for(x of nd()){var z=
e+l;W[d+t>>2]=z;l+=xc(x,z,Infinity)+1;t+=4}return 0},environ_sizes_get:(d,e)=>{var l=nd();W[d>>2]=l.length;d=0;for(var t of l)d+=yc(t)+1;W[e>>2]=d;return 0},fd_close:()=>52,fd_read:()=>52,fd_seek:function(){return 70},fd_write:(d,e,l,t)=>{for(var x=0,z=0;z<l;z++){var y=W[e>>2],B=W[e+4>>2];e+=8;for(var F=0;F<B;F++){var I=d,u=Q[y+F],a=od[I];0===u||10===u?((1===I?Ka:La)(pb(a)),a.length=0):a.push(u)}x+=B}W[t>>2]=x;return 0},framebufferPixelLocalClearValuefvWEBGL:function(d,e,l,t,x,z){(d=bd[d].I.Ma)&&
d.framebufferPixelLocalClearValuefvWEBGL(e,[l,t,x,z])},framebufferTexturePixelLocalStorageWEBGL:function(d,e,l,t,x,z){(d=bd[d].I.Ma)&&d.framebufferTexturePixelLocalStorageWEBGL(e,Zc[l],t,x,z)},glActiveTexture:d=>Z.activeTexture(d),glAttachShader:(d,e)=>{Z.attachShader(Tc[d],$c[e])},glBindBuffer:(d,e)=>{35051==d?Z.bb=e:35052==d&&(Z.Ca=e);Z.bindBuffer(d,Sc[e])},glBindBufferRange:(d,e,l,t,x)=>{Z.bindBufferRange(d,e,Sc[l],t,x)},glBindFramebuffer:(d,e)=>{Z.bindFramebuffer(d,Uc[e])},glBindRenderbuffer:(d,
e)=>{Z.bindRenderbuffer(d,Yc[e])},glBindSampler:(d,e)=>{Z.bindSampler(d,cd[e])},glBindTexture:(d,e)=>{Z.bindTexture(d,Zc[e])},glBindVertexArray:d=>{Z.bindVertexArray(ad[d])},glBlendEquation:d=>Z.blendEquation(d),glBlendFunc:(d,e)=>Z.blendFunc(d,e),glBlitFramebuffer:(d,e,l,t,x,z,y,B,F,I)=>Z.blitFramebuffer(d,e,l,t,x,z,y,B,F,I),glBufferData:(d,e,l,t)=>{l&&e?Z.bufferData(d,Q,t,l,e):Z.bufferData(d,e,t)},glBufferSubData:(d,e,l,t)=>{l&&Z.bufferSubData(d,e,Q,t,l)},glClear:d=>Z.clear(d),glClearBufferfv:(d,
e,l)=>{Z.clearBufferfv(d,e,Ta,l>>2)},glClearBufferuiv:(d,e,l)=>{Z.clearBufferuiv(d,e,W,l>>2)},glClearColor:(d,e,l,t)=>Z.clearColor(d,e,l,t),glClearDepthf:d=>Z.clearDepth(d),glClearStencil:d=>Z.clearStencil(d),glColorMask:(d,e,l,t)=>{Z.colorMask(!!d,!!e,!!l,!!t)},glCompileShader:d=>{Z.compileShader($c[d])},glCompressedTexSubImage2D:(d,e,l,t,x,z,y,B,F)=>{Z.Ca||!B?Z.compressedTexSubImage2D(d,e,l,t,x,z,y,B,F):Z.compressedTexSubImage2D(d,e,l,t,x,z,y,Q,F,B)},glCreateProgram:()=>{var d=gd(Tc),e=Z.createProgram();
e.name=d;e.Ka=e.Ia=e.Ja=0;e.mb=1;Tc[d]=e;return d},glCreateShader:d=>{var e=gd($c);$c[e]=Z.createShader(d);return e},glCullFace:d=>Z.cullFace(d),glDeleteBuffers:(d,e)=>{for(var l=0;l<d;l++){var t=S[e+4*l>>2],x=Sc[t];x&&(Z.deleteBuffer(x),x.name=0,Sc[t]=null,t==Z.bb&&(Z.bb=0),t==Z.Ca&&(Z.Ca=0))}},glDeleteFramebuffers:(d,e)=>{for(var l=0;l<d;++l){var t=S[e+4*l>>2],x=Uc[t];x&&(Z.deleteFramebuffer(x),x.name=0,Uc[t]=null)}},glDeleteProgram:d=>{if(d){var e=Tc[d];e?(Z.deleteProgram(e),e.name=0,Tc[d]=null):
hd||=1281}},glDeleteRenderbuffers:(d,e)=>{for(var l=0;l<d;l++){var t=S[e+4*l>>2],x=Yc[t];x&&(Z.deleteRenderbuffer(x),x.name=0,Yc[t]=null)}},glDeleteShader:d=>{if(d){var e=$c[d];e?(Z.deleteShader(e),$c[d]=null):hd||=1281}},glDeleteTextures:(d,e)=>{for(var l=0;l<d;l++){var t=S[e+4*l>>2],x=Zc[t];x&&(Z.deleteTexture(x),x.name=0,Zc[t]=null)}},glDeleteVertexArrays:(d,e)=>{for(var l=0;l<d;l++){var t=S[e+4*l>>2];Z.deleteVertexArray(ad[t]);ad[t]=null}},glDepthFunc:d=>Z.depthFunc(d),glDepthMask:d=>{Z.depthMask(!!d)},
glDepthRangef:(d,e)=>Z.depthRange(d,e),glDisable:d=>Z.disable(d),glDrawArrays:(d,e,l)=>{Z.drawArrays(d,e,l)},glDrawArraysInstanced:(d,e,l,t)=>{Z.drawArraysInstanced(d,e,l,t)},glDrawBuffers:(d,e)=>{for(var l=pd[d],t=0;t<d;t++)l[t]=S[e+4*t>>2];Z.drawBuffers(l)},glDrawElements:(d,e,l,t)=>{Z.drawElements(d,e,l,t)},glDrawElementsInstanced:(d,e,l,t,x)=>{Z.drawElementsInstanced(d,e,l,t,x)},glEnable:d=>Z.enable(d),glEnableVertexAttribArray:d=>{Z.enableVertexAttribArray(d)},glFlush:()=>Z.flush(),glFramebufferRenderbuffer:(d,
e,l,t)=>{Z.framebufferRenderbuffer(d,e,l,Yc[t])},glFramebufferTexture2D:(d,e,l,t,x)=>{Z.framebufferTexture2D(d,e,l,Zc[t],x)},glFrontFace:d=>Z.frontFace(d),glGenBuffers:(d,e)=>{jd(d,e,"createBuffer",Sc)},glGenFramebuffers:(d,e)=>{jd(d,e,"createFramebuffer",Uc)},glGenRenderbuffers:(d,e)=>{jd(d,e,"createRenderbuffer",Yc)},glGenTextures:(d,e)=>{jd(d,e,"createTexture",Zc)},glGenVertexArrays:(d,e)=>{jd(d,e,"createVertexArray",ad)},glGenerateMipmap:d=>Z.generateMipmap(d),glGetFramebufferAttachmentParameteriv:(d,
e,l,t)=>{d=Z.getFramebufferAttachmentParameter(d,e,l);if(d instanceof WebGLRenderbuffer||d instanceof WebGLTexture)d=d.name|0;S[t>>2]=d},glGetIntegerv:(d,e)=>rd(d,e),glGetProgramiv:(d,e,l)=>{if(l)if(d>=Rc)hd||=1281;else if(d=Tc[d],35716==e)d=Z.getProgramInfoLog(d),null===d&&(d="(unknown error)"),S[l>>2]=d.length+1;else if(35719==e){if(!d.Ka){var t=Z.getProgramParameter(d,35718);for(e=0;e<t;++e)d.Ka=Math.max(d.Ka,Z.getActiveUniform(d,e).name.length+1)}S[l>>2]=d.Ka}else if(35722==e){if(!d.Ia)for(t=
Z.getProgramParameter(d,35721),e=0;e<t;++e)d.Ia=Math.max(d.Ia,Z.getActiveAttrib(d,e).name.length+1);S[l>>2]=d.Ia}else if(35381==e){if(!d.Ja)for(t=Z.getProgramParameter(d,35382),e=0;e<t;++e)d.Ja=Math.max(d.Ja,Z.getActiveUniformBlockName(d,e).length+1);S[l>>2]=d.Ja}else S[l>>2]=Z.getProgramParameter(d,e);else hd||=1281},glGetString:d=>{var e=dd[d];if(!e){switch(d){case 7939:e=td(qd().join(" "));break;case 7936:case 7937:case 37445:case 37446:(e=Z.getParameter(d))||(hd||=1280);e=e?td(e):0;break;case 7938:e=
td(`OpenGL ES 3.0 (${Z.getParameter(7938)})`);break;case 35724:e=Z.getParameter(35724);var l=e.match(/^WebGL GLSL ES ([0-9]\.[0-9][0-9]?)(?:$| .*)/);null!==l&&(3==l[1].length&&(l[1]+="0"),e=`OpenGL ES GLSL ES ${l[1]} (${e})`);e=td(e);break;default:hd||=1280}dd[d]=e}return e},glGetUniformBlockIndex:(d,e)=>Z.getUniformBlockIndex(Tc[d],e?pb(Q,e):""),glGetUniformLocation:(d,e)=>{e=e?pb(Q,e):"";if(d=Tc[d]){var l=d,t=l.sa,x=l.Tb,z;if(!t){l.sa=t={};l.Sb={};var y=Z.getProgramParameter(l,35718);for(z=0;z<
y;++z){var B=Z.getActiveUniform(l,z);var F=B.name;B=B.size;var I=ud(F);I=0<I?F.slice(0,I):F;var u=l.mb;l.mb+=B;x[I]=[B,u];for(F=0;F<B;++F)t[u]=F,l.Sb[u++]=I}}l=d.sa;t=0;x=e;z=ud(e);0<z&&(t=parseInt(e.slice(z+1))>>>0,x=e.slice(0,z));if((x=d.Tb[x])&&t<x[0]&&(t+=x[1],l[t]=l[t]||Z.getUniformLocation(d,e)))return t}else hd||=1281;return-1},glInvalidateFramebuffer:(d,e,l)=>{for(var t=pd[e],x=0;x<e;x++)t[x]=S[l+4*x>>2];Z.invalidateFramebuffer(d,t)},glLinkProgram:d=>{d=Tc[d];Z.linkProgram(d);d.sa=0;d.Tb=
{}},glPixelStorei:(d,e)=>{3317==d?ed=e:3314==d&&(fd=e);Z.pixelStorei(d,e)},glReadPixels:(d,e,l,t,x,z,y)=>{if(Z.bb)Z.readPixels(d,e,l,t,x,z,y);else{var B=vd(z);y>>>=31-Math.clz32(B.BYTES_PER_ELEMENT);Z.readPixels(d,e,l,t,x,z,B,y)}},glRenderbufferStorage:(d,e,l,t)=>Z.renderbufferStorage(d,e,l,t),glRenderbufferStorageMultisample:(d,e,l,t,x)=>Z.renderbufferStorageMultisample(d,e,l,t,x),glScissor:(d,e,l,t)=>Z.scissor(d,e,l,t),glShaderSource:(d,e,l,t)=>{for(var x="",z=0;z<e;++z){var y=(y=W[l+4*z>>2])?pb(Q,
y,t?W[t+4*z>>2]:void 0):"";x+=y}Z.shaderSource($c[d],x)},glStencilFunc:(d,e,l)=>Z.stencilFunc(d,e,l),glStencilFuncSeparate:(d,e,l,t)=>Z.stencilFuncSeparate(d,e,l,t),glStencilMask:d=>Z.stencilMask(d),glStencilOp:(d,e,l)=>Z.stencilOp(d,e,l),glStencilOpSeparate:(d,e,l,t)=>Z.stencilOpSeparate(d,e,l,t),glTexParameteri:(d,e,l)=>Z.texParameteri(d,e,l),glTexStorage2D:(d,e,l,t,x)=>Z.texStorage2D(d,e,l,t,x),glTexStorage3D:(d,e,l,t,x,z)=>Z.texStorage3D(d,e,l,t,x,z),glTexSubImage2D:(d,e,l,t,x,z,y,B,F)=>{if(Z.Ca)Z.texSubImage2D(d,
e,l,t,x,z,y,B,F);else if(F){var I=vd(B);Z.texSubImage2D(d,e,l,t,x,z,y,B,I,F>>>31-Math.clz32(I.BYTES_PER_ELEMENT))}else{if(F){I=vd(B);var u=z*((fd||x)*({5:3,6:4,8:2,29502:3,29504:4,26917:2,26918:2,29846:3,29847:4}[y-6402]||1)*I.BYTES_PER_ELEMENT+ed-1&-ed);F=I.subarray(F>>>31-Math.clz32(I.BYTES_PER_ELEMENT),F+u>>>31-Math.clz32(I.BYTES_PER_ELEMENT))}else F=null;Z.texSubImage2D(d,e,l,t,x,z,y,B,F)}},glUniform1i:(d,e)=>{var l=Z,t=l.uniform1i;var x=Z.pc;if(x){var z=x.sa[d];"number"==typeof z&&(x.sa[d]=z=
Z.getUniformLocation(x,x.Sb[d]+(0<z?`[${z}]`:"")));d=z}else hd||=1282,d=void 0;t.call(l,d,e)},glUniformBlockBinding:(d,e,l)=>{d=Tc[d];Z.uniformBlockBinding(d,e,l)},glUseProgram:d=>{d=Tc[d];Z.useProgram(d);Z.pc=d},glVertexAttribDivisor:(d,e)=>{Z.vertexAttribDivisor(d,e)},glVertexAttribIPointer:(d,e,l,t,x)=>{Z.vertexAttribIPointer(d,e,l,t,x)},glVertexAttribPointer:(d,e,l,t,x,z)=>{Z.vertexAttribPointer(d,e,l,!!t,x,z)},glViewport:(d,e,l,t)=>Z.viewport(d,e,l,t),invoke_vii:Id,isWindowsBrowser:function(){return-1<
navigator.platform.indexOf("Win")},provokingVertexWEBGL:function(d,e){(d=bd[d].I.Lb)&&d.provokingVertexWEBGL(e)},riveWebCall:function(d,e,l,t,x,z,y){return D.riveScripting.call(d,e,l,t,x,z,y)},riveWebHasExport:function(d,e){return D.riveScripting.Dc(d,e)},riveWebMemoryPages:function(d){return D.riveScripting.Mc(d)},riveWebPreparing:function(d){return D.riveScripting.ba(d)},riveWebRaise:function(d,e){D.riveScripting.Pc(d,e)},riveWebReadMemory:function(d,e,l,t){return D.riveScripting.read(d,e,l,t)},
riveWebRegister:function(d,e,l){const t=D.riveScripting;if(!t)return 0;t.ic({exports:D,Fc:x=>{x=W[(Fd()>>2)+x];return 0===x?void 0:hc.get(x)},ia:()=>Q,Eb:()=>Ua,ra:lb,Na:xd,qa:jb,gb:sd,cb:Ab,nb:x=>x?pb(Q,x):"",Wb:(x,z,y)=>xc(x,z,y)});t.register(d,Q.subarray(e,e+l));return 1},riveWebRelease:function(d){D.riveScripting?.release(d)},riveWebStart:function(d){d=D.riveScripting.start(d);return null===d?0:td(d)},riveWebStringLength:function(d,e){return D.riveScripting.Ji(d,e)},riveWebWriteMemory:function(d,
e,l,t){return D.riveScripting.write(d,e,l,t)},upload_image:function(d,e){var l=D.images;l&&(e=l.get(e))&&(d=bd[d].I,d.pixelStorei(d.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!0),d.texImage2D(d.TEXTURE_2D,0,d.RGBA,d.RGBA,d.UNSIGNED_BYTE,e),d.pixelStorei(d.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!1))},wasm_start_image_decode:function(d,e,l){e=new Uint8Array(Xa.buffer,e,l);var t=new Uint8Array(l);t.set(e);Promise.resolve().then(function(){return createImageBitmap(new Blob([t]))}).then(function(x){var z=(new OffscreenCanvas(x.width,
x.height)).getContext("2d");z.drawImage(x,0,0);z=z.getImageData(0,0,x.width,x.height);var y=z.data.length,B=Cd(y);B?((new Uint8Array(Xa.buffer,B,y)).set(z.data),Dd(d,x.width,x.height,B,y)):Ed(d)}).catch(function(){Ed(d)})}};function Id(d,e,l){var t=kb();try{hc.get(d)(e,l)}catch(x){ib(t);if(x!==x+0)throw x;Gd(1,0)}}var Kd;
Kd=await (async function(){function d(l){l=Kd=l.exports;Ab=l.free;sd=l.malloc;D._setWebImage=l.setWebImage;zb=l.__getTypeName;Cd=D._wasm_image_decode_alloc=l.wasm_image_decode_alloc;Dd=D._wasm_image_decode_complete=l.wasm_image_decode_complete;Ed=D._wasm_image_decode_error=l.wasm_image_decode_error;D._rive_web_vm_module_export=l.rive_web_vm_module_export;D._rive_web_vm_booting=l.rive_web_vm_booting;Fd=D._rive_web_calls=l.rive_web_calls;yd=D._ma_device__on_notification_unlocked=l.ma_device__on_notification_unlocked;
D._ma_malloc_emscripten=l.ma_malloc_emscripten;D._ma_free_emscripten=l.ma_free_emscripten;zd=D._ma_device_process_pcm_frames_capture__webaudio=l.ma_device_process_pcm_frames_capture__webaudio;Ad=D._ma_device_process_pcm_frames_playback__webaudio=l.ma_device_process_pcm_frames_playback__webaudio;Gd=l.setThrew;Hd=l._emscripten_tempret_set;ib=l._emscripten_stack_restore;wd=l._emscripten_stack_alloc;kb=l.emscripten_stack_get_current;gc.vij=l.dynCall_vij;gc.iij=l.dynCall_iij;gc.ji=l.dynCall_ji;gc.iiiji=
l.dynCall_iiiji;gc.iiji=l.dynCall_iiji;gc.jii=l.dynCall_jii;gc.vijj=l.dynCall_vijj;gc.jiji=l.dynCall_jiji;Xa=l.memory;hc=l.__indirect_function_table;Wa();return Kd}var e={env:Jd,wasi_snapshot_preview1:Jd};if(D.instantiateWasm)return new Promise(l=>{D.instantiateWasm(e,(t,x)=>{l(d(t,x))})});Za??=D.locateFile?D.locateFile("canvas_advanced.wasm",Ga):Ga+"canvas_advanced.wasm";return d((await bb(e)).instance)}());
(function(){function d(){D.calledRun=!0;if(!Na){Va=!0;Kd.__wasm_call_ctors();Oa?.(D);D.onRuntimeInitialized?.();if(D.postRun)for("function"==typeof D.postRun&&(D.postRun=[D.postRun]);D.postRun.length;){var e=D.postRun.shift();fb.push(e)}eb(fb)}}if(D.preRun)for("function"==typeof D.preRun&&(D.preRun=[D.preRun]);D.preRun.length;)hb();eb(gb);D.setStatus?(D.setStatus("Running..."),setTimeout(()=>{setTimeout(()=>D.setStatus(""),1);d()},1)):d()})();Va?moduleRtn=D:moduleRtn=new Promise((d,e)=>{Oa=d;Pa=e});
;return moduleRtn}})();
/* harmony default export */ const __WEBPACK_DEFAULT_EXPORT__ = (Rive);


/***/ }),
/* 5 */
/***/ ((module) => {

module.exports = /*#__PURE__*/JSON.parse('{"name":"@rive-app/canvas","version":"2.44.2","description":"Rive\'s canvas based web api.","main":"rive.js","homepage":"https://rive.app","repository":{"type":"git","url":"https://github.com/rive-app/rive-wasm/tree/master/js"},"keywords":["rive","animation"],"author":"Rive","contributors":["Luigi Rosso <luigi@rive.app> (https://rive.app)","Maxwell Talbot <max@rive.app> (https://rive.app)","Arthur Vivian <arthur@rive.app> (https://rive.app)","Umberto Sonnino <umberto@rive.app> (https://rive.app)","Matthew Sullivan <matt.j.sullivan@gmail.com> (mailto:matt.j.sullivan@gmail.com)"],"license":"MIT","files":["rive.js","rive.js.map","rive.wasm","rive_fallback.wasm","rive.d.ts","rive_advanced.mjs.d.ts","runtimeLoader.d.ts","utils","semantics"],"typings":"rive.d.ts","dependencies":{},"browser":{"fs":false,"path":false}}');

/***/ }),
/* 6 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   AccessibilityOverlay: () => (/* reexport safe */ _accessibilityOverlay__WEBPACK_IMPORTED_MODULE_1__.AccessibilityOverlay),
/* harmony export */   CHECK_STATE_MASK: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.CHECK_STATE_MASK),
/* harmony export */   CHECK_STATE_OFFSET: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.CHECK_STATE_OFFSET),
/* harmony export */   SemanticActionType: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.SemanticActionType),
/* harmony export */   SemanticCheckState: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.SemanticCheckState),
/* harmony export */   SemanticMode: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.SemanticMode),
/* harmony export */   SemanticRole: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole),
/* harmony export */   SemanticState: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState),
/* harmony export */   SemanticTrait: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait),
/* harmony export */   SemanticTreeModel: () => (/* reexport safe */ _semanticTreeModel__WEBPACK_IMPORTED_MODULE_0__.SemanticTreeModel),
/* harmony export */   checkStateOf: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.checkStateOf),
/* harmony export */   hasState: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.hasState),
/* harmony export */   hasTrait: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.hasTrait),
/* harmony export */   roleName: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.roleName),
/* harmony export */   stateNames: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.stateNames),
/* harmony export */   traitNames: () => (/* reexport safe */ _types__WEBPACK_IMPORTED_MODULE_2__.traitNames)
/* harmony export */ });
/* harmony import */ var _semanticTreeModel__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(7);
/* harmony import */ var _accessibilityOverlay__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(9);
/* harmony import */ var _types__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(8);





/***/ }),
/* 7 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   SemanticTreeModel: () => (/* binding */ SemanticTreeModel)
/* harmony export */ });
/* harmony import */ var _types__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(8);
var __spreadArray = (undefined && undefined.__spreadArray) || function (to, from, pack) {
    if (pack || arguments.length === 2) for (var i = 0, l = from.length, ar; i < l; i++) {
        if (ar || !(i in from)) {
            if (!ar) ar = Array.prototype.slice.call(from, 0, i);
            ar[i] = from[i];
        }
    }
    return to.concat(ar || Array.prototype.slice.call(from));
};

/**
 * Maintains an in-memory semantic tree built from incremental
 * {@link SemanticsDiff} updates received each frame from the WASM runtime.
 *
 * Processing order within {@link applyDiff} follows the contract defined in
 * `semantic_snapshot.hpp`: removed → added → moved → childrenUpdated →
 * updatedSemantic → updatedGeometry.
 */
var SemanticTreeModel = /** @class */ (function () {
    function SemanticTreeModel() {
        this._nodesById = new Map();
        this._roots = [];
        this._semanticVersion = 0;
        this._geometryVersion = 0;
        this._geometryChangedIds = new Set();
        this._semanticChangedIds = new Set();
        this._debug = false;
    }
    Object.defineProperty(SemanticTreeModel.prototype, "nodeCount", {
        get: function () {
            return this._nodesById.size;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(SemanticTreeModel.prototype, "semanticVersion", {
        /** Bumped when semantic content or tree structure changes. */
        get: function () {
            return this._semanticVersion;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(SemanticTreeModel.prototype, "geometryVersion", {
        /** Bumped when node bounds change without a semantic/structural change. */
        get: function () {
            return this._geometryVersion;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(SemanticTreeModel.prototype, "geometryChangedIds", {
        /** Node IDs whose bounds changed in the most recent {@link applyDiff}. */
        get: function () {
            return this._geometryChangedIds;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(SemanticTreeModel.prototype, "semanticChangedIds", {
        /**
         * Node IDs whose semantic fields (role/label/value/hint/flags/headingLevel)
         * changed in the most recent {@link applyDiff}. Structural changes (moves,
         * child reorders, removals) bump {@link semanticVersion} but don't mark
         * nodes here — element attributes don't depend on tree position.
         */
        get: function () {
            return this._semanticChangedIds;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(SemanticTreeModel.prototype, "roots", {
        /** Root node IDs in sibling order. */
        get: function () {
            return this._roots;
        },
        enumerable: false,
        configurable: true
    });
    /** Look up a node by its ID, or undefined if not in the tree. */
    SemanticTreeModel.prototype.nodeById = function (id) {
        return this._nodesById.get(id);
    };
    /** The node with SemanticState.Focused, or undefined. */
    SemanticTreeModel.prototype.focusedNode = function () {
        // Manual iteration: runs every frame and must stop at the first hit (the
        // TS target forbids for-of over Map iterators).
        var it = this._nodesById.values();
        for (var r = it.next(); !r.done; r = it.next()) {
            if ((0,_types__WEBPACK_IMPORTED_MODULE_0__.hasState)(r.value.stateFlags, _types__WEBPACK_IMPORTED_MODULE_0__.SemanticState.Focused))
                return r.value;
        }
        return;
    };
    /** Current index of a node among its siblings (or roots), or -1 if absent. */
    SemanticTreeModel.prototype.siblingIndexOf = function (id) {
        var node = this._nodesById.get(id);
        if (!node)
            return -1;
        if (node.parentId < 0)
            return this._roots.indexOf(id);
        var parent = this._nodesById.get(node.parentId);
        return parent ? parent.children.indexOf(id) : -1;
    };
    /** Detach a node from its current parent (or from roots). */
    SemanticTreeModel.prototype.detach = function (id) {
        var node = this._nodesById.get(id);
        if (!node)
            return;
        if (node.parentId < 0) {
            var idx = this._roots.indexOf(id);
            if (idx !== -1)
                this._roots.splice(idx, 1);
        }
        else {
            var parent_1 = this._nodesById.get(node.parentId);
            if (parent_1) {
                var idx = parent_1.children.indexOf(id);
                if (idx !== -1)
                    parent_1.children.splice(idx, 1);
            }
        }
    };
    /** Attach a node under a parent at a given sibling index (or as root). */
    SemanticTreeModel.prototype.attach = function (id, parentId, siblingIndex) {
        var node = this._nodesById.get(id);
        if (!node)
            return;
        if (parentId < 0) {
            node.parentId = -1;
            var idx = clamp(siblingIndex, 0, this._roots.length);
            this._roots.splice(idx, 0, id);
        }
        else {
            var parent_2 = this._nodesById.get(parentId);
            if (!parent_2) {
                node.parentId = -1;
                this._roots.push(id);
            }
            else {
                node.parentId = parentId;
                var idx = clamp(siblingIndex, 0, parent_2.children.length);
                parent_2.children.splice(idx, 0, id);
            }
        }
    };
    /** Recursively remove a node and all descendants. */
    SemanticTreeModel.prototype.removeSubtree = function (id) {
        var node = this._nodesById.get(id);
        if (!node)
            return;
        // Copy children array — we're mutating during traversal
        var kids = __spreadArray([], node.children, true);
        for (var _i = 0, kids_1 = kids; _i < kids_1.length; _i++) {
            var child = kids_1[_i];
            this.removeSubtree(child);
        }
        this.detach(id);
        this._nodesById.delete(id);
    };
    /**
     * Apply an incremental diff to the tree. Bumps version counters and notifies
     * listeners only when the tree actually changed.
     *
     * No-op diffs (field values identical to current model) do not bump
     * versions — the native side guards against emitting these, but applyDiff
     * defends its subscribers regardless.
     */
    SemanticTreeModel.prototype.applyDiff = function (diff) {
        var _a, _b;
        var _this = this;
        this._geometryChangedIds.clear();
        this._semanticChangedIds.clear();
        var semanticChanged = false;
        var geometryChanged = false;
        var markSemantic = function () {
            semanticChanged = true;
        };
        var markSemanticNode = function (id) {
            semanticChanged = true;
            _this._semanticChangedIds.add(id);
        };
        var markGeometry = function (id) {
            geometryChanged = true;
            _this._geometryChangedIds.add(id);
        };
        // 1. removed
        for (var _i = 0, _c = diff.removed; _i < _c.length; _i++) {
            var id = _c[_i];
            if (this._nodesById.has(id)) {
                this.removeSubtree(id);
                markSemantic();
            }
        }
        // 2. added
        for (var _d = 0, _e = diff.added; _d < _e.length; _d++) {
            var n = _e[_d];
            var existing = this._nodesById.get(n.id);
            if (existing) {
                if (semanticFieldsDiffer(existing, n)) {
                    applySemantic(existing, n);
                    markSemanticNode(n.id);
                }
                if (geometryFieldsDiffer(existing, n)) {
                    applyGeometry(existing, n);
                    markGeometry(n.id);
                }
            }
            else {
                this._nodesById.set(n.id, nodeFromDiff(n));
                markSemanticNode(n.id);
                markGeometry(n.id);
            }
            this.detach(n.id);
            this.attach(n.id, n.parentId, n.siblingIndex);
        }
        // 3. moved
        // The runtime emits a node as "moved" when its parentId OR siblingIndex
        // changes, so a reorder-only move (same parent, new index) is still a
        // structural/semantic change. Compare the actual position before and after
        // re-attaching so that geometry-only or no-op moves don't bump the semantic
        // version (which would defeat the semantic/geometry version split).
        for (var _f = 0, _g = diff.moved; _f < _g.length; _f++) {
            var n = _g[_f];
            var existing = this._nodesById.get(n.id);
            if (!existing)
                continue;
            var parentChanged = existing.parentId !== n.parentId;
            var oldIndex = this.siblingIndexOf(n.id);
            var geomChanged = geometryFieldsDiffer(existing, n);
            if (geomChanged) {
                applyGeometry(existing, n);
                markGeometry(n.id);
            }
            this.detach(n.id);
            this.attach(n.id, n.parentId, n.siblingIndex);
            if (parentChanged || this.siblingIndexOf(n.id) !== oldIndex) {
                markSemantic();
            }
        }
        // 4. childrenUpdated
        for (var _h = 0, _j = diff.childrenUpdated; _h < _j.length; _h++) {
            var update = _j[_h];
            if (update.parentId < 0) {
                var next = update.childIds.filter(function (id) { return _this._nodesById.has(id); });
                if (!arraysEqual(this._roots, next)) {
                    this._roots.length = 0;
                    (_a = this._roots).push.apply(_a, next);
                    for (var _k = 0, _l = this._roots; _k < _l.length; _k++) {
                        var id = _l[_k];
                        var node = this._nodesById.get(id);
                        if (node)
                            node.parentId = -1;
                    }
                    markSemantic();
                }
            }
            else {
                var parent_3 = this._nodesById.get(update.parentId);
                if (!parent_3)
                    continue;
                var next = update.childIds.filter(function (id) { return _this._nodesById.has(id); });
                if (!arraysEqual(parent_3.children, next)) {
                    parent_3.children.length = 0;
                    (_b = parent_3.children).push.apply(_b, next);
                    for (var _m = 0, _o = parent_3.children; _m < _o.length; _m++) {
                        var id = _o[_m];
                        var node = this._nodesById.get(id);
                        if (node)
                            node.parentId = update.parentId;
                    }
                    markSemantic();
                }
            }
        }
        // 5. updatedSemantic — semantic fields updated
        for (var _p = 0, _q = diff.updatedSemantic; _p < _q.length; _p++) {
            var n = _q[_p];
            var existing = this._nodesById.get(n.id);
            if (!existing)
                continue;
            if (!semanticFieldsDiffer(existing, n))
                continue;
            applySemantic(existing, n);
            markSemanticNode(n.id);
        }
        // 6. updatedGeometry — bounds of a semantic node updated
        for (var _r = 0, _s = diff.updatedGeometry; _r < _s.length; _r++) {
            var n = _s[_r];
            var existing = this._nodesById.get(n.id);
            if (!existing)
                continue;
            if (!geometryFieldsDiffer(existing, n))
                continue;
            applyGeometry(existing, n);
            markGeometry(n.id);
        }
        if (!semanticChanged && !geometryChanged)
            return;
        if (semanticChanged)
            this._semanticVersion++;
        if (geometryChanged)
            this._geometryVersion++;
        if (this._debug) {
            this.logDiff(diff, semanticChanged, geometryChanged);
        }
    };
    Object.defineProperty(SemanticTreeModel.prototype, "debug", {
        /** Enable/disable debug logging of diffs to the console. */
        set: function (enabled) {
            this._debug = enabled;
        },
        enumerable: false,
        configurable: true
    });
    SemanticTreeModel.prototype.logDiff = function (diff, semanticChanged, geometryChanged) {
        var lines = [
            "[rive:semantics] semantic v".concat(this._semanticVersion) +
                (geometryChanged ? " geometry v".concat(this._geometryVersion) : "") +
                (semanticChanged ? "" : " (geometry-only)"),
        ];
        for (var _i = 0, _a = diff.removed; _i < _a.length; _i++) {
            var id = _a[_i];
            lines.push("  - removed #".concat(id));
        }
        for (var _b = 0, _c = diff.added; _b < _c.length; _b++) {
            var n = _c[_b];
            lines.push("  + added #".concat(n.id, " ").concat((0,_types__WEBPACK_IMPORTED_MODULE_0__.roleName)(n.role)) +
                (n.label ? " \"".concat(n.label, "\"") : "") +
                " bounds:(".concat(n.minX.toFixed(1), ",").concat(n.minY.toFixed(1), ")-(").concat(n.maxX.toFixed(1), ",").concat(n.maxY.toFixed(1), ")") +
                " states=[".concat((0,_types__WEBPACK_IMPORTED_MODULE_0__.stateNames)(n.stateFlags), "]") +
                " traits=[".concat((0,_types__WEBPACK_IMPORTED_MODULE_0__.traitNames)(n.traitFlags), "]"));
        }
        for (var _d = 0, _e = diff.moved; _d < _e.length; _d++) {
            var n = _e[_d];
            lines.push("  ~ moved #".concat(n.id, " \u2192 parent=").concat(n.parentId, " idx=").concat(n.siblingIndex) +
                " bounds:(".concat(n.minX.toFixed(1), ",").concat(n.minY.toFixed(1), ")-(").concat(n.maxX.toFixed(1), ",").concat(n.maxY.toFixed(1), ")"));
        }
        for (var _f = 0, _g = diff.childrenUpdated; _f < _g.length; _f++) {
            var u = _g[_f];
            lines.push("  \u2195 children of ".concat(u.parentId < 0 ? "root" : "#" + u.parentId, ": [").concat(u.childIds.join(", "), "]"));
        }
        for (var _h = 0, _j = diff.updatedSemantic; _h < _j.length; _h++) {
            var n = _j[_h];
            lines.push("  \u270E semantic #".concat(n.id, " ").concat((0,_types__WEBPACK_IMPORTED_MODULE_0__.roleName)(n.role)) +
                (n.label ? " \"".concat(n.label, "\"") : "") +
                " states=[".concat((0,_types__WEBPACK_IMPORTED_MODULE_0__.stateNames)(n.stateFlags), "]") +
                " traits=[".concat((0,_types__WEBPACK_IMPORTED_MODULE_0__.traitNames)(n.traitFlags), "]"));
        }
        for (var _k = 0, _l = diff.updatedGeometry; _k < _l.length; _k++) {
            var n = _l[_k];
            lines.push("  \u229E geometry #".concat(n.id, " (").concat(n.minX.toFixed(1), ",").concat(n.minY.toFixed(1), ")-(").concat(n.maxX.toFixed(1), ",").concat(n.maxY.toFixed(1), ")"));
        }
        console.log(lines.join("\n"));
    };
    /**
     * Returns every node in depth-first order, paired with its depth level.
     * Useful for debug logging / rendering a flat list.
     */
    SemanticTreeModel.prototype.flattened = function () {
        var _this = this;
        var out = [];
        var walk = function (id, depth) {
            var node = _this._nodesById.get(id);
            if (!node)
                return;
            out.push({ depth: depth, node: node });
            for (var _i = 0, _a = node.children; _i < _a.length; _i++) {
                var child = _a[_i];
                walk(child, depth + 1);
            }
        };
        for (var _i = 0, _a = this._roots; _i < _a.length; _i++) {
            var root = _a[_i];
            walk(root, 0);
        }
        return out;
    };
    return SemanticTreeModel;
}());

function clamp(v, min, max) {
    return v < min ? min : v > max ? max : v;
}
function arraysEqual(a, b) {
    if (a.length !== b.length)
        return false;
    for (var i = 0; i < a.length; i++) {
        if (a[i] !== b[i])
            return false;
    }
    return true;
}
function nodeFromDiff(n) {
    return {
        id: n.id,
        parentId: -1,
        role: n.role,
        label: n.label,
        value: n.value,
        hint: n.hint,
        stateFlags: n.stateFlags,
        traitFlags: n.traitFlags,
        headingLevel: n.headingLevel,
        minX: n.minX,
        minY: n.minY,
        maxX: n.maxX,
        maxY: n.maxY,
        children: [],
    };
}
/** Compare role/label/value/hint/stateFlags/traitFlags/headingLevel. */
function semanticFieldsDiffer(a, b) {
    return (a.role !== b.role ||
        a.label !== b.label ||
        a.value !== b.value ||
        a.hint !== b.hint ||
        a.stateFlags !== b.stateFlags ||
        a.traitFlags !== b.traitFlags ||
        a.headingLevel !== b.headingLevel);
}
/** Compare only bounds (minX/minY/maxX/maxY). */
function geometryFieldsDiffer(a, b) {
    return (a.minX !== b.minX ||
        a.minY !== b.minY ||
        a.maxX !== b.maxX ||
        a.maxY !== b.maxY);
}
function applySemantic(target, src) {
    target.role = src.role;
    target.label = src.label;
    target.value = src.value;
    target.hint = src.hint;
    target.stateFlags = src.stateFlags;
    target.traitFlags = src.traitFlags;
    target.headingLevel = src.headingLevel;
}
function applyGeometry(target, src) {
    target.minX = src.minX;
    target.minY = src.minY;
    target.maxX = src.maxX;
    target.maxY = src.maxY;
}


/***/ }),
/* 8 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   CHECK_STATE_MASK: () => (/* binding */ CHECK_STATE_MASK),
/* harmony export */   CHECK_STATE_OFFSET: () => (/* binding */ CHECK_STATE_OFFSET),
/* harmony export */   SemanticActionType: () => (/* binding */ SemanticActionType),
/* harmony export */   SemanticCheckState: () => (/* binding */ SemanticCheckState),
/* harmony export */   SemanticMode: () => (/* binding */ SemanticMode),
/* harmony export */   SemanticRole: () => (/* binding */ SemanticRole),
/* harmony export */   SemanticState: () => (/* binding */ SemanticState),
/* harmony export */   SemanticTrait: () => (/* binding */ SemanticTrait),
/* harmony export */   checkStateOf: () => (/* binding */ checkStateOf),
/* harmony export */   hasState: () => (/* binding */ hasState),
/* harmony export */   hasTrait: () => (/* binding */ hasTrait),
/* harmony export */   roleName: () => (/* binding */ roleName),
/* harmony export */   stateNames: () => (/* binding */ stateNames),
/* harmony export */   traitNames: () => (/* binding */ traitNames)
/* harmony export */ });
// ---------------------------------------------------------------------------
// SemanticRole — mirrors rive::SemanticRole
// ---------------------------------------------------------------------------
var SemanticRole = {
    none: 0,
    button: 1,
    link: 2,
    checkbox: 3,
    switchControl: 4,
    slider: 5,
    textField: 6,
    text: 7,
    image: 8,
    group: 9,
    list: 10,
    listItem: 11,
    tab: 12,
    tabList: 13,
    dialog: 14,
    alertDialog: 15,
    radioGroup: 16,
    radioButton: 17,
};
// ---------------------------------------------------------------------------
// SemanticState — mirrors rive::SemanticState bitmask
//
// Bits 0-7 are trait-gated (only meaningful when the corresponding
// SemanticTrait is set). Bits 8-13 are non-trait states.
//
// Check state is the exception: it is a two-bit *field* at bits 2-3 rather
// than a flag, because a checkbox is tri-state. It is not a member of this
// table — read it with checkStateOf().
// ---------------------------------------------------------------------------
var SemanticState = {
    None: 0,
    // Trait-gated
    Expanded: 1 << 0, // requires Expandable
    Selected: 1 << 1, // requires Selectable
    // Bits 2-3 are the check state field, not independent flags -- see SemanticCheckState.
    Toggled: 1 << 4, // requires Toggleable
    Required: 1 << 5, // requires Requirable
    Disabled: 1 << 6, // requires Enablable
    Focused: 1 << 7, // requires Focusable
    // Non-trait
    Hidden: 1 << 8,
    LiveRegion: 1 << 9,
    ReadOnly: 1 << 10,
    Modal: 1 << 11,
    Obscured: 1 << 12,
    Multiline: 1 << 13,
};
function hasState(flags, state) {
    return (flags & state) !== 0;
}
// SemanticCheckState — mirrors rive::SemanticCheckState
//
// The tri-state of a checkable node. Occupies two bits of stateFlags at
// CHECK_STATE_OFFSET, so it is read as a value rather than tested as a flag.
// Only meaningful when the Checkable trait is set.
var SemanticCheckState = {
    Unchecked: 0,
    Checked: 1,
    Mixed: 2,
};
/** Bit offset of the check field within stateFlags. */
var CHECK_STATE_OFFSET = 2;
/** Mask of the check field within stateFlags. */
var CHECK_STATE_MASK = 3 << CHECK_STATE_OFFSET;
/**
 * Decodes the check field out of `flags`. The field is two bits. Returns 0, 1, or 2
 * per the mapping in SemanticCheckState.
 */
function checkStateOf(flags) {
    var value = (flags & CHECK_STATE_MASK) >> CHECK_STATE_OFFSET;
    return value >= SemanticCheckState.Mixed
        ? SemanticCheckState.Mixed
        : value;
}
/**
 * Controls when the instance builds semantic trees and accessibility overlays.
 *
 * - `disabled`: no semantics work.
 * - `enabled`: semantics and overlay are active immediately after load.
 */
var SemanticMode = {
    Disabled: "disabled",
    Enabled: "enabled",
};
// ---------------------------------------------------------------------------
// SemanticTrait — mirrors rive::SemanticTrait bitmask
//
// Traits declare what *capabilities* a node has. A state flag is only
// meaningful when its corresponding trait is set.
// ---------------------------------------------------------------------------
var SemanticTrait = {
    None: 0,
    Expandable: 1 << 0,
    Selectable: 1 << 1,
    Checkable: 1 << 2,
    Toggleable: 1 << 3,
    Requirable: 1 << 4,
    Enablable: 1 << 5,
    Focusable: 1 << 6,
};
function hasTrait(flags, trait) {
    return (flags & trait) !== 0;
}
// ---------------------------------------------------------------------------
// SemanticActionType — mirrors rive::SemanticActionType
// ---------------------------------------------------------------------------
var SemanticActionType = {
    tap: 0,
    increase: 1,
    decrease: 2,
};
// ---------------------------------------------------------------------------
// Helpers — readable names for bitmask flags
// ---------------------------------------------------------------------------
var _roleNames = {};
for (var _i = 0, _a = Object.entries(SemanticRole); _i < _a.length; _i++) {
    var _b = _a[_i], name_1 = _b[0], val = _b[1];
    _roleNames[val] = name_1;
}
var _stateEntries = Object.entries(SemanticState).filter(function (_a) {
    var v = _a[1];
    return v !== 0;
});
var _traitEntries = Object.entries(SemanticTrait).filter(function (_a) {
    var v = _a[1];
    return v !== 0;
});
function roleName(role) {
    var _a;
    return (_a = _roleNames[role]) !== null && _a !== void 0 ? _a : "unknown(".concat(role, ")");
}
function stateNames(flags) {
    if (flags === 0)
        return "none";
    var active = [];
    for (var _i = 0, _stateEntries_1 = _stateEntries; _i < _stateEntries_1.length; _i++) {
        var _a = _stateEntries_1[_i], name_2 = _a[0], bit = _a[1];
        if (flags & bit)
            active.push(name_2);
    }
    switch (checkStateOf(flags)) {
        case SemanticCheckState.Checked:
            active.push("Checked");
            break;
        case SemanticCheckState.Mixed:
            active.push("Mixed");
            break;
    }
    return active.join(", ") || "none";
}
function traitNames(flags) {
    if (flags === 0)
        return "none";
    var active = [];
    for (var _i = 0, _traitEntries_1 = _traitEntries; _i < _traitEntries_1.length; _i++) {
        var _a = _traitEntries_1[_i], name_3 = _a[0], bit = _a[1];
        if (flags & bit)
            active.push(name_3);
    }
    return active.join(", ") || "none";
}


/***/ }),
/* 9 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   AccessibilityOverlay: () => (/* binding */ AccessibilityOverlay)
/* harmony export */ });
/* harmony import */ var _claimedKeyEvents__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(10);
/* harmony import */ var _utils_canvasOffset__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(11);
/* harmony import */ var _types__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(8);



var warnedUnpositionedWrapper = false;
/**
 * Warns once per page when no positioned element sits at or inside the canvas's
 * nearest scroll container: the overlay then can't scroll with the canvas. Reads only.
 */
function warnIfWrapperUnpositioned(canvas) {
    if (warnedUnpositionedWrapper)
        return;
    // Null when detached (or display:none / fixed); nothing to judge yet.
    var offsetParent = canvas.offsetParent;
    if (!offsetParent)
        return;
    var scrolls = function (v) { return v === "auto" || v === "scroll" || v === "overlay"; };
    var scroller = null;
    var el = canvas.parentElement;
    // Bounds pathological nesting.
    var MAX_ANCESTOR_WALK = 32;
    for (var i = 0; el && el !== document.documentElement && i < MAX_ANCESTOR_WALK; i++, el = el.parentElement) {
        var style = getComputedStyle(el);
        if (scrolls(style.overflowX) || scrolls(style.overflowY)) {
            scroller = el;
            break;
        }
    }
    // Only the viewport scrolls: any offsetParent scrolls with the page.
    if (!scroller || scroller.contains(offsetParent))
        return;
    warnedUnpositionedWrapper = true;
    console.warn('[Rive] Semantics: give the element wrapping the <canvas> "position: relative" (or another positioned value) so the semantic overlay scrolls with the canvas. Without it, screen readers may scroll to stale positions when the canvas is inside a scrolling container.');
}
/**
 * Creates and manages an invisible DOM tree overlaying a Rive canvas. This is for
 * screen readers to discover and interact with the Rive content.
 *
 * Each semantic node in the {@link SemanticTreeModel} gets a corresponding
 * DOM element with appropriate ARIA role, states, and action handlers so
 * assistive technologies (i.e. screen readers) can discover
 * and interact with the Rive content.
 *
 * Each node receives a prefixed ID (`id=rive-{instanceId}-sem-{nodeId}`) to avoid host-page ID collisions.
 * The nodeID is Rive's semantic node ID from core runtime.
 * Each node is styled with `pointer-events: none`. Interactive nodes can receive
 * programmatic focus and keydown events without entering the browser Tab order.
 */
var AccessibilityOverlay = /** @class */ (function () {
    function AccessibilityOverlay(options) {
        var _this = this;
        var _a, _b;
        this.elements = new Map();
        /** Visually-hidden description spans keyed by node ID, referenced by aria-describedby. */
        this.descElements = new Map();
        this.lastSemanticVersion = -1;
        this.lastGeometryVersion = -1;
        /** Text elements whose fit-scale needs recomputing, batched per update (see flushTextGeometry). */
        this.pendingTextGeometry = [];
        /** Last measured box-size|text key per text element, to skip redundant re-measures. */
        this.textGeometryKeys = new WeakMap();
        /** Node data last passed to applyAttributes per element; see {@link setEditingHostNode}. */
        this.appliedNodes = new WeakMap();
        this.lastCanvasPositioning = {
            width: -1, height: -1, offsetTop: -1, offsetLeft: -1,
        };
        /**
         * Set when a ResizeObserver/window-resize signals the canvas geometry may have
         * changed, cleared once the transform is re-synced. Lets {@link needsUpdate}
         * report geometry changes without a per-frame `getBoundingClientRect()` reflow.
         * Starts true so the first update computes the transform.
         */
        this._geometryDirty = true;
        /** True while reconciling the DOM (reserved for future focus-sync guards). */
        this.isUpdating = false;
        /** Node whose element the editing host stands in for, or null. */
        this.editingHostNodeId = null;
        /** Semantic version last folded into the editing host's decoration. */
        this.editingHostSemanticVersion = -1;
        /**
         * Single child div of the overlay container that carries the artboard→CSS
         * transform. All semantic node elements are children of this div and express
         * their positions in raw artboard-space coordinates. The CSS transform on
         * this container maps artboard units to CSS pixels in one GPU pass — no
         * per-node matrix multiplication required.
         */
        this.transformContainer = null;
        this._artboardBounds = { minX: 0, minY: 0, maxX: 0, maxY: 0 };
        this.repositionTimer = null;
        this.canvasResizeObserver = null;
        this.parentResizeObserver = null;
        /**
         * Detects canvas *position* drift. See {@link observePosition}.
         */
        this.positionObserver = null;
        this._onWindowResize = function () { return _this.scheduleReposition(); };
        this.instanceId = options.instanceId;
        this.fireAction = options.fireAction;
        this.requestFocus = options.requestFocus;
        this.clearFocus = options.clearFocus;
        this.canvas = options.canvas;
        this.semanticsOptions = options.semanticsOptions;
        this.allowFocusInterrupt = (_a = options.allowFocusInterrupt) !== null && _a !== void 0 ? _a : false;
        this.isEditingHostFocused = (_b = options.isEditingHostFocused) !== null && _b !== void 0 ? _b : (function () { return false; });
        this.container = this.createContainer(options.canvas);
        this.attachPositionObservers();
        warnIfWrapperUnpositioned(options.canvas);
    }
    /**
     * Hide the stood-in node's element (no duplicate textbox for AT); re-derive the previous
     * one's aria-hidden.
     */
    AccessibilityOverlay.prototype.setEditingHostNode = function (nodeId) {
        if (this.editingHostNodeId === nodeId)
            return;
        var previous = this.editingHostNodeId;
        this.editingHostNodeId = nodeId;
        if (previous !== null) {
            var previousEl = this.elements.get(previous);
            // Re-derive rather than un-hide: the node may have become Hidden mid-session.
            if (previousEl) {
                var previousNode = this.appliedNodes.get(previousEl);
                if (previousNode && this.isAriaHidden(previousNode)) {
                    setAttr(previousEl, "aria-hidden", "true");
                }
                else {
                    removeAttr(previousEl, "aria-hidden");
                }
            }
        }
        if (nodeId !== null) {
            var el = this.elements.get(nodeId);
            if (el)
                setAttr(el, "aria-hidden", "true");
        }
    };
    /**
     * Describe the focused text field on the host and hide its element; no-op unless
     * semantics changed.
     * @param force - apply even if semantics are unchanged (session start).
     */
    AccessibilityOverlay.prototype.syncEditingHost = function (tree, host, force) {
        var _a;
        if (!force && tree.semanticVersion === this.editingHostSemanticVersion)
            return;
        this.editingHostSemanticVersion = tree.semanticVersion;
        var node = tree.focusedNode();
        if (node === undefined || node.role !== _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.textField) {
            // Nothing to describe; drop the stale description rather than misname the host.
            if (this.editingHostNodeId !== null) {
                host.resetDecoration();
                this.setEditingHostNode(null);
            }
            return;
        }
        // Every key is passed on every call, nulls included, so nothing survives from
        // the previously described field.
        host.setSecure((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(node.stateFlags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Obscured));
        host.decorate({
            "aria-label": node.label || null,
            "aria-readonly": (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(node.stateFlags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.ReadOnly) ? "true" : null,
            "aria-multiline": (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(node.stateFlags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Multiline) ? "true" : null,
            // Trait-gated to match applyAttributes.
            "aria-required": (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(node.traitFlags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Requirable) &&
                (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(node.stateFlags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Required)
                ? "true"
                : null,
            // The hint span applyAttributes maintains (update() runs earlier in the frame).
            "aria-describedby": (node.hint && ((_a = this.descElements.get(node.id)) === null || _a === void 0 ? void 0 : _a.id)) || null,
        });
        this.setEditingHostNode(node.id);
    };
    /** True while the editing host stands in for a node's element. */
    AccessibilityOverlay.prototype.hasEditingHost = function () {
        return this.editingHostNodeId !== null;
    };
    /**
     * Undo the stand-in; if the host still has DOM focus, move it to the focused node's
     * element so AT announces it.
     */
    AccessibilityOverlay.prototype.releaseEditingHost = function (tree, host) {
        var _a;
        host === null || host === void 0 ? void 0 : host.resetDecoration();
        this.setEditingHostNode(null);
        this.editingHostSemanticVersion = -1;
        if (!(host === null || host === void 0 ? void 0 : host.hasDomFocus()))
            return;
        var focusedNodeId = (_a = tree === null || tree === void 0 ? void 0 : tree.focusedNode()) === null || _a === void 0 ? void 0 : _a.id;
        if (focusedNodeId !== undefined)
            this.focusNodeElement(focusedNodeId);
    };
    /** Move DOM focus onto a node's element. No-op for a node with no element. */
    AccessibilityOverlay.prototype.focusNodeElement = function (nodeId) {
        var el = this.elements.get(nodeId);
        if (!el)
            return;
        this.focusElementInPlace(el);
    };
    // The overlay is positioned outside the canvas's scroll chain, so a plain focus()
    // would scroll the root instead; scroll the canvas into view through its real ancestors.
    AccessibilityOverlay.prototype.focusElementInPlace = function (el) {
        var _a, _b;
        this.syncContainerBeforeFocus();
        el.focus({ preventScroll: true });
        (_b = (_a = this.canvas).scrollIntoView) === null || _b === void 0 ? void 0 : _b.call(_a, {
            block: "nearest",
            inline: "nearest",
            behavior: "instant",
        });
        // The scroll moved the canvas; re-align now rather than after the observer throttle.
        this.syncContainerGeometry();
        this.observePosition();
    };
    AccessibilityOverlay.prototype.getSemanticOverlayContainer = function () {
        return this.container;
    };
    // ---- Container lifecycle ----
    // Keep the a11y tree overlay matched to the canvas's position and size:
    // 1. The canvas resized           — ResizeObserver
    // 2. The canvas's parent resized   — ResizeObserver
    // 3. The window resized            — resize event
    // 4. The canvas moved/drifted      — IntersectionObserver (see observePosition)
    AccessibilityOverlay.prototype.attachPositionObservers = function () {
        var _this = this;
        this.canvasResizeObserver = new ResizeObserver(function () { return _this.scheduleReposition(); });
        this.canvasResizeObserver.observe(this.canvas);
        var parent = this.canvas.parentElement;
        if (parent) {
            this.parentResizeObserver = new ResizeObserver(function () { return _this.scheduleReposition(); });
            this.parentResizeObserver.observe(parent);
        }
        window.addEventListener("resize", this._onWindowResize);
        this.observePosition();
    };
    /**
     * Arms an IntersectionObserver whose root box is bounded to the canvas, so it
     * fires when the canvas moves relative to the viewport — position drift that
     * no ResizeObserver reports. Lets us re-sync the overlay container on a move
     * instead of recalculating the canvas bounding box every frame.
     */
    AccessibilityOverlay.prototype.observePosition = function () {
        var _this = this;
        var _a;
        if (typeof IntersectionObserver === "undefined")
            return;
        (_a = this.positionObserver) === null || _a === void 0 ? void 0 : _a.disconnect();
        this.positionObserver = null;
        var rect = this.canvas.getBoundingClientRect();
        // Can't frame a zero-area element; it will re-arm on the next reposition.
        if (!rect.width || !rect.height)
            return;
        // Shrink the viewport root box down to exactly the canvas: a negative inset
        // from each viewport edge to the matching canvas edge, in CSS shorthand
        // order (top, right, bottom, left). Rounded so sub-pixel jitter doesn't trip
        // the 1.0 threshold.
        var insetToPx = function (v) { return "".concat(-Math.round(v), "px"); };
        var rootMargin = [
            rect.top, // top:    viewport top → canvas top
            window.innerWidth - rect.right, // right:  viewport right → canvas right
            window.innerHeight - rect.bottom, // bottom: viewport bottom → canvas bottom
            rect.left, // left:   viewport left → canvas left
        ]
            .map(insetToPx)
            .join(" ");
        // The observer emits an initial notification for the current (contained)
        // state; ignore that and only react to a subsequent move.
        var armed = false;
        this.positionObserver = new IntersectionObserver(function () {
            if (!armed) {
                armed = true;
                return;
            }
            _this.scheduleReposition();
        }, { threshold: 1.0, rootMargin: rootMargin });
        this.positionObserver.observe(this.canvas);
    };
    AccessibilityOverlay.prototype.scheduleReposition = function () {
        var _this = this;
        // A resize/move may have changed canvas size/scale/position; force a
        // transform recompute on the next frame's update.
        this._geometryDirty = true;
        if (this.repositionTimer !== null)
            return;
        this.repositionTimer = setTimeout(function () {
            _this.repositionTimer = null;
            _this.syncContainerGeometry();
            // Re-arm the position observer at the canvas's new location.
            _this.observePosition();
        }, 500); // Throttle to avoid rapid style recalculations
    };
    /** Returns true when the container's size changed (node transforms need recomputing). */
    AccessibilityOverlay.prototype.syncContainerGeometry = function () {
        var rect = this.canvas.getBoundingClientRect();
        var _a = (0,_utils_canvasOffset__WEBPACK_IMPORTED_MODULE_1__.canvasOffset)(this.canvas), top = _a.top, left = _a.left;
        var sizeChanged = rect.width !== this.lastCanvasPositioning.width ||
            rect.height !== this.lastCanvasPositioning.height;
        if (!sizeChanged &&
            top === this.lastCanvasPositioning.offsetTop &&
            left === this.lastCanvasPositioning.offsetLeft)
            return false;
        this.container.style.top = top + "px";
        this.container.style.left = left + "px";
        this.container.style.width = rect.width + "px";
        this.container.style.height = rect.height + "px";
        this.container.tabIndex = -1;
        this.lastCanvasPositioning.width = rect.width;
        this.lastCanvasPositioning.height = rect.height;
        this.lastCanvasPositioning.offsetTop = top;
        this.lastCanvasPositioning.offsetLeft = left;
        return sizeChanged;
    };
    /**
     * Re-align the container right before focusing one of its elements. The position
     * observer re-syncs after scrolling on a throttle; focusing an element while the
     * container is stale would make the browser scroll the page to where it sits.
     */
    AccessibilityOverlay.prototype.syncContainerBeforeFocus = function () {
        if (this.syncContainerGeometry())
            this._geometryDirty = true;
    };
    AccessibilityOverlay.prototype.createContainer = function (canvas) {
        var _a, _b;
        var container = document.createElement("div");
        container.id = "rive-a11y-".concat(this.instanceId);
        container.setAttribute("role", "region");
        container.setAttribute("aria-label", (_b = (_a = this.semanticsOptions) === null || _a === void 0 ? void 0 : _a.riveCanvasLabel) !== null && _b !== void 0 ? _b : "Rive animation");
        // Size to the canvas's CSS layout box, not the parent container.
        var rect = canvas.getBoundingClientRect();
        var _c = (0,_utils_canvasOffset__WEBPACK_IMPORTED_MODULE_1__.canvasOffset)(this.canvas), top = _c.top, left = _c.left;
        container.style.cssText = [
            "position:absolute",
            "top:".concat(top, "px"),
            "left:".concat(left, "px"),
            "line-height:normal",
            "font-size:16px",
            "width:".concat(rect.width, "px"),
            "height:".concat(rect.height, "px"),
            "overflow:hidden",
            "pointer-events:none",
            // Visually hidden but still in the accessibility tree.
            // `display:none` and `visibility:hidden` would hide from AT.
            "opacity:0",
        ].join(";");
        canvas.insertAdjacentElement("afterend", container);
        return container;
    };
    /**
     * Returns what changed since the last update, or null if nothing changed.
     *
     * Callers use this to avoid recomputing the (relatively expensive)
     * artboard→canvas transform on frames where only node bounds changed in the
     * tree: the transform only needs recomputing when `layoutChanged` is true.
     */
    AccessibilityOverlay.prototype.needsUpdate = function (tree) {
        var semanticChanged = tree.semanticVersion !== this.lastSemanticVersion;
        var nodeGeometryChanged = tree.geometryVersion !== this.lastGeometryVersion;
        var layoutChanged = this._geometryDirty || !this.transformContainer;
        if (!semanticChanged && !nodeGeometryChanged && !layoutChanged)
            return null;
        return { semanticChanged: semanticChanged, nodeGeometryChanged: nodeGeometryChanged, layoutChanged: layoutChanged };
    };
    /**
     * Update the overlay DOM to reflect the current state of the semantic tree.
     * Call once per frame after `applyDiff` when {@link needsUpdate} reports a
     * change, when layout/transform inputs are dirty, or when a fresh
     * `forwardMat` is supplied (even if the tree versions are unchanged).
     *
     * @param tree           The in-memory semantic tree model
     * @param forwardMat     Artboard→canvas-pixel transform from `computeAlignment`,
     *                       or null to reuse the existing CSS transform on the
     *                       transform container
     * @param dpr            Device pixel ratio used for the canvas backing store
     * @param artboardBounds The artboard's own bounding rectangle
     */
    AccessibilityOverlay.prototype.update = function (tree, forwardMat, dpr, artboardBounds, change) {
        var overlayChange = change !== null && change !== void 0 ? change : this.needsUpdate(tree);
        if (!overlayChange && forwardMat) {
            overlayChange = {
                semanticChanged: false,
                nodeGeometryChanged: false,
                layoutChanged: true,
            };
        }
        if (!overlayChange)
            return;
        this.performUpdate(tree, forwardMat, dpr, artboardBounds, overlayChange);
    };
    AccessibilityOverlay.prototype.performUpdate = function (tree, forwardMat, dpr, artboardBounds, change) {
        var _a;
        var semanticChanged = change.semanticChanged, nodeGeometryChanged = change.nodeGeometryChanged;
        // Per-node change sets only describe the most recent applyDiff. If more
        // than one semantic version elapsed since our last update (first build,
        // or a diff we never consumed), fall back to re-applying attributes on
        // every node rather than trusting an incomplete set.
        var reapplyAllAttributes = tree.semanticVersion - this.lastSemanticVersion > 1;
        this.lastSemanticVersion = tree.semanticVersion;
        this.lastGeometryVersion = tree.geometryVersion;
        this.isUpdating = true;
        this._artboardBounds = artboardBounds;
        // Container box + artboard transform only run when a fresh forwardMat was
        // supplied (layout/transform dirty). Node-only bounds updates reuse the
        // existing CSS transform and go through updateGeometryForChangedNodes.
        // Skipping syncContainerGeometry here on semantic-only frames avoids a
        // getBoundingClientRect() reflow on every animation frame. The throttled
        // scheduleReposition() path still keeps the container aligned when the page
        // layout shifts.
        if (forwardMat) {
            this.syncContainerGeometry();
            this.syncTransformContainer(forwardMat, dpr, artboardBounds);
            // Transform is now in sync with the latest layout.
            this._geometryDirty = false;
        }
        if (semanticChanged) {
            var rootEl = (_a = this.transformContainer) !== null && _a !== void 0 ? _a : this.container;
            var activeIds_1 = new Set();
            this.rebuildChildren(rootEl, tree.roots, tree, 0, // parentLeft in artboard space (transform container origin)
            0, // parentTop  in artboard space
            activeIds_1, reapplyAllAttributes);
            var staleIds_2 = [];
            this.elements.forEach(function (_el, id) {
                if (!activeIds_1.has(id))
                    staleIds_2.push(id);
            });
            for (var _i = 0, staleIds_1 = staleIds_2; _i < staleIds_1.length; _i++) {
                var id = staleIds_1[_i];
                var el = this.elements.get(id);
                if (el && el.parentNode)
                    el.parentNode.removeChild(el);
                this.elements.delete(id);
                var desc = this.descElements.get(id);
                if (desc && desc.parentNode)
                    desc.parentNode.removeChild(desc);
                this.descElements.delete(id);
            }
        }
        else if (nodeGeometryChanged) {
            this.updateGeometryForChangedNodes(tree);
        }
        // Text scaling needs layout reads; batch them into one pass after all
        // position/attribute writes so each frame forces at most one reflow.
        this.flushTextGeometry();
        this.isUpdating = false;
    };
    /** Remove the overlay from the DOM entirely. */
    AccessibilityOverlay.prototype.destroy = function () {
        var _a, _b, _c;
        if (this.repositionTimer !== null) {
            clearTimeout(this.repositionTimer);
            this.repositionTimer = null;
        }
        window.removeEventListener("resize", this._onWindowResize);
        (_a = this.canvasResizeObserver) === null || _a === void 0 ? void 0 : _a.disconnect();
        (_b = this.parentResizeObserver) === null || _b === void 0 ? void 0 : _b.disconnect();
        (_c = this.positionObserver) === null || _c === void 0 ? void 0 : _c.disconnect();
        if (this.container.parentNode) {
            this.container.parentNode.removeChild(this.container);
        }
        this.elements.clear();
        this.descElements.clear();
        this.pendingTextGeometry.length = 0;
    };
    // ---- Tree → DOM reconciliation ----
    /**
     * Reconcile a parent DOM element's children with an ordered list of
     * semantic node IDs. Creates, updates, and reorders elements as needed.
     *
     * Node positions are expressed in artboard-space coordinates. The CSS
     * transform on the transform container maps artboard units to CSS pixels,
     * so no per-node matrix multiplication is required here.
     *
     * @param parentArtboardLeft  Absolute artboard minX of the parent node (0 for roots)
     * @param parentArtboardTop   Absolute artboard minY of the parent node (0 for roots)
     */
    AccessibilityOverlay.prototype.rebuildChildren = function (parentEl, childIds, tree, parentArtboardLeft, parentArtboardTop, activeIds, applyAllAttributes) {
        for (var i = 0; i < childIds.length; i++) {
            var nodeId = childIds[i];
            var nodeData = tree.nodeById(nodeId);
            if (!nodeData)
                continue;
            activeIds.add(nodeId);
            var el = this.elements.get(nodeId);
            var isNew = !el;
            if (!el) {
                el = this.createElement(nodeData);
                this.elements.set(nodeId, el);
            }
            // Attributes are only applied to new elements and nodes whose semantic
            // fields changed in the latest diff. Skipping redundant setAttribute
            // calls eliminates WebKit AX notifications that knock VoiceOver off its
            // current element.
            if (isNew || applyAllAttributes || tree.semanticChangedIds.has(nodeId)) {
                this.applyAttributes(el, nodeData);
            }
            this.applyPosition(el, nodeData, parentArtboardLeft, parentArtboardTop);
            // Only touch the DOM tree if the element isn't already in the correct
            // position. Moving a focused element can blur it and knock AT off the
            // current node.
            var currentChild = parentEl.children[i];
            if (currentChild !== el) {
                if (currentChild) {
                    parentEl.insertBefore(el, currentChild);
                }
                else {
                    parentEl.appendChild(el);
                }
            }
            // Reflect the runtime Focused state into DOM focus. Runs after insertion
            // so the element is attached, and skips elements that already hold focus.
            // The cheap trait/state flag checks gate the (per-node) DOM queries below,
            // so the activeElement/closest/contains walks only run for a node that is
            // actually focusable + focused — not for every node on every frame.
            if ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(nodeData.traitFlags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Focusable) &&
                (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(nodeData.stateFlags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Focused)) {
                var active = document.activeElement;
                // While focus is inside one of our modal dialogs, don't pull it back
                // out to a background node.
                var focusedModal = active === null || active === void 0 ? void 0 : active.closest('[aria-modal="true"]');
                var trappedByModal = !!focusedModal &&
                    this.container.contains(focusedModal) &&
                    !focusedModal.contains(el);
                // Not while the proxy has focus (would steal the caret, drop the keyboard) nor onto
                // its aria-hidden stand-in.
                if (nodeId !== this.editingHostNodeId &&
                    active !== el &&
                    !trappedByModal &&
                    !this.isEditingHostFocused() &&
                    this.canMoveFocus()) {
                    // No scrollIntoView: this runs on semantic diffs and must not drag the page.
                    this.syncContainerBeforeFocus();
                    el.focus({ preventScroll: true });
                }
            }
            // Recurse into children with this node's absolute artboard position.
            if (nodeData.children.length > 0) {
                this.rebuildChildren(el, nodeData.children, tree, nodeData.minX, nodeData.minY, activeIds, applyAllAttributes);
            }
            // Focus a modal/alert dialog when it first appears (after children are
            // built so the focus target can be resolved from the subtree).
            if (isNew)
                this.autoFocusDialogOnAppear(el, nodeData, tree);
        }
    };
    /**
     * Reposition only the subtrees whose bounds changed in the latest diff.
     * Descendants are included because node CSS positions are parent-relative.
     */
    AccessibilityOverlay.prototype.updateGeometryForChangedNodes = function (tree) {
        var _a, _b;
        for (var _i = 0, _c = Array.from(tree.geometryChangedIds); _i < _c.length; _i++) {
            var nodeId = _c[_i];
            var nodeData = tree.nodeById(nodeId);
            if (!nodeData)
                continue;
            var parentLeft = 0;
            var parentTop = 0;
            var parentEl = (_a = this.transformContainer) !== null && _a !== void 0 ? _a : this.container;
            if (nodeData.parentId >= 0) {
                var parent_1 = tree.nodeById(nodeData.parentId);
                if (parent_1) {
                    parentLeft = parent_1.minX;
                    parentTop = parent_1.minY;
                    parentEl = (_b = this.elements.get(nodeData.parentId)) !== null && _b !== void 0 ? _b : parentEl;
                }
            }
            this.updateNodeGeometrySubtree(tree, nodeId, parentLeft, parentTop, parentEl);
        }
    };
    AccessibilityOverlay.prototype.updateNodeGeometrySubtree = function (tree, nodeId, parentArtboardLeft, parentArtboardTop, _parentEl) {
        var nodeData = tree.nodeById(nodeId);
        if (!nodeData)
            return;
        var el = this.elements.get(nodeId);
        if (!el)
            return;
        this.applyPosition(el, nodeData, parentArtboardLeft, parentArtboardTop);
        for (var _i = 0, _a = nodeData.children; _i < _a.length; _i++) {
            var childId = _a[_i];
            this.updateNodeGeometrySubtree(tree, childId, nodeData.minX, nodeData.minY, el);
        }
    };
    /**
     * Whether the overlay may move focus now. Following focus already inside this
     * instance is always allowed; pulling it in from the host page is gated behind
     * allowFocusInterrupt (from the Rive class).
     */
    AccessibilityOverlay.prototype.canMoveFocus = function () {
        var active = document.activeElement;
        var focusAlreadyInScope = active === this.canvas || this.container.contains(active);
        return focusAlreadyInScope || this.allowFocusInterrupt;
    };
    /**
     * Move focus into a newly appeared modal/alert dialog so screen readers
     * announce and read its content (web ATs don't auto-enter a freshly mounted
     * dialog). Skips when focus can't move (see canMoveFocus) or a descendant
     * already holds it. The dialog's aria-modal keeps focus trapped inside.
     */
    AccessibilityOverlay.prototype.autoFocusDialogOnAppear = function (el, node, tree) {
        var _a;
        if (!isModalDialogRole(node.role, node.stateFlags))
            return;
        if (!this.canMoveFocus())
            return;
        // A descendant already holds focus — don't override it.
        var active = document.activeElement;
        if (active && active !== el && el.contains(active))
            return;
        var target = (_a = this.routeDefaultFocusTarget(node, tree)) !== null && _a !== void 0 ? _a : el;
        if (!target.hasAttribute("tabindex"))
            target.setAttribute("tabindex", "-1");
        if (document.activeElement !== target)
            target.focus({ preventScroll: true });
    };
    /**
     * Resolve the element assistive technologies (AT) should focus on appearance. Walks the subtree
     * depth-first and returns the first focusable node's host element, else the
     * inner <span> of the first labeled leaf. Container and unlabeled nodes are
     * descended into but never focused. Returns null if nothing qualifies.
     */
    AccessibilityOverlay.prototype.routeDefaultFocusTarget = function (node, tree) {
        var _a;
        for (var _i = 0, _b = node.children; _i < _b.length; _i++) {
            var childId = _b[_i];
            var child = tree.nodeById(childId);
            if (!child)
                continue;
            var childEl = this.elements.get(childId);
            // Focusable node → its host element (even if it has children).
            if (childEl && isFocusableNode(child))
                return childEl;
            // Container or unlabeled node → descend without focusing it.
            if (child.children.length > 0 || !child.label) {
                var nested = this.routeDefaultFocusTarget(child, tree);
                if (nested)
                    return nested;
                continue;
            }
            // Labeled leaf → its inner label span.
            if (childEl) {
                return (_a = childEl.querySelector(":scope > span")) !== null && _a !== void 0 ? _a : childEl;
            }
        }
        return null;
    };
    Object.defineProperty(AccessibilityOverlay.prototype, "nodeIdPrefix", {
        // ---- Element creation ----
        /** Shared `id` prefix for all semantic node elements of this instance. */
        get: function () {
            return "rive-".concat(this.instanceId, "-sem-");
        },
        enumerable: false,
        configurable: true
    });
    /** Recover the semantic node ID from an overlay element, or null. */
    AccessibilityOverlay.prototype.nodeIdFromElement = function (el) {
        if (!el.id.startsWith(this.nodeIdPrefix))
            return null;
        var raw = el.id.slice(this.nodeIdPrefix.length);
        // Guard the empty string explicitly: Number("") is 0, not NaN.
        if (!raw)
            return null;
        var id = Number(raw);
        return Number.isNaN(id) ? null : id;
    };
    AccessibilityOverlay.prototype.createElement = function (node) {
        var tag = tagForRole(node.role);
        var el = document.createElement(tag);
        el.id = "".concat(this.nodeIdPrefix).concat(node.id);
        el.style.cssText = BASE_NODE_STYLE;
        if (node.role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.text) {
            // The positioned outer div is the AX element
            // VoiceOver uses for its highlight box; an inner <span> carries the text
            // without position:absolute (which would strip the span from VoiceOver's
            // bounds calculation).
            var textSpan = document.createElement("span");
            textSpan.style.cssText = SPAN_EXP;
            el.appendChild(textSpan);
        }
        this.attachActionHandlers(el, node);
        return el;
    };
    // ---- Action handlers ----
    /**
     * Wire arrow-key roving focus for a group member (tab, radio). Arrow keys
     * move focus to the next/previous member (wrapping), optionally Home/End jump
     * to first/last, and the newly focused member receives a tap action.
     */
    AccessibilityOverlay.prototype.attachRovingNav = function (el, opts) {
        var _this = this;
        el.addEventListener("keydown", function (e) {
            var target = null;
            if (e.key === "ArrowRight" || e.key === "ArrowDown")
                target = "next";
            else if (e.key === "ArrowLeft" || e.key === "ArrowUp")
                target = "prev";
            else if (opts.includeHomeEnd && e.key === "Home")
                target = "first";
            else if (opts.includeHomeEnd && e.key === "End")
                target = "last";
            if (!target)
                return;
            (0,_claimedKeyEvents__WEBPACK_IMPORTED_MODULE_0__.claimKeyEvent)(e);
            var members = opts.members();
            var idx = members.indexOf(el);
            if (idx < 0)
                return;
            var n = members.length;
            var next = target === "next" ? members[(idx + 1) % n]
                : target === "prev" ? members[(idx - 1 + n) % n]
                    : target === "first" ? members[0]
                        : members[n - 1];
            if (next && next !== el) {
                _this.focusElementInPlace(next);
                var nextId = _this.nodeIdFromElement(next);
                if (nextId !== null)
                    _this.fireAction(nextId, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticActionType.tap);
            }
        });
    };
    AccessibilityOverlay.prototype.attachActionHandlers = function (el, node) {
        var _this = this;
        var role = node.role;
        var nodeId = node.id;
        if (isClickableRole(role)) {
            el.addEventListener("click", function () {
                _this.fireAction(nodeId, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticActionType.tap);
            });
            // Links activate on Enter only (Space scrolls the page per browser
            // convention). All other clickable roles accept both Enter and Space.
            var activationKeys_1 = role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.link ? ["Enter"] : ["Enter", " "];
            el.addEventListener("keydown", function (e) {
                if (activationKeys_1.includes(e.key)) {
                    (0,_claimedKeyEvents__WEBPACK_IMPORTED_MODULE_0__.claimKeyEvent)(e);
                    _this.fireAction(nodeId, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticActionType.tap);
                }
            });
        }
        if (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.slider) {
            el.addEventListener("keydown", function (e) {
                if (e.key === "ArrowRight" || e.key === "ArrowUp") {
                    (0,_claimedKeyEvents__WEBPACK_IMPORTED_MODULE_0__.claimKeyEvent)(e);
                    _this.fireAction(nodeId, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticActionType.increase);
                }
                else if (e.key === "ArrowLeft" || e.key === "ArrowDown") {
                    (0,_claimedKeyEvents__WEBPACK_IMPORTED_MODULE_0__.claimKeyEvent)(e);
                    _this.fireAction(nodeId, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticActionType.decrease);
                }
            });
        }
        if (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.tab) {
            this.attachRovingNav(el, {
                includeHomeEnd: true,
                members: function () {
                    var parent = el.parentElement;
                    if (!parent)
                        return [];
                    return Array.from(parent.children).filter(function (c) {
                        return c instanceof HTMLElement && c.getAttribute("role") === "tab";
                    });
                },
            });
        }
        if (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.radioButton) {
            this.attachRovingNav(el, {
                includeHomeEnd: false,
                members: function () {
                    var _a;
                    var group = (_a = el.closest('[role="radiogroup"]')) !== null && _a !== void 0 ? _a : el.parentElement;
                    if (!group)
                        return [];
                    return Array.from(group.querySelectorAll('[role="radio"]'));
                },
            });
        }
        // Focus handler for nodes with the Focusable trait. When AT focuses an
        // element, notify the C++ runtime so it can update internal focus state
        // (visual focus rings, etc.). Gated on Focusable trait
        if ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(node.traitFlags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Focusable)) {
            el.addEventListener("focus", function () {
                _this.requestFocus(nodeId);
            });
        }
    };
    // ---- Attribute application ----
    AccessibilityOverlay.prototype.isAriaHidden = function (node) {
        var role = node.role;
        var flags = node.stateFlags;
        // Hide from AT when explicitly hidden, or when an image has no accessible
        // name — a nameless role="img" is a WCAG 1.1.1 violation; treat it as
        // decorative instead.
        var isDecorativeImage = role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.image && !node.label;
        // The <input> proxy is the accessible element for the node it stands in for.
        var isEditingHostStandIn = node.id === this.editingHostNodeId;
        return ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Hidden) ||
            // Obscured text fields stay reachable (value withheld, like a native password input).
            ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Obscured) && role !== _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.textField) ||
            isDecorativeImage ||
            isEditingHostStandIn);
    };
    AccessibilityOverlay.prototype.applyAttributes = function (el, node) {
        var _a, _b, _c;
        this.appliedNodes.set(el, node);
        var role = node.role;
        var flags = node.stateFlags;
        var traits = node.traitFlags;
        // Role
        var ariaRole = ariaRoleForSemantic(role);
        if (ariaRole) {
            setAttr(el, "role", ariaRole);
        }
        else {
            removeAttr(el, "role");
        }
        // Links: a bare <a> with no href has no link semantics, so set the role
        // explicitly (ariaRoleForSemantic returns null for link → native <a>).
        if (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.link) {
            setAttr(el, "role", "link");
        }
        // Tabindex — keep the screen-reader overlay out of the browser's sequential
        // Tab order. Interactive/focusable nodes remain programmatically focusable
        // so AT/runtime focus sync can still target them when needed. List items are
        // included because Mobile Safari won't iterate them otherwise.
        if (isInteractiveRole(role) ||
            (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(traits, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Focusable) ||
            role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.listItem) {
            setAttr(el, "tabindex", "-1");
        }
        else {
            removeAttr(el, "tabindex");
        }
        // Label / value / hint
        if (node.label) {
            setAttr(el, "aria-label", node.label);
        }
        else {
            removeAttr(el, "aria-label");
        }
        if (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.slider) {
            if (node.value) {
                // aria-valuenow must be numeric; keep the display string (e.g. "75%")
                // in aria-valuetext only.
                var numericValue = parseFloat(node.value);
                if (Number.isFinite(numericValue)) {
                    setAttr(el, "aria-valuenow", String(numericValue));
                }
                else {
                    removeAttr(el, "aria-valuenow");
                }
                setAttr(el, "aria-valuetext", node.value);
            }
            else {
                removeAttr(el, "aria-valuenow");
                removeAttr(el, "aria-valuetext");
            }
            // TODO: aria-valuemin / aria-valuemax are required by ARIA.
            // Defaulting to horizontal; vertical sliders need orientation data from C++.
            setAttr(el, "aria-orientation", "horizontal");
            setBoolAttr(el, "aria-readonly", (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.ReadOnly));
        }
        else {
            removeAttr(el, "aria-valuenow");
            removeAttr(el, "aria-valuetext");
            removeAttr(el, "aria-orientation");
            removeAttr(el, "aria-readonly");
        }
        if (node.hint) {
            var descId = "rive-".concat(this.instanceId, "-desc-").concat(node.id);
            var descEl = this.descElements.get(node.id);
            if (!descEl) {
                descEl = document.createElement("span");
                descEl.id = descId;
                descEl.style.cssText = DESC_SPAN_STYLE;
                this.container.appendChild(descEl);
                this.descElements.set(node.id, descEl);
            }
            if (descEl.textContent !== node.hint)
                descEl.textContent = node.hint;
            setAttr(el, "aria-describedby", descId);
        }
        else {
            removeAttr(el, "aria-describedby");
            var staleDesc = this.descElements.get(node.id);
            if (staleDesc) {
                if (staleDesc.parentNode)
                    staleDesc.parentNode.removeChild(staleDesc);
                this.descElements.delete(node.id);
            }
        }
        // Text nodes: use textContent rather than aria-label so screen readers
        // announce the text in virtual/browse mode (aria-label on a bare <span>
        // with no widget role is ignored by some AT in document browse mode).
        if (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.text) {
            var textSpan = (_a = el.querySelector(":scope > span")) !== null && _a !== void 0 ? _a : el;
            var text = (_b = node.label) !== null && _b !== void 0 ? _b : "";
            if (textSpan.textContent !== text)
                textSpan.textContent = text;
            removeAttr(el, "aria-label");
            if (node.headingLevel > 0) {
                setAttr(el, "role", "heading");
                setAttr(el, "aria-level", String(node.headingLevel));
            }
            else {
                // The generic role branch above already cleared role="heading";
                // aria-level must go with it when a heading reverts to plain text.
                removeAttr(el, "aria-level");
            }
        }
        // ---- Trait-gated states ----
        // Only set the ARIA property when the trait is present. When the trait
        // is absent, remove the attribute so AT sees "not applicable" rather
        // than "false".
        if ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(traits, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Expandable) && ARIA_EXPANDED_ROLES.has(role)) {
            setBoolAttr(el, "aria-expanded", (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Expanded));
        }
        else {
            removeAttr(el, "aria-expanded");
        }
        // aria-selected is required on ALL tabs per ARIA spec regardless of trait;
        // for other roles it is trait-gated and guarded to ARIA_SELECTED_ROLES.
        if (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.tab) {
            setBoolAttr(el, "aria-selected", (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Selected));
        }
        else if ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(traits, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Selectable) && ARIA_SELECTED_ROLES.has(role)) {
            setBoolAttr(el, "aria-selected", (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Selected));
        }
        else {
            removeAttr(el, "aria-selected");
        }
        if ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(traits, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Checkable) && ARIA_CHECKED_ROLES.has(role)) {
            // Check state is a tri-state field, but not every checkable role
            // accepts all three values — see ARIA_MIXED_ROLES.
            var checkState = (0,_types__WEBPACK_IMPORTED_MODULE_2__.checkStateOf)(flags);
            if (checkState === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticCheckState.Mixed &&
                ARIA_MIXED_ROLES.has(role)) {
                setAttr(el, "aria-checked", "mixed");
            }
            else {
                setBoolAttr(el, "aria-checked", checkState === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticCheckState.Checked);
            }
        }
        else {
            removeAttr(el, "aria-checked");
        }
        if ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(traits, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Toggleable)) {
            if (ARIA_PRESSED_ROLES.has(role)) {
                setBoolAttr(el, "aria-pressed", (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Toggled));
            }
            else {
                removeAttr(el, "aria-pressed");
            }
            // switch uses aria-checked (not aria-pressed) for its on/off state.
            if (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.switchControl) {
                setBoolAttr(el, "aria-checked", (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Toggled));
            }
        }
        else {
            removeAttr(el, "aria-pressed");
        }
        if ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(traits, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Requirable) && ARIA_REQUIRED_ROLES.has(role)) {
            setBoolAttr(el, "aria-required", (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Required));
        }
        else {
            removeAttr(el, "aria-required");
        }
        if ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(traits, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Enablable)) {
            setBoolAttr(el, "aria-disabled", (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Disabled));
        }
        else {
            removeAttr(el, "aria-disabled");
        }
        // ---- Non-trait states ----
        if (this.isAriaHidden(node)) {
            setAttr(el, "aria-hidden", "true");
        }
        else {
            removeAttr(el, "aria-hidden");
        }
        if ((0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.LiveRegion)) {
            setAttr(el, "aria-live", "polite");
        }
        else {
            removeAttr(el, "aria-live");
        }
        // textField-specific
        // TODO: Details here may change once we implement text inputs
        if (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.textField) {
            setBoolAttr(el, "aria-readonly", (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.ReadOnly));
            setBoolAttr(el, "aria-multiline", (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Multiline));
            // Surface the current field value as DOM text so screen readers can
            // announce it. Only safe when the node has no semantic children —
            // setting textContent would remove any child elements from the DOM.
            if (node.children.length === 0) {
                var value = (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Obscured) ? "" : ((_c = node.value) !== null && _c !== void 0 ? _c : "");
                if (el.textContent !== value)
                    el.textContent = value;
            }
            else {
                // A value rendered earlier (before the node had children) must not linger,
                // least of all once the field turns Obscured.
                for (var _i = 0, _d = Array.from(el.childNodes); _i < _d.length; _i++) {
                    var child = _d[_i];
                    if (child.nodeType === Node.TEXT_NODE)
                        child.remove();
                }
            }
        }
        else {
            removeAttr(el, "aria-multiline");
        }
        // alertDialog is always modal per WAI-ARIA; a plain dialog only when the Modal state flag is set.
        if (isModalDialogRole(role, flags)) {
            setAttr(el, "aria-modal", "true");
        }
        else {
            removeAttr(el, "aria-modal");
        }
    };
    // ---- Positioning ----
    /**
     * Positions an element in artboard-space coordinates relative to its parent.
     *
     * Node bounds stay in raw artboard units — the CSS `transform: matrix(...)`
     * on the transform container maps artboard units to CSS pixels in one GPU
     * pass. No per-node forwardMat multiplication or DPR division needed here.
     *
     * Round to whole artboard units before comparing to avoid triggering AX
     * layout notifications from sub-unit floating-point animation jitter.
     */
    AccessibilityOverlay.prototype.applyPosition = function (el, node, parentArtboardLeft, parentArtboardTop) {
        // Clamp each node's rect to the artboard viewport before computing CSS.
        // Without this, nodes whose artboard bounds exceed the artboard (e.g. a
        // scroll list container whose height is the full content, not the viewport)
        // produce CSS heights that overflow the transform container. WebKit then
        // unions all descendant AX rects and extends the container's AX origin
        // above the canvas regardless of overflow:hidden on ancestors.
        var ab = this._artboardBounds;
        var clampedMinX = Math.max(node.minX, ab.minX);
        var clampedMinY = Math.max(node.minY, ab.minY);
        var clampedMaxX = Math.min(node.maxX, ab.maxX);
        var clampedMaxY = Math.min(node.maxY, ab.maxY);
        var elLeft = clampedMinX - parentArtboardLeft;
        var elTop = clampedMinY - parentArtboardTop;
        var elWidth = Math.max(0, clampedMaxX - clampedMinX);
        var elHeight = Math.max(0, clampedMaxY - clampedMinY);
        var tx = Math.round(elLeft);
        var ty = Math.round(elTop);
        var pxWidth = Math.round(elWidth) + "px";
        var pxHeight = Math.round(elHeight) + "px";
        // Use left/top layout properties for positioning. VoiceOver reliably
        // handles layout position + a single ancestor CSS transform (the transform
        // container). Chaining CSS transforms across multiple stacking contexts
        // (transform container → node identity → item translate) caused VoiceOver
        // to compute the correct SIZE but wrong POSITION. Artboard clamping above
        // guarantees tx/ty are always ≥ 0, so the negative-top overflow problem
        // that originally prompted the switch to CSS transforms no longer applies.
        var pxLeft = tx + "px";
        var pxTop = ty + "px";
        if (el.style.left !== pxLeft)
            el.style.left = pxLeft;
        if (el.style.top !== pxTop)
            el.style.top = pxTop;
        if (el.style.width !== pxWidth)
            el.style.width = pxWidth;
        if (el.style.height !== pxHeight)
            el.style.height = pxHeight;
        // Clear any CSS transform from a previous render pass.
        if (el.style.transform)
            el.style.transform = "";
        if (node.role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.text) {
            this.pendingTextGeometry.push(el);
        }
    };
    /**
     * Scale each queued text span to fit its layout box, batched so a frame
     * pays at most one synchronous layout: all measurement-reset writes first,
     * then all rect reads, then all transform writes. Interleaving
     * write→read→write per node would force a reflow per text node instead.
     *
     * Nodes whose box size and text are unchanged since the last pass are
     * skipped entirely (their existing transform is still correct — the scale
     * is a ratio of two rects, so ancestor transform changes cancel out).
     */
    AccessibilityOverlay.prototype.flushTextGeometry = function () {
        var _a;
        if (this.pendingTextGeometry.length === 0)
            return;
        // Phase 1 — writes: reset spans to their natural size for measurement.
        // The previous pass's scale must be cleared, or the rect reads below
        // would measure the scaled span and compound the correction.
        var toMeasure = [];
        for (var _i = 0, _b = this.pendingTextGeometry; _i < _b.length; _i++) {
            var host = _b[_i];
            var span = (_a = host.querySelector(":scope > span")) !== null && _a !== void 0 ? _a : host;
            var key = "".concat(host.style.width, "|").concat(host.style.height, "|").concat(span.textContent);
            if (this.textGeometryKeys.get(host) === key)
                continue;
            span.style.width = "auto";
            span.style.height = "auto";
            span.style.transformOrigin = "0 0";
            span.style.transform = "";
            toMeasure.push({ host: host, span: span, key: key });
        }
        this.pendingTextGeometry.length = 0;
        // Phase 2 — reads: one layout pass covers every rect measurement.
        var transforms = toMeasure.map(function (_a) {
            var host = _a.host, span = _a.span;
            var parentRect = host.getBoundingClientRect();
            var natural = span.getBoundingClientRect();
            if (natural.width > 0 && natural.height > 0) {
                var scaleX = parentRect.width / natural.width;
                var scaleY = parentRect.height / natural.height;
                return "scale(".concat(scaleX, ", ").concat(scaleY, ")");
            }
            return "none";
        });
        // Phase 3 — writes: apply all transforms.
        for (var i = 0; i < toMeasure.length; i++) {
            var _c = toMeasure[i], host = _c.host, span = _c.span, key = _c.key;
            span.style.transform = transforms[i];
            this.textGeometryKeys.set(host, key);
        }
    };
    // ---- Transform container ----
    /**
     * Creates (on first call) and updates the artboard-space transform container.
     *
     * The container is sized to the artboard dimensions and carries a CSS
     * `transform: matrix(...)` that maps artboard coordinates to the canvas's CSS box
     * (forwardMat in backing-store pixels, scaled by CSS size / backing-store size). All semantic
     * node elements are children of this container and use raw artboard
     * coordinates as their CSS `left/top/width/height`, so the CSS compositor
     * applies the artboard→screen mapping in one pass.
     */
    AccessibilityOverlay.prototype.syncTransformContainer = function (forwardMat, dpr, artboardBounds) {
        if (!this.transformContainer) {
            var tc = document.createElement("div");
            tc.style.cssText = [
                "position:absolute",
                "top:0",
                "left:0",
                // overflow:visible — artboard viewport clamping is done per-node in
                // applyPosition
                "overflow:visible",
                "pointer-events:none",
                "transform-origin:0 0",
            ].join(";");
            this.container.appendChild(tc);
            this.transformContainer = tc;
        }
        var w = artboardBounds.maxX - artboardBounds.minX;
        var h = artboardBounds.maxY - artboardBounds.minY;
        this.transformContainer.style.width = Math.round(w) + "px";
        this.transformContainer.style.height = Math.round(h) + "px";
        // Backing-store pixels → CSS pixels. Measured rather than 1/dpr: a canvas can be
        // stretched by CSS, or have a stale backing store after a resize, and the drawn
        // content follows the CSS box either way. Equal to 1/dpr when they match.
        var cssWidth = this.lastCanvasPositioning.width;
        var cssHeight = this.lastCanvasPositioning.height;
        var fallback = 1 / (dpr || 1);
        var sx = this.canvas.width > 0 && cssWidth > 0 ? cssWidth / this.canvas.width : fallback;
        var sy = this.canvas.height > 0 && cssHeight > 0 ? cssHeight / this.canvas.height : fallback;
        var a = forwardMat.xx * sx;
        var b = forwardMat.xy * sy;
        var c = forwardMat.yx * sx;
        var d = forwardMat.yy * sy;
        var tx = forwardMat.tx * sx;
        var ty = forwardMat.ty * sy;
        this.transformContainer.style.transform =
            "matrix(".concat(a, ",").concat(b, ",").concat(c, ",").concat(d, ",").concat(tx, ",").concat(ty, ")");
    };
    return AccessibilityOverlay;
}());

// ---------------------------------------------------------------------------
// Static helpers (module-private)
// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// ARIA attribute eligibility — role sets
//
// Each set lists the Rive roles for which a given ARIA attribute is valid per
// WAI-ARIA 1.2 ("Used in roles" + "Inherits into roles"). The trait-gated
// blocks in applyAttributes check these before setting an attribute so that
// C++ nodes with unexpected trait combinations never produce invalid markup.
//
// To add a new role: append it to the relevant set(s) here — no other changes
// needed in applyAttributes.
// ---------------------------------------------------------------------------
/** aria-expanded: button, link, checkbox, switch (inherits button), tab. */
var ARIA_EXPANDED_ROLES = new Set([
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.button,
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.link,
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.checkbox,
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.switchControl,
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.tab,
]);
/**
 * aria-selected: tab is the only role in our current set that natively
 * supports it. Tab is also handled by an explicit unconditional branch in
 * applyAttributes (ARIA requires it there regardless of Selectable trait), but
 * the set still lists it so the constraint is visible in one place.
 */
var ARIA_SELECTED_ROLES = new Set([
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.tab,
]);
/** aria-checked: checkbox, radio, switch. */
var ARIA_CHECKED_ROLES = new Set([
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.checkbox,
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.radioButton,
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.switchControl,
]);
/**
 * aria-checked="mixed": checkbox only. ARIA defines checkbox as three-valued
 * (true/false/mixed) but radio and switch as two-valued (true/false), so an
 * indeterminate check state degrades to false for those roles rather than
 * emitting a value they do not support.
 */
var ARIA_MIXED_ROLES = new Set([
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.checkbox,
]);
/**
 * aria-pressed: button only. switch is a button subclass in ARIA but uses
 * aria-checked (not aria-pressed) for its on/off state — it is intentionally
 * excluded here and handled separately in the Toggleable block.
 */
var ARIA_PRESSED_ROLES = new Set([
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.button,
]);
/** aria-required: checkbox, textbox, radiogroup. */
var ARIA_REQUIRED_ROLES = new Set([
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.checkbox,
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.textField,
    _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.radioGroup,
]);
// ---------------------------------------------------------------------------
/** Style for visually-hidden description spans used with aria-describedby. */
var DESC_SPAN_STYLE = [
    "position:absolute",
    "width:1px",
    "height:1px",
    "overflow:hidden",
    "pointer-events:none",
    "left:-9999px",
].join(";");
var BASE_NODE_STYLE = [
    "position:absolute",
    "pointer-events:none",
    "box-sizing:border-box",
    "overflow:visible",
    "margin:0",
    "padding:0",
    "transform-origin: 0px 0px 0px",
    "border:none",
    "background:transparent",
    "color:transparent",
    // "list-style:none",
].join(";");
var SPAN_EXP = [
    "display:inline-block",
    "white-space:nowrap",
    "pointer-events:none",
].join(";");
/**
 * Attribute writers that skip same-value mutations. Even a no-op setAttribute
 * fires a mutation record, and whether AX layers dedupe those is
 * browser-specific — skipping the write is the only browser-proof guard.
 */
function setAttr(el, attr, value) {
    if (el.getAttribute(attr) !== value)
        el.setAttribute(attr, value);
}
function removeAttr(el, attr) {
    if (el.hasAttribute(attr))
        el.removeAttribute(attr);
}
function setBoolAttr(el, attr, value) {
    setAttr(el, attr, value ? "true" : "false");
}
/** Roles that receive click/Enter/Space action handlers. */
function isClickableRole(role) {
    switch (role) {
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.button:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.link:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.checkbox:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.switchControl:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.tab:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.radioButton:
            return true;
        default:
            return false;
    }
}
/** Roles that receive tabindex="-1" for programmatic/AT focus (not Tab order). */
function isInteractiveRole(role) {
    switch (role) {
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.button:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.link:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.checkbox:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.switchControl:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.slider:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.tab:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.radioButton:
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.textField:
            return true;
        default:
            return false;
    }
}
/**
 * A modal/alert dialog: alertDialog is always modal per WAI-ARIA, a plain
 * dialog only when the Modal state flag is set.
 */
function isModalDialogRole(role, flags) {
    return (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.alertDialog ||
        (role === _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.dialog && (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasState)(flags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticState.Modal)));
}
/** Whether a node can receive focus: an interactive role or the Focusable trait. */
function isFocusableNode(node) {
    return (isInteractiveRole(node.role) ||
        (0,_types__WEBPACK_IMPORTED_MODULE_2__.hasTrait)(node.traitFlags, _types__WEBPACK_IMPORTED_MODULE_2__.SemanticTrait.Focusable));
}
/**
 * Choose an HTML tag for a given role. Prefer native semantic elements
 * where they exist — screen readers treat them more reliably than
 * generic elements with ARIA role overrides.
 */
function tagForRole(role) {
    switch (role) {
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.link:
            return "a";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.text:
            // The outer div is the positioned AX element VoiceOver measures for its
            // highlight box. The text itself lives in a child <span> (see createElement).
            return "div";
        default:
            return "div";
    }
}
/**
 * Maps a Rive SemanticRole to an ARIA `role` attribute value.
 * Returns null for roles that don't need an explicit role attribute here
 * (e.g. text nodes use an outer div + inner span with textContent; heading
 * role is applied separately when headingLevel > 0).
 */
function ariaRoleForSemantic(role) {
    switch (role) {
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.none:
            // TODO: Role "none" removes the node from the accessibility tree. For now, setting to Group, but maybe we want to switch to "none"
            // or "presentation".
            return "group";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.button:
            return "button";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.link:
            return null; // native <a>
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.checkbox:
            return "checkbox";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.switchControl:
            return "switch";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.slider:
            return "slider";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.textField:
            return "textbox";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.image:
            return "img";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.group:
            return "group";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.list:
            return "list";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.listItem:
            return "listitem";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.tab:
            return "tab";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.tabList:
            return "tablist";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.dialog:
            return "dialog";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.alertDialog:
            return "alertdialog";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.radioGroup:
            return "radiogroup";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.radioButton:
            return "radio";
        case _types__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.text:
            // Text nodes use <span>; heading role is applied separately
            // when headingLevel > 0.
            return null;
        default:
            return null;
    }
}


/***/ }),
/* 10 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   claimKeyEvent: () => (/* binding */ claimKeyEvent),
/* harmony export */   isKeyEventClaimed: () => (/* binding */ isKeyEventClaimed)
/* harmony export */ });
var claimed = new WeakSet();
/**
 * Marks a key event as handled by a semantic overlay widget (and prevents its
 * default), so the Rive keyboard handler leaves it alone. A plain
 * `defaultPrevented` check can't tell these apart from host-page handlers.
 */
function claimKeyEvent(event) {
    event.preventDefault();
    claimed.add(event);
}
function isKeyEventClaimed(event) {
    return claimed.has(event);
}


/***/ }),
/* 11 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   canvasOffset: () => (/* binding */ canvasOffset)
/* harmony export */ });
/**
 * Canvas offset within its offsetParent, minus the scroll of ancestors in between (they
 * move the canvas, not absolutely positioned siblings).
 */
function canvasOffset(canvas) {
    var top = canvas.offsetTop;
    var left = canvas.offsetLeft;
    var offsetParent = canvas.offsetParent;
    if (!offsetParent)
        return { top: top, left: left };
    for (var el = canvas.parentElement; el && el !== offsetParent; el = el.parentElement) {
        top -= el.scrollTop;
        left -= el.scrollLeft;
    }
    return { top: top, left: left };
}


/***/ }),
/* 12 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   AudioAssetWrapper: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.AudioAssetWrapper),
/* harmony export */   AudioWrapper: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.AudioWrapper),
/* harmony export */   BLANK_URL: () => (/* reexport safe */ _sanitizeUrl__WEBPACK_IMPORTED_MODULE_2__.BLANK_URL),
/* harmony export */   CustomFileAssetLoaderWrapper: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.CustomFileAssetLoaderWrapper),
/* harmony export */   FileAssetWrapper: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.FileAssetWrapper),
/* harmony export */   FileFinalizer: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.FileFinalizer),
/* harmony export */   FocusSessionState: () => (/* reexport safe */ _registerKeyboardInteractions__WEBPACK_IMPORTED_MODULE_1__.FocusSessionState),
/* harmony export */   FontAssetWrapper: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.FontAssetWrapper),
/* harmony export */   FontWrapper: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.FontWrapper),
/* harmony export */   ImageAssetWrapper: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.ImageAssetWrapper),
/* harmony export */   ImageWrapper: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.ImageWrapper),
/* harmony export */   KeyboardInteractions: () => (/* reexport safe */ _registerKeyboardInteractions__WEBPACK_IMPORTED_MODULE_1__.KeyboardInteractions),
/* harmony export */   RiveFont: () => (/* reexport safe */ _riveFont__WEBPACK_IMPORTED_MODULE_4__.RiveFont),
/* harmony export */   createFinalization: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.createFinalization),
/* harmony export */   finalizationRegistry: () => (/* reexport safe */ _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__.finalizationRegistry),
/* harmony export */   registerTouchInteractions: () => (/* reexport safe */ _registerTouchInteractions__WEBPACK_IMPORTED_MODULE_0__.registerTouchInteractions),
/* harmony export */   sanitizeUrl: () => (/* reexport safe */ _sanitizeUrl__WEBPACK_IMPORTED_MODULE_2__.sanitizeUrl)
/* harmony export */ });
/* harmony import */ var _registerTouchInteractions__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(13);
/* harmony import */ var _registerKeyboardInteractions__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(14);
/* harmony import */ var _sanitizeUrl__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(17);
/* harmony import */ var _finalizationRegistry__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__(18);
/* harmony import */ var _riveFont__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__(19);







/***/ }),
/* 13 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   registerTouchInteractions: () => (/* binding */ registerTouchInteractions)
/* harmony export */ });
var _this = undefined;
// A detented wheel reports lines; the runtime wants pixels.
var PIXELS_PER_LINE = 16;
// ScrollPhase.update. The DOM reports no gesture phase, so every wheel event
// is an update and the runtime closes the gesture once it goes quiet.
var SCROLL_PHASE_UPDATE = 1;
/**
 * Extracts ClientCoordinates from a TouchList, respecting multi-touch vs.
 * single-touch mode. In single-touch mode, only the touch matching
 * primaryTouchId is returned (or the first touch when primaryTouchId is null).
 */
var getTouchCoordinates = function (changedTouches, enableMultiTouch, primaryTouchId) {
    var _a;
    var coordinates = [];
    if (enableMultiTouch) {
        for (var i = 0; i < changedTouches.length; i++) {
            var touch = changedTouches[i];
            coordinates.push({
                clientX: touch.clientX,
                clientY: touch.clientY,
                identifier: touch.identifier,
            });
        }
    }
    else {
        // In "single-touch mode", only track the primary finger identified at touchstart.
        // Search changedTouches for the touch matching the recorded primary touch identifier, or (on initial touchstart)
        // take the first available touch identifier.
        var primaryTouch = primaryTouchId !== null
            ? (_a = Array.from(changedTouches).find(function (t) { return t.identifier === primaryTouchId; })) !== null && _a !== void 0 ? _a : null
            : changedTouches[0];
        if (primaryTouch) {
            coordinates.push({
                clientX: primaryTouch.clientX,
                clientY: primaryTouch.clientY,
                identifier: primaryTouch.identifier,
            });
        }
    }
    return coordinates;
};
/**
 * Returns the clientX and clientY properties from touch or mouse events. Also
 * calls preventDefault() on the event if it is a touchstart or touchmove to prevent
 * scrolling the page on mobile devices
 * @param event - Either a TouchEvent or a MouseEvent
 * @param isTouchScrollEnabled - Whether touch scrolling is enabled
 * @param enableMultiTouch - Whether to process multiple simultaneous touches
 * @param primaryTouchId - When working with single touches, only process the touch
 *   with this identifier. Pass null to accept any touch (used during touchstart to
 *   capture the first finger down).
 * @returns - Coordinates of the clientX and clientY properties from the touch/mouse event
 */
var getClientCoordinates = function (event, isTouchScrollEnabled, enableMultiTouch, primaryTouchId) {
    var _a;
    var touchEvent = event;
    if ((_a = touchEvent.changedTouches) === null || _a === void 0 ? void 0 : _a.length) {
        // This flag, if false, prevents touch events on the canvas default behavior
        // which may prevent scrolling if a drag motion on the canvas is performed
        if (!isTouchScrollEnabled && ["touchstart", "touchmove"].includes(event.type)) {
            event.preventDefault();
        }
        return getTouchCoordinates(touchEvent.changedTouches, enableMultiTouch, primaryTouchId);
    }
    return [
        {
            clientX: event.clientX,
            clientY: event.clientY,
            identifier: 0,
        },
    ];
};
/**
 * Registers mouse move/up/down and wheel callback handlers on the canvas to send meaningful
 * coordinates to the state machine pointer move/up/down/scroll functions based on cursor
 * interaction
 */
var registerTouchInteractions = function (_a) {
    var canvas = _a.canvas, artboard = _a.artboard, _b = _a.stateMachines, stateMachines = _b === void 0 ? [] : _b, renderer = _a.renderer, rive = _a.rive, fit = _a.fit, alignment = _a.alignment, _c = _a.isTouchScrollEnabled, isTouchScrollEnabled = _c === void 0 ? false : _c, _d = _a.dispatchPointerExit, dispatchPointerExit = _d === void 0 ? true : _d, _e = _a.enableMultiTouch, enableMultiTouch = _e === void 0 ? false : _e, _f = _a.layoutScaleFactor, layoutScaleFactor = _f === void 0 ? 1.0 : _f, advanceAndDrain = _a.advanceAndDrain, summonKeyboard = _a.summonKeyboard, isTextProxyFocused = _a.isTextProxyFocused;
    if (!canvas ||
        !stateMachines.length ||
        !renderer ||
        !rive ||
        !artboard ||
        typeof window === "undefined") {
        return null;
    }
    // A gesture latched before the previous listeners came down (a pause, say)
    // never reached its idle timeout, and would take the next wheel wherever
    // it lands. These instances are live here; at teardown they may not be.
    for (var _i = 0, stateMachines_1 = stateMachines; _i < stateMachines_1.length; _i++) {
        var stateMachine = stateMachines_1[_i];
        stateMachine.cancelScroll();
    }
    /**
     * After a touchend event, some browsers may fire synthetic mouse events
     * (mouseover, mousedown, mousemove, mouseup) if the touch interaction did not cause
     * any default action (such as scrolling).
     *
     * This is done to simulate the behavior of a mouse for applications that do not support
     * touch events.
     *
     * We're keeping track of the previous event to not send the synthetic mouse events if the
     * touch event was a click (touchstart -> touchend).
     *
     * Emulated events only occur when `isTouchScrollEnabled` is true; when false,
     * touchstart is default-prevented, which suppresses them.
     * Emulated mousedown is prevented while the proxy is focused, or it would blur it and
     * drop the keyboard.
     **/
    var _prevEventType = null;
    var _syntheticEventsActive = false;
    /**
     * When enableMultiTouch is false ("single-touch mode"), we track the identifier of the first finger that touched down.
     * All subsequent touch events are filtered to this identifier so that a second finger
     * moving cannot displace the tracked pointer position.
     * Reset to null when the primary finger lifts (or touchcancel is called)
     */
    var _primaryTouchId = null;
    /**
     * Maps client-space points into Artboard space through the fit and alignment
     * of the canvas.
     */
    var mapToArtboard = function (boundingRect, points) {
        var forwardMatrix = rive.computeAlignment(fit, alignment, {
            minX: 0,
            minY: 0,
            maxX: boundingRect.width,
            maxY: boundingRect.height,
        }, artboard.bounds, layoutScaleFactor);
        var invertedMatrix = new rive.Mat2D();
        forwardMatrix.invert(invertedMatrix);
        var mapped = points.map(function (_a) {
            var clientX = _a.clientX, clientY = _a.clientY;
            var canvasX = clientX - boundingRect.left;
            var canvasY = clientY - boundingRect.top;
            var canvasCoordinatesVector = new rive.Vec2D(canvasX, canvasY);
            var transformedVector = rive.mapXY(invertedMatrix, canvasCoordinatesVector);
            var point = { x: transformedVector.x(), y: transformedVector.y() };
            transformedVector.delete();
            canvasCoordinatesVector.delete();
            return point;
        });
        invertedMatrix.delete();
        forwardMatrix.delete();
        return mapped;
    };
    var processEventCallback = function (event) {
        var _a;
        // Exit early out of all synthetic mouse events
        // https://stackoverflow.com/questions/9656990/how-to-prevent-simulated-mouse-events-in-mobile-browsers
        // https://stackoverflow.com/questions/25572070/javascript-touchend-versus-click-dilemma
        if (_syntheticEventsActive && event instanceof MouseEvent) {
            if (event.type === "mousedown" && (isTextProxyFocused === null || isTextProxyFocused === void 0 ? void 0 : isTextProxyFocused())) {
                event.preventDefault();
            }
            // Synthetic event finished
            if (event.type == "mouseup") {
                _syntheticEventsActive = false;
            }
            return;
        }
        // Test if it's a "touch click". This could cause the browser to send
        // synthetic mouse events.
        _syntheticEventsActive =
            isTouchScrollEnabled &&
                event.type === "touchend" &&
                _prevEventType === "touchstart";
        _prevEventType = event.type;
        var boundingRect = event.currentTarget.getBoundingClientRect();
        // On touchstart in single-touch mode, record the first new finger as the primary
        // touch if we aren't already tracking one.
        if (!enableMultiTouch && event.type === "touchstart" && _primaryTouchId === null) {
            var firstTouch = (_a = event.changedTouches) === null || _a === void 0 ? void 0 : _a[0];
            if (firstTouch) {
                _primaryTouchId = firstTouch.identifier;
            }
        }
        var coordinateSets = getClientCoordinates(event, isTouchScrollEnabled, enableMultiTouch, enableMultiTouch ? null : _primaryTouchId);
        var positionedSets = coordinateSets.filter(function (coordinateSet) { return coordinateSet.clientX || coordinateSet.clientY; });
        mapToArtboard(boundingRect, positionedSets).forEach(function (point, i) {
            positionedSets[i].transformedX = point.x;
            positionedSets[i].transformedY = point.y;
        });
        switch (event.type) {
            /**
             * There's a 2px buffer for a hitRadius when translating the pointer coordinates
             * down to the state machine. In cases where the hitbox is about that much away
             * from the Artboard border, we don't have exact precision on determining pointer
             * exit. We're therefore adding to the translated coordinates on mouseout of a canvas
             * to ensure that we report the mouse has truly exited the hitarea.
             * https://github.com/rive-app/rive-cpp/blob/master/src/animation/state_machine_instance.cpp#L336
             *
             */
            case "mouseout":
                var _loop_1 = function (stateMachine) {
                    if (dispatchPointerExit) {
                        coordinateSets.forEach(function (coordinateSet) {
                            stateMachine.pointerExit(coordinateSet.transformedX, coordinateSet.transformedY, coordinateSet.identifier);
                        });
                    }
                    else {
                        coordinateSets.forEach(function (coordinateSet) {
                            stateMachine.pointerMove(coordinateSet.transformedX, coordinateSet.transformedY, coordinateSet.identifier);
                        });
                    }
                };
                for (var _i = 0, stateMachines_2 = stateMachines; _i < stateMachines_2.length; _i++) {
                    var stateMachine = stateMachines_2[_i];
                    _loop_1(stateMachine);
                }
                break;
            // Pointer moving/hovering on the canvas
            case "touchmove":
            case "mouseover":
            case "mousemove": {
                var _loop_2 = function (stateMachine) {
                    coordinateSets.forEach(function (coordinateSet) {
                        stateMachine.pointerMove(coordinateSet.transformedX, coordinateSet.transformedY, coordinateSet.identifier);
                    });
                };
                for (var _b = 0, stateMachines_3 = stateMachines; _b < stateMachines_3.length; _b++) {
                    var stateMachine = stateMachines_3[_b];
                    _loop_2(stateMachine);
                }
                break;
            }
            // Pointer click initiated but not released yet on the canvas
            case "touchstart":
            case "mousedown": {
                var _loop_3 = function (stateMachine) {
                    coordinateSets.forEach(function (coordinateSet) {
                        stateMachine.pointerDown(coordinateSet.transformedX, coordinateSet.transformedY, coordinateSet.identifier);
                    });
                };
                for (var _c = 0, stateMachines_4 = stateMachines; _c < stateMachines_4.length; _c++) {
                    var stateMachine = stateMachines_4[_c];
                    _loop_3(stateMachine);
                }
                // Advance the state machine immediately so pointer down(s) takes effect synchronously
                advanceAndDrain(0, { pointerDown: true });
                break;
            }
            // Pointer click released on the canvas
            case "touchend": {
                var _loop_4 = function (stateMachine) {
                    coordinateSets.forEach(function (coordinateSet) {
                        stateMachine.pointerUp(coordinateSet.transformedX, coordinateSet.transformedY, coordinateSet.identifier);
                        stateMachine.pointerExit(coordinateSet.transformedX, coordinateSet.transformedY, coordinateSet.identifier);
                    });
                };
                for (var _d = 0, stateMachines_5 = stateMachines; _d < stateMachines_5.length; _d++) {
                    var stateMachine = stateMachines_5[_d];
                    _loop_4(stateMachine);
                }
                // Advance the state machine immediately so pointer up(s) takes effect synchronously
                advanceAndDrain(0);
                // Still inside the touch gesture — summon the keyboard if the tap focused a
                // text input
                summonKeyboard === null || summonKeyboard === void 0 ? void 0 : summonKeyboard();
                // Release the primary touch lock once that finger lifts so the next
                // touchstart can claim a new primary finger.
                if (!enableMultiTouch &&
                    coordinateSets.some(function (c) { return c.identifier === _primaryTouchId; })) {
                    _primaryTouchId = null;
                }
                break;
            }
            case "mouseup": {
                var _loop_5 = function (stateMachine) {
                    coordinateSets.forEach(function (coordinateSet) {
                        stateMachine.pointerUp(coordinateSet.transformedX, coordinateSet.transformedY, coordinateSet.identifier);
                    });
                };
                for (var _e = 0, stateMachines_6 = stateMachines; _e < stateMachines_6.length; _e++) {
                    var stateMachine = stateMachines_6[_e];
                    _loop_5(stateMachine);
                }
                // Advance the state machine immediately so pointer up(s) takes effect synchronously
                advanceAndDrain(0);
                summonKeyboard === null || summonKeyboard === void 0 ? void 0 : summonKeyboard();
                break;
            }
            default:
        }
    };
    var touchCancelCallback = function () {
        _primaryTouchId = null;
    };
    /**
     * Forwards wheel and trackpad scrolling to the state machines. The page keeps
     * the wheel unless a scroll view actually moves, so it still scrolls when
     * Rive has nothing to scroll or a view sits at its edge.
     */
    var wheelCallback = function (event) {
        // Pinch zoom arrives as a ctrl+wheel and belongs to the page. A wheel that
        // can't be cancelled is part of a sequence the page is already scrolling;
        // taking it too would move both.
        if (event.ctrlKey || !event.cancelable) {
            return;
        }
        // Read before the deltas: Firefox reports pixels once a delta has been
        // read first.
        var deltaMode = event.deltaMode;
        var deltaX = event.deltaX;
        var deltaY = event.deltaY;
        // Shift+wheel scrolls sideways. Most browsers already report it on the x
        // axis; a vertical-only view would decline it on y.
        if (event.shiftKey && !deltaX) {
            deltaX = deltaY;
            deltaY = 0;
        }
        var boundingRect = event.currentTarget.getBoundingClientRect();
        // A page is the canvas's visible size, the nearest the host can get to the
        // scroll view's own.
        var toPixelsX = deltaMode === WheelEvent.DOM_DELTA_LINE
            ? PIXELS_PER_LINE
            : deltaMode === WheelEvent.DOM_DELTA_PAGE
                ? boundingRect.width
                : 1;
        var toPixelsY = deltaMode === WheelEvent.DOM_DELTA_LINE
            ? PIXELS_PER_LINE
            : deltaMode === WheelEvent.DOM_DELTA_PAGE
                ? boundingRect.height
                : 1;
        // The DOM reports how far to scroll; the runtime wants how far content
        // travels.
        var scrollX = -deltaX * toPixelsX;
        var scrollY = -deltaY * toPixelsY;
        if (!scrollX && !scrollY) {
            return;
        }
        // Mapping both ends of the delta keeps only the scale of the fit.
        var _a = mapToArtboard(boundingRect, [
            { clientX: event.clientX, clientY: event.clientY },
            { clientX: event.clientX + scrollX, clientY: event.clientY + scrollY },
        ]), position = _a[0], moved = _a[1];
        // The DOM never says whether a pixel delta came from a trackpad, so it
        // counts as precise either way, as it does in the browser's own scrolling.
        var precise = deltaMode === WheelEvent.DOM_DELTA_PIXEL;
        var timeStamp = event.timeStamp / 1000;
        // A latched state machine owns the rest of its gesture, even past its edge.
        var latched = stateMachines.filter(function (sm) { return sm.hasScrollLatch(); });
        var ordered = latched.concat(stateMachines.filter(function (sm) { return latched.indexOf(sm) === -1; }));
        var consumed = ordered.some(function (stateMachine) {
            return stateMachine.pointerScroll(position.x, position.y, moved.x - position.x, moved.y - position.y, SCROLL_PHASE_UPDATE, precise, timeStamp, 0) !== 0;
        });
        if (!consumed) {
            return;
        }
        event.preventDefault();
        // Advance now so the scroll lands on the next frame drawn.
        advanceAndDrain(0);
    };
    var callback = processEventCallback.bind(_this);
    canvas.addEventListener("mouseover", callback);
    canvas.addEventListener("mouseout", callback);
    canvas.addEventListener("mousemove", callback);
    canvas.addEventListener("mousedown", callback);
    canvas.addEventListener("mouseup", callback);
    canvas.addEventListener("touchmove", callback, {
        passive: isTouchScrollEnabled,
    });
    canvas.addEventListener("touchstart", callback, {
        passive: isTouchScrollEnabled,
    });
    canvas.addEventListener("touchend", callback);
    canvas.addEventListener("touchcancel", touchCancelCallback);
    // Not passive, or preventDefault could not keep the page from scrolling.
    canvas.addEventListener("wheel", wheelCallback, { passive: false });
    return function () {
        canvas.removeEventListener("mouseover", callback);
        canvas.removeEventListener("mouseout", callback);
        canvas.removeEventListener("mousemove", callback);
        canvas.removeEventListener("mousedown", callback);
        canvas.removeEventListener("mouseup", callback);
        canvas.removeEventListener("touchmove", callback);
        canvas.removeEventListener("touchstart", callback);
        canvas.removeEventListener("touchend", callback);
        canvas.removeEventListener("touchcancel", touchCancelCallback);
        canvas.removeEventListener("wheel", wheelCallback);
    };
};


/***/ }),
/* 14 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   FocusSessionState: () => (/* binding */ FocusSessionState),
/* harmony export */   KeyboardInteractions: () => (/* binding */ KeyboardInteractions)
/* harmony export */ });
/* harmony import */ var _keyMap__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(15);
/* harmony import */ var _semantics_claimedKeyEvents__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(10);
/* harmony import */ var _textInputProxy__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(16);



/**
 * Defaults suppressed while typing (scroll/navigate/submit). Tab is traversal's; proxy
 * Space returns earlier as a printable.
 */
var EDITING_NAV_CODES = new Set([
    "Space",
    "ArrowLeft",
    "ArrowRight",
    "ArrowUp",
    "ArrowDown",
    "Backspace",
    "Delete",
    "Home",
    "End",
    "Enter",
    "NumpadEnter",
]);
/** A keystroke that types a character (not a Ctrl/Cmd shortcut). */
function isPrintable(event) {
    return event.key.length === 1 && !event.ctrlKey && !event.metaKey;
}
function isArrowKey(key) {
    return (key === _keyMap__WEBPACK_IMPORTED_MODULE_0__.Key.left || key === _keyMap__WEBPACK_IMPORTED_MODULE_0__.Key.right || key === _keyMap__WEBPACK_IMPORTED_MODULE_0__.Key.up || key === _keyMap__WEBPACK_IMPORTED_MODULE_0__.Key.down);
}
/**
 * Alt/Ctrl/Meta arrows are browser/OS shortcuts (Alt+Left = back, Cmd+Arrow, etc.).
 * Shift is allowed: it is the spatial-navigation chord in Vivaldi/Opera.
 */
function hasShortcutModifier(event) {
    return event.altKey || event.ctrlKey || event.metaKey;
}
/**
 * Tracks the relationship between DOM focus inside this Rive focus domain (canvas or semantic overlay)
 * and Rive's internal focus for the current focus session.
 *
 * NotFocused   — DOM focus left the domain, Rive released focus internally, or Tab walked
 *                off the end of the tree. Keyboard input isn't ours, so Tab is ignored and
 *                reaches the next page element.
 * EntryPending — DOM focus is inside the domain but Rive holds no node yet, so the next Tab
 *                enters the focus tree. Set by pointer focus on the canvas, by assistive technology (AT) focus landing
 *                in the overlay, and by keyboard focus whose entry attempt found no eligible node.
 * RiveFocused  — a Rive node holds focus. Tab/Shift+Tab route to the Rive focus manager and stay
 *                inside the domain until either Rive reports focus ended (pollFocusState) or
 *                Tab walks off the edge of the tree (focusNext()/focusPrevious() returns false and
 *                Rive no longer holds focus).
 *
 * Keyboard focus on the canvas enters the tree immediately: onCanvasFocus infers direction from
 * where focus came from and goes straight to RiveFocused when a node accepts.
 */
var FocusSessionState;
(function (FocusSessionState) {
    FocusSessionState["NotFocused"] = "notFocused";
    FocusSessionState["EntryPending"] = "entryPending";
    FocusSessionState["RiveFocused"] = "riveFocused";
})(FocusSessionState || (FocusSessionState = {}));
/**
 * Manages keyboard and DOM focus interactions for Rive's focus domain (<canvas>, semantic
 * overlay, text input proxy). Owns the proxy and the text input session state.
 * Because keyboard events can apply on either part of the domain, we need to track what events we should
 * handle/intercept, and when to release focus back to the page outside of the domain.
 *
 * Tracks the canvas focus session state (focusSessionState) and routes
 * Tab/Shift+Tab to the Rive state machine's focus manager. Exposes shared
 * state as properties so the Rive render loop can read them directly.
 */
var KeyboardInteractions = /** @class */ (function () {
    function KeyboardInteractions(_a) {
        var canvas = _a.canvas, stateMachine = _a.stateMachine, hasFocusNodes = _a.hasFocusNodes, getOverlayElement = _a.getOverlayElement;
        var _this = this;
        var _b, _c;
        this.focusSessionState = FocusSessionState.NotFocused;
        /** Whether the canvas currently has browser DOM focus. */
        this.canvasHasFocus = false;
        /** After Tab exits the last Rive node, ignore keydowns until focus re-enters the focus domain. */
        this.focusDomainReleased = false;
        /** Overlay element currently wired with focusin/keydown listeners, if any. */
        this.currentOverlayElement = null;
        /** Codes whose keydown an overlay widget took, so their keyup is skipped too. */
        this.claimedKeyCodes = new Set();
        this.textInputSessionActive = false;
        /**
         * The proxy is always empty: copy Rive's selection; always preventDefault.
         */
        this.onCopy = function (event) {
            var _a;
            event.preventDefault();
            var selected = _this.mainSm.selectedText();
            if (selected)
                (_a = event.clipboardData) === null || _a === void 0 ? void 0 : _a.setData("text/plain", selected);
        };
        /**
         * Handles the canvas gaining browser focus. The behavior differs based on how focus was gained -
         *
         * Pointer-driven focus: the canvas now has focus but Rive holds nothing yet, so we move to EntryPending — this lets the
         * next Tab enter the focus tree even when the focus is pointer-driven
         *
         * Keyboard-driven focus: we enter the Rive focus tree immediately once canvas gains focus.
         * The direction is inferred from where focus came from: an element before the canvas in DOM order
         * means a forward Tab (focusNext), one after means a Shift+Tab (focusPrevious). :focus-visible
         * gates this so a click doesn't yank Rive focus to the first node on the focus event itself.
         */
        this.onCanvasFocus = function (event) {
            _this.syncOverlayListener();
            _this.canvasHasFocus = true;
            _this.focusDomainReleased = false;
            if (_this.isInFocusDomain(event.relatedTarget) ||
                !_this.hasFocusNodes ||
                _this.mainSm.focusState().hasFocus) {
                return;
            }
            _this.focusSessionState = FocusSessionState.EntryPending;
            // Pointer focus waits for the user's next Tab (handled in onKeyDown). Keyboard focus enters now.
            if (!_this.isKeyboardDrivenFocus())
                return;
            var forward = _this.cameFromBeforeCanvas(event.relatedTarget);
            if (forward ? _this.mainSm.focusNext() : _this.mainSm.focusPrevious()) {
                _this.focusSessionState = FocusSessionState.RiveFocused;
            }
        };
        /**
         * Marks internal state that the canvas has lost DOM focus. Do not actually clear
         * Rive focus though if:
         * 1. DOM focus is still within Rive domain (i.e., semantic overlay)
         * 2. Document just lost focus (i.e. tab switching)
         *
         * When we're not in either of those buckets, it's safe to call `clearFocus()` on the SMI.
         */
        this.onCanvasBlur = function (event) {
            // Must stay before the state reset because beginTextInputSession sets the session state and
            // then focuses the proxy, which synchronously blurs the canvas.
            if (_this.inputProxy.owns(event.relatedTarget))
                return;
            _this.focusSessionState = FocusSessionState.NotFocused;
            _this.canvasHasFocus = false;
            if (_this.blurStaysWithRive(event.relatedTarget))
                return;
            _this.mainSm.clearFocus();
        };
        /**
         * Assistive technology (AT) focus landing inside the overlay is DOM focus inside the Rive focus domain, so open a
         * session even when no Rive node holds focus yet. shouldRiveHandleKeyEvent treats NotFocused
         * as authoritative, so without this the overlay's Tab keydowns reach onKeyDown and get dropped
         * at that gate — the browser would move focus out of Rive instead of to the next focus node.
         */
        this.onOverlayFocusIn = function (event) {
            if (!_this.isInOverlay(event.target))
                return;
            _this.focusDomainReleased = false;
            if (!_this.hasFocusNodes)
                return;
            if (_this.focusSessionState !== FocusSessionState.NotFocused)
                return;
            _this.focusSessionState = _this.mainSm.focusState().hasFocus
                ? FocusSessionState.RiveFocused
                : FocusSessionState.EntryPending;
        };
        /** Overlay listeners attach lazily, so the first focusin only ever lands here. */
        this.onFocusDomainHostFocusIn = function (event) {
            _this.syncOverlayListener();
            _this.onOverlayFocusIn(event);
            // Focus entering the proxy re-enters the focus domain, so clear any earlier Tab-out
            // release, or onKeyDown would drop the session's keystrokes.
            if (_this.inputProxy.owns(event.target)) {
                _this.focusDomainReleased = false;
            }
        };
        /**
         * Handles keydown events in the Rive focus domain. We route certain keys to different functions, with the following
         * fallback behavior:
         * 1. Tab/Shift+Tab - check if keyInput() takes this key, if not, call focusNext/Previous traversal if there are focus nodes
         * 2. Check - if a semantic overlay element's key handler claimed the event, don't intercept the key event further
         * 3. Send key to `keyInput()` on Rive state machine if applicable.
         * 4. If the keyInput returned false, and the key is directional arrow keys, call directional focus methods on state machine
         * 5. Printables: in a session the proxy's input event supplies text; otherwise textInput().
         *    A printable fires both keyInput and textInput even if a listener claims it.
         * @param event KeyboardEvent
         * @returns void
         */
        this.onKeyDown = function (event) {
            _this.syncOverlayListener();
            // Keys that are part of an IME composition belong to the IME (candidate
            // navigation, commit). Safari reports the first one as keyCode 229 with
            // isComposing still false.
            if (event.isComposing || event.keyCode === 229)
                return;
            // After Tab exits the last Rive node, ignore keys until focus re-enters the focus domain.
            if (_this.focusDomainReleased)
                return;
            if (!_this.shouldRiveHandleKeyEvent(event))
                return;
            if (event.key === "Tab" && _this.hasFocusNodes) {
                // Tab listeners are matched per phase (down/repeat/up), as in the editor, so
                // each event is offered on its own; one that isn't claimed traverses.
                if (_this.mainSm.focusState().hasFocus &&
                    _this.mainSm.keyInput(_keyMap__WEBPACK_IMPORTED_MODULE_0__.Key.tab, (0,_keyMap__WEBPACK_IMPORTED_MODULE_0__.modifiersFromEvent)(event), true, event.repeat)) {
                    event.preventDefault();
                    return;
                }
                var forward = !event.shiftKey;
                var focusMoved = forward ? _this.mainSm.focusNext() : _this.mainSm.focusPrevious();
                var focusState = _this.mainSm.focusState();
                // focusMoved is false both at a Stop edge (focus stays put) and after walking off
                // the tree (focus cleared), so hasFocus discerns Stop edge from Rive releasing focus
                if (focusMoved || focusState.hasFocus) {
                    // A Rive node holds focus — keep trapping Tab inside Rive.
                    _this.focusSessionState = FocusSessionState.RiveFocused;
                    event.preventDefault();
                }
                else {
                    // No more traversable nodes — release Tab to the page.
                    if (document.activeElement !== _this.canvas &&
                        _this.isInFocusDomain(document.activeElement)) {
                        // The browser's default Tab starts from the focused element; from an overlay element,
                        // the nearest stop backward is the canvas itself. Start it from the
                        // canvas instead, so a Shift+Tab goes to the proper element before Rive.
                        _this.canvas.focus({ preventScroll: true });
                    }
                    _this.focusSessionState = FocusSessionState.NotFocused;
                    _this.focusDomainReleased = true;
                    _this.canvasHasFocus = false;
                }
                _this.syncOverlayListener();
                return;
            }
            // A semantic overlay widget (roving radio/tab group, slider, Enter/Space activation)
            // already acted on this key at its target, so don't act on it twice.
            if ((0,_semantics_claimedKeyEvents__WEBPACK_IMPORTED_MODULE_1__.isKeyEventClaimed)(event)) {
                _this.claimedKeyCodes.add(event.code);
                return;
            }
            // A new unclaimed press: drop any claim whose keyup never arrived.
            _this.claimedKeyCodes.delete(event.code);
            var fromProxy = _this.textInputSessionActive && _this.inputProxy.owns(event.target);
            var expectsKeyboardInput = _this.mainSm.focusState().expectsKeyboardInput;
            // The focused node gets first refusal (a TextInput's caret, or a keyboard listener
            // on that key). Keys with no Rive equivalent map to null and aren't dispatched.
            var key = (0,_keyMap__WEBPACK_IMPORTED_MODULE_0__.keyboardEventToRiveKey)(event);
            var consumed = key !== null &&
                _this.mainSm.keyInput(key, (0,_keyMap__WEBPACK_IMPORTED_MODULE_0__.modifiersFromEvent)(event), true, event.repeat);
            // An arrow key with no consumed key input moves focus spatially. While a Rive node holds focus,
            // arrows never scroll the page
            if (!consumed && key !== null && isArrowKey(key) && !hasShortcutModifier(event)) {
                consumed =
                    _this.focusInDirection(key) || _this.mainSm.focusState().hasFocus;
            }
            if (isPrintable(event)) {
                // During a session the printable must reach the proxy: its input event is the
                // (IME- and dead-key-correct) text source, and C++ inserts nothing from keyInput.
                if (fromProxy)
                    return;
                // Session live but focus elsewhere in Rive: refocus the proxy so the default inserts there.
                if (_this.textInputSessionActive && _this.isInFocusDomain(event.target)) {
                    _this.inputProxy.focus();
                    return;
                }
                if (!_this.textInputSessionActive && expectsKeyboardInput) {
                    _this.mainSm.textInput(event.key);
                }
            }
            // Rive consumed it, or an unmodified editing key while typing (would scroll/submit).
            var typing = fromProxy || expectsKeyboardInput;
            if (consumed ||
                (typing && !hasShortcutModifier(event) && EDITING_NAV_CODES.has(event.code))) {
                event.preventDefault();
            }
        };
        this.onKeyUp = function (event) {
            if (event.isComposing || event.keyCode === 229)
                return;
            if (_this.claimedKeyCodes.delete(event.code))
                return;
            if (_this.focusSessionState === FocusSessionState.NotFocused)
                return;
            var key = (0,_keyMap__WEBPACK_IMPORTED_MODULE_0__.keyboardEventToRiveKey)(event);
            if (key === null)
                return;
            _this.mainSm.keyInput(key, (0,_keyMap__WEBPACK_IMPORTED_MODULE_0__.modifiersFromEvent)(event), false, false);
        };
        this.canvas = canvas;
        this.mainSm = stateMachine;
        this.hasFocusNodes = hasFocusNodes;
        this.getOverlayElement = getOverlayElement;
        this.focusDomainHost = (_b = canvas.parentElement) !== null && _b !== void 0 ? _b : document;
        canvas.addEventListener("focus", this.onCanvasFocus);
        canvas.addEventListener("blur", this.onCanvasBlur);
        canvas.addEventListener("keydown", this.onKeyDown);
        canvas.addEventListener("keyup", this.onKeyUp);
        this.focusDomainHost.addEventListener("focusin", this.onFocusDomainHostFocusIn);
        this.syncOverlayListener();
        this.inputProxy = new _textInputProxy__WEBPACK_IMPORTED_MODULE_2__.TextInputProxy({
            container: (_c = canvas.parentElement) !== null && _c !== void 0 ? _c : document.body,
            canvas: canvas,
            textInput: function (text) { return _this.mainSm.textInput(text); },
            // Proxy keystrokes go through the same handlers as the canvas.
            onKeyDown: this.onKeyDown,
            onKeyUp: this.onKeyUp,
            onCopy: this.onCopy,
            onBlur: function (relatedTarget) { return _this.handleProxyBlur(relatedTarget); },
        });
    }
    KeyboardInteractions.prototype.isTextInputSessionActive = function () {
        return this.textInputSessionActive;
    };
    /** Session active and the proxy holds DOM focus. */
    KeyboardInteractions.prototype.isEditingHostFocused = function () {
        return this.textInputSessionActive && this.inputProxy.hasDomFocus();
    };
    /** @param focusProxy move DOM focus to the proxy; false leaves it where it is. */
    KeyboardInteractions.prototype.beginTextInputSession = function (focusProxy) {
        this.textInputSessionActive = true;
        this.focusSessionState = FocusSessionState.RiveFocused;
        if (focusProxy)
            this.inputProxy.focus();
    };
    /** @param fromBlur focus is already leaving; don't park it on the canvas. */
    KeyboardInteractions.prototype.endTextInputSession = function (fromBlur) {
        if (fromBlur === void 0) { fromBlur = false; }
        if (!this.textInputSessionActive)
            return;
        this.textInputSessionActive = false;
        this.inputProxy.endSession();
        if (fromBlur || !this.inputProxy.hasDomFocus())
            return;
        // Nothing took DOM focus from the proxy (e.g. semantics is off). Park it on the
        // canvas so keys keep reaching Rive; onCanvasFocus ignores focus from the input proxy.
        this.canvas.focus({ preventScroll: true });
    };
    /** Must run synchronously in the pointer gesture or mobile keyboards won't open. */
    KeyboardInteractions.prototype.summonForPointer = function () {
        if (this.textInputSessionActive)
            this.inputProxy.focus();
        else
            this.beginTextInputSession(true);
    };
    Object.defineProperty(KeyboardInteractions.prototype, "editingHost", {
        /** The text-input proxy, for the semantics layer to decorate while editing. */
        get: function () {
            return this.inputProxy;
        },
        enumerable: false,
        configurable: true
    });
    KeyboardInteractions.prototype.handleProxyBlur = function (relatedTarget) {
        // Teardown already in progress (endTextInputSession blurred the proxy).
        if (!this.textInputSessionActive)
            return;
        // Still in Rive or the document blurred: keep the session (pollFocusState tears down).
        if (this.blurStaysWithRive(relatedTarget))
            return;
        // Blurred out of Rive entirely: release C++ focus and end the session.
        this.mainSm.clearFocus();
        this.endTextInputSession(true);
        this.focusSessionState = FocusSessionState.NotFocused;
    };
    /**
     * Set the FocusSessionState. Useful for invoking a Rive "blur" without actually blurring from the <canvas>. This
     * helps put the DOM focus state on the canvas rather than the <body>, so the user doesn't lose the spot in page navigation
     *
     * @param state FocusSessionState enum
     */
    KeyboardInteractions.prototype.setFocusSessionState = function (state) {
        this.focusSessionState = state;
    };
    /**
     * Called by pollFocusState on the Rive instance when it observes hasFocus=true. Rive acquired
     * focus internally (e.g. via a listener action or state transition) without a DOM focus event,
     * so mark the session RiveFocused. This cannot resurrect a session that a DOM blur ended,
     * because onCanvasBlur clears Rive's focus alongside it.
     */
    KeyboardInteractions.prototype.notifyRiveFocused = function () {
        this.focusSessionState = FocusSessionState.RiveFocused;
    };
    /** Blur into the focus domain, or the whole document losing focus (tab/window switch). */
    KeyboardInteractions.prototype.blurStaysWithRive = function (relatedTarget) {
        return (this.isInFocusDomain(relatedTarget) ||
            (relatedTarget === null && !document.hasFocus()));
    };
    /** Arrow keys map to directional focus; anything else is not a focus move. */
    KeyboardInteractions.prototype.focusInDirection = function (key) {
        switch (key) {
            case _keyMap__WEBPACK_IMPORTED_MODULE_0__.Key.left:
                return this.mainSm.focusLeft();
            case _keyMap__WEBPACK_IMPORTED_MODULE_0__.Key.right:
                return this.mainSm.focusRight();
            case _keyMap__WEBPACK_IMPORTED_MODULE_0__.Key.up:
                return this.mainSm.focusUp();
            case _keyMap__WEBPACK_IMPORTED_MODULE_0__.Key.down:
                return this.mainSm.focusDown();
            default:
                return false;
        }
    };
    /**
     * Determine if Rive should handle keyboard input. If session state is `NotFocused` - no.
     * DOM focus stays parked on the canvas after Rive releases focus internally, and that Tab
     * has to reach the page rather than re-enter the tree.
     *
     * Otherwise, the event still has to belong to Rive:
     * 1. If the current DOM focus is in Rive domain (canvas or semantic overlay)
     * 2. If the target for the key input is for the semantic overlay, or the canvas
     */
    KeyboardInteractions.prototype.shouldRiveHandleKeyEvent = function (event) {
        if (this.focusSessionState === FocusSessionState.NotFocused)
            return false;
        var inFocusDomain = this.isInFocusDomain(document.activeElement) ||
            this.isInOverlay(event.target);
        var eventOnCanvas = event.target === this.canvas;
        return inFocusDomain || this.canvasHasFocus || eventOnCanvas;
    };
    /** DOM that counts as inside Rive for focus: canvas, overlay subtree, text-input proxy. */
    KeyboardInteractions.prototype.isInFocusDomain = function (target) {
        if (target === this.canvas)
            return true;
        if (this.inputProxy.owns(target))
            return true;
        return this.isInOverlay(target);
    };
    /** Overlay only (excludes the canvas) — the accessibility overlay subtree. */
    KeyboardInteractions.prototype.isInOverlay = function (target) {
        var _a, _b, _c;
        if (!(target instanceof Node))
            return false;
        return (_c = (_b = (_a = this.getOverlayElement) === null || _a === void 0 ? void 0 : _a.call(this)) === null || _b === void 0 ? void 0 : _b.contains(target)) !== null && _c !== void 0 ? _c : false;
    };
    KeyboardInteractions.prototype.syncOverlayListener = function () {
        var _a, _b, _c, _d, _e, _f, _g, _h;
        var nextOverlayElement = (_b = (_a = this.getOverlayElement) === null || _a === void 0 ? void 0 : _a.call(this)) !== null && _b !== void 0 ? _b : null;
        if (nextOverlayElement === this.currentOverlayElement)
            return;
        (_c = this.currentOverlayElement) === null || _c === void 0 ? void 0 : _c.removeEventListener("focusin", this.onOverlayFocusIn);
        (_d = this.currentOverlayElement) === null || _d === void 0 ? void 0 : _d.removeEventListener("keydown", this.onKeyDown);
        (_e = this.currentOverlayElement) === null || _e === void 0 ? void 0 : _e.removeEventListener("keyup", this.onKeyUp);
        this.currentOverlayElement = nextOverlayElement;
        (_f = this.currentOverlayElement) === null || _f === void 0 ? void 0 : _f.addEventListener("focusin", this.onOverlayFocusIn);
        // Bubble phase, so a semantic widget's own key handler runs first and can claim
        // the key via claimKeyEvent (see onKeyDown).
        (_g = this.currentOverlayElement) === null || _g === void 0 ? void 0 : _g.addEventListener("keydown", this.onKeyDown);
        (_h = this.currentOverlayElement) === null || _h === void 0 ? void 0 : _h.addEventListener("keyup", this.onKeyUp);
    };
    /**
     * Whether the canvas currently matches :focus-visible — the browser's heuristic for keyboard-
     * (vs pointer-) driven focus. For older browser versions that don't support this selector, return false
     * so that we don't incorrectly assume pointer vs keyboard focus. Next tab would enter the focus tree in those edge cases.
     */
    KeyboardInteractions.prototype.isKeyboardDrivenFocus = function () {
        try {
            return this.canvas.matches(":focus-visible");
        }
        catch (_a) {
            return false;
        }
    };
    KeyboardInteractions.prototype.cameFromBeforeCanvas = function (from) {
        if (!from)
            return true;
        var position = this.canvas.compareDocumentPosition(from);
        if (position & Node.DOCUMENT_POSITION_PRECEDING)
            return true;
        if (position & Node.DOCUMENT_POSITION_FOLLOWING)
            return false;
        return true;
    };
    KeyboardInteractions.prototype.cleanup = function () {
        var _a, _b, _c;
        this.canvas.removeEventListener("focus", this.onCanvasFocus);
        this.canvas.removeEventListener("blur", this.onCanvasBlur);
        this.canvas.removeEventListener("keydown", this.onKeyDown);
        this.canvas.removeEventListener("keyup", this.onKeyUp);
        this.claimedKeyCodes.clear();
        this.focusDomainHost.removeEventListener("focusin", this.onFocusDomainHostFocusIn);
        (_a = this.currentOverlayElement) === null || _a === void 0 ? void 0 : _a.removeEventListener("focusin", this.onOverlayFocusIn);
        (_b = this.currentOverlayElement) === null || _b === void 0 ? void 0 : _b.removeEventListener("keydown", this.onKeyDown);
        (_c = this.currentOverlayElement) === null || _c === void 0 ? void 0 : _c.removeEventListener("keyup", this.onKeyUp);
        // Removing a focused element drops focus to <body> with no blur; park on the canvas.
        if (this.inputProxy.hasDomFocus()) {
            this.canvas.focus({ preventScroll: true });
        }
        this.inputProxy.cleanup();
        this.textInputSessionActive = false;
    };
    return KeyboardInteractions;
}());



/***/ }),
/* 15 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   Key: () => (/* binding */ Key),
/* harmony export */   KeyModifiers: () => (/* binding */ KeyModifiers),
/* harmony export */   keyboardEventToRiveKey: () => (/* binding */ keyboardEventToRiveKey),
/* harmony export */   modifiersFromEvent: () => (/* binding */ modifiersFromEvent)
/* harmony export */ });
/*
 * Maps DOM keyboard events to the GLFW-style key codes and modifier bitmask
 * that StateMachineInstance.keyInput() expects. Values mirror `enum class Key`
 * and `enum class KeyModifiers` in rive-runtime focusable.hpp;
 * they are stable GLFW codes, so they live here as constants instead of an
 * embind enum.
 */
var __spreadArray = (undefined && undefined.__spreadArray) || function (to, from, pack) {
    if (pack || arguments.length === 2) for (var i = 0, l = from.length, ar; i < l; i++) {
        if (ar || !(i in from)) {
            if (!ar) ar = Array.prototype.slice.call(from, 0, i);
            ar[i] = from[i];
        }
    }
    return to.concat(ar || Array.prototype.slice.call(from));
};
var Key = {
    space: 32,
    apostrophe: 39, // '
    comma: 44, // ,
    minus: 45, // -
    period: 46, // .
    slash: 47, // /
    key0: 48,
    key1: 49,
    key2: 50,
    key3: 51,
    key4: 52,
    key5: 53,
    key6: 54,
    key7: 55,
    key8: 56,
    key9: 57,
    semicolon: 59, // ;
    equal: 61, // =
    a: 65,
    b: 66,
    c: 67,
    d: 68,
    e: 69,
    f: 70,
    g: 71,
    h: 72,
    i: 73,
    j: 74,
    k: 75,
    l: 76,
    m: 77,
    n: 78,
    o: 79,
    p: 80,
    q: 81,
    r: 82,
    s: 83,
    t: 84,
    u: 85,
    v: 86,
    w: 87,
    x: 88,
    y: 89,
    z: 90,
    leftBracket: 91, // [
    backslash: 92, // \
    rightBracket: 93, // ]
    graveAccent: 96, // `
    world1: 161, // non-US #1
    world2: 162, // non-US #2
    escape: 256,
    enter: 257,
    tab: 258,
    backspace: 259,
    insert: 260,
    deleteKey: 261,
    right: 262,
    left: 263,
    down: 264,
    up: 265,
    pageUp: 266,
    pageDown: 267,
    home: 268,
    end: 269,
    capsLock: 280,
    scrollLock: 281,
    numLock: 282,
    printScreen: 283,
    pause: 284,
    f1: 290,
    f2: 291,
    f3: 292,
    f4: 293,
    f5: 294,
    f6: 295,
    f7: 296,
    f8: 297,
    f9: 298,
    f10: 299,
    f11: 300,
    f12: 301,
    f13: 302,
    f14: 303,
    f15: 304,
    f16: 305,
    f17: 306,
    f18: 307,
    f19: 308,
    f20: 309,
    f21: 310,
    f22: 311,
    f23: 312,
    f24: 313,
    f25: 314,
    kp0: 320,
    kp1: 321,
    kp2: 322,
    kp3: 323,
    kp4: 324,
    kp5: 325,
    kp6: 326,
    kp7: 327,
    kp8: 328,
    kp9: 329,
    kpDecimal: 330,
    kpDivide: 331,
    kpMultiply: 332,
    kpSubtract: 333,
    kpAdd: 334,
    kpEnter: 335,
    kpEqual: 336,
    leftShift: 340,
    leftControl: 341,
    leftAlt: 342,
    leftSuper: 343,
    rightShift: 344,
    rightControl: 345,
    rightAlt: 346,
    rightSuper: 347,
    menu: 348,
};
var KeyModifiers = {
    none: 0,
    shift: 1 << 0,
    ctrl: 1 << 1,
    alt: 1 << 2,
    meta: 1 << 3,
};
// Physical key (KeyboardEvent.code) to Rive key, by US-QWERTY position.
var CODE_TO_KEY = {
    // Modifiers are keys too (the editor sends them), and are also in the bitmask.
    ShiftLeft: Key.leftShift,
    ShiftRight: Key.rightShift,
    ControlLeft: Key.leftControl,
    ControlRight: Key.rightControl,
    AltLeft: Key.leftAlt,
    AltRight: Key.rightAlt,
    MetaLeft: Key.leftSuper,
    MetaRight: Key.rightSuper,
    KeyA: Key.a,
    KeyB: Key.b,
    KeyC: Key.c,
    KeyD: Key.d,
    KeyE: Key.e,
    KeyF: Key.f,
    KeyG: Key.g,
    KeyH: Key.h,
    KeyI: Key.i,
    KeyJ: Key.j,
    KeyK: Key.k,
    KeyL: Key.l,
    KeyM: Key.m,
    KeyN: Key.n,
    KeyO: Key.o,
    KeyP: Key.p,
    KeyQ: Key.q,
    KeyR: Key.r,
    KeyS: Key.s,
    KeyT: Key.t,
    KeyU: Key.u,
    KeyV: Key.v,
    KeyW: Key.w,
    KeyX: Key.x,
    KeyY: Key.y,
    KeyZ: Key.z,
    Digit0: Key.key0,
    Digit1: Key.key1,
    Digit2: Key.key2,
    Digit3: Key.key3,
    Digit4: Key.key4,
    Digit5: Key.key5,
    Digit6: Key.key6,
    Digit7: Key.key7,
    Digit8: Key.key8,
    Digit9: Key.key9,
    Numpad0: Key.kp0,
    Numpad1: Key.kp1,
    Numpad2: Key.kp2,
    Numpad3: Key.kp3,
    Numpad4: Key.kp4,
    Numpad5: Key.kp5,
    Numpad6: Key.kp6,
    Numpad7: Key.kp7,
    Numpad8: Key.kp8,
    Numpad9: Key.kp9,
    NumpadDecimal: Key.kpDecimal,
    NumpadDivide: Key.kpDivide,
    NumpadMultiply: Key.kpMultiply,
    NumpadSubtract: Key.kpSubtract,
    NumpadAdd: Key.kpAdd,
    NumpadEnter: Key.kpEnter,
    NumpadEqual: Key.kpEqual,
    Minus: Key.minus,
    Equal: Key.equal,
    BracketLeft: Key.leftBracket,
    BracketRight: Key.rightBracket,
    Backslash: Key.backslash,
    Semicolon: Key.semicolon,
    Quote: Key.apostrophe,
    Backquote: Key.graveAccent,
    Comma: Key.comma,
    Period: Key.period,
    Slash: Key.slash,
    IntlBackslash: Key.world1,
    IntlRo: Key.world2,
    Space: Key.space,
    Enter: Key.enter,
    Tab: Key.tab,
    Backspace: Key.backspace,
    Insert: Key.insert,
    Delete: Key.deleteKey,
    Escape: Key.escape,
    ArrowRight: Key.right,
    ArrowLeft: Key.left,
    ArrowDown: Key.down,
    ArrowUp: Key.up,
    PageUp: Key.pageUp,
    PageDown: Key.pageDown,
    Home: Key.home,
    End: Key.end,
    CapsLock: Key.capsLock,
    ScrollLock: Key.scrollLock,
    NumLock: Key.numLock,
    PrintScreen: Key.printScreen,
    Pause: Key.pause,
    ContextMenu: Key.menu,
    F1: Key.f1,
    F2: Key.f2,
    F3: Key.f3,
    F4: Key.f4,
    F5: Key.f5,
    F6: Key.f6,
    F7: Key.f7,
    F8: Key.f8,
    F9: Key.f9,
    F10: Key.f10,
    F11: Key.f11,
    F12: Key.f12,
    F13: Key.f13,
    F14: Key.f14,
    F15: Key.f15,
    F16: Key.f16,
    F17: Key.f17,
    F18: Key.f18,
    F19: Key.f19,
    F20: Key.f20,
    F21: Key.f21,
    F22: Key.f22,
    F23: Key.f23,
    F24: Key.f24,
    F25: Key.f25,
};
// Unshifted characters (event.key, lowercased) that name a Rive key.
var CHAR_TO_KEY = (function () {
    var map = {
        " ": Key.space,
        "'": Key.apostrophe,
        ",": Key.comma,
        "-": Key.minus,
        ".": Key.period,
        "/": Key.slash,
        ";": Key.semicolon,
        "=": Key.equal,
        "[": Key.leftBracket,
        "\\": Key.backslash,
        "]": Key.rightBracket,
        "`": Key.graveAccent,
    };
    for (var c = 0; c < 26; c++)
        map[String.fromCharCode(97 + c)] = Key.a + c;
    for (var d = 0; d <= 9; d++)
        map[String(d)] = Key.key0 + d;
    return map;
})();
// Non-printable event.key values; each shares its name with its code.
var NAMED_KEYS = new Set(__spreadArray([
    "Enter",
    "Tab",
    "Backspace",
    "Insert",
    "Delete",
    "Escape",
    "ArrowRight",
    "ArrowLeft",
    "ArrowDown",
    "ArrowUp",
    "PageUp",
    "PageDown",
    "Home",
    "End",
    "CapsLock",
    "ScrollLock",
    "NumLock",
    "PrintScreen",
    "Pause",
    "ContextMenu"
], Array.from({ length: 25 }, function (_, i) { return "F".concat(i + 1); }), true));
/**
 * Maps a KeyboardEvent to a Rive key, or null if Rive has no equivalent.
 *
 * Resolves the logical key: what the user's layout labels the key, ignoring
 * Shift, so a listener on "A" fires on AZERTY's A too. `event.key` is preferred;
 * `event.code` is the fallback when `key` isn't a plain unshifted character
 * (shifted symbols, Option/AltGr output, non-Latin letters, dead keys).
 * Digit-row and numpad keys skip the character lookup so AZERTY's digit row
 * still reports digits and numpad digits stay distinct from the digit row.
 */
function keyboardEventToRiveKey(event) {
    var _a, _b;
    var code = event.code, key = event.key;
    if (NAMED_KEYS.has(key)) {
        // NumpadEnter reports key "Enter"; keep it distinct.
        return code === "NumpadEnter" ? Key.kpEnter : (_a = CODE_TO_KEY[key]) !== null && _a !== void 0 ? _a : null;
    }
    if (!code.startsWith("Digit") && !code.startsWith("Numpad") && key) {
        var fromChar = CHAR_TO_KEY[key.toLowerCase()];
        if (fromChar !== undefined)
            return fromChar;
    }
    return (_b = CODE_TO_KEY[code]) !== null && _b !== void 0 ? _b : null;
}
function modifiersFromEvent(event) {
    var modifiers = KeyModifiers.none;
    if (event.shiftKey)
        modifiers |= KeyModifiers.shift;
    if (event.ctrlKey)
        modifiers |= KeyModifiers.ctrl;
    if (event.altKey)
        modifiers |= KeyModifiers.alt;
    if (event.metaKey)
        modifiers |= KeyModifiers.meta;
    return modifiers;
}


/***/ }),
/* 16 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   TextInputProxy: () => (/* binding */ TextInputProxy)
/* harmony export */ });
/* harmony import */ var _canvasOffset__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(11);

// 16px: the smallest font size at which iOS Safari doesn't zoom on focus
var FONT_SIZE_PX = 16;
/** Capture-critical attributes decorate() must not touch. */
var PROTECTED_ATTRS = new Set([
    "type",
    "value",
    "style",
    "tabindex",
    "autocapitalize",
    "autocomplete",
    "autocorrect",
    "spellcheck",
]);
/**
 * Invisible per-instance <input> capturing IME/dead-key/paste text; focusable inside a
 * gesture to raise mobile keyboards. Always empty: C++ owns value and caret. Decorated
 * for AT only while it holds DOM focus.
 */
var TextInputProxy = /** @class */ (function () {
    function TextInputProxy(params) {
        var _this = this;
        // True between compositionstart/compositionend; we never commit mid-composition.
        this.composing = false;
        // Names set by decorate(), removed by resetDecoration().
        this.decoratedKeys = new Set();
        // Last written left/top, to skip no-op style writes.
        this.lastLeft = NaN;
        this.lastTop = NaN;
        /** Re-sync before input: the canvas may have scrolled since the session began. */
        this.reposition = function () { return _this.positionOverCanvas(); };
        this.onCompositionStart = function () {
            _this.composing = true;
        };
        this.onCompositionEnd = function () {
            _this.composing = false;
            // The composed text is now committed in element.value; forward it here. A
            // trailing `input` event (order varies by browser) then sees an empty field.
            _this.commit();
        };
        this.onInput = function () {
            if (_this.composing)
                return;
            _this.commit();
        };
        // An <input type=text> strips line breaks from its value, so forward the clipboard
        // text directly. Core's single-line fields strip them themselves.
        this.onPaste = function (event) {
            var _a;
            var text = (_a = event.clipboardData) === null || _a === void 0 ? void 0 : _a.getData("text/plain");
            if (!text)
                return;
            event.preventDefault();
            _this.params.textInput(text.replace(/\r\n?/g, "\n"));
        };
        this.onBlur = function (event) {
            _this.params.onBlur(event.relatedTarget);
        };
        this.params = params;
        this.element = this.createElement();
        this.positionOverCanvas();
        // Order vs. the overlay is unspecified; fine, Tab release starts from the canvas.
        this.params.container.appendChild(this.element);
    }
    // Registered first so it runs before the key handler inserts or moves the caret.
    TextInputProxy.prototype.listeners = function () {
        return [
            ["keydown", this.reposition],
            ["beforeinput", this.reposition],
            ["keydown", this.params.onKeyDown],
            ["keyup", this.params.onKeyUp],
            ["input", this.onInput],
            ["compositionstart", this.onCompositionStart],
            ["compositionend", this.onCompositionEnd],
            ["paste", this.onPaste],
            ["copy", this.params.onCopy],
            ["blur", this.onBlur],
        ];
    };
    TextInputProxy.prototype.createElement = function () {
        var el = document.createElement("input");
        el.type = "text";
        // Focusable but never in native tab order (Rive drives traversal synthetically),
        // and invisible without display:none (which would forbid focus).
        el.tabIndex = -1;
        el.setAttribute("aria-hidden", "true");
        // Always empty, so autocapitalize/autocorrect would misfire.
        el.setAttribute("autocapitalize", "off");
        el.setAttribute("autocomplete", "off");
        el.setAttribute("autocorrect", "off");
        el.spellcheck = false;
        // Over the canvas (not display:none) so iOS doesn't scroll-jump on keyboard open.
        // font-size must stay 16px or iOS Safari zooms on focus.
        el.style.cssText =
            "position:absolute;width:1px;height:1px;padding:0;margin:0;border:0;" +
                "outline:none;opacity:0;background:transparent;color:transparent;" +
                "caret-color:transparent;overflow:hidden;resize:none;z-index:0;" +
                // Focused programmatically only; must not swallow clicks
                "pointer-events:none;" +
                "font-size:".concat(FONT_SIZE_PX, "px;line-height:1;");
        for (var _i = 0, _a = this.listeners(); _i < _a.length; _i++) {
            var _b = _a[_i], type = _b[0], listener = _b[1];
            el.addEventListener(type, listener);
        }
        return el;
    };
    TextInputProxy.prototype.positionOverCanvas = function () {
        // Track the canvas (browsers scroll to the caret); clamp on-screen, one caret line box
        // (FONT_SIZE_PX) from the far edges, or the root scrolls to reveal it.
        var _a = (0,_canvasOffset__WEBPACK_IMPORTED_MODULE_0__.canvasOffset)(this.params.canvas), top = _a.top, left = _a.left;
        if (window.innerHeight > 0) {
            var r = this.params.canvas.getBoundingClientRect();
            top += clamp(r.top, 0, window.innerHeight - FONT_SIZE_PX) - r.top;
            left += clamp(r.left, 0, window.innerWidth - FONT_SIZE_PX) - r.left;
        }
        if (left !== this.lastLeft) {
            this.element.style.left = "".concat(left, "px");
            this.lastLeft = left;
        }
        if (top !== this.lastTop) {
            this.element.style.top = "".concat(top, "px");
            this.lastTop = top;
        }
    };
    /**
     * Forward the field's text to Rive and clear it.
     *
     * TODO: iOS won't autorepeat Backspace in an empty field (needs a text mirror).
     * */
    TextInputProxy.prototype.commit = function () {
        var text = this.element.value;
        if (!text)
            return;
        this.element.value = "";
        this.params.textInput(text);
    };
    /** Clear any transient value and decoration. Does not blur the element. */
    TextInputProxy.prototype.endSession = function () {
        this.element.value = "";
        // Browsers fire compositionend on blur, but a stuck flag would drop every
        // keystroke of the next session.
        this.composing = false;
        // Also covers blur-out, which bypasses the overlay.
        this.resetDecoration();
    };
    /** Must be called synchronously inside a gesture on iOS to raise the keyboard. */
    TextInputProxy.prototype.focus = function () {
        // Reposition first: iOS scrolls the focused input into view when the keyboard opens.
        this.positionOverCanvas();
        this.element.focus({ preventScroll: true });
    };
    TextInputProxy.prototype.hasDomFocus = function () {
        return document.activeElement === this.element;
    };
    TextInputProxy.prototype.owns = function (target) {
        return target === this.element;
    };
    /** See EditingHost.decorate. */
    TextInputProxy.prototype.decorate = function (attrs) {
        // aria-hidden outranks every attribute set below, so it must come off before this
        // element can stand in as the accessible element for the focused node.
        this.element.removeAttribute("aria-hidden");
        for (var _i = 0, _a = Object.entries(attrs); _i < _a.length; _i++) {
            var _b = _a[_i], name_1 = _b[0], value = _b[1];
            // Never override capture attrs.
            if (PROTECTED_ATTRS.has(name_1.toLowerCase()))
                continue;
            if (value === null) {
                this.element.removeAttribute(name_1);
                this.decoratedKeys.delete(name_1);
            }
            else {
                // Skip no-op writes: setting an attribute to the value it already has still
                // emits a mutation, and assistive tech is focused on this element mid-edit.
                if (this.element.getAttribute(name_1) !== value) {
                    this.element.setAttribute(name_1, value);
                }
                this.decoratedKeys.add(name_1);
            }
        }
    };
    /**
     * type=password for obscured fields so AT doesn't echo keys.
     * TODO: source "type" from Core, not semantics.
     */
    TextInputProxy.prototype.setSecure = function (secure) {
        var type = secure ? "password" : "text";
        if (this.element.type !== type)
            this.element.type = type;
    };
    /** Drop all decoration and return to the AT-invisible baseline. */
    TextInputProxy.prototype.resetDecoration = function () {
        var _this = this;
        this.setSecure(false);
        this.decoratedKeys.forEach(function (name) { return _this.element.removeAttribute(name); });
        this.decoratedKeys.clear();
        this.element.setAttribute("aria-hidden", "true");
    };
    TextInputProxy.prototype.cleanup = function () {
        for (var _i = 0, _a = this.listeners(); _i < _a.length; _i++) {
            var _b = _a[_i], type = _b[0], listener = _b[1];
            this.element.removeEventListener(type, listener);
        }
        this.element.remove();
    };
    return TextInputProxy;
}());

function clamp(v, min, max) {
    return Math.min(Math.max(v, min), Math.max(min, max));
}


/***/ }),
/* 17 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   BLANK_URL: () => (/* binding */ BLANK_URL),
/* harmony export */   sanitizeUrl: () => (/* binding */ sanitizeUrl)
/* harmony export */ });
// Reference: https://github.com/braintree/sanitize-url/tree/main
var invalidProtocolRegex = /^([^\w]*)(javascript|data|vbscript)/im;
var htmlEntitiesRegex = /&#(\w+)(^\w|;)?/g;
var htmlCtrlEntityRegex = /&(newline|tab);/gi;
var ctrlCharactersRegex = /[\u0000-\u001F\u007F-\u009F\u2000-\u200D\uFEFF]/gim;
var urlSchemeRegex = /^.+(:|&colon;)/gim;
var relativeFirstCharacters = [".", "/"];
var BLANK_URL = "about:blank";
function isRelativeUrlWithoutProtocol(url) {
    return relativeFirstCharacters.indexOf(url[0]) > -1;
}
// adapted from https://stackoverflow.com/a/29824550/2601552
function decodeHtmlCharacters(str) {
    var removedNullByte = str.replace(ctrlCharactersRegex, "");
    return removedNullByte.replace(htmlEntitiesRegex, function (match, dec) {
        return String.fromCharCode(dec);
    });
}
function sanitizeUrl(url) {
    if (!url) {
        return BLANK_URL;
    }
    var sanitizedUrl = decodeHtmlCharacters(url)
        .replace(htmlCtrlEntityRegex, "")
        .replace(ctrlCharactersRegex, "")
        .trim();
    if (!sanitizedUrl) {
        return BLANK_URL;
    }
    if (isRelativeUrlWithoutProtocol(sanitizedUrl)) {
        return sanitizedUrl;
    }
    var urlSchemeParseResults = sanitizedUrl.match(urlSchemeRegex);
    if (!urlSchemeParseResults) {
        return sanitizedUrl;
    }
    var urlScheme = urlSchemeParseResults[0];
    if (invalidProtocolRegex.test(urlScheme)) {
        return BLANK_URL;
    }
    return sanitizedUrl;
}


/***/ }),
/* 18 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   AudioAssetWrapper: () => (/* binding */ AudioAssetWrapper),
/* harmony export */   AudioWrapper: () => (/* binding */ AudioWrapper),
/* harmony export */   CustomFileAssetLoaderWrapper: () => (/* binding */ CustomFileAssetLoaderWrapper),
/* harmony export */   FileAssetWrapper: () => (/* binding */ FileAssetWrapper),
/* harmony export */   FileFinalizer: () => (/* binding */ FileFinalizer),
/* harmony export */   FontAssetWrapper: () => (/* binding */ FontAssetWrapper),
/* harmony export */   FontWrapper: () => (/* binding */ FontWrapper),
/* harmony export */   ImageAssetWrapper: () => (/* binding */ ImageAssetWrapper),
/* harmony export */   ImageWrapper: () => (/* binding */ ImageWrapper),
/* harmony export */   createFinalization: () => (/* binding */ createFinalization),
/* harmony export */   finalizationRegistry: () => (/* binding */ finalizationRegistry)
/* harmony export */ });
var __extends = (undefined && undefined.__extends) || (function () {
    var extendStatics = function (d, b) {
        extendStatics = Object.setPrototypeOf ||
            ({ __proto__: [] } instanceof Array && function (d, b) { d.__proto__ = b; }) ||
            function (d, b) { for (var p in b) if (Object.prototype.hasOwnProperty.call(b, p)) d[p] = b[p]; };
        return extendStatics(d, b);
    };
    return function (d, b) {
        if (typeof b !== "function" && b !== null)
            throw new TypeError("Class extends value " + String(b) + " is not a constructor or null");
        extendStatics(d, b);
        function __() { this.constructor = d; }
        d.prototype = b === null ? Object.create(b) : (__.prototype = b.prototype, new __());
    };
})();
var FileFinalizer = /** @class */ (function () {
    function FileFinalizer(file, session) {
        if (session === void 0) { session = null; }
        this.selfUnref = false;
        this._file = file;
        this._session = session;
    }
    FileFinalizer.prototype.unref = function () {
        if (this._file) {
            this._file.unref();
        }
        // After the file, never before: the file's resources belong to the session.
        if (this._session) {
            this._session.delete();
            this._session = null;
        }
    };
    // Disarms the GC path for a file cleanup() already released.
    FileFinalizer.prototype.release = function () {
        this._file = null;
        this._session = null;
    };
    return FileFinalizer;
}());
var ObjectFinalizer = /** @class */ (function () {
    function ObjectFinalizer(finalizableObject) {
        this._finalizableObject = finalizableObject;
    }
    ObjectFinalizer.prototype.unref = function () {
        this._finalizableObject.unref();
    };
    return ObjectFinalizer;
}());
var AssetWrapper = /** @class */ (function () {
    function AssetWrapper() {
        this.selfUnref = false;
    }
    AssetWrapper.prototype.unref = function () { };
    return AssetWrapper;
}());
var ImageWrapper = /** @class */ (function (_super) {
    __extends(ImageWrapper, _super);
    function ImageWrapper(image) {
        var _this = _super.call(this) || this;
        _this._nativeImage = image;
        return _this;
    }
    Object.defineProperty(ImageWrapper.prototype, "nativeImage", {
        get: function () {
            return this._nativeImage;
        },
        enumerable: false,
        configurable: true
    });
    ImageWrapper.prototype.unref = function () {
        if (this.selfUnref) {
            this._nativeImage.unref();
        }
    };
    return ImageWrapper;
}(AssetWrapper));
var AudioWrapper = /** @class */ (function (_super) {
    __extends(AudioWrapper, _super);
    function AudioWrapper(audio) {
        var _this = _super.call(this) || this;
        _this._nativeAudio = audio;
        return _this;
    }
    Object.defineProperty(AudioWrapper.prototype, "nativeAudio", {
        get: function () {
            return this._nativeAudio;
        },
        enumerable: false,
        configurable: true
    });
    AudioWrapper.prototype.unref = function () {
        if (this.selfUnref) {
            this._nativeAudio.unref();
        }
    };
    return AudioWrapper;
}(AssetWrapper));
var FontWrapper = /** @class */ (function (_super) {
    __extends(FontWrapper, _super);
    function FontWrapper(font) {
        var _this = _super.call(this) || this;
        _this._nativeFont = font;
        return _this;
    }
    Object.defineProperty(FontWrapper.prototype, "nativeFont", {
        get: function () {
            return this._nativeFont;
        },
        enumerable: false,
        configurable: true
    });
    FontWrapper.prototype.unref = function () {
        if (this.selfUnref) {
            this._nativeFont.unref();
        }
    };
    return FontWrapper;
}(AssetWrapper));
var CustomFileAssetLoaderWrapper = /** @class */ (function () {
    function CustomFileAssetLoaderWrapper(runtime, loaderCallback, session) {
        if (session === void 0) { session = null; }
        this._assetLoaderCallback = loaderCallback;
        this._session = session;
        this.assetLoader = new runtime.CustomFileAssetLoader({
            loadContents: this.loadContents.bind(this),
        });
    }
    CustomFileAssetLoaderWrapper.prototype.loadContents = function (asset, bytes) {
        var assetWrapper;
        if (asset.isImage) {
            assetWrapper = new ImageAssetWrapper(asset, this._session);
        }
        else if (asset.isAudio) {
            assetWrapper = new AudioAssetWrapper(asset, this._session);
        }
        else if (asset.isFont) {
            assetWrapper = new FontAssetWrapper(asset, this._session);
        }
        else {
            return false;
        }
        return this._assetLoaderCallback(assetWrapper, bytes);
    };
    return CustomFileAssetLoaderWrapper;
}());
/**
 * Rive class representing a FileAsset with relevant metadata fields to describe
 * an asset associated wtih the Rive File
 */
var FileAssetWrapper = /** @class */ (function () {
    function FileAssetWrapper(nativeAsset, session) {
        if (session === void 0) { session = null; }
        this._nativeFileAsset = nativeAsset;
        this._session = session;
    }
    FileAssetWrapper.prototype.decode = function (bytes) {
        // The session travels with the asset so a deferred file's assets are
        // session-typed without the caller needing to know the file's rendering mode.
        this._nativeFileAsset.decode(bytes, this._session);
    };
    Object.defineProperty(FileAssetWrapper.prototype, "name", {
        get: function () {
            return this._nativeFileAsset.name;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(FileAssetWrapper.prototype, "fileExtension", {
        get: function () {
            return this._nativeFileAsset.fileExtension;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(FileAssetWrapper.prototype, "uniqueFilename", {
        get: function () {
            return this._nativeFileAsset.uniqueFilename;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(FileAssetWrapper.prototype, "isAudio", {
        get: function () {
            return this._nativeFileAsset.isAudio;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(FileAssetWrapper.prototype, "isImage", {
        get: function () {
            return this._nativeFileAsset.isImage;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(FileAssetWrapper.prototype, "isFont", {
        get: function () {
            return this._nativeFileAsset.isFont;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(FileAssetWrapper.prototype, "cdnUuid", {
        get: function () {
            return this._nativeFileAsset.cdnUuid;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(FileAssetWrapper.prototype, "nativeFileAsset", {
        get: function () {
            return this._nativeFileAsset;
        },
        enumerable: false,
        configurable: true
    });
    return FileAssetWrapper;
}());
/**
 * Rive class extending the FileAsset that exposes a `setRenderImage()` API with a
 * decoded Image (via the `decodeImage()` API) to set a new Image on the Rive FileAsset
 */
var ImageAssetWrapper = /** @class */ (function (_super) {
    __extends(ImageAssetWrapper, _super);
    function ImageAssetWrapper() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    ImageAssetWrapper.prototype.setRenderImage = function (image) {
        this._nativeFileAsset.setRenderImage(image.nativeImage);
    };
    return ImageAssetWrapper;
}(FileAssetWrapper));
/**
 * Rive class extending the FileAsset that exposes a `setAudioSource()` API with a
 * decoded Audio (via the `decodeAudio()` API) to set a new Audio on the Rive FileAsset
 */
var AudioAssetWrapper = /** @class */ (function (_super) {
    __extends(AudioAssetWrapper, _super);
    function AudioAssetWrapper() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    AudioAssetWrapper.prototype.setAudioSource = function (audio) {
        this._nativeFileAsset.setAudioSource(audio.nativeAudio);
    };
    return AudioAssetWrapper;
}(FileAssetWrapper));
/**
 * Rive class extending the FileAsset that exposes a `setFont()` API with a
 * decoded Font (via the `decodeFont()` API) to set a new Font on the Rive FileAsset
 */
var FontAssetWrapper = /** @class */ (function (_super) {
    __extends(FontAssetWrapper, _super);
    function FontAssetWrapper() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    FontAssetWrapper.prototype.setFont = function (font) {
        this._nativeFileAsset.setFont(font.nativeFont);
    };
    return FontAssetWrapper;
}(FileAssetWrapper));
var FakeFinalizationRegistry = /** @class */ (function () {
    function FakeFinalizationRegistry(_) {
    }
    FakeFinalizationRegistry.prototype.register = function (object) {
        object.selfUnref = true;
    };
    FakeFinalizationRegistry.prototype.unregister = function (_) { };
    return FakeFinalizationRegistry;
}());
var MyFinalizationRegistry = typeof FinalizationRegistry !== "undefined"
    ? FinalizationRegistry
    : FakeFinalizationRegistry;
var finalizationRegistry = new MyFinalizationRegistry(function (ob) {
    ob === null || ob === void 0 ? void 0 : ob.unref();
});
var createFinalization = function (target, finalizable) {
    var finalizer = new ObjectFinalizer(finalizable);
    finalizationRegistry.register(target, finalizer);
};



/***/ }),
/* 19 */
/***/ ((__unused_webpack_module, __webpack_exports__, __webpack_require__) => {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   RiveFont: () => (/* binding */ RiveFont)
/* harmony export */ });
/* harmony import */ var _runtimeLoader__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(3);

// Class to manage fallback fonts for Rive.
var RiveFont = /** @class */ (function () {
    // Class is never instantiated
    function RiveFont() {
    }
    /**
     * Set a callback to dynamically set a list of fallback fonts based on the missing glyph and/or style of the default font.
     * Set null to clear the callback.
     * @param fontCallback Callback to set a list of fallback fonts.
     */
    RiveFont.setFallbackFontCallback = function (fontCallback) {
        RiveFont._fallbackFontCallback = fontCallback !== null && fontCallback !== void 0 ? fontCallback : null;
        RiveFont._wireFallbackProc();
    };
    // Get the pointer value to the Embind Font object from FontWrapper
    RiveFont._fontToPtr = function (fontWrapper) {
        var _a;
        if (fontWrapper == null)
            return null;
        var embindFont = fontWrapper.nativeFont;
        var ptr = (_a = embindFont === null || embindFont === void 0 ? void 0 : embindFont.ptr) === null || _a === void 0 ? void 0 : _a.call(embindFont);
        return ptr !== null && ptr !== void 0 ? ptr : null;
    };
    RiveFont._getFallbackPtr = function (fonts, index) {
        if (index < 0 || index >= fonts.length)
            return null;
        return RiveFont._fontToPtr(fonts[index]);
    };
    // Create the callback Rive expects to use for fallback fonts (regardless if set via a user-supplied static list, or callback)
    // 1. Ensure WASM is ready
    // 2. Bias for checking user callback over static list of fonts and pass it down to Rive to store as reference
    //    - When calling the user callback, check if we have any fonts left to check, and if not, return null to indicate there are no more fallbacks to try.
    //    - If the user callback returns an array of fonts, pass the pointer value to Rive of the font to try
    // 3. If no callback is provided, or the callback returns null, try the static list of fonts if they set any
    // 4. If no fallback method is set, return null.
    RiveFont._wireFallbackProc = function () {
        _runtimeLoader__WEBPACK_IMPORTED_MODULE_0__.RuntimeLoader.getInstance(function (rive) {
            var cb = RiveFont._fallbackFontCallback;
            if (cb) {
                rive.setFallbackFontCallback((function (missingGlyph, fallbackFontIndex, weight) {
                    var fontsReturned = cb(missingGlyph, weight);
                    if (fontsReturned) {
                        if (Array.isArray(fontsReturned)) {
                            return RiveFont._getFallbackPtr(fontsReturned, fallbackFontIndex);
                        }
                        // If the user callback only returns a single font, provide it to Rive the first time, otherwise if Rive
                        // calls back a second time, return null to indicate there are no more fallbacks to try.
                        return fallbackFontIndex === 0 ? RiveFont._fontToPtr(fontsReturned) : null;
                    }
                    return null;
                }));
            }
            else {
                rive.setFallbackFontCallback(null);
            }
        });
    };
    RiveFont._fallbackFontCallback = null;
    return RiveFont;
}());



/***/ })
/******/ 	]);
/************************************************************************/
/******/ 	// The module cache
/******/ 	var __webpack_module_cache__ = {};
/******/ 	
/******/ 	// The require function
/******/ 	function __webpack_require__(moduleId) {
/******/ 		// Check if module is in cache
/******/ 		var cachedModule = __webpack_module_cache__[moduleId];
/******/ 		if (cachedModule !== undefined) {
/******/ 			return cachedModule.exports;
/******/ 		}
/******/ 		// Create a new module (and put it into the cache)
/******/ 		var module = __webpack_module_cache__[moduleId] = {
/******/ 			// no module.id needed
/******/ 			// no module.loaded needed
/******/ 			exports: {}
/******/ 		};
/******/ 	
/******/ 		// Execute the module function
/******/ 		__webpack_modules__[moduleId](module, module.exports, __webpack_require__);
/******/ 	
/******/ 		// Return the exports of the module
/******/ 		return module.exports;
/******/ 	}
/******/ 	
/************************************************************************/
/******/ 	/* webpack/runtime/define property getters */
/******/ 	(() => {
/******/ 		// define getter functions for harmony exports
/******/ 		__webpack_require__.d = (exports, definition) => {
/******/ 			for(var key in definition) {
/******/ 				if(__webpack_require__.o(definition, key) && !__webpack_require__.o(exports, key)) {
/******/ 					Object.defineProperty(exports, key, { enumerable: true, get: definition[key] });
/******/ 				}
/******/ 			}
/******/ 		};
/******/ 	})();
/******/ 	
/******/ 	/* webpack/runtime/hasOwnProperty shorthand */
/******/ 	(() => {
/******/ 		__webpack_require__.o = (obj, prop) => (Object.prototype.hasOwnProperty.call(obj, prop))
/******/ 	})();
/******/ 	
/******/ 	/* webpack/runtime/make namespace object */
/******/ 	(() => {
/******/ 		// define __esModule on exports
/******/ 		__webpack_require__.r = (exports) => {
/******/ 			if(typeof Symbol !== 'undefined' && Symbol.toStringTag) {
/******/ 				Object.defineProperty(exports, Symbol.toStringTag, { value: 'Module' });
/******/ 			}
/******/ 			Object.defineProperty(exports, '__esModule', { value: true });
/******/ 		};
/******/ 	})();
/******/ 	
/************************************************************************/
var __webpack_exports__ = {};
// This entry needs to be wrapped in an IIFE because it needs to be isolated against other modules in the chunk.
(() => {
__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   Alignment: () => (/* binding */ Alignment),
/* harmony export */   DataEnum: () => (/* binding */ DataEnum),
/* harmony export */   DataType: () => (/* binding */ DataType),
/* harmony export */   DeprecationKeys: () => (/* binding */ DeprecationKeys),
/* harmony export */   DrawOptimizationOptions: () => (/* binding */ DrawOptimizationOptions),
/* harmony export */   EventType: () => (/* binding */ EventType),
/* harmony export */   Fit: () => (/* binding */ Fit),
/* harmony export */   Layout: () => (/* binding */ Layout),
/* harmony export */   LoopType: () => (/* binding */ LoopType),
/* harmony export */   Rive: () => (/* binding */ Rive),
/* harmony export */   RiveEventType: () => (/* binding */ RiveEventType),
/* harmony export */   RiveFile: () => (/* binding */ RiveFile),
/* harmony export */   RiveFont: () => (/* reexport safe */ _utils__WEBPACK_IMPORTED_MODULE_3__.RiveFont),
/* harmony export */   RuntimeLoader: () => (/* reexport safe */ _runtimeLoader__WEBPACK_IMPORTED_MODULE_1__.RuntimeLoader),
/* harmony export */   SemanticMode: () => (/* reexport safe */ _semantics__WEBPACK_IMPORTED_MODULE_2__.SemanticMode),
/* harmony export */   StateMachineInput: () => (/* binding */ StateMachineInput),
/* harmony export */   StateMachineInputType: () => (/* binding */ StateMachineInputType),
/* harmony export */   Testing: () => (/* binding */ Testing),
/* harmony export */   ViewModel: () => (/* binding */ ViewModel),
/* harmony export */   ViewModelInstance: () => (/* binding */ ViewModelInstance),
/* harmony export */   ViewModelInstanceArtboard: () => (/* binding */ ViewModelInstanceArtboard),
/* harmony export */   ViewModelInstanceAssetFont: () => (/* binding */ ViewModelInstanceAssetFont),
/* harmony export */   ViewModelInstanceAssetImage: () => (/* binding */ ViewModelInstanceAssetImage),
/* harmony export */   ViewModelInstanceBoolean: () => (/* binding */ ViewModelInstanceBoolean),
/* harmony export */   ViewModelInstanceColor: () => (/* binding */ ViewModelInstanceColor),
/* harmony export */   ViewModelInstanceEnum: () => (/* binding */ ViewModelInstanceEnum),
/* harmony export */   ViewModelInstanceList: () => (/* binding */ ViewModelInstanceList),
/* harmony export */   ViewModelInstanceNumber: () => (/* binding */ ViewModelInstanceNumber),
/* harmony export */   ViewModelInstanceString: () => (/* binding */ ViewModelInstanceString),
/* harmony export */   ViewModelInstanceTrigger: () => (/* binding */ ViewModelInstanceTrigger),
/* harmony export */   ViewModelInstanceValue: () => (/* binding */ ViewModelInstanceValue),
/* harmony export */   decodeAudio: () => (/* binding */ decodeAudio),
/* harmony export */   decodeFont: () => (/* binding */ decodeFont),
/* harmony export */   decodeImage: () => (/* binding */ decodeImage)
/* harmony export */ });
/* harmony import */ var _animation__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(1);
/* harmony import */ var _runtimeLoader__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(3);
/* harmony import */ var _semantics__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(6);
/* harmony import */ var _utils__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__(12);
var __extends = (undefined && undefined.__extends) || (function () {
    var extendStatics = function (d, b) {
        extendStatics = Object.setPrototypeOf ||
            ({ __proto__: [] } instanceof Array && function (d, b) { d.__proto__ = b; }) ||
            function (d, b) { for (var p in b) if (Object.prototype.hasOwnProperty.call(b, p)) d[p] = b[p]; };
        return extendStatics(d, b);
    };
    return function (d, b) {
        if (typeof b !== "function" && b !== null)
            throw new TypeError("Class extends value " + String(b) + " is not a constructor or null");
        extendStatics(d, b);
        function __() { this.constructor = d; }
        d.prototype = b === null ? Object.create(b) : (__.prototype = b.prototype, new __());
    };
})();
var __assign = (undefined && undefined.__assign) || function () {
    __assign = Object.assign || function(t) {
        for (var s, i = 1, n = arguments.length; i < n; i++) {
            s = arguments[i];
            for (var p in s) if (Object.prototype.hasOwnProperty.call(s, p))
                t[p] = s[p];
        }
        return t;
    };
    return __assign.apply(this, arguments);
};
var __awaiter = (undefined && undefined.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __generator = (undefined && undefined.__generator) || function (thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g = Object.create((typeof Iterator === "function" ? Iterator : Object).prototype);
    return g.next = verb(0), g["throw"] = verb(1), g["return"] = verb(2), typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (g && (g = 0, op[0] && (_ = 0)), _) try {
            if (f = 1, y && (t = op[0] & 2 ? y["return"] : op[0] ? y["throw"] || ((t = y["return"]) && t.call(y), 0) : y.next) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [op[0] & 2, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
};
var __spreadArray = (undefined && undefined.__spreadArray) || function (to, from, pack) {
    if (pack || arguments.length === 2) for (var i = 0, l = from.length, ar; i < l; i++) {
        if (ar || !(i in from)) {
            if (!ar) ar = Array.prototype.slice.call(from, 0, i);
            ar[i] = from[i];
        }
    }
    return to.concat(ar || Array.prototype.slice.call(from));
};




var RiveError = /** @class */ (function (_super) {
    __extends(RiveError, _super);
    function RiveError() {
        var _this = _super !== null && _super.apply(this, arguments) || this;
        _this.isHandledError = true;
        return _this;
    }
    return RiveError;
}(Error));


// #regions helpers
var resolveErrorMessage = function (error) {
    return error && error.isHandledError
        ? error.message
        : "Problem loading file; may be corrupt!";
};
// #region layout
// Fit options for the canvas
var Fit;
(function (Fit) {
    Fit["Cover"] = "cover";
    Fit["Contain"] = "contain";
    Fit["Fill"] = "fill";
    Fit["FitWidth"] = "fitWidth";
    Fit["FitHeight"] = "fitHeight";
    Fit["None"] = "none";
    Fit["ScaleDown"] = "scaleDown";
    Fit["Layout"] = "layout";
})(Fit || (Fit = {}));
// Alignment options for the canvas
var Alignment;
(function (Alignment) {
    Alignment["Center"] = "center";
    Alignment["TopLeft"] = "topLeft";
    Alignment["TopCenter"] = "topCenter";
    Alignment["TopRight"] = "topRight";
    Alignment["CenterLeft"] = "centerLeft";
    Alignment["CenterRight"] = "centerRight";
    Alignment["BottomLeft"] = "bottomLeft";
    Alignment["BottomCenter"] = "bottomCenter";
    Alignment["BottomRight"] = "bottomRight";
})(Alignment || (Alignment = {}));
// Drawing optimization options
var DrawOptimizationOptions;
(function (DrawOptimizationOptions) {
    DrawOptimizationOptions["AlwaysDraw"] = "alwaysDraw";
    DrawOptimizationOptions["DrawOnChanged"] = "drawOnChanged";
})(DrawOptimizationOptions || (DrawOptimizationOptions = {}));
// Alignment options for Rive animations in a HTML canvas
var Layout = /** @class */ (function () {
    function Layout(params) {
        var _a, _b, _c, _d, _e, _f, _g;
        this.fit = (_a = params === null || params === void 0 ? void 0 : params.fit) !== null && _a !== void 0 ? _a : Fit.Contain;
        this.alignment = (_b = params === null || params === void 0 ? void 0 : params.alignment) !== null && _b !== void 0 ? _b : Alignment.Center;
        this.layoutScaleFactor = (_c = params === null || params === void 0 ? void 0 : params.layoutScaleFactor) !== null && _c !== void 0 ? _c : 1;
        this.minX = (_d = params === null || params === void 0 ? void 0 : params.minX) !== null && _d !== void 0 ? _d : 0;
        this.minY = (_e = params === null || params === void 0 ? void 0 : params.minY) !== null && _e !== void 0 ? _e : 0;
        this.maxX = (_f = params === null || params === void 0 ? void 0 : params.maxX) !== null && _f !== void 0 ? _f : 0;
        this.maxY = (_g = params === null || params === void 0 ? void 0 : params.maxY) !== null && _g !== void 0 ? _g : 0;
    }
    // Alternative constructor to build a Layout from an interface/object
    Layout.new = function (_a) {
        var fit = _a.fit, alignment = _a.alignment, minX = _a.minX, minY = _a.minY, maxX = _a.maxX, maxY = _a.maxY;
        warnOnce(DeprecationKeys.legacyConstructors, "This function is deprecated: please use `new Layout({})` instead");
        return new Layout({ fit: fit, alignment: alignment, minX: minX, minY: minY, maxX: maxX, maxY: maxY });
    };
    /**
     * Makes a copy of the layout, replacing any specified parameters
     */
    Layout.prototype.copyWith = function (_a) {
        var fit = _a.fit, alignment = _a.alignment, layoutScaleFactor = _a.layoutScaleFactor, minX = _a.minX, minY = _a.minY, maxX = _a.maxX, maxY = _a.maxY;
        return new Layout({
            fit: fit !== null && fit !== void 0 ? fit : this.fit,
            alignment: alignment !== null && alignment !== void 0 ? alignment : this.alignment,
            layoutScaleFactor: layoutScaleFactor !== null && layoutScaleFactor !== void 0 ? layoutScaleFactor : this.layoutScaleFactor,
            minX: minX !== null && minX !== void 0 ? minX : this.minX,
            minY: minY !== null && minY !== void 0 ? minY : this.minY,
            maxX: maxX !== null && maxX !== void 0 ? maxX : this.maxX,
            maxY: maxY !== null && maxY !== void 0 ? maxY : this.maxY,
        });
    };
    // Returns fit for the Wasm runtime format
    Layout.prototype.runtimeFit = function (rive) {
        if (this.cachedRuntimeFit)
            return this.cachedRuntimeFit;
        var fit;
        if (this.fit === Fit.Cover)
            fit = rive.Fit.cover;
        else if (this.fit === Fit.Contain)
            fit = rive.Fit.contain;
        else if (this.fit === Fit.Fill)
            fit = rive.Fit.fill;
        else if (this.fit === Fit.FitWidth)
            fit = rive.Fit.fitWidth;
        else if (this.fit === Fit.FitHeight)
            fit = rive.Fit.fitHeight;
        else if (this.fit === Fit.ScaleDown)
            fit = rive.Fit.scaleDown;
        else if (this.fit === Fit.Layout)
            fit = rive.Fit.layout;
        else
            fit = rive.Fit.none;
        this.cachedRuntimeFit = fit;
        return fit;
    };
    // Returns alignment for the Wasm runtime format
    Layout.prototype.runtimeAlignment = function (rive) {
        if (this.cachedRuntimeAlignment)
            return this.cachedRuntimeAlignment;
        var alignment;
        if (this.alignment === Alignment.TopLeft)
            alignment = rive.Alignment.topLeft;
        else if (this.alignment === Alignment.TopCenter)
            alignment = rive.Alignment.topCenter;
        else if (this.alignment === Alignment.TopRight)
            alignment = rive.Alignment.topRight;
        else if (this.alignment === Alignment.CenterLeft)
            alignment = rive.Alignment.centerLeft;
        else if (this.alignment === Alignment.CenterRight)
            alignment = rive.Alignment.centerRight;
        else if (this.alignment === Alignment.BottomLeft)
            alignment = rive.Alignment.bottomLeft;
        else if (this.alignment === Alignment.BottomCenter)
            alignment = rive.Alignment.bottomCenter;
        else if (this.alignment === Alignment.BottomRight)
            alignment = rive.Alignment.bottomRight;
        else
            alignment = rive.Alignment.center;
        this.cachedRuntimeAlignment = alignment;
        return alignment;
    };
    return Layout;
}());

// #endregion
// #region runtime

// #endregion
// #region state machines
/**
 * @deprecated State machine inputs are deprecated and will be removed in a
 * future major version: please use data binding properties instead. See
 * {@link https://rive.app/docs/editor/data-binding/migration-guide#state-machine-inputs}
 * for how to migrate.
 */
var StateMachineInputType;
(function (StateMachineInputType) {
    StateMachineInputType[StateMachineInputType["Number"] = 56] = "Number";
    StateMachineInputType[StateMachineInputType["Trigger"] = 58] = "Trigger";
    StateMachineInputType[StateMachineInputType["Boolean"] = 59] = "Boolean";
})(StateMachineInputType || (StateMachineInputType = {}));
/**
 * An input for a state machine
 * @deprecated State machine inputs are deprecated and will be removed in a
 * future major version: please use data binding properties instead. See
 * {@link https://rive.app/docs/editor/data-binding/migration-guide#state-machine-inputs}
 * for how to migrate.
 */
var StateMachineInput = /** @class */ (function () {
    function StateMachineInput(type, runtimeInput) {
        this.type = type;
        this.runtimeInput = runtimeInput;
    }
    Object.defineProperty(StateMachineInput.prototype, "name", {
        /**
         * Returns the name of the input
         */
        get: function () {
            return this.runtimeInput.name;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(StateMachineInput.prototype, "value", {
        /**
         * Returns the current value of the input
         */
        get: function () {
            return this.runtimeInput.value;
        },
        /**
         * Sets the value of the input
         */
        set: function (value) {
            this.runtimeInput.value = value;
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Fires a trigger; does nothing on Number or Boolean input types
     */
    StateMachineInput.prototype.fire = function () {
        if (this.type === StateMachineInputType.Trigger) {
            this.runtimeInput.fire();
        }
    };
    /**
     * Deletes the input
     */
    StateMachineInput.prototype.delete = function () {
        this.runtimeInput = null;
    };
    return StateMachineInput;
}());

/**
 * @deprecated Subscribing to Rive Events at runtime is deprecated and will be removed in a future major
 * version: please use data binding instead. See
 * {@link https://rive.app/docs/runtimes/web/rive-events} for how to migrate.
 */
var RiveEventType;
(function (RiveEventType) {
    RiveEventType[RiveEventType["General"] = 128] = "General";
    RiveEventType[RiveEventType["OpenUrl"] = 131] = "OpenUrl";
})(RiveEventType || (RiveEventType = {}));
var BaseArtboard = /** @class */ (function () {
    function BaseArtboard(_isBindableArtboard) {
        this.isBindableArtboard = false;
        this.isBindableArtboard = _isBindableArtboard;
    }
    return BaseArtboard;
}());
var Artboard = /** @class */ (function (_super) {
    __extends(Artboard, _super);
    function Artboard(artboard, _file) {
        var _this = _super.call(this, false) || this;
        _this.nativeArtboard = artboard;
        _this.file = _file;
        return _this;
    }
    return Artboard;
}(BaseArtboard));
var BindableArtboard = /** @class */ (function (_super) {
    __extends(BindableArtboard, _super);
    function BindableArtboard(artboard) {
        var _this = _super.call(this, true) || this;
        _this.selfUnref = false;
        _this.nativeArtboard = artboard;
        return _this;
    }
    Object.defineProperty(BindableArtboard.prototype, "viewModel", {
        set: function (value) {
            this.nativeViewModel = value.nativeInstance;
        },
        enumerable: false,
        configurable: true
    });
    BindableArtboard.prototype.destroy = function () {
        var _a;
        if (this.selfUnref) {
            this.nativeArtboard.unref();
            (_a = this.nativeViewModel) === null || _a === void 0 ? void 0 : _a.unref();
        }
    };
    return BindableArtboard;
}(BaseArtboard));
var StateMachine = /** @class */ (function () {
    /**
     * @constructor
     * @param stateMachine runtime state machine object
     * @param instance runtime state machine instance object
     */
    function StateMachine(stateMachine, runtime, playing, artboard) {
        this.stateMachine = stateMachine;
        this.playing = playing;
        this.artboard = artboard;
        /**
         * Caches the inputs from the runtime
         */
        this.inputs = [];
        this.instance = new runtime.StateMachineInstance(stateMachine, artboard);
        this.initInputs(runtime);
    }
    Object.defineProperty(StateMachine.prototype, "hasFocusNodes", {
        /**
         * Whether this state machine has focus nodes
         */
        get: function () {
            return this.instance.hasFocusNodes();
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(StateMachine.prototype, "name", {
        get: function () {
            return this.stateMachine.name;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(StateMachine.prototype, "statesChanged", {
        /**
         * Returns a list of state names that have changed on this frame
         */
        get: function () {
            var names = [];
            for (var i = 0; i < this.instance.stateChangedCount(); i++) {
                names.push(this.instance.stateChangedNameByIndex(i));
            }
            return names;
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Advances the state machine instance by a given time.
     * @param time - the time to advance the animation by in seconds
     */
    StateMachine.prototype.advance = function (time) {
        this.instance.advance(time);
    };
    /**
     * Advances the state machine instance by a given time and apply changes to artboard.
     * @param time - the time to advance the animation by in seconds
     */
    StateMachine.prototype.advanceAndApply = function (time) {
        this.instance.advanceAndApply(time);
    };
    /**
     * Enables semantic tree tracking for this state machine instance.
     */
    StateMachine.prototype.enableSemantics = function () {
        this.instance.enableSemantics();
    };
    /**
     * Returns the incremental semantic diff since the last call, or null
     * if semantics is not enabled or nothing changed.
     */
    StateMachine.prototype.drainSemanticsDiff = function () {
        return this.instance.drainSemanticsDiff();
    };
    /**
     * Fire a semantic action (tap, increase, decrease) on a node.
     * @param nodeId - The semantic node ID to target
     * @param actionType - 0 = tap, 1 = increase, 2 = decrease
     */
    StateMachine.prototype.fireSemanticAction = function (nodeId, actionType) {
        this.instance.fireSemanticAction(nodeId, actionType);
    };
    /**
     * When tools that enable accessible experiences traverse elements with focus,
     * we should call this method to focus the semantic node. It will also trigger
     * focus on any Focus listeners for this node
     * @param nodeId ID of the Semantic Node to focus
     * @returns boolean - True if focus was set, false otherwise
     */
    StateMachine.prototype.focusSemanticNode = function (nodeId) {
        return this.instance.focusSemanticNode(nodeId);
    };
    /**
     * Returns the number of events reported from the last advance call
     * @returns Number of events reported
     */
    StateMachine.prototype.reportedEventCount = function () {
        return this.instance.reportedEventCount();
    };
    /**
     * Returns a RiveEvent object emitted from the last advance call at the given index
     * of a list of potentially multiple events. If an event at the index is not found,
     * undefined is returned.
     * @param i index of the event reported in a list of potentially multiple events
     * @returns RiveEvent or extended RiveEvent object returned, or undefined
     */
    StateMachine.prototype.reportedEventAt = function (i) {
        return this.instance.reportedEventAt(i);
    };
    /**
     * Fetches references to the state machine's inputs and caches them
     * @param runtime an instance of the runtime; needed for the SMIInput types
     */
    StateMachine.prototype.initInputs = function (runtime) {
        // Fetch the inputs from the runtime if we don't have them
        for (var i = 0; i < this.instance.inputCount(); i++) {
            var input = this.instance.input(i);
            this.inputs.push(this.mapRuntimeInput(input, runtime));
        }
    };
    /**
     * Maps a runtime input to it's appropriate type
     * @param input
     */
    StateMachine.prototype.mapRuntimeInput = function (input, runtime) {
        if (input.type === runtime.SMIInput.bool) {
            return new StateMachineInput(StateMachineInputType.Boolean, input.asBool());
        }
        else if (input.type === runtime.SMIInput.number) {
            return new StateMachineInput(StateMachineInputType.Number, input.asNumber());
        }
        else if (input.type === runtime.SMIInput.trigger) {
            return new StateMachineInput(StateMachineInputType.Trigger, input.asTrigger());
        }
    };
    /**
     * Deletes the backing Wasm state machine instance; once this is called, this
     * state machine is no more.
     */
    StateMachine.prototype.cleanup = function () {
        this.inputs.forEach(function (input) {
            input.delete();
        });
        this.inputs.length = 0;
        this.instance.delete();
    };
    StateMachine.prototype.bindViewModelInstance = function (viewModelInstance) {
        if (viewModelInstance.runtimeInstance != null) {
            this.instance.bindViewModelInstance(viewModelInstance.runtimeInstance);
        }
    };
    /**
     * Get metadata about the state of focus if applicable for this state machine.
     * @returns FocusState - { hasFocus: boolean, expectsKeyboardInput: boolean }
     */
    StateMachine.prototype.focusState = function () {
        return this.instance.focusState();
    };
    /**
     * Clear focus from the Rive focus node tree.
     */
    StateMachine.prototype.clearFocus = function () {
        this.instance.clearFocus();
    };
    return StateMachine;
}());
// #endregion
// #region animator
/**
 * Manages animation
 */
var Animator = /** @class */ (function () {
    /**
     * Constructs a new animator
     * @constructor
     * @param runtime Rive runtime; needed to instance animations & state machines
     * @param artboard the artboard that holds all animations and state machines
     * @param animations optional list of animations
     * @param stateMachines optional list of state machines
     */
    function Animator(runtime, artboard, eventManager, animations, stateMachines) {
        if (animations === void 0) { animations = []; }
        if (stateMachines === void 0) { stateMachines = []; }
        this.runtime = runtime;
        this.artboard = artboard;
        this.eventManager = eventManager;
        this.animations = animations;
        this.stateMachines = stateMachines;
    }
    /**
     * Adds animations and state machines by their names. If names are shared
     * between animations & state machines, then the first one found will be
     * created. Best not to use the same names for these in your Rive file.
     * @param animatable the name(s) of animations and state machines to add
     * @returns a list of names of the playing animations and state machines
     */
    Animator.prototype.add = function (animatables, playing, fireEvent, semanticsActive) {
        if (fireEvent === void 0) { fireEvent = true; }
        if (semanticsActive === void 0) { semanticsActive = false; }
        animatables = mapToStringArray(animatables);
        // If animatables is empty, play or pause everything
        if (animatables.length === 0) {
            this.animations.forEach(function (a) { return (a.playing = playing); });
            this.stateMachines.forEach(function (m) { return (m.playing = playing); });
        }
        else {
            // Play/pause already instanced items, or create new instances
            var instancedAnimationNames = this.animations.map(function (a) { return a.name; });
            var instancedMachineNames = this.stateMachines.map(function (m) { return m.name; });
            for (var i = 0; i < animatables.length; i++) {
                var aIndex = instancedAnimationNames.indexOf(animatables[i]);
                var mIndex = instancedMachineNames.indexOf(animatables[i]);
                if (aIndex >= 0 || mIndex >= 0) {
                    if (aIndex >= 0) {
                        // Animation is instanced, play/pause it
                        this.animations[aIndex].playing = playing;
                    }
                    else {
                        // State machine is instanced, play/pause it
                        this.stateMachines[mIndex].playing = playing;
                    }
                }
                else {
                    // Try to create a new animation instance
                    var anim = this.artboard.animationByName(animatables[i]);
                    if (anim) {
                        var newAnimation = new _animation__WEBPACK_IMPORTED_MODULE_0__.Animation(anim, this.artboard, this.runtime, playing);
                        // Display the first frame of the specified animation
                        newAnimation.advance(0);
                        newAnimation.apply(1.0);
                        this.animations.push(newAnimation);
                    }
                    else {
                        // Try to create a new state machine instance
                        var sm = this.artboard.stateMachineByName(animatables[i]);
                        if (sm) {
                            var newStateMachine = new StateMachine(sm, this.runtime, playing, this.artboard);
                            if (semanticsActive) {
                                newStateMachine.enableSemantics();
                            }
                            this.stateMachines.push(newStateMachine);
                        }
                    }
                }
            }
        }
        // Fire play/paused events for animations
        if (fireEvent) {
            if (playing) {
                this.eventManager.fire({
                    type: EventType.Play,
                    data: this.playing,
                });
            }
            else {
                this.eventManager.fire({
                    type: EventType.Pause,
                    data: this.paused,
                });
            }
        }
        return playing ? this.playing : this.paused;
    };
    /**
     * Adds linear animations by their names.
     * @param animatables the name(s) of animations to add
     * @param playing whether animations should play on instantiation
     */
    Animator.prototype.initLinearAnimations = function (animatables, playing, isFallingBackFromStateMachines) {
        if (isFallingBackFromStateMachines === void 0) { isFallingBackFromStateMachines = false; }
        // Play/pause already instanced items, or create new instances
        // This validation is kept to maintain compatibility with current behavior.
        // But given that it this is called during artboard initialization
        // it should probably be safe to remove.
        var instancedAnimationNames = this.animations.map(function (a) { return a.name; });
        for (var i = 0; i < animatables.length; i++) {
            var aIndex = instancedAnimationNames.indexOf(animatables[i]);
            if (aIndex >= 0) {
                this.animations[aIndex].playing = playing;
            }
            else {
                // Try to create a new animation instance
                var anim = this.artboard.animationByName(animatables[i]);
                if (anim) {
                    var newAnimation = new _animation__WEBPACK_IMPORTED_MODULE_0__.Animation(anim, this.artboard, this.runtime, playing);
                    // Display the first frame of the specified animation
                    newAnimation.advance(0);
                    newAnimation.apply(1.0);
                    this.animations.push(newAnimation);
                }
                else if (isFallingBackFromStateMachines) { // Throw LoadError if we cannot load the state machine name at all
                    var smInitializationMessage = "State Machine with name ".concat(animatables[i], " not found");
                    throw new RiveError(smInitializationMessage);
                }
                else {
                    console.error("Animation with name ".concat(animatables[i], " not found."));
                }
            }
        }
    };
    /**
     * Adds state machines by their names.
     * @param animatables the name(s) of state machines to add
     * @param playing whether state machines should play on instantiation
     */
    Animator.prototype.initStateMachines = function (animatables, playing, semanticsActive) {
        // Play/pause already instanced items, or create new instances
        // This validation is kept to maintain compatibility with current behavior.
        // But given that it this is called during artboard initialization
        // it should probably be safe to remove.
        var instancedStateMachineNames = this.stateMachines.map(function (a) { return a.name; });
        for (var i = 0; i < animatables.length; i++) {
            var aIndex = instancedStateMachineNames.indexOf(animatables[i]);
            if (aIndex >= 0) {
                this.stateMachines[aIndex].playing = playing;
            }
            else {
                // Try to create a new state machine instance
                var sm = this.artboard.stateMachineByName(animatables[i]);
                if (sm) {
                    var newStateMachine = new StateMachine(sm, this.runtime, playing, this.artboard);
                    if (semanticsActive) {
                        newStateMachine.enableSemantics();
                    }
                    this.stateMachines.push(newStateMachine);
                }
                else {
                    console.warn("State Machine with name ".concat(animatables[i], " not found. Falling back to find an animation with the same name."));
                    // TODO: Remove this fallback in next major release as it complicates initialization.
                    // In order to maintain compatibility with current behavior, if a state machine is not found
                    // we look for an animation with the same name
                    this.initLinearAnimations([animatables[i]], playing, true);
                }
            }
        }
    };
    /**
     * Play the named animations/state machines
     * @param animatables the names of the animations/machines to play; plays all if empty
     * @returns a list of the playing items
     */
    Animator.prototype.play = function (animatables) {
        return this.add(animatables, true);
    };
    /**
     * Advance state machines if they are paused after initialization
     */
    Animator.prototype.advanceIfPaused = function () {
        this.stateMachines.forEach(function (sm) {
            if (!sm.playing) {
                sm.advanceAndApply(0);
            }
        });
    };
    /**
     * Pauses named animations and state machines, or everything if nothing is
     * specified
     * @param animatables names of the animations and state machines to pause
     * @returns a list of names of the animations and state machines paused
     */
    Animator.prototype.pause = function (animatables) {
        return this.add(animatables, false);
    };
    /**
     * Set time of named animations
     * @param animations names of the animations to scrub
     * @param value time scrub value, a floating point number to which the playhead is jumped
     * @returns a list of names of the animations that were scrubbed
     */
    Animator.prototype.scrub = function (animatables, value) {
        var forScrubbing = this.animations.filter(function (a) {
            return animatables.includes(a.name);
        });
        forScrubbing.forEach(function (a) { return (a.scrubTo = value); });
        return forScrubbing.map(function (a) { return a.name; });
    };
    Object.defineProperty(Animator.prototype, "playing", {
        /**
         * Returns a list of names of all animations and state machines currently
         * playing
         */
        get: function () {
            return this.animations
                .filter(function (a) { return a.playing; })
                .map(function (a) { return a.name; })
                .concat(this.stateMachines.filter(function (m) { return m.playing; }).map(function (m) { return m.name; }));
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Animator.prototype, "paused", {
        /**
         * Returns a list of names of all animations and state machines currently
         * paused
         */
        get: function () {
            return this.animations
                .filter(function (a) { return !a.playing; })
                .map(function (a) { return a.name; })
                .concat(this.stateMachines.filter(function (m) { return !m.playing; }).map(function (m) { return m.name; }));
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Stops and removes all named animations and state machines
     * @param animatables animations and state machines to remove
     * @returns a list of names of removed items
     */
    Animator.prototype.stop = function (animatables) {
        var _this = this;
        animatables = mapToStringArray(animatables);
        // If nothing's specified, wipe them out, all of them
        var removedNames = [];
        // Stop everything
        if (animatables.length === 0) {
            removedNames = this.animations
                .map(function (a) { return a.name; })
                .concat(this.stateMachines.map(function (m) { return m.name; }));
            // Clean up before emptying the arrays
            this.animations.forEach(function (a) { return a.cleanup(); });
            this.stateMachines.forEach(function (m) { return m.cleanup(); });
            // Empty out the arrays
            this.animations.splice(0, this.animations.length);
            this.stateMachines.splice(0, this.stateMachines.length);
        }
        else {
            // Remove only the named animations/state machines
            var animationsToRemove = this.animations.filter(function (a) {
                return animatables.includes(a.name);
            });
            animationsToRemove.forEach(function (a) {
                a.cleanup();
                _this.animations.splice(_this.animations.indexOf(a), 1);
            });
            var machinesToRemove = this.stateMachines.filter(function (m) {
                return animatables.includes(m.name);
            });
            machinesToRemove.forEach(function (m) {
                m.cleanup();
                _this.stateMachines.splice(_this.stateMachines.indexOf(m), 1);
            });
            removedNames = animationsToRemove
                .map(function (a) { return a.name; })
                .concat(machinesToRemove.map(function (m) { return m.name; }));
        }
        this.eventManager.fire({
            type: EventType.Stop,
            data: removedNames,
        });
        // Return the list of animations removed
        return removedNames;
    };
    Object.defineProperty(Animator.prototype, "isPlaying", {
        /**
         * Returns true if at least one animation is active
         */
        get: function () {
            return (this.animations.reduce(function (acc, curr) { return acc || curr.playing; }, false) ||
                this.stateMachines.reduce(function (acc, curr) { return acc || curr.playing; }, false));
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Animator.prototype, "isPaused", {
        /**
         * Returns true if all animations are paused and there's at least one animation
         */
        get: function () {
            return (!this.isPlaying &&
                (this.animations.length > 0 || this.stateMachines.length > 0));
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Animator.prototype, "isStopped", {
        /**
         * Returns true if there are no playing or paused animations/state machines
         */
        get: function () {
            return this.animations.length === 0 && this.stateMachines.length === 0;
        },
        enumerable: false,
        configurable: true
    });
    /**
     * If there are no animations or state machines, add the first one found
     * @returns the name of the animation or state machine instanced
     */
    Animator.prototype.atLeastOne = function (playing, fireEvent, semanticsActive) {
        if (fireEvent === void 0) { fireEvent = true; }
        if (semanticsActive === void 0) { semanticsActive = false; }
        var instancedName;
        if (this.animations.length === 0 && this.stateMachines.length === 0) {
            if (this.artboard.animationCount() > 0) {
                // Warn only when the v3 default would actually change what plays
                if (this.artboard.stateMachineCount() > 0) {
                    warnOnce(DeprecationKeys.defaultStateMachine, "No `stateMachine` was specified, so the artboard's first linear animation is playing by default. " +
                        "In the next major version, the artboard's state machine will be played by default instead when one exists. " +
                        "Pass the `stateMachine` parameter to adopt that behavior now.");
                }
                // Add the first animation
                this.add([(instancedName = this.artboard.animationByIndex(0).name)], playing, fireEvent);
            }
            else if (this.artboard.stateMachineCount() > 0) {
                // Add the first state machine
                this.add([(instancedName = this.artboard.stateMachineByIndex(0).name)], playing, fireEvent, semanticsActive);
            }
        }
        return instancedName;
    };
    /**
     * Checks if any animations have looped and if so, fire the appropriate event
     */
    Animator.prototype.handleLooping = function () {
        for (var _i = 0, _a = this.animations.filter(function (a) { return a.playing; }); _i < _a.length; _i++) {
            var animation = _a[_i];
            // Emit if the animation looped
            if (animation.loopValue === 0 && animation.loopCount) {
                animation.loopCount = 0;
                // This is a one-shot; if it has ended, delete the instance
                this.stop(animation.name);
            }
            else if (animation.loopValue === 1 && animation.loopCount) {
                this.eventManager.fire({
                    type: EventType.Loop,
                    data: { animation: animation.name, type: LoopType.Loop },
                });
                animation.loopCount = 0;
            }
            // Wasm indicates a loop at each time the animation
            // changes direction, so a full loop/lap occurs every
            // two loop counts
            else if (animation.loopValue === 2 && animation.loopCount > 1) {
                this.eventManager.fire({
                    type: EventType.Loop,
                    data: { animation: animation.name, type: LoopType.PingPong },
                });
                animation.loopCount = 0;
            }
        }
    };
    /**
     * Checks if states have changed in state machines and fires a statechange
     * event
     */
    Animator.prototype.handleStateChanges = function () {
        var statesChanged = [];
        for (var _i = 0, _a = this.stateMachines.filter(function (sm) { return sm.playing; }); _i < _a.length; _i++) {
            var stateMachine = _a[_i];
            statesChanged.push.apply(statesChanged, stateMachine.statesChanged);
        }
        if (statesChanged.length > 0) {
            this.eventManager.fire({
                type: EventType.StateChange,
                data: statesChanged,
            });
        }
    };
    Animator.prototype.handleAdvancing = function (time) {
        this.eventManager.fire({
            type: EventType.Advance,
            data: time,
        });
    };
    return Animator;
}());
// #endregion
// #region events
/**
 * Supported event types triggered in Rive
 */
var EventType;
(function (EventType) {
    EventType["Load"] = "load";
    EventType["LoadError"] = "loaderror";
    EventType["Play"] = "play";
    EventType["Pause"] = "pause";
    EventType["Stop"] = "stop";
    /**
     * @deprecated Loop events are deprecated and will be removed in a future
     * major version: they are only reported for linear animation playback, which
     * is deprecated. Use a state machine to control playback and data binding to
     * react to changes instead. See
     * {@link https://rive.app/docs/editor/data-binding/migration-guide} for how
     * to migrate.
     */
    EventType["Loop"] = "loop";
    EventType["Draw"] = "draw";
    EventType["Advance"] = "advance";
    /**
     * @deprecated Subscribing to state change events at runtime is deprecated
     * and will be removed in a future major version: use data binding (view model
     * property observers) or state machine actions to react to changes from your
     * graphic instead. See
     * {@link https://rive.app/docs/editor/state-machine/states#actions} for more details.
     */
    EventType["StateChange"] = "statechange";
    /**
     * @deprecated Subscribing to Rive Events at runtime is deprecated and will be removed in a future
     * major version: please use data binding instead. See
     * {@link https://rive.app/docs/runtimes/web/rive-events} for how to migrate.
     */
    EventType["RiveEvent"] = "riveevent";
    EventType["AudioStatusChange"] = "audiostatuschange";
})(EventType || (EventType = {}));
var riveEventsDeprecationWarning = "Subscribing to Rive Events at runtime is deprecated and will be removed in a future major version: " +
    "please use data binding instead. See " +
    "https://rive.app/docs/runtimes/web/rive-events for how to migrate.";
var stateMachineInputsDeprecationWarning = "State machine inputs are deprecated and will be removed in a future major version: " +
    "please use data binding properties instead. See " +
    "https://rive.app/docs/editor/data-binding/migration-guide#state-machine-inputs for how to migrate.";
var textRunsDeprecationWarning = "Text run APIs are deprecated and will be removed in a future major version: " +
    "please use data binding instead. See " +
    "https://rive.app/docs/editor/data-binding/migration-guide#updating-text-runs-at-runtime for how to migrate.";
var loopEventsDeprecationWarning = "Loop events are deprecated and will be removed in a future major version: " +
    "they are only reported for linear animation playback, which is deprecated. " +
    "Use a state machine to control playback and data binding to react to changes instead. See " +
    "https://rive.app/docs/editor/data-binding/migration-guide for how to migrate.";
var stateChangeEventsDeprecationWarning = "Subscribing to state change events at runtime is deprecated and will be removed in a future major version: " +
    "use data binding (view model property observers) or state machine actions to react to " +
    "changes from your graphic instead. See " +
    "https://rive.app/docs/editor/state-machine/states#actions for how to migrate.";
/**
 * The deprecations this runtime warns about. Each warning prints its own id, so
 * the value can be copied straight out of the console into
 * {@link Rive.suppressDeprecationWarnings}.
 */
var DeprecationKeys = {
    animationNames: "animation-names",
    animationsParam: "animations-param",
    defaultStateMachine: "default-state-machine",
    legacyConstructors: "legacy-constructors",
    legacyUnsubscribe: "legacy-unsubscribe",
    loopEvents: "loop-events",
    namesArray: "names-array",
    riveEvents: "rive-events",
    scrub: "scrub",
    stateChangeEvents: "state-change-events",
    stateMachineInputs: "state-machine-inputs",
    stateMachinesParam: "state-machines-param",
    textRuns: "text-runs",
};
// Derived from `DeprecationKeys`
var deprecationIds = new Set(Object.keys(DeprecationKeys).map(function (name) { return DeprecationKeys[name]; }));
// Deprecations that a consumer has opted out of. Kept at module scope so
// `warnOnce` can read it without referencing the `Rive` class
var suppressedDeprecations = new Set();
/** @internal Backing store for `Rive.suppressDeprecationWarnings`. */
var setSuppressedDeprecations = function (ids) {
    suppressedDeprecations.clear();
    if (!Array.isArray(ids)) {
        console.warn("[Rive] `suppressDeprecationWarnings` expects an array of deprecation ids, " +
            "received ".concat(typeof ids, ". Nothing was suppressed."));
        return;
    }
    for (var _i = 0, ids_1 = ids; _i < ids_1.length; _i++) {
        var id = ids_1[_i];
        if (deprecationIds.has(id)) {
            suppressedDeprecations.add(id);
        }
    }
};
// Deprecation warnings already emitted; each is logged at most once per page
// session to avoid flooding the console when many instances are created.
var emittedWarnings = new Set();
/**
 * Logs a deprecation warning at most once per page session, unless its id has
 * been passed to `Rive.suppressDeprecationWarnings`.
 */
var warnOnce = function (id, message) {
    if (suppressedDeprecations.has(id)) {
        return;
    }
    var key = "".concat(id, ":").concat(message);
    if (emittedWarnings.has(key)) {
        return;
    }
    emittedWarnings.add(key);
    console.warn("[Rive: ".concat(id, "] ").concat(message, "\n") +
        "To suppress this warning, set Rive.suppressDeprecationWarnings = [\"".concat(id, "\"]"));
};
/**
 * Warns that a playback-control method received an array of names; the next
 * major version restricts these parameters to a single string, since playing
 * multiple animations or state machines at once will not be supported.
 */
var warnIfNamesArray = function (names, methodName) {
    if (Array.isArray(names)) {
        warnOnce(DeprecationKeys.namesArray, "Passing an array of names to `".concat(methodName, "()` is deprecated: in the next major version this parameter will be a single string, and playing multiple animations or state machines at once will not be supported."));
    }
};
/**
 * Warns that a playback-control method received linear animation names;
 * name-based control remains supported for state machines only, so
 * user-specified linear animation names are going away in a future major
 * version.
 */
var warnDeprecatedAnimationNames = function (methodName) {
    warnOnce(DeprecationKeys.animationNames, "Passing linear animation names to `".concat(methodName, "()` is deprecated and will be removed in a future major version: ") +
        "Pass a single state machine name to control playback instead.");
};
/**
 * Looping types: one-shot, loop, and ping-pong
 * @deprecated Loop events are deprecated and will be removed in a future major
 * version: they are only reported for linear animation playback, which is
 * deprecated. Use a state machine to control playback and data binding to
 * react to changes instead.
 */
var LoopType;
(function (LoopType) {
    LoopType["OneShot"] = "oneshot";
    LoopType["Loop"] = "loop";
    LoopType["PingPong"] = "pingpong";
})(LoopType || (LoopType = {}));
// Manages Rive events and listeners
var EventManager = /** @class */ (function () {
    function EventManager(listeners) {
        if (listeners === void 0) { listeners = []; }
        this.listeners = listeners;
    }
    // Gets listeners of specified type
    EventManager.prototype.getListeners = function (type) {
        return this.listeners.filter(function (e) { return e.type === type; });
    };
    // Adds a listener
    EventManager.prototype.add = function (listener) {
        if (!this.listeners.includes(listener)) {
            this.listeners.push(listener);
        }
    };
    /**
     * Removes a listener
     * @param listener the listener with the callback to be removed
     */
    EventManager.prototype.remove = function (listener) {
        // We can't simply look for the listener as it'll be a different instance to
        // one originally subscribed. Find all the listeners of the right type and
        // then check their callbacks which should match.
        for (var i = 0; i < this.listeners.length; i++) {
            var currentListener = this.listeners[i];
            if (currentListener.type === listener.type) {
                if (currentListener.callback === listener.callback) {
                    this.listeners.splice(i, 1);
                    break;
                }
            }
        }
    };
    /**
     * Clears all listeners of specified type, or every listener if no type is
     * specified
     * @param type the type of listeners to clear, or all listeners if not
     * specified
     */
    EventManager.prototype.removeAll = function (type) {
        var _this = this;
        if (!type) {
            this.listeners.splice(0, this.listeners.length);
        }
        else {
            this.listeners
                .filter(function (l) { return l.type === type; })
                .forEach(function (l) { return _this.remove(l); });
        }
    };
    // Fires an event
    EventManager.prototype.fire = function (event) {
        var eventListeners = this.getListeners(event.type);
        eventListeners.forEach(function (listener) { return listener.callback(event); });
    };
    return EventManager;
}());
// Manages a queue of tasks
var TaskQueueManager = /** @class */ (function () {
    function TaskQueueManager(eventManager) {
        this.eventManager = eventManager;
        this.queue = [];
    }
    // Adds a task top the queue
    TaskQueueManager.prototype.add = function (task) {
        this.queue.push(task);
    };
    // Processes all tasks in the queue
    TaskQueueManager.prototype.process = function () {
        while (this.queue.length > 0) {
            var task = this.queue.shift();
            if (task === null || task === void 0 ? void 0 : task.action) {
                task.action();
            }
            if (task === null || task === void 0 ? void 0 : task.event) {
                this.eventManager.fire(task.event);
            }
        }
    };
    return TaskQueueManager;
}());
// #endregion
// #region Audio
var SystemAudioStatus;
(function (SystemAudioStatus) {
    SystemAudioStatus[SystemAudioStatus["AVAILABLE"] = 0] = "AVAILABLE";
    SystemAudioStatus[SystemAudioStatus["UNAVAILABLE"] = 1] = "UNAVAILABLE";
})(SystemAudioStatus || (SystemAudioStatus = {}));
// Class to handle audio context availability and status changes
var AudioManager = /** @class */ (function (_super) {
    __extends(AudioManager, _super);
    function AudioManager() {
        var _this = _super !== null && _super.apply(this, arguments) || this;
        _this._started = false;
        _this._enabled = false;
        _this._status = SystemAudioStatus.UNAVAILABLE;
        return _this;
    }
    AudioManager.prototype.delay = function (time) {
        return __awaiter(this, void 0, void 0, function () {
            return __generator(this, function (_a) {
                return [2 /*return*/, new Promise(function (resolve) { return setTimeout(resolve, time); })];
            });
        });
    };
    AudioManager.prototype.timeout = function () {
        return __awaiter(this, void 0, void 0, function () {
            return __generator(this, function (_a) {
                return [2 /*return*/, new Promise(function (_, reject) { return setTimeout(reject, 50); })];
            });
        });
    };
    // Alerts animations on status changes and removes the listeners to avoid alerting twice.
    AudioManager.prototype.reportToListeners = function () {
        this.fire({ type: EventType.AudioStatusChange });
        this.removeAll();
    };
    /**
     * The audio context has been resolved.
     * Alert any listeners that we can now play audio.
     * Rive will now play audio at the configured volume.
     */
    AudioManager.prototype.enableAudio = function () {
        return __awaiter(this, void 0, void 0, function () {
            return __generator(this, function (_a) {
                if (!this._enabled) {
                    this._enabled = true;
                    this._status = SystemAudioStatus.AVAILABLE;
                    this.reportToListeners();
                }
                return [2 /*return*/];
            });
        });
    };
    /**
     * Check if we are able to play audio.
     *
     * We currently check the audio context, when resume() returns before a timeout we know that the
     * audio context is running and we can enable audio.
     */
    AudioManager.prototype.testAudio = function () {
        return __awaiter(this, void 0, void 0, function () {
            var _a;
            return __generator(this, function (_b) {
                switch (_b.label) {
                    case 0:
                        if (!(this._status === SystemAudioStatus.UNAVAILABLE &&
                            this._audioContext !== null)) return [3 /*break*/, 4];
                        _b.label = 1;
                    case 1:
                        _b.trys.push([1, 3, , 4]);
                        return [4 /*yield*/, Promise.race([this._audioContext.resume(), this.timeout()])];
                    case 2:
                        _b.sent();
                        this.enableAudio();
                        return [3 /*break*/, 4];
                    case 3:
                        _a = _b.sent();
                        return [3 /*break*/, 4];
                    case 4: return [2 /*return*/];
                }
            });
        });
    };
    /**
     * Establish audio for use with rive.
     * We both test if we can use audio intermittently and listen for user interaction.
     * The aim is to enable audio playback as soon as the browser allows this.
     */
    AudioManager.prototype._establishAudio = function () {
        return __awaiter(this, void 0, void 0, function () {
            return __generator(this, function (_a) {
                switch (_a.label) {
                    case 0:
                        if (!!this._started) return [3 /*break*/, 5];
                        this._started = true;
                        if (!(typeof window == "undefined")) return [3 /*break*/, 1];
                        this.enableAudio();
                        return [3 /*break*/, 5];
                    case 1:
                        this._audioContext = new AudioContext();
                        this.listenForUserAction();
                        _a.label = 2;
                    case 2:
                        if (!(this._status === SystemAudioStatus.UNAVAILABLE)) return [3 /*break*/, 5];
                        return [4 /*yield*/, this.testAudio()];
                    case 3:
                        _a.sent();
                        return [4 /*yield*/, this.delay(1000)];
                    case 4:
                        _a.sent();
                        return [3 /*break*/, 2];
                    case 5: return [2 /*return*/];
                }
            });
        });
    };
    AudioManager.prototype.listenForUserAction = function () {
        var _this = this;
        // NOTE: AudioContexts are ready immediately if requested in a ui callback
        // we *could* re request one in this listener.
        var _clickListener = function () { return __awaiter(_this, void 0, void 0, function () {
            return __generator(this, function (_a) {
                // note this has "better" results than calling `await this.testAudio()`
                // as we force audio to be enabled in the current thread, rather than chancing
                // the thread to be passed over for some other async context
                this.enableAudio();
                return [2 /*return*/];
            });
        }); };
        // NOTE: we should test this on mobile/pads
        document.addEventListener("pointerdown", _clickListener, {
            once: true,
        });
    };
    /**
     * Establish the audio context for rive, this lets rive know that we can play audio.
     */
    AudioManager.prototype.establishAudio = function () {
        return __awaiter(this, void 0, void 0, function () {
            return __generator(this, function (_a) {
                this._establishAudio();
                return [2 /*return*/];
            });
        });
    };
    Object.defineProperty(AudioManager.prototype, "systemVolume", {
        get: function () {
            if (this._status === SystemAudioStatus.UNAVAILABLE) {
                // We do an immediate test to avoid depending on the delay of the running test
                this.testAudio();
                return 0;
            }
            return 1;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(AudioManager.prototype, "status", {
        get: function () {
            return this._status;
        },
        enumerable: false,
        configurable: true
    });
    return AudioManager;
}(EventManager));
var audioManager = new AudioManager();
var FakeResizeObserver = /** @class */ (function () {
    function FakeResizeObserver() {
    }
    FakeResizeObserver.prototype.observe = function () { };
    FakeResizeObserver.prototype.unobserve = function () { };
    FakeResizeObserver.prototype.disconnect = function () { };
    return FakeResizeObserver;
}());
var MyResizeObserver = globalThis.ResizeObserver || FakeResizeObserver;
/**
 * This class takes care of any observers that will be attached to an animation.
 * It should be treated as a singleton because observers are much more performant
 * when used for observing multiple elements by a single instance.
 */
var ObjectObservers = /** @class */ (function () {
    function ObjectObservers() {
        var _this = this;
        this._elementsMap = new Map();
        /**
         * Resize observers trigger both when the element changes its size and also when the
         * element is added or removed from the document.
         */
        this._onObservedEntry = function (entry) {
            var observed = _this._elementsMap.get(entry.target);
            if (observed !== null) {
                observed.onResize(entry.target.clientWidth == 0 || entry.target.clientHeight == 0);
            }
            else {
                _this._resizeObserver.unobserve(entry.target);
            }
        };
        this._onObserved = function (entries) {
            entries.forEach(_this._onObservedEntry);
        };
        this._resizeObserver = new MyResizeObserver(this._onObserved);
    }
    // Adds an observable element
    ObjectObservers.prototype.add = function (element, onResize) {
        var observed = {
            onResize: onResize,
            element: element,
        };
        this._elementsMap.set(element, observed);
        this._resizeObserver.observe(element);
        return observed;
    };
    // Removes an observable element
    ObjectObservers.prototype.remove = function (observed) {
        this._resizeObserver.unobserve(observed.element);
        this._elementsMap.delete(observed.element);
    };
    return ObjectObservers;
}());
var observers = new ObjectObservers();
// #endregion
// #region Rive
var nextRiveInstanceId = 0;
/**
 * Resolves which animation/state machine names playback should start with.
 * The `stateMachine` parameter takes priority; the deprecated plural
 * `animations`/`stateMachines` parameters apply only when it
 * is not provided.
 */
var resolveStartingPlayback = function (_a) {
    var stateMachine = _a.stateMachine, animations = _a.animations, stateMachines = _a.stateMachines;
    if (animations !== undefined) {
        warnOnce(DeprecationKeys.animationsParam, "The `animations` parameter is deprecated and will be removed in a future major version: please use the `stateMachine` parameter to play a state machine instead.");
    }
    if (stateMachines !== undefined) {
        warnOnce(DeprecationKeys.stateMachinesParam, "The `stateMachines` parameter is deprecated: please use `stateMachine` with a single state machine name instead.");
    }
    if (stateMachine) {
        return {
            startingAnimationNames: [],
            startingStateMachineNames: [stateMachine],
        };
    }
    return {
        startingAnimationNames: mapToStringArray(animations),
        startingStateMachineNames: mapToStringArray(stateMachines),
    };
};
var RiveFile = /** @class */ (function () {
    function RiveFile(params) {
        // Allow the runtime to automatically load assets hosted in Rive's runtime.
        this.enableRiveAssetCDN = true;
        // When true, emits performance.mark/measure entries during RiveFile load.
        this.enablePerfMarks = false;
        this.referenceCount = 0;
        this.destroyed = false;
        this.selfUnref = false;
        this.bindableArtboards = [];
        // Deferred rendering was requested; the session may still be null if this
        // build has no deferred support.
        this.deferred = false;
        // The session this file imported through, null when immediate. Owned here:
        // it has to outlive the file, so it is deleted after the file is released.
        this.session = null;
        // Attaching is once per session: a detached session can never replay again,
        // so a claimed file re-imports for the next instance implicitly. Mirrors the native
        // latch (WebGL2DeferredSession::everBound, C2DDeferredSession::claim).
        this._sessionClaimed = false;
        // Releasing this file has to be deterministic once a session owns its
        // resources, so the finalizer is disarmed rather than left to GC.
        this.fileFinalizer = null;
        this.boundElsewhereWarned = false;
        this.src = params.src;
        this.buffer = params.buffer;
        this.deferred = !!params.enableGPUCanvas;
        if (params.assetLoader)
            this.assetLoader = params.assetLoader;
        this.enableRiveAssetCDN =
            typeof params.enableRiveAssetCDN == "boolean"
                ? params.enableRiveAssetCDN
                : true;
        this.enablePerfMarks = !!params.enablePerfMarks;
        if (this.enablePerfMarks)
            _runtimeLoader__WEBPACK_IMPORTED_MODULE_1__.RuntimeLoader.enablePerfMarks = true;
        // New event management system
        this.eventManager = new EventManager();
        if (params.onLoad)
            this.on(EventType.Load, params.onLoad);
        if (params.onLoadError)
            this.on(EventType.LoadError, params.onLoadError);
    }
    RiveFile.prototype.releaseFile = function () {
        var _a, _b;
        // Release here rather than leaving it to the finalization registry. The
        // native file frees GL-backed resources when its last reference goes, and
        // callers reach this with the renderer's context current; a finalizer runs
        // with none, so those deletes would silently do nothing.
        (_a = this.fileFinalizer) === null || _a === void 0 ? void 0 : _a.release();
        (_b = this.file) === null || _b === void 0 ? void 0 : _b.unref();
        this.fileFinalizer = null;
        this.file = null;
    };
    RiveFile.prototype.releaseSession = function () {
        var _a;
        // Known limitation: cleanup() on a file an instance is still drawing
        // deletes the session under it. A RiveFile's creator holds no reference of
        // its own, so their cleanup() can take the count to zero.
        (_a = this.session) === null || _a === void 0 ? void 0 : _a.delete();
        this.session = null;
    };
    RiveFile.prototype.releaseBindableArtboards = function () {
        this.bindableArtboards.forEach(function (bindableArtboard) {
            return bindableArtboard.destroy();
        });
    };
    RiveFile.prototype.initData = function () {
        return __awaiter(this, void 0, void 0, function () {
            var _a, error_1, loader, loaderWrapper, _b, error_2, fileFinalizer;
            return __generator(this, function (_c) {
                switch (_c.label) {
                    case 0:
                        if (!(this.src && !this.buffer)) return [3 /*break*/, 4];
                        _c.label = 1;
                    case 1:
                        _c.trys.push([1, 3, , 4]);
                        _a = this;
                        return [4 /*yield*/, loadRiveFile(this.src)];
                    case 2:
                        _a.buffer = _c.sent();
                        return [3 /*break*/, 4];
                    case 3:
                        error_1 = _c.sent();
                        if (error_1 instanceof Error) {
                            throw error_1;
                        }
                        throw new RiveError(RiveFile.fileLoadErrorMessage);
                    case 4:
                        if (this.destroyed) {
                            return [2 /*return*/];
                        }
                        if (this.deferred && this.session === null) {
                            this.session = RiveFile.makeDeferredSession(this.runtime);
                        }
                        if (this.assetLoader) {
                            loaderWrapper = new _utils__WEBPACK_IMPORTED_MODULE_3__.CustomFileAssetLoaderWrapper(this.runtime, this.assetLoader, this.session);
                            loader = loaderWrapper.assetLoader;
                        }
                        // Load the Rive file
                        if (this.enablePerfMarks)
                            performance.mark('rive:file-load:start');
                        _c.label = 5;
                    case 5:
                        _c.trys.push([5, 7, , 8]);
                        _b = this;
                        return [4 /*yield*/, this.runtime.load(new Uint8Array(this.buffer), loader, this.enableRiveAssetCDN, this.session)];
                    case 6:
                        _b.file = _c.sent();
                        return [3 /*break*/, 8];
                    case 7:
                        error_2 = _c.sent();
                        // The finalizer that would reclaim the session is only registered once
                        // the import succeeds, so a failed load has to release it here.
                        this.releaseSession();
                        throw error_2;
                    case 8:
                        if (this.enablePerfMarks) {
                            performance.mark('rive:file-load:end');
                            performance.measure('rive:file-load', 'rive:file-load:start', 'rive:file-load:end');
                        }
                        if (this.destroyed) {
                            this.releaseFile();
                            this.releaseSession();
                            return [2 /*return*/];
                        }
                        if (this.file === null) {
                            // A malformed file resolves to null rather than rejecting. Release the
                            // session now: a finalizer for a file that does not exist would keep it
                            // alive until GC, or for as long as the caller holds the failed RiveFile.
                            this.releaseSession();
                            this.fireLoadError(RiveFile.fileLoadErrorMessage);
                            return [2 /*return*/];
                        }
                        fileFinalizer = new _utils__WEBPACK_IMPORTED_MODULE_3__.FileFinalizer(this.file, this.session);
                        this.fileFinalizer = fileFinalizer;
                        _utils__WEBPACK_IMPORTED_MODULE_3__.finalizationRegistry.register(this, fileFinalizer);
                        this.eventManager.fire({
                            type: EventType.Load,
                            data: this,
                        });
                        return [2 /*return*/];
                }
            });
        });
    };
    RiveFile.prototype.loadRiveFileBytes = function () {
        return __awaiter(this, void 0, void 0, function () {
            var bufferPromise;
            return __generator(this, function (_a) {
                if (this.enablePerfMarks)
                    performance.mark('rive:fetch-riv:start');
                bufferPromise = this.src
                    ? loadRiveFile(this.src)
                    : Promise.resolve(this.buffer);
                if (this.enablePerfMarks && this.src) {
                    bufferPromise.then(function () {
                        performance.mark('rive:fetch-riv:end');
                        performance.measure('rive:fetch-riv', 'rive:fetch-riv:start', 'rive:fetch-riv:end');
                    });
                }
                return [2 /*return*/, bufferPromise];
            });
        });
    };
    RiveFile.prototype.loadRuntime = function () {
        return __awaiter(this, void 0, void 0, function () {
            var runtimePromise;
            return __generator(this, function (_a) {
                if (this.enablePerfMarks)
                    performance.mark('rive:await-wasm:start');
                runtimePromise = _runtimeLoader__WEBPACK_IMPORTED_MODULE_1__.RuntimeLoader.awaitInstance();
                if (this.enablePerfMarks) {
                    runtimePromise.then(function () {
                        performance.mark('rive:await-wasm:end');
                        performance.measure('rive:await-wasm', 'rive:await-wasm:start', 'rive:await-wasm:end');
                    });
                }
                return [2 /*return*/, runtimePromise];
            });
        });
    };
    RiveFile.prototype.init = function () {
        return __awaiter(this, void 0, void 0, function () {
            var _a, bufferResolved, runtimeResolved, error_3;
            return __generator(this, function (_b) {
                switch (_b.label) {
                    case 0:
                        // If no source file url specified, it's a bust
                        if (!this.src && !this.buffer) {
                            this.fireLoadError(RiveFile.missingErrorMessage);
                            return [2 /*return*/];
                        }
                        _b.label = 1;
                    case 1:
                        _b.trys.push([1, 4, , 5]);
                        return [4 /*yield*/, Promise.all([this.loadRiveFileBytes(), this.loadRuntime()])];
                    case 2:
                        _a = _b.sent(), bufferResolved = _a[0], runtimeResolved = _a[1];
                        if (this.destroyed) {
                            return [2 /*return*/];
                        }
                        // .riv file buffer and WASM runtime instance
                        this.buffer = bufferResolved;
                        this.runtime = runtimeResolved;
                        if (this.enablePerfMarks)
                            performance.mark('rive:init-data:start');
                        return [4 /*yield*/, this.initData()];
                    case 3:
                        _b.sent();
                        if (this.enablePerfMarks) {
                            performance.mark('rive:init-data:end');
                            performance.measure('rive:init-data', 'rive:init-data:start', 'rive:init-data:end');
                        }
                        return [3 /*break*/, 5];
                    case 4:
                        error_3 = _b.sent();
                        this.fireLoadError(error_3 instanceof Error ? error_3.message : RiveFile.fileLoadErrorMessage);
                        return [3 /*break*/, 5];
                    case 5: return [2 /*return*/];
                }
            });
        });
    };
    RiveFile.prototype.fireLoadError = function (message) {
        this.eventManager.fire({
            type: EventType.LoadError,
            data: message,
        });
        throw new RiveError(message);
    };
    /**
     * Subscribe to Rive-generated events
     * @param type the type of event to subscribe to
     * @param callback callback to fire when the event occurs
     */
    RiveFile.prototype.on = function (type, callback) {
        this.eventManager.add({
            type: type,
            callback: callback,
        });
    };
    /**
     * Unsubscribes from a Rive-generated event
     * @param type the type of event to unsubscribe from
     * @param callback the callback to unsubscribe
     */
    RiveFile.prototype.off = function (type, callback) {
        this.eventManager.remove({
            type: type,
            callback: callback,
        });
    };
    RiveFile.prototype.cleanup = function () {
        this.referenceCount -= 1;
        if (this.referenceCount <= 0) {
            this.removeAllRiveEventListeners();
            this.releaseFile();
            this.releaseBindableArtboards();
            // Everything imported through the session is gone; the session can go.
            this.releaseSession();
            this.destroyed = true;
        }
    };
    // Deferred is only compiled into some runtime builds, so feature detect it
    // and degrade to an immediate import rather than failing the load.
    RiveFile.makeDeferredSession = function (runtime) {
        var _a, _b;
        var session = (_b = (_a = runtime.makeDeferredSession) === null || _a === void 0 ? void 0 : _a.call(runtime)) !== null && _b !== void 0 ? _b : null;
        if (session === null && !RiveFile.deferredUnsupportedWarned) {
            RiveFile.deferredUnsupportedWarned = true;
            console.warn("Rive: `enableGPUCanvas: true` was ignored because this runtime build has no GPU Canvas support; importing in immediate mode. Use a @rive-app/webgl2 or @rive-app/canvas build with deferred rendering compiled in.");
        }
        return session;
    };
    Object.defineProperty(RiveFile.prototype, "deferredSession", {
        /**
         * @internal The deferred session this file imported through, or null if it
         * was imported in immediate mode.
         */
        get: function () {
            return this.session;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(RiveFile.prototype, "deferredRequested", {
        /**
         * @internal Whether deferred was asked for, even when this build could not
         * honor it and imported immediate.
         */
        get: function () {
            return this.deferred;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(RiveFile.prototype, "sessionClaimed", {
        /**
         * @internal Whether this file's session has ever been attached to a renderer.
         * Attaching is once per session, so a claimed file re-imports for the next
         * instance even after the renderer it was bound to is gone.
         */
        get: function () {
            return this._sessionClaimed;
        },
        enumerable: false,
        configurable: true
    });
    /**
     * @internal Marks this file's session as spent. There is no matching release:
     * a session that has been attached can never replay for another renderer.
     */
    RiveFile.prototype.claimSession = function () {
        this._sessionClaimed = true;
    };
    /**
     * @internal One warning per file however many instances collide on it.
     */
    RiveFile.prototype.warnBoundElsewhereOnce = function () {
        if (this.boundElsewhereWarned) {
            return;
        }
        this.boundElsewhereWarned = true;
        console.warn("Rive: this deferred RiveFile is already bound to another Rive instance's canvas, and a deferred session cannot span canvases. Re-importing the file for this instance (an extra parse of the retained buffer, no extra network request).");
    };
    /**
     * @internal Re-imports this file from its retained buffer into a mode of its
     * own. The copy is owned by whoever asked for it and never joins this file's
     * reference count.
     */
    RiveFile.prototype.reimport = function (deferred) {
        return __awaiter(this, void 0, void 0, function () {
            var copy;
            return __generator(this, function (_a) {
                switch (_a.label) {
                    case 0:
                        copy = new RiveFile({
                            buffer: this.buffer,
                            assetLoader: this.assetLoader,
                            enableRiveAssetCDN: this.enableRiveAssetCDN,
                            enablePerfMarks: this.enablePerfMarks,
                            enableGPUCanvas: deferred,
                        });
                        return [4 /*yield*/, copy.init()];
                    case 1:
                        _a.sent();
                        return [2 /*return*/, copy];
                }
            });
        });
    };
    /**
     * Unsubscribes all Rive listeners from an event type, or everything if no type is
     * given
     * @param type the type of event to unsubscribe from, or all types if
     * undefined
     */
    RiveFile.prototype.removeAllRiveEventListeners = function (type) {
        this.eventManager.removeAll(type);
    };
    RiveFile.prototype.getInstance = function () {
        if (this.file !== null) {
            this.referenceCount += 1;
            return this.file;
        }
    };
    RiveFile.prototype.destroyIfUnused = function () {
        if (this.referenceCount <= 0) {
            this.cleanup();
        }
    };
    RiveFile.prototype.createBindableArtboard = function (nativeBindableArtboard) {
        if (nativeBindableArtboard != null) {
            var bindableArtboard = new BindableArtboard(nativeBindableArtboard);
            (0,_utils__WEBPACK_IMPORTED_MODULE_3__.createFinalization)(bindableArtboard, bindableArtboard.nativeArtboard);
            this.bindableArtboards.push(bindableArtboard);
            return bindableArtboard;
        }
        return null;
    };
    /**
     * @deprecated This function is deprecated. For better stability and memory management
     * use `getBindableArtboard()` instead.
     * @param {string} name - The name of the artboard.
     * @returns {Artboard} The artboard to bind to.
     */
    RiveFile.prototype.getArtboard = function (name) {
        var nativeArtboard = this.file.artboardByName(name);
        if (nativeArtboard != null) {
            return new Artboard(nativeArtboard, this);
        }
    };
    RiveFile.prototype.getBindableArtboard = function (name) {
        var nativeArtboard = this.file.bindableArtboardByName(name);
        return this.createBindableArtboard(nativeArtboard);
    };
    RiveFile.prototype.getDefaultBindableArtboard = function () {
        var nativeArtboard = this.file.bindableArtboardDefault();
        return this.createBindableArtboard(nativeArtboard);
    };
    RiveFile.prototype.internalBindableArtboardFromArtboard = function (artboard) {
        var nativeBindableArtboard = this.file.internalBindableArtboardFromArtboard(artboard);
        return this.createBindableArtboard(nativeBindableArtboard);
    };
    RiveFile.prototype.viewModelByName = function (name) {
        var viewModel = this.file.viewModelByName(name);
        if (viewModel !== null) {
            return new ViewModel(viewModel);
        }
        return null;
    };
    /**
     * @returns the names of the file's global view models, in file order.
     */
    RiveFile.prototype.globalViewModelNames = function () {
        return this.file.globalViewModelNames();
    };
    // Error message for missing source or buffer
    RiveFile.missingErrorMessage = "Rive source file or data buffer required";
    // Error message for file load error
    RiveFile.fileLoadErrorMessage = "The file failed to load";
    // Deferred support is per build, so the warning is per page, not per file.
    RiveFile.deferredUnsupportedWarned = false;
    return RiveFile;
}());

var Rive = /** @class */ (function () {
    function Rive(params) {
        var _this = this;
        var _a, _b, _c, _d;
        // Tracks if a Rive file is loaded
        this.loaded = false;
        // Tracks if a Rive file is destroyed
        this.destroyed = false;
        // Reference of an object that handles any observers for the animation
        this._observed = null;
        /**
         * Tracks if a Rive file is loaded; we need this in addition to loaded as some
         * commands (e.g. contents) can be called as soon as the file is loaded.
         * However, playback commands need to be queued and run in order once initial
         * animations and autoplay has been sorted out. This applies to play, pause,
         * and start.
         */
        this.readyForPlaying = false;
        // Deferred rendering was requested for this instance. The file it renders
        // has the final say, since a file's mode is fixed at import.
        this.deferredRenderer = false;
        // Whether this instance imported the file in `riveFile` (true) or the caller
        // supplied it (false). Only a file we imported may be released without a
        // matching getInstance(): a caller's file is theirs however this init ends.
        this.ownsRiveFile = false;
        // Runtime artboard
        this.artboard = null;
        // place to clear up pointer/touch event listeners
        this.eventCleanup = null;
        // Manages keyboard and DOM-focus interactions for the canvas.
        this._keyboardInteractions = null;
        this.shouldDisableRiveListeners = false;
        this.automaticallyHandleEvents = false;
        this.dispatchPointerExit = true;
        // Allow all pointers to be passed to the runtime
        this.enableMultiTouch = false;
        // Allow the runtime to automatically load assets hosted in Rive's runtime.
        this.enableRiveAssetCDN = true;
        this.semanticsMode = _semantics__WEBPACK_IMPORTED_MODULE_2__.SemanticMode.Disabled;
        this.semanticsOptions = {
            riveCanvasLabel: "Rive animation",
        };
        /** True when this instance may drain semantics and render the overlay. */
        this._semanticsActive = false;
        // Keep a local value of the set volume to update it asynchronously
        this._volume = 1;
        // Keep a local value of the set width to update it asynchronously
        this._artboardWidth = undefined;
        // Keep a local value of the set height to update it asynchronously
        this._artboardHeight = undefined;
        // Keep a local value of the device pixel ratio used in rendering and canvas/artboard resizing
        this._devicePixelRatioUsed = 1;
        // Whether the canvas element's size is 0
        this._hasZeroSize = false;
        // Whether a draw operation needs to be forced
        this._needsRedraw = false;
        // Canvas width and height. Values are lazily updated so they might
        // not be in sync with current canvas size.
        this._currentCanvasWidth = 0;
        this._currentCanvasHeight = 0;
        // Audio event listener
        this._audioEventListener = null;
        // draw method bound to the class
        this._boundDraw = null;
        // Page visibility handler — prevents state machine advancing / rAF from being invoked with large time delta
        // when the browser tab is switched back to after being hidden.
        this._pageVisibilityHandler = null;
        // True only when the page visibility handler itself cancelled an active frame.
        // Set by stopRendering(), cleared by startRendering(). Prevents the
        // visibilitychange handler from restarting a rendering loop the caller intentionally stopped.
        this._explicitlyStoppedRendering = false;
        this._viewModelInstance = null;
        // User-provided global view model instances, keyed by their global view
        // model's name. Globals not present here are still driven by the default
        // instances the runtime seeds; the getter only surfaces instances the user
        // has explicitly set.
        this._globalViewModelInstances = new Map();
        this._dataEnums = null;
        this._tabIndex = null;
        this._prevHasFocus = false;
        this._prevWantsTextInputSession = false;
        // Pointer-down defers the proxy focus: mousedown's default would refocus the canvas and
        // touchstart isn't user activation; pointer-up focuses it.
        this._inPointerDownDrain = false;
        this._focusOptions = {
            allowFocusInterrupt: false,
        };
        // Tracks the semantic tree for the given graphic
        this._semanticTree = null;
        this._accessibilityOverlay = null;
        /**
         * True when an input to the accessibility overlay's artboard→canvas transform
         * (layout fit/alignment/bounds, devicePixelRatio, or layout scale) has changed
         * and the matrix must be recomputed on the next overlay update. Avoids calling
         * computeAlignment every frame when only the semantic tree changed.
         */
        this._overlayTransformDirty = true;
        // Module-level counter for unique instance IDs for semantic overlay containers
        this._instanceId = "".concat(nextRiveInstanceId++);
        this.drawOptimization = DrawOptimizationOptions.DrawOnChanged;
        // When true, emits performance.mark/measure entries for load and render.
        this.enablePerfMarks = false;
        // Durations to generate a frame for the last second. Used for performance profiling.
        this.durations = [];
        this.frameTimes = [];
        this.frameCount = 0;
        this.isTouchScrollEnabled = false;
        this.onCanvasResize = function (hasZeroSize) {
            var toggledDisplay = _this._hasZeroSize !== hasZeroSize;
            _this._hasZeroSize = hasZeroSize;
            if (!hasZeroSize) {
                if (toggledDisplay) {
                    _this.resizeDrawingSurfaceToCanvas();
                }
            }
            else if (!_this._layout.maxX || !_this._layout.maxY) {
                _this.resizeToCanvas();
            }
        };
        this.isEditingHostFocused = function () { var _a, _b; return (_b = (_a = _this._keyboardInteractions) === null || _a === void 0 ? void 0 : _a.isEditingHostFocused()) !== null && _b !== void 0 ? _b : false; };
        // Tracks the current animation frame request
        this.frameRequestId = null;
        /**
         * Used be draw to track when a second of active rendering time has passed.
         * Used for debugging purposes
         */
        this.renderSecondTimer = 0;
        this._boundDraw = this.draw.bind(this);
        if (typeof document !== 'undefined') {
            this._pageVisibilityHandler = this._onPageVisibilityChange.bind(this);
            document.addEventListener('visibilitychange', this._pageVisibilityHandler);
        }
        this.canvas = params.canvas;
        if (params.canvas.constructor === HTMLCanvasElement) {
            this._observed = observers.add(this.canvas, this.onCanvasResize);
        }
        this._currentCanvasWidth = this.canvas.width;
        this._currentCanvasHeight = this.canvas.height;
        this.src = params.src;
        this.buffer = params.buffer;
        this.riveFile = params.riveFile;
        this.layout = (_a = params.layout) !== null && _a !== void 0 ? _a : new Layout();
        this.shouldDisableRiveListeners = !!params.shouldDisableRiveListeners;
        this.isTouchScrollEnabled = !!params.isTouchScrollEnabled;
        if (params.automaticallyHandleEvents) {
            warnOnce(DeprecationKeys.riveEvents, "The `automaticallyHandleEvents` parameter is deprecated. " +
                riveEventsDeprecationWarning);
        }
        this.automaticallyHandleEvents = !!params.automaticallyHandleEvents;
        this.dispatchPointerExit =
            params.dispatchPointerExit === false
                ? params.dispatchPointerExit
                : this.dispatchPointerExit;
        this.enableMultiTouch = !!params.enableMultiTouch;
        this.drawOptimization = (_b = params.drawingOptions) !== null && _b !== void 0 ? _b : this.drawOptimization;
        this.enableRiveAssetCDN =
            params.enableRiveAssetCDN === undefined
                ? true
                : params.enableRiveAssetCDN;
        this.enablePerfMarks = !!params.enablePerfMarks;
        if (this.enablePerfMarks)
            _runtimeLoader__WEBPACK_IMPORTED_MODULE_1__.RuntimeLoader.enablePerfMarks = true;
        this._focusOptions = (_c = params.focusOptions) !== null && _c !== void 0 ? _c : this._focusOptions;
        this._tabIndex = (_d = params.tabIndex) !== null && _d !== void 0 ? _d : null;
        this.deferredRenderer = !!params.enableGPUCanvas;
        // New event management system
        this.eventManager = new EventManager();
        if (params.onLoad)
            this.on(EventType.Load, params.onLoad);
        if (params.onLoadError)
            this.on(EventType.LoadError, params.onLoadError);
        if (params.onPlay)
            this.on(EventType.Play, params.onPlay);
        if (params.onPause)
            this.on(EventType.Pause, params.onPause);
        if (params.onStop)
            this.on(EventType.Stop, params.onStop);
        if (params.onLoop)
            this.on(EventType.Loop, params.onLoop);
        if (params.onStateChange)
            this.on(EventType.StateChange, params.onStateChange);
        if (params.onAdvance)
            this.on(EventType.Advance, params.onAdvance);
        /**
         * @deprecated Use camelCase'd versions instead.
         */
        if (params.onload && !params.onLoad)
            this.on(EventType.Load, params.onload);
        if (params.onloaderror && !params.onLoadError)
            this.on(EventType.LoadError, params.onloaderror);
        if (params.onplay && !params.onPlay)
            this.on(EventType.Play, params.onplay);
        if (params.onpause && !params.onPause)
            this.on(EventType.Pause, params.onpause);
        if (params.onstop && !params.onStop)
            this.on(EventType.Stop, params.onstop);
        if (params.onloop && !params.onLoop)
            this.on(EventType.Loop, params.onloop);
        if (params.onstatechange && !params.onStateChange)
            this.on(EventType.StateChange, params.onstatechange);
        /**
         * Asset loading
         */
        if (params.assetLoader)
            this.assetLoader = params.assetLoader;
        // Hook up the task queue
        this.taskQueue = new TaskQueueManager(this.eventManager);
        this.init({
            src: this.src,
            buffer: this.buffer,
            riveFile: this.riveFile,
            autoplay: params.autoplay,
            autoBind: params.autoBind,
            stateMachine: params.stateMachine,
            animations: params.animations,
            stateMachines: params.stateMachines,
            artboard: params.artboard,
            useOffscreenRenderer: params.useOffscreenRenderer,
            tabIndex: params.tabIndex,
            semanticsMode: params.semanticsMode,
            semanticsOptions: params.semanticsOptions,
            enableGPUCanvas: this.deferredRenderer,
        });
    }
    Object.defineProperty(Rive, "suppressDeprecationWarnings", {
        /**
         * Deprecation warnings to silence, by {@link DeprecationId}. Each warning
         * prints the id needed to silence it, so you can copy it out of the console.
         *
         * ```ts
         * Rive.suppressDeprecationWarnings = ["rive-events", "text-runs"];
         * ```
         *
         * Assigning replaces the whole list.
         * There is no option to silence everything
         */
        get: function () {
            return Object.freeze(Array.from(suppressedDeprecations));
        },
        set: function (ids) {
            setSuppressedDeprecations(ids);
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "viewModelCount", {
        get: function () {
            return this.file.viewModelCount();
        },
        enumerable: false,
        configurable: true
    });
    // Alternative constructor to build a Rive instance from an interface/object
    Rive.new = function (params) {
        warnOnce(DeprecationKeys.legacyConstructors, "This function is deprecated: please use `new Rive({})` instead");
        return new Rive(params);
    };
    /**
     * @experimental Turns on semantics and the accessibility overlay for this
     * instance. Idempotent; safe to call before or after load. Use this to drive
     * a consumer-controlled accessibility toggle when constructed with the
     * default {@link SemanticMode.Disabled}.
     */
    Rive.prototype.enableSemantics = function () {
        this.semanticsMode = _semantics__WEBPACK_IMPORTED_MODULE_2__.SemanticMode.Enabled;
        this.activateSemantics();
    };
    Rive.prototype.activateSemantics = function () {
        if (this._semanticsActive || this.semanticsMode === _semantics__WEBPACK_IMPORTED_MODULE_2__.SemanticMode.Disabled) {
            return;
        }
        this._semanticsActive = true;
        this.syncSemanticsOnStateMachines();
    };
    Rive.prototype.syncSemanticsOnStateMachines = function () {
        if (!this._semanticsActive || !this.animator) {
            return;
        }
        for (var _i = 0, _a = this.animator.stateMachines; _i < _a.length; _i++) {
            var stateMachine = _a[_i];
            stateMachine.enableSemantics();
        }
    };
    /**
     * Tears down the semantic tree and accessibility overlay. The overlay captures the
     * active state machine in its action closures, so it must not outlive the
     * instances it points at (reset/load delete them)
     */
    Rive.prototype.cleanupSemantics = function () {
        this._semanticTree = null;
        if (this._accessibilityOverlay) {
            this._accessibilityOverlay.destroy();
            this._accessibilityOverlay = null;
        }
    };
    // Event handler for when audio context becomes available
    Rive.prototype.onSystemAudioChanged = function () {
        this.volume = this._volume;
    };
    // Initializes the Rive object either from constructor or load()
    Rive.prototype.init = function (_a) {
        var _this = this;
        var _b, _c, _d, _e, _f;
        var src = _a.src, buffer = _a.buffer, riveFile = _a.riveFile, stateMachine = _a.stateMachine, animations = _a.animations, stateMachines = _a.stateMachines, artboard = _a.artboard, _g = _a.autoplay, autoplay = _g === void 0 ? false : _g, _h = _a.useOffscreenRenderer, useOffscreenRenderer = _h === void 0 ? false : _h, _j = _a.autoBind, autoBind = _j === void 0 ? false : _j, tabIndex = _a.tabIndex, semanticsMode = _a.semanticsMode, semanticsOptions = _a.semanticsOptions, enableGPUCanvas = _a.enableGPUCanvas;
        if (this.destroyed) {
            return;
        }
        // Validated before any state changes, so a call with no source leaves a
        // running instance untouched.
        if (!src && !buffer && !riveFile) {
            throw new RiveError(Rive.missingErrorMessage);
        }
        // Reload releases the outgoing file in the same order cleanup() uses:
        // artboard, then the renderer's session, then the file. The artboard goes
        // first because it holds an rcp<File> and its resources came from the
        // session about to be released. stop() has already deleted the animation
        // and state machine instances; reload deliberately skips cleanupInstances(),
        // which would also drop the event listeners load() keeps in place.
        if (this.artboard) {
            // Free GPU-backed resources with their own context current. No-op on
            // the canvas2d build.
            (_c = (_b = this.renderer) === null || _b === void 0 ? void 0 : _b.bindContext) === null || _c === void 0 ? void 0 : _c.call(_b);
            (_d = this.animator) === null || _d === void 0 ? void 0 : _d.stop();
            this.artboard.delete();
            this.artboard = null;
        }
        // Detach before the session is deleted: the handle dies with it.
        if (this.renderer) {
            (_f = (_e = this.renderer).detachSession) === null || _f === void 0 ? void 0 : _f.call(_e);
        }
        this.releaseCurrentRiveFile(/* isTeardown */ false);
        this.src = src;
        this.buffer = buffer;
        this.riveFile = riveFile;
        // initData flips this if it ends up importing the file itself.
        this.ownsRiveFile = false;
        // Sticky across reloads so a constructor-set flag survives; an explicit
        // `false` still turns it off.
        this.deferredRenderer = enableGPUCanvas !== null && enableGPUCanvas !== void 0 ? enableGPUCanvas : this.deferredRenderer;
        this._tabIndex = tabIndex !== null && tabIndex !== void 0 ? tabIndex : null;
        this.semanticsMode = semanticsMode !== null && semanticsMode !== void 0 ? semanticsMode : _semantics__WEBPACK_IMPORTED_MODULE_2__.SemanticMode.Disabled;
        this.semanticsOptions = semanticsOptions !== null && semanticsOptions !== void 0 ? semanticsOptions : this.semanticsOptions;
        // Names of the animations and state machines that should be initialized
        var _k = resolveStartingPlayback({
            stateMachine: stateMachine,
            animations: animations,
            stateMachines: stateMachines,
        }), startingAnimationNames = _k.startingAnimationNames, startingStateMachineNames = _k.startingStateMachineNames;
        // Ensure loaded is marked as false if loading new file
        this.loaded = false;
        this.readyForPlaying = false;
        // Ensure the runtime is loaded
        _runtimeLoader__WEBPACK_IMPORTED_MODULE_1__.RuntimeLoader.awaitInstance()
            .then(function (runtime) {
            if (_this.destroyed) {
                return;
            }
            _this.runtime = runtime;
            _this.removeRiveListeners();
            // load() reinitializes without cleanupInstances(); drop any stale overlay
            // bound to the previous file's state machines (no-op on first construct).
            _this.cleanupSemantics();
            _this.deleteRiveRenderer();
            // Get the canvas where you want to render the animation and create a renderer
            if (_this.enablePerfMarks)
                performance.mark('rive:make-renderer:start');
            try {
                // Always immediate at creation; a deferred file attaches its session
                // to this renderer once the file has loaded.
                _this.renderer = _this.runtime.makeRenderer(_this.canvas, useOffscreenRenderer);
                if (!_this.renderer) {
                    throw new Error("Renderer is null, cannot render Rive on the canvas.");
                }
            }
            catch (e) {
                console.error(e);
                throw new RiveError("Unable to create the renderer, your environment may not support WebGL. Try the @rive-app/canvas runtime as an alternative.");
            }
            if (_this.enablePerfMarks) {
                performance.mark('rive:make-renderer:end');
                performance.measure('rive:make-renderer', 'rive:make-renderer:start', 'rive:make-renderer:end');
            }
            // Initial size adjustment based on devicePixelRatio if no width/height are
            // specified explicitly
            if (!(_this.canvas.width || _this.canvas.height)) {
                _this.resizeDrawingSurfaceToCanvas();
            }
            // Load Rive data from a source uri or a data buffer
            _this.initData(artboard, startingAnimationNames, startingStateMachineNames, autoplay, autoBind)
                .then(function (hasInitialized) {
                if (hasInitialized) {
                    return _this.setupRiveListeners();
                }
            })
                .catch(function (e) {
                // initData already catches RiveErrors for load issues like artboard/state machine initialization
                // failures, so just console error and catch here so we don't double-fire the LoadError event
                console.error(e);
            });
        })
            .catch(function (e) {
            _this.eventManager.fire({ type: EventType.LoadError, data: e.message });
        });
    };
    /**
     * Setup Rive Listeners on the canvas
     * @param riveListenerOptions - Enables TouchEvent events on the canvas. Set to true to allow
     * touch scrolling on the canvas element on touch-enabled devices
     * i.e. { isTouchScrollEnabled: true }
     */
    Rive.prototype.setupRiveListeners = function (riveListenerOptions) {
        var _this = this;
        if (this.eventCleanup) {
            this.eventCleanup();
        }
        this.cleanupKeyboardInteractions();
        if (!this.shouldDisableRiveListeners) {
            var playingStateMachines = this.animator.stateMachines.filter(function (sm) { return sm.playing; });
            var activeStateMachines = playingStateMachines
                .filter(function (sm) { return _this.runtime.hasListeners(sm.instance); })
                .map(function (sm) { return sm.instance; });
            var touchScrollEnabledOption = this.isTouchScrollEnabled;
            var dispatchPointerExit = this.dispatchPointerExit;
            var enableMultiTouch = this.enableMultiTouch;
            if (riveListenerOptions &&
                "isTouchScrollEnabled" in riveListenerOptions) {
                touchScrollEnabledOption = riveListenerOptions.isTouchScrollEnabled;
            }
            this.eventCleanup = (0,_utils__WEBPACK_IMPORTED_MODULE_3__.registerTouchInteractions)({
                canvas: this.canvas,
                artboard: this.artboard,
                stateMachines: activeStateMachines,
                renderer: this.renderer,
                rive: this.runtime,
                fit: this._layout.runtimeFit(this.runtime),
                alignment: this._layout.runtimeAlignment(this.runtime),
                isTouchScrollEnabled: touchScrollEnabledOption,
                dispatchPointerExit: dispatchPointerExit,
                enableMultiTouch: enableMultiTouch,
                layoutScaleFactor: this._layout.layoutScaleFactor,
                advanceAndDrain: this.advanceAndDrainForPointer.bind(this),
                summonKeyboard: this.summonKeyboardForGesture.bind(this),
                isTextProxyFocused: this.isEditingHostFocused,
            });
            this.ensureKeyboardInteractions();
        }
    };
    Object.defineProperty(Rive.prototype, "focusStateMachine", {
        // Assumes a single state machine drives focus.
        get: function () {
            return this.animator.stateMachines.find(function (sm) { return sm.playing && sm.hasFocusNodes; });
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Wire keyboard interactions when a playing state machine has focus nodes.
     * Called at listener setup and lazily each frame so late-bound bindable artboards work.
     */
    Rive.prototype.ensureKeyboardInteractions = function () {
        var _this = this;
        if (this._keyboardInteractions ||
            this.shouldDisableRiveListeners ||
            typeof window === "undefined" ||
            !(this.canvas instanceof HTMLCanvasElement)) {
            return;
        }
        var smWithFocusNodes = this.focusStateMachine;
        if (!smWithFocusNodes) {
            return;
        }
        var currentCanvasTabIndex = this.canvas.tabIndex;
        if (currentCanvasTabIndex === -1 || isNaN(currentCanvasTabIndex)) {
            this.canvas.tabIndex = (this._tabIndex !== null ? this._tabIndex : 0);
        }
        this._keyboardInteractions = new _utils__WEBPACK_IMPORTED_MODULE_3__.KeyboardInteractions({
            canvas: this.canvas,
            stateMachine: smWithFocusNodes.instance,
            hasFocusNodes: true,
            getOverlayElement: function () { var _a, _b; return (_b = (_a = _this._accessibilityOverlay) === null || _a === void 0 ? void 0 : _a.getSemanticOverlayContainer()) !== null && _b !== void 0 ? _b : null; },
        });
    };
    /** Runs inside touchend/mouseup so focusing the proxy raises the mobile keyboard. */
    Rive.prototype.summonKeyboardForGesture = function () {
        if (!this._keyboardInteractions)
            return;
        var activeSm = this.focusStateMachine;
        if (!activeSm)
            return;
        if (this.wantsTextInputSession(activeSm.focusState().expectsKeyboardInput)) {
            this._keyboardInteractions.summonForPointer();
        }
    };
    /** expectsKeyboardInput, and (with semantics on) the focused node is a textField. */
    Rive.prototype.wantsTextInputSession = function (expectsKeyboardInput) {
        var _a;
        if (!expectsKeyboardInput)
            return false;
        var node = (_a = this._semanticTree) === null || _a === void 0 ? void 0 : _a.focusedNode();
        return node === undefined || node.role === _semantics__WEBPACK_IMPORTED_MODULE_2__.SemanticRole.textField;
    };
    Rive.prototype.advanceAndDrainForPointer = function (elapsedTime, options) {
        if (!(options === null || options === void 0 ? void 0 : options.pointerDown)) {
            this.advanceAndReportChanges(elapsedTime);
            return;
        }
        this._inPointerDownDrain = true;
        try {
            this.advanceAndReportChanges(elapsedTime);
        }
        finally {
            this._inPointerDownDrain = false;
        }
    };
    Rive.prototype.cleanupKeyboardInteractions = function () {
        var _a;
        if (this._keyboardInteractions) {
            this._keyboardInteractions.cleanup();
            this._keyboardInteractions = null;
        }
        this._prevWantsTextInputSession = false;
        (_a = this._accessibilityOverlay) === null || _a === void 0 ? void 0 : _a.releaseEditingHost(null, null);
    };
    /**
     * Remove Rive Listeners setup on the canvas
     */
    Rive.prototype.removeRiveListeners = function () {
        if (this.eventCleanup) {
            this.eventCleanup();
            this.eventCleanup = null;
        }
        this.cleanupKeyboardInteractions();
    };
    /**
     * If the instance has audio and the system audio is not ready
     * we hook the instance to the audio manager
     */
    Rive.prototype.initializeAudio = function () {
        var _this = this;
        var _a;
        // Initialize audio if needed
        if (audioManager.status == SystemAudioStatus.UNAVAILABLE) {
            if (this.file.hasAudio ||
                (((_a = this.artboard) === null || _a === void 0 ? void 0 : _a.hasAudio) && this._audioEventListener === null)) {
                this._audioEventListener = {
                    type: EventType.AudioStatusChange,
                    callback: function () { return _this.onSystemAudioChanged(); },
                };
                audioManager.add(this._audioEventListener);
                audioManager.establishAudio();
            }
        }
    };
    Rive.prototype.initArtboardSize = function () {
        if (!this.artboard)
            return;
        // Use preset values if they are not undefined
        this._artboardWidth = this.artboard.width =
            this._artboardWidth || this.artboard.width;
        this._artboardHeight = this.artboard.height =
            this._artboardHeight || this.artboard.height;
    };
    // Initializes runtime with Rive data and preps for playing.
    // Returns true for successful initialization.
    Rive.prototype.initData = function (artboardName, animationNames, stateMachineNames, autoplay, autoBind) {
        return __awaiter(this, void 0, void 0, function () {
            var riveFile, error_4, msg;
            var _a, _b;
            return __generator(this, function (_c) {
                switch (_c.label) {
                    case 0:
                        _c.trys.push([0, 5, , 6]);
                        // A caller-supplied riveFile is theirs to release; one we import here is
                        // ours, which matters if a deferred fallback replaces it below.
                        this.ownsRiveFile = this.riveFile == null;
                        if (!this.ownsRiveFile) return [3 /*break*/, 2];
                        riveFile = new RiveFile({
                            src: this.src,
                            buffer: this.buffer,
                            enableRiveAssetCDN: this.enableRiveAssetCDN,
                            assetLoader: this.assetLoader,
                            enablePerfMarks: this.enablePerfMarks,
                            // The instance owns this file, so its flag is the file's flag.
                            enableGPUCanvas: this.deferredRenderer,
                        });
                        this.riveFile = riveFile;
                        return [4 /*yield*/, riveFile.init()];
                    case 1:
                        _c.sent();
                        if (this.destroyed) {
                            // In the very unlikely scenario where the rive file created by this Rive is shared by
                            // another rive file, we only want to destroy it if this file is the only owner.
                            riveFile.destroyIfUnused();
                            return [2 /*return*/, false];
                        }
                        _c.label = 2;
                    case 2:
                        if (!(this.riveFile.deferredSession !== null || this.deferredRenderer)) return [3 /*break*/, 4];
                        return [4 /*yield*/, this.resolveDeferredRendering()];
                    case 3:
                        _c.sent();
                        if (this.destroyed) {
                            // cleanup() ran during a re-import. Taking a reference now would
                            // resurrect a file this instance is never going to draw. Only ours
                            // to release — a caller-supplied file stays theirs.
                            if (this.ownsRiveFile) {
                                (_a = this.riveFile) === null || _a === void 0 ? void 0 : _a.destroyIfUnused();
                            }
                            return [2 /*return*/, false];
                        }
                        _c.label = 4;
                    case 4:
                        this.file = this.riveFile.getInstance();
                        this.reportDisplayScale();
                        // Initialize and draw frame
                        this.initArtboard(artboardName, animationNames, stateMachineNames, autoplay, autoBind);
                        // Initialize the artboard size
                        this.initArtboardSize();
                        // Check for audio
                        this.initializeAudio();
                        if (this.semanticsMode === _semantics__WEBPACK_IMPORTED_MODULE_2__.SemanticMode.Enabled) {
                            this.activateSemantics();
                        }
                        else if (this._semanticsActive) {
                            this.syncSemanticsOnStateMachines();
                        }
                        // Everything's set up, emit a load event
                        try {
                            this.loaded = true;
                            this.eventManager.fire({
                                type: EventType.Load,
                                data: (_b = this.src) !== null && _b !== void 0 ? _b : "buffer",
                            });
                        }
                        catch (e) {
                            // If any synchronous errors surface from the user-supplied onLoad callback,
                            // this will console.error the error but will not invoke LoadError (onLoadError).
                            // Notably, this will not interfere with Rive rendering
                            console.error(e);
                        }
                        // Only initialize paused state machines after the load event has been fired
                        // to allow users to initialize inputs and view models before the first advance
                        this.animator.advanceIfPaused();
                        // Flag ready for playback commands and clear the task queue; this order
                        // is important or it may infinitely recurse
                        this.readyForPlaying = true;
                        this.taskQueue.process();
                        this.drawFrame();
                        return [2 /*return*/, true];
                    case 5:
                        error_4 = _c.sent();
                        msg = resolveErrorMessage(error_4);
                        this.eventManager.fire({ type: EventType.LoadError, data: msg });
                        return [2 /*return*/, Promise.reject(msg)];
                    case 6: return [2 /*return*/];
                }
            });
        });
    };
    /**
     * Settles which rendering mode this Rive instance runs in. The file dictates: its rendering mode
     * is fixed at import, and an immediate renderer silently drops a deferred
     * file's resources. Every mismatch warns and degrades to something that
     * renders, so users shouldn't have a blank canvas. A fallback self-reimport replaces `this.riveFile`;
     * only a file this instance imported is released when that happens.
     */
    Rive.prototype.resolveDeferredRendering = function () {
        return __awaiter(this, void 0, void 0, function () {
            var file, ownsFile, renderer, session, attachSession, immediate, copy, copySession, fallback;
            var _a;
            return __generator(this, function (_b) {
                switch (_b.label) {
                    case 0:
                        file = this.riveFile;
                        ownsFile = this.ownsRiveFile;
                        renderer = this.renderer;
                        session = file.deferredSession;
                        if (session === null) {
                            // No second warning when the file asked for deferred and the build
                            // could not honor it; the unsupported-build warning already fired, and
                            // telling the user to set a flag they set would mislead.
                            if (this.deferredRenderer && !file.deferredRequested) {
                                console.warn("Rive: `enableGPUCanvas: true` was ignored because this RiveFile was imported without it. The mode is fixed at import: construct the RiveFile with `enableGPUCanvas: true` to opt in.");
                            }
                            return [2 /*return*/];
                        }
                        if (!this.deferredRenderer) {
                            console.warn("Rive: this RiveFile was imported with `enableGPUCanvas: true`, so this instance renders deferred even though `enableGPUCanvas` is false on the instance. An immediate renderer would drop the file's deferred resources and draw nothing.");
                        }
                        attachSession = renderer === null || renderer === void 0 ? void 0 : renderer.attachSession;
                        if (!(typeof attachSession !== "function")) return [3 /*break*/, 2];
                        // Offscreen renderers share one context across canvases, which a session
                        // cannot record for. An immediate copy of the file still renders.
                        console.warn("Rive: this renderer cannot replay a deferred session (`useOffscreenRenderer` is not supported with deferred rendering). Re-importing the file in immediate mode for this instance.");
                        return [4 /*yield*/, file.reimport(false)];
                    case 1:
                        immediate = _b.sent();
                        if (this.destroyed) {
                            // cleanup() ran while we were importing; nothing will use this copy.
                            immediate.destroyIfUnused();
                            return [2 /*return*/];
                        }
                        this.riveFile = immediate;
                        this.ownsRiveFile = true;
                        if (ownsFile) {
                            // Nothing else can reach the deferred import we just walked away from.
                            file.destroyIfUnused();
                        }
                        return [2 /*return*/];
                    case 2:
                        // Never re-offered once claimed, even if that renderer is gone: the
                        // resources the session's stream refers to died with it.
                        if (!file.sessionClaimed && attachSession.call(renderer, session)) {
                            file.claimSession();
                            return [2 /*return*/];
                        }
                        // Bound to another instance. Re-import from the retained buffer into a
                        // session of our own.
                        file.warnBoundElsewhereOnce();
                        return [4 /*yield*/, file.reimport(true)];
                    case 3:
                        copy = _b.sent();
                        if (this.destroyed) {
                            // The renderer was deleted while we imported; attaching to it would hand
                            // embind a deleted object.
                            copy.destroyIfUnused();
                            return [2 /*return*/];
                        }
                        this.riveFile = copy;
                        this.ownsRiveFile = true;
                        copySession = copy.deferredSession;
                        if (copySession !== null && attachSession.call(renderer, copySession)) {
                            copy.claimSession();
                            return [2 /*return*/];
                        }
                        // The copy's session could not take this canvas either (this renderer
                        // already holds one). An immediate re-import still renders; a deferred file
                        // on an immediate renderer would drop its resources and draw nothing.
                        console.warn("Rive: could not attach the re-imported file's deferred session to this canvas; falling back to immediate rendering for this instance.");
                        // If a leftover session is why the attach failed, it would keep routing
                        // the immediate file's draws into its recorder, which drops them.
                        (_a = renderer.detachSession) === null || _a === void 0 ? void 0 : _a.call(renderer);
                        return [4 /*yield*/, file.reimport(false)];
                    case 4:
                        fallback = _b.sent();
                        // The deferred copy was only ever ours, so it always goes.
                        copy.destroyIfUnused();
                        if (this.destroyed) {
                            fallback.destroyIfUnused();
                            return [2 /*return*/];
                        }
                        this.riveFile = fallback;
                        this.ownsRiveFile = true;
                        if (ownsFile) {
                            file.destroyIfUnused();
                        }
                        return [2 /*return*/];
                }
            });
        });
    };
    // Initialize for playback
    Rive.prototype.initArtboard = function (artboardName, animationNames, stateMachineNames, autoplay, autoBind) {
        if (!this.file) {
            return;
        }
        // Fetch the artboard
        var rootArtboard = artboardName
            ? this.file.artboardByName(artboardName)
            : this.file.defaultArtboard();
        // Check we have a working artboard
        if (!rootArtboard) {
            throw new RiveError("Invalid artboard name or no default artboard");
        }
        this.artboard = rootArtboard;
        rootArtboard.volume = this._volume * audioManager.systemVolume;
        // Initialize the animator
        this.animator = new Animator(this.runtime, this.artboard, this.eventManager);
        // Initialize the animations; as loaded hasn't happened yet, we need to
        // suppress firing the play/pause events until the load event has fired. To
        // do this we tell the animator to suppress firing events, and add event
        // firing to the task queue.
        var instanceNames;
        if (animationNames.length > 0 || stateMachineNames.length > 0) {
            instanceNames = animationNames.concat(stateMachineNames);
            this.animator.initLinearAnimations(animationNames, autoplay);
            this.animator.initStateMachines(stateMachineNames, autoplay, this._semanticsActive);
        }
        else {
            instanceNames = [this.animator.atLeastOne(autoplay, false, this._semanticsActive)];
        }
        // Queue up firing the playback events
        this.taskQueue.add({
            event: {
                type: autoplay ? EventType.Play : EventType.Pause,
                data: instanceNames,
            },
        });
        if (autoBind) {
            // Set the main view model instance (if the artboard has one)...
            var viewModel = this.file.defaultArtboardViewModel(rootArtboard);
            if (viewModel !== null) {
                var runtimeInstance = viewModel.defaultInstance();
                if (runtimeInstance !== null) {
                    var viewModelInstance = new ViewModelInstance(runtimeInstance, null);
                    (0,_utils__WEBPACK_IMPORTED_MODULE_3__.createFinalization)(viewModelInstance, viewModelInstance.runtimeInstance);
                    this.setViewModelInstance(viewModelInstance);
                }
            }
            // ...and a default instance for each global view model (no longer
            // auto-created by the runtime), then apply everything in one rebind.
            for (var _i = 0, _a = this.file.globalViewModelNames(); _i < _a.length; _i++) {
                var name_1 = _a[_i];
                var globalViewModel = this.file.viewModelByName(name_1);
                if (globalViewModel !== null) {
                    var instance = new ViewModel(globalViewModel).defaultInstance();
                    if (instance !== null) {
                        this.setGlobalViewModelInstance(name_1, instance);
                    }
                }
            }
            this.bind();
        }
    };
    // Draws the current artboard frame
    Rive.prototype.drawFrame = function () {
        var _a, _b;
        if ((_a = document === null || document === void 0 ? void 0 : document.timeline) === null || _a === void 0 ? void 0 : _a.currentTime) {
            if (this.loaded && this.artboard && !this.frameRequestId) {
                this._boundDraw(document.timeline.currentTime);
                (_b = this.runtime) === null || _b === void 0 ? void 0 : _b.resolveAnimationFrame();
            }
        }
        else {
            this.scheduleRendering();
        }
    };
    Rive.prototype._canvasSizeChanged = function () {
        var changed = false;
        if (this.canvas) {
            if (this.canvas.width !== this._currentCanvasWidth) {
                this._currentCanvasWidth = this.canvas.width;
                changed = true;
            }
            if (this.canvas.height !== this._currentCanvasHeight) {
                this._currentCanvasHeight = this.canvas.height;
                changed = true;
            }
        }
        return changed;
    };
    // Content the artboard's change flag cannot see: ore and GPU canvas commands
    // scripts record straight into the session. Must be read before the renderer
    // clears — clearing marks the stream, so a later read is true every frame.
    Rive.prototype._deferredWorkPending = function () {
        var _a, _b, _c;
        return (_c = (_b = (_a = this.riveFile) === null || _a === void 0 ? void 0 : _a.deferredSession) === null || _b === void 0 ? void 0 : _b.recordedThisFrame()) !== null && _c !== void 0 ? _c : false;
    };
    /** Rive can move focus on its own, so reconcile the text session and DOM focus each frame. */
    Rive.prototype.pollFocusState = function () {
        this.ensureKeyboardInteractions();
        var ki = this._keyboardInteractions;
        var activeSm = ki ? this.focusStateMachine : undefined;
        if (!ki || !activeSm || !(this.canvas instanceof HTMLCanvasElement)) {
            this._prevHasFocus = false;
            this._prevWantsTextInputSession = false;
            return;
        }
        var _a = activeSm.focusState(), hasFocus = _a.hasFocus, expectsKeyboardInput = _a.expectsKeyboardInput;
        this.syncTextInputSession(ki, this.wantsTextInputSession(expectsKeyboardInput));
        this.syncRiveFocus(ki, this.canvas, hasFocus);
    };
    /** Begin/end the text session on wantsSession edges; decorate the proxy while it has DOM focus. */
    Rive.prototype.syncTextInputSession = function (ki, wantsSession) {
        var host = ki.editingHost;
        var overlay = this._accessibilityOverlay;
        if (wantsSession) {
            var began = !this._prevWantsTextInputSession;
            if (began) {
                // In-domain or opted-in only; during pointer-down, pointer-up focuses instead.
                var focusProxy = !this._inPointerDownDrain &&
                    (ki.isInFocusDomain(document.activeElement) ||
                        !!this._focusOptions.allowFocusInterrupt);
                ki.beginTextInputSession(focusProxy);
            }
            // The <input> proxy stands in for the semantic DOM field only while it holds DOM focus.
            if (overlay && this._semanticTree) {
                if (host.hasDomFocus()) {
                    overlay.syncEditingHost(this._semanticTree, host, began || !overlay.hasEditingHost());
                }
                else if (overlay.hasEditingHost()) {
                    overlay.releaseEditingHost(this._semanticTree, host);
                }
            }
        }
        else if (this._prevWantsTextInputSession) {
            // Overlay hand-off first, ahead of endTextInputSession's park-on-canvas fallback.
            overlay === null || overlay === void 0 ? void 0 : overlay.releaseEditingHost(this._semanticTree, host);
            ki.endTextInputSession();
        }
        this._prevWantsTextInputSession = wantsSession;
    };
    /** Mirror Rive's hasFocus into the keyboard session state and, on opt-in, canvas focus. */
    Rive.prototype.syncRiveFocus = function (ki, canvas, hasFocus) {
        if (hasFocus) {
            ki.notifyRiveFocused();
            // Pull focus from outside Rive only with allowFocusInterrupt.
            if (!this._prevHasFocus) {
                if (!ki.isInFocusDomain(document.activeElement) &&
                    this._focusOptions.allowFocusInterrupt) {
                    canvas.focus();
                }
                this._prevHasFocus = true;
            }
        }
        else {
            this._prevHasFocus = false;
            // Rive released focus internally (e.g. a state change): drop the Tab trap so the next
            // Tab leaves the canvas.
            if (ki.focusSessionState === _utils__WEBPACK_IMPORTED_MODULE_3__.FocusSessionState.RiveFocused) {
                ki.setFocusSessionState(_utils__WEBPACK_IMPORTED_MODULE_3__.FocusSessionState.NotFocused);
            }
        }
    };
    /**
     * Handles important sequence of reporting Rive events, advancing the state machine or animation, and invoking various callbacks
     * due to state changes, view model property changes, etc.
     *
     * @param elapsedTime time to advance the state machine by
     */
    Rive.prototype.advanceAndReportChanges = function (elapsedTime) {
        var _a, _b;
        // - Advance non-paused animations by the elapsed number of seconds
        // - Advance any animations that require scrubbing
        // - Advance to the first frame even when autoplay is false
        var activeAnimations = this.animator.animations
            .filter(function (a) { return a.playing || a.needsScrub; })
            // The scrubbed animations must be applied first to prevent weird artifacts
            // if the playing animations conflict with the scrubbed animating attribuates.
            .sort(function (first) { return (first.needsScrub ? -1 : 1); });
        for (var _i = 0, activeAnimations_1 = activeAnimations; _i < activeAnimations_1.length; _i++) {
            var animation = activeAnimations_1[_i];
            animation.advance(elapsedTime);
            if (animation.instance.didLoop) {
                animation.loopCount += 1;
            }
            animation.apply(1.0);
        }
        // - Advance non-paused state machines by the elapsed number of seconds
        // - Advance to the first frame even when autoplay is false
        var activeStateMachines = this.animator.stateMachines.filter(function (a) { return a.playing; });
        // Instrument the first 3 frames so the Performance timeline shows precise
        // per-call latency for advance, draw, and flush without polluting the trace.
        var _perfFrame = this.enablePerfMarks && this.frameCount < 3 ? this.frameCount : -1;
        for (var _c = 0, activeStateMachines_1 = activeStateMachines; _c < activeStateMachines_1.length; _c++) {
            var stateMachine = activeStateMachines_1[_c];
            // Check for events before the current frame's state machine advance
            var numEventsReported = stateMachine.reportedEventCount();
            if (numEventsReported) {
                for (var i = 0; i < numEventsReported; i++) {
                    var event_1 = stateMachine.reportedEventAt(i);
                    if (event_1) {
                        if (event_1.type === RiveEventType.OpenUrl) {
                            this.eventManager.fire({
                                type: EventType.RiveEvent,
                                data: event_1,
                            });
                            // Handle the event side effect if explicitly enabled
                            if (this.automaticallyHandleEvents) {
                                var newAnchorTag = document.createElement("a");
                                var _d = event_1, url = _d.url, target = _d.target;
                                var sanitizedUrl = (0,_utils__WEBPACK_IMPORTED_MODULE_3__.sanitizeUrl)(url);
                                url && newAnchorTag.setAttribute("href", sanitizedUrl);
                                target && newAnchorTag.setAttribute("target", target);
                                if (sanitizedUrl && sanitizedUrl !== _utils__WEBPACK_IMPORTED_MODULE_3__.BLANK_URL) {
                                    newAnchorTag.click();
                                }
                            }
                        }
                        else {
                            this.eventManager.fire({
                                type: EventType.RiveEvent,
                                data: event_1,
                            });
                        }
                    }
                }
            }
            if (_perfFrame >= 0)
                performance.mark("rive:sm-advance:start:f".concat(_perfFrame));
            stateMachine.advanceAndApply(elapsedTime);
            if (_perfFrame >= 0) {
                performance.mark("rive:sm-advance:end:f".concat(_perfFrame));
                performance.measure("rive:sm-advance:f".concat(_perfFrame), "rive:sm-advance:start:f".concat(_perfFrame), "rive:sm-advance:end:f".concat(_perfFrame));
            }
            if (this._semanticsActive) {
                var diff = stateMachine.drainSemanticsDiff();
                if (diff) {
                    if (!this._semanticTree) {
                        this._semanticTree = new _semantics__WEBPACK_IMPORTED_MODULE_2__.SemanticTreeModel();
                    }
                    this._semanticTree.applyDiff(diff);
                }
            }
        }
        // Update the accessibility overlay after all state machines have
        // been advanced and their diffs applied to the tree model.
        if (this._semanticsActive &&
            this._semanticTree &&
            activeStateMachines.length > 0 &&
            this.canvas instanceof HTMLCanvasElement) {
            if (!this._accessibilityOverlay) {
                var mainSm_1 = activeStateMachines[0];
                this._accessibilityOverlay = new _semantics__WEBPACK_IMPORTED_MODULE_2__.AccessibilityOverlay({
                    canvas: this.canvas,
                    instanceId: this._instanceId,
                    semanticsOptions: this.semanticsOptions,
                    allowFocusInterrupt: this._focusOptions.allowFocusInterrupt,
                    isEditingHostFocused: this.isEditingHostFocused,
                    fireAction: function (nodeId, actionType) {
                        mainSm_1.fireSemanticAction(nodeId, actionType);
                    },
                    requestFocus: function (nodeId) {
                        return mainSm_1.focusSemanticNode(nodeId);
                    },
                    clearFocus: function () {
                        return mainSm_1.instance.clearFocus();
                    },
                });
            }
            var overlayChange = (_a = this._accessibilityOverlay) === null || _a === void 0 ? void 0 : _a.needsUpdate(this._semanticTree);
            if (overlayChange || this._overlayTransformDirty) {
                // Only recompute the artboard→canvas transform when something that
                // affects it changed (canvas geometry or a layout/dpr input). When only
                // the semantic tree changed we pass null and reuse the existing CSS
                // transform on the overlay container.
                var forwardMat = null;
                if ((overlayChange === null || overlayChange === void 0 ? void 0 : overlayChange.layoutChanged) || this._overlayTransformDirty) {
                    var fit_1 = this._layout.runtimeFit(this.runtime);
                    var alignment = this._layout.runtimeAlignment(this.runtime);
                    forwardMat = this.runtime.computeAlignment(fit_1, alignment, {
                        minX: this._layout.minX,
                        minY: this._layout.minY,
                        maxX: this._layout.maxX,
                        maxY: this._layout.maxY,
                    }, this.artboard.bounds, this._devicePixelRatioUsed * this._layout.layoutScaleFactor);
                    this._overlayTransformDirty = false;
                }
                this._accessibilityOverlay.update(this._semanticTree, forwardMat, this._devicePixelRatioUsed, this.artboard.bounds, overlayChange);
                forwardMat === null || forwardMat === void 0 ? void 0 : forwardMat.delete();
            }
        }
        // For linear animations that have been applied to the artboard, advance it
        // by the elapsed time.
        if (this.animator.stateMachines.length == 0) {
            this.artboard.advance(elapsedTime);
        }
        // Check for any animations that looped
        this.animator.handleLooping();
        // Check for any state machines that had a state change
        this.animator.handleStateChanges();
        // Report advanced time
        this.animator.handleAdvancing(elapsedTime);
        // Poll focus state to see whether or not to blur or pull up a virtual keyboard for any change to a text input node.
        this.pollFocusState();
        // Handle callbacks for main view model property changes
        (_b = this._viewModelInstance) === null || _b === void 0 ? void 0 : _b.handleCallbacks();
        // Handle callbacks for global view model property changes
        this._globalViewModelInstances.forEach(function (instance) {
            if (instance) {
                instance.handleCallbacks();
            }
        });
    };
    /**
     * Draw rendering loop; renders animation frames at the correct time interval.
     * @param time the time at which to render a frame
     */
    Rive.prototype.draw = function (time, onSecond) {
        // Clear the frameRequestId, as we're now rendering a fresh frame
        this.frameRequestId = null;
        // load() tears the artboard down and re-inits asynchronously; a frame
        // queued before that lands here with nothing to draw.
        if (!this.artboard) {
            return;
        }
        var before = performance.now();
        // Instrument the first 3 frames so the Performance timeline shows precise
        // per-call latency for advance, draw, and flush without polluting the trace.
        var _perfFrame = this.enablePerfMarks && this.frameCount < 3 ? this.frameCount : -1;
        // On the first pass, make sure lastTime has a valid value
        if (!this.lastRenderTime) {
            this.lastRenderTime = time;
        }
        // Handle the onSecond callback
        this.renderSecondTimer += time - this.lastRenderTime;
        if (this.renderSecondTimer > 5000) {
            this.renderSecondTimer = 0;
            onSecond === null || onSecond === void 0 ? void 0 : onSecond();
        }
        // Calculate the elapsed time between frames in seconds
        var elapsedTime = (time - this.lastRenderTime) / 1000;
        this.lastRenderTime = time;
        this.advanceAndReportChanges(elapsedTime);
        var renderer = this.renderer;
        // Do not draw on 0 canvas size
        if (!this._hasZeroSize) {
            // If there was no dirt on this frame, do not clear and draw. The deferred
            // term has to be read here, before `clear()` below: clearing opens the
            // session's recording window and marks its stream, so a read taken after
            // it reports every frame as dirty and nothing is ever skipped.
            if (this.drawOptimization == DrawOptimizationOptions.AlwaysDraw ||
                this.artboard.didChange() ||
                this._deferredWorkPending() ||
                this._needsRedraw ||
                this._canvasSizeChanged()) {
                // Canvas must be wiped to prevent artifacts
                renderer.clear();
                renderer.save();
                // Update the renderer alignment if necessary
                if (_perfFrame >= 0)
                    performance.mark("rive:align-renderer:start:f".concat(_perfFrame));
                this.alignRenderer();
                if (_perfFrame >= 0) {
                    performance.mark("rive:align-renderer:end:f".concat(_perfFrame));
                    performance.measure("rive:align-renderer:f".concat(_perfFrame), "rive:align-renderer:start:f".concat(_perfFrame), "rive:align-renderer:end:f".concat(_perfFrame));
                }
                if (_perfFrame >= 0)
                    performance.mark("rive:artboard-draw:start:f".concat(_perfFrame));
                this.artboard.draw(renderer);
                if (_perfFrame >= 0) {
                    performance.mark("rive:artboard-draw:end:f".concat(_perfFrame));
                    performance.measure("rive:artboard-draw:f".concat(_perfFrame), "rive:artboard-draw:start:f".concat(_perfFrame), "rive:artboard-draw:end:f".concat(_perfFrame));
                }
                renderer.restore();
                if (_perfFrame >= 0)
                    performance.mark("rive:renderer-flush:start:f".concat(_perfFrame));
                renderer.flush();
                if (_perfFrame >= 0) {
                    performance.mark("rive:renderer-flush:end:f".concat(_perfFrame));
                    performance.measure("rive:renderer-flush:f".concat(_perfFrame), "rive:renderer-flush:start:f".concat(_perfFrame), "rive:renderer-flush:end:f".concat(_perfFrame));
                }
                this._needsRedraw = false;
            }
        }
        // Add duration to create frame to durations array
        this.frameCount++;
        var after = performance.now();
        this.frameTimes.push(after);
        this.durations.push(after - before);
        while (this.frameTimes[0] <= after - 1000) {
            this.frameTimes.shift();
            this.durations.shift();
        }
        // Calling requestAnimationFrame will rerun draw() at the correct rate:
        // https://developer.mozilla.org/en-US/docs/Web/API/Canvas_API/Tutorial/Basic_animations
        if (this.animator.isPlaying) {
            // Request a new rendering frame
            this.scheduleRendering();
        }
        else if (this.animator.isPaused) {
            // Reset the end time so on playback it starts at the correct frame
            this.lastRenderTime = 0;
        }
        else if (this.animator.isStopped) {
            // Reset animation instances, artboard and time
            // TODO: implement this properly when we have instancing
            // this.initArtboard();
            // this.drawFrame();
            this.lastRenderTime = 0;
        }
    };
    /**
     * Align the renderer
     */
    Rive.prototype.alignRenderer = function () {
        var _a = this, renderer = _a.renderer, runtime = _a.runtime, _layout = _a._layout, artboard = _a.artboard;
        // Align things up safe in the knowledge we can restore if changed
        renderer.align(_layout.runtimeFit(runtime), _layout.runtimeAlignment(runtime), {
            minX: _layout.minX,
            minY: _layout.minY,
            maxX: _layout.maxX,
            maxY: _layout.maxY,
        }, artboard.bounds, this._devicePixelRatioUsed * _layout.layoutScaleFactor);
    };
    Object.defineProperty(Rive.prototype, "fps", {
        get: function () {
            return this.durations.length;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "frameTime", {
        get: function () {
            if (this.durations.length === 0) {
                return 0;
            }
            return (this.durations.reduce(function (a, b) { return a + b; }, 0) / this.durations.length).toFixed(4);
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Cleans up all Wasm-generated objects that need to be manually destroyed:
     * artboard instances, animation instances, state machine instances,
     * renderer instance, file and runtime.
     *
     * Once this is called, you will need to initialise a new instance of the
     * Rive class
     */
    Rive.prototype.cleanup = function () {
        var _a, _b, _c, _d, _e;
        this.destroyed = true;
        // Stop the renderer if it hasn't already been stopped.
        this.stopRendering();
        // Make the GL context backing this renderer current before any WASM teardown
        // that frees GPU resources. Binding here covers the artboard/file deletes;
        // deleteRiveRenderer() re-binds for the renderer's own delete. No-op on the
        // canvas2d build
        (_b = (_a = this.renderer) === null || _a === void 0 ? void 0 : _a.bindContext) === null || _b === void 0 ? void 0 : _b.call(_a);
        // Clean up any artboard, animation or state machine instances.
        this.cleanupInstances();
        // Remove from observer
        if (this._observed !== null) {
            observers.remove(this._observed);
        }
        this.removeRiveListeners();
        // The file and its session go before the renderer: the session's replay
        // state lives in the renderer's context. But the renderer must let go of
        // the session first — its detach handle dies with session.delete(), and a
        // later detach would pass embind a deleted object.
        if (this.renderer) {
            (_d = (_c = this.renderer).detachSession) === null || _d === void 0 ? void 0 : _d.call(_c);
        }
        this.releaseCurrentRiveFile(/* isTeardown */ true);
        this.riveFile = null;
        this.deleteRiveRenderer();
        if (this._audioEventListener !== null) {
            audioManager.remove(this._audioEventListener);
            this._audioEventListener = null;
        }
        if (this._pageVisibilityHandler) {
            document.removeEventListener('visibilitychange', this._pageVisibilityHandler);
            this._pageVisibilityHandler = null;
        }
        (_e = this._viewModelInstance) === null || _e === void 0 ? void 0 : _e.cleanup();
        this._viewModelInstance = null;
        this._globalViewModelInstances.forEach(function (instance) { return instance.cleanup(); });
        this._globalViewModelInstances.clear();
        this._dataEnums = null;
    };
    /**
     * Drops this instance's hold on `this.riveFile`. A reference taken through
     * getInstance() is given back; a file we imported but never referenced is
     * released outright so its session goes with it. A caller-supplied file we
     * never referenced is left alone.
     *
     * `isTeardown` distinguishes cleanup() from a reload. Teardown always hands
     * the reference back, as it always has. A reload must not do that for a
     * caller-supplied file: a RiveFile carries no reference for its creator, so
     * releasing here would take the last one and destroy a file the caller still
     * holds.
     */
    Rive.prototype.releaseCurrentRiveFile = function (isTeardown) {
        var _a, _b;
        if (this.file) {
            if (isTeardown || this.ownsRiveFile) {
                (_a = this.riveFile) === null || _a === void 0 ? void 0 : _a.cleanup();
            }
            this.file = null;
        }
        else if (this.ownsRiveFile) {
            (_b = this.riveFile) === null || _b === void 0 ? void 0 : _b.destroyIfUnused();
        }
    };
    /**
     * Cleans up the Renderer object. Only call this API if you no longer
     * need to render Rive content in your session.
     */
    Rive.prototype.deleteRiveRenderer = function () {
        var _a, _b;
        if (this.renderer) {
            // A session outliving this renderer has to stop pointing at it, and its
            // replay state has to drop while this context is still alive. No-op when
            // nothing was ever attached.
            (_b = (_a = this.renderer).detachSession) === null || _b === void 0 ? void 0 : _b.call(_a);
            this.renderer.delete();
        }
        this.renderer = null;
    };
    Object.defineProperty(Rive.prototype, "deferredRendererActive", {
        /**
         * @experimental This API is early and may encounter breaking behavior change without a major version bump
         *
         * Whether this instance is rendering through a deferred session. False whenever
         * a fallback ran, whatever was requested.
         */
        get: function () {
            var _a, _b, _c;
            return (_c = (_b = (_a = this.renderer) === null || _a === void 0 ? void 0 : _a.deferredActive) === null || _b === void 0 ? void 0 : _b.call(_a)) !== null && _c !== void 0 ? _c : false;
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Cleans up any Wasm-generated objects that need to be manually destroyed:
     * artboard instances, animation instances, state machine instances.
     *
     * Once this is called, things will need to be reinitialized or bad things
     * might happen.
     */
    Rive.prototype.cleanupInstances = function () {
        var _a;
        if (this.eventCleanup !== null) {
            this.eventCleanup();
        }
        this.cleanupKeyboardInteractions();
        // Tear down semantics before deleting state machines — the overlay's action
        // closures point at instances that stop() is about to free.
        this.cleanupSemantics();
        // Delete all animation and state machine instances synchronously via the
        // animator
        (_a = this.animator) === null || _a === void 0 ? void 0 : _a.stop();
        if (this.artboard) {
            this.artboard.delete();
            this.artboard = null;
        }
    };
    /**
     * Tries to query the setup Artboard for a text run node with the given name.
     *
     * @param textRunName - Name of the text run node associated with a text object
     * @returns - TextValueRun node or undefined if the text run cannot be queried
     */
    Rive.prototype.retrieveTextRun = function (textRunName) {
        var _a;
        if (!textRunName) {
            console.warn("No text run name provided");
            return;
        }
        if (!this.artboard) {
            console.warn("Tried to access text run, but the Artboard is null");
            return;
        }
        var textRun = this.artboard.textRun(textRunName);
        if (!textRun) {
            console.warn("Could not access a text run with name '".concat(textRunName, "' in the '").concat((_a = this.artboard) === null || _a === void 0 ? void 0 : _a.name, "' Artboard. Note that you must rename a text run node in the Rive editor to make it queryable at runtime."));
            return;
        }
        return textRun;
    };
    /**
     * Returns a string from a given text run node name, or undefined if the text run
     * cannot be queried.
     *
     * @deprecated Text run APIs are deprecated: use data binding instead. See
     * {@link https://rive.app/docs/editor/data-binding/migration-guide#updating-text-runs-at-runtime}
     * for how to migrate.
     * @param textRunName - Name of the text run node associated with a text object
     * @returns - String value of the text run node or undefined
     */
    Rive.prototype.getTextRunValue = function (textRunName) {
        warnOnce(DeprecationKeys.textRuns, textRunsDeprecationWarning);
        var textRun = this.retrieveTextRun(textRunName);
        return textRun ? textRun.text : undefined;
    };
    /**
     * Sets a text value for a given text run node name if possible
     *
     * @deprecated Text run APIs are deprecated: use data binding instead. See
     * {@link https://rive.app/docs/editor/data-binding/migration-guide#updating-text-runs-at-runtime}
     * for how to migrate.
     * @param textRunName - Name of the text run node associated with a text object
     * @param textRunValue - String value to set on the text run node
     */
    Rive.prototype.setTextRunValue = function (textRunName, textRunValue) {
        warnOnce(DeprecationKeys.textRuns, textRunsDeprecationWarning);
        var textRun = this.retrieveTextRun(textRunName);
        if (textRun) {
            textRun.text = textRunValue;
        }
    };
    /**
     * Warns when playback-control names match linear animations in the
     * Animator's instanced context; state machine playback remain supported.
     *
     * Remove for v3 release
     */
    Rive.prototype.warnIfLinearAnimationNames = function (names, methodName) {
        if (this.animator &&
            this.animator.animations.some(function (a) { return names.includes(a.name); })) {
            warnDeprecatedAnimationNames(methodName);
        }
    };
    /**
     * Plays specified animations or state machines; if none specified, it
     * unpauses everything.
     * @param animationNames Animation or state machine name(s) to play.
     *
     * Deprecated usage: passing linear animation names (control playback with a
     * state machine instead) and passing an array of names (this parameter
     * becomes a single string in the next major version).
     */
    Rive.prototype.play = function (animationNames, autoplay) {
        var _this = this;
        warnIfNamesArray(animationNames, "play");
        var names = mapToStringArray(animationNames);
        // If the file's not loaded, queue up the play
        if (!this.readyForPlaying) {
            this.taskQueue.add({
                action: function () { return _this.play(animationNames, autoplay); },
            });
            return;
        }
        this.animator.play(names);
        this.warnIfLinearAnimationNames(names, "play");
        this.syncSemanticsOnStateMachines();
        if (this.eventCleanup) {
            this.eventCleanup();
        }
        this.cleanupKeyboardInteractions();
        this.setupRiveListeners();
        this.startRendering();
    };
    /**
     * Pauses specified animations or state machines; if none specified, pauses
     * all.
     * @param animationNames Animation or state machine name(s) to pause.
     *
     * Deprecated usage: passing linear animation names (control playback with a
     * state machine instead) and passing an array of names (this parameter
     * becomes a single string in the next major version).
     */
    Rive.prototype.pause = function (animationNames) {
        var _this = this;
        warnIfNamesArray(animationNames, "pause");
        var names = mapToStringArray(animationNames);
        // If the file's not loaded, early out, nothing to pause
        if (!this.readyForPlaying) {
            this.taskQueue.add({
                action: function () { return _this.pause(animationNames); },
            });
            return;
        }
        if (this.eventCleanup) {
            this.eventCleanup();
        }
        this.cleanupKeyboardInteractions();
        this.animator.pause(names);
        this.warnIfLinearAnimationNames(names, "pause");
    };
    /**
     * Scrubs specified animations to the given time; if none specified, scrubs
     * all of them.
     * @deprecated `scrub()` will be removed in a future major version: use a
     * state machine to control playback instead
     */
    Rive.prototype.scrub = function (animationNames, value) {
        var _this = this;
        warnOnce(DeprecationKeys.scrub, "`scrub()` is deprecated and will be removed in a future major version: " +
            "use a state machine to control playback instead.");
        var names = mapToStringArray(animationNames);
        // If the file's not loaded, early out, nothing to pause
        if (!this.readyForPlaying) {
            this.taskQueue.add({
                action: function () { return _this.scrub(animationNames, value); },
            });
            return;
        }
        // Scrub the animation time; we draw a single frame here so that if
        // nothing's currently playing, the scrubbed animation is still rendered/
        this.animator.scrub(names, value || 0);
        this.drawFrame();
    };
    /**
     * Stops specified animations or state machines; if none specified, stops
     * them all.
     * @param animationNames Animation or state machine name(s) to stop.
     *
     * Deprecated usage: passing linear animation names (control playback with a
     * state machine instead) and passing an array of names (this parameter
     * becomes a single string in the next major version).
     */
    Rive.prototype.stop = function (animationNames) {
        var _this = this;
        warnIfNamesArray(animationNames, "stop");
        var names = mapToStringArray(animationNames);
        // If the file's not loaded, early out, nothing to pause
        if (!this.readyForPlaying) {
            this.taskQueue.add({
                action: function () { return _this.stop(animationNames); },
            });
            return;
        }
        this.warnIfLinearAnimationNames(names, "stop");
        // If there is no artboard, this.animator will be undefined
        if (this.animator) {
            this.animator.stop(names);
        }
        if (this.eventCleanup) {
            this.eventCleanup();
        }
        this.cleanupKeyboardInteractions();
        this.cleanupSemantics();
    };
    /**
     * Resets the animation
     * @param artboard the name of the artboard, or default if none given
     * @param stateMachine the name of the state machine for playback
     * @param autoplay whether to autoplay when reset, defaults to false
     *
     */
    Rive.prototype.reset = function (params) {
        var _a, _b;
        // Get the current artboard, animations, state machines, and playback states
        var artBoardName = params === null || params === void 0 ? void 0 : params.artboard;
        var _c = resolveStartingPlayback({
            stateMachine: params === null || params === void 0 ? void 0 : params.stateMachine,
            animations: params === null || params === void 0 ? void 0 : params.animations,
            stateMachines: params === null || params === void 0 ? void 0 : params.stateMachines,
        }), animationNames = _c.startingAnimationNames, stateMachineNames = _c.startingStateMachineNames;
        var autoplay = (_a = params === null || params === void 0 ? void 0 : params.autoplay) !== null && _a !== void 0 ? _a : false;
        var autoBind = (_b = params === null || params === void 0 ? void 0 : params.autoBind) !== null && _b !== void 0 ? _b : false;
        // Stop everything and clean up
        this.cleanupInstances();
        // Reinitialize an artboard instance with the state
        this.initArtboard(artBoardName, animationNames, stateMachineNames, autoplay, autoBind);
        // Only drain the task queue once playback commands can execute; tasks
        // queued before then re-add themselves in a loop.
        if (this.readyForPlaying) {
            this.taskQueue.process();
        }
    };
    // Loads a new Rive file, keeping listeners in place
    Rive.prototype.load = function (params) {
        // Before stop(), so a call with no source leaves playback running.
        if (!params.src && !params.buffer && !params.riveFile) {
            throw new RiveError(Rive.missingErrorMessage);
        }
        // Stop all animations
        this.stop();
        // Reinitialize
        this.init(params);
    };
    Object.defineProperty(Rive.prototype, "layout", {
        /**
         * Returns the current layout. Note that layout should be treated as
         * immutable. If you want to change the layout, create a new one use the
         * layout setter
         */
        get: function () {
            return this._layout;
        },
        // Sets a new layout
        set: function (layout) {
            this._layout = layout;
            this.reportDisplayScale();
            // Fit/alignment/bounds feed the overlay transform.
            this._overlayTransformDirty = true;
            // If the maxX or maxY are 0, then set them to the canvas width and height
            if (!layout.maxX || !layout.maxY) {
                this.resizeToCanvas();
            }
            if (this.loaded && !this.animator.isPlaying) {
                this.drawFrame();
            }
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Sets the layout bounds to the current canvas size; this is typically called
     * when the canvas is resized
     */
    Rive.prototype.resizeToCanvas = function () {
        this._layout = this.layout.copyWith({
            minX: 0,
            minY: 0,
            maxX: this.canvas.width,
            maxY: this.canvas.height,
        });
        // Layout bounds feed the overlay transform.
        this._overlayTransformDirty = true;
    };
    /**
     * Accounts for devicePixelRatio as a multiplier to render the size of the canvas drawing surface.
     * Uses the size of the backing canvas to set new width/height attributes. Need to re-render
     * and resize the layout to match the new drawing surface afterwards.
     * Useful function for consumers to include in a window resize listener.
     *
     * This method will set the {@link devicePixelRatioUsed} property.
     *
     * Optionally, you can provide a {@link customDevicePixelRatio} to provide a
     * custom value.
     */
    Rive.prototype.resizeDrawingSurfaceToCanvas = function (customDevicePixelRatio) {
        if (this.canvas instanceof HTMLCanvasElement && !!window) {
            var _a = this.canvas.getBoundingClientRect(), width = _a.width, height = _a.height;
            var dpr = customDevicePixelRatio || window.devicePixelRatio || 1;
            this.devicePixelRatioUsed = dpr;
            this.canvas.width = dpr * width;
            this.canvas.height = dpr * height;
            this._needsRedraw = true;
            this.resizeToCanvas();
            if (this.layout.fit === Fit.Layout && this.artboard) {
                var scaleFactor = this._layout.layoutScaleFactor;
                this.artboard.width = width / scaleFactor;
                this.artboard.height = height / scaleFactor;
            }
            this.drawFrame();
        }
    };
    Object.defineProperty(Rive.prototype, "source", {
        // Returns the animation source, which may be undefined
        get: function () {
            return this.src;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "activeArtboard", {
        /**
         * Returns the name of the active artboard
         */
        get: function () {
            return this.artboard ? this.artboard.name : "";
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "semanticTree", {
        /**
         * Returns the semantic tree model when semantics are enabled, or null.
         * The overlay and external consumers use this to inspect the
         * current state of the semantic tree.
         */
        get: function () {
            return this._semanticTree;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "accessibilityOverlay", {
        /**
         * Returns the accessibility overlay when semantics are enabled, or null.
         * External consumers can use this to inspect the
         * current state of the accessibility overlay for this instance.
         */
        get: function () {
            return this._accessibilityOverlay;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "animationNames", {
        // Returns a list of animation names on the chosen artboard
        get: function () {
            // If the file's not loaded, we got nothing to return
            if (!this.loaded || !this.artboard) {
                return [];
            }
            var animationNames = [];
            for (var i = 0; i < this.artboard.animationCount(); i++) {
                animationNames.push(this.artboard.animationByIndex(i).name);
            }
            return animationNames;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "stateMachineNames", {
        /**
         * Returns a list of state machine names from the current artboard
         */
        get: function () {
            // If the file's not loaded, we got nothing to return
            if (!this.loaded || !this.artboard) {
                return [];
            }
            var stateMachineNames = [];
            for (var i = 0; i < this.artboard.stateMachineCount(); i++) {
                stateMachineNames.push(this.artboard.stateMachineByIndex(i).name);
            }
            return stateMachineNames;
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Returns the inputs for the specified instanced state machine, or an empty
     * list if the name is invalid or the state machine is not instanced. Returns
     * undefined if the file is not loaded yet.
     *
     * @deprecated State machine inputs are deprecated: use data binding
     * properties instead. See
     * {@link https://rive.app/docs/editor/data-binding/migration-guide#state-machine-inputs}
     * for how to migrate.
     * @param name the state machine name
     * @returns the inputs for the named state machine or undefined
     */
    Rive.prototype.stateMachineInputs = function (name) {
        warnOnce(DeprecationKeys.stateMachineInputs, stateMachineInputsDeprecationWarning);
        // If the file's not loaded, early out, nothing to pause
        if (!this.loaded) {
            return;
        }
        var stateMachine = this.animator.stateMachines.find(function (m) { return m.name === name; });
        return stateMachine === null || stateMachine === void 0 ? void 0 : stateMachine.inputs;
    };
    // Returns the input with the provided name at the given path
    Rive.prototype.retrieveInputAtPath = function (name, path) {
        if (!name) {
            console.warn("No input name provided for path '".concat(path, "'"));
            return;
        }
        if (!this.artboard) {
            console.warn("Tried to access input: '".concat(name, "', at path: '").concat(path, "', but the Artboard is null"));
            return;
        }
        var input = this.artboard.inputByPath(name, path);
        if (!input) {
            console.warn("Could not access an input with name: '".concat(name, "', at path:'").concat(path, "'"));
            return;
        }
        return input;
    };
    /**
     * Set the boolean input with the provided name at the given path with value
     * @deprecated State machine inputs are deprecated: use data binding
     * properties instead. See
     * {@link https://rive.app/docs/editor/data-binding/migration-guide#state-machine-inputs}
     * for how to migrate.
     * @param input the state machine input name
     * @param value the value to set the input to
     * @param path the path the input is located at an artboard level
     */
    Rive.prototype.setBooleanStateAtPath = function (inputName, value, path) {
        warnOnce(DeprecationKeys.stateMachineInputs, stateMachineInputsDeprecationWarning);
        var input = this.retrieveInputAtPath(inputName, path);
        if (!input)
            return;
        if (input.type === StateMachineInputType.Boolean) {
            input.asBool().value = value;
        }
        else {
            console.warn("Input with name: '".concat(inputName, "', at path:'").concat(path, "' is not a boolean"));
        }
    };
    /**
     * Set the number input with the provided name at the given path with value
     * @deprecated State machine inputs are deprecated: use data binding
     * properties instead. See
     * {@link https://rive.app/docs/editor/data-binding/migration-guide#state-machine-inputs}
     * for how to migrate.
     * @param input the state machine input name
     * @param value the value to set the input to
     * @param path the path the input is located at an artboard level
     */
    Rive.prototype.setNumberStateAtPath = function (inputName, value, path) {
        warnOnce(DeprecationKeys.stateMachineInputs, stateMachineInputsDeprecationWarning);
        var input = this.retrieveInputAtPath(inputName, path);
        if (!input)
            return;
        if (input.type === StateMachineInputType.Number) {
            input.asNumber().value = value;
        }
        else {
            console.warn("Input with name: '".concat(inputName, "', at path:'").concat(path, "' is not a number"));
        }
    };
    /**
     * Fire the trigger with the provided name at the given path
     * @deprecated State machine inputs are deprecated: use data binding
     * properties instead. See
     * {@link https://rive.app/docs/editor/data-binding/migration-guide#state-machine-inputs}
     * for how to migrate.
     * @param input the state machine input name
     * @param path the path the input is located at an artboard level
     */
    Rive.prototype.fireStateAtPath = function (inputName, path) {
        warnOnce(DeprecationKeys.stateMachineInputs, stateMachineInputsDeprecationWarning);
        var input = this.retrieveInputAtPath(inputName, path);
        if (!input)
            return;
        if (input.type === StateMachineInputType.Trigger) {
            input.asTrigger().fire();
        }
        else {
            console.warn("Input with name: '".concat(inputName, "', at path:'").concat(path, "' is not a trigger"));
        }
    };
    // Returns the TextValueRun object for the provided name at the given path
    Rive.prototype.retrieveTextAtPath = function (name, path) {
        if (!name) {
            console.warn("No text name provided for path '".concat(path, "'"));
            return;
        }
        if (!path) {
            console.warn("No path provided for text '".concat(name, "'"));
            return;
        }
        if (!this.artboard) {
            console.warn("Tried to access text: '".concat(name, "', at path: '").concat(path, "', but the Artboard is null"));
            return;
        }
        var text = this.artboard.textByPath(name, path);
        if (!text) {
            console.warn("Could not access text with name: '".concat(name, "', at path:'").concat(path, "'"));
            return;
        }
        return text;
    };
    /**
     * Retrieves the text value for a specified text run at a given path
     * @param textName The name of the text run
     * @param path The path to the text run within the artboard
     * @returns The text value of the text run, or undefined if not found
     *
     * @example
     * // Get the text value for a text run named "title" at one nested artboard deep
     * const titleText = riveInstance.getTextRunValueAtPath("title", "artboard1");
     *
     * @example
     * // Get the text value for a text run named "subtitle" within a nested group two artboards deep
     * const subtitleText = riveInstance.getTextRunValueAtPath("subtitle", "group/nestedGroup");
     *
     * @remarks
     * If the text run cannot be found at the specified path, a warning will be logged to the console.
     *
     * @deprecated Text run APIs are deprecated: use data binding instead. See
     * {@link https://rive.app/docs/editor/data-binding/migration-guide#updating-text-runs-at-runtime}
     * for how to migrate.
     */
    Rive.prototype.getTextRunValueAtPath = function (textName, path) {
        warnOnce(DeprecationKeys.textRuns, textRunsDeprecationWarning);
        var run = this.retrieveTextAtPath(textName, path);
        if (!run) {
            console.warn("Could not get text with name: '".concat(textName, "', at path:'").concat(path, "'"));
            return;
        }
        return run.text;
    };
    /**
     * Sets the text value for a specified text run at a given path
     * @param textName The name of the text run
     * @param value The new text value to set
     * @param path The path to the text run within the artboard
     * @returns void
     *
     * @example
     * // Set the text value for a text run named "title" at one nested artboard deep
     * riveInstance.setTextRunValueAtPath("title", "New Title", "artboard1");
     *
     * @example
     * // Set the text value for a text run named "subtitle" within a nested group two artboards deep
     * riveInstance.setTextRunValueAtPath("subtitle", "New Subtitle", "group/nestedGroup");
     *
     * @remarks
     * If the text run cannot be found at the specified path, a warning will be logged to the console.
     *
     * @deprecated Text run APIs are deprecated: use data binding instead. See
     * {@link https://rive.app/docs/editor/data-binding/migration-guide#updating-text-runs-at-runtime}
     * for how to migrate.
     */
    Rive.prototype.setTextRunValueAtPath = function (textName, value, path) {
        warnOnce(DeprecationKeys.textRuns, textRunsDeprecationWarning);
        var run = this.retrieveTextAtPath(textName, path);
        if (!run) {
            console.warn("Could not set text with name: '".concat(textName, "', at path:'").concat(path, "'"));
            return;
        }
        run.text = value;
    };
    Object.defineProperty(Rive.prototype, "playingStateMachineNames", {
        // Returns a list of playing machine names
        get: function () {
            // If the file's not loaded, we got nothing to return
            if (!this.loaded) {
                return [];
            }
            return this.animator.stateMachines
                .filter(function (m) { return m.playing; })
                .map(function (m) { return m.name; });
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "playingAnimationNames", {
        // Returns a list of playing animation names
        get: function () {
            // If the file's not loaded, we got nothing to return
            if (!this.loaded) {
                return [];
            }
            return this.animator.animations.filter(function (a) { return a.playing; }).map(function (a) { return a.name; });
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "pausedAnimationNames", {
        // Returns a list of paused animation names
        get: function () {
            // If the file's not loaded, we got nothing to return
            if (!this.loaded) {
                return [];
            }
            return this.animator.animations
                .filter(function (a) { return !a.playing; })
                .map(function (a) { return a.name; });
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "pausedStateMachineNames", {
        /**
         *  Returns a list of paused machine names
         * @returns a list of state machine names that are paused
         */
        get: function () {
            // If the file's not loaded, we got nothing to return
            if (!this.loaded) {
                return [];
            }
            return this.animator.stateMachines
                .filter(function (m) { return !m.playing; })
                .map(function (m) { return m.name; });
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "isPlaying", {
        /**
         * @returns true if any animation is playing
         */
        get: function () {
            return this.animator.isPlaying;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "isPaused", {
        /**
         * @returns true if all instanced animations are paused
         */
        get: function () {
            return this.animator.isPaused;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "isStopped", {
        /**
         * @returns true if no animations are playing or paused
         */
        get: function () {
            var _a, _b;
            return (_b = (_a = this.animator) === null || _a === void 0 ? void 0 : _a.isStopped) !== null && _b !== void 0 ? _b : true;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "bounds", {
        /**
         * @returns the bounds of the current artboard, or undefined if the artboard
         * isn't loaded yet.
         */
        get: function () {
            return this.artboard ? this.artboard.bounds : undefined;
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Subscribe to Rive-generated events
     *
     * Note: subscribing to {@link EventType.RiveEvent},
     * {@link EventType.StateChange}, or {@link EventType.Loop} is deprecated;
     * use data binding instead. See
     * {@link https://rive.app/docs/runtimes/web/rive-events} (Rive Events) and
     * {@link https://rive.app/docs/editor/data-binding/migration-guide} for how
     * to migrate Rive graphics to a data binding workflow instead.
     * @param type the type of event to subscribe to
     * @param callback callback to fire when the event occurs
     */
    Rive.prototype.on = function (type, callback) {
        if (type === EventType.RiveEvent) {
            warnOnce(DeprecationKeys.riveEvents, riveEventsDeprecationWarning);
        }
        else if (type === EventType.StateChange) {
            warnOnce(DeprecationKeys.stateChangeEvents, stateChangeEventsDeprecationWarning);
        }
        else if (type === EventType.Loop) {
            warnOnce(DeprecationKeys.loopEvents, loopEventsDeprecationWarning);
        }
        this.eventManager.add({
            type: type,
            callback: callback,
        });
    };
    /**
     * Unsubscribes from a Rive-generated event
     * @param type the type of event to unsubscribe from
     * @param callback the callback to unsubscribe
     */
    Rive.prototype.off = function (type, callback) {
        this.eventManager.remove({
            type: type,
            callback: callback,
        });
    };
    /**
     * Unsubscribes from a Rive-generated event
     * @deprecated
     * @param callback the callback to unsubscribe from
     */
    Rive.prototype.unsubscribe = function (type, callback) {
        warnOnce(DeprecationKeys.legacyUnsubscribe, "This function is deprecated: please use `off()` instead.");
        this.off(type, callback);
    };
    /**
     * Unsubscribes all Rive listeners from an event type, or everything if no type is
     * given
     * @param type the type of event to unsubscribe from, or all types if
     * undefined
     */
    Rive.prototype.removeAllRiveEventListeners = function (type) {
        this.eventManager.removeAll(type);
    };
    /**
     * Unsubscribes all listeners from an event type, or everything if no type is
     * given
     * @deprecated
     * @param type the type of event to unsubscribe from, or all types if
     * undefined
     */
    Rive.prototype.unsubscribeAll = function (type) {
        warnOnce(DeprecationKeys.legacyUnsubscribe, "This function is deprecated: please use `removeAllRiveEventListeners()` instead.");
        this.removeAllRiveEventListeners(type);
    };
    /**
     * Stops the rendering loop; this is different from pausing in that it doesn't
     * change the state of any animation. It stops rendering from occurring. This
     * is designed for situations such as when Rive isn't visible.
     *
     * The only way to start rendering again is to call `startRendering`.
     * Animations that are marked as playing will start from the position that
     * they would have been at if rendering had not been stopped.
     */
    Rive.prototype.stopRendering = function () {
        this._explicitlyStoppedRendering = true;
        if (this.loaded && this.frameRequestId) {
            if (this.runtime.cancelAnimationFrame) {
                this.runtime.cancelAnimationFrame(this.frameRequestId);
            }
            else {
                cancelAnimationFrame(this.frameRequestId);
            }
            this.frameRequestId = null;
        }
    };
    /**
     * Starts the rendering loop if it has been previously stopped. If the
     * renderer is already active, then this will have zero effect.
     */
    Rive.prototype.startRendering = function () {
        this._explicitlyStoppedRendering = false;
        this.drawFrame();
    };
    Rive.prototype.scheduleRendering = function () {
        if (this.loaded && this.artboard && !this.frameRequestId) {
            if (this.runtime.requestAnimationFrame) {
                this.frameRequestId = this.runtime.requestAnimationFrame(this._boundDraw);
            }
            else {
                this.frameRequestId = requestAnimationFrame(this._boundDraw);
            }
        }
    };
    /**
     * Called when document.visibilitychange fires (tab change, window minimize, etc.).
     * Cancels the rAF loop on hide and resets the time reference so that no accumulated time is
     * applied to state machines when the tab becomes visible again. This prevents state machine
     * advances with large time deltas when rAF starts up again.
     */
    Rive.prototype._onPageVisibilityChange = function () {
        var _a, _b;
        if (document.hidden) {
            if (this.frameRequestId !== null) {
                if ((_a = this.runtime) === null || _a === void 0 ? void 0 : _a.cancelAnimationFrame) {
                    this.runtime.cancelAnimationFrame(this.frameRequestId);
                }
                else {
                    cancelAnimationFrame(this.frameRequestId);
                }
                this.frameRequestId = null;
            }
            // Reset so the first resumed frame starts with elapsedTime === 0.
            this.lastRenderTime = 0;
        }
        else if (((_b = this.animator) === null || _b === void 0 ? void 0 : _b.isPlaying) && !this._explicitlyStoppedRendering) {
            this.scheduleRendering();
        }
    };
    /**
     * Enables frames-per-second (FPS) reporting for the runtime
     * If no callback is provided, Rive will append a fixed-position div at the top-right corner of
     * the page with the FPS reading
     * @param fpsCallback - Callback from the runtime during the RAF loop that supplies the FPS value
     */
    Rive.prototype.enableFPSCounter = function (fpsCallback) {
        this.runtime.enableFPSCounter(fpsCallback);
    };
    /**
     * Disables frames-per-second (FPS) reporting for the runtime
     */
    Rive.prototype.disableFPSCounter = function () {
        this.runtime.disableFPSCounter();
    };
    Object.defineProperty(Rive.prototype, "contents", {
        /**
         * Returns the contents of a Rive file: the artboards, animations, and state machines
         */
        get: function () {
            if (!this.loaded) {
                return undefined;
            }
            var riveContents = {
                artboards: [],
            };
            for (var i = 0; i < this.file.artboardCount(); i++) {
                var artboard = this.file.artboardByIndex(i);
                var artboardContents = {
                    name: artboard.name,
                    animations: [],
                    stateMachines: [],
                };
                for (var j = 0; j < artboard.animationCount(); j++) {
                    var animation = artboard.animationByIndex(j);
                    artboardContents.animations.push(animation.name);
                }
                for (var k = 0; k < artboard.stateMachineCount(); k++) {
                    var stateMachine = artboard.stateMachineByIndex(k);
                    var name_2 = stateMachine.name;
                    var instance = new this.runtime.StateMachineInstance(stateMachine, artboard);
                    var inputContents = [];
                    for (var l = 0; l < instance.inputCount(); l++) {
                        var input = instance.input(l);
                        inputContents.push({ name: input.name, type: input.type });
                    }
                    artboardContents.stateMachines.push({
                        name: name_2,
                        inputs: inputContents,
                    });
                }
                riveContents.artboards.push(artboardContents);
            }
            return riveContents;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "volume", {
        /**
         * Getter / Setter for the volume of the artboard
         */
        get: function () {
            if (this.artboard && this.artboard.volume !== this._volume) {
                this._volume = this.artboard.volume;
            }
            return this._volume;
        },
        set: function (value) {
            this._volume = value;
            if (this.artboard) {
                this.artboard.volume = value * audioManager.systemVolume;
            }
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "artboardWidth", {
        /**
         * The width of the artboard.
         *
         * This will return 0 if the artboard is not loaded yet and a custom
         * width has not been set.
         *
         * Do not set this value manually when using {@link resizeDrawingSurfaceToCanvas}
         * with a {@link Layout.fit} of {@link Fit.Layout}, as the artboard width is
         * automatically set.
         */
        get: function () {
            var _a;
            if (this.artboard) {
                return this.artboard.width;
            }
            return (_a = this._artboardWidth) !== null && _a !== void 0 ? _a : 0;
        },
        set: function (value) {
            this._artboardWidth = value;
            if (this.artboard) {
                this.artboard.width = value;
            }
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(Rive.prototype, "artboardHeight", {
        /**
         * The height of the artboard.
         *
         * This will return 0 if the artboard is not loaded yet and a custom
         * height has not been set.
         *
         * Do not set this value manually when using {@link resizeDrawingSurfaceToCanvas}
         * with a {@link Layout.fit} of {@link Fit.Layout}, as the artboard height is
         * automatically set.
         */
        get: function () {
            var _a;
            if (this.artboard) {
                return this.artboard.height;
            }
            return (_a = this._artboardHeight) !== null && _a !== void 0 ? _a : 0;
        },
        set: function (value) {
            this._artboardHeight = value;
            if (this.artboard) {
                this.artboard.height = value;
            }
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Reset the artboard size to its original values.
     */
    Rive.prototype.resetArtboardSize = function () {
        if (this.artboard) {
            this.artboard.resetArtboardSize();
            this._artboardWidth = this.artboard.width;
            this._artboardHeight = this.artboard.height;
        }
        else {
            // If the artboard isn't loaded, we need to reset the custom width and height
            this._artboardWidth = undefined;
            this._artboardHeight = undefined;
        }
    };
    Object.defineProperty(Rive.prototype, "devicePixelRatioUsed", {
        /**
         * The device pixel ratio used in rendering and canvas/artboard resizing.
         *
         * This value will be overidden by the device pixel ratio used in
         * {@link resizeDrawingSurfaceToCanvas}. If you use that method, do not set this value.
         */
        get: function () {
            return this._devicePixelRatioUsed;
        },
        set: function (value) {
            if (value !== this._devicePixelRatioUsed) {
                this._overlayTransformDirty = true;
            }
            this._devicePixelRatioUsed = value;
            this.reportDisplayScale();
        },
        enumerable: false,
        configurable: true
    });
    // Not per frame: the scale is per file, and instances sharing one would trade
    // it every frame.
    Rive.prototype.reportDisplayScale = function () {
        var _a;
        (_a = this.file) === null || _a === void 0 ? void 0 : _a.displayScale(this._devicePixelRatioUsed * this._layout.layoutScaleFactor);
    };
    /**
     * Sets the main view model instance and applies it (rebinds). Equivalent to
     * `setViewModelInstance(vmi)` followed by `bind()`.
     */
    Rive.prototype.bindViewModelInstance = function (viewModelInstance) {
        if (!viewModelInstance) {
            return;
        }
        this.setViewModelInstance(viewModelInstance);
        this.bind();
    };
    /**
     * Sets the main view model instance in the data context WITHOUT rebinding.
     * Call {@link bind} to apply. Use this with {@link setGlobalViewModelInstance}
     * to batch multiple changes into a single rebind.
     */
    Rive.prototype.setViewModelInstance = function (viewModelInstance) {
        var _a;
        var runtimeInstance = viewModelInstance === null || viewModelInstance === void 0 ? void 0 : viewModelInstance.runtimeInstance;
        if (!this.artboard ||
            this.destroyed ||
            !viewModelInstance ||
            !runtimeInstance) {
            return;
        }
        viewModelInstance.internalIncrementReferenceCount();
        (_a = this._viewModelInstance) === null || _a === void 0 ? void 0 : _a.cleanup();
        this._viewModelInstance = viewModelInstance;
        if (this.animator.stateMachines.length > 0) {
            this.animator.stateMachines.forEach(function (stateMachine) {
                return stateMachine.instance.setViewModelInstance(runtimeInstance);
            });
        }
        else {
            this.artboard.setViewModelInstance(runtimeInstance);
        }
    };
    /**
     * Applies any pending `set*` view model instance changes by rebinding the
     * data binds once.
     * Implicitly creates and binds any view models that have not been set.
     */
    Rive.prototype.bind = function () {
        if (!this.artboard || this.destroyed) {
            return;
        }
        if (this.animator.stateMachines.length > 0) {
            this.animator.stateMachines.forEach(function (stateMachine) {
                return stateMachine.instance.bind();
            });
        }
        else {
            this.artboard.bind();
        }
    };
    Object.defineProperty(Rive.prototype, "viewModelInstance", {
        get: function () {
            return this._viewModelInstance;
        },
        enumerable: false,
        configurable: true
    });
    /**
     * Sets (or replaces) the global view model instance for the given global view
     * model name in the data context WITHOUT rebinding. The main instance and any
     * other globals keep their order. Call {@link bind} to apply — batch several
     * `set*` calls then a single `bind()` to avoid rebinding per set.
     * @param name - the name of the global view model
     * @param viewModelInstance - the instance to set for that global
     * @returns whether the instance was set (false if `name` does not match a
     * global view model in the file)
     */
    Rive.prototype.setGlobalViewModelInstance = function (name, viewModelInstance) {
        var _a;
        var runtimeInstance = viewModelInstance === null || viewModelInstance === void 0 ? void 0 : viewModelInstance.runtimeInstance;
        if (!this.artboard || this.destroyed || !runtimeInstance) {
            return false;
        }
        var bound = false;
        if (this.animator.stateMachines.length > 0) {
            this.animator.stateMachines.forEach(function (stateMachine) {
                if (stateMachine.instance.setGlobalViewModelInstance(name, runtimeInstance)) {
                    bound = true;
                }
            });
        }
        else {
            bound = this.artboard.setGlobalViewModelInstance(name, runtimeInstance);
        }
        if (bound) {
            viewModelInstance.internalIncrementReferenceCount();
            (_a = this._globalViewModelInstances.get(name)) === null || _a === void 0 ? void 0 : _a.cleanup();
            this._globalViewModelInstances.set(name, viewModelInstance);
        }
        return bound;
    };
    /**
     * @param name - the name of the global view model
     * @returns the global view model instance bound under the given name — the
     * instance set via {@link setGlobalViewModelInstance} or one created by
     * auto-bind — or null if none has been set/created for that name (globals are
     * not auto-created; the getter never creates one).
     */
    Rive.prototype.globalViewModelInstance = function (name) {
        var cached = this._globalViewModelInstances.get(name);
        if (cached) {
            return cached;
        }
        if (!this.artboard || this.destroyed) {
            return null;
        }
        // State machines share the artboard's data context; query the first one
        // when present (mirroring how the setter routes), otherwise the artboard.
        // This is a pure read — it returns null unless an instance was set/bound.
        var runtimeInstance = this.animator.stateMachines.length > 0
            ? this.animator.stateMachines[0].instance.globalViewModelInstance(name)
            : this.artboard.globalViewModelInstance(name);
        if (runtimeInstance === null) {
            return null;
        }
        var viewModelInstance = new ViewModelInstance(runtimeInstance, null);
        (0,_utils__WEBPACK_IMPORTED_MODULE_3__.createFinalization)(viewModelInstance, runtimeInstance);
        viewModelInstance.internalIncrementReferenceCount();
        this._globalViewModelInstances.set(name, viewModelInstance);
        return viewModelInstance;
    };
    /**
     * @returns the names of the file's global view models, in file order. Use
     * these with {@link setGlobalViewModelInstance} / {@link globalViewModelInstance}.
     */
    Rive.prototype.globalViewModelNames = function () {
        var _a, _b;
        return (_b = (_a = this.file) === null || _a === void 0 ? void 0 : _a.globalViewModelNames()) !== null && _b !== void 0 ? _b : [];
    };
    Rive.prototype.viewModelByIndex = function (index) {
        var viewModel = this.file.viewModelByIndex(index);
        if (viewModel !== null) {
            return new ViewModel(viewModel);
        }
        return null;
    };
    Rive.prototype.viewModelByName = function (name) {
        var _a;
        return (_a = this.riveFile) === null || _a === void 0 ? void 0 : _a.viewModelByName(name);
    };
    Rive.prototype.enums = function () {
        if (this._dataEnums === null) {
            var dataEnums = this.file.enums();
            this._dataEnums = dataEnums.map(function (dataEnum) {
                return new DataEnum(dataEnum);
            });
        }
        return this._dataEnums;
    };
    Rive.prototype.defaultViewModel = function () {
        if (this.artboard) {
            var viewModel = this.file.defaultArtboardViewModel(this.artboard);
            if (viewModel) {
                return new ViewModel(viewModel);
            }
        }
        return null;
    };
    /**
     * @deprecated This function is deprecated. For better stability and memory management
     * use `getBindableArtboard()` instead.
     * @param {string} name - The name of the artboard.
     * @returns {Artboard} The artboard to bind to.
     */
    Rive.prototype.getArtboard = function (name) {
        var _a, _b;
        return (_b = (_a = this.riveFile) === null || _a === void 0 ? void 0 : _a.getArtboard(name)) !== null && _b !== void 0 ? _b : null;
    };
    Rive.prototype.getBindableArtboard = function (name) {
        var _a, _b;
        return (_b = (_a = this.riveFile) === null || _a === void 0 ? void 0 : _a.getBindableArtboard(name)) !== null && _b !== void 0 ? _b : null;
    };
    Rive.prototype.getDefaultBindableArtboard = function () {
        var _a, _b;
        return (_b = (_a = this.riveFile) === null || _a === void 0 ? void 0 : _a.getDefaultBindableArtboard()) !== null && _b !== void 0 ? _b : null;
    };
    /**
     * Clear focus applicable to active state machines with focus nodes. Useful if users want to
     * reset focus state and behavior within the Rive graphic at any point (i.e. blurring off the canvas)
     */
    Rive.prototype.clearFocus = function () {
        var playingStateMachines = this.animator.stateMachines.filter(function (sm) { return sm.playing && sm.hasFocusNodes; });
        playingStateMachines.forEach(function (sm) { return sm.clearFocus(); });
    };
    // Error message for missing source or buffer
    Rive.missingErrorMessage = "Rive source file or data buffer required";
    // Error message for removed rive file
    Rive.cleanupErrorMessage = "Attempt to use file after calling cleanup.";
    return Rive;
}());

var DataType;
(function (DataType) {
    DataType["none"] = "none";
    DataType["string"] = "string";
    DataType["number"] = "number";
    DataType["boolean"] = "boolean";
    DataType["color"] = "color";
    DataType["list"] = "list";
    DataType["enumType"] = "enumType";
    DataType["trigger"] = "trigger";
    DataType["viewModel"] = "viewModel";
    DataType["integer"] = "integer";
    DataType["listIndex"] = "listIndex";
    DataType["image"] = "image";
    DataType["artboard"] = "artboard";
})(DataType || (DataType = {}));
var ViewModel = /** @class */ (function () {
    function ViewModel(viewModel) {
        this._viewModel = viewModel;
    }
    Object.defineProperty(ViewModel.prototype, "instanceCount", {
        get: function () {
            return this._viewModel.instanceCount;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(ViewModel.prototype, "name", {
        get: function () {
            return this._viewModel.name;
        },
        enumerable: false,
        configurable: true
    });
    ViewModel.prototype.instanceByIndex = function (index) {
        var instance = this._viewModel.instanceByIndex(index);
        if (instance !== null) {
            var viewModelInstance = new ViewModelInstance(instance, null);
            (0,_utils__WEBPACK_IMPORTED_MODULE_3__.createFinalization)(viewModelInstance, instance);
            return viewModelInstance;
        }
        return null;
    };
    ViewModel.prototype.instanceByName = function (name) {
        var instance = this._viewModel.instanceByName(name);
        if (instance !== null) {
            var viewModelInstance = new ViewModelInstance(instance, null);
            (0,_utils__WEBPACK_IMPORTED_MODULE_3__.createFinalization)(viewModelInstance, instance);
            return viewModelInstance;
        }
        return null;
    };
    ViewModel.prototype.defaultInstance = function () {
        var runtimeInstance = this._viewModel.defaultInstance();
        if (runtimeInstance !== null) {
            var viewModelInstance = new ViewModelInstance(runtimeInstance, null);
            (0,_utils__WEBPACK_IMPORTED_MODULE_3__.createFinalization)(viewModelInstance, runtimeInstance);
            return viewModelInstance;
        }
        return null;
    };
    ViewModel.prototype.instance = function () {
        var runtimeInstance = this._viewModel.instance();
        if (runtimeInstance !== null) {
            var viewModelInstance = new ViewModelInstance(runtimeInstance, null);
            (0,_utils__WEBPACK_IMPORTED_MODULE_3__.createFinalization)(viewModelInstance, runtimeInstance);
            return viewModelInstance;
        }
        return null;
    };
    Object.defineProperty(ViewModel.prototype, "properties", {
        get: function () {
            return this._viewModel.getProperties();
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(ViewModel.prototype, "instanceNames", {
        get: function () {
            return this._viewModel.getInstanceNames();
        },
        enumerable: false,
        configurable: true
    });
    return ViewModel;
}());

var DataEnum = /** @class */ (function () {
    function DataEnum(dataEnum) {
        this._dataEnum = dataEnum;
    }
    Object.defineProperty(DataEnum.prototype, "name", {
        get: function () {
            return this._dataEnum.name;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(DataEnum.prototype, "values", {
        get: function () {
            return this._dataEnum.values;
        },
        enumerable: false,
        configurable: true
    });
    return DataEnum;
}());

var PropertyType;
(function (PropertyType) {
    PropertyType["Number"] = "number";
    PropertyType["String"] = "string";
    PropertyType["Boolean"] = "boolean";
    PropertyType["Color"] = "color";
    PropertyType["Trigger"] = "trigger";
    PropertyType["Enum"] = "enum";
    PropertyType["List"] = "list";
    PropertyType["Image"] = "image";
    PropertyType["Font"] = "font";
    PropertyType["Artboard"] = "artboard";
})(PropertyType || (PropertyType = {}));
var ViewModelInstance = /** @class */ (function () {
    function ViewModelInstance(runtimeInstance, parent) {
        this._parents = [];
        this._children = [];
        this._viewModelInstances = new Map();
        this._propertiesWithCallbacks = [];
        this._referenceCount = 0;
        this.selfUnref = false;
        this._runtimeInstance = runtimeInstance;
        if (parent !== null) {
            this._parents.push(parent);
        }
    }
    Object.defineProperty(ViewModelInstance.prototype, "runtimeInstance", {
        get: function () {
            return this._runtimeInstance;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(ViewModelInstance.prototype, "nativeInstance", {
        get: function () {
            return this._runtimeInstance;
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstance.prototype.handleCallbacks = function () {
        if (this._propertiesWithCallbacks.length !== 0) {
            this._propertiesWithCallbacks.forEach(function (property) {
                property.handleCallbacks();
            });
            this._propertiesWithCallbacks.forEach(function (property) {
                property.clearChanges();
            });
        }
        this._children.forEach(function (child) { return child.handleCallbacks(); });
    };
    ViewModelInstance.prototype.addParent = function (parent) {
        if (!this._parents.includes(parent)) {
            this._parents.push(parent);
            if (this._propertiesWithCallbacks.length > 0 ||
                this._children.length > 0) {
                parent.addToViewModelCallbacks(this);
            }
        }
    };
    ViewModelInstance.prototype.removeParent = function (parent) {
        var index = this._parents.indexOf(parent);
        if (index !== -1) {
            var parent_1 = this._parents[index];
            parent_1.removeFromViewModelCallbacks(this);
            this._parents.splice(index, 1);
        }
    };
    /*
     * method for internal use, it shouldn't be called externally
     */
    ViewModelInstance.prototype.addToPropertyCallbacks = function (property) {
        var _this = this;
        if (!this._propertiesWithCallbacks.includes(property)) {
            this._propertiesWithCallbacks.push(property);
            if (this._propertiesWithCallbacks.length > 0) {
                this._parents.forEach(function (parent) {
                    parent.addToViewModelCallbacks(_this);
                });
            }
        }
    };
    /*
     * method for internal use, it shouldn't be called externally
     */
    ViewModelInstance.prototype.removeFromPropertyCallbacks = function (property) {
        var _this = this;
        if (this._propertiesWithCallbacks.includes(property)) {
            this._propertiesWithCallbacks = this._propertiesWithCallbacks.filter(function (prop) { return prop !== property; });
            if (this._children.length === 0 &&
                this._propertiesWithCallbacks.length === 0) {
                this._parents.forEach(function (parent) {
                    parent.removeFromViewModelCallbacks(_this);
                });
            }
        }
    };
    /*
     * method for internal use, it shouldn't be called externally
     */
    ViewModelInstance.prototype.addToViewModelCallbacks = function (instance) {
        var _this = this;
        if (!this._children.includes(instance)) {
            this._children.push(instance);
            this._parents.forEach(function (parent) {
                parent.addToViewModelCallbacks(_this);
            });
        }
    };
    /*
     * method for internal use, it shouldn't be called externally
     */
    ViewModelInstance.prototype.removeFromViewModelCallbacks = function (instance) {
        var _this = this;
        if (this._children.includes(instance)) {
            this._children = this._children.filter(function (child) { return child !== instance; });
            if (this._children.length === 0 &&
                this._propertiesWithCallbacks.length === 0) {
                this._parents.forEach(function (parent) {
                    parent.removeFromViewModelCallbacks(_this);
                });
            }
        }
    };
    ViewModelInstance.prototype.clearCallbacks = function () {
        this._propertiesWithCallbacks.forEach(function (property) {
            property.clearCallbacks();
        });
    };
    ViewModelInstance.prototype.propertyFromPath = function (path, type) {
        var pathSegments = path.split("/");
        return this.propertyFromPathSegments(pathSegments, 0, type);
    };
    ViewModelInstance.prototype.viewModelFromPathSegments = function (pathSegments, index) {
        var viewModelInstance = this.internalViewModelInstance(pathSegments[index]);
        if (viewModelInstance !== null) {
            if (index == pathSegments.length - 1) {
                return viewModelInstance;
            }
            else {
                return viewModelInstance.viewModelFromPathSegments(pathSegments, index++);
            }
        }
        return null;
    };
    ViewModelInstance.prototype.propertyFromPathSegments = function (pathSegments, index, type) {
        var _a, _b, _c, _d, _e, _f, _g, _h, _j, _k, _l, _m, _o, _p, _q, _r, _s, _t, _u, _v;
        if (index < pathSegments.length - 1) {
            var viewModelInstance = this.internalViewModelInstance(pathSegments[index]);
            if (viewModelInstance !== null) {
                return viewModelInstance.propertyFromPathSegments(pathSegments, index + 1, type);
            }
            else {
                return null;
            }
        }
        var instance = null;
        switch (type) {
            case PropertyType.Number:
                instance = (_b = (_a = this._runtimeInstance) === null || _a === void 0 ? void 0 : _a.number(pathSegments[index])) !== null && _b !== void 0 ? _b : null;
                if (instance !== null) {
                    return new ViewModelInstanceNumber(instance, this);
                }
                break;
            case PropertyType.String:
                instance = (_d = (_c = this._runtimeInstance) === null || _c === void 0 ? void 0 : _c.string(pathSegments[index])) !== null && _d !== void 0 ? _d : null;
                if (instance !== null) {
                    return new ViewModelInstanceString(instance, this);
                }
                break;
            case PropertyType.Boolean:
                instance = (_f = (_e = this._runtimeInstance) === null || _e === void 0 ? void 0 : _e.boolean(pathSegments[index])) !== null && _f !== void 0 ? _f : null;
                if (instance !== null) {
                    return new ViewModelInstanceBoolean(instance, this);
                }
                break;
            case PropertyType.Color:
                instance = (_h = (_g = this._runtimeInstance) === null || _g === void 0 ? void 0 : _g.color(pathSegments[index])) !== null && _h !== void 0 ? _h : null;
                if (instance !== null) {
                    return new ViewModelInstanceColor(instance, this);
                }
                break;
            case PropertyType.Trigger:
                instance = (_k = (_j = this._runtimeInstance) === null || _j === void 0 ? void 0 : _j.trigger(pathSegments[index])) !== null && _k !== void 0 ? _k : null;
                if (instance !== null) {
                    return new ViewModelInstanceTrigger(instance, this);
                }
                break;
            case PropertyType.Enum:
                instance = (_m = (_l = this._runtimeInstance) === null || _l === void 0 ? void 0 : _l.enum(pathSegments[index])) !== null && _m !== void 0 ? _m : null;
                if (instance !== null) {
                    return new ViewModelInstanceEnum(instance, this);
                }
                break;
            case PropertyType.List:
                instance = (_p = (_o = this._runtimeInstance) === null || _o === void 0 ? void 0 : _o.list(pathSegments[index])) !== null && _p !== void 0 ? _p : null;
                if (instance !== null) {
                    return new ViewModelInstanceList(instance, this);
                }
                break;
            case PropertyType.Image:
                instance = (_r = (_q = this._runtimeInstance) === null || _q === void 0 ? void 0 : _q.image(pathSegments[index])) !== null && _r !== void 0 ? _r : null;
                if (instance !== null) {
                    return new ViewModelInstanceAssetImage(instance, this);
                }
                break;
            case PropertyType.Font:
                instance = (_t = (_s = this._runtimeInstance) === null || _s === void 0 ? void 0 : _s.font(pathSegments[index])) !== null && _t !== void 0 ? _t : null;
                if (instance !== null) {
                    return new ViewModelInstanceAssetFont(instance, this);
                }
                break;
            case PropertyType.Artboard:
                instance = (_v = (_u = this._runtimeInstance) === null || _u === void 0 ? void 0 : _u.artboard(pathSegments[index])) !== null && _v !== void 0 ? _v : null;
                if (instance !== null) {
                    return new ViewModelInstanceArtboard(instance, this);
                }
                break;
        }
        return null;
    };
    ViewModelInstance.prototype.internalViewModelInstance = function (name) {
        var _a;
        if (this._viewModelInstances.has(name)) {
            return this._viewModelInstances.get(name);
        }
        var viewModelRuntimeInstance = (_a = this._runtimeInstance) === null || _a === void 0 ? void 0 : _a.viewModel(name);
        if (viewModelRuntimeInstance !== null) {
            var viewModelInstance = new ViewModelInstance(viewModelRuntimeInstance, this);
            (0,_utils__WEBPACK_IMPORTED_MODULE_3__.createFinalization)(viewModelInstance, viewModelRuntimeInstance);
            viewModelInstance.internalIncrementReferenceCount();
            this._viewModelInstances.set(name, viewModelInstance);
            return viewModelInstance;
        }
        return null;
    };
    /**
     * method to access a property instance of type number belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the number property
     */
    ViewModelInstance.prototype.number = function (path) {
        var viewmodelInstanceValue = this.propertyFromPath(path, PropertyType.Number);
        return viewmodelInstanceValue;
    };
    /**
     * method to access a property instance of type string belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the string property
     */
    ViewModelInstance.prototype.string = function (path) {
        var viewmodelInstanceValue = this.propertyFromPath(path, PropertyType.String);
        return viewmodelInstanceValue;
    };
    /**
     * method to access a property instance of type boolean belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the boolean property
     */
    ViewModelInstance.prototype.boolean = function (path) {
        var viewmodelInstanceValue = this.propertyFromPath(path, PropertyType.Boolean);
        return viewmodelInstanceValue;
    };
    /**
     * method to access a property instance of type color belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the ttrigger property
     */
    ViewModelInstance.prototype.color = function (path) {
        var viewmodelInstanceValue = this.propertyFromPath(path, PropertyType.Color);
        return viewmodelInstanceValue;
    };
    /**
     * method to access a property instance of type trigger belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the trigger property
     */
    ViewModelInstance.prototype.trigger = function (path) {
        var viewmodelInstanceValue = this.propertyFromPath(path, PropertyType.Trigger);
        return viewmodelInstanceValue;
    };
    /**
     * method to access a property instance of type enum belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the enum property
     */
    ViewModelInstance.prototype.enum = function (path) {
        var viewmodelInstanceValue = this.propertyFromPath(path, PropertyType.Enum);
        return viewmodelInstanceValue;
    };
    /**
     * method to access a property instance of type list belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the list property
     */
    ViewModelInstance.prototype.list = function (path) {
        var viewmodelInstanceValue = this.propertyFromPath(path, PropertyType.List);
        return viewmodelInstanceValue;
    };
    /**
     * method to access a view model property instance belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the image property
     */
    ViewModelInstance.prototype.image = function (path) {
        var viewmodelInstanceValue = this.propertyFromPath(path, PropertyType.Image);
        return viewmodelInstanceValue;
    };
    /**
     * method to access a view model property instance belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the font property
     */
    ViewModelInstance.prototype.font = function (path) {
        var viewmodelInstanceValue = this.propertyFromPath(path, PropertyType.Font);
        return viewmodelInstanceValue;
    };
    /**
     * method to access an artboard property instance belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the image property
     */
    ViewModelInstance.prototype.artboard = function (path) {
        var viewmodelInstanceValue = this.propertyFromPath(path, PropertyType.Artboard);
        return viewmodelInstanceValue;
    };
    /**
     * method to access a view model property instance belonging
     * to the view model instance or to a nested view model instance
     * @param path - path to the view model property
     */
    ViewModelInstance.prototype.viewModel = function (path) {
        var pathSegments = path.split("/");
        var parentViewModelInstance = pathSegments.length > 1
            ? this.viewModelFromPathSegments(pathSegments.slice(0, pathSegments.length - 1), 0)
            : this;
        if (parentViewModelInstance != null) {
            return parentViewModelInstance.internalViewModelInstance(pathSegments[pathSegments.length - 1]);
        }
        return null;
    };
    ViewModelInstance.prototype.internalReplaceViewModel = function (name, value) {
        var _a;
        if (value.runtimeInstance !== null) {
            var result = ((_a = this._runtimeInstance) === null || _a === void 0 ? void 0 : _a.replaceViewModel(name, value.runtimeInstance)) ||
                false;
            if (result) {
                value.internalIncrementReferenceCount();
                var oldInstance_1 = this.internalViewModelInstance(name);
                if (oldInstance_1 !== null) {
                    oldInstance_1.removeParent(this);
                    if (this._children.includes(oldInstance_1)) {
                        this._children = this._children.filter(function (child) { return child !== oldInstance_1; });
                    }
                    oldInstance_1.cleanup();
                }
                this._viewModelInstances.set(name, value);
                value.addParent(this);
            }
            return result;
        }
        return false;
    };
    /**
     * method to replace a view model property with another view model value
     * @param path - path to the view model property
     * @param value - view model that will replace the original
     */
    ViewModelInstance.prototype.replaceViewModel = function (path, value) {
        var _a;
        var pathSegments = path.split("/");
        var viewModelInstance = pathSegments.length > 1
            ? this.viewModelFromPathSegments(pathSegments.slice(0, pathSegments.length - 1), 0)
            : this;
        return ((_a = viewModelInstance === null || viewModelInstance === void 0 ? void 0 : viewModelInstance.internalReplaceViewModel(pathSegments[pathSegments.length - 1], value)) !== null && _a !== void 0 ? _a : false);
    };
    /*
     * method to add one to the reference counter of the instance.
     * Use if the file owning the reference is destroyed but the instance needs to stay around
     */
    ViewModelInstance.prototype.incrementReferenceCount = function () {
        var _a;
        this._referenceCount++;
        (_a = this._runtimeInstance) === null || _a === void 0 ? void 0 : _a.incrementReferenceCount();
    };
    /*
     * method to subtract one to the reference counter of the instance.
     * Use if incrementReferenceCount has been called
     */
    ViewModelInstance.prototype.decrementReferenceCount = function () {
        var _a;
        this._referenceCount--;
        (_a = this._runtimeInstance) === null || _a === void 0 ? void 0 : _a.decrementReferenceCount();
    };
    Object.defineProperty(ViewModelInstance.prototype, "properties", {
        get: function () {
            var _a;
            return (((_a = this._runtimeInstance) === null || _a === void 0 ? void 0 : _a.getProperties().map(function (prop) { return (__assign({}, prop)); })) || []);
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(ViewModelInstance.prototype, "viewModelName", {
        /**
         * Get the name of the ViewModel definition this instance was created from.
         */
        get: function () {
            var _a, _b;
            return (_b = (_a = this._runtimeInstance) === null || _a === void 0 ? void 0 : _a.getViewModelName()) !== null && _b !== void 0 ? _b : "";
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstance.prototype.internalIncrementReferenceCount = function () {
        this._referenceCount++;
    };
    ViewModelInstance.prototype.cleanup = function () {
        var _this = this;
        var _a;
        this._referenceCount--;
        if (this._referenceCount <= 0) {
            if (this.selfUnref) {
                (_a = this._runtimeInstance) === null || _a === void 0 ? void 0 : _a.unref();
            }
            this._runtimeInstance = null;
            this.clearCallbacks();
            this._propertiesWithCallbacks = [];
            this._viewModelInstances.forEach(function (value) {
                value.cleanup();
            });
            this._viewModelInstances.clear();
            var children = __spreadArray([], this._children, true);
            this._children.length = 0;
            var parents = __spreadArray([], this._parents, true);
            this._parents.length = 0;
            children.forEach(function (child) {
                child.removeParent(_this);
            });
            parents.forEach(function (parent) {
                parent.removeFromViewModelCallbacks(_this);
            });
        }
    };
    return ViewModelInstance;
}());

var ViewModelInstanceValue = /** @class */ (function () {
    function ViewModelInstanceValue(instance, parent) {
        this.callbacks = [];
        this._viewModelInstanceValue = instance;
        this._parentViewModel = parent;
    }
    ViewModelInstanceValue.prototype.on = function (callback) {
        // Since we don't clean the changed flag for properties that don't have listeners,
        // we clean it the first time we add a listener to it
        if (this.callbacks.length === 0) {
            this._viewModelInstanceValue.clearChanges();
        }
        if (!this.callbacks.includes(callback)) {
            this.callbacks.push(callback);
            this._parentViewModel.addToPropertyCallbacks(this);
        }
    };
    ViewModelInstanceValue.prototype.off = function (callback) {
        if (!callback) {
            this.callbacks.length = 0;
        }
        else {
            this.callbacks = this.callbacks.filter(function (cb) { return cb !== callback; });
        }
        if (this.callbacks.length === 0) {
            this._parentViewModel.removeFromPropertyCallbacks(this);
        }
    };
    ViewModelInstanceValue.prototype.internalHandleCallback = function (callback) { };
    ViewModelInstanceValue.prototype.handleCallbacks = function () {
        var _this = this;
        if (this._viewModelInstanceValue.hasChanged) {
            this.callbacks.forEach(function (callback) {
                _this.internalHandleCallback(callback);
            });
        }
    };
    ViewModelInstanceValue.prototype.clearChanges = function () {
        this._viewModelInstanceValue.clearChanges();
    };
    ViewModelInstanceValue.prototype.clearCallbacks = function () {
        this.callbacks.length = 0;
    };
    Object.defineProperty(ViewModelInstanceValue.prototype, "name", {
        get: function () {
            return this._viewModelInstanceValue.name;
        },
        enumerable: false,
        configurable: true
    });
    return ViewModelInstanceValue;
}());

var ViewModelInstanceString = /** @class */ (function (_super) {
    __extends(ViewModelInstanceString, _super);
    function ViewModelInstanceString(instance, parent) {
        return _super.call(this, instance, parent) || this;
    }
    Object.defineProperty(ViewModelInstanceString.prototype, "value", {
        get: function () {
            return this._viewModelInstanceValue.value;
        },
        set: function (val) {
            this._viewModelInstanceValue.value = val;
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstanceString.prototype.internalHandleCallback = function (callback) {
        callback(this.value);
    };
    return ViewModelInstanceString;
}(ViewModelInstanceValue));

var ViewModelInstanceNumber = /** @class */ (function (_super) {
    __extends(ViewModelInstanceNumber, _super);
    function ViewModelInstanceNumber(instance, parent) {
        return _super.call(this, instance, parent) || this;
    }
    Object.defineProperty(ViewModelInstanceNumber.prototype, "value", {
        get: function () {
            return this._viewModelInstanceValue.value;
        },
        set: function (val) {
            this._viewModelInstanceValue.value = val;
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstanceNumber.prototype.internalHandleCallback = function (callback) {
        callback(this.value);
    };
    return ViewModelInstanceNumber;
}(ViewModelInstanceValue));

var ViewModelInstanceBoolean = /** @class */ (function (_super) {
    __extends(ViewModelInstanceBoolean, _super);
    function ViewModelInstanceBoolean(instance, parent) {
        return _super.call(this, instance, parent) || this;
    }
    Object.defineProperty(ViewModelInstanceBoolean.prototype, "value", {
        get: function () {
            return this._viewModelInstanceValue.value;
        },
        set: function (val) {
            this._viewModelInstanceValue.value = val;
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstanceBoolean.prototype.internalHandleCallback = function (callback) {
        callback(this.value);
    };
    return ViewModelInstanceBoolean;
}(ViewModelInstanceValue));

var ViewModelInstanceTrigger = /** @class */ (function (_super) {
    __extends(ViewModelInstanceTrigger, _super);
    function ViewModelInstanceTrigger(instance, parent) {
        return _super.call(this, instance, parent) || this;
    }
    ViewModelInstanceTrigger.prototype.trigger = function () {
        return this._viewModelInstanceValue.trigger();
    };
    ViewModelInstanceTrigger.prototype.internalHandleCallback = function (callback) {
        callback();
    };
    return ViewModelInstanceTrigger;
}(ViewModelInstanceValue));

var ViewModelInstanceEnum = /** @class */ (function (_super) {
    __extends(ViewModelInstanceEnum, _super);
    function ViewModelInstanceEnum(instance, parent) {
        return _super.call(this, instance, parent) || this;
    }
    Object.defineProperty(ViewModelInstanceEnum.prototype, "value", {
        get: function () {
            return this._viewModelInstanceValue.value;
        },
        set: function (val) {
            this._viewModelInstanceValue.value = val;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(ViewModelInstanceEnum.prototype, "valueIndex", {
        get: function () {
            return this._viewModelInstanceValue
                .valueIndex;
        },
        set: function (val) {
            this._viewModelInstanceValue.valueIndex = val;
        },
        enumerable: false,
        configurable: true
    });
    Object.defineProperty(ViewModelInstanceEnum.prototype, "values", {
        get: function () {
            return this._viewModelInstanceValue.values;
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstanceEnum.prototype.internalHandleCallback = function (callback) {
        callback(this.value);
    };
    return ViewModelInstanceEnum;
}(ViewModelInstanceValue));

var ViewModelInstanceList = /** @class */ (function (_super) {
    __extends(ViewModelInstanceList, _super);
    function ViewModelInstanceList(instance, parent) {
        return _super.call(this, instance, parent) || this;
    }
    Object.defineProperty(ViewModelInstanceList.prototype, "length", {
        get: function () {
            return this._viewModelInstanceValue.size;
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstanceList.prototype.addInstance = function (instance) {
        if (instance.runtimeInstance != null) {
            this._viewModelInstanceValue.addInstance(instance.runtimeInstance);
            instance.addParent(this._parentViewModel);
        }
    };
    ViewModelInstanceList.prototype.addInstanceAt = function (instance, index) {
        if (instance.runtimeInstance != null) {
            if (this._viewModelInstanceValue.addInstanceAt(instance.runtimeInstance, index)) {
                instance.addParent(this._parentViewModel);
                return true;
            }
        }
        return false;
    };
    ViewModelInstanceList.prototype.removeInstance = function (instance) {
        if (instance.runtimeInstance != null) {
            this._viewModelInstanceValue.removeInstance(instance.runtimeInstance);
            instance.removeParent(this._parentViewModel);
        }
    };
    ViewModelInstanceList.prototype.removeInstanceAt = function (index) {
        this._viewModelInstanceValue.removeInstanceAt(index);
    };
    ViewModelInstanceList.prototype.instanceAt = function (index) {
        var runtimeInstance = this._viewModelInstanceValue.instanceAt(index);
        if (runtimeInstance != null) {
            var viewModelInstance = new ViewModelInstance(runtimeInstance, this._parentViewModel);
            (0,_utils__WEBPACK_IMPORTED_MODULE_3__.createFinalization)(viewModelInstance, runtimeInstance);
            return viewModelInstance;
        }
        return null;
    };
    ViewModelInstanceList.prototype.swap = function (a, b) {
        this._viewModelInstanceValue.swap(a, b);
    };
    ViewModelInstanceList.prototype.internalHandleCallback = function (callback) {
        callback();
    };
    return ViewModelInstanceList;
}(ViewModelInstanceValue));

var ViewModelInstanceColor = /** @class */ (function (_super) {
    __extends(ViewModelInstanceColor, _super);
    function ViewModelInstanceColor(instance, parent) {
        return _super.call(this, instance, parent) || this;
    }
    Object.defineProperty(ViewModelInstanceColor.prototype, "value", {
        get: function () {
            return this._viewModelInstanceValue.value;
        },
        set: function (val) {
            this._viewModelInstanceValue.value = val;
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstanceColor.prototype.rgb = function (r, g, b) {
        this._viewModelInstanceValue.rgb(r, g, b);
    };
    ViewModelInstanceColor.prototype.rgba = function (r, g, b, a) {
        this._viewModelInstanceValue.argb(a, r, g, b);
    };
    ViewModelInstanceColor.prototype.argb = function (a, r, g, b) {
        this._viewModelInstanceValue.argb(a, r, g, b);
    };
    // Value 0 to 255
    ViewModelInstanceColor.prototype.alpha = function (a) {
        this._viewModelInstanceValue.alpha(a);
    };
    // Value 0 to 1
    ViewModelInstanceColor.prototype.opacity = function (o) {
        this._viewModelInstanceValue.alpha(Math.round(Math.max(0, Math.min(1, o)) * 255));
    };
    ViewModelInstanceColor.prototype.internalHandleCallback = function (callback) {
        callback(this.value);
    };
    return ViewModelInstanceColor;
}(ViewModelInstanceValue));

var ViewModelInstanceAssetImage = /** @class */ (function (_super) {
    __extends(ViewModelInstanceAssetImage, _super);
    function ViewModelInstanceAssetImage(instance, root) {
        return _super.call(this, instance, root) || this;
    }
    Object.defineProperty(ViewModelInstanceAssetImage.prototype, "value", {
        set: function (image) {
            var _a;
            this._viewModelInstanceValue.value((_a = image === null || image === void 0 ? void 0 : image.nativeImage) !== null && _a !== void 0 ? _a : null);
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstanceAssetImage.prototype.internalHandleCallback = function (callback) {
        callback();
    };
    return ViewModelInstanceAssetImage;
}(ViewModelInstanceValue));

var ViewModelInstanceAssetFont = /** @class */ (function (_super) {
    __extends(ViewModelInstanceAssetFont, _super);
    function ViewModelInstanceAssetFont(instance, root) {
        return _super.call(this, instance, root) || this;
    }
    Object.defineProperty(ViewModelInstanceAssetFont.prototype, "value", {
        set: function (font) {
            var _a;
            this._viewModelInstanceValue.value((_a = font === null || font === void 0 ? void 0 : font.nativeFont) !== null && _a !== void 0 ? _a : null);
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstanceAssetFont.prototype.internalHandleCallback = function (callback) {
        callback();
    };
    return ViewModelInstanceAssetFont;
}(ViewModelInstanceValue));

var ViewModelInstanceArtboard = /** @class */ (function (_super) {
    __extends(ViewModelInstanceArtboard, _super);
    function ViewModelInstanceArtboard(instance, root) {
        return _super.call(this, instance, root) || this;
    }
    Object.defineProperty(ViewModelInstanceArtboard.prototype, "value", {
        set: function (artboard) {
            var _a, _b;
            var bindableArtboard;
            if (artboard.isBindableArtboard) {
                bindableArtboard = artboard;
            }
            else {
                bindableArtboard = artboard.file.internalBindableArtboardFromArtboard(artboard.nativeArtboard);
            }
            this._viewModelInstanceValue.value((_a = bindableArtboard === null || bindableArtboard === void 0 ? void 0 : bindableArtboard.nativeArtboard) !== null && _a !== void 0 ? _a : null);
            if (bindableArtboard === null || bindableArtboard === void 0 ? void 0 : bindableArtboard.nativeViewModel) {
                this._viewModelInstanceValue.viewModelInstance((_b = bindableArtboard === null || bindableArtboard === void 0 ? void 0 : bindableArtboard.nativeViewModel) !== null && _b !== void 0 ? _b : null);
            }
        },
        enumerable: false,
        configurable: true
    });
    ViewModelInstanceArtboard.prototype.internalHandleCallback = function (callback) {
        callback();
    };
    return ViewModelInstanceArtboard;
}(ViewModelInstanceValue));

// Loads Rive data from a URI via fetch.
var loadRiveFile = function (src) { return __awaiter(void 0, void 0, void 0, function () {
    var req, res, buffer;
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0:
                req = new Request(src);
                return [4 /*yield*/, fetch(req)];
            case 1:
                res = _a.sent();
                if (!res.ok) {
                    throw new Error("Failed to fetch the Rive file: HTTP ".concat(res.status));
                }
                return [4 /*yield*/, res.arrayBuffer()];
            case 2:
                buffer = _a.sent();
                return [2 /*return*/, buffer];
        }
    });
}); };
// #endregion
// #region utility functions
/*
 * Utility function to ensure an object is a string array
 */
var mapToStringArray = function (obj) {
    if (typeof obj === "string") {
        return [obj];
    }
    else if (obj instanceof Array) {
        return obj;
    }
    // If obj is undefined, return empty array
    return [];
};
// #endregion
// #region testing utilities
// Exports to only be used for tests
var Testing = {
    EventManager: EventManager,
    TaskQueueManager: TaskQueueManager,
};
// #endregion
// #region asset loaders
/**
 * Decodes bytes into an audio asset.
 *
 * Be sure to call `.unref()` on the audio once it is no longer needed. This
 * allows the engine to clean it up when it is not used by any more animations.
 */
var decodeAudio = function (bytes) { return __awaiter(void 0, void 0, void 0, function () {
    var decodedPromise, audio, audioWrapper;
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0:
                decodedPromise = new Promise(function (resolve) {
                    return _runtimeLoader__WEBPACK_IMPORTED_MODULE_1__.RuntimeLoader.getInstance(function (rive) {
                        rive.decodeAudio(bytes, resolve, null);
                    });
                });
                return [4 /*yield*/, decodedPromise];
            case 1:
                audio = _a.sent();
                audioWrapper = new _utils__WEBPACK_IMPORTED_MODULE_3__.AudioWrapper(audio);
                _utils__WEBPACK_IMPORTED_MODULE_3__.finalizationRegistry.register(audioWrapper, audio);
                return [2 /*return*/, audioWrapper];
        }
    });
}); };
/**
 * Decodes bytes into an image.
 *
 * Be sure to call `.unref()` on the image once it is no longer needed. This
 * allows the engine to clean it up when it is not used by any more animations.
 */
var decodeImage = function (bytes) { return __awaiter(void 0, void 0, void 0, function () {
    var decodedPromise, image, imageWrapper;
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0:
                decodedPromise = new Promise(function (resolve) {
                    return _runtimeLoader__WEBPACK_IMPORTED_MODULE_1__.RuntimeLoader.getInstance(function (rive) {
                        rive.decodeImage(bytes, resolve, null);
                    });
                });
                return [4 /*yield*/, decodedPromise];
            case 1:
                image = _a.sent();
                imageWrapper = new _utils__WEBPACK_IMPORTED_MODULE_3__.ImageWrapper(image);
                _utils__WEBPACK_IMPORTED_MODULE_3__.finalizationRegistry.register(imageWrapper, image);
                return [2 /*return*/, imageWrapper];
        }
    });
}); };
/**
 * Decodes bytes into a font.
 *
 * Be sure to call `.unref()` on the font once it is no longer needed. This
 * allows the engine to clean it up when it is not used by any more animations.
 */
var decodeFont = function (bytes) { return __awaiter(void 0, void 0, void 0, function () {
    var decodedPromise, font, fontWrapper;
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0:
                decodedPromise = new Promise(function (resolve) {
                    return _runtimeLoader__WEBPACK_IMPORTED_MODULE_1__.RuntimeLoader.getInstance(function (rive) {
                        rive.decodeFont(bytes, resolve, null);
                    });
                });
                return [4 /*yield*/, decodedPromise];
            case 1:
                font = _a.sent();
                fontWrapper = new _utils__WEBPACK_IMPORTED_MODULE_3__.FontWrapper(font);
                _utils__WEBPACK_IMPORTED_MODULE_3__.finalizationRegistry.register(fontWrapper, font);
                return [2 /*return*/, fontWrapper];
        }
    });
}); };
// #endregion

})();

/******/ 	return __webpack_exports__;
/******/ })()
;
});
//# sourceMappingURL=rive.js.map