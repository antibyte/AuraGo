var ms={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2},gs={ROTATE:0,PAN:1,DOLLY_PAN:2,DOLLY_ROTATE:3},nf=0,Ch=1,sf=2;var Gs=1,rf=2,Or=3,ti=0,mn=1,Kt=2,ni=0,zr=1,tn=2,Ph=3,Ih=4,af=5;var Vs=100,of=101,lf=102,cf=103,hf=104,uf=200,df=201,ff=202,pf=203,Dh=204,Lh=205,mf=206,gf=207,xf=208,bf=209,vf=210,_f=211,yf=212,Mf=213,Sf=214,Yo=0,Jo=1,Zo=2,gr=3,$o=4,Qo=5,el=6,tl=7,Al=0,wf=1,Ef=2,xi=0,Va=1,Wa=2,qa=3,Ws=4,Xa=5,ja=6,Ka=7,bh="attached",Tf="detached",Fh=300,xs=301,qs=302,Rl=303,Cl=304,Ya=306,Ti=1e3,Qn=1001,xr=1002,on=1003,Pl=1004;var Xs=1005;var rn=1006,kr=1007;var bi=1008;var Gn=1009,Nh=1010,Uh=1011,Br=1012,Il=1013,vi=1014,Yn=1015,pn=1016,Dl=1017,Ll=1018,Hr=1020,Oh=35902,zh=35899,kh=1021,Bh=1022,Jn=1023,Ai=1026,bs=1027,Fl=1028,Nl=1029,vs=1030,Ul=1031;var Ol=1033,Ja=33776,Za=33777,$a=33778,Qa=33779,zl=35840,kl=35841,Bl=35842,Hl=35843,Gl=36196,Vl=37492,Wl=37496,ql=37488,Xl=37489,eo=37490,jl=37491,Kl=37808,Yl=37809,Jl=37810,Zl=37811,$l=37812,Ql=37813,ec=37814,tc=37815,nc=37816,ic=37817,sc=37818,rc=37819,ac=37820,oc=37821,lc=36492,cc=36494,hc=36495,uc=36283,dc=36284,to=36285,fc=36286,pc=2200,Af=2201,Rf=2202,Ls=2300,Fs=2301,Xo=2302,vh=2303,Ps=2400,Is=2401,ma=2402,mc=2500,Cf=2501,Hh=0,no=1,Gr=2,Pf=3200;var io=0,If=1,ii="",sn="srgb",Un="srgb-linear",ga="linear",Gt="srgb";var jo=7680;var Df=519,Lf=512,Ff=513,Nf=514,gc=515,Uf=516,Of=517,xc=518,zf=519,Gh=35044;var Vh="300 es",ui=2e3,br=2001;function Gm(s){for(let e=s.length-1;e>=0;--e)if(s[e]>=65535)return!0;return!1}function Vm(s){return ArrayBuffer.isView(s)&&!(s instanceof DataView)}function vr(s){return document.createElementNS("http://www.w3.org/1999/xhtml",s)}function kf(){let s=vr("canvas");return s.style.display="block",s}var fd={},_r=null;function xa(...s){let e="THREE."+s.shift();_r?_r("log",e,...s):console.log(e,...s)}function Bf(s){let e=s[0];if(typeof e=="string"&&e.startsWith("TSL:")){let t=s[1];t&&t.isStackTrace?s[0]+=" "+t.getLocation():s[1]='Stack trace not available. Enable "THREE.Node.captureStackTrace" to capture stack traces.'}return s}function rt(...s){s=Bf(s);let e="THREE."+s.shift();if(_r)_r("warn",e,...s);else{let t=s[0];t&&t.isStackTrace?console.warn(t.getError(e)):console.warn(e,...s)}}function pt(...s){s=Bf(s);let e="THREE."+s.shift();if(_r)_r("error",e,...s);else{let t=s[0];t&&t.isStackTrace?console.error(t.getError(e)):console.error(e,...s)}}function Ds(...s){let e=s.join(" ");e in fd||(fd[e]=!0,rt(...s))}function Hf(s,e,t){return new Promise(function(n,i){function r(){switch(s.clientWaitSync(e,s.SYNC_FLUSH_COMMANDS_BIT,0)){case s.WAIT_FAILED:i();break;case s.TIMEOUT_EXPIRED:setTimeout(r,t);break;default:n()}}setTimeout(r,t)})}var Gf={[Yo]:Jo,[Zo]:el,[$o]:tl,[gr]:Qo,[Jo]:Yo,[el]:Zo,[tl]:$o,[Qo]:gr},ei=class{addEventListener(e,t){this._listeners===void 0&&(this._listeners={});let n=this._listeners;n[e]===void 0&&(n[e]=[]),n[e].indexOf(t)===-1&&n[e].push(t)}hasEventListener(e,t){let n=this._listeners;return n===void 0?!1:n[e]!==void 0&&n[e].indexOf(t)!==-1}removeEventListener(e,t){let n=this._listeners;if(n===void 0)return;let i=n[e];if(i!==void 0){let r=i.indexOf(t);r!==-1&&i.splice(r,1)}}dispatchEvent(e){let t=this._listeners;if(t===void 0)return;let n=t[e.type];if(n!==void 0){e.target=this;let i=n.slice(0);for(let r=0,a=i.length;r<a;r++)i[r].call(this,e);e.target=null}}},An=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],pd=1234567,ua=Math.PI/180,Ns=180/Math.PI;function di(){let s=Math.random()*4294967295|0,e=Math.random()*4294967295|0,t=Math.random()*4294967295|0,n=Math.random()*4294967295|0;return(An[s&255]+An[s>>8&255]+An[s>>16&255]+An[s>>24&255]+"-"+An[e&255]+An[e>>8&255]+"-"+An[e>>16&15|64]+An[e>>24&255]+"-"+An[t&63|128]+An[t>>8&255]+"-"+An[t>>16&255]+An[t>>24&255]+An[n&255]+An[n>>8&255]+An[n>>16&255]+An[n>>24&255]).toLowerCase()}function At(s,e,t){return Math.max(e,Math.min(t,s))}function Wh(s,e){return(s%e+e)%e}function Wm(s,e,t,n,i){return n+(s-e)*(i-n)/(t-e)}function qm(s,e,t){return s!==e?(t-s)/(e-s):0}function da(s,e,t){return(1-t)*s+t*e}function Xm(s,e,t,n){return da(s,e,1-Math.exp(-t*n))}function jm(s,e=1){return e-Math.abs(Wh(s,e*2)-e)}function Km(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*(3-2*s))}function Ym(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*s*(s*(s*6-15)+10))}function Jm(s,e){return s+Math.floor(Math.random()*(e-s+1))}function Zm(s,e){return s+Math.random()*(e-s)}function $m(s){return s*(.5-Math.random())}function Qm(s){s!==void 0&&(pd=s);let e=pd+=1831565813;return e=Math.imul(e^e>>>15,e|1),e^=e+Math.imul(e^e>>>7,e|61),((e^e>>>14)>>>0)/4294967296}function eg(s){return s*ua}function tg(s){return s*Ns}function ng(s){return s>0&&Number.isInteger(s)&&2**Math.round(Math.log2(s))===s}function ig(s){return Math.pow(2,Math.ceil(Math.log(s)/Math.LN2))}function sg(s){return Math.pow(2,Math.floor(Math.log(s)/Math.LN2))}function rg(s,e,t,n,i){let r=Math.cos,a=Math.sin,o=r(t/2),l=a(t/2),c=r((e+n)/2),h=a((e+n)/2),u=r((e-n)/2),d=a((e-n)/2),p=r((n-e)/2),x=a((n-e)/2);switch(i){case"XYX":s.set(o*h,l*u,l*d,o*c);break;case"YZY":s.set(l*d,o*h,l*u,o*c);break;case"ZXZ":s.set(l*u,l*d,o*h,o*c);break;case"XZX":s.set(o*h,l*x,l*p,o*c);break;case"YXY":s.set(l*p,o*h,l*x,o*c);break;case"ZYZ":s.set(l*x,l*p,o*h,o*c);break;default:rt("MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+i)}}function hi(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return s/4294967295;case Uint16Array:return s/65535;case Uint8Array:case Uint8ClampedArray:return s/255;case Int32Array:return Math.max(s/2147483647,-1);case Int16Array:return Math.max(s/32767,-1);case Int8Array:return Math.max(s/127,-1);default:throw new Error("THREE.MathUtils: Invalid component type.")}}function jt(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return Math.round(s*4294967295);case Uint16Array:return Math.round(s*65535);case Uint8Array:case Uint8ClampedArray:return Math.round(s*255);case Int32Array:return Math.round(s*2147483647);case Int16Array:return Math.round(s*32767);case Int8Array:return Math.round(s*127);default:throw new Error("THREE.MathUtils: Invalid component type.")}}var Ft={DEG2RAD:ua,RAD2DEG:Ns,generateUUID:di,clamp:At,euclideanModulo:Wh,mapLinear:Wm,inverseLerp:qm,lerp:da,damp:Xm,pingpong:jm,smoothstep:Km,smootherstep:Ym,randInt:Jm,randFloat:Zm,randFloatSpread:$m,seededRandom:Qm,degToRad:eg,radToDeg:tg,isPowerOfTwo:ng,ceilPowerOfTwo:ig,floorPowerOfTwo:sg,setQuaternionFromProperEuler:rg,normalize:jt,denormalize:hi},Be=class s{static{s.prototype.isVector2=!0}constructor(e=0,t=0){this.x=e,this.y=t}get width(){return this.x}set width(e){this.x=e}get height(){return this.y}set height(e){this.y=e}set(e,t){return this.x=e,this.y=t,this}setScalar(e){return this.x=e,this.y=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;default:throw new Error("THREE.Vector2: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;default:throw new Error("THREE.Vector2: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y)}copy(e){return this.x=e.x,this.y=e.y,this}add(e){return this.x+=e.x,this.y+=e.y,this}addScalar(e){return this.x+=e,this.y+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this}subScalar(e){return this.x-=e,this.y-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this}multiply(e){return this.x*=e.x,this.y*=e.y,this}multiplyScalar(e){return this.x*=e,this.y*=e,this}divide(e){return this.x/=e.x,this.y/=e.y,this}divideScalar(e){return this.multiplyScalar(1/e)}applyMatrix3(e){let t=this.x,n=this.y,i=e.elements;return this.x=i[0]*t+i[3]*n+i[6],this.y=i[1]*t+i[4]*n+i[7],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this}clamp(e,t){return this.x=At(this.x,e.x,t.x),this.y=At(this.y,e.y,t.y),this}clampScalar(e,t){return this.x=At(this.x,e,t),this.y=At(this.y,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(At(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(e){return this.x*e.x+this.y*e.y}cross(e){return this.x*e.y-this.y*e.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(At(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y;return t*t+n*n}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this}equals(e){return e.x===this.x&&e.y===this.y}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this}rotateAround(e,t){let n=Math.cos(t),i=Math.sin(t),r=this.x-e.x,a=this.y-e.y;return this.x=r*n-a*i+e.x,this.y=r*i+a*n+e.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},Qt=class{constructor(e=0,t=0,n=0,i=1){this.isQuaternion=!0,this._x=e,this._y=t,this._z=n,this._w=i}static slerpFlat(e,t,n,i,r,a,o){let l=n[i+0],c=n[i+1],h=n[i+2],u=n[i+3],d=r[a+0],p=r[a+1],x=r[a+2],v=r[a+3];if(u!==v||l!==d||c!==p||h!==x){let m=l*d+c*p+h*x+u*v;m<0&&(d=-d,p=-p,x=-x,v=-v,m=-m);let f=1-o;if(m<.9995){let y=Math.acos(m),T=Math.sin(y);f=Math.sin(f*y)/T,o=Math.sin(o*y)/T,l=l*f+d*o,c=c*f+p*o,h=h*f+x*o,u=u*f+v*o}else{l=l*f+d*o,c=c*f+p*o,h=h*f+x*o,u=u*f+v*o;let y=1/Math.sqrt(l*l+c*c+h*h+u*u);l*=y,c*=y,h*=y,u*=y}}e[t]=l,e[t+1]=c,e[t+2]=h,e[t+3]=u}static multiplyQuaternionsFlat(e,t,n,i,r,a){let o=n[i],l=n[i+1],c=n[i+2],h=n[i+3],u=r[a],d=r[a+1],p=r[a+2],x=r[a+3];return e[t]=o*x+h*u+l*p-c*d,e[t+1]=l*x+h*d+c*u-o*p,e[t+2]=c*x+h*p+o*d-l*u,e[t+3]=h*x-o*u-l*d-c*p,e}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get w(){return this._w}set w(e){this._w=e,this._onChangeCallback()}set(e,t,n,i){return this._x=e,this._y=t,this._z=n,this._w=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(e){return this._x=e.x,this._y=e.y,this._z=e.z,this._w=e.w,this._onChangeCallback(),this}setFromEuler(e,t=!0){let n=e._x,i=e._y,r=e._z,a=e._order,o=Math.cos,l=Math.sin,c=o(n/2),h=o(i/2),u=o(r/2),d=l(n/2),p=l(i/2),x=l(r/2);switch(a){case"XYZ":this._x=d*h*u+c*p*x,this._y=c*p*u-d*h*x,this._z=c*h*x+d*p*u,this._w=c*h*u-d*p*x;break;case"YXZ":this._x=d*h*u+c*p*x,this._y=c*p*u-d*h*x,this._z=c*h*x-d*p*u,this._w=c*h*u+d*p*x;break;case"ZXY":this._x=d*h*u-c*p*x,this._y=c*p*u+d*h*x,this._z=c*h*x+d*p*u,this._w=c*h*u-d*p*x;break;case"ZYX":this._x=d*h*u-c*p*x,this._y=c*p*u+d*h*x,this._z=c*h*x-d*p*u,this._w=c*h*u+d*p*x;break;case"YZX":this._x=d*h*u+c*p*x,this._y=c*p*u+d*h*x,this._z=c*h*x-d*p*u,this._w=c*h*u-d*p*x;break;case"XZY":this._x=d*h*u-c*p*x,this._y=c*p*u-d*h*x,this._z=c*h*x+d*p*u,this._w=c*h*u+d*p*x;break;default:rt("Quaternion: .setFromEuler() encountered an unknown order: "+a)}return t===!0&&this._onChangeCallback(),this}setFromAxisAngle(e,t){let n=t/2,i=Math.sin(n);return this._x=e.x*i,this._y=e.y*i,this._z=e.z*i,this._w=Math.cos(n),this._onChangeCallback(),this}setFromRotationMatrix(e){let t=e.elements,n=t[0],i=t[4],r=t[8],a=t[1],o=t[5],l=t[9],c=t[2],h=t[6],u=t[10],d=n+o+u;if(d>0){let p=.5/Math.sqrt(d+1);this._w=.25/p,this._x=(h-l)*p,this._y=(r-c)*p,this._z=(a-i)*p}else if(n>o&&n>u){let p=2*Math.sqrt(1+n-o-u);this._w=(h-l)/p,this._x=.25*p,this._y=(i+a)/p,this._z=(r+c)/p}else if(o>u){let p=2*Math.sqrt(1+o-n-u);this._w=(r-c)/p,this._x=(i+a)/p,this._y=.25*p,this._z=(l+h)/p}else{let p=2*Math.sqrt(1+u-n-o);this._w=(a-i)/p,this._x=(r+c)/p,this._y=(l+h)/p,this._z=.25*p}return this._onChangeCallback(),this}setFromUnitVectors(e,t){let n=e.dot(t)+1;return n<1e-8?(n=0,Math.abs(e.x)>Math.abs(e.z)?(this._x=-e.y,this._y=e.x,this._z=0,this._w=n):(this._x=0,this._y=-e.z,this._z=e.y,this._w=n)):(this._x=e.y*t.z-e.z*t.y,this._y=e.z*t.x-e.x*t.z,this._z=e.x*t.y-e.y*t.x,this._w=n),this.normalize()}angleTo(e){return 2*Math.acos(Math.abs(At(this.dot(e),-1,1)))}rotateTowards(e,t){let n=this.angleTo(e);if(n===0)return this;let i=Math.min(1,t/n);return this.slerp(e,i),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(e){return this._x*e._x+this._y*e._y+this._z*e._z+this._w*e._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let e=this.length();return e===0?(this._x=0,this._y=0,this._z=0,this._w=1):(e=1/e,this._x=this._x*e,this._y=this._y*e,this._z=this._z*e,this._w=this._w*e),this._onChangeCallback(),this}multiply(e){return this.multiplyQuaternions(this,e)}premultiply(e){return this.multiplyQuaternions(e,this)}multiplyQuaternions(e,t){let n=e._x,i=e._y,r=e._z,a=e._w,o=t._x,l=t._y,c=t._z,h=t._w;return this._x=n*h+a*o+i*c-r*l,this._y=i*h+a*l+r*o-n*c,this._z=r*h+a*c+n*l-i*o,this._w=a*h-n*o-i*l-r*c,this._onChangeCallback(),this}slerp(e,t){let n=e._x,i=e._y,r=e._z,a=e._w,o=this.dot(e);o<0&&(n=-n,i=-i,r=-r,a=-a,o=-o);let l=1-t;if(o<.9995){let c=Math.acos(o),h=Math.sin(c);l=Math.sin(l*c)/h,t=Math.sin(t*c)/h,this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+a*t,this._onChangeCallback()}else this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+a*t,this.normalize();return this}slerpQuaternions(e,t,n){return this.copy(e).slerp(t,n)}random(){let e=2*Math.PI*Math.random(),t=2*Math.PI*Math.random(),n=Math.random(),i=Math.sqrt(1-n),r=Math.sqrt(n);return this.set(i*Math.sin(e),i*Math.cos(e),r*Math.sin(t),r*Math.cos(t))}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._w===this._w}fromArray(e,t=0){return this._x=e[t],this._y=e[t+1],this._z=e[t+2],this._w=e[t+3],this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._w,e}fromBufferAttribute(e,t){return this._x=e.getX(t),this._y=e.getY(t),this._z=e.getZ(t),this._w=e.getW(t),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},B=class s{static{s.prototype.isVector3=!0}constructor(e=0,t=0,n=0){this.x=e,this.y=t,this.z=n}set(e,t,n){return n===void 0&&(n=this.z),this.x=e,this.y=t,this.z=n,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;default:throw new Error("THREE.Vector3: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("THREE.Vector3: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this}multiplyVectors(e,t){return this.x=e.x*t.x,this.y=e.y*t.y,this.z=e.z*t.z,this}applyEuler(e){return this.applyQuaternion(md.setFromEuler(e))}applyAxisAngle(e,t){return this.applyQuaternion(md.setFromAxisAngle(e,t))}applyMatrix3(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[3]*n+r[6]*i,this.y=r[1]*t+r[4]*n+r[7]*i,this.z=r[2]*t+r[5]*n+r[8]*i,this}applyNormalMatrix(e){return this.applyMatrix3(e).normalize()}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=e.elements,a=1/(r[3]*t+r[7]*n+r[11]*i+r[15]);return this.x=(r[0]*t+r[4]*n+r[8]*i+r[12])*a,this.y=(r[1]*t+r[5]*n+r[9]*i+r[13])*a,this.z=(r[2]*t+r[6]*n+r[10]*i+r[14])*a,this}applyQuaternion(e){let t=this.x,n=this.y,i=this.z,r=e.x,a=e.y,o=e.z,l=e.w,c=2*(a*i-o*n),h=2*(o*t-r*i),u=2*(r*n-a*t);return this.x=t+l*c+a*u-o*h,this.y=n+l*h+o*c-r*u,this.z=i+l*u+r*h-a*c,this}project(e){return this.applyMatrix4(e.matrixWorldInverse).applyMatrix4(e.projectionMatrix)}unproject(e){return this.applyMatrix4(e.projectionMatrixInverse).applyMatrix4(e.matrixWorld)}transformDirection(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[4]*n+r[8]*i,this.y=r[1]*t+r[5]*n+r[9]*i,this.z=r[2]*t+r[6]*n+r[10]*i,this.normalize()}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this}divideScalar(e){return this.multiplyScalar(1/e)}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this}clamp(e,t){return this.x=At(this.x,e.x,t.x),this.y=At(this.y,e.y,t.y),this.z=At(this.z,e.z,t.z),this}clampScalar(e,t){return this.x=At(this.x,e,t),this.y=At(this.y,e,t),this.z=At(this.z,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(At(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this}cross(e){return this.crossVectors(this,e)}crossVectors(e,t){let n=e.x,i=e.y,r=e.z,a=t.x,o=t.y,l=t.z;return this.x=i*l-r*o,this.y=r*a-n*l,this.z=n*o-i*a,this}projectOnVector(e){let t=e.lengthSq();if(t===0)return this.set(0,0,0);let n=e.dot(this)/t;return this.copy(e).multiplyScalar(n)}projectOnPlane(e){return qc.copy(this).projectOnVector(e),this.sub(qc)}reflect(e){return this.sub(qc.copy(e).multiplyScalar(2*this.dot(e)))}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(At(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y,i=this.z-e.z;return t*t+n*n+i*i}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)+Math.abs(this.z-e.z)}setFromSpherical(e){return this.setFromSphericalCoords(e.radius,e.phi,e.theta)}setFromSphericalCoords(e,t,n){let i=Math.sin(t)*e;return this.x=i*Math.sin(n),this.y=Math.cos(t)*e,this.z=i*Math.cos(n),this}setFromCylindrical(e){return this.setFromCylindricalCoords(e.radius,e.theta,e.y)}setFromCylindricalCoords(e,t,n){return this.x=e*Math.sin(t),this.y=n,this.z=e*Math.cos(t),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this}setFromMatrixScale(e){let t=this.setFromMatrixColumn(e,0).length(),n=this.setFromMatrixColumn(e,1).length(),i=this.setFromMatrixColumn(e,2).length();return this.x=t,this.y=n,this.z=i,this}setFromMatrixColumn(e,t){return this.fromArray(e.elements,t*4)}setFromMatrix3Column(e,t){return this.fromArray(e.elements,t*3)}setFromEuler(e){return this.x=e._x,this.y=e._y,this.z=e._z,this}setFromColor(e){return this.x=e.r,this.y=e.g,this.z=e.b,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let e=Math.random()*Math.PI*2,t=Math.random()*2-1,n=Math.sqrt(1-t*t);return this.x=n*Math.cos(e),this.y=t,this.z=n*Math.sin(e),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},qc=new B,md=new Qt,xt=class s{static{s.prototype.isMatrix3=!0}constructor(e,t,n,i,r,a,o,l,c){this.elements=[1,0,0,0,1,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,a,o,l,c)}set(e,t,n,i,r,a,o,l,c){let h=this.elements;return h[0]=e,h[1]=i,h[2]=o,h[3]=t,h[4]=r,h[5]=l,h[6]=n,h[7]=a,h[8]=c,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],this}extractBasis(e,t,n){return e.setFromMatrix3Column(this,0),t.setFromMatrix3Column(this,1),n.setFromMatrix3Column(this,2),this}setFromMatrix4(e){let t=e.elements;return this.set(t[0],t[4],t[8],t[1],t[5],t[9],t[2],t[6],t[10]),this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,a=n[0],o=n[3],l=n[6],c=n[1],h=n[4],u=n[7],d=n[2],p=n[5],x=n[8],v=i[0],m=i[3],f=i[6],y=i[1],T=i[4],g=i[7],_=i[2],w=i[5],A=i[8];return r[0]=a*v+o*y+l*_,r[3]=a*m+o*T+l*w,r[6]=a*f+o*g+l*A,r[1]=c*v+h*y+u*_,r[4]=c*m+h*T+u*w,r[7]=c*f+h*g+u*A,r[2]=d*v+p*y+x*_,r[5]=d*m+p*T+x*w,r[8]=d*f+p*g+x*A,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[3]*=e,t[6]*=e,t[1]*=e,t[4]*=e,t[7]*=e,t[2]*=e,t[5]*=e,t[8]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8];return t*a*h-t*o*c-n*r*h+n*o*l+i*r*c-i*a*l}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8],u=h*a-o*c,d=o*l-h*r,p=c*r-a*l,x=t*u+n*d+i*p;if(x===0)return this.set(0,0,0,0,0,0,0,0,0);let v=1/x;return e[0]=u*v,e[1]=(i*c-h*n)*v,e[2]=(o*n-i*a)*v,e[3]=d*v,e[4]=(h*t-i*l)*v,e[5]=(i*r-o*t)*v,e[6]=p*v,e[7]=(n*l-c*t)*v,e[8]=(a*t-n*r)*v,this}transpose(){let e,t=this.elements;return e=t[1],t[1]=t[3],t[3]=e,e=t[2],t[2]=t[6],t[6]=e,e=t[5],t[5]=t[7],t[7]=e,this}getNormalMatrix(e){return this.setFromMatrix4(e).invert().transpose()}transposeIntoArray(e){let t=this.elements;return e[0]=t[0],e[1]=t[3],e[2]=t[6],e[3]=t[1],e[4]=t[4],e[5]=t[7],e[6]=t[2],e[7]=t[5],e[8]=t[8],this}setUvTransform(e,t,n,i,r,a,o){let l=Math.cos(r),c=Math.sin(r);return this.set(n*l,n*c,-n*(l*a+c*o)+a+e,-i*c,i*l,-i*(-c*a+l*o)+o+t,0,0,1),this}scale(e,t){return Ds("Matrix3: .scale() is deprecated. Use .makeScale() instead."),this.premultiply(Xc.makeScale(e,t)),this}rotate(e){return Ds("Matrix3: .rotate() is deprecated. Use .makeRotation() instead."),this.premultiply(Xc.makeRotation(-e)),this}translate(e,t){return Ds("Matrix3: .translate() is deprecated. Use .makeTranslation() instead."),this.premultiply(Xc.makeTranslation(e,t)),this}makeTranslation(e,t){return e.isVector2?this.set(1,0,e.x,0,1,e.y,0,0,1):this.set(1,0,e,0,1,t,0,0,1),this}makeRotation(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,n,t,0,0,0,1),this}makeScale(e,t){return this.set(e,0,0,0,t,0,0,0,1),this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<9;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<9;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e}clone(){return new this.constructor().fromArray(this.elements)}},Xc=new xt,gd=new xt().set(.4123908,.3575843,.1804808,.212639,.7151687,.0721923,.0193308,.1191948,.9505322),xd=new xt().set(3.2409699,-1.5373832,-.4986108,-.9692436,1.8759675,.0415551,.0556301,-.203977,1.0569715);function ag(){let s={enabled:!0,workingColorSpace:Un,spaces:{},convert:function(i,r,a){return this.enabled===!1||r===a||!r||!a||(this.spaces[r].transfer===Gt&&(i.r=Xi(i.r),i.g=Xi(i.g),i.b=Xi(i.b)),this.spaces[r].primaries!==this.spaces[a].primaries&&(i.applyMatrix3(this.spaces[r].toXYZ),i.applyMatrix3(this.spaces[a].fromXYZ)),this.spaces[a].transfer===Gt&&(i.r=mr(i.r),i.g=mr(i.g),i.b=mr(i.b))),i},workingToColorSpace:function(i,r){return this.convert(i,this.workingColorSpace,r)},colorSpaceToWorking:function(i,r){return this.convert(i,r,this.workingColorSpace)},getPrimaries:function(i){return this.spaces[i].primaries},getTransfer:function(i){return i===ii?ga:this.spaces[i].transfer},getToneMappingMode:function(i){return this.spaces[i].outputColorSpaceConfig.toneMappingMode||"standard"},getLuminanceCoefficients:function(i,r=this.workingColorSpace){return i.fromArray(this.spaces[r].luminanceCoefficients)},define:function(i){Object.assign(this.spaces,i)},_getMatrix:function(i,r,a){return i.copy(this.spaces[r].toXYZ).multiply(this.spaces[a].fromXYZ)},_getDrawingBufferColorSpace:function(i){return this.spaces[i].outputColorSpaceConfig.drawingBufferColorSpace},_getUnpackColorSpace:function(i=this.workingColorSpace){return this.spaces[i].workingColorSpaceConfig.unpackColorSpace},fromWorkingColorSpace:function(i,r){return Ds("ColorManagement: .fromWorkingColorSpace() has been renamed to .workingToColorSpace()."),s.workingToColorSpace(i,r)},toWorkingColorSpace:function(i,r){return Ds("ColorManagement: .toWorkingColorSpace() has been renamed to .colorSpaceToWorking()."),s.colorSpaceToWorking(i,r)}},e=[.64,.33,.3,.6,.15,.06],t=[.2126,.7152,.0722],n=[.3127,.329];return s.define({[Un]:{primaries:e,whitePoint:n,transfer:ga,toXYZ:gd,fromXYZ:xd,luminanceCoefficients:t,workingColorSpaceConfig:{unpackColorSpace:sn},outputColorSpaceConfig:{drawingBufferColorSpace:sn}},[sn]:{primaries:e,whitePoint:n,transfer:Gt,toXYZ:gd,fromXYZ:xd,luminanceCoefficients:t,outputColorSpaceConfig:{drawingBufferColorSpace:sn}}}),s}var Tt=ag();function Xi(s){return s<.04045?s*.0773993808:Math.pow(s*.9478672986+.0521327014,2.4)}function mr(s){return s<.0031308?s*12.92:1.055*Math.pow(s,.41666)-.055}var tr,nl=class{static getDataURL(e,t="image/png"){if(/^data:/i.test(e.src)||typeof HTMLCanvasElement>"u")return e.src;let n;if(e instanceof HTMLCanvasElement)n=e;else{tr===void 0&&(tr=vr("canvas")),tr.width=e.width,tr.height=e.height;let i=tr.getContext("2d");e instanceof ImageData?i.putImageData(e,0,0):i.drawImage(e,0,0,e.width,e.height),n=tr}return n.toDataURL(t)}static sRGBToLinear(e){if(typeof HTMLImageElement<"u"&&e instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&e instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&e instanceof ImageBitmap){let t=vr("canvas");t.width=e.width,t.height=e.height;let n=t.getContext("2d");n.drawImage(e,0,0,e.width,e.height);let i=n.getImageData(0,0,e.width,e.height),r=i.data;for(let a=0;a<r.length;a++)r[a]=Xi(r[a]/255)*255;return n.putImageData(i,0,0),t}else if(e.data){let t=e.data.slice(0);for(let n=0;n<t.length;n++)t instanceof Uint8Array||t instanceof Uint8ClampedArray?t[n]=Math.floor(Xi(t[n]/255)*255):t[n]=Xi(t[n]);return{data:t,width:e.width,height:e.height}}else return rt("ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),e}},og=0,yr=class{constructor(e=null){this.isTextureSource=!0,Object.defineProperty(this,"id",{value:og++}),this.uuid=di(),this.data=e,this.dataReady=!0,this.version=0}getSize(e){let t=this.data;return typeof HTMLVideoElement<"u"&&t instanceof HTMLVideoElement?e.set(t.videoWidth,t.videoHeight,0):typeof VideoFrame<"u"&&t instanceof VideoFrame?e.set(t.displayWidth,t.displayHeight,0):t!==null?e.set(t.width,t.height,t.depth||0):e.set(0,0,0),e}set needsUpdate(e){e===!0&&this.version++}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.images[this.uuid]!==void 0)return e.images[this.uuid];let n={uuid:this.uuid,url:""},i=this.data;if(i!==null){let r;if(Array.isArray(i)){r=[];for(let a=0,o=i.length;a<o;a++)i[a].isDataTexture?r.push(jc(i[a].image)):r.push(jc(i[a]))}else r=jc(i);n.url=r}return t||(e.images[this.uuid]=n),n}};function jc(s){return typeof HTMLImageElement<"u"&&s instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&s instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&s instanceof ImageBitmap?nl.getDataURL(s):s.data?{data:Array.from(s.data),width:s.width,height:s.height,type:s.data.constructor.name}:(rt("Texture: Unable to serialize Texture."),{})}var lg=0,Kc=new B,ln=class s extends ei{constructor(e=s.DEFAULT_IMAGE,t=s.DEFAULT_MAPPING,n=Qn,i=Qn,r=rn,a=bi,o=Jn,l=Gn,c=s.DEFAULT_ANISOTROPY,h=ii){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:lg++}),this.uuid=di(),this.name="",this.source=new yr(e),this.mipmaps=[],this.mapping=t,this.channel=0,this.wrapS=n,this.wrapT=i,this.magFilter=r,this.minFilter=a,this.anisotropy=c,this.format=o,this.internalFormat=null,this.type=l,this.offset=new Be(0,0),this.repeat=new Be(1,1),this.center=new Be(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new xt,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=h,this.userData={},this.updateRanges=[],this.version=0,this.onUpdate=null,this.renderTarget=null,this.isRenderTargetTexture=!1,this.isArrayTexture=!!(e&&e.depth&&e.depth>1),this.pmremVersion=0,this.normalized=!1}get width(){return this.source.getSize(Kc).x}get height(){return this.source.getSize(Kc).y}get depth(){return this.source.getSize(Kc).z}get image(){return this.source.data}set image(e){this.source.data=e}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}clone(){return new this.constructor().copy(this)}copy(e){return this.name=e.name,this.source=e.source,this.mipmaps=e.mipmaps.slice(0),this.mapping=e.mapping,this.channel=e.channel,this.wrapS=e.wrapS,this.wrapT=e.wrapT,this.magFilter=e.magFilter,this.minFilter=e.minFilter,this.anisotropy=e.anisotropy,this.format=e.format,this.internalFormat=e.internalFormat,this.type=e.type,this.normalized=e.normalized,this.offset.copy(e.offset),this.repeat.copy(e.repeat),this.center.copy(e.center),this.rotation=e.rotation,this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrix.copy(e.matrix),this.generateMipmaps=e.generateMipmaps,this.premultiplyAlpha=e.premultiplyAlpha,this.flipY=e.flipY,this.unpackAlignment=e.unpackAlignment,this.colorSpace=e.colorSpace,this.renderTarget=e.renderTarget,this.isRenderTargetTexture=e.isRenderTargetTexture,this.isArrayTexture=e.isArrayTexture,this.userData=JSON.parse(JSON.stringify(e.userData)),this.needsUpdate=!0,this}setValues(e){for(let t in e){let n=e[t];if(n===void 0){rt(`Texture.setValues(): parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){rt(`Texture.setValues(): property '${t}' does not exist.`);continue}i&&n&&i.isVector2&&n.isVector2||i&&n&&i.isVector3&&n.isVector3||i&&n&&i.isMatrix3&&n.isMatrix3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.textures[this.uuid]!==void 0)return e.textures[this.uuid];let n={metadata:{version:4.7,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(e).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,normalized:this.normalized,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(n.userData=this.userData),t||(e.textures[this.uuid]=n),n}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(e){if(this.mapping!==Fh)return e;if(e.applyMatrix3(this.matrix),e.x<0||e.x>1)switch(this.wrapS){case Ti:e.x=e.x-Math.floor(e.x);break;case Qn:e.x=e.x<0?0:1;break;case xr:Math.abs(Math.floor(e.x)%2)===1?e.x=Math.ceil(e.x)-e.x:e.x=e.x-Math.floor(e.x);break}if(e.y<0||e.y>1)switch(this.wrapT){case Ti:e.y=e.y-Math.floor(e.y);break;case Qn:e.y=e.y<0?0:1;break;case xr:Math.abs(Math.floor(e.y)%2)===1?e.y=Math.ceil(e.y)-e.y:e.y=e.y-Math.floor(e.y);break}return this.flipY&&(e.y=1-e.y),e}set needsUpdate(e){e===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(e){e===!0&&this.pmremVersion++}};ln.DEFAULT_IMAGE=null;ln.DEFAULT_MAPPING=Fh;ln.DEFAULT_ANISOTROPY=1;var Ot=class s{static{s.prototype.isVector4=!0}constructor(e=0,t=0,n=0,i=1){this.x=e,this.y=t,this.z=n,this.w=i}get width(){return this.z}set width(e){this.z=e}get height(){return this.w}set height(e){this.w=e}set(e,t,n,i){return this.x=e,this.y=t,this.z=n,this.w=i,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this.w=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setW(e){return this.w=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;case 3:this.w=t;break;default:throw new Error("THREE.Vector4: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("THREE.Vector4: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this.w=e.w!==void 0?e.w:1,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this.w+=e.w,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this.w+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this.w=e.w+t.w,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this.w+=e.w*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this.w-=e.w,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this.w-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this.w=e.w-t.w,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this.w*=e.w,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this.w*=e,this}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=this.w,a=e.elements;return this.x=a[0]*t+a[4]*n+a[8]*i+a[12]*r,this.y=a[1]*t+a[5]*n+a[9]*i+a[13]*r,this.z=a[2]*t+a[6]*n+a[10]*i+a[14]*r,this.w=a[3]*t+a[7]*n+a[11]*i+a[15]*r,this}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this.w/=e.w,this}divideScalar(e){return this.multiplyScalar(1/e)}setAxisAngleFromQuaternion(e){this.w=2*Math.acos(e.w);let t=Math.sqrt(1-e.w*e.w);return t<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=e.x/t,this.y=e.y/t,this.z=e.z/t),this}setAxisAngleFromRotationMatrix(e){let t,n,i,r,l=e.elements,c=l[0],h=l[4],u=l[8],d=l[1],p=l[5],x=l[9],v=l[2],m=l[6],f=l[10];if(Math.abs(h-d)<.01&&Math.abs(u-v)<.01&&Math.abs(x-m)<.01){if(Math.abs(h+d)<.1&&Math.abs(u+v)<.1&&Math.abs(x+m)<.1&&Math.abs(c+p+f-3)<.1)return this.set(1,0,0,0),this;t=Math.PI;let T=(c+1)/2,g=(p+1)/2,_=(f+1)/2,w=(h+d)/4,A=(u+v)/4,b=(x+m)/4;return T>g&&T>_?T<.01?(n=0,i=.707106781,r=.707106781):(n=Math.sqrt(T),i=w/n,r=A/n):g>_?g<.01?(n=.707106781,i=0,r=.707106781):(i=Math.sqrt(g),n=w/i,r=b/i):_<.01?(n=.707106781,i=.707106781,r=0):(r=Math.sqrt(_),n=A/r,i=b/r),this.set(n,i,r,t),this}let y=Math.sqrt((m-x)*(m-x)+(u-v)*(u-v)+(d-h)*(d-h));return Math.abs(y)<.001&&(y=1),this.x=(m-x)/y,this.y=(u-v)/y,this.z=(d-h)/y,this.w=Math.acos((c+p+f-1)/2),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this.w=t[15],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this.w=Math.min(this.w,e.w),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this.w=Math.max(this.w,e.w),this}clamp(e,t){return this.x=At(this.x,e.x,t.x),this.y=At(this.y,e.y,t.y),this.z=At(this.z,e.z,t.z),this.w=At(this.w,e.w,t.w),this}clampScalar(e,t){return this.x=At(this.x,e,t),this.y=At(this.y,e,t),this.z=At(this.z,e,t),this.w=At(this.w,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(At(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z+this.w*e.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this.w+=(e.w-this.w)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this.w=e.w+(t.w-e.w)*n,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z&&e.w===this.w}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this.w=e[t+3],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e[t+3]=this.w,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this.w=e.getW(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},il=class extends ei{constructor(e=1,t=1,n={}){super(),n=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:rn,depthBuffer:!0,stencilBuffer:!1,resolveColorBuffer:!0,resolveDepthBuffer:!0,resolveStencilBuffer:!0,storeMultisampledColorBuffer:!0,storeMultisampledDepthBuffer:!0,storeMultisampledStencilBuffer:!0,depthTexture:null,samples:0,count:1,depth:1,multiview:!1,useArrayDepthTexture:!1},n),this.isRenderTarget=!0,this.width=e,this.height=t,this.depth=n.depth,this.scissor=new Ot(0,0,e,t),this.scissorTest=!1,this.viewport=new Ot(0,0,e,t),this.textures=[];let i={width:e,height:t,depth:n.depth},r=new ln(i),a=n.count;for(let o=0;o<a;o++)this.textures[o]=r.clone(),this.textures[o].isRenderTargetTexture=!0,this.textures[o].renderTarget=this;this._setTextureOptions(n),this.depthBuffer=n.depthBuffer,this.stencilBuffer=n.stencilBuffer,this.resolveColorBuffer=n.resolveColorBuffer,this.resolveDepthBuffer=n.resolveDepthBuffer,this.resolveStencilBuffer=n.resolveStencilBuffer,this.storeMultisampledColorBuffer=n.storeMultisampledColorBuffer,this.storeMultisampledDepthBuffer=n.storeMultisampledDepthBuffer,this.storeMultisampledStencilBuffer=n.storeMultisampledStencilBuffer,this._depthTexture=null,this.depthTexture=n.depthTexture,this.samples=n.samples,this.multiview=n.multiview,this.useArrayDepthTexture=n.useArrayDepthTexture}_setTextureOptions(e={}){let t={minFilter:rn,generateMipmaps:!1,flipY:!1,internalFormat:null};e.mapping!==void 0&&(t.mapping=e.mapping),e.wrapS!==void 0&&(t.wrapS=e.wrapS),e.wrapT!==void 0&&(t.wrapT=e.wrapT),e.wrapR!==void 0&&(t.wrapR=e.wrapR),e.magFilter!==void 0&&(t.magFilter=e.magFilter),e.minFilter!==void 0&&(t.minFilter=e.minFilter),e.format!==void 0&&(t.format=e.format),e.type!==void 0&&(t.type=e.type),e.anisotropy!==void 0&&(t.anisotropy=e.anisotropy),e.colorSpace!==void 0&&(t.colorSpace=e.colorSpace),e.flipY!==void 0&&(t.flipY=e.flipY),e.generateMipmaps!==void 0&&(t.generateMipmaps=e.generateMipmaps),e.internalFormat!==void 0&&(t.internalFormat=e.internalFormat);for(let n=0;n<this.textures.length;n++)this.textures[n].setValues(t)}get texture(){return this.textures[0]}set texture(e){this.textures[0]=e}set depthTexture(e){this._depthTexture!==null&&this._depthTexture.renderTarget===this&&(this._depthTexture.renderTarget=null),e!==null&&e.renderTarget===null&&(e.renderTarget=this),this._depthTexture=e}get depthTexture(){return this._depthTexture}setSize(e,t,n=1){if(this.width!==e||this.height!==t||this.depth!==n){this.width=e,this.height=t,this.depth=n;for(let i=0,r=this.textures.length;i<r;i++)this.textures[i].image.width=e,this.textures[i].image.height=t,this.textures[i].image.depth=n,this.textures[i].isData3DTexture!==!0&&(this.textures[i].isArrayTexture=this.textures[i].image.depth>1);this.dispose()}this.viewport.set(0,0,e,t),this.scissor.set(0,0,e,t)}clone(){return new this.constructor().copy(this)}copy(e){this.width=e.width,this.height=e.height,this.depth=e.depth,this.scissor.copy(e.scissor),this.scissorTest=e.scissorTest,this.viewport.copy(e.viewport),this.textures.length=0;for(let t=0,n=e.textures.length;t<n;t++){this.textures[t]=e.textures[t].clone(),this.textures[t].isRenderTargetTexture=!0,this.textures[t].renderTarget=this;let i=Object.assign({},e.textures[t].image);this.textures[t].source=new yr(i)}if(this.depthBuffer=e.depthBuffer,this.stencilBuffer=e.stencilBuffer,this.resolveColorBuffer=e.resolveColorBuffer,this.resolveDepthBuffer=e.resolveDepthBuffer,this.resolveStencilBuffer=e.resolveStencilBuffer,this.storeMultisampledColorBuffer=e.storeMultisampledColorBuffer,this.storeMultisampledDepthBuffer=e.storeMultisampledDepthBuffer,this.storeMultisampledStencilBuffer=e.storeMultisampledStencilBuffer,e.depthTexture!==null)if(e.depthTexture.renderTarget===e){let t=e.depthTexture.clone();t.renderTarget=null,this.depthTexture=t}else this.depthTexture=e.depthTexture;return this.samples=e.samples,this.multiview=e.multiview,this.useArrayDepthTexture=e.useArrayDepthTexture,this}dispose(){this.dispatchEvent({type:"dispose"})}},an=class extends il{constructor(e=1,t=1,n={}){super(e,t,n),this.isWebGLRenderTarget=!0}},ba=class extends ln{constructor(e=null,t=1,n=1,i=1){super(null),this.isDataArrayTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=on,this.minFilter=on,this.wrapR=Qn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}copy(e){return super.copy(e),this.wrapR=e.wrapR,this}addLayerUpdate(e){this.layerUpdates.add(e)}clearLayerUpdates(){this.layerUpdates.clear()}};var sl=class extends ln{constructor(e=null,t=1,n=1,i=1){super(null),this.isData3DTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=on,this.minFilter=on,this.wrapR=Qn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}copy(e){return super.copy(e),this.wrapR=e.wrapR,this}};var ft=class s{static{s.prototype.isMatrix4=!0}constructor(e,t,n,i,r,a,o,l,c,h,u,d,p,x,v,m){this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,a,o,l,c,h,u,d,p,x,v,m)}set(e,t,n,i,r,a,o,l,c,h,u,d,p,x,v,m){let f=this.elements;return f[0]=e,f[4]=t,f[8]=n,f[12]=i,f[1]=r,f[5]=a,f[9]=o,f[13]=l,f[2]=c,f[6]=h,f[10]=u,f[14]=d,f[3]=p,f[7]=x,f[11]=v,f[15]=m,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new s().fromArray(this.elements)}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],t[9]=n[9],t[10]=n[10],t[11]=n[11],t[12]=n[12],t[13]=n[13],t[14]=n[14],t[15]=n[15],this}copyPosition(e){let t=this.elements,n=e.elements;return t[12]=n[12],t[13]=n[13],t[14]=n[14],this}setFromMatrix3(e){let t=e.elements;return this.set(t[0],t[3],t[6],0,t[1],t[4],t[7],0,t[2],t[5],t[8],0,0,0,0,1),this}extractBasis(e,t,n){return this.determinantAffine()===0?(e.set(1,0,0),t.set(0,1,0),n.set(0,0,1),this):(e.setFromMatrixColumn(this,0),t.setFromMatrixColumn(this,1),n.setFromMatrixColumn(this,2),this)}makeBasis(e,t,n){return this.set(e.x,t.x,n.x,0,e.y,t.y,n.y,0,e.z,t.z,n.z,0,0,0,0,1),this}extractRotation(e){if(e.determinantAffine()===0)return this.identity();let t=this.elements,n=e.elements,i=1/nr.setFromMatrixColumn(e,0).length(),r=1/nr.setFromMatrixColumn(e,1).length(),a=1/nr.setFromMatrixColumn(e,2).length();return t[0]=n[0]*i,t[1]=n[1]*i,t[2]=n[2]*i,t[3]=0,t[4]=n[4]*r,t[5]=n[5]*r,t[6]=n[6]*r,t[7]=0,t[8]=n[8]*a,t[9]=n[9]*a,t[10]=n[10]*a,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromEuler(e){let t=this.elements,n=e.x,i=e.y,r=e.z,a=Math.cos(n),o=Math.sin(n),l=Math.cos(i),c=Math.sin(i),h=Math.cos(r),u=Math.sin(r);if(e.order==="XYZ"){let d=a*h,p=a*u,x=o*h,v=o*u;t[0]=l*h,t[4]=-l*u,t[8]=c,t[1]=p+x*c,t[5]=d-v*c,t[9]=-o*l,t[2]=v-d*c,t[6]=x+p*c,t[10]=a*l}else if(e.order==="YXZ"){let d=l*h,p=l*u,x=c*h,v=c*u;t[0]=d+v*o,t[4]=x*o-p,t[8]=a*c,t[1]=a*u,t[5]=a*h,t[9]=-o,t[2]=p*o-x,t[6]=v+d*o,t[10]=a*l}else if(e.order==="ZXY"){let d=l*h,p=l*u,x=c*h,v=c*u;t[0]=d-v*o,t[4]=-a*u,t[8]=x+p*o,t[1]=p+x*o,t[5]=a*h,t[9]=v-d*o,t[2]=-a*c,t[6]=o,t[10]=a*l}else if(e.order==="ZYX"){let d=a*h,p=a*u,x=o*h,v=o*u;t[0]=l*h,t[4]=x*c-p,t[8]=d*c+v,t[1]=l*u,t[5]=v*c+d,t[9]=p*c-x,t[2]=-c,t[6]=o*l,t[10]=a*l}else if(e.order==="YZX"){let d=a*l,p=a*c,x=o*l,v=o*c;t[0]=l*h,t[4]=v-d*u,t[8]=x*u+p,t[1]=u,t[5]=a*h,t[9]=-o*h,t[2]=-c*h,t[6]=p*u+x,t[10]=d-v*u}else if(e.order==="XZY"){let d=a*l,p=a*c,x=o*l,v=o*c;t[0]=l*h,t[4]=-u,t[8]=c*h,t[1]=d*u+v,t[5]=a*h,t[9]=p*u-x,t[2]=x*u-p,t[6]=o*h,t[10]=v*u+d}return t[3]=0,t[7]=0,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromQuaternion(e){return this.compose(cg,e,hg)}lookAt(e,t,n){let i=this.elements;return Wn.subVectors(e,t),Wn.lengthSq()===0&&(Wn.z=1),Wn.normalize(),is.crossVectors(n,Wn),is.lengthSq()===0&&(Math.abs(n.z)===1?Wn.x+=1e-4:Wn.z+=1e-4,Wn.normalize(),is.crossVectors(n,Wn)),is.normalize(),yo.crossVectors(Wn,is),i[0]=is.x,i[4]=yo.x,i[8]=Wn.x,i[1]=is.y,i[5]=yo.y,i[9]=Wn.y,i[2]=is.z,i[6]=yo.z,i[10]=Wn.z,this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,a=n[0],o=n[4],l=n[8],c=n[12],h=n[1],u=n[5],d=n[9],p=n[13],x=n[2],v=n[6],m=n[10],f=n[14],y=n[3],T=n[7],g=n[11],_=n[15],w=i[0],A=i[4],b=i[8],C=i[12],S=i[1],P=i[5],M=i[9],F=i[13],I=i[2],N=i[6],H=i[10],D=i[14],V=i[3],k=i[7],E=i[11],Z=i[15];return r[0]=a*w+o*S+l*I+c*V,r[4]=a*A+o*P+l*N+c*k,r[8]=a*b+o*M+l*H+c*E,r[12]=a*C+o*F+l*D+c*Z,r[1]=h*w+u*S+d*I+p*V,r[5]=h*A+u*P+d*N+p*k,r[9]=h*b+u*M+d*H+p*E,r[13]=h*C+u*F+d*D+p*Z,r[2]=x*w+v*S+m*I+f*V,r[6]=x*A+v*P+m*N+f*k,r[10]=x*b+v*M+m*H+f*E,r[14]=x*C+v*F+m*D+f*Z,r[3]=y*w+T*S+g*I+_*V,r[7]=y*A+T*P+g*N+_*k,r[11]=y*b+T*M+g*H+_*E,r[15]=y*C+T*F+g*D+_*Z,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[4]*=e,t[8]*=e,t[12]*=e,t[1]*=e,t[5]*=e,t[9]*=e,t[13]*=e,t[2]*=e,t[6]*=e,t[10]*=e,t[14]*=e,t[3]*=e,t[7]*=e,t[11]*=e,t[15]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[12],a=e[1],o=e[5],l=e[9],c=e[13],h=e[2],u=e[6],d=e[10],p=e[14],x=e[3],v=e[7],m=e[11],f=e[15],y=l*p-c*d,T=o*p-c*u,g=o*d-l*u,_=a*p-c*h,w=a*d-l*h,A=a*u-o*h;return t*(v*y-m*T+f*g)-n*(x*y-m*_+f*w)+i*(x*T-v*_+f*A)-r*(x*g-v*w+m*A)}determinantAffine(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[1],a=e[5],o=e[9],l=e[2],c=e[6],h=e[10];return t*(a*h-o*c)-n*(r*h-o*l)+i*(r*c-a*l)}transpose(){let e=this.elements,t;return t=e[1],e[1]=e[4],e[4]=t,t=e[2],e[2]=e[8],e[8]=t,t=e[6],e[6]=e[9],e[9]=t,t=e[3],e[3]=e[12],e[12]=t,t=e[7],e[7]=e[13],e[13]=t,t=e[11],e[11]=e[14],e[14]=t,this}setPosition(e,t,n){let i=this.elements;return e.isVector3?(i[12]=e.x,i[13]=e.y,i[14]=e.z):(i[12]=e,i[13]=t,i[14]=n),this}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8],u=e[9],d=e[10],p=e[11],x=e[12],v=e[13],m=e[14],f=e[15],y=t*o-n*a,T=t*l-i*a,g=t*c-r*a,_=n*l-i*o,w=n*c-r*o,A=i*c-r*l,b=h*v-u*x,C=h*m-d*x,S=h*f-p*x,P=u*m-d*v,M=u*f-p*v,F=d*f-p*m,I=y*F-T*M+g*P+_*S-w*C+A*b;if(I===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let N=1/I;return e[0]=(o*F-l*M+c*P)*N,e[1]=(i*M-n*F-r*P)*N,e[2]=(v*A-m*w+f*_)*N,e[3]=(d*w-u*A-p*_)*N,e[4]=(l*S-a*F-c*C)*N,e[5]=(t*F-i*S+r*C)*N,e[6]=(m*g-x*A-f*T)*N,e[7]=(h*A-d*g+p*T)*N,e[8]=(a*M-o*S+c*b)*N,e[9]=(n*S-t*M-r*b)*N,e[10]=(x*w-v*g+f*y)*N,e[11]=(u*g-h*w-p*y)*N,e[12]=(o*C-a*P-l*b)*N,e[13]=(t*P-n*C+i*b)*N,e[14]=(v*T-x*_-m*y)*N,e[15]=(h*_-u*T+d*y)*N,this}scale(e){let t=this.elements,n=e.x,i=e.y,r=e.z;return t[0]*=n,t[4]*=i,t[8]*=r,t[1]*=n,t[5]*=i,t[9]*=r,t[2]*=n,t[6]*=i,t[10]*=r,t[3]*=n,t[7]*=i,t[11]*=r,this}getMaxScaleOnAxis(){let e=this.elements,t=e[0]*e[0]+e[1]*e[1]+e[2]*e[2],n=e[4]*e[4]+e[5]*e[5]+e[6]*e[6],i=e[8]*e[8]+e[9]*e[9]+e[10]*e[10];return Math.sqrt(Math.max(t,n,i))}makeTranslation(e,t,n){return e.isVector3?this.set(1,0,0,e.x,0,1,0,e.y,0,0,1,e.z,0,0,0,1):this.set(1,0,0,e,0,1,0,t,0,0,1,n,0,0,0,1),this}makeRotationX(e){let t=Math.cos(e),n=Math.sin(e);return this.set(1,0,0,0,0,t,-n,0,0,n,t,0,0,0,0,1),this}makeRotationY(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,0,n,0,0,1,0,0,-n,0,t,0,0,0,0,1),this}makeRotationZ(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,0,n,t,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(e,t){let n=Math.cos(t),i=Math.sin(t),r=1-n,a=e.x,o=e.y,l=e.z,c=r*a,h=r*o;return this.set(c*a+n,c*o-i*l,c*l+i*o,0,c*o+i*l,h*o+n,h*l-i*a,0,c*l-i*o,h*l+i*a,r*l*l+n,0,0,0,0,1),this}makeScale(e,t,n){return this.set(e,0,0,0,0,t,0,0,0,0,n,0,0,0,0,1),this}makeShear(e,t,n,i,r,a){return this.set(1,n,r,0,e,1,a,0,t,i,1,0,0,0,0,1),this}compose(e,t,n){let i=this.elements,r=t._x,a=t._y,o=t._z,l=t._w,c=r+r,h=a+a,u=o+o,d=r*c,p=r*h,x=r*u,v=a*h,m=a*u,f=o*u,y=l*c,T=l*h,g=l*u,_=n.x,w=n.y,A=n.z;return i[0]=(1-(v+f))*_,i[1]=(p+g)*_,i[2]=(x-T)*_,i[3]=0,i[4]=(p-g)*w,i[5]=(1-(d+f))*w,i[6]=(m+y)*w,i[7]=0,i[8]=(x+T)*A,i[9]=(m-y)*A,i[10]=(1-(d+v))*A,i[11]=0,i[12]=e.x,i[13]=e.y,i[14]=e.z,i[15]=1,this}decompose(e,t,n){let i=this.elements;e.x=i[12],e.y=i[13],e.z=i[14];let r=this.determinantAffine();if(r===0)return n.set(1,1,1),t.identity(),this;let a=nr.set(i[0],i[1],i[2]).length(),o=nr.set(i[4],i[5],i[6]).length(),l=nr.set(i[8],i[9],i[10]).length();r<0&&(a=-a),oi.copy(this);let c=1/a,h=1/o,u=1/l;return oi.elements[0]*=c,oi.elements[1]*=c,oi.elements[2]*=c,oi.elements[4]*=h,oi.elements[5]*=h,oi.elements[6]*=h,oi.elements[8]*=u,oi.elements[9]*=u,oi.elements[10]*=u,t.setFromRotationMatrix(oi),n.x=a,n.y=o,n.z=l,this}makePerspective(e,t,n,i,r,a,o=ui,l=!1){let c=this.elements,h=2*r/(t-e),u=2*r/(n-i),d=(t+e)/(t-e),p=(n+i)/(n-i),x,v;if(l)x=r/(a-r),v=a*r/(a-r);else if(o===ui)x=-(a+r)/(a-r),v=-2*a*r/(a-r);else if(o===br)x=-a/(a-r),v=-a*r/(a-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+o);return c[0]=h,c[4]=0,c[8]=d,c[12]=0,c[1]=0,c[5]=u,c[9]=p,c[13]=0,c[2]=0,c[6]=0,c[10]=x,c[14]=v,c[3]=0,c[7]=0,c[11]=-1,c[15]=0,this}makeOrthographic(e,t,n,i,r,a,o=ui,l=!1){let c=this.elements,h=2/(t-e),u=2/(n-i),d=-(t+e)/(t-e),p=-(n+i)/(n-i),x,v;if(l)x=1/(a-r),v=a/(a-r);else if(o===ui)x=-2/(a-r),v=-(a+r)/(a-r);else if(o===br)x=-1/(a-r),v=-r/(a-r);else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+o);return c[0]=h,c[4]=0,c[8]=0,c[12]=d,c[1]=0,c[5]=u,c[9]=0,c[13]=p,c[2]=0,c[6]=0,c[10]=x,c[14]=v,c[3]=0,c[7]=0,c[11]=0,c[15]=1,this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<16;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<16;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e[t+9]=n[9],e[t+10]=n[10],e[t+11]=n[11],e[t+12]=n[12],e[t+13]=n[13],e[t+14]=n[14],e[t+15]=n[15],e}},nr=new B,oi=new ft,cg=new B(0,0,0),hg=new B(1,1,1),is=new B,yo=new B,Wn=new B,bd=new ft,vd=new Qt,kn=class s{constructor(e=0,t=0,n=0,i=s.DEFAULT_ORDER){this.isEuler=!0,this._x=e,this._y=t,this._z=n,this._order=i}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get order(){return this._order}set order(e){this._order=e,this._onChangeCallback()}set(e,t,n,i=this._order){return this._x=e,this._y=t,this._z=n,this._order=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(e){return this._x=e._x,this._y=e._y,this._z=e._z,this._order=e._order,this._onChangeCallback(),this}setFromRotationMatrix(e,t=this._order,n=!0){let i=e.elements,r=i[0],a=i[4],o=i[8],l=i[1],c=i[5],h=i[9],u=i[2],d=i[6],p=i[10];switch(t){case"XYZ":this._y=Math.asin(At(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(-h,p),this._z=Math.atan2(-a,r)):(this._x=Math.atan2(d,c),this._z=0);break;case"YXZ":this._x=Math.asin(-At(h,-1,1)),Math.abs(h)<.9999999?(this._y=Math.atan2(o,p),this._z=Math.atan2(l,c)):(this._y=Math.atan2(-u,r),this._z=0);break;case"ZXY":this._x=Math.asin(At(d,-1,1)),Math.abs(d)<.9999999?(this._y=Math.atan2(-u,p),this._z=Math.atan2(-a,c)):(this._y=0,this._z=Math.atan2(l,r));break;case"ZYX":this._y=Math.asin(-At(u,-1,1)),Math.abs(u)<.9999999?(this._x=Math.atan2(d,p),this._z=Math.atan2(l,r)):(this._x=0,this._z=Math.atan2(-a,c));break;case"YZX":this._z=Math.asin(At(l,-1,1)),Math.abs(l)<.9999999?(this._x=Math.atan2(-h,c),this._y=Math.atan2(-u,r)):(this._x=0,this._y=Math.atan2(o,p));break;case"XZY":this._z=Math.asin(-At(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(d,c),this._y=Math.atan2(o,r)):(this._x=Math.atan2(-h,p),this._y=0);break;default:rt("Euler: .setFromRotationMatrix() encountered an unknown order: "+t)}return this._order=t,n===!0&&this._onChangeCallback(),this}setFromQuaternion(e,t,n){return bd.makeRotationFromQuaternion(e),this.setFromRotationMatrix(bd,t,n)}setFromVector3(e,t=this._order){return this.set(e.x,e.y,e.z,t)}reorder(e){return vd.setFromEuler(this),this.setFromQuaternion(vd,e)}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._order===this._order}fromArray(e){return this._x=e[0],this._y=e[1],this._z=e[2],e[3]!==void 0&&(this._order=e[3]),this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._order,e}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};kn.DEFAULT_ORDER="XYZ";var Mr=class{constructor(){this.mask=1}set(e){this.mask=(1<<e|0)>>>0}enable(e){this.mask|=1<<e|0}enableAll(){this.mask=-1}toggle(e){this.mask^=1<<e|0}disable(e){this.mask&=~(1<<e|0)}disableAll(){this.mask=0}test(e){return(this.mask&e.mask)!==0}isEnabled(e){return(this.mask&(1<<e|0))!==0}},ug=0,_d=new B,ir=new Qt,Bi=new ft,Mo=new B,ia=new B,dg=new B,fg=new Qt,yd=new B(1,0,0),Md=new B(0,1,0),Sd=new B(0,0,1),wd={type:"added"},pg={type:"removed"},sr={type:"childadded",child:null},Yc={type:"childremoved",child:null},Wt=class s extends ei{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:ug++}),this.uuid=di(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=s.DEFAULT_UP.clone();let e=new B,t=new kn,n=new Qt,i=new B(1,1,1);function r(){n.setFromEuler(t,!1)}function a(){t.setFromQuaternion(n,void 0,!1)}t._onChange(r),n._onChange(a),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:e},rotation:{configurable:!0,enumerable:!0,value:t},quaternion:{configurable:!0,enumerable:!0,value:n},scale:{configurable:!0,enumerable:!0,value:i},modelViewMatrix:{value:new ft},normalMatrix:{value:new xt}}),this.matrix=new ft,this.matrixWorld=new ft,this.matrixAutoUpdate=s.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=s.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new Mr,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.customDepthMaterial=void 0,this.customDistanceMaterial=void 0,this.static=!1,this.userData={},this.pivot=null}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(e){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(e),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(e){return this.quaternion.premultiply(e),this}setRotationFromAxisAngle(e,t){this.quaternion.setFromAxisAngle(e,t)}setRotationFromEuler(e){this.quaternion.setFromEuler(e,!0)}setRotationFromMatrix(e){this.quaternion.setFromRotationMatrix(e)}setRotationFromQuaternion(e){this.quaternion.copy(e)}rotateOnAxis(e,t){return ir.setFromAxisAngle(e,t),this.quaternion.multiply(ir),this}rotateOnWorldAxis(e,t){return ir.setFromAxisAngle(e,t),this.quaternion.premultiply(ir),this}rotateX(e){return this.rotateOnAxis(yd,e)}rotateY(e){return this.rotateOnAxis(Md,e)}rotateZ(e){return this.rotateOnAxis(Sd,e)}translateOnAxis(e,t){return _d.copy(e).applyQuaternion(this.quaternion),this.position.add(_d.multiplyScalar(t)),this}translateX(e){return this.translateOnAxis(yd,e)}translateY(e){return this.translateOnAxis(Md,e)}translateZ(e){return this.translateOnAxis(Sd,e)}localToWorld(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(this.matrixWorld)}worldToLocal(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(Bi.copy(this.matrixWorld).invert())}lookAt(e,t,n){e.isVector3?Mo.copy(e):Mo.set(e,t,n);let i=this.parent;this.updateWorldMatrix(!0,!1),ia.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?Bi.lookAt(ia,Mo,this.up):Bi.lookAt(Mo,ia,this.up),this.quaternion.setFromRotationMatrix(Bi),i&&(Bi.extractRotation(i.matrixWorld),ir.setFromRotationMatrix(Bi),this.quaternion.premultiply(ir.invert()))}add(e){if(arguments.length>1){for(let t=0;t<arguments.length;t++)this.add(arguments[t]);return this}return e===this?(pt("Object3D.add: object can't be added as a child of itself.",e),this):(e&&e.isObject3D?(e.removeFromParent(),e.parent=this,this.children.push(e),e.dispatchEvent(wd),sr.child=e,this.dispatchEvent(sr),sr.child=null):pt("Object3D.add: object not an instance of THREE.Object3D.",e),this)}remove(e){if(arguments.length>1){for(let n=0;n<arguments.length;n++)this.remove(arguments[n]);return this}let t=this.children.indexOf(e);return t!==-1&&(e.parent=null,this.children.splice(t,1),e.dispatchEvent(pg),Yc.child=e,this.dispatchEvent(Yc),Yc.child=null),this}removeFromParent(){let e=this.parent;return e!==null&&e.remove(this),this}clear(){return this.remove(...this.children)}attach(e){return this.updateWorldMatrix(!0,!1),Bi.copy(this.matrixWorld).invert(),e.parent!==null&&(e.parent.updateWorldMatrix(!0,!1),Bi.multiply(e.parent.matrixWorld)),e.applyMatrix4(Bi),e.removeFromParent(),e.parent=this,this.children.push(e),e.updateWorldMatrix(!1,!0),e.dispatchEvent(wd),sr.child=e,this.dispatchEvent(sr),sr.child=null,this}getObjectById(e){return this.getObjectByProperty("id",e)}getObjectByName(e){return this.getObjectByProperty("name",e)}getObjectByProperty(e,t){if(this[e]===t)return this;for(let n=0,i=this.children.length;n<i;n++){let a=this.children[n].getObjectByProperty(e,t);if(a!==void 0)return a}}getObjectsByProperty(e,t,n=[]){this[e]===t&&n.push(this);let i=this.children;for(let r=0,a=i.length;r<a;r++)i[r].getObjectsByProperty(e,t,n);return n}getWorldPosition(e){return this.updateWorldMatrix(!0,!1),e.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(ia,e,dg),e}getWorldScale(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(ia,fg,e),e}getWorldDirection(e){this.updateWorldMatrix(!0,!1);let t=this.matrixWorld.elements;return e.set(t[8],t[9],t[10]).normalize()}raycast(){}intersectsFrustum(){}traverse(e){e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverse(e)}traverseVisible(e){if(this.visible===!1)return;e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverseVisible(e)}traverseAncestors(e){let t=this.parent;t!==null&&(e(t),t.traverseAncestors(e))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale);let e=this.pivot;if(e!==null){let t=e.x,n=e.y,i=e.z,r=this.matrix.elements;r[12]+=t-r[0]*t-r[4]*n-r[8]*i,r[13]+=n-r[1]*t-r[5]*n-r[9]*i,r[14]+=i-r[2]*t-r[6]*n-r[10]*i}this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(e){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||e)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,e=!0);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].updateMatrixWorld(e)}updateWorldMatrix(e,t,n=!1){let i=this.parent;if(e===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||n)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,n=!0),t===!0){let r=this.children;for(let a=0,o=r.length;a<o;a++)r[a].updateWorldMatrix(!1,!0,n)}}toJSON(e){let t=e===void 0||typeof e=="string",n={};t&&(e={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},n.metadata={version:4.7,type:"Object",generator:"Object3D.toJSON"});let i={};i.uuid=this.uuid,i.type=this.type,i.name=this.name,i.castShadow=this.castShadow,i.receiveShadow=this.receiveShadow,i.visible=this.visible,i.frustumCulled=this.frustumCulled,i.renderOrder=this.renderOrder,i.static=this.static,i.matrixAutoUpdate=this.matrixAutoUpdate,Object.keys(this.userData).length>0&&(i.userData=this.userData),i.layers=this.layers.mask,i.matrix=this.matrix.toArray(),i.up=this.up.toArray(),this.pivot!==null&&(i.pivot=this.pivot.toArray()),this.morphTargetDictionary!==void 0&&(i.morphTargetDictionary=Object.assign({},this.morphTargetDictionary)),this.morphTargetInfluences!==void 0&&(i.morphTargetInfluences=this.morphTargetInfluences.slice()),this.isInstancedMesh&&(i.type="InstancedMesh",i.count=this.count,i.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(i.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(i.type="BatchedMesh",i.perObjectFrustumCulled=this.perObjectFrustumCulled,i.sortObjects=this.sortObjects,i.drawRanges=this._drawRanges,i.reservedRanges=this._reservedRanges,i.geometryInfo=this._geometryInfo.map(o=>({...o,boundingBox:o.boundingBox?o.boundingBox.toJSON():void 0,boundingSphere:o.boundingSphere?o.boundingSphere.toJSON():void 0})),i.instanceInfo=this._instanceInfo.map(o=>({...o})),i.availableInstanceIds=this._availableInstanceIds.slice(),i.availableGeometryIds=this._availableGeometryIds.slice(),i.nextIndexStart=this._nextIndexStart,i.nextVertexStart=this._nextVertexStart,i.geometryCount=this._geometryCount,i.maxInstanceCount=this._maxInstanceCount,i.maxVertexCount=this._maxVertexCount,i.maxIndexCount=this._maxIndexCount,i.geometryInitialized=this._geometryInitialized,i.matricesTexture=this._matricesTexture.toJSON(e),i.indirectTexture=this._indirectTexture.toJSON(e),this._colorsTexture!==null&&(i.colorsTexture=this._colorsTexture.toJSON(e)),this.boundingSphere!==null&&(i.boundingSphere=this.boundingSphere.toJSON()),this.boundingBox!==null&&(i.boundingBox=this.boundingBox.toJSON()));function r(o,l){return o[l.uuid]===void 0&&(o[l.uuid]=l.toJSON(e)),l.uuid}if(this.isScene)this.background&&(this.background.isColor?i.background=this.background.toJSON():this.background.isTexture&&(i.background=this.background.toJSON(e).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(i.environment=this.environment.toJSON(e).uuid);else if(this.isMesh||this.isLine||this.isPoints){i.geometry=r(e.geometries,this.geometry);let o=this.geometry.parameters;if(o!==void 0&&o.shapes!==void 0){let l=o.shapes;if(Array.isArray(l))for(let c=0,h=l.length;c<h;c++){let u=l[c];r(e.shapes,u)}else r(e.shapes,l)}}if(this.isSkinnedMesh&&(i.bindMode=this.bindMode,i.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(e.skeletons,this.skeleton),i.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let o=[];for(let l=0,c=this.material.length;l<c;l++)o.push(r(e.materials,this.material[l]));i.material=o}else i.material=r(e.materials,this.material);if(this.children.length>0){i.children=[];for(let o=0;o<this.children.length;o++)i.children.push(this.children[o].toJSON(e).object)}if(this.animations.length>0){i.animations=[];for(let o=0;o<this.animations.length;o++){let l=this.animations[o];i.animations.push(r(e.animations,l))}}if(t){let o=a(e.geometries),l=a(e.materials),c=a(e.textures),h=a(e.images),u=a(e.shapes),d=a(e.skeletons),p=a(e.animations),x=a(e.nodes);o.length>0&&(n.geometries=o),l.length>0&&(n.materials=l),c.length>0&&(n.textures=c),h.length>0&&(n.images=h),u.length>0&&(n.shapes=u),d.length>0&&(n.skeletons=d),p.length>0&&(n.animations=p),x.length>0&&(n.nodes=x)}return n.object=i,n;function a(o){let l=[];for(let c in o){let h=o[c];delete h.metadata,l.push(h)}return l}}clone(e){return new this.constructor().copy(this,e)}copy(e,t=!0){if(this.name=e.name,this.up.copy(e.up),this.position.copy(e.position),this.rotation.order=e.rotation.order,this.quaternion.copy(e.quaternion),this.scale.copy(e.scale),this.pivot=e.pivot!==null?e.pivot.clone():null,this.matrix.copy(e.matrix),this.matrixWorld.copy(e.matrixWorld),this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrixWorldAutoUpdate=e.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=e.matrixWorldNeedsUpdate,this.layers.mask=e.layers.mask,this.visible=e.visible,this.castShadow=e.castShadow,this.receiveShadow=e.receiveShadow,this.frustumCulled=e.frustumCulled,this.renderOrder=e.renderOrder,this.static=e.static,this.animations=e.animations.slice(),this.userData=JSON.parse(JSON.stringify(e.userData)),t===!0)for(let n=0;n<e.children.length;n++){let i=e.children[n];this.add(i.clone())}return this}dispose(){this.dispatchEvent({type:"dispose"})}};Wt.DEFAULT_UP=new B(0,1,0);Wt.DEFAULT_MATRIX_AUTO_UPDATE=!0;Wt.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var vt=class extends Wt{constructor(){super(),this.isGroup=!0,this.type="Group"}},mg={type:"move"},Sr=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new vt,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new vt,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new B,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new B),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new vt,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new B,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new B,this._grip.eventsEnabled=!1),this._grip}dispatchEvent(e){return this._targetRay!==null&&this._targetRay.dispatchEvent(e),this._grip!==null&&this._grip.dispatchEvent(e),this._hand!==null&&this._hand.dispatchEvent(e),this}connect(e){if(e&&e.hand){let t=this._hand;if(t)for(let n of e.hand.values())this._getHandJoint(t,n)}return this.dispatchEvent({type:"connected",data:e}),this}disconnect(e){return this.dispatchEvent({type:"disconnected",data:e}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(e,t,n){let i=null,r=null,a=null,o=this._targetRay,l=this._grip,c=this._hand;if(e&&t.session.visibilityState!=="visible-blurred"){if(c&&e.hand){a=!0;for(let v of e.hand.values()){let m=t.getJointPose(v,n),f=this._getHandJoint(c,v);m!==null&&(f.matrix.fromArray(m.transform.matrix),f.matrix.decompose(f.position,f.rotation,f.scale),f.matrixWorldNeedsUpdate=!0,f.jointRadius=m.radius),f.visible=m!==null}let h=c.joints["index-finger-tip"],u=c.joints["thumb-tip"],d=h.position.distanceTo(u.position),p=.02,x=.005;c.inputState.pinching&&d>p+x?(c.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:e.handedness,target:this})):!c.inputState.pinching&&d<=p-x&&(c.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:e.handedness,target:this}))}else l!==null&&e.gripSpace&&(r=t.getPose(e.gripSpace,n),r!==null&&(l.matrix.fromArray(r.transform.matrix),l.matrix.decompose(l.position,l.rotation,l.scale),l.matrixWorldNeedsUpdate=!0,r.linearVelocity?(l.hasLinearVelocity=!0,l.linearVelocity.copy(r.linearVelocity)):l.hasLinearVelocity=!1,r.angularVelocity?(l.hasAngularVelocity=!0,l.angularVelocity.copy(r.angularVelocity)):l.hasAngularVelocity=!1,l.eventsEnabled&&l.dispatchEvent({type:"gripUpdated",data:e,target:this})));o!==null&&(i=t.getPose(e.targetRaySpace,n),i===null&&r!==null&&(i=r),i!==null&&(o.matrix.fromArray(i.transform.matrix),o.matrix.decompose(o.position,o.rotation,o.scale),o.matrixWorldNeedsUpdate=!0,i.linearVelocity?(o.hasLinearVelocity=!0,o.linearVelocity.copy(i.linearVelocity)):o.hasLinearVelocity=!1,i.angularVelocity?(o.hasAngularVelocity=!0,o.angularVelocity.copy(i.angularVelocity)):o.hasAngularVelocity=!1,this.dispatchEvent(mg)))}return o!==null&&(o.visible=i!==null),l!==null&&(l.visible=r!==null),c!==null&&(c.visible=a!==null),this}_getHandJoint(e,t){if(e.joints[t.jointName]===void 0){let n=new vt;n.matrixAutoUpdate=!1,n.visible=!1,e.joints[t.jointName]=n,e.add(n)}return e.joints[t.jointName]}},Vf={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},ss={h:0,s:0,l:0},So={h:0,s:0,l:0};function Jc(s,e,t){return t<0&&(t+=1),t>1&&(t-=1),t<1/6?s+(e-s)*6*t:t<1/2?e:t<2/3?s+(e-s)*6*(2/3-t):s}var Le=class{constructor(e,t,n){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(e,t,n)}set(e,t,n){if(t===void 0&&n===void 0){let i=e;i&&i.isColor?this.copy(i):typeof i=="number"?this.setHex(i):typeof i=="string"&&this.setStyle(i)}else this.setRGB(e,t,n);return this}setScalar(e){return this.r=e,this.g=e,this.b=e,this}setHex(e,t=sn){return e=Math.floor(e),this.r=(e>>16&255)/255,this.g=(e>>8&255)/255,this.b=(e&255)/255,Tt.colorSpaceToWorking(this,t),this}setRGB(e,t,n,i=Tt.workingColorSpace){return this.r=e,this.g=t,this.b=n,Tt.colorSpaceToWorking(this,i),this}setHSL(e,t,n,i=Tt.workingColorSpace){if(e=Wh(e,1),t=At(t,0,1),n=At(n,0,1),t===0)this.r=this.g=this.b=n;else{let r=n<=.5?n*(1+t):n+t-n*t,a=2*n-r;this.r=Jc(a,r,e+1/3),this.g=Jc(a,r,e),this.b=Jc(a,r,e-1/3)}return Tt.colorSpaceToWorking(this,i),this}setStyle(e,t=sn){function n(r){r!==void 0&&parseFloat(r)<1&&rt("Color: Alpha component of "+e+" will be ignored.")}let i;if(i=/^(\w+)\(([^\)]*)\)/.exec(e)){let r,a=i[1],o=i[2];switch(a){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,t);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,t);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,t);break;default:rt("Color: Unknown color model "+e)}}else if(i=/^\#([A-Fa-f\d]+)$/.exec(e)){let r=i[1],a=r.length;if(a===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,t);if(a===6)return this.setHex(parseInt(r,16),t);rt("Color: Invalid hex color "+e)}else if(e&&e.length>0)return this.setColorName(e,t);return this}setColorName(e,t=sn){let n=Vf[e.toLowerCase()];return n!==void 0?this.setHex(n,t):rt("Color: Unknown color "+e),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(e){return this.r=e.r,this.g=e.g,this.b=e.b,this}copySRGBToLinear(e){return this.r=Xi(e.r),this.g=Xi(e.g),this.b=Xi(e.b),this}copyLinearToSRGB(e){return this.r=mr(e.r),this.g=mr(e.g),this.b=mr(e.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(e=sn){return Tt.workingToColorSpace(Rn.copy(this),e),Math.round(At(Rn.r*255,0,255))*65536+Math.round(At(Rn.g*255,0,255))*256+Math.round(At(Rn.b*255,0,255))}getHexString(e=sn){return("000000"+this.getHex(e).toString(16)).slice(-6)}getHSL(e,t=Tt.workingColorSpace){Tt.workingToColorSpace(Rn.copy(this),t);let n=Rn.r,i=Rn.g,r=Rn.b,a=Math.max(n,i,r),o=Math.min(n,i,r),l,c,h=(o+a)/2;if(o===a)l=0,c=0;else{let u=a-o;switch(c=h<=.5?u/(a+o):u/(2-a-o),a){case n:l=(i-r)/u+(i<r?6:0);break;case i:l=(r-n)/u+2;break;case r:l=(n-i)/u+4;break}l/=6}return e.h=l,e.s=c,e.l=h,e}getRGB(e,t=Tt.workingColorSpace){return Tt.workingToColorSpace(Rn.copy(this),t),e.r=Rn.r,e.g=Rn.g,e.b=Rn.b,e}getStyle(e=sn){Tt.workingToColorSpace(Rn.copy(this),e);let t=Rn.r,n=Rn.g,i=Rn.b;return e!==sn?`color(${e} ${t.toFixed(3)} ${n.toFixed(3)} ${i.toFixed(3)})`:`rgb(${Math.round(t*255)},${Math.round(n*255)},${Math.round(i*255)})`}offsetHSL(e,t,n){return this.getHSL(ss),this.setHSL(ss.h+e,ss.s+t,ss.l+n)}add(e){return this.r+=e.r,this.g+=e.g,this.b+=e.b,this}addColors(e,t){return this.r=e.r+t.r,this.g=e.g+t.g,this.b=e.b+t.b,this}addScalar(e){return this.r+=e,this.g+=e,this.b+=e,this}sub(e){return this.r=Math.max(0,this.r-e.r),this.g=Math.max(0,this.g-e.g),this.b=Math.max(0,this.b-e.b),this}multiply(e){return this.r*=e.r,this.g*=e.g,this.b*=e.b,this}multiplyScalar(e){return this.r*=e,this.g*=e,this.b*=e,this}lerp(e,t){return this.r+=(e.r-this.r)*t,this.g+=(e.g-this.g)*t,this.b+=(e.b-this.b)*t,this}lerpColors(e,t,n){return this.r=e.r+(t.r-e.r)*n,this.g=e.g+(t.g-e.g)*n,this.b=e.b+(t.b-e.b)*n,this}lerpHSL(e,t){this.getHSL(ss),e.getHSL(So);let n=da(ss.h,So.h,t),i=da(ss.s,So.s,t),r=da(ss.l,So.l,t);return this.setHSL(n,i,r),this}setFromVector3(e){return this.r=e.x,this.g=e.y,this.b=e.z,this}applyMatrix3(e){let t=this.r,n=this.g,i=this.b,r=e.elements;return this.r=r[0]*t+r[3]*n+r[6]*i,this.g=r[1]*t+r[4]*n+r[7]*i,this.b=r[2]*t+r[5]*n+r[8]*i,this}equals(e){return e.r===this.r&&e.g===this.g&&e.b===this.b}fromArray(e,t=0){return this.r=e[t],this.g=e[t+1],this.b=e[t+2],this}toArray(e=[],t=0){return e[t]=this.r,e[t+1]=this.g,e[t+2]=this.b,e}fromBufferAttribute(e,t){return this.r=e.getX(t),this.g=e.getY(t),this.b=e.getZ(t),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},Rn=new Le;Le.NAMES=Vf;var va=class s{constructor(e,t=25e-5){this.isFogExp2=!0,this.name="",this.color=new Le(e),this.density=t}clone(){return new s(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var Us=class extends Wt{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new kn,this.environmentIntensity=1,this.environmentRotation=new kn,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(e,t){return super.copy(e,t),e.background!==null&&(this.background=e.background.clone()),e.environment!==null&&(this.environment=e.environment.clone()),e.fog!==null&&(this.fog=e.fog.clone()),this.backgroundBlurriness=e.backgroundBlurriness,this.backgroundIntensity=e.backgroundIntensity,this.backgroundRotation.copy(e.backgroundRotation),this.environmentIntensity=e.environmentIntensity,this.environmentRotation.copy(e.environmentRotation),e.overrideMaterial!==null&&(this.overrideMaterial=e.overrideMaterial.clone()),this.matrixAutoUpdate=e.matrixAutoUpdate,this}toJSON(e){let t=super.toJSON(e);return this.fog!==null&&(t.object.fog=this.fog.toJSON()),t.object.backgroundBlurriness=this.backgroundBlurriness,t.object.backgroundIntensity=this.backgroundIntensity,t.object.backgroundRotation=this.backgroundRotation.toArray(),t.object.environmentIntensity=this.environmentIntensity,t.object.environmentRotation=this.environmentRotation.toArray(),t}},li=new B,Hi=new B,Zc=new B,Gi=new B,rr=new B,ar=new B,Ed=new B,$c=new B,Qc=new B,eh=new B,th=new Ot,nh=new Ot,ih=new Ot,cs=class s{constructor(e=new B,t=new B,n=new B){this.a=e,this.b=t,this.c=n}static getNormal(e,t,n,i){i.subVectors(n,t),li.subVectors(e,t),i.cross(li);let r=i.lengthSq();return r>0?i.multiplyScalar(1/Math.sqrt(r)):i.set(0,0,0)}static getBarycoord(e,t,n,i,r){li.subVectors(i,t),Hi.subVectors(n,t),Zc.subVectors(e,t);let a=li.dot(li),o=li.dot(Hi),l=li.dot(Zc),c=Hi.dot(Hi),h=Hi.dot(Zc),u=a*c-o*o;if(u===0)return r.set(0,0,0),null;let d=1/u,p=(c*l-o*h)*d,x=(a*h-o*l)*d;return r.set(1-p-x,x,p)}static containsPoint(e,t,n,i){return this.getBarycoord(e,t,n,i,Gi)===null?!1:Gi.x>=0&&Gi.y>=0&&Gi.x+Gi.y<=1}static getInterpolation(e,t,n,i,r,a,o,l){return this.getBarycoord(e,t,n,i,Gi)===null?(l.x=0,l.y=0,"z"in l&&(l.z=0),"w"in l&&(l.w=0),null):(l.setScalar(0),l.addScaledVector(r,Gi.x),l.addScaledVector(a,Gi.y),l.addScaledVector(o,Gi.z),l)}static getInterpolatedAttribute(e,t,n,i,r,a){return th.setScalar(0),nh.setScalar(0),ih.setScalar(0),th.fromBufferAttribute(e,t),nh.fromBufferAttribute(e,n),ih.fromBufferAttribute(e,i),a.setScalar(0),a.addScaledVector(th,r.x),a.addScaledVector(nh,r.y),a.addScaledVector(ih,r.z),a}static isFrontFacing(e,t,n,i){return li.subVectors(n,t),Hi.subVectors(e,t),li.cross(Hi).dot(i)<0}set(e,t,n){return this.a.copy(e),this.b.copy(t),this.c.copy(n),this}setFromPointsAndIndices(e,t,n,i){return this.a.copy(e[t]),this.b.copy(e[n]),this.c.copy(e[i]),this}setFromAttributeAndIndices(e,t,n,i){return this.a.fromBufferAttribute(e,t),this.b.fromBufferAttribute(e,n),this.c.fromBufferAttribute(e,i),this}clone(){return new this.constructor().copy(this)}copy(e){return this.a.copy(e.a),this.b.copy(e.b),this.c.copy(e.c),this}getArea(){return li.subVectors(this.c,this.b),Hi.subVectors(this.a,this.b),li.cross(Hi).length()*.5}getMidpoint(e){return e.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(e){return s.getNormal(this.a,this.b,this.c,e)}getPlane(e){return e.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(e,t){return s.getBarycoord(e,this.a,this.b,this.c,t)}getInterpolation(e,t,n,i,r){return s.getInterpolation(e,this.a,this.b,this.c,t,n,i,r)}containsPoint(e){return s.containsPoint(e,this.a,this.b,this.c)}isFrontFacing(e){return s.isFrontFacing(this.a,this.b,this.c,e)}intersectsBox(e){return e.intersectsTriangle(this)}closestPointToPoint(e,t){let n=this.a,i=this.b,r=this.c,a,o;rr.subVectors(i,n),ar.subVectors(r,n),$c.subVectors(e,n);let l=rr.dot($c),c=ar.dot($c);if(l<=0&&c<=0)return t.copy(n);Qc.subVectors(e,i);let h=rr.dot(Qc),u=ar.dot(Qc);if(h>=0&&u<=h)return t.copy(i);let d=l*u-h*c;if(d<=0&&l>=0&&h<=0)return a=l/(l-h),t.copy(n).addScaledVector(rr,a);eh.subVectors(e,r);let p=rr.dot(eh),x=ar.dot(eh);if(x>=0&&p<=x)return t.copy(r);let v=p*c-l*x;if(v<=0&&c>=0&&x<=0)return o=c/(c-x),t.copy(n).addScaledVector(ar,o);let m=h*x-p*u;if(m<=0&&u-h>=0&&p-x>=0)return Ed.subVectors(r,i),o=(u-h)/(u-h+(p-x)),t.copy(i).addScaledVector(Ed,o);let f=1/(m+v+d);return a=v*f,o=d*f,t.copy(n).addScaledVector(rr,a).addScaledVector(ar,o)}equals(e){return e.a.equals(this.a)&&e.b.equals(this.b)&&e.c.equals(this.c)}},cn=class{constructor(e=new B(1/0,1/0,1/0),t=new B(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=e,this.max=t}set(e,t){return this.min.copy(e),this.max.copy(t),this}setFromArray(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t+=3)this.expandByPoint(ci.fromArray(e,t));return this}setFromBufferAttribute(e){this.makeEmpty();for(let t=0,n=e.count;t<n;t++)this.expandByPoint(ci.fromBufferAttribute(e,t));return this}setFromPoints(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t++)this.expandByPoint(e[t]);return this}setFromCenterAndSize(e,t){let n=ci.copy(t).multiplyScalar(.5);return this.min.copy(e).sub(n),this.max.copy(e).add(n),this}setFromObject(e,t=!1){return this.makeEmpty(),this.expandByObject(e,t)}clone(){return new this.constructor().copy(this)}copy(e){return this.min.copy(e.min),this.max.copy(e.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(e){return this.isEmpty()?e.set(0,0,0):e.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(e){return this.isEmpty()?e.set(0,0,0):e.subVectors(this.max,this.min)}expandByPoint(e){return this.min.min(e),this.max.max(e),this}expandByVector(e){return this.min.sub(e),this.max.add(e),this}expandByScalar(e){return this.min.addScalar(-e),this.max.addScalar(e),this}expandByObject(e,t=!1){e.updateWorldMatrix(!1,!1);let n=e.geometry;if(n!==void 0){let r=n.getAttribute("position");if(t===!0&&r!==void 0&&e.isInstancedMesh!==!0)for(let a=0,o=r.count;a<o;a++)e.isMesh===!0?e.getVertexPosition(a,ci):ci.fromBufferAttribute(r,a),ci.applyMatrix4(e.matrixWorld),this.expandByPoint(ci);else e.boundingBox!==void 0?(e.boundingBox===null&&e.computeBoundingBox(),wo.copy(e.boundingBox)):(n.boundingBox===null&&n.computeBoundingBox(),wo.copy(n.boundingBox)),wo.applyMatrix4(e.matrixWorld),this.union(wo)}let i=e.children;for(let r=0,a=i.length;r<a;r++)this.expandByObject(i[r],t);return this}containsPoint(e){return e.x>=this.min.x&&e.x<=this.max.x&&e.y>=this.min.y&&e.y<=this.max.y&&e.z>=this.min.z&&e.z<=this.max.z}containsBox(e){return this.min.x<=e.min.x&&e.max.x<=this.max.x&&this.min.y<=e.min.y&&e.max.y<=this.max.y&&this.min.z<=e.min.z&&e.max.z<=this.max.z}getParameter(e,t){return t.set((e.x-this.min.x)/(this.max.x-this.min.x),(e.y-this.min.y)/(this.max.y-this.min.y),(e.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(e){return e.max.x>=this.min.x&&e.min.x<=this.max.x&&e.max.y>=this.min.y&&e.min.y<=this.max.y&&e.max.z>=this.min.z&&e.min.z<=this.max.z}intersectsSphere(e){return this.clampPoint(e.center,ci),ci.distanceToSquared(e.center)<=e.radius*e.radius}intersectsPlane(e){let t,n;return e.normal.x>0?(t=e.normal.x*this.min.x,n=e.normal.x*this.max.x):(t=e.normal.x*this.max.x,n=e.normal.x*this.min.x),e.normal.y>0?(t+=e.normal.y*this.min.y,n+=e.normal.y*this.max.y):(t+=e.normal.y*this.max.y,n+=e.normal.y*this.min.y),e.normal.z>0?(t+=e.normal.z*this.min.z,n+=e.normal.z*this.max.z):(t+=e.normal.z*this.max.z,n+=e.normal.z*this.min.z),t<=-e.constant&&n>=-e.constant}intersectsTriangle(e){if(this.isEmpty())return!1;this.getCenter(sa),Eo.subVectors(this.max,sa),or.subVectors(e.a,sa),lr.subVectors(e.b,sa),cr.subVectors(e.c,sa),rs.subVectors(lr,or),as.subVectors(cr,lr),Ts.subVectors(or,cr);let t=[0,-rs.z,rs.y,0,-as.z,as.y,0,-Ts.z,Ts.y,rs.z,0,-rs.x,as.z,0,-as.x,Ts.z,0,-Ts.x,-rs.y,rs.x,0,-as.y,as.x,0,-Ts.y,Ts.x,0];return!sh(t,or,lr,cr,Eo)||(t=[1,0,0,0,1,0,0,0,1],!sh(t,or,lr,cr,Eo))?!1:(To.crossVectors(rs,as),t=[To.x,To.y,To.z],sh(t,or,lr,cr,Eo))}clampPoint(e,t){return t.copy(e).clamp(this.min,this.max)}distanceToPoint(e){return this.clampPoint(e,ci).distanceTo(e)}getBoundingSphere(e){return this.isEmpty()?e.makeEmpty():(this.getCenter(e.center),e.radius=this.getSize(ci).length()*.5),e}intersect(e){return this.min.max(e.min),this.max.min(e.max),this.isEmpty()&&this.makeEmpty(),this}union(e){return this.min.min(e.min),this.max.max(e.max),this}applyMatrix4(e){return this.isEmpty()?this:(Vi[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(e),Vi[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(e),Vi[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(e),Vi[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(e),Vi[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(e),Vi[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(e),Vi[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(e),Vi[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(e),this.setFromPoints(Vi),this)}translate(e){return this.min.add(e),this.max.add(e),this}equals(e){return e.min.equals(this.min)&&e.max.equals(this.max)}toJSON(){return{min:this.min.toArray(),max:this.max.toArray()}}fromJSON(e){return this.min.fromArray(e.min),this.max.fromArray(e.max),this}},Vi=[new B,new B,new B,new B,new B,new B,new B,new B],ci=new B,wo=new cn,or=new B,lr=new B,cr=new B,rs=new B,as=new B,Ts=new B,sa=new B,Eo=new B,To=new B,As=new B;function sh(s,e,t,n,i){for(let r=0,a=s.length-3;r<=a;r+=3){As.fromArray(s,r);let o=i.x*Math.abs(As.x)+i.y*Math.abs(As.y)+i.z*Math.abs(As.z),l=e.dot(As),c=t.dot(As),h=n.dot(As);if(Math.max(-Math.max(l,c,h),Math.min(l,c,h))>o)return!1}return!0}var un=new B,Ao=new Be,gg=0,Rt=class extends ei{constructor(e,t,n=!1){if(super(),Array.isArray(e))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,Object.defineProperty(this,"id",{value:gg++}),this.name="",this.array=e,this.itemSize=t,this.count=e!==void 0?e.length/t:0,this.normalized=n,this.usage=Gh,this.updateRanges=[],this.gpuType=Yn,this.version=0}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.name=e.name,this.array=new e.array.constructor(e.array),this.itemSize=e.itemSize,this.count=e.count,this.normalized=e.normalized,this.usage=e.usage,this.gpuType=e.gpuType,this}copyAt(e,t,n){e*=this.itemSize,n*=t.itemSize;for(let i=0,r=this.itemSize;i<r;i++)this.array[e+i]=t.array[n+i];return this}copyArray(e){return this.array.set(e),this}applyMatrix3(e){if(this.itemSize===2)for(let t=0,n=this.count;t<n;t++)Ao.fromBufferAttribute(this,t),Ao.applyMatrix3(e),this.setXY(t,Ao.x,Ao.y);else if(this.itemSize===3)for(let t=0,n=this.count;t<n;t++)un.fromBufferAttribute(this,t),un.applyMatrix3(e),this.setXYZ(t,un.x,un.y,un.z);return this}applyMatrix4(e){for(let t=0,n=this.count;t<n;t++)un.fromBufferAttribute(this,t),un.applyMatrix4(e),this.setXYZ(t,un.x,un.y,un.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)un.fromBufferAttribute(this,t),un.applyNormalMatrix(e),this.setXYZ(t,un.x,un.y,un.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)un.fromBufferAttribute(this,t),un.transformDirection(e),this.setXYZ(t,un.x,un.y,un.z);return this}set(e,t=0){return this.array.set(e,t),this}getComponent(e,t){let n=this.array[e*this.itemSize+t];return this.normalized&&(n=hi(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=jt(n,this.array)),this.array[e*this.itemSize+t]=n,this}getX(e){let t=this.array[e*this.itemSize];return this.normalized&&(t=hi(t,this.array)),t}setX(e,t){return this.normalized&&(t=jt(t,this.array)),this.array[e*this.itemSize]=t,this}getY(e){let t=this.array[e*this.itemSize+1];return this.normalized&&(t=hi(t,this.array)),t}setY(e,t){return this.normalized&&(t=jt(t,this.array)),this.array[e*this.itemSize+1]=t,this}getZ(e){let t=this.array[e*this.itemSize+2];return this.normalized&&(t=hi(t,this.array)),t}setZ(e,t){return this.normalized&&(t=jt(t,this.array)),this.array[e*this.itemSize+2]=t,this}getW(e){let t=this.array[e*this.itemSize+3];return this.normalized&&(t=hi(t,this.array)),t}setW(e,t){return this.normalized&&(t=jt(t,this.array)),this.array[e*this.itemSize+3]=t,this}setXY(e,t,n){return e*=this.itemSize,this.normalized&&(t=jt(t,this.array),n=jt(n,this.array)),this.array[e+0]=t,this.array[e+1]=n,this}setXYZ(e,t,n,i){return e*=this.itemSize,this.normalized&&(t=jt(t,this.array),n=jt(n,this.array),i=jt(i,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e*=this.itemSize,this.normalized&&(t=jt(t,this.array),n=jt(n,this.array),i=jt(i,this.array),r=jt(r,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this.array[e+3]=r,this}onUpload(e){return this.onUploadCallback=e,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let e={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return e.name=this.name,e.usage=this.usage,e.gpuType=this.gpuType,e}dispose(){this.dispatchEvent({type:"dispose"})}};var _a=class extends Rt{constructor(e,t,n){super(new Uint16Array(e),t,n)}};var ya=class extends Rt{constructor(e,t,n){super(new Uint32Array(e),t,n)}};var mt=class extends Rt{constructor(e,t,n){super(new Float32Array(e),t,n)}},xg=new cn,ra=new B,rh=new B,Cn=class{constructor(e=new B,t=-1){this.isSphere=!0,this.center=e,this.radius=t}set(e,t){return this.center.copy(e),this.radius=t,this}setFromPoints(e,t){let n=this.center;t!==void 0?n.copy(t):xg.setFromPoints(e).getCenter(n);let i=0;for(let r=0,a=e.length;r<a;r++)i=Math.max(i,n.distanceToSquared(e[r]));return this.radius=Math.sqrt(i),this}copy(e){return this.center.copy(e.center),this.radius=e.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(e){return e.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(e){return e.distanceTo(this.center)-this.radius}intersectsSphere(e){let t=this.radius+e.radius;return e.center.distanceToSquared(this.center)<=t*t}intersectsBox(e){return e.intersectsSphere(this)}intersectsPlane(e){return Math.abs(e.distanceToPoint(this.center))<=this.radius}clampPoint(e,t){let n=this.center.distanceToSquared(e);return t.copy(e),n>this.radius*this.radius&&(t.sub(this.center).normalize(),t.multiplyScalar(this.radius).add(this.center)),t}getBoundingBox(e){return this.isEmpty()?(e.makeEmpty(),e):(e.set(this.center,this.center),e.expandByScalar(this.radius),e)}applyMatrix4(e){return this.center.applyMatrix4(e),this.radius=this.radius*e.getMaxScaleOnAxis(),this}translate(e){return this.center.add(e),this}expandByPoint(e){if(this.isEmpty())return this.center.copy(e),this.radius=0,this;ra.subVectors(e,this.center);let t=ra.lengthSq();if(t>this.radius*this.radius){let n=Math.sqrt(t),i=(n-this.radius)*.5;this.center.addScaledVector(ra,i/n),this.radius+=i}return this}union(e){return e.isEmpty()?this:this.isEmpty()?(this.copy(e),this):(this.center.equals(e.center)===!0?this.radius=Math.max(this.radius,e.radius):(rh.subVectors(e.center,this.center).setLength(e.radius),this.expandByPoint(ra.copy(e.center).add(rh)),this.expandByPoint(ra.copy(e.center).sub(rh))),this)}equals(e){return e.center.equals(this.center)&&e.radius===this.radius}clone(){return new this.constructor().copy(this)}toJSON(){return{radius:this.radius,center:this.center.toArray()}}fromJSON(e){return this.radius=e.radius,this.center.fromArray(e.center),this}},bg=0,$n=new ft,ah=new Wt,hr=new B,qn=new cn,aa=new cn,_n=new B,_t=class s extends ei{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:bg++}),this.uuid=di(),this.name="",this.type="BufferGeometry",this.index=null,this.indirect=null,this.indirectOffset=0,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={},this._transformed=!1}getIndex(){return this.index}setIndex(e){return Array.isArray(e)?this.index=new(Gm(e)?ya:_a)(e,1):this.index=e,this}setIndirect(e,t=0){return this.indirect=e,this.indirectOffset=t,this}getIndirect(){return this.indirect}getAttribute(e){return this.attributes[e]}setAttribute(e,t){return this.attributes[e]=t,this}deleteAttribute(e){return delete this.attributes[e],this}hasAttribute(e){return this.attributes[e]!==void 0}addGroup(e,t,n=0){this.groups.push({start:e,count:t,materialIndex:n})}clearGroups(){this.groups=[]}setDrawRange(e,t){this.drawRange.start=e,this.drawRange.count=t}applyMatrix4(e){let t=this.attributes.position;t!==void 0&&(t.applyMatrix4(e),t.needsUpdate=!0);let n=this.attributes.normal;if(n!==void 0){let r=new xt().getNormalMatrix(e);n.applyNormalMatrix(r),n.needsUpdate=!0}let i=this.attributes.tangent;return i!==void 0&&(i.transformDirection(e),i.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this._transformed=!0,this}applyQuaternion(e){return $n.makeRotationFromQuaternion(e),this.applyMatrix4($n),this}rotateX(e){return $n.makeRotationX(e),this.applyMatrix4($n),this}rotateY(e){return $n.makeRotationY(e),this.applyMatrix4($n),this}rotateZ(e){return $n.makeRotationZ(e),this.applyMatrix4($n),this}translate(e,t,n){return $n.makeTranslation(e,t,n),this.applyMatrix4($n),this}scale(e,t,n){return $n.makeScale(e,t,n),this.applyMatrix4($n),this}lookAt(e){return ah.lookAt(e),ah.updateMatrix(),this.applyMatrix4(ah.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(hr).negate(),this.translate(hr.x,hr.y,hr.z),this}setFromPoints(e){let t=this.getAttribute("position");if(t===void 0){let n=[];for(let i=0,r=e.length;i<r;i++){let a=e[i];n.push(a.x,a.y,a.z||0)}this.setAttribute("position",new mt(n,3))}else{let n=Math.min(e.length,t.count);for(let i=0;i<n;i++){let r=e[i];t.setXYZ(i,r.x,r.y,r.z||0)}e.length>t.count&&rt("BufferGeometry: Buffer size too small for points data. Use .dispose() and create a new geometry."),t.needsUpdate=!0}return this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new cn);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){pt("BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new B(-1/0,-1/0,-1/0),new B(1/0,1/0,1/0));return}if(e!==void 0){if(this.boundingBox.setFromBufferAttribute(e),t)for(let n=0,i=t.length;n<i;n++){let r=t[n];qn.setFromBufferAttribute(r),this.morphTargetsRelative?(_n.addVectors(this.boundingBox.min,qn.min),this.boundingBox.expandByPoint(_n),_n.addVectors(this.boundingBox.max,qn.max),this.boundingBox.expandByPoint(_n)):(this.boundingBox.expandByPoint(qn.min),this.boundingBox.expandByPoint(qn.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&pt('BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new Cn);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){pt("BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new B,1/0);return}if(e){let n=this.boundingSphere.center;if(qn.setFromBufferAttribute(e),t)for(let r=0,a=t.length;r<a;r++){let o=t[r];aa.setFromBufferAttribute(o),this.morphTargetsRelative?(_n.addVectors(qn.min,aa.min),qn.expandByPoint(_n),_n.addVectors(qn.max,aa.max),qn.expandByPoint(_n)):(qn.expandByPoint(aa.min),qn.expandByPoint(aa.max))}qn.getCenter(n);let i=0;for(let r=0,a=e.count;r<a;r++)_n.fromBufferAttribute(e,r),i=Math.max(i,n.distanceToSquared(_n));if(t)for(let r=0,a=t.length;r<a;r++){let o=t[r],l=this.morphTargetsRelative;for(let c=0,h=o.count;c<h;c++)_n.fromBufferAttribute(o,c),l&&(hr.fromBufferAttribute(e,c),_n.add(hr)),i=Math.max(i,n.distanceToSquared(_n))}this.boundingSphere.radius=Math.sqrt(i),isNaN(this.boundingSphere.radius)&&pt('BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let e=this.index,t=this.attributes;if(e===null||t.position===void 0||t.normal===void 0||t.uv===void 0){pt("BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let n=t.position,i=t.normal,r=t.uv,a=this.getAttribute("tangent");(a===void 0||a.count!==n.count)&&(a=new Rt(new Float32Array(4*n.count),4),this.setAttribute("tangent",a));let o=[],l=[];for(let b=0;b<n.count;b++)o[b]=new B,l[b]=new B;let c=new B,h=new B,u=new B,d=new Be,p=new Be,x=new Be,v=new B,m=new B;function f(b,C,S){c.fromBufferAttribute(n,b),h.fromBufferAttribute(n,C),u.fromBufferAttribute(n,S),d.fromBufferAttribute(r,b),p.fromBufferAttribute(r,C),x.fromBufferAttribute(r,S),h.sub(c),u.sub(c),p.sub(d),x.sub(d);let P=1/(p.x*x.y-x.x*p.y);isFinite(P)&&(v.copy(h).multiplyScalar(x.y).addScaledVector(u,-p.y).multiplyScalar(P),m.copy(u).multiplyScalar(p.x).addScaledVector(h,-x.x).multiplyScalar(P),o[b].add(v),o[C].add(v),o[S].add(v),l[b].add(m),l[C].add(m),l[S].add(m))}let y=this.groups;y.length===0&&(y=[{start:0,count:e.count}]);for(let b=0,C=y.length;b<C;++b){let S=y[b],P=S.start,M=S.count;for(let F=P,I=P+M;F<I;F+=3)f(e.getX(F+0),e.getX(F+1),e.getX(F+2))}let T=new B,g=new B,_=new B,w=new B;function A(b){_.fromBufferAttribute(i,b),w.copy(_);let C=o[b];T.copy(C),T.sub(_.multiplyScalar(_.dot(C))).normalize(),g.crossVectors(w,C);let P=g.dot(l[b])<0?-1:1;a.setXYZW(b,T.x,T.y,T.z,P)}for(let b=0,C=y.length;b<C;++b){let S=y[b],P=S.start,M=S.count;for(let F=P,I=P+M;F<I;F+=3)A(e.getX(F+0)),A(e.getX(F+1)),A(e.getX(F+2))}this._transformed=!0}computeVertexNormals(){let e=this.index,t=this.getAttribute("position");if(t!==void 0){let n=this.getAttribute("normal");if(n===void 0||n.count!==t.count)n=new Rt(new Float32Array(t.count*3),3),this.setAttribute("normal",n);else for(let d=0,p=n.count;d<p;d++)n.setXYZ(d,0,0,0);let i=new B,r=new B,a=new B,o=new B,l=new B,c=new B,h=new B,u=new B;if(e)for(let d=0,p=e.count;d<p;d+=3){let x=e.getX(d+0),v=e.getX(d+1),m=e.getX(d+2);i.fromBufferAttribute(t,x),r.fromBufferAttribute(t,v),a.fromBufferAttribute(t,m),h.subVectors(a,r),u.subVectors(i,r),h.cross(u),o.fromBufferAttribute(n,x),l.fromBufferAttribute(n,v),c.fromBufferAttribute(n,m),o.add(h),l.add(h),c.add(h),n.setXYZ(x,o.x,o.y,o.z),n.setXYZ(v,l.x,l.y,l.z),n.setXYZ(m,c.x,c.y,c.z)}else for(let d=0,p=t.count;d<p;d+=3)i.fromBufferAttribute(t,d+0),r.fromBufferAttribute(t,d+1),a.fromBufferAttribute(t,d+2),h.subVectors(a,r),u.subVectors(i,r),h.cross(u),n.setXYZ(d+0,h.x,h.y,h.z),n.setXYZ(d+1,h.x,h.y,h.z),n.setXYZ(d+2,h.x,h.y,h.z);this.normalizeNormals(),n.needsUpdate=!0}}normalizeNormals(){let e=this.attributes.normal;for(let t=0,n=e.count;t<n;t++)_n.fromBufferAttribute(e,t),_n.normalize(),e.setXYZ(t,_n.x,_n.y,_n.z)}toNonIndexed(){function e(o,l){let c=o.array,h=o.itemSize,u=o.normalized,d=new c.constructor(l.length*h),p=0,x=0;for(let v=0,m=l.length;v<m;v++){o.isInterleavedBufferAttribute?p=l[v]*o.data.stride+o.offset:p=l[v]*h;for(let f=0;f<h;f++)d[x++]=c[p++]}return new Rt(d,h,u)}if(this.index===null)return rt("BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let t=new s,n=this.index.array,i=this.attributes;for(let o in i){let l=i[o],c=e(l,n);t.setAttribute(o,c)}let r=this.morphAttributes;for(let o in r){let l=[],c=r[o];for(let h=0,u=c.length;h<u;h++){let d=c[h],p=e(d,n);l.push(p)}t.morphAttributes[o]=l}t.morphTargetsRelative=this.morphTargetsRelative;let a=this.groups;for(let o=0,l=a.length;o<l;o++){let c=a[o];t.addGroup(c.start,c.count,c.materialIndex)}return t}toJSON(){let e={metadata:{version:4.7,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(e.uuid=this.uuid,e.type=this.parameters!==void 0&&this._transformed===!0?"BufferGeometry":this.type,e.name=this.name,Object.keys(this.userData).length>0&&(e.userData=this.userData),this.parameters!==void 0&&this._transformed!==!0){let l=this.parameters;for(let c in l)l[c]!==void 0&&(e[c]=l[c]);return e}e.data={attributes:{}};let t=this.index;t!==null&&(e.data.index={type:t.array.constructor.name,array:Array.prototype.slice.call(t.array)});let n=this.attributes;for(let l in n){let c=n[l];e.data.attributes[l]=c.toJSON(e.data)}let i={},r=!1;for(let l in this.morphAttributes){let c=this.morphAttributes[l],h=[];for(let u=0,d=c.length;u<d;u++){let p=c[u];h.push(p.toJSON(e.data))}h.length>0&&(i[l]=h,r=!0)}r&&(e.data.morphAttributes=i,e.data.morphTargetsRelative=this.morphTargetsRelative);let a=this.groups;a.length>0&&(e.data.groups=JSON.parse(JSON.stringify(a)));let o=this.boundingSphere;return o!==null&&(e.data.boundingSphere=o.toJSON()),e}clone(){return new this.constructor().copy(this)}copy(e){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let t={};this.name=e.name;let n=e.index;n!==null&&this.setIndex(n.clone());let i=e.attributes;for(let c in i){let h=i[c];this.setAttribute(c,h.clone(t))}let r=e.morphAttributes;for(let c in r){let h=[],u=r[c];for(let d=0,p=u.length;d<p;d++)h.push(u[d].clone(t));this.morphAttributes[c]=h}this.morphTargetsRelative=e.morphTargetsRelative;let a=e.groups;for(let c=0,h=a.length;c<h;c++){let u=a[c];this.addGroup(u.start,u.count,u.materialIndex)}let o=e.boundingBox;o!==null&&(this.boundingBox=o.clone());let l=e.boundingSphere;return l!==null&&(this.boundingSphere=l.clone()),this.drawRange.start=e.drawRange.start,this.drawRange.count=e.drawRange.count,this.userData=e.userData,this._transformed=e._transformed,this}dispose(){this.dispatchEvent({type:"dispose"})}},wr=class{constructor(e,t){this.isInterleavedBuffer=!0,this.array=e,this.stride=t,this.count=e!==void 0?e.length/t:0,this.usage=Gh,this.updateRanges=[],this.version=0,this.uuid=di()}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.array=new e.array.constructor(e.array),this.count=e.count,this.stride=e.stride,this.usage=e.usage,this}copyAt(e,t,n){e*=this.stride,n*=t.stride;for(let i=0,r=this.stride;i<r;i++)this.array[e+i]=t.array[n+i];return this}set(e,t=0){return this.array.set(e,t),this}clone(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=di()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=this.array.slice(0).buffer);let t=new this.array.constructor(e.arrayBuffers[this.array.buffer._uuid]),n=new this.constructor(t,this.stride);return n.setUsage(this.usage),n}onUpload(e){return this.onUploadCallback=e,this}toJSON(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=di()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=Array.from(new Uint32Array(this.array.buffer)));let t={uuid:this.uuid,buffer:this.array.buffer._uuid,type:this.array.constructor.name,stride:this.stride};return t.usage=this.usage,t}},Nn=new B,Er=class s{constructor(e,t,n,i=!1){this.isInterleavedBufferAttribute=!0,this.name="",this.data=e,this.itemSize=t,this.offset=n,this.normalized=i}get count(){return this.data.count}get array(){return this.data.array}set needsUpdate(e){this.data.needsUpdate=e}applyMatrix4(e){for(let t=0,n=this.data.count;t<n;t++)Nn.fromBufferAttribute(this,t),Nn.applyMatrix4(e),this.setXYZ(t,Nn.x,Nn.y,Nn.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)Nn.fromBufferAttribute(this,t),Nn.applyNormalMatrix(e),this.setXYZ(t,Nn.x,Nn.y,Nn.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)Nn.fromBufferAttribute(this,t),Nn.transformDirection(e),this.setXYZ(t,Nn.x,Nn.y,Nn.z);return this}getComponent(e,t){let n=this.array[e*this.data.stride+this.offset+t];return this.normalized&&(n=hi(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=jt(n,this.array)),this.data.array[e*this.data.stride+this.offset+t]=n,this}setX(e,t){return this.normalized&&(t=jt(t,this.array)),this.data.array[e*this.data.stride+this.offset]=t,this}setY(e,t){return this.normalized&&(t=jt(t,this.array)),this.data.array[e*this.data.stride+this.offset+1]=t,this}setZ(e,t){return this.normalized&&(t=jt(t,this.array)),this.data.array[e*this.data.stride+this.offset+2]=t,this}setW(e,t){return this.normalized&&(t=jt(t,this.array)),this.data.array[e*this.data.stride+this.offset+3]=t,this}getX(e){let t=this.data.array[e*this.data.stride+this.offset];return this.normalized&&(t=hi(t,this.array)),t}getY(e){let t=this.data.array[e*this.data.stride+this.offset+1];return this.normalized&&(t=hi(t,this.array)),t}getZ(e){let t=this.data.array[e*this.data.stride+this.offset+2];return this.normalized&&(t=hi(t,this.array)),t}getW(e){let t=this.data.array[e*this.data.stride+this.offset+3];return this.normalized&&(t=hi(t,this.array)),t}setXY(e,t,n){return e=e*this.data.stride+this.offset,this.normalized&&(t=jt(t,this.array),n=jt(n,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this}setXYZ(e,t,n,i){return e=e*this.data.stride+this.offset,this.normalized&&(t=jt(t,this.array),n=jt(n,this.array),i=jt(i,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e=e*this.data.stride+this.offset,this.normalized&&(t=jt(t,this.array),n=jt(n,this.array),i=jt(i,this.array),r=jt(r,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this.data.array[e+3]=r,this}clone(e){if(e===void 0){xa("InterleavedBufferAttribute.clone(): Cloning an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return new Rt(new this.array.constructor(t),this.itemSize,this.normalized)}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.clone(e)),new s(e.interleavedBuffers[this.data.uuid],this.itemSize,this.offset,this.normalized)}toJSON(e){if(e===void 0){xa("InterleavedBufferAttribute.toJSON(): Serializing an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return{itemSize:this.itemSize,type:this.array.constructor.name,array:t,normalized:this.normalized}}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.toJSON(e)),{isInterleavedBufferAttribute:!0,itemSize:this.itemSize,data:this.data.uuid,offset:this.offset,normalized:this.normalized}}},oh=new B,vg=new B,_g=new xt,Xn=class{constructor(e=new B(1,0,0),t=0){this.isPlane=!0,this.normal=e,this.constant=t}set(e,t){return this.normal.copy(e),this.constant=t,this}setComponents(e,t,n,i){return this.normal.set(e,t,n),this.constant=i,this}setFromNormalAndCoplanarPoint(e,t){return this.normal.copy(e),this.constant=-t.dot(this.normal),this}setFromCoplanarPoints(e,t,n){let i=oh.subVectors(n,t).cross(vg.subVectors(e,t)).normalize();return this.setFromNormalAndCoplanarPoint(i,e),this}copy(e){return this.normal.copy(e.normal),this.constant=e.constant,this}normalize(){let e=1/this.normal.length();return this.normal.multiplyScalar(e),this.constant*=e,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(e){return this.normal.dot(e)+this.constant}distanceToSphere(e){return this.distanceToPoint(e.center)-e.radius}projectPoint(e,t){return t.copy(e).addScaledVector(this.normal,-this.distanceToPoint(e))}intersectLine(e,t,n=!0){let i=e.delta(oh),r=this.normal.dot(i);if(r===0)return this.distanceToPoint(e.start)===0?t.copy(e.start):null;let a=-(e.start.dot(this.normal)+this.constant)/r;return n===!0&&(a<0||a>1)?null:t.copy(e.start).addScaledVector(i,a)}intersectsLine(e){let t=this.distanceToPoint(e.start),n=this.distanceToPoint(e.end);return t<0&&n>0||n<0&&t>0}intersectsBox(e){return e.intersectsPlane(this)}intersectsSphere(e){return e.intersectsPlane(this)}coplanarPoint(e){return e.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(e,t){let n=t||_g.getNormalMatrix(e),i=this.coplanarPoint(oh).applyMatrix4(e),r=this.normal.applyMatrix3(n).normalize();return this.constant=-i.dot(r),this}translate(e){return this.constant-=e.dot(this.normal),this}equals(e){return e.normal.equals(this.normal)&&e.constant===this.constant}clone(){return new this.constructor().copy(this)}toJSON(){return{normal:this.normal.toArray(),constant:this.constant}}fromJSON(e){return this.normal.fromArray(e.normal),this.constant=e.constant,this}},yg=0,On=class extends ei{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:yg++}),this.uuid=di(),this.name="",this.type="Material",this.blending=zr,this.side=ti,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=Dh,this.blendDst=Lh,this.blendEquation=Vs,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new Le(0,0,0),this.blendAlpha=0,this.depthFunc=gr,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=Df,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=jo,this.stencilZFail=jo,this.stencilZPass=jo,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.allowOverride=!0,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(e){this._alphaTest>0!=e>0&&this.version++,this._alphaTest=e}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(e){if(e!==void 0)for(let t in e){let n=e[t];if(n===void 0){rt(`Material: parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){rt(`Material: '${t}' is not a property of THREE.${this.type}.`);continue}i&&i.isColor?i.set(n):i&&i.isVector2&&n&&n.isVector2||i&&i.isEuler&&n&&n.isEuler||i&&i.isVector3&&n&&n.isVector3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";t&&(e={textures:{},images:{}});let n={metadata:{version:4.7,type:"Material",generator:"Material.toJSON"}};n.uuid=this.uuid,n.type=this.type,n.blending=this.blending,n.side=this.side,n.shadowSide=this.shadowSide,n.vertexColors=this.vertexColors,n.opacity=this.opacity,n.transparent=this.transparent,n.blendSrc=this.blendSrc,n.blendDst=this.blendDst,n.blendEquation=this.blendEquation,n.blendSrcAlpha=this.blendSrcAlpha,n.blendDstAlpha=this.blendDstAlpha,n.blendEquationAlpha=this.blendEquationAlpha,n.blendColor=this.blendColor.getHex(),n.blendAlpha=this.blendAlpha,n.depthFunc=this.depthFunc,n.depthTest=this.depthTest,n.depthWrite=this.depthWrite,n.colorWrite=this.colorWrite,n.clipIntersection=this.clipIntersection,n.clipShadows=this.clipShadows,n.stencilWriteMask=this.stencilWriteMask,n.stencilFunc=this.stencilFunc,n.stencilRef=this.stencilRef,n.stencilFuncMask=this.stencilFuncMask,n.stencilFail=this.stencilFail,n.stencilZFail=this.stencilZFail,n.stencilZPass=this.stencilZPass,n.stencilWrite=this.stencilWrite,n.polygonOffset=this.polygonOffset,n.polygonOffsetFactor=this.polygonOffsetFactor,n.polygonOffsetUnits=this.polygonOffsetUnits,n.dithering=this.dithering,n.alphaTest=this.alphaTest,n.alphaHash=this.alphaHash,n.alphaToCoverage=this.alphaToCoverage,n.premultipliedAlpha=this.premultipliedAlpha,n.forceSinglePass=this.forceSinglePass,n.allowOverride=this.allowOverride,n.visible=this.visible,n.toneMapped=this.toneMapped,n.name=this.name,this.color&&this.color.isColor&&(n.color=this.color.getHex()),this.roughness!==void 0&&(n.roughness=this.roughness),this.metalness!==void 0&&(n.metalness=this.metalness),this.sheen!==void 0&&(n.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(n.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(n.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(n.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&(n.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(n.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(n.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(n.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(n.shininess=this.shininess),this.clearcoat!==void 0&&(n.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(n.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(n.clearcoatMap=this.clearcoatMap.toJSON(e).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(n.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(e).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(n.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(e).uuid,n.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.sheenColorMap&&this.sheenColorMap.isTexture&&(n.sheenColorMap=this.sheenColorMap.toJSON(e).uuid),this.sheenRoughnessMap&&this.sheenRoughnessMap.isTexture&&(n.sheenRoughnessMap=this.sheenRoughnessMap.toJSON(e).uuid),this.dispersion!==void 0&&(n.dispersion=this.dispersion),this.retroreflectivity!==void 0&&(n.retroreflectivity=this.retroreflectivity),this.iridescence!==void 0&&(n.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(n.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(n.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(n.iridescenceMap=this.iridescenceMap.toJSON(e).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(n.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(e).uuid),this.anisotropy!==void 0&&(n.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(n.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(n.anisotropyMap=this.anisotropyMap.toJSON(e).uuid),this.map&&this.map.isTexture&&(n.map=this.map.toJSON(e).uuid),this.matcap&&this.matcap.isTexture&&(n.matcap=this.matcap.toJSON(e).uuid),this.alphaMap&&this.alphaMap.isTexture&&(n.alphaMap=this.alphaMap.toJSON(e).uuid),this.lightMap&&this.lightMap.isTexture&&(n.lightMap=this.lightMap.toJSON(e).uuid,n.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(n.aoMap=this.aoMap.toJSON(e).uuid,n.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(n.bumpMap=this.bumpMap.toJSON(e).uuid,n.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(n.normalMap=this.normalMap.toJSON(e).uuid,n.normalMapType=this.normalMapType,n.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(n.displacementMap=this.displacementMap.toJSON(e).uuid,n.displacementScale=this.displacementScale,n.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(n.roughnessMap=this.roughnessMap.toJSON(e).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(n.metalnessMap=this.metalnessMap.toJSON(e).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(n.emissiveMap=this.emissiveMap.toJSON(e).uuid),this.specularMap&&this.specularMap.isTexture&&(n.specularMap=this.specularMap.toJSON(e).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(n.specularIntensityMap=this.specularIntensityMap.toJSON(e).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(n.specularColorMap=this.specularColorMap.toJSON(e).uuid),this.envMap&&this.envMap.isTexture&&(n.envMap=this.envMap.toJSON(e).uuid,this.combine!==void 0&&(n.combine=this.combine)),this.envMapRotation!==void 0&&(n.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(n.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(n.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(n.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(n.gradientMap=this.gradientMap.toJSON(e).uuid),this.transmission!==void 0&&(n.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(n.transmissionMap=this.transmissionMap.toJSON(e).uuid),this.thickness!==void 0&&(n.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(n.thicknessMap=this.thicknessMap.toJSON(e).uuid),this.attenuationDistance!==void 0&&(n.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(n.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(n.size=this.size),this.sizeAttenuation!==void 0&&(n.sizeAttenuation=this.sizeAttenuation),Array.isArray(this.clippingPlanes)&&this.clippingPlanes.length>0&&(n.clippingPlanes=this.clippingPlanes.map(r=>r.toJSON())),this.rotation!==void 0&&(n.rotation=this.rotation),this.depthPacking!==void 0&&(n.depthPacking=this.depthPacking),this.linewidth!==void 0&&(n.linewidth=this.linewidth),this.linecap!==void 0&&(n.linecap=this.linecap),this.linejoin!==void 0&&(n.linejoin=this.linejoin),this.dashSize!==void 0&&(n.dashSize=this.dashSize),this.gapSize!==void 0&&(n.gapSize=this.gapSize),this.scale!==void 0&&(n.scale=this.scale),this.wireframe!==void 0&&(n.wireframe=this.wireframe),this.wireframeLinewidth!==void 0&&(n.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!==void 0&&(n.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!==void 0&&(n.wireframeLinejoin=this.wireframeLinejoin),this.flatShading!==void 0&&(n.flatShading=this.flatShading),this.fog!==void 0&&(n.fog=this.fog),Object.keys(this.userData).length>0&&(n.userData=this.userData);function i(r){let a=[];for(let o in r){let l=r[o];delete l.metadata,a.push(l)}return a}if(t){let r=i(e.textures),a=i(e.images);r.length>0&&(n.textures=r),a.length>0&&(n.images=a)}return n}fromJSON(e,t){if(e.uuid!==void 0&&(this.uuid=e.uuid),e.name!==void 0&&(this.name=e.name),e.color!==void 0&&this.color!==void 0&&this.color.setHex(e.color),e.roughness!==void 0&&(this.roughness=e.roughness),e.metalness!==void 0&&(this.metalness=e.metalness),e.sheen!==void 0&&(this.sheen=e.sheen),e.sheenColor!==void 0&&(this.sheenColor=new Le().setHex(e.sheenColor)),e.sheenRoughness!==void 0&&(this.sheenRoughness=e.sheenRoughness),e.emissive!==void 0&&this.emissive!==void 0&&this.emissive.setHex(e.emissive),e.specular!==void 0&&this.specular!==void 0&&this.specular.setHex(e.specular),e.specularIntensity!==void 0&&(this.specularIntensity=e.specularIntensity),e.specularColor!==void 0&&this.specularColor!==void 0&&this.specularColor.setHex(e.specularColor),e.shininess!==void 0&&(this.shininess=e.shininess),e.clearcoat!==void 0&&(this.clearcoat=e.clearcoat),e.clearcoatRoughness!==void 0&&(this.clearcoatRoughness=e.clearcoatRoughness),e.dispersion!==void 0&&(this.dispersion=e.dispersion),e.retroreflectivity!==void 0&&(this.retroreflectivity=e.retroreflectivity),e.iridescence!==void 0&&(this.iridescence=e.iridescence),e.iridescenceIOR!==void 0&&(this.iridescenceIOR=e.iridescenceIOR),e.iridescenceThicknessRange!==void 0&&(this.iridescenceThicknessRange=e.iridescenceThicknessRange),e.transmission!==void 0&&(this.transmission=e.transmission),e.thickness!==void 0&&(this.thickness=e.thickness),e.attenuationDistance!==void 0&&(this.attenuationDistance=e.attenuationDistance),e.attenuationColor!==void 0&&this.attenuationColor!==void 0&&this.attenuationColor.setHex(e.attenuationColor),e.anisotropy!==void 0&&(this.anisotropy=e.anisotropy),e.anisotropyRotation!==void 0&&(this.anisotropyRotation=e.anisotropyRotation),e.fog!==void 0&&(this.fog=e.fog),e.flatShading!==void 0&&(this.flatShading=e.flatShading),e.blending!==void 0&&(this.blending=e.blending),e.combine!==void 0&&(this.combine=e.combine),e.side!==void 0&&(this.side=e.side),e.shadowSide!==void 0&&(this.shadowSide=e.shadowSide),e.opacity!==void 0&&(this.opacity=e.opacity),e.transparent!==void 0&&(this.transparent=e.transparent),e.alphaTest!==void 0&&(this.alphaTest=e.alphaTest),e.alphaHash!==void 0&&(this.alphaHash=e.alphaHash),e.depthFunc!==void 0&&(this.depthFunc=e.depthFunc),e.depthTest!==void 0&&(this.depthTest=e.depthTest),e.depthWrite!==void 0&&(this.depthWrite=e.depthWrite),e.colorWrite!==void 0&&(this.colorWrite=e.colorWrite),e.clippingPlanes!==void 0&&(this.clippingPlanes=e.clippingPlanes.map(n=>new Xn().fromJSON(n))),e.clipIntersection!==void 0&&(this.clipIntersection=e.clipIntersection),e.clipShadows!==void 0&&(this.clipShadows=e.clipShadows),e.depthPacking!==void 0&&(this.depthPacking=e.depthPacking),e.blendSrc!==void 0&&(this.blendSrc=e.blendSrc),e.blendDst!==void 0&&(this.blendDst=e.blendDst),e.blendEquation!==void 0&&(this.blendEquation=e.blendEquation),e.blendSrcAlpha!==void 0&&(this.blendSrcAlpha=e.blendSrcAlpha),e.blendDstAlpha!==void 0&&(this.blendDstAlpha=e.blendDstAlpha),e.blendEquationAlpha!==void 0&&(this.blendEquationAlpha=e.blendEquationAlpha),e.blendColor!==void 0&&this.blendColor!==void 0&&this.blendColor.setHex(e.blendColor),e.blendAlpha!==void 0&&(this.blendAlpha=e.blendAlpha),e.stencilWriteMask!==void 0&&(this.stencilWriteMask=e.stencilWriteMask),e.stencilFunc!==void 0&&(this.stencilFunc=e.stencilFunc),e.stencilRef!==void 0&&(this.stencilRef=e.stencilRef),e.stencilFuncMask!==void 0&&(this.stencilFuncMask=e.stencilFuncMask),e.stencilFail!==void 0&&(this.stencilFail=e.stencilFail),e.stencilZFail!==void 0&&(this.stencilZFail=e.stencilZFail),e.stencilZPass!==void 0&&(this.stencilZPass=e.stencilZPass),e.stencilWrite!==void 0&&(this.stencilWrite=e.stencilWrite),e.wireframe!==void 0&&(this.wireframe=e.wireframe),e.wireframeLinewidth!==void 0&&(this.wireframeLinewidth=e.wireframeLinewidth),e.wireframeLinecap!==void 0&&(this.wireframeLinecap=e.wireframeLinecap),e.wireframeLinejoin!==void 0&&(this.wireframeLinejoin=e.wireframeLinejoin),e.rotation!==void 0&&(this.rotation=e.rotation),e.linewidth!==void 0&&(this.linewidth=e.linewidth),e.linecap!==void 0&&(this.linecap=e.linecap),e.linejoin!==void 0&&(this.linejoin=e.linejoin),e.dashSize!==void 0&&(this.dashSize=e.dashSize),e.gapSize!==void 0&&(this.gapSize=e.gapSize),e.scale!==void 0&&(this.scale=e.scale),e.polygonOffset!==void 0&&(this.polygonOffset=e.polygonOffset),e.polygonOffsetFactor!==void 0&&(this.polygonOffsetFactor=e.polygonOffsetFactor),e.polygonOffsetUnits!==void 0&&(this.polygonOffsetUnits=e.polygonOffsetUnits),e.dithering!==void 0&&(this.dithering=e.dithering),e.alphaToCoverage!==void 0&&(this.alphaToCoverage=e.alphaToCoverage),e.premultipliedAlpha!==void 0&&(this.premultipliedAlpha=e.premultipliedAlpha),e.forceSinglePass!==void 0&&(this.forceSinglePass=e.forceSinglePass),e.allowOverride!==void 0&&(this.allowOverride=e.allowOverride),e.visible!==void 0&&(this.visible=e.visible),e.toneMapped!==void 0&&(this.toneMapped=e.toneMapped),e.userData!==void 0&&(this.userData=e.userData),e.vertexColors!==void 0&&(typeof e.vertexColors=="number"?this.vertexColors=e.vertexColors>0:this.vertexColors=e.vertexColors),e.size!==void 0&&(this.size=e.size),e.sizeAttenuation!==void 0&&(this.sizeAttenuation=e.sizeAttenuation),e.map!==void 0&&(this.map=t[e.map]||null),e.matcap!==void 0&&(this.matcap=t[e.matcap]||null),e.alphaMap!==void 0&&(this.alphaMap=t[e.alphaMap]||null),e.bumpMap!==void 0&&(this.bumpMap=t[e.bumpMap]||null),e.bumpScale!==void 0&&(this.bumpScale=e.bumpScale),e.normalMap!==void 0&&(this.normalMap=t[e.normalMap]||null),e.normalMapType!==void 0&&(this.normalMapType=e.normalMapType),e.normalScale!==void 0){let n=e.normalScale;Array.isArray(n)===!1&&(n=[n,n]),this.normalScale=new Be().fromArray(n)}return e.displacementMap!==void 0&&(this.displacementMap=t[e.displacementMap]||null),e.displacementScale!==void 0&&(this.displacementScale=e.displacementScale),e.displacementBias!==void 0&&(this.displacementBias=e.displacementBias),e.roughnessMap!==void 0&&(this.roughnessMap=t[e.roughnessMap]||null),e.metalnessMap!==void 0&&(this.metalnessMap=t[e.metalnessMap]||null),e.emissiveMap!==void 0&&(this.emissiveMap=t[e.emissiveMap]||null),e.emissiveIntensity!==void 0&&(this.emissiveIntensity=e.emissiveIntensity),e.specularMap!==void 0&&(this.specularMap=t[e.specularMap]||null),e.specularIntensityMap!==void 0&&(this.specularIntensityMap=t[e.specularIntensityMap]||null),e.specularColorMap!==void 0&&(this.specularColorMap=t[e.specularColorMap]||null),e.envMap!==void 0&&(this.envMap=t[e.envMap]||null),e.envMapRotation!==void 0&&this.envMapRotation.fromArray(e.envMapRotation),e.envMapIntensity!==void 0&&(this.envMapIntensity=e.envMapIntensity),e.reflectivity!==void 0&&(this.reflectivity=e.reflectivity),e.refractionRatio!==void 0&&(this.refractionRatio=e.refractionRatio),e.lightMap!==void 0&&(this.lightMap=t[e.lightMap]||null),e.lightMapIntensity!==void 0&&(this.lightMapIntensity=e.lightMapIntensity),e.aoMap!==void 0&&(this.aoMap=t[e.aoMap]||null),e.aoMapIntensity!==void 0&&(this.aoMapIntensity=e.aoMapIntensity),e.gradientMap!==void 0&&(this.gradientMap=t[e.gradientMap]||null),e.clearcoatMap!==void 0&&(this.clearcoatMap=t[e.clearcoatMap]||null),e.clearcoatRoughnessMap!==void 0&&(this.clearcoatRoughnessMap=t[e.clearcoatRoughnessMap]||null),e.clearcoatNormalMap!==void 0&&(this.clearcoatNormalMap=t[e.clearcoatNormalMap]||null),e.clearcoatNormalScale!==void 0&&(this.clearcoatNormalScale=new Be().fromArray(e.clearcoatNormalScale)),e.iridescenceMap!==void 0&&(this.iridescenceMap=t[e.iridescenceMap]||null),e.iridescenceThicknessMap!==void 0&&(this.iridescenceThicknessMap=t[e.iridescenceThicknessMap]||null),e.transmissionMap!==void 0&&(this.transmissionMap=t[e.transmissionMap]||null),e.thicknessMap!==void 0&&(this.thicknessMap=t[e.thicknessMap]||null),e.anisotropyMap!==void 0&&(this.anisotropyMap=t[e.anisotropyMap]||null),e.sheenColorMap!==void 0&&(this.sheenColorMap=t[e.sheenColorMap]||null),e.sheenRoughnessMap!==void 0&&(this.sheenRoughnessMap=t[e.sheenRoughnessMap]||null),this}clone(){return new this.constructor().copy(this)}copy(e){this.name=e.name,this.blending=e.blending,this.side=e.side,this.vertexColors=e.vertexColors,this.opacity=e.opacity,this.transparent=e.transparent,this.blendSrc=e.blendSrc,this.blendDst=e.blendDst,this.blendEquation=e.blendEquation,this.blendSrcAlpha=e.blendSrcAlpha,this.blendDstAlpha=e.blendDstAlpha,this.blendEquationAlpha=e.blendEquationAlpha,this.blendColor.copy(e.blendColor),this.blendAlpha=e.blendAlpha,this.depthFunc=e.depthFunc,this.depthTest=e.depthTest,this.depthWrite=e.depthWrite,this.stencilWriteMask=e.stencilWriteMask,this.stencilFunc=e.stencilFunc,this.stencilRef=e.stencilRef,this.stencilFuncMask=e.stencilFuncMask,this.stencilFail=e.stencilFail,this.stencilZFail=e.stencilZFail,this.stencilZPass=e.stencilZPass,this.stencilWrite=e.stencilWrite;let t=e.clippingPlanes,n=null;if(t!==null){let i=t.length;n=new Array(i);for(let r=0;r!==i;++r)n[r]=t[r].clone()}return this.clippingPlanes=n,this.clipIntersection=e.clipIntersection,this.clipShadows=e.clipShadows,this.shadowSide=e.shadowSide,this.colorWrite=e.colorWrite,this.precision=e.precision,this.polygonOffset=e.polygonOffset,this.polygonOffsetFactor=e.polygonOffsetFactor,this.polygonOffsetUnits=e.polygonOffsetUnits,this.dithering=e.dithering,this.alphaTest=e.alphaTest,this.alphaHash=e.alphaHash,this.alphaToCoverage=e.alphaToCoverage,this.premultipliedAlpha=e.premultipliedAlpha,this.forceSinglePass=e.forceSinglePass,this.allowOverride=e.allowOverride,this.visible=e.visible,this.toneMapped=e.toneMapped,this.userData=JSON.parse(JSON.stringify(e.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(e){e===!0&&this.version++}};var Wi=new B,lh=new B,Ro=new B,Co=new B,Ri=class{constructor(e=new B,t=new B(0,0,-1)){this.origin=e,this.direction=t}set(e,t){return this.origin.copy(e),this.direction.copy(t),this}copy(e){return this.origin.copy(e.origin),this.direction.copy(e.direction),this}at(e,t){return t.copy(this.origin).addScaledVector(this.direction,e)}lookAt(e){return this.direction.copy(e).sub(this.origin).normalize(),this}recast(e){return this.origin.copy(this.at(e,Wi)),this}closestPointToPoint(e,t){t.subVectors(e,this.origin);let n=t.dot(this.direction);return n<0?t.copy(this.origin):t.copy(this.origin).addScaledVector(this.direction,n)}distanceToPoint(e){return Math.sqrt(this.distanceSqToPoint(e))}distanceSqToPoint(e){let t=Wi.subVectors(e,this.origin).dot(this.direction);return t<0?this.origin.distanceToSquared(e):(Wi.copy(this.origin).addScaledVector(this.direction,t),Wi.distanceToSquared(e))}distanceSqToSegment(e,t,n,i){lh.copy(e).add(t).multiplyScalar(.5),Ro.copy(t).sub(e).normalize(),Co.copy(this.origin).sub(lh);let r=e.distanceTo(t)*.5,a=-this.direction.dot(Ro),o=Co.dot(this.direction),l=-Co.dot(Ro),c=Co.lengthSq(),h=Math.abs(1-a*a),u,d,p,x;if(h>0)if(u=a*l-o,d=a*o-l,x=r*h,u>=0)if(d>=-x)if(d<=x){let v=1/h;u*=v,d*=v,p=u*(u+a*d+2*o)+d*(a*u+d+2*l)+c}else d=r,u=Math.max(0,-(a*d+o)),p=-u*u+d*(d+2*l)+c;else d=-r,u=Math.max(0,-(a*d+o)),p=-u*u+d*(d+2*l)+c;else d<=-x?(u=Math.max(0,-(-a*r+o)),d=u>0?-r:Math.min(Math.max(-r,-l),r),p=-u*u+d*(d+2*l)+c):d<=x?(u=0,d=Math.min(Math.max(-r,-l),r),p=d*(d+2*l)+c):(u=Math.max(0,-(a*r+o)),d=u>0?r:Math.min(Math.max(-r,-l),r),p=-u*u+d*(d+2*l)+c);else d=a>0?-r:r,u=Math.max(0,-(a*d+o)),p=-u*u+d*(d+2*l)+c;return n&&n.copy(this.origin).addScaledVector(this.direction,u),i&&i.copy(lh).addScaledVector(Ro,d),p}intersectSphere(e,t){if(e.radius<0)return null;Wi.subVectors(e.center,this.origin);let n=Wi.dot(this.direction),i=Wi.dot(Wi)-n*n,r=e.radius*e.radius;if(i>r)return null;let a=Math.sqrt(r-i),o=n-a,l=n+a;return l<0?null:o<0?this.at(l,t):this.at(o,t)}intersectsSphere(e){return e.radius<0?!1:this.distanceSqToPoint(e.center)<=e.radius*e.radius}distanceToPlane(e){let t=e.normal.dot(this.direction);if(t===0)return e.distanceToPoint(this.origin)===0?0:null;let n=-(this.origin.dot(e.normal)+e.constant)/t;return n>=0?n:null}intersectPlane(e,t){let n=this.distanceToPlane(e);return n===null?null:this.at(n,t)}intersectsPlane(e){let t=e.distanceToPoint(this.origin);return t===0||e.normal.dot(this.direction)*t<0}intersectBox(e,t){let n,i,r,a,o,l,c=1/this.direction.x,h=1/this.direction.y,u=1/this.direction.z,d=this.origin;return c>=0?(n=(e.min.x-d.x)*c,i=(e.max.x-d.x)*c):(n=(e.max.x-d.x)*c,i=(e.min.x-d.x)*c),h>=0?(r=(e.min.y-d.y)*h,a=(e.max.y-d.y)*h):(r=(e.max.y-d.y)*h,a=(e.min.y-d.y)*h),n>a||r>i||((r>n||isNaN(n))&&(n=r),(a<i||isNaN(i))&&(i=a),u>=0?(o=(e.min.z-d.z)*u,l=(e.max.z-d.z)*u):(o=(e.max.z-d.z)*u,l=(e.min.z-d.z)*u),n>l||o>i)||((o>n||n!==n)&&(n=o),(l<i||i!==i)&&(i=l),i<0)?null:this.at(n>=0?n:i,t)}intersectsBox(e){return this.intersectBox(e,Wi)!==null}intersectTriangle(e,t,n,i,r){let a=this.origin,o=this.direction,l=o.x,c=o.y,h=o.z,u=e.x-a.x,d=e.y-a.y,p=e.z-a.z,x=t.x-a.x,v=t.y-a.y,m=t.z-a.z,f=n.x-a.x,y=n.y-a.y,T=n.z-a.z,g=Math.abs(l),_=Math.abs(c),w=Math.abs(h),A,b,C,S,P,M,F,I,N,H,D,V;if(g>=_&&g>=w?(C=l,M=u,N=x,V=f,l>=0?(A=c,b=h,S=d,P=p,F=v,I=m,H=y,D=T):(A=h,b=c,S=p,P=d,F=m,I=v,H=T,D=y)):_>=w?(C=c,M=d,N=v,V=y,c>=0?(A=h,b=l,S=p,P=u,F=m,I=x,H=T,D=f):(A=l,b=h,S=u,P=p,F=x,I=m,H=f,D=T)):(C=h,M=p,N=m,V=T,h>=0?(A=l,b=c,S=u,P=d,F=x,I=v,H=f,D=y):(A=c,b=l,S=d,P=u,F=v,I=x,H=y,D=f)),C===0)return null;let k=A/C,E=b/C,Z=1/C,W=S-k*M,Q=P-E*M,Fe=F-k*N,Ne=I-E*N,ze=H-k*V,se=D-E*V,le=ze*Ne-se*Fe,De=W*se-Q*ze,be=Fe*Q-Ne*W;if(i){if(le<0||De<0||be<0)return null}else if((le<0||De<0||be<0)&&(le>0||De>0||be>0))return null;let ue=le+De+be;if(ue===0)return null;let te=Z*(le*M+De*N+be*V);return(ue>0?te<0:te>0)?null:this.at(te/ue,r)}applyMatrix4(e){return this.origin.applyMatrix4(e),this.direction.transformDirection(e),this}equals(e){return e.origin.equals(this.origin)&&e.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},Vt=class extends On{constructor(e){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new Le(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new kn,this.combine=Al,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.fog=e.fog,this}},Td=new ft,Rs=new Ri,Po=new Cn,Ad=new B,Io=new B,Do=new B,Lo=new B,ch=new B,Fo=new B,Rd=new B,No=new B,st=class extends Wt{constructor(e=new _t,t=new Vt){super(),this.isMesh=!0,this.type="Mesh",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.count=1,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),e.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=e.morphTargetInfluences.slice()),e.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},e.morphTargetDictionary)),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}getVertexPosition(e,t){let n=this.geometry,i=n.attributes.position,r=n.morphAttributes.position,a=n.morphTargetsRelative;t.fromBufferAttribute(i,e);let o=this.morphTargetInfluences;if(r&&o){Fo.set(0,0,0);for(let l=0,c=r.length;l<c;l++){let h=o[l],u=r[l];h!==0&&(ch.fromBufferAttribute(u,e),a?Fo.addScaledVector(ch,h):Fo.addScaledVector(ch.sub(t),h))}t.add(Fo)}return t}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.material,r=this.matrixWorld;i!==void 0&&(n.boundingSphere===null&&n.computeBoundingSphere(),Po.copy(n.boundingSphere),Po.applyMatrix4(r),Rs.copy(e.ray).recast(e.near),!(Po.containsPoint(Rs.origin)===!1&&(Rs.intersectSphere(Po,Ad)===null||Rs.origin.distanceToSquared(Ad)>(e.far-e.near)**2))&&(Td.copy(r).invert(),Rs.copy(e.ray).applyMatrix4(Td),!(n.boundingBox!==null&&Rs.intersectsBox(n.boundingBox)===!1)&&this._computeIntersections(e,t,Rs)))}_computeIntersections(e,t,n){let i,r=this.geometry,a=this.material,o=r.index,l=r.attributes.position,c=r.attributes.uv,h=r.attributes.uv1,u=r.attributes.normal,d=r.groups,p=r.drawRange;if(o!==null)if(Array.isArray(a))for(let x=0,v=d.length;x<v;x++){let m=d[x],f=a[m.materialIndex],y=Math.max(m.start,p.start),T=Math.min(o.count,Math.min(m.start+m.count,p.start+p.count));for(let g=y,_=T;g<_;g+=3){let w=o.getX(g),A=o.getX(g+1),b=o.getX(g+2);i=Uo(this,f,e,n,c,h,u,w,A,b),i&&(i.faceIndex=Math.floor(g/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let x=Math.max(0,p.start),v=Math.min(o.count,p.start+p.count);for(let m=x,f=v;m<f;m+=3){let y=o.getX(m),T=o.getX(m+1),g=o.getX(m+2);i=Uo(this,a,e,n,c,h,u,y,T,g),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}else if(l!==void 0)if(Array.isArray(a))for(let x=0,v=d.length;x<v;x++){let m=d[x],f=a[m.materialIndex],y=Math.max(m.start,p.start),T=Math.min(l.count,Math.min(m.start+m.count,p.start+p.count));for(let g=y,_=T;g<_;g+=3){let w=g,A=g+1,b=g+2;i=Uo(this,f,e,n,c,h,u,w,A,b),i&&(i.faceIndex=Math.floor(g/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let x=Math.max(0,p.start),v=Math.min(l.count,p.start+p.count);for(let m=x,f=v;m<f;m+=3){let y=m,T=m+1,g=m+2;i=Uo(this,a,e,n,c,h,u,y,T,g),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}}};function Mg(s,e,t,n,i,r,a,o){let l;if(e.side===mn?l=n.intersectTriangle(a,r,i,!0,o):l=n.intersectTriangle(i,r,a,e.side===ti,o),l===null)return null;No.copy(o),No.applyMatrix4(s.matrixWorld);let c=t.ray.origin.distanceTo(No);return c<t.near||c>t.far?null:{distance:c,point:No.clone(),object:s}}function Uo(s,e,t,n,i,r,a,o,l,c){s.getVertexPosition(o,Io),s.getVertexPosition(l,Do),s.getVertexPosition(c,Lo);let h=Mg(s,e,t,n,Io,Do,Lo,Rd);if(h){let u=new B;cs.getBarycoord(Rd,Io,Do,Lo,u),i&&(h.uv=cs.getInterpolatedAttribute(i,o,l,c,u,new Be)),r&&(h.uv1=cs.getInterpolatedAttribute(r,o,l,c,u,new Be)),a&&(h.normal=cs.getInterpolatedAttribute(a,o,l,c,u,new B),h.normal.dot(n.direction)>0&&h.normal.multiplyScalar(-1));let d={a:o,b:l,c,normal:new B,materialIndex:0};cs.getNormal(Io,Do,Lo,d.normal),h.face=d,h.barycoord=u}return h}var oa=new Ot,Cd=new Ot,Pd=new Ot,Sg=new Ot,Id=new ft,Oo=new B,hh=new Cn,Dd=new ft,uh=new Ri,Ma=class extends st{constructor(e,t){super(e,t),this.isSkinnedMesh=!0,this.type="SkinnedMesh",this.bindMode=bh,this.bindMatrix=new ft,this.bindMatrixInverse=new ft,this.boundingBox=null,this.boundingSphere=null}computeBoundingBox(){let e=this.geometry;this.boundingBox===null&&(this.boundingBox=new cn),this.boundingBox.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,Oo),this.boundingBox.expandByPoint(Oo)}computeBoundingSphere(){let e=this.geometry;this.boundingSphere===null&&(this.boundingSphere=new Cn),this.boundingSphere.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,Oo),this.boundingSphere.expandByPoint(Oo)}copy(e,t){return super.copy(e,t),this.bindMode=e.bindMode,this.bindMatrix.copy(e.bindMatrix),this.bindMatrixInverse.copy(e.bindMatrixInverse),this.skeleton=e.skeleton,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}raycast(e,t){let n=this.material,i=this.matrixWorld;n!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),hh.copy(this.boundingSphere),hh.applyMatrix4(i),e.ray.intersectsSphere(hh)!==!1&&(Dd.copy(i).invert(),uh.copy(e.ray).applyMatrix4(Dd),!(this.boundingBox!==null&&uh.intersectsBox(this.boundingBox)===!1)&&this._computeIntersections(e,t,uh)))}getVertexPosition(e,t){return super.getVertexPosition(e,t),this.applyBoneTransform(e,t),t}bind(e,t){this.skeleton=e,t===void 0&&(this.updateMatrixWorld(!0),this.skeleton.calculateInverses(),t=this.matrixWorld),this.bindMatrix.copy(t),this.bindMatrixInverse.copy(t).invert()}pose(){this.skeleton.pose()}normalizeSkinWeights(){let e=new Ot,t=this.geometry.attributes.skinWeight;for(let n=0,i=t.count;n<i;n++){e.fromBufferAttribute(t,n);let r=1/e.manhattanLength();r!==1/0?e.multiplyScalar(r):e.set(1,0,0,0),t.setXYZW(n,e.x,e.y,e.z,e.w)}}updateMatrixWorld(e){super.updateMatrixWorld(e),this.bindMode===bh?this.bindMatrixInverse.copy(this.matrixWorld).invert():this.bindMode===Tf?this.bindMatrixInverse.copy(this.bindMatrix).invert():rt("SkinnedMesh: Unrecognized bindMode: "+this.bindMode)}applyBoneTransform(e,t){let n=this.skeleton,i=this.geometry;Cd.fromBufferAttribute(i.attributes.skinIndex,e),Pd.fromBufferAttribute(i.attributes.skinWeight,e),t.isVector4?(oa.copy(t),t.set(0,0,0,0)):(oa.set(...t,1),t.set(0,0,0)),oa.applyMatrix4(this.bindMatrix);for(let r=0;r<4;r++){let a=Pd.getComponent(r);if(a!==0){let o=Cd.getComponent(r);Id.multiplyMatrices(n.bones[o].matrixWorld,n.boneInverses[o]),t.addScaledVector(Sg.copy(oa).applyMatrix4(Id),a)}}return t.isVector4&&(t.w=oa.w),t.applyMatrix4(this.bindMatrixInverse)}},Tr=class extends Wt{constructor(){super(),this.isBone=!0,this.type="Bone"}},hs=class extends ln{constructor(e=null,t=1,n=1,i,r,a,o,l,c=on,h=on,u,d){super(null,a,o,l,c,h,i,r,u,d),this.isDataTexture=!0,this.image={data:e,width:t,height:n},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}},Ld=new ft,wg=new ft,Sa=class s{constructor(e=[],t=[]){this.uuid=di(),this.bones=e.slice(0),this.boneInverses=t,this.boneMatrices=null,this.boneTexture=null,this.init()}init(){let e=this.bones,t=this.boneInverses;if(this.boneMatrices=new Float32Array(e.length*16),t.length===0)this.calculateInverses();else if(e.length!==t.length){rt("Skeleton: Number of inverse bone matrices does not match amount of bones."),this.boneInverses=[];for(let n=0,i=this.bones.length;n<i;n++)this.boneInverses.push(new ft)}}calculateInverses(){this.boneInverses.length=0;for(let e=0,t=this.bones.length;e<t;e++){let n=new ft;this.bones[e]&&n.copy(this.bones[e].matrixWorld).invert(),this.boneInverses.push(n)}}pose(){for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&n.matrixWorld.copy(this.boneInverses[e]).invert()}for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&(n.parent&&n.parent.isBone?(n.matrix.copy(n.parent.matrixWorld).invert(),n.matrix.multiply(n.matrixWorld)):n.matrix.copy(n.matrixWorld),n.matrix.decompose(n.position,n.quaternion,n.scale))}}update(){let e=this.bones,t=this.boneInverses,n=this.boneMatrices,i=this.boneTexture;for(let r=0,a=e.length;r<a;r++){let o=e[r]?e[r].matrixWorld:wg;Ld.multiplyMatrices(o,t[r]),Ld.toArray(n,r*16)}i!==null&&(i.needsUpdate=!0)}clone(){return new s(this.bones,this.boneInverses)}computeBoneTexture(){let e=Math.sqrt(this.bones.length*4);e=Math.ceil(e/4)*4,e=Math.max(e,4);let t=new Float32Array(e*e*4);t.set(this.boneMatrices);let n=new hs(t,e,e,Jn,Yn);return n.needsUpdate=!0,this.boneMatrices=t,this.boneTexture=n,this}getBoneByName(e){for(let t=0,n=this.bones.length;t<n;t++){let i=this.bones[t];if(i.name===e)return i}}dispose(){this.boneTexture!==null&&(this.boneTexture.dispose(),this.boneTexture=null)}fromJSON(e,t){this.uuid=e.uuid;for(let n=0,i=e.bones.length;n<i;n++){let r=e.bones[n],a=t[r];a===void 0&&(rt("Skeleton: No bone found with UUID:",r),a=new Tr),this.bones.push(a),this.boneInverses.push(new ft().fromArray(e.boneInverses[n]))}return this.init(),this}toJSON(){let e={metadata:{version:4.7,type:"Skeleton",generator:"Skeleton.toJSON"},bones:[],boneInverses:[]};e.uuid=this.uuid;let t=this.bones,n=this.boneInverses;for(let i=0,r=t.length;i<r;i++){let a=t[i];e.bones.push(a.uuid);let o=n[i];e.boneInverses.push(o.toArray())}return e}},fi=class extends Rt{constructor(e,t,n,i=1){super(e,t,n),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=i}copy(e){return super.copy(e),this.meshPerAttribute=e.meshPerAttribute,this}toJSON(){let e=super.toJSON();return e.meshPerAttribute=this.meshPerAttribute,e.isInstancedBufferAttribute=!0,e}},ur=new ft,Fd=new ft,zo=[],Nd=new cn,Eg=new ft,la=new st,ca=new Cn,Sn=class extends st{constructor(e,t,n){super(e,t),this.isInstancedMesh=!0,this.instanceMatrix=new fi(new Float32Array(n*16),16),this.instanceColor=null,this.morphTexture=null,this.count=n,this.boundingBox=null,this.boundingSphere=null;for(let i=0;i<n;i++)this.setMatrixAt(i,Eg)}computeBoundingBox(){let e=this.geometry,t=this.count;this.boundingBox===null&&(this.boundingBox=new cn),e.boundingBox===null&&e.computeBoundingBox(),this.boundingBox.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,ur),Nd.copy(e.boundingBox).applyMatrix4(ur),this.boundingBox.union(Nd)}computeBoundingSphere(){let e=this.geometry,t=this.count;this.boundingSphere===null&&(this.boundingSphere=new Cn),e.boundingSphere===null&&e.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,ur),ca.copy(e.boundingSphere).applyMatrix4(ur),this.boundingSphere.union(ca)}copy(e,t){return super.copy(e,t),this.instanceMatrix.copy(e.instanceMatrix),e.morphTexture!==null&&(this.morphTexture=e.morphTexture.clone()),e.instanceColor!==null&&(this.instanceColor=e.instanceColor.clone()),this.count=e.count,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}getColorAt(e,t){return this.instanceColor===null?t.setRGB(1,1,1):t.fromArray(this.instanceColor.array,e*3)}getMatrixAt(e,t){return t.fromArray(this.instanceMatrix.array,e*16)}getMorphAt(e,t){let n=t.morphTargetInfluences,i=this.morphTexture.source.data.data,r=n.length+1,a=e*r+1;for(let o=0;o<n.length;o++)n[o]=i[a+o]}raycast(e,t){let n=this.matrixWorld,i=this.count;if(la.geometry=this.geometry,la.material=this.material,la.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),ca.copy(this.boundingSphere),ca.applyMatrix4(n),e.ray.intersectsSphere(ca)!==!1))for(let r=0;r<i;r++){this.getMatrixAt(r,ur),Fd.multiplyMatrices(n,ur),la.matrixWorld=Fd,la.raycast(e,zo);for(let a=0,o=zo.length;a<o;a++){let l=zo[a];l.instanceId=r,l.object=this,t.push(l)}zo.length=0}}setColorAt(e,t){return this.instanceColor===null&&(this.instanceColor=new fi(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),t.toArray(this.instanceColor.array,e*3),this}setMatrixAt(e,t){return t.toArray(this.instanceMatrix.array,e*16),this}setMorphAt(e,t){let n=t.morphTargetInfluences,i=n.length+1;this.morphTexture===null&&(this.morphTexture=new hs(new Float32Array(i*this.count),i,this.count,Fl,Yn));let r=this.morphTexture.source.data.data,a=0;for(let c=0;c<n.length;c++)a+=n[c];let o=this.geometry.morphTargetsRelative?1:1-a,l=i*e;return r[l]=o,r.set(n,l+1),this}updateMorphTargets(){}dispose(){super.dispose(),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null)}},Cs=new Cn,Tg=new Be(.5,.5),ko=new B,Ar=class{constructor(e=new Xn,t=new Xn,n=new Xn,i=new Xn,r=new Xn,a=new Xn){this.planes=[e,t,n,i,r,a]}set(e,t,n,i,r,a){let o=this.planes;return o[0].copy(e),o[1].copy(t),o[2].copy(n),o[3].copy(i),o[4].copy(r),o[5].copy(a),this}copy(e){let t=this.planes;for(let n=0;n<6;n++)t[n].copy(e.planes[n]);return this}setFromProjectionMatrix(e,t=ui,n=!1){let i=this.planes,r=e.elements,a=r[0],o=r[1],l=r[2],c=r[3],h=r[4],u=r[5],d=r[6],p=r[7],x=r[8],v=r[9],m=r[10],f=r[11],y=r[12],T=r[13],g=r[14],_=r[15];if(i[0].setComponents(c-a,p-h,f-x,_-y).normalize(),i[1].setComponents(c+a,p+h,f+x,_+y).normalize(),i[2].setComponents(c+o,p+u,f+v,_+T).normalize(),i[3].setComponents(c-o,p-u,f-v,_-T).normalize(),n)i[4].setComponents(l,d,m,g).normalize(),i[5].setComponents(c-l,p-d,f-m,_-g).normalize();else if(i[4].setComponents(c-l,p-d,f-m,_-g).normalize(),t===ui)i[5].setComponents(c+l,p+d,f+m,_+g).normalize();else if(t===br)i[5].setComponents(l,d,m,g).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+t);return this}intersectsObject(e){if(e.boundingSphere!==void 0)e.boundingSphere===null&&e.computeBoundingSphere(),Cs.copy(e.boundingSphere).applyMatrix4(e.matrixWorld);else{let t=e.geometry;t.boundingSphere===null&&t.computeBoundingSphere(),Cs.copy(t.boundingSphere).applyMatrix4(e.matrixWorld)}return this.intersectsSphere(Cs)}intersectsSprite(e){Cs.center.set(0,0,0);let t=Tg.distanceTo(e.center);return Cs.radius=.7071067811865476+t,Cs.applyMatrix4(e.matrixWorld),this.intersectsSphere(Cs)}intersectsSphere(e){let t=this.planes,n=e.center,i=-e.radius;for(let r=0;r<6;r++)if(t[r].distanceToPoint(n)<i)return!1;return!0}intersectsBox(e){let t=this.planes;for(let n=0;n<6;n++){let i=t[n];if(ko.x=i.normal.x>0?e.max.x:e.min.x,ko.y=i.normal.y>0?e.max.y:e.min.y,ko.z=i.normal.z>0?e.max.z:e.min.z,i.distanceToPoint(ko)<0)return!1}return!0}containsPoint(e){let t=this.planes;for(let n=0;n<6;n++)if(t[n].distanceToPoint(e)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};var Ci=class extends On{constructor(e){super(),this.isLineBasicMaterial=!0,this.type="LineBasicMaterial",this.color=new Le(16777215),this.map=null,this.linewidth=1,this.linecap="round",this.linejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.linewidth=e.linewidth,this.linecap=e.linecap,this.linejoin=e.linejoin,this.fog=e.fog,this}},rl=new B,al=new B,Ud=new ft,ha=new Ri,Bo=new Cn,dh=new B,Od=new B,ji=class extends Wt{constructor(e=new _t,t=new Ci){super(),this.isLine=!0,this.type="Line",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[0];for(let i=1,r=t.count;i<r;i++)rl.fromBufferAttribute(t,i-1),al.fromBufferAttribute(t,i),n[i]=n[i-1],n[i]+=rl.distanceTo(al);e.setAttribute("lineDistance",new mt(n,1))}else rt("Line.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Line.threshold,a=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),Bo.copy(n.boundingSphere),Bo.applyMatrix4(i),Bo.radius+=r,e.ray.intersectsSphere(Bo)===!1)return;Ud.copy(i).invert(),ha.copy(e.ray).applyMatrix4(Ud);let o=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=o*o,c=this.isLineSegments?2:1,h=n.index,d=n.attributes.position;if(h!==null){let p=Math.max(0,a.start),x=Math.min(h.count,a.start+a.count);for(let v=p,m=x-1;v<m;v+=c){let f=h.getX(v),y=h.getX(v+1),T=Ho(this,e,ha,l,f,y,v);T&&t.push(T)}if(this.isLineLoop){let v=h.getX(x-1),m=h.getX(p),f=Ho(this,e,ha,l,v,m,x-1);f&&t.push(f)}}else{let p=Math.max(0,a.start),x=Math.min(d.count,a.start+a.count);for(let v=p,m=x-1;v<m;v+=c){let f=Ho(this,e,ha,l,v,v+1,v);f&&t.push(f)}if(this.isLineLoop){let v=Ho(this,e,ha,l,x-1,p,x-1);v&&t.push(v)}}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}};function Ho(s,e,t,n,i,r,a){let o=s.geometry.attributes.position;if(rl.fromBufferAttribute(o,i),al.fromBufferAttribute(o,r),t.distanceSqToSegment(rl,al,dh,Od)>n)return;dh.applyMatrix4(s.matrixWorld);let c=e.ray.origin.distanceTo(dh);if(!(c<e.near||c>e.far))return{distance:c,point:Od.clone().applyMatrix4(s.matrixWorld),index:a,face:null,faceIndex:null,barycoord:null,object:s}}var zd=new B,kd=new B,Os=class extends ji{constructor(e,t){super(e,t),this.isLineSegments=!0,this.type="LineSegments"}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[];for(let i=0,r=t.count;i<r;i+=2)zd.fromBufferAttribute(t,i),kd.fromBufferAttribute(t,i+1),n[i]=i===0?0:n[i-1],n[i+1]=n[i]+zd.distanceTo(kd);e.setAttribute("lineDistance",new mt(n,1))}else rt("LineSegments.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}},wa=class extends ji{constructor(e,t){super(e,t),this.isLineLoop=!0,this.type="LineLoop"}},Rr=class extends On{constructor(e){super(),this.isPointsMaterial=!0,this.type="PointsMaterial",this.color=new Le(16777215),this.map=null,this.alphaMap=null,this.size=1,this.sizeAttenuation=!0,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.alphaMap=e.alphaMap,this.size=e.size,this.sizeAttenuation=e.sizeAttenuation,this.fog=e.fog,this}},Bd=new ft,_h=new Ri,Go=new Cn,Vo=new B,yn=class extends Wt{constructor(e=new _t,t=new Rr){super(),this.isPoints=!0,this.type="Points",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Points.threshold,a=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),Go.copy(n.boundingSphere),Go.applyMatrix4(i),Go.radius+=r,e.ray.intersectsSphere(Go)===!1)return;Bd.copy(i).invert(),_h.copy(e.ray).applyMatrix4(Bd);let o=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=o*o,c=n.index,u=n.attributes.position;if(c!==null){let d=Math.max(0,a.start),p=Math.min(c.count,a.start+a.count);for(let x=d,v=p;x<v;x++){let m=c.getX(x);Vo.fromBufferAttribute(u,m),Hd(Vo,m,l,i,e,t,this)}}else{let d=Math.max(0,a.start),p=Math.min(u.count,a.start+a.count);for(let x=d,v=p;x<v;x++)Vo.fromBufferAttribute(u,x),Hd(Vo,x,l,i,e,t,this)}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}};function Hd(s,e,t,n,i,r,a){let o=_h.distanceSqToPoint(s);if(o<t){let l=new B;_h.closestPointToPoint(s,l),l.applyMatrix4(n);let c=i.ray.origin.distanceTo(l);if(c<i.near||c>i.far)return;r.push({distance:c,distanceToRay:Math.sqrt(o),point:l,index:e,face:null,faceIndex:null,barycoord:null,object:a})}}var Ea=class extends ln{constructor(e=[],t=xs,n,i,r,a,o,l,c,h){super(e,t,n,i,r,a,o,l,c,h),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(e){this.image=e}},zs=class extends ln{constructor(e,t,n,i,r,a,o,l,c){super(e,t,n,i,r,a,o,l,c),this.isCanvasTexture=!0,this.needsUpdate=!0}};var us=class extends ln{constructor(e,t,n=vi,i,r,a,o=on,l=on,c,h=Ai,u=1){if(h!==Ai&&h!==bs)throw new Error("THREE.DepthTexture: format must be either THREE.DepthFormat or THREE.DepthStencilFormat");let d={width:e,height:t,depth:u};super(d,i,r,a,o,l,h,n,c),this.isDepthTexture=!0,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(e){return super.copy(e),this.source=new yr(Object.assign({},e.image)),this.compareFunction=e.compareFunction,this}toJSON(e){let t=super.toJSON(e);return t.compareFunction=this.compareFunction,t}},ol=class extends us{constructor(e,t=vi,n=xs,i,r,a=on,o=on,l,c=Ai){let h={width:e,height:e,depth:1},u=[h,h,h,h,h,h];super(e,e,t,n,i,r,a,o,l,c),this.image=u,this.isCubeDepthTexture=!0,this.isCubeTexture=!0}get images(){return this.image}set images(e){this.image=e}},Ta=class extends ln{constructor(e=null){super(),this.sourceTexture=e,this.isExternalTexture=!0}copy(e){return super.copy(e),this.sourceTexture=e.sourceTexture,this}},pi=class s extends _t{constructor(e=1,t=1,n=1,i=1,r=1,a=1){super(),this.type="BoxGeometry",this.parameters={width:e,height:t,depth:n,widthSegments:i,heightSegments:r,depthSegments:a};let o=this;i=Math.floor(i),r=Math.floor(r),a=Math.floor(a);let l=[],c=[],h=[],u=[],d=0,p=0;x("z","y","x",-1,-1,n,t,e,a,r,0),x("z","y","x",1,-1,n,t,-e,a,r,1),x("x","z","y",1,1,e,n,t,i,a,2),x("x","z","y",1,-1,e,n,-t,i,a,3),x("x","y","z",1,-1,e,t,n,i,r,4),x("x","y","z",-1,-1,e,t,-n,i,r,5),this.setIndex(l),this.setAttribute("position",new mt(c,3)),this.setAttribute("normal",new mt(h,3)),this.setAttribute("uv",new mt(u,2));function x(v,m,f,y,T,g,_,w,A,b,C){let S=g/A,P=_/b,M=g/2,F=_/2,I=w/2,N=A+1,H=b+1,D=0,V=0,k=new B;for(let E=0;E<H;E++){let Z=E*P-F;for(let W=0;W<N;W++){let Q=W*S-M;k[v]=Q*y,k[m]=Z*T,k[f]=I,c.push(k.x,k.y,k.z),k[v]=0,k[m]=0,k[f]=w>0?1:-1,h.push(k.x,k.y,k.z),u.push(W/A),u.push(1-E/b),D+=1}}for(let E=0;E<b;E++)for(let Z=0;Z<A;Z++){let W=d+Z+N*E,Q=d+Z+N*(E+1),Fe=d+(Z+1)+N*(E+1),Ne=d+(Z+1)+N*E;l.push(W,Q,Ne),l.push(Q,Fe,Ne),V+=6}o.addGroup(p,V,C),p+=V,d+=D}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.depth,e.widthSegments,e.heightSegments,e.depthSegments)}};var Cr=class s extends _t{constructor(e=1,t=32,n=0,i=Math.PI*2){super(),this.type="CircleGeometry",this.parameters={radius:e,segments:t,thetaStart:n,thetaLength:i},t=Math.max(3,t);let r=[],a=[],o=[],l=[],c=new B,h=new Be;a.push(0,0,0),o.push(0,0,1),l.push(.5,.5);for(let u=0,d=3;u<=t;u++,d+=3){let p=n+u/t*i;c.x=e*Math.cos(p),c.y=e*Math.sin(p),a.push(c.x,c.y,c.z),o.push(0,0,1),h.x=(a[d]/e+1)/2,h.y=(a[d+1]/e+1)/2,l.push(h.x,h.y)}for(let u=1;u<=t;u++)r.push(u,u+1,0);this.setIndex(r),this.setAttribute("position",new mt(a,3)),this.setAttribute("normal",new mt(o,3)),this.setAttribute("uv",new mt(l,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.segments,e.thetaStart,e.thetaLength)}},ds=class s extends _t{constructor(e=1,t=1,n=1,i=32,r=1,a=!1,o=0,l=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:e,radiusBottom:t,height:n,radialSegments:i,heightSegments:r,openEnded:a,thetaStart:o,thetaLength:l};let c=this;i=Math.floor(i),r=Math.floor(r);let h=[],u=[],d=[],p=[],x=0,v=[],m=n/2,f=0;y(),a===!1&&(e>0&&T(!0),t>0&&T(!1)),this.setIndex(h),this.setAttribute("position",new mt(u,3)),this.setAttribute("normal",new mt(d,3)),this.setAttribute("uv",new mt(p,2));function y(){let g=new B,_=new B,w=0,A=(t-e)/n;for(let b=0;b<=r;b++){let C=[],S=b/r,P=S*(t-e)+e;for(let M=0;M<=i;M++){let F=M/i,I=F*l+o,N=Math.sin(I),H=Math.cos(I);_.x=P*N,_.y=-S*n+m,_.z=P*H,u.push(_.x,_.y,_.z),g.set(N,A,H).normalize(),d.push(g.x,g.y,g.z),p.push(F,1-S),C.push(x++)}v.push(C)}for(let b=0;b<i;b++)for(let C=0;C<r;C++){let S=v[C][b],P=v[C+1][b],M=v[C+1][b+1],F=v[C][b+1];(e>0||C!==0)&&(h.push(S,P,F),w+=3),(t>0||C!==r-1)&&(h.push(P,M,F),w+=3)}c.addGroup(f,w,0),f+=w}function T(g){let _=x,w=new Be,A=new B,b=0,C=g===!0?e:t,S=g===!0?1:-1;for(let M=1;M<=i;M++)u.push(0,m*S,0),d.push(0,S,0),p.push(.5,.5),x++;let P=x;for(let M=0;M<=i;M++){let I=M/i*l+o,N=Math.cos(I),H=Math.sin(I);A.x=C*H,A.y=m*S,A.z=C*N,u.push(A.x,A.y,A.z),d.push(0,S,0),w.x=N*.5+.5,w.y=H*.5*S+.5,p.push(w.x,w.y),x++}for(let M=0;M<i;M++){let F=_+M,I=P+M;g===!0?h.push(I,I+1,F):h.push(I+1,I,F),b+=3}c.addGroup(f,b,g===!0?1:2),f+=b}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radiusTop,e.radiusBottom,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},ks=class s extends ds{constructor(e=1,t=1,n=32,i=1,r=!1,a=0,o=Math.PI*2){super(0,e,t,n,i,r,a,o),this.type="ConeGeometry",this.parameters={radius:e,height:t,radialSegments:n,heightSegments:i,openEnded:r,thetaStart:a,thetaLength:o}}static fromJSON(e){return new s(e.radius,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},ll=class s extends _t{constructor(e=[],t=[],n=1,i=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:e,indices:t,radius:n,detail:i};let r=[],a=[];o(i),c(n),h(),this.setAttribute("position",new mt(r,3)),this.setAttribute("normal",new mt(r.slice(),3)),this.setAttribute("uv",new mt(a,2)),i===0?this.computeVertexNormals():this.normalizeNormals();function o(y){let T=new B,g=new B,_=new B;for(let w=0;w<t.length;w+=3)p(t[w+0],T),p(t[w+1],g),p(t[w+2],_),l(T,g,_,y)}function l(y,T,g,_){let w=_+1,A=[];for(let b=0;b<=w;b++){A[b]=[];let C=y.clone().lerp(g,b/w),S=T.clone().lerp(g,b/w),P=w-b;for(let M=0;M<=P;M++)M===0&&b===w?A[b][M]=C:A[b][M]=C.clone().lerp(S,M/P)}for(let b=0;b<w;b++)for(let C=0;C<2*(w-b)-1;C++){let S=Math.floor(C/2);C%2===0?(d(A[b][S+1]),d(A[b+1][S]),d(A[b][S])):(d(A[b][S+1]),d(A[b+1][S+1]),d(A[b+1][S]))}}function c(y){let T=new B;for(let g=0;g<r.length;g+=3)T.x=r[g+0],T.y=r[g+1],T.z=r[g+2],T.normalize().multiplyScalar(y),r[g+0]=T.x,r[g+1]=T.y,r[g+2]=T.z}function h(){let y=new B;for(let T=0;T<r.length;T+=3){y.x=r[T+0],y.y=r[T+1],y.z=r[T+2];let g=m(y)/2/Math.PI+.5,_=f(y)/Math.PI+.5;a.push(g,1-_)}x(),u()}function u(){for(let y=0;y<a.length;y+=6){let T=a[y+0],g=a[y+2],_=a[y+4],w=Math.max(T,g,_),A=Math.min(T,g,_);w>.9&&A<.1&&(T<.2&&(a[y+0]+=1),g<.2&&(a[y+2]+=1),_<.2&&(a[y+4]+=1))}}function d(y){r.push(y.x,y.y,y.z)}function p(y,T){let g=y*3;T.x=e[g+0],T.y=e[g+1],T.z=e[g+2]}function x(){let y=new B,T=new B,g=new B,_=new B,w=new Be,A=new Be,b=new Be;for(let C=0,S=0;C<r.length;C+=9,S+=6){y.set(r[C+0],r[C+1],r[C+2]),T.set(r[C+3],r[C+4],r[C+5]),g.set(r[C+6],r[C+7],r[C+8]),w.set(a[S+0],a[S+1]),A.set(a[S+2],a[S+3]),b.set(a[S+4],a[S+5]),_.copy(y).add(T).add(g).divideScalar(3);let P=m(_);v(w,S+0,y,P),v(A,S+2,T,P),v(b,S+4,g,P)}}function v(y,T,g,_){_<0&&y.x===1&&(a[T]=y.x-1),g.x===0&&g.z===0&&(a[T]=_/2/Math.PI+.5)}function m(y){return Math.atan2(y.z,-y.x)}function f(y){return Math.atan2(-y.y,Math.sqrt(y.x*y.x+y.z*y.z))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.vertices,e.indices,e.radius,e.detail)}};var jn=class{constructor(){this.type="Curve",this.arcLengthDivisions=200,this.needsUpdate=!1,this.cacheArcLengths=null}getPoint(){rt("Curve: .getPoint() not implemented.")}getPointAt(e,t){let n=this.getUtoTmapping(e);return this.getPoint(n,t)}getPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return t}getSpacedPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPointAt(n/e));return t}getLength(){let e=this.getLengths();return e[e.length-1]}getLengths(e=this.arcLengthDivisions){if(this.cacheArcLengths&&this.cacheArcLengths.length===e+1&&!this.needsUpdate)return this.cacheArcLengths;this.needsUpdate=!1;let t=[],n,i=this.getPoint(0),r=0;t.push(0);for(let a=1;a<=e;a++)n=this.getPoint(a/e),r+=n.distanceTo(i),t.push(r),i=n;return this.cacheArcLengths=t,t}updateArcLengths(){this.needsUpdate=!0,this.getLengths()}getUtoTmapping(e,t=null){let n=this.getLengths(),i=0,r=n.length,a;t?a=t:a=e*n[r-1];let o=0,l=r-1,c;for(;o<=l;)if(i=Math.floor(o+(l-o)/2),c=n[i]-a,c<0)o=i+1;else if(c>0)l=i-1;else{l=i;break}if(i=l,n[i]===a)return i/(r-1);let h=n[i],d=n[i+1]-h,p=(a-h)/d;return(i+p)/(r-1)}getTangent(e,t){let i=e-1e-4,r=e+1e-4;i<0&&(i=0),r>1&&(r=1);let a=this.getPoint(i),o=this.getPoint(r),l=t||(a.isVector2?new Be:new B);return l.copy(o).sub(a).normalize(),l}getTangentAt(e,t){let n=this.getUtoTmapping(e);return this.getTangent(n,t)}computeFrenetFrames(e,t=!1){let n=new B,i=[],r=[],a=[],o=new B,l=new ft;for(let p=0;p<=e;p++){let x=p/e;i[p]=this.getTangentAt(x,new B)}r[0]=new B,a[0]=new B;let c=Number.MAX_VALUE,h=Math.abs(i[0].x),u=Math.abs(i[0].y),d=Math.abs(i[0].z);h<=c&&(c=h,n.set(1,0,0)),u<=c&&(c=u,n.set(0,1,0)),d<=c&&n.set(0,0,1),o.crossVectors(i[0],n).normalize(),r[0].crossVectors(i[0],o),a[0].crossVectors(i[0],r[0]);for(let p=1;p<=e;p++){if(r[p]=r[p-1].clone(),a[p]=a[p-1].clone(),o.crossVectors(i[p-1],i[p]),o.length()>Number.EPSILON){o.normalize();let x=Math.acos(At(i[p-1].dot(i[p]),-1,1));r[p].applyMatrix4(l.makeRotationAxis(o,x))}a[p].crossVectors(i[p],r[p])}if(t===!0){let p=Math.acos(At(r[0].dot(r[e]),-1,1));p/=e,i[0].dot(o.crossVectors(r[0],r[e]))>0&&(p=-p);for(let x=1;x<=e;x++)r[x].applyMatrix4(l.makeRotationAxis(i[x],p*x)),a[x].crossVectors(i[x],r[x])}return{tangents:i,normals:r,binormals:a}}clone(){return new this.constructor().copy(this)}copy(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}toJSON(){let e={metadata:{version:4.7,type:"Curve",generator:"Curve.toJSON"}};return e.arcLengthDivisions=this.arcLengthDivisions,e.type=this.type,e}fromJSON(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}},Aa=class extends jn{constructor(e=0,t=0,n=1,i=1,r=0,a=Math.PI*2,o=!1,l=0){super(),this.isEllipseCurve=!0,this.type="EllipseCurve",this.aX=e,this.aY=t,this.xRadius=n,this.yRadius=i,this.aStartAngle=r,this.aEndAngle=a,this.aClockwise=o,this.aRotation=l}getPoint(e,t=new Be){let n=t,i=Math.PI*2,r=this.aEndAngle-this.aStartAngle,a=Math.abs(r)<Number.EPSILON;for(;r<0;)r+=i;for(;r>i;)r-=i;r<Number.EPSILON&&(a?r=0:r=i),this.aClockwise===!0&&!a&&(r===i?r=-i:r=r-i);let o=this.aStartAngle+e*r,l=this.aX+this.xRadius*Math.cos(o),c=this.aY+this.yRadius*Math.sin(o);if(this.aRotation!==0){let h=Math.cos(this.aRotation),u=Math.sin(this.aRotation),d=l-this.aX,p=c-this.aY;l=d*h-p*u+this.aX,c=d*u+p*h+this.aY}return n.set(l,c)}copy(e){return super.copy(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}toJSON(){let e=super.toJSON();return e.aX=this.aX,e.aY=this.aY,e.xRadius=this.xRadius,e.yRadius=this.yRadius,e.aStartAngle=this.aStartAngle,e.aEndAngle=this.aEndAngle,e.aClockwise=this.aClockwise,e.aRotation=this.aRotation,e}fromJSON(e){return super.fromJSON(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}},cl=class extends Aa{constructor(e,t,n,i,r,a){super(e,t,n,n,i,r,a),this.isArcCurve=!0,this.type="ArcCurve"}};function qh(){let s=0,e=0,t=0,n=0;function i(r,a,o,l){s=r,e=o,t=-3*r+3*a-2*o-l,n=2*r-2*a+o+l}return{initCatmullRom:function(r,a,o,l,c){i(a,o,c*(o-r),c*(l-a))},initNonuniformCatmullRom:function(r,a,o,l,c,h,u){let d=(a-r)/c-(o-r)/(c+h)+(o-a)/h,p=(o-a)/h-(l-a)/(h+u)+(l-o)/u;d*=h,p*=h,i(a,o,d,p)},calc:function(r){let a=r*r,o=a*r;return s+e*r+t*a+n*o}}}var Gd=new B,Vd=new B,fh=new qh,ph=new qh,mh=new qh,Pr=class extends jn{constructor(e=[],t=!1,n="centripetal",i=.5){super(),this.isCatmullRomCurve3=!0,this.type="CatmullRomCurve3",this.points=e,this.closed=t,this.curveType=n,this.tension=i}getPoint(e,t=new B){let n=t,i=this.points,r=i.length,a=(r-(this.closed?0:1))*e,o=Math.floor(a),l=a-o;this.closed?o+=o>0?0:(Math.floor(Math.abs(o)/r)+1)*r:l===0&&o===r-1&&(o=r-2,l=1);let c,h;this.closed||o>0?c=i[(o-1)%r]:(Vd.subVectors(i[0],i[1]).add(i[0]),c=Vd);let u=i[o%r],d=i[(o+1)%r];if(this.closed||o+2<r?h=i[(o+2)%r]:(Gd.subVectors(i[r-1],i[r-2]).add(i[r-1]),h=Gd),this.curveType==="centripetal"||this.curveType==="chordal"){let p=this.curveType==="chordal"?.5:.25,x=Math.pow(c.distanceToSquared(u),p),v=Math.pow(u.distanceToSquared(d),p),m=Math.pow(d.distanceToSquared(h),p);v<1e-4&&(v=1),x<1e-4&&(x=v),m<1e-4&&(m=v),fh.initNonuniformCatmullRom(c.x,u.x,d.x,h.x,x,v,m),ph.initNonuniformCatmullRom(c.y,u.y,d.y,h.y,x,v,m),mh.initNonuniformCatmullRom(c.z,u.z,d.z,h.z,x,v,m)}else this.curveType==="catmullrom"&&(fh.initCatmullRom(c.x,u.x,d.x,h.x,this.tension),ph.initCatmullRom(c.y,u.y,d.y,h.y,this.tension),mh.initCatmullRom(c.z,u.z,d.z,h.z,this.tension));return n.set(fh.calc(l),ph.calc(l),mh.calc(l)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e.closed=this.closed,e.curveType=this.curveType,e.tension=this.tension,e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new B().fromArray(i))}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}};function Wd(s,e,t,n,i){let r=(n-e)*.5,a=(i-t)*.5,o=s*s,l=s*o;return(2*t-2*n+r+a)*l+(-3*t+3*n-2*r-a)*o+r*s+t}function Ag(s,e){let t=1-s;return t*t*e}function Rg(s,e){return 2*(1-s)*s*e}function Cg(s,e){return s*s*e}function fa(s,e,t,n){return Ag(s,e)+Rg(s,t)+Cg(s,n)}function Pg(s,e){let t=1-s;return t*t*t*e}function Ig(s,e){let t=1-s;return 3*t*t*s*e}function Dg(s,e){return 3*(1-s)*s*s*e}function Lg(s,e){return s*s*s*e}function pa(s,e,t,n,i){return Pg(s,e)+Ig(s,t)+Dg(s,n)+Lg(s,i)}var hl=class extends jn{constructor(e=new Be,t=new Be,n=new Be,i=new Be){super(),this.isCubicBezierCurve=!0,this.type="CubicBezierCurve",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new Be){let n=t,i=this.v0,r=this.v1,a=this.v2,o=this.v3;return n.set(pa(e,i.x,r.x,a.x,o.x),pa(e,i.y,r.y,a.y,o.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},ul=class extends jn{constructor(e=new B,t=new B,n=new B,i=new B){super(),this.isCubicBezierCurve3=!0,this.type="CubicBezierCurve3",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new B){let n=t,i=this.v0,r=this.v1,a=this.v2,o=this.v3;return n.set(pa(e,i.x,r.x,a.x,o.x),pa(e,i.y,r.y,a.y,o.y),pa(e,i.z,r.z,a.z,o.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},dl=class extends jn{constructor(e=new Be,t=new Be){super(),this.isLineCurve=!0,this.type="LineCurve",this.v1=e,this.v2=t}getPoint(e,t=new Be){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new Be){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Ir=class extends jn{constructor(e=new B,t=new B){super(),this.isLineCurve3=!0,this.type="LineCurve3",this.v1=e,this.v2=t}getPoint(e,t=new B){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new B){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},fl=class extends jn{constructor(e=new Be,t=new Be,n=new Be){super(),this.isQuadraticBezierCurve=!0,this.type="QuadraticBezierCurve",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new Be){let n=t,i=this.v0,r=this.v1,a=this.v2;return n.set(fa(e,i.x,r.x,a.x),fa(e,i.y,r.y,a.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},mi=class extends jn{constructor(e=new B,t=new B,n=new B){super(),this.isQuadraticBezierCurve3=!0,this.type="QuadraticBezierCurve3",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new B){let n=t,i=this.v0,r=this.v1,a=this.v2;return n.set(fa(e,i.x,r.x,a.x),fa(e,i.y,r.y,a.y),fa(e,i.z,r.z,a.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},pl=class extends jn{constructor(e=[]){super(),this.isSplineCurve=!0,this.type="SplineCurve",this.points=e}getPoint(e,t=new Be){let n=t,i=this.points,r=(i.length-1)*e,a=Math.floor(r),o=r-a,l=i[a===0?a:a-1],c=i[a],h=i[a>i.length-2?i.length-1:a+1],u=i[a>i.length-3?i.length-1:a+2];return n.set(Wd(o,l.x,c.x,h.x,u.x),Wd(o,l.y,c.y,h.y,u.y)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new Be().fromArray(i))}return this}},yh=Object.freeze({__proto__:null,ArcCurve:cl,CatmullRomCurve3:Pr,CubicBezierCurve:hl,CubicBezierCurve3:ul,EllipseCurve:Aa,LineCurve:dl,LineCurve3:Ir,QuadraticBezierCurve:fl,QuadraticBezierCurve3:mi,SplineCurve:pl}),Ra=class extends jn{constructor(){super(),this.type="CurvePath",this.curves=[],this.autoClose=!1}add(e){this.curves.push(e)}closePath(){let e=this.curves[0].getPoint(0),t=this.curves[this.curves.length-1].getPoint(1);if(!e.equals(t)){let n=e.isVector2===!0?"LineCurve":"LineCurve3";this.curves.push(new yh[n](t,e))}return this}getPoint(e,t){let n=e*this.getLength(),i=this.getCurveLengths(),r=0;for(;r<i.length;){if(i[r]>=n){let a=i[r]-n,o=this.curves[r],l=o.getLength(),c=l===0?0:1-a/l;return o.getPointAt(c,t)}r++}return null}getLength(){let e=this.getCurveLengths();return e[e.length-1]}updateArcLengths(){this.needsUpdate=!0,this.cacheLengths=null,this.getCurveLengths()}getCurveLengths(){if(this.cacheLengths&&this.cacheLengths.length===this.curves.length)return this.cacheLengths;let e=[],t=0;for(let n=0,i=this.curves.length;n<i;n++)t+=this.curves[n].getLength(),e.push(t);return this.cacheLengths=e,e}getSpacedPoints(e=40){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return this.autoClose&&t.push(t[0]),t}getPoints(e=12){let t=[],n;for(let i=0,r=this.curves;i<r.length;i++){let a=r[i],o=a.isEllipseCurve?e*2:a.isLineCurve||a.isLineCurve3?1:a.isSplineCurve?e*a.points.length:e,l=a.getPoints(o);for(let c=0;c<l.length;c++){let h=l[c];n&&n.equals(h)||(t.push(h),n=h)}}return this.autoClose&&t.length>1&&!t[t.length-1].equals(t[0])&&t.push(t[0]),t}copy(e){super.copy(e),this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(i.clone())}return this.autoClose=e.autoClose,this}toJSON(){let e=super.toJSON();e.autoClose=this.autoClose,e.curves=[];for(let t=0,n=this.curves.length;t<n;t++){let i=this.curves[t];e.curves.push(i.toJSON())}return e}fromJSON(e){super.fromJSON(e),this.autoClose=e.autoClose,this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(new yh[i.type]().fromJSON(i))}return this}};var Dr=class s extends ll{constructor(e=1,t=0){let n=(1+Math.sqrt(5))/2,i=[-1,n,0,1,n,0,-1,-n,0,1,-n,0,0,-1,n,0,1,n,0,-1,-n,0,1,-n,n,0,-1,n,0,1,-n,0,-1,-n,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(i,r,e,t),this.type="IcosahedronGeometry",this.parameters={radius:e,detail:t}}static fromJSON(e){return new s(e.radius,e.detail)}};var fn=class s extends _t{constructor(e=1,t=1,n=1,i=1){super(),this.type="PlaneGeometry",this.parameters={width:e,height:t,widthSegments:n,heightSegments:i};let r=e/2,a=t/2,o=Math.floor(n),l=Math.floor(i),c=o+1,h=l+1,u=e/o,d=t/l,p=[],x=[],v=[],m=[];for(let f=0;f<h;f++){let y=f*d-a;for(let T=0;T<c;T++){let g=T*u-r;x.push(g,-y,0),v.push(0,0,1),m.push(T/o),m.push(1-f/l)}}for(let f=0;f<l;f++)for(let y=0;y<o;y++){let T=y+c*f,g=y+c*(f+1),_=y+1+c*(f+1),w=y+1+c*f;p.push(T,g,w),p.push(g,_,w)}this.setIndex(p),this.setAttribute("position",new mt(x,3)),this.setAttribute("normal",new mt(v,3)),this.setAttribute("uv",new mt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.widthSegments,e.heightSegments)}},fs=class s extends _t{constructor(e=.5,t=1,n=32,i=1,r=0,a=Math.PI*2){super(),this.type="RingGeometry",this.parameters={innerRadius:e,outerRadius:t,thetaSegments:n,phiSegments:i,thetaStart:r,thetaLength:a},n=Math.max(3,n),i=Math.max(1,i);let o=[],l=[],c=[],h=[],u=e,d=(t-e)/i,p=new B,x=new Be;for(let v=0;v<=i;v++){for(let m=0;m<=n;m++){let f=r+m/n*a;p.x=u*Math.cos(f),p.y=u*Math.sin(f),l.push(p.x,p.y,p.z),c.push(0,0,1),x.x=(p.x/t+1)/2,x.y=(p.y/t+1)/2,h.push(x.x,x.y)}u+=d}for(let v=0;v<i;v++){let m=v*(n+1);for(let f=0;f<n;f++){let y=f+m,T=y,g=y+n+1,_=y+n+2,w=y+1;o.push(T,g,w),o.push(g,_,w)}}this.setIndex(o),this.setAttribute("position",new mt(l,3)),this.setAttribute("normal",new mt(c,3)),this.setAttribute("uv",new mt(h,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.innerRadius,e.outerRadius,e.thetaSegments,e.phiSegments,e.thetaStart,e.thetaLength)}};var Pi=class s extends _t{constructor(e=1,t=32,n=16,i=0,r=Math.PI*2,a=0,o=Math.PI){super(),this.type="SphereGeometry",this.parameters={radius:e,widthSegments:t,heightSegments:n,phiStart:i,phiLength:r,thetaStart:a,thetaLength:o},t=Math.max(3,Math.floor(t)),n=Math.max(2,Math.floor(n));let l=Math.min(a+o,Math.PI),c=0,h=[],u=new B,d=new B,p=[],x=[],v=[],m=[];for(let f=0;f<=n;f++){let y=[],T=f/n,g=a+T*o,_=e*Math.cos(g),w=Math.sqrt(e*e-_*_),A=0;f===0&&a===0?A=.5/t:f===n&&l===Math.PI&&(A=-.5/t);for(let b=0;b<=t;b++){let C=b/t,S=i+C*r;u.x=-w*Math.cos(S),u.y=_,u.z=w*Math.sin(S),x.push(u.x,u.y,u.z),d.copy(u).normalize(),v.push(d.x,d.y,d.z),m.push(C+A,1-T),y.push(c++)}h.push(y)}for(let f=0;f<n;f++)for(let y=0;y<t;y++){let T=h[f][y+1],g=h[f][y],_=h[f+1][y],w=h[f+1][y+1];(f!==0||a>0)&&p.push(T,g,w),(f!==n-1||l<Math.PI)&&p.push(g,_,w)}this.setIndex(p),this.setAttribute("position",new mt(x,3)),this.setAttribute("normal",new mt(v,3)),this.setAttribute("uv",new mt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.widthSegments,e.heightSegments,e.phiStart,e.phiLength,e.thetaStart,e.thetaLength)}};var Kn=class s extends _t{constructor(e=1,t=.4,n=12,i=48,r=Math.PI*2,a=0,o=Math.PI*2){super(),this.type="TorusGeometry",this.parameters={radius:e,tube:t,radialSegments:n,tubularSegments:i,arc:r,thetaStart:a,thetaLength:o},n=Math.floor(n),i=Math.floor(i);let l=[],c=[],h=[],u=[],d=new B,p=new B,x=new B;for(let v=0;v<=n;v++){let m=a+v/n*o;for(let f=0;f<=i;f++){let y=f/i*r;p.x=(e+t*Math.cos(m))*Math.cos(y),p.y=(e+t*Math.cos(m))*Math.sin(y),p.z=t*Math.sin(m),c.push(p.x,p.y,p.z),d.x=e*Math.cos(y),d.y=e*Math.sin(y),x.subVectors(p,d).normalize(),h.push(x.x,x.y,x.z),u.push(f/i),u.push(v/n)}}for(let v=1;v<=n;v++)for(let m=1;m<=i;m++){let f=(i+1)*v+m-1,y=(i+1)*(v-1)+m-1,T=(i+1)*(v-1)+m,g=(i+1)*v+m;l.push(f,y,g),l.push(y,T,g)}this.setIndex(l),this.setAttribute("position",new mt(c,3)),this.setAttribute("normal",new mt(h,3)),this.setAttribute("uv",new mt(u,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.tube,e.radialSegments,e.tubularSegments,e.arc,e.thetaStart,e.thetaLength)}};var Ca=class s extends _t{constructor(e=new mi(new B(-1,-1,0),new B(-1,1,0),new B(1,1,0)),t=64,n=1,i=8,r=!1){super(),this.type="TubeGeometry",this.parameters={path:e,tubularSegments:t,radius:n,radialSegments:i,closed:r};let a=e.computeFrenetFrames(t,r);this.tangents=a.tangents,this.normals=a.normals,this.binormals=a.binormals;let o=new B,l=new B,c=new Be,h=new B,u=[],d=[],p=[],x=[];v(),this.setIndex(x),this.setAttribute("position",new mt(u,3)),this.setAttribute("normal",new mt(d,3)),this.setAttribute("uv",new mt(p,2));function v(){for(let T=0;T<t;T++)m(T);m(r===!1?t:0),y(),f()}function m(T){h=e.getPointAt(T/t,h);let g=a.normals[T],_=a.binormals[T];for(let w=0;w<=i;w++){let A=w/i*Math.PI*2,b=Math.sin(A),C=-Math.cos(A);l.x=C*g.x+b*_.x,l.y=C*g.y+b*_.y,l.z=C*g.z+b*_.z,l.normalize(),d.push(l.x,l.y,l.z),o.x=h.x+n*l.x,o.y=h.y+n*l.y,o.z=h.z+n*l.z,u.push(o.x,o.y,o.z)}}function f(){for(let T=1;T<=t;T++)for(let g=1;g<=i;g++){let _=(i+1)*(T-1)+(g-1),w=(i+1)*T+(g-1),A=(i+1)*T+g,b=(i+1)*(T-1)+g;x.push(_,w,b),x.push(w,A,b)}}function y(){for(let T=0;T<=t;T++)for(let g=0;g<=i;g++)c.x=T/t,c.y=g/i,p.push(c.x,c.y)}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}toJSON(){let e=super.toJSON();return e.path=this.parameters.path.toJSON(),e}static fromJSON(e){return new s(new yh[e.path.type]().fromJSON(e.path),e.tubularSegments,e.radius,e.radialSegments,e.closed)}},Pa=class extends _t{constructor(e=null){if(super(),this.type="WireframeGeometry",this.parameters={geometry:e},e!==null){let t=[],n=new Set,i=new B,r=new B;if(e.index!==null){let a=e.attributes.position,o=e.index,l=e.groups;l.length===0&&(l=[{start:0,count:o.count,materialIndex:0}]);for(let c=0,h=l.length;c<h;++c){let u=l[c],d=u.start,p=u.count;for(let x=d,v=d+p;x<v;x+=3)for(let m=0;m<3;m++){let f=o.getX(x+m),y=o.getX(x+(m+1)%3);i.fromBufferAttribute(a,f),r.fromBufferAttribute(a,y),qd(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}}else{let a=e.attributes.position;for(let o=0,l=a.count/3;o<l;o++)for(let c=0;c<3;c++){let h=3*o+c,u=3*o+(c+1)%3;i.fromBufferAttribute(a,h),r.fromBufferAttribute(a,u),qd(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}this.setAttribute("position",new mt(t,3))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}};function qd(s,e,t){let n=`${s.x},${s.y},${s.z}-${e.x},${e.y},${e.z}`,i=`${e.x},${e.y},${e.z}-${s.x},${s.y},${s.z}`;return t.has(n)===!0||t.has(i)===!0?!1:(t.add(n),t.add(i),!0)}function js(s){let e={};for(let t in s){e[t]={};for(let n in s[t]){let i=s[t][n];if(Xd(i))i.isRenderTargetTexture?(rt("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),e[t][n]=null):e[t][n]=i.clone();else if(Array.isArray(i))if(Xd(i[0])){let r=[];for(let a=0,o=i.length;a<o;a++)r[a]=i[a].clone();e[t][n]=r}else e[t][n]=i.slice();else e[t][n]=i}}return e}function Pn(s){let e={};for(let t=0;t<s.length;t++){let n=js(s[t]);for(let i in n)e[i]=n[i]}return e}function Xd(s){return s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)}function Fg(s){let e=[];for(let t=0;t<s.length;t++)e.push(s[t].clone());return e}function Xh(s){let e=s.getRenderTarget();return e===null?s.outputColorSpace:e.isXRRenderTarget===!0?e.texture.colorSpace:Tt.workingColorSpace}var _i={clone:js,merge:Pn},Ng=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,Ug=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,yt=class extends On{constructor(e){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=Ng,this.fragmentShader=Ug,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,e!==void 0&&this.setValues(e)}copy(e){return super.copy(e),this.fragmentShader=e.fragmentShader,this.vertexShader=e.vertexShader,this.uniforms=js(e.uniforms),this.uniformsGroups=Fg(e.uniformsGroups),this.defines=Object.assign({},e.defines),this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.fog=e.fog,this.lights=e.lights,this.clipping=e.clipping,this.extensions=Object.assign({},e.extensions),this.glslVersion=e.glslVersion,this.defaultAttributeValues=Object.assign({},e.defaultAttributeValues),this.index0AttributeName=e.index0AttributeName,this.uniformsNeedUpdate=e.uniformsNeedUpdate,this}toJSON(e){let t=super.toJSON(e);t.glslVersion=this.glslVersion,t.uniforms={};for(let i in this.uniforms){let a=this.uniforms[i].value;a&&a.isTexture?t.uniforms[i]={type:"t",value:a.toJSON(e).uuid}:a&&a.isColor?t.uniforms[i]={type:"c",value:a.getHex()}:a&&a.isVector2?t.uniforms[i]={type:"v2",value:a.toArray()}:a&&a.isVector3?t.uniforms[i]={type:"v3",value:a.toArray()}:a&&a.isVector4?t.uniforms[i]={type:"v4",value:a.toArray()}:a&&a.isMatrix3?t.uniforms[i]={type:"m3",value:a.toArray()}:a&&a.isMatrix4?t.uniforms[i]={type:"m4",value:a.toArray()}:t.uniforms[i]={value:a}}Object.keys(this.defines).length>0&&(t.defines=this.defines),t.vertexShader=this.vertexShader,t.fragmentShader=this.fragmentShader,t.lights=this.lights,t.clipping=this.clipping;let n={};for(let i in this.extensions)this.extensions[i]===!0&&(n[i]=!0);return Object.keys(n).length>0&&(t.extensions=n),t}fromJSON(e,t){if(super.fromJSON(e,t),e.uniforms!==void 0)for(let n in e.uniforms){let i=e.uniforms[n];switch(this.uniforms[n]={},i.type){case"t":this.uniforms[n].value=t[i.value]||null;break;case"c":this.uniforms[n].value=new Le().setHex(i.value);break;case"v2":this.uniforms[n].value=new Be().fromArray(i.value);break;case"v3":this.uniforms[n].value=new B().fromArray(i.value);break;case"v4":this.uniforms[n].value=new Ot().fromArray(i.value);break;case"m3":this.uniforms[n].value=new xt().fromArray(i.value);break;case"m4":this.uniforms[n].value=new ft().fromArray(i.value);break;default:this.uniforms[n].value=i.value}}if(e.defines!==void 0&&(this.defines=e.defines),e.vertexShader!==void 0&&(this.vertexShader=e.vertexShader),e.fragmentShader!==void 0&&(this.fragmentShader=e.fragmentShader),e.glslVersion!==void 0&&(this.glslVersion=e.glslVersion),e.extensions!==void 0)for(let n in e.extensions)this.extensions[n]=e.extensions[n];return e.lights!==void 0&&(this.lights=e.lights),e.clipping!==void 0&&(this.clipping=e.clipping),this}},Lr=class extends yt{constructor(e){super(e),this.isRawShaderMaterial=!0,this.type="RawShaderMaterial"}},zn=class extends On{constructor(e){super(),this.isMeshStandardMaterial=!0,this.type="MeshStandardMaterial",this.defines={STANDARD:""},this.color=new Le(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Le(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=io,this.normalScale=new Be(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new kn,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.defines={STANDARD:""},this.color.copy(e.color),this.roughness=e.roughness,this.metalness=e.metalness,this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.roughnessMap=e.roughnessMap,this.metalnessMap=e.metalnessMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.envMapIntensity=e.envMapIntensity,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},Bn=class extends zn{constructor(e){super(),this.isMeshPhysicalMaterial=!0,this.defines={STANDARD:"",PHYSICAL:""},this.type="MeshPhysicalMaterial",this.anisotropyRotation=0,this.anisotropyMap=null,this.clearcoatMap=null,this.clearcoatRoughness=0,this.clearcoatRoughnessMap=null,this.clearcoatNormalScale=new Be(1,1),this.clearcoatNormalMap=null,this.ior=1.5,Object.defineProperty(this,"reflectivity",{get:function(){return At(2.5*(this.ior-1)/(this.ior+1),0,1)},set:function(t){this.ior=(1+.4*t)/(1-.4*t)}}),this.iridescenceMap=null,this.iridescenceIOR=1.3,this.iridescenceThicknessRange=[100,400],this.iridescenceThicknessMap=null,this.sheenColor=new Le(0),this.sheenColorMap=null,this.sheenRoughness=1,this.sheenRoughnessMap=null,this.transmissionMap=null,this.thickness=0,this.thicknessMap=null,this.attenuationDistance=1/0,this.attenuationColor=new Le(1,1,1),this.specularIntensity=1,this.specularIntensityMap=null,this.specularColor=new Le(1,1,1),this.specularColorMap=null,this._anisotropy=0,this._clearcoat=0,this._dispersion=0,this._iridescence=0,this._retroreflectivity=0,this._sheen=0,this._transmission=0,this.setValues(e)}get anisotropy(){return this._anisotropy}set anisotropy(e){this._anisotropy>0!=e>0&&this.version++,this._anisotropy=e}get clearcoat(){return this._clearcoat}set clearcoat(e){this._clearcoat>0!=e>0&&this.version++,this._clearcoat=e}get iridescence(){return this._iridescence}set iridescence(e){this._iridescence>0!=e>0&&this.version++,this._iridescence=e}get dispersion(){return this._dispersion}set dispersion(e){this._dispersion>0!=e>0&&this.version++,this._dispersion=e}get retroreflectivity(){return this._retroreflectivity}set retroreflectivity(e){this._retroreflectivity>0!=e>0&&this.version++,this._retroreflectivity=e}get sheen(){return this._sheen}set sheen(e){this._sheen>0!=e>0&&this.version++,this._sheen=e}get transmission(){return this._transmission}set transmission(e){this._transmission>0!=e>0&&this.version++,this._transmission=e}copy(e){return super.copy(e),this.defines={STANDARD:"",PHYSICAL:""},this.anisotropy=e.anisotropy,this.anisotropyRotation=e.anisotropyRotation,this.anisotropyMap=e.anisotropyMap,this.clearcoat=e.clearcoat,this.clearcoatMap=e.clearcoatMap,this.clearcoatRoughness=e.clearcoatRoughness,this.clearcoatRoughnessMap=e.clearcoatRoughnessMap,this.clearcoatNormalMap=e.clearcoatNormalMap,this.clearcoatNormalScale.copy(e.clearcoatNormalScale),this.dispersion=e.dispersion,this.ior=e.ior,this.iridescence=e.iridescence,this.iridescenceMap=e.iridescenceMap,this.iridescenceIOR=e.iridescenceIOR,this.iridescenceThicknessRange=[...e.iridescenceThicknessRange],this.iridescenceThicknessMap=e.iridescenceThicknessMap,this.retroreflectivity=e.retroreflectivity,this.sheen=e.sheen,this.sheenColor.copy(e.sheenColor),this.sheenColorMap=e.sheenColorMap,this.sheenRoughness=e.sheenRoughness,this.sheenRoughnessMap=e.sheenRoughnessMap,this.transmission=e.transmission,this.transmissionMap=e.transmissionMap,this.thickness=e.thickness,this.thicknessMap=e.thicknessMap,this.attenuationDistance=e.attenuationDistance,this.attenuationColor.copy(e.attenuationColor),this.specularIntensity=e.specularIntensity,this.specularIntensityMap=e.specularIntensityMap,this.specularColor.copy(e.specularColor),this.specularColorMap=e.specularColorMap,this}};var Ia=class extends On{constructor(e){super(),this.isMeshLambertMaterial=!0,this.type="MeshLambertMaterial",this.color=new Le(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Le(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=io,this.normalScale=new Be(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new kn,this.combine=Al,this.reflectivity=1,this.envMapIntensity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.envMapIntensity=e.envMapIntensity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},ml=class extends On{constructor(e){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=Pf,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(e)}copy(e){return super.copy(e),this.depthPacking=e.depthPacking,this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this}},gl=class extends On{constructor(e){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(e)}copy(e){return super.copy(e),this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this}};function ls(s,e){return!s||s.constructor===e?s:typeof e.BYTES_PER_ELEMENT=="number"?new e(s):Array.prototype.slice.call(s)}function Ko(s){return s!==void 0&&s.inTangents!==void 0&&s.outTangents!==void 0}function Og(s){function e(i,r){return s[i]-s[r]}let t=s.length,n=new Array(t);for(let i=0;i!==t;++i)n[i]=i;return n.sort(e),n}function jd(s,e,t){let n=s.length,i=new s.constructor(n);for(let r=0,a=0;a!==n;++r){let o=t[r]*e;for(let l=0;l!==e;++l)i[a++]=s[o+l]}return i}function zg(s,e,t,n){let i=1,r=s[0];for(;r!==void 0&&r[n]===void 0;)r=s[i++];if(r===void 0)return;let a=r[n];if(a!==void 0)if(Array.isArray(a))do a=r[n],a!==void 0&&(e.push(r.time),t.push(...a)),r=s[i++];while(r!==void 0);else if(a.toArray!==void 0)do a=r[n],a!==void 0&&(e.push(r.time),a.toArray(t,t.length)),r=s[i++];while(r!==void 0);else do a=r[n],a!==void 0&&(e.push(r.time),t.push(a)),r=s[i++];while(r!==void 0)}var Ii=class{constructor(e,t,n,i){this.parameterPositions=e,this._cachedIndex=0,this.resultBuffer=i!==void 0?i:new t.constructor(n),this.sampleValues=t,this.valueSize=n,this.settings=null,this.DefaultSettings_={}}evaluate(e){let t=this.parameterPositions,n=this._cachedIndex,i=t[n],r=t[n-1];e:{t:{let a;n:{i:if(!(e<i)){for(let o=n+2;;){if(i===void 0){if(e<r)break i;return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}if(n===o)break;if(r=i,i=t[++n],e<i)break t}a=t.length;break n}if(!(e>=r)){let o=t[1];e<o&&(n=2,r=o);for(let l=n-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(n===l)break;if(i=r,r=t[--n-1],e>=r)break t}a=n,n=0;break n}break e}for(;n<a;){let o=n+a>>>1;e<t[o]?a=o:n=o+1}if(i=t[n],r=t[n-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===void 0)return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}this._cachedIndex=n,this.intervalChanged_(n,r,i)}return this.interpolate_(n,r,e,i)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i;for(let a=0;a!==i;++a)t[a]=n[r+a];return t}interpolate_(){throw new Error("THREE.Interpolant: Call to abstract method.")}intervalChanged_(){}},xl=class extends Ii{constructor(e,t,n,i){super(e,t,n,i),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:Ps,endingEnd:Ps}}intervalChanged_(e,t,n){let i=this.parameterPositions,r=e-2,a=e+1,o=i[r],l=i[a];if(o===void 0)switch(this.getSettings_().endingStart){case Is:r=e,o=2*t-n;break;case ma:r=i.length-2,o=t+i[r]-i[r+1];break;default:r=e,o=n}if(l===void 0)switch(this.getSettings_().endingEnd){case Is:a=e,l=2*n-t;break;case ma:a=1,l=n+i[1]-i[0];break;default:a=e-1,l=t}let c=(n-t)*.5,h=this.valueSize;this._weightPrev=c/(t-o),this._weightNext=c/(l-n),this._offsetPrev=r*h,this._offsetNext=a*h}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=this._offsetPrev,u=this._offsetNext,d=this._weightPrev,p=this._weightNext,x=(n-t)/(i-t),v=x*x,m=v*x,f=-d*m+2*d*v-d*x,y=(1+d)*m+(-1.5-2*d)*v+(-.5+d)*x+1,T=(-1-p)*m+(1.5+p)*v+.5*x,g=p*m-p*v;for(let _=0;_!==o;++_)r[_]=f*a[h+_]+y*a[c+_]+T*a[l+_]+g*a[u+_];return r}},Da=class extends Ii{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=(n-t)/(i-t),u=1-h;for(let d=0;d!==o;++d)r[d]=a[c+d]*u+a[l+d]*h;return r}},bl=class extends Ii{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e){return this.copySampleValue_(e-1)}},vl=class extends Ii{interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=this.inTangents,u=this.outTangents;if(!h||!u){let x=(n-t)/(i-t),v=1-x;for(let m=0;m!==o;++m)r[m]=a[c+m]*v+a[l+m]*x;return r}let d=o*2,p=e-1;for(let x=0;x!==o;++x){let v=a[c+x],m=a[l+x],f=p*d+x*2,y=u[f],T=u[f+1],g=e*d+x*2,_=h[g],w=h[g+1],A=Bg(n,t,y,_,i);r[x]=Wf(A,v,T,w,m)}return r}};function Wf(s,e,t,n,i){let r=1-s;return r*r*r*e+3*r*r*s*t+3*r*s*s*n+s*s*s*i}function kg(s,e,t,n,i){let r=1-s;return 3*r*r*(t-e)+6*r*s*(n-t)+3*s*s*(i-n)}function Bg(s,e,t,n,i){let r=(s-e)/(i-e);for(let a=0;a<8;a++){let o=Wf(r,e,t,n,i)-s;if(Math.abs(o)<1e-10)break;let l=kg(r,e,t,n,i);if(Math.abs(l)<1e-10)break;r=Math.max(0,Math.min(1,r-o/l))}return r}var Hn=class{constructor(e,t,n,i){if(e===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(t===void 0||t.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+e);this.name=e,this.times=ls(t,this.TimeBufferType),this.values=ls(n,this.ValueBufferType),this.setInterpolation(i||this.DefaultInterpolation)}static toJSON(e){let t=e.constructor,n;if(t.toJSON!==this.toJSON)n=t.toJSON(e);else{n={name:e.name,times:ls(e.times,Array),values:ls(e.values,Array)};let i=e.getInterpolation();i!==e.DefaultInterpolation&&(n.interpolation=i),Ko(e.settings)&&(n.settings={inTangents:ls(e.settings.inTangents,Array),outTangents:ls(e.settings.outTangents,Array)})}return n.type=e.ValueTypeName,n}InterpolantFactoryMethodDiscrete(e){return new bl(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodLinear(e){return new Da(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodSmooth(e){return new xl(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodBezier(e){let t=new vl(this.times,this.values,this.getValueSize(),e);return this.settings&&(t.inTangents=this.settings.inTangents,t.outTangents=this.settings.outTangents),t}setInterpolation(e){let t;switch(e){case Ls:t=this.InterpolantFactoryMethodDiscrete;break;case Fs:t=this.InterpolantFactoryMethodLinear;break;case Xo:t=this.InterpolantFactoryMethodSmooth;break;case vh:t=this.InterpolantFactoryMethodBezier;break}if(t===void 0){let n="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(e!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(n);return rt("KeyframeTrack:",n),this}return this.createInterpolant=t,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return Ls;case this.InterpolantFactoryMethodLinear:return Fs;case this.InterpolantFactoryMethodSmooth:return Xo;case this.InterpolantFactoryMethodBezier:return vh}}getValueSize(){return this.values.length/this.times.length}shift(e){if(e!==0){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]+=e}return this}scale(e){if(e!==1){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]*=e;Ko(this.settings)&&(Kd(this.settings.inTangents,e),Kd(this.settings.outTangents,e))}return this}trim(e,t){let n=this.times,i=n.length,r=0,a=i-1;for(;r!==i&&n[r]<e;)++r;for(;a!==-1&&n[a]>t;)--a;if(++a,r!==0||a!==i){r>=a&&(a=Math.max(a,1),r=a-1);let o=this.getValueSize();this.times=n.slice(r,a),this.values=this.values.slice(r*o,a*o)}return this}validate(){let e=!0,t=this.getValueSize();t-Math.floor(t)!==0&&(pt("KeyframeTrack: Invalid value size in track.",this),e=!1);let n=this.times,i=this.values,r=n.length;r===0&&(pt("KeyframeTrack: Track is empty.",this),e=!1);let a=null;for(let o=0;o!==r;o++){let l=n[o];if(typeof l=="number"&&isNaN(l)){pt("KeyframeTrack: Time is not a valid number.",this,o,l),e=!1;break}if(a!==null&&a>l){pt("KeyframeTrack: Out of order keys.",this,o,l,a),e=!1;break}a=l}if(i!==void 0&&Vm(i))for(let o=0,l=i.length;o!==l;++o){let c=i[o];if(isNaN(c)){pt("KeyframeTrack: Value is not a valid number.",this,o,c),e=!1;break}}return e}optimize(){let e=this.times.slice(),t=this.values.slice(),n=this.getValueSize(),i=this.getInterpolation()===Xo,r=e.length-1,a=1;for(let o=1;o<r;++o){let l=!1,c=e[o],h=e[o+1];if(c!==h&&(o!==1||c!==e[0]))if(i)l=!0;else{let u=o*n,d=u-n,p=u+n;for(let x=0;x!==n;++x){let v=t[u+x];if(v!==t[d+x]||v!==t[p+x]){l=!0;break}}}if(l){if(o!==a){e[a]=e[o];let u=o*n,d=a*n;for(let p=0;p!==n;++p)t[d+p]=t[u+p]}++a}}if(r>0){e[a]=e[r];for(let o=r*n,l=a*n,c=0;c!==n;++c)t[l+c]=t[o+c];++a}return a!==e.length?(this.times=e.slice(0,a),this.values=t.slice(0,a*n)):(this.times=e,this.values=t),this}clone(){let e=this.times.slice(),t=this.values.slice(),n=this.constructor,i=new n(this.name,e,t);return i.createInterpolant=this.createInterpolant,Ko(this.settings)&&(i.settings={inTangents:this.settings.inTangents.slice(),outTangents:this.settings.outTangents.slice()}),i}};function Kd(s,e){for(let t=0,n=s.length;t!==n;t+=2)s[t]*=e}Hn.prototype.ValueTypeName="";Hn.prototype.TimeBufferType=Float32Array;Hn.prototype.ValueBufferType=Float32Array;Hn.prototype.DefaultInterpolation=Fs;var Ki=class extends Hn{constructor(e,t,n){super(e,t,n)}};Ki.prototype.ValueTypeName="bool";Ki.prototype.ValueBufferType=Array;Ki.prototype.DefaultInterpolation=Ls;Ki.prototype.InterpolantFactoryMethodLinear=void 0;Ki.prototype.InterpolantFactoryMethodSmooth=void 0;var La=class extends Hn{constructor(e,t,n,i){super(e,t,n,i)}};La.prototype.ValueTypeName="color";var Yi=class extends Hn{constructor(e,t,n,i){super(e,t,n,i)}};Yi.prototype.ValueTypeName="number";var _l=class extends Ii{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=(n-t)/(i-t),c=e*o;for(let h=c+o;c!==h;c+=4)Qt.slerpFlat(r,0,a,c-o,a,c,l);return r}},Ji=class extends Hn{constructor(e,t,n,i){super(e,t,n,i)}InterpolantFactoryMethodLinear(e){return new _l(this.times,this.values,this.getValueSize(),e)}};Ji.prototype.ValueTypeName="quaternion";Ji.prototype.InterpolantFactoryMethodSmooth=void 0;var Zi=class extends Hn{constructor(e,t,n){super(e,t,n)}};Zi.prototype.ValueTypeName="string";Zi.prototype.ValueBufferType=Array;Zi.prototype.DefaultInterpolation=Ls;Zi.prototype.InterpolantFactoryMethodLinear=void 0;Zi.prototype.InterpolantFactoryMethodSmooth=void 0;var ps=class extends Hn{constructor(e,t,n,i){super(e,t,n,i)}};ps.prototype.ValueTypeName="vector";var Bs=class{constructor(e="",t=-1,n=[],i=mc){this.name=e,this.tracks=n,this.duration=t,this.blendMode=i,this.uuid=di(),this.userData={},this.duration<0&&this.resetDuration()}static parse(e){let t=[],n=e.tracks,i=1/(e.fps||1);for(let a=0,o=n.length;a!==o;++a)t.push(Gg(n[a]).scale(i));let r=new this(e.name,e.duration,t,e.blendMode);return r.uuid=e.uuid,r.userData=JSON.parse(e.userData||"{}"),r}static toJSON(e){let t=[],n=e.tracks,i={name:e.name,duration:e.duration,tracks:t,uuid:e.uuid,blendMode:e.blendMode,userData:JSON.stringify(e.userData)};for(let r=0,a=n.length;r!==a;++r)t.push(Hn.toJSON(n[r]));return i}static CreateFromMorphTargetSequence(e,t,n,i){let r=t.length,a=[];for(let o=0;o<r;o++){let l=[],c=[];l.push((o+r-1)%r,o,(o+1)%r),c.push(0,1,0);let h=Og(l);l=jd(l,1,h),c=jd(c,1,h),!i&&l[0]===0&&(l.push(r),c.push(c[0])),a.push(new Yi(".morphTargetInfluences["+t[o].name+"]",l,c).scale(1/n))}return new this(e,-1,a)}static findByName(e,t){let n=e;if(!Array.isArray(e)){let i=e;n=i.geometry&&i.geometry.animations||i.animations}for(let i=0;i<n.length;i++)if(n[i].name===t)return n[i];return null}static CreateClipsFromMorphTargetSequences(e,t,n){let i={},r=/^([\w-]*?)([\d]+)$/;for(let o=0,l=e.length;o<l;o++){let c=e[o],h=c.name.match(r);if(h&&h.length>1){let u=h[1],d=i[u];d||(i[u]=d=[]),d.push(c)}}let a=[];for(let o in i)a.push(this.CreateFromMorphTargetSequence(o,i[o],t,n));return a}resetDuration(){let e=this.tracks,t=0;for(let n=0,i=e.length;n!==i;++n){let r=this.tracks[n];t=Math.max(t,r.times[r.times.length-1])}return this.duration=t,this}trim(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].trim(0,this.duration);return this}validate(){let e=!0;for(let t=0;t<this.tracks.length;t++)e=e&&this.tracks[t].validate();return e}optimize(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].optimize();return this}clone(){let e=[];for(let n=0;n<this.tracks.length;n++)e.push(this.tracks[n].clone());let t=new this.constructor(this.name,this.duration,e,this.blendMode);return t.userData=JSON.parse(JSON.stringify(this.userData)),t}toJSON(){return this.constructor.toJSON(this)}};function Hg(s){switch(s.toLowerCase()){case"scalar":case"double":case"float":case"number":case"integer":return Yi;case"vector":case"vector2":case"vector3":case"vector4":return ps;case"color":return La;case"quaternion":return Ji;case"bool":case"boolean":return Ki;case"string":return Zi}throw new Error("THREE.KeyframeTrack: Unsupported typeName: "+s)}function Gg(s){if(s.type===void 0)throw new Error("THREE.KeyframeTrack: track type undefined, can not parse");let e=Hg(s.type);if(s.times===void 0){let n=[],i=[];zg(s.keys,n,i,"value"),s.times=n,s.values=i}let t;return e.parse!==void 0?t=e.parse(s):t=new e(s.name,s.times,s.values,s.interpolation),Ko(s.settings)&&(t.settings={inTangents:ls(s.settings.inTangents,Float32Array),outTangents:ls(s.settings.outTangents,Float32Array)}),t}var Ei={enabled:!1,files:{},add:function(s,e){this.enabled!==!1&&(Yd(s)||(this.files[s]=e))},get:function(s){if(this.enabled!==!1&&!Yd(s))return this.files[s]},remove:function(s){delete this.files[s]},clear:function(){this.files={}}};function Yd(s){try{let e=s.slice(s.indexOf(":")+1);return new URL(e).protocol==="blob:"}catch{return!1}}var yl=class{constructor(e,t,n){let i=this,r=!1,a=0,o=0,l,c=[];this.onStart=void 0,this.onLoad=e,this.onProgress=t,this.onError=n,this._abortController=null,this.itemStart=function(h){o++,r===!1&&i.onStart!==void 0&&i.onStart(h,a,o),r=!0},this.itemEnd=function(h){a++,i.onProgress!==void 0&&i.onProgress(h,a,o),a===o&&(r=!1,i.onLoad!==void 0&&i.onLoad())},this.itemError=function(h){i.onError!==void 0&&i.onError(h)},this.resolveURL=function(h){return h=h.normalize("NFC"),l?l(h):h},this.setURLModifier=function(h){return l=h,this},this.addHandler=function(h,u){return c.push(h,u),this},this.removeHandler=function(h){let u=c.indexOf(h);return u!==-1&&c.splice(u,2),this},this.getHandler=function(h){for(let u=0,d=c.length;u<d;u+=2){let p=c[u],x=c[u+1];if(p.global&&(p.lastIndex=0),p.test(h))return x}return null},this.abort=function(){return this.abortController.abort(),this._abortController=null,this}}get abortController(){return this._abortController||(this._abortController=new AbortController),this._abortController}},qf=new yl,Di=class{constructor(e){this.manager=e!==void 0?e:qf,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}load(){}loadAsync(e,t){let n=this;return new Promise(function(i,r){n.load(e,i,t,r)})}parse(){}setCrossOrigin(e){return this.crossOrigin=e,this}setWithCredentials(e){return this.withCredentials=e,this}setPath(e){return this.path=e,this}setResourcePath(e){return this.resourcePath=e,this}setRequestHeader(e){return this.requestHeader=e,this}abort(){return this}};Di.DEFAULT_MATERIAL_NAME="__DEFAULT";var qi={},Mh=class extends Error{constructor(e,t){super(e),this.response=t}},Fr=class extends Di{constructor(e){super(e),this.mimeType="",this.responseType="",this._abortController=new AbortController}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=Ei.get(`file:${e}`);if(r!==void 0){this.manager.itemStart(e),setTimeout(()=>{t&&t(r),this.manager.itemEnd(e)},0);return}if(qi[e]!==void 0){qi[e].push({onLoad:t,onProgress:n,onError:i});return}qi[e]=[],qi[e].push({onLoad:t,onProgress:n,onError:i});let a=new Request(e,{headers:new Headers(this.requestHeader),credentials:this.withCredentials?"include":"same-origin",signal:typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal}),o=this.mimeType,l=this.responseType;fetch(a).then(c=>{if(c.status===200||c.status===0){if(c.status===0&&rt("FileLoader: HTTP Status 0 received."),typeof ReadableStream>"u"||c.body===void 0||c.body.getReader===void 0)return c;let h=qi[e],u=c.body.getReader(),d=c.headers.get("X-File-Size")||c.headers.get("Content-Length"),p=d?parseInt(d):0,x=p!==0,v=0,m=new ReadableStream({start(f){y();function y(){u.read().then(({done:T,value:g})=>{if(T)f.close();else{v+=g.byteLength;let _=new ProgressEvent("progress",{lengthComputable:x,loaded:v,total:p});for(let w=0,A=h.length;w<A;w++){let b=h[w];b.onProgress&&b.onProgress(_)}f.enqueue(g),y()}},T=>{f.error(T)})}}});return new Response(m)}else throw new Mh(`fetch for "${c.url}" responded with ${c.status}: ${c.statusText}`,c)}).then(c=>{switch(l){case"arraybuffer":return c.arrayBuffer();case"blob":return c.blob();case"document":return c.text().then(h=>new DOMParser().parseFromString(h,o));case"json":return c.json();default:if(o==="")return c.text();{let u=/charset="?([^;"\s]*)"?/i.exec(o),d=u&&u[1]?u[1].toLowerCase():void 0,p=new TextDecoder(d);return c.arrayBuffer().then(x=>p.decode(x))}}}).then(c=>{Ei.add(`file:${e}`,c);let h=qi[e];delete qi[e];for(let u=0,d=h.length;u<d;u++){let p=h[u];p.onLoad&&p.onLoad(c)}}).catch(c=>{let h=qi[e];if(h===void 0)throw this.manager.itemError(e),c;delete qi[e];for(let u=0,d=h.length;u<d;u++){let p=h[u];p.onError&&p.onError(c)}this.manager.itemError(e)}).finally(()=>{this.manager.itemEnd(e)}),this.manager.itemStart(e)}setResponseType(e){return this.responseType=e,this}setMimeType(e){return this.mimeType=e,this}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var dr=new WeakMap,Ml=class extends Di{constructor(e){super(e)}load(e,t,n,i){this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,a=Ei.get(`image:${e}`);if(a!==void 0){if(a.complete===!0)r.manager.itemStart(e),setTimeout(function(){t&&t(a),r.manager.itemEnd(e)},0);else{let u=dr.get(a);u===void 0&&(u=[],dr.set(a,u)),u.push({onLoad:t,onError:i})}return a}let o=vr("img");function l(){h(),t&&t(this);let u=dr.get(this)||[];for(let d=0;d<u.length;d++){let p=u[d];p.onLoad&&p.onLoad(this)}dr.delete(this),r.manager.itemEnd(e)}function c(u){h(),i&&i(u),Ei.remove(`image:${e}`);let d=dr.get(this)||[];for(let p=0;p<d.length;p++){let x=d[p];x.onError&&x.onError(u)}dr.delete(this),r.manager.itemError(e),r.manager.itemEnd(e)}function h(){o.removeEventListener("load",l,!1),o.removeEventListener("error",c,!1)}return o.addEventListener("load",l,!1),o.addEventListener("error",c,!1),e.slice(0,5)!=="data:"&&this.crossOrigin!==void 0&&(o.crossOrigin=this.crossOrigin),Ei.add(`image:${e}`,o),r.manager.itemStart(e),o.src=e,o}};var Fa=class extends Di{constructor(e){super(e)}load(e,t,n,i){let r=new ln,a=new Ml(this.manager);return a.setCrossOrigin(this.crossOrigin),a.setPath(this.path),a.load(e,function(o){r.image=o,r.needsUpdate=!0,t!==void 0&&t(r)},n,i),r}},Hs=class extends Wt{constructor(e,t=1){super(),this.isLight=!0,this.type="Light",this.color=new Le(e),this.intensity=t}copy(e,t){return super.copy(e,t),this.color.copy(e.color),this.intensity=e.intensity,this}toJSON(e){let t=super.toJSON(e);return t.object.color=this.color.getHex(),t.object.intensity=this.intensity,t}},Na=class extends Hs{constructor(e,t,n){super(e,n),this.isHemisphereLight=!0,this.type="HemisphereLight",this.position.copy(Wt.DEFAULT_UP),this.updateMatrix(),this.groundColor=new Le(t)}copy(e,t){return super.copy(e,t),this.groundColor.copy(e.groundColor),this}toJSON(e){let t=super.toJSON(e);return t.object.groundColor=this.groundColor.getHex(),t}},gh=new ft,Jd=new B,Zd=new B,Nr=class{constructor(e){this.camera=e,this.intensity=1,this.bias=0,this.biasNode=null,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new Be(512,512),this.mapType=Gn,this.map=null,this.mapPass=null,this.matrix=new ft,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new Ar,this._frameExtents=new Be(1,1),this._viewportCount=1,this._viewports=[new Ot(0,0,1,1)]}getViewportCount(){return this._viewportCount}getCamera(){return this.camera}getFrustum(){return this._frustum}updateMatrices(e){let t=this.camera;Jd.setFromMatrixPosition(e.matrixWorld),t.position.copy(Jd),Zd.setFromMatrixPosition(e.target.matrixWorld),t.lookAt(Zd),t.updateMatrixWorld(),this._updateMatrix(t,this.matrix,this._frustum)}_updateMatrix(e,t,n,i){gh.multiplyMatrices(e.projectionMatrix,e.matrixWorldInverse),n.setFromProjectionMatrix(gh,e.coordinateSystem,e.reversedDepth);let r=this._frameExtents,a=i?i.z/r.x:1,o=i?i.w/r.y:1,l=i?i.x/r.x:0,c=i?i.y/r.y:0;e.coordinateSystem===br||e.reversedDepth?t.set(.5*a,0,0,.5*a+l,0,.5*o,0,.5*o+c,0,0,1,0,0,0,0,1):t.set(.5*a,0,0,.5*a+l,0,.5*o,0,.5*o+c,0,0,.5,.5,0,0,0,1),t.multiply(gh)}getViewport(e){return this._viewports[e]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(e){return this.camera=e.camera.clone(),this.intensity=e.intensity,this.bias=e.bias,this.radius=e.radius,this.autoUpdate=e.autoUpdate,this.needsUpdate=e.needsUpdate,this.normalBias=e.normalBias,this.blurSamples=e.blurSamples,this.mapSize.copy(e.mapSize),this.biasNode=e.biasNode,this}clone(){return new this.constructor().copy(this)}toJSON(){let e={};return e.intensity=this.intensity,e.bias=this.bias,e.normalBias=this.normalBias,e.radius=this.radius,e.blurSamples=this.blurSamples,e.mapSize=this.mapSize.toArray(),e.camera=this.camera.toJSON(!1).object,delete e.camera.matrix,e}},Wo=new B,qo=new Qt,wi=new B,Ua=class extends Wt{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new ft,this.projectionMatrix=new ft,this.projectionMatrixInverse=new ft,this.coordinateSystem=ui,this._reversedDepth=!1}get reversedDepth(){return this._reversedDepth}copy(e,t){return super.copy(e,t),this.matrixWorldInverse.copy(e.matrixWorldInverse),this.projectionMatrix.copy(e.projectionMatrix),this.projectionMatrixInverse.copy(e.projectionMatrixInverse),this.coordinateSystem=e.coordinateSystem,this}getWorldDirection(e){return super.getWorldDirection(e).negate()}updateMatrixWorld(e){super.updateMatrixWorld(e),this.matrixWorld.decompose(Wo,qo,wi),wi.x===1&&wi.y===1&&wi.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Wo,qo,wi.set(1,1,1)).invert()}updateWorldMatrix(e,t,n=!1){super.updateWorldMatrix(e,t,n),this.matrixWorld.decompose(Wo,qo,wi),wi.x===1&&wi.y===1&&wi.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Wo,qo,wi.set(1,1,1)).invert()}clone(){return new this.constructor().copy(this)}},os=new B,$d=new Be,Qd=new Be,dn=class extends Ua{constructor(e=50,t=1,n=.1,i=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=e,this.zoom=1,this.near=n,this.far=i,this.focus=10,this.aspect=t,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.fov=e.fov,this.zoom=e.zoom,this.near=e.near,this.far=e.far,this.focus=e.focus,this.aspect=e.aspect,this.view=e.view===null?null:Object.assign({},e.view),this.filmGauge=e.filmGauge,this.filmOffset=e.filmOffset,this}setFocalLength(e){let t=.5*this.getFilmHeight()/e;this.fov=Ns*2*Math.atan(t),this.updateProjectionMatrix()}getFocalLength(){let e=Math.tan(ua*.5*this.fov);return .5*this.getFilmHeight()/e}getEffectiveFOV(){return Ns*2*Math.atan(Math.tan(ua*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(e,t,n){os.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),t.set(os.x,os.y).multiplyScalar(-e/os.z),os.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),n.set(os.x,os.y).multiplyScalar(-e/os.z)}getViewSize(e,t){return this.getViewBounds(e,$d,Qd),t.subVectors(Qd,$d)}setViewOffset(e,t,n,i,r,a){this.aspect=e/t,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=a,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=this.near,t=e*Math.tan(ua*.5*this.fov)/this.zoom,n=2*t,i=this.aspect*n,r=-.5*i,a=this.view;if(this.view!==null&&this.view.enabled){let l=a.fullWidth,c=a.fullHeight;r+=a.offsetX*i/l,t-=a.offsetY*n/c,i*=a.width/l,n*=a.height/c}let o=this.filmOffset;o!==0&&(r+=e*o/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+i,t,t-n,e,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.fov=this.fov,t.object.zoom=this.zoom,t.object.near=this.near,t.object.far=this.far,t.object.focus=this.focus,t.object.aspect=this.aspect,this.view!==null&&(t.object.view=Object.assign({},this.view)),t.object.filmGauge=this.filmGauge,t.object.filmOffset=this.filmOffset,t}},Sh=class extends Nr{constructor(){super(new dn(50,1,.5,500)),this.isSpotLightShadow=!0,this.focus=1,this.aspect=1}updateMatrices(e){let t=this.camera,n=Ns*2*e.angle*this.focus,i=this.mapSize.width/this.mapSize.height*this.aspect,r=e.distance||t.far;(n!==t.fov||i!==t.aspect||r!==t.far)&&(t.fov=n,t.aspect=i,t.far=r,t.updateProjectionMatrix()),super.updateMatrices(e)}copy(e){return super.copy(e),this.focus=e.focus,this.aspect=e.aspect,this}toJSON(){let e=super.toJSON();return e.focus=this.focus,e.aspect=this.aspect,e}},Oa=class extends Hs{constructor(e,t,n=0,i=Math.PI/3,r=0,a=2){super(e,t),this.isSpotLight=!0,this.type="SpotLight",this.position.copy(Wt.DEFAULT_UP),this.updateMatrix(),this.target=new Wt,this.distance=n,this.angle=i,this.penumbra=r,this.decay=a,this.map=null,this.shadow=new Sh}get power(){return this.intensity*Math.PI}set power(e){this.intensity=e/Math.PI}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.angle=e.angle,this.penumbra=e.penumbra,this.decay=e.decay,this.target=e.target.clone(),this.map=e.map,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.angle=this.angle,t.object.decay=this.decay,t.object.penumbra=this.penumbra,t.object.target=this.target.uuid,this.map&&this.map.isTexture&&(t.object.map=this.map.toJSON(e).uuid),t.object.shadow=this.shadow.toJSON(),t}},wh=class extends Nr{constructor(){super(new dn(90,1,.5,500)),this.isPointLightShadow=!0}},gi=class extends Hs{constructor(e,t,n=0,i=2){super(e,t),this.isPointLight=!0,this.type="PointLight",this.distance=n,this.decay=i,this.shadow=new wh}get power(){return this.intensity*4*Math.PI}set power(e){this.intensity=e/(4*Math.PI)}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.decay=e.decay,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.decay=this.decay,t.object.shadow=this.shadow.toJSON(),t}},Li=class extends Ua{constructor(e=-1,t=1,n=1,i=-1,r=.1,a=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=e,this.right=t,this.top=n,this.bottom=i,this.near=r,this.far=a,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.left=e.left,this.right=e.right,this.top=e.top,this.bottom=e.bottom,this.near=e.near,this.far=e.far,this.zoom=e.zoom,this.view=e.view===null?null:Object.assign({},e.view),this}setViewOffset(e,t,n,i,r,a){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=a,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=(this.right-this.left)/(2*this.zoom),t=(this.top-this.bottom)/(2*this.zoom),n=(this.right+this.left)/2,i=(this.top+this.bottom)/2,r=n-e,a=n+e,o=i+t,l=i-t;if(this.view!==null&&this.view.enabled){let c=(this.right-this.left)/this.view.fullWidth/this.zoom,h=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=c*this.view.offsetX,a=r+c*this.view.width,o-=h*this.view.offsetY,l=o-h*this.view.height}this.projectionMatrix.makeOrthographic(r,a,o,l,this.near,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.zoom=this.zoom,t.object.left=this.left,t.object.right=this.right,t.object.top=this.top,t.object.bottom=this.bottom,t.object.near=this.near,t.object.far=this.far,this.view!==null&&(t.object.view=Object.assign({},this.view)),t}},Eh=class extends Nr{constructor(){super(new Li(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},$i=class extends Hs{constructor(e,t){super(e,t),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(Wt.DEFAULT_UP),this.updateMatrix(),this.target=new Wt,this.shadow=new Eh}dispose(){super.dispose(),this.shadow.dispose()}copy(e){return super.copy(e),this.target=e.target.clone(),this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.shadow=this.shadow.toJSON(),t.object.target=this.target.uuid,t}};var Qi=class{static extractUrlBase(e){let t=e.lastIndexOf("/");return t===-1?"./":e.slice(0,t+1)}static resolveURL(e,t){return typeof e!="string"||e===""?"":(/^https?:\/\//i.test(t)&&/^\//.test(e)&&(t=t.replace(/(^https?:\/\/[^\/]+).*/i,"$1")),/^(https?:)?\/\//i.test(e)||/^data:.*,.*$/i.test(e)||/^blob:.*$/i.test(e)?e:t+e)}};var xh=new WeakMap,za=class extends Di{constructor(e){super(e),this.isImageBitmapLoader=!0,typeof createImageBitmap>"u"&&rt("ImageBitmapLoader: createImageBitmap() not supported."),typeof fetch>"u"&&rt("ImageBitmapLoader: fetch() not supported."),this.options={premultiplyAlpha:"none"},this._abortController=new AbortController}setOptions(e){return this.options=e,this}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,a=Ei.get(`image-bitmap:${e}`);if(a!==void 0){if(r.manager.itemStart(e),a.then){a.then(c=>{xh.has(a)===!0?(i&&i(xh.get(a)),r.manager.itemError(e),r.manager.itemEnd(e)):(t&&t(c),r.manager.itemEnd(e))});return}setTimeout(function(){t&&t(a),r.manager.itemEnd(e)},0);return}let o={};o.credentials=this.crossOrigin==="anonymous"?"same-origin":"include",o.headers=this.requestHeader,o.signal=typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal;let l=fetch(e,o).then(function(c){return c.blob()}).then(function(c){return createImageBitmap(c,Object.assign({},r.options,{colorSpaceConversion:"none"}))}).then(function(c){return Ei.add(`image-bitmap:${e}`,c),t&&t(c),r.manager.itemEnd(e),c}).catch(function(c){i&&i(c),xh.set(l,c),Ei.remove(`image-bitmap:${e}`),r.manager.itemError(e),r.manager.itemEnd(e)});Ei.add(`image-bitmap:${e}`,l),r.manager.itemStart(e)}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var fr=-90,pr=1,Sl=class extends Wt{constructor(e,t,n){super(),this.type="CubeCamera",this.renderTarget=n,this.coordinateSystem=null,this.activeMipmapLevel=0;let i=new dn(fr,pr,e,t);i.layers=this.layers,this.add(i);let r=new dn(fr,pr,e,t);r.layers=this.layers,this.add(r);let a=new dn(fr,pr,e,t);a.layers=this.layers,this.add(a);let o=new dn(fr,pr,e,t);o.layers=this.layers,this.add(o);let l=new dn(fr,pr,e,t);l.layers=this.layers,this.add(l);let c=new dn(fr,pr,e,t);c.layers=this.layers,this.add(c)}updateCoordinateSystem(){let e=this.coordinateSystem,t=this.children.concat(),[n,i,r,a,o,l]=t;for(let c of t)this.remove(c);if(e===ui)n.up.set(0,1,0),n.lookAt(1,0,0),i.up.set(0,1,0),i.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),a.up.set(0,0,1),a.lookAt(0,-1,0),o.up.set(0,1,0),o.lookAt(0,0,1),l.up.set(0,1,0),l.lookAt(0,0,-1);else if(e===br)n.up.set(0,-1,0),n.lookAt(-1,0,0),i.up.set(0,-1,0),i.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),a.up.set(0,0,-1),a.lookAt(0,-1,0),o.up.set(0,-1,0),o.lookAt(0,0,1),l.up.set(0,-1,0),l.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+e);for(let c of t)this.add(c),c.updateMatrixWorld()}update(e,t){this.parent===null&&this.updateMatrixWorld();let{renderTarget:n,activeMipmapLevel:i}=this;this.coordinateSystem!==e.coordinateSystem&&(this.coordinateSystem=e.coordinateSystem,this.updateCoordinateSystem());let[r,a,o,l,c,h]=this.children,u=e.getRenderTarget(),d=e.getActiveCubeFace(),p=e.getActiveMipmapLevel(),x=e.xr.enabled;e.xr.enabled=!1;let v=n.texture.generateMipmaps;n.texture.generateMipmaps=!1;let m=!1;e.isWebGLRenderer===!0?m=e.state.buffers.depth.getReversed():m=e.reversedDepthBuffer,e.setRenderTarget(n,0,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,r),e.setRenderTarget(n,1,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,a),e.setRenderTarget(n,2,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,o),e.setRenderTarget(n,3,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,l),e.setRenderTarget(n,4,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,c),n.texture.generateMipmaps=v,e.setRenderTarget(n,5,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,h),e.setRenderTarget(u,d,p),e.xr.enabled=x,n.texture.needsPMREMUpdate=!0}},wl=class extends dn{constructor(e=[]){super(),this.isArrayCamera=!0,this.isMultiViewCamera=!1,this.cameras=e}},ka=class{constructor(){this._previousTime=0,this._currentTime=0,this._startTime=performance.now(),this._delta=0,this._elapsed=0,this._timescale=1,this._document=null,this._pageVisibilityHandler=null}connect(e){this._document=e,e.hidden!==void 0&&(this._pageVisibilityHandler=Vg.bind(this),e.addEventListener("visibilitychange",this._pageVisibilityHandler,!1))}disconnect(){this._pageVisibilityHandler!==null&&(this._document.removeEventListener("visibilitychange",this._pageVisibilityHandler),this._pageVisibilityHandler=null),this._document=null}getDelta(){return this._delta/1e3}getElapsed(){return this._elapsed/1e3}getTimescale(){return this._timescale}setTimescale(e){return this._timescale=e,this}reset(){return this._currentTime=performance.now()-this._startTime,this}dispose(){this.disconnect()}update(e){return this._pageVisibilityHandler!==null&&this._document.hidden===!0?this._delta=0:(this._previousTime=this._currentTime,this._currentTime=(e!==void 0?e:performance.now())-this._startTime,this._delta=(this._currentTime-this._previousTime)*this._timescale,this._elapsed+=this._delta),this}};function Vg(){this._document.hidden===!1&&this.reset()}var El=class{constructor(e,t,n){this.binding=e,this.valueSize=n;let i,r,a;switch(t){case"quaternion":i=this._slerp,r=this._slerpAdditive,a=this._setAdditiveIdentityQuaternion,this.buffer=new Float64Array(n*6),this._workIndex=5;break;case"string":case"bool":i=this._select,r=this._select,a=this._setAdditiveIdentityOther,this.buffer=new Array(n*5);break;default:i=this._lerp,r=this._lerpAdditive,a=this._setAdditiveIdentityNumeric,this.buffer=new Float64Array(n*5)}this._mixBufferRegion=i,this._mixBufferRegionAdditive=r,this._setIdentity=a,this._origIndex=3,this._addIndex=4,this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,this.useCount=0,this.referenceCount=0}accumulate(e,t){let n=this.buffer,i=this.valueSize,r=e*i+i,a=this.cumulativeWeight;if(a===0){for(let o=0;o!==i;++o)n[r+o]=n[o];a=t}else{a+=t;let o=t/a;this._mixBufferRegion(n,r,0,o,i)}this.cumulativeWeight=a}accumulateAdditive(e){let t=this.buffer,n=this.valueSize,i=n*this._addIndex;this.cumulativeWeightAdditive===0&&this._setIdentity(),this._mixBufferRegionAdditive(t,i,0,e,n),this.cumulativeWeightAdditive+=e}apply(e){let t=this.valueSize,n=this.buffer,i=e*t+t,r=this.cumulativeWeight,a=this.cumulativeWeightAdditive,o=this.binding;if(this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,r<1){let l=t*this._origIndex;this._mixBufferRegion(n,i,l,1-r,t)}a>0&&this._mixBufferRegionAdditive(n,i,this._addIndex*t,1,t);for(let l=t,c=t+t;l!==c;++l)if(n[l]!==n[l+t]){o.setValue(n,i);break}}saveOriginalState(){let e=this.binding,t=this.buffer,n=this.valueSize,i=n*this._origIndex;e.getValue(t,i);for(let r=n,a=i;r!==a;++r)t[r]=t[i+r%n];this._setIdentity(),this.cumulativeWeight=0,this.cumulativeWeightAdditive=0}restoreOriginalState(){let e=this.valueSize*3;this.binding.setValue(this.buffer,e)}_setAdditiveIdentityNumeric(){let e=this._addIndex*this.valueSize,t=e+this.valueSize;for(let n=e;n<t;n++)this.buffer[n]=0}_setAdditiveIdentityQuaternion(){this._setAdditiveIdentityNumeric(),this.buffer[this._addIndex*this.valueSize+3]=1}_setAdditiveIdentityOther(){let e=this._origIndex*this.valueSize,t=this._addIndex*this.valueSize;for(let n=0;n<this.valueSize;n++)this.buffer[t+n]=this.buffer[e+n]}_select(e,t,n,i,r){if(i>=.5)for(let a=0;a!==r;++a)e[t+a]=e[n+a]}_slerp(e,t,n,i){Qt.slerpFlat(e,t,e,t,e,n,i)}_slerpAdditive(e,t,n,i,r){let a=this._workIndex*r;Qt.multiplyQuaternionsFlat(e,a,e,t,e,n),Qt.slerpFlat(e,t,e,t,e,a,i)}_lerp(e,t,n,i,r){let a=1-i;for(let o=0;o!==r;++o){let l=t+o;e[l]=e[l]*a+e[n+o]*i}}_lerpAdditive(e,t,n,i,r){for(let a=0;a!==r;++a){let o=t+a;e[o]=e[o]+e[n+a]*i}}},jh="\\[\\]\\.:\\/",Wg=new RegExp("["+jh+"]","g"),Kh="[^"+jh+"]",qg="[^"+jh.replace("\\.","")+"]",Xg=/((?:WC+[\/:])*)/.source.replace("WC",Kh),jg=/(WCOD+)?/.source.replace("WCOD",qg),Kg=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",Kh),Yg=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",Kh),Jg=new RegExp("^"+Xg+jg+Kg+Yg+"$"),Zg=["material","materials","bones","map"],Th=class{constructor(e,t,n){let i=n||$t.parseTrackName(t);this._targetGroup=e,this._bindings=e.subscribe_(t,i)}getValue(e,t){this.bind();let n=this._targetGroup.nCachedObjects_,i=this._bindings[n];i!==void 0&&i.getValue(e,t)}setValue(e,t){let n=this._bindings;for(let i=this._targetGroup.nCachedObjects_,r=n.length;i!==r;++i)n[i].setValue(e,t)}bind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].bind()}unbind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].unbind()}},$t=class s{constructor(e,t,n){this.path=t,this.parsedPath=n||s.parseTrackName(t),this.node=s.findNode(e,this.parsedPath.nodeName),this.rootNode=e,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(e,t,n){return e&&e.isAnimationObjectGroup?new s.Composite(e,t,n):new s(e,t,n)}static sanitizeNodeName(e){return e.replace(/\s/g,"_").replace(Wg,"")}static parseTrackName(e){let t=Jg.exec(e);if(t===null)throw new Error("THREE.PropertyBinding: Cannot parse trackName: "+e);let n={nodeName:t[2],objectName:t[3],objectIndex:t[4],propertyName:t[5],propertyIndex:t[6]},i=n.nodeName&&n.nodeName.lastIndexOf(".");if(i!==void 0&&i!==-1){let r=n.nodeName.substring(i+1);Zg.indexOf(r)!==-1&&(n.nodeName=n.nodeName.substring(0,i),n.objectName=r)}if(n.propertyName===null||n.propertyName.length===0)throw new Error("THREE.PropertyBinding: can not parse propertyName from trackName: "+e);return n}static findNode(e,t){if(t===void 0||t===""||t==="."||t===-1||t===e.name||t===e.uuid)return e;if(e.skeleton){let n=e.skeleton.getBoneByName(t);if(n!==void 0)return n}if(e.children){let n=function(r){for(let a=0;a<r.length;a++){let o=r[a];if(o.name===t||o.uuid===t)return o;let l=n(o.children);if(l)return l}return null},i=n(e.children);if(i)return i}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(e,t){e[t]=this.targetObject[this.propertyName]}_getValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)e[t++]=n[i]}_getValue_arrayElement(e,t){e[t]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(e,t){this.resolvedProperty.toArray(e,t)}_setValue_direct(e,t){this.targetObject[this.propertyName]=e[t]}_setValue_direct_setNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++]}_setValue_array_setNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(e,t){this.resolvedProperty[this.propertyIndex]=e[t]}_setValue_arrayElement_setNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(e,t){this.resolvedProperty.fromArray(e,t)}_setValue_fromArray_setNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(e,t){this.bind(),this.getValue(e,t)}_setValue_unbound(e,t){this.bind(),this.setValue(e,t)}bind(){let e=this.node,t=this.parsedPath,n=t.objectName,i=t.propertyName,r=t.propertyIndex;if(e||(e=s.findNode(this.rootNode,t.nodeName),this.node=e),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!e){rt("PropertyBinding: No target node found for track: "+this.path+".");return}if(n){let c=t.objectIndex;switch(n){case"materials":if(!e.material){pt("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.materials){pt("PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}e=e.material.materials;break;case"bones":if(!e.skeleton){pt("PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}e=e.skeleton.bones;for(let h=0;h<e.length;h++)if(e[h].name===c){c=h;break}break;case"map":if("map"in e){e=e.map;break}if(!e.material){pt("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.map){pt("PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}e=e.material.map;break;default:if(e[n]===void 0){pt("PropertyBinding: Can not bind to objectName of node undefined.",this);return}e=e[n]}if(c!==void 0){if(e[c]===void 0){pt("PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,e);return}e=e[c]}}let a=e[i];if(a===void 0){let c=t.nodeName;pt("PropertyBinding: Trying to update property for track: "+c+"."+i+" but it wasn't found.",e);return}let o=this.Versioning.None;this.targetObject=e,e.isMaterial===!0?o=this.Versioning.NeedsUpdate:e.isObject3D===!0&&(o=this.Versioning.MatrixWorldNeedsUpdate);let l=this.BindingType.Direct;if(r!==void 0){if(i==="morphTargetInfluences"){if(!e.geometry){pt("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!e.geometry.morphAttributes){pt("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}e.morphTargetDictionary[r]!==void 0&&(r=e.morphTargetDictionary[r])}l=this.BindingType.ArrayElement,this.resolvedProperty=a,this.propertyIndex=r}else a.fromArray!==void 0&&a.toArray!==void 0?(l=this.BindingType.HasFromToArray,this.resolvedProperty=a):Array.isArray(a)?(l=this.BindingType.EntireArray,this.resolvedProperty=a):this.propertyName=i;this.getValue=this.GetterByBindingType[l],this.setValue=this.SetterByBindingTypeAndVersioning[l][o]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};$t.Composite=Th;$t.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};$t.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};$t.prototype.GetterByBindingType=[$t.prototype._getValue_direct,$t.prototype._getValue_array,$t.prototype._getValue_arrayElement,$t.prototype._getValue_toArray];$t.prototype.SetterByBindingTypeAndVersioning=[[$t.prototype._setValue_direct,$t.prototype._setValue_direct_setNeedsUpdate,$t.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[$t.prototype._setValue_array,$t.prototype._setValue_array_setNeedsUpdate,$t.prototype._setValue_array_setMatrixWorldNeedsUpdate],[$t.prototype._setValue_arrayElement,$t.prototype._setValue_arrayElement_setNeedsUpdate,$t.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[$t.prototype._setValue_fromArray,$t.prototype._setValue_fromArray_setNeedsUpdate,$t.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var Tl=class{constructor(e,t,n=null,i=t.blendMode){this._mixer=e,this._clip=t,this._localRoot=n,this.blendMode=i;let r=t.tracks,a=r.length,o=new Array(a),l={endingStart:Ps,endingEnd:Ps};for(let c=0;c!==a;++c){let h=r[c].createInterpolant(null);o[c]=h,h.settings=l}this._interpolantSettings=l,this._interpolants=o,this._propertyBindings=new Array(a),this._cacheIndex=null,this._byClipCacheIndex=null,this._timeScaleInterpolant=null,this._restoreTimeScale=null,this._weightInterpolant=null,this.loop=Af,this._loopCount=-1,this._startTime=null,this.time=0,this.timeScale=1,this._effectiveTimeScale=1,this.weight=1,this._effectiveWeight=1,this.repetitions=1/0,this.paused=!1,this.enabled=!0,this.clampWhenFinished=!1,this.zeroSlopeAtStart=!0,this.zeroSlopeAtEnd=!0}play(){return this._mixer._activateAction(this),this}stop(){return this._mixer._deactivateAction(this),this.reset()}reset(){return this.paused=!1,this.enabled=!0,this.time=0,this._loopCount=-1,this._startTime=null,this.stopFading().stopWarping()}isRunning(){return this.enabled&&!this.paused&&this.timeScale!==0&&this._startTime===null&&this._mixer._isActiveAction(this)}isScheduled(){return this._mixer._isActiveAction(this)}startAt(e){return this._startTime=e,this}setLoop(e,t){return this.loop=e,this.repetitions=t,this}setEffectiveWeight(e){return this.weight=e,this._effectiveWeight=this.enabled?e:0,this.stopFading()}getEffectiveWeight(){return this._effectiveWeight}fadeIn(e){return this._scheduleFading(e,0,1)}fadeOut(e){return this._scheduleFading(e,1,0)}crossFadeFrom(e,t,n=!1){if(e.fadeOut(t),this.fadeIn(t),n===!0){let i=this._clip.duration,r=e._clip.duration,a=r/i,o=i/r;e._restoreTimeScale=e.timeScale,this._restoreTimeScale=this.timeScale,e.warp(1,a,t),this.warp(o,1,t)}return this}crossFadeTo(e,t,n=!1){return e.crossFadeFrom(this,t,n)}stopFading(){let e=this._weightInterpolant;return e!==null&&(this._weightInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this}setEffectiveTimeScale(e){return this.timeScale=e,this._effectiveTimeScale=this.paused?0:e,this.stopWarping()}getEffectiveTimeScale(){return this._effectiveTimeScale}setDuration(e){return this.timeScale=this._clip.duration/e,this.stopWarping()}syncWith(e){return this.time=e.time,this.timeScale=e.timeScale,this.stopWarping()}halt(e){return this.warp(this._effectiveTimeScale,0,e)}warp(e,t,n){let i=this._mixer,r=i.time,a=this.timeScale,o=this._timeScaleInterpolant;o===null&&(o=i._lendControlInterpolant(),this._timeScaleInterpolant=o);let l=o.parameterPositions,c=o.sampleValues;return l[0]=r,l[1]=r+n,c[0]=e/a,c[1]=t/a,this}stopWarping(){let e=this._timeScaleInterpolant;return e!==null&&(this._timeScaleInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this._restoreTimeScale=null,this}getMixer(){return this._mixer}getClip(){return this._clip}getRoot(){return this._localRoot||this._mixer._root}_update(e,t,n,i){if(!this.enabled){this._updateWeight(e);return}let r=this._startTime;if(r!==null){let l=(e-r)*n;l<0||n===0?t=0:(this._startTime=null,t=n*l)}t*=this._updateTimeScale(e);let a=this._updateTime(t),o=this._updateWeight(e);if(o>0){let l=this._interpolants,c=this._propertyBindings;switch(this.blendMode){case Cf:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(a),c[h].accumulateAdditive(o);break;case mc:default:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(a),c[h].accumulate(i,o)}}}_updateWeight(e){let t=0;if(this.enabled){t=this.weight;let n=this._weightInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(this.stopFading(),i===0&&(this.enabled=!1))}}return this._effectiveWeight=t,t}_updateTimeScale(e){let t=0;if(!this.paused){t=this.timeScale;let n=this._timeScaleInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(t===0?this.paused=!0:(this._restoreTimeScale!==null&&(t=this._restoreTimeScale),this.timeScale=t),this.stopWarping())}}return this._effectiveTimeScale=t,t}_updateTime(e){let t=this._clip.duration,n=this.loop,i=this.time+e,r=this._loopCount,a=n===Rf;if(e===0)return r===-1?i:a&&(r&1)===1?t-i:i;if(n===pc){r===-1&&(this._loopCount=0,this._setEndings(!0,!0,!1));e:{if(i>=t)i=t;else if(i<0)i=0;else{this.time=i;break e}this.clampWhenFinished?this.paused=!0:this.enabled=!1,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e<0?-1:1})}}else{if(r===-1&&(e>=0?(r=0,this._setEndings(!0,this.repetitions===0,a)):this._setEndings(this.repetitions===0,!0,a)),i>=t||i<0){let o=Math.floor(i/t);i-=t*o,r+=Math.abs(o);let l=this.repetitions-r;if(l<=0)this.clampWhenFinished?this.paused=!0:this.enabled=!1,i=e>0?t:0,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e>0?1:-1});else{if(l===1){let c=e<0;this._setEndings(c,!c,a)}else this._setEndings(!1,!1,a);this._loopCount=r,this.time=i,this._mixer.dispatchEvent({type:"loop",action:this,loopDelta:o})}}else this._loopCount=r,this.time=i;if(a&&(r&1)===1)return t-i}return i}_setEndings(e,t,n){let i=this._interpolantSettings;n?(i.endingStart=Is,i.endingEnd=Is):(e?i.endingStart=this.zeroSlopeAtStart?Is:Ps:i.endingStart=ma,t?i.endingEnd=this.zeroSlopeAtEnd?Is:Ps:i.endingEnd=ma)}_scheduleFading(e,t,n){let i=this._mixer,r=i.time,a=this._weightInterpolant;a===null&&(a=i._lendControlInterpolant(),this._weightInterpolant=a);let o=a.parameterPositions,l=a.sampleValues;return o[0]=r,l[0]=t,o[1]=r+e,l[1]=n,this}},$g=new Float32Array(1),Ba=class extends ei{constructor(e){super(),this._root=e,this._initMemoryManager(),this._accuIndex=0,this.time=0,this.timeScale=1,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}_bindAction(e,t){let n=e._localRoot||this._root,i=e._clip.tracks,r=i.length,a=e._propertyBindings,o=e._interpolants,l=n.uuid,c=this._bindingsByRootAndName,h=c[l];h===void 0&&(h={},c[l]=h);for(let u=0;u!==r;++u){let d=i[u],p=d.name,x=h[p];if(x!==void 0)++x.referenceCount,a[u]=x;else{if(x=a[u],x!==void 0){x._cacheIndex===null&&(++x.referenceCount,this._addInactiveBinding(x,l,p));continue}let v=t&&t._propertyBindings[u].binding.parsedPath;x=new El($t.create(n,p,v),d.ValueTypeName,d.getValueSize()),++x.referenceCount,this._addInactiveBinding(x,l,p),a[u]=x}o[u].resultBuffer=x.buffer}}_activateAction(e){if(!this._isActiveAction(e)){if(e._cacheIndex===null){let n=(e._localRoot||this._root).uuid,i=e._clip.uuid,r=this._actionsByClip[i];this._bindAction(e,r&&r.knownActions[0]),this._addInactiveAction(e,i,n)}let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];r.useCount++===0&&(this._lendBinding(r),r.saveOriginalState())}this._lendAction(e)}}_deactivateAction(e){if(this._isActiveAction(e)){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.useCount===0&&(r.restoreOriginalState(),this._takeBackBinding(r))}this._takeBackAction(e)}}_initMemoryManager(){this._actions=[],this._nActiveActions=0,this._actionsByClip={},this._bindings=[],this._nActiveBindings=0,this._bindingsByRootAndName={},this._controlInterpolants=[],this._nActiveControlInterpolants=0;let e=this;this.stats={actions:{get total(){return e._actions.length},get inUse(){return e._nActiveActions}},bindings:{get total(){return e._bindings.length},get inUse(){return e._nActiveBindings}},controlInterpolants:{get total(){return e._controlInterpolants.length},get inUse(){return e._nActiveControlInterpolants}}}}_isActiveAction(e){let t=e._cacheIndex;return t!==null&&t<this._nActiveActions}_addInactiveAction(e,t,n){let i=this._actions,r=this._actionsByClip,a=r[t];if(a===void 0)a={knownActions:[e],actionByRoot:{}},e._byClipCacheIndex=0,r[t]=a;else{let o=a.knownActions;e._byClipCacheIndex=o.length,o.push(e)}e._cacheIndex=i.length,i.push(e),a.actionByRoot[n]=e}_removeInactiveAction(e){let t=this._actions,n=t[t.length-1],i=e._cacheIndex;n._cacheIndex=i,t[i]=n,t.pop(),e._cacheIndex=null;let r=e._clip.uuid,a=this._actionsByClip,o=a[r],l=o.knownActions,c=l[l.length-1],h=e._byClipCacheIndex;c._byClipCacheIndex=h,l[h]=c,l.pop(),e._byClipCacheIndex=null;let u=o.actionByRoot,d=(e._localRoot||this._root).uuid;delete u[d],l.length===0&&delete a[r],this._removeInactiveBindingsForAction(e)}_removeInactiveBindingsForAction(e){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.referenceCount===0&&this._removeInactiveBinding(r)}}_lendAction(e){let t=this._actions,n=e._cacheIndex,i=this._nActiveActions++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackAction(e){let t=this._actions,n=e._cacheIndex,i=--this._nActiveActions,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_addInactiveBinding(e,t,n){let i=this._bindingsByRootAndName,r=this._bindings,a=i[t];a===void 0&&(a={},i[t]=a),a[n]=e,e._cacheIndex=r.length,r.push(e)}_removeInactiveBinding(e){let t=this._bindings,n=e.binding,i=n.rootNode.uuid,r=n.path,a=this._bindingsByRootAndName,o=a[i],l=t[t.length-1],c=e._cacheIndex;l._cacheIndex=c,t[c]=l,t.pop(),delete o[r],Object.keys(o).length===0&&delete a[i]}_lendBinding(e){let t=this._bindings,n=e._cacheIndex,i=this._nActiveBindings++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackBinding(e){let t=this._bindings,n=e._cacheIndex,i=--this._nActiveBindings,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_lendControlInterpolant(){let e=this._controlInterpolants,t=this._nActiveControlInterpolants++,n=e[t];return n===void 0&&(n=new Da(new Float32Array(2),new Float32Array(2),1,$g),n.__cacheIndex=t,e[t]=n),n}_takeBackControlInterpolant(e){let t=this._controlInterpolants,n=e.__cacheIndex,i=--this._nActiveControlInterpolants,r=t[i];e.__cacheIndex=i,t[i]=e,r.__cacheIndex=n,t[n]=r}clipAction(e,t,n){let i=t||this._root,r=i.uuid,a=typeof e=="string"?Bs.findByName(i,e):e,o=a!==null?a.uuid:e,l=this._actionsByClip[o],c=null;if(n===void 0&&(a!==null?n=a.blendMode:n=mc),l!==void 0){let u=l.actionByRoot[r];if(u!==void 0&&u.blendMode===n)return u;c=l.knownActions[0],a===null&&(a=c._clip)}if(a===null)return null;let h=new Tl(this,a,t,n);return this._bindAction(h,c),this._addInactiveAction(h,o,r),h}existingAction(e,t){let n=t||this._root,i=n.uuid,r=typeof e=="string"?Bs.findByName(n,e):e,a=r?r.uuid:e,o=this._actionsByClip[a];return o!==void 0&&o.actionByRoot[i]||null}stopAllAction(){let e=this._actions,t=this._nActiveActions;for(let n=t-1;n>=0;--n)e[n].stop();return this}update(e){e*=this.timeScale;let t=this._actions,n=this._nActiveActions,i=this.time+=e,r=Math.sign(e),a=this._accuIndex^=1;for(let c=0;c!==n;++c)t[c]._update(i,e,r,a);let o=this._bindings,l=this._nActiveBindings;for(let c=0;c!==l;++c)o[c].apply(a);return this}setTime(e){this.time=0;for(let t=0;t<this._actions.length;t++)this._actions[t].time=0;return this.update(e)}getRoot(){return this._root}uncacheClip(e){let t=this._actions,n=e.uuid,i=this._actionsByClip,r=i[n];if(r!==void 0){let a=r.knownActions;for(let o=0,l=a.length;o!==l;++o){let c=a[o];this._deactivateAction(c);let h=c._cacheIndex,u=t[t.length-1];c._cacheIndex=null,c._byClipCacheIndex=null,u._cacheIndex=h,t[h]=u,t.pop(),this._removeInactiveBindingsForAction(c)}delete i[n]}}uncacheRoot(e){let t=e.uuid,n=this._actionsByClip;for(let a in n){let o=n[a].actionByRoot,l=o[t];l!==void 0&&(this._deactivateAction(l),this._removeInactiveAction(l))}let i=this._bindingsByRootAndName,r=i[t];if(r!==void 0)for(let a in r){let o=r[a];o.restoreOriginalState(),this._removeInactiveBinding(o)}}uncacheAction(e,t){let n=this.existingAction(e,t);n!==null&&(this._deactivateAction(n),this._removeInactiveAction(n))}};var ef=new ft,Ha=class{constructor(e,t,n=0,i=1/0){this.ray=new Ri(e,t),this.near=n,this.far=i,this.camera=null,this.layers=new Mr,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(e,t){this.ray.set(e,t)}setFromCamera(e,t){t.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(e.x,e.y,.5).unproject(t).sub(this.ray.origin).normalize(),this.camera=t):t.isOrthographicCamera?(this.ray.origin.set(e.x,e.y,t.projectionMatrix.elements[14]).unproject(t),this.ray.direction.set(0,0,-1).transformDirection(t.matrixWorld),this.camera=t):pt("Raycaster: Unsupported camera type: "+t.type)}setFromXRController(e){return ef.identity().extractRotation(e.matrixWorld),this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(ef),this}intersectObject(e,t=!0,n=[]){return Ah(e,this,n,t),n.sort(tf),n}intersectObjects(e,t=!0,n=[]){for(let i=0,r=e.length;i<r;i++)Ah(e[i],this,n,t);return n.sort(tf),n}};function tf(s,e){return s.distance-e.distance}function Ah(s,e,t,n){let i=!0;if(s.layers.test(e.layers)&&s.raycast(e,t)===!1&&(i=!1),i===!0&&n===!0){let r=s.children;for(let a=0,o=r.length;a<o;a++)Ah(r[a],e,t,!0)}}var Ur=class{constructor(e=1,t=0,n=0){this.radius=e,this.phi=t,this.theta=n}set(e,t,n){return this.radius=e,this.phi=t,this.theta=n,this}copy(e){return this.radius=e.radius,this.phi=e.phi,this.theta=e.theta,this}makeSafe(){return this.phi=At(this.phi,1e-6,Math.PI-1e-6),this}setFromVector3(e){return this.setFromCartesianCoords(e.x,e.y,e.z)}setFromCartesianCoords(e,t,n){return this.radius=Math.sqrt(e*e+t*t+n*n),this.radius===0?(this.theta=0,this.phi=0):(this.theta=Math.atan2(e,n),this.phi=Math.acos(At(t/this.radius,-1,1))),this}clone(){return new this.constructor().copy(this)}};var Rh=class s{static{s.prototype.isMatrix2=!0}constructor(e,t,n,i){this.elements=[1,0,0,1],e!==void 0&&this.set(e,t,n,i)}identity(){return this.set(1,0,0,1),this}fromArray(e,t=0){for(let n=0;n<4;n++)this.elements[n]=e[n+t];return this}set(e,t,n,i){let r=this.elements;return r[0]=e,r[2]=t,r[1]=n,r[3]=i,this}};var Ga=class extends ei{constructor(e,t=null){super(),this.object=e,this.domElement=t,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(e){this.domElement!==null&&this.disconnect(),this.domElement=e}disconnect(){}dispose(){}update(){}};function Yh(s,e,t,n){let i=Qg(n);switch(t){case kh:return s*e;case Fl:return s*e/i.components*i.byteLength;case Nl:return s*e/i.components*i.byteLength;case vs:return s*e*2/i.components*i.byteLength;case Ul:return s*e*2/i.components*i.byteLength;case Bh:return s*e*3/i.components*i.byteLength;case Jn:return s*e*4/i.components*i.byteLength;case Ol:return s*e*4/i.components*i.byteLength;case Ja:case Za:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case $a:case Qa:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case kl:case Hl:return Math.max(s,16)*Math.max(e,8)/4;case zl:case Bl:return Math.max(s,8)*Math.max(e,8)/2;case Gl:case Vl:case ql:case Xl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case Wl:case eo:case jl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Kl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Yl:return Math.floor((s+4)/5)*Math.floor((e+3)/4)*16;case Jl:return Math.floor((s+4)/5)*Math.floor((e+4)/5)*16;case Zl:return Math.floor((s+5)/6)*Math.floor((e+4)/5)*16;case $l:return Math.floor((s+5)/6)*Math.floor((e+5)/6)*16;case Ql:return Math.floor((s+7)/8)*Math.floor((e+4)/5)*16;case ec:return Math.floor((s+7)/8)*Math.floor((e+5)/6)*16;case tc:return Math.floor((s+7)/8)*Math.floor((e+7)/8)*16;case nc:return Math.floor((s+9)/10)*Math.floor((e+4)/5)*16;case ic:return Math.floor((s+9)/10)*Math.floor((e+5)/6)*16;case sc:return Math.floor((s+9)/10)*Math.floor((e+7)/8)*16;case rc:return Math.floor((s+9)/10)*Math.floor((e+9)/10)*16;case ac:return Math.floor((s+11)/12)*Math.floor((e+9)/10)*16;case oc:return Math.floor((s+11)/12)*Math.floor((e+11)/12)*16;case lc:case cc:case hc:return Math.ceil(s/4)*Math.ceil(e/4)*16;case uc:case dc:return Math.ceil(s/4)*Math.ceil(e/4)*8;case to:case fc:return Math.ceil(s/4)*Math.ceil(e/4)*16}throw new Error(`Unable to determine texture byte length for ${t} format.`)}function Qg(s){switch(s){case Gn:case Nh:return{byteLength:1,components:1};case Br:case Uh:case pn:return{byteLength:2,components:1};case Dl:case Ll:return{byteLength:2,components:4};case vi:case Il:case Yn:return{byteLength:4,components:1};case Oh:case zh:return{byteLength:4,components:3}}throw new Error(`THREE.TextureUtils: Unknown texture type ${s}.`)}typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"186"}}));typeof window<"u"&&(window.__THREE__?rt("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="186");function fp(){let s=null,e=!1,t=null,n=null;function i(r,a){n=s.requestAnimationFrame(i),t(r,a)}return{start:function(){e!==!0&&t!==null&&s!==null&&(n=s.requestAnimationFrame(i),e=!0)},stop:function(){s!==null&&s.cancelAnimationFrame(n),e=!1},setAnimationLoop:function(r){t=r},setContext:function(r){s=r}}}function t0(s){let e=new WeakMap;function t(o,l){let c=o.array,h=o.usage,u=c.byteLength,d=s.createBuffer();s.bindBuffer(l,d),s.bufferData(l,c,h),o.onUploadCallback();let p;if(c instanceof Float32Array)p=s.FLOAT;else if(typeof Float16Array<"u"&&c instanceof Float16Array)p=s.HALF_FLOAT;else if(c instanceof Uint16Array)o.isFloat16BufferAttribute?p=s.HALF_FLOAT:p=s.UNSIGNED_SHORT;else if(c instanceof Int16Array)p=s.SHORT;else if(c instanceof Uint32Array)p=s.UNSIGNED_INT;else if(c instanceof Int32Array)p=s.INT;else if(c instanceof Int8Array)p=s.BYTE;else if(c instanceof Uint8Array)p=s.UNSIGNED_BYTE;else if(c instanceof Uint8ClampedArray)p=s.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+c);return{buffer:d,type:p,bytesPerElement:c.BYTES_PER_ELEMENT,version:o.version,size:u}}function n(o,l,c){let h=l.array,u=l.updateRanges;if(s.bindBuffer(c,o),u.length===0)s.bufferSubData(c,0,h);else{u.sort((p,x)=>p.start-x.start);let d=0;for(let p=1;p<u.length;p++){let x=u[d],v=u[p];v.start<=x.start+x.count+1?x.count=Math.max(x.count,v.start+v.count-x.start):(++d,u[d]=v)}u.length=d+1;for(let p=0,x=u.length;p<x;p++){let v=u[p];s.bufferSubData(c,v.start*h.BYTES_PER_ELEMENT,h,v.start,v.count)}l.clearUpdateRanges()}l.onUploadCallback()}function i(o){return o.isInterleavedBufferAttribute&&(o=o.data),e.get(o)}function r(o){o.isInterleavedBufferAttribute&&(o=o.data);let l=e.get(o);l&&(s.deleteBuffer(l.buffer),e.delete(o))}function a(o,l){if(o.isInterleavedBufferAttribute&&(o=o.data),o.isGLBufferAttribute){let h=e.get(o);(!h||h.version<o.version)&&e.set(o,{buffer:o.buffer,type:o.type,bytesPerElement:o.elementSize,version:o.version});return}let c=e.get(o);if(c===void 0)e.set(o,t(o,l));else if(c.version<o.version){if(c.size!==o.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");n(c.buffer,o,l),c.version=o.version}}return{get:i,remove:r,update:a}}var n0=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,i0=`#ifdef USE_ALPHAHASH
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
#endif`,s0=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,r0=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,a0=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,o0=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,l0=`#ifdef USE_AOMAP
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
#endif`,c0=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,h0=`#ifdef USE_BATCHING
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
#endif`,u0=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,d0=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,f0=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,p0=`float G_BlinnPhong_Implicit( ) {
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
} // validated`,m0=`#ifdef USE_IRIDESCENCE
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
#endif`,g0=`#ifdef USE_BUMPMAP
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
#endif`,x0=`#if NUM_CLIPPING_PLANES > 0
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
#endif`,b0=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,v0=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,_0=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,y0=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#endif`,M0=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#endif`,S0=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec4 vColor;
#endif`,w0=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
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
#endif`,E0=`#define PI 3.141592653589793
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
} // validated`,T0=`#ifdef ENVMAP_TYPE_CUBE_UV
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
#endif`,A0=`vec3 transformedNormal = objectNormal;
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
#endif`,R0=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,C0=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,P0=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	#ifdef DECODE_VIDEO_TEXTURE_EMISSIVE
		emissiveColor = sRGBTransferEOTF( emissiveColor );
	#endif
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,I0=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,D0="gl_FragColor = linearToOutputTexel( gl_FragColor );",L0=`vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferEOTF( in vec4 value ) {
	return vec4( mix( pow( value.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), value.rgb * 0.0773993808, vec3( lessThanEqual( value.rgb, vec3( 0.04045 ) ) ) ), value.a );
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,F0=`#ifdef USE_ENVMAP
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
#endif`,N0=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
#endif`,U0=`#ifdef USE_ENVMAP
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
#endif`,O0=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,z0=`#ifdef USE_ENVMAP
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
#endif`,k0=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,B0=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,H0=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,G0=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,V0=`#ifdef USE_GRADIENTMAP
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
}`,W0=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,q0=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,X0=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,j0=`uniform bool receiveShadow;
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
#include <lightprobes_pars_fragment>`,K0=`#ifdef USE_ENVMAP
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
#endif`,Y0=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,J0=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,Z0=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,$0=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,Q0=`PhysicalMaterial material;
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
#endif`,ex=`uniform sampler2D dfgLUT;
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
}`,tx=`
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
#endif`,nx=`#if defined( RE_IndirectDiffuse )
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
#endif`,ix=`#if defined( RE_IndirectDiffuse )
	#if defined( LAMBERT ) || defined( PHONG )
		irradiance += iblIrradiance;
	#endif
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,sx=`#ifdef USE_LIGHT_PROBES_GRID
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
#endif`,rx=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,ax=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,ox=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,lx=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,cx=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = sRGBTransferEOTF( sampledDiffuseColor );
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,hx=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,ux=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
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
#endif`,dx=`#if defined( USE_POINTS_UV )
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
#endif`,fx=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,px=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,mx=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,gx=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,xx=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,bx=`#ifdef USE_MORPHTARGETS
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
#endif`,vx=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,_x=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
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
vec3 nonPerturbedNormal = normal;`,yx=`#ifdef USE_NORMALMAP_OBJECTSPACE
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
#endif`,Mx=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,Sx=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,wx=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
		#ifdef FLIP_SIDED
			vBitangent = - vBitangent;
		#endif
	#endif
#endif`,Ex=`#ifdef USE_NORMALMAP
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
#endif`,Tx=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,Ax=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,Rx=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,Cx=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,Px=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,Ix=`vec3 packNormalToRGB( const in vec3 normal ) {
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
}`,Dx=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,Lx=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,Fx=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,Nx=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,Ux=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,Ox=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,zx=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,kx=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,Bx=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_SUN_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
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
#endif`,Hx=`float getShadowMask() {
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
}`,Gx=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,Vx=`#ifdef USE_SKINNING
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
#endif`,Wx=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,qx=`#ifdef USE_SKINNING
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
#endif`,Xx=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,jx=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,Kx=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,Yx=`#ifndef saturate
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
vec3 CustomToneMapping( vec3 color ) { return color; }`,Jx=`#ifdef USE_TRANSMISSION
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
#endif`,Zx=`#ifdef USE_TRANSMISSION
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
#endif`,$x=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,Qx=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,eb=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,tb=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,nb=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,ib=`uniform sampler2D t2D;
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
}`,sb=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,rb=`#ifdef ENVMAP_TYPE_CUBE
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
}`,ab=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,ob=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,lb=`#include <common>
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
}`,cb=`#if DEPTH_PACKING == 3200
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
}`,hb=`#define DISTANCE
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
}`,ub=`#define DISTANCE
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
}`,db=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,fb=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,pb=`uniform float scale;
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
}`,mb=`uniform vec3 diffuse;
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
}`,gb=`#include <common>
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
}`,xb=`uniform vec3 diffuse;
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
}`,bb=`#define LAMBERT
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
}`,vb=`#define LAMBERT
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
}`,_b=`#define MATCAP
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
}`,yb=`#define MATCAP
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
}`,Mb=`#define NORMAL
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
}`,Sb=`#define NORMAL
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
}`,wb=`#define PHONG
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
}`,Eb=`#define PHONG
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
}`,Tb=`#define STANDARD
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
}`,Ab=`#define STANDARD
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
}`,Rb=`#define TOON
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
}`,Cb=`#define TOON
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
}`,Pb=`uniform float size;
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
}`,Ib=`uniform vec3 diffuse;
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
}`,Db=`#include <common>
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
}`,Lb=`uniform vec3 color;
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
}`,Fb=`uniform float rotation;
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
}`,Nb=`uniform vec3 diffuse;
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
}`,Ct={alphahash_fragment:n0,alphahash_pars_fragment:i0,alphamap_fragment:s0,alphamap_pars_fragment:r0,alphatest_fragment:a0,alphatest_pars_fragment:o0,aomap_fragment:l0,aomap_pars_fragment:c0,batching_pars_vertex:h0,batching_vertex:u0,begin_vertex:d0,beginnormal_vertex:f0,bsdfs:p0,iridescence_fragment:m0,bumpmap_pars_fragment:g0,clipping_planes_fragment:x0,clipping_planes_pars_fragment:b0,clipping_planes_pars_vertex:v0,clipping_planes_vertex:_0,color_fragment:y0,color_pars_fragment:M0,color_pars_vertex:S0,color_vertex:w0,common:E0,cube_uv_reflection_fragment:T0,defaultnormal_vertex:A0,displacementmap_pars_vertex:R0,displacementmap_vertex:C0,emissivemap_fragment:P0,emissivemap_pars_fragment:I0,colorspace_fragment:D0,colorspace_pars_fragment:L0,envmap_fragment:F0,envmap_common_pars_fragment:N0,envmap_pars_fragment:U0,envmap_pars_vertex:O0,envmap_physical_pars_fragment:K0,envmap_vertex:z0,fog_vertex:k0,fog_pars_vertex:B0,fog_fragment:H0,fog_pars_fragment:G0,gradientmap_pars_fragment:V0,lightmap_pars_fragment:W0,lights_lambert_fragment:q0,lights_lambert_pars_fragment:X0,lights_pars_begin:j0,lights_toon_fragment:Y0,lights_toon_pars_fragment:J0,lights_phong_fragment:Z0,lights_phong_pars_fragment:$0,lights_physical_fragment:Q0,lights_physical_pars_fragment:ex,lights_fragment_begin:tx,lights_fragment_maps:nx,lights_fragment_end:ix,lightprobes_pars_fragment:sx,logdepthbuf_fragment:rx,logdepthbuf_pars_fragment:ax,logdepthbuf_pars_vertex:ox,logdepthbuf_vertex:lx,map_fragment:cx,map_pars_fragment:hx,map_particle_fragment:ux,map_particle_pars_fragment:dx,metalnessmap_fragment:fx,metalnessmap_pars_fragment:px,morphinstance_vertex:mx,morphcolor_vertex:gx,morphnormal_vertex:xx,morphtarget_pars_vertex:bx,morphtarget_vertex:vx,normal_fragment_begin:_x,normal_fragment_maps:yx,normal_pars_fragment:Mx,normal_pars_vertex:Sx,normal_vertex:wx,normalmap_pars_fragment:Ex,clearcoat_normal_fragment_begin:Tx,clearcoat_normal_fragment_maps:Ax,clearcoat_pars_fragment:Rx,iridescence_pars_fragment:Cx,opaque_fragment:Px,packing:Ix,premultiplied_alpha_fragment:Dx,project_vertex:Lx,dithering_fragment:Fx,dithering_pars_fragment:Nx,roughnessmap_fragment:Ux,roughnessmap_pars_fragment:Ox,shadowmap_pars_fragment:zx,shadowmap_pars_vertex:kx,shadowmap_vertex:Bx,shadowmask_pars_fragment:Hx,skinbase_vertex:Gx,skinning_pars_vertex:Vx,skinning_vertex:Wx,skinnormal_vertex:qx,specularmap_fragment:Xx,specularmap_pars_fragment:jx,tonemapping_fragment:Kx,tonemapping_pars_fragment:Yx,transmission_fragment:Jx,transmission_pars_fragment:Zx,uv_pars_fragment:$x,uv_pars_vertex:Qx,uv_vertex:eb,worldpos_vertex:tb,background_vert:nb,background_frag:ib,backgroundCube_vert:sb,backgroundCube_frag:rb,cube_vert:ab,cube_frag:ob,depth_vert:lb,depth_frag:cb,distance_vert:hb,distance_frag:ub,equirect_vert:db,equirect_frag:fb,linedashed_vert:pb,linedashed_frag:mb,meshbasic_vert:gb,meshbasic_frag:xb,meshlambert_vert:bb,meshlambert_frag:vb,meshmatcap_vert:_b,meshmatcap_frag:yb,meshnormal_vert:Mb,meshnormal_frag:Sb,meshphong_vert:wb,meshphong_frag:Eb,meshphysical_vert:Tb,meshphysical_frag:Ab,meshtoon_vert:Rb,meshtoon_frag:Cb,points_vert:Pb,points_frag:Ib,shadow_vert:Db,shadow_frag:Lb,sprite_vert:Fb,sprite_frag:Nb},Je={common:{diffuse:{value:new Le(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new xt},alphaMap:{value:null},alphaMapTransform:{value:new xt},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new xt}},envmap:{envMap:{value:null},envMapRotation:{value:new xt},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98},dfgLUT:{value:null}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new xt}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new xt}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new xt},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new xt},normalScale:{value:new Be(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new xt},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new xt}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new xt}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new xt}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new Le(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},sunLights:{value:[],properties:{direction:{},color:{}}},sunLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},sunShadowMatrix:{value:[]},sunShadowCascade:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null},probesSH:{value:null},probesMin:{value:new B},probesMax:{value:new B},probesResolution:{value:new B}},points:{diffuse:{value:new Le(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new xt},alphaTest:{value:0},uvTransform:{value:new xt}},sprite:{diffuse:{value:new Le(16777215)},opacity:{value:1},center:{value:new Be(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new xt},alphaMap:{value:null},alphaMapTransform:{value:new xt},alphaTest:{value:0}}},Ni={basic:{uniforms:Pn([Je.common,Je.specularmap,Je.envmap,Je.aomap,Je.lightmap,Je.fog]),vertexShader:Ct.meshbasic_vert,fragmentShader:Ct.meshbasic_frag},lambert:{uniforms:Pn([Je.common,Je.specularmap,Je.envmap,Je.aomap,Je.lightmap,Je.emissivemap,Je.bumpmap,Je.normalmap,Je.displacementmap,Je.fog,Je.lights,{emissive:{value:new Le(0)},envMapIntensity:{value:1}}]),vertexShader:Ct.meshlambert_vert,fragmentShader:Ct.meshlambert_frag},phong:{uniforms:Pn([Je.common,Je.specularmap,Je.envmap,Je.aomap,Je.lightmap,Je.emissivemap,Je.bumpmap,Je.normalmap,Je.displacementmap,Je.fog,Je.lights,{emissive:{value:new Le(0)},specular:{value:new Le(1118481)},shininess:{value:30},envMapIntensity:{value:1}}]),vertexShader:Ct.meshphong_vert,fragmentShader:Ct.meshphong_frag},standard:{uniforms:Pn([Je.common,Je.envmap,Je.aomap,Je.lightmap,Je.emissivemap,Je.bumpmap,Je.normalmap,Je.displacementmap,Je.roughnessmap,Je.metalnessmap,Je.fog,Je.lights,{emissive:{value:new Le(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:Ct.meshphysical_vert,fragmentShader:Ct.meshphysical_frag},toon:{uniforms:Pn([Je.common,Je.aomap,Je.lightmap,Je.emissivemap,Je.bumpmap,Je.normalmap,Je.displacementmap,Je.gradientmap,Je.fog,Je.lights,{emissive:{value:new Le(0)}}]),vertexShader:Ct.meshtoon_vert,fragmentShader:Ct.meshtoon_frag},matcap:{uniforms:Pn([Je.common,Je.bumpmap,Je.normalmap,Je.displacementmap,Je.fog,{matcap:{value:null}}]),vertexShader:Ct.meshmatcap_vert,fragmentShader:Ct.meshmatcap_frag},points:{uniforms:Pn([Je.points,Je.fog]),vertexShader:Ct.points_vert,fragmentShader:Ct.points_frag},dashed:{uniforms:Pn([Je.common,Je.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:Ct.linedashed_vert,fragmentShader:Ct.linedashed_frag},depth:{uniforms:Pn([Je.common,Je.displacementmap]),vertexShader:Ct.depth_vert,fragmentShader:Ct.depth_frag},normal:{uniforms:Pn([Je.common,Je.bumpmap,Je.normalmap,Je.displacementmap,{opacity:{value:1}}]),vertexShader:Ct.meshnormal_vert,fragmentShader:Ct.meshnormal_frag},sprite:{uniforms:Pn([Je.sprite,Je.fog]),vertexShader:Ct.sprite_vert,fragmentShader:Ct.sprite_frag},background:{uniforms:{uvTransform:{value:new xt},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:Ct.background_vert,fragmentShader:Ct.background_frag},backgroundCube:{uniforms:{envMap:{value:null},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new xt}},vertexShader:Ct.backgroundCube_vert,fragmentShader:Ct.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:Ct.cube_vert,fragmentShader:Ct.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:Ct.equirect_vert,fragmentShader:Ct.equirect_frag},distance:{uniforms:Pn([Je.common,Je.displacementmap,{referencePosition:{value:new B},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:Ct.distance_vert,fragmentShader:Ct.distance_frag},shadow:{uniforms:Pn([Je.lights,Je.fog,{color:{value:new Le(0)},opacity:{value:1}}]),vertexShader:Ct.shadow_vert,fragmentShader:Ct.shadow_frag}};Ni.physical={uniforms:Pn([Ni.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new xt},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new xt},clearcoatNormalScale:{value:new Be(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new xt},dispersion:{value:0},retroreflectivity:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new xt},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new xt},sheen:{value:0},sheenColor:{value:new Le(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new xt},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new xt},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new xt},transmissionSamplerSize:{value:new Be},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new xt},attenuationDistance:{value:0},attenuationColor:{value:new Le(0)},specularColor:{value:new Le(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new xt},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new xt},anisotropyVector:{value:new Be},anisotropyMap:{value:null},anisotropyMapTransform:{value:new xt}}]),vertexShader:Ct.meshphysical_vert,fragmentShader:Ct.meshphysical_frag};var bc={r:0,b:0,g:0},Ub=new ft,pp=new xt;pp.set(-1,0,0,0,1,0,0,0,1);function Ob(s,e,t,n,i,r){let a=new Le(0),o=i===!0?0:1,l,c,h=null,u=0,d=null;function p(y){let T=y.isScene===!0?y.background:null;if(T&&T.isTexture){let g=y.backgroundBlurriness>0;T=e.get(T,g)}return T}function x(y){let T=!1,g=p(y);g===null?m(a,o):g&&g.isColor&&(m(g,1),T=!0);let _=s.xr.getEnvironmentBlendMode();_==="additive"?t.buffers.color.setClear(0,0,0,1,r):_==="alpha-blend"&&t.buffers.color.setClear(0,0,0,0,r),(s.autoClear||T)&&(t.buffers.depth.setTest(!0),t.buffers.depth.setMask(!0),t.buffers.color.setMask(!0),s.clear(s.autoClearColor,s.autoClearDepth,s.autoClearStencil))}function v(y,T){let g=p(T);g&&(g.isCubeTexture||g.mapping===Ya)?(c===void 0&&(c=new st(new pi(1,1,1),new yt({name:"BackgroundCubeMaterial",uniforms:js(Ni.backgroundCube.uniforms),vertexShader:Ni.backgroundCube.vertexShader,fragmentShader:Ni.backgroundCube.fragmentShader,side:mn,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),c.geometry.deleteAttribute("normal"),c.geometry.deleteAttribute("uv"),c.onBeforeRender=function(_,w,A){this.matrixWorld.copyPosition(A.matrixWorld)},Object.defineProperty(c.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),n.update(c)),c.material.uniforms.envMap.value=g,c.material.uniforms.backgroundBlurriness.value=T.backgroundBlurriness,c.material.uniforms.backgroundIntensity.value=T.backgroundIntensity,c.material.uniforms.backgroundRotation.value.setFromMatrix4(Ub.makeRotationFromEuler(T.backgroundRotation)).transpose(),g.isCubeTexture&&g.isRenderTargetTexture===!1&&c.material.uniforms.backgroundRotation.value.premultiply(pp),c.material.toneMapped=Tt.getTransfer(g.colorSpace)!==Gt,(h!==g||u!==g.version||d!==s.toneMapping)&&(c.material.needsUpdate=!0,h=g,u=g.version,d=s.toneMapping),c.layers.enableAll(),y.unshift(c,c.geometry,c.material,0,0,null)):g&&g.isTexture&&(l===void 0&&(l=new st(new fn(2,2),new yt({name:"BackgroundMaterial",uniforms:js(Ni.background.uniforms),vertexShader:Ni.background.vertexShader,fragmentShader:Ni.background.fragmentShader,side:ti,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),l.geometry.deleteAttribute("normal"),Object.defineProperty(l.material,"map",{get:function(){return this.uniforms.t2D.value}}),n.update(l)),l.material.uniforms.t2D.value=g,l.material.uniforms.backgroundIntensity.value=T.backgroundIntensity,l.material.toneMapped=Tt.getTransfer(g.colorSpace)!==Gt,g.matrixAutoUpdate===!0&&g.updateMatrix(),l.material.uniforms.uvTransform.value.copy(g.matrix),(h!==g||u!==g.version||d!==s.toneMapping)&&(l.material.needsUpdate=!0,h=g,u=g.version,d=s.toneMapping),l.layers.enableAll(),y.unshift(l,l.geometry,l.material,0,0,null))}function m(y,T){y.getRGB(bc,Xh(s)),t.buffers.color.setClear(bc.r,bc.g,bc.b,T,r)}function f(){c!==void 0&&(c.geometry.dispose(),c.material.dispose(),c=void 0),l!==void 0&&(l.geometry.dispose(),l.material.dispose(),l=void 0)}return{getClearColor:function(){return a},setClearColor:function(y,T=1){a.set(y),o=T,m(a,o)},getClearAlpha:function(){return o},setClearAlpha:function(y){o=y,m(a,o)},render:x,addToRenderList:v,dispose:f}}function zb(s,e){let t=s.getParameter(s.MAX_VERTEX_ATTRIBS),n={},i=d(null),r=i,a=!1;function o(P,M,F,I,N){let H=!1,D=u(P,I,F,M);r!==D&&(r=D,c(r.object)),H=p(P,I,F,N),H&&x(P,I,F,N),N!==null&&e.update(N,s.ELEMENT_ARRAY_BUFFER),(H||a)&&(a=!1,g(P,M,F,I),N!==null&&s.bindBuffer(s.ELEMENT_ARRAY_BUFFER,e.get(N).buffer))}function l(){return s.createVertexArray()}function c(P){return s.bindVertexArray(P)}function h(P){return s.deleteVertexArray(P)}function u(P,M,F,I){let N=I.wireframe===!0,H=n[M.id];H===void 0&&(H={},n[M.id]=H);let D=P.isInstancedMesh===!0?P.id:0,V=H[D];V===void 0&&(V={},H[D]=V);let k=V[F.id];k===void 0&&(k={},V[F.id]=k);let E=k[N];return E===void 0&&(E=d(l()),k[N]=E),E}function d(P){let M=[],F=[],I=[];for(let N=0;N<t;N++)M[N]=0,F[N]=0,I[N]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:M,enabledAttributes:F,attributeDivisors:I,object:P,attributes:{},index:null}}function p(P,M,F,I){let N=r.attributes,H=M.attributes,D=0,V=F.getAttributes();for(let k in V)if(V[k].location>=0){let Z=N[k],W=H[k];if(W===void 0&&(k==="instanceMatrix"&&P.instanceMatrix&&(W=P.instanceMatrix),k==="instanceColor"&&P.instanceColor&&(W=P.instanceColor)),Z===void 0||Z.attribute!==W||W&&Z.data!==W.data)return!0;D++}return r.attributesNum!==D||r.index!==I}function x(P,M,F,I){let N={},H=M.attributes,D=0,V=F.getAttributes();for(let k in V)if(V[k].location>=0){let Z=H[k];Z===void 0&&(k==="instanceMatrix"&&P.instanceMatrix&&(Z=P.instanceMatrix),k==="instanceColor"&&P.instanceColor&&(Z=P.instanceColor));let W={};W.attribute=Z,Z&&Z.data&&(W.data=Z.data),N[k]=W,D++}r.attributes=N,r.attributesNum=D,r.index=I}function v(){let P=r.newAttributes;for(let M=0,F=P.length;M<F;M++)P[M]=0}function m(P){f(P,0)}function f(P,M){let F=r.newAttributes,I=r.enabledAttributes,N=r.attributeDivisors;F[P]=1,I[P]===0&&(s.enableVertexAttribArray(P),I[P]=1),N[P]!==M&&(s.vertexAttribDivisor(P,M),N[P]=M)}function y(){let P=r.newAttributes,M=r.enabledAttributes;for(let F=0,I=M.length;F<I;F++)M[F]!==P[F]&&(s.disableVertexAttribArray(F),M[F]=0)}function T(P,M,F,I,N,H,D){D===!0?s.vertexAttribIPointer(P,M,F,N,H):s.vertexAttribPointer(P,M,F,I,N,H)}function g(P,M,F,I){v();let N=I.attributes,H=F.getAttributes(),D=M.defaultAttributeValues;for(let V in H){let k=H[V];if(k.location>=0){let E=N[V];if(E===void 0&&(V==="instanceMatrix"&&P.instanceMatrix&&(E=P.instanceMatrix),V==="instanceColor"&&P.instanceColor&&(E=P.instanceColor)),E!==void 0){let Z=E.normalized,W=E.itemSize,Q=e.get(E);if(Q===void 0)continue;let Fe=Q.buffer,Ne=Q.type,ze=Q.bytesPerElement,se=Ne===s.INT||Ne===s.UNSIGNED_INT||E.gpuType===Il;if(E.isInterleavedBufferAttribute){let le=E.data,De=le.stride,be=E.offset;if(le.isInstancedInterleavedBuffer){for(let ue=0;ue<k.locationSize;ue++)f(k.location+ue,le.meshPerAttribute);P.isInstancedMesh!==!0&&I._maxInstanceCount===void 0&&(I._maxInstanceCount=le.meshPerAttribute*le.count)}else for(let ue=0;ue<k.locationSize;ue++)m(k.location+ue);s.bindBuffer(s.ARRAY_BUFFER,Fe);for(let ue=0;ue<k.locationSize;ue++)T(k.location+ue,W/k.locationSize,Ne,Z,De*ze,(be+W/k.locationSize*ue)*ze,se)}else{if(E.isInstancedBufferAttribute){for(let le=0;le<k.locationSize;le++)f(k.location+le,E.meshPerAttribute);P.isInstancedMesh!==!0&&I._maxInstanceCount===void 0&&(I._maxInstanceCount=E.meshPerAttribute*E.count)}else for(let le=0;le<k.locationSize;le++)m(k.location+le);s.bindBuffer(s.ARRAY_BUFFER,Fe);for(let le=0;le<k.locationSize;le++)T(k.location+le,W/k.locationSize,Ne,Z,W*ze,W/k.locationSize*le*ze,se)}}else if(D!==void 0){let Z=D[V];if(Z!==void 0)switch(Z.length){case 2:s.vertexAttrib2fv(k.location,Z);break;case 3:s.vertexAttrib3fv(k.location,Z);break;case 4:s.vertexAttrib4fv(k.location,Z);break;default:s.vertexAttrib1fv(k.location,Z)}}}}y()}function _(){C();for(let P in n){let M=n[P];for(let F in M){let I=M[F];for(let N in I){let H=I[N];for(let D in H)h(H[D].object),delete H[D];delete I[N]}}delete n[P]}}function w(P){if(n[P.id]===void 0)return;let M=n[P.id];for(let F in M){let I=M[F];for(let N in I){let H=I[N];for(let D in H)h(H[D].object),delete H[D];delete I[N]}}delete n[P.id]}function A(P){for(let M in n){let F=n[M];for(let I in F){let N=F[I];if(N[P.id]===void 0)continue;let H=N[P.id];for(let D in H)h(H[D].object),delete H[D];delete N[P.id]}}}function b(P){for(let M in n){let F=n[M],I=P.isInstancedMesh===!0?P.id:0,N=F[I];if(N!==void 0){for(let H in N){let D=N[H];for(let V in D)h(D[V].object),delete D[V];delete N[H]}delete F[I],Object.keys(F).length===0&&delete n[M]}}}function C(){S(),a=!0,r!==i&&(r=i,c(r.object))}function S(){i.geometry=null,i.program=null,i.wireframe=!1}return{setup:o,reset:C,resetDefaultState:S,dispose:_,releaseStatesOfGeometry:w,releaseStatesOfObject:b,releaseStatesOfProgram:A,initAttributes:v,enableAttribute:m,disableUnusedAttributes:y}}function kb(s,e,t){let n;function i(l){n=l}function r(l,c){s.drawArrays(n,l,c),t.update(c,n,1)}function a(l,c,h){h!==0&&(s.drawArraysInstanced(n,l,c,h),t.update(c,n,h))}function o(l,c,h){if(h===0)return;e.get("WEBGL_multi_draw").multiDrawArraysWEBGL(n,l,0,c,0,h);let d=0;for(let p=0;p<h;p++)d+=c[p];t.update(d,n,1)}this.setMode=i,this.render=r,this.renderInstances=a,this.renderMultiDraw=o}function Bb(s,e,t,n){let i;function r(){if(i!==void 0)return i;if(e.has("EXT_texture_filter_anisotropic")===!0){let A=e.get("EXT_texture_filter_anisotropic");i=s.getParameter(A.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else i=0;return i}function a(A){return!(A!==Jn&&n.convert(A)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_FORMAT))}function o(A){let b=A===pn&&(e.has("EXT_color_buffer_half_float")||e.has("EXT_color_buffer_float"));return!(A!==Gn&&A!==Yn&&!b&&n.convert(A)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_TYPE))}function l(A){if(A==="highp"){if(s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.HIGH_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.HIGH_FLOAT).precision>0)return"highp";A="mediump"}return A==="mediump"&&s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.MEDIUM_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let c=t.precision!==void 0?t.precision:"highp",h=l(c);h!==c&&(rt("WebGLRenderer:",c,"not supported, using",h,"instead."),c=h);let u=t.logarithmicDepthBuffer===!0,d=t.reversedDepthBuffer===!0&&e.has("EXT_clip_control");t.reversedDepthBuffer===!0&&d===!1&&rt("WebGLRenderer: Unable to use reversed depth buffer due to missing EXT_clip_control extension. Fallback to default depth buffer.");let p=s.getParameter(s.MAX_TEXTURE_IMAGE_UNITS),x=s.getParameter(s.MAX_VERTEX_TEXTURE_IMAGE_UNITS),v=s.getParameter(s.MAX_TEXTURE_SIZE),m=s.getParameter(s.MAX_CUBE_MAP_TEXTURE_SIZE),f=s.getParameter(s.MAX_VERTEX_ATTRIBS),y=s.getParameter(s.MAX_VERTEX_UNIFORM_VECTORS),T=s.getParameter(s.MAX_VARYING_VECTORS),g=s.getParameter(s.MAX_FRAGMENT_UNIFORM_VECTORS),_=s.getParameter(s.MAX_SAMPLES),w=s.getParameter(s.SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:l,textureFormatReadable:a,textureTypeReadable:o,precision:c,logarithmicDepthBuffer:u,reversedDepthBuffer:d,maxTextures:p,maxVertexTextures:x,maxTextureSize:v,maxCubemapSize:m,maxAttributes:f,maxVertexUniforms:y,maxVaryings:T,maxFragmentUniforms:g,maxSamples:_,samples:w}}function Hb(s){let e=this,t=null,n=0,i=!1,r=!1,a=new Xn,o=new xt,l={value:null,needsUpdate:!1};this.uniform=l,this.numPlanes=0,this.numIntersection=0,this.init=function(u,d){let p=u.length!==0||d||n!==0||i;return i=d,n=u.length,p},this.beginShadows=function(){r=!0,h(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(u,d){t=h(u,d,0)},this.setState=function(u,d,p){let x=u.clippingPlanes,v=u.clipIntersection,m=u.clipShadows,f=s.get(u);if(!i||x===null||x.length===0||r&&!m)r?h(null):c();else{let y=r?0:n,T=y*4,g=f.clippingState||null;l.value=g,g=h(x,d,T,p);for(let _=0;_!==T;++_)g[_]=t[_];f.clippingState=g,this.numIntersection=v?this.numPlanes:0,this.numPlanes+=y}};function c(){l.value!==t&&(l.value=t,l.needsUpdate=n>0),e.numPlanes=n,e.numIntersection=0}function h(u,d,p,x){let v=u!==null?u.length:0,m=null;if(v!==0){if(m=l.value,x!==!0||m===null){let f=p+v*4,y=d.matrixWorldInverse;o.getNormalMatrix(y),(m===null||m.length<f)&&(m=new Float32Array(f));for(let T=0,g=p;T!==v;++T,g+=4)a.copy(u[T]).applyMatrix4(y,o),a.normal.toArray(m,g),m[g+3]=a.constant}l.value=m,l.needsUpdate=!0}return e.numPlanes=v,e.numIntersection=0,m}}var Wr=4,Gb=6,Vb=20,Wb=256,so=new Li,Xf=new Le,Jh=null,Zh=0,$h=0,Qh=!1,qb=new B,Ks=new B,Xr=class{constructor(e){this._renderer=e,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._sizeLods=[],this._lodMeshes=[],this._backgroundBox=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._blurMaterial=null,this._ggxMaterial=null}fromScene(e,t=0,n=.1,i=100,r={}){let{size:a=256,position:o=qb}=r;Jh=this._renderer.getRenderTarget(),Zh=this._renderer.getActiveCubeFace(),$h=this._renderer.getActiveMipmapLevel(),Qh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(a);let l=this._allocateTargets();return l.depthBuffer=!0,this._sceneToCubeUV(e,n,i,l,o),t>0&&this._blur(l,0,0,t),this._applyPMREM(l),this._cleanup(l),l}fromEquirectangular(e,t=null){return this._fromTexture(e,t)}fromCubemap(e,t=null){return this._fromTexture(e,t)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=Yf(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=Kf(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose(),this._backgroundBox!==null&&(this._backgroundBox.geometry.dispose(),this._backgroundBox.material.dispose())}_setSize(e){this._lodMax=Math.floor(Math.log2(e)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._ggxMaterial!==null&&this._ggxMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let e=0;e<this._lodMeshes.length;e++)this._lodMeshes[e].geometry.dispose()}_cleanup(e){this._renderer.setRenderTarget(Jh,Zh,$h),this._renderer.xr.enabled=Qh,e.scissorTest=!1,Vr(e,0,0,e.width,e.height)}_fromTexture(e,t){e.mapping===xs||e.mapping===qs?this._setSize(e.image.length===0?16:e.image[0].width||e.image[0].image.width):this._setSize(e.image.width/4),Jh=this._renderer.getRenderTarget(),Zh=this._renderer.getActiveCubeFace(),$h=this._renderer.getActiveMipmapLevel(),Qh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let n=t||this._allocateTargets();return this._textureToCubeUV(e,n),this._applyPMREM(n),this._cleanup(n),n}_allocateTargets(){let e=3*Math.max(this._cubeSize,112),t=4*this._cubeSize,n={magFilter:rn,minFilter:rn,generateMipmaps:!1,type:pn,format:Jn,colorSpace:Un,depthBuffer:!1},i=jf(e,t,n);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==e||this._pingPongRenderTarget.height!==t){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=jf(e,t,n);let{_lodMax:r}=this;({lodMeshes:this._lodMeshes,sizeLods:this._sizeLods}=Xb(r)),this._blurMaterial=Kb(r,e,t),this._ggxMaterial=jb(r,e,t)}return i}_compileMaterial(e){let t=new st(new _t,e);this._renderer.compile(t,so)}_sceneToCubeUV(e,t,n,i,r){let l=new dn(90,1,t,n),c=[1,-1,1,1,1,1],h=[1,1,1,-1,-1,-1],u=this._renderer,d=u.autoClear,p=u.toneMapping;u.getClearColor(Xf),u.toneMapping=xi,u.autoClear=!1,u.state.buffers.depth.getReversed()&&(u.setRenderTarget(i),u.clearDepth(),u.setRenderTarget(null)),this._backgroundBox===null&&(this._backgroundBox=new st(new pi,new Vt({name:"PMREM.Background",side:mn,depthWrite:!1,depthTest:!1})));let v=this._backgroundBox,m=v.material,f=!1,y=e.background;y?y.isColor&&(m.color.copy(y),e.background=null,f=!0):(m.color.copy(Xf),f=!0);for(let T=0;T<6;T++){let g=T%3;g===0?(l.up.set(0,c[T],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x+h[T],r.y,r.z)):g===1?(l.up.set(0,0,c[T]),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y+h[T],r.z)):(l.up.set(0,c[T],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y,r.z+h[T]));let _=this._cubeSize;Vr(i,g*_,T>2?_:0,_,_),u.setRenderTarget(i),f&&u.render(v,l),u.render(e,l)}u.toneMapping=p,u.autoClear=d,e.background=y}_textureToCubeUV(e,t){let n=this._renderer,i=e.mapping===xs||e.mapping===qs;i?(this._cubemapMaterial===null&&(this._cubemapMaterial=Yf()),this._cubemapMaterial.uniforms.flipEnvMap.value=e.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=Kf());let r=i?this._cubemapMaterial:this._equirectMaterial,a=this._lodMeshes[0];a.material=r;let o=r.uniforms;o.envMap.value=e;let l=this._cubeSize;Vr(t,0,0,3*l,2*l),n.setRenderTarget(t),n.render(a,so)}_applyPMREM(e){let t=this._renderer,n=t.autoClear;t.autoClear=!1;let i=this._lodMeshes.length;for(let r=1;r<i;r++)this._applyGGXFilter(e,r-1,r);t.autoClear=n}_applyGGXFilter(e,t,n){let i=this._renderer,r=this._pingPongRenderTarget,a=this._ggxMaterial,o=this._lodMeshes[n];o.material=a;let l=a.uniforms,c=n/(this._lodMeshes.length-1),h=t/(this._lodMeshes.length-1),u=Math.sqrt(c*c-h*h),d=c*1.25,p=u*d,{_lodMax:x}=this,v=this._sizeLods[n],m=3*v*(n>x-Wr?n-x+Wr:0),f=4*(this._cubeSize-v);l.envMap.value=e.texture,l.roughness.value=p,l.mipInt.value=x-t,Vr(r,m,f,3*v,2*v),i.setRenderTarget(r),i.render(o,so),l.envMap.value=r.texture,l.roughness.value=0,l.mipInt.value=x-n,Vr(e,m,f,3*v,2*v),i.setRenderTarget(e),i.render(o,so)}_blur(e,t,n,i){let r=this._pingPongRenderTarget,a=Math.min(i,Math.PI)/Math.SQRT2;this._blurPass(e,r,t,n,a),this._blurPass(r,e,n,n,a)}_blurPass(e,t,n,i,r){let a=this._renderer,o=this._blurMaterial,l=this._lodMeshes[i];l.material=o;let c=o.uniforms;c.envMap.value=e.texture,c.sigma.value=r,c.mipInt.value=this._lodMax-n;let h=this._sizeLods[i],u=3*h*(i>this._lodMax-Wr?i-this._lodMax+Wr:0),d=4*(this._cubeSize-h);Vr(t,u,d,3*h,2*h),a.setRenderTarget(t),a.render(l,so)}};function Xb(s){let e=[],t=[],n=s,i=s-Wr+1+Gb;for(let r=0;r<i;r++){let a=Math.pow(2,n);e.push(a);let o=1/(a-2),l=-o,c=1+o,h=[l,l,c,l,c,c,l,l,c,c,l,c],u=6,d=6,p=3,x=new Float32Array(p*d*u),v=new Float32Array(p*d*u);for(let f=0;f<u;f++){let y=f%3*2/3-1,T=f>2?0:-1,g=[y,T,0,y+2/3,T,0,y+2/3,T+1,0,y,T,0,y+2/3,T+1,0,y,T+1,0];x.set(g,p*d*f);for(let _=0;_<d;_++){let w=h[_*2]*2-1,A=h[_*2+1]*2-1;f===0?Ks.set(1,A,w):f===1?Ks.set(-w,1,-A):f===2?Ks.set(-w,A,1):f===3?Ks.set(-1,A,-w):f===4?Ks.set(-w,-1,A):Ks.set(w,A,-1),Ks.toArray(v,(f*d+_)*p)}}let m=new _t;m.setAttribute("position",new Rt(x,p)),m.setAttribute("outputDirection",new Rt(v,p)),t.push(new st(m,null)),n>Wr&&n--}return{lodMeshes:t,sizeLods:e}}function jf(s,e,t){let n=new an(s,e,t);return n.texture.mapping=Ya,n.texture.name="PMREM.cubeUv",n.scissorTest=!0,n}function Vr(s,e,t,n,i){s.viewport.set(e,t,n,i),s.scissor.set(e,t,n,i)}function jb(s,e,t){return new yt({name:"PMREMGGXConvolution",defines:{GGX_SAMPLES:Wb,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},roughness:{value:0},mipInt:{value:0}},vertexShader:Mc(),fragmentShader:`

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
		`,blending:ni,depthTest:!1,depthWrite:!1})}function Kb(s,e,t){return new yt({name:"SphericalGaussianBlur",defines:{SAMPLES:Vb,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},sigma:{value:0},mipInt:{value:0}},vertexShader:Mc(),fragmentShader:`

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
		`,blending:ni,depthTest:!1,depthWrite:!1})}function Kf(){return new yt({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:Mc(),fragmentShader:`

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
		`,blending:ni,depthTest:!1,depthWrite:!1})}function Yf(){return new yt({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:Mc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:ni,depthTest:!1,depthWrite:!1})}function Mc(){return`

		precision mediump float;
		precision mediump int;

		attribute vec3 outputDirection;

		varying vec3 vOutputDirection;

		void main() {

			vOutputDirection = outputDirection;
			gl_Position = vec4( position, 1.0 );

		}
	`}var _c=class extends an{constructor(e=1,t={}){super(e,e,t),this.isWebGLCubeRenderTarget=!0;let n={width:e,height:e,depth:1},i=[n,n,n,n,n,n];this.texture=new Ea(i),this._setTextureOptions(t),this.texture.isRenderTargetTexture=!0}fromEquirectangularTexture(e,t){this.texture.type=t.type,this.texture.colorSpace=t.colorSpace,this.texture.generateMipmaps=t.generateMipmaps,this.texture.minFilter=t.minFilter,this.texture.magFilter=t.magFilter;let n={uniforms:{tEquirect:{value:null}},vertexShader:`

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
			`},i=new pi(5,5,5),r=new yt({name:"CubemapFromEquirect",uniforms:js(n.uniforms),vertexShader:n.vertexShader,fragmentShader:n.fragmentShader,side:mn,blending:ni});r.uniforms.tEquirect.value=t;let a=new st(i,r),o=t.minFilter;return t.minFilter===bi&&(t.minFilter=rn),new Sl(1,10,this).update(e,a),t.minFilter=o,a.geometry.dispose(),a.material.dispose(),this}clear(e,t=!0,n=!0,i=!0){let r=e.getRenderTarget();for(let a=0;a<6;a++)e.setRenderTarget(this,a),e.clear(t,n,i);e.setRenderTarget(r)}};function Yb(s){let e=new WeakMap,t=new WeakMap,n=null;function i(d,p=!1){return d==null?null:p?a(d):r(d)}function r(d){if(d&&d.isTexture){let p=d.mapping;if(p===Rl||p===Cl)if(e.has(d)){let x=e.get(d).texture;return o(x,d.mapping)}else{let x=d.image;if(x&&x.height>0){let v=new _c(x.height);return v.fromEquirectangularTexture(s,d),e.set(d,v),d.addEventListener("dispose",c),o(v.texture,d.mapping)}else return null}}return d}function a(d){if(d&&d.isTexture){let p=d.mapping,x=p===Rl||p===Cl,v=p===xs||p===qs;if(x||v){let m=t.get(d),f=m!==void 0?m.texture.pmremVersion:0;if(d.isRenderTargetTexture&&d.pmremVersion!==f)return n===null&&(n=new Xr(s)),m=x?n.fromEquirectangular(d,m):n.fromCubemap(d,m),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),m.texture;if(m!==void 0)return m.texture;{let y=d.image;return x&&y&&y.height>0||v&&y&&l(y)?(n===null&&(n=new Xr(s)),m=x?n.fromEquirectangular(d):n.fromCubemap(d),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),d.addEventListener("dispose",h),m.texture):null}}}return d}function o(d,p){return p===Rl?d.mapping=xs:p===Cl&&(d.mapping=qs),d}function l(d){let p=0,x=6;for(let v=0;v<x;v++)d[v]!==void 0&&p++;return p===x}function c(d){let p=d.target;p.removeEventListener("dispose",c);let x=e.get(p);x!==void 0&&(e.delete(p),x.dispose())}function h(d){let p=d.target;p.removeEventListener("dispose",h);let x=t.get(p);x!==void 0&&(t.delete(p),x.dispose())}function u(){e=new WeakMap,t=new WeakMap,n!==null&&(n.dispose(),n=null)}return{get:i,dispose:u}}function Jb(s){let e={};function t(n){if(e[n]!==void 0)return e[n];let i=s.getExtension(n);return e[n]=i,i}return{has:function(n){return t(n)!==null},init:function(){t("EXT_color_buffer_float"),t("WEBGL_clip_cull_distance"),t("OES_texture_float_linear"),t("EXT_color_buffer_half_float"),t("WEBGL_multisampled_render_to_texture"),t("WEBGL_render_shared_exponent")},get:function(n){let i=t(n);return i===null&&Ds("WebGLRenderer: "+n+" extension not supported."),i}}}function Zb(s,e,t,n){let i={},r=new WeakMap;function a(u){let d=u.target;d.index!==null&&e.remove(d.index);for(let x in d.attributes)e.remove(d.attributes[x]);d.removeEventListener("dispose",a),delete i[d.id];let p=r.get(d);p&&(e.remove(p),r.delete(d)),n.releaseStatesOfGeometry(d),d.isInstancedBufferGeometry===!0&&delete d._maxInstanceCount,t.memory.geometries--}function o(u,d){return i[d.id]===!0||(d.addEventListener("dispose",a),i[d.id]=!0,t.memory.geometries++),d}function l(u){let d=u.attributes;for(let p in d)e.update(d[p],s.ARRAY_BUFFER)}function c(u){let d=[],p=u.index,x=u.attributes.position,v=0;if(x===void 0)return;if(p!==null){let y=p.array;v=p.version;for(let T=0,g=y.length;T<g;T+=3){let _=y[T+0],w=y[T+1],A=y[T+2];d.push(_,w,w,A,A,_)}}else{let y=x.array;v=x.version;for(let T=0,g=y.length/3-1;T<g;T+=3){let _=T+0,w=T+1,A=T+2;d.push(_,w,w,A,A,_)}}let m=new(x.count>=65535?ya:_a)(d,1);m.version=v;let f=r.get(u);f&&e.remove(f),r.set(u,m)}function h(u){let d=r.get(u);if(d){let p=u.index;p!==null&&d.version<p.version&&c(u)}else c(u);return r.get(u)}return{get:o,update:l,getWireframeAttribute:h}}function $b(s,e,t){let n;function i(u){n=u}let r,a;function o(u){r=u.type,a=u.bytesPerElement}function l(u,d){s.drawElements(n,d,r,u*a),t.update(d,n,1)}function c(u,d,p){p!==0&&(s.drawElementsInstanced(n,d,r,u*a,p),t.update(d,n,p))}function h(u,d,p){if(p===0)return;e.get("WEBGL_multi_draw").multiDrawElementsWEBGL(n,d,0,r,u,0,p);let v=0;for(let m=0;m<p;m++)v+=d[m];t.update(v,n,1)}this.setMode=i,this.setIndex=o,this.render=l,this.renderInstances=c,this.renderMultiDraw=h}function Qb(s){let e={geometries:0,textures:0},t={frame:0,calls:0,triangles:0,points:0,lines:0};function n(r,a,o){switch(t.calls++,a){case s.TRIANGLES:t.triangles+=o*(r/3);break;case s.LINES:t.lines+=o*(r/2);break;case s.LINE_STRIP:t.lines+=o*(r-1);break;case s.LINE_LOOP:t.lines+=o*r;break;case s.POINTS:t.points+=o*r;break;default:pt("WebGLInfo: Unknown draw mode:",a);break}}function i(){t.calls=0,t.triangles=0,t.points=0,t.lines=0}return{memory:e,render:t,programs:null,autoReset:!0,reset:i,update:n}}function ev(s,e,t){let n=new WeakMap,i=new Ot;function r(a,o,l){let c=a.morphTargetInfluences,h=o.morphAttributes.position||o.morphAttributes.normal||o.morphAttributes.color,u=h!==void 0?h.length:0,d=n.get(o);if(d===void 0||d.count!==u){let C=function(){A.dispose(),n.delete(o),o.removeEventListener("dispose",C)};d!==void 0&&d.texture.dispose();let p=o.morphAttributes.position!==void 0,x=o.morphAttributes.normal!==void 0,v=o.morphAttributes.color!==void 0,m=o.morphAttributes.position||[],f=o.morphAttributes.normal||[],y=o.morphAttributes.color||[],T=0;p===!0&&(T=1),x===!0&&(T=2),v===!0&&(T=3);let g=o.attributes.position.count*T,_=1;g>e.maxTextureSize&&(_=Math.ceil(g/e.maxTextureSize),g=e.maxTextureSize);let w=new Float32Array(g*_*4*u),A=new ba(w,g,_,u);A.type=Yn,A.needsUpdate=!0;let b=T*4;for(let S=0;S<u;S++){let P=m[S],M=f[S],F=y[S],I=g*_*4*S;for(let N=0;N<P.count;N++){let H=N*b;p===!0&&(i.fromBufferAttribute(P,N),w[I+H+0]=i.x,w[I+H+1]=i.y,w[I+H+2]=i.z,w[I+H+3]=0),x===!0&&(i.fromBufferAttribute(M,N),w[I+H+4]=i.x,w[I+H+5]=i.y,w[I+H+6]=i.z,w[I+H+7]=0),v===!0&&(i.fromBufferAttribute(F,N),w[I+H+8]=i.x,w[I+H+9]=i.y,w[I+H+10]=i.z,w[I+H+11]=F.itemSize===4?i.w:1)}}d={count:u,texture:A,size:new Be(g,_)},n.set(o,d),o.addEventListener("dispose",C)}if(a.isInstancedMesh===!0&&a.morphTexture!==null)l.getUniforms().setValue(s,"morphTexture",a.morphTexture,t);else{let p=0;for(let v=0;v<c.length;v++)p+=c[v];let x=o.morphTargetsRelative?1:1-p;l.getUniforms().setValue(s,"morphTargetBaseInfluence",x),l.getUniforms().setValue(s,"morphTargetInfluences",c)}l.getUniforms().setValue(s,"morphTargetsTexture",d.texture,t),l.getUniforms().setValue(s,"morphTargetsTextureSize",d.size)}return{update:r}}function tv(s,e,t,n,i){let r=new WeakMap;function a(c){let h=i.render.frame,u=c.geometry,d=e.get(c,u);if(r.get(d)!==h&&(e.update(d),r.set(d,h)),c.isInstancedMesh&&(c.hasEventListener("dispose",l)===!1&&c.addEventListener("dispose",l),r.get(c)!==h&&(t.update(c.instanceMatrix,s.ARRAY_BUFFER),c.instanceColor!==null&&t.update(c.instanceColor,s.ARRAY_BUFFER),r.set(c,h))),c.isSkinnedMesh){let p=c.skeleton;r.get(p)!==h&&(p.update(),r.set(p,h))}return d}function o(){r=new WeakMap}function l(c){let h=c.target;h.removeEventListener("dispose",l),n.releaseStatesOfObject(h),t.remove(h.instanceMatrix),h.instanceColor!==null&&t.remove(h.instanceColor)}return{update:a,dispose:o}}var nv={[Va]:"LINEAR_TONE_MAPPING",[Wa]:"REINHARD_TONE_MAPPING",[qa]:"CINEON_TONE_MAPPING",[Ws]:"ACES_FILMIC_TONE_MAPPING",[ja]:"AGX_TONE_MAPPING",[Ka]:"NEUTRAL_TONE_MAPPING",[Xa]:"CUSTOM_TONE_MAPPING"};function iv(s,e,t,n,i,r){let a=new an(e,t,{type:s,depthBuffer:i,stencilBuffer:r,samples:n?4:0,storeMultisampledDepthBuffer:!1,storeMultisampledStencilBuffer:!1,resolveDepthBuffer:!1,resolveStencilBuffer:!1}),o=null,l=null,c=new _t;c.setAttribute("position",new mt([-1,3,0,-1,-1,0,3,-1,0],3)),c.setAttribute("uv",new mt([0,2,0,0,2,0],2));let h=new Lr({uniforms:{tDiffuse:{value:null}},vertexShader:`
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
			}`,depthTest:!1,depthWrite:!1}),u=new st(c,h),d=new Li(-1,1,1,-1,0,1),p=null,x=null,v=!1,m,f=null,y=[],T=!1;this.setSize=function(g,_){a.setSize(g,_),o!==null&&o.setSize(g,_),l!==null&&l.setSize(g,_);for(let w=0;w<y.length;w++){let A=y[w];A.setSize&&A.setSize(g,_)}},this.setEffects=function(g){y=g,T=y.length>0&&y[0].isRenderPass===!0;let _=a.width,w=a.height;y.length>0&&o===null&&(o=new an(_,w,{type:pn,depthBuffer:!1,stencilBuffer:!1}),l=new an(_,w,{type:pn,depthBuffer:!1,stencilBuffer:!1}));for(let A=0;A<y.length;A++){let b=y[A];b.setSize&&b.setSize(_,w)}},this.begin=function(g,_){if(v||g.toneMapping===xi&&y.length===0)return!1;if(f=_,_!==null){let w=_.width,A=_.height;(a.width!==w||a.height!==A)&&this.setSize(w,A)}return T===!1&&g.setRenderTarget(a),m=g.toneMapping,g.toneMapping=xi,!0},this.hasRenderPass=function(){return T},this.end=function(g,_){g.toneMapping=m,v=!0;let w=a,A=o;for(let b=0;b<y.length;b++){let C=y[b];C.enabled!==!1&&(C.render(g,A,w,_),C.needsSwap!==!1&&(w=A,A=A===o?l:o))}if(p!==g.outputColorSpace||x!==g.toneMapping){p=g.outputColorSpace,x=g.toneMapping,h.defines={},Tt.getTransfer(p)===Gt&&(h.defines.SRGB_TRANSFER="");let b=nv[x];b&&(h.defines[b]=""),h.needsUpdate=!0}h.uniforms.tDiffuse.value=w.texture,g.setRenderTarget(f),g.render(u,d),f=null,v=!1},this.isCompositing=function(){return v},this.dispose=function(){a.dispose(),o!==null&&o.dispose(),l!==null&&l.dispose(),c.dispose(),h.dispose()}}var mp=new ln,nu=new us(1,1),gp=new ba,xp=new sl,bp=new Ea,Jf=[],Zf=[],$f=new Float32Array(16),Qf=new Float32Array(9),ep=new Float32Array(4);function jr(s,e,t){let n=s[0];if(n<=0||n>0)return s;let i=e*t,r=Jf[i];if(r===void 0&&(r=new Float32Array(i),Jf[i]=r),e!==0){n.toArray(r,0);for(let a=1,o=0;a!==e;++a)o+=t,s[a].toArray(r,o)}return r}function gn(s,e){if(s.length!==e.length)return!1;for(let t=0,n=s.length;t<n;t++)if(s[t]!==e[t])return!1;return!0}function xn(s,e){for(let t=0,n=e.length;t<n;t++)s[t]=e[t]}function Sc(s,e){let t=Zf[e];t===void 0&&(t=new Int32Array(e),Zf[e]=t);for(let n=0;n!==e;++n)t[n]=s.allocateTextureUnit();return t}function sv(s,e){let t=this.cache;t[0]!==e&&(s.uniform1f(this.addr,e),t[0]=e)}function rv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2f(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(gn(t,e))return;s.uniform2fv(this.addr,e),xn(t,e)}}function av(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3f(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else if(e.r!==void 0)(t[0]!==e.r||t[1]!==e.g||t[2]!==e.b)&&(s.uniform3f(this.addr,e.r,e.g,e.b),t[0]=e.r,t[1]=e.g,t[2]=e.b);else{if(gn(t,e))return;s.uniform3fv(this.addr,e),xn(t,e)}}function ov(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4f(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(gn(t,e))return;s.uniform4fv(this.addr,e),xn(t,e)}}function lv(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(gn(t,e))return;s.uniformMatrix2fv(this.addr,!1,e),xn(t,e)}else{if(gn(t,n))return;ep.set(n),s.uniformMatrix2fv(this.addr,!1,ep),xn(t,n)}}function cv(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(gn(t,e))return;s.uniformMatrix3fv(this.addr,!1,e),xn(t,e)}else{if(gn(t,n))return;Qf.set(n),s.uniformMatrix3fv(this.addr,!1,Qf),xn(t,n)}}function hv(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(gn(t,e))return;s.uniformMatrix4fv(this.addr,!1,e),xn(t,e)}else{if(gn(t,n))return;$f.set(n),s.uniformMatrix4fv(this.addr,!1,$f),xn(t,n)}}function uv(s,e){let t=this.cache;t[0]!==e&&(s.uniform1i(this.addr,e),t[0]=e)}function dv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2i(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(gn(t,e))return;s.uniform2iv(this.addr,e),xn(t,e)}}function fv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3i(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(gn(t,e))return;s.uniform3iv(this.addr,e),xn(t,e)}}function pv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4i(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(gn(t,e))return;s.uniform4iv(this.addr,e),xn(t,e)}}function mv(s,e){let t=this.cache;t[0]!==e&&(s.uniform1ui(this.addr,e),t[0]=e)}function gv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2ui(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(gn(t,e))return;s.uniform2uiv(this.addr,e),xn(t,e)}}function xv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3ui(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(gn(t,e))return;s.uniform3uiv(this.addr,e),xn(t,e)}}function bv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4ui(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(gn(t,e))return;s.uniform4uiv(this.addr,e),xn(t,e)}}function vv(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i);let r;this.type===s.SAMPLER_2D_SHADOW?(nu.compareFunction=t.isReversedDepthBuffer()?xc:gc,r=nu):r=mp,t.setTexture2D(e||r,i)}function _v(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture3D(e||xp,i)}function yv(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTextureCube(e||bp,i)}function Mv(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture2DArray(e||gp,i)}function Sv(s){switch(s){case 5126:return sv;case 35664:return rv;case 35665:return av;case 35666:return ov;case 35674:return lv;case 35675:return cv;case 35676:return hv;case 5124:case 35670:return uv;case 35667:case 35671:return dv;case 35668:case 35672:return fv;case 35669:case 35673:return pv;case 5125:return mv;case 36294:return gv;case 36295:return xv;case 36296:return bv;case 35678:case 36198:case 36298:case 36306:case 35682:return vv;case 35679:case 36299:case 36307:return _v;case 35680:case 36300:case 36308:case 36293:return yv;case 36289:case 36303:case 36311:case 36292:return Mv}}function wv(s,e){s.uniform1fv(this.addr,e)}function Ev(s,e){let t=jr(e,this.size,2);s.uniform2fv(this.addr,t)}function Tv(s,e){let t=jr(e,this.size,3);s.uniform3fv(this.addr,t)}function Av(s,e){let t=jr(e,this.size,4);s.uniform4fv(this.addr,t)}function Rv(s,e){let t=jr(e,this.size,4);s.uniformMatrix2fv(this.addr,!1,t)}function Cv(s,e){let t=jr(e,this.size,9);s.uniformMatrix3fv(this.addr,!1,t)}function Pv(s,e){let t=jr(e,this.size,16);s.uniformMatrix4fv(this.addr,!1,t)}function Iv(s,e){s.uniform1iv(this.addr,e)}function Dv(s,e){s.uniform2iv(this.addr,e)}function Lv(s,e){s.uniform3iv(this.addr,e)}function Fv(s,e){s.uniform4iv(this.addr,e)}function Nv(s,e){s.uniform1uiv(this.addr,e)}function Uv(s,e){s.uniform2uiv(this.addr,e)}function Ov(s,e){s.uniform3uiv(this.addr,e)}function zv(s,e){s.uniform4uiv(this.addr,e)}function kv(s,e,t){let n=this.cache,i=e.length,r=Sc(t,i);gn(n,r)||(s.uniform1iv(this.addr,r),xn(n,r));let a;this.type===s.SAMPLER_2D_SHADOW?a=nu:a=mp;for(let o=0;o!==i;++o)t.setTexture2D(e[o]||a,r[o])}function Bv(s,e,t){let n=this.cache,i=e.length,r=Sc(t,i);gn(n,r)||(s.uniform1iv(this.addr,r),xn(n,r));for(let a=0;a!==i;++a)t.setTexture3D(e[a]||xp,r[a])}function Hv(s,e,t){let n=this.cache,i=e.length,r=Sc(t,i);gn(n,r)||(s.uniform1iv(this.addr,r),xn(n,r));for(let a=0;a!==i;++a)t.setTextureCube(e[a]||bp,r[a])}function Gv(s,e,t){let n=this.cache,i=e.length,r=Sc(t,i);gn(n,r)||(s.uniform1iv(this.addr,r),xn(n,r));for(let a=0;a!==i;++a)t.setTexture2DArray(e[a]||gp,r[a])}function Vv(s){switch(s){case 5126:return wv;case 35664:return Ev;case 35665:return Tv;case 35666:return Av;case 35674:return Rv;case 35675:return Cv;case 35676:return Pv;case 5124:case 35670:return Iv;case 35667:case 35671:return Dv;case 35668:case 35672:return Lv;case 35669:case 35673:return Fv;case 5125:return Nv;case 36294:return Uv;case 36295:return Ov;case 36296:return zv;case 35678:case 36198:case 36298:case 36306:case 35682:return kv;case 35679:case 36299:case 36307:return Bv;case 35680:case 36300:case 36308:case 36293:return Hv;case 36289:case 36303:case 36311:case 36292:return Gv}}var iu=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.setValue=Sv(t.type)}},su=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.size=t.size,this.setValue=Vv(t.type)}},ru=class{constructor(e){this.id=e,this.seq=[],this.map={}}setValue(e,t,n){let i=this.seq;for(let r=0,a=i.length;r!==a;++r){let o=i[r];o.setValue(e,t[o.id],n)}}},eu=/(\w+)(\])?(\[|\.)?/g;function tp(s,e){s.seq.push(e),s.map[e.id]=e}function Wv(s,e,t){let n=s.name,i=n.length;for(eu.lastIndex=0;;){let r=eu.exec(n),a=eu.lastIndex,o=r[1],l=r[2]==="]",c=r[3];if(l&&(o=o|0),c===void 0||c==="["&&a+2===i){tp(t,c===void 0?new iu(o,s,e):new su(o,s,e));break}else{let u=t.map[o];u===void 0&&(u=new ru(o),tp(t,u)),t=u}}}var qr=class{constructor(e,t){this.seq=[],this.map={};let n=e.getProgramParameter(t,e.ACTIVE_UNIFORMS);for(let a=0;a<n;++a){let o=e.getActiveUniform(t,a),l=e.getUniformLocation(t,o.name);Wv(o,l,this)}let i=[],r=[];for(let a of this.seq)a.type===e.SAMPLER_2D_SHADOW||a.type===e.SAMPLER_CUBE_SHADOW||a.type===e.SAMPLER_2D_ARRAY_SHADOW?i.push(a):r.push(a);i.length>0&&(this.seq=i.concat(r))}setValue(e,t,n,i){let r=this.map[t];r!==void 0&&r.setValue(e,n,i)}setOptional(e,t,n){let i=t[n];i!==void 0&&this.setValue(e,n,i)}static upload(e,t,n,i){for(let r=0,a=t.length;r!==a;++r){let o=t[r],l=n[o.id];l.needsUpdate!==!1&&o.setValue(e,l.value,i)}}static seqWithValue(e,t){let n=[];for(let i=0,r=e.length;i!==r;++i){let a=e[i];a.id in t&&n.push(a)}return n}};function np(s,e,t){let n=s.createShader(e);return s.shaderSource(n,t),s.compileShader(n),n}var qv=37297,Xv=0;function jv(s,e){let t=s.split(`
`),n=[],i=Math.max(e-6,0),r=Math.min(e+6,t.length);for(let a=i;a<r;a++){let o=a+1;n.push(`${o===e?">":" "} ${o}: ${t[a]}`)}return n.join(`
`)}var ip=new xt;function Kv(s){Tt._getMatrix(ip,Tt.workingColorSpace,s);let e=`mat3( ${ip.elements.map(t=>t.toFixed(4))} )`;switch(Tt.getTransfer(s)){case ga:return[e,"LinearTransferOETF"];case Gt:return[e,"sRGBTransferOETF"];default:return rt("WebGLProgram: Unsupported color space: ",s),[e,"LinearTransferOETF"]}}function sp(s,e,t){let n=s.getShaderParameter(e,s.COMPILE_STATUS),r=(s.getShaderInfoLog(e)||"").trim();if(n&&r==="")return"";let a=/ERROR: 0:(\d+)/.exec(r);if(a){let o=parseInt(a[1]);return t.toUpperCase()+`

`+r+`

`+jv(s.getShaderSource(e),o)}else return r}function Yv(s,e){let t=Kv(e);return[`vec4 ${s}( vec4 value ) {`,`	return ${t[1]}( vec4( value.rgb * ${t[0]}, value.a ) );`,"}"].join(`
`)}var Jv={[Va]:"Linear",[Wa]:"Reinhard",[qa]:"Cineon",[Ws]:"ACESFilmic",[ja]:"AgX",[Ka]:"Neutral",[Xa]:"Custom"};function Zv(s,e){let t=Jv[e];return t===void 0?(rt("WebGLProgram: Unsupported toneMapping:",e),"vec3 "+s+"( vec3 color ) { return LinearToneMapping( color ); }"):"vec3 "+s+"( vec3 color ) { return "+t+"ToneMapping( color ); }"}var vc=new B;function $v(){Tt.getLuminanceCoefficients(vc);let s=vc.x.toFixed(4),e=vc.y.toFixed(4),t=vc.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${s}, ${e}, ${t} );`,"	return dot( weights, rgb );","}"].join(`
`)}function Qv(s){return[s.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",s.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(ao).join(`
`)}function e_(s){let e=[];for(let t in s){let n=s[t];n!==!1&&e.push("#define "+t+" "+n)}return e.join(`
`)}function t_(s,e){let t={},n=s.getProgramParameter(e,s.ACTIVE_ATTRIBUTES);for(let i=0;i<n;i++){let r=s.getActiveAttrib(e,i),a=r.name,o=1;r.type===s.FLOAT_MAT2&&(o=2),r.type===s.FLOAT_MAT3&&(o=3),r.type===s.FLOAT_MAT4&&(o=4),t[a]={type:r.type,location:s.getAttribLocation(e,a),locationSize:o}}return t}function ao(s){return s!==""}function rp(s,e){let t=e.numSpotLightShadows+e.numSpotLightMaps-e.numSpotLightShadowsWithMaps;return s.replace(/NUM_SUN_LIGHTS/g,e.numSunLights).replace(/NUM_DIR_LIGHTS/g,e.numDirLights).replace(/NUM_SPOT_LIGHTS/g,e.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,e.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,t).replace(/NUM_RECT_AREA_LIGHTS/g,e.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,e.numPointLights).replace(/NUM_HEMI_LIGHTS/g,e.numHemiLights).replace(/NUM_SUN_LIGHT_SHADOWS/g,e.numSunLightShadows).replace(/NUM_DIR_LIGHT_SHADOWS/g,e.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,e.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,e.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,e.numPointLightShadows)}function ap(s,e){return s.replace(/NUM_CLIPPING_PLANES/g,e.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,e.numClippingPlanes-e.numClipIntersection)}var n_=/^[ \t]*#include +<([\w\d./]+)>/gm;function au(s){return s.replace(n_,s_)}var i_=new Map;function s_(s,e){let t=Ct[e];if(t===void 0){let n=i_.get(e);if(n!==void 0)t=Ct[n],rt('WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',e,n);else throw new Error("THREE.WebGLProgram: Can not resolve #include <"+e+">")}return au(t)}var r_=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function op(s){return s.replace(r_,a_)}function a_(s,e,t,n){let i="";for(let r=parseInt(e);r<parseInt(t);r++)i+=n.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return i}function lp(s){let e=`precision ${s.precision} float;
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
#define LOW_PRECISION`),e}var o_={[Gs]:"SHADOWMAP_TYPE_PCF",[Or]:"SHADOWMAP_TYPE_VSM"};function l_(s){return o_[s.shadowMapType]||"SHADOWMAP_TYPE_BASIC"}var c_={[xs]:"ENVMAP_TYPE_CUBE",[qs]:"ENVMAP_TYPE_CUBE",[Ya]:"ENVMAP_TYPE_CUBE_UV"};function h_(s){return s.envMap===!1?"ENVMAP_TYPE_CUBE":c_[s.envMapMode]||"ENVMAP_TYPE_CUBE"}var u_={[qs]:"ENVMAP_MODE_REFRACTION"};function d_(s){return s.envMap===!1?"ENVMAP_MODE_REFLECTION":u_[s.envMapMode]||"ENVMAP_MODE_REFLECTION"}var f_={[Al]:"ENVMAP_BLENDING_MULTIPLY",[wf]:"ENVMAP_BLENDING_MIX",[Ef]:"ENVMAP_BLENDING_ADD"};function p_(s){return s.envMap===!1?"ENVMAP_BLENDING_NONE":f_[s.combine]||"ENVMAP_BLENDING_NONE"}function m_(s){let e=s.envMapCubeUVHeight;if(e===null)return null;let t=Math.log2(e)-2,n=1/e;return{texelWidth:1/(3*Math.max(Math.pow(2,t),112)),texelHeight:n,maxMip:t}}function g_(s,e,t,n){let i=s.getContext(),r=t.defines,a=t.vertexShader,o=t.fragmentShader,l=l_(t),c=h_(t),h=d_(t),u=p_(t),d=m_(t),p=Qv(t),x=e_(r),v=i.createProgram(),m,f,y=t.glslVersion?"#version "+t.glslVersion+`
`:"";t.isRawShaderMaterial?(m=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x].filter(ao).join(`
`),m.length>0&&(m+=`
`),f=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x].filter(ao).join(`
`),f.length>0&&(f+=`
`)):(m=[lp(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x,t.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",t.batching?"#define USE_BATCHING":"",t.batchingColor?"#define USE_BATCHING_COLOR":"",t.instancing?"#define USE_INSTANCING":"",t.instancingColor?"#define USE_INSTANCING_COLOR":"",t.instancingMorph?"#define USE_INSTANCING_MORPH":"",t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.map?"#define USE_MAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+h:"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.displacementMap?"#define USE_DISPLACEMENTMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.mapUv?"#define MAP_UV "+t.mapUv:"",t.alphaMapUv?"#define ALPHAMAP_UV "+t.alphaMapUv:"",t.lightMapUv?"#define LIGHTMAP_UV "+t.lightMapUv:"",t.aoMapUv?"#define AOMAP_UV "+t.aoMapUv:"",t.emissiveMapUv?"#define EMISSIVEMAP_UV "+t.emissiveMapUv:"",t.bumpMapUv?"#define BUMPMAP_UV "+t.bumpMapUv:"",t.normalMapUv?"#define NORMALMAP_UV "+t.normalMapUv:"",t.displacementMapUv?"#define DISPLACEMENTMAP_UV "+t.displacementMapUv:"",t.metalnessMapUv?"#define METALNESSMAP_UV "+t.metalnessMapUv:"",t.roughnessMapUv?"#define ROUGHNESSMAP_UV "+t.roughnessMapUv:"",t.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+t.anisotropyMapUv:"",t.clearcoatMapUv?"#define CLEARCOATMAP_UV "+t.clearcoatMapUv:"",t.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+t.clearcoatNormalMapUv:"",t.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+t.clearcoatRoughnessMapUv:"",t.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+t.iridescenceMapUv:"",t.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+t.iridescenceThicknessMapUv:"",t.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+t.sheenColorMapUv:"",t.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+t.sheenRoughnessMapUv:"",t.specularMapUv?"#define SPECULARMAP_UV "+t.specularMapUv:"",t.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+t.specularColorMapUv:"",t.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+t.specularIntensityMapUv:"",t.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+t.transmissionMapUv:"",t.thicknessMapUv?"#define THICKNESSMAP_UV "+t.thicknessMapUv:"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexNormals?"#define HAS_NORMAL":"",t.vertexColors?"#define USE_COLOR":"",t.vertexAlphas?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.flatShading?"#define FLAT_SHADED":"",t.skinning?"#define USE_SKINNING":"",t.morphTargets?"#define USE_MORPHTARGETS":"",t.morphNormals&&t.flatShading===!1?"#define USE_MORPHNORMALS":"",t.morphColors?"#define USE_MORPHCOLORS":"",t.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+t.morphTextureStride:"",t.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+t.morphTargetsCount:"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.sizeAttenuation?"#define USE_SIZEATTENUATION":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(ao).join(`
`),f=[lp(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x,t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",t.map?"#define USE_MAP":"",t.matcap?"#define USE_MATCAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+c:"",t.envMap?"#define "+h:"",t.envMap?"#define "+u:"",d?"#define CUBEUV_TEXEL_WIDTH "+d.texelWidth:"",d?"#define CUBEUV_TEXEL_HEIGHT "+d.texelHeight:"",d?"#define CUBEUV_MAX_MIP "+d.maxMip+".0":"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.packedNormalMap?"#define USE_PACKED_NORMALMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoat?"#define USE_CLEARCOAT":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.dispersion?"#define USE_DISPERSION":"",t.retroreflection?"#define USE_RETROREFLECTION":"",t.iridescence?"#define USE_IRIDESCENCE":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaTest?"#define USE_ALPHATEST":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.sheen?"#define USE_SHEEN":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexColors||t.instancingColor?"#define USE_COLOR":"",t.vertexAlphas||t.batchingColor?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.gradientMap?"#define USE_GRADIENTMAP":"",t.flatShading?"#define FLAT_SHADED":"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.numLightProbeGrids>0?"#define USE_LIGHT_PROBES_GRID":"",t.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",t.decodeVideoTextureEmissive?"#define DECODE_VIDEO_TEXTURE_EMISSIVE":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",t.toneMapping!==xi?"#define TONE_MAPPING":"",t.toneMapping!==xi?Ct.tonemapping_pars_fragment:"",t.toneMapping!==xi?Zv("toneMapping",t.toneMapping):"",t.dithering?"#define DITHERING":"",t.opaque?"#define OPAQUE":"",Ct.colorspace_pars_fragment,Yv("linearToOutputTexel",t.outputColorSpace),$v(),t.useDepthPacking?"#define DEPTH_PACKING "+t.depthPacking:"",`
`].filter(ao).join(`
`)),a=au(a),a=rp(a,t),a=ap(a,t),o=au(o),o=rp(o,t),o=ap(o,t),a=op(a),o=op(o),t.isRawShaderMaterial!==!0&&(y=`#version 300 es
`,m=[p,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+m,f=["#define varying in",t.glslVersion===Vh?"":"layout(location = 0) out highp vec4 pc_fragColor;",t.glslVersion===Vh?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+f);let T=y+m+a,g=y+f+o,_=np(i,i.VERTEX_SHADER,T),w=np(i,i.FRAGMENT_SHADER,g);i.attachShader(v,_),i.attachShader(v,w),t.index0AttributeName!==void 0?i.bindAttribLocation(v,0,t.index0AttributeName):t.hasPositionAttribute===!0&&i.bindAttribLocation(v,0,"position"),i.linkProgram(v);function A(P){if(s.debug.checkShaderErrors){let M=i.getProgramInfoLog(v)||"",F=i.getShaderInfoLog(_)||"",I=i.getShaderInfoLog(w)||"",N=M.trim(),H=F.trim(),D=I.trim(),V=!0,k=!0;if(i.getProgramParameter(v,i.LINK_STATUS)===!1)if(V=!1,typeof s.debug.onShaderError=="function")s.debug.onShaderError(i,v,_,w);else{let E=sp(i,_,"vertex"),Z=sp(i,w,"fragment");pt("WebGLProgram: Shader Error "+i.getError()+" - VALIDATE_STATUS "+i.getProgramParameter(v,i.VALIDATE_STATUS)+`

Material Name: `+P.name+`
Material Type: `+P.type+`

Program Info Log: `+N+`
`+E+`
`+Z)}else N!==""?rt("WebGLProgram: Program Info Log:",N):(H===""||D==="")&&(k=!1);k&&(P.diagnostics={runnable:V,programLog:N,vertexShader:{log:H,prefix:m},fragmentShader:{log:D,prefix:f}})}i.deleteShader(_),i.deleteShader(w),b=new qr(i,v),C=t_(i,v)}let b;this.getUniforms=function(){return b===void 0&&A(this),b};let C;this.getAttributes=function(){return C===void 0&&A(this),C};let S=t.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return S===!1&&(S=i.getProgramParameter(v,qv)),S},this.destroy=function(){n.releaseStatesOfProgram(this),i.deleteProgram(v),this.program=void 0},this.type=t.shaderType,this.name=t.shaderName,this.id=Xv++,this.cacheKey=e,this.usedTimes=1,this.program=v,this.vertexShader=_,this.fragmentShader=w,this}var x_=0,ou=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(e,t,n){let i=this._getShaderCacheForMaterial(e);return i.has(t)===!1&&(i.add(t),t.usedTimes++),i.has(n)===!1&&(i.add(n),n.usedTimes++),this}remove(e){let t=this.materialCache.get(e);for(let n of t)n.usedTimes--,n.usedTimes===0&&this.shaderCache.delete(n.code);return this.materialCache.delete(e),this}getVertexShaderStage(e){return this._getShaderStage(e.vertexShader)}getFragmentShaderStage(e){return this._getShaderStage(e.fragmentShader)}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(e){let t=this.materialCache,n=t.get(e);return n===void 0&&(n=new Set,t.set(e,n)),n}_getShaderStage(e){let t=this.shaderCache,n=t.get(e);return n===void 0&&(n=new lu(e),t.set(e,n)),n}},lu=class{constructor(e){this.id=x_++,this.code=e,this.usedTimes=0}};function b_(s){return s===vs||s===eo||s===to}function v_(s,e,t,n,i,r){let a=new Mr,o=new ou,l=new Set,c=[],h=new Map,u=n.logarithmicDepthBuffer,d=n.precision,p={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distance",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function x(b){return l.add(b),b===0?"uv":`uv${b}`}function v(b,C,S,P,M,F){let I=P.fog,N=M.geometry,H=b.isMeshStandardMaterial||b.isMeshLambertMaterial||b.isMeshPhongMaterial?P.environment:null,D=b.isMeshStandardMaterial||b.isMeshLambertMaterial&&!b.envMap||b.isMeshPhongMaterial&&!b.envMap,V=e.get(b.envMap||H,D),k=V&&V.mapping===Ya?V.image.height:null,E=p[b.type];b.precision!==null&&(d=n.getMaxPrecision(b.precision),d!==b.precision&&rt("WebGLProgram.getParameters:",b.precision,"not supported, using",d,"instead."));let Z=N.morphAttributes.position||N.morphAttributes.normal||N.morphAttributes.color,W=Z!==void 0?Z.length:0,Q=0;N.morphAttributes.position!==void 0&&(Q=1),N.morphAttributes.normal!==void 0&&(Q=2),N.morphAttributes.color!==void 0&&(Q=3);let Fe,Ne,ze,se;if(E){let U=Ni[E];Fe=U.vertexShader,Ne=U.fragmentShader}else{Fe=b.vertexShader,Ne=b.fragmentShader;let U=o.getVertexShaderStage(b),q=o.getFragmentShaderStage(b);o.update(b,U,q),ze=U.id,se=q.id}let le=s.getRenderTarget(),De=s.state.buffers.depth.getReversed(),be=M.isInstancedMesh===!0,ue=M.isBatchedMesh===!0,te=!!b.map,pe=!!b.matcap,ce=!!V,ne=!!b.aoMap,de=!!b.lightMap,we=!!b.bumpMap&&b.wireframe===!1,Ve=!!b.normalMap,Ke=!!b.displacementMap,it=!!b.emissiveMap,et=!!b.metalnessMap,ht=!!b.roughnessMap,J=b.anisotropy>0,Ut=b.clearcoat>0,ut=b.dispersion>0,O=b.retroreflectivity>0,R=b.iridescence>0,re=b.sheen>0,oe=b.transmission>0,xe=J&&!!b.anisotropyMap,Ue=Ut&&!!b.clearcoatMap,ke=Ut&&!!b.clearcoatNormalMap,G=Ut&&!!b.clearcoatRoughnessMap,j=R&&!!b.iridescenceMap,he=R&&!!b.iridescenceThicknessMap,Se=re&&!!b.sheenColorMap,ie=re&&!!b.sheenRoughnessMap,ve=!!b.specularMap,Ee=!!b.specularColorMap,Te=!!b.specularIntensityMap,Ie=oe&&!!b.transmissionMap,X=oe&&!!b.thicknessMap,Ae=!!b.gradientMap,Me=!!b.alphaMap,fe=b.alphaTest>0,Pe=!!b.alphaHash,_e=!!b.extensions,qe=xi;b.toneMapped&&(le===null||le.isXRRenderTarget===!0)&&(qe=s.toneMapping);let Xe={shaderID:E,shaderType:b.type,shaderName:b.name,vertexShader:Fe,fragmentShader:Ne,defines:b.defines,customVertexShaderID:ze,customFragmentShaderID:se,isRawShaderMaterial:b.isRawShaderMaterial===!0,glslVersion:b.glslVersion,precision:d,batching:ue,batchingColor:ue&&M._colorsTexture!==null,instancing:be,instancingColor:be&&M.instanceColor!==null,instancingMorph:be&&M.morphTexture!==null,outputColorSpace:le===null?s.outputColorSpace:le.isXRRenderTarget===!0?le.texture.colorSpace:Tt.workingColorSpace,alphaToCoverage:!!b.alphaToCoverage,map:te,matcap:pe,envMap:ce,envMapMode:ce&&V.mapping,envMapCubeUVHeight:k,aoMap:ne,lightMap:de,bumpMap:we,normalMap:Ve,displacementMap:Ke,emissiveMap:it,normalMapObjectSpace:Ve&&b.normalMapType===If,normalMapTangentSpace:Ve&&b.normalMapType===io,packedNormalMap:Ve&&b.normalMapType===io&&b_(b.normalMap.format),metalnessMap:et,roughnessMap:ht,anisotropy:J,anisotropyMap:xe,clearcoat:Ut,clearcoatMap:Ue,clearcoatNormalMap:ke,clearcoatRoughnessMap:G,dispersion:ut,retroreflection:O,iridescence:R,iridescenceMap:j,iridescenceThicknessMap:he,sheen:re,sheenColorMap:Se,sheenRoughnessMap:ie,specularMap:ve,specularColorMap:Ee,specularIntensityMap:Te,transmission:oe,transmissionMap:Ie,thicknessMap:X,gradientMap:Ae,opaque:b.transparent===!1&&b.blending===zr&&b.alphaToCoverage===!1,alphaMap:Me,alphaTest:fe,alphaHash:Pe,combine:b.combine,mapUv:te&&x(b.map.channel),aoMapUv:ne&&x(b.aoMap.channel),lightMapUv:de&&x(b.lightMap.channel),bumpMapUv:we&&x(b.bumpMap.channel),normalMapUv:Ve&&x(b.normalMap.channel),displacementMapUv:Ke&&x(b.displacementMap.channel),emissiveMapUv:it&&x(b.emissiveMap.channel),metalnessMapUv:et&&x(b.metalnessMap.channel),roughnessMapUv:ht&&x(b.roughnessMap.channel),anisotropyMapUv:xe&&x(b.anisotropyMap.channel),clearcoatMapUv:Ue&&x(b.clearcoatMap.channel),clearcoatNormalMapUv:ke&&x(b.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:G&&x(b.clearcoatRoughnessMap.channel),iridescenceMapUv:j&&x(b.iridescenceMap.channel),iridescenceThicknessMapUv:he&&x(b.iridescenceThicknessMap.channel),sheenColorMapUv:Se&&x(b.sheenColorMap.channel),sheenRoughnessMapUv:ie&&x(b.sheenRoughnessMap.channel),specularMapUv:ve&&x(b.specularMap.channel),specularColorMapUv:Ee&&x(b.specularColorMap.channel),specularIntensityMapUv:Te&&x(b.specularIntensityMap.channel),transmissionMapUv:Ie&&x(b.transmissionMap.channel),thicknessMapUv:X&&x(b.thicknessMap.channel),alphaMapUv:Me&&x(b.alphaMap.channel),vertexTangents:!!N.attributes.tangent&&(Ve||J),vertexNormals:!!N.attributes.normal,vertexColors:b.vertexColors,vertexAlphas:b.vertexColors===!0&&!!N.attributes.color&&N.attributes.color.itemSize===4,pointsUvs:M.isPoints===!0&&!!N.attributes.uv&&(te||Me),fog:!!I,useFog:b.fog===!0,fogExp2:!!I&&I.isFogExp2,flatShading:b.wireframe===!1&&(b.flatShading===!0||N.attributes.normal===void 0&&Ve===!1&&(b.isMeshLambertMaterial||b.isMeshPhongMaterial||b.isMeshStandardMaterial||b.isMeshPhysicalMaterial)),sizeAttenuation:b.sizeAttenuation===!0,logarithmicDepthBuffer:u,reversedDepthBuffer:De,skinning:M.isSkinnedMesh===!0,hasPositionAttribute:N.attributes.position!==void 0,morphTargets:N.morphAttributes.position!==void 0,morphNormals:N.morphAttributes.normal!==void 0,morphColors:N.morphAttributes.color!==void 0,morphTargetsCount:W,morphTextureStride:Q,numSunLights:C.sun.length,numDirLights:C.directional.length,numPointLights:C.point.length,numSpotLights:C.spot.length,numSpotLightMaps:C.spotLightMap.length,numRectAreaLights:C.rectArea.length,numHemiLights:C.hemi.length,numSunLightShadows:C.sunShadowMap.length,numDirLightShadows:C.directionalShadowMap.length,numPointLightShadows:C.pointShadowMap.length,numSpotLightShadows:C.spotShadowMap.length,numSpotLightShadowsWithMaps:C.numSpotLightShadowsWithMaps,numLightProbes:C.numLightProbes,numLightProbeGrids:F.length,numClippingPlanes:r.numPlanes,numClipIntersection:r.numIntersection,dithering:b.dithering,shadowMapEnabled:s.shadowMap.enabled&&S.length>0,shadowMapType:s.shadowMap.type,toneMapping:qe,decodeVideoTexture:te&&b.map.isVideoTexture===!0&&Tt.getTransfer(b.map.colorSpace)===Gt,decodeVideoTextureEmissive:it&&b.emissiveMap.isVideoTexture===!0&&Tt.getTransfer(b.emissiveMap.colorSpace)===Gt,premultipliedAlpha:b.premultipliedAlpha,doubleSided:b.side===Kt,flipSided:b.side===mn,useDepthPacking:b.depthPacking>=0,depthPacking:b.depthPacking||0,index0AttributeName:b.index0AttributeName,extensionClipCullDistance:_e&&b.extensions.clipCullDistance===!0&&t.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(_e&&b.extensions.multiDraw===!0||ue)&&t.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:t.has("KHR_parallel_shader_compile"),customProgramCacheKey:b.customProgramCacheKey()};return Xe.vertexUv1s=l.has(1),Xe.vertexUv2s=l.has(2),Xe.vertexUv3s=l.has(3),l.clear(),Xe}function m(b){let C=[];if(b.shaderID?C.push(b.shaderID):(C.push(b.customVertexShaderID),C.push(b.customFragmentShaderID)),b.defines!==void 0)for(let S in b.defines)C.push(S),C.push(b.defines[S]);return b.isRawShaderMaterial===!1&&(f(C,b),y(C,b),C.push(s.outputColorSpace)),C.push(b.customProgramCacheKey),C.join()}function f(b,C){b.push(C.precision),b.push(C.outputColorSpace),b.push(C.envMapMode),b.push(C.envMapCubeUVHeight),b.push(C.mapUv),b.push(C.alphaMapUv),b.push(C.lightMapUv),b.push(C.aoMapUv),b.push(C.bumpMapUv),b.push(C.normalMapUv),b.push(C.displacementMapUv),b.push(C.emissiveMapUv),b.push(C.metalnessMapUv),b.push(C.roughnessMapUv),b.push(C.anisotropyMapUv),b.push(C.clearcoatMapUv),b.push(C.clearcoatNormalMapUv),b.push(C.clearcoatRoughnessMapUv),b.push(C.iridescenceMapUv),b.push(C.iridescenceThicknessMapUv),b.push(C.sheenColorMapUv),b.push(C.sheenRoughnessMapUv),b.push(C.specularMapUv),b.push(C.specularColorMapUv),b.push(C.specularIntensityMapUv),b.push(C.transmissionMapUv),b.push(C.thicknessMapUv),b.push(C.combine),b.push(C.fogExp2),b.push(C.sizeAttenuation),b.push(C.morphTargetsCount),b.push(C.morphAttributeCount),b.push(C.numSunLights),b.push(C.numDirLights),b.push(C.numPointLights),b.push(C.numSpotLights),b.push(C.numSpotLightMaps),b.push(C.numHemiLights),b.push(C.numRectAreaLights),b.push(C.numSunLightShadows),b.push(C.numDirLightShadows),b.push(C.numPointLightShadows),b.push(C.numSpotLightShadows),b.push(C.numSpotLightShadowsWithMaps),b.push(C.numLightProbes),b.push(C.shadowMapType),b.push(C.toneMapping),b.push(C.numClippingPlanes),b.push(C.numClipIntersection),b.push(C.depthPacking)}function y(b,C){a.disableAll(),C.instancing&&a.enable(0),C.instancingColor&&a.enable(1),C.instancingMorph&&a.enable(2),C.matcap&&a.enable(3),C.envMap&&a.enable(4),C.normalMapObjectSpace&&a.enable(5),C.normalMapTangentSpace&&a.enable(6),C.clearcoat&&a.enable(7),C.iridescence&&a.enable(8),C.alphaTest&&a.enable(9),C.vertexColors&&a.enable(10),C.vertexAlphas&&a.enable(11),C.vertexUv1s&&a.enable(12),C.vertexUv2s&&a.enable(13),C.vertexUv3s&&a.enable(14),C.vertexTangents&&a.enable(15),C.anisotropy&&a.enable(16),C.alphaHash&&a.enable(17),C.batching&&a.enable(18),C.dispersion&&a.enable(19),C.retroreflection&&a.enable(24),C.batchingColor&&a.enable(20),C.gradientMap&&a.enable(21),C.packedNormalMap&&a.enable(22),C.vertexNormals&&a.enable(23),b.push(a.mask),a.disableAll(),C.fog&&a.enable(0),C.useFog&&a.enable(1),C.flatShading&&a.enable(2),C.logarithmicDepthBuffer&&a.enable(3),C.reversedDepthBuffer&&a.enable(4),C.skinning&&a.enable(5),C.morphTargets&&a.enable(6),C.morphNormals&&a.enable(7),C.morphColors&&a.enable(8),C.premultipliedAlpha&&a.enable(9),C.shadowMapEnabled&&a.enable(10),C.doubleSided&&a.enable(11),C.flipSided&&a.enable(12),C.useDepthPacking&&a.enable(13),C.dithering&&a.enable(14),C.transmission&&a.enable(15),C.sheen&&a.enable(16),C.opaque&&a.enable(17),C.pointsUvs&&a.enable(18),C.decodeVideoTexture&&a.enable(19),C.decodeVideoTextureEmissive&&a.enable(20),C.alphaToCoverage&&a.enable(21),C.numLightProbeGrids>0&&a.enable(22),C.hasPositionAttribute&&a.enable(23),b.push(a.mask)}function T(b){let C=p[b.type],S;if(C){let P=Ni[C];S=_i.clone(P.uniforms)}else S=b.uniforms;return S}function g(b,C){let S=h.get(C);return S!==void 0?++S.usedTimes:(S=new g_(s,C,b,i),c.push(S),h.set(C,S)),S}function _(b){if(--b.usedTimes===0){let C=c.indexOf(b);c[C]=c[c.length-1],c.pop(),h.delete(b.cacheKey),b.destroy()}}function w(b){o.remove(b)}function A(){o.dispose()}return{getParameters:v,getProgramCacheKey:m,getUniforms:T,acquireProgram:g,releaseProgram:_,releaseShaderCache:w,programs:c,dispose:A}}function __(){let s=new WeakMap;function e(a){return s.has(a)}function t(a){let o=s.get(a);return o===void 0&&(o={},s.set(a,o)),o}function n(a){s.delete(a)}function i(a,o,l){s.get(a)[o]=l}function r(){s=new WeakMap}return{has:e,get:t,remove:n,update:i,dispose:r}}function y_(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.material.id!==e.material.id?s.material.id-e.material.id:s.materialVariant!==e.materialVariant?s.materialVariant-e.materialVariant:s.z!==e.z?s.z-e.z:s.id-e.id}function cp(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.z!==e.z?e.z-s.z:s.id-e.id}function hp(){let s=[],e=0,t=[],n=[],i=[];function r(){e=0,t.length=0,n.length=0,i.length=0}function a(d){let p=0;return d.isInstancedMesh&&(p+=2),d.isSkinnedMesh&&(p+=1),p}function o(d,p,x,v,m,f){let y=s[e];return y===void 0?(y={id:d.id,object:d,geometry:p,material:x,materialVariant:a(d),groupOrder:v,renderOrder:d.renderOrder,z:m,group:f},s[e]=y):(y.id=d.id,y.object=d,y.geometry=p,y.material=x,y.materialVariant=a(d),y.groupOrder=v,y.renderOrder=d.renderOrder,y.z=m,y.group=f),e++,y}function l(d,p,x,v,m,f,y){y.reversedDepth===!0&&(m=-m);let T=o(d,p,x,v,m,f);x.transmission>0?n.push(T):x.transparent===!0?i.push(T):t.push(T)}function c(d,p,x,v,m,f){let y=o(d,p,x,v,m,f);x.transmission>0?n.unshift(y):x.transparent===!0?i.unshift(y):t.unshift(y)}function h(d,p){t.length>1&&t.sort(d||y_),n.length>1&&n.sort(p||cp),i.length>1&&i.sort(p||cp)}function u(){for(let d=e,p=s.length;d<p;d++){let x=s[d];if(x.id===null)break;x.id=null,x.object=null,x.geometry=null,x.material=null,x.group=null}}return{opaque:t,transmissive:n,transparent:i,init:r,push:l,unshift:c,finish:u,sort:h}}function M_(){let s=new WeakMap;function e(n,i){let r=s.get(n),a;return r===void 0?(a=new hp,s.set(n,[a])):i>=r.length?(a=new hp,r.push(a)):a=r[i],a}function t(){s=new WeakMap}return{get:e,dispose:t}}function S_(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"SunLight":case"DirectionalLight":t={direction:new B,color:new Le};break;case"SpotLight":t={position:new B,direction:new B,color:new Le,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":t={position:new B,color:new Le,distance:0,decay:0};break;case"HemisphereLight":t={direction:new B,skyColor:new Le,groundColor:new Le};break;case"RectAreaLight":t={color:new Le,position:new B,halfWidth:new B,halfHeight:new B};break}return s[e.id]=t,t}}}function w_(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"SunLight":case"DirectionalLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Be};break;case"SpotLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Be};break;case"PointLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Be,shadowCameraNear:1,shadowCameraFar:1e3};break}return s[e.id]=t,t}}}var E_=0;function T_(s,e){return(e.castShadow?2:0)-(s.castShadow?2:0)+(e.map?1:0)-(s.map?1:0)}function A_(s){let e=new S_,t=w_(),n={version:0,hash:{sunLength:-1,directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numSunShadows:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],sun:[],sunShadow:[],sunShadowMap:[],sunShadowMatrix:[],sunShadowCascade:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let c=0;c<9;c++)n.probe.push(new B);let i=new B,r=new ft,a=new ft;function o(c){let h=0,u=0,d=0;for(let M=0;M<9;M++)n.probe[M].set(0,0,0);let p=0,x=0,v=0,m=0,f=0,y=0,T=0,g=0,_=0,w=0,A=0,b=0,C=0,S=0;c.sort(T_);for(let M=0,F=c.length;M<F;M++){let I=c[M],N=I.color,H=I.intensity,D=I.distance,V=null;if(I.shadow&&I.shadow.map&&(I.shadow.map.texture.format===vs?V=I.shadow.map.texture:V=I.shadow.map.depthTexture||I.shadow.map.texture),I.isAmbientLight)h+=N.r*H,u+=N.g*H,d+=N.b*H;else if(I.isLightProbe){for(let k=0;k<9;k++)n.probe[k].addScaledVector(I.sh.coefficients[k],H);S++}else if(I.isSunLight){let k=e.get(I);if(k.color.copy(I.color).multiplyScalar(I.intensity),I.castShadow){let E=I.shadow,Z=t.get(I);Z.shadowIntensity=E.intensity,Z.shadowBias=E.bias,Z.shadowNormalBias=E.normalBias,Z.shadowRadius=E.radius,Z.shadowMapSize.copy(E.mapSize).multiply(E.getFrameExtents()),n.sunShadow[x]=Z,n.sunShadowMap[x]=V;let W=E.getViewportCount();for(let Q=0;Q<W;Q++)n.sunShadowMatrix[v+Q]=E.getMatrix(Q),n.sunShadowCascade[v+Q]=E._cascadeData[Q];v+=W,x++}n.sun[p]=k,p++}else if(I.isDirectionalLight){let k=e.get(I);if(k.color.copy(I.color).multiplyScalar(I.intensity),I.castShadow){let E=I.shadow,Z=t.get(I);Z.shadowIntensity=E.intensity,Z.shadowBias=E.bias,Z.shadowNormalBias=E.normalBias,Z.shadowRadius=E.radius,Z.shadowMapSize=E.mapSize,n.directionalShadow[m]=Z,n.directionalShadowMap[m]=V,n.directionalShadowMatrix[m]=I.shadow.matrix,_++}n.directional[m]=k,m++}else if(I.isSpotLight){let k=e.get(I);k.position.setFromMatrixPosition(I.matrixWorld),k.color.copy(N).multiplyScalar(H),k.distance=D,k.coneCos=Math.cos(I.angle),k.penumbraCos=Math.cos(I.angle*(1-I.penumbra)),k.decay=I.decay,n.spot[y]=k;let E=I.shadow;if(I.map&&(n.spotLightMap[b]=I.map,b++,E.updateMatrices(I),I.castShadow&&C++),n.spotLightMatrix[y]=E.matrix,I.castShadow){let Z=t.get(I);Z.shadowIntensity=E.intensity,Z.shadowBias=E.bias,Z.shadowNormalBias=E.normalBias,Z.shadowRadius=E.radius,Z.shadowMapSize=E.mapSize,n.spotShadow[y]=Z,n.spotShadowMap[y]=V,A++}y++}else if(I.isRectAreaLight){let k=e.get(I);k.color.copy(N).multiplyScalar(H),k.halfWidth.set(I.width*.5,0,0),k.halfHeight.set(0,I.height*.5,0),n.rectArea[T]=k,T++}else if(I.isPointLight){let k=e.get(I);if(k.color.copy(I.color).multiplyScalar(I.intensity),k.distance=I.distance,k.decay=I.decay,I.castShadow){let E=I.shadow,Z=t.get(I);Z.shadowIntensity=E.intensity,Z.shadowBias=E.bias,Z.shadowNormalBias=E.normalBias,Z.shadowRadius=E.radius,Z.shadowMapSize=E.mapSize,Z.shadowCameraNear=E.camera.near,Z.shadowCameraFar=E.camera.far,n.pointShadow[f]=Z,n.pointShadowMap[f]=V,n.pointShadowMatrix[f]=I.shadow.matrix,w++}n.point[f]=k,f++}else if(I.isHemisphereLight){let k=e.get(I);k.skyColor.copy(I.color).multiplyScalar(H),k.groundColor.copy(I.groundColor).multiplyScalar(H),n.hemi[g]=k,g++}}T>0&&(s.has("OES_texture_float_linear")===!0?(n.rectAreaLTC1=Je.LTC_FLOAT_1,n.rectAreaLTC2=Je.LTC_FLOAT_2):(n.rectAreaLTC1=Je.LTC_HALF_1,n.rectAreaLTC2=Je.LTC_HALF_2)),n.ambient[0]=h,n.ambient[1]=u,n.ambient[2]=d;let P=n.hash;(P.sunLength!==p||P.directionalLength!==m||P.pointLength!==f||P.spotLength!==y||P.rectAreaLength!==T||P.hemiLength!==g||P.numSunShadows!==x||P.numDirectionalShadows!==_||P.numPointShadows!==w||P.numSpotShadows!==A||P.numSpotMaps!==b||P.numLightProbes!==S)&&(n.sun.length=p,n.directional.length=m,n.spot.length=y,n.rectArea.length=T,n.point.length=f,n.hemi.length=g,n.sunShadow.length=x,n.sunShadowMap.length=x,n.sunShadowMatrix.length=v,n.sunShadowCascade.length=v,n.directionalShadow.length=_,n.directionalShadowMap.length=_,n.directionalShadowMatrix.length=_,n.pointShadow.length=w,n.pointShadowMap.length=w,n.pointShadowMatrix.length=w,n.spotShadow.length=A,n.spotShadowMap.length=A,n.spotLightMatrix.length=A+b-C,n.spotLightMap.length=b,n.numSpotLightShadowsWithMaps=C,n.numLightProbes=S,P.sunLength=p,P.directionalLength=m,P.pointLength=f,P.spotLength=y,P.rectAreaLength=T,P.hemiLength=g,P.numSunShadows=x,P.numDirectionalShadows=_,P.numPointShadows=w,P.numSpotShadows=A,P.numSpotMaps=b,P.numLightProbes=S,n.version=E_++)}function l(c,h){let u=0,d=0,p=0,x=0,v=0,m=0,f=h.matrixWorldInverse;for(let y=0,T=c.length;y<T;y++){let g=c[y];if(g.isSunLight){let _=n.sun[u];_.direction.setFromMatrixPosition(g.matrixWorld),_.direction.transformDirection(f),u++}else if(g.isDirectionalLight){let _=n.directional[d];_.direction.setFromMatrixPosition(g.matrixWorld),i.setFromMatrixPosition(g.target.matrixWorld),_.direction.sub(i),_.direction.transformDirection(f),d++}else if(g.isSpotLight){let _=n.spot[x];_.position.setFromMatrixPosition(g.matrixWorld),_.position.applyMatrix4(f),_.direction.setFromMatrixPosition(g.matrixWorld),i.setFromMatrixPosition(g.target.matrixWorld),_.direction.sub(i),_.direction.transformDirection(f),x++}else if(g.isRectAreaLight){let _=n.rectArea[v];_.position.setFromMatrixPosition(g.matrixWorld),_.position.applyMatrix4(f),a.identity(),r.copy(g.matrixWorld),r.premultiply(f),a.extractRotation(r),_.halfWidth.set(g.width*.5,0,0),_.halfHeight.set(0,g.height*.5,0),_.halfWidth.applyMatrix4(a),_.halfHeight.applyMatrix4(a),v++}else if(g.isPointLight){let _=n.point[p];_.position.setFromMatrixPosition(g.matrixWorld),_.position.applyMatrix4(f),p++}else if(g.isHemisphereLight){let _=n.hemi[m];_.direction.setFromMatrixPosition(g.matrixWorld),_.direction.transformDirection(f),m++}}}return{setup:o,setupView:l,state:n}}function up(s){let e=new A_(s),t=[],n=[],i=[];function r(d){u.camera=d,t.length=0,n.length=0,i.length=0}function a(d){t.push(d)}function o(d){n.push(d)}function l(d){i.push(d)}function c(){e.setup(t)}function h(d){e.setupView(t,d)}let u={lightsArray:t,shadowsArray:n,lightProbeGridArray:i,camera:null,lights:e,transmissionRenderTarget:{},textureUnits:0};return{init:r,state:u,setupLights:c,setupLightsView:h,pushLight:a,pushShadow:o,pushLightProbeGrid:l}}function R_(s){let e=new WeakMap;function t(i,r=0){let a=e.get(i),o;return a===void 0?(o=new up(s),e.set(i,[o])):r>=a.length?(o=new up(s),a.push(o)):o=a[r],o}function n(){e=new WeakMap}return{get:t,dispose:n}}var C_=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,P_=`uniform sampler2D shadow_pass;
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
}`,I_=[new B(1,0,0),new B(-1,0,0),new B(0,1,0),new B(0,-1,0),new B(0,0,1),new B(0,0,-1)],D_=[new B(0,-1,0),new B(0,-1,0),new B(0,0,1),new B(0,0,-1),new B(0,-1,0),new B(0,-1,0)],dp=new ft,ro=new B,tu=new B;function L_(s,e,t){let n=new Ar,i=new Be,r=new Be,a=new Ot,o=new ml,l=new gl,c={},h=t.maxTextureSize,u={[ti]:mn,[mn]:ti,[Kt]:Kt},d=new yt({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new Be},radius:{value:4}},vertexShader:C_,fragmentShader:P_}),p=d.clone();p.defines.HORIZONTAL_PASS=1;let x=new _t;x.setAttribute("position",new Rt(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let v=new st(x,d),m=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=Gs;let f=this.type;this.render=function(w,A,b){if(m.enabled===!1||m.autoUpdate===!1&&m.needsUpdate===!1||w.length===0)return;this.type===rf&&(rt("WebGLShadowMap: PCFSoftShadowMap has been removed. Using PCFShadowMap instead."),this.type=Gs);let C=s.getRenderTarget(),S=s.getActiveCubeFace(),P=s.getActiveMipmapLevel(),M=s.state;M.setBlending(ni),M.buffers.depth.getReversed()===!0?M.buffers.color.setClear(0,0,0,0):M.buffers.color.setClear(1,1,1,1),M.buffers.depth.setTest(!0),M.setScissorTest(!1);let F=f!==this.type;F&&A.traverse(function(I){I.material&&(Array.isArray(I.material)?I.material.forEach(N=>N.needsUpdate=!0):I.material.needsUpdate=!0)});for(let I=0,N=w.length;I<N;I++){let H=w[I],D=H.shadow;if(D===void 0){rt("WebGLShadowMap:",H,"has no shadow.");continue}if(D.autoUpdate===!1&&D.needsUpdate===!1)continue;i.copy(D.mapSize);let V=D.getFrameExtents();i.multiply(V),r.copy(D.mapSize),(i.x>h||i.y>h)&&(i.x>h&&(r.x=Math.floor(h/V.x),i.x=r.x*V.x,D.mapSize.x=r.x),i.y>h&&(r.y=Math.floor(h/V.y),i.y=r.y*V.y,D.mapSize.y=r.y));let k=s.state.buffers.depth.getReversed();if(D.camera._reversedDepth=k,D.map===null||F===!0){if(D.map!==null&&(D.map.depthTexture!==null&&(D.map.depthTexture.dispose(),D.map.depthTexture=null),D.map.dispose()),this.type===Or){if(H.isPointLight){rt("WebGLShadowMap: VSM shadow maps are not supported for PointLights. Use PCF or BasicShadowMap instead.");continue}D.map=new an(i.x,i.y,{format:vs,type:pn,minFilter:rn,magFilter:rn,generateMipmaps:!1}),D.map.texture.name=H.name+".shadowMap",D.map.depthTexture=new us(i.x,i.y,Yn),D.map.depthTexture.name=H.name+".shadowMapDepth",D.map.depthTexture.format=Ai,D.map.depthTexture.compareFunction=null,D.map.depthTexture.minFilter=on,D.map.depthTexture.magFilter=on}else H.isPointLight?(D.map=new _c(i.x),D.map.depthTexture=new ol(i.x,vi)):(D.map=new an(i.x,i.y),D.map.depthTexture=new us(i.x,i.y,vi)),D.map.depthTexture.name=H.name+".shadowMap",D.map.depthTexture.format=Ai,this.type===Gs?(D.map.depthTexture.compareFunction=k?xc:gc,D.map.depthTexture.minFilter=rn,D.map.depthTexture.magFilter=rn):(D.map.depthTexture.compareFunction=null,D.map.depthTexture.minFilter=on,D.map.depthTexture.magFilter=on);D.camera.updateProjectionMatrix()}D.map.isWebGLCubeRenderTarget!==!0&&(D.map.width!==i.x||D.map.height!==i.y)&&D.map.setSize(i.x,i.y);let E=D.map.isWebGLCubeRenderTarget?6:D.getViewportCount();H.isPointLight!==!0&&D.updateMatrices(H,b);for(let Z=0;Z<E;Z++){let W=D.getCamera(Z);if(H.isPointLight){let Q=D.camera,Fe=D.matrix,Ne=H.distance||Q.far;Ne!==Q.far&&(Q.far=Ne,Q.updateProjectionMatrix()),ro.setFromMatrixPosition(H.matrixWorld),Q.position.copy(ro),tu.copy(Q.position),tu.add(I_[Z]),Q.up.copy(D_[Z]),Q.lookAt(tu),Q.updateMatrixWorld(),Fe.makeTranslation(-ro.x,-ro.y,-ro.z),dp.multiplyMatrices(Q.projectionMatrix,Q.matrixWorldInverse),D._frustum.setFromProjectionMatrix(dp,Q.coordinateSystem,Q.reversedDepth)}if(D.map.isWebGLCubeRenderTarget)s.setRenderTarget(D.map,Z),s.clear();else{Z===0&&(s.setRenderTarget(D.map),s.clear());let Q=D.getViewport(Z);a.set(r.x*Q.x,r.y*Q.y,r.x*Q.z,r.y*Q.w),M.viewport(a)}n=D.getFrustum(Z),g(A,b,W,H,this.type)}D.isPointLightShadow!==!0&&this.type===Or&&y(D,b),D.needsUpdate=!1}f=this.type,m.needsUpdate=!1,s.setRenderTarget(C,S,P)};function y(w,A){let b=e.update(v);d.defines.VSM_SAMPLES!==w.blurSamples&&(d.defines.VSM_SAMPLES=w.blurSamples,p.defines.VSM_SAMPLES=w.blurSamples,d.needsUpdate=!0,p.needsUpdate=!0),w.mapPass===null?w.mapPass=new an(i.x,i.y,{format:vs,type:pn}):(w.mapPass.width!==w.map.width||w.mapPass.height!==w.map.height)&&w.mapPass.setSize(w.map.width,w.map.height),d.uniforms.shadow_pass.value=w.map.depthTexture,d.uniforms.resolution.value.set(w.map.width,w.map.height),d.uniforms.radius.value=w.radius,s.setRenderTarget(w.mapPass),s.clear(),s.renderBufferDirect(A,null,b,d,v,null),p.uniforms.shadow_pass.value=w.mapPass.texture,p.uniforms.resolution.value.set(w.map.width,w.map.height),p.uniforms.radius.value=w.radius,s.setRenderTarget(w.map),s.clear(),s.renderBufferDirect(A,null,b,p,v,null)}function T(w,A,b,C){let S=null,P=b.isPointLight===!0?w.customDistanceMaterial:w.customDepthMaterial;if(P!==void 0)S=P;else if(S=b.isPointLight===!0?l:o,s.localClippingEnabled&&A.clipShadows===!0&&Array.isArray(A.clippingPlanes)&&A.clippingPlanes.length!==0||A.displacementMap&&A.displacementScale!==0||A.alphaMap&&A.alphaTest>0||A.map&&A.alphaTest>0||A.alphaToCoverage===!0){let M=S.uuid,F=A.uuid,I=c[M];I===void 0&&(I={},c[M]=I);let N=I[F];N===void 0&&(N=S.clone(),I[F]=N,A.addEventListener("dispose",_)),S=N}if(S.visible=A.visible,S.wireframe=A.wireframe,C===Or?S.side=A.shadowSide!==null?A.shadowSide:A.side:S.side=A.shadowSide!==null?A.shadowSide:u[A.side],S.alphaMap=A.alphaMap,S.alphaTest=A.alphaToCoverage===!0?.5:A.alphaTest,S.map=A.map,S.clipShadows=A.clipShadows,S.clippingPlanes=A.clippingPlanes,S.clipIntersection=A.clipIntersection,S.displacementMap=A.displacementMap,S.displacementScale=A.displacementScale,S.displacementBias=A.displacementBias,S.wireframeLinewidth=A.wireframeLinewidth,S.linewidth=A.linewidth,b.isPointLight===!0&&S.isMeshDistanceMaterial===!0){let M=s.properties.get(S);M.light=b}return S}function g(w,A,b,C,S){if(w.visible===!1)return;if(w.layers.test(A.layers)&&(w.isMesh||w.isLine||w.isPoints)&&(w.castShadow||w.receiveShadow&&S===Or)&&(!w.frustumCulled||w.intersectsFrustum(n))){w.modelViewMatrix.multiplyMatrices(b.matrixWorldInverse,w.matrixWorld);let F=e.update(w),I=w.material;if(Array.isArray(I)){let N=F.groups;for(let H=0,D=N.length;H<D;H++){let V=N[H],k=I[V.materialIndex];if(k&&k.visible){let E=T(w,k,C,S);w.onBeforeShadow(s,w,A,b,F,E,V),s.renderBufferDirect(b,null,F,E,w,V),w.onAfterShadow(s,w,A,b,F,E,V)}}}else if(I.visible){let N=T(w,I,C,S);w.onBeforeShadow(s,w,A,b,F,N,null),s.renderBufferDirect(b,null,F,N,w,null),w.onAfterShadow(s,w,A,b,F,N,null)}}let M=w.children;for(let F=0,I=M.length;F<I;F++)g(M[F],A,b,C,S)}function _(w){w.target.removeEventListener("dispose",_);for(let b in c){let C=c[b],S=w.target.uuid;S in C&&(C[S].dispose(),delete C[S])}}}function F_(s,e){function t(){let X=!1,Ae=new Ot,Me=null,fe=new Ot(0,0,0,0);return{setMask:function(Pe){Me!==Pe&&!X&&(s.colorMask(Pe,Pe,Pe,Pe),Me=Pe)},setLocked:function(Pe){X=Pe},setClear:function(Pe,_e,qe,Xe,U){U===!0&&(Pe*=Xe,_e*=Xe,qe*=Xe),Ae.set(Pe,_e,qe,Xe),fe.equals(Ae)===!1&&(s.clearColor(Pe,_e,qe,Xe),fe.copy(Ae))},reset:function(){X=!1,Me=null,fe.set(-1,0,0,0)}}}function n(){let X=!1,Ae=!1,Me=null,fe=null,Pe=null;return{setReversed:function(_e){if(Ae!==_e){let qe=e.get("EXT_clip_control");_e?qe.clipControlEXT(qe.LOWER_LEFT_EXT,qe.ZERO_TO_ONE_EXT):qe.clipControlEXT(qe.LOWER_LEFT_EXT,qe.NEGATIVE_ONE_TO_ONE_EXT),Ae=_e;let Xe=Pe;Pe=null,this.setClear(Xe)}},getReversed:function(){return Ae},setTest:function(_e){_e?le(s.DEPTH_TEST):De(s.DEPTH_TEST)},setMask:function(_e){Me!==_e&&!X&&(s.depthMask(_e),Me=_e)},setFunc:function(_e){if(Ae&&(_e=Gf[_e]),fe!==_e){switch(_e){case Yo:s.depthFunc(s.NEVER);break;case Jo:s.depthFunc(s.ALWAYS);break;case Zo:s.depthFunc(s.LESS);break;case gr:s.depthFunc(s.LEQUAL);break;case $o:s.depthFunc(s.EQUAL);break;case Qo:s.depthFunc(s.GEQUAL);break;case el:s.depthFunc(s.GREATER);break;case tl:s.depthFunc(s.NOTEQUAL);break;default:s.depthFunc(s.LEQUAL)}fe=_e}},setLocked:function(_e){X=_e},setClear:function(_e){Pe!==_e&&(Pe=_e,Ae&&(_e=1-_e),s.clearDepth(_e))},reset:function(){X=!1,Me=null,fe=null,Pe=null,Ae=!1}}}function i(){let X=!1,Ae=null,Me=null,fe=null,Pe=null,_e=null,qe=null,Xe=null,U=null;return{setTest:function(q){X||(q?le(s.STENCIL_TEST):De(s.STENCIL_TEST))},setMask:function(q){Ae!==q&&!X&&(s.stencilMask(q),Ae=q)},setFunc:function(q,Y,ae){(Me!==q||fe!==Y||Pe!==ae)&&(s.stencilFunc(q,Y,ae),Me=q,fe=Y,Pe=ae)},setOp:function(q,Y,ae){(_e!==q||qe!==Y||Xe!==ae)&&(s.stencilOp(q,Y,ae),_e=q,qe=Y,Xe=ae)},setLocked:function(q){X=q},setClear:function(q){U!==q&&(s.clearStencil(q),U=q)},reset:function(){X=!1,Ae=null,Me=null,fe=null,Pe=null,_e=null,qe=null,Xe=null,U=null}}}let r=new t,a=new n,o=new i,l=new WeakMap,c=new WeakMap,h={},u={},d={},p=new WeakMap,x=[],v=null,m=!1,f=null,y=null,T=null,g=null,_=null,w=null,A=null,b=new Le(0,0,0),C=0,S=!1,P=null,M=null,F=null,I=null,N=null,H=s.getParameter(s.MAX_COMBINED_TEXTURE_IMAGE_UNITS),D=!1,V=0,k=s.getParameter(s.VERSION);k.indexOf("WebGL")!==-1?(V=parseFloat(/^WebGL (\d)/.exec(k)[1]),D=V>=1):k.indexOf("OpenGL ES")!==-1&&(V=parseFloat(/^OpenGL ES (\d)/.exec(k)[1]),D=V>=2);let E=null,Z={},W=s.getParameter(s.SCISSOR_BOX),Q=s.getParameter(s.VIEWPORT),Fe=new Ot().fromArray(W),Ne=new Ot().fromArray(Q);function ze(X,Ae,Me,fe){let Pe=new Uint8Array(4),_e=s.createTexture();s.bindTexture(X,_e),s.texParameteri(X,s.TEXTURE_MIN_FILTER,s.NEAREST),s.texParameteri(X,s.TEXTURE_MAG_FILTER,s.NEAREST);for(let qe=0;qe<Me;qe++)X===s.TEXTURE_3D||X===s.TEXTURE_2D_ARRAY?s.texImage3D(Ae,0,s.RGBA,1,1,fe,0,s.RGBA,s.UNSIGNED_BYTE,Pe):s.texImage2D(Ae+qe,0,s.RGBA,1,1,0,s.RGBA,s.UNSIGNED_BYTE,Pe);return _e}let se={};se[s.TEXTURE_2D]=ze(s.TEXTURE_2D,s.TEXTURE_2D,1),se[s.TEXTURE_CUBE_MAP]=ze(s.TEXTURE_CUBE_MAP,s.TEXTURE_CUBE_MAP_POSITIVE_X,6),se[s.TEXTURE_2D_ARRAY]=ze(s.TEXTURE_2D_ARRAY,s.TEXTURE_2D_ARRAY,1,1),se[s.TEXTURE_3D]=ze(s.TEXTURE_3D,s.TEXTURE_3D,1,1),r.setClear(0,0,0,1),a.setClear(1),o.setClear(0),le(s.DEPTH_TEST),a.setFunc(gr),we(!1),Ve(Ch),le(s.CULL_FACE),ne(ni);function le(X){h[X]!==!0&&(s.enable(X),h[X]=!0)}function De(X){h[X]!==!1&&(s.disable(X),h[X]=!1)}function be(X,Ae){return d[X]!==Ae?(s.bindFramebuffer(X,Ae),d[X]=Ae,X===s.DRAW_FRAMEBUFFER&&(d[s.FRAMEBUFFER]=Ae),X===s.FRAMEBUFFER&&(d[s.DRAW_FRAMEBUFFER]=Ae),!0):!1}function ue(X,Ae){let Me=x,fe=!1;if(X){Me=p.get(Ae),Me===void 0&&(Me=[],p.set(Ae,Me));let Pe=X.textures;if(Me.length!==Pe.length||Me[0]!==s.COLOR_ATTACHMENT0){for(let _e=0,qe=Pe.length;_e<qe;_e++)Me[_e]=s.COLOR_ATTACHMENT0+_e;Me.length=Pe.length,fe=!0}}else Me[0]!==s.BACK&&(Me[0]=s.BACK,fe=!0);fe&&s.drawBuffers(Me)}function te(X){return v!==X?(s.useProgram(X),v=X,!0):!1}let pe={[Vs]:s.FUNC_ADD,[of]:s.FUNC_SUBTRACT,[lf]:s.FUNC_REVERSE_SUBTRACT};pe[cf]=s.MIN,pe[hf]=s.MAX;let ce={[uf]:s.ZERO,[df]:s.ONE,[ff]:s.SRC_COLOR,[Dh]:s.SRC_ALPHA,[vf]:s.SRC_ALPHA_SATURATE,[xf]:s.DST_COLOR,[mf]:s.DST_ALPHA,[pf]:s.ONE_MINUS_SRC_COLOR,[Lh]:s.ONE_MINUS_SRC_ALPHA,[bf]:s.ONE_MINUS_DST_COLOR,[gf]:s.ONE_MINUS_DST_ALPHA,[_f]:s.CONSTANT_COLOR,[yf]:s.ONE_MINUS_CONSTANT_COLOR,[Mf]:s.CONSTANT_ALPHA,[Sf]:s.ONE_MINUS_CONSTANT_ALPHA};function ne(X,Ae,Me,fe,Pe,_e,qe,Xe,U,q){if(X===ni){m===!0&&(De(s.BLEND),m=!1);return}if(m===!1&&(le(s.BLEND),m=!0),X!==af){if(X!==f||q!==S){if((y!==Vs||_!==Vs)&&(s.blendEquation(s.FUNC_ADD),y=Vs,_=Vs),q)switch(X){case zr:s.blendFuncSeparate(s.ONE,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case tn:s.blendFunc(s.ONE,s.ONE);break;case Ph:s.blendFuncSeparate(s.ZERO,s.ONE_MINUS_SRC_COLOR,s.ZERO,s.ONE);break;case Ih:s.blendFuncSeparate(s.DST_COLOR,s.ONE_MINUS_SRC_ALPHA,s.ZERO,s.ONE);break;default:pt("WebGLState: Invalid blending: ",X);break}else switch(X){case zr:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case tn:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE,s.ONE,s.ONE);break;case Ph:pt("WebGLState: SubtractiveBlending requires material.premultipliedAlpha = true");break;case Ih:pt("WebGLState: MultiplyBlending requires material.premultipliedAlpha = true");break;default:pt("WebGLState: Invalid blending: ",X);break}T=null,g=null,w=null,A=null,b.set(0,0,0),C=0,f=X,S=q}return}Pe=Pe||Ae,_e=_e||Me,qe=qe||fe,(Ae!==y||Pe!==_)&&(s.blendEquationSeparate(pe[Ae],pe[Pe]),y=Ae,_=Pe),(Me!==T||fe!==g||_e!==w||qe!==A)&&(s.blendFuncSeparate(ce[Me],ce[fe],ce[_e],ce[qe]),T=Me,g=fe,w=_e,A=qe),(Xe.equals(b)===!1||U!==C)&&(s.blendColor(Xe.r,Xe.g,Xe.b,U),b.copy(Xe),C=U),f=X,S=!1}function de(X,Ae){X.side===Kt?De(s.CULL_FACE):le(s.CULL_FACE);let Me=X.side===mn;Ae&&(Me=!Me),we(Me),X.blending===zr&&X.transparent===!1?ne(ni):ne(X.blending,X.blendEquation,X.blendSrc,X.blendDst,X.blendEquationAlpha,X.blendSrcAlpha,X.blendDstAlpha,X.blendColor,X.blendAlpha,X.premultipliedAlpha),a.setFunc(X.depthFunc),a.setTest(X.depthTest),a.setMask(X.depthWrite),r.setMask(X.colorWrite);let fe=X.stencilWrite;o.setTest(fe),fe&&(o.setMask(X.stencilWriteMask),o.setFunc(X.stencilFunc,X.stencilRef,X.stencilFuncMask),o.setOp(X.stencilFail,X.stencilZFail,X.stencilZPass)),it(X.polygonOffset,X.polygonOffsetFactor,X.polygonOffsetUnits),X.alphaToCoverage===!0?le(s.SAMPLE_ALPHA_TO_COVERAGE):De(s.SAMPLE_ALPHA_TO_COVERAGE)}function we(X){P!==X&&(X?s.frontFace(s.CW):s.frontFace(s.CCW),P=X)}function Ve(X){X!==nf?(le(s.CULL_FACE),X!==M&&(X===Ch?s.cullFace(s.BACK):X===sf?s.cullFace(s.FRONT):s.cullFace(s.FRONT_AND_BACK))):De(s.CULL_FACE),M=X}function Ke(X){X!==F&&(D&&s.lineWidth(X),F=X)}function it(X,Ae,Me){X?(le(s.POLYGON_OFFSET_FILL),(I!==Ae||N!==Me)&&(I=Ae,N=Me,a.getReversed()&&(Ae=-Ae),s.polygonOffset(Ae,Me))):De(s.POLYGON_OFFSET_FILL)}function et(X){X?le(s.SCISSOR_TEST):De(s.SCISSOR_TEST)}function ht(X){X===void 0&&(X=s.TEXTURE0+H-1),E!==X&&(s.activeTexture(X),E=X)}function J(X,Ae,Me){Me===void 0&&(E===null?Me=s.TEXTURE0+H-1:Me=E);let fe=Z[Me];fe===void 0&&(fe={type:void 0,texture:void 0},Z[Me]=fe),(fe.type!==X||fe.texture!==Ae)&&(E!==Me&&(s.activeTexture(Me),E=Me),s.bindTexture(X,Ae||se[X]),fe.type=X,fe.texture=Ae)}function Ut(){let X=Z[E];X!==void 0&&X.type!==void 0&&(s.bindTexture(X.type,null),X.type=void 0,X.texture=void 0)}function ut(){try{s.compressedTexImage2D(...arguments)}catch(X){pt("WebGLState:",X)}}function O(){try{s.compressedTexImage3D(...arguments)}catch(X){pt("WebGLState:",X)}}function R(){try{s.texSubImage2D(...arguments)}catch(X){pt("WebGLState:",X)}}function re(){try{s.texSubImage3D(...arguments)}catch(X){pt("WebGLState:",X)}}function oe(){try{s.compressedTexSubImage2D(...arguments)}catch(X){pt("WebGLState:",X)}}function xe(){try{s.compressedTexSubImage3D(...arguments)}catch(X){pt("WebGLState:",X)}}function Ue(){try{s.texStorage2D(...arguments)}catch(X){pt("WebGLState:",X)}}function ke(){try{s.texStorage3D(...arguments)}catch(X){pt("WebGLState:",X)}}function G(){try{s.texImage2D(...arguments)}catch(X){pt("WebGLState:",X)}}function j(){try{s.texImage3D(...arguments)}catch(X){pt("WebGLState:",X)}}function he(X){return u[X]!==void 0?u[X]:s.getParameter(X)}function Se(X,Ae){u[X]!==Ae&&(s.pixelStorei(X,Ae),u[X]=Ae)}function ie(X){Fe.equals(X)===!1&&(s.scissor(X.x,X.y,X.z,X.w),Fe.copy(X))}function ve(X){Ne.equals(X)===!1&&(s.viewport(X.x,X.y,X.z,X.w),Ne.copy(X))}function Ee(X,Ae){let Me=c.get(Ae);Me===void 0&&(Me=new WeakMap,c.set(Ae,Me));let fe=Me.get(X);fe===void 0&&(fe=s.getUniformBlockIndex(Ae,X.name),Me.set(X,fe))}function Te(X,Ae){let fe=c.get(Ae).get(X);l.get(Ae)!==fe&&(s.uniformBlockBinding(Ae,fe,X.__bindingPointIndex),l.set(Ae,fe))}function Ie(){s.disable(s.BLEND),s.disable(s.CULL_FACE),s.disable(s.DEPTH_TEST),s.disable(s.POLYGON_OFFSET_FILL),s.disable(s.SCISSOR_TEST),s.disable(s.STENCIL_TEST),s.disable(s.SAMPLE_ALPHA_TO_COVERAGE),s.blendEquation(s.FUNC_ADD),s.blendFunc(s.ONE,s.ZERO),s.blendFuncSeparate(s.ONE,s.ZERO,s.ONE,s.ZERO),s.blendColor(0,0,0,0),s.colorMask(!0,!0,!0,!0),s.clearColor(0,0,0,0),s.depthMask(!0),s.depthFunc(s.LESS),a.setReversed(!1),s.clearDepth(1),s.stencilMask(4294967295),s.stencilFunc(s.ALWAYS,0,4294967295),s.stencilOp(s.KEEP,s.KEEP,s.KEEP),s.clearStencil(0),s.cullFace(s.BACK),s.frontFace(s.CCW),s.polygonOffset(0,0),s.activeTexture(s.TEXTURE0),s.bindFramebuffer(s.FRAMEBUFFER,null),s.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),s.bindFramebuffer(s.READ_FRAMEBUFFER,null),s.useProgram(null),s.lineWidth(1),s.scissor(0,0,s.canvas.width,s.canvas.height),s.viewport(0,0,s.canvas.width,s.canvas.height),s.pixelStorei(s.PACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,!1),s.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!1),s.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,s.BROWSER_DEFAULT_WEBGL),s.pixelStorei(s.PACK_ROW_LENGTH,0),s.pixelStorei(s.PACK_SKIP_PIXELS,0),s.pixelStorei(s.PACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_ROW_LENGTH,0),s.pixelStorei(s.UNPACK_IMAGE_HEIGHT,0),s.pixelStorei(s.UNPACK_SKIP_PIXELS,0),s.pixelStorei(s.UNPACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_SKIP_IMAGES,0),h={},u={},E=null,Z={},d={},p=new WeakMap,x=[],v=null,m=!1,f=null,y=null,T=null,g=null,_=null,w=null,A=null,b=new Le(0,0,0),C=0,S=!1,P=null,M=null,F=null,I=null,N=null,Fe.set(0,0,s.canvas.width,s.canvas.height),Ne.set(0,0,s.canvas.width,s.canvas.height),r.reset(),a.reset(),o.reset()}return{buffers:{color:r,depth:a,stencil:o},enable:le,disable:De,bindFramebuffer:be,drawBuffers:ue,useProgram:te,setBlending:ne,setMaterial:de,setFlipSided:we,setCullFace:Ve,setLineWidth:Ke,setPolygonOffset:it,setScissorTest:et,activeTexture:ht,bindTexture:J,unbindTexture:Ut,compressedTexImage2D:ut,compressedTexImage3D:O,texImage2D:G,texImage3D:j,pixelStorei:Se,getParameter:he,updateUBOMapping:Ee,uniformBlockBinding:Te,texStorage2D:Ue,texStorage3D:ke,texSubImage2D:R,texSubImage3D:re,compressedTexSubImage2D:oe,compressedTexSubImage3D:xe,scissor:ie,viewport:ve,reset:Ie}}function N_(s,e,t,n,i,r,a){let o=e.has("WEBGL_multisampled_render_to_texture")?e.get("WEBGL_multisampled_render_to_texture"):null,l=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),c=new Be,h=new WeakMap,u=new Set,d,p=new WeakMap,x=!1;try{x=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function v(O,R){return x?new OffscreenCanvas(O,R):vr("canvas")}function m(O,R,re){let oe=1,xe=ut(O);if((xe.width>re||xe.height>re)&&(oe=re/Math.max(xe.width,xe.height)),oe<1)if(typeof HTMLImageElement<"u"&&O instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&O instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&O instanceof ImageBitmap||typeof VideoFrame<"u"&&O instanceof VideoFrame){let Ue=Math.floor(oe*xe.width),ke=Math.floor(oe*xe.height);d===void 0&&(d=v(Ue,ke));let G=R?v(Ue,ke):d;return G.width=Ue,G.height=ke,G.getContext("2d").drawImage(O,0,0,Ue,ke),rt("WebGLRenderer: Texture has been resized from ("+xe.width+"x"+xe.height+") to ("+Ue+"x"+ke+")."),G}else return"data"in O&&rt("WebGLRenderer: Image in DataTexture is too big ("+xe.width+"x"+xe.height+")."),O;return O}function f(O){return O.generateMipmaps}function y(O){s.generateMipmap(O)}function T(O){return O.isWebGLCubeRenderTarget?s.TEXTURE_CUBE_MAP:O.isWebGL3DRenderTarget?s.TEXTURE_3D:O.isWebGLArrayRenderTarget||O.isCompressedArrayTexture?s.TEXTURE_2D_ARRAY:s.TEXTURE_2D}function g(O,R,re,oe,xe,Ue=!1){if(O!==null){if(s[O]!==void 0)return s[O];rt("WebGLRenderer: Attempt to use non-existing WebGL internal format '"+O+"'")}let ke;oe&&(ke=e.get("EXT_texture_norm16"),ke||rt("WebGLRenderer: Unable to use normalized textures without EXT_texture_norm16 extension"));let G=R;if(R===s.RED&&(re===s.FLOAT&&(G=s.R32F),re===s.HALF_FLOAT&&(G=s.R16F),re===s.UNSIGNED_BYTE&&(G=s.R8),re===s.UNSIGNED_SHORT&&ke&&(G=ke.R16_EXT),re===s.SHORT&&ke&&(G=ke.R16_SNORM_EXT)),R===s.RED_INTEGER&&(re===s.UNSIGNED_BYTE&&(G=s.R8UI),re===s.UNSIGNED_SHORT&&(G=s.R16UI),re===s.UNSIGNED_INT&&(G=s.R32UI),re===s.BYTE&&(G=s.R8I),re===s.SHORT&&(G=s.R16I),re===s.INT&&(G=s.R32I)),R===s.RG&&(re===s.FLOAT&&(G=s.RG32F),re===s.HALF_FLOAT&&(G=s.RG16F),re===s.UNSIGNED_BYTE&&(G=s.RG8),re===s.UNSIGNED_SHORT&&ke&&(G=ke.RG16_EXT),re===s.SHORT&&ke&&(G=ke.RG16_SNORM_EXT)),R===s.RG_INTEGER&&(re===s.UNSIGNED_BYTE&&(G=s.RG8UI),re===s.UNSIGNED_SHORT&&(G=s.RG16UI),re===s.UNSIGNED_INT&&(G=s.RG32UI),re===s.BYTE&&(G=s.RG8I),re===s.SHORT&&(G=s.RG16I),re===s.INT&&(G=s.RG32I)),R===s.RGB_INTEGER&&(re===s.UNSIGNED_BYTE&&(G=s.RGB8UI),re===s.UNSIGNED_SHORT&&(G=s.RGB16UI),re===s.UNSIGNED_INT&&(G=s.RGB32UI),re===s.BYTE&&(G=s.RGB8I),re===s.SHORT&&(G=s.RGB16I),re===s.INT&&(G=s.RGB32I)),R===s.RGBA_INTEGER&&(re===s.UNSIGNED_BYTE&&(G=s.RGBA8UI),re===s.UNSIGNED_SHORT&&(G=s.RGBA16UI),re===s.UNSIGNED_INT&&(G=s.RGBA32UI),re===s.BYTE&&(G=s.RGBA8I),re===s.SHORT&&(G=s.RGBA16I),re===s.INT&&(G=s.RGBA32I)),R===s.RGB&&(re===s.UNSIGNED_SHORT&&ke&&(G=ke.RGB16_EXT),re===s.SHORT&&ke&&(G=ke.RGB16_SNORM_EXT),re===s.UNSIGNED_INT_5_9_9_9_REV&&(G=s.RGB9_E5),re===s.UNSIGNED_INT_10F_11F_11F_REV&&(G=s.R11F_G11F_B10F)),R===s.RGBA){let j=Ue?ga:Tt.getTransfer(xe);re===s.FLOAT&&(G=s.RGBA32F),re===s.HALF_FLOAT&&(G=s.RGBA16F),re===s.UNSIGNED_BYTE&&(G=j===Gt?s.SRGB8_ALPHA8:s.RGBA8),re===s.UNSIGNED_SHORT&&ke&&(G=ke.RGBA16_EXT),re===s.SHORT&&ke&&(G=ke.RGBA16_SNORM_EXT),re===s.UNSIGNED_SHORT_4_4_4_4&&(G=s.RGBA4),re===s.UNSIGNED_SHORT_5_5_5_1&&(G=s.RGB5_A1)}return(G===s.R16F||G===s.R32F||G===s.RG16F||G===s.RG32F||G===s.RGBA16F||G===s.RGBA32F)&&e.get("EXT_color_buffer_float"),G}function _(O,R){let re;return O?R===null||R===vi||R===Hr?re=s.DEPTH24_STENCIL8:R===Yn?re=s.DEPTH32F_STENCIL8:R===Br&&(re=s.DEPTH24_STENCIL8,rt("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):R===null||R===vi||R===Hr?re=s.DEPTH_COMPONENT24:R===Yn?re=s.DEPTH_COMPONENT32F:R===Br&&(re=s.DEPTH_COMPONENT16),re}function w(O,R){return f(O)===!0||O.isFramebufferTexture&&O.minFilter!==on&&O.minFilter!==rn?Math.log2(Math.max(R.width,R.height))+1:O.mipmaps!==void 0&&O.mipmaps.length>0?O.mipmaps.length:O.isCompressedTexture&&Array.isArray(O.image)?R.mipmaps.length:1}function A(O){let R=O.target;R.removeEventListener("dispose",A),C(R),R.isVideoTexture&&h.delete(R),R.isHTMLTexture&&u.delete(R)}function b(O){let R=O.target;R.removeEventListener("dispose",b),P(R)}function C(O){let R=n.get(O);if(R.__webglInit===void 0)return;let re=O.source,oe=p.get(re);if(oe){let xe=oe[R.__cacheKey];xe.usedTimes--,xe.usedTimes===0&&S(O),Object.keys(oe).length===0&&p.delete(re)}n.remove(O)}function S(O){let R=n.get(O);s.deleteTexture(R.__webglTexture);let re=O.source,oe=p.get(re);delete oe[R.__cacheKey],a.memory.textures--}function P(O){let R=n.get(O);if(O.depthTexture&&(O.depthTexture.dispose(),n.remove(O.depthTexture)),O.isWebGLCubeRenderTarget)for(let oe=0;oe<6;oe++){if(Array.isArray(R.__webglFramebuffer[oe]))for(let xe=0;xe<R.__webglFramebuffer[oe].length;xe++)s.deleteFramebuffer(R.__webglFramebuffer[oe][xe]);else s.deleteFramebuffer(R.__webglFramebuffer[oe]);R.__webglDepthbuffer&&s.deleteRenderbuffer(R.__webglDepthbuffer[oe])}else{if(Array.isArray(R.__webglFramebuffer))for(let oe=0;oe<R.__webglFramebuffer.length;oe++)s.deleteFramebuffer(R.__webglFramebuffer[oe]);else s.deleteFramebuffer(R.__webglFramebuffer);if(R.__webglDepthbuffer&&s.deleteRenderbuffer(R.__webglDepthbuffer),R.__webglMultisampledFramebuffer&&s.deleteFramebuffer(R.__webglMultisampledFramebuffer),R.__webglColorRenderbuffer)for(let oe=0;oe<R.__webglColorRenderbuffer.length;oe++)R.__webglColorRenderbuffer[oe]&&s.deleteRenderbuffer(R.__webglColorRenderbuffer[oe]);R.__webglDepthRenderbuffer&&s.deleteRenderbuffer(R.__webglDepthRenderbuffer)}let re=O.textures;for(let oe=0,xe=re.length;oe<xe;oe++){let Ue=n.get(re[oe]);Ue.__webglTexture&&(s.deleteTexture(Ue.__webglTexture),a.memory.textures--),n.remove(re[oe])}n.remove(O)}let M=0;function F(){M=0}function I(){return M}function N(O){M=O}function H(){let O=M;return O>=i.maxTextures&&rt("WebGLTextures: Trying to use "+(O+1)+" texture units while this GPU supports only "+i.maxTextures),M+=1,O}function D(O){let R=[];return R.push(O.wrapS),R.push(O.wrapT),R.push(O.wrapR||0),R.push(O.magFilter),R.push(O.minFilter),R.push(O.anisotropy),R.push(O.internalFormat),R.push(O.format),R.push(O.type),R.push(O.generateMipmaps),R.push(O.premultiplyAlpha),R.push(O.flipY),R.push(O.unpackAlignment),R.push(O.colorSpace),R.join()}function V(O,R){let re=n.get(O);if(O.isVideoTexture&&J(O),O.isRenderTargetTexture===!1&&O.isExternalTexture!==!0&&O.version>0&&re.__version!==O.version){let oe=O.image;if(oe===null)rt("WebGLRenderer: Texture marked for update but no image data found.");else if(oe.complete===!1)rt("WebGLRenderer: Texture marked for update but image is incomplete");else{De(re,O,R);return}}else O.isExternalTexture&&(re.__webglTexture=O.sourceTexture?O.sourceTexture:null);t.bindTexture(s.TEXTURE_2D,re.__webglTexture,s.TEXTURE0+R)}function k(O,R){let re=n.get(O);if(O.isRenderTargetTexture===!1&&O.version>0&&re.__version!==O.version){De(re,O,R);return}else O.isExternalTexture&&(re.__webglTexture=O.sourceTexture?O.sourceTexture:null);t.bindTexture(s.TEXTURE_2D_ARRAY,re.__webglTexture,s.TEXTURE0+R)}function E(O,R){let re=n.get(O);if(O.isRenderTargetTexture===!1&&O.version>0&&re.__version!==O.version){De(re,O,R);return}t.bindTexture(s.TEXTURE_3D,re.__webglTexture,s.TEXTURE0+R)}function Z(O,R){let re=n.get(O);if(O.isCubeDepthTexture!==!0&&O.version>0&&re.__version!==O.version){be(re,O,R);return}t.bindTexture(s.TEXTURE_CUBE_MAP,re.__webglTexture,s.TEXTURE0+R)}let W={[Ti]:s.REPEAT,[Qn]:s.CLAMP_TO_EDGE,[xr]:s.MIRRORED_REPEAT},Q={[on]:s.NEAREST,[Pl]:s.NEAREST_MIPMAP_NEAREST,[Xs]:s.NEAREST_MIPMAP_LINEAR,[rn]:s.LINEAR,[kr]:s.LINEAR_MIPMAP_NEAREST,[bi]:s.LINEAR_MIPMAP_LINEAR},Fe={[Lf]:s.NEVER,[zf]:s.ALWAYS,[Ff]:s.LESS,[gc]:s.LEQUAL,[Nf]:s.EQUAL,[xc]:s.GEQUAL,[Uf]:s.GREATER,[Of]:s.NOTEQUAL};function Ne(O,R){if(R.type===Yn&&e.has("OES_texture_float_linear")===!1&&(R.magFilter===rn||R.magFilter===kr||R.magFilter===Xs||R.magFilter===bi||R.minFilter===rn||R.minFilter===kr||R.minFilter===Xs||R.minFilter===bi)&&rt("WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),s.texParameteri(O,s.TEXTURE_WRAP_S,W[R.wrapS]),s.texParameteri(O,s.TEXTURE_WRAP_T,W[R.wrapT]),(O===s.TEXTURE_3D||O===s.TEXTURE_2D_ARRAY)&&s.texParameteri(O,s.TEXTURE_WRAP_R,W[R.wrapR]),s.texParameteri(O,s.TEXTURE_MAG_FILTER,Q[R.magFilter]),s.texParameteri(O,s.TEXTURE_MIN_FILTER,Q[R.minFilter]),R.compareFunction&&(s.texParameteri(O,s.TEXTURE_COMPARE_MODE,s.COMPARE_REF_TO_TEXTURE),s.texParameteri(O,s.TEXTURE_COMPARE_FUNC,Fe[R.compareFunction])),e.has("EXT_texture_filter_anisotropic")===!0){if(R.magFilter===on||R.minFilter!==Xs&&R.minFilter!==bi||R.type===Yn&&e.has("OES_texture_float_linear")===!1)return;if(R.anisotropy>1||n.get(R).__currentAnisotropy){let re=e.get("EXT_texture_filter_anisotropic");s.texParameterf(O,re.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(R.anisotropy,i.getMaxAnisotropy())),n.get(R).__currentAnisotropy=R.anisotropy}}}function ze(O,R){let re=!1;O.__webglInit===void 0&&(O.__webglInit=!0,R.addEventListener("dispose",A));let oe=R.source,xe=p.get(oe);xe===void 0&&(xe={},p.set(oe,xe));let Ue=D(R);if(Ue!==O.__cacheKey){xe[Ue]===void 0&&(xe[Ue]={texture:s.createTexture(),usedTimes:0},a.memory.textures++,re=!0),xe[Ue].usedTimes++;let ke=xe[O.__cacheKey];ke!==void 0&&(xe[O.__cacheKey].usedTimes--,ke.usedTimes===0&&S(R)),O.__cacheKey=Ue,O.__webglTexture=xe[Ue].texture}return re}function se(O,R,re){return Math.floor(Math.floor(O/re)/R)}function le(O,R,re,oe){let Ue=O.updateRanges;if(Ue.length===0)t.texSubImage2D(s.TEXTURE_2D,0,0,0,R.width,R.height,re,oe,R.data);else{Ue.sort((Se,ie)=>Se.start-ie.start);let ke=0;for(let Se=1;Se<Ue.length;Se++){let ie=Ue[ke],ve=Ue[Se],Ee=ie.start+ie.count,Te=se(ve.start,R.width,4),Ie=se(ie.start,R.width,4);ve.start<=Ee+1&&Te===Ie&&se(ve.start+ve.count-1,R.width,4)===Te?ie.count=Math.max(ie.count,ve.start+ve.count-ie.start):(++ke,Ue[ke]=ve)}Ue.length=ke+1;let G=t.getParameter(s.UNPACK_ROW_LENGTH),j=t.getParameter(s.UNPACK_SKIP_PIXELS),he=t.getParameter(s.UNPACK_SKIP_ROWS);t.pixelStorei(s.UNPACK_ROW_LENGTH,R.width);for(let Se=0,ie=Ue.length;Se<ie;Se++){let ve=Ue[Se],Ee=Math.floor(ve.start/4),Te=Math.ceil(ve.count/4),Ie=Ee%R.width,X=Math.floor(Ee/R.width),Ae=Te,Me=1;t.pixelStorei(s.UNPACK_SKIP_PIXELS,Ie),t.pixelStorei(s.UNPACK_SKIP_ROWS,X),t.texSubImage2D(s.TEXTURE_2D,0,Ie,X,Ae,Me,re,oe,R.data)}O.clearUpdateRanges(),t.pixelStorei(s.UNPACK_ROW_LENGTH,G),t.pixelStorei(s.UNPACK_SKIP_PIXELS,j),t.pixelStorei(s.UNPACK_SKIP_ROWS,he)}}function De(O,R,re){let oe=s.TEXTURE_2D;(R.isDataArrayTexture||R.isCompressedArrayTexture)&&(oe=s.TEXTURE_2D_ARRAY),R.isData3DTexture&&(oe=s.TEXTURE_3D);let xe=ze(O,R),Ue=R.source;t.bindTexture(oe,O.__webglTexture,s.TEXTURE0+re);let ke=n.get(Ue);if(Ue.version!==ke.__version||xe===!0){if(t.activeTexture(s.TEXTURE0+re),(typeof ImageBitmap<"u"&&R.image instanceof ImageBitmap)===!1){let Me=Tt.getPrimaries(Tt.workingColorSpace),fe=R.colorSpace===ii?null:Tt.getPrimaries(R.colorSpace),Pe=R.colorSpace===ii||Me===fe?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,R.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,R.premultiplyAlpha),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,Pe)}t.pixelStorei(s.UNPACK_ALIGNMENT,R.unpackAlignment);let j=m(R.image,!1,i.maxTextureSize);j=Ut(R,j);let he=r.convert(R.format,R.colorSpace),Se=r.convert(R.type),ie=g(R.internalFormat,he,Se,R.normalized,R.colorSpace,R.isVideoTexture);Ne(oe,R);let ve,Ee=R.mipmaps,Te=R.isVideoTexture!==!0,Ie=ke.__version===void 0||xe===!0,X=Ue.dataReady,Ae=w(R,j);if(R.isDepthTexture)ie=_(R.format===bs,R.type),Ie&&(Te?t.texStorage2D(s.TEXTURE_2D,1,ie,j.width,j.height):t.texImage2D(s.TEXTURE_2D,0,ie,j.width,j.height,0,he,Se,null));else if(R.isDataTexture)if(Ee.length>0){Te&&Ie&&t.texStorage2D(s.TEXTURE_2D,Ae,ie,Ee[0].width,Ee[0].height);for(let Me=0,fe=Ee.length;Me<fe;Me++)ve=Ee[Me],Te?X&&t.texSubImage2D(s.TEXTURE_2D,Me,0,0,ve.width,ve.height,he,Se,ve.data):t.texImage2D(s.TEXTURE_2D,Me,ie,ve.width,ve.height,0,he,Se,ve.data);R.generateMipmaps=!1}else Te?(Ie&&t.texStorage2D(s.TEXTURE_2D,Ae,ie,j.width,j.height),X&&le(R,j,he,Se)):t.texImage2D(s.TEXTURE_2D,0,ie,j.width,j.height,0,he,Se,j.data);else if(R.isCompressedTexture)if(R.isCompressedArrayTexture){Te&&Ie&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Ae,ie,Ee[0].width,Ee[0].height,j.depth);for(let Me=0,fe=Ee.length;Me<fe;Me++)if(ve=Ee[Me],R.format!==Jn)if(he!==null)if(Te){if(X)if(R.layerUpdates.size>0){let Pe=Yh(ve.width,ve.height,R.format,R.type);for(let _e of R.layerUpdates){let qe=ve.data.subarray(_e*Pe/ve.data.BYTES_PER_ELEMENT,(_e+1)*Pe/ve.data.BYTES_PER_ELEMENT);t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,Me,0,0,_e,ve.width,ve.height,1,he,qe)}}else t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,Me,0,0,0,ve.width,ve.height,j.depth,he,ve.data)}else t.compressedTexImage3D(s.TEXTURE_2D_ARRAY,Me,ie,ve.width,ve.height,j.depth,0,ve.data,0,0);else rt("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else Te?X&&t.texSubImage3D(s.TEXTURE_2D_ARRAY,Me,0,0,0,ve.width,ve.height,j.depth,he,Se,ve.data):t.texImage3D(s.TEXTURE_2D_ARRAY,Me,ie,ve.width,ve.height,j.depth,0,he,Se,ve.data);R.layerUpdates.size>0&&R.clearLayerUpdates()}else{Te&&Ie&&t.texStorage2D(s.TEXTURE_2D,Ae,ie,Ee[0].width,Ee[0].height);for(let Me=0,fe=Ee.length;Me<fe;Me++)ve=Ee[Me],R.format!==Jn?he!==null?Te?X&&t.compressedTexSubImage2D(s.TEXTURE_2D,Me,0,0,ve.width,ve.height,he,ve.data):t.compressedTexImage2D(s.TEXTURE_2D,Me,ie,ve.width,ve.height,0,ve.data):rt("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):Te?X&&t.texSubImage2D(s.TEXTURE_2D,Me,0,0,ve.width,ve.height,he,Se,ve.data):t.texImage2D(s.TEXTURE_2D,Me,ie,ve.width,ve.height,0,he,Se,ve.data)}else if(R.isDataArrayTexture)if(Te){if(Ie&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Ae,ie,j.width,j.height,j.depth),X)if(R.layerUpdates.size>0){let Me=Yh(j.width,j.height,R.format,R.type);for(let fe of R.layerUpdates){let Pe=j.data.subarray(fe*Me/j.data.BYTES_PER_ELEMENT,(fe+1)*Me/j.data.BYTES_PER_ELEMENT);t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,fe,j.width,j.height,1,he,Se,Pe)}R.clearLayerUpdates()}else t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,0,j.width,j.height,j.depth,he,Se,j.data)}else t.texImage3D(s.TEXTURE_2D_ARRAY,0,ie,j.width,j.height,j.depth,0,he,Se,j.data);else if(R.isData3DTexture)Te?(Ie&&t.texStorage3D(s.TEXTURE_3D,Ae,ie,j.width,j.height,j.depth),X&&t.texSubImage3D(s.TEXTURE_3D,0,0,0,0,j.width,j.height,j.depth,he,Se,j.data)):t.texImage3D(s.TEXTURE_3D,0,ie,j.width,j.height,j.depth,0,he,Se,j.data);else if(R.isFramebufferTexture){if(Ie)if(Te)t.texStorage2D(s.TEXTURE_2D,Ae,ie,j.width,j.height);else{let Me=j.width,fe=j.height;for(let Pe=0;Pe<Ae;Pe++)t.texImage2D(s.TEXTURE_2D,Pe,ie,Me,fe,0,he,Se,null),Me>>=1,fe>>=1}}else if(R.isHTMLTexture){if("texElementImage2D"in s){let Me=s.canvas;if(Me.hasAttribute("layoutsubtree")||Me.setAttribute("layoutsubtree","true"),j.parentNode!==Me){Me.appendChild(j),u.add(R),Me.onpaint=fe=>{let Pe=fe.changedElements;for(let _e of u)Pe.includes(_e.image)&&(_e.needsUpdate=!0)},Me.requestPaint();return}if(s.texElementImage2D.length===3)s.texElementImage2D(s.TEXTURE_2D,s.RGBA8,j);else{let Pe=s.RGBA,_e=s.RGBA,qe=s.UNSIGNED_BYTE;s.texElementImage2D(s.TEXTURE_2D,0,Pe,_e,qe,j)}s.texParameteri(s.TEXTURE_2D,s.TEXTURE_MIN_FILTER,s.LINEAR),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_S,s.CLAMP_TO_EDGE),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_T,s.CLAMP_TO_EDGE)}}else if(Ee.length>0){if(Te&&Ie){let Me=ut(Ee[0]);t.texStorage2D(s.TEXTURE_2D,Ae,ie,Me.width,Me.height)}for(let Me=0,fe=Ee.length;Me<fe;Me++)ve=Ee[Me],Te?X&&t.texSubImage2D(s.TEXTURE_2D,Me,0,0,he,Se,ve):t.texImage2D(s.TEXTURE_2D,Me,ie,he,Se,ve);R.generateMipmaps=!1}else if(Te){if(Ie){let Me=ut(j);t.texStorage2D(s.TEXTURE_2D,Ae,ie,Me.width,Me.height)}X&&t.texSubImage2D(s.TEXTURE_2D,0,0,0,he,Se,j)}else t.texImage2D(s.TEXTURE_2D,0,ie,he,Se,j);f(R)&&y(oe),ke.__version=Ue.version,R.onUpdate&&R.onUpdate(R)}O.__version=R.version}function be(O,R,re){if(R.image.length!==6)return;let oe=ze(O,R),xe=R.source;t.bindTexture(s.TEXTURE_CUBE_MAP,O.__webglTexture,s.TEXTURE0+re);let Ue=n.get(xe);if(xe.version!==Ue.__version||oe===!0){t.activeTexture(s.TEXTURE0+re);let ke=Tt.getPrimaries(Tt.workingColorSpace),G=R.colorSpace===ii?null:Tt.getPrimaries(R.colorSpace),j=R.colorSpace===ii||ke===G?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,R.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,R.premultiplyAlpha),t.pixelStorei(s.UNPACK_ALIGNMENT,R.unpackAlignment),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,j);let he=R.isCompressedTexture||R.image[0].isCompressedTexture,Se=R.image[0]&&R.image[0].isDataTexture,ie=[];for(let _e=0;_e<6;_e++)!he&&!Se?ie[_e]=m(R.image[_e],!0,i.maxCubemapSize):ie[_e]=Se?R.image[_e].image:R.image[_e],ie[_e]=Ut(R,ie[_e]);let ve=ie[0],Ee=r.convert(R.format,R.colorSpace),Te=r.convert(R.type),Ie=g(R.internalFormat,Ee,Te,R.normalized,R.colorSpace),X=R.isVideoTexture!==!0,Ae=Ue.__version===void 0||oe===!0,Me=xe.dataReady,fe=w(R,ve);Ne(s.TEXTURE_CUBE_MAP,R);let Pe;if(he){X&&Ae&&t.texStorage2D(s.TEXTURE_CUBE_MAP,fe,Ie,ve.width,ve.height);for(let _e=0;_e<6;_e++){Pe=ie[_e].mipmaps;for(let qe=0;qe<Pe.length;qe++){let Xe=Pe[qe];R.format!==Jn?Ee!==null?X?Me&&t.compressedTexSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,qe,0,0,Xe.width,Xe.height,Ee,Xe.data):t.compressedTexImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,qe,Ie,Xe.width,Xe.height,0,Xe.data):rt("WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):X?Me&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,qe,0,0,Xe.width,Xe.height,Ee,Te,Xe.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,qe,Ie,Xe.width,Xe.height,0,Ee,Te,Xe.data)}}}else{if(Pe=R.mipmaps,X&&Ae){Pe.length>0&&fe++;let _e=ut(ie[0]);t.texStorage2D(s.TEXTURE_CUBE_MAP,fe,Ie,_e.width,_e.height)}for(let _e=0;_e<6;_e++)if(Se){X?Me&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,0,0,0,ie[_e].width,ie[_e].height,Ee,Te,ie[_e].data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,0,Ie,ie[_e].width,ie[_e].height,0,Ee,Te,ie[_e].data);for(let qe=0;qe<Pe.length;qe++){let U=Pe[qe].image[_e].image;X?Me&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,qe+1,0,0,U.width,U.height,Ee,Te,U.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,qe+1,Ie,U.width,U.height,0,Ee,Te,U.data)}}else{X?Me&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,0,0,0,Ee,Te,ie[_e]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,0,Ie,Ee,Te,ie[_e]);for(let qe=0;qe<Pe.length;qe++){let Xe=Pe[qe];X?Me&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,qe+1,0,0,Ee,Te,Xe.image[_e]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+_e,qe+1,Ie,Ee,Te,Xe.image[_e])}}}f(R)&&y(s.TEXTURE_CUBE_MAP),Ue.__version=xe.version,R.onUpdate&&R.onUpdate(R)}O.__version=R.version}function ue(O,R,re,oe,xe,Ue){let ke=r.convert(re.format,re.colorSpace),G=r.convert(re.type),j=g(re.internalFormat,ke,G,re.normalized,re.colorSpace),he=n.get(R),Se=n.get(re);if(Se.__renderTarget=R,!he.__hasExternalTextures){let ie=Math.max(1,R.width>>Ue),ve=Math.max(1,R.height>>Ue);xe===s.TEXTURE_3D||xe===s.TEXTURE_2D_ARRAY?t.texImage3D(xe,Ue,j,ie,ve,R.depth,0,ke,G,null):t.texImage2D(xe,Ue,j,ie,ve,0,ke,G,null)}t.bindFramebuffer(s.FRAMEBUFFER,O),ht(R)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,oe,xe,Se.__webglTexture,0,et(R)):(xe===s.TEXTURE_2D||xe>=s.TEXTURE_CUBE_MAP_POSITIVE_X&&xe<=s.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&s.framebufferTexture2D(s.FRAMEBUFFER,oe,xe,Se.__webglTexture,Ue),t.bindFramebuffer(s.FRAMEBUFFER,null)}function te(O,R,re){if(s.bindRenderbuffer(s.RENDERBUFFER,O),R.depthBuffer){let oe=R.depthTexture,xe=oe&&oe.isDepthTexture?oe.type:null,Ue=_(R.stencilBuffer,xe),ke=R.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;ht(R)?o.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,et(R),Ue,R.width,R.height):re?s.renderbufferStorageMultisample(s.RENDERBUFFER,et(R),Ue,R.width,R.height):s.renderbufferStorage(s.RENDERBUFFER,Ue,R.width,R.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,ke,s.RENDERBUFFER,O)}else{let oe=R.textures;for(let xe=0;xe<oe.length;xe++){let Ue=oe[xe],ke=r.convert(Ue.format,Ue.colorSpace),G=r.convert(Ue.type),j=g(Ue.internalFormat,ke,G,Ue.normalized,Ue.colorSpace);ht(R)?o.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,et(R),j,R.width,R.height):re?s.renderbufferStorageMultisample(s.RENDERBUFFER,et(R),j,R.width,R.height):s.renderbufferStorage(s.RENDERBUFFER,j,R.width,R.height)}}s.bindRenderbuffer(s.RENDERBUFFER,null)}function pe(O,R,re){let oe=R.isWebGLCubeRenderTarget===!0;if(t.bindFramebuffer(s.FRAMEBUFFER,O),!(R.depthTexture&&R.depthTexture.isDepthTexture))throw new Error("THREE.WebGLTextures: renderTarget.depthTexture must be an instance of THREE.DepthTexture.");let xe=n.get(R.depthTexture);if(xe.__renderTarget=R,(!xe.__webglTexture||R.depthTexture.image.width!==R.width||R.depthTexture.image.height!==R.height)&&(R.depthTexture.image.width=R.width,R.depthTexture.image.height=R.height,R.depthTexture.needsUpdate=!0),oe){if(xe.__webglInit===void 0&&(xe.__webglInit=!0,R.depthTexture.addEventListener("dispose",A)),xe.__webglTexture===void 0){xe.__webglTexture=s.createTexture(),t.bindTexture(s.TEXTURE_CUBE_MAP,xe.__webglTexture),Ne(s.TEXTURE_CUBE_MAP,R.depthTexture);let he=r.convert(R.depthTexture.format),Se=r.convert(R.depthTexture.type),ie;R.depthTexture.format===Ai?ie=s.DEPTH_COMPONENT24:R.depthTexture.format===bs&&(ie=s.DEPTH24_STENCIL8);for(let ve=0;ve<6;ve++)s.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ve,0,ie,R.width,R.height,0,he,Se,null)}}else V(R.depthTexture,0);let Ue=xe.__webglTexture,ke=et(R),G=oe?s.TEXTURE_CUBE_MAP_POSITIVE_X+re:s.TEXTURE_2D,j=R.depthTexture.format===bs?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;if(R.depthTexture.format===Ai)ht(R)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,j,G,Ue,0,ke):s.framebufferTexture2D(s.FRAMEBUFFER,j,G,Ue,0);else if(R.depthTexture.format===bs)ht(R)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,j,G,Ue,0,ke):s.framebufferTexture2D(s.FRAMEBUFFER,j,G,Ue,0);else throw new Error("THREE.WebGLTextures: Unknown depthTexture format.")}function ce(O){let R=n.get(O),re=O.isWebGLCubeRenderTarget===!0;if(R.__boundDepthTexture!==O.depthTexture){let oe=O.depthTexture;if(R.__depthDisposeCallback&&R.__depthDisposeCallback(),oe){let xe=()=>{delete R.__boundDepthTexture,delete R.__depthDisposeCallback,oe.removeEventListener("dispose",xe)};oe.addEventListener("dispose",xe),R.__depthDisposeCallback=xe}R.__boundDepthTexture=oe}if(O.depthTexture&&!R.__autoAllocateDepthBuffer)if(re)for(let oe=0;oe<6;oe++)pe(R.__webglFramebuffer[oe],O,oe);else{let oe=O.texture.mipmaps;oe&&oe.length>0?pe(R.__webglFramebuffer[0],O,0):pe(R.__webglFramebuffer,O,0)}else if(re){R.__webglDepthbuffer=[];for(let oe=0;oe<6;oe++)if(t.bindFramebuffer(s.FRAMEBUFFER,R.__webglFramebuffer[oe]),R.__webglDepthbuffer[oe]===void 0)R.__webglDepthbuffer[oe]=s.createRenderbuffer(),te(R.__webglDepthbuffer[oe],O,!1);else{let xe=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Ue=R.__webglDepthbuffer[oe];s.bindRenderbuffer(s.RENDERBUFFER,Ue),s.framebufferRenderbuffer(s.FRAMEBUFFER,xe,s.RENDERBUFFER,Ue)}}else{let oe=O.texture.mipmaps;if(oe&&oe.length>0?t.bindFramebuffer(s.FRAMEBUFFER,R.__webglFramebuffer[0]):t.bindFramebuffer(s.FRAMEBUFFER,R.__webglFramebuffer),R.__webglDepthbuffer===void 0)R.__webglDepthbuffer=s.createRenderbuffer(),te(R.__webglDepthbuffer,O,!1);else{let xe=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Ue=R.__webglDepthbuffer;s.bindRenderbuffer(s.RENDERBUFFER,Ue),s.framebufferRenderbuffer(s.FRAMEBUFFER,xe,s.RENDERBUFFER,Ue)}}t.bindFramebuffer(s.FRAMEBUFFER,null)}function ne(O,R,re){let oe=n.get(O);R!==void 0&&ue(oe.__webglFramebuffer,O,O.texture,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,0),re!==void 0&&ce(O)}function de(O){let R=O.texture,re=n.get(O),oe=n.get(R);O.addEventListener("dispose",b);let xe=O.textures,Ue=O.isWebGLCubeRenderTarget===!0,ke=xe.length>1;if(ke||(oe.__webglTexture===void 0&&(oe.__webglTexture=s.createTexture()),oe.__version=R.version,a.memory.textures++),Ue){re.__webglFramebuffer=[];for(let G=0;G<6;G++)if(R.mipmaps&&R.mipmaps.length>0){re.__webglFramebuffer[G]=[];for(let j=0;j<R.mipmaps.length;j++)re.__webglFramebuffer[G][j]=s.createFramebuffer()}else re.__webglFramebuffer[G]=s.createFramebuffer()}else{if(R.mipmaps&&R.mipmaps.length>0){re.__webglFramebuffer=[];for(let G=0;G<R.mipmaps.length;G++)re.__webglFramebuffer[G]=s.createFramebuffer()}else re.__webglFramebuffer=s.createFramebuffer();if(ke)for(let G=0,j=xe.length;G<j;G++){let he=n.get(xe[G]);he.__webglTexture===void 0&&(he.__webglTexture=s.createTexture(),a.memory.textures++)}if(O.samples>0&&ht(O)===!1){re.__webglMultisampledFramebuffer=s.createFramebuffer(),re.__webglColorRenderbuffer=[],t.bindFramebuffer(s.FRAMEBUFFER,re.__webglMultisampledFramebuffer);for(let G=0;G<xe.length;G++){let j=xe[G];re.__webglColorRenderbuffer[G]=s.createRenderbuffer(),s.bindRenderbuffer(s.RENDERBUFFER,re.__webglColorRenderbuffer[G]);let he=r.convert(j.format,j.colorSpace),Se=r.convert(j.type),ie=g(j.internalFormat,he,Se,j.normalized,j.colorSpace,O.isXRRenderTarget===!0),ve=et(O);s.renderbufferStorageMultisample(s.RENDERBUFFER,ve,ie,O.width,O.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+G,s.RENDERBUFFER,re.__webglColorRenderbuffer[G])}s.bindRenderbuffer(s.RENDERBUFFER,null),O.depthBuffer&&(re.__webglDepthRenderbuffer=s.createRenderbuffer(),te(re.__webglDepthRenderbuffer,O,!0)),t.bindFramebuffer(s.FRAMEBUFFER,null)}}if(Ue){t.bindTexture(s.TEXTURE_CUBE_MAP,oe.__webglTexture),Ne(s.TEXTURE_CUBE_MAP,R);for(let G=0;G<6;G++)if(R.mipmaps&&R.mipmaps.length>0)for(let j=0;j<R.mipmaps.length;j++)ue(re.__webglFramebuffer[G][j],O,R,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+G,j);else ue(re.__webglFramebuffer[G],O,R,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+G,0);f(R)&&y(s.TEXTURE_CUBE_MAP),t.unbindTexture()}else if(ke){for(let G=0,j=xe.length;G<j;G++){let he=xe[G],Se=n.get(he),ie=s.TEXTURE_2D;(O.isWebGL3DRenderTarget||O.isWebGLArrayRenderTarget)&&(ie=O.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(ie,Se.__webglTexture),Ne(ie,he),ue(re.__webglFramebuffer,O,he,s.COLOR_ATTACHMENT0+G,ie,0),f(he)&&y(ie)}t.unbindTexture()}else{let G=s.TEXTURE_2D;if((O.isWebGL3DRenderTarget||O.isWebGLArrayRenderTarget)&&(G=O.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(G,oe.__webglTexture),Ne(G,R),R.mipmaps&&R.mipmaps.length>0)for(let j=0;j<R.mipmaps.length;j++)ue(re.__webglFramebuffer[j],O,R,s.COLOR_ATTACHMENT0,G,j);else ue(re.__webglFramebuffer,O,R,s.COLOR_ATTACHMENT0,G,0);f(R)&&y(G),t.unbindTexture()}O.depthBuffer&&ce(O)}function we(O){let R=O.textures;for(let re=0,oe=R.length;re<oe;re++){let xe=R[re];if(f(xe)){let Ue=T(O),ke=n.get(xe).__webglTexture;t.bindTexture(Ue,ke),y(Ue),t.unbindTexture()}}}let Ve=[],Ke=[];function it(O){if(O.samples>0){if(ht(O)===!1){let R=O.textures,re=O.width,oe=O.height,xe=s.COLOR_BUFFER_BIT,Ue=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,ke=n.get(O),G=R.length>1;if(G)for(let he=0;he<R.length;he++)t.bindFramebuffer(s.FRAMEBUFFER,ke.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+he,s.RENDERBUFFER,null),t.bindFramebuffer(s.FRAMEBUFFER,ke.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+he,s.TEXTURE_2D,null,0);t.bindFramebuffer(s.READ_FRAMEBUFFER,ke.__webglMultisampledFramebuffer);let j=O.texture.mipmaps;j&&j.length>0?t.bindFramebuffer(s.DRAW_FRAMEBUFFER,ke.__webglFramebuffer[0]):t.bindFramebuffer(s.DRAW_FRAMEBUFFER,ke.__webglFramebuffer);for(let he=0;he<R.length;he++){if(O.resolveDepthBuffer&&(O.depthBuffer&&(xe|=s.DEPTH_BUFFER_BIT),O.stencilBuffer&&O.resolveStencilBuffer&&(xe|=s.STENCIL_BUFFER_BIT)),G){s.framebufferRenderbuffer(s.READ_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.RENDERBUFFER,ke.__webglColorRenderbuffer[he]);let Se=n.get(R[he]).__webglTexture;s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,Se,0)}s.blitFramebuffer(0,0,re,oe,0,0,re,oe,xe,s.NEAREST),l===!0&&(Ve.length=0,Ke.length=0,Ve.push(s.COLOR_ATTACHMENT0+he),O.depthBuffer&&O.storeMultisampledDepthBuffer===!1&&(Ve.push(Ue),Ke.push(Ue),s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,Ke)),s.invalidateFramebuffer(s.READ_FRAMEBUFFER,Ve))}if(t.bindFramebuffer(s.READ_FRAMEBUFFER,null),t.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),G)for(let he=0;he<R.length;he++){t.bindFramebuffer(s.FRAMEBUFFER,ke.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+he,s.RENDERBUFFER,ke.__webglColorRenderbuffer[he]);let Se=n.get(R[he]).__webglTexture;t.bindFramebuffer(s.FRAMEBUFFER,ke.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+he,s.TEXTURE_2D,Se,0)}t.bindFramebuffer(s.DRAW_FRAMEBUFFER,ke.__webglMultisampledFramebuffer)}else if(O.depthBuffer&&O.storeMultisampledDepthBuffer===!1&&l){let R=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,[R])}}}function et(O){return Math.min(i.maxSamples,O.samples)}function ht(O){let R=n.get(O);return O.samples>0&&e.has("WEBGL_multisampled_render_to_texture")===!0&&R.__useRenderToTexture!==!1}function J(O){let R=a.render.frame;h.get(O)!==R&&(h.set(O,R),O.update())}function Ut(O,R){let re=O.colorSpace,oe=O.format,xe=O.type;return O.isCompressedTexture===!0||O.isVideoTexture===!0||re!==Un&&re!==ii&&(Tt.getTransfer(re)===Gt?(oe!==Jn||xe!==Gn)&&rt("WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):pt("WebGLTextures: Unsupported texture color space:",re)),R}function ut(O){return typeof HTMLImageElement<"u"&&O instanceof HTMLImageElement?(c.width=O.naturalWidth||O.width,c.height=O.naturalHeight||O.height):typeof VideoFrame<"u"&&O instanceof VideoFrame?(c.width=O.displayWidth,c.height=O.displayHeight):(c.width=O.width,c.height=O.height),c}this.allocateTextureUnit=H,this.resetTextureUnits=F,this.getTextureUnits=I,this.setTextureUnits=N,this.setTexture2D=V,this.setTexture2DArray=k,this.setTexture3D=E,this.setTextureCube=Z,this.rebindTextures=ne,this.setupRenderTarget=de,this.updateRenderTargetMipmap=we,this.updateMultisampleRenderTarget=it,this.setupDepthRenderbuffer=ce,this.setupFrameBufferTexture=ue,this.useMultisampledRTT=ht,this.isReversedDepthBuffer=function(){return t.buffers.depth.getReversed()}}function U_(s,e){function t(n,i=ii){let r,a=Tt.getTransfer(i);if(n===Gn)return s.UNSIGNED_BYTE;if(n===Dl)return s.UNSIGNED_SHORT_4_4_4_4;if(n===Ll)return s.UNSIGNED_SHORT_5_5_5_1;if(n===Oh)return s.UNSIGNED_INT_5_9_9_9_REV;if(n===zh)return s.UNSIGNED_INT_10F_11F_11F_REV;if(n===Nh)return s.BYTE;if(n===Uh)return s.SHORT;if(n===Br)return s.UNSIGNED_SHORT;if(n===Il)return s.INT;if(n===vi)return s.UNSIGNED_INT;if(n===Yn)return s.FLOAT;if(n===pn)return s.HALF_FLOAT;if(n===kh)return s.ALPHA;if(n===Bh)return s.RGB;if(n===Jn)return s.RGBA;if(n===Ai)return s.DEPTH_COMPONENT;if(n===bs)return s.DEPTH_STENCIL;if(n===Fl)return s.RED;if(n===Nl)return s.RED_INTEGER;if(n===vs)return s.RG;if(n===Ul)return s.RG_INTEGER;if(n===Ol)return s.RGBA_INTEGER;if(n===Ja||n===Za||n===$a||n===Qa)if(a===Gt)if(r=e.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(n===Ja)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(n===Za)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(n===$a)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(n===Qa)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=e.get("WEBGL_compressed_texture_s3tc"),r!==null){if(n===Ja)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(n===Za)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(n===$a)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(n===Qa)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(n===zl||n===kl||n===Bl||n===Hl)if(r=e.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(n===zl)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(n===kl)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(n===Bl)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(n===Hl)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(n===Gl||n===Vl||n===Wl||n===ql||n===Xl||n===eo||n===jl)if(r=e.get("WEBGL_compressed_texture_etc"),r!==null){if(n===Gl||n===Vl)return a===Gt?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(n===Wl)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC;if(n===ql)return r.COMPRESSED_R11_EAC;if(n===Xl)return r.COMPRESSED_SIGNED_R11_EAC;if(n===eo)return r.COMPRESSED_RG11_EAC;if(n===jl)return r.COMPRESSED_SIGNED_RG11_EAC}else return null;if(n===Kl||n===Yl||n===Jl||n===Zl||n===$l||n===Ql||n===ec||n===tc||n===nc||n===ic||n===sc||n===rc||n===ac||n===oc)if(r=e.get("WEBGL_compressed_texture_astc"),r!==null){if(n===Kl)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(n===Yl)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(n===Jl)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(n===Zl)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(n===$l)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(n===Ql)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(n===ec)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(n===tc)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(n===nc)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(n===ic)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(n===sc)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(n===rc)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(n===ac)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(n===oc)return a===Gt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(n===lc||n===cc||n===hc)if(r=e.get("EXT_texture_compression_bptc"),r!==null){if(n===lc)return a===Gt?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(n===cc)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(n===hc)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(n===uc||n===dc||n===to||n===fc)if(r=e.get("EXT_texture_compression_rgtc"),r!==null){if(n===uc)return r.COMPRESSED_RED_RGTC1_EXT;if(n===dc)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(n===to)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(n===fc)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return n===Hr?s.UNSIGNED_INT_24_8:s[n]!==void 0?s[n]:null}return{convert:t}}var O_=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,z_=`
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

}`,cu=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(e,t){if(this.texture===null){let n=new Ta(e.texture);(e.depthNear!==t.depthNear||e.depthFar!==t.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=n}}getMesh(e){if(this.texture!==null&&this.mesh===null){let t=e.cameras[0].viewport,n=new yt({vertexShader:O_,fragmentShader:z_,uniforms:{depthColor:{value:this.texture},depthWidth:{value:t.z},depthHeight:{value:t.w}}});this.mesh=new st(new fn(20,20),n)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},hu=class extends ei{constructor(e,t){super();let n=this,i=null,r=1,a=null,o="local-floor",l=1,c=null,h=null,u=null,d=null,p=null,x=null,v=typeof XRWebGLBinding<"u",m=new cu,f={},y=t.getContextAttributes(),T=null,g=null,_=[],w=[],A=new Be,b=null,C=null,S=new dn;S.viewport=new Ot;let P=new dn;P.viewport=new Ot;let M=[S,P],F=new wl,I=null,N=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(se){let le=_[se];return le===void 0&&(le=new Sr,_[se]=le),le.getTargetRaySpace()},this.getControllerGrip=function(se){let le=_[se];return le===void 0&&(le=new Sr,_[se]=le),le.getGripSpace()},this.getHand=function(se){let le=_[se];return le===void 0&&(le=new Sr,_[se]=le),le.getHandSpace()};function H(se){let le=w.indexOf(se.inputSource);if(le===-1)return;let De=_[le];De!==void 0&&(De.update(se.inputSource,se.frame,c||a),De.dispatchEvent({type:se.type,data:se.inputSource}))}function D(){i.removeEventListener("select",H),i.removeEventListener("selectstart",H),i.removeEventListener("selectend",H),i.removeEventListener("squeeze",H),i.removeEventListener("squeezestart",H),i.removeEventListener("squeezeend",H),i.removeEventListener("end",D),i.removeEventListener("inputsourceschange",V);for(let se=0;se<_.length;se++){let le=w[se];le!==null&&(w[se]=null,_[se].disconnect(le))}I=null,N=null,m.reset();for(let se in f)delete f[se];if(e.setRenderTarget(T),p=null,d=null,u=null,i=null,g=null,ze.stop(),n.isPresenting=!1,e.setPixelRatio(b),e.setSize(A.width,A.height,!1),C!==null){let se=C.camera;se.fov=C.fov,se.zoom=C.zoom,se.updateProjectionMatrix(),C=null}n.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(se){r=se,n.isPresenting===!0&&rt("WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(se){o=se,n.isPresenting===!0&&rt("WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return c||a},this.setReferenceSpace=function(se){c=se},this.getBaseLayer=function(){return d!==null?d:p},this.getBinding=function(){return u===null&&v&&(u=new XRWebGLBinding(i,t)),u},this.getFrame=function(){return x},this.getSession=function(){return i},this.setSession=async function(se){if(i=se,i!==null){if(T=e.getRenderTarget(),i.addEventListener("select",H),i.addEventListener("selectstart",H),i.addEventListener("selectend",H),i.addEventListener("squeeze",H),i.addEventListener("squeezestart",H),i.addEventListener("squeezeend",H),i.addEventListener("end",D),i.addEventListener("inputsourceschange",V),y.xrCompatible!==!0&&await t.makeXRCompatible(),b=e.getPixelRatio(),e.getSize(A),v&&"createProjectionLayer"in XRWebGLBinding.prototype){let De=null,be=null,ue=null;y.depth&&(ue=y.stencil?t.DEPTH24_STENCIL8:t.DEPTH_COMPONENT24,De=y.stencil?bs:Ai,be=y.stencil?Hr:vi);let te={colorFormat:t.RGBA8,depthFormat:ue,scaleFactor:r};u=this.getBinding(),d=u.createProjectionLayer(te),i.updateRenderState({layers:[d]}),e.setPixelRatio(1),e.setSize(d.textureWidth,d.textureHeight,!1),g=new an(d.textureWidth,d.textureHeight,{format:Jn,type:Gn,depthTexture:new us(d.textureWidth,d.textureHeight,be,void 0,void 0,void 0,void 0,void 0,void 0,De),stencilBuffer:y.stencil,colorSpace:e.outputColorSpace,samples:y.antialias?4:0,resolveDepthBuffer:d.ignoreDepthValues===!1,resolveStencilBuffer:d.ignoreDepthValues===!1,storeMultisampledDepthBuffer:d.ignoreDepthValues===!1,storeMultisampledStencilBuffer:d.ignoreDepthValues===!1})}else{let De={antialias:y.antialias,alpha:!0,depth:y.depth,stencil:y.stencil,framebufferScaleFactor:r};p=new XRWebGLLayer(i,t,De),i.updateRenderState({baseLayer:p}),e.setPixelRatio(1),e.setSize(p.framebufferWidth,p.framebufferHeight,!1),g=new an(p.framebufferWidth,p.framebufferHeight,{format:Jn,type:Gn,colorSpace:e.outputColorSpace,stencilBuffer:y.stencil,resolveDepthBuffer:p.ignoreDepthValues===!1,resolveStencilBuffer:p.ignoreDepthValues===!1,storeMultisampledDepthBuffer:p.ignoreDepthValues===!1,storeMultisampledStencilBuffer:p.ignoreDepthValues===!1})}g.isXRRenderTarget=!0,this.setFoveation(l),c=null,a=await i.requestReferenceSpace(o),ze.setContext(i),ze.start(),n.isPresenting=!0,n.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(i!==null)return i.environmentBlendMode},this.getDepthTexture=function(){return m.getDepthTexture()};function V(se){for(let le=0;le<se.removed.length;le++){let De=se.removed[le],be=w.indexOf(De);be>=0&&(w[be]=null,_[be].disconnect(De))}for(let le=0;le<se.added.length;le++){let De=se.added[le],be=w.indexOf(De);if(be===-1){for(let te=0;te<_.length;te++)if(te>=w.length){w.push(De),be=te;break}else if(w[te]===null){w[te]=De,be=te;break}if(be===-1)break}let ue=_[be];ue&&ue.connect(De)}}let k=new B,E=new B;function Z(se,le,De){k.setFromMatrixPosition(le.matrixWorld),E.setFromMatrixPosition(De.matrixWorld);let be=k.distanceTo(E),ue=le.projectionMatrix.elements,te=De.projectionMatrix.elements,pe=ue[14]/(ue[10]-1),ce=ue[14]/(ue[10]+1),ne=(ue[9]+1)/ue[5],de=(ue[9]-1)/ue[5],we=(ue[8]-1)/ue[0],Ve=(te[8]+1)/te[0],Ke=pe*we,it=pe*Ve,et=be/(-we+Ve),ht=et*-we;if(le.matrixWorld.decompose(se.position,se.quaternion,se.scale),se.translateX(ht),se.translateZ(et),se.matrixWorld.compose(se.position,se.quaternion,se.scale),se.matrixWorldInverse.copy(se.matrixWorld).invert(),ue[10]===-1)se.projectionMatrix.copy(le.projectionMatrix),se.projectionMatrixInverse.copy(le.projectionMatrixInverse);else{let J=pe+et,Ut=ce+et,ut=Ke-ht,O=it+(be-ht),R=ne*ce/Ut*J,re=de*ce/Ut*J;se.projectionMatrix.makePerspective(ut,O,R,re,J,Ut),se.projectionMatrixInverse.copy(se.projectionMatrix).invert()}}function W(se,le){le===null?se.matrixWorld.copy(se.matrix):se.matrixWorld.multiplyMatrices(le.matrixWorld,se.matrix),se.matrixWorldInverse.copy(se.matrixWorld).invert()}this.updateCamera=function(se){if(i===null)return;let le=se.near,De=se.far;m.texture!==null&&(m.depthNear>0&&(le=m.depthNear),m.depthFar>0&&(De=m.depthFar)),F.near=P.near=S.near=le,F.far=P.far=S.far=De,(I!==F.near||N!==F.far)&&(i.updateRenderState({depthNear:F.near,depthFar:F.far}),I=F.near,N=F.far),F.layers.mask=se.layers.mask|6,S.layers.mask=F.layers.mask&-5,P.layers.mask=F.layers.mask&-3;let be=se.parent,ue=F.cameras;W(F,be);for(let te=0;te<ue.length;te++)W(ue[te],be);ue.length===2?Z(F,S,P):F.projectionMatrix.copy(S.projectionMatrix),C===null&&se.isPerspectiveCamera&&(C={camera:se,fov:se.fov,zoom:se.zoom}),Q(se,F,be)};function Q(se,le,De){De===null?se.matrix.copy(le.matrixWorld):(se.matrix.copy(De.matrixWorld),se.matrix.invert(),se.matrix.multiply(le.matrixWorld)),se.matrix.decompose(se.position,se.quaternion,se.scale),se.updateMatrixWorld(!0),se.projectionMatrix.copy(le.projectionMatrix),se.projectionMatrixInverse.copy(le.projectionMatrixInverse),se.isPerspectiveCamera&&(se.fov=Ns*2*Math.atan(1/se.projectionMatrix.elements[5]),se.zoom=1)}this.getCamera=function(){return F},this.getFoveation=function(){if(!(d===null&&p===null))return l},this.setFoveation=function(se){l=se,d!==null&&(d.fixedFoveation=se),p!==null&&p.fixedFoveation!==void 0&&(p.fixedFoveation=se)},this.hasDepthSensing=function(){return m.texture!==null},this.getDepthSensingMesh=function(){return m.getMesh(F)},this.getCameraTexture=function(se){return f[se]};let Fe=null;function Ne(se,le){if(h=le.getViewerPose(c||a),x=le,h!==null){let De=h.views;p!==null&&(e.setRenderTargetFramebuffer(g,p.framebuffer),e.setRenderTarget(g));let be=!1;De.length!==F.cameras.length&&(F.cameras.length=0,be=!0);for(let ce=0;ce<De.length;ce++){let ne=De[ce],de=null;if(p!==null)de=p.getViewport(ne);else{let Ve=u.getViewSubImage(d,ne);de=Ve.viewport,ce===0&&(e.setRenderTargetTextures(g,Ve.colorTexture,Ve.depthStencilTexture),e.setRenderTarget(g))}let we=M[ce];we===void 0&&(we=new dn,we.layers.enable(ce),we.viewport=new Ot,M[ce]=we),we.matrix.fromArray(ne.transform.matrix),we.matrix.decompose(we.position,we.quaternion,we.scale),we.projectionMatrix.fromArray(ne.projectionMatrix),we.projectionMatrixInverse.copy(we.projectionMatrix).invert(),we.viewport.set(de.x,de.y,de.width,de.height),ce===0&&(F.matrix.copy(we.matrix),F.matrix.decompose(F.position,F.quaternion,F.scale)),be===!0&&F.cameras.push(we)}let ue=i.enabledFeatures;if(ue&&ue.includes("depth-sensing")&&i.depthUsage=="gpu-optimized"&&v){u=n.getBinding();let ce=u.getDepthInformation(De[0]);ce&&ce.isValid&&ce.texture&&m.init(ce,i.renderState)}if(ue&&ue.includes("camera-access")&&v){e.state.unbindTexture(),u=n.getBinding();for(let ce=0;ce<De.length;ce++){let ne=De[ce].camera;if(ne){let de=f[ne];de||(de=new Ta,f[ne]=de);let we=u.getCameraImage(ne);de.sourceTexture=we}}}}for(let De=0;De<_.length;De++){let be=w[De],ue=_[De];be!==null&&ue!==void 0&&ue.update(be,le,c||a)}Fe&&Fe(se,le),le.detectedPlanes&&n.dispatchEvent({type:"planesdetected",data:le}),x=null}let ze=new fp;ze.setAnimationLoop(Ne),this.setAnimationLoop=function(se){Fe=se},this.dispose=function(){}}},k_=new ft,vp=new xt;vp.set(-1,0,0,0,1,0,0,0,1);function B_(s,e){function t(m,f){m.matrixAutoUpdate===!0&&m.updateMatrix(),f.value.copy(m.matrix)}function n(m,f){f.color.getRGB(m.fogColor.value,Xh(s)),f.isFog?(m.fogNear.value=f.near,m.fogFar.value=f.far):f.isFogExp2&&(m.fogDensity.value=f.density)}function i(m,f,y,T,g){f.isNodeMaterial?f.uniformsNeedUpdate=!1:f.isMeshBasicMaterial?r(m,f):f.isMeshLambertMaterial?(r(m,f),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)):f.isMeshToonMaterial?(r(m,f),u(m,f)):f.isMeshPhongMaterial?(r(m,f),h(m,f),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)):f.isMeshStandardMaterial?(r(m,f),d(m,f),f.isMeshPhysicalMaterial&&p(m,f,g)):f.isMeshMatcapMaterial?(r(m,f),x(m,f)):f.isMeshDepthMaterial?r(m,f):f.isMeshDistanceMaterial?(r(m,f),v(m,f)):f.isMeshNormalMaterial?r(m,f):f.isLineBasicMaterial?(a(m,f),f.isLineDashedMaterial&&o(m,f)):f.isPointsMaterial?l(m,f,y,T):f.isSpriteMaterial?c(m,f):f.isShadowMaterial?(m.color.value.copy(f.color),m.opacity.value=f.opacity):f.isShaderMaterial&&(f.uniformsNeedUpdate=!1)}function r(m,f){m.opacity.value=f.opacity,f.color&&m.diffuse.value.copy(f.color),f.emissive&&m.emissive.value.copy(f.emissive).multiplyScalar(f.emissiveIntensity),f.map&&(m.map.value=f.map,t(f.map,m.mapTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.bumpMap&&(m.bumpMap.value=f.bumpMap,t(f.bumpMap,m.bumpMapTransform),m.bumpScale.value=f.bumpScale,f.side===mn&&(m.bumpScale.value*=-1)),f.normalMap&&(m.normalMap.value=f.normalMap,t(f.normalMap,m.normalMapTransform),m.normalScale.value.copy(f.normalScale),f.side===mn&&m.normalScale.value.negate()),f.displacementMap&&(m.displacementMap.value=f.displacementMap,t(f.displacementMap,m.displacementMapTransform),m.displacementScale.value=f.displacementScale,m.displacementBias.value=f.displacementBias),f.emissiveMap&&(m.emissiveMap.value=f.emissiveMap,t(f.emissiveMap,m.emissiveMapTransform)),f.specularMap&&(m.specularMap.value=f.specularMap,t(f.specularMap,m.specularMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest);let y=e.get(f),T=y.envMap,g=y.envMapRotation;T&&(m.envMap.value=T,m.envMapRotation.value.setFromMatrix4(k_.makeRotationFromEuler(g)).transpose(),T.isCubeTexture&&T.isRenderTargetTexture===!1&&m.envMapRotation.value.premultiply(vp),m.reflectivity.value=f.reflectivity,m.ior.value=f.ior,m.refractionRatio.value=f.refractionRatio),f.lightMap&&(m.lightMap.value=f.lightMap,m.lightMapIntensity.value=f.lightMapIntensity,t(f.lightMap,m.lightMapTransform)),f.aoMap&&(m.aoMap.value=f.aoMap,m.aoMapIntensity.value=f.aoMapIntensity,t(f.aoMap,m.aoMapTransform))}function a(m,f){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,f.map&&(m.map.value=f.map,t(f.map,m.mapTransform))}function o(m,f){m.dashSize.value=f.dashSize,m.totalSize.value=f.dashSize+f.gapSize,m.scale.value=f.scale}function l(m,f,y,T){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,m.size.value=f.size*y,m.scale.value=T*.5,f.map&&(m.map.value=f.map,t(f.map,m.uvTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest)}function c(m,f){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,m.rotation.value=f.rotation,f.map&&(m.map.value=f.map,t(f.map,m.mapTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest)}function h(m,f){m.specular.value.copy(f.specular),m.shininess.value=Math.max(f.shininess,1e-4)}function u(m,f){f.gradientMap&&(m.gradientMap.value=f.gradientMap)}function d(m,f){m.metalness.value=f.metalness,f.metalnessMap&&(m.metalnessMap.value=f.metalnessMap,t(f.metalnessMap,m.metalnessMapTransform)),m.roughness.value=f.roughness,f.roughnessMap&&(m.roughnessMap.value=f.roughnessMap,t(f.roughnessMap,m.roughnessMapTransform)),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)}function p(m,f,y){m.ior.value=f.ior,f.sheen>0&&(m.sheenColor.value.copy(f.sheenColor).multiplyScalar(f.sheen),m.sheenRoughness.value=f.sheenRoughness,f.sheenColorMap&&(m.sheenColorMap.value=f.sheenColorMap,t(f.sheenColorMap,m.sheenColorMapTransform)),f.sheenRoughnessMap&&(m.sheenRoughnessMap.value=f.sheenRoughnessMap,t(f.sheenRoughnessMap,m.sheenRoughnessMapTransform))),f.clearcoat>0&&(m.clearcoat.value=f.clearcoat,m.clearcoatRoughness.value=f.clearcoatRoughness,f.clearcoatMap&&(m.clearcoatMap.value=f.clearcoatMap,t(f.clearcoatMap,m.clearcoatMapTransform)),f.clearcoatRoughnessMap&&(m.clearcoatRoughnessMap.value=f.clearcoatRoughnessMap,t(f.clearcoatRoughnessMap,m.clearcoatRoughnessMapTransform)),f.clearcoatNormalMap&&(m.clearcoatNormalMap.value=f.clearcoatNormalMap,t(f.clearcoatNormalMap,m.clearcoatNormalMapTransform),m.clearcoatNormalScale.value.copy(f.clearcoatNormalScale),f.side===mn&&m.clearcoatNormalScale.value.negate())),f.dispersion>0&&(m.dispersion.value=f.dispersion),f.retroreflectivity>0&&(m.retroreflectivity.value=f.retroreflectivity),f.iridescence>0&&(m.iridescence.value=f.iridescence,m.iridescenceIOR.value=f.iridescenceIOR,m.iridescenceThicknessMinimum.value=f.iridescenceThicknessRange[0],m.iridescenceThicknessMaximum.value=f.iridescenceThicknessRange[1],f.iridescenceMap&&(m.iridescenceMap.value=f.iridescenceMap,t(f.iridescenceMap,m.iridescenceMapTransform)),f.iridescenceThicknessMap&&(m.iridescenceThicknessMap.value=f.iridescenceThicknessMap,t(f.iridescenceThicknessMap,m.iridescenceThicknessMapTransform))),f.transmission>0&&(m.transmission.value=f.transmission,m.transmissionSamplerMap.value=y.texture,m.transmissionSamplerSize.value.set(y.width,y.height),f.transmissionMap&&(m.transmissionMap.value=f.transmissionMap,t(f.transmissionMap,m.transmissionMapTransform)),m.thickness.value=f.thickness,f.thicknessMap&&(m.thicknessMap.value=f.thicknessMap,t(f.thicknessMap,m.thicknessMapTransform)),m.attenuationDistance.value=f.attenuationDistance,m.attenuationColor.value.copy(f.attenuationColor)),f.anisotropy>0&&(m.anisotropyVector.value.set(f.anisotropy*Math.cos(f.anisotropyRotation),f.anisotropy*Math.sin(f.anisotropyRotation)),f.anisotropyMap&&(m.anisotropyMap.value=f.anisotropyMap,t(f.anisotropyMap,m.anisotropyMapTransform))),m.specularIntensity.value=f.specularIntensity,m.specularColor.value.copy(f.specularColor),f.specularColorMap&&(m.specularColorMap.value=f.specularColorMap,t(f.specularColorMap,m.specularColorMapTransform)),f.specularIntensityMap&&(m.specularIntensityMap.value=f.specularIntensityMap,t(f.specularIntensityMap,m.specularIntensityMapTransform))}function x(m,f){f.matcap&&(m.matcap.value=f.matcap)}function v(m,f){let y=e.get(f).light;m.referencePosition.value.setFromMatrixPosition(y.matrixWorld),m.nearDistance.value=y.shadow.camera.near,m.farDistance.value=y.shadow.camera.far}return{refreshFogUniforms:n,refreshMaterialUniforms:i}}function H_(s,e,t,n){let i={},r={},a=[],o=s.getParameter(s.MAX_UNIFORM_BUFFER_BINDINGS);function l(g,_){let w=_.program;n.uniformBlockBinding(g,w)}function c(g,_){let w=i[g.id];w===void 0&&(m(g),w=h(g),i[g.id]=w,g.addEventListener("dispose",y));let A=_.program;n.updateUBOMapping(g,A);let b=e.render.frame;r[g.id]!==b&&(d(g),r[g.id]=b)}function h(g){let _=u();g.__bindingPointIndex=_;let w=s.createBuffer(),A=g.__size,b=g.usage;return s.bindBuffer(s.UNIFORM_BUFFER,w),s.bufferData(s.UNIFORM_BUFFER,A,b),s.bindBuffer(s.UNIFORM_BUFFER,null),s.bindBufferBase(s.UNIFORM_BUFFER,_,w),w}function u(){for(let g=0;g<o;g++)if(a.indexOf(g)===-1)return a.push(g),g;return pt("WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function d(g){let _=i[g.id],w=g.uniforms,A=g.__cache;s.bindBuffer(s.UNIFORM_BUFFER,_);for(let b=0,C=w.length;b<C;b++){let S=w[b];if(Array.isArray(S))for(let P=0,M=S.length;P<M;P++)p(S[P],b,P,A);else p(S,b,0,A)}s.bindBuffer(s.UNIFORM_BUFFER,null)}function p(g,_,w,A){if(v(g,_,w,A)===!0){let b=g.__offset,C=g.value;if(Array.isArray(C)){let S=0;for(let P=0;P<C.length;P++){let M=C[P],F=f(M);x(M,g.__data,S),typeof M!="number"&&typeof M!="boolean"&&!M.isMatrix3&&!ArrayBuffer.isView(M)&&(S+=F.storage/Float32Array.BYTES_PER_ELEMENT)}}else x(C,g.__data,0);s.bufferSubData(s.UNIFORM_BUFFER,b,g.__data)}}function x(g,_,w){typeof g=="number"||typeof g=="boolean"?_[0]=g:g.isMatrix3?(_[0]=g.elements[0],_[1]=g.elements[1],_[2]=g.elements[2],_[3]=0,_[4]=g.elements[3],_[5]=g.elements[4],_[6]=g.elements[5],_[7]=0,_[8]=g.elements[6],_[9]=g.elements[7],_[10]=g.elements[8],_[11]=0):ArrayBuffer.isView(g)?_.set(new g.constructor(g.buffer,g.byteOffset,_.length)):g.toArray(_,w)}function v(g,_,w,A){let b=g.value,C=_+"_"+w;if(A[C]===void 0)return typeof b=="number"||typeof b=="boolean"?A[C]=b:ArrayBuffer.isView(b)?A[C]=b.slice():A[C]=b.clone(),!0;{let S=A[C];if(typeof b=="number"||typeof b=="boolean"){if(S!==b)return A[C]=b,!0}else{if(ArrayBuffer.isView(b))return!0;if(S.equals(b)===!1)return S.copy(b),!0}}return!1}function m(g){let _=g.uniforms,w=0,A=16;for(let C=0,S=_.length;C<S;C++){let P=Array.isArray(_[C])?_[C]:[_[C]];for(let M=0,F=P.length;M<F;M++){let I=P[M],N=Array.isArray(I.value)?I.value:[I.value];for(let H=0,D=N.length;H<D;H++){let V=N[H],k=f(V),E=w%A,Z=E%k.boundary,W=E+Z;w+=Z,W!==0&&A-W<k.storage&&(w+=A-W),I.__data=new Float32Array(k.storage/Float32Array.BYTES_PER_ELEMENT),I.__offset=w,w+=k.storage}}}let b=w%A;return b>0&&(w+=A-b),g.__size=w,g.__cache={},this}function f(g){let _={boundary:0,storage:0};return typeof g=="number"||typeof g=="boolean"?(_.boundary=4,_.storage=4):g.isVector2?(_.boundary=8,_.storage=8):g.isVector3||g.isColor?(_.boundary=16,_.storage=12):g.isVector4?(_.boundary=16,_.storage=16):g.isMatrix3?(_.boundary=48,_.storage=48):g.isMatrix4?(_.boundary=64,_.storage=64):g.isTexture?rt("WebGLRenderer: Texture samplers can not be part of an uniforms group."):ArrayBuffer.isView(g)?(_.boundary=16,_.storage=g.byteLength):rt("WebGLRenderer: Unsupported uniform value type.",g),_}function y(g){let _=g.target;_.removeEventListener("dispose",y);let w=a.indexOf(_.__bindingPointIndex);a.splice(w,1),s.deleteBuffer(i[_.id]),delete i[_.id],delete r[_.id]}function T(){for(let g in i)s.deleteBuffer(i[g]);a=[],i={},r={}}return{bind:l,update:c,dispose:T}}var G_=new Uint16Array([12469,15057,12620,14925,13266,14620,13807,14376,14323,13990,14545,13625,14713,13328,14840,12882,14931,12528,14996,12233,15039,11829,15066,11525,15080,11295,15085,10976,15082,10705,15073,10495,13880,14564,13898,14542,13977,14430,14158,14124,14393,13732,14556,13410,14702,12996,14814,12596,14891,12291,14937,11834,14957,11489,14958,11194,14943,10803,14921,10506,14893,10278,14858,9960,14484,14039,14487,14025,14499,13941,14524,13740,14574,13468,14654,13106,14743,12678,14818,12344,14867,11893,14889,11509,14893,11180,14881,10751,14852,10428,14812,10128,14765,9754,14712,9466,14764,13480,14764,13475,14766,13440,14766,13347,14769,13070,14786,12713,14816,12387,14844,11957,14860,11549,14868,11215,14855,10751,14825,10403,14782,10044,14729,9651,14666,9352,14599,9029,14967,12835,14966,12831,14963,12804,14954,12723,14936,12564,14917,12347,14900,11958,14886,11569,14878,11247,14859,10765,14828,10401,14784,10011,14727,9600,14660,9289,14586,8893,14508,8533,15111,12234,15110,12234,15104,12216,15092,12156,15067,12010,15028,11776,14981,11500,14942,11205,14902,10752,14861,10393,14812,9991,14752,9570,14682,9252,14603,8808,14519,8445,14431,8145,15209,11449,15208,11451,15202,11451,15190,11438,15163,11384,15117,11274,15055,10979,14994,10648,14932,10343,14871,9936,14803,9532,14729,9218,14645,8742,14556,8381,14461,8020,14365,7603,15273,10603,15272,10607,15267,10619,15256,10631,15231,10614,15182,10535,15118,10389,15042,10167,14963,9787,14883,9447,14800,9115,14710,8665,14615,8318,14514,7911,14411,7507,14279,7198,15314,9675,15313,9683,15309,9712,15298,9759,15277,9797,15229,9773,15166,9668,15084,9487,14995,9274,14898,8910,14800,8539,14697,8234,14590,7790,14479,7409,14367,7067,14178,6621,15337,8619,15337,8631,15333,8677,15325,8769,15305,8871,15264,8940,15202,8909,15119,8775,15022,8565,14916,8328,14804,8009,14688,7614,14569,7287,14448,6888,14321,6483,14088,6171,15350,7402,15350,7419,15347,7480,15340,7613,15322,7804,15287,7973,15229,8057,15148,8012,15046,7846,14933,7611,14810,7357,14682,7069,14552,6656,14421,6316,14251,5948,14007,5528,15356,5942,15356,5977,15353,6119,15348,6294,15332,6551,15302,6824,15249,7044,15171,7122,15070,7050,14949,6861,14818,6611,14679,6349,14538,6067,14398,5651,14189,5311,13935,4958,15359,4123,15359,4153,15356,4296,15353,4646,15338,5160,15311,5508,15263,5829,15188,6042,15088,6094,14966,6001,14826,5796,14678,5543,14527,5287,14377,4985,14133,4586,13869,4257,15360,1563,15360,1642,15358,2076,15354,2636,15341,3350,15317,4019,15273,4429,15203,4732,15105,4911,14981,4932,14836,4818,14679,4621,14517,4386,14359,4156,14083,3795,13808,3437,15360,122,15360,137,15358,285,15355,636,15344,1274,15322,2177,15281,2765,15215,3223,15120,3451,14995,3569,14846,3567,14681,3466,14511,3305,14344,3121,14037,2800,13753,2467,15360,0,15360,1,15359,21,15355,89,15346,253,15325,479,15287,796,15225,1148,15133,1492,15008,1749,14856,1882,14685,1886,14506,1783,14324,1608,13996,1398,13702,1183]),Fi=null;function V_(){return Fi===null&&(Fi=new hs(G_,16,16,vs,pn),Fi.name="DFG_LUT",Fi.minFilter=rn,Fi.magFilter=rn,Fi.wrapS=Qn,Fi.wrapT=Qn,Fi.generateMipmaps=!1,Fi.needsUpdate=!0),Fi}var yc=class{constructor(e={}){let{canvas:t=kf(),context:n=null,depth:i=!0,stencil:r=!1,alpha:a=!1,antialias:o=!1,premultipliedAlpha:l=!0,preserveDrawingBuffer:c=!1,powerPreference:h="default",failIfMajorPerformanceCaveat:u=!1,reversedDepthBuffer:d=!1,outputBufferType:p=Gn}=e;this.isWebGLRenderer=!0;let x;if(n!==null){if(typeof WebGLRenderingContext<"u"&&n instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");x=n.getContextAttributes().alpha}else x=a;let v=p,m=new Set([Ol,Ul,Nl]),f=new Set([Gn,vi,Br,Hr,Dl,Ll]),y=new Uint32Array(4),T=new Int32Array(4),g=new B,_=null,w=null,A=[],b=[],C=null;this.domElement=t,this.debug={checkShaderErrors:!0,diagnostics:{keywords:!1},onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this.toneMapping=xi,this.toneMappingExposure=1,this.transmissionResolutionScale=1;let S=this,P=!1,M=null,F=null,I=null,N=null;this._outputColorSpace=sn;let H=0,D=0,V=null,k=-1,E=null,Z=new Ot,W=new Ot,Q=null,Fe=new Le(0),Ne=0,ze=t.width,se=t.height,le=1,De=null,be=null,ue=new Ot(0,0,ze,se),te=new Ot(0,0,ze,se),pe=!1,ce=new Ar,ne=!1,de=!1,we=new ft,Ve=new B,Ke=new Ot,it={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},et=!1;function ht(){return V===null?le:1}let J=n;function Ut(L,ee){return t.getContext(L,ee)}let ut,O,R,re,oe,xe,Ue,ke,G,j,he,Se,ie,ve,Ee,Te,Ie,X,Ae,Me,fe,Pe,_e;try{let L={alpha:!0,depth:i,stencil:r,antialias:o,premultipliedAlpha:l,preserveDrawingBuffer:c,powerPreference:h,failIfMajorPerformanceCaveat:u};if("setAttribute"in t&&t.setAttribute("data-engine",`three.js r${"186"}`),t.addEventListener("webglcontextlost",U,!1),t.addEventListener("webglcontextrestored",q,!1),t.addEventListener("webglcontextcreationerror",Y,!1),J===null){let ee="webgl2";if(J=Ut(ee,L),J===null)throw Ut(ee)?new Error("THREE.WebGLRenderer: Error creating WebGL context with your selected attributes."):new Error("THREE.WebGLRenderer: Error creating WebGL context.")}qe()}catch(L){throw t.removeEventListener("webglcontextlost",U,!1),t.removeEventListener("webglcontextrestored",q,!1),t.removeEventListener("webglcontextcreationerror",Y,!1),pt("WebGLRenderer: "+L.message),L}function qe(){ut=new Jb(J),ut.init(),fe=new U_(J,ut),O=new Bb(J,ut,e,fe),R=new F_(J,ut),O.reversedDepthBuffer&&d&&R.buffers.depth.setReversed(!0),F=J.createFramebuffer(),I=J.createFramebuffer(),N=J.createFramebuffer(),re=new Qb(J),oe=new __,xe=new N_(J,ut,R,oe,O,fe,re),Ue=new Yb(S),ke=new t0(J),Pe=new zb(J,ke),G=new Zb(J,ke,re,Pe),j=new tv(J,G,ke,Pe,re),X=new ev(J,O,xe),Ee=new Hb(oe),he=new v_(S,Ue,ut,O,Pe,Ee),Se=new B_(S,oe),ie=new M_,ve=new R_(ut),Ie=new Ob(S,Ue,R,j,x,l),Te=new L_(S,j,O),_e=new H_(J,re,O,R),Ae=new kb(J,ut,re),Me=new $b(J,ut,re),re.programs=he.programs,S.capabilities=O,S.extensions=ut,S.properties=oe,S.renderLists=ie,S.shadowMap=Te,S.state=R,S.info=re}v!==Gn&&(C=new iv(v,t.width,t.height,o,i,r));let Xe=new hu(S,J);this.xr=Xe,this.getContext=function(){return J},this.getContextAttributes=function(){return J.getContextAttributes()},this.forceContextLoss=function(){let L=ut.get("WEBGL_lose_context");L&&L.loseContext()},this.forceContextRestore=function(){let L=ut.get("WEBGL_lose_context");L&&L.restoreContext()},this.getPixelRatio=function(){return le},this.setPixelRatio=function(L){L!==void 0&&(le=L,this.setSize(ze,se,!1))},this.getSize=function(L){return L.set(ze,se)},this.setSize=function(L,ee,ye=!0){if(Xe.isPresenting){rt("WebGLRenderer: Can't change size while VR device is presenting.");return}ze=L,se=ee,t.width=Math.floor(L*le),t.height=Math.floor(ee*le),ye===!0&&(t.style.width=L+"px",t.style.height=ee+"px"),C!==null&&C.setSize(t.width,t.height),this.setViewport(0,0,L,ee)},this.getDrawingBufferSize=function(L){return L.set(ze*le,se*le).floor()},this.setDrawingBufferSize=function(L,ee,ye){ze=L,se=ee,le=ye,t.width=Math.floor(L*ye),t.height=Math.floor(ee*ye),this.setViewport(0,0,L,ee)},this.setEffects=function(L){if(v===Gn){pt("WebGLRenderer: setEffects() requires outputBufferType set to HalfFloatType or FloatType.");return}if(L){for(let ee=0;ee<L.length;ee++)if(L[ee].isOutputPass===!0){rt("WebGLRenderer: OutputPass is not needed in setEffects(). Tone mapping and color space conversion are applied automatically.");break}}C.setEffects(L||[])},this.getCurrentViewport=function(L){return L.copy(Z)},this.getViewport=function(L){return L.copy(ue)},this.setViewport=function(L,ee,ye,me){L.isVector4?ue.set(L.x,L.y,L.z,L.w):ue.set(L,ee,ye,me),R.viewport(Z.copy(ue).multiplyScalar(le).round())},this.getScissor=function(L){return L.copy(te)},this.setScissor=function(L,ee,ye,me){L.isVector4?te.set(L.x,L.y,L.z,L.w):te.set(L,ee,ye,me),R.scissor(W.copy(te).multiplyScalar(le).round())},this.getScissorTest=function(){return pe},this.setScissorTest=function(L){R.setScissorTest(pe=L)},this.setOpaqueSort=function(L){De=L},this.setTransparentSort=function(L){be=L},this.getClearColor=function(L){return L.copy(Ie.getClearColor())},this.setClearColor=function(){Ie.setClearColor(...arguments)},this.getClearAlpha=function(){return Ie.getClearAlpha()},this.setClearAlpha=function(){Ie.setClearAlpha(...arguments)},this.clear=function(L=!0,ee=!0,ye=!0){let me=0;if(L){let ge=!1;if(V!==null){let je=V.texture.format;ge=m.has(je)}if(ge){let je=V.texture.type,$e=f.has(je),Ye=Ie.getClearColor(),tt=Ie.getClearAlpha(),nt=Ye.r,St=Ye.g,bt=Ye.b;$e?(y[0]=nt,y[1]=St,y[2]=bt,y[3]=tt,J.clearBufferuiv(J.COLOR,0,y)):(T[0]=nt,T[1]=St,T[2]=bt,T[3]=tt,J.clearBufferiv(J.COLOR,0,T))}else me|=J.COLOR_BUFFER_BIT}ee&&(me|=J.DEPTH_BUFFER_BIT,this.state.buffers.depth.setMask(!0)),ye&&(me|=J.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),me!==0&&J.clear(me)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.setNodesHandler=function(L){L.setRenderer(this),M=L},this.dispose=function(){t.removeEventListener("webglcontextlost",U,!1),t.removeEventListener("webglcontextrestored",q,!1),t.removeEventListener("webglcontextcreationerror",Y,!1),Ie.dispose(),ie.dispose(),ve.dispose(),oe.dispose(),Ue.dispose(),j.dispose(),Pe.dispose(),_e.dispose(),he.dispose(),Xe.dispose(),Xe.removeEventListener("sessionstart",Ze),Xe.removeEventListener("sessionend",Jt),Xt.stop()};function U(L){L.preventDefault(),xa("WebGLRenderer: Context Lost."),P=!0}function q(){xa("WebGLRenderer: Context Restored."),P=!1;let L=re.autoReset,ee=Te.enabled,ye=Te.autoUpdate,me=Te.needsUpdate,ge=Te.type;qe(),re.autoReset=L,Te.enabled=ee,Te.autoUpdate=ye,Te.needsUpdate=me,Te.type=ge}function Y(L){pt("WebGLRenderer: A WebGL context could not be created. Reason: ",L.statusMessage)}function ae(L){let ee=L.target;ee.removeEventListener("dispose",ae),K(ee)}function K(L){$(L),oe.remove(L)}function $(L){let ee=oe.get(L).programs;ee!==void 0&&(ee.forEach(function(ye){he.releaseProgram(ye)}),L.isShaderMaterial&&he.releaseShaderCache(L))}this.renderBufferDirect=function(L,ee,ye,me,ge,je){ee===null&&(ee=it);let $e=ge.isMesh&&ge.matrixWorld.determinantAffine()<0,Ye=Fn(L,ee,ye,me,ge);R.setMaterial(me,$e);let tt=ye.index,nt=1;if(me.wireframe===!0){if(tt=G.getWireframeAttribute(ye),tt===void 0)return;nt=2}let St=ye.drawRange,bt=ye.attributes.position,z=St.start*nt,Ce=(St.start+St.count)*nt;je!==null&&(z=Math.max(z,je.start*nt),Ce=Math.min(Ce,(je.start+je.count)*nt)),tt!==null?(z=Math.max(z,0),Ce=Math.min(Ce,tt.count)):bt!=null&&(z=Math.max(z,0),Ce=Math.min(Ce,bt.count));let We=Ce-z;if(We<0||We===1/0)return;Pe.setup(ge,me,Ye,ye,tt);let Qe,at=Ae;if(tt!==null&&(Qe=ke.get(tt),at=Me,at.setIndex(Qe)),ge.isMesh)me.wireframe===!0?(R.setLineWidth(me.wireframeLinewidth*ht()),at.setMode(J.LINES)):at.setMode(J.TRIANGLES);else if(ge.isLine){let zt=me.linewidth;zt===void 0&&(zt=1),R.setLineWidth(zt*ht()),ge.isLineSegments?at.setMode(J.LINES):ge.isLineLoop?at.setMode(J.LINE_LOOP):at.setMode(J.LINE_STRIP)}else ge.isPoints?at.setMode(J.POINTS):ge.isSprite&&at.setMode(J.TRIANGLES);if(ge.isBatchedMesh)if(ut.get("WEBGL_multi_draw"))at.renderMultiDraw(ge._multiDrawStarts,ge._multiDrawCounts,ge._multiDrawCount);else{let zt=ge._multiDrawStarts,Oe=ge._multiDrawCounts,ct=ge._multiDrawCount,lt=tt?ke.get(tt).bytesPerElement:1,dt=oe.get(me).currentProgram.getUniforms();for(let Pt=0;Pt<ct;Pt++)dt.setValue(J,"_gl_DrawID",Pt),at.render(zt[Pt]/lt,Oe[Pt])}else if(ge.isInstancedMesh)at.renderInstances(z,We,ge.count);else if(ye.isInstancedBufferGeometry){let zt=ye._maxInstanceCount!==void 0?ye._maxInstanceCount:1/0,Oe=Math.min(ye.instanceCount,zt);at.renderInstances(z,We,Oe)}else at.render(z,We)};function Re(L,ee,ye,me){M!==null&&L.isNodeMaterial&&M.setObject(me,L),ne===!0&&Ee.setState(L,ye,!1),L.transparent===!0&&L.side===Kt&&L.forceSinglePass===!1?(L.side=mn,L.needsUpdate=!0,wn(L,ee,me),L.side=ti,L.needsUpdate=!0,wn(L,ee,me),L.side=Kt):wn(L,ee,me)}this.compile=function(L,ee,ye=null){ye===null&&(ye=L),M!==null&&M.renderStart(L,ee,ye),w=ve.get(ye),w.init(ee),b.push(w),ye.traverseVisible(function(ge){ge.isLight&&ge.layers.test(ee.layers)&&(w.pushLight(ge),ge.castShadow&&w.pushShadow(ge))}),L!==ye&&L.traverseVisible(function(ge){ge.isLight&&ge.layers.test(ee.layers)&&(w.pushLight(ge),ge.castShadow&&w.pushShadow(ge))}),w.setupLights(),M!==null&&M.updateLights(w.state.lightsArray),de=this.localClippingEnabled,ne=Ee.init(this.clippingPlanes,de),ne===!0&&Ee.setGlobalState(this.clippingPlanes,ee),M!==null&&Te.render(w.state.shadowsArray,ye,ee);let me=new Set;return L.traverse(function(ge){if(!(ge.isMesh||ge.isPoints||ge.isLine||ge.isSprite))return;let je=ge.material;if(je)if(Array.isArray(je))for(let $e=0;$e<je.length;$e++){let Ye=je[$e];Re(Ye,ye,ee,ge),me.add(Ye)}else Re(je,ye,ee,ge),me.add(je)}),w=b.pop(),M!==null&&M.renderEnd(),me},this.compileAsync=function(L,ee,ye=null){let me=this.compile(L,ee,ye);return new Promise(ge=>{function je(){if(me.forEach(function($e){let tt=oe.get($e).currentProgram;(tt===void 0||tt.isReady())&&me.delete($e)}),me.size===0){ge(L);return}setTimeout(je,10)}ut.get("KHR_parallel_shader_compile")!==null?je():setTimeout(je,10)})};let He=null;function ot(L){He&&He(L)}function Ze(){Xt.stop()}function Jt(){Xt.start()}let Xt=new fp;Xt.setAnimationLoop(ot),typeof self<"u"&&Xt.setContext(self),this.setAnimationLoop=function(L){He=L,Xe.setAnimationLoop(L),L===null?Xt.stop():Xt.start()},Xe.addEventListener("sessionstart",Ze),Xe.addEventListener("sessionend",Jt),this.render=function(L,ee){if(ee!==void 0&&ee.isCamera!==!0){pt("WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(P===!0)return;M!==null&&M.renderStart(L,ee);let ye=Xe.enabled===!0&&Xe.isPresenting===!0,me=C!==null&&(V===null||ye)&&C.begin(S,V);if(L.matrixWorldAutoUpdate===!0&&L.updateMatrixWorld(),ee.parent===null&&ee.matrixWorldAutoUpdate===!0&&ee.updateMatrixWorld(),Xe.enabled===!0&&Xe.isPresenting===!0&&(C===null||C.isCompositing()===!1)&&(Xe.cameraAutoUpdate===!0&&Xe.updateCamera(ee),ee=Xe.getCamera()),L.isScene===!0&&L.onBeforeRender(S,L,ee,V),w=ve.get(L,b.length),w.init(ee),w.state.textureUnits=xe.getTextureUnits(),b.push(w),we.multiplyMatrices(ee.projectionMatrix,ee.matrixWorldInverse),ce.setFromProjectionMatrix(we,ui,ee.reversedDepth),de=this.localClippingEnabled,ne=Ee.init(this.clippingPlanes,de),_=ie.get(L,A.length),_.init(),A.push(_),Xe.enabled===!0&&Xe.isPresenting===!0){let $e=S.xr.getDepthSensingMesh();$e!==null&&Mt($e,ee,-1/0,S.sortObjects)}Mt(L,ee,0,S.sortObjects),_.finish(),M!==null&&M.updateLights(w.state.lightsArray),S.sortObjects===!0&&_.sort(De,be),et=Xe.enabled===!1||Xe.isPresenting===!1||Xe.hasDepthSensing()===!1,et&&Ie.addToRenderList(_,L),this.info.render.frame++,this.info.autoReset===!0&&this.info.reset(),ne===!0&&Ee.beginShadows();let ge=w.state.shadowsArray;if(Te.render(ge,L,ee),ne===!0&&Ee.endShadows(),(me&&C.hasRenderPass())===!1){let $e=_.opaque,Ye=_.transmissive;if(w.setupLights(),ee.isArrayCamera){let tt=ee.cameras;if(Ye.length>0)for(let nt=0,St=tt.length;nt<St;nt++){let bt=tt[nt];kt($e,Ye,L,bt)}et&&Ie.render(L);for(let nt=0,St=tt.length;nt<St;nt++){let bt=tt[nt];Mn(_,L,bt,bt.viewport)}}else Ye.length>0&&kt($e,Ye,L,ee),et&&Ie.render(L),Mn(_,L,ee)}V!==null&&D===0&&(xe.updateMultisampleRenderTarget(V),xe.updateRenderTargetMipmap(V)),me&&C.end(S),L.isScene===!0&&L.onAfterRender(S,L,ee),Pe.resetDefaultState(),k=-1,E=null,b.pop(),b.length>0?(w=b[b.length-1],xe.setTextureUnits(w.state.textureUnits),ne===!0&&Ee.setGlobalState(S.clippingPlanes,w.state.camera)):w=null,A.pop(),A.length>0?_=A[A.length-1]:_=null,M!==null&&M.renderEnd()};function Mt(L,ee,ye,me){if(L.visible===!1)return;if(L.layers.test(ee.layers)){if(L.isGroup)ye=L.renderOrder;else if(L.isLOD)L.autoUpdate===!0&&L.update(ee);else if(L.isLightProbeGrid)w.pushLightProbeGrid(L);else if(L.isLight)w.pushLight(L),L.castShadow&&w.pushShadow(L);else if(L.isSprite){if(!L.frustumCulled||L.intersectsFrustum(ce)){me&&Ke.setFromMatrixPosition(L.matrixWorld).applyMatrix4(we);let $e=j.update(L),Ye=L.material;Ye.visible&&_.push(L,$e,Ye,ye,Ke.z,null,ee)}}else if((L.isMesh||L.isLine||L.isPoints)&&(!L.frustumCulled||L.intersectsFrustum(ce))){let $e=j.update(L),Ye=L.material;if(me&&(L.boundingSphere!==void 0?(L.boundingSphere===null&&L.computeBoundingSphere(),Ke.copy(L.boundingSphere.center)):($e.boundingSphere===null&&$e.computeBoundingSphere(),Ke.copy($e.boundingSphere.center)),Ke.applyMatrix4(L.matrixWorld).applyMatrix4(we)),Array.isArray(Ye)){let tt=$e.groups;for(let nt=0,St=tt.length;nt<St;nt++){let bt=tt[nt],z=Ye[bt.materialIndex];z&&z.visible&&_.push(L,$e,z,ye,Ke.z,bt,ee)}}else Ye.visible&&_.push(L,$e,Ye,ye,Ke.z,null,ee)}}let je=L.children;for(let $e=0,Ye=je.length;$e<Ye;$e++)Mt(je[$e],ee,ye,me)}function Mn(L,ee,ye,me){let{opaque:ge,transmissive:je,transparent:$e}=L;w.setupLightsView(ye),ne===!0&&Ee.setGlobalState(S.clippingPlanes,ye),me&&R.viewport(Z.copy(me)),ge.length>0&&nn(ge,ee,ye),je.length>0&&nn(je,ee,ye),$e.length>0&&nn($e,ee,ye),R.buffers.depth.setTest(!0),R.buffers.depth.setMask(!0),R.buffers.color.setMask(!0),R.setPolygonOffset(!1)}function kt(L,ee,ye,me){if((ye.isScene===!0?ye.overrideMaterial:null)!==null)return;if(w.state.transmissionRenderTarget[me.id]===void 0){let z=ut.has("EXT_color_buffer_half_float")||ut.has("EXT_color_buffer_float");w.state.transmissionRenderTarget[me.id]=new an(1,1,{generateMipmaps:!0,type:z?pn:Gn,minFilter:bi,samples:Math.max(4,O.samples),stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,storeMultisampledDepthBuffer:!1,storeMultisampledStencilBuffer:!1,colorSpace:Tt.workingColorSpace})}let je=w.state.transmissionRenderTarget[me.id],$e=me.viewport||Z;je.setSize($e.z*S.transmissionResolutionScale,$e.w*S.transmissionResolutionScale);let Ye=S.getRenderTarget(),tt=S.getActiveCubeFace(),nt=S.getActiveMipmapLevel();S.setRenderTarget(je),S.getClearColor(Fe),Ne=S.getClearAlpha(),Ne<1&&S.setClearColor(16777215,.5),S.clear(),et&&Ie.render(ye);let St=S.toneMapping;S.toneMapping=xi;let bt=me.viewport;if(me.viewport!==void 0&&(me.viewport=void 0),w.setupLightsView(me),ne===!0&&Ee.setGlobalState(S.clippingPlanes,me),nn(L,ye,me),xe.updateMultisampleRenderTarget(je),xe.updateRenderTargetMipmap(je),ut.has("WEBGL_multisampled_render_to_texture")===!1){let z=!1;for(let Ce=0,We=ee.length;Ce<We;Ce++){let Qe=ee[Ce],{object:at,geometry:zt,material:Oe,group:ct}=Qe;if(Oe.side===Kt&&at.layers.test(me.layers)){let lt=Oe.side;Oe.side=mn,Oe.needsUpdate=!0,Zt(at,ye,me,zt,Oe,ct),Oe.side=lt,Oe.needsUpdate=!0,z=!0}}z===!0&&(xe.updateMultisampleRenderTarget(je),xe.updateRenderTargetMipmap(je))}S.setRenderTarget(Ye,tt,nt),S.setClearColor(Fe,Ne),bt!==void 0&&(me.viewport=bt),S.toneMapping=St}function nn(L,ee,ye){let me=ee.isScene===!0?ee.overrideMaterial:null;for(let ge=0,je=L.length;ge<je;ge++){let $e=L[ge],{object:Ye,geometry:tt,group:nt}=$e,St=$e.material;St.allowOverride===!0&&me!==null&&(St=me),Ye.layers.test(ye.layers)&&Zt(Ye,ee,ye,tt,St,nt)}}function Zt(L,ee,ye,me,ge,je){M!==null&&ge.isNodeMaterial&&M.setObject(L,ge),L.onBeforeRender(S,ee,ye,me,ge,je),L.modelViewMatrix.multiplyMatrices(ye.matrixWorldInverse,L.matrixWorld),L.normalMatrix.getNormalMatrix(L.modelViewMatrix),ge.onBeforeRender(S,ee,ye,me,L,je),ge.transparent===!0&&ge.side===Kt&&ge.forceSinglePass===!1?(ge.side=mn,ge.needsUpdate=!0,S.renderBufferDirect(ye,ee,me,ge,L,je),ge.side=ti,ge.needsUpdate=!0,S.renderBufferDirect(ye,ee,me,ge,L,je),ge.side=Kt):S.renderBufferDirect(ye,ee,me,ge,L,je),L.onAfterRender(S,ee,ye,me,ge,je)}function wn(L,ee,ye){ee.isScene!==!0&&(ee=it);let me=oe.get(L),ge=w.state.lights,je=w.state.shadowsArray,$e=ge.state.version,Ye=he.getParameters(L,ge.state,je,ee,ye,w.state.lightProbeGridArray),tt=he.getProgramCacheKey(Ye),nt=me.programs;me.environment=L.isMeshStandardMaterial||L.isMeshLambertMaterial||L.isMeshPhongMaterial?ee.environment:null,me.fog=ee.fog;let St=L.isMeshStandardMaterial||L.isMeshLambertMaterial&&!L.envMap||L.isMeshPhongMaterial&&!L.envMap;me.envMap=Ue.get(L.envMap||me.environment,St),me.envMapRotation=me.environment!==null&&L.envMap===null?ee.environmentRotation:L.envMapRotation,nt===void 0&&(L.addEventListener("dispose",ae),nt=new Map,me.programs=nt);let bt=nt.get(tt);if(bt!==void 0){if(me.currentProgram===bt&&me.lightsStateVersion===$e)return En(L,Ye),bt}else Ye.uniforms=he.getUniforms(L),M!==null&&L.isNodeMaterial&&M.build(L,ye,Ye),L.onBeforeCompile(Ye,S),bt=he.acquireProgram(Ye,tt),nt.set(tt,bt),me.uniforms=Ye.uniforms;let z=me.uniforms;return(!L.isShaderMaterial&&!L.isRawShaderMaterial||L.clipping===!0)&&(z.clippingPlanes=Ee.uniform),En(L,Ye),me.needsLights=Es(L),me.lightsStateVersion=$e,me.needsLights&&(z.ambientLightColor.value=ge.state.ambient,z.lightProbe.value=ge.state.probe,z.sunLights.value=ge.state.sun,z.sunLightShadows.value=ge.state.sunShadow,z.directionalLights.value=ge.state.directional,z.directionalLightShadows.value=ge.state.directionalShadow,z.spotLights.value=ge.state.spot,z.spotLightShadows.value=ge.state.spotShadow,z.rectAreaLights.value=ge.state.rectArea,z.ltc_1.value=ge.state.rectAreaLTC1,z.ltc_2.value=ge.state.rectAreaLTC2,z.pointLights.value=ge.state.point,z.pointLightShadows.value=ge.state.pointShadow,z.hemisphereLights.value=ge.state.hemi,z.sunShadowMatrix.value=ge.state.sunShadowMatrix,z.sunShadowCascade.value=ge.state.sunShadowCascade,z.directionalShadowMatrix.value=ge.state.directionalShadowMatrix,z.spotLightMatrix.value=ge.state.spotLightMatrix,z.spotLightMap.value=ge.state.spotLightMap,z.pointShadowMatrix.value=ge.state.pointShadowMatrix),me.lightProbeGrid=w.state.lightProbeGridArray.length>0,me.currentProgram=bt,me.uniformsList=null,bt}function ai(L){if(L.uniformsList===null){let ee=L.currentProgram.getUniforms();L.uniformsList=qr.seqWithValue(ee.seq,L.uniforms)}return L.uniformsList}function En(L,ee){let ye=oe.get(L);ye.outputColorSpace=ee.outputColorSpace,ye.batching=ee.batching,ye.batchingColor=ee.batchingColor,ye.instancing=ee.instancing,ye.instancingColor=ee.instancingColor,ye.instancingMorph=ee.instancingMorph,ye.skinning=ee.skinning,ye.morphTargets=ee.morphTargets,ye.morphNormals=ee.morphNormals,ye.morphColors=ee.morphColors,ye.morphTargetsCount=ee.morphTargetsCount,ye.numClippingPlanes=ee.numClippingPlanes,ye.numIntersection=ee.numClipIntersection,ye.vertexAlphas=ee.vertexAlphas,ye.vertexTangents=ee.vertexTangents,ye.toneMapping=ee.toneMapping}function Tn(L,ee){if(L.length===0)return null;if(L.length===1)return L[0].texture!==null?L[0]:null;g.setFromMatrixPosition(ee.matrixWorld);for(let ye=0,me=L.length;ye<me;ye++){let ge=L[ye];if(ge.texture!==null&&ge.boundingBox.containsPoint(g))return ge}return null}function Fn(L,ee,ye,me,ge){ee.isScene!==!0&&(ee=it),xe.resetTextureUnits();let je=ee.fog,$e=me.isMeshStandardMaterial||me.isMeshLambertMaterial||me.isMeshPhongMaterial?ee.environment:null,Ye=V===null?S.outputColorSpace:V.isXRRenderTarget===!0?V.texture.colorSpace:Tt.workingColorSpace,tt=me.isMeshStandardMaterial||me.isMeshLambertMaterial&&!me.envMap||me.isMeshPhongMaterial&&!me.envMap,nt=Ue.get(me.envMap||$e,tt),St=me.vertexColors===!0&&!!ye.attributes.color&&ye.attributes.color.itemSize===4,bt=!!ye.attributes.tangent&&(!!me.normalMap||me.anisotropy>0),z=!!ye.morphAttributes.position,Ce=!!ye.morphAttributes.normal,We=!!ye.morphAttributes.color,Qe=xi;me.toneMapped&&(V===null||V.isXRRenderTarget===!0)&&(Qe=S.toneMapping);let at=ye.morphAttributes.position||ye.morphAttributes.normal||ye.morphAttributes.color,zt=at!==void 0?at.length:0,Oe=oe.get(me),ct=w.state.lights;if(ne===!0&&(de===!0||L!==E)){let Lt=L===E&&me.id===k;Ee.setState(me,L,Lt)}let lt=!1;me.version===Oe.__version?(Oe.needsLights&&Oe.lightsStateVersion!==ct.state.version||Oe.outputColorSpace!==Ye||ge.isBatchedMesh&&Oe.batching===!1||!ge.isBatchedMesh&&Oe.batching===!0||ge.isBatchedMesh&&Oe.batchingColor===!0&&ge._colorsTexture===null||ge.isBatchedMesh&&Oe.batchingColor===!1&&ge._colorsTexture!==null||ge.isInstancedMesh&&Oe.instancing===!1||!ge.isInstancedMesh&&Oe.instancing===!0||ge.isSkinnedMesh&&Oe.skinning===!1||!ge.isSkinnedMesh&&Oe.skinning===!0||ge.isInstancedMesh&&Oe.instancingColor===!0&&ge.instanceColor===null||ge.isInstancedMesh&&Oe.instancingColor===!1&&ge.instanceColor!==null||ge.isInstancedMesh&&Oe.instancingMorph===!0&&ge.morphTexture===null||ge.isInstancedMesh&&Oe.instancingMorph===!1&&ge.morphTexture!==null||Oe.envMap!==nt||me.fog===!0&&Oe.fog!==je||Oe.numClippingPlanes!==void 0&&(Oe.numClippingPlanes!==Ee.numPlanes||Oe.numIntersection!==Ee.numIntersection)||Oe.vertexAlphas!==St||Oe.vertexTangents!==bt||Oe.morphTargets!==z||Oe.morphNormals!==Ce||Oe.morphColors!==We||Oe.toneMapping!==Qe||Oe.morphTargetsCount!==zt||!!Oe.lightProbeGrid!=w.state.lightProbeGridArray.length>0)&&(lt=!0):(lt=!0,Oe.__version=me.version);let dt=Oe.currentProgram;lt===!0&&(dt=wn(me,ee,ge),M&&me.isNodeMaterial&&M.onUpdateProgram(me,dt,Oe));let Pt=!1,Et=!1,It=!1,Ge=dt.getUniforms(),wt=Oe.uniforms;if(R.useProgram(dt.program)&&(Pt=!0,Et=!0,It=!0),me.id!==k&&(k=me.id,Et=!0),Oe.needsLights){let Lt=Tn(w.state.lightProbeGridArray,ge);Oe.lightProbeGrid!==Lt&&(Oe.lightProbeGrid=Lt,Et=!0)}if(Pt||E!==L){R.buffers.depth.getReversed()&&L.reversedDepth!==!0&&(L._reversedDepth=!0,L.updateProjectionMatrix()),Ge.setValue(J,"projectionMatrix",L.projectionMatrix),Ge.setValue(J,"viewMatrix",L.matrixWorldInverse);let Si=Ge.map.cameraPosition;Si!==void 0&&Si.setValue(J,Ve.setFromMatrixPosition(L.matrixWorld)),O.logarithmicDepthBuffer&&Ge.setValue(J,"logDepthBufFC",2/(Math.log(L.far+1)/Math.LN2)),(me.isMeshPhongMaterial||me.isMeshToonMaterial||me.isMeshLambertMaterial||me.isMeshBasicMaterial||me.isMeshStandardMaterial||me.isShaderMaterial)&&Ge.setValue(J,"isOrthographic",L.isOrthographicCamera===!0),E!==L&&(E=L,Et=!0,It=!0)}if(Oe.needsLights&&(ct.state.sunShadowMap.length>0&&Ge.setValue(J,"sunShadowMap",ct.state.sunShadowMap,xe),ct.state.directionalShadowMap.length>0&&Ge.setValue(J,"directionalShadowMap",ct.state.directionalShadowMap,xe),ct.state.spotShadowMap.length>0&&Ge.setValue(J,"spotShadowMap",ct.state.spotShadowMap,xe),ct.state.pointShadowMap.length>0&&Ge.setValue(J,"pointShadowMap",ct.state.pointShadowMap,xe)),ge.isSkinnedMesh){Ge.setOptional(J,ge,"bindMatrix"),Ge.setOptional(J,ge,"bindMatrixInverse");let Lt=ge.skeleton;Lt&&(Lt.boneTexture===null&&Lt.computeBoneTexture(),Ge.setValue(J,"boneTexture",Lt.boneTexture,xe))}ge.isBatchedMesh&&(Ge.setOptional(J,ge,"batchingTexture"),Ge.setValue(J,"batchingTexture",ge._matricesTexture,xe),Ge.setOptional(J,ge,"batchingIdTexture"),Ge.setValue(J,"batchingIdTexture",ge._indirectTexture,xe),Ge.setOptional(J,ge,"batchingColorTexture"),ge._colorsTexture!==null&&Ge.setValue(J,"batchingColorTexture",ge._colorsTexture,xe));let en=ye.morphAttributes;if((en.position!==void 0||en.normal!==void 0||en.color!==void 0)&&X.update(ge,ye,dt),(Et||Oe.receiveShadow!==ge.receiveShadow)&&(Oe.receiveShadow=ge.receiveShadow,Ge.setValue(J,"receiveShadow",ge.receiveShadow)),(me.isMeshStandardMaterial||me.isMeshLambertMaterial||me.isMeshPhongMaterial)&&me.envMap===null&&ee.environment!==null&&(wt.envMapIntensity.value=ee.environmentIntensity),wt.dfgLUT!==void 0&&(wt.dfgLUT.value=V_()),Et){if(Ge.setValue(J,"toneMappingExposure",S.toneMappingExposure),Oe.needsLights&&na(wt,It),je&&me.fog===!0&&Se.refreshFogUniforms(wt,je),Se.refreshMaterialUniforms(wt,me,le,se,w.state.transmissionRenderTarget[L.id]),Oe.needsLights&&Oe.lightProbeGrid){let Lt=Oe.lightProbeGrid;wt.probesSH.value=Lt.texture,wt.probesMin.value.copy(Lt.boundingBox.min),wt.probesMax.value.copy(Lt.boundingBox.max),wt.probesResolution.value.copy(Lt.resolution)}qr.upload(J,ai(Oe),wt,xe)}if(me.isShaderMaterial&&me.uniformsNeedUpdate===!0&&(qr.upload(J,ai(Oe),wt,xe),me.uniformsNeedUpdate=!1),me.isSpriteMaterial&&Ge.setValue(J,"center",ge.center),Ge.setValue(J,"modelViewMatrix",ge.modelViewMatrix),Ge.setValue(J,"normalMatrix",ge.normalMatrix),Ge.setValue(J,"modelMatrix",ge.matrixWorld),me.uniformsGroups!==void 0){let Lt=me.uniformsGroups;for(let Si=0,er=Lt.length;Si<er;Si++){let dd=Lt[Si];_e.update(dd,dt),_e.bind(dd,dt)}}return dt}function na(L,ee){L.ambientLightColor.needsUpdate=ee,L.lightProbe.needsUpdate=ee,L.sunLights.needsUpdate=ee,L.sunLightShadows.needsUpdate=ee,L.directionalLights.needsUpdate=ee,L.directionalLightShadows.needsUpdate=ee,L.pointLights.needsUpdate=ee,L.pointLightShadows.needsUpdate=ee,L.spotLights.needsUpdate=ee,L.spotLightShadows.needsUpdate=ee,L.rectAreaLights.needsUpdate=ee,L.hemisphereLights.needsUpdate=ee}function Es(L){return L.isMeshLambertMaterial||L.isMeshToonMaterial||L.isMeshPhongMaterial||L.isMeshStandardMaterial||L.isShadowMaterial||L.isShaderMaterial&&L.lights===!0}this.getActiveCubeFace=function(){return H},this.getActiveMipmapLevel=function(){return D},this.getRenderTarget=function(){return V},this.setRenderTargetTextures=function(L,ee,ye){let me=oe.get(L);me.__autoAllocateDepthBuffer=L.resolveDepthBuffer===!1,me.__autoAllocateDepthBuffer===!1&&(me.__useRenderToTexture=!1),oe.get(L.texture).__webglTexture=ee,oe.get(L.depthTexture).__webglTexture=me.__autoAllocateDepthBuffer?void 0:ye,me.__hasExternalTextures=!0},this.setRenderTargetFramebuffer=function(L,ee){let ye=oe.get(L);ye.__webglFramebuffer=ee,ye.__useDefaultFramebuffer=ee===void 0},this.setRenderTarget=function(L,ee=0,ye=0){V=L,H=ee,D=ye;let me=null,ge=!1,je=!1;if(L){let Ye=oe.get(L);if(Ye.__useDefaultFramebuffer!==void 0){R.bindFramebuffer(J.FRAMEBUFFER,Ye.__webglFramebuffer),Z.copy(L.viewport),W.copy(L.scissor),Q=L.scissorTest,R.viewport(Z),R.scissor(W),R.setScissorTest(Q),k=-1;return}else if(Ye.__webglFramebuffer===void 0)xe.setupRenderTarget(L);else if(Ye.__hasExternalTextures)xe.rebindTextures(L,oe.get(L.texture).__webglTexture,oe.get(L.depthTexture).__webglTexture);else if(L.depthBuffer){let St=L.depthTexture;if(Ye.__boundDepthTexture!==St){if(St!==null&&oe.has(St)&&(L.width!==St.image.width||L.height!==St.image.height))throw new Error("THREE.WebGLRenderer: Attached DepthTexture is initialized to the incorrect size.");xe.setupDepthRenderbuffer(L)}}let tt=L.texture;(tt.isData3DTexture||tt.isDataArrayTexture||tt.isCompressedArrayTexture)&&(je=!0);let nt=oe.get(L).__webglFramebuffer;L.isWebGLCubeRenderTarget?(Array.isArray(nt[ee])?me=nt[ee][ye]:me=nt[ee],ge=!0):L.samples>0&&xe.useMultisampledRTT(L)===!1?me=oe.get(L).__webglMultisampledFramebuffer:Array.isArray(nt)?me=nt[ye]:me=nt,Z.copy(L.viewport),W.copy(L.scissor),Q=L.scissorTest}else Z.copy(ue).multiplyScalar(le).floor(),W.copy(te).multiplyScalar(le).floor(),Q=pe;if(ye!==0&&(me=F),R.bindFramebuffer(J.FRAMEBUFFER,me)&&R.drawBuffers(L,me),R.viewport(Z),R.scissor(W),R.setScissorTest(Q),ge){let Ye=oe.get(L.texture);J.framebufferTexture2D(J.FRAMEBUFFER,J.COLOR_ATTACHMENT0,J.TEXTURE_CUBE_MAP_POSITIVE_X+ee,Ye.__webglTexture,ye)}else if(je){let Ye=ee;for(let tt=0;tt<L.textures.length;tt++){let nt=oe.get(L.textures[tt]);J.framebufferTextureLayer(J.FRAMEBUFFER,J.COLOR_ATTACHMENT0+tt,nt.__webglTexture,ye,Ye)}}else if(L!==null&&ye!==0){let Ye=oe.get(L.texture);J.framebufferTexture2D(J.FRAMEBUFFER,J.COLOR_ATTACHMENT0,J.TEXTURE_2D,Ye.__webglTexture,ye)}k=-1};function _o(L){let ee=oe.get(L);return(ee.__readFormat!==L.format||ee.__readType!==L.type)&&(ee.__readFormat=L.format,ee.__readType=L.type,ee.__formatReadable=O.textureFormatReadable(L.format),ee.__typeReadable=O.textureTypeReadable(L.type)),ee}this.readRenderTargetPixels=function(L,ee,ye,me,ge,je,$e,Ye=0){if(!(L&&L.isWebGLRenderTarget)){pt("WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let tt=oe.get(L).__webglFramebuffer;if(L.isWebGLCubeRenderTarget&&$e!==void 0&&(tt=tt[$e]),tt){R.bindFramebuffer(J.FRAMEBUFFER,tt);try{let nt=L.textures[Ye],St=nt.format,bt=nt.type;L.textures.length>1&&J.readBuffer(J.COLOR_ATTACHMENT0+Ye);let z=_o(nt);if(z.__formatReadable===!1){pt("WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if(z.__typeReadable===!1){pt("WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}ee>=0&&ee<=L.width-me&&ye>=0&&ye<=L.height-ge&&J.readPixels(ee,ye,me,ge,fe.convert(St),fe.convert(bt),je)}finally{let nt=V!==null?oe.get(V).__webglFramebuffer:null;R.bindFramebuffer(J.FRAMEBUFFER,nt)}}},this.readRenderTargetPixelsAsync=async function(L,ee,ye,me,ge,je,$e,Ye=0){if(!(L&&L.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let tt=oe.get(L).__webglFramebuffer;if(L.isWebGLCubeRenderTarget&&$e!==void 0&&(tt=tt[$e]),tt)if(ee>=0&&ee<=L.width-me&&ye>=0&&ye<=L.height-ge){R.bindFramebuffer(J.FRAMEBUFFER,tt);let nt=L.textures[Ye],St=nt.format,bt=nt.type;L.textures.length>1&&J.readBuffer(J.COLOR_ATTACHMENT0+Ye);let z=_o(nt);if(z.__formatReadable===!1)throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if(z.__typeReadable===!1)throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");let Ce=J.createBuffer();J.bindBuffer(J.PIXEL_PACK_BUFFER,Ce),J.bufferData(J.PIXEL_PACK_BUFFER,je.byteLength,J.STREAM_READ),J.readPixels(ee,ye,me,ge,fe.convert(St),fe.convert(bt),0),J.bindBuffer(J.PIXEL_PACK_BUFFER,null);let We=V!==null?oe.get(V).__webglFramebuffer:null;R.bindFramebuffer(J.FRAMEBUFFER,We);let Qe=J.fenceSync(J.SYNC_GPU_COMMANDS_COMPLETE,0);return J.flush(),await Hf(J,Qe,4),J.bindBuffer(J.PIXEL_PACK_BUFFER,Ce),J.getBufferSubData(J.PIXEL_PACK_BUFFER,0,je),J.bindBuffer(J.PIXEL_PACK_BUFFER,null),J.deleteBuffer(Ce),J.deleteSync(Qe),je}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")},this.copyFramebufferToTexture=function(L,ee=null,ye=0){let me=Math.pow(2,-ye),ge=Math.floor(L.image.width*me),je=Math.floor(L.image.height*me),$e=ee!==null?ee.x:0,Ye=ee!==null?ee.y:0;xe.setTexture2D(L,0),J.copyTexSubImage2D(J.TEXTURE_2D,ye,0,0,$e,Ye,ge,je),R.unbindTexture()},this.copyTextureToTexture=function(L,ee,ye=null,me=null,ge=0,je=0){let $e,Ye,tt,nt,St,bt,z,Ce,We,Qe=L.isCompressedTexture?L.mipmaps[je]:L.image;if(ye!==null)$e=ye.max.x-ye.min.x,Ye=ye.max.y-ye.min.y,tt=ye.isBox3?ye.max.z-ye.min.z:1,nt=ye.min.x,St=ye.min.y,bt=ye.isBox3?ye.min.z:0;else{let wt=Math.pow(2,-ge);$e=Math.floor(Qe.width*wt),Ye=Math.floor(Qe.height*wt),L.isDataArrayTexture?tt=Qe.depth:L.isData3DTexture?tt=Math.floor(Qe.depth*wt):tt=1,nt=0,St=0,bt=0}me!==null?(z=me.x,Ce=me.y,We=me.z):(z=0,Ce=0,We=0);let at=fe.convert(ee.format),zt=fe.convert(ee.type),Oe;ee.isData3DTexture?(xe.setTexture3D(ee,0),Oe=J.TEXTURE_3D):ee.isDataArrayTexture||ee.isCompressedArrayTexture?(xe.setTexture2DArray(ee,0),Oe=J.TEXTURE_2D_ARRAY):(xe.setTexture2D(ee,0),Oe=J.TEXTURE_2D),R.activeTexture(J.TEXTURE0),R.pixelStorei(J.UNPACK_FLIP_Y_WEBGL,ee.flipY),R.pixelStorei(J.UNPACK_PREMULTIPLY_ALPHA_WEBGL,ee.premultiplyAlpha),R.pixelStorei(J.UNPACK_ALIGNMENT,ee.unpackAlignment);let ct=R.getParameter(J.UNPACK_ROW_LENGTH),lt=R.getParameter(J.UNPACK_IMAGE_HEIGHT),dt=R.getParameter(J.UNPACK_SKIP_PIXELS),Pt=R.getParameter(J.UNPACK_SKIP_ROWS),Et=R.getParameter(J.UNPACK_SKIP_IMAGES);R.pixelStorei(J.UNPACK_ROW_LENGTH,Qe.width),R.pixelStorei(J.UNPACK_IMAGE_HEIGHT,Qe.height),R.pixelStorei(J.UNPACK_SKIP_PIXELS,nt),R.pixelStorei(J.UNPACK_SKIP_ROWS,St),R.pixelStorei(J.UNPACK_SKIP_IMAGES,bt);let It=L.isDataArrayTexture||L.isData3DTexture,Ge=ee.isDataArrayTexture||ee.isData3DTexture;if(L.isDepthTexture){let wt=oe.get(L),en=oe.get(ee),Lt=oe.get(wt.__renderTarget),Si=oe.get(en.__renderTarget);R.bindFramebuffer(J.READ_FRAMEBUFFER,Lt.__webglFramebuffer),R.bindFramebuffer(J.DRAW_FRAMEBUFFER,Si.__webglFramebuffer);for(let er=0;er<tt;er++)It&&(J.framebufferTextureLayer(J.READ_FRAMEBUFFER,J.COLOR_ATTACHMENT0,oe.get(L).__webglTexture,ge,bt+er),J.framebufferTextureLayer(J.DRAW_FRAMEBUFFER,J.COLOR_ATTACHMENT0,oe.get(ee).__webglTexture,je,We+er)),J.blitFramebuffer(nt,St,$e,Ye,z,Ce,$e,Ye,J.DEPTH_BUFFER_BIT,J.NEAREST);R.bindFramebuffer(J.READ_FRAMEBUFFER,null),R.bindFramebuffer(J.DRAW_FRAMEBUFFER,null)}else if(ge!==0||L.isRenderTargetTexture||oe.has(L)){let wt=oe.get(L),en=oe.get(ee);R.bindFramebuffer(J.READ_FRAMEBUFFER,I),R.bindFramebuffer(J.DRAW_FRAMEBUFFER,N);for(let Lt=0;Lt<tt;Lt++)It?J.framebufferTextureLayer(J.READ_FRAMEBUFFER,J.COLOR_ATTACHMENT0,wt.__webglTexture,ge,bt+Lt):J.framebufferTexture2D(J.READ_FRAMEBUFFER,J.COLOR_ATTACHMENT0,J.TEXTURE_2D,wt.__webglTexture,ge),Ge?J.framebufferTextureLayer(J.DRAW_FRAMEBUFFER,J.COLOR_ATTACHMENT0,en.__webglTexture,je,We+Lt):J.framebufferTexture2D(J.DRAW_FRAMEBUFFER,J.COLOR_ATTACHMENT0,J.TEXTURE_2D,en.__webglTexture,je),ge!==0?J.blitFramebuffer(nt,St,$e,Ye,z,Ce,$e,Ye,J.COLOR_BUFFER_BIT,J.NEAREST):Ge?J.copyTexSubImage3D(Oe,je,z,Ce,We+Lt,nt,St,$e,Ye):J.copyTexSubImage2D(Oe,je,z,Ce,nt,St,$e,Ye);R.bindFramebuffer(J.READ_FRAMEBUFFER,null),R.bindFramebuffer(J.DRAW_FRAMEBUFFER,null)}else Ge?L.isDataTexture||L.isData3DTexture?J.texSubImage3D(Oe,je,z,Ce,We,$e,Ye,tt,at,zt,Qe.data):ee.isCompressedArrayTexture?J.compressedTexSubImage3D(Oe,je,z,Ce,We,$e,Ye,tt,at,Qe.data):J.texSubImage3D(Oe,je,z,Ce,We,$e,Ye,tt,at,zt,Qe):L.isDataTexture?J.texSubImage2D(J.TEXTURE_2D,je,z,Ce,$e,Ye,at,zt,Qe.data):L.isCompressedTexture?J.compressedTexSubImage2D(J.TEXTURE_2D,je,z,Ce,Qe.width,Qe.height,at,Qe.data):J.texSubImage2D(J.TEXTURE_2D,je,z,Ce,$e,Ye,at,zt,Qe);R.pixelStorei(J.UNPACK_ROW_LENGTH,ct),R.pixelStorei(J.UNPACK_IMAGE_HEIGHT,lt),R.pixelStorei(J.UNPACK_SKIP_PIXELS,dt),R.pixelStorei(J.UNPACK_SKIP_ROWS,Pt),R.pixelStorei(J.UNPACK_SKIP_IMAGES,Et),je===0&&ee.generateMipmaps&&J.generateMipmap(Oe),R.unbindTexture()},this.initRenderTarget=function(L){oe.get(L).__webglFramebuffer===void 0&&xe.setupRenderTarget(L)},this.initTexture=function(L){L.isCubeTexture?xe.setTextureCube(L,0):L.isData3DTexture?xe.setTexture3D(L,0):L.isDataArrayTexture||L.isCompressedArrayTexture?xe.setTexture2DArray(L,0):xe.setTexture2D(L,0),R.unbindTexture()},this.resetState=function(){H=0,D=0,V=null,R.reset(),Pe.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return ui}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(e){this._outputColorSpace=e;let t=this.getContext();t.drawingBufferColorSpace=Tt._getDrawingBufferColorSpace(e),t.unpackColorSpace=Tt._getUnpackColorSpace()}};function uu(s,e){if(e===Hh)return console.warn("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Geometry already defined as triangles."),s;if(e===Gr||e===no){let t=s.getIndex();if(t===null){let r=[],a=s.getAttribute("position");if(a!==void 0){for(let o=0;o<a.count;o++)r.push(o);s.setIndex(r),t=s.getIndex()}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Undefined position attribute. Processing not possible."),s}let n=t.count-2,i=[];if(e===Gr)for(let r=1;r<=n;r++)i.push(t.getX(0)),i.push(t.getX(r)),i.push(t.getX(r+1));else for(let r=0;r<n;r++)r%2===0?(i.push(t.getX(r)),i.push(t.getX(r+1)),i.push(t.getX(r+2))):(i.push(t.getX(r+2)),i.push(t.getX(r+1)),i.push(t.getX(r)));return i.length/3!==n&&console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unable to generate correct amount of triangles."),s.setIndex(i),s.clearGroups(),s}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unknown draw mode:",e),s}function _p(s){let e=new Map,t=new Map,n=s.clone();return yp(s,n,function(i,r){e.set(r,i),t.set(i,r)}),n.traverse(function(i){if(!i.isSkinnedMesh)return;let r=i,a=e.get(i),o=a.skeleton.bones;r.skeleton=a.skeleton.clone(),r.bindMatrix.copy(a.bindMatrix),r.skeleton.bones=o.map(function(l){return t.get(l)}),r.bind(r.skeleton,r.bindMatrix)}),n}function yp(s,e,t){t(s,e);for(let n=0;n<s.children.length;n++)yp(s.children[n],e.children[n],t)}var yi=class extends Di{constructor(e){super(e),this.dracoLoader=null,this.ktx2Loader=null,this.meshoptDecoder=null,this.pluginCallbacks=[],this.register(function(t){return new bu(t)}),this.register(function(t){return new vu(t)}),this.register(function(t){return new Ru(t)}),this.register(function(t){return new Cu(t)}),this.register(function(t){return new Pu(t)}),this.register(function(t){return new yu(t)}),this.register(function(t){return new Mu(t)}),this.register(function(t){return new Su(t)}),this.register(function(t){return new wu(t)}),this.register(function(t){return new xu(t)}),this.register(function(t){return new Eu(t)}),this.register(function(t){return new _u(t)}),this.register(function(t){return new Au(t)}),this.register(function(t){return new Tu(t)}),this.register(function(t){return new mu(t)}),this.register(function(t){return new wc(t,Dt.EXT_MESHOPT_COMPRESSION)}),this.register(function(t){return new wc(t,Dt.KHR_MESHOPT_COMPRESSION)}),this.register(function(t){return new Iu(t)})}load(e,t,n,i){let r=this,a;if(this.resourcePath!=="")a=this.resourcePath;else if(this.path!==""){let c=Qi.extractUrlBase(e);a=Qi.resolveURL(c,this.path)}else a=Qi.extractUrlBase(e);this.manager.itemStart(e);let o=function(c){i?i(c):console.error(c),r.manager.itemError(e),r.manager.itemEnd(e)},l=new Fr(this.manager);l.setPath(this.path),l.setResponseType("arraybuffer"),l.setRequestHeader(this.requestHeader),l.setWithCredentials(this.withCredentials),l.load(e,function(c){try{r.parse(c,a,function(h){t(h),r.manager.itemEnd(e)},o)}catch(h){o(h)}},n,o)}setDRACOLoader(e){return this.dracoLoader=e,this}setKTX2Loader(e){return this.ktx2Loader=e,this}setMeshoptDecoder(e){return this.meshoptDecoder=e,this}register(e){return this.pluginCallbacks.indexOf(e)===-1&&this.pluginCallbacks.push(e),this}unregister(e){return this.pluginCallbacks.indexOf(e)!==-1&&this.pluginCallbacks.splice(this.pluginCallbacks.indexOf(e),1),this}parse(e,t,n,i){let r,a={},o={},l=new TextDecoder;if(typeof e=="string")r=JSON.parse(e);else if(e instanceof ArrayBuffer)if(l.decode(new Uint8Array(e,0,4))===Tp){try{a[Dt.KHR_BINARY_GLTF]=new Du(e)}catch(u){i&&i(u);return}r=JSON.parse(a[Dt.KHR_BINARY_GLTF].content)}else r=JSON.parse(l.decode(e));else r=e;if(r.asset===void 0||r.asset.version[0]<2){i&&i(new Error("THREE.GLTFLoader: Unsupported asset. glTF versions >=2.0 are supported."));return}let c=new ku(r,{path:t||this.resourcePath||"",crossOrigin:this.crossOrigin,requestHeader:this.requestHeader,manager:this.manager,ktx2Loader:this.ktx2Loader,meshoptDecoder:this.meshoptDecoder});c.fileLoader.setRequestHeader(this.requestHeader);for(let h=0;h<this.pluginCallbacks.length;h++){let u=this.pluginCallbacks[h](c);u.name||console.error("THREE.GLTFLoader: Invalid plugin found: missing name"),o[u.name]=u,a[u.name]=!0}if(r.extensionsUsed)for(let h=0;h<r.extensionsUsed.length;++h){let u=r.extensionsUsed[h],d=r.extensionsRequired||[];switch(u){case Dt.KHR_MATERIALS_UNLIT:a[u]=new gu;break;case Dt.KHR_DRACO_MESH_COMPRESSION:a[u]=new Lu(r,this.dracoLoader);break;case Dt.KHR_TEXTURE_TRANSFORM:a[u]=new Fu;break;case Dt.KHR_MESH_QUANTIZATION:a[u]=new Nu;break;default:d.indexOf(u)>=0&&o[u]===void 0&&console.warn('THREE.GLTFLoader: Unknown extension "'+u+'".')}}c.setExtensions(a),c.setPlugins(o),c.parse(n,i)}parseAsync(e,t){let n=this;return new Promise(function(i,r){n.parse(e,t,i,r)})}};function W_(){let s={};return{get:function(e){return s[e]},add:function(e,t){s[e]=t},remove:function(e){delete s[e]},removeAll:function(){s={}}}}function hn(s,e,t){let n=s.json.materials[e];return n.extensions&&n.extensions[t]?n.extensions[t]:null}var Dt={KHR_BINARY_GLTF:"KHR_binary_glTF",KHR_DRACO_MESH_COMPRESSION:"KHR_draco_mesh_compression",KHR_LIGHTS_PUNCTUAL:"KHR_lights_punctual",KHR_MATERIALS_CLEARCOAT:"KHR_materials_clearcoat",KHR_MATERIALS_DISPERSION:"KHR_materials_dispersion",KHR_MATERIALS_IOR:"KHR_materials_ior",KHR_MATERIALS_SHEEN:"KHR_materials_sheen",KHR_MATERIALS_SPECULAR:"KHR_materials_specular",KHR_MATERIALS_TRANSMISSION:"KHR_materials_transmission",KHR_MATERIALS_IRIDESCENCE:"KHR_materials_iridescence",KHR_MATERIALS_ANISOTROPY:"KHR_materials_anisotropy",KHR_MATERIALS_UNLIT:"KHR_materials_unlit",KHR_MATERIALS_VOLUME:"KHR_materials_volume",KHR_TEXTURE_BASISU:"KHR_texture_basisu",KHR_TEXTURE_TRANSFORM:"KHR_texture_transform",KHR_MESH_QUANTIZATION:"KHR_mesh_quantization",KHR_MATERIALS_EMISSIVE_STRENGTH:"KHR_materials_emissive_strength",EXT_MATERIALS_BUMP:"EXT_materials_bump",EXT_TEXTURE_WEBP:"EXT_texture_webp",EXT_TEXTURE_AVIF:"EXT_texture_avif",EXT_MESHOPT_COMPRESSION:"EXT_meshopt_compression",KHR_MESHOPT_COMPRESSION:"KHR_meshopt_compression",EXT_MESH_GPU_INSTANCING:"EXT_mesh_gpu_instancing"},mu=class{constructor(e){this.parser=e,this.name=Dt.KHR_LIGHTS_PUNCTUAL,this.cache={refs:{},uses:{}}}_markDefs(){let e=this.parser,t=this.parser.json.nodes||[];for(let n=0,i=t.length;n<i;n++){let r=t[n];r.extensions&&r.extensions[this.name]&&r.extensions[this.name].light!==void 0&&e._addNodeRef(this.cache,r.extensions[this.name].light)}}_loadLight(e){let t=this.parser,n="light:"+e,i=t.cache.get(n);if(i)return i;let r=t.json,l=((r.extensions&&r.extensions[this.name]||{}).lights||[])[e],c,h=new Le(16777215);l.color!==void 0&&h.setRGB(l.color[0],l.color[1],l.color[2],Un);let u=l.range!==void 0?l.range:0;switch(l.type){case"directional":c=new $i(h),c.target.position.set(0,0,-1),c.add(c.target);break;case"point":c=new gi(h),c.distance=u;break;case"spot":c=new Oa(h),c.distance=u,l.spot=l.spot||{},l.spot.innerConeAngle=l.spot.innerConeAngle!==void 0?l.spot.innerConeAngle:0,l.spot.outerConeAngle=l.spot.outerConeAngle!==void 0?l.spot.outerConeAngle:Math.PI/4,c.angle=l.spot.outerConeAngle,c.penumbra=1-l.spot.innerConeAngle/l.spot.outerConeAngle,c.target.position.set(0,0,-1),c.add(c.target);break;default:throw new Error("THREE.GLTFLoader: Unexpected light type: "+l.type)}return c.position.set(0,0,0),Ui(c,l),l.intensity!==void 0&&(c.intensity=l.intensity),c.name=t.createUniqueName(l.name||"light_"+e),i=Promise.resolve(c),t.cache.add(n,i),i}getDependency(e,t){if(e==="light")return this._loadLight(t)}createNodeAttachment(e){let t=this,n=this.parser,r=n.json.nodes[e],o=(r.extensions&&r.extensions[this.name]||{}).light;return o===void 0?null:this._loadLight(o).then(function(l){return n._getNodeRef(t.cache,o,l)})}},gu=class{constructor(){this.name=Dt.KHR_MATERIALS_UNLIT}getMaterialType(){return Vt}extendParams(e,t,n){let i=[];e.color=new Le(1,1,1),e.opacity=1;let r=t.pbrMetallicRoughness;if(r){if(Array.isArray(r.baseColorFactor)){let a=r.baseColorFactor;e.color.setRGB(a[0],a[1],a[2],Un),e.opacity=a[3]}r.baseColorTexture!==void 0&&i.push(n.assignTexture(e,"map",r.baseColorTexture,sn))}return Promise.all(i)}},xu=class{constructor(e){this.parser=e,this.name=Dt.KHR_MATERIALS_EMISSIVE_STRENGTH}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);return n===null||n.emissiveStrength!==void 0&&(t.emissiveIntensity=n.emissiveStrength),Promise.resolve()}},bu=class{constructor(e){this.parser=e,this.name=Dt.KHR_MATERIALS_CLEARCOAT}getMaterialType(e){return hn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(n.clearcoatFactor!==void 0&&(t.clearcoat=n.clearcoatFactor),n.clearcoatTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatMap",n.clearcoatTexture)),n.clearcoatRoughnessFactor!==void 0&&(t.clearcoatRoughness=n.clearcoatRoughnessFactor),n.clearcoatRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatRoughnessMap",n.clearcoatRoughnessTexture)),n.clearcoatNormalTexture!==void 0&&(i.push(this.parser.assignTexture(t,"clearcoatNormalMap",n.clearcoatNormalTexture)),n.clearcoatNormalTexture.scale!==void 0)){let r=n.clearcoatNormalTexture.scale;t.clearcoatNormalScale=new Be(r,r)}return Promise.all(i)}},vu=class{constructor(e){this.parser=e,this.name=Dt.KHR_MATERIALS_DISPERSION}getMaterialType(e){return hn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);return n===null||(t.dispersion=n.dispersion!==void 0?n.dispersion:0),Promise.resolve()}},_u=class{constructor(e){this.parser=e,this.name=Dt.KHR_MATERIALS_IRIDESCENCE}getMaterialType(e){return hn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.iridescenceFactor!==void 0&&(t.iridescence=n.iridescenceFactor),n.iridescenceTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceMap",n.iridescenceTexture)),n.iridescenceIor!==void 0&&(t.iridescenceIOR=n.iridescenceIor),t.iridescenceThicknessRange===void 0&&(t.iridescenceThicknessRange=[100,400]),n.iridescenceThicknessMinimum!==void 0&&(t.iridescenceThicknessRange[0]=n.iridescenceThicknessMinimum),n.iridescenceThicknessMaximum!==void 0&&(t.iridescenceThicknessRange[1]=n.iridescenceThicknessMaximum),n.iridescenceThicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceThicknessMap",n.iridescenceThicknessTexture)),Promise.all(i)}},yu=class{constructor(e){this.parser=e,this.name=Dt.KHR_MATERIALS_SHEEN}getMaterialType(e){return hn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(t.sheenColor=new Le(0,0,0),t.sheenRoughness=0,t.sheen=1,n.sheenColorFactor!==void 0){let r=n.sheenColorFactor;t.sheenColor.setRGB(r[0],r[1],r[2],Un)}return n.sheenRoughnessFactor!==void 0&&(t.sheenRoughness=n.sheenRoughnessFactor),n.sheenColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenColorMap",n.sheenColorTexture,sn)),n.sheenRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenRoughnessMap",n.sheenRoughnessTexture)),Promise.all(i)}},Mu=class{constructor(e){this.parser=e,this.name=Dt.KHR_MATERIALS_TRANSMISSION}getMaterialType(e){return hn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.transmissionFactor!==void 0&&(t.transmission=n.transmissionFactor),n.transmissionTexture!==void 0&&i.push(this.parser.assignTexture(t,"transmissionMap",n.transmissionTexture)),Promise.all(i)}},Su=class{constructor(e){this.parser=e,this.name=Dt.KHR_MATERIALS_VOLUME}getMaterialType(e){return hn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.thickness=n.thicknessFactor!==void 0?n.thicknessFactor:0,n.thicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"thicknessMap",n.thicknessTexture)),t.attenuationDistance=n.attenuationDistance||1/0;let r=n.attenuationColor||[1,1,1];return t.attenuationColor=new Le().setRGB(r[0],r[1],r[2],Un),Promise.all(i)}},wu=class{constructor(e){this.parser=e,this.name=Dt.KHR_MATERIALS_IOR}getMaterialType(e){return hn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);return n===null||(t.ior=n.ior!==void 0?n.ior:1.5,t.ior===0&&(t.ior=1e3)),Promise.resolve()}},Eu=class{constructor(e){this.parser=e,this.name=Dt.KHR_MATERIALS_SPECULAR}getMaterialType(e){return hn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.specularIntensity=n.specularFactor!==void 0?n.specularFactor:1,n.specularTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularIntensityMap",n.specularTexture));let r=n.specularColorFactor||[1,1,1];return t.specularColor=new Le().setRGB(r[0],r[1],r[2],Un),n.specularColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularColorMap",n.specularColorTexture,sn)),Promise.all(i)}},Tu=class{constructor(e){this.parser=e,this.name=Dt.EXT_MATERIALS_BUMP}getMaterialType(e){return hn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return t.bumpScale=n.bumpFactor!==void 0?n.bumpFactor:1,n.bumpTexture!==void 0&&i.push(this.parser.assignTexture(t,"bumpMap",n.bumpTexture)),Promise.all(i)}},Au=class{constructor(e){this.parser=e,this.name=Dt.KHR_MATERIALS_ANISOTROPY}getMaterialType(e){return hn(this.parser,e,this.name)!==null?Bn:null}extendMaterialParams(e,t){let n=hn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.anisotropyStrength!==void 0&&(t.anisotropy=n.anisotropyStrength),n.anisotropyRotation!==void 0&&(t.anisotropyRotation=n.anisotropyRotation),n.anisotropyTexture!==void 0&&i.push(this.parser.assignTexture(t,"anisotropyMap",n.anisotropyTexture)),Promise.all(i)}},Ru=class{constructor(e){this.parser=e,this.name=Dt.KHR_TEXTURE_BASISU}loadTexture(e){let t=this.parser,n=t.json,i=n.textures[e];if(!i.extensions||!i.extensions[this.name])return null;let r=i.extensions[this.name],a=t.options.ktx2Loader;if(!a){if(n.extensionsRequired&&n.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setKTX2Loader must be called before loading KTX2 textures");return null}return t.loadTextureImage(e,r.source,a)}},Cu=class{constructor(e){this.parser=e,this.name=Dt.EXT_TEXTURE_WEBP}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let a=r.extensions[t],o=i.images[a.source],l=n.textureLoader;if(o.uri){let c=n.options.manager.getHandler(o.uri);c!==null&&(l=c)}return n.loadTextureImage(e,a.source,l)}},Pu=class{constructor(e){this.parser=e,this.name=Dt.EXT_TEXTURE_AVIF}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let a=r.extensions[t],o=i.images[a.source],l=n.textureLoader;if(o.uri){let c=n.options.manager.getHandler(o.uri);c!==null&&(l=c)}return n.loadTextureImage(e,a.source,l)}},wc=class{constructor(e,t){this.name=t,this.parser=e}loadBufferView(e){let t=this.parser.json,n=t.bufferViews[e];if(n.extensions&&n.extensions[this.name]){let i=n.extensions[this.name],r=this.parser.getDependency("buffer",i.buffer),a=this.parser.options.meshoptDecoder;if(!a||!a.supported){if(t.extensionsRequired&&t.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setMeshoptDecoder must be called before loading compressed files");return null}return r.then(function(o){let l=i.byteOffset||0,c=i.byteLength||0,h=i.count,u=i.byteStride,d=new Uint8Array(o,l,c);return a.decodeGltfBufferAsync?a.decodeGltfBufferAsync(h,u,d,i.mode,i.filter).then(function(p){return p.buffer}):a.ready.then(function(){let p=new ArrayBuffer(h*u);return a.decodeGltfBuffer(new Uint8Array(p),h,u,d,i.mode,i.filter),p})})}else return null}},Iu=class{constructor(e){this.name=Dt.EXT_MESH_GPU_INSTANCING,this.parser=e}createNodeMesh(e){let t=this.parser.json,n=t.nodes[e];if(!n.extensions||!n.extensions[this.name]||n.mesh===void 0)return null;let i=t.meshes[n.mesh];for(let c of i.primitives)if(c.mode!==si.TRIANGLES&&c.mode!==si.TRIANGLE_STRIP&&c.mode!==si.TRIANGLE_FAN&&c.mode!==void 0)return null;let a=n.extensions[this.name].attributes,o=[],l={};for(let c in a)o.push(this.parser.getDependency("accessor",a[c]).then(h=>(l[c]=h,l[c])));return o.length<1?null:(o.push(this.parser.createNodeMesh(e)),Promise.all(o).then(c=>{let h=c.pop(),u=h.isGroup?h.children:[h],d=c[0].count,p=[];for(let x of u){let v=new ft,m=new B,f=new Qt,y=new B(1,1,1),T=new Sn(x.geometry,x.material,d);for(let _=0;_<d;_++)l.TRANSLATION&&m.fromBufferAttribute(l.TRANSLATION,_),l.ROTATION&&f.fromBufferAttribute(l.ROTATION,_),l.SCALE&&y.fromBufferAttribute(l.SCALE,_),T.setMatrixAt(_,v.compose(m,f,y));let g=null;for(let _ in l)if(_==="_COLOR_0"){let w=l[_];T.instanceColor=new fi(w.array,w.itemSize,w.normalized)}else if(_!=="TRANSLATION"&&_!=="ROTATION"&&_!=="SCALE"){if(g===null){let A=T.geometry;g=new _t,g.name=A.name;for(let b in A.attributes)g.setAttribute(b,A.attributes[b]);for(let b in A.morphAttributes)g.morphAttributes[b]=A.morphAttributes[b];A.index!==null&&g.setIndex(A.index),g.morphTargetsRelative=A.morphTargetsRelative;for(let b of A.groups)g.addGroup(b.start,b.count,b.materialIndex);A.boundingBox!==null&&(g.boundingBox=A.boundingBox.clone()),A.boundingSphere!==null&&(g.boundingSphere=A.boundingSphere.clone()),g.drawRange.start=A.drawRange.start,g.drawRange.count=A.drawRange.count,g.userData=Object.assign({},A.userData),T.geometry=g}let w=l[_];g.setAttribute(_,new fi(w.array,w.itemSize,w.normalized))}Wt.prototype.copy.call(T,x),this.parser.assignFinalMaterial(T),p.push(T)}return h.isGroup?(h.clear(),h.add(...p),h):p[0]}))}},Tp="glTF",oo=12,Mp={JSON:1313821514,BIN:5130562},Du=class{constructor(e){this.name=Dt.KHR_BINARY_GLTF,this.content=null,this.body=null;let t=new DataView(e,0,oo),n=new TextDecoder;if(this.header={magic:n.decode(new Uint8Array(e.slice(0,4))),version:t.getUint32(4,!0),length:t.getUint32(8,!0)},this.header.magic!==Tp)throw new Error("THREE.GLTFLoader: Unsupported glTF-Binary header.");if(this.header.version<2)throw new Error("THREE.GLTFLoader: Legacy binary file detected.");let i=this.header.length-oo,r=new DataView(e,oo),a=0;for(;a<i;){let o=r.getUint32(a,!0);a+=4;let l=r.getUint32(a,!0);if(a+=4,l===Mp.JSON){let c=new Uint8Array(e,oo+a,o);this.content=n.decode(c)}else if(l===Mp.BIN){let c=oo+a;this.body=e.slice(c,c+o)}a+=o}if(this.content===null)throw new Error("THREE.GLTFLoader: JSON content not found.")}},Lu=class{constructor(e,t){if(!t)throw new Error("THREE.GLTFLoader: No DRACOLoader instance provided.");this.name=Dt.KHR_DRACO_MESH_COMPRESSION,this.json=e,this.dracoLoader=t,this.dracoLoader.preload()}decodePrimitive(e,t){let n=this.json,i=this.dracoLoader,r=e.extensions[this.name].bufferView,a=e.extensions[this.name].attributes,o={},l={},c={};for(let h in a){let u=Ou[h]||h.toLowerCase();o[u]=a[h]}for(let h in e.attributes){let u=Ou[h]||h.toLowerCase();if(a[h]!==void 0){let d=n.accessors[e.attributes[h]],p=Kr[d.componentType];c[u]=p.name,l[u]=d.normalized===!0}}return t.getDependency("bufferView",r).then(function(h){return new Promise(function(u,d){i.decodeDracoFile(h,function(p){for(let x in p.attributes){let v=p.attributes[x],m=l[x];m!==void 0&&(v.normalized=m)}u(p)},o,c,Un,d)})})}},Fu=class{constructor(){this.name=Dt.KHR_TEXTURE_TRANSFORM}extendTexture(e,t){if((t.texCoord===void 0||t.texCoord===e.channel)&&t.offset===void 0&&t.rotation===void 0&&t.scale===void 0)return e;if(e=e.clone(),t.texCoord!==void 0&&(e.channel=t.texCoord),t.offset!==void 0&&e.offset.fromArray(t.offset),t.rotation!==void 0&&(e.rotation=t.rotation),t.scale!==void 0&&e.repeat.fromArray(t.scale),t.rotation!==void 0){let n=Math.cos(e.rotation),i=Math.sin(e.rotation);e.matrix.set(e.repeat.x*n,e.repeat.y*i,e.offset.x,-e.repeat.x*i,e.repeat.y*n,e.offset.y,0,0,1),e.matrixAutoUpdate=!1}return e.needsUpdate=!0,e}},Nu=class{constructor(){this.name=Dt.KHR_MESH_QUANTIZATION}},Ec=class extends Ii{constructor(e,t,n,i){super(e,t,n,i)}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i*3+i;for(let a=0;a!==i;a++)t[a]=n[r+a];return t}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=o*2,c=o*3,h=i-t,u=(n-t)/h,d=u*u,p=d*u,x=e*c,v=x-c,m=-2*p+3*d,f=p-d,y=1-m,T=f-d+u;for(let g=0;g!==o;g++){let _=a[v+g+o],w=a[v+g+l]*h,A=a[x+g+o],b=a[x+g]*h;r[g]=y*_+T*w+m*A+f*b}return r}},q_=new Qt,Uu=class extends Ec{interpolate_(e,t,n,i){let r=super.interpolate_(e,t,n,i);return q_.fromArray(r).normalize().toArray(r),r}},si={FLOAT:5126,FLOAT_MAT3:35675,FLOAT_MAT4:35676,FLOAT_VEC2:35664,FLOAT_VEC3:35665,FLOAT_VEC4:35666,LINEAR:9729,REPEAT:10497,SAMPLER_2D:35678,POINTS:0,LINES:1,LINE_LOOP:2,LINE_STRIP:3,TRIANGLES:4,TRIANGLE_STRIP:5,TRIANGLE_FAN:6,UNSIGNED_BYTE:5121,UNSIGNED_SHORT:5123},Kr={5120:Int8Array,5121:Uint8Array,5122:Int16Array,5123:Uint16Array,5125:Uint32Array,5126:Float32Array},Sp={9728:on,9729:rn,9984:Pl,9985:kr,9986:Xs,9987:bi},wp={33071:Qn,33648:xr,10497:Ti},du={SCALAR:1,VEC2:2,VEC3:3,VEC4:4,MAT2:4,MAT3:9,MAT4:16},Ou={POSITION:"position",NORMAL:"normal",TANGENT:"tangent",TEXCOORD_0:"uv",TEXCOORD_1:"uv1",TEXCOORD_2:"uv2",TEXCOORD_3:"uv3",COLOR_0:"color",WEIGHTS_0:"skinWeight",JOINTS_0:"skinIndex"},_s={scale:"scale",translation:"position",rotation:"quaternion",weights:"morphTargetInfluences"},X_={CUBICSPLINE:void 0,LINEAR:Fs,STEP:Ls},fu={OPAQUE:"OPAQUE",MASK:"MASK",BLEND:"BLEND"};function j_(s){return s.DefaultMaterial===void 0&&(s.DefaultMaterial=new zn({color:16777215,emissive:0,metalness:1,roughness:1,transparent:!1,depthTest:!0,side:ti})),s.DefaultMaterial}function Ys(s,e,t){for(let n in t.extensions)s[n]===void 0&&(e.userData.gltfExtensions=e.userData.gltfExtensions||{},e.userData.gltfExtensions[n]=t.extensions[n])}function Ui(s,e){e.extras!==void 0&&(typeof e.extras=="object"?Object.assign(s.userData,e.extras):console.warn("THREE.GLTFLoader: Ignoring primitive type .extras, "+e.extras))}function K_(s,e,t){let n=!1,i=!1,r=!1;for(let c=0,h=e.length;c<h;c++){let u=e[c];if(u.POSITION!==void 0&&(n=!0),u.NORMAL!==void 0&&(i=!0),u.COLOR_0!==void 0&&(r=!0),n&&i&&r)break}if(!n&&!i&&!r)return Promise.resolve(s);let a=[],o=[],l=[];for(let c=0,h=e.length;c<h;c++){let u=e[c];if(n){let d=u.POSITION!==void 0?t.getDependency("accessor",u.POSITION):s.attributes.position;a.push(d)}if(i){let d=u.NORMAL!==void 0?t.getDependency("accessor",u.NORMAL):s.attributes.normal;o.push(d)}if(r){let d=u.COLOR_0!==void 0?t.getDependency("accessor",u.COLOR_0):s.attributes.color;l.push(d)}}return Promise.all([Promise.all(a),Promise.all(o),Promise.all(l)]).then(function(c){let h=c[0],u=c[1],d=c[2];return n&&(s.morphAttributes.position=h),i&&(s.morphAttributes.normal=u),r&&(s.morphAttributes.color=d),s.morphTargetsRelative=!0,s})}function Y_(s,e){if(s.updateMorphTargets(),e.weights!==void 0)for(let t=0,n=e.weights.length;t<n;t++)s.morphTargetInfluences[t]=e.weights[t];if(e.extras&&Array.isArray(e.extras.targetNames)){let t=e.extras.targetNames;if(s.morphTargetInfluences.length===t.length){s.morphTargetDictionary={};for(let n=0,i=t.length;n<i;n++)s.morphTargetDictionary[t[n]]=n}else console.warn("THREE.GLTFLoader: Invalid extras.targetNames length. Ignoring names.")}}function J_(s){let e,t=s.extensions&&s.extensions[Dt.KHR_DRACO_MESH_COMPRESSION];if(t?e="draco:"+t.bufferView+":"+t.indices+":"+pu(t.attributes):e=s.indices+":"+pu(s.attributes)+":"+s.mode,s.targets!==void 0)for(let n=0,i=s.targets.length;n<i;n++)e+=":"+pu(s.targets[n]);return e}function pu(s){let e="",t=Object.keys(s).sort();for(let n=0,i=t.length;n<i;n++)e+=t[n]+":"+s[t[n]]+";";return e}function zu(s){switch(s){case Int8Array:return 1/127;case Uint8Array:return 1/255;case Int16Array:return 1/32767;case Uint16Array:return 1/65535;default:throw new Error("THREE.GLTFLoader: Unsupported normalized accessor component type.")}}function Z_(s){return s.search(/\.jpe?g($|\?)/i)>0||s.search(/^data\:image\/jpeg/)===0?"image/jpeg":s.search(/\.webp($|\?)/i)>0||s.search(/^data\:image\/webp/)===0?"image/webp":s.search(/\.ktx2($|\?)/i)>0||s.search(/^data\:image\/ktx2/)===0?"image/ktx2":"image/png"}var $_=new ft,ku=class{constructor(e={},t={}){this.json=e,this.extensions={},this.plugins={},this.options=t,this.cache=new W_,this.associations=new Map,this.primitiveCache={},this.nodeCache={},this.meshCache={refs:{},uses:{}},this.cameraCache={refs:{},uses:{}},this.lightCache={refs:{},uses:{}},this.sourceCache={},this.textureCache={},this.nodeNamesUsed={};let n=!1,i=-1,r=!1,a=-1;if(typeof navigator<"u"&&typeof navigator.userAgent<"u"){let o=navigator.userAgent;n=/^((?!chrome|android).)*safari/i.test(o)===!0;let l=o.match(/Version\/(\d+)/);i=n&&l?parseInt(l[1],10):-1,r=o.indexOf("Firefox")>-1,a=r?o.match(/Firefox\/([0-9]+)\./)[1]:-1}typeof createImageBitmap>"u"||n&&i<17||r&&a<98?this.textureLoader=new Fa(this.options.manager):this.textureLoader=new za(this.options.manager),this.textureLoader.setCrossOrigin(this.options.crossOrigin),this.textureLoader.setRequestHeader(this.options.requestHeader),this.fileLoader=new Fr(this.options.manager),this.fileLoader.setResponseType("arraybuffer"),this.options.crossOrigin==="use-credentials"&&this.fileLoader.setWithCredentials(!0)}setExtensions(e){this.extensions=e}setPlugins(e){this.plugins=e}parse(e,t){let n=this,i=this.json,r=this.extensions;this.cache.removeAll(),this.nodeCache={},this._invokeAll(function(a){return a._markDefs&&a._markDefs()}),Promise.all(this._invokeAll(function(a){return a.beforeRoot&&a.beforeRoot()})).then(function(){return Promise.all([n.getDependencies("scene"),n.getDependencies("animation"),n.getDependencies("camera")])}).then(function(a){let o={scene:a[0][i.scene||0],scenes:a[0],animations:a[1],cameras:a[2],asset:i.asset,parser:n,userData:{}};return Ys(r,o,i),Ui(o,i),Promise.all(n._invokeAll(function(l){return l.afterRoot&&l.afterRoot(o)})).then(function(){for(let l of o.scenes)l.updateMatrixWorld();e(o)})}).catch(t)}_markDefs(){let e=this.json.nodes||[],t=this.json.skins||[],n=this.json.meshes||[];for(let i=0,r=t.length;i<r;i++){let a=t[i].joints;for(let o=0,l=a.length;o<l;o++)e[a[o]].isBone=!0}for(let i=0,r=e.length;i<r;i++){let a=e[i];a.mesh!==void 0&&(this._addNodeRef(this.meshCache,a.mesh),a.skin!==void 0&&(n[a.mesh].isSkinnedMesh=!0)),a.camera!==void 0&&this._addNodeRef(this.cameraCache,a.camera)}}_addNodeRef(e,t){t!==void 0&&(e.refs[t]===void 0&&(e.refs[t]=e.uses[t]=0),e.refs[t]++)}_getNodeRef(e,t,n){if(e.refs[t]<=1)return n;let i=n.clone(),r=(a,o)=>{let l=this.associations.get(a);l!=null&&this.associations.set(o,l);for(let[c,h]of a.children.entries())r(h,o.children[c])};return r(n,i),i.name+="_instance_"+e.uses[t]++,i}_invokeOne(e){let t=Object.values(this.plugins);t.push(this);for(let n=0;n<t.length;n++){let i=e(t[n]);if(i)return i}return null}_invokeAll(e){let t=Object.values(this.plugins);t.unshift(this);let n=[];for(let i=0;i<t.length;i++){let r=e(t[i]);r&&n.push(r)}return n}getDependency(e,t){let n=e+":"+t,i=this.cache.get(n);if(!i){switch(e){case"scene":i=this.loadScene(t);break;case"node":i=this._invokeOne(function(r){return r.loadNode&&r.loadNode(t)});break;case"mesh":i=this._invokeOne(function(r){return r.loadMesh&&r.loadMesh(t)});break;case"accessor":i=this.loadAccessor(t);break;case"bufferView":i=this._invokeOne(function(r){return r.loadBufferView&&r.loadBufferView(t)});break;case"buffer":i=this.loadBuffer(t);break;case"material":i=this._invokeOne(function(r){return r.loadMaterial&&r.loadMaterial(t)});break;case"texture":i=this._invokeOne(function(r){return r.loadTexture&&r.loadTexture(t)});break;case"skin":i=this.loadSkin(t);break;case"animation":i=this._invokeOne(function(r){return r.loadAnimation&&r.loadAnimation(t)});break;case"camera":i=this.loadCamera(t);break;default:if(i=this._invokeOne(function(r){return r!=this&&r.getDependency&&r.getDependency(e,t)}),!i)throw new Error("Unknown type: "+e);break}this.cache.add(n,i)}return i}getDependencies(e){let t=this.cache.get(e);if(!t){let n=this,i=this.json[e+(e==="mesh"?"es":"s")]||[];t=Promise.all(i.map(function(r,a){return n.getDependency(e,a)})),this.cache.add(e,t)}return t}loadBuffer(e){let t=this.json.buffers[e],n=this.fileLoader;if(t.type&&t.type!=="arraybuffer")throw new Error("THREE.GLTFLoader: "+t.type+" buffer type is not supported.");if(t.uri===void 0&&e===0)return Promise.resolve(this.extensions[Dt.KHR_BINARY_GLTF].body);let i=this.options;return new Promise(function(r,a){n.load(Qi.resolveURL(t.uri,i.path),r,void 0,function(){a(new Error('THREE.GLTFLoader: Failed to load buffer "'+t.uri+'".'))})})}loadBufferView(e){let t=this.json.bufferViews[e];return this.getDependency("buffer",t.buffer).then(function(n){let i=t.byteLength||0,r=t.byteOffset||0;return n.slice(r,r+i)})}loadAccessor(e){let t=this,n=this.json,i=this.json.accessors[e];if(i.bufferView===void 0&&i.sparse===void 0){let a=du[i.type],o=Kr[i.componentType],l=i.normalized===!0,c=new o(i.count*a);return Promise.resolve(new Rt(c,a,l))}let r=[];return i.bufferView!==void 0?r.push(this.getDependency("bufferView",i.bufferView)):r.push(null),i.sparse!==void 0&&(r.push(this.getDependency("bufferView",i.sparse.indices.bufferView)),r.push(this.getDependency("bufferView",i.sparse.values.bufferView))),Promise.all(r).then(function(a){let o=a[0],l=du[i.type],c=Kr[i.componentType],h=c.BYTES_PER_ELEMENT,u=h*l,d=i.byteOffset||0,p=i.bufferView!==void 0?n.bufferViews[i.bufferView].byteStride:void 0,x=i.normalized===!0,v,m;if(p&&p!==u){let f=Math.floor(d/p),y="InterleavedBuffer:"+i.bufferView+":"+i.componentType+":"+f+":"+i.count,T=t.cache.get(y);T||(v=new c(o,f*p,i.count*p/h),T=new wr(v,p/h),t.cache.add(y,T)),m=new Er(T,l,d%p/h,x)}else o===null?v=new c(i.count*l):v=new c(o,d,i.count*l),m=new Rt(v,l,x);if(i.sparse!==void 0){let f=du.SCALAR,y=Kr[i.sparse.indices.componentType],T=i.sparse.indices.byteOffset||0,g=i.sparse.values.byteOffset||0,_=new y(a[1],T,i.sparse.count*f),w=new c(a[2],g,i.sparse.count*l);o!==null&&(m=new Rt(m.array.slice(),m.itemSize,m.normalized)),m.normalized=!1;for(let A=0,b=_.length;A<b;A++){let C=_[A];if(m.setX(C,w[A*l]),l>=2&&m.setY(C,w[A*l+1]),l>=3&&m.setZ(C,w[A*l+2]),l>=4&&m.setW(C,w[A*l+3]),l>=5)throw new Error("THREE.GLTFLoader: Unsupported itemSize in sparse BufferAttribute.")}m.normalized=x}return m})}loadTexture(e){let t=this.json,n=this.options,r=t.textures[e].source,a=t.images[r],o=this.textureLoader;if(a.uri){let l=n.manager.getHandler(a.uri);l!==null&&(o=l)}return this.loadTextureImage(e,r,o)}loadTextureImage(e,t,n){let i=this,r=this.json,a=r.textures[e],o=r.images[t],l=(o.uri||o.bufferView)+":"+a.sampler;if(this.textureCache[l])return this.textureCache[l];let c=this.loadImageSource(t,n).then(function(h){h.flipY=!1,h.name=a.name||o.name||"",h.name===""&&typeof o.uri=="string"&&o.uri.startsWith("data:image/")===!1&&(h.name=o.uri);let d=(r.samplers||{})[a.sampler]||{};return h.magFilter=Sp[d.magFilter]||rn,h.minFilter=Sp[d.minFilter]||bi,h.wrapS=wp[d.wrapS]||Ti,h.wrapT=wp[d.wrapT]||Ti,h.generateMipmaps=!h.isCompressedTexture&&h.minFilter!==on&&h.minFilter!==rn,i.associations.set(h,{textures:e}),h}).catch(function(){return null});return this.textureCache[l]=c,c}loadImageSource(e,t){let n=this,i=this.json,r=this.options;if(this.sourceCache[e]!==void 0)return this.sourceCache[e].then(u=>u.clone());let a=i.images[e],o=self.URL||self.webkitURL,l=a.uri||"",c=!1;if(a.bufferView!==void 0)l=n.getDependency("bufferView",a.bufferView).then(function(u){c=!0;let d=new Blob([u],{type:a.mimeType});return l=o.createObjectURL(d),l});else if(a.uri===void 0)throw new Error("THREE.GLTFLoader: Image "+e+" is missing URI and bufferView");let h=Promise.resolve(l).then(function(u){return new Promise(function(d,p){let x=d;t.isImageBitmapLoader===!0&&(x=function(v){let m=new ln(v);m.needsUpdate=!0,d(m)}),t.load(Qi.resolveURL(u,r.path),x,void 0,p)})}).then(function(u){return c===!0&&o.revokeObjectURL(l),Ui(u,a),u.userData.mimeType=a.mimeType||Z_(a.uri),u}).catch(function(u){throw console.error("THREE.GLTFLoader: Couldn't load texture",l),u});return this.sourceCache[e]=h,h}assignTexture(e,t,n,i){let r=this;return this.getDependency("texture",n.index).then(function(a){if(!a)return null;if(n.texCoord!==void 0&&n.texCoord>0&&(a=a.clone(),a.channel=n.texCoord),r.extensions[Dt.KHR_TEXTURE_TRANSFORM]){let o=n.extensions!==void 0?n.extensions[Dt.KHR_TEXTURE_TRANSFORM]:void 0;if(o){let l=r.associations.get(a);a=r.extensions[Dt.KHR_TEXTURE_TRANSFORM].extendTexture(a,o),r.associations.set(a,l)}}return i!==void 0&&(a.colorSpace=i),e[t]=a,a})}assignFinalMaterial(e){let t=e.geometry,n=e.material,i=t.attributes.tangent===void 0,r=t.attributes.color!==void 0,a=t.attributes.normal===void 0;if(e.isPoints){let o="PointsMaterial:"+n.uuid,l=this.cache.get(o);l||(l=new Rr,On.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,l.sizeAttenuation=!1,this.cache.add(o,l)),n=l}else if(e.isLine){let o="LineBasicMaterial:"+n.uuid,l=this.cache.get(o);l||(l=new Ci,On.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,this.cache.add(o,l)),n=l}if(i||r||a){let o="ClonedMaterial:"+n.uuid+":";i&&(o+="derivative-tangents:"),r&&(o+="vertex-colors:"),a&&(o+="flat-shading:");let l=this.cache.get(o);l||(l=n.clone(),r&&(l.vertexColors=!0),a&&(l.flatShading=!0),i&&(l.normalScale&&(l.normalScale.y*=-1),l.clearcoatNormalScale&&(l.clearcoatNormalScale.y*=-1)),this.cache.add(o,l),this.associations.set(l,this.associations.get(n))),n=l}e.material=n}getMaterialType(){return zn}loadMaterial(e){let t=this,n=this.json,i=this.extensions,r=n.materials[e],a,o={},l=r.extensions||{},c=[];if(l[Dt.KHR_MATERIALS_UNLIT]){let u=i[Dt.KHR_MATERIALS_UNLIT];a=u.getMaterialType(),c.push(u.extendParams(o,r,t))}else{let u=r.pbrMetallicRoughness||{};if(o.color=new Le(1,1,1),o.opacity=1,Array.isArray(u.baseColorFactor)){let d=u.baseColorFactor;o.color.setRGB(d[0],d[1],d[2],Un),o.opacity=d[3]}u.baseColorTexture!==void 0&&c.push(t.assignTexture(o,"map",u.baseColorTexture,sn)),o.metalness=u.metallicFactor!==void 0?u.metallicFactor:1,o.roughness=u.roughnessFactor!==void 0?u.roughnessFactor:1,u.metallicRoughnessTexture!==void 0&&(c.push(t.assignTexture(o,"metalnessMap",u.metallicRoughnessTexture)),c.push(t.assignTexture(o,"roughnessMap",u.metallicRoughnessTexture))),a=this._invokeOne(function(d){return d.getMaterialType&&d.getMaterialType(e)}),c.push(Promise.all(this._invokeAll(function(d){return d.extendMaterialParams&&d.extendMaterialParams(e,o)})))}r.doubleSided===!0&&(o.side=Kt);let h=r.alphaMode||fu.OPAQUE;if(h===fu.BLEND?(o.transparent=!0,o.depthWrite=!1):(o.transparent=!1,h===fu.MASK&&(o.alphaTest=r.alphaCutoff!==void 0?r.alphaCutoff:.5)),r.normalTexture!==void 0&&a!==Vt&&(c.push(t.assignTexture(o,"normalMap",r.normalTexture)),o.normalScale=new Be(1,1),r.normalTexture.scale!==void 0)){let u=r.normalTexture.scale;o.normalScale.set(u,u)}if(r.occlusionTexture!==void 0&&a!==Vt&&(c.push(t.assignTexture(o,"aoMap",r.occlusionTexture)),r.occlusionTexture.strength!==void 0&&(o.aoMapIntensity=r.occlusionTexture.strength)),r.emissiveFactor!==void 0&&a!==Vt){let u=r.emissiveFactor;o.emissive=new Le().setRGB(u[0],u[1],u[2],Un)}return r.emissiveTexture!==void 0&&a!==Vt&&c.push(t.assignTexture(o,"emissiveMap",r.emissiveTexture,sn)),Promise.all(c).then(function(){let u=new a(o);return r.name&&(u.name=r.name),Ui(u,r),t.associations.set(u,{materials:e}),r.extensions&&Ys(i,u,r),u})}createUniqueName(e){let t=$t.sanitizeNodeName(e||"");return t in this.nodeNamesUsed?t+"_"+ ++this.nodeNamesUsed[t]:(this.nodeNamesUsed[t]=0,t)}loadGeometries(e){let t=this,n=this.extensions,i=this.primitiveCache;function r(o){return n[Dt.KHR_DRACO_MESH_COMPRESSION].decodePrimitive(o,t).then(function(l){return Ep(l,o,t)})}let a=[];for(let o=0,l=e.length;o<l;o++){let c=e[o],h=J_(c),u=i[h];if(u)a.push(u.promise);else{let d;c.extensions&&c.extensions[Dt.KHR_DRACO_MESH_COMPRESSION]?d=r(c):d=Ep(new _t,c,t),c.mode===si.TRIANGLE_STRIP?d=d.then(p=>uu(p,no)):c.mode===si.TRIANGLE_FAN&&(d=d.then(p=>uu(p,Gr))),i[h]={primitive:c,promise:d},a.push(d)}}return Promise.all(a)}loadMesh(e){let t=this,n=this.json,i=this.extensions,r=n.meshes[e],a=r.primitives,o=[];for(let l=0,c=a.length;l<c;l++){let h=a[l].material===void 0?j_(this.cache):this.getDependency("material",a[l].material);o.push(h)}return o.push(t.loadGeometries(a)),Promise.all(o).then(async function(l){let c=l.slice(0,l.length-1),h=l[l.length-1],u=[];for(let p=0,x=h.length;p<x;p++){let v=h[p],m=a[p],f,y=c[p];if(m.mode===si.TRIANGLES||m.mode===si.TRIANGLE_STRIP||m.mode===si.TRIANGLE_FAN||m.mode===void 0){let T=r.isSkinnedMesh===!0,g=v.hasAttribute("skinIndex")&&v.hasAttribute("skinWeight");T&&g===!1&&console.warn("THREE.GLTFLoader: Missing skinIndex or skinWeight attributes. Skinning disabled."),f=T&&g?new Ma(v,y):new st(v,y),f.isSkinnedMesh===!0&&f.normalizeSkinWeights()}else if(m.mode===si.LINES)f=new Os(v,y);else if(m.mode===si.LINE_STRIP)f=new ji(v,y);else if(m.mode===si.LINE_LOOP)f=new wa(v,y);else if(m.mode===si.POINTS)f=new yn(v,y);else throw new Error("THREE.GLTFLoader: Primitive mode unsupported: "+m.mode);Object.keys(f.geometry.morphAttributes).length>0&&Y_(f,r),f.name=t.createUniqueName(r.name||"mesh_"+e),Ui(f,r),m.extensions&&Ys(i,f,m),t.assignFinalMaterial(f),u.push(f)}for(let p=0,x=u.length;p<x;p++)t.associations.set(u[p],{meshes:e,primitives:p});if(u.length===1)return r.extensions&&Ys(i,u[0],r),u[0];let d=new vt;r.extensions&&Ys(i,d,r),t.associations.set(d,{meshes:e});for(let p=0,x=u.length;p<x;p++)d.add(u[p]);return d})}loadCamera(e){let t,n=this.json.cameras[e],i=n[n.type];if(!i){console.warn("THREE.GLTFLoader: Missing camera parameters.");return}return n.type==="perspective"?t=new dn(Ft.radToDeg(i.yfov),i.aspectRatio||1,i.znear||1,i.zfar||2e6):n.type==="orthographic"&&(t=new Li(-i.xmag,i.xmag,i.ymag,-i.ymag,i.znear,i.zfar)),n.name&&(t.name=this.createUniqueName(n.name)),Ui(t,n),Promise.resolve(t)}loadSkin(e){let t=this.json.skins[e],n=[];for(let i=0,r=t.joints.length;i<r;i++)n.push(this._loadNodeShallow(t.joints[i]));return t.inverseBindMatrices!==void 0?n.push(this.getDependency("accessor",t.inverseBindMatrices)):n.push(null),Promise.all(n).then(function(i){let r=i.pop(),a=i,o=[],l=[];for(let c=0,h=a.length;c<h;c++){let u=a[c];if(u){o.push(u);let d=new ft;r!==null&&d.fromArray(r.array,c*16),l.push(d)}else console.warn('THREE.GLTFLoader: Joint "%s" could not be found.',t.joints[c])}return new Sa(o,l)})}loadAnimation(e){let t=this.json,n=this,i=t.animations[e],r=i.name?i.name:"animation_"+e,a=[],o=[],l=[],c=[],h=[];for(let u=0,d=i.channels.length;u<d;u++){let p=i.channels[u],x=i.samplers[p.sampler],v=p.target,m=v.node,f=i.parameters!==void 0?i.parameters[x.input]:x.input,y=i.parameters!==void 0?i.parameters[x.output]:x.output;v.node!==void 0&&(a.push(this.getDependency("node",m)),o.push(this.getDependency("accessor",f)),l.push(this.getDependency("accessor",y)),c.push(x),h.push(v))}return Promise.all([Promise.all(a),Promise.all(o),Promise.all(l),Promise.all(c),Promise.all(h)]).then(function(u){let d=u[0],p=u[1],x=u[2],v=u[3],m=u[4],f=[];for(let T=0,g=d.length;T<g;T++){let _=d[T],w=p[T],A=x[T],b=v[T],C=m[T];if(_===void 0)continue;_.updateMatrix&&_.updateMatrix();let S=n._createAnimationTracks(_,w,A,b,C);if(S)for(let P=0;P<S.length;P++)f.push(S[P])}let y=new Bs(r,void 0,f);return Ui(y,i),y})}createNodeMesh(e){let t=this.json,n=this,i=t.nodes[e];return i.mesh===void 0?null:n.getDependency("mesh",i.mesh).then(function(r){let a=n._getNodeRef(n.meshCache,i.mesh,r);return i.weights!==void 0&&a.traverse(function(o){if(o.isMesh)for(let l=0,c=i.weights.length;l<c;l++)o.morphTargetInfluences[l]=i.weights[l]}),a})}loadNode(e){let t=this.json,n=this,i=t.nodes[e],r=n._loadNodeShallow(e),a=[],o=i.children||[];for(let c=0,h=o.length;c<h;c++)a.push(n.getDependency("node",o[c]));let l=i.skin===void 0?Promise.resolve(null):n.getDependency("skin",i.skin);return Promise.all([r,Promise.all(a),l]).then(function(c){let h=c[0],u=c[1],d=c[2];d!==null&&h.traverse(function(p){p.isSkinnedMesh&&p.bind(d,$_)});for(let p=0,x=u.length;p<x;p++)h.add(u[p]);if(h.userData.pivot!==void 0&&u.length>0){let p=h.userData.pivot,x=u[0];h.pivot=new B().fromArray(p),h.position.x-=p[0],h.position.y-=p[1],h.position.z-=p[2],x.position.set(0,0,0),delete h.userData.pivot}return h})}_loadNodeShallow(e){let t=this.json,n=this.extensions,i=this;if(this.nodeCache[e]!==void 0)return this.nodeCache[e];let r=t.nodes[e],a=r.name?i.createUniqueName(r.name):"",o=[],l=i._invokeOne(function(c){return c.createNodeMesh&&c.createNodeMesh(e)});return l&&o.push(l),r.camera!==void 0&&o.push(i.getDependency("camera",r.camera).then(function(c){return i._getNodeRef(i.cameraCache,r.camera,c)})),i._invokeAll(function(c){return c.createNodeAttachment&&c.createNodeAttachment(e)}).forEach(function(c){o.push(c)}),this.nodeCache[e]=Promise.all(o).then(function(c){let h;if(r.isBone===!0?h=new Tr:c.length>1?h=new vt:c.length===1?h=c[0]:h=new Wt,h!==c[0])for(let u=0,d=c.length;u<d;u++)h.add(c[u]);if(r.name&&(h.userData.name=r.name,h.name=a),Ui(h,r),r.extensions&&Ys(n,h,r),r.matrix!==void 0){let u=new ft;u.fromArray(r.matrix),h.applyMatrix4(u)}else r.translation!==void 0&&h.position.fromArray(r.translation),r.rotation!==void 0&&h.quaternion.fromArray(r.rotation),r.scale!==void 0&&h.scale.fromArray(r.scale);if(!i.associations.has(h))i.associations.set(h,{});else if(r.mesh!==void 0&&i.meshCache.refs[r.mesh]>1){let u=i.associations.get(h);i.associations.set(h,{...u})}return i.associations.get(h).nodes=e,h}),this.nodeCache[e]}loadScene(e){let t=this.extensions,n=this.json.scenes[e],i=this,r=new vt;n.name&&(r.name=i.createUniqueName(n.name)),Ui(r,n),n.extensions&&Ys(t,r,n);let a=n.nodes||[],o=[];for(let l=0,c=a.length;l<c;l++)o.push(i.getDependency("node",a[l]));return Promise.all(o).then(function(l){for(let h=0,u=l.length;h<u;h++){let d=l[h];d.parent!==null?r.add(_p(d)):r.add(d)}let c=h=>{let u=new Map;for(let[d,p]of i.associations)(d instanceof On||d instanceof ln)&&u.set(d,p);return h.traverse(d=>{let p=i.associations.get(d);p!=null&&u.set(d,p)}),u};return i.associations=c(r),r})}_createAnimationTracks(e,t,n,i,r){let a=[],o=e.name?e.name:e.uuid,l=[];function c(p){p.morphTargetInfluences&&l.push(p.name?p.name:p.uuid)}_s[r.path]===_s.weights?(c(e),e.isGroup&&e.children.forEach(c)):l.push(o);let h;switch(_s[r.path]){case _s.weights:h=Yi;break;case _s.rotation:h=Ji;break;case _s.translation:case _s.scale:h=ps;break;default:n.itemSize===1?h=Yi:h=ps;break}let u=i.interpolation!==void 0?X_[i.interpolation]:Fs,d=this._getArrayFromAccessor(n);for(let p=0,x=l.length;p<x;p++){let v=new h(l[p]+"."+_s[r.path],t.array,d,u);i.interpolation==="CUBICSPLINE"&&this._createCubicSplineTrackInterpolant(v),a.push(v)}return a}_getArrayFromAccessor(e){let t=e.array;if(e.normalized){let n=zu(t.constructor),i=new Float32Array(t.length);for(let r=0,a=t.length;r<a;r++)i[r]=t[r]*n;t=i}return t}_createCubicSplineTrackInterpolant(e){e.createInterpolant=function(n){let i=this instanceof Ji?Uu:Ec;return new i(this.times,this.values,this.getValueSize()/3,n)},e.createInterpolant.isInterpolantFactoryMethodGLTFCubicSpline=!0}};function Q_(s,e,t){let n=e.attributes,i=new cn;if(n.POSITION!==void 0){let o=t.json.accessors[n.POSITION],l=o.min,c=o.max;if(l!==void 0&&c!==void 0){if(i.set(new B(l[0],l[1],l[2]),new B(c[0],c[1],c[2])),o.normalized){let h=zu(Kr[o.componentType]);i.min.multiplyScalar(h),i.max.multiplyScalar(h)}}else{console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.");return}}else return;let r=e.targets;if(r!==void 0){let o=new B,l=new B;for(let c=0,h=r.length;c<h;c++){let u=r[c];if(u.POSITION!==void 0){let d=t.json.accessors[u.POSITION],p=d.min,x=d.max;if(p!==void 0&&x!==void 0){if(l.setX(Math.max(Math.abs(p[0]),Math.abs(x[0]))),l.setY(Math.max(Math.abs(p[1]),Math.abs(x[1]))),l.setZ(Math.max(Math.abs(p[2]),Math.abs(x[2]))),d.normalized){let v=zu(Kr[d.componentType]);l.multiplyScalar(v)}o.max(l)}else console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.")}}i.expandByVector(o)}s.boundingBox=i;let a=new Cn;i.getCenter(a.center),a.radius=i.min.distanceTo(i.max)/2,s.boundingSphere=a}function Ep(s,e,t){let n=e.attributes,i=[];function r(a,o){return t.getDependency("accessor",a).then(function(l){s.setAttribute(o,l)})}for(let a in n){let o=Ou[a]||a.toLowerCase();o in s.attributes||i.push(r(n[a],o))}if(e.indices!==void 0&&!s.index){let a=t.getDependency("accessor",e.indices).then(function(o){s.setIndex(o)});i.push(a)}return Tt.workingColorSpace!==Un&&"COLOR_0"in n&&console.warn(`THREE.GLTFLoader: Converting vertex colors from "srgb-linear" to "${Tt.workingColorSpace}" not supported.`),Ui(s,e),Q_(s,e,t),Promise.all(i).then(function(){return e.targets!==void 0?K_(s,e.targets,t):s})}var Ap={type:"change"},Hu={type:"start"},Cp={type:"end"},Tc=new Ri,Rp=new Xn,ey=Math.cos(70*Ft.DEG2RAD),bn=new B,Vn=2*Math.PI,Yt={NONE:-1,ROTATE:0,DOLLY:1,PAN:2,TOUCH_ROTATE:3,TOUCH_PAN:4,TOUCH_DOLLY_PAN:5,TOUCH_DOLLY_ROTATE:6},Bu=1e-6,Ac=class extends Ga{constructor(e,t=null){super(e,t),this.state=Yt.NONE,this.target=new B,this.cursor=new B,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.minTargetRadius=0,this.maxTargetRadius=1/0,this.minPolarAngle=0,this.maxPolarAngle=Math.PI,this.minAzimuthAngle=-1/0,this.maxAzimuthAngle=1/0,this.enableDamping=!1,this.dampingFactor=.05,this.enableZoom=!0,this.zoomSpeed=1,this.enableRotate=!0,this.rotateSpeed=1,this.keyRotateSpeed=1,this.enablePan=!0,this.panSpeed=1,this.screenSpacePanning=!0,this.keyPanSpeed=7,this.zoomToCursor=!1,this.autoRotate=!1,this.autoRotateSpeed=2,this.keys={LEFT:"ArrowLeft",UP:"ArrowUp",RIGHT:"ArrowRight",BOTTOM:"ArrowDown"},this.mouseButtons={LEFT:ms.ROTATE,MIDDLE:ms.DOLLY,RIGHT:ms.PAN},this.touches={ONE:gs.ROTATE,TWO:gs.DOLLY_PAN},this.target0=this.target.clone(),this.position0=this.object.position.clone(),this.zoom0=this.object.zoom,this._cursorStyle="auto",this._domElementKeyEvents=null,this._lastPosition=new B,this._lastQuaternion=new Qt,this._lastTargetPosition=new B,this._quat=new Qt().setFromUnitVectors(e.up,new B(0,1,0)),this._quatInverse=this._quat.clone().invert(),this._spherical=new Ur,this._sphericalDelta=new Ur,this._scale=1,this._panOffset=new B,this._rotateStart=new Be,this._rotateEnd=new Be,this._rotateDelta=new Be,this._panStart=new Be,this._panEnd=new Be,this._panDelta=new Be,this._dollyStart=new Be,this._dollyEnd=new Be,this._dollyDelta=new Be,this._dollyDirection=new B,this._mouse=new Be,this._performCursorZoom=!1,this._pointers=[],this._pointerPositions={},this._controlActive=!1,this._onPointerMove=ny.bind(this),this._onPointerDown=ty.bind(this),this._onPointerUp=iy.bind(this),this._onContextMenu=hy.bind(this),this._onMouseWheel=ay.bind(this),this._onKeyDown=oy.bind(this),this._onTouchStart=ly.bind(this),this._onTouchMove=cy.bind(this),this._onMouseDown=sy.bind(this),this._onMouseMove=ry.bind(this),this._interceptControlDown=uy.bind(this),this._interceptControlUp=dy.bind(this),this.domElement!==null&&this.connect(this.domElement),this.update()}set cursorStyle(e){this._cursorStyle=e,e==="grab"?this.domElement.style.cursor="grab":this.domElement.style.cursor="auto"}get cursorStyle(){return this._cursorStyle}connect(e){super.connect(e),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerUp),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.getRootNode().addEventListener("keydown",this._interceptControlDown,{passive:!0,capture:!0}),this.domElement.style.touchAction="none"}disconnect(){this.state=Yt.NONE,this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerUp),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.stopListenToKeyEvents();let e=this.domElement.getRootNode();e.removeEventListener("keydown",this._interceptControlDown,{capture:!0}),e.removeEventListener("keyup",this._interceptControlUp,{capture:!0}),this._controlActive=!1,this._pointers.length=0,this._pointerPositions={},this.domElement.style.touchAction="",this.domElement.style.cursor="auto"}dispose(){this.disconnect()}getPolarAngle(){return this._spherical.phi}getAzimuthalAngle(){return this._spherical.theta}getDistance(){return this.object.position.distanceTo(this.target)}listenToKeyEvents(e){e.addEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=e}stopListenToKeyEvents(){this._domElementKeyEvents!==null&&(this._domElementKeyEvents.removeEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=null)}saveState(){this.target0.copy(this.target),this.position0.copy(this.object.position),this.zoom0=this.object.zoom}reset(){this.target.copy(this.target0),this.object.position.copy(this.position0),this.object.zoom=this.zoom0,this.object.updateProjectionMatrix(),this.dispatchEvent(Ap),this.update(),this.state=Yt.NONE}pan(e,t){this._pan(e,t),this.update()}dollyIn(e){this._dollyIn(e),this.update()}dollyOut(e){this._dollyOut(e),this.update()}rotateLeft(e){this._rotateLeft(e),this.update()}rotateUp(e){this._rotateUp(e),this.update()}update(e=null){let t=this.object.position;bn.copy(t).sub(this.target),bn.applyQuaternion(this._quat),this._spherical.setFromVector3(bn),this.autoRotate&&this.state===Yt.NONE&&this._rotateLeft(this._getAutoRotationAngle(e)),this.enableDamping?(this._spherical.theta+=this._sphericalDelta.theta*this.dampingFactor,this._spherical.phi+=this._sphericalDelta.phi*this.dampingFactor):(this._spherical.theta+=this._sphericalDelta.theta,this._spherical.phi+=this._sphericalDelta.phi);let n=this.minAzimuthAngle,i=this.maxAzimuthAngle;isFinite(n)&&isFinite(i)&&(n<-Math.PI?n+=Vn:n>Math.PI&&(n-=Vn),i<-Math.PI?i+=Vn:i>Math.PI&&(i-=Vn),n<=i?this._spherical.theta=Math.max(n,Math.min(i,this._spherical.theta)):this._spherical.theta=this._spherical.theta>(n+i)/2?Math.max(n,this._spherical.theta):Math.min(i,this._spherical.theta)),this._spherical.phi=Math.max(this.minPolarAngle,Math.min(this.maxPolarAngle,this._spherical.phi)),this._spherical.makeSafe(),this.enableDamping===!0?this.target.addScaledVector(this._panOffset,this.dampingFactor):this.target.add(this._panOffset),this.target.sub(this.cursor),this.target.clampLength(this.minTargetRadius,this.maxTargetRadius),this.target.add(this.cursor);let r=!1;if(this.zoomToCursor&&this._performCursorZoom||this.object.isOrthographicCamera)this._spherical.radius=this._clampDistance(this._spherical.radius);else{let a=this._spherical.radius;this._spherical.radius=this._clampDistance(this._spherical.radius*this._scale),r=a!=this._spherical.radius}if(bn.setFromSpherical(this._spherical),bn.applyQuaternion(this._quatInverse),t.copy(this.target).add(bn),this.object.lookAt(this.target),this.enableDamping===!0?(this._sphericalDelta.theta*=1-this.dampingFactor,this._sphericalDelta.phi*=1-this.dampingFactor,this._panOffset.multiplyScalar(1-this.dampingFactor)):(this._sphericalDelta.set(0,0,0),this._panOffset.set(0,0,0)),this.zoomToCursor&&this._performCursorZoom){let a=null;if(this.object.isPerspectiveCamera){let o=bn.length();a=this._clampDistance(o*this._scale);let l=o-a;this.object.position.addScaledVector(this._dollyDirection,l),this.object.updateMatrixWorld(),r=!!l}else if(this.object.isOrthographicCamera){let o=new B(this._mouse.x,this._mouse.y,0);o.unproject(this.object);let l=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),this.object.updateProjectionMatrix(),r=l!==this.object.zoom;let c=new B(this._mouse.x,this._mouse.y,0);c.unproject(this.object),this.object.position.sub(c).add(o),this.object.updateMatrixWorld(),a=bn.length()}else console.warn("WARNING: OrbitControls.js encountered an unknown camera type - zoom to cursor disabled."),this.zoomToCursor=!1;a!==null&&(this.screenSpacePanning?this.target.set(0,0,-1).transformDirection(this.object.matrix).multiplyScalar(a).add(this.object.position):(Tc.origin.copy(this.object.position),Tc.direction.set(0,0,-1).transformDirection(this.object.matrix),Math.abs(this.object.up.dot(Tc.direction))<ey?this.object.lookAt(this.target):(Rp.setFromNormalAndCoplanarPoint(this.object.up,this.target),Tc.intersectPlane(Rp,this.target))))}else if(this.object.isOrthographicCamera){let a=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),a!==this.object.zoom&&(this.object.updateProjectionMatrix(),r=!0)}return this._scale=1,this._performCursorZoom=!1,r||this._lastPosition.distanceToSquared(this.object.position)>Bu||8*(1-this._lastQuaternion.dot(this.object.quaternion))>Bu||this._lastTargetPosition.distanceToSquared(this.target)>Bu?(this.dispatchEvent(Ap),this._lastPosition.copy(this.object.position),this._lastQuaternion.copy(this.object.quaternion),this._lastTargetPosition.copy(this.target),!0):!1}_getAutoRotationAngle(e){return e!==null?Vn/60*this.autoRotateSpeed*e:Vn/60/60*this.autoRotateSpeed}_getZoomScale(e){let t=Math.abs(e*.01);return Math.pow(.95,this.zoomSpeed*t)}_rotateLeft(e){this._sphericalDelta.theta-=e}_rotateUp(e){this._sphericalDelta.phi-=e}_panLeft(e,t){bn.setFromMatrixColumn(t,0),bn.multiplyScalar(-e),this._panOffset.add(bn)}_panUp(e,t){this.screenSpacePanning===!0?bn.setFromMatrixColumn(t,1):(bn.setFromMatrixColumn(t,0),bn.crossVectors(this.object.up,bn)),bn.multiplyScalar(e),this._panOffset.add(bn)}_pan(e,t){let n=this.domElement;if(this.object.isPerspectiveCamera){let i=this.object.position;bn.copy(i).sub(this.target);let r=bn.length();r*=Math.tan(this.object.fov/2*Math.PI/180),this._panLeft(2*e*r/n.clientHeight,this.object.matrix),this._panUp(2*t*r/n.clientHeight,this.object.matrix)}else this.object.isOrthographicCamera?(this._panLeft(e*(this.object.right-this.object.left)/this.object.zoom/n.clientWidth,this.object.matrix),this._panUp(t*(this.object.top-this.object.bottom)/this.object.zoom/n.clientHeight,this.object.matrix)):(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - pan disabled."),this.enablePan=!1)}_dollyOut(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale/=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_dollyIn(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale*=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_updateZoomParameters(e,t){if(!this.zoomToCursor)return;this._performCursorZoom=!0;let n=this.domElement.getBoundingClientRect(),i=e-n.left,r=t-n.top,a=n.width,o=n.height;this._mouse.x=i/a*2-1,this._mouse.y=-(r/o)*2+1,this._dollyDirection.set(this._mouse.x,this._mouse.y,1).unproject(this.object).sub(this.object.position).normalize()}_clampDistance(e){return Math.max(this.minDistance,Math.min(this.maxDistance,e))}_handleMouseDownRotate(e){this._rotateStart.set(e.clientX,e.clientY)}_handleMouseDownDolly(e){this._updateZoomParameters(e.clientX,e.clientX),this._dollyStart.set(e.clientX,e.clientY)}_handleMouseDownPan(e){this._panStart.set(e.clientX,e.clientY)}_handleMouseMoveRotate(e){this._rotateEnd.set(e.clientX,e.clientY),this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Vn*this._rotateDelta.x/t.clientHeight),this._rotateUp(Vn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd),this.update()}_handleMouseMoveDolly(e){this._dollyEnd.set(e.clientX,e.clientY),this._dollyDelta.subVectors(this._dollyEnd,this._dollyStart),this._dollyDelta.y>0?this._dollyOut(this._getZoomScale(this._dollyDelta.y)):this._dollyDelta.y<0&&this._dollyIn(this._getZoomScale(this._dollyDelta.y)),this._dollyStart.copy(this._dollyEnd),this.update()}_handleMouseMovePan(e){this._panEnd.set(e.clientX,e.clientY),this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd),this.update()}_handleMouseWheel(e){this._updateZoomParameters(e.clientX,e.clientY),e.deltaY<0?this._dollyIn(this._getZoomScale(e.deltaY)):e.deltaY>0&&this._dollyOut(this._getZoomScale(e.deltaY)),this.update()}_handleKeyDown(e){let t=!1;switch(e.code){case this.keys.UP:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(Vn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,this.keyPanSpeed),t=!0;break;case this.keys.BOTTOM:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(-Vn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,-this.keyPanSpeed),t=!0;break;case this.keys.LEFT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(Vn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(this.keyPanSpeed,0),t=!0;break;case this.keys.RIGHT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(-Vn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(-this.keyPanSpeed,0),t=!0;break}t&&(e.preventDefault(),this.update())}_handleTouchStartRotate(e){if(this._pointers.length===1)this._rotateStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._rotateStart.set(n,i)}}_handleTouchStartPan(e){if(this._pointers.length===1)this._panStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panStart.set(n,i)}}_handleTouchStartDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyStart.set(0,r)}_handleTouchStartDollyPan(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enablePan&&this._handleTouchStartPan(e)}_handleTouchStartDollyRotate(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enableRotate&&this._handleTouchStartRotate(e)}_handleTouchMoveRotate(e){if(this._pointers.length==1)this._rotateEnd.set(e.pageX,e.pageY);else{let n=this._getSecondPointerPosition(e),i=.5*(e.pageX+n.x),r=.5*(e.pageY+n.y);this._rotateEnd.set(i,r)}this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Vn*this._rotateDelta.x/t.clientHeight),this._rotateUp(Vn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd)}_handleTouchMovePan(e){if(this._pointers.length===1)this._panEnd.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panEnd.set(n,i)}this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd)}_handleTouchMoveDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyEnd.set(0,r),this._dollyDelta.set(0,Math.pow(this._dollyEnd.y/this._dollyStart.y,this.zoomSpeed)),this._dollyOut(this._dollyDelta.y),this._dollyStart.copy(this._dollyEnd);let a=(e.pageX+t.x)*.5,o=(e.pageY+t.y)*.5;this._updateZoomParameters(a,o)}_handleTouchMoveDollyPan(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enablePan&&this._handleTouchMovePan(e)}_handleTouchMoveDollyRotate(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enableRotate&&this._handleTouchMoveRotate(e)}_addPointer(e){this._pointers.push(e.pointerId)}_removePointer(e){delete this._pointerPositions[e.pointerId];for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId){this._pointers.splice(t,1);return}}_isTrackingPointer(e){for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId)return!0;return!1}_trackPointer(e){let t=this._pointerPositions[e.pointerId];t===void 0&&(t=new Be,this._pointerPositions[e.pointerId]=t),t.set(e.pageX,e.pageY)}_getSecondPointerPosition(e){let t=e.pointerId===this._pointers[0]?this._pointers[1]:this._pointers[0];return this._pointerPositions[t]}_customWheelEvent(e){let t=e.deltaMode,n={clientX:e.clientX,clientY:e.clientY,deltaY:e.deltaY};switch(t){case 1:n.deltaY*=16;break;case 2:n.deltaY*=100;break}return e.ctrlKey&&!this._controlActive&&(n.deltaY*=10),n}};function ty(s){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(s.pointerId),this.domElement.ownerDocument.addEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.addEventListener("pointerup",this._onPointerUp)),!this._isTrackingPointer(s)&&(this._addPointer(s),s.pointerType==="touch"?this._onTouchStart(s):this._onMouseDown(s),this._cursorStyle==="grab"&&(this.domElement.style.cursor="grabbing")))}function ny(s){this.enabled!==!1&&(s.pointerType==="touch"?this._onTouchMove(s):this._onMouseMove(s))}function iy(s){switch(this._removePointer(s),this._pointers.length){case 0:this.domElement.releasePointerCapture(s.pointerId),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.dispatchEvent(Cp),this.state=Yt.NONE,this._cursorStyle==="grab"&&(this.domElement.style.cursor="grab");break;case 1:let e=this._pointers[0],t=this._pointerPositions[e];this._onTouchStart({pointerId:e,pageX:t.x,pageY:t.y});break}}function sy(s){let e;switch(s.button){case 0:e=this.mouseButtons.LEFT;break;case 1:e=this.mouseButtons.MIDDLE;break;case 2:e=this.mouseButtons.RIGHT;break;default:e=-1}switch(e){case ms.DOLLY:if(this.enableZoom===!1)return;this._handleMouseDownDolly(s),this.state=Yt.DOLLY;break;case ms.ROTATE:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Yt.PAN}else{if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Yt.ROTATE}break;case ms.PAN:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Yt.ROTATE}else{if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Yt.PAN}break;default:this.state=Yt.NONE}this.state!==Yt.NONE&&this.dispatchEvent(Hu)}function ry(s){switch(this.state){case Yt.ROTATE:if(this.enableRotate===!1)return;this._handleMouseMoveRotate(s);break;case Yt.DOLLY:if(this.enableZoom===!1)return;this._handleMouseMoveDolly(s);break;case Yt.PAN:if(this.enablePan===!1)return;this._handleMouseMovePan(s);break}}function ay(s){this.enabled===!1||this.enableZoom===!1||this.state!==Yt.NONE||(s.preventDefault(),this.dispatchEvent(Hu),this._handleMouseWheel(this._customWheelEvent(s)),this.dispatchEvent(Cp))}function oy(s){this.enabled!==!1&&this._handleKeyDown(s)}function ly(s){switch(this._trackPointer(s),this._pointers.length){case 1:switch(this.touches.ONE){case gs.ROTATE:if(this.enableRotate===!1)return;this._handleTouchStartRotate(s),this.state=Yt.TOUCH_ROTATE;break;case gs.PAN:if(this.enablePan===!1)return;this._handleTouchStartPan(s),this.state=Yt.TOUCH_PAN;break;default:this.state=Yt.NONE}break;case 2:switch(this.touches.TWO){case gs.DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchStartDollyPan(s),this.state=Yt.TOUCH_DOLLY_PAN;break;case gs.DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchStartDollyRotate(s),this.state=Yt.TOUCH_DOLLY_ROTATE;break;default:this.state=Yt.NONE}break;default:this.state=Yt.NONE}this.state!==Yt.NONE&&this.dispatchEvent(Hu)}function cy(s){switch(this._trackPointer(s),this.state){case Yt.TOUCH_ROTATE:if(this.enableRotate===!1)return;this._handleTouchMoveRotate(s),this.update();break;case Yt.TOUCH_PAN:if(this.enablePan===!1)return;this._handleTouchMovePan(s),this.update();break;case Yt.TOUCH_DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchMoveDollyPan(s),this.update();break;case Yt.TOUCH_DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchMoveDollyRotate(s),this.update();break;default:this.state=Yt.NONE}}function hy(s){this.enabled!==!1&&s.preventDefault()}function uy(s){s.key==="Control"&&(this._controlActive=!0,this.domElement.getRootNode().addEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}function dy(s){s.key==="Control"&&(this._controlActive=!1,this.domElement.getRootNode().removeEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}var Yr=(function(){var s="b9H79Tebbbe8Fv9Gbb9Gvuuuuueu9Giuuub9Geueu9Giuuueuixkbeeeddddillviebeoweuecj:Gdkr;Neqo9TW9T9VV95dbH9F9F939H79T9F9J9H229F9Jt9VV7bb8A9TW79O9V9Wt9F9KW9J9V9KW9wWVtW949c919M9MWVbeY9TW79O9V9Wt9F9KW9J9V9KW69U9KW949c919M9MWVbdE9TW79O9V9Wt9F9KW9J9V9KW69U9KW949tWG91W9U9JWbiL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9p9JtblK9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9r919HtbvL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVT949WboY9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVJ9V29VVbrl79IV9Rbwq:VZkdbk:XYi5ud9:du8Jjjjjbcj;kb9Rgv8Kjjjjbc9:hodnalTmbcuhoaiRbbgrc;WeGc:Ge9hmbarcsGgwce0mbc9:hoalcufadcd4cbawEgDadfgrcKcaawEgqaraq0Egk6mbaicefhxcj;abad9Uc;WFbGcjdadca0EhmaialfgPar9Rgoadfhsavaoadz:jjjjbgzceVhHcbhOdndninaeaO9nmeaPax9RaD6mdamaeaO9RaOamfgoae6EgAcsfglc9WGhCabaOad2fhXaAcethQaxaDfhiaOaeaoaeao6E9RhLalcl4cifcd4hKazcj;cbfaAfhYcbh8AazcjdfhEaHh3incbh5dnawTmbaxa8Acd4fRbbh5kcbh8Eazcj;cbfhqinaih8Fdndndndna5a8Ecet4ciGgoc9:fPdebdkaPa8F9RaA6mrazcj;cbfa8EaA2fa8FaAz:jjjjb8Aa8FaAfhixdkazcj;cbfa8EaA2fcbaAz:kjjjb8Aa8FhixekaPa8F9RaK6mva8FaKfhidnaCTmbaPai9RcK6mbaocdtc:q:G:cjbfcj:G:cjbawEhaczhrcbhlinargoc9Wfghaqfhrdndndndndndnaaa8Fahco4fRbbalcoG4ciGcdtfydbPDbedvivvvlvkar9cb83bwar9cb83bbxlkarcbaiRbdai8Xbb9c:c:qj:bw9:9c:q;c1:I1e:d9c:b:c:e1z9:gg9cjjjjjz:dg8J9qE86bbaqaofgrcGfcbaicdfa8J9c8N1:NfghRbbag9cjjjjjw:dg8J9qE86bbarcVfcbaha8J9c8M1:NfghRbbag9cjjjjjl:dg8J9qE86bbarc7fcbaha8J9c8L1:NfghRbbag9cjjjjjd:dg8J9qE86bbarctfcbaha8J9c8K1:NfghRbbag9cjjjjje:dg8J9qE86bbarc91fcbaha8J9c8J1:NfghRbbag9cjjjj;ab:dg8J9qE86bbarc4fcbaha8J9cg1:NfghRbbag9cjjjja:dg8J9qE86bbarc93fcbaha8J9ch1:NfghRbbag9cjjjjz:dgg9qE86bbarc94fcbahag9ca1:NfghRbbai8Xbe9c:c:qj:bw9:9c:q;c1:I1e:d9c:b:c:e1z9:gg9cjjjjjz:dg8J9qE86bbarc95fcbaha8J9c8N1:NfgiRbbag9cjjjjjw:dg8J9qE86bbarc96fcbaia8J9c8M1:NfgiRbbag9cjjjjjl:dg8J9qE86bbarc97fcbaia8J9c8L1:NfgiRbbag9cjjjjjd:dg8J9qE86bbarc98fcbaia8J9c8K1:NfgiRbbag9cjjjjje:dg8J9qE86bbarc99fcbaia8J9c8J1:NfgiRbbag9cjjjj;ab:dg8J9qE86bbarc9:fcbaia8J9cg1:NfgiRbbag9cjjjja:dg8J9qE86bbarcufcbaia8J9ch1:NfgiRbbag9cjjjjz:dgg9qE86bbaiag9ca1:NfhixikaraiRblaiRbbghco4g8Ka8KciSg8KE86bbaqaofgrcGfaiclfa8Kfg8KRbbahcl4ciGg8La8LciSg8LE86bbarcVfa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc7fa8Ka8Lfg8KRbbahciGghahciSghE86bbarctfa8Kahfg8KRbbaiRbeghco4g8La8LciSg8LE86bbarc91fa8Ka8Lfg8KRbbahcl4ciGg8La8LciSg8LE86bbarc4fa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc93fa8Ka8Lfg8KRbbahciGghahciSghE86bbarc94fa8Kahfg8KRbbaiRbdghco4g8La8LciSg8LE86bbarc95fa8Ka8Lfg8KRbbahcl4ciGg8La8LciSg8LE86bbarc96fa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc97fa8Ka8Lfg8KRbbahciGghahciSghE86bbarc98fa8KahfghRbbaiRbigico4g8Ka8KciSg8KE86bbarc99faha8KfghRbbaicl4ciGg8Ka8KciSg8KE86bbarc9:faha8KfghRbbaicd4ciGg8Ka8KciSg8KE86bbarcufaha8KfgrRbbaiciGgiaiciSgiE86bbaraifhixdkaraiRbwaiRbbghcl4g8Ka8KcsSg8KE86bbaqaofgrcGfaicwfa8Kfg8KRbbahcsGghahcsSghE86bbarcVfa8KahfghRbbaiRbeg8Kcl4g8La8LcsSg8LE86bbarc7faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarctfaha8KfghRbbaiRbdg8Kcl4g8La8LcsSg8LE86bbarc91faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc4faha8KfghRbbaiRbig8Kcl4g8La8LcsSg8LE86bbarc93faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc94faha8KfghRbbaiRblg8Kcl4g8La8LcsSg8LE86bbarc95faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc96faha8KfghRbbaiRbvg8Kcl4g8La8LcsSg8LE86bbarc97faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc98faha8KfghRbbaiRbog8Kcl4g8La8LcsSg8LE86bbarc99faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc9:faha8KfghRbbaiRbrgicl4g8Ka8KcsSg8KE86bbarcufaha8KfgrRbbaicsGgiaicsSgiE86bbaraifhixekarai8Pbw83bwarai8Pbb83bbaiczfhikdnaoaC9pmbalcdfhlaoczfhraPai9RcL0mekkaoaC6moaimexokaCmva8FTmvkaqaAfhqa8Ecefg8Ecl9hmbkdndndndnawTmbasa8Acd4fRbbgociGPlbedrbkaATmdaza8Afh8Fazcj;cbfhhcbh8EaEhaina8FRbbhraahocbhlinaoahalfRbbgqce4cbaqceG9R7arfgr86bbaoadfhoaAalcefgl9hmbkaacefhaa8Fcefh8FahaAfhha8Ecefg8Ecl9hmbxikkaATmeaza8Afhaazcj;cbfhhcbhoceh8EaYh8FinaEaofhlaa8Vbbhrcbhoinala8FaofRbbcwtahaofRbbgqVc;:FiGce4cbaqceG9R7arfgr87bbaladfhlaLaocefgofmbka8FaQfh8FcdhoaacdfhaahaQfhha8EceGhlcbh8EalmbxdkkaATmbaocl4h8Eaza8AfRbbhqcwhoa3hlinalRbbaotaqVhqalcefhlaocwfgoca9hmbkcbhhaEh8FaYhainazcj;cbfahfRbbhrcwhoaahlinalRbbaotarVhralaAfhlaocwfgoca9hmbkara8E94aq7hqcbhoa8Fhlinalaqao486bbalcefhlaocwfgoca9hmbka8Fadfh8FaacefhaahcefghaA9hmbkkaEclfhEa3clfh3a8Aclfg8Aad6mbkaXazcjdfaAad2z:jjjjb8AazazcjdfaAcufad2fadz:jjjjb8AaAaOfhOaihxaimbkc9:hoxdkcbc99aPax9RakSEhoxekc9:hokavcj;kbf8Kjjjjbaok:ysezu8Jjjjjbc;ae9Rgv8Kjjjjbc9:hodnalaeci9UgrcHf6mbcuhoaiRbbgwc;WeGc;Ge9hmbawcsGgDce0mbavc;abfcFecjez:kjjjb8Aav9cu83iUav9cu83i8Wav9cu83iyav9cu83iaav9cu83iKav9cu83izav9cu83iwav9cu83ibaialfc9WfhqaicefgwarfhldnaeTmbcmcsaDceSEhkcbhxcbhmcbhrcbhicbhoindnalaq9nmbc9:hoxikdndnawRbbgDc;Ve0mbavc;abfaoaDcu7gPcl4fcsGcitfgsydlhzasydbhHdndnaDcsGgsak9pmbavaiaPfcsGcdtfydbaxasEhDaxasTgOfhxxekdndnascsSmbcehOasc987asamffcefhDxekalcefhDal8SbbgscFeGhPdndnascu9mmbaDhlxekalcvfhlaPcFbGhPcrhsdninaD8SbbgOcFbGastaPVhPaOcu9kmeaDcefhDascrfgsc8J9hmbxdkkaDcefhlkcehOaPce4cbaPceG9R7amfhDkaDhmkavc;abfaocitfgsaDBdbasazBdlavaicdtfaDBdbavc;abfaocefcsGcitfgsaHBdbasaDBdlaocdfhoaOaifhidnadcd9hmbabarcetfgsaH87ebasclfaD87ebascdfaz87ebxdkabarcdtfgsaHBdbascwfaDBdbasclfazBdbxekdnaDcpe0mbavaiaqaDcsGfRbbgscl4gP9RcsGcdtfydbaxcefgOaPEhDavaias9RcsGcdtfydbaOaPTgzfgOascsGgPEhsaPThPdndnadcd9hmbabarcetfgHax87ebaHclfas87ebaHcdfaD87ebxekabarcdtfgHaxBdbaHcwfasBdbaHclfaDBdbkavaicdtfaxBdbavc;abfaocitfgHaDBdbaHaxBdlavaicefgicsGcdtfaDBdbavc;abfaocefcsGcitfgHasBdbaHaDBdlavaiazfgicsGcdtfasBdbavc;abfaocdfcsGcitfgDaxBdbaDasBdlaocifhoaiaPfhiaOaPfhxxekaxcbalRbbgsEgHaDc;:eSgDfhOascsGhAdndnascl4gCmbaOcefhzxekaOhzavaiaC9RcsGcdtfydbhOkdndnaAmbazcefhxxekazhxavaias9RcsGcdtfydbhzkdndnaDTmbalcefhDxekalcdfhDal8SbegPcFeGhsdnaPcu9kmbalcofhHascFbGhscrhldninaD8SbbgPcFbGaltasVhsaPcu9kmeaDcefhDalcrfglc8J9hmbkaHhDxekaDcefhDkasce4cbasceG9R7amfgmhHkdndnaCcsSmbaDhsxekaDcefhsaD8SbbglcFeGhPdnalcu9kmbaDcvfhOaPcFbGhPcrhldninas8SbbgDcFbGaltaPVhPaDcu9kmeascefhsalcrfglc8J9hmbkaOhsxekascefhskaPce4cbaPceG9R7amfgmhOkdndnaAcsSmbashlxekascefhlas8SbbgDcFeGhPdnaDcu9kmbascvfhzaPcFbGhPcrhDdninal8SbbgscFbGaDtaPVhPascu9kmealcefhlaDcrfgDc8J9hmbkazhlxekalcefhlkaPce4cbaPceG9R7amfgmhzkdndnadcd9hmbabarcetfgDaH87ebaDclfaz87ebaDcdfaO87ebxekabarcdtfgDaHBdbaDcwfazBdbaDclfaOBdbkavc;abfaocitfgDaOBdbaDaHBdlavaicdtfaHBdbavc;abfaocefcsGcitfgDazBdbaDaOBdlavaicefgicsGcdtfaOBdbavc;abfaocdfcsGcitfgDaHBdbaDazBdlavaiaCTaCcsSVfgicsGcdtfazBdbaiaATaAcsSVfhiaocifhokawcefhwaocsGhoaicsGhiarcifgrae6mbkkcbc99alaqSEhokavc;aef8Kjjjjbaok:clevu8Jjjjjbcz9Rhvdnalaecvf9pmbc9:skdnaiRbbc;:eGc;qeSmbcuskav9cb83iwaicefhoaialfc98fhrdnaeTmbdnadcdSmbcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcdtfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgiBdbalaiBdbawcefgwae9hmbxdkkcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcetfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgi87ebalaiBdbawcefgwae9hmbkkcbc99aoarSEk:Lvoeue99dud99eud99dndnadcl9hmbaeTmeindndnabcdfgd8Sbb:Yab8Sbbgi:Ygl:l:tabcefgv8Sbbgo:Ygr:l:tgwJbb;:9cawawNJbbbbawawJbbbb9GgDEgq:mgkaqaicb9iEalMgwawNakaqaocb9iEarMgqaqNMM:r:vglNJbbbZJbbb:;aDEMgr:lJbbb9p9DTmbar:Ohixekcjjjj94hikadai86bbdndnaqalNJbbbZJbbb:;aqJbbbb9GEMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkavad86bbdndnawalNJbbbZJbbb:;awJbbbb9GEMgw:lJbbb9p9DTmbaw:Ohdxekcjjjj94hdkabad86bbabclfhbaecufgembxdkkaeTmbindndnabclfgd8Ueb:Yab8Uebgi:Ygl:l:tabcdfgv8Uebgo:Ygr:l:tgwJb;:FSawawNJbbbbawawJbbbb9GgDEgq:mgkaqaicb9iEalMgwawNakaqaocb9iEarMgqaqNMM:r:vglNJbbbZJbbb:;aDEMgr:lJbbb9p9DTmbar:Ohixekcjjjj94hikadai87ebdndnaqalNJbbbZJbbb:;aqJbbbb9GEMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkavad87ebdndnawalNJbbbZJbbb:;awJbbbb9GEMgw:lJbbb9p9DTmbaw:Ohdxekcjjjj94hdkabad87ebabcwfhbaecufgembkkk:4ioiue99dud99dud99dnaeTmbcbhiabhlindndnal8Uebgv:YgoJ:ji:1Salcof8UebgrciVgw:Y:vgDNJbbbZJbbb:;avcu9kEMgq:lJbbb9p9DTmbaq:Ohkxekcjjjj94hkkalclf8Uebhvalcdf8UebhxalarcefciGcetfak87ebdndnax:YgqaDNJbbbZJbbb:;axcu9kEMgm:lJbbb9p9DTmbam:Ohxxekcjjjj94hxkabaiarciGgkfcd7cetfax87ebdndnav:YgmaDNJbbbZJbbb:;avcu9kEMgP:lJbbb9p9DTmbaP:Ohvxekcjjjj94hvkalarcufciGcetfav87ebdndnawaw2:ZgPaPMaoaoN:taqaqN:tamamN:tgoJbbbbaoJbbbb9GE:raDNJbbbZMgD:lJbbb9p9DTmbaD:Ohrxekcjjjj94hrkalakcetfar87ebalcwfhlaiclfhiaecufgembkkk9mbdnadcd4ae2gdTmbinababydbgecwtcw91:Yaece91cjjj98Gcjjj;8if::NUdbabclfhbadcufgdmbkkk:Tvirud99eudndnadcl9hmbaeTmeindndnabRbbgiabcefgl8Sbbgvabcdfgo8Sbbgrf9R:YJbbuJabcifgwRbbgdce4adVgDcd4aDVgDcl4aDVgD:Z:vgqNJbbbZMgk:lJbbb9p9DTmbak:Ohxxekcjjjj94hxkaoax86bbdndnaraif:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohoxekcjjjj94hokalao86bbdndnavaifar9R:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohixekcjjjj94hikabai86bbdndnaDadcetGadceGV:ZaqNJbbbZMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkawad86bbabclfhbaecufgembxdkkaeTmbindndnab8Vebgiabcdfgl8Uebgvabclfgo8Uebgrf9R:YJbFu9habcofgw8Vebgdce4adVgDcd4aDVgDcl4aDVgDcw4aDVgD:Z:vgqNJbbbZMgk:lJbbb9p9DTmbak:Ohxxekcjjjj94hxkaoax87ebdndnaraif:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohoxekcjjjj94hokalao87ebdndnavaifar9R:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohixekcjjjj94hikabai87ebdndnaDadcetGadceGV:ZaqNJbbbZMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkawad87ebabcwfhbaecufgembkkk9teiucbcbyd:K:G:cjbgeabcifc98GfgbBd:K:G:cjbdndnabZbcztgd9nmbcuhiabad9RcFFifcz4nbcuSmekaehikaik;LeeeudndnaeabVciGTmbabhixekdndnadcz9pmbabhixekabhiinaiaeydbBdbaiclfaeclfydbBdbaicwfaecwfydbBdbaicxfaecxfydbBdbaeczfheaiczfhiadc9Wfgdcs0mbkkadcl6mbinaiaeydbBdbaeclfheaiclfhiadc98fgdci0mbkkdnadTmbinaiaeRbb86bbaicefhiaecefheadcufgdmbkkabk;aeedudndnabciGTmbabhixekaecFeGc:b:c:ew2hldndnadcz9pmbabhixekabhiinaialBdbaicxfalBdbaicwfalBdbaiclfalBdbaiczfhiadc9Wfgdcs0mbkkadcl6mbinaialBdbaiclfhiadc98fgdci0mbkkdnadTmbinaiae86bbaicefhiadcufgdmbkkabkk83dbcj:Gdk8Kbbbbdbbblbbbwbbbbbbbebbbdbbblbbbwbbbbc:K:Gdkl8W:qbb",e="b9H79TebbbeKl9Gbb9Gvuuuuueu9Giuuub9Geueuixkbbebeeddddilve9Weeeviebeoweuecj:Gdkr;Neqo9TW9T9VV95dbH9F9F939H79T9F9J9H229F9Jt9VV7bb8A9TW79O9V9Wt9F9KW9J9V9KW9wWVtW949c919M9MWVbdY9TW79O9V9Wt9F9KW9J9V9KW69U9KW949c919M9MWVblE9TW79O9V9Wt9F9KW9J9V9KW69U9KW949tWG91W9U9JWbvL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9p9JtboK9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9r919HtbrL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVT949WbwY9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVJ9V29VVbDl79IV9Rbqq:W9Dklbzik94evu8Jjjjjbcz9Rhbcbheincbhdcbhiinabcwfadfaicjuaead4ceGglE86bbaialfhiadcefgdcw9hmbkaeai86b:q:W:cjbaecitab8Piw83i:q:G:cjbaecefgecjd9hmbkk:JBl8Aud97dur978Jjjjjbcj;kb9Rgv8Kjjjjbc9:hodnalTmbcuhoaiRbbgrc;WeGc:Ge9hmbarcsGgwce0mbc9:hoalcufadcd4cbawEgDadfgrcKcaawEgqaraq0Egk6mbaialfgxar9RhodnadTgmmbavaoad;8qbbkaicefhPcj;abad9Uc;WFbGcjdadca0EhsdndndnadTmbaoadfhzcbhHinaeaH9nmdaxaP9RaD6miabaHad2fhOaPaDfhAasaeaH9RaHasfae6EgCcsfgocl4cifcd4hXavcj;cbfaoc9WGgQcetfhLavcj;cbfaQci2fhKavcj;cbfaQfhYcbh8Aaoc;ab6hEincbh3dnawTmbaPa8Acd4fRbbh3kcbh5avcj;cbfh8Eindndndndna3a5cet4ciGgoc9:fPdebdkaxaA9RaQ6mwdnaQTmbavcj;cbfa5aQ2faAaQ;8qbbkaAaCfhAxdkaQTmeavcj;cbfa5aQ2fcbaQ;8kbxekaxaA9RaX6moaoclVcbawEhraAaXfhocbhidnaEmbaxao9Rc;Gb6mbcbhlina8EalfhidndndndndndnaAalco4fRbbgqciGarfPDbedibledibkaipxbbbbbbbbbbbbbbbbpklbxlkaiaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaiaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaiaopbbbpklbaoczfhoxekaiaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqcd4ciGarfPDbedibledibkaiczfpxbbbbbbbbbbbbbbbbpklbxlkaiczfaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaiczfaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaiczfaopbbbpklbaoczfhoxekaiczfaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqcl4ciGarfPDbedibledibkaicafpxbbbbbbbbbbbbbbbbpklbxlkaicafaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaicafaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaicafaopbbbpklbaoczfhoxekaicafaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqco4arfPDbedibledibkaic8Wfpxbbbbbbbbbbbbbbbbpklbxlkaic8Wfaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Ngicitpbi:q:G:cjbaiRb:q:W:cjbgipsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Ngqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbaiaoclffaqRb:q:W:cjbfhoxikaic8Wfaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Ngicitpbi:q:G:cjbaiRb:q:W:cjbgipsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Ngqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbaiaocwffaqRb:q:W:cjbfhoxdkaic8Wfaopbbbpklbaoczfhoxekaic8WfaopbbdaoRbbgicitpbi:q:G:cjbaiRb:q:W:cjbgipsaoRbegqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbaiaocdffaqRb:q:W:cjbfhokalc;abfhialcjefaQ0meaihlaxao9Rc;Fb0mbkkdnaiaQ9pmbaici4hlinaxao9RcK6mwa8EaifhqdndndndndndnaAaico4fRbbalcoG4ciGarfPDbedibledibkaqpxbbbbbbbbbbbbbbbbpkbbxlkaqaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spkbbahaoclffagRb:q:W:cjbfhoxikaqaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spkbbahaocwffagRb:q:W:cjbfhoxdkaqaopbbbpkbbaoczfhoxekaqaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpkbbahaocdffagRb:q:W:cjbfhokalcdfhlaiczfgiaQ6mbkkaohAaoTmoka8EaQfh8Ea5cefg5cl9hmbkdndndndnawTmbaza8Acd4fRbbglciGPlbedwbkaQTmdavcjdfa8Afhlava8Afpbdbh8Jcbhoinalavcj;cbfaofpblbg8KaYaofpblbg8LpmbzeHdOiAlCvXoQrLg8MaLaofpblbg8NaKaofpblbgypmbzeHdOiAlCvXoQrLg8PpmbezHdiOAlvCXorQLg8Fcep9Ta8Fpxeeeeeeeeeeeeeeeegap9op9Hp9rg8Fa8Jp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ma8PpmwDKYqk8AExm35Ps8E8Fg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ka8LpmwKDYq8AkEx3m5P8Es8Fg8Ka8NaypmwKDYq8AkEx3m5P8Es8Fg8LpmbezHdiOAlvCXorQLg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ka8LpmwDKYqk8AExm35Ps8E8Fg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ugap9Abbbaladfglaaa8Fa8Fpmlvorlvorlvorlvorp9Ugap9Abbbaladfglaaa8Fa8FpmwDqkwDqkwDqkwDqkp9Ugap9Abbbaladfglaaa8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9AbbbaladfhlaoczfgoaQ6mbxikkaQTmeavcjdfa8Afhlava8Afpbdbh8Jcbhoinalavcj;cbfaofpblbg8KaYaofpblbg8LpmbzeHdOiAlCvXoQrLg8MaLaofpblbg8NaKaofpblbgypmbzeHdOiAlCvXoQrLg8PpmbezHdiOAlvCXorQLg8Fcep:nea8Fpxebebebebebebebebgap9op:bep9rg8Fa8Jp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ma8PpmwDKYqk8AExm35Ps8E8Fg8Fcep:nea8Faap9op:bep9rg8Fp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ka8LpmwKDYq8AkEx3m5P8Es8Fg8Ka8NaypmwKDYq8AkEx3m5P8Es8Fg8LpmbezHdiOAlvCXorQLg8Fcep:nea8Faap9op:bep9rg8Fp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ka8LpmwDKYqk8AExm35Ps8E8Fg8Fcep:nea8Faap9op:bep9rg8Fp:oegap9Abbbaladfglaaa8Fa8Fpmlvorlvorlvorlvorp:oegap9Abbbaladfglaaa8Fa8FpmwDqkwDqkwDqkwDqkp:oegap9Abbbaladfglaaa8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9AbbbaladfhlaoczfgoaQ6mbxdkkaQTmbcbhocbalcl4gl9Rc8FGhiavcjdfa8Afhrava8Afpbdbhainaravcj;cbfaofpblbg8JaYaofpblbg8KpmbzeHdOiAlCvXoQrLg8LaLaofpblbg8MaKaofpblbg8NpmbzeHdOiAlCvXoQrLgypmbezHdiOAlvCXorQLg8Faip:Rea8Falp:Tep9qg8Faap9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8LaypmwDKYqk8AExm35Ps8E8Fg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8Ja8KpmwKDYq8AkEx3m5P8Es8Fg8Ja8Ma8NpmwKDYq8AkEx3m5P8Es8Fg8KpmbezHdiOAlvCXorQLg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8Ja8KpmwDKYqk8AExm35Ps8E8Fg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9AbbbaradfhraoczfgoaQ6mbkka8Aclfg8Aad6mbkdnaCad2goTmbaOavcjdfao;8qbbkdnammbavavcjdfaCcufad2fad;8qbbkaCaHfhHc9:hoaAhPaAmbxlkkaeTmbaDalfhrcbhocuhlinaralaD9RglfaD6mdasaeao9Raoasfae6Eaofgoae6mbkaial9RhPkcbc99axaP9RakSEhoxekc9:hokavcj;kbf8Kjjjjbaokwbz:bjjjbkNsezu8Jjjjjbc;ae9Rgv8Kjjjjbc9:hodnalaeci9UgrcHf6mbcuhoaiRbbgwc;WeGc;Ge9hmbawcsGgDce0mbavc;abfcFecje;8kbav9cu83iUav9cu83i8Wav9cu83iyav9cu83iaav9cu83iKav9cu83izav9cu83iwav9cu83ibaialfc9WfhqaicefgwarfhldnaeTmbcmcsaDceSEhkcbhxcbhmcbhrcbhicbhoindnalaq9nmbc9:hoxikdndnawRbbgDc;Ve0mbavc;abfaoaDcu7gPcl4fcsGcitfgsydlhzasydbhHdndnaDcsGgsak9pmbavaiaPfcsGcdtfydbaxasEhDaxasTgOfhxxekdndnascsSmbcehOasc987asamffcefhDxekalcefhDal8SbbgscFeGhPdndnascu9mmbaDhlxekalcvfhlaPcFbGhPcrhsdninaD8SbbgOcFbGastaPVhPaOcu9kmeaDcefhDascrfgsc8J9hmbxdkkaDcefhlkcehOaPce4cbaPceG9R7amfhDkaDhmkavc;abfaocitfgsaDBdbasazBdlavaicdtfaDBdbavc;abfaocefcsGcitfgsaHBdbasaDBdlaocdfhoaOaifhidnadcd9hmbabarcetfgsaH87ebasclfaD87ebascdfaz87ebxdkabarcdtfgsaHBdbascwfaDBdbasclfazBdbxekdnaDcpe0mbavaiaqaDcsGfRbbgscl4gP9RcsGcdtfydbaxcefgOaPEhDavaias9RcsGcdtfydbaOaPTgzfgOascsGgPEhsaPThPdndnadcd9hmbabarcetfgHax87ebaHclfas87ebaHcdfaD87ebxekabarcdtfgHaxBdbaHcwfasBdbaHclfaDBdbkavaicdtfaxBdbavc;abfaocitfgHaDBdbaHaxBdlavaicefgicsGcdtfaDBdbavc;abfaocefcsGcitfgHasBdbaHaDBdlavaiazfgicsGcdtfasBdbavc;abfaocdfcsGcitfgDaxBdbaDasBdlaocifhoaiaPfhiaOaPfhxxekaxcbalRbbgsEgHaDc;:eSgDfhOascsGhAdndnascl4gCmbaOcefhzxekaOhzavaiaC9RcsGcdtfydbhOkdndnaAmbazcefhxxekazhxavaias9RcsGcdtfydbhzkdndnaDTmbalcefhDxekalcdfhDal8SbegPcFeGhsdnaPcu9kmbalcofhHascFbGhscrhldninaD8SbbgPcFbGaltasVhsaPcu9kmeaDcefhDalcrfglc8J9hmbkaHhDxekaDcefhDkasce4cbasceG9R7amfgmhHkdndnaCcsSmbaDhsxekaDcefhsaD8SbbglcFeGhPdnalcu9kmbaDcvfhOaPcFbGhPcrhldninas8SbbgDcFbGaltaPVhPaDcu9kmeascefhsalcrfglc8J9hmbkaOhsxekascefhskaPce4cbaPceG9R7amfgmhOkdndnaAcsSmbashlxekascefhlas8SbbgDcFeGhPdnaDcu9kmbascvfhzaPcFbGhPcrhDdninal8SbbgscFbGaDtaPVhPascu9kmealcefhlaDcrfgDc8J9hmbkazhlxekalcefhlkaPce4cbaPceG9R7amfgmhzkdndnadcd9hmbabarcetfgDaH87ebaDclfaz87ebaDcdfaO87ebxekabarcdtfgDaHBdbaDcwfazBdbaDclfaOBdbkavc;abfaocitfgDaOBdbaDaHBdlavaicdtfaHBdbavc;abfaocefcsGcitfgDazBdbaDaOBdlavaicefgicsGcdtfaOBdbavc;abfaocdfcsGcitfgDaHBdbaDazBdlavaiaCTaCcsSVfgicsGcdtfazBdbaiaATaAcsSVfhiaocifhokawcefhwaocsGhoaicsGhiarcifgrae6mbkkcbc99alaqSEhokavc;aef8Kjjjjbaok:clevu8Jjjjjbcz9Rhvdnalaecvf9pmbc9:skdnaiRbbc;:eGc;qeSmbcuskav9cb83iwaicefhoaialfc98fhrdnaeTmbdnadcdSmbcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcdtfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgiBdbalaiBdbawcefgwae9hmbxdkkcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcetfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgi87ebalaiBdbawcefgwae9hmbkkcbc99aoarSEk;Toio97eue97aec98Ghedndnadcl9hmbaeTmecbhdinababpbbbgicKp:RecKp:Sep;6eglaicwp:RecKp:Sep;6ealp;Geaiczp:RecKp:Sep;6egvp;Gep;Kep;Legopxbbbbbbbbbbbbbbbbp:2egralpxbbbjbbbjbbbjbbbjgwp9op9rp;Keglpxbb;:9cbb;:9cbb;:9cbb;:9calalp;Meaoaop;Meavaravawp9op9rp;Keglalp;Mep;Kep;Kep;Jep;Negvp;Mepxbbn0bbn0bbn0bbn0grp;KepxFbbbFbbbFbbbFbbbp9oaipxbbbFbbbFbbbFbbbFp9op9qalavp;Mearp;Kecwp:RepxbFbbbFbbbFbbbFbbp9op9qaoavp;Mearp;Keczp:RepxbbFbbbFbbbFbbbFbp9op9qpkbbabczfhbadclfgdae6mbxdkkaeTmbcbhdinabczfgDaDpbbbgipxbbbbbbFFbbbbbbFFgwp9oabpbbbgoaipmbediwDqkzHOAKY8AEgvczp:Reczp:Sep;6eglaoaipmlvorxmPsCXQL358E8FpxFubbFubbFubbFubbp9op;6eavczp:Sep;6egvp;Gealp;Gep;Kep;Legipxbbbbbbbbbbbbbbbbp:2egralpxbbbjbbbjbbbjbbbjgqp9op9rp;Keglpxb;:FSb;:FSb;:FSb;:FSalalp;Meaiaip;Meavaravaqp9op9rp;Keglalp;Mep;Kep;Kep;Jep;Negvp;Mepxbbn0bbn0bbn0bbn0grp;KepxFFbbFFbbFFbbFFbbp9oaiavp;Mearp;Keczp:Rep9qgialavp;Mearp;KepxFFbbFFbbFFbbFFbbp9oglpmwDKYqk8AExm35Ps8E8Fp9qpkbbabaoawp9oaialpmbezHdiOAlvCXorQLp9qpkbbabcafhbadclfgdae6mbkkk;2ileue97euo97dnaec98GgiTmbcbheinabcKfpx:ji:1S:ji:1S:ji:1S:ji:1SabpbbbglabczfgvpbbbgopmlvorxmPsCXQL358E8Fgrczp:Segwpxibbbibbbibbbibbbp9qp;6egDp;NegqaDaDp;MegDaDp;KealaopmbediwDqkzHOAKY8AEgDczp:Reczp:Sep;6eglalp;MeaDczp:Sep;6egoaop;Mearczp:Reczp:Sep;6egrarp;Mep;Kep;Kep;Lepxbbbbbbbbbbbbbbbbp:4ep;Jep;Mepxbbn0bbn0bbn0bbn0gDp;KepxFFbbFFbbFFbbFFbbgkp9oaqaop;MeaDp;Keczp:Rep9qgoaqalp;MeaDp;Keakp9oaqarp;MeaDp;Keczp:Rep9qgDpmwDKYqk8AExm35Ps8E8Fglp5eawclp:RegqpEi:T:j83ibavalp5baqpEd:T:j83ibabcwfaoaDpmbezHdiOAlvCXorQLgDp5eaqpEe:T:j83ibabaDp5baqpEb:T:j83ibabcafhbaeclfgeai6mbkkkuee97dnadcd4ae2c98GgeTmbcbhdinababpbbbgicwp:Recwp:Sep;6eaicep:SepxbbjFbbjFbbjFbbjFp9opxbbjZbbjZbbjZbbjZp:Uep;Mepkbbabczfhbadclfgdae6mbkkk:Sodw97euaec98Ghedndnadcl9hmbaeTmecbhdinabpxbbuJbbuJbbuJbbuJabpbbbgicKp:TeglaicYp:Tep9qgvcdp:Teavp9qgvclp:Teavp9qgop;6ep;Negvaicwp:RecKp:SegraipxFbbbFbbbFbbbFbbbgwp9ogDp:Uep;6ep;Mepxbbn0bbn0bbn0bbn0gqp;Kecwp:RepxbFbbbFbbbFbbbFbbp9oavaDarp:Xeaiczp:RecKp:Segip:Uep;6ep;Meaqp;Keawp9op9qavaDaraip:Uep:Xep;6ep;Meaqp;Keczp:RepxbbFbbbFbbbFbbbFbp9op9qavaoalcep:Rep9oalpxebbbebbbebbbebbbp9op9qp;6ep;Meaqp;KecKp:Rep9qpkbbabczfhbadclfgdae6mbxdkkaeTmbcbhdinabczfgkpxbFu9hbFu9hbFu9hbFu9habpbbbglakpbbbgrpmlvorxmPsCXQL358E8Fgvczp:TegqavcHp:Tep9qgicdp:Teaip9qgiclp:Teaip9qgicwp:Teaip9qgop;6ep;NegialarpmbediwDqkzHOAKY8AEgDpxFFbbFFbbFFbbFFbbglp9ograDczp:Segwp:Ueavczp:Reczp:SegDp:Xep;6ep;Mepxbbn0bbn0bbn0bbn0gvp;Kealp9oaiarawaDp:Uep:Xep;6ep;Meavp;Keczp:Rep9qgwaiaoaqcep:Rep9oaqpxebbbebbbebbbebbbp9op9qp;6ep;Meavp;Keczp:ReaiaDarp:Uep;6ep;Meavp;Kealp9op9qgipmwDKYqk8AExm35Ps8E8FpkbbabawaipmbezHdiOAlvCXorQLpkbbabcafhbadclfgdae6mbkkk9teiucbcbydj:G:cjbgeabcifc98GfgbBdj:G:cjbdndnabZbcztgd9nmbcuhiabad9RcFFifcz4nbcuSmekaehikaikkxebcj:Gdklz:zbb",t=new Uint8Array([0,97,115,109,1,0,0,0,1,4,1,96,0,0,3,3,2,0,0,5,3,1,0,1,12,1,0,10,22,2,12,0,65,0,65,0,65,0,252,10,0,0,11,7,0,65,0,253,15,26,11]),n=new Uint8Array([32,0,65,2,1,106,34,33,3,128,11,4,13,64,6,253,10,7,15,116,127,5,8,12,40,16,19,54,20,9,27,255,113,17,42,67,24,23,146,148,18,14,22,45,70,69,56,114,101,21,25,63,75,136,108,28,118,29,73,115]);if(typeof WebAssembly!="object")return{supported:!1};var i=WebAssembly.validate(t)?o(e):o(s),r,a=WebAssembly.instantiate(i,{}).then(function(f){r=f.instance,r.exports.__wasm_call_ctors()});function o(f){for(var y=new Uint8Array(f.length),T=0;T<f.length;++T){var g=f.charCodeAt(T);y[T]=g>96?g-97:g>64?g-39:g+4}for(var _=0,T=0;T<f.length;++T)y[_++]=y[T]<60?n[y[T]]:(y[T]-60)*64+y[++T];return y.buffer.slice(0,_)}function l(f,y,T,g,_,w,A){var b=f.exports.sbrk,C=g+3&-4,S=b(C*_),P=b(w.length),M=new Uint8Array(f.exports.memory.buffer);M.set(w,P);var F=y(S,g,_,P,w.length);if(F==0&&A&&A(S,C,_),T.set(M.subarray(S,S+g*_)),b(S-b(0)),F!=0)throw new Error("Malformed buffer data: "+F)}var c={NONE:"",OCTAHEDRAL:"meshopt_decodeFilterOct",QUATERNION:"meshopt_decodeFilterQuat",EXPONENTIAL:"meshopt_decodeFilterExp",COLOR:"meshopt_decodeFilterColor"},h={ATTRIBUTES:"meshopt_decodeVertexBuffer",TRIANGLES:"meshopt_decodeIndexBuffer",INDICES:"meshopt_decodeIndexSequence"},u=[],d=0;function p(f){var y={object:new Worker(f),pending:0,requests:{}};return y.object.onmessage=function(T){var g=T.data;y.pending-=g.count,y.requests[g.id][g.action](g.value),delete y.requests[g.id]},y}function x(f){for(var y="self.ready = WebAssembly.instantiate(new Uint8Array(["+new Uint8Array(i)+"]), {}).then(function(result) { result.instance.exports.__wasm_call_ctors(); return result.instance; });self.onmessage = "+m.name+";"+l.toString()+m.toString(),T=new Blob([y],{type:"text/javascript"}),g=URL.createObjectURL(T),_=u.length;_<f;++_)u[_]=p(g);for(var _=f;_<u.length;++_)u[_].object.postMessage({});u.length=f,URL.revokeObjectURL(g)}function v(f,y,T,g,_){for(var w=u[0],A=1;A<u.length;++A)u[A].pending<w.pending&&(w=u[A]);return new Promise(function(b,C){var S=new Uint8Array(T),P=++d;w.pending+=f,w.requests[P]={resolve:b,reject:C},w.object.postMessage({id:P,count:f,size:y,source:S,mode:g,filter:_},[S.buffer])})}function m(f){var y=f.data;self.ready.then(function(T){if(!y.id)return self.close();try{var g=new Uint8Array(y.count*y.size);l(T,T.exports[y.mode],g,y.count,y.size,y.source,T.exports[y.filter]),self.postMessage({id:y.id,count:y.count,action:"resolve",value:g},[g.buffer])}catch(_){self.postMessage({id:y.id,count:y.count,action:"reject",value:_})}})}return{ready:a,supported:!0,useWorkers:function(f){x(f)},decodeVertexBuffer:function(f,y,T,g,_){l(r,r.exports.meshopt_decodeVertexBuffer,f,y,T,g,r.exports[c[_]])},decodeIndexBuffer:function(f,y,T,g){l(r,r.exports.meshopt_decodeIndexBuffer,f,y,T,g)},decodeIndexSequence:function(f,y,T,g){l(r,r.exports.meshopt_decodeIndexSequence,f,y,T,g)},decodeGltfBuffer:function(f,y,T,g,_,w){l(r,r.exports[h[_]],f,y,T,g,r.exports[c[w]])},decodeGltfBufferAsync:function(f,y,T,g,_){return u.length>0?v(f,y,T,h[g],c[_]):a.then(function(){var w=new Uint8Array(f*y);return l(r,r.exports[h[g]],w,f,y,T,r.exports[c[_]]),w})}}})();var Rc=class extends Us{constructor(){super(),this.name="RoomEnvironment",this.position.y=-3.5;let e=new pi;e.deleteAttribute("uv");let t=new zn({side:mn}),n=new zn,i=new gi(16777215,900,28,2);i.position.set(.418,16.199,.3),this.add(i);let r=new st(e,t);r.position.set(-.757,13.219,.717),r.scale.set(31.713,28.305,28.591),this.add(r);let a=new Sn(e,n,6),o=new Wt;o.position.set(-10.906,2.009,1.846),o.rotation.set(0,-.195,0),o.scale.set(2.328,7.905,4.651),o.updateMatrix(),a.setMatrixAt(0,o.matrix),o.position.set(-5.607,-.754,-.758),o.rotation.set(0,.994,0),o.scale.set(1.97,1.534,3.955),o.updateMatrix(),a.setMatrixAt(1,o.matrix),o.position.set(6.167,.857,7.803),o.rotation.set(0,.561,0),o.scale.set(3.927,6.285,3.687),o.updateMatrix(),a.setMatrixAt(2,o.matrix),o.position.set(-2.017,.018,6.124),o.rotation.set(0,.333,0),o.scale.set(2.002,4.566,2.064),o.updateMatrix(),a.setMatrixAt(3,o.matrix),o.position.set(2.291,-.756,-2.621),o.rotation.set(0,-.286,0),o.scale.set(1.546,1.552,1.496),o.updateMatrix(),a.setMatrixAt(4,o.matrix),o.position.set(-2.193,-.369,-5.547),o.rotation.set(0,.516,0),o.scale.set(3.875,3.487,2.986),o.updateMatrix(),a.setMatrixAt(5,o.matrix),this.add(a);let l=new st(e,Jr(50));l.position.set(-16.116,14.37,8.208),l.scale.set(.1,2.428,2.739),this.add(l);let c=new st(e,Jr(50));c.position.set(-16.109,18.021,-8.207),c.scale.set(.1,2.425,2.751),this.add(c);let h=new st(e,Jr(17));h.position.set(14.904,12.198,-1.832),h.scale.set(.15,4.265,6.331),this.add(h);let u=new st(e,Jr(43));u.position.set(-.462,8.89,14.52),u.scale.set(4.38,5.441,.088),this.add(u);let d=new st(e,Jr(20));d.position.set(3.235,11.486,-12.541),d.scale.set(2.5,2,.1),this.add(d);let p=new st(e,Jr(100));p.position.set(0,20,0),p.scale.set(1,.1,1),this.add(p)}dispose(){let e=new Set;this.traverse(t=>{t.isMesh&&(e.add(t.geometry),e.add(t.material))});for(let t of e)t.dispose()}};function Jr(s){return new Ia({color:0,emissive:16777215,emissiveIntensity:s})}var es={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};var Zn=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},fy=new Li(-1,1,1,-1,0,1),Gu=class extends _t{constructor(){super(),this.setAttribute("position",new mt([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new mt([0,2,0,0,2,0],2))}},py=new Gu,Oi=class{constructor(e){this._mesh=new st(py,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,fy)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var Zr=class extends Zn{constructor(e,t="tDiffuse"){super(),this.textureID=t,this.uniforms=null,this.material=null,e instanceof yt?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=_i.clone(e.uniforms),this.material=new yt({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new Oi(this.material)}render(e,t,n){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=n.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var lo=class extends Zn{constructor(e,t){super(),this.scene=e,this.camera=t,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,t,n){let i=e.getContext(),r=e.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let a,o;this.inverse?(a=0,o=1):(a=1,o=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(i.REPLACE,i.REPLACE,i.REPLACE),r.buffers.stencil.setFunc(i.ALWAYS,a,4294967295),r.buffers.stencil.setClear(o),r.buffers.stencil.setLocked(!0),e.setRenderTarget(n),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(t),this.clear&&e.clear(),e.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(i.EQUAL,1,4294967295),r.buffers.stencil.setOp(i.KEEP,i.KEEP,i.KEEP),r.buffers.stencil.setLocked(!0)}},Cc=class extends Zn{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var Pc=class{constructor(e,t){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),t===void 0){let n=e.getSize(new Be);this._width=n.width,this._height=n.height,t=new an(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:pn}),t.texture.name="EffectComposer.rt1"}else this._width=t.width,this._height=t.height;this.renderTarget1=t,this.renderTarget2=t.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Zr(es),this.copyPass.material.blending=ni,this.timer=new ka}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,t){this.passes.splice(t,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let t=this.passes.indexOf(e);t!==-1&&this.passes.splice(t,1)}isLastEnabledPass(e){for(let t=e+1;t<this.passes.length;t++)if(this.passes[t].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let t=this.renderer.getRenderTarget(),n=!1;for(let i=0,r=this.passes.length;i<r;i++){let a=this.passes[i];if(a.enabled!==!1){if(a.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(i),a.render(this.renderer,this.writeBuffer,this.readBuffer,e,n),a.needsSwap){if(n){let o=this.renderer.getContext(),l=this.renderer.state.buffers.stencil;l.setFunc(o.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),l.setFunc(o.EQUAL,1,4294967295)}this.swapBuffers()}lo!==void 0&&(a instanceof lo?n=!0:a instanceof Cc&&(n=!1))}}this.renderer.setRenderTarget(t)}reset(e){if(e===void 0){let t=this.renderer.getSize(new Be);this._pixelRatio=this.renderer.getPixelRatio(),this._width=t.width,this._height=t.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,t){this._width=e,this._height=t;let n=this._width*this._pixelRatio,i=this._height*this._pixelRatio;this.renderTarget1.setSize(n,i),this.renderTarget2.setSize(n,i);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(n,i)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var Pp={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Le(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var $r=class s extends Zn{constructor(e,t=1,n,i){super(),this.strength=t,this.radius=n,this.threshold=i,this.resolution=e!==void 0?new Be(e.x,e.y):new Be(256,256),this.clearColor=new Le(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),a=Math.round(this.resolution.y/2);this.renderTargetBright=new an(r,a,{type:pn,depthBuffer:!1}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let h=0;h<this.nMips;h++){let u=new an(r,a,{type:pn,depthBuffer:!1});u.texture.name="UnrealBloomPass.h"+h,u.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(u);let d=new an(r,a,{type:pn,depthBuffer:!1});d.texture.name="UnrealBloomPass.v"+h,d.texture.generateMipmaps=!1,this.renderTargetsVertical.push(d),r=Math.round(r/2),a=Math.round(a/2)}let o=Pp;this.highPassUniforms=_i.clone(o.uniforms),this.highPassUniforms.luminosityThreshold.value=i,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new yt({uniforms:this.highPassUniforms,vertexShader:o.vertexShader,fragmentShader:o.fragmentShader}),this.separableBlurMaterials=[];let l=[6,10,14,18,22];r=Math.round(this.resolution.x/2),a=Math.round(this.resolution.y/2);for(let h=0;h<this.nMips;h++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(l[h])),this.separableBlurMaterials[h].uniforms.invSize.value=new Be(1/r,1/a),r=Math.round(r/2),a=Math.round(a/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=t,this.compositeMaterial.uniforms.bloomRadius.value=.1;let c=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=c,this.bloomTintColors=[new B(1,1,1),new B(1,1,1),new B(1,1,1),new B(1,1,1),new B(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=_i.clone(es.uniforms),this.blendMaterial=new yt({uniforms:this.copyUniforms,vertexShader:es.vertexShader,fragmentShader:es.fragmentShader,premultipliedAlpha:!0,blending:tn,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new Le,this._oldClearAlpha=1,this._basic=new Vt,this._fsQuad=new Oi(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,t){let n=Math.round(e/2),i=Math.round(t/2);this.renderTargetBright.setSize(n,i);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(n,i),this.renderTargetsVertical[r].setSize(n,i),this.separableBlurMaterials[r].uniforms.invSize.value=new Be(1/n,1/i),n=Math.round(n/2),i=Math.round(i/2)}render(e,t,n,i,r){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let a=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),r&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=n.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=n.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let o=this.renderTargetBright;for(let l=0;l<this.nMips;l++)this._fsQuad.material=this.separableBlurMaterials[l],this.separableBlurMaterials[l].uniforms.colorTexture.value=o.texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[l]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[l].uniforms.colorTexture.value=this.renderTargetsHorizontal[l].texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[l]),e.clear(),this._fsQuad.render(e),o=this.renderTargetsVertical[l];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(n),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=a}_getSeparableBlurMaterial(e){let t=[],n=e/3;for(let a=0;a<e;a++)t.push(.39894*Math.exp(-.5*a*a/(n*n))/n);let i=[],r=[];for(let a=1;a<e;a+=2){let o=t[a],l=a+1<e?t[a+1]:0,c=o+l;i.push((a*o+(a+1)*l)/c),r.push(c)}return new yt({defines:{KERNEL_PAIRS:i.length},uniforms:{colorTexture:{value:null},invSize:{value:new Be(.5,.5)},direction:{value:new Be(.5,.5)},centerWeight:{value:t[0]},gaussianOffsets:{value:i},gaussianWeights:{value:r}},vertexShader:`

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

				}`})}_getCompositeMaterial(e){return new yt({defines:{NUM_MIPS:e},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`

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

				}`})}};$r.BlurDirectionX=new Be(1,0);$r.BlurDirectionY=new Be(0,1);var co={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var Ic=class extends Zn{constructor(){super(),this.isOutputPass=!0,this.uniforms=_i.clone(co.uniforms),this.material=new Lr({name:co.name,uniforms:this.uniforms,vertexShader:co.vertexShader,fragmentShader:co.fragmentShader}),this._fsQuad=new Oi(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,t,n){this.uniforms.tDiffuse.value=n.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},Tt.getTransfer(this._outputColorSpace)===Gt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Va?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===Wa?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===qa?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===Ws?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===ja?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Ka?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Xa&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Mi=512,my=7.5;function Vu(s,e=my){let t=new Ra,n=s.map(([o,l])=>new B(o,0,l)),i=[],r=[],a=n.length;for(let o=0;o<a;o++){let l=n[o],c=n[(o+a-1)%a],h=n[(o+1)%a],u=Math.min(e,l.distanceTo(c)/2,l.distanceTo(h)/2);i.push(l.clone().addScaledVector(c.clone().sub(l).normalize(),u)),r.push(l.clone().addScaledVector(h.clone().sub(l).normalize(),u))}for(let o=0;o<a;o++)t.add(new mi(i[o],n[o],r[o])),t.add(new Ir(r[o],i[(o+1)%a]));return t}function gy(s){let e=Vu(s),t=e.getSpacedPoints(Mi),n=[];for(let i=0;i<Mi;i++){let r=t[(i+Mi-1)%Mi],a=t[(i+1)%Mi];n.push(new B(a.x-r.x,0,a.z-r.z).normalize())}return{points:t,tangents:n,length:e.getLength()}}function Ip(s){let e=[],t=(n,i,r,a)=>{let o=Math.cos(n.angle),l=Math.sin(n.angle);e.push({x:n.x+i*o+r*l,z:n.z-i*l+r*o,r:a,asset:n.asset})};for(let n of s)if(!(n.z<-100))if(n.asset==="data-tram")for(let i of[-2.4,0,2.4])t(n,i,0,1.5);else n.asset==="server-rack"?t(n,0,0,.8):n.asset==="street-lamp"?t(n,0,0,.5):n.asset==="planter"&&(t(n,-.9,0,.85),t(n,.9,0,.85));return e}function Dp({routes:s,obstacles:e=[],robotRadius:t=1.3,lanes:n=[2.2,0,-2.2,3.9,-3.9],defaultLane:i=2.2,speeds:r=[],blocked:a=()=>!1}){let o=s.map(gy),l=s.map((S,P)=>({route:P,s:o[P].length*(P*.173%1),dir:1,lane:i,laneTarget:i,speed:0,target:0,laneVel:0,cruise:r[P]??5.1+P*.43,state:"cruise",heading:0,wait:0,cooldown:0,x:0,z:0,fx:0,fz:1,rx:1,rz:0,vx:0,vz:0,turns:0})),c=new B,h=new B,u=new B;function d(S,P,M,F,I){let N=(P/S.length%1+1)%1*Mi,H=Math.floor(N)%Mi,D=N-Math.floor(N);F.lerpVectors(S.points[H],S.points[(H+1)%Mi],D),I.lerpVectors(S.tangents[H],S.tangents[(H+1)%Mi],D).normalize().multiplyScalar(M)}let p=S=>Math.atan2(Math.sin(S),Math.cos(S));function x(S){d(o[S.route],S.s,S.dir,c,h),S.fx=h.x,S.fz=h.z,S.rx=-h.z,S.rz=h.x,S.x=c.x+S.rx*S.lane,S.z=c.z+S.rz*S.lane}function v(S,P){for(let M of e){let F=M.r+t;if((S-M.x)**2+(P-M.z)**2<F*F)return!0}return!1}function m(S,P){let M=o[S.route];for(let F of[-2.5,0,1,3,6,9,12,15,18])if(d(M,S.s+S.dir*F,S.dir,c,h),u.set(c.x-h.z*P,0,c.z+h.x*P),v(u.x,u.z)||F>=0&&a(S,u.x,u.z))return!1;return!0}function f(S){S.dir=-S.dir,S.lane=-S.lane,S.laneTarget=i,S.laneVel=0,S.state="turn",S.cooldown=4,S.wait=0,S.target=0,S.turns++,S.blocked=0}function y(S,P){if(S.held){S.target=0,S.speed=0,S.laneVel=0;return}if(S.cooldown>0&&(S.cooldown-=P),S.state==="turn"){if(S.target=0,S.speed<.05){let N=Math.atan2(S.fx,S.fz),H=p(N-S.heading);S.heading+=H*Math.min(1,P*3.2),Math.abs(H)<.1&&(S.heading=N,S.state="cruise")}return}let M=null,F=[...n].sort((N,H)=>Math.abs(N-i)-Math.abs(H-i)||H-N);for(let N of F)if(m(S,N)){M=N;break}if(M===null){S.blocked=(S.blocked||0)+P,S.target=0,S.blocked>2.5&&S.cooldown<=0&&(S.blocked=0,f(S));return}S.blocked=0,S.laneTarget=M,S.target=S.cruise,Math.abs(S.laneTarget-S.lane)>.5&&!m(S,S.lane)&&(S.target=Math.min(S.target,2.4));let I=!1;for(let N of l){if(N===S)continue;let H=A(S,S.laneTarget,N);if(!H)continue;I=!0;let D=N.fx*S.fx+N.fz*S.fz,V=N.speed<.3;if(V||D<-.2){let k=g.get(N)[T.length-1],E=V?0:Math.sign((k.x-S.x)*S.rx+(k.z-S.z)*S.rz)||-1,Z=null;for(let W of F)if(!(E&&Math.sign(W-S.lane)===E)&&m(S,W)&&!A(S,W,N)){Z=W;break}Z!==null?S.laneTarget=Z:S.target=0}else D>.5?H.da>=H.db&&(S.target=Math.min(S.target,H.da<3.8?0:N.speed*.9)):(H.da>H.db||H.da===H.db&&S.route>N.route)&&(S.target=Math.min(S.target,H.da<=4?0:2))}I&&S.target===0?S.wait+=P:S.wait=0,S.wait>2.5&&S.cooldown<=0&&f(S)}let T=[0,2,4,6,8,10,12],g=new Map,_=new B;function w(S,P,M,F){d(o[S.route],S.s+S.dir*P,S.dir,c,h),F.set(c.x-h.z*M,0,c.z+h.x*M)}function A(S,P,M){let F=g.get(M),I=(t*2+.6)**2,N=M.speed<.3;for(let H=0;H<T.length;H++){w(S,T[H],P,_);let D=T[H]/Math.max(S.speed,1.5);for(let V=0;V<T.length&&!(N&&V>0);V++){let k=T[V]/Math.max(M.speed,1.5);if(!(T[H]>4&&Math.abs(D-k)>1.6)&&_.distanceToSquared(F[V])<I)return{da:T[H],db:T[V]}}}return null}function b(S){if(S>0){for(let P of l){x(P),g.has(P)||g.set(P,T.map(()=>new B));let M=g.get(P);T.forEach((F,I)=>w(P,P.state==="cruise"?F:0,P.lane,M[I]))}for(let P of l)y(P,S);for(let P=0;P<l.length;P++)for(let M=P+1;M<l.length;M++){let F=l[P],I=l[M],N=I.x-F.x,H=I.z-F.z,D=Math.hypot(N,H),V=t*2+.2;if(D>=V)continue;let k=F.state==="turn"||F.speed<.3,E=I.state==="turn"||I.speed<.3,Z=k===E?.5:k?0:1,W=Math.min(V-Math.max(D,.01),2.5*S),Q=N*F.rx+H*F.rz,Fe=-(N*I.rx+H*I.rz);F.lane=Ft.clamp(F.lane-Math.sign(Q||1)*W*Z,-4.2,4.2),I.lane=Ft.clamp(I.lane-Math.sign(Fe||1)*W*(1-Z),-4.2,4.2),F.target=Math.min(F.target,.5),I.target=Math.min(I.target,.5)}for(let P of l){let M=P.x,F=P.z,I=P.target>P.speed?6:9;if(P.held){P.vx=P.vz=0;continue}P.speed+=Ft.clamp(P.target-P.speed,-I*S,I*S),P.speed<0&&(P.speed=0),P.s+=P.dir*P.speed*S;let N=P.laneTarget-P.lane,H=P.state==="turn"?0:Math.min(3,P.speed*.6),D=Math.sign(N)*Math.min(H,Math.sqrt(8*Math.abs(N)),Math.abs(N)/S);if(P.laneVel+=Ft.clamp(D-P.laneVel,-4*S,4*S),P.lane+=P.laneVel*S,x(P),P.vx=(P.x-M)/S,P.vz=(P.z-F)/S,P.state==="cruise"){let V=P.speed>.25?Math.atan2(P.vx,P.vz):Math.atan2(P.fx,P.fz);P.heading+=p(V-P.heading)*Math.min(1,S*(P.speed>.25?8:3.2))}}}}let C=[];for(let S of l){for(let P=0;P<600&&(x(S),!(C.every(F=>Math.hypot(S.x-F.x,S.z-F.z)>t*2+6)&&m(S,S.lane)));P++)S.s+=1;S.heading=Math.atan2(S.fx,S.fz),C.push(S)}return{agents:l,step:b,hitsObstacle:v,seek(S,P){S.s=P,x(S),S.heading=Math.atan2(S.fx,S.fz)},adopt(S,P){let M=o[S.route],F=0,I=1/0;for(let N=0;N<Mi;N++){let H=M.points[N],D=Math.hypot(H.x-P.x,H.z-P.z);D<I&&(I=D,F=N)}S.s=F/Mi*M.length,d(M,S.s,S.dir,c,h),S.lane=(P.x-c.x)*-h.z+(P.z-c.z)*h.x,S.laneTarget=i,S.speed=0,S.laneVel=0,S.state="cruise",S.heading=P.heading,x(S)},stats:()=>({states:l.map(S=>S.state),turns:l.reduce((S,P)=>S+P.turns,0),lanes:l.map(S=>Math.round(S.lane*10)/10)})}}var xy=(s,e,t)=>Math.max(e,Math.min(t,s)),Fc=(s,e)=>Math.atan2(Math.sin(e-s),Math.cos(e-s)),ho=s=>({x:s.x,y:s.y||0,z:s.z,heading:s.heading||0});function ts(s,e=1){let t=s.min.map(d=>d*e),n=s.max.map(d=>d*e),i=(n[0]-t[0])/2,r=(n[2]-t[2])/2,a=i>r,o=Math.max(i,r),l=Math.min(i,r),c=Math.min(7,Math.max(1,Math.ceil(o/Math.max(.3,l)))),h=o/c,u=[];for(let d=0;d<c;d++){let p=-o+h+d*h*2;u.push({x:(t[0]+n[0])/2+(a?p:0),z:(t[2]+n[2])/2+(a?0:p),r:Math.hypot(l,h)})}return{circles:u,minY:t[1],maxY:n[1],reach:Math.max(...u.map(d=>Math.hypot(d.x,d.z)+d.r))}}function Qr(s,e){let t=Math.cos(e.heading),n=Math.sin(e.heading);return{x:e.x+s.x*t+s.z*n,z:e.z-s.x*n+s.z*t,r:s.r}}function Lc(s,e,t,n,i){let r=e-s;if(Math.abs(r)<1e-10)return s>=t&&s<=n;let a=(t-s)/r,o=(n-s)/r;return i[0]=Math.max(i[0],Math.min(a,o)),i[1]=Math.min(i[1],Math.max(a,o)),i[0]<=i[1]}function by(s,e,t,n,i,r){let a=[0,1];return Lc(e.y-i.y,t.y-r.y,n.minY-s.maxY+.06,n.maxY-s.minY-.06,a)?a:null}function Dc(s,e,t,n,i,r){let a=s.reach+n.reach+.06;if(Math.min(e.x,t.x)-Math.max(i.x,r.x)>a||Math.min(i.x,r.x)-Math.max(e.x,t.x)>a||Math.min(e.z,t.z)-Math.max(i.z,r.z)>a||Math.min(i.z,r.z)-Math.max(e.z,t.z)>a)return!1;let o=by(s,e,t,n,i,r);if(!o)return!1;for(let l of s.circles)for(let c of n.circles){let h=.06+Math.abs(Fc(e.heading,t.heading))*Math.hypot(l.x,l.z)+Math.abs(Fc(i.heading,r.heading))*Math.hypot(c.x,c.z),u=Qr(l,e),d=Qr(l,t),p=Qr(c,i),x=Qr(c,r),v=u.x-p.x,m=u.z-p.z,f=d.x-x.x-v,y=d.z-x.z-m,T=xy(-(v*f+m*y)/Math.max(1e-12,f*f+y*y),...o),g=l.r+c.r+h;if((v+f*T)**2+(m+y*T)**2<g*g)return!0}return!1}function Wu(s,e,t,n){let i=s.reach+.06;if(n.world&&(Math.min(e.x,t.x)-i>n.world[2]||Math.max(e.x,t.x)+i<n.world[0]||Math.min(e.z,t.z)-i>n.world[3]||Math.max(e.z,t.z)+i<n.world[1])||Math.min(e.y,t.y)+s.minY>=n.max[1]||Math.max(e.y,t.y)+s.maxY<=n.min[1])return!1;let r=Math.cos(n.heading||0),a=Math.sin(n.heading||0),o=l=>({x:(l.x-n.x)*r-(l.z-n.z)*a,z:(l.x-n.x)*a+(l.z-n.z)*r});for(let l of s.circles){let c=o(Qr(l,e)),h=o(Qr(l,t)),u=l.r+.06+Math.abs(Fc(e.heading,t.heading))*Math.hypot(l.x,l.z),d=[0,1];if(Lc(e.y,t.y,n.min[1]-s.maxY+.06,n.max[1]-s.minY-.06,d)&&Lc(c.x,h.x,n.min[0]-u,n.max[0]+u,d)&&Lc(c.z,h.z,n.min[2]-u,n.max[2]+u,d))return!0}return!1}function Nc(){let s=new Map,e=new Map,t=new Map,n=0,i=0,r=0,a=0,o=(x,v)=>x.id===v.id||x.ignore===v.id||v.ignore===x.id||x.enabled===!1||v.enabled===!1;function l(x,v,m,f=!0){if(![m.x,m.y,m.z,m.heading].every(Number.isFinite))return"invalid-pose";for(let y of e.values())if(y.enabled!==!1&&!(y.owner&&(y.owner===x.ignore||y.owner===x.id))&&Wu(x,v,m,y))return y.id;if(f){for(let y of s.values())if(!o(x,y)&&Dc(x,v,m,y,y,y))return y.id}return null}let c=(x,v,m,f=!0)=>!l(x,v,m,f);function h(x,v,m,f=0){if(s.has(x))return s.get(x);let y={id:x,...v,...ho(m),priority:f,enabled:!0,blocked:0};return c(y,y,y)?(s.set(x,y),y):null}function u(x,v,m=12){let f={...x,enabled:!0};for(let y=0;y<=m;y+=.5)for(let T=0;T<(y?24:1);T++){let g={...ho(v),x:v.x+Math.cos(T*Math.PI/12)*y,z:v.z+Math.sin(T*Math.PI/12)*y};if(c(f,g,g))return g}return null}function d(x,v,m=12){let f=u(x,v,m);return f?(Object.assign(x,f),!0):!1}function p(x){let v=[...s.values()].filter(m=>m.enabled!==!1).map(m=>{let f=t.get(m.id),y=f?.next||ho(m);return{body:m,from:ho(m),next:y,request:f,accepted:c(m,m,y,!1)}});for(let m=0;m<=v.length;m++){let f=!1;for(let y of v)y.accepted&&y.body.follow&&v.some(T=>T.body.id===y.body.follow&&!T.accepted)&&(y.accepted=!1,f=!0);for(let y=0;y<v.length;y++)for(let T=y+1;T<v.length;T++){let g=v[y],_=v[T];if(o(g.body,_.body)||!g.accepted&&!_.accepted)continue;let w=g.accepted?g.next:g.from,A=_.accepted?_.next:_.from;if(!Dc(g.body,g.from,w,_.body,_.from,A))continue;let b=k=>k.request&&k.accepted&&(Math.hypot(k.next.x-k.from.x,k.next.y-k.from.y,k.next.z-k.from.z)>1e-8||Math.abs(Fc(k.from.heading,k.next.heading))>1e-8),C=b(g),S=b(_);if(!C&&!S)continue;let P=g.next.x-g.from.x,M=g.next.z-g.from.z,F=_.next.x-_.from.x,I=_.next.z-_.from.z,N=P*F+M*I>.8*Math.hypot(P,M)*Math.hypot(F,I),H=C&&!Dc(g.body,g.from,g.next,_.body,_.from,_.from),D=S&&!Dc(g.body,g.from,g.from,_.body,_.from,_.next),V=C?S?H&&!D?_:D&&!H?g:g.body.priority!==_.body.priority?g.body.priority<_.body.priority?g:_:N?(_.from.x-g.from.x)*P+(_.from.z-g.from.z)*M>0?g:_:g.body.id>_.body.id?g:_:g:_;V.accepted=!1,f=!0}if(!f)break}for(let m of v)m.accepted?(Object.assign(m.body,m.next),m.body.blocked=0,m.body.obstruction=null):m.request&&(m.body.blocked+=x,m.body.obstruction=l(m.body,m.from,m.next),n++);for(let m of v)m.request?.commit?.(m.accepted,m.body);t.clear(),i++,r+=x}return{register:h,relocate:d,findFree:u,clear:c,obstruction:l,solve:p,begin(){t.clear()},propose(x,v,m){x?.enabled!==!1&&x&&t.set(x.id,{next:ho(v),commit:m})},remove(x){s.delete(x),t.delete(x)},solid(x,v){let m=e.get(x),f={id:x,x:0,z:0,heading:0,...v};if(m&&m.x===f.x&&m.z===f.z&&m.heading===f.heading&&m.enabled===f.enabled&&m.min.every((w,A)=>w===f.min[A])&&m.max.every((w,A)=>w===f.max[A]))return;let y=Math.cos(f.heading),T=Math.sin(f.heading),g=[],_=[];for(let w of[f.min[0],f.max[0]])for(let A of[f.min[2],f.max[2]])g.push(f.x+w*y+A*T),_.push(f.z-w*T+A*y);f.world=[Math.min(...g),Math.min(..._),Math.max(...g),Math.max(..._)],e.set(x,f),a++},removeSolid(x){e.delete(x)&&a++},removeOwner(x){for(let[v,m]of e)m.owner===x&&(e.delete(v),a++)},revision:()=>a,solidClear(x){return[...s.values()].every(v=>v.enabled===!1||x.owner&&v.ignore===x.owner||!Wu(v,v,v,{x:0,z:0,heading:0,...x}))},ceiling(x,v,m){let f=0;for(let y of e.values())y.enabled!==!1&&Wu(x,{x:v,y:y.min[1],z:m,heading:0},{x:v,y:y.min[1],z:m,heading:0},y)&&(f=Math.max(f,y.max[1]-x.minY+1));return f},body:x=>s.get(x),neighbors(x,v=12){return[...s.values()].filter(m=>!o(x,m)&&Math.abs(m.y-x.y)<4&&Math.hypot(m.x-x.x,m.z-x.z)<v)},stats:()=>({bodies:s.size,solids:e.size,stops:n,steps:i,time:r,poses:[...s.values()].filter(x=>x.enabled!==!1).map(x=>({id:x.id,x:x.x,y:x.y,z:x.z,heading:x.heading,blocked:x.blocked,obstruction:x.obstruction}))}),dispose(){s.clear(),e.clear(),t.clear()}}}function Lp(s,e=1/60){let t=0;return n=>{if(!(n>0))return t=0,0;t=Math.min(t+n,e*6);let i=0;for(;t>=e-1e-9;)s(e),t-=e,i++;return i}}var Bt={xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80,quayZ:74},gt={ground:0,road:-.14,pavement:0,quay:.08,room:.16,gallery:4.16},Dn=["agent","memory","missions"].map((s,e)=>{let t=[0,-43,43][e],n=73;return{id:s,x:t,z:n,width:12,depth:12,front:-1,doorZ:n-6,liftX:t+4,liftZ:n+3}}),ri=[{id:"infra",x:-67,z:-53,platformX:-74,platformZ:-53,angle:Math.PI/2},{id:"memory",x:-67,z:-10,platformX:-74,platformZ:-10,angle:Math.PI/2},{id:"graph",x:-43,z:59,platformX:-43,platformZ:64.5,angle:0},{id:"operations",x:0,z:59,platformX:0,platformZ:64.5,angle:0},{id:"missions",x:43,z:59,platformX:43,platformZ:64.5,angle:0},{id:"integrations",x:67,z:-10,platformX:74,platformZ:-10,angle:-Math.PI/2},{id:"agent",x:0,z:-77,platformX:0,platformZ:-84,angle:Math.PI}],qu=[[-67,-77],[-67,59],[67,59],[67,-77]],Fp=(s,e)=>{let t=s*Math.PI/180;return{x:Math.sin(t)*e,z:-12-Math.cos(t)*e,angle:Math.PI-t}},qt={x:0,z:-12,floor:72.2,inner:4.2,outer:8,bridge:1,lift:{x:9.8,z:-12,base:.88,cab:.03,travel:71.32},telescopes:[45,135,225,315].map(s=>({id:"deck-"+s,...Fp(s,7.25)})),benches:[0,180,270].map(s=>Fp(s,6.6))};function Np(s,e,t){let n=Math.hypot(s-qt.x,e-qt.z);return n>=qt.inner&&n<=qt.outer?!0:t&&s>=qt.outer-.5&&s<=qt.lift.x+1&&Math.abs(e-qt.lift.z)<qt.bridge}var zi={x:78,z:30},Up=[{id:"south",x:-7.8,z:63.75,angle:Math.PI},{id:"harbour",x:23.3,z:75,angle:-Math.PI/2},{id:"west",x:-72.3,z:-22,angle:Math.PI/2},{id:"east",x:72.3,z:42,angle:-Math.PI/2},{id:"north",x:-8,z:-26.7,angle:Math.PI}],Uc=[{x:30.5,z:-55,scale:1},{x:49,z:-55,scale:1.25},{x:-3,z:-57,scale:1.2}];function vy(s,e,t,n=0){return e>=s.x-5.85+n&&e<=s.x+2.4-n&&t>=s.z+n&&t<=s.z+5.85-n}function Op(s,e,t,n=0){return Math.abs(e-s.liftX)<=1.6-n&&Math.abs(t-s.liftZ)<=1.6-n}function zp(){let s=[],e=[2.4,1.4,5.6,4.6],t=(n,i,r,a)=>{r>n&&a>i&&s.push({x:(n+r)/2,z:(i+a)/2,sx:(r-n)/4,sz:(a-i)/4})};for(let n=-4;n<=4;n+=4)for(let i=-4;i<=4;i+=4){let[r,a,o,l]=[n-2,i-2,n+2,i+2],[c,h,u,d]=[Math.max(r,e[0]),Math.max(a,e[1]),Math.min(o,e[2]),Math.min(l,e[3])];if(c>=u||h>=d){t(r,a,o,l);continue}t(r,a,c,l),t(u,a,o,l),t(c,a,u,h),t(c,d,u,l)}return s}function Xu(s,e,t,n){return vy(s,e,t,.25)?!0:n?e>=s.x+1.8&&e<=s.liftX+1.25&&Math.abs(t-s.liftZ)<1.25:!1}var Ht={line:3.3,run:.14,top:.14,rise:.14,droppedRun:1.2,opening:1.2,flare:.8,zebraOffset:8,cornerRadius:1.2,endDepth:.5,pavementLine:4.75},ki=4,_y=1,ju=48,Js={top:-.2,bottom:-3},Bp=.15,yy=.1,kp=s=>s<0?0:s>1?1:s,Ku=[...Bt.zs.map(s=>({axis:"x",line:s,from:Bt.minX,to:Bt.maxX,crossings:Bt.xs})),...Bt.xs.map(s=>({axis:"z",line:s,from:Bt.minZ,to:Bt.quayZ,crossings:Bt.zs}))];var kc=[];for(let s of Ku)for(let e of s.crossings)for(let t of[-1,1]){let n=e+t*Ht.zebraOffset;n>=s.from&&n<=s.to&&kc.push({axis:s.axis,line:s.line,at:n,x:s.axis==="x"?n:s.line,z:s.axis==="x"?s.line:n})}function Hp(){return kc.map(s=>({...s}))}function My(s){return s<=Ht.opening?Ht.droppedRun:s>=Ht.opening+Ht.flare?Ht.run:Ht.droppedRun+(Ht.run-Ht.droppedRun)*(s-Ht.opening)/Ht.flare}function Yu(s,e){let{xs:t,zs:n,halfWidth:i,minX:r,maxX:a,minZ:o,quayZ:l}=Bt,c=1/0,h=1/0;if(e>=o&&e<=l)for(let g of t)c=Math.min(c,Math.abs(s-g));if(s>=r&&s<=a)for(let g of n)h=Math.min(h,Math.abs(e-g));let u=c<=i,d=h<=i;if(!u&&!d)return null;let p=gt.road,x,v=Ht.run;if(u&&d)if(c<=Ht.line||h<=Ht.line)x=-1;else{let g=c-Ht.line,_=h-Ht.line,w=Ht.cornerRadius;x=g<w&&_<w?w-Math.hypot(w-g,w-_):Math.min(g,_)}else{let g=d?s:e,_=d?t:n,w=1/0;for(let A of _)w=Math.min(w,Math.abs(g-A-Ht.zebraOffset),Math.abs(g-A+Ht.zebraOffset));x=(d?h:c)-Ht.line,v=My(w)}let m=p+Ht.rise*kp(x/v),f=x<=0?"road":x>=v?"pavement":"kerb",y=Math.min(u?e-o:1/0,u?l-e:1/0,d?s-r:1/0,d?a-s:1/0),T=Ht.endDepth+Ht.run;if(y<T){let g=p+Ht.rise*kp((T-y)/Ht.run);g>m&&(m=g,f="kerb")}return{height:m,kind:f,d:x,run:v}}function Sy(){let s=[];for(let e of Ku){let t=[{at:e.from,kind:"end"},...e.crossings.map(n=>({at:n,kind:"crossing"})),{at:e.to,kind:"end"}];for(let n=0;n+1<t.length;n++){let i=t[n],r=t[n+1];s.push({axis:e.axis,line:e.line,start:i.kind,end:r.kind,from:i.kind==="crossing"?i.at+Bt.halfWidth:i.at,to:r.kind==="crossing"?r.at-Bt.halfWidth:r.at})}}return s}var wy={x:0,z:Math.PI/2};function uo(s,e,t,n,i){let r=(n+i)/2;return{id:s,axis:e,line:t,from:n,to:i,x:e==="x"?r:t,z:e==="x"?t:r,angle:wy[e]}}function Gp(){let s=[];for(let e of Bt.xs)for(let t of Bt.zs)s.push({id:"street-crossing",axis:null,line:null,from:null,to:null,x:e,z:t,angle:0});for(let e of Sy()){let t=e.from,n=e.to;e.start==="crossing"&&(s.push(uo("street-zebra",e.axis,e.line,t,t+ki)),t+=ki),e.end==="crossing"&&(s.push(uo("street-zebra",e.axis,e.line,n-ki,n)),n-=ki);let i=e.start==="crossing",r=Math.floor((n-t)/ki+1e-9),a=Math.round(n-t-r*ki);for(let o=0;o<r;o++){let l=o%3===2?"street-drain":"street-plain";s.push(i?uo(l,e.axis,e.line,t+o*ki,t+(o+1)*ki):uo(l,e.axis,e.line,n-(o+1)*ki,n-o*ki))}for(let o=0;o<a;o++){let l=i?n-a+o:t+o;s.push(uo("street-fill",e.axis,e.line,l,l+_y))}for(let[o,l,c]of[[e.start,e.from,-1],[e.end,e.to,1]])o==="end"&&s.push({id:"street-end",axis:e.axis,line:e.line,from:l,to:l,x:e.axis==="x"?l:e.line,z:e.axis==="x"?e.line:l,angle:e.axis==="x"?c>0?0:Math.PI:c>0?-Math.PI/2:Math.PI/2})}return s}function Vp(){let{xs:s,zs:e,halfWidth:t,minX:n,maxX:i,minZ:r,maxZ:a,quayZ:o}=Bt,l=(h,u,d)=>{let p=[],x=u;for(let v of h)p.push([x,v-t]),x=v+t;return p.push([x,d]),p},c=[];for(let[h,u]of l(s,n,i))for(let[d,p]of l(e,r,o))c.push({kind:"plot",x0:h,z0:d,x1:u,z1:p,top:gt.ground,bottom:Js.top});return c.push({kind:"quay",x0:n,z0:o,x1:i,z1:a,top:gt.ground,bottom:Js.top}),c}var Oc=Ht.line+Ht.droppedRun+Bp;function Ey(s,e){let t=Math.min(s.x,e.x),n=Math.max(s.x,e.x),i=Math.min(s.z,e.z),r=Math.max(s.z,e.z);return Ku.some(a=>a.axis==="x"?i<=a.line+Oc&&r>=a.line-Oc&&n>=a.from&&t<=a.to:t<=a.line+Oc&&n>=a.line-Oc&&r>=a.from&&i<=a.to)}function Ty(s,e){return kc.some(t=>t.axis==="x"?Math.abs(s-t.at)<=Ht.opening&&Math.abs(e-t.line)<=Bt.halfWidth:Math.abs(e-t.at)<=Ht.opening&&Math.abs(s-t.line)<=Bt.halfWidth)}function Ju(s,e,t=!1){if(!Ey(s,e))return!0;let n=Math.max(1,Math.ceil(Math.hypot(e.x-s.x,e.z-s.z)/yy));for(let i=0;i<=n;i++){let r=i/n,a=s.x+(e.x-s.x)*r,o=s.z+(e.z-s.z)*r,l=Yu(a,o);if(!(!l||l.kind==="pavement"&&l.d-l.run>=Bp)&&!(t&&Ty(a,o)))return!1}return!0}var zc=null;function Wp(){if(zc)return zc;let s=Ht.pavementLine,e=[],t=[];for(let n of Bt.xs)for(let i of Bt.zs)for(let[r,a]of[[1,1],[1,-1],[-1,1],[-1,-1]])e.push({x:n+r*s,z:i+a*s,kind:"corner"});for(let n of kc){let i=e.length;for(let r of[-1,1])e.push(n.axis==="x"?{x:n.at,z:n.line+r*s,kind:"zebra"}:{x:n.line+r*s,z:n.at,kind:"zebra"});e[i].pair=i+1,e[i+1].pair=i,t.push({a:i,b:i+1,zebra:!0,length:2*s})}for(let n=0;n<e.length;n++)for(let i=n+1;i<e.length;i++){if(e[n].pair===i)continue;let r=Math.hypot(e[n].x-e[i].x,e[n].z-e[i].z);r<ju&&Ju(e[n],e[i])&&t.push({a:n,b:i,zebra:!1,length:r})}return e.forEach(Object.freeze),t.forEach(Object.freeze),zc=Object.freeze({nodes:Object.freeze(e),links:Object.freeze(t)}),zc}var Zu={agent:[[-8,-8,8,8]],memory:[[-11.4,-10.4,11.4,8.4]],integrations:[[-11.4,-4.9,-4.6,4.9],[4.6,-4.9,11.4,4.9],[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]],missions:[[-13.4,-8.9,13.4,6.9]],infra:[[-10.7,-9.4,10.7,3.4],[-10,-2.6,10,8.5]]},Ay={agent:[23,23],memory:[26,22],integrations:[26,16],missions:[30,21],infra:[25,23],graph:[24,24],operations:[9,9]},$u={agent:[[5.9,8,13],[5.4,13,29],[4.7,29,42.5],[4.2,42.5,61.5],[3.6,61.5,72.2],[2.6,72.2,87]]};function qp(s,e,t){for(let n of t){let i=Ay[n.id];if(i&&Math.abs(s-n.x)<i[0]/2&&Math.abs(e-n.z)<i[1]/2)return .88}return 0}function Ry(s,e,t){for(let n of t){let i=s-n.x,r=e-n.z;if(n.id==="graph"&&Math.hypot(i,r)<10.2||n.id==="operations"&&Math.hypot(i,r)<3||(Zu[n.id]||[]).some(([a,o,l,c])=>i>a&&i<l&&r>o&&r<c))return!0}return Uc.some(n=>Math.abs(s-n.x)<6&&Math.abs(e-n.z)<5)}function ys(s,e){return Dn.find(t=>Math.abs(s-t.x)<t.width/2&&Math.abs(e-t.z)<t.depth/2)}function Ln(s,e){if(s>=-81.5&&s<=-78.5&&e>=46&&e<=52)return(52-e)/3;if(s>=-78.3&&s<=-75.3&&e>=46&&e<=52)return Math.min(2,Math.ceil((52-e)*2)/6);if(s>=-82&&s<=-75&&e>=34&&e<46)return 2;if(s>=-80&&s<=-77&&e>=28&&e<34)return(e-28)/3;if(ys(s,e))return gt.room;for(let n of ri){let i=!!n.angle&&Math.abs(n.angle)!==Math.PI;if(Math.abs(s-n.platformX)<(i?2:4.5)&&Math.abs(e-n.platformZ)<(i?4.5:2))return gt.pavement+.3}let t=Yu(s,e);return t?t.height:e>=74&&e<=80&&s>=-80&&s<=80?gt.quay:gt.ground}function Qu(s,e,t,n,i){if(Math.abs(s)>84||e<-90||e>79)return!1;let r=ys(s,e),a=ys(t.x,t.z);if(r||a){let o=r||a;return!(r?.id!==a?.id&&(Math.abs(s-o.x)>1.15||Math.abs(t.x-o.x)>1.15||Math.min((e-o.doorZ)*o.front,(t.z-o.doorZ)*o.front)<-.9||!n(o.id))||r&&(Math.abs(s-o.x)>5.55||(e-o.z)*o.front<-5.55))}return!Ry(s,e,i)}function Bc(s,e=new Date){let t=s==="day"?12:s==="evening"?18.5:s==="night"?0:e.getHours()+e.getMinutes()/60;return{hour:t,amount:Math.max(0,Math.min(1,Math.sin((t-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(t-18.5)/2)}}var Xp=[[[-67,-77],[-18,-77],[-18,-32],[-67,-32]],[[-18,-77],[18,-77],[18,-32],[-18,-32]],[[18,-77],[67,-77],[67,-32],[18,-32]],[[18,13],[18,-32],[67,-32],[67,13]],[[-67,-77],[-18,-77],[-18,-32],[-67,-32]]];function jp(s,e,t){let n=!1,i=0,r=!1,a=!1,o=0,l=!1,c=new vt;c.name="city-life",s.add(c);let h=t.traffic,u=[],d=Dp({routes:Xp,obstacles:t.obstacles||[],blocked:(be,ue,te)=>{let pe=u[be.route],ce=pe?{...pe,x:ue,z:te}:null;return pe?!h.clear(pe,ce,ce):!1}}),p=new Set,x=new Set,v=new Set,m=[],f=new AbortController,y=[],T=[],g=be=>(p.add(be),be),_=be=>(x.add(be),be),w=g(new Kn(1,.013,5,64));for(let be of e){let ue=_(new Vt({color:7452878,transparent:!0,opacity:.08,depthWrite:!1,toneMapped:!1})),te=new st(w,ue);te.rotation.x=Math.PI/2,te.position.set(be.x,.6,be.z),te.scale.setScalar(be.radius*1.12),c.add(te),T.push({id:be.id,ring:te,material:ue,state:"unknown",eventUntil:0,color:new Le(7438733)})}let A=new Map,b=[],C=new Map,S=Date.now(),P=3e3,M=e.find(be=>be.id==="agent"),F=new B(0,0,1),I=new B,N={memory:12690431,graph:16765844,infra:7978495,integrations:7728086,missions:10268415,operations:16758915};if(M){for(let be of e)if(be!==M){let ue=new B(M.x,M.height+3,M.z),te=new B(be.x,be.height+4,be.z),pe=ue.clone().lerp(te,.5);pe.y=Math.max(ue.y,te.y)+12;let ce=new mi(ue,pe,te),ne=_(new Ci({color:N[be.id],transparent:!0,opacity:0,depthWrite:!1,blending:tn,toneMapped:!1})),de=new ji(g(new _t().setFromPoints(ce.getPoints(48))),ne);de.visible=!1,c.add(de),A.set(be.id,{curve:ce,line:de})}}let H=g(new Kn(1,.06,5,40,Math.PI*.95));for(let be=0;be<12;be++){let ue=[];for(let te=0;te<4;te++){let pe=_(new Vt({color:10345983,transparent:!0,opacity:0,depthWrite:!1,blending:tn,toneMapped:!1})),ce=new st(te===3?w:H,pe);ce.visible=!1,c.add(ce),ue.push(ce)}b.push({at:-1/0,waves:ue,link:null})}function D(be,ue){if(!l||t.active?.()===!1||be.at<S||ue-be.at>P||be.at>ue||!["started","succeeded","failed","sanitized","progress"].includes(be.state))return;let te=be.to==="agent",pe=te?be.from:be.to;if(!te&&be.from!=="agent"||!A.has(pe))return;let ce=pe+":"+te;if(be.at-(C.get(ce)??-1/0)<450)return;C.set(ce,be.at);let ne=b.reduce((de,we)=>de.at<we.at?de:we);Object.assign(ne,{at:be.at,link:A.get(pe),incoming:te,from:be.from,to:be.to,state:be.state}),ne.waves.forEach(de=>de.material.color.setHex(be.state==="failed"?16737881:N[pe]))}function V(be){let ue=Date.now();A.forEach(te=>{te.line.visible=!1});for(let te of b){(!be||t.active?.()===!1)&&(te.at=-1/0);let pe=(ue-te.at)/P,ce=pe>=0&&pe<1;if(te.waves.forEach(de=>{de.visible=!1}),!ce||!te.link)continue;te.link.line.visible=!0,te.link.line.material.opacity=.12*Math.sin(pe*Math.PI);for(let de=0;de<3;de++){let we=(pe-de*.075)/.75;if(we<0||we>1)continue;let Ve=te.incoming?1-we:we,Ke=te.waves[de];te.link.curve.getPoint(Ve,Ke.position),te.link.curve.getTangent(Ve,I),te.incoming&&I.negate(),Ke.quaternion.setFromUnitVectors(F,I),Ke.rotateZ(Math.PI*.525),Ke.scale.setScalar(1.2+Math.sin(we*Math.PI)*3.5),Ke.material.opacity=Math.sin(we*Math.PI)*.8,Ke.visible=!0}let ne=(pe-.74)/.26;if(ne>0){let de=te.waves[3];te.link.curve.getPoint(te.incoming?0:1,de.position),de.rotation.set(-Math.PI/2,0,0),de.scale.setScalar(1+ne*8),de.material.opacity=(1-ne)*.6,de.visible=!0}}}let k=g(new fn(7,7)),E=_(new yt({transparent:!0,depthWrite:!1,blending:tn,uniforms:{color:{value:new Le(5495284)}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"varying vec2 vUv;uniform vec3 color;void main(){float r=length(vUv-.5)*2.;float a=exp(-r*r*5.)*.23*(1.-smoothstep(.65,1.,r));gl_FragColor=vec4(color,a);}"})),Z=g(new ks(.42,1.3,12,1,!0)),W=_(new Vt({color:9629439,transparent:!0,opacity:.22,depthWrite:!1,toneMapped:!1,blending:tn}));Xp.forEach((be,ue)=>{let te=new vt;te.name="city-white-robot-"+(ue+1);let pe=new vt;te.add(pe),c.add(te);let ce=new st(k,E);ce.rotation.x=-Math.PI/2,ce.position.y=.08,c.add(ce);let ne=new st(Z,W);ne.rotation.z=Math.PI,ne.position.y=-.45,te.add(ne),te.visible=!1,ce.visible=!1,y.push({shadow:t.shadows?.claim()||null,root:te,body:pe,glow:ce,jet:ne,agent:d.agents[ue],lastX:0,lastZ:0,phase:0,gait:0})});function Q(be){be.traverse(ue=>{if(ue.isMesh){p.add(ue.geometry);for(let te of Array.isArray(ue.material)?ue.material:[ue.material]){x.add(te);for(let pe of Object.values(te))pe?.isTexture&&v.add(pe)}}})}function Fe(){p.forEach(ue=>ue.dispose()),x.forEach(ue=>ue.dispose());let be=new Set;v.forEach(ue=>{ue.image&&be.add(ue.image),ue.dispose()}),be.forEach(ue=>ue.close?.()),p.clear(),x.clear(),v.clear()}async function Ne(){let be=setTimeout(()=>f.abort(),15e3);try{let ue=await fetch(t.robotURL,{signal:f.signal});if(!ue.ok)throw Error("Robot unavailable");let te=await ue.arrayBuffer();if(n)return;let{scene:pe}=await new yi().parseAsync(te,"");if(Q(pe),n){Fe();return}let ce=new cn().setFromObject(pe),ne=ce.getSize(new B),de=Math.max(ne.x,ne.y,ne.z);if(!Number.isFinite(de)||de<=0)throw Error("Invalid robot bounds");pe.scale.setScalar(6/de),pe.updateMatrixWorld(!0),ce.setFromObject(pe);let we=ce.getCenter(new B);pe.position.set(-we.x,-ce.min.y,-we.z),pe.rotation.y=-Math.PI/2,pe.updateMatrixWorld(!0),ce.setFromObject(pe),ce.expandByScalar(.4);let Ve=ts({min:ce.min.toArray(),max:ce.max.toArray()});pe.traverse(Ke=>{Ke.isMesh&&(Ke.castShadow=!1,Ke.receiveShadow=!0)}),y.forEach((Ke,it)=>{if(Ke.body.add(pe.clone(!0)),Ke.root.visible=!0,Ke.glow.visible=!0,h){let et=Ke.agent,ht=null;for(let J=0;J<600&&!ht;J++)ht=h.register("patrol-"+it,Ve,{x:et.x,y:1.6,z:et.z,heading:et.heading},1),ht||d.seek(et,et.s+1);ht?(u[it]=ht,t.society?.patrol(ht.id,ht,Ke.root,(J,Ut)=>{et.held=J,Ut&&d.adopt(et,Ut)})):(Ke.root.visible=!1,Ke.glow.visible=!1,et.held=!0)}}),r=!0,le(0,!1)}catch(ue){n||(a=!0,t.onError?.(ue))}finally{clearTimeout(be)}}function ze(be){m.forEach(te=>{te.dispose(),x.delete(te)}),m.length=0,be.children.find(te=>te.userData.district==="operations")?.getObjectByName("signal")?.traverse(te=>{if(!te.isMesh)return;let pe=ce=>{let ne=_(ce.clone());return m.push(ne),ne};te.material=Array.isArray(te.material)?te.material.map(pe):pe(te.material)})}function se(be,ue=[]){for(let pe of T){let ce=be.find(ne=>ne.id===pe.id);pe.state=!ce||ce.stale?"unknown":ce.state,pe.color.setHex(pe.state==="error"?16730430:pe.state==="running"?7400403:pe.state==="unknown"?7438733:7452878),pe.material.color.copy(pe.color)}let te=Date.now();for(let pe of[...ue].reverse())if(pe.id>o&&(D(pe,te),l&&pe.at>=S&&t.active?.()!==!1&&t.society?.event(pe),te-pe.at<6e3)){let ce=T.find(ne=>ne.id===pe.district);ce&&(ce.eventUntil=pe.at+5e3)}o=Math.max(o,...ue.map(pe=>pe.id))}function le(be,ue){if(n)return;l=ue,V(ue);let te=ue?Math.min(.1,Math.max(0,be)):0,pe=h?d.agents.map(ce=>({...ce})):null;ue&&te>0&&(i+=te,d.step(te));for(let[ce,ne]of y.entries()){let de=ne.agent,we=Math.hypot(de.vx,de.vz),Ve=(!ue||de.held)&&u[ce]?u[ce]:de,Ke=Math.hypot(Ve.x-ne.lastX,Ve.z-ne.lastZ),it=Ke>.5?0:Ke;ne.lastX=Ve.x,ne.lastZ=Ve.z,ne.phase+=it*1.4,ne.gait=Ft.damp(ne.gait,Math.min(1,it/Math.max(te,1e-4)/1.2),6,te);let et=ne.gait,ht=1-et;ne.root.position.set(Ve.x,1.6+Math.sin(ne.phase)*.14*et+Math.sin(i*.7+ce*2.1)*.03*ht,Ve.z),ne.root.rotation.y=Ve.heading;let J=(de.vx*de.rx+de.vz*de.rz)/Math.max(1,we);ne.body.rotation.z=Math.sin(i*.85+ce)*.035*et-J*.08,ne.body.rotation.x=-.035+Math.sin(i*1.1+ce)*.018*et-Math.min(.09,we*.015),ne.glow.position.x=Ve.x,ne.glow.position.z=Ve.z,ne.jet.scale.y=1+Math.sin(i*4+ce)*.12+we*.04;let Ut=Ln(ne.glow.position.x,ne.glow.position.z);if(ne.glow.position.y=Ut+.08,ne.shadow&&(ne.root.visible?ne.shadow.set(ne.glow.position.x,Ut,ne.glow.position.z,1.1,.3):ne.shadow.hide()),ue&&u[ce]&&!de.held){let ut=u[ce],O={x:de.x,y:1.6,z:de.z,heading:de.heading};ne.escape&&i<ne.escape.until?(O.x=ut.x+ne.escape.x*te*1.8,O.z=ut.z+ne.escape.z*te*1.8,O.heading=ut.heading):ne.escape=null,!h.clear(ut,ut,O,!1)&&h.clear(ut,ut,{...O,heading:ut.heading},!1)&&(O.heading=ut.heading);let R=Math.hypot(O.x-ut.x,O.z-ut.z);h.propose(ut,O,(re,oe)=>{if(ne.wait=re&&R>.002?0:(ne.wait||0)+te,ne.escape&&re&&d.adopt(de,oe),ne.wait>2.5){let xe=Math.sin(oe.heading),Ue=Math.cos(oe.heading);for(let[ke,G]of[[Ue,-xe],[-Ue,xe],[xe,Ue],[-xe,-Ue]])if(h.clear(oe,oe,{...oe,x:oe.x+ke*2.5,z:oe.z+G*2.5})){ne.escape={x:ke,z:G,until:i+2};break}ne.wait=0}if(re)de.trafficWait=0,de.heading=oe.heading;else{let xe=(de.trafficWait||0)+te;Object.assign(de,pe[ce]),de.speed=0,de.vx=de.vz=0,de.trafficWait=xe,xe>2.5&&(de.state!=="turn"?(de.dir=-de.dir,de.lane=-de.lane,de.laneTarget=2.2,de.state="turn",de.turns++):de.state="cruise",de.trafficWait=0)}ne.root.position.x=oe.x,ne.root.position.z=oe.z,ne.root.rotation.y=oe.heading,ne.glow.position.x=oe.x,ne.glow.position.z=oe.z})}}for(let ce of T){let ne=ue?.5+.5*Math.sin(i*(ce.state==="error"?3.2:1.8)):.5,de=ce.state==="error"||ce.state==="running"||ce.eventUntil>Date.now();if(ce.material.opacity=ce.state==="unknown"?.03:de?.2+ne*.46:.065,ce.ring.scale.setScalar(e.find(we=>we.id===ce.id).radius*1.12*(1+(de&&ue?ne*.035:0))),ce.id==="operations")for(let we of m)we.color.copy(ce.color),we.emissive?.copy(ce.color),we.emissiveIntensity=ce.state==="error"?1.1+ne*2.2:ce.state==="unknown"?.08:.55}}function De(){n||(n=!0,f.abort(),t.signal?.removeEventListener("abort",De),c.removeFromParent(),Fe(),y.length=0,m.length=0,u.forEach(be=>h.remove(be.id)))}return t.signal?.addEventListener("abort",De,{once:!0}),t.signal?.aborted?De():Ne(),{update:le,setData:se,attachLandmarks:ze,dispose:De,stats:()=>({robots:r?y.filter(be=>be.root.visible).length:0,robotError:a,time:i,navigation:{...d.stats(),gaits:y.map(be=>+(be.gait||0).toFixed(2)),held:y.map(be=>!!be.agent.held),enabled:y.map(be=>!!u[y.indexOf(be)]?.enabled)},positions:y.map(be=>be.root.position.toArray()),transmissions:b.filter(be=>be.waves.some(ue=>ue.visible)).map(be=>({from:be.from,to:be.to,state:be.state,age:Date.now()-be.at})),signals:T.map(be=>({id:be.id,state:be.state,intensity:be.material.opacity}))})}}var fo='Geist, "Segoe UI", system-ui, sans-serif',Hc=13,Cy=s=>{let e=2166136261;for(let t=0;t<s.length;t++)e=Math.imul(e^s.charCodeAt(t),16777619);return(e>>>0).toString(16).padStart(8,"0")},ed={vertex:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:`uniform sampler2D map;uniform float time,glitch,fade,gain;uniform vec3 tint;varying vec2 vUv;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){vec2 uv=vUv;float row=floor(uv.y*44.0),frame=floor(time*24.0);
float g=glitch*step(0.7,hash(row+frame*0.37));uv.x+=(hash(row*3.1+frame)-0.5)*0.14*g;
float split=0.0025+glitch*0.02;vec4 c=texture2D(map,uv);
float r=texture2D(map,uv+vec2(split,0.0)).r,b=texture2D(map,uv-vec2(split,0.0)).b;
float a=max(c.a,max(texture2D(map,uv+vec2(split,0.0)).a,texture2D(map,uv-vec2(split,0.0)).a));
float lines=0.84+0.16*sin(uv.y*420.0-time*9.0);float flicker=0.95+0.05*sin(time*31.0)*sin(time*7.3);
float bx=(fract(uv.y*0.55-time*0.11)-0.5)*22.0;float band=0.35*exp(-bx*bx);
float edge=smoothstep(0.0,0.05,uv.x)*smoothstep(0.0,0.05,1.0-uv.x)*smoothstep(0.0,0.09,uv.y)*smoothstep(0.0,0.09,1.0-uv.y);
vec3 col=vec3(r,c.g,b)*tint*(lines*flicker+band)*gain;gl_FragColor=vec4(col*a*fade*edge,1.0);}`},Py={vertex:"varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",fragment:`uniform float time,strength;uniform vec3 base,top;varying vec2 vUv;varying vec3 vNormal,vView;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){float h=vUv.y;float rim=pow(clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0),1.5);
float fall=pow(clamp(1.0-h,0.0,1.0),1.35)*0.75+0.25*clamp(1.0-h,0.0,1.0);float scan=0.7+0.3*sin(h*64.0-time*5.5);
float noise=0.85+0.15*hash(floor(h*96.0)+floor(time*18.0));float shimmer=0.8+0.2*sin(vUv.x*40.0+time*2.0);
float a=(0.28+0.72*rim)*fall*scan*noise*shimmer*strength;gl_FragColor=vec4(mix(base,top,h)*a,1.0);}`},Iy={vertex:ed.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float r=length(vUv-0.5)*2.0;float rings=smoothstep(0.35,1.0,sin(r*16.0-time*2.6));
float core=exp(-r*r*7.0);float a=(core*1.2+pow(max(0.0,1.0-r),1.8)*(0.25+0.75*rings)*0.6)*strength*(1.0-smoothstep(0.85,1.0,r));
gl_FragColor=vec4(color*a,1.0);}`},Kp={vertex:ed.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float dash=step(0.45,fract(vUv.x*28.0+time*0.35));float pulse=0.75+0.25*sin(vUv.x*6.2831*3.0-time*4.0);
gl_FragColor=vec4(color*dash*pulse*strength,1.0);}`},Dy={vertex:`attribute float seed;uniform float time,pointScale;varying float vLife;
void main(){float speed=0.55+seed*0.9;float life=fract(seed*3.17+time*speed/13.0);float y=life*13.0;
float ang=seed*6.2831+time*(0.35+seed*0.4)+life*2.2;float rad=mix(1.8,6.8,life)*(0.3+0.7*fract(seed*7.13));
vec4 mv=modelViewMatrix*vec4(cos(ang)*rad,y,sin(ang)*rad,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.09+0.16*fract(seed*3.7))*pointScale/max(1.0,-mv.z);vLife=life;}`,fragment:`uniform vec3 color;uniform float strength;varying float vLife;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,2.0)*sin(vLife*3.1416)*strength;gl_FragColor=vec4(color*a,1.0);}`};function ea(s,e,t={}){return new yt({uniforms:e,vertexShader:s.vertex,fragmentShader:s.fragment,transparent:!0,depthWrite:!1,blending:tn,side:Kt,...t})}function Ly(s,e=24){let t=[],n=new Set;for(let i of Array.isArray(s)?s:[]){if(typeof i!="string")continue;let r=i.replace(/[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u2028-\u202e\u2066-\u2069]/g," ").replace(/\s+/g," ").trim().slice(0,140);if(!(r.length<4||n.has(r))&&(n.add(r),t.push(r),t.length>=e))break}return t}function Yp(s,e,t,n){let i=[],r=c=>s.measureText(c).width<=t,a="";for(let c of e.split(" ")){if(i.length>=n)break;let h=a?a+" "+c:c;if(r(h)){a=h;continue}if(a&&(i.push(a),a="",i.length>=n))break;let u=c;for(;!r(u)&&i.length<n;){let d=u.length-1;for(;d>1&&!r(u.slice(0,d));)d--;i.push(u.slice(0,d)),u=u.slice(d)}a=u}a&&i.length<n&&(i.push(a),a="");let o=i.join("").replace(/\s/g,"").length,l=e.replace(/\s/g,"").length;if(o<l&&i.length){let c=i[i.length-1].replace(/[\s,.;:]+$/,"");for(;c.length&&!r(c+"\u2026");)c=c.slice(0,-1);i[i.length-1]=c+"\u2026"}return i.length?i:[""]}function Jp(s,e,t={}){let n=t.roof??21,i=String(t.label||"MEMORY").toUpperCase(),r=new vt;r.name="memory-hologram",r.position.set(e.x,n,e.z),s.add(r);let a=[],o=[],l=[],c=(te,pe)=>(a.push(te),o.push(pe),new st(te,pe)),h=new Le(6544639),u=new Le(10980351),d=new Le(14218751),p={time:{value:0}},x=!1,v=0,m=[],f=[],y=0,T="none",g=!1,_=c(new fn(11,11),ea(Iy,{time:p.time,strength:{value:.9},color:{value:h}}));_.rotation.x=-Math.PI/2,_.position.y=.18,r.add(_);let w=c(new ds(7.4,2.3,Hc,56,1,!0),ea(Py,{time:p.time,strength:{value:.32},base:{value:h},top:{value:u}}));w.position.y=Hc/2+.3,r.add(w);let A=c(new Kn(7.5,.055,6,128),ea(Kp,{time:p.time,strength:{value:1.6},color:{value:h}}));A.rotation.x=Math.PI/2,A.position.y=Hc+.3,r.add(A);let b=c(new Kn(2.6,.05,6,96),ea(Kp,{time:p.time,strength:{value:2},color:{value:d}}));b.rotation.x=Math.PI/2,b.position.y=.55,r.add(b);let C=new Dr(1.35,1),S=new Pa(C),P=new Ci({color:10217983,transparent:!0,opacity:.75,blending:tn,depthWrite:!1,toneMapped:!1}),M=new Os(S,P);M.position.y=4.6,r.add(M),a.push(C,S),o.push(P);let F=new Vt({color:4175871,transparent:!0,opacity:.12,blending:tn,depthWrite:!1,toneMapped:!1}),I=c(new Dr(1.1,1),F);M.add(I);let N=160,H=new Float32Array(N);for(let te=0;te<N;te++)H[te]=te*.618033988749895%1;let D=new _t;D.setAttribute("position",new Rt(new Float32Array(N*3),3)),D.setAttribute("seed",new Rt(H,1)),D.boundingSphere=new Cn(new B(0,Hc/2,0),12);let V=ea(Dy,{time:p.time,pointScale:{value:800},color:{value:d},strength:{value:.9}}),k=new yn(D,V);k.frustumCulled=!1,r.add(k),a.push(D),o.push(V);let E=new gi(6478079,26,46,2);E.position.y=6,r.add(E);function Z(te,pe,ce,ne){let de=document.createElement("canvas");de.width=ce,de.height=ne;let we=de.getContext("2d"),Ve=new zs(de);Ve.colorSpace=sn,Ve.generateMipmaps=!1,Ve.minFilter=rn,l.push(Ve);let Ke=ea(ed,{map:{value:Ve},time:p.time,glitch:{value:0},fade:{value:0},gain:{value:1.35},tint:{value:new Le(16777215)}},{side:ti}),it=c(new fn(te,pe),Ke);return{canvas:de,context:we,texture:Ve,material:Ke,mesh:it,text:"",targetFade:1,next:0}}let W=Z(17.5,8.55,1024,500),Q=[Z(8.6,2.1,640,156),Z(8.6,2.1,640,156)];W.mesh.position.y=10.4,r.add(W.mesh),Q.forEach((te,pe)=>{te.mesh.position.y=pe?14.4:5.9,te.orbit=pe?-.22:.27,te.angle=pe*2.3,r.add(te.mesh)});function Fe(te,pe,ce){let{context:ne,canvas:de}=W,we=de.width,Ve=de.height;ne.clearRect(0,0,we,Ve),ne.fillStyle="rgba(48,150,214,0.14)",ne.beginPath(),ne.roundRect(14,14,we-28,Ve-28,22),ne.fill(),ne.strokeStyle="rgba(150,230,255,0.85)",ne.lineWidth=3;for(let[Ke,it,et,ht]of[[18,18,1,1],[we-18,18,-1,1],[18,Ve-18,1,-1],[we-18,Ve-18,-1,-1]])ne.beginPath(),ne.moveTo(Ke,it+ht*42),ne.lineTo(Ke,it),ne.lineTo(Ke+et*42,it),ne.stroke();ne.fillStyle="rgba(150,230,255,0.55)",ne.fillRect(48,108,we-96,2),ne.font="600 27px "+fo,ne.textBaseline="middle",ne.fillStyle="rgba(160,232,255,0.92)",ne.textAlign="left",ne.fillText("\u258C "+i+(ce?"  \xB7  "+String(pe+1).padStart(2,"0")+" / "+String(ce).padStart(2,"0"):""),50,72),ne.textAlign="right",ne.font="500 25px "+fo,ne.fillStyle="rgba(190,150,255,0.85)",ne.fillText("0x"+Cy(te||i).toUpperCase(),we-52,72),ne.textAlign="left",ne.shadowColor="rgba(120,225,255,0.9)",ne.shadowBlur=16,te?(ne.font="600 54px "+fo,ne.fillStyle="rgba(232,250,255,0.97)",Yp(ne,te,we-110,4).forEach((it,et)=>ne.fillText(it,54,172+et*72))):(ne.font="600 40px "+fo,ne.fillStyle="rgba(180,235,255,0.7)",ne.fillText("\u25AE \u25AE \u25AF \u25AE \u25AF \u25AF \u25AE \u25AF \u25AE \u25AE \u25AF \u25AE",54,250)),ne.shadowBlur=0,W.texture.needsUpdate=!0}function Ne(te,pe){let{context:ce,canvas:ne}=te,de=ne.width,we=ne.height;ce.clearRect(0,0,de,we),ce.fillStyle="rgba(48,150,214,0.12)",ce.beginPath(),ce.roundRect(6,6,de-12,we-12,14),ce.fill(),ce.fillStyle="rgba(190,150,255,0.8)",ce.fillRect(20,26,6,we-52),ce.font="600 42px "+fo,ce.textBaseline="middle",ce.textAlign="left",ce.shadowColor="rgba(160,140,255,0.9)",ce.shadowBlur=12,ce.fillStyle="rgba(236,240,255,0.95)",ce.fillText(pe?Yp(ce,pe,de-70,1)[0]:"\u25AF \u25AE \u25AF \u25AE \u25AF",42,we/2),ce.shadowBlur=0,te.texture.needsUpdate=!0}function ze(){return m.length?(y>=f.length&&(f=m.map((te,pe)=>pe).sort(()=>Math.random()-.5),y=0),m[f[y++]]):""}let se=.8,le=[3.5,6.5];function De(te){let pe=ze();W.text=pe,v++,Fe(pe,m.indexOf(pe),m.length),W.material.uniforms.glitch.value=te?1:0,W.material.uniforms.fade.value=te?.35:1,se=x?12:4.5+Math.random()*3}function be(te,pe){let ce=Q[te],ne=ze();ce.text=ne,Ne(ce,ne.length>46?ne.slice(0,44).replace(/\s+\S*$/,"")+"\u2026":ne),ce.material.uniforms.glitch.value=pe?.7:0,ce.material.uniforms.fade.value=pe?.3:1,le[te]=x?15:6+Math.random()*4}Fe("",0,0),Q.forEach(te=>Ne(te,"")),document.fonts?.ready?.then(()=>{g||(Fe(W.text,m.indexOf(W.text),m.length),Q.forEach(te=>Ne(te,te.text)))});let ue=new B;return{group:r,setTexts(te,pe="live"){let ce=Ly(te),ne=ce.length!==m.length||ce.some((de,we)=>de!==m[we]);m=ce,T=ce.length?pe:"none",ne&&(f=[],y=0,(!W.text||!m.includes(W.text))&&(se=Math.min(se,.6)))},setPointScale(te){V.uniforms.pointScale.value=te},setReducedMotion(te){x=!!te},update(te,pe,ce){if(g)return;let ne=ce&&!x,de=ne?Math.min(.1,Math.max(0,te)):0;p.time.value+=de,se-=te,se<=0&&De(ne),le.forEach((we,Ve)=>{le[Ve]=we-te,le[Ve]<=0&&be(Ve,ne)});for(let we of[W,...Q]){let Ve=we.material.uniforms;Ve.glitch.value=Math.max(0,Ve.glitch.value-te*2.4),Ve.fade.value=Math.min(1,Ve.fade.value+te*1.8),we.mesh.quaternion.copy(pe.quaternion)}Q.forEach(we=>{we.angle+=we.orbit*de,we.mesh.position.x=Math.cos(we.angle)*6.4,we.mesh.position.z=Math.sin(we.angle)*6.4,ue.copy(we.mesh.position).add(r.position).sub(pe.position).normalize();let Ve=-(ue.x*Math.cos(we.angle)+ue.z*Math.sin(we.angle));we.material.uniforms.gain.value=1.35*Ft.clamp(.35+Ve*.9,.15,1.2)}),W.mesh.position.y=9.6+Math.sin(p.time.value*.9)*.22,M.rotation.y+=de*.7,M.rotation.x+=de*.31,I.rotation.y-=de*1.1,A.rotation.z+=de*.18,b.rotation.z-=de*.42,E.intensity=26+Math.sin(p.time.value*2.1)*6+W.material.uniforms.glitch.value*22},stats:()=>({artifacts:m.length,source:T,switches:v,shown:W.text?m.indexOf(W.text):-1}),dispose(){g||(g=!0,r.removeFromParent(),a.forEach(te=>te.dispose()),o.forEach(te=>te.dispose()),l.forEach(te=>te.dispose()),E.dispose())}}}var Fy=`uniform sampler2D tDiffuse;uniform float time,vignette,grain,aberration,streak,shafts,letterbox,aspect,contrast,saturation;
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
gl_FragColor=vec4(c,1.0);}`;function Zp(s){let e={tDiffuse:{value:null},time:s,vignette:{value:.5},grain:{value:.03},aberration:{value:.0024},streak:{value:.16},shafts:{value:0},letterbox:{value:0},aspect:{value:1.7777777777777777},contrast:{value:.22},saturation:{value:1.08},lightPos:{value:new Be(.5,.5)},shaftColor:{value:new Le(12376319)},shadowTint:{value:new Le(6686)},highlightTint:{value:new Le(1444864)}},t=new Zr(new yt({uniforms:e,vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:Fy}));t.enabled=!1;let n=new B,i=new Le(16761992),r=new Le(12376319),a=!1;return{pass:t,setCinematic(o){a=!!o},update(o,l,c,{day:h,dusk:u,cloud:d,animated:p}){e.aspect.value=l.aspect;let x=a?1:0;e.letterbox.value=p?e.letterbox.value+(x-e.letterbox.value)*Math.min(1,o*2.2):x,n.copy(c).multiplyScalar(1e3).add(l.position).project(l);let v=(n.x+1)/2,m=(n.y+1)/2,f=n.z<1?Math.max(0,1-Math.max(0,Math.abs(v-.5)-.5,Math.abs(m-.5)-.5)*4):0;e.lightPos.value.set(v,m),e.shafts.value=f*(.06+u*.14+(1-h)*.03)*(1-d*.75),e.shaftColor.value.copy(r).lerp(i,u),e.shadowTint.value.setRGB(0,.018+.01*(1-h),.024+.01*(1-h)),e.highlightTint.value.setRGB(.02+u*.03,.01+u*.006,0)},stats:()=>({enabled:t.enabled,letterbox:+e.letterbox.value.toFixed(2),shafts:+e.shafts.value.toFixed(2)}),dispose(){t.material.dispose(),t.dispose?.()}}}var Ny=new Le(16767392),Uy=new Le(16758903),nd=`float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`,td="varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}",Oy="float fbm(vec2 p){float v=0.0,a=0.5;for(int i=0;i<5;i++){if(float(i)>=octaves)break;v+=a*vnoise(p);p=p*2.03+vec2(1.7,9.2);a*=0.5;}return v;}",zy=`uniform float time,day,dusk,cloud,flash,octaves;uniform vec3 zenith,horizon,haze,warm,sunDir,glow;varying vec3 vWorld;${nd}${Oy}
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
gl_FragColor=vec4(col,1.0);}`,$p={vertex:`attribute float phase,speed,size;uniform float time,pointScale,cloud;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0)*(1.0-cloud*0.85);
float k=fract(phase*3.1);vTint=k>0.86?vec3(1.0,0.84,0.66):k>0.7?vec3(0.7,0.82,1.0):vec3(0.82,0.9,1.0);}`,fragment:"varying float vAlpha;varying vec3 vTint;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vTint*a,1.0);}"},Qp={vertex:`varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,fragment:`uniform float time,day;uniform vec3 deep,shallow,sky,sunDir,sunColor,glow;uniform vec4 island;varying vec3 vWorld;
#include <fog_pars_fragment>
${nd}
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
}`},ky=`uniform float time,strength;uniform vec3 color;uniform vec4 island;varying vec3 vWorld;${nd}
void main(){vec2 p=vWorld.xz;float n=vnoise(p*0.012+vec2(time*0.017,-time*0.011))*0.6+vnoise(p*0.031-vec2(time*0.02,time*0.013))*0.4;
float mist=smoothstep(0.32,0.82,n);vec2 d2=max(abs(p-island.xy)-island.zw,0.0);float outside=smoothstep(0.0,70.0,length(d2));
float far=1.0-smoothstep(500.0,850.0,length(p));gl_FragColor=vec4(color,mist*outside*far*strength);}`,em={vertex:`attribute vec3 seed;uniform float time,pointScale;varying float vAlpha;
void main(){vec3 p=position;p.x+=sin(time*0.11*seed.x+seed.y*6.28)*9.0+time*0.35*(seed.z-0.5);p.y+=sin(time*0.13+seed.z*6.28)*3.0;
p.z+=cos(time*0.09*seed.y+seed.x*6.28)*9.0;p.x=mod(p.x+90.0,180.0)-90.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.06+0.11*seed.x)*pointScale/max(1.0,-mv.z);vAlpha=0.55+0.45*sin(time*(1.0+seed.y)+seed.z*6.28);}`,fragment:`uniform vec3 color;uniform float strength;varying float vAlpha;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,2.2)*vAlpha*strength,1.0);}`},By=`uniform float strength;uniform vec3 color;varying vec2 vUv;
void main(){float along=pow(clamp(1.0-vUv.x,0.0,1.0),2.4);float across=pow(clamp(1.0-abs(vUv.y-0.5)*2.0,0.0,1.0),1.7);gl_FragColor=vec4(color*along*across*strength,1.0);}`,Hy=`uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;
void main(){float rim=clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0);float a=pow(clamp(vUv.y,0.0,1.0),1.6)*(0.35+0.65*rim)*strength;gl_FragColor=vec4(color*a,1.0);}`,tm={vertex:"attribute float lift;varying float vLift;void main(){vLift=lift;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:"uniform vec3 haze;uniform float shade;varying float vLift;void main(){gl_FragColor=vec4(haze*mix(1.0,shade,smoothstep(0.0,0.6,vLift)),1.0);}"},nm={vertex:`attribute float phase;uniform float time,pointScale,strength;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=strength*(0.75+0.25*sin(time*(0.7+fract(phase)*1.3)+phase));gl_PointSize=clamp(2.2*pointScale/max(1.0,-mv.z),1.5,4.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(1.4,0.95,0.55)*pow(1.0-d,1.5)*vA,1.0);}"},im={vertex:"varying vec2 vUv;varying float vRim,vNear;void main(){vUv=uv;vec3 n=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vRim=abs(dot(n,normalize(-mv.xyz)));vNear=smoothstep(40.0,160.0,-mv.z);gl_Position=projectionMatrix*mv;}",fragment:"uniform vec3 color;uniform float strength;varying vec2 vUv;varying float vRim,vNear;void main(){float along=pow(clamp(1.0-vUv.y,0.0,1.0),1.6)*smoothstep(0.0,0.03,vUv.y);gl_FragColor=vec4(color*along*pow(clamp(vRim,0.0,1.0),2.5)*vNear*strength,1.0);}"},sm={vertex:`attribute float phase;uniform float time,pointScale;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=0.22+0.78*pow(max(0.0,sin(time*1.9+phase)),10.0);gl_PointSize=clamp(1.5*pointScale/max(1.0,-mv.z),2.0,14.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(3.2,0.35,0.25)*pow(1.0-d,2.0)*vA,1.0);}"};function rm(s,e={}){let t=[],n=[],i=new vt;i.name="city-atmosphere",s.add(i);let r=(fe,Pe)=>(t.push(fe),n.push(Pe),new st(fe,Pe)),a={value:0},o={value:900},l={value:0},c={value:0},h={value:.25},u={value:0},d={value:5},p=new Le(4860440),x=(e.sunDirection||new B(-70,145,85)).clone().normalize(),v=new Le(330010),m=new Le(1323596),f=new Le(8011050),y=new Le(s.fog?.color||528926),T=(fe,Pe,_e,qe={})=>new yt({uniforms:_e,vertexShader:fe,fragmentShader:Pe,...qe}),g={transparent:!0,depthWrite:!1,blending:tn},_=r(new Pi(1500,48,24),T(td,zy,{time:a,day:l,dusk:c,cloud:h,flash:u,octaves:d,glow:{value:p},zenith:{value:v},horizon:{value:m},haze:{value:y},warm:{value:f},sunDir:{value:x}},{side:mn,depthWrite:!1,fog:!1}));_.frustumCulled=!1,_.renderOrder=-10,i.add(_);let w=1400,A=new Float32Array(w*3),b=new Float32Array(w),C=new Float32Array(w),S=new Float32Array(w),P=12345,M=()=>(P=Math.imul(P,1664525)+1013904223>>>0)/4294967296;for(let fe=0;fe<w;fe++){let Pe=M()*Math.PI*2,_e=.03+M()*.97,qe=Math.sqrt(1-_e*_e);A.set([Math.cos(Pe)*qe*1450,_e*1450,Math.sin(Pe)*qe*1450],fe*3),b[fe]=M()*Math.PI*2,C[fe]=.4+M()*1.6,S[fe]=8e-4+M()*M()*.0022}let F=new _t;F.setAttribute("position",new Rt(A,3)),F.setAttribute("phase",new Rt(b,1)),F.setAttribute("speed",new Rt(C,1)),F.setAttribute("size",new Rt(S,1));let I=T($p.vertex,$p.fragment,{time:a,pointScale:o,cloud:h},{...g,fog:!1}),N=new yn(F,I);N.frustumCulled=!1,N.renderOrder=-9,i.add(N),t.push(F),n.push(I);let H=r(new fn(2800,2800),T(Qp.vertex,Qp.fragment,{time:a,day:l,glow:{value:p},island:{value:new Ot(0,-7,85,87)},deep:{value:new Le(398368)},shallow:{value:new Le(930640)},sky:{value:m.clone().multiplyScalar(.7)},sunDir:{value:x},sunColor:{value:new Le(14676223)},fogColor:{value:new Le},fogDensity:{value:0},fogNear:{value:1},fogFar:{value:1e3}},{fog:!0}));H.rotation.x=-Math.PI/2,H.position.y=-3.2,i.add(H);let D=r(new fn(1900,1900),T(td,ky,{time:a,strength:{value:.55},color:{value:new Le(1717320)},island:{value:new Ot(0,-7,85,87)}},{transparent:!0,depthWrite:!1,fog:!1}));D.rotation.x=-Math.PI/2,D.position.y=-2.4,i.add(D);let V=500,k=new Float32Array(V*3),E=new Float32Array(V*3);for(let fe=0;fe<V;fe++)k.set([(M()-.5)*180,2+M()*M()*75,-95+M()*170],fe*3),E.set([M(),M(),M()],fe*3);let Z=new _t;Z.setAttribute("position",new Rt(k,3)),Z.setAttribute("seed",new Rt(E,3));let W=T(em.vertex,em.fragment,{time:a,pointScale:o,color:{value:new Le(10475775)},strength:{value:.32}},g),Q=new yn(Z,W);Q.frustumCulled=!1,i.add(Q),t.push(Z),n.push(W);let Fe=new vt;Fe.position.set(0,86.5,-12),i.add(Fe);let Ne=T(td,By,{strength:{value:.45},color:{value:new Le(9430783)}},{...g,side:Kt}),ze=new fn(100,5.2);ze.translate(50,0,0),t.push(ze),n.push(Ne);let se=new st(ze,Ne),le=new st(ze,Ne);le.rotation.x=Math.PI/2;let De=new vt;De.add(se,le),De.rotation.z=-.07,Fe.add(De);let be=new Vt({color:12579583,toneMapped:!1}),ue=r(new Pi(.9,16,12),be);Fe.add(ue);let te=(e.lamps||[]).slice(0,64),pe=T("varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",Hy,{color:{value:new Le(16767392)},strength:{value:.3}},{...g,side:Kt}),ce=new ks(2.7,6.1,18,1,!0),ne=new Sn(ce,pe,Math.max(1,te.length));t.push(ce),n.push(pe);let de=new Wt;te.forEach((fe,Pe)=>{de.position.set(fe.x+1.9*Math.cos(fe.angle||0),(fe.y||0)+2.9,fe.z-1.9*Math.sin(fe.angle||0)),de.updateMatrix(),ne.setMatrixAt(Pe,de.matrix)}),ne.count=te.length,ne.instanceMatrix.needsUpdate=!0,i.add(ne);let we=[],Ve=[],Ke=[],it=[],et=[],ht=360,J=fe=>Math.max(0,.45+.3*Math.sin(fe*5+1.3)+.2*Math.sin(fe*13+.4)+.12*Math.sin(fe*31+2.1))*(.35+.65*Math.max(0,Math.sin(fe*2+.6)));for(let fe=0;fe<=ht;fe++){let Pe=Math.PI*.8+Math.PI*1.45*fe/ht,_e=1180+40*Math.sin(Pe*7),qe=6+74*J(Pe);if(we.push(Math.cos(Pe)*_e,-3.5,Math.sin(Pe)*_e,Math.cos(Pe)*_e,qe,Math.sin(Pe)*_e),Ve.push(0,qe/80),fe<ht){let Xe=fe*2;Ke.push(Xe,Xe+1,Xe+2,Xe+1,Xe+3,Xe+2)}}for(let fe=0;fe<90;fe++){let Pe=Math.PI*.82+Math.PI*1.41*M(),_e=1170+40*Math.sin(Pe*7);J(Pe)<.08||(it.push(Math.cos(Pe)*_e,1+M()*M()*28,Math.sin(Pe)*_e),et.push(M()*20))}let Ut=new _t;Ut.setAttribute("position",new mt(we,3)),Ut.setAttribute("lift",new mt(Ve,1)),Ut.setIndex(Ke);let ut={value:.8},O=r(Ut,T(tm.vertex,tm.fragment,{haze:{value:y},shade:ut},{fog:!1,side:Kt}));O.frustumCulled=!1,O.renderOrder=-8,i.add(O);let R=new _t;R.setAttribute("position",new mt(it,3)),R.setAttribute("phase",new mt(et,1));let re={value:0},oe=T(nm.vertex,nm.fragment,{time:a,pointScale:o,strength:re},{...g,fog:!1}),xe=new yn(R,oe);xe.frustumCulled=!1,xe.renderOrder=-7,i.add(xe),t.push(R),n.push(oe);let Ue=T(im.vertex,im.fragment,{color:{value:new Le(11128063)},strength:{value:0}},{...g,side:Kt,fog:!1}),ke=new ds(11,.7,280,20,1,!0);ke.translate(0,140,0),t.push(ke),n.push(Ue);let G=[[-72,-168,0],[64,-186,2.1],[4,-232,4.2]].map(([fe,Pe,_e])=>{let qe=new st(ke,Ue);return qe.position.set(fe,-3,Pe),qe.rotation.order="YXZ",qe.userData.offset=_e,qe.frustumCulled=!1,i.add(qe),qe}),j=96,he=new Float32Array(j*3),Se=new Float32Array(j);for(let fe=0;fe<j;fe++)Se[fe]=M()*Math.PI*2;let ie=new _t;ie.setAttribute("position",new Rt(he,3)),ie.setAttribute("phase",new Rt(Se,1)),ie.setDrawRange(0,0);let ve=T(sm.vertex,sm.fragment,{time:a,pointScale:o},{...g,fog:!1}),Ee=new yn(ie,ve);Ee.frustumCulled=!1,i.add(Ee),t.push(ie),n.push(ve);let Te=Zp(a),Ie=Te.pass,X=!1,Ae="high",Me=0;return{post:Ie,sea:H,setLighting(fe,Pe,_e){x.copy(_e).normalize(),l.value=fe,c.value=Math.min(1,Pe)*(1-fe),v.set(330010).lerp(new Le(3108776),fe).lerp(new Le(1909062),c.value*.8),m.set(1323596).lerp(new Le(11126484),fe).lerp(new Le(12609598),c.value*.85),f.set(Pe>.1?16747077:8011050),pe.uniforms.color.value.copy(Ny).lerp(Uy,c.value),p.set(4860440).lerp(new Le(7027234),c.value),y.copy(s.fog.color),N.visible=fe<.35,H.material.uniforms.sky.value.copy(m).lerp(v,c.value*.65).multiplyScalar(.7),H.material.uniforms.deep.value.set(398368).lerp(new Le(669270),fe),H.material.uniforms.shallow.value.set(930640).lerp(new Le(1929112),fe),D.material.uniforms.strength.value=.55*(1-fe*.85),ut.value=.82-.22*(1-fe)-.2*c.value,re.value=Math.max(0,1-fe*1.6)*(1-c.value*.5),H.material.uniforms.sunColor.value.set(14676223).lerp(new Le(16752736),c.value)},setWeather(fe){h.value=fe==="rain"?.95:fe==="fog"?.7:.25},setFlash(fe){u.value=Math.max(0,Math.min(1,fe))},setCinematic(fe){Te.setCinematic(fe)},setAviation(fe){let Pe=Math.min(j,fe.length);for(let _e=0;_e<Pe;_e++)he.set([fe[_e].x,fe[_e].y,fe[_e].z],_e*3);ie.attributes.position.needsUpdate=!0,ie.setDrawRange(0,Pe)},setTier(fe){Ae=fe;let Pe=Ae!=="low";D.visible=Pe,Q.visible=Pe,ne.visible=Pe,G.forEach(_e=>{_e.visible=Pe}),d.value=Ae==="low"?3:Ae==="medium"?4:5,Ie.enabled=Ae==="high"||Ae==="ultra"},setBusy(fe){X=!!fe},setPointScale(fe){o.value=fe},update(fe,Pe,_e,qe){let Xe=qe?Math.min(.1,Math.max(0,fe)):0;a.value+=Xe,s.fog&&(H.material.uniforms.fogColor.value.copy(s.fog.color),s.fog.isFogExp2&&(H.material.uniforms.fogDensity.value=s.fog.density)),H.position.x=_e.position.x,H.position.z=_e.position.z;let U=X?1.15:.32,q=Fe.userData.rate??U;Fe.userData.rate=q+(U-q)*Math.min(1,Xe*1.5),De.rotation.y+=Xe*Fe.userData.rate,Ne.uniforms.strength.value=X?.85:.45,ue.scale.setScalar(1+Math.sin(a.value*(X?6:2.2))*.18),pe.uniforms.strength.value=.3*(1+.035*Math.sin(a.value*1.6)+.015*Math.sin(a.value*4.1+1.2)),Me+=Xe;let Y=1-l.value;Ue.uniforms.strength.value=Y*Y*.075*(1-c.value*.7)*(1-h.value*.45),G.forEach((ae,K)=>{let $=ae.userData.offset;ae.rotation.y=Math.PI+Math.sin(Me*(.11+K*.025)+$)*.95,ae.rotation.x=.22+.14*Math.sin(Me*.13+$*1.7)}),Ie.enabled&&Te.update(Xe,_e,x,{day:l.value,dusk:c.value,cloud:h.value,animated:qe})},stats:()=>({tier:Ae,busy:X,stars:w,dust:V,lamps:te.length,post:Ie.enabled,coast:Ke.length/6,shoreLights:et.length,clouds:+h.value.toFixed(2),searchlights:G.length,aviation:ie.drawRange.count,cinema:Te.stats()}),dispose(){i.removeFromParent(),ne.dispose(),t.forEach(fe=>fe.dispose()),n.forEach(fe=>fe.dispose()),Te.dispose()}}}var po=3,Gy=3,Vy=192,am=[9365742,7730385,16763274,12166399];function om(s,e={}){let t=e.anchors||[],n=new vt;n.name="city-flow",s.add(n);let i=[];for(let _=0;_<t.length;_++)for(let w of[1,3]){let A=t[_],b=t[(_+w)%t.length];!A||!b||A.id===b.id||i.push({from:A,to:b,lift:22+(_+w)%4*11})}if(i.length<2)return n.removeFromParent(),{group:n,update(){},setTier(){},stats:()=>({routes:0,points:0,visible:!1}),dispose(){}};let r=Math.min(Vy/po,i.length*Gy),a=r*po,o=new Float32Array(a*3),l=new Float32Array(a),c=new Float32Array(a),h=new Float32Array(a*3),u=new _t;u.setAttribute("position",new Rt(o,3)),u.setAttribute("alpha",new Rt(l,1)),u.setAttribute("size",new Rt(c,1)),u.setAttribute("tint",new Rt(h,3));let d=new yt({transparent:!0,depthWrite:!1,blending:tn,fog:!1,uniforms:{pointScale:{value:900},strength:{value:.4}},vertexShader:`attribute float alpha,size;attribute vec3 tint;uniform float pointScale;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=clamp(size*pointScale/max(1.0,-mv.z),1.0,12.0);vAlpha=alpha;vTint=tint;}`,fragmentShader:`uniform float strength;varying float vAlpha;varying vec3 vTint;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;
gl_FragColor=vec4(vTint*pow(1.0-d,2.4)*vAlpha*strength,1.0);}`}),p=new yn(u,d);p.frustumCulled=!1,p.renderOrder=2,n.add(p);let x=[],v=new Le;for(let _=0;_<r;_++){let w=i[_%i.length];v.setHex(am[_%am.length]),x.push({route:w,t:_*.3819%1,speed:.045+_*7%9/190,tint:[v.r,v.g,v.b],size:.42+_*3%5*.08})}function m(_,w,A){let b=_.from,C=_.to,S=1-w,P=(b.x+C.x)/2,M=(b.z+C.z)/2,F=Math.max(b.height,C.height)+_.lift,I=b.height+4,N=C.height+4;return A.x=S*S*b.x+2*S*w*P+w*w*C.x,A.y=S*S*I+2*S*w*F+w*w*N,A.z=S*S*b.z+2*S*w*M+w*w*C.z,A}let f={x:0,y:0,z:0},y="high",T=.4;function g(_,w,A){if(!p.visible)return;let b=w?Math.min(_,.05):0;for(let C=0;C<x.length;C++){let S=x[C];S.t=(S.t+b*S.speed)%1;for(let P=0;P<po;P++){let M=C*po+P,F=(S.t-P*.012+1)%1;m(S.route,F,f),o[M*3]=f.x,o[M*3+1]=f.y,o[M*3+2]=f.z;let I=Math.min(1,Math.min(F,1-F)*7);l[M]=I*(1-P/po)*.9,c[M]=S.size*(1-P*.22),h[M*3]=S.tint[0],h[M*3+1]=S.tint[1],h[M*3+2]=S.tint[2]}}u.attributes.position.needsUpdate=!0,u.attributes.alpha.needsUpdate=!0,u.attributes.size.needsUpdate=!0,u.attributes.tint.needsUpdate=!0,T+=(.18+A*.82-T)*Math.min(1,Math.max(0,_)*2),d.uniforms.strength.value=T}return{group:n,update:g,setPointScale(_){d.uniforms.pointScale.value=_},setTier(_){y=_,p.visible=y!=="low",n.visible=y!=="low"},stats:()=>({routes:i.length,points:a,visible:p.visible,tier:y}),dispose(){n.removeFromParent(),u.dispose(),d.dispose()}}}var lm=[{points:[[-62,34,-62],[8,44,-74],[62,38,-22],[56,47,42],[-8,41,62],[-64,36,12]],speed:9.5},{points:[[30,56,-12],[0,62,18],[-30,58,-12],[0,66,-42]],speed:8},{points:[[42,29,-56],[-28,33,-42],[-52,27,20],[18,31,52],[60,33,8]],speed:11}];function cm(s,e={}){let t=new vt;t.name="city-drones",s.add(t);let n=[],i=[],r=[],a=new B,o=new B,l=new B,c=new Pi(.16,8,6);n.push(c);let h=g=>{let _=new Vt({color:g,toneMapped:!1});return i.push(_),_},u=h(16730684),d=h(5046154),p=h(16777215);[...lm,...lm].forEach((g,_)=>{let w=new Pr(g.points.map(S=>new B(S[0],S[1]+(_>=3?8:0),S[2])),!0,"centripetal",.6),A=new vt;A.name="city-drone-"+(_+1),A.visible=!1;let b=new vt;A.add(b);let C=[new st(c,u),new st(c,d),new st(c,p)];C[0].position.set(-2.6,.8,0),C[1].position.set(2.6,.8,0),C[2].position.set(0,2.1,-.4),A.add(...C),t.add(A),r.push({root:A,body:b,lights:C,curve:w,length:w.getLength(),speed:g.speed,t:_*.37%1,rotors:[],roll:0,dir:1,waiting:0})});let x=!0,v="high",m=!1,f=new Map,y=g=>{if(!f.has(g)){let _=g.clone();e.surfaces?.apply(_,"object",g.name),f.set(g,_)}return f.get(g)};function T(g,_){let{curve:w,root:A}=g,b=g.t;if(g.t=(g.t+_*g.speed*g.dir/g.length+1)%1,w.getPointAt(g.t,A.position),w.getTangentAt(g.t,a),a.multiplyScalar(g.dir),g.collider&&e.traffic){let P=g.collider,M=e.traffic;w.getPointAt((g.t+g.dir*20/g.length+1)%1,l),g.avoidTime=Math.max(0,(g.avoidTime||0)-_);let F=Math.max(A.position.y,M.ceiling(P,A.position.x,A.position.z),M.ceiling(P,l.x,l.z),g.avoidTime?g.avoidHeight:0);A.position.y=P.y+Ft.clamp(F-P.y,-_*5,_*7);let I=Math.atan2(a.x,a.z),N=P.heading+Math.atan2(Math.sin(I-P.heading),Math.cos(I-P.heading))*Math.min(1,_*3),H={...A.position,heading:N};if(g.obstruction=M.obstruction(P,P,H),g.obstruction){let D=M.body(g.obstruction),V=g.obstruction.startsWith("drone-")&&(Math.abs(P.y-D.y)>1?P.y<D.y:P.id<D.id);V||(g.avoidHeight=Math.min(170,P.y+8),g.avoidTime=8),H={...P,y:V?P.y:Math.min(170,P.y+_*5)},g.t=b}M.propose(P,H,(D,V)=>{D?g.waiting=0:(g.t=b,g.waiting+=_,g.waiting>2.5&&(g.dir=-g.dir,g.waiting=0)),A.position.set(V.x,V.y,V.z),A.rotation.set(0,V.heading,0)})}w.getTangentAt((g.t+.015)%1,o),g.collider||A.rotation.set(0,Math.atan2(a.x,a.z),0);let C=Math.atan2(o.x,o.z)-Math.atan2(a.x,a.z),S=Math.atan2(Math.sin(C),Math.cos(C));g.roll+=(Ft.clamp(S*12,-.55,.55)-g.roll)*(_>0?Math.min(1,_*3):1),g.body.rotation.z=g.roll}return{setTier(g){v=g},setTemplate(g){if(!g)return;m=!0;let _=f;f=new Map;for(let[w,A]of r.entries()){A.body.clear(),A.rotors.length=0;let b=g.clone(!0);b.scale.setScalar(1.9),b.updateMatrixWorld(!0);let C=new cn().setFromObject(b);if(C.expandByScalar(.5),C.min.y-=1,C.max.y+=1,b.traverse(S=>{S.isMesh&&(S.castShadow=!1,S.receiveShadow=!1,S.material=Array.isArray(S.material)?S.material.map(y):y(S.material)),/^rotor_/.test(S.name)&&A.rotors.push(S)}),A.body.add(b),A.root.visible=w<(v==="low"?2:v==="medium"?3:6),e.traffic&&!A.collider){T(A,0);let S=ts({min:C.min.toArray(),max:C.max.toArray()});A.root.position.y=Math.max(A.root.position.y,e.traffic.ceiling(S,A.root.position.x,A.root.position.z)),A.collider=e.traffic.register("drone-"+w,S,{...A.root.position,heading:A.root.rotation.y},0),A.collider||(A.root.visible=!1)}}_.forEach(w=>w.dispose())},update(g,_,w){x=w;let A=w?Math.min(.1,Math.max(0,g)):0;r.forEach((b,C)=>{if(b.root.visible=m&&C<(v==="low"?2:v==="medium"?3:6)&&(!e.traffic||!!b.collider),b.collider&&(b.root.visible&&!b.collider.enabled&&!e.traffic.relocate(b.collider,b.collider)&&(b.root.visible=!1),b.collider.enabled=b.root.visible),!b.root.visible)return;T(b,A),b.rotors.forEach((P,M)=>{P.rotation.y+=A*(M%2?-46:46)});let S=Math.sin(_*5+C*1.7);b.lights[0].visible=b.lights[1].visible=!w||S>-.2,b.lights[2].visible=!w||S>.93,b.lights[2].scale.setScalar(w?1.6:1)})},stats:()=>({drones:r.filter(g=>g.root.visible).length,animated:x,positions:r.map(g=>g.root.position.toArray().map(_=>Math.round(_))),obstructions:r.map(g=>g.obstruction)}),dispose(){t.removeFromParent(),n.forEach(g=>g.dispose()),i.forEach(g=>g.dispose()),f.forEach(g=>g.dispose()),r.forEach(g=>{g.collider&&e.traffic.remove(g.collider.id)}),r.length=0}}}var Ms=[{id:"repair-bay",district:"infra",x:-28,z:-62,role:"technician"},{id:"parcel-sorter",district:"missions",x:35,z:49,role:"courier"},{id:"relay-mast",district:"integrations",x:57,z:-20,role:"archivist"},{id:"kinetic-fountain",district:"graph",x:-28,z:47,role:"archivist"},{id:"glass-garden",district:"graph",x:40,z:-68,role:"technician"},{id:"meeting-charge",district:"operations",x:8,z:46,role:"courier"}],hm=[{id:"data-monument",x:0,z:24,y:0,angle:0,clip:"spin"},{id:"neon-gate",x:-18,z:0,y:gt.pavement,angle:0,clip:"pulse"},{id:"sky-pod",x:56,z:47,y:0,angle:0,clip:"scan"},{id:"tidal-turbine",x:0,z:96,y:-3.2,angle:Math.PI,clip:"spin"}],Wy=2*Bt.halfWidth;function qy(s){return Hp().some(e=>e.axis==="x"?Math.abs(s.x-e.at)<=Ht.opening&&Math.abs(s.z-e.line)<=Bt.halfWidth:Math.abs(s.z-e.at)<=Ht.opening&&Math.abs(s.x-e.line)<=Bt.halfWidth)}function um(s,e){let t=Wp(),n=t.nodes.map(o=>({x:o.x,z:o.z,y:e(o.x,o.z),heading:0})),i=new Map,r=-1;function a(o,l){r!==s.revision()&&(i.clear(),r=s.revision());let c=o.circles.map(g=>[g.x,g.z,g.r].join(",")).join(";")+":"+o.maxY;if(!i.has(c)){let g=new Map,_=[];n.forEach((A,b)=>{s.clear(o,A,A,!1)&&(g.set(b,_.length),_.push(A))});let w=_.map(()=>[]);for(let A of t.links){let b=g.get(A.a),C=g.get(A.b);b===void 0||C===void 0||s.clear(o,_[b],_[C],!1)&&(w[b].push([C,A.length]),w[C].push([b,A.length]))}i.set(c,{nodes:_,edges:w})}let h=i.get(c),u=[...h.nodes,{x:o.x,y:o.y,z:o.z,heading:o.heading},{...l,heading:0}],d=u.length-2,p=u.length-1,x=qy(u[d]),v=h.edges.map(g=>[...g]);v.push([],[]);for(let g of[d,p])for(let _=0;_<g;_++){let w=u[g],A=u[_],b=Math.hypot(w.x-A.x,w.z-A.z);(b<ju||_===d)&&Ju(w,A,g===d&&x&&b<Wy)&&s.clear(o,w,A,!1)&&(v[g].push([_,b]),v[_].push([g,b]))}let m=u.map(()=>1/0),f=u.map(()=>-1),y=new Set(u.map((g,_)=>_));for(m[d]=0;y.size;){let g=-1;for(let _ of y)(g<0||m[_]<m[g])&&(g=_);if(!Number.isFinite(m[g])||g===p)break;y.delete(g);for(let[_,w]of v[g])m[g]+w<m[_]&&(m[_]=m[g]+w,f[_]=g)}if(!Number.isFinite(m[p]))return[];let T=[];for(let g=p;g!==d;g=f[g]){if(g<0)return[];T.unshift(u[g])}return T}return{route:a,points:n}}function dm(s){let e=new vt;e.name="city-conversations",s.add(e);let t=new Kn(1,.025,5,40,Math.PI*1.45),n=new fs(.92,1,48),i=new B(0,0,1),r=new B,a=[],o=0;for(let l=0;l<8;l++){let c=[];for(let h=0;h<4;h++){let u=new Vt({color:8445392,transparent:!0,opacity:0,blending:tn,depthWrite:!1,side:Kt,toneMapped:!1}),d=new st(h===3?n:t,u);d.visible=!1,e.add(d),c.push(d)}a.push({meshes:c,age:10,from:null,to:null})}return{send(l,c,h="ambient"){let u=a.find(d=>d.age>=2.4);return u?(Object.assign(u,{from:l,to:c,age:0,source:h}),o++,u.meshes.forEach(d=>d.material.color.setHex(h==="live"?16105851:8445392)),!0):!1},update(l,c=!0){for(let h of a){if((!c||h.from?.enabled===!1||h.to?.enabled===!1)&&(h.age=10),h.age+=l,h.meshes.forEach(m=>{m.visible=!1}),h.age>=2.4)continue;let u=h.from,d=h.to,p=d.x-u.x,x=d.z-u.z;r.set(p,d.y+Math.min(2,d.maxY*.7)-(u.y+Math.min(2,u.maxY*.7)),x).normalize();for(let m=0;m<3;m++){let f=(h.age-m*.18)/1.55;if(f<0||f>1)continue;let y=h.meshes[m];y.visible=!0,y.position.set(u.x+p*f,Ft.lerp(u.y+Math.min(2,u.maxY*.7),d.y+Math.min(2,d.maxY*.7),f)+Math.sin(f*Math.PI)*.65,u.z+x*f),y.quaternion.setFromUnitVectors(i,r),y.rotateZ(m*.65),y.scale.setScalar(.25+Math.sin(f*Math.PI)*.65),y.material.opacity=Math.sin(Math.PI*f)*.75}let v=(h.age-1.55)/.85;if(v>0){let m=h.meshes[3];m.visible=!0,m.position.set(d.x,d.y+Math.min(2,d.maxY*.7),d.z),m.quaternion.setFromUnitVectors(i,r),m.scale.setScalar(1.25*(1-v)+.12),m.material.opacity=Math.sin(v*Math.PI)*.85}}},clear(){a.forEach(l=>{l.age=10,l.meshes.forEach(c=>{c.visible=!1})})},stats:()=>({sent:o,active:a.filter(l=>l.age<2.4).length,sources:a.filter(l=>l.age<2.4).map(l=>l.source)}),dispose(){e.removeFromParent(),t.dispose(),n.dispose(),a.forEach(l=>l.meshes.forEach(c=>c.material.dispose()))}}}var Xy=[["leg_l","shin_l","foot_l"],["leg_r","shin_r","foot_r"]],fm=[[0,-.13,-.085],[0,-.13,.27]];function Ss(s,e,t,n=14,i=.6){if(!(t>0)||!Number.isFinite(s.y)||Math.abs(e-s.y)>i)return s.y=e,s.v=0,e;let r=s.y-e,a=Math.exp(-n*t),o=(s.v+n*r)*t;return s.v=(s.v-n*o)*a,s.y=e+(r+o)*a,s.y}function jy(s,e=.025,t=.07){let n=Math.min(1,Math.max(0,(s-e)/(t-e)));return 1-n*n*(3-2*n)}var mo=new B,go=new B,id=new B,Ky=new B,sd=new B,rd=new B,Gc={hip:new Qt,knee:new Qt};function Yy(s,e,t,n,i){mo.subVectors(e,s),go.subVectors(t,e);let r=mo.length(),a=go.length();if(r<1e-6||a<1e-6||i.lengthSq()<1e-12)return null;id.copy(i).normalize();let o=Math.min(r+a-1e-4,Math.max(Math.abs(r-a)+1e-4,n.distanceTo(s))),l=Math.atan2(Ky.crossVectors(mo,go).dot(id),mo.dot(go)),c=Math.acos(Math.max(-1,Math.min(1,(r*r+a*a-o*o)/(2*r*a)))),h=(Math.PI-c)*(l<0?-1:1);return Gc.knee.setFromAxisAngle(id,h-l),sd.copy(go).applyQuaternion(Gc.knee).add(mo),rd.subVectors(n,s),sd.lengthSq()<1e-12||rd.lengthSq()<1e-12?null:(Gc.hip.setFromUnitVectors(sd.normalize(),rd.normalize()),Gc)}var xo=new B,bo=new B,pm=new B,mm=new B,ad=new B,od=new B,gm=new B,ld=new Qt,xm=new Qt,bm=new Qt;function _m(s){xo.fromArray(fm[0]).applyMatrix4(s.matrixWorld),bo.fromArray(fm[1]).applyMatrix4(s.matrixWorld)}function ym(s,e){return _m(s),Math.min(xo.y,bo.y)-e.matrixWorld.elements[13]}function Jy(s,e){return _m(s),Math.max(e(xo.x,xo.z)-xo.y,e(bo.x,bo.z)-bo.y)}function vm(s,e){s.parent.getWorldQuaternion(ld),s.quaternion.premultiply(ld).premultiply(e).premultiply(ld.invert())}function Mm(s){let e=s.getObjectByName("structure");if(!e)return null;let t=[];for(let n of Xy){let[i,r,a]=n.map(o=>s.getObjectByName(o));if(!i||!r||!a||r.parent!==i||a.parent!==r)return null;t.push({leg:i,shin:r,foot:a})}s.updateMatrixWorld(!0);for(let n of t)if(Math.abs(ym(n.foot,s))>.04)return null;return{structure:e,legs:t}}function Sm(s,e,t,{maxDrop:n=.25,maxRaise:i=.35}={}){e.updateMatrixWorld(!0);let r=s.legs.map(l=>({l,lift:ym(l.foot,e),pen:Jy(l.foot,t)})),a=0;for(let l of r)a=Math.max(a,jy(l.lift)*Math.max(0,-l.pen));a=Math.min(a,n),a<1e-4&&(a=0),a&&(e.position.y-=a,e.updateMatrixWorld(!0));let o=[];for(let l of r){let c=Math.min(i,l.pen+a);if(o.push(Math.max(0,c)),c<=5e-4)continue;let{leg:h,shin:u,foot:d}=l.l;h.getWorldPosition(pm),u.getWorldPosition(mm),d.getWorldPosition(ad),od.copy(ad),od.y+=c,gm.setFromMatrixColumn(h.matrixWorld,0);let p=Yy(pm,mm,ad,od,gm);p&&(d.getWorldQuaternion(bm),vm(u,p.knee),vm(h,p.hip),h.updateMatrixWorld(!0),u.getWorldQuaternion(xm),d.quaternion.copy(xm.invert().multiply(bm)),d.updateMatrixWorld(!0))}return{drop:a,raised:o}}var wm=(s,e,t)=>s+Math.atan2(Math.sin(e-s),Math.cos(e-s))*Math.min(1,t*5),Em=(s,e,t)=>{let n=s.play?.(e);n&&(n.timeScale=t/1.65)};function Tm(s,{traffic:e,camera:t,floor:n,onSelect:i,onDemonstrate:r,active:a,onGait:o,shadows:l}){let c=[],h=new Map,u=[],d=[],p=um(e,n),x=dm(s),v=0,m=null,f=!1,y="high",T=!1,g=0,_="",w=()=>y==="low"?3:y==="medium"?10:19;function A(D){D.slot&&(D.slot.owner=null,D.slot=null),D.path=[],D.goal=null}function b(D){return D?{id:D.id,role:D.role,state:D.state,source:D.source||"ambient",guided:!!D.guide}:null}function C(D){m=D?.id||null,_=D?.state||"",i?.(b(D))}function S(D,V,k){let E="resident-"+V,Z=ts(k,1.15),W=["courier","technician","archivist"][V%3],Q=null;Z.circles=[{x:0,z:0,r:Z.reach}];let Ne=[...u.filter(se=>se.role===W).map(se=>({x:se.x,z:se.z+2.2,y:n(se.x,se.z+2.2),heading:Math.PI})),...p.points.map((se,le)=>p.points[(V*7+le)%p.points.length])];for(let se of Ne)if(Q=e.register(E,Z,se,1),Q)break;if(!Q)return D.node.visible=!1,null;let ze={...D,id:E,index:V,body:Q,role:W,state:"idle",path:[],until:V*.3,trip:V,cooldown:8+V*.7,source:"ambient",carrying:!1,parcels:[],soundX:Q.x,soundZ:Q.z};return D.node.traverse(se=>{se.isMesh&&/parcel/.test(se.name)&&(ze.parcels.push(se),se.visible=!1)}),c.push(ze),D.node.position.set(Q.x,Q.y,Q.z),ze.height={y:Q.y,v:0},ze.pitch={y:0,v:0},D.node.rotation.order="YXZ",ze.rig=Mm(D.node),ze.shadow=l?.claim()||null,ze}function P(D,V,k,E){let Z={id:D,body:V,node:k,pause:E,role:"patrol",state:"patrol",cooldown:7+c.length,path:[],soundX:V.x,soundZ:V.z};return c.push(Z),Z}function M(D){A(D),D.source="ambient",D.state="idle",D.until=v+2;let V=u.filter(k=>!k.owner&&(D.role==="courier"?k.place===(D.carrying?"meeting-charge":"parcel-sorter"):k.role===D.role||k.place==="meeting-charge"));for(let k=0;k<V.length;k++){let E=V[(D.trip+k)%V.length],Z=p.route(D.body,E);if(Z.length){E.owner=D.id,D.slot=E,D.path=Z,D.goal=E,D.trip++,D.state=D.carrying?"carry":"walk";return}}for(let k=0;k<8;k++){let E=p.points[(D.trip++*13+D.index)%p.points.length],Z=p.route(D.body,E);if(Z.length){D.path=Z,D.state="walk";return}}}function F(D){for(let V of[D.a,D.b])A(V),V.meeting=null,V.cooldown=v+12+(V.index||0),V.pause?.(!1),V.state=V.pause?"patrol":"idle",V.until=v+1}function I(){d.forEach(F),d.length=0,x.clear(),m=null,i?.(null);for(let D of c)D.guide=null,D.pause?.(!1,D.body),D.trafficYield=!1,D.yieldTram=null,D.pause||(A(D),D.state="idle",D.until=v+1)}function N(D,V){if(T)return;V&&(v+=D);for(let E of c){let Z=E.pause||E.index<w();if(Z&&!E.body.enabled){if(!e.relocate(E.body,E.body)){E.node.visible=!1;continue}E.body.enabled=!0,E.node.position.set(E.body.x,E.body.y,E.body.z)}!Z&&E.body.enabled&&(A(E),E.body.enabled=!1,E.meeting&&F(E.meeting)),E.node.visible=!!Z,E.soundX==null&&(E.soundX=E.body.x,E.soundZ=E.body.z);let W=Math.hypot(E.body.x-E.soundX,E.body.z-E.soundZ);if(E.soundX=E.body.x,E.soundZ=E.body.z,o?.(E.id,E.body.x,E.body.y,E.body.z,W>.004,!!E.carrying,E.role==="patrol"?"glide":""),!Z||!V)continue;let Q=e.neighbors(E.body,22).sort((Fe,Ne)=>Math.hypot(Fe.x-E.body.x,Fe.z-E.body.z)-Math.hypot(Ne.x-E.body.x,Ne.z-E.body.z)).find(Fe=>{if(!Fe.id.startsWith("tram-"))return!1;let Ne=E.body.x-Fe.x,ze=E.body.z-Fe.z,se=Math.cos(Fe.heading),le=Math.sin(Fe.heading);return Math.abs(Ne*se-ze*le)<E.body.reach+(E.trafficYield?4:2.3)&&Ne*le+ze*se>-8-E.body.reach&&Ne*le+ze*se<14});if(!Q&&E.yieldTram){let Fe=e.body(E.yieldTram),Ne=Fe?E.body.x-Fe.x:0,ze=Fe?E.body.z-Fe.z:0;if(Fe?.enabled&&Math.hypot(Ne,ze)<26&&Ne*Math.sin(Fe.heading)+ze*Math.cos(Fe.heading)>-Fe.reach-E.body.reach-1){E.pause?.(!0),E.play?.("idle");continue}E.yieldTram=null}if(Q){if(E.trafficYield=!0,E.yieldTram=Q.id,E.meeting){let ne=E.meeting;F(ne),d.splice(d.indexOf(ne),1)}E.pause?.(!0);let Fe=Math.cos(Q.heading),Ne=Math.sin(Q.heading),ze=Math.sign((E.body.x-Q.x)*Fe-(E.body.z-Q.z)*Ne)||1,se=(E.body.x-Q.x)*Fe-(E.body.z-Q.z)*Ne,le=Math.sign((E.body.x-Q.x)*Ne+(E.body.z-Q.z)*Fe)||1,De=[[Fe*ze,-Ne*ze,E.body.reach+4.5-Math.abs(se)],[-Fe*ze,Ne*ze,E.body.reach+4.5+Math.abs(se)],[Ne*le,Fe*le,1.2]],be=Array.from({length:16},(ne,de)=>[Math.cos(de*Math.PI/8),Math.sin(de*Math.PI/8),2]),ue=([ne,de])=>ne*(E.body.x-Q.x)+de*(E.body.z-Q.z);De.push(...be.filter(ne=>ue(ne)>.2).sort((ne,de)=>ue(de)-ue(ne)));let te=E.escape&&v<E.escape.until?E.escape:null,pe=te?De.filter(ne=>ne[0]===te.x&&ne[1]===te.z).concat(De.filter(ne=>!(ne[0]===te.x&&ne[1]===te.z))):De,ce=!1;for(let[ne,de,we]of pe){let Ve={x:E.body.x+ne*we,y:E.body.y,z:E.body.z+de*we,heading:E.body.heading};if(!e.clear(E.body,E.body,Ve))continue;let Ke={...Ve,x:E.body.x+ne*D*1.8,z:E.body.z+de*D*1.8};Ke.y=E.pause?E.body.y:n(Ke.x,Ke.z),e.propose(E.body,Ke,(it,et)=>{E.node.position.set(et.x,et.y,et.z),Em(E,"walk",1.8)}),E.escape={x:ne,z:de,until:v+.8},E.state="yield",E.cooldown=v+5,ce=!0;break}ce||(E.play?.("idle"),E.state="yield");continue}if(E.trafficYield&&(E.trafficYield=!1,E.pause?.(!1,E.body),E.state=E.pause?"patrol":E.path.length?"walk":"idle"),!E.meeting){if(E.pause){E.state==="greet"&&v>E.until&&(E.pause(!1),E.state="patrol");continue}if(E.guide&&Math.hypot(t.position.x-E.body.x,t.position.z-E.body.z)>9){E.play("idle");continue}if(E.path.length){let Fe=E.path[0],Ne=Fe.x-E.body.x,ze=Fe.z-E.body.z,se=Math.hypot(Ne,ze),le=E.guide?1.9:1.65;if(se<.18){E.path.shift();continue}let De=Math.min(se,le*D),be={x:Ne/se,z:ze/se},ue=E.body.x+be.x*De,te=E.body.z+be.z*De,pe=wm(E.body.heading,Math.atan2(Ne,ze),D),ce={x:E.body.x+be.x*1.8,y:n(ue,te),z:E.body.z+be.z*1.8,heading:pe};if(e.clear(E.body,E.body,ce))E.detour=null;else{let de=[[be.z,-be.x],[-be.z,be.x]],we=E.detour&&v<E.detour.until?de.findIndex(it=>it[0]===E.detour.x&&it[1]===E.detour.z):-1,Ve=we>=0?[de[we],...de.filter((it,et)=>et!==we)]:de,Ke=!1;for(let[it,et]of Ve){let ht={x:E.body.x+it*1.3,y:E.body.y,z:E.body.z+et*1.3,heading:E.body.heading};if(!e.clear(E.body,E.body,ht))continue;let J={x:ht.x+be.x*1.8,y:n(ht.x,ht.z),z:ht.z+be.z*1.8,heading:E.body.heading};if(e.clear(E.body,ht,J)){ue=E.body.x+it*De,te=E.body.z+et*De,E.detour={x:it,z:et,until:v+1.2},Ke=!0;break}}Ke||(ue=E.body.x,te=E.body.z)}e.clear(E.body,E.body,{x:ue,y:n(ue,te),z:te,heading:pe},!1)||(pe=E.body.heading);let ne=Math.hypot(ue-E.body.x,te-E.body.z)>.001;e.propose(E.body,{x:ue,y:n(ue,te),z:te,heading:pe},(de,we)=>{E.node.position.set(we.x,we.y,we.z),E.node.rotation.y=we.heading,!de||!ne?(E.wait=(E.wait||0)+D,E.play("idle"),E.state="yield"):(E.wait=0,E.state=E.guide?"guide":E.carrying?"carry":"walk",Em(E,E.carrying?"carry":"walk",le)),E.wait>2.5&&(E.wait=0,A(E),E.state=E.guide?"guide":"idle",E.until=v+.5,E.guide&&(E.path=p.route(E.body,E.guide)))})}else if(E.state==="walk"||E.state==="carry"||E.state==="guide"||E.state==="yield"){if(E.guide&&Math.hypot(E.body.x-E.guide.x,E.body.z-E.guide.z)>.5){E.play("idle"),v>E.until&&(E.path=p.route(E.body,E.guide),E.until=v+2.5);continue}E.guide=null,E.state=E.slot?.place==="meeting-charge"?"charge":E.slot?"work":"idle",E.until=v+5+E.index%4,E.play(E.state==="work"?"work":"idle"),E.slot&&r?.(E.slot.place,5),g++}else v>E.until&&(E.role==="courier"&&E.slot&&(E.state==="work"||E.state==="charge")&&(E.carrying=E.slot.place==="parcel-sorter",E.parcels.forEach(Fe=>{Fe.visible=E.carrying})),M(E))}}if(!V){x.update(0,!1);return}for(let E of c){if(!E.body.enabled||E.trafficYield||E.meeting||E.guide||E.state==="greet"||v<E.cooldown)continue;let Z=c.find(Q=>Q!==E&&Q.body.enabled&&!Q.trafficYield&&Q.state!=="greet"&&!Q.meeting&&!Q.guide&&v>=Q.cooldown&&Math.abs(E.body.y-Q.body.y)<2&&Math.hypot(E.body.x-Q.body.x,E.body.z-Q.body.z)>2.8&&Math.hypot(E.body.x-Q.body.x,E.body.z-Q.body.z)<6);if(!Z)continue;let W={a:E,b:Z,at:v,sent:!1,replied:!1};d.push(W);for(let Q of[E,Z])Q.meeting=W,Q.state="exchange",Q.pause?.(!0),Q.play?.("greet")}for(let E=d.length-1;E>=0;E--){let Z=d[E],W=v-Z.at;if(W>4.8||!Z.a.body.enabled||!Z.b.body.enabled){F(Z),d.splice(E,1);continue}for(let[Q,Fe]of[[Z.a,Z.b],[Z.b,Z.a]])e.propose(Q.body,{...Q.body,heading:wm(Q.body.heading,Math.atan2(Fe.body.x-Q.body.x,Fe.body.z-Q.body.z),D)},(Ne,ze)=>{Q.node.rotation.y=ze.heading});W>.6&&!Z.sent&&(x.send(Z.a.body,Z.b.body),Z.sent=!0),W>2.4&&!Z.replied&&(x.send(Z.b.body,Z.a.body),Z.replied=!0)}x.update(D,!0);let k=c.find(E=>E.id===m);k&&k.state!==_&&(_=k.state,i?.({...b(k),refresh:!0}))}function H(D){if(T)return;let V=y!=="low";for(let k of c){if(!k.height)continue;if(!k.body.enabled||!k.node.visible){k.shadow?.hide();continue}let E=k.body,Z=Ss(k.height,E.y,D),W=Math.sin(E.heading),Q=Math.cos(E.heading),Fe=n(E.x+W*.35,E.z+Q*.35)-n(E.x-W*.35,E.z-Q*.35),Ne=Ss(k.pitch,Math.max(-.08,Math.min(.08,Math.atan2(Fe,.7)*.5)),D);k.node.position.set(E.x,Z,E.z),k.node.rotation.y=E.heading,k.node.rotation.x=-Ne,V&&k.rig&&Sm(k.rig,k.node,n),k.shadow?.set(E.x,n(E.x,E.z),E.z,.62,.5)}}return{add:S,patrol:P,update:N,suspend:I,settle:H,addPlace(D,V){if(!h.has(D.id)){h.set(D.id,D);for(let k of V.navigation.workpoints||[])u.push({id:D.id+":"+k.id,place:D.id,role:D.role,x:D.x+k.position[0],y:n(D.x,D.z),z:D.z+k.position[2],owner:null})}},nearby(){let D=[];for(let V of c)if(V.body.enabled){let k=Math.hypot(t.position.x-V.body.x,t.position.z-V.body.z);k<5&&Math.abs(t.position.y-V.body.y)<8&&D.push({kind:"resident",id:V.id,distance:k,x:V.body.x,z:V.body.z})}for(let V of h.values()){let k=Math.hypot(t.position.x-V.x,t.position.z-V.z);k<7&&D.push({kind:"demonstrate",id:V.id,distance:k,x:V.x,z:V.z})}return D},inspect(D){let V=c.find(k=>k.id===D);return C(V),b(V)},action(D,V){let k=c.find(E=>E.id===m);if(!k)return null;if(k.meeting){let E=k.meeting,Z=d.indexOf(E);F(E),Z>=0&&d.splice(Z,1)}if(D==="greet"&&(A(k),k.guide=null,k.pause?.(!0),k.state="greet",k.until=v+3,k.cooldown=v+5,k.play?.("greet")),D==="guide"&&!k.pause&&V){A(k);for(let E of[0,1.5,3]){for(let Z=0;Z<(E?8:1)&&!k.path.length;Z++){let W=V.x+Math.cos(Z*Math.PI/4)*E,Q=V.z+Math.sin(Z*Math.PI/4)*E;k.path=p.route(k.body,{x:W,z:Q,y:n(W,Q)})}if(k.path.length)break}k.guide=k.path.length?k.path[k.path.length-1]:null,k.state=k.guide?"guide":"idle"}return D==="cancel"&&(A(k),k.guide=null,k.pause?.(!1),k.state=k.pause?"patrol":"idle",k.until=v+1),C(k),b(k)},demonstrate(D,V=!1){h.has(D)&&r?.(D,V?0:6)},setTier(D){y=D},setReplay(D){f!==D&&(f=D,I())},event(D){if(f||!a()||!["started","succeeded","failed","sanitized","progress"].includes(D.state)||!Number.isFinite(D.at)||Date.now()-D.at>2500||D.at>Date.now())return;let V=Ms.find(k=>k.district===(D.to==="agent"?D.from:D.to));V&&h.has(V.id)&&r?.(V.id,2,D.state)},stats:()=>({completed:g,rigs:c.filter(D=>D.rig).length,meetings:d.length,reservations:u.filter(D=>D.owner).length,signals:x.stats(),residents:c.filter(D=>D.body.enabled).map(D=>({...b(D),x:D.body.x,z:D.body.z,heading:D.body.heading})),places:[...h.keys()]}),dispose(){T=!0,I(),c.forEach(D=>e.remove(D.id)),c.length=0,h.clear(),u.length=0,x.dispose()}}}function Am(s){let e=new vt;e.name="living-machinery",s.add(e);let t=new Set,n=new Set,i=new Map,r=0,a="high",o=T=>(t.add(T),T),l=T=>(n.add(T),T),c=o(new Kn(1,.025,6,64)),h=o(new Cr(1,48)),u=o(new _t),d=new Float32Array(288),p=new Float32Array(96);for(let T=0;T<96;T++)d[T*3]=Math.cos(T*2.399)*Math.sqrt(T/96),d[T*3+2]=Math.sin(T*2.399)*Math.sqrt(T/96),p[T]=T/96;u.setAttribute("position",new Rt(d,3)),u.setAttribute("phase",new Rt(p,1));for(let T of Ms){let g=new vt;g.name=T.id,g.position.set(T.x,.12,T.z),g.visible=!1,e.add(g);let _=l(new Vt({color:7529695,transparent:!0,opacity:.15,depthWrite:!1,blending:tn,toneMapped:!1})),w=[];for(let C=0;C<3;C++){let S=new st(c,l(_.clone()));S.rotation.x=-Math.PI/2,g.add(S),w.push(S)}let A=new st(h,l(_.clone()));A.rotation.x=-Math.PI/2,A.scale.setScalar(2.5),A.position.y=.2,g.add(A);let b=new yn(u,l(new yt({transparent:!0,depthWrite:!1,blending:tn,uniforms:{time:{value:0},strength:{value:0},color:{value:new Le(10414816)}},vertexShader:"attribute float phase;uniform float time;uniform float strength;varying float a;void main(){float t=fract(phase+time*.31);vec3 p=position*vec3(2.,1.,2.);p.y=t*3.;p.xz*=1.-t*.7;vec4 v=modelViewMatrix*vec4(p,1.);a=sin(t*3.14159)*strength;gl_PointSize=clamp(65./max(1.,-v.z),1.,5.);gl_Position=projectionMatrix*v;}",fragmentShader:"uniform vec3 color;varying float a;void main(){float d=length(gl_PointCoord-.5);gl_FragColor=vec4(color,a*(1.-smoothstep(.05,.5,d)));}"})));g.add(b),i.set(T.id,{group:g,waves:w,base:A,motes:b,district:T.district,started:0,until:0,source:"ambient",model:null})}let x=new st(o(new fn(3.9,2.3)),l(new yt({transparent:!0,depthWrite:!1,side:Kt,blending:tn,uniforms:{time:{value:0},strength:{value:0}},vertexShader:"varying vec2 q;void main(){q=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 q;uniform float time;uniform float strength;void main(){float line=pow(max(0.,1.-abs(q.y-fract(time*.4))*14.),3.);float grid=step(.94,fract(q.x*24.))*step(.9,fract(q.y*18.));gl_FragColor=vec4(.18,.9,.8,(line*.36+grid*.1)*sin(q.x*3.14159)*strength);}"})));x.position.set(0,1.4,.8),i.get("repair-bay").group.add(x);let v=i.get("kinetic-fountain"),m=l(new yt({transparent:!0,depthWrite:!1,side:Kt,uniforms:{time:{value:0}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 vUv;uniform float time;void main(){float pulse=.5+.5*sin(vUv.x*95.-time*9.);float edge=.35+.65*pow(max(0.,sin(vUv.y*3.14159)),2.);gl_FragColor=vec4(mix(vec3(.09,.35,.42),vec3(.6,.95,1.),pow(max(0.,pulse),5.)),edge*.56);}"}));for(let T=0;T<8;T++){let g=T*Math.PI/4,_=new B(Math.cos(g)*2.3,.5,Math.sin(g)*2.3),w=new B(Math.cos(g+.5)*.7,.55,Math.sin(g+.5)*.7),A=new mi(_,new B(Math.cos(g)*1.5,3.7,Math.sin(g)*1.5),w);v.group.add(new st(o(new Ca(A,24,.045,5,!1)),m))}let f=l(new yt({transparent:!0,depthWrite:!1,uniforms:{time:{value:0}},vertexShader:"varying vec2 q;uniform float time;void main(){q=position.xy;vec3 p=position;p.z+=sin(length(q)*18.-time*4.)*.012;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.);}",fragmentShader:"varying vec2 q;uniform float time;void main(){float r=length(q);float a=.5+.5*sin(r*16.-time*3.+sin(q.x*3.+q.y*2.+time*.7)*.35);float rim=1.-smoothstep(2.35,2.58,r);gl_FragColor=vec4(mix(vec3(.03,.13,.17),vec3(.22,.55,.58),pow(max(0.,a),9.)*.3),.78*rim);}"})),y=new st(o(new Cr(2.58,64)),f);return y.rotation.x=-Math.PI/2,y.position.y=.3,v.group.add(y),{attach(T,g){let _=i.get(T);_&&(_.model=g,_.group.visible=!0)},demonstrate(T,g=6,_=null){let w=i.get(T);w&&(w.until<=r&&(w.started=r),w.until=r+g,w.source=_?"live":"ambient",w.status=_,w.model?.play("operate"))},clear(){i.forEach(T=>{T.until=0,T.source="ambient",T.status=null})},setTier(T){a=T},update(T,g){g&&(r+=T),m.uniforms.time.value=f.uniforms.time.value=x.material.uniforms.time.value=r;for(let[_,w]of i){let A=w.until>r,b=_==="kinetic-fountain",C=A?Ft.smoothstep(r-w.started,0,.35)*Ft.smoothstep(w.until-r,0,.65):0,S=b?Math.max(.25,C):C;A||(w.source="ambient",w.status=null);let P=w.status==="failed"?16737881:w.source==="live"?{infra:7978495,integrations:7728086,missions:10268415,graph:16765844,operations:16758915}[w.district]:7529695;if(w.waves.forEach((M,F)=>{M.visible=A;let I=(r*.6+F/3)%1;M.position.y=.4+I*2,M.scale.setScalar(.4+I*2.6),M.material.opacity=(1-I)*.22*C,M.material.color.setHex(P)}),_==="repair-bay"&&(x.visible=A,x.material.uniforms.strength.value=C),_==="relay-mast"&&A){let M=w.model?.node.getObjectByName("antenna");M&&(M.rotation.y=Ft.damp(M.rotation.y,Math.atan2(-w.group.position.x,-12-w.group.position.z),3,g?T:0))}w.base.material.opacity=.08*C,w.motes.visible=a!=="low"&&S>0,w.motes.material.uniforms.time.value=r,w.motes.material.uniforms.strength.value=S,w.model?.mixer&&(w.model.mixer.timeScale=g&&(A||b)?.65:0)}},stats:()=>({places:[...i].filter(([,T])=>T.model).map(([T,g])=>({id:T,operating:g.until>r,source:g.source})),time:r}),dispose(){e.removeFromParent(),t.forEach(T=>T.dispose()),n.forEach(T=>T.dispose()),i.clear()}}}var Zy={cruise:12,boost:26,climb:7,sink:7,ease:5,eye:2.4,takeoff:1.5,takeoffTime:.7,ceiling:150,minX:-140,maxX:140,minZ:-150,maxZ:130,glide:8,descend:6,reach:40,bob:.03,slack:15},Nt=Zy,Zs=(s,e,t)=>Math.max(e,Math.min(t,s)),$y=s=>s<=0?0:s>=1?1:s*s*(3-2*s);function Rm({clear:s,ground:e,walkable:t,reduced:n=()=>!1}){let i="idle",r=!1,a=0,o=0,l=0,c=0,h=0,u=0,d=0,p=0,x=0,v=null,m=0,f=!1,y=0,T=0,g=(M,F)=>e(M,F)+Nt.eye,_=()=>({x:a,y:o+m,z:l}),w=()=>{c=h=u=0},A=(M,F,I)=>(M+=(F-M)*(1-Math.exp(-Nt.ease*I)),Math.abs(M)<.01&&!F?0:M);function b(M,F=[]){for(let I=0;I<=Math.max(Nt.maxX-Nt.minX,Nt.maxZ-Nt.minZ);I+=I<Nt.reach?1.5:4){let N=I?Math.ceil(I*Math.PI*2/(I<Nt.reach?1.5:4)):1;for(let H=0;H<N;H++){let D=H/N*Math.PI*2,V=a+Math.cos(D)*I,k=l+Math.sin(D)*I;if(V<Nt.minX||V>Nt.maxX||k<Nt.minZ||k>Nt.maxZ||F.some(Z=>Math.hypot(Z.x-V,Z.z-k)<1)||!t(V,k))continue;let E=g(V,k);if(!(M&&!s({x:V,y:Math.max(o,E),z:k},{x:V,y:E,z:k})))return{x:V,y:E,z:k,blocked:0,skipped:F}}}return null}let C=M=>{a=M.x,o=M.y,l=M.z,w(),m=0,i="idle",r=!0,v=null};function S(M){let F=a,I=l,N=Zs(a+c*M,Nt.minX,Nt.maxX),H=Zs(l+u*M,Nt.minZ,Nt.maxZ);N!==a+c*M&&(c=0),H!==l+u*M&&(u=0),N!==a&&(s({x:a,y:o,z:l},{x:N,y:o,z:l})?a=N:c=0),H!==l&&(s({x:a,y:o,z:l},{x:a,y:o,z:H})?l=H:u=0);let D=g(a,l);o<D&&(s({x:a,y:o,z:l},{x:a,y:D,z:l})?o=D:(a=F,l=I,c=u=0,D=g(a,l)));let V=Zs(o+h*M,D,Nt.ceiling);V!==o+h*M&&(h=0),V!==o&&(s({x:a,y:o,z:l},{x:a,y:V,z:l})?o=V:h=0)}function P(M){let F=v,I=F.x-a,N=F.z-l,H=Math.hypot(I,N),D=o-F.y;if(y+=M,y>T){let Fe=b(!1);Fe?C(Fe):(i="flying",v=null);return}if(H<.05&&D<.02){C(F);return}let V=H>.05?Math.min(Nt.glide,Math.max(.8,H*1.4)):0,k=D>.02?Math.min(Nt.descend,Math.max(.6,D*1.8)):0,E=H>.05?I/H*Math.min(V*M,H):0,Z=H>.05?N/H*Math.min(V*M,H):0,W=-Math.min(k*M,Math.max(0,D)),Q={x:a,y:o,z:l};if(s(Q,{x:a+E,y:o+W,z:l+Z}))a+=E,o+=W,l+=Z,c=E/M,u=Z/M,h=W/M,F.blocked=0;else if((E||Z)&&s(Q,{x:a+E,y:o,z:l+Z}))a+=E,l+=Z,c=E/M,u=Z/M,h=0,F.blocked=0;else if(w(),H>=.05&&o<Nt.ceiling&&s(Q,{x:a,y:o+Nt.descend*M,z:l}))o+=Nt.descend*M,h=Nt.descend,F.blocked=0;else if((F.blocked+=M)>1){let Fe=b(!0,[...F.skipped,F]);Fe?v=Fe:(i="flying",v=null)}}return{start(M){return a=M.x,o=Math.max(M.y,g(M.x,M.z)),l=M.z,w(),i="takeoff",r=!1,d=0,p=o,x=0,v=null,m=0,_()},place(M){a=M.x,o=M.y,l=M.z,w(),m=0},step(M,F={},I=0){if(M=Zs(M,0,.1),i==="landing"&&(+F.forward||+F.right||+F.up?f&&(i="flying",v=null):f=!0),i==="takeoff"){d=n()?Nt.takeoffTime:Math.min(Nt.takeoffTime,d+M);let N=p+Nt.takeoff*$y(d/Nt.takeoffTime);N>o&&s({x:a,y:o,z:l},{x:a,y:N,z:l})&&(o=N),d>=Nt.takeoffTime&&(i="flying")}else if(i==="landing")M>0&&P(M);else if(i==="flying"){let N=Zs(+F.forward||0,-1,1),H=Zs(+F.right||0,-1,1),D=Math.hypot(N,H);D>1&&(N/=D,H/=D);let V=F.boost?Nt.boost:Nt.cruise,k=Math.sin(I),E=Math.cos(I),Z=Zs(+F.up||0,-1,1);c=A(c,(-k*N+E*H)*V,M),u=A(u,(-E*N-k*H)*V,M),h=A(h,Z*(Z>0?Nt.climb:Nt.sink),M),S(M),x+=M,m=n()?0:Math.min(Nt.bob,Math.max(0,o-g(a,l)),Nt.ceiling-o)*Math.sin(x*2.1)}return _()},land(M=!1){let F=M||n();if(i==="idle")return!1;if(i==="landing"&&!F)return!0;let I=b(!F);return I?F?(C(I),!0):(v=I,i="landing",m=0,f=!1,y=0,T=Nt.slack+Math.max(Math.hypot(I.x-a,I.z-l)/Nt.glide,Math.max(0,o-I.y)/Nt.descend),!0):!1},cancel(){return i!=="landing"?!1:(i="flying",v=null,!0)},reset(){i="idle",r=!1,v=null,w(),m=0},state(){return{phase:i,flying:i!=="idle",landing:i==="landing",target:v?{x:v.x,z:v.z}:null,done:r,x:a,y:o+m,z:l,altitude:Math.max(0,o-g(a,l)),speed:Math.hypot(c,u),climb:h}}}}function Cm(s,e){let t=new vt;t.name="world-2",s.add(t);let n=new Map,i=new Set,r=new Set,a=[],o=[],l=[],c=new Map,h=new Map,u=[],d=new Map,p=new Map,x=new Set,v=[],m=new Set,f=[],y=new Map,T=new AbortController,g=[],_=0,w=!1,A=!1,b=e.tier||"high",C=null,S=0,P=0,M=null,F=null,I=0,N=new Set,H=null,D=0,V="",k=new B,E=new B,Z=new B,W=e.camera,Q={ready:!1,lift:null,cab:null,value:0,motion:null,idle:0},Fe=[],Ne=new kn(0,0,0,"YXZ"),ze=U=>Math.atan2(Math.sin(U),Math.cos(U)),se=U=>U<=0?0:U>=1?1:U*U*(3-2*U),le=e.traffic||Nc(),De=Am(s),be=new Map;function ue(U,q){let Y=Ln(U,q);for(let ae of Ms)for(let K of C?.assets.find($=>$.id===ae.id)?.navigation.surfaces||[]){let[$,Re,He,ot]=K.rect;U>=ae.x+$&&U<=ae.x+He&&q>=ae.z+Re&&q<=ae.z+ot&&(Y=Math.max(Y,Ln(ae.x,ae.z)+K.height))}return Y}let te=(U,q)=>{let Y=qp(U,q,e.districts||[]);return Y>0?Math.max(ue(U,q),Y):ue(U,q)},pe=[],ce=()=>le.body("visitor"),ne=new B,de=Rm({ground:te,reduced:()=>e.reduced(),clear:(U,q)=>e.clear?e.clear(U,q):!ce()||le.clear(ce(),{...U,heading:0},{...q,heading:0},!1),walkable:(U,q)=>{if(ys(U,q)||!Qu(U,q,{x:U,z:q},()=>!1,e.districts||[]))return!1;let Y={x:U,y:te(U,q)+2.4,z:q,heading:0};return!ce()||le.clear(ce(),Y,Y)}}),we=Tm(s,{traffic:le,camera:W,floor:ue,onSelect:e.onSociety,onDemonstrate:De.demonstrate,active:e.active,onGait:e.onGait,shadows:e.shadows}),Ve=0,Ke=0;try{N=new Set(JSON.parse(localStorage.getItem("aurago.desktop.sysworld.discoveries")||"[]").filter(U=>ri.some(q=>q.id===U)))}catch{}let it=Vu(qu),et=it.getLength(),ht=ri.map(U=>{let q=1/0,Y=0;for(let ae=0;ae<3e3;ae++){it.getPointAt(ae/3e3,k);let K=Math.hypot(k.x-U.x,k.z-U.z);K<q&&(q=K,Y=ae/3e3)}return{...U,u:Y}}),J=(U,q,Y)=>e.onSound?.(U,q?.x||0,q?.y||0,q?.z||0,Y),Ut=new zn({color:11834208,metalness:.8,roughness:.35});r.add(Ut);for(let U of[-1,1]){let q=[],Y=[];for(let $=0;$<=512;$++){it.getPointAt($/512,k),it.getTangentAt($/512,E);for(let Re of[-.045,.045]){let He=U*.85+Re;q.push(k.x-E.z*He,gt.road+.09,k.z+E.x*He)}if($<512){let Re=$*2;Y.push(Re,Re+1,Re+2,Re+1,Re+3,Re+2)}}let K=new _t;K.setAttribute("position",new mt(q,3)),K.setIndex(Y),K.computeVertexNormals(),i.add(K),t.add(new st(K,Ut))}async function ut(U,q=b==="low"?2:b==="medium"?1:0){let Y=U+":"+q;return n.has(Y)||n.set(Y,(async()=>{let ae=C.assets.find(Ze=>Ze.id===U),K=ae?.lods.find(Ze=>Ze.level===q);if(!K||!/^[\w-]+\.lod[0-2]\.glb$/.test(K.file))throw Error("Invalid world asset");let $=await fetch(e.assetURL(K.file),{signal:T.signal});if(!$.ok)throw Error("World asset unavailable");let Re=await $.arrayBuffer();if(A)throw Error("Disposed");S+=Re.byteLength;let He=await new yi().parseAsync(Re,""),ot=He.animations.length>0||/^(tram|service-cart|robot-|door|lift|sky-lift|telescope)/.test(U);if(He.scene.traverse(Ze=>{if(Ze.isMesh){i.add(Ze.geometry);for(let Jt of Array.isArray(Ze.material)?Ze.material:[Ze.material])r.add(Jt);Ze.castShadow=!0,Ze.receiveShadow=!0,e.surfaces?.prepare(Ze,ot)}}),A)throw i.forEach(Ze=>Ze.dispose()),r.forEach(Ze=>Ze.dispose()),Error("Disposed");return x.add(U),He})()),n.get(Y)}async function O(U,q,Y,ae=0,K=0,$=t,Re=[1,1,1],He){let ot=await ut(U,He);if(A)return null;let Ze=ot.scene.clone(!0);Ze.position.set(q,ae,Y),Ze.rotation.y=K,Ze.scale.set(...Re),$.add(Ze),Ze.traverse(Zt=>{Zt.isMesh&&(Zt.userData.worldAsset=U,Zt.userData.worldPart=Zt.name,v.push(Zt))});let Jt=C.assets.find(Zt=>Zt.id===U),Xt="furnishing:"+Ve++;for(let[Zt,wn]of(Jt.navigation.colliders||[]).entries())le.solid(Xt+":"+Zt,{owner:Xt,x:q,z:Y,heading:K,min:[wn[0]*Re[0],ae+wn[1]*Re[1],wn[2]*Re[2]],max:[wn[3]*Re[0],ae+wn[4]*Re[1],wn[5]*Re[2]]});let Mt=ot.animations.length?new Ba(Ze):null;Mt&&a.push(Mt);let Mn=null,kt=null,nn=/^robot-/.test(U);return{node:Ze,mixer:Mt,owner:Xt,bounds:Jt.motion_bounds||Jt.lods[0].bounds,clips:ot.animations,play(Zt,wn=!1){if(!Mt)return;let ai=ot.animations.find(Fn=>Fn.name===Zt);if(!ai)return;let En=Mt.clipAction(ai);if(Mn===En)return En;let Tn=kt;return En.reset(),wn&&(En.setLoop(pc,1),En.clampWhenFinished=!0),En.play(),Mn&&En.crossFadeFrom(Mn,.25,!1),Mn=En,kt=Zt,nn&&Tn&&Zt!==Tn&&Zt!=="walk"&&e.onSound?.("robot_"+Zt,Ze.position.x,Ze.position.y,Ze.position.z,Xt),En}}}function R(U){U.updateMatrixWorld(!0);let q=new Map;U.traverse(ae=>{if(!ae.isMesh)return;let K=ae.geometry.uuid+":"+(Array.isArray(ae.material)?ae.material.map($=>$.uuid).join(","):ae.material.uuid);q.has(K)||q.set(K,{n:ae,matrices:[]}),q.get(K).matrices.push(ae.matrixWorld.clone())});let Y=new vt;t.add(Y);for(let{n:ae,matrices:K}of q.values()){let $=new Sn(ae.geometry,ae.material,K.length);K.forEach((Re,He)=>$.setMatrixAt(He,Re)),$.castShadow=!0,$.receiveShadow=!0,$.userData={...ae.userData},v.push($),Y.add($)}return U.traverse(ae=>{let K=v.indexOf(ae);K>=0&&v.splice(K,1)}),U.removeFromParent(),d.set(Y.uuid,Y),Y}function re(U){if(p.has(U.id))return p.get(U.id);let q=(async()=>{let Y=new vt;t.add(Y);let ae=[];for(let K of zp())ae.push(O("floor",U.x+K.x,U.z+K.z,gt.room,0,Y,[K.sx,1,K.sz]));for(let K=-4;K<=4;K+=4)for(let $=-4;$<=4;$+=4)ae.push(O("ceiling",U.x+K,U.z+$,8.15+gt.room,0,Y));for(let K=-4;K<=4;K+=4)ae.push(O("wall",U.x+K,U.z+6,gt.room,0,Y));for(let K=-4;K<=4;K+=4)for(let $ of[-1,1])for(let Re of[0,4])ae.push(O("window",U.x+$*6,U.z+K,gt.room+Re,Math.PI/2,Y));for(let K=-4;K<=4;K+=4)for(let $ of[-1,1])ae.push(O("window",U.x+K,U.z+$*6,gt.gallery,0,Y));for(let K of[-4,4])ae.push(O("wall",U.x+K,U.doorZ,gt.room,0,Y));for(let[K,$]of[[-4,1],[.2,1.1]])ae.push(O("floor",U.x+K,U.z+3,gt.gallery,0,Y,[$,1,1.5])),ae.push(O("railing",U.x+K,U.z,gt.gallery,0,Y,[$,1,1]));for(let K of[.7,5.3])ae.push(O("railing",U.x+2.4,U.z+K,gt.gallery,Math.PI/2,Y,[.35,1,1]));for(let K of[-5.6,1.9])ae.push(O("wall",U.x+K,U.z+.15,gt.room,0,Y,[.05,1,.6]));if(await Promise.all(ae),!A)return R(Y)})();return p.set(U.id,q),q}async function oe(U){if(!(c.has(U.id)||A)){c.set(U.id,{ready:!1,lift:null,liftValue:0,liftTarget:0}),D++;try{await re(U);let q=new vt;t.add(q);let Y=[];for(let K of[-3,0])Y.push(O(U.id==="missions"?"cargo":U.id==="memory"?"archive-shelf":"console",U.x+K,U.z+3,gt.room,Math.PI,q));if(Y.push(O("bench",U.x-4,U.z-1,gt.room,Math.PI/2,q)),Y.push(O("bench",U.x-4,U.z+4.8,gt.gallery,0,q)),Y.push(O("console",U.x,U.z+4.8,gt.gallery,Math.PI,q)),await Promise.all(Y),A)return;R(q);let ae=c.get(U.id);ae.hologram=await O("hologram",U.x-1,U.z,gt.room),ae.hologram?.play("operate"),U.id==="memory"&&ke(),ae.lift=await O("lift",U.liftX,U.liftZ,gt.room),ae.liftAction=ae.lift?.play("operate",!0),ae.liftAction&&(ae.liftAction.paused=!0),await G(U.id,U.x-2.2,U.z+.9,gt.gallery,Math.PI,{skip:null,limit:.9,pitch:[-.14,.18],eye:0}),ae.ready=!0}catch(q){A||e.onError?.(q)}finally{D--}}}async function xe(){try{let U=await fetch(e.assetURL("manifest.json"),{signal:T.signal});if(!U.ok)throw Error("World manifest unavailable");C=await U.json();let q=new vt;t.add(q);let Y=[];for(let $ of ri)Y.push(O("station",$.platformX,$.platformZ,gt.pavement,$.angle,q));for(let $ of Dn){Y.push(re($));let Re=await O("door",$.x,$.doorZ,gt.room,Math.PI);if(A)return;let He=Re.play("open",!0);He.paused=!0,h.set($.id,{...Re,action:He,value:0,open:!1})}for(let $=-76;$<=76;$+=8)Y.push(O("quay",$,77,gt.quay,0,q));for(let $ of[-68,-52,-28,28,52,68])Y.push(O("quay-lantern",$,75.5,gt.quay,0,q));for(let[$,Re]of[[-79,68],[-79,-67],[55,2],[8,40],[55,-24]])Y.push(O("garden",$,Re,0,0,q)),Y.push(O("bench",$+3,Re,0,Math.PI/2,q));for(let[$,Re]of[[-30,23],[30,23],[-28,72]])Y.push(O("arcade",$,Re,0,0,q));Y.push(O("bridge",-78.5,40,2,0,q,[1.75,1,1]),O("ramp",-80,49,0,Math.PI,q),O("stairs",-76.8,49,0,Math.PI,q),O("ramp",-78.5,31,0,0,q)),Y.push(O("pad",zi.x,zi.z,0,0,q)),Y.push(O("sky-deck",qt.x,qt.z,qt.floor,0,q));for(let $ of qt.benches)Y.push(O("bench",$.x,$.z,qt.floor,$.angle,q));for(let[$,Re]of[[-60,-56],[-60,-46],[51,49]])Y.push(O("charger",$,Re,0,0,q));if(await Promise.all(Y),A||(R(q),Q.lift=await O("sky-lift",qt.lift.x,qt.lift.z,qt.lift.base),A))return;Q.cab=Q.lift?.node.getObjectByName("cab")||null,Q.ready=!!Q.cab;for(let $ of qt.telescopes)await G($.id,$.x,$.z,qt.floor,$.angle,{skip:"agent",limit:1.45,pitch:[-1.1,.3],eye:1.25});for(let[$,Re]of[[-57,-61],[-57,-52]])(await O("cooler",$,Re))?.play("operate");for(let $ of Ms){let Re=await O($.id,$.x,$.z,Ln($.x,$.z),0,t,[1,1,1],2);if(A)return;Re&&(Re.level=2,be.set($.id,Re),De.attach($.id,Re),we.addPlace($,C.assets.find(He=>He.id===$.id)),Re.play("operate"))}for(let $ of hm){let Re=await O($.id,$.x,$.z,$.y,$.angle||0,t);if(!Re){if(A)return;continue}Re.play($.clip),u.push({id:$.id,model:Re})}let ae=C.assets.find($=>$.id==="hover-pack");if(ae)try{let $=ae.navigation.interaction?.[0]?.position||[0,0,1.2];for(let Re of Up){let He=await O("hover-pack",Re.x,Re.z,te(Re.x,Re.z),Re.angle);if(A)return;if(!He)continue;He.play("idle");let ot=Math.sin(Re.angle),Ze=Math.cos(Re.angle);pe.push({...Re,model:He,pack:He.node.getObjectByName("pack")||null,docked:!0,ix:Re.x+$[0]*Ze+$[2]*ot,iz:Re.z-$[0]*ot+$[2]*Ze})}}catch($){A||e.onError?.($)}let K=await O("service-cart",55,49,gt.ground,Math.PI/2);K&&K.play("open",!0);for(let $=0;$<19;$++){let Re=["courier","technician","archivist"][$%3],He=await O("robot-"+Re,0,0);if(!He)return;He.node.scale.setScalar(1.15),o.push(He),we.add(He,$,He.bounds),He.play("idle")}for(let $=0;$<2;$++){let Re=await O("tram",0,0);if(!Re)return;let He=Re.play("open",!0);He.paused=!0;let ot=ht[$?3:0].u;it.getPointAt(ot,k),it.getTangentAt(ot,E);let Ze=le.register("tram-"+$,ts(Re.bounds),{x:k.x,y:gt.road+.03,z:k.z,heading:Math.atan2(E.x,E.z)},3);l.push({...Re,body:Ze,u:ot,speed:0,dwell:5,stop:$?3:0,door:He})}for(let $=0;$<3;$++){let Re=await O("cargo",43,70);Re&&(le.removeOwner(Re.owner),Re.node.visible=!1,f.push({...Re,body:null,elapsed:9,direction:1}))}e.onReady?.()}catch(U){A||e.onError?.(U)}}xe();async function Ue(U,q,Y){if(!(q.level===Y||q.requested===Y)){q.requested=Y;try{let ae=await ut(U,Y);if(A||q.requested!==Y)return;let K=new Map;ae.scene.traverse($=>{$.isMesh&&K.set($.name,$)}),q.node.traverse($=>{let Re=K.get($.name);$.isMesh&&Re&&($.geometry=Re.geometry,$.material=Re.material)}),q.level=Y,q.requested=null,e.onReady?.()}catch(ae){q.requested=null,A||e.onError?.(ae)}}}function ke(){let U=c.get("memory");if(!U?.hologram||!U.text&&!g.length)return;if(!U.text){let $=document.createElement("canvas");$.width=1024,$.height=512;let Re=new zs($);Re.colorSpace=sn,m.add(Re);let He=new fn(4.8,2.4),ot=new Vt({map:Re,transparent:!0,depthWrite:!1,side:Kt,toneMapped:!1});i.add(He),r.add(ot);let Ze=new st(He,ot),Jt=Dn.find(Xt=>Xt.id==="memory");Ze.position.set(Jt.x-1,3.6+gt.room,Jt.z+.3),Ze.rotation.y=Math.PI,t.add(Ze),U.text={canvas:$,texture:Re,mesh:Ze}}let{canvas:q,texture:Y,mesh:ae}=U.text,K=q.getContext("2d");K.clearRect(0,0,1024,512),ae.visible=g.length>0,K.fillStyle="rgba(5,24,32,.87)",K.fillRect(0,0,1024,512),K.fillStyle="#b9f4ef",K.font="28px sans-serif",g.slice(0,4).forEach(($,Re)=>{let He=Array.from($).slice(0,96);K.fillText(He.slice(0,48).join(""),30,55+Re*118),K.fillText(He.slice(48).join(""),30,94+Re*118)}),Y.needsUpdate=!0}async function G(U,q,Y,ae,K,$){let Re=await O("telescope",q,Y,ae,K);!Re||A||Fe.push({id:U,model:Re,tube:Re.node.getObjectByName("tube"),x:q,z:Y,floor:ae,angle:K,...$,ex:q+Math.sin(K)*$.eye,ez:Y+Math.cos(K)*$.eye})}function j(U){let q=Fe.find(He=>He.id===U);if(!q)return;let Y=ze(q.angle+Math.PI),ae=q.floor+2.05,K=Y,$=-.08,Re=1/0;for(let He of e.districts||[]){if(He.id===q.skip)continue;let ot=He.x-q.ex,Ze=He.z-q.ez,Jt=Math.hypot(ot,Ze),Xt=Math.atan2(-ot,-Ze),Mt=Math.abs(ze(Xt-Y));Jt>12&&Mt<q.limit&&Mt<Re&&(Re=Mt,K=Xt,$=Ft.clamp(Math.atan2(He.height*.45-ae,Jt),...q.pitch))}M={kind:"scope",id:U,scope:q,yaw0:Y,fov:14,sent:"",target:null,back:{position:W.position.clone(),quaternion:W.quaternion.clone(),fov:W.fov}},q.model.node.visible=!1,W.position.set(q.ex,ae,q.ez),W.quaternion.setFromEuler(Ne.set($,K,0,"YXZ")),e.reduced()&&(W.fov=M.fov,W.updateProjectionMatrix()),e.onRide?.("scope"),J("door",W.position)}function he(U,q,Y){let ae=Y.radius*.6,K=U.x-Y.x,$=U.z-Y.z,Re=q.x*q.x+q.z*q.z;if(Re<1e-9)return 1/0;let He=2*(K*q.x+$*q.z),ot=He*He-4*Re*(K*K+$*$-ae*ae);if(ot<0)return 1/0;let Ze=(-He-Math.sqrt(ot))/(2*Re);if(Ze<0&&(Ze=(-He+Math.sqrt(ot))/(2*Re)),Ze<0)return 1/0;let Jt=U.y+q.y*Ze;return Jt>=0&&Jt<=Y.height?Ze:1/0}function Se(U){let q=M.scope;Ne.setFromQuaternion(W.quaternion,"YXZ"),Ne.y=M.yaw0+Ft.clamp(ze(Ne.y-M.yaw0),-q.limit,q.limit),Ne.x=Ft.clamp(Ne.x,...q.pitch),Ne.z=0,W.quaternion.setFromEuler(Ne),W.position.set(q.ex,q.floor+2.05,q.ez);let Y=e.reduced()?M.fov:Ft.damp(W.fov,M.fov,9,Math.min(U,.1));Math.abs(Y-M.fov)<.02&&(Y=M.fov),Y!==W.fov&&(W.fov=Y,W.updateProjectionMatrix()),W.getWorldDirection(k);let ae=null,K=1/0;for(let ot of e.districts||[]){if(ot.id===q.skip)continue;let Ze=he(W.position,k,ot);Ze>12&&Ze<K&&(K=Ze,ae=ot.id)}M.target=ae;let $=M.back.fov/W.fov,Re=(Math.atan2(k.x,-k.z)*180/Math.PI+360)%360,He=[ae,ae?Math.round(K):0,$.toFixed(1),Math.round(Re)].join();He!==M.sent&&(M.sent=He,e.onScope?.({id:M.id,target:ae,distance:ae?Math.round(K):null,zoom:+$.toFixed(1),bearing:Math.round(Re)%360}))}function ie(U,q,Y){Q.motion={from:Q.value,to:U,t:0,duration:e.reduced()?0:q},Q.idle=0,Y&&J("lift",Q.lift.node.position)}function ve(){let U=qt.lift;W.position.set(U.x,U.base+U.cab+Q.value+2.4,U.z)}function Ee(U){ie(U,15,!0),M={kind:"sky",to:U},ve(),W.quaternion.setFromEuler(Ne.set(-.42,Math.PI,0,"YXZ")),e.onRide?.("sky")}function Te(U,q){let Y=qt.lift;if(!Q.motion&&M?.kind!=="sky"&&q>0&&Math.hypot(W.position.x-Y.x,W.position.z-Y.z)>30&&(Q.idle+=q,Q.idle>28&&ie(Q.value>1?0:Y.travel,15,!1)),Q.motion){let ae=Q.motion;ae.t+=Math.min(U,.1);let K=ae.duration?ae.t/ae.duration:1;Q.value=ae.from+(ae.to-ae.from)*se(K),K>=1&&(Q.value=ae.to,Q.motion=null)}Q.cab.position.y=Q.value,M?.kind==="sky"&&(ve(),Q.motion||(M=null,e.onRide?.(null),J("lift",W.position)))}function Ie(U){let q=pe.find(Y=>Y.id===U);!q?.docked||e.reduced()||(q.docked=!1,q.pack&&(q.pack.visible=!1),M={kind:"hover",id:U},V="",ne.copy(W.position),de.start(W.position),e.onRide?.("hover"),J("hover",W.position),Me())}function X(U){let q=pe.find(Y=>Y.id===M?.id);q&&(q.docked=!0,q.pack&&(q.pack.visible=!0)),M=null,V="",e.onRide?.(null),e.onFlight?.(null),U&&J("hover",W.position)}function Ae(){let U=ce();U&&le.relocate(U,W.position)&&W.position.set(U.x,U.y,U.z)}function Me(){let U=de.state();if(U.done){W.position.set(U.x,U.y,U.z),Ae(),X(!0);return}let q=[Math.round(U.altitude),Math.round(U.speed*3.6),U.landing].join();q!==V&&(V=q,e.onFlight?.({altitude:Math.round(U.altitude),speed:Math.round(U.speed*3.6),landing:U.landing}))}function fe(){if(M?.kind==="hover"){de.state().landing?de.cancel():de.land(),Me();return}if(M){if(M.kind==="tram"&&l[M.index].dwell<=1){M.exitRequested=!0;return}Pe();return}if(F){if(F.kind==="telescope"){j(F.id);return}if(F.kind==="skycall"){ie(W.position.y>40?qt.lift.travel:0,7,!0);return}if(F.kind==="skyup"||F.kind==="skydown"){Ee(F.kind==="skyup"?qt.lift.travel:0);return}if(F.kind==="resident"){we.inspect(F.id);return}if(F.kind==="demonstrate"){we.demonstrate(F.id,e.reduced()),e.onSociety?.({id:F.id,role:"installation",state:e.reduced()?"idle":"work",source:"ambient"});return}if(F.kind==="door"){let U=h.get(F.id);if(U){U.open=!U.open,J("door",U.node.position);let q=Dn.find(Y=>Y.id===F.id);oe(q)}}if(F.kind==="discover"){N.add(F.id);try{localStorage.setItem("aurago.desktop.sysworld.discoveries",JSON.stringify([...N]))}catch{}e.onDiscover?.(F.id),J("discover",W.position)}if(F.kind==="tram"&&!e.reduced()&&(M={kind:"waiting",station:F.id},e.onRide?.("waiting")),F.kind==="terminal"&&e.onTerminal?.(F.id),F.kind==="hoverpack"){Ie(F.id);return}if(F.kind==="drone"&&!e.reduced()&&(M={kind:"drone",elapsed:0,origin:W.position.clone()},e.onRide?.("drone"),J("tram",W.position)),F.kind==="lift"){let U=c.get(F.id);if(U){let q=W.position.y>5?4:0;if(Math.abs(U.liftValue-q)>.02){U.liftTarget=q;return}U.liftTarget=q?0:4,M={kind:"lift",id:F.id},e.onRide?.("lift"),J("lift",W.position)}}}}function Pe(){if(M){if(M.kind==="hover"){let U=pe.find(q=>q.id===M.id);if(de.land(!0)){let q=de.state();W.position.set(q.x,q.y,q.z)}else U&&W.position.set(U.ix,te(U.ix,U.iz)+2.4,U.iz);Ae(),de.reset(),X(!1);return}if(M.kind==="lift"){let U=c.get(M.id),q=Dn.find(Y=>Y.id===M.id);U.liftTarget=U.liftValue<2?0:4,W.position.set(q.x+1.5,2.4+gt.room+U.liftTarget,q.liftZ)}if(M.kind==="tram"){let U=ri.find(q=>q.id===M.station)||ri[0];W.position.set(U.platformX,2.4+Ln(U.platformX,U.platformZ),U.platformZ)}if(M.kind==="drone"&&W.position.copy(M.origin),M.kind==="sky"&&(Q.motion=null,Q.value=M.to,Q.cab.position.y=Q.value,ve()),M.kind==="scope"){let U=M.scope;Ne.setFromQuaternion(W.quaternion,"YXZ"),U.tube?.rotation.set(-Ne.x,ze(Ne.y+Math.PI-U.angle),0,"YXZ");let q=Ne.y;U.model.node.visible=!0,W.position.copy(M.back.position),Ne.setFromQuaternion(M.back.quaternion,"YXZ"),Ne.y=q,W.quaternion.setFromEuler(Ne),W.fov=M.back.fov,W.updateProjectionMatrix(),e.onScope?.(null)}M=null,e.onRide?.(null)}}function _e(){if(M?.kind==="hover")return de.state().landing?[]:[{kind:"land",id:M.id,distance:0}];if(M)return[{kind:M.kind==="scope"?"unscope":"exit",id:M.kind,distance:0}];W.getWorldDirection(k);let U=we.nearby().filter(Y=>(Y.x-W.position.x)*k.x+(Y.z-W.position.z)*k.z>Y.distance*.3),q=(Y,ae,K,$,Re)=>{let He=Math.hypot(W.position.x-K,W.position.z-$);He<Re&&U.push({kind:Y,id:ae,distance:He})};for(let Y of Dn)W.position.y<5&&q("door",Y.id,Y.x,Y.doorZ,3),c.get(Y.id)?.ready&&(q("lift",Y.id,Y.liftX,Y.liftZ,2.3),W.position.y<5&&q("terminal",Y.id,Y.x-2,Y.z+3,2.5));if(W.position.y<5)for(let Y of ri)q("tram",Y.id,Y.platformX,Y.platformZ,4),N.has(Y.id)||q("discover",Y.id,Y.platformX+(Y.angle===0?5:0),Y.platformZ+(Y.angle===0?0:5),3);if(q("drone","drone",zi.x,zi.z,5),W.position.y<5&&!e.reduced())for(let Y of pe)Y.docked&&q("hoverpack",Y.id,Y.ix,Y.iz,2.4);if(Q.ready){let Y=qt.lift,ae=W.position.y>40,K=Math.hypot(W.position.x-Y.x,W.position.z-Y.z),$=ae?Y.travel:0;K<(ae?3.6:2.4)&&(ae||W.position.y<10)&&U.push({kind:Q.motion||Math.abs(Q.value-$)>.05?"skycall":ae?"skydown":"skyup",id:"sky",distance:K})}for(let Y of Fe)Math.abs(W.position.y-(Y.floor+2.4))<1.5&&q("telescope",Y.id,Y.x,Y.z,1.7);return U.sort((Y,ae)=>Y.distance-ae.distance)}function qe(U,q,Y){if(A||!C)return;let ae=q?Math.min(U,.05):0;if(P+=ae,H=(M?.kind==="hover"&&W.position.y>8.3?null:ys(W.position.x,W.position.z)?.id)||null,Ke+=U,Ke>.5){Ke=0;for(let[K,$]of be){let Re=W.position.distanceTo($.node.position),He=b==="low"||Re>55?2:b==="medium"||Re>32?1:0;Ue(K,$,He)}}e.traffic||le.begin();for(let K of Dn)Math.hypot(W.position.x-K.x,W.position.z-K.z)<32&&oe(K);a.forEach(K=>{K.getRoot().visible&&K.update(ae)});for(let K of h.values()){let $=Ft.damp(K.value,K.open?1:0,5,Math.min(U,.1));Math.abs($-(K.open?1:0))<.001&&($=K.open?1:0);let Re=He=>[-1,1].map(ot=>({owner:K.owner,x:K.node.position.x,z:K.node.position.z,min:[ot*.8+ot*1.6*He-.8,gt.room,-.13],max:[ot*.8+ot*1.6*He+.8,gt.room+3.6,.13]}));(K.open||Re($).every(He=>le.solidClear(He)))&&(K.value=$),Re(K.value).forEach((He,ot)=>le.solid(K.owner+":panel:"+ot,He)),K.action.time=K.value*K.action.getClip().duration,K.mixer.update(0)}Q.ready&&Te(U,ae),M?.kind==="scope"&&Se(U);for(let[K,$]of c)if($.liftAction&&($.liftValue=Ft.damp($.liftValue,$.liftTarget,1.8,Math.min(U,.1)),Math.abs($.liftValue-$.liftTarget)<.02&&($.liftValue=$.liftTarget),$.liftAction.time=$.liftValue/4*$.liftAction.getClip().duration,$.lift.mixer.update(0),M?.kind==="lift"&&M.id===K)){let Re=Dn.find(He=>He.id===K);W.position.set(Re.liftX,2.4+gt.room+$.liftValue,Re.liftZ),$.liftValue===$.liftTarget&&(M=null,e.onRide?.(null))}if(we.update(ae,q),De.update(ae,q),l.forEach((K,$)=>{if(K.node.visible=!!K.body&&$<(b==="low"?1:2),K.body&&(K.node.visible&&!K.body.enabled&&!le.clear({...K.body,enabled:!0},K.body,K.body)&&(K.node.visible=!1),K.body.enabled=K.node.visible),!K.node.visible)return;let Re={u:K.u,dwell:K.dwell,stop:K.stop},He=!1;if(K.dwell>0)K.dwell-=ae,K.speed=0;else{let ot=kt=>(kt.u-K.u+1)%1*et,Ze=ht.reduce((kt,nn)=>ot(nn)>1e-5&&ot(nn)<ot(kt)?nn:kt,ht.find(kt=>ot(kt)>1e-5)||ht[0]),Jt=Math.min(11,Math.sqrt(ot(Ze)*8));for(let kt=1;kt<=3;kt++){let nn=(K.u+(1.2+K.speed*.7)*kt/3/et)%1;it.getPointAt(nn,k),it.getTangentAt(nn,E);let Zt={x:k.x,y:gt.road+.03,z:k.z,heading:Math.atan2(E.x,E.z)};if(!le.clear(K.body,Zt,Zt)){Jt=0;break}}K.speed+=Ft.clamp(Jt-K.speed,-ae*8,ae*3.5);let Xt=ae*K.speed/et,Mt=K.u;K.u=(K.u+Xt)%1;let Mn=ht.findIndex(kt=>(kt.u-Mt+1)%1>0&&(kt.u-Mt+1)%1<=Xt+1e-5);Mn>=0&&(K.stop=Mn,K.u=ht[Mn].u,K.dwell=6,K.speed=0,He=!0)}if(it.getPointAt(K.u,K.node.position),K.node.position.y=gt.road+.03,it.getTangentAt(K.u,E),K.node.rotation.y=Math.atan2(E.x,E.z),K.body&&(K.body.enabled=K.node.visible,le.propose(K.body,{...K.node.position,heading:K.node.rotation.y},(ot,Ze)=>{ot||(Object.assign(K,Re),K.speed=0),K.node.position.set(Ze.x,Ze.y,Ze.z),K.node.rotation.y=Ze.heading,ot&&He&&J("tram",K.node.position)})),K.door.time=(K.dwell>1?1:0)*K.door.getClip().duration,K.mixer.update(0),M?.kind==="waiting"&&ht[K.stop].id===M.station&&K.dwell>1&&(M={kind:"tram",index:$,station:M.station,offset:0},e.onRide?.("tram")),M?.kind==="tram"&&M.index===$){if(K.dwell>1&&(M.station=ht[K.stop].id,M.exitRequested)){Pe();return}W.position.copy(K.node.position).addScaledVector(E,M.offset),W.position.y+=.9+1.4,W.lookAt(K.node.position.x+E.x*16,W.position.y,K.node.position.z+E.z*16)}}),M?.kind==="drone"){M.elapsed+=Math.min(U,.1);let K=M.elapsed/40*Math.PI*2;W.position.set(Math.cos(K)*100,45+Math.sin(K*2)*12,Math.sin(K)*95-10),W.lookAt(0,22,-12),M.elapsed>=40&&Pe()}if(F=Y==="street"&&_e()[0]||null,Y==="street"&&!M){let K=W.position.distanceTo(Z);K<3&&(I+=K),I>1.8&&(J(H?"step_inside":"step",W.position),I=0)}Z.copy(W.position),e.onEnvironment?.(!!H);for(let[K,$]of f.entries()){if($.elapsed+=ae,$.node.visible=q&&$.elapsed<8,!$.node.visible){$.body&&le.remove($.body.id),$.body=null;continue}let Re=$.direction>0?$.elapsed/8:1-$.elapsed/8,He={x:40+Re*3,y:gt.room,z:70,heading:0};if($.body||($.body=le.register("freight-"+K,ts($.bounds),He,0)),!$.body){$.elapsed=9,$.node.visible=!1;continue}le.propose($.body,He,(ot,Ze)=>{ot||($.elapsed-=ae),$.node.position.set(Ze.x,Ze.y,Ze.z)})}e.traffic||(le.solve(ae),we.settle(ae)),e.onInteraction?.(F,N.size,H)}async function Xe(U){if(b=U,!C)return;let q=U==="low"?2:U==="medium"?1:0;we.setTier(U),De.setTier(U);try{let Y=new Map(await Promise.all([...x].filter(ae=>!be.has(ae)).map(async ae=>{let K=await ut(ae,q),$=new Map;return K.scene.traverse(Re=>{Re.isMesh&&$.set(Re.name,Re)}),[ae,$]})));if(A||b!==U)return;for(let ae of v){let K=Y.get(ae.userData.worldAsset)?.get(ae.userData.worldPart);K&&(ae.geometry=K.geometry,ae.material=K.material)}e.onReady?.()}catch(Y){A||e.onError?.(Y)}}return{update:qe,interact:fe,endRide:Pe,setTier:Xe,society:we,settle(U){!A&&C&&we.settle(U)},suspend(){we.suspend(),De.clear(),w=!1;for(let U of f)U.elapsed=9,U.node.visible=!1,U.body&&le.remove(U.body.id),U.body=null},syncRide(){if(M?.kind==="tram"){let U=l[M.index];it.getTangentAt(U.u,E),W.position.copy(U.node.position).addScaledVector(E,M.offset),W.position.y+=2.3,W.lookAt(U.node.position.x+E.x*16,W.position.y,U.node.position.z+E.z*16)}},socialAction(U,q){let Y=ri.find(ae=>ae.id===q);return we.action(e.reduced()&&U==="guide"?"cancel":U,Y?{x:Y.platformX,z:Y.platformZ}:null)},setMemory(U){g=(Array.isArray(U)?U:[]).filter(q=>typeof q=="string").slice(0,8),ke()},setWorld(U,q=!1){q?(we.setReplay(!0),De.clear()):we.setReplay(!1);let Y=U?.entities?.filter(K=>K.kind==="mission")||[],ae=new Set;for(let K of Y){ae.add(K.id);let $=y.get(K.id);if(!q&&e.active()&&w&&$!=null&&$!==K.state&&["running","completed","failed","cancelled"].includes(K.state)&&Date.now()-K.at<3e4){let Re=f.find(He=>He.elapsed>=8);Re&&(Re.elapsed=0,Re.direction=K.state==="running"?1:-1,_++)}y.set(K.id,K.state)}for(let K of y.keys())ae.has(K)||y.delete(K);if(w=!q,q)for(let K of f)K.elapsed=9,K.node.visible=!1},walkRide(U,q,Y=0){if(!M)return!1;if(M.kind==="tram"&&(M.offset=Ft.clamp(M.offset+U*q*3,-2.5,2.5)),M.kind==="scope"){Ne.setFromQuaternion(W.quaternion,"YXZ");let ae=q*.9*W.fov/M.back.fov;Ne.x+=U*ae,Ne.y-=Y*ae,W.quaternion.setFromEuler(Ne)}return!0},hovering:()=>M?.kind==="hover",fly(U,q){if(M?.kind!=="hover")return!1;W.position.distanceToSquared(ne)>.01&&de.place(W.position),W.getWorldDirection(k);let Y=de.step(U,q,Math.atan2(-k.x,-k.z));return ne.set(Y.x,Y.y,Y.z),W.position.copy(ne),Me(),!0},zoom(U){return M?.kind!=="scope"?!1:(M.fov=Ft.clamp(M.fov*Math.exp(U*.0012),5,26),!0)},scoping:()=>M?.kind==="scope",move(U,q,Y){if(M)return!1;if(Y.y>40)return Np(U,q,!Q.motion&&Q.value===qt.lift.travel);let ae=ys(Y.x,Y.z);return ae&&Y.y>5?Math.abs(q-ae.z-4.8)<.85&&(Math.abs(U-ae.x)<1.1||Math.abs(U-ae.x+4)<1.75)?!1:Xu(ae,U,q,c.get(ae.id)?.liftValue===4):ae&&Y.y<5&&([-5.6,1.9].some(K=>Math.abs(U-ae.x-K)<.25&&Math.abs(q-ae.z-.15)<.25)||Op(ae,U,q)&&!(U<ae.liftX+1.25&&Math.abs(q-ae.liftZ)<1.25&&c.get(ae.id)?.liftValue===0)||Math.hypot(U-(ae.x-1),q-ae.z)<1.55||[-3,0].some(K=>Math.abs(U-ae.x-K)<1.1&&Math.abs(q-ae.z-3)<.8))?!1:Qu(U,q,Y,K=>h.get(K)?.value>.85&&!!c.get(K)?.ready,e.districts)},floor(U,q){if(W.position.y>40)return qt.floor;let Y=ys(U,q);return Y&&W.position.y>5&&Xu(Y,U,q,c.get(Y.id)?.liftValue===4)?gt.gallery:te(U,q)},visit(U){Pe(),M=null;let q=Ms.find(ae=>ae.id===U);if(q){let ae=C?.assets.find(K=>K.id===U)?.navigation.interaction[0]?.position||[0,1.5,4];W.position.set(q.x,2.4+Ln(q.x,q.z),q.z+ae[2]+1),W.lookAt(q.x,2,q.z);return}if(U==="drone"){W.position.set(zi.x,2.7,zi.z+3),W.lookAt(zi.x,2,zi.z);return}if(U==="skydeck"){let ae=57*Math.PI/180,K=qt.x+Math.sin(ae)*7.2,$=qt.z-Math.cos(ae)*7.2;W.position.set(K,qt.floor+2.4,$),W.lookAt(K,qt.floor+.4,$-10);return}let Y=ri.find(ae=>ae.id===U);if(Y){let ae=N.has(U)?0:5,K=Y.platformX+(Y.angle===0?ae:0),$=Y.platformZ+(Y.angle===0?0:ae);W.position.set(K,2.4+Ln(K,$),$),W.lookAt(Y.platformX,2,Y.platformZ===$?Y.platformZ+1:Y.platformZ)}},destination(U){let q=Dn.find(Y=>Y.id===U);if(q){Pe(),M=null;let Y=q.doorZ+q.front*4;W.position.set(q.x,2.4+Ln(q.x,Y),Y),W.lookAt(q.x,2.4+Ln(q.x,Y),q.z)}},isRiding:()=>!!M,interaction:()=>F,rideBody:()=>M?.kind==="tram"?"tram-"+M.index:M?.kind==="scope"?M.scope.model.owner:M?.kind==="sky"&&Q.lift?.owner||null,stats:()=>({society:we.stats(),machinery:De.stats(),details:[...be].map(([U,q])=>({id:U,level:q.level})),loaded:[...x],bytes:S,freightEvents:_,residents:o.filter(U=>U.node.visible).length,trams:l.filter(U=>U.node.visible).length,rooms:[...c].filter(([,U])=>U.ready).map(([U])=>U),inside:H,ride:M?.kind||null,station:M?.station||null,discovered:[...N],loading:D,interactions:F?{kind:F.kind,id:F.id}:null,landmarks:u.map(U=>U.id),sky:{ready:Q.ready,value:+Q.value.toFixed(2),moving:!!Q.motion},telescopes:Fe.length,hover:(U=>({pads:pe.length,docked:pe.filter(q=>q.docked).length,flying:!!U,altitude:U?+U.altitude.toFixed(1):0,speed:U?+(U.speed*3.6).toFixed(1):0,landing:!!U?.landing}))(M?.kind==="hover"?de.state():null),scope:M?.kind==="scope"?{id:M.id,target:M.target,fov:+W.fov.toFixed(1)}:null}),dispose(){A||(A=!0,T.abort(),we.dispose(),De.dispose(),e.traffic||le.dispose(),g=[],y.clear(),t.removeFromParent(),a.forEach(U=>{U.stopAllAction(),U.uncacheRoot(U.getRoot())}),t.traverse(U=>{U.isInstancedMesh&&U.dispose()}),i.forEach(U=>U.dispose()),r.forEach(U=>U.dispose()),m.forEach(U=>U.dispose()),n.clear())}}}var Qy=s=>.5+(s-6)/12*3.62,Pm=new B;function Im(s,{sun:e,rim:t,hemisphere:n,atmosphere:i,surfaces:r,onThunder:a}){let o="local",l="clear",c="high",h=!1,u=-100,d=!1,p=n.intensity,x=14,v=-1,m=0,f={value:0},y=new _t,T=[],g=317,_=()=>(g=Math.imul(g,1664525)+1013904223>>>0)/4294967296;for(let I=0;I<1200;I++)T.push((_()-.5)*170,_()*60,(_()-.5)*174-7);y.setAttribute("position",new mt(T,3));let w=new yt({transparent:!0,depthWrite:!1,uniforms:{time:f,flash:{value:0}},vertexShader:"uniform float time;varying float vFade;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;gl_PointSize=clamp(260.0/max(1.0,-mv.z),2.0,26.0);vFade=smoothstep(0.0,4.0,p.y)*smoothstep(460.0,60.0,-mv.z);}",fragmentShader:"uniform float flash;varying float vFade;void main(){vec2 c=gl_PointCoord-.5;float a=(1.0-smoothstep(0.0,0.07,abs(c.x)))*(1.0-smoothstep(0.2,0.5,abs(c.y)))*.42*vFade;if(a<.004)discard;gl_FragColor=vec4(mix(vec3(.6,.78,.86),vec3(.95,.97,1.0),flash),a*(1.0+flash));}"}),A=new yn(y,w);A.frustumCulled=!1,s.add(A);let b=new Le(9417679),C=new Le(528926),S=new Le(2762296);function P(){let I=Bc(o),N=I.amount,H=Qy(I.hour);e.intensity=.8+N*2.6,e.color.set(I.evening>.1?16757370:13164287),e.position.set(Math.cos(I.hour/24*Math.PI*2)*120,40+N*115,85),Pm.set(Math.cos(H)*120,40-I.evening*18+N*115,Math.sin(H)*120),t.intensity=.7+I.evening*1.6,p=n.intensity=.45+N*.45,s.fog.color.copy(C).lerp(b,N).lerp(S,I.evening*(1-N)*.5),s.fog.density=l==="fog"?.018:l==="rain"?.008:.003-N*.0016,i.setLighting?.(N,I.evening,Pm),i.setWeather?.(l),r?.setLighting(N,I.evening,l==="clear"?0:1,s.fog.color),r?.setWeather(l),A.visible=l==="rain"&&!h,y.setDrawRange(0,c==="low"?200:1200)}function M(I,N){if(l!=="rain"||h||!N){v>=0&&(v=-1,F(0));return}if(x-=I,v<0&&x<=0&&(v=0,m++,x=9+_()*16,a?.(.6+_()*3.2)),v<0)return;v+=I;let H=v<.09?1:v<.16?.25:v<.24?.75:Math.max(0,1-(v-.24)/.35)*.5;F(H),v>.6&&(v=-1,F(0))}function F(I){n.intensity=p+I*2.4,w.uniforms.flash.value=I,i.setFlash?.(I)}return P(),{set(I={}){["local","day","evening","night"].includes(I.time)&&(o=I.time),["clear","rain","fog"].includes(I.weather)&&(l=I.weather),P()},setTier(I){c=I,P()},setIndoor(I){h!==I&&(h=I,P())},update(I,N,H){if(!d)return A.visible=l==="rain"&&!h&&H,H&&(f.value+=Math.min(I,.1)),M(Math.min(I,.1),H),N-u>30?(u=N,P(),!0):!1},mood(){let I=Bc(o);return{day:I.amount,evening:I.evening,weather:l,indoor:h}},stats:()=>({time:o,weather:l,indoor:h,daylight:Bc(o).amount,strikes:m}),dispose(){d=!0,A.removeFromParent(),y.dispose(),w.dispose()}}}var ws={cx:30,cz:69.5,rx:4.5,rz:2.5,seconds:16,gaitHz:1.7},eM=["body","fl_hip","fr_hip","hl_hip","hr_hip","fl_uleg","fr_uleg","hl_uleg","hr_uleg","fl_lleg","fr_lleg","hl_lleg","hr_lleg"],tM={fl:0,hr:0,fr:Math.PI,hl:Math.PI};function nM(s){let e=s/ws.seconds*Math.PI*2,t=ws.cx+ws.rx*Math.cos(e),n=ws.cz+ws.rz*Math.sin(e),i=-ws.rx*Math.sin(e),r=ws.rz*Math.cos(e);return{x:t,z:n,heading:Math.atan2(-r,i),phase:s*ws.gaitHz*Math.PI*2}}function iM(s){let e={};for(let[t,n]of Object.entries(tM))e[t]={hip:.26*Math.sin(s+n),uleg:.13*Math.sin(s+n+.9)};return e}function Dm({scene:s,resourceURL:e,signal:t,ground:n=Ln,shadows:i=null,onError:r}){let a=new vt;a.visible=!1,s.add(a);let o=Object.create(null),l=new cn,c={y:NaN,v:0},h={y:0,v:0},u=i?.claim()||null,d=null,p=0,x=0,v=!1,m=!1,f=new yi;f.setMeshoptDecoder(Yr);let y=(async()=>{try{await Yr.ready;let g=await fetch(e("/3d/system-world/spot.glb"),{signal:t});if(!g.ok)throw Error("Companion asset unavailable: "+g.status);let _=await g.arrayBuffer();if(m)return!1;let w=await f.parseAsync(_,"");if(m)return!1;d=w.scene,d.traverse(A=>{A.isMesh&&(A.castShadow=!0,A.receiveShadow=!0)});for(let A of eM)o[A]=d.getObjectByName(A)||null;if(!o.body)throw Error("Companion joint hierarchy missing");return l.setFromObject(d),p=-l.min.y,a.add(d),a.visible=!0,!0}catch(g){return v=!0,r?.(g),!1}})();function T(g,_){if(!d||m)return;let w=_?Math.min(.1,Math.max(0,g)):0;x+=w;let A=nM(x),b=Math.cos(A.heading),C=-Math.sin(A.heading),S=.55,P=n(A.x,A.z),M=Ss(c,P,w),F=n(A.x+b*S,A.z+C*S)-n(A.x-b*S,A.z-C*S),I=Ss(h,Math.atan2(F,2*S),w);a.position.set(A.x,M+p+Math.abs(Math.sin(A.phase))*.012,A.z),a.rotation.set(0,A.heading,I,"YZX"),u?.set(A.x,P,A.z,.6,.45);for(let[N,H]of Object.entries(iM(A.phase)))o[N+"_hip"]&&(o[N+"_hip"].rotation.y=H.hip),o[N+"_uleg"]&&(o[N+"_uleg"].rotation.y=H.uleg)}return{ready:y,update:T,stats:()=>({loaded:!!d,failed:v,time:+x.toFixed(2),joints:Object.keys(o).filter(g=>o[g]).length,position:[+a.position.x.toFixed(2),+a.position.z.toFixed(2)]}),dispose(){m||(m=!0,u?.hide(),d&&d.traverse(g=>{if(g.isMesh){g.geometry.dispose();for(let _ of Array.isArray(g.material)?g.material:[g.material])_.dispose()}}),a.removeFromParent(),a.clear())}}}function Lm(s,{capacity:e=40}={}){let t=new fn(1,1);t.rotateX(-Math.PI/2);let n=new fi(new Float32Array(e),1);t.setAttribute("swShadow",n);let i=new yt({transparent:!0,depthWrite:!1,polygonOffset:!0,polygonOffsetFactor:-2,polygonOffsetUnits:-2,vertexShader:`attribute float swShadow;varying vec2 vUv;varying float vA;void main(){vUv=uv;vA=swShadow;vec4 p=vec4(position,1.0);
#ifdef USE_INSTANCING
p=instanceMatrix*p;
#endif
gl_Position=projectionMatrix*modelViewMatrix*p;}`,fragmentShader:"varying vec2 vUv;varying float vA;void main(){float r=length(vUv-.5)*2.;gl_FragColor=vec4(0.,0.,0.,vA*(1.-smoothstep(.1,1.,r)));}"}),r=new Sn(t,i,e),a=new ft;r.name="contact-shadows",r.frustumCulled=!1,r.renderOrder=1,r.castShadow=!1,r.receiveShadow=!1;for(let l=0;l<e;l++)r.setMatrixAt(l,a.makeScale(0,0,0));s.add(r);let o=0;return{claim(){if(o>=e)return null;let l=o++;return{set(c,h,u,d,p){r.setMatrixAt(l,a.makeScale(d*2,1,d*2).setPosition(c,h+.015,u)),n.setX(l,p),r.instanceMatrix.needsUpdate=!0,n.needsUpdate=!0},hide(){r.setMatrixAt(l,a.makeScale(0,0,0)),n.setX(l,0),r.instanceMatrix.needsUpdate=!0,n.needsUpdate=!0}}},stats:()=>({used:o,capacity:e}),dispose(){r.removeFromParent(),r.dispose(),t.dispose(),i.dispose()}}}function Fm(s,e,t,n){for(let i of t){let r=e.get(i.asset).lods[0].bounds;(Zu[i.id]||(i.id==="graph"?[[-10.2,-10.2,10.2,10.2]]:[[-3,-3,3,3]])).forEach(([o,l,c,h],u)=>s.solid("district:"+i.id+":"+u,{x:i.x,z:i.z,min:[o,0,l],max:[c,8,h]})),$u[i.id]?$u[i.id].forEach(([o,l,c],h)=>s.solid("district:"+i.id+":upper:"+h,{x:i.x,z:i.z,min:[-o,l,-o],max:[o,c,o]})):s.solid("district:"+i.id+":upper",{x:i.x,z:i.z,min:[r.min[0],8,r.min[2]],max:r.max})}n.forEach((i,r)=>{if(i.asset==="street-tile"||i.asset==="street-crossing")return;let a=e.get(i.asset)?.lods[0].bounds;if(!a)return;let o=a.min.map((c,h)=>c*i.scale[h]),l=a.max.map((c,h)=>c*i.scale[h]);o[1]+=i.y,l[1]+=i.y,i.asset==="street-lamp"&&(s.solid("city:"+r+":pole",{x:i.x,z:i.z,min:[-.2,i.y,-.2],max:[.2,l[1],.2]}),o[1]=Math.max(o[1],l[1]-1)),s.solid("city:"+r,{x:i.x,z:i.z,heading:i.angle,min:o,max:l})})}var vo={"city.road":{top:"asphalt",side:"concrete",albedo:.55,rough:.7,bump:.012,wet:1,puddle:[.447,.442]},"city.stone":{top:"pavers",side:"concrete",albedo:.5,rough:.6,bump:.014,wet:1,puddle:[.523,.516]},"city.graphite":{top:"gravel",side:"panels",albedo:.42,rough:.5,bump:.016,wet:.8,metalTop:.3},"city.titanium":{top:"panels",side:"panels",albedo:.2,rough:.45,bump:.006,wet:.6},"city.bronze":{top:"panels",side:"panels",albedo:.18,rough:.4,bump:.006,wet:.6},"city.robot-rubber":{top:"panels",side:"panels",albedo:.3,rough:.75,bump:.008,wet:.25},"city.ceramic":{top:"concrete",side:"concrete",albedo:.3,rough:.4,bump:.008,wet:.8,puddle:[.464,.46]},"city.leaf":{top:"foliage",side:"foliage",albedo:.7,rough:.3,bump:.03,wet:.5},"city.kerb":{top:"concrete",side:"concrete",albedo:.45,rough:.55,bump:.01,wet:1},"city.setts":{top:"pavers",side:"concrete",albedo:.5,rough:.6,bump:.014,wet:1,topScale:2,puddle:[.523,.516]},"city.iron":{top:"panels",side:"panels",albedo:.25,rough:.45,bump:.008,wet:.9},"city.marking":{top:"asphalt",side:"concrete",albedo:.25,rough:.5,bump:.006,wet:1,puddle:[.447,.442]},"city.tactile":{top:"concrete",side:"concrete",albedo:.4,rough:.6,bump:.02,wet:1},ground:{top:"pavers",side:"concrete",albedo:.55,rough:.6,bump:.014,wet:1,topScale:.5,puddle:[.523,.516],waterline:!0}},Nm=new Set(["city.ivory","city.warm"]),sM={low:0,medium:1,high:2,ultra:2},$s=s=>Number(s).toFixed(4),rM=`varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
attribute float swSeed;varying float vSwSeed;
#endif`,aM=`
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
#endif`,oM=`uniform float swWet;uniform float swRain;uniform float swTime;uniform vec4 swWindow;uniform vec2 swFade;uniform vec4 swShelter[3];uniform vec3 swSky;
varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
varying float vSwSeed;
#endif
#ifdef SW_SURFACE
uniform sampler2D swTop;uniform sampler2D swSide;uniform vec2 swScale;
float swHash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
vec3 swPerturb(vec3 pos,vec3 n,vec2 dh,float face){vec3 sx=dFdx(pos),sy=dFdy(pos),r1=cross(sy,n),r2=cross(n,sx);float det=dot(sx,r1)*face;return normalize(abs(det)*n-sign(det)*(dh.x*r1+dh.y*r2));}
#endif`,lM=`
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
#endif`,cM=`
#ifdef SW_SURFACE
diffuseColor.rgb*=swAlbedoF*mix(1.0,0.56,swWetness)*mix(1.0,0.7,swPuddle);
#endif`,hM=`
#ifdef SW_SURFACE
roughnessFactor=clamp(roughnessFactor*swRoughF,0.04,1.0);roughnessFactor=mix(roughnessFactor,roughnessFactor*0.6,swWetness);roughnessFactor=mix(roughnessFactor,0.04,swPuddle);
#endif`,uM=`
#ifdef SW_SURFACE
metalnessFactor*=mix(1.0,SW_METAL_TOP,swW.y);
#endif`,dM=`
#if SW_LEVEL>1
normal=swPerturb(-vViewPosition,normal,vec2(dFdx(swH),dFdy(swH))*SW_BUMP*swDetail,faceDirection);
#endif`,fM=`
#ifdef SW_SURFACE
float swFresnel=pow(1.0-clamp(dot(normal,normalize(vViewPosition)),0.0,1.0),5.0)*0.95+0.05;
totalEmissiveRadiance+=swSky*swFresnel*(swPuddle*0.6+swWetness*0.1);
#endif
#ifdef SW_WINDOWS
float swPane=(1.0-smoothstep(0.35,0.65,abs(swN.y)))*(1.0-step(1.5,vSwSeed));
totalEmissiveRadiance*=mix(swWindow.w,mix(swWindow.z,swWindow.y,step(vSwSeed,swWindow.x)),swPane);
#endif`;function pM(s){let e=s?.getAttribute("position");if(!e||s.getAttribute("swSeed"))return;let t=e.count,n=new Int32Array(t),i=s.getIndex();for(let u=0;u<t;u++)n[u]=u;let r=u=>{for(;n[u]!==u;)n[u]=n[n[u]],u=n[u];return u},a=u=>i?i.getX(u):u,o=i?i.count:t;for(let u=0;u+2<o;u+=3){let d=r(a(u));n[r(a(u+1))]=d,n[r(a(u+2))]=d}let l=new Map,c=new Map;for(let u=0;u<t;u++){let d=r(u),p=c.get(d)||[0,0,0];l.set(d,(l.get(d)||0)+1),p[0]+=e.getX(u),p[1]+=e.getY(u),p[2]+=e.getZ(u),c.set(d,p)}let h=new Float32Array(t);for(let u=0;u<t;u++){let d=r(u),p=l.get(d),x=c.get(d);if(p>6){h[u]=2;continue}let v=Math.sin(x[0]/p*12.9898+x[1]/p*78.233+x[2]/p*37.719)*43758.5453;h[u]=v-Math.floor(v)}s.setAttribute("swSeed",new Rt(h,1))}function Um(s,e={}){let t=new AbortController,n=new Map,i=[],r=new hs(new Uint8Array([128,128,128,255]),1,1);r.colorSpace=ii,r.needsUpdate=!0;let a={},o={},l={};for(let M of Object.values(vo))for(let F of[M.top,M.side])a[F]||={value:r};for(let M of Object.keys(vo))l[M]={value:new Be(1/4,1/4)};let c=(e.shelters||[]).slice(0,3).map(M=>new Ot(...M));for(;c.length<3;)c.push(new Ot(1e6,1e6,-1e6,-1e6));let h={swWet:{value:0},swRain:{value:0},swTime:{value:0},swShelter:{value:c},swSky:{value:new Le(528926)},swWindow:{value:new Ot(.8,1,.06,1)},swFade:{value:new Be(35,140)}},u=2,d=null,p=!1,x=0,v=0,m=0,f=0,y=!1,T=s.capabilities.getMaxAnisotropy(),g=()=>u>1?Math.min(8,T):Math.min(4,T);function _(M,F){let{kind:I,windows:N,space:H}=n.get(F),D=vo[I];Object.assign(M.uniforms,h);let V=["#define SW_LEVEL "+(D?u:0)];H==="world"&&V.push("#define SW_WORLD"),N&&V.push("#define SW_WINDOWS"),D&&(M.uniforms.swTop=a[D.top],M.uniforms.swSide=a[D.side],M.uniforms.swScale=l[I],V.push("#define SW_SURFACE","#define SW_ALBEDO "+$s(D.albedo),"#define SW_ROUGH "+$s(D.rough),"#define SW_BUMP "+$s(D.bump),"#define SW_WETK "+$s(H==="world"?D.wet:D.wet*.5),"#define SW_METAL_TOP "+$s(D.metalTop??1),"#define SW_PUDDLE_DRY "+$s(D.puddle?.[0]??-1),"#define SW_PUDDLE_FULL "+$s(D.puddle?.[1]??-2)),D.waterline&&V.push("#define SW_WATERLINE"));let k=V.join(`
`)+`
`;M.vertexShader=k+M.vertexShader.replace("#include <common>",`#include <common>
`+rM).replace("#include <begin_vertex>","#include <begin_vertex>"+aM),M.fragmentShader=k+M.fragmentShader.replace("#include <common>",`#include <common>
`+oM).replace("#include <map_fragment>","#include <map_fragment>"+lM).replace("#include <color_fragment>","#include <color_fragment>"+cM).replace("#include <roughnessmap_fragment>","#include <roughnessmap_fragment>"+hM).replace("#include <metalnessmap_fragment>","#include <metalnessmap_fragment>"+uM).replace("#include <normal_fragment_maps>","#include <normal_fragment_maps>"+dM).replace("#include <emissivemap_fragment>","#include <emissivemap_fragment>"+fM)}function w(M,F="world",I=M?.name){if(!M?.isMeshStandardMaterial||n.has(M))return;let N=vo[I]?I:"",H=Nm.has(M.name);!N&&!H||(n.set(M,{kind:N,windows:H,space:F}),M.addEventListener("dispose",()=>n.delete(M)),M.onBeforeCompile=D=>_(D,M),M.customProgramCacheKey=()=>["sw",N,H,F,N?u:0].join("|"),M.needsUpdate=!0)}function A(M,F=!1){if(M?.isMesh)for(let I of Array.isArray(M.material)?M.material:[M.material])w(I,F?"object":"world"),Nm.has(I?.name)&&pM(M.geometry)}let b=M=>e.url?.(M),C=null;async function S(M){let F=await fetch(b(M.file),{signal:t.signal});if(!F.ok)throw Error("Surface texture unavailable");let I=await F.blob();if(p)return;let N=await createImageBitmap(I,{premultiplyAlpha:"none",colorSpaceConversion:"none"});if(p){N.close?.();return}let H=new ln(N);H.wrapS=H.wrapT=Ti,H.colorSpace=ii,H.flipY=!1,H.anisotropy=g(),H.needsUpdate=!0,i.push(H),a[M.id].value=H,m++,x+=I.size,e.onProgress?.(x,v)}function P(){return d||p||!e.url||(d=(async()=>{if(!C){let I=await fetch(b("manifest.json"),{signal:t.signal});if(!I.ok)throw Error("Surface manifest unavailable");C=await I.json();let N=(C.textures||[]).filter(H=>a[H.id]&&/^[a-z-]+\.webp$/.test(H.file)&&H.tile_metres>0);C.entries=N,v=N.reduce((H,D)=>H+(D.bytes||0),0);for(let H of N)o[H.id]=H.tile_metres;for(let[H,D]of Object.entries(vo))l[H].value.set(1/((o[D.top]||4)/(D.topScale||1)),1/(o[D.side]||4))}let M=C.entries.filter(I=>a[I.id].value===r),F=await Promise.allSettled(M.map(S));f+=F.filter(I=>I.status==="rejected").length})().catch(()=>{f++}).finally(()=>{d=null})),d}return{apply:w,prepare:A,setTier(M){let F=sM[M]??2;if(F!==u){u=F;for(let[I,N]of n)N.kind&&(I.needsUpdate=!0)}for(let I of i)I.anisotropy!==g()&&(I.anisotropy=g(),I.needsUpdate=!0);u>0&&P()},setLighting(M,F,I=0,N=null){N&&h.swSky.value.copy(N).multiplyScalar(1.25);let H=1-M,D=Ft.smoothstep(H,.1,.9);h.swWindow.value.set(Math.min(.9,.12+.72*D+.12*I),.22+H*.98+Math.min(1,F)*.1,.05,.55+.45*H)},setWeather(M){y=M==="rain",h.swRain.value=y?1:0},update(M,F){let I=y?1:0,N=h.swWet.value;if(!F){h.swWet.value=I;return}let H=Math.min(.1,Math.max(0,M));h.swTime.value+=H,h.swWet.value=N+(I-N)*Math.min(1,H/(I>N?14:70))},progress:()=>({bytes:x,expected:v}),stats:()=>({detail:u,textures:m,failures:f,bytes:x,materials:n.size,wet:+h.swWet.value.toFixed(2),windows:+h.swWindow.value.x.toFixed(2)}),dispose(){p=!0,t.abort();for(let M of i)M.dispose(),M.source.data?.close?.();i.length=0,r.dispose()}}}function mM(s){let e=Math.max(64,Math.round(s.sampleRate*.03)||1440),t=[],n=0;for(let a=0;a<s.numberOfChannels;a++){let o=s.getChannelData(a);for(let l=0;l<o.length;l+=e){let c=0,h=Math.min(l+e,o.length);for(let u=l;u<h;u++){let d=o[u];if(!Number.isFinite(d))throw Error("Invalid voice");n=Math.max(n,Math.abs(d)),c+=d*d}t.push(Math.sqrt(c/(h-l)))}}if(n<1e-4)throw Error("Silent voice");let i=t.filter(a=>a>Math.max(.003,n*.003)).sort((a,o)=>a-o),r=i[Math.floor((i.length-1)*.7)]||n*.5;return Math.min(8,Math.max(1/n,.26/Math.max(r,.001)))}function Om(s,e){let t=!1,n=!1,i=0,r=null,a=null,o=[],l=null,c=null,h=null,u=250,d=0,p=0,x=0,v="idle",m=null,f=()=>.04+.46/(1+(u/110)**2);function y(){let b=s.currentTime;l?.gain.setTargetAtTime(f(),b,.18),c?.frequency.setTargetAtTime(1800+4400/(1+u/95),b,.18),h?.pan.setTargetAtTime(d,b,.18)}function T(){if(a){a.onended=null;try{a.stop()}catch{}a=null}o.forEach(b=>b.disconnect()),o=[],l=c=h=null}function g(){p++,clearTimeout(i),i=0,r?.abort(),r=null,T(),v="idle"}function _(b=2e4+Math.random()*2e4){clearTimeout(i),t&&!n&&(i=setTimeout(()=>{i=0,A()},b))}function w(){if(m)return m;m=s.createBuffer(2,Math.ceil(s.sampleRate*.85),s.sampleRate);let b=42;for(let C=0;C<2;C++){let S=m.getChannelData(C),P=Math.round(s.sampleRate*.015),M=0;for(let I=0;I<S.length;I++){b=Math.imul(b,1664525)+1013904223>>>0;let N=(I-P)/(S.length-P);S[I]=I<P?0:(b/4294967296*2-1)*Math.pow(1-N,2.25),M+=S[I]*S[I]}let F=.65/Math.sqrt(M||1);for(let I=0;I<S.length;I++)S[I]*=F}return m}async function A(){if(!t||n||r||a||s.state!=="running"){_();return}let b=p,C=new AbortController;r=C,v="loading";let S=setTimeout(()=>C.abort(),32e3);try{let P=await fetch("/api/desktop/system-world/voice",{method:"POST",credentials:"same-origin",cache:"no-store",signal:C.signal});if(P.status===204){v="idle";return}if(!P.ok||!P.headers.get("Content-Type")?.startsWith("audio/"))throw Error("Voice unavailable");if(Number(P.headers.get("Content-Length"))>8*1024*1024)throw Error("Voice too large");let M=await P.arrayBuffer();if(M.byteLength>8*1024*1024)throw Error("Voice too large");if(b!==p||!t||n)return;let F=await s.decodeAudioData(M);if(b!==p||!t||n)return;if(!Number.isFinite(F.duration)||F.duration<=0||F.duration>25||F.numberOfChannels>2)throw Error("Invalid voice");let I=mM(F),N=ze=>(o.push(ze),ze);a=N(s.createBufferSource()),a.buffer=F;let H=N(s.createGain()),D=N(s.createBiquadFilter());H.gain.value=I,D.type="highpass",D.frequency.value=90,c=N(s.createBiquadFilter()),c.type="lowpass",c.Q.value=.55;let V=N(s.createGain()),k=N(s.createGain()),E=N(s.createGain()),Z=N(s.createGain());V.gain.value=.76,k.gain.value=.22,E.gain.value=.28;let W=N(s.createDelay(.2)),Q=N(s.createConvolver());W.delayTime.value=.085,Q.normalize=!1,Q.buffer=w();let Fe=N(s.createWaveShaper()),Ne=new Float32Array(1024);for(let ze=0;ze<Ne.length;ze++)Ne[ze]=Math.max(-.75,Math.min(.75,ze*2/(Ne.length-1)-1));Fe.curve=Ne,l=N(s.createGain()),l.gain.value=f(),h=N(s.createStereoPanner()),h.pan.value=d,a.connect(H).connect(D).connect(c),c.connect(V).connect(Z),c.connect(W).connect(k).connect(Z),c.connect(Q).connect(E).connect(Z),Z.connect(Fe).connect(l).connect(h).connect(e),y(),v="speaking",x++,a.onended=()=>{b!==p||!t||n||(v="tail",i=setTimeout(()=>{T(),v="idle",_()},900))},a.start()}catch{b===p&&!n&&(v=C.signal.aborted?"idle":"unavailable")}finally{clearTimeout(S),r===C&&(r=null),b===p&&!n&&!a&&_(v==="unavailable"?6e4:void 0)}}return{setActive(b){t===b||n||(t=b,t?_(3e3+Math.random()*2e3):g())},setListener(b,C,S,P,M){if(!Number.isFinite(b)||!Number.isFinite(C)||!Number.isFinite(S)||!Number.isFinite(P)||!Number.isFinite(M))return;let F=-b,I=-12-S,N=Math.max(0,Math.hypot(F,I)-13);u=Math.hypot(N,Math.max(0,C-87,-C));let H=Math.hypot(F,I),D=Math.hypot(P,M);d=H&&D?Math.max(-.8,Math.min(.8,(I*P-F*M)/H/D))*.8:0,y()},stats(){return{state:v,phrases:x,distance:u,gain:f(),pan:d,pending:!!r,scheduled:!!i}},dispose(){n||(n=!0,t=!1,g(),m=null)}}}function zm(s,e,t=e){let n=new Set,i={x:0,z:0,fx:0,fz:-1},r=!1,a=!1,o="clear",l=s.createBuffer(1,s.sampleRate*2,s.sampleRate),c=31,h=l.getChannelData(0);for(let f=0;f<h.length;f++)c=Math.imul(c,1664525)+1013904223|0,h[f]=(c>>>0)/2147483648-1;let u=s.createBufferSource(),d=s.createBiquadFilter(),p=s.createGain();u.buffer=l,u.loop=!0,d.type="lowpass",d.frequency.value=450,p.gain.value=0,u.connect(d).connect(p).connect(t),u.start();let x=[{x:-57,z:-57,f:94,g:.055},{x:43,z:62,f:143,g:.035},{x:0,z:14,f:220,g:.016},{x:0,z:79,f:340,g:.06}].map((f,y)=>{let T=y===3?s.createBufferSource():s.createOscillator(),g=s.createGain(),_=s.createStereoPanner(),w=s.createBiquadFilter();return y===3?(T.buffer=l,T.loop=!0):(T.type="sine",T.frequency.value=f.f),w.type="lowpass",w.frequency.value=f.f*2,g.gain.value=0,T.connect(w).connect(g).connect(_).connect(t),T.start(),{...f,source:T,level:g,pan:_,low:w}});function v(){p.gain.setTargetAtTime(r?(o==="rain"?.07:.015)*(a?.12:1):0,s.currentTime,.1),d.frequency.setTargetAtTime(a?280:o==="rain"?2100:600,s.currentTime,.2)}function m(){for(let f of[...n]){try{f.source.stop()}catch{}f.release()}}return{setActive(f){if(r=f,v(),!f){m();for(let y of x)y.level.gain.setTargetAtTime(0,s.currentTime,.05)}},environment(f,y){a=f,y&&(o=y),v()},listener(f,y,T,g,_){Object.assign(i,{x:f,z:T,fx:g,fz:_});for(let w of x){let A=Math.hypot(f-w.x,T-w.z);w.level.gain.setTargetAtTime(r?w.g/(1+A*A*.025)*(a?.25:1):0,s.currentTime,.1),w.pan.pan.value=Math.max(-1,Math.min(1,((w.x-f)*-_+(w.z-T)*g)/Math.max(1,A)))}},play(f,y=0,T=0,g=0){if(!r||n.size>=16)return;let _=Math.hypot(y-i.x,g-i.z);if(_>90)return;let w={step:[.12,240,.16],step_inside:[.1,550,.12],door:[.7,850,.06],lift:[1.5,120,.05],tram:[.8,330,.08],discover:[.45,880,.08],hover:[1.2,160,.07]}[f];if(!w)return;let[A,b,C]=w,S=f.startsWith("step")||f==="door"||f==="hover"?s.createBufferSource():s.createOscillator();"buffer"in S?S.buffer=l:(S.type="sine",S.frequency.value=b);let P=s.createBiquadFilter(),M=s.createGain(),F=s.createStereoPanner();P.type="lowpass",P.frequency.value=a?b*.7:b,M.gain.setValueAtTime(0,s.currentTime),M.gain.linearRampToValueAtTime(C/(1+_*.08),s.currentTime+.015),M.gain.exponentialRampToValueAtTime(1e-4,s.currentTime+A),F.pan.value=Math.max(-1,Math.min(1,((y-i.x)*-i.fz+(g-i.z)*i.fx)/Math.max(_,1))),S.connect(P).connect(M).connect(F).connect(e);let I={source:S,release(){if(n.delete(I))for(let N of[S,P,M,F])N.disconnect()}};n.add(I),S.onended=I.release,S.start(),S.stop(s.currentTime+A+.02)},stats:()=>({voices:n.size,weather:o,inside:a}),dispose(){r=!1,m(),u.stop();for(let f of x){f.source.stop();for(let y of[f.source,f.level,f.pan,f.low])y.disconnect()}for(let f of[u,d,p])f.disconnect()}}}function km(s,e,t=e){let o=new Set,l=new Map,c=new Map,h={x:0,y:0,z:0,fx:0,fz:-1},u={x:0,z:0},d=!1,p=!1,x=0,v=0,m=0,f={kind:0,inactive:0,distant:0,pool:0,retrigger:0,quiet:0},y=s.createBuffer(1,s.sampleRate*2,s.sampleRate),T=9371,g=y.getChannelData(0);for(let C=0;C<g.length;C++)T=Math.imul(T,1664525)+1013904223|0,g[C]=(T>>>0)/2147483648-1;let _={step:{gain:.07,layers:[{wave:"noise",f:1900,q:1.7,dur:.05,lvl:.5},{wave:"noise",f:520,q:.9,dur:.1,lvl:.4},{wave:"tone",f:104,f2:74,dur:.12,lvl:.3}]},step_carry:{gain:.075,layers:[{wave:"noise",f:1250,q:1.2,dur:.06,lvl:.45},{wave:"noise",f:340,q:.8,dur:.13,lvl:.5},{wave:"tone",f:88,f2:62,dur:.15,lvl:.36}]},glide:{gain:.028,layers:[{wave:"noise",f:700,q:.6,dur:.26,lvl:.8},{wave:"tone",f:150,f2:120,dur:.24,lvl:.18}]},greet:{gain:.05,layers:[{wave:"tone",f:520,f2:740,dur:.11,lvl:.5,type:"triangle"},{wave:"tone",f:790,f2:960,dur:.13,lvl:.3,type:"triangle",delay:.1}]},work:{gain:.045,layers:[{wave:"noise",f:2700,q:2.6,dur:.05,lvl:.6},{wave:"tone",f:240,f2:175,dur:.22,lvl:.26,type:"sawtooth"}]},carry:{gain:.034,layers:[{wave:"noise",f:950,q:.7,dur:.12,lvl:.7},{wave:"noise",f:2300,q:1.5,dur:.07,lvl:.25}]},idle:{gain:.02,layers:[{wave:"noise",f:3400,q:3.2,dur:.028,lvl:.7}]},turn:{gain:.026,layers:[{wave:"tone",f:900,f2:1440,dur:.18,lvl:.4,type:"sawtooth"}]}};function w(C,S,P,M,F){let I=C.startsWith("robot_")?C.slice(6):C,N=_[I];if(!N)return f.kind++,!1;if(!d)return f.inactive++,!1;let H=S-u.x,D=M-u.z,V=Math.hypot(H,D);if(V>48)return f.distant++,!1;if(o.size>=10){let pe=null;for(let ce of o)(!pe||ce.distance>pe.distance)&&(pe=ce);if(!pe||V>=pe.distance)return f.pool++,!1;pe.release()}let k=s.currentTime;if(F!=null){let pe=c.get(F);if(pe!=null&&k-pe<.25)return f.retrigger++,!1;c.set(F,k)}let E=Math.abs(P-h.y),Z=N.gain*4.5/(1+V*.12)/(1+E*.0045)*(p?.5:1);if(Z<.002)return f.quiet++,!1;let W=S-h.x,Q=M-h.z,Fe=Math.max(-1,Math.min(1,(W*-h.fz+Q*h.fx)/Math.max(1,Math.hypot(W,Q)))),Ne=s.createGain(),ze=s.createStereoPanner(),se=s.createGain();Ne.gain.value=Z,ze.pan.value=Fe,se.gain.value=p?.14:.26;let le=s.createBiquadFilter();le.type="lowpass",le.Q.value=.6,le.frequency.value=Math.max(700,Math.min(11e3,11e3-V*130)),le.connect(Ne),Ne.connect(ze).connect(e),ze.connect(se).connect(t);let De=[le],be={distance:V,release(){if(o.delete(be)){for(let pe of De){try{pe.stop?.()}catch{}try{pe.disconnect()}catch{}}for(let pe of[Ne,ze,se])try{pe.disconnect()}catch{}}}};o.add(be);let ue=null,te=0;for(let pe of N.layers){let ce=k+(pe.delay||0),ne=Math.max(pe.dur,(pe.attack||.006)+.02),de=s.createGain();de.gain.setValueAtTime(0,ce),de.gain.linearRampToValueAtTime(pe.lvl,ce+(pe.attack||.006)),de.gain.exponentialRampToValueAtTime(1e-4,ce+ne),de.connect(le);let we;if(pe.wave==="noise"){we=s.createBufferSource(),we.buffer=y,we.loop=!0;let Ke=s.createBiquadFilter();Ke.type="bandpass",Ke.frequency.value=pe.f,Ke.Q.value=pe.q||1,we.connect(Ke).connect(de),De.push(Ke)}else we=s.createOscillator(),we.type=pe.type||"sine",we.frequency.setValueAtTime(pe.f,ce),pe.f2&&we.frequency.exponentialRampToValueAtTime(Math.max(20,pe.f2),ce+ne),we.connect(de);De.push(we,de);let Ve=ce+ne+.02;Ve>te&&(te=Ve,ue=we),we.start(ce),we.stop(Ve)}return ue&&(ue.onended=be.release),x++,Z>m&&(m=Z),!0}function A(C,S,P,M,F,I,N){if(!d)return;let H=l.get(C);if(!H){l.set(C,{x:S,z:M,phase:0});return}let D=S-H.x,V=M-H.z,k=Math.hypot(D,V);if(H.x=S,H.z=M,!F){H.phase=0;return}if(k>8)return;H.phase+=k;let E=N==="glide"?1.6:I?1.05:.8;H.phase<E||(H.phase-=E,w(N==="glide"?"glide":I?"step_carry":"step",S,P,M,C),v++)}function b(){for(let C of[...o])C.release();for(let[,C]of l)C.x=h.x,C.z=h.z,C.phase=0;c.clear()}return{setActive(C){d!==C&&(d=C,C||b())},environment(C){p=!!C},listener(C,S,P,M,F,I,N){Object.assign(h,{x:C,y:S,z:P,fx:M,fz:F}),u.x=I??C,u.z=N??P},action(C,S,P,M,F){return w(C,S,P,M,F)},gait:A,reset(){l.clear(),c.clear()},stats:()=>({voices:o.size,played:x,steps:v,gaits:l.size,inside:p,peak:+m.toFixed(4),rejected:{...f}}),dispose(){d=!1,b()}}}var cd=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]],Bm=[440,523.25,587.33,659.25,783.99,880,1046.5],ns=12,Vc={harbour:{x:0,z:79},quay:{x:0,z:72},tower:{x:0,z:-12},square:{x:8,z:40}},gM=[{kind:"chime",weight:15},{kind:"swoosh",weight:13},{kind:"gust",weight:7},{kind:"birds",weight:8,when:s=>s.day>.45&&s.weather==="clear"},{kind:"gull",weight:6,when:s=>s.day>.2&&s.weather==="clear"},{kind:"pigeon",weight:4,when:s=>s.day>.6&&!s.busy},{kind:"cricket",weight:6,when:s=>s.day<.25&&s.weather!=="rain"},{kind:"horn",weight:4,when:s=>s.day<.45&&s.weather!=="rain"},{kind:"bell",weight:3,when:s=>s.day<.7},{kind:"hammer",weight:4,when:s=>s.day>.3},{kind:"drip",weight:10,when:s=>s.weather==="rain"},{kind:"windchime",weight:4,when:s=>s.weather==="clear"&&!s.busy},{kind:"siren",weight:2,when:s=>s.evening>.3&&s.busy}];function xM(s,e,t){let n=Math.floor(s.sampleRate*e),i=s.createBuffer(2,n,s.sampleRate),r=977;for(let a=0;a<2;a++){let o=i.getChannelData(a);for(let l=0;l<n;l++)r=Math.imul(r,1664525)+1013904223|0,o[l]=((r>>>0)/2147483648-1)*Math.pow(1-l/n,t)}return i}function bM(s=()=>{}){let e=null,t=null,n=null,i=!1,r=!1,a=!1,o=0,l=null,c=[],h=[],u=new Float32Array(256),d=new Set,p=.18,x=null,v=null,m=null,f=null,y=null,T=!1,g="clear",_=[],w=null,A=null,b=null,C=null,S=null,P=null,M=null,F=null,I=null,N="0.000",H="0.000",D="0.000",V=0,k=0,E=0,Z=0,W={x:0,y:2.4,z:0,fx:0,fz:-1,origin:{x:0,z:0}},Q={busy:!1,day:0,evening:0,weather:"clear"},Fe={ambience:1,effects:1,voice:1};try{a=localStorage.getItem("aurago.desktop.sysworld.sound")==="true"}catch{}function Ne(){if(e||i)return;e=new(window.AudioContext||window.webkitAudioContext);let G=Mt=>(c.push(Mt),Mt),j=Mt=>(h.push(Mt),G(Mt));t=G(e.createGain()),t.gain.value=0,n=G(e.createAnalyser()),n.fftSize=512,t.connect(n),n.connect(e.destination),m=G(e.createGain()),m.gain.value=Fe.ambience,m.connect(t),f=G(e.createGain()),f.gain.value=Fe.effects,f.connect(t),y=G(e.createGain()),y.gain.value=Fe.voice,y.connect(t),S=G(e.createConvolver()),S.buffer=xM(e,3.4,2.6);let he=G(e.createGain());he.gain.value=.55,S.connect(he).connect(m),l=Om(e,y),x=zm(e,f,m),x.environment(T,g),v=km(e,f,m),v.environment(T),v.listener(W.x,W.y,W.z,W.fx,W.fz,W.origin.x,W.origin.z);for(let[Mt,Mn]of[[55,.07],[82.4069,.022],[110,.01]]){let kt=j(e.createOscillator()),nn=G(e.createGain());kt.frequency.value=Mt,nn.gain.value=Mn,kt.connect(nn).connect(m),kt.start()}w=G(e.createBiquadFilter()),w.type="lowpass",w.frequency.value=650,w.Q.value=.6,A=G(e.createGain()),A.gain.value=.042;let Se=G(e.createGain());Se.gain.value=.9,w.connect(A).connect(m),A.connect(Se).connect(S),_=cd[0].map(Mt=>[-6,6].map(Mn=>{let kt=j(e.createOscillator()),nn=G(e.createGain());return kt.type="sawtooth",kt.frequency.value=Mt,kt.detune.value=Mn,nn.gain.value=.22,kt.connect(nn).connect(w),kt.start(),kt}));let ie=j(e.createOscillator()),ve=G(e.createGain());ie.frequency.value=.05,ve.gain.value=180,ie.connect(ve).connect(w.frequency),ie.start();let Ee=e.createBuffer(1,e.sampleRate*4,e.sampleRate),Te=Ee.getChannelData(0),Ie=0;for(let Mt=0;Mt<Te.length;Mt++)Ie=(Ie+(Math.random()*2-1)*.018)/1.018,Te[Mt]=Ie;let X=Te[Te.length-1]-Te[0];for(let Mt=0;Mt<Te.length;Mt++)Te[Mt]-=X*Mt/(Te.length-1);let Ae=j(e.createBufferSource());b=G(e.createBiquadFilter()),C=G(e.createGain()),Ae.buffer=Ee,Ae.loop=!0,b.type="lowpass",b.frequency.value=680,b.Q.value=.25,C.gain.value=.32,Ae.connect(b).connect(C).connect(m),Ae.start();let Me=j(e.createOscillator()),fe=G(e.createGain());Me.frequency.value=.075,fe.gain.value=160,Me.connect(fe).connect(b.frequency),Me.start(),P=e.createBuffer(1,e.sampleRate*2,e.sampleRate);let Pe=P.getChannelData(0);for(let Mt=0;Mt<Pe.length;Mt++)Pe[Mt]=Math.random()*2-1;let _e=j(e.createBufferSource());_e.buffer=P,_e.loop=!0;let qe=G(e.createBiquadFilter());qe.type="bandpass",qe.frequency.value=620,qe.Q.value=.5;let Xe=j(e.createOscillator()),U=G(e.createGain());Xe.frequency.value=.08,U.gain.value=140,Xe.connect(U).connect(qe.frequency),Xe.start(),M=G(e.createGain()),M.gain.value=0,_e.connect(qe).connect(M).connect(m),_e.start();let q=j(e.createBufferSource());q.buffer=P,q.loop=!0;let Y=G(e.createBiquadFilter());Y.type="bandpass",Y.frequency.value=320,Y.Q.value=.7;let ae=j(e.createOscillator()),K=G(e.createGain());ae.frequency.value=.13,K.gain.value=90,ae.connect(K).connect(Y.frequency),ae.start(),F=G(e.createGain()),F.gain.value=0,q.connect(Y).connect(F).connect(m),q.start();let $=j(e.createOscillator()),Re=j(e.createOscillator());$.type="sine",$.frequency.value=118,Re.type="sine",Re.frequency.value=236;let He=G(e.createGain());He.gain.value=.26;let ot=G(e.createGain());ot.gain.value=.9;let Ze=j(e.createOscillator()),Jt=G(e.createGain());Ze.frequency.value=.19,Jt.gain.value=.08,Ze.connect(Jt).connect(ot.gain),Ze.start();let Xt=G(e.createBiquadFilter());Xt.type="lowpass",Xt.frequency.value=420,Xt.Q.value=.4,I=G(e.createGain()),I.gain.value=0,$.connect(ot),Re.connect(He).connect(ot),ot.connect(Xt).connect(I).connect(m),$.start(),Re.start(),xe(),Ue()}let ze=()=>!!e&&a&&r&&!i&&p>0;function se(G){let j={release(){if(d.delete(j))for(let he of G){try{he.stop?.()}catch{}try{he.disconnect()}catch{}}}};return d.add(j),G[0].onended=j.release,Z++,j}function le(G,j,he){let Se=e.createStereoPanner(),ie=e.createGain();return Se.pan.value=j,ie.gain.value=he,G.connect(Se).connect(m),Se.connect(ie).connect(S),[Se,ie]}function De(G){let j=G.x-W.x,he=G.z-W.z,Se=G.x-W.origin.x,ie=G.z-W.origin.z,ve=Math.hypot(Se,ie),Ee=Math.max(1,Math.hypot(j,he));return{distance:ve,pan:Math.max(-1,Math.min(1,(j*-W.fz+he*W.fx)/Ee)),fade:1/(1+ve*.02)}}function be(G){if(G==="horn"||G==="gull")return Vc.harbour;if(G==="windchime")return Vc.quay;if(G==="bell")return Vc.tower;if(G==="pigeon")return Vc.square;let j=Math.random()*Math.PI*2,he=24+Math.random()*44;return{x:W.origin.x+Math.sin(j)*he,z:W.origin.z+Math.cos(j)*he}}function ue(G){let j=e.currentTime,he=Bm[Math.floor(Math.random()*Bm.length)],Se=e.createOscillator(),ie=e.createOscillator(),ve=e.createGain(),Ee=e.createGain(),{pan:Te,fade:Ie}=De(G);Se.frequency.value=he,ie.frequency.value=he*2.76,ve.gain.value=.22,Ee.gain.setValueAtTime(0,j),Ee.gain.linearRampToValueAtTime(Q.busy?.02:.014,j+.008),Ee.gain.exponentialRampToValueAtTime(1e-4,j+2.8),Se.connect(Ee),ie.connect(ve).connect(Ee),se([Se,ie,ve,Ee,...le(Ee,Te,1.4)]),Se.start(j),ie.start(j),Se.stop(j+2.9),ie.stop(j+2.9)}function te(G){let j=e.currentTime,he=e.createBufferSource(),Se=e.createBiquadFilter(),ie=e.createGain(),{pan:ve,fade:Ee}=De(G);he.buffer=P,he.loop=!0,Se.type="bandpass",Se.Q.value=1.4,Se.frequency.setValueAtTime(260,j),Se.frequency.exponentialRampToValueAtTime(1100,j+1.6),Se.frequency.exponentialRampToValueAtTime(380,j+3.4),ie.gain.setValueAtTime(0,j),ie.gain.linearRampToValueAtTime(.03*Ee,j+1.6),ie.gain.exponentialRampToValueAtTime(1e-4,j+3.6),he.connect(Se).connect(ie);let[Te,Ie]=le(ie,ve,.5),X=Math.max(-1,ve-.6),Ae=Math.min(1,ve+.6);Te.pan.setValueAtTime(X,j),Te.pan.linearRampToValueAtTime(Ae,j+3.4),se([he,Se,ie,Te,Ie]),he.start(j),he.stop(j+3.7)}function pe(){let G=e.currentTime,j=g==="rain"?.36:.32;b.frequency.cancelScheduledValues(G),C.gain.cancelScheduledValues(G),b.frequency.setTargetAtTime(1200+Math.random()*700,G,.8),b.frequency.setTargetAtTime(680,G+2.6,1.4),C.gain.setTargetAtTime(j*1.7,G,.7),C.gain.setTargetAtTime(j,G+2.6,1.5),Z++}function ce(G){let j=e.currentTime,he=e.createBiquadFilter(),Se=e.createGain(),{pan:ie,fade:ve}=De(G);he.type="lowpass",he.frequency.value=420,Se.gain.setValueAtTime(0,j),Se.gain.linearRampToValueAtTime(.028*ve,j+.9),Se.gain.setValueAtTime(.028*ve,j+3.2),Se.gain.exponentialRampToValueAtTime(1e-4,j+5.5);let Ee=[69.3,103.83].map(Te=>{let Ie=e.createOscillator();return Ie.type="sawtooth",Ie.frequency.value=Te,Ie.detune.value=Math.random()*8-4,Ie.connect(he),Ie});he.connect(Se),se([...Ee,he,Se,...le(Se,ie,1.6)]),Ee.forEach(Te=>{Te.start(j),Te.stop(j+5.6)})}function ne(G){let j=e.currentTime,he=2+Math.floor(Math.random()*3),{pan:Se,fade:ie}=De(G);for(let ve=0;ve<he&&d.size<ns;ve++){let Ee=j+ve*(.12+Math.random()*.1),Te=e.createOscillator(),Ie=e.createGain(),X=2600+Math.random()*900;Te.frequency.setValueAtTime(X,Ee),Te.frequency.exponentialRampToValueAtTime(X*1.5,Ee+.07),Ie.gain.setValueAtTime(0,Ee),Ie.gain.linearRampToValueAtTime(.006*ie,Ee+.015),Ie.gain.exponentialRampToValueAtTime(1e-4,Ee+.13),Te.connect(Ie),se([Te,Ie,...le(Ie,Se,.6)]),Te.start(Ee),Te.stop(Ee+.15)}}function de(G){let j=e.currentTime,he=2+Math.floor(Math.random()*3),Se=760+Math.random()*280,{pan:ie,fade:ve}=De(G);for(let Ee=0;Ee<he&&d.size<ns;Ee++){let Te=j+Ee*(.2+Math.random()*.14),Ie=e.createOscillator(),X=e.createBiquadFilter(),Ae=e.createGain();Ie.type="sawtooth",Ie.frequency.setValueAtTime(Se,Te),Ie.frequency.exponentialRampToValueAtTime(Se*.5,Te+.26),X.type="bandpass",X.frequency.value=1500,X.Q.value=3.6,Ae.gain.setValueAtTime(0,Te),Ae.gain.linearRampToValueAtTime(.012*ve,Te+.04),Ae.gain.exponentialRampToValueAtTime(1e-4,Te+.34),Ie.connect(X).connect(Ae),se([Ie,X,Ae,...le(Ae,ie,.9)]),Ie.start(Te),Ie.stop(Te+.36)}}function we(G){let j=e.currentTime,he=2+Math.floor(Math.random()*2),Se=420+Math.random()*90,{pan:ie,fade:ve}=De(G);for(let Ee=0;Ee<he&&d.size<ns;Ee++){let Te=j+Ee*(.17+Math.random()*.08),Ie=e.createOscillator(),X=e.createBiquadFilter(),Ae=e.createGain();Ie.type="triangle",Ie.frequency.setValueAtTime(Se,Te),Ie.frequency.linearRampToValueAtTime(Se*.72,Te+.12),X.type="lowpass",X.frequency.value=1100,Ae.gain.setValueAtTime(0,Te),Ae.gain.linearRampToValueAtTime(.01*ve,Te+.03),Ae.gain.exponentialRampToValueAtTime(1e-4,Te+.16),Ie.connect(X).connect(Ae),se([Ie,X,Ae,...le(Ae,ie,.5)]),Ie.start(Te),Ie.stop(Te+.18)}}function Ve(G){let j=e.currentTime,he=3+Math.floor(Math.random()*4),Se=3900+Math.random()*900,{pan:ie,fade:ve}=De(G);for(let Ee=0;Ee<he&&d.size<ns;Ee++){let Te=j+Ee*.055,Ie=e.createOscillator(),X=e.createGain();Ie.type="square",Ie.frequency.value=Se,X.gain.setValueAtTime(0,Te),X.gain.linearRampToValueAtTime(.005*ve,Te+.008),X.gain.exponentialRampToValueAtTime(1e-4,Te+.035),Ie.connect(X),se([Ie,X,...le(X,ie,.35)]),Ie.start(Te),Ie.stop(Te+.05)}}function Ke(G){let j=e.currentTime,he=196,{pan:Se,fade:ie}=De(G),ve=e.createGain();ve.gain.setValueAtTime(0,j),ve.gain.linearRampToValueAtTime(.02*ie,j+.012),ve.gain.exponentialRampToValueAtTime(1e-4,j+4.2);let Ee=[1,2.02,2.99,4.18,5.43].map((Te,Ie)=>{let X=e.createOscillator(),Ae=e.createGain();return X.type="sine",X.frequency.value=he*Te,Ae.gain.value=.5/(Ie+1),X.connect(Ae).connect(ve),X});se([...Ee,ve,...le(ve,Se,1.8)]),Ee.forEach(Te=>{Te.start(j),Te.stop(j+4.3)})}function it(G){let j=e.currentTime,he=2+Math.floor(Math.random()*3),{pan:Se,fade:ie}=De(G);for(let ve=0;ve<he&&d.size<ns;ve++){let Ee=j+ve*(.5+Math.random()*.35),Te=e.createBufferSource(),Ie=e.createBiquadFilter(),X=e.createGain(),Ae=e.createOscillator(),Me=e.createGain();Te.buffer=P,Te.loop=!0,Ie.type="bandpass",Ie.frequency.value=420,Ie.Q.value=1.1,X.gain.setValueAtTime(0,Ee),X.gain.linearRampToValueAtTime(.02*ie,Ee+.006),X.gain.exponentialRampToValueAtTime(1e-4,Ee+.42),Ae.type="sine",Ae.frequency.value=1180,Me.gain.value=.2,Te.connect(Ie).connect(X),Ae.connect(Me).connect(X),se([Te,Ie,X,Ae,Me,...le(X,Se,.9)]),Te.start(Ee),Te.stop(Ee+.44),Ae.start(Ee),Ae.stop(Ee+.44)}}function et(G){let j=e.currentTime,he=2+Math.floor(Math.random()*4),{pan:Se,fade:ie}=De(G);for(let ve=0;ve<he&&d.size<ns;ve++){let Ee=j+ve*(.06+Math.random()*.14),Te=e.createOscillator(),Ie=e.createGain();Te.type="sine",Te.frequency.setValueAtTime(900+Math.random()*700,Ee),Te.frequency.exponentialRampToValueAtTime(320,Ee+.06),Ie.gain.setValueAtTime(0,Ee),Ie.gain.linearRampToValueAtTime(.008*ie,Ee+.005),Ie.gain.exponentialRampToValueAtTime(1e-4,Ee+.12),Te.connect(Ie),se([Te,Ie,...le(Ie,Se,.4)]),Te.start(Ee),Te.stop(Ee+.14)}}function ht(G){let j=e.currentTime,he=3+Math.floor(Math.random()*4),Se=[1046.5,1174.7,1396.9,1567.98,1760,2093],{pan:ie,fade:ve}=De(G);for(let Ee=0;Ee<he&&d.size<ns;Ee++){let Te=j+Ee*(.09+Math.random()*.13),Ie=Se[Math.floor(Math.random()*Se.length)],X=e.createOscillator(),Ae=e.createGain();X.type="sine",X.frequency.value=Ie,Ae.gain.setValueAtTime(0,Te),Ae.gain.linearRampToValueAtTime(.006*ve,Te+.004),Ae.gain.exponentialRampToValueAtTime(1e-4,Te+.9),X.connect(Ae),se([X,Ae,...le(Ae,ie,1.5)]),X.start(Te),X.stop(Te+.95)}}function J(G){let j=e.currentTime,{pan:he,fade:Se}=De(G),ie=e.createOscillator(),ve=e.createGain();ie.type="sine",ie.frequency.setValueAtTime(420,j),ie.frequency.linearRampToValueAtTime(620,j+1.8),ie.frequency.linearRampToValueAtTime(430,j+3.6),ve.gain.setValueAtTime(0,j),ve.gain.linearRampToValueAtTime(.01*Se,j+.5),ve.gain.setValueAtTime(.01*Se,j+2.8),ve.gain.exponentialRampToValueAtTime(1e-4,j+4.2),ie.connect(ve),se([ie,ve,...le(ve,he,1.2)]),ie.start(j),ie.stop(j+4.3)}let Ut={chime:ue,swoosh:te,gust:pe,horn:ce,birds:ne,gull:de,pigeon:we,cricket:Ve,bell:Ke,hammer:it,drip:et,windchime:ht,siren:J};function ut(){let G=gM.filter(Se=>!Se.when||Se.when(Q));if(!G.length)return"chime";let j=0;for(let Se of G)j+=Se.weight;let he=Math.random()*j;for(let Se of G)if(he-=Se.weight,he<=0)return Se.kind;return G[G.length-1].kind}function O(){if(d.size>=ns)return;let G=ut(),j=be(G);Ut[G]?.(j)}function R(){clearTimeout(V),V=0,ze()&&(V=setTimeout(()=>{V=0,ze()&&(O(),R())},(Q.busy?1900:3600)+Math.random()*9e3))}function re(){clearTimeout(k),k=0,ze()&&(k=setTimeout(()=>{k=0,ze()&&(E=(E+1)%cd.length,_.forEach((G,j)=>G.forEach(he=>he.frequency.setTargetAtTime(cd[E][j],e.currentTime,1.8))),re())},16e3))}function oe(){clearTimeout(V),clearTimeout(k),V=k=0;for(let G of[...d])G.release()}function xe(){if(!e)return;let G=e.currentTime;w.frequency.setTargetAtTime(Q.busy?1500:650+Q.day*450+Q.evening*150,G,2.5),A.gain.setTargetAtTime((Q.busy?.055:.042)*(Q.weather==="rain"?.75:1),G,2)}function Ue(){if(!e||!M||!F||!I)return;let G=Math.max(-76,Math.min(76,W.origin.x)),j=Math.hypot(W.origin.x-G,W.origin.z-77),he=Math.max(0,1-(j-6)/42),Se=Math.max(0,1-(j-25)/110)*.18,ie=(T?0:Math.min(1,he+Se)*.05*(g==="rain"?1.2:1)).toFixed(3),ve=((Q.busy?.03:.012)*(.35+Q.day*.65)*(T?.25:1)).toFixed(3),Ee=Math.hypot(W.origin.x-G,W.origin.z-75.5),Te=Math.max(0,1-(Ee-8)/46),Ie=(T?0:Math.min(1,Te)*.012*(1-Q.day)*(g==="rain"?.6:1)).toFixed(3);if(ie===N&&ve===H&&Ie===D)return;N=ie,H=ve,D=Ie;let X=e.currentTime;M.gain.setTargetAtTime(Number(ie),X,.5),F.gain.setTargetAtTime(Number(ve),X,.8),I.gain.setTargetAtTime(Number(Ie),X,.8)}function ke(G=!1){if(clearTimeout(o),!i){if(G&&a)try{Ne()}catch{a=!1,s(!1);return}e&&(l?.setActive(a&&r&&p>0&&Fe.voice>0),x?.setActive(a&&r&&p>0),v?.setActive(a&&r&&p>0&&Fe.effects>0),a&&r?(e.resume().catch(()=>{}),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(p,e.currentTime,.25),V||R(),k||re()):(oe(),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(0,e.currentTime,.035),o=setTimeout(()=>{!i&&(!a||!r)&&e.suspend().catch(()=>{})},180)))}}return{enabled:()=>a,toggle(){a=!a;try{localStorage.setItem("aurago.desktop.sysworld.sound",String(a))}catch{}ke(!0),s(a)},unlock(){ke(!0)},setActive(G){r!==G&&(r=G,ke())},setVolume(G){Number.isFinite(G)&&(p=Math.max(0,Math.min(.35,G)),ke())},setListener(G,j,he,Se,ie,ve,Ee){Object.assign(W,{x:G,y:j,z:he,fx:Se,fz:ie}),W.origin.x=ve??G,W.origin.z=Ee??he,l?.setListener(G,j,he,Se,ie),x?.listener(G,j,he,Se,ie),v?.listener(G,j,he,Se,ie,W.origin.x,W.origin.z),Ue()},setChannel(G,j){if(!(G in Fe)||!Number.isFinite(j))return;Fe[G]=Math.max(0,Math.min(1,j));let he={ambience:m,effects:f,voice:y}[G];he&&he.gain.setTargetAtTime(Fe[G],e.currentTime,.05),ke()},effect(G,j,he,Se,ie){if(String(G).startsWith("robot_")){v?.action(G,j,he,Se,ie);return}x?.play(G,j,he,Se)},gait(G,j,he,Se,ie,ve,Ee){v?.gait(G,j,he,Se,ie,ve,Ee)},setEnvironment(G,j){T=!!G,["clear","rain","fog"].includes(j)&&(g=j),x?.environment(T,g),v?.environment(T),Ue()},setMood(G={}){Object.assign(Q,{busy:!!G.busy,day:Number.isFinite(G.day)?G.day:Q.day,evening:Number.isFinite(G.evening)?G.evening:Q.evening,weather:["clear","rain","fog"].includes(G.weather)?G.weather:Q.weather}),xe(),Ue()},thunder(G=1){if(!ze()||T||d.size>=ns)return;let j=Math.max(0,Math.min(5,G)),he=e.currentTime+j,Se=Math.min(1,j/4),ie=e.createBufferSource(),ve=e.createBiquadFilter(),Ee=e.createGain();ie.buffer=P,ie.loop=!0,ve.type="lowpass",ve.Q.value=.7,ve.frequency.setValueAtTime(900-Se*500,he),ve.frequency.exponentialRampToValueAtTime(70,he+3.5),Ee.gain.setValueAtTime(0,he),Ee.gain.linearRampToValueAtTime(.09*(1-Se*.5),he+.06+Se*.25),Ee.gain.exponentialRampToValueAtTime(.025,he+1.3),Ee.gain.exponentialRampToValueAtTime(1e-4,he+5.5),ie.connect(ve).connect(Ee),se([ie,ve,Ee,...le(Ee,Math.random()*.8-.4,.8)]),ie.start(he),ie.stop(he+5.6)},stats(){let G=0;if(n&&e.state==="running"){n.getFloatTimeDomainData(u);for(let j of u)G+=j*j}return{voice:l?.stats(),effects:x?.stats(),foley:v?.stats(),channels:{...Fe},enabled:a,active:r,state:e?.state||"uninitialized",volume:p,rms:Math.sqrt(G/u.length),soundscape:{chord:E,events:Z,transient:d.size,scheduled:!!V,mood:{...Q},beds:{water:Number(N),murmur:Number(H),lamp:Number(D)},listener:{x:W.x,z:W.z}}}},dispose(){i||(i=!0,oe(),v?.dispose(),x?.dispose(),l?.dispose(),clearTimeout(o),h.forEach(G=>{try{G.stop()}catch{}}),c.forEach(G=>G.disconnect()),e&&e.close().catch(()=>{}))}}}var vn=[{id:"agent",asset:"agent-spire",x:0,z:-12,height:87,radius:13},{id:"infra",asset:"compute-foundry",x:-43,z:-53,height:24,radius:18},{id:"integrations",asset:"integration-gate",x:43,z:-10,height:34,radius:14},{id:"missions",asset:"mission-terminal",x:43,z:36,height:23,radius:15},{id:"memory",asset:"memory-archive",x:-43,z:-10,height:37,radius:14},{id:"graph",asset:"knowledge-atrium",x:-43,z:36,height:28,radius:17},{id:"operations",asset:"operations-beacon",x:0,z:36,height:22,radius:6}],Wc={low:{lod:2,dpr:1,shadow:0,bloom:!1},medium:{lod:1,dpr:1.25,shadow:1024,bloom:!1},high:{lod:0,dpr:1.5,shadow:2048,bloom:!0},ultra:{lod:0,dpr:2,shadow:4096,bloom:!0}},hd=new B(122,106,183),ud=class extends Zn{constructor(e,t,n){super(),this.scene=e,this.camera=t,this.needsSwap=!1,this.target=new an(1,1,{type:pn,samples:n}),this.quad=new Oi(new yt({uniforms:_i.clone(es.uniforms),vertexShader:es.vertexShader,depthTest:!1,depthWrite:!1,fragmentShader:"uniform sampler2D tDiffuse;varying vec2 vUv;void main(){vec4 c=texture2D(tDiffuse,vUv);if(any(isnan(c))||any(isinf(c)))c=vec4(0.0,0.0,0.0,1.0);gl_FragColor=c;}"}))}render(e,t,n){let i=e.autoClear;e.autoClear=!1,e.setRenderTarget(this.target),e.clear(),e.render(this.scene,this.camera),this.quad.material.uniforms.tDiffuse.value=this.target.texture,e.setRenderTarget(this.renderToScreen?null:n),this.quad.render(e),e.autoClear=i}setSize(e,t){this.target.setSize(e,t)}dispose(){this.target.dispose(),this.quad.material.dispose(),this.quad.dispose()}},ta=new B(0,22,-12);function vM(s){return s.name+(s.vertexColors?"|vertex-colours":"")}var Hm=Gp().map(s=>({asset:s.id,x:s.x,z:s.z,y:0,angle:s.angle,scale:[1,1,1]})),Qs=[];{let s=(a,o,l,c=0,h=0,u=[1,1,1])=>Qs.push({asset:a,x:o,z:l,y:c,angle:h,scale:u}),{xs:e,zs:t}=Bt,n=gt.pavement-.05;for(let a of t)for(let o of[-55,-31,31,55])s("street-lamp",o,a-4.5,n),s("planter",o+4*Math.sign(Math.sign(o)*43-o),a-4.5,n);for(let{x:a,z:o,scale:l}of Uc)s("data-tower-a",a,o,0,0,[1,l,1]);s("skybridge",39.75,-55,15),s("server-rack",-55,-36.7,gt.pavement),s("server-rack",-49,-36.7,gt.pavement);let i=7919,r=()=>(i=Math.imul(i,1664525)+1013904223>>>0)/4294967296;for(let a=0;a<4;a++)for(let o=0;o<12;o++){let l=r(),c=r(),h=r(),u=r(),d=r(),p=r();if(u<.12)continue;let x=(o-5.5)*22+(l-.5)*12,v=-143-a*29+(c-.5)*14,m=1-Math.min(1,Math.abs(x)/150),f=(.5+h*h+.45*m)*(a?1:.85),y=.7+.34*d;s(l>.5?"data-tower-a":"data-tower-b",x,v,-3,p<.3?Math.PI/2:0,[y,f,y])}for(let a=0;a<16;a++){let o=(a-7.5)*34+(r()-.5)*16,l=-268-r()*46;s(r()<.5?"data-tower-a":"data-tower-b",o,l,-3,0,[1.1,.55+r()*1.1,1.1])}}var _M=[...Ip(Qs),...Dn.flatMap(s=>[-3,3].flatMap(e=>[-3,3].map(t=>({x:s.x+e,z:s.z+t,r:4.25,asset:"interior"}))))];async function hT(s,e){let t=!1,n=!0,i="orbit",r=e.quality||"auto",a=r==="auto"?"high":r,o=0,l=null,c=0,h=0,u=0,d=0,p=0,x=0,v=0,m=null,f=e.reducedMotion,y=1,T=1,g=[],_=!1,w=null,A=null,b=null,C=new AbortController,S=new Set,P=new Map,M=new Map,F=new Set,I=new Set,N=new Set,H=[],D=new yc({antialias:!0,alpha:!1});D.toneMapping=Ws,D.toneMappingExposure=.98,D.shadowMap.type=Gs,D.shadowMap.autoUpdate=!1,D.info.autoReset=!1;let V=D.domElement;V.className="sysworld-gl",V.tabIndex=0,V.setAttribute("aria-label",e.label),s.append(V);let k=new Us;k.background=new Le(528926),k.fog=new va(528926,.004);let E=new dn(43,1,.3,1800);E.position.copy(hd);let Z=Nc(),W=Z.register("visitor",{circles:[{x:0,z:0,r:.38}],minY:-.4,maxY:.4,reach:.38},E.position,100),Q=new Ac(E,V);Q.target.copy(ta),Q.enableDamping=!0,Q.dampingFactor=.085,Q.minDistance=12,Q.maxDistance=410,Q.maxPolarAngle=Math.PI*.485,Q.update();let Fe=0,Ne=()=>{let z=ze?.progress();e.onProgress?.(p+(z?.bytes||0),Fe+(z?.expected||0))},ze=Um(D,{url:z=>e.resourceURL("/3d/system-world/textures/v1/"+z),onProgress:()=>Ne(),shelters:Dn.map(z=>[z.x-z.width/2,z.z-z.depth/2,z.x+z.width/2,z.z+z.depth/2])}),se=new Xr(D),le=new Rc,De=se.fromScene(le,.05);k.environment=De.texture,k.environmentIntensity=.48,le.dispose(),se.dispose();let be=new Na(12639487,1056813,.8);k.add(be);let ue=new $i(13164287,3.2);ue.position.set(-70,145,85),ue.castShadow=!0,Object.assign(ue.shadow.camera,{left:-108,right:108,top:108,bottom:-108,near:1,far:360}),ue.shadow.normalBias=.09,ue.shadow.bias=-15e-5,ue.shadow.radius=2.4,k.add(ue);let te=new $i(16758130,2.4);te.position.set(90,80,-120),k.add(te);let pe=new $i(14195306,.42);pe.position.set(-10,-40,30),k.add(pe);let ce=new gi(6479871,0,75,2);ce.position.set(0,34,-12),k.add(ce);let ne=new an(1,1,{type:pn}),de=new Pc(D,ne);de.addPass(new ud(k,E,Math.min(4,D.capabilities.maxSamples)));let we=new $r(new Be(1,1),.28,.68,1.06);de.addPass(we),de.addPass(new Ic);let Ve=new vt,Ke=new vt,it=new vt;k.add(Ve),Ve.add(Ke,it);let et=Lm(k);function ht(z,Ce){return F.add(z),I.add(Ce),new st(z,Ce)}let J=new zn({color:1911345,metalness:.05,roughness:.82}),Ut=new zn({color:1382685,metalness:.05,roughness:.9});I.add(J),I.add(Ut);let ut=new pi(Bt.maxX-Bt.minX,Js.top-Js.bottom,Bt.maxZ-Bt.minZ);F.add(ut);let O=new st(ut,[J,J,Ut,J,J,J]);O.position.set((Bt.minX+Bt.maxX)/2,(Js.top+Js.bottom)/2,(Bt.minZ+Bt.maxZ)/2),O.receiveShadow=!0,Ve.add(O),ze.apply(J,"world","ground"),ze.apply(Ut,"world","city.road");let R=Vp(),re=new pi(1,1,1);F.add(re);let oe=new Sn(re,J,R.length);oe.name="plots",oe.receiveShadow=!0,R.forEach((z,Ce)=>oe.setMatrixAt(Ce,new ft().compose(new B((z.x0+z.x1)/2,(z.top+z.bottom)/2,(z.z0+z.z1)/2),new Qt,new B(z.x1-z.x0,z.top-z.bottom,z.z1-z.z0)))),Ve.add(oe);let xe=rm(k,{sunDirection:ue.position,lamps:Qs.filter(z=>z.asset==="street-lamp")});de.addPass(xe.post);let Ue=om(k,{anchors:vn.map(z=>({id:z.id,x:z.x,z:z.z,height:z.height}))}),ke=vn.find(z=>z.id==="memory"),G=Jp(k,ke,{roof:21,label:e.memoryLabel}),j=cm(k,{traffic:Z,surfaces:ze}),he=Im(k,{sun:ue,rim:te,hemisphere:be,atmosphere:xe,surfaces:ze,onThunder:z=>e.onThunder?.(z)}),Se="",ie=null,ve={value:0},Ee=new fs(1,1.045,96);F.add(Ee);let Te=(z,Ce)=>{let We=new yt({transparent:!0,depthWrite:!1,side:Kt,uniforms:{color:{value:new Le(9365742)},time:ve,opacity:{value:z},dashes:{value:Ce}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float time,opacity,dashes;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;float dash=dashes>0.5?step(0.3,fract(a*dashes-time*0.35)):1.0;gl_FragColor=vec4(color,opacity*dash*(0.8+0.2*sin(time*2.2)));}"});return I.add(We),We},Ie=new st(Ee,Te(.9,18)),X=new st(Ee,Te(.38,0));for(let z of[Ie,X])z.rotation.x=-Math.PI/2,z.visible=!1,Ve.add(z);let Ae=new yt({transparent:!0,depthWrite:!1,side:Kt,uniforms:{color:{value:new Le(9365742)},alpha:{value:0}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float alpha;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;gl_FragColor=vec4(color,alpha*(0.55+0.45*sin(a*3.0)));}"});I.add(Ae);let Me=new st(new fs(.9,1,96),Ae);F.add(Me.geometry),Me.rotation.x=-Math.PI/2,Me.visible=!1,Ve.add(Me);let fe=2,Pe={error:16742504,running:7730385,stale:8096667,idle:8175587},_e={},qe=()=>Ie.material.uniforms.color.value.setHex(Pe[_e[m]]??9365742),Xe=new Vt({color:6742230,toneMapped:!1}),U=new Sn(new Pi(.65,10,6),Xe,vn.length);F.add(U.geometry),I.add(Xe),Ve.add(U);let q=new Wt,Y=new B,ae=new Ha,K=new Be;vn.forEach((z,Ce)=>{q.position.set(z.x,z.height+2,z.z),q.updateMatrix(),U.setMatrixAt(Ce,q.matrix),U.setColorAt(Ce,new Le(8425630))}),e.signal?.addEventListener("abort",bt,{once:!0});let $;try{if(e.signal?.aborted)throw Error("Disposed");let z=await fetch(e.assetURL("manifest.json"),{signal:C.signal});if(!z.ok)throw Error("City manifest unavailable");$=await z.json()}catch(z){throw bt(),z}let Re=new Map($.assets.map(z=>[z.id,z])),He=new Map;try{let z=await fetch(e.resourceURL("/3d/system-world/v3/manifest.json"),{signal:C.signal}),Ce=z.ok?await z.json():null;if(Ce?.schema_version===3&&Array.isArray(Ce.assets))for(let We of Ce.assets)He.set(We.id,We)}catch(z){if(C.signal.aborted)throw bt(),z}Fm(Z,Re,vn,Qs),ie=Cm(k,{shadows:et,camera:E,districts:vn,tier:a,traffic:Z,reduced:()=>f,active:()=>n&&!_&&i!=="map"&&!f&&!e.replaying?.(),assetURL:z=>e.resourceURL("/3d/system-world/v2/"+z),surfaces:ze,clear:(z,Ce)=>Z.clear(W,{...z,heading:0},{...Ce,heading:0},!1),onFlight:z=>e.onFlight?.(z),onInteraction:e.onInteraction,onScope:z=>e.onScope?.(z),onDiscover:e.onDiscover,onSound:e.onSound,onGait:e.onGait,onTerminal:e.onTerminal,onSociety:e.onSociety,onEnvironment:z=>{he.setIndoor(z),e.onEnvironment?.(z)},onError:e.onError,onReady:()=>{D.shadowMap.needsUpdate=!0}});let ot={v1:{catalog:Re,url:z=>e.assetURL(z),loader:()=>new yi},v3:{catalog:He,url:z=>e.resourceURL("/3d/system-world/v3/"+z),loader:()=>new yi().setMeshoptDecoder(Yr)}};async function Ze(z,Ce,We="v1"){let Qe=We+":"+z+":"+Ce;return P.has(Qe)||P.set(Qe,(async()=>{let at=ot[We],zt=at.catalog.get(z)?.lods.find(lt=>lt.level===Ce);if(!zt||!/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(zt.file))throw Error("Invalid city asset");let Oe=new AbortController;S.add(Oe);let ct=setTimeout(()=>Oe.abort(),15e3);try{let lt=await fetch(at.url(zt.file),{signal:Oe.signal});if(!lt.ok)throw Error("City asset unavailable");let dt=await lt.arrayBuffer();if(t)throw Error("Disposed");We==="v3"&&await Yr.ready;let Pt=await at.loader().parseAsync(dt,"");if(t)throw Pt.scene.traverse(Et=>{Et.isMesh&&(Et.geometry.dispose(),Et.material.dispose())}),Error("Disposed");return p+=dt.byteLength,Ne(),Pt.scene.traverse(Et=>{if(!Et.isMesh)return;F.add(Et.geometry),Et.castShadow=!0,Et.receiveShadow=!0;let Ge=(Array.isArray(Et.material)?Et.material:[Et.material]).map(wt=>{let en=vM(wt);if(M.has(en)){let Lt=wt;wt=M.get(en),Lt.dispose()}else M.set(en,wt),I.add(wt);return wt});Et.material=Array.isArray(Et.material)?Ge:Ge[0],ze.prepare(Et)}),Pt.scene}finally{clearTimeout(ct),S.delete(Oe)}})().catch(at=>{throw P.delete(Qe),at})),P.get(Qe)}function Jt(z){z.traverse(Ce=>{Ce.isInstancedMesh&&Ce.dispose()}),z.clear()}async function Xt(){let z=++o,Ce=Wc[a],We=Ce.lod,Qe=[...new Set([...Qs.map(Oe=>Oe.asset),...vn.map(Oe=>Oe.asset),"service-drone"])],at=[...new Set(Hm.map(Oe=>Oe.asset))],zt=(Oe,ct)=>Re.get(Oe)?.lods.find(lt=>lt.level===ct)?.bytes||0;Fe=Qe.reduce((Oe,ct)=>Oe+(P.has("v1:"+ct+":"+We)?0:zt(ct,We)),p)+(We===2?0:["data-tower-a","data-tower-b"].reduce((Oe,ct)=>Oe+(P.has("v1:"+ct+":2")?0:zt(ct,2)),0))+at.reduce((Oe,ct)=>Oe+(P.has("v3:"+ct+":"+We)?0:He.get(ct)?.lods.find(lt=>lt.level===We)?.bytes||0),0);try{let Oe=new Map(await Promise.all(Qe.map(async Ge=>[Ge,await Ze(Ge,We)]))),ct=new Map(await Promise.all(["data-tower-a","data-tower-b"].map(async Ge=>[Ge,await Ze(Ge,2)]))),lt=new Map(await Promise.all(at.map(async Ge=>[Ge,await Ze(Ge,We,"v3")])));if(t||z!==o)return;Jt(Ke),Jt(it),g=[];let dt=new Map,Pt=(Ge,wt)=>{wt.updateMatrixWorld(!0);let en=new ft().compose(new B(Ge.x,Ge.y,Ge.z),new Qt().setFromAxisAngle(new B(0,1,0),Ge.angle),new B(...Ge.scale));wt.traverse(Lt=>{Lt.isMesh&&(dt.has(Lt.uuid)||dt.set(Lt.uuid,{node:Lt,matrices:[]}),dt.get(Lt.uuid).matrices.push(new ft().multiplyMatrices(en,Lt.matrixWorld)))})};for(let Ge of Qs)Pt(Ge,(Ge.z<-100?ct:Oe).get(Ge.asset));for(let Ge of Hm)Pt(Ge,lt.get(Ge.asset));for(let{node:Ge,matrices:wt}of dt.values()){let en=new Sn(Ge.geometry,Ge.material,wt.length);wt.forEach((Lt,Si)=>en.setMatrixAt(Si,Lt)),en.castShadow=!0,en.receiveShadow=!0,Ke.add(en)}for(let Ge of vn){let wt=Oe.get(Ge.asset).clone(!0);wt.position.set(Ge.x,0,Ge.z),wt.userData.district=Ge.id,it.add(wt),g.push(wt)}A?.attachLandmarks(it),j.setTemplate(Oe.get("service-drone"));let Et=new cn,It=new Map;for(let Ge of["data-tower-a","data-tower-b"])It.set(Ge,Et.setFromObject(ct.get(Ge)).max.y);xe.setAviation(Qs.filter(Ge=>It.has(Ge.asset)&&(Ge.z<-100||Ge.scale[1]>1.1)).map(Ge=>({x:Ge.x,y:Ge.y+It.get(Ge.asset)*Ge.scale[1]+.6,z:Ge.z}))),D.shadowMap.needsUpdate=!0,e.onReady?.()}catch(Oe){!t&&z===o&&e.onError?.(Oe)}}function Mt(){if(t)return;y=Math.max(1,s.clientWidth),T=Math.max(1,s.clientHeight);let z=Math.min(devicePixelRatio||1,Wc[a].dpr,Math.sqrt(3840*2160/(y*T)));D.setPixelRatio(z),D.setSize(y,T,!1),de.setPixelRatio(z),de.setSize(y,T),E.aspect=y/T,E.updateProjectionMatrix(),G.setPointScale(T*z),xe.setPointScale(T*z),Ue.setPointScale(T*z)}function Mn(z){return r=z in Wc||z==="auto"?z:"auto",a=r==="auto"?"high":r,kt(),Xt()}function kt(){let z=Wc[a];D.shadowMap.enabled=z.shadow>0,z.shadow&&ue.shadow.mapSize.x!==z.shadow&&(ue.shadow.mapSize.set(z.shadow,z.shadow),ue.shadow.map?.dispose(),ue.shadow.map=null),D.shadowMap.needsUpdate=!0,we.enabled=z.bloom,ze.setTier(a),xe.setTier(a),ie.setTier(a),he.setTier(a),j.setTier(a),Ue.setTier(a),Mt(),e.onQuality?.(r,a)}function nn(z,Ce){l=null;let We=Z.findFree(W,z);if(!We)return;if(z=new B(We.x,We.y,We.z),f){E.position.copy(z),Object.assign(W,We),Q.target.copy(Ce),Q.update(),l=null;return}let Qe=(ct,lt)=>Z.clear(W,{...ct,heading:0},{...lt,heading:0},!1),at=E.position.clone(),zt=Math.max(at.y,z.y,110),Oe=[at,z];if(!Qe(at,z)){let ct=[at],lt=new B(z.x,zt,z.z),dt=[-1],Pt=[0],Et=-1;if(!Qe(lt,z))return;for(let It of[3,8,16])for(let Ge=0;Ge<8;Ge++)ct.push(new B(at.x+Math.cos(Ge*Math.PI/4)*It,at.y,at.z+Math.sin(Ge*Math.PI/4)*It));for(let It of Dn)if(Math.hypot(at.x-It.x,at.z-It.z)<24)for(let Ge of[It.z,It.doorZ-It.front*2,It.doorZ+It.front*3])ct.push(new B(It.x,at.y,Ge));for(;Pt.length;){let It=Pt.shift(),Ge=ct[It],wt=new B(Ge.x,zt,Ge.z);if(Qe(Ge,wt)&&Qe(wt,lt)){Et=It;break}for(let en=1;en<ct.length;en++)dt[en]===void 0&&Ge.distanceTo(ct[en])<=20&&Qe(Ge,ct[en])&&(dt[en]=It,Pt.push(en))}if(Et<0)return;Oe=[];for(let It=Et;It>=0;It=dt[It])Oe.unshift(ct[It]);Oe.push(new B(ct[Et].x,zt,ct[Et].z),lt,z)}l={start:E.position.clone(),targetStart:Q.target.clone(),end:z.clone(),target:Ce.clone(),time:0,waypoints:Oe}}function Zt(){Z.relocate(W,E.position),E.position.set(W.x,W.y,W.z)}function wn(z){let Ce=vn.find(We=>We.id===z);Ce&&(m=z,Ie.position.set(Ce.x,.55,Ce.z),Ie.scale.setScalar(Ce.radius*1.3),Ie.visible=!0,qe(),Me.position.set(Ce.x,.62,Ce.z),Me.scale.setScalar(Ce.radius*1.3*.35),Ae.uniforms.color.value.setHex(Pe[_e[z]]??9365742),Ae.uniforms.alpha.value=0,fe=0,Me.visible=!f,X.visible=!!L&&L!==m,i==="street"?(ie.endRide(),e.onCut?.(),E.position.set(Ce.x,2.4+Ln(Ce.x,Ce.z+Ce.radius+7),Ce.z+Ce.radius+7),E.lookAt(Ce.x,Ce.height*.4,Ce.z),Zt()):nn(new B(Ce.x+Ce.radius*2.7,Ce.height*.7+18,Ce.z+Ce.radius*4),new B(Ce.x,Ce.height*.4,Ce.z)))}function ai(z){ie.endRide(),ie.society.suspend(),N.clear(),l=null,i=z,Q.enabled=i==="orbit"||i==="tour",W.minY=i==="street"?-2.3:-.4,W.ignore=W.follow=null,W.circles[0].r=W.reach=i==="street"?.24:.38,document.pointerLockElement===V&&document.exitPointerLock(),i==="street"?(e.onCut?.(),me(),E.position.set(18,2.4+Ln(18,57),57),E.lookAt(0,26,-12),Zt()):i!=="map"&&(Zt(),nn(hd.clone().multiplyScalar(E.aspect<1?1.3:1),ta)),x=0,v=0,e.onMode?.(i)}let En=()=>{l=null,i==="tour"&&(i="orbit",e.onMode?.(i))};Q.addEventListener("start",En);function Tn(z,Ce,We,Qe){z.addEventListener(Ce,We,Qe),H.push(()=>z.removeEventListener(Ce,We,Qe))}let Fn=null,na=!1,Es=new kn(0,0,0,"YXZ");function _o(z,Ce){let We=.0025*E.fov/43;Es.setFromQuaternion(E.quaternion),Es.y-=z*We,Es.x=Ft.clamp(Es.x-Ce*We,-1.35,1.35),E.quaternion.setFromEuler(Es)}Tn(V,"pointerdown",z=>{V.focus({preventScroll:!0}),En(),Fn=[z.clientX,z.clientY],na=!1,i==="street"&&V.setPointerCapture(z.pointerId)});let L=null,ee=-1e9;function ye(z){if(z===L)return;L=z,V.style.cursor=z?"pointer":"";let Ce=vn.find(We=>We.id===z);X.visible=!!Ce&&z!==m,Ce&&(X.position.set(Ce.x,.56,Ce.z),X.scale.setScalar(Ce.radius*1.3)),e.onHover?.(z)}function me(){ye(null)}function ge(z){if(i==="street"||i==="map"||Fn||z.timeStamp-ee<80)return;ee=z.timeStamp;let Ce=V.getBoundingClientRect();K.set((z.clientX-Ce.left)/Ce.width*2-1,1-(z.clientY-Ce.top)/Ce.height*2),ae.setFromCamera(K,E);let We=ae.intersectObjects(g,!0)[0]?.object;for(;We&&!We.userData.district;)We=We.parent;ye(We?.userData.district||null)}Tn(V,"pointerleave",me),Tn(V,"pointermove",z=>{i==="street"&&(document.pointerLockElement===V||Fn)&&_o(z.movementX,z.movementY),ge(z),Fn&&Math.hypot(z.clientX-Fn[0],z.clientY-Fn[1])>5&&(na=!0)}),Tn(V,"pointerup",z=>{if(Fn&&!na&&i!=="street"){let Ce=V.getBoundingClientRect();K.set((z.clientX-Ce.left)/Ce.width*2-1,1-(z.clientY-Ce.top)/Ce.height*2),ae.setFromCamera(K,E);let We=ae.intersectObjects(g,!0)[0];if(We){let Qe=We.object;for(;Qe&&!Qe.userData.district;)Qe=Qe.parent;Qe&&e.onSelect?.(Qe.userData.district)}}Fn=null}),Tn(V,"pointercancel",()=>{Fn=null,N.clear()}),Tn(V,"keydown",z=>{if(z.key==="Escape"){ie.scoping()?ie.endRide():ai("orbit"),z.preventDefault();return}if(i==="street"&&["+","=","-"].includes(z.key)&&ie.zoom(z.key==="-"?240:-240)){z.preventDefault();return}if(i==="street"&&z.code==="KeyE"&&!z.repeat){ie.interact(),z.preventDefault();return}i==="street"&&["KeyW","KeyA","KeyS","KeyD","ArrowUp","ArrowDown","ArrowLeft","ArrowRight","ShiftLeft"].includes(z.code)&&(N.add(z.code),z.preventDefault(),z.stopPropagation()),i==="street"&&["Space","KeyC"].includes(z.code)&&!z.ctrlKey&&!z.metaKey&&!z.altKey&&(N.add(z.code),z.preventDefault(),z.stopPropagation())}),Tn(V,"keyup",z=>N.delete(z.code)),Tn(V,"wheel",z=>{i==="street"&&ie.zoom(z.deltaY)&&z.preventDefault()},{passive:!1}),Tn(V,"blur",()=>N.clear()),Tn(window,"blur",()=>{N.clear(),Fn=null}),Tn(V,"webglcontextlost",z=>{z.preventDefault(),_=!0,N.clear(),e.onContextLost?.()}),w=new ResizeObserver(Mt),w.observe(s);let je={y:NaN,v:0};function $e(z){let Ce=(N.has("ShiftLeft")?25:11)*z,We=Number(N.has("KeyW")||N.has("ArrowUp"))-Number(N.has("KeyS")||N.has("ArrowDown")),Qe=Number(N.has("KeyD")||N.has("ArrowRight"))-Number(N.has("KeyA")||N.has("ArrowLeft"));if(ie.hovering()){ie.fly(z,{forward:We,right:Qe,up:Number(N.has("Space"))-Number(N.has("KeyC")),boost:N.has("ShiftLeft")}),je.y=NaN;return}if(We||Qe){if(ie.walkRide(We,z,Qe))return;let at=Ce/Math.hypot(We,Qe);E.getWorldDirection(Y),Y.y=0,Y.normalize();let zt=(Y.x*We-Y.z*Qe)*at,Oe=(Y.z*We+Y.x*Qe)*at,ct=(lt,dt)=>ie.move(lt,dt,E.position)&&Z.clear(W,W,{x:lt,y:2.4+ie.floor(lt,dt),z:dt,heading:0});ct(E.position.x+zt,E.position.z)&&(E.position.x+=zt),ct(E.position.x,E.position.z+Oe)&&(E.position.z+=Oe)}if(ie.isRiding()){je.y=NaN;return}Math.abs(E.position.y-je.y)<1e-6||(je.y=E.position.y,je.v=0),E.position.y=Ss(je,2.4+ie.floor(E.position.x,E.position.z),f?0:z)}let Ye=0,tt=Lp(z=>{Z.begin(),Ye+=f?0:z,i==="street"&&$e(z),ie.update(z,!f,i);let Ce=W.follow;W.follow=ie.rideBody(),W.ignore=W.follow||Ce,A?.update(z,!f),j.update(z,Ye,!f),Z.propose(W,{...E.position,heading:0},(We,Qe)=>E.position.set(Qe.x,Qe.y,Qe.z)),Z.solve(z),ie.settle(f?0:z),ie.syncRide?.(),W.follow&&Object.assign(W,{x:E.position.x,y:E.position.y,z:E.position.z}),W.ignore=W.follow});function nt(z,Ce){if(t||!n||_||i==="map")return;let We=null,Qe=0,at=null;if(i!=="street"){if(i==="tour"&&(x-=z,x<=0)){let dt=vn[v++%vn.length].id;wn(dt),e.onTourFocus?.(dt),x=7}if(l){We=l,Qe=l.time,l.time+=z;let dt=Math.min(1,l.time/(l.duration||(l.waypoints.length===2?1.1:3.2))),Pt=dt*dt*(3-2*dt),Et=Math.min(l.waypoints.length-1-1e-6,Pt*(l.waypoints.length-1)),It=Math.floor(Et);E.position.lerpVectors(l.waypoints[It],l.waypoints[It+1],Et-It),Q.target.lerpVectors(l.targetStart,l.target,Pt),E.lookAt(Q.target),at=E.position.clone(),dt===1&&(l=null)}else Q.update()}tt(Math.min(.1,Math.max(0,z)))||(Z.begin(),Z.propose(W,{...E.position,heading:0},(dt,Pt)=>E.position.set(Pt.x,Pt.y,Pt.z)),Z.solve(0)),We&&(E.position.distanceToSquared(at)>1e-6?(We.wait=(We.wait||0)+z,We.time=Qe,l=We.wait<6?We:null):We.wait=0),E.getWorldDirection(Y);let zt=i!=="street"&&E.position.y>15;e.onListener?.(E.position.x,E.position.y,E.position.z,Y.x,Y.z,zt?Q.target.x:E.position.x,zt?Q.target.z:E.position.z);let Oe=!!e.busy?.();he.update(z,Ce,!f)&&(D.shadowMap.needsUpdate=!0),ce.intensity=f?0:Oe?180+Math.sin(Ce*2)*35:12,xe.setBusy(Oe),xe.setCinematic(i==="tour"),xe.update(z,Ce,E,!f),ze.update(z,!f),b?.update(z,!f&&n&&i!=="map"&&!e.replaying?.());let ct=he.mood();if(Ue.update(z,!f,1-(ct.day||0)),!f){if(fe<1.4){fe+=z;let dt=Math.min(1,fe/1.4),Pt=vn.find(Et=>Et.id===m);Pt&&(Me.scale.setScalar(Pt.radius*1.3*(.35+dt*1.05)),Ae.uniforms.alpha.value=(1-dt)*.55),dt>=1&&(Me.visible=!1)}for(let dt=0;dt<vn.length;dt++){let Pt=vn[dt];q.position.set(Pt.x,Pt.height+2,Pt.z),q.scale.setScalar(1+Math.sin(Ce*1.7+dt*1.13)*.13),q.updateMatrix(),U.setMatrixAt(dt,q.matrix)}U.instanceMatrix.needsUpdate=!0}f||(ve.value+=Math.min(.1,Math.max(0,z)));let lt=[Oe,ct.day.toFixed(1),ct.evening.toFixed(1),ct.weather].join();if(we.strength=.2+.15*(1-ct.day),D.toneMappingExposure=.95+.05*(1-ct.day),lt!==Se&&(Se=lt,e.onMood?.({busy:Oe,day:ct.day,evening:ct.evening,weather:ct.weather})),G.update(z,E,!f),D.info.reset(),de.render(),c++,r==="auto"&&z>0&&z<.2&&Ce-d>12&&(h+=z,u++,u>=180)){let dt=h/u,Pt=["low","medium","high"],Et=Pt.indexOf(a),It=dt>.028&&Et>0?Pt[Et-1]:dt<.017&&Et<2?Pt[Et+1]:a;u=0,h=0,It!==a&&(a=It,d=Ce,kt(),Xt())}}function St(z,Ce){A?.setData(z,Ce),vn.forEach((We,Qe)=>{let at=z.find(zt=>zt.id===We.id);_e[We.id]=!at||at.stale?"stale":at.state==="error"?"error":at.state==="running"?"running":"idle",U.setColorAt(Qe,new Le(Pe[_e[We.id]]))}),U.instanceColor.needsUpdate=!0,qe()}function bt(){t||(t=!0,o++,C.abort(),S.forEach(z=>z.abort()),N.clear(),document.pointerLockElement===V&&document.exitPointerLock(),e.signal?.removeEventListener("abort",bt),H.forEach(z=>z()),w?.disconnect(),Q.dispose(),A?.dispose(),b?.dispose(),ie?.dispose(),he.dispose(),G.dispose(),j.dispose(),et.dispose(),Z.dispose(),xe.dispose(),ze.dispose(),Ue.dispose(),Jt(Ke),Jt(it),U.dispose(),oe.dispose(),F.forEach(z=>z.dispose()),I.forEach(z=>z.dispose()),de.passes.forEach(z=>z.dispose?.()),de.dispose(),De.dispose(),ue.shadow.dispose(),D.dispose(),D.forceContextLoss(),V.remove(),P.clear(),M.clear())}A=jp(k,vn,{traffic:Z,society:ie.society,robotURL:e.resourceURL("/3d/system-world/white-robot.glb"),signal:e.signal,onError:e.onRobotError,active:()=>n&&!_&&i!=="map"&&!e.replaying?.(),obstacles:_M,shadows:et}),b=Dm({scene:k,resourceURL:e.resourceURL,signal:C.signal,shadows:et,onError:e.onCompanionError}),G.setReducedMotion(!!f);try{kt(),await Xt()}catch(z){throw bt(),z}return{districts:vn,canvas:V,update:nt,focus(z){En(),wn(z)},setMode:ai,setQuality:Mn,setData:St,dispose:bt,interact(){ie.interact()},socialAction:ie.socialAction,visit(z){ai("street"),ie.visit(z),Zt()},enter(z){ai("street"),ie.destination(z),Zt()},setEnvironment(z){he.set(z),D.shadowMap.needsUpdate=!0},setVisible(z){n=z,z||(tt(0),ie.suspend(),A?.update(0,!1),N.clear(),Fn=null,document.pointerLockElement===V&&document.exitPointerLock())},setReducedMotion(z){f=z,G.setReducedMotion(!!z),z&&(l=null,ie.endRide(),ie.society.suspend()),z&&i==="tour"&&ai("orbit")},setHologram(z,Ce){G.setTexts(z,Ce),ie.setMemory(z)},setWorld(z,Ce){ie.setWorld(z,Ce)},moveKey(z,Ce){Ce?N.add(z):N.delete(z)},lockPointer(){if(i==="street")return V.requestPointerLock()},project(z){let Ce=vn.find(Qe=>Qe.id===z);if(!Ce)return null;Y.set(Ce.x,Ce.height+4,Ce.z);let We=E.position.distanceTo(Y);return Y.project(E),{x:(Y.x+1)*y/2,y:(1-Y.y)*T/2,visible:Y.z<1&&Y.z>-1,distance:We}},capture(){return t||_?Promise.resolve(null):(de.render(),new Promise(z=>V.toBlob(z,"image/png")))},intro(){if(f||i!=="orbit"||t)return;let z=hd.clone().multiplyScalar(E.aspect<1?1.3:1);E.position.copy(z).sub(ta).applyAxisAngle(new B(0,1,0),-.55).multiplyScalar(1.22).add(ta),E.position.y+=34,E.lookAt(ta),Zt(),nn(z,ta),l&&(l.duration=3.6)},stats(){return{flying:!!l,traffic:Z.stats(),experience:ie.stats(),weather:he.stats(),life:A?.stats(),hologram:G.stats(),atmosphere:xe.stats(),surfaces:ze.stats(),drones:j.stats(),flow:Ue.stats(),companion:b?.stats(),frames:c,tier:a,mode:i,focusedDistrict:m,hovered:L,loadedBytes:p+ie.stats().bytes+ze.stats().bytes,cachedModels:P.size,calls:D.info.render.calls,triangles:D.info.render.triangles,geometries:D.info.memory.geometries,position:E.position.toArray(),renderer:D.getContext().getParameter(D.getContext().getExtension("WEBGL_debug_renderer_info")?.UNMASKED_RENDERER_WEBGL||D.getContext().RENDERER),disposed:t}}}}export{hT as createCity,bM as createCityAmbience,vn as districts,Hm as kitPlacements,_M as obstacles,Qs as placements,vM as sharedMaterialKey};
/*! Bundled license information:

three/build/three.core.js:
three/build/three.module.js:
  (**
   * @license
   * Copyright 2010-2026 Three.js Authors
   * SPDX-License-Identifier: MIT
   *)
*/
