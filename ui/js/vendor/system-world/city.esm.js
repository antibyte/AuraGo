var vs={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2},_s={ROTATE:0,PAN:1,DOLLY_PAN:2,DOLLY_ROTATE:3},rf=0,Dh=1,af=2;var qs=1,of=2,kr=3,ii=0,xn=1,Yt=2,si=0,Br=1,tn=2,Lh=3,Fh=4,lf=5;var Xs=100,cf=101,hf=102,uf=103,df=104,ff=200,pf=201,mf=202,gf=203,Nh=204,Uh=205,xf=206,bf=207,vf=208,_f=209,yf=210,Mf=211,wf=212,Sf=213,Ef=214,Zo=0,Qo=1,$o=2,br=3,el=4,tl=5,nl=6,il=7,Cl=0,Tf=1,Af=2,vi=0,Xa=1,ja=2,Ka=3,js=4,Ya=5,Ja=6,Za=7,yh="attached",Rf="detached",Oh=300,ys=301,Ks=302,Pl=303,Il=304,Qa=306,Ai=1e3,ti=1001,vr=1002,ln=1003,Dl=1004;var Ys=1005;var rn=1006,Hr=1007;var _i=1008;var Gn=1009,zh=1010,kh=1011,Gr=1012,Ll=1013,yi=1014,Zn=1015,gn=1016,Fl=1017,Nl=1018,Vr=1020,Bh=35902,Hh=35899,Gh=1021,Vh=1022,Qn=1023,Ri=1026,Ms=1027,Ul=1028,Ol=1029,ws=1030,zl=1031;var kl=1033,$a=33776,eo=33777,to=33778,no=33779,Bl=35840,Hl=35841,Gl=35842,Vl=35843,Wl=36196,ql=37492,Xl=37496,jl=37488,Kl=37489,io=37490,Yl=37491,Jl=37808,Zl=37809,Ql=37810,$l=37811,ec=37812,tc=37813,nc=37814,ic=37815,sc=37816,rc=37817,ac=37818,oc=37819,lc=37820,cc=37821,hc=36492,uc=36494,dc=36495,fc=36283,pc=36284,so=36285,mc=36286,gc=2200,Cf=2201,Pf=2202,Us=2300,Os=2301,Ko=2302,Mh=2303,Ls=2400,Fs=2401,ba=2402,xc=2500,If=2501,Wh=0,ro=1,Wr=2,Df=3200;var ao=0,Lf=1,ri="",sn="srgb",On="srgb-linear",va="linear",Wt="srgb";var Yo=7680;var Ff=519,Nf=512,Uf=513,Of=514,bc=515,zf=516,kf=517,vc=518,Bf=519,qh=35044;var Xh="300 es",fi=2e3,_r=2001;function Km(s){for(let e=s.length-1;e>=0;--e)if(s[e]>=65535)return!0;return!1}function Ym(s){return ArrayBuffer.isView(s)&&!(s instanceof DataView)}function yr(s){return document.createElementNS("http://www.w3.org/1999/xhtml",s)}function Hf(){let s=yr("canvas");return s.style.display="block",s}var md={},Mr=null;function _a(...s){let e="THREE."+s.shift();Mr?Mr("log",e,...s):console.log(e,...s)}function Gf(s){let e=s[0];if(typeof e=="string"&&e.startsWith("TSL:")){let t=s[1];t&&t.isStackTrace?s[0]+=" "+t.getLocation():s[1]='Stack trace not available. Enable "THREE.Node.captureStackTrace" to capture stack traces.'}return s}function ht(...s){s=Gf(s);let e="THREE."+s.shift();if(Mr)Mr("warn",e,...s);else{let t=s[0];t&&t.isStackTrace?console.warn(t.getError(e)):console.warn(e,...s)}}function gt(...s){s=Gf(s);let e="THREE."+s.shift();if(Mr)Mr("error",e,...s);else{let t=s[0];t&&t.isStackTrace?console.error(t.getError(e)):console.error(e,...s)}}function Ns(...s){let e=s.join(" ");e in md||(md[e]=!0,ht(...s))}function Vf(s,e,t){return new Promise(function(n,i){function r(){switch(s.clientWaitSync(e,s.SYNC_FLUSH_COMMANDS_BIT,0)){case s.WAIT_FAILED:i();break;case s.TIMEOUT_EXPIRED:setTimeout(r,t);break;default:n()}}setTimeout(r,t)})}var Wf={[Zo]:Qo,[$o]:nl,[el]:il,[br]:tl,[Qo]:Zo,[nl]:$o,[il]:el,[tl]:br},ni=class{addEventListener(e,t){this._listeners===void 0&&(this._listeners={});let n=this._listeners;n[e]===void 0&&(n[e]=[]),n[e].indexOf(t)===-1&&n[e].push(t)}hasEventListener(e,t){let n=this._listeners;return n===void 0?!1:n[e]!==void 0&&n[e].indexOf(t)!==-1}removeEventListener(e,t){let n=this._listeners;if(n===void 0)return;let i=n[e];if(i!==void 0){let r=i.indexOf(t);r!==-1&&i.splice(r,1)}}dispatchEvent(e){let t=this._listeners;if(t===void 0)return;let n=t[e.type];if(n!==void 0){e.target=this;let i=n.slice(0);for(let r=0,a=i.length;r<a;r++)i[r].call(this,e);e.target=null}}},Cn=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],gd=1234567,pa=Math.PI/180,zs=180/Math.PI;function pi(){let s=Math.random()*4294967295|0,e=Math.random()*4294967295|0,t=Math.random()*4294967295|0,n=Math.random()*4294967295|0;return(Cn[s&255]+Cn[s>>8&255]+Cn[s>>16&255]+Cn[s>>24&255]+"-"+Cn[e&255]+Cn[e>>8&255]+"-"+Cn[e>>16&15|64]+Cn[e>>24&255]+"-"+Cn[t&63|128]+Cn[t>>8&255]+"-"+Cn[t>>16&255]+Cn[t>>24&255]+Cn[n&255]+Cn[n>>8&255]+Cn[n>>16&255]+Cn[n>>24&255]).toLowerCase()}function Ct(s,e,t){return Math.max(e,Math.min(t,s))}function jh(s,e){return(s%e+e)%e}function Jm(s,e,t,n,i){return n+(s-e)*(i-n)/(t-e)}function Zm(s,e,t){return s!==e?(t-s)/(e-s):0}function ma(s,e,t){return(1-t)*s+t*e}function Qm(s,e,t,n){return ma(s,e,1-Math.exp(-t*n))}function $m(s,e=1){return e-Math.abs(jh(s,e*2)-e)}function eg(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*(3-2*s))}function tg(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*s*(s*(s*6-15)+10))}function ng(s,e){return s+Math.floor(Math.random()*(e-s+1))}function ig(s,e){return s+Math.random()*(e-s)}function sg(s){return s*(.5-Math.random())}function rg(s){s!==void 0&&(gd=s);let e=gd+=1831565813;return e=Math.imul(e^e>>>15,e|1),e^=e+Math.imul(e^e>>>7,e|61),((e^e>>>14)>>>0)/4294967296}function ag(s){return s*pa}function og(s){return s*zs}function lg(s){return s>0&&Number.isInteger(s)&&2**Math.round(Math.log2(s))===s}function cg(s){return Math.pow(2,Math.ceil(Math.log(s)/Math.LN2))}function hg(s){return Math.pow(2,Math.floor(Math.log(s)/Math.LN2))}function ug(s,e,t,n,i){let r=Math.cos,a=Math.sin,o=r(t/2),l=a(t/2),c=r((e+n)/2),h=a((e+n)/2),d=r((e-n)/2),u=a((e-n)/2),p=r((n-e)/2),x=a((n-e)/2);switch(i){case"XYX":s.set(o*h,l*d,l*u,o*c);break;case"YZY":s.set(l*u,o*h,l*d,o*c);break;case"ZXZ":s.set(l*d,l*u,o*h,o*c);break;case"XZX":s.set(o*h,l*x,l*p,o*c);break;case"YXY":s.set(l*p,o*h,l*x,o*c);break;case"ZYZ":s.set(l*x,l*p,o*h,o*c);break;default:ht("MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+i)}}function di(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return s/4294967295;case Uint16Array:return s/65535;case Uint8Array:case Uint8ClampedArray:return s/255;case Int32Array:return Math.max(s/2147483647,-1);case Int16Array:return Math.max(s/32767,-1);case Int8Array:return Math.max(s/127,-1);default:throw new Error("THREE.MathUtils: Invalid component type.")}}function Kt(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return Math.round(s*4294967295);case Uint16Array:return Math.round(s*65535);case Uint8Array:case Uint8ClampedArray:return Math.round(s*255);case Int32Array:return Math.round(s*2147483647);case Int16Array:return Math.round(s*32767);case Int8Array:return Math.round(s*127);default:throw new Error("THREE.MathUtils: Invalid component type.")}}var It={DEG2RAD:pa,RAD2DEG:zs,generateUUID:pi,clamp:Ct,euclideanModulo:jh,mapLinear:Jm,inverseLerp:Zm,lerp:ma,damp:Qm,pingpong:$m,smoothstep:eg,smootherstep:tg,randInt:ng,randFloat:ig,randFloatSpread:sg,seededRandom:rg,degToRad:ag,radToDeg:og,isPowerOfTwo:lg,ceilPowerOfTwo:cg,floorPowerOfTwo:hg,setQuaternionFromProperEuler:ug,normalize:Kt,denormalize:di},ze=class s{static{s.prototype.isVector2=!0}constructor(e=0,t=0){this.x=e,this.y=t}get width(){return this.x}set width(e){this.x=e}get height(){return this.y}set height(e){this.y=e}set(e,t){return this.x=e,this.y=t,this}setScalar(e){return this.x=e,this.y=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;default:throw new Error("THREE.Vector2: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;default:throw new Error("THREE.Vector2: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y)}copy(e){return this.x=e.x,this.y=e.y,this}add(e){return this.x+=e.x,this.y+=e.y,this}addScalar(e){return this.x+=e,this.y+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this}subScalar(e){return this.x-=e,this.y-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this}multiply(e){return this.x*=e.x,this.y*=e.y,this}multiplyScalar(e){return this.x*=e,this.y*=e,this}divide(e){return this.x/=e.x,this.y/=e.y,this}divideScalar(e){return this.multiplyScalar(1/e)}applyMatrix3(e){let t=this.x,n=this.y,i=e.elements;return this.x=i[0]*t+i[3]*n+i[6],this.y=i[1]*t+i[4]*n+i[7],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this}clamp(e,t){return this.x=Ct(this.x,e.x,t.x),this.y=Ct(this.y,e.y,t.y),this}clampScalar(e,t){return this.x=Ct(this.x,e,t),this.y=Ct(this.y,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(Ct(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(e){return this.x*e.x+this.y*e.y}cross(e){return this.x*e.y-this.y*e.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(Ct(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y;return t*t+n*n}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this}equals(e){return e.x===this.x&&e.y===this.y}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this}rotateAround(e,t){let n=Math.cos(t),i=Math.sin(t),r=this.x-e.x,a=this.y-e.y;return this.x=r*n-a*i+e.x,this.y=r*i+a*n+e.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},$t=class{constructor(e=0,t=0,n=0,i=1){this.isQuaternion=!0,this._x=e,this._y=t,this._z=n,this._w=i}static slerpFlat(e,t,n,i,r,a,o){let l=n[i+0],c=n[i+1],h=n[i+2],d=n[i+3],u=r[a+0],p=r[a+1],x=r[a+2],v=r[a+3];if(d!==v||l!==u||c!==p||h!==x){let m=l*u+c*p+h*x+d*v;m<0&&(u=-u,p=-p,x=-x,v=-v,m=-m);let f=1-o;if(m<.9995){let y=Math.acos(m),T=Math.sin(y);f=Math.sin(f*y)/T,o=Math.sin(o*y)/T,l=l*f+u*o,c=c*f+p*o,h=h*f+x*o,d=d*f+v*o}else{l=l*f+u*o,c=c*f+p*o,h=h*f+x*o,d=d*f+v*o;let y=1/Math.sqrt(l*l+c*c+h*h+d*d);l*=y,c*=y,h*=y,d*=y}}e[t]=l,e[t+1]=c,e[t+2]=h,e[t+3]=d}static multiplyQuaternionsFlat(e,t,n,i,r,a){let o=n[i],l=n[i+1],c=n[i+2],h=n[i+3],d=r[a],u=r[a+1],p=r[a+2],x=r[a+3];return e[t]=o*x+h*d+l*p-c*u,e[t+1]=l*x+h*u+c*d-o*p,e[t+2]=c*x+h*p+o*u-l*d,e[t+3]=h*x-o*d-l*u-c*p,e}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get w(){return this._w}set w(e){this._w=e,this._onChangeCallback()}set(e,t,n,i){return this._x=e,this._y=t,this._z=n,this._w=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(e){return this._x=e.x,this._y=e.y,this._z=e.z,this._w=e.w,this._onChangeCallback(),this}setFromEuler(e,t=!0){let n=e._x,i=e._y,r=e._z,a=e._order,o=Math.cos,l=Math.sin,c=o(n/2),h=o(i/2),d=o(r/2),u=l(n/2),p=l(i/2),x=l(r/2);switch(a){case"XYZ":this._x=u*h*d+c*p*x,this._y=c*p*d-u*h*x,this._z=c*h*x+u*p*d,this._w=c*h*d-u*p*x;break;case"YXZ":this._x=u*h*d+c*p*x,this._y=c*p*d-u*h*x,this._z=c*h*x-u*p*d,this._w=c*h*d+u*p*x;break;case"ZXY":this._x=u*h*d-c*p*x,this._y=c*p*d+u*h*x,this._z=c*h*x+u*p*d,this._w=c*h*d-u*p*x;break;case"ZYX":this._x=u*h*d-c*p*x,this._y=c*p*d+u*h*x,this._z=c*h*x-u*p*d,this._w=c*h*d+u*p*x;break;case"YZX":this._x=u*h*d+c*p*x,this._y=c*p*d+u*h*x,this._z=c*h*x-u*p*d,this._w=c*h*d-u*p*x;break;case"XZY":this._x=u*h*d-c*p*x,this._y=c*p*d-u*h*x,this._z=c*h*x+u*p*d,this._w=c*h*d+u*p*x;break;default:ht("Quaternion: .setFromEuler() encountered an unknown order: "+a)}return t===!0&&this._onChangeCallback(),this}setFromAxisAngle(e,t){let n=t/2,i=Math.sin(n);return this._x=e.x*i,this._y=e.y*i,this._z=e.z*i,this._w=Math.cos(n),this._onChangeCallback(),this}setFromRotationMatrix(e){let t=e.elements,n=t[0],i=t[4],r=t[8],a=t[1],o=t[5],l=t[9],c=t[2],h=t[6],d=t[10],u=n+o+d;if(u>0){let p=.5/Math.sqrt(u+1);this._w=.25/p,this._x=(h-l)*p,this._y=(r-c)*p,this._z=(a-i)*p}else if(n>o&&n>d){let p=2*Math.sqrt(1+n-o-d);this._w=(h-l)/p,this._x=.25*p,this._y=(i+a)/p,this._z=(r+c)/p}else if(o>d){let p=2*Math.sqrt(1+o-n-d);this._w=(r-c)/p,this._x=(i+a)/p,this._y=.25*p,this._z=(l+h)/p}else{let p=2*Math.sqrt(1+d-n-o);this._w=(a-i)/p,this._x=(r+c)/p,this._y=(l+h)/p,this._z=.25*p}return this._onChangeCallback(),this}setFromUnitVectors(e,t){let n=e.dot(t)+1;return n<1e-8?(n=0,Math.abs(e.x)>Math.abs(e.z)?(this._x=-e.y,this._y=e.x,this._z=0,this._w=n):(this._x=0,this._y=-e.z,this._z=e.y,this._w=n)):(this._x=e.y*t.z-e.z*t.y,this._y=e.z*t.x-e.x*t.z,this._z=e.x*t.y-e.y*t.x,this._w=n),this.normalize()}angleTo(e){return 2*Math.acos(Math.abs(Ct(this.dot(e),-1,1)))}rotateTowards(e,t){let n=this.angleTo(e);if(n===0)return this;let i=Math.min(1,t/n);return this.slerp(e,i),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(e){return this._x*e._x+this._y*e._y+this._z*e._z+this._w*e._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let e=this.length();return e===0?(this._x=0,this._y=0,this._z=0,this._w=1):(e=1/e,this._x=this._x*e,this._y=this._y*e,this._z=this._z*e,this._w=this._w*e),this._onChangeCallback(),this}multiply(e){return this.multiplyQuaternions(this,e)}premultiply(e){return this.multiplyQuaternions(e,this)}multiplyQuaternions(e,t){let n=e._x,i=e._y,r=e._z,a=e._w,o=t._x,l=t._y,c=t._z,h=t._w;return this._x=n*h+a*o+i*c-r*l,this._y=i*h+a*l+r*o-n*c,this._z=r*h+a*c+n*l-i*o,this._w=a*h-n*o-i*l-r*c,this._onChangeCallback(),this}slerp(e,t){let n=e._x,i=e._y,r=e._z,a=e._w,o=this.dot(e);o<0&&(n=-n,i=-i,r=-r,a=-a,o=-o);let l=1-t;if(o<.9995){let c=Math.acos(o),h=Math.sin(c);l=Math.sin(l*c)/h,t=Math.sin(t*c)/h,this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+a*t,this._onChangeCallback()}else this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+a*t,this.normalize();return this}slerpQuaternions(e,t,n){return this.copy(e).slerp(t,n)}random(){let e=2*Math.PI*Math.random(),t=2*Math.PI*Math.random(),n=Math.random(),i=Math.sqrt(1-n),r=Math.sqrt(n);return this.set(i*Math.sin(e),i*Math.cos(e),r*Math.sin(t),r*Math.cos(t))}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._w===this._w}fromArray(e,t=0){return this._x=e[t],this._y=e[t+1],this._z=e[t+2],this._w=e[t+3],this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._w,e}fromBufferAttribute(e,t){return this._x=e.getX(t),this._y=e.getY(t),this._z=e.getZ(t),this._w=e.getW(t),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},k=class s{static{s.prototype.isVector3=!0}constructor(e=0,t=0,n=0){this.x=e,this.y=t,this.z=n}set(e,t,n){return n===void 0&&(n=this.z),this.x=e,this.y=t,this.z=n,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;default:throw new Error("THREE.Vector3: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("THREE.Vector3: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this}multiplyVectors(e,t){return this.x=e.x*t.x,this.y=e.y*t.y,this.z=e.z*t.z,this}applyEuler(e){return this.applyQuaternion(xd.setFromEuler(e))}applyAxisAngle(e,t){return this.applyQuaternion(xd.setFromAxisAngle(e,t))}applyMatrix3(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[3]*n+r[6]*i,this.y=r[1]*t+r[4]*n+r[7]*i,this.z=r[2]*t+r[5]*n+r[8]*i,this}applyNormalMatrix(e){return this.applyMatrix3(e).normalize()}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=e.elements,a=1/(r[3]*t+r[7]*n+r[11]*i+r[15]);return this.x=(r[0]*t+r[4]*n+r[8]*i+r[12])*a,this.y=(r[1]*t+r[5]*n+r[9]*i+r[13])*a,this.z=(r[2]*t+r[6]*n+r[10]*i+r[14])*a,this}applyQuaternion(e){let t=this.x,n=this.y,i=this.z,r=e.x,a=e.y,o=e.z,l=e.w,c=2*(a*i-o*n),h=2*(o*t-r*i),d=2*(r*n-a*t);return this.x=t+l*c+a*d-o*h,this.y=n+l*h+o*c-r*d,this.z=i+l*d+r*h-a*c,this}project(e){return this.applyMatrix4(e.matrixWorldInverse).applyMatrix4(e.projectionMatrix)}unproject(e){return this.applyMatrix4(e.projectionMatrixInverse).applyMatrix4(e.matrixWorld)}transformDirection(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[4]*n+r[8]*i,this.y=r[1]*t+r[5]*n+r[9]*i,this.z=r[2]*t+r[6]*n+r[10]*i,this.normalize()}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this}divideScalar(e){return this.multiplyScalar(1/e)}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this}clamp(e,t){return this.x=Ct(this.x,e.x,t.x),this.y=Ct(this.y,e.y,t.y),this.z=Ct(this.z,e.z,t.z),this}clampScalar(e,t){return this.x=Ct(this.x,e,t),this.y=Ct(this.y,e,t),this.z=Ct(this.z,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(Ct(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this}cross(e){return this.crossVectors(this,e)}crossVectors(e,t){let n=e.x,i=e.y,r=e.z,a=t.x,o=t.y,l=t.z;return this.x=i*l-r*o,this.y=r*a-n*l,this.z=n*o-i*a,this}projectOnVector(e){let t=e.lengthSq();if(t===0)return this.set(0,0,0);let n=e.dot(this)/t;return this.copy(e).multiplyScalar(n)}projectOnPlane(e){return Kc.copy(this).projectOnVector(e),this.sub(Kc)}reflect(e){return this.sub(Kc.copy(e).multiplyScalar(2*this.dot(e)))}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(Ct(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y,i=this.z-e.z;return t*t+n*n+i*i}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)+Math.abs(this.z-e.z)}setFromSpherical(e){return this.setFromSphericalCoords(e.radius,e.phi,e.theta)}setFromSphericalCoords(e,t,n){let i=Math.sin(t)*e;return this.x=i*Math.sin(n),this.y=Math.cos(t)*e,this.z=i*Math.cos(n),this}setFromCylindrical(e){return this.setFromCylindricalCoords(e.radius,e.theta,e.y)}setFromCylindricalCoords(e,t,n){return this.x=e*Math.sin(t),this.y=n,this.z=e*Math.cos(t),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this}setFromMatrixScale(e){let t=this.setFromMatrixColumn(e,0).length(),n=this.setFromMatrixColumn(e,1).length(),i=this.setFromMatrixColumn(e,2).length();return this.x=t,this.y=n,this.z=i,this}setFromMatrixColumn(e,t){return this.fromArray(e.elements,t*4)}setFromMatrix3Column(e,t){return this.fromArray(e.elements,t*3)}setFromEuler(e){return this.x=e._x,this.y=e._y,this.z=e._z,this}setFromColor(e){return this.x=e.r,this.y=e.g,this.z=e.b,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let e=Math.random()*Math.PI*2,t=Math.random()*2-1,n=Math.sqrt(1-t*t);return this.x=n*Math.cos(e),this.y=t,this.z=n*Math.sin(e),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},Kc=new k,xd=new $t,_t=class s{static{s.prototype.isMatrix3=!0}constructor(e,t,n,i,r,a,o,l,c){this.elements=[1,0,0,0,1,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,a,o,l,c)}set(e,t,n,i,r,a,o,l,c){let h=this.elements;return h[0]=e,h[1]=i,h[2]=o,h[3]=t,h[4]=r,h[5]=l,h[6]=n,h[7]=a,h[8]=c,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],this}extractBasis(e,t,n){return e.setFromMatrix3Column(this,0),t.setFromMatrix3Column(this,1),n.setFromMatrix3Column(this,2),this}setFromMatrix4(e){let t=e.elements;return this.set(t[0],t[4],t[8],t[1],t[5],t[9],t[2],t[6],t[10]),this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,a=n[0],o=n[3],l=n[6],c=n[1],h=n[4],d=n[7],u=n[2],p=n[5],x=n[8],v=i[0],m=i[3],f=i[6],y=i[1],T=i[4],g=i[7],_=i[2],S=i[5],A=i[8];return r[0]=a*v+o*y+l*_,r[3]=a*m+o*T+l*S,r[6]=a*f+o*g+l*A,r[1]=c*v+h*y+d*_,r[4]=c*m+h*T+d*S,r[7]=c*f+h*g+d*A,r[2]=u*v+p*y+x*_,r[5]=u*m+p*T+x*S,r[8]=u*f+p*g+x*A,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[3]*=e,t[6]*=e,t[1]*=e,t[4]*=e,t[7]*=e,t[2]*=e,t[5]*=e,t[8]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8];return t*a*h-t*o*c-n*r*h+n*o*l+i*r*c-i*a*l}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8],d=h*a-o*c,u=o*l-h*r,p=c*r-a*l,x=t*d+n*u+i*p;if(x===0)return this.set(0,0,0,0,0,0,0,0,0);let v=1/x;return e[0]=d*v,e[1]=(i*c-h*n)*v,e[2]=(o*n-i*a)*v,e[3]=u*v,e[4]=(h*t-i*l)*v,e[5]=(i*r-o*t)*v,e[6]=p*v,e[7]=(n*l-c*t)*v,e[8]=(a*t-n*r)*v,this}transpose(){let e,t=this.elements;return e=t[1],t[1]=t[3],t[3]=e,e=t[2],t[2]=t[6],t[6]=e,e=t[5],t[5]=t[7],t[7]=e,this}getNormalMatrix(e){return this.setFromMatrix4(e).invert().transpose()}transposeIntoArray(e){let t=this.elements;return e[0]=t[0],e[1]=t[3],e[2]=t[6],e[3]=t[1],e[4]=t[4],e[5]=t[7],e[6]=t[2],e[7]=t[5],e[8]=t[8],this}setUvTransform(e,t,n,i,r,a,o){let l=Math.cos(r),c=Math.sin(r);return this.set(n*l,n*c,-n*(l*a+c*o)+a+e,-i*c,i*l,-i*(-c*a+l*o)+o+t,0,0,1),this}scale(e,t){return Ns("Matrix3: .scale() is deprecated. Use .makeScale() instead."),this.premultiply(Yc.makeScale(e,t)),this}rotate(e){return Ns("Matrix3: .rotate() is deprecated. Use .makeRotation() instead."),this.premultiply(Yc.makeRotation(-e)),this}translate(e,t){return Ns("Matrix3: .translate() is deprecated. Use .makeTranslation() instead."),this.premultiply(Yc.makeTranslation(e,t)),this}makeTranslation(e,t){return e.isVector2?this.set(1,0,e.x,0,1,e.y,0,0,1):this.set(1,0,e,0,1,t,0,0,1),this}makeRotation(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,n,t,0,0,0,1),this}makeScale(e,t){return this.set(e,0,0,0,t,0,0,0,1),this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<9;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<9;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e}clone(){return new this.constructor().fromArray(this.elements)}},Yc=new _t,bd=new _t().set(.4123908,.3575843,.1804808,.212639,.7151687,.0721923,.0193308,.1191948,.9505322),vd=new _t().set(3.2409699,-1.5373832,-.4986108,-.9692436,1.8759675,.0415551,.0556301,-.203977,1.0569715);function dg(){let s={enabled:!0,workingColorSpace:On,spaces:{},convert:function(i,r,a){return this.enabled===!1||r===a||!r||!a||(this.spaces[r].transfer===Wt&&(i.r=ji(i.r),i.g=ji(i.g),i.b=ji(i.b)),this.spaces[r].primaries!==this.spaces[a].primaries&&(i.applyMatrix3(this.spaces[r].toXYZ),i.applyMatrix3(this.spaces[a].fromXYZ)),this.spaces[a].transfer===Wt&&(i.r=xr(i.r),i.g=xr(i.g),i.b=xr(i.b))),i},workingToColorSpace:function(i,r){return this.convert(i,this.workingColorSpace,r)},colorSpaceToWorking:function(i,r){return this.convert(i,r,this.workingColorSpace)},getPrimaries:function(i){return this.spaces[i].primaries},getTransfer:function(i){return i===ri?va:this.spaces[i].transfer},getToneMappingMode:function(i){return this.spaces[i].outputColorSpaceConfig.toneMappingMode||"standard"},getLuminanceCoefficients:function(i,r=this.workingColorSpace){return i.fromArray(this.spaces[r].luminanceCoefficients)},define:function(i){Object.assign(this.spaces,i)},_getMatrix:function(i,r,a){return i.copy(this.spaces[r].toXYZ).multiply(this.spaces[a].fromXYZ)},_getDrawingBufferColorSpace:function(i){return this.spaces[i].outputColorSpaceConfig.drawingBufferColorSpace},_getUnpackColorSpace:function(i=this.workingColorSpace){return this.spaces[i].workingColorSpaceConfig.unpackColorSpace},fromWorkingColorSpace:function(i,r){return Ns("ColorManagement: .fromWorkingColorSpace() has been renamed to .workingToColorSpace()."),s.workingToColorSpace(i,r)},toWorkingColorSpace:function(i,r){return Ns("ColorManagement: .toWorkingColorSpace() has been renamed to .colorSpaceToWorking()."),s.colorSpaceToWorking(i,r)}},e=[.64,.33,.3,.6,.15,.06],t=[.2126,.7152,.0722],n=[.3127,.329];return s.define({[On]:{primaries:e,whitePoint:n,transfer:va,toXYZ:bd,fromXYZ:vd,luminanceCoefficients:t,workingColorSpaceConfig:{unpackColorSpace:sn},outputColorSpaceConfig:{drawingBufferColorSpace:sn}},[sn]:{primaries:e,whitePoint:n,transfer:Wt,toXYZ:bd,fromXYZ:vd,luminanceCoefficients:t,outputColorSpaceConfig:{drawingBufferColorSpace:sn}}}),s}var Rt=dg();function ji(s){return s<.04045?s*.0773993808:Math.pow(s*.9478672986+.0521327014,2.4)}function xr(s){return s<.0031308?s*12.92:1.055*Math.pow(s,.41666)-.055}var ir,sl=class{static getDataURL(e,t="image/png"){if(/^data:/i.test(e.src)||typeof HTMLCanvasElement>"u")return e.src;let n;if(e instanceof HTMLCanvasElement)n=e;else{ir===void 0&&(ir=yr("canvas")),ir.width=e.width,ir.height=e.height;let i=ir.getContext("2d");e instanceof ImageData?i.putImageData(e,0,0):i.drawImage(e,0,0,e.width,e.height),n=ir}return n.toDataURL(t)}static sRGBToLinear(e){if(typeof HTMLImageElement<"u"&&e instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&e instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&e instanceof ImageBitmap){let t=yr("canvas");t.width=e.width,t.height=e.height;let n=t.getContext("2d");n.drawImage(e,0,0,e.width,e.height);let i=n.getImageData(0,0,e.width,e.height),r=i.data;for(let a=0;a<r.length;a++)r[a]=ji(r[a]/255)*255;return n.putImageData(i,0,0),t}else if(e.data){let t=e.data.slice(0);for(let n=0;n<t.length;n++)t instanceof Uint8Array||t instanceof Uint8ClampedArray?t[n]=Math.floor(ji(t[n]/255)*255):t[n]=ji(t[n]);return{data:t,width:e.width,height:e.height}}else return ht("ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),e}},fg=0,wr=class{constructor(e=null){this.isTextureSource=!0,Object.defineProperty(this,"id",{value:fg++}),this.uuid=pi(),this.data=e,this.dataReady=!0,this.version=0}getSize(e){let t=this.data;return typeof HTMLVideoElement<"u"&&t instanceof HTMLVideoElement?e.set(t.videoWidth,t.videoHeight,0):typeof VideoFrame<"u"&&t instanceof VideoFrame?e.set(t.displayWidth,t.displayHeight,0):t!==null?e.set(t.width,t.height,t.depth||0):e.set(0,0,0),e}set needsUpdate(e){e===!0&&this.version++}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.images[this.uuid]!==void 0)return e.images[this.uuid];let n={uuid:this.uuid,url:""},i=this.data;if(i!==null){let r;if(Array.isArray(i)){r=[];for(let a=0,o=i.length;a<o;a++)i[a].isDataTexture?r.push(Jc(i[a].image)):r.push(Jc(i[a]))}else r=Jc(i);n.url=r}return t||(e.images[this.uuid]=n),n}};function Jc(s){return typeof HTMLImageElement<"u"&&s instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&s instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&s instanceof ImageBitmap?sl.getDataURL(s):s.data?{data:Array.from(s.data),width:s.width,height:s.height,type:s.data.constructor.name}:(ht("Texture: Unable to serialize Texture."),{})}var pg=0,Zc=new k,cn=class s extends ni{constructor(e=s.DEFAULT_IMAGE,t=s.DEFAULT_MAPPING,n=ti,i=ti,r=rn,a=_i,o=Qn,l=Gn,c=s.DEFAULT_ANISOTROPY,h=ri){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:pg++}),this.uuid=pi(),this.name="",this.source=new wr(e),this.mipmaps=[],this.mapping=t,this.channel=0,this.wrapS=n,this.wrapT=i,this.magFilter=r,this.minFilter=a,this.anisotropy=c,this.format=o,this.internalFormat=null,this.type=l,this.offset=new ze(0,0),this.repeat=new ze(1,1),this.center=new ze(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new _t,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=h,this.userData={},this.updateRanges=[],this.version=0,this.onUpdate=null,this.renderTarget=null,this.isRenderTargetTexture=!1,this.isArrayTexture=!!(e&&e.depth&&e.depth>1),this.pmremVersion=0,this.normalized=!1}get width(){return this.source.getSize(Zc).x}get height(){return this.source.getSize(Zc).y}get depth(){return this.source.getSize(Zc).z}get image(){return this.source.data}set image(e){this.source.data=e}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}clone(){return new this.constructor().copy(this)}copy(e){return this.name=e.name,this.source=e.source,this.mipmaps=e.mipmaps.slice(0),this.mapping=e.mapping,this.channel=e.channel,this.wrapS=e.wrapS,this.wrapT=e.wrapT,this.magFilter=e.magFilter,this.minFilter=e.minFilter,this.anisotropy=e.anisotropy,this.format=e.format,this.internalFormat=e.internalFormat,this.type=e.type,this.normalized=e.normalized,this.offset.copy(e.offset),this.repeat.copy(e.repeat),this.center.copy(e.center),this.rotation=e.rotation,this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrix.copy(e.matrix),this.generateMipmaps=e.generateMipmaps,this.premultiplyAlpha=e.premultiplyAlpha,this.flipY=e.flipY,this.unpackAlignment=e.unpackAlignment,this.colorSpace=e.colorSpace,this.renderTarget=e.renderTarget,this.isRenderTargetTexture=e.isRenderTargetTexture,this.isArrayTexture=e.isArrayTexture,this.userData=JSON.parse(JSON.stringify(e.userData)),this.needsUpdate=!0,this}setValues(e){for(let t in e){let n=e[t];if(n===void 0){ht(`Texture.setValues(): parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){ht(`Texture.setValues(): property '${t}' does not exist.`);continue}i&&n&&i.isVector2&&n.isVector2||i&&n&&i.isVector3&&n.isVector3||i&&n&&i.isMatrix3&&n.isMatrix3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.textures[this.uuid]!==void 0)return e.textures[this.uuid];let n={metadata:{version:4.7,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(e).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,normalized:this.normalized,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(n.userData=this.userData),t||(e.textures[this.uuid]=n),n}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(e){if(this.mapping!==Oh)return e;if(e.applyMatrix3(this.matrix),e.x<0||e.x>1)switch(this.wrapS){case Ai:e.x=e.x-Math.floor(e.x);break;case ti:e.x=e.x<0?0:1;break;case vr:Math.abs(Math.floor(e.x)%2)===1?e.x=Math.ceil(e.x)-e.x:e.x=e.x-Math.floor(e.x);break}if(e.y<0||e.y>1)switch(this.wrapT){case Ai:e.y=e.y-Math.floor(e.y);break;case ti:e.y=e.y<0?0:1;break;case vr:Math.abs(Math.floor(e.y)%2)===1?e.y=Math.ceil(e.y)-e.y:e.y=e.y-Math.floor(e.y);break}return this.flipY&&(e.y=1-e.y),e}set needsUpdate(e){e===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(e){e===!0&&this.pmremVersion++}};cn.DEFAULT_IMAGE=null;cn.DEFAULT_MAPPING=Oh;cn.DEFAULT_ANISOTROPY=1;var Nt=class s{static{s.prototype.isVector4=!0}constructor(e=0,t=0,n=0,i=1){this.x=e,this.y=t,this.z=n,this.w=i}get width(){return this.z}set width(e){this.z=e}get height(){return this.w}set height(e){this.w=e}set(e,t,n,i){return this.x=e,this.y=t,this.z=n,this.w=i,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this.w=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setW(e){return this.w=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;case 3:this.w=t;break;default:throw new Error("THREE.Vector4: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("THREE.Vector4: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this.w=e.w!==void 0?e.w:1,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this.w+=e.w,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this.w+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this.w=e.w+t.w,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this.w+=e.w*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this.w-=e.w,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this.w-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this.w=e.w-t.w,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this.w*=e.w,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this.w*=e,this}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=this.w,a=e.elements;return this.x=a[0]*t+a[4]*n+a[8]*i+a[12]*r,this.y=a[1]*t+a[5]*n+a[9]*i+a[13]*r,this.z=a[2]*t+a[6]*n+a[10]*i+a[14]*r,this.w=a[3]*t+a[7]*n+a[11]*i+a[15]*r,this}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this.w/=e.w,this}divideScalar(e){return this.multiplyScalar(1/e)}setAxisAngleFromQuaternion(e){this.w=2*Math.acos(e.w);let t=Math.sqrt(1-e.w*e.w);return t<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=e.x/t,this.y=e.y/t,this.z=e.z/t),this}setAxisAngleFromRotationMatrix(e){let t,n,i,r,l=e.elements,c=l[0],h=l[4],d=l[8],u=l[1],p=l[5],x=l[9],v=l[2],m=l[6],f=l[10];if(Math.abs(h-u)<.01&&Math.abs(d-v)<.01&&Math.abs(x-m)<.01){if(Math.abs(h+u)<.1&&Math.abs(d+v)<.1&&Math.abs(x+m)<.1&&Math.abs(c+p+f-3)<.1)return this.set(1,0,0,0),this;t=Math.PI;let T=(c+1)/2,g=(p+1)/2,_=(f+1)/2,S=(h+u)/4,A=(d+v)/4,b=(x+m)/4;return T>g&&T>_?T<.01?(n=0,i=.707106781,r=.707106781):(n=Math.sqrt(T),i=S/n,r=A/n):g>_?g<.01?(n=.707106781,i=0,r=.707106781):(i=Math.sqrt(g),n=S/i,r=b/i):_<.01?(n=.707106781,i=.707106781,r=0):(r=Math.sqrt(_),n=A/r,i=b/r),this.set(n,i,r,t),this}let y=Math.sqrt((m-x)*(m-x)+(d-v)*(d-v)+(u-h)*(u-h));return Math.abs(y)<.001&&(y=1),this.x=(m-x)/y,this.y=(d-v)/y,this.z=(u-h)/y,this.w=Math.acos((c+p+f-1)/2),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this.w=t[15],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this.w=Math.min(this.w,e.w),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this.w=Math.max(this.w,e.w),this}clamp(e,t){return this.x=Ct(this.x,e.x,t.x),this.y=Ct(this.y,e.y,t.y),this.z=Ct(this.z,e.z,t.z),this.w=Ct(this.w,e.w,t.w),this}clampScalar(e,t){return this.x=Ct(this.x,e,t),this.y=Ct(this.y,e,t),this.z=Ct(this.z,e,t),this.w=Ct(this.w,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(Ct(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z+this.w*e.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this.w+=(e.w-this.w)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this.w=e.w+(t.w-e.w)*n,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z&&e.w===this.w}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this.w=e[t+3],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e[t+3]=this.w,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this.w=e.getW(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},rl=class extends ni{constructor(e=1,t=1,n={}){super(),n=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:rn,depthBuffer:!0,stencilBuffer:!1,resolveColorBuffer:!0,resolveDepthBuffer:!0,resolveStencilBuffer:!0,storeMultisampledColorBuffer:!0,storeMultisampledDepthBuffer:!0,storeMultisampledStencilBuffer:!0,depthTexture:null,samples:0,count:1,depth:1,multiview:!1,useArrayDepthTexture:!1},n),this.isRenderTarget=!0,this.width=e,this.height=t,this.depth=n.depth,this.scissor=new Nt(0,0,e,t),this.scissorTest=!1,this.viewport=new Nt(0,0,e,t),this.textures=[];let i={width:e,height:t,depth:n.depth},r=new cn(i),a=n.count;for(let o=0;o<a;o++)this.textures[o]=r.clone(),this.textures[o].isRenderTargetTexture=!0,this.textures[o].renderTarget=this;this._setTextureOptions(n),this.depthBuffer=n.depthBuffer,this.stencilBuffer=n.stencilBuffer,this.resolveColorBuffer=n.resolveColorBuffer,this.resolveDepthBuffer=n.resolveDepthBuffer,this.resolveStencilBuffer=n.resolveStencilBuffer,this.storeMultisampledColorBuffer=n.storeMultisampledColorBuffer,this.storeMultisampledDepthBuffer=n.storeMultisampledDepthBuffer,this.storeMultisampledStencilBuffer=n.storeMultisampledStencilBuffer,this._depthTexture=null,this.depthTexture=n.depthTexture,this.samples=n.samples,this.multiview=n.multiview,this.useArrayDepthTexture=n.useArrayDepthTexture}_setTextureOptions(e={}){let t={minFilter:rn,generateMipmaps:!1,flipY:!1,internalFormat:null};e.mapping!==void 0&&(t.mapping=e.mapping),e.wrapS!==void 0&&(t.wrapS=e.wrapS),e.wrapT!==void 0&&(t.wrapT=e.wrapT),e.wrapR!==void 0&&(t.wrapR=e.wrapR),e.magFilter!==void 0&&(t.magFilter=e.magFilter),e.minFilter!==void 0&&(t.minFilter=e.minFilter),e.format!==void 0&&(t.format=e.format),e.type!==void 0&&(t.type=e.type),e.anisotropy!==void 0&&(t.anisotropy=e.anisotropy),e.colorSpace!==void 0&&(t.colorSpace=e.colorSpace),e.flipY!==void 0&&(t.flipY=e.flipY),e.generateMipmaps!==void 0&&(t.generateMipmaps=e.generateMipmaps),e.internalFormat!==void 0&&(t.internalFormat=e.internalFormat);for(let n=0;n<this.textures.length;n++)this.textures[n].setValues(t)}get texture(){return this.textures[0]}set texture(e){this.textures[0]=e}set depthTexture(e){this._depthTexture!==null&&this._depthTexture.renderTarget===this&&(this._depthTexture.renderTarget=null),e!==null&&e.renderTarget===null&&(e.renderTarget=this),this._depthTexture=e}get depthTexture(){return this._depthTexture}setSize(e,t,n=1){if(this.width!==e||this.height!==t||this.depth!==n){this.width=e,this.height=t,this.depth=n;for(let i=0,r=this.textures.length;i<r;i++)this.textures[i].image.width=e,this.textures[i].image.height=t,this.textures[i].image.depth=n,this.textures[i].isData3DTexture!==!0&&(this.textures[i].isArrayTexture=this.textures[i].image.depth>1);this.dispose()}this.viewport.set(0,0,e,t),this.scissor.set(0,0,e,t)}clone(){return new this.constructor().copy(this)}copy(e){this.width=e.width,this.height=e.height,this.depth=e.depth,this.scissor.copy(e.scissor),this.scissorTest=e.scissorTest,this.viewport.copy(e.viewport),this.textures.length=0;for(let t=0,n=e.textures.length;t<n;t++){this.textures[t]=e.textures[t].clone(),this.textures[t].isRenderTargetTexture=!0,this.textures[t].renderTarget=this;let i=Object.assign({},e.textures[t].image);this.textures[t].source=new wr(i)}if(this.depthBuffer=e.depthBuffer,this.stencilBuffer=e.stencilBuffer,this.resolveColorBuffer=e.resolveColorBuffer,this.resolveDepthBuffer=e.resolveDepthBuffer,this.resolveStencilBuffer=e.resolveStencilBuffer,this.storeMultisampledColorBuffer=e.storeMultisampledColorBuffer,this.storeMultisampledDepthBuffer=e.storeMultisampledDepthBuffer,this.storeMultisampledStencilBuffer=e.storeMultisampledStencilBuffer,e.depthTexture!==null)if(e.depthTexture.renderTarget===e){let t=e.depthTexture.clone();t.renderTarget=null,this.depthTexture=t}else this.depthTexture=e.depthTexture;return this.samples=e.samples,this.multiview=e.multiview,this.useArrayDepthTexture=e.useArrayDepthTexture,this}dispose(){this.dispatchEvent({type:"dispose"})}},an=class extends rl{constructor(e=1,t=1,n={}){super(e,t,n),this.isWebGLRenderTarget=!0}},ya=class extends cn{constructor(e=null,t=1,n=1,i=1){super(null),this.isDataArrayTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=ln,this.minFilter=ln,this.wrapR=ti,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}copy(e){return super.copy(e),this.wrapR=e.wrapR,this}addLayerUpdate(e){this.layerUpdates.add(e)}clearLayerUpdates(){this.layerUpdates.clear()}};var al=class extends cn{constructor(e=null,t=1,n=1,i=1){super(null),this.isData3DTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=ln,this.minFilter=ln,this.wrapR=ti,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}copy(e){return super.copy(e),this.wrapR=e.wrapR,this}};var mt=class s{static{s.prototype.isMatrix4=!0}constructor(e,t,n,i,r,a,o,l,c,h,d,u,p,x,v,m){this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,a,o,l,c,h,d,u,p,x,v,m)}set(e,t,n,i,r,a,o,l,c,h,d,u,p,x,v,m){let f=this.elements;return f[0]=e,f[4]=t,f[8]=n,f[12]=i,f[1]=r,f[5]=a,f[9]=o,f[13]=l,f[2]=c,f[6]=h,f[10]=d,f[14]=u,f[3]=p,f[7]=x,f[11]=v,f[15]=m,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new s().fromArray(this.elements)}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],t[9]=n[9],t[10]=n[10],t[11]=n[11],t[12]=n[12],t[13]=n[13],t[14]=n[14],t[15]=n[15],this}copyPosition(e){let t=this.elements,n=e.elements;return t[12]=n[12],t[13]=n[13],t[14]=n[14],this}setFromMatrix3(e){let t=e.elements;return this.set(t[0],t[3],t[6],0,t[1],t[4],t[7],0,t[2],t[5],t[8],0,0,0,0,1),this}extractBasis(e,t,n){return this.determinantAffine()===0?(e.set(1,0,0),t.set(0,1,0),n.set(0,0,1),this):(e.setFromMatrixColumn(this,0),t.setFromMatrixColumn(this,1),n.setFromMatrixColumn(this,2),this)}makeBasis(e,t,n){return this.set(e.x,t.x,n.x,0,e.y,t.y,n.y,0,e.z,t.z,n.z,0,0,0,0,1),this}extractRotation(e){if(e.determinantAffine()===0)return this.identity();let t=this.elements,n=e.elements,i=1/sr.setFromMatrixColumn(e,0).length(),r=1/sr.setFromMatrixColumn(e,1).length(),a=1/sr.setFromMatrixColumn(e,2).length();return t[0]=n[0]*i,t[1]=n[1]*i,t[2]=n[2]*i,t[3]=0,t[4]=n[4]*r,t[5]=n[5]*r,t[6]=n[6]*r,t[7]=0,t[8]=n[8]*a,t[9]=n[9]*a,t[10]=n[10]*a,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromEuler(e){let t=this.elements,n=e.x,i=e.y,r=e.z,a=Math.cos(n),o=Math.sin(n),l=Math.cos(i),c=Math.sin(i),h=Math.cos(r),d=Math.sin(r);if(e.order==="XYZ"){let u=a*h,p=a*d,x=o*h,v=o*d;t[0]=l*h,t[4]=-l*d,t[8]=c,t[1]=p+x*c,t[5]=u-v*c,t[9]=-o*l,t[2]=v-u*c,t[6]=x+p*c,t[10]=a*l}else if(e.order==="YXZ"){let u=l*h,p=l*d,x=c*h,v=c*d;t[0]=u+v*o,t[4]=x*o-p,t[8]=a*c,t[1]=a*d,t[5]=a*h,t[9]=-o,t[2]=p*o-x,t[6]=v+u*o,t[10]=a*l}else if(e.order==="ZXY"){let u=l*h,p=l*d,x=c*h,v=c*d;t[0]=u-v*o,t[4]=-a*d,t[8]=x+p*o,t[1]=p+x*o,t[5]=a*h,t[9]=v-u*o,t[2]=-a*c,t[6]=o,t[10]=a*l}else if(e.order==="ZYX"){let u=a*h,p=a*d,x=o*h,v=o*d;t[0]=l*h,t[4]=x*c-p,t[8]=u*c+v,t[1]=l*d,t[5]=v*c+u,t[9]=p*c-x,t[2]=-c,t[6]=o*l,t[10]=a*l}else if(e.order==="YZX"){let u=a*l,p=a*c,x=o*l,v=o*c;t[0]=l*h,t[4]=v-u*d,t[8]=x*d+p,t[1]=d,t[5]=a*h,t[9]=-o*h,t[2]=-c*h,t[6]=p*d+x,t[10]=u-v*d}else if(e.order==="XZY"){let u=a*l,p=a*c,x=o*l,v=o*c;t[0]=l*h,t[4]=-d,t[8]=c*h,t[1]=u*d+v,t[5]=a*h,t[9]=p*d-x,t[2]=x*d-p,t[6]=o*h,t[10]=v*d+u}return t[3]=0,t[7]=0,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromQuaternion(e){return this.compose(mg,e,gg)}lookAt(e,t,n){let i=this.elements;return Xn.subVectors(e,t),Xn.lengthSq()===0&&(Xn.z=1),Xn.normalize(),os.crossVectors(n,Xn),os.lengthSq()===0&&(Math.abs(n.z)===1?Xn.x+=1e-4:Xn.z+=1e-4,Xn.normalize(),os.crossVectors(n,Xn)),os.normalize(),wo.crossVectors(Xn,os),i[0]=os.x,i[4]=wo.x,i[8]=Xn.x,i[1]=os.y,i[5]=wo.y,i[9]=Xn.y,i[2]=os.z,i[6]=wo.z,i[10]=Xn.z,this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,a=n[0],o=n[4],l=n[8],c=n[12],h=n[1],d=n[5],u=n[9],p=n[13],x=n[2],v=n[6],m=n[10],f=n[14],y=n[3],T=n[7],g=n[11],_=n[15],S=i[0],A=i[4],b=i[8],C=i[12],w=i[1],I=i[5],M=i[9],N=i[13],D=i[2],U=i[6],H=i[10],L=i[14],W=i[3],z=i[7],E=i[11],J=i[15];return r[0]=a*S+o*w+l*D+c*W,r[4]=a*A+o*I+l*U+c*z,r[8]=a*b+o*M+l*H+c*E,r[12]=a*C+o*N+l*L+c*J,r[1]=h*S+d*w+u*D+p*W,r[5]=h*A+d*I+u*U+p*z,r[9]=h*b+d*M+u*H+p*E,r[13]=h*C+d*N+u*L+p*J,r[2]=x*S+v*w+m*D+f*W,r[6]=x*A+v*I+m*U+f*z,r[10]=x*b+v*M+m*H+f*E,r[14]=x*C+v*N+m*L+f*J,r[3]=y*S+T*w+g*D+_*W,r[7]=y*A+T*I+g*U+_*z,r[11]=y*b+T*M+g*H+_*E,r[15]=y*C+T*N+g*L+_*J,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[4]*=e,t[8]*=e,t[12]*=e,t[1]*=e,t[5]*=e,t[9]*=e,t[13]*=e,t[2]*=e,t[6]*=e,t[10]*=e,t[14]*=e,t[3]*=e,t[7]*=e,t[11]*=e,t[15]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[12],a=e[1],o=e[5],l=e[9],c=e[13],h=e[2],d=e[6],u=e[10],p=e[14],x=e[3],v=e[7],m=e[11],f=e[15],y=l*p-c*u,T=o*p-c*d,g=o*u-l*d,_=a*p-c*h,S=a*u-l*h,A=a*d-o*h;return t*(v*y-m*T+f*g)-n*(x*y-m*_+f*S)+i*(x*T-v*_+f*A)-r*(x*g-v*S+m*A)}determinantAffine(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[1],a=e[5],o=e[9],l=e[2],c=e[6],h=e[10];return t*(a*h-o*c)-n*(r*h-o*l)+i*(r*c-a*l)}transpose(){let e=this.elements,t;return t=e[1],e[1]=e[4],e[4]=t,t=e[2],e[2]=e[8],e[8]=t,t=e[6],e[6]=e[9],e[9]=t,t=e[3],e[3]=e[12],e[12]=t,t=e[7],e[7]=e[13],e[13]=t,t=e[11],e[11]=e[14],e[14]=t,this}setPosition(e,t,n){let i=this.elements;return e.isVector3?(i[12]=e.x,i[13]=e.y,i[14]=e.z):(i[12]=e,i[13]=t,i[14]=n),this}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8],d=e[9],u=e[10],p=e[11],x=e[12],v=e[13],m=e[14],f=e[15],y=t*o-n*a,T=t*l-i*a,g=t*c-r*a,_=n*l-i*o,S=n*c-r*o,A=i*c-r*l,b=h*v-d*x,C=h*m-u*x,w=h*f-p*x,I=d*m-u*v,M=d*f-p*v,N=u*f-p*m,D=y*N-T*M+g*I+_*w-S*C+A*b;if(D===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let U=1/D;return e[0]=(o*N-l*M+c*I)*U,e[1]=(i*M-n*N-r*I)*U,e[2]=(v*A-m*S+f*_)*U,e[3]=(u*S-d*A-p*_)*U,e[4]=(l*w-a*N-c*C)*U,e[5]=(t*N-i*w+r*C)*U,e[6]=(m*g-x*A-f*T)*U,e[7]=(h*A-u*g+p*T)*U,e[8]=(a*M-o*w+c*b)*U,e[9]=(n*w-t*M-r*b)*U,e[10]=(x*S-v*g+f*y)*U,e[11]=(d*g-h*S-p*y)*U,e[12]=(o*C-a*I-l*b)*U,e[13]=(t*I-n*C+i*b)*U,e[14]=(v*T-x*_-m*y)*U,e[15]=(h*_-d*T+u*y)*U,this}scale(e){let t=this.elements,n=e.x,i=e.y,r=e.z;return t[0]*=n,t[4]*=i,t[8]*=r,t[1]*=n,t[5]*=i,t[9]*=r,t[2]*=n,t[6]*=i,t[10]*=r,t[3]*=n,t[7]*=i,t[11]*=r,this}getMaxScaleOnAxis(){let e=this.elements,t=e[0]*e[0]+e[1]*e[1]+e[2]*e[2],n=e[4]*e[4]+e[5]*e[5]+e[6]*e[6],i=e[8]*e[8]+e[9]*e[9]+e[10]*e[10];return Math.sqrt(Math.max(t,n,i))}makeTranslation(e,t,n){return e.isVector3?this.set(1,0,0,e.x,0,1,0,e.y,0,0,1,e.z,0,0,0,1):this.set(1,0,0,e,0,1,0,t,0,0,1,n,0,0,0,1),this}makeRotationX(e){let t=Math.cos(e),n=Math.sin(e);return this.set(1,0,0,0,0,t,-n,0,0,n,t,0,0,0,0,1),this}makeRotationY(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,0,n,0,0,1,0,0,-n,0,t,0,0,0,0,1),this}makeRotationZ(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,0,n,t,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(e,t){let n=Math.cos(t),i=Math.sin(t),r=1-n,a=e.x,o=e.y,l=e.z,c=r*a,h=r*o;return this.set(c*a+n,c*o-i*l,c*l+i*o,0,c*o+i*l,h*o+n,h*l-i*a,0,c*l-i*o,h*l+i*a,r*l*l+n,0,0,0,0,1),this}makeScale(e,t,n){return this.set(e,0,0,0,0,t,0,0,0,0,n,0,0,0,0,1),this}makeShear(e,t,n,i,r,a){return this.set(1,n,r,0,e,1,a,0,t,i,1,0,0,0,0,1),this}compose(e,t,n){let i=this.elements,r=t._x,a=t._y,o=t._z,l=t._w,c=r+r,h=a+a,d=o+o,u=r*c,p=r*h,x=r*d,v=a*h,m=a*d,f=o*d,y=l*c,T=l*h,g=l*d,_=n.x,S=n.y,A=n.z;return i[0]=(1-(v+f))*_,i[1]=(p+g)*_,i[2]=(x-T)*_,i[3]=0,i[4]=(p-g)*S,i[5]=(1-(u+f))*S,i[6]=(m+y)*S,i[7]=0,i[8]=(x+T)*A,i[9]=(m-y)*A,i[10]=(1-(u+v))*A,i[11]=0,i[12]=e.x,i[13]=e.y,i[14]=e.z,i[15]=1,this}decompose(e,t,n){let i=this.elements;e.x=i[12],e.y=i[13],e.z=i[14];let r=this.determinantAffine();if(r===0)return n.set(1,1,1),t.identity(),this;let a=sr.set(i[0],i[1],i[2]).length(),o=sr.set(i[4],i[5],i[6]).length(),l=sr.set(i[8],i[9],i[10]).length();r<0&&(a=-a),ci.copy(this);let c=1/a,h=1/o,d=1/l;return ci.elements[0]*=c,ci.elements[1]*=c,ci.elements[2]*=c,ci.elements[4]*=h,ci.elements[5]*=h,ci.elements[6]*=h,ci.elements[8]*=d,ci.elements[9]*=d,ci.elements[10]*=d,t.setFromRotationMatrix(ci),n.x=a,n.y=o,n.z=l,this}makePerspective(e,t,n,i,r,a,o=fi,l=!1){let c=this.elements,h=2*r/(t-e),d=2*r/(n-i),u=(t+e)/(t-e),p=(n+i)/(n-i),x,v;if(l)x=r/(a-r),v=a*r/(a-r);else if(o===fi)x=-(a+r)/(a-r),v=-2*a*r/(a-r);else if(o===_r)x=-a/(a-r),v=-a*r/(a-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+o);return c[0]=h,c[4]=0,c[8]=u,c[12]=0,c[1]=0,c[5]=d,c[9]=p,c[13]=0,c[2]=0,c[6]=0,c[10]=x,c[14]=v,c[3]=0,c[7]=0,c[11]=-1,c[15]=0,this}makeOrthographic(e,t,n,i,r,a,o=fi,l=!1){let c=this.elements,h=2/(t-e),d=2/(n-i),u=-(t+e)/(t-e),p=-(n+i)/(n-i),x,v;if(l)x=1/(a-r),v=a/(a-r);else if(o===fi)x=-2/(a-r),v=-(a+r)/(a-r);else if(o===_r)x=-1/(a-r),v=-r/(a-r);else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+o);return c[0]=h,c[4]=0,c[8]=0,c[12]=u,c[1]=0,c[5]=d,c[9]=0,c[13]=p,c[2]=0,c[6]=0,c[10]=x,c[14]=v,c[3]=0,c[7]=0,c[11]=0,c[15]=1,this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<16;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<16;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e[t+9]=n[9],e[t+10]=n[10],e[t+11]=n[11],e[t+12]=n[12],e[t+13]=n[13],e[t+14]=n[14],e[t+15]=n[15],e}},sr=new k,ci=new mt,mg=new k(0,0,0),gg=new k(1,1,1),os=new k,wo=new k,Xn=new k,_d=new mt,yd=new $t,kn=class s{constructor(e=0,t=0,n=0,i=s.DEFAULT_ORDER){this.isEuler=!0,this._x=e,this._y=t,this._z=n,this._order=i}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get order(){return this._order}set order(e){this._order=e,this._onChangeCallback()}set(e,t,n,i=this._order){return this._x=e,this._y=t,this._z=n,this._order=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(e){return this._x=e._x,this._y=e._y,this._z=e._z,this._order=e._order,this._onChangeCallback(),this}setFromRotationMatrix(e,t=this._order,n=!0){let i=e.elements,r=i[0],a=i[4],o=i[8],l=i[1],c=i[5],h=i[9],d=i[2],u=i[6],p=i[10];switch(t){case"XYZ":this._y=Math.asin(Ct(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(-h,p),this._z=Math.atan2(-a,r)):(this._x=Math.atan2(u,c),this._z=0);break;case"YXZ":this._x=Math.asin(-Ct(h,-1,1)),Math.abs(h)<.9999999?(this._y=Math.atan2(o,p),this._z=Math.atan2(l,c)):(this._y=Math.atan2(-d,r),this._z=0);break;case"ZXY":this._x=Math.asin(Ct(u,-1,1)),Math.abs(u)<.9999999?(this._y=Math.atan2(-d,p),this._z=Math.atan2(-a,c)):(this._y=0,this._z=Math.atan2(l,r));break;case"ZYX":this._y=Math.asin(-Ct(d,-1,1)),Math.abs(d)<.9999999?(this._x=Math.atan2(u,p),this._z=Math.atan2(l,r)):(this._x=0,this._z=Math.atan2(-a,c));break;case"YZX":this._z=Math.asin(Ct(l,-1,1)),Math.abs(l)<.9999999?(this._x=Math.atan2(-h,c),this._y=Math.atan2(-d,r)):(this._x=0,this._y=Math.atan2(o,p));break;case"XZY":this._z=Math.asin(-Ct(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(u,c),this._y=Math.atan2(o,r)):(this._x=Math.atan2(-h,p),this._y=0);break;default:ht("Euler: .setFromRotationMatrix() encountered an unknown order: "+t)}return this._order=t,n===!0&&this._onChangeCallback(),this}setFromQuaternion(e,t,n){return _d.makeRotationFromQuaternion(e),this.setFromRotationMatrix(_d,t,n)}setFromVector3(e,t=this._order){return this.set(e.x,e.y,e.z,t)}reorder(e){return yd.setFromEuler(this),this.setFromQuaternion(yd,e)}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._order===this._order}fromArray(e){return this._x=e[0],this._y=e[1],this._z=e[2],e[3]!==void 0&&(this._order=e[3]),this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._order,e}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};kn.DEFAULT_ORDER="XYZ";var Sr=class{constructor(){this.mask=1}set(e){this.mask=(1<<e|0)>>>0}enable(e){this.mask|=1<<e|0}enableAll(){this.mask=-1}toggle(e){this.mask^=1<<e|0}disable(e){this.mask&=~(1<<e|0)}disableAll(){this.mask=0}test(e){return(this.mask&e.mask)!==0}isEnabled(e){return(this.mask&(1<<e|0))!==0}},xg=0,Md=new k,rr=new $t,Hi=new mt,So=new k,aa=new k,bg=new k,vg=new $t,wd=new k(1,0,0),Sd=new k(0,1,0),Ed=new k(0,0,1),Td={type:"added"},_g={type:"removed"},ar={type:"childadded",child:null},Qc={type:"childremoved",child:null},Xt=class s extends ni{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:xg++}),this.uuid=pi(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=s.DEFAULT_UP.clone();let e=new k,t=new kn,n=new $t,i=new k(1,1,1);function r(){n.setFromEuler(t,!1)}function a(){t.setFromQuaternion(n,void 0,!1)}t._onChange(r),n._onChange(a),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:e},rotation:{configurable:!0,enumerable:!0,value:t},quaternion:{configurable:!0,enumerable:!0,value:n},scale:{configurable:!0,enumerable:!0,value:i},modelViewMatrix:{value:new mt},normalMatrix:{value:new _t}}),this.matrix=new mt,this.matrixWorld=new mt,this.matrixAutoUpdate=s.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=s.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new Sr,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.customDepthMaterial=void 0,this.customDistanceMaterial=void 0,this.static=!1,this.userData={},this.pivot=null}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(e){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(e),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(e){return this.quaternion.premultiply(e),this}setRotationFromAxisAngle(e,t){this.quaternion.setFromAxisAngle(e,t)}setRotationFromEuler(e){this.quaternion.setFromEuler(e,!0)}setRotationFromMatrix(e){this.quaternion.setFromRotationMatrix(e)}setRotationFromQuaternion(e){this.quaternion.copy(e)}rotateOnAxis(e,t){return rr.setFromAxisAngle(e,t),this.quaternion.multiply(rr),this}rotateOnWorldAxis(e,t){return rr.setFromAxisAngle(e,t),this.quaternion.premultiply(rr),this}rotateX(e){return this.rotateOnAxis(wd,e)}rotateY(e){return this.rotateOnAxis(Sd,e)}rotateZ(e){return this.rotateOnAxis(Ed,e)}translateOnAxis(e,t){return Md.copy(e).applyQuaternion(this.quaternion),this.position.add(Md.multiplyScalar(t)),this}translateX(e){return this.translateOnAxis(wd,e)}translateY(e){return this.translateOnAxis(Sd,e)}translateZ(e){return this.translateOnAxis(Ed,e)}localToWorld(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(this.matrixWorld)}worldToLocal(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(Hi.copy(this.matrixWorld).invert())}lookAt(e,t,n){e.isVector3?So.copy(e):So.set(e,t,n);let i=this.parent;this.updateWorldMatrix(!0,!1),aa.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?Hi.lookAt(aa,So,this.up):Hi.lookAt(So,aa,this.up),this.quaternion.setFromRotationMatrix(Hi),i&&(Hi.extractRotation(i.matrixWorld),rr.setFromRotationMatrix(Hi),this.quaternion.premultiply(rr.invert()))}add(e){if(arguments.length>1){for(let t=0;t<arguments.length;t++)this.add(arguments[t]);return this}return e===this?(gt("Object3D.add: object can't be added as a child of itself.",e),this):(e&&e.isObject3D?(e.removeFromParent(),e.parent=this,this.children.push(e),e.dispatchEvent(Td),ar.child=e,this.dispatchEvent(ar),ar.child=null):gt("Object3D.add: object not an instance of THREE.Object3D.",e),this)}remove(e){if(arguments.length>1){for(let n=0;n<arguments.length;n++)this.remove(arguments[n]);return this}let t=this.children.indexOf(e);return t!==-1&&(e.parent=null,this.children.splice(t,1),e.dispatchEvent(_g),Qc.child=e,this.dispatchEvent(Qc),Qc.child=null),this}removeFromParent(){let e=this.parent;return e!==null&&e.remove(this),this}clear(){return this.remove(...this.children)}attach(e){return this.updateWorldMatrix(!0,!1),Hi.copy(this.matrixWorld).invert(),e.parent!==null&&(e.parent.updateWorldMatrix(!0,!1),Hi.multiply(e.parent.matrixWorld)),e.applyMatrix4(Hi),e.removeFromParent(),e.parent=this,this.children.push(e),e.updateWorldMatrix(!1,!0),e.dispatchEvent(Td),ar.child=e,this.dispatchEvent(ar),ar.child=null,this}getObjectById(e){return this.getObjectByProperty("id",e)}getObjectByName(e){return this.getObjectByProperty("name",e)}getObjectByProperty(e,t){if(this[e]===t)return this;for(let n=0,i=this.children.length;n<i;n++){let a=this.children[n].getObjectByProperty(e,t);if(a!==void 0)return a}}getObjectsByProperty(e,t,n=[]){this[e]===t&&n.push(this);let i=this.children;for(let r=0,a=i.length;r<a;r++)i[r].getObjectsByProperty(e,t,n);return n}getWorldPosition(e){return this.updateWorldMatrix(!0,!1),e.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(aa,e,bg),e}getWorldScale(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(aa,vg,e),e}getWorldDirection(e){this.updateWorldMatrix(!0,!1);let t=this.matrixWorld.elements;return e.set(t[8],t[9],t[10]).normalize()}raycast(){}intersectsFrustum(){}traverse(e){e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverse(e)}traverseVisible(e){if(this.visible===!1)return;e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverseVisible(e)}traverseAncestors(e){let t=this.parent;t!==null&&(e(t),t.traverseAncestors(e))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale);let e=this.pivot;if(e!==null){let t=e.x,n=e.y,i=e.z,r=this.matrix.elements;r[12]+=t-r[0]*t-r[4]*n-r[8]*i,r[13]+=n-r[1]*t-r[5]*n-r[9]*i,r[14]+=i-r[2]*t-r[6]*n-r[10]*i}this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(e){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||e)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,e=!0);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].updateMatrixWorld(e)}updateWorldMatrix(e,t,n=!1){let i=this.parent;if(e===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||n)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,n=!0),t===!0){let r=this.children;for(let a=0,o=r.length;a<o;a++)r[a].updateWorldMatrix(!1,!0,n)}}toJSON(e){let t=e===void 0||typeof e=="string",n={};t&&(e={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},n.metadata={version:4.7,type:"Object",generator:"Object3D.toJSON"});let i={};i.uuid=this.uuid,i.type=this.type,i.name=this.name,i.castShadow=this.castShadow,i.receiveShadow=this.receiveShadow,i.visible=this.visible,i.frustumCulled=this.frustumCulled,i.renderOrder=this.renderOrder,i.static=this.static,i.matrixAutoUpdate=this.matrixAutoUpdate,Object.keys(this.userData).length>0&&(i.userData=this.userData),i.layers=this.layers.mask,i.matrix=this.matrix.toArray(),i.up=this.up.toArray(),this.pivot!==null&&(i.pivot=this.pivot.toArray()),this.morphTargetDictionary!==void 0&&(i.morphTargetDictionary=Object.assign({},this.morphTargetDictionary)),this.morphTargetInfluences!==void 0&&(i.morphTargetInfluences=this.morphTargetInfluences.slice()),this.isInstancedMesh&&(i.type="InstancedMesh",i.count=this.count,i.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(i.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(i.type="BatchedMesh",i.perObjectFrustumCulled=this.perObjectFrustumCulled,i.sortObjects=this.sortObjects,i.drawRanges=this._drawRanges,i.reservedRanges=this._reservedRanges,i.geometryInfo=this._geometryInfo.map(o=>({...o,boundingBox:o.boundingBox?o.boundingBox.toJSON():void 0,boundingSphere:o.boundingSphere?o.boundingSphere.toJSON():void 0})),i.instanceInfo=this._instanceInfo.map(o=>({...o})),i.availableInstanceIds=this._availableInstanceIds.slice(),i.availableGeometryIds=this._availableGeometryIds.slice(),i.nextIndexStart=this._nextIndexStart,i.nextVertexStart=this._nextVertexStart,i.geometryCount=this._geometryCount,i.maxInstanceCount=this._maxInstanceCount,i.maxVertexCount=this._maxVertexCount,i.maxIndexCount=this._maxIndexCount,i.geometryInitialized=this._geometryInitialized,i.matricesTexture=this._matricesTexture.toJSON(e),i.indirectTexture=this._indirectTexture.toJSON(e),this._colorsTexture!==null&&(i.colorsTexture=this._colorsTexture.toJSON(e)),this.boundingSphere!==null&&(i.boundingSphere=this.boundingSphere.toJSON()),this.boundingBox!==null&&(i.boundingBox=this.boundingBox.toJSON()));function r(o,l){return o[l.uuid]===void 0&&(o[l.uuid]=l.toJSON(e)),l.uuid}if(this.isScene)this.background&&(this.background.isColor?i.background=this.background.toJSON():this.background.isTexture&&(i.background=this.background.toJSON(e).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(i.environment=this.environment.toJSON(e).uuid);else if(this.isMesh||this.isLine||this.isPoints){i.geometry=r(e.geometries,this.geometry);let o=this.geometry.parameters;if(o!==void 0&&o.shapes!==void 0){let l=o.shapes;if(Array.isArray(l))for(let c=0,h=l.length;c<h;c++){let d=l[c];r(e.shapes,d)}else r(e.shapes,l)}}if(this.isSkinnedMesh&&(i.bindMode=this.bindMode,i.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(e.skeletons,this.skeleton),i.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let o=[];for(let l=0,c=this.material.length;l<c;l++)o.push(r(e.materials,this.material[l]));i.material=o}else i.material=r(e.materials,this.material);if(this.children.length>0){i.children=[];for(let o=0;o<this.children.length;o++)i.children.push(this.children[o].toJSON(e).object)}if(this.animations.length>0){i.animations=[];for(let o=0;o<this.animations.length;o++){let l=this.animations[o];i.animations.push(r(e.animations,l))}}if(t){let o=a(e.geometries),l=a(e.materials),c=a(e.textures),h=a(e.images),d=a(e.shapes),u=a(e.skeletons),p=a(e.animations),x=a(e.nodes);o.length>0&&(n.geometries=o),l.length>0&&(n.materials=l),c.length>0&&(n.textures=c),h.length>0&&(n.images=h),d.length>0&&(n.shapes=d),u.length>0&&(n.skeletons=u),p.length>0&&(n.animations=p),x.length>0&&(n.nodes=x)}return n.object=i,n;function a(o){let l=[];for(let c in o){let h=o[c];delete h.metadata,l.push(h)}return l}}clone(e){return new this.constructor().copy(this,e)}copy(e,t=!0){if(this.name=e.name,this.up.copy(e.up),this.position.copy(e.position),this.rotation.order=e.rotation.order,this.quaternion.copy(e.quaternion),this.scale.copy(e.scale),this.pivot=e.pivot!==null?e.pivot.clone():null,this.matrix.copy(e.matrix),this.matrixWorld.copy(e.matrixWorld),this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrixWorldAutoUpdate=e.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=e.matrixWorldNeedsUpdate,this.layers.mask=e.layers.mask,this.visible=e.visible,this.castShadow=e.castShadow,this.receiveShadow=e.receiveShadow,this.frustumCulled=e.frustumCulled,this.renderOrder=e.renderOrder,this.static=e.static,this.animations=e.animations.slice(),this.userData=JSON.parse(JSON.stringify(e.userData)),t===!0)for(let n=0;n<e.children.length;n++){let i=e.children[n];this.add(i.clone())}return this}dispose(){this.dispatchEvent({type:"dispose"})}};Xt.DEFAULT_UP=new k(0,1,0);Xt.DEFAULT_MATRIX_AUTO_UPDATE=!0;Xt.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var yt=class extends Xt{constructor(){super(),this.isGroup=!0,this.type="Group"}},yg={type:"move"},Er=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new yt,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new yt,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new k,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new k),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new yt,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new k,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new k,this._grip.eventsEnabled=!1),this._grip}dispatchEvent(e){return this._targetRay!==null&&this._targetRay.dispatchEvent(e),this._grip!==null&&this._grip.dispatchEvent(e),this._hand!==null&&this._hand.dispatchEvent(e),this}connect(e){if(e&&e.hand){let t=this._hand;if(t)for(let n of e.hand.values())this._getHandJoint(t,n)}return this.dispatchEvent({type:"connected",data:e}),this}disconnect(e){return this.dispatchEvent({type:"disconnected",data:e}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(e,t,n){let i=null,r=null,a=null,o=this._targetRay,l=this._grip,c=this._hand;if(e&&t.session.visibilityState!=="visible-blurred"){if(c&&e.hand){a=!0;for(let v of e.hand.values()){let m=t.getJointPose(v,n),f=this._getHandJoint(c,v);m!==null&&(f.matrix.fromArray(m.transform.matrix),f.matrix.decompose(f.position,f.rotation,f.scale),f.matrixWorldNeedsUpdate=!0,f.jointRadius=m.radius),f.visible=m!==null}let h=c.joints["index-finger-tip"],d=c.joints["thumb-tip"],u=h.position.distanceTo(d.position),p=.02,x=.005;c.inputState.pinching&&u>p+x?(c.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:e.handedness,target:this})):!c.inputState.pinching&&u<=p-x&&(c.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:e.handedness,target:this}))}else l!==null&&e.gripSpace&&(r=t.getPose(e.gripSpace,n),r!==null&&(l.matrix.fromArray(r.transform.matrix),l.matrix.decompose(l.position,l.rotation,l.scale),l.matrixWorldNeedsUpdate=!0,r.linearVelocity?(l.hasLinearVelocity=!0,l.linearVelocity.copy(r.linearVelocity)):l.hasLinearVelocity=!1,r.angularVelocity?(l.hasAngularVelocity=!0,l.angularVelocity.copy(r.angularVelocity)):l.hasAngularVelocity=!1,l.eventsEnabled&&l.dispatchEvent({type:"gripUpdated",data:e,target:this})));o!==null&&(i=t.getPose(e.targetRaySpace,n),i===null&&r!==null&&(i=r),i!==null&&(o.matrix.fromArray(i.transform.matrix),o.matrix.decompose(o.position,o.rotation,o.scale),o.matrixWorldNeedsUpdate=!0,i.linearVelocity?(o.hasLinearVelocity=!0,o.linearVelocity.copy(i.linearVelocity)):o.hasLinearVelocity=!1,i.angularVelocity?(o.hasAngularVelocity=!0,o.angularVelocity.copy(i.angularVelocity)):o.hasAngularVelocity=!1,this.dispatchEvent(yg)))}return o!==null&&(o.visible=i!==null),l!==null&&(l.visible=r!==null),c!==null&&(c.visible=a!==null),this}_getHandJoint(e,t){if(e.joints[t.jointName]===void 0){let n=new yt;n.matrixAutoUpdate=!1,n.visible=!1,e.joints[t.jointName]=n,e.add(n)}return e.joints[t.jointName]}},qf={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},ls={h:0,s:0,l:0},Eo={h:0,s:0,l:0};function $c(s,e,t){return t<0&&(t+=1),t>1&&(t-=1),t<1/6?s+(e-s)*6*t:t<1/2?e:t<2/3?s+(e-s)*6*(2/3-t):s}var Ce=class{constructor(e,t,n){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(e,t,n)}set(e,t,n){if(t===void 0&&n===void 0){let i=e;i&&i.isColor?this.copy(i):typeof i=="number"?this.setHex(i):typeof i=="string"&&this.setStyle(i)}else this.setRGB(e,t,n);return this}setScalar(e){return this.r=e,this.g=e,this.b=e,this}setHex(e,t=sn){return e=Math.floor(e),this.r=(e>>16&255)/255,this.g=(e>>8&255)/255,this.b=(e&255)/255,Rt.colorSpaceToWorking(this,t),this}setRGB(e,t,n,i=Rt.workingColorSpace){return this.r=e,this.g=t,this.b=n,Rt.colorSpaceToWorking(this,i),this}setHSL(e,t,n,i=Rt.workingColorSpace){if(e=jh(e,1),t=Ct(t,0,1),n=Ct(n,0,1),t===0)this.r=this.g=this.b=n;else{let r=n<=.5?n*(1+t):n+t-n*t,a=2*n-r;this.r=$c(a,r,e+1/3),this.g=$c(a,r,e),this.b=$c(a,r,e-1/3)}return Rt.colorSpaceToWorking(this,i),this}setStyle(e,t=sn){function n(r){r!==void 0&&parseFloat(r)<1&&ht("Color: Alpha component of "+e+" will be ignored.")}let i;if(i=/^(\w+)\(([^\)]*)\)/.exec(e)){let r,a=i[1],o=i[2];switch(a){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,t);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,t);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,t);break;default:ht("Color: Unknown color model "+e)}}else if(i=/^\#([A-Fa-f\d]+)$/.exec(e)){let r=i[1],a=r.length;if(a===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,t);if(a===6)return this.setHex(parseInt(r,16),t);ht("Color: Invalid hex color "+e)}else if(e&&e.length>0)return this.setColorName(e,t);return this}setColorName(e,t=sn){let n=qf[e.toLowerCase()];return n!==void 0?this.setHex(n,t):ht("Color: Unknown color "+e),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(e){return this.r=e.r,this.g=e.g,this.b=e.b,this}copySRGBToLinear(e){return this.r=ji(e.r),this.g=ji(e.g),this.b=ji(e.b),this}copyLinearToSRGB(e){return this.r=xr(e.r),this.g=xr(e.g),this.b=xr(e.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(e=sn){return Rt.workingToColorSpace(Pn.copy(this),e),Math.round(Ct(Pn.r*255,0,255))*65536+Math.round(Ct(Pn.g*255,0,255))*256+Math.round(Ct(Pn.b*255,0,255))}getHexString(e=sn){return("000000"+this.getHex(e).toString(16)).slice(-6)}getHSL(e,t=Rt.workingColorSpace){Rt.workingToColorSpace(Pn.copy(this),t);let n=Pn.r,i=Pn.g,r=Pn.b,a=Math.max(n,i,r),o=Math.min(n,i,r),l,c,h=(o+a)/2;if(o===a)l=0,c=0;else{let d=a-o;switch(c=h<=.5?d/(a+o):d/(2-a-o),a){case n:l=(i-r)/d+(i<r?6:0);break;case i:l=(r-n)/d+2;break;case r:l=(n-i)/d+4;break}l/=6}return e.h=l,e.s=c,e.l=h,e}getRGB(e,t=Rt.workingColorSpace){return Rt.workingToColorSpace(Pn.copy(this),t),e.r=Pn.r,e.g=Pn.g,e.b=Pn.b,e}getStyle(e=sn){Rt.workingToColorSpace(Pn.copy(this),e);let t=Pn.r,n=Pn.g,i=Pn.b;return e!==sn?`color(${e} ${t.toFixed(3)} ${n.toFixed(3)} ${i.toFixed(3)})`:`rgb(${Math.round(t*255)},${Math.round(n*255)},${Math.round(i*255)})`}offsetHSL(e,t,n){return this.getHSL(ls),this.setHSL(ls.h+e,ls.s+t,ls.l+n)}add(e){return this.r+=e.r,this.g+=e.g,this.b+=e.b,this}addColors(e,t){return this.r=e.r+t.r,this.g=e.g+t.g,this.b=e.b+t.b,this}addScalar(e){return this.r+=e,this.g+=e,this.b+=e,this}sub(e){return this.r=Math.max(0,this.r-e.r),this.g=Math.max(0,this.g-e.g),this.b=Math.max(0,this.b-e.b),this}multiply(e){return this.r*=e.r,this.g*=e.g,this.b*=e.b,this}multiplyScalar(e){return this.r*=e,this.g*=e,this.b*=e,this}lerp(e,t){return this.r+=(e.r-this.r)*t,this.g+=(e.g-this.g)*t,this.b+=(e.b-this.b)*t,this}lerpColors(e,t,n){return this.r=e.r+(t.r-e.r)*n,this.g=e.g+(t.g-e.g)*n,this.b=e.b+(t.b-e.b)*n,this}lerpHSL(e,t){this.getHSL(ls),e.getHSL(Eo);let n=ma(ls.h,Eo.h,t),i=ma(ls.s,Eo.s,t),r=ma(ls.l,Eo.l,t);return this.setHSL(n,i,r),this}setFromVector3(e){return this.r=e.x,this.g=e.y,this.b=e.z,this}applyMatrix3(e){let t=this.r,n=this.g,i=this.b,r=e.elements;return this.r=r[0]*t+r[3]*n+r[6]*i,this.g=r[1]*t+r[4]*n+r[7]*i,this.b=r[2]*t+r[5]*n+r[8]*i,this}equals(e){return e.r===this.r&&e.g===this.g&&e.b===this.b}fromArray(e,t=0){return this.r=e[t],this.g=e[t+1],this.b=e[t+2],this}toArray(e=[],t=0){return e[t]=this.r,e[t+1]=this.g,e[t+2]=this.b,e}fromBufferAttribute(e,t){return this.r=e.getX(t),this.g=e.getY(t),this.b=e.getZ(t),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},Pn=new Ce;Ce.NAMES=qf;var Ma=class s{constructor(e,t=25e-5){this.isFogExp2=!0,this.name="",this.color=new Ce(e),this.density=t}clone(){return new s(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var ks=class extends Xt{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new kn,this.environmentIntensity=1,this.environmentRotation=new kn,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(e,t){return super.copy(e,t),e.background!==null&&(this.background=e.background.clone()),e.environment!==null&&(this.environment=e.environment.clone()),e.fog!==null&&(this.fog=e.fog.clone()),this.backgroundBlurriness=e.backgroundBlurriness,this.backgroundIntensity=e.backgroundIntensity,this.backgroundRotation.copy(e.backgroundRotation),this.environmentIntensity=e.environmentIntensity,this.environmentRotation.copy(e.environmentRotation),e.overrideMaterial!==null&&(this.overrideMaterial=e.overrideMaterial.clone()),this.matrixAutoUpdate=e.matrixAutoUpdate,this}toJSON(e){let t=super.toJSON(e);return this.fog!==null&&(t.object.fog=this.fog.toJSON()),t.object.backgroundBlurriness=this.backgroundBlurriness,t.object.backgroundIntensity=this.backgroundIntensity,t.object.backgroundRotation=this.backgroundRotation.toArray(),t.object.environmentIntensity=this.environmentIntensity,t.object.environmentRotation=this.environmentRotation.toArray(),t}},hi=new k,Gi=new k,eh=new k,Vi=new k,or=new k,lr=new k,Ad=new k,th=new k,nh=new k,ih=new k,sh=new Nt,rh=new Nt,ah=new Nt,fs=class s{constructor(e=new k,t=new k,n=new k){this.a=e,this.b=t,this.c=n}static getNormal(e,t,n,i){i.subVectors(n,t),hi.subVectors(e,t),i.cross(hi);let r=i.lengthSq();return r>0?i.multiplyScalar(1/Math.sqrt(r)):i.set(0,0,0)}static getBarycoord(e,t,n,i,r){hi.subVectors(i,t),Gi.subVectors(n,t),eh.subVectors(e,t);let a=hi.dot(hi),o=hi.dot(Gi),l=hi.dot(eh),c=Gi.dot(Gi),h=Gi.dot(eh),d=a*c-o*o;if(d===0)return r.set(0,0,0),null;let u=1/d,p=(c*l-o*h)*u,x=(a*h-o*l)*u;return r.set(1-p-x,x,p)}static containsPoint(e,t,n,i){return this.getBarycoord(e,t,n,i,Vi)===null?!1:Vi.x>=0&&Vi.y>=0&&Vi.x+Vi.y<=1}static getInterpolation(e,t,n,i,r,a,o,l){return this.getBarycoord(e,t,n,i,Vi)===null?(l.x=0,l.y=0,"z"in l&&(l.z=0),"w"in l&&(l.w=0),null):(l.setScalar(0),l.addScaledVector(r,Vi.x),l.addScaledVector(a,Vi.y),l.addScaledVector(o,Vi.z),l)}static getInterpolatedAttribute(e,t,n,i,r,a){return sh.setScalar(0),rh.setScalar(0),ah.setScalar(0),sh.fromBufferAttribute(e,t),rh.fromBufferAttribute(e,n),ah.fromBufferAttribute(e,i),a.setScalar(0),a.addScaledVector(sh,r.x),a.addScaledVector(rh,r.y),a.addScaledVector(ah,r.z),a}static isFrontFacing(e,t,n,i){return hi.subVectors(n,t),Gi.subVectors(e,t),hi.cross(Gi).dot(i)<0}set(e,t,n){return this.a.copy(e),this.b.copy(t),this.c.copy(n),this}setFromPointsAndIndices(e,t,n,i){return this.a.copy(e[t]),this.b.copy(e[n]),this.c.copy(e[i]),this}setFromAttributeAndIndices(e,t,n,i){return this.a.fromBufferAttribute(e,t),this.b.fromBufferAttribute(e,n),this.c.fromBufferAttribute(e,i),this}clone(){return new this.constructor().copy(this)}copy(e){return this.a.copy(e.a),this.b.copy(e.b),this.c.copy(e.c),this}getArea(){return hi.subVectors(this.c,this.b),Gi.subVectors(this.a,this.b),hi.cross(Gi).length()*.5}getMidpoint(e){return e.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(e){return s.getNormal(this.a,this.b,this.c,e)}getPlane(e){return e.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(e,t){return s.getBarycoord(e,this.a,this.b,this.c,t)}getInterpolation(e,t,n,i,r){return s.getInterpolation(e,this.a,this.b,this.c,t,n,i,r)}containsPoint(e){return s.containsPoint(e,this.a,this.b,this.c)}isFrontFacing(e){return s.isFrontFacing(this.a,this.b,this.c,e)}intersectsBox(e){return e.intersectsTriangle(this)}closestPointToPoint(e,t){let n=this.a,i=this.b,r=this.c,a,o;or.subVectors(i,n),lr.subVectors(r,n),th.subVectors(e,n);let l=or.dot(th),c=lr.dot(th);if(l<=0&&c<=0)return t.copy(n);nh.subVectors(e,i);let h=or.dot(nh),d=lr.dot(nh);if(h>=0&&d<=h)return t.copy(i);let u=l*d-h*c;if(u<=0&&l>=0&&h<=0)return a=l/(l-h),t.copy(n).addScaledVector(or,a);ih.subVectors(e,r);let p=or.dot(ih),x=lr.dot(ih);if(x>=0&&p<=x)return t.copy(r);let v=p*c-l*x;if(v<=0&&c>=0&&x<=0)return o=c/(c-x),t.copy(n).addScaledVector(lr,o);let m=h*x-p*d;if(m<=0&&d-h>=0&&p-x>=0)return Ad.subVectors(r,i),o=(d-h)/(d-h+(p-x)),t.copy(i).addScaledVector(Ad,o);let f=1/(m+v+u);return a=v*f,o=u*f,t.copy(n).addScaledVector(or,a).addScaledVector(lr,o)}equals(e){return e.a.equals(this.a)&&e.b.equals(this.b)&&e.c.equals(this.c)}},hn=class{constructor(e=new k(1/0,1/0,1/0),t=new k(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=e,this.max=t}set(e,t){return this.min.copy(e),this.max.copy(t),this}setFromArray(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t+=3)this.expandByPoint(ui.fromArray(e,t));return this}setFromBufferAttribute(e){this.makeEmpty();for(let t=0,n=e.count;t<n;t++)this.expandByPoint(ui.fromBufferAttribute(e,t));return this}setFromPoints(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t++)this.expandByPoint(e[t]);return this}setFromCenterAndSize(e,t){let n=ui.copy(t).multiplyScalar(.5);return this.min.copy(e).sub(n),this.max.copy(e).add(n),this}setFromObject(e,t=!1){return this.makeEmpty(),this.expandByObject(e,t)}clone(){return new this.constructor().copy(this)}copy(e){return this.min.copy(e.min),this.max.copy(e.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(e){return this.isEmpty()?e.set(0,0,0):e.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(e){return this.isEmpty()?e.set(0,0,0):e.subVectors(this.max,this.min)}expandByPoint(e){return this.min.min(e),this.max.max(e),this}expandByVector(e){return this.min.sub(e),this.max.add(e),this}expandByScalar(e){return this.min.addScalar(-e),this.max.addScalar(e),this}expandByObject(e,t=!1){e.updateWorldMatrix(!1,!1);let n=e.geometry;if(n!==void 0){let r=n.getAttribute("position");if(t===!0&&r!==void 0&&e.isInstancedMesh!==!0)for(let a=0,o=r.count;a<o;a++)e.isMesh===!0?e.getVertexPosition(a,ui):ui.fromBufferAttribute(r,a),ui.applyMatrix4(e.matrixWorld),this.expandByPoint(ui);else e.boundingBox!==void 0?(e.boundingBox===null&&e.computeBoundingBox(),To.copy(e.boundingBox)):(n.boundingBox===null&&n.computeBoundingBox(),To.copy(n.boundingBox)),To.applyMatrix4(e.matrixWorld),this.union(To)}let i=e.children;for(let r=0,a=i.length;r<a;r++)this.expandByObject(i[r],t);return this}containsPoint(e){return e.x>=this.min.x&&e.x<=this.max.x&&e.y>=this.min.y&&e.y<=this.max.y&&e.z>=this.min.z&&e.z<=this.max.z}containsBox(e){return this.min.x<=e.min.x&&e.max.x<=this.max.x&&this.min.y<=e.min.y&&e.max.y<=this.max.y&&this.min.z<=e.min.z&&e.max.z<=this.max.z}getParameter(e,t){return t.set((e.x-this.min.x)/(this.max.x-this.min.x),(e.y-this.min.y)/(this.max.y-this.min.y),(e.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(e){return e.max.x>=this.min.x&&e.min.x<=this.max.x&&e.max.y>=this.min.y&&e.min.y<=this.max.y&&e.max.z>=this.min.z&&e.min.z<=this.max.z}intersectsSphere(e){return this.clampPoint(e.center,ui),ui.distanceToSquared(e.center)<=e.radius*e.radius}intersectsPlane(e){let t,n;return e.normal.x>0?(t=e.normal.x*this.min.x,n=e.normal.x*this.max.x):(t=e.normal.x*this.max.x,n=e.normal.x*this.min.x),e.normal.y>0?(t+=e.normal.y*this.min.y,n+=e.normal.y*this.max.y):(t+=e.normal.y*this.max.y,n+=e.normal.y*this.min.y),e.normal.z>0?(t+=e.normal.z*this.min.z,n+=e.normal.z*this.max.z):(t+=e.normal.z*this.max.z,n+=e.normal.z*this.min.z),t<=-e.constant&&n>=-e.constant}intersectsTriangle(e){if(this.isEmpty())return!1;this.getCenter(oa),Ao.subVectors(this.max,oa),cr.subVectors(e.a,oa),hr.subVectors(e.b,oa),ur.subVectors(e.c,oa),cs.subVectors(hr,cr),hs.subVectors(ur,hr),Cs.subVectors(cr,ur);let t=[0,-cs.z,cs.y,0,-hs.z,hs.y,0,-Cs.z,Cs.y,cs.z,0,-cs.x,hs.z,0,-hs.x,Cs.z,0,-Cs.x,-cs.y,cs.x,0,-hs.y,hs.x,0,-Cs.y,Cs.x,0];return!oh(t,cr,hr,ur,Ao)||(t=[1,0,0,0,1,0,0,0,1],!oh(t,cr,hr,ur,Ao))?!1:(Ro.crossVectors(cs,hs),t=[Ro.x,Ro.y,Ro.z],oh(t,cr,hr,ur,Ao))}clampPoint(e,t){return t.copy(e).clamp(this.min,this.max)}distanceToPoint(e){return this.clampPoint(e,ui).distanceTo(e)}getBoundingSphere(e){return this.isEmpty()?e.makeEmpty():(this.getCenter(e.center),e.radius=this.getSize(ui).length()*.5),e}intersect(e){return this.min.max(e.min),this.max.min(e.max),this.isEmpty()&&this.makeEmpty(),this}union(e){return this.min.min(e.min),this.max.max(e.max),this}applyMatrix4(e){return this.isEmpty()?this:(Wi[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(e),Wi[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(e),Wi[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(e),Wi[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(e),Wi[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(e),Wi[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(e),Wi[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(e),Wi[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(e),this.setFromPoints(Wi),this)}translate(e){return this.min.add(e),this.max.add(e),this}equals(e){return e.min.equals(this.min)&&e.max.equals(this.max)}toJSON(){return{min:this.min.toArray(),max:this.max.toArray()}}fromJSON(e){return this.min.fromArray(e.min),this.max.fromArray(e.max),this}},Wi=[new k,new k,new k,new k,new k,new k,new k,new k],ui=new k,To=new hn,cr=new k,hr=new k,ur=new k,cs=new k,hs=new k,Cs=new k,oa=new k,Ao=new k,Ro=new k,Ps=new k;function oh(s,e,t,n,i){for(let r=0,a=s.length-3;r<=a;r+=3){Ps.fromArray(s,r);let o=i.x*Math.abs(Ps.x)+i.y*Math.abs(Ps.y)+i.z*Math.abs(Ps.z),l=e.dot(Ps),c=t.dot(Ps),h=n.dot(Ps);if(Math.max(-Math.max(l,c,h),Math.min(l,c,h))>o)return!1}return!0}var pn=new k,Co=new ze,Mg=0,Tt=class extends ni{constructor(e,t,n=!1){if(super(),Array.isArray(e))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,Object.defineProperty(this,"id",{value:Mg++}),this.name="",this.array=e,this.itemSize=t,this.count=e!==void 0?e.length/t:0,this.normalized=n,this.usage=qh,this.updateRanges=[],this.gpuType=Zn,this.version=0}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.name=e.name,this.array=new e.array.constructor(e.array),this.itemSize=e.itemSize,this.count=e.count,this.normalized=e.normalized,this.usage=e.usage,this.gpuType=e.gpuType,this}copyAt(e,t,n){e*=this.itemSize,n*=t.itemSize;for(let i=0,r=this.itemSize;i<r;i++)this.array[e+i]=t.array[n+i];return this}copyArray(e){return this.array.set(e),this}applyMatrix3(e){if(this.itemSize===2)for(let t=0,n=this.count;t<n;t++)Co.fromBufferAttribute(this,t),Co.applyMatrix3(e),this.setXY(t,Co.x,Co.y);else if(this.itemSize===3)for(let t=0,n=this.count;t<n;t++)pn.fromBufferAttribute(this,t),pn.applyMatrix3(e),this.setXYZ(t,pn.x,pn.y,pn.z);return this}applyMatrix4(e){for(let t=0,n=this.count;t<n;t++)pn.fromBufferAttribute(this,t),pn.applyMatrix4(e),this.setXYZ(t,pn.x,pn.y,pn.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)pn.fromBufferAttribute(this,t),pn.applyNormalMatrix(e),this.setXYZ(t,pn.x,pn.y,pn.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)pn.fromBufferAttribute(this,t),pn.transformDirection(e),this.setXYZ(t,pn.x,pn.y,pn.z);return this}set(e,t=0){return this.array.set(e,t),this}getComponent(e,t){let n=this.array[e*this.itemSize+t];return this.normalized&&(n=di(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=Kt(n,this.array)),this.array[e*this.itemSize+t]=n,this}getX(e){let t=this.array[e*this.itemSize];return this.normalized&&(t=di(t,this.array)),t}setX(e,t){return this.normalized&&(t=Kt(t,this.array)),this.array[e*this.itemSize]=t,this}getY(e){let t=this.array[e*this.itemSize+1];return this.normalized&&(t=di(t,this.array)),t}setY(e,t){return this.normalized&&(t=Kt(t,this.array)),this.array[e*this.itemSize+1]=t,this}getZ(e){let t=this.array[e*this.itemSize+2];return this.normalized&&(t=di(t,this.array)),t}setZ(e,t){return this.normalized&&(t=Kt(t,this.array)),this.array[e*this.itemSize+2]=t,this}getW(e){let t=this.array[e*this.itemSize+3];return this.normalized&&(t=di(t,this.array)),t}setW(e,t){return this.normalized&&(t=Kt(t,this.array)),this.array[e*this.itemSize+3]=t,this}setXY(e,t,n){return e*=this.itemSize,this.normalized&&(t=Kt(t,this.array),n=Kt(n,this.array)),this.array[e+0]=t,this.array[e+1]=n,this}setXYZ(e,t,n,i){return e*=this.itemSize,this.normalized&&(t=Kt(t,this.array),n=Kt(n,this.array),i=Kt(i,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e*=this.itemSize,this.normalized&&(t=Kt(t,this.array),n=Kt(n,this.array),i=Kt(i,this.array),r=Kt(r,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this.array[e+3]=r,this}onUpload(e){return this.onUploadCallback=e,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let e={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return e.name=this.name,e.usage=this.usage,e.gpuType=this.gpuType,e}dispose(){this.dispatchEvent({type:"dispose"})}};var wa=class extends Tt{constructor(e,t,n){super(new Uint16Array(e),t,n)}};var Sa=class extends Tt{constructor(e,t,n){super(new Uint32Array(e),t,n)}};var xt=class extends Tt{constructor(e,t,n){super(new Float32Array(e),t,n)}},wg=new hn,la=new k,lh=new k,In=class{constructor(e=new k,t=-1){this.isSphere=!0,this.center=e,this.radius=t}set(e,t){return this.center.copy(e),this.radius=t,this}setFromPoints(e,t){let n=this.center;t!==void 0?n.copy(t):wg.setFromPoints(e).getCenter(n);let i=0;for(let r=0,a=e.length;r<a;r++)i=Math.max(i,n.distanceToSquared(e[r]));return this.radius=Math.sqrt(i),this}copy(e){return this.center.copy(e.center),this.radius=e.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(e){return e.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(e){return e.distanceTo(this.center)-this.radius}intersectsSphere(e){let t=this.radius+e.radius;return e.center.distanceToSquared(this.center)<=t*t}intersectsBox(e){return e.intersectsSphere(this)}intersectsPlane(e){return Math.abs(e.distanceToPoint(this.center))<=this.radius}clampPoint(e,t){let n=this.center.distanceToSquared(e);return t.copy(e),n>this.radius*this.radius&&(t.sub(this.center).normalize(),t.multiplyScalar(this.radius).add(this.center)),t}getBoundingBox(e){return this.isEmpty()?(e.makeEmpty(),e):(e.set(this.center,this.center),e.expandByScalar(this.radius),e)}applyMatrix4(e){return this.center.applyMatrix4(e),this.radius=this.radius*e.getMaxScaleOnAxis(),this}translate(e){return this.center.add(e),this}expandByPoint(e){if(this.isEmpty())return this.center.copy(e),this.radius=0,this;la.subVectors(e,this.center);let t=la.lengthSq();if(t>this.radius*this.radius){let n=Math.sqrt(t),i=(n-this.radius)*.5;this.center.addScaledVector(la,i/n),this.radius+=i}return this}union(e){return e.isEmpty()?this:this.isEmpty()?(this.copy(e),this):(this.center.equals(e.center)===!0?this.radius=Math.max(this.radius,e.radius):(lh.subVectors(e.center,this.center).setLength(e.radius),this.expandByPoint(la.copy(e.center).add(lh)),this.expandByPoint(la.copy(e.center).sub(lh))),this)}equals(e){return e.center.equals(this.center)&&e.radius===this.radius}clone(){return new this.constructor().copy(this)}toJSON(){return{radius:this.radius,center:this.center.toArray()}}fromJSON(e){return this.radius=e.radius,this.center.fromArray(e.center),this}},Sg=0,ei=new mt,ch=new Xt,dr=new k,jn=new hn,ca=new hn,Sn=new k,Mt=class s extends ni{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:Sg++}),this.uuid=pi(),this.name="",this.type="BufferGeometry",this.index=null,this.indirect=null,this.indirectOffset=0,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={},this._transformed=!1}getIndex(){return this.index}setIndex(e){return Array.isArray(e)?this.index=new(Km(e)?Sa:wa)(e,1):this.index=e,this}setIndirect(e,t=0){return this.indirect=e,this.indirectOffset=t,this}getIndirect(){return this.indirect}getAttribute(e){return this.attributes[e]}setAttribute(e,t){return this.attributes[e]=t,this}deleteAttribute(e){return delete this.attributes[e],this}hasAttribute(e){return this.attributes[e]!==void 0}addGroup(e,t,n=0){this.groups.push({start:e,count:t,materialIndex:n})}clearGroups(){this.groups=[]}setDrawRange(e,t){this.drawRange.start=e,this.drawRange.count=t}applyMatrix4(e){let t=this.attributes.position;t!==void 0&&(t.applyMatrix4(e),t.needsUpdate=!0);let n=this.attributes.normal;if(n!==void 0){let r=new _t().getNormalMatrix(e);n.applyNormalMatrix(r),n.needsUpdate=!0}let i=this.attributes.tangent;return i!==void 0&&(i.transformDirection(e),i.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this._transformed=!0,this}applyQuaternion(e){return ei.makeRotationFromQuaternion(e),this.applyMatrix4(ei),this}rotateX(e){return ei.makeRotationX(e),this.applyMatrix4(ei),this}rotateY(e){return ei.makeRotationY(e),this.applyMatrix4(ei),this}rotateZ(e){return ei.makeRotationZ(e),this.applyMatrix4(ei),this}translate(e,t,n){return ei.makeTranslation(e,t,n),this.applyMatrix4(ei),this}scale(e,t,n){return ei.makeScale(e,t,n),this.applyMatrix4(ei),this}lookAt(e){return ch.lookAt(e),ch.updateMatrix(),this.applyMatrix4(ch.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(dr).negate(),this.translate(dr.x,dr.y,dr.z),this}setFromPoints(e){let t=this.getAttribute("position");if(t===void 0){let n=[];for(let i=0,r=e.length;i<r;i++){let a=e[i];n.push(a.x,a.y,a.z||0)}this.setAttribute("position",new xt(n,3))}else{let n=Math.min(e.length,t.count);for(let i=0;i<n;i++){let r=e[i];t.setXYZ(i,r.x,r.y,r.z||0)}e.length>t.count&&ht("BufferGeometry: Buffer size too small for points data. Use .dispose() and create a new geometry."),t.needsUpdate=!0}return this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new hn);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){gt("BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new k(-1/0,-1/0,-1/0),new k(1/0,1/0,1/0));return}if(e!==void 0){if(this.boundingBox.setFromBufferAttribute(e),t)for(let n=0,i=t.length;n<i;n++){let r=t[n];jn.setFromBufferAttribute(r),this.morphTargetsRelative?(Sn.addVectors(this.boundingBox.min,jn.min),this.boundingBox.expandByPoint(Sn),Sn.addVectors(this.boundingBox.max,jn.max),this.boundingBox.expandByPoint(Sn)):(this.boundingBox.expandByPoint(jn.min),this.boundingBox.expandByPoint(jn.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&gt('BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new In);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){gt("BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new k,1/0);return}if(e){let n=this.boundingSphere.center;if(jn.setFromBufferAttribute(e),t)for(let r=0,a=t.length;r<a;r++){let o=t[r];ca.setFromBufferAttribute(o),this.morphTargetsRelative?(Sn.addVectors(jn.min,ca.min),jn.expandByPoint(Sn),Sn.addVectors(jn.max,ca.max),jn.expandByPoint(Sn)):(jn.expandByPoint(ca.min),jn.expandByPoint(ca.max))}jn.getCenter(n);let i=0;for(let r=0,a=e.count;r<a;r++)Sn.fromBufferAttribute(e,r),i=Math.max(i,n.distanceToSquared(Sn));if(t)for(let r=0,a=t.length;r<a;r++){let o=t[r],l=this.morphTargetsRelative;for(let c=0,h=o.count;c<h;c++)Sn.fromBufferAttribute(o,c),l&&(dr.fromBufferAttribute(e,c),Sn.add(dr)),i=Math.max(i,n.distanceToSquared(Sn))}this.boundingSphere.radius=Math.sqrt(i),isNaN(this.boundingSphere.radius)&&gt('BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let e=this.index,t=this.attributes;if(e===null||t.position===void 0||t.normal===void 0||t.uv===void 0){gt("BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let n=t.position,i=t.normal,r=t.uv,a=this.getAttribute("tangent");(a===void 0||a.count!==n.count)&&(a=new Tt(new Float32Array(4*n.count),4),this.setAttribute("tangent",a));let o=[],l=[];for(let b=0;b<n.count;b++)o[b]=new k,l[b]=new k;let c=new k,h=new k,d=new k,u=new ze,p=new ze,x=new ze,v=new k,m=new k;function f(b,C,w){c.fromBufferAttribute(n,b),h.fromBufferAttribute(n,C),d.fromBufferAttribute(n,w),u.fromBufferAttribute(r,b),p.fromBufferAttribute(r,C),x.fromBufferAttribute(r,w),h.sub(c),d.sub(c),p.sub(u),x.sub(u);let I=1/(p.x*x.y-x.x*p.y);isFinite(I)&&(v.copy(h).multiplyScalar(x.y).addScaledVector(d,-p.y).multiplyScalar(I),m.copy(d).multiplyScalar(p.x).addScaledVector(h,-x.x).multiplyScalar(I),o[b].add(v),o[C].add(v),o[w].add(v),l[b].add(m),l[C].add(m),l[w].add(m))}let y=this.groups;y.length===0&&(y=[{start:0,count:e.count}]);for(let b=0,C=y.length;b<C;++b){let w=y[b],I=w.start,M=w.count;for(let N=I,D=I+M;N<D;N+=3)f(e.getX(N+0),e.getX(N+1),e.getX(N+2))}let T=new k,g=new k,_=new k,S=new k;function A(b){_.fromBufferAttribute(i,b),S.copy(_);let C=o[b];T.copy(C),T.sub(_.multiplyScalar(_.dot(C))).normalize(),g.crossVectors(S,C);let I=g.dot(l[b])<0?-1:1;a.setXYZW(b,T.x,T.y,T.z,I)}for(let b=0,C=y.length;b<C;++b){let w=y[b],I=w.start,M=w.count;for(let N=I,D=I+M;N<D;N+=3)A(e.getX(N+0)),A(e.getX(N+1)),A(e.getX(N+2))}this._transformed=!0}computeVertexNormals(){let e=this.index,t=this.getAttribute("position");if(t!==void 0){let n=this.getAttribute("normal");if(n===void 0||n.count!==t.count)n=new Tt(new Float32Array(t.count*3),3),this.setAttribute("normal",n);else for(let u=0,p=n.count;u<p;u++)n.setXYZ(u,0,0,0);let i=new k,r=new k,a=new k,o=new k,l=new k,c=new k,h=new k,d=new k;if(e)for(let u=0,p=e.count;u<p;u+=3){let x=e.getX(u+0),v=e.getX(u+1),m=e.getX(u+2);i.fromBufferAttribute(t,x),r.fromBufferAttribute(t,v),a.fromBufferAttribute(t,m),h.subVectors(a,r),d.subVectors(i,r),h.cross(d),o.fromBufferAttribute(n,x),l.fromBufferAttribute(n,v),c.fromBufferAttribute(n,m),o.add(h),l.add(h),c.add(h),n.setXYZ(x,o.x,o.y,o.z),n.setXYZ(v,l.x,l.y,l.z),n.setXYZ(m,c.x,c.y,c.z)}else for(let u=0,p=t.count;u<p;u+=3)i.fromBufferAttribute(t,u+0),r.fromBufferAttribute(t,u+1),a.fromBufferAttribute(t,u+2),h.subVectors(a,r),d.subVectors(i,r),h.cross(d),n.setXYZ(u+0,h.x,h.y,h.z),n.setXYZ(u+1,h.x,h.y,h.z),n.setXYZ(u+2,h.x,h.y,h.z);this.normalizeNormals(),n.needsUpdate=!0}}normalizeNormals(){let e=this.attributes.normal;for(let t=0,n=e.count;t<n;t++)Sn.fromBufferAttribute(e,t),Sn.normalize(),e.setXYZ(t,Sn.x,Sn.y,Sn.z)}toNonIndexed(){function e(o,l){let c=o.array,h=o.itemSize,d=o.normalized,u=new c.constructor(l.length*h),p=0,x=0;for(let v=0,m=l.length;v<m;v++){o.isInterleavedBufferAttribute?p=l[v]*o.data.stride+o.offset:p=l[v]*h;for(let f=0;f<h;f++)u[x++]=c[p++]}return new Tt(u,h,d)}if(this.index===null)return ht("BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let t=new s,n=this.index.array,i=this.attributes;for(let o in i){let l=i[o],c=e(l,n);t.setAttribute(o,c)}let r=this.morphAttributes;for(let o in r){let l=[],c=r[o];for(let h=0,d=c.length;h<d;h++){let u=c[h],p=e(u,n);l.push(p)}t.morphAttributes[o]=l}t.morphTargetsRelative=this.morphTargetsRelative;let a=this.groups;for(let o=0,l=a.length;o<l;o++){let c=a[o];t.addGroup(c.start,c.count,c.materialIndex)}return t}toJSON(){let e={metadata:{version:4.7,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(e.uuid=this.uuid,e.type=this.parameters!==void 0&&this._transformed===!0?"BufferGeometry":this.type,e.name=this.name,Object.keys(this.userData).length>0&&(e.userData=this.userData),this.parameters!==void 0&&this._transformed!==!0){let l=this.parameters;for(let c in l)l[c]!==void 0&&(e[c]=l[c]);return e}e.data={attributes:{}};let t=this.index;t!==null&&(e.data.index={type:t.array.constructor.name,array:Array.prototype.slice.call(t.array)});let n=this.attributes;for(let l in n){let c=n[l];e.data.attributes[l]=c.toJSON(e.data)}let i={},r=!1;for(let l in this.morphAttributes){let c=this.morphAttributes[l],h=[];for(let d=0,u=c.length;d<u;d++){let p=c[d];h.push(p.toJSON(e.data))}h.length>0&&(i[l]=h,r=!0)}r&&(e.data.morphAttributes=i,e.data.morphTargetsRelative=this.morphTargetsRelative);let a=this.groups;a.length>0&&(e.data.groups=JSON.parse(JSON.stringify(a)));let o=this.boundingSphere;return o!==null&&(e.data.boundingSphere=o.toJSON()),e}clone(){return new this.constructor().copy(this)}copy(e){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let t={};this.name=e.name;let n=e.index;n!==null&&this.setIndex(n.clone());let i=e.attributes;for(let c in i){let h=i[c];this.setAttribute(c,h.clone(t))}let r=e.morphAttributes;for(let c in r){let h=[],d=r[c];for(let u=0,p=d.length;u<p;u++)h.push(d[u].clone(t));this.morphAttributes[c]=h}this.morphTargetsRelative=e.morphTargetsRelative;let a=e.groups;for(let c=0,h=a.length;c<h;c++){let d=a[c];this.addGroup(d.start,d.count,d.materialIndex)}let o=e.boundingBox;o!==null&&(this.boundingBox=o.clone());let l=e.boundingSphere;return l!==null&&(this.boundingSphere=l.clone()),this.drawRange.start=e.drawRange.start,this.drawRange.count=e.drawRange.count,this.userData=e.userData,this._transformed=e._transformed,this}dispose(){this.dispatchEvent({type:"dispose"})}},Tr=class{constructor(e,t){this.isInterleavedBuffer=!0,this.array=e,this.stride=t,this.count=e!==void 0?e.length/t:0,this.usage=qh,this.updateRanges=[],this.version=0,this.uuid=pi()}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.array=new e.array.constructor(e.array),this.count=e.count,this.stride=e.stride,this.usage=e.usage,this}copyAt(e,t,n){e*=this.stride,n*=t.stride;for(let i=0,r=this.stride;i<r;i++)this.array[e+i]=t.array[n+i];return this}set(e,t=0){return this.array.set(e,t),this}clone(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=pi()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=this.array.slice(0).buffer);let t=new this.array.constructor(e.arrayBuffers[this.array.buffer._uuid]),n=new this.constructor(t,this.stride);return n.setUsage(this.usage),n}onUpload(e){return this.onUploadCallback=e,this}toJSON(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=pi()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=Array.from(new Uint32Array(this.array.buffer)));let t={uuid:this.uuid,buffer:this.array.buffer._uuid,type:this.array.constructor.name,stride:this.stride};return t.usage=this.usage,t}},Un=new k,Ar=class s{constructor(e,t,n,i=!1){this.isInterleavedBufferAttribute=!0,this.name="",this.data=e,this.itemSize=t,this.offset=n,this.normalized=i}get count(){return this.data.count}get array(){return this.data.array}set needsUpdate(e){this.data.needsUpdate=e}applyMatrix4(e){for(let t=0,n=this.data.count;t<n;t++)Un.fromBufferAttribute(this,t),Un.applyMatrix4(e),this.setXYZ(t,Un.x,Un.y,Un.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)Un.fromBufferAttribute(this,t),Un.applyNormalMatrix(e),this.setXYZ(t,Un.x,Un.y,Un.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)Un.fromBufferAttribute(this,t),Un.transformDirection(e),this.setXYZ(t,Un.x,Un.y,Un.z);return this}getComponent(e,t){let n=this.array[e*this.data.stride+this.offset+t];return this.normalized&&(n=di(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=Kt(n,this.array)),this.data.array[e*this.data.stride+this.offset+t]=n,this}setX(e,t){return this.normalized&&(t=Kt(t,this.array)),this.data.array[e*this.data.stride+this.offset]=t,this}setY(e,t){return this.normalized&&(t=Kt(t,this.array)),this.data.array[e*this.data.stride+this.offset+1]=t,this}setZ(e,t){return this.normalized&&(t=Kt(t,this.array)),this.data.array[e*this.data.stride+this.offset+2]=t,this}setW(e,t){return this.normalized&&(t=Kt(t,this.array)),this.data.array[e*this.data.stride+this.offset+3]=t,this}getX(e){let t=this.data.array[e*this.data.stride+this.offset];return this.normalized&&(t=di(t,this.array)),t}getY(e){let t=this.data.array[e*this.data.stride+this.offset+1];return this.normalized&&(t=di(t,this.array)),t}getZ(e){let t=this.data.array[e*this.data.stride+this.offset+2];return this.normalized&&(t=di(t,this.array)),t}getW(e){let t=this.data.array[e*this.data.stride+this.offset+3];return this.normalized&&(t=di(t,this.array)),t}setXY(e,t,n){return e=e*this.data.stride+this.offset,this.normalized&&(t=Kt(t,this.array),n=Kt(n,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this}setXYZ(e,t,n,i){return e=e*this.data.stride+this.offset,this.normalized&&(t=Kt(t,this.array),n=Kt(n,this.array),i=Kt(i,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e=e*this.data.stride+this.offset,this.normalized&&(t=Kt(t,this.array),n=Kt(n,this.array),i=Kt(i,this.array),r=Kt(r,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this.data.array[e+3]=r,this}clone(e){if(e===void 0){_a("InterleavedBufferAttribute.clone(): Cloning an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return new Tt(new this.array.constructor(t),this.itemSize,this.normalized)}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.clone(e)),new s(e.interleavedBuffers[this.data.uuid],this.itemSize,this.offset,this.normalized)}toJSON(e){if(e===void 0){_a("InterleavedBufferAttribute.toJSON(): Serializing an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return{itemSize:this.itemSize,type:this.array.constructor.name,array:t,normalized:this.normalized}}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.toJSON(e)),{isInterleavedBufferAttribute:!0,itemSize:this.itemSize,data:this.data.uuid,offset:this.offset,normalized:this.normalized}}},hh=new k,Eg=new k,Tg=new _t,Kn=class{constructor(e=new k(1,0,0),t=0){this.isPlane=!0,this.normal=e,this.constant=t}set(e,t){return this.normal.copy(e),this.constant=t,this}setComponents(e,t,n,i){return this.normal.set(e,t,n),this.constant=i,this}setFromNormalAndCoplanarPoint(e,t){return this.normal.copy(e),this.constant=-t.dot(this.normal),this}setFromCoplanarPoints(e,t,n){let i=hh.subVectors(n,t).cross(Eg.subVectors(e,t)).normalize();return this.setFromNormalAndCoplanarPoint(i,e),this}copy(e){return this.normal.copy(e.normal),this.constant=e.constant,this}normalize(){let e=1/this.normal.length();return this.normal.multiplyScalar(e),this.constant*=e,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(e){return this.normal.dot(e)+this.constant}distanceToSphere(e){return this.distanceToPoint(e.center)-e.radius}projectPoint(e,t){return t.copy(e).addScaledVector(this.normal,-this.distanceToPoint(e))}intersectLine(e,t,n=!0){let i=e.delta(hh),r=this.normal.dot(i);if(r===0)return this.distanceToPoint(e.start)===0?t.copy(e.start):null;let a=-(e.start.dot(this.normal)+this.constant)/r;return n===!0&&(a<0||a>1)?null:t.copy(e.start).addScaledVector(i,a)}intersectsLine(e){let t=this.distanceToPoint(e.start),n=this.distanceToPoint(e.end);return t<0&&n>0||n<0&&t>0}intersectsBox(e){return e.intersectsPlane(this)}intersectsSphere(e){return e.intersectsPlane(this)}coplanarPoint(e){return e.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(e,t){let n=t||Tg.getNormalMatrix(e),i=this.coplanarPoint(hh).applyMatrix4(e),r=this.normal.applyMatrix3(n).normalize();return this.constant=-i.dot(r),this}translate(e){return this.constant-=e.dot(this.normal),this}equals(e){return e.normal.equals(this.normal)&&e.constant===this.constant}clone(){return new this.constructor().copy(this)}toJSON(){return{normal:this.normal.toArray(),constant:this.constant}}fromJSON(e){return this.normal.fromArray(e.normal),this.constant=e.constant,this}},Ag=0,zn=class extends ni{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:Ag++}),this.uuid=pi(),this.name="",this.type="Material",this.blending=Br,this.side=ii,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=Nh,this.blendDst=Uh,this.blendEquation=Xs,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new Ce(0,0,0),this.blendAlpha=0,this.depthFunc=br,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=Ff,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=Yo,this.stencilZFail=Yo,this.stencilZPass=Yo,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.allowOverride=!0,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(e){this._alphaTest>0!=e>0&&this.version++,this._alphaTest=e}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(e){if(e!==void 0)for(let t in e){let n=e[t];if(n===void 0){ht(`Material: parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){ht(`Material: '${t}' is not a property of THREE.${this.type}.`);continue}i&&i.isColor?i.set(n):i&&i.isVector2&&n&&n.isVector2||i&&i.isEuler&&n&&n.isEuler||i&&i.isVector3&&n&&n.isVector3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";t&&(e={textures:{},images:{}});let n={metadata:{version:4.7,type:"Material",generator:"Material.toJSON"}};n.uuid=this.uuid,n.type=this.type,n.blending=this.blending,n.side=this.side,n.shadowSide=this.shadowSide,n.vertexColors=this.vertexColors,n.opacity=this.opacity,n.transparent=this.transparent,n.blendSrc=this.blendSrc,n.blendDst=this.blendDst,n.blendEquation=this.blendEquation,n.blendSrcAlpha=this.blendSrcAlpha,n.blendDstAlpha=this.blendDstAlpha,n.blendEquationAlpha=this.blendEquationAlpha,n.blendColor=this.blendColor.getHex(),n.blendAlpha=this.blendAlpha,n.depthFunc=this.depthFunc,n.depthTest=this.depthTest,n.depthWrite=this.depthWrite,n.colorWrite=this.colorWrite,n.clipIntersection=this.clipIntersection,n.clipShadows=this.clipShadows,n.stencilWriteMask=this.stencilWriteMask,n.stencilFunc=this.stencilFunc,n.stencilRef=this.stencilRef,n.stencilFuncMask=this.stencilFuncMask,n.stencilFail=this.stencilFail,n.stencilZFail=this.stencilZFail,n.stencilZPass=this.stencilZPass,n.stencilWrite=this.stencilWrite,n.polygonOffset=this.polygonOffset,n.polygonOffsetFactor=this.polygonOffsetFactor,n.polygonOffsetUnits=this.polygonOffsetUnits,n.dithering=this.dithering,n.alphaTest=this.alphaTest,n.alphaHash=this.alphaHash,n.alphaToCoverage=this.alphaToCoverage,n.premultipliedAlpha=this.premultipliedAlpha,n.forceSinglePass=this.forceSinglePass,n.allowOverride=this.allowOverride,n.visible=this.visible,n.toneMapped=this.toneMapped,n.name=this.name,this.color&&this.color.isColor&&(n.color=this.color.getHex()),this.roughness!==void 0&&(n.roughness=this.roughness),this.metalness!==void 0&&(n.metalness=this.metalness),this.sheen!==void 0&&(n.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(n.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(n.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(n.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&(n.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(n.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(n.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(n.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(n.shininess=this.shininess),this.clearcoat!==void 0&&(n.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(n.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(n.clearcoatMap=this.clearcoatMap.toJSON(e).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(n.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(e).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(n.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(e).uuid,n.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.sheenColorMap&&this.sheenColorMap.isTexture&&(n.sheenColorMap=this.sheenColorMap.toJSON(e).uuid),this.sheenRoughnessMap&&this.sheenRoughnessMap.isTexture&&(n.sheenRoughnessMap=this.sheenRoughnessMap.toJSON(e).uuid),this.dispersion!==void 0&&(n.dispersion=this.dispersion),this.retroreflectivity!==void 0&&(n.retroreflectivity=this.retroreflectivity),this.iridescence!==void 0&&(n.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(n.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(n.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(n.iridescenceMap=this.iridescenceMap.toJSON(e).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(n.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(e).uuid),this.anisotropy!==void 0&&(n.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(n.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(n.anisotropyMap=this.anisotropyMap.toJSON(e).uuid),this.map&&this.map.isTexture&&(n.map=this.map.toJSON(e).uuid),this.matcap&&this.matcap.isTexture&&(n.matcap=this.matcap.toJSON(e).uuid),this.alphaMap&&this.alphaMap.isTexture&&(n.alphaMap=this.alphaMap.toJSON(e).uuid),this.lightMap&&this.lightMap.isTexture&&(n.lightMap=this.lightMap.toJSON(e).uuid,n.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(n.aoMap=this.aoMap.toJSON(e).uuid,n.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(n.bumpMap=this.bumpMap.toJSON(e).uuid,n.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(n.normalMap=this.normalMap.toJSON(e).uuid,n.normalMapType=this.normalMapType,n.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(n.displacementMap=this.displacementMap.toJSON(e).uuid,n.displacementScale=this.displacementScale,n.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(n.roughnessMap=this.roughnessMap.toJSON(e).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(n.metalnessMap=this.metalnessMap.toJSON(e).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(n.emissiveMap=this.emissiveMap.toJSON(e).uuid),this.specularMap&&this.specularMap.isTexture&&(n.specularMap=this.specularMap.toJSON(e).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(n.specularIntensityMap=this.specularIntensityMap.toJSON(e).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(n.specularColorMap=this.specularColorMap.toJSON(e).uuid),this.envMap&&this.envMap.isTexture&&(n.envMap=this.envMap.toJSON(e).uuid,this.combine!==void 0&&(n.combine=this.combine)),this.envMapRotation!==void 0&&(n.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(n.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(n.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(n.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(n.gradientMap=this.gradientMap.toJSON(e).uuid),this.transmission!==void 0&&(n.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(n.transmissionMap=this.transmissionMap.toJSON(e).uuid),this.thickness!==void 0&&(n.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(n.thicknessMap=this.thicknessMap.toJSON(e).uuid),this.attenuationDistance!==void 0&&(n.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(n.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(n.size=this.size),this.sizeAttenuation!==void 0&&(n.sizeAttenuation=this.sizeAttenuation),Array.isArray(this.clippingPlanes)&&this.clippingPlanes.length>0&&(n.clippingPlanes=this.clippingPlanes.map(r=>r.toJSON())),this.rotation!==void 0&&(n.rotation=this.rotation),this.depthPacking!==void 0&&(n.depthPacking=this.depthPacking),this.linewidth!==void 0&&(n.linewidth=this.linewidth),this.linecap!==void 0&&(n.linecap=this.linecap),this.linejoin!==void 0&&(n.linejoin=this.linejoin),this.dashSize!==void 0&&(n.dashSize=this.dashSize),this.gapSize!==void 0&&(n.gapSize=this.gapSize),this.scale!==void 0&&(n.scale=this.scale),this.wireframe!==void 0&&(n.wireframe=this.wireframe),this.wireframeLinewidth!==void 0&&(n.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!==void 0&&(n.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!==void 0&&(n.wireframeLinejoin=this.wireframeLinejoin),this.flatShading!==void 0&&(n.flatShading=this.flatShading),this.fog!==void 0&&(n.fog=this.fog),Object.keys(this.userData).length>0&&(n.userData=this.userData);function i(r){let a=[];for(let o in r){let l=r[o];delete l.metadata,a.push(l)}return a}if(t){let r=i(e.textures),a=i(e.images);r.length>0&&(n.textures=r),a.length>0&&(n.images=a)}return n}fromJSON(e,t){if(e.uuid!==void 0&&(this.uuid=e.uuid),e.name!==void 0&&(this.name=e.name),e.color!==void 0&&this.color!==void 0&&this.color.setHex(e.color),e.roughness!==void 0&&(this.roughness=e.roughness),e.metalness!==void 0&&(this.metalness=e.metalness),e.sheen!==void 0&&(this.sheen=e.sheen),e.sheenColor!==void 0&&(this.sheenColor=new Ce().setHex(e.sheenColor)),e.sheenRoughness!==void 0&&(this.sheenRoughness=e.sheenRoughness),e.emissive!==void 0&&this.emissive!==void 0&&this.emissive.setHex(e.emissive),e.specular!==void 0&&this.specular!==void 0&&this.specular.setHex(e.specular),e.specularIntensity!==void 0&&(this.specularIntensity=e.specularIntensity),e.specularColor!==void 0&&this.specularColor!==void 0&&this.specularColor.setHex(e.specularColor),e.shininess!==void 0&&(this.shininess=e.shininess),e.clearcoat!==void 0&&(this.clearcoat=e.clearcoat),e.clearcoatRoughness!==void 0&&(this.clearcoatRoughness=e.clearcoatRoughness),e.dispersion!==void 0&&(this.dispersion=e.dispersion),e.retroreflectivity!==void 0&&(this.retroreflectivity=e.retroreflectivity),e.iridescence!==void 0&&(this.iridescence=e.iridescence),e.iridescenceIOR!==void 0&&(this.iridescenceIOR=e.iridescenceIOR),e.iridescenceThicknessRange!==void 0&&(this.iridescenceThicknessRange=e.iridescenceThicknessRange),e.transmission!==void 0&&(this.transmission=e.transmission),e.thickness!==void 0&&(this.thickness=e.thickness),e.attenuationDistance!==void 0&&(this.attenuationDistance=e.attenuationDistance),e.attenuationColor!==void 0&&this.attenuationColor!==void 0&&this.attenuationColor.setHex(e.attenuationColor),e.anisotropy!==void 0&&(this.anisotropy=e.anisotropy),e.anisotropyRotation!==void 0&&(this.anisotropyRotation=e.anisotropyRotation),e.fog!==void 0&&(this.fog=e.fog),e.flatShading!==void 0&&(this.flatShading=e.flatShading),e.blending!==void 0&&(this.blending=e.blending),e.combine!==void 0&&(this.combine=e.combine),e.side!==void 0&&(this.side=e.side),e.shadowSide!==void 0&&(this.shadowSide=e.shadowSide),e.opacity!==void 0&&(this.opacity=e.opacity),e.transparent!==void 0&&(this.transparent=e.transparent),e.alphaTest!==void 0&&(this.alphaTest=e.alphaTest),e.alphaHash!==void 0&&(this.alphaHash=e.alphaHash),e.depthFunc!==void 0&&(this.depthFunc=e.depthFunc),e.depthTest!==void 0&&(this.depthTest=e.depthTest),e.depthWrite!==void 0&&(this.depthWrite=e.depthWrite),e.colorWrite!==void 0&&(this.colorWrite=e.colorWrite),e.clippingPlanes!==void 0&&(this.clippingPlanes=e.clippingPlanes.map(n=>new Kn().fromJSON(n))),e.clipIntersection!==void 0&&(this.clipIntersection=e.clipIntersection),e.clipShadows!==void 0&&(this.clipShadows=e.clipShadows),e.depthPacking!==void 0&&(this.depthPacking=e.depthPacking),e.blendSrc!==void 0&&(this.blendSrc=e.blendSrc),e.blendDst!==void 0&&(this.blendDst=e.blendDst),e.blendEquation!==void 0&&(this.blendEquation=e.blendEquation),e.blendSrcAlpha!==void 0&&(this.blendSrcAlpha=e.blendSrcAlpha),e.blendDstAlpha!==void 0&&(this.blendDstAlpha=e.blendDstAlpha),e.blendEquationAlpha!==void 0&&(this.blendEquationAlpha=e.blendEquationAlpha),e.blendColor!==void 0&&this.blendColor!==void 0&&this.blendColor.setHex(e.blendColor),e.blendAlpha!==void 0&&(this.blendAlpha=e.blendAlpha),e.stencilWriteMask!==void 0&&(this.stencilWriteMask=e.stencilWriteMask),e.stencilFunc!==void 0&&(this.stencilFunc=e.stencilFunc),e.stencilRef!==void 0&&(this.stencilRef=e.stencilRef),e.stencilFuncMask!==void 0&&(this.stencilFuncMask=e.stencilFuncMask),e.stencilFail!==void 0&&(this.stencilFail=e.stencilFail),e.stencilZFail!==void 0&&(this.stencilZFail=e.stencilZFail),e.stencilZPass!==void 0&&(this.stencilZPass=e.stencilZPass),e.stencilWrite!==void 0&&(this.stencilWrite=e.stencilWrite),e.wireframe!==void 0&&(this.wireframe=e.wireframe),e.wireframeLinewidth!==void 0&&(this.wireframeLinewidth=e.wireframeLinewidth),e.wireframeLinecap!==void 0&&(this.wireframeLinecap=e.wireframeLinecap),e.wireframeLinejoin!==void 0&&(this.wireframeLinejoin=e.wireframeLinejoin),e.rotation!==void 0&&(this.rotation=e.rotation),e.linewidth!==void 0&&(this.linewidth=e.linewidth),e.linecap!==void 0&&(this.linecap=e.linecap),e.linejoin!==void 0&&(this.linejoin=e.linejoin),e.dashSize!==void 0&&(this.dashSize=e.dashSize),e.gapSize!==void 0&&(this.gapSize=e.gapSize),e.scale!==void 0&&(this.scale=e.scale),e.polygonOffset!==void 0&&(this.polygonOffset=e.polygonOffset),e.polygonOffsetFactor!==void 0&&(this.polygonOffsetFactor=e.polygonOffsetFactor),e.polygonOffsetUnits!==void 0&&(this.polygonOffsetUnits=e.polygonOffsetUnits),e.dithering!==void 0&&(this.dithering=e.dithering),e.alphaToCoverage!==void 0&&(this.alphaToCoverage=e.alphaToCoverage),e.premultipliedAlpha!==void 0&&(this.premultipliedAlpha=e.premultipliedAlpha),e.forceSinglePass!==void 0&&(this.forceSinglePass=e.forceSinglePass),e.allowOverride!==void 0&&(this.allowOverride=e.allowOverride),e.visible!==void 0&&(this.visible=e.visible),e.toneMapped!==void 0&&(this.toneMapped=e.toneMapped),e.userData!==void 0&&(this.userData=e.userData),e.vertexColors!==void 0&&(typeof e.vertexColors=="number"?this.vertexColors=e.vertexColors>0:this.vertexColors=e.vertexColors),e.size!==void 0&&(this.size=e.size),e.sizeAttenuation!==void 0&&(this.sizeAttenuation=e.sizeAttenuation),e.map!==void 0&&(this.map=t[e.map]||null),e.matcap!==void 0&&(this.matcap=t[e.matcap]||null),e.alphaMap!==void 0&&(this.alphaMap=t[e.alphaMap]||null),e.bumpMap!==void 0&&(this.bumpMap=t[e.bumpMap]||null),e.bumpScale!==void 0&&(this.bumpScale=e.bumpScale),e.normalMap!==void 0&&(this.normalMap=t[e.normalMap]||null),e.normalMapType!==void 0&&(this.normalMapType=e.normalMapType),e.normalScale!==void 0){let n=e.normalScale;Array.isArray(n)===!1&&(n=[n,n]),this.normalScale=new ze().fromArray(n)}return e.displacementMap!==void 0&&(this.displacementMap=t[e.displacementMap]||null),e.displacementScale!==void 0&&(this.displacementScale=e.displacementScale),e.displacementBias!==void 0&&(this.displacementBias=e.displacementBias),e.roughnessMap!==void 0&&(this.roughnessMap=t[e.roughnessMap]||null),e.metalnessMap!==void 0&&(this.metalnessMap=t[e.metalnessMap]||null),e.emissiveMap!==void 0&&(this.emissiveMap=t[e.emissiveMap]||null),e.emissiveIntensity!==void 0&&(this.emissiveIntensity=e.emissiveIntensity),e.specularMap!==void 0&&(this.specularMap=t[e.specularMap]||null),e.specularIntensityMap!==void 0&&(this.specularIntensityMap=t[e.specularIntensityMap]||null),e.specularColorMap!==void 0&&(this.specularColorMap=t[e.specularColorMap]||null),e.envMap!==void 0&&(this.envMap=t[e.envMap]||null),e.envMapRotation!==void 0&&this.envMapRotation.fromArray(e.envMapRotation),e.envMapIntensity!==void 0&&(this.envMapIntensity=e.envMapIntensity),e.reflectivity!==void 0&&(this.reflectivity=e.reflectivity),e.refractionRatio!==void 0&&(this.refractionRatio=e.refractionRatio),e.lightMap!==void 0&&(this.lightMap=t[e.lightMap]||null),e.lightMapIntensity!==void 0&&(this.lightMapIntensity=e.lightMapIntensity),e.aoMap!==void 0&&(this.aoMap=t[e.aoMap]||null),e.aoMapIntensity!==void 0&&(this.aoMapIntensity=e.aoMapIntensity),e.gradientMap!==void 0&&(this.gradientMap=t[e.gradientMap]||null),e.clearcoatMap!==void 0&&(this.clearcoatMap=t[e.clearcoatMap]||null),e.clearcoatRoughnessMap!==void 0&&(this.clearcoatRoughnessMap=t[e.clearcoatRoughnessMap]||null),e.clearcoatNormalMap!==void 0&&(this.clearcoatNormalMap=t[e.clearcoatNormalMap]||null),e.clearcoatNormalScale!==void 0&&(this.clearcoatNormalScale=new ze().fromArray(e.clearcoatNormalScale)),e.iridescenceMap!==void 0&&(this.iridescenceMap=t[e.iridescenceMap]||null),e.iridescenceThicknessMap!==void 0&&(this.iridescenceThicknessMap=t[e.iridescenceThicknessMap]||null),e.transmissionMap!==void 0&&(this.transmissionMap=t[e.transmissionMap]||null),e.thicknessMap!==void 0&&(this.thicknessMap=t[e.thicknessMap]||null),e.anisotropyMap!==void 0&&(this.anisotropyMap=t[e.anisotropyMap]||null),e.sheenColorMap!==void 0&&(this.sheenColorMap=t[e.sheenColorMap]||null),e.sheenRoughnessMap!==void 0&&(this.sheenRoughnessMap=t[e.sheenRoughnessMap]||null),this}clone(){return new this.constructor().copy(this)}copy(e){this.name=e.name,this.blending=e.blending,this.side=e.side,this.vertexColors=e.vertexColors,this.opacity=e.opacity,this.transparent=e.transparent,this.blendSrc=e.blendSrc,this.blendDst=e.blendDst,this.blendEquation=e.blendEquation,this.blendSrcAlpha=e.blendSrcAlpha,this.blendDstAlpha=e.blendDstAlpha,this.blendEquationAlpha=e.blendEquationAlpha,this.blendColor.copy(e.blendColor),this.blendAlpha=e.blendAlpha,this.depthFunc=e.depthFunc,this.depthTest=e.depthTest,this.depthWrite=e.depthWrite,this.stencilWriteMask=e.stencilWriteMask,this.stencilFunc=e.stencilFunc,this.stencilRef=e.stencilRef,this.stencilFuncMask=e.stencilFuncMask,this.stencilFail=e.stencilFail,this.stencilZFail=e.stencilZFail,this.stencilZPass=e.stencilZPass,this.stencilWrite=e.stencilWrite;let t=e.clippingPlanes,n=null;if(t!==null){let i=t.length;n=new Array(i);for(let r=0;r!==i;++r)n[r]=t[r].clone()}return this.clippingPlanes=n,this.clipIntersection=e.clipIntersection,this.clipShadows=e.clipShadows,this.shadowSide=e.shadowSide,this.colorWrite=e.colorWrite,this.precision=e.precision,this.polygonOffset=e.polygonOffset,this.polygonOffsetFactor=e.polygonOffsetFactor,this.polygonOffsetUnits=e.polygonOffsetUnits,this.dithering=e.dithering,this.alphaTest=e.alphaTest,this.alphaHash=e.alphaHash,this.alphaToCoverage=e.alphaToCoverage,this.premultipliedAlpha=e.premultipliedAlpha,this.forceSinglePass=e.forceSinglePass,this.allowOverride=e.allowOverride,this.visible=e.visible,this.toneMapped=e.toneMapped,this.userData=JSON.parse(JSON.stringify(e.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(e){e===!0&&this.version++}};var qi=new k,uh=new k,Po=new k,Io=new k,Ci=class{constructor(e=new k,t=new k(0,0,-1)){this.origin=e,this.direction=t}set(e,t){return this.origin.copy(e),this.direction.copy(t),this}copy(e){return this.origin.copy(e.origin),this.direction.copy(e.direction),this}at(e,t){return t.copy(this.origin).addScaledVector(this.direction,e)}lookAt(e){return this.direction.copy(e).sub(this.origin).normalize(),this}recast(e){return this.origin.copy(this.at(e,qi)),this}closestPointToPoint(e,t){t.subVectors(e,this.origin);let n=t.dot(this.direction);return n<0?t.copy(this.origin):t.copy(this.origin).addScaledVector(this.direction,n)}distanceToPoint(e){return Math.sqrt(this.distanceSqToPoint(e))}distanceSqToPoint(e){let t=qi.subVectors(e,this.origin).dot(this.direction);return t<0?this.origin.distanceToSquared(e):(qi.copy(this.origin).addScaledVector(this.direction,t),qi.distanceToSquared(e))}distanceSqToSegment(e,t,n,i){uh.copy(e).add(t).multiplyScalar(.5),Po.copy(t).sub(e).normalize(),Io.copy(this.origin).sub(uh);let r=e.distanceTo(t)*.5,a=-this.direction.dot(Po),o=Io.dot(this.direction),l=-Io.dot(Po),c=Io.lengthSq(),h=Math.abs(1-a*a),d,u,p,x;if(h>0)if(d=a*l-o,u=a*o-l,x=r*h,d>=0)if(u>=-x)if(u<=x){let v=1/h;d*=v,u*=v,p=d*(d+a*u+2*o)+u*(a*d+u+2*l)+c}else u=r,d=Math.max(0,-(a*u+o)),p=-d*d+u*(u+2*l)+c;else u=-r,d=Math.max(0,-(a*u+o)),p=-d*d+u*(u+2*l)+c;else u<=-x?(d=Math.max(0,-(-a*r+o)),u=d>0?-r:Math.min(Math.max(-r,-l),r),p=-d*d+u*(u+2*l)+c):u<=x?(d=0,u=Math.min(Math.max(-r,-l),r),p=u*(u+2*l)+c):(d=Math.max(0,-(a*r+o)),u=d>0?r:Math.min(Math.max(-r,-l),r),p=-d*d+u*(u+2*l)+c);else u=a>0?-r:r,d=Math.max(0,-(a*u+o)),p=-d*d+u*(u+2*l)+c;return n&&n.copy(this.origin).addScaledVector(this.direction,d),i&&i.copy(uh).addScaledVector(Po,u),p}intersectSphere(e,t){if(e.radius<0)return null;qi.subVectors(e.center,this.origin);let n=qi.dot(this.direction),i=qi.dot(qi)-n*n,r=e.radius*e.radius;if(i>r)return null;let a=Math.sqrt(r-i),o=n-a,l=n+a;return l<0?null:o<0?this.at(l,t):this.at(o,t)}intersectsSphere(e){return e.radius<0?!1:this.distanceSqToPoint(e.center)<=e.radius*e.radius}distanceToPlane(e){let t=e.normal.dot(this.direction);if(t===0)return e.distanceToPoint(this.origin)===0?0:null;let n=-(this.origin.dot(e.normal)+e.constant)/t;return n>=0?n:null}intersectPlane(e,t){let n=this.distanceToPlane(e);return n===null?null:this.at(n,t)}intersectsPlane(e){let t=e.distanceToPoint(this.origin);return t===0||e.normal.dot(this.direction)*t<0}intersectBox(e,t){let n,i,r,a,o,l,c=1/this.direction.x,h=1/this.direction.y,d=1/this.direction.z,u=this.origin;return c>=0?(n=(e.min.x-u.x)*c,i=(e.max.x-u.x)*c):(n=(e.max.x-u.x)*c,i=(e.min.x-u.x)*c),h>=0?(r=(e.min.y-u.y)*h,a=(e.max.y-u.y)*h):(r=(e.max.y-u.y)*h,a=(e.min.y-u.y)*h),n>a||r>i||((r>n||isNaN(n))&&(n=r),(a<i||isNaN(i))&&(i=a),d>=0?(o=(e.min.z-u.z)*d,l=(e.max.z-u.z)*d):(o=(e.max.z-u.z)*d,l=(e.min.z-u.z)*d),n>l||o>i)||((o>n||n!==n)&&(n=o),(l<i||i!==i)&&(i=l),i<0)?null:this.at(n>=0?n:i,t)}intersectsBox(e){return this.intersectBox(e,qi)!==null}intersectTriangle(e,t,n,i,r){let a=this.origin,o=this.direction,l=o.x,c=o.y,h=o.z,d=e.x-a.x,u=e.y-a.y,p=e.z-a.z,x=t.x-a.x,v=t.y-a.y,m=t.z-a.z,f=n.x-a.x,y=n.y-a.y,T=n.z-a.z,g=Math.abs(l),_=Math.abs(c),S=Math.abs(h),A,b,C,w,I,M,N,D,U,H,L,W;if(g>=_&&g>=S?(C=l,M=d,U=x,W=f,l>=0?(A=c,b=h,w=u,I=p,N=v,D=m,H=y,L=T):(A=h,b=c,w=p,I=u,N=m,D=v,H=T,L=y)):_>=S?(C=c,M=u,U=v,W=y,c>=0?(A=h,b=l,w=p,I=d,N=m,D=x,H=T,L=f):(A=l,b=h,w=d,I=p,N=x,D=m,H=f,L=T)):(C=h,M=p,U=m,W=T,h>=0?(A=l,b=c,w=d,I=u,N=x,D=v,H=f,L=y):(A=c,b=l,w=u,I=d,N=v,D=x,H=y,L=f)),C===0)return null;let z=A/C,E=b/C,J=1/C,q=w-z*M,te=I-E*M,Le=N-z*U,Fe=D-E*U,Ne=H-z*W,se=L-E*W,oe=Ne*Fe-se*Le,Pe=q*se-te*Ne,ge=Le*te-Fe*q;if(i){if(oe<0||Pe<0||ge<0)return null}else if((oe<0||Pe<0||ge<0)&&(oe>0||Pe>0||ge>0))return null;let xe=oe+Pe+ge;if(xe===0)return null;let $=J*(oe*M+Pe*U+ge*W);return(xe>0?$<0:$>0)?null:this.at($/xe,r)}applyMatrix4(e){return this.origin.applyMatrix4(e),this.direction.transformDirection(e),this}equals(e){return e.origin.equals(this.origin)&&e.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},qt=class extends zn{constructor(e){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new Ce(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new kn,this.combine=Cl,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.fog=e.fog,this}},Rd=new mt,Is=new Ci,Do=new In,Cd=new k,Lo=new k,Fo=new k,No=new k,dh=new k,Uo=new k,Pd=new k,Oo=new k,ot=class extends Xt{constructor(e=new Mt,t=new qt){super(),this.isMesh=!0,this.type="Mesh",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.count=1,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),e.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=e.morphTargetInfluences.slice()),e.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},e.morphTargetDictionary)),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}getVertexPosition(e,t){let n=this.geometry,i=n.attributes.position,r=n.morphAttributes.position,a=n.morphTargetsRelative;t.fromBufferAttribute(i,e);let o=this.morphTargetInfluences;if(r&&o){Uo.set(0,0,0);for(let l=0,c=r.length;l<c;l++){let h=o[l],d=r[l];h!==0&&(dh.fromBufferAttribute(d,e),a?Uo.addScaledVector(dh,h):Uo.addScaledVector(dh.sub(t),h))}t.add(Uo)}return t}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.material,r=this.matrixWorld;i!==void 0&&(n.boundingSphere===null&&n.computeBoundingSphere(),Do.copy(n.boundingSphere),Do.applyMatrix4(r),Is.copy(e.ray).recast(e.near),!(Do.containsPoint(Is.origin)===!1&&(Is.intersectSphere(Do,Cd)===null||Is.origin.distanceToSquared(Cd)>(e.far-e.near)**2))&&(Rd.copy(r).invert(),Is.copy(e.ray).applyMatrix4(Rd),!(n.boundingBox!==null&&Is.intersectsBox(n.boundingBox)===!1)&&this._computeIntersections(e,t,Is)))}_computeIntersections(e,t,n){let i,r=this.geometry,a=this.material,o=r.index,l=r.attributes.position,c=r.attributes.uv,h=r.attributes.uv1,d=r.attributes.normal,u=r.groups,p=r.drawRange;if(o!==null)if(Array.isArray(a))for(let x=0,v=u.length;x<v;x++){let m=u[x],f=a[m.materialIndex],y=Math.max(m.start,p.start),T=Math.min(o.count,Math.min(m.start+m.count,p.start+p.count));for(let g=y,_=T;g<_;g+=3){let S=o.getX(g),A=o.getX(g+1),b=o.getX(g+2);i=zo(this,f,e,n,c,h,d,S,A,b),i&&(i.faceIndex=Math.floor(g/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let x=Math.max(0,p.start),v=Math.min(o.count,p.start+p.count);for(let m=x,f=v;m<f;m+=3){let y=o.getX(m),T=o.getX(m+1),g=o.getX(m+2);i=zo(this,a,e,n,c,h,d,y,T,g),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}else if(l!==void 0)if(Array.isArray(a))for(let x=0,v=u.length;x<v;x++){let m=u[x],f=a[m.materialIndex],y=Math.max(m.start,p.start),T=Math.min(l.count,Math.min(m.start+m.count,p.start+p.count));for(let g=y,_=T;g<_;g+=3){let S=g,A=g+1,b=g+2;i=zo(this,f,e,n,c,h,d,S,A,b),i&&(i.faceIndex=Math.floor(g/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let x=Math.max(0,p.start),v=Math.min(l.count,p.start+p.count);for(let m=x,f=v;m<f;m+=3){let y=m,T=m+1,g=m+2;i=zo(this,a,e,n,c,h,d,y,T,g),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}}};function Rg(s,e,t,n,i,r,a,o){let l;if(e.side===xn?l=n.intersectTriangle(a,r,i,!0,o):l=n.intersectTriangle(i,r,a,e.side===ii,o),l===null)return null;Oo.copy(o),Oo.applyMatrix4(s.matrixWorld);let c=t.ray.origin.distanceTo(Oo);return c<t.near||c>t.far?null:{distance:c,point:Oo.clone(),object:s}}function zo(s,e,t,n,i,r,a,o,l,c){s.getVertexPosition(o,Lo),s.getVertexPosition(l,Fo),s.getVertexPosition(c,No);let h=Rg(s,e,t,n,Lo,Fo,No,Pd);if(h){let d=new k;fs.getBarycoord(Pd,Lo,Fo,No,d),i&&(h.uv=fs.getInterpolatedAttribute(i,o,l,c,d,new ze)),r&&(h.uv1=fs.getInterpolatedAttribute(r,o,l,c,d,new ze)),a&&(h.normal=fs.getInterpolatedAttribute(a,o,l,c,d,new k),h.normal.dot(n.direction)>0&&h.normal.multiplyScalar(-1));let u={a:o,b:l,c,normal:new k,materialIndex:0};fs.getNormal(Lo,Fo,No,u.normal),h.face=u,h.barycoord=d}return h}var ha=new Nt,Id=new Nt,Dd=new Nt,Cg=new Nt,Ld=new mt,ko=new k,fh=new In,Fd=new mt,ph=new Ci,Ea=class extends ot{constructor(e,t){super(e,t),this.isSkinnedMesh=!0,this.type="SkinnedMesh",this.bindMode=yh,this.bindMatrix=new mt,this.bindMatrixInverse=new mt,this.boundingBox=null,this.boundingSphere=null}computeBoundingBox(){let e=this.geometry;this.boundingBox===null&&(this.boundingBox=new hn),this.boundingBox.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,ko),this.boundingBox.expandByPoint(ko)}computeBoundingSphere(){let e=this.geometry;this.boundingSphere===null&&(this.boundingSphere=new In),this.boundingSphere.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,ko),this.boundingSphere.expandByPoint(ko)}copy(e,t){return super.copy(e,t),this.bindMode=e.bindMode,this.bindMatrix.copy(e.bindMatrix),this.bindMatrixInverse.copy(e.bindMatrixInverse),this.skeleton=e.skeleton,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}raycast(e,t){let n=this.material,i=this.matrixWorld;n!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),fh.copy(this.boundingSphere),fh.applyMatrix4(i),e.ray.intersectsSphere(fh)!==!1&&(Fd.copy(i).invert(),ph.copy(e.ray).applyMatrix4(Fd),!(this.boundingBox!==null&&ph.intersectsBox(this.boundingBox)===!1)&&this._computeIntersections(e,t,ph)))}getVertexPosition(e,t){return super.getVertexPosition(e,t),this.applyBoneTransform(e,t),t}bind(e,t){this.skeleton=e,t===void 0&&(this.updateMatrixWorld(!0),this.skeleton.calculateInverses(),t=this.matrixWorld),this.bindMatrix.copy(t),this.bindMatrixInverse.copy(t).invert()}pose(){this.skeleton.pose()}normalizeSkinWeights(){let e=new Nt,t=this.geometry.attributes.skinWeight;for(let n=0,i=t.count;n<i;n++){e.fromBufferAttribute(t,n);let r=1/e.manhattanLength();r!==1/0?e.multiplyScalar(r):e.set(1,0,0,0),t.setXYZW(n,e.x,e.y,e.z,e.w)}}updateMatrixWorld(e){super.updateMatrixWorld(e),this.bindMode===yh?this.bindMatrixInverse.copy(this.matrixWorld).invert():this.bindMode===Rf?this.bindMatrixInverse.copy(this.bindMatrix).invert():ht("SkinnedMesh: Unrecognized bindMode: "+this.bindMode)}applyBoneTransform(e,t){let n=this.skeleton,i=this.geometry;Id.fromBufferAttribute(i.attributes.skinIndex,e),Dd.fromBufferAttribute(i.attributes.skinWeight,e),t.isVector4?(ha.copy(t),t.set(0,0,0,0)):(ha.set(...t,1),t.set(0,0,0)),ha.applyMatrix4(this.bindMatrix);for(let r=0;r<4;r++){let a=Dd.getComponent(r);if(a!==0){let o=Id.getComponent(r);Ld.multiplyMatrices(n.bones[o].matrixWorld,n.boneInverses[o]),t.addScaledVector(Cg.copy(ha).applyMatrix4(Ld),a)}}return t.isVector4&&(t.w=ha.w),t.applyMatrix4(this.bindMatrixInverse)}},Rr=class extends Xt{constructor(){super(),this.isBone=!0,this.type="Bone"}},ps=class extends cn{constructor(e=null,t=1,n=1,i,r,a,o,l,c=ln,h=ln,d,u){super(null,a,o,l,c,h,i,r,d,u),this.isDataTexture=!0,this.image={data:e,width:t,height:n},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}},Nd=new mt,Pg=new mt,Ta=class s{constructor(e=[],t=[]){this.uuid=pi(),this.bones=e.slice(0),this.boneInverses=t,this.boneMatrices=null,this.boneTexture=null,this.init()}init(){let e=this.bones,t=this.boneInverses;if(this.boneMatrices=new Float32Array(e.length*16),t.length===0)this.calculateInverses();else if(e.length!==t.length){ht("Skeleton: Number of inverse bone matrices does not match amount of bones."),this.boneInverses=[];for(let n=0,i=this.bones.length;n<i;n++)this.boneInverses.push(new mt)}}calculateInverses(){this.boneInverses.length=0;for(let e=0,t=this.bones.length;e<t;e++){let n=new mt;this.bones[e]&&n.copy(this.bones[e].matrixWorld).invert(),this.boneInverses.push(n)}}pose(){for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&n.matrixWorld.copy(this.boneInverses[e]).invert()}for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&(n.parent&&n.parent.isBone?(n.matrix.copy(n.parent.matrixWorld).invert(),n.matrix.multiply(n.matrixWorld)):n.matrix.copy(n.matrixWorld),n.matrix.decompose(n.position,n.quaternion,n.scale))}}update(){let e=this.bones,t=this.boneInverses,n=this.boneMatrices,i=this.boneTexture;for(let r=0,a=e.length;r<a;r++){let o=e[r]?e[r].matrixWorld:Pg;Nd.multiplyMatrices(o,t[r]),Nd.toArray(n,r*16)}i!==null&&(i.needsUpdate=!0)}clone(){return new s(this.bones,this.boneInverses)}computeBoneTexture(){let e=Math.sqrt(this.bones.length*4);e=Math.ceil(e/4)*4,e=Math.max(e,4);let t=new Float32Array(e*e*4);t.set(this.boneMatrices);let n=new ps(t,e,e,Qn,Zn);return n.needsUpdate=!0,this.boneMatrices=t,this.boneTexture=n,this}getBoneByName(e){for(let t=0,n=this.bones.length;t<n;t++){let i=this.bones[t];if(i.name===e)return i}}dispose(){this.boneTexture!==null&&(this.boneTexture.dispose(),this.boneTexture=null)}fromJSON(e,t){this.uuid=e.uuid;for(let n=0,i=e.bones.length;n<i;n++){let r=e.bones[n],a=t[r];a===void 0&&(ht("Skeleton: No bone found with UUID:",r),a=new Rr),this.bones.push(a),this.boneInverses.push(new mt().fromArray(e.boneInverses[n]))}return this.init(),this}toJSON(){let e={metadata:{version:4.7,type:"Skeleton",generator:"Skeleton.toJSON"},bones:[],boneInverses:[]};e.uuid=this.uuid;let t=this.bones,n=this.boneInverses;for(let i=0,r=t.length;i<r;i++){let a=t[i];e.bones.push(a.uuid);let o=n[i];e.boneInverses.push(o.toArray())}return e}},mi=class extends Tt{constructor(e,t,n,i=1){super(e,t,n),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=i}copy(e){return super.copy(e),this.meshPerAttribute=e.meshPerAttribute,this}toJSON(){let e=super.toJSON();return e.meshPerAttribute=this.meshPerAttribute,e.isInstancedBufferAttribute=!0,e}},fr=new mt,Ud=new mt,Bo=[],Od=new hn,Ig=new mt,ua=new ot,da=new In,En=class extends ot{constructor(e,t,n){super(e,t),this.isInstancedMesh=!0,this.instanceMatrix=new mi(new Float32Array(n*16),16),this.instanceColor=null,this.morphTexture=null,this.count=n,this.boundingBox=null,this.boundingSphere=null;for(let i=0;i<n;i++)this.setMatrixAt(i,Ig)}computeBoundingBox(){let e=this.geometry,t=this.count;this.boundingBox===null&&(this.boundingBox=new hn),e.boundingBox===null&&e.computeBoundingBox(),this.boundingBox.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,fr),Od.copy(e.boundingBox).applyMatrix4(fr),this.boundingBox.union(Od)}computeBoundingSphere(){let e=this.geometry,t=this.count;this.boundingSphere===null&&(this.boundingSphere=new In),e.boundingSphere===null&&e.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,fr),da.copy(e.boundingSphere).applyMatrix4(fr),this.boundingSphere.union(da)}copy(e,t){return super.copy(e,t),this.instanceMatrix.copy(e.instanceMatrix),e.morphTexture!==null&&(this.morphTexture=e.morphTexture.clone()),e.instanceColor!==null&&(this.instanceColor=e.instanceColor.clone()),this.count=e.count,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}getColorAt(e,t){return this.instanceColor===null?t.setRGB(1,1,1):t.fromArray(this.instanceColor.array,e*3)}getMatrixAt(e,t){return t.fromArray(this.instanceMatrix.array,e*16)}getMorphAt(e,t){let n=t.morphTargetInfluences,i=this.morphTexture.source.data.data,r=n.length+1,a=e*r+1;for(let o=0;o<n.length;o++)n[o]=i[a+o]}raycast(e,t){let n=this.matrixWorld,i=this.count;if(ua.geometry=this.geometry,ua.material=this.material,ua.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),da.copy(this.boundingSphere),da.applyMatrix4(n),e.ray.intersectsSphere(da)!==!1))for(let r=0;r<i;r++){this.getMatrixAt(r,fr),Ud.multiplyMatrices(n,fr),ua.matrixWorld=Ud,ua.raycast(e,Bo);for(let a=0,o=Bo.length;a<o;a++){let l=Bo[a];l.instanceId=r,l.object=this,t.push(l)}Bo.length=0}}setColorAt(e,t){return this.instanceColor===null&&(this.instanceColor=new mi(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),t.toArray(this.instanceColor.array,e*3),this}setMatrixAt(e,t){return t.toArray(this.instanceMatrix.array,e*16),this}setMorphAt(e,t){let n=t.morphTargetInfluences,i=n.length+1;this.morphTexture===null&&(this.morphTexture=new ps(new Float32Array(i*this.count),i,this.count,Ul,Zn));let r=this.morphTexture.source.data.data,a=0;for(let c=0;c<n.length;c++)a+=n[c];let o=this.geometry.morphTargetsRelative?1:1-a,l=i*e;return r[l]=o,r.set(n,l+1),this}updateMorphTargets(){}dispose(){super.dispose(),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null)}},Ds=new In,Dg=new ze(.5,.5),Ho=new k,Cr=class{constructor(e=new Kn,t=new Kn,n=new Kn,i=new Kn,r=new Kn,a=new Kn){this.planes=[e,t,n,i,r,a]}set(e,t,n,i,r,a){let o=this.planes;return o[0].copy(e),o[1].copy(t),o[2].copy(n),o[3].copy(i),o[4].copy(r),o[5].copy(a),this}copy(e){let t=this.planes;for(let n=0;n<6;n++)t[n].copy(e.planes[n]);return this}setFromProjectionMatrix(e,t=fi,n=!1){let i=this.planes,r=e.elements,a=r[0],o=r[1],l=r[2],c=r[3],h=r[4],d=r[5],u=r[6],p=r[7],x=r[8],v=r[9],m=r[10],f=r[11],y=r[12],T=r[13],g=r[14],_=r[15];if(i[0].setComponents(c-a,p-h,f-x,_-y).normalize(),i[1].setComponents(c+a,p+h,f+x,_+y).normalize(),i[2].setComponents(c+o,p+d,f+v,_+T).normalize(),i[3].setComponents(c-o,p-d,f-v,_-T).normalize(),n)i[4].setComponents(l,u,m,g).normalize(),i[5].setComponents(c-l,p-u,f-m,_-g).normalize();else if(i[4].setComponents(c-l,p-u,f-m,_-g).normalize(),t===fi)i[5].setComponents(c+l,p+u,f+m,_+g).normalize();else if(t===_r)i[5].setComponents(l,u,m,g).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+t);return this}intersectsObject(e){if(e.boundingSphere!==void 0)e.boundingSphere===null&&e.computeBoundingSphere(),Ds.copy(e.boundingSphere).applyMatrix4(e.matrixWorld);else{let t=e.geometry;t.boundingSphere===null&&t.computeBoundingSphere(),Ds.copy(t.boundingSphere).applyMatrix4(e.matrixWorld)}return this.intersectsSphere(Ds)}intersectsSprite(e){Ds.center.set(0,0,0);let t=Dg.distanceTo(e.center);return Ds.radius=.7071067811865476+t,Ds.applyMatrix4(e.matrixWorld),this.intersectsSphere(Ds)}intersectsSphere(e){let t=this.planes,n=e.center,i=-e.radius;for(let r=0;r<6;r++)if(t[r].distanceToPoint(n)<i)return!1;return!0}intersectsBox(e){let t=this.planes;for(let n=0;n<6;n++){let i=t[n];if(Ho.x=i.normal.x>0?e.max.x:e.min.x,Ho.y=i.normal.y>0?e.max.y:e.min.y,Ho.z=i.normal.z>0?e.max.z:e.min.z,i.distanceToPoint(Ho)<0)return!1}return!0}containsPoint(e){let t=this.planes;for(let n=0;n<6;n++)if(t[n].distanceToPoint(e)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};var Pi=class extends zn{constructor(e){super(),this.isLineBasicMaterial=!0,this.type="LineBasicMaterial",this.color=new Ce(16777215),this.map=null,this.linewidth=1,this.linecap="round",this.linejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.linewidth=e.linewidth,this.linecap=e.linecap,this.linejoin=e.linejoin,this.fog=e.fog,this}},ol=new k,ll=new k,zd=new mt,fa=new Ci,Go=new In,mh=new k,kd=new k,Ki=class extends Xt{constructor(e=new Mt,t=new Pi){super(),this.isLine=!0,this.type="Line",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[0];for(let i=1,r=t.count;i<r;i++)ol.fromBufferAttribute(t,i-1),ll.fromBufferAttribute(t,i),n[i]=n[i-1],n[i]+=ol.distanceTo(ll);e.setAttribute("lineDistance",new xt(n,1))}else ht("Line.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Line.threshold,a=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),Go.copy(n.boundingSphere),Go.applyMatrix4(i),Go.radius+=r,e.ray.intersectsSphere(Go)===!1)return;zd.copy(i).invert(),fa.copy(e.ray).applyMatrix4(zd);let o=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=o*o,c=this.isLineSegments?2:1,h=n.index,u=n.attributes.position;if(h!==null){let p=Math.max(0,a.start),x=Math.min(h.count,a.start+a.count);for(let v=p,m=x-1;v<m;v+=c){let f=h.getX(v),y=h.getX(v+1),T=Vo(this,e,fa,l,f,y,v);T&&t.push(T)}if(this.isLineLoop){let v=h.getX(x-1),m=h.getX(p),f=Vo(this,e,fa,l,v,m,x-1);f&&t.push(f)}}else{let p=Math.max(0,a.start),x=Math.min(u.count,a.start+a.count);for(let v=p,m=x-1;v<m;v+=c){let f=Vo(this,e,fa,l,v,v+1,v);f&&t.push(f)}if(this.isLineLoop){let v=Vo(this,e,fa,l,x-1,p,x-1);v&&t.push(v)}}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}};function Vo(s,e,t,n,i,r,a){let o=s.geometry.attributes.position;if(ol.fromBufferAttribute(o,i),ll.fromBufferAttribute(o,r),t.distanceSqToSegment(ol,ll,mh,kd)>n)return;mh.applyMatrix4(s.matrixWorld);let c=e.ray.origin.distanceTo(mh);if(!(c<e.near||c>e.far))return{distance:c,point:kd.clone().applyMatrix4(s.matrixWorld),index:a,face:null,faceIndex:null,barycoord:null,object:s}}var Bd=new k,Hd=new k,Bs=class extends Ki{constructor(e,t){super(e,t),this.isLineSegments=!0,this.type="LineSegments"}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[];for(let i=0,r=t.count;i<r;i+=2)Bd.fromBufferAttribute(t,i),Hd.fromBufferAttribute(t,i+1),n[i]=i===0?0:n[i-1],n[i+1]=n[i]+Bd.distanceTo(Hd);e.setAttribute("lineDistance",new xt(n,1))}else ht("LineSegments.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}},Aa=class extends Ki{constructor(e,t){super(e,t),this.isLineLoop=!0,this.type="LineLoop"}},Pr=class extends zn{constructor(e){super(),this.isPointsMaterial=!0,this.type="PointsMaterial",this.color=new Ce(16777215),this.map=null,this.alphaMap=null,this.size=1,this.sizeAttenuation=!0,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.alphaMap=e.alphaMap,this.size=e.size,this.sizeAttenuation=e.sizeAttenuation,this.fog=e.fog,this}},Gd=new mt,wh=new Ci,Wo=new In,qo=new k,Tn=class extends Xt{constructor(e=new Mt,t=new Pr){super(),this.isPoints=!0,this.type="Points",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Points.threshold,a=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),Wo.copy(n.boundingSphere),Wo.applyMatrix4(i),Wo.radius+=r,e.ray.intersectsSphere(Wo)===!1)return;Gd.copy(i).invert(),wh.copy(e.ray).applyMatrix4(Gd);let o=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=o*o,c=n.index,d=n.attributes.position;if(c!==null){let u=Math.max(0,a.start),p=Math.min(c.count,a.start+a.count);for(let x=u,v=p;x<v;x++){let m=c.getX(x);qo.fromBufferAttribute(d,m),Vd(qo,m,l,i,e,t,this)}}else{let u=Math.max(0,a.start),p=Math.min(d.count,a.start+a.count);for(let x=u,v=p;x<v;x++)qo.fromBufferAttribute(d,x),Vd(qo,x,l,i,e,t,this)}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}};function Vd(s,e,t,n,i,r,a){let o=wh.distanceSqToPoint(s);if(o<t){let l=new k;wh.closestPointToPoint(s,l),l.applyMatrix4(n);let c=i.ray.origin.distanceTo(l);if(c<i.near||c>i.far)return;r.push({distance:c,distanceToRay:Math.sqrt(o),point:l,index:e,face:null,faceIndex:null,barycoord:null,object:a})}}var Ra=class extends cn{constructor(e=[],t=ys,n,i,r,a,o,l,c,h){super(e,t,n,i,r,a,o,l,c,h),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(e){this.image=e}},Hs=class extends cn{constructor(e,t,n,i,r,a,o,l,c){super(e,t,n,i,r,a,o,l,c),this.isCanvasTexture=!0,this.needsUpdate=!0}};var ms=class extends cn{constructor(e,t,n=yi,i,r,a,o=ln,l=ln,c,h=Ri,d=1){if(h!==Ri&&h!==Ms)throw new Error("THREE.DepthTexture: format must be either THREE.DepthFormat or THREE.DepthStencilFormat");let u={width:e,height:t,depth:d};super(u,i,r,a,o,l,h,n,c),this.isDepthTexture=!0,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(e){return super.copy(e),this.source=new wr(Object.assign({},e.image)),this.compareFunction=e.compareFunction,this}toJSON(e){let t=super.toJSON(e);return t.compareFunction=this.compareFunction,t}},cl=class extends ms{constructor(e,t=yi,n=ys,i,r,a=ln,o=ln,l,c=Ri){let h={width:e,height:e,depth:1},d=[h,h,h,h,h,h];super(e,e,t,n,i,r,a,o,l,c),this.image=d,this.isCubeDepthTexture=!0,this.isCubeTexture=!0}get images(){return this.image}set images(e){this.image=e}},Ca=class extends cn{constructor(e=null){super(),this.sourceTexture=e,this.isExternalTexture=!0}copy(e){return super.copy(e),this.sourceTexture=e.sourceTexture,this}},gi=class s extends Mt{constructor(e=1,t=1,n=1,i=1,r=1,a=1){super(),this.type="BoxGeometry",this.parameters={width:e,height:t,depth:n,widthSegments:i,heightSegments:r,depthSegments:a};let o=this;i=Math.floor(i),r=Math.floor(r),a=Math.floor(a);let l=[],c=[],h=[],d=[],u=0,p=0;x("z","y","x",-1,-1,n,t,e,a,r,0),x("z","y","x",1,-1,n,t,-e,a,r,1),x("x","z","y",1,1,e,n,t,i,a,2),x("x","z","y",1,-1,e,n,-t,i,a,3),x("x","y","z",1,-1,e,t,n,i,r,4),x("x","y","z",-1,-1,e,t,-n,i,r,5),this.setIndex(l),this.setAttribute("position",new xt(c,3)),this.setAttribute("normal",new xt(h,3)),this.setAttribute("uv",new xt(d,2));function x(v,m,f,y,T,g,_,S,A,b,C){let w=g/A,I=_/b,M=g/2,N=_/2,D=S/2,U=A+1,H=b+1,L=0,W=0,z=new k;for(let E=0;E<H;E++){let J=E*I-N;for(let q=0;q<U;q++){let te=q*w-M;z[v]=te*y,z[m]=J*T,z[f]=D,c.push(z.x,z.y,z.z),z[v]=0,z[m]=0,z[f]=S>0?1:-1,h.push(z.x,z.y,z.z),d.push(q/A),d.push(1-E/b),L+=1}}for(let E=0;E<b;E++)for(let J=0;J<A;J++){let q=u+J+U*E,te=u+J+U*(E+1),Le=u+(J+1)+U*(E+1),Fe=u+(J+1)+U*E;l.push(q,te,Fe),l.push(te,Le,Fe),W+=6}o.addGroup(p,W,C),p+=W,u+=L}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.depth,e.widthSegments,e.heightSegments,e.depthSegments)}};var Ir=class s extends Mt{constructor(e=1,t=32,n=0,i=Math.PI*2){super(),this.type="CircleGeometry",this.parameters={radius:e,segments:t,thetaStart:n,thetaLength:i},t=Math.max(3,t);let r=[],a=[],o=[],l=[],c=new k,h=new ze;a.push(0,0,0),o.push(0,0,1),l.push(.5,.5);for(let d=0,u=3;d<=t;d++,u+=3){let p=n+d/t*i;c.x=e*Math.cos(p),c.y=e*Math.sin(p),a.push(c.x,c.y,c.z),o.push(0,0,1),h.x=(a[u]/e+1)/2,h.y=(a[u+1]/e+1)/2,l.push(h.x,h.y)}for(let d=1;d<=t;d++)r.push(d,d+1,0);this.setIndex(r),this.setAttribute("position",new xt(a,3)),this.setAttribute("normal",new xt(o,3)),this.setAttribute("uv",new xt(l,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.segments,e.thetaStart,e.thetaLength)}},gs=class s extends Mt{constructor(e=1,t=1,n=1,i=32,r=1,a=!1,o=0,l=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:e,radiusBottom:t,height:n,radialSegments:i,heightSegments:r,openEnded:a,thetaStart:o,thetaLength:l};let c=this;i=Math.floor(i),r=Math.floor(r);let h=[],d=[],u=[],p=[],x=0,v=[],m=n/2,f=0;y(),a===!1&&(e>0&&T(!0),t>0&&T(!1)),this.setIndex(h),this.setAttribute("position",new xt(d,3)),this.setAttribute("normal",new xt(u,3)),this.setAttribute("uv",new xt(p,2));function y(){let g=new k,_=new k,S=0,A=(t-e)/n;for(let b=0;b<=r;b++){let C=[],w=b/r,I=w*(t-e)+e;for(let M=0;M<=i;M++){let N=M/i,D=N*l+o,U=Math.sin(D),H=Math.cos(D);_.x=I*U,_.y=-w*n+m,_.z=I*H,d.push(_.x,_.y,_.z),g.set(U,A,H).normalize(),u.push(g.x,g.y,g.z),p.push(N,1-w),C.push(x++)}v.push(C)}for(let b=0;b<i;b++)for(let C=0;C<r;C++){let w=v[C][b],I=v[C+1][b],M=v[C+1][b+1],N=v[C][b+1];(e>0||C!==0)&&(h.push(w,I,N),S+=3),(t>0||C!==r-1)&&(h.push(I,M,N),S+=3)}c.addGroup(f,S,0),f+=S}function T(g){let _=x,S=new ze,A=new k,b=0,C=g===!0?e:t,w=g===!0?1:-1;for(let M=1;M<=i;M++)d.push(0,m*w,0),u.push(0,w,0),p.push(.5,.5),x++;let I=x;for(let M=0;M<=i;M++){let D=M/i*l+o,U=Math.cos(D),H=Math.sin(D);A.x=C*H,A.y=m*w,A.z=C*U,d.push(A.x,A.y,A.z),u.push(0,w,0),S.x=U*.5+.5,S.y=H*.5*w+.5,p.push(S.x,S.y),x++}for(let M=0;M<i;M++){let N=_+M,D=I+M;g===!0?h.push(D,D+1,N):h.push(D+1,D,N),b+=3}c.addGroup(f,b,g===!0?1:2),f+=b}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radiusTop,e.radiusBottom,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Gs=class s extends gs{constructor(e=1,t=1,n=32,i=1,r=!1,a=0,o=Math.PI*2){super(0,e,t,n,i,r,a,o),this.type="ConeGeometry",this.parameters={radius:e,height:t,radialSegments:n,heightSegments:i,openEnded:r,thetaStart:a,thetaLength:o}}static fromJSON(e){return new s(e.radius,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},hl=class s extends Mt{constructor(e=[],t=[],n=1,i=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:e,indices:t,radius:n,detail:i};let r=[],a=[];o(i),c(n),h(),this.setAttribute("position",new xt(r,3)),this.setAttribute("normal",new xt(r.slice(),3)),this.setAttribute("uv",new xt(a,2)),i===0?this.computeVertexNormals():this.normalizeNormals();function o(y){let T=new k,g=new k,_=new k;for(let S=0;S<t.length;S+=3)p(t[S+0],T),p(t[S+1],g),p(t[S+2],_),l(T,g,_,y)}function l(y,T,g,_){let S=_+1,A=[];for(let b=0;b<=S;b++){A[b]=[];let C=y.clone().lerp(g,b/S),w=T.clone().lerp(g,b/S),I=S-b;for(let M=0;M<=I;M++)M===0&&b===S?A[b][M]=C:A[b][M]=C.clone().lerp(w,M/I)}for(let b=0;b<S;b++)for(let C=0;C<2*(S-b)-1;C++){let w=Math.floor(C/2);C%2===0?(u(A[b][w+1]),u(A[b+1][w]),u(A[b][w])):(u(A[b][w+1]),u(A[b+1][w+1]),u(A[b+1][w]))}}function c(y){let T=new k;for(let g=0;g<r.length;g+=3)T.x=r[g+0],T.y=r[g+1],T.z=r[g+2],T.normalize().multiplyScalar(y),r[g+0]=T.x,r[g+1]=T.y,r[g+2]=T.z}function h(){let y=new k;for(let T=0;T<r.length;T+=3){y.x=r[T+0],y.y=r[T+1],y.z=r[T+2];let g=m(y)/2/Math.PI+.5,_=f(y)/Math.PI+.5;a.push(g,1-_)}x(),d()}function d(){for(let y=0;y<a.length;y+=6){let T=a[y+0],g=a[y+2],_=a[y+4],S=Math.max(T,g,_),A=Math.min(T,g,_);S>.9&&A<.1&&(T<.2&&(a[y+0]+=1),g<.2&&(a[y+2]+=1),_<.2&&(a[y+4]+=1))}}function u(y){r.push(y.x,y.y,y.z)}function p(y,T){let g=y*3;T.x=e[g+0],T.y=e[g+1],T.z=e[g+2]}function x(){let y=new k,T=new k,g=new k,_=new k,S=new ze,A=new ze,b=new ze;for(let C=0,w=0;C<r.length;C+=9,w+=6){y.set(r[C+0],r[C+1],r[C+2]),T.set(r[C+3],r[C+4],r[C+5]),g.set(r[C+6],r[C+7],r[C+8]),S.set(a[w+0],a[w+1]),A.set(a[w+2],a[w+3]),b.set(a[w+4],a[w+5]),_.copy(y).add(T).add(g).divideScalar(3);let I=m(_);v(S,w+0,y,I),v(A,w+2,T,I),v(b,w+4,g,I)}}function v(y,T,g,_){_<0&&y.x===1&&(a[T]=y.x-1),g.x===0&&g.z===0&&(a[T]=_/2/Math.PI+.5)}function m(y){return Math.atan2(y.z,-y.x)}function f(y){return Math.atan2(-y.y,Math.sqrt(y.x*y.x+y.z*y.z))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.vertices,e.indices,e.radius,e.detail)}};var Yn=class{constructor(){this.type="Curve",this.arcLengthDivisions=200,this.needsUpdate=!1,this.cacheArcLengths=null}getPoint(){ht("Curve: .getPoint() not implemented.")}getPointAt(e,t){let n=this.getUtoTmapping(e);return this.getPoint(n,t)}getPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return t}getSpacedPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPointAt(n/e));return t}getLength(){let e=this.getLengths();return e[e.length-1]}getLengths(e=this.arcLengthDivisions){if(this.cacheArcLengths&&this.cacheArcLengths.length===e+1&&!this.needsUpdate)return this.cacheArcLengths;this.needsUpdate=!1;let t=[],n,i=this.getPoint(0),r=0;t.push(0);for(let a=1;a<=e;a++)n=this.getPoint(a/e),r+=n.distanceTo(i),t.push(r),i=n;return this.cacheArcLengths=t,t}updateArcLengths(){this.needsUpdate=!0,this.getLengths()}getUtoTmapping(e,t=null){let n=this.getLengths(),i=0,r=n.length,a;t?a=t:a=e*n[r-1];let o=0,l=r-1,c;for(;o<=l;)if(i=Math.floor(o+(l-o)/2),c=n[i]-a,c<0)o=i+1;else if(c>0)l=i-1;else{l=i;break}if(i=l,n[i]===a)return i/(r-1);let h=n[i],u=n[i+1]-h,p=(a-h)/u;return(i+p)/(r-1)}getTangent(e,t){let i=e-1e-4,r=e+1e-4;i<0&&(i=0),r>1&&(r=1);let a=this.getPoint(i),o=this.getPoint(r),l=t||(a.isVector2?new ze:new k);return l.copy(o).sub(a).normalize(),l}getTangentAt(e,t){let n=this.getUtoTmapping(e);return this.getTangent(n,t)}computeFrenetFrames(e,t=!1){let n=new k,i=[],r=[],a=[],o=new k,l=new mt;for(let p=0;p<=e;p++){let x=p/e;i[p]=this.getTangentAt(x,new k)}r[0]=new k,a[0]=new k;let c=Number.MAX_VALUE,h=Math.abs(i[0].x),d=Math.abs(i[0].y),u=Math.abs(i[0].z);h<=c&&(c=h,n.set(1,0,0)),d<=c&&(c=d,n.set(0,1,0)),u<=c&&n.set(0,0,1),o.crossVectors(i[0],n).normalize(),r[0].crossVectors(i[0],o),a[0].crossVectors(i[0],r[0]);for(let p=1;p<=e;p++){if(r[p]=r[p-1].clone(),a[p]=a[p-1].clone(),o.crossVectors(i[p-1],i[p]),o.length()>Number.EPSILON){o.normalize();let x=Math.acos(Ct(i[p-1].dot(i[p]),-1,1));r[p].applyMatrix4(l.makeRotationAxis(o,x))}a[p].crossVectors(i[p],r[p])}if(t===!0){let p=Math.acos(Ct(r[0].dot(r[e]),-1,1));p/=e,i[0].dot(o.crossVectors(r[0],r[e]))>0&&(p=-p);for(let x=1;x<=e;x++)r[x].applyMatrix4(l.makeRotationAxis(i[x],p*x)),a[x].crossVectors(i[x],r[x])}return{tangents:i,normals:r,binormals:a}}clone(){return new this.constructor().copy(this)}copy(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}toJSON(){let e={metadata:{version:4.7,type:"Curve",generator:"Curve.toJSON"}};return e.arcLengthDivisions=this.arcLengthDivisions,e.type=this.type,e}fromJSON(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}},Pa=class extends Yn{constructor(e=0,t=0,n=1,i=1,r=0,a=Math.PI*2,o=!1,l=0){super(),this.isEllipseCurve=!0,this.type="EllipseCurve",this.aX=e,this.aY=t,this.xRadius=n,this.yRadius=i,this.aStartAngle=r,this.aEndAngle=a,this.aClockwise=o,this.aRotation=l}getPoint(e,t=new ze){let n=t,i=Math.PI*2,r=this.aEndAngle-this.aStartAngle,a=Math.abs(r)<Number.EPSILON;for(;r<0;)r+=i;for(;r>i;)r-=i;r<Number.EPSILON&&(a?r=0:r=i),this.aClockwise===!0&&!a&&(r===i?r=-i:r=r-i);let o=this.aStartAngle+e*r,l=this.aX+this.xRadius*Math.cos(o),c=this.aY+this.yRadius*Math.sin(o);if(this.aRotation!==0){let h=Math.cos(this.aRotation),d=Math.sin(this.aRotation),u=l-this.aX,p=c-this.aY;l=u*h-p*d+this.aX,c=u*d+p*h+this.aY}return n.set(l,c)}copy(e){return super.copy(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}toJSON(){let e=super.toJSON();return e.aX=this.aX,e.aY=this.aY,e.xRadius=this.xRadius,e.yRadius=this.yRadius,e.aStartAngle=this.aStartAngle,e.aEndAngle=this.aEndAngle,e.aClockwise=this.aClockwise,e.aRotation=this.aRotation,e}fromJSON(e){return super.fromJSON(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}},ul=class extends Pa{constructor(e,t,n,i,r,a){super(e,t,n,n,i,r,a),this.isArcCurve=!0,this.type="ArcCurve"}};function Kh(){let s=0,e=0,t=0,n=0;function i(r,a,o,l){s=r,e=o,t=-3*r+3*a-2*o-l,n=2*r-2*a+o+l}return{initCatmullRom:function(r,a,o,l,c){i(a,o,c*(o-r),c*(l-a))},initNonuniformCatmullRom:function(r,a,o,l,c,h,d){let u=(a-r)/c-(o-r)/(c+h)+(o-a)/h,p=(o-a)/h-(l-a)/(h+d)+(l-o)/d;u*=h,p*=h,i(a,o,u,p)},calc:function(r){let a=r*r,o=a*r;return s+e*r+t*a+n*o}}}var Wd=new k,qd=new k,gh=new Kh,xh=new Kh,bh=new Kh,Dr=class extends Yn{constructor(e=[],t=!1,n="centripetal",i=.5){super(),this.isCatmullRomCurve3=!0,this.type="CatmullRomCurve3",this.points=e,this.closed=t,this.curveType=n,this.tension=i}getPoint(e,t=new k){let n=t,i=this.points,r=i.length,a=(r-(this.closed?0:1))*e,o=Math.floor(a),l=a-o;this.closed?o+=o>0?0:(Math.floor(Math.abs(o)/r)+1)*r:l===0&&o===r-1&&(o=r-2,l=1);let c,h;this.closed||o>0?c=i[(o-1)%r]:(qd.subVectors(i[0],i[1]).add(i[0]),c=qd);let d=i[o%r],u=i[(o+1)%r];if(this.closed||o+2<r?h=i[(o+2)%r]:(Wd.subVectors(i[r-1],i[r-2]).add(i[r-1]),h=Wd),this.curveType==="centripetal"||this.curveType==="chordal"){let p=this.curveType==="chordal"?.5:.25,x=Math.pow(c.distanceToSquared(d),p),v=Math.pow(d.distanceToSquared(u),p),m=Math.pow(u.distanceToSquared(h),p);v<1e-4&&(v=1),x<1e-4&&(x=v),m<1e-4&&(m=v),gh.initNonuniformCatmullRom(c.x,d.x,u.x,h.x,x,v,m),xh.initNonuniformCatmullRom(c.y,d.y,u.y,h.y,x,v,m),bh.initNonuniformCatmullRom(c.z,d.z,u.z,h.z,x,v,m)}else this.curveType==="catmullrom"&&(gh.initCatmullRom(c.x,d.x,u.x,h.x,this.tension),xh.initCatmullRom(c.y,d.y,u.y,h.y,this.tension),bh.initCatmullRom(c.z,d.z,u.z,h.z,this.tension));return n.set(gh.calc(l),xh.calc(l),bh.calc(l)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e.closed=this.closed,e.curveType=this.curveType,e.tension=this.tension,e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new k().fromArray(i))}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}};function Xd(s,e,t,n,i){let r=(n-e)*.5,a=(i-t)*.5,o=s*s,l=s*o;return(2*t-2*n+r+a)*l+(-3*t+3*n-2*r-a)*o+r*s+t}function Lg(s,e){let t=1-s;return t*t*e}function Fg(s,e){return 2*(1-s)*s*e}function Ng(s,e){return s*s*e}function ga(s,e,t,n){return Lg(s,e)+Fg(s,t)+Ng(s,n)}function Ug(s,e){let t=1-s;return t*t*t*e}function Og(s,e){let t=1-s;return 3*t*t*s*e}function zg(s,e){return 3*(1-s)*s*s*e}function kg(s,e){return s*s*s*e}function xa(s,e,t,n,i){return Ug(s,e)+Og(s,t)+zg(s,n)+kg(s,i)}var dl=class extends Yn{constructor(e=new ze,t=new ze,n=new ze,i=new ze){super(),this.isCubicBezierCurve=!0,this.type="CubicBezierCurve",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new ze){let n=t,i=this.v0,r=this.v1,a=this.v2,o=this.v3;return n.set(xa(e,i.x,r.x,a.x,o.x),xa(e,i.y,r.y,a.y,o.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},fl=class extends Yn{constructor(e=new k,t=new k,n=new k,i=new k){super(),this.isCubicBezierCurve3=!0,this.type="CubicBezierCurve3",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new k){let n=t,i=this.v0,r=this.v1,a=this.v2,o=this.v3;return n.set(xa(e,i.x,r.x,a.x,o.x),xa(e,i.y,r.y,a.y,o.y),xa(e,i.z,r.z,a.z,o.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},pl=class extends Yn{constructor(e=new ze,t=new ze){super(),this.isLineCurve=!0,this.type="LineCurve",this.v1=e,this.v2=t}getPoint(e,t=new ze){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new ze){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Lr=class extends Yn{constructor(e=new k,t=new k){super(),this.isLineCurve3=!0,this.type="LineCurve3",this.v1=e,this.v2=t}getPoint(e,t=new k){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new k){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},ml=class extends Yn{constructor(e=new ze,t=new ze,n=new ze){super(),this.isQuadraticBezierCurve=!0,this.type="QuadraticBezierCurve",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new ze){let n=t,i=this.v0,r=this.v1,a=this.v2;return n.set(ga(e,i.x,r.x,a.x),ga(e,i.y,r.y,a.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},xi=class extends Yn{constructor(e=new k,t=new k,n=new k){super(),this.isQuadraticBezierCurve3=!0,this.type="QuadraticBezierCurve3",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new k){let n=t,i=this.v0,r=this.v1,a=this.v2;return n.set(ga(e,i.x,r.x,a.x),ga(e,i.y,r.y,a.y),ga(e,i.z,r.z,a.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},gl=class extends Yn{constructor(e=[]){super(),this.isSplineCurve=!0,this.type="SplineCurve",this.points=e}getPoint(e,t=new ze){let n=t,i=this.points,r=(i.length-1)*e,a=Math.floor(r),o=r-a,l=i[a===0?a:a-1],c=i[a],h=i[a>i.length-2?i.length-1:a+1],d=i[a>i.length-3?i.length-1:a+2];return n.set(Xd(o,l.x,c.x,h.x,d.x),Xd(o,l.y,c.y,h.y,d.y)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new ze().fromArray(i))}return this}},Sh=Object.freeze({__proto__:null,ArcCurve:ul,CatmullRomCurve3:Dr,CubicBezierCurve:dl,CubicBezierCurve3:fl,EllipseCurve:Pa,LineCurve:pl,LineCurve3:Lr,QuadraticBezierCurve:ml,QuadraticBezierCurve3:xi,SplineCurve:gl}),Ia=class extends Yn{constructor(){super(),this.type="CurvePath",this.curves=[],this.autoClose=!1}add(e){this.curves.push(e)}closePath(){let e=this.curves[0].getPoint(0),t=this.curves[this.curves.length-1].getPoint(1);if(!e.equals(t)){let n=e.isVector2===!0?"LineCurve":"LineCurve3";this.curves.push(new Sh[n](t,e))}return this}getPoint(e,t){let n=e*this.getLength(),i=this.getCurveLengths(),r=0;for(;r<i.length;){if(i[r]>=n){let a=i[r]-n,o=this.curves[r],l=o.getLength(),c=l===0?0:1-a/l;return o.getPointAt(c,t)}r++}return null}getLength(){let e=this.getCurveLengths();return e[e.length-1]}updateArcLengths(){this.needsUpdate=!0,this.cacheLengths=null,this.getCurveLengths()}getCurveLengths(){if(this.cacheLengths&&this.cacheLengths.length===this.curves.length)return this.cacheLengths;let e=[],t=0;for(let n=0,i=this.curves.length;n<i;n++)t+=this.curves[n].getLength(),e.push(t);return this.cacheLengths=e,e}getSpacedPoints(e=40){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return this.autoClose&&t.push(t[0]),t}getPoints(e=12){let t=[],n;for(let i=0,r=this.curves;i<r.length;i++){let a=r[i],o=a.isEllipseCurve?e*2:a.isLineCurve||a.isLineCurve3?1:a.isSplineCurve?e*a.points.length:e,l=a.getPoints(o);for(let c=0;c<l.length;c++){let h=l[c];n&&n.equals(h)||(t.push(h),n=h)}}return this.autoClose&&t.length>1&&!t[t.length-1].equals(t[0])&&t.push(t[0]),t}copy(e){super.copy(e),this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(i.clone())}return this.autoClose=e.autoClose,this}toJSON(){let e=super.toJSON();e.autoClose=this.autoClose,e.curves=[];for(let t=0,n=this.curves.length;t<n;t++){let i=this.curves[t];e.curves.push(i.toJSON())}return e}fromJSON(e){super.fromJSON(e),this.autoClose=e.autoClose,this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(new Sh[i.type]().fromJSON(i))}return this}};var Fr=class s extends hl{constructor(e=1,t=0){let n=(1+Math.sqrt(5))/2,i=[-1,n,0,1,n,0,-1,-n,0,1,-n,0,0,-1,n,0,1,n,0,-1,-n,0,1,-n,n,0,-1,n,0,1,-n,0,-1,-n,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(i,r,e,t),this.type="IcosahedronGeometry",this.parameters={radius:e,detail:t}}static fromJSON(e){return new s(e.radius,e.detail)}};var un=class s extends Mt{constructor(e=1,t=1,n=1,i=1){super(),this.type="PlaneGeometry",this.parameters={width:e,height:t,widthSegments:n,heightSegments:i};let r=e/2,a=t/2,o=Math.floor(n),l=Math.floor(i),c=o+1,h=l+1,d=e/o,u=t/l,p=[],x=[],v=[],m=[];for(let f=0;f<h;f++){let y=f*u-a;for(let T=0;T<c;T++){let g=T*d-r;x.push(g,-y,0),v.push(0,0,1),m.push(T/o),m.push(1-f/l)}}for(let f=0;f<l;f++)for(let y=0;y<o;y++){let T=y+c*f,g=y+c*(f+1),_=y+1+c*(f+1),S=y+1+c*f;p.push(T,g,S),p.push(g,_,S)}this.setIndex(p),this.setAttribute("position",new xt(x,3)),this.setAttribute("normal",new xt(v,3)),this.setAttribute("uv",new xt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.widthSegments,e.heightSegments)}},xs=class s extends Mt{constructor(e=.5,t=1,n=32,i=1,r=0,a=Math.PI*2){super(),this.type="RingGeometry",this.parameters={innerRadius:e,outerRadius:t,thetaSegments:n,phiSegments:i,thetaStart:r,thetaLength:a},n=Math.max(3,n),i=Math.max(1,i);let o=[],l=[],c=[],h=[],d=e,u=(t-e)/i,p=new k,x=new ze;for(let v=0;v<=i;v++){for(let m=0;m<=n;m++){let f=r+m/n*a;p.x=d*Math.cos(f),p.y=d*Math.sin(f),l.push(p.x,p.y,p.z),c.push(0,0,1),x.x=(p.x/t+1)/2,x.y=(p.y/t+1)/2,h.push(x.x,x.y)}d+=u}for(let v=0;v<i;v++){let m=v*(n+1);for(let f=0;f<n;f++){let y=f+m,T=y,g=y+n+1,_=y+n+2,S=y+1;o.push(T,g,S),o.push(g,_,S)}}this.setIndex(o),this.setAttribute("position",new xt(l,3)),this.setAttribute("normal",new xt(c,3)),this.setAttribute("uv",new xt(h,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.innerRadius,e.outerRadius,e.thetaSegments,e.phiSegments,e.thetaStart,e.thetaLength)}};var Ii=class s extends Mt{constructor(e=1,t=32,n=16,i=0,r=Math.PI*2,a=0,o=Math.PI){super(),this.type="SphereGeometry",this.parameters={radius:e,widthSegments:t,heightSegments:n,phiStart:i,phiLength:r,thetaStart:a,thetaLength:o},t=Math.max(3,Math.floor(t)),n=Math.max(2,Math.floor(n));let l=Math.min(a+o,Math.PI),c=0,h=[],d=new k,u=new k,p=[],x=[],v=[],m=[];for(let f=0;f<=n;f++){let y=[],T=f/n,g=a+T*o,_=e*Math.cos(g),S=Math.sqrt(e*e-_*_),A=0;f===0&&a===0?A=.5/t:f===n&&l===Math.PI&&(A=-.5/t);for(let b=0;b<=t;b++){let C=b/t,w=i+C*r;d.x=-S*Math.cos(w),d.y=_,d.z=S*Math.sin(w),x.push(d.x,d.y,d.z),u.copy(d).normalize(),v.push(u.x,u.y,u.z),m.push(C+A,1-T),y.push(c++)}h.push(y)}for(let f=0;f<n;f++)for(let y=0;y<t;y++){let T=h[f][y+1],g=h[f][y],_=h[f+1][y],S=h[f+1][y+1];(f!==0||a>0)&&p.push(T,g,S),(f!==n-1||l<Math.PI)&&p.push(g,_,S)}this.setIndex(p),this.setAttribute("position",new xt(x,3)),this.setAttribute("normal",new xt(v,3)),this.setAttribute("uv",new xt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.widthSegments,e.heightSegments,e.phiStart,e.phiLength,e.thetaStart,e.thetaLength)}};var Jn=class s extends Mt{constructor(e=1,t=.4,n=12,i=48,r=Math.PI*2,a=0,o=Math.PI*2){super(),this.type="TorusGeometry",this.parameters={radius:e,tube:t,radialSegments:n,tubularSegments:i,arc:r,thetaStart:a,thetaLength:o},n=Math.floor(n),i=Math.floor(i);let l=[],c=[],h=[],d=[],u=new k,p=new k,x=new k;for(let v=0;v<=n;v++){let m=a+v/n*o;for(let f=0;f<=i;f++){let y=f/i*r;p.x=(e+t*Math.cos(m))*Math.cos(y),p.y=(e+t*Math.cos(m))*Math.sin(y),p.z=t*Math.sin(m),c.push(p.x,p.y,p.z),u.x=e*Math.cos(y),u.y=e*Math.sin(y),x.subVectors(p,u).normalize(),h.push(x.x,x.y,x.z),d.push(f/i),d.push(v/n)}}for(let v=1;v<=n;v++)for(let m=1;m<=i;m++){let f=(i+1)*v+m-1,y=(i+1)*(v-1)+m-1,T=(i+1)*(v-1)+m,g=(i+1)*v+m;l.push(f,y,g),l.push(y,T,g)}this.setIndex(l),this.setAttribute("position",new xt(c,3)),this.setAttribute("normal",new xt(h,3)),this.setAttribute("uv",new xt(d,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.tube,e.radialSegments,e.tubularSegments,e.arc,e.thetaStart,e.thetaLength)}};var Da=class s extends Mt{constructor(e=new xi(new k(-1,-1,0),new k(-1,1,0),new k(1,1,0)),t=64,n=1,i=8,r=!1){super(),this.type="TubeGeometry",this.parameters={path:e,tubularSegments:t,radius:n,radialSegments:i,closed:r};let a=e.computeFrenetFrames(t,r);this.tangents=a.tangents,this.normals=a.normals,this.binormals=a.binormals;let o=new k,l=new k,c=new ze,h=new k,d=[],u=[],p=[],x=[];v(),this.setIndex(x),this.setAttribute("position",new xt(d,3)),this.setAttribute("normal",new xt(u,3)),this.setAttribute("uv",new xt(p,2));function v(){for(let T=0;T<t;T++)m(T);m(r===!1?t:0),y(),f()}function m(T){h=e.getPointAt(T/t,h);let g=a.normals[T],_=a.binormals[T];for(let S=0;S<=i;S++){let A=S/i*Math.PI*2,b=Math.sin(A),C=-Math.cos(A);l.x=C*g.x+b*_.x,l.y=C*g.y+b*_.y,l.z=C*g.z+b*_.z,l.normalize(),u.push(l.x,l.y,l.z),o.x=h.x+n*l.x,o.y=h.y+n*l.y,o.z=h.z+n*l.z,d.push(o.x,o.y,o.z)}}function f(){for(let T=1;T<=t;T++)for(let g=1;g<=i;g++){let _=(i+1)*(T-1)+(g-1),S=(i+1)*T+(g-1),A=(i+1)*T+g,b=(i+1)*(T-1)+g;x.push(_,S,b),x.push(S,A,b)}}function y(){for(let T=0;T<=t;T++)for(let g=0;g<=i;g++)c.x=T/t,c.y=g/i,p.push(c.x,c.y)}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}toJSON(){let e=super.toJSON();return e.path=this.parameters.path.toJSON(),e}static fromJSON(e){return new s(new Sh[e.path.type]().fromJSON(e.path),e.tubularSegments,e.radius,e.radialSegments,e.closed)}},La=class extends Mt{constructor(e=null){if(super(),this.type="WireframeGeometry",this.parameters={geometry:e},e!==null){let t=[],n=new Set,i=new k,r=new k;if(e.index!==null){let a=e.attributes.position,o=e.index,l=e.groups;l.length===0&&(l=[{start:0,count:o.count,materialIndex:0}]);for(let c=0,h=l.length;c<h;++c){let d=l[c],u=d.start,p=d.count;for(let x=u,v=u+p;x<v;x+=3)for(let m=0;m<3;m++){let f=o.getX(x+m),y=o.getX(x+(m+1)%3);i.fromBufferAttribute(a,f),r.fromBufferAttribute(a,y),jd(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}}else{let a=e.attributes.position;for(let o=0,l=a.count/3;o<l;o++)for(let c=0;c<3;c++){let h=3*o+c,d=3*o+(c+1)%3;i.fromBufferAttribute(a,h),r.fromBufferAttribute(a,d),jd(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}this.setAttribute("position",new xt(t,3))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}};function jd(s,e,t){let n=`${s.x},${s.y},${s.z}-${e.x},${e.y},${e.z}`,i=`${e.x},${e.y},${e.z}-${s.x},${s.y},${s.z}`;return t.has(n)===!0||t.has(i)===!0?!1:(t.add(n),t.add(i),!0)}function Js(s){let e={};for(let t in s){e[t]={};for(let n in s[t]){let i=s[t][n];if(Kd(i))i.isRenderTargetTexture?(ht("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),e[t][n]=null):e[t][n]=i.clone();else if(Array.isArray(i))if(Kd(i[0])){let r=[];for(let a=0,o=i.length;a<o;a++)r[a]=i[a].clone();e[t][n]=r}else e[t][n]=i.slice();else e[t][n]=i}}return e}function Dn(s){let e={};for(let t=0;t<s.length;t++){let n=Js(s[t]);for(let i in n)e[i]=n[i]}return e}function Kd(s){return s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)}function Bg(s){let e=[];for(let t=0;t<s.length;t++)e.push(s[t].clone());return e}function Yh(s){let e=s.getRenderTarget();return e===null?s.outputColorSpace:e.isXRRenderTarget===!0?e.texture.colorSpace:Rt.workingColorSpace}var Mi={clone:Js,merge:Dn},Hg=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,Gg=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,wt=class extends zn{constructor(e){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=Hg,this.fragmentShader=Gg,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,e!==void 0&&this.setValues(e)}copy(e){return super.copy(e),this.fragmentShader=e.fragmentShader,this.vertexShader=e.vertexShader,this.uniforms=Js(e.uniforms),this.uniformsGroups=Bg(e.uniformsGroups),this.defines=Object.assign({},e.defines),this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.fog=e.fog,this.lights=e.lights,this.clipping=e.clipping,this.extensions=Object.assign({},e.extensions),this.glslVersion=e.glslVersion,this.defaultAttributeValues=Object.assign({},e.defaultAttributeValues),this.index0AttributeName=e.index0AttributeName,this.uniformsNeedUpdate=e.uniformsNeedUpdate,this}toJSON(e){let t=super.toJSON(e);t.glslVersion=this.glslVersion,t.uniforms={};for(let i in this.uniforms){let a=this.uniforms[i].value;a&&a.isTexture?t.uniforms[i]={type:"t",value:a.toJSON(e).uuid}:a&&a.isColor?t.uniforms[i]={type:"c",value:a.getHex()}:a&&a.isVector2?t.uniforms[i]={type:"v2",value:a.toArray()}:a&&a.isVector3?t.uniforms[i]={type:"v3",value:a.toArray()}:a&&a.isVector4?t.uniforms[i]={type:"v4",value:a.toArray()}:a&&a.isMatrix3?t.uniforms[i]={type:"m3",value:a.toArray()}:a&&a.isMatrix4?t.uniforms[i]={type:"m4",value:a.toArray()}:t.uniforms[i]={value:a}}Object.keys(this.defines).length>0&&(t.defines=this.defines),t.vertexShader=this.vertexShader,t.fragmentShader=this.fragmentShader,t.lights=this.lights,t.clipping=this.clipping;let n={};for(let i in this.extensions)this.extensions[i]===!0&&(n[i]=!0);return Object.keys(n).length>0&&(t.extensions=n),t}fromJSON(e,t){if(super.fromJSON(e,t),e.uniforms!==void 0)for(let n in e.uniforms){let i=e.uniforms[n];switch(this.uniforms[n]={},i.type){case"t":this.uniforms[n].value=t[i.value]||null;break;case"c":this.uniforms[n].value=new Ce().setHex(i.value);break;case"v2":this.uniforms[n].value=new ze().fromArray(i.value);break;case"v3":this.uniforms[n].value=new k().fromArray(i.value);break;case"v4":this.uniforms[n].value=new Nt().fromArray(i.value);break;case"m3":this.uniforms[n].value=new _t().fromArray(i.value);break;case"m4":this.uniforms[n].value=new mt().fromArray(i.value);break;default:this.uniforms[n].value=i.value}}if(e.defines!==void 0&&(this.defines=e.defines),e.vertexShader!==void 0&&(this.vertexShader=e.vertexShader),e.fragmentShader!==void 0&&(this.fragmentShader=e.fragmentShader),e.glslVersion!==void 0&&(this.glslVersion=e.glslVersion),e.extensions!==void 0)for(let n in e.extensions)this.extensions[n]=e.extensions[n];return e.lights!==void 0&&(this.lights=e.lights),e.clipping!==void 0&&(this.clipping=e.clipping),this}},Nr=class extends wt{constructor(e){super(e),this.isRawShaderMaterial=!0,this.type="RawShaderMaterial"}},An=class extends zn{constructor(e){super(),this.isMeshStandardMaterial=!0,this.type="MeshStandardMaterial",this.defines={STANDARD:""},this.color=new Ce(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Ce(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=ao,this.normalScale=new ze(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new kn,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.defines={STANDARD:""},this.color.copy(e.color),this.roughness=e.roughness,this.metalness=e.metalness,this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.roughnessMap=e.roughnessMap,this.metalnessMap=e.metalnessMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.envMapIntensity=e.envMapIntensity,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},Bn=class extends An{constructor(e){super(),this.isMeshPhysicalMaterial=!0,this.defines={STANDARD:"",PHYSICAL:""},this.type="MeshPhysicalMaterial",this.anisotropyRotation=0,this.anisotropyMap=null,this.clearcoatMap=null,this.clearcoatRoughness=0,this.clearcoatRoughnessMap=null,this.clearcoatNormalScale=new ze(1,1),this.clearcoatNormalMap=null,this.ior=1.5,Object.defineProperty(this,"reflectivity",{get:function(){return Ct(2.5*(this.ior-1)/(this.ior+1),0,1)},set:function(t){this.ior=(1+.4*t)/(1-.4*t)}}),this.iridescenceMap=null,this.iridescenceIOR=1.3,this.iridescenceThicknessRange=[100,400],this.iridescenceThicknessMap=null,this.sheenColor=new Ce(0),this.sheenColorMap=null,this.sheenRoughness=1,this.sheenRoughnessMap=null,this.transmissionMap=null,this.thickness=0,this.thicknessMap=null,this.attenuationDistance=1/0,this.attenuationColor=new Ce(1,1,1),this.specularIntensity=1,this.specularIntensityMap=null,this.specularColor=new Ce(1,1,1),this.specularColorMap=null,this._anisotropy=0,this._clearcoat=0,this._dispersion=0,this._iridescence=0,this._retroreflectivity=0,this._sheen=0,this._transmission=0,this.setValues(e)}get anisotropy(){return this._anisotropy}set anisotropy(e){this._anisotropy>0!=e>0&&this.version++,this._anisotropy=e}get clearcoat(){return this._clearcoat}set clearcoat(e){this._clearcoat>0!=e>0&&this.version++,this._clearcoat=e}get iridescence(){return this._iridescence}set iridescence(e){this._iridescence>0!=e>0&&this.version++,this._iridescence=e}get dispersion(){return this._dispersion}set dispersion(e){this._dispersion>0!=e>0&&this.version++,this._dispersion=e}get retroreflectivity(){return this._retroreflectivity}set retroreflectivity(e){this._retroreflectivity>0!=e>0&&this.version++,this._retroreflectivity=e}get sheen(){return this._sheen}set sheen(e){this._sheen>0!=e>0&&this.version++,this._sheen=e}get transmission(){return this._transmission}set transmission(e){this._transmission>0!=e>0&&this.version++,this._transmission=e}copy(e){return super.copy(e),this.defines={STANDARD:"",PHYSICAL:""},this.anisotropy=e.anisotropy,this.anisotropyRotation=e.anisotropyRotation,this.anisotropyMap=e.anisotropyMap,this.clearcoat=e.clearcoat,this.clearcoatMap=e.clearcoatMap,this.clearcoatRoughness=e.clearcoatRoughness,this.clearcoatRoughnessMap=e.clearcoatRoughnessMap,this.clearcoatNormalMap=e.clearcoatNormalMap,this.clearcoatNormalScale.copy(e.clearcoatNormalScale),this.dispersion=e.dispersion,this.ior=e.ior,this.iridescence=e.iridescence,this.iridescenceMap=e.iridescenceMap,this.iridescenceIOR=e.iridescenceIOR,this.iridescenceThicknessRange=[...e.iridescenceThicknessRange],this.iridescenceThicknessMap=e.iridescenceThicknessMap,this.retroreflectivity=e.retroreflectivity,this.sheen=e.sheen,this.sheenColor.copy(e.sheenColor),this.sheenColorMap=e.sheenColorMap,this.sheenRoughness=e.sheenRoughness,this.sheenRoughnessMap=e.sheenRoughnessMap,this.transmission=e.transmission,this.transmissionMap=e.transmissionMap,this.thickness=e.thickness,this.thicknessMap=e.thicknessMap,this.attenuationDistance=e.attenuationDistance,this.attenuationColor.copy(e.attenuationColor),this.specularIntensity=e.specularIntensity,this.specularIntensityMap=e.specularIntensityMap,this.specularColor.copy(e.specularColor),this.specularColorMap=e.specularColorMap,this}};var Fa=class extends zn{constructor(e){super(),this.isMeshLambertMaterial=!0,this.type="MeshLambertMaterial",this.color=new Ce(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Ce(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=ao,this.normalScale=new ze(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new kn,this.combine=Cl,this.reflectivity=1,this.envMapIntensity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.envMapIntensity=e.envMapIntensity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},xl=class extends zn{constructor(e){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=Df,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(e)}copy(e){return super.copy(e),this.depthPacking=e.depthPacking,this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this}},bl=class extends zn{constructor(e){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(e)}copy(e){return super.copy(e),this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this}};function ds(s,e){return!s||s.constructor===e?s:typeof e.BYTES_PER_ELEMENT=="number"?new e(s):Array.prototype.slice.call(s)}function Jo(s){return s!==void 0&&s.inTangents!==void 0&&s.outTangents!==void 0}function Vg(s){function e(i,r){return s[i]-s[r]}let t=s.length,n=new Array(t);for(let i=0;i!==t;++i)n[i]=i;return n.sort(e),n}function Yd(s,e,t){let n=s.length,i=new s.constructor(n);for(let r=0,a=0;a!==n;++r){let o=t[r]*e;for(let l=0;l!==e;++l)i[a++]=s[o+l]}return i}function Wg(s,e,t,n){let i=1,r=s[0];for(;r!==void 0&&r[n]===void 0;)r=s[i++];if(r===void 0)return;let a=r[n];if(a!==void 0)if(Array.isArray(a))do a=r[n],a!==void 0&&(e.push(r.time),t.push(...a)),r=s[i++];while(r!==void 0);else if(a.toArray!==void 0)do a=r[n],a!==void 0&&(e.push(r.time),a.toArray(t,t.length)),r=s[i++];while(r!==void 0);else do a=r[n],a!==void 0&&(e.push(r.time),t.push(a)),r=s[i++];while(r!==void 0)}var Di=class{constructor(e,t,n,i){this.parameterPositions=e,this._cachedIndex=0,this.resultBuffer=i!==void 0?i:new t.constructor(n),this.sampleValues=t,this.valueSize=n,this.settings=null,this.DefaultSettings_={}}evaluate(e){let t=this.parameterPositions,n=this._cachedIndex,i=t[n],r=t[n-1];e:{t:{let a;n:{i:if(!(e<i)){for(let o=n+2;;){if(i===void 0){if(e<r)break i;return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}if(n===o)break;if(r=i,i=t[++n],e<i)break t}a=t.length;break n}if(!(e>=r)){let o=t[1];e<o&&(n=2,r=o);for(let l=n-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(n===l)break;if(i=r,r=t[--n-1],e>=r)break t}a=n,n=0;break n}break e}for(;n<a;){let o=n+a>>>1;e<t[o]?a=o:n=o+1}if(i=t[n],r=t[n-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===void 0)return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}this._cachedIndex=n,this.intervalChanged_(n,r,i)}return this.interpolate_(n,r,e,i)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i;for(let a=0;a!==i;++a)t[a]=n[r+a];return t}interpolate_(){throw new Error("THREE.Interpolant: Call to abstract method.")}intervalChanged_(){}},vl=class extends Di{constructor(e,t,n,i){super(e,t,n,i),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:Ls,endingEnd:Ls}}intervalChanged_(e,t,n){let i=this.parameterPositions,r=e-2,a=e+1,o=i[r],l=i[a];if(o===void 0)switch(this.getSettings_().endingStart){case Fs:r=e,o=2*t-n;break;case ba:r=i.length-2,o=t+i[r]-i[r+1];break;default:r=e,o=n}if(l===void 0)switch(this.getSettings_().endingEnd){case Fs:a=e,l=2*n-t;break;case ba:a=1,l=n+i[1]-i[0];break;default:a=e-1,l=t}let c=(n-t)*.5,h=this.valueSize;this._weightPrev=c/(t-o),this._weightNext=c/(l-n),this._offsetPrev=r*h,this._offsetNext=a*h}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=this._offsetPrev,d=this._offsetNext,u=this._weightPrev,p=this._weightNext,x=(n-t)/(i-t),v=x*x,m=v*x,f=-u*m+2*u*v-u*x,y=(1+u)*m+(-1.5-2*u)*v+(-.5+u)*x+1,T=(-1-p)*m+(1.5+p)*v+.5*x,g=p*m-p*v;for(let _=0;_!==o;++_)r[_]=f*a[h+_]+y*a[c+_]+T*a[l+_]+g*a[d+_];return r}},Na=class extends Di{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=(n-t)/(i-t),d=1-h;for(let u=0;u!==o;++u)r[u]=a[c+u]*d+a[l+u]*h;return r}},_l=class extends Di{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e){return this.copySampleValue_(e-1)}},yl=class extends Di{interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=this.inTangents,d=this.outTangents;if(!h||!d){let x=(n-t)/(i-t),v=1-x;for(let m=0;m!==o;++m)r[m]=a[c+m]*v+a[l+m]*x;return r}let u=o*2,p=e-1;for(let x=0;x!==o;++x){let v=a[c+x],m=a[l+x],f=p*u+x*2,y=d[f],T=d[f+1],g=e*u+x*2,_=h[g],S=h[g+1],A=Xg(n,t,y,_,i);r[x]=Xf(A,v,T,S,m)}return r}};function Xf(s,e,t,n,i){let r=1-s;return r*r*r*e+3*r*r*s*t+3*r*s*s*n+s*s*s*i}function qg(s,e,t,n,i){let r=1-s;return 3*r*r*(t-e)+6*r*s*(n-t)+3*s*s*(i-n)}function Xg(s,e,t,n,i){let r=(s-e)/(i-e);for(let a=0;a<8;a++){let o=Xf(r,e,t,n,i)-s;if(Math.abs(o)<1e-10)break;let l=qg(r,e,t,n,i);if(Math.abs(l)<1e-10)break;r=Math.max(0,Math.min(1,r-o/l))}return r}var Hn=class{constructor(e,t,n,i){if(e===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(t===void 0||t.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+e);this.name=e,this.times=ds(t,this.TimeBufferType),this.values=ds(n,this.ValueBufferType),this.setInterpolation(i||this.DefaultInterpolation)}static toJSON(e){let t=e.constructor,n;if(t.toJSON!==this.toJSON)n=t.toJSON(e);else{n={name:e.name,times:ds(e.times,Array),values:ds(e.values,Array)};let i=e.getInterpolation();i!==e.DefaultInterpolation&&(n.interpolation=i),Jo(e.settings)&&(n.settings={inTangents:ds(e.settings.inTangents,Array),outTangents:ds(e.settings.outTangents,Array)})}return n.type=e.ValueTypeName,n}InterpolantFactoryMethodDiscrete(e){return new _l(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodLinear(e){return new Na(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodSmooth(e){return new vl(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodBezier(e){let t=new yl(this.times,this.values,this.getValueSize(),e);return this.settings&&(t.inTangents=this.settings.inTangents,t.outTangents=this.settings.outTangents),t}setInterpolation(e){let t;switch(e){case Us:t=this.InterpolantFactoryMethodDiscrete;break;case Os:t=this.InterpolantFactoryMethodLinear;break;case Ko:t=this.InterpolantFactoryMethodSmooth;break;case Mh:t=this.InterpolantFactoryMethodBezier;break}if(t===void 0){let n="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(e!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(n);return ht("KeyframeTrack:",n),this}return this.createInterpolant=t,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return Us;case this.InterpolantFactoryMethodLinear:return Os;case this.InterpolantFactoryMethodSmooth:return Ko;case this.InterpolantFactoryMethodBezier:return Mh}}getValueSize(){return this.values.length/this.times.length}shift(e){if(e!==0){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]+=e}return this}scale(e){if(e!==1){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]*=e;Jo(this.settings)&&(Jd(this.settings.inTangents,e),Jd(this.settings.outTangents,e))}return this}trim(e,t){let n=this.times,i=n.length,r=0,a=i-1;for(;r!==i&&n[r]<e;)++r;for(;a!==-1&&n[a]>t;)--a;if(++a,r!==0||a!==i){r>=a&&(a=Math.max(a,1),r=a-1);let o=this.getValueSize();this.times=n.slice(r,a),this.values=this.values.slice(r*o,a*o)}return this}validate(){let e=!0,t=this.getValueSize();t-Math.floor(t)!==0&&(gt("KeyframeTrack: Invalid value size in track.",this),e=!1);let n=this.times,i=this.values,r=n.length;r===0&&(gt("KeyframeTrack: Track is empty.",this),e=!1);let a=null;for(let o=0;o!==r;o++){let l=n[o];if(typeof l=="number"&&isNaN(l)){gt("KeyframeTrack: Time is not a valid number.",this,o,l),e=!1;break}if(a!==null&&a>l){gt("KeyframeTrack: Out of order keys.",this,o,l,a),e=!1;break}a=l}if(i!==void 0&&Ym(i))for(let o=0,l=i.length;o!==l;++o){let c=i[o];if(isNaN(c)){gt("KeyframeTrack: Value is not a valid number.",this,o,c),e=!1;break}}return e}optimize(){let e=this.times.slice(),t=this.values.slice(),n=this.getValueSize(),i=this.getInterpolation()===Ko,r=e.length-1,a=1;for(let o=1;o<r;++o){let l=!1,c=e[o],h=e[o+1];if(c!==h&&(o!==1||c!==e[0]))if(i)l=!0;else{let d=o*n,u=d-n,p=d+n;for(let x=0;x!==n;++x){let v=t[d+x];if(v!==t[u+x]||v!==t[p+x]){l=!0;break}}}if(l){if(o!==a){e[a]=e[o];let d=o*n,u=a*n;for(let p=0;p!==n;++p)t[u+p]=t[d+p]}++a}}if(r>0){e[a]=e[r];for(let o=r*n,l=a*n,c=0;c!==n;++c)t[l+c]=t[o+c];++a}return a!==e.length?(this.times=e.slice(0,a),this.values=t.slice(0,a*n)):(this.times=e,this.values=t),this}clone(){let e=this.times.slice(),t=this.values.slice(),n=this.constructor,i=new n(this.name,e,t);return i.createInterpolant=this.createInterpolant,Jo(this.settings)&&(i.settings={inTangents:this.settings.inTangents.slice(),outTangents:this.settings.outTangents.slice()}),i}};function Jd(s,e){for(let t=0,n=s.length;t!==n;t+=2)s[t]*=e}Hn.prototype.ValueTypeName="";Hn.prototype.TimeBufferType=Float32Array;Hn.prototype.ValueBufferType=Float32Array;Hn.prototype.DefaultInterpolation=Os;var Yi=class extends Hn{constructor(e,t,n){super(e,t,n)}};Yi.prototype.ValueTypeName="bool";Yi.prototype.ValueBufferType=Array;Yi.prototype.DefaultInterpolation=Us;Yi.prototype.InterpolantFactoryMethodLinear=void 0;Yi.prototype.InterpolantFactoryMethodSmooth=void 0;var Ua=class extends Hn{constructor(e,t,n,i){super(e,t,n,i)}};Ua.prototype.ValueTypeName="color";var Ji=class extends Hn{constructor(e,t,n,i){super(e,t,n,i)}};Ji.prototype.ValueTypeName="number";var Ml=class extends Di{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=(n-t)/(i-t),c=e*o;for(let h=c+o;c!==h;c+=4)$t.slerpFlat(r,0,a,c-o,a,c,l);return r}},Zi=class extends Hn{constructor(e,t,n,i){super(e,t,n,i)}InterpolantFactoryMethodLinear(e){return new Ml(this.times,this.values,this.getValueSize(),e)}};Zi.prototype.ValueTypeName="quaternion";Zi.prototype.InterpolantFactoryMethodSmooth=void 0;var Qi=class extends Hn{constructor(e,t,n){super(e,t,n)}};Qi.prototype.ValueTypeName="string";Qi.prototype.ValueBufferType=Array;Qi.prototype.DefaultInterpolation=Us;Qi.prototype.InterpolantFactoryMethodLinear=void 0;Qi.prototype.InterpolantFactoryMethodSmooth=void 0;var bs=class extends Hn{constructor(e,t,n,i){super(e,t,n,i)}};bs.prototype.ValueTypeName="vector";var Vs=class{constructor(e="",t=-1,n=[],i=xc){this.name=e,this.tracks=n,this.duration=t,this.blendMode=i,this.uuid=pi(),this.userData={},this.duration<0&&this.resetDuration()}static parse(e){let t=[],n=e.tracks,i=1/(e.fps||1);for(let a=0,o=n.length;a!==o;++a)t.push(Kg(n[a]).scale(i));let r=new this(e.name,e.duration,t,e.blendMode);return r.uuid=e.uuid,r.userData=JSON.parse(e.userData||"{}"),r}static toJSON(e){let t=[],n=e.tracks,i={name:e.name,duration:e.duration,tracks:t,uuid:e.uuid,blendMode:e.blendMode,userData:JSON.stringify(e.userData)};for(let r=0,a=n.length;r!==a;++r)t.push(Hn.toJSON(n[r]));return i}static CreateFromMorphTargetSequence(e,t,n,i){let r=t.length,a=[];for(let o=0;o<r;o++){let l=[],c=[];l.push((o+r-1)%r,o,(o+1)%r),c.push(0,1,0);let h=Vg(l);l=Yd(l,1,h),c=Yd(c,1,h),!i&&l[0]===0&&(l.push(r),c.push(c[0])),a.push(new Ji(".morphTargetInfluences["+t[o].name+"]",l,c).scale(1/n))}return new this(e,-1,a)}static findByName(e,t){let n=e;if(!Array.isArray(e)){let i=e;n=i.geometry&&i.geometry.animations||i.animations}for(let i=0;i<n.length;i++)if(n[i].name===t)return n[i];return null}static CreateClipsFromMorphTargetSequences(e,t,n){let i={},r=/^([\w-]*?)([\d]+)$/;for(let o=0,l=e.length;o<l;o++){let c=e[o],h=c.name.match(r);if(h&&h.length>1){let d=h[1],u=i[d];u||(i[d]=u=[]),u.push(c)}}let a=[];for(let o in i)a.push(this.CreateFromMorphTargetSequence(o,i[o],t,n));return a}resetDuration(){let e=this.tracks,t=0;for(let n=0,i=e.length;n!==i;++n){let r=this.tracks[n];t=Math.max(t,r.times[r.times.length-1])}return this.duration=t,this}trim(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].trim(0,this.duration);return this}validate(){let e=!0;for(let t=0;t<this.tracks.length;t++)e=e&&this.tracks[t].validate();return e}optimize(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].optimize();return this}clone(){let e=[];for(let n=0;n<this.tracks.length;n++)e.push(this.tracks[n].clone());let t=new this.constructor(this.name,this.duration,e,this.blendMode);return t.userData=JSON.parse(JSON.stringify(this.userData)),t}toJSON(){return this.constructor.toJSON(this)}};function jg(s){switch(s.toLowerCase()){case"scalar":case"double":case"float":case"number":case"integer":return Ji;case"vector":case"vector2":case"vector3":case"vector4":return bs;case"color":return Ua;case"quaternion":return Zi;case"bool":case"boolean":return Yi;case"string":return Qi}throw new Error("THREE.KeyframeTrack: Unsupported typeName: "+s)}function Kg(s){if(s.type===void 0)throw new Error("THREE.KeyframeTrack: track type undefined, can not parse");let e=jg(s.type);if(s.times===void 0){let n=[],i=[];Wg(s.keys,n,i,"value"),s.times=n,s.values=i}let t;return e.parse!==void 0?t=e.parse(s):t=new e(s.name,s.times,s.values,s.interpolation),Jo(s.settings)&&(t.settings={inTangents:ds(s.settings.inTangents,Float32Array),outTangents:ds(s.settings.outTangents,Float32Array)}),t}var Ti={enabled:!1,files:{},add:function(s,e){this.enabled!==!1&&(Zd(s)||(this.files[s]=e))},get:function(s){if(this.enabled!==!1&&!Zd(s))return this.files[s]},remove:function(s){delete this.files[s]},clear:function(){this.files={}}};function Zd(s){try{let e=s.slice(s.indexOf(":")+1);return new URL(e).protocol==="blob:"}catch{return!1}}var wl=class{constructor(e,t,n){let i=this,r=!1,a=0,o=0,l,c=[];this.onStart=void 0,this.onLoad=e,this.onProgress=t,this.onError=n,this._abortController=null,this.itemStart=function(h){o++,r===!1&&i.onStart!==void 0&&i.onStart(h,a,o),r=!0},this.itemEnd=function(h){a++,i.onProgress!==void 0&&i.onProgress(h,a,o),a===o&&(r=!1,i.onLoad!==void 0&&i.onLoad())},this.itemError=function(h){i.onError!==void 0&&i.onError(h)},this.resolveURL=function(h){return h=h.normalize("NFC"),l?l(h):h},this.setURLModifier=function(h){return l=h,this},this.addHandler=function(h,d){return c.push(h,d),this},this.removeHandler=function(h){let d=c.indexOf(h);return d!==-1&&c.splice(d,2),this},this.getHandler=function(h){for(let d=0,u=c.length;d<u;d+=2){let p=c[d],x=c[d+1];if(p.global&&(p.lastIndex=0),p.test(h))return x}return null},this.abort=function(){return this.abortController.abort(),this._abortController=null,this}}get abortController(){return this._abortController||(this._abortController=new AbortController),this._abortController}},jf=new wl,Li=class{constructor(e){this.manager=e!==void 0?e:jf,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}load(){}loadAsync(e,t){let n=this;return new Promise(function(i,r){n.load(e,i,t,r)})}parse(){}setCrossOrigin(e){return this.crossOrigin=e,this}setWithCredentials(e){return this.withCredentials=e,this}setPath(e){return this.path=e,this}setResourcePath(e){return this.resourcePath=e,this}setRequestHeader(e){return this.requestHeader=e,this}abort(){return this}};Li.DEFAULT_MATERIAL_NAME="__DEFAULT";var Xi={},Eh=class extends Error{constructor(e,t){super(e),this.response=t}},Ur=class extends Li{constructor(e){super(e),this.mimeType="",this.responseType="",this._abortController=new AbortController}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=Ti.get(`file:${e}`);if(r!==void 0){this.manager.itemStart(e),setTimeout(()=>{t&&t(r),this.manager.itemEnd(e)},0);return}if(Xi[e]!==void 0){Xi[e].push({onLoad:t,onProgress:n,onError:i});return}Xi[e]=[],Xi[e].push({onLoad:t,onProgress:n,onError:i});let a=new Request(e,{headers:new Headers(this.requestHeader),credentials:this.withCredentials?"include":"same-origin",signal:typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal}),o=this.mimeType,l=this.responseType;fetch(a).then(c=>{if(c.status===200||c.status===0){if(c.status===0&&ht("FileLoader: HTTP Status 0 received."),typeof ReadableStream>"u"||c.body===void 0||c.body.getReader===void 0)return c;let h=Xi[e],d=c.body.getReader(),u=c.headers.get("X-File-Size")||c.headers.get("Content-Length"),p=u?parseInt(u):0,x=p!==0,v=0,m=new ReadableStream({start(f){y();function y(){d.read().then(({done:T,value:g})=>{if(T)f.close();else{v+=g.byteLength;let _=new ProgressEvent("progress",{lengthComputable:x,loaded:v,total:p});for(let S=0,A=h.length;S<A;S++){let b=h[S];b.onProgress&&b.onProgress(_)}f.enqueue(g),y()}},T=>{f.error(T)})}}});return new Response(m)}else throw new Eh(`fetch for "${c.url}" responded with ${c.status}: ${c.statusText}`,c)}).then(c=>{switch(l){case"arraybuffer":return c.arrayBuffer();case"blob":return c.blob();case"document":return c.text().then(h=>new DOMParser().parseFromString(h,o));case"json":return c.json();default:if(o==="")return c.text();{let d=/charset="?([^;"\s]*)"?/i.exec(o),u=d&&d[1]?d[1].toLowerCase():void 0,p=new TextDecoder(u);return c.arrayBuffer().then(x=>p.decode(x))}}}).then(c=>{Ti.add(`file:${e}`,c);let h=Xi[e];delete Xi[e];for(let d=0,u=h.length;d<u;d++){let p=h[d];p.onLoad&&p.onLoad(c)}}).catch(c=>{let h=Xi[e];if(h===void 0)throw this.manager.itemError(e),c;delete Xi[e];for(let d=0,u=h.length;d<u;d++){let p=h[d];p.onError&&p.onError(c)}this.manager.itemError(e)}).finally(()=>{this.manager.itemEnd(e)}),this.manager.itemStart(e)}setResponseType(e){return this.responseType=e,this}setMimeType(e){return this.mimeType=e,this}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var pr=new WeakMap,Sl=class extends Li{constructor(e){super(e)}load(e,t,n,i){this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,a=Ti.get(`image:${e}`);if(a!==void 0){if(a.complete===!0)r.manager.itemStart(e),setTimeout(function(){t&&t(a),r.manager.itemEnd(e)},0);else{let d=pr.get(a);d===void 0&&(d=[],pr.set(a,d)),d.push({onLoad:t,onError:i})}return a}let o=yr("img");function l(){h(),t&&t(this);let d=pr.get(this)||[];for(let u=0;u<d.length;u++){let p=d[u];p.onLoad&&p.onLoad(this)}pr.delete(this),r.manager.itemEnd(e)}function c(d){h(),i&&i(d),Ti.remove(`image:${e}`);let u=pr.get(this)||[];for(let p=0;p<u.length;p++){let x=u[p];x.onError&&x.onError(d)}pr.delete(this),r.manager.itemError(e),r.manager.itemEnd(e)}function h(){o.removeEventListener("load",l,!1),o.removeEventListener("error",c,!1)}return o.addEventListener("load",l,!1),o.addEventListener("error",c,!1),e.slice(0,5)!=="data:"&&this.crossOrigin!==void 0&&(o.crossOrigin=this.crossOrigin),Ti.add(`image:${e}`,o),r.manager.itemStart(e),o.src=e,o}};var Oa=class extends Li{constructor(e){super(e)}load(e,t,n,i){let r=new cn,a=new Sl(this.manager);return a.setCrossOrigin(this.crossOrigin),a.setPath(this.path),a.load(e,function(o){r.image=o,r.needsUpdate=!0,t!==void 0&&t(r)},n,i),r}},Ws=class extends Xt{constructor(e,t=1){super(),this.isLight=!0,this.type="Light",this.color=new Ce(e),this.intensity=t}copy(e,t){return super.copy(e,t),this.color.copy(e.color),this.intensity=e.intensity,this}toJSON(e){let t=super.toJSON(e);return t.object.color=this.color.getHex(),t.object.intensity=this.intensity,t}},za=class extends Ws{constructor(e,t,n){super(e,n),this.isHemisphereLight=!0,this.type="HemisphereLight",this.position.copy(Xt.DEFAULT_UP),this.updateMatrix(),this.groundColor=new Ce(t)}copy(e,t){return super.copy(e,t),this.groundColor.copy(e.groundColor),this}toJSON(e){let t=super.toJSON(e);return t.object.groundColor=this.groundColor.getHex(),t}},vh=new mt,Qd=new k,$d=new k,Or=class{constructor(e){this.camera=e,this.intensity=1,this.bias=0,this.biasNode=null,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new ze(512,512),this.mapType=Gn,this.map=null,this.mapPass=null,this.matrix=new mt,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new Cr,this._frameExtents=new ze(1,1),this._viewportCount=1,this._viewports=[new Nt(0,0,1,1)]}getViewportCount(){return this._viewportCount}getCamera(){return this.camera}getFrustum(){return this._frustum}updateMatrices(e){let t=this.camera;Qd.setFromMatrixPosition(e.matrixWorld),t.position.copy(Qd),$d.setFromMatrixPosition(e.target.matrixWorld),t.lookAt($d),t.updateMatrixWorld(),this._updateMatrix(t,this.matrix,this._frustum)}_updateMatrix(e,t,n,i){vh.multiplyMatrices(e.projectionMatrix,e.matrixWorldInverse),n.setFromProjectionMatrix(vh,e.coordinateSystem,e.reversedDepth);let r=this._frameExtents,a=i?i.z/r.x:1,o=i?i.w/r.y:1,l=i?i.x/r.x:0,c=i?i.y/r.y:0;e.coordinateSystem===_r||e.reversedDepth?t.set(.5*a,0,0,.5*a+l,0,.5*o,0,.5*o+c,0,0,1,0,0,0,0,1):t.set(.5*a,0,0,.5*a+l,0,.5*o,0,.5*o+c,0,0,.5,.5,0,0,0,1),t.multiply(vh)}getViewport(e){return this._viewports[e]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(e){return this.camera=e.camera.clone(),this.intensity=e.intensity,this.bias=e.bias,this.radius=e.radius,this.autoUpdate=e.autoUpdate,this.needsUpdate=e.needsUpdate,this.normalBias=e.normalBias,this.blurSamples=e.blurSamples,this.mapSize.copy(e.mapSize),this.biasNode=e.biasNode,this}clone(){return new this.constructor().copy(this)}toJSON(){let e={};return e.intensity=this.intensity,e.bias=this.bias,e.normalBias=this.normalBias,e.radius=this.radius,e.blurSamples=this.blurSamples,e.mapSize=this.mapSize.toArray(),e.camera=this.camera.toJSON(!1).object,delete e.camera.matrix,e}},Xo=new k,jo=new $t,Ei=new k,ka=class extends Xt{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new mt,this.projectionMatrix=new mt,this.projectionMatrixInverse=new mt,this.coordinateSystem=fi,this._reversedDepth=!1}get reversedDepth(){return this._reversedDepth}copy(e,t){return super.copy(e,t),this.matrixWorldInverse.copy(e.matrixWorldInverse),this.projectionMatrix.copy(e.projectionMatrix),this.projectionMatrixInverse.copy(e.projectionMatrixInverse),this.coordinateSystem=e.coordinateSystem,this}getWorldDirection(e){return super.getWorldDirection(e).negate()}updateMatrixWorld(e){super.updateMatrixWorld(e),this.matrixWorld.decompose(Xo,jo,Ei),Ei.x===1&&Ei.y===1&&Ei.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Xo,jo,Ei.set(1,1,1)).invert()}updateWorldMatrix(e,t,n=!1){super.updateWorldMatrix(e,t,n),this.matrixWorld.decompose(Xo,jo,Ei),Ei.x===1&&Ei.y===1&&Ei.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Xo,jo,Ei.set(1,1,1)).invert()}clone(){return new this.constructor().copy(this)}},us=new k,ef=new ze,tf=new ze,mn=class extends ka{constructor(e=50,t=1,n=.1,i=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=e,this.zoom=1,this.near=n,this.far=i,this.focus=10,this.aspect=t,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.fov=e.fov,this.zoom=e.zoom,this.near=e.near,this.far=e.far,this.focus=e.focus,this.aspect=e.aspect,this.view=e.view===null?null:Object.assign({},e.view),this.filmGauge=e.filmGauge,this.filmOffset=e.filmOffset,this}setFocalLength(e){let t=.5*this.getFilmHeight()/e;this.fov=zs*2*Math.atan(t),this.updateProjectionMatrix()}getFocalLength(){let e=Math.tan(pa*.5*this.fov);return .5*this.getFilmHeight()/e}getEffectiveFOV(){return zs*2*Math.atan(Math.tan(pa*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(e,t,n){us.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),t.set(us.x,us.y).multiplyScalar(-e/us.z),us.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),n.set(us.x,us.y).multiplyScalar(-e/us.z)}getViewSize(e,t){return this.getViewBounds(e,ef,tf),t.subVectors(tf,ef)}setViewOffset(e,t,n,i,r,a){this.aspect=e/t,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=a,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=this.near,t=e*Math.tan(pa*.5*this.fov)/this.zoom,n=2*t,i=this.aspect*n,r=-.5*i,a=this.view;if(this.view!==null&&this.view.enabled){let l=a.fullWidth,c=a.fullHeight;r+=a.offsetX*i/l,t-=a.offsetY*n/c,i*=a.width/l,n*=a.height/c}let o=this.filmOffset;o!==0&&(r+=e*o/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+i,t,t-n,e,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.fov=this.fov,t.object.zoom=this.zoom,t.object.near=this.near,t.object.far=this.far,t.object.focus=this.focus,t.object.aspect=this.aspect,this.view!==null&&(t.object.view=Object.assign({},this.view)),t.object.filmGauge=this.filmGauge,t.object.filmOffset=this.filmOffset,t}},Th=class extends Or{constructor(){super(new mn(50,1,.5,500)),this.isSpotLightShadow=!0,this.focus=1,this.aspect=1}updateMatrices(e){let t=this.camera,n=zs*2*e.angle*this.focus,i=this.mapSize.width/this.mapSize.height*this.aspect,r=e.distance||t.far;(n!==t.fov||i!==t.aspect||r!==t.far)&&(t.fov=n,t.aspect=i,t.far=r,t.updateProjectionMatrix()),super.updateMatrices(e)}copy(e){return super.copy(e),this.focus=e.focus,this.aspect=e.aspect,this}toJSON(){let e=super.toJSON();return e.focus=this.focus,e.aspect=this.aspect,e}},Ba=class extends Ws{constructor(e,t,n=0,i=Math.PI/3,r=0,a=2){super(e,t),this.isSpotLight=!0,this.type="SpotLight",this.position.copy(Xt.DEFAULT_UP),this.updateMatrix(),this.target=new Xt,this.distance=n,this.angle=i,this.penumbra=r,this.decay=a,this.map=null,this.shadow=new Th}get power(){return this.intensity*Math.PI}set power(e){this.intensity=e/Math.PI}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.angle=e.angle,this.penumbra=e.penumbra,this.decay=e.decay,this.target=e.target.clone(),this.map=e.map,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.angle=this.angle,t.object.decay=this.decay,t.object.penumbra=this.penumbra,t.object.target=this.target.uuid,this.map&&this.map.isTexture&&(t.object.map=this.map.toJSON(e).uuid),t.object.shadow=this.shadow.toJSON(),t}},Ah=class extends Or{constructor(){super(new mn(90,1,.5,500)),this.isPointLightShadow=!0}},bi=class extends Ws{constructor(e,t,n=0,i=2){super(e,t),this.isPointLight=!0,this.type="PointLight",this.distance=n,this.decay=i,this.shadow=new Ah}get power(){return this.intensity*4*Math.PI}set power(e){this.intensity=e/(4*Math.PI)}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.decay=e.decay,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.decay=this.decay,t.object.shadow=this.shadow.toJSON(),t}},Fi=class extends ka{constructor(e=-1,t=1,n=1,i=-1,r=.1,a=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=e,this.right=t,this.top=n,this.bottom=i,this.near=r,this.far=a,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.left=e.left,this.right=e.right,this.top=e.top,this.bottom=e.bottom,this.near=e.near,this.far=e.far,this.zoom=e.zoom,this.view=e.view===null?null:Object.assign({},e.view),this}setViewOffset(e,t,n,i,r,a){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=a,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=(this.right-this.left)/(2*this.zoom),t=(this.top-this.bottom)/(2*this.zoom),n=(this.right+this.left)/2,i=(this.top+this.bottom)/2,r=n-e,a=n+e,o=i+t,l=i-t;if(this.view!==null&&this.view.enabled){let c=(this.right-this.left)/this.view.fullWidth/this.zoom,h=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=c*this.view.offsetX,a=r+c*this.view.width,o-=h*this.view.offsetY,l=o-h*this.view.height}this.projectionMatrix.makeOrthographic(r,a,o,l,this.near,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.zoom=this.zoom,t.object.left=this.left,t.object.right=this.right,t.object.top=this.top,t.object.bottom=this.bottom,t.object.near=this.near,t.object.far=this.far,this.view!==null&&(t.object.view=Object.assign({},this.view)),t}},Rh=class extends Or{constructor(){super(new Fi(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},$i=class extends Ws{constructor(e,t){super(e,t),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(Xt.DEFAULT_UP),this.updateMatrix(),this.target=new Xt,this.shadow=new Rh}dispose(){super.dispose(),this.shadow.dispose()}copy(e){return super.copy(e),this.target=e.target.clone(),this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.shadow=this.shadow.toJSON(),t.object.target=this.target.uuid,t}};var es=class{static extractUrlBase(e){let t=e.lastIndexOf("/");return t===-1?"./":e.slice(0,t+1)}static resolveURL(e,t){return typeof e!="string"||e===""?"":(/^https?:\/\//i.test(t)&&/^\//.test(e)&&(t=t.replace(/(^https?:\/\/[^\/]+).*/i,"$1")),/^(https?:)?\/\//i.test(e)||/^data:.*,.*$/i.test(e)||/^blob:.*$/i.test(e)?e:t+e)}};var _h=new WeakMap,Ha=class extends Li{constructor(e){super(e),this.isImageBitmapLoader=!0,typeof createImageBitmap>"u"&&ht("ImageBitmapLoader: createImageBitmap() not supported."),typeof fetch>"u"&&ht("ImageBitmapLoader: fetch() not supported."),this.options={premultiplyAlpha:"none"},this._abortController=new AbortController}setOptions(e){return this.options=e,this}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,a=Ti.get(`image-bitmap:${e}`);if(a!==void 0){if(r.manager.itemStart(e),a.then){a.then(c=>{_h.has(a)===!0?(i&&i(_h.get(a)),r.manager.itemError(e),r.manager.itemEnd(e)):(t&&t(c),r.manager.itemEnd(e))});return}setTimeout(function(){t&&t(a),r.manager.itemEnd(e)},0);return}let o={};o.credentials=this.crossOrigin==="anonymous"?"same-origin":"include",o.headers=this.requestHeader,o.signal=typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal;let l=fetch(e,o).then(function(c){return c.blob()}).then(function(c){return createImageBitmap(c,Object.assign({},r.options,{colorSpaceConversion:"none"}))}).then(function(c){return Ti.add(`image-bitmap:${e}`,c),t&&t(c),r.manager.itemEnd(e),c}).catch(function(c){i&&i(c),_h.set(l,c),Ti.remove(`image-bitmap:${e}`),r.manager.itemError(e),r.manager.itemEnd(e)});Ti.add(`image-bitmap:${e}`,l),r.manager.itemStart(e)}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var mr=-90,gr=1,El=class extends Xt{constructor(e,t,n){super(),this.type="CubeCamera",this.renderTarget=n,this.coordinateSystem=null,this.activeMipmapLevel=0;let i=new mn(mr,gr,e,t);i.layers=this.layers,this.add(i);let r=new mn(mr,gr,e,t);r.layers=this.layers,this.add(r);let a=new mn(mr,gr,e,t);a.layers=this.layers,this.add(a);let o=new mn(mr,gr,e,t);o.layers=this.layers,this.add(o);let l=new mn(mr,gr,e,t);l.layers=this.layers,this.add(l);let c=new mn(mr,gr,e,t);c.layers=this.layers,this.add(c)}updateCoordinateSystem(){let e=this.coordinateSystem,t=this.children.concat(),[n,i,r,a,o,l]=t;for(let c of t)this.remove(c);if(e===fi)n.up.set(0,1,0),n.lookAt(1,0,0),i.up.set(0,1,0),i.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),a.up.set(0,0,1),a.lookAt(0,-1,0),o.up.set(0,1,0),o.lookAt(0,0,1),l.up.set(0,1,0),l.lookAt(0,0,-1);else if(e===_r)n.up.set(0,-1,0),n.lookAt(-1,0,0),i.up.set(0,-1,0),i.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),a.up.set(0,0,-1),a.lookAt(0,-1,0),o.up.set(0,-1,0),o.lookAt(0,0,1),l.up.set(0,-1,0),l.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+e);for(let c of t)this.add(c),c.updateMatrixWorld()}update(e,t){this.parent===null&&this.updateMatrixWorld();let{renderTarget:n,activeMipmapLevel:i}=this;this.coordinateSystem!==e.coordinateSystem&&(this.coordinateSystem=e.coordinateSystem,this.updateCoordinateSystem());let[r,a,o,l,c,h]=this.children,d=e.getRenderTarget(),u=e.getActiveCubeFace(),p=e.getActiveMipmapLevel(),x=e.xr.enabled;e.xr.enabled=!1;let v=n.texture.generateMipmaps;n.texture.generateMipmaps=!1;let m=!1;e.isWebGLRenderer===!0?m=e.state.buffers.depth.getReversed():m=e.reversedDepthBuffer,e.setRenderTarget(n,0,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,r),e.setRenderTarget(n,1,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,a),e.setRenderTarget(n,2,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,o),e.setRenderTarget(n,3,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,l),e.setRenderTarget(n,4,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,c),n.texture.generateMipmaps=v,e.setRenderTarget(n,5,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,h),e.setRenderTarget(d,u,p),e.xr.enabled=x,n.texture.needsPMREMUpdate=!0}},Tl=class extends mn{constructor(e=[]){super(),this.isArrayCamera=!0,this.isMultiViewCamera=!1,this.cameras=e}},Ga=class{constructor(){this._previousTime=0,this._currentTime=0,this._startTime=performance.now(),this._delta=0,this._elapsed=0,this._timescale=1,this._document=null,this._pageVisibilityHandler=null}connect(e){this._document=e,e.hidden!==void 0&&(this._pageVisibilityHandler=Yg.bind(this),e.addEventListener("visibilitychange",this._pageVisibilityHandler,!1))}disconnect(){this._pageVisibilityHandler!==null&&(this._document.removeEventListener("visibilitychange",this._pageVisibilityHandler),this._pageVisibilityHandler=null),this._document=null}getDelta(){return this._delta/1e3}getElapsed(){return this._elapsed/1e3}getTimescale(){return this._timescale}setTimescale(e){return this._timescale=e,this}reset(){return this._currentTime=performance.now()-this._startTime,this}dispose(){this.disconnect()}update(e){return this._pageVisibilityHandler!==null&&this._document.hidden===!0?this._delta=0:(this._previousTime=this._currentTime,this._currentTime=(e!==void 0?e:performance.now())-this._startTime,this._delta=(this._currentTime-this._previousTime)*this._timescale,this._elapsed+=this._delta),this}};function Yg(){this._document.hidden===!1&&this.reset()}var Al=class{constructor(e,t,n){this.binding=e,this.valueSize=n;let i,r,a;switch(t){case"quaternion":i=this._slerp,r=this._slerpAdditive,a=this._setAdditiveIdentityQuaternion,this.buffer=new Float64Array(n*6),this._workIndex=5;break;case"string":case"bool":i=this._select,r=this._select,a=this._setAdditiveIdentityOther,this.buffer=new Array(n*5);break;default:i=this._lerp,r=this._lerpAdditive,a=this._setAdditiveIdentityNumeric,this.buffer=new Float64Array(n*5)}this._mixBufferRegion=i,this._mixBufferRegionAdditive=r,this._setIdentity=a,this._origIndex=3,this._addIndex=4,this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,this.useCount=0,this.referenceCount=0}accumulate(e,t){let n=this.buffer,i=this.valueSize,r=e*i+i,a=this.cumulativeWeight;if(a===0){for(let o=0;o!==i;++o)n[r+o]=n[o];a=t}else{a+=t;let o=t/a;this._mixBufferRegion(n,r,0,o,i)}this.cumulativeWeight=a}accumulateAdditive(e){let t=this.buffer,n=this.valueSize,i=n*this._addIndex;this.cumulativeWeightAdditive===0&&this._setIdentity(),this._mixBufferRegionAdditive(t,i,0,e,n),this.cumulativeWeightAdditive+=e}apply(e){let t=this.valueSize,n=this.buffer,i=e*t+t,r=this.cumulativeWeight,a=this.cumulativeWeightAdditive,o=this.binding;if(this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,r<1){let l=t*this._origIndex;this._mixBufferRegion(n,i,l,1-r,t)}a>0&&this._mixBufferRegionAdditive(n,i,this._addIndex*t,1,t);for(let l=t,c=t+t;l!==c;++l)if(n[l]!==n[l+t]){o.setValue(n,i);break}}saveOriginalState(){let e=this.binding,t=this.buffer,n=this.valueSize,i=n*this._origIndex;e.getValue(t,i);for(let r=n,a=i;r!==a;++r)t[r]=t[i+r%n];this._setIdentity(),this.cumulativeWeight=0,this.cumulativeWeightAdditive=0}restoreOriginalState(){let e=this.valueSize*3;this.binding.setValue(this.buffer,e)}_setAdditiveIdentityNumeric(){let e=this._addIndex*this.valueSize,t=e+this.valueSize;for(let n=e;n<t;n++)this.buffer[n]=0}_setAdditiveIdentityQuaternion(){this._setAdditiveIdentityNumeric(),this.buffer[this._addIndex*this.valueSize+3]=1}_setAdditiveIdentityOther(){let e=this._origIndex*this.valueSize,t=this._addIndex*this.valueSize;for(let n=0;n<this.valueSize;n++)this.buffer[t+n]=this.buffer[e+n]}_select(e,t,n,i,r){if(i>=.5)for(let a=0;a!==r;++a)e[t+a]=e[n+a]}_slerp(e,t,n,i){$t.slerpFlat(e,t,e,t,e,n,i)}_slerpAdditive(e,t,n,i,r){let a=this._workIndex*r;$t.multiplyQuaternionsFlat(e,a,e,t,e,n),$t.slerpFlat(e,t,e,t,e,a,i)}_lerp(e,t,n,i,r){let a=1-i;for(let o=0;o!==r;++o){let l=t+o;e[l]=e[l]*a+e[n+o]*i}}_lerpAdditive(e,t,n,i,r){for(let a=0;a!==r;++a){let o=t+a;e[o]=e[o]+e[n+a]*i}}},Jh="\\[\\]\\.:\\/",Jg=new RegExp("["+Jh+"]","g"),Zh="[^"+Jh+"]",Zg="[^"+Jh.replace("\\.","")+"]",Qg=/((?:WC+[\/:])*)/.source.replace("WC",Zh),$g=/(WCOD+)?/.source.replace("WCOD",Zg),e0=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",Zh),t0=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",Zh),n0=new RegExp("^"+Qg+$g+e0+t0+"$"),i0=["material","materials","bones","map"],Ch=class{constructor(e,t,n){let i=n||Qt.parseTrackName(t);this._targetGroup=e,this._bindings=e.subscribe_(t,i)}getValue(e,t){this.bind();let n=this._targetGroup.nCachedObjects_,i=this._bindings[n];i!==void 0&&i.getValue(e,t)}setValue(e,t){let n=this._bindings;for(let i=this._targetGroup.nCachedObjects_,r=n.length;i!==r;++i)n[i].setValue(e,t)}bind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].bind()}unbind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].unbind()}},Qt=class s{constructor(e,t,n){this.path=t,this.parsedPath=n||s.parseTrackName(t),this.node=s.findNode(e,this.parsedPath.nodeName),this.rootNode=e,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(e,t,n){return e&&e.isAnimationObjectGroup?new s.Composite(e,t,n):new s(e,t,n)}static sanitizeNodeName(e){return e.replace(/\s/g,"_").replace(Jg,"")}static parseTrackName(e){let t=n0.exec(e);if(t===null)throw new Error("THREE.PropertyBinding: Cannot parse trackName: "+e);let n={nodeName:t[2],objectName:t[3],objectIndex:t[4],propertyName:t[5],propertyIndex:t[6]},i=n.nodeName&&n.nodeName.lastIndexOf(".");if(i!==void 0&&i!==-1){let r=n.nodeName.substring(i+1);i0.indexOf(r)!==-1&&(n.nodeName=n.nodeName.substring(0,i),n.objectName=r)}if(n.propertyName===null||n.propertyName.length===0)throw new Error("THREE.PropertyBinding: can not parse propertyName from trackName: "+e);return n}static findNode(e,t){if(t===void 0||t===""||t==="."||t===-1||t===e.name||t===e.uuid)return e;if(e.skeleton){let n=e.skeleton.getBoneByName(t);if(n!==void 0)return n}if(e.children){let n=function(r){for(let a=0;a<r.length;a++){let o=r[a];if(o.name===t||o.uuid===t)return o;let l=n(o.children);if(l)return l}return null},i=n(e.children);if(i)return i}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(e,t){e[t]=this.targetObject[this.propertyName]}_getValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)e[t++]=n[i]}_getValue_arrayElement(e,t){e[t]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(e,t){this.resolvedProperty.toArray(e,t)}_setValue_direct(e,t){this.targetObject[this.propertyName]=e[t]}_setValue_direct_setNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++]}_setValue_array_setNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(e,t){this.resolvedProperty[this.propertyIndex]=e[t]}_setValue_arrayElement_setNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(e,t){this.resolvedProperty.fromArray(e,t)}_setValue_fromArray_setNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(e,t){this.bind(),this.getValue(e,t)}_setValue_unbound(e,t){this.bind(),this.setValue(e,t)}bind(){let e=this.node,t=this.parsedPath,n=t.objectName,i=t.propertyName,r=t.propertyIndex;if(e||(e=s.findNode(this.rootNode,t.nodeName),this.node=e),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!e){ht("PropertyBinding: No target node found for track: "+this.path+".");return}if(n){let c=t.objectIndex;switch(n){case"materials":if(!e.material){gt("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.materials){gt("PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}e=e.material.materials;break;case"bones":if(!e.skeleton){gt("PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}e=e.skeleton.bones;for(let h=0;h<e.length;h++)if(e[h].name===c){c=h;break}break;case"map":if("map"in e){e=e.map;break}if(!e.material){gt("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.map){gt("PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}e=e.material.map;break;default:if(e[n]===void 0){gt("PropertyBinding: Can not bind to objectName of node undefined.",this);return}e=e[n]}if(c!==void 0){if(e[c]===void 0){gt("PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,e);return}e=e[c]}}let a=e[i];if(a===void 0){let c=t.nodeName;gt("PropertyBinding: Trying to update property for track: "+c+"."+i+" but it wasn't found.",e);return}let o=this.Versioning.None;this.targetObject=e,e.isMaterial===!0?o=this.Versioning.NeedsUpdate:e.isObject3D===!0&&(o=this.Versioning.MatrixWorldNeedsUpdate);let l=this.BindingType.Direct;if(r!==void 0){if(i==="morphTargetInfluences"){if(!e.geometry){gt("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!e.geometry.morphAttributes){gt("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}e.morphTargetDictionary[r]!==void 0&&(r=e.morphTargetDictionary[r])}l=this.BindingType.ArrayElement,this.resolvedProperty=a,this.propertyIndex=r}else a.fromArray!==void 0&&a.toArray!==void 0?(l=this.BindingType.HasFromToArray,this.resolvedProperty=a):Array.isArray(a)?(l=this.BindingType.EntireArray,this.resolvedProperty=a):this.propertyName=i;this.getValue=this.GetterByBindingType[l],this.setValue=this.SetterByBindingTypeAndVersioning[l][o]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};Qt.Composite=Ch;Qt.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};Qt.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};Qt.prototype.GetterByBindingType=[Qt.prototype._getValue_direct,Qt.prototype._getValue_array,Qt.prototype._getValue_arrayElement,Qt.prototype._getValue_toArray];Qt.prototype.SetterByBindingTypeAndVersioning=[[Qt.prototype._setValue_direct,Qt.prototype._setValue_direct_setNeedsUpdate,Qt.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[Qt.prototype._setValue_array,Qt.prototype._setValue_array_setNeedsUpdate,Qt.prototype._setValue_array_setMatrixWorldNeedsUpdate],[Qt.prototype._setValue_arrayElement,Qt.prototype._setValue_arrayElement_setNeedsUpdate,Qt.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[Qt.prototype._setValue_fromArray,Qt.prototype._setValue_fromArray_setNeedsUpdate,Qt.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var Rl=class{constructor(e,t,n=null,i=t.blendMode){this._mixer=e,this._clip=t,this._localRoot=n,this.blendMode=i;let r=t.tracks,a=r.length,o=new Array(a),l={endingStart:Ls,endingEnd:Ls};for(let c=0;c!==a;++c){let h=r[c].createInterpolant(null);o[c]=h,h.settings=l}this._interpolantSettings=l,this._interpolants=o,this._propertyBindings=new Array(a),this._cacheIndex=null,this._byClipCacheIndex=null,this._timeScaleInterpolant=null,this._restoreTimeScale=null,this._weightInterpolant=null,this.loop=Cf,this._loopCount=-1,this._startTime=null,this.time=0,this.timeScale=1,this._effectiveTimeScale=1,this.weight=1,this._effectiveWeight=1,this.repetitions=1/0,this.paused=!1,this.enabled=!0,this.clampWhenFinished=!1,this.zeroSlopeAtStart=!0,this.zeroSlopeAtEnd=!0}play(){return this._mixer._activateAction(this),this}stop(){return this._mixer._deactivateAction(this),this.reset()}reset(){return this.paused=!1,this.enabled=!0,this.time=0,this._loopCount=-1,this._startTime=null,this.stopFading().stopWarping()}isRunning(){return this.enabled&&!this.paused&&this.timeScale!==0&&this._startTime===null&&this._mixer._isActiveAction(this)}isScheduled(){return this._mixer._isActiveAction(this)}startAt(e){return this._startTime=e,this}setLoop(e,t){return this.loop=e,this.repetitions=t,this}setEffectiveWeight(e){return this.weight=e,this._effectiveWeight=this.enabled?e:0,this.stopFading()}getEffectiveWeight(){return this._effectiveWeight}fadeIn(e){return this._scheduleFading(e,0,1)}fadeOut(e){return this._scheduleFading(e,1,0)}crossFadeFrom(e,t,n=!1){if(e.fadeOut(t),this.fadeIn(t),n===!0){let i=this._clip.duration,r=e._clip.duration,a=r/i,o=i/r;e._restoreTimeScale=e.timeScale,this._restoreTimeScale=this.timeScale,e.warp(1,a,t),this.warp(o,1,t)}return this}crossFadeTo(e,t,n=!1){return e.crossFadeFrom(this,t,n)}stopFading(){let e=this._weightInterpolant;return e!==null&&(this._weightInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this}setEffectiveTimeScale(e){return this.timeScale=e,this._effectiveTimeScale=this.paused?0:e,this.stopWarping()}getEffectiveTimeScale(){return this._effectiveTimeScale}setDuration(e){return this.timeScale=this._clip.duration/e,this.stopWarping()}syncWith(e){return this.time=e.time,this.timeScale=e.timeScale,this.stopWarping()}halt(e){return this.warp(this._effectiveTimeScale,0,e)}warp(e,t,n){let i=this._mixer,r=i.time,a=this.timeScale,o=this._timeScaleInterpolant;o===null&&(o=i._lendControlInterpolant(),this._timeScaleInterpolant=o);let l=o.parameterPositions,c=o.sampleValues;return l[0]=r,l[1]=r+n,c[0]=e/a,c[1]=t/a,this}stopWarping(){let e=this._timeScaleInterpolant;return e!==null&&(this._timeScaleInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this._restoreTimeScale=null,this}getMixer(){return this._mixer}getClip(){return this._clip}getRoot(){return this._localRoot||this._mixer._root}_update(e,t,n,i){if(!this.enabled){this._updateWeight(e);return}let r=this._startTime;if(r!==null){let l=(e-r)*n;l<0||n===0?t=0:(this._startTime=null,t=n*l)}t*=this._updateTimeScale(e);let a=this._updateTime(t),o=this._updateWeight(e);if(o>0){let l=this._interpolants,c=this._propertyBindings;switch(this.blendMode){case If:for(let h=0,d=l.length;h!==d;++h)l[h].evaluate(a),c[h].accumulateAdditive(o);break;case xc:default:for(let h=0,d=l.length;h!==d;++h)l[h].evaluate(a),c[h].accumulate(i,o)}}}_updateWeight(e){let t=0;if(this.enabled){t=this.weight;let n=this._weightInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(this.stopFading(),i===0&&(this.enabled=!1))}}return this._effectiveWeight=t,t}_updateTimeScale(e){let t=0;if(!this.paused){t=this.timeScale;let n=this._timeScaleInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(t===0?this.paused=!0:(this._restoreTimeScale!==null&&(t=this._restoreTimeScale),this.timeScale=t),this.stopWarping())}}return this._effectiveTimeScale=t,t}_updateTime(e){let t=this._clip.duration,n=this.loop,i=this.time+e,r=this._loopCount,a=n===Pf;if(e===0)return r===-1?i:a&&(r&1)===1?t-i:i;if(n===gc){r===-1&&(this._loopCount=0,this._setEndings(!0,!0,!1));e:{if(i>=t)i=t;else if(i<0)i=0;else{this.time=i;break e}this.clampWhenFinished?this.paused=!0:this.enabled=!1,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e<0?-1:1})}}else{if(r===-1&&(e>=0?(r=0,this._setEndings(!0,this.repetitions===0,a)):this._setEndings(this.repetitions===0,!0,a)),i>=t||i<0){let o=Math.floor(i/t);i-=t*o,r+=Math.abs(o);let l=this.repetitions-r;if(l<=0)this.clampWhenFinished?this.paused=!0:this.enabled=!1,i=e>0?t:0,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e>0?1:-1});else{if(l===1){let c=e<0;this._setEndings(c,!c,a)}else this._setEndings(!1,!1,a);this._loopCount=r,this.time=i,this._mixer.dispatchEvent({type:"loop",action:this,loopDelta:o})}}else this._loopCount=r,this.time=i;if(a&&(r&1)===1)return t-i}return i}_setEndings(e,t,n){let i=this._interpolantSettings;n?(i.endingStart=Fs,i.endingEnd=Fs):(e?i.endingStart=this.zeroSlopeAtStart?Fs:Ls:i.endingStart=ba,t?i.endingEnd=this.zeroSlopeAtEnd?Fs:Ls:i.endingEnd=ba)}_scheduleFading(e,t,n){let i=this._mixer,r=i.time,a=this._weightInterpolant;a===null&&(a=i._lendControlInterpolant(),this._weightInterpolant=a);let o=a.parameterPositions,l=a.sampleValues;return o[0]=r,l[0]=t,o[1]=r+e,l[1]=n,this}},s0=new Float32Array(1),Va=class extends ni{constructor(e){super(),this._root=e,this._initMemoryManager(),this._accuIndex=0,this.time=0,this.timeScale=1,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}_bindAction(e,t){let n=e._localRoot||this._root,i=e._clip.tracks,r=i.length,a=e._propertyBindings,o=e._interpolants,l=n.uuid,c=this._bindingsByRootAndName,h=c[l];h===void 0&&(h={},c[l]=h);for(let d=0;d!==r;++d){let u=i[d],p=u.name,x=h[p];if(x!==void 0)++x.referenceCount,a[d]=x;else{if(x=a[d],x!==void 0){x._cacheIndex===null&&(++x.referenceCount,this._addInactiveBinding(x,l,p));continue}let v=t&&t._propertyBindings[d].binding.parsedPath;x=new Al(Qt.create(n,p,v),u.ValueTypeName,u.getValueSize()),++x.referenceCount,this._addInactiveBinding(x,l,p),a[d]=x}o[d].resultBuffer=x.buffer}}_activateAction(e){if(!this._isActiveAction(e)){if(e._cacheIndex===null){let n=(e._localRoot||this._root).uuid,i=e._clip.uuid,r=this._actionsByClip[i];this._bindAction(e,r&&r.knownActions[0]),this._addInactiveAction(e,i,n)}let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];r.useCount++===0&&(this._lendBinding(r),r.saveOriginalState())}this._lendAction(e)}}_deactivateAction(e){if(this._isActiveAction(e)){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.useCount===0&&(r.restoreOriginalState(),this._takeBackBinding(r))}this._takeBackAction(e)}}_initMemoryManager(){this._actions=[],this._nActiveActions=0,this._actionsByClip={},this._bindings=[],this._nActiveBindings=0,this._bindingsByRootAndName={},this._controlInterpolants=[],this._nActiveControlInterpolants=0;let e=this;this.stats={actions:{get total(){return e._actions.length},get inUse(){return e._nActiveActions}},bindings:{get total(){return e._bindings.length},get inUse(){return e._nActiveBindings}},controlInterpolants:{get total(){return e._controlInterpolants.length},get inUse(){return e._nActiveControlInterpolants}}}}_isActiveAction(e){let t=e._cacheIndex;return t!==null&&t<this._nActiveActions}_addInactiveAction(e,t,n){let i=this._actions,r=this._actionsByClip,a=r[t];if(a===void 0)a={knownActions:[e],actionByRoot:{}},e._byClipCacheIndex=0,r[t]=a;else{let o=a.knownActions;e._byClipCacheIndex=o.length,o.push(e)}e._cacheIndex=i.length,i.push(e),a.actionByRoot[n]=e}_removeInactiveAction(e){let t=this._actions,n=t[t.length-1],i=e._cacheIndex;n._cacheIndex=i,t[i]=n,t.pop(),e._cacheIndex=null;let r=e._clip.uuid,a=this._actionsByClip,o=a[r],l=o.knownActions,c=l[l.length-1],h=e._byClipCacheIndex;c._byClipCacheIndex=h,l[h]=c,l.pop(),e._byClipCacheIndex=null;let d=o.actionByRoot,u=(e._localRoot||this._root).uuid;delete d[u],l.length===0&&delete a[r],this._removeInactiveBindingsForAction(e)}_removeInactiveBindingsForAction(e){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.referenceCount===0&&this._removeInactiveBinding(r)}}_lendAction(e){let t=this._actions,n=e._cacheIndex,i=this._nActiveActions++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackAction(e){let t=this._actions,n=e._cacheIndex,i=--this._nActiveActions,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_addInactiveBinding(e,t,n){let i=this._bindingsByRootAndName,r=this._bindings,a=i[t];a===void 0&&(a={},i[t]=a),a[n]=e,e._cacheIndex=r.length,r.push(e)}_removeInactiveBinding(e){let t=this._bindings,n=e.binding,i=n.rootNode.uuid,r=n.path,a=this._bindingsByRootAndName,o=a[i],l=t[t.length-1],c=e._cacheIndex;l._cacheIndex=c,t[c]=l,t.pop(),delete o[r],Object.keys(o).length===0&&delete a[i]}_lendBinding(e){let t=this._bindings,n=e._cacheIndex,i=this._nActiveBindings++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackBinding(e){let t=this._bindings,n=e._cacheIndex,i=--this._nActiveBindings,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_lendControlInterpolant(){let e=this._controlInterpolants,t=this._nActiveControlInterpolants++,n=e[t];return n===void 0&&(n=new Na(new Float32Array(2),new Float32Array(2),1,s0),n.__cacheIndex=t,e[t]=n),n}_takeBackControlInterpolant(e){let t=this._controlInterpolants,n=e.__cacheIndex,i=--this._nActiveControlInterpolants,r=t[i];e.__cacheIndex=i,t[i]=e,r.__cacheIndex=n,t[n]=r}clipAction(e,t,n){let i=t||this._root,r=i.uuid,a=typeof e=="string"?Vs.findByName(i,e):e,o=a!==null?a.uuid:e,l=this._actionsByClip[o],c=null;if(n===void 0&&(a!==null?n=a.blendMode:n=xc),l!==void 0){let d=l.actionByRoot[r];if(d!==void 0&&d.blendMode===n)return d;c=l.knownActions[0],a===null&&(a=c._clip)}if(a===null)return null;let h=new Rl(this,a,t,n);return this._bindAction(h,c),this._addInactiveAction(h,o,r),h}existingAction(e,t){let n=t||this._root,i=n.uuid,r=typeof e=="string"?Vs.findByName(n,e):e,a=r?r.uuid:e,o=this._actionsByClip[a];return o!==void 0&&o.actionByRoot[i]||null}stopAllAction(){let e=this._actions,t=this._nActiveActions;for(let n=t-1;n>=0;--n)e[n].stop();return this}update(e){e*=this.timeScale;let t=this._actions,n=this._nActiveActions,i=this.time+=e,r=Math.sign(e),a=this._accuIndex^=1;for(let c=0;c!==n;++c)t[c]._update(i,e,r,a);let o=this._bindings,l=this._nActiveBindings;for(let c=0;c!==l;++c)o[c].apply(a);return this}setTime(e){this.time=0;for(let t=0;t<this._actions.length;t++)this._actions[t].time=0;return this.update(e)}getRoot(){return this._root}uncacheClip(e){let t=this._actions,n=e.uuid,i=this._actionsByClip,r=i[n];if(r!==void 0){let a=r.knownActions;for(let o=0,l=a.length;o!==l;++o){let c=a[o];this._deactivateAction(c);let h=c._cacheIndex,d=t[t.length-1];c._cacheIndex=null,c._byClipCacheIndex=null,d._cacheIndex=h,t[h]=d,t.pop(),this._removeInactiveBindingsForAction(c)}delete i[n]}}uncacheRoot(e){let t=e.uuid,n=this._actionsByClip;for(let a in n){let o=n[a].actionByRoot,l=o[t];l!==void 0&&(this._deactivateAction(l),this._removeInactiveAction(l))}let i=this._bindingsByRootAndName,r=i[t];if(r!==void 0)for(let a in r){let o=r[a];o.restoreOriginalState(),this._removeInactiveBinding(o)}}uncacheAction(e,t){let n=this.existingAction(e,t);n!==null&&(this._deactivateAction(n),this._removeInactiveAction(n))}};var nf=new mt,Wa=class{constructor(e,t,n=0,i=1/0){this.ray=new Ci(e,t),this.near=n,this.far=i,this.camera=null,this.layers=new Sr,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(e,t){this.ray.set(e,t)}setFromCamera(e,t){t.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(e.x,e.y,.5).unproject(t).sub(this.ray.origin).normalize(),this.camera=t):t.isOrthographicCamera?(this.ray.origin.set(e.x,e.y,t.projectionMatrix.elements[14]).unproject(t),this.ray.direction.set(0,0,-1).transformDirection(t.matrixWorld),this.camera=t):gt("Raycaster: Unsupported camera type: "+t.type)}setFromXRController(e){return nf.identity().extractRotation(e.matrixWorld),this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(nf),this}intersectObject(e,t=!0,n=[]){return Ph(e,this,n,t),n.sort(sf),n}intersectObjects(e,t=!0,n=[]){for(let i=0,r=e.length;i<r;i++)Ph(e[i],this,n,t);return n.sort(sf),n}};function sf(s,e){return s.distance-e.distance}function Ph(s,e,t,n){let i=!0;if(s.layers.test(e.layers)&&s.raycast(e,t)===!1&&(i=!1),i===!0&&n===!0){let r=s.children;for(let a=0,o=r.length;a<o;a++)Ph(r[a],e,t,!0)}}var zr=class{constructor(e=1,t=0,n=0){this.radius=e,this.phi=t,this.theta=n}set(e,t,n){return this.radius=e,this.phi=t,this.theta=n,this}copy(e){return this.radius=e.radius,this.phi=e.phi,this.theta=e.theta,this}makeSafe(){return this.phi=Ct(this.phi,1e-6,Math.PI-1e-6),this}setFromVector3(e){return this.setFromCartesianCoords(e.x,e.y,e.z)}setFromCartesianCoords(e,t,n){return this.radius=Math.sqrt(e*e+t*t+n*n),this.radius===0?(this.theta=0,this.phi=0):(this.theta=Math.atan2(e,n),this.phi=Math.acos(Ct(t/this.radius,-1,1))),this}clone(){return new this.constructor().copy(this)}};var Ih=class s{static{s.prototype.isMatrix2=!0}constructor(e,t,n,i){this.elements=[1,0,0,1],e!==void 0&&this.set(e,t,n,i)}identity(){return this.set(1,0,0,1),this}fromArray(e,t=0){for(let n=0;n<4;n++)this.elements[n]=e[n+t];return this}set(e,t,n,i){let r=this.elements;return r[0]=e,r[2]=t,r[1]=n,r[3]=i,this}};var qa=class extends ni{constructor(e,t=null){super(),this.object=e,this.domElement=t,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(e){this.domElement!==null&&this.disconnect(),this.domElement=e}disconnect(){}dispose(){}update(){}};function Qh(s,e,t,n){let i=r0(n);switch(t){case Gh:return s*e;case Ul:return s*e/i.components*i.byteLength;case Ol:return s*e/i.components*i.byteLength;case ws:return s*e*2/i.components*i.byteLength;case zl:return s*e*2/i.components*i.byteLength;case Vh:return s*e*3/i.components*i.byteLength;case Qn:return s*e*4/i.components*i.byteLength;case kl:return s*e*4/i.components*i.byteLength;case $a:case eo:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case to:case no:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Hl:case Vl:return Math.max(s,16)*Math.max(e,8)/4;case Bl:case Gl:return Math.max(s,8)*Math.max(e,8)/2;case Wl:case ql:case jl:case Kl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case Xl:case io:case Yl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Jl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Zl:return Math.floor((s+4)/5)*Math.floor((e+3)/4)*16;case Ql:return Math.floor((s+4)/5)*Math.floor((e+4)/5)*16;case $l:return Math.floor((s+5)/6)*Math.floor((e+4)/5)*16;case ec:return Math.floor((s+5)/6)*Math.floor((e+5)/6)*16;case tc:return Math.floor((s+7)/8)*Math.floor((e+4)/5)*16;case nc:return Math.floor((s+7)/8)*Math.floor((e+5)/6)*16;case ic:return Math.floor((s+7)/8)*Math.floor((e+7)/8)*16;case sc:return Math.floor((s+9)/10)*Math.floor((e+4)/5)*16;case rc:return Math.floor((s+9)/10)*Math.floor((e+5)/6)*16;case ac:return Math.floor((s+9)/10)*Math.floor((e+7)/8)*16;case oc:return Math.floor((s+9)/10)*Math.floor((e+9)/10)*16;case lc:return Math.floor((s+11)/12)*Math.floor((e+9)/10)*16;case cc:return Math.floor((s+11)/12)*Math.floor((e+11)/12)*16;case hc:case uc:case dc:return Math.ceil(s/4)*Math.ceil(e/4)*16;case fc:case pc:return Math.ceil(s/4)*Math.ceil(e/4)*8;case so:case mc:return Math.ceil(s/4)*Math.ceil(e/4)*16}throw new Error(`Unable to determine texture byte length for ${t} format.`)}function r0(s){switch(s){case Gn:case zh:return{byteLength:1,components:1};case Gr:case kh:case gn:return{byteLength:2,components:1};case Fl:case Nl:return{byteLength:2,components:4};case yi:case Ll:case Zn:return{byteLength:4,components:1};case Bh:case Hh:return{byteLength:4,components:3}}throw new Error(`THREE.TextureUtils: Unknown texture type ${s}.`)}typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"186"}}));typeof window<"u"&&(window.__THREE__?ht("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="186");function mp(){let s=null,e=!1,t=null,n=null;function i(r,a){n=s.requestAnimationFrame(i),t(r,a)}return{start:function(){e!==!0&&t!==null&&s!==null&&(n=s.requestAnimationFrame(i),e=!0)},stop:function(){s!==null&&s.cancelAnimationFrame(n),e=!1},setAnimationLoop:function(r){t=r},setContext:function(r){s=r}}}function o0(s){let e=new WeakMap;function t(o,l){let c=o.array,h=o.usage,d=c.byteLength,u=s.createBuffer();s.bindBuffer(l,u),s.bufferData(l,c,h),o.onUploadCallback();let p;if(c instanceof Float32Array)p=s.FLOAT;else if(typeof Float16Array<"u"&&c instanceof Float16Array)p=s.HALF_FLOAT;else if(c instanceof Uint16Array)o.isFloat16BufferAttribute?p=s.HALF_FLOAT:p=s.UNSIGNED_SHORT;else if(c instanceof Int16Array)p=s.SHORT;else if(c instanceof Uint32Array)p=s.UNSIGNED_INT;else if(c instanceof Int32Array)p=s.INT;else if(c instanceof Int8Array)p=s.BYTE;else if(c instanceof Uint8Array)p=s.UNSIGNED_BYTE;else if(c instanceof Uint8ClampedArray)p=s.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+c);return{buffer:u,type:p,bytesPerElement:c.BYTES_PER_ELEMENT,version:o.version,size:d}}function n(o,l,c){let h=l.array,d=l.updateRanges;if(s.bindBuffer(c,o),d.length===0)s.bufferSubData(c,0,h);else{d.sort((p,x)=>p.start-x.start);let u=0;for(let p=1;p<d.length;p++){let x=d[u],v=d[p];v.start<=x.start+x.count+1?x.count=Math.max(x.count,v.start+v.count-x.start):(++u,d[u]=v)}d.length=u+1;for(let p=0,x=d.length;p<x;p++){let v=d[p];s.bufferSubData(c,v.start*h.BYTES_PER_ELEMENT,h,v.start,v.count)}l.clearUpdateRanges()}l.onUploadCallback()}function i(o){return o.isInterleavedBufferAttribute&&(o=o.data),e.get(o)}function r(o){o.isInterleavedBufferAttribute&&(o=o.data);let l=e.get(o);l&&(s.deleteBuffer(l.buffer),e.delete(o))}function a(o,l){if(o.isInterleavedBufferAttribute&&(o=o.data),o.isGLBufferAttribute){let h=e.get(o);(!h||h.version<o.version)&&e.set(o,{buffer:o.buffer,type:o.type,bytesPerElement:o.elementSize,version:o.version});return}let c=e.get(o);if(c===void 0)e.set(o,t(o,l));else if(c.version<o.version){if(c.size!==o.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");n(c.buffer,o,l),c.version=o.version}}return{get:i,remove:r,update:a}}var l0=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,c0=`#ifdef USE_ALPHAHASH
	const float ALPHA_HASH_SCALE = 0.05;
	float hash2D( vec2 value ) {
		return fract( 1.0e4 * sin( 17.0 * value.x + 0.1 * value.y ) * ( 0.1 + abs( sin( 13.0 * value.y + value.x ) ) ) );
	}
	float hash3D( vec3 value ) {
		return hash2D( vec2( hash2D( value.xy ), value.z ) );
	}
	float getAlphaHashThreshold( vec3 position ) {
		float maxDeriv = max(
			length( dFdx( position.xyz ) ),
			length( dFdy( position.xyz ) )
		);
		float pixScale = 1.0 / ( ALPHA_HASH_SCALE * maxDeriv );
		vec2 pixScales = vec2(
			exp2( floor( log2( pixScale ) ) ),
			exp2( ceil( log2( pixScale ) ) )
		);
		vec2 alpha = vec2(
			hash3D( floor( pixScales.x * position.xyz ) ),
			hash3D( floor( pixScales.y * position.xyz ) )
		);
		float lerpFactor = fract( log2( pixScale ) );
		float x = ( 1.0 - lerpFactor ) * alpha.x + lerpFactor * alpha.y;
		float a = min( lerpFactor, 1.0 - lerpFactor );
		vec3 cases = vec3(
			x * x / ( 2.0 * a * ( 1.0 - a ) ),
			( x - 0.5 * a ) / ( 1.0 - a ),
			1.0 - ( ( 1.0 - x ) * ( 1.0 - x ) / ( 2.0 * a * ( 1.0 - a ) ) )
		);
		float threshold = ( x < ( 1.0 - a ) )
			? ( ( x < a ) ? cases.x : cases.y )
			: cases.z;
		return clamp( threshold , 1.0e-6, 1.0 );
	}
#endif`,h0=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,u0=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,d0=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,f0=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,p0=`#ifdef USE_AOMAP
	float ambientOcclusion = ( texture2D( aoMap, vAoMapUv ).r - 1.0 ) * aoMapIntensity + 1.0;
	reflectedLight.indirectDiffuse *= ambientOcclusion;
	#if defined( USE_CLEARCOAT ) 
		clearcoatSpecularIndirect *= ambientOcclusion;
	#endif
	#if defined( USE_SHEEN ) 
		sheenSpecularIndirect *= ambientOcclusion;
	#endif
	#if defined( USE_ENVMAP ) && defined( STANDARD )
		float dotNV = saturate( dot( geometryNormal, geometryViewDir ) );
		reflectedLight.indirectSpecular *= computeSpecularOcclusion( dotNV, ambientOcclusion, material.roughness );
	#endif
#endif`,m0=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,g0=`#ifdef USE_BATCHING
	#if ! defined( GL_ANGLE_multi_draw )
	#define gl_DrawID _gl_DrawID
	uniform int _gl_DrawID;
	#endif
	uniform highp sampler2D batchingTexture;
	uniform highp usampler2D batchingIdTexture;
	mat4 getBatchingMatrix( const in float i ) {
		int size = textureSize( batchingTexture, 0 ).x;
		int j = int( i ) * 4;
		int x = j % size;
		int y = j / size;
		vec4 v1 = texelFetch( batchingTexture, ivec2( x, y ), 0 );
		vec4 v2 = texelFetch( batchingTexture, ivec2( x + 1, y ), 0 );
		vec4 v3 = texelFetch( batchingTexture, ivec2( x + 2, y ), 0 );
		vec4 v4 = texelFetch( batchingTexture, ivec2( x + 3, y ), 0 );
		return mat4( v1, v2, v3, v4 );
	}
	float getIndirectIndex( const in int i ) {
		int size = textureSize( batchingIdTexture, 0 ).x;
		int x = i % size;
		int y = i / size;
		return float( texelFetch( batchingIdTexture, ivec2( x, y ), 0 ).r );
	}
#endif
#ifdef USE_BATCHING_COLOR
	uniform sampler2D batchingColorTexture;
	vec4 getBatchingColor( const in float i ) {
		int size = textureSize( batchingColorTexture, 0 ).x;
		int j = int( i );
		int x = j % size;
		int y = j / size;
		return texelFetch( batchingColorTexture, ivec2( x, y ), 0 );
	}
#endif`,x0=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,b0=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,v0=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,_0=`float G_BlinnPhong_Implicit( ) {
	return 0.25;
}
float D_BlinnPhong( const in float shininess, const in float dotNH ) {
	return RECIPROCAL_PI * ( shininess * 0.5 + 1.0 ) * pow( dotNH, shininess );
}
vec3 BRDF_BlinnPhong( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, const in vec3 specularColor, const in float shininess ) {
	vec3 halfDir = normalize( lightDir + viewDir );
	float dotNH = saturate( dot( normal, halfDir ) );
	float dotVH = saturate( dot( viewDir, halfDir ) );
	vec3 F = F_Schlick( specularColor, 1.0, dotVH );
	float G = G_BlinnPhong_Implicit( );
	float D = D_BlinnPhong( shininess, dotNH );
	return F * ( G * D );
} // validated`,y0=`#ifdef USE_IRIDESCENCE
	const mat3 XYZ_TO_REC709 = mat3(
		 3.2404542, -0.9692660,  0.0556434,
		-1.5371385,  1.8760108, -0.2040259,
		-0.4985314,  0.0415560,  1.0572252
	);
	vec3 Fresnel0ToIor( vec3 fresnel0 ) {
		vec3 sqrtF0 = sqrt( fresnel0 );
		return ( vec3( 1.0 ) + sqrtF0 ) / ( vec3( 1.0 ) - sqrtF0 );
	}
	vec3 IorToFresnel0( vec3 transmittedIor, float incidentIor ) {
		return pow2( ( transmittedIor - vec3( incidentIor ) ) / ( transmittedIor + vec3( incidentIor ) ) );
	}
	float IorToFresnel0( float transmittedIor, float incidentIor ) {
		return pow2( ( transmittedIor - incidentIor ) / ( transmittedIor + incidentIor ));
	}
	vec3 evalSensitivity( float OPD, vec3 shift ) {
		float phase = 2.0 * PI * OPD * 1.0e-9;
		vec3 val = vec3( 5.4856e-13, 4.4201e-13, 5.2481e-13 );
		vec3 pos = vec3( 1.6810e+06, 1.7953e+06, 2.2084e+06 );
		vec3 var = vec3( 4.3278e+09, 9.3046e+09, 6.6121e+09 );
		vec3 xyz = val * sqrt( 2.0 * PI * var ) * cos( pos * phase + shift ) * exp( - pow2( phase ) * var );
		xyz.x += 9.7470e-14 * sqrt( 2.0 * PI * 4.5282e+09 ) * cos( 2.2399e+06 * phase + shift[ 0 ] ) * exp( - 4.5282e+09 * pow2( phase ) );
		xyz /= 1.0685e-7;
		vec3 rgb = XYZ_TO_REC709 * xyz;
		return rgb;
	}
	vec3 evalIridescence( float outsideIOR, float eta2, float cosTheta1, float thinFilmThickness, vec3 baseF0 ) {
		vec3 I;
		float iridescenceIOR = mix( outsideIOR, eta2, smoothstep( 0.0, 0.03, thinFilmThickness ) );
		float sinTheta2Sq = pow2( outsideIOR / iridescenceIOR ) * ( 1.0 - pow2( cosTheta1 ) );
		float cosTheta2Sq = 1.0 - sinTheta2Sq;
		if ( cosTheta2Sq < 0.0 ) {
			return vec3( 1.0 );
		}
		float cosTheta2 = sqrt( cosTheta2Sq );
		float R0 = IorToFresnel0( iridescenceIOR, outsideIOR );
		float R12 = F_Schlick( R0, 1.0, cosTheta1 );
		float T121 = 1.0 - R12;
		float phi12 = 0.0;
		if ( iridescenceIOR < outsideIOR ) phi12 = PI;
		float phi21 = PI - phi12;
		vec3 baseIOR = Fresnel0ToIor( clamp( baseF0, 0.0, 0.9999 ) );		vec3 R1 = IorToFresnel0( baseIOR, iridescenceIOR );
		vec3 R23 = F_Schlick( R1, 1.0, cosTheta2 );
		vec3 phi23 = vec3( 0.0 );
		if ( baseIOR[ 0 ] < iridescenceIOR ) phi23[ 0 ] = PI;
		if ( baseIOR[ 1 ] < iridescenceIOR ) phi23[ 1 ] = PI;
		if ( baseIOR[ 2 ] < iridescenceIOR ) phi23[ 2 ] = PI;
		float OPD = 2.0 * iridescenceIOR * thinFilmThickness * cosTheta2;
		vec3 phi = vec3( phi21 ) + phi23;
		vec3 R123 = clamp( R12 * R23, 1e-5, 0.9999 );
		vec3 r123 = sqrt( R123 );
		vec3 Rs = pow2( T121 ) * R23 / ( vec3( 1.0 ) - R123 );
		vec3 C0 = R12 + Rs;
		I = C0;
		vec3 Cm = Rs - T121;
		for ( int m = 1; m <= 2; ++ m ) {
			Cm *= r123;
			vec3 Sm = 2.0 * evalSensitivity( float( m ) * OPD, float( m ) * phi );
			I += Cm * Sm;
		}
		return max( I, vec3( 0.0 ) );
	}
#endif`,M0=`#ifdef USE_BUMPMAP
	uniform sampler2D bumpMap;
	uniform float bumpScale;
	vec2 dHdxy_fwd() {
		vec2 dSTdx = dFdx( vBumpMapUv );
		vec2 dSTdy = dFdy( vBumpMapUv );
		float Hll = bumpScale * texture2D( bumpMap, vBumpMapUv ).x;
		float dBx = bumpScale * texture2D( bumpMap, vBumpMapUv + dSTdx ).x - Hll;
		float dBy = bumpScale * texture2D( bumpMap, vBumpMapUv + dSTdy ).x - Hll;
		return vec2( dBx, dBy );
	}
	vec3 perturbNormalArb( vec3 surf_pos, vec3 surf_norm, vec2 dHdxy, float faceDirection ) {
		vec3 vSigmaX = normalize( dFdx( surf_pos.xyz ) );
		vec3 vSigmaY = normalize( dFdy( surf_pos.xyz ) );
		vec3 vN = surf_norm;
		vec3 R1 = cross( vSigmaY, vN );
		vec3 R2 = cross( vN, vSigmaX );
		float fDet = dot( vSigmaX, R1 ) * faceDirection;
		vec3 vGrad = sign( fDet ) * ( dHdxy.x * R1 + dHdxy.y * R2 );
		return normalize( abs( fDet ) * surf_norm - vGrad );
	}
#endif`,w0=`#if NUM_CLIPPING_PLANES > 0
	vec4 plane;
	#ifdef ALPHA_TO_COVERAGE
		float distanceToPlane, distanceGradient;
		float clipOpacity = 1.0;
		#pragma unroll_loop_start
		for ( int i = 0; i < UNION_CLIPPING_PLANES; i ++ ) {
			plane = clippingPlanes[ i ];
			distanceToPlane = - dot( vClipPosition, plane.xyz ) + plane.w;
			distanceGradient = fwidth( distanceToPlane ) / 2.0;
			clipOpacity *= smoothstep( - distanceGradient, distanceGradient, distanceToPlane );
			if ( clipOpacity == 0.0 ) discard;
		}
		#pragma unroll_loop_end
		#if UNION_CLIPPING_PLANES < NUM_CLIPPING_PLANES
			float unionClipOpacity = 1.0;
			#pragma unroll_loop_start
			for ( int i = UNION_CLIPPING_PLANES; i < NUM_CLIPPING_PLANES; i ++ ) {
				plane = clippingPlanes[ i ];
				distanceToPlane = - dot( vClipPosition, plane.xyz ) + plane.w;
				distanceGradient = fwidth( distanceToPlane ) / 2.0;
				unionClipOpacity *= 1.0 - smoothstep( - distanceGradient, distanceGradient, distanceToPlane );
			}
			#pragma unroll_loop_end
			clipOpacity *= 1.0 - unionClipOpacity;
		#endif
		diffuseColor.a *= clipOpacity;
		if ( diffuseColor.a == 0.0 ) discard;
	#else
		#pragma unroll_loop_start
		for ( int i = 0; i < UNION_CLIPPING_PLANES; i ++ ) {
			plane = clippingPlanes[ i ];
			if ( dot( vClipPosition, plane.xyz ) > plane.w ) discard;
		}
		#pragma unroll_loop_end
		#if UNION_CLIPPING_PLANES < NUM_CLIPPING_PLANES
			bool clipped = true;
			#pragma unroll_loop_start
			for ( int i = UNION_CLIPPING_PLANES; i < NUM_CLIPPING_PLANES; i ++ ) {
				plane = clippingPlanes[ i ];
				clipped = ( dot( vClipPosition, plane.xyz ) > plane.w ) && clipped;
			}
			#pragma unroll_loop_end
			if ( clipped ) discard;
		#endif
	#endif
#endif`,S0=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,E0=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,T0=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,A0=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#endif`,R0=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#endif`,C0=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec4 vColor;
#endif`,P0=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	vColor = vec4( 1.0 );
#endif
#ifdef USE_COLOR_ALPHA
	vColor *= color;
#elif defined( USE_COLOR )
	vColor.rgb *= color;
#endif
#ifdef USE_INSTANCING_COLOR
	vColor.rgb *= instanceColor.rgb;
#endif
#ifdef USE_BATCHING_COLOR
	vColor *= getBatchingColor( getIndirectIndex( gl_DrawID ) );
#endif`,I0=`#define PI 3.141592653589793
#define PI2 6.283185307179586
#define PI_HALF 1.5707963267948966
#define RECIPROCAL_PI 0.3183098861837907
#define RECIPROCAL_PI2 0.15915494309189535
#define EPSILON 1e-6
#ifndef saturate
#define saturate( a ) clamp( a, 0.0, 1.0 )
#endif
#define whiteComplement( a ) ( 1.0 - saturate( a ) )
float pow2( const in float x ) { return x*x; }
vec3 pow2( const in vec3 x ) { return x*x; }
float pow3( const in float x ) { return x*x*x; }
float pow4( const in float x ) { float x2 = x*x; return x2*x2; }
float max3( const in vec3 v ) { return max( max( v.x, v.y ), v.z ); }
float average( const in vec3 v ) { return dot( v, vec3( 0.3333333 ) ); }
highp float rand( const in vec2 uv ) {
	const highp float a = 12.9898, b = 78.233, c = 43758.5453;
	highp float dt = dot( uv.xy, vec2( a,b ) ), sn = mod( dt, PI );
	return fract( sin( sn ) * c );
}
#ifdef HIGH_PRECISION
	float precisionSafeLength( vec3 v ) { return length( v ); }
#else
	float precisionSafeLength( vec3 v ) {
		float maxComponent = max3( abs( v ) );
		return length( v / maxComponent ) * maxComponent;
	}
#endif
struct IncidentLight {
	vec3 color;
	vec3 direction;
	bool visible;
};
struct ReflectedLight {
	vec3 directDiffuse;
	vec3 directSpecular;
	vec3 indirectDiffuse;
	vec3 indirectSpecular;
};
#ifdef USE_ALPHAHASH
	varying vec3 vPosition;
#endif
vec3 transformDirection( in vec3 dir, in mat4 matrix ) {
	return normalize( ( matrix * vec4( dir, 0.0 ) ).xyz );
}
#define inverseTransformDirection transformDirectionByInverseViewMatrix
vec3 transformNormalByInverseViewMatrix( in vec3 normal, in mat4 viewMatrix ) {
	return normalize( ( vec4( normal, 0.0 ) * viewMatrix ).xyz );
}
vec3 transformDirectionByInverseViewMatrix( in vec3 dir, in mat4 viewMatrix ) {
	return normalize( ( vec4( dir, 0.0 ) * viewMatrix ).xyz );
}
bool isPerspectiveMatrix( mat4 m ) {
	return m[ 2 ][ 3 ] == - 1.0;
}
vec2 equirectUv( in vec3 dir ) {
	float u = atan( dir.z, dir.x ) * RECIPROCAL_PI2 + 0.5;
	float v = asin( clamp( dir.y, - 1.0, 1.0 ) ) * RECIPROCAL_PI + 0.5;
	return vec2( u, v );
}
vec3 BRDF_Lambert( const in vec3 diffuseColor ) {
	return RECIPROCAL_PI * diffuseColor;
}
vec3 F_Schlick( const in vec3 f0, const in float f90, const in float dotVH ) {
	float fresnel = exp2( ( - 5.55473 * dotVH - 6.98316 ) * dotVH );
	return f0 * ( 1.0 - fresnel ) + ( f90 * fresnel );
}
float F_Schlick( const in float f0, const in float f90, const in float dotVH ) {
	float fresnel = exp2( ( - 5.55473 * dotVH - 6.98316 ) * dotVH );
	return f0 * ( 1.0 - fresnel ) + ( f90 * fresnel );
} // validated`,D0=`#ifdef ENVMAP_TYPE_CUBE_UV
	#define cubeUV_minMipLevel 4.0
	#define cubeUV_minTileSize 16.0
	float getFace( vec3 direction ) {
		vec3 absDirection = abs( direction );
		float face = - 1.0;
		if ( absDirection.x > absDirection.z ) {
			if ( absDirection.x > absDirection.y )
				face = direction.x > 0.0 ? 0.0 : 3.0;
			else
				face = direction.y > 0.0 ? 1.0 : 4.0;
		} else {
			if ( absDirection.z > absDirection.y )
				face = direction.z > 0.0 ? 2.0 : 5.0;
			else
				face = direction.y > 0.0 ? 1.0 : 4.0;
		}
		return face;
	}
	vec2 getUV( vec3 direction, float face ) {
		vec2 uv;
		if ( face == 0.0 ) {
			uv = vec2( direction.z, direction.y ) / abs( direction.x );
		} else if ( face == 1.0 ) {
			uv = vec2( - direction.x, - direction.z ) / abs( direction.y );
		} else if ( face == 2.0 ) {
			uv = vec2( - direction.x, direction.y ) / abs( direction.z );
		} else if ( face == 3.0 ) {
			uv = vec2( - direction.z, direction.y ) / abs( direction.x );
		} else if ( face == 4.0 ) {
			uv = vec2( - direction.x, direction.z ) / abs( direction.y );
		} else {
			uv = vec2( direction.x, direction.y ) / abs( direction.z );
		}
		return 0.5 * ( uv + 1.0 );
	}
	vec3 bilinearCubeUV( sampler2D envMap, vec3 direction, float mipInt ) {
		float face = getFace( direction );
		float filterInt = max( cubeUV_minMipLevel - mipInt, 0.0 );
		mipInt = max( mipInt, cubeUV_minMipLevel );
		float faceSize = exp2( mipInt );
		highp vec2 uv = getUV( direction, face ) * ( faceSize - 2.0 ) + 1.0;
		if ( face > 2.0 ) {
			uv.y += faceSize;
			face -= 3.0;
		}
		uv.x += face * faceSize;
		uv.x += filterInt * 3.0 * cubeUV_minTileSize;
		uv.y += 4.0 * ( exp2( CUBEUV_MAX_MIP ) - faceSize );
		uv.x *= CUBEUV_TEXEL_WIDTH;
		uv.y *= CUBEUV_TEXEL_HEIGHT;
		#ifdef texture2DGradEXT
			return texture2DGradEXT( envMap, uv, vec2( 0.0 ), vec2( 0.0 ) ).rgb;
		#else
			return texture2D( envMap, uv ).rgb;
		#endif
	}
	#define cubeUV_r0 1.0
	#define cubeUV_m0 - 2.0
	#define cubeUV_r1 0.8
	#define cubeUV_m1 - 1.0
	#define cubeUV_r4 0.4
	#define cubeUV_m4 2.0
	#define cubeUV_r5 0.305
	#define cubeUV_m5 3.0
	#define cubeUV_r6 0.21
	#define cubeUV_m6 4.0
	float roughnessToMip( float roughness ) {
		float mip = 0.0;
		if ( roughness >= cubeUV_r1 ) {
			mip = ( cubeUV_r0 - roughness ) * ( cubeUV_m1 - cubeUV_m0 ) / ( cubeUV_r0 - cubeUV_r1 ) + cubeUV_m0;
		} else if ( roughness >= cubeUV_r4 ) {
			mip = ( cubeUV_r1 - roughness ) * ( cubeUV_m4 - cubeUV_m1 ) / ( cubeUV_r1 - cubeUV_r4 ) + cubeUV_m1;
		} else if ( roughness >= cubeUV_r5 ) {
			mip = ( cubeUV_r4 - roughness ) * ( cubeUV_m5 - cubeUV_m4 ) / ( cubeUV_r4 - cubeUV_r5 ) + cubeUV_m4;
		} else if ( roughness >= cubeUV_r6 ) {
			mip = ( cubeUV_r5 - roughness ) * ( cubeUV_m6 - cubeUV_m5 ) / ( cubeUV_r5 - cubeUV_r6 ) + cubeUV_m5;
		} else {
			mip = - 2.0 * log2( 1.16 * roughness );		}
		return mip;
	}
	vec4 textureCubeUV( sampler2D envMap, vec3 sampleDir, float roughness ) {
		float mip = clamp( roughnessToMip( roughness ), cubeUV_m0, CUBEUV_MAX_MIP );
		float mipF = fract( mip );
		float mipInt = floor( mip );
		vec3 color0 = bilinearCubeUV( envMap, sampleDir, mipInt );
		if ( mipF == 0.0 ) {
			return vec4( color0, 1.0 );
		} else {
			vec3 color1 = bilinearCubeUV( envMap, sampleDir, mipInt + 1.0 );
			return vec4( mix( color0, color1, mipF ), 1.0 );
		}
	}
#endif`,L0=`vec3 transformedNormal = objectNormal;
#ifdef USE_TANGENT
	vec3 transformedTangent = objectTangent;
#endif
#ifdef USE_BATCHING
	mat3 bm = mat3( batchingMatrix );
	transformedNormal /= vec3( dot( bm[ 0 ], bm[ 0 ] ), dot( bm[ 1 ], bm[ 1 ] ), dot( bm[ 2 ], bm[ 2 ] ) );
	transformedNormal = bm * transformedNormal;
	#ifdef USE_TANGENT
		transformedTangent = bm * transformedTangent;
	#endif
#endif
#ifdef USE_INSTANCING
	mat3 im = mat3( instanceMatrix );
	transformedNormal /= vec3( dot( im[ 0 ], im[ 0 ] ), dot( im[ 1 ], im[ 1 ] ), dot( im[ 2 ], im[ 2 ] ) );
	transformedNormal = im * transformedNormal;
	#ifdef USE_TANGENT
		transformedTangent = im * transformedTangent;
	#endif
#endif
transformedNormal = normalMatrix * transformedNormal;
#ifdef FLIP_SIDED
	transformedNormal = - transformedNormal;
#endif
#ifdef USE_TANGENT
	transformedTangent = ( modelViewMatrix * vec4( transformedTangent, 0.0 ) ).xyz;
#endif`,F0=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,N0=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,U0=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	#ifdef DECODE_VIDEO_TEXTURE_EMISSIVE
		emissiveColor = sRGBTransferEOTF( emissiveColor );
	#endif
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,O0=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,z0="gl_FragColor = linearToOutputTexel( gl_FragColor );",k0=`vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferEOTF( in vec4 value ) {
	return vec4( mix( pow( value.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), value.rgb * 0.0773993808, vec3( lessThanEqual( value.rgb, vec3( 0.04045 ) ) ) ), value.a );
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,B0=`#ifdef USE_ENVMAP
	#ifdef ENV_WORLDPOS
		vec3 cameraToFrag;
		if ( isOrthographic ) {
			cameraToFrag = normalize( vec3( - viewMatrix[ 0 ][ 2 ], - viewMatrix[ 1 ][ 2 ], - viewMatrix[ 2 ][ 2 ] ) );
		} else {
			cameraToFrag = normalize( vWorldPosition - cameraPosition );
		}
		vec3 worldNormal = transformNormalByInverseViewMatrix( normal, viewMatrix );
		#ifdef ENVMAP_MODE_REFLECTION
			vec3 reflectVec = reflect( cameraToFrag, worldNormal );
		#else
			vec3 reflectVec = refract( cameraToFrag, worldNormal, refractionRatio );
		#endif
	#else
		vec3 reflectVec = vReflect;
	#endif
	#ifdef ENVMAP_TYPE_CUBE
		vec4 envColor = textureCube( envMap, envMapRotation * reflectVec );
		#ifdef ENVMAP_BLENDING_MULTIPLY
			outgoingLight = mix( outgoingLight, outgoingLight * envColor.xyz, specularStrength * reflectivity );
		#elif defined( ENVMAP_BLENDING_MIX )
			outgoingLight = mix( outgoingLight, envColor.xyz, specularStrength * reflectivity );
		#elif defined( ENVMAP_BLENDING_ADD )
			outgoingLight += envColor.xyz * specularStrength * reflectivity;
		#endif
	#endif
#endif`,H0=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
#endif`,G0=`#ifdef USE_ENVMAP
	uniform float reflectivity;
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		varying vec3 vWorldPosition;
		uniform float refractionRatio;
	#else
		varying vec3 vReflect;
	#endif
#endif`,V0=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,W0=`#ifdef USE_ENVMAP
	#ifdef ENV_WORLDPOS
		vWorldPosition = worldPosition.xyz;
	#else
		vec3 cameraToVertex;
		if ( isOrthographic ) {
			cameraToVertex = normalize( vec3( - viewMatrix[ 0 ][ 2 ], - viewMatrix[ 1 ][ 2 ], - viewMatrix[ 2 ][ 2 ] ) );
		} else {
			cameraToVertex = normalize( worldPosition.xyz - cameraPosition );
		}
		vec3 worldNormal = transformNormalByInverseViewMatrix( transformedNormal, viewMatrix );
		#ifdef ENVMAP_MODE_REFLECTION
			vReflect = reflect( cameraToVertex, worldNormal );
		#else
			vReflect = refract( cameraToVertex, worldNormal, refractionRatio );
		#endif
	#endif
#endif`,q0=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,X0=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,j0=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,K0=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,Y0=`#ifdef USE_GRADIENTMAP
	uniform sampler2D gradientMap;
#endif
vec3 getGradientIrradiance( vec3 normal, vec3 lightDirection ) {
	float dotNL = dot( normal, lightDirection );
	vec2 coord = vec2( dotNL * 0.5 + 0.5, 0.0 );
	#ifdef USE_GRADIENTMAP
		return vec3( texture2D( gradientMap, coord ).r );
	#else
		vec2 fw = fwidth( coord ) * 0.5;
		return mix( vec3( 0.7 ), vec3( 1.0 ), smoothstep( 0.7 - fw.x, 0.7 + fw.x, coord.x ) );
	#endif
}`,J0=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,Z0=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,Q0=`varying vec3 vViewPosition;
struct LambertMaterial {
	vec3 diffuseColor;
	float specularStrength;
};
void RE_Direct_Lambert( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in LambertMaterial material, inout ReflectedLight reflectedLight ) {
	float dotNL = saturate( dot( geometryNormal, directLight.direction ) );
	vec3 irradiance = dotNL * directLight.color;
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
void RE_IndirectDiffuse_Lambert( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in LambertMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
#define RE_Direct				RE_Direct_Lambert
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,$0=`uniform bool receiveShadow;
uniform vec3 ambientLightColor;
#if defined( USE_LIGHT_PROBES )
	uniform vec3 lightProbe[ 9 ];
#endif
vec3 shGetIrradianceAt( in vec3 normal, in vec3 shCoefficients[ 9 ] ) {
	float x = normal.x, y = normal.y, z = normal.z;
	vec3 result = shCoefficients[ 0 ] * 0.886227;
	result += shCoefficients[ 1 ] * 2.0 * 0.511664 * y;
	result += shCoefficients[ 2 ] * 2.0 * 0.511664 * z;
	result += shCoefficients[ 3 ] * 2.0 * 0.511664 * x;
	result += shCoefficients[ 4 ] * 2.0 * 0.429043 * x * y;
	result += shCoefficients[ 5 ] * 2.0 * 0.429043 * y * z;
	result += shCoefficients[ 6 ] * ( 0.743125 * z * z - 0.247708 );
	result += shCoefficients[ 7 ] * 2.0 * 0.429043 * x * z;
	result += shCoefficients[ 8 ] * 0.429043 * ( x * x - y * y );
	return result;
}
vec3 getLightProbeIrradiance( const in vec3 lightProbe[ 9 ], const in vec3 normal ) {
	vec3 worldNormal = transformNormalByInverseViewMatrix( normal, viewMatrix );
	vec3 irradiance = shGetIrradianceAt( worldNormal, lightProbe );
	return irradiance;
}
vec3 getAmbientLightIrradiance( const in vec3 ambientLightColor ) {
	vec3 irradiance = ambientLightColor;
	return irradiance;
}
float getDistanceAttenuation( const in float lightDistance, const in float cutoffDistance, const in float decayExponent ) {
	float distanceFalloff = 1.0 / max( pow( lightDistance, decayExponent ), 0.01 );
	if ( cutoffDistance > 0.0 ) {
		distanceFalloff *= pow2( saturate( 1.0 - pow4( lightDistance / cutoffDistance ) ) );
	}
	return distanceFalloff;
}
float getSpotAttenuation( const in float coneCosine, const in float penumbraCosine, const in float angleCosine ) {
	return smoothstep( coneCosine, penumbraCosine, angleCosine );
}
#if NUM_SUN_LIGHTS > 0
	struct SunLight {
		vec3 direction;
		vec3 color;
	};
	uniform SunLight sunLights[ NUM_SUN_LIGHTS ];
	void getSunLightInfo( const in SunLight sunLight, out IncidentLight light ) {
		light.color = sunLight.color;
		light.direction = sunLight.direction;
		light.visible = true;
	}
#endif
#if NUM_DIR_LIGHTS > 0
	struct DirectionalLight {
		vec3 direction;
		vec3 color;
	};
	uniform DirectionalLight directionalLights[ NUM_DIR_LIGHTS ];
	void getDirectionalLightInfo( const in DirectionalLight directionalLight, out IncidentLight light ) {
		light.color = directionalLight.color;
		light.direction = directionalLight.direction;
		light.visible = true;
	}
#endif
#if NUM_POINT_LIGHTS > 0
	struct PointLight {
		vec3 position;
		vec3 color;
		float distance;
		float decay;
	};
	uniform PointLight pointLights[ NUM_POINT_LIGHTS ];
	void getPointLightInfo( const in PointLight pointLight, const in vec3 geometryPosition, out IncidentLight light ) {
		vec3 lVector = pointLight.position - geometryPosition;
		light.direction = normalize( lVector );
		float lightDistance = length( lVector );
		light.color = pointLight.color;
		light.color *= getDistanceAttenuation( lightDistance, pointLight.distance, pointLight.decay );
		light.visible = ( light.color != vec3( 0.0 ) );
	}
#endif
#if NUM_SPOT_LIGHTS > 0
	struct SpotLight {
		vec3 position;
		vec3 direction;
		vec3 color;
		float distance;
		float decay;
		float coneCos;
		float penumbraCos;
	};
	uniform SpotLight spotLights[ NUM_SPOT_LIGHTS ];
	void getSpotLightInfo( const in SpotLight spotLight, const in vec3 geometryPosition, out IncidentLight light ) {
		vec3 lVector = spotLight.position - geometryPosition;
		light.direction = normalize( lVector );
		float angleCos = dot( light.direction, spotLight.direction );
		float spotAttenuation = getSpotAttenuation( spotLight.coneCos, spotLight.penumbraCos, angleCos );
		if ( spotAttenuation > 0.0 ) {
			float lightDistance = length( lVector );
			light.color = spotLight.color * spotAttenuation;
			light.color *= getDistanceAttenuation( lightDistance, spotLight.distance, spotLight.decay );
			light.visible = ( light.color != vec3( 0.0 ) );
		} else {
			light.color = vec3( 0.0 );
			light.visible = false;
		}
	}
#endif
#if NUM_RECT_AREA_LIGHTS > 0
	struct RectAreaLight {
		vec3 color;
		vec3 position;
		vec3 halfWidth;
		vec3 halfHeight;
	};
	uniform sampler2D ltc_1;	uniform sampler2D ltc_2;
	uniform RectAreaLight rectAreaLights[ NUM_RECT_AREA_LIGHTS ];
#endif
#if NUM_HEMI_LIGHTS > 0
	struct HemisphereLight {
		vec3 direction;
		vec3 skyColor;
		vec3 groundColor;
	};
	uniform HemisphereLight hemisphereLights[ NUM_HEMI_LIGHTS ];
	vec3 getHemisphereLightIrradiance( const in HemisphereLight hemiLight, const in vec3 normal ) {
		float dotNL = dot( normal, hemiLight.direction );
		float hemiDiffuseWeight = 0.5 * dotNL + 0.5;
		vec3 irradiance = mix( hemiLight.groundColor, hemiLight.skyColor, hemiDiffuseWeight );
		return irradiance;
	}
#endif
#include <lightprobes_pars_fragment>`,ex=`#ifdef USE_ENVMAP
	vec3 getIBLIrradiance( const in vec3 normal ) {
		#ifdef ENVMAP_TYPE_CUBE_UV
			vec3 worldNormal = transformNormalByInverseViewMatrix( normal, viewMatrix );
			vec4 envMapColor = textureCubeUV( envMap, envMapRotation * worldNormal, 1.0 );
			return PI * envMapColor.rgb * envMapIntensity;
		#else
			return vec3( 0.0 );
		#endif
	}
	vec3 getIBLRadiance( const in vec3 viewDir, const in vec3 normal, const in float roughness ) {
		#ifdef ENVMAP_TYPE_CUBE_UV
			vec3 reflectVec = reflect( - viewDir, normal );
			reflectVec = normalize( mix( reflectVec, normal, pow4( roughness ) ) );
			reflectVec = transformDirectionByInverseViewMatrix( reflectVec, viewMatrix );
			vec4 envMapColor = textureCubeUV( envMap, envMapRotation * reflectVec, roughness );
			return envMapColor.rgb * envMapIntensity;
		#else
			return vec3( 0.0 );
		#endif
	}
	#ifdef USE_RETROREFLECTION
		vec3 getIBLRetroRadiance( const in vec3 viewDir, const in vec3 normal, const in float roughness ) {
			#ifdef ENVMAP_TYPE_CUBE_UV
				vec3 retroVec = normalize( mix( viewDir, normal, pow4( roughness ) ) );
				retroVec = transformDirectionByInverseViewMatrix( retroVec, viewMatrix );
				vec4 envMapColor = textureCubeUV( envMap, envMapRotation * retroVec, roughness );
				return envMapColor.rgb * envMapIntensity;
			#else
				return vec3( 0.0 );
			#endif
		}
	#endif
	#ifdef USE_ANISOTROPY
		vec3 getIBLAnisotropyRadiance( const in vec3 viewDir, const in vec3 normal, const in float roughness, const in vec3 bitangent, const in float anisotropy ) {
			#ifdef ENVMAP_TYPE_CUBE_UV
				vec3 bentNormal = cross( bitangent, viewDir );
				bentNormal = normalize( cross( bentNormal, bitangent ) );
				bentNormal = normalize( mix( bentNormal, normal, pow2( pow2( 1.0 - anisotropy * ( 1.0 - roughness ) ) ) ) );
				return getIBLRadiance( viewDir, bentNormal, roughness );
			#else
				return vec3( 0.0 );
			#endif
		}
		#ifdef USE_RETROREFLECTION
			vec3 getIBLAnisotropyRetroRadiance( const in vec3 viewDir, const in vec3 normal, const in float roughness, const in vec3 bitangent, const in float anisotropy ) {
				#ifdef ENVMAP_TYPE_CUBE_UV
					vec3 bentNormal = cross( bitangent, viewDir );
					bentNormal = normalize( cross( bentNormal, bitangent ) );
					bentNormal = normalize( mix( bentNormal, normal, pow2( pow2( 1.0 - anisotropy * ( 1.0 - roughness ) ) ) ) );
					return getIBLRetroRadiance( viewDir, bentNormal, roughness );
				#else
					return vec3( 0.0 );
				#endif
			}
		#endif
	#endif
#endif`,tx=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,nx=`varying vec3 vViewPosition;
struct ToonMaterial {
	vec3 diffuseColor;
};
void RE_Direct_Toon( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in ToonMaterial material, inout ReflectedLight reflectedLight ) {
	vec3 irradiance = getGradientIrradiance( geometryNormal, directLight.direction ) * directLight.color;
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
void RE_IndirectDiffuse_Toon( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in ToonMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
#define RE_Direct				RE_Direct_Toon
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,ix=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,sx=`varying vec3 vViewPosition;
struct BlinnPhongMaterial {
	vec3 diffuseColor;
	vec3 specularColor;
	float specularShininess;
	float specularStrength;
};
void RE_Direct_BlinnPhong( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in BlinnPhongMaterial material, inout ReflectedLight reflectedLight ) {
	float dotNL = saturate( dot( geometryNormal, directLight.direction ) );
	vec3 irradiance = dotNL * directLight.color;
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
	reflectedLight.directSpecular += irradiance * BRDF_BlinnPhong( directLight.direction, geometryViewDir, geometryNormal, material.specularColor, material.specularShininess ) * material.specularStrength;
}
void RE_IndirectDiffuse_BlinnPhong( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in BlinnPhongMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
#define RE_Direct				RE_Direct_BlinnPhong
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,rx=`PhysicalMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.diffuseContribution = diffuseColor.rgb * ( 1.0 - metalnessFactor );
material.metalness = metalnessFactor;
vec3 dxy = max( abs( dFdx( nonPerturbedNormal ) ), abs( dFdy( nonPerturbedNormal ) ) );
float geometryRoughness = max( max( dxy.x, dxy.y ), dxy.z );
material.roughness = max( roughnessFactor, 0.0525 );material.roughness += geometryRoughness;
material.roughness = min( material.roughness, 1.0 );
#ifdef IOR
	material.ior = ior;
	#ifdef USE_SPECULAR
		float specularIntensityFactor = specularIntensity;
		vec3 specularColorFactor = specularColor;
		#ifdef USE_SPECULAR_COLORMAP
			specularColorFactor *= texture2D( specularColorMap, vSpecularColorMapUv ).rgb;
		#endif
		#ifdef USE_SPECULAR_INTENSITYMAP
			specularIntensityFactor *= texture2D( specularIntensityMap, vSpecularIntensityMapUv ).a;
		#endif
		material.specularF90 = mix( specularIntensityFactor, 1.0, metalnessFactor );
	#else
		float specularIntensityFactor = 1.0;
		vec3 specularColorFactor = vec3( 1.0 );
		material.specularF90 = 1.0;
	#endif
	material.specularColor = min( pow2( ( material.ior - 1.0 ) / ( material.ior + 1.0 ) ) * specularColorFactor, vec3( 1.0 ) ) * specularIntensityFactor;
	material.specularColorBlended = mix( material.specularColor, diffuseColor.rgb, metalnessFactor );
#else
	material.specularColor = vec3( 0.04 );
	material.specularColorBlended = mix( material.specularColor, diffuseColor.rgb, metalnessFactor );
	material.specularF90 = 1.0;
#endif
#ifdef USE_CLEARCOAT
	material.clearcoat = clearcoat;
	material.clearcoatRoughness = clearcoatRoughness;
	material.clearcoatF0 = vec3( 0.04 );
	material.clearcoatF90 = 1.0;
	#ifdef USE_CLEARCOATMAP
		material.clearcoat *= texture2D( clearcoatMap, vClearcoatMapUv ).x;
	#endif
	#ifdef USE_CLEARCOAT_ROUGHNESSMAP
		material.clearcoatRoughness *= texture2D( clearcoatRoughnessMap, vClearcoatRoughnessMapUv ).y;
	#endif
	material.clearcoat = saturate( material.clearcoat );	material.clearcoatRoughness = max( material.clearcoatRoughness, 0.0525 );
	material.clearcoatRoughness += geometryRoughness;
	material.clearcoatRoughness = min( material.clearcoatRoughness, 1.0 );
#endif
#ifdef USE_DISPERSION
	material.dispersion = dispersion;
#endif
#ifdef USE_RETROREFLECTION
	material.retroreflectivity = retroreflectivity;
#endif
#ifdef USE_IRIDESCENCE
	material.iridescence = iridescence;
	material.iridescenceIOR = iridescenceIOR;
	#ifdef USE_IRIDESCENCEMAP
		material.iridescence *= texture2D( iridescenceMap, vIridescenceMapUv ).r;
	#endif
	#ifdef USE_IRIDESCENCE_THICKNESSMAP
		material.iridescenceThickness = (iridescenceThicknessMaximum - iridescenceThicknessMinimum) * texture2D( iridescenceThicknessMap, vIridescenceThicknessMapUv ).g + iridescenceThicknessMinimum;
	#else
		material.iridescenceThickness = iridescenceThicknessMaximum;
	#endif
#endif
#ifdef USE_SHEEN
	material.sheenColor = sheenColor;
	#ifdef USE_SHEEN_COLORMAP
		material.sheenColor *= texture2D( sheenColorMap, vSheenColorMapUv ).rgb;
	#endif
	material.sheenRoughness = clamp( sheenRoughness, 0.0001, 1.0 );
	#ifdef USE_SHEEN_ROUGHNESSMAP
		material.sheenRoughness *= texture2D( sheenRoughnessMap, vSheenRoughnessMapUv ).a;
	#endif
#endif
#ifdef USE_ANISOTROPY
	#ifdef USE_ANISOTROPYMAP
		mat2 anisotropyMat = mat2( anisotropyVector.x, anisotropyVector.y, - anisotropyVector.y, anisotropyVector.x );
		vec3 anisotropyPolar = texture2D( anisotropyMap, vAnisotropyMapUv ).rgb;
		vec2 anisotropyV = anisotropyMat * normalize( 2.0 * anisotropyPolar.rg - vec2( 1.0 ) ) * anisotropyPolar.b;
	#else
		vec2 anisotropyV = anisotropyVector;
	#endif
	material.anisotropy = length( anisotropyV );
	if( material.anisotropy == 0.0 ) {
		anisotropyV = vec2( 1.0, 0.0 );
	} else {
		anisotropyV /= material.anisotropy;
		material.anisotropy = saturate( material.anisotropy );
	}
	material.alphaT = mix( pow2( material.roughness ), 1.0, pow2( material.anisotropy ) );
	material.anisotropyT = tbn[ 0 ] * anisotropyV.x + tbn[ 1 ] * anisotropyV.y;
	material.anisotropyB = tbn[ 1 ] * anisotropyV.x - tbn[ 0 ] * anisotropyV.y;
#endif`,ax=`uniform sampler2D dfgLUT;
struct PhysicalMaterial {
	vec3 diffuseColor;
	vec3 diffuseContribution;
	vec3 specularColor;
	vec3 specularColorBlended;
	float roughness;
	float metalness;
	float specularF90;
	float dispersion;
	vec2 dfg;
	vec3 multiScatteringCompensation;
	#ifdef USE_RETROREFLECTION
		float retroreflectivity;
	#endif
	#ifdef USE_CLEARCOAT
		float clearcoat;
		float clearcoatRoughness;
		vec3 clearcoatF0;
		float clearcoatF90;
	#endif
	#ifdef USE_IRIDESCENCE
		float iridescence;
		float iridescenceIOR;
		float iridescenceThickness;
		vec3 iridescenceFresnel;
		vec3 iridescenceF0Dielectric;
		vec3 iridescenceF0Metallic;
	#endif
	#ifdef USE_SHEEN
		vec3 sheenColor;
		float sheenRoughness;
	#endif
	#ifdef IOR
		float ior;
	#endif
	#ifdef USE_TRANSMISSION
		float transmission;
		float transmissionAlpha;
		float thickness;
		float attenuationDistance;
		vec3 attenuationColor;
	#endif
	#ifdef USE_ANISOTROPY
		float anisotropy;
		float alphaT;
		vec3 anisotropyT;
		vec3 anisotropyB;
	#endif
};
vec3 clearcoatSpecularDirect = vec3( 0.0 );
vec3 clearcoatSpecularIndirect = vec3( 0.0 );
vec3 sheenSpecularDirect = vec3( 0.0 );
vec3 sheenSpecularIndirect = vec3(0.0 );
vec3 Schlick_to_F0( const in vec3 f, const in float f90, const in float dotVH ) {
    float x = clamp( 1.0 - dotVH, 0.0, 1.0 );
    float x2 = x * x;
    float x5 = clamp( x * x2 * x2, 0.0, 0.9999 );
    return ( f - vec3( f90 ) * x5 ) / ( 1.0 - x5 );
}
float V_GGX_SmithCorrelated( const in float alpha, const in float dotNL, const in float dotNV ) {
	float a2 = pow2( alpha );
	float gv = dotNL * sqrt( a2 + ( 1.0 - a2 ) * pow2( dotNV ) );
	float gl = dotNV * sqrt( a2 + ( 1.0 - a2 ) * pow2( dotNL ) );
	return 0.5 / max( gv + gl, EPSILON );
}
float D_GGX( const in float alpha, const in float dotNH ) {
	float a2 = pow2( alpha );
	float denom = pow2( dotNH ) * ( a2 - 1.0 ) + 1.0;
	return RECIPROCAL_PI * a2 / pow2( denom );
}
#ifdef USE_ANISOTROPY
	float V_GGX_SmithCorrelated_Anisotropic( const in float alphaT, const in float alphaB, const in float dotTV, const in float dotBV, const in float dotTL, const in float dotBL, const in float dotNV, const in float dotNL ) {
		float gv = dotNL * length( vec3( alphaT * dotTV, alphaB * dotBV, dotNV ) );
		float gl = dotNV * length( vec3( alphaT * dotTL, alphaB * dotBL, dotNL ) );
		return 0.5 / max( gv + gl, EPSILON );
	}
	float D_GGX_Anisotropic( const in float alphaT, const in float alphaB, const in float dotNH, const in float dotTH, const in float dotBH ) {
		float a2 = alphaT * alphaB;
		highp vec3 v = vec3( alphaB * dotTH, alphaT * dotBH, a2 * dotNH );
		highp float v2 = dot( v, v );
		float w2 = a2 / v2;
		return RECIPROCAL_PI * a2 * pow2 ( w2 );
	}
#endif
#ifdef USE_CLEARCOAT
	vec3 BRDF_GGX_Clearcoat( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, const in PhysicalMaterial material) {
		vec3 f0 = material.clearcoatF0;
		float f90 = material.clearcoatF90;
		float roughness = material.clearcoatRoughness;
		float alpha = pow2( roughness );
		vec3 halfDir = normalize( lightDir + viewDir );
		float dotNL = saturate( dot( normal, lightDir ) );
		float dotNV = saturate( dot( normal, viewDir ) );
		float dotNH = saturate( dot( normal, halfDir ) );
		float dotVH = saturate( dot( viewDir, halfDir ) );
		vec3 F = F_Schlick( f0, f90, dotVH );
		float V = V_GGX_SmithCorrelated( alpha, dotNL, dotNV );
		float D = D_GGX( alpha, dotNH );
		return F * ( V * D );
	}
#endif
vec3 BRDF_GGX( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, const in PhysicalMaterial material ) {
	vec3 f0 = material.specularColorBlended;
	float f90 = material.specularF90;
	float roughness = material.roughness;
	float alpha = pow2( roughness );
	vec3 halfDir = normalize( lightDir + viewDir );
	float dotNL = saturate( dot( normal, lightDir ) );
	float dotNV = saturate( dot( normal, viewDir ) );
	float dotNH = saturate( dot( normal, halfDir ) );
	float dotVH = saturate( dot( viewDir, halfDir ) );
	vec3 F = F_Schlick( f0, f90, dotVH );
	#ifdef USE_IRIDESCENCE
		F = mix( F, material.iridescenceFresnel, material.iridescence );
	#endif
	#ifdef USE_ANISOTROPY
		float dotTL = dot( material.anisotropyT, lightDir );
		float dotTV = dot( material.anisotropyT, viewDir );
		float dotTH = dot( material.anisotropyT, halfDir );
		float dotBL = dot( material.anisotropyB, lightDir );
		float dotBV = dot( material.anisotropyB, viewDir );
		float dotBH = dot( material.anisotropyB, halfDir );
		float V = V_GGX_SmithCorrelated_Anisotropic( material.alphaT, alpha, dotTV, dotBV, dotTL, dotBL, dotNV, dotNL );
		float D = D_GGX_Anisotropic( material.alphaT, alpha, dotNH, dotTH, dotBH );
	#else
		float V = V_GGX_SmithCorrelated( alpha, dotNL, dotNV );
		float D = D_GGX( alpha, dotNH );
	#endif
	return F * ( V * D );
}
vec2 LTC_Uv( const in vec3 N, const in vec3 V, const in float roughness ) {
	const float LUT_SIZE = 64.0;
	const float LUT_SCALE = ( LUT_SIZE - 1.0 ) / LUT_SIZE;
	const float LUT_BIAS = 0.5 / LUT_SIZE;
	float dotNV = saturate( dot( N, V ) );
	vec2 uv = vec2( roughness, sqrt( 1.0 - dotNV ) );
	uv = uv * LUT_SCALE + LUT_BIAS;
	return uv;
}
float LTC_ClippedSphereFormFactor( const in vec3 f ) {
	float l = length( f );
	return max( ( l * l + f.z ) / ( l + 1.0 ), 0.0 );
}
vec3 LTC_EdgeVectorFormFactor( const in vec3 v1, const in vec3 v2 ) {
	float x = dot( v1, v2 );
	float y = abs( x );
	float a = 0.8543985 + ( 0.4965155 + 0.0145206 * y ) * y;
	float b = 3.4175940 + ( 4.1616724 + y ) * y;
	float v = a / b;
	float theta_sintheta = ( x > 0.0 ) ? v : 0.5 * inversesqrt( max( 1.0 - x * x, 1e-7 ) ) - v;
	return cross( v1, v2 ) * theta_sintheta;
}
vec3 LTC_Evaluate( const in vec3 N, const in vec3 V, const in vec3 P, const in mat3 mInv, const in vec3 rectCoords[ 4 ] ) {
	vec3 v1 = rectCoords[ 1 ] - rectCoords[ 0 ];
	vec3 v2 = rectCoords[ 3 ] - rectCoords[ 0 ];
	vec3 lightNormal = cross( v1, v2 );
	if( dot( lightNormal, P - rectCoords[ 0 ] ) < 0.0 ) return vec3( 0.0 );
	vec3 T1, T2;
	T1 = normalize( V - N * dot( V, N ) );
	T2 = - cross( N, T1 );
	mat3 mat = mInv * transpose( mat3( T1, T2, N ) );
	vec3 coords[ 4 ];
	coords[ 0 ] = mat * ( rectCoords[ 0 ] - P );
	coords[ 1 ] = mat * ( rectCoords[ 1 ] - P );
	coords[ 2 ] = mat * ( rectCoords[ 2 ] - P );
	coords[ 3 ] = mat * ( rectCoords[ 3 ] - P );
	coords[ 0 ] = normalize( coords[ 0 ] );
	coords[ 1 ] = normalize( coords[ 1 ] );
	coords[ 2 ] = normalize( coords[ 2 ] );
	coords[ 3 ] = normalize( coords[ 3 ] );
	vec3 vectorFormFactor = vec3( 0.0 );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 0 ], coords[ 1 ] );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 1 ], coords[ 2 ] );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 2 ], coords[ 3 ] );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 3 ], coords[ 0 ] );
	float result = LTC_ClippedSphereFormFactor( vectorFormFactor );
	return vec3( result );
}
#if defined( USE_SHEEN )
float D_Charlie( float roughness, float dotNH ) {
	float alpha = pow2( roughness );
	float invAlpha = 1.0 / alpha;
	float cos2h = dotNH * dotNH;
	float sin2h = max( 1.0 - cos2h, 0.0078125 );
	return ( 2.0 + invAlpha ) * pow( sin2h, invAlpha * 0.5 ) / ( 2.0 * PI );
}
float V_Neubelt( float dotNV, float dotNL ) {
	return saturate( 1.0 / ( 4.0 * ( dotNL + dotNV - dotNL * dotNV ) ) );
}
vec3 BRDF_Sheen( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, vec3 sheenColor, const in float sheenRoughness ) {
	vec3 halfDir = normalize( lightDir + viewDir );
	float dotNL = saturate( dot( normal, lightDir ) );
	float dotNV = saturate( dot( normal, viewDir ) );
	float dotNH = saturate( dot( normal, halfDir ) );
	float D = D_Charlie( sheenRoughness, dotNH );
	float V = V_Neubelt( dotNV, dotNL );
	return sheenColor * ( D * V );
}
#endif
float IBLSheenBRDF( const in vec3 normal, const in vec3 viewDir, const in float roughness ) {
	float dotNV = saturate( dot( normal, viewDir ) );
	float r2 = roughness * roughness;
	float rInv = 1.0 / ( roughness + 0.1 );
	float a = -1.9362 + 1.0678 * roughness + 0.4573 * r2 - 0.8469 * rInv;
	float b = -0.6014 + 0.5538 * roughness - 0.4670 * r2 - 0.1255 * rInv;
	float DG = exp( a * dotNV + b );
	return saturate( DG );
}
vec3 EnvironmentBRDF( const in vec3 normal, const in vec3 viewDir, const in vec3 specularColor, const in float specularF90, const in float roughness ) {
	float dotNV = saturate( dot( normal, viewDir ) );
	vec2 fab = texture2D( dfgLUT, vec2( roughness, dotNV ) ).rg;
	return specularColor * fab.x + specularF90 * fab.y;
}
#ifdef USE_IRIDESCENCE
void computeMultiscatteringIridescence( const in vec2 fab, const in vec3 specularColor, const in float specularF90, const in float iridescence, const in vec3 iridescenceF0, inout vec3 singleScatter, inout vec3 multiScatter ) {
#else
void computeMultiscattering( const in vec2 fab, const in vec3 specularColor, const in float specularF90, inout vec3 singleScatter, inout vec3 multiScatter ) {
#endif
	#ifdef USE_IRIDESCENCE
		vec3 Fr = mix( specularColor, iridescenceF0, iridescence );
	#else
		vec3 Fr = specularColor;
	#endif
	vec3 FssEss = Fr * fab.x + specularF90 * fab.y;
	float Ess = fab.x + fab.y;
	float Ems = 1.0 - Ess;
	vec3 Favg = Fr + ( 1.0 - Fr ) * 0.047619;	vec3 Fms = FssEss * Favg / ( 1.0 - Ems * Favg );
	singleScatter += FssEss;
	multiScatter += Fms * Ems;
}
#if NUM_RECT_AREA_LIGHTS > 0
	void RE_Direct_RectArea_Physical( const in RectAreaLight rectAreaLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight ) {
		vec3 normal = geometryNormal;
		vec3 viewDir = geometryViewDir;
		vec3 position = geometryPosition;
		vec3 lightPos = rectAreaLight.position;
		vec3 halfWidth = rectAreaLight.halfWidth;
		vec3 halfHeight = rectAreaLight.halfHeight;
		vec3 lightColor = rectAreaLight.color;
		float roughness = material.roughness;
		vec3 rectCoords[ 4 ];
		rectCoords[ 0 ] = lightPos + halfWidth - halfHeight;		rectCoords[ 1 ] = lightPos - halfWidth - halfHeight;
		rectCoords[ 2 ] = lightPos - halfWidth + halfHeight;
		rectCoords[ 3 ] = lightPos + halfWidth + halfHeight;
		vec2 uv = LTC_Uv( normal, viewDir, roughness );
		vec4 t1 = texture2D( ltc_1, uv );
		vec4 t2 = texture2D( ltc_2, uv );
		mat3 mInv = mat3(
			vec3( t1.x, 0, t1.y ),
			vec3(    0, 1,    0 ),
			vec3( t1.z, 0, t1.w )
		);
		vec3 fresnel = ( material.specularColorBlended * t2.x + ( material.specularF90 - material.specularColorBlended ) * t2.y );
		reflectedLight.directSpecular += lightColor * fresnel * LTC_Evaluate( normal, viewDir, position, mInv, rectCoords );
		reflectedLight.directDiffuse += lightColor * material.diffuseContribution * LTC_Evaluate( normal, viewDir, position, mat3( 1.0 ), rectCoords );
		#ifdef USE_CLEARCOAT
			vec3 Ncc = geometryClearcoatNormal;
			vec2 uvClearcoat = LTC_Uv( Ncc, viewDir, material.clearcoatRoughness );
			vec4 t1Clearcoat = texture2D( ltc_1, uvClearcoat );
			vec4 t2Clearcoat = texture2D( ltc_2, uvClearcoat );
			mat3 mInvClearcoat = mat3(
				vec3( t1Clearcoat.x, 0, t1Clearcoat.y ),
				vec3(             0, 1,             0 ),
				vec3( t1Clearcoat.z, 0, t1Clearcoat.w )
			);
			vec3 fresnelClearcoat = material.clearcoatF0 * t2Clearcoat.x + ( material.clearcoatF90 - material.clearcoatF0 ) * t2Clearcoat.y;
			clearcoatSpecularDirect += lightColor * fresnelClearcoat * LTC_Evaluate( Ncc, viewDir, position, mInvClearcoat, rectCoords );
		#endif
	}
#endif
void RE_Direct_Physical( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight ) {
	float dotNL = saturate( dot( geometryNormal, directLight.direction ) );
	vec3 irradiance = dotNL * directLight.color;
	#ifdef USE_CLEARCOAT
		float dotNLcc = saturate( dot( geometryClearcoatNormal, directLight.direction ) );
		vec3 ccIrradiance = dotNLcc * directLight.color;
		clearcoatSpecularDirect += ccIrradiance * BRDF_GGX_Clearcoat( directLight.direction, geometryViewDir, geometryClearcoatNormal, material );
	#endif
	#ifdef USE_SHEEN
 
 		sheenSpecularDirect += irradiance * BRDF_Sheen( directLight.direction, geometryViewDir, geometryNormal, material.sheenColor, material.sheenRoughness );
 
 		float sheenAlbedoV = IBLSheenBRDF( geometryNormal, geometryViewDir, material.sheenRoughness );
 		float sheenAlbedoL = IBLSheenBRDF( geometryNormal, directLight.direction, material.sheenRoughness );
 
 		float sheenEnergyComp = 1.0 - max3( material.sheenColor ) * max( sheenAlbedoV, sheenAlbedoL );
 
 		irradiance *= sheenEnergyComp;
 
 	#endif
	vec3 specularBRDF = BRDF_GGX( directLight.direction, geometryViewDir, geometryNormal, material );
	#ifdef USE_RETROREFLECTION
		vec3 retroViewDir = reflect( - geometryViewDir, geometryNormal );
		vec3 retroSpecularBRDF = BRDF_GGX( directLight.direction, retroViewDir, geometryNormal, material );
		specularBRDF = mix( specularBRDF, retroSpecularBRDF, saturate( material.retroreflectivity ) );
	#endif
	reflectedLight.directSpecular += irradiance * specularBRDF * material.multiScatteringCompensation;
	vec3 halfDir = normalize( directLight.direction + geometryViewDir );
	float dotVH = saturate( dot( geometryViewDir, halfDir ) );
	vec3 F = F_Schlick( material.specularColor, material.specularF90, dotVH );
	#ifdef USE_RETROREFLECTION
		vec3 retroHalfDir = normalize( directLight.direction + retroViewDir );
		float dotRetroVH = saturate( dot( retroViewDir, retroHalfDir ) );
		vec3 retroF = F_Schlick( material.specularColor, material.specularF90, dotRetroVH );
		F = mix( F, retroF, saturate( material.retroreflectivity ) );
	#endif
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseContribution ) * ( 1.0 - F );
}
void RE_IndirectDiffuse_Physical( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight ) {
	vec3 singleScattering = vec3( 0.0 );
	vec3 multiScattering = vec3( 0.0 );
	#ifdef USE_IRIDESCENCE
		computeMultiscatteringIridescence( material.dfg, material.specularColor, material.specularF90, material.iridescence, material.iridescenceF0Dielectric, singleScattering, multiScattering );
	#else
		computeMultiscattering( material.dfg, material.specularColor, material.specularF90, singleScattering, multiScattering );
	#endif
	vec3 diffuse = irradiance * BRDF_Lambert( material.diffuseContribution ) * ( 1.0 - singleScattering - multiScattering );
	#ifdef USE_SHEEN
		float sheenAlbedo = IBLSheenBRDF( geometryNormal, geometryViewDir, material.sheenRoughness );
		sheenSpecularIndirect += irradiance * material.sheenColor * sheenAlbedo * RECIPROCAL_PI;
		float sheenEnergyComp = 1.0 - max3( material.sheenColor ) * sheenAlbedo;
		diffuse *= sheenEnergyComp;
	#endif
	reflectedLight.indirectDiffuse += diffuse;
}
void RE_IndirectSpecular_Physical( const in vec3 radiance, const in vec3 irradiance, const in vec3 clearcoatRadiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight) {
	#ifdef USE_CLEARCOAT
		clearcoatSpecularIndirect += clearcoatRadiance * EnvironmentBRDF( geometryClearcoatNormal, geometryViewDir, material.clearcoatF0, material.clearcoatF90, material.clearcoatRoughness );
	#endif
	#ifdef USE_SHEEN
		sheenSpecularIndirect += irradiance * material.sheenColor * IBLSheenBRDF( geometryNormal, geometryViewDir, material.sheenRoughness ) * RECIPROCAL_PI;
 	#endif
	vec3 singleScatteringDielectric = vec3( 0.0 );
	vec3 multiScatteringDielectric = vec3( 0.0 );
	vec3 singleScatteringMetallic = vec3( 0.0 );
	vec3 multiScatteringMetallic = vec3( 0.0 );
	#ifdef USE_IRIDESCENCE
		computeMultiscatteringIridescence( material.dfg, material.specularColor, material.specularF90, material.iridescence, material.iridescenceF0Dielectric, singleScatteringDielectric, multiScatteringDielectric );
		computeMultiscatteringIridescence( material.dfg, material.diffuseColor, material.specularF90, material.iridescence, material.iridescenceF0Metallic, singleScatteringMetallic, multiScatteringMetallic );
	#else
		computeMultiscattering( material.dfg, material.specularColor, material.specularF90, singleScatteringDielectric, multiScatteringDielectric );
		computeMultiscattering( material.dfg, material.diffuseColor, material.specularF90, singleScatteringMetallic, multiScatteringMetallic );
	#endif
	vec3 singleScattering = mix( singleScatteringDielectric, singleScatteringMetallic, material.metalness );
	vec3 multiScattering = mix( multiScatteringDielectric, multiScatteringMetallic, material.metalness );
	vec3 totalScatteringDielectric = singleScatteringDielectric + multiScatteringDielectric;
	vec3 diffuse = material.diffuseContribution * ( 1.0 - totalScatteringDielectric );
	vec3 cosineWeightedIrradiance = irradiance * RECIPROCAL_PI;
	vec3 indirectSpecular = radiance * singleScattering;
	indirectSpecular += multiScattering * cosineWeightedIrradiance;
	vec3 indirectDiffuse = diffuse * cosineWeightedIrradiance;
	#ifdef USE_SHEEN
		float sheenAlbedo = IBLSheenBRDF( geometryNormal, geometryViewDir, material.sheenRoughness );
		float sheenEnergyComp = 1.0 - max3( material.sheenColor ) * sheenAlbedo;
		indirectSpecular *= sheenEnergyComp;
		indirectDiffuse *= sheenEnergyComp;
	#endif
	reflectedLight.indirectSpecular += indirectSpecular;
	reflectedLight.indirectDiffuse += indirectDiffuse;
}
#define RE_Direct				RE_Direct_Physical
#define RE_Direct_RectArea		RE_Direct_RectArea_Physical
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Physical
#define RE_IndirectSpecular		RE_IndirectSpecular_Physical
float computeSpecularOcclusion( const in float dotNV, const in float ambientOcclusion, const in float roughness ) {
	return saturate( pow( dotNV + ambientOcclusion, exp2( - 16.0 * roughness - 1.0 ) ) - 1.0 + ambientOcclusion );
}`,ox=`
vec3 geometryPosition = - vViewPosition;
vec3 geometryNormal = normal;
vec3 geometryViewDir = ( isOrthographic ) ? vec3( 0, 0, 1 ) : normalize( vViewPosition );
vec3 geometryClearcoatNormal = vec3( 0.0 );
#ifdef USE_CLEARCOAT
	geometryClearcoatNormal = clearcoatNormal;
#endif
#ifdef USE_IRIDESCENCE
	float dotNVi = saturate( dot( normal, geometryViewDir ) );
	if ( material.iridescenceThickness == 0.0 ) {
		material.iridescence = 0.0;
	} else {
		material.iridescence = saturate( material.iridescence );
	}
	if ( material.iridescence > 0.0 ) {
		vec3 iridescenceFresnelDielectric = evalIridescence( 1.0, material.iridescenceIOR, dotNVi, material.iridescenceThickness, material.specularColor );
		vec3 iridescenceFresnelMetallic = evalIridescence( 1.0, material.iridescenceIOR, dotNVi, material.iridescenceThickness, material.diffuseColor );
		material.iridescenceFresnel = mix( iridescenceFresnelDielectric, iridescenceFresnelMetallic, material.metalness );
		material.iridescenceF0Dielectric = Schlick_to_F0( iridescenceFresnelDielectric, 1.0, dotNVi );
		material.iridescenceF0Metallic = Schlick_to_F0( iridescenceFresnelMetallic, 1.0, dotNVi );
	}
#endif
#ifdef STANDARD
	float dotNVms = saturate( dot( geometryNormal, geometryViewDir ) );
	material.dfg = texture2D( dfgLUT, vec2( material.roughness, dotNVms ) ).rg;
	#if ( NUM_SUN_LIGHTS > 0 || NUM_DIR_LIGHTS > 0 || NUM_POINT_LIGHTS > 0 || NUM_SPOT_LIGHTS > 0 )
		float EssMs = material.dfg.x + material.dfg.y;
		material.multiScatteringCompensation = 1.0 + material.specularColorBlended * ( 1.0 / EssMs - 1.0 );
	#endif
#endif
IncidentLight directLight;
#if ( NUM_POINT_LIGHTS > 0 ) && defined( RE_Direct )
	PointLight pointLight;
	#if defined( USE_SHADOWMAP ) && NUM_POINT_LIGHT_SHADOWS > 0
	PointLightShadow pointLightShadow;
	#endif
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_POINT_LIGHTS; i ++ ) {
		pointLight = pointLights[ i ];
		getPointLightInfo( pointLight, geometryPosition, directLight );
		#if defined( USE_SHADOWMAP ) && ( UNROLLED_LOOP_INDEX < NUM_POINT_LIGHT_SHADOWS ) && ( defined( SHADOWMAP_TYPE_PCF ) || defined( SHADOWMAP_TYPE_BASIC ) )
		pointLightShadow = pointLightShadows[ i ];
		directLight.color *= ( directLight.visible && receiveShadow ) ? getPointShadow( pointShadowMap[ i ], pointLightShadow.shadowMapSize, pointLightShadow.shadowIntensity, pointLightShadow.shadowBias, pointLightShadow.shadowRadius, vPointShadowCoord[ i ], pointLightShadow.shadowCameraNear, pointLightShadow.shadowCameraFar ) : 1.0;
		#endif
		RE_Direct( directLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if ( NUM_SPOT_LIGHTS > 0 ) && defined( RE_Direct )
	SpotLight spotLight;
	vec4 spotColor;
	vec3 spotLightCoord;
	bool inSpotLightMap;
	#if defined( USE_SHADOWMAP ) && NUM_SPOT_LIGHT_SHADOWS > 0
	SpotLightShadow spotLightShadow;
	#endif
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SPOT_LIGHTS; i ++ ) {
		spotLight = spotLights[ i ];
		getSpotLightInfo( spotLight, geometryPosition, directLight );
		#if ( UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS )
		#define SPOT_LIGHT_MAP_INDEX UNROLLED_LOOP_INDEX
		#elif ( UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS )
		#define SPOT_LIGHT_MAP_INDEX NUM_SPOT_LIGHT_MAPS
		#else
		#define SPOT_LIGHT_MAP_INDEX ( UNROLLED_LOOP_INDEX - NUM_SPOT_LIGHT_SHADOWS + NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS )
		#endif
		#if ( SPOT_LIGHT_MAP_INDEX < NUM_SPOT_LIGHT_MAPS )
			spotLightCoord = vSpotLightCoord[ i ].xyz / vSpotLightCoord[ i ].w;
			inSpotLightMap = all( lessThan( abs( spotLightCoord * 2. - 1. ), vec3( 1.0 ) ) );
			spotColor = texture2D( spotLightMap[ SPOT_LIGHT_MAP_INDEX ], spotLightCoord.xy );
			directLight.color = inSpotLightMap ? directLight.color * spotColor.rgb : directLight.color;
		#endif
		#undef SPOT_LIGHT_MAP_INDEX
		#if defined( USE_SHADOWMAP ) && ( UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS )
		spotLightShadow = spotLightShadows[ i ];
		directLight.color *= ( directLight.visible && receiveShadow ) ? getShadow( spotShadowMap[ i ], spotLightShadow.shadowMapSize, spotLightShadow.shadowIntensity, spotLightShadow.shadowBias, spotLightShadow.shadowRadius, vSpotLightCoord[ i ] ) : 1.0;
		#endif
		RE_Direct( directLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if ( NUM_SUN_LIGHTS > 0 ) && defined( RE_Direct )
	SunLight sunLight;
	#if defined( USE_SHADOWMAP ) && NUM_SUN_LIGHT_SHADOWS > 0
	SunLightShadow sunLightShadow;
	#endif
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SUN_LIGHTS; i ++ ) {
		sunLight = sunLights[ i ];
		getSunLightInfo( sunLight, directLight );
		#if defined( USE_SHADOWMAP ) && ( UNROLLED_LOOP_INDEX < NUM_SUN_LIGHT_SHADOWS )
		sunLightShadow = sunLightShadows[ i ];
		directLight.color *= ( directLight.visible && receiveShadow ) ? getSunShadow( sunShadowMap[ i ], sunLightShadow, UNROLLED_LOOP_INDEX ) : 1.0;
		#endif
		RE_Direct( directLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if ( NUM_DIR_LIGHTS > 0 ) && defined( RE_Direct )
	DirectionalLight directionalLight;
	#if defined( USE_SHADOWMAP ) && NUM_DIR_LIGHT_SHADOWS > 0
	DirectionalLightShadow directionalLightShadow;
	#endif
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_DIR_LIGHTS; i ++ ) {
		directionalLight = directionalLights[ i ];
		getDirectionalLightInfo( directionalLight, directLight );
		#if defined( USE_SHADOWMAP ) && ( UNROLLED_LOOP_INDEX < NUM_DIR_LIGHT_SHADOWS )
		directionalLightShadow = directionalLightShadows[ i ];
		directLight.color *= ( directLight.visible && receiveShadow ) ? getShadow( directionalShadowMap[ i ], directionalLightShadow.shadowMapSize, directionalLightShadow.shadowIntensity, directionalLightShadow.shadowBias, directionalLightShadow.shadowRadius, vDirectionalShadowCoord[ i ] ) : 1.0;
		#endif
		RE_Direct( directLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if ( NUM_RECT_AREA_LIGHTS > 0 ) && defined( RE_Direct_RectArea )
	RectAreaLight rectAreaLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_RECT_AREA_LIGHTS; i ++ ) {
		rectAreaLight = rectAreaLights[ i ];
		RE_Direct_RectArea( rectAreaLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if defined( RE_IndirectDiffuse )
	vec3 iblIrradiance = vec3( 0.0 );
	vec3 irradiance = getAmbientLightIrradiance( ambientLightColor );
	#if defined( USE_LIGHT_PROBES )
		irradiance += getLightProbeIrradiance( lightProbe, geometryNormal );
	#endif
	#if ( NUM_HEMI_LIGHTS > 0 )
		#pragma unroll_loop_start
		for ( int i = 0; i < NUM_HEMI_LIGHTS; i ++ ) {
			irradiance += getHemisphereLightIrradiance( hemisphereLights[ i ], geometryNormal );
		}
		#pragma unroll_loop_end
	#endif
	#ifdef USE_LIGHT_PROBES_GRID
		vec3 probeWorldPos = ( ( vec4( geometryPosition, 1.0 ) - viewMatrix[ 3 ] ) * viewMatrix ).xyz;
		vec3 probeWorldNormal = transformNormalByInverseViewMatrix( geometryNormal, viewMatrix );
		irradiance += getLightProbeGridIrradiance( probeWorldPos, probeWorldNormal );
	#endif
#endif
#if defined( RE_IndirectSpecular )
	vec3 radiance = vec3( 0.0 );
	vec3 clearcoatRadiance = vec3( 0.0 );
#endif`,lx=`#if defined( RE_IndirectDiffuse )
	#ifdef USE_LIGHTMAP
		vec4 lightMapTexel = texture2D( lightMap, vLightMapUv );
		vec3 lightMapIrradiance = lightMapTexel.rgb * lightMapIntensity;
		irradiance += lightMapIrradiance;
	#endif
	#if defined( USE_ENVMAP ) && defined( ENVMAP_TYPE_CUBE_UV )
		#if defined( STANDARD ) || defined( LAMBERT ) || defined( PHONG )
			iblIrradiance += getIBLIrradiance( geometryNormal );
		#endif
	#endif
#endif
#if defined( USE_ENVMAP ) && defined( RE_IndirectSpecular )
	#ifdef USE_ANISOTROPY
		vec3 iblRadiance = getIBLAnisotropyRadiance( geometryViewDir, geometryNormal, material.roughness, material.anisotropyB, material.anisotropy );
	#else
		vec3 iblRadiance = getIBLRadiance( geometryViewDir, geometryNormal, material.roughness );
	#endif
	#ifdef USE_RETROREFLECTION
		#ifdef USE_ANISOTROPY
			vec3 retroIBLRadiance = getIBLAnisotropyRetroRadiance( geometryViewDir, geometryNormal, material.roughness, material.anisotropyB, material.anisotropy );
		#else
			vec3 retroIBLRadiance = getIBLRetroRadiance( geometryViewDir, geometryNormal, material.roughness );
		#endif
		iblRadiance = mix( iblRadiance, retroIBLRadiance, saturate( material.retroreflectivity ) );
	#endif
	radiance += iblRadiance;
	#ifdef USE_CLEARCOAT
		clearcoatRadiance += getIBLRadiance( geometryViewDir, geometryClearcoatNormal, material.clearcoatRoughness );
	#endif
#endif`,cx=`#if defined( RE_IndirectDiffuse )
	#if defined( LAMBERT ) || defined( PHONG )
		irradiance += iblIrradiance;
	#endif
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,hx=`#ifdef USE_LIGHT_PROBES_GRID
uniform highp sampler3D probesSH;
uniform vec3 probesMin;
uniform vec3 probesMax;
uniform vec3 probesResolution;
vec3 getLightProbeGridIrradiance( vec3 worldPos, vec3 worldNormal ) {
	vec3 res = probesResolution;
	vec3 gridRange = probesMax - probesMin;
	vec3 resMinusOne = res - 1.0;
	vec3 probeSpacing = gridRange / resMinusOne;
	vec3 samplePos = worldPos + worldNormal * probeSpacing * 0.5;
	vec3 uvw = clamp( ( samplePos - probesMin ) / gridRange, 0.0, 1.0 );
	uvw = uvw * resMinusOne / res + 0.5 / res;
	float nz          = res.z;
	float paddedSlices = nz + 2.0;
	float atlasDepth  = 7.0 * paddedSlices;
	float uvZBase     = uvw.z * nz + 1.0;
	vec4 s0 = texture( probesSH, vec3( uvw.xy, ( uvZBase                       ) / atlasDepth ) );
	vec4 s1 = texture( probesSH, vec3( uvw.xy, ( uvZBase +       paddedSlices   ) / atlasDepth ) );
	vec4 s2 = texture( probesSH, vec3( uvw.xy, ( uvZBase + 2.0 * paddedSlices   ) / atlasDepth ) );
	vec4 s3 = texture( probesSH, vec3( uvw.xy, ( uvZBase + 3.0 * paddedSlices   ) / atlasDepth ) );
	vec4 s4 = texture( probesSH, vec3( uvw.xy, ( uvZBase + 4.0 * paddedSlices   ) / atlasDepth ) );
	vec4 s5 = texture( probesSH, vec3( uvw.xy, ( uvZBase + 5.0 * paddedSlices   ) / atlasDepth ) );
	vec4 s6 = texture( probesSH, vec3( uvw.xy, ( uvZBase + 6.0 * paddedSlices   ) / atlasDepth ) );
	vec3 c0 = s0.xyz;
	vec3 c1 = vec3( s0.w, s1.xy );
	vec3 c2 = vec3( s1.zw, s2.x );
	vec3 c3 = s2.yzw;
	vec3 c4 = s3.xyz;
	vec3 c5 = vec3( s3.w, s4.xy );
	vec3 c6 = vec3( s4.zw, s5.x );
	vec3 c7 = s5.yzw;
	vec3 c8 = s6.xyz;
	float x = worldNormal.x, y = worldNormal.y, z = worldNormal.z;
	vec3 result = c0 * 0.886227;
	result += c1 * 2.0 * 0.511664 * y;
	result += c2 * 2.0 * 0.511664 * z;
	result += c3 * 2.0 * 0.511664 * x;
	result += c4 * 2.0 * 0.429043 * x * y;
	result += c5 * 2.0 * 0.429043 * y * z;
	result += c6 * ( 0.743125 * z * z - 0.247708 );
	result += c7 * 2.0 * 0.429043 * x * z;
	result += c8 * 0.429043 * ( x * x - y * y );
	return max( result, vec3( 0.0 ) );
}
#endif`,ux=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,dx=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,fx=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,px=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,mx=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = sRGBTransferEOTF( sampledDiffuseColor );
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,gx=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,xx=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
	#if defined( USE_POINTS_UV )
		vec2 uv = vUv;
	#else
		vec2 uv = ( uvTransform * vec3( gl_PointCoord.x, 1.0 - gl_PointCoord.y, 1 ) ).xy;
	#endif
#endif
#ifdef USE_MAP
	diffuseColor *= texture2D( map, uv );
#endif
#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, uv ).g;
#endif`,bx=`#if defined( USE_POINTS_UV )
	varying vec2 vUv;
#else
	#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
		uniform mat3 uvTransform;
	#endif
#endif
#ifdef USE_MAP
	uniform sampler2D map;
#endif
#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,vx=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,_x=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,yx=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,Mx=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,wx=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,Sx=`#ifdef USE_MORPHTARGETS
	#ifndef USE_INSTANCING_MORPH
		uniform float morphTargetBaseInfluence;
		uniform float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	#endif
	uniform sampler2DArray morphTargetsTexture;
	uniform ivec2 morphTargetsTextureSize;
	vec4 getMorph( const in int vertexIndex, const in int morphTargetIndex, const in int offset ) {
		int texelIndex = vertexIndex * MORPHTARGETS_TEXTURE_STRIDE + offset;
		int y = texelIndex / morphTargetsTextureSize.x;
		int x = texelIndex - y * morphTargetsTextureSize.x;
		ivec3 morphUV = ivec3( x, y, morphTargetIndex );
		return texelFetch( morphTargetsTexture, morphUV, 0 );
	}
#endif`,Ex=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,Tx=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
#ifdef FLAT_SHADED
	vec3 fdx = dFdx( vViewPosition );
	vec3 fdy = dFdy( vViewPosition );
	vec3 normal = normalize( cross( fdx, fdy ) );
#else
	vec3 normal = normalize( vNormal );
	#ifdef DOUBLE_SIDED
		normal *= faceDirection;
	#endif
#endif
#if defined( USE_NORMALMAP_TANGENTSPACE ) || defined( USE_CLEARCOAT_NORMALMAP ) || defined( USE_ANISOTROPY )
	#ifdef USE_TANGENT
		mat3 tbn = mat3( normalize( vTangent ), normalize( vBitangent ), normal );
	#else
		mat3 tbn = getTangentFrame( - vViewPosition, normal,
		#if defined( USE_NORMALMAP )
			vNormalMapUv
		#elif defined( USE_CLEARCOAT_NORMALMAP )
			vClearcoatNormalMapUv
		#else
			vUv
		#endif
		);
	#endif
	#ifdef DOUBLE_SIDED
		tbn[0] *= faceDirection;
		tbn[1] *= faceDirection;
	#endif
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	#ifdef USE_TANGENT
		mat3 tbn2 = mat3( normalize( vTangent ), normalize( vBitangent ), normal );
	#else
		mat3 tbn2 = getTangentFrame( - vViewPosition, normal, vClearcoatNormalMapUv );
	#endif
	#ifdef DOUBLE_SIDED
		tbn2[0] *= faceDirection;
		tbn2[1] *= faceDirection;
	#endif
#endif
vec3 nonPerturbedNormal = normal;`,Ax=`#ifdef USE_NORMALMAP_OBJECTSPACE
	normal = texture2D( normalMap, vNormalMapUv ).xyz * 2.0 - 1.0;
	#ifdef FLIP_SIDED
		normal = - normal;
	#endif
	#ifdef DOUBLE_SIDED
		normal = normal * faceDirection;
	#endif
	normal = normalize( normalMatrix * normal );
#elif defined( USE_NORMALMAP_TANGENTSPACE )
	vec3 mapN = texture2D( normalMap, vNormalMapUv ).xyz * 2.0 - 1.0;
	#if defined( USE_PACKED_NORMALMAP )
		mapN = vec3( mapN.xy, sqrt( saturate( 1.0 - dot( mapN.xy, mapN.xy ) ) ) );
	#endif
	mapN.xy *= normalScale;
	normal = normalize( tbn * mapN );
#elif defined( USE_BUMPMAP )
	normal = perturbNormalArb( - vViewPosition, normal, dHdxy_fwd(), faceDirection );
#endif`,Rx=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,Cx=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,Px=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
		#ifdef FLIP_SIDED
			vBitangent = - vBitangent;
		#endif
	#endif
#endif`,Ix=`#ifdef USE_NORMALMAP
	uniform sampler2D normalMap;
	uniform vec2 normalScale;
#endif
#ifdef USE_NORMALMAP_OBJECTSPACE
	uniform mat3 normalMatrix;
#endif
#if ! defined ( USE_TANGENT ) && ( defined ( USE_NORMALMAP_TANGENTSPACE ) || defined ( USE_CLEARCOAT_NORMALMAP ) || defined( USE_ANISOTROPY ) )
	mat3 getTangentFrame( vec3 eye_pos, vec3 surf_norm, vec2 uv ) {
		vec3 q0 = dFdx( eye_pos.xyz );
		vec3 q1 = dFdy( eye_pos.xyz );
		vec2 st0 = dFdx( uv.st );
		vec2 st1 = dFdy( uv.st );
		vec3 N = surf_norm;
		vec3 q1perp = cross( q1, N );
		vec3 q0perp = cross( N, q0 );
		vec3 T = q1perp * st0.x + q0perp * st1.x;
		vec3 B = q1perp * st0.y + q0perp * st1.y;
		float det = max( dot( T, T ), dot( B, B ) );
		float scale = ( det == 0.0 ) ? 0.0 : inversesqrt( det );
		return mat3( T * scale, B * scale, N );
	}
#endif`,Dx=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,Lx=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,Fx=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,Nx=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,Ux=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,Ox=`vec3 packNormalToRGB( const in vec3 normal ) {
	return normalize( normal ) * 0.5 + 0.5;
}
vec3 unpackRGBToNormal( const in vec3 rgb ) {
	return 2.0 * rgb.xyz - 1.0;
}
const float PackUpscale = 256. / 255.;const float UnpackDownscale = 255. / 256.;const float ShiftRight8 = 1. / 256.;
const float Inv255 = 1. / 255.;
const vec4 PackFactors = vec4( 1.0, 256.0, 256.0 * 256.0, 256.0 * 256.0 * 256.0 );
const vec2 UnpackFactors2 = vec2( UnpackDownscale, 1.0 / PackFactors.g );
const vec3 UnpackFactors3 = vec3( UnpackDownscale / PackFactors.rg, 1.0 / PackFactors.b );
const vec4 UnpackFactors4 = vec4( UnpackDownscale / PackFactors.rgb, 1.0 / PackFactors.a );
vec4 packDepthToRGBA( const in float v ) {
	if( v <= 0.0 )
		return vec4( 0., 0., 0., 0. );
	if( v >= 1.0 )
		return vec4( 1., 1., 1., 1. );
	float vuf;
	float af = modf( v * PackFactors.a, vuf );
	float bf = modf( vuf * ShiftRight8, vuf );
	float gf = modf( vuf * ShiftRight8, vuf );
	return vec4( vuf * Inv255, gf * PackUpscale, bf * PackUpscale, af );
}
vec3 packDepthToRGB( const in float v ) {
	if( v <= 0.0 )
		return vec3( 0., 0., 0. );
	if( v >= 1.0 )
		return vec3( 1., 1., 1. );
	float vuf;
	float bf = modf( v * PackFactors.b, vuf );
	float gf = modf( vuf * ShiftRight8, vuf );
	return vec3( vuf * Inv255, gf * PackUpscale, bf );
}
vec2 packDepthToRG( const in float v ) {
	if( v <= 0.0 )
		return vec2( 0., 0. );
	if( v >= 1.0 )
		return vec2( 1., 1. );
	float vuf;
	float gf = modf( v * 256., vuf );
	return vec2( vuf * Inv255, gf );
}
float unpackRGBAToDepth( const in vec4 v ) {
	return dot( v, UnpackFactors4 );
}
float unpackRGBToDepth( const in vec3 v ) {
	return dot( v, UnpackFactors3 );
}
float unpackRGToDepth( const in vec2 v ) {
	return v.r * UnpackFactors2.r + v.g * UnpackFactors2.g;
}
vec4 pack2HalfToRGBA( const in vec2 v ) {
	vec4 r = vec4( v.x, fract( v.x * 255.0 ), v.y, fract( v.y * 255.0 ) );
	return vec4( r.x - r.y / 255.0, r.y, r.z - r.w / 255.0, r.w );
}
vec2 unpackRGBATo2Half( const in vec4 v ) {
	return vec2( v.x + ( v.y / 255.0 ), v.z + ( v.w / 255.0 ) );
}
float viewZToOrthographicDepth( const in float viewZ, const in float near, const in float far ) {
	return ( viewZ + near ) / ( near - far );
}
float orthographicDepthToViewZ( const in float depth, const in float near, const in float far ) {
	#ifdef USE_REVERSED_DEPTH_BUFFER
	
		return depth * ( far - near ) - far;
	#else
		return depth * ( near - far ) - near;
	#endif
}
float viewZToPerspectiveDepth( const in float viewZ, const in float near, const in float far ) {
	return ( ( near + viewZ ) * far ) / ( ( far - near ) * viewZ );
}
float perspectiveDepthToViewZ( const in float depth, const in float near, const in float far ) {
	
	#ifdef USE_REVERSED_DEPTH_BUFFER
		return ( near * far ) / ( ( near - far ) * depth - near );
	#else
		return ( near * far ) / ( ( far - near ) * depth - far );
	#endif
}`,zx=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,kx=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,Bx=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,Hx=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,Gx=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,Vx=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,Wx=`#if NUM_SPOT_LIGHT_COORDS > 0
	varying vec4 vSpotLightCoord[ NUM_SPOT_LIGHT_COORDS ];
#endif
#if NUM_SPOT_LIGHT_MAPS > 0
	uniform sampler2D spotLightMap[ NUM_SPOT_LIGHT_MAPS ];
#endif
#ifdef USE_SHADOWMAP
	#if NUM_SUN_LIGHT_SHADOWS > 0
		#define SUN_LIGHT_CASCADES 2
		#if defined( SHADOWMAP_TYPE_PCF )
			uniform sampler2DShadow sunShadowMap[ NUM_SUN_LIGHT_SHADOWS ];
		#else
			uniform sampler2D sunShadowMap[ NUM_SUN_LIGHT_SHADOWS ];
		#endif
		uniform mat4 sunShadowMatrix[ NUM_SUN_LIGHT_SHADOWS * SUN_LIGHT_CASCADES ];
		uniform vec4 sunShadowCascade[ NUM_SUN_LIGHT_SHADOWS * SUN_LIGHT_CASCADES ];
		varying vec4 vSunShadowWorldPosition;
		varying vec3 vSunShadowWorldNormal;
		struct SunLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform SunLightShadow sunLightShadows[ NUM_SUN_LIGHT_SHADOWS ];
	#endif
	#if NUM_DIR_LIGHT_SHADOWS > 0
		#if defined( SHADOWMAP_TYPE_PCF )
			uniform sampler2DShadow directionalShadowMap[ NUM_DIR_LIGHT_SHADOWS ];
		#else
			uniform sampler2D directionalShadowMap[ NUM_DIR_LIGHT_SHADOWS ];
		#endif
		varying vec4 vDirectionalShadowCoord[ NUM_DIR_LIGHT_SHADOWS ];
		struct DirectionalLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform DirectionalLightShadow directionalLightShadows[ NUM_DIR_LIGHT_SHADOWS ];
	#endif
	#if NUM_SPOT_LIGHT_SHADOWS > 0
		#if defined( SHADOWMAP_TYPE_PCF )
			uniform sampler2DShadow spotShadowMap[ NUM_SPOT_LIGHT_SHADOWS ];
		#else
			uniform sampler2D spotShadowMap[ NUM_SPOT_LIGHT_SHADOWS ];
		#endif
		struct SpotLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform SpotLightShadow spotLightShadows[ NUM_SPOT_LIGHT_SHADOWS ];
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
		#if defined( SHADOWMAP_TYPE_PCF )
			uniform samplerCubeShadow pointShadowMap[ NUM_POINT_LIGHT_SHADOWS ];
		#elif defined( SHADOWMAP_TYPE_BASIC )
			uniform samplerCube pointShadowMap[ NUM_POINT_LIGHT_SHADOWS ];
		#endif
		varying vec4 vPointShadowCoord[ NUM_POINT_LIGHT_SHADOWS ];
		struct PointLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
			float shadowCameraNear;
			float shadowCameraFar;
		};
		uniform PointLightShadow pointLightShadows[ NUM_POINT_LIGHT_SHADOWS ];
	#endif
	#if defined( SHADOWMAP_TYPE_PCF )
		float interleavedGradientNoise( vec2 position ) {
			return fract( 52.9829189 * fract( dot( position, vec2( 0.06711056, 0.00583715 ) ) ) );
		}
		vec2 vogelDiskSample( int sampleIndex, int samplesCount, float phi ) {
			const float goldenAngle = 2.399963229728653;
			float r = sqrt( ( float( sampleIndex ) + 0.5 ) / float( samplesCount ) );
			float theta = float( sampleIndex ) * goldenAngle + phi;
			return vec2( cos( theta ), sin( theta ) ) * r;
		}
	#endif
	#if defined( SHADOWMAP_TYPE_PCF )
		float getShadow( sampler2DShadow shadowMap, vec2 shadowMapSize, float shadowIntensity, float shadowBias, float shadowRadius, vec4 shadowCoord ) {
			float shadow = 1.0;
			shadowCoord.xyz /= shadowCoord.w;
			shadowCoord.z += shadowBias;
			bool inFrustum = shadowCoord.x >= 0.0 && shadowCoord.x <= 1.0 && shadowCoord.y >= 0.0 && shadowCoord.y <= 1.0;
			bool frustumTest = inFrustum && shadowCoord.z <= 1.0;
			if ( frustumTest ) {
				vec2 texelSize = vec2( 1.0 ) / shadowMapSize;
				float radius = shadowRadius * texelSize.x;
				float phi = interleavedGradientNoise( gl_FragCoord.xy ) * PI2;
				shadow = (
					texture( shadowMap, vec3( shadowCoord.xy + vogelDiskSample( 0, 5, phi ) * radius, shadowCoord.z ) ) +
					texture( shadowMap, vec3( shadowCoord.xy + vogelDiskSample( 1, 5, phi ) * radius, shadowCoord.z ) ) +
					texture( shadowMap, vec3( shadowCoord.xy + vogelDiskSample( 2, 5, phi ) * radius, shadowCoord.z ) ) +
					texture( shadowMap, vec3( shadowCoord.xy + vogelDiskSample( 3, 5, phi ) * radius, shadowCoord.z ) ) +
					texture( shadowMap, vec3( shadowCoord.xy + vogelDiskSample( 4, 5, phi ) * radius, shadowCoord.z ) )
				) * 0.2;
			}
			return mix( 1.0, shadow, shadowIntensity );
		}
	#elif defined( SHADOWMAP_TYPE_VSM )
		float getShadow( sampler2D shadowMap, vec2 shadowMapSize, float shadowIntensity, float shadowBias, float shadowRadius, vec4 shadowCoord ) {
			float shadow = 1.0;
			shadowCoord.xyz /= shadowCoord.w;
			#ifdef USE_REVERSED_DEPTH_BUFFER
				shadowCoord.z -= shadowBias;
			#else
				shadowCoord.z += shadowBias;
			#endif
			bool inFrustum = shadowCoord.x >= 0.0 && shadowCoord.x <= 1.0 && shadowCoord.y >= 0.0 && shadowCoord.y <= 1.0;
			bool frustumTest = inFrustum && shadowCoord.z <= 1.0;
			if ( frustumTest ) {
				vec2 distribution = texture2D( shadowMap, shadowCoord.xy ).rg;
				float mean = distribution.x;
				float variance = distribution.y * distribution.y;
				#ifdef USE_REVERSED_DEPTH_BUFFER
					float hard_shadow = step( mean, shadowCoord.z );
				#else
					float hard_shadow = step( shadowCoord.z, mean );
				#endif
				
				if ( hard_shadow == 1.0 ) {
					shadow = 1.0;
				} else {
					variance = max( variance, 0.0000001 );
					float d = shadowCoord.z - mean;
					float p_max = variance / ( variance + d * d );
					p_max = clamp( ( p_max - 0.3 ) / 0.65, 0.0, 1.0 );
					shadow = max( hard_shadow, p_max );
				}
			}
			return mix( 1.0, shadow, shadowIntensity );
		}
	#else
		float getShadow( sampler2D shadowMap, vec2 shadowMapSize, float shadowIntensity, float shadowBias, float shadowRadius, vec4 shadowCoord ) {
			float shadow = 1.0;
			shadowCoord.xyz /= shadowCoord.w;
			#ifdef USE_REVERSED_DEPTH_BUFFER
				shadowCoord.z -= shadowBias;
			#else
				shadowCoord.z += shadowBias;
			#endif
			bool inFrustum = shadowCoord.x >= 0.0 && shadowCoord.x <= 1.0 && shadowCoord.y >= 0.0 && shadowCoord.y <= 1.0;
			bool frustumTest = inFrustum && shadowCoord.z <= 1.0;
			if ( frustumTest ) {
				float depth = texture2D( shadowMap, shadowCoord.xy ).r;
				#ifdef USE_REVERSED_DEPTH_BUFFER
					shadow = step( depth, shadowCoord.z );
				#else
					shadow = step( shadowCoord.z, depth );
				#endif
			}
			return mix( 1.0, shadow, shadowIntensity );
		}
	#endif
	#if NUM_SUN_LIGHT_SHADOWS > 0
		float getSunShadow(
			#if defined( SHADOWMAP_TYPE_PCF )
				sampler2DShadow shadowMap,
			#else
				sampler2D shadowMap,
			#endif
			SunLightShadow sunLightShadow,
			int shadowIndex
		) {
			vec4 shadowWorldPosition = vec4( vSunShadowWorldPosition.xyz + vSunShadowWorldNormal * sunLightShadow.shadowNormalBias, 1.0 );
			float viewDepth = vSunShadowWorldPosition.w;
			int cascadeOffset = shadowIndex * SUN_LIGHT_CASCADES;
			float shadow = 1.0;
			for ( int i = SUN_LIGHT_CASCADES - 1; i >= 0; i -- ) {
				vec4 cascade = sunShadowCascade[ cascadeOffset + i ];
				if ( viewDepth >= cascade.x && viewDepth < cascade.y ) {
					float cascadeShadow = getShadow(
						shadowMap,
						sunLightShadow.shadowMapSize,
						sunLightShadow.shadowIntensity,
						sunLightShadow.shadowBias,
						sunLightShadow.shadowRadius,
						sunShadowMatrix[ cascadeOffset + i ] * shadowWorldPosition
					);
					shadow = mix( cascadeShadow, shadow, smoothstep( cascade.z, cascade.y, viewDepth ) );
				}
			}
			return shadow;
		}
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
	#if defined( SHADOWMAP_TYPE_PCF )
	float getPointShadow( samplerCubeShadow shadowMap, vec2 shadowMapSize, float shadowIntensity, float shadowBias, float shadowRadius, vec4 shadowCoord, float shadowCameraNear, float shadowCameraFar ) {
		float shadow = 1.0;
		vec3 lightToPosition = shadowCoord.xyz;
		vec3 bd3D = normalize( lightToPosition );
		vec3 absVec = abs( lightToPosition );
		float viewSpaceZ = max( max( absVec.x, absVec.y ), absVec.z );
		if ( viewSpaceZ - shadowCameraFar <= 0.0 && viewSpaceZ - shadowCameraNear >= 0.0 ) {
			#ifdef USE_REVERSED_DEPTH_BUFFER
				float dp = ( shadowCameraNear * ( shadowCameraFar - viewSpaceZ ) ) / ( viewSpaceZ * ( shadowCameraFar - shadowCameraNear ) );
				dp -= shadowBias;
			#else
				float dp = ( shadowCameraFar * ( viewSpaceZ - shadowCameraNear ) ) / ( viewSpaceZ * ( shadowCameraFar - shadowCameraNear ) );
				dp += shadowBias;
			#endif
			float texelSize = shadowRadius / shadowMapSize.x;
			vec3 absDir = abs( bd3D );
			vec3 tangent = absDir.x > absDir.z ? vec3( 0.0, 1.0, 0.0 ) : vec3( 1.0, 0.0, 0.0 );
			tangent = normalize( cross( bd3D, tangent ) );
			vec3 bitangent = cross( bd3D, tangent );
			float phi = interleavedGradientNoise( gl_FragCoord.xy ) * PI2;
			vec2 sample0 = vogelDiskSample( 0, 5, phi );
			vec2 sample1 = vogelDiskSample( 1, 5, phi );
			vec2 sample2 = vogelDiskSample( 2, 5, phi );
			vec2 sample3 = vogelDiskSample( 3, 5, phi );
			vec2 sample4 = vogelDiskSample( 4, 5, phi );
			shadow = (
				texture( shadowMap, vec4( bd3D + ( tangent * sample0.x + bitangent * sample0.y ) * texelSize, dp ) ) +
				texture( shadowMap, vec4( bd3D + ( tangent * sample1.x + bitangent * sample1.y ) * texelSize, dp ) ) +
				texture( shadowMap, vec4( bd3D + ( tangent * sample2.x + bitangent * sample2.y ) * texelSize, dp ) ) +
				texture( shadowMap, vec4( bd3D + ( tangent * sample3.x + bitangent * sample3.y ) * texelSize, dp ) ) +
				texture( shadowMap, vec4( bd3D + ( tangent * sample4.x + bitangent * sample4.y ) * texelSize, dp ) )
			) * 0.2;
		}
		return mix( 1.0, shadow, shadowIntensity );
	}
	#elif defined( SHADOWMAP_TYPE_BASIC )
	float getPointShadow( samplerCube shadowMap, vec2 shadowMapSize, float shadowIntensity, float shadowBias, float shadowRadius, vec4 shadowCoord, float shadowCameraNear, float shadowCameraFar ) {
		float shadow = 1.0;
		vec3 lightToPosition = shadowCoord.xyz;
		vec3 absVec = abs( lightToPosition );
		float viewSpaceZ = max( max( absVec.x, absVec.y ), absVec.z );
		if ( viewSpaceZ - shadowCameraFar <= 0.0 && viewSpaceZ - shadowCameraNear >= 0.0 ) {
			float dp = ( shadowCameraFar * ( viewSpaceZ - shadowCameraNear ) ) / ( viewSpaceZ * ( shadowCameraFar - shadowCameraNear ) );
			dp += shadowBias;
			vec3 bd3D = normalize( lightToPosition );
			float depth = textureCube( shadowMap, bd3D ).r;
			#ifdef USE_REVERSED_DEPTH_BUFFER
				depth = 1.0 - depth;
			#endif
			shadow = step( dp, depth );
		}
		return mix( 1.0, shadow, shadowIntensity );
	}
	#endif
	#endif
#endif`,qx=`#if NUM_SPOT_LIGHT_COORDS > 0
	uniform mat4 spotLightMatrix[ NUM_SPOT_LIGHT_COORDS ];
	varying vec4 vSpotLightCoord[ NUM_SPOT_LIGHT_COORDS ];
#endif
#ifdef USE_SHADOWMAP
	#if NUM_SUN_LIGHT_SHADOWS > 0
		varying vec4 vSunShadowWorldPosition;
		varying vec3 vSunShadowWorldNormal;
	#endif
	#if NUM_DIR_LIGHT_SHADOWS > 0
		uniform mat4 directionalShadowMatrix[ NUM_DIR_LIGHT_SHADOWS ];
		varying vec4 vDirectionalShadowCoord[ NUM_DIR_LIGHT_SHADOWS ];
		struct DirectionalLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform DirectionalLightShadow directionalLightShadows[ NUM_DIR_LIGHT_SHADOWS ];
	#endif
	#if NUM_SPOT_LIGHT_SHADOWS > 0
		struct SpotLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform SpotLightShadow spotLightShadows[ NUM_SPOT_LIGHT_SHADOWS ];
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
		uniform mat4 pointShadowMatrix[ NUM_POINT_LIGHT_SHADOWS ];
		varying vec4 vPointShadowCoord[ NUM_POINT_LIGHT_SHADOWS ];
		struct PointLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
			float shadowCameraNear;
			float shadowCameraFar;
		};
		uniform PointLightShadow pointLightShadows[ NUM_POINT_LIGHT_SHADOWS ];
	#endif
#endif`,Xx=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_SUN_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
	#ifdef HAS_NORMAL
		vec3 shadowWorldNormal = transformNormalByInverseViewMatrix( transformedNormal, viewMatrix );
	#else
		vec3 shadowWorldNormal = vec3( 0.0 );
	#endif
	vec4 shadowWorldPosition;
#endif
#if defined( USE_SHADOWMAP )
	#if NUM_SUN_LIGHT_SHADOWS > 0
		vSunShadowWorldPosition = vec4( worldPosition.xyz, - mvPosition.z );
		vSunShadowWorldNormal = shadowWorldNormal;
	#endif
	#if NUM_DIR_LIGHT_SHADOWS > 0
		#pragma unroll_loop_start
		for ( int i = 0; i < NUM_DIR_LIGHT_SHADOWS; i ++ ) {
			shadowWorldPosition = worldPosition + vec4( shadowWorldNormal * directionalLightShadows[ i ].shadowNormalBias, 0 );
			vDirectionalShadowCoord[ i ] = directionalShadowMatrix[ i ] * shadowWorldPosition;
		}
		#pragma unroll_loop_end
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
		#pragma unroll_loop_start
		for ( int i = 0; i < NUM_POINT_LIGHT_SHADOWS; i ++ ) {
			shadowWorldPosition = worldPosition + vec4( shadowWorldNormal * pointLightShadows[ i ].shadowNormalBias, 0 );
			vPointShadowCoord[ i ] = pointShadowMatrix[ i ] * shadowWorldPosition;
		}
		#pragma unroll_loop_end
	#endif
#endif
#if NUM_SPOT_LIGHT_COORDS > 0
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SPOT_LIGHT_COORDS; i ++ ) {
		shadowWorldPosition = worldPosition;
		#if ( defined( USE_SHADOWMAP ) && UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS )
			shadowWorldPosition.xyz += shadowWorldNormal * spotLightShadows[ i ].shadowNormalBias;
		#endif
		vSpotLightCoord[ i ] = spotLightMatrix[ i ] * shadowWorldPosition;
	}
	#pragma unroll_loop_end
#endif`,jx=`float getShadowMask() {
	float shadow = 1.0;
	#ifdef USE_SHADOWMAP
	#if NUM_SUN_LIGHT_SHADOWS > 0
	SunLightShadow sunLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SUN_LIGHT_SHADOWS; i ++ ) {
		sunLight = sunLightShadows[ i ];
		shadow *= receiveShadow ? getSunShadow( sunShadowMap[ i ], sunLight, UNROLLED_LOOP_INDEX ) : 1.0;
	}
	#pragma unroll_loop_end
	#endif
	#if NUM_DIR_LIGHT_SHADOWS > 0
	DirectionalLightShadow directionalLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_DIR_LIGHT_SHADOWS; i ++ ) {
		directionalLight = directionalLightShadows[ i ];
		shadow *= receiveShadow ? getShadow( directionalShadowMap[ i ], directionalLight.shadowMapSize, directionalLight.shadowIntensity, directionalLight.shadowBias, directionalLight.shadowRadius, vDirectionalShadowCoord[ i ] ) : 1.0;
	}
	#pragma unroll_loop_end
	#endif
	#if NUM_SPOT_LIGHT_SHADOWS > 0
	SpotLightShadow spotLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SPOT_LIGHT_SHADOWS; i ++ ) {
		spotLight = spotLightShadows[ i ];
		shadow *= receiveShadow ? getShadow( spotShadowMap[ i ], spotLight.shadowMapSize, spotLight.shadowIntensity, spotLight.shadowBias, spotLight.shadowRadius, vSpotLightCoord[ i ] ) : 1.0;
	}
	#pragma unroll_loop_end
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0 && ( defined( SHADOWMAP_TYPE_PCF ) || defined( SHADOWMAP_TYPE_BASIC ) )
	PointLightShadow pointLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_POINT_LIGHT_SHADOWS; i ++ ) {
		pointLight = pointLightShadows[ i ];
		shadow *= receiveShadow ? getPointShadow( pointShadowMap[ i ], pointLight.shadowMapSize, pointLight.shadowIntensity, pointLight.shadowBias, pointLight.shadowRadius, vPointShadowCoord[ i ], pointLight.shadowCameraNear, pointLight.shadowCameraFar ) : 1.0;
	}
	#pragma unroll_loop_end
	#endif
	#endif
	return shadow;
}`,Kx=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,Yx=`#ifdef USE_SKINNING
	uniform mat4 bindMatrix;
	uniform mat4 bindMatrixInverse;
	uniform highp sampler2D boneTexture;
	mat4 getBoneMatrix( const in float i ) {
		int size = textureSize( boneTexture, 0 ).x;
		int j = int( i ) * 4;
		int x = j % size;
		int y = j / size;
		vec4 v1 = texelFetch( boneTexture, ivec2( x, y ), 0 );
		vec4 v2 = texelFetch( boneTexture, ivec2( x + 1, y ), 0 );
		vec4 v3 = texelFetch( boneTexture, ivec2( x + 2, y ), 0 );
		vec4 v4 = texelFetch( boneTexture, ivec2( x + 3, y ), 0 );
		return mat4( v1, v2, v3, v4 );
	}
#endif`,Jx=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,Zx=`#ifdef USE_SKINNING
	mat4 skinMatrix = mat4( 0.0 );
	skinMatrix += skinWeight.x * boneMatX;
	skinMatrix += skinWeight.y * boneMatY;
	skinMatrix += skinWeight.z * boneMatZ;
	skinMatrix += skinWeight.w * boneMatW;
	skinMatrix = bindMatrixInverse * skinMatrix * bindMatrix;
	objectNormal = vec4( skinMatrix * vec4( objectNormal, 0.0 ) ).xyz;
	#ifdef USE_TANGENT
		objectTangent = vec4( skinMatrix * vec4( objectTangent, 0.0 ) ).xyz;
	#endif
#endif`,Qx=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,$x=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,eb=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,tb=`#ifndef saturate
#define saturate( a ) clamp( a, 0.0, 1.0 )
#endif
uniform float toneMappingExposure;
vec3 LinearToneMapping( vec3 color ) {
	return saturate( toneMappingExposure * color );
}
vec3 ReinhardToneMapping( vec3 color ) {
	color *= toneMappingExposure;
	return saturate( color / ( vec3( 1.0 ) + color ) );
}
vec3 CineonToneMapping( vec3 color ) {
	color *= toneMappingExposure;
	color = max( vec3( 0.0 ), color - 0.004 );
	return pow( ( color * ( 6.2 * color + 0.5 ) ) / ( color * ( 6.2 * color + 1.7 ) + 0.06 ), vec3( 2.2 ) );
}
vec3 RRTAndODTFit( vec3 v ) {
	vec3 a = v * ( v + 0.0245786 ) - 0.000090537;
	vec3 b = v * ( 0.983729 * v + 0.4329510 ) + 0.238081;
	return a / b;
}
vec3 ACESFilmicToneMapping( vec3 color ) {
	const mat3 ACESInputMat = mat3(
		vec3( 0.59719, 0.07600, 0.02840 ),		vec3( 0.35458, 0.90834, 0.13383 ),
		vec3( 0.04823, 0.01566, 0.83777 )
	);
	const mat3 ACESOutputMat = mat3(
		vec3(  1.60475, -0.10208, -0.00327 ),		vec3( -0.53108,  1.10813, -0.07276 ),
		vec3( -0.07367, -0.00605,  1.07602 )
	);
	color *= toneMappingExposure / 0.6;
	color = ACESInputMat * color;
	color = RRTAndODTFit( color );
	color = ACESOutputMat * color;
	return saturate( color );
}
const mat3 LINEAR_REC2020_TO_LINEAR_SRGB = mat3(
	vec3( 1.6605, - 0.1246, - 0.0182 ),
	vec3( - 0.5876, 1.1329, - 0.1006 ),
	vec3( - 0.0728, - 0.0083, 1.1187 )
);
const mat3 LINEAR_SRGB_TO_LINEAR_REC2020 = mat3(
	vec3( 0.6274, 0.0691, 0.0164 ),
	vec3( 0.3293, 0.9195, 0.0880 ),
	vec3( 0.0433, 0.0113, 0.8956 )
);
vec3 agxDefaultContrastApprox( vec3 x ) {
	vec3 x2 = x * x;
	vec3 x4 = x2 * x2;
	return + 15.5 * x4 * x2
		- 40.14 * x4 * x
		+ 31.96 * x4
		- 6.868 * x2 * x
		+ 0.4298 * x2
		+ 0.1191 * x
		- 0.00232;
}
vec3 AgXToneMapping( vec3 color ) {
	const mat3 AgXInsetMatrix = mat3(
		vec3( 0.856627153315983, 0.137318972929847, 0.11189821299995 ),
		vec3( 0.0951212405381588, 0.761241990602591, 0.0767994186031903 ),
		vec3( 0.0482516061458583, 0.101439036467562, 0.811302368396859 )
	);
	const mat3 AgXOutsetMatrix = mat3(
		vec3( 1.1271005818144368, - 0.1413297634984383, - 0.14132976349843826 ),
		vec3( - 0.11060664309660323, 1.157823702216272, - 0.11060664309660294 ),
		vec3( - 0.016493938717834573, - 0.016493938717834257, 1.2519364065950405 )
	);
	const float AgxMinEv = - 12.47393;	const float AgxMaxEv = 4.026069;
	color *= toneMappingExposure;
	color = LINEAR_SRGB_TO_LINEAR_REC2020 * color;
	color = AgXInsetMatrix * color;
	color = max( color, 1e-10 );	color = log2( color );
	color = ( color - AgxMinEv ) / ( AgxMaxEv - AgxMinEv );
	color = clamp( color, 0.0, 1.0 );
	color = agxDefaultContrastApprox( color );
	color = AgXOutsetMatrix * color;
	color = pow( max( vec3( 0.0 ), color ), vec3( 2.2 ) );
	color = LINEAR_REC2020_TO_LINEAR_SRGB * color;
	color = clamp( color, 0.0, 1.0 );
	return color;
}
vec3 NeutralToneMapping( vec3 color ) {
	const float StartCompression = 0.8 - 0.04;
	const float Desaturation = 0.15;
	color *= toneMappingExposure;
	float x = min( color.r, min( color.g, color.b ) );
	float offset = x < 0.08 ? x - 6.25 * x * x : 0.04;
	color -= offset;
	float peak = max( color.r, max( color.g, color.b ) );
	if ( peak < StartCompression ) return color;
	float d = 1. - StartCompression;
	float newPeak = 1. - d * d / ( peak + d - StartCompression );
	color *= newPeak / peak;
	float g = 1. - 1. / ( Desaturation * ( peak - newPeak ) + 1. );
	return mix( color, vec3( newPeak ), g );
}
vec3 CustomToneMapping( vec3 color ) { return color; }`,nb=`#ifdef USE_TRANSMISSION
	material.transmission = transmission;
	material.transmissionAlpha = 1.0;
	material.thickness = thickness;
	material.attenuationDistance = attenuationDistance;
	material.attenuationColor = attenuationColor;
	#ifdef USE_TRANSMISSIONMAP
		material.transmission *= texture2D( transmissionMap, vTransmissionMapUv ).r;
	#endif
	#ifdef USE_THICKNESSMAP
		material.thickness *= texture2D( thicknessMap, vThicknessMapUv ).g;
	#endif
	vec3 pos = vWorldPosition;
	vec3 v = normalize( cameraPosition - pos );
	vec3 n = transformNormalByInverseViewMatrix( normal, viewMatrix );
	vec4 transmitted = getIBLVolumeRefraction(
		n, v, material.roughness, material.diffuseContribution, material.specularColorBlended, material.specularF90,
		pos, modelMatrix, viewMatrix, projectionMatrix, material.dispersion, material.ior, material.thickness,
		material.attenuationColor, material.attenuationDistance );
	material.transmissionAlpha = mix( material.transmissionAlpha, transmitted.a, material.transmission );
	totalDiffuse = mix( totalDiffuse, transmitted.rgb, material.transmission );
#endif`,ib=`#ifdef USE_TRANSMISSION
	uniform float transmission;
	uniform float thickness;
	uniform float attenuationDistance;
	uniform vec3 attenuationColor;
	#ifdef USE_TRANSMISSIONMAP
		uniform sampler2D transmissionMap;
	#endif
	#ifdef USE_THICKNESSMAP
		uniform sampler2D thicknessMap;
	#endif
	uniform vec2 transmissionSamplerSize;
	uniform sampler2D transmissionSamplerMap;
	uniform mat4 modelMatrix;
	uniform mat4 projectionMatrix;
	varying vec3 vWorldPosition;
	float w0( float a ) {
		return ( 1.0 / 6.0 ) * ( a * ( a * ( - a + 3.0 ) - 3.0 ) + 1.0 );
	}
	float w1( float a ) {
		return ( 1.0 / 6.0 ) * ( a *  a * ( 3.0 * a - 6.0 ) + 4.0 );
	}
	float w2( float a ){
		return ( 1.0 / 6.0 ) * ( a * ( a * ( - 3.0 * a + 3.0 ) + 3.0 ) + 1.0 );
	}
	float w3( float a ) {
		return ( 1.0 / 6.0 ) * ( a * a * a );
	}
	float g0( float a ) {
		return w0( a ) + w1( a );
	}
	float g1( float a ) {
		return w2( a ) + w3( a );
	}
	float h0( float a ) {
		return - 1.0 + w1( a ) / ( w0( a ) + w1( a ) );
	}
	float h1( float a ) {
		return 1.0 + w3( a ) / ( w2( a ) + w3( a ) );
	}
	vec4 bicubic( sampler2D tex, vec2 uv, vec4 texelSize, float lod ) {
		uv = uv * texelSize.zw + 0.5;
		vec2 iuv = floor( uv );
		vec2 fuv = fract( uv );
		float g0x = g0( fuv.x );
		float g1x = g1( fuv.x );
		float h0x = h0( fuv.x );
		float h1x = h1( fuv.x );
		float h0y = h0( fuv.y );
		float h1y = h1( fuv.y );
		vec2 p0 = ( vec2( iuv.x + h0x, iuv.y + h0y ) - 0.5 ) * texelSize.xy;
		vec2 p1 = ( vec2( iuv.x + h1x, iuv.y + h0y ) - 0.5 ) * texelSize.xy;
		vec2 p2 = ( vec2( iuv.x + h0x, iuv.y + h1y ) - 0.5 ) * texelSize.xy;
		vec2 p3 = ( vec2( iuv.x + h1x, iuv.y + h1y ) - 0.5 ) * texelSize.xy;
		return g0( fuv.y ) * ( g0x * textureLod( tex, p0, lod ) + g1x * textureLod( tex, p1, lod ) ) +
			g1( fuv.y ) * ( g0x * textureLod( tex, p2, lod ) + g1x * textureLod( tex, p3, lod ) );
	}
	vec4 textureBicubic( sampler2D sampler, vec2 uv, float lod ) {
		vec2 fLodSize = vec2( textureSize( sampler, int( lod ) ) );
		vec2 cLodSize = vec2( textureSize( sampler, int( lod + 1.0 ) ) );
		vec2 fLodSizeInv = 1.0 / fLodSize;
		vec2 cLodSizeInv = 1.0 / cLodSize;
		vec4 fSample = bicubic( sampler, uv, vec4( fLodSizeInv, fLodSize ), floor( lod ) );
		vec4 cSample = bicubic( sampler, uv, vec4( cLodSizeInv, cLodSize ), ceil( lod ) );
		return mix( fSample, cSample, fract( lod ) );
	}
	vec3 getVolumeTransmissionRay( const in vec3 n, const in vec3 v, const in float thickness, const in float ior, const in mat4 modelMatrix ) {
		vec3 refractionVector = refract( - v, normalize( n ), 1.0 / ior );
		vec3 modelScale;
		modelScale.x = length( vec3( modelMatrix[ 0 ].xyz ) );
		modelScale.y = length( vec3( modelMatrix[ 1 ].xyz ) );
		modelScale.z = length( vec3( modelMatrix[ 2 ].xyz ) );
		return normalize( refractionVector ) * thickness * modelScale;
	}
	float applyIorToRoughness( const in float roughness, const in float ior ) {
		return roughness * clamp( ior * 2.0 - 2.0, 0.0, 1.0 );
	}
	vec4 getTransmissionSample( const in vec2 fragCoord, const in float roughness, const in float ior ) {
		float lod = log2( transmissionSamplerSize.x ) * applyIorToRoughness( roughness, ior );
		return textureBicubic( transmissionSamplerMap, fragCoord.xy, lod );
	}
	vec3 volumeAttenuation( const in float transmissionDistance, const in vec3 attenuationColor, const in float attenuationDistance ) {
		if ( isinf( attenuationDistance ) ) {
			return vec3( 1.0 );
		} else {
			vec3 attenuationCoefficient = -log( attenuationColor ) / attenuationDistance;
			vec3 transmittance = exp( - attenuationCoefficient * transmissionDistance );			return transmittance;
		}
	}
	vec4 getIBLVolumeRefraction( const in vec3 n, const in vec3 v, const in float roughness, const in vec3 diffuseColor,
		const in vec3 specularColor, const in float specularF90, const in vec3 position, const in mat4 modelMatrix,
		const in mat4 viewMatrix, const in mat4 projMatrix, const in float dispersion, const in float ior, const in float thickness,
		const in vec3 attenuationColor, const in float attenuationDistance ) {
		vec4 transmittedLight;
		vec3 transmittance;
		#ifdef USE_DISPERSION
			float halfSpread = ( ior - 1.0 ) * 0.025 * dispersion;
			vec3 iors = vec3( ior - halfSpread, ior, ior + halfSpread );
			for ( int i = 0; i < 3; i ++ ) {
				vec3 transmissionRay = getVolumeTransmissionRay( n, v, thickness, iors[ i ], modelMatrix );
				vec3 refractedRayExit = position + transmissionRay;
				vec4 ndcPos = projMatrix * viewMatrix * vec4( refractedRayExit, 1.0 );
				vec2 refractionCoords = ndcPos.xy / ndcPos.w;
				refractionCoords += 1.0;
				refractionCoords /= 2.0;
				vec4 transmissionSample = getTransmissionSample( refractionCoords, roughness, iors[ i ] );
				transmittedLight[ i ] = transmissionSample[ i ];
				transmittedLight.a += transmissionSample.a;
				transmittance[ i ] = diffuseColor[ i ] * volumeAttenuation( length( transmissionRay ), attenuationColor, attenuationDistance )[ i ];
			}
			transmittedLight.a /= 3.0;
		#else
			vec3 transmissionRay = getVolumeTransmissionRay( n, v, thickness, ior, modelMatrix );
			vec3 refractedRayExit = position + transmissionRay;
			vec4 ndcPos = projMatrix * viewMatrix * vec4( refractedRayExit, 1.0 );
			vec2 refractionCoords = ndcPos.xy / ndcPos.w;
			refractionCoords += 1.0;
			refractionCoords /= 2.0;
			transmittedLight = getTransmissionSample( refractionCoords, roughness, ior );
			transmittance = diffuseColor * volumeAttenuation( length( transmissionRay ), attenuationColor, attenuationDistance );
		#endif
		vec3 attenuatedColor = transmittance * transmittedLight.rgb;
		vec3 F = EnvironmentBRDF( n, v, specularColor, specularF90, roughness );
		float transmittanceFactor = ( transmittance.r + transmittance.g + transmittance.b ) / 3.0;
		return vec4( ( 1.0 - F ) * attenuatedColor, 1.0 - ( 1.0 - transmittedLight.a ) * transmittanceFactor );
	}
#endif`,sb=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
	varying vec2 vUv;
#endif
#ifdef USE_MAP
	varying vec2 vMapUv;
#endif
#ifdef USE_ALPHAMAP
	varying vec2 vAlphaMapUv;
#endif
#ifdef USE_LIGHTMAP
	varying vec2 vLightMapUv;
#endif
#ifdef USE_AOMAP
	varying vec2 vAoMapUv;
#endif
#ifdef USE_BUMPMAP
	varying vec2 vBumpMapUv;
#endif
#ifdef USE_NORMALMAP
	varying vec2 vNormalMapUv;
#endif
#ifdef USE_EMISSIVEMAP
	varying vec2 vEmissiveMapUv;
#endif
#ifdef USE_METALNESSMAP
	varying vec2 vMetalnessMapUv;
#endif
#ifdef USE_ROUGHNESSMAP
	varying vec2 vRoughnessMapUv;
#endif
#ifdef USE_ANISOTROPYMAP
	varying vec2 vAnisotropyMapUv;
#endif
#ifdef USE_CLEARCOATMAP
	varying vec2 vClearcoatMapUv;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	varying vec2 vClearcoatNormalMapUv;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	varying vec2 vClearcoatRoughnessMapUv;
#endif
#ifdef USE_IRIDESCENCEMAP
	varying vec2 vIridescenceMapUv;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	varying vec2 vIridescenceThicknessMapUv;
#endif
#ifdef USE_SHEEN_COLORMAP
	varying vec2 vSheenColorMapUv;
#endif
#ifdef USE_SHEEN_ROUGHNESSMAP
	varying vec2 vSheenRoughnessMapUv;
#endif
#ifdef USE_SPECULARMAP
	varying vec2 vSpecularMapUv;
#endif
#ifdef USE_SPECULAR_COLORMAP
	varying vec2 vSpecularColorMapUv;
#endif
#ifdef USE_SPECULAR_INTENSITYMAP
	varying vec2 vSpecularIntensityMapUv;
#endif
#ifdef USE_TRANSMISSIONMAP
	uniform mat3 transmissionMapTransform;
	varying vec2 vTransmissionMapUv;
#endif
#ifdef USE_THICKNESSMAP
	uniform mat3 thicknessMapTransform;
	varying vec2 vThicknessMapUv;
#endif`,rb=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
	varying vec2 vUv;
#endif
#ifdef USE_MAP
	uniform mat3 mapTransform;
	varying vec2 vMapUv;
#endif
#ifdef USE_ALPHAMAP
	uniform mat3 alphaMapTransform;
	varying vec2 vAlphaMapUv;
#endif
#ifdef USE_LIGHTMAP
	uniform mat3 lightMapTransform;
	varying vec2 vLightMapUv;
#endif
#ifdef USE_AOMAP
	uniform mat3 aoMapTransform;
	varying vec2 vAoMapUv;
#endif
#ifdef USE_BUMPMAP
	uniform mat3 bumpMapTransform;
	varying vec2 vBumpMapUv;
#endif
#ifdef USE_NORMALMAP
	uniform mat3 normalMapTransform;
	varying vec2 vNormalMapUv;
#endif
#ifdef USE_DISPLACEMENTMAP
	uniform mat3 displacementMapTransform;
	varying vec2 vDisplacementMapUv;
#endif
#ifdef USE_EMISSIVEMAP
	uniform mat3 emissiveMapTransform;
	varying vec2 vEmissiveMapUv;
#endif
#ifdef USE_METALNESSMAP
	uniform mat3 metalnessMapTransform;
	varying vec2 vMetalnessMapUv;
#endif
#ifdef USE_ROUGHNESSMAP
	uniform mat3 roughnessMapTransform;
	varying vec2 vRoughnessMapUv;
#endif
#ifdef USE_ANISOTROPYMAP
	uniform mat3 anisotropyMapTransform;
	varying vec2 vAnisotropyMapUv;
#endif
#ifdef USE_CLEARCOATMAP
	uniform mat3 clearcoatMapTransform;
	varying vec2 vClearcoatMapUv;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform mat3 clearcoatNormalMapTransform;
	varying vec2 vClearcoatNormalMapUv;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform mat3 clearcoatRoughnessMapTransform;
	varying vec2 vClearcoatRoughnessMapUv;
#endif
#ifdef USE_SHEEN_COLORMAP
	uniform mat3 sheenColorMapTransform;
	varying vec2 vSheenColorMapUv;
#endif
#ifdef USE_SHEEN_ROUGHNESSMAP
	uniform mat3 sheenRoughnessMapTransform;
	varying vec2 vSheenRoughnessMapUv;
#endif
#ifdef USE_IRIDESCENCEMAP
	uniform mat3 iridescenceMapTransform;
	varying vec2 vIridescenceMapUv;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform mat3 iridescenceThicknessMapTransform;
	varying vec2 vIridescenceThicknessMapUv;
#endif
#ifdef USE_SPECULARMAP
	uniform mat3 specularMapTransform;
	varying vec2 vSpecularMapUv;
#endif
#ifdef USE_SPECULAR_COLORMAP
	uniform mat3 specularColorMapTransform;
	varying vec2 vSpecularColorMapUv;
#endif
#ifdef USE_SPECULAR_INTENSITYMAP
	uniform mat3 specularIntensityMapTransform;
	varying vec2 vSpecularIntensityMapUv;
#endif
#ifdef USE_TRANSMISSIONMAP
	uniform mat3 transmissionMapTransform;
	varying vec2 vTransmissionMapUv;
#endif
#ifdef USE_THICKNESSMAP
	uniform mat3 thicknessMapTransform;
	varying vec2 vThicknessMapUv;
#endif`,ab=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
	vUv = vec3( uv, 1 ).xy;
#endif
#ifdef USE_MAP
	vMapUv = ( mapTransform * vec3( MAP_UV, 1 ) ).xy;
#endif
#ifdef USE_ALPHAMAP
	vAlphaMapUv = ( alphaMapTransform * vec3( ALPHAMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_LIGHTMAP
	vLightMapUv = ( lightMapTransform * vec3( LIGHTMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_AOMAP
	vAoMapUv = ( aoMapTransform * vec3( AOMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_BUMPMAP
	vBumpMapUv = ( bumpMapTransform * vec3( BUMPMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_NORMALMAP
	vNormalMapUv = ( normalMapTransform * vec3( NORMALMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_DISPLACEMENTMAP
	vDisplacementMapUv = ( displacementMapTransform * vec3( DISPLACEMENTMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_EMISSIVEMAP
	vEmissiveMapUv = ( emissiveMapTransform * vec3( EMISSIVEMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_METALNESSMAP
	vMetalnessMapUv = ( metalnessMapTransform * vec3( METALNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_ROUGHNESSMAP
	vRoughnessMapUv = ( roughnessMapTransform * vec3( ROUGHNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_ANISOTROPYMAP
	vAnisotropyMapUv = ( anisotropyMapTransform * vec3( ANISOTROPYMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_CLEARCOATMAP
	vClearcoatMapUv = ( clearcoatMapTransform * vec3( CLEARCOATMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	vClearcoatNormalMapUv = ( clearcoatNormalMapTransform * vec3( CLEARCOAT_NORMALMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	vClearcoatRoughnessMapUv = ( clearcoatRoughnessMapTransform * vec3( CLEARCOAT_ROUGHNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_IRIDESCENCEMAP
	vIridescenceMapUv = ( iridescenceMapTransform * vec3( IRIDESCENCEMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	vIridescenceThicknessMapUv = ( iridescenceThicknessMapTransform * vec3( IRIDESCENCE_THICKNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SHEEN_COLORMAP
	vSheenColorMapUv = ( sheenColorMapTransform * vec3( SHEEN_COLORMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SHEEN_ROUGHNESSMAP
	vSheenRoughnessMapUv = ( sheenRoughnessMapTransform * vec3( SHEEN_ROUGHNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SPECULARMAP
	vSpecularMapUv = ( specularMapTransform * vec3( SPECULARMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SPECULAR_COLORMAP
	vSpecularColorMapUv = ( specularColorMapTransform * vec3( SPECULAR_COLORMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SPECULAR_INTENSITYMAP
	vSpecularIntensityMapUv = ( specularIntensityMapTransform * vec3( SPECULAR_INTENSITYMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_TRANSMISSIONMAP
	vTransmissionMapUv = ( transmissionMapTransform * vec3( TRANSMISSIONMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_THICKNESSMAP
	vThicknessMapUv = ( thicknessMapTransform * vec3( THICKNESSMAP_UV, 1 ) ).xy;
#endif`,ob=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,lb=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,cb=`uniform sampler2D t2D;
uniform float backgroundIntensity;
varying vec2 vUv;
void main() {
	vec4 texColor = texture2D( t2D, vUv );
	#ifdef DECODE_VIDEO_TEXTURE
		texColor = vec4( mix( pow( texColor.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), texColor.rgb * 0.0773993808, vec3( lessThanEqual( texColor.rgb, vec3( 0.04045 ) ) ) ), texColor.w );
	#endif
	texColor.rgb *= backgroundIntensity;
	gl_FragColor = texColor;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,hb=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,ub=`#ifdef ENVMAP_TYPE_CUBE
	uniform samplerCube envMap;
#elif defined( ENVMAP_TYPE_CUBE_UV )
	uniform sampler2D envMap;
#endif
uniform float backgroundBlurriness;
uniform float backgroundIntensity;
uniform mat3 backgroundRotation;
varying vec3 vWorldDirection;
#include <cube_uv_reflection_fragment>
void main() {
	#ifdef ENVMAP_TYPE_CUBE
		vec4 texColor = textureCube( envMap, backgroundRotation * vWorldDirection );
	#elif defined( ENVMAP_TYPE_CUBE_UV )
		vec4 texColor = textureCubeUV( envMap, backgroundRotation * vWorldDirection, backgroundBlurriness );
	#else
		vec4 texColor = vec4( 0.0, 0.0, 0.0, 1.0 );
	#endif
	texColor.rgb *= backgroundIntensity;
	gl_FragColor = texColor;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,db=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,fb=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,pb=`#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
varying vec2 vHighPrecisionZW;
void main() {
	#include <uv_vertex>
	#include <batching_vertex>
	#include <skinbase_vertex>
	#include <morphinstance_vertex>
	#ifdef USE_DISPLACEMENTMAP
		#include <beginnormal_vertex>
		#include <morphnormal_vertex>
		#include <skinnormal_vertex>
	#endif
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vHighPrecisionZW = gl_Position.zw;
}`,mb=`#if DEPTH_PACKING == 3200
	uniform float opacity;
#endif
#include <common>
#include <packing>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
varying vec2 vHighPrecisionZW;
void main() {
	vec4 diffuseColor = vec4( 1.0 );
	#include <clipping_planes_fragment>
	#if DEPTH_PACKING == 3200
		diffuseColor.a = opacity;
	#endif
	#include <map_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <logdepthbuf_fragment>
	#ifdef USE_REVERSED_DEPTH_BUFFER
		float fragCoordZ = vHighPrecisionZW[ 0 ] / vHighPrecisionZW[ 1 ];
	#else
		float fragCoordZ = 0.5 * vHighPrecisionZW[ 0 ] / vHighPrecisionZW[ 1 ] + 0.5;
	#endif
	#if DEPTH_PACKING == 3200
		gl_FragColor = vec4( vec3( 1.0 - fragCoordZ ), opacity );
	#elif DEPTH_PACKING == 3201
		gl_FragColor = packDepthToRGBA( fragCoordZ );
	#elif DEPTH_PACKING == 3202
		gl_FragColor = vec4( packDepthToRGB( fragCoordZ ), 1.0 );
	#elif DEPTH_PACKING == 3203
		gl_FragColor = vec4( packDepthToRG( fragCoordZ ), 0.0, 1.0 );
	#endif
}`,gb=`#define DISTANCE
varying vec3 vWorldPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <batching_vertex>
	#include <skinbase_vertex>
	#include <morphinstance_vertex>
	#ifdef USE_DISPLACEMENTMAP
		#include <beginnormal_vertex>
		#include <morphnormal_vertex>
		#include <skinnormal_vertex>
	#endif
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <worldpos_vertex>
	#include <clipping_planes_vertex>
	vWorldPosition = worldPosition.xyz;
}`,xb=`#define DISTANCE
uniform vec3 referencePosition;
uniform float nearDistance;
uniform float farDistance;
varying vec3 vWorldPosition;
#include <common>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( 1.0 );
	#include <clipping_planes_fragment>
	#include <map_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	float dist = length( vWorldPosition - referencePosition );
	dist = ( dist - nearDistance ) / ( farDistance - nearDistance );
	dist = saturate( dist );
	gl_FragColor = vec4( dist, 0.0, 0.0, 1.0 );
}`,bb=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,vb=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,_b=`uniform float scale;
attribute float lineDistance;
varying float vLineDistance;
#include <common>
#include <uv_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	vLineDistance = scale * lineDistance;
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <fog_vertex>
}`,yb=`uniform vec3 diffuse;
uniform float opacity;
uniform float dashSize;
uniform float totalSize;
varying float vLineDistance;
#include <common>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <fog_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	if ( mod( vLineDistance, totalSize ) > dashSize ) {
		discard;
	}
	vec3 outgoingLight = vec3( 0.0 );
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	outgoingLight = diffuseColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
}`,Mb=`#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <envmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#if defined ( USE_ENVMAP ) || defined ( USE_SKINNING )
		#include <beginnormal_vertex>
		#include <morphnormal_vertex>
		#include <skinbase_vertex>
		#include <skinnormal_vertex>
		#include <defaultnormal_vertex>
	#endif
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <worldpos_vertex>
	#include <envmap_vertex>
	#include <fog_vertex>
}`,wb=`uniform vec3 diffuse;
uniform float opacity;
#ifndef FLAT_SHADED
	varying vec3 vNormal;
#endif
#include <common>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <fog_pars_fragment>
#include <specularmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <specularmap_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	#ifdef USE_LIGHTMAP
		vec4 lightMapTexel = texture2D( lightMap, vLightMapUv );
		reflectedLight.indirectDiffuse += lightMapTexel.rgb * lightMapIntensity * RECIPROCAL_PI;
	#else
		reflectedLight.indirectDiffuse += vec3( 1.0 );
	#endif
	#include <aomap_fragment>
	reflectedLight.indirectDiffuse *= diffuseColor.rgb;
	vec3 outgoingLight = reflectedLight.indirectDiffuse;
	#include <envmap_fragment>
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Sb=`#define LAMBERT
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <envmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <envmap_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,Eb=`#define LAMBERT
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float opacity;
#include <common>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_lambert_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <specularmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <specularmap_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_lambert_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + totalEmissiveRadiance;
	#include <envmap_fragment>
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Tb=`#define MATCAP
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <color_pars_vertex>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <fog_vertex>
	vViewPosition = - mvPosition.xyz;
}`,Ab=`#define MATCAP
uniform vec3 diffuse;
uniform float opacity;
uniform sampler2D matcap;
varying vec3 vViewPosition;
#include <common>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <fog_pars_fragment>
#include <normal_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	vec3 viewDir = normalize( vViewPosition );
	vec3 x = normalize( vec3( viewDir.z, 0.0, - viewDir.x ) );
	vec3 y = cross( viewDir, x );
	vec2 uv = vec2( dot( x, normal ), dot( y, normal ) ) * 0.495 + 0.5;
	#ifdef USE_MATCAP
		vec4 matcapColor = texture2D( matcap, uv );
	#else
		vec4 matcapColor = vec4( vec3( mix( 0.2, 0.8, uv.y ) ), 1.0 );
	#endif
	vec3 outgoingLight = diffuseColor.rgb * matcapColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Rb=`#define NORMAL
#if defined( FLAT_SHADED ) || defined( USE_BUMPMAP ) || defined( USE_NORMALMAP_TANGENTSPACE )
	varying vec3 vViewPosition;
#endif
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphinstance_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
#if defined( FLAT_SHADED ) || defined( USE_BUMPMAP ) || defined( USE_NORMALMAP_TANGENTSPACE )
	vViewPosition = - mvPosition.xyz;
#endif
}`,Cb=`#define NORMAL
uniform float opacity;
#if defined( FLAT_SHADED ) || defined( USE_BUMPMAP ) || defined( USE_NORMALMAP_TANGENTSPACE )
	varying vec3 vViewPosition;
#endif
#include <uv_pars_fragment>
#include <normal_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( 0.0, 0.0, 0.0, opacity );
	#include <clipping_planes_fragment>
	#include <logdepthbuf_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	gl_FragColor = vec4( normalize( normal ) * 0.5 + 0.5, diffuseColor.a );
	#ifdef OPAQUE
		gl_FragColor.a = 1.0;
	#endif
}`,Pb=`#define PHONG
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <envmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphinstance_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <envmap_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,Ib=`#define PHONG
uniform vec3 diffuse;
uniform vec3 emissive;
uniform vec3 specular;
uniform float shininess;
uniform float opacity;
#include <common>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_phong_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <specularmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <specularmap_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_phong_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + reflectedLight.directSpecular + reflectedLight.indirectSpecular + totalEmissiveRadiance;
	#include <envmap_fragment>
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Db=`#define STANDARD
varying vec3 vViewPosition;
#ifdef USE_TRANSMISSION
	varying vec3 vWorldPosition;
#endif
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
#ifdef USE_TRANSMISSION
	vWorldPosition = worldPosition.xyz;
#endif
}`,Lb=`#define STANDARD
#ifdef PHYSICAL
	#define IOR
	#define USE_SPECULAR
#endif
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;
#ifdef IOR
	uniform float ior;
#endif
#ifdef USE_SPECULAR
	uniform float specularIntensity;
	uniform vec3 specularColor;
	#ifdef USE_SPECULAR_COLORMAP
		uniform sampler2D specularColorMap;
	#endif
	#ifdef USE_SPECULAR_INTENSITYMAP
		uniform sampler2D specularIntensityMap;
	#endif
#endif
#ifdef USE_CLEARCOAT
	uniform float clearcoat;
	uniform float clearcoatRoughness;
#endif
#ifdef USE_DISPERSION
	uniform float dispersion;
#endif
#ifdef USE_RETROREFLECTION
	uniform float retroreflectivity;
#endif
#ifdef USE_IRIDESCENCE
	uniform float iridescence;
	uniform float iridescenceIOR;
	uniform float iridescenceThicknessMinimum;
	uniform float iridescenceThicknessMaximum;
#endif
#ifdef USE_SHEEN
	uniform vec3 sheenColor;
	uniform float sheenRoughness;
	#ifdef USE_SHEEN_COLORMAP
		uniform sampler2D sheenColorMap;
	#endif
	#ifdef USE_SHEEN_ROUGHNESSMAP
		uniform sampler2D sheenRoughnessMap;
	#endif
#endif
#ifdef USE_ANISOTROPY
	uniform vec2 anisotropyVector;
	#ifdef USE_ANISOTROPYMAP
		uniform sampler2D anisotropyMap;
	#endif
#endif
varying vec3 vViewPosition;
#include <common>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <iridescence_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_physical_pars_fragment>
#include <transmission_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <iridescence_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <roughnessmap_fragment>
	#include <metalnessmap_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <clearcoat_normal_fragment_begin>
	#include <clearcoat_normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_physical_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 totalDiffuse = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse;
	vec3 totalSpecular = reflectedLight.directSpecular + reflectedLight.indirectSpecular;
	#include <transmission_fragment>
	vec3 outgoingLight = totalDiffuse + totalSpecular + totalEmissiveRadiance;
	#ifdef USE_SHEEN
 
		outgoingLight = outgoingLight + sheenSpecularDirect + sheenSpecularIndirect;
 
 	#endif
	#ifdef USE_CLEARCOAT
		float dotNVcc = saturate( dot( geometryClearcoatNormal, geometryViewDir ) );
		vec3 Fcc = F_Schlick( material.clearcoatF0, material.clearcoatF90, dotNVcc );
		outgoingLight = outgoingLight * ( 1.0 - material.clearcoat * Fcc ) + ( clearcoatSpecularDirect + clearcoatSpecularIndirect ) * material.clearcoat;
	#endif
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Fb=`#define TOON
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,Nb=`#define TOON
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float opacity;
#include <common>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <gradientmap_pars_fragment>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_toon_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_toon_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + totalEmissiveRadiance;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Ub=`uniform float size;
uniform float scale;
#include <common>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
#ifdef USE_POINTS_UV
	varying vec2 vUv;
	uniform mat3 uvTransform;
#endif
void main() {
	#ifdef USE_POINTS_UV
		vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	#endif
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <project_vertex>
	gl_PointSize = size;
	#ifdef USE_SIZEATTENUATION
		bool isPerspective = isPerspectiveMatrix( projectionMatrix );
		if ( isPerspective ) gl_PointSize *= ( scale / - mvPosition.z );
	#endif
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <worldpos_vertex>
	#include <fog_vertex>
}`,Ob=`uniform vec3 diffuse;
uniform float opacity;
#include <common>
#include <color_pars_fragment>
#include <map_particle_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <fog_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	vec3 outgoingLight = vec3( 0.0 );
	#include <logdepthbuf_fragment>
	#include <map_particle_fragment>
	#include <color_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	outgoingLight = diffuseColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
}`,zb=`#include <common>
#include <batching_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <shadowmap_pars_vertex>
void main() {
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphinstance_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <worldpos_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,kb=`uniform vec3 color;
uniform float opacity;
#include <common>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <logdepthbuf_pars_fragment>
#include <shadowmap_pars_fragment>
#include <shadowmask_pars_fragment>
void main() {
	#include <logdepthbuf_fragment>
	gl_FragColor = vec4( color, opacity * ( 1.0 - getShadowMask() ) );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
}`,Bb=`uniform float rotation;
uniform vec2 center;
#include <common>
#include <uv_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	vec4 mvPosition = modelViewMatrix[ 3 ];
	vec2 scale = vec2( length( modelMatrix[ 0 ].xyz ), length( modelMatrix[ 1 ].xyz ) );
	#ifndef USE_SIZEATTENUATION
		bool isPerspective = isPerspectiveMatrix( projectionMatrix );
		if ( isPerspective ) scale *= - mvPosition.z;
	#endif
	vec2 alignedPosition = ( position.xy - ( center - vec2( 0.5 ) ) ) * scale;
	vec2 rotatedPosition;
	rotatedPosition.x = cos( rotation ) * alignedPosition.x - sin( rotation ) * alignedPosition.y;
	rotatedPosition.y = sin( rotation ) * alignedPosition.x + cos( rotation ) * alignedPosition.y;
	mvPosition.xy += rotatedPosition;
	gl_Position = projectionMatrix * mvPosition;
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <fog_vertex>
}`,Hb=`uniform vec3 diffuse;
uniform float opacity;
#include <common>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <fog_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	vec3 outgoingLight = vec3( 0.0 );
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	outgoingLight = diffuseColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
}`,Dt={alphahash_fragment:l0,alphahash_pars_fragment:c0,alphamap_fragment:h0,alphamap_pars_fragment:u0,alphatest_fragment:d0,alphatest_pars_fragment:f0,aomap_fragment:p0,aomap_pars_fragment:m0,batching_pars_vertex:g0,batching_vertex:x0,begin_vertex:b0,beginnormal_vertex:v0,bsdfs:_0,iridescence_fragment:y0,bumpmap_pars_fragment:M0,clipping_planes_fragment:w0,clipping_planes_pars_fragment:S0,clipping_planes_pars_vertex:E0,clipping_planes_vertex:T0,color_fragment:A0,color_pars_fragment:R0,color_pars_vertex:C0,color_vertex:P0,common:I0,cube_uv_reflection_fragment:D0,defaultnormal_vertex:L0,displacementmap_pars_vertex:F0,displacementmap_vertex:N0,emissivemap_fragment:U0,emissivemap_pars_fragment:O0,colorspace_fragment:z0,colorspace_pars_fragment:k0,envmap_fragment:B0,envmap_common_pars_fragment:H0,envmap_pars_fragment:G0,envmap_pars_vertex:V0,envmap_physical_pars_fragment:ex,envmap_vertex:W0,fog_vertex:q0,fog_pars_vertex:X0,fog_fragment:j0,fog_pars_fragment:K0,gradientmap_pars_fragment:Y0,lightmap_pars_fragment:J0,lights_lambert_fragment:Z0,lights_lambert_pars_fragment:Q0,lights_pars_begin:$0,lights_toon_fragment:tx,lights_toon_pars_fragment:nx,lights_phong_fragment:ix,lights_phong_pars_fragment:sx,lights_physical_fragment:rx,lights_physical_pars_fragment:ax,lights_fragment_begin:ox,lights_fragment_maps:lx,lights_fragment_end:cx,lightprobes_pars_fragment:hx,logdepthbuf_fragment:ux,logdepthbuf_pars_fragment:dx,logdepthbuf_pars_vertex:fx,logdepthbuf_vertex:px,map_fragment:mx,map_pars_fragment:gx,map_particle_fragment:xx,map_particle_pars_fragment:bx,metalnessmap_fragment:vx,metalnessmap_pars_fragment:_x,morphinstance_vertex:yx,morphcolor_vertex:Mx,morphnormal_vertex:wx,morphtarget_pars_vertex:Sx,morphtarget_vertex:Ex,normal_fragment_begin:Tx,normal_fragment_maps:Ax,normal_pars_fragment:Rx,normal_pars_vertex:Cx,normal_vertex:Px,normalmap_pars_fragment:Ix,clearcoat_normal_fragment_begin:Dx,clearcoat_normal_fragment_maps:Lx,clearcoat_pars_fragment:Fx,iridescence_pars_fragment:Nx,opaque_fragment:Ux,packing:Ox,premultiplied_alpha_fragment:zx,project_vertex:kx,dithering_fragment:Bx,dithering_pars_fragment:Hx,roughnessmap_fragment:Gx,roughnessmap_pars_fragment:Vx,shadowmap_pars_fragment:Wx,shadowmap_pars_vertex:qx,shadowmap_vertex:Xx,shadowmask_pars_fragment:jx,skinbase_vertex:Kx,skinning_pars_vertex:Yx,skinning_vertex:Jx,skinnormal_vertex:Zx,specularmap_fragment:Qx,specularmap_pars_fragment:$x,tonemapping_fragment:eb,tonemapping_pars_fragment:tb,transmission_fragment:nb,transmission_pars_fragment:ib,uv_pars_fragment:sb,uv_pars_vertex:rb,uv_vertex:ab,worldpos_vertex:ob,background_vert:lb,background_frag:cb,backgroundCube_vert:hb,backgroundCube_frag:ub,cube_vert:db,cube_frag:fb,depth_vert:pb,depth_frag:mb,distance_vert:gb,distance_frag:xb,equirect_vert:bb,equirect_frag:vb,linedashed_vert:_b,linedashed_frag:yb,meshbasic_vert:Mb,meshbasic_frag:wb,meshlambert_vert:Sb,meshlambert_frag:Eb,meshmatcap_vert:Tb,meshmatcap_frag:Ab,meshnormal_vert:Rb,meshnormal_frag:Cb,meshphong_vert:Pb,meshphong_frag:Ib,meshphysical_vert:Db,meshphysical_frag:Lb,meshtoon_vert:Fb,meshtoon_frag:Nb,points_vert:Ub,points_frag:Ob,shadow_vert:zb,shadow_frag:kb,sprite_vert:Bb,sprite_frag:Hb},Ke={common:{diffuse:{value:new Ce(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new _t},alphaMap:{value:null},alphaMapTransform:{value:new _t},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new _t}},envmap:{envMap:{value:null},envMapRotation:{value:new _t},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98},dfgLUT:{value:null}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new _t}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new _t}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new _t},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new _t},normalScale:{value:new ze(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new _t},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new _t}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new _t}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new _t}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new Ce(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},sunLights:{value:[],properties:{direction:{},color:{}}},sunLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},sunShadowMatrix:{value:[]},sunShadowCascade:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null},probesSH:{value:null},probesMin:{value:new k},probesMax:{value:new k},probesResolution:{value:new k}},points:{diffuse:{value:new Ce(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new _t},alphaTest:{value:0},uvTransform:{value:new _t}},sprite:{diffuse:{value:new Ce(16777215)},opacity:{value:1},center:{value:new ze(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new _t},alphaMap:{value:null},alphaMapTransform:{value:new _t},alphaTest:{value:0}}},Ui={basic:{uniforms:Dn([Ke.common,Ke.specularmap,Ke.envmap,Ke.aomap,Ke.lightmap,Ke.fog]),vertexShader:Dt.meshbasic_vert,fragmentShader:Dt.meshbasic_frag},lambert:{uniforms:Dn([Ke.common,Ke.specularmap,Ke.envmap,Ke.aomap,Ke.lightmap,Ke.emissivemap,Ke.bumpmap,Ke.normalmap,Ke.displacementmap,Ke.fog,Ke.lights,{emissive:{value:new Ce(0)},envMapIntensity:{value:1}}]),vertexShader:Dt.meshlambert_vert,fragmentShader:Dt.meshlambert_frag},phong:{uniforms:Dn([Ke.common,Ke.specularmap,Ke.envmap,Ke.aomap,Ke.lightmap,Ke.emissivemap,Ke.bumpmap,Ke.normalmap,Ke.displacementmap,Ke.fog,Ke.lights,{emissive:{value:new Ce(0)},specular:{value:new Ce(1118481)},shininess:{value:30},envMapIntensity:{value:1}}]),vertexShader:Dt.meshphong_vert,fragmentShader:Dt.meshphong_frag},standard:{uniforms:Dn([Ke.common,Ke.envmap,Ke.aomap,Ke.lightmap,Ke.emissivemap,Ke.bumpmap,Ke.normalmap,Ke.displacementmap,Ke.roughnessmap,Ke.metalnessmap,Ke.fog,Ke.lights,{emissive:{value:new Ce(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:Dt.meshphysical_vert,fragmentShader:Dt.meshphysical_frag},toon:{uniforms:Dn([Ke.common,Ke.aomap,Ke.lightmap,Ke.emissivemap,Ke.bumpmap,Ke.normalmap,Ke.displacementmap,Ke.gradientmap,Ke.fog,Ke.lights,{emissive:{value:new Ce(0)}}]),vertexShader:Dt.meshtoon_vert,fragmentShader:Dt.meshtoon_frag},matcap:{uniforms:Dn([Ke.common,Ke.bumpmap,Ke.normalmap,Ke.displacementmap,Ke.fog,{matcap:{value:null}}]),vertexShader:Dt.meshmatcap_vert,fragmentShader:Dt.meshmatcap_frag},points:{uniforms:Dn([Ke.points,Ke.fog]),vertexShader:Dt.points_vert,fragmentShader:Dt.points_frag},dashed:{uniforms:Dn([Ke.common,Ke.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:Dt.linedashed_vert,fragmentShader:Dt.linedashed_frag},depth:{uniforms:Dn([Ke.common,Ke.displacementmap]),vertexShader:Dt.depth_vert,fragmentShader:Dt.depth_frag},normal:{uniforms:Dn([Ke.common,Ke.bumpmap,Ke.normalmap,Ke.displacementmap,{opacity:{value:1}}]),vertexShader:Dt.meshnormal_vert,fragmentShader:Dt.meshnormal_frag},sprite:{uniforms:Dn([Ke.sprite,Ke.fog]),vertexShader:Dt.sprite_vert,fragmentShader:Dt.sprite_frag},background:{uniforms:{uvTransform:{value:new _t},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:Dt.background_vert,fragmentShader:Dt.background_frag},backgroundCube:{uniforms:{envMap:{value:null},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new _t}},vertexShader:Dt.backgroundCube_vert,fragmentShader:Dt.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:Dt.cube_vert,fragmentShader:Dt.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:Dt.equirect_vert,fragmentShader:Dt.equirect_frag},distance:{uniforms:Dn([Ke.common,Ke.displacementmap,{referencePosition:{value:new k},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:Dt.distance_vert,fragmentShader:Dt.distance_frag},shadow:{uniforms:Dn([Ke.lights,Ke.fog,{color:{value:new Ce(0)},opacity:{value:1}}]),vertexShader:Dt.shadow_vert,fragmentShader:Dt.shadow_frag}};Ui.physical={uniforms:Dn([Ui.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new _t},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new _t},clearcoatNormalScale:{value:new ze(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new _t},dispersion:{value:0},retroreflectivity:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new _t},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new _t},sheen:{value:0},sheenColor:{value:new Ce(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new _t},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new _t},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new _t},transmissionSamplerSize:{value:new ze},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new _t},attenuationDistance:{value:0},attenuationColor:{value:new Ce(0)},specularColor:{value:new Ce(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new _t},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new _t},anisotropyVector:{value:new ze},anisotropyMap:{value:null},anisotropyMapTransform:{value:new _t}}]),vertexShader:Dt.meshphysical_vert,fragmentShader:Dt.meshphysical_frag};var _c={r:0,b:0,g:0},Gb=new mt,gp=new _t;gp.set(-1,0,0,0,1,0,0,0,1);function Vb(s,e,t,n,i,r){let a=new Ce(0),o=i===!0?0:1,l,c,h=null,d=0,u=null;function p(y){let T=y.isScene===!0?y.background:null;if(T&&T.isTexture){let g=y.backgroundBlurriness>0;T=e.get(T,g)}return T}function x(y){let T=!1,g=p(y);g===null?m(a,o):g&&g.isColor&&(m(g,1),T=!0);let _=s.xr.getEnvironmentBlendMode();_==="additive"?t.buffers.color.setClear(0,0,0,1,r):_==="alpha-blend"&&t.buffers.color.setClear(0,0,0,0,r),(s.autoClear||T)&&(t.buffers.depth.setTest(!0),t.buffers.depth.setMask(!0),t.buffers.color.setMask(!0),s.clear(s.autoClearColor,s.autoClearDepth,s.autoClearStencil))}function v(y,T){let g=p(T);g&&(g.isCubeTexture||g.mapping===Qa)?(c===void 0&&(c=new ot(new gi(1,1,1),new wt({name:"BackgroundCubeMaterial",uniforms:Js(Ui.backgroundCube.uniforms),vertexShader:Ui.backgroundCube.vertexShader,fragmentShader:Ui.backgroundCube.fragmentShader,side:xn,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),c.geometry.deleteAttribute("normal"),c.geometry.deleteAttribute("uv"),c.onBeforeRender=function(_,S,A){this.matrixWorld.copyPosition(A.matrixWorld)},Object.defineProperty(c.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),n.update(c)),c.material.uniforms.envMap.value=g,c.material.uniforms.backgroundBlurriness.value=T.backgroundBlurriness,c.material.uniforms.backgroundIntensity.value=T.backgroundIntensity,c.material.uniforms.backgroundRotation.value.setFromMatrix4(Gb.makeRotationFromEuler(T.backgroundRotation)).transpose(),g.isCubeTexture&&g.isRenderTargetTexture===!1&&c.material.uniforms.backgroundRotation.value.premultiply(gp),c.material.toneMapped=Rt.getTransfer(g.colorSpace)!==Wt,(h!==g||d!==g.version||u!==s.toneMapping)&&(c.material.needsUpdate=!0,h=g,d=g.version,u=s.toneMapping),c.layers.enableAll(),y.unshift(c,c.geometry,c.material,0,0,null)):g&&g.isTexture&&(l===void 0&&(l=new ot(new un(2,2),new wt({name:"BackgroundMaterial",uniforms:Js(Ui.background.uniforms),vertexShader:Ui.background.vertexShader,fragmentShader:Ui.background.fragmentShader,side:ii,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),l.geometry.deleteAttribute("normal"),Object.defineProperty(l.material,"map",{get:function(){return this.uniforms.t2D.value}}),n.update(l)),l.material.uniforms.t2D.value=g,l.material.uniforms.backgroundIntensity.value=T.backgroundIntensity,l.material.toneMapped=Rt.getTransfer(g.colorSpace)!==Wt,g.matrixAutoUpdate===!0&&g.updateMatrix(),l.material.uniforms.uvTransform.value.copy(g.matrix),(h!==g||d!==g.version||u!==s.toneMapping)&&(l.material.needsUpdate=!0,h=g,d=g.version,u=s.toneMapping),l.layers.enableAll(),y.unshift(l,l.geometry,l.material,0,0,null))}function m(y,T){y.getRGB(_c,Yh(s)),t.buffers.color.setClear(_c.r,_c.g,_c.b,T,r)}function f(){c!==void 0&&(c.geometry.dispose(),c.material.dispose(),c=void 0),l!==void 0&&(l.geometry.dispose(),l.material.dispose(),l=void 0)}return{getClearColor:function(){return a},setClearColor:function(y,T=1){a.set(y),o=T,m(a,o)},getClearAlpha:function(){return o},setClearAlpha:function(y){o=y,m(a,o)},render:x,addToRenderList:v,dispose:f}}function Wb(s,e){let t=s.getParameter(s.MAX_VERTEX_ATTRIBS),n={},i=u(null),r=i,a=!1;function o(I,M,N,D,U){let H=!1,L=d(I,D,N,M);r!==L&&(r=L,c(r.object)),H=p(I,D,N,U),H&&x(I,D,N,U),U!==null&&e.update(U,s.ELEMENT_ARRAY_BUFFER),(H||a)&&(a=!1,g(I,M,N,D),U!==null&&s.bindBuffer(s.ELEMENT_ARRAY_BUFFER,e.get(U).buffer))}function l(){return s.createVertexArray()}function c(I){return s.bindVertexArray(I)}function h(I){return s.deleteVertexArray(I)}function d(I,M,N,D){let U=D.wireframe===!0,H=n[M.id];H===void 0&&(H={},n[M.id]=H);let L=I.isInstancedMesh===!0?I.id:0,W=H[L];W===void 0&&(W={},H[L]=W);let z=W[N.id];z===void 0&&(z={},W[N.id]=z);let E=z[U];return E===void 0&&(E=u(l()),z[U]=E),E}function u(I){let M=[],N=[],D=[];for(let U=0;U<t;U++)M[U]=0,N[U]=0,D[U]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:M,enabledAttributes:N,attributeDivisors:D,object:I,attributes:{},index:null}}function p(I,M,N,D){let U=r.attributes,H=M.attributes,L=0,W=N.getAttributes();for(let z in W)if(W[z].location>=0){let J=U[z],q=H[z];if(q===void 0&&(z==="instanceMatrix"&&I.instanceMatrix&&(q=I.instanceMatrix),z==="instanceColor"&&I.instanceColor&&(q=I.instanceColor)),J===void 0||J.attribute!==q||q&&J.data!==q.data)return!0;L++}return r.attributesNum!==L||r.index!==D}function x(I,M,N,D){let U={},H=M.attributes,L=0,W=N.getAttributes();for(let z in W)if(W[z].location>=0){let J=H[z];J===void 0&&(z==="instanceMatrix"&&I.instanceMatrix&&(J=I.instanceMatrix),z==="instanceColor"&&I.instanceColor&&(J=I.instanceColor));let q={};q.attribute=J,J&&J.data&&(q.data=J.data),U[z]=q,L++}r.attributes=U,r.attributesNum=L,r.index=D}function v(){let I=r.newAttributes;for(let M=0,N=I.length;M<N;M++)I[M]=0}function m(I){f(I,0)}function f(I,M){let N=r.newAttributes,D=r.enabledAttributes,U=r.attributeDivisors;N[I]=1,D[I]===0&&(s.enableVertexAttribArray(I),D[I]=1),U[I]!==M&&(s.vertexAttribDivisor(I,M),U[I]=M)}function y(){let I=r.newAttributes,M=r.enabledAttributes;for(let N=0,D=M.length;N<D;N++)M[N]!==I[N]&&(s.disableVertexAttribArray(N),M[N]=0)}function T(I,M,N,D,U,H,L){L===!0?s.vertexAttribIPointer(I,M,N,U,H):s.vertexAttribPointer(I,M,N,D,U,H)}function g(I,M,N,D){v();let U=D.attributes,H=N.getAttributes(),L=M.defaultAttributeValues;for(let W in H){let z=H[W];if(z.location>=0){let E=U[W];if(E===void 0&&(W==="instanceMatrix"&&I.instanceMatrix&&(E=I.instanceMatrix),W==="instanceColor"&&I.instanceColor&&(E=I.instanceColor)),E!==void 0){let J=E.normalized,q=E.itemSize,te=e.get(E);if(te===void 0)continue;let Le=te.buffer,Fe=te.type,Ne=te.bytesPerElement,se=Fe===s.INT||Fe===s.UNSIGNED_INT||E.gpuType===Ll;if(E.isInterleavedBufferAttribute){let oe=E.data,Pe=oe.stride,ge=E.offset;if(oe.isInstancedInterleavedBuffer){for(let xe=0;xe<z.locationSize;xe++)f(z.location+xe,oe.meshPerAttribute);I.isInstancedMesh!==!0&&D._maxInstanceCount===void 0&&(D._maxInstanceCount=oe.meshPerAttribute*oe.count)}else for(let xe=0;xe<z.locationSize;xe++)m(z.location+xe);s.bindBuffer(s.ARRAY_BUFFER,Le);for(let xe=0;xe<z.locationSize;xe++)T(z.location+xe,q/z.locationSize,Fe,J,Pe*Ne,(ge+q/z.locationSize*xe)*Ne,se)}else{if(E.isInstancedBufferAttribute){for(let oe=0;oe<z.locationSize;oe++)f(z.location+oe,E.meshPerAttribute);I.isInstancedMesh!==!0&&D._maxInstanceCount===void 0&&(D._maxInstanceCount=E.meshPerAttribute*E.count)}else for(let oe=0;oe<z.locationSize;oe++)m(z.location+oe);s.bindBuffer(s.ARRAY_BUFFER,Le);for(let oe=0;oe<z.locationSize;oe++)T(z.location+oe,q/z.locationSize,Fe,J,q*Ne,q/z.locationSize*oe*Ne,se)}}else if(L!==void 0){let J=L[W];if(J!==void 0)switch(J.length){case 2:s.vertexAttrib2fv(z.location,J);break;case 3:s.vertexAttrib3fv(z.location,J);break;case 4:s.vertexAttrib4fv(z.location,J);break;default:s.vertexAttrib1fv(z.location,J)}}}}y()}function _(){C();for(let I in n){let M=n[I];for(let N in M){let D=M[N];for(let U in D){let H=D[U];for(let L in H)h(H[L].object),delete H[L];delete D[U]}}delete n[I]}}function S(I){if(n[I.id]===void 0)return;let M=n[I.id];for(let N in M){let D=M[N];for(let U in D){let H=D[U];for(let L in H)h(H[L].object),delete H[L];delete D[U]}}delete n[I.id]}function A(I){for(let M in n){let N=n[M];for(let D in N){let U=N[D];if(U[I.id]===void 0)continue;let H=U[I.id];for(let L in H)h(H[L].object),delete H[L];delete U[I.id]}}}function b(I){for(let M in n){let N=n[M],D=I.isInstancedMesh===!0?I.id:0,U=N[D];if(U!==void 0){for(let H in U){let L=U[H];for(let W in L)h(L[W].object),delete L[W];delete U[H]}delete N[D],Object.keys(N).length===0&&delete n[M]}}}function C(){w(),a=!0,r!==i&&(r=i,c(r.object))}function w(){i.geometry=null,i.program=null,i.wireframe=!1}return{setup:o,reset:C,resetDefaultState:w,dispose:_,releaseStatesOfGeometry:S,releaseStatesOfObject:b,releaseStatesOfProgram:A,initAttributes:v,enableAttribute:m,disableUnusedAttributes:y}}function qb(s,e,t){let n;function i(l){n=l}function r(l,c){s.drawArrays(n,l,c),t.update(c,n,1)}function a(l,c,h){h!==0&&(s.drawArraysInstanced(n,l,c,h),t.update(c,n,h))}function o(l,c,h){if(h===0)return;e.get("WEBGL_multi_draw").multiDrawArraysWEBGL(n,l,0,c,0,h);let u=0;for(let p=0;p<h;p++)u+=c[p];t.update(u,n,1)}this.setMode=i,this.render=r,this.renderInstances=a,this.renderMultiDraw=o}function Xb(s,e,t,n){let i;function r(){if(i!==void 0)return i;if(e.has("EXT_texture_filter_anisotropic")===!0){let A=e.get("EXT_texture_filter_anisotropic");i=s.getParameter(A.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else i=0;return i}function a(A){return!(A!==Qn&&n.convert(A)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_FORMAT))}function o(A){let b=A===gn&&(e.has("EXT_color_buffer_half_float")||e.has("EXT_color_buffer_float"));return!(A!==Gn&&A!==Zn&&!b&&n.convert(A)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_TYPE))}function l(A){if(A==="highp"){if(s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.HIGH_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.HIGH_FLOAT).precision>0)return"highp";A="mediump"}return A==="mediump"&&s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.MEDIUM_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let c=t.precision!==void 0?t.precision:"highp",h=l(c);h!==c&&(ht("WebGLRenderer:",c,"not supported, using",h,"instead."),c=h);let d=t.logarithmicDepthBuffer===!0,u=t.reversedDepthBuffer===!0&&e.has("EXT_clip_control");t.reversedDepthBuffer===!0&&u===!1&&ht("WebGLRenderer: Unable to use reversed depth buffer due to missing EXT_clip_control extension. Fallback to default depth buffer.");let p=s.getParameter(s.MAX_TEXTURE_IMAGE_UNITS),x=s.getParameter(s.MAX_VERTEX_TEXTURE_IMAGE_UNITS),v=s.getParameter(s.MAX_TEXTURE_SIZE),m=s.getParameter(s.MAX_CUBE_MAP_TEXTURE_SIZE),f=s.getParameter(s.MAX_VERTEX_ATTRIBS),y=s.getParameter(s.MAX_VERTEX_UNIFORM_VECTORS),T=s.getParameter(s.MAX_VARYING_VECTORS),g=s.getParameter(s.MAX_FRAGMENT_UNIFORM_VECTORS),_=s.getParameter(s.MAX_SAMPLES),S=s.getParameter(s.SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:l,textureFormatReadable:a,textureTypeReadable:o,precision:c,logarithmicDepthBuffer:d,reversedDepthBuffer:u,maxTextures:p,maxVertexTextures:x,maxTextureSize:v,maxCubemapSize:m,maxAttributes:f,maxVertexUniforms:y,maxVaryings:T,maxFragmentUniforms:g,maxSamples:_,samples:S}}function jb(s){let e=this,t=null,n=0,i=!1,r=!1,a=new Kn,o=new _t,l={value:null,needsUpdate:!1};this.uniform=l,this.numPlanes=0,this.numIntersection=0,this.init=function(d,u){let p=d.length!==0||u||n!==0||i;return i=u,n=d.length,p},this.beginShadows=function(){r=!0,h(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(d,u){t=h(d,u,0)},this.setState=function(d,u,p){let x=d.clippingPlanes,v=d.clipIntersection,m=d.clipShadows,f=s.get(d);if(!i||x===null||x.length===0||r&&!m)r?h(null):c();else{let y=r?0:n,T=y*4,g=f.clippingState||null;l.value=g,g=h(x,u,T,p);for(let _=0;_!==T;++_)g[_]=t[_];f.clippingState=g,this.numIntersection=v?this.numPlanes:0,this.numPlanes+=y}};function c(){l.value!==t&&(l.value=t,l.needsUpdate=n>0),e.numPlanes=n,e.numIntersection=0}function h(d,u,p,x){let v=d!==null?d.length:0,m=null;if(v!==0){if(m=l.value,x!==!0||m===null){let f=p+v*4,y=u.matrixWorldInverse;o.getNormalMatrix(y),(m===null||m.length<f)&&(m=new Float32Array(f));for(let T=0,g=p;T!==v;++T,g+=4)a.copy(d[T]).applyMatrix4(y,o),a.normal.toArray(m,g),m[g+3]=a.constant}l.value=m,l.needsUpdate=!0}return e.numPlanes=v,e.numIntersection=0,m}}var Xr=4,Kb=6,Yb=20,Jb=256,oo=new Fi,Kf=new Ce,$h=null,eu=0,tu=0,nu=!1,Zb=new k,Zs=new k,Kr=class{constructor(e){this._renderer=e,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._sizeLods=[],this._lodMeshes=[],this._backgroundBox=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._blurMaterial=null,this._ggxMaterial=null}fromScene(e,t=0,n=.1,i=100,r={}){let{size:a=256,position:o=Zb}=r;$h=this._renderer.getRenderTarget(),eu=this._renderer.getActiveCubeFace(),tu=this._renderer.getActiveMipmapLevel(),nu=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(a);let l=this._allocateTargets();return l.depthBuffer=!0,this._sceneToCubeUV(e,n,i,l,o),t>0&&this._blur(l,0,0,t),this._applyPMREM(l),this._cleanup(l),l}fromEquirectangular(e,t=null){return this._fromTexture(e,t)}fromCubemap(e,t=null){return this._fromTexture(e,t)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=Zf(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=Jf(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose(),this._backgroundBox!==null&&(this._backgroundBox.geometry.dispose(),this._backgroundBox.material.dispose())}_setSize(e){this._lodMax=Math.floor(Math.log2(e)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._ggxMaterial!==null&&this._ggxMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let e=0;e<this._lodMeshes.length;e++)this._lodMeshes[e].geometry.dispose()}_cleanup(e){this._renderer.setRenderTarget($h,eu,tu),this._renderer.xr.enabled=nu,e.scissorTest=!1,qr(e,0,0,e.width,e.height)}_fromTexture(e,t){e.mapping===ys||e.mapping===Ks?this._setSize(e.image.length===0?16:e.image[0].width||e.image[0].image.width):this._setSize(e.image.width/4),$h=this._renderer.getRenderTarget(),eu=this._renderer.getActiveCubeFace(),tu=this._renderer.getActiveMipmapLevel(),nu=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let n=t||this._allocateTargets();return this._textureToCubeUV(e,n),this._applyPMREM(n),this._cleanup(n),n}_allocateTargets(){let e=3*Math.max(this._cubeSize,112),t=4*this._cubeSize,n={magFilter:rn,minFilter:rn,generateMipmaps:!1,type:gn,format:Qn,colorSpace:On,depthBuffer:!1},i=Yf(e,t,n);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==e||this._pingPongRenderTarget.height!==t){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=Yf(e,t,n);let{_lodMax:r}=this;({lodMeshes:this._lodMeshes,sizeLods:this._sizeLods}=Qb(r)),this._blurMaterial=ev(r,e,t),this._ggxMaterial=$b(r,e,t)}return i}_compileMaterial(e){let t=new ot(new Mt,e);this._renderer.compile(t,oo)}_sceneToCubeUV(e,t,n,i,r){let l=new mn(90,1,t,n),c=[1,-1,1,1,1,1],h=[1,1,1,-1,-1,-1],d=this._renderer,u=d.autoClear,p=d.toneMapping;d.getClearColor(Kf),d.toneMapping=vi,d.autoClear=!1,d.state.buffers.depth.getReversed()&&(d.setRenderTarget(i),d.clearDepth(),d.setRenderTarget(null)),this._backgroundBox===null&&(this._backgroundBox=new ot(new gi,new qt({name:"PMREM.Background",side:xn,depthWrite:!1,depthTest:!1})));let v=this._backgroundBox,m=v.material,f=!1,y=e.background;y?y.isColor&&(m.color.copy(y),e.background=null,f=!0):(m.color.copy(Kf),f=!0);for(let T=0;T<6;T++){let g=T%3;g===0?(l.up.set(0,c[T],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x+h[T],r.y,r.z)):g===1?(l.up.set(0,0,c[T]),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y+h[T],r.z)):(l.up.set(0,c[T],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y,r.z+h[T]));let _=this._cubeSize;qr(i,g*_,T>2?_:0,_,_),d.setRenderTarget(i),f&&d.render(v,l),d.render(e,l)}d.toneMapping=p,d.autoClear=u,e.background=y}_textureToCubeUV(e,t){let n=this._renderer,i=e.mapping===ys||e.mapping===Ks;i?(this._cubemapMaterial===null&&(this._cubemapMaterial=Zf()),this._cubemapMaterial.uniforms.flipEnvMap.value=e.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=Jf());let r=i?this._cubemapMaterial:this._equirectMaterial,a=this._lodMeshes[0];a.material=r;let o=r.uniforms;o.envMap.value=e;let l=this._cubeSize;qr(t,0,0,3*l,2*l),n.setRenderTarget(t),n.render(a,oo)}_applyPMREM(e){let t=this._renderer,n=t.autoClear;t.autoClear=!1;let i=this._lodMeshes.length;for(let r=1;r<i;r++)this._applyGGXFilter(e,r-1,r);t.autoClear=n}_applyGGXFilter(e,t,n){let i=this._renderer,r=this._pingPongRenderTarget,a=this._ggxMaterial,o=this._lodMeshes[n];o.material=a;let l=a.uniforms,c=n/(this._lodMeshes.length-1),h=t/(this._lodMeshes.length-1),d=Math.sqrt(c*c-h*h),u=c*1.25,p=d*u,{_lodMax:x}=this,v=this._sizeLods[n],m=3*v*(n>x-Xr?n-x+Xr:0),f=4*(this._cubeSize-v);l.envMap.value=e.texture,l.roughness.value=p,l.mipInt.value=x-t,qr(r,m,f,3*v,2*v),i.setRenderTarget(r),i.render(o,oo),l.envMap.value=r.texture,l.roughness.value=0,l.mipInt.value=x-n,qr(e,m,f,3*v,2*v),i.setRenderTarget(e),i.render(o,oo)}_blur(e,t,n,i){let r=this._pingPongRenderTarget,a=Math.min(i,Math.PI)/Math.SQRT2;this._blurPass(e,r,t,n,a),this._blurPass(r,e,n,n,a)}_blurPass(e,t,n,i,r){let a=this._renderer,o=this._blurMaterial,l=this._lodMeshes[i];l.material=o;let c=o.uniforms;c.envMap.value=e.texture,c.sigma.value=r,c.mipInt.value=this._lodMax-n;let h=this._sizeLods[i],d=3*h*(i>this._lodMax-Xr?i-this._lodMax+Xr:0),u=4*(this._cubeSize-h);qr(t,d,u,3*h,2*h),a.setRenderTarget(t),a.render(l,oo)}};function Qb(s){let e=[],t=[],n=s,i=s-Xr+1+Kb;for(let r=0;r<i;r++){let a=Math.pow(2,n);e.push(a);let o=1/(a-2),l=-o,c=1+o,h=[l,l,c,l,c,c,l,l,c,c,l,c],d=6,u=6,p=3,x=new Float32Array(p*u*d),v=new Float32Array(p*u*d);for(let f=0;f<d;f++){let y=f%3*2/3-1,T=f>2?0:-1,g=[y,T,0,y+2/3,T,0,y+2/3,T+1,0,y,T,0,y+2/3,T+1,0,y,T+1,0];x.set(g,p*u*f);for(let _=0;_<u;_++){let S=h[_*2]*2-1,A=h[_*2+1]*2-1;f===0?Zs.set(1,A,S):f===1?Zs.set(-S,1,-A):f===2?Zs.set(-S,A,1):f===3?Zs.set(-1,A,-S):f===4?Zs.set(-S,-1,A):Zs.set(S,A,-1),Zs.toArray(v,(f*u+_)*p)}}let m=new Mt;m.setAttribute("position",new Tt(x,p)),m.setAttribute("outputDirection",new Tt(v,p)),t.push(new ot(m,null)),n>Xr&&n--}return{lodMeshes:t,sizeLods:e}}function Yf(s,e,t){let n=new an(s,e,t);return n.texture.mapping=Qa,n.texture.name="PMREM.cubeUv",n.scissorTest=!0,n}function qr(s,e,t,n,i){s.viewport.set(e,t,n,i),s.scissor.set(e,t,n,i)}function $b(s,e,t){return new wt({name:"PMREMGGXConvolution",defines:{GGX_SAMPLES:Jb,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},roughness:{value:0},mipInt:{value:0}},vertexShader:Sc(),fragmentShader:`

			precision highp float;
			precision highp int;

			varying vec3 vOutputDirection;

			uniform sampler2D envMap;
			uniform float roughness;
			uniform float mipInt;

			#define ENVMAP_TYPE_CUBE_UV
			#include <cube_uv_reflection_fragment>

			#define PI 3.14159265359

			// Van der Corput radical inverse
			float radicalInverse_VdC(uint bits) {
				bits = (bits << 16u) | (bits >> 16u);
				bits = ((bits & 0x55555555u) << 1u) | ((bits & 0xAAAAAAAAu) >> 1u);
				bits = ((bits & 0x33333333u) << 2u) | ((bits & 0xCCCCCCCCu) >> 2u);
				bits = ((bits & 0x0F0F0F0Fu) << 4u) | ((bits & 0xF0F0F0F0u) >> 4u);
				bits = ((bits & 0x00FF00FFu) << 8u) | ((bits & 0xFF00FF00u) >> 8u);
				return float(bits) * 2.3283064365386963e-10; // / 0x100000000
			}

			// Hammersley sequence
			vec2 hammersley(uint i, uint N) {
				return vec2(float(i) / float(N), radicalInverse_VdC(i));
			}

			// GGX VNDF importance sampling (Eric Heitz 2018)
			// "Sampling the GGX Distribution of Visible Normals"
			// https://jcgt.org/published/0007/04/01/
			vec3 importanceSampleGGX_VNDF(vec2 Xi, vec3 V, float roughness) {
				float alpha = roughness * roughness;

				// Section 4.1: Orthonormal basis
				vec3 T1 = vec3(1.0, 0.0, 0.0);
				vec3 T2 = cross(V, T1);

				// Section 4.2: Parameterization of projected area
				float r = sqrt(Xi.x);
				float phi = 2.0 * PI * Xi.y;
				float t1 = r * cos(phi);
				float t2 = r * sin(phi);
				float s = 0.5 * (1.0 + V.z);
				t2 = (1.0 - s) * sqrt(1.0 - t1 * t1) + s * t2;

				// Section 4.3: Reprojection onto hemisphere
				vec3 Nh = t1 * T1 + t2 * T2 + sqrt(max(0.0, 1.0 - t1 * t1 - t2 * t2)) * V;

				// Section 3.4: Transform back to ellipsoid configuration
				return normalize(vec3(alpha * Nh.x, alpha * Nh.y, max(0.0, Nh.z)));
			}

			void main() {
				vec3 N = normalize(vOutputDirection);
				vec3 V = N; // Assume view direction equals normal for pre-filtering

				vec3 prefilteredColor = vec3(0.0);
				float totalWeight = 0.0;

				// For very low roughness, just sample the environment directly
				if (roughness < 0.001) {
					gl_FragColor = vec4(bilinearCubeUV(envMap, N, mipInt), 1.0);
					return;
				}

				// Tangent space basis for VNDF sampling
				vec3 up = abs(N.z) < 0.999 ? vec3(0.0, 0.0, 1.0) : vec3(1.0, 0.0, 0.0);
				vec3 tangent = normalize(cross(up, N));
				vec3 bitangent = cross(N, tangent);

				for(uint i = 0u; i < uint(GGX_SAMPLES); i++) {
					vec2 Xi = hammersley(i, uint(GGX_SAMPLES));

					// For PMREM, V = N, so in tangent space V is always (0, 0, 1)
					vec3 H_tangent = importanceSampleGGX_VNDF(Xi, vec3(0.0, 0.0, 1.0), roughness);

					// Transform H back to world space
					vec3 H = normalize(tangent * H_tangent.x + bitangent * H_tangent.y + N * H_tangent.z);
					vec3 L = normalize(2.0 * dot(V, H) * H - V);

					float NdotL = max(dot(N, L), 0.0);

					if(NdotL > 0.0) {
						// Sample environment at fixed mip level
						// VNDF importance sampling handles the distribution filtering
						vec3 sampleColor = bilinearCubeUV(envMap, L, mipInt);

						// Weight by NdotL for the split-sum approximation
						// VNDF PDF naturally accounts for the visible microfacet distribution
						prefilteredColor += sampleColor * NdotL;
						totalWeight += NdotL;
					}
				}

				if (totalWeight > 0.0) {
					prefilteredColor = prefilteredColor / totalWeight;
				}

				gl_FragColor = vec4(prefilteredColor, 1.0);
			}
		`,blending:si,depthTest:!1,depthWrite:!1})}function ev(s,e,t){return new wt({name:"SphericalGaussianBlur",defines:{SAMPLES:Yb,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},sigma:{value:0},mipInt:{value:0}},vertexShader:Sc(),fragmentShader:`

			precision highp float;
			precision highp int;

			varying vec3 vOutputDirection;

			uniform sampler2D envMap;
			uniform float sigma;
			uniform float mipInt;

			#define ENVMAP_TYPE_CUBE_UV
			#include <cube_uv_reflection_fragment>

			#define PI 3.14159265359
			#define GOLDEN_ANGLE 2.39996322973

			void main() {

				if ( sigma == 0.0 ) {

					gl_FragColor = vec4( bilinearCubeUV( envMap, vOutputDirection, mipInt ), 1.0 );
					return;

				}

				vec3 outputDirection = normalize( vOutputDirection );

				vec3 up = abs( outputDirection.z ) < 0.999 ? vec3( 0.0, 0.0, 1.0 ) : vec3( 1.0, 0.0, 0.0 );
				vec3 tangent = normalize( cross( up, outputDirection ) );
				vec3 bitangent = cross( outputDirection, tangent );

				// Truncate the kernel at three standard deviations or at the antipode.
				float thetaMax = min( 3.0 * sigma, PI );
				float truncation = 1.0 - exp( - 0.5 * thetaMax * thetaMax / ( sigma * sigma ) );

				vec3 accumColor = vec3( 0.0 );
				float accumWeight = 0.0;

				for ( int i = 0; i < SAMPLES; i ++ ) {

					// Stratified inverse-CDF sampling of the Gaussian, placed on a golden-angle spiral.
					float stratum = ( float( i ) + 0.5 ) / float( SAMPLES );
					float theta = sigma * sqrt( - 2.0 * log( 1.0 - stratum * truncation ) );
					float phi = float( i ) * GOLDEN_ANGLE;

					vec3 offset = cos( phi ) * tangent + sin( phi ) * bitangent;
					vec3 sampleDirection = cos( theta ) * outputDirection + sin( theta ) * offset;

					// Correct the planar sample density to solid angle.
					float weight = sin( theta ) / theta;

					accumColor += weight * bilinearCubeUV( envMap, sampleDirection, mipInt );
					accumWeight += weight;

				}

				gl_FragColor = vec4( accumColor / accumWeight, 1.0 );

			}
		`,blending:si,depthTest:!1,depthWrite:!1})}function Jf(){return new wt({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:Sc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			varying vec3 vOutputDirection;

			uniform sampler2D envMap;

			#include <common>

			void main() {

				vec3 outputDirection = normalize( vOutputDirection );
				vec2 uv = equirectUv( outputDirection );

				gl_FragColor = vec4( texture2D ( envMap, uv ).rgb, 1.0 );

			}
		`,blending:si,depthTest:!1,depthWrite:!1})}function Zf(){return new wt({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:Sc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:si,depthTest:!1,depthWrite:!1})}function Sc(){return`

		precision mediump float;
		precision mediump int;

		attribute vec3 outputDirection;

		varying vec3 vOutputDirection;

		void main() {

			vOutputDirection = outputDirection;
			gl_Position = vec4( position, 1.0 );

		}
	`}var Mc=class extends an{constructor(e=1,t={}){super(e,e,t),this.isWebGLCubeRenderTarget=!0;let n={width:e,height:e,depth:1},i=[n,n,n,n,n,n];this.texture=new Ra(i),this._setTextureOptions(t),this.texture.isRenderTargetTexture=!0}fromEquirectangularTexture(e,t){this.texture.type=t.type,this.texture.colorSpace=t.colorSpace,this.texture.generateMipmaps=t.generateMipmaps,this.texture.minFilter=t.minFilter,this.texture.magFilter=t.magFilter;let n={uniforms:{tEquirect:{value:null}},vertexShader:`

				varying vec3 vWorldDirection;

				vec3 transformDirection( in vec3 dir, in mat4 matrix ) {

					return normalize( ( matrix * vec4( dir, 0.0 ) ).xyz );

				}

				void main() {

					vWorldDirection = transformDirection( position, modelMatrix );

					#include <begin_vertex>
					#include <project_vertex>

				}
			`,fragmentShader:`

				uniform sampler2D tEquirect;

				varying vec3 vWorldDirection;

				#include <common>

				void main() {

					vec3 direction = normalize( vWorldDirection );

					vec2 sampleUV = equirectUv( direction );

					gl_FragColor = texture2D( tEquirect, sampleUV );

				}
			`},i=new gi(5,5,5),r=new wt({name:"CubemapFromEquirect",uniforms:Js(n.uniforms),vertexShader:n.vertexShader,fragmentShader:n.fragmentShader,side:xn,blending:si});r.uniforms.tEquirect.value=t;let a=new ot(i,r),o=t.minFilter;return t.minFilter===_i&&(t.minFilter=rn),new El(1,10,this).update(e,a),t.minFilter=o,a.geometry.dispose(),a.material.dispose(),this}clear(e,t=!0,n=!0,i=!0){let r=e.getRenderTarget();for(let a=0;a<6;a++)e.setRenderTarget(this,a),e.clear(t,n,i);e.setRenderTarget(r)}};function tv(s){let e=new WeakMap,t=new WeakMap,n=null;function i(u,p=!1){return u==null?null:p?a(u):r(u)}function r(u){if(u&&u.isTexture){let p=u.mapping;if(p===Pl||p===Il)if(e.has(u)){let x=e.get(u).texture;return o(x,u.mapping)}else{let x=u.image;if(x&&x.height>0){let v=new Mc(x.height);return v.fromEquirectangularTexture(s,u),e.set(u,v),u.addEventListener("dispose",c),o(v.texture,u.mapping)}else return null}}return u}function a(u){if(u&&u.isTexture){let p=u.mapping,x=p===Pl||p===Il,v=p===ys||p===Ks;if(x||v){let m=t.get(u),f=m!==void 0?m.texture.pmremVersion:0;if(u.isRenderTargetTexture&&u.pmremVersion!==f)return n===null&&(n=new Kr(s)),m=x?n.fromEquirectangular(u,m):n.fromCubemap(u,m),m.texture.pmremVersion=u.pmremVersion,t.set(u,m),m.texture;if(m!==void 0)return m.texture;{let y=u.image;return x&&y&&y.height>0||v&&y&&l(y)?(n===null&&(n=new Kr(s)),m=x?n.fromEquirectangular(u):n.fromCubemap(u),m.texture.pmremVersion=u.pmremVersion,t.set(u,m),u.addEventListener("dispose",h),m.texture):null}}}return u}function o(u,p){return p===Pl?u.mapping=ys:p===Il&&(u.mapping=Ks),u}function l(u){let p=0,x=6;for(let v=0;v<x;v++)u[v]!==void 0&&p++;return p===x}function c(u){let p=u.target;p.removeEventListener("dispose",c);let x=e.get(p);x!==void 0&&(e.delete(p),x.dispose())}function h(u){let p=u.target;p.removeEventListener("dispose",h);let x=t.get(p);x!==void 0&&(t.delete(p),x.dispose())}function d(){e=new WeakMap,t=new WeakMap,n!==null&&(n.dispose(),n=null)}return{get:i,dispose:d}}function nv(s){let e={};function t(n){if(e[n]!==void 0)return e[n];let i=s.getExtension(n);return e[n]=i,i}return{has:function(n){return t(n)!==null},init:function(){t("EXT_color_buffer_float"),t("WEBGL_clip_cull_distance"),t("OES_texture_float_linear"),t("EXT_color_buffer_half_float"),t("WEBGL_multisampled_render_to_texture"),t("WEBGL_render_shared_exponent")},get:function(n){let i=t(n);return i===null&&Ns("WebGLRenderer: "+n+" extension not supported."),i}}}function iv(s,e,t,n){let i={},r=new WeakMap;function a(d){let u=d.target;u.index!==null&&e.remove(u.index);for(let x in u.attributes)e.remove(u.attributes[x]);u.removeEventListener("dispose",a),delete i[u.id];let p=r.get(u);p&&(e.remove(p),r.delete(u)),n.releaseStatesOfGeometry(u),u.isInstancedBufferGeometry===!0&&delete u._maxInstanceCount,t.memory.geometries--}function o(d,u){return i[u.id]===!0||(u.addEventListener("dispose",a),i[u.id]=!0,t.memory.geometries++),u}function l(d){let u=d.attributes;for(let p in u)e.update(u[p],s.ARRAY_BUFFER)}function c(d){let u=[],p=d.index,x=d.attributes.position,v=0;if(x===void 0)return;if(p!==null){let y=p.array;v=p.version;for(let T=0,g=y.length;T<g;T+=3){let _=y[T+0],S=y[T+1],A=y[T+2];u.push(_,S,S,A,A,_)}}else{let y=x.array;v=x.version;for(let T=0,g=y.length/3-1;T<g;T+=3){let _=T+0,S=T+1,A=T+2;u.push(_,S,S,A,A,_)}}let m=new(x.count>=65535?Sa:wa)(u,1);m.version=v;let f=r.get(d);f&&e.remove(f),r.set(d,m)}function h(d){let u=r.get(d);if(u){let p=d.index;p!==null&&u.version<p.version&&c(d)}else c(d);return r.get(d)}return{get:o,update:l,getWireframeAttribute:h}}function sv(s,e,t){let n;function i(d){n=d}let r,a;function o(d){r=d.type,a=d.bytesPerElement}function l(d,u){s.drawElements(n,u,r,d*a),t.update(u,n,1)}function c(d,u,p){p!==0&&(s.drawElementsInstanced(n,u,r,d*a,p),t.update(u,n,p))}function h(d,u,p){if(p===0)return;e.get("WEBGL_multi_draw").multiDrawElementsWEBGL(n,u,0,r,d,0,p);let v=0;for(let m=0;m<p;m++)v+=u[m];t.update(v,n,1)}this.setMode=i,this.setIndex=o,this.render=l,this.renderInstances=c,this.renderMultiDraw=h}function rv(s){let e={geometries:0,textures:0},t={frame:0,calls:0,triangles:0,points:0,lines:0};function n(r,a,o){switch(t.calls++,a){case s.TRIANGLES:t.triangles+=o*(r/3);break;case s.LINES:t.lines+=o*(r/2);break;case s.LINE_STRIP:t.lines+=o*(r-1);break;case s.LINE_LOOP:t.lines+=o*r;break;case s.POINTS:t.points+=o*r;break;default:gt("WebGLInfo: Unknown draw mode:",a);break}}function i(){t.calls=0,t.triangles=0,t.points=0,t.lines=0}return{memory:e,render:t,programs:null,autoReset:!0,reset:i,update:n}}function av(s,e,t){let n=new WeakMap,i=new Nt;function r(a,o,l){let c=a.morphTargetInfluences,h=o.morphAttributes.position||o.morphAttributes.normal||o.morphAttributes.color,d=h!==void 0?h.length:0,u=n.get(o);if(u===void 0||u.count!==d){let C=function(){A.dispose(),n.delete(o),o.removeEventListener("dispose",C)};u!==void 0&&u.texture.dispose();let p=o.morphAttributes.position!==void 0,x=o.morphAttributes.normal!==void 0,v=o.morphAttributes.color!==void 0,m=o.morphAttributes.position||[],f=o.morphAttributes.normal||[],y=o.morphAttributes.color||[],T=0;p===!0&&(T=1),x===!0&&(T=2),v===!0&&(T=3);let g=o.attributes.position.count*T,_=1;g>e.maxTextureSize&&(_=Math.ceil(g/e.maxTextureSize),g=e.maxTextureSize);let S=new Float32Array(g*_*4*d),A=new ya(S,g,_,d);A.type=Zn,A.needsUpdate=!0;let b=T*4;for(let w=0;w<d;w++){let I=m[w],M=f[w],N=y[w],D=g*_*4*w;for(let U=0;U<I.count;U++){let H=U*b;p===!0&&(i.fromBufferAttribute(I,U),S[D+H+0]=i.x,S[D+H+1]=i.y,S[D+H+2]=i.z,S[D+H+3]=0),x===!0&&(i.fromBufferAttribute(M,U),S[D+H+4]=i.x,S[D+H+5]=i.y,S[D+H+6]=i.z,S[D+H+7]=0),v===!0&&(i.fromBufferAttribute(N,U),S[D+H+8]=i.x,S[D+H+9]=i.y,S[D+H+10]=i.z,S[D+H+11]=N.itemSize===4?i.w:1)}}u={count:d,texture:A,size:new ze(g,_)},n.set(o,u),o.addEventListener("dispose",C)}if(a.isInstancedMesh===!0&&a.morphTexture!==null)l.getUniforms().setValue(s,"morphTexture",a.morphTexture,t);else{let p=0;for(let v=0;v<c.length;v++)p+=c[v];let x=o.morphTargetsRelative?1:1-p;l.getUniforms().setValue(s,"morphTargetBaseInfluence",x),l.getUniforms().setValue(s,"morphTargetInfluences",c)}l.getUniforms().setValue(s,"morphTargetsTexture",u.texture,t),l.getUniforms().setValue(s,"morphTargetsTextureSize",u.size)}return{update:r}}function ov(s,e,t,n,i){let r=new WeakMap;function a(c){let h=i.render.frame,d=c.geometry,u=e.get(c,d);if(r.get(u)!==h&&(e.update(u),r.set(u,h)),c.isInstancedMesh&&(c.hasEventListener("dispose",l)===!1&&c.addEventListener("dispose",l),r.get(c)!==h&&(t.update(c.instanceMatrix,s.ARRAY_BUFFER),c.instanceColor!==null&&t.update(c.instanceColor,s.ARRAY_BUFFER),r.set(c,h))),c.isSkinnedMesh){let p=c.skeleton;r.get(p)!==h&&(p.update(),r.set(p,h))}return u}function o(){r=new WeakMap}function l(c){let h=c.target;h.removeEventListener("dispose",l),n.releaseStatesOfObject(h),t.remove(h.instanceMatrix),h.instanceColor!==null&&t.remove(h.instanceColor)}return{update:a,dispose:o}}var lv={[Xa]:"LINEAR_TONE_MAPPING",[ja]:"REINHARD_TONE_MAPPING",[Ka]:"CINEON_TONE_MAPPING",[js]:"ACES_FILMIC_TONE_MAPPING",[Ja]:"AGX_TONE_MAPPING",[Za]:"NEUTRAL_TONE_MAPPING",[Ya]:"CUSTOM_TONE_MAPPING"};function cv(s,e,t,n,i,r){let a=new an(e,t,{type:s,depthBuffer:i,stencilBuffer:r,samples:n?4:0,storeMultisampledDepthBuffer:!1,storeMultisampledStencilBuffer:!1,resolveDepthBuffer:!1,resolveStencilBuffer:!1}),o=null,l=null,c=new Mt;c.setAttribute("position",new xt([-1,3,0,-1,-1,0,3,-1,0],3)),c.setAttribute("uv",new xt([0,2,0,0,2,0],2));let h=new Nr({uniforms:{tDiffuse:{value:null}},vertexShader:`
			precision highp float;

			uniform mat4 modelViewMatrix;
			uniform mat4 projectionMatrix;

			attribute vec3 position;
			attribute vec2 uv;

			varying vec2 vUv;

			void main() {
				vUv = uv;
				gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
			}`,fragmentShader:`
			precision highp float;

			uniform sampler2D tDiffuse;

			varying vec2 vUv;

			#include <tonemapping_pars_fragment>
			#include <colorspace_pars_fragment>

			void main() {
				gl_FragColor = texture2D( tDiffuse, vUv );

				#ifdef LINEAR_TONE_MAPPING
					gl_FragColor.rgb = LinearToneMapping( gl_FragColor.rgb );
				#elif defined( REINHARD_TONE_MAPPING )
					gl_FragColor.rgb = ReinhardToneMapping( gl_FragColor.rgb );
				#elif defined( CINEON_TONE_MAPPING )
					gl_FragColor.rgb = CineonToneMapping( gl_FragColor.rgb );
				#elif defined( ACES_FILMIC_TONE_MAPPING )
					gl_FragColor.rgb = ACESFilmicToneMapping( gl_FragColor.rgb );
				#elif defined( AGX_TONE_MAPPING )
					gl_FragColor.rgb = AgXToneMapping( gl_FragColor.rgb );
				#elif defined( NEUTRAL_TONE_MAPPING )
					gl_FragColor.rgb = NeutralToneMapping( gl_FragColor.rgb );
				#elif defined( CUSTOM_TONE_MAPPING )
					gl_FragColor.rgb = CustomToneMapping( gl_FragColor.rgb );
				#endif

				#ifdef SRGB_TRANSFER
					gl_FragColor = sRGBTransferOETF( gl_FragColor );
				#endif
			}`,depthTest:!1,depthWrite:!1}),d=new ot(c,h),u=new Fi(-1,1,1,-1,0,1),p=null,x=null,v=!1,m,f=null,y=[],T=!1;this.setSize=function(g,_){a.setSize(g,_),o!==null&&o.setSize(g,_),l!==null&&l.setSize(g,_);for(let S=0;S<y.length;S++){let A=y[S];A.setSize&&A.setSize(g,_)}},this.setEffects=function(g){y=g,T=y.length>0&&y[0].isRenderPass===!0;let _=a.width,S=a.height;y.length>0&&o===null&&(o=new an(_,S,{type:gn,depthBuffer:!1,stencilBuffer:!1}),l=new an(_,S,{type:gn,depthBuffer:!1,stencilBuffer:!1}));for(let A=0;A<y.length;A++){let b=y[A];b.setSize&&b.setSize(_,S)}},this.begin=function(g,_){if(v||g.toneMapping===vi&&y.length===0)return!1;if(f=_,_!==null){let S=_.width,A=_.height;(a.width!==S||a.height!==A)&&this.setSize(S,A)}return T===!1&&g.setRenderTarget(a),m=g.toneMapping,g.toneMapping=vi,!0},this.hasRenderPass=function(){return T},this.end=function(g,_){g.toneMapping=m,v=!0;let S=a,A=o;for(let b=0;b<y.length;b++){let C=y[b];C.enabled!==!1&&(C.render(g,A,S,_),C.needsSwap!==!1&&(S=A,A=A===o?l:o))}if(p!==g.outputColorSpace||x!==g.toneMapping){p=g.outputColorSpace,x=g.toneMapping,h.defines={},Rt.getTransfer(p)===Wt&&(h.defines.SRGB_TRANSFER="");let b=lv[x];b&&(h.defines[b]=""),h.needsUpdate=!0}h.uniforms.tDiffuse.value=S.texture,g.setRenderTarget(f),g.render(d,u),f=null,v=!1},this.isCompositing=function(){return v},this.dispose=function(){a.dispose(),o!==null&&o.dispose(),l!==null&&l.dispose(),c.dispose(),h.dispose()}}var xp=new cn,ru=new ms(1,1),bp=new ya,vp=new al,_p=new Ra,Qf=[],$f=[],ep=new Float32Array(16),tp=new Float32Array(9),np=new Float32Array(4);function Yr(s,e,t){let n=s[0];if(n<=0||n>0)return s;let i=e*t,r=Qf[i];if(r===void 0&&(r=new Float32Array(i),Qf[i]=r),e!==0){n.toArray(r,0);for(let a=1,o=0;a!==e;++a)o+=t,s[a].toArray(r,o)}return r}function bn(s,e){if(s.length!==e.length)return!1;for(let t=0,n=s.length;t<n;t++)if(s[t]!==e[t])return!1;return!0}function vn(s,e){for(let t=0,n=e.length;t<n;t++)s[t]=e[t]}function Ec(s,e){let t=$f[e];t===void 0&&(t=new Int32Array(e),$f[e]=t);for(let n=0;n!==e;++n)t[n]=s.allocateTextureUnit();return t}function hv(s,e){let t=this.cache;t[0]!==e&&(s.uniform1f(this.addr,e),t[0]=e)}function uv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2f(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(bn(t,e))return;s.uniform2fv(this.addr,e),vn(t,e)}}function dv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3f(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else if(e.r!==void 0)(t[0]!==e.r||t[1]!==e.g||t[2]!==e.b)&&(s.uniform3f(this.addr,e.r,e.g,e.b),t[0]=e.r,t[1]=e.g,t[2]=e.b);else{if(bn(t,e))return;s.uniform3fv(this.addr,e),vn(t,e)}}function fv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4f(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(bn(t,e))return;s.uniform4fv(this.addr,e),vn(t,e)}}function pv(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(bn(t,e))return;s.uniformMatrix2fv(this.addr,!1,e),vn(t,e)}else{if(bn(t,n))return;np.set(n),s.uniformMatrix2fv(this.addr,!1,np),vn(t,n)}}function mv(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(bn(t,e))return;s.uniformMatrix3fv(this.addr,!1,e),vn(t,e)}else{if(bn(t,n))return;tp.set(n),s.uniformMatrix3fv(this.addr,!1,tp),vn(t,n)}}function gv(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(bn(t,e))return;s.uniformMatrix4fv(this.addr,!1,e),vn(t,e)}else{if(bn(t,n))return;ep.set(n),s.uniformMatrix4fv(this.addr,!1,ep),vn(t,n)}}function xv(s,e){let t=this.cache;t[0]!==e&&(s.uniform1i(this.addr,e),t[0]=e)}function bv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2i(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(bn(t,e))return;s.uniform2iv(this.addr,e),vn(t,e)}}function vv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3i(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(bn(t,e))return;s.uniform3iv(this.addr,e),vn(t,e)}}function _v(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4i(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(bn(t,e))return;s.uniform4iv(this.addr,e),vn(t,e)}}function yv(s,e){let t=this.cache;t[0]!==e&&(s.uniform1ui(this.addr,e),t[0]=e)}function Mv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2ui(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(bn(t,e))return;s.uniform2uiv(this.addr,e),vn(t,e)}}function wv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3ui(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(bn(t,e))return;s.uniform3uiv(this.addr,e),vn(t,e)}}function Sv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4ui(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(bn(t,e))return;s.uniform4uiv(this.addr,e),vn(t,e)}}function Ev(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i);let r;this.type===s.SAMPLER_2D_SHADOW?(ru.compareFunction=t.isReversedDepthBuffer()?vc:bc,r=ru):r=xp,t.setTexture2D(e||r,i)}function Tv(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture3D(e||vp,i)}function Av(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTextureCube(e||_p,i)}function Rv(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture2DArray(e||bp,i)}function Cv(s){switch(s){case 5126:return hv;case 35664:return uv;case 35665:return dv;case 35666:return fv;case 35674:return pv;case 35675:return mv;case 35676:return gv;case 5124:case 35670:return xv;case 35667:case 35671:return bv;case 35668:case 35672:return vv;case 35669:case 35673:return _v;case 5125:return yv;case 36294:return Mv;case 36295:return wv;case 36296:return Sv;case 35678:case 36198:case 36298:case 36306:case 35682:return Ev;case 35679:case 36299:case 36307:return Tv;case 35680:case 36300:case 36308:case 36293:return Av;case 36289:case 36303:case 36311:case 36292:return Rv}}function Pv(s,e){s.uniform1fv(this.addr,e)}function Iv(s,e){let t=Yr(e,this.size,2);s.uniform2fv(this.addr,t)}function Dv(s,e){let t=Yr(e,this.size,3);s.uniform3fv(this.addr,t)}function Lv(s,e){let t=Yr(e,this.size,4);s.uniform4fv(this.addr,t)}function Fv(s,e){let t=Yr(e,this.size,4);s.uniformMatrix2fv(this.addr,!1,t)}function Nv(s,e){let t=Yr(e,this.size,9);s.uniformMatrix3fv(this.addr,!1,t)}function Uv(s,e){let t=Yr(e,this.size,16);s.uniformMatrix4fv(this.addr,!1,t)}function Ov(s,e){s.uniform1iv(this.addr,e)}function zv(s,e){s.uniform2iv(this.addr,e)}function kv(s,e){s.uniform3iv(this.addr,e)}function Bv(s,e){s.uniform4iv(this.addr,e)}function Hv(s,e){s.uniform1uiv(this.addr,e)}function Gv(s,e){s.uniform2uiv(this.addr,e)}function Vv(s,e){s.uniform3uiv(this.addr,e)}function Wv(s,e){s.uniform4uiv(this.addr,e)}function qv(s,e,t){let n=this.cache,i=e.length,r=Ec(t,i);bn(n,r)||(s.uniform1iv(this.addr,r),vn(n,r));let a;this.type===s.SAMPLER_2D_SHADOW?a=ru:a=xp;for(let o=0;o!==i;++o)t.setTexture2D(e[o]||a,r[o])}function Xv(s,e,t){let n=this.cache,i=e.length,r=Ec(t,i);bn(n,r)||(s.uniform1iv(this.addr,r),vn(n,r));for(let a=0;a!==i;++a)t.setTexture3D(e[a]||vp,r[a])}function jv(s,e,t){let n=this.cache,i=e.length,r=Ec(t,i);bn(n,r)||(s.uniform1iv(this.addr,r),vn(n,r));for(let a=0;a!==i;++a)t.setTextureCube(e[a]||_p,r[a])}function Kv(s,e,t){let n=this.cache,i=e.length,r=Ec(t,i);bn(n,r)||(s.uniform1iv(this.addr,r),vn(n,r));for(let a=0;a!==i;++a)t.setTexture2DArray(e[a]||bp,r[a])}function Yv(s){switch(s){case 5126:return Pv;case 35664:return Iv;case 35665:return Dv;case 35666:return Lv;case 35674:return Fv;case 35675:return Nv;case 35676:return Uv;case 5124:case 35670:return Ov;case 35667:case 35671:return zv;case 35668:case 35672:return kv;case 35669:case 35673:return Bv;case 5125:return Hv;case 36294:return Gv;case 36295:return Vv;case 36296:return Wv;case 35678:case 36198:case 36298:case 36306:case 35682:return qv;case 35679:case 36299:case 36307:return Xv;case 35680:case 36300:case 36308:case 36293:return jv;case 36289:case 36303:case 36311:case 36292:return Kv}}var au=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.setValue=Cv(t.type)}},ou=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.size=t.size,this.setValue=Yv(t.type)}},lu=class{constructor(e){this.id=e,this.seq=[],this.map={}}setValue(e,t,n){let i=this.seq;for(let r=0,a=i.length;r!==a;++r){let o=i[r];o.setValue(e,t[o.id],n)}}},iu=/(\w+)(\])?(\[|\.)?/g;function ip(s,e){s.seq.push(e),s.map[e.id]=e}function Jv(s,e,t){let n=s.name,i=n.length;for(iu.lastIndex=0;;){let r=iu.exec(n),a=iu.lastIndex,o=r[1],l=r[2]==="]",c=r[3];if(l&&(o=o|0),c===void 0||c==="["&&a+2===i){ip(t,c===void 0?new au(o,s,e):new ou(o,s,e));break}else{let d=t.map[o];d===void 0&&(d=new lu(o),ip(t,d)),t=d}}}var jr=class{constructor(e,t){this.seq=[],this.map={};let n=e.getProgramParameter(t,e.ACTIVE_UNIFORMS);for(let a=0;a<n;++a){let o=e.getActiveUniform(t,a),l=e.getUniformLocation(t,o.name);Jv(o,l,this)}let i=[],r=[];for(let a of this.seq)a.type===e.SAMPLER_2D_SHADOW||a.type===e.SAMPLER_CUBE_SHADOW||a.type===e.SAMPLER_2D_ARRAY_SHADOW?i.push(a):r.push(a);i.length>0&&(this.seq=i.concat(r))}setValue(e,t,n,i){let r=this.map[t];r!==void 0&&r.setValue(e,n,i)}setOptional(e,t,n){let i=t[n];i!==void 0&&this.setValue(e,n,i)}static upload(e,t,n,i){for(let r=0,a=t.length;r!==a;++r){let o=t[r],l=n[o.id];l.needsUpdate!==!1&&o.setValue(e,l.value,i)}}static seqWithValue(e,t){let n=[];for(let i=0,r=e.length;i!==r;++i){let a=e[i];a.id in t&&n.push(a)}return n}};function sp(s,e,t){let n=s.createShader(e);return s.shaderSource(n,t),s.compileShader(n),n}var Zv=37297,Qv=0;function $v(s,e){let t=s.split(`
`),n=[],i=Math.max(e-6,0),r=Math.min(e+6,t.length);for(let a=i;a<r;a++){let o=a+1;n.push(`${o===e?">":" "} ${o}: ${t[a]}`)}return n.join(`
`)}var rp=new _t;function e_(s){Rt._getMatrix(rp,Rt.workingColorSpace,s);let e=`mat3( ${rp.elements.map(t=>t.toFixed(4))} )`;switch(Rt.getTransfer(s)){case va:return[e,"LinearTransferOETF"];case Wt:return[e,"sRGBTransferOETF"];default:return ht("WebGLProgram: Unsupported color space: ",s),[e,"LinearTransferOETF"]}}function ap(s,e,t){let n=s.getShaderParameter(e,s.COMPILE_STATUS),r=(s.getShaderInfoLog(e)||"").trim();if(n&&r==="")return"";let a=/ERROR: 0:(\d+)/.exec(r);if(a){let o=parseInt(a[1]);return t.toUpperCase()+`

`+r+`

`+$v(s.getShaderSource(e),o)}else return r}function t_(s,e){let t=e_(e);return[`vec4 ${s}( vec4 value ) {`,`	return ${t[1]}( vec4( value.rgb * ${t[0]}, value.a ) );`,"}"].join(`
`)}var n_={[Xa]:"Linear",[ja]:"Reinhard",[Ka]:"Cineon",[js]:"ACESFilmic",[Ja]:"AgX",[Za]:"Neutral",[Ya]:"Custom"};function i_(s,e){let t=n_[e];return t===void 0?(ht("WebGLProgram: Unsupported toneMapping:",e),"vec3 "+s+"( vec3 color ) { return LinearToneMapping( color ); }"):"vec3 "+s+"( vec3 color ) { return "+t+"ToneMapping( color ); }"}var yc=new k;function s_(){Rt.getLuminanceCoefficients(yc);let s=yc.x.toFixed(4),e=yc.y.toFixed(4),t=yc.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${s}, ${e}, ${t} );`,"	return dot( weights, rgb );","}"].join(`
`)}function r_(s){return[s.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",s.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(co).join(`
`)}function a_(s){let e=[];for(let t in s){let n=s[t];n!==!1&&e.push("#define "+t+" "+n)}return e.join(`
`)}function o_(s,e){let t={},n=s.getProgramParameter(e,s.ACTIVE_ATTRIBUTES);for(let i=0;i<n;i++){let r=s.getActiveAttrib(e,i),a=r.name,o=1;r.type===s.FLOAT_MAT2&&(o=2),r.type===s.FLOAT_MAT3&&(o=3),r.type===s.FLOAT_MAT4&&(o=4),t[a]={type:r.type,location:s.getAttribLocation(e,a),locationSize:o}}return t}function co(s){return s!==""}function op(s,e){let t=e.numSpotLightShadows+e.numSpotLightMaps-e.numSpotLightShadowsWithMaps;return s.replace(/NUM_SUN_LIGHTS/g,e.numSunLights).replace(/NUM_DIR_LIGHTS/g,e.numDirLights).replace(/NUM_SPOT_LIGHTS/g,e.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,e.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,t).replace(/NUM_RECT_AREA_LIGHTS/g,e.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,e.numPointLights).replace(/NUM_HEMI_LIGHTS/g,e.numHemiLights).replace(/NUM_SUN_LIGHT_SHADOWS/g,e.numSunLightShadows).replace(/NUM_DIR_LIGHT_SHADOWS/g,e.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,e.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,e.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,e.numPointLightShadows)}function lp(s,e){return s.replace(/NUM_CLIPPING_PLANES/g,e.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,e.numClippingPlanes-e.numClipIntersection)}var l_=/^[ \t]*#include +<([\w\d./]+)>/gm;function cu(s){return s.replace(l_,h_)}var c_=new Map;function h_(s,e){let t=Dt[e];if(t===void 0){let n=c_.get(e);if(n!==void 0)t=Dt[n],ht('WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',e,n);else throw new Error("THREE.WebGLProgram: Can not resolve #include <"+e+">")}return cu(t)}var u_=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function cp(s){return s.replace(u_,d_)}function d_(s,e,t,n){let i="";for(let r=parseInt(e);r<parseInt(t);r++)i+=n.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return i}function hp(s){let e=`precision ${s.precision} float;
	precision ${s.precision} int;
	precision ${s.precision} sampler2D;
	precision ${s.precision} samplerCube;
	precision ${s.precision} sampler3D;
	precision ${s.precision} sampler2DArray;
	precision ${s.precision} sampler2DShadow;
	precision ${s.precision} samplerCubeShadow;
	precision ${s.precision} sampler2DArrayShadow;
	precision ${s.precision} isampler2D;
	precision ${s.precision} isampler3D;
	precision ${s.precision} isamplerCube;
	precision ${s.precision} isampler2DArray;
	precision ${s.precision} usampler2D;
	precision ${s.precision} usampler3D;
	precision ${s.precision} usamplerCube;
	precision ${s.precision} usampler2DArray;
	`;return s.precision==="highp"?e+=`
#define HIGH_PRECISION`:s.precision==="mediump"?e+=`
#define MEDIUM_PRECISION`:s.precision==="lowp"&&(e+=`
#define LOW_PRECISION`),e}var f_={[qs]:"SHADOWMAP_TYPE_PCF",[kr]:"SHADOWMAP_TYPE_VSM"};function p_(s){return f_[s.shadowMapType]||"SHADOWMAP_TYPE_BASIC"}var m_={[ys]:"ENVMAP_TYPE_CUBE",[Ks]:"ENVMAP_TYPE_CUBE",[Qa]:"ENVMAP_TYPE_CUBE_UV"};function g_(s){return s.envMap===!1?"ENVMAP_TYPE_CUBE":m_[s.envMapMode]||"ENVMAP_TYPE_CUBE"}var x_={[Ks]:"ENVMAP_MODE_REFRACTION"};function b_(s){return s.envMap===!1?"ENVMAP_MODE_REFLECTION":x_[s.envMapMode]||"ENVMAP_MODE_REFLECTION"}var v_={[Cl]:"ENVMAP_BLENDING_MULTIPLY",[Tf]:"ENVMAP_BLENDING_MIX",[Af]:"ENVMAP_BLENDING_ADD"};function __(s){return s.envMap===!1?"ENVMAP_BLENDING_NONE":v_[s.combine]||"ENVMAP_BLENDING_NONE"}function y_(s){let e=s.envMapCubeUVHeight;if(e===null)return null;let t=Math.log2(e)-2,n=1/e;return{texelWidth:1/(3*Math.max(Math.pow(2,t),112)),texelHeight:n,maxMip:t}}function M_(s,e,t,n){let i=s.getContext(),r=t.defines,a=t.vertexShader,o=t.fragmentShader,l=p_(t),c=g_(t),h=b_(t),d=__(t),u=y_(t),p=r_(t),x=a_(r),v=i.createProgram(),m,f,y=t.glslVersion?"#version "+t.glslVersion+`
`:"";t.isRawShaderMaterial?(m=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x].filter(co).join(`
`),m.length>0&&(m+=`
`),f=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x].filter(co).join(`
`),f.length>0&&(f+=`
`)):(m=[hp(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x,t.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",t.batching?"#define USE_BATCHING":"",t.batchingColor?"#define USE_BATCHING_COLOR":"",t.instancing?"#define USE_INSTANCING":"",t.instancingColor?"#define USE_INSTANCING_COLOR":"",t.instancingMorph?"#define USE_INSTANCING_MORPH":"",t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.map?"#define USE_MAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+h:"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.displacementMap?"#define USE_DISPLACEMENTMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.mapUv?"#define MAP_UV "+t.mapUv:"",t.alphaMapUv?"#define ALPHAMAP_UV "+t.alphaMapUv:"",t.lightMapUv?"#define LIGHTMAP_UV "+t.lightMapUv:"",t.aoMapUv?"#define AOMAP_UV "+t.aoMapUv:"",t.emissiveMapUv?"#define EMISSIVEMAP_UV "+t.emissiveMapUv:"",t.bumpMapUv?"#define BUMPMAP_UV "+t.bumpMapUv:"",t.normalMapUv?"#define NORMALMAP_UV "+t.normalMapUv:"",t.displacementMapUv?"#define DISPLACEMENTMAP_UV "+t.displacementMapUv:"",t.metalnessMapUv?"#define METALNESSMAP_UV "+t.metalnessMapUv:"",t.roughnessMapUv?"#define ROUGHNESSMAP_UV "+t.roughnessMapUv:"",t.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+t.anisotropyMapUv:"",t.clearcoatMapUv?"#define CLEARCOATMAP_UV "+t.clearcoatMapUv:"",t.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+t.clearcoatNormalMapUv:"",t.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+t.clearcoatRoughnessMapUv:"",t.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+t.iridescenceMapUv:"",t.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+t.iridescenceThicknessMapUv:"",t.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+t.sheenColorMapUv:"",t.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+t.sheenRoughnessMapUv:"",t.specularMapUv?"#define SPECULARMAP_UV "+t.specularMapUv:"",t.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+t.specularColorMapUv:"",t.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+t.specularIntensityMapUv:"",t.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+t.transmissionMapUv:"",t.thicknessMapUv?"#define THICKNESSMAP_UV "+t.thicknessMapUv:"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexNormals?"#define HAS_NORMAL":"",t.vertexColors?"#define USE_COLOR":"",t.vertexAlphas?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.flatShading?"#define FLAT_SHADED":"",t.skinning?"#define USE_SKINNING":"",t.morphTargets?"#define USE_MORPHTARGETS":"",t.morphNormals&&t.flatShading===!1?"#define USE_MORPHNORMALS":"",t.morphColors?"#define USE_MORPHCOLORS":"",t.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+t.morphTextureStride:"",t.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+t.morphTargetsCount:"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.sizeAttenuation?"#define USE_SIZEATTENUATION":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(co).join(`
`),f=[hp(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x,t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",t.map?"#define USE_MAP":"",t.matcap?"#define USE_MATCAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+c:"",t.envMap?"#define "+h:"",t.envMap?"#define "+d:"",u?"#define CUBEUV_TEXEL_WIDTH "+u.texelWidth:"",u?"#define CUBEUV_TEXEL_HEIGHT "+u.texelHeight:"",u?"#define CUBEUV_MAX_MIP "+u.maxMip+".0":"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.packedNormalMap?"#define USE_PACKED_NORMALMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoat?"#define USE_CLEARCOAT":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.dispersion?"#define USE_DISPERSION":"",t.retroreflection?"#define USE_RETROREFLECTION":"",t.iridescence?"#define USE_IRIDESCENCE":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaTest?"#define USE_ALPHATEST":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.sheen?"#define USE_SHEEN":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexColors||t.instancingColor?"#define USE_COLOR":"",t.vertexAlphas||t.batchingColor?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.gradientMap?"#define USE_GRADIENTMAP":"",t.flatShading?"#define FLAT_SHADED":"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.numLightProbeGrids>0?"#define USE_LIGHT_PROBES_GRID":"",t.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",t.decodeVideoTextureEmissive?"#define DECODE_VIDEO_TEXTURE_EMISSIVE":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",t.toneMapping!==vi?"#define TONE_MAPPING":"",t.toneMapping!==vi?Dt.tonemapping_pars_fragment:"",t.toneMapping!==vi?i_("toneMapping",t.toneMapping):"",t.dithering?"#define DITHERING":"",t.opaque?"#define OPAQUE":"",Dt.colorspace_pars_fragment,t_("linearToOutputTexel",t.outputColorSpace),s_(),t.useDepthPacking?"#define DEPTH_PACKING "+t.depthPacking:"",`
`].filter(co).join(`
`)),a=cu(a),a=op(a,t),a=lp(a,t),o=cu(o),o=op(o,t),o=lp(o,t),a=cp(a),o=cp(o),t.isRawShaderMaterial!==!0&&(y=`#version 300 es
`,m=[p,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+m,f=["#define varying in",t.glslVersion===Xh?"":"layout(location = 0) out highp vec4 pc_fragColor;",t.glslVersion===Xh?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+f);let T=y+m+a,g=y+f+o,_=sp(i,i.VERTEX_SHADER,T),S=sp(i,i.FRAGMENT_SHADER,g);i.attachShader(v,_),i.attachShader(v,S),t.index0AttributeName!==void 0?i.bindAttribLocation(v,0,t.index0AttributeName):t.hasPositionAttribute===!0&&i.bindAttribLocation(v,0,"position"),i.linkProgram(v);function A(I){if(s.debug.checkShaderErrors){let M=i.getProgramInfoLog(v)||"",N=i.getShaderInfoLog(_)||"",D=i.getShaderInfoLog(S)||"",U=M.trim(),H=N.trim(),L=D.trim(),W=!0,z=!0;if(i.getProgramParameter(v,i.LINK_STATUS)===!1)if(W=!1,typeof s.debug.onShaderError=="function")s.debug.onShaderError(i,v,_,S);else{let E=ap(i,_,"vertex"),J=ap(i,S,"fragment");gt("WebGLProgram: Shader Error "+i.getError()+" - VALIDATE_STATUS "+i.getProgramParameter(v,i.VALIDATE_STATUS)+`

Material Name: `+I.name+`
Material Type: `+I.type+`

Program Info Log: `+U+`
`+E+`
`+J)}else U!==""?ht("WebGLProgram: Program Info Log:",U):(H===""||L==="")&&(z=!1);z&&(I.diagnostics={runnable:W,programLog:U,vertexShader:{log:H,prefix:m},fragmentShader:{log:L,prefix:f}})}i.deleteShader(_),i.deleteShader(S),b=new jr(i,v),C=o_(i,v)}let b;this.getUniforms=function(){return b===void 0&&A(this),b};let C;this.getAttributes=function(){return C===void 0&&A(this),C};let w=t.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return w===!1&&(w=i.getProgramParameter(v,Zv)),w},this.destroy=function(){n.releaseStatesOfProgram(this),i.deleteProgram(v),this.program=void 0},this.type=t.shaderType,this.name=t.shaderName,this.id=Qv++,this.cacheKey=e,this.usedTimes=1,this.program=v,this.vertexShader=_,this.fragmentShader=S,this}var w_=0,hu=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(e,t,n){let i=this._getShaderCacheForMaterial(e);return i.has(t)===!1&&(i.add(t),t.usedTimes++),i.has(n)===!1&&(i.add(n),n.usedTimes++),this}remove(e){let t=this.materialCache.get(e);for(let n of t)n.usedTimes--,n.usedTimes===0&&this.shaderCache.delete(n.code);return this.materialCache.delete(e),this}getVertexShaderStage(e){return this._getShaderStage(e.vertexShader)}getFragmentShaderStage(e){return this._getShaderStage(e.fragmentShader)}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(e){let t=this.materialCache,n=t.get(e);return n===void 0&&(n=new Set,t.set(e,n)),n}_getShaderStage(e){let t=this.shaderCache,n=t.get(e);return n===void 0&&(n=new uu(e),t.set(e,n)),n}},uu=class{constructor(e){this.id=w_++,this.code=e,this.usedTimes=0}};function S_(s){return s===ws||s===io||s===so}function E_(s,e,t,n,i,r){let a=new Sr,o=new hu,l=new Set,c=[],h=new Map,d=n.logarithmicDepthBuffer,u=n.precision,p={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distance",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function x(b){return l.add(b),b===0?"uv":`uv${b}`}function v(b,C,w,I,M,N){let D=I.fog,U=M.geometry,H=b.isMeshStandardMaterial||b.isMeshLambertMaterial||b.isMeshPhongMaterial?I.environment:null,L=b.isMeshStandardMaterial||b.isMeshLambertMaterial&&!b.envMap||b.isMeshPhongMaterial&&!b.envMap,W=e.get(b.envMap||H,L),z=W&&W.mapping===Qa?W.image.height:null,E=p[b.type];b.precision!==null&&(u=n.getMaxPrecision(b.precision),u!==b.precision&&ht("WebGLProgram.getParameters:",b.precision,"not supported, using",u,"instead."));let J=U.morphAttributes.position||U.morphAttributes.normal||U.morphAttributes.color,q=J!==void 0?J.length:0,te=0;U.morphAttributes.position!==void 0&&(te=1),U.morphAttributes.normal!==void 0&&(te=2),U.morphAttributes.color!==void 0&&(te=3);let Le,Fe,Ne,se;if(E){let P=Ui[E];Le=P.vertexShader,Fe=P.fragmentShader}else{Le=b.vertexShader,Fe=b.fragmentShader;let P=o.getVertexShaderStage(b),B=o.getFragmentShaderStage(b);o.update(b,P,B),Ne=P.id,se=B.id}let oe=s.getRenderTarget(),Pe=s.state.buffers.depth.getReversed(),ge=M.isInstancedMesh===!0,xe=M.isBatchedMesh===!0,$=!!b.map,ue=!!b.matcap,ce=!!W,ie=!!b.aoMap,de=!!b.lightMap,ve=!!b.bumpMap&&b.wireframe===!1,We=!!b.normalMap,Ve=!!b.displacementMap,lt=!!b.emissiveMap,nt=!!b.metalnessMap,ct=!!b.roughnessMap,Q=b.anisotropy>0,Pt=b.clearcoat>0,ft=b.dispersion>0,O=b.retroreflectivity>0,R=b.iridescence>0,re=b.sheen>0,le=b.transmission>0,be=Q&&!!b.anisotropyMap,De=Pt&&!!b.clearcoatMap,Ue=Pt&&!!b.clearcoatNormalMap,V=Pt&&!!b.clearcoatRoughnessMap,Y=R&&!!b.iridescenceMap,he=R&&!!b.iridescenceThicknessMap,_e=re&&!!b.sheenColorMap,me=re&&!!b.sheenRoughnessMap,ae=!!b.specularMap,Se=!!b.specularColorMap,Me=!!b.specularIntensityMap,Ie=le&&!!b.transmissionMap,j=le&&!!b.thicknessMap,Te=!!b.gradientMap,we=!!b.alphaMap,Oe=b.alphaTest>0,ke=!!b.alphaHash,Ae=!!b.extensions,Ye=vi;b.toneMapped&&(oe===null||oe.isXRRenderTarget===!0)&&(Ye=s.toneMapping);let Qe={shaderID:E,shaderType:b.type,shaderName:b.name,vertexShader:Le,fragmentShader:Fe,defines:b.defines,customVertexShaderID:Ne,customFragmentShaderID:se,isRawShaderMaterial:b.isRawShaderMaterial===!0,glslVersion:b.glslVersion,precision:u,batching:xe,batchingColor:xe&&M._colorsTexture!==null,instancing:ge,instancingColor:ge&&M.instanceColor!==null,instancingMorph:ge&&M.morphTexture!==null,outputColorSpace:oe===null?s.outputColorSpace:oe.isXRRenderTarget===!0?oe.texture.colorSpace:Rt.workingColorSpace,alphaToCoverage:!!b.alphaToCoverage,map:$,matcap:ue,envMap:ce,envMapMode:ce&&W.mapping,envMapCubeUVHeight:z,aoMap:ie,lightMap:de,bumpMap:ve,normalMap:We,displacementMap:Ve,emissiveMap:lt,normalMapObjectSpace:We&&b.normalMapType===Lf,normalMapTangentSpace:We&&b.normalMapType===ao,packedNormalMap:We&&b.normalMapType===ao&&S_(b.normalMap.format),metalnessMap:nt,roughnessMap:ct,anisotropy:Q,anisotropyMap:be,clearcoat:Pt,clearcoatMap:De,clearcoatNormalMap:Ue,clearcoatRoughnessMap:V,dispersion:ft,retroreflection:O,iridescence:R,iridescenceMap:Y,iridescenceThicknessMap:he,sheen:re,sheenColorMap:_e,sheenRoughnessMap:me,specularMap:ae,specularColorMap:Se,specularIntensityMap:Me,transmission:le,transmissionMap:Ie,thicknessMap:j,gradientMap:Te,opaque:b.transparent===!1&&b.blending===Br&&b.alphaToCoverage===!1,alphaMap:we,alphaTest:Oe,alphaHash:ke,combine:b.combine,mapUv:$&&x(b.map.channel),aoMapUv:ie&&x(b.aoMap.channel),lightMapUv:de&&x(b.lightMap.channel),bumpMapUv:ve&&x(b.bumpMap.channel),normalMapUv:We&&x(b.normalMap.channel),displacementMapUv:Ve&&x(b.displacementMap.channel),emissiveMapUv:lt&&x(b.emissiveMap.channel),metalnessMapUv:nt&&x(b.metalnessMap.channel),roughnessMapUv:ct&&x(b.roughnessMap.channel),anisotropyMapUv:be&&x(b.anisotropyMap.channel),clearcoatMapUv:De&&x(b.clearcoatMap.channel),clearcoatNormalMapUv:Ue&&x(b.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:V&&x(b.clearcoatRoughnessMap.channel),iridescenceMapUv:Y&&x(b.iridescenceMap.channel),iridescenceThicknessMapUv:he&&x(b.iridescenceThicknessMap.channel),sheenColorMapUv:_e&&x(b.sheenColorMap.channel),sheenRoughnessMapUv:me&&x(b.sheenRoughnessMap.channel),specularMapUv:ae&&x(b.specularMap.channel),specularColorMapUv:Se&&x(b.specularColorMap.channel),specularIntensityMapUv:Me&&x(b.specularIntensityMap.channel),transmissionMapUv:Ie&&x(b.transmissionMap.channel),thicknessMapUv:j&&x(b.thicknessMap.channel),alphaMapUv:we&&x(b.alphaMap.channel),vertexTangents:!!U.attributes.tangent&&(We||Q),vertexNormals:!!U.attributes.normal,vertexColors:b.vertexColors,vertexAlphas:b.vertexColors===!0&&!!U.attributes.color&&U.attributes.color.itemSize===4,pointsUvs:M.isPoints===!0&&!!U.attributes.uv&&($||we),fog:!!D,useFog:b.fog===!0,fogExp2:!!D&&D.isFogExp2,flatShading:b.wireframe===!1&&(b.flatShading===!0||U.attributes.normal===void 0&&We===!1&&(b.isMeshLambertMaterial||b.isMeshPhongMaterial||b.isMeshStandardMaterial||b.isMeshPhysicalMaterial)),sizeAttenuation:b.sizeAttenuation===!0,logarithmicDepthBuffer:d,reversedDepthBuffer:Pe,skinning:M.isSkinnedMesh===!0,hasPositionAttribute:U.attributes.position!==void 0,morphTargets:U.morphAttributes.position!==void 0,morphNormals:U.morphAttributes.normal!==void 0,morphColors:U.morphAttributes.color!==void 0,morphTargetsCount:q,morphTextureStride:te,numSunLights:C.sun.length,numDirLights:C.directional.length,numPointLights:C.point.length,numSpotLights:C.spot.length,numSpotLightMaps:C.spotLightMap.length,numRectAreaLights:C.rectArea.length,numHemiLights:C.hemi.length,numSunLightShadows:C.sunShadowMap.length,numDirLightShadows:C.directionalShadowMap.length,numPointLightShadows:C.pointShadowMap.length,numSpotLightShadows:C.spotShadowMap.length,numSpotLightShadowsWithMaps:C.numSpotLightShadowsWithMaps,numLightProbes:C.numLightProbes,numLightProbeGrids:N.length,numClippingPlanes:r.numPlanes,numClipIntersection:r.numIntersection,dithering:b.dithering,shadowMapEnabled:s.shadowMap.enabled&&w.length>0,shadowMapType:s.shadowMap.type,toneMapping:Ye,decodeVideoTexture:$&&b.map.isVideoTexture===!0&&Rt.getTransfer(b.map.colorSpace)===Wt,decodeVideoTextureEmissive:lt&&b.emissiveMap.isVideoTexture===!0&&Rt.getTransfer(b.emissiveMap.colorSpace)===Wt,premultipliedAlpha:b.premultipliedAlpha,doubleSided:b.side===Yt,flipSided:b.side===xn,useDepthPacking:b.depthPacking>=0,depthPacking:b.depthPacking||0,index0AttributeName:b.index0AttributeName,extensionClipCullDistance:Ae&&b.extensions.clipCullDistance===!0&&t.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(Ae&&b.extensions.multiDraw===!0||xe)&&t.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:t.has("KHR_parallel_shader_compile"),customProgramCacheKey:b.customProgramCacheKey()};return Qe.vertexUv1s=l.has(1),Qe.vertexUv2s=l.has(2),Qe.vertexUv3s=l.has(3),l.clear(),Qe}function m(b){let C=[];if(b.shaderID?C.push(b.shaderID):(C.push(b.customVertexShaderID),C.push(b.customFragmentShaderID)),b.defines!==void 0)for(let w in b.defines)C.push(w),C.push(b.defines[w]);return b.isRawShaderMaterial===!1&&(f(C,b),y(C,b),C.push(s.outputColorSpace)),C.push(b.customProgramCacheKey),C.join()}function f(b,C){b.push(C.precision),b.push(C.outputColorSpace),b.push(C.envMapMode),b.push(C.envMapCubeUVHeight),b.push(C.mapUv),b.push(C.alphaMapUv),b.push(C.lightMapUv),b.push(C.aoMapUv),b.push(C.bumpMapUv),b.push(C.normalMapUv),b.push(C.displacementMapUv),b.push(C.emissiveMapUv),b.push(C.metalnessMapUv),b.push(C.roughnessMapUv),b.push(C.anisotropyMapUv),b.push(C.clearcoatMapUv),b.push(C.clearcoatNormalMapUv),b.push(C.clearcoatRoughnessMapUv),b.push(C.iridescenceMapUv),b.push(C.iridescenceThicknessMapUv),b.push(C.sheenColorMapUv),b.push(C.sheenRoughnessMapUv),b.push(C.specularMapUv),b.push(C.specularColorMapUv),b.push(C.specularIntensityMapUv),b.push(C.transmissionMapUv),b.push(C.thicknessMapUv),b.push(C.combine),b.push(C.fogExp2),b.push(C.sizeAttenuation),b.push(C.morphTargetsCount),b.push(C.morphAttributeCount),b.push(C.numSunLights),b.push(C.numDirLights),b.push(C.numPointLights),b.push(C.numSpotLights),b.push(C.numSpotLightMaps),b.push(C.numHemiLights),b.push(C.numRectAreaLights),b.push(C.numSunLightShadows),b.push(C.numDirLightShadows),b.push(C.numPointLightShadows),b.push(C.numSpotLightShadows),b.push(C.numSpotLightShadowsWithMaps),b.push(C.numLightProbes),b.push(C.shadowMapType),b.push(C.toneMapping),b.push(C.numClippingPlanes),b.push(C.numClipIntersection),b.push(C.depthPacking)}function y(b,C){a.disableAll(),C.instancing&&a.enable(0),C.instancingColor&&a.enable(1),C.instancingMorph&&a.enable(2),C.matcap&&a.enable(3),C.envMap&&a.enable(4),C.normalMapObjectSpace&&a.enable(5),C.normalMapTangentSpace&&a.enable(6),C.clearcoat&&a.enable(7),C.iridescence&&a.enable(8),C.alphaTest&&a.enable(9),C.vertexColors&&a.enable(10),C.vertexAlphas&&a.enable(11),C.vertexUv1s&&a.enable(12),C.vertexUv2s&&a.enable(13),C.vertexUv3s&&a.enable(14),C.vertexTangents&&a.enable(15),C.anisotropy&&a.enable(16),C.alphaHash&&a.enable(17),C.batching&&a.enable(18),C.dispersion&&a.enable(19),C.retroreflection&&a.enable(24),C.batchingColor&&a.enable(20),C.gradientMap&&a.enable(21),C.packedNormalMap&&a.enable(22),C.vertexNormals&&a.enable(23),b.push(a.mask),a.disableAll(),C.fog&&a.enable(0),C.useFog&&a.enable(1),C.flatShading&&a.enable(2),C.logarithmicDepthBuffer&&a.enable(3),C.reversedDepthBuffer&&a.enable(4),C.skinning&&a.enable(5),C.morphTargets&&a.enable(6),C.morphNormals&&a.enable(7),C.morphColors&&a.enable(8),C.premultipliedAlpha&&a.enable(9),C.shadowMapEnabled&&a.enable(10),C.doubleSided&&a.enable(11),C.flipSided&&a.enable(12),C.useDepthPacking&&a.enable(13),C.dithering&&a.enable(14),C.transmission&&a.enable(15),C.sheen&&a.enable(16),C.opaque&&a.enable(17),C.pointsUvs&&a.enable(18),C.decodeVideoTexture&&a.enable(19),C.decodeVideoTextureEmissive&&a.enable(20),C.alphaToCoverage&&a.enable(21),C.numLightProbeGrids>0&&a.enable(22),C.hasPositionAttribute&&a.enable(23),b.push(a.mask)}function T(b){let C=p[b.type],w;if(C){let I=Ui[C];w=Mi.clone(I.uniforms)}else w=b.uniforms;return w}function g(b,C){let w=h.get(C);return w!==void 0?++w.usedTimes:(w=new M_(s,C,b,i),c.push(w),h.set(C,w)),w}function _(b){if(--b.usedTimes===0){let C=c.indexOf(b);c[C]=c[c.length-1],c.pop(),h.delete(b.cacheKey),b.destroy()}}function S(b){o.remove(b)}function A(){o.dispose()}return{getParameters:v,getProgramCacheKey:m,getUniforms:T,acquireProgram:g,releaseProgram:_,releaseShaderCache:S,programs:c,dispose:A}}function T_(){let s=new WeakMap;function e(a){return s.has(a)}function t(a){let o=s.get(a);return o===void 0&&(o={},s.set(a,o)),o}function n(a){s.delete(a)}function i(a,o,l){s.get(a)[o]=l}function r(){s=new WeakMap}return{has:e,get:t,remove:n,update:i,dispose:r}}function A_(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.material.id!==e.material.id?s.material.id-e.material.id:s.materialVariant!==e.materialVariant?s.materialVariant-e.materialVariant:s.z!==e.z?s.z-e.z:s.id-e.id}function up(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.z!==e.z?e.z-s.z:s.id-e.id}function dp(){let s=[],e=0,t=[],n=[],i=[];function r(){e=0,t.length=0,n.length=0,i.length=0}function a(u){let p=0;return u.isInstancedMesh&&(p+=2),u.isSkinnedMesh&&(p+=1),p}function o(u,p,x,v,m,f){let y=s[e];return y===void 0?(y={id:u.id,object:u,geometry:p,material:x,materialVariant:a(u),groupOrder:v,renderOrder:u.renderOrder,z:m,group:f},s[e]=y):(y.id=u.id,y.object=u,y.geometry=p,y.material=x,y.materialVariant=a(u),y.groupOrder=v,y.renderOrder=u.renderOrder,y.z=m,y.group=f),e++,y}function l(u,p,x,v,m,f,y){y.reversedDepth===!0&&(m=-m);let T=o(u,p,x,v,m,f);x.transmission>0?n.push(T):x.transparent===!0?i.push(T):t.push(T)}function c(u,p,x,v,m,f){let y=o(u,p,x,v,m,f);x.transmission>0?n.unshift(y):x.transparent===!0?i.unshift(y):t.unshift(y)}function h(u,p){t.length>1&&t.sort(u||A_),n.length>1&&n.sort(p||up),i.length>1&&i.sort(p||up)}function d(){for(let u=e,p=s.length;u<p;u++){let x=s[u];if(x.id===null)break;x.id=null,x.object=null,x.geometry=null,x.material=null,x.group=null}}return{opaque:t,transmissive:n,transparent:i,init:r,push:l,unshift:c,finish:d,sort:h}}function R_(){let s=new WeakMap;function e(n,i){let r=s.get(n),a;return r===void 0?(a=new dp,s.set(n,[a])):i>=r.length?(a=new dp,r.push(a)):a=r[i],a}function t(){s=new WeakMap}return{get:e,dispose:t}}function C_(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"SunLight":case"DirectionalLight":t={direction:new k,color:new Ce};break;case"SpotLight":t={position:new k,direction:new k,color:new Ce,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":t={position:new k,color:new Ce,distance:0,decay:0};break;case"HemisphereLight":t={direction:new k,skyColor:new Ce,groundColor:new Ce};break;case"RectAreaLight":t={color:new Ce,position:new k,halfWidth:new k,halfHeight:new k};break}return s[e.id]=t,t}}}function P_(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"SunLight":case"DirectionalLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new ze};break;case"SpotLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new ze};break;case"PointLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new ze,shadowCameraNear:1,shadowCameraFar:1e3};break}return s[e.id]=t,t}}}var I_=0;function D_(s,e){return(e.castShadow?2:0)-(s.castShadow?2:0)+(e.map?1:0)-(s.map?1:0)}function L_(s){let e=new C_,t=P_(),n={version:0,hash:{sunLength:-1,directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numSunShadows:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],sun:[],sunShadow:[],sunShadowMap:[],sunShadowMatrix:[],sunShadowCascade:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let c=0;c<9;c++)n.probe.push(new k);let i=new k,r=new mt,a=new mt;function o(c){let h=0,d=0,u=0;for(let M=0;M<9;M++)n.probe[M].set(0,0,0);let p=0,x=0,v=0,m=0,f=0,y=0,T=0,g=0,_=0,S=0,A=0,b=0,C=0,w=0;c.sort(D_);for(let M=0,N=c.length;M<N;M++){let D=c[M],U=D.color,H=D.intensity,L=D.distance,W=null;if(D.shadow&&D.shadow.map&&(D.shadow.map.texture.format===ws?W=D.shadow.map.texture:W=D.shadow.map.depthTexture||D.shadow.map.texture),D.isAmbientLight)h+=U.r*H,d+=U.g*H,u+=U.b*H;else if(D.isLightProbe){for(let z=0;z<9;z++)n.probe[z].addScaledVector(D.sh.coefficients[z],H);w++}else if(D.isSunLight){let z=e.get(D);if(z.color.copy(D.color).multiplyScalar(D.intensity),D.castShadow){let E=D.shadow,J=t.get(D);J.shadowIntensity=E.intensity,J.shadowBias=E.bias,J.shadowNormalBias=E.normalBias,J.shadowRadius=E.radius,J.shadowMapSize.copy(E.mapSize).multiply(E.getFrameExtents()),n.sunShadow[x]=J,n.sunShadowMap[x]=W;let q=E.getViewportCount();for(let te=0;te<q;te++)n.sunShadowMatrix[v+te]=E.getMatrix(te),n.sunShadowCascade[v+te]=E._cascadeData[te];v+=q,x++}n.sun[p]=z,p++}else if(D.isDirectionalLight){let z=e.get(D);if(z.color.copy(D.color).multiplyScalar(D.intensity),D.castShadow){let E=D.shadow,J=t.get(D);J.shadowIntensity=E.intensity,J.shadowBias=E.bias,J.shadowNormalBias=E.normalBias,J.shadowRadius=E.radius,J.shadowMapSize=E.mapSize,n.directionalShadow[m]=J,n.directionalShadowMap[m]=W,n.directionalShadowMatrix[m]=D.shadow.matrix,_++}n.directional[m]=z,m++}else if(D.isSpotLight){let z=e.get(D);z.position.setFromMatrixPosition(D.matrixWorld),z.color.copy(U).multiplyScalar(H),z.distance=L,z.coneCos=Math.cos(D.angle),z.penumbraCos=Math.cos(D.angle*(1-D.penumbra)),z.decay=D.decay,n.spot[y]=z;let E=D.shadow;if(D.map&&(n.spotLightMap[b]=D.map,b++,E.updateMatrices(D),D.castShadow&&C++),n.spotLightMatrix[y]=E.matrix,D.castShadow){let J=t.get(D);J.shadowIntensity=E.intensity,J.shadowBias=E.bias,J.shadowNormalBias=E.normalBias,J.shadowRadius=E.radius,J.shadowMapSize=E.mapSize,n.spotShadow[y]=J,n.spotShadowMap[y]=W,A++}y++}else if(D.isRectAreaLight){let z=e.get(D);z.color.copy(U).multiplyScalar(H),z.halfWidth.set(D.width*.5,0,0),z.halfHeight.set(0,D.height*.5,0),n.rectArea[T]=z,T++}else if(D.isPointLight){let z=e.get(D);if(z.color.copy(D.color).multiplyScalar(D.intensity),z.distance=D.distance,z.decay=D.decay,D.castShadow){let E=D.shadow,J=t.get(D);J.shadowIntensity=E.intensity,J.shadowBias=E.bias,J.shadowNormalBias=E.normalBias,J.shadowRadius=E.radius,J.shadowMapSize=E.mapSize,J.shadowCameraNear=E.camera.near,J.shadowCameraFar=E.camera.far,n.pointShadow[f]=J,n.pointShadowMap[f]=W,n.pointShadowMatrix[f]=D.shadow.matrix,S++}n.point[f]=z,f++}else if(D.isHemisphereLight){let z=e.get(D);z.skyColor.copy(D.color).multiplyScalar(H),z.groundColor.copy(D.groundColor).multiplyScalar(H),n.hemi[g]=z,g++}}T>0&&(s.has("OES_texture_float_linear")===!0?(n.rectAreaLTC1=Ke.LTC_FLOAT_1,n.rectAreaLTC2=Ke.LTC_FLOAT_2):(n.rectAreaLTC1=Ke.LTC_HALF_1,n.rectAreaLTC2=Ke.LTC_HALF_2)),n.ambient[0]=h,n.ambient[1]=d,n.ambient[2]=u;let I=n.hash;(I.sunLength!==p||I.directionalLength!==m||I.pointLength!==f||I.spotLength!==y||I.rectAreaLength!==T||I.hemiLength!==g||I.numSunShadows!==x||I.numDirectionalShadows!==_||I.numPointShadows!==S||I.numSpotShadows!==A||I.numSpotMaps!==b||I.numLightProbes!==w)&&(n.sun.length=p,n.directional.length=m,n.spot.length=y,n.rectArea.length=T,n.point.length=f,n.hemi.length=g,n.sunShadow.length=x,n.sunShadowMap.length=x,n.sunShadowMatrix.length=v,n.sunShadowCascade.length=v,n.directionalShadow.length=_,n.directionalShadowMap.length=_,n.directionalShadowMatrix.length=_,n.pointShadow.length=S,n.pointShadowMap.length=S,n.pointShadowMatrix.length=S,n.spotShadow.length=A,n.spotShadowMap.length=A,n.spotLightMatrix.length=A+b-C,n.spotLightMap.length=b,n.numSpotLightShadowsWithMaps=C,n.numLightProbes=w,I.sunLength=p,I.directionalLength=m,I.pointLength=f,I.spotLength=y,I.rectAreaLength=T,I.hemiLength=g,I.numSunShadows=x,I.numDirectionalShadows=_,I.numPointShadows=S,I.numSpotShadows=A,I.numSpotMaps=b,I.numLightProbes=w,n.version=I_++)}function l(c,h){let d=0,u=0,p=0,x=0,v=0,m=0,f=h.matrixWorldInverse;for(let y=0,T=c.length;y<T;y++){let g=c[y];if(g.isSunLight){let _=n.sun[d];_.direction.setFromMatrixPosition(g.matrixWorld),_.direction.transformDirection(f),d++}else if(g.isDirectionalLight){let _=n.directional[u];_.direction.setFromMatrixPosition(g.matrixWorld),i.setFromMatrixPosition(g.target.matrixWorld),_.direction.sub(i),_.direction.transformDirection(f),u++}else if(g.isSpotLight){let _=n.spot[x];_.position.setFromMatrixPosition(g.matrixWorld),_.position.applyMatrix4(f),_.direction.setFromMatrixPosition(g.matrixWorld),i.setFromMatrixPosition(g.target.matrixWorld),_.direction.sub(i),_.direction.transformDirection(f),x++}else if(g.isRectAreaLight){let _=n.rectArea[v];_.position.setFromMatrixPosition(g.matrixWorld),_.position.applyMatrix4(f),a.identity(),r.copy(g.matrixWorld),r.premultiply(f),a.extractRotation(r),_.halfWidth.set(g.width*.5,0,0),_.halfHeight.set(0,g.height*.5,0),_.halfWidth.applyMatrix4(a),_.halfHeight.applyMatrix4(a),v++}else if(g.isPointLight){let _=n.point[p];_.position.setFromMatrixPosition(g.matrixWorld),_.position.applyMatrix4(f),p++}else if(g.isHemisphereLight){let _=n.hemi[m];_.direction.setFromMatrixPosition(g.matrixWorld),_.direction.transformDirection(f),m++}}}return{setup:o,setupView:l,state:n}}function fp(s){let e=new L_(s),t=[],n=[],i=[];function r(u){d.camera=u,t.length=0,n.length=0,i.length=0}function a(u){t.push(u)}function o(u){n.push(u)}function l(u){i.push(u)}function c(){e.setup(t)}function h(u){e.setupView(t,u)}let d={lightsArray:t,shadowsArray:n,lightProbeGridArray:i,camera:null,lights:e,transmissionRenderTarget:{},textureUnits:0};return{init:r,state:d,setupLights:c,setupLightsView:h,pushLight:a,pushShadow:o,pushLightProbeGrid:l}}function F_(s){let e=new WeakMap;function t(i,r=0){let a=e.get(i),o;return a===void 0?(o=new fp(s),e.set(i,[o])):r>=a.length?(o=new fp(s),a.push(o)):o=a[r],o}function n(){e=new WeakMap}return{get:t,dispose:n}}var N_=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,U_=`uniform sampler2D shadow_pass;
uniform vec2 resolution;
uniform float radius;
void main() {
	const float samples = float( VSM_SAMPLES );
	float mean = 0.0;
	float squared_mean = 0.0;
	float uvStride = samples <= 1.0 ? 0.0 : 2.0 / ( samples - 1.0 );
	float uvStart = samples <= 1.0 ? 0.0 : - 1.0;
	for ( float i = 0.0; i < samples; i ++ ) {
		float uvOffset = uvStart + i * uvStride;
		#ifdef HORIZONTAL_PASS
			vec2 distribution = texture2D( shadow_pass, ( gl_FragCoord.xy + vec2( uvOffset, 0.0 ) * radius ) / resolution ).rg;
			mean += distribution.x;
			squared_mean += distribution.y * distribution.y + distribution.x * distribution.x;
		#else
			float depth = texture2D( shadow_pass, ( gl_FragCoord.xy + vec2( 0.0, uvOffset ) * radius ) / resolution ).r;
			mean += depth;
			squared_mean += depth * depth;
		#endif
	}
	mean = mean / samples;
	squared_mean = squared_mean / samples;
	float std_dev = sqrt( max( 0.0, squared_mean - mean * mean ) );
	gl_FragColor = vec4( mean, std_dev, 0.0, 1.0 );
}`,O_=[new k(1,0,0),new k(-1,0,0),new k(0,1,0),new k(0,-1,0),new k(0,0,1),new k(0,0,-1)],z_=[new k(0,-1,0),new k(0,-1,0),new k(0,0,1),new k(0,0,-1),new k(0,-1,0),new k(0,-1,0)],pp=new mt,lo=new k,su=new k;function k_(s,e,t){let n=new Cr,i=new ze,r=new ze,a=new Nt,o=new xl,l=new bl,c={},h=t.maxTextureSize,d={[ii]:xn,[xn]:ii,[Yt]:Yt},u=new wt({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new ze},radius:{value:4}},vertexShader:N_,fragmentShader:U_}),p=u.clone();p.defines.HORIZONTAL_PASS=1;let x=new Mt;x.setAttribute("position",new Tt(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let v=new ot(x,u),m=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=qs;let f=this.type;this.render=function(S,A,b){if(m.enabled===!1||m.autoUpdate===!1&&m.needsUpdate===!1||S.length===0)return;this.type===of&&(ht("WebGLShadowMap: PCFSoftShadowMap has been removed. Using PCFShadowMap instead."),this.type=qs);let C=s.getRenderTarget(),w=s.getActiveCubeFace(),I=s.getActiveMipmapLevel(),M=s.state;M.setBlending(si),M.buffers.depth.getReversed()===!0?M.buffers.color.setClear(0,0,0,0):M.buffers.color.setClear(1,1,1,1),M.buffers.depth.setTest(!0),M.setScissorTest(!1);let N=f!==this.type;N&&A.traverse(function(D){D.material&&(Array.isArray(D.material)?D.material.forEach(U=>U.needsUpdate=!0):D.material.needsUpdate=!0)});for(let D=0,U=S.length;D<U;D++){let H=S[D],L=H.shadow;if(L===void 0){ht("WebGLShadowMap:",H,"has no shadow.");continue}if(L.autoUpdate===!1&&L.needsUpdate===!1)continue;i.copy(L.mapSize);let W=L.getFrameExtents();i.multiply(W),r.copy(L.mapSize),(i.x>h||i.y>h)&&(i.x>h&&(r.x=Math.floor(h/W.x),i.x=r.x*W.x,L.mapSize.x=r.x),i.y>h&&(r.y=Math.floor(h/W.y),i.y=r.y*W.y,L.mapSize.y=r.y));let z=s.state.buffers.depth.getReversed();if(L.camera._reversedDepth=z,L.map===null||N===!0){if(L.map!==null&&(L.map.depthTexture!==null&&(L.map.depthTexture.dispose(),L.map.depthTexture=null),L.map.dispose()),this.type===kr){if(H.isPointLight){ht("WebGLShadowMap: VSM shadow maps are not supported for PointLights. Use PCF or BasicShadowMap instead.");continue}L.map=new an(i.x,i.y,{format:ws,type:gn,minFilter:rn,magFilter:rn,generateMipmaps:!1}),L.map.texture.name=H.name+".shadowMap",L.map.depthTexture=new ms(i.x,i.y,Zn),L.map.depthTexture.name=H.name+".shadowMapDepth",L.map.depthTexture.format=Ri,L.map.depthTexture.compareFunction=null,L.map.depthTexture.minFilter=ln,L.map.depthTexture.magFilter=ln}else H.isPointLight?(L.map=new Mc(i.x),L.map.depthTexture=new cl(i.x,yi)):(L.map=new an(i.x,i.y),L.map.depthTexture=new ms(i.x,i.y,yi)),L.map.depthTexture.name=H.name+".shadowMap",L.map.depthTexture.format=Ri,this.type===qs?(L.map.depthTexture.compareFunction=z?vc:bc,L.map.depthTexture.minFilter=rn,L.map.depthTexture.magFilter=rn):(L.map.depthTexture.compareFunction=null,L.map.depthTexture.minFilter=ln,L.map.depthTexture.magFilter=ln);L.camera.updateProjectionMatrix()}L.map.isWebGLCubeRenderTarget!==!0&&(L.map.width!==i.x||L.map.height!==i.y)&&L.map.setSize(i.x,i.y);let E=L.map.isWebGLCubeRenderTarget?6:L.getViewportCount();H.isPointLight!==!0&&L.updateMatrices(H,b);for(let J=0;J<E;J++){let q=L.getCamera(J);if(H.isPointLight){let te=L.camera,Le=L.matrix,Fe=H.distance||te.far;Fe!==te.far&&(te.far=Fe,te.updateProjectionMatrix()),lo.setFromMatrixPosition(H.matrixWorld),te.position.copy(lo),su.copy(te.position),su.add(O_[J]),te.up.copy(z_[J]),te.lookAt(su),te.updateMatrixWorld(),Le.makeTranslation(-lo.x,-lo.y,-lo.z),pp.multiplyMatrices(te.projectionMatrix,te.matrixWorldInverse),L._frustum.setFromProjectionMatrix(pp,te.coordinateSystem,te.reversedDepth)}if(L.map.isWebGLCubeRenderTarget)s.setRenderTarget(L.map,J),s.clear();else{J===0&&(s.setRenderTarget(L.map),s.clear());let te=L.getViewport(J);a.set(r.x*te.x,r.y*te.y,r.x*te.z,r.y*te.w),M.viewport(a)}n=L.getFrustum(J),g(A,b,q,H,this.type)}L.isPointLightShadow!==!0&&this.type===kr&&y(L,b),L.needsUpdate=!1}f=this.type,m.needsUpdate=!1,s.setRenderTarget(C,w,I)};function y(S,A){let b=e.update(v);u.defines.VSM_SAMPLES!==S.blurSamples&&(u.defines.VSM_SAMPLES=S.blurSamples,p.defines.VSM_SAMPLES=S.blurSamples,u.needsUpdate=!0,p.needsUpdate=!0),S.mapPass===null?S.mapPass=new an(i.x,i.y,{format:ws,type:gn}):(S.mapPass.width!==S.map.width||S.mapPass.height!==S.map.height)&&S.mapPass.setSize(S.map.width,S.map.height),u.uniforms.shadow_pass.value=S.map.depthTexture,u.uniforms.resolution.value.set(S.map.width,S.map.height),u.uniforms.radius.value=S.radius,s.setRenderTarget(S.mapPass),s.clear(),s.renderBufferDirect(A,null,b,u,v,null),p.uniforms.shadow_pass.value=S.mapPass.texture,p.uniforms.resolution.value.set(S.map.width,S.map.height),p.uniforms.radius.value=S.radius,s.setRenderTarget(S.map),s.clear(),s.renderBufferDirect(A,null,b,p,v,null)}function T(S,A,b,C){let w=null,I=b.isPointLight===!0?S.customDistanceMaterial:S.customDepthMaterial;if(I!==void 0)w=I;else if(w=b.isPointLight===!0?l:o,s.localClippingEnabled&&A.clipShadows===!0&&Array.isArray(A.clippingPlanes)&&A.clippingPlanes.length!==0||A.displacementMap&&A.displacementScale!==0||A.alphaMap&&A.alphaTest>0||A.map&&A.alphaTest>0||A.alphaToCoverage===!0){let M=w.uuid,N=A.uuid,D=c[M];D===void 0&&(D={},c[M]=D);let U=D[N];U===void 0&&(U=w.clone(),D[N]=U,A.addEventListener("dispose",_)),w=U}if(w.visible=A.visible,w.wireframe=A.wireframe,C===kr?w.side=A.shadowSide!==null?A.shadowSide:A.side:w.side=A.shadowSide!==null?A.shadowSide:d[A.side],w.alphaMap=A.alphaMap,w.alphaTest=A.alphaToCoverage===!0?.5:A.alphaTest,w.map=A.map,w.clipShadows=A.clipShadows,w.clippingPlanes=A.clippingPlanes,w.clipIntersection=A.clipIntersection,w.displacementMap=A.displacementMap,w.displacementScale=A.displacementScale,w.displacementBias=A.displacementBias,w.wireframeLinewidth=A.wireframeLinewidth,w.linewidth=A.linewidth,b.isPointLight===!0&&w.isMeshDistanceMaterial===!0){let M=s.properties.get(w);M.light=b}return w}function g(S,A,b,C,w){if(S.visible===!1)return;if(S.layers.test(A.layers)&&(S.isMesh||S.isLine||S.isPoints)&&(S.castShadow||S.receiveShadow&&w===kr)&&(!S.frustumCulled||S.intersectsFrustum(n))){S.modelViewMatrix.multiplyMatrices(b.matrixWorldInverse,S.matrixWorld);let N=e.update(S),D=S.material;if(Array.isArray(D)){let U=N.groups;for(let H=0,L=U.length;H<L;H++){let W=U[H],z=D[W.materialIndex];if(z&&z.visible){let E=T(S,z,C,w);S.onBeforeShadow(s,S,A,b,N,E,W),s.renderBufferDirect(b,null,N,E,S,W),S.onAfterShadow(s,S,A,b,N,E,W)}}}else if(D.visible){let U=T(S,D,C,w);S.onBeforeShadow(s,S,A,b,N,U,null),s.renderBufferDirect(b,null,N,U,S,null),S.onAfterShadow(s,S,A,b,N,U,null)}}let M=S.children;for(let N=0,D=M.length;N<D;N++)g(M[N],A,b,C,w)}function _(S){S.target.removeEventListener("dispose",_);for(let b in c){let C=c[b],w=S.target.uuid;w in C&&(C[w].dispose(),delete C[w])}}}function B_(s,e){function t(){let j=!1,Te=new Nt,we=null,Oe=new Nt(0,0,0,0);return{setMask:function(ke){we!==ke&&!j&&(s.colorMask(ke,ke,ke,ke),we=ke)},setLocked:function(ke){j=ke},setClear:function(ke,Ae,Ye,Qe,P){P===!0&&(ke*=Qe,Ae*=Qe,Ye*=Qe),Te.set(ke,Ae,Ye,Qe),Oe.equals(Te)===!1&&(s.clearColor(ke,Ae,Ye,Qe),Oe.copy(Te))},reset:function(){j=!1,we=null,Oe.set(-1,0,0,0)}}}function n(){let j=!1,Te=!1,we=null,Oe=null,ke=null;return{setReversed:function(Ae){if(Te!==Ae){let Ye=e.get("EXT_clip_control");Ae?Ye.clipControlEXT(Ye.LOWER_LEFT_EXT,Ye.ZERO_TO_ONE_EXT):Ye.clipControlEXT(Ye.LOWER_LEFT_EXT,Ye.NEGATIVE_ONE_TO_ONE_EXT),Te=Ae;let Qe=ke;ke=null,this.setClear(Qe)}},getReversed:function(){return Te},setTest:function(Ae){Ae?oe(s.DEPTH_TEST):Pe(s.DEPTH_TEST)},setMask:function(Ae){we!==Ae&&!j&&(s.depthMask(Ae),we=Ae)},setFunc:function(Ae){if(Te&&(Ae=Wf[Ae]),Oe!==Ae){switch(Ae){case Zo:s.depthFunc(s.NEVER);break;case Qo:s.depthFunc(s.ALWAYS);break;case $o:s.depthFunc(s.LESS);break;case br:s.depthFunc(s.LEQUAL);break;case el:s.depthFunc(s.EQUAL);break;case tl:s.depthFunc(s.GEQUAL);break;case nl:s.depthFunc(s.GREATER);break;case il:s.depthFunc(s.NOTEQUAL);break;default:s.depthFunc(s.LEQUAL)}Oe=Ae}},setLocked:function(Ae){j=Ae},setClear:function(Ae){ke!==Ae&&(ke=Ae,Te&&(Ae=1-Ae),s.clearDepth(Ae))},reset:function(){j=!1,we=null,Oe=null,ke=null,Te=!1}}}function i(){let j=!1,Te=null,we=null,Oe=null,ke=null,Ae=null,Ye=null,Qe=null,P=null;return{setTest:function(B){j||(B?oe(s.STENCIL_TEST):Pe(s.STENCIL_TEST))},setMask:function(B){Te!==B&&!j&&(s.stencilMask(B),Te=B)},setFunc:function(B,K,ee){(we!==B||Oe!==K||ke!==ee)&&(s.stencilFunc(B,K,ee),we=B,Oe=K,ke=ee)},setOp:function(B,K,ee){(Ae!==B||Ye!==K||Qe!==ee)&&(s.stencilOp(B,K,ee),Ae=B,Ye=K,Qe=ee)},setLocked:function(B){j=B},setClear:function(B){P!==B&&(s.clearStencil(B),P=B)},reset:function(){j=!1,Te=null,we=null,Oe=null,ke=null,Ae=null,Ye=null,Qe=null,P=null}}}let r=new t,a=new n,o=new i,l=new WeakMap,c=new WeakMap,h={},d={},u={},p=new WeakMap,x=[],v=null,m=!1,f=null,y=null,T=null,g=null,_=null,S=null,A=null,b=new Ce(0,0,0),C=0,w=!1,I=null,M=null,N=null,D=null,U=null,H=s.getParameter(s.MAX_COMBINED_TEXTURE_IMAGE_UNITS),L=!1,W=0,z=s.getParameter(s.VERSION);z.indexOf("WebGL")!==-1?(W=parseFloat(/^WebGL (\d)/.exec(z)[1]),L=W>=1):z.indexOf("OpenGL ES")!==-1&&(W=parseFloat(/^OpenGL ES (\d)/.exec(z)[1]),L=W>=2);let E=null,J={},q=s.getParameter(s.SCISSOR_BOX),te=s.getParameter(s.VIEWPORT),Le=new Nt().fromArray(q),Fe=new Nt().fromArray(te);function Ne(j,Te,we,Oe){let ke=new Uint8Array(4),Ae=s.createTexture();s.bindTexture(j,Ae),s.texParameteri(j,s.TEXTURE_MIN_FILTER,s.NEAREST),s.texParameteri(j,s.TEXTURE_MAG_FILTER,s.NEAREST);for(let Ye=0;Ye<we;Ye++)j===s.TEXTURE_3D||j===s.TEXTURE_2D_ARRAY?s.texImage3D(Te,0,s.RGBA,1,1,Oe,0,s.RGBA,s.UNSIGNED_BYTE,ke):s.texImage2D(Te+Ye,0,s.RGBA,1,1,0,s.RGBA,s.UNSIGNED_BYTE,ke);return Ae}let se={};se[s.TEXTURE_2D]=Ne(s.TEXTURE_2D,s.TEXTURE_2D,1),se[s.TEXTURE_CUBE_MAP]=Ne(s.TEXTURE_CUBE_MAP,s.TEXTURE_CUBE_MAP_POSITIVE_X,6),se[s.TEXTURE_2D_ARRAY]=Ne(s.TEXTURE_2D_ARRAY,s.TEXTURE_2D_ARRAY,1,1),se[s.TEXTURE_3D]=Ne(s.TEXTURE_3D,s.TEXTURE_3D,1,1),r.setClear(0,0,0,1),a.setClear(1),o.setClear(0),oe(s.DEPTH_TEST),a.setFunc(br),ve(!1),We(Dh),oe(s.CULL_FACE),ie(si);function oe(j){h[j]!==!0&&(s.enable(j),h[j]=!0)}function Pe(j){h[j]!==!1&&(s.disable(j),h[j]=!1)}function ge(j,Te){return u[j]!==Te?(s.bindFramebuffer(j,Te),u[j]=Te,j===s.DRAW_FRAMEBUFFER&&(u[s.FRAMEBUFFER]=Te),j===s.FRAMEBUFFER&&(u[s.DRAW_FRAMEBUFFER]=Te),!0):!1}function xe(j,Te){let we=x,Oe=!1;if(j){we=p.get(Te),we===void 0&&(we=[],p.set(Te,we));let ke=j.textures;if(we.length!==ke.length||we[0]!==s.COLOR_ATTACHMENT0){for(let Ae=0,Ye=ke.length;Ae<Ye;Ae++)we[Ae]=s.COLOR_ATTACHMENT0+Ae;we.length=ke.length,Oe=!0}}else we[0]!==s.BACK&&(we[0]=s.BACK,Oe=!0);Oe&&s.drawBuffers(we)}function $(j){return v!==j?(s.useProgram(j),v=j,!0):!1}let ue={[Xs]:s.FUNC_ADD,[cf]:s.FUNC_SUBTRACT,[hf]:s.FUNC_REVERSE_SUBTRACT};ue[uf]=s.MIN,ue[df]=s.MAX;let ce={[ff]:s.ZERO,[pf]:s.ONE,[mf]:s.SRC_COLOR,[Nh]:s.SRC_ALPHA,[yf]:s.SRC_ALPHA_SATURATE,[vf]:s.DST_COLOR,[xf]:s.DST_ALPHA,[gf]:s.ONE_MINUS_SRC_COLOR,[Uh]:s.ONE_MINUS_SRC_ALPHA,[_f]:s.ONE_MINUS_DST_COLOR,[bf]:s.ONE_MINUS_DST_ALPHA,[Mf]:s.CONSTANT_COLOR,[wf]:s.ONE_MINUS_CONSTANT_COLOR,[Sf]:s.CONSTANT_ALPHA,[Ef]:s.ONE_MINUS_CONSTANT_ALPHA};function ie(j,Te,we,Oe,ke,Ae,Ye,Qe,P,B){if(j===si){m===!0&&(Pe(s.BLEND),m=!1);return}if(m===!1&&(oe(s.BLEND),m=!0),j!==lf){if(j!==f||B!==w){if((y!==Xs||_!==Xs)&&(s.blendEquation(s.FUNC_ADD),y=Xs,_=Xs),B)switch(j){case Br:s.blendFuncSeparate(s.ONE,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case tn:s.blendFunc(s.ONE,s.ONE);break;case Lh:s.blendFuncSeparate(s.ZERO,s.ONE_MINUS_SRC_COLOR,s.ZERO,s.ONE);break;case Fh:s.blendFuncSeparate(s.DST_COLOR,s.ONE_MINUS_SRC_ALPHA,s.ZERO,s.ONE);break;default:gt("WebGLState: Invalid blending: ",j);break}else switch(j){case Br:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case tn:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE,s.ONE,s.ONE);break;case Lh:gt("WebGLState: SubtractiveBlending requires material.premultipliedAlpha = true");break;case Fh:gt("WebGLState: MultiplyBlending requires material.premultipliedAlpha = true");break;default:gt("WebGLState: Invalid blending: ",j);break}T=null,g=null,S=null,A=null,b.set(0,0,0),C=0,f=j,w=B}return}ke=ke||Te,Ae=Ae||we,Ye=Ye||Oe,(Te!==y||ke!==_)&&(s.blendEquationSeparate(ue[Te],ue[ke]),y=Te,_=ke),(we!==T||Oe!==g||Ae!==S||Ye!==A)&&(s.blendFuncSeparate(ce[we],ce[Oe],ce[Ae],ce[Ye]),T=we,g=Oe,S=Ae,A=Ye),(Qe.equals(b)===!1||P!==C)&&(s.blendColor(Qe.r,Qe.g,Qe.b,P),b.copy(Qe),C=P),f=j,w=!1}function de(j,Te){j.side===Yt?Pe(s.CULL_FACE):oe(s.CULL_FACE);let we=j.side===xn;Te&&(we=!we),ve(we),j.blending===Br&&j.transparent===!1?ie(si):ie(j.blending,j.blendEquation,j.blendSrc,j.blendDst,j.blendEquationAlpha,j.blendSrcAlpha,j.blendDstAlpha,j.blendColor,j.blendAlpha,j.premultipliedAlpha),a.setFunc(j.depthFunc),a.setTest(j.depthTest),a.setMask(j.depthWrite),r.setMask(j.colorWrite);let Oe=j.stencilWrite;o.setTest(Oe),Oe&&(o.setMask(j.stencilWriteMask),o.setFunc(j.stencilFunc,j.stencilRef,j.stencilFuncMask),o.setOp(j.stencilFail,j.stencilZFail,j.stencilZPass)),lt(j.polygonOffset,j.polygonOffsetFactor,j.polygonOffsetUnits),j.alphaToCoverage===!0?oe(s.SAMPLE_ALPHA_TO_COVERAGE):Pe(s.SAMPLE_ALPHA_TO_COVERAGE)}function ve(j){I!==j&&(j?s.frontFace(s.CW):s.frontFace(s.CCW),I=j)}function We(j){j!==rf?(oe(s.CULL_FACE),j!==M&&(j===Dh?s.cullFace(s.BACK):j===af?s.cullFace(s.FRONT):s.cullFace(s.FRONT_AND_BACK))):Pe(s.CULL_FACE),M=j}function Ve(j){j!==N&&(L&&s.lineWidth(j),N=j)}function lt(j,Te,we){j?(oe(s.POLYGON_OFFSET_FILL),(D!==Te||U!==we)&&(D=Te,U=we,a.getReversed()&&(Te=-Te),s.polygonOffset(Te,we))):Pe(s.POLYGON_OFFSET_FILL)}function nt(j){j?oe(s.SCISSOR_TEST):Pe(s.SCISSOR_TEST)}function ct(j){j===void 0&&(j=s.TEXTURE0+H-1),E!==j&&(s.activeTexture(j),E=j)}function Q(j,Te,we){we===void 0&&(E===null?we=s.TEXTURE0+H-1:we=E);let Oe=J[we];Oe===void 0&&(Oe={type:void 0,texture:void 0},J[we]=Oe),(Oe.type!==j||Oe.texture!==Te)&&(E!==we&&(s.activeTexture(we),E=we),s.bindTexture(j,Te||se[j]),Oe.type=j,Oe.texture=Te)}function Pt(){let j=J[E];j!==void 0&&j.type!==void 0&&(s.bindTexture(j.type,null),j.type=void 0,j.texture=void 0)}function ft(){try{s.compressedTexImage2D(...arguments)}catch(j){gt("WebGLState:",j)}}function O(){try{s.compressedTexImage3D(...arguments)}catch(j){gt("WebGLState:",j)}}function R(){try{s.texSubImage2D(...arguments)}catch(j){gt("WebGLState:",j)}}function re(){try{s.texSubImage3D(...arguments)}catch(j){gt("WebGLState:",j)}}function le(){try{s.compressedTexSubImage2D(...arguments)}catch(j){gt("WebGLState:",j)}}function be(){try{s.compressedTexSubImage3D(...arguments)}catch(j){gt("WebGLState:",j)}}function De(){try{s.texStorage2D(...arguments)}catch(j){gt("WebGLState:",j)}}function Ue(){try{s.texStorage3D(...arguments)}catch(j){gt("WebGLState:",j)}}function V(){try{s.texImage2D(...arguments)}catch(j){gt("WebGLState:",j)}}function Y(){try{s.texImage3D(...arguments)}catch(j){gt("WebGLState:",j)}}function he(j){return d[j]!==void 0?d[j]:s.getParameter(j)}function _e(j,Te){d[j]!==Te&&(s.pixelStorei(j,Te),d[j]=Te)}function me(j){Le.equals(j)===!1&&(s.scissor(j.x,j.y,j.z,j.w),Le.copy(j))}function ae(j){Fe.equals(j)===!1&&(s.viewport(j.x,j.y,j.z,j.w),Fe.copy(j))}function Se(j,Te){let we=c.get(Te);we===void 0&&(we=new WeakMap,c.set(Te,we));let Oe=we.get(j);Oe===void 0&&(Oe=s.getUniformBlockIndex(Te,j.name),we.set(j,Oe))}function Me(j,Te){let Oe=c.get(Te).get(j);l.get(Te)!==Oe&&(s.uniformBlockBinding(Te,Oe,j.__bindingPointIndex),l.set(Te,Oe))}function Ie(){s.disable(s.BLEND),s.disable(s.CULL_FACE),s.disable(s.DEPTH_TEST),s.disable(s.POLYGON_OFFSET_FILL),s.disable(s.SCISSOR_TEST),s.disable(s.STENCIL_TEST),s.disable(s.SAMPLE_ALPHA_TO_COVERAGE),s.blendEquation(s.FUNC_ADD),s.blendFunc(s.ONE,s.ZERO),s.blendFuncSeparate(s.ONE,s.ZERO,s.ONE,s.ZERO),s.blendColor(0,0,0,0),s.colorMask(!0,!0,!0,!0),s.clearColor(0,0,0,0),s.depthMask(!0),s.depthFunc(s.LESS),a.setReversed(!1),s.clearDepth(1),s.stencilMask(4294967295),s.stencilFunc(s.ALWAYS,0,4294967295),s.stencilOp(s.KEEP,s.KEEP,s.KEEP),s.clearStencil(0),s.cullFace(s.BACK),s.frontFace(s.CCW),s.polygonOffset(0,0),s.activeTexture(s.TEXTURE0),s.bindFramebuffer(s.FRAMEBUFFER,null),s.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),s.bindFramebuffer(s.READ_FRAMEBUFFER,null),s.useProgram(null),s.lineWidth(1),s.scissor(0,0,s.canvas.width,s.canvas.height),s.viewport(0,0,s.canvas.width,s.canvas.height),s.pixelStorei(s.PACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,!1),s.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!1),s.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,s.BROWSER_DEFAULT_WEBGL),s.pixelStorei(s.PACK_ROW_LENGTH,0),s.pixelStorei(s.PACK_SKIP_PIXELS,0),s.pixelStorei(s.PACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_ROW_LENGTH,0),s.pixelStorei(s.UNPACK_IMAGE_HEIGHT,0),s.pixelStorei(s.UNPACK_SKIP_PIXELS,0),s.pixelStorei(s.UNPACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_SKIP_IMAGES,0),h={},d={},E=null,J={},u={},p=new WeakMap,x=[],v=null,m=!1,f=null,y=null,T=null,g=null,_=null,S=null,A=null,b=new Ce(0,0,0),C=0,w=!1,I=null,M=null,N=null,D=null,U=null,Le.set(0,0,s.canvas.width,s.canvas.height),Fe.set(0,0,s.canvas.width,s.canvas.height),r.reset(),a.reset(),o.reset()}return{buffers:{color:r,depth:a,stencil:o},enable:oe,disable:Pe,bindFramebuffer:ge,drawBuffers:xe,useProgram:$,setBlending:ie,setMaterial:de,setFlipSided:ve,setCullFace:We,setLineWidth:Ve,setPolygonOffset:lt,setScissorTest:nt,activeTexture:ct,bindTexture:Q,unbindTexture:Pt,compressedTexImage2D:ft,compressedTexImage3D:O,texImage2D:V,texImage3D:Y,pixelStorei:_e,getParameter:he,updateUBOMapping:Se,uniformBlockBinding:Me,texStorage2D:De,texStorage3D:Ue,texSubImage2D:R,texSubImage3D:re,compressedTexSubImage2D:le,compressedTexSubImage3D:be,scissor:me,viewport:ae,reset:Ie}}function H_(s,e,t,n,i,r,a){let o=e.has("WEBGL_multisampled_render_to_texture")?e.get("WEBGL_multisampled_render_to_texture"):null,l=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),c=new ze,h=new WeakMap,d=new Set,u,p=new WeakMap,x=!1;try{x=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function v(O,R){return x?new OffscreenCanvas(O,R):yr("canvas")}function m(O,R,re){let le=1,be=ft(O);if((be.width>re||be.height>re)&&(le=re/Math.max(be.width,be.height)),le<1)if(typeof HTMLImageElement<"u"&&O instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&O instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&O instanceof ImageBitmap||typeof VideoFrame<"u"&&O instanceof VideoFrame){let De=Math.floor(le*be.width),Ue=Math.floor(le*be.height);u===void 0&&(u=v(De,Ue));let V=R?v(De,Ue):u;return V.width=De,V.height=Ue,V.getContext("2d").drawImage(O,0,0,De,Ue),ht("WebGLRenderer: Texture has been resized from ("+be.width+"x"+be.height+") to ("+De+"x"+Ue+")."),V}else return"data"in O&&ht("WebGLRenderer: Image in DataTexture is too big ("+be.width+"x"+be.height+")."),O;return O}function f(O){return O.generateMipmaps}function y(O){s.generateMipmap(O)}function T(O){return O.isWebGLCubeRenderTarget?s.TEXTURE_CUBE_MAP:O.isWebGL3DRenderTarget?s.TEXTURE_3D:O.isWebGLArrayRenderTarget||O.isCompressedArrayTexture?s.TEXTURE_2D_ARRAY:s.TEXTURE_2D}function g(O,R,re,le,be,De=!1){if(O!==null){if(s[O]!==void 0)return s[O];ht("WebGLRenderer: Attempt to use non-existing WebGL internal format '"+O+"'")}let Ue;le&&(Ue=e.get("EXT_texture_norm16"),Ue||ht("WebGLRenderer: Unable to use normalized textures without EXT_texture_norm16 extension"));let V=R;if(R===s.RED&&(re===s.FLOAT&&(V=s.R32F),re===s.HALF_FLOAT&&(V=s.R16F),re===s.UNSIGNED_BYTE&&(V=s.R8),re===s.UNSIGNED_SHORT&&Ue&&(V=Ue.R16_EXT),re===s.SHORT&&Ue&&(V=Ue.R16_SNORM_EXT)),R===s.RED_INTEGER&&(re===s.UNSIGNED_BYTE&&(V=s.R8UI),re===s.UNSIGNED_SHORT&&(V=s.R16UI),re===s.UNSIGNED_INT&&(V=s.R32UI),re===s.BYTE&&(V=s.R8I),re===s.SHORT&&(V=s.R16I),re===s.INT&&(V=s.R32I)),R===s.RG&&(re===s.FLOAT&&(V=s.RG32F),re===s.HALF_FLOAT&&(V=s.RG16F),re===s.UNSIGNED_BYTE&&(V=s.RG8),re===s.UNSIGNED_SHORT&&Ue&&(V=Ue.RG16_EXT),re===s.SHORT&&Ue&&(V=Ue.RG16_SNORM_EXT)),R===s.RG_INTEGER&&(re===s.UNSIGNED_BYTE&&(V=s.RG8UI),re===s.UNSIGNED_SHORT&&(V=s.RG16UI),re===s.UNSIGNED_INT&&(V=s.RG32UI),re===s.BYTE&&(V=s.RG8I),re===s.SHORT&&(V=s.RG16I),re===s.INT&&(V=s.RG32I)),R===s.RGB_INTEGER&&(re===s.UNSIGNED_BYTE&&(V=s.RGB8UI),re===s.UNSIGNED_SHORT&&(V=s.RGB16UI),re===s.UNSIGNED_INT&&(V=s.RGB32UI),re===s.BYTE&&(V=s.RGB8I),re===s.SHORT&&(V=s.RGB16I),re===s.INT&&(V=s.RGB32I)),R===s.RGBA_INTEGER&&(re===s.UNSIGNED_BYTE&&(V=s.RGBA8UI),re===s.UNSIGNED_SHORT&&(V=s.RGBA16UI),re===s.UNSIGNED_INT&&(V=s.RGBA32UI),re===s.BYTE&&(V=s.RGBA8I),re===s.SHORT&&(V=s.RGBA16I),re===s.INT&&(V=s.RGBA32I)),R===s.RGB&&(re===s.UNSIGNED_SHORT&&Ue&&(V=Ue.RGB16_EXT),re===s.SHORT&&Ue&&(V=Ue.RGB16_SNORM_EXT),re===s.UNSIGNED_INT_5_9_9_9_REV&&(V=s.RGB9_E5),re===s.UNSIGNED_INT_10F_11F_11F_REV&&(V=s.R11F_G11F_B10F)),R===s.RGBA){let Y=De?va:Rt.getTransfer(be);re===s.FLOAT&&(V=s.RGBA32F),re===s.HALF_FLOAT&&(V=s.RGBA16F),re===s.UNSIGNED_BYTE&&(V=Y===Wt?s.SRGB8_ALPHA8:s.RGBA8),re===s.UNSIGNED_SHORT&&Ue&&(V=Ue.RGBA16_EXT),re===s.SHORT&&Ue&&(V=Ue.RGBA16_SNORM_EXT),re===s.UNSIGNED_SHORT_4_4_4_4&&(V=s.RGBA4),re===s.UNSIGNED_SHORT_5_5_5_1&&(V=s.RGB5_A1)}return(V===s.R16F||V===s.R32F||V===s.RG16F||V===s.RG32F||V===s.RGBA16F||V===s.RGBA32F)&&e.get("EXT_color_buffer_float"),V}function _(O,R){let re;return O?R===null||R===yi||R===Vr?re=s.DEPTH24_STENCIL8:R===Zn?re=s.DEPTH32F_STENCIL8:R===Gr&&(re=s.DEPTH24_STENCIL8,ht("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):R===null||R===yi||R===Vr?re=s.DEPTH_COMPONENT24:R===Zn?re=s.DEPTH_COMPONENT32F:R===Gr&&(re=s.DEPTH_COMPONENT16),re}function S(O,R){return f(O)===!0||O.isFramebufferTexture&&O.minFilter!==ln&&O.minFilter!==rn?Math.log2(Math.max(R.width,R.height))+1:O.mipmaps!==void 0&&O.mipmaps.length>0?O.mipmaps.length:O.isCompressedTexture&&Array.isArray(O.image)?R.mipmaps.length:1}function A(O){let R=O.target;R.removeEventListener("dispose",A),C(R),R.isVideoTexture&&h.delete(R),R.isHTMLTexture&&d.delete(R)}function b(O){let R=O.target;R.removeEventListener("dispose",b),I(R)}function C(O){let R=n.get(O);if(R.__webglInit===void 0)return;let re=O.source,le=p.get(re);if(le){let be=le[R.__cacheKey];be.usedTimes--,be.usedTimes===0&&w(O),Object.keys(le).length===0&&p.delete(re)}n.remove(O)}function w(O){let R=n.get(O);s.deleteTexture(R.__webglTexture);let re=O.source,le=p.get(re);delete le[R.__cacheKey],a.memory.textures--}function I(O){let R=n.get(O);if(O.depthTexture&&(O.depthTexture.dispose(),n.remove(O.depthTexture)),O.isWebGLCubeRenderTarget)for(let le=0;le<6;le++){if(Array.isArray(R.__webglFramebuffer[le]))for(let be=0;be<R.__webglFramebuffer[le].length;be++)s.deleteFramebuffer(R.__webglFramebuffer[le][be]);else s.deleteFramebuffer(R.__webglFramebuffer[le]);R.__webglDepthbuffer&&s.deleteRenderbuffer(R.__webglDepthbuffer[le])}else{if(Array.isArray(R.__webglFramebuffer))for(let le=0;le<R.__webglFramebuffer.length;le++)s.deleteFramebuffer(R.__webglFramebuffer[le]);else s.deleteFramebuffer(R.__webglFramebuffer);if(R.__webglDepthbuffer&&s.deleteRenderbuffer(R.__webglDepthbuffer),R.__webglMultisampledFramebuffer&&s.deleteFramebuffer(R.__webglMultisampledFramebuffer),R.__webglColorRenderbuffer)for(let le=0;le<R.__webglColorRenderbuffer.length;le++)R.__webglColorRenderbuffer[le]&&s.deleteRenderbuffer(R.__webglColorRenderbuffer[le]);R.__webglDepthRenderbuffer&&s.deleteRenderbuffer(R.__webglDepthRenderbuffer)}let re=O.textures;for(let le=0,be=re.length;le<be;le++){let De=n.get(re[le]);De.__webglTexture&&(s.deleteTexture(De.__webglTexture),a.memory.textures--),n.remove(re[le])}n.remove(O)}let M=0;function N(){M=0}function D(){return M}function U(O){M=O}function H(){let O=M;return O>=i.maxTextures&&ht("WebGLTextures: Trying to use "+(O+1)+" texture units while this GPU supports only "+i.maxTextures),M+=1,O}function L(O){let R=[];return R.push(O.wrapS),R.push(O.wrapT),R.push(O.wrapR||0),R.push(O.magFilter),R.push(O.minFilter),R.push(O.anisotropy),R.push(O.internalFormat),R.push(O.format),R.push(O.type),R.push(O.generateMipmaps),R.push(O.premultiplyAlpha),R.push(O.flipY),R.push(O.unpackAlignment),R.push(O.colorSpace),R.join()}function W(O,R){let re=n.get(O);if(O.isVideoTexture&&Q(O),O.isRenderTargetTexture===!1&&O.isExternalTexture!==!0&&O.version>0&&re.__version!==O.version){let le=O.image;if(le===null)ht("WebGLRenderer: Texture marked for update but no image data found.");else if(le.complete===!1)ht("WebGLRenderer: Texture marked for update but image is incomplete");else{Pe(re,O,R);return}}else O.isExternalTexture&&(re.__webglTexture=O.sourceTexture?O.sourceTexture:null);t.bindTexture(s.TEXTURE_2D,re.__webglTexture,s.TEXTURE0+R)}function z(O,R){let re=n.get(O);if(O.isRenderTargetTexture===!1&&O.version>0&&re.__version!==O.version){Pe(re,O,R);return}else O.isExternalTexture&&(re.__webglTexture=O.sourceTexture?O.sourceTexture:null);t.bindTexture(s.TEXTURE_2D_ARRAY,re.__webglTexture,s.TEXTURE0+R)}function E(O,R){let re=n.get(O);if(O.isRenderTargetTexture===!1&&O.version>0&&re.__version!==O.version){Pe(re,O,R);return}t.bindTexture(s.TEXTURE_3D,re.__webglTexture,s.TEXTURE0+R)}function J(O,R){let re=n.get(O);if(O.isCubeDepthTexture!==!0&&O.version>0&&re.__version!==O.version){ge(re,O,R);return}t.bindTexture(s.TEXTURE_CUBE_MAP,re.__webglTexture,s.TEXTURE0+R)}let q={[Ai]:s.REPEAT,[ti]:s.CLAMP_TO_EDGE,[vr]:s.MIRRORED_REPEAT},te={[ln]:s.NEAREST,[Dl]:s.NEAREST_MIPMAP_NEAREST,[Ys]:s.NEAREST_MIPMAP_LINEAR,[rn]:s.LINEAR,[Hr]:s.LINEAR_MIPMAP_NEAREST,[_i]:s.LINEAR_MIPMAP_LINEAR},Le={[Nf]:s.NEVER,[Bf]:s.ALWAYS,[Uf]:s.LESS,[bc]:s.LEQUAL,[Of]:s.EQUAL,[vc]:s.GEQUAL,[zf]:s.GREATER,[kf]:s.NOTEQUAL};function Fe(O,R){if(R.type===Zn&&e.has("OES_texture_float_linear")===!1&&(R.magFilter===rn||R.magFilter===Hr||R.magFilter===Ys||R.magFilter===_i||R.minFilter===rn||R.minFilter===Hr||R.minFilter===Ys||R.minFilter===_i)&&ht("WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),s.texParameteri(O,s.TEXTURE_WRAP_S,q[R.wrapS]),s.texParameteri(O,s.TEXTURE_WRAP_T,q[R.wrapT]),(O===s.TEXTURE_3D||O===s.TEXTURE_2D_ARRAY)&&s.texParameteri(O,s.TEXTURE_WRAP_R,q[R.wrapR]),s.texParameteri(O,s.TEXTURE_MAG_FILTER,te[R.magFilter]),s.texParameteri(O,s.TEXTURE_MIN_FILTER,te[R.minFilter]),R.compareFunction&&(s.texParameteri(O,s.TEXTURE_COMPARE_MODE,s.COMPARE_REF_TO_TEXTURE),s.texParameteri(O,s.TEXTURE_COMPARE_FUNC,Le[R.compareFunction])),e.has("EXT_texture_filter_anisotropic")===!0){if(R.magFilter===ln||R.minFilter!==Ys&&R.minFilter!==_i||R.type===Zn&&e.has("OES_texture_float_linear")===!1)return;if(R.anisotropy>1||n.get(R).__currentAnisotropy){let re=e.get("EXT_texture_filter_anisotropic");s.texParameterf(O,re.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(R.anisotropy,i.getMaxAnisotropy())),n.get(R).__currentAnisotropy=R.anisotropy}}}function Ne(O,R){let re=!1;O.__webglInit===void 0&&(O.__webglInit=!0,R.addEventListener("dispose",A));let le=R.source,be=p.get(le);be===void 0&&(be={},p.set(le,be));let De=L(R);if(De!==O.__cacheKey){be[De]===void 0&&(be[De]={texture:s.createTexture(),usedTimes:0},a.memory.textures++,re=!0),be[De].usedTimes++;let Ue=be[O.__cacheKey];Ue!==void 0&&(be[O.__cacheKey].usedTimes--,Ue.usedTimes===0&&w(R)),O.__cacheKey=De,O.__webglTexture=be[De].texture}return re}function se(O,R,re){return Math.floor(Math.floor(O/re)/R)}function oe(O,R,re,le){let De=O.updateRanges;if(De.length===0)t.texSubImage2D(s.TEXTURE_2D,0,0,0,R.width,R.height,re,le,R.data);else{De.sort((_e,me)=>_e.start-me.start);let Ue=0;for(let _e=1;_e<De.length;_e++){let me=De[Ue],ae=De[_e],Se=me.start+me.count,Me=se(ae.start,R.width,4),Ie=se(me.start,R.width,4);ae.start<=Se+1&&Me===Ie&&se(ae.start+ae.count-1,R.width,4)===Me?me.count=Math.max(me.count,ae.start+ae.count-me.start):(++Ue,De[Ue]=ae)}De.length=Ue+1;let V=t.getParameter(s.UNPACK_ROW_LENGTH),Y=t.getParameter(s.UNPACK_SKIP_PIXELS),he=t.getParameter(s.UNPACK_SKIP_ROWS);t.pixelStorei(s.UNPACK_ROW_LENGTH,R.width);for(let _e=0,me=De.length;_e<me;_e++){let ae=De[_e],Se=Math.floor(ae.start/4),Me=Math.ceil(ae.count/4),Ie=Se%R.width,j=Math.floor(Se/R.width),Te=Me,we=1;t.pixelStorei(s.UNPACK_SKIP_PIXELS,Ie),t.pixelStorei(s.UNPACK_SKIP_ROWS,j),t.texSubImage2D(s.TEXTURE_2D,0,Ie,j,Te,we,re,le,R.data)}O.clearUpdateRanges(),t.pixelStorei(s.UNPACK_ROW_LENGTH,V),t.pixelStorei(s.UNPACK_SKIP_PIXELS,Y),t.pixelStorei(s.UNPACK_SKIP_ROWS,he)}}function Pe(O,R,re){let le=s.TEXTURE_2D;(R.isDataArrayTexture||R.isCompressedArrayTexture)&&(le=s.TEXTURE_2D_ARRAY),R.isData3DTexture&&(le=s.TEXTURE_3D);let be=Ne(O,R),De=R.source;t.bindTexture(le,O.__webglTexture,s.TEXTURE0+re);let Ue=n.get(De);if(De.version!==Ue.__version||be===!0){if(t.activeTexture(s.TEXTURE0+re),(typeof ImageBitmap<"u"&&R.image instanceof ImageBitmap)===!1){let we=Rt.getPrimaries(Rt.workingColorSpace),Oe=R.colorSpace===ri?null:Rt.getPrimaries(R.colorSpace),ke=R.colorSpace===ri||we===Oe?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,R.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,R.premultiplyAlpha),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,ke)}t.pixelStorei(s.UNPACK_ALIGNMENT,R.unpackAlignment);let Y=m(R.image,!1,i.maxTextureSize);Y=Pt(R,Y);let he=r.convert(R.format,R.colorSpace),_e=r.convert(R.type),me=g(R.internalFormat,he,_e,R.normalized,R.colorSpace,R.isVideoTexture);Fe(le,R);let ae,Se=R.mipmaps,Me=R.isVideoTexture!==!0,Ie=Ue.__version===void 0||be===!0,j=De.dataReady,Te=S(R,Y);if(R.isDepthTexture)me=_(R.format===Ms,R.type),Ie&&(Me?t.texStorage2D(s.TEXTURE_2D,1,me,Y.width,Y.height):t.texImage2D(s.TEXTURE_2D,0,me,Y.width,Y.height,0,he,_e,null));else if(R.isDataTexture)if(Se.length>0){Me&&Ie&&t.texStorage2D(s.TEXTURE_2D,Te,me,Se[0].width,Se[0].height);for(let we=0,Oe=Se.length;we<Oe;we++)ae=Se[we],Me?j&&t.texSubImage2D(s.TEXTURE_2D,we,0,0,ae.width,ae.height,he,_e,ae.data):t.texImage2D(s.TEXTURE_2D,we,me,ae.width,ae.height,0,he,_e,ae.data);R.generateMipmaps=!1}else Me?(Ie&&t.texStorage2D(s.TEXTURE_2D,Te,me,Y.width,Y.height),j&&oe(R,Y,he,_e)):t.texImage2D(s.TEXTURE_2D,0,me,Y.width,Y.height,0,he,_e,Y.data);else if(R.isCompressedTexture)if(R.isCompressedArrayTexture){Me&&Ie&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Te,me,Se[0].width,Se[0].height,Y.depth);for(let we=0,Oe=Se.length;we<Oe;we++)if(ae=Se[we],R.format!==Qn)if(he!==null)if(Me){if(j)if(R.layerUpdates.size>0){let ke=Qh(ae.width,ae.height,R.format,R.type);for(let Ae of R.layerUpdates){let Ye=ae.data.subarray(Ae*ke/ae.data.BYTES_PER_ELEMENT,(Ae+1)*ke/ae.data.BYTES_PER_ELEMENT);t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,we,0,0,Ae,ae.width,ae.height,1,he,Ye)}}else t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,we,0,0,0,ae.width,ae.height,Y.depth,he,ae.data)}else t.compressedTexImage3D(s.TEXTURE_2D_ARRAY,we,me,ae.width,ae.height,Y.depth,0,ae.data,0,0);else ht("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else Me?j&&t.texSubImage3D(s.TEXTURE_2D_ARRAY,we,0,0,0,ae.width,ae.height,Y.depth,he,_e,ae.data):t.texImage3D(s.TEXTURE_2D_ARRAY,we,me,ae.width,ae.height,Y.depth,0,he,_e,ae.data);R.layerUpdates.size>0&&R.clearLayerUpdates()}else{Me&&Ie&&t.texStorage2D(s.TEXTURE_2D,Te,me,Se[0].width,Se[0].height);for(let we=0,Oe=Se.length;we<Oe;we++)ae=Se[we],R.format!==Qn?he!==null?Me?j&&t.compressedTexSubImage2D(s.TEXTURE_2D,we,0,0,ae.width,ae.height,he,ae.data):t.compressedTexImage2D(s.TEXTURE_2D,we,me,ae.width,ae.height,0,ae.data):ht("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):Me?j&&t.texSubImage2D(s.TEXTURE_2D,we,0,0,ae.width,ae.height,he,_e,ae.data):t.texImage2D(s.TEXTURE_2D,we,me,ae.width,ae.height,0,he,_e,ae.data)}else if(R.isDataArrayTexture)if(Me){if(Ie&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Te,me,Y.width,Y.height,Y.depth),j)if(R.layerUpdates.size>0){let we=Qh(Y.width,Y.height,R.format,R.type);for(let Oe of R.layerUpdates){let ke=Y.data.subarray(Oe*we/Y.data.BYTES_PER_ELEMENT,(Oe+1)*we/Y.data.BYTES_PER_ELEMENT);t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,Oe,Y.width,Y.height,1,he,_e,ke)}R.clearLayerUpdates()}else t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,0,Y.width,Y.height,Y.depth,he,_e,Y.data)}else t.texImage3D(s.TEXTURE_2D_ARRAY,0,me,Y.width,Y.height,Y.depth,0,he,_e,Y.data);else if(R.isData3DTexture)Me?(Ie&&t.texStorage3D(s.TEXTURE_3D,Te,me,Y.width,Y.height,Y.depth),j&&t.texSubImage3D(s.TEXTURE_3D,0,0,0,0,Y.width,Y.height,Y.depth,he,_e,Y.data)):t.texImage3D(s.TEXTURE_3D,0,me,Y.width,Y.height,Y.depth,0,he,_e,Y.data);else if(R.isFramebufferTexture){if(Ie)if(Me)t.texStorage2D(s.TEXTURE_2D,Te,me,Y.width,Y.height);else{let we=Y.width,Oe=Y.height;for(let ke=0;ke<Te;ke++)t.texImage2D(s.TEXTURE_2D,ke,me,we,Oe,0,he,_e,null),we>>=1,Oe>>=1}}else if(R.isHTMLTexture){if("texElementImage2D"in s){let we=s.canvas;if(we.hasAttribute("layoutsubtree")||we.setAttribute("layoutsubtree","true"),Y.parentNode!==we){we.appendChild(Y),d.add(R),we.onpaint=Oe=>{let ke=Oe.changedElements;for(let Ae of d)ke.includes(Ae.image)&&(Ae.needsUpdate=!0)},we.requestPaint();return}if(s.texElementImage2D.length===3)s.texElementImage2D(s.TEXTURE_2D,s.RGBA8,Y);else{let ke=s.RGBA,Ae=s.RGBA,Ye=s.UNSIGNED_BYTE;s.texElementImage2D(s.TEXTURE_2D,0,ke,Ae,Ye,Y)}s.texParameteri(s.TEXTURE_2D,s.TEXTURE_MIN_FILTER,s.LINEAR),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_S,s.CLAMP_TO_EDGE),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_T,s.CLAMP_TO_EDGE)}}else if(Se.length>0){if(Me&&Ie){let we=ft(Se[0]);t.texStorage2D(s.TEXTURE_2D,Te,me,we.width,we.height)}for(let we=0,Oe=Se.length;we<Oe;we++)ae=Se[we],Me?j&&t.texSubImage2D(s.TEXTURE_2D,we,0,0,he,_e,ae):t.texImage2D(s.TEXTURE_2D,we,me,he,_e,ae);R.generateMipmaps=!1}else if(Me){if(Ie){let we=ft(Y);t.texStorage2D(s.TEXTURE_2D,Te,me,we.width,we.height)}j&&t.texSubImage2D(s.TEXTURE_2D,0,0,0,he,_e,Y)}else t.texImage2D(s.TEXTURE_2D,0,me,he,_e,Y);f(R)&&y(le),Ue.__version=De.version,R.onUpdate&&R.onUpdate(R)}O.__version=R.version}function ge(O,R,re){if(R.image.length!==6)return;let le=Ne(O,R),be=R.source;t.bindTexture(s.TEXTURE_CUBE_MAP,O.__webglTexture,s.TEXTURE0+re);let De=n.get(be);if(be.version!==De.__version||le===!0){t.activeTexture(s.TEXTURE0+re);let Ue=Rt.getPrimaries(Rt.workingColorSpace),V=R.colorSpace===ri?null:Rt.getPrimaries(R.colorSpace),Y=R.colorSpace===ri||Ue===V?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,R.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,R.premultiplyAlpha),t.pixelStorei(s.UNPACK_ALIGNMENT,R.unpackAlignment),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,Y);let he=R.isCompressedTexture||R.image[0].isCompressedTexture,_e=R.image[0]&&R.image[0].isDataTexture,me=[];for(let Ae=0;Ae<6;Ae++)!he&&!_e?me[Ae]=m(R.image[Ae],!0,i.maxCubemapSize):me[Ae]=_e?R.image[Ae].image:R.image[Ae],me[Ae]=Pt(R,me[Ae]);let ae=me[0],Se=r.convert(R.format,R.colorSpace),Me=r.convert(R.type),Ie=g(R.internalFormat,Se,Me,R.normalized,R.colorSpace),j=R.isVideoTexture!==!0,Te=De.__version===void 0||le===!0,we=be.dataReady,Oe=S(R,ae);Fe(s.TEXTURE_CUBE_MAP,R);let ke;if(he){j&&Te&&t.texStorage2D(s.TEXTURE_CUBE_MAP,Oe,Ie,ae.width,ae.height);for(let Ae=0;Ae<6;Ae++){ke=me[Ae].mipmaps;for(let Ye=0;Ye<ke.length;Ye++){let Qe=ke[Ye];R.format!==Qn?Se!==null?j?we&&t.compressedTexSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,Ye,0,0,Qe.width,Qe.height,Se,Qe.data):t.compressedTexImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,Ye,Ie,Qe.width,Qe.height,0,Qe.data):ht("WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):j?we&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,Ye,0,0,Qe.width,Qe.height,Se,Me,Qe.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,Ye,Ie,Qe.width,Qe.height,0,Se,Me,Qe.data)}}}else{if(ke=R.mipmaps,j&&Te){ke.length>0&&Oe++;let Ae=ft(me[0]);t.texStorage2D(s.TEXTURE_CUBE_MAP,Oe,Ie,Ae.width,Ae.height)}for(let Ae=0;Ae<6;Ae++)if(_e){j?we&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,0,0,0,me[Ae].width,me[Ae].height,Se,Me,me[Ae].data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,0,Ie,me[Ae].width,me[Ae].height,0,Se,Me,me[Ae].data);for(let Ye=0;Ye<ke.length;Ye++){let P=ke[Ye].image[Ae].image;j?we&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,Ye+1,0,0,P.width,P.height,Se,Me,P.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,Ye+1,Ie,P.width,P.height,0,Se,Me,P.data)}}else{j?we&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,0,0,0,Se,Me,me[Ae]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,0,Ie,Se,Me,me[Ae]);for(let Ye=0;Ye<ke.length;Ye++){let Qe=ke[Ye];j?we&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,Ye+1,0,0,Se,Me,Qe.image[Ae]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ae,Ye+1,Ie,Se,Me,Qe.image[Ae])}}}f(R)&&y(s.TEXTURE_CUBE_MAP),De.__version=be.version,R.onUpdate&&R.onUpdate(R)}O.__version=R.version}function xe(O,R,re,le,be,De){let Ue=r.convert(re.format,re.colorSpace),V=r.convert(re.type),Y=g(re.internalFormat,Ue,V,re.normalized,re.colorSpace),he=n.get(R),_e=n.get(re);if(_e.__renderTarget=R,!he.__hasExternalTextures){let me=Math.max(1,R.width>>De),ae=Math.max(1,R.height>>De);be===s.TEXTURE_3D||be===s.TEXTURE_2D_ARRAY?t.texImage3D(be,De,Y,me,ae,R.depth,0,Ue,V,null):t.texImage2D(be,De,Y,me,ae,0,Ue,V,null)}t.bindFramebuffer(s.FRAMEBUFFER,O),ct(R)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,le,be,_e.__webglTexture,0,nt(R)):(be===s.TEXTURE_2D||be>=s.TEXTURE_CUBE_MAP_POSITIVE_X&&be<=s.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&s.framebufferTexture2D(s.FRAMEBUFFER,le,be,_e.__webglTexture,De),t.bindFramebuffer(s.FRAMEBUFFER,null)}function $(O,R,re){if(s.bindRenderbuffer(s.RENDERBUFFER,O),R.depthBuffer){let le=R.depthTexture,be=le&&le.isDepthTexture?le.type:null,De=_(R.stencilBuffer,be),Ue=R.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;ct(R)?o.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,nt(R),De,R.width,R.height):re?s.renderbufferStorageMultisample(s.RENDERBUFFER,nt(R),De,R.width,R.height):s.renderbufferStorage(s.RENDERBUFFER,De,R.width,R.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,Ue,s.RENDERBUFFER,O)}else{let le=R.textures;for(let be=0;be<le.length;be++){let De=le[be],Ue=r.convert(De.format,De.colorSpace),V=r.convert(De.type),Y=g(De.internalFormat,Ue,V,De.normalized,De.colorSpace);ct(R)?o.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,nt(R),Y,R.width,R.height):re?s.renderbufferStorageMultisample(s.RENDERBUFFER,nt(R),Y,R.width,R.height):s.renderbufferStorage(s.RENDERBUFFER,Y,R.width,R.height)}}s.bindRenderbuffer(s.RENDERBUFFER,null)}function ue(O,R,re){let le=R.isWebGLCubeRenderTarget===!0;if(t.bindFramebuffer(s.FRAMEBUFFER,O),!(R.depthTexture&&R.depthTexture.isDepthTexture))throw new Error("THREE.WebGLTextures: renderTarget.depthTexture must be an instance of THREE.DepthTexture.");let be=n.get(R.depthTexture);if(be.__renderTarget=R,(!be.__webglTexture||R.depthTexture.image.width!==R.width||R.depthTexture.image.height!==R.height)&&(R.depthTexture.image.width=R.width,R.depthTexture.image.height=R.height,R.depthTexture.needsUpdate=!0),le){if(be.__webglInit===void 0&&(be.__webglInit=!0,R.depthTexture.addEventListener("dispose",A)),be.__webglTexture===void 0){be.__webglTexture=s.createTexture(),t.bindTexture(s.TEXTURE_CUBE_MAP,be.__webglTexture),Fe(s.TEXTURE_CUBE_MAP,R.depthTexture);let he=r.convert(R.depthTexture.format),_e=r.convert(R.depthTexture.type),me;R.depthTexture.format===Ri?me=s.DEPTH_COMPONENT24:R.depthTexture.format===Ms&&(me=s.DEPTH24_STENCIL8);for(let ae=0;ae<6;ae++)s.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ae,0,me,R.width,R.height,0,he,_e,null)}}else W(R.depthTexture,0);let De=be.__webglTexture,Ue=nt(R),V=le?s.TEXTURE_CUBE_MAP_POSITIVE_X+re:s.TEXTURE_2D,Y=R.depthTexture.format===Ms?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;if(R.depthTexture.format===Ri)ct(R)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,Y,V,De,0,Ue):s.framebufferTexture2D(s.FRAMEBUFFER,Y,V,De,0);else if(R.depthTexture.format===Ms)ct(R)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,Y,V,De,0,Ue):s.framebufferTexture2D(s.FRAMEBUFFER,Y,V,De,0);else throw new Error("THREE.WebGLTextures: Unknown depthTexture format.")}function ce(O){let R=n.get(O),re=O.isWebGLCubeRenderTarget===!0;if(R.__boundDepthTexture!==O.depthTexture){let le=O.depthTexture;if(R.__depthDisposeCallback&&R.__depthDisposeCallback(),le){let be=()=>{delete R.__boundDepthTexture,delete R.__depthDisposeCallback,le.removeEventListener("dispose",be)};le.addEventListener("dispose",be),R.__depthDisposeCallback=be}R.__boundDepthTexture=le}if(O.depthTexture&&!R.__autoAllocateDepthBuffer)if(re)for(let le=0;le<6;le++)ue(R.__webglFramebuffer[le],O,le);else{let le=O.texture.mipmaps;le&&le.length>0?ue(R.__webglFramebuffer[0],O,0):ue(R.__webglFramebuffer,O,0)}else if(re){R.__webglDepthbuffer=[];for(let le=0;le<6;le++)if(t.bindFramebuffer(s.FRAMEBUFFER,R.__webglFramebuffer[le]),R.__webglDepthbuffer[le]===void 0)R.__webglDepthbuffer[le]=s.createRenderbuffer(),$(R.__webglDepthbuffer[le],O,!1);else{let be=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,De=R.__webglDepthbuffer[le];s.bindRenderbuffer(s.RENDERBUFFER,De),s.framebufferRenderbuffer(s.FRAMEBUFFER,be,s.RENDERBUFFER,De)}}else{let le=O.texture.mipmaps;if(le&&le.length>0?t.bindFramebuffer(s.FRAMEBUFFER,R.__webglFramebuffer[0]):t.bindFramebuffer(s.FRAMEBUFFER,R.__webglFramebuffer),R.__webglDepthbuffer===void 0)R.__webglDepthbuffer=s.createRenderbuffer(),$(R.__webglDepthbuffer,O,!1);else{let be=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,De=R.__webglDepthbuffer;s.bindRenderbuffer(s.RENDERBUFFER,De),s.framebufferRenderbuffer(s.FRAMEBUFFER,be,s.RENDERBUFFER,De)}}t.bindFramebuffer(s.FRAMEBUFFER,null)}function ie(O,R,re){let le=n.get(O);R!==void 0&&xe(le.__webglFramebuffer,O,O.texture,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,0),re!==void 0&&ce(O)}function de(O){let R=O.texture,re=n.get(O),le=n.get(R);O.addEventListener("dispose",b);let be=O.textures,De=O.isWebGLCubeRenderTarget===!0,Ue=be.length>1;if(Ue||(le.__webglTexture===void 0&&(le.__webglTexture=s.createTexture()),le.__version=R.version,a.memory.textures++),De){re.__webglFramebuffer=[];for(let V=0;V<6;V++)if(R.mipmaps&&R.mipmaps.length>0){re.__webglFramebuffer[V]=[];for(let Y=0;Y<R.mipmaps.length;Y++)re.__webglFramebuffer[V][Y]=s.createFramebuffer()}else re.__webglFramebuffer[V]=s.createFramebuffer()}else{if(R.mipmaps&&R.mipmaps.length>0){re.__webglFramebuffer=[];for(let V=0;V<R.mipmaps.length;V++)re.__webglFramebuffer[V]=s.createFramebuffer()}else re.__webglFramebuffer=s.createFramebuffer();if(Ue)for(let V=0,Y=be.length;V<Y;V++){let he=n.get(be[V]);he.__webglTexture===void 0&&(he.__webglTexture=s.createTexture(),a.memory.textures++)}if(O.samples>0&&ct(O)===!1){re.__webglMultisampledFramebuffer=s.createFramebuffer(),re.__webglColorRenderbuffer=[],t.bindFramebuffer(s.FRAMEBUFFER,re.__webglMultisampledFramebuffer);for(let V=0;V<be.length;V++){let Y=be[V];re.__webglColorRenderbuffer[V]=s.createRenderbuffer(),s.bindRenderbuffer(s.RENDERBUFFER,re.__webglColorRenderbuffer[V]);let he=r.convert(Y.format,Y.colorSpace),_e=r.convert(Y.type),me=g(Y.internalFormat,he,_e,Y.normalized,Y.colorSpace,O.isXRRenderTarget===!0),ae=nt(O);s.renderbufferStorageMultisample(s.RENDERBUFFER,ae,me,O.width,O.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+V,s.RENDERBUFFER,re.__webglColorRenderbuffer[V])}s.bindRenderbuffer(s.RENDERBUFFER,null),O.depthBuffer&&(re.__webglDepthRenderbuffer=s.createRenderbuffer(),$(re.__webglDepthRenderbuffer,O,!0)),t.bindFramebuffer(s.FRAMEBUFFER,null)}}if(De){t.bindTexture(s.TEXTURE_CUBE_MAP,le.__webglTexture),Fe(s.TEXTURE_CUBE_MAP,R);for(let V=0;V<6;V++)if(R.mipmaps&&R.mipmaps.length>0)for(let Y=0;Y<R.mipmaps.length;Y++)xe(re.__webglFramebuffer[V][Y],O,R,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+V,Y);else xe(re.__webglFramebuffer[V],O,R,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+V,0);f(R)&&y(s.TEXTURE_CUBE_MAP),t.unbindTexture()}else if(Ue){for(let V=0,Y=be.length;V<Y;V++){let he=be[V],_e=n.get(he),me=s.TEXTURE_2D;(O.isWebGL3DRenderTarget||O.isWebGLArrayRenderTarget)&&(me=O.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(me,_e.__webglTexture),Fe(me,he),xe(re.__webglFramebuffer,O,he,s.COLOR_ATTACHMENT0+V,me,0),f(he)&&y(me)}t.unbindTexture()}else{let V=s.TEXTURE_2D;if((O.isWebGL3DRenderTarget||O.isWebGLArrayRenderTarget)&&(V=O.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(V,le.__webglTexture),Fe(V,R),R.mipmaps&&R.mipmaps.length>0)for(let Y=0;Y<R.mipmaps.length;Y++)xe(re.__webglFramebuffer[Y],O,R,s.COLOR_ATTACHMENT0,V,Y);else xe(re.__webglFramebuffer,O,R,s.COLOR_ATTACHMENT0,V,0);f(R)&&y(V),t.unbindTexture()}O.depthBuffer&&ce(O)}function ve(O){let R=O.textures;for(let re=0,le=R.length;re<le;re++){let be=R[re];if(f(be)){let De=T(O),Ue=n.get(be).__webglTexture;t.bindTexture(De,Ue),y(De),t.unbindTexture()}}}let We=[],Ve=[];function lt(O){if(O.samples>0){if(ct(O)===!1){let R=O.textures,re=O.width,le=O.height,be=s.COLOR_BUFFER_BIT,De=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Ue=n.get(O),V=R.length>1;if(V)for(let he=0;he<R.length;he++)t.bindFramebuffer(s.FRAMEBUFFER,Ue.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+he,s.RENDERBUFFER,null),t.bindFramebuffer(s.FRAMEBUFFER,Ue.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+he,s.TEXTURE_2D,null,0);t.bindFramebuffer(s.READ_FRAMEBUFFER,Ue.__webglMultisampledFramebuffer);let Y=O.texture.mipmaps;Y&&Y.length>0?t.bindFramebuffer(s.DRAW_FRAMEBUFFER,Ue.__webglFramebuffer[0]):t.bindFramebuffer(s.DRAW_FRAMEBUFFER,Ue.__webglFramebuffer);for(let he=0;he<R.length;he++){if(O.resolveDepthBuffer&&(O.depthBuffer&&(be|=s.DEPTH_BUFFER_BIT),O.stencilBuffer&&O.resolveStencilBuffer&&(be|=s.STENCIL_BUFFER_BIT)),V){s.framebufferRenderbuffer(s.READ_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.RENDERBUFFER,Ue.__webglColorRenderbuffer[he]);let _e=n.get(R[he]).__webglTexture;s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,_e,0)}s.blitFramebuffer(0,0,re,le,0,0,re,le,be,s.NEAREST),l===!0&&(We.length=0,Ve.length=0,We.push(s.COLOR_ATTACHMENT0+he),O.depthBuffer&&O.storeMultisampledDepthBuffer===!1&&(We.push(De),Ve.push(De),s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,Ve)),s.invalidateFramebuffer(s.READ_FRAMEBUFFER,We))}if(t.bindFramebuffer(s.READ_FRAMEBUFFER,null),t.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),V)for(let he=0;he<R.length;he++){t.bindFramebuffer(s.FRAMEBUFFER,Ue.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+he,s.RENDERBUFFER,Ue.__webglColorRenderbuffer[he]);let _e=n.get(R[he]).__webglTexture;t.bindFramebuffer(s.FRAMEBUFFER,Ue.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+he,s.TEXTURE_2D,_e,0)}t.bindFramebuffer(s.DRAW_FRAMEBUFFER,Ue.__webglMultisampledFramebuffer)}else if(O.depthBuffer&&O.storeMultisampledDepthBuffer===!1&&l){let R=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,[R])}}}function nt(O){return Math.min(i.maxSamples,O.samples)}function ct(O){let R=n.get(O);return O.samples>0&&e.has("WEBGL_multisampled_render_to_texture")===!0&&R.__useRenderToTexture!==!1}function Q(O){let R=a.render.frame;h.get(O)!==R&&(h.set(O,R),O.update())}function Pt(O,R){let re=O.colorSpace,le=O.format,be=O.type;return O.isCompressedTexture===!0||O.isVideoTexture===!0||re!==On&&re!==ri&&(Rt.getTransfer(re)===Wt?(le!==Qn||be!==Gn)&&ht("WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):gt("WebGLTextures: Unsupported texture color space:",re)),R}function ft(O){return typeof HTMLImageElement<"u"&&O instanceof HTMLImageElement?(c.width=O.naturalWidth||O.width,c.height=O.naturalHeight||O.height):typeof VideoFrame<"u"&&O instanceof VideoFrame?(c.width=O.displayWidth,c.height=O.displayHeight):(c.width=O.width,c.height=O.height),c}this.allocateTextureUnit=H,this.resetTextureUnits=N,this.getTextureUnits=D,this.setTextureUnits=U,this.setTexture2D=W,this.setTexture2DArray=z,this.setTexture3D=E,this.setTextureCube=J,this.rebindTextures=ie,this.setupRenderTarget=de,this.updateRenderTargetMipmap=ve,this.updateMultisampleRenderTarget=lt,this.setupDepthRenderbuffer=ce,this.setupFrameBufferTexture=xe,this.useMultisampledRTT=ct,this.isReversedDepthBuffer=function(){return t.buffers.depth.getReversed()}}function G_(s,e){function t(n,i=ri){let r,a=Rt.getTransfer(i);if(n===Gn)return s.UNSIGNED_BYTE;if(n===Fl)return s.UNSIGNED_SHORT_4_4_4_4;if(n===Nl)return s.UNSIGNED_SHORT_5_5_5_1;if(n===Bh)return s.UNSIGNED_INT_5_9_9_9_REV;if(n===Hh)return s.UNSIGNED_INT_10F_11F_11F_REV;if(n===zh)return s.BYTE;if(n===kh)return s.SHORT;if(n===Gr)return s.UNSIGNED_SHORT;if(n===Ll)return s.INT;if(n===yi)return s.UNSIGNED_INT;if(n===Zn)return s.FLOAT;if(n===gn)return s.HALF_FLOAT;if(n===Gh)return s.ALPHA;if(n===Vh)return s.RGB;if(n===Qn)return s.RGBA;if(n===Ri)return s.DEPTH_COMPONENT;if(n===Ms)return s.DEPTH_STENCIL;if(n===Ul)return s.RED;if(n===Ol)return s.RED_INTEGER;if(n===ws)return s.RG;if(n===zl)return s.RG_INTEGER;if(n===kl)return s.RGBA_INTEGER;if(n===$a||n===eo||n===to||n===no)if(a===Wt)if(r=e.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(n===$a)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(n===eo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(n===to)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(n===no)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=e.get("WEBGL_compressed_texture_s3tc"),r!==null){if(n===$a)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(n===eo)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(n===to)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(n===no)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(n===Bl||n===Hl||n===Gl||n===Vl)if(r=e.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(n===Bl)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(n===Hl)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(n===Gl)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(n===Vl)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(n===Wl||n===ql||n===Xl||n===jl||n===Kl||n===io||n===Yl)if(r=e.get("WEBGL_compressed_texture_etc"),r!==null){if(n===Wl||n===ql)return a===Wt?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(n===Xl)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC;if(n===jl)return r.COMPRESSED_R11_EAC;if(n===Kl)return r.COMPRESSED_SIGNED_R11_EAC;if(n===io)return r.COMPRESSED_RG11_EAC;if(n===Yl)return r.COMPRESSED_SIGNED_RG11_EAC}else return null;if(n===Jl||n===Zl||n===Ql||n===$l||n===ec||n===tc||n===nc||n===ic||n===sc||n===rc||n===ac||n===oc||n===lc||n===cc)if(r=e.get("WEBGL_compressed_texture_astc"),r!==null){if(n===Jl)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(n===Zl)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(n===Ql)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(n===$l)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(n===ec)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(n===tc)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(n===nc)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(n===ic)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(n===sc)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(n===rc)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(n===ac)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(n===oc)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(n===lc)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(n===cc)return a===Wt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(n===hc||n===uc||n===dc)if(r=e.get("EXT_texture_compression_bptc"),r!==null){if(n===hc)return a===Wt?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(n===uc)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(n===dc)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(n===fc||n===pc||n===so||n===mc)if(r=e.get("EXT_texture_compression_rgtc"),r!==null){if(n===fc)return r.COMPRESSED_RED_RGTC1_EXT;if(n===pc)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(n===so)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(n===mc)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return n===Vr?s.UNSIGNED_INT_24_8:s[n]!==void 0?s[n]:null}return{convert:t}}var V_=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,W_=`
uniform sampler2DArray depthColor;
uniform float depthWidth;
uniform float depthHeight;

void main() {

	vec2 coord = vec2( gl_FragCoord.x / depthWidth, gl_FragCoord.y / depthHeight );

	if ( coord.x >= 1.0 ) {

		gl_FragDepth = texture( depthColor, vec3( coord.x - 1.0, coord.y, 1 ) ).r;

	} else {

		gl_FragDepth = texture( depthColor, vec3( coord.x, coord.y, 0 ) ).r;

	}

}`,du=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(e,t){if(this.texture===null){let n=new Ca(e.texture);(e.depthNear!==t.depthNear||e.depthFar!==t.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=n}}getMesh(e){if(this.texture!==null&&this.mesh===null){let t=e.cameras[0].viewport,n=new wt({vertexShader:V_,fragmentShader:W_,uniforms:{depthColor:{value:this.texture},depthWidth:{value:t.z},depthHeight:{value:t.w}}});this.mesh=new ot(new un(20,20),n)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},fu=class extends ni{constructor(e,t){super();let n=this,i=null,r=1,a=null,o="local-floor",l=1,c=null,h=null,d=null,u=null,p=null,x=null,v=typeof XRWebGLBinding<"u",m=new du,f={},y=t.getContextAttributes(),T=null,g=null,_=[],S=[],A=new ze,b=null,C=null,w=new mn;w.viewport=new Nt;let I=new mn;I.viewport=new Nt;let M=[w,I],N=new Tl,D=null,U=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(se){let oe=_[se];return oe===void 0&&(oe=new Er,_[se]=oe),oe.getTargetRaySpace()},this.getControllerGrip=function(se){let oe=_[se];return oe===void 0&&(oe=new Er,_[se]=oe),oe.getGripSpace()},this.getHand=function(se){let oe=_[se];return oe===void 0&&(oe=new Er,_[se]=oe),oe.getHandSpace()};function H(se){let oe=S.indexOf(se.inputSource);if(oe===-1)return;let Pe=_[oe];Pe!==void 0&&(Pe.update(se.inputSource,se.frame,c||a),Pe.dispatchEvent({type:se.type,data:se.inputSource}))}function L(){i.removeEventListener("select",H),i.removeEventListener("selectstart",H),i.removeEventListener("selectend",H),i.removeEventListener("squeeze",H),i.removeEventListener("squeezestart",H),i.removeEventListener("squeezeend",H),i.removeEventListener("end",L),i.removeEventListener("inputsourceschange",W);for(let se=0;se<_.length;se++){let oe=S[se];oe!==null&&(S[se]=null,_[se].disconnect(oe))}D=null,U=null,m.reset();for(let se in f)delete f[se];if(e.setRenderTarget(T),p=null,u=null,d=null,i=null,g=null,Ne.stop(),n.isPresenting=!1,e.setPixelRatio(b),e.setSize(A.width,A.height,!1),C!==null){let se=C.camera;se.fov=C.fov,se.zoom=C.zoom,se.updateProjectionMatrix(),C=null}n.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(se){r=se,n.isPresenting===!0&&ht("WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(se){o=se,n.isPresenting===!0&&ht("WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return c||a},this.setReferenceSpace=function(se){c=se},this.getBaseLayer=function(){return u!==null?u:p},this.getBinding=function(){return d===null&&v&&(d=new XRWebGLBinding(i,t)),d},this.getFrame=function(){return x},this.getSession=function(){return i},this.setSession=async function(se){if(i=se,i!==null){if(T=e.getRenderTarget(),i.addEventListener("select",H),i.addEventListener("selectstart",H),i.addEventListener("selectend",H),i.addEventListener("squeeze",H),i.addEventListener("squeezestart",H),i.addEventListener("squeezeend",H),i.addEventListener("end",L),i.addEventListener("inputsourceschange",W),y.xrCompatible!==!0&&await t.makeXRCompatible(),b=e.getPixelRatio(),e.getSize(A),v&&"createProjectionLayer"in XRWebGLBinding.prototype){let Pe=null,ge=null,xe=null;y.depth&&(xe=y.stencil?t.DEPTH24_STENCIL8:t.DEPTH_COMPONENT24,Pe=y.stencil?Ms:Ri,ge=y.stencil?Vr:yi);let $={colorFormat:t.RGBA8,depthFormat:xe,scaleFactor:r};d=this.getBinding(),u=d.createProjectionLayer($),i.updateRenderState({layers:[u]}),e.setPixelRatio(1),e.setSize(u.textureWidth,u.textureHeight,!1),g=new an(u.textureWidth,u.textureHeight,{format:Qn,type:Gn,depthTexture:new ms(u.textureWidth,u.textureHeight,ge,void 0,void 0,void 0,void 0,void 0,void 0,Pe),stencilBuffer:y.stencil,colorSpace:e.outputColorSpace,samples:y.antialias?4:0,resolveDepthBuffer:u.ignoreDepthValues===!1,resolveStencilBuffer:u.ignoreDepthValues===!1,storeMultisampledDepthBuffer:u.ignoreDepthValues===!1,storeMultisampledStencilBuffer:u.ignoreDepthValues===!1})}else{let Pe={antialias:y.antialias,alpha:!0,depth:y.depth,stencil:y.stencil,framebufferScaleFactor:r};p=new XRWebGLLayer(i,t,Pe),i.updateRenderState({baseLayer:p}),e.setPixelRatio(1),e.setSize(p.framebufferWidth,p.framebufferHeight,!1),g=new an(p.framebufferWidth,p.framebufferHeight,{format:Qn,type:Gn,colorSpace:e.outputColorSpace,stencilBuffer:y.stencil,resolveDepthBuffer:p.ignoreDepthValues===!1,resolveStencilBuffer:p.ignoreDepthValues===!1,storeMultisampledDepthBuffer:p.ignoreDepthValues===!1,storeMultisampledStencilBuffer:p.ignoreDepthValues===!1})}g.isXRRenderTarget=!0,this.setFoveation(l),c=null,a=await i.requestReferenceSpace(o),Ne.setContext(i),Ne.start(),n.isPresenting=!0,n.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(i!==null)return i.environmentBlendMode},this.getDepthTexture=function(){return m.getDepthTexture()};function W(se){for(let oe=0;oe<se.removed.length;oe++){let Pe=se.removed[oe],ge=S.indexOf(Pe);ge>=0&&(S[ge]=null,_[ge].disconnect(Pe))}for(let oe=0;oe<se.added.length;oe++){let Pe=se.added[oe],ge=S.indexOf(Pe);if(ge===-1){for(let $=0;$<_.length;$++)if($>=S.length){S.push(Pe),ge=$;break}else if(S[$]===null){S[$]=Pe,ge=$;break}if(ge===-1)break}let xe=_[ge];xe&&xe.connect(Pe)}}let z=new k,E=new k;function J(se,oe,Pe){z.setFromMatrixPosition(oe.matrixWorld),E.setFromMatrixPosition(Pe.matrixWorld);let ge=z.distanceTo(E),xe=oe.projectionMatrix.elements,$=Pe.projectionMatrix.elements,ue=xe[14]/(xe[10]-1),ce=xe[14]/(xe[10]+1),ie=(xe[9]+1)/xe[5],de=(xe[9]-1)/xe[5],ve=(xe[8]-1)/xe[0],We=($[8]+1)/$[0],Ve=ue*ve,lt=ue*We,nt=ge/(-ve+We),ct=nt*-ve;if(oe.matrixWorld.decompose(se.position,se.quaternion,se.scale),se.translateX(ct),se.translateZ(nt),se.matrixWorld.compose(se.position,se.quaternion,se.scale),se.matrixWorldInverse.copy(se.matrixWorld).invert(),xe[10]===-1)se.projectionMatrix.copy(oe.projectionMatrix),se.projectionMatrixInverse.copy(oe.projectionMatrixInverse);else{let Q=ue+nt,Pt=ce+nt,ft=Ve-ct,O=lt+(ge-ct),R=ie*ce/Pt*Q,re=de*ce/Pt*Q;se.projectionMatrix.makePerspective(ft,O,R,re,Q,Pt),se.projectionMatrixInverse.copy(se.projectionMatrix).invert()}}function q(se,oe){oe===null?se.matrixWorld.copy(se.matrix):se.matrixWorld.multiplyMatrices(oe.matrixWorld,se.matrix),se.matrixWorldInverse.copy(se.matrixWorld).invert()}this.updateCamera=function(se){if(i===null)return;let oe=se.near,Pe=se.far;m.texture!==null&&(m.depthNear>0&&(oe=m.depthNear),m.depthFar>0&&(Pe=m.depthFar)),N.near=I.near=w.near=oe,N.far=I.far=w.far=Pe,(D!==N.near||U!==N.far)&&(i.updateRenderState({depthNear:N.near,depthFar:N.far}),D=N.near,U=N.far),N.layers.mask=se.layers.mask|6,w.layers.mask=N.layers.mask&-5,I.layers.mask=N.layers.mask&-3;let ge=se.parent,xe=N.cameras;q(N,ge);for(let $=0;$<xe.length;$++)q(xe[$],ge);xe.length===2?J(N,w,I):N.projectionMatrix.copy(w.projectionMatrix),C===null&&se.isPerspectiveCamera&&(C={camera:se,fov:se.fov,zoom:se.zoom}),te(se,N,ge)};function te(se,oe,Pe){Pe===null?se.matrix.copy(oe.matrixWorld):(se.matrix.copy(Pe.matrixWorld),se.matrix.invert(),se.matrix.multiply(oe.matrixWorld)),se.matrix.decompose(se.position,se.quaternion,se.scale),se.updateMatrixWorld(!0),se.projectionMatrix.copy(oe.projectionMatrix),se.projectionMatrixInverse.copy(oe.projectionMatrixInverse),se.isPerspectiveCamera&&(se.fov=zs*2*Math.atan(1/se.projectionMatrix.elements[5]),se.zoom=1)}this.getCamera=function(){return N},this.getFoveation=function(){if(!(u===null&&p===null))return l},this.setFoveation=function(se){l=se,u!==null&&(u.fixedFoveation=se),p!==null&&p.fixedFoveation!==void 0&&(p.fixedFoveation=se)},this.hasDepthSensing=function(){return m.texture!==null},this.getDepthSensingMesh=function(){return m.getMesh(N)},this.getCameraTexture=function(se){return f[se]};let Le=null;function Fe(se,oe){if(h=oe.getViewerPose(c||a),x=oe,h!==null){let Pe=h.views;p!==null&&(e.setRenderTargetFramebuffer(g,p.framebuffer),e.setRenderTarget(g));let ge=!1;Pe.length!==N.cameras.length&&(N.cameras.length=0,ge=!0);for(let ce=0;ce<Pe.length;ce++){let ie=Pe[ce],de=null;if(p!==null)de=p.getViewport(ie);else{let We=d.getViewSubImage(u,ie);de=We.viewport,ce===0&&(e.setRenderTargetTextures(g,We.colorTexture,We.depthStencilTexture),e.setRenderTarget(g))}let ve=M[ce];ve===void 0&&(ve=new mn,ve.layers.enable(ce),ve.viewport=new Nt,M[ce]=ve),ve.matrix.fromArray(ie.transform.matrix),ve.matrix.decompose(ve.position,ve.quaternion,ve.scale),ve.projectionMatrix.fromArray(ie.projectionMatrix),ve.projectionMatrixInverse.copy(ve.projectionMatrix).invert(),ve.viewport.set(de.x,de.y,de.width,de.height),ce===0&&(N.matrix.copy(ve.matrix),N.matrix.decompose(N.position,N.quaternion,N.scale)),ge===!0&&N.cameras.push(ve)}let xe=i.enabledFeatures;if(xe&&xe.includes("depth-sensing")&&i.depthUsage=="gpu-optimized"&&v){d=n.getBinding();let ce=d.getDepthInformation(Pe[0]);ce&&ce.isValid&&ce.texture&&m.init(ce,i.renderState)}if(xe&&xe.includes("camera-access")&&v){e.state.unbindTexture(),d=n.getBinding();for(let ce=0;ce<Pe.length;ce++){let ie=Pe[ce].camera;if(ie){let de=f[ie];de||(de=new Ca,f[ie]=de);let ve=d.getCameraImage(ie);de.sourceTexture=ve}}}}for(let Pe=0;Pe<_.length;Pe++){let ge=S[Pe],xe=_[Pe];ge!==null&&xe!==void 0&&xe.update(ge,oe,c||a)}Le&&Le(se,oe),oe.detectedPlanes&&n.dispatchEvent({type:"planesdetected",data:oe}),x=null}let Ne=new mp;Ne.setAnimationLoop(Fe),this.setAnimationLoop=function(se){Le=se},this.dispose=function(){}}},q_=new mt,yp=new _t;yp.set(-1,0,0,0,1,0,0,0,1);function X_(s,e){function t(m,f){m.matrixAutoUpdate===!0&&m.updateMatrix(),f.value.copy(m.matrix)}function n(m,f){f.color.getRGB(m.fogColor.value,Yh(s)),f.isFog?(m.fogNear.value=f.near,m.fogFar.value=f.far):f.isFogExp2&&(m.fogDensity.value=f.density)}function i(m,f,y,T,g){f.isNodeMaterial?f.uniformsNeedUpdate=!1:f.isMeshBasicMaterial?r(m,f):f.isMeshLambertMaterial?(r(m,f),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)):f.isMeshToonMaterial?(r(m,f),d(m,f)):f.isMeshPhongMaterial?(r(m,f),h(m,f),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)):f.isMeshStandardMaterial?(r(m,f),u(m,f),f.isMeshPhysicalMaterial&&p(m,f,g)):f.isMeshMatcapMaterial?(r(m,f),x(m,f)):f.isMeshDepthMaterial?r(m,f):f.isMeshDistanceMaterial?(r(m,f),v(m,f)):f.isMeshNormalMaterial?r(m,f):f.isLineBasicMaterial?(a(m,f),f.isLineDashedMaterial&&o(m,f)):f.isPointsMaterial?l(m,f,y,T):f.isSpriteMaterial?c(m,f):f.isShadowMaterial?(m.color.value.copy(f.color),m.opacity.value=f.opacity):f.isShaderMaterial&&(f.uniformsNeedUpdate=!1)}function r(m,f){m.opacity.value=f.opacity,f.color&&m.diffuse.value.copy(f.color),f.emissive&&m.emissive.value.copy(f.emissive).multiplyScalar(f.emissiveIntensity),f.map&&(m.map.value=f.map,t(f.map,m.mapTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.bumpMap&&(m.bumpMap.value=f.bumpMap,t(f.bumpMap,m.bumpMapTransform),m.bumpScale.value=f.bumpScale,f.side===xn&&(m.bumpScale.value*=-1)),f.normalMap&&(m.normalMap.value=f.normalMap,t(f.normalMap,m.normalMapTransform),m.normalScale.value.copy(f.normalScale),f.side===xn&&m.normalScale.value.negate()),f.displacementMap&&(m.displacementMap.value=f.displacementMap,t(f.displacementMap,m.displacementMapTransform),m.displacementScale.value=f.displacementScale,m.displacementBias.value=f.displacementBias),f.emissiveMap&&(m.emissiveMap.value=f.emissiveMap,t(f.emissiveMap,m.emissiveMapTransform)),f.specularMap&&(m.specularMap.value=f.specularMap,t(f.specularMap,m.specularMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest);let y=e.get(f),T=y.envMap,g=y.envMapRotation;T&&(m.envMap.value=T,m.envMapRotation.value.setFromMatrix4(q_.makeRotationFromEuler(g)).transpose(),T.isCubeTexture&&T.isRenderTargetTexture===!1&&m.envMapRotation.value.premultiply(yp),m.reflectivity.value=f.reflectivity,m.ior.value=f.ior,m.refractionRatio.value=f.refractionRatio),f.lightMap&&(m.lightMap.value=f.lightMap,m.lightMapIntensity.value=f.lightMapIntensity,t(f.lightMap,m.lightMapTransform)),f.aoMap&&(m.aoMap.value=f.aoMap,m.aoMapIntensity.value=f.aoMapIntensity,t(f.aoMap,m.aoMapTransform))}function a(m,f){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,f.map&&(m.map.value=f.map,t(f.map,m.mapTransform))}function o(m,f){m.dashSize.value=f.dashSize,m.totalSize.value=f.dashSize+f.gapSize,m.scale.value=f.scale}function l(m,f,y,T){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,m.size.value=f.size*y,m.scale.value=T*.5,f.map&&(m.map.value=f.map,t(f.map,m.uvTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest)}function c(m,f){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,m.rotation.value=f.rotation,f.map&&(m.map.value=f.map,t(f.map,m.mapTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest)}function h(m,f){m.specular.value.copy(f.specular),m.shininess.value=Math.max(f.shininess,1e-4)}function d(m,f){f.gradientMap&&(m.gradientMap.value=f.gradientMap)}function u(m,f){m.metalness.value=f.metalness,f.metalnessMap&&(m.metalnessMap.value=f.metalnessMap,t(f.metalnessMap,m.metalnessMapTransform)),m.roughness.value=f.roughness,f.roughnessMap&&(m.roughnessMap.value=f.roughnessMap,t(f.roughnessMap,m.roughnessMapTransform)),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)}function p(m,f,y){m.ior.value=f.ior,f.sheen>0&&(m.sheenColor.value.copy(f.sheenColor).multiplyScalar(f.sheen),m.sheenRoughness.value=f.sheenRoughness,f.sheenColorMap&&(m.sheenColorMap.value=f.sheenColorMap,t(f.sheenColorMap,m.sheenColorMapTransform)),f.sheenRoughnessMap&&(m.sheenRoughnessMap.value=f.sheenRoughnessMap,t(f.sheenRoughnessMap,m.sheenRoughnessMapTransform))),f.clearcoat>0&&(m.clearcoat.value=f.clearcoat,m.clearcoatRoughness.value=f.clearcoatRoughness,f.clearcoatMap&&(m.clearcoatMap.value=f.clearcoatMap,t(f.clearcoatMap,m.clearcoatMapTransform)),f.clearcoatRoughnessMap&&(m.clearcoatRoughnessMap.value=f.clearcoatRoughnessMap,t(f.clearcoatRoughnessMap,m.clearcoatRoughnessMapTransform)),f.clearcoatNormalMap&&(m.clearcoatNormalMap.value=f.clearcoatNormalMap,t(f.clearcoatNormalMap,m.clearcoatNormalMapTransform),m.clearcoatNormalScale.value.copy(f.clearcoatNormalScale),f.side===xn&&m.clearcoatNormalScale.value.negate())),f.dispersion>0&&(m.dispersion.value=f.dispersion),f.retroreflectivity>0&&(m.retroreflectivity.value=f.retroreflectivity),f.iridescence>0&&(m.iridescence.value=f.iridescence,m.iridescenceIOR.value=f.iridescenceIOR,m.iridescenceThicknessMinimum.value=f.iridescenceThicknessRange[0],m.iridescenceThicknessMaximum.value=f.iridescenceThicknessRange[1],f.iridescenceMap&&(m.iridescenceMap.value=f.iridescenceMap,t(f.iridescenceMap,m.iridescenceMapTransform)),f.iridescenceThicknessMap&&(m.iridescenceThicknessMap.value=f.iridescenceThicknessMap,t(f.iridescenceThicknessMap,m.iridescenceThicknessMapTransform))),f.transmission>0&&(m.transmission.value=f.transmission,m.transmissionSamplerMap.value=y.texture,m.transmissionSamplerSize.value.set(y.width,y.height),f.transmissionMap&&(m.transmissionMap.value=f.transmissionMap,t(f.transmissionMap,m.transmissionMapTransform)),m.thickness.value=f.thickness,f.thicknessMap&&(m.thicknessMap.value=f.thicknessMap,t(f.thicknessMap,m.thicknessMapTransform)),m.attenuationDistance.value=f.attenuationDistance,m.attenuationColor.value.copy(f.attenuationColor)),f.anisotropy>0&&(m.anisotropyVector.value.set(f.anisotropy*Math.cos(f.anisotropyRotation),f.anisotropy*Math.sin(f.anisotropyRotation)),f.anisotropyMap&&(m.anisotropyMap.value=f.anisotropyMap,t(f.anisotropyMap,m.anisotropyMapTransform))),m.specularIntensity.value=f.specularIntensity,m.specularColor.value.copy(f.specularColor),f.specularColorMap&&(m.specularColorMap.value=f.specularColorMap,t(f.specularColorMap,m.specularColorMapTransform)),f.specularIntensityMap&&(m.specularIntensityMap.value=f.specularIntensityMap,t(f.specularIntensityMap,m.specularIntensityMapTransform))}function x(m,f){f.matcap&&(m.matcap.value=f.matcap)}function v(m,f){let y=e.get(f).light;m.referencePosition.value.setFromMatrixPosition(y.matrixWorld),m.nearDistance.value=y.shadow.camera.near,m.farDistance.value=y.shadow.camera.far}return{refreshFogUniforms:n,refreshMaterialUniforms:i}}function j_(s,e,t,n){let i={},r={},a=[],o=s.getParameter(s.MAX_UNIFORM_BUFFER_BINDINGS);function l(g,_){let S=_.program;n.uniformBlockBinding(g,S)}function c(g,_){let S=i[g.id];S===void 0&&(m(g),S=h(g),i[g.id]=S,g.addEventListener("dispose",y));let A=_.program;n.updateUBOMapping(g,A);let b=e.render.frame;r[g.id]!==b&&(u(g),r[g.id]=b)}function h(g){let _=d();g.__bindingPointIndex=_;let S=s.createBuffer(),A=g.__size,b=g.usage;return s.bindBuffer(s.UNIFORM_BUFFER,S),s.bufferData(s.UNIFORM_BUFFER,A,b),s.bindBuffer(s.UNIFORM_BUFFER,null),s.bindBufferBase(s.UNIFORM_BUFFER,_,S),S}function d(){for(let g=0;g<o;g++)if(a.indexOf(g)===-1)return a.push(g),g;return gt("WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function u(g){let _=i[g.id],S=g.uniforms,A=g.__cache;s.bindBuffer(s.UNIFORM_BUFFER,_);for(let b=0,C=S.length;b<C;b++){let w=S[b];if(Array.isArray(w))for(let I=0,M=w.length;I<M;I++)p(w[I],b,I,A);else p(w,b,0,A)}s.bindBuffer(s.UNIFORM_BUFFER,null)}function p(g,_,S,A){if(v(g,_,S,A)===!0){let b=g.__offset,C=g.value;if(Array.isArray(C)){let w=0;for(let I=0;I<C.length;I++){let M=C[I],N=f(M);x(M,g.__data,w),typeof M!="number"&&typeof M!="boolean"&&!M.isMatrix3&&!ArrayBuffer.isView(M)&&(w+=N.storage/Float32Array.BYTES_PER_ELEMENT)}}else x(C,g.__data,0);s.bufferSubData(s.UNIFORM_BUFFER,b,g.__data)}}function x(g,_,S){typeof g=="number"||typeof g=="boolean"?_[0]=g:g.isMatrix3?(_[0]=g.elements[0],_[1]=g.elements[1],_[2]=g.elements[2],_[3]=0,_[4]=g.elements[3],_[5]=g.elements[4],_[6]=g.elements[5],_[7]=0,_[8]=g.elements[6],_[9]=g.elements[7],_[10]=g.elements[8],_[11]=0):ArrayBuffer.isView(g)?_.set(new g.constructor(g.buffer,g.byteOffset,_.length)):g.toArray(_,S)}function v(g,_,S,A){let b=g.value,C=_+"_"+S;if(A[C]===void 0)return typeof b=="number"||typeof b=="boolean"?A[C]=b:ArrayBuffer.isView(b)?A[C]=b.slice():A[C]=b.clone(),!0;{let w=A[C];if(typeof b=="number"||typeof b=="boolean"){if(w!==b)return A[C]=b,!0}else{if(ArrayBuffer.isView(b))return!0;if(w.equals(b)===!1)return w.copy(b),!0}}return!1}function m(g){let _=g.uniforms,S=0,A=16;for(let C=0,w=_.length;C<w;C++){let I=Array.isArray(_[C])?_[C]:[_[C]];for(let M=0,N=I.length;M<N;M++){let D=I[M],U=Array.isArray(D.value)?D.value:[D.value];for(let H=0,L=U.length;H<L;H++){let W=U[H],z=f(W),E=S%A,J=E%z.boundary,q=E+J;S+=J,q!==0&&A-q<z.storage&&(S+=A-q),D.__data=new Float32Array(z.storage/Float32Array.BYTES_PER_ELEMENT),D.__offset=S,S+=z.storage}}}let b=S%A;return b>0&&(S+=A-b),g.__size=S,g.__cache={},this}function f(g){let _={boundary:0,storage:0};return typeof g=="number"||typeof g=="boolean"?(_.boundary=4,_.storage=4):g.isVector2?(_.boundary=8,_.storage=8):g.isVector3||g.isColor?(_.boundary=16,_.storage=12):g.isVector4?(_.boundary=16,_.storage=16):g.isMatrix3?(_.boundary=48,_.storage=48):g.isMatrix4?(_.boundary=64,_.storage=64):g.isTexture?ht("WebGLRenderer: Texture samplers can not be part of an uniforms group."):ArrayBuffer.isView(g)?(_.boundary=16,_.storage=g.byteLength):ht("WebGLRenderer: Unsupported uniform value type.",g),_}function y(g){let _=g.target;_.removeEventListener("dispose",y);let S=a.indexOf(_.__bindingPointIndex);a.splice(S,1),s.deleteBuffer(i[_.id]),delete i[_.id],delete r[_.id]}function T(){for(let g in i)s.deleteBuffer(i[g]);a=[],i={},r={}}return{bind:l,update:c,dispose:T}}var K_=new Uint16Array([12469,15057,12620,14925,13266,14620,13807,14376,14323,13990,14545,13625,14713,13328,14840,12882,14931,12528,14996,12233,15039,11829,15066,11525,15080,11295,15085,10976,15082,10705,15073,10495,13880,14564,13898,14542,13977,14430,14158,14124,14393,13732,14556,13410,14702,12996,14814,12596,14891,12291,14937,11834,14957,11489,14958,11194,14943,10803,14921,10506,14893,10278,14858,9960,14484,14039,14487,14025,14499,13941,14524,13740,14574,13468,14654,13106,14743,12678,14818,12344,14867,11893,14889,11509,14893,11180,14881,10751,14852,10428,14812,10128,14765,9754,14712,9466,14764,13480,14764,13475,14766,13440,14766,13347,14769,13070,14786,12713,14816,12387,14844,11957,14860,11549,14868,11215,14855,10751,14825,10403,14782,10044,14729,9651,14666,9352,14599,9029,14967,12835,14966,12831,14963,12804,14954,12723,14936,12564,14917,12347,14900,11958,14886,11569,14878,11247,14859,10765,14828,10401,14784,10011,14727,9600,14660,9289,14586,8893,14508,8533,15111,12234,15110,12234,15104,12216,15092,12156,15067,12010,15028,11776,14981,11500,14942,11205,14902,10752,14861,10393,14812,9991,14752,9570,14682,9252,14603,8808,14519,8445,14431,8145,15209,11449,15208,11451,15202,11451,15190,11438,15163,11384,15117,11274,15055,10979,14994,10648,14932,10343,14871,9936,14803,9532,14729,9218,14645,8742,14556,8381,14461,8020,14365,7603,15273,10603,15272,10607,15267,10619,15256,10631,15231,10614,15182,10535,15118,10389,15042,10167,14963,9787,14883,9447,14800,9115,14710,8665,14615,8318,14514,7911,14411,7507,14279,7198,15314,9675,15313,9683,15309,9712,15298,9759,15277,9797,15229,9773,15166,9668,15084,9487,14995,9274,14898,8910,14800,8539,14697,8234,14590,7790,14479,7409,14367,7067,14178,6621,15337,8619,15337,8631,15333,8677,15325,8769,15305,8871,15264,8940,15202,8909,15119,8775,15022,8565,14916,8328,14804,8009,14688,7614,14569,7287,14448,6888,14321,6483,14088,6171,15350,7402,15350,7419,15347,7480,15340,7613,15322,7804,15287,7973,15229,8057,15148,8012,15046,7846,14933,7611,14810,7357,14682,7069,14552,6656,14421,6316,14251,5948,14007,5528,15356,5942,15356,5977,15353,6119,15348,6294,15332,6551,15302,6824,15249,7044,15171,7122,15070,7050,14949,6861,14818,6611,14679,6349,14538,6067,14398,5651,14189,5311,13935,4958,15359,4123,15359,4153,15356,4296,15353,4646,15338,5160,15311,5508,15263,5829,15188,6042,15088,6094,14966,6001,14826,5796,14678,5543,14527,5287,14377,4985,14133,4586,13869,4257,15360,1563,15360,1642,15358,2076,15354,2636,15341,3350,15317,4019,15273,4429,15203,4732,15105,4911,14981,4932,14836,4818,14679,4621,14517,4386,14359,4156,14083,3795,13808,3437,15360,122,15360,137,15358,285,15355,636,15344,1274,15322,2177,15281,2765,15215,3223,15120,3451,14995,3569,14846,3567,14681,3466,14511,3305,14344,3121,14037,2800,13753,2467,15360,0,15360,1,15359,21,15355,89,15346,253,15325,479,15287,796,15225,1148,15133,1492,15008,1749,14856,1882,14685,1886,14506,1783,14324,1608,13996,1398,13702,1183]),Ni=null;function Y_(){return Ni===null&&(Ni=new ps(K_,16,16,ws,gn),Ni.name="DFG_LUT",Ni.minFilter=rn,Ni.magFilter=rn,Ni.wrapS=ti,Ni.wrapT=ti,Ni.generateMipmaps=!1,Ni.needsUpdate=!0),Ni}var wc=class{constructor(e={}){let{canvas:t=Hf(),context:n=null,depth:i=!0,stencil:r=!1,alpha:a=!1,antialias:o=!1,premultipliedAlpha:l=!0,preserveDrawingBuffer:c=!1,powerPreference:h="default",failIfMajorPerformanceCaveat:d=!1,reversedDepthBuffer:u=!1,outputBufferType:p=Gn}=e;this.isWebGLRenderer=!0;let x;if(n!==null){if(typeof WebGLRenderingContext<"u"&&n instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");x=n.getContextAttributes().alpha}else x=a;let v=p,m=new Set([kl,zl,Ol]),f=new Set([Gn,yi,Gr,Vr,Fl,Nl]),y=new Uint32Array(4),T=new Int32Array(4),g=new k,_=null,S=null,A=[],b=[],C=null;this.domElement=t,this.debug={checkShaderErrors:!0,diagnostics:{keywords:!1},onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this.toneMapping=vi,this.toneMappingExposure=1,this.transmissionResolutionScale=1;let w=this,I=!1,M=null,N=null,D=null,U=null;this._outputColorSpace=sn;let H=0,L=0,W=null,z=-1,E=null,J=new Nt,q=new Nt,te=null,Le=new Ce(0),Fe=0,Ne=t.width,se=t.height,oe=1,Pe=null,ge=null,xe=new Nt(0,0,Ne,se),$=new Nt(0,0,Ne,se),ue=!1,ce=new Cr,ie=!1,de=!1,ve=new mt,We=new k,Ve=new Nt,lt={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},nt=!1;function ct(){return W===null?oe:1}let Q=n;function Pt(F,ne){return t.getContext(F,ne)}let ft,O,R,re,le,be,De,Ue,V,Y,he,_e,me,ae,Se,Me,Ie,j,Te,we,Oe,ke,Ae;try{let F={alpha:!0,depth:i,stencil:r,antialias:o,premultipliedAlpha:l,preserveDrawingBuffer:c,powerPreference:h,failIfMajorPerformanceCaveat:d};if("setAttribute"in t&&t.setAttribute("data-engine",`three.js r${"186"}`),t.addEventListener("webglcontextlost",P,!1),t.addEventListener("webglcontextrestored",B,!1),t.addEventListener("webglcontextcreationerror",K,!1),Q===null){let ne="webgl2";if(Q=Pt(ne,F),Q===null)throw Pt(ne)?new Error("THREE.WebGLRenderer: Error creating WebGL context with your selected attributes."):new Error("THREE.WebGLRenderer: Error creating WebGL context.")}Ye()}catch(F){throw t.removeEventListener("webglcontextlost",P,!1),t.removeEventListener("webglcontextrestored",B,!1),t.removeEventListener("webglcontextcreationerror",K,!1),gt("WebGLRenderer: "+F.message),F}function Ye(){ft=new nv(Q),ft.init(),Oe=new G_(Q,ft),O=new Xb(Q,ft,e,Oe),R=new B_(Q,ft),O.reversedDepthBuffer&&u&&R.buffers.depth.setReversed(!0),N=Q.createFramebuffer(),D=Q.createFramebuffer(),U=Q.createFramebuffer(),re=new rv(Q),le=new T_,be=new H_(Q,ft,R,le,O,Oe,re),De=new tv(w),Ue=new o0(Q),ke=new Wb(Q,Ue),V=new iv(Q,Ue,re,ke),Y=new ov(Q,V,Ue,ke,re),j=new av(Q,O,be),Se=new jb(le),he=new E_(w,De,ft,O,ke,Se),_e=new X_(w,le),me=new R_,ae=new F_(ft),Ie=new Vb(w,De,R,Y,x,l),Me=new k_(w,Y,O),Ae=new j_(Q,re,O,R),Te=new qb(Q,ft,re),we=new sv(Q,ft,re),re.programs=he.programs,w.capabilities=O,w.extensions=ft,w.properties=le,w.renderLists=me,w.shadowMap=Me,w.state=R,w.info=re}v!==Gn&&(C=new cv(v,t.width,t.height,o,i,r));let Qe=new fu(w,Q);this.xr=Qe,this.getContext=function(){return Q},this.getContextAttributes=function(){return Q.getContextAttributes()},this.forceContextLoss=function(){let F=ft.get("WEBGL_lose_context");F&&F.loseContext()},this.forceContextRestore=function(){let F=ft.get("WEBGL_lose_context");F&&F.restoreContext()},this.getPixelRatio=function(){return oe},this.setPixelRatio=function(F){F!==void 0&&(oe=F,this.setSize(Ne,se,!1))},this.getSize=function(F){return F.set(Ne,se)},this.setSize=function(F,ne,ye=!0){if(Qe.isPresenting){ht("WebGLRenderer: Can't change size while VR device is presenting.");return}Ne=F,se=ne,t.width=Math.floor(F*oe),t.height=Math.floor(ne*oe),ye===!0&&(t.style.width=F+"px",t.style.height=ne+"px"),C!==null&&C.setSize(t.width,t.height),this.setViewport(0,0,F,ne)},this.getDrawingBufferSize=function(F){return F.set(Ne*oe,se*oe).floor()},this.setDrawingBufferSize=function(F,ne,ye){Ne=F,se=ne,oe=ye,t.width=Math.floor(F*ye),t.height=Math.floor(ne*ye),this.setViewport(0,0,F,ne)},this.setEffects=function(F){if(v===Gn){gt("WebGLRenderer: setEffects() requires outputBufferType set to HalfFloatType or FloatType.");return}if(F){for(let ne=0;ne<F.length;ne++)if(F[ne].isOutputPass===!0){ht("WebGLRenderer: OutputPass is not needed in setEffects(). Tone mapping and color space conversion are applied automatically.");break}}C.setEffects(F||[])},this.getCurrentViewport=function(F){return F.copy(J)},this.getViewport=function(F){return F.copy(xe)},this.setViewport=function(F,ne,ye,fe){F.isVector4?xe.set(F.x,F.y,F.z,F.w):xe.set(F,ne,ye,fe),R.viewport(J.copy(xe).multiplyScalar(oe).round())},this.getScissor=function(F){return F.copy($)},this.setScissor=function(F,ne,ye,fe){F.isVector4?$.set(F.x,F.y,F.z,F.w):$.set(F,ne,ye,fe),R.scissor(q.copy($).multiplyScalar(oe).round())},this.getScissorTest=function(){return ue},this.setScissorTest=function(F){R.setScissorTest(ue=F)},this.setOpaqueSort=function(F){Pe=F},this.setTransparentSort=function(F){ge=F},this.getClearColor=function(F){return F.copy(Ie.getClearColor())},this.setClearColor=function(){Ie.setClearColor(...arguments)},this.getClearAlpha=function(){return Ie.getClearAlpha()},this.setClearAlpha=function(){Ie.setClearAlpha(...arguments)},this.clear=function(F=!0,ne=!0,ye=!0){let fe=0;if(F){let pe=!1;if(W!==null){let je=W.texture.format;pe=m.has(je)}if(pe){let je=W.texture.type,Ze=f.has(je),Xe=Ie.getClearColor(),it=Ie.getClearAlpha(),rt=Xe.r,Et=Xe.g,Lt=Xe.b;Ze?(y[0]=rt,y[1]=Et,y[2]=Lt,y[3]=it,Q.clearBufferuiv(Q.COLOR,0,y)):(T[0]=rt,T[1]=Et,T[2]=Lt,T[3]=it,Q.clearBufferiv(Q.COLOR,0,T))}else fe|=Q.COLOR_BUFFER_BIT}ne&&(fe|=Q.DEPTH_BUFFER_BIT,this.state.buffers.depth.setMask(!0)),ye&&(fe|=Q.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),fe!==0&&Q.clear(fe)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.setNodesHandler=function(F){F.setRenderer(this),M=F},this.dispose=function(){t.removeEventListener("webglcontextlost",P,!1),t.removeEventListener("webglcontextrestored",B,!1),t.removeEventListener("webglcontextcreationerror",K,!1),Ie.dispose(),me.dispose(),ae.dispose(),le.dispose(),De.dispose(),Y.dispose(),ke.dispose(),Ae.dispose(),he.dispose(),Qe.dispose(),Qe.removeEventListener("sessionstart",Je),Qe.removeEventListener("sessionend",kt),Bt.stop()};function P(F){F.preventDefault(),_a("WebGLRenderer: Context Lost."),I=!0}function B(){_a("WebGLRenderer: Context Restored."),I=!1;let F=re.autoReset,ne=Me.enabled,ye=Me.autoUpdate,fe=Me.needsUpdate,pe=Me.type;Ye(),re.autoReset=F,Me.enabled=ne,Me.autoUpdate=ye,Me.needsUpdate=fe,Me.type=pe}function K(F){gt("WebGLRenderer: A WebGL context could not be created. Reason: ",F.statusMessage)}function ee(F){let ne=F.target;ne.removeEventListener("dispose",ee),X(ne)}function X(F){Z(F),le.remove(F)}function Z(F){let ne=le.get(F).programs;ne!==void 0&&(ne.forEach(function(ye){he.releaseProgram(ye)}),F.isShaderMaterial&&he.releaseShaderCache(F))}this.renderBufferDirect=function(F,ne,ye,fe,pe,je){ne===null&&(ne=lt);let Ze=pe.isMesh&&pe.matrixWorld.determinantAffine()<0,Xe=Nn(F,ne,ye,fe,pe);R.setMaterial(fe,Ze);let it=ye.index,rt=1;if(fe.wireframe===!0){if(it=V.getWireframeAttribute(ye),it===void 0)return;rt=2}let Et=ye.drawRange,Lt=ye.attributes.position,$e=Et.start*rt,G=(Et.start+Et.count)*rt;je!==null&&($e=Math.max($e,je.start*rt),G=Math.min(G,(je.start+je.count)*rt)),it!==null?($e=Math.max($e,0),G=Math.min(G,it.count)):Lt!=null&&($e=Math.max($e,0),G=Math.min(G,Lt.count));let Re=G-$e;if(Re<0||Re===1/0)return;ke.setup(pe,fe,Xe,ye,it);let He,et=Te;if(it!==null&&(He=Ue.get(it),et=we,et.setIndex(He)),pe.isMesh)fe.wireframe===!0?(R.setLineWidth(fe.wireframeLinewidth*ct()),et.setMode(Q.LINES)):et.setMode(Q.TRIANGLES);else if(pe.isLine){let bt=fe.linewidth;bt===void 0&&(bt=1),R.setLineWidth(bt*ct()),pe.isLineSegments?et.setMode(Q.LINES):pe.isLineLoop?et.setMode(Q.LINE_LOOP):et.setMode(Q.LINE_STRIP)}else pe.isPoints?et.setMode(Q.POINTS):pe.isSprite&&et.setMode(Q.TRIANGLES);if(pe.isBatchedMesh)if(ft.get("WEBGL_multi_draw"))et.renderMultiDraw(pe._multiDrawStarts,pe._multiDrawCounts,pe._multiDrawCount);else{let bt=pe._multiDrawStarts,Ge=pe._multiDrawCounts,zt=pe._multiDrawCount,tt=it?Ue.get(it).bytesPerElement:1,ut=le.get(fe).currentProgram.getUniforms();for(let dt=0;dt<zt;dt++)ut.setValue(Q,"_gl_DrawID",dt),et.render(bt[dt]/tt,Ge[dt])}else if(pe.isInstancedMesh)et.renderInstances($e,Re,pe.count);else if(ye.isInstancedBufferGeometry){let bt=ye._maxInstanceCount!==void 0?ye._maxInstanceCount:1/0,Ge=Math.min(ye.instanceCount,bt);et.renderInstances($e,Re,Ge)}else et.render($e,Re)};function Ee(F,ne,ye,fe){M!==null&&F.isNodeMaterial&&M.setObject(fe,F),ie===!0&&Se.setState(F,ye,!1),F.transparent===!0&&F.side===Yt&&F.forceSinglePass===!1?(F.side=xn,F.needsUpdate=!0,fn(F,ne,fe),F.side=ii,F.needsUpdate=!0,fn(F,ne,fe),F.side=Yt):fn(F,ne,fe)}this.compile=function(F,ne,ye=null){ye===null&&(ye=F),M!==null&&M.renderStart(F,ne,ye),S=ae.get(ye),S.init(ne),b.push(S),ye.traverseVisible(function(pe){pe.isLight&&pe.layers.test(ne.layers)&&(S.pushLight(pe),pe.castShadow&&S.pushShadow(pe))}),F!==ye&&F.traverseVisible(function(pe){pe.isLight&&pe.layers.test(ne.layers)&&(S.pushLight(pe),pe.castShadow&&S.pushShadow(pe))}),S.setupLights(),M!==null&&M.updateLights(S.state.lightsArray),de=this.localClippingEnabled,ie=Se.init(this.clippingPlanes,de),ie===!0&&Se.setGlobalState(this.clippingPlanes,ne),M!==null&&Me.render(S.state.shadowsArray,ye,ne);let fe=new Set;return F.traverse(function(pe){if(!(pe.isMesh||pe.isPoints||pe.isLine||pe.isSprite))return;let je=pe.material;if(je)if(Array.isArray(je))for(let Ze=0;Ze<je.length;Ze++){let Xe=je[Ze];Ee(Xe,ye,ne,pe),fe.add(Xe)}else Ee(je,ye,ne,pe),fe.add(je)}),S=b.pop(),M!==null&&M.renderEnd(),fe},this.compileAsync=function(F,ne,ye=null){let fe=this.compile(F,ne,ye);return new Promise(pe=>{function je(){if(fe.forEach(function(Ze){let it=le.get(Ze).currentProgram;(it===void 0||it.isReady())&&fe.delete(Ze)}),fe.size===0){pe(F);return}setTimeout(je,10)}ft.get("KHR_parallel_shader_compile")!==null?je():setTimeout(je,10)})};let Be=null;function st(F){Be&&Be(F)}function Je(){Bt.stop()}function kt(){Bt.start()}let Bt=new mp;Bt.setAnimationLoop(st),typeof self<"u"&&Bt.setContext(self),this.setAnimationLoop=function(F){Be=F,Qe.setAnimationLoop(F),F===null?Bt.stop():Bt.start()},Qe.addEventListener("sessionstart",Je),Qe.addEventListener("sessionend",kt),this.render=function(F,ne){if(ne!==void 0&&ne.isCamera!==!0){gt("WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(I===!0)return;M!==null&&M.renderStart(F,ne);let ye=Qe.enabled===!0&&Qe.isPresenting===!0,fe=C!==null&&(W===null||ye)&&C.begin(w,W);if(F.matrixWorldAutoUpdate===!0&&F.updateMatrixWorld(),ne.parent===null&&ne.matrixWorldAutoUpdate===!0&&ne.updateMatrixWorld(),Qe.enabled===!0&&Qe.isPresenting===!0&&(C===null||C.isCompositing()===!1)&&(Qe.cameraAutoUpdate===!0&&Qe.updateCamera(ne),ne=Qe.getCamera()),F.isScene===!0&&F.onBeforeRender(w,F,ne,W),S=ae.get(F,b.length),S.init(ne),S.state.textureUnits=be.getTextureUnits(),b.push(S),ve.multiplyMatrices(ne.projectionMatrix,ne.matrixWorldInverse),ce.setFromProjectionMatrix(ve,fi,ne.reversedDepth),de=this.localClippingEnabled,ie=Se.init(this.clippingPlanes,de),_=me.get(F,A.length),_.init(),A.push(_),Qe.enabled===!0&&Qe.isPresenting===!0){let Ze=w.xr.getDepthSensingMesh();Ze!==null&&St(Ze,ne,-1/0,w.sortObjects)}St(F,ne,0,w.sortObjects),_.finish(),M!==null&&M.updateLights(S.state.lightsArray),w.sortObjects===!0&&_.sort(Pe,ge),nt=Qe.enabled===!1||Qe.isPresenting===!1||Qe.hasDepthSensing()===!1,nt&&Ie.addToRenderList(_,F),this.info.render.frame++,this.info.autoReset===!0&&this.info.reset(),ie===!0&&Se.beginShadows();let pe=S.state.shadowsArray;if(Me.render(pe,F,ne),ie===!0&&Se.endShadows(),(fe&&C.hasRenderPass())===!1){let Ze=_.opaque,Xe=_.transmissive;if(S.setupLights(),ne.isArrayCamera){let it=ne.cameras;if(Xe.length>0)for(let rt=0,Et=it.length;rt<Et;rt++){let Lt=it[rt];Vt(Ze,Xe,F,Lt)}nt&&Ie.render(F);for(let rt=0,Et=it.length;rt<Et;rt++){let Lt=it[rt];Mn(_,F,Lt,Lt.viewport)}}else Xe.length>0&&Vt(Ze,Xe,F,ne),nt&&Ie.render(F),Mn(_,F,ne)}W!==null&&L===0&&(be.updateMultisampleRenderTarget(W),be.updateRenderTargetMipmap(W)),fe&&C.end(w),F.isScene===!0&&F.onAfterRender(w,F,ne),ke.resetDefaultState(),z=-1,E=null,b.pop(),b.length>0?(S=b[b.length-1],be.setTextureUnits(S.state.textureUnits),ie===!0&&Se.setGlobalState(w.clippingPlanes,S.state.camera)):S=null,A.pop(),A.length>0?_=A[A.length-1]:_=null,M!==null&&M.renderEnd()};function St(F,ne,ye,fe){if(F.visible===!1)return;if(F.layers.test(ne.layers)){if(F.isGroup)ye=F.renderOrder;else if(F.isLOD)F.autoUpdate===!0&&F.update(ne);else if(F.isLightProbeGrid)S.pushLightProbeGrid(F);else if(F.isLight)S.pushLight(F),F.castShadow&&S.pushShadow(F);else if(F.isSprite){if(!F.frustumCulled||F.intersectsFrustum(ce)){fe&&Ve.setFromMatrixPosition(F.matrixWorld).applyMatrix4(ve);let Ze=Y.update(F),Xe=F.material;Xe.visible&&_.push(F,Ze,Xe,ye,Ve.z,null,ne)}}else if((F.isMesh||F.isLine||F.isPoints)&&(!F.frustumCulled||F.intersectsFrustum(ce))){let Ze=Y.update(F),Xe=F.material;if(fe&&(F.boundingSphere!==void 0?(F.boundingSphere===null&&F.computeBoundingSphere(),Ve.copy(F.boundingSphere.center)):(Ze.boundingSphere===null&&Ze.computeBoundingSphere(),Ve.copy(Ze.boundingSphere.center)),Ve.applyMatrix4(F.matrixWorld).applyMatrix4(ve)),Array.isArray(Xe)){let it=Ze.groups;for(let rt=0,Et=it.length;rt<Et;rt++){let Lt=it[rt],$e=Xe[Lt.materialIndex];$e&&$e.visible&&_.push(F,Ze,$e,ye,Ve.z,Lt,ne)}}else Xe.visible&&_.push(F,Ze,Xe,ye,Ve.z,null,ne)}}let je=F.children;for(let Ze=0,Xe=je.length;Ze<Xe;Ze++)St(je[Ze],ne,ye,fe)}function Mn(F,ne,ye,fe){let{opaque:pe,transmissive:je,transparent:Ze}=F;S.setupLightsView(ye),ie===!0&&Se.setGlobalState(w.clippingPlanes,ye),fe&&R.viewport(J.copy(fe)),pe.length>0&&nn(pe,ne,ye),je.length>0&&nn(je,ne,ye),Ze.length>0&&nn(Ze,ne,ye),R.buffers.depth.setTest(!0),R.buffers.depth.setMask(!0),R.buffers.color.setMask(!0),R.setPolygonOffset(!1)}function Vt(F,ne,ye,fe){if((ye.isScene===!0?ye.overrideMaterial:null)!==null)return;if(S.state.transmissionRenderTarget[fe.id]===void 0){let $e=ft.has("EXT_color_buffer_half_float")||ft.has("EXT_color_buffer_float");S.state.transmissionRenderTarget[fe.id]=new an(1,1,{generateMipmaps:!0,type:$e?gn:Gn,minFilter:_i,samples:Math.max(4,O.samples),stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,storeMultisampledDepthBuffer:!1,storeMultisampledStencilBuffer:!1,colorSpace:Rt.workingColorSpace})}let je=S.state.transmissionRenderTarget[fe.id],Ze=fe.viewport||J;je.setSize(Ze.z*w.transmissionResolutionScale,Ze.w*w.transmissionResolutionScale);let Xe=w.getRenderTarget(),it=w.getActiveCubeFace(),rt=w.getActiveMipmapLevel();w.setRenderTarget(je),w.getClearColor(Le),Fe=w.getClearAlpha(),Fe<1&&w.setClearColor(16777215,.5),w.clear(),nt&&Ie.render(ye);let Et=w.toneMapping;w.toneMapping=vi;let Lt=fe.viewport;if(fe.viewport!==void 0&&(fe.viewport=void 0),S.setupLightsView(fe),ie===!0&&Se.setGlobalState(w.clippingPlanes,fe),nn(F,ye,fe),be.updateMultisampleRenderTarget(je),be.updateRenderTargetMipmap(je),ft.has("WEBGL_multisampled_render_to_texture")===!1){let $e=!1;for(let G=0,Re=ne.length;G<Re;G++){let He=ne[G],{object:et,geometry:bt,material:Ge,group:zt}=He;if(Ge.side===Yt&&et.layers.test(fe.layers)){let tt=Ge.side;Ge.side=xn,Ge.needsUpdate=!0,en(et,ye,fe,bt,Ge,zt),Ge.side=tt,Ge.needsUpdate=!0,$e=!0}}$e===!0&&(be.updateMultisampleRenderTarget(je),be.updateRenderTargetMipmap(je))}w.setRenderTarget(Xe,it,rt),w.setClearColor(Le,Fe),Lt!==void 0&&(fe.viewport=Lt),w.toneMapping=Et}function nn(F,ne,ye){let fe=ne.isScene===!0?ne.overrideMaterial:null;for(let pe=0,je=F.length;pe<je;pe++){let Ze=F[pe],{object:Xe,geometry:it,group:rt}=Ze,Et=Ze.material;Et.allowOverride===!0&&fe!==null&&(Et=fe),Xe.layers.test(ye.layers)&&en(Xe,ne,ye,it,Et,rt)}}function en(F,ne,ye,fe,pe,je){M!==null&&pe.isNodeMaterial&&M.setObject(F,pe),F.onBeforeRender(w,ne,ye,fe,pe,je),F.modelViewMatrix.multiplyMatrices(ye.matrixWorldInverse,F.matrixWorld),F.normalMatrix.getNormalMatrix(F.modelViewMatrix),pe.onBeforeRender(w,ne,ye,fe,F,je),pe.transparent===!0&&pe.side===Yt&&pe.forceSinglePass===!1?(pe.side=xn,pe.needsUpdate=!0,w.renderBufferDirect(ye,ne,fe,pe,F,je),pe.side=ii,pe.needsUpdate=!0,w.renderBufferDirect(ye,ne,fe,pe,F,je),pe.side=Yt):w.renderBufferDirect(ye,ne,fe,pe,F,je),F.onAfterRender(w,ne,ye,fe,pe,je)}function fn(F,ne,ye){ne.isScene!==!0&&(ne=lt);let fe=le.get(F),pe=S.state.lights,je=S.state.shadowsArray,Ze=pe.state.version,Xe=he.getParameters(F,pe.state,je,ne,ye,S.state.lightProbeGridArray),it=he.getProgramCacheKey(Xe),rt=fe.programs;fe.environment=F.isMeshStandardMaterial||F.isMeshLambertMaterial||F.isMeshPhongMaterial?ne.environment:null,fe.fog=ne.fog;let Et=F.isMeshStandardMaterial||F.isMeshLambertMaterial&&!F.envMap||F.isMeshPhongMaterial&&!F.envMap;fe.envMap=De.get(F.envMap||fe.environment,Et),fe.envMapRotation=fe.environment!==null&&F.envMap===null?ne.environmentRotation:F.envMapRotation,rt===void 0&&(F.addEventListener("dispose",ee),rt=new Map,fe.programs=rt);let Lt=rt.get(it);if(Lt!==void 0){if(fe.currentProgram===Lt&&fe.lightsStateVersion===Ze)return wn(F,Xe),Lt}else Xe.uniforms=he.getUniforms(F),M!==null&&F.isNodeMaterial&&M.build(F,ye,Xe),F.onBeforeCompile(Xe,w),Lt=he.acquireProgram(Xe,it),rt.set(it,Lt),fe.uniforms=Xe.uniforms;let $e=fe.uniforms;return(!F.isShaderMaterial&&!F.isRawShaderMaterial||F.clipping===!0)&&($e.clippingPlanes=Se.uniform),wn(F,Xe),fe.needsLights=ra(F),fe.lightsStateVersion=Ze,fe.needsLights&&($e.ambientLightColor.value=pe.state.ambient,$e.lightProbe.value=pe.state.probe,$e.sunLights.value=pe.state.sun,$e.sunLightShadows.value=pe.state.sunShadow,$e.directionalLights.value=pe.state.directional,$e.directionalLightShadows.value=pe.state.directionalShadow,$e.spotLights.value=pe.state.spot,$e.spotLightShadows.value=pe.state.spotShadow,$e.rectAreaLights.value=pe.state.rectArea,$e.ltc_1.value=pe.state.rectAreaLTC1,$e.ltc_2.value=pe.state.rectAreaLTC2,$e.pointLights.value=pe.state.point,$e.pointLightShadows.value=pe.state.pointShadow,$e.hemisphereLights.value=pe.state.hemi,$e.sunShadowMatrix.value=pe.state.sunShadowMatrix,$e.sunShadowCascade.value=pe.state.sunShadowCascade,$e.directionalShadowMatrix.value=pe.state.directionalShadowMatrix,$e.spotLightMatrix.value=pe.state.spotLightMatrix,$e.spotLightMap.value=pe.state.spotLightMap,$e.pointShadowMatrix.value=pe.state.pointShadowMatrix),fe.lightProbeGrid=S.state.lightProbeGridArray.length>0,fe.currentProgram=Lt,fe.uniformsList=null,Lt}function ss(F){if(F.uniformsList===null){let ne=F.currentProgram.getUniforms();F.uniformsList=jr.seqWithValue(ne.seq,F.uniforms)}return F.uniformsList}function wn(F,ne){let ye=le.get(F);ye.outputColorSpace=ne.outputColorSpace,ye.batching=ne.batching,ye.batchingColor=ne.batchingColor,ye.instancing=ne.instancing,ye.instancingColor=ne.instancingColor,ye.instancingMorph=ne.instancingMorph,ye.skinning=ne.skinning,ye.morphTargets=ne.morphTargets,ye.morphNormals=ne.morphNormals,ye.morphColors=ne.morphColors,ye.morphTargetsCount=ne.morphTargetsCount,ye.numClippingPlanes=ne.numClippingPlanes,ye.numIntersection=ne.numClipIntersection,ye.vertexAlphas=ne.vertexAlphas,ye.vertexTangents=ne.vertexTangents,ye.toneMapping=ne.toneMapping}function rs(F,ne){if(F.length===0)return null;if(F.length===1)return F[0].texture!==null?F[0]:null;g.setFromMatrixPosition(ne.matrixWorld);for(let ye=0,fe=F.length;ye<fe;ye++){let pe=F[ye];if(pe.texture!==null&&pe.boundingBox.containsPoint(g))return pe}return null}function Nn(F,ne,ye,fe,pe){ne.isScene!==!0&&(ne=lt),be.resetTextureUnits();let je=ne.fog,Ze=fe.isMeshStandardMaterial||fe.isMeshLambertMaterial||fe.isMeshPhongMaterial?ne.environment:null,Xe=W===null?w.outputColorSpace:W.isXRRenderTarget===!0?W.texture.colorSpace:Rt.workingColorSpace,it=fe.isMeshStandardMaterial||fe.isMeshLambertMaterial&&!fe.envMap||fe.isMeshPhongMaterial&&!fe.envMap,rt=De.get(fe.envMap||Ze,it),Et=fe.vertexColors===!0&&!!ye.attributes.color&&ye.attributes.color.itemSize===4,Lt=!!ye.attributes.tangent&&(!!fe.normalMap||fe.anisotropy>0),$e=!!ye.morphAttributes.position,G=!!ye.morphAttributes.normal,Re=!!ye.morphAttributes.color,He=vi;fe.toneMapped&&(W===null||W.isXRRenderTarget===!0)&&(He=w.toneMapping);let et=ye.morphAttributes.position||ye.morphAttributes.normal||ye.morphAttributes.color,bt=et!==void 0?et.length:0,Ge=le.get(fe),zt=S.state.lights;if(ie===!0&&(de===!0||F!==E)){let qe=F===E&&fe.id===z;Se.setState(fe,F,qe)}let tt=!1;fe.version===Ge.__version?(Ge.needsLights&&Ge.lightsStateVersion!==zt.state.version||Ge.outputColorSpace!==Xe||pe.isBatchedMesh&&Ge.batching===!1||!pe.isBatchedMesh&&Ge.batching===!0||pe.isBatchedMesh&&Ge.batchingColor===!0&&pe._colorsTexture===null||pe.isBatchedMesh&&Ge.batchingColor===!1&&pe._colorsTexture!==null||pe.isInstancedMesh&&Ge.instancing===!1||!pe.isInstancedMesh&&Ge.instancing===!0||pe.isSkinnedMesh&&Ge.skinning===!1||!pe.isSkinnedMesh&&Ge.skinning===!0||pe.isInstancedMesh&&Ge.instancingColor===!0&&pe.instanceColor===null||pe.isInstancedMesh&&Ge.instancingColor===!1&&pe.instanceColor!==null||pe.isInstancedMesh&&Ge.instancingMorph===!0&&pe.morphTexture===null||pe.isInstancedMesh&&Ge.instancingMorph===!1&&pe.morphTexture!==null||Ge.envMap!==rt||fe.fog===!0&&Ge.fog!==je||Ge.numClippingPlanes!==void 0&&(Ge.numClippingPlanes!==Se.numPlanes||Ge.numIntersection!==Se.numIntersection)||Ge.vertexAlphas!==Et||Ge.vertexTangents!==Lt||Ge.morphTargets!==$e||Ge.morphNormals!==G||Ge.morphColors!==Re||Ge.toneMapping!==He||Ge.morphTargetsCount!==bt||!!Ge.lightProbeGrid!=S.state.lightProbeGridArray.length>0)&&(tt=!0):(tt=!0,Ge.__version=fe.version);let ut=Ge.currentProgram;tt===!0&&(ut=fn(fe,ne,pe),M&&fe.isNodeMaterial&&M.onUpdateProgram(fe,ut,Ge));let dt=!1,At=!1,vt=!1,at=ut.getUniforms(),Ft=Ge.uniforms;if(R.useProgram(ut.program)&&(dt=!0,At=!0,vt=!0),fe.id!==z&&(z=fe.id,At=!0),Ge.needsLights){let qe=rs(S.state.lightProbeGridArray,pe);Ge.lightProbeGrid!==qe&&(Ge.lightProbeGrid=qe,At=!0)}if(dt||E!==F){R.buffers.depth.getReversed()&&F.reversedDepth!==!0&&(F._reversedDepth=!0,F.updateProjectionMatrix()),at.setValue(Q,"projectionMatrix",F.projectionMatrix),at.setValue(Q,"viewMatrix",F.matrixWorldInverse);let on=at.map.cameraPosition;on!==void 0&&on.setValue(Q,We.setFromMatrixPosition(F.matrixWorld)),O.logarithmicDepthBuffer&&at.setValue(Q,"logDepthBufFC",2/(Math.log(F.far+1)/Math.LN2)),(fe.isMeshPhongMaterial||fe.isMeshToonMaterial||fe.isMeshLambertMaterial||fe.isMeshBasicMaterial||fe.isMeshStandardMaterial||fe.isShaderMaterial)&&at.setValue(Q,"isOrthographic",F.isOrthographicCamera===!0),E!==F&&(E=F,At=!0,vt=!0)}if(Ge.needsLights&&(zt.state.sunShadowMap.length>0&&at.setValue(Q,"sunShadowMap",zt.state.sunShadowMap,be),zt.state.directionalShadowMap.length>0&&at.setValue(Q,"directionalShadowMap",zt.state.directionalShadowMap,be),zt.state.spotShadowMap.length>0&&at.setValue(Q,"spotShadowMap",zt.state.spotShadowMap,be),zt.state.pointShadowMap.length>0&&at.setValue(Q,"pointShadowMap",zt.state.pointShadowMap,be)),pe.isSkinnedMesh){at.setOptional(Q,pe,"bindMatrix"),at.setOptional(Q,pe,"bindMatrixInverse");let qe=pe.skeleton;qe&&(qe.boneTexture===null&&qe.computeBoneTexture(),at.setValue(Q,"boneTexture",qe.boneTexture,be))}pe.isBatchedMesh&&(at.setOptional(Q,pe,"batchingTexture"),at.setValue(Q,"batchingTexture",pe._matricesTexture,be),at.setOptional(Q,pe,"batchingIdTexture"),at.setValue(Q,"batchingIdTexture",pe._indirectTexture,be),at.setOptional(Q,pe,"batchingColorTexture"),pe._colorsTexture!==null&&at.setValue(Q,"batchingColorTexture",pe._colorsTexture,be));let Zt=ye.morphAttributes;if((Zt.position!==void 0||Zt.normal!==void 0||Zt.color!==void 0)&&j.update(pe,ye,ut),(At||Ge.receiveShadow!==pe.receiveShadow)&&(Ge.receiveShadow=pe.receiveShadow,at.setValue(Q,"receiveShadow",pe.receiveShadow)),(fe.isMeshStandardMaterial||fe.isMeshLambertMaterial||fe.isMeshPhongMaterial)&&fe.envMap===null&&ne.environment!==null&&(Ft.envMapIntensity.value=ne.environmentIntensity),Ft.dfgLUT!==void 0&&(Ft.dfgLUT.value=Y_()),At){if(at.setValue(Q,"toneMappingExposure",w.toneMappingExposure),Ge.needsLights&&Wn(Ft,vt),je&&fe.fog===!0&&_e.refreshFogUniforms(Ft,je),_e.refreshMaterialUniforms(Ft,fe,oe,se,S.state.transmissionRenderTarget[F.id]),Ge.needsLights&&Ge.lightProbeGrid){let qe=Ge.lightProbeGrid;Ft.probesSH.value=qe.texture,Ft.probesMin.value.copy(qe.boundingBox.min),Ft.probesMax.value.copy(qe.boundingBox.max),Ft.probesResolution.value.copy(qe.resolution)}jr.upload(Q,ss(Ge),Ft,be)}if(fe.isShaderMaterial&&fe.uniformsNeedUpdate===!0&&(jr.upload(Q,ss(Ge),Ft,be),fe.uniformsNeedUpdate=!1),fe.isSpriteMaterial&&at.setValue(Q,"center",pe.center),at.setValue(Q,"modelViewMatrix",pe.modelViewMatrix),at.setValue(Q,"normalMatrix",pe.normalMatrix),at.setValue(Q,"modelMatrix",pe.matrixWorld),fe.uniformsGroups!==void 0){let qe=fe.uniformsGroups;for(let on=0,qn=qe.length;on<qn;on++){let li=qe[on];Ae.update(li,ut),Ae.bind(li,ut)}}return ut}function Wn(F,ne){F.ambientLightColor.needsUpdate=ne,F.lightProbe.needsUpdate=ne,F.sunLights.needsUpdate=ne,F.sunLightShadows.needsUpdate=ne,F.directionalLights.needsUpdate=ne,F.directionalLightShadows.needsUpdate=ne,F.pointLights.needsUpdate=ne,F.pointLightShadows.needsUpdate=ne,F.spotLights.needsUpdate=ne,F.spotLightShadows.needsUpdate=ne,F.rectAreaLights.needsUpdate=ne,F.hemisphereLights.needsUpdate=ne}function ra(F){return F.isMeshLambertMaterial||F.isMeshToonMaterial||F.isMeshPhongMaterial||F.isMeshStandardMaterial||F.isShadowMaterial||F.isShaderMaterial&&F.lights===!0}this.getActiveCubeFace=function(){return H},this.getActiveMipmapLevel=function(){return L},this.getRenderTarget=function(){return W},this.setRenderTargetTextures=function(F,ne,ye){let fe=le.get(F);fe.__autoAllocateDepthBuffer=F.resolveDepthBuffer===!1,fe.__autoAllocateDepthBuffer===!1&&(fe.__useRenderToTexture=!1),le.get(F.texture).__webglTexture=ne,le.get(F.depthTexture).__webglTexture=fe.__autoAllocateDepthBuffer?void 0:ye,fe.__hasExternalTextures=!0},this.setRenderTargetFramebuffer=function(F,ne){let ye=le.get(F);ye.__webglFramebuffer=ne,ye.__useDefaultFramebuffer=ne===void 0},this.setRenderTarget=function(F,ne=0,ye=0){W=F,H=ne,L=ye;let fe=null,pe=!1,je=!1;if(F){let Xe=le.get(F);if(Xe.__useDefaultFramebuffer!==void 0){R.bindFramebuffer(Q.FRAMEBUFFER,Xe.__webglFramebuffer),J.copy(F.viewport),q.copy(F.scissor),te=F.scissorTest,R.viewport(J),R.scissor(q),R.setScissorTest(te),z=-1;return}else if(Xe.__webglFramebuffer===void 0)be.setupRenderTarget(F);else if(Xe.__hasExternalTextures)be.rebindTextures(F,le.get(F.texture).__webglTexture,le.get(F.depthTexture).__webglTexture);else if(F.depthBuffer){let Et=F.depthTexture;if(Xe.__boundDepthTexture!==Et){if(Et!==null&&le.has(Et)&&(F.width!==Et.image.width||F.height!==Et.image.height))throw new Error("THREE.WebGLRenderer: Attached DepthTexture is initialized to the incorrect size.");be.setupDepthRenderbuffer(F)}}let it=F.texture;(it.isData3DTexture||it.isDataArrayTexture||it.isCompressedArrayTexture)&&(je=!0);let rt=le.get(F).__webglFramebuffer;F.isWebGLCubeRenderTarget?(Array.isArray(rt[ne])?fe=rt[ne][ye]:fe=rt[ne],pe=!0):F.samples>0&&be.useMultisampledRTT(F)===!1?fe=le.get(F).__webglMultisampledFramebuffer:Array.isArray(rt)?fe=rt[ye]:fe=rt,J.copy(F.viewport),q.copy(F.scissor),te=F.scissorTest}else J.copy(xe).multiplyScalar(oe).floor(),q.copy($).multiplyScalar(oe).floor(),te=ue;if(ye!==0&&(fe=N),R.bindFramebuffer(Q.FRAMEBUFFER,fe)&&R.drawBuffers(F,fe),R.viewport(J),R.scissor(q),R.setScissorTest(te),pe){let Xe=le.get(F.texture);Q.framebufferTexture2D(Q.FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_CUBE_MAP_POSITIVE_X+ne,Xe.__webglTexture,ye)}else if(je){let Xe=ne;for(let it=0;it<F.textures.length;it++){let rt=le.get(F.textures[it]);Q.framebufferTextureLayer(Q.FRAMEBUFFER,Q.COLOR_ATTACHMENT0+it,rt.__webglTexture,ye,Xe)}}else if(F!==null&&ye!==0){let Xe=le.get(F.texture);Q.framebufferTexture2D(Q.FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_2D,Xe.__webglTexture,ye)}z=-1};function as(F){let ne=le.get(F);return(ne.__readFormat!==F.format||ne.__readType!==F.type)&&(ne.__readFormat=F.format,ne.__readType=F.type,ne.__formatReadable=O.textureFormatReadable(F.format),ne.__typeReadable=O.textureTypeReadable(F.type)),ne}this.readRenderTargetPixels=function(F,ne,ye,fe,pe,je,Ze,Xe=0){if(!(F&&F.isWebGLRenderTarget)){gt("WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let it=le.get(F).__webglFramebuffer;if(F.isWebGLCubeRenderTarget&&Ze!==void 0&&(it=it[Ze]),it){R.bindFramebuffer(Q.FRAMEBUFFER,it);try{let rt=F.textures[Xe],Et=rt.format,Lt=rt.type;F.textures.length>1&&Q.readBuffer(Q.COLOR_ATTACHMENT0+Xe);let $e=as(rt);if($e.__formatReadable===!1){gt("WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if($e.__typeReadable===!1){gt("WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}ne>=0&&ne<=F.width-fe&&ye>=0&&ye<=F.height-pe&&Q.readPixels(ne,ye,fe,pe,Oe.convert(Et),Oe.convert(Lt),je)}finally{let rt=W!==null?le.get(W).__webglFramebuffer:null;R.bindFramebuffer(Q.FRAMEBUFFER,rt)}}},this.readRenderTargetPixelsAsync=async function(F,ne,ye,fe,pe,je,Ze,Xe=0){if(!(F&&F.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let it=le.get(F).__webglFramebuffer;if(F.isWebGLCubeRenderTarget&&Ze!==void 0&&(it=it[Ze]),it)if(ne>=0&&ne<=F.width-fe&&ye>=0&&ye<=F.height-pe){R.bindFramebuffer(Q.FRAMEBUFFER,it);let rt=F.textures[Xe],Et=rt.format,Lt=rt.type;F.textures.length>1&&Q.readBuffer(Q.COLOR_ATTACHMENT0+Xe);let $e=as(rt);if($e.__formatReadable===!1)throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if($e.__typeReadable===!1)throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");let G=Q.createBuffer();Q.bindBuffer(Q.PIXEL_PACK_BUFFER,G),Q.bufferData(Q.PIXEL_PACK_BUFFER,je.byteLength,Q.STREAM_READ),Q.readPixels(ne,ye,fe,pe,Oe.convert(Et),Oe.convert(Lt),0),Q.bindBuffer(Q.PIXEL_PACK_BUFFER,null);let Re=W!==null?le.get(W).__webglFramebuffer:null;R.bindFramebuffer(Q.FRAMEBUFFER,Re);let He=Q.fenceSync(Q.SYNC_GPU_COMMANDS_COMPLETE,0);return Q.flush(),await Vf(Q,He,4),Q.bindBuffer(Q.PIXEL_PACK_BUFFER,G),Q.getBufferSubData(Q.PIXEL_PACK_BUFFER,0,je),Q.bindBuffer(Q.PIXEL_PACK_BUFFER,null),Q.deleteBuffer(G),Q.deleteSync(He),je}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")},this.copyFramebufferToTexture=function(F,ne=null,ye=0){let fe=Math.pow(2,-ye),pe=Math.floor(F.image.width*fe),je=Math.floor(F.image.height*fe),Ze=ne!==null?ne.x:0,Xe=ne!==null?ne.y:0;be.setTexture2D(F,0),Q.copyTexSubImage2D(Q.TEXTURE_2D,ye,0,0,Ze,Xe,pe,je),R.unbindTexture()},this.copyTextureToTexture=function(F,ne,ye=null,fe=null,pe=0,je=0){let Ze,Xe,it,rt,Et,Lt,$e,G,Re,He=F.isCompressedTexture?F.mipmaps[je]:F.image;if(ye!==null)Ze=ye.max.x-ye.min.x,Xe=ye.max.y-ye.min.y,it=ye.isBox3?ye.max.z-ye.min.z:1,rt=ye.min.x,Et=ye.min.y,Lt=ye.isBox3?ye.min.z:0;else{let Ft=Math.pow(2,-pe);Ze=Math.floor(He.width*Ft),Xe=Math.floor(He.height*Ft),F.isDataArrayTexture?it=He.depth:F.isData3DTexture?it=Math.floor(He.depth*Ft):it=1,rt=0,Et=0,Lt=0}fe!==null?($e=fe.x,G=fe.y,Re=fe.z):($e=0,G=0,Re=0);let et=Oe.convert(ne.format),bt=Oe.convert(ne.type),Ge;ne.isData3DTexture?(be.setTexture3D(ne,0),Ge=Q.TEXTURE_3D):ne.isDataArrayTexture||ne.isCompressedArrayTexture?(be.setTexture2DArray(ne,0),Ge=Q.TEXTURE_2D_ARRAY):(be.setTexture2D(ne,0),Ge=Q.TEXTURE_2D),R.activeTexture(Q.TEXTURE0),R.pixelStorei(Q.UNPACK_FLIP_Y_WEBGL,ne.flipY),R.pixelStorei(Q.UNPACK_PREMULTIPLY_ALPHA_WEBGL,ne.premultiplyAlpha),R.pixelStorei(Q.UNPACK_ALIGNMENT,ne.unpackAlignment);let zt=R.getParameter(Q.UNPACK_ROW_LENGTH),tt=R.getParameter(Q.UNPACK_IMAGE_HEIGHT),ut=R.getParameter(Q.UNPACK_SKIP_PIXELS),dt=R.getParameter(Q.UNPACK_SKIP_ROWS),At=R.getParameter(Q.UNPACK_SKIP_IMAGES);R.pixelStorei(Q.UNPACK_ROW_LENGTH,He.width),R.pixelStorei(Q.UNPACK_IMAGE_HEIGHT,He.height),R.pixelStorei(Q.UNPACK_SKIP_PIXELS,rt),R.pixelStorei(Q.UNPACK_SKIP_ROWS,Et),R.pixelStorei(Q.UNPACK_SKIP_IMAGES,Lt);let vt=F.isDataArrayTexture||F.isData3DTexture,at=ne.isDataArrayTexture||ne.isData3DTexture;if(F.isDepthTexture){let Ft=le.get(F),Zt=le.get(ne),qe=le.get(Ft.__renderTarget),on=le.get(Zt.__renderTarget);R.bindFramebuffer(Q.READ_FRAMEBUFFER,qe.__webglFramebuffer),R.bindFramebuffer(Q.DRAW_FRAMEBUFFER,on.__webglFramebuffer);for(let qn=0;qn<it;qn++)vt&&(Q.framebufferTextureLayer(Q.READ_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,le.get(F).__webglTexture,pe,Lt+qn),Q.framebufferTextureLayer(Q.DRAW_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,le.get(ne).__webglTexture,je,Re+qn)),Q.blitFramebuffer(rt,Et,Ze,Xe,$e,G,Ze,Xe,Q.DEPTH_BUFFER_BIT,Q.NEAREST);R.bindFramebuffer(Q.READ_FRAMEBUFFER,null),R.bindFramebuffer(Q.DRAW_FRAMEBUFFER,null)}else if(pe!==0||F.isRenderTargetTexture||le.has(F)){let Ft=le.get(F),Zt=le.get(ne);R.bindFramebuffer(Q.READ_FRAMEBUFFER,D),R.bindFramebuffer(Q.DRAW_FRAMEBUFFER,U);for(let qe=0;qe<it;qe++)vt?Q.framebufferTextureLayer(Q.READ_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Ft.__webglTexture,pe,Lt+qe):Q.framebufferTexture2D(Q.READ_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_2D,Ft.__webglTexture,pe),at?Q.framebufferTextureLayer(Q.DRAW_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Zt.__webglTexture,je,Re+qe):Q.framebufferTexture2D(Q.DRAW_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_2D,Zt.__webglTexture,je),pe!==0?Q.blitFramebuffer(rt,Et,Ze,Xe,$e,G,Ze,Xe,Q.COLOR_BUFFER_BIT,Q.NEAREST):at?Q.copyTexSubImage3D(Ge,je,$e,G,Re+qe,rt,Et,Ze,Xe):Q.copyTexSubImage2D(Ge,je,$e,G,rt,Et,Ze,Xe);R.bindFramebuffer(Q.READ_FRAMEBUFFER,null),R.bindFramebuffer(Q.DRAW_FRAMEBUFFER,null)}else at?F.isDataTexture||F.isData3DTexture?Q.texSubImage3D(Ge,je,$e,G,Re,Ze,Xe,it,et,bt,He.data):ne.isCompressedArrayTexture?Q.compressedTexSubImage3D(Ge,je,$e,G,Re,Ze,Xe,it,et,He.data):Q.texSubImage3D(Ge,je,$e,G,Re,Ze,Xe,it,et,bt,He):F.isDataTexture?Q.texSubImage2D(Q.TEXTURE_2D,je,$e,G,Ze,Xe,et,bt,He.data):F.isCompressedTexture?Q.compressedTexSubImage2D(Q.TEXTURE_2D,je,$e,G,He.width,He.height,et,He.data):Q.texSubImage2D(Q.TEXTURE_2D,je,$e,G,Ze,Xe,et,bt,He);R.pixelStorei(Q.UNPACK_ROW_LENGTH,zt),R.pixelStorei(Q.UNPACK_IMAGE_HEIGHT,tt),R.pixelStorei(Q.UNPACK_SKIP_PIXELS,ut),R.pixelStorei(Q.UNPACK_SKIP_ROWS,dt),R.pixelStorei(Q.UNPACK_SKIP_IMAGES,At),je===0&&ne.generateMipmaps&&Q.generateMipmap(Ge),R.unbindTexture()},this.initRenderTarget=function(F){le.get(F).__webglFramebuffer===void 0&&be.setupRenderTarget(F)},this.initTexture=function(F){F.isCubeTexture?be.setTextureCube(F,0):F.isData3DTexture?be.setTexture3D(F,0):F.isDataArrayTexture||F.isCompressedArrayTexture?be.setTexture2DArray(F,0):be.setTexture2D(F,0),R.unbindTexture()},this.resetState=function(){H=0,L=0,W=null,R.reset(),ke.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return fi}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(e){this._outputColorSpace=e;let t=this.getContext();t.drawingBufferColorSpace=Rt._getDrawingBufferColorSpace(e),t.unpackColorSpace=Rt._getUnpackColorSpace()}};function pu(s,e){if(e===Wh)return console.warn("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Geometry already defined as triangles."),s;if(e===Wr||e===ro){let t=s.getIndex();if(t===null){let r=[],a=s.getAttribute("position");if(a!==void 0){for(let o=0;o<a.count;o++)r.push(o);s.setIndex(r),t=s.getIndex()}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Undefined position attribute. Processing not possible."),s}let n=t.count-2,i=[];if(e===Wr)for(let r=1;r<=n;r++)i.push(t.getX(0)),i.push(t.getX(r)),i.push(t.getX(r+1));else for(let r=0;r<n;r++)r%2===0?(i.push(t.getX(r)),i.push(t.getX(r+1)),i.push(t.getX(r+2))):(i.push(t.getX(r+2)),i.push(t.getX(r+1)),i.push(t.getX(r)));return i.length/3!==n&&console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unable to generate correct amount of triangles."),s.setIndex(i),s.clearGroups(),s}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unknown draw mode:",e),s}function Mp(s){let e=new Map,t=new Map,n=s.clone();return wp(s,n,function(i,r){e.set(r,i),t.set(i,r)}),n.traverse(function(i){if(!i.isSkinnedMesh)return;let r=i,a=e.get(i),o=a.skeleton.bones;r.skeleton=a.skeleton.clone(),r.bindMatrix.copy(a.bindMatrix),r.skeleton.bones=o.map(function(l){return t.get(l)}),r.bind(r.skeleton,r.bindMatrix)}),n}function wp(s,e,t){t(s,e);for(let n=0;n<s.children.length;n++)wp(s.children[n],e.children[n],t)}var wi=class extends Li{constructor(e){super(e),this.dracoLoader=null,this.ktx2Loader=null,this.meshoptDecoder=null,this.pluginCallbacks=[],this.register(function(t){return new yu(t)}),this.register(function(t){return new Mu(t)}),this.register(function(t){return new Iu(t)}),this.register(function(t){return new Du(t)}),this.register(function(t){return new Lu(t)}),this.register(function(t){return new Su(t)}),this.register(function(t){return new Eu(t)}),this.register(function(t){return new Tu(t)}),this.register(function(t){return new Au(t)}),this.register(function(t){return new _u(t)}),this.register(function(t){return new Ru(t)}),this.register(function(t){return new wu(t)}),this.register(function(t){return new Pu(t)}),this.register(function(t){return new Cu(t)}),this.register(function(t){return new bu(t)}),this.register(function(t){return new Tc(t,Ut.EXT_MESHOPT_COMPRESSION)}),this.register(function(t){return new Tc(t,Ut.KHR_MESHOPT_COMPRESSION)}),this.register(function(t){return new Fu(t)})}load(e,t,n,i){let r=this,a;if(this.resourcePath!=="")a=this.resourcePath;else if(this.path!==""){let c=es.extractUrlBase(e);a=es.resolveURL(c,this.path)}else a=es.extractUrlBase(e);this.manager.itemStart(e);let o=function(c){i?i(c):console.error(c),r.manager.itemError(e),r.manager.itemEnd(e)},l=new Ur(this.manager);l.setPath(this.path),l.setResponseType("arraybuffer"),l.setRequestHeader(this.requestHeader),l.setWithCredentials(this.withCredentials),l.load(e,function(c){try{r.parse(c,a,function(h){t(h),r.manager.itemEnd(e)},o)}catch(h){o(h)}},n,o)}setDRACOLoader(e){return this.dracoLoader=e,this}setKTX2Loader(e){return this.ktx2Loader=e,this}setMeshoptDecoder(e){return this.meshoptDecoder=e,this}register(e){return this.pluginCallbacks.indexOf(e)===-1&&this.pluginCallbacks.push(e),this}unregister(e){return this.pluginCallbacks.indexOf(e)!==-1&&this.pluginCallbacks.splice(this.pluginCallbacks.indexOf(e),1),this}parse(e,t,n,i){let r,a={},o={},l=new TextDecoder;if(typeof e=="string")r=JSON.parse(e);else if(e instanceof ArrayBuffer)if(l.decode(new Uint8Array(e,0,4))===Rp){try{a[Ut.KHR_BINARY_GLTF]=new Nu(e)}catch(d){i&&i(d);return}r=JSON.parse(a[Ut.KHR_BINARY_GLTF].content)}else r=JSON.parse(l.decode(e));else r=e;if(r.asset===void 0||r.asset.version[0]<2){i&&i(new Error("THREE.GLTFLoader: Unsupported asset. glTF versions >=2.0 are supported."));return}let c=new Gu(r,{path:t||this.resourcePath||"",crossOrigin:this.crossOrigin,requestHeader:this.requestHeader,manager:this.manager,ktx2Loader:this.ktx2Loader,meshoptDecoder:this.meshoptDecoder});c.fileLoader.setRequestHeader(this.requestHeader);for(let h=0;h<this.pluginCallbacks.length;h++){let d=this.pluginCallbacks[h](c);d.name||console.error("THREE.GLTFLoader: Invalid plugin found: missing name"),o[d.name]=d,a[d.name]=!0}if(r.extensionsUsed)for(let h=0;h<r.extensionsUsed.length;++h){let d=r.extensionsUsed[h],u=r.extensionsRequired||[];switch(d){case Ut.KHR_MATERIALS_UNLIT:a[d]=new vu;break;case Ut.KHR_DRACO_MESH_COMPRESSION:a[d]=new Uu(r,this.dracoLoader);break;case Ut.KHR_TEXTURE_TRANSFORM:a[d]=new Ou;break;case Ut.KHR_MESH_QUANTIZATION:a[d]=new zu;break;default:u.indexOf(d)>=0&&o[d]===void 0&&console.warn('THREE.GLTFLoader: Unknown extension "'+d+'".')}}c.setExtensions(a),c.setPlugins(o),c.parse(n,i)}parseAsync(e,t){let n=this;return new Promise(function(i,r){n.parse(e,t,i,r)})}};function J_(){let s={};return{get:function(e){return s[e]},add:function(e,t){s[e]=t},remove:function(e){delete s[e]},removeAll:function(){s={}}}}function dn(s,e,t){let n=s.json.materials[e];return n.extensions&&n.extensions[t]?n.extensions[t]:null}var Ut={KHR_BINARY_GLTF:"KHR_binary_glTF",KHR_DRACO_MESH_COMPRESSION:"KHR_draco_mesh_compression",KHR_LIGHTS_PUNCTUAL:"KHR_lights_punctual",KHR_MATERIALS_CLEARCOAT:"KHR_materials_clearcoat",KHR_MATERIALS_DISPERSION:"KHR_materials_dispersion",KHR_MATERIALS_IOR:"KHR_materials_ior",KHR_MATERIALS_SHEEN:"KHR_materials_sheen",KHR_MATERIALS_SPECULAR:"KHR_materials_specular",KHR_MATERIALS_TRANSMISSION:"KHR_materials_transmission",KHR_MATERIALS_IRIDESCENCE:"KHR_materials_iridescence",KHR_MATERIALS_ANISOTROPY:"KHR_materials_anisotropy",KHR_MATERIALS_UNLIT:"KHR_materials_unlit",KHR_MATERIALS_VOLUME:"KHR_materials_volume",KHR_TEXTURE_BASISU:"KHR_texture_basisu",KHR_TEXTURE_TRANSFORM:"KHR_texture_transform",KHR_MESH_QUANTIZATION:"KHR_mesh_quantization",KHR_MATERIALS_EMISSIVE_STRENGTH:"KHR_materials_emissive_strength",EXT_MATERIALS_BUMP:"EXT_materials_bump",EXT_TEXTURE_WEBP:"EXT_texture_webp",EXT_TEXTURE_AVIF:"EXT_texture_avif",EXT_MESHOPT_COMPRESSION:"EXT_meshopt_compression",KHR_MESHOPT_COMPRESSION:"KHR_meshopt_compression",EXT_MESH_GPU_INSTANCING:"EXT_mesh_gpu_instancing"},bu=class{constructor(e){this.parser=e,this.name=Ut.KHR_LIGHTS_PUNCTUAL,this.cache={refs:{},uses:{}}}_markDefs(){let e=this.parser,t=this.parser.json.nodes||[];for(let n=0,i=t.length;n<i;n++){let r=t[n];r.extensions&&r.extensions[this.name]&&r.extensions[this.name].light!==void 0&&e._addNodeRef(this.cache,r.extensions[this.name].light)}}_loadLight(e){let t=this.parser,n="light:"+e,i=t.cache.get(n);if(i)return i;let r=t.json,l=((r.extensions&&r.extensions[this.name]||{}).lights||[])[e],c,h=new Ce(16777215);l.color!==void 0&&h.setRGB(l.color[0],l.color[1],l.color[2],On);let d=l.range!==void 0?l.range:0;switch(l.type){case"directional":c=new $i(h),c.target.position.set(0,0,-1),c.add(c.target);break;case"point":c=new bi(h),c.distance=d;break;case"spot":c=new Ba(h),c.distance=d,l.spot=l.spot||{},l.spot.innerConeAngle=l.spot.innerConeAngle!==void 0?l.spot.innerConeAngle:0,l.spot.outerConeAngle=l.spot.outerConeAngle!==void 0?l.spot.outerConeAngle:Math.PI/4,c.angle=l.spot.outerConeAngle,c.penumbra=1-l.spot.innerConeAngle/l.spot.outerConeAngle,c.target.position.set(0,0,-1),c.add(c.target);break;default:throw new Error("THREE.GLTFLoader: Unexpected light type: "+l.type)}return c.position.set(0,0,0),Oi(c,l),l.intensity!==void 0&&(c.intensity=l.intensity),c.name=t.createUniqueName(l.name||"light_"+e),i=Promise.resolve(c),t.cache.add(n,i),i}getDependency(e,t){if(e==="light")return this._loadLight(t)}createNodeAttachment(e){let t=this,n=this.parser,r=n.json.nodes[e],o=(r.extensions&&r.extensions[this.name]||{}).light;return o===void 0?null:this._loadLight(o).then(function(l){return n._getNodeRef(t.cache,o,l)})}},vu=class{constructor(){this.name=Ut.KHR_MATERIALS_UNLIT}getMaterialType(){return qt}extendParams(e,t,n){let i=[];e.color=new Ce(1,1,1),e.opacity=1;let r=t.pbrMetallicRoughness;if(r){if(Array.isArray(r.baseColorFactor)){let a=r.baseColorFactor;e.color.setRGB(a[0],a[1],a[2],On),e.opacity=a[3]}r.baseColorTexture!==void 0&&i.push(n.assignTexture(e,"map",r.baseColorTexture,sn))}return Promise.all(i)}},_u=class{constructor(e){this.parser=e,this.name=Ut.KHR_MATERIALS_EMISSIVE_STRENGTH}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);return n===null||n.emissiveStrength!==void 0&&(t.emissiveIntensity=n.emissiveStrength),Promise.resolve()}},yu=class{constructor(e){this.parser=e,this.name=Ut.KHR_MATERIALS_CLEARCOAT}getMaterialType(e){return dn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(n.clearcoatFactor!==void 0&&(t.clearcoat=n.clearcoatFactor),n.clearcoatTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatMap",n.clearcoatTexture)),n.clearcoatRoughnessFactor!==void 0&&(t.clearcoatRoughness=n.clearcoatRoughnessFactor),n.clearcoatRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatRoughnessMap",n.clearcoatRoughnessTexture)),n.clearcoatNormalTexture!==void 0&&(i.push(this.parser.assignTexture(t,"clearcoatNormalMap",n.clearcoatNormalTexture)),n.clearcoatNormalTexture.scale!==void 0)){let r=n.clearcoatNormalTexture.scale;t.clearcoatNormalScale=new ze(r,r)}return Promise.all(i)}},Mu=class{constructor(e){this.parser=e,this.name=Ut.KHR_MATERIALS_DISPERSION}getMaterialType(e){return dn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);return n===null||(t.dispersion=n.dispersion!==void 0?n.dispersion:0),Promise.resolve()}},wu=class{constructor(e){this.parser=e,this.name=Ut.KHR_MATERIALS_IRIDESCENCE}getMaterialType(e){return dn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.iridescenceFactor!==void 0&&(t.iridescence=n.iridescenceFactor),n.iridescenceTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceMap",n.iridescenceTexture)),n.iridescenceIor!==void 0&&(t.iridescenceIOR=n.iridescenceIor),t.iridescenceThicknessRange===void 0&&(t.iridescenceThicknessRange=[100,400]),n.iridescenceThicknessMinimum!==void 0&&(t.iridescenceThicknessRange[0]=n.iridescenceThicknessMinimum),n.iridescenceThicknessMaximum!==void 0&&(t.iridescenceThicknessRange[1]=n.iridescenceThicknessMaximum),n.iridescenceThicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceThicknessMap",n.iridescenceThicknessTexture)),Promise.all(i)}},Su=class{constructor(e){this.parser=e,this.name=Ut.KHR_MATERIALS_SHEEN}getMaterialType(e){return dn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(t.sheenColor=new Ce(0,0,0),t.sheenRoughness=0,t.sheen=1,n.sheenColorFactor!==void 0){let r=n.sheenColorFactor;t.sheenColor.setRGB(r[0],r[1],r[2],On)}return n.sheenRoughnessFactor!==void 0&&(t.sheenRoughness=n.sheenRoughnessFactor),n.sheenColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenColorMap",n.sheenColorTexture,sn)),n.sheenRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenRoughnessMap",n.sheenRoughnessTexture)),Promise.all(i)}},Eu=class{constructor(e){this.parser=e,this.name=Ut.KHR_MATERIALS_TRANSMISSION}getMaterialType(e){return dn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.transmissionFactor!==void 0&&(t.transmission=n.transmissionFactor),n.transmissionTexture!==void 0&&i.push(this.parser.assignTexture(t,"transmissionMap",n.transmissionTexture)),Promise.all(i)}},Tu=class{constructor(e){this.parser=e,this.name=Ut.KHR_MATERIALS_VOLUME}getMaterialType(e){return dn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.thickness=n.thicknessFactor!==void 0?n.thicknessFactor:0,n.thicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"thicknessMap",n.thicknessTexture)),t.attenuationDistance=n.attenuationDistance||1/0;let r=n.attenuationColor||[1,1,1];return t.attenuationColor=new Ce().setRGB(r[0],r[1],r[2],On),Promise.all(i)}},Au=class{constructor(e){this.parser=e,this.name=Ut.KHR_MATERIALS_IOR}getMaterialType(e){return dn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);return n===null||(t.ior=n.ior!==void 0?n.ior:1.5,t.ior===0&&(t.ior=1e3)),Promise.resolve()}},Ru=class{constructor(e){this.parser=e,this.name=Ut.KHR_MATERIALS_SPECULAR}getMaterialType(e){return dn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.specularIntensity=n.specularFactor!==void 0?n.specularFactor:1,n.specularTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularIntensityMap",n.specularTexture));let r=n.specularColorFactor||[1,1,1];return t.specularColor=new Ce().setRGB(r[0],r[1],r[2],On),n.specularColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularColorMap",n.specularColorTexture,sn)),Promise.all(i)}},Cu=class{constructor(e){this.parser=e,this.name=Ut.EXT_MATERIALS_BUMP}getMaterialType(e){return dn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return t.bumpScale=n.bumpFactor!==void 0?n.bumpFactor:1,n.bumpTexture!==void 0&&i.push(this.parser.assignTexture(t,"bumpMap",n.bumpTexture)),Promise.all(i)}},Pu=class{constructor(e){this.parser=e,this.name=Ut.KHR_MATERIALS_ANISOTROPY}getMaterialType(e){return dn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=dn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.anisotropyStrength!==void 0&&(t.anisotropy=n.anisotropyStrength),n.anisotropyRotation!==void 0&&(t.anisotropyRotation=n.anisotropyRotation),n.anisotropyTexture!==void 0&&i.push(this.parser.assignTexture(t,"anisotropyMap",n.anisotropyTexture)),Promise.all(i)}},Iu=class{constructor(e){this.parser=e,this.name=Ut.KHR_TEXTURE_BASISU}loadTexture(e){let t=this.parser,n=t.json,i=n.textures[e];if(!i.extensions||!i.extensions[this.name])return null;let r=i.extensions[this.name],a=t.options.ktx2Loader;if(!a){if(n.extensionsRequired&&n.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setKTX2Loader must be called before loading KTX2 textures");return null}return t.loadTextureImage(e,r.source,a)}},Du=class{constructor(e){this.parser=e,this.name=Ut.EXT_TEXTURE_WEBP}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let a=r.extensions[t],o=i.images[a.source],l=n.textureLoader;if(o.uri){let c=n.options.manager.getHandler(o.uri);c!==null&&(l=c)}return n.loadTextureImage(e,a.source,l)}},Lu=class{constructor(e){this.parser=e,this.name=Ut.EXT_TEXTURE_AVIF}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let a=r.extensions[t],o=i.images[a.source],l=n.textureLoader;if(o.uri){let c=n.options.manager.getHandler(o.uri);c!==null&&(l=c)}return n.loadTextureImage(e,a.source,l)}},Tc=class{constructor(e,t){this.name=t,this.parser=e}loadBufferView(e){let t=this.parser.json,n=t.bufferViews[e];if(n.extensions&&n.extensions[this.name]){let i=n.extensions[this.name],r=this.parser.getDependency("buffer",i.buffer),a=this.parser.options.meshoptDecoder;if(!a||!a.supported){if(t.extensionsRequired&&t.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setMeshoptDecoder must be called before loading compressed files");return null}return r.then(function(o){let l=i.byteOffset||0,c=i.byteLength||0,h=i.count,d=i.byteStride,u=new Uint8Array(o,l,c);return a.decodeGltfBufferAsync?a.decodeGltfBufferAsync(h,d,u,i.mode,i.filter).then(function(p){return p.buffer}):a.ready.then(function(){let p=new ArrayBuffer(h*d);return a.decodeGltfBuffer(new Uint8Array(p),h,d,u,i.mode,i.filter),p})})}else return null}},Fu=class{constructor(e){this.name=Ut.EXT_MESH_GPU_INSTANCING,this.parser=e}createNodeMesh(e){let t=this.parser.json,n=t.nodes[e];if(!n.extensions||!n.extensions[this.name]||n.mesh===void 0)return null;let i=t.meshes[n.mesh];for(let c of i.primitives)if(c.mode!==ai.TRIANGLES&&c.mode!==ai.TRIANGLE_STRIP&&c.mode!==ai.TRIANGLE_FAN&&c.mode!==void 0)return null;let a=n.extensions[this.name].attributes,o=[],l={};for(let c in a)o.push(this.parser.getDependency("accessor",a[c]).then(h=>(l[c]=h,l[c])));return o.length<1?null:(o.push(this.parser.createNodeMesh(e)),Promise.all(o).then(c=>{let h=c.pop(),d=h.isGroup?h.children:[h],u=c[0].count,p=[];for(let x of d){let v=new mt,m=new k,f=new $t,y=new k(1,1,1),T=new En(x.geometry,x.material,u);for(let _=0;_<u;_++)l.TRANSLATION&&m.fromBufferAttribute(l.TRANSLATION,_),l.ROTATION&&f.fromBufferAttribute(l.ROTATION,_),l.SCALE&&y.fromBufferAttribute(l.SCALE,_),T.setMatrixAt(_,v.compose(m,f,y));let g=null;for(let _ in l)if(_==="_COLOR_0"){let S=l[_];T.instanceColor=new mi(S.array,S.itemSize,S.normalized)}else if(_!=="TRANSLATION"&&_!=="ROTATION"&&_!=="SCALE"){if(g===null){let A=T.geometry;g=new Mt,g.name=A.name;for(let b in A.attributes)g.setAttribute(b,A.attributes[b]);for(let b in A.morphAttributes)g.morphAttributes[b]=A.morphAttributes[b];A.index!==null&&g.setIndex(A.index),g.morphTargetsRelative=A.morphTargetsRelative;for(let b of A.groups)g.addGroup(b.start,b.count,b.materialIndex);A.boundingBox!==null&&(g.boundingBox=A.boundingBox.clone()),A.boundingSphere!==null&&(g.boundingSphere=A.boundingSphere.clone()),g.drawRange.start=A.drawRange.start,g.drawRange.count=A.drawRange.count,g.userData=Object.assign({},A.userData),T.geometry=g}let S=l[_];g.setAttribute(_,new mi(S.array,S.itemSize,S.normalized))}Xt.prototype.copy.call(T,x),this.parser.assignFinalMaterial(T),p.push(T)}return h.isGroup?(h.clear(),h.add(...p),h):p[0]}))}},Rp="glTF",ho=12,Sp={JSON:1313821514,BIN:5130562},Nu=class{constructor(e){this.name=Ut.KHR_BINARY_GLTF,this.content=null,this.body=null;let t=new DataView(e,0,ho),n=new TextDecoder;if(this.header={magic:n.decode(new Uint8Array(e.slice(0,4))),version:t.getUint32(4,!0),length:t.getUint32(8,!0)},this.header.magic!==Rp)throw new Error("THREE.GLTFLoader: Unsupported glTF-Binary header.");if(this.header.version<2)throw new Error("THREE.GLTFLoader: Legacy binary file detected.");let i=this.header.length-ho,r=new DataView(e,ho),a=0;for(;a<i;){let o=r.getUint32(a,!0);a+=4;let l=r.getUint32(a,!0);if(a+=4,l===Sp.JSON){let c=new Uint8Array(e,ho+a,o);this.content=n.decode(c)}else if(l===Sp.BIN){let c=ho+a;this.body=e.slice(c,c+o)}a+=o}if(this.content===null)throw new Error("THREE.GLTFLoader: JSON content not found.")}},Uu=class{constructor(e,t){if(!t)throw new Error("THREE.GLTFLoader: No DRACOLoader instance provided.");this.name=Ut.KHR_DRACO_MESH_COMPRESSION,this.json=e,this.dracoLoader=t,this.dracoLoader.preload()}decodePrimitive(e,t){let n=this.json,i=this.dracoLoader,r=e.extensions[this.name].bufferView,a=e.extensions[this.name].attributes,o={},l={},c={};for(let h in a){let d=Bu[h]||h.toLowerCase();o[d]=a[h]}for(let h in e.attributes){let d=Bu[h]||h.toLowerCase();if(a[h]!==void 0){let u=n.accessors[e.attributes[h]],p=Jr[u.componentType];c[d]=p.name,l[d]=u.normalized===!0}}return t.getDependency("bufferView",r).then(function(h){return new Promise(function(d,u){i.decodeDracoFile(h,function(p){for(let x in p.attributes){let v=p.attributes[x],m=l[x];m!==void 0&&(v.normalized=m)}d(p)},o,c,On,u)})})}},Ou=class{constructor(){this.name=Ut.KHR_TEXTURE_TRANSFORM}extendTexture(e,t){if((t.texCoord===void 0||t.texCoord===e.channel)&&t.offset===void 0&&t.rotation===void 0&&t.scale===void 0)return e;if(e=e.clone(),t.texCoord!==void 0&&(e.channel=t.texCoord),t.offset!==void 0&&e.offset.fromArray(t.offset),t.rotation!==void 0&&(e.rotation=t.rotation),t.scale!==void 0&&e.repeat.fromArray(t.scale),t.rotation!==void 0){let n=Math.cos(e.rotation),i=Math.sin(e.rotation);e.matrix.set(e.repeat.x*n,e.repeat.y*i,e.offset.x,-e.repeat.x*i,e.repeat.y*n,e.offset.y,0,0,1),e.matrixAutoUpdate=!1}return e.needsUpdate=!0,e}},zu=class{constructor(){this.name=Ut.KHR_MESH_QUANTIZATION}},Ac=class extends Di{constructor(e,t,n,i){super(e,t,n,i)}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i*3+i;for(let a=0;a!==i;a++)t[a]=n[r+a];return t}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=o*2,c=o*3,h=i-t,d=(n-t)/h,u=d*d,p=u*d,x=e*c,v=x-c,m=-2*p+3*u,f=p-u,y=1-m,T=f-u+d;for(let g=0;g!==o;g++){let _=a[v+g+o],S=a[v+g+l]*h,A=a[x+g+o],b=a[x+g]*h;r[g]=y*_+T*S+m*A+f*b}return r}},Z_=new $t,ku=class extends Ac{interpolate_(e,t,n,i){let r=super.interpolate_(e,t,n,i);return Z_.fromArray(r).normalize().toArray(r),r}},ai={FLOAT:5126,FLOAT_MAT3:35675,FLOAT_MAT4:35676,FLOAT_VEC2:35664,FLOAT_VEC3:35665,FLOAT_VEC4:35666,LINEAR:9729,REPEAT:10497,SAMPLER_2D:35678,POINTS:0,LINES:1,LINE_LOOP:2,LINE_STRIP:3,TRIANGLES:4,TRIANGLE_STRIP:5,TRIANGLE_FAN:6,UNSIGNED_BYTE:5121,UNSIGNED_SHORT:5123},Jr={5120:Int8Array,5121:Uint8Array,5122:Int16Array,5123:Uint16Array,5125:Uint32Array,5126:Float32Array},Ep={9728:ln,9729:rn,9984:Dl,9985:Hr,9986:Ys,9987:_i},Tp={33071:ti,33648:vr,10497:Ai},mu={SCALAR:1,VEC2:2,VEC3:3,VEC4:4,MAT2:4,MAT3:9,MAT4:16},Bu={POSITION:"position",NORMAL:"normal",TANGENT:"tangent",TEXCOORD_0:"uv",TEXCOORD_1:"uv1",TEXCOORD_2:"uv2",TEXCOORD_3:"uv3",COLOR_0:"color",WEIGHTS_0:"skinWeight",JOINTS_0:"skinIndex"},Ss={scale:"scale",translation:"position",rotation:"quaternion",weights:"morphTargetInfluences"},Q_={CUBICSPLINE:void 0,LINEAR:Os,STEP:Us},gu={OPAQUE:"OPAQUE",MASK:"MASK",BLEND:"BLEND"};function $_(s){return s.DefaultMaterial===void 0&&(s.DefaultMaterial=new An({color:16777215,emissive:0,metalness:1,roughness:1,transparent:!1,depthTest:!0,side:ii})),s.DefaultMaterial}function Qs(s,e,t){for(let n in t.extensions)s[n]===void 0&&(e.userData.gltfExtensions=e.userData.gltfExtensions||{},e.userData.gltfExtensions[n]=t.extensions[n])}function Oi(s,e){e.extras!==void 0&&(typeof e.extras=="object"?Object.assign(s.userData,e.extras):console.warn("THREE.GLTFLoader: Ignoring primitive type .extras, "+e.extras))}function ey(s,e,t){let n=!1,i=!1,r=!1;for(let c=0,h=e.length;c<h;c++){let d=e[c];if(d.POSITION!==void 0&&(n=!0),d.NORMAL!==void 0&&(i=!0),d.COLOR_0!==void 0&&(r=!0),n&&i&&r)break}if(!n&&!i&&!r)return Promise.resolve(s);let a=[],o=[],l=[];for(let c=0,h=e.length;c<h;c++){let d=e[c];if(n){let u=d.POSITION!==void 0?t.getDependency("accessor",d.POSITION):s.attributes.position;a.push(u)}if(i){let u=d.NORMAL!==void 0?t.getDependency("accessor",d.NORMAL):s.attributes.normal;o.push(u)}if(r){let u=d.COLOR_0!==void 0?t.getDependency("accessor",d.COLOR_0):s.attributes.color;l.push(u)}}return Promise.all([Promise.all(a),Promise.all(o),Promise.all(l)]).then(function(c){let h=c[0],d=c[1],u=c[2];return n&&(s.morphAttributes.position=h),i&&(s.morphAttributes.normal=d),r&&(s.morphAttributes.color=u),s.morphTargetsRelative=!0,s})}function ty(s,e){if(s.updateMorphTargets(),e.weights!==void 0)for(let t=0,n=e.weights.length;t<n;t++)s.morphTargetInfluences[t]=e.weights[t];if(e.extras&&Array.isArray(e.extras.targetNames)){let t=e.extras.targetNames;if(s.morphTargetInfluences.length===t.length){s.morphTargetDictionary={};for(let n=0,i=t.length;n<i;n++)s.morphTargetDictionary[t[n]]=n}else console.warn("THREE.GLTFLoader: Invalid extras.targetNames length. Ignoring names.")}}function ny(s){let e,t=s.extensions&&s.extensions[Ut.KHR_DRACO_MESH_COMPRESSION];if(t?e="draco:"+t.bufferView+":"+t.indices+":"+xu(t.attributes):e=s.indices+":"+xu(s.attributes)+":"+s.mode,s.targets!==void 0)for(let n=0,i=s.targets.length;n<i;n++)e+=":"+xu(s.targets[n]);return e}function xu(s){let e="",t=Object.keys(s).sort();for(let n=0,i=t.length;n<i;n++)e+=t[n]+":"+s[t[n]]+";";return e}function Hu(s){switch(s){case Int8Array:return 1/127;case Uint8Array:return 1/255;case Int16Array:return 1/32767;case Uint16Array:return 1/65535;default:throw new Error("THREE.GLTFLoader: Unsupported normalized accessor component type.")}}function iy(s){return s.search(/\.jpe?g($|\?)/i)>0||s.search(/^data\:image\/jpeg/)===0?"image/jpeg":s.search(/\.webp($|\?)/i)>0||s.search(/^data\:image\/webp/)===0?"image/webp":s.search(/\.ktx2($|\?)/i)>0||s.search(/^data\:image\/ktx2/)===0?"image/ktx2":"image/png"}var sy=new mt,Gu=class{constructor(e={},t={}){this.json=e,this.extensions={},this.plugins={},this.options=t,this.cache=new J_,this.associations=new Map,this.primitiveCache={},this.nodeCache={},this.meshCache={refs:{},uses:{}},this.cameraCache={refs:{},uses:{}},this.lightCache={refs:{},uses:{}},this.sourceCache={},this.textureCache={},this.nodeNamesUsed={};let n=!1,i=-1,r=!1,a=-1;if(typeof navigator<"u"&&typeof navigator.userAgent<"u"){let o=navigator.userAgent;n=/^((?!chrome|android).)*safari/i.test(o)===!0;let l=o.match(/Version\/(\d+)/);i=n&&l?parseInt(l[1],10):-1,r=o.indexOf("Firefox")>-1,a=r?o.match(/Firefox\/([0-9]+)\./)[1]:-1}typeof createImageBitmap>"u"||n&&i<17||r&&a<98?this.textureLoader=new Oa(this.options.manager):this.textureLoader=new Ha(this.options.manager),this.textureLoader.setCrossOrigin(this.options.crossOrigin),this.textureLoader.setRequestHeader(this.options.requestHeader),this.fileLoader=new Ur(this.options.manager),this.fileLoader.setResponseType("arraybuffer"),this.options.crossOrigin==="use-credentials"&&this.fileLoader.setWithCredentials(!0)}setExtensions(e){this.extensions=e}setPlugins(e){this.plugins=e}parse(e,t){let n=this,i=this.json,r=this.extensions;this.cache.removeAll(),this.nodeCache={},this._invokeAll(function(a){return a._markDefs&&a._markDefs()}),Promise.all(this._invokeAll(function(a){return a.beforeRoot&&a.beforeRoot()})).then(function(){return Promise.all([n.getDependencies("scene"),n.getDependencies("animation"),n.getDependencies("camera")])}).then(function(a){let o={scene:a[0][i.scene||0],scenes:a[0],animations:a[1],cameras:a[2],asset:i.asset,parser:n,userData:{}};return Qs(r,o,i),Oi(o,i),Promise.all(n._invokeAll(function(l){return l.afterRoot&&l.afterRoot(o)})).then(function(){for(let l of o.scenes)l.updateMatrixWorld();e(o)})}).catch(t)}_markDefs(){let e=this.json.nodes||[],t=this.json.skins||[],n=this.json.meshes||[];for(let i=0,r=t.length;i<r;i++){let a=t[i].joints;for(let o=0,l=a.length;o<l;o++)e[a[o]].isBone=!0}for(let i=0,r=e.length;i<r;i++){let a=e[i];a.mesh!==void 0&&(this._addNodeRef(this.meshCache,a.mesh),a.skin!==void 0&&(n[a.mesh].isSkinnedMesh=!0)),a.camera!==void 0&&this._addNodeRef(this.cameraCache,a.camera)}}_addNodeRef(e,t){t!==void 0&&(e.refs[t]===void 0&&(e.refs[t]=e.uses[t]=0),e.refs[t]++)}_getNodeRef(e,t,n){if(e.refs[t]<=1)return n;let i=n.clone(),r=(a,o)=>{let l=this.associations.get(a);l!=null&&this.associations.set(o,l);for(let[c,h]of a.children.entries())r(h,o.children[c])};return r(n,i),i.name+="_instance_"+e.uses[t]++,i}_invokeOne(e){let t=Object.values(this.plugins);t.push(this);for(let n=0;n<t.length;n++){let i=e(t[n]);if(i)return i}return null}_invokeAll(e){let t=Object.values(this.plugins);t.unshift(this);let n=[];for(let i=0;i<t.length;i++){let r=e(t[i]);r&&n.push(r)}return n}getDependency(e,t){let n=e+":"+t,i=this.cache.get(n);if(!i){switch(e){case"scene":i=this.loadScene(t);break;case"node":i=this._invokeOne(function(r){return r.loadNode&&r.loadNode(t)});break;case"mesh":i=this._invokeOne(function(r){return r.loadMesh&&r.loadMesh(t)});break;case"accessor":i=this.loadAccessor(t);break;case"bufferView":i=this._invokeOne(function(r){return r.loadBufferView&&r.loadBufferView(t)});break;case"buffer":i=this.loadBuffer(t);break;case"material":i=this._invokeOne(function(r){return r.loadMaterial&&r.loadMaterial(t)});break;case"texture":i=this._invokeOne(function(r){return r.loadTexture&&r.loadTexture(t)});break;case"skin":i=this.loadSkin(t);break;case"animation":i=this._invokeOne(function(r){return r.loadAnimation&&r.loadAnimation(t)});break;case"camera":i=this.loadCamera(t);break;default:if(i=this._invokeOne(function(r){return r!=this&&r.getDependency&&r.getDependency(e,t)}),!i)throw new Error("Unknown type: "+e);break}this.cache.add(n,i)}return i}getDependencies(e){let t=this.cache.get(e);if(!t){let n=this,i=this.json[e+(e==="mesh"?"es":"s")]||[];t=Promise.all(i.map(function(r,a){return n.getDependency(e,a)})),this.cache.add(e,t)}return t}loadBuffer(e){let t=this.json.buffers[e],n=this.fileLoader;if(t.type&&t.type!=="arraybuffer")throw new Error("THREE.GLTFLoader: "+t.type+" buffer type is not supported.");if(t.uri===void 0&&e===0)return Promise.resolve(this.extensions[Ut.KHR_BINARY_GLTF].body);let i=this.options;return new Promise(function(r,a){n.load(es.resolveURL(t.uri,i.path),r,void 0,function(){a(new Error('THREE.GLTFLoader: Failed to load buffer "'+t.uri+'".'))})})}loadBufferView(e){let t=this.json.bufferViews[e];return this.getDependency("buffer",t.buffer).then(function(n){let i=t.byteLength||0,r=t.byteOffset||0;return n.slice(r,r+i)})}loadAccessor(e){let t=this,n=this.json,i=this.json.accessors[e];if(i.bufferView===void 0&&i.sparse===void 0){let a=mu[i.type],o=Jr[i.componentType],l=i.normalized===!0,c=new o(i.count*a);return Promise.resolve(new Tt(c,a,l))}let r=[];return i.bufferView!==void 0?r.push(this.getDependency("bufferView",i.bufferView)):r.push(null),i.sparse!==void 0&&(r.push(this.getDependency("bufferView",i.sparse.indices.bufferView)),r.push(this.getDependency("bufferView",i.sparse.values.bufferView))),Promise.all(r).then(function(a){let o=a[0],l=mu[i.type],c=Jr[i.componentType],h=c.BYTES_PER_ELEMENT,d=h*l,u=i.byteOffset||0,p=i.bufferView!==void 0?n.bufferViews[i.bufferView].byteStride:void 0,x=i.normalized===!0,v,m;if(p&&p!==d){let f=Math.floor(u/p),y="InterleavedBuffer:"+i.bufferView+":"+i.componentType+":"+f+":"+i.count,T=t.cache.get(y);T||(v=new c(o,f*p,i.count*p/h),T=new Tr(v,p/h),t.cache.add(y,T)),m=new Ar(T,l,u%p/h,x)}else o===null?v=new c(i.count*l):v=new c(o,u,i.count*l),m=new Tt(v,l,x);if(i.sparse!==void 0){let f=mu.SCALAR,y=Jr[i.sparse.indices.componentType],T=i.sparse.indices.byteOffset||0,g=i.sparse.values.byteOffset||0,_=new y(a[1],T,i.sparse.count*f),S=new c(a[2],g,i.sparse.count*l);o!==null&&(m=new Tt(m.array.slice(),m.itemSize,m.normalized)),m.normalized=!1;for(let A=0,b=_.length;A<b;A++){let C=_[A];if(m.setX(C,S[A*l]),l>=2&&m.setY(C,S[A*l+1]),l>=3&&m.setZ(C,S[A*l+2]),l>=4&&m.setW(C,S[A*l+3]),l>=5)throw new Error("THREE.GLTFLoader: Unsupported itemSize in sparse BufferAttribute.")}m.normalized=x}return m})}loadTexture(e){let t=this.json,n=this.options,r=t.textures[e].source,a=t.images[r],o=this.textureLoader;if(a.uri){let l=n.manager.getHandler(a.uri);l!==null&&(o=l)}return this.loadTextureImage(e,r,o)}loadTextureImage(e,t,n){let i=this,r=this.json,a=r.textures[e],o=r.images[t],l=(o.uri||o.bufferView)+":"+a.sampler;if(this.textureCache[l])return this.textureCache[l];let c=this.loadImageSource(t,n).then(function(h){h.flipY=!1,h.name=a.name||o.name||"",h.name===""&&typeof o.uri=="string"&&o.uri.startsWith("data:image/")===!1&&(h.name=o.uri);let u=(r.samplers||{})[a.sampler]||{};return h.magFilter=Ep[u.magFilter]||rn,h.minFilter=Ep[u.minFilter]||_i,h.wrapS=Tp[u.wrapS]||Ai,h.wrapT=Tp[u.wrapT]||Ai,h.generateMipmaps=!h.isCompressedTexture&&h.minFilter!==ln&&h.minFilter!==rn,i.associations.set(h,{textures:e}),h}).catch(function(){return null});return this.textureCache[l]=c,c}loadImageSource(e,t){let n=this,i=this.json,r=this.options;if(this.sourceCache[e]!==void 0)return this.sourceCache[e].then(d=>d.clone());let a=i.images[e],o=self.URL||self.webkitURL,l=a.uri||"",c=!1;if(a.bufferView!==void 0)l=n.getDependency("bufferView",a.bufferView).then(function(d){c=!0;let u=new Blob([d],{type:a.mimeType});return l=o.createObjectURL(u),l});else if(a.uri===void 0)throw new Error("THREE.GLTFLoader: Image "+e+" is missing URI and bufferView");let h=Promise.resolve(l).then(function(d){return new Promise(function(u,p){let x=u;t.isImageBitmapLoader===!0&&(x=function(v){let m=new cn(v);m.needsUpdate=!0,u(m)}),t.load(es.resolveURL(d,r.path),x,void 0,p)})}).then(function(d){return c===!0&&o.revokeObjectURL(l),Oi(d,a),d.userData.mimeType=a.mimeType||iy(a.uri),d}).catch(function(d){throw console.error("THREE.GLTFLoader: Couldn't load texture",l),d});return this.sourceCache[e]=h,h}assignTexture(e,t,n,i){let r=this;return this.getDependency("texture",n.index).then(function(a){if(!a)return null;if(n.texCoord!==void 0&&n.texCoord>0&&(a=a.clone(),a.channel=n.texCoord),r.extensions[Ut.KHR_TEXTURE_TRANSFORM]){let o=n.extensions!==void 0?n.extensions[Ut.KHR_TEXTURE_TRANSFORM]:void 0;if(o){let l=r.associations.get(a);a=r.extensions[Ut.KHR_TEXTURE_TRANSFORM].extendTexture(a,o),r.associations.set(a,l)}}return i!==void 0&&(a.colorSpace=i),e[t]=a,a})}assignFinalMaterial(e){let t=e.geometry,n=e.material,i=t.attributes.tangent===void 0,r=t.attributes.color!==void 0,a=t.attributes.normal===void 0;if(e.isPoints){let o="PointsMaterial:"+n.uuid,l=this.cache.get(o);l||(l=new Pr,zn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,l.sizeAttenuation=!1,this.cache.add(o,l)),n=l}else if(e.isLine){let o="LineBasicMaterial:"+n.uuid,l=this.cache.get(o);l||(l=new Pi,zn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,this.cache.add(o,l)),n=l}if(i||r||a){let o="ClonedMaterial:"+n.uuid+":";i&&(o+="derivative-tangents:"),r&&(o+="vertex-colors:"),a&&(o+="flat-shading:");let l=this.cache.get(o);l||(l=n.clone(),r&&(l.vertexColors=!0),a&&(l.flatShading=!0),i&&(l.normalScale&&(l.normalScale.y*=-1),l.clearcoatNormalScale&&(l.clearcoatNormalScale.y*=-1)),this.cache.add(o,l),this.associations.set(l,this.associations.get(n))),n=l}e.material=n}getMaterialType(){return An}loadMaterial(e){let t=this,n=this.json,i=this.extensions,r=n.materials[e],a,o={},l=r.extensions||{},c=[];if(l[Ut.KHR_MATERIALS_UNLIT]){let d=i[Ut.KHR_MATERIALS_UNLIT];a=d.getMaterialType(),c.push(d.extendParams(o,r,t))}else{let d=r.pbrMetallicRoughness||{};if(o.color=new Ce(1,1,1),o.opacity=1,Array.isArray(d.baseColorFactor)){let u=d.baseColorFactor;o.color.setRGB(u[0],u[1],u[2],On),o.opacity=u[3]}d.baseColorTexture!==void 0&&c.push(t.assignTexture(o,"map",d.baseColorTexture,sn)),o.metalness=d.metallicFactor!==void 0?d.metallicFactor:1,o.roughness=d.roughnessFactor!==void 0?d.roughnessFactor:1,d.metallicRoughnessTexture!==void 0&&(c.push(t.assignTexture(o,"metalnessMap",d.metallicRoughnessTexture)),c.push(t.assignTexture(o,"roughnessMap",d.metallicRoughnessTexture))),a=this._invokeOne(function(u){return u.getMaterialType&&u.getMaterialType(e)}),c.push(Promise.all(this._invokeAll(function(u){return u.extendMaterialParams&&u.extendMaterialParams(e,o)})))}r.doubleSided===!0&&(o.side=Yt);let h=r.alphaMode||gu.OPAQUE;if(h===gu.BLEND?(o.transparent=!0,o.depthWrite=!1):(o.transparent=!1,h===gu.MASK&&(o.alphaTest=r.alphaCutoff!==void 0?r.alphaCutoff:.5)),r.normalTexture!==void 0&&a!==qt&&(c.push(t.assignTexture(o,"normalMap",r.normalTexture)),o.normalScale=new ze(1,1),r.normalTexture.scale!==void 0)){let d=r.normalTexture.scale;o.normalScale.set(d,d)}if(r.occlusionTexture!==void 0&&a!==qt&&(c.push(t.assignTexture(o,"aoMap",r.occlusionTexture)),r.occlusionTexture.strength!==void 0&&(o.aoMapIntensity=r.occlusionTexture.strength)),r.emissiveFactor!==void 0&&a!==qt){let d=r.emissiveFactor;o.emissive=new Ce().setRGB(d[0],d[1],d[2],On)}return r.emissiveTexture!==void 0&&a!==qt&&c.push(t.assignTexture(o,"emissiveMap",r.emissiveTexture,sn)),Promise.all(c).then(function(){let d=new a(o);return r.name&&(d.name=r.name),Oi(d,r),t.associations.set(d,{materials:e}),r.extensions&&Qs(i,d,r),d})}createUniqueName(e){let t=Qt.sanitizeNodeName(e||"");return t in this.nodeNamesUsed?t+"_"+ ++this.nodeNamesUsed[t]:(this.nodeNamesUsed[t]=0,t)}loadGeometries(e){let t=this,n=this.extensions,i=this.primitiveCache;function r(o){return n[Ut.KHR_DRACO_MESH_COMPRESSION].decodePrimitive(o,t).then(function(l){return Ap(l,o,t)})}let a=[];for(let o=0,l=e.length;o<l;o++){let c=e[o],h=ny(c),d=i[h];if(d)a.push(d.promise);else{let u;c.extensions&&c.extensions[Ut.KHR_DRACO_MESH_COMPRESSION]?u=r(c):u=Ap(new Mt,c,t),c.mode===ai.TRIANGLE_STRIP?u=u.then(p=>pu(p,ro)):c.mode===ai.TRIANGLE_FAN&&(u=u.then(p=>pu(p,Wr))),i[h]={primitive:c,promise:u},a.push(u)}}return Promise.all(a)}loadMesh(e){let t=this,n=this.json,i=this.extensions,r=n.meshes[e],a=r.primitives,o=[];for(let l=0,c=a.length;l<c;l++){let h=a[l].material===void 0?$_(this.cache):this.getDependency("material",a[l].material);o.push(h)}return o.push(t.loadGeometries(a)),Promise.all(o).then(async function(l){let c=l.slice(0,l.length-1),h=l[l.length-1],d=[];for(let p=0,x=h.length;p<x;p++){let v=h[p],m=a[p],f,y=c[p];if(m.mode===ai.TRIANGLES||m.mode===ai.TRIANGLE_STRIP||m.mode===ai.TRIANGLE_FAN||m.mode===void 0){let T=r.isSkinnedMesh===!0,g=v.hasAttribute("skinIndex")&&v.hasAttribute("skinWeight");T&&g===!1&&console.warn("THREE.GLTFLoader: Missing skinIndex or skinWeight attributes. Skinning disabled."),f=T&&g?new Ea(v,y):new ot(v,y),f.isSkinnedMesh===!0&&f.normalizeSkinWeights()}else if(m.mode===ai.LINES)f=new Bs(v,y);else if(m.mode===ai.LINE_STRIP)f=new Ki(v,y);else if(m.mode===ai.LINE_LOOP)f=new Aa(v,y);else if(m.mode===ai.POINTS)f=new Tn(v,y);else throw new Error("THREE.GLTFLoader: Primitive mode unsupported: "+m.mode);Object.keys(f.geometry.morphAttributes).length>0&&ty(f,r),f.name=t.createUniqueName(r.name||"mesh_"+e),Oi(f,r),m.extensions&&Qs(i,f,m),t.assignFinalMaterial(f),d.push(f)}for(let p=0,x=d.length;p<x;p++)t.associations.set(d[p],{meshes:e,primitives:p});if(d.length===1)return r.extensions&&Qs(i,d[0],r),d[0];let u=new yt;r.extensions&&Qs(i,u,r),t.associations.set(u,{meshes:e});for(let p=0,x=d.length;p<x;p++)u.add(d[p]);return u})}loadCamera(e){let t,n=this.json.cameras[e],i=n[n.type];if(!i){console.warn("THREE.GLTFLoader: Missing camera parameters.");return}return n.type==="perspective"?t=new mn(It.radToDeg(i.yfov),i.aspectRatio||1,i.znear||1,i.zfar||2e6):n.type==="orthographic"&&(t=new Fi(-i.xmag,i.xmag,i.ymag,-i.ymag,i.znear,i.zfar)),n.name&&(t.name=this.createUniqueName(n.name)),Oi(t,n),Promise.resolve(t)}loadSkin(e){let t=this.json.skins[e],n=[];for(let i=0,r=t.joints.length;i<r;i++)n.push(this._loadNodeShallow(t.joints[i]));return t.inverseBindMatrices!==void 0?n.push(this.getDependency("accessor",t.inverseBindMatrices)):n.push(null),Promise.all(n).then(function(i){let r=i.pop(),a=i,o=[],l=[];for(let c=0,h=a.length;c<h;c++){let d=a[c];if(d){o.push(d);let u=new mt;r!==null&&u.fromArray(r.array,c*16),l.push(u)}else console.warn('THREE.GLTFLoader: Joint "%s" could not be found.',t.joints[c])}return new Ta(o,l)})}loadAnimation(e){let t=this.json,n=this,i=t.animations[e],r=i.name?i.name:"animation_"+e,a=[],o=[],l=[],c=[],h=[];for(let d=0,u=i.channels.length;d<u;d++){let p=i.channels[d],x=i.samplers[p.sampler],v=p.target,m=v.node,f=i.parameters!==void 0?i.parameters[x.input]:x.input,y=i.parameters!==void 0?i.parameters[x.output]:x.output;v.node!==void 0&&(a.push(this.getDependency("node",m)),o.push(this.getDependency("accessor",f)),l.push(this.getDependency("accessor",y)),c.push(x),h.push(v))}return Promise.all([Promise.all(a),Promise.all(o),Promise.all(l),Promise.all(c),Promise.all(h)]).then(function(d){let u=d[0],p=d[1],x=d[2],v=d[3],m=d[4],f=[];for(let T=0,g=u.length;T<g;T++){let _=u[T],S=p[T],A=x[T],b=v[T],C=m[T];if(_===void 0)continue;_.updateMatrix&&_.updateMatrix();let w=n._createAnimationTracks(_,S,A,b,C);if(w)for(let I=0;I<w.length;I++)f.push(w[I])}let y=new Vs(r,void 0,f);return Oi(y,i),y})}createNodeMesh(e){let t=this.json,n=this,i=t.nodes[e];return i.mesh===void 0?null:n.getDependency("mesh",i.mesh).then(function(r){let a=n._getNodeRef(n.meshCache,i.mesh,r);return i.weights!==void 0&&a.traverse(function(o){if(o.isMesh)for(let l=0,c=i.weights.length;l<c;l++)o.morphTargetInfluences[l]=i.weights[l]}),a})}loadNode(e){let t=this.json,n=this,i=t.nodes[e],r=n._loadNodeShallow(e),a=[],o=i.children||[];for(let c=0,h=o.length;c<h;c++)a.push(n.getDependency("node",o[c]));let l=i.skin===void 0?Promise.resolve(null):n.getDependency("skin",i.skin);return Promise.all([r,Promise.all(a),l]).then(function(c){let h=c[0],d=c[1],u=c[2];u!==null&&h.traverse(function(p){p.isSkinnedMesh&&p.bind(u,sy)});for(let p=0,x=d.length;p<x;p++)h.add(d[p]);if(h.userData.pivot!==void 0&&d.length>0){let p=h.userData.pivot,x=d[0];h.pivot=new k().fromArray(p),h.position.x-=p[0],h.position.y-=p[1],h.position.z-=p[2],x.position.set(0,0,0),delete h.userData.pivot}return h})}_loadNodeShallow(e){let t=this.json,n=this.extensions,i=this;if(this.nodeCache[e]!==void 0)return this.nodeCache[e];let r=t.nodes[e],a=r.name?i.createUniqueName(r.name):"",o=[],l=i._invokeOne(function(c){return c.createNodeMesh&&c.createNodeMesh(e)});return l&&o.push(l),r.camera!==void 0&&o.push(i.getDependency("camera",r.camera).then(function(c){return i._getNodeRef(i.cameraCache,r.camera,c)})),i._invokeAll(function(c){return c.createNodeAttachment&&c.createNodeAttachment(e)}).forEach(function(c){o.push(c)}),this.nodeCache[e]=Promise.all(o).then(function(c){let h;if(r.isBone===!0?h=new Rr:c.length>1?h=new yt:c.length===1?h=c[0]:h=new Xt,h!==c[0])for(let d=0,u=c.length;d<u;d++)h.add(c[d]);if(r.name&&(h.userData.name=r.name,h.name=a),Oi(h,r),r.extensions&&Qs(n,h,r),r.matrix!==void 0){let d=new mt;d.fromArray(r.matrix),h.applyMatrix4(d)}else r.translation!==void 0&&h.position.fromArray(r.translation),r.rotation!==void 0&&h.quaternion.fromArray(r.rotation),r.scale!==void 0&&h.scale.fromArray(r.scale);if(!i.associations.has(h))i.associations.set(h,{});else if(r.mesh!==void 0&&i.meshCache.refs[r.mesh]>1){let d=i.associations.get(h);i.associations.set(h,{...d})}return i.associations.get(h).nodes=e,h}),this.nodeCache[e]}loadScene(e){let t=this.extensions,n=this.json.scenes[e],i=this,r=new yt;n.name&&(r.name=i.createUniqueName(n.name)),Oi(r,n),n.extensions&&Qs(t,r,n);let a=n.nodes||[],o=[];for(let l=0,c=a.length;l<c;l++)o.push(i.getDependency("node",a[l]));return Promise.all(o).then(function(l){for(let h=0,d=l.length;h<d;h++){let u=l[h];u.parent!==null?r.add(Mp(u)):r.add(u)}let c=h=>{let d=new Map;for(let[u,p]of i.associations)(u instanceof zn||u instanceof cn)&&d.set(u,p);return h.traverse(u=>{let p=i.associations.get(u);p!=null&&d.set(u,p)}),d};return i.associations=c(r),r})}_createAnimationTracks(e,t,n,i,r){let a=[],o=e.name?e.name:e.uuid,l=[];function c(p){p.morphTargetInfluences&&l.push(p.name?p.name:p.uuid)}Ss[r.path]===Ss.weights?(c(e),e.isGroup&&e.children.forEach(c)):l.push(o);let h;switch(Ss[r.path]){case Ss.weights:h=Ji;break;case Ss.rotation:h=Zi;break;case Ss.translation:case Ss.scale:h=bs;break;default:n.itemSize===1?h=Ji:h=bs;break}let d=i.interpolation!==void 0?Q_[i.interpolation]:Os,u=this._getArrayFromAccessor(n);for(let p=0,x=l.length;p<x;p++){let v=new h(l[p]+"."+Ss[r.path],t.array,u,d);i.interpolation==="CUBICSPLINE"&&this._createCubicSplineTrackInterpolant(v),a.push(v)}return a}_getArrayFromAccessor(e){let t=e.array;if(e.normalized){let n=Hu(t.constructor),i=new Float32Array(t.length);for(let r=0,a=t.length;r<a;r++)i[r]=t[r]*n;t=i}return t}_createCubicSplineTrackInterpolant(e){e.createInterpolant=function(n){let i=this instanceof Zi?ku:Ac;return new i(this.times,this.values,this.getValueSize()/3,n)},e.createInterpolant.isInterpolantFactoryMethodGLTFCubicSpline=!0}};function ry(s,e,t){let n=e.attributes,i=new hn;if(n.POSITION!==void 0){let o=t.json.accessors[n.POSITION],l=o.min,c=o.max;if(l!==void 0&&c!==void 0){if(i.set(new k(l[0],l[1],l[2]),new k(c[0],c[1],c[2])),o.normalized){let h=Hu(Jr[o.componentType]);i.min.multiplyScalar(h),i.max.multiplyScalar(h)}}else{console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.");return}}else return;let r=e.targets;if(r!==void 0){let o=new k,l=new k;for(let c=0,h=r.length;c<h;c++){let d=r[c];if(d.POSITION!==void 0){let u=t.json.accessors[d.POSITION],p=u.min,x=u.max;if(p!==void 0&&x!==void 0){if(l.setX(Math.max(Math.abs(p[0]),Math.abs(x[0]))),l.setY(Math.max(Math.abs(p[1]),Math.abs(x[1]))),l.setZ(Math.max(Math.abs(p[2]),Math.abs(x[2]))),u.normalized){let v=Hu(Jr[u.componentType]);l.multiplyScalar(v)}o.max(l)}else console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.")}}i.expandByVector(o)}s.boundingBox=i;let a=new In;i.getCenter(a.center),a.radius=i.min.distanceTo(i.max)/2,s.boundingSphere=a}function Ap(s,e,t){let n=e.attributes,i=[];function r(a,o){return t.getDependency("accessor",a).then(function(l){s.setAttribute(o,l)})}for(let a in n){let o=Bu[a]||a.toLowerCase();o in s.attributes||i.push(r(n[a],o))}if(e.indices!==void 0&&!s.index){let a=t.getDependency("accessor",e.indices).then(function(o){s.setIndex(o)});i.push(a)}return Rt.workingColorSpace!==On&&"COLOR_0"in n&&console.warn(`THREE.GLTFLoader: Converting vertex colors from "srgb-linear" to "${Rt.workingColorSpace}" not supported.`),Oi(s,e),ry(s,e,t),Promise.all(i).then(function(){return e.targets!==void 0?ey(s,e.targets,t):s})}var Cp={type:"change"},Wu={type:"start"},Ip={type:"end"},Rc=new Ci,Pp=new Kn,ay=Math.cos(70*It.DEG2RAD),_n=new k,Vn=2*Math.PI,Jt={NONE:-1,ROTATE:0,DOLLY:1,PAN:2,TOUCH_ROTATE:3,TOUCH_PAN:4,TOUCH_DOLLY_PAN:5,TOUCH_DOLLY_ROTATE:6},Vu=1e-6,Cc=class extends qa{constructor(e,t=null){super(e,t),this.state=Jt.NONE,this.target=new k,this.cursor=new k,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.minTargetRadius=0,this.maxTargetRadius=1/0,this.minPolarAngle=0,this.maxPolarAngle=Math.PI,this.minAzimuthAngle=-1/0,this.maxAzimuthAngle=1/0,this.enableDamping=!1,this.dampingFactor=.05,this.enableZoom=!0,this.zoomSpeed=1,this.enableRotate=!0,this.rotateSpeed=1,this.keyRotateSpeed=1,this.enablePan=!0,this.panSpeed=1,this.screenSpacePanning=!0,this.keyPanSpeed=7,this.zoomToCursor=!1,this.autoRotate=!1,this.autoRotateSpeed=2,this.keys={LEFT:"ArrowLeft",UP:"ArrowUp",RIGHT:"ArrowRight",BOTTOM:"ArrowDown"},this.mouseButtons={LEFT:vs.ROTATE,MIDDLE:vs.DOLLY,RIGHT:vs.PAN},this.touches={ONE:_s.ROTATE,TWO:_s.DOLLY_PAN},this.target0=this.target.clone(),this.position0=this.object.position.clone(),this.zoom0=this.object.zoom,this._cursorStyle="auto",this._domElementKeyEvents=null,this._lastPosition=new k,this._lastQuaternion=new $t,this._lastTargetPosition=new k,this._quat=new $t().setFromUnitVectors(e.up,new k(0,1,0)),this._quatInverse=this._quat.clone().invert(),this._spherical=new zr,this._sphericalDelta=new zr,this._scale=1,this._panOffset=new k,this._rotateStart=new ze,this._rotateEnd=new ze,this._rotateDelta=new ze,this._panStart=new ze,this._panEnd=new ze,this._panDelta=new ze,this._dollyStart=new ze,this._dollyEnd=new ze,this._dollyDelta=new ze,this._dollyDirection=new k,this._mouse=new ze,this._performCursorZoom=!1,this._pointers=[],this._pointerPositions={},this._controlActive=!1,this._onPointerMove=ly.bind(this),this._onPointerDown=oy.bind(this),this._onPointerUp=cy.bind(this),this._onContextMenu=gy.bind(this),this._onMouseWheel=dy.bind(this),this._onKeyDown=fy.bind(this),this._onTouchStart=py.bind(this),this._onTouchMove=my.bind(this),this._onMouseDown=hy.bind(this),this._onMouseMove=uy.bind(this),this._interceptControlDown=xy.bind(this),this._interceptControlUp=by.bind(this),this.domElement!==null&&this.connect(this.domElement),this.update()}set cursorStyle(e){this._cursorStyle=e,e==="grab"?this.domElement.style.cursor="grab":this.domElement.style.cursor="auto"}get cursorStyle(){return this._cursorStyle}connect(e){super.connect(e),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerUp),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.getRootNode().addEventListener("keydown",this._interceptControlDown,{passive:!0,capture:!0}),this.domElement.style.touchAction="none"}disconnect(){this.state=Jt.NONE,this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerUp),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.stopListenToKeyEvents();let e=this.domElement.getRootNode();e.removeEventListener("keydown",this._interceptControlDown,{capture:!0}),e.removeEventListener("keyup",this._interceptControlUp,{capture:!0}),this._controlActive=!1,this._pointers.length=0,this._pointerPositions={},this.domElement.style.touchAction="",this.domElement.style.cursor="auto"}dispose(){this.disconnect()}getPolarAngle(){return this._spherical.phi}getAzimuthalAngle(){return this._spherical.theta}getDistance(){return this.object.position.distanceTo(this.target)}listenToKeyEvents(e){e.addEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=e}stopListenToKeyEvents(){this._domElementKeyEvents!==null&&(this._domElementKeyEvents.removeEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=null)}saveState(){this.target0.copy(this.target),this.position0.copy(this.object.position),this.zoom0=this.object.zoom}reset(){this.target.copy(this.target0),this.object.position.copy(this.position0),this.object.zoom=this.zoom0,this.object.updateProjectionMatrix(),this.dispatchEvent(Cp),this.update(),this.state=Jt.NONE}pan(e,t){this._pan(e,t),this.update()}dollyIn(e){this._dollyIn(e),this.update()}dollyOut(e){this._dollyOut(e),this.update()}rotateLeft(e){this._rotateLeft(e),this.update()}rotateUp(e){this._rotateUp(e),this.update()}update(e=null){let t=this.object.position;_n.copy(t).sub(this.target),_n.applyQuaternion(this._quat),this._spherical.setFromVector3(_n),this.autoRotate&&this.state===Jt.NONE&&this._rotateLeft(this._getAutoRotationAngle(e)),this.enableDamping?(this._spherical.theta+=this._sphericalDelta.theta*this.dampingFactor,this._spherical.phi+=this._sphericalDelta.phi*this.dampingFactor):(this._spherical.theta+=this._sphericalDelta.theta,this._spherical.phi+=this._sphericalDelta.phi);let n=this.minAzimuthAngle,i=this.maxAzimuthAngle;isFinite(n)&&isFinite(i)&&(n<-Math.PI?n+=Vn:n>Math.PI&&(n-=Vn),i<-Math.PI?i+=Vn:i>Math.PI&&(i-=Vn),n<=i?this._spherical.theta=Math.max(n,Math.min(i,this._spherical.theta)):this._spherical.theta=this._spherical.theta>(n+i)/2?Math.max(n,this._spherical.theta):Math.min(i,this._spherical.theta)),this._spherical.phi=Math.max(this.minPolarAngle,Math.min(this.maxPolarAngle,this._spherical.phi)),this._spherical.makeSafe(),this.enableDamping===!0?this.target.addScaledVector(this._panOffset,this.dampingFactor):this.target.add(this._panOffset),this.target.sub(this.cursor),this.target.clampLength(this.minTargetRadius,this.maxTargetRadius),this.target.add(this.cursor);let r=!1;if(this.zoomToCursor&&this._performCursorZoom||this.object.isOrthographicCamera)this._spherical.radius=this._clampDistance(this._spherical.radius);else{let a=this._spherical.radius;this._spherical.radius=this._clampDistance(this._spherical.radius*this._scale),r=a!=this._spherical.radius}if(_n.setFromSpherical(this._spherical),_n.applyQuaternion(this._quatInverse),t.copy(this.target).add(_n),this.object.lookAt(this.target),this.enableDamping===!0?(this._sphericalDelta.theta*=1-this.dampingFactor,this._sphericalDelta.phi*=1-this.dampingFactor,this._panOffset.multiplyScalar(1-this.dampingFactor)):(this._sphericalDelta.set(0,0,0),this._panOffset.set(0,0,0)),this.zoomToCursor&&this._performCursorZoom){let a=null;if(this.object.isPerspectiveCamera){let o=_n.length();a=this._clampDistance(o*this._scale);let l=o-a;this.object.position.addScaledVector(this._dollyDirection,l),this.object.updateMatrixWorld(),r=!!l}else if(this.object.isOrthographicCamera){let o=new k(this._mouse.x,this._mouse.y,0);o.unproject(this.object);let l=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),this.object.updateProjectionMatrix(),r=l!==this.object.zoom;let c=new k(this._mouse.x,this._mouse.y,0);c.unproject(this.object),this.object.position.sub(c).add(o),this.object.updateMatrixWorld(),a=_n.length()}else console.warn("WARNING: OrbitControls.js encountered an unknown camera type - zoom to cursor disabled."),this.zoomToCursor=!1;a!==null&&(this.screenSpacePanning?this.target.set(0,0,-1).transformDirection(this.object.matrix).multiplyScalar(a).add(this.object.position):(Rc.origin.copy(this.object.position),Rc.direction.set(0,0,-1).transformDirection(this.object.matrix),Math.abs(this.object.up.dot(Rc.direction))<ay?this.object.lookAt(this.target):(Pp.setFromNormalAndCoplanarPoint(this.object.up,this.target),Rc.intersectPlane(Pp,this.target))))}else if(this.object.isOrthographicCamera){let a=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),a!==this.object.zoom&&(this.object.updateProjectionMatrix(),r=!0)}return this._scale=1,this._performCursorZoom=!1,r||this._lastPosition.distanceToSquared(this.object.position)>Vu||8*(1-this._lastQuaternion.dot(this.object.quaternion))>Vu||this._lastTargetPosition.distanceToSquared(this.target)>Vu?(this.dispatchEvent(Cp),this._lastPosition.copy(this.object.position),this._lastQuaternion.copy(this.object.quaternion),this._lastTargetPosition.copy(this.target),!0):!1}_getAutoRotationAngle(e){return e!==null?Vn/60*this.autoRotateSpeed*e:Vn/60/60*this.autoRotateSpeed}_getZoomScale(e){let t=Math.abs(e*.01);return Math.pow(.95,this.zoomSpeed*t)}_rotateLeft(e){this._sphericalDelta.theta-=e}_rotateUp(e){this._sphericalDelta.phi-=e}_panLeft(e,t){_n.setFromMatrixColumn(t,0),_n.multiplyScalar(-e),this._panOffset.add(_n)}_panUp(e,t){this.screenSpacePanning===!0?_n.setFromMatrixColumn(t,1):(_n.setFromMatrixColumn(t,0),_n.crossVectors(this.object.up,_n)),_n.multiplyScalar(e),this._panOffset.add(_n)}_pan(e,t){let n=this.domElement;if(this.object.isPerspectiveCamera){let i=this.object.position;_n.copy(i).sub(this.target);let r=_n.length();r*=Math.tan(this.object.fov/2*Math.PI/180),this._panLeft(2*e*r/n.clientHeight,this.object.matrix),this._panUp(2*t*r/n.clientHeight,this.object.matrix)}else this.object.isOrthographicCamera?(this._panLeft(e*(this.object.right-this.object.left)/this.object.zoom/n.clientWidth,this.object.matrix),this._panUp(t*(this.object.top-this.object.bottom)/this.object.zoom/n.clientHeight,this.object.matrix)):(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - pan disabled."),this.enablePan=!1)}_dollyOut(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale/=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_dollyIn(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale*=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_updateZoomParameters(e,t){if(!this.zoomToCursor)return;this._performCursorZoom=!0;let n=this.domElement.getBoundingClientRect(),i=e-n.left,r=t-n.top,a=n.width,o=n.height;this._mouse.x=i/a*2-1,this._mouse.y=-(r/o)*2+1,this._dollyDirection.set(this._mouse.x,this._mouse.y,1).unproject(this.object).sub(this.object.position).normalize()}_clampDistance(e){return Math.max(this.minDistance,Math.min(this.maxDistance,e))}_handleMouseDownRotate(e){this._rotateStart.set(e.clientX,e.clientY)}_handleMouseDownDolly(e){this._updateZoomParameters(e.clientX,e.clientX),this._dollyStart.set(e.clientX,e.clientY)}_handleMouseDownPan(e){this._panStart.set(e.clientX,e.clientY)}_handleMouseMoveRotate(e){this._rotateEnd.set(e.clientX,e.clientY),this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Vn*this._rotateDelta.x/t.clientHeight),this._rotateUp(Vn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd),this.update()}_handleMouseMoveDolly(e){this._dollyEnd.set(e.clientX,e.clientY),this._dollyDelta.subVectors(this._dollyEnd,this._dollyStart),this._dollyDelta.y>0?this._dollyOut(this._getZoomScale(this._dollyDelta.y)):this._dollyDelta.y<0&&this._dollyIn(this._getZoomScale(this._dollyDelta.y)),this._dollyStart.copy(this._dollyEnd),this.update()}_handleMouseMovePan(e){this._panEnd.set(e.clientX,e.clientY),this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd),this.update()}_handleMouseWheel(e){this._updateZoomParameters(e.clientX,e.clientY),e.deltaY<0?this._dollyIn(this._getZoomScale(e.deltaY)):e.deltaY>0&&this._dollyOut(this._getZoomScale(e.deltaY)),this.update()}_handleKeyDown(e){let t=!1;switch(e.code){case this.keys.UP:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(Vn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,this.keyPanSpeed),t=!0;break;case this.keys.BOTTOM:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(-Vn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,-this.keyPanSpeed),t=!0;break;case this.keys.LEFT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(Vn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(this.keyPanSpeed,0),t=!0;break;case this.keys.RIGHT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(-Vn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(-this.keyPanSpeed,0),t=!0;break}t&&(e.preventDefault(),this.update())}_handleTouchStartRotate(e){if(this._pointers.length===1)this._rotateStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._rotateStart.set(n,i)}}_handleTouchStartPan(e){if(this._pointers.length===1)this._panStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panStart.set(n,i)}}_handleTouchStartDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyStart.set(0,r)}_handleTouchStartDollyPan(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enablePan&&this._handleTouchStartPan(e)}_handleTouchStartDollyRotate(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enableRotate&&this._handleTouchStartRotate(e)}_handleTouchMoveRotate(e){if(this._pointers.length==1)this._rotateEnd.set(e.pageX,e.pageY);else{let n=this._getSecondPointerPosition(e),i=.5*(e.pageX+n.x),r=.5*(e.pageY+n.y);this._rotateEnd.set(i,r)}this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Vn*this._rotateDelta.x/t.clientHeight),this._rotateUp(Vn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd)}_handleTouchMovePan(e){if(this._pointers.length===1)this._panEnd.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panEnd.set(n,i)}this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd)}_handleTouchMoveDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyEnd.set(0,r),this._dollyDelta.set(0,Math.pow(this._dollyEnd.y/this._dollyStart.y,this.zoomSpeed)),this._dollyOut(this._dollyDelta.y),this._dollyStart.copy(this._dollyEnd);let a=(e.pageX+t.x)*.5,o=(e.pageY+t.y)*.5;this._updateZoomParameters(a,o)}_handleTouchMoveDollyPan(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enablePan&&this._handleTouchMovePan(e)}_handleTouchMoveDollyRotate(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enableRotate&&this._handleTouchMoveRotate(e)}_addPointer(e){this._pointers.push(e.pointerId)}_removePointer(e){delete this._pointerPositions[e.pointerId];for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId){this._pointers.splice(t,1);return}}_isTrackingPointer(e){for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId)return!0;return!1}_trackPointer(e){let t=this._pointerPositions[e.pointerId];t===void 0&&(t=new ze,this._pointerPositions[e.pointerId]=t),t.set(e.pageX,e.pageY)}_getSecondPointerPosition(e){let t=e.pointerId===this._pointers[0]?this._pointers[1]:this._pointers[0];return this._pointerPositions[t]}_customWheelEvent(e){let t=e.deltaMode,n={clientX:e.clientX,clientY:e.clientY,deltaY:e.deltaY};switch(t){case 1:n.deltaY*=16;break;case 2:n.deltaY*=100;break}return e.ctrlKey&&!this._controlActive&&(n.deltaY*=10),n}};function oy(s){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(s.pointerId),this.domElement.ownerDocument.addEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.addEventListener("pointerup",this._onPointerUp)),!this._isTrackingPointer(s)&&(this._addPointer(s),s.pointerType==="touch"?this._onTouchStart(s):this._onMouseDown(s),this._cursorStyle==="grab"&&(this.domElement.style.cursor="grabbing")))}function ly(s){this.enabled!==!1&&(s.pointerType==="touch"?this._onTouchMove(s):this._onMouseMove(s))}function cy(s){switch(this._removePointer(s),this._pointers.length){case 0:this.domElement.releasePointerCapture(s.pointerId),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.dispatchEvent(Ip),this.state=Jt.NONE,this._cursorStyle==="grab"&&(this.domElement.style.cursor="grab");break;case 1:let e=this._pointers[0],t=this._pointerPositions[e];this._onTouchStart({pointerId:e,pageX:t.x,pageY:t.y});break}}function hy(s){let e;switch(s.button){case 0:e=this.mouseButtons.LEFT;break;case 1:e=this.mouseButtons.MIDDLE;break;case 2:e=this.mouseButtons.RIGHT;break;default:e=-1}switch(e){case vs.DOLLY:if(this.enableZoom===!1)return;this._handleMouseDownDolly(s),this.state=Jt.DOLLY;break;case vs.ROTATE:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Jt.PAN}else{if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Jt.ROTATE}break;case vs.PAN:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Jt.ROTATE}else{if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Jt.PAN}break;default:this.state=Jt.NONE}this.state!==Jt.NONE&&this.dispatchEvent(Wu)}function uy(s){switch(this.state){case Jt.ROTATE:if(this.enableRotate===!1)return;this._handleMouseMoveRotate(s);break;case Jt.DOLLY:if(this.enableZoom===!1)return;this._handleMouseMoveDolly(s);break;case Jt.PAN:if(this.enablePan===!1)return;this._handleMouseMovePan(s);break}}function dy(s){this.enabled===!1||this.enableZoom===!1||this.state!==Jt.NONE||(s.preventDefault(),this.dispatchEvent(Wu),this._handleMouseWheel(this._customWheelEvent(s)),this.dispatchEvent(Ip))}function fy(s){this.enabled!==!1&&this._handleKeyDown(s)}function py(s){switch(this._trackPointer(s),this._pointers.length){case 1:switch(this.touches.ONE){case _s.ROTATE:if(this.enableRotate===!1)return;this._handleTouchStartRotate(s),this.state=Jt.TOUCH_ROTATE;break;case _s.PAN:if(this.enablePan===!1)return;this._handleTouchStartPan(s),this.state=Jt.TOUCH_PAN;break;default:this.state=Jt.NONE}break;case 2:switch(this.touches.TWO){case _s.DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchStartDollyPan(s),this.state=Jt.TOUCH_DOLLY_PAN;break;case _s.DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchStartDollyRotate(s),this.state=Jt.TOUCH_DOLLY_ROTATE;break;default:this.state=Jt.NONE}break;default:this.state=Jt.NONE}this.state!==Jt.NONE&&this.dispatchEvent(Wu)}function my(s){switch(this._trackPointer(s),this.state){case Jt.TOUCH_ROTATE:if(this.enableRotate===!1)return;this._handleTouchMoveRotate(s),this.update();break;case Jt.TOUCH_PAN:if(this.enablePan===!1)return;this._handleTouchMovePan(s),this.update();break;case Jt.TOUCH_DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchMoveDollyPan(s),this.update();break;case Jt.TOUCH_DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchMoveDollyRotate(s),this.update();break;default:this.state=Jt.NONE}}function gy(s){this.enabled!==!1&&s.preventDefault()}function xy(s){s.key==="Control"&&(this._controlActive=!0,this.domElement.getRootNode().addEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}function by(s){s.key==="Control"&&(this._controlActive=!1,this.domElement.getRootNode().removeEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}var Zr=(function(){var s="b9H79Tebbbe8Fv9Gbb9Gvuuuuueu9Giuuub9Geueu9Giuuueuixkbeeeddddillviebeoweuecj:Gdkr;Neqo9TW9T9VV95dbH9F9F939H79T9F9J9H229F9Jt9VV7bb8A9TW79O9V9Wt9F9KW9J9V9KW9wWVtW949c919M9MWVbeY9TW79O9V9Wt9F9KW9J9V9KW69U9KW949c919M9MWVbdE9TW79O9V9Wt9F9KW9J9V9KW69U9KW949tWG91W9U9JWbiL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9p9JtblK9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9r919HtbvL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVT949WboY9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVJ9V29VVbrl79IV9Rbwq:VZkdbk:XYi5ud9:du8Jjjjjbcj;kb9Rgv8Kjjjjbc9:hodnalTmbcuhoaiRbbgrc;WeGc:Ge9hmbarcsGgwce0mbc9:hoalcufadcd4cbawEgDadfgrcKcaawEgqaraq0Egk6mbaicefhxcj;abad9Uc;WFbGcjdadca0EhmaialfgPar9Rgoadfhsavaoadz:jjjjbgzceVhHcbhOdndninaeaO9nmeaPax9RaD6mdamaeaO9RaOamfgoae6EgAcsfglc9WGhCabaOad2fhXaAcethQaxaDfhiaOaeaoaeao6E9RhLalcl4cifcd4hKazcj;cbfaAfhYcbh8AazcjdfhEaHh3incbh5dnawTmbaxa8Acd4fRbbh5kcbh8Eazcj;cbfhqinaih8Fdndndndna5a8Ecet4ciGgoc9:fPdebdkaPa8F9RaA6mrazcj;cbfa8EaA2fa8FaAz:jjjjb8Aa8FaAfhixdkazcj;cbfa8EaA2fcbaAz:kjjjb8Aa8FhixekaPa8F9RaK6mva8FaKfhidnaCTmbaPai9RcK6mbaocdtc:q:G:cjbfcj:G:cjbawEhaczhrcbhlinargoc9Wfghaqfhrdndndndndndnaaa8Fahco4fRbbalcoG4ciGcdtfydbPDbedvivvvlvkar9cb83bwar9cb83bbxlkarcbaiRbdai8Xbb9c:c:qj:bw9:9c:q;c1:I1e:d9c:b:c:e1z9:gg9cjjjjjz:dg8J9qE86bbaqaofgrcGfcbaicdfa8J9c8N1:NfghRbbag9cjjjjjw:dg8J9qE86bbarcVfcbaha8J9c8M1:NfghRbbag9cjjjjjl:dg8J9qE86bbarc7fcbaha8J9c8L1:NfghRbbag9cjjjjjd:dg8J9qE86bbarctfcbaha8J9c8K1:NfghRbbag9cjjjjje:dg8J9qE86bbarc91fcbaha8J9c8J1:NfghRbbag9cjjjj;ab:dg8J9qE86bbarc4fcbaha8J9cg1:NfghRbbag9cjjjja:dg8J9qE86bbarc93fcbaha8J9ch1:NfghRbbag9cjjjjz:dgg9qE86bbarc94fcbahag9ca1:NfghRbbai8Xbe9c:c:qj:bw9:9c:q;c1:I1e:d9c:b:c:e1z9:gg9cjjjjjz:dg8J9qE86bbarc95fcbaha8J9c8N1:NfgiRbbag9cjjjjjw:dg8J9qE86bbarc96fcbaia8J9c8M1:NfgiRbbag9cjjjjjl:dg8J9qE86bbarc97fcbaia8J9c8L1:NfgiRbbag9cjjjjjd:dg8J9qE86bbarc98fcbaia8J9c8K1:NfgiRbbag9cjjjjje:dg8J9qE86bbarc99fcbaia8J9c8J1:NfgiRbbag9cjjjj;ab:dg8J9qE86bbarc9:fcbaia8J9cg1:NfgiRbbag9cjjjja:dg8J9qE86bbarcufcbaia8J9ch1:NfgiRbbag9cjjjjz:dgg9qE86bbaiag9ca1:NfhixikaraiRblaiRbbghco4g8Ka8KciSg8KE86bbaqaofgrcGfaiclfa8Kfg8KRbbahcl4ciGg8La8LciSg8LE86bbarcVfa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc7fa8Ka8Lfg8KRbbahciGghahciSghE86bbarctfa8Kahfg8KRbbaiRbeghco4g8La8LciSg8LE86bbarc91fa8Ka8Lfg8KRbbahcl4ciGg8La8LciSg8LE86bbarc4fa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc93fa8Ka8Lfg8KRbbahciGghahciSghE86bbarc94fa8Kahfg8KRbbaiRbdghco4g8La8LciSg8LE86bbarc95fa8Ka8Lfg8KRbbahcl4ciGg8La8LciSg8LE86bbarc96fa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc97fa8Ka8Lfg8KRbbahciGghahciSghE86bbarc98fa8KahfghRbbaiRbigico4g8Ka8KciSg8KE86bbarc99faha8KfghRbbaicl4ciGg8Ka8KciSg8KE86bbarc9:faha8KfghRbbaicd4ciGg8Ka8KciSg8KE86bbarcufaha8KfgrRbbaiciGgiaiciSgiE86bbaraifhixdkaraiRbwaiRbbghcl4g8Ka8KcsSg8KE86bbaqaofgrcGfaicwfa8Kfg8KRbbahcsGghahcsSghE86bbarcVfa8KahfghRbbaiRbeg8Kcl4g8La8LcsSg8LE86bbarc7faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarctfaha8KfghRbbaiRbdg8Kcl4g8La8LcsSg8LE86bbarc91faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc4faha8KfghRbbaiRbig8Kcl4g8La8LcsSg8LE86bbarc93faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc94faha8KfghRbbaiRblg8Kcl4g8La8LcsSg8LE86bbarc95faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc96faha8KfghRbbaiRbvg8Kcl4g8La8LcsSg8LE86bbarc97faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc98faha8KfghRbbaiRbog8Kcl4g8La8LcsSg8LE86bbarc99faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc9:faha8KfghRbbaiRbrgicl4g8Ka8KcsSg8KE86bbarcufaha8KfgrRbbaicsGgiaicsSgiE86bbaraifhixekarai8Pbw83bwarai8Pbb83bbaiczfhikdnaoaC9pmbalcdfhlaoczfhraPai9RcL0mekkaoaC6moaimexokaCmva8FTmvkaqaAfhqa8Ecefg8Ecl9hmbkdndndndnawTmbasa8Acd4fRbbgociGPlbedrbkaATmdaza8Afh8Fazcj;cbfhhcbh8EaEhaina8FRbbhraahocbhlinaoahalfRbbgqce4cbaqceG9R7arfgr86bbaoadfhoaAalcefgl9hmbkaacefhaa8Fcefh8FahaAfhha8Ecefg8Ecl9hmbxikkaATmeaza8Afhaazcj;cbfhhcbhoceh8EaYh8FinaEaofhlaa8Vbbhrcbhoinala8FaofRbbcwtahaofRbbgqVc;:FiGce4cbaqceG9R7arfgr87bbaladfhlaLaocefgofmbka8FaQfh8FcdhoaacdfhaahaQfhha8EceGhlcbh8EalmbxdkkaATmbaocl4h8Eaza8AfRbbhqcwhoa3hlinalRbbaotaqVhqalcefhlaocwfgoca9hmbkcbhhaEh8FaYhainazcj;cbfahfRbbhrcwhoaahlinalRbbaotarVhralaAfhlaocwfgoca9hmbkara8E94aq7hqcbhoa8Fhlinalaqao486bbalcefhlaocwfgoca9hmbka8Fadfh8FaacefhaahcefghaA9hmbkkaEclfhEa3clfh3a8Aclfg8Aad6mbkaXazcjdfaAad2z:jjjjb8AazazcjdfaAcufad2fadz:jjjjb8AaAaOfhOaihxaimbkc9:hoxdkcbc99aPax9RakSEhoxekc9:hokavcj;kbf8Kjjjjbaok:ysezu8Jjjjjbc;ae9Rgv8Kjjjjbc9:hodnalaeci9UgrcHf6mbcuhoaiRbbgwc;WeGc;Ge9hmbawcsGgDce0mbavc;abfcFecjez:kjjjb8Aav9cu83iUav9cu83i8Wav9cu83iyav9cu83iaav9cu83iKav9cu83izav9cu83iwav9cu83ibaialfc9WfhqaicefgwarfhldnaeTmbcmcsaDceSEhkcbhxcbhmcbhrcbhicbhoindnalaq9nmbc9:hoxikdndnawRbbgDc;Ve0mbavc;abfaoaDcu7gPcl4fcsGcitfgsydlhzasydbhHdndnaDcsGgsak9pmbavaiaPfcsGcdtfydbaxasEhDaxasTgOfhxxekdndnascsSmbcehOasc987asamffcefhDxekalcefhDal8SbbgscFeGhPdndnascu9mmbaDhlxekalcvfhlaPcFbGhPcrhsdninaD8SbbgOcFbGastaPVhPaOcu9kmeaDcefhDascrfgsc8J9hmbxdkkaDcefhlkcehOaPce4cbaPceG9R7amfhDkaDhmkavc;abfaocitfgsaDBdbasazBdlavaicdtfaDBdbavc;abfaocefcsGcitfgsaHBdbasaDBdlaocdfhoaOaifhidnadcd9hmbabarcetfgsaH87ebasclfaD87ebascdfaz87ebxdkabarcdtfgsaHBdbascwfaDBdbasclfazBdbxekdnaDcpe0mbavaiaqaDcsGfRbbgscl4gP9RcsGcdtfydbaxcefgOaPEhDavaias9RcsGcdtfydbaOaPTgzfgOascsGgPEhsaPThPdndnadcd9hmbabarcetfgHax87ebaHclfas87ebaHcdfaD87ebxekabarcdtfgHaxBdbaHcwfasBdbaHclfaDBdbkavaicdtfaxBdbavc;abfaocitfgHaDBdbaHaxBdlavaicefgicsGcdtfaDBdbavc;abfaocefcsGcitfgHasBdbaHaDBdlavaiazfgicsGcdtfasBdbavc;abfaocdfcsGcitfgDaxBdbaDasBdlaocifhoaiaPfhiaOaPfhxxekaxcbalRbbgsEgHaDc;:eSgDfhOascsGhAdndnascl4gCmbaOcefhzxekaOhzavaiaC9RcsGcdtfydbhOkdndnaAmbazcefhxxekazhxavaias9RcsGcdtfydbhzkdndnaDTmbalcefhDxekalcdfhDal8SbegPcFeGhsdnaPcu9kmbalcofhHascFbGhscrhldninaD8SbbgPcFbGaltasVhsaPcu9kmeaDcefhDalcrfglc8J9hmbkaHhDxekaDcefhDkasce4cbasceG9R7amfgmhHkdndnaCcsSmbaDhsxekaDcefhsaD8SbbglcFeGhPdnalcu9kmbaDcvfhOaPcFbGhPcrhldninas8SbbgDcFbGaltaPVhPaDcu9kmeascefhsalcrfglc8J9hmbkaOhsxekascefhskaPce4cbaPceG9R7amfgmhOkdndnaAcsSmbashlxekascefhlas8SbbgDcFeGhPdnaDcu9kmbascvfhzaPcFbGhPcrhDdninal8SbbgscFbGaDtaPVhPascu9kmealcefhlaDcrfgDc8J9hmbkazhlxekalcefhlkaPce4cbaPceG9R7amfgmhzkdndnadcd9hmbabarcetfgDaH87ebaDclfaz87ebaDcdfaO87ebxekabarcdtfgDaHBdbaDcwfazBdbaDclfaOBdbkavc;abfaocitfgDaOBdbaDaHBdlavaicdtfaHBdbavc;abfaocefcsGcitfgDazBdbaDaOBdlavaicefgicsGcdtfaOBdbavc;abfaocdfcsGcitfgDaHBdbaDazBdlavaiaCTaCcsSVfgicsGcdtfazBdbaiaATaAcsSVfhiaocifhokawcefhwaocsGhoaicsGhiarcifgrae6mbkkcbc99alaqSEhokavc;aef8Kjjjjbaok:clevu8Jjjjjbcz9Rhvdnalaecvf9pmbc9:skdnaiRbbc;:eGc;qeSmbcuskav9cb83iwaicefhoaialfc98fhrdnaeTmbdnadcdSmbcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcdtfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgiBdbalaiBdbawcefgwae9hmbxdkkcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcetfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgi87ebalaiBdbawcefgwae9hmbkkcbc99aoarSEk:Lvoeue99dud99eud99dndnadcl9hmbaeTmeindndnabcdfgd8Sbb:Yab8Sbbgi:Ygl:l:tabcefgv8Sbbgo:Ygr:l:tgwJbb;:9cawawNJbbbbawawJbbbb9GgDEgq:mgkaqaicb9iEalMgwawNakaqaocb9iEarMgqaqNMM:r:vglNJbbbZJbbb:;aDEMgr:lJbbb9p9DTmbar:Ohixekcjjjj94hikadai86bbdndnaqalNJbbbZJbbb:;aqJbbbb9GEMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkavad86bbdndnawalNJbbbZJbbb:;awJbbbb9GEMgw:lJbbb9p9DTmbaw:Ohdxekcjjjj94hdkabad86bbabclfhbaecufgembxdkkaeTmbindndnabclfgd8Ueb:Yab8Uebgi:Ygl:l:tabcdfgv8Uebgo:Ygr:l:tgwJb;:FSawawNJbbbbawawJbbbb9GgDEgq:mgkaqaicb9iEalMgwawNakaqaocb9iEarMgqaqNMM:r:vglNJbbbZJbbb:;aDEMgr:lJbbb9p9DTmbar:Ohixekcjjjj94hikadai87ebdndnaqalNJbbbZJbbb:;aqJbbbb9GEMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkavad87ebdndnawalNJbbbZJbbb:;awJbbbb9GEMgw:lJbbb9p9DTmbaw:Ohdxekcjjjj94hdkabad87ebabcwfhbaecufgembkkk:4ioiue99dud99dud99dnaeTmbcbhiabhlindndnal8Uebgv:YgoJ:ji:1Salcof8UebgrciVgw:Y:vgDNJbbbZJbbb:;avcu9kEMgq:lJbbb9p9DTmbaq:Ohkxekcjjjj94hkkalclf8Uebhvalcdf8UebhxalarcefciGcetfak87ebdndnax:YgqaDNJbbbZJbbb:;axcu9kEMgm:lJbbb9p9DTmbam:Ohxxekcjjjj94hxkabaiarciGgkfcd7cetfax87ebdndnav:YgmaDNJbbbZJbbb:;avcu9kEMgP:lJbbb9p9DTmbaP:Ohvxekcjjjj94hvkalarcufciGcetfav87ebdndnawaw2:ZgPaPMaoaoN:taqaqN:tamamN:tgoJbbbbaoJbbbb9GE:raDNJbbbZMgD:lJbbb9p9DTmbaD:Ohrxekcjjjj94hrkalakcetfar87ebalcwfhlaiclfhiaecufgembkkk9mbdnadcd4ae2gdTmbinababydbgecwtcw91:Yaece91cjjj98Gcjjj;8if::NUdbabclfhbadcufgdmbkkk:Tvirud99eudndnadcl9hmbaeTmeindndnabRbbgiabcefgl8Sbbgvabcdfgo8Sbbgrf9R:YJbbuJabcifgwRbbgdce4adVgDcd4aDVgDcl4aDVgD:Z:vgqNJbbbZMgk:lJbbb9p9DTmbak:Ohxxekcjjjj94hxkaoax86bbdndnaraif:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohoxekcjjjj94hokalao86bbdndnavaifar9R:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohixekcjjjj94hikabai86bbdndnaDadcetGadceGV:ZaqNJbbbZMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkawad86bbabclfhbaecufgembxdkkaeTmbindndnab8Vebgiabcdfgl8Uebgvabclfgo8Uebgrf9R:YJbFu9habcofgw8Vebgdce4adVgDcd4aDVgDcl4aDVgDcw4aDVgD:Z:vgqNJbbbZMgk:lJbbb9p9DTmbak:Ohxxekcjjjj94hxkaoax87ebdndnaraif:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohoxekcjjjj94hokalao87ebdndnavaifar9R:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohixekcjjjj94hikabai87ebdndnaDadcetGadceGV:ZaqNJbbbZMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkawad87ebabcwfhbaecufgembkkk9teiucbcbyd:K:G:cjbgeabcifc98GfgbBd:K:G:cjbdndnabZbcztgd9nmbcuhiabad9RcFFifcz4nbcuSmekaehikaik;LeeeudndnaeabVciGTmbabhixekdndnadcz9pmbabhixekabhiinaiaeydbBdbaiclfaeclfydbBdbaicwfaecwfydbBdbaicxfaecxfydbBdbaeczfheaiczfhiadc9Wfgdcs0mbkkadcl6mbinaiaeydbBdbaeclfheaiclfhiadc98fgdci0mbkkdnadTmbinaiaeRbb86bbaicefhiaecefheadcufgdmbkkabk;aeedudndnabciGTmbabhixekaecFeGc:b:c:ew2hldndnadcz9pmbabhixekabhiinaialBdbaicxfalBdbaicwfalBdbaiclfalBdbaiczfhiadc9Wfgdcs0mbkkadcl6mbinaialBdbaiclfhiadc98fgdci0mbkkdnadTmbinaiae86bbaicefhiadcufgdmbkkabkk83dbcj:Gdk8Kbbbbdbbblbbbwbbbbbbbebbbdbbblbbbwbbbbc:K:Gdkl8W:qbb",e="b9H79TebbbeKl9Gbb9Gvuuuuueu9Giuuub9Geueuixkbbebeeddddilve9Weeeviebeoweuecj:Gdkr;Neqo9TW9T9VV95dbH9F9F939H79T9F9J9H229F9Jt9VV7bb8A9TW79O9V9Wt9F9KW9J9V9KW9wWVtW949c919M9MWVbdY9TW79O9V9Wt9F9KW9J9V9KW69U9KW949c919M9MWVblE9TW79O9V9Wt9F9KW9J9V9KW69U9KW949tWG91W9U9JWbvL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9p9JtboK9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9r919HtbrL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVT949WbwY9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVJ9V29VVbDl79IV9Rbqq:W9Dklbzik94evu8Jjjjjbcz9Rhbcbheincbhdcbhiinabcwfadfaicjuaead4ceGglE86bbaialfhiadcefgdcw9hmbkaeai86b:q:W:cjbaecitab8Piw83i:q:G:cjbaecefgecjd9hmbkk:JBl8Aud97dur978Jjjjjbcj;kb9Rgv8Kjjjjbc9:hodnalTmbcuhoaiRbbgrc;WeGc:Ge9hmbarcsGgwce0mbc9:hoalcufadcd4cbawEgDadfgrcKcaawEgqaraq0Egk6mbaialfgxar9RhodnadTgmmbavaoad;8qbbkaicefhPcj;abad9Uc;WFbGcjdadca0EhsdndndnadTmbaoadfhzcbhHinaeaH9nmdaxaP9RaD6miabaHad2fhOaPaDfhAasaeaH9RaHasfae6EgCcsfgocl4cifcd4hXavcj;cbfaoc9WGgQcetfhLavcj;cbfaQci2fhKavcj;cbfaQfhYcbh8Aaoc;ab6hEincbh3dnawTmbaPa8Acd4fRbbh3kcbh5avcj;cbfh8Eindndndndna3a5cet4ciGgoc9:fPdebdkaxaA9RaQ6mwdnaQTmbavcj;cbfa5aQ2faAaQ;8qbbkaAaCfhAxdkaQTmeavcj;cbfa5aQ2fcbaQ;8kbxekaxaA9RaX6moaoclVcbawEhraAaXfhocbhidnaEmbaxao9Rc;Gb6mbcbhlina8EalfhidndndndndndnaAalco4fRbbgqciGarfPDbedibledibkaipxbbbbbbbbbbbbbbbbpklbxlkaiaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaiaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaiaopbbbpklbaoczfhoxekaiaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqcd4ciGarfPDbedibledibkaiczfpxbbbbbbbbbbbbbbbbpklbxlkaiczfaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaiczfaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaiczfaopbbbpklbaoczfhoxekaiczfaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqcl4ciGarfPDbedibledibkaicafpxbbbbbbbbbbbbbbbbpklbxlkaicafaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaicafaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaicafaopbbbpklbaoczfhoxekaicafaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqco4arfPDbedibledibkaic8Wfpxbbbbbbbbbbbbbbbbpklbxlkaic8Wfaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Ngicitpbi:q:G:cjbaiRb:q:W:cjbgipsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Ngqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbaiaoclffaqRb:q:W:cjbfhoxikaic8Wfaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Ngicitpbi:q:G:cjbaiRb:q:W:cjbgipsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Ngqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbaiaocwffaqRb:q:W:cjbfhoxdkaic8Wfaopbbbpklbaoczfhoxekaic8WfaopbbdaoRbbgicitpbi:q:G:cjbaiRb:q:W:cjbgipsaoRbegqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbaiaocdffaqRb:q:W:cjbfhokalc;abfhialcjefaQ0meaihlaxao9Rc;Fb0mbkkdnaiaQ9pmbaici4hlinaxao9RcK6mwa8EaifhqdndndndndndnaAaico4fRbbalcoG4ciGarfPDbedibledibkaqpxbbbbbbbbbbbbbbbbpkbbxlkaqaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spkbbahaoclffagRb:q:W:cjbfhoxikaqaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spkbbahaocwffagRb:q:W:cjbfhoxdkaqaopbbbpkbbaoczfhoxekaqaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpkbbahaocdffagRb:q:W:cjbfhokalcdfhlaiczfgiaQ6mbkkaohAaoTmoka8EaQfh8Ea5cefg5cl9hmbkdndndndnawTmbaza8Acd4fRbbglciGPlbedwbkaQTmdavcjdfa8Afhlava8Afpbdbh8Jcbhoinalavcj;cbfaofpblbg8KaYaofpblbg8LpmbzeHdOiAlCvXoQrLg8MaLaofpblbg8NaKaofpblbgypmbzeHdOiAlCvXoQrLg8PpmbezHdiOAlvCXorQLg8Fcep9Ta8Fpxeeeeeeeeeeeeeeeegap9op9Hp9rg8Fa8Jp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ma8PpmwDKYqk8AExm35Ps8E8Fg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ka8LpmwKDYq8AkEx3m5P8Es8Fg8Ka8NaypmwKDYq8AkEx3m5P8Es8Fg8LpmbezHdiOAlvCXorQLg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ka8LpmwDKYqk8AExm35Ps8E8Fg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ugap9Abbbaladfglaaa8Fa8Fpmlvorlvorlvorlvorp9Ugap9Abbbaladfglaaa8Fa8FpmwDqkwDqkwDqkwDqkp9Ugap9Abbbaladfglaaa8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9AbbbaladfhlaoczfgoaQ6mbxikkaQTmeavcjdfa8Afhlava8Afpbdbh8Jcbhoinalavcj;cbfaofpblbg8KaYaofpblbg8LpmbzeHdOiAlCvXoQrLg8MaLaofpblbg8NaKaofpblbgypmbzeHdOiAlCvXoQrLg8PpmbezHdiOAlvCXorQLg8Fcep:nea8Fpxebebebebebebebebgap9op:bep9rg8Fa8Jp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ma8PpmwDKYqk8AExm35Ps8E8Fg8Fcep:nea8Faap9op:bep9rg8Fp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ka8LpmwKDYq8AkEx3m5P8Es8Fg8Ka8NaypmwKDYq8AkEx3m5P8Es8Fg8LpmbezHdiOAlvCXorQLg8Fcep:nea8Faap9op:bep9rg8Fp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ka8LpmwDKYqk8AExm35Ps8E8Fg8Fcep:nea8Faap9op:bep9rg8Fp:oegap9Abbbaladfglaaa8Fa8Fpmlvorlvorlvorlvorp:oegap9Abbbaladfglaaa8Fa8FpmwDqkwDqkwDqkwDqkp:oegap9Abbbaladfglaaa8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9AbbbaladfhlaoczfgoaQ6mbxdkkaQTmbcbhocbalcl4gl9Rc8FGhiavcjdfa8Afhrava8Afpbdbhainaravcj;cbfaofpblbg8JaYaofpblbg8KpmbzeHdOiAlCvXoQrLg8LaLaofpblbg8MaKaofpblbg8NpmbzeHdOiAlCvXoQrLgypmbezHdiOAlvCXorQLg8Faip:Rea8Falp:Tep9qg8Faap9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8LaypmwDKYqk8AExm35Ps8E8Fg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8Ja8KpmwKDYq8AkEx3m5P8Es8Fg8Ja8Ma8NpmwKDYq8AkEx3m5P8Es8Fg8KpmbezHdiOAlvCXorQLg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8Ja8KpmwDKYqk8AExm35Ps8E8Fg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9AbbbaradfhraoczfgoaQ6mbkka8Aclfg8Aad6mbkdnaCad2goTmbaOavcjdfao;8qbbkdnammbavavcjdfaCcufad2fad;8qbbkaCaHfhHc9:hoaAhPaAmbxlkkaeTmbaDalfhrcbhocuhlinaralaD9RglfaD6mdasaeao9Raoasfae6Eaofgoae6mbkaial9RhPkcbc99axaP9RakSEhoxekc9:hokavcj;kbf8Kjjjjbaokwbz:bjjjbkNsezu8Jjjjjbc;ae9Rgv8Kjjjjbc9:hodnalaeci9UgrcHf6mbcuhoaiRbbgwc;WeGc;Ge9hmbawcsGgDce0mbavc;abfcFecje;8kbav9cu83iUav9cu83i8Wav9cu83iyav9cu83iaav9cu83iKav9cu83izav9cu83iwav9cu83ibaialfc9WfhqaicefgwarfhldnaeTmbcmcsaDceSEhkcbhxcbhmcbhrcbhicbhoindnalaq9nmbc9:hoxikdndnawRbbgDc;Ve0mbavc;abfaoaDcu7gPcl4fcsGcitfgsydlhzasydbhHdndnaDcsGgsak9pmbavaiaPfcsGcdtfydbaxasEhDaxasTgOfhxxekdndnascsSmbcehOasc987asamffcefhDxekalcefhDal8SbbgscFeGhPdndnascu9mmbaDhlxekalcvfhlaPcFbGhPcrhsdninaD8SbbgOcFbGastaPVhPaOcu9kmeaDcefhDascrfgsc8J9hmbxdkkaDcefhlkcehOaPce4cbaPceG9R7amfhDkaDhmkavc;abfaocitfgsaDBdbasazBdlavaicdtfaDBdbavc;abfaocefcsGcitfgsaHBdbasaDBdlaocdfhoaOaifhidnadcd9hmbabarcetfgsaH87ebasclfaD87ebascdfaz87ebxdkabarcdtfgsaHBdbascwfaDBdbasclfazBdbxekdnaDcpe0mbavaiaqaDcsGfRbbgscl4gP9RcsGcdtfydbaxcefgOaPEhDavaias9RcsGcdtfydbaOaPTgzfgOascsGgPEhsaPThPdndnadcd9hmbabarcetfgHax87ebaHclfas87ebaHcdfaD87ebxekabarcdtfgHaxBdbaHcwfasBdbaHclfaDBdbkavaicdtfaxBdbavc;abfaocitfgHaDBdbaHaxBdlavaicefgicsGcdtfaDBdbavc;abfaocefcsGcitfgHasBdbaHaDBdlavaiazfgicsGcdtfasBdbavc;abfaocdfcsGcitfgDaxBdbaDasBdlaocifhoaiaPfhiaOaPfhxxekaxcbalRbbgsEgHaDc;:eSgDfhOascsGhAdndnascl4gCmbaOcefhzxekaOhzavaiaC9RcsGcdtfydbhOkdndnaAmbazcefhxxekazhxavaias9RcsGcdtfydbhzkdndnaDTmbalcefhDxekalcdfhDal8SbegPcFeGhsdnaPcu9kmbalcofhHascFbGhscrhldninaD8SbbgPcFbGaltasVhsaPcu9kmeaDcefhDalcrfglc8J9hmbkaHhDxekaDcefhDkasce4cbasceG9R7amfgmhHkdndnaCcsSmbaDhsxekaDcefhsaD8SbbglcFeGhPdnalcu9kmbaDcvfhOaPcFbGhPcrhldninas8SbbgDcFbGaltaPVhPaDcu9kmeascefhsalcrfglc8J9hmbkaOhsxekascefhskaPce4cbaPceG9R7amfgmhOkdndnaAcsSmbashlxekascefhlas8SbbgDcFeGhPdnaDcu9kmbascvfhzaPcFbGhPcrhDdninal8SbbgscFbGaDtaPVhPascu9kmealcefhlaDcrfgDc8J9hmbkazhlxekalcefhlkaPce4cbaPceG9R7amfgmhzkdndnadcd9hmbabarcetfgDaH87ebaDclfaz87ebaDcdfaO87ebxekabarcdtfgDaHBdbaDcwfazBdbaDclfaOBdbkavc;abfaocitfgDaOBdbaDaHBdlavaicdtfaHBdbavc;abfaocefcsGcitfgDazBdbaDaOBdlavaicefgicsGcdtfaOBdbavc;abfaocdfcsGcitfgDaHBdbaDazBdlavaiaCTaCcsSVfgicsGcdtfazBdbaiaATaAcsSVfhiaocifhokawcefhwaocsGhoaicsGhiarcifgrae6mbkkcbc99alaqSEhokavc;aef8Kjjjjbaok:clevu8Jjjjjbcz9Rhvdnalaecvf9pmbc9:skdnaiRbbc;:eGc;qeSmbcuskav9cb83iwaicefhoaialfc98fhrdnaeTmbdnadcdSmbcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcdtfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgiBdbalaiBdbawcefgwae9hmbxdkkcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcetfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgi87ebalaiBdbawcefgwae9hmbkkcbc99aoarSEk;Toio97eue97aec98Ghedndnadcl9hmbaeTmecbhdinababpbbbgicKp:RecKp:Sep;6eglaicwp:RecKp:Sep;6ealp;Geaiczp:RecKp:Sep;6egvp;Gep;Kep;Legopxbbbbbbbbbbbbbbbbp:2egralpxbbbjbbbjbbbjbbbjgwp9op9rp;Keglpxbb;:9cbb;:9cbb;:9cbb;:9calalp;Meaoaop;Meavaravawp9op9rp;Keglalp;Mep;Kep;Kep;Jep;Negvp;Mepxbbn0bbn0bbn0bbn0grp;KepxFbbbFbbbFbbbFbbbp9oaipxbbbFbbbFbbbFbbbFp9op9qalavp;Mearp;Kecwp:RepxbFbbbFbbbFbbbFbbp9op9qaoavp;Mearp;Keczp:RepxbbFbbbFbbbFbbbFbp9op9qpkbbabczfhbadclfgdae6mbxdkkaeTmbcbhdinabczfgDaDpbbbgipxbbbbbbFFbbbbbbFFgwp9oabpbbbgoaipmbediwDqkzHOAKY8AEgvczp:Reczp:Sep;6eglaoaipmlvorxmPsCXQL358E8FpxFubbFubbFubbFubbp9op;6eavczp:Sep;6egvp;Gealp;Gep;Kep;Legipxbbbbbbbbbbbbbbbbp:2egralpxbbbjbbbjbbbjbbbjgqp9op9rp;Keglpxb;:FSb;:FSb;:FSb;:FSalalp;Meaiaip;Meavaravaqp9op9rp;Keglalp;Mep;Kep;Kep;Jep;Negvp;Mepxbbn0bbn0bbn0bbn0grp;KepxFFbbFFbbFFbbFFbbp9oaiavp;Mearp;Keczp:Rep9qgialavp;Mearp;KepxFFbbFFbbFFbbFFbbp9oglpmwDKYqk8AExm35Ps8E8Fp9qpkbbabaoawp9oaialpmbezHdiOAlvCXorQLp9qpkbbabcafhbadclfgdae6mbkkk;2ileue97euo97dnaec98GgiTmbcbheinabcKfpx:ji:1S:ji:1S:ji:1S:ji:1SabpbbbglabczfgvpbbbgopmlvorxmPsCXQL358E8Fgrczp:Segwpxibbbibbbibbbibbbp9qp;6egDp;NegqaDaDp;MegDaDp;KealaopmbediwDqkzHOAKY8AEgDczp:Reczp:Sep;6eglalp;MeaDczp:Sep;6egoaop;Mearczp:Reczp:Sep;6egrarp;Mep;Kep;Kep;Lepxbbbbbbbbbbbbbbbbp:4ep;Jep;Mepxbbn0bbn0bbn0bbn0gDp;KepxFFbbFFbbFFbbFFbbgkp9oaqaop;MeaDp;Keczp:Rep9qgoaqalp;MeaDp;Keakp9oaqarp;MeaDp;Keczp:Rep9qgDpmwDKYqk8AExm35Ps8E8Fglp5eawclp:RegqpEi:T:j83ibavalp5baqpEd:T:j83ibabcwfaoaDpmbezHdiOAlvCXorQLgDp5eaqpEe:T:j83ibabaDp5baqpEb:T:j83ibabcafhbaeclfgeai6mbkkkuee97dnadcd4ae2c98GgeTmbcbhdinababpbbbgicwp:Recwp:Sep;6eaicep:SepxbbjFbbjFbbjFbbjFp9opxbbjZbbjZbbjZbbjZp:Uep;Mepkbbabczfhbadclfgdae6mbkkk:Sodw97euaec98Ghedndnadcl9hmbaeTmecbhdinabpxbbuJbbuJbbuJbbuJabpbbbgicKp:TeglaicYp:Tep9qgvcdp:Teavp9qgvclp:Teavp9qgop;6ep;Negvaicwp:RecKp:SegraipxFbbbFbbbFbbbFbbbgwp9ogDp:Uep;6ep;Mepxbbn0bbn0bbn0bbn0gqp;Kecwp:RepxbFbbbFbbbFbbbFbbp9oavaDarp:Xeaiczp:RecKp:Segip:Uep;6ep;Meaqp;Keawp9op9qavaDaraip:Uep:Xep;6ep;Meaqp;Keczp:RepxbbFbbbFbbbFbbbFbp9op9qavaoalcep:Rep9oalpxebbbebbbebbbebbbp9op9qp;6ep;Meaqp;KecKp:Rep9qpkbbabczfhbadclfgdae6mbxdkkaeTmbcbhdinabczfgkpxbFu9hbFu9hbFu9hbFu9habpbbbglakpbbbgrpmlvorxmPsCXQL358E8Fgvczp:TegqavcHp:Tep9qgicdp:Teaip9qgiclp:Teaip9qgicwp:Teaip9qgop;6ep;NegialarpmbediwDqkzHOAKY8AEgDpxFFbbFFbbFFbbFFbbglp9ograDczp:Segwp:Ueavczp:Reczp:SegDp:Xep;6ep;Mepxbbn0bbn0bbn0bbn0gvp;Kealp9oaiarawaDp:Uep:Xep;6ep;Meavp;Keczp:Rep9qgwaiaoaqcep:Rep9oaqpxebbbebbbebbbebbbp9op9qp;6ep;Meavp;Keczp:ReaiaDarp:Uep;6ep;Meavp;Kealp9op9qgipmwDKYqk8AExm35Ps8E8FpkbbabawaipmbezHdiOAlvCXorQLpkbbabcafhbadclfgdae6mbkkk9teiucbcbydj:G:cjbgeabcifc98GfgbBdj:G:cjbdndnabZbcztgd9nmbcuhiabad9RcFFifcz4nbcuSmekaehikaikkxebcj:Gdklz:zbb",t=new Uint8Array([0,97,115,109,1,0,0,0,1,4,1,96,0,0,3,3,2,0,0,5,3,1,0,1,12,1,0,10,22,2,12,0,65,0,65,0,65,0,252,10,0,0,11,7,0,65,0,253,15,26,11]),n=new Uint8Array([32,0,65,2,1,106,34,33,3,128,11,4,13,64,6,253,10,7,15,116,127,5,8,12,40,16,19,54,20,9,27,255,113,17,42,67,24,23,146,148,18,14,22,45,70,69,56,114,101,21,25,63,75,136,108,28,118,29,73,115]);if(typeof WebAssembly!="object")return{supported:!1};var i=WebAssembly.validate(t)?o(e):o(s),r,a=WebAssembly.instantiate(i,{}).then(function(f){r=f.instance,r.exports.__wasm_call_ctors()});function o(f){for(var y=new Uint8Array(f.length),T=0;T<f.length;++T){var g=f.charCodeAt(T);y[T]=g>96?g-97:g>64?g-39:g+4}for(var _=0,T=0;T<f.length;++T)y[_++]=y[T]<60?n[y[T]]:(y[T]-60)*64+y[++T];return y.buffer.slice(0,_)}function l(f,y,T,g,_,S,A){var b=f.exports.sbrk,C=g+3&-4,w=b(C*_),I=b(S.length),M=new Uint8Array(f.exports.memory.buffer);M.set(S,I);var N=y(w,g,_,I,S.length);if(N==0&&A&&A(w,C,_),T.set(M.subarray(w,w+g*_)),b(w-b(0)),N!=0)throw new Error("Malformed buffer data: "+N)}var c={NONE:"",OCTAHEDRAL:"meshopt_decodeFilterOct",QUATERNION:"meshopt_decodeFilterQuat",EXPONENTIAL:"meshopt_decodeFilterExp",COLOR:"meshopt_decodeFilterColor"},h={ATTRIBUTES:"meshopt_decodeVertexBuffer",TRIANGLES:"meshopt_decodeIndexBuffer",INDICES:"meshopt_decodeIndexSequence"},d=[],u=0;function p(f){var y={object:new Worker(f),pending:0,requests:{}};return y.object.onmessage=function(T){var g=T.data;y.pending-=g.count,y.requests[g.id][g.action](g.value),delete y.requests[g.id]},y}function x(f){for(var y="self.ready = WebAssembly.instantiate(new Uint8Array(["+new Uint8Array(i)+"]), {}).then(function(result) { result.instance.exports.__wasm_call_ctors(); return result.instance; });self.onmessage = "+m.name+";"+l.toString()+m.toString(),T=new Blob([y],{type:"text/javascript"}),g=URL.createObjectURL(T),_=d.length;_<f;++_)d[_]=p(g);for(var _=f;_<d.length;++_)d[_].object.postMessage({});d.length=f,URL.revokeObjectURL(g)}function v(f,y,T,g,_){for(var S=d[0],A=1;A<d.length;++A)d[A].pending<S.pending&&(S=d[A]);return new Promise(function(b,C){var w=new Uint8Array(T),I=++u;S.pending+=f,S.requests[I]={resolve:b,reject:C},S.object.postMessage({id:I,count:f,size:y,source:w,mode:g,filter:_},[w.buffer])})}function m(f){var y=f.data;self.ready.then(function(T){if(!y.id)return self.close();try{var g=new Uint8Array(y.count*y.size);l(T,T.exports[y.mode],g,y.count,y.size,y.source,T.exports[y.filter]),self.postMessage({id:y.id,count:y.count,action:"resolve",value:g},[g.buffer])}catch(_){self.postMessage({id:y.id,count:y.count,action:"reject",value:_})}})}return{ready:a,supported:!0,useWorkers:function(f){x(f)},decodeVertexBuffer:function(f,y,T,g,_){l(r,r.exports.meshopt_decodeVertexBuffer,f,y,T,g,r.exports[c[_]])},decodeIndexBuffer:function(f,y,T,g){l(r,r.exports.meshopt_decodeIndexBuffer,f,y,T,g)},decodeIndexSequence:function(f,y,T,g){l(r,r.exports.meshopt_decodeIndexSequence,f,y,T,g)},decodeGltfBuffer:function(f,y,T,g,_,S){l(r,r.exports[h[_]],f,y,T,g,r.exports[c[S]])},decodeGltfBufferAsync:function(f,y,T,g,_){return d.length>0?v(f,y,T,h[g],c[_]):a.then(function(){var S=new Uint8Array(f*y);return l(r,r.exports[h[g]],S,f,y,T,r.exports[c[_]]),S})}}})();var Pc=class extends ks{constructor(){super(),this.name="RoomEnvironment",this.position.y=-3.5;let e=new gi;e.deleteAttribute("uv");let t=new An({side:xn}),n=new An,i=new bi(16777215,900,28,2);i.position.set(.418,16.199,.3),this.add(i);let r=new ot(e,t);r.position.set(-.757,13.219,.717),r.scale.set(31.713,28.305,28.591),this.add(r);let a=new En(e,n,6),o=new Xt;o.position.set(-10.906,2.009,1.846),o.rotation.set(0,-.195,0),o.scale.set(2.328,7.905,4.651),o.updateMatrix(),a.setMatrixAt(0,o.matrix),o.position.set(-5.607,-.754,-.758),o.rotation.set(0,.994,0),o.scale.set(1.97,1.534,3.955),o.updateMatrix(),a.setMatrixAt(1,o.matrix),o.position.set(6.167,.857,7.803),o.rotation.set(0,.561,0),o.scale.set(3.927,6.285,3.687),o.updateMatrix(),a.setMatrixAt(2,o.matrix),o.position.set(-2.017,.018,6.124),o.rotation.set(0,.333,0),o.scale.set(2.002,4.566,2.064),o.updateMatrix(),a.setMatrixAt(3,o.matrix),o.position.set(2.291,-.756,-2.621),o.rotation.set(0,-.286,0),o.scale.set(1.546,1.552,1.496),o.updateMatrix(),a.setMatrixAt(4,o.matrix),o.position.set(-2.193,-.369,-5.547),o.rotation.set(0,.516,0),o.scale.set(3.875,3.487,2.986),o.updateMatrix(),a.setMatrixAt(5,o.matrix),this.add(a);let l=new ot(e,Qr(50));l.position.set(-16.116,14.37,8.208),l.scale.set(.1,2.428,2.739),this.add(l);let c=new ot(e,Qr(50));c.position.set(-16.109,18.021,-8.207),c.scale.set(.1,2.425,2.751),this.add(c);let h=new ot(e,Qr(17));h.position.set(14.904,12.198,-1.832),h.scale.set(.15,4.265,6.331),this.add(h);let d=new ot(e,Qr(43));d.position.set(-.462,8.89,14.52),d.scale.set(4.38,5.441,.088),this.add(d);let u=new ot(e,Qr(20));u.position.set(3.235,11.486,-12.541),u.scale.set(2.5,2,.1),this.add(u);let p=new ot(e,Qr(100));p.position.set(0,20,0),p.scale.set(1,.1,1),this.add(p)}dispose(){let e=new Set;this.traverse(t=>{t.isMesh&&(e.add(t.geometry),e.add(t.material))});for(let t of e)t.dispose()}};function Qr(s){return new Fa({color:0,emissive:16777215,emissiveIntensity:s})}var ts={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

		varying vec2 vUv;

		void main() {

			vUv = uv;
			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		uniform float opacity;

		uniform sampler2D tDiffuse;

		varying vec2 vUv;

		void main() {

			vec4 texel = texture2D( tDiffuse, vUv );
			gl_FragColor = opacity * texel;


		}`};var $n=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},vy=new Fi(-1,1,1,-1,0,1),qu=class extends Mt{constructor(){super(),this.setAttribute("position",new xt([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new xt([0,2,0,0,2,0],2))}},_y=new qu,zi=class{constructor(e){this._mesh=new ot(_y,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,vy)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var $r=class extends $n{constructor(e,t="tDiffuse"){super(),this.textureID=t,this.uniforms=null,this.material=null,e instanceof wt?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=Mi.clone(e.uniforms),this.material=new wt({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new zi(this.material)}render(e,t,n){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=n.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var uo=class extends $n{constructor(e,t){super(),this.scene=e,this.camera=t,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,t,n){let i=e.getContext(),r=e.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let a,o;this.inverse?(a=0,o=1):(a=1,o=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(i.REPLACE,i.REPLACE,i.REPLACE),r.buffers.stencil.setFunc(i.ALWAYS,a,4294967295),r.buffers.stencil.setClear(o),r.buffers.stencil.setLocked(!0),e.setRenderTarget(n),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(t),this.clear&&e.clear(),e.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(i.EQUAL,1,4294967295),r.buffers.stencil.setOp(i.KEEP,i.KEEP,i.KEEP),r.buffers.stencil.setLocked(!0)}},Ic=class extends $n{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var Dc=class{constructor(e,t){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),t===void 0){let n=e.getSize(new ze);this._width=n.width,this._height=n.height,t=new an(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:gn}),t.texture.name="EffectComposer.rt1"}else this._width=t.width,this._height=t.height;this.renderTarget1=t,this.renderTarget2=t.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new $r(ts),this.copyPass.material.blending=si,this.timer=new Ga}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,t){this.passes.splice(t,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let t=this.passes.indexOf(e);t!==-1&&this.passes.splice(t,1)}isLastEnabledPass(e){for(let t=e+1;t<this.passes.length;t++)if(this.passes[t].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let t=this.renderer.getRenderTarget(),n=!1;for(let i=0,r=this.passes.length;i<r;i++){let a=this.passes[i];if(a.enabled!==!1){if(a.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(i),a.render(this.renderer,this.writeBuffer,this.readBuffer,e,n),a.needsSwap){if(n){let o=this.renderer.getContext(),l=this.renderer.state.buffers.stencil;l.setFunc(o.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),l.setFunc(o.EQUAL,1,4294967295)}this.swapBuffers()}uo!==void 0&&(a instanceof uo?n=!0:a instanceof Ic&&(n=!1))}}this.renderer.setRenderTarget(t)}reset(e){if(e===void 0){let t=this.renderer.getSize(new ze);this._pixelRatio=this.renderer.getPixelRatio(),this._width=t.width,this._height=t.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,t){this._width=e,this._height=t;let n=this._width*this._pixelRatio,i=this._height*this._pixelRatio;this.renderTarget1.setSize(n,i),this.renderTarget2.setSize(n,i);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(n,i)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var Dp={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Ce(0)},defaultOpacity:{value:0}},vertexShader:`

		varying vec2 vUv;

		void main() {

			vUv = uv;

			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		uniform sampler2D tDiffuse;
		uniform vec3 defaultColor;
		uniform float defaultOpacity;
		uniform float luminosityThreshold;
		uniform float smoothWidth;

		varying vec2 vUv;

		void main() {

			vec4 texel = texture2D( tDiffuse, vUv );

			float v = luminance( texel.xyz );

			vec4 outputColor = vec4( defaultColor.rgb, defaultOpacity );

			float alpha = smoothstep( luminosityThreshold, luminosityThreshold + smoothWidth, v );

			gl_FragColor = mix( outputColor, texel, alpha );

		}`};var ea=class s extends $n{constructor(e,t=1,n,i){super(),this.strength=t,this.radius=n,this.threshold=i,this.resolution=e!==void 0?new ze(e.x,e.y):new ze(256,256),this.clearColor=new Ce(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),a=Math.round(this.resolution.y/2);this.renderTargetBright=new an(r,a,{type:gn,depthBuffer:!1}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let h=0;h<this.nMips;h++){let d=new an(r,a,{type:gn,depthBuffer:!1});d.texture.name="UnrealBloomPass.h"+h,d.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(d);let u=new an(r,a,{type:gn,depthBuffer:!1});u.texture.name="UnrealBloomPass.v"+h,u.texture.generateMipmaps=!1,this.renderTargetsVertical.push(u),r=Math.round(r/2),a=Math.round(a/2)}let o=Dp;this.highPassUniforms=Mi.clone(o.uniforms),this.highPassUniforms.luminosityThreshold.value=i,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new wt({uniforms:this.highPassUniforms,vertexShader:o.vertexShader,fragmentShader:o.fragmentShader}),this.separableBlurMaterials=[];let l=[6,10,14,18,22];r=Math.round(this.resolution.x/2),a=Math.round(this.resolution.y/2);for(let h=0;h<this.nMips;h++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(l[h])),this.separableBlurMaterials[h].uniforms.invSize.value=new ze(1/r,1/a),r=Math.round(r/2),a=Math.round(a/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=t,this.compositeMaterial.uniforms.bloomRadius.value=.1;let c=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=c,this.bloomTintColors=[new k(1,1,1),new k(1,1,1),new k(1,1,1),new k(1,1,1),new k(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=Mi.clone(ts.uniforms),this.blendMaterial=new wt({uniforms:this.copyUniforms,vertexShader:ts.vertexShader,fragmentShader:ts.fragmentShader,premultipliedAlpha:!0,blending:tn,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new Ce,this._oldClearAlpha=1,this._basic=new qt,this._fsQuad=new zi(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,t){let n=Math.round(e/2),i=Math.round(t/2);this.renderTargetBright.setSize(n,i);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(n,i),this.renderTargetsVertical[r].setSize(n,i),this.separableBlurMaterials[r].uniforms.invSize.value=new ze(1/n,1/i),n=Math.round(n/2),i=Math.round(i/2)}render(e,t,n,i,r){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let a=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),r&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=n.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=n.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let o=this.renderTargetBright;for(let l=0;l<this.nMips;l++)this._fsQuad.material=this.separableBlurMaterials[l],this.separableBlurMaterials[l].uniforms.colorTexture.value=o.texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[l]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[l].uniforms.colorTexture.value=this.renderTargetsHorizontal[l].texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[l]),e.clear(),this._fsQuad.render(e),o=this.renderTargetsVertical[l];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(n),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=a}_getSeparableBlurMaterial(e){let t=[],n=e/3;for(let a=0;a<e;a++)t.push(.39894*Math.exp(-.5*a*a/(n*n))/n);let i=[],r=[];for(let a=1;a<e;a+=2){let o=t[a],l=a+1<e?t[a+1]:0,c=o+l;i.push((a*o+(a+1)*l)/c),r.push(c)}return new wt({defines:{KERNEL_PAIRS:i.length},uniforms:{colorTexture:{value:null},invSize:{value:new ze(.5,.5)},direction:{value:new ze(.5,.5)},centerWeight:{value:t[0]},gaussianOffsets:{value:i},gaussianWeights:{value:r}},vertexShader:`

				varying vec2 vUv;

				void main() {

					vUv = uv;
					gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

				}`,fragmentShader:`

				#include <common>

				varying vec2 vUv;

				uniform sampler2D colorTexture;
				uniform vec2 invSize;
				uniform vec2 direction;
				uniform float centerWeight;
				uniform float gaussianOffsets[KERNEL_PAIRS];
				uniform float gaussianWeights[KERNEL_PAIRS];

				void main() {

					vec3 diffuseSum = texture2D( colorTexture, vUv ).rgb * centerWeight;

					for ( int i = 0; i < KERNEL_PAIRS; i ++ ) {

						vec2 uvOffset = direction * invSize * gaussianOffsets[ i ];
						vec3 sample1 = texture2D( colorTexture, vUv + uvOffset ).rgb;
						vec3 sample2 = texture2D( colorTexture, vUv - uvOffset ).rgb;
						diffuseSum += ( sample1 + sample2 ) * gaussianWeights[ i ];

					}

					gl_FragColor = vec4( diffuseSum, 1.0 );

				}`})}_getCompositeMaterial(e){return new wt({defines:{NUM_MIPS:e},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`

				varying vec2 vUv;

				void main() {

					vUv = uv;
					gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

				}`,fragmentShader:`

				varying vec2 vUv;

				uniform sampler2D blurTexture1;
				uniform sampler2D blurTexture2;
				uniform sampler2D blurTexture3;
				uniform sampler2D blurTexture4;
				uniform sampler2D blurTexture5;
				uniform float bloomStrength;
				uniform float bloomRadius;
				uniform float bloomFactors[NUM_MIPS];
				uniform vec3 bloomTintColors[NUM_MIPS];

				float lerpBloomFactor( const in float factor ) {

					float mirrorFactor = 1.2 - factor;
					return mix( factor, mirrorFactor, bloomRadius );

				}

				void main() {

					// 3.0 for backwards compatibility with previous alpha-based intensity
					vec3 bloom = 3.0 * bloomStrength * (
						lerpBloomFactor( bloomFactors[ 0 ] ) * bloomTintColors[ 0 ] * texture2D( blurTexture1, vUv ).rgb +
						lerpBloomFactor( bloomFactors[ 1 ] ) * bloomTintColors[ 1 ] * texture2D( blurTexture2, vUv ).rgb +
						lerpBloomFactor( bloomFactors[ 2 ] ) * bloomTintColors[ 2 ] * texture2D( blurTexture3, vUv ).rgb +
						lerpBloomFactor( bloomFactors[ 3 ] ) * bloomTintColors[ 3 ] * texture2D( blurTexture4, vUv ).rgb +
						lerpBloomFactor( bloomFactors[ 4 ] ) * bloomTintColors[ 4 ] * texture2D( blurTexture5, vUv ).rgb
					);

					float bloomAlpha = max( bloom.r, max( bloom.g, bloom.b ) );
					gl_FragColor = vec4( bloom, bloomAlpha );

				}`})}};ea.BlurDirectionX=new ze(1,0);ea.BlurDirectionY=new ze(0,1);var fo={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
		precision highp float;

		uniform mat4 modelViewMatrix;
		uniform mat4 projectionMatrix;

		attribute vec3 position;
		attribute vec2 uv;

		varying vec2 vUv;

		void main() {

			vUv = uv;
			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		precision highp float;

		uniform sampler2D tDiffuse;

		#include <tonemapping_pars_fragment>
		#include <colorspace_pars_fragment>

		varying vec2 vUv;

		void main() {

			gl_FragColor = texture2D( tDiffuse, vUv );

			// tone mapping

			#ifdef LINEAR_TONE_MAPPING

				gl_FragColor.rgb = LinearToneMapping( gl_FragColor.rgb );

			#elif defined( REINHARD_TONE_MAPPING )

				gl_FragColor.rgb = ReinhardToneMapping( gl_FragColor.rgb );

			#elif defined( CINEON_TONE_MAPPING )

				gl_FragColor.rgb = CineonToneMapping( gl_FragColor.rgb );

			#elif defined( ACES_FILMIC_TONE_MAPPING )

				gl_FragColor.rgb = ACESFilmicToneMapping( gl_FragColor.rgb );

			#elif defined( AGX_TONE_MAPPING )

				gl_FragColor.rgb = AgXToneMapping( gl_FragColor.rgb );

			#elif defined( NEUTRAL_TONE_MAPPING )

				gl_FragColor.rgb = NeutralToneMapping( gl_FragColor.rgb );

			#elif defined( CUSTOM_TONE_MAPPING )

				gl_FragColor.rgb = CustomToneMapping( gl_FragColor.rgb );

			#endif

			// color space

			#ifdef SRGB_TRANSFER

				gl_FragColor = sRGBTransferOETF( gl_FragColor );

			#endif

		}`};var Lc=class extends $n{constructor(){super(),this.isOutputPass=!0,this.uniforms=Mi.clone(fo.uniforms),this.material=new Nr({name:fo.name,uniforms:this.uniforms,vertexShader:fo.vertexShader,fragmentShader:fo.fragmentShader}),this._fsQuad=new zi(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,t,n){this.uniforms.tDiffuse.value=n.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},Rt.getTransfer(this._outputColorSpace)===Wt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Xa?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===ja?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===Ka?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===js?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===Ja?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Za?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Ya&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Si=512,yy=7.5;function Xu(s,e=yy){let t=new Ia,n=s.map(([o,l])=>new k(o,0,l)),i=[],r=[],a=n.length;for(let o=0;o<a;o++){let l=n[o],c=n[(o+a-1)%a],h=n[(o+1)%a],d=Math.min(e,l.distanceTo(c)/2,l.distanceTo(h)/2);i.push(l.clone().addScaledVector(c.clone().sub(l).normalize(),d)),r.push(l.clone().addScaledVector(h.clone().sub(l).normalize(),d))}for(let o=0;o<a;o++)t.add(new xi(i[o],n[o],r[o])),t.add(new Lr(r[o],i[(o+1)%a]));return t}function My(s){let e=Xu(s),t=e.getSpacedPoints(Si),n=[];for(let i=0;i<Si;i++){let r=t[(i+Si-1)%Si],a=t[(i+1)%Si];n.push(new k(a.x-r.x,0,a.z-r.z).normalize())}return{points:t,tangents:n,length:e.getLength()}}function Lp(s){let e=[],t=(n,i,r,a)=>{let o=Math.cos(n.angle),l=Math.sin(n.angle);e.push({x:n.x+i*o+r*l,z:n.z-i*l+r*o,r:a,asset:n.asset})};for(let n of s)if(!(n.z<-100))if(n.asset==="data-tram")for(let i of[-2.4,0,2.4])t(n,i,0,1.5);else n.asset==="server-rack"?t(n,0,0,.8):n.asset==="street-lamp"?t(n,0,0,.5):n.asset==="planter"&&(t(n,-.9,0,.85),t(n,.9,0,.85));return e}function Fp({routes:s,obstacles:e=[],robotRadius:t=1.3,lanes:n=[2.2,0,-2.2,3.9,-3.9],defaultLane:i=2.2,speeds:r=[],blocked:a=()=>!1}){let o=s.map(My),l=s.map((w,I)=>({route:I,s:o[I].length*(I*.173%1),dir:1,lane:i,laneTarget:i,speed:0,target:0,laneVel:0,cruise:r[I]??5.1+I*.43,state:"cruise",heading:0,wait:0,cooldown:0,x:0,z:0,fx:0,fz:1,rx:1,rz:0,vx:0,vz:0,turns:0})),c=new k,h=new k,d=new k;function u(w,I,M,N,D){let U=(I/w.length%1+1)%1*Si,H=Math.floor(U)%Si,L=U-Math.floor(U);N.lerpVectors(w.points[H],w.points[(H+1)%Si],L),D.lerpVectors(w.tangents[H],w.tangents[(H+1)%Si],L).normalize().multiplyScalar(M)}let p=w=>Math.atan2(Math.sin(w),Math.cos(w));function x(w){u(o[w.route],w.s,w.dir,c,h),w.fx=h.x,w.fz=h.z,w.rx=-h.z,w.rz=h.x,w.x=c.x+w.rx*w.lane,w.z=c.z+w.rz*w.lane}function v(w,I){for(let M of e){let N=M.r+t;if((w-M.x)**2+(I-M.z)**2<N*N)return!0}return!1}function m(w,I){let M=o[w.route];for(let N of[-2.5,0,1,3,6,9,12,15,18])if(u(M,w.s+w.dir*N,w.dir,c,h),d.set(c.x-h.z*I,0,c.z+h.x*I),v(d.x,d.z)||N>=0&&a(w,d.x,d.z))return!1;return!0}function f(w){w.dir=-w.dir,w.lane=-w.lane,w.laneTarget=i,w.laneVel=0,w.state="turn",w.cooldown=4,w.wait=0,w.target=0,w.turns++,w.blocked=0}function y(w,I){if(w.held){w.target=0,w.speed=0,w.laneVel=0;return}if(w.cooldown>0&&(w.cooldown-=I),w.state==="turn"){if(w.target=0,w.speed<.05){let U=Math.atan2(w.fx,w.fz),H=p(U-w.heading);w.heading+=H*Math.min(1,I*3.2),Math.abs(H)<.1&&(w.heading=U,w.state="cruise")}return}let M=null,N=[...n].sort((U,H)=>Math.abs(U-i)-Math.abs(H-i)||H-U);for(let U of N)if(m(w,U)){M=U;break}if(M===null){w.blocked=(w.blocked||0)+I,w.target=0,w.blocked>2.5&&w.cooldown<=0&&(w.blocked=0,f(w));return}w.blocked=0,w.laneTarget=M,w.target=w.cruise,Math.abs(w.laneTarget-w.lane)>.5&&!m(w,w.lane)&&(w.target=Math.min(w.target,2.4));let D=!1;for(let U of l){if(U===w)continue;let H=A(w,w.laneTarget,U);if(!H)continue;D=!0;let L=U.fx*w.fx+U.fz*w.fz,W=U.speed<.3;if(W||L<-.2){let z=g.get(U)[T.length-1],E=W?0:Math.sign((z.x-w.x)*w.rx+(z.z-w.z)*w.rz)||-1,J=null;for(let q of N)if(!(E&&Math.sign(q-w.lane)===E)&&m(w,q)&&!A(w,q,U)){J=q;break}J!==null?w.laneTarget=J:w.target=0}else L>.5?H.da>=H.db&&(w.target=Math.min(w.target,H.da<3.8?0:U.speed*.9)):(H.da>H.db||H.da===H.db&&w.route>U.route)&&(w.target=Math.min(w.target,H.da<=4?0:2))}D&&w.target===0?w.wait+=I:w.wait=0,w.wait>2.5&&w.cooldown<=0&&f(w)}let T=[0,2,4,6,8,10,12],g=new Map,_=new k;function S(w,I,M,N){u(o[w.route],w.s+w.dir*I,w.dir,c,h),N.set(c.x-h.z*M,0,c.z+h.x*M)}function A(w,I,M){let N=g.get(M),D=(t*2+.6)**2,U=M.speed<.3;for(let H=0;H<T.length;H++){S(w,T[H],I,_);let L=T[H]/Math.max(w.speed,1.5);for(let W=0;W<T.length&&!(U&&W>0);W++){let z=T[W]/Math.max(M.speed,1.5);if(!(T[H]>4&&Math.abs(L-z)>1.6)&&_.distanceToSquared(N[W])<D)return{da:T[H],db:T[W]}}}return null}function b(w){if(w>0){for(let I of l){x(I),g.has(I)||g.set(I,T.map(()=>new k));let M=g.get(I);T.forEach((N,D)=>S(I,I.state==="cruise"?N:0,I.lane,M[D]))}for(let I of l)y(I,w);for(let I=0;I<l.length;I++)for(let M=I+1;M<l.length;M++){let N=l[I],D=l[M],U=D.x-N.x,H=D.z-N.z,L=Math.hypot(U,H),W=t*2+.2;if(L>=W)continue;let z=N.state==="turn"||N.speed<.3,E=D.state==="turn"||D.speed<.3,J=z===E?.5:z?0:1,q=Math.min(W-Math.max(L,.01),2.5*w),te=U*N.rx+H*N.rz,Le=-(U*D.rx+H*D.rz);N.lane=It.clamp(N.lane-Math.sign(te||1)*q*J,-4.2,4.2),D.lane=It.clamp(D.lane-Math.sign(Le||1)*q*(1-J),-4.2,4.2),N.target=Math.min(N.target,.5),D.target=Math.min(D.target,.5)}for(let I of l){let M=I.x,N=I.z,D=I.target>I.speed?6:9;if(I.held){I.vx=I.vz=0;continue}I.speed+=It.clamp(I.target-I.speed,-D*w,D*w),I.speed<0&&(I.speed=0),I.s+=I.dir*I.speed*w;let U=I.laneTarget-I.lane,H=I.state==="turn"?0:Math.min(3,I.speed*.6),L=Math.sign(U)*Math.min(H,Math.sqrt(8*Math.abs(U)),Math.abs(U)/w);if(I.laneVel+=It.clamp(L-I.laneVel,-4*w,4*w),I.lane+=I.laneVel*w,x(I),I.vx=(I.x-M)/w,I.vz=(I.z-N)/w,I.state==="cruise"){let W=I.speed>.25?Math.atan2(I.vx,I.vz):Math.atan2(I.fx,I.fz);I.heading+=p(W-I.heading)*Math.min(1,w*(I.speed>.25?8:3.2))}}}}let C=[];for(let w of l){for(let I=0;I<600&&(x(w),!(C.every(N=>Math.hypot(w.x-N.x,w.z-N.z)>t*2+6)&&m(w,w.lane)));I++)w.s+=1;w.heading=Math.atan2(w.fx,w.fz),C.push(w)}return{agents:l,step:b,hitsObstacle:v,seek(w,I){w.s=I,x(w),w.heading=Math.atan2(w.fx,w.fz)},adopt(w,I){let M=o[w.route],N=0,D=1/0;for(let U=0;U<Si;U++){let H=M.points[U],L=Math.hypot(H.x-I.x,H.z-I.z);L<D&&(D=L,N=U)}w.s=N/Si*M.length,u(M,w.s,w.dir,c,h),w.lane=(I.x-c.x)*-h.z+(I.z-c.z)*h.x,w.laneTarget=i,w.speed=0,w.laneVel=0,w.state="cruise",w.heading=I.heading,x(w)},stats:()=>({states:l.map(w=>w.state),turns:l.reduce((w,I)=>w+I.turns,0),lanes:l.map(w=>Math.round(w.lane*10)/10)})}}var wy=(s,e,t)=>Math.max(e,Math.min(t,s)),Uc=(s,e)=>Math.atan2(Math.sin(e-s),Math.cos(e-s)),po=s=>({x:s.x,y:s.y||0,z:s.z,heading:s.heading||0});function ns(s,e=1){let t=s.min.map(u=>u*e),n=s.max.map(u=>u*e),i=(n[0]-t[0])/2,r=(n[2]-t[2])/2,a=i>r,o=Math.max(i,r),l=Math.min(i,r),c=Math.min(7,Math.max(1,Math.ceil(o/Math.max(.3,l)))),h=o/c,d=[];for(let u=0;u<c;u++){let p=-o+h+u*h*2;d.push({x:(t[0]+n[0])/2+(a?p:0),z:(t[2]+n[2])/2+(a?0:p),r:Math.hypot(l,h)})}return{circles:d,minY:t[1],maxY:n[1],reach:Math.max(...d.map(u=>Math.hypot(u.x,u.z)+u.r))}}function ta(s,e){let t=Math.cos(e.heading),n=Math.sin(e.heading);return{x:e.x+s.x*t+s.z*n,z:e.z-s.x*n+s.z*t,r:s.r}}function Nc(s,e,t,n,i){let r=e-s;if(Math.abs(r)<1e-10)return s>=t&&s<=n;let a=(t-s)/r,o=(n-s)/r;return i[0]=Math.max(i[0],Math.min(a,o)),i[1]=Math.min(i[1],Math.max(a,o)),i[0]<=i[1]}function Sy(s,e,t,n,i,r){let a=[0,1];return Nc(e.y-i.y,t.y-r.y,n.minY-s.maxY+.06,n.maxY-s.minY-.06,a)?a:null}function Fc(s,e,t,n,i,r){let a=s.reach+n.reach+.06;if(Math.min(e.x,t.x)-Math.max(i.x,r.x)>a||Math.min(i.x,r.x)-Math.max(e.x,t.x)>a||Math.min(e.z,t.z)-Math.max(i.z,r.z)>a||Math.min(i.z,r.z)-Math.max(e.z,t.z)>a)return!1;let o=Sy(s,e,t,n,i,r);if(!o)return!1;for(let l of s.circles)for(let c of n.circles){let h=.06+Math.abs(Uc(e.heading,t.heading))*Math.hypot(l.x,l.z)+Math.abs(Uc(i.heading,r.heading))*Math.hypot(c.x,c.z),d=ta(l,e),u=ta(l,t),p=ta(c,i),x=ta(c,r),v=d.x-p.x,m=d.z-p.z,f=u.x-x.x-v,y=u.z-x.z-m,T=wy(-(v*f+m*y)/Math.max(1e-12,f*f+y*y),...o),g=l.r+c.r+h;if((v+f*T)**2+(m+y*T)**2<g*g)return!0}return!1}function ju(s,e,t,n){let i=s.reach+.06;if(n.world&&(Math.min(e.x,t.x)-i>n.world[2]||Math.max(e.x,t.x)+i<n.world[0]||Math.min(e.z,t.z)-i>n.world[3]||Math.max(e.z,t.z)+i<n.world[1])||Math.min(e.y,t.y)+s.minY>=n.max[1]||Math.max(e.y,t.y)+s.maxY<=n.min[1])return!1;let r=Math.cos(n.heading||0),a=Math.sin(n.heading||0),o=l=>({x:(l.x-n.x)*r-(l.z-n.z)*a,z:(l.x-n.x)*a+(l.z-n.z)*r});for(let l of s.circles){let c=o(ta(l,e)),h=o(ta(l,t)),d=l.r+.06+Math.abs(Uc(e.heading,t.heading))*Math.hypot(l.x,l.z),u=[0,1];if(Nc(e.y,t.y,n.min[1]-s.maxY+.06,n.max[1]-s.minY-.06,u)&&Nc(c.x,h.x,n.min[0]-d,n.max[0]+d,u)&&Nc(c.z,h.z,n.min[2]-d,n.max[2]+d,u))return!0}return!1}function Oc(){let s=new Map,e=new Map,t=new Map,n=0,i=0,r=0,a=0,o=(x,v)=>x.id===v.id||x.ignore===v.id||v.ignore===x.id||x.enabled===!1||v.enabled===!1;function l(x,v,m,f=!0){if(![m.x,m.y,m.z,m.heading].every(Number.isFinite))return"invalid-pose";for(let y of e.values())if(y.enabled!==!1&&!(y.owner&&(y.owner===x.ignore||y.owner===x.id))&&ju(x,v,m,y))return y.id;if(f){for(let y of s.values())if(!o(x,y)&&Fc(x,v,m,y,y,y))return y.id}return null}let c=(x,v,m,f=!0)=>!l(x,v,m,f);function h(x,v,m,f=0){if(s.has(x))return s.get(x);let y={id:x,...v,...po(m),priority:f,enabled:!0,blocked:0};return c(y,y,y)?(s.set(x,y),y):null}function d(x,v,m=12){let f={...x,enabled:!0};for(let y=0;y<=m;y+=.5)for(let T=0;T<(y?24:1);T++){let g={...po(v),x:v.x+Math.cos(T*Math.PI/12)*y,z:v.z+Math.sin(T*Math.PI/12)*y};if(c(f,g,g))return g}return null}function u(x,v,m=12){let f=d(x,v,m);return f?(Object.assign(x,f),!0):!1}function p(x){let v=[...s.values()].filter(m=>m.enabled!==!1).map(m=>{let f=t.get(m.id),y=f?.next||po(m);return{body:m,from:po(m),next:y,request:f,accepted:c(m,m,y,!1)}});for(let m=0;m<=v.length;m++){let f=!1;for(let y of v)y.accepted&&y.body.follow&&v.some(T=>T.body.id===y.body.follow&&!T.accepted)&&(y.accepted=!1,f=!0);for(let y=0;y<v.length;y++)for(let T=y+1;T<v.length;T++){let g=v[y],_=v[T];if(o(g.body,_.body)||!g.accepted&&!_.accepted)continue;let S=g.accepted?g.next:g.from,A=_.accepted?_.next:_.from;if(!Fc(g.body,g.from,S,_.body,_.from,A))continue;let b=z=>z.request&&z.accepted&&(Math.hypot(z.next.x-z.from.x,z.next.y-z.from.y,z.next.z-z.from.z)>1e-8||Math.abs(Uc(z.from.heading,z.next.heading))>1e-8),C=b(g),w=b(_);if(!C&&!w)continue;let I=g.next.x-g.from.x,M=g.next.z-g.from.z,N=_.next.x-_.from.x,D=_.next.z-_.from.z,U=I*N+M*D>.8*Math.hypot(I,M)*Math.hypot(N,D),H=C&&!Fc(g.body,g.from,g.next,_.body,_.from,_.from),L=w&&!Fc(g.body,g.from,g.from,_.body,_.from,_.next),W=C?w?H&&!L?_:L&&!H?g:g.body.priority!==_.body.priority?g.body.priority<_.body.priority?g:_:U?(_.from.x-g.from.x)*I+(_.from.z-g.from.z)*M>0?g:_:g.body.id>_.body.id?g:_:g:_;W.accepted=!1,f=!0}if(!f)break}for(let m of v)m.accepted?(Object.assign(m.body,m.next),m.body.blocked=0,m.body.obstruction=null):m.request&&(m.body.blocked+=x,m.body.obstruction=l(m.body,m.from,m.next),n++);for(let m of v)m.request?.commit?.(m.accepted,m.body);t.clear(),i++,r+=x}return{register:h,relocate:u,findFree:d,clear:c,obstruction:l,solve:p,begin(){t.clear()},propose(x,v,m){x?.enabled!==!1&&x&&t.set(x.id,{next:po(v),commit:m})},remove(x){s.delete(x),t.delete(x)},solid(x,v){let m=e.get(x),f={id:x,x:0,z:0,heading:0,...v};if(m&&m.x===f.x&&m.z===f.z&&m.heading===f.heading&&m.enabled===f.enabled&&m.min.every((S,A)=>S===f.min[A])&&m.max.every((S,A)=>S===f.max[A]))return;let y=Math.cos(f.heading),T=Math.sin(f.heading),g=[],_=[];for(let S of[f.min[0],f.max[0]])for(let A of[f.min[2],f.max[2]])g.push(f.x+S*y+A*T),_.push(f.z-S*T+A*y);f.world=[Math.min(...g),Math.min(..._),Math.max(...g),Math.max(..._)],e.set(x,f),a++},removeSolid(x){e.delete(x)&&a++},removeOwner(x){for(let[v,m]of e)m.owner===x&&(e.delete(v),a++)},revision:()=>a,solidClear(x){return[...s.values()].every(v=>v.enabled===!1||x.owner&&v.ignore===x.owner||!ju(v,v,v,{x:0,z:0,heading:0,...x}))},ceiling(x,v,m){let f=0;for(let y of e.values())y.enabled!==!1&&ju(x,{x:v,y:y.min[1],z:m,heading:0},{x:v,y:y.min[1],z:m,heading:0},y)&&(f=Math.max(f,y.max[1]-x.minY+1));return f},body:x=>s.get(x),neighbors(x,v=12){return[...s.values()].filter(m=>!o(x,m)&&Math.abs(m.y-x.y)<4&&Math.hypot(m.x-x.x,m.z-x.z)<v)},stats:()=>({bodies:s.size,solids:e.size,stops:n,steps:i,time:r,poses:[...s.values()].filter(x=>x.enabled!==!1).map(x=>({id:x.id,x:x.x,y:x.y,z:x.z,heading:x.heading,blocked:x.blocked,obstruction:x.obstruction}))}),dispose(){s.clear(),e.clear(),t.clear()}}}function Np(s,e=1/60){let t=0;return n=>{if(!(n>0))return t=0,0;t=Math.min(t+n,e*6);let i=0;for(;t>=e-1e-9;)s(e),t-=e,i++;return i}}var Ht={xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80,quayZ:74},pt={ground:0,road:-.14,pavement:0,quay:.08,room:.16,gallery:4.16},Ln=["agent","memory","missions"].map((s,e)=>{let t=[0,-43,43][e],n=73;return{id:s,x:t,z:n,width:12,depth:12,front:-1,doorZ:n-6,liftX:t+4,liftZ:n+3}}),oi=[{id:"infra",x:-67,z:-53,platformX:-74,platformZ:-53,angle:Math.PI/2},{id:"memory",x:-67,z:-10,platformX:-74,platformZ:-10,angle:Math.PI/2},{id:"graph",x:-43,z:59,platformX:-43,platformZ:64.5,angle:0},{id:"operations",x:0,z:59,platformX:0,platformZ:64.5,angle:0},{id:"missions",x:43,z:59,platformX:43,platformZ:64.5,angle:0},{id:"integrations",x:67,z:-10,platformX:74,platformZ:-10,angle:-Math.PI/2},{id:"agent",x:0,z:-77,platformX:0,platformZ:-84,angle:Math.PI}],Ku=[[-67,-77],[-67,59],[67,59],[67,-77]],Up=(s,e)=>{let t=s*Math.PI/180;return{x:Math.sin(t)*e,z:-12-Math.cos(t)*e,angle:Math.PI-t}},jt={x:0,z:-12,floor:72.2,inner:4.2,outer:8,bridge:1,lift:{x:9.8,z:-12,base:.88,cab:.03,travel:71.32},telescopes:[45,135,225,315].map(s=>({id:"deck-"+s,...Up(s,7.25)})),benches:[0,180,270].map(s=>Up(s,6.6))};function Op(s,e,t){let n=Math.hypot(s-jt.x,e-jt.z);return n>=jt.inner&&n<=jt.outer?!0:t&&s>=jt.outer-.5&&s<=jt.lift.x+1&&Math.abs(e-jt.lift.z)<jt.bridge}var ki={x:78,z:30},zp=[{id:"south",x:-7.8,z:63.75,angle:Math.PI},{id:"harbour",x:23.3,z:75,angle:-Math.PI/2},{id:"west",x:-72.3,z:-22,angle:Math.PI/2},{id:"east",x:72.3,z:42,angle:-Math.PI/2},{id:"north",x:-8,z:-26.7,angle:Math.PI}],zc=[{x:30.5,z:-55,scale:1},{x:49,z:-55,scale:1.25},{x:-3,z:-57,scale:1.2}];function Ey(s,e,t,n=0){return e>=s.x-5.85+n&&e<=s.x+2.4-n&&t>=s.z+n&&t<=s.z+5.85-n}function kp(s,e,t,n=0){return Math.abs(e-s.liftX)<=1.6-n&&Math.abs(t-s.liftZ)<=1.6-n}function Bp(){let s=[],e=[2.4,1.4,5.6,4.6],t=(n,i,r,a)=>{r>n&&a>i&&s.push({x:(n+r)/2,z:(i+a)/2,sx:(r-n)/4,sz:(a-i)/4})};for(let n=-4;n<=4;n+=4)for(let i=-4;i<=4;i+=4){let[r,a,o,l]=[n-2,i-2,n+2,i+2],[c,h,d,u]=[Math.max(r,e[0]),Math.max(a,e[1]),Math.min(o,e[2]),Math.min(l,e[3])];if(c>=d||h>=u){t(r,a,o,l);continue}t(r,a,c,l),t(d,a,o,l),t(c,a,d,h),t(c,u,d,l)}return s}function Yu(s,e,t,n){return Ey(s,e,t,.25)?!0:n?e>=s.x+1.8&&e<=s.liftX+1.25&&Math.abs(t-s.liftZ)<1.25:!1}var Gt={line:3.3,run:.14,top:.14,rise:.14,droppedRun:1.2,opening:1.2,flare:.8,zebraOffset:8,cornerRadius:1.2,endDepth:.5,pavementLine:4.75},Bi=4,Ty=1,Ju=48,$s={top:-.2,bottom:-3},Gp=.15,Ay=.1,Hp=s=>s<0?0:s>1?1:s,Zu=[...Ht.zs.map(s=>({axis:"x",line:s,from:Ht.minX,to:Ht.maxX,crossings:Ht.xs})),...Ht.xs.map(s=>({axis:"z",line:s,from:Ht.minZ,to:Ht.quayZ,crossings:Ht.zs}))];var Hc=[];for(let s of Zu)for(let e of s.crossings)for(let t of[-1,1]){let n=e+t*Gt.zebraOffset;n>=s.from&&n<=s.to&&Hc.push({axis:s.axis,line:s.line,at:n,x:s.axis==="x"?n:s.line,z:s.axis==="x"?s.line:n})}function Vp(){return Hc.map(s=>({...s}))}function Ry(s){return s<=Gt.opening?Gt.droppedRun:s>=Gt.opening+Gt.flare?Gt.run:Gt.droppedRun+(Gt.run-Gt.droppedRun)*(s-Gt.opening)/Gt.flare}function Qu(s,e){let{xs:t,zs:n,halfWidth:i,minX:r,maxX:a,minZ:o,quayZ:l}=Ht,c=1/0,h=1/0;if(e>=o&&e<=l)for(let g of t)c=Math.min(c,Math.abs(s-g));if(s>=r&&s<=a)for(let g of n)h=Math.min(h,Math.abs(e-g));let d=c<=i,u=h<=i;if(!d&&!u)return null;let p=pt.road,x,v=Gt.run;if(d&&u)if(c<=Gt.line||h<=Gt.line)x=-1;else{let g=c-Gt.line,_=h-Gt.line,S=Gt.cornerRadius;x=g<S&&_<S?S-Math.hypot(S-g,S-_):Math.min(g,_)}else{let g=u?s:e,_=u?t:n,S=1/0;for(let A of _)S=Math.min(S,Math.abs(g-A-Gt.zebraOffset),Math.abs(g-A+Gt.zebraOffset));x=(u?h:c)-Gt.line,v=Ry(S)}let m=p+Gt.rise*Hp(x/v),f=x<=0?"road":x>=v?"pavement":"kerb",y=Math.min(d?e-o:1/0,d?l-e:1/0,u?s-r:1/0,u?a-s:1/0),T=Gt.endDepth+Gt.run;if(y<T){let g=p+Gt.rise*Hp((T-y)/Gt.run);g>m&&(m=g,f="kerb")}return{height:m,kind:f,d:x,run:v}}function Cy(){let s=[];for(let e of Zu){let t=[{at:e.from,kind:"end"},...e.crossings.map(n=>({at:n,kind:"crossing"})),{at:e.to,kind:"end"}];for(let n=0;n+1<t.length;n++){let i=t[n],r=t[n+1];s.push({axis:e.axis,line:e.line,start:i.kind,end:r.kind,from:i.kind==="crossing"?i.at+Ht.halfWidth:i.at,to:r.kind==="crossing"?r.at-Ht.halfWidth:r.at})}}return s}var Py={x:0,z:Math.PI/2};function mo(s,e,t,n,i){let r=(n+i)/2;return{id:s,axis:e,line:t,from:n,to:i,x:e==="x"?r:t,z:e==="x"?t:r,angle:Py[e]}}function Wp(){let s=[];for(let e of Ht.xs)for(let t of Ht.zs)s.push({id:"street-crossing",axis:null,line:null,from:null,to:null,x:e,z:t,angle:0});for(let e of Cy()){let t=e.from,n=e.to;e.start==="crossing"&&(s.push(mo("street-zebra",e.axis,e.line,t,t+Bi)),t+=Bi),e.end==="crossing"&&(s.push(mo("street-zebra",e.axis,e.line,n-Bi,n)),n-=Bi);let i=e.start==="crossing",r=Math.floor((n-t)/Bi+1e-9),a=Math.round(n-t-r*Bi);for(let o=0;o<r;o++){let l=o%3===2?"street-drain":"street-plain";s.push(i?mo(l,e.axis,e.line,t+o*Bi,t+(o+1)*Bi):mo(l,e.axis,e.line,n-(o+1)*Bi,n-o*Bi))}for(let o=0;o<a;o++){let l=i?n-a+o:t+o;s.push(mo("street-fill",e.axis,e.line,l,l+Ty))}for(let[o,l,c]of[[e.start,e.from,-1],[e.end,e.to,1]])o==="end"&&s.push({id:"street-end",axis:e.axis,line:e.line,from:l,to:l,x:e.axis==="x"?l:e.line,z:e.axis==="x"?e.line:l,angle:e.axis==="x"?c>0?0:Math.PI:c>0?-Math.PI/2:Math.PI/2})}return s}function qp(){let{xs:s,zs:e,halfWidth:t,minX:n,maxX:i,minZ:r,maxZ:a,quayZ:o}=Ht,l=(h,d,u)=>{let p=[],x=d;for(let v of h)p.push([x,v-t]),x=v+t;return p.push([x,u]),p},c=[];for(let[h,d]of l(s,n,i))for(let[u,p]of l(e,r,o))c.push({kind:"plot",x0:h,z0:u,x1:d,z1:p,top:pt.ground,bottom:$s.top});return c.push({kind:"quay",x0:n,z0:o,x1:i,z1:a,top:pt.ground,bottom:$s.top}),c}var kc=Gt.line+Gt.droppedRun+Gp;function Iy(s,e){let t=Math.min(s.x,e.x),n=Math.max(s.x,e.x),i=Math.min(s.z,e.z),r=Math.max(s.z,e.z);return Zu.some(a=>a.axis==="x"?i<=a.line+kc&&r>=a.line-kc&&n>=a.from&&t<=a.to:t<=a.line+kc&&n>=a.line-kc&&r>=a.from&&i<=a.to)}function Dy(s,e){return Hc.some(t=>t.axis==="x"?Math.abs(s-t.at)<=Gt.opening&&Math.abs(e-t.line)<=Ht.halfWidth:Math.abs(e-t.at)<=Gt.opening&&Math.abs(s-t.line)<=Ht.halfWidth)}function $u(s,e,t=!1){if(!Iy(s,e))return!0;let n=Math.max(1,Math.ceil(Math.hypot(e.x-s.x,e.z-s.z)/Ay));for(let i=0;i<=n;i++){let r=i/n,a=s.x+(e.x-s.x)*r,o=s.z+(e.z-s.z)*r,l=Qu(a,o);if(!(!l||l.kind==="pavement"&&l.d-l.run>=Gp)&&!(t&&Dy(a,o)))return!1}return!0}var Bc=null;function Xp(){if(Bc)return Bc;let s=Gt.pavementLine,e=[],t=[];for(let n of Ht.xs)for(let i of Ht.zs)for(let[r,a]of[[1,1],[1,-1],[-1,1],[-1,-1]])e.push({x:n+r*s,z:i+a*s,kind:"corner"});for(let n of Hc){let i=e.length;for(let r of[-1,1])e.push(n.axis==="x"?{x:n.at,z:n.line+r*s,kind:"zebra"}:{x:n.line+r*s,z:n.at,kind:"zebra"});e[i].pair=i+1,e[i+1].pair=i,t.push({a:i,b:i+1,zebra:!0,length:2*s})}for(let n=0;n<e.length;n++)for(let i=n+1;i<e.length;i++){if(e[n].pair===i)continue;let r=Math.hypot(e[n].x-e[i].x,e[n].z-e[i].z);r<Ju&&$u(e[n],e[i])&&t.push({a:n,b:i,zebra:!1,length:r})}return e.forEach(Object.freeze),t.forEach(Object.freeze),Bc=Object.freeze({nodes:Object.freeze(e),links:Object.freeze(t)}),Bc}var ed={agent:[[-8,-8,8,8]],memory:[[-11.4,-10.4,11.4,8.4]],integrations:[[-11.4,-4.9,-4.6,4.9],[4.6,-4.9,11.4,4.9],[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]],missions:[[-13.4,-8.9,13.4,6.9]],infra:[[-10.7,-9.4,10.7,3.4],[-10,-2.6,10,8.5]]},Ly={agent:[23,23],memory:[26,22],integrations:[26,16],missions:[30,21],infra:[25,23],graph:[24,24],operations:[9,9]},td={agent:[[5.9,8,13],[5.4,13,29],[4.7,29,42.5],[4.2,42.5,61.5],[3.6,61.5,72.2],[2.6,72.2,87]]};function jp(s,e,t){for(let n of t){let i=Ly[n.id];if(i&&Math.abs(s-n.x)<i[0]/2&&Math.abs(e-n.z)<i[1]/2)return .88}return 0}function Fy(s,e,t){for(let n of t){let i=s-n.x,r=e-n.z;if(n.id==="graph"&&Math.hypot(i,r)<10.2||n.id==="operations"&&Math.hypot(i,r)<3||(ed[n.id]||[]).some(([a,o,l,c])=>i>a&&i<l&&r>o&&r<c))return!0}return zc.some(n=>Math.abs(s-n.x)<6&&Math.abs(e-n.z)<5)}function Es(s,e){return Ln.find(t=>Math.abs(s-t.x)<t.width/2&&Math.abs(e-t.z)<t.depth/2)}function Fn(s,e){if(s>=-81.5&&s<=-78.5&&e>=46&&e<=52)return(52-e)/3;if(s>=-78.3&&s<=-75.3&&e>=46&&e<=52)return Math.min(2,Math.ceil((52-e)*2)/6);if(s>=-82&&s<=-75&&e>=34&&e<46)return 2;if(s>=-80&&s<=-77&&e>=28&&e<34)return(e-28)/3;if(Es(s,e))return pt.room;for(let n of oi){let i=!!n.angle&&Math.abs(n.angle)!==Math.PI;if(Math.abs(s-n.platformX)<(i?2:4.5)&&Math.abs(e-n.platformZ)<(i?4.5:2))return pt.pavement+.3}let t=Qu(s,e);return t?t.height:e>=74&&e<=80&&s>=-80&&s<=80?pt.quay:pt.ground}function nd(s,e,t,n,i){if(Math.abs(s)>84||e<-90||e>79)return!1;let r=Es(s,e),a=Es(t.x,t.z);if(r||a){let o=r||a;return!(r?.id!==a?.id&&(Math.abs(s-o.x)>1.15||Math.abs(t.x-o.x)>1.15||Math.min((e-o.doorZ)*o.front,(t.z-o.doorZ)*o.front)<-.9||!n(o.id))||r&&(Math.abs(s-o.x)>5.55||(e-o.z)*o.front<-5.55))}return!Fy(s,e,i)}function Gc(s,e=new Date){let t=s==="day"?12:s==="evening"?18.5:s==="night"?0:e.getHours()+e.getMinutes()/60;return{hour:t,amount:Math.max(0,Math.min(1,Math.sin((t-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(t-18.5)/2)}}var Kp=[[[-67,-77],[-18,-77],[-18,-32],[-67,-32]],[[-18,-77],[18,-77],[18,-32],[-18,-32]],[[18,-77],[67,-77],[67,-32],[18,-32]],[[18,13],[18,-32],[67,-32],[67,13]],[[-67,-77],[-18,-77],[-18,-32],[-67,-32]]];function Yp(s,e,t){let n=!1,i=0,r=!1,a=!1,o=0,l=!1,c=new yt;c.name="city-life",s.add(c);let h=t.traffic,d=[],u=Fp({routes:Kp,obstacles:t.obstacles||[],blocked:(ge,xe,$)=>{let ue=d[ge.route],ce=ue?{...ue,x:xe,z:$}:null;return ue?!h.clear(ue,ce,ce):!1}}),p=new Set,x=new Set,v=new Set,m=[],f=new AbortController,y=[],T=[],g=ge=>(p.add(ge),ge),_=ge=>(x.add(ge),ge),S=g(new Jn(1,.013,5,64));for(let ge of e){let xe=_(new qt({color:7452878,transparent:!0,opacity:.08,depthWrite:!1,toneMapped:!1})),$=new ot(S,xe);$.rotation.x=Math.PI/2,$.position.set(ge.x,.6,ge.z),$.scale.setScalar(ge.radius*1.12),c.add($),T.push({id:ge.id,ring:$,material:xe,state:"unknown",eventUntil:0,color:new Ce(7438733)})}let A=new Map,b=[],C=new Map,w=Date.now(),I=3e3,M=e.find(ge=>ge.id==="agent"),N=new k(0,0,1),D=new k,U={memory:12690431,graph:16765844,infra:7978495,integrations:7728086,missions:10268415,operations:16758915};if(M){for(let ge of e)if(ge!==M){let xe=new k(M.x,M.height+3,M.z),$=new k(ge.x,ge.height+4,ge.z),ue=xe.clone().lerp($,.5);ue.y=Math.max(xe.y,$.y)+12;let ce=new xi(xe,ue,$),ie=_(new Pi({color:U[ge.id],transparent:!0,opacity:0,depthWrite:!1,blending:tn,toneMapped:!1})),de=new Ki(g(new Mt().setFromPoints(ce.getPoints(48))),ie);de.visible=!1,c.add(de),A.set(ge.id,{curve:ce,line:de})}}let H=g(new Jn(1,.06,5,40,Math.PI*.95));for(let ge=0;ge<12;ge++){let xe=[];for(let $=0;$<4;$++){let ue=_(new qt({color:10345983,transparent:!0,opacity:0,depthWrite:!1,blending:tn,toneMapped:!1})),ce=new ot($===3?S:H,ue);ce.visible=!1,c.add(ce),xe.push(ce)}b.push({at:-1/0,waves:xe,link:null})}function L(ge,xe){if(!l||t.active?.()===!1||ge.at<w||xe-ge.at>I||ge.at>xe||!["started","succeeded","failed","sanitized","progress"].includes(ge.state))return;let $=ge.to==="agent",ue=$?ge.from:ge.to;if(!$&&ge.from!=="agent"||!A.has(ue))return;let ce=ue+":"+$;if(ge.at-(C.get(ce)??-1/0)<450)return;C.set(ce,ge.at);let ie=b.reduce((de,ve)=>de.at<ve.at?de:ve);Object.assign(ie,{at:ge.at,link:A.get(ue),incoming:$,from:ge.from,to:ge.to,state:ge.state}),ie.waves.forEach(de=>de.material.color.setHex(ge.state==="failed"?16737881:U[ue]))}function W(ge){let xe=Date.now();A.forEach($=>{$.line.visible=!1});for(let $ of b){(!ge||t.active?.()===!1)&&($.at=-1/0);let ue=(xe-$.at)/I,ce=ue>=0&&ue<1;if($.waves.forEach(de=>{de.visible=!1}),!ce||!$.link)continue;$.link.line.visible=!0,$.link.line.material.opacity=.12*Math.sin(ue*Math.PI);for(let de=0;de<3;de++){let ve=(ue-de*.075)/.75;if(ve<0||ve>1)continue;let We=$.incoming?1-ve:ve,Ve=$.waves[de];$.link.curve.getPoint(We,Ve.position),$.link.curve.getTangent(We,D),$.incoming&&D.negate(),Ve.quaternion.setFromUnitVectors(N,D),Ve.rotateZ(Math.PI*.525),Ve.scale.setScalar(1.2+Math.sin(ve*Math.PI)*3.5),Ve.material.opacity=Math.sin(ve*Math.PI)*.8,Ve.visible=!0}let ie=(ue-.74)/.26;if(ie>0){let de=$.waves[3];$.link.curve.getPoint($.incoming?0:1,de.position),de.rotation.set(-Math.PI/2,0,0),de.scale.setScalar(1+ie*8),de.material.opacity=(1-ie)*.6,de.visible=!0}}}let z=g(new un(7,7)),E=_(new wt({transparent:!0,depthWrite:!1,blending:tn,uniforms:{color:{value:new Ce(5495284)}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"varying vec2 vUv;uniform vec3 color;void main(){float r=length(vUv-.5)*2.;float a=exp(-r*r*5.)*.23*(1.-smoothstep(.65,1.,r));gl_FragColor=vec4(color,a);}"})),J=g(new Gs(.42,1.3,12,1,!0)),q=_(new qt({color:9629439,transparent:!0,opacity:.22,depthWrite:!1,toneMapped:!1,blending:tn}));Kp.forEach((ge,xe)=>{let $=new yt;$.name="city-white-robot-"+(xe+1);let ue=new yt;$.add(ue),c.add($);let ce=new ot(z,E);ce.rotation.x=-Math.PI/2,ce.position.y=.08,c.add(ce);let ie=new ot(J,q);ie.rotation.z=Math.PI,ie.position.y=-.45,$.add(ie),$.visible=!1,ce.visible=!1,y.push({shadow:t.shadows?.claim()||null,root:$,body:ue,glow:ce,jet:ie,agent:u.agents[xe],lastX:0,lastZ:0,phase:0,gait:0})});function te(ge){ge.traverse(xe=>{if(xe.isMesh){p.add(xe.geometry);for(let $ of Array.isArray(xe.material)?xe.material:[xe.material]){x.add($);for(let ue of Object.values($))ue?.isTexture&&v.add(ue)}}})}function Le(){p.forEach(xe=>xe.dispose()),x.forEach(xe=>xe.dispose());let ge=new Set;v.forEach(xe=>{xe.image&&ge.add(xe.image),xe.dispose()}),ge.forEach(xe=>xe.close?.()),p.clear(),x.clear(),v.clear()}async function Fe(){let ge=setTimeout(()=>f.abort(),15e3);try{let xe=await fetch(t.robotURL,{signal:f.signal});if(!xe.ok)throw Error("Robot unavailable");let $=await xe.arrayBuffer();if(n)return;let{scene:ue}=await new wi().parseAsync($,"");if(te(ue),n){Le();return}let ce=new hn().setFromObject(ue),ie=ce.getSize(new k),de=Math.max(ie.x,ie.y,ie.z);if(!Number.isFinite(de)||de<=0)throw Error("Invalid robot bounds");ue.scale.setScalar(6/de),ue.updateMatrixWorld(!0),ce.setFromObject(ue);let ve=ce.getCenter(new k);ue.position.set(-ve.x,-ce.min.y,-ve.z),ue.rotation.y=-Math.PI/2,ue.updateMatrixWorld(!0),ce.setFromObject(ue),ce.expandByScalar(.4);let We=ns({min:ce.min.toArray(),max:ce.max.toArray()});ue.traverse(Ve=>{Ve.isMesh&&(Ve.castShadow=!1,Ve.receiveShadow=!0)}),y.forEach((Ve,lt)=>{if(Ve.body.add(ue.clone(!0)),Ve.root.visible=!0,Ve.glow.visible=!0,h){let nt=Ve.agent,ct=null;for(let Q=0;Q<600&&!ct;Q++)ct=h.register("patrol-"+lt,We,{x:nt.x,y:1.6,z:nt.z,heading:nt.heading},1),ct||u.seek(nt,nt.s+1);ct?(d[lt]=ct,t.society?.patrol(ct.id,ct,Ve.root,(Q,Pt)=>{nt.held=Q,Pt&&u.adopt(nt,Pt)})):(Ve.root.visible=!1,Ve.glow.visible=!1,nt.held=!0)}}),r=!0,oe(0,!1)}catch(xe){n||(a=!0,t.onError?.(xe))}finally{clearTimeout(ge)}}function Ne(ge){m.forEach($=>{$.dispose(),x.delete($)}),m.length=0,ge.children.find($=>$.userData.district==="operations")?.getObjectByName("signal")?.traverse($=>{if(!$.isMesh)return;let ue=ce=>{let ie=_(ce.clone());return m.push(ie),ie};$.material=Array.isArray($.material)?$.material.map(ue):ue($.material)})}function se(ge,xe=[]){for(let ue of T){let ce=ge.find(ie=>ie.id===ue.id);ue.state=!ce||ce.stale?"unknown":ce.state,ue.color.setHex(ue.state==="error"?16730430:ue.state==="running"?7400403:ue.state==="unknown"?7438733:7452878),ue.material.color.copy(ue.color)}let $=Date.now();for(let ue of[...xe].reverse())if(ue.id>o&&(L(ue,$),l&&ue.at>=w&&t.active?.()!==!1&&t.society?.event(ue),$-ue.at<6e3)){let ce=T.find(ie=>ie.id===ue.district);ce&&(ce.eventUntil=ue.at+5e3)}o=Math.max(o,...xe.map(ue=>ue.id))}function oe(ge,xe){if(n)return;l=xe,W(xe);let $=xe?Math.min(.1,Math.max(0,ge)):0,ue=h?u.agents.map(ce=>({...ce})):null;xe&&$>0&&(i+=$,u.step($));for(let[ce,ie]of y.entries()){let de=ie.agent,ve=Math.hypot(de.vx,de.vz),We=(!xe||de.held)&&d[ce]?d[ce]:de,Ve=Math.hypot(We.x-ie.lastX,We.z-ie.lastZ),lt=Ve>.5?0:Ve;ie.lastX=We.x,ie.lastZ=We.z,ie.phase+=lt*1.4,ie.gait=It.damp(ie.gait,Math.min(1,lt/Math.max($,1e-4)/1.2),6,$);let nt=ie.gait,ct=1-nt;ie.root.position.set(We.x,1.6+Math.sin(ie.phase)*.14*nt+Math.sin(i*.7+ce*2.1)*.03*ct,We.z),ie.root.rotation.y=We.heading;let Q=(de.vx*de.rx+de.vz*de.rz)/Math.max(1,ve);ie.body.rotation.z=Math.sin(i*.85+ce)*.035*nt-Q*.08,ie.body.rotation.x=-.035+Math.sin(i*1.1+ce)*.018*nt-Math.min(.09,ve*.015),ie.glow.position.x=We.x,ie.glow.position.z=We.z,ie.jet.scale.y=1+Math.sin(i*4+ce)*.12+ve*.04;let Pt=Fn(ie.glow.position.x,ie.glow.position.z);if(ie.glow.position.y=Pt+.08,ie.shadow&&(ie.root.visible?ie.shadow.set(ie.glow.position.x,Pt,ie.glow.position.z,1.1,.3):ie.shadow.hide()),xe&&d[ce]&&!de.held){let ft=d[ce],O={x:de.x,y:1.6,z:de.z,heading:de.heading};ie.escape&&i<ie.escape.until?(O.x=ft.x+ie.escape.x*$*1.8,O.z=ft.z+ie.escape.z*$*1.8,O.heading=ft.heading):ie.escape=null,!h.clear(ft,ft,O,!1)&&h.clear(ft,ft,{...O,heading:ft.heading},!1)&&(O.heading=ft.heading);let R=Math.hypot(O.x-ft.x,O.z-ft.z);h.propose(ft,O,(re,le)=>{if(ie.wait=re&&R>.002?0:(ie.wait||0)+$,ie.escape&&re&&u.adopt(de,le),ie.wait>2.5){let be=Math.sin(le.heading),De=Math.cos(le.heading);for(let[Ue,V]of[[De,-be],[-De,be],[be,De],[-be,-De]])if(h.clear(le,le,{...le,x:le.x+Ue*2.5,z:le.z+V*2.5})){ie.escape={x:Ue,z:V,until:i+2};break}ie.wait=0}if(re)de.trafficWait=0,de.heading=le.heading;else{let be=(de.trafficWait||0)+$;Object.assign(de,ue[ce]),de.speed=0,de.vx=de.vz=0,de.trafficWait=be,be>2.5&&(de.state!=="turn"?(de.dir=-de.dir,de.lane=-de.lane,de.laneTarget=2.2,de.state="turn",de.turns++):de.state="cruise",de.trafficWait=0)}ie.root.position.x=le.x,ie.root.position.z=le.z,ie.root.rotation.y=le.heading,ie.glow.position.x=le.x,ie.glow.position.z=le.z})}}for(let ce of T){let ie=xe?.5+.5*Math.sin(i*(ce.state==="error"?3.2:1.8)):.5,de=ce.state==="error"||ce.state==="running"||ce.eventUntil>Date.now();if(ce.material.opacity=ce.state==="unknown"?.03:de?.2+ie*.46:.065,ce.ring.scale.setScalar(e.find(ve=>ve.id===ce.id).radius*1.12*(1+(de&&xe?ie*.035:0))),ce.id==="operations")for(let ve of m)ve.color.copy(ce.color),ve.emissive?.copy(ce.color),ve.emissiveIntensity=ce.state==="error"?1.1+ie*2.2:ce.state==="unknown"?.08:.55}}function Pe(){n||(n=!0,f.abort(),t.signal?.removeEventListener("abort",Pe),c.removeFromParent(),Le(),y.length=0,m.length=0,d.forEach(ge=>h.remove(ge.id)))}return t.signal?.addEventListener("abort",Pe,{once:!0}),t.signal?.aborted?Pe():Fe(),{update:oe,setData:se,attachLandmarks:Ne,dispose:Pe,stats:()=>({robots:r?y.filter(ge=>ge.root.visible).length:0,robotError:a,time:i,navigation:{...u.stats(),gaits:y.map(ge=>+(ge.gait||0).toFixed(2)),held:y.map(ge=>!!ge.agent.held),enabled:y.map(ge=>!!d[y.indexOf(ge)]?.enabled)},positions:y.map(ge=>ge.root.position.toArray()),transmissions:b.filter(ge=>ge.waves.some(xe=>xe.visible)).map(ge=>({from:ge.from,to:ge.to,state:ge.state,age:Date.now()-ge.at})),signals:T.map(ge=>({id:ge.id,state:ge.state,intensity:ge.material.opacity}))})}}var go='Geist, "Segoe UI", system-ui, sans-serif',Vc=13,Ny=s=>{let e=2166136261;for(let t=0;t<s.length;t++)e=Math.imul(e^s.charCodeAt(t),16777619);return(e>>>0).toString(16).padStart(8,"0")},id={vertex:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:`uniform sampler2D map;uniform float time,glitch,fade,gain;uniform vec3 tint;varying vec2 vUv;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){vec2 uv=vUv;float row=floor(uv.y*44.0),frame=floor(time*24.0);
float g=glitch*step(0.7,hash(row+frame*0.37));uv.x+=(hash(row*3.1+frame)-0.5)*0.14*g;
float split=0.0025+glitch*0.02;vec4 c=texture2D(map,uv);
float r=texture2D(map,uv+vec2(split,0.0)).r,b=texture2D(map,uv-vec2(split,0.0)).b;
float a=max(c.a,max(texture2D(map,uv+vec2(split,0.0)).a,texture2D(map,uv-vec2(split,0.0)).a));
float lines=0.84+0.16*sin(uv.y*420.0-time*9.0);float flicker=0.95+0.05*sin(time*31.0)*sin(time*7.3);
float bx=(fract(uv.y*0.55-time*0.11)-0.5)*22.0;float band=0.35*exp(-bx*bx);
float edge=smoothstep(0.0,0.05,uv.x)*smoothstep(0.0,0.05,1.0-uv.x)*smoothstep(0.0,0.09,uv.y)*smoothstep(0.0,0.09,1.0-uv.y);
vec3 col=vec3(r,c.g,b)*tint*(lines*flicker+band)*gain;gl_FragColor=vec4(col*a*fade*edge,1.0);}`},Uy={vertex:"varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",fragment:`uniform float time,strength;uniform vec3 base,top;varying vec2 vUv;varying vec3 vNormal,vView;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){float h=vUv.y;float rim=pow(clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0),1.5);
float fall=pow(clamp(1.0-h,0.0,1.0),1.35)*0.75+0.25*clamp(1.0-h,0.0,1.0);float scan=0.7+0.3*sin(h*64.0-time*5.5);
float noise=0.85+0.15*hash(floor(h*96.0)+floor(time*18.0));float shimmer=0.8+0.2*sin(vUv.x*40.0+time*2.0);
float a=(0.28+0.72*rim)*fall*scan*noise*shimmer*strength;gl_FragColor=vec4(mix(base,top,h)*a,1.0);}`},Oy={vertex:id.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float r=length(vUv-0.5)*2.0;float rings=smoothstep(0.35,1.0,sin(r*16.0-time*2.6));
float core=exp(-r*r*7.0);float a=(core*1.2+pow(max(0.0,1.0-r),1.8)*(0.25+0.75*rings)*0.6)*strength*(1.0-smoothstep(0.85,1.0,r));
gl_FragColor=vec4(color*a,1.0);}`},Jp={vertex:id.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float dash=step(0.45,fract(vUv.x*28.0+time*0.35));float pulse=0.75+0.25*sin(vUv.x*6.2831*3.0-time*4.0);
gl_FragColor=vec4(color*dash*pulse*strength,1.0);}`},zy={vertex:`attribute float seed;uniform float time,pointScale;varying float vLife;
void main(){float speed=0.55+seed*0.9;float life=fract(seed*3.17+time*speed/13.0);float y=life*13.0;
float ang=seed*6.2831+time*(0.35+seed*0.4)+life*2.2;float rad=mix(1.8,6.8,life)*(0.3+0.7*fract(seed*7.13));
vec4 mv=modelViewMatrix*vec4(cos(ang)*rad,y,sin(ang)*rad,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.09+0.16*fract(seed*3.7))*pointScale/max(1.0,-mv.z);vLife=life;}`,fragment:`uniform vec3 color;uniform float strength;varying float vLife;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,2.0)*sin(vLife*3.1416)*strength;gl_FragColor=vec4(color*a,1.0);}`};function na(s,e,t={}){return new wt({uniforms:e,vertexShader:s.vertex,fragmentShader:s.fragment,transparent:!0,depthWrite:!1,blending:tn,side:Yt,...t})}function ky(s,e=24){let t=[],n=new Set;for(let i of Array.isArray(s)?s:[]){if(typeof i!="string")continue;let r=i.replace(/[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u2028-\u202e\u2066-\u2069]/g," ").replace(/\s+/g," ").trim().slice(0,140);if(!(r.length<4||n.has(r))&&(n.add(r),t.push(r),t.length>=e))break}return t}function Zp(s,e,t,n){let i=[],r=c=>s.measureText(c).width<=t,a="";for(let c of e.split(" ")){if(i.length>=n)break;let h=a?a+" "+c:c;if(r(h)){a=h;continue}if(a&&(i.push(a),a="",i.length>=n))break;let d=c;for(;!r(d)&&i.length<n;){let u=d.length-1;for(;u>1&&!r(d.slice(0,u));)u--;i.push(d.slice(0,u)),d=d.slice(u)}a=d}a&&i.length<n&&(i.push(a),a="");let o=i.join("").replace(/\s/g,"").length,l=e.replace(/\s/g,"").length;if(o<l&&i.length){let c=i[i.length-1].replace(/[\s,.;:]+$/,"");for(;c.length&&!r(c+"\u2026");)c=c.slice(0,-1);i[i.length-1]=c+"\u2026"}return i.length?i:[""]}function Qp(s,e,t={}){let n=t.roof??21,i=String(t.label||"MEMORY").toUpperCase(),r=new yt;r.name="memory-hologram",r.position.set(e.x,n,e.z),s.add(r);let a=[],o=[],l=[],c=($,ue)=>(a.push($),o.push(ue),new ot($,ue)),h=new Ce(6544639),d=new Ce(10980351),u=new Ce(14218751),p={time:{value:0}},x=!1,v=0,m=[],f=[],y=0,T="none",g=!1,_=c(new un(11,11),na(Oy,{time:p.time,strength:{value:.9},color:{value:h}}));_.rotation.x=-Math.PI/2,_.position.y=.18,r.add(_);let S=c(new gs(7.4,2.3,Vc,56,1,!0),na(Uy,{time:p.time,strength:{value:.32},base:{value:h},top:{value:d}}));S.position.y=Vc/2+.3,r.add(S);let A=c(new Jn(7.5,.055,6,128),na(Jp,{time:p.time,strength:{value:1.6},color:{value:h}}));A.rotation.x=Math.PI/2,A.position.y=Vc+.3,r.add(A);let b=c(new Jn(2.6,.05,6,96),na(Jp,{time:p.time,strength:{value:2},color:{value:u}}));b.rotation.x=Math.PI/2,b.position.y=.55,r.add(b);let C=new Fr(1.35,1),w=new La(C),I=new Pi({color:10217983,transparent:!0,opacity:.75,blending:tn,depthWrite:!1,toneMapped:!1}),M=new Bs(w,I);M.position.y=4.6,r.add(M),a.push(C,w),o.push(I);let N=new qt({color:4175871,transparent:!0,opacity:.12,blending:tn,depthWrite:!1,toneMapped:!1}),D=c(new Fr(1.1,1),N);M.add(D);let U=160,H=new Float32Array(U);for(let $=0;$<U;$++)H[$]=$*.618033988749895%1;let L=new Mt;L.setAttribute("position",new Tt(new Float32Array(U*3),3)),L.setAttribute("seed",new Tt(H,1)),L.boundingSphere=new In(new k(0,Vc/2,0),12);let W=na(zy,{time:p.time,pointScale:{value:800},color:{value:u},strength:{value:.9}}),z=new Tn(L,W);z.frustumCulled=!1,r.add(z),a.push(L),o.push(W);let E=new bi(6478079,26,46,2);E.position.y=6,r.add(E);function J($,ue,ce,ie){let de=document.createElement("canvas");de.width=ce,de.height=ie;let ve=de.getContext("2d"),We=new Hs(de);We.colorSpace=sn,We.generateMipmaps=!1,We.minFilter=rn,l.push(We);let Ve=na(id,{map:{value:We},time:p.time,glitch:{value:0},fade:{value:0},gain:{value:1.35},tint:{value:new Ce(16777215)}},{side:ii}),lt=c(new un($,ue),Ve);return{canvas:de,context:ve,texture:We,material:Ve,mesh:lt,text:"",targetFade:1,next:0}}let q=J(17.5,8.55,1024,500),te=[J(8.6,2.1,640,156),J(8.6,2.1,640,156)];q.mesh.position.y=10.4,r.add(q.mesh),te.forEach(($,ue)=>{$.mesh.position.y=ue?14.4:5.9,$.orbit=ue?-.22:.27,$.angle=ue*2.3,r.add($.mesh)});function Le($,ue,ce){let{context:ie,canvas:de}=q,ve=de.width,We=de.height;ie.clearRect(0,0,ve,We),ie.fillStyle="rgba(48,150,214,0.14)",ie.beginPath(),ie.roundRect(14,14,ve-28,We-28,22),ie.fill(),ie.strokeStyle="rgba(150,230,255,0.85)",ie.lineWidth=3;for(let[Ve,lt,nt,ct]of[[18,18,1,1],[ve-18,18,-1,1],[18,We-18,1,-1],[ve-18,We-18,-1,-1]])ie.beginPath(),ie.moveTo(Ve,lt+ct*42),ie.lineTo(Ve,lt),ie.lineTo(Ve+nt*42,lt),ie.stroke();ie.fillStyle="rgba(150,230,255,0.55)",ie.fillRect(48,108,ve-96,2),ie.font="600 27px "+go,ie.textBaseline="middle",ie.fillStyle="rgba(160,232,255,0.92)",ie.textAlign="left",ie.fillText("\u258C "+i+(ce?"  \xB7  "+String(ue+1).padStart(2,"0")+" / "+String(ce).padStart(2,"0"):""),50,72),ie.textAlign="right",ie.font="500 25px "+go,ie.fillStyle="rgba(190,150,255,0.85)",ie.fillText("0x"+Ny($||i).toUpperCase(),ve-52,72),ie.textAlign="left",ie.shadowColor="rgba(120,225,255,0.9)",ie.shadowBlur=16,$?(ie.font="600 54px "+go,ie.fillStyle="rgba(232,250,255,0.97)",Zp(ie,$,ve-110,4).forEach((lt,nt)=>ie.fillText(lt,54,172+nt*72))):(ie.font="600 40px "+go,ie.fillStyle="rgba(180,235,255,0.7)",ie.fillText("\u25AE \u25AE \u25AF \u25AE \u25AF \u25AF \u25AE \u25AF \u25AE \u25AE \u25AF \u25AE",54,250)),ie.shadowBlur=0,q.texture.needsUpdate=!0}function Fe($,ue){let{context:ce,canvas:ie}=$,de=ie.width,ve=ie.height;ce.clearRect(0,0,de,ve),ce.fillStyle="rgba(48,150,214,0.12)",ce.beginPath(),ce.roundRect(6,6,de-12,ve-12,14),ce.fill(),ce.fillStyle="rgba(190,150,255,0.8)",ce.fillRect(20,26,6,ve-52),ce.font="600 42px "+go,ce.textBaseline="middle",ce.textAlign="left",ce.shadowColor="rgba(160,140,255,0.9)",ce.shadowBlur=12,ce.fillStyle="rgba(236,240,255,0.95)",ce.fillText(ue?Zp(ce,ue,de-70,1)[0]:"\u25AF \u25AE \u25AF \u25AE \u25AF",42,ve/2),ce.shadowBlur=0,$.texture.needsUpdate=!0}function Ne(){return m.length?(y>=f.length&&(f=m.map(($,ue)=>ue).sort(()=>Math.random()-.5),y=0),m[f[y++]]):""}let se=.8,oe=[3.5,6.5];function Pe($){let ue=Ne();q.text=ue,v++,Le(ue,m.indexOf(ue),m.length),q.material.uniforms.glitch.value=$?1:0,q.material.uniforms.fade.value=$?.35:1,se=x?12:4.5+Math.random()*3}function ge($,ue){let ce=te[$],ie=Ne();ce.text=ie,Fe(ce,ie.length>46?ie.slice(0,44).replace(/\s+\S*$/,"")+"\u2026":ie),ce.material.uniforms.glitch.value=ue?.7:0,ce.material.uniforms.fade.value=ue?.3:1,oe[$]=x?15:6+Math.random()*4}Le("",0,0),te.forEach($=>Fe($,"")),document.fonts?.ready?.then(()=>{g||(Le(q.text,m.indexOf(q.text),m.length),te.forEach($=>Fe($,$.text)))});let xe=new k;return{group:r,setTexts($,ue="live"){let ce=ky($),ie=ce.length!==m.length||ce.some((de,ve)=>de!==m[ve]);m=ce,T=ce.length?ue:"none",ie&&(f=[],y=0,(!q.text||!m.includes(q.text))&&(se=Math.min(se,.6)))},setPointScale($){W.uniforms.pointScale.value=$},setReducedMotion($){x=!!$},update($,ue,ce){if(g)return;let ie=ce&&!x,de=ie?Math.min(.1,Math.max(0,$)):0;p.time.value+=de,se-=$,se<=0&&Pe(ie),oe.forEach((ve,We)=>{oe[We]=ve-$,oe[We]<=0&&ge(We,ie)});for(let ve of[q,...te]){let We=ve.material.uniforms;We.glitch.value=Math.max(0,We.glitch.value-$*2.4),We.fade.value=Math.min(1,We.fade.value+$*1.8),ve.mesh.quaternion.copy(ue.quaternion)}te.forEach(ve=>{ve.angle+=ve.orbit*de,ve.mesh.position.x=Math.cos(ve.angle)*6.4,ve.mesh.position.z=Math.sin(ve.angle)*6.4,xe.copy(ve.mesh.position).add(r.position).sub(ue.position).normalize();let We=-(xe.x*Math.cos(ve.angle)+xe.z*Math.sin(ve.angle));ve.material.uniforms.gain.value=1.35*It.clamp(.35+We*.9,.15,1.2)}),q.mesh.position.y=9.6+Math.sin(p.time.value*.9)*.22,M.rotation.y+=de*.7,M.rotation.x+=de*.31,D.rotation.y-=de*1.1,A.rotation.z+=de*.18,b.rotation.z-=de*.42,E.intensity=26+Math.sin(p.time.value*2.1)*6+q.material.uniforms.glitch.value*22},stats:()=>({artifacts:m.length,source:T,switches:v,shown:q.text?m.indexOf(q.text):-1}),dispose(){g||(g=!0,r.removeFromParent(),a.forEach($=>$.dispose()),o.forEach($=>$.dispose()),l.forEach($=>$.dispose()),E.dispose())}}}var By=`uniform sampler2D tDiffuse;uniform float time,vignette,grain,aberration,streak,shafts,letterbox,aspect,contrast,saturation;
uniform vec2 lightPos;uniform vec3 shaftColor,shadowTint,highlightTint;varying vec2 vUv;
float hash2(vec2 p){return fract(sin(dot(p,vec2(12.9898,78.233)))*43758.5453);}
float luma(vec3 c){return dot(c,vec3(0.2126,0.7152,0.0722));}
void main(){vec2 d=vUv-0.5;float r2=dot(d,d);
vec2 off=d*aberration*r2*4.0;
vec3 c=vec3(texture2D(tDiffuse,vUv+off).r,texture2D(tDiffuse,vUv).g,texture2D(tDiffuse,vUv-off).b);
if(streak>0.0){vec3 s=vec3(0.0);
for(int i=1;i<=6;i++){float o=float(i)*0.009,w=1.0-float(i)/7.0;
s+=(max(texture2D(tDiffuse,vUv+vec2(o,0.0)).rgb-0.8,0.0)+max(texture2D(tDiffuse,vUv-vec2(o,0.0)).rgb-0.8,0.0))*w;}
c+=vec3(luma(s))*vec3(0.35,0.62,1.0)*streak;}
if(shafts>0.0){vec2 delta=(vUv-lightPos)/16.0;vec2 uv=vUv;float decay=1.0,sum=0.0;
for(int i=0;i<16;i++){uv-=delta;vec2 q=(uv-lightPos)*vec2(aspect,1.0);
float l=luma(texture2D(tDiffuse,clamp(uv,0.0,1.0)).rgb);sum+=max(l-0.72,0.0)*decay*exp(-dot(q,q)*16.0);decay*=0.93;}
c+=shaftColor*sum*shafts;}
float l=luma(c);c=mix(vec3(l),c,saturation);
c+=shadowTint*(1.0-smoothstep(0.0,0.45,l))+highlightTint*smoothstep(0.5,1.0,l);
c=clamp(c,0.0,1.0);c=mix(c,c*c*(3.0-2.0*c),contrast);
c*=1.0-vignette*smoothstep(0.28,1.05,r2*2.4);
c+=(hash2(vUv*vec2(1920.0,1080.0)+fract(time)*7.0)-0.5)*grain*(0.45+0.55*(1.0-l));
float bar=letterbox*clamp((1.0-aspect/2.39)*0.5,0.0,0.12);
c*=smoothstep(bar,bar+0.003,vUv.y)*smoothstep(bar,bar+0.003,1.0-vUv.y);
gl_FragColor=vec4(c,1.0);}`;function $p(s){let e={tDiffuse:{value:null},time:s,vignette:{value:.5},grain:{value:.03},aberration:{value:.0024},streak:{value:.16},shafts:{value:0},letterbox:{value:0},aspect:{value:1.7777777777777777},contrast:{value:.22},saturation:{value:1.08},lightPos:{value:new ze(.5,.5)},shaftColor:{value:new Ce(12376319)},shadowTint:{value:new Ce(6686)},highlightTint:{value:new Ce(1444864)}},t=new $r(new wt({uniforms:e,vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:By}));t.enabled=!1;let n=new k,i=new Ce(16761992),r=new Ce(12376319),a=!1;return{pass:t,setCinematic(o){a=!!o},update(o,l,c,{day:h,dusk:d,cloud:u,animated:p}){e.aspect.value=l.aspect;let x=a?1:0;e.letterbox.value=p?e.letterbox.value+(x-e.letterbox.value)*Math.min(1,o*2.2):x,n.copy(c).multiplyScalar(1e3).add(l.position).project(l);let v=(n.x+1)/2,m=(n.y+1)/2,f=n.z<1?Math.max(0,1-Math.max(0,Math.abs(v-.5)-.5,Math.abs(m-.5)-.5)*4):0;e.lightPos.value.set(v,m),e.shafts.value=f*(.06+d*.14+(1-h)*.03)*(1-u*.75),e.shaftColor.value.copy(r).lerp(i,d),e.shadowTint.value.setRGB(0,.018+.01*(1-h),.024+.01*(1-h)),e.highlightTint.value.setRGB(.02+d*.03,.01+d*.006,0)},stats:()=>({enabled:t.enabled,letterbox:+e.letterbox.value.toFixed(2),shafts:+e.shafts.value.toFixed(2)}),dispose(){t.material.dispose(),t.dispose?.()}}}var Hy=new Ce(16767392),Gy=new Ce(16758903),rd=`float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`,sd="varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}",Vy="float fbm(vec2 p){float v=0.0,a=0.5;for(int i=0;i<5;i++){if(float(i)>=octaves)break;v+=a*vnoise(p);p=p*2.03+vec2(1.7,9.2);a*=0.5;}return v;}",Wy=`uniform float time,day,dusk,cloud,flash,octaves;uniform vec3 zenith,horizon,haze,warm,sunDir,glow;varying vec3 vWorld;${rd}${Vy}
void main(){vec3 d=normalize(vWorld);float h=clamp(d.y,-0.08,1.0),hp=max(h,0.0),night=1.0-day;
vec3 col=mix(horizon,zenith,pow(smoothstep(0.0,0.62,hp),0.55));
vec3 s=normalize(sunDir);
vec3 level=normalize(vec3(d.x,0.0,d.z)+vec3(0.0,0.0,1e-4));
float az=max(0.0,dot(level,normalize(vec3(s.x,0.0,s.z)+vec3(0.0,0.0,1e-4))));
col+=warm*pow(az,4.0)*exp(-hp*7.0)*(0.55+dusk*1.5);
col+=vec3(0.42,0.13,0.3)*dusk*pow(az,1.2)*exp(-hp*2.4)*0.45;
col+=glow*exp(-hp*11.0)*night*(1.0-dusk*0.5);
float bend=atan(d.x,d.z);
float b1=(h-0.2+0.05*sin(bend*3.0+time*0.07))*13.0,b2=(h-0.33+0.04*sin(bend*2.5-time*0.05+1.7))*17.0;
float a1=exp(-b1*b1),a2=exp(-b2*b2);
float ripple=0.5+0.5*sin(bend*7.0+time*0.16+sin(bend*3.0)*1.3);
col+=(vec3(0.08,0.32,0.34)*a1*ripple*0.5+vec3(0.26,0.12,0.4)*a2*(1.0-ripple)*0.42)*night*(1.0-cloud*0.8);
vec3 bn=normalize(vec3(0.42,0.3,-0.86));float band=dot(d,bn);
float mw=exp(-band*band*22.0)*(0.35+0.65*fbm(vec2(bend*4.0,hp*9.0)+3.1))*smoothstep(0.04,0.3,hp);
col+=vec3(0.13,0.15,0.24)*mw*night*(1.0-cloud)*0.9;
float period=9.0,idx=floor(time/period),lt=time-idx*period;
float r1=hash2(vec2(idx,1.3)),r2=hash2(vec2(idx,7.1)),r3=hash2(vec2(idx,3.7));
if(night>0.5&&r3>0.35&&lt<1.1){vec2 q=d.xz/(hp+0.35);vec2 st=vec2(r1*2.4-1.2,-r2*1.4-0.2),dir=normalize(vec2(r3-0.65,0.55));
float prog=lt/1.1,len=prog*0.9;vec2 pa=q-st;float t=clamp(dot(pa,dir),0.0,len),dist=length(pa-dir*t);
col+=vec3(0.75,0.85,1.0)*exp(-dist*dist*40000.0)*smoothstep(len-0.4,len,t)*sin(prog*3.14159)*night*(1.0-cloud)*1.6*smoothstep(0.08,0.2,hp);}
float mu=max(0.0,dot(d,s));
float disc=smoothstep(0.99955,0.99982,mu);
float crater=1.0-0.22*night*(1.0-dusk)*vnoise(d.xy*180.0+d.z*97.0);
vec3 discCol=mix(vec3(0.92,0.95,1.06),vec3(1.9,0.85,0.4),dusk);
vec3 haloCol=mix(vec3(0.45,0.62,1.0),vec3(1.0,0.55,0.25),dusk);
float halo=pow(mu,220.0)*0.55+pow(mu,48.0)*0.12+pow(mu,6.0)*dusk*0.22;
vec2 cp=d.xz/(hp+0.1)*0.55+vec2(time*0.004,time*0.0025);
float n=fbm(cp)+0.35*vnoise(cp*5.0-vec2(time*0.01,0.0))-0.15;
float th=mix(0.66,0.28,cloud);
float dens=smoothstep(th,th+0.3,n)*smoothstep(0.0,0.1,hp)*(0.55+0.45*cloud);
float lit=pow(clamp(dot(d,s)*0.5+0.5,0.0,1.0),3.0);
vec3 cNight=vec3(0.035,0.045,0.07)+glow*0.55*exp(-hp*3.0);
vec3 cDay=mix(vec3(0.5,0.56,0.64),vec3(0.98,0.98,1.0),lit);
vec3 cDusk=mix(vec3(0.2,0.11,0.2),vec3(1.1,0.52,0.26),lit*az);
vec3 cc=mix(mix(cNight,cDay,day),cDusk,dusk*0.9)+haloCol*pow(mu,16.0)*(0.12+0.3*day+0.3*dusk)+vec3(0.55,0.6,0.85)*flash;
col+=discCol*disc*2.4*crater*(1.0-dens*0.9)+haloCol*halo*(1.0-dens*0.6);
col=mix(col,cc,dens);
col+=vec3(0.35,0.38,0.55)*flash*0.35;
col=mix(haze,col,smoothstep(0.0,0.14,h));
col=mix(col,haze*0.6,smoothstep(0.0,0.08,-d.y));
gl_FragColor=vec4(col,1.0);}`,em={vertex:`attribute float phase,speed,size;uniform float time,pointScale,cloud;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0)*(1.0-cloud*0.85);
float k=fract(phase*3.1);vTint=k>0.86?vec3(1.0,0.84,0.66):k>0.7?vec3(0.7,0.82,1.0):vec3(0.82,0.9,1.0);}`,fragment:"varying float vAlpha;varying vec3 vTint;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vTint*a,1.0);}"},tm={vertex:`varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,fragment:`uniform float time,day;uniform vec3 deep,shallow,sky,sunDir,sunColor,glow;uniform vec4 island;varying vec3 vWorld;
#include <fog_pars_fragment>
${rd}
float height(vec2 p){return 0.36*sin(p.x*0.09+time*0.9+sin(p.y*0.07)*1.5)+0.26*sin(p.y*0.11-time*0.7+p.x*0.03)
+0.12*sin((p.x+p.y)*0.21+time*1.4)+0.07*sin(p.x*0.52-p.y*0.37+time*2.2);}
void main(){vec2 p=vWorld.xz;float e=0.6;
vec3 n=normalize(vec3(height(p-vec2(e,0.0))-height(p+vec2(e,0.0)),2.0*e*0.9,height(p-vec2(0.0,e))-height(p+vec2(0.0,e))));
vec3 view=normalize(cameraPosition-vWorld);float fresnel=pow(clamp(1.0-dot(n,view),0.0,1.0),4.0);
vec3 col=mix(deep,shallow,clamp(0.5+n.x*2.5,0.0,1.0));col=mix(col,sky,0.22+0.62*fresnel);
vec3 refl=reflect(-view,n);float spec=pow(max(dot(refl,sunDir),0.0),260.0)*2.6+pow(max(dot(refl,sunDir),0.0),28.0)*0.22;
float sparkle=pow(max(0.0,sin(p.x*3.7+time*3.0)*sin(p.y*4.1-time*2.3)),40.0)*0.35*max(0.0,dot(refl,sunDir));
float path=pow(max(dot(refl,sunDir),0.0),60.0)*smoothstep(0.55,0.95,vnoise(p*1.7+vec2(time*0.8,-time*0.5)))*0.9;
col+=sunColor*(spec+sparkle+path);
vec2 q=max(abs(p-island.xy)-island.zw,0.0);float edge=length(q);
float wobble=(vnoise(p*0.35+vec2(time*0.4,time*0.23))-0.5)*2.6;
float foam=smoothstep(3.2,0.0,edge+wobble)*(0.55+0.45*sin(edge*1.7-time*2.1))*smoothstep(0.35,0.7,vnoise(p*1.3-time*0.3));
col=mix(col,vec3(0.62,0.74,0.8)*(0.28+0.72*day),clamp(foam,0.0,1.0)*0.7);
float night=1.0-day;
col+=glow*exp(-edge*0.045)*night*(0.3+0.7*smoothstep(0.3,0.85,vnoise(p*0.45+vec2(time*0.35,-time*0.2))*vnoise(p*0.17-vec2(time*0.12,0.0))*1.8))*(0.25+fresnel*0.9);
gl_FragColor=vec4(col,1.0);
#include <fog_fragment>
}`},qy=`uniform float time,strength;uniform vec3 color;uniform vec4 island;varying vec3 vWorld;${rd}
void main(){vec2 p=vWorld.xz;float n=vnoise(p*0.012+vec2(time*0.017,-time*0.011))*0.6+vnoise(p*0.031-vec2(time*0.02,time*0.013))*0.4;
float mist=smoothstep(0.32,0.82,n);vec2 d2=max(abs(p-island.xy)-island.zw,0.0);float outside=smoothstep(0.0,70.0,length(d2));
float far=1.0-smoothstep(500.0,850.0,length(p));gl_FragColor=vec4(color,mist*outside*far*strength);}`,nm={vertex:`attribute vec3 seed;uniform float time,pointScale;varying float vAlpha;
void main(){vec3 p=position;p.x+=sin(time*0.11*seed.x+seed.y*6.28)*9.0+time*0.35*(seed.z-0.5);p.y+=sin(time*0.13+seed.z*6.28)*3.0;
p.z+=cos(time*0.09*seed.y+seed.x*6.28)*9.0;p.x=mod(p.x+90.0,180.0)-90.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.06+0.11*seed.x)*pointScale/max(1.0,-mv.z);vAlpha=0.55+0.45*sin(time*(1.0+seed.y)+seed.z*6.28);}`,fragment:`uniform vec3 color;uniform float strength;varying float vAlpha;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,2.2)*vAlpha*strength,1.0);}`},Xy=`uniform float strength;uniform vec3 color;varying vec2 vUv;
void main(){float along=pow(clamp(1.0-vUv.x,0.0,1.0),2.4);float across=pow(clamp(1.0-abs(vUv.y-0.5)*2.0,0.0,1.0),1.7);gl_FragColor=vec4(color*along*across*strength,1.0);}`,jy=`uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;
void main(){float facing=clamp(abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0);float h=clamp(vUv.y,0.0,1.0);
float a=pow(h,2.4)*pow(facing,1.5)*strength;gl_FragColor=vec4(color*a,1.0);}`,Ky=`uniform vec3 color;uniform float strength;varying vec2 vUv;
void main(){float d=clamp(length(vUv-0.5)*2.0,0.0,1.0);float f=1.0-smoothstep(0.0,1.0,d);f=f*(0.35+0.65*f);gl_FragColor=vec4(color*f*strength,1.0);}`,im={vertex:"attribute float lift;varying float vLift;void main(){vLift=lift;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:"uniform vec3 haze;uniform float shade;varying float vLift;void main(){gl_FragColor=vec4(haze*mix(1.0,shade,smoothstep(0.0,0.6,vLift)),1.0);}"},sm={vertex:`attribute float phase;uniform float time,pointScale,strength;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=strength*(0.75+0.25*sin(time*(0.7+fract(phase)*1.3)+phase));gl_PointSize=clamp(2.2*pointScale/max(1.0,-mv.z),1.5,4.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(1.4,0.95,0.55)*pow(1.0-d,1.5)*vA,1.0);}"},rm={vertex:"varying vec2 vUv;varying float vRim,vNear;void main(){vUv=uv;vec3 n=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vRim=abs(dot(n,normalize(-mv.xyz)));vNear=smoothstep(40.0,160.0,-mv.z);gl_Position=projectionMatrix*mv;}",fragment:"uniform vec3 color;uniform float strength;varying vec2 vUv;varying float vRim,vNear;void main(){float along=pow(clamp(1.0-vUv.y,0.0,1.0),1.6)*smoothstep(0.0,0.03,vUv.y);gl_FragColor=vec4(color*along*pow(clamp(vRim,0.0,1.0),2.5)*vNear*strength,1.0);}"},am={vertex:`attribute float phase;uniform float time,pointScale;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=0.22+0.78*pow(max(0.0,sin(time*1.9+phase)),10.0);gl_PointSize=clamp(1.5*pointScale/max(1.0,-mv.z),2.0,14.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(3.2,0.35,0.25)*pow(1.0-d,2.0)*vA,1.0);}"};function om(s,e={}){let t=[],n=[],i=new yt;i.name="city-atmosphere",s.add(i);let r=(P,B)=>(t.push(P),n.push(B),new ot(P,B)),a={value:0},o={value:900},l={value:0},c={value:0},h={value:.25},d={value:0},u={value:5},p=new Ce(4860440),x=(e.sunDirection||new k(-70,145,85)).clone().normalize(),v=new Ce(330010),m=new Ce(1323596),f=new Ce(8011050),y=new Ce(s.fog?.color||528926),T=(P,B,K,ee={})=>new wt({uniforms:K,vertexShader:P,fragmentShader:B,...ee}),g={transparent:!0,depthWrite:!1,blending:tn},_=r(new Ii(1500,48,24),T(sd,Wy,{time:a,day:l,dusk:c,cloud:h,flash:d,octaves:u,glow:{value:p},zenith:{value:v},horizon:{value:m},haze:{value:y},warm:{value:f},sunDir:{value:x}},{side:xn,depthWrite:!1,fog:!1}));_.frustumCulled=!1,_.renderOrder=-10,i.add(_);let S=1400,A=new Float32Array(S*3),b=new Float32Array(S),C=new Float32Array(S),w=new Float32Array(S),I=12345,M=()=>(I=Math.imul(I,1664525)+1013904223>>>0)/4294967296;for(let P=0;P<S;P++){let B=M()*Math.PI*2,K=.03+M()*.97,ee=Math.sqrt(1-K*K);A.set([Math.cos(B)*ee*1450,K*1450,Math.sin(B)*ee*1450],P*3),b[P]=M()*Math.PI*2,C[P]=.4+M()*1.6,w[P]=8e-4+M()*M()*.0022}let N=new Mt;N.setAttribute("position",new Tt(A,3)),N.setAttribute("phase",new Tt(b,1)),N.setAttribute("speed",new Tt(C,1)),N.setAttribute("size",new Tt(w,1));let D=T(em.vertex,em.fragment,{time:a,pointScale:o,cloud:h},{...g,fog:!1}),U=new Tn(N,D);U.frustumCulled=!1,U.renderOrder=-9,i.add(U),t.push(N),n.push(D);let H=r(new un(2800,2800),T(tm.vertex,tm.fragment,{time:a,day:l,glow:{value:p},island:{value:new Nt(0,-7,85,87)},deep:{value:new Ce(398368)},shallow:{value:new Ce(930640)},sky:{value:m.clone().multiplyScalar(.7)},sunDir:{value:x},sunColor:{value:new Ce(14676223)},fogColor:{value:new Ce},fogDensity:{value:0},fogNear:{value:1},fogFar:{value:1e3}},{fog:!0}));H.rotation.x=-Math.PI/2,H.position.y=-3.2,i.add(H);let L=r(new un(1900,1900),T(sd,qy,{time:a,strength:{value:.55},color:{value:new Ce(1717320)},island:{value:new Nt(0,-7,85,87)}},{transparent:!0,depthWrite:!1,fog:!1}));L.rotation.x=-Math.PI/2,L.position.y=-2.4,i.add(L);let W=500,z=new Float32Array(W*3),E=new Float32Array(W*3);for(let P=0;P<W;P++)z.set([(M()-.5)*180,2+M()*M()*75,-95+M()*170],P*3),E.set([M(),M(),M()],P*3);let J=new Mt;J.setAttribute("position",new Tt(z,3)),J.setAttribute("seed",new Tt(E,3));let q=T(nm.vertex,nm.fragment,{time:a,pointScale:o,color:{value:new Ce(10475775)},strength:{value:.32}},g),te=new Tn(J,q);te.frustumCulled=!1,i.add(te),t.push(J),n.push(q);let Le=new yt;Le.position.set(0,86.5,-12),i.add(Le);let Fe=T(sd,Xy,{strength:{value:.45},color:{value:new Ce(9430783)}},{...g,side:Yt}),Ne=new un(100,5.2);Ne.translate(50,0,0),t.push(Ne),n.push(Fe);let se=new ot(Ne,Fe),oe=new ot(Ne,Fe);oe.rotation.x=Math.PI/2;let Pe=new yt;Pe.add(se,oe),Pe.rotation.z=-.07,Le.add(Pe);let ge=new qt({color:12579583,toneMapped:!1}),xe=r(new Ii(.9,16,12),ge);Le.add(xe);let $=(e.lamps||[]).slice(0,64),ue=T(`varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 p=vec4(position,1.0);
#ifdef USE_INSTANCING
p=instanceMatrix*p;
#endif
vec4 mv=modelViewMatrix*p;vView=-mv.xyz;gl_Position=projectionMatrix*mv;}`,jy,{color:{value:new Ce(16767392)},strength:{value:.3}},{...g,side:Yt}),ce=new Gs(2.7,6.5,18,1,!0),ie=new En(ce,ue,Math.max(1,$.length));t.push(ce),n.push(ue);let de=new Xt;$.forEach((P,B)=>{{let K=(P.y||0)+6.5,ee=K-pt.pavement;de.position.set(P.x+1.55*Math.cos(P.angle||0),pt.pavement+ee/2,P.z-1.55*Math.sin(P.angle||0)),de.scale.set(1,ee/6.5,1),de.updateMatrix(),de.scale.set(1,1,1)}ie.setMatrixAt(B,de.matrix)}),ie.count=$.length,ie.instanceMatrix.needsUpdate=!0,i.add(ie);let ve=T(`varying vec2 vUv;void main(){vUv=uv;vec4 p=vec4(position,1.0);
#ifdef USE_INSTANCING
p=instanceMatrix*p;
#endif
gl_Position=projectionMatrix*modelViewMatrix*p;}`,Ky,{color:{value:new Ce(16767392)},strength:{value:.3}},{...g,polygonOffset:!0,polygonOffsetFactor:-2,polygonOffsetUnits:-2}),We=new un(1,1);We.rotateX(-Math.PI/2);let Ve=new En(We,ve,Math.max(1,$.length));Ve.name="lamp-pools",Ve.renderOrder=1,Ve.frustumCulled=!1,t.push(We),n.push(ve),$.forEach((P,B)=>{de.position.set(P.x+1.55*Math.cos(P.angle||0),pt.pavement+.02,P.z-1.55*Math.sin(P.angle||0)),de.scale.set(6.8,1,6.8),de.updateMatrix(),Ve.setMatrixAt(B,de.matrix)}),de.scale.set(1,1,1),Ve.count=$.length,Ve.instanceMatrix.needsUpdate=!0,i.add(Ve);let lt=[],nt=[],ct=[],Q=[],Pt=[],ft=360,O=P=>Math.max(0,.45+.3*Math.sin(P*5+1.3)+.2*Math.sin(P*13+.4)+.12*Math.sin(P*31+2.1))*(.35+.65*Math.max(0,Math.sin(P*2+.6)));for(let P=0;P<=ft;P++){let B=Math.PI*.8+Math.PI*1.45*P/ft,K=1180+40*Math.sin(B*7),ee=6+74*O(B);if(lt.push(Math.cos(B)*K,-3.5,Math.sin(B)*K,Math.cos(B)*K,ee,Math.sin(B)*K),nt.push(0,ee/80),P<ft){let X=P*2;ct.push(X,X+1,X+2,X+1,X+3,X+2)}}for(let P=0;P<90;P++){let B=Math.PI*.82+Math.PI*1.41*M(),K=1170+40*Math.sin(B*7);O(B)<.08||(Q.push(Math.cos(B)*K,1+M()*M()*28,Math.sin(B)*K),Pt.push(M()*20))}let R=new Mt;R.setAttribute("position",new xt(lt,3)),R.setAttribute("lift",new xt(nt,1)),R.setIndex(ct);let re={value:.8},le=r(R,T(im.vertex,im.fragment,{haze:{value:y},shade:re},{fog:!1,side:Yt}));le.frustumCulled=!1,le.renderOrder=-8,i.add(le);let be=new Mt;be.setAttribute("position",new xt(Q,3)),be.setAttribute("phase",new xt(Pt,1));let De={value:0},Ue=T(sm.vertex,sm.fragment,{time:a,pointScale:o,strength:De},{...g,fog:!1}),V=new Tn(be,Ue);V.frustumCulled=!1,V.renderOrder=-7,i.add(V),t.push(be),n.push(Ue);let Y=T(rm.vertex,rm.fragment,{color:{value:new Ce(11128063)},strength:{value:0}},{...g,side:Yt,fog:!1}),he=new gs(11,.7,280,20,1,!0);he.translate(0,140,0),t.push(he),n.push(Y);let _e=[[-72,-168,0],[64,-186,2.1],[4,-232,4.2]].map(([P,B,K])=>{let ee=new ot(he,Y);return ee.position.set(P,-3,B),ee.rotation.order="YXZ",ee.userData.offset=K,ee.frustumCulled=!1,i.add(ee),ee}),me=96,ae=new Float32Array(me*3),Se=new Float32Array(me);for(let P=0;P<me;P++)Se[P]=M()*Math.PI*2;let Me=new Mt;Me.setAttribute("position",new Tt(ae,3)),Me.setAttribute("phase",new Tt(Se,1)),Me.setDrawRange(0,0);let Ie=T(am.vertex,am.fragment,{time:a,pointScale:o},{...g,fog:!1}),j=new Tn(Me,Ie);j.frustumCulled=!1,i.add(j),t.push(Me),n.push(Ie);let Te=$p(a),we=Te.pass,Oe=!1,ke="high",Ae=0,Ye=1,Qe=()=>{ie.visible=Ve.visible=ke!=="low"&&Ye>.01};return{post:we,sea:H,setLighting(P,B,K){x.copy(K).normalize(),l.value=P,c.value=Math.min(1,B)*(1-P),v.set(330010).lerp(new Ce(3108776),P).lerp(new Ce(1909062),c.value*.8),m.set(1323596).lerp(new Ce(11126484),P).lerp(new Ce(12609598),c.value*.85),f.set(B>.1?16747077:8011050),ue.uniforms.color.value.copy(Hy).lerp(Gy,c.value),ve.uniforms.color.value.copy(ue.uniforms.color.value),p.set(4860440).lerp(new Ce(7027234),c.value),y.copy(s.fog.color),U.visible=P<.35,H.material.uniforms.sky.value.copy(m).lerp(v,c.value*.65).multiplyScalar(.7),H.material.uniforms.deep.value.set(398368).lerp(new Ce(669270),P),H.material.uniforms.shallow.value.set(930640).lerp(new Ce(1929112),P),L.material.uniforms.strength.value=.55*(1-P*.85),re.value=.82-.22*(1-P)-.2*c.value,De.value=Math.max(0,1-P*1.6)*(1-c.value*.5),H.material.uniforms.sunColor.value.set(14676223).lerp(new Ce(16752736),c.value),Ye=1-It.smoothstep(P,.12,.55),Qe()},setWeather(P){h.value=P==="rain"?.95:P==="fog"?.7:.25},setFlash(P){d.value=Math.max(0,Math.min(1,P))},setCinematic(P){Te.setCinematic(P)},setAviation(P){let B=Math.min(me,P.length);for(let K=0;K<B;K++)ae.set([P[K].x,P[K].y,P[K].z],K*3);Me.attributes.position.needsUpdate=!0,Me.setDrawRange(0,B)},setTier(P){ke=P;let B=ke!=="low";L.visible=B,te.visible=B,Qe(),_e.forEach(K=>{K.visible=B}),u.value=ke==="low"?3:ke==="medium"?4:5,we.enabled=ke==="high"||ke==="ultra"},setBusy(P){Oe=!!P},setPointScale(P){o.value=P},update(P,B,K,ee){let X=ee?Math.min(.1,Math.max(0,P)):0;a.value+=X,s.fog&&(H.material.uniforms.fogColor.value.copy(s.fog.color),s.fog.isFogExp2&&(H.material.uniforms.fogDensity.value=s.fog.density)),H.position.x=K.position.x,H.position.z=K.position.z;let Z=Oe?1.15:.32,Ee=Le.userData.rate??Z;Le.userData.rate=Ee+(Z-Ee)*Math.min(1,X*1.5),Pe.rotation.y+=X*Le.userData.rate,Fe.uniforms.strength.value=Oe?.85:.45,xe.scale.setScalar(1+Math.sin(a.value*(Oe?6:2.2))*.18);let Be=Ye*(1+.035*Math.sin(a.value*1.6)+.015*Math.sin(a.value*4.1+1.2));ue.uniforms.strength.value=.3*Be,ve.uniforms.strength.value=.6*Be,Ae+=X;let st=1-l.value;Y.uniforms.strength.value=st*st*.075*(1-c.value*.7)*(1-h.value*.45),_e.forEach((Je,kt)=>{let Bt=Je.userData.offset;Je.rotation.y=Math.PI+Math.sin(Ae*(.11+kt*.025)+Bt)*.95,Je.rotation.x=.22+.14*Math.sin(Ae*.13+Bt*1.7)}),we.enabled&&Te.update(X,K,x,{day:l.value,dusk:c.value,cloud:h.value,animated:ee})},stats:()=>({tier:ke,busy:Oe,stars:S,dust:W,lamps:$.length,lampLevel:+Ye.toFixed(2),post:we.enabled,coast:ct.length/6,shoreLights:Pt.length,clouds:+h.value.toFixed(2),searchlights:_e.length,aviation:Me.drawRange.count,cinema:Te.stats()}),dispose(){i.removeFromParent(),ie.dispose(),Ve.dispose(),t.forEach(P=>P.dispose()),n.forEach(P=>P.dispose()),Te.dispose()}}}var xo=3,Yy=3,Jy=192,lm=[9365742,7730385,16763274,12166399];function cm(s,e={}){let t=e.anchors||[],n=new yt;n.name="city-flow",s.add(n);let i=[];for(let _=0;_<t.length;_++)for(let S of[1,3]){let A=t[_],b=t[(_+S)%t.length];!A||!b||A.id===b.id||i.push({from:A,to:b,lift:22+(_+S)%4*11})}if(i.length<2)return n.removeFromParent(),{group:n,update(){},setTier(){},stats:()=>({routes:0,points:0,visible:!1}),dispose(){}};let r=Math.min(Jy/xo,i.length*Yy),a=r*xo,o=new Float32Array(a*3),l=new Float32Array(a),c=new Float32Array(a),h=new Float32Array(a*3),d=new Mt;d.setAttribute("position",new Tt(o,3)),d.setAttribute("alpha",new Tt(l,1)),d.setAttribute("size",new Tt(c,1)),d.setAttribute("tint",new Tt(h,3));let u=new wt({transparent:!0,depthWrite:!1,blending:tn,fog:!1,uniforms:{pointScale:{value:900},strength:{value:.4}},vertexShader:`attribute float alpha,size;attribute vec3 tint;uniform float pointScale;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=clamp(size*pointScale/max(1.0,-mv.z),1.0,12.0);vAlpha=alpha;vTint=tint;}`,fragmentShader:`uniform float strength;varying float vAlpha;varying vec3 vTint;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;
gl_FragColor=vec4(vTint*pow(1.0-d,2.4)*vAlpha*strength,1.0);}`}),p=new Tn(d,u);p.frustumCulled=!1,p.renderOrder=2,n.add(p);let x=[],v=new Ce;for(let _=0;_<r;_++){let S=i[_%i.length];v.setHex(lm[_%lm.length]),x.push({route:S,t:_*.3819%1,speed:.045+_*7%9/190,tint:[v.r,v.g,v.b],size:.42+_*3%5*.08})}function m(_,S,A){let b=_.from,C=_.to,w=1-S,I=(b.x+C.x)/2,M=(b.z+C.z)/2,N=Math.max(b.height,C.height)+_.lift,D=b.height+4,U=C.height+4;return A.x=w*w*b.x+2*w*S*I+S*S*C.x,A.y=w*w*D+2*w*S*N+S*S*U,A.z=w*w*b.z+2*w*S*M+S*S*C.z,A}let f={x:0,y:0,z:0},y="high",T=.4;function g(_,S,A){if(!p.visible)return;let b=S?Math.min(_,.05):0;for(let C=0;C<x.length;C++){let w=x[C];w.t=(w.t+b*w.speed)%1;for(let I=0;I<xo;I++){let M=C*xo+I,N=(w.t-I*.012+1)%1;m(w.route,N,f),o[M*3]=f.x,o[M*3+1]=f.y,o[M*3+2]=f.z;let D=Math.min(1,Math.min(N,1-N)*7);l[M]=D*(1-I/xo)*.9,c[M]=w.size*(1-I*.22),h[M*3]=w.tint[0],h[M*3+1]=w.tint[1],h[M*3+2]=w.tint[2]}}d.attributes.position.needsUpdate=!0,d.attributes.alpha.needsUpdate=!0,d.attributes.size.needsUpdate=!0,d.attributes.tint.needsUpdate=!0,T+=(.18+A*.82-T)*Math.min(1,Math.max(0,_)*2),u.uniforms.strength.value=T}return{group:n,update:g,setPointScale(_){u.uniforms.pointScale.value=_},setTier(_){y=_,p.visible=y!=="low",n.visible=y!=="low"},stats:()=>({routes:i.length,points:a,visible:p.visible,tier:y}),dispose(){n.removeFromParent(),d.dispose(),u.dispose()}}}var hm=[{points:[[-62,34,-62],[8,44,-74],[62,38,-22],[56,47,42],[-8,41,62],[-64,36,12]],speed:9.5},{points:[[30,56,-12],[0,62,18],[-30,58,-12],[0,66,-42]],speed:8},{points:[[42,29,-56],[-28,33,-42],[-52,27,20],[18,31,52],[60,33,8]],speed:11}];function um(s,e={}){let t=new yt;t.name="city-drones",s.add(t);let n=[],i=[],r=[],a=new k,o=new k,l=new k,c=new Ii(.16,8,6);n.push(c);let h=g=>{let _=new qt({color:g,toneMapped:!1});return i.push(_),_},d=h(16730684),u=h(5046154),p=h(16777215);[...hm,...hm].forEach((g,_)=>{let S=new Dr(g.points.map(w=>new k(w[0],w[1]+(_>=3?8:0),w[2])),!0,"centripetal",.6),A=new yt;A.name="city-drone-"+(_+1),A.visible=!1;let b=new yt;A.add(b);let C=[new ot(c,d),new ot(c,u),new ot(c,p)];C[0].position.set(-2.6,.8,0),C[1].position.set(2.6,.8,0),C[2].position.set(0,2.1,-.4),A.add(...C),t.add(A),r.push({root:A,body:b,lights:C,curve:S,length:S.getLength(),speed:g.speed,t:_*.37%1,rotors:[],roll:0,dir:1,waiting:0})});let x=!0,v="high",m=!1,f=new Map,y=g=>{if(!f.has(g)){let _=g.clone();e.surfaces?.apply(_,"object",g.name),f.set(g,_)}return f.get(g)};function T(g,_){let{curve:S,root:A}=g,b=g.t;if(g.t=(g.t+_*g.speed*g.dir/g.length+1)%1,S.getPointAt(g.t,A.position),S.getTangentAt(g.t,a),a.multiplyScalar(g.dir),g.collider&&e.traffic){let I=g.collider,M=e.traffic;S.getPointAt((g.t+g.dir*20/g.length+1)%1,l),g.avoidTime=Math.max(0,(g.avoidTime||0)-_);let N=Math.max(A.position.y,M.ceiling(I,A.position.x,A.position.z),M.ceiling(I,l.x,l.z),g.avoidTime?g.avoidHeight:0);A.position.y=I.y+It.clamp(N-I.y,-_*5,_*7);let D=Math.atan2(a.x,a.z),U=I.heading+Math.atan2(Math.sin(D-I.heading),Math.cos(D-I.heading))*Math.min(1,_*3),H={...A.position,heading:U};if(g.obstruction=M.obstruction(I,I,H),g.obstruction){let L=M.body(g.obstruction),W=g.obstruction.startsWith("drone-")&&(Math.abs(I.y-L.y)>1?I.y<L.y:I.id<L.id);W||(g.avoidHeight=Math.min(170,I.y+8),g.avoidTime=8),H={...I,y:W?I.y:Math.min(170,I.y+_*5)},g.t=b}M.propose(I,H,(L,W)=>{L?g.waiting=0:(g.t=b,g.waiting+=_,g.waiting>2.5&&(g.dir=-g.dir,g.waiting=0)),A.position.set(W.x,W.y,W.z),A.rotation.set(0,W.heading,0)})}S.getTangentAt((g.t+.015)%1,o),g.collider||A.rotation.set(0,Math.atan2(a.x,a.z),0);let C=Math.atan2(o.x,o.z)-Math.atan2(a.x,a.z),w=Math.atan2(Math.sin(C),Math.cos(C));g.roll+=(It.clamp(w*12,-.55,.55)-g.roll)*(_>0?Math.min(1,_*3):1),g.body.rotation.z=g.roll}return{setTier(g){v=g},setTemplate(g){if(!g)return;m=!0;let _=f;f=new Map;for(let[S,A]of r.entries()){A.body.clear(),A.rotors.length=0;let b=g.clone(!0);b.scale.setScalar(1.9),b.updateMatrixWorld(!0);let C=new hn().setFromObject(b);if(C.expandByScalar(.5),C.min.y-=1,C.max.y+=1,b.traverse(w=>{w.isMesh&&(w.castShadow=!1,w.receiveShadow=!1,w.material=Array.isArray(w.material)?w.material.map(y):y(w.material)),/^rotor_/.test(w.name)&&A.rotors.push(w)}),A.body.add(b),A.root.visible=S<(v==="low"?2:v==="medium"?3:6),e.traffic&&!A.collider){T(A,0);let w=ns({min:C.min.toArray(),max:C.max.toArray()});A.root.position.y=Math.max(A.root.position.y,e.traffic.ceiling(w,A.root.position.x,A.root.position.z)),A.collider=e.traffic.register("drone-"+S,w,{...A.root.position,heading:A.root.rotation.y},0),A.collider||(A.root.visible=!1)}}_.forEach(S=>S.dispose())},update(g,_,S){x=S;let A=S?Math.min(.1,Math.max(0,g)):0;r.forEach((b,C)=>{if(b.root.visible=m&&C<(v==="low"?2:v==="medium"?3:6)&&(!e.traffic||!!b.collider),b.collider&&(b.root.visible&&!b.collider.enabled&&!e.traffic.relocate(b.collider,b.collider)&&(b.root.visible=!1),b.collider.enabled=b.root.visible),!b.root.visible)return;T(b,A),b.rotors.forEach((I,M)=>{I.rotation.y+=A*(M%2?-46:46)});let w=Math.sin(_*5+C*1.7);b.lights[0].visible=b.lights[1].visible=!S||w>-.2,b.lights[2].visible=!S||w>.93,b.lights[2].scale.setScalar(S?1.6:1)})},stats:()=>({drones:r.filter(g=>g.root.visible).length,animated:x,positions:r.map(g=>g.root.position.toArray().map(_=>Math.round(_))),obstructions:r.map(g=>g.obstruction)}),dispose(){t.removeFromParent(),n.forEach(g=>g.dispose()),i.forEach(g=>g.dispose()),f.forEach(g=>g.dispose()),r.forEach(g=>{g.collider&&e.traffic.remove(g.collider.id)}),r.length=0}}}var Ts=[{id:"repair-bay",district:"infra",x:-28,z:-62,role:"technician"},{id:"parcel-sorter",district:"missions",x:35,z:49,role:"courier"},{id:"relay-mast",district:"integrations",x:57,z:-20,role:"archivist"},{id:"kinetic-fountain",district:"graph",x:-28,z:47,role:"archivist"},{id:"glass-garden",district:"graph",x:40,z:-68,role:"technician"},{id:"meeting-charge",district:"operations",x:8,z:46,role:"courier"}],dm=[{id:"data-monument",x:0,z:24,y:0,angle:0,clip:"spin"},{id:"neon-gate",x:-18,z:0,y:pt.pavement,angle:0,clip:"pulse"},{id:"sky-pod",x:56,z:47,y:0,angle:0,clip:"scan"},{id:"tidal-turbine",x:0,z:96,y:-3.2,angle:Math.PI,clip:"spin"}],Zy=2*Ht.halfWidth;function Qy(s){return Vp().some(e=>e.axis==="x"?Math.abs(s.x-e.at)<=Gt.opening&&Math.abs(s.z-e.line)<=Ht.halfWidth:Math.abs(s.z-e.at)<=Gt.opening&&Math.abs(s.x-e.line)<=Ht.halfWidth)}function fm(s,e){let t=Xp(),n=t.nodes.map(o=>({x:o.x,z:o.z,y:e(o.x,o.z),heading:0})),i=new Map,r=-1;function a(o,l){r!==s.revision()&&(i.clear(),r=s.revision());let c=o.circles.map(g=>[g.x,g.z,g.r].join(",")).join(";")+":"+o.maxY;if(!i.has(c)){let g=new Map,_=[];n.forEach((A,b)=>{s.clear(o,A,A,!1)&&(g.set(b,_.length),_.push(A))});let S=_.map(()=>[]);for(let A of t.links){let b=g.get(A.a),C=g.get(A.b);b===void 0||C===void 0||s.clear(o,_[b],_[C],!1)&&(S[b].push([C,A.length]),S[C].push([b,A.length]))}i.set(c,{nodes:_,edges:S})}let h=i.get(c),d=[...h.nodes,{x:o.x,y:o.y,z:o.z,heading:o.heading},{...l,heading:0}],u=d.length-2,p=d.length-1,x=Qy(d[u]),v=h.edges.map(g=>[...g]);v.push([],[]);for(let g of[u,p])for(let _=0;_<g;_++){let S=d[g],A=d[_],b=Math.hypot(S.x-A.x,S.z-A.z);(b<Ju||_===u)&&$u(S,A,g===u&&x&&b<Zy)&&s.clear(o,S,A,!1)&&(v[g].push([_,b]),v[_].push([g,b]))}let m=d.map(()=>1/0),f=d.map(()=>-1),y=new Set(d.map((g,_)=>_));for(m[u]=0;y.size;){let g=-1;for(let _ of y)(g<0||m[_]<m[g])&&(g=_);if(!Number.isFinite(m[g])||g===p)break;y.delete(g);for(let[_,S]of v[g])m[g]+S<m[_]&&(m[_]=m[g]+S,f[_]=g)}if(!Number.isFinite(m[p]))return[];let T=[];for(let g=p;g!==u;g=f[g]){if(g<0)return[];T.unshift(d[g])}return T}return{route:a,points:n}}function pm(s){let e=new yt;e.name="city-conversations",s.add(e);let t=new Jn(1,.025,5,40,Math.PI*1.45),n=new xs(.92,1,48),i=new k(0,0,1),r=new k,a=[],o=0;for(let l=0;l<8;l++){let c=[];for(let h=0;h<4;h++){let d=new qt({color:8445392,transparent:!0,opacity:0,blending:tn,depthWrite:!1,side:Yt,toneMapped:!1}),u=new ot(h===3?n:t,d);u.visible=!1,e.add(u),c.push(u)}a.push({meshes:c,age:10,from:null,to:null})}return{send(l,c,h="ambient"){let d=a.find(u=>u.age>=2.4);return d?(Object.assign(d,{from:l,to:c,age:0,source:h}),o++,d.meshes.forEach(u=>u.material.color.setHex(h==="live"?16105851:8445392)),!0):!1},update(l,c=!0){for(let h of a){if((!c||h.from?.enabled===!1||h.to?.enabled===!1)&&(h.age=10),h.age+=l,h.meshes.forEach(m=>{m.visible=!1}),h.age>=2.4)continue;let d=h.from,u=h.to,p=u.x-d.x,x=u.z-d.z;r.set(p,u.y+Math.min(2,u.maxY*.7)-(d.y+Math.min(2,d.maxY*.7)),x).normalize();for(let m=0;m<3;m++){let f=(h.age-m*.18)/1.55;if(f<0||f>1)continue;let y=h.meshes[m];y.visible=!0,y.position.set(d.x+p*f,It.lerp(d.y+Math.min(2,d.maxY*.7),u.y+Math.min(2,u.maxY*.7),f)+Math.sin(f*Math.PI)*.65,d.z+x*f),y.quaternion.setFromUnitVectors(i,r),y.rotateZ(m*.65),y.scale.setScalar(.25+Math.sin(f*Math.PI)*.65),y.material.opacity=Math.sin(Math.PI*f)*.75}let v=(h.age-1.55)/.85;if(v>0){let m=h.meshes[3];m.visible=!0,m.position.set(u.x,u.y+Math.min(2,u.maxY*.7),u.z),m.quaternion.setFromUnitVectors(i,r),m.scale.setScalar(1.25*(1-v)+.12),m.material.opacity=Math.sin(v*Math.PI)*.85}}},clear(){a.forEach(l=>{l.age=10,l.meshes.forEach(c=>{c.visible=!1})})},stats:()=>({sent:o,active:a.filter(l=>l.age<2.4).length,sources:a.filter(l=>l.age<2.4).map(l=>l.source)}),dispose(){e.removeFromParent(),t.dispose(),n.dispose(),a.forEach(l=>l.meshes.forEach(c=>c.material.dispose()))}}}var $y=[["leg_l","shin_l","foot_l"],["leg_r","shin_r","foot_r"]],mm=[[0,-.13,-.085],[0,-.13,.27]];function As(s,e,t,n=14,i=.6){if(!(t>0)||!Number.isFinite(s.y)||Math.abs(e-s.y)>i)return s.y=e,s.v=0,e;let r=s.y-e,a=Math.exp(-n*t),o=(s.v+n*r)*t;return s.v=(s.v-n*o)*a,s.y=e+(r+o)*a,s.y}function eM(s,e=.025,t=.07){let n=Math.min(1,Math.max(0,(s-e)/(t-e)));return 1-n*n*(3-2*n)}var bo=new k,vo=new k,ad=new k,tM=new k,od=new k,ld=new k,Wc={hip:new $t,knee:new $t};function nM(s,e,t,n,i){bo.subVectors(e,s),vo.subVectors(t,e);let r=bo.length(),a=vo.length();if(r<1e-6||a<1e-6||i.lengthSq()<1e-12)return null;ad.copy(i).normalize();let o=Math.min(r+a-1e-4,Math.max(Math.abs(r-a)+1e-4,n.distanceTo(s))),l=Math.atan2(tM.crossVectors(bo,vo).dot(ad),bo.dot(vo)),c=Math.acos(Math.max(-1,Math.min(1,(r*r+a*a-o*o)/(2*r*a)))),h=(Math.PI-c)*(l<0?-1:1);return Wc.knee.setFromAxisAngle(ad,h-l),od.copy(vo).applyQuaternion(Wc.knee).add(bo),ld.subVectors(n,s),od.lengthSq()<1e-12||ld.lengthSq()<1e-12?null:(Wc.hip.setFromUnitVectors(od.normalize(),ld.normalize()),Wc)}var _o=new k,yo=new k,gm=new k,xm=new k,cd=new k,hd=new k,bm=new k,ud=new $t,vm=new $t,_m=new $t;function Mm(s){_o.fromArray(mm[0]).applyMatrix4(s.matrixWorld),yo.fromArray(mm[1]).applyMatrix4(s.matrixWorld)}function wm(s,e){return Mm(s),Math.min(_o.y,yo.y)-e.matrixWorld.elements[13]}function iM(s,e){return Mm(s),Math.max(e(_o.x,_o.z)-_o.y,e(yo.x,yo.z)-yo.y)}function ym(s,e){s.parent.getWorldQuaternion(ud),s.quaternion.premultiply(ud).premultiply(e).premultiply(ud.invert())}function Sm(s){let e=s.getObjectByName("structure");if(!e)return null;let t=[];for(let n of $y){let[i,r,a]=n.map(o=>s.getObjectByName(o));if(!i||!r||!a||r.parent!==i||a.parent!==r)return null;t.push({leg:i,shin:r,foot:a})}s.updateMatrixWorld(!0);for(let n of t)if(Math.abs(wm(n.foot,s))>.04)return null;return{structure:e,legs:t}}function Em(s,e,t,{maxDrop:n=.25,maxRaise:i=.35}={}){e.updateMatrixWorld(!0);let r=s.legs.map(l=>({l,lift:wm(l.foot,e),pen:iM(l.foot,t)})),a=0;for(let l of r)a=Math.max(a,eM(l.lift)*Math.max(0,-l.pen));a=Math.min(a,n),a<1e-4&&(a=0),a&&(e.position.y-=a,e.updateMatrixWorld(!0));let o=[];for(let l of r){let c=Math.min(i,l.pen+a);if(o.push(Math.max(0,c)),c<=5e-4)continue;let{leg:h,shin:d,foot:u}=l.l;h.getWorldPosition(gm),d.getWorldPosition(xm),u.getWorldPosition(cd),hd.copy(cd),hd.y+=c,bm.setFromMatrixColumn(h.matrixWorld,0);let p=nM(gm,xm,cd,hd,bm);p&&(u.getWorldQuaternion(_m),ym(d,p.knee),ym(h,p.hip),h.updateMatrixWorld(!0),d.getWorldQuaternion(vm),u.quaternion.copy(vm.invert().multiply(_m)),u.updateMatrixWorld(!0))}return{drop:a,raised:o}}var Tm=(s,e,t)=>s+Math.atan2(Math.sin(e-s),Math.cos(e-s))*Math.min(1,t*5),Am=(s,e,t)=>{let n=s.play?.(e);n&&(n.timeScale=t/1.65)};function Rm(s,{traffic:e,camera:t,floor:n,onSelect:i,onDemonstrate:r,active:a,onGait:o,shadows:l}){let c=[],h=new Map,d=[],u=[],p=fm(e,n),x=pm(s),v=0,m=null,f=!1,y="high",T=!1,g=0,_="",S=()=>y==="low"?3:y==="medium"?10:19;function A(L){L.slot&&(L.slot.owner=null,L.slot=null),L.path=[],L.goal=null}function b(L){return L?{id:L.id,role:L.role,state:L.state,source:L.source||"ambient",guided:!!L.guide}:null}function C(L){m=L?.id||null,_=L?.state||"",i?.(b(L))}function w(L,W,z){let E="resident-"+W,J=ns(z,1.15),q=["courier","technician","archivist"][W%3],te=null;J.circles=[{x:0,z:0,r:J.reach}];let Fe=[...d.filter(se=>se.role===q).map(se=>({x:se.x,z:se.z+2.2,y:n(se.x,se.z+2.2),heading:Math.PI})),...p.points.map((se,oe)=>p.points[(W*7+oe)%p.points.length])];for(let se of Fe)if(te=e.register(E,J,se,1),te)break;if(!te)return L.node.visible=!1,null;let Ne={...L,id:E,index:W,body:te,role:q,state:"idle",path:[],until:W*.3,trip:W,cooldown:8+W*.7,source:"ambient",carrying:!1,parcels:[],soundX:te.x,soundZ:te.z};return L.node.traverse(se=>{se.isMesh&&/parcel/.test(se.name)&&(Ne.parcels.push(se),se.visible=!1)}),c.push(Ne),L.node.position.set(te.x,te.y,te.z),Ne.height={y:te.y,v:0},Ne.pitch={y:0,v:0},L.node.rotation.order="YXZ",Ne.rig=Sm(L.node),Ne.shadow=l?.claim()||null,Ne}function I(L,W,z,E){let J={id:L,body:W,node:z,pause:E,role:"patrol",state:"patrol",cooldown:7+c.length,path:[],soundX:W.x,soundZ:W.z};return c.push(J),J}function M(L){A(L),L.source="ambient",L.state="idle",L.until=v+2;let W=d.filter(z=>!z.owner&&(L.role==="courier"?z.place===(L.carrying?"meeting-charge":"parcel-sorter"):z.role===L.role||z.place==="meeting-charge"));for(let z=0;z<W.length;z++){let E=W[(L.trip+z)%W.length],J=p.route(L.body,E);if(J.length){E.owner=L.id,L.slot=E,L.path=J,L.goal=E,L.trip++,L.state=L.carrying?"carry":"walk";return}}for(let z=0;z<8;z++){let E=p.points[(L.trip++*13+L.index)%p.points.length],J=p.route(L.body,E);if(J.length){L.path=J,L.state="walk";return}}}function N(L){for(let W of[L.a,L.b])A(W),W.meeting=null,W.cooldown=v+12+(W.index||0),W.pause?.(!1),W.state=W.pause?"patrol":"idle",W.until=v+1}function D(){u.forEach(N),u.length=0,x.clear(),m=null,i?.(null);for(let L of c)L.guide=null,L.pause?.(!1,L.body),L.trafficYield=!1,L.yieldTram=null,L.pause||(A(L),L.state="idle",L.until=v+1)}function U(L,W){if(T)return;W&&(v+=L);for(let E of c){let J=E.pause||E.index<S();if(J&&!E.body.enabled){if(!e.relocate(E.body,E.body)){E.node.visible=!1;continue}E.body.enabled=!0,E.node.position.set(E.body.x,E.body.y,E.body.z)}!J&&E.body.enabled&&(A(E),E.body.enabled=!1,E.meeting&&N(E.meeting)),E.node.visible=!!J,E.soundX==null&&(E.soundX=E.body.x,E.soundZ=E.body.z);let q=Math.hypot(E.body.x-E.soundX,E.body.z-E.soundZ);if(E.soundX=E.body.x,E.soundZ=E.body.z,o?.(E.id,E.body.x,E.body.y,E.body.z,q>.004,!!E.carrying,E.role==="patrol"?"glide":""),!J||!W)continue;let te=e.neighbors(E.body,22).sort((Le,Fe)=>Math.hypot(Le.x-E.body.x,Le.z-E.body.z)-Math.hypot(Fe.x-E.body.x,Fe.z-E.body.z)).find(Le=>{if(!Le.id.startsWith("tram-"))return!1;let Fe=E.body.x-Le.x,Ne=E.body.z-Le.z,se=Math.cos(Le.heading),oe=Math.sin(Le.heading);return Math.abs(Fe*se-Ne*oe)<E.body.reach+(E.trafficYield?4:2.3)&&Fe*oe+Ne*se>-8-E.body.reach&&Fe*oe+Ne*se<14});if(!te&&E.yieldTram){let Le=e.body(E.yieldTram),Fe=Le?E.body.x-Le.x:0,Ne=Le?E.body.z-Le.z:0;if(Le?.enabled&&Math.hypot(Fe,Ne)<26&&Fe*Math.sin(Le.heading)+Ne*Math.cos(Le.heading)>-Le.reach-E.body.reach-1){E.pause?.(!0),E.play?.("idle");continue}E.yieldTram=null}if(te){if(E.trafficYield=!0,E.yieldTram=te.id,E.meeting){let ie=E.meeting;N(ie),u.splice(u.indexOf(ie),1)}E.pause?.(!0);let Le=Math.cos(te.heading),Fe=Math.sin(te.heading),Ne=Math.sign((E.body.x-te.x)*Le-(E.body.z-te.z)*Fe)||1,se=(E.body.x-te.x)*Le-(E.body.z-te.z)*Fe,oe=Math.sign((E.body.x-te.x)*Fe+(E.body.z-te.z)*Le)||1,Pe=[[Le*Ne,-Fe*Ne,E.body.reach+4.5-Math.abs(se)],[-Le*Ne,Fe*Ne,E.body.reach+4.5+Math.abs(se)],[Fe*oe,Le*oe,1.2]],ge=Array.from({length:16},(ie,de)=>[Math.cos(de*Math.PI/8),Math.sin(de*Math.PI/8),2]),xe=([ie,de])=>ie*(E.body.x-te.x)+de*(E.body.z-te.z);Pe.push(...ge.filter(ie=>xe(ie)>.2).sort((ie,de)=>xe(de)-xe(ie)));let $=E.escape&&v<E.escape.until?E.escape:null,ue=$?Pe.filter(ie=>ie[0]===$.x&&ie[1]===$.z).concat(Pe.filter(ie=>!(ie[0]===$.x&&ie[1]===$.z))):Pe,ce=!1;for(let[ie,de,ve]of ue){let We={x:E.body.x+ie*ve,y:E.body.y,z:E.body.z+de*ve,heading:E.body.heading};if(!e.clear(E.body,E.body,We))continue;let Ve={...We,x:E.body.x+ie*L*1.8,z:E.body.z+de*L*1.8};Ve.y=E.pause?E.body.y:n(Ve.x,Ve.z),e.propose(E.body,Ve,(lt,nt)=>{E.node.position.set(nt.x,nt.y,nt.z),Am(E,"walk",1.8)}),E.escape={x:ie,z:de,until:v+.8},E.state="yield",E.cooldown=v+5,ce=!0;break}ce||(E.play?.("idle"),E.state="yield");continue}if(E.trafficYield&&(E.trafficYield=!1,E.pause?.(!1,E.body),E.state=E.pause?"patrol":E.path.length?"walk":"idle"),!E.meeting){if(E.pause){E.state==="greet"&&v>E.until&&(E.pause(!1),E.state="patrol");continue}if(E.guide&&Math.hypot(t.position.x-E.body.x,t.position.z-E.body.z)>9){E.play("idle");continue}if(E.path.length){let Le=E.path[0],Fe=Le.x-E.body.x,Ne=Le.z-E.body.z,se=Math.hypot(Fe,Ne),oe=E.guide?1.9:1.65;if(se<.18){E.path.shift();continue}let Pe=Math.min(se,oe*L),ge={x:Fe/se,z:Ne/se},xe=E.body.x+ge.x*Pe,$=E.body.z+ge.z*Pe,ue=Tm(E.body.heading,Math.atan2(Fe,Ne),L),ce={x:E.body.x+ge.x*1.8,y:n(xe,$),z:E.body.z+ge.z*1.8,heading:ue};if(e.clear(E.body,E.body,ce))E.detour=null;else{let de=[[ge.z,-ge.x],[-ge.z,ge.x]],ve=E.detour&&v<E.detour.until?de.findIndex(lt=>lt[0]===E.detour.x&&lt[1]===E.detour.z):-1,We=ve>=0?[de[ve],...de.filter((lt,nt)=>nt!==ve)]:de,Ve=!1;for(let[lt,nt]of We){let ct={x:E.body.x+lt*1.3,y:E.body.y,z:E.body.z+nt*1.3,heading:E.body.heading};if(!e.clear(E.body,E.body,ct))continue;let Q={x:ct.x+ge.x*1.8,y:n(ct.x,ct.z),z:ct.z+ge.z*1.8,heading:E.body.heading};if(e.clear(E.body,ct,Q)){xe=E.body.x+lt*Pe,$=E.body.z+nt*Pe,E.detour={x:lt,z:nt,until:v+1.2},Ve=!0;break}}Ve||(xe=E.body.x,$=E.body.z)}e.clear(E.body,E.body,{x:xe,y:n(xe,$),z:$,heading:ue},!1)||(ue=E.body.heading);let ie=Math.hypot(xe-E.body.x,$-E.body.z)>.001;e.propose(E.body,{x:xe,y:n(xe,$),z:$,heading:ue},(de,ve)=>{E.node.position.set(ve.x,ve.y,ve.z),E.node.rotation.y=ve.heading,!de||!ie?(E.wait=(E.wait||0)+L,E.play("idle"),E.state="yield"):(E.wait=0,E.state=E.guide?"guide":E.carrying?"carry":"walk",Am(E,E.carrying?"carry":"walk",oe)),E.wait>2.5&&(E.wait=0,A(E),E.state=E.guide?"guide":"idle",E.until=v+.5,E.guide&&(E.path=p.route(E.body,E.guide)))})}else if(E.state==="walk"||E.state==="carry"||E.state==="guide"||E.state==="yield"){if(E.guide&&Math.hypot(E.body.x-E.guide.x,E.body.z-E.guide.z)>.5){E.play("idle"),v>E.until&&(E.path=p.route(E.body,E.guide),E.until=v+2.5);continue}E.guide=null,E.state=E.slot?.place==="meeting-charge"?"charge":E.slot?"work":"idle",E.until=v+5+E.index%4,E.play(E.state==="work"?"work":"idle"),E.slot&&r?.(E.slot.place,5),g++}else v>E.until&&(E.role==="courier"&&E.slot&&(E.state==="work"||E.state==="charge")&&(E.carrying=E.slot.place==="parcel-sorter",E.parcels.forEach(Le=>{Le.visible=E.carrying})),M(E))}}if(!W){x.update(0,!1);return}for(let E of c){if(!E.body.enabled||E.trafficYield||E.meeting||E.guide||E.state==="greet"||v<E.cooldown)continue;let J=c.find(te=>te!==E&&te.body.enabled&&!te.trafficYield&&te.state!=="greet"&&!te.meeting&&!te.guide&&v>=te.cooldown&&Math.abs(E.body.y-te.body.y)<2&&Math.hypot(E.body.x-te.body.x,E.body.z-te.body.z)>2.8&&Math.hypot(E.body.x-te.body.x,E.body.z-te.body.z)<6);if(!J)continue;let q={a:E,b:J,at:v,sent:!1,replied:!1};u.push(q);for(let te of[E,J])te.meeting=q,te.state="exchange",te.pause?.(!0),te.play?.("greet")}for(let E=u.length-1;E>=0;E--){let J=u[E],q=v-J.at;if(q>4.8||!J.a.body.enabled||!J.b.body.enabled){N(J),u.splice(E,1);continue}for(let[te,Le]of[[J.a,J.b],[J.b,J.a]])e.propose(te.body,{...te.body,heading:Tm(te.body.heading,Math.atan2(Le.body.x-te.body.x,Le.body.z-te.body.z),L)},(Fe,Ne)=>{te.node.rotation.y=Ne.heading});q>.6&&!J.sent&&(x.send(J.a.body,J.b.body),J.sent=!0),q>2.4&&!J.replied&&(x.send(J.b.body,J.a.body),J.replied=!0)}x.update(L,!0);let z=c.find(E=>E.id===m);z&&z.state!==_&&(_=z.state,i?.({...b(z),refresh:!0}))}function H(L){if(T)return;let W=y!=="low";for(let z of c){if(!z.height)continue;if(!z.body.enabled||!z.node.visible){z.shadow?.hide();continue}let E=z.body,J=As(z.height,E.y,L),q=Math.sin(E.heading),te=Math.cos(E.heading),Le=n(E.x+q*.35,E.z+te*.35)-n(E.x-q*.35,E.z-te*.35),Fe=As(z.pitch,Math.max(-.08,Math.min(.08,Math.atan2(Le,.7)*.5)),L);z.node.position.set(E.x,J,E.z),z.node.rotation.y=E.heading,z.node.rotation.x=-Fe,W&&z.rig&&Em(z.rig,z.node,n),z.shadow?.set(E.x,n(E.x,E.z),E.z,.62,.5)}}return{add:w,patrol:I,update:U,suspend:D,settle:H,addPlace(L,W){if(!h.has(L.id)){h.set(L.id,L);for(let z of W.navigation.workpoints||[])d.push({id:L.id+":"+z.id,place:L.id,role:L.role,x:L.x+z.position[0],y:n(L.x,L.z),z:L.z+z.position[2],owner:null})}},nearby(){let L=[];for(let W of c)if(W.body.enabled){let z=Math.hypot(t.position.x-W.body.x,t.position.z-W.body.z);z<5&&Math.abs(t.position.y-W.body.y)<8&&L.push({kind:"resident",id:W.id,distance:z,x:W.body.x,z:W.body.z})}for(let W of h.values()){let z=Math.hypot(t.position.x-W.x,t.position.z-W.z);z<7&&L.push({kind:"demonstrate",id:W.id,distance:z,x:W.x,z:W.z})}return L},inspect(L){let W=c.find(z=>z.id===L);return C(W),b(W)},action(L,W){let z=c.find(E=>E.id===m);if(!z)return null;if(z.meeting){let E=z.meeting,J=u.indexOf(E);N(E),J>=0&&u.splice(J,1)}if(L==="greet"&&(A(z),z.guide=null,z.pause?.(!0),z.state="greet",z.until=v+3,z.cooldown=v+5,z.play?.("greet")),L==="guide"&&!z.pause&&W){A(z);for(let E of[0,1.5,3]){for(let J=0;J<(E?8:1)&&!z.path.length;J++){let q=W.x+Math.cos(J*Math.PI/4)*E,te=W.z+Math.sin(J*Math.PI/4)*E;z.path=p.route(z.body,{x:q,z:te,y:n(q,te)})}if(z.path.length)break}z.guide=z.path.length?z.path[z.path.length-1]:null,z.state=z.guide?"guide":"idle"}return L==="cancel"&&(A(z),z.guide=null,z.pause?.(!1),z.state=z.pause?"patrol":"idle",z.until=v+1),C(z),b(z)},demonstrate(L,W=!1){h.has(L)&&r?.(L,W?0:6)},setTier(L){y=L},setReplay(L){f!==L&&(f=L,D())},event(L){if(f||!a()||!["started","succeeded","failed","sanitized","progress"].includes(L.state)||!Number.isFinite(L.at)||Date.now()-L.at>2500||L.at>Date.now())return;let W=Ts.find(z=>z.district===(L.to==="agent"?L.from:L.to));W&&h.has(W.id)&&r?.(W.id,2,L.state)},stats:()=>({completed:g,rigs:c.filter(L=>L.rig).length,meetings:u.length,reservations:d.filter(L=>L.owner).length,signals:x.stats(),residents:c.filter(L=>L.body.enabled).map(L=>({...b(L),x:L.body.x,z:L.body.z,heading:L.body.heading})),places:[...h.keys()]}),dispose(){T=!0,D(),c.forEach(L=>e.remove(L.id)),c.length=0,h.clear(),d.length=0,x.dispose()}}}function Cm(s){let e=new yt;e.name="living-machinery",s.add(e);let t=new Set,n=new Set,i=new Map,r=0,a="high",o=T=>(t.add(T),T),l=T=>(n.add(T),T),c=o(new Jn(1,.025,6,64)),h=o(new Ir(1,48)),d=o(new Mt),u=new Float32Array(288),p=new Float32Array(96);for(let T=0;T<96;T++)u[T*3]=Math.cos(T*2.399)*Math.sqrt(T/96),u[T*3+2]=Math.sin(T*2.399)*Math.sqrt(T/96),p[T]=T/96;d.setAttribute("position",new Tt(u,3)),d.setAttribute("phase",new Tt(p,1));for(let T of Ts){let g=new yt;g.name=T.id,g.position.set(T.x,.12,T.z),g.visible=!1,e.add(g);let _=l(new qt({color:7529695,transparent:!0,opacity:.15,depthWrite:!1,blending:tn,toneMapped:!1})),S=[];for(let C=0;C<3;C++){let w=new ot(c,l(_.clone()));w.rotation.x=-Math.PI/2,g.add(w),S.push(w)}let A=new ot(h,l(_.clone()));A.rotation.x=-Math.PI/2,A.scale.setScalar(2.5),A.position.y=.2,g.add(A);let b=new Tn(d,l(new wt({transparent:!0,depthWrite:!1,blending:tn,uniforms:{time:{value:0},strength:{value:0},color:{value:new Ce(10414816)}},vertexShader:"attribute float phase;uniform float time;uniform float strength;varying float a;void main(){float t=fract(phase+time*.31);vec3 p=position*vec3(2.,1.,2.);p.y=t*3.;p.xz*=1.-t*.7;vec4 v=modelViewMatrix*vec4(p,1.);a=sin(t*3.14159)*strength;gl_PointSize=clamp(65./max(1.,-v.z),1.,5.);gl_Position=projectionMatrix*v;}",fragmentShader:"uniform vec3 color;varying float a;void main(){float d=length(gl_PointCoord-.5);gl_FragColor=vec4(color,a*(1.-smoothstep(.05,.5,d)));}"})));g.add(b),i.set(T.id,{group:g,waves:S,base:A,motes:b,district:T.district,started:0,until:0,source:"ambient",model:null})}let x=new ot(o(new un(3.9,2.3)),l(new wt({transparent:!0,depthWrite:!1,side:Yt,blending:tn,uniforms:{time:{value:0},strength:{value:0}},vertexShader:"varying vec2 q;void main(){q=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 q;uniform float time;uniform float strength;void main(){float line=pow(max(0.,1.-abs(q.y-fract(time*.4))*14.),3.);float grid=step(.94,fract(q.x*24.))*step(.9,fract(q.y*18.));gl_FragColor=vec4(.18,.9,.8,(line*.36+grid*.1)*sin(q.x*3.14159)*strength);}"})));x.position.set(0,1.4,.8),i.get("repair-bay").group.add(x);let v=i.get("kinetic-fountain"),m=l(new wt({transparent:!0,depthWrite:!1,side:Yt,uniforms:{time:{value:0}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 vUv;uniform float time;void main(){float pulse=.5+.5*sin(vUv.x*95.-time*9.);float edge=.35+.65*pow(max(0.,sin(vUv.y*3.14159)),2.);gl_FragColor=vec4(mix(vec3(.09,.35,.42),vec3(.6,.95,1.),pow(max(0.,pulse),5.)),edge*.56);}"}));for(let T=0;T<8;T++){let g=T*Math.PI/4,_=new k(Math.cos(g)*2.3,.5,Math.sin(g)*2.3),S=new k(Math.cos(g+.5)*.7,.55,Math.sin(g+.5)*.7),A=new xi(_,new k(Math.cos(g)*1.5,3.7,Math.sin(g)*1.5),S);v.group.add(new ot(o(new Da(A,24,.045,5,!1)),m))}let f=l(new wt({transparent:!0,depthWrite:!1,uniforms:{time:{value:0}},vertexShader:"varying vec2 q;uniform float time;void main(){q=position.xy;vec3 p=position;p.z+=sin(length(q)*18.-time*4.)*.012;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.);}",fragmentShader:"varying vec2 q;uniform float time;void main(){float r=length(q);float a=.5+.5*sin(r*16.-time*3.+sin(q.x*3.+q.y*2.+time*.7)*.35);float rim=1.-smoothstep(2.35,2.58,r);gl_FragColor=vec4(mix(vec3(.03,.13,.17),vec3(.22,.55,.58),pow(max(0.,a),9.)*.3),.78*rim);}"})),y=new ot(o(new Ir(2.58,64)),f);return y.rotation.x=-Math.PI/2,y.position.y=.3,v.group.add(y),{attach(T,g){let _=i.get(T);_&&(_.model=g,_.group.visible=!0)},demonstrate(T,g=6,_=null){let S=i.get(T);S&&(S.until<=r&&(S.started=r),S.until=r+g,S.source=_?"live":"ambient",S.status=_,S.model?.play("operate"))},clear(){i.forEach(T=>{T.until=0,T.source="ambient",T.status=null})},setTier(T){a=T},update(T,g){g&&(r+=T),m.uniforms.time.value=f.uniforms.time.value=x.material.uniforms.time.value=r;for(let[_,S]of i){let A=S.until>r,b=_==="kinetic-fountain",C=A?It.smoothstep(r-S.started,0,.35)*It.smoothstep(S.until-r,0,.65):0,w=b?Math.max(.25,C):C;A||(S.source="ambient",S.status=null);let I=S.status==="failed"?16737881:S.source==="live"?{infra:7978495,integrations:7728086,missions:10268415,graph:16765844,operations:16758915}[S.district]:7529695;if(S.waves.forEach((M,N)=>{M.visible=A;let D=(r*.6+N/3)%1;M.position.y=.4+D*2,M.scale.setScalar(.4+D*2.6),M.material.opacity=(1-D)*.22*C,M.material.color.setHex(I)}),_==="repair-bay"&&(x.visible=A,x.material.uniforms.strength.value=C),_==="relay-mast"&&A){let M=S.model?.node.getObjectByName("antenna");M&&(M.rotation.y=It.damp(M.rotation.y,Math.atan2(-S.group.position.x,-12-S.group.position.z),3,g?T:0))}S.base.material.opacity=.08*C,S.motes.visible=a!=="low"&&w>0,S.motes.material.uniforms.time.value=r,S.motes.material.uniforms.strength.value=w,S.model?.mixer&&(S.model.mixer.timeScale=g&&(A||b)?.65:0)}},stats:()=>({places:[...i].filter(([,T])=>T.model).map(([T,g])=>({id:T,operating:g.until>r,source:g.source})),time:r}),dispose(){e.removeFromParent(),t.forEach(T=>T.dispose()),n.forEach(T=>T.dispose()),i.clear()}}}var sM={cruise:12,boost:26,climb:7,sink:7,ease:5,eye:2.4,takeoff:1.5,takeoffTime:.7,ceiling:150,minX:-140,maxX:140,minZ:-150,maxZ:130,glide:8,descend:6,reach:40,bob:.03,slack:15},Ot=sM,er=(s,e,t)=>Math.max(e,Math.min(t,s)),rM=s=>s<=0?0:s>=1?1:s*s*(3-2*s);function Pm({clear:s,ground:e,walkable:t,reduced:n=()=>!1}){let i="idle",r=!1,a=0,o=0,l=0,c=0,h=0,d=0,u=0,p=0,x=0,v=null,m=0,f=!1,y=0,T=0,g=(M,N)=>e(M,N)+Ot.eye,_=()=>({x:a,y:o+m,z:l}),S=()=>{c=h=d=0},A=(M,N,D)=>(M+=(N-M)*(1-Math.exp(-Ot.ease*D)),Math.abs(M)<.01&&!N?0:M);function b(M,N=[]){for(let D=0;D<=Math.max(Ot.maxX-Ot.minX,Ot.maxZ-Ot.minZ);D+=D<Ot.reach?1.5:4){let U=D?Math.ceil(D*Math.PI*2/(D<Ot.reach?1.5:4)):1;for(let H=0;H<U;H++){let L=H/U*Math.PI*2,W=a+Math.cos(L)*D,z=l+Math.sin(L)*D;if(W<Ot.minX||W>Ot.maxX||z<Ot.minZ||z>Ot.maxZ||N.some(J=>Math.hypot(J.x-W,J.z-z)<1)||!t(W,z))continue;let E=g(W,z);if(!(M&&!s({x:W,y:Math.max(o,E),z},{x:W,y:E,z})))return{x:W,y:E,z,blocked:0,skipped:N}}}return null}let C=M=>{a=M.x,o=M.y,l=M.z,S(),m=0,i="idle",r=!0,v=null};function w(M){let N=a,D=l,U=er(a+c*M,Ot.minX,Ot.maxX),H=er(l+d*M,Ot.minZ,Ot.maxZ);U!==a+c*M&&(c=0),H!==l+d*M&&(d=0),U!==a&&(s({x:a,y:o,z:l},{x:U,y:o,z:l})?a=U:c=0),H!==l&&(s({x:a,y:o,z:l},{x:a,y:o,z:H})?l=H:d=0);let L=g(a,l);o<L&&(s({x:a,y:o,z:l},{x:a,y:L,z:l})?o=L:(a=N,l=D,c=d=0,L=g(a,l)));let W=er(o+h*M,L,Ot.ceiling);W!==o+h*M&&(h=0),W!==o&&(s({x:a,y:o,z:l},{x:a,y:W,z:l})?o=W:h=0)}function I(M){let N=v,D=N.x-a,U=N.z-l,H=Math.hypot(D,U),L=o-N.y;if(y+=M,y>T){let Le=b(!1);Le?C(Le):(i="flying",v=null);return}if(H<.05&&L<.02){C(N);return}let W=H>.05?Math.min(Ot.glide,Math.max(.8,H*1.4)):0,z=L>.02?Math.min(Ot.descend,Math.max(.6,L*1.8)):0,E=H>.05?D/H*Math.min(W*M,H):0,J=H>.05?U/H*Math.min(W*M,H):0,q=-Math.min(z*M,Math.max(0,L)),te={x:a,y:o,z:l};if(s(te,{x:a+E,y:o+q,z:l+J}))a+=E,o+=q,l+=J,c=E/M,d=J/M,h=q/M,N.blocked=0;else if((E||J)&&s(te,{x:a+E,y:o,z:l+J}))a+=E,l+=J,c=E/M,d=J/M,h=0,N.blocked=0;else if(S(),H>=.05&&o<Ot.ceiling&&s(te,{x:a,y:o+Ot.descend*M,z:l}))o+=Ot.descend*M,h=Ot.descend,N.blocked=0;else if((N.blocked+=M)>1){let Le=b(!0,[...N.skipped,N]);Le?v=Le:(i="flying",v=null)}}return{start(M){return a=M.x,o=Math.max(M.y,g(M.x,M.z)),l=M.z,S(),i="takeoff",r=!1,u=0,p=o,x=0,v=null,m=0,_()},place(M){a=M.x,o=M.y,l=M.z,S(),m=0},step(M,N={},D=0){if(M=er(M,0,.1),i==="landing"&&(+N.forward||+N.right||+N.up?f&&(i="flying",v=null):f=!0),i==="takeoff"){u=n()?Ot.takeoffTime:Math.min(Ot.takeoffTime,u+M);let U=p+Ot.takeoff*rM(u/Ot.takeoffTime);U>o&&s({x:a,y:o,z:l},{x:a,y:U,z:l})&&(o=U),u>=Ot.takeoffTime&&(i="flying")}else if(i==="landing")M>0&&I(M);else if(i==="flying"){let U=er(+N.forward||0,-1,1),H=er(+N.right||0,-1,1),L=Math.hypot(U,H);L>1&&(U/=L,H/=L);let W=N.boost?Ot.boost:Ot.cruise,z=Math.sin(D),E=Math.cos(D),J=er(+N.up||0,-1,1);c=A(c,(-z*U+E*H)*W,M),d=A(d,(-E*U-z*H)*W,M),h=A(h,J*(J>0?Ot.climb:Ot.sink),M),w(M),x+=M,m=n()?0:Math.min(Ot.bob,Math.max(0,o-g(a,l)),Ot.ceiling-o)*Math.sin(x*2.1)}return _()},land(M=!1){let N=M||n();if(i==="idle")return!1;if(i==="landing"&&!N)return!0;let D=b(!N);return D?N?(C(D),!0):(v=D,i="landing",m=0,f=!1,y=0,T=Ot.slack+Math.max(Math.hypot(D.x-a,D.z-l)/Ot.glide,Math.max(0,o-D.y)/Ot.descend),!0):!1},cancel(){return i!=="landing"?!1:(i="flying",v=null,!0)},reset(){i="idle",r=!1,v=null,S(),m=0},state(){return{phase:i,flying:i!=="idle",landing:i==="landing",target:v?{x:v.x,z:v.z}:null,done:r,x:a,y:o+m,z:l,altitude:Math.max(0,o-g(a,l)),speed:Math.hypot(c,d),climb:h}}}}function Im(s,e){let t=new yt;t.name="world-2",s.add(t);let n=new Map,i=new Set,r=new Set,a=[],o=[],l=[],c=new Map,h=new Map,d=[],u=new Map,p=new Map,x=new Set,v=[],m=new Set,f=[],y=new Map,T=new AbortController,g=[],_=0,S=!1,A=!1,b=e.tier||"high",C=null,w=0,I=0,M=null,N=null,D=0,U=new Set,H=null,L=0,W="",z=new k,E=new k,J=new k,q=e.camera,te={ready:!1,lift:null,cab:null,value:0,motion:null,idle:0},Le=[],Fe=new kn(0,0,0,"YXZ"),Ne=P=>Math.atan2(Math.sin(P),Math.cos(P)),se=P=>P<=0?0:P>=1?1:P*P*(3-2*P),oe=e.traffic||Oc(),Pe=Cm(s),ge=new Map;function xe(P,B){let K=Fn(P,B);for(let ee of Ts)for(let X of C?.assets.find(Z=>Z.id===ee.id)?.navigation.surfaces||[]){let[Z,Ee,Be,st]=X.rect;P>=ee.x+Z&&P<=ee.x+Be&&B>=ee.z+Ee&&B<=ee.z+st&&(K=Math.max(K,Fn(ee.x,ee.z)+X.height))}return K}let $=(P,B)=>{let K=jp(P,B,e.districts||[]);return K>0?Math.max(xe(P,B),K):xe(P,B)},ue=[],ce=()=>oe.body("visitor"),ie=new k,de=Pm({ground:$,reduced:()=>e.reduced(),clear:(P,B)=>e.clear?e.clear(P,B):!ce()||oe.clear(ce(),{...P,heading:0},{...B,heading:0},!1),walkable:(P,B)=>{if(Es(P,B)||!nd(P,B,{x:P,z:B},()=>!1,e.districts||[]))return!1;let K={x:P,y:$(P,B)+2.4,z:B,heading:0};return!ce()||oe.clear(ce(),K,K)}}),ve=Rm(s,{traffic:oe,camera:q,floor:xe,onSelect:e.onSociety,onDemonstrate:Pe.demonstrate,active:e.active,onGait:e.onGait,shadows:e.shadows}),We=0,Ve=0;try{U=new Set(JSON.parse(localStorage.getItem("aurago.desktop.sysworld.discoveries")||"[]").filter(P=>oi.some(B=>B.id===P)))}catch{}let lt=Xu(Ku),nt=lt.getLength(),ct=oi.map(P=>{let B=1/0,K=0;for(let ee=0;ee<3e3;ee++){lt.getPointAt(ee/3e3,z);let X=Math.hypot(z.x-P.x,z.z-P.z);X<B&&(B=X,K=ee/3e3)}return{...P,u:K}}),Q=(P,B,K)=>e.onSound?.(P,B?.x||0,B?.y||0,B?.z||0,K),Pt=new An({color:11834208,metalness:.8,roughness:.35});r.add(Pt);for(let P of[-1,1]){let B=[],K=[];for(let Z=0;Z<=512;Z++){lt.getPointAt(Z/512,z),lt.getTangentAt(Z/512,E);for(let Ee of[-.045,.045]){let Be=P*.85+Ee;B.push(z.x-E.z*Be,pt.road+.09,z.z+E.x*Be)}if(Z<512){let Ee=Z*2;K.push(Ee,Ee+1,Ee+2,Ee+1,Ee+3,Ee+2)}}let X=new Mt;X.setAttribute("position",new xt(B,3)),X.setIndex(K),X.computeVertexNormals(),i.add(X),t.add(new ot(X,Pt))}async function ft(P,B=b==="low"?2:b==="medium"?1:0){let K=P+":"+B;return n.has(K)||n.set(K,(async()=>{let ee=C.assets.find(Je=>Je.id===P),X=ee?.lods.find(Je=>Je.level===B);if(!X||!/^[\w-]+\.lod[0-2]\.glb$/.test(X.file))throw Error("Invalid world asset");let Z=await fetch(e.assetURL(X.file),{signal:T.signal});if(!Z.ok)throw Error("World asset unavailable");let Ee=await Z.arrayBuffer();if(A)throw Error("Disposed");w+=Ee.byteLength;let Be=await new wi().parseAsync(Ee,""),st=Be.animations.length>0||/^(tram|service-cart|robot-|door|lift|sky-lift|telescope)/.test(P);if(Be.scene.traverse(Je=>{if(Je.isMesh){i.add(Je.geometry);for(let kt of Array.isArray(Je.material)?Je.material:[Je.material])kt.isMeshStandardMaterial&&kt.metalness>.5&&(kt.roughness=Math.max(kt.roughness,.45)),r.add(kt);Je.castShadow=!0,Je.receiveShadow=!0,e.surfaces?.prepare(Je,st)}}),A)throw i.forEach(Je=>Je.dispose()),r.forEach(Je=>Je.dispose()),Error("Disposed");return x.add(P),Be})()),n.get(K)}async function O(P,B,K,ee=0,X=0,Z=t,Ee=[1,1,1],Be){let st=await ft(P,Be);if(A)return null;let Je=st.scene.clone(!0);Je.position.set(B,ee,K),Je.rotation.y=X,Je.scale.set(...Ee),Z.add(Je),Je.traverse(en=>{en.isMesh&&(en.userData.worldAsset=P,en.userData.worldPart=en.name,v.push(en))});let kt=C.assets.find(en=>en.id===P),Bt="furnishing:"+We++;for(let[en,fn]of(kt.navigation.colliders||[]).entries())oe.solid(Bt+":"+en,{owner:Bt,x:B,z:K,heading:X,min:[fn[0]*Ee[0],ee+fn[1]*Ee[1],fn[2]*Ee[2]],max:[fn[3]*Ee[0],ee+fn[4]*Ee[1],fn[5]*Ee[2]]});let St=st.animations.length?new Va(Je):null;St&&a.push(St);let Mn=null,Vt=null,nn=/^robot-/.test(P);return{node:Je,mixer:St,owner:Bt,bounds:kt.motion_bounds||kt.lods[0].bounds,clips:st.animations,play(en,fn=!1){if(!St)return;let ss=st.animations.find(Nn=>Nn.name===en);if(!ss)return;let wn=St.clipAction(ss);if(Mn===wn)return wn;let rs=Vt;return wn.reset(),fn&&(wn.setLoop(gc,1),wn.clampWhenFinished=!0),wn.play(),Mn&&wn.crossFadeFrom(Mn,.25,!1),Mn=wn,Vt=en,nn&&rs&&en!==rs&&en!=="walk"&&e.onSound?.("robot_"+en,Je.position.x,Je.position.y,Je.position.z,Bt),wn}}}function R(P){P.updateMatrixWorld(!0);let B=new Map;P.traverse(ee=>{if(!ee.isMesh)return;let X=ee.geometry.uuid+":"+(Array.isArray(ee.material)?ee.material.map(Z=>Z.uuid).join(","):ee.material.uuid);B.has(X)||B.set(X,{n:ee,matrices:[]}),B.get(X).matrices.push(ee.matrixWorld.clone())});let K=new yt;t.add(K);for(let{n:ee,matrices:X}of B.values()){let Z=new En(ee.geometry,ee.material,X.length);X.forEach((Ee,Be)=>Z.setMatrixAt(Be,Ee)),Z.castShadow=!0,Z.receiveShadow=!0,Z.userData={...ee.userData},v.push(Z),K.add(Z)}return P.traverse(ee=>{let X=v.indexOf(ee);X>=0&&v.splice(X,1)}),P.removeFromParent(),u.set(K.uuid,K),K}function re(P){if(p.has(P.id))return p.get(P.id);let B=(async()=>{let K=new yt;t.add(K);let ee=[];for(let X of Bp())ee.push(O("floor",P.x+X.x,P.z+X.z,pt.room,0,K,[X.sx,1,X.sz]));for(let X=-4;X<=4;X+=4)for(let Z=-4;Z<=4;Z+=4)ee.push(O("ceiling",P.x+X,P.z+Z,8.15+pt.room,0,K));for(let X=-4;X<=4;X+=4)ee.push(O("wall",P.x+X,P.z+6,pt.room,0,K));for(let X=-4;X<=4;X+=4)for(let Z of[-1,1])for(let Ee of[0,4])ee.push(O("window",P.x+Z*6,P.z+X,pt.room+Ee,Math.PI/2,K));for(let X=-4;X<=4;X+=4)for(let Z of[-1,1])ee.push(O("window",P.x+X,P.z+Z*6,pt.gallery,0,K));for(let X of[-4,4])ee.push(O("wall",P.x+X,P.doorZ,pt.room,0,K));for(let[X,Z]of[[-4,1],[.2,1.1]])ee.push(O("floor",P.x+X,P.z+3,pt.gallery,0,K,[Z,1,1.5])),ee.push(O("railing",P.x+X,P.z,pt.gallery,0,K,[Z,1,1]));for(let X of[.7,5.3])ee.push(O("railing",P.x+2.4,P.z+X,pt.gallery,Math.PI/2,K,[.35,1,1]));for(let X of[-5.6,1.9])ee.push(O("wall",P.x+X,P.z+.15,pt.room,0,K,[.05,1,.6]));if(await Promise.all(ee),!A)return R(K)})();return p.set(P.id,B),B}async function le(P){if(!(c.has(P.id)||A)){c.set(P.id,{ready:!1,lift:null,liftValue:0,liftTarget:0}),L++;try{await re(P);let B=new yt;t.add(B);let K=[];for(let X of[-3,0])K.push(O(P.id==="missions"?"cargo":P.id==="memory"?"archive-shelf":"console",P.x+X,P.z+3,pt.room,Math.PI,B));if(K.push(O("bench",P.x-4,P.z-1,pt.room,Math.PI/2,B)),K.push(O("bench",P.x-4,P.z+4.8,pt.gallery,0,B)),K.push(O("console",P.x,P.z+4.8,pt.gallery,Math.PI,B)),await Promise.all(K),A)return;R(B);let ee=c.get(P.id);ee.hologram=await O("hologram",P.x-1,P.z,pt.room),ee.hologram?.play("operate"),P.id==="memory"&&Ue(),ee.lift=await O("lift",P.liftX,P.liftZ,pt.room),ee.liftAction=ee.lift?.play("operate",!0),ee.liftAction&&(ee.liftAction.paused=!0),await V(P.id,P.x-2.2,P.z+.9,pt.gallery,Math.PI,{skip:null,limit:.9,pitch:[-.14,.18],eye:0}),ee.ready=!0}catch(B){A||e.onError?.(B)}finally{L--}}}async function be(){try{let P=await fetch(e.assetURL("manifest.json"),{signal:T.signal});if(!P.ok)throw Error("World manifest unavailable");C=await P.json();let B=new yt;t.add(B);let K=[];for(let Z of oi)K.push(O("station",Z.platformX,Z.platformZ,pt.pavement,Z.angle,B));for(let Z of Ln){K.push(re(Z));let Ee=await O("door",Z.x,Z.doorZ,pt.room,Math.PI);if(A)return;let Be=Ee.play("open",!0);Be.paused=!0,h.set(Z.id,{...Ee,action:Be,value:0,open:!1})}for(let Z=-76;Z<=76;Z+=8)K.push(O("quay",Z,77,pt.quay,0,B));for(let Z of[-68,-52,-28,28,52,68])K.push(O("quay-lantern",Z,75.5,pt.quay,0,B));for(let[Z,Ee]of[[-79,68],[-79,-67],[55,2],[8,40],[55,-24]])K.push(O("garden",Z,Ee,0,0,B)),K.push(O("bench",Z+3,Ee,0,Math.PI/2,B));for(let[Z,Ee]of[[-30,23],[30,23],[-28,72]])K.push(O("arcade",Z,Ee,0,0,B));K.push(O("bridge",-78.5,40,2,0,B,[1.75,1,1]),O("ramp",-80,49,0,Math.PI,B),O("stairs",-76.8,49,0,Math.PI,B),O("ramp",-78.5,31,0,0,B)),K.push(O("pad",ki.x,ki.z,0,0,B)),K.push(O("sky-deck",jt.x,jt.z,jt.floor,0,B));for(let Z of jt.benches)K.push(O("bench",Z.x,Z.z,jt.floor,Z.angle,B));for(let[Z,Ee]of[[-60,-56],[-60,-46],[51,49]])K.push(O("charger",Z,Ee,0,0,B));if(await Promise.all(K),A||(R(B),te.lift=await O("sky-lift",jt.lift.x,jt.lift.z,jt.lift.base),A))return;te.cab=te.lift?.node.getObjectByName("cab")||null,te.ready=!!te.cab;for(let Z of jt.telescopes)await V(Z.id,Z.x,Z.z,jt.floor,Z.angle,{skip:"agent",limit:1.45,pitch:[-1.1,.3],eye:1.25});for(let[Z,Ee]of[[-57,-61],[-57,-52]])(await O("cooler",Z,Ee))?.play("operate");for(let Z of Ts){let Ee=await O(Z.id,Z.x,Z.z,Fn(Z.x,Z.z),0,t,[1,1,1],2);if(A)return;Ee&&(Ee.level=2,ge.set(Z.id,Ee),Pe.attach(Z.id,Ee),ve.addPlace(Z,C.assets.find(Be=>Be.id===Z.id)),Ee.play("operate"))}for(let Z of dm){let Ee=await O(Z.id,Z.x,Z.z,Z.y,Z.angle||0,t);if(!Ee){if(A)return;continue}Ee.play(Z.clip),d.push({id:Z.id,model:Ee})}let ee=C.assets.find(Z=>Z.id==="hover-pack");if(ee)try{let Z=ee.navigation.interaction?.[0]?.position||[0,0,1.2];for(let Ee of zp){let Be=await O("hover-pack",Ee.x,Ee.z,$(Ee.x,Ee.z),Ee.angle);if(A)return;if(!Be)continue;Be.play("idle");let st=Math.sin(Ee.angle),Je=Math.cos(Ee.angle);ue.push({...Ee,model:Be,pack:Be.node.getObjectByName("pack")||null,docked:!0,ix:Ee.x+Z[0]*Je+Z[2]*st,iz:Ee.z-Z[0]*st+Z[2]*Je})}}catch(Z){A||e.onError?.(Z)}let X=await O("service-cart",55,49,pt.ground,Math.PI/2);X&&X.play("open",!0);for(let Z=0;Z<19;Z++){let Ee=["courier","technician","archivist"][Z%3],Be=await O("robot-"+Ee,0,0);if(!Be)return;Be.node.scale.setScalar(1.15),o.push(Be),ve.add(Be,Z,Be.bounds),Be.play("idle")}for(let Z=0;Z<2;Z++){let Ee=await O("tram",0,0);if(!Ee)return;let Be=Ee.play("open",!0);Be.paused=!0;let st=ct[Z?3:0].u;lt.getPointAt(st,z),lt.getTangentAt(st,E);let Je=oe.register("tram-"+Z,ns(Ee.bounds),{x:z.x,y:pt.road+.03,z:z.z,heading:Math.atan2(E.x,E.z)},3);l.push({...Ee,body:Je,u:st,speed:0,dwell:5,stop:Z?3:0,door:Be})}for(let Z=0;Z<3;Z++){let Ee=await O("cargo",43,70);Ee&&(oe.removeOwner(Ee.owner),Ee.node.visible=!1,f.push({...Ee,body:null,elapsed:9,direction:1}))}e.onReady?.()}catch(P){A||e.onError?.(P)}}be();async function De(P,B,K){if(!(B.level===K||B.requested===K)){B.requested=K;try{let ee=await ft(P,K);if(A||B.requested!==K)return;let X=new Map;ee.scene.traverse(Z=>{Z.isMesh&&X.set(Z.name,Z)}),B.node.traverse(Z=>{let Ee=X.get(Z.name);Z.isMesh&&Ee&&(Z.geometry=Ee.geometry,Z.material=Ee.material)}),B.level=K,B.requested=null,e.onReady?.()}catch(ee){B.requested=null,A||e.onError?.(ee)}}}function Ue(){let P=c.get("memory");if(!P?.hologram||!P.text&&!g.length)return;if(!P.text){let Z=document.createElement("canvas");Z.width=1024,Z.height=512;let Ee=new Hs(Z);Ee.colorSpace=sn,m.add(Ee);let Be=new un(4.8,2.4),st=new qt({map:Ee,transparent:!0,depthWrite:!1,side:Yt,toneMapped:!1});i.add(Be),r.add(st);let Je=new ot(Be,st),kt=Ln.find(Bt=>Bt.id==="memory");Je.position.set(kt.x-1,3.6+pt.room,kt.z+.3),Je.rotation.y=Math.PI,t.add(Je),P.text={canvas:Z,texture:Ee,mesh:Je}}let{canvas:B,texture:K,mesh:ee}=P.text,X=B.getContext("2d");X.clearRect(0,0,1024,512),ee.visible=g.length>0,X.fillStyle="rgba(5,24,32,.87)",X.fillRect(0,0,1024,512),X.fillStyle="#b9f4ef",X.font="28px sans-serif",g.slice(0,4).forEach((Z,Ee)=>{let Be=Array.from(Z).slice(0,96);X.fillText(Be.slice(0,48).join(""),30,55+Ee*118),X.fillText(Be.slice(48).join(""),30,94+Ee*118)}),K.needsUpdate=!0}async function V(P,B,K,ee,X,Z){let Ee=await O("telescope",B,K,ee,X);!Ee||A||Le.push({id:P,model:Ee,tube:Ee.node.getObjectByName("tube"),x:B,z:K,floor:ee,angle:X,...Z,ex:B+Math.sin(X)*Z.eye,ez:K+Math.cos(X)*Z.eye})}function Y(P){let B=Le.find(Be=>Be.id===P);if(!B)return;let K=Ne(B.angle+Math.PI),ee=B.floor+2.05,X=K,Z=-.08,Ee=1/0;for(let Be of e.districts||[]){if(Be.id===B.skip)continue;let st=Be.x-B.ex,Je=Be.z-B.ez,kt=Math.hypot(st,Je),Bt=Math.atan2(-st,-Je),St=Math.abs(Ne(Bt-K));kt>12&&St<B.limit&&St<Ee&&(Ee=St,X=Bt,Z=It.clamp(Math.atan2(Be.height*.45-ee,kt),...B.pitch))}M={kind:"scope",id:P,scope:B,yaw0:K,fov:14,sent:"",target:null,back:{position:q.position.clone(),quaternion:q.quaternion.clone(),fov:q.fov}},B.model.node.visible=!1,q.position.set(B.ex,ee,B.ez),q.quaternion.setFromEuler(Fe.set(Z,X,0,"YXZ")),e.reduced()&&(q.fov=M.fov,q.updateProjectionMatrix()),e.onRide?.("scope"),Q("door",q.position)}function he(P,B,K){let ee=K.radius*.6,X=P.x-K.x,Z=P.z-K.z,Ee=B.x*B.x+B.z*B.z;if(Ee<1e-9)return 1/0;let Be=2*(X*B.x+Z*B.z),st=Be*Be-4*Ee*(X*X+Z*Z-ee*ee);if(st<0)return 1/0;let Je=(-Be-Math.sqrt(st))/(2*Ee);if(Je<0&&(Je=(-Be+Math.sqrt(st))/(2*Ee)),Je<0)return 1/0;let kt=P.y+B.y*Je;return kt>=0&&kt<=K.height?Je:1/0}function _e(P){let B=M.scope;Fe.setFromQuaternion(q.quaternion,"YXZ"),Fe.y=M.yaw0+It.clamp(Ne(Fe.y-M.yaw0),-B.limit,B.limit),Fe.x=It.clamp(Fe.x,...B.pitch),Fe.z=0,q.quaternion.setFromEuler(Fe),q.position.set(B.ex,B.floor+2.05,B.ez);let K=e.reduced()?M.fov:It.damp(q.fov,M.fov,9,Math.min(P,.1));Math.abs(K-M.fov)<.02&&(K=M.fov),K!==q.fov&&(q.fov=K,q.updateProjectionMatrix()),q.getWorldDirection(z);let ee=null,X=1/0;for(let st of e.districts||[]){if(st.id===B.skip)continue;let Je=he(q.position,z,st);Je>12&&Je<X&&(X=Je,ee=st.id)}M.target=ee;let Z=M.back.fov/q.fov,Ee=(Math.atan2(z.x,-z.z)*180/Math.PI+360)%360,Be=[ee,ee?Math.round(X):0,Z.toFixed(1),Math.round(Ee)].join();Be!==M.sent&&(M.sent=Be,e.onScope?.({id:M.id,target:ee,distance:ee?Math.round(X):null,zoom:+Z.toFixed(1),bearing:Math.round(Ee)%360}))}function me(P,B,K){te.motion={from:te.value,to:P,t:0,duration:e.reduced()?0:B},te.idle=0,K&&Q("lift",te.lift.node.position)}function ae(){let P=jt.lift;q.position.set(P.x,P.base+P.cab+te.value+2.4,P.z)}function Se(P){me(P,15,!0),M={kind:"sky",to:P},ae(),q.quaternion.setFromEuler(Fe.set(-.42,Math.PI,0,"YXZ")),e.onRide?.("sky")}function Me(P,B){let K=jt.lift;if(!te.motion&&M?.kind!=="sky"&&B>0&&Math.hypot(q.position.x-K.x,q.position.z-K.z)>30&&(te.idle+=B,te.idle>28&&me(te.value>1?0:K.travel,15,!1)),te.motion){let ee=te.motion;ee.t+=Math.min(P,.1);let X=ee.duration?ee.t/ee.duration:1;te.value=ee.from+(ee.to-ee.from)*se(X),X>=1&&(te.value=ee.to,te.motion=null)}te.cab.position.y=te.value,M?.kind==="sky"&&(ae(),te.motion||(M=null,e.onRide?.(null),Q("lift",q.position)))}function Ie(P){let B=ue.find(K=>K.id===P);!B?.docked||e.reduced()||(B.docked=!1,B.pack&&(B.pack.visible=!1),M={kind:"hover",id:P},W="",ie.copy(q.position),de.start(q.position),e.onRide?.("hover"),Q("hover",q.position),we())}function j(P){let B=ue.find(K=>K.id===M?.id);B&&(B.docked=!0,B.pack&&(B.pack.visible=!0)),M=null,W="",e.onRide?.(null),e.onFlight?.(null),P&&Q("hover",q.position)}function Te(){let P=ce();P&&oe.relocate(P,q.position)&&q.position.set(P.x,P.y,P.z)}function we(){let P=de.state();if(P.done){q.position.set(P.x,P.y,P.z),Te(),j(!0);return}let B=[Math.round(P.altitude),Math.round(P.speed*3.6),P.landing].join();B!==W&&(W=B,e.onFlight?.({altitude:Math.round(P.altitude),speed:Math.round(P.speed*3.6),landing:P.landing}))}function Oe(){if(M?.kind==="hover"){de.state().landing?de.cancel():de.land(),we();return}if(M){if(M.kind==="tram"&&l[M.index].dwell<=1){M.exitRequested=!0;return}ke();return}if(N){if(N.kind==="telescope"){Y(N.id);return}if(N.kind==="skycall"){me(q.position.y>40?jt.lift.travel:0,7,!0);return}if(N.kind==="skyup"||N.kind==="skydown"){Se(N.kind==="skyup"?jt.lift.travel:0);return}if(N.kind==="resident"){ve.inspect(N.id);return}if(N.kind==="demonstrate"){ve.demonstrate(N.id,e.reduced()),e.onSociety?.({id:N.id,role:"installation",state:e.reduced()?"idle":"work",source:"ambient"});return}if(N.kind==="door"){let P=h.get(N.id);if(P){P.open=!P.open,Q("door",P.node.position);let B=Ln.find(K=>K.id===N.id);le(B)}}if(N.kind==="discover"){U.add(N.id);try{localStorage.setItem("aurago.desktop.sysworld.discoveries",JSON.stringify([...U]))}catch{}e.onDiscover?.(N.id),Q("discover",q.position)}if(N.kind==="tram"&&!e.reduced()&&(M={kind:"waiting",station:N.id},e.onRide?.("waiting")),N.kind==="terminal"&&e.onTerminal?.(N.id),N.kind==="hoverpack"){Ie(N.id);return}if(N.kind==="drone"&&!e.reduced()&&(M={kind:"drone",elapsed:0,origin:q.position.clone()},e.onRide?.("drone"),Q("tram",q.position)),N.kind==="lift"){let P=c.get(N.id);if(P){let B=q.position.y>5?4:0;if(Math.abs(P.liftValue-B)>.02){P.liftTarget=B;return}P.liftTarget=B?0:4,M={kind:"lift",id:N.id},e.onRide?.("lift"),Q("lift",q.position)}}}}function ke(){if(M){if(M.kind==="hover"){let P=ue.find(B=>B.id===M.id);if(de.land(!0)){let B=de.state();q.position.set(B.x,B.y,B.z)}else P&&q.position.set(P.ix,$(P.ix,P.iz)+2.4,P.iz);Te(),de.reset(),j(!1);return}if(M.kind==="lift"){let P=c.get(M.id),B=Ln.find(K=>K.id===M.id);P.liftTarget=P.liftValue<2?0:4,q.position.set(B.x+1.5,2.4+pt.room+P.liftTarget,B.liftZ)}if(M.kind==="tram"){let P=oi.find(B=>B.id===M.station)||oi[0];q.position.set(P.platformX,2.4+Fn(P.platformX,P.platformZ),P.platformZ)}if(M.kind==="drone"&&q.position.copy(M.origin),M.kind==="sky"&&(te.motion=null,te.value=M.to,te.cab.position.y=te.value,ae()),M.kind==="scope"){let P=M.scope;Fe.setFromQuaternion(q.quaternion,"YXZ"),P.tube?.rotation.set(-Fe.x,Ne(Fe.y+Math.PI-P.angle),0,"YXZ");let B=Fe.y;P.model.node.visible=!0,q.position.copy(M.back.position),Fe.setFromQuaternion(M.back.quaternion,"YXZ"),Fe.y=B,q.quaternion.setFromEuler(Fe),q.fov=M.back.fov,q.updateProjectionMatrix(),e.onScope?.(null)}M=null,e.onRide?.(null)}}function Ae(){if(M?.kind==="hover")return de.state().landing?[]:[{kind:"land",id:M.id,distance:0}];if(M)return[{kind:M.kind==="scope"?"unscope":"exit",id:M.kind,distance:0}];q.getWorldDirection(z);let P=ve.nearby().filter(K=>(K.x-q.position.x)*z.x+(K.z-q.position.z)*z.z>K.distance*.3),B=(K,ee,X,Z,Ee)=>{let Be=Math.hypot(q.position.x-X,q.position.z-Z);Be<Ee&&P.push({kind:K,id:ee,distance:Be})};for(let K of Ln)q.position.y<5&&B("door",K.id,K.x,K.doorZ,3),c.get(K.id)?.ready&&(B("lift",K.id,K.liftX,K.liftZ,2.3),q.position.y<5&&B("terminal",K.id,K.x-2,K.z+3,2.5));if(q.position.y<5)for(let K of oi)B("tram",K.id,K.platformX,K.platformZ,4),U.has(K.id)||B("discover",K.id,K.platformX+(K.angle===0?5:0),K.platformZ+(K.angle===0?0:5),3);if(B("drone","drone",ki.x,ki.z,5),q.position.y<5&&!e.reduced())for(let K of ue)K.docked&&B("hoverpack",K.id,K.ix,K.iz,2.4);if(te.ready){let K=jt.lift,ee=q.position.y>40,X=Math.hypot(q.position.x-K.x,q.position.z-K.z),Z=ee?K.travel:0;X<(ee?3.6:2.4)&&(ee||q.position.y<10)&&P.push({kind:te.motion||Math.abs(te.value-Z)>.05?"skycall":ee?"skydown":"skyup",id:"sky",distance:X})}for(let K of Le)Math.abs(q.position.y-(K.floor+2.4))<1.5&&B("telescope",K.id,K.x,K.z,1.7);return P.sort((K,ee)=>K.distance-ee.distance)}function Ye(P,B,K){if(A||!C)return;let ee=B?Math.min(P,.05):0;if(I+=ee,H=(M?.kind==="hover"&&q.position.y>8.3?null:Es(q.position.x,q.position.z)?.id)||null,Ve+=P,Ve>.5){Ve=0;for(let[X,Z]of ge){let Ee=q.position.distanceTo(Z.node.position),Be=b==="low"||Ee>55?2:b==="medium"||Ee>32?1:0;De(X,Z,Be)}}e.traffic||oe.begin();for(let X of Ln)Math.hypot(q.position.x-X.x,q.position.z-X.z)<32&&le(X);a.forEach(X=>{X.getRoot().visible&&X.update(ee)});for(let X of h.values()){let Z=It.damp(X.value,X.open?1:0,5,Math.min(P,.1));Math.abs(Z-(X.open?1:0))<.001&&(Z=X.open?1:0);let Ee=Be=>[-1,1].map(st=>({owner:X.owner,x:X.node.position.x,z:X.node.position.z,min:[st*.8+st*1.6*Be-.8,pt.room,-.13],max:[st*.8+st*1.6*Be+.8,pt.room+3.6,.13]}));(X.open||Ee(Z).every(Be=>oe.solidClear(Be)))&&(X.value=Z),Ee(X.value).forEach((Be,st)=>oe.solid(X.owner+":panel:"+st,Be)),X.action.time=X.value*X.action.getClip().duration,X.mixer.update(0)}te.ready&&Me(P,ee),M?.kind==="scope"&&_e(P);for(let[X,Z]of c)if(Z.liftAction&&(Z.liftValue=It.damp(Z.liftValue,Z.liftTarget,1.8,Math.min(P,.1)),Math.abs(Z.liftValue-Z.liftTarget)<.02&&(Z.liftValue=Z.liftTarget),Z.liftAction.time=Z.liftValue/4*Z.liftAction.getClip().duration,Z.lift.mixer.update(0),M?.kind==="lift"&&M.id===X)){let Ee=Ln.find(Be=>Be.id===X);q.position.set(Ee.liftX,2.4+pt.room+Z.liftValue,Ee.liftZ),Z.liftValue===Z.liftTarget&&(M=null,e.onRide?.(null))}if(ve.update(ee,B),Pe.update(ee,B),l.forEach((X,Z)=>{if(X.node.visible=!!X.body&&Z<(b==="low"?1:2),X.body&&(X.node.visible&&!X.body.enabled&&!oe.clear({...X.body,enabled:!0},X.body,X.body)&&(X.node.visible=!1),X.body.enabled=X.node.visible),!X.node.visible)return;let Ee={u:X.u,dwell:X.dwell,stop:X.stop},Be=!1;if(X.dwell>0)X.dwell-=ee,X.speed=0;else{let st=Vt=>(Vt.u-X.u+1)%1*nt,Je=ct.reduce((Vt,nn)=>st(nn)>1e-5&&st(nn)<st(Vt)?nn:Vt,ct.find(Vt=>st(Vt)>1e-5)||ct[0]),kt=Math.min(11,Math.sqrt(st(Je)*8));for(let Vt=1;Vt<=3;Vt++){let nn=(X.u+(1.2+X.speed*.7)*Vt/3/nt)%1;lt.getPointAt(nn,z),lt.getTangentAt(nn,E);let en={x:z.x,y:pt.road+.03,z:z.z,heading:Math.atan2(E.x,E.z)};if(!oe.clear(X.body,en,en)){kt=0;break}}X.speed+=It.clamp(kt-X.speed,-ee*8,ee*3.5);let Bt=ee*X.speed/nt,St=X.u;X.u=(X.u+Bt)%1;let Mn=ct.findIndex(Vt=>(Vt.u-St+1)%1>0&&(Vt.u-St+1)%1<=Bt+1e-5);Mn>=0&&(X.stop=Mn,X.u=ct[Mn].u,X.dwell=6,X.speed=0,Be=!0)}if(lt.getPointAt(X.u,X.node.position),X.node.position.y=pt.road+.03,lt.getTangentAt(X.u,E),X.node.rotation.y=Math.atan2(E.x,E.z),X.body&&(X.body.enabled=X.node.visible,oe.propose(X.body,{...X.node.position,heading:X.node.rotation.y},(st,Je)=>{st||(Object.assign(X,Ee),X.speed=0),X.node.position.set(Je.x,Je.y,Je.z),X.node.rotation.y=Je.heading,st&&Be&&Q("tram",X.node.position)})),X.door.time=(X.dwell>1?1:0)*X.door.getClip().duration,X.mixer.update(0),M?.kind==="waiting"&&ct[X.stop].id===M.station&&X.dwell>1&&(M={kind:"tram",index:Z,station:M.station,offset:0},e.onRide?.("tram")),M?.kind==="tram"&&M.index===Z){if(X.dwell>1&&(M.station=ct[X.stop].id,M.exitRequested)){ke();return}q.position.copy(X.node.position).addScaledVector(E,M.offset),q.position.y+=.9+1.4,q.lookAt(X.node.position.x+E.x*16,q.position.y,X.node.position.z+E.z*16)}}),M?.kind==="drone"){M.elapsed+=Math.min(P,.1);let X=M.elapsed/40*Math.PI*2;q.position.set(Math.cos(X)*100,45+Math.sin(X*2)*12,Math.sin(X)*95-10),q.lookAt(0,22,-12),M.elapsed>=40&&ke()}if(N=K==="street"&&Ae()[0]||null,K==="street"&&!M){let X=q.position.distanceTo(J);X<3&&(D+=X),D>1.8&&(Q(H?"step_inside":"step",q.position),D=0)}J.copy(q.position),e.onEnvironment?.(!!H);for(let[X,Z]of f.entries()){if(Z.elapsed+=ee,Z.node.visible=B&&Z.elapsed<8,!Z.node.visible){Z.body&&oe.remove(Z.body.id),Z.body=null;continue}let Ee=Z.direction>0?Z.elapsed/8:1-Z.elapsed/8,Be={x:40+Ee*3,y:pt.room,z:70,heading:0};if(Z.body||(Z.body=oe.register("freight-"+X,ns(Z.bounds),Be,0)),!Z.body){Z.elapsed=9,Z.node.visible=!1;continue}oe.propose(Z.body,Be,(st,Je)=>{st||(Z.elapsed-=ee),Z.node.position.set(Je.x,Je.y,Je.z)})}e.traffic||(oe.solve(ee),ve.settle(ee)),e.onInteraction?.(N,U.size,H)}async function Qe(P){if(b=P,!C)return;let B=P==="low"?2:P==="medium"?1:0;ve.setTier(P),Pe.setTier(P);try{let K=new Map(await Promise.all([...x].filter(ee=>!ge.has(ee)).map(async ee=>{let X=await ft(ee,B),Z=new Map;return X.scene.traverse(Ee=>{Ee.isMesh&&Z.set(Ee.name,Ee)}),[ee,Z]})));if(A||b!==P)return;for(let ee of v){let X=K.get(ee.userData.worldAsset)?.get(ee.userData.worldPart);X&&(ee.geometry=X.geometry,ee.material=X.material)}e.onReady?.()}catch(K){A||e.onError?.(K)}}return{update:Ye,interact:Oe,endRide:ke,setTier:Qe,society:ve,settle(P){!A&&C&&ve.settle(P)},suspend(){ve.suspend(),Pe.clear(),S=!1;for(let P of f)P.elapsed=9,P.node.visible=!1,P.body&&oe.remove(P.body.id),P.body=null},syncRide(){if(M?.kind==="tram"){let P=l[M.index];lt.getTangentAt(P.u,E),q.position.copy(P.node.position).addScaledVector(E,M.offset),q.position.y+=2.3,q.lookAt(P.node.position.x+E.x*16,q.position.y,P.node.position.z+E.z*16)}},socialAction(P,B){let K=oi.find(ee=>ee.id===B);return ve.action(e.reduced()&&P==="guide"?"cancel":P,K?{x:K.platformX,z:K.platformZ}:null)},setMemory(P){g=(Array.isArray(P)?P:[]).filter(B=>typeof B=="string").slice(0,8),Ue()},setWorld(P,B=!1){B?(ve.setReplay(!0),Pe.clear()):ve.setReplay(!1);let K=P?.entities?.filter(X=>X.kind==="mission")||[],ee=new Set;for(let X of K){ee.add(X.id);let Z=y.get(X.id);if(!B&&e.active()&&S&&Z!=null&&Z!==X.state&&["running","completed","failed","cancelled"].includes(X.state)&&Date.now()-X.at<3e4){let Ee=f.find(Be=>Be.elapsed>=8);Ee&&(Ee.elapsed=0,Ee.direction=X.state==="running"?1:-1,_++)}y.set(X.id,X.state)}for(let X of y.keys())ee.has(X)||y.delete(X);if(S=!B,B)for(let X of f)X.elapsed=9,X.node.visible=!1},walkRide(P,B,K=0){if(!M)return!1;if(M.kind==="tram"&&(M.offset=It.clamp(M.offset+P*B*3,-2.5,2.5)),M.kind==="scope"){Fe.setFromQuaternion(q.quaternion,"YXZ");let ee=B*.9*q.fov/M.back.fov;Fe.x+=P*ee,Fe.y-=K*ee,q.quaternion.setFromEuler(Fe)}return!0},hovering:()=>M?.kind==="hover",fly(P,B){if(M?.kind!=="hover")return!1;q.position.distanceToSquared(ie)>.01&&de.place(q.position),q.getWorldDirection(z);let K=de.step(P,B,Math.atan2(-z.x,-z.z));return ie.set(K.x,K.y,K.z),q.position.copy(ie),we(),!0},zoom(P){return M?.kind!=="scope"?!1:(M.fov=It.clamp(M.fov*Math.exp(P*.0012),5,26),!0)},scoping:()=>M?.kind==="scope",move(P,B,K){if(M)return!1;if(K.y>40)return Op(P,B,!te.motion&&te.value===jt.lift.travel);let ee=Es(K.x,K.z);return ee&&K.y>5?Math.abs(B-ee.z-4.8)<.85&&(Math.abs(P-ee.x)<1.1||Math.abs(P-ee.x+4)<1.75)?!1:Yu(ee,P,B,c.get(ee.id)?.liftValue===4):ee&&K.y<5&&([-5.6,1.9].some(X=>Math.abs(P-ee.x-X)<.25&&Math.abs(B-ee.z-.15)<.25)||kp(ee,P,B)&&!(P<ee.liftX+1.25&&Math.abs(B-ee.liftZ)<1.25&&c.get(ee.id)?.liftValue===0)||Math.hypot(P-(ee.x-1),B-ee.z)<1.55||[-3,0].some(X=>Math.abs(P-ee.x-X)<1.1&&Math.abs(B-ee.z-3)<.8))?!1:nd(P,B,K,X=>h.get(X)?.value>.85&&!!c.get(X)?.ready,e.districts)},floor(P,B){if(q.position.y>40)return jt.floor;let K=Es(P,B);return K&&q.position.y>5&&Yu(K,P,B,c.get(K.id)?.liftValue===4)?pt.gallery:$(P,B)},visit(P){ke(),M=null;let B=Ts.find(ee=>ee.id===P);if(B){let ee=C?.assets.find(X=>X.id===P)?.navigation.interaction[0]?.position||[0,1.5,4];q.position.set(B.x,2.4+Fn(B.x,B.z),B.z+ee[2]+1),q.lookAt(B.x,2,B.z);return}if(P==="drone"){q.position.set(ki.x,2.7,ki.z+3),q.lookAt(ki.x,2,ki.z);return}if(P==="skydeck"){let ee=57*Math.PI/180,X=jt.x+Math.sin(ee)*7.2,Z=jt.z-Math.cos(ee)*7.2;q.position.set(X,jt.floor+2.4,Z),q.lookAt(X,jt.floor+.4,Z-10);return}let K=oi.find(ee=>ee.id===P);if(K){let ee=U.has(P)?0:5,X=K.platformX+(K.angle===0?ee:0),Z=K.platformZ+(K.angle===0?0:ee);q.position.set(X,2.4+Fn(X,Z),Z),q.lookAt(K.platformX,2,K.platformZ===Z?K.platformZ+1:K.platformZ)}},destination(P){let B=Ln.find(K=>K.id===P);if(B){ke(),M=null;let K=B.doorZ+B.front*4;q.position.set(B.x,2.4+Fn(B.x,K),K),q.lookAt(B.x,2.4+Fn(B.x,K),B.z)}},isRiding:()=>!!M,interaction:()=>N,rideBody:()=>M?.kind==="tram"?"tram-"+M.index:M?.kind==="scope"?M.scope.model.owner:M?.kind==="sky"&&te.lift?.owner||null,stats:()=>({society:ve.stats(),machinery:Pe.stats(),details:[...ge].map(([P,B])=>({id:P,level:B.level})),loaded:[...x],bytes:w,freightEvents:_,residents:o.filter(P=>P.node.visible).length,trams:l.filter(P=>P.node.visible).length,rooms:[...c].filter(([,P])=>P.ready).map(([P])=>P),inside:H,ride:M?.kind||null,station:M?.station||null,discovered:[...U],loading:L,interactions:N?{kind:N.kind,id:N.id}:null,landmarks:d.map(P=>P.id),sky:{ready:te.ready,value:+te.value.toFixed(2),moving:!!te.motion},telescopes:Le.length,hover:(P=>({pads:ue.length,docked:ue.filter(B=>B.docked).length,flying:!!P,altitude:P?+P.altitude.toFixed(1):0,speed:P?+(P.speed*3.6).toFixed(1):0,landing:!!P?.landing}))(M?.kind==="hover"?de.state():null),scope:M?.kind==="scope"?{id:M.id,target:M.target,fov:+q.fov.toFixed(1)}:null}),dispose(){A||(A=!0,T.abort(),ve.dispose(),Pe.dispose(),e.traffic||oe.dispose(),g=[],y.clear(),t.removeFromParent(),a.forEach(P=>{P.stopAllAction(),P.uncacheRoot(P.getRoot())}),t.traverse(P=>{P.isInstancedMesh&&P.dispose()}),i.forEach(P=>P.dispose()),r.forEach(P=>P.dispose()),m.forEach(P=>P.dispose()),n.clear())}}}var aM=s=>.5+(s-6)/12*3.62,Dm=new k;function Lm(s,{sun:e,rim:t,hemisphere:n,atmosphere:i,surfaces:r,onThunder:a}){let o="local",l="clear",c="high",h=!1,d=-100,u=!1,p=n.intensity,x=14,v=-1,m=0,f={value:0},y=new Mt,T=[],g=317,_=()=>(g=Math.imul(g,1664525)+1013904223>>>0)/4294967296;for(let D=0;D<1200;D++)T.push((_()-.5)*170,_()*60,(_()-.5)*174-7);y.setAttribute("position",new xt(T,3));let S=new wt({transparent:!0,depthWrite:!1,uniforms:{time:f,flash:{value:0}},vertexShader:"uniform float time;varying float vFade;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;gl_PointSize=clamp(260.0/max(1.0,-mv.z),2.0,26.0);vFade=smoothstep(0.0,4.0,p.y)*smoothstep(460.0,60.0,-mv.z);}",fragmentShader:"uniform float flash;varying float vFade;void main(){vec2 c=gl_PointCoord-.5;float a=(1.0-smoothstep(0.0,0.07,abs(c.x)))*(1.0-smoothstep(0.2,0.5,abs(c.y)))*.42*vFade;if(a<.004)discard;gl_FragColor=vec4(mix(vec3(.6,.78,.86),vec3(.95,.97,1.0),flash),a*(1.0+flash));}"}),A=new Tn(y,S);A.frustumCulled=!1,s.add(A);let b=new Ce(9417679),C=new Ce(528926),w=new Ce(2762296);function I(){let D=Gc(o),U=D.amount,H=aM(D.hour);e.intensity=.8+U*2.6,e.color.set(D.evening>.1?16757370:13164287),e.position.set(Math.cos(D.hour/24*Math.PI*2)*120,40+U*115,85),Dm.set(Math.cos(H)*120,40-D.evening*18+U*115,Math.sin(H)*120),t.intensity=.7+D.evening*1.6,p=n.intensity=.45+U*.45,s.fog.color.copy(C).lerp(b,U).lerp(w,D.evening*(1-U)*.5),s.fog.density=l==="fog"?.018:l==="rain"?.008:.003-U*.0016,i.setLighting?.(U,D.evening,Dm),i.setWeather?.(l),r?.setLighting(U,D.evening,l==="clear"?0:1,s.fog.color),r?.setWeather(l),A.visible=l==="rain"&&!h,y.setDrawRange(0,c==="low"?200:1200)}function M(D,U){if(l!=="rain"||h||!U){v>=0&&(v=-1,N(0));return}if(x-=D,v<0&&x<=0&&(v=0,m++,x=9+_()*16,a?.(.6+_()*3.2)),v<0)return;v+=D;let H=v<.09?1:v<.16?.25:v<.24?.75:Math.max(0,1-(v-.24)/.35)*.5;N(H),v>.6&&(v=-1,N(0))}function N(D){n.intensity=p+D*2.4,S.uniforms.flash.value=D,i.setFlash?.(D)}return I(),{set(D={}){["local","day","evening","night"].includes(D.time)&&(o=D.time),["clear","rain","fog"].includes(D.weather)&&(l=D.weather),I()},setTier(D){c=D,I()},setIndoor(D){h!==D&&(h=D,I())},update(D,U,H){if(!u)return A.visible=l==="rain"&&!h&&H,H&&(f.value+=Math.min(D,.1)),M(Math.min(D,.1),H),U-d>30?(d=U,I(),!0):!1},mood(){let D=Gc(o);return{day:D.amount,evening:D.evening,weather:l,indoor:h}},stats:()=>({time:o,weather:l,indoor:h,daylight:Gc(o).amount,strikes:m}),dispose(){u=!0,A.removeFromParent(),y.dispose(),S.dispose()}}}var Rs={cx:30,cz:69.5,rx:4.5,rz:2.5,seconds:16,gaitHz:1.7},oM=["body","fl_hip","fr_hip","hl_hip","hr_hip","fl_uleg","fr_uleg","hl_uleg","hr_uleg","fl_lleg","fr_lleg","hl_lleg","hr_lleg"],lM={fl:0,hr:0,fr:Math.PI,hl:Math.PI};function cM(s){let e=s/Rs.seconds*Math.PI*2,t=Rs.cx+Rs.rx*Math.cos(e),n=Rs.cz+Rs.rz*Math.sin(e),i=-Rs.rx*Math.sin(e),r=Rs.rz*Math.cos(e);return{x:t,z:n,heading:Math.atan2(-r,i),phase:s*Rs.gaitHz*Math.PI*2}}function hM(s){let e={};for(let[t,n]of Object.entries(lM))e[t]={hip:.26*Math.sin(s+n),uleg:.13*Math.sin(s+n+.9)};return e}function Fm({scene:s,resourceURL:e,signal:t,ground:n=Fn,shadows:i=null,onError:r}){let a=new yt;a.visible=!1,s.add(a);let o=Object.create(null),l=new hn,c={y:NaN,v:0},h={y:0,v:0},d=i?.claim()||null,u=null,p=0,x=0,v=!1,m=!1,f=new wi;f.setMeshoptDecoder(Zr);let y=(async()=>{try{await Zr.ready;let g=await fetch(e("/3d/system-world/spot.glb"),{signal:t});if(!g.ok)throw Error("Companion asset unavailable: "+g.status);let _=await g.arrayBuffer();if(m)return!1;let S=await f.parseAsync(_,"");if(m)return!1;u=S.scene,u.traverse(A=>{A.isMesh&&(A.castShadow=!0,A.receiveShadow=!0)});for(let A of oM)o[A]=u.getObjectByName(A)||null;if(!o.body)throw Error("Companion joint hierarchy missing");return l.setFromObject(u),p=-l.min.y,a.add(u),a.visible=!0,!0}catch(g){return v=!0,r?.(g),!1}})();function T(g,_){if(!u||m)return;let S=_?Math.min(.1,Math.max(0,g)):0;x+=S;let A=cM(x),b=Math.cos(A.heading),C=-Math.sin(A.heading),w=.55,I=n(A.x,A.z),M=As(c,I,S),N=n(A.x+b*w,A.z+C*w)-n(A.x-b*w,A.z-C*w),D=As(h,Math.atan2(N,2*w),S);a.position.set(A.x,M+p+Math.abs(Math.sin(A.phase))*.012,A.z),a.rotation.set(0,A.heading,D,"YZX"),d?.set(A.x,I,A.z,.6,.45);for(let[U,H]of Object.entries(hM(A.phase)))o[U+"_hip"]&&(o[U+"_hip"].rotation.y=H.hip),o[U+"_uleg"]&&(o[U+"_uleg"].rotation.y=H.uleg)}return{ready:y,update:T,stats:()=>({loaded:!!u,failed:v,time:+x.toFixed(2),joints:Object.keys(o).filter(g=>o[g]).length,position:[+a.position.x.toFixed(2),+a.position.z.toFixed(2)]}),dispose(){m||(m=!0,d?.hide(),u&&u.traverse(g=>{if(g.isMesh){g.geometry.dispose();for(let _ of Array.isArray(g.material)?g.material:[g.material])_.dispose()}}),a.removeFromParent(),a.clear())}}}function Nm(s,{capacity:e=40}={}){let t=new un(1,1);t.rotateX(-Math.PI/2);let n=new mi(new Float32Array(e),1);t.setAttribute("swShadow",n);let i=new wt({transparent:!0,depthWrite:!1,polygonOffset:!0,polygonOffsetFactor:-2,polygonOffsetUnits:-2,vertexShader:`attribute float swShadow;varying vec2 vUv;varying float vA;void main(){vUv=uv;vA=swShadow;vec4 p=vec4(position,1.0);
#ifdef USE_INSTANCING
p=instanceMatrix*p;
#endif
gl_Position=projectionMatrix*modelViewMatrix*p;}`,fragmentShader:"varying vec2 vUv;varying float vA;void main(){float r=length(vUv-.5)*2.;gl_FragColor=vec4(0.,0.,0.,vA*(1.-smoothstep(.1,1.,r)));}"}),r=new En(t,i,e),a=new mt;r.name="contact-shadows",r.frustumCulled=!1,r.renderOrder=1,r.castShadow=!1,r.receiveShadow=!1;for(let l=0;l<e;l++)r.setMatrixAt(l,a.makeScale(0,0,0));s.add(r);let o=0;return{claim(){if(o>=e)return null;let l=o++;return{set(c,h,d,u,p){r.setMatrixAt(l,a.makeScale(u*2,1,u*2).setPosition(c,h+.015,d)),n.setX(l,p),r.instanceMatrix.needsUpdate=!0,n.needsUpdate=!0},hide(){r.setMatrixAt(l,a.makeScale(0,0,0)),n.setX(l,0),r.instanceMatrix.needsUpdate=!0,n.needsUpdate=!0}}},stats:()=>({used:o,capacity:e}),dispose(){r.removeFromParent(),r.dispose(),t.dispose(),i.dispose()}}}function Um(...s){let e=new Map;for(let t of s)for(let[n,i]of t instanceof Map?t:(t?.assets||[]).map(r=>[r.id,r]))e.set(n,i);return e}function Om(s,e,t,n){for(let i of t){let r=e.get(i.asset).lods[0].bounds;(ed[i.id]||(i.id==="graph"?[[-10.2,-10.2,10.2,10.2]]:[[-3,-3,3,3]])).forEach(([o,l,c,h],d)=>s.solid("district:"+i.id+":"+d,{x:i.x,z:i.z,min:[o,0,l],max:[c,8,h]})),td[i.id]?td[i.id].forEach(([o,l,c],h)=>s.solid("district:"+i.id+":upper:"+h,{x:i.x,z:i.z,min:[-o,l,-o],max:[o,c,o]})):s.solid("district:"+i.id+":upper",{x:i.x,z:i.z,min:[r.min[0],8,r.min[2]],max:r.max})}n.forEach((i,r)=>{if(i.asset==="street-tile"||i.asset==="street-crossing")return;let a=e.get(i.asset)?.lods[0].bounds;if(!a)return;let o=a.min.map((c,h)=>c*i.scale[h]),l=a.max.map((c,h)=>c*i.scale[h]);o[1]+=i.y,l[1]+=i.y,i.asset==="street-lamp"&&(s.solid("city:"+r+":pole",{x:i.x,z:i.z,min:[-.2,i.y,-.2],max:[.2,l[1],.2]}),o[1]=Math.max(o[1],l[1]-1)),s.solid("city:"+r,{x:i.x,z:i.z,heading:i.angle,min:o,max:l})})}var Mo={"city.road":{top:"asphalt",side:"concrete",albedo:.55,rough:.7,bump:.012,wet:1,puddle:[.447,.442]},"city.stone":{top:"pavers",side:"concrete",albedo:.5,rough:.6,bump:.014,wet:1,puddle:[.523,.516]},"city.graphite":{top:"gravel",side:"panels",albedo:.42,rough:.5,bump:.016,wet:.8,metalTop:.3},"city.titanium":{top:"panels",side:"panels",albedo:.2,rough:.45,bump:.006,wet:.6},"city.bronze":{top:"panels",side:"panels",albedo:.18,rough:.4,bump:.006,wet:.6},"city.robot-rubber":{top:"panels",side:"panels",albedo:.3,rough:.75,bump:.008,wet:.25},"city.ceramic":{top:"concrete",side:"concrete",albedo:.3,rough:.4,bump:.008,wet:.8,puddle:[.464,.46]},"city.leaf":{top:"foliage",side:"foliage",albedo:.7,rough:.3,bump:.03,wet:.5},"city.kerb":{top:"concrete",side:"concrete",albedo:.45,rough:.55,bump:.01,wet:1},"city.setts":{top:"pavers",side:"concrete",albedo:.5,rough:.6,bump:.014,wet:1,topScale:2,puddle:[.523,.516]},"city.iron":{top:"panels",side:"panels",albedo:.25,rough:.45,bump:.008,wet:.9},"city.marking":{top:"asphalt",side:"concrete",albedo:.25,rough:.5,bump:.006,wet:1,puddle:[.447,.442]},"city.tactile":{top:"concrete",side:"concrete",albedo:.4,rough:.6,bump:.02,wet:1},"city.glass":{top:"panels",side:"panels",albedo:.15,rough:.3,bump:.002,wet:.6},ground:{top:"pavers",side:"concrete",albedo:.55,rough:.6,bump:.014,wet:1,topScale:.5,puddle:[.523,.516],waterline:!0}},zm=new Set(["city.ivory","city.warm"]),uM="city.window",dM={low:0,medium:1,high:2,ultra:2},tr=s=>Number(s).toFixed(4),fM=`varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
attribute float swSeed;varying float vSwSeed;
#endif`,pM=`
#ifdef USE_INSTANCING
mat4 swObject=instanceMatrix;
#else
mat4 swObject=mat4(1.0);
#endif
#ifdef SW_WORLD
swObject=modelMatrix*swObject;
#endif
vSwPos=(swObject*vec4(transformed,1.0)).xyz;vSwNrm=mat3(swObject)*objectNormal;
#ifdef SW_WINDOWS
vSwSeed=swSeed>1.5?2.0:fract(swSeed+dot(swObject[3].xyz,vec3(.0913,.0571,.0729)));
#endif`,mM=`uniform float swWet;uniform float swRain;uniform float swTime;uniform vec4 swWindow;uniform vec2 swFade;uniform vec4 swShelter[3];uniform vec3 swSky;
varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
varying float vSwSeed;
#endif
#ifdef SW_SURFACE
uniform sampler2D swTop;uniform sampler2D swSide;uniform vec2 swScale;
float swHash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
vec3 swPerturb(vec3 pos,vec3 n,vec2 dh,float face){vec3 sx=dFdx(pos),sy=dFdy(pos),r1=cross(sy,n),r2=cross(n,sx);float det=dot(sx,r1)*face;return normalize(abs(det)*n-sign(det)*(dh.x*r1+dh.y*r2));}
#endif`,gM=`
vec3 swN=normalize(vSwNrm);
#ifdef SW_SURFACE
vec3 swW=pow(abs(swN),vec3(4.0));swW/=max(swW.x+swW.y+swW.z,1e-4);
float swDetail=1.0-smoothstep(swFade.x,swFade.y,length(vViewPosition));
float swUp=clamp(swN.y,0.0,1.0),swDry=0.0;
#ifdef SW_WORLD
for(int i=0;i<3;i++){vec4 r=swShelter[i];swDry=max(swDry,step(r.x,vSwPos.x)*step(vSwPos.x,r.z)*step(r.y,vSwPos.z)*step(vSwPos.z,r.w)*step(vSwPos.y,3.6));}
#endif
float swWetness=swWet*SW_WETK*(0.35+0.65*swUp)*(1.0-swDry),swAlbedoF=1.0,swRoughF=1.0,swH=0.5,swPuddle=0.0;
#if SW_LEVEL>0
vec4 swS=texture2D(swTop,vSwPos.xz*swScale.x)*swW.y+texture2D(swSide,vSwPos.zy*swScale.y)*swW.x+texture2D(swSide,vSwPos.xy*swScale.y)*swW.z;
vec4 swM=texture2D(swTop,vSwPos.xz*swScale.x*0.125,4.0);
swAlbedoF=mix(1.0,swS.r*2.0,SW_ALBEDO*swDetail)*mix(1.0,swM.r*2.0,SW_ALBEDO*0.6);
swRoughF=mix(1.0,swS.g*2.0,SW_ROUGH);swH=swS.b;
swPuddle=swWet*SW_WETK*(1.0-swDry)*smoothstep(0.8,0.97,swUp)*smoothstep(SW_PUDDLE_DRY,SW_PUDDLE_FULL,swM.b);
if(swRain>0.0){
  vec2 swRp=vSwPos.xz*2.2,swCell=floor(swRp),swF=fract(swRp)-0.5,swO=(vec2(swHash(swCell+17.3),swHash(swCell+41.9))-0.5)*0.3;
  float swPhase=fract(swTime*0.8+swHash(swCell)),swR=length(swF-swO),swFront=swPhase*0.33;
  float swRing=sin((swR-swFront)*70.0)*smoothstep(swFront+0.07,swFront,swR)*smoothstep(swFront-0.16,swFront,swR)*(1.0-swPhase);
  swH=mix(swH,0.5+0.3*swRing*swRain*smoothstep(0.35,0.8,swPuddle),swPuddle);
}
#endif
#ifdef SW_WORLD
float swSide=1.0-swW.y;
swAlbedoF*=mix(1.0,mix(0.64,1.0,smoothstep(0.35,1.9,vSwPos.y)),swSide*step(0.0,vSwPos.y));
#ifdef SW_WATERLINE
swAlbedoF*=mix(1.0,0.48,swSide*(1.0-smoothstep(-2.9,-1.7,vSwPos.y)));
#endif
#endif
#endif`,xM=`
#ifdef SW_SURFACE
diffuseColor.rgb*=swAlbedoF*mix(1.0,0.56,swWetness)*mix(1.0,0.7,swPuddle);
#endif`,bM=`
#ifdef SW_SURFACE
roughnessFactor=clamp(roughnessFactor*swRoughF,0.04,1.0);roughnessFactor=mix(roughnessFactor,roughnessFactor*0.6,swWetness);roughnessFactor=mix(roughnessFactor,0.04,swPuddle);
#endif`,vM=`
#ifdef SW_SURFACE
metalnessFactor*=mix(1.0,SW_METAL_TOP,swW.y);
#endif`,_M=`
#if SW_LEVEL>1
normal=swPerturb(-vViewPosition,normal,vec2(dFdx(swH),dFdy(swH))*SW_BUMP*swDetail,faceDirection);
#endif`,yM=`
#ifdef SW_SURFACE
float swFresnel=pow(1.0-clamp(dot(normal,normalize(vViewPosition)),0.0,1.0),5.0)*0.95+0.05;
totalEmissiveRadiance+=swSky*swFresnel*(swPuddle*0.6+swWetness*0.1);
#endif
#ifdef SW_WINDOWS
float swPane=(1.0-smoothstep(0.35,0.65,abs(swN.y)))*(1.0-step(1.5,vSwSeed));
totalEmissiveRadiance*=mix(swWindow.w,mix(swWindow.z,swWindow.y,step(vSwSeed,swWindow.x)),swPane);
#endif`;function MM(s,e=!1){let t=s?.getAttribute("position");if(!t||s.getAttribute("swSeed"))return;if(e){s.setAttribute("swSeed",new Tt(new Float32Array(t.count).fill(2),1));return}let n=t.count,i=new Int32Array(n),r=s.getIndex();for(let u=0;u<n;u++)i[u]=u;let a=u=>{for(;i[u]!==u;)i[u]=i[i[u]],u=i[u];return u},o=u=>r?r.getX(u):u,l=r?r.count:n;for(let u=0;u+2<l;u+=3){let p=a(o(u));i[a(o(u+1))]=p,i[a(o(u+2))]=p}let c=new Map,h=new Map;for(let u=0;u<n;u++){let p=a(u),x=h.get(p)||[0,0,0];c.set(p,(c.get(p)||0)+1),x[0]+=t.getX(u),x[1]+=t.getY(u),x[2]+=t.getZ(u),h.set(p,x)}let d=new Float32Array(n);for(let u=0;u<n;u++){let p=a(u),x=c.get(p),v=h.get(p);if(x>6){d[u]=2;continue}let m=Math.sin(v[0]/x*12.9898+v[1]/x*78.233+v[2]/x*37.719)*43758.5453;d[u]=m-Math.floor(m)}s.setAttribute("swSeed",new Tt(d,1))}function km(s,e={}){let t=new AbortController,n=new Map,i=[],r=new ps(new Uint8Array([128,128,128,255]),1,1);r.colorSpace=ri,r.needsUpdate=!0;let a={},o={},l={};for(let M of Object.values(Mo))for(let N of[M.top,M.side])a[N]||={value:r};for(let M of Object.keys(Mo))l[M]={value:new ze(1/4,1/4)};let c=(e.shelters||[]).slice(0,3).map(M=>new Nt(...M));for(;c.length<3;)c.push(new Nt(1e6,1e6,-1e6,-1e6));let h={swWet:{value:0},swRain:{value:0},swTime:{value:0},swShelter:{value:c},swSky:{value:new Ce(528926)},swWindow:{value:new Nt(.8,1,.06,1)},swFade:{value:new ze(35,140)}},d=2,u=null,p=!1,x=0,v=0,m=0,f=0,y=!1,T=s.capabilities.getMaxAnisotropy(),g=()=>d>1?Math.min(8,T):Math.min(4,T);function _(M,N){let{kind:D,windows:U,space:H}=n.get(N),L=Mo[D];Object.assign(M.uniforms,h);let W=["#define SW_LEVEL "+(L?d:0)];H==="world"&&W.push("#define SW_WORLD"),U&&W.push("#define SW_WINDOWS"),L&&(M.uniforms.swTop=a[L.top],M.uniforms.swSide=a[L.side],M.uniforms.swScale=l[D],W.push("#define SW_SURFACE","#define SW_ALBEDO "+tr(L.albedo),"#define SW_ROUGH "+tr(L.rough),"#define SW_BUMP "+tr(L.bump),"#define SW_WETK "+tr(H==="world"?L.wet:L.wet*.5),"#define SW_METAL_TOP "+tr(L.metalTop??1),"#define SW_PUDDLE_DRY "+tr(L.puddle?.[0]??-1),"#define SW_PUDDLE_FULL "+tr(L.puddle?.[1]??-2)),L.waterline&&W.push("#define SW_WATERLINE"));let z=W.join(`
`)+`
`;M.vertexShader=z+M.vertexShader.replace("#include <common>",`#include <common>
`+fM).replace("#include <begin_vertex>","#include <begin_vertex>"+pM),M.fragmentShader=z+M.fragmentShader.replace("#include <common>",`#include <common>
`+mM).replace("#include <map_fragment>","#include <map_fragment>"+gM).replace("#include <color_fragment>","#include <color_fragment>"+xM).replace("#include <roughnessmap_fragment>","#include <roughnessmap_fragment>"+bM).replace("#include <metalnessmap_fragment>","#include <metalnessmap_fragment>"+vM).replace("#include <normal_fragment_maps>","#include <normal_fragment_maps>"+_M).replace("#include <emissivemap_fragment>","#include <emissivemap_fragment>"+yM)}function S(M,N="world",D=M?.name){if(!M?.isMeshStandardMaterial||n.has(M)||M.name===uM)return;let U=Mo[D]?D:"",H=zm.has(M.name);!U&&!H||(n.set(M,{kind:U,windows:H,space:N}),M.addEventListener("dispose",()=>n.delete(M)),M.onBeforeCompile=L=>_(L,M),M.customProgramCacheKey=()=>["sw",U,H,N,U?d:0].join("|"),M.needsUpdate=!0)}function A(M,N=!1){if(M?.isMesh)for(let D of Array.isArray(M.material)?M.material:[M.material])S(D,N?"object":"world"),zm.has(D?.name)&&MM(M.geometry,!!D.vertexColors)}let b=M=>e.url?.(M),C=null;async function w(M){let N=await fetch(b(M.file),{signal:t.signal});if(!N.ok)throw Error("Surface texture unavailable");let D=await N.blob();if(p)return;let U=await createImageBitmap(D,{premultiplyAlpha:"none",colorSpaceConversion:"none"});if(p){U.close?.();return}let H=new cn(U);H.wrapS=H.wrapT=Ai,H.colorSpace=ri,H.flipY=!1,H.anisotropy=g(),H.needsUpdate=!0,i.push(H),a[M.id].value=H,m++,x+=D.size,e.onProgress?.(x,v)}function I(){return u||p||!e.url||(u=(async()=>{if(!C){let D=await fetch(b("manifest.json"),{signal:t.signal});if(!D.ok)throw Error("Surface manifest unavailable");C=await D.json();let U=(C.textures||[]).filter(H=>a[H.id]&&/^[a-z-]+\.webp$/.test(H.file)&&H.tile_metres>0);C.entries=U,v=U.reduce((H,L)=>H+(L.bytes||0),0);for(let H of U)o[H.id]=H.tile_metres;for(let[H,L]of Object.entries(Mo))l[H].value.set(1/((o[L.top]||4)/(L.topScale||1)),1/(o[L.side]||4))}let M=C.entries.filter(D=>a[D.id].value===r),N=await Promise.allSettled(M.map(w));f+=N.filter(D=>D.status==="rejected").length})().catch(()=>{f++}).finally(()=>{u=null})),u}return{apply:S,prepare:A,setTier(M){let N=dM[M]??2;if(N!==d){d=N;for(let[D,U]of n)U.kind&&(D.needsUpdate=!0)}for(let D of i)D.anisotropy!==g()&&(D.anisotropy=g(),D.needsUpdate=!0);d>0&&I()},setLighting(M,N,D=0,U=null){U&&h.swSky.value.copy(U).multiplyScalar(1.25);let H=1-M,L=It.smoothstep(H,.1,.9);h.swWindow.value.set(Math.min(.9,.12+.72*L+.12*D),.22+H*.98+Math.min(1,N)*.1,.05,.55+.45*H)},setWeather(M){y=M==="rain",h.swRain.value=y?1:0},update(M,N){let D=y?1:0,U=h.swWet.value;if(!N){h.swWet.value=D;return}let H=Math.min(.1,Math.max(0,M));h.swTime.value+=H,h.swWet.value=U+(D-U)*Math.min(1,H/(D>U?14:70))},progress:()=>({bytes:x,expected:v}),stats:()=>({detail:d,textures:m,failures:f,bytes:x,materials:n.size,wet:+h.swWet.value.toFixed(2),windows:+h.swWindow.value.x.toFixed(2)}),dispose(){p=!0,t.abort();for(let M of i)M.dispose(),M.source.data?.close?.();i.length=0,r.dispose()}}}var Bm="city.window",wM={low:0,medium:1,high:2,ultra:2},SM=s=>wM[s]??2,qc=s=>s>0?s<1?s:1:0,EM=(s,e,t)=>{let n=qc((s-e)/(t-e));return n*n*(3-2*n)};function TM(s,e,t=0){let n=qc(s),i=EM(1-n,.1,.9);return{share:Math.min(.9,.12+.38*i+.1*qc(e)*i+.08*qc(t)),ambient:.012+.1*n,lobby:i,glow:.6+.9*i}}var ia={vertexPars:"varying vec3 vWinPos;varying vec3 vWinNrm;varying vec2 vWinRoom;varying float vWinSeed;",vertexMain:`
#ifdef USE_INSTANCING
mat4 winObject=modelMatrix*instanceMatrix;
#else
mat4 winObject=modelMatrix;
#endif
vWinPos=(winObject*vec4(transformed,1.0)).xyz;
vWinNrm=mat3(winObject)*objectNormal;
// glTF stores V flipped: rows count upwards from the first upper floor again.
vWinRoom=vec2(uv.x,1.0-uv.y);
// One seed per placed building from its world translation, so instanced towers differ.
vec3 winKey=fract(floor(winObject[3].xyz*4.0)*0.1031);
winKey+=dot(winKey,winKey.zyx+31.32);
vWinSeed=fract((winKey.x+winKey.y)*winKey.z);`,fragmentPars:`uniform float winTime;uniform vec4 winState;uniform vec3 winSky;
varying vec3 vWinPos;varying vec3 vWinNrm;varying vec2 vWinRoom;varying float vWinSeed;
float winHash(vec2 p){vec3 q=fract(vec3(p.xyx)*0.1031);q+=dot(q,q.yzx+33.33);return fract((q.x+q.y)*q.z);}
// Box coverage with a smooth edge about one pixel wide (aa = pixel footprint in the same units).
float winBox(vec2 p,vec2 lo,vec2 hi,vec2 aa){vec2 s=smoothstep(lo-aa,lo+aa,p)*(1.0-smoothstep(hi-aa,hi+aa,p));return s.x*s.y;}
// Robot silhouette in metres around its feet: two legs, a torso and a round head.
float winRobot(vec2 r,vec2 aa){float a=max(aa.x,aa.y);return min(1.0,winBox(vec2(abs(r.x),r.y),vec2(0.05,0.0),vec2(0.13,0.78),aa)+winBox(r,vec2(-0.2,0.78),vec2(0.2,1.36),aa)+(1.0-smoothstep(0.13-a,0.13+a,length(r-vec2(0.0,1.52)))));}
// Light on a room surface: lamp light falls off a little with depth, daylight a lot (2/3 at the back).
vec3 winLightAt(vec3 lamp,vec3 fill,float z){return min(lamp*(1.0-0.3*z)+fill*(1.0-0.67*z),vec3(1.2));}`,fragmentMain:`
// Room cell, per-room hashes and the light state (rooms re-roll their light every 20-60 s).
vec2 winCell=floor(vWinRoom),winF=vWinRoom-winCell;
vec2 winAa=max(fwidth(vWinRoom),vec2(1e-4));
vec3 winN=vWinNrm/max(length(vWinNrm),1e-6);
vec3 winV=vWinPos-cameraPosition;winV/=max(length(winV),1e-4);
float winFres=pow(clamp(1.0-abs(dot(winV,winN)),0.0,1.0),5.0);
vec2 winKey=winCell+vWinSeed*vec2(71.3,113.9);
float winH0=winHash(winKey),winH1=winHash(winKey*1.31+3.7),winH2=winHash(winKey*0.71+9.1);
float winLobby=step(winCell.y,-0.5);
#if WIN_LEVEL>0
float winEpoch=floor(winTime/(20.0+40.0*winH0)+winH0);
#else
float winEpoch=0.0;
#endif
float winOn=max(step(winHash(winKey+winEpoch*vec2(3.1,1.7)+vec2(5.3,2.9)),winState.x),winLobby*step(0.5,winState.z));
vec3 winWall=mix(vec3(0.62,0.56,0.48),vec3(0.40,0.48,0.55),step(0.55,winH1));
vec3 winLamp=mix(vec3(1.0,0.8,0.56),vec3(0.8,0.9,1.0),step(0.72,winH1))*mix(vec3(1.0),vec3(1.0,0.92,0.78),winLobby);
// Light budget (linear radiance before tone mapping): lit walls stay below about 0.85, only the
// ceiling panel (1.2) and screens and visors (1.15) pass the bloom threshold. Daylight (read back
// from the unlit ambient) fills every room, brightest at the glass and a third of it at the back.
float winDusk=clamp((winState.w-0.6)*1.1111,0.0,1.0);
float winDay=clamp((winState.y-0.012)*10.0,0.0,1.0);
vec3 winLampL=winLamp*(mix(0.08,0.7,winDusk)*winOn);
vec3 winFill=vec3(0.94,1.0,1.06)*(0.012+0.26*winDay);
float winAlive=max(winOn,winDay*step(fract(winH0*7.7),0.45));
vec2 winS=vec2(WIN_BAY,WIN_STOREY);
vec3 winRoomCol;
#if WIN_LEVEL>0
// Tangent frame and metres per room from screen-space derivatives; an analytic frame (horizontal
// tangent, rows upwards, default room) replaces them when they are degenerate or implausible.
vec3 winDpx=dFdx(vWinPos),winDpy=dFdy(vWinPos);
vec2 winDux=dFdx(vWinRoom),winDuy=dFdy(vWinRoom);
float winDet=winDux.x*winDuy.y-winDux.y*winDuy.x;
float winOk=step(1e-12,abs(winDet));
float winDiv=mix(1.0,winDet,winOk);
vec3 winPu=(winDuy.y*winDpx-winDux.y*winDpy)/winDiv,winPv=(winDux.x*winDpy-winDuy.x*winDpx)/winDiv;
float winLu=length(winPu),winLv=length(winPv);
winOk*=step(0.25,winLu)*step(winLu,40.0)*step(0.25,winLv)*step(winLv,40.0);
vec3 winC=cross(vec3(0.0,1.0,0.0),winN);
vec3 winT=mix(winC/max(length(winC),1e-4),winPu/max(winLu,1e-4),winOk);
vec3 winB=mix(vec3(0.0,1.0,0.0),winPv/max(winLv,1e-4),winOk);
// Derived sizes jitter per 2x2 pixel quad (stair-steps, moire); within 15 % of the kit's room size
// they snap to it, so only genuinely rescaled buildings keep the measured size.
vec2 winSm=vec2(winLu,winLv);
winSm=mix(winSm,winS,step(abs(winSm-winS),winS*0.15));
winS=mix(winS,winSm,winOk);
// View ray into a box room one cell wide, one storey high and WIN_DEPTH deep.
vec3 winD=vec3(dot(winV,winT),dot(winV,winB),max(-dot(winV,winN),0.02));
vec3 winSize=vec3(winS,WIN_DEPTH),winP=vec3(winF*winS,0.0);
vec3 winT3=abs(step(0.0,winD)*winSize-winP)/max(abs(winD),vec3(1e-4));
float winHit=min(min(winT3.x,winT3.y),winT3.z);
vec3 winQ=clamp((winP+winD*winHit)/winSize,0.0,1.0);
float winBack=step(winT3.z,min(winT3.x,winT3.y));
float winSide=(1.0-winBack)*step(winT3.x,winT3.y);
float winFlr=(1.0-winBack-winSide)*step(winD.y,0.0);
float winCeil=1.0-winBack-winSide-winFlr;
vec3 winAlb=winWall*(winBack+0.72*winSide+0.9*winCeil)+vec3(0.22,0.2,0.21)*winFlr;
vec2 winAq=min(max(fwidth(winQ.xz),vec2(1e-3)),vec2(0.2));
float winPanel=winCeil*winBox(winQ.xz,vec2(0.28,0.3),vec2(0.72,0.62),winAq*0.6);
winRoomCol=winAlb*winLightAt(winLampL,winFill,winQ.z)+winPanel*winLamp*winOn*mix(0.5,1.2,winDusk);
float winAh=max(fwidth(winHit),1e-3);
// Furniture at 55 % depth: a desk or counter, sometimes a tall shelf.
float winTf=0.55*WIN_DEPTH/winD.z;
vec2 winPf=(winP+winD*winTf).xy;
float winX0=(0.12+0.3*winH1)*winS.x,winX1=winX0+(0.3+0.2*fract(winH1*7.0))*winS.x;
vec2 winAf=min(max(fwidth(winPf),vec2(1e-3)),vec2(0.2))*0.6;
float winFurn=min(1.0,winBox(winPf,vec2(winX0,0.0),vec2(winX1,0.76),winAf)+step(0.5,fract(winH1*13.0))*winBox(winPf,vec2(winS.x-0.42,0.0),vec2(winS.x-0.1,1.9),winAf))*smoothstep(winTf-winAh,winTf+winAh,winHit);
winRoomCol=mix(winRoomCol,winWall*0.3*winLightAt(winLampL,winFill,0.55),winFurn);
#if WIN_LEVEL>1
// Life at 30 % depth in a share of the lit rooms: a robot walking across or standing and bobbing
// (two in lobbies), and a flickering screen on the back wall of offices.
float winTl=0.3*WIN_DEPTH/winD.z;
vec2 winPl=(winP+winD*winTl).xy;
float winWalk=step(0.5,fract(winH2*5.3));
float winX=mix(0.2+0.6*fract(winH2*29.0),0.5+0.36*sin(winTime*(0.25+0.25*fract(winH2*17.0))+winH2*40.0),winWalk)*winS.x;
float winBob=0.025*sin(winTime*2.6+winH2*30.0)*(1.0-winWalk);
vec2 winR1=vec2(winPl.x-winX,winPl.y-winBob),winR2=vec2(winPl.x-winS.x+winX,winPl.y+winBob);
vec2 winAl=min(max(fwidth(winPl),vec2(1e-3)),vec2(0.2))*0.6;
float winBots=max(winRobot(winR1,winAl),winRobot(winR2,winAl)*winLobby)*winAlive*step(winH2,mix(0.4,0.85,winLobby))*smoothstep(winTl-winAh,winTl+winAh,winHit);
float winVisor=max(winBox(winR1,vec2(-0.09,1.49),vec2(0.09,1.55),winAl),winBox(winR2,vec2(-0.09,1.49),vec2(0.09,1.55),winAl)*winLobby)*winBots*winOn;
vec2 winSp=vec2((winQ.x-0.5)*winS.x,winQ.y*winS.y);
float winScreen=winOn*winBack*(1.0-winLobby)*(1.0-winFurn)*step(fract(winH2*3.1),0.35)*winBox(winSp,vec2(-0.5,1.0),vec2(0.5,1.6),min(max(fwidth(winSp),vec2(1e-3)),vec2(0.2))*0.6);
winRoomCol+=winScreen*vec3(0.3,0.7,1.0)*1.15*(0.8+0.2*sin(winTime*9.0+winH2*50.0)*sin(winTime*2.3+winH2*20.0));
winRoomCol=mix(winRoomCol,vec3(0.03,0.035,0.045)+0.12*winLightAt(winLampL,winFill,0.3)*vec3(0.6,0.65,0.7),winBots)+winVisor*vec3(0.2,0.9,1.0)*1.15;
#endif
// Blinds in about one room in five, drawn on the glass; slats fade out when they get sub-pixel.
float winBlindTop=0.7-0.6*fract(winH2*41.0);
float winBlind=step(fract(winH1*23.0),0.2)*smoothstep(winBlindTop-winAa.y,winBlindTop+winAa.y,winF.y);
// Slats: anti-aliased triangle wave; they fade to their average colour below about two pixels.
float winSlatPx=winAa.y*WIN_STOREY*12.0;
float winSlatW=smoothstep(0.5-min(winSlatPx,0.5),0.5+min(winSlatPx,0.5),abs(fract(winF.y*WIN_STOREY*12.0)-0.5)*2.0);
float winSlat=mix(0.8+0.2*winSlatW,0.9,smoothstep(0.12,0.25,winSlatPx));
winRoomCol=mix(winRoomCol,vec3(0.55,0.5,0.42)*winSlat*winLightAt(winLampL,winFill,0.0)*0.8,winBlind);
#else
// Low tier: a flat lit or unlit cell, brighter under the ceiling; no parallax, no motion.
winRoomCol=winWall*winLightAt(winLampL,winFill,0.4)*0.8+winLamp*winOn*mix(0.1,0.2,winDusk)*smoothstep(0.35,1.0,winF.y);
#endif
winRoomCol*=1.0-winFres;
// A faint frame (mullions on the cell's vertical edges, a spandrel at its foot) so the lowest LOD,
// which has no fins or slabs, still reads as a facade.
vec2 winM=vec2(0.06,0.3)/winS;
float winFrame=max(1.0-smoothstep(winM.x-winAa.x,winM.x+winAa.x,min(winF.x,1.0-winF.x)),1.0-smoothstep(winM.y-winAa.y,winM.y+winAa.y,winF.y));
vec3 winGlow=(max(winRoomCol,vec3(0.0))+winSky*(0.04+0.6*winFres))*(1.0-winFrame);
diffuseColor.rgb=mix(diffuseColor.rgb,vec3(0.035,0.042,0.05),winFrame);`,roughness:`
roughnessFactor=mix(roughnessFactor,0.55,winFrame);`,emissive:`
totalEmissiveRadiance+=winGlow;`};function Hm(){let s={winTime:{value:0},winState:{value:new Nt(.12,.112,0,.6)},winSky:{value:new Ce(528926)}},e=2,t="high",n=!1,i=0,r=!1,a=new An({name:Bm,color:new Ce(.018,.032,.045),roughness:.08,metalness:0,vertexColors:!0});return a.onBeforeCompile=o=>{Object.assign(o.uniforms,s);let l=`#define WIN_LEVEL ${e}
#define WIN_DEPTH 3.0
#define WIN_BAY 1.9
#define WIN_STOREY 3.05
`;o.vertexShader=l+o.vertexShader.replace("#include <common>",`#include <common>
`+ia.vertexPars).replace("#include <begin_vertex>","#include <begin_vertex>"+ia.vertexMain),o.fragmentShader=l+o.fragmentShader.replace("#include <common>",`#include <common>
`+ia.fragmentPars).replace("#include <color_fragment>","#include <color_fragment>"+ia.fragmentMain).replace("#include <roughnessmap_fragment>","#include <roughnessmap_fragment>"+ia.roughness).replace("#include <emissivemap_fragment>","#include <emissivemap_fragment>"+ia.emissive)},a.customProgramCacheKey=()=>"sw-windows|"+e,{material:a,uniforms:s,owns:o=>o===a,adopt(o){if(!o?.isMesh||r)return!1;let l=Array.isArray(o.material)?o.material:[o.material],c=!1,h=l.map(d=>d===a||d?.name!==Bm?d:(c=!0,d.dispose(),a));return c?(o.material=Array.isArray(o.material)?h:h[0],i++,!0):!1},setLighting(o,l,c=0,h=null){let d=TM(o,l,c);s.winState.value.set(d.share,d.ambient,d.lobby,d.glow),h&&s.winSky.value.copy(h)},setTier(o){t=o;let l=SM(o);l!==e&&(e=l,a.needsUpdate=!0)},setReducedMotion(o){n=!!o},update(o,l){!l||n||r||(s.winTime.value=(s.winTime.value+Math.min(.1,Math.max(0,o||0)))%7200)},stats:()=>({level:e,tier:t,reduced:n,meshes:i,share:+s.winState.value.x.toFixed(2),lobby:+s.winState.value.z.toFixed(2),glow:+s.winState.value.w.toFixed(2),time:+s.winTime.value.toFixed(1)}),dispose(){r=!0,a.dispose()}}}function AM(s){let e=Math.max(64,Math.round(s.sampleRate*.03)||1440),t=[],n=0;for(let a=0;a<s.numberOfChannels;a++){let o=s.getChannelData(a);for(let l=0;l<o.length;l+=e){let c=0,h=Math.min(l+e,o.length);for(let d=l;d<h;d++){let u=o[d];if(!Number.isFinite(u))throw Error("Invalid voice");n=Math.max(n,Math.abs(u)),c+=u*u}t.push(Math.sqrt(c/(h-l)))}}if(n<1e-4)throw Error("Silent voice");let i=t.filter(a=>a>Math.max(.003,n*.003)).sort((a,o)=>a-o),r=i[Math.floor((i.length-1)*.7)]||n*.5;return Math.min(8,Math.max(1/n,.26/Math.max(r,.001)))}function Gm(s,e){let t=!1,n=!1,i=0,r=null,a=null,o=[],l=null,c=null,h=null,d=250,u=0,p=0,x=0,v="idle",m=null,f=()=>.04+.46/(1+(d/110)**2);function y(){let b=s.currentTime;l?.gain.setTargetAtTime(f(),b,.18),c?.frequency.setTargetAtTime(1800+4400/(1+d/95),b,.18),h?.pan.setTargetAtTime(u,b,.18)}function T(){if(a){a.onended=null;try{a.stop()}catch{}a=null}o.forEach(b=>b.disconnect()),o=[],l=c=h=null}function g(){p++,clearTimeout(i),i=0,r?.abort(),r=null,T(),v="idle"}function _(b=2e4+Math.random()*2e4){clearTimeout(i),t&&!n&&(i=setTimeout(()=>{i=0,A()},b))}function S(){if(m)return m;m=s.createBuffer(2,Math.ceil(s.sampleRate*.85),s.sampleRate);let b=42;for(let C=0;C<2;C++){let w=m.getChannelData(C),I=Math.round(s.sampleRate*.015),M=0;for(let D=0;D<w.length;D++){b=Math.imul(b,1664525)+1013904223>>>0;let U=(D-I)/(w.length-I);w[D]=D<I?0:(b/4294967296*2-1)*Math.pow(1-U,2.25),M+=w[D]*w[D]}let N=.65/Math.sqrt(M||1);for(let D=0;D<w.length;D++)w[D]*=N}return m}async function A(){if(!t||n||r||a||s.state!=="running"){_();return}let b=p,C=new AbortController;r=C,v="loading";let w=setTimeout(()=>C.abort(),32e3);try{let I=await fetch("/api/desktop/system-world/voice",{method:"POST",credentials:"same-origin",cache:"no-store",signal:C.signal});if(I.status===204){v="idle";return}if(!I.ok||!I.headers.get("Content-Type")?.startsWith("audio/"))throw Error("Voice unavailable");if(Number(I.headers.get("Content-Length"))>8*1024*1024)throw Error("Voice too large");let M=await I.arrayBuffer();if(M.byteLength>8*1024*1024)throw Error("Voice too large");if(b!==p||!t||n)return;let N=await s.decodeAudioData(M);if(b!==p||!t||n)return;if(!Number.isFinite(N.duration)||N.duration<=0||N.duration>25||N.numberOfChannels>2)throw Error("Invalid voice");let D=AM(N),U=Ne=>(o.push(Ne),Ne);a=U(s.createBufferSource()),a.buffer=N;let H=U(s.createGain()),L=U(s.createBiquadFilter());H.gain.value=D,L.type="highpass",L.frequency.value=90,c=U(s.createBiquadFilter()),c.type="lowpass",c.Q.value=.55;let W=U(s.createGain()),z=U(s.createGain()),E=U(s.createGain()),J=U(s.createGain());W.gain.value=.76,z.gain.value=.22,E.gain.value=.28;let q=U(s.createDelay(.2)),te=U(s.createConvolver());q.delayTime.value=.085,te.normalize=!1,te.buffer=S();let Le=U(s.createWaveShaper()),Fe=new Float32Array(1024);for(let Ne=0;Ne<Fe.length;Ne++)Fe[Ne]=Math.max(-.75,Math.min(.75,Ne*2/(Fe.length-1)-1));Le.curve=Fe,l=U(s.createGain()),l.gain.value=f(),h=U(s.createStereoPanner()),h.pan.value=u,a.connect(H).connect(L).connect(c),c.connect(W).connect(J),c.connect(q).connect(z).connect(J),c.connect(te).connect(E).connect(J),J.connect(Le).connect(l).connect(h).connect(e),y(),v="speaking",x++,a.onended=()=>{b!==p||!t||n||(v="tail",i=setTimeout(()=>{T(),v="idle",_()},900))},a.start()}catch{b===p&&!n&&(v=C.signal.aborted?"idle":"unavailable")}finally{clearTimeout(w),r===C&&(r=null),b===p&&!n&&!a&&_(v==="unavailable"?6e4:void 0)}}return{setActive(b){t===b||n||(t=b,t?_(3e3+Math.random()*2e3):g())},setListener(b,C,w,I,M){if(!Number.isFinite(b)||!Number.isFinite(C)||!Number.isFinite(w)||!Number.isFinite(I)||!Number.isFinite(M))return;let N=-b,D=-12-w,U=Math.max(0,Math.hypot(N,D)-13);d=Math.hypot(U,Math.max(0,C-87,-C));let H=Math.hypot(N,D),L=Math.hypot(I,M);u=H&&L?Math.max(-.8,Math.min(.8,(D*I-N*M)/H/L))*.8:0,y()},stats(){return{state:v,phrases:x,distance:d,gain:f(),pan:u,pending:!!r,scheduled:!!i}},dispose(){n||(n=!0,t=!1,g(),m=null)}}}function Vm(s,e,t=e){let n=new Set,i={x:0,z:0,fx:0,fz:-1},r=!1,a=!1,o="clear",l=s.createBuffer(1,s.sampleRate*2,s.sampleRate),c=31,h=l.getChannelData(0);for(let f=0;f<h.length;f++)c=Math.imul(c,1664525)+1013904223|0,h[f]=(c>>>0)/2147483648-1;let d=s.createBufferSource(),u=s.createBiquadFilter(),p=s.createGain();d.buffer=l,d.loop=!0,u.type="lowpass",u.frequency.value=450,p.gain.value=0,d.connect(u).connect(p).connect(t),d.start();let x=[{x:-57,z:-57,f:94,g:.055},{x:43,z:62,f:143,g:.035},{x:0,z:14,f:220,g:.016},{x:0,z:79,f:340,g:.06}].map((f,y)=>{let T=y===3?s.createBufferSource():s.createOscillator(),g=s.createGain(),_=s.createStereoPanner(),S=s.createBiquadFilter();return y===3?(T.buffer=l,T.loop=!0):(T.type="sine",T.frequency.value=f.f),S.type="lowpass",S.frequency.value=f.f*2,g.gain.value=0,T.connect(S).connect(g).connect(_).connect(t),T.start(),{...f,source:T,level:g,pan:_,low:S}});function v(){p.gain.setTargetAtTime(r?(o==="rain"?.07:.015)*(a?.12:1):0,s.currentTime,.1),u.frequency.setTargetAtTime(a?280:o==="rain"?2100:600,s.currentTime,.2)}function m(){for(let f of[...n]){try{f.source.stop()}catch{}f.release()}}return{setActive(f){if(r=f,v(),!f){m();for(let y of x)y.level.gain.setTargetAtTime(0,s.currentTime,.05)}},environment(f,y){a=f,y&&(o=y),v()},listener(f,y,T,g,_){Object.assign(i,{x:f,z:T,fx:g,fz:_});for(let S of x){let A=Math.hypot(f-S.x,T-S.z);S.level.gain.setTargetAtTime(r?S.g/(1+A*A*.025)*(a?.25:1):0,s.currentTime,.1),S.pan.pan.value=Math.max(-1,Math.min(1,((S.x-f)*-_+(S.z-T)*g)/Math.max(1,A)))}},play(f,y=0,T=0,g=0){if(!r||n.size>=16)return;let _=Math.hypot(y-i.x,g-i.z);if(_>90)return;let S={step:[.12,240,.16],step_inside:[.1,550,.12],door:[.7,850,.06],lift:[1.5,120,.05],tram:[.8,330,.08],discover:[.45,880,.08],hover:[1.2,160,.07]}[f];if(!S)return;let[A,b,C]=S,w=f.startsWith("step")||f==="door"||f==="hover"?s.createBufferSource():s.createOscillator();"buffer"in w?w.buffer=l:(w.type="sine",w.frequency.value=b);let I=s.createBiquadFilter(),M=s.createGain(),N=s.createStereoPanner();I.type="lowpass",I.frequency.value=a?b*.7:b,M.gain.setValueAtTime(0,s.currentTime),M.gain.linearRampToValueAtTime(C/(1+_*.08),s.currentTime+.015),M.gain.exponentialRampToValueAtTime(1e-4,s.currentTime+A),N.pan.value=Math.max(-1,Math.min(1,((y-i.x)*-i.fz+(g-i.z)*i.fx)/Math.max(_,1))),w.connect(I).connect(M).connect(N).connect(e);let D={source:w,release(){if(n.delete(D))for(let U of[w,I,M,N])U.disconnect()}};n.add(D),w.onended=D.release,w.start(),w.stop(s.currentTime+A+.02)},stats:()=>({voices:n.size,weather:o,inside:a}),dispose(){r=!1,m(),d.stop();for(let f of x){f.source.stop();for(let y of[f.source,f.level,f.pan,f.low])y.disconnect()}for(let f of[d,u,p])f.disconnect()}}}function Wm(s,e,t=e){let o=new Set,l=new Map,c=new Map,h={x:0,y:0,z:0,fx:0,fz:-1},d={x:0,z:0},u=!1,p=!1,x=0,v=0,m=0,f={kind:0,inactive:0,distant:0,pool:0,retrigger:0,quiet:0},y=s.createBuffer(1,s.sampleRate*2,s.sampleRate),T=9371,g=y.getChannelData(0);for(let C=0;C<g.length;C++)T=Math.imul(T,1664525)+1013904223|0,g[C]=(T>>>0)/2147483648-1;let _={step:{gain:.07,layers:[{wave:"noise",f:1900,q:1.7,dur:.05,lvl:.5},{wave:"noise",f:520,q:.9,dur:.1,lvl:.4},{wave:"tone",f:104,f2:74,dur:.12,lvl:.3}]},step_carry:{gain:.075,layers:[{wave:"noise",f:1250,q:1.2,dur:.06,lvl:.45},{wave:"noise",f:340,q:.8,dur:.13,lvl:.5},{wave:"tone",f:88,f2:62,dur:.15,lvl:.36}]},glide:{gain:.028,layers:[{wave:"noise",f:700,q:.6,dur:.26,lvl:.8},{wave:"tone",f:150,f2:120,dur:.24,lvl:.18}]},greet:{gain:.05,layers:[{wave:"tone",f:520,f2:740,dur:.11,lvl:.5,type:"triangle"},{wave:"tone",f:790,f2:960,dur:.13,lvl:.3,type:"triangle",delay:.1}]},work:{gain:.045,layers:[{wave:"noise",f:2700,q:2.6,dur:.05,lvl:.6},{wave:"tone",f:240,f2:175,dur:.22,lvl:.26,type:"sawtooth"}]},carry:{gain:.034,layers:[{wave:"noise",f:950,q:.7,dur:.12,lvl:.7},{wave:"noise",f:2300,q:1.5,dur:.07,lvl:.25}]},idle:{gain:.02,layers:[{wave:"noise",f:3400,q:3.2,dur:.028,lvl:.7}]},turn:{gain:.026,layers:[{wave:"tone",f:900,f2:1440,dur:.18,lvl:.4,type:"sawtooth"}]}};function S(C,w,I,M,N){let D=C.startsWith("robot_")?C.slice(6):C,U=_[D];if(!U)return f.kind++,!1;if(!u)return f.inactive++,!1;let H=w-d.x,L=M-d.z,W=Math.hypot(H,L);if(W>48)return f.distant++,!1;if(o.size>=10){let ue=null;for(let ce of o)(!ue||ce.distance>ue.distance)&&(ue=ce);if(!ue||W>=ue.distance)return f.pool++,!1;ue.release()}let z=s.currentTime;if(N!=null){let ue=c.get(N);if(ue!=null&&z-ue<.25)return f.retrigger++,!1;c.set(N,z)}let E=Math.abs(I-h.y),J=U.gain*4.5/(1+W*.12)/(1+E*.0045)*(p?.5:1);if(J<.002)return f.quiet++,!1;let q=w-h.x,te=M-h.z,Le=Math.max(-1,Math.min(1,(q*-h.fz+te*h.fx)/Math.max(1,Math.hypot(q,te)))),Fe=s.createGain(),Ne=s.createStereoPanner(),se=s.createGain();Fe.gain.value=J,Ne.pan.value=Le,se.gain.value=p?.14:.26;let oe=s.createBiquadFilter();oe.type="lowpass",oe.Q.value=.6,oe.frequency.value=Math.max(700,Math.min(11e3,11e3-W*130)),oe.connect(Fe),Fe.connect(Ne).connect(e),Ne.connect(se).connect(t);let Pe=[oe],ge={distance:W,release(){if(o.delete(ge)){for(let ue of Pe){try{ue.stop?.()}catch{}try{ue.disconnect()}catch{}}for(let ue of[Fe,Ne,se])try{ue.disconnect()}catch{}}}};o.add(ge);let xe=null,$=0;for(let ue of U.layers){let ce=z+(ue.delay||0),ie=Math.max(ue.dur,(ue.attack||.006)+.02),de=s.createGain();de.gain.setValueAtTime(0,ce),de.gain.linearRampToValueAtTime(ue.lvl,ce+(ue.attack||.006)),de.gain.exponentialRampToValueAtTime(1e-4,ce+ie),de.connect(oe);let ve;if(ue.wave==="noise"){ve=s.createBufferSource(),ve.buffer=y,ve.loop=!0;let Ve=s.createBiquadFilter();Ve.type="bandpass",Ve.frequency.value=ue.f,Ve.Q.value=ue.q||1,ve.connect(Ve).connect(de),Pe.push(Ve)}else ve=s.createOscillator(),ve.type=ue.type||"sine",ve.frequency.setValueAtTime(ue.f,ce),ue.f2&&ve.frequency.exponentialRampToValueAtTime(Math.max(20,ue.f2),ce+ie),ve.connect(de);Pe.push(ve,de);let We=ce+ie+.02;We>$&&($=We,xe=ve),ve.start(ce),ve.stop(We)}return xe&&(xe.onended=ge.release),x++,J>m&&(m=J),!0}function A(C,w,I,M,N,D,U){if(!u)return;let H=l.get(C);if(!H){l.set(C,{x:w,z:M,phase:0});return}let L=w-H.x,W=M-H.z,z=Math.hypot(L,W);if(H.x=w,H.z=M,!N){H.phase=0;return}if(z>8)return;H.phase+=z;let E=U==="glide"?1.6:D?1.05:.8;H.phase<E||(H.phase-=E,S(U==="glide"?"glide":D?"step_carry":"step",w,I,M,C),v++)}function b(){for(let C of[...o])C.release();for(let[,C]of l)C.x=h.x,C.z=h.z,C.phase=0;c.clear()}return{setActive(C){u!==C&&(u=C,C||b())},environment(C){p=!!C},listener(C,w,I,M,N,D,U){Object.assign(h,{x:C,y:w,z:I,fx:M,fz:N}),d.x=D??C,d.z=U??I},action(C,w,I,M,N){return S(C,w,I,M,N)},gait:A,reset(){l.clear(),c.clear()},stats:()=>({voices:o.size,played:x,steps:v,gaits:l.size,inside:p,peak:+m.toFixed(4),rejected:{...f}}),dispose(){u=!1,b()}}}var dd=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]],qm=[440,523.25,587.33,659.25,783.99,880,1046.5],is=12,Xc={harbour:{x:0,z:79},quay:{x:0,z:72},tower:{x:0,z:-12},square:{x:8,z:40}},RM=[{kind:"chime",weight:15},{kind:"swoosh",weight:13},{kind:"gust",weight:7},{kind:"birds",weight:8,when:s=>s.day>.45&&s.weather==="clear"},{kind:"gull",weight:6,when:s=>s.day>.2&&s.weather==="clear"},{kind:"pigeon",weight:4,when:s=>s.day>.6&&!s.busy},{kind:"cricket",weight:6,when:s=>s.day<.25&&s.weather!=="rain"},{kind:"horn",weight:4,when:s=>s.day<.45&&s.weather!=="rain"},{kind:"bell",weight:3,when:s=>s.day<.7},{kind:"hammer",weight:4,when:s=>s.day>.3},{kind:"drip",weight:10,when:s=>s.weather==="rain"},{kind:"windchime",weight:4,when:s=>s.weather==="clear"&&!s.busy},{kind:"siren",weight:2,when:s=>s.evening>.3&&s.busy}];function CM(s,e,t){let n=Math.floor(s.sampleRate*e),i=s.createBuffer(2,n,s.sampleRate),r=977;for(let a=0;a<2;a++){let o=i.getChannelData(a);for(let l=0;l<n;l++)r=Math.imul(r,1664525)+1013904223|0,o[l]=((r>>>0)/2147483648-1)*Math.pow(1-l/n,t)}return i}function PM(s=()=>{}){let e=null,t=null,n=null,i=!1,r=!1,a=!1,o=0,l=null,c=[],h=[],d=new Float32Array(256),u=new Set,p=.18,x=null,v=null,m=null,f=null,y=null,T=!1,g="clear",_=[],S=null,A=null,b=null,C=null,w=null,I=null,M=null,N=null,D=null,U="0.000",H="0.000",L="0.000",W=0,z=0,E=0,J=0,q={x:0,y:2.4,z:0,fx:0,fz:-1,origin:{x:0,z:0}},te={busy:!1,day:0,evening:0,weather:"clear"},Le={ambience:1,effects:1,voice:1};try{a=localStorage.getItem("aurago.desktop.sysworld.sound")==="true"}catch{}function Fe(){if(e||i)return;e=new(window.AudioContext||window.webkitAudioContext);let V=St=>(c.push(St),St),Y=St=>(h.push(St),V(St));t=V(e.createGain()),t.gain.value=0,n=V(e.createAnalyser()),n.fftSize=512,t.connect(n),n.connect(e.destination),m=V(e.createGain()),m.gain.value=Le.ambience,m.connect(t),f=V(e.createGain()),f.gain.value=Le.effects,f.connect(t),y=V(e.createGain()),y.gain.value=Le.voice,y.connect(t),w=V(e.createConvolver()),w.buffer=CM(e,3.4,2.6);let he=V(e.createGain());he.gain.value=.55,w.connect(he).connect(m),l=Gm(e,y),x=Vm(e,f,m),x.environment(T,g),v=Wm(e,f,m),v.environment(T),v.listener(q.x,q.y,q.z,q.fx,q.fz,q.origin.x,q.origin.z);for(let[St,Mn]of[[55,.07],[82.4069,.022],[110,.01]]){let Vt=Y(e.createOscillator()),nn=V(e.createGain());Vt.frequency.value=St,nn.gain.value=Mn,Vt.connect(nn).connect(m),Vt.start()}S=V(e.createBiquadFilter()),S.type="lowpass",S.frequency.value=650,S.Q.value=.6,A=V(e.createGain()),A.gain.value=.042;let _e=V(e.createGain());_e.gain.value=.9,S.connect(A).connect(m),A.connect(_e).connect(w),_=dd[0].map(St=>[-6,6].map(Mn=>{let Vt=Y(e.createOscillator()),nn=V(e.createGain());return Vt.type="sawtooth",Vt.frequency.value=St,Vt.detune.value=Mn,nn.gain.value=.22,Vt.connect(nn).connect(S),Vt.start(),Vt}));let me=Y(e.createOscillator()),ae=V(e.createGain());me.frequency.value=.05,ae.gain.value=180,me.connect(ae).connect(S.frequency),me.start();let Se=e.createBuffer(1,e.sampleRate*4,e.sampleRate),Me=Se.getChannelData(0),Ie=0;for(let St=0;St<Me.length;St++)Ie=(Ie+(Math.random()*2-1)*.018)/1.018,Me[St]=Ie;let j=Me[Me.length-1]-Me[0];for(let St=0;St<Me.length;St++)Me[St]-=j*St/(Me.length-1);let Te=Y(e.createBufferSource());b=V(e.createBiquadFilter()),C=V(e.createGain()),Te.buffer=Se,Te.loop=!0,b.type="lowpass",b.frequency.value=680,b.Q.value=.25,C.gain.value=.32,Te.connect(b).connect(C).connect(m),Te.start();let we=Y(e.createOscillator()),Oe=V(e.createGain());we.frequency.value=.075,Oe.gain.value=160,we.connect(Oe).connect(b.frequency),we.start(),I=e.createBuffer(1,e.sampleRate*2,e.sampleRate);let ke=I.getChannelData(0);for(let St=0;St<ke.length;St++)ke[St]=Math.random()*2-1;let Ae=Y(e.createBufferSource());Ae.buffer=I,Ae.loop=!0;let Ye=V(e.createBiquadFilter());Ye.type="bandpass",Ye.frequency.value=620,Ye.Q.value=.5;let Qe=Y(e.createOscillator()),P=V(e.createGain());Qe.frequency.value=.08,P.gain.value=140,Qe.connect(P).connect(Ye.frequency),Qe.start(),M=V(e.createGain()),M.gain.value=0,Ae.connect(Ye).connect(M).connect(m),Ae.start();let B=Y(e.createBufferSource());B.buffer=I,B.loop=!0;let K=V(e.createBiquadFilter());K.type="bandpass",K.frequency.value=320,K.Q.value=.7;let ee=Y(e.createOscillator()),X=V(e.createGain());ee.frequency.value=.13,X.gain.value=90,ee.connect(X).connect(K.frequency),ee.start(),N=V(e.createGain()),N.gain.value=0,B.connect(K).connect(N).connect(m),B.start();let Z=Y(e.createOscillator()),Ee=Y(e.createOscillator());Z.type="sine",Z.frequency.value=118,Ee.type="sine",Ee.frequency.value=236;let Be=V(e.createGain());Be.gain.value=.26;let st=V(e.createGain());st.gain.value=.9;let Je=Y(e.createOscillator()),kt=V(e.createGain());Je.frequency.value=.19,kt.gain.value=.08,Je.connect(kt).connect(st.gain),Je.start();let Bt=V(e.createBiquadFilter());Bt.type="lowpass",Bt.frequency.value=420,Bt.Q.value=.4,D=V(e.createGain()),D.gain.value=0,Z.connect(st),Ee.connect(Be).connect(st),st.connect(Bt).connect(D).connect(m),Z.start(),Ee.start(),be(),De()}let Ne=()=>!!e&&a&&r&&!i&&p>0;function se(V){let Y={release(){if(u.delete(Y))for(let he of V){try{he.stop?.()}catch{}try{he.disconnect()}catch{}}}};return u.add(Y),V[0].onended=Y.release,J++,Y}function oe(V,Y,he){let _e=e.createStereoPanner(),me=e.createGain();return _e.pan.value=Y,me.gain.value=he,V.connect(_e).connect(m),_e.connect(me).connect(w),[_e,me]}function Pe(V){let Y=V.x-q.x,he=V.z-q.z,_e=V.x-q.origin.x,me=V.z-q.origin.z,ae=Math.hypot(_e,me),Se=Math.max(1,Math.hypot(Y,he));return{distance:ae,pan:Math.max(-1,Math.min(1,(Y*-q.fz+he*q.fx)/Se)),fade:1/(1+ae*.02)}}function ge(V){if(V==="horn"||V==="gull")return Xc.harbour;if(V==="windchime")return Xc.quay;if(V==="bell")return Xc.tower;if(V==="pigeon")return Xc.square;let Y=Math.random()*Math.PI*2,he=24+Math.random()*44;return{x:q.origin.x+Math.sin(Y)*he,z:q.origin.z+Math.cos(Y)*he}}function xe(V){let Y=e.currentTime,he=qm[Math.floor(Math.random()*qm.length)],_e=e.createOscillator(),me=e.createOscillator(),ae=e.createGain(),Se=e.createGain(),{pan:Me,fade:Ie}=Pe(V);_e.frequency.value=he,me.frequency.value=he*2.76,ae.gain.value=.22,Se.gain.setValueAtTime(0,Y),Se.gain.linearRampToValueAtTime(te.busy?.02:.014,Y+.008),Se.gain.exponentialRampToValueAtTime(1e-4,Y+2.8),_e.connect(Se),me.connect(ae).connect(Se),se([_e,me,ae,Se,...oe(Se,Me,1.4)]),_e.start(Y),me.start(Y),_e.stop(Y+2.9),me.stop(Y+2.9)}function $(V){let Y=e.currentTime,he=e.createBufferSource(),_e=e.createBiquadFilter(),me=e.createGain(),{pan:ae,fade:Se}=Pe(V);he.buffer=I,he.loop=!0,_e.type="bandpass",_e.Q.value=1.4,_e.frequency.setValueAtTime(260,Y),_e.frequency.exponentialRampToValueAtTime(1100,Y+1.6),_e.frequency.exponentialRampToValueAtTime(380,Y+3.4),me.gain.setValueAtTime(0,Y),me.gain.linearRampToValueAtTime(.03*Se,Y+1.6),me.gain.exponentialRampToValueAtTime(1e-4,Y+3.6),he.connect(_e).connect(me);let[Me,Ie]=oe(me,ae,.5),j=Math.max(-1,ae-.6),Te=Math.min(1,ae+.6);Me.pan.setValueAtTime(j,Y),Me.pan.linearRampToValueAtTime(Te,Y+3.4),se([he,_e,me,Me,Ie]),he.start(Y),he.stop(Y+3.7)}function ue(){let V=e.currentTime,Y=g==="rain"?.36:.32;b.frequency.cancelScheduledValues(V),C.gain.cancelScheduledValues(V),b.frequency.setTargetAtTime(1200+Math.random()*700,V,.8),b.frequency.setTargetAtTime(680,V+2.6,1.4),C.gain.setTargetAtTime(Y*1.7,V,.7),C.gain.setTargetAtTime(Y,V+2.6,1.5),J++}function ce(V){let Y=e.currentTime,he=e.createBiquadFilter(),_e=e.createGain(),{pan:me,fade:ae}=Pe(V);he.type="lowpass",he.frequency.value=420,_e.gain.setValueAtTime(0,Y),_e.gain.linearRampToValueAtTime(.028*ae,Y+.9),_e.gain.setValueAtTime(.028*ae,Y+3.2),_e.gain.exponentialRampToValueAtTime(1e-4,Y+5.5);let Se=[69.3,103.83].map(Me=>{let Ie=e.createOscillator();return Ie.type="sawtooth",Ie.frequency.value=Me,Ie.detune.value=Math.random()*8-4,Ie.connect(he),Ie});he.connect(_e),se([...Se,he,_e,...oe(_e,me,1.6)]),Se.forEach(Me=>{Me.start(Y),Me.stop(Y+5.6)})}function ie(V){let Y=e.currentTime,he=2+Math.floor(Math.random()*3),{pan:_e,fade:me}=Pe(V);for(let ae=0;ae<he&&u.size<is;ae++){let Se=Y+ae*(.12+Math.random()*.1),Me=e.createOscillator(),Ie=e.createGain(),j=2600+Math.random()*900;Me.frequency.setValueAtTime(j,Se),Me.frequency.exponentialRampToValueAtTime(j*1.5,Se+.07),Ie.gain.setValueAtTime(0,Se),Ie.gain.linearRampToValueAtTime(.006*me,Se+.015),Ie.gain.exponentialRampToValueAtTime(1e-4,Se+.13),Me.connect(Ie),se([Me,Ie,...oe(Ie,_e,.6)]),Me.start(Se),Me.stop(Se+.15)}}function de(V){let Y=e.currentTime,he=2+Math.floor(Math.random()*3),_e=760+Math.random()*280,{pan:me,fade:ae}=Pe(V);for(let Se=0;Se<he&&u.size<is;Se++){let Me=Y+Se*(.2+Math.random()*.14),Ie=e.createOscillator(),j=e.createBiquadFilter(),Te=e.createGain();Ie.type="sawtooth",Ie.frequency.setValueAtTime(_e,Me),Ie.frequency.exponentialRampToValueAtTime(_e*.5,Me+.26),j.type="bandpass",j.frequency.value=1500,j.Q.value=3.6,Te.gain.setValueAtTime(0,Me),Te.gain.linearRampToValueAtTime(.012*ae,Me+.04),Te.gain.exponentialRampToValueAtTime(1e-4,Me+.34),Ie.connect(j).connect(Te),se([Ie,j,Te,...oe(Te,me,.9)]),Ie.start(Me),Ie.stop(Me+.36)}}function ve(V){let Y=e.currentTime,he=2+Math.floor(Math.random()*2),_e=420+Math.random()*90,{pan:me,fade:ae}=Pe(V);for(let Se=0;Se<he&&u.size<is;Se++){let Me=Y+Se*(.17+Math.random()*.08),Ie=e.createOscillator(),j=e.createBiquadFilter(),Te=e.createGain();Ie.type="triangle",Ie.frequency.setValueAtTime(_e,Me),Ie.frequency.linearRampToValueAtTime(_e*.72,Me+.12),j.type="lowpass",j.frequency.value=1100,Te.gain.setValueAtTime(0,Me),Te.gain.linearRampToValueAtTime(.01*ae,Me+.03),Te.gain.exponentialRampToValueAtTime(1e-4,Me+.16),Ie.connect(j).connect(Te),se([Ie,j,Te,...oe(Te,me,.5)]),Ie.start(Me),Ie.stop(Me+.18)}}function We(V){let Y=e.currentTime,he=3+Math.floor(Math.random()*4),_e=3900+Math.random()*900,{pan:me,fade:ae}=Pe(V);for(let Se=0;Se<he&&u.size<is;Se++){let Me=Y+Se*.055,Ie=e.createOscillator(),j=e.createGain();Ie.type="square",Ie.frequency.value=_e,j.gain.setValueAtTime(0,Me),j.gain.linearRampToValueAtTime(.005*ae,Me+.008),j.gain.exponentialRampToValueAtTime(1e-4,Me+.035),Ie.connect(j),se([Ie,j,...oe(j,me,.35)]),Ie.start(Me),Ie.stop(Me+.05)}}function Ve(V){let Y=e.currentTime,he=196,{pan:_e,fade:me}=Pe(V),ae=e.createGain();ae.gain.setValueAtTime(0,Y),ae.gain.linearRampToValueAtTime(.02*me,Y+.012),ae.gain.exponentialRampToValueAtTime(1e-4,Y+4.2);let Se=[1,2.02,2.99,4.18,5.43].map((Me,Ie)=>{let j=e.createOscillator(),Te=e.createGain();return j.type="sine",j.frequency.value=he*Me,Te.gain.value=.5/(Ie+1),j.connect(Te).connect(ae),j});se([...Se,ae,...oe(ae,_e,1.8)]),Se.forEach(Me=>{Me.start(Y),Me.stop(Y+4.3)})}function lt(V){let Y=e.currentTime,he=2+Math.floor(Math.random()*3),{pan:_e,fade:me}=Pe(V);for(let ae=0;ae<he&&u.size<is;ae++){let Se=Y+ae*(.5+Math.random()*.35),Me=e.createBufferSource(),Ie=e.createBiquadFilter(),j=e.createGain(),Te=e.createOscillator(),we=e.createGain();Me.buffer=I,Me.loop=!0,Ie.type="bandpass",Ie.frequency.value=420,Ie.Q.value=1.1,j.gain.setValueAtTime(0,Se),j.gain.linearRampToValueAtTime(.02*me,Se+.006),j.gain.exponentialRampToValueAtTime(1e-4,Se+.42),Te.type="sine",Te.frequency.value=1180,we.gain.value=.2,Me.connect(Ie).connect(j),Te.connect(we).connect(j),se([Me,Ie,j,Te,we,...oe(j,_e,.9)]),Me.start(Se),Me.stop(Se+.44),Te.start(Se),Te.stop(Se+.44)}}function nt(V){let Y=e.currentTime,he=2+Math.floor(Math.random()*4),{pan:_e,fade:me}=Pe(V);for(let ae=0;ae<he&&u.size<is;ae++){let Se=Y+ae*(.06+Math.random()*.14),Me=e.createOscillator(),Ie=e.createGain();Me.type="sine",Me.frequency.setValueAtTime(900+Math.random()*700,Se),Me.frequency.exponentialRampToValueAtTime(320,Se+.06),Ie.gain.setValueAtTime(0,Se),Ie.gain.linearRampToValueAtTime(.008*me,Se+.005),Ie.gain.exponentialRampToValueAtTime(1e-4,Se+.12),Me.connect(Ie),se([Me,Ie,...oe(Ie,_e,.4)]),Me.start(Se),Me.stop(Se+.14)}}function ct(V){let Y=e.currentTime,he=3+Math.floor(Math.random()*4),_e=[1046.5,1174.7,1396.9,1567.98,1760,2093],{pan:me,fade:ae}=Pe(V);for(let Se=0;Se<he&&u.size<is;Se++){let Me=Y+Se*(.09+Math.random()*.13),Ie=_e[Math.floor(Math.random()*_e.length)],j=e.createOscillator(),Te=e.createGain();j.type="sine",j.frequency.value=Ie,Te.gain.setValueAtTime(0,Me),Te.gain.linearRampToValueAtTime(.006*ae,Me+.004),Te.gain.exponentialRampToValueAtTime(1e-4,Me+.9),j.connect(Te),se([j,Te,...oe(Te,me,1.5)]),j.start(Me),j.stop(Me+.95)}}function Q(V){let Y=e.currentTime,{pan:he,fade:_e}=Pe(V),me=e.createOscillator(),ae=e.createGain();me.type="sine",me.frequency.setValueAtTime(420,Y),me.frequency.linearRampToValueAtTime(620,Y+1.8),me.frequency.linearRampToValueAtTime(430,Y+3.6),ae.gain.setValueAtTime(0,Y),ae.gain.linearRampToValueAtTime(.01*_e,Y+.5),ae.gain.setValueAtTime(.01*_e,Y+2.8),ae.gain.exponentialRampToValueAtTime(1e-4,Y+4.2),me.connect(ae),se([me,ae,...oe(ae,he,1.2)]),me.start(Y),me.stop(Y+4.3)}let Pt={chime:xe,swoosh:$,gust:ue,horn:ce,birds:ie,gull:de,pigeon:ve,cricket:We,bell:Ve,hammer:lt,drip:nt,windchime:ct,siren:Q};function ft(){let V=RM.filter(_e=>!_e.when||_e.when(te));if(!V.length)return"chime";let Y=0;for(let _e of V)Y+=_e.weight;let he=Math.random()*Y;for(let _e of V)if(he-=_e.weight,he<=0)return _e.kind;return V[V.length-1].kind}function O(){if(u.size>=is)return;let V=ft(),Y=ge(V);Pt[V]?.(Y)}function R(){clearTimeout(W),W=0,Ne()&&(W=setTimeout(()=>{W=0,Ne()&&(O(),R())},(te.busy?1900:3600)+Math.random()*9e3))}function re(){clearTimeout(z),z=0,Ne()&&(z=setTimeout(()=>{z=0,Ne()&&(E=(E+1)%dd.length,_.forEach((V,Y)=>V.forEach(he=>he.frequency.setTargetAtTime(dd[E][Y],e.currentTime,1.8))),re())},16e3))}function le(){clearTimeout(W),clearTimeout(z),W=z=0;for(let V of[...u])V.release()}function be(){if(!e)return;let V=e.currentTime;S.frequency.setTargetAtTime(te.busy?1500:650+te.day*450+te.evening*150,V,2.5),A.gain.setTargetAtTime((te.busy?.055:.042)*(te.weather==="rain"?.75:1),V,2)}function De(){if(!e||!M||!N||!D)return;let V=Math.max(-76,Math.min(76,q.origin.x)),Y=Math.hypot(q.origin.x-V,q.origin.z-77),he=Math.max(0,1-(Y-6)/42),_e=Math.max(0,1-(Y-25)/110)*.18,me=(T?0:Math.min(1,he+_e)*.05*(g==="rain"?1.2:1)).toFixed(3),ae=((te.busy?.03:.012)*(.35+te.day*.65)*(T?.25:1)).toFixed(3),Se=Math.hypot(q.origin.x-V,q.origin.z-75.5),Me=Math.max(0,1-(Se-8)/46),Ie=(T?0:Math.min(1,Me)*.012*(1-te.day)*(g==="rain"?.6:1)).toFixed(3);if(me===U&&ae===H&&Ie===L)return;U=me,H=ae,L=Ie;let j=e.currentTime;M.gain.setTargetAtTime(Number(me),j,.5),N.gain.setTargetAtTime(Number(ae),j,.8),D.gain.setTargetAtTime(Number(Ie),j,.8)}function Ue(V=!1){if(clearTimeout(o),!i){if(V&&a)try{Fe()}catch{a=!1,s(!1);return}e&&(l?.setActive(a&&r&&p>0&&Le.voice>0),x?.setActive(a&&r&&p>0),v?.setActive(a&&r&&p>0&&Le.effects>0),a&&r?(e.resume().catch(()=>{}),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(p,e.currentTime,.25),W||R(),z||re()):(le(),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(0,e.currentTime,.035),o=setTimeout(()=>{!i&&(!a||!r)&&e.suspend().catch(()=>{})},180)))}}return{enabled:()=>a,toggle(){a=!a;try{localStorage.setItem("aurago.desktop.sysworld.sound",String(a))}catch{}Ue(!0),s(a)},unlock(){Ue(!0)},setActive(V){r!==V&&(r=V,Ue())},setVolume(V){Number.isFinite(V)&&(p=Math.max(0,Math.min(.35,V)),Ue())},setListener(V,Y,he,_e,me,ae,Se){Object.assign(q,{x:V,y:Y,z:he,fx:_e,fz:me}),q.origin.x=ae??V,q.origin.z=Se??he,l?.setListener(V,Y,he,_e,me),x?.listener(V,Y,he,_e,me),v?.listener(V,Y,he,_e,me,q.origin.x,q.origin.z),De()},setChannel(V,Y){if(!(V in Le)||!Number.isFinite(Y))return;Le[V]=Math.max(0,Math.min(1,Y));let he={ambience:m,effects:f,voice:y}[V];he&&he.gain.setTargetAtTime(Le[V],e.currentTime,.05),Ue()},effect(V,Y,he,_e,me){if(String(V).startsWith("robot_")){v?.action(V,Y,he,_e,me);return}x?.play(V,Y,he,_e)},gait(V,Y,he,_e,me,ae,Se){v?.gait(V,Y,he,_e,me,ae,Se)},setEnvironment(V,Y){T=!!V,["clear","rain","fog"].includes(Y)&&(g=Y),x?.environment(T,g),v?.environment(T),De()},setMood(V={}){Object.assign(te,{busy:!!V.busy,day:Number.isFinite(V.day)?V.day:te.day,evening:Number.isFinite(V.evening)?V.evening:te.evening,weather:["clear","rain","fog"].includes(V.weather)?V.weather:te.weather}),be(),De()},thunder(V=1){if(!Ne()||T||u.size>=is)return;let Y=Math.max(0,Math.min(5,V)),he=e.currentTime+Y,_e=Math.min(1,Y/4),me=e.createBufferSource(),ae=e.createBiquadFilter(),Se=e.createGain();me.buffer=I,me.loop=!0,ae.type="lowpass",ae.Q.value=.7,ae.frequency.setValueAtTime(900-_e*500,he),ae.frequency.exponentialRampToValueAtTime(70,he+3.5),Se.gain.setValueAtTime(0,he),Se.gain.linearRampToValueAtTime(.09*(1-_e*.5),he+.06+_e*.25),Se.gain.exponentialRampToValueAtTime(.025,he+1.3),Se.gain.exponentialRampToValueAtTime(1e-4,he+5.5),me.connect(ae).connect(Se),se([me,ae,Se,...oe(Se,Math.random()*.8-.4,.8)]),me.start(he),me.stop(he+5.6)},stats(){let V=0;if(n&&e.state==="running"){n.getFloatTimeDomainData(d);for(let Y of d)V+=Y*Y}return{voice:l?.stats(),effects:x?.stats(),foley:v?.stats(),channels:{...Le},enabled:a,active:r,state:e?.state||"uninitialized",volume:p,rms:Math.sqrt(V/d.length),soundscape:{chord:E,events:J,transient:u.size,scheduled:!!W,mood:{...te},beds:{water:Number(U),murmur:Number(H),lamp:Number(L)},listener:{x:q.x,z:q.z}}}},dispose(){i||(i=!0,le(),v?.dispose(),x?.dispose(),l?.dispose(),clearTimeout(o),h.forEach(V=>{try{V.stop()}catch{}}),c.forEach(V=>V.disconnect()),e&&e.close().catch(()=>{}))}}}var yn=[{id:"agent",asset:"agent-spire",x:0,z:-12,height:87,radius:13},{id:"infra",asset:"compute-foundry",x:-43,z:-53,height:24,radius:18},{id:"integrations",asset:"integration-gate",x:43,z:-10,height:34,radius:14},{id:"missions",asset:"mission-terminal",x:43,z:36,height:23,radius:15},{id:"memory",asset:"memory-archive",x:-43,z:-10,height:37,radius:14},{id:"graph",asset:"knowledge-atrium",x:-43,z:36,height:28,radius:17},{id:"operations",asset:"operations-beacon",x:0,z:36,height:22,radius:6}],jc={low:{lod:2,dpr:1,shadow:0,bloom:!1},medium:{lod:1,dpr:1.25,shadow:1024,bloom:!1},high:{lod:0,dpr:1.5,shadow:2048,bloom:!0},ultra:{lod:0,dpr:2,shadow:4096,bloom:!0}},fd=new k(122,106,183),pd=class extends $n{constructor(e,t,n){super(),this.scene=e,this.camera=t,this.needsSwap=!1,this.target=new an(1,1,{type:gn,samples:n}),this.quad=new zi(new wt({uniforms:Mi.clone(ts.uniforms),vertexShader:ts.vertexShader,depthTest:!1,depthWrite:!1,fragmentShader:"uniform sampler2D tDiffuse;varying vec2 vUv;void main(){vec4 c=texture2D(tDiffuse,vUv);if(any(isnan(c))||any(isinf(c)))c=vec4(0.0,0.0,0.0,1.0);gl_FragColor=c;}"}))}render(e,t,n){let i=e.autoClear;e.autoClear=!1,e.setRenderTarget(this.target),e.clear(),e.render(this.scene,this.camera),this.quad.material.uniforms.tDiffuse.value=this.target.texture,e.setRenderTarget(this.renderToScreen?null:n),this.quad.render(e),e.autoClear=i}setSize(e,t){this.target.setSize(e,t)}dispose(){this.target.dispose(),this.quad.material.dispose(),this.quad.dispose()}},sa=new k(0,22,-12);function IM(s){return s.name+(s.vertexColors?"|vertex-colours":"")}var Xm=Wp().map(s=>({asset:s.id,x:s.x,z:s.z,y:0,angle:s.angle,scale:[1,1,1]})),TT=["agent-spire","memory-archive","integration-gate","mission-terminal","compute-foundry","knowledge-atrium","operations-beacon","data-tower-a","data-tower-b","skybridge","street-lamp","planter"];function DM(s,e){return e?.has(s)?"v3":"v1"}var nr=[];{let s=(a,o,l,c=0,h=0,d=[1,1,1])=>nr.push({asset:a,x:o,z:l,y:c,angle:h,scale:d}),{xs:e,zs:t}=Ht,n=pt.pavement-.05;for(let a of t)for(let o of[-55,-31,31,55])s("street-lamp",o,a-4.5,n),s("planter",o+4*Math.sign(Math.sign(o)*43-o),a-4.5,n);for(let{x:a,z:o,scale:l}of zc)s("data-tower-a",a,o,0,0,[1,l,1]);s("skybridge",39.75,-55,15),s("server-rack",-55,-36.7,pt.pavement),s("server-rack",-49,-36.7,pt.pavement);let i=7919,r=()=>(i=Math.imul(i,1664525)+1013904223>>>0)/4294967296;for(let a=0;a<4;a++)for(let o=0;o<12;o++){let l=r(),c=r(),h=r(),d=r(),u=r(),p=r();if(d<.12)continue;let x=(o-5.5)*22+(l-.5)*12,v=-143-a*29+(c-.5)*14,m=1-Math.min(1,Math.abs(x)/150),f=(.5+h*h+.45*m)*(a?1:.85),y=.7+.34*u;s(l>.5?"data-tower-a":"data-tower-b",x,v,-3,p<.3?Math.PI/2:0,[y,f,y])}for(let a=0;a<16;a++){let o=(a-7.5)*34+(r()-.5)*16,l=-268-r()*46;s(r()<.5?"data-tower-a":"data-tower-b",o,l,-3,0,[1.1,.55+r()*1.1,1.1])}}var LM=[...Lp(nr),...Ln.flatMap(s=>[-3,3].flatMap(e=>[-3,3].map(t=>({x:s.x+e,z:s.z+t,r:4.25,asset:"interior"}))))];async function AT(s,e){let t=!1,n=!0,i="orbit",r=e.quality||"auto",a=r==="auto"?"high":r,o=0,l=null,c=0,h=0,d=0,u=0,p=0,x=0,v=0,m=null,f=e.reducedMotion,y=1,T=1,g=[],_=!1,S=null,A=null,b=null,C=new AbortController,w=new Set,I=new Map,M=new Map,N=new Set,D=new Set,U=new Set,H=[],L=new wc({antialias:!0,alpha:!1});L.toneMapping=js,L.toneMappingExposure=.98,L.shadowMap.type=qs,L.shadowMap.autoUpdate=!1,L.info.autoReset=!1;let W=L.domElement;W.className="sysworld-gl",W.tabIndex=0,W.setAttribute("aria-label",e.label),s.append(W);let z=new ks;z.background=new Ce(528926),z.fog=new Ma(528926,.004);let E=new mn(43,1,.3,1800);E.position.copy(fd);let J=Oc(),q=J.register("visitor",{circles:[{x:0,z:0,r:.38}],minY:-.4,maxY:.4,reach:.38},E.position,100),te=new Cc(E,W);te.target.copy(sa),te.enableDamping=!0,te.dampingFactor=.085,te.minDistance=12,te.maxDistance=410,te.maxPolarAngle=Math.PI*.485,te.update();let Le=0,Fe=()=>{let G=Ne?.progress();e.onProgress?.(p+(G?.bytes||0),Le+(G?.expected||0))},Ne=km(L,{url:G=>e.resourceURL("/3d/system-world/textures/v1/"+G),onProgress:()=>Fe(),shelters:Ln.map(G=>[G.x-G.width/2,G.z-G.depth/2,G.x+G.width/2,G.z+G.depth/2])}),se=Hm(),oe=new Kr(L),Pe=new Pc,ge=oe.fromScene(Pe,.05);z.environment=ge.texture,z.environmentIntensity=.48,Pe.dispose(),oe.dispose();let xe=new za(12639487,1056813,.8);z.add(xe);let $=new $i(13164287,3.2);$.position.set(-70,145,85),$.castShadow=!0,Object.assign($.shadow.camera,{left:-108,right:108,top:108,bottom:-108,near:1,far:360}),$.shadow.normalBias=.09,$.shadow.bias=-15e-5,$.shadow.radius=2.4,z.add($);let ue=new $i(16758130,2.4);ue.position.set(90,80,-120),z.add(ue);let ce=new $i(14195306,.42);ce.position.set(-10,-40,30),z.add(ce);let ie=new bi(6479871,0,75,2);ie.position.set(0,34,-12),z.add(ie);let de=new an(1,1,{type:gn}),ve=new Dc(L,de);ve.addPass(new pd(z,E,Math.min(4,L.capabilities.maxSamples)));let We=new ea(new ze(1,1),.28,.68,1.06);ve.addPass(We),ve.addPass(new Lc);let Ve=new yt,lt=new yt,nt=new yt;z.add(Ve),Ve.add(lt,nt);let ct=Nm(z);function Q(G,Re){return N.add(G),D.add(Re),new ot(G,Re)}let Pt=new An({color:1911345,metalness:.05,roughness:.82}),ft=new An({color:1382685,metalness:.05,roughness:.9});D.add(Pt),D.add(ft);let O=new gi(Ht.maxX-Ht.minX,$s.top-$s.bottom,Ht.maxZ-Ht.minZ);N.add(O);let R=new ot(O,[Pt,Pt,ft,Pt,Pt,Pt]);R.position.set((Ht.minX+Ht.maxX)/2,($s.top+$s.bottom)/2,(Ht.minZ+Ht.maxZ)/2),R.receiveShadow=!0,Ve.add(R),Ne.apply(Pt,"world","ground"),Ne.apply(ft,"world","city.road");let re=qp(),le=new gi(1,1,1);N.add(le);let be=new En(le,Pt,re.length);be.name="plots",be.receiveShadow=!0,re.forEach((G,Re)=>be.setMatrixAt(Re,new mt().compose(new k((G.x0+G.x1)/2,(G.top+G.bottom)/2,(G.z0+G.z1)/2),new $t,new k(G.x1-G.x0,G.top-G.bottom,G.z1-G.z0)))),Ve.add(be);let De=om(z,{sunDirection:$.position,lamps:nr.filter(G=>G.asset==="street-lamp")});ve.addPass(De.post);let Ue=cm(z,{anchors:yn.map(G=>({id:G.id,x:G.x,z:G.z,height:G.height}))}),V=yn.find(G=>G.id==="memory"),Y=Qp(z,V,{roof:21,label:e.memoryLabel}),he=um(z,{traffic:J,surfaces:Ne}),_e=Lm(z,{sun:$,rim:ue,hemisphere:xe,atmosphere:De,surfaces:Ne,onThunder:G=>e.onThunder?.(G)}),me="",ae=null,Se={value:0},Me=new xs(1,1.045,96);N.add(Me);let Ie=(G,Re)=>{let He=new wt({transparent:!0,depthWrite:!1,side:Yt,uniforms:{color:{value:new Ce(9365742)},time:Se,opacity:{value:G},dashes:{value:Re}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float time,opacity,dashes;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;float dash=dashes>0.5?step(0.3,fract(a*dashes-time*0.35)):1.0;gl_FragColor=vec4(color,opacity*dash*(0.8+0.2*sin(time*2.2)));}"});return D.add(He),He},j=new ot(Me,Ie(.9,18)),Te=new ot(Me,Ie(.38,0));for(let G of[j,Te])G.rotation.x=-Math.PI/2,G.visible=!1,Ve.add(G);let we=new wt({transparent:!0,depthWrite:!1,side:Yt,uniforms:{color:{value:new Ce(9365742)},alpha:{value:0}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float alpha;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;gl_FragColor=vec4(color,alpha*(0.55+0.45*sin(a*3.0)));}"});D.add(we);let Oe=new ot(new xs(.9,1,96),we);N.add(Oe.geometry),Oe.rotation.x=-Math.PI/2,Oe.visible=!1,Ve.add(Oe);let ke=2,Ae={error:16742504,running:7730385,stale:8096667,idle:8175587},Ye={},Qe=()=>j.material.uniforms.color.value.setHex(Ae[Ye[m]]??9365742),P=new qt({color:6742230,toneMapped:!1}),B=new En(new Ii(.65,10,6),P,yn.length);N.add(B.geometry),D.add(P),Ve.add(B);let K=new Xt,ee=new k,X=new Wa,Z=new ze;yn.forEach((G,Re)=>{K.position.set(G.x,G.height+2,G.z),K.updateMatrix(),B.setMatrixAt(Re,K.matrix),B.setColorAt(Re,new Ce(8425630))}),e.signal?.addEventListener("abort",$e,{once:!0});let Ee;try{if(e.signal?.aborted)throw Error("Disposed");let G=await fetch(e.assetURL("manifest.json"),{signal:C.signal});if(!G.ok)throw Error("City manifest unavailable");Ee=await G.json()}catch(G){throw $e(),G}let Be=new Map(Ee.assets.map(G=>[G.id,G])),st=new Map;try{let G=await fetch(e.resourceURL("/3d/system-world/v3/manifest.json"),{signal:C.signal}),Re=G.ok?await G.json():null;if(Re?.schema_version===3&&Array.isArray(Re.assets))for(let He of Re.assets)st.set(He.id,He)}catch(G){if(C.signal.aborted)throw $e(),G}Om(J,Um(Be,st),yn,nr),ae=Im(z,{shadows:ct,camera:E,districts:yn,tier:a,traffic:J,reduced:()=>f,active:()=>n&&!_&&i!=="map"&&!f&&!e.replaying?.(),assetURL:G=>e.resourceURL("/3d/system-world/v2/"+G),surfaces:Ne,clear:(G,Re)=>J.clear(q,{...G,heading:0},{...Re,heading:0},!1),onFlight:G=>e.onFlight?.(G),onInteraction:e.onInteraction,onScope:G=>e.onScope?.(G),onDiscover:e.onDiscover,onSound:e.onSound,onGait:e.onGait,onTerminal:e.onTerminal,onSociety:e.onSociety,onEnvironment:G=>{_e.setIndoor(G),e.onEnvironment?.(G)},onError:e.onError,onReady:()=>{L.shadowMap.needsUpdate=!0}});let Je={v1:{catalog:Be,url:G=>e.assetURL(G),loader:()=>new wi},v3:{catalog:st,url:G=>e.resourceURL("/3d/system-world/v3/"+G),loader:()=>new wi().setMeshoptDecoder(Zr)}};async function kt(G,Re,He="v1"){let et=He+":"+G+":"+Re;return I.has(et)||I.set(et,(async()=>{let bt=Je[He],Ge=bt.catalog.get(G)?.lods.find(ut=>ut.level===Re);if(!Ge||!/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(Ge.file))throw Error("Invalid city asset");let zt=new AbortController;w.add(zt);let tt=setTimeout(()=>zt.abort(),15e3);try{let ut=await fetch(bt.url(Ge.file),{signal:zt.signal});if(!ut.ok)throw Error("City asset unavailable");let dt=await ut.arrayBuffer();if(t)throw Error("Disposed");He==="v3"&&await Zr.ready;let At=await bt.loader().parseAsync(dt,"");if(t)throw At.scene.traverse(vt=>{vt.isMesh&&(vt.geometry.dispose(),vt.material.dispose())}),Error("Disposed");return p+=dt.byteLength,Fe(),At.scene.traverse(vt=>{if(!vt.isMesh)return;N.add(vt.geometry),vt.castShadow=!0,vt.receiveShadow=!0,se.adopt(vt);let Ft=(Array.isArray(vt.material)?vt.material:[vt.material]).map(Zt=>{if(se.owns(Zt))return Zt;let qe=IM(Zt);if(M.has(qe)){let on=Zt;Zt=M.get(qe),on.dispose()}else Zt.isMeshStandardMaterial&&Zt.metalness>.5&&(Zt.roughness=Math.max(Zt.roughness,.45)),M.set(qe,Zt),D.add(Zt);return Zt});vt.material=Array.isArray(vt.material)?Ft:Ft[0],Ne.prepare(vt)}),At.scene}finally{clearTimeout(tt),w.delete(zt)}})().catch(bt=>{throw I.delete(et),bt})),I.get(et)}function Bt(G){G.traverse(Re=>{Re.isInstancedMesh&&Re.dispose()}),G.clear()}async function St(){let G=++o,Re=jc[a],He=Re.lod,et=[...new Set([...nr.map(ut=>ut.asset),...yn.map(ut=>ut.asset),"service-drone"])],bt=[...new Set(Xm.map(ut=>ut.asset))],Ge=ut=>DM(ut,st),zt=(ut,dt,At=Ge(ut))=>(At==="v3"?st:Be).get(ut)?.lods.find(vt=>vt.level===dt)?.bytes||0,tt=(ut,dt,At=Ge(ut))=>I.has(At+":"+ut+":"+dt)?0:zt(ut,dt,At);Le=et.reduce((ut,dt)=>ut+tt(dt,He),p)+(He===2?0:["data-tower-a","data-tower-b"].reduce((ut,dt)=>ut+tt(dt,2),0))+bt.reduce((ut,dt)=>ut+tt(dt,He,"v3"),0);try{let ut=new Map(await Promise.all(et.map(async qe=>[qe,await kt(qe,He,Ge(qe))]))),dt=new Map(await Promise.all(["data-tower-a","data-tower-b"].map(async qe=>[qe,await kt(qe,2,Ge(qe))]))),At=new Map(await Promise.all(bt.map(async qe=>[qe,await kt(qe,He,"v3")])));if(t||G!==o)return;Bt(lt),Bt(nt),g=[];let vt=new Map,at=(qe,on)=>{on.updateMatrixWorld(!0);let qn=new mt().compose(new k(qe.x,qe.y,qe.z),new $t().setFromAxisAngle(new k(0,1,0),qe.angle),new k(...qe.scale));on.traverse(li=>{li.isMesh&&(vt.has(li.uuid)||vt.set(li.uuid,{node:li,matrices:[]}),vt.get(li.uuid).matrices.push(new mt().multiplyMatrices(qn,li.matrixWorld)))})};for(let qe of nr)at(qe,(qe.z<-100?dt:ut).get(qe.asset));for(let qe of Xm)at(qe,At.get(qe.asset));for(let{node:qe,matrices:on}of vt.values()){let qn=new En(qe.geometry,qe.material,on.length);on.forEach((li,jm)=>qn.setMatrixAt(jm,li)),qn.castShadow=!0,qn.receiveShadow=!0,lt.add(qn)}for(let qe of yn){let on=ut.get(qe.asset).clone(!0);on.position.set(qe.x,0,qe.z),on.userData.district=qe.id,nt.add(on),g.push(on)}A?.attachLandmarks(nt),he.setTemplate(ut.get("service-drone"));let Ft=new hn,Zt=new Map;for(let qe of["data-tower-a","data-tower-b"])Zt.set(qe,Ft.setFromObject(dt.get(qe)).max.y);De.setAviation(nr.filter(qe=>Zt.has(qe.asset)&&(qe.z<-100||qe.scale[1]>1.1)).map(qe=>({x:qe.x,y:qe.y+Zt.get(qe.asset)*qe.scale[1]+.6,z:qe.z}))),L.shadowMap.needsUpdate=!0,e.onReady?.()}catch(ut){!t&&G===o&&e.onError?.(ut)}}function Mn(){if(t)return;y=Math.max(1,s.clientWidth),T=Math.max(1,s.clientHeight);let G=Math.min(devicePixelRatio||1,jc[a].dpr,Math.sqrt(3840*2160/(y*T)));L.setPixelRatio(G),L.setSize(y,T,!1),ve.setPixelRatio(G),ve.setSize(y,T),E.aspect=y/T,E.updateProjectionMatrix(),Y.setPointScale(T*G),De.setPointScale(T*G),Ue.setPointScale(T*G)}function Vt(G){return r=G in jc||G==="auto"?G:"auto",a=r==="auto"?"high":r,nn(),St()}function nn(){let G=jc[a];L.shadowMap.enabled=G.shadow>0,G.shadow&&$.shadow.mapSize.x!==G.shadow&&($.shadow.mapSize.set(G.shadow,G.shadow),$.shadow.map?.dispose(),$.shadow.map=null),L.shadowMap.needsUpdate=!0,We.enabled=G.bloom,Ne.setTier(a),se.setTier(a),De.setTier(a),ae.setTier(a),_e.setTier(a),he.setTier(a),Ue.setTier(a),Mn(),e.onQuality?.(r,a)}function en(G,Re){l=null;let He=J.findFree(q,G);if(!He)return;if(G=new k(He.x,He.y,He.z),f){E.position.copy(G),Object.assign(q,He),te.target.copy(Re),te.update(),l=null;return}let et=(tt,ut)=>J.clear(q,{...tt,heading:0},{...ut,heading:0},!1),bt=E.position.clone(),Ge=Math.max(bt.y,G.y,110),zt=[bt,G];if(!et(bt,G)){let tt=[bt],ut=new k(G.x,Ge,G.z),dt=[-1],At=[0],vt=-1;if(!et(ut,G))return;for(let at of[3,8,16])for(let Ft=0;Ft<8;Ft++)tt.push(new k(bt.x+Math.cos(Ft*Math.PI/4)*at,bt.y,bt.z+Math.sin(Ft*Math.PI/4)*at));for(let at of Ln)if(Math.hypot(bt.x-at.x,bt.z-at.z)<24)for(let Ft of[at.z,at.doorZ-at.front*2,at.doorZ+at.front*3])tt.push(new k(at.x,bt.y,Ft));for(;At.length;){let at=At.shift(),Ft=tt[at],Zt=new k(Ft.x,Ge,Ft.z);if(et(Ft,Zt)&&et(Zt,ut)){vt=at;break}for(let qe=1;qe<tt.length;qe++)dt[qe]===void 0&&Ft.distanceTo(tt[qe])<=20&&et(Ft,tt[qe])&&(dt[qe]=at,At.push(qe))}if(vt<0)return;zt=[];for(let at=vt;at>=0;at=dt[at])zt.unshift(tt[at]);zt.push(new k(tt[vt].x,Ge,tt[vt].z),ut,G)}l={start:E.position.clone(),targetStart:te.target.clone(),end:G.clone(),target:Re.clone(),time:0,waypoints:zt}}function fn(){J.relocate(q,E.position),E.position.set(q.x,q.y,q.z)}function ss(G){let Re=yn.find(He=>He.id===G);Re&&(m=G,j.position.set(Re.x,.55,Re.z),j.scale.setScalar(Re.radius*1.3),j.visible=!0,Qe(),Oe.position.set(Re.x,.62,Re.z),Oe.scale.setScalar(Re.radius*1.3*.35),we.uniforms.color.value.setHex(Ae[Ye[G]]??9365742),we.uniforms.alpha.value=0,ke=0,Oe.visible=!f,Te.visible=!!ne&&ne!==m,i==="street"?(ae.endRide(),e.onCut?.(),E.position.set(Re.x,2.4+Fn(Re.x,Re.z+Re.radius+7),Re.z+Re.radius+7),E.lookAt(Re.x,Re.height*.4,Re.z),fn()):en(new k(Re.x+Re.radius*2.7,Re.height*.7+18,Re.z+Re.radius*4),new k(Re.x,Re.height*.4,Re.z)))}function wn(G){ae.endRide(),ae.society.suspend(),U.clear(),l=null,i=G,te.enabled=i==="orbit"||i==="tour",q.minY=i==="street"?-2.3:-.4,q.ignore=q.follow=null,q.circles[0].r=q.reach=i==="street"?.24:.38,document.pointerLockElement===W&&document.exitPointerLock(),i==="street"?(e.onCut?.(),pe(),E.position.set(18,2.4+Fn(18,57),57),E.lookAt(0,26,-12),fn()):i!=="map"&&(fn(),en(fd.clone().multiplyScalar(E.aspect<1?1.3:1),sa)),x=0,v=0,e.onMode?.(i)}let rs=()=>{l=null,i==="tour"&&(i="orbit",e.onMode?.(i))};te.addEventListener("start",rs);function Nn(G,Re,He,et){G.addEventListener(Re,He,et),H.push(()=>G.removeEventListener(Re,He,et))}let Wn=null,ra=!1,as=new kn(0,0,0,"YXZ");function F(G,Re){let He=.0025*E.fov/43;as.setFromQuaternion(E.quaternion),as.y-=G*He,as.x=It.clamp(as.x-Re*He,-1.35,1.35),E.quaternion.setFromEuler(as)}Nn(W,"pointerdown",G=>{W.focus({preventScroll:!0}),rs(),Wn=[G.clientX,G.clientY],ra=!1,i==="street"&&W.setPointerCapture(G.pointerId)});let ne=null,ye=-1e9;function fe(G){if(G===ne)return;ne=G,W.style.cursor=G?"pointer":"";let Re=yn.find(He=>He.id===G);Te.visible=!!Re&&G!==m,Re&&(Te.position.set(Re.x,.56,Re.z),Te.scale.setScalar(Re.radius*1.3)),e.onHover?.(G)}function pe(){fe(null)}function je(G){if(i==="street"||i==="map"||Wn||G.timeStamp-ye<80)return;ye=G.timeStamp;let Re=W.getBoundingClientRect();Z.set((G.clientX-Re.left)/Re.width*2-1,1-(G.clientY-Re.top)/Re.height*2),X.setFromCamera(Z,E);let He=X.intersectObjects(g,!0)[0]?.object;for(;He&&!He.userData.district;)He=He.parent;fe(He?.userData.district||null)}Nn(W,"pointerleave",pe),Nn(W,"pointermove",G=>{i==="street"&&(document.pointerLockElement===W||Wn)&&F(G.movementX,G.movementY),je(G),Wn&&Math.hypot(G.clientX-Wn[0],G.clientY-Wn[1])>5&&(ra=!0)}),Nn(W,"pointerup",G=>{if(Wn&&!ra&&i!=="street"){let Re=W.getBoundingClientRect();Z.set((G.clientX-Re.left)/Re.width*2-1,1-(G.clientY-Re.top)/Re.height*2),X.setFromCamera(Z,E);let He=X.intersectObjects(g,!0)[0];if(He){let et=He.object;for(;et&&!et.userData.district;)et=et.parent;et&&e.onSelect?.(et.userData.district)}}Wn=null}),Nn(W,"pointercancel",()=>{Wn=null,U.clear()}),Nn(W,"keydown",G=>{if(G.key==="Escape"){ae.scoping()?ae.endRide():wn("orbit"),G.preventDefault();return}if(i==="street"&&["+","=","-"].includes(G.key)&&ae.zoom(G.key==="-"?240:-240)){G.preventDefault();return}if(i==="street"&&G.code==="KeyE"&&!G.repeat){ae.interact(),G.preventDefault();return}i==="street"&&["KeyW","KeyA","KeyS","KeyD","ArrowUp","ArrowDown","ArrowLeft","ArrowRight","ShiftLeft"].includes(G.code)&&(U.add(G.code),G.preventDefault(),G.stopPropagation()),i==="street"&&["Space","KeyC"].includes(G.code)&&!G.ctrlKey&&!G.metaKey&&!G.altKey&&(U.add(G.code),G.preventDefault(),G.stopPropagation())}),Nn(W,"keyup",G=>U.delete(G.code)),Nn(W,"wheel",G=>{i==="street"&&ae.zoom(G.deltaY)&&G.preventDefault()},{passive:!1}),Nn(W,"blur",()=>U.clear()),Nn(window,"blur",()=>{U.clear(),Wn=null}),Nn(W,"webglcontextlost",G=>{G.preventDefault(),_=!0,U.clear(),e.onContextLost?.()}),S=new ResizeObserver(Mn),S.observe(s);let Ze={y:NaN,v:0};function Xe(G){let Re=(U.has("ShiftLeft")?25:11)*G,He=Number(U.has("KeyW")||U.has("ArrowUp"))-Number(U.has("KeyS")||U.has("ArrowDown")),et=Number(U.has("KeyD")||U.has("ArrowRight"))-Number(U.has("KeyA")||U.has("ArrowLeft"));if(ae.hovering()){ae.fly(G,{forward:He,right:et,up:Number(U.has("Space"))-Number(U.has("KeyC")),boost:U.has("ShiftLeft")}),Ze.y=NaN;return}if(He||et){if(ae.walkRide(He,G,et))return;let bt=Re/Math.hypot(He,et);E.getWorldDirection(ee),ee.y=0,ee.normalize();let Ge=(ee.x*He-ee.z*et)*bt,zt=(ee.z*He+ee.x*et)*bt,tt=(ut,dt)=>ae.move(ut,dt,E.position)&&J.clear(q,q,{x:ut,y:2.4+ae.floor(ut,dt),z:dt,heading:0});tt(E.position.x+Ge,E.position.z)&&(E.position.x+=Ge),tt(E.position.x,E.position.z+zt)&&(E.position.z+=zt)}if(ae.isRiding()){Ze.y=NaN;return}Math.abs(E.position.y-Ze.y)<1e-6||(Ze.y=E.position.y,Ze.v=0),E.position.y=As(Ze,2.4+ae.floor(E.position.x,E.position.z),f?0:G)}let it=0,rt=Np(G=>{J.begin(),it+=f?0:G,i==="street"&&Xe(G),ae.update(G,!f,i);let Re=q.follow;q.follow=ae.rideBody(),q.ignore=q.follow||Re,A?.update(G,!f),he.update(G,it,!f),J.propose(q,{...E.position,heading:0},(He,et)=>E.position.set(et.x,et.y,et.z)),J.solve(G),ae.settle(f?0:G),ae.syncRide?.(),q.follow&&Object.assign(q,{x:E.position.x,y:E.position.y,z:E.position.z}),q.ignore=q.follow});function Et(G,Re){if(t||!n||_||i==="map")return;let He=null,et=0,bt=null;if(i!=="street"){if(i==="tour"&&(x-=G,x<=0)){let dt=yn[v++%yn.length].id;ss(dt),e.onTourFocus?.(dt),x=7}if(l){He=l,et=l.time,l.time+=G;let dt=Math.min(1,l.time/(l.duration||(l.waypoints.length===2?1.1:3.2))),At=dt*dt*(3-2*dt),vt=Math.min(l.waypoints.length-1-1e-6,At*(l.waypoints.length-1)),at=Math.floor(vt);E.position.lerpVectors(l.waypoints[at],l.waypoints[at+1],vt-at),te.target.lerpVectors(l.targetStart,l.target,At),E.lookAt(te.target),bt=E.position.clone(),dt===1&&(l=null)}else te.update()}rt(Math.min(.1,Math.max(0,G)))||(J.begin(),J.propose(q,{...E.position,heading:0},(dt,At)=>E.position.set(At.x,At.y,At.z)),J.solve(0)),He&&(E.position.distanceToSquared(bt)>1e-6?(He.wait=(He.wait||0)+G,He.time=et,l=He.wait<6?He:null):He.wait=0),E.getWorldDirection(ee);let Ge=i!=="street"&&E.position.y>15;e.onListener?.(E.position.x,E.position.y,E.position.z,ee.x,ee.z,Ge?te.target.x:E.position.x,Ge?te.target.z:E.position.z);let zt=!!e.busy?.();_e.update(G,Re,!f)&&(L.shadowMap.needsUpdate=!0),ie.intensity=f?0:zt?180+Math.sin(Re*2)*35:12,De.setBusy(zt),De.setCinematic(i==="tour"),De.update(G,Re,E,!f),Ne.update(G,!f),b?.update(G,!f&&n&&i!=="map"&&!e.replaying?.());let tt=_e.mood();if(Ue.update(G,!f,1-(tt.day||0)),se.setLighting(tt.day,tt.evening,tt.weather==="clear"?0:1,z.fog.color),se.update(G,!f),!f){if(ke<1.4){ke+=G;let dt=Math.min(1,ke/1.4),At=yn.find(vt=>vt.id===m);At&&(Oe.scale.setScalar(At.radius*1.3*(.35+dt*1.05)),we.uniforms.alpha.value=(1-dt)*.55),dt>=1&&(Oe.visible=!1)}for(let dt=0;dt<yn.length;dt++){let At=yn[dt];K.position.set(At.x,At.height+2,At.z),K.scale.setScalar(1+Math.sin(Re*1.7+dt*1.13)*.13),K.updateMatrix(),B.setMatrixAt(dt,K.matrix)}B.instanceMatrix.needsUpdate=!0}f||(Se.value+=Math.min(.1,Math.max(0,G)));let ut=[zt,tt.day.toFixed(1),tt.evening.toFixed(1),tt.weather].join();if(We.strength=.2+.15*(1-tt.day),We.threshold=1.06+.26*It.smoothstep(tt.day,0,1),L.toneMappingExposure=.95+.05*(1-tt.day),ut!==me&&(me=ut,e.onMood?.({busy:zt,day:tt.day,evening:tt.evening,weather:tt.weather})),Y.update(G,E,!f),L.info.reset(),ve.render(),c++,r==="auto"&&G>0&&G<.2&&Re-u>12&&(h+=G,d++,d>=180)){let dt=h/d,At=["low","medium","high"],vt=At.indexOf(a),at=dt>.028&&vt>0?At[vt-1]:dt<.017&&vt<2?At[vt+1]:a;d=0,h=0,at!==a&&(a=at,u=Re,nn(),St())}}function Lt(G,Re){A?.setData(G,Re),yn.forEach((He,et)=>{let bt=G.find(Ge=>Ge.id===He.id);Ye[He.id]=!bt||bt.stale?"stale":bt.state==="error"?"error":bt.state==="running"?"running":"idle",B.setColorAt(et,new Ce(Ae[Ye[He.id]]))}),B.instanceColor.needsUpdate=!0,Qe()}function $e(){t||(t=!0,o++,C.abort(),w.forEach(G=>G.abort()),U.clear(),document.pointerLockElement===W&&document.exitPointerLock(),e.signal?.removeEventListener("abort",$e),H.forEach(G=>G()),S?.disconnect(),te.dispose(),A?.dispose(),b?.dispose(),ae?.dispose(),_e.dispose(),Y.dispose(),he.dispose(),ct.dispose(),J.dispose(),De.dispose(),Ne.dispose(),se.dispose(),Ue.dispose(),Bt(lt),Bt(nt),B.dispose(),be.dispose(),N.forEach(G=>G.dispose()),D.forEach(G=>G.dispose()),ve.passes.forEach(G=>G.dispose?.()),ve.dispose(),ge.dispose(),$.shadow.dispose(),L.dispose(),L.forceContextLoss(),W.remove(),I.clear(),M.clear())}A=Yp(z,yn,{traffic:J,society:ae.society,robotURL:e.resourceURL("/3d/system-world/white-robot.glb"),signal:e.signal,onError:e.onRobotError,active:()=>n&&!_&&i!=="map"&&!e.replaying?.(),obstacles:LM,shadows:ct}),b=Fm({scene:z,resourceURL:e.resourceURL,signal:C.signal,shadows:ct,onError:e.onCompanionError}),Y.setReducedMotion(!!f),se.setReducedMotion(!!f);try{nn(),await St()}catch(G){throw $e(),G}return{districts:yn,canvas:W,update:Et,focus(G){rs(),ss(G)},setMode:wn,setQuality:Vt,setData:Lt,dispose:$e,interact(){ae.interact()},socialAction:ae.socialAction,visit(G){wn("street"),ae.visit(G),fn()},enter(G){wn("street"),ae.destination(G),fn()},setEnvironment(G){_e.set(G),L.shadowMap.needsUpdate=!0},setVisible(G){n=G,G||(rt(0),ae.suspend(),A?.update(0,!1),U.clear(),Wn=null,document.pointerLockElement===W&&document.exitPointerLock())},setReducedMotion(G){f=G,Y.setReducedMotion(!!G),se.setReducedMotion(!!G),G&&(l=null,ae.endRide(),ae.society.suspend()),G&&i==="tour"&&wn("orbit")},setHologram(G,Re){Y.setTexts(G,Re),ae.setMemory(G)},setWorld(G,Re){ae.setWorld(G,Re)},moveKey(G,Re){Re?U.add(G):U.delete(G)},lockPointer(){if(i==="street")return W.requestPointerLock()},project(G){let Re=yn.find(et=>et.id===G);if(!Re)return null;ee.set(Re.x,Re.height+4,Re.z);let He=E.position.distanceTo(ee);return ee.project(E),{x:(ee.x+1)*y/2,y:(1-ee.y)*T/2,visible:ee.z<1&&ee.z>-1,distance:He}},capture(){return t||_?Promise.resolve(null):(ve.render(),new Promise(G=>W.toBlob(G,"image/png")))},intro(){if(f||i!=="orbit"||t)return;let G=fd.clone().multiplyScalar(E.aspect<1?1.3:1);E.position.copy(G).sub(sa).applyAxisAngle(new k(0,1,0),-.55).multiplyScalar(1.22).add(sa),E.position.y+=34,E.lookAt(sa),fn(),en(G,sa),l&&(l.duration=3.6)},stats(){return{flying:!!l,traffic:J.stats(),experience:ae.stats(),weather:_e.stats(),life:A?.stats(),hologram:Y.stats(),atmosphere:De.stats(),surfaces:Ne.stats(),windows:se.stats(),drones:he.stats(),flow:Ue.stats(),companion:b?.stats(),frames:c,tier:a,mode:i,focusedDistrict:m,hovered:ne,loadedBytes:p+ae.stats().bytes+Ne.stats().bytes,cachedModels:I.size,kitModels:{v1:[...I.keys()].filter(G=>G.startsWith("v1:")).length,v3:[...I.keys()].filter(G=>G.startsWith("v3:")).length},calls:L.info.render.calls,triangles:L.info.render.triangles,geometries:L.info.memory.geometries,position:E.position.toArray(),renderer:L.getContext().getParameter(L.getContext().getExtension("WEBGL_debug_renderer_info")?.UNMASKED_RENDERER_WEBGL||L.getContext().RENDERER),disposed:t}}}}export{AT as createCity,PM as createCityAmbience,yn as districts,DM as kitFor,Xm as kitPlacements,LM as obstacles,nr as placements,IM as sharedMaterialKey,TT as supersededV1};
/*! Bundled license information:

three/build/three.core.js:
three/build/three.module.js:
  (**
   * @license
   * Copyright 2010-2026 Three.js Authors
   * SPDX-License-Identifier: MIT
   *)
*/
