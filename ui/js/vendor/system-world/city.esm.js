var cs={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2},hs={ROTATE:0,PAN:1,DOLLY_PAN:2,DOLLY_ROTATE:3},Ld=0,mh=1,Fd=2;var Ns=1,Nd=2,Ir=3,jn=0,on=1,Ot=2,Kn=0,Dr=1,qt=2,gh=3,bh=4,Ud=5;var Us=100,Od=101,zd=102,Bd=103,kd=104,Hd=200,Gd=201,Vd=202,Wd=203,xh=204,vh=205,qd=206,Xd=207,jd=208,Kd=209,Yd=210,Jd=211,Zd=212,$d=213,Qd=214,Bo=0,ko=1,Ho=2,hr=3,Go=4,Vo=5,Wo=6,qo=7,bl=0,ef=1,tf=2,ai=0,Ua=1,Oa=2,za=3,Os=4,Ba=5,ka=6,Ha=7,sh="attached",nf="detached",_h=300,us=301,zs=302,xl=303,vl=304,Ga=306,pi=1e3,qn=1001,ur=1002,$t=1003,_l=1004;var Bs=1005;var jt=1006,Lr=1007;var oi=1008;var In=1009,yh=1010,Mh=1011,Fr=1012,yl=1013,li=1014,kn=1015,rn=1016,Ml=1017,Sl=1018,Nr=1020,Sh=35902,wh=35899,Eh=1021,Th=1022,Hn=1023,mi=1026,ds=1027,wl=1028,El=1029,fs=1030,Tl=1031;var Al=1033,Va=33776,Wa=33777,qa=33778,Xa=33779,Rl=35840,Cl=35841,Pl=35842,Il=35843,Dl=36196,Ll=37492,Fl=37496,Nl=37488,Ul=37489,ja=37490,Ol=37491,zl=37808,Bl=37809,kl=37810,Hl=37811,Gl=37812,Vl=37813,Wl=37814,ql=37815,Xl=37816,jl=37817,Kl=37818,Yl=37819,Jl=37820,Zl=37821,$l=36492,Ql=36494,ec=36495,tc=36283,nc=36284,Ka=36285,ic=36286,sc=2200,sf=2201,rf=2202,Ts=2300,As=2301,Uo=2302,rh=2303,Ss=2400,ws=2401,la=2402,rc=2500,af=2501,Ah=0,Ya=1,Ur=2,of=3200;var Ja=0,lf=1,Yn="",Xt="srgb",wn="srgb-linear",ca="linear",Dt="srgb";var Oo=7680;var cf=519,hf=512,uf=513,df=514,ac=515,ff=516,pf=517,oc=518,mf=519,Rh=35044;var Ch="300 es",ni=2e3,dr=2001;function nm(s){for(let e=s.length-1;e>=0;--e)if(s[e]>=65535)return!0;return!1}function im(s){return ArrayBuffer.isView(s)&&!(s instanceof DataView)}function fr(s){return document.createElementNS("http://www.w3.org/1999/xhtml",s)}function gf(){let s=fr("canvas");return s.style.display="block",s}var Wu={},pr=null;function ha(...s){let e="THREE."+s.shift();pr?pr("log",e,...s):console.log(e,...s)}function bf(s){let e=s[0];if(typeof e=="string"&&e.startsWith("TSL:")){let t=s[1];t&&t.isStackTrace?s[0]+=" "+t.getLocation():s[1]='Stack trace not available. Enable "THREE.Node.captureStackTrace" to capture stack traces.'}return s}function nt(...s){s=bf(s);let e="THREE."+s.shift();if(pr)pr("warn",e,...s);else{let t=s[0];t&&t.isStackTrace?console.warn(t.getError(e)):console.warn(e,...s)}}function ht(...s){s=bf(s);let e="THREE."+s.shift();if(pr)pr("error",e,...s);else{let t=s[0];t&&t.isStackTrace?console.error(t.getError(e)):console.error(e,...s)}}function Es(...s){let e=s.join(" ");e in Wu||(Wu[e]=!0,nt(...s))}function xf(s,e,t){return new Promise(function(n,i){function r(){switch(s.clientWaitSync(e,s.SYNC_FLUSH_COMMANDS_BIT,0)){case s.WAIT_FAILED:i();break;case s.TIMEOUT_EXPIRED:setTimeout(r,t);break;default:n()}}setTimeout(r,t)})}var vf={[Bo]:ko,[Ho]:Wo,[Go]:qo,[hr]:Vo,[ko]:Bo,[Wo]:Ho,[qo]:Go,[Vo]:hr},Xn=class{addEventListener(e,t){this._listeners===void 0&&(this._listeners={});let n=this._listeners;n[e]===void 0&&(n[e]=[]),n[e].indexOf(t)===-1&&n[e].push(t)}hasEventListener(e,t){let n=this._listeners;return n===void 0?!1:n[e]!==void 0&&n[e].indexOf(t)!==-1}removeEventListener(e,t){let n=this._listeners;if(n===void 0)return;let i=n[e];if(i!==void 0){let r=i.indexOf(t);r!==-1&&i.splice(r,1)}}dispatchEvent(e){let t=this._listeners;if(t===void 0)return;let n=t[e.type];if(n!==void 0){e.target=this;let i=n.slice(0);for(let r=0,a=i.length;r<a;r++)i[r].call(this,e);e.target=null}}},gn=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],qu=1234567,sa=Math.PI/180,Rs=180/Math.PI;function ii(){let s=Math.random()*4294967295|0,e=Math.random()*4294967295|0,t=Math.random()*4294967295|0,n=Math.random()*4294967295|0;return(gn[s&255]+gn[s>>8&255]+gn[s>>16&255]+gn[s>>24&255]+"-"+gn[e&255]+gn[e>>8&255]+"-"+gn[e>>16&15|64]+gn[e>>24&255]+"-"+gn[t&63|128]+gn[t>>8&255]+"-"+gn[t>>16&255]+gn[t>>24&255]+gn[n&255]+gn[n>>8&255]+gn[n>>16&255]+gn[n>>24&255]).toLowerCase()}function yt(s,e,t){return Math.max(e,Math.min(t,s))}function Ph(s,e){return(s%e+e)%e}function sm(s,e,t,n,i){return n+(s-e)*(i-n)/(t-e)}function rm(s,e,t){return s!==e?(t-s)/(e-s):0}function ra(s,e,t){return(1-t)*s+t*e}function am(s,e,t,n){return ra(s,e,1-Math.exp(-t*n))}function om(s,e=1){return e-Math.abs(Ph(s,e*2)-e)}function lm(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*(3-2*s))}function cm(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*s*(s*(s*6-15)+10))}function hm(s,e){return s+Math.floor(Math.random()*(e-s+1))}function um(s,e){return s+Math.random()*(e-s)}function dm(s){return s*(.5-Math.random())}function fm(s){s!==void 0&&(qu=s);let e=qu+=1831565813;return e=Math.imul(e^e>>>15,e|1),e^=e+Math.imul(e^e>>>7,e|61),((e^e>>>14)>>>0)/4294967296}function pm(s){return s*sa}function mm(s){return s*Rs}function gm(s){return s>0&&Number.isInteger(s)&&2**Math.round(Math.log2(s))===s}function bm(s){return Math.pow(2,Math.ceil(Math.log(s)/Math.LN2))}function xm(s){return Math.pow(2,Math.floor(Math.log(s)/Math.LN2))}function vm(s,e,t,n,i){let r=Math.cos,a=Math.sin,o=r(t/2),l=a(t/2),c=r((e+n)/2),h=a((e+n)/2),u=r((e-n)/2),d=a((e-n)/2),p=r((n-e)/2),g=a((n-e)/2);switch(i){case"XYX":s.set(o*h,l*u,l*d,o*c);break;case"YZY":s.set(l*d,o*h,l*u,o*c);break;case"ZXZ":s.set(l*u,l*d,o*h,o*c);break;case"XZX":s.set(o*h,l*g,l*p,o*c);break;case"YXY":s.set(l*p,o*h,l*g,o*c);break;case"ZYZ":s.set(l*g,l*p,o*h,o*c);break;default:nt("MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+i)}}function ti(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return s/4294967295;case Uint16Array:return s/65535;case Uint8Array:case Uint8ClampedArray:return s/255;case Int32Array:return Math.max(s/2147483647,-1);case Int16Array:return Math.max(s/32767,-1);case Int8Array:return Math.max(s/127,-1);default:throw new Error("THREE.MathUtils: Invalid component type.")}}function Ut(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return Math.round(s*4294967295);case Uint16Array:return Math.round(s*65535);case Uint8Array:case Uint8ClampedArray:return Math.round(s*255);case Int32Array:return Math.round(s*2147483647);case Int16Array:return Math.round(s*32767);case Int8Array:return Math.round(s*127);default:throw new Error("THREE.MathUtils: Invalid component type.")}}var Tt={DEG2RAD:sa,RAD2DEG:Rs,generateUUID:ii,clamp:yt,euclideanModulo:Ph,mapLinear:sm,inverseLerp:rm,lerp:ra,damp:am,pingpong:om,smoothstep:lm,smootherstep:cm,randInt:hm,randFloat:um,randFloatSpread:dm,seededRandom:fm,degToRad:pm,radToDeg:mm,isPowerOfTwo:gm,ceilPowerOfTwo:bm,floorPowerOfTwo:xm,setQuaternionFromProperEuler:vm,normalize:Ut,denormalize:ti},Be=class s{static{s.prototype.isVector2=!0}constructor(e=0,t=0){this.x=e,this.y=t}get width(){return this.x}set width(e){this.x=e}get height(){return this.y}set height(e){this.y=e}set(e,t){return this.x=e,this.y=t,this}setScalar(e){return this.x=e,this.y=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;default:throw new Error("THREE.Vector2: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;default:throw new Error("THREE.Vector2: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y)}copy(e){return this.x=e.x,this.y=e.y,this}add(e){return this.x+=e.x,this.y+=e.y,this}addScalar(e){return this.x+=e,this.y+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this}subScalar(e){return this.x-=e,this.y-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this}multiply(e){return this.x*=e.x,this.y*=e.y,this}multiplyScalar(e){return this.x*=e,this.y*=e,this}divide(e){return this.x/=e.x,this.y/=e.y,this}divideScalar(e){return this.multiplyScalar(1/e)}applyMatrix3(e){let t=this.x,n=this.y,i=e.elements;return this.x=i[0]*t+i[3]*n+i[6],this.y=i[1]*t+i[4]*n+i[7],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this}clamp(e,t){return this.x=yt(this.x,e.x,t.x),this.y=yt(this.y,e.y,t.y),this}clampScalar(e,t){return this.x=yt(this.x,e,t),this.y=yt(this.y,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(yt(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(e){return this.x*e.x+this.y*e.y}cross(e){return this.x*e.y-this.y*e.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(yt(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y;return t*t+n*n}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this}equals(e){return e.x===this.x&&e.y===this.y}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this}rotateAround(e,t){let n=Math.cos(t),i=Math.sin(t),r=this.x-e.x,a=this.y-e.y;return this.x=r*n-a*i+e.x,this.y=r*i+a*n+e.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},Zt=class{constructor(e=0,t=0,n=0,i=1){this.isQuaternion=!0,this._x=e,this._y=t,this._z=n,this._w=i}static slerpFlat(e,t,n,i,r,a,o){let l=n[i+0],c=n[i+1],h=n[i+2],u=n[i+3],d=r[a+0],p=r[a+1],g=r[a+2],_=r[a+3];if(u!==_||l!==d||c!==p||h!==g){let m=l*d+c*p+h*g+u*_;m<0&&(d=-d,p=-p,g=-g,_=-_,m=-m);let f=1-o;if(m<.9995){let v=Math.acos(m),S=Math.sin(v);f=Math.sin(f*v)/S,o=Math.sin(o*v)/S,l=l*f+d*o,c=c*f+p*o,h=h*f+g*o,u=u*f+_*o}else{l=l*f+d*o,c=c*f+p*o,h=h*f+g*o,u=u*f+_*o;let v=1/Math.sqrt(l*l+c*c+h*h+u*u);l*=v,c*=v,h*=v,u*=v}}e[t]=l,e[t+1]=c,e[t+2]=h,e[t+3]=u}static multiplyQuaternionsFlat(e,t,n,i,r,a){let o=n[i],l=n[i+1],c=n[i+2],h=n[i+3],u=r[a],d=r[a+1],p=r[a+2],g=r[a+3];return e[t]=o*g+h*u+l*p-c*d,e[t+1]=l*g+h*d+c*u-o*p,e[t+2]=c*g+h*p+o*d-l*u,e[t+3]=h*g-o*u-l*d-c*p,e}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get w(){return this._w}set w(e){this._w=e,this._onChangeCallback()}set(e,t,n,i){return this._x=e,this._y=t,this._z=n,this._w=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(e){return this._x=e.x,this._y=e.y,this._z=e.z,this._w=e.w,this._onChangeCallback(),this}setFromEuler(e,t=!0){let n=e._x,i=e._y,r=e._z,a=e._order,o=Math.cos,l=Math.sin,c=o(n/2),h=o(i/2),u=o(r/2),d=l(n/2),p=l(i/2),g=l(r/2);switch(a){case"XYZ":this._x=d*h*u+c*p*g,this._y=c*p*u-d*h*g,this._z=c*h*g+d*p*u,this._w=c*h*u-d*p*g;break;case"YXZ":this._x=d*h*u+c*p*g,this._y=c*p*u-d*h*g,this._z=c*h*g-d*p*u,this._w=c*h*u+d*p*g;break;case"ZXY":this._x=d*h*u-c*p*g,this._y=c*p*u+d*h*g,this._z=c*h*g+d*p*u,this._w=c*h*u-d*p*g;break;case"ZYX":this._x=d*h*u-c*p*g,this._y=c*p*u+d*h*g,this._z=c*h*g-d*p*u,this._w=c*h*u+d*p*g;break;case"YZX":this._x=d*h*u+c*p*g,this._y=c*p*u+d*h*g,this._z=c*h*g-d*p*u,this._w=c*h*u-d*p*g;break;case"XZY":this._x=d*h*u-c*p*g,this._y=c*p*u-d*h*g,this._z=c*h*g+d*p*u,this._w=c*h*u+d*p*g;break;default:nt("Quaternion: .setFromEuler() encountered an unknown order: "+a)}return t===!0&&this._onChangeCallback(),this}setFromAxisAngle(e,t){let n=t/2,i=Math.sin(n);return this._x=e.x*i,this._y=e.y*i,this._z=e.z*i,this._w=Math.cos(n),this._onChangeCallback(),this}setFromRotationMatrix(e){let t=e.elements,n=t[0],i=t[4],r=t[8],a=t[1],o=t[5],l=t[9],c=t[2],h=t[6],u=t[10],d=n+o+u;if(d>0){let p=.5/Math.sqrt(d+1);this._w=.25/p,this._x=(h-l)*p,this._y=(r-c)*p,this._z=(a-i)*p}else if(n>o&&n>u){let p=2*Math.sqrt(1+n-o-u);this._w=(h-l)/p,this._x=.25*p,this._y=(i+a)/p,this._z=(r+c)/p}else if(o>u){let p=2*Math.sqrt(1+o-n-u);this._w=(r-c)/p,this._x=(i+a)/p,this._y=.25*p,this._z=(l+h)/p}else{let p=2*Math.sqrt(1+u-n-o);this._w=(a-i)/p,this._x=(r+c)/p,this._y=(l+h)/p,this._z=.25*p}return this._onChangeCallback(),this}setFromUnitVectors(e,t){let n=e.dot(t)+1;return n<1e-8?(n=0,Math.abs(e.x)>Math.abs(e.z)?(this._x=-e.y,this._y=e.x,this._z=0,this._w=n):(this._x=0,this._y=-e.z,this._z=e.y,this._w=n)):(this._x=e.y*t.z-e.z*t.y,this._y=e.z*t.x-e.x*t.z,this._z=e.x*t.y-e.y*t.x,this._w=n),this.normalize()}angleTo(e){return 2*Math.acos(Math.abs(yt(this.dot(e),-1,1)))}rotateTowards(e,t){let n=this.angleTo(e);if(n===0)return this;let i=Math.min(1,t/n);return this.slerp(e,i),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(e){return this._x*e._x+this._y*e._y+this._z*e._z+this._w*e._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let e=this.length();return e===0?(this._x=0,this._y=0,this._z=0,this._w=1):(e=1/e,this._x=this._x*e,this._y=this._y*e,this._z=this._z*e,this._w=this._w*e),this._onChangeCallback(),this}multiply(e){return this.multiplyQuaternions(this,e)}premultiply(e){return this.multiplyQuaternions(e,this)}multiplyQuaternions(e,t){let n=e._x,i=e._y,r=e._z,a=e._w,o=t._x,l=t._y,c=t._z,h=t._w;return this._x=n*h+a*o+i*c-r*l,this._y=i*h+a*l+r*o-n*c,this._z=r*h+a*c+n*l-i*o,this._w=a*h-n*o-i*l-r*c,this._onChangeCallback(),this}slerp(e,t){let n=e._x,i=e._y,r=e._z,a=e._w,o=this.dot(e);o<0&&(n=-n,i=-i,r=-r,a=-a,o=-o);let l=1-t;if(o<.9995){let c=Math.acos(o),h=Math.sin(c);l=Math.sin(l*c)/h,t=Math.sin(t*c)/h,this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+a*t,this._onChangeCallback()}else this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+a*t,this.normalize();return this}slerpQuaternions(e,t,n){return this.copy(e).slerp(t,n)}random(){let e=2*Math.PI*Math.random(),t=2*Math.PI*Math.random(),n=Math.random(),i=Math.sqrt(1-n),r=Math.sqrt(n);return this.set(i*Math.sin(e),i*Math.cos(e),r*Math.sin(t),r*Math.cos(t))}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._w===this._w}fromArray(e,t=0){return this._x=e[t],this._y=e[t+1],this._z=e[t+2],this._w=e[t+3],this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._w,e}fromBufferAttribute(e,t){return this._x=e.getX(t),this._y=e.getY(t),this._z=e.getZ(t),this._w=e.getW(t),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},k=class s{static{s.prototype.isVector3=!0}constructor(e=0,t=0,n=0){this.x=e,this.y=t,this.z=n}set(e,t,n){return n===void 0&&(n=this.z),this.x=e,this.y=t,this.z=n,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;default:throw new Error("THREE.Vector3: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("THREE.Vector3: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this}multiplyVectors(e,t){return this.x=e.x*t.x,this.y=e.y*t.y,this.z=e.z*t.z,this}applyEuler(e){return this.applyQuaternion(Xu.setFromEuler(e))}applyAxisAngle(e,t){return this.applyQuaternion(Xu.setFromAxisAngle(e,t))}applyMatrix3(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[3]*n+r[6]*i,this.y=r[1]*t+r[4]*n+r[7]*i,this.z=r[2]*t+r[5]*n+r[8]*i,this}applyNormalMatrix(e){return this.applyMatrix3(e).normalize()}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=e.elements,a=1/(r[3]*t+r[7]*n+r[11]*i+r[15]);return this.x=(r[0]*t+r[4]*n+r[8]*i+r[12])*a,this.y=(r[1]*t+r[5]*n+r[9]*i+r[13])*a,this.z=(r[2]*t+r[6]*n+r[10]*i+r[14])*a,this}applyQuaternion(e){let t=this.x,n=this.y,i=this.z,r=e.x,a=e.y,o=e.z,l=e.w,c=2*(a*i-o*n),h=2*(o*t-r*i),u=2*(r*n-a*t);return this.x=t+l*c+a*u-o*h,this.y=n+l*h+o*c-r*u,this.z=i+l*u+r*h-a*c,this}project(e){return this.applyMatrix4(e.matrixWorldInverse).applyMatrix4(e.projectionMatrix)}unproject(e){return this.applyMatrix4(e.projectionMatrixInverse).applyMatrix4(e.matrixWorld)}transformDirection(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[4]*n+r[8]*i,this.y=r[1]*t+r[5]*n+r[9]*i,this.z=r[2]*t+r[6]*n+r[10]*i,this.normalize()}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this}divideScalar(e){return this.multiplyScalar(1/e)}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this}clamp(e,t){return this.x=yt(this.x,e.x,t.x),this.y=yt(this.y,e.y,t.y),this.z=yt(this.z,e.z,t.z),this}clampScalar(e,t){return this.x=yt(this.x,e,t),this.y=yt(this.y,e,t),this.z=yt(this.z,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(yt(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this}cross(e){return this.crossVectors(this,e)}crossVectors(e,t){let n=e.x,i=e.y,r=e.z,a=t.x,o=t.y,l=t.z;return this.x=i*l-r*o,this.y=r*a-n*l,this.z=n*o-i*a,this}projectOnVector(e){let t=e.lengthSq();if(t===0)return this.set(0,0,0);let n=e.dot(this)/t;return this.copy(e).multiplyScalar(n)}projectOnPlane(e){return Ic.copy(this).projectOnVector(e),this.sub(Ic)}reflect(e){return this.sub(Ic.copy(e).multiplyScalar(2*this.dot(e)))}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(yt(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y,i=this.z-e.z;return t*t+n*n+i*i}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)+Math.abs(this.z-e.z)}setFromSpherical(e){return this.setFromSphericalCoords(e.radius,e.phi,e.theta)}setFromSphericalCoords(e,t,n){let i=Math.sin(t)*e;return this.x=i*Math.sin(n),this.y=Math.cos(t)*e,this.z=i*Math.cos(n),this}setFromCylindrical(e){return this.setFromCylindricalCoords(e.radius,e.theta,e.y)}setFromCylindricalCoords(e,t,n){return this.x=e*Math.sin(t),this.y=n,this.z=e*Math.cos(t),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this}setFromMatrixScale(e){let t=this.setFromMatrixColumn(e,0).length(),n=this.setFromMatrixColumn(e,1).length(),i=this.setFromMatrixColumn(e,2).length();return this.x=t,this.y=n,this.z=i,this}setFromMatrixColumn(e,t){return this.fromArray(e.elements,t*4)}setFromMatrix3Column(e,t){return this.fromArray(e.elements,t*3)}setFromEuler(e){return this.x=e._x,this.y=e._y,this.z=e._z,this}setFromColor(e){return this.x=e.r,this.y=e.g,this.z=e.b,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let e=Math.random()*Math.PI*2,t=Math.random()*2-1,n=Math.sqrt(1-t*t);return this.x=n*Math.cos(e),this.y=t,this.z=n*Math.sin(e),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},Ic=new k,Xu=new Zt,mt=class s{static{s.prototype.isMatrix3=!0}constructor(e,t,n,i,r,a,o,l,c){this.elements=[1,0,0,0,1,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,a,o,l,c)}set(e,t,n,i,r,a,o,l,c){let h=this.elements;return h[0]=e,h[1]=i,h[2]=o,h[3]=t,h[4]=r,h[5]=l,h[6]=n,h[7]=a,h[8]=c,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],this}extractBasis(e,t,n){return e.setFromMatrix3Column(this,0),t.setFromMatrix3Column(this,1),n.setFromMatrix3Column(this,2),this}setFromMatrix4(e){let t=e.elements;return this.set(t[0],t[4],t[8],t[1],t[5],t[9],t[2],t[6],t[10]),this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,a=n[0],o=n[3],l=n[6],c=n[1],h=n[4],u=n[7],d=n[2],p=n[5],g=n[8],_=i[0],m=i[3],f=i[6],v=i[1],S=i[4],b=i[7],M=i[2],E=i[5],P=i[8];return r[0]=a*_+o*v+l*M,r[3]=a*m+o*S+l*E,r[6]=a*f+o*b+l*P,r[1]=c*_+h*v+u*M,r[4]=c*m+h*S+u*E,r[7]=c*f+h*b+u*P,r[2]=d*_+p*v+g*M,r[5]=d*m+p*S+g*E,r[8]=d*f+p*b+g*P,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[3]*=e,t[6]*=e,t[1]*=e,t[4]*=e,t[7]*=e,t[2]*=e,t[5]*=e,t[8]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8];return t*a*h-t*o*c-n*r*h+n*o*l+i*r*c-i*a*l}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8],u=h*a-o*c,d=o*l-h*r,p=c*r-a*l,g=t*u+n*d+i*p;if(g===0)return this.set(0,0,0,0,0,0,0,0,0);let _=1/g;return e[0]=u*_,e[1]=(i*c-h*n)*_,e[2]=(o*n-i*a)*_,e[3]=d*_,e[4]=(h*t-i*l)*_,e[5]=(i*r-o*t)*_,e[6]=p*_,e[7]=(n*l-c*t)*_,e[8]=(a*t-n*r)*_,this}transpose(){let e,t=this.elements;return e=t[1],t[1]=t[3],t[3]=e,e=t[2],t[2]=t[6],t[6]=e,e=t[5],t[5]=t[7],t[7]=e,this}getNormalMatrix(e){return this.setFromMatrix4(e).invert().transpose()}transposeIntoArray(e){let t=this.elements;return e[0]=t[0],e[1]=t[3],e[2]=t[6],e[3]=t[1],e[4]=t[4],e[5]=t[7],e[6]=t[2],e[7]=t[5],e[8]=t[8],this}setUvTransform(e,t,n,i,r,a,o){let l=Math.cos(r),c=Math.sin(r);return this.set(n*l,n*c,-n*(l*a+c*o)+a+e,-i*c,i*l,-i*(-c*a+l*o)+o+t,0,0,1),this}scale(e,t){return Es("Matrix3: .scale() is deprecated. Use .makeScale() instead."),this.premultiply(Dc.makeScale(e,t)),this}rotate(e){return Es("Matrix3: .rotate() is deprecated. Use .makeRotation() instead."),this.premultiply(Dc.makeRotation(-e)),this}translate(e,t){return Es("Matrix3: .translate() is deprecated. Use .makeTranslation() instead."),this.premultiply(Dc.makeTranslation(e,t)),this}makeTranslation(e,t){return e.isVector2?this.set(1,0,e.x,0,1,e.y,0,0,1):this.set(1,0,e,0,1,t,0,0,1),this}makeRotation(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,n,t,0,0,0,1),this}makeScale(e,t){return this.set(e,0,0,0,t,0,0,0,1),this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<9;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<9;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e}clone(){return new this.constructor().fromArray(this.elements)}},Dc=new mt,ju=new mt().set(.4123908,.3575843,.1804808,.212639,.7151687,.0721923,.0193308,.1191948,.9505322),Ku=new mt().set(3.2409699,-1.5373832,-.4986108,-.9692436,1.8759675,.0415551,.0556301,-.203977,1.0569715);function _m(){let s={enabled:!0,workingColorSpace:wn,spaces:{},convert:function(i,r,a){return this.enabled===!1||r===a||!r||!a||(this.spaces[r].transfer===Dt&&(i.r=Ni(i.r),i.g=Ni(i.g),i.b=Ni(i.b)),this.spaces[r].primaries!==this.spaces[a].primaries&&(i.applyMatrix3(this.spaces[r].toXYZ),i.applyMatrix3(this.spaces[a].fromXYZ)),this.spaces[a].transfer===Dt&&(i.r=cr(i.r),i.g=cr(i.g),i.b=cr(i.b))),i},workingToColorSpace:function(i,r){return this.convert(i,this.workingColorSpace,r)},colorSpaceToWorking:function(i,r){return this.convert(i,r,this.workingColorSpace)},getPrimaries:function(i){return this.spaces[i].primaries},getTransfer:function(i){return i===Yn?ca:this.spaces[i].transfer},getToneMappingMode:function(i){return this.spaces[i].outputColorSpaceConfig.toneMappingMode||"standard"},getLuminanceCoefficients:function(i,r=this.workingColorSpace){return i.fromArray(this.spaces[r].luminanceCoefficients)},define:function(i){Object.assign(this.spaces,i)},_getMatrix:function(i,r,a){return i.copy(this.spaces[r].toXYZ).multiply(this.spaces[a].fromXYZ)},_getDrawingBufferColorSpace:function(i){return this.spaces[i].outputColorSpaceConfig.drawingBufferColorSpace},_getUnpackColorSpace:function(i=this.workingColorSpace){return this.spaces[i].workingColorSpaceConfig.unpackColorSpace},fromWorkingColorSpace:function(i,r){return Es("ColorManagement: .fromWorkingColorSpace() has been renamed to .workingToColorSpace()."),s.workingToColorSpace(i,r)},toWorkingColorSpace:function(i,r){return Es("ColorManagement: .toWorkingColorSpace() has been renamed to .colorSpaceToWorking()."),s.colorSpaceToWorking(i,r)}},e=[.64,.33,.3,.6,.15,.06],t=[.2126,.7152,.0722],n=[.3127,.329];return s.define({[wn]:{primaries:e,whitePoint:n,transfer:ca,toXYZ:ju,fromXYZ:Ku,luminanceCoefficients:t,workingColorSpaceConfig:{unpackColorSpace:Xt},outputColorSpaceConfig:{drawingBufferColorSpace:Xt}},[Xt]:{primaries:e,whitePoint:n,transfer:Dt,toXYZ:ju,fromXYZ:Ku,luminanceCoefficients:t,outputColorSpaceConfig:{drawingBufferColorSpace:Xt}}}),s}var _t=_m();function Ni(s){return s<.04045?s*.0773993808:Math.pow(s*.9478672986+.0521327014,2.4)}function cr(s){return s<.0031308?s*12.92:1.055*Math.pow(s,.41666)-.055}var Ys,Xo=class{static getDataURL(e,t="image/png"){if(/^data:/i.test(e.src)||typeof HTMLCanvasElement>"u")return e.src;let n;if(e instanceof HTMLCanvasElement)n=e;else{Ys===void 0&&(Ys=fr("canvas")),Ys.width=e.width,Ys.height=e.height;let i=Ys.getContext("2d");e instanceof ImageData?i.putImageData(e,0,0):i.drawImage(e,0,0,e.width,e.height),n=Ys}return n.toDataURL(t)}static sRGBToLinear(e){if(typeof HTMLImageElement<"u"&&e instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&e instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&e instanceof ImageBitmap){let t=fr("canvas");t.width=e.width,t.height=e.height;let n=t.getContext("2d");n.drawImage(e,0,0,e.width,e.height);let i=n.getImageData(0,0,e.width,e.height),r=i.data;for(let a=0;a<r.length;a++)r[a]=Ni(r[a]/255)*255;return n.putImageData(i,0,0),t}else if(e.data){let t=e.data.slice(0);for(let n=0;n<t.length;n++)t instanceof Uint8Array||t instanceof Uint8ClampedArray?t[n]=Math.floor(Ni(t[n]/255)*255):t[n]=Ni(t[n]);return{data:t,width:e.width,height:e.height}}else return nt("ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),e}},ym=0,mr=class{constructor(e=null){this.isTextureSource=!0,Object.defineProperty(this,"id",{value:ym++}),this.uuid=ii(),this.data=e,this.dataReady=!0,this.version=0}getSize(e){let t=this.data;return typeof HTMLVideoElement<"u"&&t instanceof HTMLVideoElement?e.set(t.videoWidth,t.videoHeight,0):typeof VideoFrame<"u"&&t instanceof VideoFrame?e.set(t.displayWidth,t.displayHeight,0):t!==null?e.set(t.width,t.height,t.depth||0):e.set(0,0,0),e}set needsUpdate(e){e===!0&&this.version++}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.images[this.uuid]!==void 0)return e.images[this.uuid];let n={uuid:this.uuid,url:""},i=this.data;if(i!==null){let r;if(Array.isArray(i)){r=[];for(let a=0,o=i.length;a<o;a++)i[a].isDataTexture?r.push(Lc(i[a].image)):r.push(Lc(i[a]))}else r=Lc(i);n.url=r}return t||(e.images[this.uuid]=n),n}};function Lc(s){return typeof HTMLImageElement<"u"&&s instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&s instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&s instanceof ImageBitmap?Xo.getDataURL(s):s.data?{data:Array.from(s.data),width:s.width,height:s.height,type:s.data.constructor.name}:(nt("Texture: Unable to serialize Texture."),{})}var Mm=0,Fc=new k,Qt=class s extends Xn{constructor(e=s.DEFAULT_IMAGE,t=s.DEFAULT_MAPPING,n=qn,i=qn,r=jt,a=oi,o=Hn,l=In,c=s.DEFAULT_ANISOTROPY,h=Yn){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:Mm++}),this.uuid=ii(),this.name="",this.source=new mr(e),this.mipmaps=[],this.mapping=t,this.channel=0,this.wrapS=n,this.wrapT=i,this.magFilter=r,this.minFilter=a,this.anisotropy=c,this.format=o,this.internalFormat=null,this.type=l,this.offset=new Be(0,0),this.repeat=new Be(1,1),this.center=new Be(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new mt,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=h,this.userData={},this.updateRanges=[],this.version=0,this.onUpdate=null,this.renderTarget=null,this.isRenderTargetTexture=!1,this.isArrayTexture=!!(e&&e.depth&&e.depth>1),this.pmremVersion=0,this.normalized=!1}get width(){return this.source.getSize(Fc).x}get height(){return this.source.getSize(Fc).y}get depth(){return this.source.getSize(Fc).z}get image(){return this.source.data}set image(e){this.source.data=e}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}clone(){return new this.constructor().copy(this)}copy(e){return this.name=e.name,this.source=e.source,this.mipmaps=e.mipmaps.slice(0),this.mapping=e.mapping,this.channel=e.channel,this.wrapS=e.wrapS,this.wrapT=e.wrapT,this.magFilter=e.magFilter,this.minFilter=e.minFilter,this.anisotropy=e.anisotropy,this.format=e.format,this.internalFormat=e.internalFormat,this.type=e.type,this.normalized=e.normalized,this.offset.copy(e.offset),this.repeat.copy(e.repeat),this.center.copy(e.center),this.rotation=e.rotation,this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrix.copy(e.matrix),this.generateMipmaps=e.generateMipmaps,this.premultiplyAlpha=e.premultiplyAlpha,this.flipY=e.flipY,this.unpackAlignment=e.unpackAlignment,this.colorSpace=e.colorSpace,this.renderTarget=e.renderTarget,this.isRenderTargetTexture=e.isRenderTargetTexture,this.isArrayTexture=e.isArrayTexture,this.userData=JSON.parse(JSON.stringify(e.userData)),this.needsUpdate=!0,this}setValues(e){for(let t in e){let n=e[t];if(n===void 0){nt(`Texture.setValues(): parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){nt(`Texture.setValues(): property '${t}' does not exist.`);continue}i&&n&&i.isVector2&&n.isVector2||i&&n&&i.isVector3&&n.isVector3||i&&n&&i.isMatrix3&&n.isMatrix3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.textures[this.uuid]!==void 0)return e.textures[this.uuid];let n={metadata:{version:4.7,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(e).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,normalized:this.normalized,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(n.userData=this.userData),t||(e.textures[this.uuid]=n),n}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(e){if(this.mapping!==_h)return e;if(e.applyMatrix3(this.matrix),e.x<0||e.x>1)switch(this.wrapS){case pi:e.x=e.x-Math.floor(e.x);break;case qn:e.x=e.x<0?0:1;break;case ur:Math.abs(Math.floor(e.x)%2)===1?e.x=Math.ceil(e.x)-e.x:e.x=e.x-Math.floor(e.x);break}if(e.y<0||e.y>1)switch(this.wrapT){case pi:e.y=e.y-Math.floor(e.y);break;case qn:e.y=e.y<0?0:1;break;case ur:Math.abs(Math.floor(e.y)%2)===1?e.y=Math.ceil(e.y)-e.y:e.y=e.y-Math.floor(e.y);break}return this.flipY&&(e.y=1-e.y),e}set needsUpdate(e){e===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(e){e===!0&&this.pmremVersion++}};Qt.DEFAULT_IMAGE=null;Qt.DEFAULT_MAPPING=_h;Qt.DEFAULT_ANISOTROPY=1;var Pt=class s{static{s.prototype.isVector4=!0}constructor(e=0,t=0,n=0,i=1){this.x=e,this.y=t,this.z=n,this.w=i}get width(){return this.z}set width(e){this.z=e}get height(){return this.w}set height(e){this.w=e}set(e,t,n,i){return this.x=e,this.y=t,this.z=n,this.w=i,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this.w=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setW(e){return this.w=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;case 3:this.w=t;break;default:throw new Error("THREE.Vector4: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("THREE.Vector4: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this.w=e.w!==void 0?e.w:1,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this.w+=e.w,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this.w+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this.w=e.w+t.w,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this.w+=e.w*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this.w-=e.w,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this.w-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this.w=e.w-t.w,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this.w*=e.w,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this.w*=e,this}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=this.w,a=e.elements;return this.x=a[0]*t+a[4]*n+a[8]*i+a[12]*r,this.y=a[1]*t+a[5]*n+a[9]*i+a[13]*r,this.z=a[2]*t+a[6]*n+a[10]*i+a[14]*r,this.w=a[3]*t+a[7]*n+a[11]*i+a[15]*r,this}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this.w/=e.w,this}divideScalar(e){return this.multiplyScalar(1/e)}setAxisAngleFromQuaternion(e){this.w=2*Math.acos(e.w);let t=Math.sqrt(1-e.w*e.w);return t<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=e.x/t,this.y=e.y/t,this.z=e.z/t),this}setAxisAngleFromRotationMatrix(e){let t,n,i,r,l=e.elements,c=l[0],h=l[4],u=l[8],d=l[1],p=l[5],g=l[9],_=l[2],m=l[6],f=l[10];if(Math.abs(h-d)<.01&&Math.abs(u-_)<.01&&Math.abs(g-m)<.01){if(Math.abs(h+d)<.1&&Math.abs(u+_)<.1&&Math.abs(g+m)<.1&&Math.abs(c+p+f-3)<.1)return this.set(1,0,0,0),this;t=Math.PI;let S=(c+1)/2,b=(p+1)/2,M=(f+1)/2,E=(h+d)/4,P=(u+_)/4,x=(g+m)/4;return S>b&&S>M?S<.01?(n=0,i=.707106781,r=.707106781):(n=Math.sqrt(S),i=E/n,r=P/n):b>M?b<.01?(n=.707106781,i=0,r=.707106781):(i=Math.sqrt(b),n=E/i,r=x/i):M<.01?(n=.707106781,i=.707106781,r=0):(r=Math.sqrt(M),n=P/r,i=x/r),this.set(n,i,r,t),this}let v=Math.sqrt((m-g)*(m-g)+(u-_)*(u-_)+(d-h)*(d-h));return Math.abs(v)<.001&&(v=1),this.x=(m-g)/v,this.y=(u-_)/v,this.z=(d-h)/v,this.w=Math.acos((c+p+f-1)/2),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this.w=t[15],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this.w=Math.min(this.w,e.w),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this.w=Math.max(this.w,e.w),this}clamp(e,t){return this.x=yt(this.x,e.x,t.x),this.y=yt(this.y,e.y,t.y),this.z=yt(this.z,e.z,t.z),this.w=yt(this.w,e.w,t.w),this}clampScalar(e,t){return this.x=yt(this.x,e,t),this.y=yt(this.y,e,t),this.z=yt(this.z,e,t),this.w=yt(this.w,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(yt(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z+this.w*e.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this.w+=(e.w-this.w)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this.w=e.w+(t.w-e.w)*n,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z&&e.w===this.w}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this.w=e[t+3],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e[t+3]=this.w,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this.w=e.getW(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},jo=class extends Xn{constructor(e=1,t=1,n={}){super(),n=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:jt,depthBuffer:!0,stencilBuffer:!1,resolveColorBuffer:!0,resolveDepthBuffer:!0,resolveStencilBuffer:!0,storeMultisampledColorBuffer:!0,storeMultisampledDepthBuffer:!0,storeMultisampledStencilBuffer:!0,depthTexture:null,samples:0,count:1,depth:1,multiview:!1,useArrayDepthTexture:!1},n),this.isRenderTarget=!0,this.width=e,this.height=t,this.depth=n.depth,this.scissor=new Pt(0,0,e,t),this.scissorTest=!1,this.viewport=new Pt(0,0,e,t),this.textures=[];let i={width:e,height:t,depth:n.depth},r=new Qt(i),a=n.count;for(let o=0;o<a;o++)this.textures[o]=r.clone(),this.textures[o].isRenderTargetTexture=!0,this.textures[o].renderTarget=this;this._setTextureOptions(n),this.depthBuffer=n.depthBuffer,this.stencilBuffer=n.stencilBuffer,this.resolveColorBuffer=n.resolveColorBuffer,this.resolveDepthBuffer=n.resolveDepthBuffer,this.resolveStencilBuffer=n.resolveStencilBuffer,this.storeMultisampledColorBuffer=n.storeMultisampledColorBuffer,this.storeMultisampledDepthBuffer=n.storeMultisampledDepthBuffer,this.storeMultisampledStencilBuffer=n.storeMultisampledStencilBuffer,this._depthTexture=null,this.depthTexture=n.depthTexture,this.samples=n.samples,this.multiview=n.multiview,this.useArrayDepthTexture=n.useArrayDepthTexture}_setTextureOptions(e={}){let t={minFilter:jt,generateMipmaps:!1,flipY:!1,internalFormat:null};e.mapping!==void 0&&(t.mapping=e.mapping),e.wrapS!==void 0&&(t.wrapS=e.wrapS),e.wrapT!==void 0&&(t.wrapT=e.wrapT),e.wrapR!==void 0&&(t.wrapR=e.wrapR),e.magFilter!==void 0&&(t.magFilter=e.magFilter),e.minFilter!==void 0&&(t.minFilter=e.minFilter),e.format!==void 0&&(t.format=e.format),e.type!==void 0&&(t.type=e.type),e.anisotropy!==void 0&&(t.anisotropy=e.anisotropy),e.colorSpace!==void 0&&(t.colorSpace=e.colorSpace),e.flipY!==void 0&&(t.flipY=e.flipY),e.generateMipmaps!==void 0&&(t.generateMipmaps=e.generateMipmaps),e.internalFormat!==void 0&&(t.internalFormat=e.internalFormat);for(let n=0;n<this.textures.length;n++)this.textures[n].setValues(t)}get texture(){return this.textures[0]}set texture(e){this.textures[0]=e}set depthTexture(e){this._depthTexture!==null&&this._depthTexture.renderTarget===this&&(this._depthTexture.renderTarget=null),e!==null&&e.renderTarget===null&&(e.renderTarget=this),this._depthTexture=e}get depthTexture(){return this._depthTexture}setSize(e,t,n=1){if(this.width!==e||this.height!==t||this.depth!==n){this.width=e,this.height=t,this.depth=n;for(let i=0,r=this.textures.length;i<r;i++)this.textures[i].image.width=e,this.textures[i].image.height=t,this.textures[i].image.depth=n,this.textures[i].isData3DTexture!==!0&&(this.textures[i].isArrayTexture=this.textures[i].image.depth>1);this.dispose()}this.viewport.set(0,0,e,t),this.scissor.set(0,0,e,t)}clone(){return new this.constructor().copy(this)}copy(e){this.width=e.width,this.height=e.height,this.depth=e.depth,this.scissor.copy(e.scissor),this.scissorTest=e.scissorTest,this.viewport.copy(e.viewport),this.textures.length=0;for(let t=0,n=e.textures.length;t<n;t++){this.textures[t]=e.textures[t].clone(),this.textures[t].isRenderTargetTexture=!0,this.textures[t].renderTarget=this;let i=Object.assign({},e.textures[t].image);this.textures[t].source=new mr(i)}if(this.depthBuffer=e.depthBuffer,this.stencilBuffer=e.stencilBuffer,this.resolveColorBuffer=e.resolveColorBuffer,this.resolveDepthBuffer=e.resolveDepthBuffer,this.resolveStencilBuffer=e.resolveStencilBuffer,this.storeMultisampledColorBuffer=e.storeMultisampledColorBuffer,this.storeMultisampledDepthBuffer=e.storeMultisampledDepthBuffer,this.storeMultisampledStencilBuffer=e.storeMultisampledStencilBuffer,e.depthTexture!==null)if(e.depthTexture.renderTarget===e){let t=e.depthTexture.clone();t.renderTarget=null,this.depthTexture=t}else this.depthTexture=e.depthTexture;return this.samples=e.samples,this.multiview=e.multiview,this.useArrayDepthTexture=e.useArrayDepthTexture,this}dispose(){this.dispatchEvent({type:"dispose"})}},Kt=class extends jo{constructor(e=1,t=1,n={}){super(e,t,n),this.isWebGLRenderTarget=!0}},ua=class extends Qt{constructor(e=null,t=1,n=1,i=1){super(null),this.isDataArrayTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=$t,this.minFilter=$t,this.wrapR=qn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}copy(e){return super.copy(e),this.wrapR=e.wrapR,this}addLayerUpdate(e){this.layerUpdates.add(e)}clearLayerUpdates(){this.layerUpdates.clear()}};var Ko=class extends Qt{constructor(e=null,t=1,n=1,i=1){super(null),this.isData3DTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=$t,this.minFilter=$t,this.wrapR=qn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}copy(e){return super.copy(e),this.wrapR=e.wrapR,this}};var pt=class s{static{s.prototype.isMatrix4=!0}constructor(e,t,n,i,r,a,o,l,c,h,u,d,p,g,_,m){this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,a,o,l,c,h,u,d,p,g,_,m)}set(e,t,n,i,r,a,o,l,c,h,u,d,p,g,_,m){let f=this.elements;return f[0]=e,f[4]=t,f[8]=n,f[12]=i,f[1]=r,f[5]=a,f[9]=o,f[13]=l,f[2]=c,f[6]=h,f[10]=u,f[14]=d,f[3]=p,f[7]=g,f[11]=_,f[15]=m,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new s().fromArray(this.elements)}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],t[9]=n[9],t[10]=n[10],t[11]=n[11],t[12]=n[12],t[13]=n[13],t[14]=n[14],t[15]=n[15],this}copyPosition(e){let t=this.elements,n=e.elements;return t[12]=n[12],t[13]=n[13],t[14]=n[14],this}setFromMatrix3(e){let t=e.elements;return this.set(t[0],t[3],t[6],0,t[1],t[4],t[7],0,t[2],t[5],t[8],0,0,0,0,1),this}extractBasis(e,t,n){return this.determinantAffine()===0?(e.set(1,0,0),t.set(0,1,0),n.set(0,0,1),this):(e.setFromMatrixColumn(this,0),t.setFromMatrixColumn(this,1),n.setFromMatrixColumn(this,2),this)}makeBasis(e,t,n){return this.set(e.x,t.x,n.x,0,e.y,t.y,n.y,0,e.z,t.z,n.z,0,0,0,0,1),this}extractRotation(e){if(e.determinantAffine()===0)return this.identity();let t=this.elements,n=e.elements,i=1/Js.setFromMatrixColumn(e,0).length(),r=1/Js.setFromMatrixColumn(e,1).length(),a=1/Js.setFromMatrixColumn(e,2).length();return t[0]=n[0]*i,t[1]=n[1]*i,t[2]=n[2]*i,t[3]=0,t[4]=n[4]*r,t[5]=n[5]*r,t[6]=n[6]*r,t[7]=0,t[8]=n[8]*a,t[9]=n[9]*a,t[10]=n[10]*a,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromEuler(e){let t=this.elements,n=e.x,i=e.y,r=e.z,a=Math.cos(n),o=Math.sin(n),l=Math.cos(i),c=Math.sin(i),h=Math.cos(r),u=Math.sin(r);if(e.order==="XYZ"){let d=a*h,p=a*u,g=o*h,_=o*u;t[0]=l*h,t[4]=-l*u,t[8]=c,t[1]=p+g*c,t[5]=d-_*c,t[9]=-o*l,t[2]=_-d*c,t[6]=g+p*c,t[10]=a*l}else if(e.order==="YXZ"){let d=l*h,p=l*u,g=c*h,_=c*u;t[0]=d+_*o,t[4]=g*o-p,t[8]=a*c,t[1]=a*u,t[5]=a*h,t[9]=-o,t[2]=p*o-g,t[6]=_+d*o,t[10]=a*l}else if(e.order==="ZXY"){let d=l*h,p=l*u,g=c*h,_=c*u;t[0]=d-_*o,t[4]=-a*u,t[8]=g+p*o,t[1]=p+g*o,t[5]=a*h,t[9]=_-d*o,t[2]=-a*c,t[6]=o,t[10]=a*l}else if(e.order==="ZYX"){let d=a*h,p=a*u,g=o*h,_=o*u;t[0]=l*h,t[4]=g*c-p,t[8]=d*c+_,t[1]=l*u,t[5]=_*c+d,t[9]=p*c-g,t[2]=-c,t[6]=o*l,t[10]=a*l}else if(e.order==="YZX"){let d=a*l,p=a*c,g=o*l,_=o*c;t[0]=l*h,t[4]=_-d*u,t[8]=g*u+p,t[1]=u,t[5]=a*h,t[9]=-o*h,t[2]=-c*h,t[6]=p*u+g,t[10]=d-_*u}else if(e.order==="XZY"){let d=a*l,p=a*c,g=o*l,_=o*c;t[0]=l*h,t[4]=-u,t[8]=c*h,t[1]=d*u+_,t[5]=a*h,t[9]=p*u-g,t[2]=g*u-p,t[6]=o*h,t[10]=_*u+d}return t[3]=0,t[7]=0,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromQuaternion(e){return this.compose(Sm,e,wm)}lookAt(e,t,n){let i=this.elements;return Nn.subVectors(e,t),Nn.lengthSq()===0&&(Nn.z=1),Nn.normalize(),Zi.crossVectors(n,Nn),Zi.lengthSq()===0&&(Math.abs(n.z)===1?Nn.x+=1e-4:Nn.z+=1e-4,Nn.normalize(),Zi.crossVectors(n,Nn)),Zi.normalize(),ho.crossVectors(Nn,Zi),i[0]=Zi.x,i[4]=ho.x,i[8]=Nn.x,i[1]=Zi.y,i[5]=ho.y,i[9]=Nn.y,i[2]=Zi.z,i[6]=ho.z,i[10]=Nn.z,this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,a=n[0],o=n[4],l=n[8],c=n[12],h=n[1],u=n[5],d=n[9],p=n[13],g=n[2],_=n[6],m=n[10],f=n[14],v=n[3],S=n[7],b=n[11],M=n[15],E=i[0],P=i[4],x=i[8],C=i[12],w=i[1],I=i[5],y=i[9],N=i[13],D=i[2],R=i[6],U=i[10],z=i[14],T=i[3],G=i[7],X=i[11],Z=i[15];return r[0]=a*E+o*w+l*D+c*T,r[4]=a*P+o*I+l*R+c*G,r[8]=a*x+o*y+l*U+c*X,r[12]=a*C+o*N+l*z+c*Z,r[1]=h*E+u*w+d*D+p*T,r[5]=h*P+u*I+d*R+p*G,r[9]=h*x+u*y+d*U+p*X,r[13]=h*C+u*N+d*z+p*Z,r[2]=g*E+_*w+m*D+f*T,r[6]=g*P+_*I+m*R+f*G,r[10]=g*x+_*y+m*U+f*X,r[14]=g*C+_*N+m*z+f*Z,r[3]=v*E+S*w+b*D+M*T,r[7]=v*P+S*I+b*R+M*G,r[11]=v*x+S*y+b*U+M*X,r[15]=v*C+S*N+b*z+M*Z,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[4]*=e,t[8]*=e,t[12]*=e,t[1]*=e,t[5]*=e,t[9]*=e,t[13]*=e,t[2]*=e,t[6]*=e,t[10]*=e,t[14]*=e,t[3]*=e,t[7]*=e,t[11]*=e,t[15]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[12],a=e[1],o=e[5],l=e[9],c=e[13],h=e[2],u=e[6],d=e[10],p=e[14],g=e[3],_=e[7],m=e[11],f=e[15],v=l*p-c*d,S=o*p-c*u,b=o*d-l*u,M=a*p-c*h,E=a*d-l*h,P=a*u-o*h;return t*(_*v-m*S+f*b)-n*(g*v-m*M+f*E)+i*(g*S-_*M+f*P)-r*(g*b-_*E+m*P)}determinantAffine(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[1],a=e[5],o=e[9],l=e[2],c=e[6],h=e[10];return t*(a*h-o*c)-n*(r*h-o*l)+i*(r*c-a*l)}transpose(){let e=this.elements,t;return t=e[1],e[1]=e[4],e[4]=t,t=e[2],e[2]=e[8],e[8]=t,t=e[6],e[6]=e[9],e[9]=t,t=e[3],e[3]=e[12],e[12]=t,t=e[7],e[7]=e[13],e[13]=t,t=e[11],e[11]=e[14],e[14]=t,this}setPosition(e,t,n){let i=this.elements;return e.isVector3?(i[12]=e.x,i[13]=e.y,i[14]=e.z):(i[12]=e,i[13]=t,i[14]=n),this}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8],u=e[9],d=e[10],p=e[11],g=e[12],_=e[13],m=e[14],f=e[15],v=t*o-n*a,S=t*l-i*a,b=t*c-r*a,M=n*l-i*o,E=n*c-r*o,P=i*c-r*l,x=h*_-u*g,C=h*m-d*g,w=h*f-p*g,I=u*m-d*_,y=u*f-p*_,N=d*f-p*m,D=v*N-S*y+b*I+M*w-E*C+P*x;if(D===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let R=1/D;return e[0]=(o*N-l*y+c*I)*R,e[1]=(i*y-n*N-r*I)*R,e[2]=(_*P-m*E+f*M)*R,e[3]=(d*E-u*P-p*M)*R,e[4]=(l*w-a*N-c*C)*R,e[5]=(t*N-i*w+r*C)*R,e[6]=(m*b-g*P-f*S)*R,e[7]=(h*P-d*b+p*S)*R,e[8]=(a*y-o*w+c*x)*R,e[9]=(n*w-t*y-r*x)*R,e[10]=(g*E-_*b+f*v)*R,e[11]=(u*b-h*E-p*v)*R,e[12]=(o*C-a*I-l*x)*R,e[13]=(t*I-n*C+i*x)*R,e[14]=(_*S-g*M-m*v)*R,e[15]=(h*M-u*S+d*v)*R,this}scale(e){let t=this.elements,n=e.x,i=e.y,r=e.z;return t[0]*=n,t[4]*=i,t[8]*=r,t[1]*=n,t[5]*=i,t[9]*=r,t[2]*=n,t[6]*=i,t[10]*=r,t[3]*=n,t[7]*=i,t[11]*=r,this}getMaxScaleOnAxis(){let e=this.elements,t=e[0]*e[0]+e[1]*e[1]+e[2]*e[2],n=e[4]*e[4]+e[5]*e[5]+e[6]*e[6],i=e[8]*e[8]+e[9]*e[9]+e[10]*e[10];return Math.sqrt(Math.max(t,n,i))}makeTranslation(e,t,n){return e.isVector3?this.set(1,0,0,e.x,0,1,0,e.y,0,0,1,e.z,0,0,0,1):this.set(1,0,0,e,0,1,0,t,0,0,1,n,0,0,0,1),this}makeRotationX(e){let t=Math.cos(e),n=Math.sin(e);return this.set(1,0,0,0,0,t,-n,0,0,n,t,0,0,0,0,1),this}makeRotationY(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,0,n,0,0,1,0,0,-n,0,t,0,0,0,0,1),this}makeRotationZ(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,0,n,t,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(e,t){let n=Math.cos(t),i=Math.sin(t),r=1-n,a=e.x,o=e.y,l=e.z,c=r*a,h=r*o;return this.set(c*a+n,c*o-i*l,c*l+i*o,0,c*o+i*l,h*o+n,h*l-i*a,0,c*l-i*o,h*l+i*a,r*l*l+n,0,0,0,0,1),this}makeScale(e,t,n){return this.set(e,0,0,0,0,t,0,0,0,0,n,0,0,0,0,1),this}makeShear(e,t,n,i,r,a){return this.set(1,n,r,0,e,1,a,0,t,i,1,0,0,0,0,1),this}compose(e,t,n){let i=this.elements,r=t._x,a=t._y,o=t._z,l=t._w,c=r+r,h=a+a,u=o+o,d=r*c,p=r*h,g=r*u,_=a*h,m=a*u,f=o*u,v=l*c,S=l*h,b=l*u,M=n.x,E=n.y,P=n.z;return i[0]=(1-(_+f))*M,i[1]=(p+b)*M,i[2]=(g-S)*M,i[3]=0,i[4]=(p-b)*E,i[5]=(1-(d+f))*E,i[6]=(m+v)*E,i[7]=0,i[8]=(g+S)*P,i[9]=(m-v)*P,i[10]=(1-(d+_))*P,i[11]=0,i[12]=e.x,i[13]=e.y,i[14]=e.z,i[15]=1,this}decompose(e,t,n){let i=this.elements;e.x=i[12],e.y=i[13],e.z=i[14];let r=this.determinantAffine();if(r===0)return n.set(1,1,1),t.identity(),this;let a=Js.set(i[0],i[1],i[2]).length(),o=Js.set(i[4],i[5],i[6]).length(),l=Js.set(i[8],i[9],i[10]).length();r<0&&(a=-a),$n.copy(this);let c=1/a,h=1/o,u=1/l;return $n.elements[0]*=c,$n.elements[1]*=c,$n.elements[2]*=c,$n.elements[4]*=h,$n.elements[5]*=h,$n.elements[6]*=h,$n.elements[8]*=u,$n.elements[9]*=u,$n.elements[10]*=u,t.setFromRotationMatrix($n),n.x=a,n.y=o,n.z=l,this}makePerspective(e,t,n,i,r,a,o=ni,l=!1){let c=this.elements,h=2*r/(t-e),u=2*r/(n-i),d=(t+e)/(t-e),p=(n+i)/(n-i),g,_;if(l)g=r/(a-r),_=a*r/(a-r);else if(o===ni)g=-(a+r)/(a-r),_=-2*a*r/(a-r);else if(o===dr)g=-a/(a-r),_=-a*r/(a-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+o);return c[0]=h,c[4]=0,c[8]=d,c[12]=0,c[1]=0,c[5]=u,c[9]=p,c[13]=0,c[2]=0,c[6]=0,c[10]=g,c[14]=_,c[3]=0,c[7]=0,c[11]=-1,c[15]=0,this}makeOrthographic(e,t,n,i,r,a,o=ni,l=!1){let c=this.elements,h=2/(t-e),u=2/(n-i),d=-(t+e)/(t-e),p=-(n+i)/(n-i),g,_;if(l)g=1/(a-r),_=a/(a-r);else if(o===ni)g=-2/(a-r),_=-(a+r)/(a-r);else if(o===dr)g=-1/(a-r),_=-r/(a-r);else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+o);return c[0]=h,c[4]=0,c[8]=0,c[12]=d,c[1]=0,c[5]=u,c[9]=0,c[13]=p,c[2]=0,c[6]=0,c[10]=g,c[14]=_,c[3]=0,c[7]=0,c[11]=0,c[15]=1,this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<16;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<16;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e[t+9]=n[9],e[t+10]=n[10],e[t+11]=n[11],e[t+12]=n[12],e[t+13]=n[13],e[t+14]=n[14],e[t+15]=n[15],e}},Js=new k,$n=new pt,Sm=new k(0,0,0),wm=new k(1,1,1),Zi=new k,ho=new k,Nn=new k,Yu=new pt,Ju=new Zt,Tn=class s{constructor(e=0,t=0,n=0,i=s.DEFAULT_ORDER){this.isEuler=!0,this._x=e,this._y=t,this._z=n,this._order=i}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get order(){return this._order}set order(e){this._order=e,this._onChangeCallback()}set(e,t,n,i=this._order){return this._x=e,this._y=t,this._z=n,this._order=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(e){return this._x=e._x,this._y=e._y,this._z=e._z,this._order=e._order,this._onChangeCallback(),this}setFromRotationMatrix(e,t=this._order,n=!0){let i=e.elements,r=i[0],a=i[4],o=i[8],l=i[1],c=i[5],h=i[9],u=i[2],d=i[6],p=i[10];switch(t){case"XYZ":this._y=Math.asin(yt(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(-h,p),this._z=Math.atan2(-a,r)):(this._x=Math.atan2(d,c),this._z=0);break;case"YXZ":this._x=Math.asin(-yt(h,-1,1)),Math.abs(h)<.9999999?(this._y=Math.atan2(o,p),this._z=Math.atan2(l,c)):(this._y=Math.atan2(-u,r),this._z=0);break;case"ZXY":this._x=Math.asin(yt(d,-1,1)),Math.abs(d)<.9999999?(this._y=Math.atan2(-u,p),this._z=Math.atan2(-a,c)):(this._y=0,this._z=Math.atan2(l,r));break;case"ZYX":this._y=Math.asin(-yt(u,-1,1)),Math.abs(u)<.9999999?(this._x=Math.atan2(d,p),this._z=Math.atan2(l,r)):(this._x=0,this._z=Math.atan2(-a,c));break;case"YZX":this._z=Math.asin(yt(l,-1,1)),Math.abs(l)<.9999999?(this._x=Math.atan2(-h,c),this._y=Math.atan2(-u,r)):(this._x=0,this._y=Math.atan2(o,p));break;case"XZY":this._z=Math.asin(-yt(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(d,c),this._y=Math.atan2(o,r)):(this._x=Math.atan2(-h,p),this._y=0);break;default:nt("Euler: .setFromRotationMatrix() encountered an unknown order: "+t)}return this._order=t,n===!0&&this._onChangeCallback(),this}setFromQuaternion(e,t,n){return Yu.makeRotationFromQuaternion(e),this.setFromRotationMatrix(Yu,t,n)}setFromVector3(e,t=this._order){return this.set(e.x,e.y,e.z,t)}reorder(e){return Ju.setFromEuler(this),this.setFromQuaternion(Ju,e)}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._order===this._order}fromArray(e){return this._x=e[0],this._y=e[1],this._z=e[2],e[3]!==void 0&&(this._order=e[3]),this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._order,e}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};Tn.DEFAULT_ORDER="XYZ";var gr=class{constructor(){this.mask=1}set(e){this.mask=(1<<e|0)>>>0}enable(e){this.mask|=1<<e|0}enableAll(){this.mask=-1}toggle(e){this.mask^=1<<e|0}disable(e){this.mask&=~(1<<e|0)}disableAll(){this.mask=0}test(e){return(this.mask&e.mask)!==0}isEnabled(e){return(this.mask&(1<<e|0))!==0}},Em=0,Zu=new k,Zs=new Zt,Ci=new pt,uo=new k,Jr=new k,Tm=new k,Am=new Zt,$u=new k(1,0,0),Qu=new k(0,1,0),ed=new k(0,0,1),td={type:"added"},Rm={type:"removed"},$s={type:"childadded",child:null},Nc={type:"childremoved",child:null},Ft=class s extends Xn{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:Em++}),this.uuid=ii(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=s.DEFAULT_UP.clone();let e=new k,t=new Tn,n=new Zt,i=new k(1,1,1);function r(){n.setFromEuler(t,!1)}function a(){t.setFromQuaternion(n,void 0,!1)}t._onChange(r),n._onChange(a),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:e},rotation:{configurable:!0,enumerable:!0,value:t},quaternion:{configurable:!0,enumerable:!0,value:n},scale:{configurable:!0,enumerable:!0,value:i},modelViewMatrix:{value:new pt},normalMatrix:{value:new mt}}),this.matrix=new pt,this.matrixWorld=new pt,this.matrixAutoUpdate=s.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=s.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new gr,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.customDepthMaterial=void 0,this.customDistanceMaterial=void 0,this.static=!1,this.userData={},this.pivot=null}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(e){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(e),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(e){return this.quaternion.premultiply(e),this}setRotationFromAxisAngle(e,t){this.quaternion.setFromAxisAngle(e,t)}setRotationFromEuler(e){this.quaternion.setFromEuler(e,!0)}setRotationFromMatrix(e){this.quaternion.setFromRotationMatrix(e)}setRotationFromQuaternion(e){this.quaternion.copy(e)}rotateOnAxis(e,t){return Zs.setFromAxisAngle(e,t),this.quaternion.multiply(Zs),this}rotateOnWorldAxis(e,t){return Zs.setFromAxisAngle(e,t),this.quaternion.premultiply(Zs),this}rotateX(e){return this.rotateOnAxis($u,e)}rotateY(e){return this.rotateOnAxis(Qu,e)}rotateZ(e){return this.rotateOnAxis(ed,e)}translateOnAxis(e,t){return Zu.copy(e).applyQuaternion(this.quaternion),this.position.add(Zu.multiplyScalar(t)),this}translateX(e){return this.translateOnAxis($u,e)}translateY(e){return this.translateOnAxis(Qu,e)}translateZ(e){return this.translateOnAxis(ed,e)}localToWorld(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(this.matrixWorld)}worldToLocal(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(Ci.copy(this.matrixWorld).invert())}lookAt(e,t,n){e.isVector3?uo.copy(e):uo.set(e,t,n);let i=this.parent;this.updateWorldMatrix(!0,!1),Jr.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?Ci.lookAt(Jr,uo,this.up):Ci.lookAt(uo,Jr,this.up),this.quaternion.setFromRotationMatrix(Ci),i&&(Ci.extractRotation(i.matrixWorld),Zs.setFromRotationMatrix(Ci),this.quaternion.premultiply(Zs.invert()))}add(e){if(arguments.length>1){for(let t=0;t<arguments.length;t++)this.add(arguments[t]);return this}return e===this?(ht("Object3D.add: object can't be added as a child of itself.",e),this):(e&&e.isObject3D?(e.removeFromParent(),e.parent=this,this.children.push(e),e.dispatchEvent(td),$s.child=e,this.dispatchEvent($s),$s.child=null):ht("Object3D.add: object not an instance of THREE.Object3D.",e),this)}remove(e){if(arguments.length>1){for(let n=0;n<arguments.length;n++)this.remove(arguments[n]);return this}let t=this.children.indexOf(e);return t!==-1&&(e.parent=null,this.children.splice(t,1),e.dispatchEvent(Rm),Nc.child=e,this.dispatchEvent(Nc),Nc.child=null),this}removeFromParent(){let e=this.parent;return e!==null&&e.remove(this),this}clear(){return this.remove(...this.children)}attach(e){return this.updateWorldMatrix(!0,!1),Ci.copy(this.matrixWorld).invert(),e.parent!==null&&(e.parent.updateWorldMatrix(!0,!1),Ci.multiply(e.parent.matrixWorld)),e.applyMatrix4(Ci),e.removeFromParent(),e.parent=this,this.children.push(e),e.updateWorldMatrix(!1,!0),e.dispatchEvent(td),$s.child=e,this.dispatchEvent($s),$s.child=null,this}getObjectById(e){return this.getObjectByProperty("id",e)}getObjectByName(e){return this.getObjectByProperty("name",e)}getObjectByProperty(e,t){if(this[e]===t)return this;for(let n=0,i=this.children.length;n<i;n++){let a=this.children[n].getObjectByProperty(e,t);if(a!==void 0)return a}}getObjectsByProperty(e,t,n=[]){this[e]===t&&n.push(this);let i=this.children;for(let r=0,a=i.length;r<a;r++)i[r].getObjectsByProperty(e,t,n);return n}getWorldPosition(e){return this.updateWorldMatrix(!0,!1),e.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Jr,e,Tm),e}getWorldScale(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Jr,Am,e),e}getWorldDirection(e){this.updateWorldMatrix(!0,!1);let t=this.matrixWorld.elements;return e.set(t[8],t[9],t[10]).normalize()}raycast(){}intersectsFrustum(){}traverse(e){e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverse(e)}traverseVisible(e){if(this.visible===!1)return;e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverseVisible(e)}traverseAncestors(e){let t=this.parent;t!==null&&(e(t),t.traverseAncestors(e))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale);let e=this.pivot;if(e!==null){let t=e.x,n=e.y,i=e.z,r=this.matrix.elements;r[12]+=t-r[0]*t-r[4]*n-r[8]*i,r[13]+=n-r[1]*t-r[5]*n-r[9]*i,r[14]+=i-r[2]*t-r[6]*n-r[10]*i}this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(e){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||e)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,e=!0);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].updateMatrixWorld(e)}updateWorldMatrix(e,t,n=!1){let i=this.parent;if(e===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||n)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,n=!0),t===!0){let r=this.children;for(let a=0,o=r.length;a<o;a++)r[a].updateWorldMatrix(!1,!0,n)}}toJSON(e){let t=e===void 0||typeof e=="string",n={};t&&(e={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},n.metadata={version:4.7,type:"Object",generator:"Object3D.toJSON"});let i={};i.uuid=this.uuid,i.type=this.type,i.name=this.name,i.castShadow=this.castShadow,i.receiveShadow=this.receiveShadow,i.visible=this.visible,i.frustumCulled=this.frustumCulled,i.renderOrder=this.renderOrder,i.static=this.static,i.matrixAutoUpdate=this.matrixAutoUpdate,Object.keys(this.userData).length>0&&(i.userData=this.userData),i.layers=this.layers.mask,i.matrix=this.matrix.toArray(),i.up=this.up.toArray(),this.pivot!==null&&(i.pivot=this.pivot.toArray()),this.morphTargetDictionary!==void 0&&(i.morphTargetDictionary=Object.assign({},this.morphTargetDictionary)),this.morphTargetInfluences!==void 0&&(i.morphTargetInfluences=this.morphTargetInfluences.slice()),this.isInstancedMesh&&(i.type="InstancedMesh",i.count=this.count,i.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(i.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(i.type="BatchedMesh",i.perObjectFrustumCulled=this.perObjectFrustumCulled,i.sortObjects=this.sortObjects,i.drawRanges=this._drawRanges,i.reservedRanges=this._reservedRanges,i.geometryInfo=this._geometryInfo.map(o=>({...o,boundingBox:o.boundingBox?o.boundingBox.toJSON():void 0,boundingSphere:o.boundingSphere?o.boundingSphere.toJSON():void 0})),i.instanceInfo=this._instanceInfo.map(o=>({...o})),i.availableInstanceIds=this._availableInstanceIds.slice(),i.availableGeometryIds=this._availableGeometryIds.slice(),i.nextIndexStart=this._nextIndexStart,i.nextVertexStart=this._nextVertexStart,i.geometryCount=this._geometryCount,i.maxInstanceCount=this._maxInstanceCount,i.maxVertexCount=this._maxVertexCount,i.maxIndexCount=this._maxIndexCount,i.geometryInitialized=this._geometryInitialized,i.matricesTexture=this._matricesTexture.toJSON(e),i.indirectTexture=this._indirectTexture.toJSON(e),this._colorsTexture!==null&&(i.colorsTexture=this._colorsTexture.toJSON(e)),this.boundingSphere!==null&&(i.boundingSphere=this.boundingSphere.toJSON()),this.boundingBox!==null&&(i.boundingBox=this.boundingBox.toJSON()));function r(o,l){return o[l.uuid]===void 0&&(o[l.uuid]=l.toJSON(e)),l.uuid}if(this.isScene)this.background&&(this.background.isColor?i.background=this.background.toJSON():this.background.isTexture&&(i.background=this.background.toJSON(e).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(i.environment=this.environment.toJSON(e).uuid);else if(this.isMesh||this.isLine||this.isPoints){i.geometry=r(e.geometries,this.geometry);let o=this.geometry.parameters;if(o!==void 0&&o.shapes!==void 0){let l=o.shapes;if(Array.isArray(l))for(let c=0,h=l.length;c<h;c++){let u=l[c];r(e.shapes,u)}else r(e.shapes,l)}}if(this.isSkinnedMesh&&(i.bindMode=this.bindMode,i.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(e.skeletons,this.skeleton),i.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let o=[];for(let l=0,c=this.material.length;l<c;l++)o.push(r(e.materials,this.material[l]));i.material=o}else i.material=r(e.materials,this.material);if(this.children.length>0){i.children=[];for(let o=0;o<this.children.length;o++)i.children.push(this.children[o].toJSON(e).object)}if(this.animations.length>0){i.animations=[];for(let o=0;o<this.animations.length;o++){let l=this.animations[o];i.animations.push(r(e.animations,l))}}if(t){let o=a(e.geometries),l=a(e.materials),c=a(e.textures),h=a(e.images),u=a(e.shapes),d=a(e.skeletons),p=a(e.animations),g=a(e.nodes);o.length>0&&(n.geometries=o),l.length>0&&(n.materials=l),c.length>0&&(n.textures=c),h.length>0&&(n.images=h),u.length>0&&(n.shapes=u),d.length>0&&(n.skeletons=d),p.length>0&&(n.animations=p),g.length>0&&(n.nodes=g)}return n.object=i,n;function a(o){let l=[];for(let c in o){let h=o[c];delete h.metadata,l.push(h)}return l}}clone(e){return new this.constructor().copy(this,e)}copy(e,t=!0){if(this.name=e.name,this.up.copy(e.up),this.position.copy(e.position),this.rotation.order=e.rotation.order,this.quaternion.copy(e.quaternion),this.scale.copy(e.scale),this.pivot=e.pivot!==null?e.pivot.clone():null,this.matrix.copy(e.matrix),this.matrixWorld.copy(e.matrixWorld),this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrixWorldAutoUpdate=e.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=e.matrixWorldNeedsUpdate,this.layers.mask=e.layers.mask,this.visible=e.visible,this.castShadow=e.castShadow,this.receiveShadow=e.receiveShadow,this.frustumCulled=e.frustumCulled,this.renderOrder=e.renderOrder,this.static=e.static,this.animations=e.animations.slice(),this.userData=JSON.parse(JSON.stringify(e.userData)),t===!0)for(let n=0;n<e.children.length;n++){let i=e.children[n];this.add(i.clone())}return this}dispose(){this.dispatchEvent({type:"dispose"})}};Ft.DEFAULT_UP=new k(0,1,0);Ft.DEFAULT_MATRIX_AUTO_UPDATE=!0;Ft.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var gt=class extends Ft{constructor(){super(),this.isGroup=!0,this.type="Group"}},Cm={type:"move"},br=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new gt,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new gt,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new k,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new k),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new gt,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new k,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new k,this._grip.eventsEnabled=!1),this._grip}dispatchEvent(e){return this._targetRay!==null&&this._targetRay.dispatchEvent(e),this._grip!==null&&this._grip.dispatchEvent(e),this._hand!==null&&this._hand.dispatchEvent(e),this}connect(e){if(e&&e.hand){let t=this._hand;if(t)for(let n of e.hand.values())this._getHandJoint(t,n)}return this.dispatchEvent({type:"connected",data:e}),this}disconnect(e){return this.dispatchEvent({type:"disconnected",data:e}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(e,t,n){let i=null,r=null,a=null,o=this._targetRay,l=this._grip,c=this._hand;if(e&&t.session.visibilityState!=="visible-blurred"){if(c&&e.hand){a=!0;for(let _ of e.hand.values()){let m=t.getJointPose(_,n),f=this._getHandJoint(c,_);m!==null&&(f.matrix.fromArray(m.transform.matrix),f.matrix.decompose(f.position,f.rotation,f.scale),f.matrixWorldNeedsUpdate=!0,f.jointRadius=m.radius),f.visible=m!==null}let h=c.joints["index-finger-tip"],u=c.joints["thumb-tip"],d=h.position.distanceTo(u.position),p=.02,g=.005;c.inputState.pinching&&d>p+g?(c.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:e.handedness,target:this})):!c.inputState.pinching&&d<=p-g&&(c.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:e.handedness,target:this}))}else l!==null&&e.gripSpace&&(r=t.getPose(e.gripSpace,n),r!==null&&(l.matrix.fromArray(r.transform.matrix),l.matrix.decompose(l.position,l.rotation,l.scale),l.matrixWorldNeedsUpdate=!0,r.linearVelocity?(l.hasLinearVelocity=!0,l.linearVelocity.copy(r.linearVelocity)):l.hasLinearVelocity=!1,r.angularVelocity?(l.hasAngularVelocity=!0,l.angularVelocity.copy(r.angularVelocity)):l.hasAngularVelocity=!1,l.eventsEnabled&&l.dispatchEvent({type:"gripUpdated",data:e,target:this})));o!==null&&(i=t.getPose(e.targetRaySpace,n),i===null&&r!==null&&(i=r),i!==null&&(o.matrix.fromArray(i.transform.matrix),o.matrix.decompose(o.position,o.rotation,o.scale),o.matrixWorldNeedsUpdate=!0,i.linearVelocity?(o.hasLinearVelocity=!0,o.linearVelocity.copy(i.linearVelocity)):o.hasLinearVelocity=!1,i.angularVelocity?(o.hasAngularVelocity=!0,o.angularVelocity.copy(i.angularVelocity)):o.hasAngularVelocity=!1,this.dispatchEvent(Cm)))}return o!==null&&(o.visible=i!==null),l!==null&&(l.visible=r!==null),c!==null&&(c.visible=a!==null),this}_getHandJoint(e,t){if(e.joints[t.jointName]===void 0){let n=new gt;n.matrixAutoUpdate=!1,n.visible=!1,e.joints[t.jointName]=n,e.add(n)}return e.joints[t.jointName]}},_f={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},$i={h:0,s:0,l:0},fo={h:0,s:0,l:0};function Uc(s,e,t){return t<0&&(t+=1),t>1&&(t-=1),t<1/6?s+(e-s)*6*t:t<1/2?e:t<2/3?s+(e-s)*6*(2/3-t):s}var Le=class{constructor(e,t,n){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(e,t,n)}set(e,t,n){if(t===void 0&&n===void 0){let i=e;i&&i.isColor?this.copy(i):typeof i=="number"?this.setHex(i):typeof i=="string"&&this.setStyle(i)}else this.setRGB(e,t,n);return this}setScalar(e){return this.r=e,this.g=e,this.b=e,this}setHex(e,t=Xt){return e=Math.floor(e),this.r=(e>>16&255)/255,this.g=(e>>8&255)/255,this.b=(e&255)/255,_t.colorSpaceToWorking(this,t),this}setRGB(e,t,n,i=_t.workingColorSpace){return this.r=e,this.g=t,this.b=n,_t.colorSpaceToWorking(this,i),this}setHSL(e,t,n,i=_t.workingColorSpace){if(e=Ph(e,1),t=yt(t,0,1),n=yt(n,0,1),t===0)this.r=this.g=this.b=n;else{let r=n<=.5?n*(1+t):n+t-n*t,a=2*n-r;this.r=Uc(a,r,e+1/3),this.g=Uc(a,r,e),this.b=Uc(a,r,e-1/3)}return _t.colorSpaceToWorking(this,i),this}setStyle(e,t=Xt){function n(r){r!==void 0&&parseFloat(r)<1&&nt("Color: Alpha component of "+e+" will be ignored.")}let i;if(i=/^(\w+)\(([^\)]*)\)/.exec(e)){let r,a=i[1],o=i[2];switch(a){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,t);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,t);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,t);break;default:nt("Color: Unknown color model "+e)}}else if(i=/^\#([A-Fa-f\d]+)$/.exec(e)){let r=i[1],a=r.length;if(a===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,t);if(a===6)return this.setHex(parseInt(r,16),t);nt("Color: Invalid hex color "+e)}else if(e&&e.length>0)return this.setColorName(e,t);return this}setColorName(e,t=Xt){let n=_f[e.toLowerCase()];return n!==void 0?this.setHex(n,t):nt("Color: Unknown color "+e),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(e){return this.r=e.r,this.g=e.g,this.b=e.b,this}copySRGBToLinear(e){return this.r=Ni(e.r),this.g=Ni(e.g),this.b=Ni(e.b),this}copyLinearToSRGB(e){return this.r=cr(e.r),this.g=cr(e.g),this.b=cr(e.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(e=Xt){return _t.workingToColorSpace(bn.copy(this),e),Math.round(yt(bn.r*255,0,255))*65536+Math.round(yt(bn.g*255,0,255))*256+Math.round(yt(bn.b*255,0,255))}getHexString(e=Xt){return("000000"+this.getHex(e).toString(16)).slice(-6)}getHSL(e,t=_t.workingColorSpace){_t.workingToColorSpace(bn.copy(this),t);let n=bn.r,i=bn.g,r=bn.b,a=Math.max(n,i,r),o=Math.min(n,i,r),l,c,h=(o+a)/2;if(o===a)l=0,c=0;else{let u=a-o;switch(c=h<=.5?u/(a+o):u/(2-a-o),a){case n:l=(i-r)/u+(i<r?6:0);break;case i:l=(r-n)/u+2;break;case r:l=(n-i)/u+4;break}l/=6}return e.h=l,e.s=c,e.l=h,e}getRGB(e,t=_t.workingColorSpace){return _t.workingToColorSpace(bn.copy(this),t),e.r=bn.r,e.g=bn.g,e.b=bn.b,e}getStyle(e=Xt){_t.workingToColorSpace(bn.copy(this),e);let t=bn.r,n=bn.g,i=bn.b;return e!==Xt?`color(${e} ${t.toFixed(3)} ${n.toFixed(3)} ${i.toFixed(3)})`:`rgb(${Math.round(t*255)},${Math.round(n*255)},${Math.round(i*255)})`}offsetHSL(e,t,n){return this.getHSL($i),this.setHSL($i.h+e,$i.s+t,$i.l+n)}add(e){return this.r+=e.r,this.g+=e.g,this.b+=e.b,this}addColors(e,t){return this.r=e.r+t.r,this.g=e.g+t.g,this.b=e.b+t.b,this}addScalar(e){return this.r+=e,this.g+=e,this.b+=e,this}sub(e){return this.r=Math.max(0,this.r-e.r),this.g=Math.max(0,this.g-e.g),this.b=Math.max(0,this.b-e.b),this}multiply(e){return this.r*=e.r,this.g*=e.g,this.b*=e.b,this}multiplyScalar(e){return this.r*=e,this.g*=e,this.b*=e,this}lerp(e,t){return this.r+=(e.r-this.r)*t,this.g+=(e.g-this.g)*t,this.b+=(e.b-this.b)*t,this}lerpColors(e,t,n){return this.r=e.r+(t.r-e.r)*n,this.g=e.g+(t.g-e.g)*n,this.b=e.b+(t.b-e.b)*n,this}lerpHSL(e,t){this.getHSL($i),e.getHSL(fo);let n=ra($i.h,fo.h,t),i=ra($i.s,fo.s,t),r=ra($i.l,fo.l,t);return this.setHSL(n,i,r),this}setFromVector3(e){return this.r=e.x,this.g=e.y,this.b=e.z,this}applyMatrix3(e){let t=this.r,n=this.g,i=this.b,r=e.elements;return this.r=r[0]*t+r[3]*n+r[6]*i,this.g=r[1]*t+r[4]*n+r[7]*i,this.b=r[2]*t+r[5]*n+r[8]*i,this}equals(e){return e.r===this.r&&e.g===this.g&&e.b===this.b}fromArray(e,t=0){return this.r=e[t],this.g=e[t+1],this.b=e[t+2],this}toArray(e=[],t=0){return e[t]=this.r,e[t+1]=this.g,e[t+2]=this.b,e}fromBufferAttribute(e,t){return this.r=e.getX(t),this.g=e.getY(t),this.b=e.getZ(t),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},bn=new Le;Le.NAMES=_f;var da=class s{constructor(e,t=25e-5){this.isFogExp2=!0,this.name="",this.color=new Le(e),this.density=t}clone(){return new s(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var Cs=class extends Ft{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new Tn,this.environmentIntensity=1,this.environmentRotation=new Tn,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(e,t){return super.copy(e,t),e.background!==null&&(this.background=e.background.clone()),e.environment!==null&&(this.environment=e.environment.clone()),e.fog!==null&&(this.fog=e.fog.clone()),this.backgroundBlurriness=e.backgroundBlurriness,this.backgroundIntensity=e.backgroundIntensity,this.backgroundRotation.copy(e.backgroundRotation),this.environmentIntensity=e.environmentIntensity,this.environmentRotation.copy(e.environmentRotation),e.overrideMaterial!==null&&(this.overrideMaterial=e.overrideMaterial.clone()),this.matrixAutoUpdate=e.matrixAutoUpdate,this}toJSON(e){let t=super.toJSON(e);return this.fog!==null&&(t.object.fog=this.fog.toJSON()),t.object.backgroundBlurriness=this.backgroundBlurriness,t.object.backgroundIntensity=this.backgroundIntensity,t.object.backgroundRotation=this.backgroundRotation.toArray(),t.object.environmentIntensity=this.environmentIntensity,t.object.environmentRotation=this.environmentRotation.toArray(),t}},Qn=new k,Pi=new k,Oc=new k,Ii=new k,Qs=new k,er=new k,nd=new k,zc=new k,Bc=new k,kc=new k,Hc=new Pt,Gc=new Pt,Vc=new Pt,is=class s{constructor(e=new k,t=new k,n=new k){this.a=e,this.b=t,this.c=n}static getNormal(e,t,n,i){i.subVectors(n,t),Qn.subVectors(e,t),i.cross(Qn);let r=i.lengthSq();return r>0?i.multiplyScalar(1/Math.sqrt(r)):i.set(0,0,0)}static getBarycoord(e,t,n,i,r){Qn.subVectors(i,t),Pi.subVectors(n,t),Oc.subVectors(e,t);let a=Qn.dot(Qn),o=Qn.dot(Pi),l=Qn.dot(Oc),c=Pi.dot(Pi),h=Pi.dot(Oc),u=a*c-o*o;if(u===0)return r.set(0,0,0),null;let d=1/u,p=(c*l-o*h)*d,g=(a*h-o*l)*d;return r.set(1-p-g,g,p)}static containsPoint(e,t,n,i){return this.getBarycoord(e,t,n,i,Ii)===null?!1:Ii.x>=0&&Ii.y>=0&&Ii.x+Ii.y<=1}static getInterpolation(e,t,n,i,r,a,o,l){return this.getBarycoord(e,t,n,i,Ii)===null?(l.x=0,l.y=0,"z"in l&&(l.z=0),"w"in l&&(l.w=0),null):(l.setScalar(0),l.addScaledVector(r,Ii.x),l.addScaledVector(a,Ii.y),l.addScaledVector(o,Ii.z),l)}static getInterpolatedAttribute(e,t,n,i,r,a){return Hc.setScalar(0),Gc.setScalar(0),Vc.setScalar(0),Hc.fromBufferAttribute(e,t),Gc.fromBufferAttribute(e,n),Vc.fromBufferAttribute(e,i),a.setScalar(0),a.addScaledVector(Hc,r.x),a.addScaledVector(Gc,r.y),a.addScaledVector(Vc,r.z),a}static isFrontFacing(e,t,n,i){return Qn.subVectors(n,t),Pi.subVectors(e,t),Qn.cross(Pi).dot(i)<0}set(e,t,n){return this.a.copy(e),this.b.copy(t),this.c.copy(n),this}setFromPointsAndIndices(e,t,n,i){return this.a.copy(e[t]),this.b.copy(e[n]),this.c.copy(e[i]),this}setFromAttributeAndIndices(e,t,n,i){return this.a.fromBufferAttribute(e,t),this.b.fromBufferAttribute(e,n),this.c.fromBufferAttribute(e,i),this}clone(){return new this.constructor().copy(this)}copy(e){return this.a.copy(e.a),this.b.copy(e.b),this.c.copy(e.c),this}getArea(){return Qn.subVectors(this.c,this.b),Pi.subVectors(this.a,this.b),Qn.cross(Pi).length()*.5}getMidpoint(e){return e.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(e){return s.getNormal(this.a,this.b,this.c,e)}getPlane(e){return e.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(e,t){return s.getBarycoord(e,this.a,this.b,this.c,t)}getInterpolation(e,t,n,i,r){return s.getInterpolation(e,this.a,this.b,this.c,t,n,i,r)}containsPoint(e){return s.containsPoint(e,this.a,this.b,this.c)}isFrontFacing(e){return s.isFrontFacing(this.a,this.b,this.c,e)}intersectsBox(e){return e.intersectsTriangle(this)}closestPointToPoint(e,t){let n=this.a,i=this.b,r=this.c,a,o;Qs.subVectors(i,n),er.subVectors(r,n),zc.subVectors(e,n);let l=Qs.dot(zc),c=er.dot(zc);if(l<=0&&c<=0)return t.copy(n);Bc.subVectors(e,i);let h=Qs.dot(Bc),u=er.dot(Bc);if(h>=0&&u<=h)return t.copy(i);let d=l*u-h*c;if(d<=0&&l>=0&&h<=0)return a=l/(l-h),t.copy(n).addScaledVector(Qs,a);kc.subVectors(e,r);let p=Qs.dot(kc),g=er.dot(kc);if(g>=0&&p<=g)return t.copy(r);let _=p*c-l*g;if(_<=0&&c>=0&&g<=0)return o=c/(c-g),t.copy(n).addScaledVector(er,o);let m=h*g-p*u;if(m<=0&&u-h>=0&&p-g>=0)return nd.subVectors(r,i),o=(u-h)/(u-h+(p-g)),t.copy(i).addScaledVector(nd,o);let f=1/(m+_+d);return a=_*f,o=d*f,t.copy(n).addScaledVector(Qs,a).addScaledVector(er,o)}equals(e){return e.a.equals(this.a)&&e.b.equals(this.b)&&e.c.equals(this.c)}},en=class{constructor(e=new k(1/0,1/0,1/0),t=new k(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=e,this.max=t}set(e,t){return this.min.copy(e),this.max.copy(t),this}setFromArray(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t+=3)this.expandByPoint(ei.fromArray(e,t));return this}setFromBufferAttribute(e){this.makeEmpty();for(let t=0,n=e.count;t<n;t++)this.expandByPoint(ei.fromBufferAttribute(e,t));return this}setFromPoints(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t++)this.expandByPoint(e[t]);return this}setFromCenterAndSize(e,t){let n=ei.copy(t).multiplyScalar(.5);return this.min.copy(e).sub(n),this.max.copy(e).add(n),this}setFromObject(e,t=!1){return this.makeEmpty(),this.expandByObject(e,t)}clone(){return new this.constructor().copy(this)}copy(e){return this.min.copy(e.min),this.max.copy(e.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(e){return this.isEmpty()?e.set(0,0,0):e.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(e){return this.isEmpty()?e.set(0,0,0):e.subVectors(this.max,this.min)}expandByPoint(e){return this.min.min(e),this.max.max(e),this}expandByVector(e){return this.min.sub(e),this.max.add(e),this}expandByScalar(e){return this.min.addScalar(-e),this.max.addScalar(e),this}expandByObject(e,t=!1){e.updateWorldMatrix(!1,!1);let n=e.geometry;if(n!==void 0){let r=n.getAttribute("position");if(t===!0&&r!==void 0&&e.isInstancedMesh!==!0)for(let a=0,o=r.count;a<o;a++)e.isMesh===!0?e.getVertexPosition(a,ei):ei.fromBufferAttribute(r,a),ei.applyMatrix4(e.matrixWorld),this.expandByPoint(ei);else e.boundingBox!==void 0?(e.boundingBox===null&&e.computeBoundingBox(),po.copy(e.boundingBox)):(n.boundingBox===null&&n.computeBoundingBox(),po.copy(n.boundingBox)),po.applyMatrix4(e.matrixWorld),this.union(po)}let i=e.children;for(let r=0,a=i.length;r<a;r++)this.expandByObject(i[r],t);return this}containsPoint(e){return e.x>=this.min.x&&e.x<=this.max.x&&e.y>=this.min.y&&e.y<=this.max.y&&e.z>=this.min.z&&e.z<=this.max.z}containsBox(e){return this.min.x<=e.min.x&&e.max.x<=this.max.x&&this.min.y<=e.min.y&&e.max.y<=this.max.y&&this.min.z<=e.min.z&&e.max.z<=this.max.z}getParameter(e,t){return t.set((e.x-this.min.x)/(this.max.x-this.min.x),(e.y-this.min.y)/(this.max.y-this.min.y),(e.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(e){return e.max.x>=this.min.x&&e.min.x<=this.max.x&&e.max.y>=this.min.y&&e.min.y<=this.max.y&&e.max.z>=this.min.z&&e.min.z<=this.max.z}intersectsSphere(e){return this.clampPoint(e.center,ei),ei.distanceToSquared(e.center)<=e.radius*e.radius}intersectsPlane(e){let t,n;return e.normal.x>0?(t=e.normal.x*this.min.x,n=e.normal.x*this.max.x):(t=e.normal.x*this.max.x,n=e.normal.x*this.min.x),e.normal.y>0?(t+=e.normal.y*this.min.y,n+=e.normal.y*this.max.y):(t+=e.normal.y*this.max.y,n+=e.normal.y*this.min.y),e.normal.z>0?(t+=e.normal.z*this.min.z,n+=e.normal.z*this.max.z):(t+=e.normal.z*this.max.z,n+=e.normal.z*this.min.z),t<=-e.constant&&n>=-e.constant}intersectsTriangle(e){if(this.isEmpty())return!1;this.getCenter(Zr),mo.subVectors(this.max,Zr),tr.subVectors(e.a,Zr),nr.subVectors(e.b,Zr),ir.subVectors(e.c,Zr),Qi.subVectors(nr,tr),es.subVectors(ir,nr),vs.subVectors(tr,ir);let t=[0,-Qi.z,Qi.y,0,-es.z,es.y,0,-vs.z,vs.y,Qi.z,0,-Qi.x,es.z,0,-es.x,vs.z,0,-vs.x,-Qi.y,Qi.x,0,-es.y,es.x,0,-vs.y,vs.x,0];return!Wc(t,tr,nr,ir,mo)||(t=[1,0,0,0,1,0,0,0,1],!Wc(t,tr,nr,ir,mo))?!1:(go.crossVectors(Qi,es),t=[go.x,go.y,go.z],Wc(t,tr,nr,ir,mo))}clampPoint(e,t){return t.copy(e).clamp(this.min,this.max)}distanceToPoint(e){return this.clampPoint(e,ei).distanceTo(e)}getBoundingSphere(e){return this.isEmpty()?e.makeEmpty():(this.getCenter(e.center),e.radius=this.getSize(ei).length()*.5),e}intersect(e){return this.min.max(e.min),this.max.min(e.max),this.isEmpty()&&this.makeEmpty(),this}union(e){return this.min.min(e.min),this.max.max(e.max),this}applyMatrix4(e){return this.isEmpty()?this:(Di[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(e),Di[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(e),Di[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(e),Di[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(e),Di[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(e),Di[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(e),Di[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(e),Di[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(e),this.setFromPoints(Di),this)}translate(e){return this.min.add(e),this.max.add(e),this}equals(e){return e.min.equals(this.min)&&e.max.equals(this.max)}toJSON(){return{min:this.min.toArray(),max:this.max.toArray()}}fromJSON(e){return this.min.fromArray(e.min),this.max.fromArray(e.max),this}},Di=[new k,new k,new k,new k,new k,new k,new k,new k],ei=new k,po=new en,tr=new k,nr=new k,ir=new k,Qi=new k,es=new k,vs=new k,Zr=new k,mo=new k,go=new k,_s=new k;function Wc(s,e,t,n,i){for(let r=0,a=s.length-3;r<=a;r+=3){_s.fromArray(s,r);let o=i.x*Math.abs(_s.x)+i.y*Math.abs(_s.y)+i.z*Math.abs(_s.z),l=e.dot(_s),c=t.dot(_s),h=n.dot(_s);if(Math.max(-Math.max(l,c,h),Math.min(l,c,h))>o)return!1}return!0}var nn=new k,bo=new Be,Pm=0,Mt=class extends Xn{constructor(e,t,n=!1){if(super(),Array.isArray(e))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,Object.defineProperty(this,"id",{value:Pm++}),this.name="",this.array=e,this.itemSize=t,this.count=e!==void 0?e.length/t:0,this.normalized=n,this.usage=Rh,this.updateRanges=[],this.gpuType=kn,this.version=0}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.name=e.name,this.array=new e.array.constructor(e.array),this.itemSize=e.itemSize,this.count=e.count,this.normalized=e.normalized,this.usage=e.usage,this.gpuType=e.gpuType,this}copyAt(e,t,n){e*=this.itemSize,n*=t.itemSize;for(let i=0,r=this.itemSize;i<r;i++)this.array[e+i]=t.array[n+i];return this}copyArray(e){return this.array.set(e),this}applyMatrix3(e){if(this.itemSize===2)for(let t=0,n=this.count;t<n;t++)bo.fromBufferAttribute(this,t),bo.applyMatrix3(e),this.setXY(t,bo.x,bo.y);else if(this.itemSize===3)for(let t=0,n=this.count;t<n;t++)nn.fromBufferAttribute(this,t),nn.applyMatrix3(e),this.setXYZ(t,nn.x,nn.y,nn.z);return this}applyMatrix4(e){for(let t=0,n=this.count;t<n;t++)nn.fromBufferAttribute(this,t),nn.applyMatrix4(e),this.setXYZ(t,nn.x,nn.y,nn.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)nn.fromBufferAttribute(this,t),nn.applyNormalMatrix(e),this.setXYZ(t,nn.x,nn.y,nn.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)nn.fromBufferAttribute(this,t),nn.transformDirection(e),this.setXYZ(t,nn.x,nn.y,nn.z);return this}set(e,t=0){return this.array.set(e,t),this}getComponent(e,t){let n=this.array[e*this.itemSize+t];return this.normalized&&(n=ti(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=Ut(n,this.array)),this.array[e*this.itemSize+t]=n,this}getX(e){let t=this.array[e*this.itemSize];return this.normalized&&(t=ti(t,this.array)),t}setX(e,t){return this.normalized&&(t=Ut(t,this.array)),this.array[e*this.itemSize]=t,this}getY(e){let t=this.array[e*this.itemSize+1];return this.normalized&&(t=ti(t,this.array)),t}setY(e,t){return this.normalized&&(t=Ut(t,this.array)),this.array[e*this.itemSize+1]=t,this}getZ(e){let t=this.array[e*this.itemSize+2];return this.normalized&&(t=ti(t,this.array)),t}setZ(e,t){return this.normalized&&(t=Ut(t,this.array)),this.array[e*this.itemSize+2]=t,this}getW(e){let t=this.array[e*this.itemSize+3];return this.normalized&&(t=ti(t,this.array)),t}setW(e,t){return this.normalized&&(t=Ut(t,this.array)),this.array[e*this.itemSize+3]=t,this}setXY(e,t,n){return e*=this.itemSize,this.normalized&&(t=Ut(t,this.array),n=Ut(n,this.array)),this.array[e+0]=t,this.array[e+1]=n,this}setXYZ(e,t,n,i){return e*=this.itemSize,this.normalized&&(t=Ut(t,this.array),n=Ut(n,this.array),i=Ut(i,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e*=this.itemSize,this.normalized&&(t=Ut(t,this.array),n=Ut(n,this.array),i=Ut(i,this.array),r=Ut(r,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this.array[e+3]=r,this}onUpload(e){return this.onUploadCallback=e,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let e={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return e.name=this.name,e.usage=this.usage,e.gpuType=this.gpuType,e}dispose(){this.dispatchEvent({type:"dispose"})}};var fa=class extends Mt{constructor(e,t,n){super(new Uint16Array(e),t,n)}};var pa=class extends Mt{constructor(e,t,n){super(new Uint32Array(e),t,n)}};var ut=class extends Mt{constructor(e,t,n){super(new Float32Array(e),t,n)}},Im=new en,$r=new k,qc=new k,xn=class{constructor(e=new k,t=-1){this.isSphere=!0,this.center=e,this.radius=t}set(e,t){return this.center.copy(e),this.radius=t,this}setFromPoints(e,t){let n=this.center;t!==void 0?n.copy(t):Im.setFromPoints(e).getCenter(n);let i=0;for(let r=0,a=e.length;r<a;r++)i=Math.max(i,n.distanceToSquared(e[r]));return this.radius=Math.sqrt(i),this}copy(e){return this.center.copy(e.center),this.radius=e.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(e){return e.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(e){return e.distanceTo(this.center)-this.radius}intersectsSphere(e){let t=this.radius+e.radius;return e.center.distanceToSquared(this.center)<=t*t}intersectsBox(e){return e.intersectsSphere(this)}intersectsPlane(e){return Math.abs(e.distanceToPoint(this.center))<=this.radius}clampPoint(e,t){let n=this.center.distanceToSquared(e);return t.copy(e),n>this.radius*this.radius&&(t.sub(this.center).normalize(),t.multiplyScalar(this.radius).add(this.center)),t}getBoundingBox(e){return this.isEmpty()?(e.makeEmpty(),e):(e.set(this.center,this.center),e.expandByScalar(this.radius),e)}applyMatrix4(e){return this.center.applyMatrix4(e),this.radius=this.radius*e.getMaxScaleOnAxis(),this}translate(e){return this.center.add(e),this}expandByPoint(e){if(this.isEmpty())return this.center.copy(e),this.radius=0,this;$r.subVectors(e,this.center);let t=$r.lengthSq();if(t>this.radius*this.radius){let n=Math.sqrt(t),i=(n-this.radius)*.5;this.center.addScaledVector($r,i/n),this.radius+=i}return this}union(e){return e.isEmpty()?this:this.isEmpty()?(this.copy(e),this):(this.center.equals(e.center)===!0?this.radius=Math.max(this.radius,e.radius):(qc.subVectors(e.center,this.center).setLength(e.radius),this.expandByPoint($r.copy(e.center).add(qc)),this.expandByPoint($r.copy(e.center).sub(qc))),this)}equals(e){return e.center.equals(this.center)&&e.radius===this.radius}clone(){return new this.constructor().copy(this)}toJSON(){return{radius:this.radius,center:this.center.toArray()}}fromJSON(e){return this.radius=e.radius,this.center.fromArray(e.center),this}},Dm=0,Wn=new pt,Xc=new Ft,sr=new k,Un=new en,Qr=new en,dn=new k,bt=class s extends Xn{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:Dm++}),this.uuid=ii(),this.name="",this.type="BufferGeometry",this.index=null,this.indirect=null,this.indirectOffset=0,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={},this._transformed=!1}getIndex(){return this.index}setIndex(e){return Array.isArray(e)?this.index=new(nm(e)?pa:fa)(e,1):this.index=e,this}setIndirect(e,t=0){return this.indirect=e,this.indirectOffset=t,this}getIndirect(){return this.indirect}getAttribute(e){return this.attributes[e]}setAttribute(e,t){return this.attributes[e]=t,this}deleteAttribute(e){return delete this.attributes[e],this}hasAttribute(e){return this.attributes[e]!==void 0}addGroup(e,t,n=0){this.groups.push({start:e,count:t,materialIndex:n})}clearGroups(){this.groups=[]}setDrawRange(e,t){this.drawRange.start=e,this.drawRange.count=t}applyMatrix4(e){let t=this.attributes.position;t!==void 0&&(t.applyMatrix4(e),t.needsUpdate=!0);let n=this.attributes.normal;if(n!==void 0){let r=new mt().getNormalMatrix(e);n.applyNormalMatrix(r),n.needsUpdate=!0}let i=this.attributes.tangent;return i!==void 0&&(i.transformDirection(e),i.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this._transformed=!0,this}applyQuaternion(e){return Wn.makeRotationFromQuaternion(e),this.applyMatrix4(Wn),this}rotateX(e){return Wn.makeRotationX(e),this.applyMatrix4(Wn),this}rotateY(e){return Wn.makeRotationY(e),this.applyMatrix4(Wn),this}rotateZ(e){return Wn.makeRotationZ(e),this.applyMatrix4(Wn),this}translate(e,t,n){return Wn.makeTranslation(e,t,n),this.applyMatrix4(Wn),this}scale(e,t,n){return Wn.makeScale(e,t,n),this.applyMatrix4(Wn),this}lookAt(e){return Xc.lookAt(e),Xc.updateMatrix(),this.applyMatrix4(Xc.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(sr).negate(),this.translate(sr.x,sr.y,sr.z),this}setFromPoints(e){let t=this.getAttribute("position");if(t===void 0){let n=[];for(let i=0,r=e.length;i<r;i++){let a=e[i];n.push(a.x,a.y,a.z||0)}this.setAttribute("position",new ut(n,3))}else{let n=Math.min(e.length,t.count);for(let i=0;i<n;i++){let r=e[i];t.setXYZ(i,r.x,r.y,r.z||0)}e.length>t.count&&nt("BufferGeometry: Buffer size too small for points data. Use .dispose() and create a new geometry."),t.needsUpdate=!0}return this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new en);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){ht("BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new k(-1/0,-1/0,-1/0),new k(1/0,1/0,1/0));return}if(e!==void 0){if(this.boundingBox.setFromBufferAttribute(e),t)for(let n=0,i=t.length;n<i;n++){let r=t[n];Un.setFromBufferAttribute(r),this.morphTargetsRelative?(dn.addVectors(this.boundingBox.min,Un.min),this.boundingBox.expandByPoint(dn),dn.addVectors(this.boundingBox.max,Un.max),this.boundingBox.expandByPoint(dn)):(this.boundingBox.expandByPoint(Un.min),this.boundingBox.expandByPoint(Un.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&ht('BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new xn);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){ht("BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new k,1/0);return}if(e){let n=this.boundingSphere.center;if(Un.setFromBufferAttribute(e),t)for(let r=0,a=t.length;r<a;r++){let o=t[r];Qr.setFromBufferAttribute(o),this.morphTargetsRelative?(dn.addVectors(Un.min,Qr.min),Un.expandByPoint(dn),dn.addVectors(Un.max,Qr.max),Un.expandByPoint(dn)):(Un.expandByPoint(Qr.min),Un.expandByPoint(Qr.max))}Un.getCenter(n);let i=0;for(let r=0,a=e.count;r<a;r++)dn.fromBufferAttribute(e,r),i=Math.max(i,n.distanceToSquared(dn));if(t)for(let r=0,a=t.length;r<a;r++){let o=t[r],l=this.morphTargetsRelative;for(let c=0,h=o.count;c<h;c++)dn.fromBufferAttribute(o,c),l&&(sr.fromBufferAttribute(e,c),dn.add(sr)),i=Math.max(i,n.distanceToSquared(dn))}this.boundingSphere.radius=Math.sqrt(i),isNaN(this.boundingSphere.radius)&&ht('BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let e=this.index,t=this.attributes;if(e===null||t.position===void 0||t.normal===void 0||t.uv===void 0){ht("BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let n=t.position,i=t.normal,r=t.uv,a=this.getAttribute("tangent");(a===void 0||a.count!==n.count)&&(a=new Mt(new Float32Array(4*n.count),4),this.setAttribute("tangent",a));let o=[],l=[];for(let x=0;x<n.count;x++)o[x]=new k,l[x]=new k;let c=new k,h=new k,u=new k,d=new Be,p=new Be,g=new Be,_=new k,m=new k;function f(x,C,w){c.fromBufferAttribute(n,x),h.fromBufferAttribute(n,C),u.fromBufferAttribute(n,w),d.fromBufferAttribute(r,x),p.fromBufferAttribute(r,C),g.fromBufferAttribute(r,w),h.sub(c),u.sub(c),p.sub(d),g.sub(d);let I=1/(p.x*g.y-g.x*p.y);isFinite(I)&&(_.copy(h).multiplyScalar(g.y).addScaledVector(u,-p.y).multiplyScalar(I),m.copy(u).multiplyScalar(p.x).addScaledVector(h,-g.x).multiplyScalar(I),o[x].add(_),o[C].add(_),o[w].add(_),l[x].add(m),l[C].add(m),l[w].add(m))}let v=this.groups;v.length===0&&(v=[{start:0,count:e.count}]);for(let x=0,C=v.length;x<C;++x){let w=v[x],I=w.start,y=w.count;for(let N=I,D=I+y;N<D;N+=3)f(e.getX(N+0),e.getX(N+1),e.getX(N+2))}let S=new k,b=new k,M=new k,E=new k;function P(x){M.fromBufferAttribute(i,x),E.copy(M);let C=o[x];S.copy(C),S.sub(M.multiplyScalar(M.dot(C))).normalize(),b.crossVectors(E,C);let I=b.dot(l[x])<0?-1:1;a.setXYZW(x,S.x,S.y,S.z,I)}for(let x=0,C=v.length;x<C;++x){let w=v[x],I=w.start,y=w.count;for(let N=I,D=I+y;N<D;N+=3)P(e.getX(N+0)),P(e.getX(N+1)),P(e.getX(N+2))}this._transformed=!0}computeVertexNormals(){let e=this.index,t=this.getAttribute("position");if(t!==void 0){let n=this.getAttribute("normal");if(n===void 0||n.count!==t.count)n=new Mt(new Float32Array(t.count*3),3),this.setAttribute("normal",n);else for(let d=0,p=n.count;d<p;d++)n.setXYZ(d,0,0,0);let i=new k,r=new k,a=new k,o=new k,l=new k,c=new k,h=new k,u=new k;if(e)for(let d=0,p=e.count;d<p;d+=3){let g=e.getX(d+0),_=e.getX(d+1),m=e.getX(d+2);i.fromBufferAttribute(t,g),r.fromBufferAttribute(t,_),a.fromBufferAttribute(t,m),h.subVectors(a,r),u.subVectors(i,r),h.cross(u),o.fromBufferAttribute(n,g),l.fromBufferAttribute(n,_),c.fromBufferAttribute(n,m),o.add(h),l.add(h),c.add(h),n.setXYZ(g,o.x,o.y,o.z),n.setXYZ(_,l.x,l.y,l.z),n.setXYZ(m,c.x,c.y,c.z)}else for(let d=0,p=t.count;d<p;d+=3)i.fromBufferAttribute(t,d+0),r.fromBufferAttribute(t,d+1),a.fromBufferAttribute(t,d+2),h.subVectors(a,r),u.subVectors(i,r),h.cross(u),n.setXYZ(d+0,h.x,h.y,h.z),n.setXYZ(d+1,h.x,h.y,h.z),n.setXYZ(d+2,h.x,h.y,h.z);this.normalizeNormals(),n.needsUpdate=!0}}normalizeNormals(){let e=this.attributes.normal;for(let t=0,n=e.count;t<n;t++)dn.fromBufferAttribute(e,t),dn.normalize(),e.setXYZ(t,dn.x,dn.y,dn.z)}toNonIndexed(){function e(o,l){let c=o.array,h=o.itemSize,u=o.normalized,d=new c.constructor(l.length*h),p=0,g=0;for(let _=0,m=l.length;_<m;_++){o.isInterleavedBufferAttribute?p=l[_]*o.data.stride+o.offset:p=l[_]*h;for(let f=0;f<h;f++)d[g++]=c[p++]}return new Mt(d,h,u)}if(this.index===null)return nt("BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let t=new s,n=this.index.array,i=this.attributes;for(let o in i){let l=i[o],c=e(l,n);t.setAttribute(o,c)}let r=this.morphAttributes;for(let o in r){let l=[],c=r[o];for(let h=0,u=c.length;h<u;h++){let d=c[h],p=e(d,n);l.push(p)}t.morphAttributes[o]=l}t.morphTargetsRelative=this.morphTargetsRelative;let a=this.groups;for(let o=0,l=a.length;o<l;o++){let c=a[o];t.addGroup(c.start,c.count,c.materialIndex)}return t}toJSON(){let e={metadata:{version:4.7,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(e.uuid=this.uuid,e.type=this.parameters!==void 0&&this._transformed===!0?"BufferGeometry":this.type,e.name=this.name,Object.keys(this.userData).length>0&&(e.userData=this.userData),this.parameters!==void 0&&this._transformed!==!0){let l=this.parameters;for(let c in l)l[c]!==void 0&&(e[c]=l[c]);return e}e.data={attributes:{}};let t=this.index;t!==null&&(e.data.index={type:t.array.constructor.name,array:Array.prototype.slice.call(t.array)});let n=this.attributes;for(let l in n){let c=n[l];e.data.attributes[l]=c.toJSON(e.data)}let i={},r=!1;for(let l in this.morphAttributes){let c=this.morphAttributes[l],h=[];for(let u=0,d=c.length;u<d;u++){let p=c[u];h.push(p.toJSON(e.data))}h.length>0&&(i[l]=h,r=!0)}r&&(e.data.morphAttributes=i,e.data.morphTargetsRelative=this.morphTargetsRelative);let a=this.groups;a.length>0&&(e.data.groups=JSON.parse(JSON.stringify(a)));let o=this.boundingSphere;return o!==null&&(e.data.boundingSphere=o.toJSON()),e}clone(){return new this.constructor().copy(this)}copy(e){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let t={};this.name=e.name;let n=e.index;n!==null&&this.setIndex(n.clone());let i=e.attributes;for(let c in i){let h=i[c];this.setAttribute(c,h.clone(t))}let r=e.morphAttributes;for(let c in r){let h=[],u=r[c];for(let d=0,p=u.length;d<p;d++)h.push(u[d].clone(t));this.morphAttributes[c]=h}this.morphTargetsRelative=e.morphTargetsRelative;let a=e.groups;for(let c=0,h=a.length;c<h;c++){let u=a[c];this.addGroup(u.start,u.count,u.materialIndex)}let o=e.boundingBox;o!==null&&(this.boundingBox=o.clone());let l=e.boundingSphere;return l!==null&&(this.boundingSphere=l.clone()),this.drawRange.start=e.drawRange.start,this.drawRange.count=e.drawRange.count,this.userData=e.userData,this._transformed=e._transformed,this}dispose(){this.dispatchEvent({type:"dispose"})}},xr=class{constructor(e,t){this.isInterleavedBuffer=!0,this.array=e,this.stride=t,this.count=e!==void 0?e.length/t:0,this.usage=Rh,this.updateRanges=[],this.version=0,this.uuid=ii()}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.array=new e.array.constructor(e.array),this.count=e.count,this.stride=e.stride,this.usage=e.usage,this}copyAt(e,t,n){e*=this.stride,n*=t.stride;for(let i=0,r=this.stride;i<r;i++)this.array[e+i]=t.array[n+i];return this}set(e,t=0){return this.array.set(e,t),this}clone(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=ii()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=this.array.slice(0).buffer);let t=new this.array.constructor(e.arrayBuffers[this.array.buffer._uuid]),n=new this.constructor(t,this.stride);return n.setUsage(this.usage),n}onUpload(e){return this.onUploadCallback=e,this}toJSON(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=ii()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=Array.from(new Uint32Array(this.array.buffer)));let t={uuid:this.uuid,buffer:this.array.buffer._uuid,type:this.array.constructor.name,stride:this.stride};return t.usage=this.usage,t}},Sn=new k,vr=class s{constructor(e,t,n,i=!1){this.isInterleavedBufferAttribute=!0,this.name="",this.data=e,this.itemSize=t,this.offset=n,this.normalized=i}get count(){return this.data.count}get array(){return this.data.array}set needsUpdate(e){this.data.needsUpdate=e}applyMatrix4(e){for(let t=0,n=this.data.count;t<n;t++)Sn.fromBufferAttribute(this,t),Sn.applyMatrix4(e),this.setXYZ(t,Sn.x,Sn.y,Sn.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)Sn.fromBufferAttribute(this,t),Sn.applyNormalMatrix(e),this.setXYZ(t,Sn.x,Sn.y,Sn.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)Sn.fromBufferAttribute(this,t),Sn.transformDirection(e),this.setXYZ(t,Sn.x,Sn.y,Sn.z);return this}getComponent(e,t){let n=this.array[e*this.data.stride+this.offset+t];return this.normalized&&(n=ti(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=Ut(n,this.array)),this.data.array[e*this.data.stride+this.offset+t]=n,this}setX(e,t){return this.normalized&&(t=Ut(t,this.array)),this.data.array[e*this.data.stride+this.offset]=t,this}setY(e,t){return this.normalized&&(t=Ut(t,this.array)),this.data.array[e*this.data.stride+this.offset+1]=t,this}setZ(e,t){return this.normalized&&(t=Ut(t,this.array)),this.data.array[e*this.data.stride+this.offset+2]=t,this}setW(e,t){return this.normalized&&(t=Ut(t,this.array)),this.data.array[e*this.data.stride+this.offset+3]=t,this}getX(e){let t=this.data.array[e*this.data.stride+this.offset];return this.normalized&&(t=ti(t,this.array)),t}getY(e){let t=this.data.array[e*this.data.stride+this.offset+1];return this.normalized&&(t=ti(t,this.array)),t}getZ(e){let t=this.data.array[e*this.data.stride+this.offset+2];return this.normalized&&(t=ti(t,this.array)),t}getW(e){let t=this.data.array[e*this.data.stride+this.offset+3];return this.normalized&&(t=ti(t,this.array)),t}setXY(e,t,n){return e=e*this.data.stride+this.offset,this.normalized&&(t=Ut(t,this.array),n=Ut(n,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this}setXYZ(e,t,n,i){return e=e*this.data.stride+this.offset,this.normalized&&(t=Ut(t,this.array),n=Ut(n,this.array),i=Ut(i,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e=e*this.data.stride+this.offset,this.normalized&&(t=Ut(t,this.array),n=Ut(n,this.array),i=Ut(i,this.array),r=Ut(r,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this.data.array[e+3]=r,this}clone(e){if(e===void 0){ha("InterleavedBufferAttribute.clone(): Cloning an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return new Mt(new this.array.constructor(t),this.itemSize,this.normalized)}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.clone(e)),new s(e.interleavedBuffers[this.data.uuid],this.itemSize,this.offset,this.normalized)}toJSON(e){if(e===void 0){ha("InterleavedBufferAttribute.toJSON(): Serializing an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return{itemSize:this.itemSize,type:this.array.constructor.name,array:t,normalized:this.normalized}}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.toJSON(e)),{isInterleavedBufferAttribute:!0,itemSize:this.itemSize,data:this.data.uuid,offset:this.offset,normalized:this.normalized}}},jc=new k,Lm=new k,Fm=new mt,On=class{constructor(e=new k(1,0,0),t=0){this.isPlane=!0,this.normal=e,this.constant=t}set(e,t){return this.normal.copy(e),this.constant=t,this}setComponents(e,t,n,i){return this.normal.set(e,t,n),this.constant=i,this}setFromNormalAndCoplanarPoint(e,t){return this.normal.copy(e),this.constant=-t.dot(this.normal),this}setFromCoplanarPoints(e,t,n){let i=jc.subVectors(n,t).cross(Lm.subVectors(e,t)).normalize();return this.setFromNormalAndCoplanarPoint(i,e),this}copy(e){return this.normal.copy(e.normal),this.constant=e.constant,this}normalize(){let e=1/this.normal.length();return this.normal.multiplyScalar(e),this.constant*=e,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(e){return this.normal.dot(e)+this.constant}distanceToSphere(e){return this.distanceToPoint(e.center)-e.radius}projectPoint(e,t){return t.copy(e).addScaledVector(this.normal,-this.distanceToPoint(e))}intersectLine(e,t,n=!0){let i=e.delta(jc),r=this.normal.dot(i);if(r===0)return this.distanceToPoint(e.start)===0?t.copy(e.start):null;let a=-(e.start.dot(this.normal)+this.constant)/r;return n===!0&&(a<0||a>1)?null:t.copy(e.start).addScaledVector(i,a)}intersectsLine(e){let t=this.distanceToPoint(e.start),n=this.distanceToPoint(e.end);return t<0&&n>0||n<0&&t>0}intersectsBox(e){return e.intersectsPlane(this)}intersectsSphere(e){return e.intersectsPlane(this)}coplanarPoint(e){return e.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(e,t){let n=t||Fm.getNormalMatrix(e),i=this.coplanarPoint(jc).applyMatrix4(e),r=this.normal.applyMatrix3(n).normalize();return this.constant=-i.dot(r),this}translate(e){return this.constant-=e.dot(this.normal),this}equals(e){return e.normal.equals(this.normal)&&e.constant===this.constant}clone(){return new this.constructor().copy(this)}toJSON(){return{normal:this.normal.toArray(),constant:this.constant}}fromJSON(e){return this.normal.fromArray(e.normal),this.constant=e.constant,this}},Nm=0,En=class extends Xn{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:Nm++}),this.uuid=ii(),this.name="",this.type="Material",this.blending=Dr,this.side=jn,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=xh,this.blendDst=vh,this.blendEquation=Us,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new Le(0,0,0),this.blendAlpha=0,this.depthFunc=hr,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=cf,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=Oo,this.stencilZFail=Oo,this.stencilZPass=Oo,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.allowOverride=!0,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(e){this._alphaTest>0!=e>0&&this.version++,this._alphaTest=e}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(e){if(e!==void 0)for(let t in e){let n=e[t];if(n===void 0){nt(`Material: parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){nt(`Material: '${t}' is not a property of THREE.${this.type}.`);continue}i&&i.isColor?i.set(n):i&&i.isVector2&&n&&n.isVector2||i&&i.isEuler&&n&&n.isEuler||i&&i.isVector3&&n&&n.isVector3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";t&&(e={textures:{},images:{}});let n={metadata:{version:4.7,type:"Material",generator:"Material.toJSON"}};n.uuid=this.uuid,n.type=this.type,n.blending=this.blending,n.side=this.side,n.shadowSide=this.shadowSide,n.vertexColors=this.vertexColors,n.opacity=this.opacity,n.transparent=this.transparent,n.blendSrc=this.blendSrc,n.blendDst=this.blendDst,n.blendEquation=this.blendEquation,n.blendSrcAlpha=this.blendSrcAlpha,n.blendDstAlpha=this.blendDstAlpha,n.blendEquationAlpha=this.blendEquationAlpha,n.blendColor=this.blendColor.getHex(),n.blendAlpha=this.blendAlpha,n.depthFunc=this.depthFunc,n.depthTest=this.depthTest,n.depthWrite=this.depthWrite,n.colorWrite=this.colorWrite,n.clipIntersection=this.clipIntersection,n.clipShadows=this.clipShadows,n.stencilWriteMask=this.stencilWriteMask,n.stencilFunc=this.stencilFunc,n.stencilRef=this.stencilRef,n.stencilFuncMask=this.stencilFuncMask,n.stencilFail=this.stencilFail,n.stencilZFail=this.stencilZFail,n.stencilZPass=this.stencilZPass,n.stencilWrite=this.stencilWrite,n.polygonOffset=this.polygonOffset,n.polygonOffsetFactor=this.polygonOffsetFactor,n.polygonOffsetUnits=this.polygonOffsetUnits,n.dithering=this.dithering,n.alphaTest=this.alphaTest,n.alphaHash=this.alphaHash,n.alphaToCoverage=this.alphaToCoverage,n.premultipliedAlpha=this.premultipliedAlpha,n.forceSinglePass=this.forceSinglePass,n.allowOverride=this.allowOverride,n.visible=this.visible,n.toneMapped=this.toneMapped,n.name=this.name,this.color&&this.color.isColor&&(n.color=this.color.getHex()),this.roughness!==void 0&&(n.roughness=this.roughness),this.metalness!==void 0&&(n.metalness=this.metalness),this.sheen!==void 0&&(n.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(n.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(n.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(n.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&(n.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(n.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(n.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(n.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(n.shininess=this.shininess),this.clearcoat!==void 0&&(n.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(n.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(n.clearcoatMap=this.clearcoatMap.toJSON(e).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(n.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(e).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(n.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(e).uuid,n.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.sheenColorMap&&this.sheenColorMap.isTexture&&(n.sheenColorMap=this.sheenColorMap.toJSON(e).uuid),this.sheenRoughnessMap&&this.sheenRoughnessMap.isTexture&&(n.sheenRoughnessMap=this.sheenRoughnessMap.toJSON(e).uuid),this.dispersion!==void 0&&(n.dispersion=this.dispersion),this.retroreflectivity!==void 0&&(n.retroreflectivity=this.retroreflectivity),this.iridescence!==void 0&&(n.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(n.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(n.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(n.iridescenceMap=this.iridescenceMap.toJSON(e).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(n.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(e).uuid),this.anisotropy!==void 0&&(n.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(n.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(n.anisotropyMap=this.anisotropyMap.toJSON(e).uuid),this.map&&this.map.isTexture&&(n.map=this.map.toJSON(e).uuid),this.matcap&&this.matcap.isTexture&&(n.matcap=this.matcap.toJSON(e).uuid),this.alphaMap&&this.alphaMap.isTexture&&(n.alphaMap=this.alphaMap.toJSON(e).uuid),this.lightMap&&this.lightMap.isTexture&&(n.lightMap=this.lightMap.toJSON(e).uuid,n.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(n.aoMap=this.aoMap.toJSON(e).uuid,n.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(n.bumpMap=this.bumpMap.toJSON(e).uuid,n.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(n.normalMap=this.normalMap.toJSON(e).uuid,n.normalMapType=this.normalMapType,n.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(n.displacementMap=this.displacementMap.toJSON(e).uuid,n.displacementScale=this.displacementScale,n.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(n.roughnessMap=this.roughnessMap.toJSON(e).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(n.metalnessMap=this.metalnessMap.toJSON(e).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(n.emissiveMap=this.emissiveMap.toJSON(e).uuid),this.specularMap&&this.specularMap.isTexture&&(n.specularMap=this.specularMap.toJSON(e).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(n.specularIntensityMap=this.specularIntensityMap.toJSON(e).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(n.specularColorMap=this.specularColorMap.toJSON(e).uuid),this.envMap&&this.envMap.isTexture&&(n.envMap=this.envMap.toJSON(e).uuid,this.combine!==void 0&&(n.combine=this.combine)),this.envMapRotation!==void 0&&(n.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(n.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(n.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(n.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(n.gradientMap=this.gradientMap.toJSON(e).uuid),this.transmission!==void 0&&(n.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(n.transmissionMap=this.transmissionMap.toJSON(e).uuid),this.thickness!==void 0&&(n.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(n.thicknessMap=this.thicknessMap.toJSON(e).uuid),this.attenuationDistance!==void 0&&(n.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(n.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(n.size=this.size),this.sizeAttenuation!==void 0&&(n.sizeAttenuation=this.sizeAttenuation),Array.isArray(this.clippingPlanes)&&this.clippingPlanes.length>0&&(n.clippingPlanes=this.clippingPlanes.map(r=>r.toJSON())),this.rotation!==void 0&&(n.rotation=this.rotation),this.depthPacking!==void 0&&(n.depthPacking=this.depthPacking),this.linewidth!==void 0&&(n.linewidth=this.linewidth),this.linecap!==void 0&&(n.linecap=this.linecap),this.linejoin!==void 0&&(n.linejoin=this.linejoin),this.dashSize!==void 0&&(n.dashSize=this.dashSize),this.gapSize!==void 0&&(n.gapSize=this.gapSize),this.scale!==void 0&&(n.scale=this.scale),this.wireframe!==void 0&&(n.wireframe=this.wireframe),this.wireframeLinewidth!==void 0&&(n.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!==void 0&&(n.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!==void 0&&(n.wireframeLinejoin=this.wireframeLinejoin),this.flatShading!==void 0&&(n.flatShading=this.flatShading),this.fog!==void 0&&(n.fog=this.fog),Object.keys(this.userData).length>0&&(n.userData=this.userData);function i(r){let a=[];for(let o in r){let l=r[o];delete l.metadata,a.push(l)}return a}if(t){let r=i(e.textures),a=i(e.images);r.length>0&&(n.textures=r),a.length>0&&(n.images=a)}return n}fromJSON(e,t){if(e.uuid!==void 0&&(this.uuid=e.uuid),e.name!==void 0&&(this.name=e.name),e.color!==void 0&&this.color!==void 0&&this.color.setHex(e.color),e.roughness!==void 0&&(this.roughness=e.roughness),e.metalness!==void 0&&(this.metalness=e.metalness),e.sheen!==void 0&&(this.sheen=e.sheen),e.sheenColor!==void 0&&(this.sheenColor=new Le().setHex(e.sheenColor)),e.sheenRoughness!==void 0&&(this.sheenRoughness=e.sheenRoughness),e.emissive!==void 0&&this.emissive!==void 0&&this.emissive.setHex(e.emissive),e.specular!==void 0&&this.specular!==void 0&&this.specular.setHex(e.specular),e.specularIntensity!==void 0&&(this.specularIntensity=e.specularIntensity),e.specularColor!==void 0&&this.specularColor!==void 0&&this.specularColor.setHex(e.specularColor),e.shininess!==void 0&&(this.shininess=e.shininess),e.clearcoat!==void 0&&(this.clearcoat=e.clearcoat),e.clearcoatRoughness!==void 0&&(this.clearcoatRoughness=e.clearcoatRoughness),e.dispersion!==void 0&&(this.dispersion=e.dispersion),e.retroreflectivity!==void 0&&(this.retroreflectivity=e.retroreflectivity),e.iridescence!==void 0&&(this.iridescence=e.iridescence),e.iridescenceIOR!==void 0&&(this.iridescenceIOR=e.iridescenceIOR),e.iridescenceThicknessRange!==void 0&&(this.iridescenceThicknessRange=e.iridescenceThicknessRange),e.transmission!==void 0&&(this.transmission=e.transmission),e.thickness!==void 0&&(this.thickness=e.thickness),e.attenuationDistance!==void 0&&(this.attenuationDistance=e.attenuationDistance),e.attenuationColor!==void 0&&this.attenuationColor!==void 0&&this.attenuationColor.setHex(e.attenuationColor),e.anisotropy!==void 0&&(this.anisotropy=e.anisotropy),e.anisotropyRotation!==void 0&&(this.anisotropyRotation=e.anisotropyRotation),e.fog!==void 0&&(this.fog=e.fog),e.flatShading!==void 0&&(this.flatShading=e.flatShading),e.blending!==void 0&&(this.blending=e.blending),e.combine!==void 0&&(this.combine=e.combine),e.side!==void 0&&(this.side=e.side),e.shadowSide!==void 0&&(this.shadowSide=e.shadowSide),e.opacity!==void 0&&(this.opacity=e.opacity),e.transparent!==void 0&&(this.transparent=e.transparent),e.alphaTest!==void 0&&(this.alphaTest=e.alphaTest),e.alphaHash!==void 0&&(this.alphaHash=e.alphaHash),e.depthFunc!==void 0&&(this.depthFunc=e.depthFunc),e.depthTest!==void 0&&(this.depthTest=e.depthTest),e.depthWrite!==void 0&&(this.depthWrite=e.depthWrite),e.colorWrite!==void 0&&(this.colorWrite=e.colorWrite),e.clippingPlanes!==void 0&&(this.clippingPlanes=e.clippingPlanes.map(n=>new On().fromJSON(n))),e.clipIntersection!==void 0&&(this.clipIntersection=e.clipIntersection),e.clipShadows!==void 0&&(this.clipShadows=e.clipShadows),e.depthPacking!==void 0&&(this.depthPacking=e.depthPacking),e.blendSrc!==void 0&&(this.blendSrc=e.blendSrc),e.blendDst!==void 0&&(this.blendDst=e.blendDst),e.blendEquation!==void 0&&(this.blendEquation=e.blendEquation),e.blendSrcAlpha!==void 0&&(this.blendSrcAlpha=e.blendSrcAlpha),e.blendDstAlpha!==void 0&&(this.blendDstAlpha=e.blendDstAlpha),e.blendEquationAlpha!==void 0&&(this.blendEquationAlpha=e.blendEquationAlpha),e.blendColor!==void 0&&this.blendColor!==void 0&&this.blendColor.setHex(e.blendColor),e.blendAlpha!==void 0&&(this.blendAlpha=e.blendAlpha),e.stencilWriteMask!==void 0&&(this.stencilWriteMask=e.stencilWriteMask),e.stencilFunc!==void 0&&(this.stencilFunc=e.stencilFunc),e.stencilRef!==void 0&&(this.stencilRef=e.stencilRef),e.stencilFuncMask!==void 0&&(this.stencilFuncMask=e.stencilFuncMask),e.stencilFail!==void 0&&(this.stencilFail=e.stencilFail),e.stencilZFail!==void 0&&(this.stencilZFail=e.stencilZFail),e.stencilZPass!==void 0&&(this.stencilZPass=e.stencilZPass),e.stencilWrite!==void 0&&(this.stencilWrite=e.stencilWrite),e.wireframe!==void 0&&(this.wireframe=e.wireframe),e.wireframeLinewidth!==void 0&&(this.wireframeLinewidth=e.wireframeLinewidth),e.wireframeLinecap!==void 0&&(this.wireframeLinecap=e.wireframeLinecap),e.wireframeLinejoin!==void 0&&(this.wireframeLinejoin=e.wireframeLinejoin),e.rotation!==void 0&&(this.rotation=e.rotation),e.linewidth!==void 0&&(this.linewidth=e.linewidth),e.linecap!==void 0&&(this.linecap=e.linecap),e.linejoin!==void 0&&(this.linejoin=e.linejoin),e.dashSize!==void 0&&(this.dashSize=e.dashSize),e.gapSize!==void 0&&(this.gapSize=e.gapSize),e.scale!==void 0&&(this.scale=e.scale),e.polygonOffset!==void 0&&(this.polygonOffset=e.polygonOffset),e.polygonOffsetFactor!==void 0&&(this.polygonOffsetFactor=e.polygonOffsetFactor),e.polygonOffsetUnits!==void 0&&(this.polygonOffsetUnits=e.polygonOffsetUnits),e.dithering!==void 0&&(this.dithering=e.dithering),e.alphaToCoverage!==void 0&&(this.alphaToCoverage=e.alphaToCoverage),e.premultipliedAlpha!==void 0&&(this.premultipliedAlpha=e.premultipliedAlpha),e.forceSinglePass!==void 0&&(this.forceSinglePass=e.forceSinglePass),e.allowOverride!==void 0&&(this.allowOverride=e.allowOverride),e.visible!==void 0&&(this.visible=e.visible),e.toneMapped!==void 0&&(this.toneMapped=e.toneMapped),e.userData!==void 0&&(this.userData=e.userData),e.vertexColors!==void 0&&(typeof e.vertexColors=="number"?this.vertexColors=e.vertexColors>0:this.vertexColors=e.vertexColors),e.size!==void 0&&(this.size=e.size),e.sizeAttenuation!==void 0&&(this.sizeAttenuation=e.sizeAttenuation),e.map!==void 0&&(this.map=t[e.map]||null),e.matcap!==void 0&&(this.matcap=t[e.matcap]||null),e.alphaMap!==void 0&&(this.alphaMap=t[e.alphaMap]||null),e.bumpMap!==void 0&&(this.bumpMap=t[e.bumpMap]||null),e.bumpScale!==void 0&&(this.bumpScale=e.bumpScale),e.normalMap!==void 0&&(this.normalMap=t[e.normalMap]||null),e.normalMapType!==void 0&&(this.normalMapType=e.normalMapType),e.normalScale!==void 0){let n=e.normalScale;Array.isArray(n)===!1&&(n=[n,n]),this.normalScale=new Be().fromArray(n)}return e.displacementMap!==void 0&&(this.displacementMap=t[e.displacementMap]||null),e.displacementScale!==void 0&&(this.displacementScale=e.displacementScale),e.displacementBias!==void 0&&(this.displacementBias=e.displacementBias),e.roughnessMap!==void 0&&(this.roughnessMap=t[e.roughnessMap]||null),e.metalnessMap!==void 0&&(this.metalnessMap=t[e.metalnessMap]||null),e.emissiveMap!==void 0&&(this.emissiveMap=t[e.emissiveMap]||null),e.emissiveIntensity!==void 0&&(this.emissiveIntensity=e.emissiveIntensity),e.specularMap!==void 0&&(this.specularMap=t[e.specularMap]||null),e.specularIntensityMap!==void 0&&(this.specularIntensityMap=t[e.specularIntensityMap]||null),e.specularColorMap!==void 0&&(this.specularColorMap=t[e.specularColorMap]||null),e.envMap!==void 0&&(this.envMap=t[e.envMap]||null),e.envMapRotation!==void 0&&this.envMapRotation.fromArray(e.envMapRotation),e.envMapIntensity!==void 0&&(this.envMapIntensity=e.envMapIntensity),e.reflectivity!==void 0&&(this.reflectivity=e.reflectivity),e.refractionRatio!==void 0&&(this.refractionRatio=e.refractionRatio),e.lightMap!==void 0&&(this.lightMap=t[e.lightMap]||null),e.lightMapIntensity!==void 0&&(this.lightMapIntensity=e.lightMapIntensity),e.aoMap!==void 0&&(this.aoMap=t[e.aoMap]||null),e.aoMapIntensity!==void 0&&(this.aoMapIntensity=e.aoMapIntensity),e.gradientMap!==void 0&&(this.gradientMap=t[e.gradientMap]||null),e.clearcoatMap!==void 0&&(this.clearcoatMap=t[e.clearcoatMap]||null),e.clearcoatRoughnessMap!==void 0&&(this.clearcoatRoughnessMap=t[e.clearcoatRoughnessMap]||null),e.clearcoatNormalMap!==void 0&&(this.clearcoatNormalMap=t[e.clearcoatNormalMap]||null),e.clearcoatNormalScale!==void 0&&(this.clearcoatNormalScale=new Be().fromArray(e.clearcoatNormalScale)),e.iridescenceMap!==void 0&&(this.iridescenceMap=t[e.iridescenceMap]||null),e.iridescenceThicknessMap!==void 0&&(this.iridescenceThicknessMap=t[e.iridescenceThicknessMap]||null),e.transmissionMap!==void 0&&(this.transmissionMap=t[e.transmissionMap]||null),e.thicknessMap!==void 0&&(this.thicknessMap=t[e.thicknessMap]||null),e.anisotropyMap!==void 0&&(this.anisotropyMap=t[e.anisotropyMap]||null),e.sheenColorMap!==void 0&&(this.sheenColorMap=t[e.sheenColorMap]||null),e.sheenRoughnessMap!==void 0&&(this.sheenRoughnessMap=t[e.sheenRoughnessMap]||null),this}clone(){return new this.constructor().copy(this)}copy(e){this.name=e.name,this.blending=e.blending,this.side=e.side,this.vertexColors=e.vertexColors,this.opacity=e.opacity,this.transparent=e.transparent,this.blendSrc=e.blendSrc,this.blendDst=e.blendDst,this.blendEquation=e.blendEquation,this.blendSrcAlpha=e.blendSrcAlpha,this.blendDstAlpha=e.blendDstAlpha,this.blendEquationAlpha=e.blendEquationAlpha,this.blendColor.copy(e.blendColor),this.blendAlpha=e.blendAlpha,this.depthFunc=e.depthFunc,this.depthTest=e.depthTest,this.depthWrite=e.depthWrite,this.stencilWriteMask=e.stencilWriteMask,this.stencilFunc=e.stencilFunc,this.stencilRef=e.stencilRef,this.stencilFuncMask=e.stencilFuncMask,this.stencilFail=e.stencilFail,this.stencilZFail=e.stencilZFail,this.stencilZPass=e.stencilZPass,this.stencilWrite=e.stencilWrite;let t=e.clippingPlanes,n=null;if(t!==null){let i=t.length;n=new Array(i);for(let r=0;r!==i;++r)n[r]=t[r].clone()}return this.clippingPlanes=n,this.clipIntersection=e.clipIntersection,this.clipShadows=e.clipShadows,this.shadowSide=e.shadowSide,this.colorWrite=e.colorWrite,this.precision=e.precision,this.polygonOffset=e.polygonOffset,this.polygonOffsetFactor=e.polygonOffsetFactor,this.polygonOffsetUnits=e.polygonOffsetUnits,this.dithering=e.dithering,this.alphaTest=e.alphaTest,this.alphaHash=e.alphaHash,this.alphaToCoverage=e.alphaToCoverage,this.premultipliedAlpha=e.premultipliedAlpha,this.forceSinglePass=e.forceSinglePass,this.allowOverride=e.allowOverride,this.visible=e.visible,this.toneMapped=e.toneMapped,this.userData=JSON.parse(JSON.stringify(e.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(e){e===!0&&this.version++}};var Li=new k,Kc=new k,xo=new k,vo=new k,gi=class{constructor(e=new k,t=new k(0,0,-1)){this.origin=e,this.direction=t}set(e,t){return this.origin.copy(e),this.direction.copy(t),this}copy(e){return this.origin.copy(e.origin),this.direction.copy(e.direction),this}at(e,t){return t.copy(this.origin).addScaledVector(this.direction,e)}lookAt(e){return this.direction.copy(e).sub(this.origin).normalize(),this}recast(e){return this.origin.copy(this.at(e,Li)),this}closestPointToPoint(e,t){t.subVectors(e,this.origin);let n=t.dot(this.direction);return n<0?t.copy(this.origin):t.copy(this.origin).addScaledVector(this.direction,n)}distanceToPoint(e){return Math.sqrt(this.distanceSqToPoint(e))}distanceSqToPoint(e){let t=Li.subVectors(e,this.origin).dot(this.direction);return t<0?this.origin.distanceToSquared(e):(Li.copy(this.origin).addScaledVector(this.direction,t),Li.distanceToSquared(e))}distanceSqToSegment(e,t,n,i){Kc.copy(e).add(t).multiplyScalar(.5),xo.copy(t).sub(e).normalize(),vo.copy(this.origin).sub(Kc);let r=e.distanceTo(t)*.5,a=-this.direction.dot(xo),o=vo.dot(this.direction),l=-vo.dot(xo),c=vo.lengthSq(),h=Math.abs(1-a*a),u,d,p,g;if(h>0)if(u=a*l-o,d=a*o-l,g=r*h,u>=0)if(d>=-g)if(d<=g){let _=1/h;u*=_,d*=_,p=u*(u+a*d+2*o)+d*(a*u+d+2*l)+c}else d=r,u=Math.max(0,-(a*d+o)),p=-u*u+d*(d+2*l)+c;else d=-r,u=Math.max(0,-(a*d+o)),p=-u*u+d*(d+2*l)+c;else d<=-g?(u=Math.max(0,-(-a*r+o)),d=u>0?-r:Math.min(Math.max(-r,-l),r),p=-u*u+d*(d+2*l)+c):d<=g?(u=0,d=Math.min(Math.max(-r,-l),r),p=d*(d+2*l)+c):(u=Math.max(0,-(a*r+o)),d=u>0?r:Math.min(Math.max(-r,-l),r),p=-u*u+d*(d+2*l)+c);else d=a>0?-r:r,u=Math.max(0,-(a*d+o)),p=-u*u+d*(d+2*l)+c;return n&&n.copy(this.origin).addScaledVector(this.direction,u),i&&i.copy(Kc).addScaledVector(xo,d),p}intersectSphere(e,t){if(e.radius<0)return null;Li.subVectors(e.center,this.origin);let n=Li.dot(this.direction),i=Li.dot(Li)-n*n,r=e.radius*e.radius;if(i>r)return null;let a=Math.sqrt(r-i),o=n-a,l=n+a;return l<0?null:o<0?this.at(l,t):this.at(o,t)}intersectsSphere(e){return e.radius<0?!1:this.distanceSqToPoint(e.center)<=e.radius*e.radius}distanceToPlane(e){let t=e.normal.dot(this.direction);if(t===0)return e.distanceToPoint(this.origin)===0?0:null;let n=-(this.origin.dot(e.normal)+e.constant)/t;return n>=0?n:null}intersectPlane(e,t){let n=this.distanceToPlane(e);return n===null?null:this.at(n,t)}intersectsPlane(e){let t=e.distanceToPoint(this.origin);return t===0||e.normal.dot(this.direction)*t<0}intersectBox(e,t){let n,i,r,a,o,l,c=1/this.direction.x,h=1/this.direction.y,u=1/this.direction.z,d=this.origin;return c>=0?(n=(e.min.x-d.x)*c,i=(e.max.x-d.x)*c):(n=(e.max.x-d.x)*c,i=(e.min.x-d.x)*c),h>=0?(r=(e.min.y-d.y)*h,a=(e.max.y-d.y)*h):(r=(e.max.y-d.y)*h,a=(e.min.y-d.y)*h),n>a||r>i||((r>n||isNaN(n))&&(n=r),(a<i||isNaN(i))&&(i=a),u>=0?(o=(e.min.z-d.z)*u,l=(e.max.z-d.z)*u):(o=(e.max.z-d.z)*u,l=(e.min.z-d.z)*u),n>l||o>i)||((o>n||n!==n)&&(n=o),(l<i||i!==i)&&(i=l),i<0)?null:this.at(n>=0?n:i,t)}intersectsBox(e){return this.intersectBox(e,Li)!==null}intersectTriangle(e,t,n,i,r){let a=this.origin,o=this.direction,l=o.x,c=o.y,h=o.z,u=e.x-a.x,d=e.y-a.y,p=e.z-a.z,g=t.x-a.x,_=t.y-a.y,m=t.z-a.z,f=n.x-a.x,v=n.y-a.y,S=n.z-a.z,b=Math.abs(l),M=Math.abs(c),E=Math.abs(h),P,x,C,w,I,y,N,D,R,U,z,T;if(b>=M&&b>=E?(C=l,y=u,R=g,T=f,l>=0?(P=c,x=h,w=d,I=p,N=_,D=m,U=v,z=S):(P=h,x=c,w=p,I=d,N=m,D=_,U=S,z=v)):M>=E?(C=c,y=d,R=_,T=v,c>=0?(P=h,x=l,w=p,I=u,N=m,D=g,U=S,z=f):(P=l,x=h,w=u,I=p,N=g,D=m,U=f,z=S)):(C=h,y=p,R=m,T=S,h>=0?(P=l,x=c,w=u,I=d,N=g,D=_,U=f,z=v):(P=c,x=l,w=d,I=u,N=_,D=g,U=v,z=f)),C===0)return null;let G=P/C,X=x/C,Z=1/C,H=w-G*y,oe=I-X*y,ke=N-G*R,Fe=D-X*R,We=U-G*T,fe=z-X*T,re=We*Fe-fe*ke,Pe=H*fe-oe*We,be=ke*oe-Fe*H;if(i){if(re<0||Pe<0||be<0)return null}else if((re<0||Pe<0||be<0)&&(re>0||Pe>0||be>0))return null;let ge=re+Pe+be;if(ge===0)return null;let ie=Z*(re*y+Pe*R+be*T);return(ge>0?ie<0:ie>0)?null:this.at(ie/ge,r)}applyMatrix4(e){return this.origin.applyMatrix4(e),this.direction.transformDirection(e),this}equals(e){return e.origin.equals(this.origin)&&e.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},Lt=class extends En{constructor(e){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new Le(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Tn,this.combine=bl,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.fog=e.fog,this}},id=new pt,ys=new gi,_o=new xn,sd=new k,yo=new k,Mo=new k,So=new k,Yc=new k,wo=new k,rd=new k,Eo=new k,it=class extends Ft{constructor(e=new bt,t=new Lt){super(),this.isMesh=!0,this.type="Mesh",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.count=1,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),e.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=e.morphTargetInfluences.slice()),e.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},e.morphTargetDictionary)),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}getVertexPosition(e,t){let n=this.geometry,i=n.attributes.position,r=n.morphAttributes.position,a=n.morphTargetsRelative;t.fromBufferAttribute(i,e);let o=this.morphTargetInfluences;if(r&&o){wo.set(0,0,0);for(let l=0,c=r.length;l<c;l++){let h=o[l],u=r[l];h!==0&&(Yc.fromBufferAttribute(u,e),a?wo.addScaledVector(Yc,h):wo.addScaledVector(Yc.sub(t),h))}t.add(wo)}return t}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.material,r=this.matrixWorld;i!==void 0&&(n.boundingSphere===null&&n.computeBoundingSphere(),_o.copy(n.boundingSphere),_o.applyMatrix4(r),ys.copy(e.ray).recast(e.near),!(_o.containsPoint(ys.origin)===!1&&(ys.intersectSphere(_o,sd)===null||ys.origin.distanceToSquared(sd)>(e.far-e.near)**2))&&(id.copy(r).invert(),ys.copy(e.ray).applyMatrix4(id),!(n.boundingBox!==null&&ys.intersectsBox(n.boundingBox)===!1)&&this._computeIntersections(e,t,ys)))}_computeIntersections(e,t,n){let i,r=this.geometry,a=this.material,o=r.index,l=r.attributes.position,c=r.attributes.uv,h=r.attributes.uv1,u=r.attributes.normal,d=r.groups,p=r.drawRange;if(o!==null)if(Array.isArray(a))for(let g=0,_=d.length;g<_;g++){let m=d[g],f=a[m.materialIndex],v=Math.max(m.start,p.start),S=Math.min(o.count,Math.min(m.start+m.count,p.start+p.count));for(let b=v,M=S;b<M;b+=3){let E=o.getX(b),P=o.getX(b+1),x=o.getX(b+2);i=To(this,f,e,n,c,h,u,E,P,x),i&&(i.faceIndex=Math.floor(b/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,p.start),_=Math.min(o.count,p.start+p.count);for(let m=g,f=_;m<f;m+=3){let v=o.getX(m),S=o.getX(m+1),b=o.getX(m+2);i=To(this,a,e,n,c,h,u,v,S,b),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}else if(l!==void 0)if(Array.isArray(a))for(let g=0,_=d.length;g<_;g++){let m=d[g],f=a[m.materialIndex],v=Math.max(m.start,p.start),S=Math.min(l.count,Math.min(m.start+m.count,p.start+p.count));for(let b=v,M=S;b<M;b+=3){let E=b,P=b+1,x=b+2;i=To(this,f,e,n,c,h,u,E,P,x),i&&(i.faceIndex=Math.floor(b/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,p.start),_=Math.min(l.count,p.start+p.count);for(let m=g,f=_;m<f;m+=3){let v=m,S=m+1,b=m+2;i=To(this,a,e,n,c,h,u,v,S,b),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}}};function Um(s,e,t,n,i,r,a,o){let l;if(e.side===on?l=n.intersectTriangle(a,r,i,!0,o):l=n.intersectTriangle(i,r,a,e.side===jn,o),l===null)return null;Eo.copy(o),Eo.applyMatrix4(s.matrixWorld);let c=t.ray.origin.distanceTo(Eo);return c<t.near||c>t.far?null:{distance:c,point:Eo.clone(),object:s}}function To(s,e,t,n,i,r,a,o,l,c){s.getVertexPosition(o,yo),s.getVertexPosition(l,Mo),s.getVertexPosition(c,So);let h=Um(s,e,t,n,yo,Mo,So,rd);if(h){let u=new k;is.getBarycoord(rd,yo,Mo,So,u),i&&(h.uv=is.getInterpolatedAttribute(i,o,l,c,u,new Be)),r&&(h.uv1=is.getInterpolatedAttribute(r,o,l,c,u,new Be)),a&&(h.normal=is.getInterpolatedAttribute(a,o,l,c,u,new k),h.normal.dot(n.direction)>0&&h.normal.multiplyScalar(-1));let d={a:o,b:l,c,normal:new k,materialIndex:0};is.getNormal(yo,Mo,So,d.normal),h.face=d,h.barycoord=u}return h}var ea=new Pt,ad=new Pt,od=new Pt,Om=new Pt,ld=new pt,Ao=new k,Jc=new xn,cd=new pt,Zc=new gi,ma=class extends it{constructor(e,t){super(e,t),this.isSkinnedMesh=!0,this.type="SkinnedMesh",this.bindMode=sh,this.bindMatrix=new pt,this.bindMatrixInverse=new pt,this.boundingBox=null,this.boundingSphere=null}computeBoundingBox(){let e=this.geometry;this.boundingBox===null&&(this.boundingBox=new en),this.boundingBox.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,Ao),this.boundingBox.expandByPoint(Ao)}computeBoundingSphere(){let e=this.geometry;this.boundingSphere===null&&(this.boundingSphere=new xn),this.boundingSphere.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,Ao),this.boundingSphere.expandByPoint(Ao)}copy(e,t){return super.copy(e,t),this.bindMode=e.bindMode,this.bindMatrix.copy(e.bindMatrix),this.bindMatrixInverse.copy(e.bindMatrixInverse),this.skeleton=e.skeleton,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}raycast(e,t){let n=this.material,i=this.matrixWorld;n!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Jc.copy(this.boundingSphere),Jc.applyMatrix4(i),e.ray.intersectsSphere(Jc)!==!1&&(cd.copy(i).invert(),Zc.copy(e.ray).applyMatrix4(cd),!(this.boundingBox!==null&&Zc.intersectsBox(this.boundingBox)===!1)&&this._computeIntersections(e,t,Zc)))}getVertexPosition(e,t){return super.getVertexPosition(e,t),this.applyBoneTransform(e,t),t}bind(e,t){this.skeleton=e,t===void 0&&(this.updateMatrixWorld(!0),this.skeleton.calculateInverses(),t=this.matrixWorld),this.bindMatrix.copy(t),this.bindMatrixInverse.copy(t).invert()}pose(){this.skeleton.pose()}normalizeSkinWeights(){let e=new Pt,t=this.geometry.attributes.skinWeight;for(let n=0,i=t.count;n<i;n++){e.fromBufferAttribute(t,n);let r=1/e.manhattanLength();r!==1/0?e.multiplyScalar(r):e.set(1,0,0,0),t.setXYZW(n,e.x,e.y,e.z,e.w)}}updateMatrixWorld(e){super.updateMatrixWorld(e),this.bindMode===sh?this.bindMatrixInverse.copy(this.matrixWorld).invert():this.bindMode===nf?this.bindMatrixInverse.copy(this.bindMatrix).invert():nt("SkinnedMesh: Unrecognized bindMode: "+this.bindMode)}applyBoneTransform(e,t){let n=this.skeleton,i=this.geometry;ad.fromBufferAttribute(i.attributes.skinIndex,e),od.fromBufferAttribute(i.attributes.skinWeight,e),t.isVector4?(ea.copy(t),t.set(0,0,0,0)):(ea.set(...t,1),t.set(0,0,0)),ea.applyMatrix4(this.bindMatrix);for(let r=0;r<4;r++){let a=od.getComponent(r);if(a!==0){let o=ad.getComponent(r);ld.multiplyMatrices(n.bones[o].matrixWorld,n.boneInverses[o]),t.addScaledVector(Om.copy(ea).applyMatrix4(ld),a)}}return t.isVector4&&(t.w=ea.w),t.applyMatrix4(this.bindMatrixInverse)}},_r=class extends Ft{constructor(){super(),this.isBone=!0,this.type="Bone"}},ss=class extends Qt{constructor(e=null,t=1,n=1,i,r,a,o,l,c=$t,h=$t,u,d){super(null,a,o,l,c,h,i,r,u,d),this.isDataTexture=!0,this.image={data:e,width:t,height:n},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}},hd=new pt,zm=new pt,ga=class s{constructor(e=[],t=[]){this.uuid=ii(),this.bones=e.slice(0),this.boneInverses=t,this.boneMatrices=null,this.boneTexture=null,this.init()}init(){let e=this.bones,t=this.boneInverses;if(this.boneMatrices=new Float32Array(e.length*16),t.length===0)this.calculateInverses();else if(e.length!==t.length){nt("Skeleton: Number of inverse bone matrices does not match amount of bones."),this.boneInverses=[];for(let n=0,i=this.bones.length;n<i;n++)this.boneInverses.push(new pt)}}calculateInverses(){this.boneInverses.length=0;for(let e=0,t=this.bones.length;e<t;e++){let n=new pt;this.bones[e]&&n.copy(this.bones[e].matrixWorld).invert(),this.boneInverses.push(n)}}pose(){for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&n.matrixWorld.copy(this.boneInverses[e]).invert()}for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&(n.parent&&n.parent.isBone?(n.matrix.copy(n.parent.matrixWorld).invert(),n.matrix.multiply(n.matrixWorld)):n.matrix.copy(n.matrixWorld),n.matrix.decompose(n.position,n.quaternion,n.scale))}}update(){let e=this.bones,t=this.boneInverses,n=this.boneMatrices,i=this.boneTexture;for(let r=0,a=e.length;r<a;r++){let o=e[r]?e[r].matrixWorld:zm;hd.multiplyMatrices(o,t[r]),hd.toArray(n,r*16)}i!==null&&(i.needsUpdate=!0)}clone(){return new s(this.bones,this.boneInverses)}computeBoneTexture(){let e=Math.sqrt(this.bones.length*4);e=Math.ceil(e/4)*4,e=Math.max(e,4);let t=new Float32Array(e*e*4);t.set(this.boneMatrices);let n=new ss(t,e,e,Hn,kn);return n.needsUpdate=!0,this.boneMatrices=t,this.boneTexture=n,this}getBoneByName(e){for(let t=0,n=this.bones.length;t<n;t++){let i=this.bones[t];if(i.name===e)return i}}dispose(){this.boneTexture!==null&&(this.boneTexture.dispose(),this.boneTexture=null)}fromJSON(e,t){this.uuid=e.uuid;for(let n=0,i=e.bones.length;n<i;n++){let r=e.bones[n],a=t[r];a===void 0&&(nt("Skeleton: No bone found with UUID:",r),a=new _r),this.bones.push(a),this.boneInverses.push(new pt().fromArray(e.boneInverses[n]))}return this.init(),this}toJSON(){let e={metadata:{version:4.7,type:"Skeleton",generator:"Skeleton.toJSON"},bones:[],boneInverses:[]};e.uuid=this.uuid;let t=this.bones,n=this.boneInverses;for(let i=0,r=t.length;i<r;i++){let a=t[i];e.bones.push(a.uuid);let o=n[i];e.boneInverses.push(o.toArray())}return e}},Ui=class extends Mt{constructor(e,t,n,i=1){super(e,t,n),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=i}copy(e){return super.copy(e),this.meshPerAttribute=e.meshPerAttribute,this}toJSON(){let e=super.toJSON();return e.meshPerAttribute=this.meshPerAttribute,e.isInstancedBufferAttribute=!0,e}},rr=new pt,ud=new pt,Ro=[],dd=new en,Bm=new pt,ta=new it,na=new xn,An=class extends it{constructor(e,t,n){super(e,t),this.isInstancedMesh=!0,this.instanceMatrix=new Ui(new Float32Array(n*16),16),this.instanceColor=null,this.morphTexture=null,this.count=n,this.boundingBox=null,this.boundingSphere=null;for(let i=0;i<n;i++)this.setMatrixAt(i,Bm)}computeBoundingBox(){let e=this.geometry,t=this.count;this.boundingBox===null&&(this.boundingBox=new en),e.boundingBox===null&&e.computeBoundingBox(),this.boundingBox.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,rr),dd.copy(e.boundingBox).applyMatrix4(rr),this.boundingBox.union(dd)}computeBoundingSphere(){let e=this.geometry,t=this.count;this.boundingSphere===null&&(this.boundingSphere=new xn),e.boundingSphere===null&&e.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,rr),na.copy(e.boundingSphere).applyMatrix4(rr),this.boundingSphere.union(na)}copy(e,t){return super.copy(e,t),this.instanceMatrix.copy(e.instanceMatrix),e.morphTexture!==null&&(this.morphTexture=e.morphTexture.clone()),e.instanceColor!==null&&(this.instanceColor=e.instanceColor.clone()),this.count=e.count,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}getColorAt(e,t){return this.instanceColor===null?t.setRGB(1,1,1):t.fromArray(this.instanceColor.array,e*3)}getMatrixAt(e,t){return t.fromArray(this.instanceMatrix.array,e*16)}getMorphAt(e,t){let n=t.morphTargetInfluences,i=this.morphTexture.source.data.data,r=n.length+1,a=e*r+1;for(let o=0;o<n.length;o++)n[o]=i[a+o]}raycast(e,t){let n=this.matrixWorld,i=this.count;if(ta.geometry=this.geometry,ta.material=this.material,ta.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),na.copy(this.boundingSphere),na.applyMatrix4(n),e.ray.intersectsSphere(na)!==!1))for(let r=0;r<i;r++){this.getMatrixAt(r,rr),ud.multiplyMatrices(n,rr),ta.matrixWorld=ud,ta.raycast(e,Ro);for(let a=0,o=Ro.length;a<o;a++){let l=Ro[a];l.instanceId=r,l.object=this,t.push(l)}Ro.length=0}}setColorAt(e,t){return this.instanceColor===null&&(this.instanceColor=new Ui(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),t.toArray(this.instanceColor.array,e*3),this}setMatrixAt(e,t){return t.toArray(this.instanceMatrix.array,e*16),this}setMorphAt(e,t){let n=t.morphTargetInfluences,i=n.length+1;this.morphTexture===null&&(this.morphTexture=new ss(new Float32Array(i*this.count),i,this.count,wl,kn));let r=this.morphTexture.source.data.data,a=0;for(let c=0;c<n.length;c++)a+=n[c];let o=this.geometry.morphTargetsRelative?1:1-a,l=i*e;return r[l]=o,r.set(n,l+1),this}updateMorphTargets(){}dispose(){super.dispose(),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null)}},Ms=new xn,km=new Be(.5,.5),Co=new k,yr=class{constructor(e=new On,t=new On,n=new On,i=new On,r=new On,a=new On){this.planes=[e,t,n,i,r,a]}set(e,t,n,i,r,a){let o=this.planes;return o[0].copy(e),o[1].copy(t),o[2].copy(n),o[3].copy(i),o[4].copy(r),o[5].copy(a),this}copy(e){let t=this.planes;for(let n=0;n<6;n++)t[n].copy(e.planes[n]);return this}setFromProjectionMatrix(e,t=ni,n=!1){let i=this.planes,r=e.elements,a=r[0],o=r[1],l=r[2],c=r[3],h=r[4],u=r[5],d=r[6],p=r[7],g=r[8],_=r[9],m=r[10],f=r[11],v=r[12],S=r[13],b=r[14],M=r[15];if(i[0].setComponents(c-a,p-h,f-g,M-v).normalize(),i[1].setComponents(c+a,p+h,f+g,M+v).normalize(),i[2].setComponents(c+o,p+u,f+_,M+S).normalize(),i[3].setComponents(c-o,p-u,f-_,M-S).normalize(),n)i[4].setComponents(l,d,m,b).normalize(),i[5].setComponents(c-l,p-d,f-m,M-b).normalize();else if(i[4].setComponents(c-l,p-d,f-m,M-b).normalize(),t===ni)i[5].setComponents(c+l,p+d,f+m,M+b).normalize();else if(t===dr)i[5].setComponents(l,d,m,b).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+t);return this}intersectsObject(e){if(e.boundingSphere!==void 0)e.boundingSphere===null&&e.computeBoundingSphere(),Ms.copy(e.boundingSphere).applyMatrix4(e.matrixWorld);else{let t=e.geometry;t.boundingSphere===null&&t.computeBoundingSphere(),Ms.copy(t.boundingSphere).applyMatrix4(e.matrixWorld)}return this.intersectsSphere(Ms)}intersectsSprite(e){Ms.center.set(0,0,0);let t=km.distanceTo(e.center);return Ms.radius=.7071067811865476+t,Ms.applyMatrix4(e.matrixWorld),this.intersectsSphere(Ms)}intersectsSphere(e){let t=this.planes,n=e.center,i=-e.radius;for(let r=0;r<6;r++)if(t[r].distanceToPoint(n)<i)return!1;return!0}intersectsBox(e){let t=this.planes;for(let n=0;n<6;n++){let i=t[n];if(Co.x=i.normal.x>0?e.max.x:e.min.x,Co.y=i.normal.y>0?e.max.y:e.min.y,Co.z=i.normal.z>0?e.max.z:e.min.z,i.distanceToPoint(Co)<0)return!1}return!0}containsPoint(e){let t=this.planes;for(let n=0;n<6;n++)if(t[n].distanceToPoint(e)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};var bi=class extends En{constructor(e){super(),this.isLineBasicMaterial=!0,this.type="LineBasicMaterial",this.color=new Le(16777215),this.map=null,this.linewidth=1,this.linecap="round",this.linejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.linewidth=e.linewidth,this.linecap=e.linecap,this.linejoin=e.linejoin,this.fog=e.fog,this}},Yo=new k,Jo=new k,fd=new pt,ia=new gi,Po=new xn,$c=new k,pd=new k,Oi=class extends Ft{constructor(e=new bt,t=new bi){super(),this.isLine=!0,this.type="Line",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[0];for(let i=1,r=t.count;i<r;i++)Yo.fromBufferAttribute(t,i-1),Jo.fromBufferAttribute(t,i),n[i]=n[i-1],n[i]+=Yo.distanceTo(Jo);e.setAttribute("lineDistance",new ut(n,1))}else nt("Line.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Line.threshold,a=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),Po.copy(n.boundingSphere),Po.applyMatrix4(i),Po.radius+=r,e.ray.intersectsSphere(Po)===!1)return;fd.copy(i).invert(),ia.copy(e.ray).applyMatrix4(fd);let o=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=o*o,c=this.isLineSegments?2:1,h=n.index,d=n.attributes.position;if(h!==null){let p=Math.max(0,a.start),g=Math.min(h.count,a.start+a.count);for(let _=p,m=g-1;_<m;_+=c){let f=h.getX(_),v=h.getX(_+1),S=Io(this,e,ia,l,f,v,_);S&&t.push(S)}if(this.isLineLoop){let _=h.getX(g-1),m=h.getX(p),f=Io(this,e,ia,l,_,m,g-1);f&&t.push(f)}}else{let p=Math.max(0,a.start),g=Math.min(d.count,a.start+a.count);for(let _=p,m=g-1;_<m;_+=c){let f=Io(this,e,ia,l,_,_+1,_);f&&t.push(f)}if(this.isLineLoop){let _=Io(this,e,ia,l,g-1,p,g-1);_&&t.push(_)}}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}};function Io(s,e,t,n,i,r,a){let o=s.geometry.attributes.position;if(Yo.fromBufferAttribute(o,i),Jo.fromBufferAttribute(o,r),t.distanceSqToSegment(Yo,Jo,$c,pd)>n)return;$c.applyMatrix4(s.matrixWorld);let c=e.ray.origin.distanceTo($c);if(!(c<e.near||c>e.far))return{distance:c,point:pd.clone().applyMatrix4(s.matrixWorld),index:a,face:null,faceIndex:null,barycoord:null,object:s}}var md=new k,gd=new k,Ps=class extends Oi{constructor(e,t){super(e,t),this.isLineSegments=!0,this.type="LineSegments"}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[];for(let i=0,r=t.count;i<r;i+=2)md.fromBufferAttribute(t,i),gd.fromBufferAttribute(t,i+1),n[i]=i===0?0:n[i-1],n[i+1]=n[i]+md.distanceTo(gd);e.setAttribute("lineDistance",new ut(n,1))}else nt("LineSegments.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}},ba=class extends Oi{constructor(e,t){super(e,t),this.isLineLoop=!0,this.type="LineLoop"}},Mr=class extends En{constructor(e){super(),this.isPointsMaterial=!0,this.type="PointsMaterial",this.color=new Le(16777215),this.map=null,this.alphaMap=null,this.size=1,this.sizeAttenuation=!0,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.alphaMap=e.alphaMap,this.size=e.size,this.sizeAttenuation=e.sizeAttenuation,this.fog=e.fog,this}},bd=new pt,ah=new gi,Do=new xn,Lo=new k,fn=class extends Ft{constructor(e=new bt,t=new Mr){super(),this.isPoints=!0,this.type="Points",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Points.threshold,a=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),Do.copy(n.boundingSphere),Do.applyMatrix4(i),Do.radius+=r,e.ray.intersectsSphere(Do)===!1)return;bd.copy(i).invert(),ah.copy(e.ray).applyMatrix4(bd);let o=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=o*o,c=n.index,u=n.attributes.position;if(c!==null){let d=Math.max(0,a.start),p=Math.min(c.count,a.start+a.count);for(let g=d,_=p;g<_;g++){let m=c.getX(g);Lo.fromBufferAttribute(u,m),xd(Lo,m,l,i,e,t,this)}}else{let d=Math.max(0,a.start),p=Math.min(u.count,a.start+a.count);for(let g=d,_=p;g<_;g++)Lo.fromBufferAttribute(u,g),xd(Lo,g,l,i,e,t,this)}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}};function xd(s,e,t,n,i,r,a){let o=ah.distanceSqToPoint(s);if(o<t){let l=new k;ah.closestPointToPoint(s,l),l.applyMatrix4(n);let c=i.ray.origin.distanceTo(l);if(c<i.near||c>i.far)return;r.push({distance:c,distanceToRay:Math.sqrt(o),point:l,index:e,face:null,faceIndex:null,barycoord:null,object:a})}}var xa=class extends Qt{constructor(e=[],t=us,n,i,r,a,o,l,c,h){super(e,t,n,i,r,a,o,l,c,h),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(e){this.image=e}},Is=class extends Qt{constructor(e,t,n,i,r,a,o,l,c){super(e,t,n,i,r,a,o,l,c),this.isCanvasTexture=!0,this.needsUpdate=!0}};var rs=class extends Qt{constructor(e,t,n=li,i,r,a,o=$t,l=$t,c,h=mi,u=1){if(h!==mi&&h!==ds)throw new Error("THREE.DepthTexture: format must be either THREE.DepthFormat or THREE.DepthStencilFormat");let d={width:e,height:t,depth:u};super(d,i,r,a,o,l,h,n,c),this.isDepthTexture=!0,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(e){return super.copy(e),this.source=new mr(Object.assign({},e.image)),this.compareFunction=e.compareFunction,this}toJSON(e){let t=super.toJSON(e);return t.compareFunction=this.compareFunction,t}},Zo=class extends rs{constructor(e,t=li,n=us,i,r,a=$t,o=$t,l,c=mi){let h={width:e,height:e,depth:1},u=[h,h,h,h,h,h];super(e,e,t,n,i,r,a,o,l,c),this.image=u,this.isCubeDepthTexture=!0,this.isCubeTexture=!0}get images(){return this.image}set images(e){this.image=e}},va=class extends Qt{constructor(e=null){super(),this.sourceTexture=e,this.isExternalTexture=!0}copy(e){return super.copy(e),this.sourceTexture=e.sourceTexture,this}},xi=class s extends bt{constructor(e=1,t=1,n=1,i=1,r=1,a=1){super(),this.type="BoxGeometry",this.parameters={width:e,height:t,depth:n,widthSegments:i,heightSegments:r,depthSegments:a};let o=this;i=Math.floor(i),r=Math.floor(r),a=Math.floor(a);let l=[],c=[],h=[],u=[],d=0,p=0;g("z","y","x",-1,-1,n,t,e,a,r,0),g("z","y","x",1,-1,n,t,-e,a,r,1),g("x","z","y",1,1,e,n,t,i,a,2),g("x","z","y",1,-1,e,n,-t,i,a,3),g("x","y","z",1,-1,e,t,n,i,r,4),g("x","y","z",-1,-1,e,t,-n,i,r,5),this.setIndex(l),this.setAttribute("position",new ut(c,3)),this.setAttribute("normal",new ut(h,3)),this.setAttribute("uv",new ut(u,2));function g(_,m,f,v,S,b,M,E,P,x,C){let w=b/P,I=M/x,y=b/2,N=M/2,D=E/2,R=P+1,U=x+1,z=0,T=0,G=new k;for(let X=0;X<U;X++){let Z=X*I-N;for(let H=0;H<R;H++){let oe=H*w-y;G[_]=oe*v,G[m]=Z*S,G[f]=D,c.push(G.x,G.y,G.z),G[_]=0,G[m]=0,G[f]=E>0?1:-1,h.push(G.x,G.y,G.z),u.push(H/P),u.push(1-X/x),z+=1}}for(let X=0;X<x;X++)for(let Z=0;Z<P;Z++){let H=d+Z+R*X,oe=d+Z+R*(X+1),ke=d+(Z+1)+R*(X+1),Fe=d+(Z+1)+R*X;l.push(H,oe,Fe),l.push(oe,ke,Fe),T+=6}o.addGroup(p,T,C),p+=T,d+=z}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.depth,e.widthSegments,e.heightSegments,e.depthSegments)}};var Sr=class s extends bt{constructor(e=1,t=32,n=0,i=Math.PI*2){super(),this.type="CircleGeometry",this.parameters={radius:e,segments:t,thetaStart:n,thetaLength:i},t=Math.max(3,t);let r=[],a=[],o=[],l=[],c=new k,h=new Be;a.push(0,0,0),o.push(0,0,1),l.push(.5,.5);for(let u=0,d=3;u<=t;u++,d+=3){let p=n+u/t*i;c.x=e*Math.cos(p),c.y=e*Math.sin(p),a.push(c.x,c.y,c.z),o.push(0,0,1),h.x=(a[d]/e+1)/2,h.y=(a[d+1]/e+1)/2,l.push(h.x,h.y)}for(let u=1;u<=t;u++)r.push(u,u+1,0);this.setIndex(r),this.setAttribute("position",new ut(a,3)),this.setAttribute("normal",new ut(o,3)),this.setAttribute("uv",new ut(l,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.segments,e.thetaStart,e.thetaLength)}},as=class s extends bt{constructor(e=1,t=1,n=1,i=32,r=1,a=!1,o=0,l=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:e,radiusBottom:t,height:n,radialSegments:i,heightSegments:r,openEnded:a,thetaStart:o,thetaLength:l};let c=this;i=Math.floor(i),r=Math.floor(r);let h=[],u=[],d=[],p=[],g=0,_=[],m=n/2,f=0;v(),a===!1&&(e>0&&S(!0),t>0&&S(!1)),this.setIndex(h),this.setAttribute("position",new ut(u,3)),this.setAttribute("normal",new ut(d,3)),this.setAttribute("uv",new ut(p,2));function v(){let b=new k,M=new k,E=0,P=(t-e)/n;for(let x=0;x<=r;x++){let C=[],w=x/r,I=w*(t-e)+e;for(let y=0;y<=i;y++){let N=y/i,D=N*l+o,R=Math.sin(D),U=Math.cos(D);M.x=I*R,M.y=-w*n+m,M.z=I*U,u.push(M.x,M.y,M.z),b.set(R,P,U).normalize(),d.push(b.x,b.y,b.z),p.push(N,1-w),C.push(g++)}_.push(C)}for(let x=0;x<i;x++)for(let C=0;C<r;C++){let w=_[C][x],I=_[C+1][x],y=_[C+1][x+1],N=_[C][x+1];(e>0||C!==0)&&(h.push(w,I,N),E+=3),(t>0||C!==r-1)&&(h.push(I,y,N),E+=3)}c.addGroup(f,E,0),f+=E}function S(b){let M=g,E=new Be,P=new k,x=0,C=b===!0?e:t,w=b===!0?1:-1;for(let y=1;y<=i;y++)u.push(0,m*w,0),d.push(0,w,0),p.push(.5,.5),g++;let I=g;for(let y=0;y<=i;y++){let D=y/i*l+o,R=Math.cos(D),U=Math.sin(D);P.x=C*U,P.y=m*w,P.z=C*R,u.push(P.x,P.y,P.z),d.push(0,w,0),E.x=R*.5+.5,E.y=U*.5*w+.5,p.push(E.x,E.y),g++}for(let y=0;y<i;y++){let N=M+y,D=I+y;b===!0?h.push(D,D+1,N):h.push(D+1,D,N),x+=3}c.addGroup(f,x,b===!0?1:2),f+=x}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radiusTop,e.radiusBottom,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Ds=class s extends as{constructor(e=1,t=1,n=32,i=1,r=!1,a=0,o=Math.PI*2){super(0,e,t,n,i,r,a,o),this.type="ConeGeometry",this.parameters={radius:e,height:t,radialSegments:n,heightSegments:i,openEnded:r,thetaStart:a,thetaLength:o}}static fromJSON(e){return new s(e.radius,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},$o=class s extends bt{constructor(e=[],t=[],n=1,i=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:e,indices:t,radius:n,detail:i};let r=[],a=[];o(i),c(n),h(),this.setAttribute("position",new ut(r,3)),this.setAttribute("normal",new ut(r.slice(),3)),this.setAttribute("uv",new ut(a,2)),i===0?this.computeVertexNormals():this.normalizeNormals();function o(v){let S=new k,b=new k,M=new k;for(let E=0;E<t.length;E+=3)p(t[E+0],S),p(t[E+1],b),p(t[E+2],M),l(S,b,M,v)}function l(v,S,b,M){let E=M+1,P=[];for(let x=0;x<=E;x++){P[x]=[];let C=v.clone().lerp(b,x/E),w=S.clone().lerp(b,x/E),I=E-x;for(let y=0;y<=I;y++)y===0&&x===E?P[x][y]=C:P[x][y]=C.clone().lerp(w,y/I)}for(let x=0;x<E;x++)for(let C=0;C<2*(E-x)-1;C++){let w=Math.floor(C/2);C%2===0?(d(P[x][w+1]),d(P[x+1][w]),d(P[x][w])):(d(P[x][w+1]),d(P[x+1][w+1]),d(P[x+1][w]))}}function c(v){let S=new k;for(let b=0;b<r.length;b+=3)S.x=r[b+0],S.y=r[b+1],S.z=r[b+2],S.normalize().multiplyScalar(v),r[b+0]=S.x,r[b+1]=S.y,r[b+2]=S.z}function h(){let v=new k;for(let S=0;S<r.length;S+=3){v.x=r[S+0],v.y=r[S+1],v.z=r[S+2];let b=m(v)/2/Math.PI+.5,M=f(v)/Math.PI+.5;a.push(b,1-M)}g(),u()}function u(){for(let v=0;v<a.length;v+=6){let S=a[v+0],b=a[v+2],M=a[v+4],E=Math.max(S,b,M),P=Math.min(S,b,M);E>.9&&P<.1&&(S<.2&&(a[v+0]+=1),b<.2&&(a[v+2]+=1),M<.2&&(a[v+4]+=1))}}function d(v){r.push(v.x,v.y,v.z)}function p(v,S){let b=v*3;S.x=e[b+0],S.y=e[b+1],S.z=e[b+2]}function g(){let v=new k,S=new k,b=new k,M=new k,E=new Be,P=new Be,x=new Be;for(let C=0,w=0;C<r.length;C+=9,w+=6){v.set(r[C+0],r[C+1],r[C+2]),S.set(r[C+3],r[C+4],r[C+5]),b.set(r[C+6],r[C+7],r[C+8]),E.set(a[w+0],a[w+1]),P.set(a[w+2],a[w+3]),x.set(a[w+4],a[w+5]),M.copy(v).add(S).add(b).divideScalar(3);let I=m(M);_(E,w+0,v,I),_(P,w+2,S,I),_(x,w+4,b,I)}}function _(v,S,b,M){M<0&&v.x===1&&(a[S]=v.x-1),b.x===0&&b.z===0&&(a[S]=M/2/Math.PI+.5)}function m(v){return Math.atan2(v.z,-v.x)}function f(v){return Math.atan2(-v.y,Math.sqrt(v.x*v.x+v.z*v.z))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.vertices,e.indices,e.radius,e.detail)}};var zn=class{constructor(){this.type="Curve",this.arcLengthDivisions=200,this.needsUpdate=!1,this.cacheArcLengths=null}getPoint(){nt("Curve: .getPoint() not implemented.")}getPointAt(e,t){let n=this.getUtoTmapping(e);return this.getPoint(n,t)}getPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return t}getSpacedPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPointAt(n/e));return t}getLength(){let e=this.getLengths();return e[e.length-1]}getLengths(e=this.arcLengthDivisions){if(this.cacheArcLengths&&this.cacheArcLengths.length===e+1&&!this.needsUpdate)return this.cacheArcLengths;this.needsUpdate=!1;let t=[],n,i=this.getPoint(0),r=0;t.push(0);for(let a=1;a<=e;a++)n=this.getPoint(a/e),r+=n.distanceTo(i),t.push(r),i=n;return this.cacheArcLengths=t,t}updateArcLengths(){this.needsUpdate=!0,this.getLengths()}getUtoTmapping(e,t=null){let n=this.getLengths(),i=0,r=n.length,a;t?a=t:a=e*n[r-1];let o=0,l=r-1,c;for(;o<=l;)if(i=Math.floor(o+(l-o)/2),c=n[i]-a,c<0)o=i+1;else if(c>0)l=i-1;else{l=i;break}if(i=l,n[i]===a)return i/(r-1);let h=n[i],d=n[i+1]-h,p=(a-h)/d;return(i+p)/(r-1)}getTangent(e,t){let i=e-1e-4,r=e+1e-4;i<0&&(i=0),r>1&&(r=1);let a=this.getPoint(i),o=this.getPoint(r),l=t||(a.isVector2?new Be:new k);return l.copy(o).sub(a).normalize(),l}getTangentAt(e,t){let n=this.getUtoTmapping(e);return this.getTangent(n,t)}computeFrenetFrames(e,t=!1){let n=new k,i=[],r=[],a=[],o=new k,l=new pt;for(let p=0;p<=e;p++){let g=p/e;i[p]=this.getTangentAt(g,new k)}r[0]=new k,a[0]=new k;let c=Number.MAX_VALUE,h=Math.abs(i[0].x),u=Math.abs(i[0].y),d=Math.abs(i[0].z);h<=c&&(c=h,n.set(1,0,0)),u<=c&&(c=u,n.set(0,1,0)),d<=c&&n.set(0,0,1),o.crossVectors(i[0],n).normalize(),r[0].crossVectors(i[0],o),a[0].crossVectors(i[0],r[0]);for(let p=1;p<=e;p++){if(r[p]=r[p-1].clone(),a[p]=a[p-1].clone(),o.crossVectors(i[p-1],i[p]),o.length()>Number.EPSILON){o.normalize();let g=Math.acos(yt(i[p-1].dot(i[p]),-1,1));r[p].applyMatrix4(l.makeRotationAxis(o,g))}a[p].crossVectors(i[p],r[p])}if(t===!0){let p=Math.acos(yt(r[0].dot(r[e]),-1,1));p/=e,i[0].dot(o.crossVectors(r[0],r[e]))>0&&(p=-p);for(let g=1;g<=e;g++)r[g].applyMatrix4(l.makeRotationAxis(i[g],p*g)),a[g].crossVectors(i[g],r[g])}return{tangents:i,normals:r,binormals:a}}clone(){return new this.constructor().copy(this)}copy(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}toJSON(){let e={metadata:{version:4.7,type:"Curve",generator:"Curve.toJSON"}};return e.arcLengthDivisions=this.arcLengthDivisions,e.type=this.type,e}fromJSON(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}},_a=class extends zn{constructor(e=0,t=0,n=1,i=1,r=0,a=Math.PI*2,o=!1,l=0){super(),this.isEllipseCurve=!0,this.type="EllipseCurve",this.aX=e,this.aY=t,this.xRadius=n,this.yRadius=i,this.aStartAngle=r,this.aEndAngle=a,this.aClockwise=o,this.aRotation=l}getPoint(e,t=new Be){let n=t,i=Math.PI*2,r=this.aEndAngle-this.aStartAngle,a=Math.abs(r)<Number.EPSILON;for(;r<0;)r+=i;for(;r>i;)r-=i;r<Number.EPSILON&&(a?r=0:r=i),this.aClockwise===!0&&!a&&(r===i?r=-i:r=r-i);let o=this.aStartAngle+e*r,l=this.aX+this.xRadius*Math.cos(o),c=this.aY+this.yRadius*Math.sin(o);if(this.aRotation!==0){let h=Math.cos(this.aRotation),u=Math.sin(this.aRotation),d=l-this.aX,p=c-this.aY;l=d*h-p*u+this.aX,c=d*u+p*h+this.aY}return n.set(l,c)}copy(e){return super.copy(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}toJSON(){let e=super.toJSON();return e.aX=this.aX,e.aY=this.aY,e.xRadius=this.xRadius,e.yRadius=this.yRadius,e.aStartAngle=this.aStartAngle,e.aEndAngle=this.aEndAngle,e.aClockwise=this.aClockwise,e.aRotation=this.aRotation,e}fromJSON(e){return super.fromJSON(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}},Qo=class extends _a{constructor(e,t,n,i,r,a){super(e,t,n,n,i,r,a),this.isArcCurve=!0,this.type="ArcCurve"}};function Ih(){let s=0,e=0,t=0,n=0;function i(r,a,o,l){s=r,e=o,t=-3*r+3*a-2*o-l,n=2*r-2*a+o+l}return{initCatmullRom:function(r,a,o,l,c){i(a,o,c*(o-r),c*(l-a))},initNonuniformCatmullRom:function(r,a,o,l,c,h,u){let d=(a-r)/c-(o-r)/(c+h)+(o-a)/h,p=(o-a)/h-(l-a)/(h+u)+(l-o)/u;d*=h,p*=h,i(a,o,d,p)},calc:function(r){let a=r*r,o=a*r;return s+e*r+t*a+n*o}}}var vd=new k,_d=new k,Qc=new Ih,eh=new Ih,th=new Ih,wr=class extends zn{constructor(e=[],t=!1,n="centripetal",i=.5){super(),this.isCatmullRomCurve3=!0,this.type="CatmullRomCurve3",this.points=e,this.closed=t,this.curveType=n,this.tension=i}getPoint(e,t=new k){let n=t,i=this.points,r=i.length,a=(r-(this.closed?0:1))*e,o=Math.floor(a),l=a-o;this.closed?o+=o>0?0:(Math.floor(Math.abs(o)/r)+1)*r:l===0&&o===r-1&&(o=r-2,l=1);let c,h;this.closed||o>0?c=i[(o-1)%r]:(_d.subVectors(i[0],i[1]).add(i[0]),c=_d);let u=i[o%r],d=i[(o+1)%r];if(this.closed||o+2<r?h=i[(o+2)%r]:(vd.subVectors(i[r-1],i[r-2]).add(i[r-1]),h=vd),this.curveType==="centripetal"||this.curveType==="chordal"){let p=this.curveType==="chordal"?.5:.25,g=Math.pow(c.distanceToSquared(u),p),_=Math.pow(u.distanceToSquared(d),p),m=Math.pow(d.distanceToSquared(h),p);_<1e-4&&(_=1),g<1e-4&&(g=_),m<1e-4&&(m=_),Qc.initNonuniformCatmullRom(c.x,u.x,d.x,h.x,g,_,m),eh.initNonuniformCatmullRom(c.y,u.y,d.y,h.y,g,_,m),th.initNonuniformCatmullRom(c.z,u.z,d.z,h.z,g,_,m)}else this.curveType==="catmullrom"&&(Qc.initCatmullRom(c.x,u.x,d.x,h.x,this.tension),eh.initCatmullRom(c.y,u.y,d.y,h.y,this.tension),th.initCatmullRom(c.z,u.z,d.z,h.z,this.tension));return n.set(Qc.calc(l),eh.calc(l),th.calc(l)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e.closed=this.closed,e.curveType=this.curveType,e.tension=this.tension,e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new k().fromArray(i))}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}};function yd(s,e,t,n,i){let r=(n-e)*.5,a=(i-t)*.5,o=s*s,l=s*o;return(2*t-2*n+r+a)*l+(-3*t+3*n-2*r-a)*o+r*s+t}function Hm(s,e){let t=1-s;return t*t*e}function Gm(s,e){return 2*(1-s)*s*e}function Vm(s,e){return s*s*e}function aa(s,e,t,n){return Hm(s,e)+Gm(s,t)+Vm(s,n)}function Wm(s,e){let t=1-s;return t*t*t*e}function qm(s,e){let t=1-s;return 3*t*t*s*e}function Xm(s,e){return 3*(1-s)*s*s*e}function jm(s,e){return s*s*s*e}function oa(s,e,t,n,i){return Wm(s,e)+qm(s,t)+Xm(s,n)+jm(s,i)}var el=class extends zn{constructor(e=new Be,t=new Be,n=new Be,i=new Be){super(),this.isCubicBezierCurve=!0,this.type="CubicBezierCurve",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new Be){let n=t,i=this.v0,r=this.v1,a=this.v2,o=this.v3;return n.set(oa(e,i.x,r.x,a.x,o.x),oa(e,i.y,r.y,a.y,o.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},tl=class extends zn{constructor(e=new k,t=new k,n=new k,i=new k){super(),this.isCubicBezierCurve3=!0,this.type="CubicBezierCurve3",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new k){let n=t,i=this.v0,r=this.v1,a=this.v2,o=this.v3;return n.set(oa(e,i.x,r.x,a.x,o.x),oa(e,i.y,r.y,a.y,o.y),oa(e,i.z,r.z,a.z,o.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},nl=class extends zn{constructor(e=new Be,t=new Be){super(),this.isLineCurve=!0,this.type="LineCurve",this.v1=e,this.v2=t}getPoint(e,t=new Be){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new Be){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Er=class extends zn{constructor(e=new k,t=new k){super(),this.isLineCurve3=!0,this.type="LineCurve3",this.v1=e,this.v2=t}getPoint(e,t=new k){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new k){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},il=class extends zn{constructor(e=new Be,t=new Be,n=new Be){super(),this.isQuadraticBezierCurve=!0,this.type="QuadraticBezierCurve",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new Be){let n=t,i=this.v0,r=this.v1,a=this.v2;return n.set(aa(e,i.x,r.x,a.x),aa(e,i.y,r.y,a.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},si=class extends zn{constructor(e=new k,t=new k,n=new k){super(),this.isQuadraticBezierCurve3=!0,this.type="QuadraticBezierCurve3",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new k){let n=t,i=this.v0,r=this.v1,a=this.v2;return n.set(aa(e,i.x,r.x,a.x),aa(e,i.y,r.y,a.y),aa(e,i.z,r.z,a.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},sl=class extends zn{constructor(e=[]){super(),this.isSplineCurve=!0,this.type="SplineCurve",this.points=e}getPoint(e,t=new Be){let n=t,i=this.points,r=(i.length-1)*e,a=Math.floor(r),o=r-a,l=i[a===0?a:a-1],c=i[a],h=i[a>i.length-2?i.length-1:a+1],u=i[a>i.length-3?i.length-1:a+2];return n.set(yd(o,l.x,c.x,h.x,u.x),yd(o,l.y,c.y,h.y,u.y)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new Be().fromArray(i))}return this}},oh=Object.freeze({__proto__:null,ArcCurve:Qo,CatmullRomCurve3:wr,CubicBezierCurve:el,CubicBezierCurve3:tl,EllipseCurve:_a,LineCurve:nl,LineCurve3:Er,QuadraticBezierCurve:il,QuadraticBezierCurve3:si,SplineCurve:sl}),ya=class extends zn{constructor(){super(),this.type="CurvePath",this.curves=[],this.autoClose=!1}add(e){this.curves.push(e)}closePath(){let e=this.curves[0].getPoint(0),t=this.curves[this.curves.length-1].getPoint(1);if(!e.equals(t)){let n=e.isVector2===!0?"LineCurve":"LineCurve3";this.curves.push(new oh[n](t,e))}return this}getPoint(e,t){let n=e*this.getLength(),i=this.getCurveLengths(),r=0;for(;r<i.length;){if(i[r]>=n){let a=i[r]-n,o=this.curves[r],l=o.getLength(),c=l===0?0:1-a/l;return o.getPointAt(c,t)}r++}return null}getLength(){let e=this.getCurveLengths();return e[e.length-1]}updateArcLengths(){this.needsUpdate=!0,this.cacheLengths=null,this.getCurveLengths()}getCurveLengths(){if(this.cacheLengths&&this.cacheLengths.length===this.curves.length)return this.cacheLengths;let e=[],t=0;for(let n=0,i=this.curves.length;n<i;n++)t+=this.curves[n].getLength(),e.push(t);return this.cacheLengths=e,e}getSpacedPoints(e=40){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return this.autoClose&&t.push(t[0]),t}getPoints(e=12){let t=[],n;for(let i=0,r=this.curves;i<r.length;i++){let a=r[i],o=a.isEllipseCurve?e*2:a.isLineCurve||a.isLineCurve3?1:a.isSplineCurve?e*a.points.length:e,l=a.getPoints(o);for(let c=0;c<l.length;c++){let h=l[c];n&&n.equals(h)||(t.push(h),n=h)}}return this.autoClose&&t.length>1&&!t[t.length-1].equals(t[0])&&t.push(t[0]),t}copy(e){super.copy(e),this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(i.clone())}return this.autoClose=e.autoClose,this}toJSON(){let e=super.toJSON();e.autoClose=this.autoClose,e.curves=[];for(let t=0,n=this.curves.length;t<n;t++){let i=this.curves[t];e.curves.push(i.toJSON())}return e}fromJSON(e){super.fromJSON(e),this.autoClose=e.autoClose,this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(new oh[i.type]().fromJSON(i))}return this}};var Tr=class s extends $o{constructor(e=1,t=0){let n=(1+Math.sqrt(5))/2,i=[-1,n,0,1,n,0,-1,-n,0,1,-n,0,0,-1,n,0,1,n,0,-1,-n,0,1,-n,n,0,-1,n,0,1,-n,0,-1,-n,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(i,r,e,t),this.type="IcosahedronGeometry",this.parameters={radius:e,detail:t}}static fromJSON(e){return new s(e.radius,e.detail)}};var pn=class s extends bt{constructor(e=1,t=1,n=1,i=1){super(),this.type="PlaneGeometry",this.parameters={width:e,height:t,widthSegments:n,heightSegments:i};let r=e/2,a=t/2,o=Math.floor(n),l=Math.floor(i),c=o+1,h=l+1,u=e/o,d=t/l,p=[],g=[],_=[],m=[];for(let f=0;f<h;f++){let v=f*d-a;for(let S=0;S<c;S++){let b=S*u-r;g.push(b,-v,0),_.push(0,0,1),m.push(S/o),m.push(1-f/l)}}for(let f=0;f<l;f++)for(let v=0;v<o;v++){let S=v+c*f,b=v+c*(f+1),M=v+1+c*(f+1),E=v+1+c*f;p.push(S,b,E),p.push(b,M,E)}this.setIndex(p),this.setAttribute("position",new ut(g,3)),this.setAttribute("normal",new ut(_,3)),this.setAttribute("uv",new ut(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.widthSegments,e.heightSegments)}},os=class s extends bt{constructor(e=.5,t=1,n=32,i=1,r=0,a=Math.PI*2){super(),this.type="RingGeometry",this.parameters={innerRadius:e,outerRadius:t,thetaSegments:n,phiSegments:i,thetaStart:r,thetaLength:a},n=Math.max(3,n),i=Math.max(1,i);let o=[],l=[],c=[],h=[],u=e,d=(t-e)/i,p=new k,g=new Be;for(let _=0;_<=i;_++){for(let m=0;m<=n;m++){let f=r+m/n*a;p.x=u*Math.cos(f),p.y=u*Math.sin(f),l.push(p.x,p.y,p.z),c.push(0,0,1),g.x=(p.x/t+1)/2,g.y=(p.y/t+1)/2,h.push(g.x,g.y)}u+=d}for(let _=0;_<i;_++){let m=_*(n+1);for(let f=0;f<n;f++){let v=f+m,S=v,b=v+n+1,M=v+n+2,E=v+1;o.push(S,b,E),o.push(b,M,E)}}this.setIndex(o),this.setAttribute("position",new ut(l,3)),this.setAttribute("normal",new ut(c,3)),this.setAttribute("uv",new ut(h,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.innerRadius,e.outerRadius,e.thetaSegments,e.phiSegments,e.thetaStart,e.thetaLength)}};var vi=class s extends bt{constructor(e=1,t=32,n=16,i=0,r=Math.PI*2,a=0,o=Math.PI){super(),this.type="SphereGeometry",this.parameters={radius:e,widthSegments:t,heightSegments:n,phiStart:i,phiLength:r,thetaStart:a,thetaLength:o},t=Math.max(3,Math.floor(t)),n=Math.max(2,Math.floor(n));let l=Math.min(a+o,Math.PI),c=0,h=[],u=new k,d=new k,p=[],g=[],_=[],m=[];for(let f=0;f<=n;f++){let v=[],S=f/n,b=a+S*o,M=e*Math.cos(b),E=Math.sqrt(e*e-M*M),P=0;f===0&&a===0?P=.5/t:f===n&&l===Math.PI&&(P=-.5/t);for(let x=0;x<=t;x++){let C=x/t,w=i+C*r;u.x=-E*Math.cos(w),u.y=M,u.z=E*Math.sin(w),g.push(u.x,u.y,u.z),d.copy(u).normalize(),_.push(d.x,d.y,d.z),m.push(C+P,1-S),v.push(c++)}h.push(v)}for(let f=0;f<n;f++)for(let v=0;v<t;v++){let S=h[f][v+1],b=h[f][v],M=h[f+1][v],E=h[f+1][v+1];(f!==0||a>0)&&p.push(S,b,E),(f!==n-1||l<Math.PI)&&p.push(b,M,E)}this.setIndex(p),this.setAttribute("position",new ut(g,3)),this.setAttribute("normal",new ut(_,3)),this.setAttribute("uv",new ut(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.widthSegments,e.heightSegments,e.phiStart,e.phiLength,e.thetaStart,e.thetaLength)}};var Bn=class s extends bt{constructor(e=1,t=.4,n=12,i=48,r=Math.PI*2,a=0,o=Math.PI*2){super(),this.type="TorusGeometry",this.parameters={radius:e,tube:t,radialSegments:n,tubularSegments:i,arc:r,thetaStart:a,thetaLength:o},n=Math.floor(n),i=Math.floor(i);let l=[],c=[],h=[],u=[],d=new k,p=new k,g=new k;for(let _=0;_<=n;_++){let m=a+_/n*o;for(let f=0;f<=i;f++){let v=f/i*r;p.x=(e+t*Math.cos(m))*Math.cos(v),p.y=(e+t*Math.cos(m))*Math.sin(v),p.z=t*Math.sin(m),c.push(p.x,p.y,p.z),d.x=e*Math.cos(v),d.y=e*Math.sin(v),g.subVectors(p,d).normalize(),h.push(g.x,g.y,g.z),u.push(f/i),u.push(_/n)}}for(let _=1;_<=n;_++)for(let m=1;m<=i;m++){let f=(i+1)*_+m-1,v=(i+1)*(_-1)+m-1,S=(i+1)*(_-1)+m,b=(i+1)*_+m;l.push(f,v,b),l.push(v,S,b)}this.setIndex(l),this.setAttribute("position",new ut(c,3)),this.setAttribute("normal",new ut(h,3)),this.setAttribute("uv",new ut(u,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.tube,e.radialSegments,e.tubularSegments,e.arc,e.thetaStart,e.thetaLength)}};var Ma=class s extends bt{constructor(e=new si(new k(-1,-1,0),new k(-1,1,0),new k(1,1,0)),t=64,n=1,i=8,r=!1){super(),this.type="TubeGeometry",this.parameters={path:e,tubularSegments:t,radius:n,radialSegments:i,closed:r};let a=e.computeFrenetFrames(t,r);this.tangents=a.tangents,this.normals=a.normals,this.binormals=a.binormals;let o=new k,l=new k,c=new Be,h=new k,u=[],d=[],p=[],g=[];_(),this.setIndex(g),this.setAttribute("position",new ut(u,3)),this.setAttribute("normal",new ut(d,3)),this.setAttribute("uv",new ut(p,2));function _(){for(let S=0;S<t;S++)m(S);m(r===!1?t:0),v(),f()}function m(S){h=e.getPointAt(S/t,h);let b=a.normals[S],M=a.binormals[S];for(let E=0;E<=i;E++){let P=E/i*Math.PI*2,x=Math.sin(P),C=-Math.cos(P);l.x=C*b.x+x*M.x,l.y=C*b.y+x*M.y,l.z=C*b.z+x*M.z,l.normalize(),d.push(l.x,l.y,l.z),o.x=h.x+n*l.x,o.y=h.y+n*l.y,o.z=h.z+n*l.z,u.push(o.x,o.y,o.z)}}function f(){for(let S=1;S<=t;S++)for(let b=1;b<=i;b++){let M=(i+1)*(S-1)+(b-1),E=(i+1)*S+(b-1),P=(i+1)*S+b,x=(i+1)*(S-1)+b;g.push(M,E,x),g.push(E,P,x)}}function v(){for(let S=0;S<=t;S++)for(let b=0;b<=i;b++)c.x=S/t,c.y=b/i,p.push(c.x,c.y)}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}toJSON(){let e=super.toJSON();return e.path=this.parameters.path.toJSON(),e}static fromJSON(e){return new s(new oh[e.path.type]().fromJSON(e.path),e.tubularSegments,e.radius,e.radialSegments,e.closed)}},Sa=class extends bt{constructor(e=null){if(super(),this.type="WireframeGeometry",this.parameters={geometry:e},e!==null){let t=[],n=new Set,i=new k,r=new k;if(e.index!==null){let a=e.attributes.position,o=e.index,l=e.groups;l.length===0&&(l=[{start:0,count:o.count,materialIndex:0}]);for(let c=0,h=l.length;c<h;++c){let u=l[c],d=u.start,p=u.count;for(let g=d,_=d+p;g<_;g+=3)for(let m=0;m<3;m++){let f=o.getX(g+m),v=o.getX(g+(m+1)%3);i.fromBufferAttribute(a,f),r.fromBufferAttribute(a,v),Md(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}}else{let a=e.attributes.position;for(let o=0,l=a.count/3;o<l;o++)for(let c=0;c<3;c++){let h=3*o+c,u=3*o+(c+1)%3;i.fromBufferAttribute(a,h),r.fromBufferAttribute(a,u),Md(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}this.setAttribute("position",new ut(t,3))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}};function Md(s,e,t){let n=`${s.x},${s.y},${s.z}-${e.x},${e.y},${e.z}`,i=`${e.x},${e.y},${e.z}-${s.x},${s.y},${s.z}`;return t.has(n)===!0||t.has(i)===!0?!1:(t.add(n),t.add(i),!0)}function ks(s){let e={};for(let t in s){e[t]={};for(let n in s[t]){let i=s[t][n];if(Sd(i))i.isRenderTargetTexture?(nt("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),e[t][n]=null):e[t][n]=i.clone();else if(Array.isArray(i))if(Sd(i[0])){let r=[];for(let a=0,o=i.length;a<o;a++)r[a]=i[a].clone();e[t][n]=r}else e[t][n]=i.slice();else e[t][n]=i}}return e}function vn(s){let e={};for(let t=0;t<s.length;t++){let n=ks(s[t]);for(let i in n)e[i]=n[i]}return e}function Sd(s){return s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)}function Km(s){let e=[];for(let t=0;t<s.length;t++)e.push(s[t].clone());return e}function Dh(s){let e=s.getRenderTarget();return e===null?s.outputColorSpace:e.isXRRenderTarget===!0?e.texture.colorSpace:_t.workingColorSpace}var ci={clone:ks,merge:vn},Ym=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,Jm=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,vt=class extends En{constructor(e){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=Ym,this.fragmentShader=Jm,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,e!==void 0&&this.setValues(e)}copy(e){return super.copy(e),this.fragmentShader=e.fragmentShader,this.vertexShader=e.vertexShader,this.uniforms=ks(e.uniforms),this.uniformsGroups=Km(e.uniformsGroups),this.defines=Object.assign({},e.defines),this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.fog=e.fog,this.lights=e.lights,this.clipping=e.clipping,this.extensions=Object.assign({},e.extensions),this.glslVersion=e.glslVersion,this.defaultAttributeValues=Object.assign({},e.defaultAttributeValues),this.index0AttributeName=e.index0AttributeName,this.uniformsNeedUpdate=e.uniformsNeedUpdate,this}toJSON(e){let t=super.toJSON(e);t.glslVersion=this.glslVersion,t.uniforms={};for(let i in this.uniforms){let a=this.uniforms[i].value;a&&a.isTexture?t.uniforms[i]={type:"t",value:a.toJSON(e).uuid}:a&&a.isColor?t.uniforms[i]={type:"c",value:a.getHex()}:a&&a.isVector2?t.uniforms[i]={type:"v2",value:a.toArray()}:a&&a.isVector3?t.uniforms[i]={type:"v3",value:a.toArray()}:a&&a.isVector4?t.uniforms[i]={type:"v4",value:a.toArray()}:a&&a.isMatrix3?t.uniforms[i]={type:"m3",value:a.toArray()}:a&&a.isMatrix4?t.uniforms[i]={type:"m4",value:a.toArray()}:t.uniforms[i]={value:a}}Object.keys(this.defines).length>0&&(t.defines=this.defines),t.vertexShader=this.vertexShader,t.fragmentShader=this.fragmentShader,t.lights=this.lights,t.clipping=this.clipping;let n={};for(let i in this.extensions)this.extensions[i]===!0&&(n[i]=!0);return Object.keys(n).length>0&&(t.extensions=n),t}fromJSON(e,t){if(super.fromJSON(e,t),e.uniforms!==void 0)for(let n in e.uniforms){let i=e.uniforms[n];switch(this.uniforms[n]={},i.type){case"t":this.uniforms[n].value=t[i.value]||null;break;case"c":this.uniforms[n].value=new Le().setHex(i.value);break;case"v2":this.uniforms[n].value=new Be().fromArray(i.value);break;case"v3":this.uniforms[n].value=new k().fromArray(i.value);break;case"v4":this.uniforms[n].value=new Pt().fromArray(i.value);break;case"m3":this.uniforms[n].value=new mt().fromArray(i.value);break;case"m4":this.uniforms[n].value=new pt().fromArray(i.value);break;default:this.uniforms[n].value=i.value}}if(e.defines!==void 0&&(this.defines=e.defines),e.vertexShader!==void 0&&(this.vertexShader=e.vertexShader),e.fragmentShader!==void 0&&(this.fragmentShader=e.fragmentShader),e.glslVersion!==void 0&&(this.glslVersion=e.glslVersion),e.extensions!==void 0)for(let n in e.extensions)this.extensions[n]=e.extensions[n];return e.lights!==void 0&&(this.lights=e.lights),e.clipping!==void 0&&(this.clipping=e.clipping),this}},Ar=class extends vt{constructor(e){super(e),this.isRawShaderMaterial=!0,this.type="RawShaderMaterial"}},Rn=class extends En{constructor(e){super(),this.isMeshStandardMaterial=!0,this.type="MeshStandardMaterial",this.defines={STANDARD:""},this.color=new Le(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Le(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Ja,this.normalScale=new Be(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Tn,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.defines={STANDARD:""},this.color.copy(e.color),this.roughness=e.roughness,this.metalness=e.metalness,this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.roughnessMap=e.roughnessMap,this.metalnessMap=e.metalnessMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.envMapIntensity=e.envMapIntensity,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},Cn=class extends Rn{constructor(e){super(),this.isMeshPhysicalMaterial=!0,this.defines={STANDARD:"",PHYSICAL:""},this.type="MeshPhysicalMaterial",this.anisotropyRotation=0,this.anisotropyMap=null,this.clearcoatMap=null,this.clearcoatRoughness=0,this.clearcoatRoughnessMap=null,this.clearcoatNormalScale=new Be(1,1),this.clearcoatNormalMap=null,this.ior=1.5,Object.defineProperty(this,"reflectivity",{get:function(){return yt(2.5*(this.ior-1)/(this.ior+1),0,1)},set:function(t){this.ior=(1+.4*t)/(1-.4*t)}}),this.iridescenceMap=null,this.iridescenceIOR=1.3,this.iridescenceThicknessRange=[100,400],this.iridescenceThicknessMap=null,this.sheenColor=new Le(0),this.sheenColorMap=null,this.sheenRoughness=1,this.sheenRoughnessMap=null,this.transmissionMap=null,this.thickness=0,this.thicknessMap=null,this.attenuationDistance=1/0,this.attenuationColor=new Le(1,1,1),this.specularIntensity=1,this.specularIntensityMap=null,this.specularColor=new Le(1,1,1),this.specularColorMap=null,this._anisotropy=0,this._clearcoat=0,this._dispersion=0,this._iridescence=0,this._retroreflectivity=0,this._sheen=0,this._transmission=0,this.setValues(e)}get anisotropy(){return this._anisotropy}set anisotropy(e){this._anisotropy>0!=e>0&&this.version++,this._anisotropy=e}get clearcoat(){return this._clearcoat}set clearcoat(e){this._clearcoat>0!=e>0&&this.version++,this._clearcoat=e}get iridescence(){return this._iridescence}set iridescence(e){this._iridescence>0!=e>0&&this.version++,this._iridescence=e}get dispersion(){return this._dispersion}set dispersion(e){this._dispersion>0!=e>0&&this.version++,this._dispersion=e}get retroreflectivity(){return this._retroreflectivity}set retroreflectivity(e){this._retroreflectivity>0!=e>0&&this.version++,this._retroreflectivity=e}get sheen(){return this._sheen}set sheen(e){this._sheen>0!=e>0&&this.version++,this._sheen=e}get transmission(){return this._transmission}set transmission(e){this._transmission>0!=e>0&&this.version++,this._transmission=e}copy(e){return super.copy(e),this.defines={STANDARD:"",PHYSICAL:""},this.anisotropy=e.anisotropy,this.anisotropyRotation=e.anisotropyRotation,this.anisotropyMap=e.anisotropyMap,this.clearcoat=e.clearcoat,this.clearcoatMap=e.clearcoatMap,this.clearcoatRoughness=e.clearcoatRoughness,this.clearcoatRoughnessMap=e.clearcoatRoughnessMap,this.clearcoatNormalMap=e.clearcoatNormalMap,this.clearcoatNormalScale.copy(e.clearcoatNormalScale),this.dispersion=e.dispersion,this.ior=e.ior,this.iridescence=e.iridescence,this.iridescenceMap=e.iridescenceMap,this.iridescenceIOR=e.iridescenceIOR,this.iridescenceThicknessRange=[...e.iridescenceThicknessRange],this.iridescenceThicknessMap=e.iridescenceThicknessMap,this.retroreflectivity=e.retroreflectivity,this.sheen=e.sheen,this.sheenColor.copy(e.sheenColor),this.sheenColorMap=e.sheenColorMap,this.sheenRoughness=e.sheenRoughness,this.sheenRoughnessMap=e.sheenRoughnessMap,this.transmission=e.transmission,this.transmissionMap=e.transmissionMap,this.thickness=e.thickness,this.thicknessMap=e.thicknessMap,this.attenuationDistance=e.attenuationDistance,this.attenuationColor.copy(e.attenuationColor),this.specularIntensity=e.specularIntensity,this.specularIntensityMap=e.specularIntensityMap,this.specularColor.copy(e.specularColor),this.specularColorMap=e.specularColorMap,this}};var wa=class extends En{constructor(e){super(),this.isMeshLambertMaterial=!0,this.type="MeshLambertMaterial",this.color=new Le(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Le(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Ja,this.normalScale=new Be(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Tn,this.combine=bl,this.reflectivity=1,this.envMapIntensity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.envMapIntensity=e.envMapIntensity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},rl=class extends En{constructor(e){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=of,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(e)}copy(e){return super.copy(e),this.depthPacking=e.depthPacking,this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this}},al=class extends En{constructor(e){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(e)}copy(e){return super.copy(e),this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this}};function ns(s,e){return!s||s.constructor===e?s:typeof e.BYTES_PER_ELEMENT=="number"?new e(s):Array.prototype.slice.call(s)}function zo(s){return s!==void 0&&s.inTangents!==void 0&&s.outTangents!==void 0}function Zm(s){function e(i,r){return s[i]-s[r]}let t=s.length,n=new Array(t);for(let i=0;i!==t;++i)n[i]=i;return n.sort(e),n}function wd(s,e,t){let n=s.length,i=new s.constructor(n);for(let r=0,a=0;a!==n;++r){let o=t[r]*e;for(let l=0;l!==e;++l)i[a++]=s[o+l]}return i}function $m(s,e,t,n){let i=1,r=s[0];for(;r!==void 0&&r[n]===void 0;)r=s[i++];if(r===void 0)return;let a=r[n];if(a!==void 0)if(Array.isArray(a))do a=r[n],a!==void 0&&(e.push(r.time),t.push(...a)),r=s[i++];while(r!==void 0);else if(a.toArray!==void 0)do a=r[n],a!==void 0&&(e.push(r.time),a.toArray(t,t.length)),r=s[i++];while(r!==void 0);else do a=r[n],a!==void 0&&(e.push(r.time),t.push(a)),r=s[i++];while(r!==void 0)}var _i=class{constructor(e,t,n,i){this.parameterPositions=e,this._cachedIndex=0,this.resultBuffer=i!==void 0?i:new t.constructor(n),this.sampleValues=t,this.valueSize=n,this.settings=null,this.DefaultSettings_={}}evaluate(e){let t=this.parameterPositions,n=this._cachedIndex,i=t[n],r=t[n-1];e:{t:{let a;n:{i:if(!(e<i)){for(let o=n+2;;){if(i===void 0){if(e<r)break i;return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}if(n===o)break;if(r=i,i=t[++n],e<i)break t}a=t.length;break n}if(!(e>=r)){let o=t[1];e<o&&(n=2,r=o);for(let l=n-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(n===l)break;if(i=r,r=t[--n-1],e>=r)break t}a=n,n=0;break n}break e}for(;n<a;){let o=n+a>>>1;e<t[o]?a=o:n=o+1}if(i=t[n],r=t[n-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===void 0)return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}this._cachedIndex=n,this.intervalChanged_(n,r,i)}return this.interpolate_(n,r,e,i)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i;for(let a=0;a!==i;++a)t[a]=n[r+a];return t}interpolate_(){throw new Error("THREE.Interpolant: Call to abstract method.")}intervalChanged_(){}},ol=class extends _i{constructor(e,t,n,i){super(e,t,n,i),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:Ss,endingEnd:Ss}}intervalChanged_(e,t,n){let i=this.parameterPositions,r=e-2,a=e+1,o=i[r],l=i[a];if(o===void 0)switch(this.getSettings_().endingStart){case ws:r=e,o=2*t-n;break;case la:r=i.length-2,o=t+i[r]-i[r+1];break;default:r=e,o=n}if(l===void 0)switch(this.getSettings_().endingEnd){case ws:a=e,l=2*n-t;break;case la:a=1,l=n+i[1]-i[0];break;default:a=e-1,l=t}let c=(n-t)*.5,h=this.valueSize;this._weightPrev=c/(t-o),this._weightNext=c/(l-n),this._offsetPrev=r*h,this._offsetNext=a*h}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=this._offsetPrev,u=this._offsetNext,d=this._weightPrev,p=this._weightNext,g=(n-t)/(i-t),_=g*g,m=_*g,f=-d*m+2*d*_-d*g,v=(1+d)*m+(-1.5-2*d)*_+(-.5+d)*g+1,S=(-1-p)*m+(1.5+p)*_+.5*g,b=p*m-p*_;for(let M=0;M!==o;++M)r[M]=f*a[h+M]+v*a[c+M]+S*a[l+M]+b*a[u+M];return r}},Ea=class extends _i{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=(n-t)/(i-t),u=1-h;for(let d=0;d!==o;++d)r[d]=a[c+d]*u+a[l+d]*h;return r}},ll=class extends _i{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e){return this.copySampleValue_(e-1)}},cl=class extends _i{interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=this.inTangents,u=this.outTangents;if(!h||!u){let g=(n-t)/(i-t),_=1-g;for(let m=0;m!==o;++m)r[m]=a[c+m]*_+a[l+m]*g;return r}let d=o*2,p=e-1;for(let g=0;g!==o;++g){let _=a[c+g],m=a[l+g],f=p*d+g*2,v=u[f],S=u[f+1],b=e*d+g*2,M=h[b],E=h[b+1],P=eg(n,t,v,M,i);r[g]=yf(P,_,S,E,m)}return r}};function yf(s,e,t,n,i){let r=1-s;return r*r*r*e+3*r*r*s*t+3*r*s*s*n+s*s*s*i}function Qm(s,e,t,n,i){let r=1-s;return 3*r*r*(t-e)+6*r*s*(n-t)+3*s*s*(i-n)}function eg(s,e,t,n,i){let r=(s-e)/(i-e);for(let a=0;a<8;a++){let o=yf(r,e,t,n,i)-s;if(Math.abs(o)<1e-10)break;let l=Qm(r,e,t,n,i);if(Math.abs(l)<1e-10)break;r=Math.max(0,Math.min(1,r-o/l))}return r}var Pn=class{constructor(e,t,n,i){if(e===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(t===void 0||t.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+e);this.name=e,this.times=ns(t,this.TimeBufferType),this.values=ns(n,this.ValueBufferType),this.setInterpolation(i||this.DefaultInterpolation)}static toJSON(e){let t=e.constructor,n;if(t.toJSON!==this.toJSON)n=t.toJSON(e);else{n={name:e.name,times:ns(e.times,Array),values:ns(e.values,Array)};let i=e.getInterpolation();i!==e.DefaultInterpolation&&(n.interpolation=i),zo(e.settings)&&(n.settings={inTangents:ns(e.settings.inTangents,Array),outTangents:ns(e.settings.outTangents,Array)})}return n.type=e.ValueTypeName,n}InterpolantFactoryMethodDiscrete(e){return new ll(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodLinear(e){return new Ea(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodSmooth(e){return new ol(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodBezier(e){let t=new cl(this.times,this.values,this.getValueSize(),e);return this.settings&&(t.inTangents=this.settings.inTangents,t.outTangents=this.settings.outTangents),t}setInterpolation(e){let t;switch(e){case Ts:t=this.InterpolantFactoryMethodDiscrete;break;case As:t=this.InterpolantFactoryMethodLinear;break;case Uo:t=this.InterpolantFactoryMethodSmooth;break;case rh:t=this.InterpolantFactoryMethodBezier;break}if(t===void 0){let n="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(e!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(n);return nt("KeyframeTrack:",n),this}return this.createInterpolant=t,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return Ts;case this.InterpolantFactoryMethodLinear:return As;case this.InterpolantFactoryMethodSmooth:return Uo;case this.InterpolantFactoryMethodBezier:return rh}}getValueSize(){return this.values.length/this.times.length}shift(e){if(e!==0){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]+=e}return this}scale(e){if(e!==1){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]*=e;zo(this.settings)&&(Ed(this.settings.inTangents,e),Ed(this.settings.outTangents,e))}return this}trim(e,t){let n=this.times,i=n.length,r=0,a=i-1;for(;r!==i&&n[r]<e;)++r;for(;a!==-1&&n[a]>t;)--a;if(++a,r!==0||a!==i){r>=a&&(a=Math.max(a,1),r=a-1);let o=this.getValueSize();this.times=n.slice(r,a),this.values=this.values.slice(r*o,a*o)}return this}validate(){let e=!0,t=this.getValueSize();t-Math.floor(t)!==0&&(ht("KeyframeTrack: Invalid value size in track.",this),e=!1);let n=this.times,i=this.values,r=n.length;r===0&&(ht("KeyframeTrack: Track is empty.",this),e=!1);let a=null;for(let o=0;o!==r;o++){let l=n[o];if(typeof l=="number"&&isNaN(l)){ht("KeyframeTrack: Time is not a valid number.",this,o,l),e=!1;break}if(a!==null&&a>l){ht("KeyframeTrack: Out of order keys.",this,o,l,a),e=!1;break}a=l}if(i!==void 0&&im(i))for(let o=0,l=i.length;o!==l;++o){let c=i[o];if(isNaN(c)){ht("KeyframeTrack: Value is not a valid number.",this,o,c),e=!1;break}}return e}optimize(){let e=this.times.slice(),t=this.values.slice(),n=this.getValueSize(),i=this.getInterpolation()===Uo,r=e.length-1,a=1;for(let o=1;o<r;++o){let l=!1,c=e[o],h=e[o+1];if(c!==h&&(o!==1||c!==e[0]))if(i)l=!0;else{let u=o*n,d=u-n,p=u+n;for(let g=0;g!==n;++g){let _=t[u+g];if(_!==t[d+g]||_!==t[p+g]){l=!0;break}}}if(l){if(o!==a){e[a]=e[o];let u=o*n,d=a*n;for(let p=0;p!==n;++p)t[d+p]=t[u+p]}++a}}if(r>0){e[a]=e[r];for(let o=r*n,l=a*n,c=0;c!==n;++c)t[l+c]=t[o+c];++a}return a!==e.length?(this.times=e.slice(0,a),this.values=t.slice(0,a*n)):(this.times=e,this.values=t),this}clone(){let e=this.times.slice(),t=this.values.slice(),n=this.constructor,i=new n(this.name,e,t);return i.createInterpolant=this.createInterpolant,zo(this.settings)&&(i.settings={inTangents:this.settings.inTangents.slice(),outTangents:this.settings.outTangents.slice()}),i}};function Ed(s,e){for(let t=0,n=s.length;t!==n;t+=2)s[t]*=e}Pn.prototype.ValueTypeName="";Pn.prototype.TimeBufferType=Float32Array;Pn.prototype.ValueBufferType=Float32Array;Pn.prototype.DefaultInterpolation=As;var zi=class extends Pn{constructor(e,t,n){super(e,t,n)}};zi.prototype.ValueTypeName="bool";zi.prototype.ValueBufferType=Array;zi.prototype.DefaultInterpolation=Ts;zi.prototype.InterpolantFactoryMethodLinear=void 0;zi.prototype.InterpolantFactoryMethodSmooth=void 0;var Ta=class extends Pn{constructor(e,t,n,i){super(e,t,n,i)}};Ta.prototype.ValueTypeName="color";var Bi=class extends Pn{constructor(e,t,n,i){super(e,t,n,i)}};Bi.prototype.ValueTypeName="number";var hl=class extends _i{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=(n-t)/(i-t),c=e*o;for(let h=c+o;c!==h;c+=4)Zt.slerpFlat(r,0,a,c-o,a,c,l);return r}},ki=class extends Pn{constructor(e,t,n,i){super(e,t,n,i)}InterpolantFactoryMethodLinear(e){return new hl(this.times,this.values,this.getValueSize(),e)}};ki.prototype.ValueTypeName="quaternion";ki.prototype.InterpolantFactoryMethodSmooth=void 0;var Hi=class extends Pn{constructor(e,t,n){super(e,t,n)}};Hi.prototype.ValueTypeName="string";Hi.prototype.ValueBufferType=Array;Hi.prototype.DefaultInterpolation=Ts;Hi.prototype.InterpolantFactoryMethodLinear=void 0;Hi.prototype.InterpolantFactoryMethodSmooth=void 0;var ls=class extends Pn{constructor(e,t,n,i){super(e,t,n,i)}};ls.prototype.ValueTypeName="vector";var Ls=class{constructor(e="",t=-1,n=[],i=rc){this.name=e,this.tracks=n,this.duration=t,this.blendMode=i,this.uuid=ii(),this.userData={},this.duration<0&&this.resetDuration()}static parse(e){let t=[],n=e.tracks,i=1/(e.fps||1);for(let a=0,o=n.length;a!==o;++a)t.push(ng(n[a]).scale(i));let r=new this(e.name,e.duration,t,e.blendMode);return r.uuid=e.uuid,r.userData=JSON.parse(e.userData||"{}"),r}static toJSON(e){let t=[],n=e.tracks,i={name:e.name,duration:e.duration,tracks:t,uuid:e.uuid,blendMode:e.blendMode,userData:JSON.stringify(e.userData)};for(let r=0,a=n.length;r!==a;++r)t.push(Pn.toJSON(n[r]));return i}static CreateFromMorphTargetSequence(e,t,n,i){let r=t.length,a=[];for(let o=0;o<r;o++){let l=[],c=[];l.push((o+r-1)%r,o,(o+1)%r),c.push(0,1,0);let h=Zm(l);l=wd(l,1,h),c=wd(c,1,h),!i&&l[0]===0&&(l.push(r),c.push(c[0])),a.push(new Bi(".morphTargetInfluences["+t[o].name+"]",l,c).scale(1/n))}return new this(e,-1,a)}static findByName(e,t){let n=e;if(!Array.isArray(e)){let i=e;n=i.geometry&&i.geometry.animations||i.animations}for(let i=0;i<n.length;i++)if(n[i].name===t)return n[i];return null}static CreateClipsFromMorphTargetSequences(e,t,n){let i={},r=/^([\w-]*?)([\d]+)$/;for(let o=0,l=e.length;o<l;o++){let c=e[o],h=c.name.match(r);if(h&&h.length>1){let u=h[1],d=i[u];d||(i[u]=d=[]),d.push(c)}}let a=[];for(let o in i)a.push(this.CreateFromMorphTargetSequence(o,i[o],t,n));return a}resetDuration(){let e=this.tracks,t=0;for(let n=0,i=e.length;n!==i;++n){let r=this.tracks[n];t=Math.max(t,r.times[r.times.length-1])}return this.duration=t,this}trim(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].trim(0,this.duration);return this}validate(){let e=!0;for(let t=0;t<this.tracks.length;t++)e=e&&this.tracks[t].validate();return e}optimize(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].optimize();return this}clone(){let e=[];for(let n=0;n<this.tracks.length;n++)e.push(this.tracks[n].clone());let t=new this.constructor(this.name,this.duration,e,this.blendMode);return t.userData=JSON.parse(JSON.stringify(this.userData)),t}toJSON(){return this.constructor.toJSON(this)}};function tg(s){switch(s.toLowerCase()){case"scalar":case"double":case"float":case"number":case"integer":return Bi;case"vector":case"vector2":case"vector3":case"vector4":return ls;case"color":return Ta;case"quaternion":return ki;case"bool":case"boolean":return zi;case"string":return Hi}throw new Error("THREE.KeyframeTrack: Unsupported typeName: "+s)}function ng(s){if(s.type===void 0)throw new Error("THREE.KeyframeTrack: track type undefined, can not parse");let e=tg(s.type);if(s.times===void 0){let n=[],i=[];$m(s.keys,n,i,"value"),s.times=n,s.values=i}let t;return e.parse!==void 0?t=e.parse(s):t=new e(s.name,s.times,s.values,s.interpolation),zo(s.settings)&&(t.settings={inTangents:ns(s.settings.inTangents,Float32Array),outTangents:ns(s.settings.outTangents,Float32Array)}),t}var fi={enabled:!1,files:{},add:function(s,e){this.enabled!==!1&&(Td(s)||(this.files[s]=e))},get:function(s){if(this.enabled!==!1&&!Td(s))return this.files[s]},remove:function(s){delete this.files[s]},clear:function(){this.files={}}};function Td(s){try{let e=s.slice(s.indexOf(":")+1);return new URL(e).protocol==="blob:"}catch{return!1}}var ul=class{constructor(e,t,n){let i=this,r=!1,a=0,o=0,l,c=[];this.onStart=void 0,this.onLoad=e,this.onProgress=t,this.onError=n,this._abortController=null,this.itemStart=function(h){o++,r===!1&&i.onStart!==void 0&&i.onStart(h,a,o),r=!0},this.itemEnd=function(h){a++,i.onProgress!==void 0&&i.onProgress(h,a,o),a===o&&(r=!1,i.onLoad!==void 0&&i.onLoad())},this.itemError=function(h){i.onError!==void 0&&i.onError(h)},this.resolveURL=function(h){return h=h.normalize("NFC"),l?l(h):h},this.setURLModifier=function(h){return l=h,this},this.addHandler=function(h,u){return c.push(h,u),this},this.removeHandler=function(h){let u=c.indexOf(h);return u!==-1&&c.splice(u,2),this},this.getHandler=function(h){for(let u=0,d=c.length;u<d;u+=2){let p=c[u],g=c[u+1];if(p.global&&(p.lastIndex=0),p.test(h))return g}return null},this.abort=function(){return this.abortController.abort(),this._abortController=null,this}}get abortController(){return this._abortController||(this._abortController=new AbortController),this._abortController}},Mf=new ul,yi=class{constructor(e){this.manager=e!==void 0?e:Mf,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}load(){}loadAsync(e,t){let n=this;return new Promise(function(i,r){n.load(e,i,t,r)})}parse(){}setCrossOrigin(e){return this.crossOrigin=e,this}setWithCredentials(e){return this.withCredentials=e,this}setPath(e){return this.path=e,this}setResourcePath(e){return this.resourcePath=e,this}setRequestHeader(e){return this.requestHeader=e,this}abort(){return this}};yi.DEFAULT_MATERIAL_NAME="__DEFAULT";var Fi={},lh=class extends Error{constructor(e,t){super(e),this.response=t}},Rr=class extends yi{constructor(e){super(e),this.mimeType="",this.responseType="",this._abortController=new AbortController}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=fi.get(`file:${e}`);if(r!==void 0){this.manager.itemStart(e),setTimeout(()=>{t&&t(r),this.manager.itemEnd(e)},0);return}if(Fi[e]!==void 0){Fi[e].push({onLoad:t,onProgress:n,onError:i});return}Fi[e]=[],Fi[e].push({onLoad:t,onProgress:n,onError:i});let a=new Request(e,{headers:new Headers(this.requestHeader),credentials:this.withCredentials?"include":"same-origin",signal:typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal}),o=this.mimeType,l=this.responseType;fetch(a).then(c=>{if(c.status===200||c.status===0){if(c.status===0&&nt("FileLoader: HTTP Status 0 received."),typeof ReadableStream>"u"||c.body===void 0||c.body.getReader===void 0)return c;let h=Fi[e],u=c.body.getReader(),d=c.headers.get("X-File-Size")||c.headers.get("Content-Length"),p=d?parseInt(d):0,g=p!==0,_=0,m=new ReadableStream({start(f){v();function v(){u.read().then(({done:S,value:b})=>{if(S)f.close();else{_+=b.byteLength;let M=new ProgressEvent("progress",{lengthComputable:g,loaded:_,total:p});for(let E=0,P=h.length;E<P;E++){let x=h[E];x.onProgress&&x.onProgress(M)}f.enqueue(b),v()}},S=>{f.error(S)})}}});return new Response(m)}else throw new lh(`fetch for "${c.url}" responded with ${c.status}: ${c.statusText}`,c)}).then(c=>{switch(l){case"arraybuffer":return c.arrayBuffer();case"blob":return c.blob();case"document":return c.text().then(h=>new DOMParser().parseFromString(h,o));case"json":return c.json();default:if(o==="")return c.text();{let u=/charset="?([^;"\s]*)"?/i.exec(o),d=u&&u[1]?u[1].toLowerCase():void 0,p=new TextDecoder(d);return c.arrayBuffer().then(g=>p.decode(g))}}}).then(c=>{fi.add(`file:${e}`,c);let h=Fi[e];delete Fi[e];for(let u=0,d=h.length;u<d;u++){let p=h[u];p.onLoad&&p.onLoad(c)}}).catch(c=>{let h=Fi[e];if(h===void 0)throw this.manager.itemError(e),c;delete Fi[e];for(let u=0,d=h.length;u<d;u++){let p=h[u];p.onError&&p.onError(c)}this.manager.itemError(e)}).finally(()=>{this.manager.itemEnd(e)}),this.manager.itemStart(e)}setResponseType(e){return this.responseType=e,this}setMimeType(e){return this.mimeType=e,this}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var ar=new WeakMap,dl=class extends yi{constructor(e){super(e)}load(e,t,n,i){this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,a=fi.get(`image:${e}`);if(a!==void 0){if(a.complete===!0)r.manager.itemStart(e),setTimeout(function(){t&&t(a),r.manager.itemEnd(e)},0);else{let u=ar.get(a);u===void 0&&(u=[],ar.set(a,u)),u.push({onLoad:t,onError:i})}return a}let o=fr("img");function l(){h(),t&&t(this);let u=ar.get(this)||[];for(let d=0;d<u.length;d++){let p=u[d];p.onLoad&&p.onLoad(this)}ar.delete(this),r.manager.itemEnd(e)}function c(u){h(),i&&i(u),fi.remove(`image:${e}`);let d=ar.get(this)||[];for(let p=0;p<d.length;p++){let g=d[p];g.onError&&g.onError(u)}ar.delete(this),r.manager.itemError(e),r.manager.itemEnd(e)}function h(){o.removeEventListener("load",l,!1),o.removeEventListener("error",c,!1)}return o.addEventListener("load",l,!1),o.addEventListener("error",c,!1),e.slice(0,5)!=="data:"&&this.crossOrigin!==void 0&&(o.crossOrigin=this.crossOrigin),fi.add(`image:${e}`,o),r.manager.itemStart(e),o.src=e,o}};var Aa=class extends yi{constructor(e){super(e)}load(e,t,n,i){let r=new Qt,a=new dl(this.manager);return a.setCrossOrigin(this.crossOrigin),a.setPath(this.path),a.load(e,function(o){r.image=o,r.needsUpdate=!0,t!==void 0&&t(r)},n,i),r}},Fs=class extends Ft{constructor(e,t=1){super(),this.isLight=!0,this.type="Light",this.color=new Le(e),this.intensity=t}copy(e,t){return super.copy(e,t),this.color.copy(e.color),this.intensity=e.intensity,this}toJSON(e){let t=super.toJSON(e);return t.object.color=this.color.getHex(),t.object.intensity=this.intensity,t}},Ra=class extends Fs{constructor(e,t,n){super(e,n),this.isHemisphereLight=!0,this.type="HemisphereLight",this.position.copy(Ft.DEFAULT_UP),this.updateMatrix(),this.groundColor=new Le(t)}copy(e,t){return super.copy(e,t),this.groundColor.copy(e.groundColor),this}toJSON(e){let t=super.toJSON(e);return t.object.groundColor=this.groundColor.getHex(),t}},nh=new pt,Ad=new k,Rd=new k,Cr=class{constructor(e){this.camera=e,this.intensity=1,this.bias=0,this.biasNode=null,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new Be(512,512),this.mapType=In,this.map=null,this.mapPass=null,this.matrix=new pt,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new yr,this._frameExtents=new Be(1,1),this._viewportCount=1,this._viewports=[new Pt(0,0,1,1)]}getViewportCount(){return this._viewportCount}getCamera(){return this.camera}getFrustum(){return this._frustum}updateMatrices(e){let t=this.camera;Ad.setFromMatrixPosition(e.matrixWorld),t.position.copy(Ad),Rd.setFromMatrixPosition(e.target.matrixWorld),t.lookAt(Rd),t.updateMatrixWorld(),this._updateMatrix(t,this.matrix,this._frustum)}_updateMatrix(e,t,n,i){nh.multiplyMatrices(e.projectionMatrix,e.matrixWorldInverse),n.setFromProjectionMatrix(nh,e.coordinateSystem,e.reversedDepth);let r=this._frameExtents,a=i?i.z/r.x:1,o=i?i.w/r.y:1,l=i?i.x/r.x:0,c=i?i.y/r.y:0;e.coordinateSystem===dr||e.reversedDepth?t.set(.5*a,0,0,.5*a+l,0,.5*o,0,.5*o+c,0,0,1,0,0,0,0,1):t.set(.5*a,0,0,.5*a+l,0,.5*o,0,.5*o+c,0,0,.5,.5,0,0,0,1),t.multiply(nh)}getViewport(e){return this._viewports[e]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(e){return this.camera=e.camera.clone(),this.intensity=e.intensity,this.bias=e.bias,this.radius=e.radius,this.autoUpdate=e.autoUpdate,this.needsUpdate=e.needsUpdate,this.normalBias=e.normalBias,this.blurSamples=e.blurSamples,this.mapSize.copy(e.mapSize),this.biasNode=e.biasNode,this}clone(){return new this.constructor().copy(this)}toJSON(){let e={};return e.intensity=this.intensity,e.bias=this.bias,e.normalBias=this.normalBias,e.radius=this.radius,e.blurSamples=this.blurSamples,e.mapSize=this.mapSize.toArray(),e.camera=this.camera.toJSON(!1).object,delete e.camera.matrix,e}},Fo=new k,No=new Zt,di=new k,Ca=class extends Ft{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new pt,this.projectionMatrix=new pt,this.projectionMatrixInverse=new pt,this.coordinateSystem=ni,this._reversedDepth=!1}get reversedDepth(){return this._reversedDepth}copy(e,t){return super.copy(e,t),this.matrixWorldInverse.copy(e.matrixWorldInverse),this.projectionMatrix.copy(e.projectionMatrix),this.projectionMatrixInverse.copy(e.projectionMatrixInverse),this.coordinateSystem=e.coordinateSystem,this}getWorldDirection(e){return super.getWorldDirection(e).negate()}updateMatrixWorld(e){super.updateMatrixWorld(e),this.matrixWorld.decompose(Fo,No,di),di.x===1&&di.y===1&&di.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Fo,No,di.set(1,1,1)).invert()}updateWorldMatrix(e,t,n=!1){super.updateWorldMatrix(e,t,n),this.matrixWorld.decompose(Fo,No,di),di.x===1&&di.y===1&&di.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Fo,No,di.set(1,1,1)).invert()}clone(){return new this.constructor().copy(this)}},ts=new k,Cd=new Be,Pd=new Be,sn=class extends Ca{constructor(e=50,t=1,n=.1,i=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=e,this.zoom=1,this.near=n,this.far=i,this.focus=10,this.aspect=t,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.fov=e.fov,this.zoom=e.zoom,this.near=e.near,this.far=e.far,this.focus=e.focus,this.aspect=e.aspect,this.view=e.view===null?null:Object.assign({},e.view),this.filmGauge=e.filmGauge,this.filmOffset=e.filmOffset,this}setFocalLength(e){let t=.5*this.getFilmHeight()/e;this.fov=Rs*2*Math.atan(t),this.updateProjectionMatrix()}getFocalLength(){let e=Math.tan(sa*.5*this.fov);return .5*this.getFilmHeight()/e}getEffectiveFOV(){return Rs*2*Math.atan(Math.tan(sa*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(e,t,n){ts.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),t.set(ts.x,ts.y).multiplyScalar(-e/ts.z),ts.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),n.set(ts.x,ts.y).multiplyScalar(-e/ts.z)}getViewSize(e,t){return this.getViewBounds(e,Cd,Pd),t.subVectors(Pd,Cd)}setViewOffset(e,t,n,i,r,a){this.aspect=e/t,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=a,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=this.near,t=e*Math.tan(sa*.5*this.fov)/this.zoom,n=2*t,i=this.aspect*n,r=-.5*i,a=this.view;if(this.view!==null&&this.view.enabled){let l=a.fullWidth,c=a.fullHeight;r+=a.offsetX*i/l,t-=a.offsetY*n/c,i*=a.width/l,n*=a.height/c}let o=this.filmOffset;o!==0&&(r+=e*o/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+i,t,t-n,e,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.fov=this.fov,t.object.zoom=this.zoom,t.object.near=this.near,t.object.far=this.far,t.object.focus=this.focus,t.object.aspect=this.aspect,this.view!==null&&(t.object.view=Object.assign({},this.view)),t.object.filmGauge=this.filmGauge,t.object.filmOffset=this.filmOffset,t}},ch=class extends Cr{constructor(){super(new sn(50,1,.5,500)),this.isSpotLightShadow=!0,this.focus=1,this.aspect=1}updateMatrices(e){let t=this.camera,n=Rs*2*e.angle*this.focus,i=this.mapSize.width/this.mapSize.height*this.aspect,r=e.distance||t.far;(n!==t.fov||i!==t.aspect||r!==t.far)&&(t.fov=n,t.aspect=i,t.far=r,t.updateProjectionMatrix()),super.updateMatrices(e)}copy(e){return super.copy(e),this.focus=e.focus,this.aspect=e.aspect,this}toJSON(){let e=super.toJSON();return e.focus=this.focus,e.aspect=this.aspect,e}},Pa=class extends Fs{constructor(e,t,n=0,i=Math.PI/3,r=0,a=2){super(e,t),this.isSpotLight=!0,this.type="SpotLight",this.position.copy(Ft.DEFAULT_UP),this.updateMatrix(),this.target=new Ft,this.distance=n,this.angle=i,this.penumbra=r,this.decay=a,this.map=null,this.shadow=new ch}get power(){return this.intensity*Math.PI}set power(e){this.intensity=e/Math.PI}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.angle=e.angle,this.penumbra=e.penumbra,this.decay=e.decay,this.target=e.target.clone(),this.map=e.map,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.angle=this.angle,t.object.decay=this.decay,t.object.penumbra=this.penumbra,t.object.target=this.target.uuid,this.map&&this.map.isTexture&&(t.object.map=this.map.toJSON(e).uuid),t.object.shadow=this.shadow.toJSON(),t}},hh=class extends Cr{constructor(){super(new sn(90,1,.5,500)),this.isPointLightShadow=!0}},ri=class extends Fs{constructor(e,t,n=0,i=2){super(e,t),this.isPointLight=!0,this.type="PointLight",this.distance=n,this.decay=i,this.shadow=new hh}get power(){return this.intensity*4*Math.PI}set power(e){this.intensity=e/(4*Math.PI)}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.decay=e.decay,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.decay=this.decay,t.object.shadow=this.shadow.toJSON(),t}},Mi=class extends Ca{constructor(e=-1,t=1,n=1,i=-1,r=.1,a=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=e,this.right=t,this.top=n,this.bottom=i,this.near=r,this.far=a,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.left=e.left,this.right=e.right,this.top=e.top,this.bottom=e.bottom,this.near=e.near,this.far=e.far,this.zoom=e.zoom,this.view=e.view===null?null:Object.assign({},e.view),this}setViewOffset(e,t,n,i,r,a){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=a,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=(this.right-this.left)/(2*this.zoom),t=(this.top-this.bottom)/(2*this.zoom),n=(this.right+this.left)/2,i=(this.top+this.bottom)/2,r=n-e,a=n+e,o=i+t,l=i-t;if(this.view!==null&&this.view.enabled){let c=(this.right-this.left)/this.view.fullWidth/this.zoom,h=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=c*this.view.offsetX,a=r+c*this.view.width,o-=h*this.view.offsetY,l=o-h*this.view.height}this.projectionMatrix.makeOrthographic(r,a,o,l,this.near,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.zoom=this.zoom,t.object.left=this.left,t.object.right=this.right,t.object.top=this.top,t.object.bottom=this.bottom,t.object.near=this.near,t.object.far=this.far,this.view!==null&&(t.object.view=Object.assign({},this.view)),t}},uh=class extends Cr{constructor(){super(new Mi(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},Gi=class extends Fs{constructor(e,t){super(e,t),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(Ft.DEFAULT_UP),this.updateMatrix(),this.target=new Ft,this.shadow=new uh}dispose(){super.dispose(),this.shadow.dispose()}copy(e){return super.copy(e),this.target=e.target.clone(),this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.shadow=this.shadow.toJSON(),t.object.target=this.target.uuid,t}};var Vi=class{static extractUrlBase(e){let t=e.lastIndexOf("/");return t===-1?"./":e.slice(0,t+1)}static resolveURL(e,t){return typeof e!="string"||e===""?"":(/^https?:\/\//i.test(t)&&/^\//.test(e)&&(t=t.replace(/(^https?:\/\/[^\/]+).*/i,"$1")),/^(https?:)?\/\//i.test(e)||/^data:.*,.*$/i.test(e)||/^blob:.*$/i.test(e)?e:t+e)}};var ih=new WeakMap,Ia=class extends yi{constructor(e){super(e),this.isImageBitmapLoader=!0,typeof createImageBitmap>"u"&&nt("ImageBitmapLoader: createImageBitmap() not supported."),typeof fetch>"u"&&nt("ImageBitmapLoader: fetch() not supported."),this.options={premultiplyAlpha:"none"},this._abortController=new AbortController}setOptions(e){return this.options=e,this}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,a=fi.get(`image-bitmap:${e}`);if(a!==void 0){if(r.manager.itemStart(e),a.then){a.then(c=>{ih.has(a)===!0?(i&&i(ih.get(a)),r.manager.itemError(e),r.manager.itemEnd(e)):(t&&t(c),r.manager.itemEnd(e))});return}setTimeout(function(){t&&t(a),r.manager.itemEnd(e)},0);return}let o={};o.credentials=this.crossOrigin==="anonymous"?"same-origin":"include",o.headers=this.requestHeader,o.signal=typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal;let l=fetch(e,o).then(function(c){return c.blob()}).then(function(c){return createImageBitmap(c,Object.assign({},r.options,{colorSpaceConversion:"none"}))}).then(function(c){return fi.add(`image-bitmap:${e}`,c),t&&t(c),r.manager.itemEnd(e),c}).catch(function(c){i&&i(c),ih.set(l,c),fi.remove(`image-bitmap:${e}`),r.manager.itemError(e),r.manager.itemEnd(e)});fi.add(`image-bitmap:${e}`,l),r.manager.itemStart(e)}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var or=-90,lr=1,fl=class extends Ft{constructor(e,t,n){super(),this.type="CubeCamera",this.renderTarget=n,this.coordinateSystem=null,this.activeMipmapLevel=0;let i=new sn(or,lr,e,t);i.layers=this.layers,this.add(i);let r=new sn(or,lr,e,t);r.layers=this.layers,this.add(r);let a=new sn(or,lr,e,t);a.layers=this.layers,this.add(a);let o=new sn(or,lr,e,t);o.layers=this.layers,this.add(o);let l=new sn(or,lr,e,t);l.layers=this.layers,this.add(l);let c=new sn(or,lr,e,t);c.layers=this.layers,this.add(c)}updateCoordinateSystem(){let e=this.coordinateSystem,t=this.children.concat(),[n,i,r,a,o,l]=t;for(let c of t)this.remove(c);if(e===ni)n.up.set(0,1,0),n.lookAt(1,0,0),i.up.set(0,1,0),i.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),a.up.set(0,0,1),a.lookAt(0,-1,0),o.up.set(0,1,0),o.lookAt(0,0,1),l.up.set(0,1,0),l.lookAt(0,0,-1);else if(e===dr)n.up.set(0,-1,0),n.lookAt(-1,0,0),i.up.set(0,-1,0),i.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),a.up.set(0,0,-1),a.lookAt(0,-1,0),o.up.set(0,-1,0),o.lookAt(0,0,1),l.up.set(0,-1,0),l.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+e);for(let c of t)this.add(c),c.updateMatrixWorld()}update(e,t){this.parent===null&&this.updateMatrixWorld();let{renderTarget:n,activeMipmapLevel:i}=this;this.coordinateSystem!==e.coordinateSystem&&(this.coordinateSystem=e.coordinateSystem,this.updateCoordinateSystem());let[r,a,o,l,c,h]=this.children,u=e.getRenderTarget(),d=e.getActiveCubeFace(),p=e.getActiveMipmapLevel(),g=e.xr.enabled;e.xr.enabled=!1;let _=n.texture.generateMipmaps;n.texture.generateMipmaps=!1;let m=!1;e.isWebGLRenderer===!0?m=e.state.buffers.depth.getReversed():m=e.reversedDepthBuffer,e.setRenderTarget(n,0,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,r),e.setRenderTarget(n,1,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,a),e.setRenderTarget(n,2,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,o),e.setRenderTarget(n,3,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,l),e.setRenderTarget(n,4,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,c),n.texture.generateMipmaps=_,e.setRenderTarget(n,5,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,h),e.setRenderTarget(u,d,p),e.xr.enabled=g,n.texture.needsPMREMUpdate=!0}},pl=class extends sn{constructor(e=[]){super(),this.isArrayCamera=!0,this.isMultiViewCamera=!1,this.cameras=e}},Da=class{constructor(){this._previousTime=0,this._currentTime=0,this._startTime=performance.now(),this._delta=0,this._elapsed=0,this._timescale=1,this._document=null,this._pageVisibilityHandler=null}connect(e){this._document=e,e.hidden!==void 0&&(this._pageVisibilityHandler=ig.bind(this),e.addEventListener("visibilitychange",this._pageVisibilityHandler,!1))}disconnect(){this._pageVisibilityHandler!==null&&(this._document.removeEventListener("visibilitychange",this._pageVisibilityHandler),this._pageVisibilityHandler=null),this._document=null}getDelta(){return this._delta/1e3}getElapsed(){return this._elapsed/1e3}getTimescale(){return this._timescale}setTimescale(e){return this._timescale=e,this}reset(){return this._currentTime=performance.now()-this._startTime,this}dispose(){this.disconnect()}update(e){return this._pageVisibilityHandler!==null&&this._document.hidden===!0?this._delta=0:(this._previousTime=this._currentTime,this._currentTime=(e!==void 0?e:performance.now())-this._startTime,this._delta=(this._currentTime-this._previousTime)*this._timescale,this._elapsed+=this._delta),this}};function ig(){this._document.hidden===!1&&this.reset()}var ml=class{constructor(e,t,n){this.binding=e,this.valueSize=n;let i,r,a;switch(t){case"quaternion":i=this._slerp,r=this._slerpAdditive,a=this._setAdditiveIdentityQuaternion,this.buffer=new Float64Array(n*6),this._workIndex=5;break;case"string":case"bool":i=this._select,r=this._select,a=this._setAdditiveIdentityOther,this.buffer=new Array(n*5);break;default:i=this._lerp,r=this._lerpAdditive,a=this._setAdditiveIdentityNumeric,this.buffer=new Float64Array(n*5)}this._mixBufferRegion=i,this._mixBufferRegionAdditive=r,this._setIdentity=a,this._origIndex=3,this._addIndex=4,this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,this.useCount=0,this.referenceCount=0}accumulate(e,t){let n=this.buffer,i=this.valueSize,r=e*i+i,a=this.cumulativeWeight;if(a===0){for(let o=0;o!==i;++o)n[r+o]=n[o];a=t}else{a+=t;let o=t/a;this._mixBufferRegion(n,r,0,o,i)}this.cumulativeWeight=a}accumulateAdditive(e){let t=this.buffer,n=this.valueSize,i=n*this._addIndex;this.cumulativeWeightAdditive===0&&this._setIdentity(),this._mixBufferRegionAdditive(t,i,0,e,n),this.cumulativeWeightAdditive+=e}apply(e){let t=this.valueSize,n=this.buffer,i=e*t+t,r=this.cumulativeWeight,a=this.cumulativeWeightAdditive,o=this.binding;if(this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,r<1){let l=t*this._origIndex;this._mixBufferRegion(n,i,l,1-r,t)}a>0&&this._mixBufferRegionAdditive(n,i,this._addIndex*t,1,t);for(let l=t,c=t+t;l!==c;++l)if(n[l]!==n[l+t]){o.setValue(n,i);break}}saveOriginalState(){let e=this.binding,t=this.buffer,n=this.valueSize,i=n*this._origIndex;e.getValue(t,i);for(let r=n,a=i;r!==a;++r)t[r]=t[i+r%n];this._setIdentity(),this.cumulativeWeight=0,this.cumulativeWeightAdditive=0}restoreOriginalState(){let e=this.valueSize*3;this.binding.setValue(this.buffer,e)}_setAdditiveIdentityNumeric(){let e=this._addIndex*this.valueSize,t=e+this.valueSize;for(let n=e;n<t;n++)this.buffer[n]=0}_setAdditiveIdentityQuaternion(){this._setAdditiveIdentityNumeric(),this.buffer[this._addIndex*this.valueSize+3]=1}_setAdditiveIdentityOther(){let e=this._origIndex*this.valueSize,t=this._addIndex*this.valueSize;for(let n=0;n<this.valueSize;n++)this.buffer[t+n]=this.buffer[e+n]}_select(e,t,n,i,r){if(i>=.5)for(let a=0;a!==r;++a)e[t+a]=e[n+a]}_slerp(e,t,n,i){Zt.slerpFlat(e,t,e,t,e,n,i)}_slerpAdditive(e,t,n,i,r){let a=this._workIndex*r;Zt.multiplyQuaternionsFlat(e,a,e,t,e,n),Zt.slerpFlat(e,t,e,t,e,a,i)}_lerp(e,t,n,i,r){let a=1-i;for(let o=0;o!==r;++o){let l=t+o;e[l]=e[l]*a+e[n+o]*i}}_lerpAdditive(e,t,n,i,r){for(let a=0;a!==r;++a){let o=t+a;e[o]=e[o]+e[n+a]*i}}},Lh="\\[\\]\\.:\\/",sg=new RegExp("["+Lh+"]","g"),Fh="[^"+Lh+"]",rg="[^"+Lh.replace("\\.","")+"]",ag=/((?:WC+[\/:])*)/.source.replace("WC",Fh),og=/(WCOD+)?/.source.replace("WCOD",rg),lg=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",Fh),cg=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",Fh),hg=new RegExp("^"+ag+og+lg+cg+"$"),ug=["material","materials","bones","map"],dh=class{constructor(e,t,n){let i=n||kt.parseTrackName(t);this._targetGroup=e,this._bindings=e.subscribe_(t,i)}getValue(e,t){this.bind();let n=this._targetGroup.nCachedObjects_,i=this._bindings[n];i!==void 0&&i.getValue(e,t)}setValue(e,t){let n=this._bindings;for(let i=this._targetGroup.nCachedObjects_,r=n.length;i!==r;++i)n[i].setValue(e,t)}bind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].bind()}unbind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].unbind()}},kt=class s{constructor(e,t,n){this.path=t,this.parsedPath=n||s.parseTrackName(t),this.node=s.findNode(e,this.parsedPath.nodeName),this.rootNode=e,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(e,t,n){return e&&e.isAnimationObjectGroup?new s.Composite(e,t,n):new s(e,t,n)}static sanitizeNodeName(e){return e.replace(/\s/g,"_").replace(sg,"")}static parseTrackName(e){let t=hg.exec(e);if(t===null)throw new Error("THREE.PropertyBinding: Cannot parse trackName: "+e);let n={nodeName:t[2],objectName:t[3],objectIndex:t[4],propertyName:t[5],propertyIndex:t[6]},i=n.nodeName&&n.nodeName.lastIndexOf(".");if(i!==void 0&&i!==-1){let r=n.nodeName.substring(i+1);ug.indexOf(r)!==-1&&(n.nodeName=n.nodeName.substring(0,i),n.objectName=r)}if(n.propertyName===null||n.propertyName.length===0)throw new Error("THREE.PropertyBinding: can not parse propertyName from trackName: "+e);return n}static findNode(e,t){if(t===void 0||t===""||t==="."||t===-1||t===e.name||t===e.uuid)return e;if(e.skeleton){let n=e.skeleton.getBoneByName(t);if(n!==void 0)return n}if(e.children){let n=function(r){for(let a=0;a<r.length;a++){let o=r[a];if(o.name===t||o.uuid===t)return o;let l=n(o.children);if(l)return l}return null},i=n(e.children);if(i)return i}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(e,t){e[t]=this.targetObject[this.propertyName]}_getValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)e[t++]=n[i]}_getValue_arrayElement(e,t){e[t]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(e,t){this.resolvedProperty.toArray(e,t)}_setValue_direct(e,t){this.targetObject[this.propertyName]=e[t]}_setValue_direct_setNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++]}_setValue_array_setNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(e,t){this.resolvedProperty[this.propertyIndex]=e[t]}_setValue_arrayElement_setNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(e,t){this.resolvedProperty.fromArray(e,t)}_setValue_fromArray_setNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(e,t){this.bind(),this.getValue(e,t)}_setValue_unbound(e,t){this.bind(),this.setValue(e,t)}bind(){let e=this.node,t=this.parsedPath,n=t.objectName,i=t.propertyName,r=t.propertyIndex;if(e||(e=s.findNode(this.rootNode,t.nodeName),this.node=e),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!e){nt("PropertyBinding: No target node found for track: "+this.path+".");return}if(n){let c=t.objectIndex;switch(n){case"materials":if(!e.material){ht("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.materials){ht("PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}e=e.material.materials;break;case"bones":if(!e.skeleton){ht("PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}e=e.skeleton.bones;for(let h=0;h<e.length;h++)if(e[h].name===c){c=h;break}break;case"map":if("map"in e){e=e.map;break}if(!e.material){ht("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.map){ht("PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}e=e.material.map;break;default:if(e[n]===void 0){ht("PropertyBinding: Can not bind to objectName of node undefined.",this);return}e=e[n]}if(c!==void 0){if(e[c]===void 0){ht("PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,e);return}e=e[c]}}let a=e[i];if(a===void 0){let c=t.nodeName;ht("PropertyBinding: Trying to update property for track: "+c+"."+i+" but it wasn't found.",e);return}let o=this.Versioning.None;this.targetObject=e,e.isMaterial===!0?o=this.Versioning.NeedsUpdate:e.isObject3D===!0&&(o=this.Versioning.MatrixWorldNeedsUpdate);let l=this.BindingType.Direct;if(r!==void 0){if(i==="morphTargetInfluences"){if(!e.geometry){ht("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!e.geometry.morphAttributes){ht("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}e.morphTargetDictionary[r]!==void 0&&(r=e.morphTargetDictionary[r])}l=this.BindingType.ArrayElement,this.resolvedProperty=a,this.propertyIndex=r}else a.fromArray!==void 0&&a.toArray!==void 0?(l=this.BindingType.HasFromToArray,this.resolvedProperty=a):Array.isArray(a)?(l=this.BindingType.EntireArray,this.resolvedProperty=a):this.propertyName=i;this.getValue=this.GetterByBindingType[l],this.setValue=this.SetterByBindingTypeAndVersioning[l][o]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};kt.Composite=dh;kt.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};kt.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};kt.prototype.GetterByBindingType=[kt.prototype._getValue_direct,kt.prototype._getValue_array,kt.prototype._getValue_arrayElement,kt.prototype._getValue_toArray];kt.prototype.SetterByBindingTypeAndVersioning=[[kt.prototype._setValue_direct,kt.prototype._setValue_direct_setNeedsUpdate,kt.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[kt.prototype._setValue_array,kt.prototype._setValue_array_setNeedsUpdate,kt.prototype._setValue_array_setMatrixWorldNeedsUpdate],[kt.prototype._setValue_arrayElement,kt.prototype._setValue_arrayElement_setNeedsUpdate,kt.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[kt.prototype._setValue_fromArray,kt.prototype._setValue_fromArray_setNeedsUpdate,kt.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var gl=class{constructor(e,t,n=null,i=t.blendMode){this._mixer=e,this._clip=t,this._localRoot=n,this.blendMode=i;let r=t.tracks,a=r.length,o=new Array(a),l={endingStart:Ss,endingEnd:Ss};for(let c=0;c!==a;++c){let h=r[c].createInterpolant(null);o[c]=h,h.settings=l}this._interpolantSettings=l,this._interpolants=o,this._propertyBindings=new Array(a),this._cacheIndex=null,this._byClipCacheIndex=null,this._timeScaleInterpolant=null,this._restoreTimeScale=null,this._weightInterpolant=null,this.loop=sf,this._loopCount=-1,this._startTime=null,this.time=0,this.timeScale=1,this._effectiveTimeScale=1,this.weight=1,this._effectiveWeight=1,this.repetitions=1/0,this.paused=!1,this.enabled=!0,this.clampWhenFinished=!1,this.zeroSlopeAtStart=!0,this.zeroSlopeAtEnd=!0}play(){return this._mixer._activateAction(this),this}stop(){return this._mixer._deactivateAction(this),this.reset()}reset(){return this.paused=!1,this.enabled=!0,this.time=0,this._loopCount=-1,this._startTime=null,this.stopFading().stopWarping()}isRunning(){return this.enabled&&!this.paused&&this.timeScale!==0&&this._startTime===null&&this._mixer._isActiveAction(this)}isScheduled(){return this._mixer._isActiveAction(this)}startAt(e){return this._startTime=e,this}setLoop(e,t){return this.loop=e,this.repetitions=t,this}setEffectiveWeight(e){return this.weight=e,this._effectiveWeight=this.enabled?e:0,this.stopFading()}getEffectiveWeight(){return this._effectiveWeight}fadeIn(e){return this._scheduleFading(e,0,1)}fadeOut(e){return this._scheduleFading(e,1,0)}crossFadeFrom(e,t,n=!1){if(e.fadeOut(t),this.fadeIn(t),n===!0){let i=this._clip.duration,r=e._clip.duration,a=r/i,o=i/r;e._restoreTimeScale=e.timeScale,this._restoreTimeScale=this.timeScale,e.warp(1,a,t),this.warp(o,1,t)}return this}crossFadeTo(e,t,n=!1){return e.crossFadeFrom(this,t,n)}stopFading(){let e=this._weightInterpolant;return e!==null&&(this._weightInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this}setEffectiveTimeScale(e){return this.timeScale=e,this._effectiveTimeScale=this.paused?0:e,this.stopWarping()}getEffectiveTimeScale(){return this._effectiveTimeScale}setDuration(e){return this.timeScale=this._clip.duration/e,this.stopWarping()}syncWith(e){return this.time=e.time,this.timeScale=e.timeScale,this.stopWarping()}halt(e){return this.warp(this._effectiveTimeScale,0,e)}warp(e,t,n){let i=this._mixer,r=i.time,a=this.timeScale,o=this._timeScaleInterpolant;o===null&&(o=i._lendControlInterpolant(),this._timeScaleInterpolant=o);let l=o.parameterPositions,c=o.sampleValues;return l[0]=r,l[1]=r+n,c[0]=e/a,c[1]=t/a,this}stopWarping(){let e=this._timeScaleInterpolant;return e!==null&&(this._timeScaleInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this._restoreTimeScale=null,this}getMixer(){return this._mixer}getClip(){return this._clip}getRoot(){return this._localRoot||this._mixer._root}_update(e,t,n,i){if(!this.enabled){this._updateWeight(e);return}let r=this._startTime;if(r!==null){let l=(e-r)*n;l<0||n===0?t=0:(this._startTime=null,t=n*l)}t*=this._updateTimeScale(e);let a=this._updateTime(t),o=this._updateWeight(e);if(o>0){let l=this._interpolants,c=this._propertyBindings;switch(this.blendMode){case af:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(a),c[h].accumulateAdditive(o);break;case rc:default:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(a),c[h].accumulate(i,o)}}}_updateWeight(e){let t=0;if(this.enabled){t=this.weight;let n=this._weightInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(this.stopFading(),i===0&&(this.enabled=!1))}}return this._effectiveWeight=t,t}_updateTimeScale(e){let t=0;if(!this.paused){t=this.timeScale;let n=this._timeScaleInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(t===0?this.paused=!0:(this._restoreTimeScale!==null&&(t=this._restoreTimeScale),this.timeScale=t),this.stopWarping())}}return this._effectiveTimeScale=t,t}_updateTime(e){let t=this._clip.duration,n=this.loop,i=this.time+e,r=this._loopCount,a=n===rf;if(e===0)return r===-1?i:a&&(r&1)===1?t-i:i;if(n===sc){r===-1&&(this._loopCount=0,this._setEndings(!0,!0,!1));e:{if(i>=t)i=t;else if(i<0)i=0;else{this.time=i;break e}this.clampWhenFinished?this.paused=!0:this.enabled=!1,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e<0?-1:1})}}else{if(r===-1&&(e>=0?(r=0,this._setEndings(!0,this.repetitions===0,a)):this._setEndings(this.repetitions===0,!0,a)),i>=t||i<0){let o=Math.floor(i/t);i-=t*o,r+=Math.abs(o);let l=this.repetitions-r;if(l<=0)this.clampWhenFinished?this.paused=!0:this.enabled=!1,i=e>0?t:0,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e>0?1:-1});else{if(l===1){let c=e<0;this._setEndings(c,!c,a)}else this._setEndings(!1,!1,a);this._loopCount=r,this.time=i,this._mixer.dispatchEvent({type:"loop",action:this,loopDelta:o})}}else this._loopCount=r,this.time=i;if(a&&(r&1)===1)return t-i}return i}_setEndings(e,t,n){let i=this._interpolantSettings;n?(i.endingStart=ws,i.endingEnd=ws):(e?i.endingStart=this.zeroSlopeAtStart?ws:Ss:i.endingStart=la,t?i.endingEnd=this.zeroSlopeAtEnd?ws:Ss:i.endingEnd=la)}_scheduleFading(e,t,n){let i=this._mixer,r=i.time,a=this._weightInterpolant;a===null&&(a=i._lendControlInterpolant(),this._weightInterpolant=a);let o=a.parameterPositions,l=a.sampleValues;return o[0]=r,l[0]=t,o[1]=r+e,l[1]=n,this}},dg=new Float32Array(1),La=class extends Xn{constructor(e){super(),this._root=e,this._initMemoryManager(),this._accuIndex=0,this.time=0,this.timeScale=1,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}_bindAction(e,t){let n=e._localRoot||this._root,i=e._clip.tracks,r=i.length,a=e._propertyBindings,o=e._interpolants,l=n.uuid,c=this._bindingsByRootAndName,h=c[l];h===void 0&&(h={},c[l]=h);for(let u=0;u!==r;++u){let d=i[u],p=d.name,g=h[p];if(g!==void 0)++g.referenceCount,a[u]=g;else{if(g=a[u],g!==void 0){g._cacheIndex===null&&(++g.referenceCount,this._addInactiveBinding(g,l,p));continue}let _=t&&t._propertyBindings[u].binding.parsedPath;g=new ml(kt.create(n,p,_),d.ValueTypeName,d.getValueSize()),++g.referenceCount,this._addInactiveBinding(g,l,p),a[u]=g}o[u].resultBuffer=g.buffer}}_activateAction(e){if(!this._isActiveAction(e)){if(e._cacheIndex===null){let n=(e._localRoot||this._root).uuid,i=e._clip.uuid,r=this._actionsByClip[i];this._bindAction(e,r&&r.knownActions[0]),this._addInactiveAction(e,i,n)}let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];r.useCount++===0&&(this._lendBinding(r),r.saveOriginalState())}this._lendAction(e)}}_deactivateAction(e){if(this._isActiveAction(e)){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.useCount===0&&(r.restoreOriginalState(),this._takeBackBinding(r))}this._takeBackAction(e)}}_initMemoryManager(){this._actions=[],this._nActiveActions=0,this._actionsByClip={},this._bindings=[],this._nActiveBindings=0,this._bindingsByRootAndName={},this._controlInterpolants=[],this._nActiveControlInterpolants=0;let e=this;this.stats={actions:{get total(){return e._actions.length},get inUse(){return e._nActiveActions}},bindings:{get total(){return e._bindings.length},get inUse(){return e._nActiveBindings}},controlInterpolants:{get total(){return e._controlInterpolants.length},get inUse(){return e._nActiveControlInterpolants}}}}_isActiveAction(e){let t=e._cacheIndex;return t!==null&&t<this._nActiveActions}_addInactiveAction(e,t,n){let i=this._actions,r=this._actionsByClip,a=r[t];if(a===void 0)a={knownActions:[e],actionByRoot:{}},e._byClipCacheIndex=0,r[t]=a;else{let o=a.knownActions;e._byClipCacheIndex=o.length,o.push(e)}e._cacheIndex=i.length,i.push(e),a.actionByRoot[n]=e}_removeInactiveAction(e){let t=this._actions,n=t[t.length-1],i=e._cacheIndex;n._cacheIndex=i,t[i]=n,t.pop(),e._cacheIndex=null;let r=e._clip.uuid,a=this._actionsByClip,o=a[r],l=o.knownActions,c=l[l.length-1],h=e._byClipCacheIndex;c._byClipCacheIndex=h,l[h]=c,l.pop(),e._byClipCacheIndex=null;let u=o.actionByRoot,d=(e._localRoot||this._root).uuid;delete u[d],l.length===0&&delete a[r],this._removeInactiveBindingsForAction(e)}_removeInactiveBindingsForAction(e){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.referenceCount===0&&this._removeInactiveBinding(r)}}_lendAction(e){let t=this._actions,n=e._cacheIndex,i=this._nActiveActions++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackAction(e){let t=this._actions,n=e._cacheIndex,i=--this._nActiveActions,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_addInactiveBinding(e,t,n){let i=this._bindingsByRootAndName,r=this._bindings,a=i[t];a===void 0&&(a={},i[t]=a),a[n]=e,e._cacheIndex=r.length,r.push(e)}_removeInactiveBinding(e){let t=this._bindings,n=e.binding,i=n.rootNode.uuid,r=n.path,a=this._bindingsByRootAndName,o=a[i],l=t[t.length-1],c=e._cacheIndex;l._cacheIndex=c,t[c]=l,t.pop(),delete o[r],Object.keys(o).length===0&&delete a[i]}_lendBinding(e){let t=this._bindings,n=e._cacheIndex,i=this._nActiveBindings++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackBinding(e){let t=this._bindings,n=e._cacheIndex,i=--this._nActiveBindings,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_lendControlInterpolant(){let e=this._controlInterpolants,t=this._nActiveControlInterpolants++,n=e[t];return n===void 0&&(n=new Ea(new Float32Array(2),new Float32Array(2),1,dg),n.__cacheIndex=t,e[t]=n),n}_takeBackControlInterpolant(e){let t=this._controlInterpolants,n=e.__cacheIndex,i=--this._nActiveControlInterpolants,r=t[i];e.__cacheIndex=i,t[i]=e,r.__cacheIndex=n,t[n]=r}clipAction(e,t,n){let i=t||this._root,r=i.uuid,a=typeof e=="string"?Ls.findByName(i,e):e,o=a!==null?a.uuid:e,l=this._actionsByClip[o],c=null;if(n===void 0&&(a!==null?n=a.blendMode:n=rc),l!==void 0){let u=l.actionByRoot[r];if(u!==void 0&&u.blendMode===n)return u;c=l.knownActions[0],a===null&&(a=c._clip)}if(a===null)return null;let h=new gl(this,a,t,n);return this._bindAction(h,c),this._addInactiveAction(h,o,r),h}existingAction(e,t){let n=t||this._root,i=n.uuid,r=typeof e=="string"?Ls.findByName(n,e):e,a=r?r.uuid:e,o=this._actionsByClip[a];return o!==void 0&&o.actionByRoot[i]||null}stopAllAction(){let e=this._actions,t=this._nActiveActions;for(let n=t-1;n>=0;--n)e[n].stop();return this}update(e){e*=this.timeScale;let t=this._actions,n=this._nActiveActions,i=this.time+=e,r=Math.sign(e),a=this._accuIndex^=1;for(let c=0;c!==n;++c)t[c]._update(i,e,r,a);let o=this._bindings,l=this._nActiveBindings;for(let c=0;c!==l;++c)o[c].apply(a);return this}setTime(e){this.time=0;for(let t=0;t<this._actions.length;t++)this._actions[t].time=0;return this.update(e)}getRoot(){return this._root}uncacheClip(e){let t=this._actions,n=e.uuid,i=this._actionsByClip,r=i[n];if(r!==void 0){let a=r.knownActions;for(let o=0,l=a.length;o!==l;++o){let c=a[o];this._deactivateAction(c);let h=c._cacheIndex,u=t[t.length-1];c._cacheIndex=null,c._byClipCacheIndex=null,u._cacheIndex=h,t[h]=u,t.pop(),this._removeInactiveBindingsForAction(c)}delete i[n]}}uncacheRoot(e){let t=e.uuid,n=this._actionsByClip;for(let a in n){let o=n[a].actionByRoot,l=o[t];l!==void 0&&(this._deactivateAction(l),this._removeInactiveAction(l))}let i=this._bindingsByRootAndName,r=i[t];if(r!==void 0)for(let a in r){let o=r[a];o.restoreOriginalState(),this._removeInactiveBinding(o)}}uncacheAction(e,t){let n=this.existingAction(e,t);n!==null&&(this._deactivateAction(n),this._removeInactiveAction(n))}};var Id=new pt,Fa=class{constructor(e,t,n=0,i=1/0){this.ray=new gi(e,t),this.near=n,this.far=i,this.camera=null,this.layers=new gr,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(e,t){this.ray.set(e,t)}setFromCamera(e,t){t.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(e.x,e.y,.5).unproject(t).sub(this.ray.origin).normalize(),this.camera=t):t.isOrthographicCamera?(this.ray.origin.set(e.x,e.y,t.projectionMatrix.elements[14]).unproject(t),this.ray.direction.set(0,0,-1).transformDirection(t.matrixWorld),this.camera=t):ht("Raycaster: Unsupported camera type: "+t.type)}setFromXRController(e){return Id.identity().extractRotation(e.matrixWorld),this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(Id),this}intersectObject(e,t=!0,n=[]){return fh(e,this,n,t),n.sort(Dd),n}intersectObjects(e,t=!0,n=[]){for(let i=0,r=e.length;i<r;i++)fh(e[i],this,n,t);return n.sort(Dd),n}};function Dd(s,e){return s.distance-e.distance}function fh(s,e,t,n){let i=!0;if(s.layers.test(e.layers)&&s.raycast(e,t)===!1&&(i=!1),i===!0&&n===!0){let r=s.children;for(let a=0,o=r.length;a<o;a++)fh(r[a],e,t,!0)}}var Pr=class{constructor(e=1,t=0,n=0){this.radius=e,this.phi=t,this.theta=n}set(e,t,n){return this.radius=e,this.phi=t,this.theta=n,this}copy(e){return this.radius=e.radius,this.phi=e.phi,this.theta=e.theta,this}makeSafe(){return this.phi=yt(this.phi,1e-6,Math.PI-1e-6),this}setFromVector3(e){return this.setFromCartesianCoords(e.x,e.y,e.z)}setFromCartesianCoords(e,t,n){return this.radius=Math.sqrt(e*e+t*t+n*n),this.radius===0?(this.theta=0,this.phi=0):(this.theta=Math.atan2(e,n),this.phi=Math.acos(yt(t/this.radius,-1,1))),this}clone(){return new this.constructor().copy(this)}};var ph=class s{static{s.prototype.isMatrix2=!0}constructor(e,t,n,i){this.elements=[1,0,0,1],e!==void 0&&this.set(e,t,n,i)}identity(){return this.set(1,0,0,1),this}fromArray(e,t=0){for(let n=0;n<4;n++)this.elements[n]=e[n+t];return this}set(e,t,n,i){let r=this.elements;return r[0]=e,r[2]=t,r[1]=n,r[3]=i,this}};var Na=class extends Xn{constructor(e,t=null){super(),this.object=e,this.domElement=t,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(e){this.domElement!==null&&this.disconnect(),this.domElement=e}disconnect(){}dispose(){}update(){}};function Nh(s,e,t,n){let i=fg(n);switch(t){case Eh:return s*e;case wl:return s*e/i.components*i.byteLength;case El:return s*e/i.components*i.byteLength;case fs:return s*e*2/i.components*i.byteLength;case Tl:return s*e*2/i.components*i.byteLength;case Th:return s*e*3/i.components*i.byteLength;case Hn:return s*e*4/i.components*i.byteLength;case Al:return s*e*4/i.components*i.byteLength;case Va:case Wa:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case qa:case Xa:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Cl:case Il:return Math.max(s,16)*Math.max(e,8)/4;case Rl:case Pl:return Math.max(s,8)*Math.max(e,8)/2;case Dl:case Ll:case Nl:case Ul:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case Fl:case ja:case Ol:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case zl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Bl:return Math.floor((s+4)/5)*Math.floor((e+3)/4)*16;case kl:return Math.floor((s+4)/5)*Math.floor((e+4)/5)*16;case Hl:return Math.floor((s+5)/6)*Math.floor((e+4)/5)*16;case Gl:return Math.floor((s+5)/6)*Math.floor((e+5)/6)*16;case Vl:return Math.floor((s+7)/8)*Math.floor((e+4)/5)*16;case Wl:return Math.floor((s+7)/8)*Math.floor((e+5)/6)*16;case ql:return Math.floor((s+7)/8)*Math.floor((e+7)/8)*16;case Xl:return Math.floor((s+9)/10)*Math.floor((e+4)/5)*16;case jl:return Math.floor((s+9)/10)*Math.floor((e+5)/6)*16;case Kl:return Math.floor((s+9)/10)*Math.floor((e+7)/8)*16;case Yl:return Math.floor((s+9)/10)*Math.floor((e+9)/10)*16;case Jl:return Math.floor((s+11)/12)*Math.floor((e+9)/10)*16;case Zl:return Math.floor((s+11)/12)*Math.floor((e+11)/12)*16;case $l:case Ql:case ec:return Math.ceil(s/4)*Math.ceil(e/4)*16;case tc:case nc:return Math.ceil(s/4)*Math.ceil(e/4)*8;case Ka:case ic:return Math.ceil(s/4)*Math.ceil(e/4)*16}throw new Error(`Unable to determine texture byte length for ${t} format.`)}function fg(s){switch(s){case In:case yh:return{byteLength:1,components:1};case Fr:case Mh:case rn:return{byteLength:2,components:1};case Ml:case Sl:return{byteLength:2,components:4};case li:case yl:case kn:return{byteLength:4,components:1};case Sh:case wh:return{byteLength:4,components:3}}throw new Error(`THREE.TextureUtils: Unknown texture type ${s}.`)}typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"186"}}));typeof window<"u"&&(window.__THREE__?nt("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="186");function Wf(){let s=null,e=!1,t=null,n=null;function i(r,a){n=s.requestAnimationFrame(i),t(r,a)}return{start:function(){e!==!0&&t!==null&&s!==null&&(n=s.requestAnimationFrame(i),e=!0)},stop:function(){s!==null&&s.cancelAnimationFrame(n),e=!1},setAnimationLoop:function(r){t=r},setContext:function(r){s=r}}}function mg(s){let e=new WeakMap;function t(o,l){let c=o.array,h=o.usage,u=c.byteLength,d=s.createBuffer();s.bindBuffer(l,d),s.bufferData(l,c,h),o.onUploadCallback();let p;if(c instanceof Float32Array)p=s.FLOAT;else if(typeof Float16Array<"u"&&c instanceof Float16Array)p=s.HALF_FLOAT;else if(c instanceof Uint16Array)o.isFloat16BufferAttribute?p=s.HALF_FLOAT:p=s.UNSIGNED_SHORT;else if(c instanceof Int16Array)p=s.SHORT;else if(c instanceof Uint32Array)p=s.UNSIGNED_INT;else if(c instanceof Int32Array)p=s.INT;else if(c instanceof Int8Array)p=s.BYTE;else if(c instanceof Uint8Array)p=s.UNSIGNED_BYTE;else if(c instanceof Uint8ClampedArray)p=s.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+c);return{buffer:d,type:p,bytesPerElement:c.BYTES_PER_ELEMENT,version:o.version,size:u}}function n(o,l,c){let h=l.array,u=l.updateRanges;if(s.bindBuffer(c,o),u.length===0)s.bufferSubData(c,0,h);else{u.sort((p,g)=>p.start-g.start);let d=0;for(let p=1;p<u.length;p++){let g=u[d],_=u[p];_.start<=g.start+g.count+1?g.count=Math.max(g.count,_.start+_.count-g.start):(++d,u[d]=_)}u.length=d+1;for(let p=0,g=u.length;p<g;p++){let _=u[p];s.bufferSubData(c,_.start*h.BYTES_PER_ELEMENT,h,_.start,_.count)}l.clearUpdateRanges()}l.onUploadCallback()}function i(o){return o.isInterleavedBufferAttribute&&(o=o.data),e.get(o)}function r(o){o.isInterleavedBufferAttribute&&(o=o.data);let l=e.get(o);l&&(s.deleteBuffer(l.buffer),e.delete(o))}function a(o,l){if(o.isInterleavedBufferAttribute&&(o=o.data),o.isGLBufferAttribute){let h=e.get(o);(!h||h.version<o.version)&&e.set(o,{buffer:o.buffer,type:o.type,bytesPerElement:o.elementSize,version:o.version});return}let c=e.get(o);if(c===void 0)e.set(o,t(o,l));else if(c.version<o.version){if(c.size!==o.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");n(c.buffer,o,l),c.version=o.version}}return{get:i,remove:r,update:a}}var gg=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,bg=`#ifdef USE_ALPHAHASH
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
#endif`,xg=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,vg=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,_g=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,yg=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,Mg=`#ifdef USE_AOMAP
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
#endif`,Sg=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,wg=`#ifdef USE_BATCHING
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
#endif`,Eg=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,Tg=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,Ag=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,Rg=`float G_BlinnPhong_Implicit( ) {
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
} // validated`,Cg=`#ifdef USE_IRIDESCENCE
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
#endif`,Pg=`#ifdef USE_BUMPMAP
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
#endif`,Ig=`#if NUM_CLIPPING_PLANES > 0
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
#endif`,Dg=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,Lg=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,Fg=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,Ng=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#endif`,Ug=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#endif`,Og=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec4 vColor;
#endif`,zg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
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
#endif`,Bg=`#define PI 3.141592653589793
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
} // validated`,kg=`#ifdef ENVMAP_TYPE_CUBE_UV
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
#endif`,Hg=`vec3 transformedNormal = objectNormal;
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
#endif`,Gg=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,Vg=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,Wg=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	#ifdef DECODE_VIDEO_TEXTURE_EMISSIVE
		emissiveColor = sRGBTransferEOTF( emissiveColor );
	#endif
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,qg=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,Xg="gl_FragColor = linearToOutputTexel( gl_FragColor );",jg=`vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferEOTF( in vec4 value ) {
	return vec4( mix( pow( value.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), value.rgb * 0.0773993808, vec3( lessThanEqual( value.rgb, vec3( 0.04045 ) ) ) ), value.a );
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,Kg=`#ifdef USE_ENVMAP
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
#endif`,Yg=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
#endif`,Jg=`#ifdef USE_ENVMAP
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
#endif`,Zg=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,$g=`#ifdef USE_ENVMAP
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
#endif`,Qg=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,e0=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,t0=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,n0=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,i0=`#ifdef USE_GRADIENTMAP
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
}`,s0=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,r0=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,a0=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,o0=`uniform bool receiveShadow;
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
#include <lightprobes_pars_fragment>`,l0=`#ifdef USE_ENVMAP
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
#endif`,c0=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,h0=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,u0=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,d0=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,f0=`PhysicalMaterial material;
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
#endif`,p0=`uniform sampler2D dfgLUT;
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
}`,m0=`
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
#endif`,g0=`#if defined( RE_IndirectDiffuse )
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
#endif`,b0=`#if defined( RE_IndirectDiffuse )
	#if defined( LAMBERT ) || defined( PHONG )
		irradiance += iblIrradiance;
	#endif
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,x0=`#ifdef USE_LIGHT_PROBES_GRID
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
#endif`,v0=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,_0=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,y0=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,M0=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,S0=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = sRGBTransferEOTF( sampledDiffuseColor );
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,w0=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,E0=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
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
#endif`,T0=`#if defined( USE_POINTS_UV )
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
#endif`,A0=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,R0=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,C0=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,P0=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,I0=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,D0=`#ifdef USE_MORPHTARGETS
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
#endif`,L0=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,F0=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
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
vec3 nonPerturbedNormal = normal;`,N0=`#ifdef USE_NORMALMAP_OBJECTSPACE
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
#endif`,U0=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,O0=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,z0=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
		#ifdef FLIP_SIDED
			vBitangent = - vBitangent;
		#endif
	#endif
#endif`,B0=`#ifdef USE_NORMALMAP
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
#endif`,k0=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,H0=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,G0=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,V0=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,W0=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,q0=`vec3 packNormalToRGB( const in vec3 normal ) {
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
}`,X0=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,j0=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,K0=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,Y0=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,J0=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,Z0=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,$0=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,Q0=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,eb=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_SUN_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
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
#endif`,tb=`float getShadowMask() {
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
}`,nb=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,ib=`#ifdef USE_SKINNING
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
#endif`,sb=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,rb=`#ifdef USE_SKINNING
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
#endif`,ab=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,ob=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,lb=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,cb=`#ifndef saturate
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
vec3 CustomToneMapping( vec3 color ) { return color; }`,hb=`#ifdef USE_TRANSMISSION
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
#endif`,ub=`#ifdef USE_TRANSMISSION
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
#endif`,db=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,fb=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,pb=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,mb=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,gb=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,bb=`uniform sampler2D t2D;
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
}`,xb=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,vb=`#ifdef ENVMAP_TYPE_CUBE
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
}`,_b=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,yb=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,Mb=`#include <common>
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
}`,Sb=`#if DEPTH_PACKING == 3200
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
}`,wb=`#define DISTANCE
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
}`,Eb=`#define DISTANCE
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
}`,Tb=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,Ab=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,Rb=`uniform float scale;
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
}`,Cb=`uniform vec3 diffuse;
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
}`,Pb=`#include <common>
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
}`,Ib=`uniform vec3 diffuse;
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
}`,Db=`#define LAMBERT
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
}`,Lb=`#define LAMBERT
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
}`,Fb=`#define MATCAP
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
}`,Nb=`#define MATCAP
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
}`,Ub=`#define NORMAL
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
}`,Ob=`#define NORMAL
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
}`,zb=`#define PHONG
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
}`,Bb=`#define PHONG
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
}`,kb=`#define STANDARD
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
}`,Hb=`#define STANDARD
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
}`,Gb=`#define TOON
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
}`,Vb=`#define TOON
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
}`,Wb=`uniform float size;
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
}`,qb=`uniform vec3 diffuse;
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
}`,Xb=`#include <common>
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
}`,jb=`uniform vec3 color;
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
}`,Kb=`uniform float rotation;
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
}`,Yb=`uniform vec3 diffuse;
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
}`,St={alphahash_fragment:gg,alphahash_pars_fragment:bg,alphamap_fragment:xg,alphamap_pars_fragment:vg,alphatest_fragment:_g,alphatest_pars_fragment:yg,aomap_fragment:Mg,aomap_pars_fragment:Sg,batching_pars_vertex:wg,batching_vertex:Eg,begin_vertex:Tg,beginnormal_vertex:Ag,bsdfs:Rg,iridescence_fragment:Cg,bumpmap_pars_fragment:Pg,clipping_planes_fragment:Ig,clipping_planes_pars_fragment:Dg,clipping_planes_pars_vertex:Lg,clipping_planes_vertex:Fg,color_fragment:Ng,color_pars_fragment:Ug,color_pars_vertex:Og,color_vertex:zg,common:Bg,cube_uv_reflection_fragment:kg,defaultnormal_vertex:Hg,displacementmap_pars_vertex:Gg,displacementmap_vertex:Vg,emissivemap_fragment:Wg,emissivemap_pars_fragment:qg,colorspace_fragment:Xg,colorspace_pars_fragment:jg,envmap_fragment:Kg,envmap_common_pars_fragment:Yg,envmap_pars_fragment:Jg,envmap_pars_vertex:Zg,envmap_physical_pars_fragment:l0,envmap_vertex:$g,fog_vertex:Qg,fog_pars_vertex:e0,fog_fragment:t0,fog_pars_fragment:n0,gradientmap_pars_fragment:i0,lightmap_pars_fragment:s0,lights_lambert_fragment:r0,lights_lambert_pars_fragment:a0,lights_pars_begin:o0,lights_toon_fragment:c0,lights_toon_pars_fragment:h0,lights_phong_fragment:u0,lights_phong_pars_fragment:d0,lights_physical_fragment:f0,lights_physical_pars_fragment:p0,lights_fragment_begin:m0,lights_fragment_maps:g0,lights_fragment_end:b0,lightprobes_pars_fragment:x0,logdepthbuf_fragment:v0,logdepthbuf_pars_fragment:_0,logdepthbuf_pars_vertex:y0,logdepthbuf_vertex:M0,map_fragment:S0,map_pars_fragment:w0,map_particle_fragment:E0,map_particle_pars_fragment:T0,metalnessmap_fragment:A0,metalnessmap_pars_fragment:R0,morphinstance_vertex:C0,morphcolor_vertex:P0,morphnormal_vertex:I0,morphtarget_pars_vertex:D0,morphtarget_vertex:L0,normal_fragment_begin:F0,normal_fragment_maps:N0,normal_pars_fragment:U0,normal_pars_vertex:O0,normal_vertex:z0,normalmap_pars_fragment:B0,clearcoat_normal_fragment_begin:k0,clearcoat_normal_fragment_maps:H0,clearcoat_pars_fragment:G0,iridescence_pars_fragment:V0,opaque_fragment:W0,packing:q0,premultiplied_alpha_fragment:X0,project_vertex:j0,dithering_fragment:K0,dithering_pars_fragment:Y0,roughnessmap_fragment:J0,roughnessmap_pars_fragment:Z0,shadowmap_pars_fragment:$0,shadowmap_pars_vertex:Q0,shadowmap_vertex:eb,shadowmask_pars_fragment:tb,skinbase_vertex:nb,skinning_pars_vertex:ib,skinning_vertex:sb,skinnormal_vertex:rb,specularmap_fragment:ab,specularmap_pars_fragment:ob,tonemapping_fragment:lb,tonemapping_pars_fragment:cb,transmission_fragment:hb,transmission_pars_fragment:ub,uv_pars_fragment:db,uv_pars_vertex:fb,uv_vertex:pb,worldpos_vertex:mb,background_vert:gb,background_frag:bb,backgroundCube_vert:xb,backgroundCube_frag:vb,cube_vert:_b,cube_frag:yb,depth_vert:Mb,depth_frag:Sb,distance_vert:wb,distance_frag:Eb,equirect_vert:Tb,equirect_frag:Ab,linedashed_vert:Rb,linedashed_frag:Cb,meshbasic_vert:Pb,meshbasic_frag:Ib,meshlambert_vert:Db,meshlambert_frag:Lb,meshmatcap_vert:Fb,meshmatcap_frag:Nb,meshnormal_vert:Ub,meshnormal_frag:Ob,meshphong_vert:zb,meshphong_frag:Bb,meshphysical_vert:kb,meshphysical_frag:Hb,meshtoon_vert:Gb,meshtoon_frag:Vb,points_vert:Wb,points_frag:qb,shadow_vert:Xb,shadow_frag:jb,sprite_vert:Kb,sprite_frag:Yb},Je={common:{diffuse:{value:new Le(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new mt},alphaMap:{value:null},alphaMapTransform:{value:new mt},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new mt}},envmap:{envMap:{value:null},envMapRotation:{value:new mt},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98},dfgLUT:{value:null}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new mt}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new mt}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new mt},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new mt},normalScale:{value:new Be(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new mt},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new mt}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new mt}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new mt}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new Le(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},sunLights:{value:[],properties:{direction:{},color:{}}},sunLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},sunShadowMatrix:{value:[]},sunShadowCascade:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null},probesSH:{value:null},probesMin:{value:new k},probesMax:{value:new k},probesResolution:{value:new k}},points:{diffuse:{value:new Le(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new mt},alphaTest:{value:0},uvTransform:{value:new mt}},sprite:{diffuse:{value:new Le(16777215)},opacity:{value:1},center:{value:new Be(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new mt},alphaMap:{value:null},alphaMapTransform:{value:new mt},alphaTest:{value:0}}},wi={basic:{uniforms:vn([Je.common,Je.specularmap,Je.envmap,Je.aomap,Je.lightmap,Je.fog]),vertexShader:St.meshbasic_vert,fragmentShader:St.meshbasic_frag},lambert:{uniforms:vn([Je.common,Je.specularmap,Je.envmap,Je.aomap,Je.lightmap,Je.emissivemap,Je.bumpmap,Je.normalmap,Je.displacementmap,Je.fog,Je.lights,{emissive:{value:new Le(0)},envMapIntensity:{value:1}}]),vertexShader:St.meshlambert_vert,fragmentShader:St.meshlambert_frag},phong:{uniforms:vn([Je.common,Je.specularmap,Je.envmap,Je.aomap,Je.lightmap,Je.emissivemap,Je.bumpmap,Je.normalmap,Je.displacementmap,Je.fog,Je.lights,{emissive:{value:new Le(0)},specular:{value:new Le(1118481)},shininess:{value:30},envMapIntensity:{value:1}}]),vertexShader:St.meshphong_vert,fragmentShader:St.meshphong_frag},standard:{uniforms:vn([Je.common,Je.envmap,Je.aomap,Je.lightmap,Je.emissivemap,Je.bumpmap,Je.normalmap,Je.displacementmap,Je.roughnessmap,Je.metalnessmap,Je.fog,Je.lights,{emissive:{value:new Le(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:St.meshphysical_vert,fragmentShader:St.meshphysical_frag},toon:{uniforms:vn([Je.common,Je.aomap,Je.lightmap,Je.emissivemap,Je.bumpmap,Je.normalmap,Je.displacementmap,Je.gradientmap,Je.fog,Je.lights,{emissive:{value:new Le(0)}}]),vertexShader:St.meshtoon_vert,fragmentShader:St.meshtoon_frag},matcap:{uniforms:vn([Je.common,Je.bumpmap,Je.normalmap,Je.displacementmap,Je.fog,{matcap:{value:null}}]),vertexShader:St.meshmatcap_vert,fragmentShader:St.meshmatcap_frag},points:{uniforms:vn([Je.points,Je.fog]),vertexShader:St.points_vert,fragmentShader:St.points_frag},dashed:{uniforms:vn([Je.common,Je.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:St.linedashed_vert,fragmentShader:St.linedashed_frag},depth:{uniforms:vn([Je.common,Je.displacementmap]),vertexShader:St.depth_vert,fragmentShader:St.depth_frag},normal:{uniforms:vn([Je.common,Je.bumpmap,Je.normalmap,Je.displacementmap,{opacity:{value:1}}]),vertexShader:St.meshnormal_vert,fragmentShader:St.meshnormal_frag},sprite:{uniforms:vn([Je.sprite,Je.fog]),vertexShader:St.sprite_vert,fragmentShader:St.sprite_frag},background:{uniforms:{uvTransform:{value:new mt},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:St.background_vert,fragmentShader:St.background_frag},backgroundCube:{uniforms:{envMap:{value:null},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new mt}},vertexShader:St.backgroundCube_vert,fragmentShader:St.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:St.cube_vert,fragmentShader:St.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:St.equirect_vert,fragmentShader:St.equirect_frag},distance:{uniforms:vn([Je.common,Je.displacementmap,{referencePosition:{value:new k},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:St.distance_vert,fragmentShader:St.distance_frag},shadow:{uniforms:vn([Je.lights,Je.fog,{color:{value:new Le(0)},opacity:{value:1}}]),vertexShader:St.shadow_vert,fragmentShader:St.shadow_frag}};wi.physical={uniforms:vn([wi.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new mt},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new mt},clearcoatNormalScale:{value:new Be(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new mt},dispersion:{value:0},retroreflectivity:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new mt},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new mt},sheen:{value:0},sheenColor:{value:new Le(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new mt},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new mt},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new mt},transmissionSamplerSize:{value:new Be},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new mt},attenuationDistance:{value:0},attenuationColor:{value:new Le(0)},specularColor:{value:new Le(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new mt},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new mt},anisotropyVector:{value:new Be},anisotropyMap:{value:null},anisotropyMapTransform:{value:new mt}}]),vertexShader:St.meshphysical_vert,fragmentShader:St.meshphysical_frag};var lc={r:0,b:0,g:0},Jb=new pt,qf=new mt;qf.set(-1,0,0,0,1,0,0,0,1);function Zb(s,e,t,n,i,r){let a=new Le(0),o=i===!0?0:1,l,c,h=null,u=0,d=null;function p(v){let S=v.isScene===!0?v.background:null;if(S&&S.isTexture){let b=v.backgroundBlurriness>0;S=e.get(S,b)}return S}function g(v){let S=!1,b=p(v);b===null?m(a,o):b&&b.isColor&&(m(b,1),S=!0);let M=s.xr.getEnvironmentBlendMode();M==="additive"?t.buffers.color.setClear(0,0,0,1,r):M==="alpha-blend"&&t.buffers.color.setClear(0,0,0,0,r),(s.autoClear||S)&&(t.buffers.depth.setTest(!0),t.buffers.depth.setMask(!0),t.buffers.color.setMask(!0),s.clear(s.autoClearColor,s.autoClearDepth,s.autoClearStencil))}function _(v,S){let b=p(S);b&&(b.isCubeTexture||b.mapping===Ga)?(c===void 0&&(c=new it(new xi(1,1,1),new vt({name:"BackgroundCubeMaterial",uniforms:ks(wi.backgroundCube.uniforms),vertexShader:wi.backgroundCube.vertexShader,fragmentShader:wi.backgroundCube.fragmentShader,side:on,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),c.geometry.deleteAttribute("normal"),c.geometry.deleteAttribute("uv"),c.onBeforeRender=function(M,E,P){this.matrixWorld.copyPosition(P.matrixWorld)},Object.defineProperty(c.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),n.update(c)),c.material.uniforms.envMap.value=b,c.material.uniforms.backgroundBlurriness.value=S.backgroundBlurriness,c.material.uniforms.backgroundIntensity.value=S.backgroundIntensity,c.material.uniforms.backgroundRotation.value.setFromMatrix4(Jb.makeRotationFromEuler(S.backgroundRotation)).transpose(),b.isCubeTexture&&b.isRenderTargetTexture===!1&&c.material.uniforms.backgroundRotation.value.premultiply(qf),c.material.toneMapped=_t.getTransfer(b.colorSpace)!==Dt,(h!==b||u!==b.version||d!==s.toneMapping)&&(c.material.needsUpdate=!0,h=b,u=b.version,d=s.toneMapping),c.layers.enableAll(),v.unshift(c,c.geometry,c.material,0,0,null)):b&&b.isTexture&&(l===void 0&&(l=new it(new pn(2,2),new vt({name:"BackgroundMaterial",uniforms:ks(wi.background.uniforms),vertexShader:wi.background.vertexShader,fragmentShader:wi.background.fragmentShader,side:jn,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),l.geometry.deleteAttribute("normal"),Object.defineProperty(l.material,"map",{get:function(){return this.uniforms.t2D.value}}),n.update(l)),l.material.uniforms.t2D.value=b,l.material.uniforms.backgroundIntensity.value=S.backgroundIntensity,l.material.toneMapped=_t.getTransfer(b.colorSpace)!==Dt,b.matrixAutoUpdate===!0&&b.updateMatrix(),l.material.uniforms.uvTransform.value.copy(b.matrix),(h!==b||u!==b.version||d!==s.toneMapping)&&(l.material.needsUpdate=!0,h=b,u=b.version,d=s.toneMapping),l.layers.enableAll(),v.unshift(l,l.geometry,l.material,0,0,null))}function m(v,S){v.getRGB(lc,Dh(s)),t.buffers.color.setClear(lc.r,lc.g,lc.b,S,r)}function f(){c!==void 0&&(c.geometry.dispose(),c.material.dispose(),c=void 0),l!==void 0&&(l.geometry.dispose(),l.material.dispose(),l=void 0)}return{getClearColor:function(){return a},setClearColor:function(v,S=1){a.set(v),o=S,m(a,o)},getClearAlpha:function(){return o},setClearAlpha:function(v){o=v,m(a,o)},render:g,addToRenderList:_,dispose:f}}function $b(s,e){let t=s.getParameter(s.MAX_VERTEX_ATTRIBS),n={},i=d(null),r=i,a=!1;function o(I,y,N,D,R){let U=!1,z=u(I,D,N,y);r!==z&&(r=z,c(r.object)),U=p(I,D,N,R),U&&g(I,D,N,R),R!==null&&e.update(R,s.ELEMENT_ARRAY_BUFFER),(U||a)&&(a=!1,b(I,y,N,D),R!==null&&s.bindBuffer(s.ELEMENT_ARRAY_BUFFER,e.get(R).buffer))}function l(){return s.createVertexArray()}function c(I){return s.bindVertexArray(I)}function h(I){return s.deleteVertexArray(I)}function u(I,y,N,D){let R=D.wireframe===!0,U=n[y.id];U===void 0&&(U={},n[y.id]=U);let z=I.isInstancedMesh===!0?I.id:0,T=U[z];T===void 0&&(T={},U[z]=T);let G=T[N.id];G===void 0&&(G={},T[N.id]=G);let X=G[R];return X===void 0&&(X=d(l()),G[R]=X),X}function d(I){let y=[],N=[],D=[];for(let R=0;R<t;R++)y[R]=0,N[R]=0,D[R]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:y,enabledAttributes:N,attributeDivisors:D,object:I,attributes:{},index:null}}function p(I,y,N,D){let R=r.attributes,U=y.attributes,z=0,T=N.getAttributes();for(let G in T)if(T[G].location>=0){let Z=R[G],H=U[G];if(H===void 0&&(G==="instanceMatrix"&&I.instanceMatrix&&(H=I.instanceMatrix),G==="instanceColor"&&I.instanceColor&&(H=I.instanceColor)),Z===void 0||Z.attribute!==H||H&&Z.data!==H.data)return!0;z++}return r.attributesNum!==z||r.index!==D}function g(I,y,N,D){let R={},U=y.attributes,z=0,T=N.getAttributes();for(let G in T)if(T[G].location>=0){let Z=U[G];Z===void 0&&(G==="instanceMatrix"&&I.instanceMatrix&&(Z=I.instanceMatrix),G==="instanceColor"&&I.instanceColor&&(Z=I.instanceColor));let H={};H.attribute=Z,Z&&Z.data&&(H.data=Z.data),R[G]=H,z++}r.attributes=R,r.attributesNum=z,r.index=D}function _(){let I=r.newAttributes;for(let y=0,N=I.length;y<N;y++)I[y]=0}function m(I){f(I,0)}function f(I,y){let N=r.newAttributes,D=r.enabledAttributes,R=r.attributeDivisors;N[I]=1,D[I]===0&&(s.enableVertexAttribArray(I),D[I]=1),R[I]!==y&&(s.vertexAttribDivisor(I,y),R[I]=y)}function v(){let I=r.newAttributes,y=r.enabledAttributes;for(let N=0,D=y.length;N<D;N++)y[N]!==I[N]&&(s.disableVertexAttribArray(N),y[N]=0)}function S(I,y,N,D,R,U,z){z===!0?s.vertexAttribIPointer(I,y,N,R,U):s.vertexAttribPointer(I,y,N,D,R,U)}function b(I,y,N,D){_();let R=D.attributes,U=N.getAttributes(),z=y.defaultAttributeValues;for(let T in U){let G=U[T];if(G.location>=0){let X=R[T];if(X===void 0&&(T==="instanceMatrix"&&I.instanceMatrix&&(X=I.instanceMatrix),T==="instanceColor"&&I.instanceColor&&(X=I.instanceColor)),X!==void 0){let Z=X.normalized,H=X.itemSize,oe=e.get(X);if(oe===void 0)continue;let ke=oe.buffer,Fe=oe.type,We=oe.bytesPerElement,fe=Fe===s.INT||Fe===s.UNSIGNED_INT||X.gpuType===yl;if(X.isInterleavedBufferAttribute){let re=X.data,Pe=re.stride,be=X.offset;if(re.isInstancedInterleavedBuffer){for(let ge=0;ge<G.locationSize;ge++)f(G.location+ge,re.meshPerAttribute);I.isInstancedMesh!==!0&&D._maxInstanceCount===void 0&&(D._maxInstanceCount=re.meshPerAttribute*re.count)}else for(let ge=0;ge<G.locationSize;ge++)m(G.location+ge);s.bindBuffer(s.ARRAY_BUFFER,ke);for(let ge=0;ge<G.locationSize;ge++)S(G.location+ge,H/G.locationSize,Fe,Z,Pe*We,(be+H/G.locationSize*ge)*We,fe)}else{if(X.isInstancedBufferAttribute){for(let re=0;re<G.locationSize;re++)f(G.location+re,X.meshPerAttribute);I.isInstancedMesh!==!0&&D._maxInstanceCount===void 0&&(D._maxInstanceCount=X.meshPerAttribute*X.count)}else for(let re=0;re<G.locationSize;re++)m(G.location+re);s.bindBuffer(s.ARRAY_BUFFER,ke);for(let re=0;re<G.locationSize;re++)S(G.location+re,H/G.locationSize,Fe,Z,H*We,H/G.locationSize*re*We,fe)}}else if(z!==void 0){let Z=z[T];if(Z!==void 0)switch(Z.length){case 2:s.vertexAttrib2fv(G.location,Z);break;case 3:s.vertexAttrib3fv(G.location,Z);break;case 4:s.vertexAttrib4fv(G.location,Z);break;default:s.vertexAttrib1fv(G.location,Z)}}}}v()}function M(){C();for(let I in n){let y=n[I];for(let N in y){let D=y[N];for(let R in D){let U=D[R];for(let z in U)h(U[z].object),delete U[z];delete D[R]}}delete n[I]}}function E(I){if(n[I.id]===void 0)return;let y=n[I.id];for(let N in y){let D=y[N];for(let R in D){let U=D[R];for(let z in U)h(U[z].object),delete U[z];delete D[R]}}delete n[I.id]}function P(I){for(let y in n){let N=n[y];for(let D in N){let R=N[D];if(R[I.id]===void 0)continue;let U=R[I.id];for(let z in U)h(U[z].object),delete U[z];delete R[I.id]}}}function x(I){for(let y in n){let N=n[y],D=I.isInstancedMesh===!0?I.id:0,R=N[D];if(R!==void 0){for(let U in R){let z=R[U];for(let T in z)h(z[T].object),delete z[T];delete R[U]}delete N[D],Object.keys(N).length===0&&delete n[y]}}}function C(){w(),a=!0,r!==i&&(r=i,c(r.object))}function w(){i.geometry=null,i.program=null,i.wireframe=!1}return{setup:o,reset:C,resetDefaultState:w,dispose:M,releaseStatesOfGeometry:E,releaseStatesOfObject:x,releaseStatesOfProgram:P,initAttributes:_,enableAttribute:m,disableUnusedAttributes:v}}function Qb(s,e,t){let n;function i(l){n=l}function r(l,c){s.drawArrays(n,l,c),t.update(c,n,1)}function a(l,c,h){h!==0&&(s.drawArraysInstanced(n,l,c,h),t.update(c,n,h))}function o(l,c,h){if(h===0)return;e.get("WEBGL_multi_draw").multiDrawArraysWEBGL(n,l,0,c,0,h);let d=0;for(let p=0;p<h;p++)d+=c[p];t.update(d,n,1)}this.setMode=i,this.render=r,this.renderInstances=a,this.renderMultiDraw=o}function ex(s,e,t,n){let i;function r(){if(i!==void 0)return i;if(e.has("EXT_texture_filter_anisotropic")===!0){let P=e.get("EXT_texture_filter_anisotropic");i=s.getParameter(P.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else i=0;return i}function a(P){return!(P!==Hn&&n.convert(P)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_FORMAT))}function o(P){let x=P===rn&&(e.has("EXT_color_buffer_half_float")||e.has("EXT_color_buffer_float"));return!(P!==In&&P!==kn&&!x&&n.convert(P)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_TYPE))}function l(P){if(P==="highp"){if(s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.HIGH_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.HIGH_FLOAT).precision>0)return"highp";P="mediump"}return P==="mediump"&&s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.MEDIUM_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let c=t.precision!==void 0?t.precision:"highp",h=l(c);h!==c&&(nt("WebGLRenderer:",c,"not supported, using",h,"instead."),c=h);let u=t.logarithmicDepthBuffer===!0,d=t.reversedDepthBuffer===!0&&e.has("EXT_clip_control");t.reversedDepthBuffer===!0&&d===!1&&nt("WebGLRenderer: Unable to use reversed depth buffer due to missing EXT_clip_control extension. Fallback to default depth buffer.");let p=s.getParameter(s.MAX_TEXTURE_IMAGE_UNITS),g=s.getParameter(s.MAX_VERTEX_TEXTURE_IMAGE_UNITS),_=s.getParameter(s.MAX_TEXTURE_SIZE),m=s.getParameter(s.MAX_CUBE_MAP_TEXTURE_SIZE),f=s.getParameter(s.MAX_VERTEX_ATTRIBS),v=s.getParameter(s.MAX_VERTEX_UNIFORM_VECTORS),S=s.getParameter(s.MAX_VARYING_VECTORS),b=s.getParameter(s.MAX_FRAGMENT_UNIFORM_VECTORS),M=s.getParameter(s.MAX_SAMPLES),E=s.getParameter(s.SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:l,textureFormatReadable:a,textureTypeReadable:o,precision:c,logarithmicDepthBuffer:u,reversedDepthBuffer:d,maxTextures:p,maxVertexTextures:g,maxTextureSize:_,maxCubemapSize:m,maxAttributes:f,maxVertexUniforms:v,maxVaryings:S,maxFragmentUniforms:b,maxSamples:M,samples:E}}function tx(s){let e=this,t=null,n=0,i=!1,r=!1,a=new On,o=new mt,l={value:null,needsUpdate:!1};this.uniform=l,this.numPlanes=0,this.numIntersection=0,this.init=function(u,d){let p=u.length!==0||d||n!==0||i;return i=d,n=u.length,p},this.beginShadows=function(){r=!0,h(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(u,d){t=h(u,d,0)},this.setState=function(u,d,p){let g=u.clippingPlanes,_=u.clipIntersection,m=u.clipShadows,f=s.get(u);if(!i||g===null||g.length===0||r&&!m)r?h(null):c();else{let v=r?0:n,S=v*4,b=f.clippingState||null;l.value=b,b=h(g,d,S,p);for(let M=0;M!==S;++M)b[M]=t[M];f.clippingState=b,this.numIntersection=_?this.numPlanes:0,this.numPlanes+=v}};function c(){l.value!==t&&(l.value=t,l.needsUpdate=n>0),e.numPlanes=n,e.numIntersection=0}function h(u,d,p,g){let _=u!==null?u.length:0,m=null;if(_!==0){if(m=l.value,g!==!0||m===null){let f=p+_*4,v=d.matrixWorldInverse;o.getNormalMatrix(v),(m===null||m.length<f)&&(m=new Float32Array(f));for(let S=0,b=p;S!==_;++S,b+=4)a.copy(u[S]).applyMatrix4(v,o),a.normal.toArray(m,b),m[b+3]=a.constant}l.value=m,l.needsUpdate=!0}return e.numPlanes=_,e.numIntersection=0,m}}var zr=4,nx=6,ix=20,sx=256,Za=new Mi,Sf=new Le,Uh=null,Oh=0,zh=0,Bh=!1,rx=new k,Hs=new k,kr=class{constructor(e){this._renderer=e,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._sizeLods=[],this._lodMeshes=[],this._backgroundBox=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._blurMaterial=null,this._ggxMaterial=null}fromScene(e,t=0,n=.1,i=100,r={}){let{size:a=256,position:o=rx}=r;Uh=this._renderer.getRenderTarget(),Oh=this._renderer.getActiveCubeFace(),zh=this._renderer.getActiveMipmapLevel(),Bh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(a);let l=this._allocateTargets();return l.depthBuffer=!0,this._sceneToCubeUV(e,n,i,l,o),t>0&&this._blur(l,0,0,t),this._applyPMREM(l),this._cleanup(l),l}fromEquirectangular(e,t=null){return this._fromTexture(e,t)}fromCubemap(e,t=null){return this._fromTexture(e,t)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=Tf(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=Ef(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose(),this._backgroundBox!==null&&(this._backgroundBox.geometry.dispose(),this._backgroundBox.material.dispose())}_setSize(e){this._lodMax=Math.floor(Math.log2(e)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._ggxMaterial!==null&&this._ggxMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let e=0;e<this._lodMeshes.length;e++)this._lodMeshes[e].geometry.dispose()}_cleanup(e){this._renderer.setRenderTarget(Uh,Oh,zh),this._renderer.xr.enabled=Bh,e.scissorTest=!1,Or(e,0,0,e.width,e.height)}_fromTexture(e,t){e.mapping===us||e.mapping===zs?this._setSize(e.image.length===0?16:e.image[0].width||e.image[0].image.width):this._setSize(e.image.width/4),Uh=this._renderer.getRenderTarget(),Oh=this._renderer.getActiveCubeFace(),zh=this._renderer.getActiveMipmapLevel(),Bh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let n=t||this._allocateTargets();return this._textureToCubeUV(e,n),this._applyPMREM(n),this._cleanup(n),n}_allocateTargets(){let e=3*Math.max(this._cubeSize,112),t=4*this._cubeSize,n={magFilter:jt,minFilter:jt,generateMipmaps:!1,type:rn,format:Hn,colorSpace:wn,depthBuffer:!1},i=wf(e,t,n);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==e||this._pingPongRenderTarget.height!==t){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=wf(e,t,n);let{_lodMax:r}=this;({lodMeshes:this._lodMeshes,sizeLods:this._sizeLods}=ax(r)),this._blurMaterial=lx(r,e,t),this._ggxMaterial=ox(r,e,t)}return i}_compileMaterial(e){let t=new it(new bt,e);this._renderer.compile(t,Za)}_sceneToCubeUV(e,t,n,i,r){let l=new sn(90,1,t,n),c=[1,-1,1,1,1,1],h=[1,1,1,-1,-1,-1],u=this._renderer,d=u.autoClear,p=u.toneMapping;u.getClearColor(Sf),u.toneMapping=ai,u.autoClear=!1,u.state.buffers.depth.getReversed()&&(u.setRenderTarget(i),u.clearDepth(),u.setRenderTarget(null)),this._backgroundBox===null&&(this._backgroundBox=new it(new xi,new Lt({name:"PMREM.Background",side:on,depthWrite:!1,depthTest:!1})));let _=this._backgroundBox,m=_.material,f=!1,v=e.background;v?v.isColor&&(m.color.copy(v),e.background=null,f=!0):(m.color.copy(Sf),f=!0);for(let S=0;S<6;S++){let b=S%3;b===0?(l.up.set(0,c[S],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x+h[S],r.y,r.z)):b===1?(l.up.set(0,0,c[S]),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y+h[S],r.z)):(l.up.set(0,c[S],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y,r.z+h[S]));let M=this._cubeSize;Or(i,b*M,S>2?M:0,M,M),u.setRenderTarget(i),f&&u.render(_,l),u.render(e,l)}u.toneMapping=p,u.autoClear=d,e.background=v}_textureToCubeUV(e,t){let n=this._renderer,i=e.mapping===us||e.mapping===zs;i?(this._cubemapMaterial===null&&(this._cubemapMaterial=Tf()),this._cubemapMaterial.uniforms.flipEnvMap.value=e.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=Ef());let r=i?this._cubemapMaterial:this._equirectMaterial,a=this._lodMeshes[0];a.material=r;let o=r.uniforms;o.envMap.value=e;let l=this._cubeSize;Or(t,0,0,3*l,2*l),n.setRenderTarget(t),n.render(a,Za)}_applyPMREM(e){let t=this._renderer,n=t.autoClear;t.autoClear=!1;let i=this._lodMeshes.length;for(let r=1;r<i;r++)this._applyGGXFilter(e,r-1,r);t.autoClear=n}_applyGGXFilter(e,t,n){let i=this._renderer,r=this._pingPongRenderTarget,a=this._ggxMaterial,o=this._lodMeshes[n];o.material=a;let l=a.uniforms,c=n/(this._lodMeshes.length-1),h=t/(this._lodMeshes.length-1),u=Math.sqrt(c*c-h*h),d=c*1.25,p=u*d,{_lodMax:g}=this,_=this._sizeLods[n],m=3*_*(n>g-zr?n-g+zr:0),f=4*(this._cubeSize-_);l.envMap.value=e.texture,l.roughness.value=p,l.mipInt.value=g-t,Or(r,m,f,3*_,2*_),i.setRenderTarget(r),i.render(o,Za),l.envMap.value=r.texture,l.roughness.value=0,l.mipInt.value=g-n,Or(e,m,f,3*_,2*_),i.setRenderTarget(e),i.render(o,Za)}_blur(e,t,n,i){let r=this._pingPongRenderTarget,a=Math.min(i,Math.PI)/Math.SQRT2;this._blurPass(e,r,t,n,a),this._blurPass(r,e,n,n,a)}_blurPass(e,t,n,i,r){let a=this._renderer,o=this._blurMaterial,l=this._lodMeshes[i];l.material=o;let c=o.uniforms;c.envMap.value=e.texture,c.sigma.value=r,c.mipInt.value=this._lodMax-n;let h=this._sizeLods[i],u=3*h*(i>this._lodMax-zr?i-this._lodMax+zr:0),d=4*(this._cubeSize-h);Or(t,u,d,3*h,2*h),a.setRenderTarget(t),a.render(l,Za)}};function ax(s){let e=[],t=[],n=s,i=s-zr+1+nx;for(let r=0;r<i;r++){let a=Math.pow(2,n);e.push(a);let o=1/(a-2),l=-o,c=1+o,h=[l,l,c,l,c,c,l,l,c,c,l,c],u=6,d=6,p=3,g=new Float32Array(p*d*u),_=new Float32Array(p*d*u);for(let f=0;f<u;f++){let v=f%3*2/3-1,S=f>2?0:-1,b=[v,S,0,v+2/3,S,0,v+2/3,S+1,0,v,S,0,v+2/3,S+1,0,v,S+1,0];g.set(b,p*d*f);for(let M=0;M<d;M++){let E=h[M*2]*2-1,P=h[M*2+1]*2-1;f===0?Hs.set(1,P,E):f===1?Hs.set(-E,1,-P):f===2?Hs.set(-E,P,1):f===3?Hs.set(-1,P,-E):f===4?Hs.set(-E,-1,P):Hs.set(E,P,-1),Hs.toArray(_,(f*d+M)*p)}}let m=new bt;m.setAttribute("position",new Mt(g,p)),m.setAttribute("outputDirection",new Mt(_,p)),t.push(new it(m,null)),n>zr&&n--}return{lodMeshes:t,sizeLods:e}}function wf(s,e,t){let n=new Kt(s,e,t);return n.texture.mapping=Ga,n.texture.name="PMREM.cubeUv",n.scissorTest=!0,n}function Or(s,e,t,n,i){s.viewport.set(e,t,n,i),s.scissor.set(e,t,n,i)}function ox(s,e,t){return new vt({name:"PMREMGGXConvolution",defines:{GGX_SAMPLES:sx,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},roughness:{value:0},mipInt:{value:0}},vertexShader:dc(),fragmentShader:`

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
		`,blending:Kn,depthTest:!1,depthWrite:!1})}function lx(s,e,t){return new vt({name:"SphericalGaussianBlur",defines:{SAMPLES:ix,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},sigma:{value:0},mipInt:{value:0}},vertexShader:dc(),fragmentShader:`

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
		`,blending:Kn,depthTest:!1,depthWrite:!1})}function Ef(){return new vt({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:dc(),fragmentShader:`

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
		`,blending:Kn,depthTest:!1,depthWrite:!1})}function Tf(){return new vt({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:dc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:Kn,depthTest:!1,depthWrite:!1})}function dc(){return`

		precision mediump float;
		precision mediump int;

		attribute vec3 outputDirection;

		varying vec3 vOutputDirection;

		void main() {

			vOutputDirection = outputDirection;
			gl_Position = vec4( position, 1.0 );

		}
	`}var hc=class extends Kt{constructor(e=1,t={}){super(e,e,t),this.isWebGLCubeRenderTarget=!0;let n={width:e,height:e,depth:1},i=[n,n,n,n,n,n];this.texture=new xa(i),this._setTextureOptions(t),this.texture.isRenderTargetTexture=!0}fromEquirectangularTexture(e,t){this.texture.type=t.type,this.texture.colorSpace=t.colorSpace,this.texture.generateMipmaps=t.generateMipmaps,this.texture.minFilter=t.minFilter,this.texture.magFilter=t.magFilter;let n={uniforms:{tEquirect:{value:null}},vertexShader:`

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
			`},i=new xi(5,5,5),r=new vt({name:"CubemapFromEquirect",uniforms:ks(n.uniforms),vertexShader:n.vertexShader,fragmentShader:n.fragmentShader,side:on,blending:Kn});r.uniforms.tEquirect.value=t;let a=new it(i,r),o=t.minFilter;return t.minFilter===oi&&(t.minFilter=jt),new fl(1,10,this).update(e,a),t.minFilter=o,a.geometry.dispose(),a.material.dispose(),this}clear(e,t=!0,n=!0,i=!0){let r=e.getRenderTarget();for(let a=0;a<6;a++)e.setRenderTarget(this,a),e.clear(t,n,i);e.setRenderTarget(r)}};function cx(s){let e=new WeakMap,t=new WeakMap,n=null;function i(d,p=!1){return d==null?null:p?a(d):r(d)}function r(d){if(d&&d.isTexture){let p=d.mapping;if(p===xl||p===vl)if(e.has(d)){let g=e.get(d).texture;return o(g,d.mapping)}else{let g=d.image;if(g&&g.height>0){let _=new hc(g.height);return _.fromEquirectangularTexture(s,d),e.set(d,_),d.addEventListener("dispose",c),o(_.texture,d.mapping)}else return null}}return d}function a(d){if(d&&d.isTexture){let p=d.mapping,g=p===xl||p===vl,_=p===us||p===zs;if(g||_){let m=t.get(d),f=m!==void 0?m.texture.pmremVersion:0;if(d.isRenderTargetTexture&&d.pmremVersion!==f)return n===null&&(n=new kr(s)),m=g?n.fromEquirectangular(d,m):n.fromCubemap(d,m),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),m.texture;if(m!==void 0)return m.texture;{let v=d.image;return g&&v&&v.height>0||_&&v&&l(v)?(n===null&&(n=new kr(s)),m=g?n.fromEquirectangular(d):n.fromCubemap(d),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),d.addEventListener("dispose",h),m.texture):null}}}return d}function o(d,p){return p===xl?d.mapping=us:p===vl&&(d.mapping=zs),d}function l(d){let p=0,g=6;for(let _=0;_<g;_++)d[_]!==void 0&&p++;return p===g}function c(d){let p=d.target;p.removeEventListener("dispose",c);let g=e.get(p);g!==void 0&&(e.delete(p),g.dispose())}function h(d){let p=d.target;p.removeEventListener("dispose",h);let g=t.get(p);g!==void 0&&(t.delete(p),g.dispose())}function u(){e=new WeakMap,t=new WeakMap,n!==null&&(n.dispose(),n=null)}return{get:i,dispose:u}}function hx(s){let e={};function t(n){if(e[n]!==void 0)return e[n];let i=s.getExtension(n);return e[n]=i,i}return{has:function(n){return t(n)!==null},init:function(){t("EXT_color_buffer_float"),t("WEBGL_clip_cull_distance"),t("OES_texture_float_linear"),t("EXT_color_buffer_half_float"),t("WEBGL_multisampled_render_to_texture"),t("WEBGL_render_shared_exponent")},get:function(n){let i=t(n);return i===null&&Es("WebGLRenderer: "+n+" extension not supported."),i}}}function ux(s,e,t,n){let i={},r=new WeakMap;function a(u){let d=u.target;d.index!==null&&e.remove(d.index);for(let g in d.attributes)e.remove(d.attributes[g]);d.removeEventListener("dispose",a),delete i[d.id];let p=r.get(d);p&&(e.remove(p),r.delete(d)),n.releaseStatesOfGeometry(d),d.isInstancedBufferGeometry===!0&&delete d._maxInstanceCount,t.memory.geometries--}function o(u,d){return i[d.id]===!0||(d.addEventListener("dispose",a),i[d.id]=!0,t.memory.geometries++),d}function l(u){let d=u.attributes;for(let p in d)e.update(d[p],s.ARRAY_BUFFER)}function c(u){let d=[],p=u.index,g=u.attributes.position,_=0;if(g===void 0)return;if(p!==null){let v=p.array;_=p.version;for(let S=0,b=v.length;S<b;S+=3){let M=v[S+0],E=v[S+1],P=v[S+2];d.push(M,E,E,P,P,M)}}else{let v=g.array;_=g.version;for(let S=0,b=v.length/3-1;S<b;S+=3){let M=S+0,E=S+1,P=S+2;d.push(M,E,E,P,P,M)}}let m=new(g.count>=65535?pa:fa)(d,1);m.version=_;let f=r.get(u);f&&e.remove(f),r.set(u,m)}function h(u){let d=r.get(u);if(d){let p=u.index;p!==null&&d.version<p.version&&c(u)}else c(u);return r.get(u)}return{get:o,update:l,getWireframeAttribute:h}}function dx(s,e,t){let n;function i(u){n=u}let r,a;function o(u){r=u.type,a=u.bytesPerElement}function l(u,d){s.drawElements(n,d,r,u*a),t.update(d,n,1)}function c(u,d,p){p!==0&&(s.drawElementsInstanced(n,d,r,u*a,p),t.update(d,n,p))}function h(u,d,p){if(p===0)return;e.get("WEBGL_multi_draw").multiDrawElementsWEBGL(n,d,0,r,u,0,p);let _=0;for(let m=0;m<p;m++)_+=d[m];t.update(_,n,1)}this.setMode=i,this.setIndex=o,this.render=l,this.renderInstances=c,this.renderMultiDraw=h}function fx(s){let e={geometries:0,textures:0},t={frame:0,calls:0,triangles:0,points:0,lines:0};function n(r,a,o){switch(t.calls++,a){case s.TRIANGLES:t.triangles+=o*(r/3);break;case s.LINES:t.lines+=o*(r/2);break;case s.LINE_STRIP:t.lines+=o*(r-1);break;case s.LINE_LOOP:t.lines+=o*r;break;case s.POINTS:t.points+=o*r;break;default:ht("WebGLInfo: Unknown draw mode:",a);break}}function i(){t.calls=0,t.triangles=0,t.points=0,t.lines=0}return{memory:e,render:t,programs:null,autoReset:!0,reset:i,update:n}}function px(s,e,t){let n=new WeakMap,i=new Pt;function r(a,o,l){let c=a.morphTargetInfluences,h=o.morphAttributes.position||o.morphAttributes.normal||o.morphAttributes.color,u=h!==void 0?h.length:0,d=n.get(o);if(d===void 0||d.count!==u){let C=function(){P.dispose(),n.delete(o),o.removeEventListener("dispose",C)};d!==void 0&&d.texture.dispose();let p=o.morphAttributes.position!==void 0,g=o.morphAttributes.normal!==void 0,_=o.morphAttributes.color!==void 0,m=o.morphAttributes.position||[],f=o.morphAttributes.normal||[],v=o.morphAttributes.color||[],S=0;p===!0&&(S=1),g===!0&&(S=2),_===!0&&(S=3);let b=o.attributes.position.count*S,M=1;b>e.maxTextureSize&&(M=Math.ceil(b/e.maxTextureSize),b=e.maxTextureSize);let E=new Float32Array(b*M*4*u),P=new ua(E,b,M,u);P.type=kn,P.needsUpdate=!0;let x=S*4;for(let w=0;w<u;w++){let I=m[w],y=f[w],N=v[w],D=b*M*4*w;for(let R=0;R<I.count;R++){let U=R*x;p===!0&&(i.fromBufferAttribute(I,R),E[D+U+0]=i.x,E[D+U+1]=i.y,E[D+U+2]=i.z,E[D+U+3]=0),g===!0&&(i.fromBufferAttribute(y,R),E[D+U+4]=i.x,E[D+U+5]=i.y,E[D+U+6]=i.z,E[D+U+7]=0),_===!0&&(i.fromBufferAttribute(N,R),E[D+U+8]=i.x,E[D+U+9]=i.y,E[D+U+10]=i.z,E[D+U+11]=N.itemSize===4?i.w:1)}}d={count:u,texture:P,size:new Be(b,M)},n.set(o,d),o.addEventListener("dispose",C)}if(a.isInstancedMesh===!0&&a.morphTexture!==null)l.getUniforms().setValue(s,"morphTexture",a.morphTexture,t);else{let p=0;for(let _=0;_<c.length;_++)p+=c[_];let g=o.morphTargetsRelative?1:1-p;l.getUniforms().setValue(s,"morphTargetBaseInfluence",g),l.getUniforms().setValue(s,"morphTargetInfluences",c)}l.getUniforms().setValue(s,"morphTargetsTexture",d.texture,t),l.getUniforms().setValue(s,"morphTargetsTextureSize",d.size)}return{update:r}}function mx(s,e,t,n,i){let r=new WeakMap;function a(c){let h=i.render.frame,u=c.geometry,d=e.get(c,u);if(r.get(d)!==h&&(e.update(d),r.set(d,h)),c.isInstancedMesh&&(c.hasEventListener("dispose",l)===!1&&c.addEventListener("dispose",l),r.get(c)!==h&&(t.update(c.instanceMatrix,s.ARRAY_BUFFER),c.instanceColor!==null&&t.update(c.instanceColor,s.ARRAY_BUFFER),r.set(c,h))),c.isSkinnedMesh){let p=c.skeleton;r.get(p)!==h&&(p.update(),r.set(p,h))}return d}function o(){r=new WeakMap}function l(c){let h=c.target;h.removeEventListener("dispose",l),n.releaseStatesOfObject(h),t.remove(h.instanceMatrix),h.instanceColor!==null&&t.remove(h.instanceColor)}return{update:a,dispose:o}}var gx={[Ua]:"LINEAR_TONE_MAPPING",[Oa]:"REINHARD_TONE_MAPPING",[za]:"CINEON_TONE_MAPPING",[Os]:"ACES_FILMIC_TONE_MAPPING",[ka]:"AGX_TONE_MAPPING",[Ha]:"NEUTRAL_TONE_MAPPING",[Ba]:"CUSTOM_TONE_MAPPING"};function bx(s,e,t,n,i,r){let a=new Kt(e,t,{type:s,depthBuffer:i,stencilBuffer:r,samples:n?4:0,storeMultisampledDepthBuffer:!1,storeMultisampledStencilBuffer:!1,resolveDepthBuffer:!1,resolveStencilBuffer:!1}),o=null,l=null,c=new bt;c.setAttribute("position",new ut([-1,3,0,-1,-1,0,3,-1,0],3)),c.setAttribute("uv",new ut([0,2,0,0,2,0],2));let h=new Ar({uniforms:{tDiffuse:{value:null}},vertexShader:`
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
			}`,depthTest:!1,depthWrite:!1}),u=new it(c,h),d=new Mi(-1,1,1,-1,0,1),p=null,g=null,_=!1,m,f=null,v=[],S=!1;this.setSize=function(b,M){a.setSize(b,M),o!==null&&o.setSize(b,M),l!==null&&l.setSize(b,M);for(let E=0;E<v.length;E++){let P=v[E];P.setSize&&P.setSize(b,M)}},this.setEffects=function(b){v=b,S=v.length>0&&v[0].isRenderPass===!0;let M=a.width,E=a.height;v.length>0&&o===null&&(o=new Kt(M,E,{type:rn,depthBuffer:!1,stencilBuffer:!1}),l=new Kt(M,E,{type:rn,depthBuffer:!1,stencilBuffer:!1}));for(let P=0;P<v.length;P++){let x=v[P];x.setSize&&x.setSize(M,E)}},this.begin=function(b,M){if(_||b.toneMapping===ai&&v.length===0)return!1;if(f=M,M!==null){let E=M.width,P=M.height;(a.width!==E||a.height!==P)&&this.setSize(E,P)}return S===!1&&b.setRenderTarget(a),m=b.toneMapping,b.toneMapping=ai,!0},this.hasRenderPass=function(){return S},this.end=function(b,M){b.toneMapping=m,_=!0;let E=a,P=o;for(let x=0;x<v.length;x++){let C=v[x];C.enabled!==!1&&(C.render(b,P,E,M),C.needsSwap!==!1&&(E=P,P=P===o?l:o))}if(p!==b.outputColorSpace||g!==b.toneMapping){p=b.outputColorSpace,g=b.toneMapping,h.defines={},_t.getTransfer(p)===Dt&&(h.defines.SRGB_TRANSFER="");let x=gx[g];x&&(h.defines[x]=""),h.needsUpdate=!0}h.uniforms.tDiffuse.value=E.texture,b.setRenderTarget(f),b.render(u,d),f=null,_=!1},this.isCompositing=function(){return _},this.dispose=function(){a.dispose(),o!==null&&o.dispose(),l!==null&&l.dispose(),c.dispose(),h.dispose()}}var Xf=new Qt,Gh=new rs(1,1),jf=new ua,Kf=new Ko,Yf=new xa,Af=[],Rf=[],Cf=new Float32Array(16),Pf=new Float32Array(9),If=new Float32Array(4);function Hr(s,e,t){let n=s[0];if(n<=0||n>0)return s;let i=e*t,r=Af[i];if(r===void 0&&(r=new Float32Array(i),Af[i]=r),e!==0){n.toArray(r,0);for(let a=1,o=0;a!==e;++a)o+=t,s[a].toArray(r,o)}return r}function ln(s,e){if(s.length!==e.length)return!1;for(let t=0,n=s.length;t<n;t++)if(s[t]!==e[t])return!1;return!0}function cn(s,e){for(let t=0,n=e.length;t<n;t++)s[t]=e[t]}function fc(s,e){let t=Rf[e];t===void 0&&(t=new Int32Array(e),Rf[e]=t);for(let n=0;n!==e;++n)t[n]=s.allocateTextureUnit();return t}function xx(s,e){let t=this.cache;t[0]!==e&&(s.uniform1f(this.addr,e),t[0]=e)}function vx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2f(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(ln(t,e))return;s.uniform2fv(this.addr,e),cn(t,e)}}function _x(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3f(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else if(e.r!==void 0)(t[0]!==e.r||t[1]!==e.g||t[2]!==e.b)&&(s.uniform3f(this.addr,e.r,e.g,e.b),t[0]=e.r,t[1]=e.g,t[2]=e.b);else{if(ln(t,e))return;s.uniform3fv(this.addr,e),cn(t,e)}}function yx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4f(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(ln(t,e))return;s.uniform4fv(this.addr,e),cn(t,e)}}function Mx(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(ln(t,e))return;s.uniformMatrix2fv(this.addr,!1,e),cn(t,e)}else{if(ln(t,n))return;If.set(n),s.uniformMatrix2fv(this.addr,!1,If),cn(t,n)}}function Sx(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(ln(t,e))return;s.uniformMatrix3fv(this.addr,!1,e),cn(t,e)}else{if(ln(t,n))return;Pf.set(n),s.uniformMatrix3fv(this.addr,!1,Pf),cn(t,n)}}function wx(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(ln(t,e))return;s.uniformMatrix4fv(this.addr,!1,e),cn(t,e)}else{if(ln(t,n))return;Cf.set(n),s.uniformMatrix4fv(this.addr,!1,Cf),cn(t,n)}}function Ex(s,e){let t=this.cache;t[0]!==e&&(s.uniform1i(this.addr,e),t[0]=e)}function Tx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2i(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(ln(t,e))return;s.uniform2iv(this.addr,e),cn(t,e)}}function Ax(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3i(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(ln(t,e))return;s.uniform3iv(this.addr,e),cn(t,e)}}function Rx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4i(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(ln(t,e))return;s.uniform4iv(this.addr,e),cn(t,e)}}function Cx(s,e){let t=this.cache;t[0]!==e&&(s.uniform1ui(this.addr,e),t[0]=e)}function Px(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2ui(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(ln(t,e))return;s.uniform2uiv(this.addr,e),cn(t,e)}}function Ix(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3ui(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(ln(t,e))return;s.uniform3uiv(this.addr,e),cn(t,e)}}function Dx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4ui(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(ln(t,e))return;s.uniform4uiv(this.addr,e),cn(t,e)}}function Lx(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i);let r;this.type===s.SAMPLER_2D_SHADOW?(Gh.compareFunction=t.isReversedDepthBuffer()?oc:ac,r=Gh):r=Xf,t.setTexture2D(e||r,i)}function Fx(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture3D(e||Kf,i)}function Nx(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTextureCube(e||Yf,i)}function Ux(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture2DArray(e||jf,i)}function Ox(s){switch(s){case 5126:return xx;case 35664:return vx;case 35665:return _x;case 35666:return yx;case 35674:return Mx;case 35675:return Sx;case 35676:return wx;case 5124:case 35670:return Ex;case 35667:case 35671:return Tx;case 35668:case 35672:return Ax;case 35669:case 35673:return Rx;case 5125:return Cx;case 36294:return Px;case 36295:return Ix;case 36296:return Dx;case 35678:case 36198:case 36298:case 36306:case 35682:return Lx;case 35679:case 36299:case 36307:return Fx;case 35680:case 36300:case 36308:case 36293:return Nx;case 36289:case 36303:case 36311:case 36292:return Ux}}function zx(s,e){s.uniform1fv(this.addr,e)}function Bx(s,e){let t=Hr(e,this.size,2);s.uniform2fv(this.addr,t)}function kx(s,e){let t=Hr(e,this.size,3);s.uniform3fv(this.addr,t)}function Hx(s,e){let t=Hr(e,this.size,4);s.uniform4fv(this.addr,t)}function Gx(s,e){let t=Hr(e,this.size,4);s.uniformMatrix2fv(this.addr,!1,t)}function Vx(s,e){let t=Hr(e,this.size,9);s.uniformMatrix3fv(this.addr,!1,t)}function Wx(s,e){let t=Hr(e,this.size,16);s.uniformMatrix4fv(this.addr,!1,t)}function qx(s,e){s.uniform1iv(this.addr,e)}function Xx(s,e){s.uniform2iv(this.addr,e)}function jx(s,e){s.uniform3iv(this.addr,e)}function Kx(s,e){s.uniform4iv(this.addr,e)}function Yx(s,e){s.uniform1uiv(this.addr,e)}function Jx(s,e){s.uniform2uiv(this.addr,e)}function Zx(s,e){s.uniform3uiv(this.addr,e)}function $x(s,e){s.uniform4uiv(this.addr,e)}function Qx(s,e,t){let n=this.cache,i=e.length,r=fc(t,i);ln(n,r)||(s.uniform1iv(this.addr,r),cn(n,r));let a;this.type===s.SAMPLER_2D_SHADOW?a=Gh:a=Xf;for(let o=0;o!==i;++o)t.setTexture2D(e[o]||a,r[o])}function ev(s,e,t){let n=this.cache,i=e.length,r=fc(t,i);ln(n,r)||(s.uniform1iv(this.addr,r),cn(n,r));for(let a=0;a!==i;++a)t.setTexture3D(e[a]||Kf,r[a])}function tv(s,e,t){let n=this.cache,i=e.length,r=fc(t,i);ln(n,r)||(s.uniform1iv(this.addr,r),cn(n,r));for(let a=0;a!==i;++a)t.setTextureCube(e[a]||Yf,r[a])}function nv(s,e,t){let n=this.cache,i=e.length,r=fc(t,i);ln(n,r)||(s.uniform1iv(this.addr,r),cn(n,r));for(let a=0;a!==i;++a)t.setTexture2DArray(e[a]||jf,r[a])}function iv(s){switch(s){case 5126:return zx;case 35664:return Bx;case 35665:return kx;case 35666:return Hx;case 35674:return Gx;case 35675:return Vx;case 35676:return Wx;case 5124:case 35670:return qx;case 35667:case 35671:return Xx;case 35668:case 35672:return jx;case 35669:case 35673:return Kx;case 5125:return Yx;case 36294:return Jx;case 36295:return Zx;case 36296:return $x;case 35678:case 36198:case 36298:case 36306:case 35682:return Qx;case 35679:case 36299:case 36307:return ev;case 35680:case 36300:case 36308:case 36293:return tv;case 36289:case 36303:case 36311:case 36292:return nv}}var Vh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.setValue=Ox(t.type)}},Wh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.size=t.size,this.setValue=iv(t.type)}},qh=class{constructor(e){this.id=e,this.seq=[],this.map={}}setValue(e,t,n){let i=this.seq;for(let r=0,a=i.length;r!==a;++r){let o=i[r];o.setValue(e,t[o.id],n)}}},kh=/(\w+)(\])?(\[|\.)?/g;function Df(s,e){s.seq.push(e),s.map[e.id]=e}function sv(s,e,t){let n=s.name,i=n.length;for(kh.lastIndex=0;;){let r=kh.exec(n),a=kh.lastIndex,o=r[1],l=r[2]==="]",c=r[3];if(l&&(o=o|0),c===void 0||c==="["&&a+2===i){Df(t,c===void 0?new Vh(o,s,e):new Wh(o,s,e));break}else{let u=t.map[o];u===void 0&&(u=new qh(o),Df(t,u)),t=u}}}var Br=class{constructor(e,t){this.seq=[],this.map={};let n=e.getProgramParameter(t,e.ACTIVE_UNIFORMS);for(let a=0;a<n;++a){let o=e.getActiveUniform(t,a),l=e.getUniformLocation(t,o.name);sv(o,l,this)}let i=[],r=[];for(let a of this.seq)a.type===e.SAMPLER_2D_SHADOW||a.type===e.SAMPLER_CUBE_SHADOW||a.type===e.SAMPLER_2D_ARRAY_SHADOW?i.push(a):r.push(a);i.length>0&&(this.seq=i.concat(r))}setValue(e,t,n,i){let r=this.map[t];r!==void 0&&r.setValue(e,n,i)}setOptional(e,t,n){let i=t[n];i!==void 0&&this.setValue(e,n,i)}static upload(e,t,n,i){for(let r=0,a=t.length;r!==a;++r){let o=t[r],l=n[o.id];l.needsUpdate!==!1&&o.setValue(e,l.value,i)}}static seqWithValue(e,t){let n=[];for(let i=0,r=e.length;i!==r;++i){let a=e[i];a.id in t&&n.push(a)}return n}};function Lf(s,e,t){let n=s.createShader(e);return s.shaderSource(n,t),s.compileShader(n),n}var rv=37297,av=0;function ov(s,e){let t=s.split(`
`),n=[],i=Math.max(e-6,0),r=Math.min(e+6,t.length);for(let a=i;a<r;a++){let o=a+1;n.push(`${o===e?">":" "} ${o}: ${t[a]}`)}return n.join(`
`)}var Ff=new mt;function lv(s){_t._getMatrix(Ff,_t.workingColorSpace,s);let e=`mat3( ${Ff.elements.map(t=>t.toFixed(4))} )`;switch(_t.getTransfer(s)){case ca:return[e,"LinearTransferOETF"];case Dt:return[e,"sRGBTransferOETF"];default:return nt("WebGLProgram: Unsupported color space: ",s),[e,"LinearTransferOETF"]}}function Nf(s,e,t){let n=s.getShaderParameter(e,s.COMPILE_STATUS),r=(s.getShaderInfoLog(e)||"").trim();if(n&&r==="")return"";let a=/ERROR: 0:(\d+)/.exec(r);if(a){let o=parseInt(a[1]);return t.toUpperCase()+`

`+r+`

`+ov(s.getShaderSource(e),o)}else return r}function cv(s,e){let t=lv(e);return[`vec4 ${s}( vec4 value ) {`,`	return ${t[1]}( vec4( value.rgb * ${t[0]}, value.a ) );`,"}"].join(`
`)}var hv={[Ua]:"Linear",[Oa]:"Reinhard",[za]:"Cineon",[Os]:"ACESFilmic",[ka]:"AgX",[Ha]:"Neutral",[Ba]:"Custom"};function uv(s,e){let t=hv[e];return t===void 0?(nt("WebGLProgram: Unsupported toneMapping:",e),"vec3 "+s+"( vec3 color ) { return LinearToneMapping( color ); }"):"vec3 "+s+"( vec3 color ) { return "+t+"ToneMapping( color ); }"}var cc=new k;function dv(){_t.getLuminanceCoefficients(cc);let s=cc.x.toFixed(4),e=cc.y.toFixed(4),t=cc.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${s}, ${e}, ${t} );`,"	return dot( weights, rgb );","}"].join(`
`)}function fv(s){return[s.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",s.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(Qa).join(`
`)}function pv(s){let e=[];for(let t in s){let n=s[t];n!==!1&&e.push("#define "+t+" "+n)}return e.join(`
`)}function mv(s,e){let t={},n=s.getProgramParameter(e,s.ACTIVE_ATTRIBUTES);for(let i=0;i<n;i++){let r=s.getActiveAttrib(e,i),a=r.name,o=1;r.type===s.FLOAT_MAT2&&(o=2),r.type===s.FLOAT_MAT3&&(o=3),r.type===s.FLOAT_MAT4&&(o=4),t[a]={type:r.type,location:s.getAttribLocation(e,a),locationSize:o}}return t}function Qa(s){return s!==""}function Uf(s,e){let t=e.numSpotLightShadows+e.numSpotLightMaps-e.numSpotLightShadowsWithMaps;return s.replace(/NUM_SUN_LIGHTS/g,e.numSunLights).replace(/NUM_DIR_LIGHTS/g,e.numDirLights).replace(/NUM_SPOT_LIGHTS/g,e.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,e.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,t).replace(/NUM_RECT_AREA_LIGHTS/g,e.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,e.numPointLights).replace(/NUM_HEMI_LIGHTS/g,e.numHemiLights).replace(/NUM_SUN_LIGHT_SHADOWS/g,e.numSunLightShadows).replace(/NUM_DIR_LIGHT_SHADOWS/g,e.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,e.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,e.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,e.numPointLightShadows)}function Of(s,e){return s.replace(/NUM_CLIPPING_PLANES/g,e.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,e.numClippingPlanes-e.numClipIntersection)}var gv=/^[ \t]*#include +<([\w\d./]+)>/gm;function Xh(s){return s.replace(gv,xv)}var bv=new Map;function xv(s,e){let t=St[e];if(t===void 0){let n=bv.get(e);if(n!==void 0)t=St[n],nt('WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',e,n);else throw new Error("THREE.WebGLProgram: Can not resolve #include <"+e+">")}return Xh(t)}var vv=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function zf(s){return s.replace(vv,_v)}function _v(s,e,t,n){let i="";for(let r=parseInt(e);r<parseInt(t);r++)i+=n.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return i}function Bf(s){let e=`precision ${s.precision} float;
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
#define LOW_PRECISION`),e}var yv={[Ns]:"SHADOWMAP_TYPE_PCF",[Ir]:"SHADOWMAP_TYPE_VSM"};function Mv(s){return yv[s.shadowMapType]||"SHADOWMAP_TYPE_BASIC"}var Sv={[us]:"ENVMAP_TYPE_CUBE",[zs]:"ENVMAP_TYPE_CUBE",[Ga]:"ENVMAP_TYPE_CUBE_UV"};function wv(s){return s.envMap===!1?"ENVMAP_TYPE_CUBE":Sv[s.envMapMode]||"ENVMAP_TYPE_CUBE"}var Ev={[zs]:"ENVMAP_MODE_REFRACTION"};function Tv(s){return s.envMap===!1?"ENVMAP_MODE_REFLECTION":Ev[s.envMapMode]||"ENVMAP_MODE_REFLECTION"}var Av={[bl]:"ENVMAP_BLENDING_MULTIPLY",[ef]:"ENVMAP_BLENDING_MIX",[tf]:"ENVMAP_BLENDING_ADD"};function Rv(s){return s.envMap===!1?"ENVMAP_BLENDING_NONE":Av[s.combine]||"ENVMAP_BLENDING_NONE"}function Cv(s){let e=s.envMapCubeUVHeight;if(e===null)return null;let t=Math.log2(e)-2,n=1/e;return{texelWidth:1/(3*Math.max(Math.pow(2,t),112)),texelHeight:n,maxMip:t}}function Pv(s,e,t,n){let i=s.getContext(),r=t.defines,a=t.vertexShader,o=t.fragmentShader,l=Mv(t),c=wv(t),h=Tv(t),u=Rv(t),d=Cv(t),p=fv(t),g=pv(r),_=i.createProgram(),m,f,v=t.glslVersion?"#version "+t.glslVersion+`
`:"";t.isRawShaderMaterial?(m=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(Qa).join(`
`),m.length>0&&(m+=`
`),f=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(Qa).join(`
`),f.length>0&&(f+=`
`)):(m=[Bf(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",t.batching?"#define USE_BATCHING":"",t.batchingColor?"#define USE_BATCHING_COLOR":"",t.instancing?"#define USE_INSTANCING":"",t.instancingColor?"#define USE_INSTANCING_COLOR":"",t.instancingMorph?"#define USE_INSTANCING_MORPH":"",t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.map?"#define USE_MAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+h:"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.displacementMap?"#define USE_DISPLACEMENTMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.mapUv?"#define MAP_UV "+t.mapUv:"",t.alphaMapUv?"#define ALPHAMAP_UV "+t.alphaMapUv:"",t.lightMapUv?"#define LIGHTMAP_UV "+t.lightMapUv:"",t.aoMapUv?"#define AOMAP_UV "+t.aoMapUv:"",t.emissiveMapUv?"#define EMISSIVEMAP_UV "+t.emissiveMapUv:"",t.bumpMapUv?"#define BUMPMAP_UV "+t.bumpMapUv:"",t.normalMapUv?"#define NORMALMAP_UV "+t.normalMapUv:"",t.displacementMapUv?"#define DISPLACEMENTMAP_UV "+t.displacementMapUv:"",t.metalnessMapUv?"#define METALNESSMAP_UV "+t.metalnessMapUv:"",t.roughnessMapUv?"#define ROUGHNESSMAP_UV "+t.roughnessMapUv:"",t.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+t.anisotropyMapUv:"",t.clearcoatMapUv?"#define CLEARCOATMAP_UV "+t.clearcoatMapUv:"",t.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+t.clearcoatNormalMapUv:"",t.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+t.clearcoatRoughnessMapUv:"",t.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+t.iridescenceMapUv:"",t.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+t.iridescenceThicknessMapUv:"",t.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+t.sheenColorMapUv:"",t.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+t.sheenRoughnessMapUv:"",t.specularMapUv?"#define SPECULARMAP_UV "+t.specularMapUv:"",t.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+t.specularColorMapUv:"",t.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+t.specularIntensityMapUv:"",t.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+t.transmissionMapUv:"",t.thicknessMapUv?"#define THICKNESSMAP_UV "+t.thicknessMapUv:"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexNormals?"#define HAS_NORMAL":"",t.vertexColors?"#define USE_COLOR":"",t.vertexAlphas?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.flatShading?"#define FLAT_SHADED":"",t.skinning?"#define USE_SKINNING":"",t.morphTargets?"#define USE_MORPHTARGETS":"",t.morphNormals&&t.flatShading===!1?"#define USE_MORPHNORMALS":"",t.morphColors?"#define USE_MORPHCOLORS":"",t.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+t.morphTextureStride:"",t.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+t.morphTargetsCount:"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.sizeAttenuation?"#define USE_SIZEATTENUATION":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(Qa).join(`
`),f=[Bf(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",t.map?"#define USE_MAP":"",t.matcap?"#define USE_MATCAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+c:"",t.envMap?"#define "+h:"",t.envMap?"#define "+u:"",d?"#define CUBEUV_TEXEL_WIDTH "+d.texelWidth:"",d?"#define CUBEUV_TEXEL_HEIGHT "+d.texelHeight:"",d?"#define CUBEUV_MAX_MIP "+d.maxMip+".0":"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.packedNormalMap?"#define USE_PACKED_NORMALMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoat?"#define USE_CLEARCOAT":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.dispersion?"#define USE_DISPERSION":"",t.retroreflection?"#define USE_RETROREFLECTION":"",t.iridescence?"#define USE_IRIDESCENCE":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaTest?"#define USE_ALPHATEST":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.sheen?"#define USE_SHEEN":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexColors||t.instancingColor?"#define USE_COLOR":"",t.vertexAlphas||t.batchingColor?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.gradientMap?"#define USE_GRADIENTMAP":"",t.flatShading?"#define FLAT_SHADED":"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.numLightProbeGrids>0?"#define USE_LIGHT_PROBES_GRID":"",t.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",t.decodeVideoTextureEmissive?"#define DECODE_VIDEO_TEXTURE_EMISSIVE":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",t.toneMapping!==ai?"#define TONE_MAPPING":"",t.toneMapping!==ai?St.tonemapping_pars_fragment:"",t.toneMapping!==ai?uv("toneMapping",t.toneMapping):"",t.dithering?"#define DITHERING":"",t.opaque?"#define OPAQUE":"",St.colorspace_pars_fragment,cv("linearToOutputTexel",t.outputColorSpace),dv(),t.useDepthPacking?"#define DEPTH_PACKING "+t.depthPacking:"",`
`].filter(Qa).join(`
`)),a=Xh(a),a=Uf(a,t),a=Of(a,t),o=Xh(o),o=Uf(o,t),o=Of(o,t),a=zf(a),o=zf(o),t.isRawShaderMaterial!==!0&&(v=`#version 300 es
`,m=[p,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+m,f=["#define varying in",t.glslVersion===Ch?"":"layout(location = 0) out highp vec4 pc_fragColor;",t.glslVersion===Ch?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+f);let S=v+m+a,b=v+f+o,M=Lf(i,i.VERTEX_SHADER,S),E=Lf(i,i.FRAGMENT_SHADER,b);i.attachShader(_,M),i.attachShader(_,E),t.index0AttributeName!==void 0?i.bindAttribLocation(_,0,t.index0AttributeName):t.hasPositionAttribute===!0&&i.bindAttribLocation(_,0,"position"),i.linkProgram(_);function P(I){if(s.debug.checkShaderErrors){let y=i.getProgramInfoLog(_)||"",N=i.getShaderInfoLog(M)||"",D=i.getShaderInfoLog(E)||"",R=y.trim(),U=N.trim(),z=D.trim(),T=!0,G=!0;if(i.getProgramParameter(_,i.LINK_STATUS)===!1)if(T=!1,typeof s.debug.onShaderError=="function")s.debug.onShaderError(i,_,M,E);else{let X=Nf(i,M,"vertex"),Z=Nf(i,E,"fragment");ht("WebGLProgram: Shader Error "+i.getError()+" - VALIDATE_STATUS "+i.getProgramParameter(_,i.VALIDATE_STATUS)+`

Material Name: `+I.name+`
Material Type: `+I.type+`

Program Info Log: `+R+`
`+X+`
`+Z)}else R!==""?nt("WebGLProgram: Program Info Log:",R):(U===""||z==="")&&(G=!1);G&&(I.diagnostics={runnable:T,programLog:R,vertexShader:{log:U,prefix:m},fragmentShader:{log:z,prefix:f}})}i.deleteShader(M),i.deleteShader(E),x=new Br(i,_),C=mv(i,_)}let x;this.getUniforms=function(){return x===void 0&&P(this),x};let C;this.getAttributes=function(){return C===void 0&&P(this),C};let w=t.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return w===!1&&(w=i.getProgramParameter(_,rv)),w},this.destroy=function(){n.releaseStatesOfProgram(this),i.deleteProgram(_),this.program=void 0},this.type=t.shaderType,this.name=t.shaderName,this.id=av++,this.cacheKey=e,this.usedTimes=1,this.program=_,this.vertexShader=M,this.fragmentShader=E,this}var Iv=0,jh=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(e,t,n){let i=this._getShaderCacheForMaterial(e);return i.has(t)===!1&&(i.add(t),t.usedTimes++),i.has(n)===!1&&(i.add(n),n.usedTimes++),this}remove(e){let t=this.materialCache.get(e);for(let n of t)n.usedTimes--,n.usedTimes===0&&this.shaderCache.delete(n.code);return this.materialCache.delete(e),this}getVertexShaderStage(e){return this._getShaderStage(e.vertexShader)}getFragmentShaderStage(e){return this._getShaderStage(e.fragmentShader)}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(e){let t=this.materialCache,n=t.get(e);return n===void 0&&(n=new Set,t.set(e,n)),n}_getShaderStage(e){let t=this.shaderCache,n=t.get(e);return n===void 0&&(n=new Kh(e),t.set(e,n)),n}},Kh=class{constructor(e){this.id=Iv++,this.code=e,this.usedTimes=0}};function Dv(s){return s===fs||s===ja||s===Ka}function Lv(s,e,t,n,i,r){let a=new gr,o=new jh,l=new Set,c=[],h=new Map,u=n.logarithmicDepthBuffer,d=n.precision,p={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distance",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function g(x){return l.add(x),x===0?"uv":`uv${x}`}function _(x,C,w,I,y,N){let D=I.fog,R=y.geometry,U=x.isMeshStandardMaterial||x.isMeshLambertMaterial||x.isMeshPhongMaterial?I.environment:null,z=x.isMeshStandardMaterial||x.isMeshLambertMaterial&&!x.envMap||x.isMeshPhongMaterial&&!x.envMap,T=e.get(x.envMap||U,z),G=T&&T.mapping===Ga?T.image.height:null,X=p[x.type];x.precision!==null&&(d=n.getMaxPrecision(x.precision),d!==x.precision&&nt("WebGLProgram.getParameters:",x.precision,"not supported, using",d,"instead."));let Z=R.morphAttributes.position||R.morphAttributes.normal||R.morphAttributes.color,H=Z!==void 0?Z.length:0,oe=0;R.morphAttributes.position!==void 0&&(oe=1),R.morphAttributes.normal!==void 0&&(oe=2),R.morphAttributes.color!==void 0&&(oe=3);let ke,Fe,We,fe;if(X){let B=wi[X];ke=B.vertexShader,Fe=B.fragmentShader}else{ke=x.vertexShader,Fe=x.fragmentShader;let B=o.getVertexShaderStage(x),W=o.getFragmentShaderStage(x);o.update(x,B,W),We=B.id,fe=W.id}let re=s.getRenderTarget(),Pe=s.state.buffers.depth.getReversed(),be=y.isInstancedMesh===!0,ge=y.isBatchedMesh===!0,ie=!!x.map,le=!!x.matcap,se=!!T,ce=!!x.aoMap,ve=!!x.lightMap,Ee=!!x.bumpMap&&x.wireframe===!1,qe=!!x.normalMap,Ye=!!x.displacementMap,st=!!x.emissiveMap,at=!!x.metalnessMap,ot=!!x.roughnessMap,J=x.anisotropy>0,dt=x.clearcoat>0,ct=x.dispersion>0,O=x.retroreflectivity>0,A=x.iridescence>0,ne=x.sheen>0,de=x.transmission>0,he=J&&!!x.anisotropyMap,Ue=dt&&!!x.clearcoatMap,Oe=dt&&!!x.clearcoatNormalMap,V=dt&&!!x.clearcoatRoughnessMap,j=A&&!!x.iridescenceMap,me=A&&!!x.iridescenceThicknessMap,_e=ne&&!!x.sheenColorMap,pe=ne&&!!x.sheenRoughnessMap,xe=!!x.specularMap,Se=!!x.specularColorMap,we=!!x.specularIntensityMap,Ie=de&&!!x.transmissionMap,q=de&&!!x.thicknessMap,Te=!!x.gradientMap,Me=!!x.alphaMap,ue=x.alphaTest>0,Re=!!x.alphaHash,ye=!!x.extensions,je=ai;x.toneMapped&&(re===null||re.isXRRenderTarget===!0)&&(je=s.toneMapping);let Ke={shaderID:X,shaderType:x.type,shaderName:x.name,vertexShader:ke,fragmentShader:Fe,defines:x.defines,customVertexShaderID:We,customFragmentShaderID:fe,isRawShaderMaterial:x.isRawShaderMaterial===!0,glslVersion:x.glslVersion,precision:d,batching:ge,batchingColor:ge&&y._colorsTexture!==null,instancing:be,instancingColor:be&&y.instanceColor!==null,instancingMorph:be&&y.morphTexture!==null,outputColorSpace:re===null?s.outputColorSpace:re.isXRRenderTarget===!0?re.texture.colorSpace:_t.workingColorSpace,alphaToCoverage:!!x.alphaToCoverage,map:ie,matcap:le,envMap:se,envMapMode:se&&T.mapping,envMapCubeUVHeight:G,aoMap:ce,lightMap:ve,bumpMap:Ee,normalMap:qe,displacementMap:Ye,emissiveMap:st,normalMapObjectSpace:qe&&x.normalMapType===lf,normalMapTangentSpace:qe&&x.normalMapType===Ja,packedNormalMap:qe&&x.normalMapType===Ja&&Dv(x.normalMap.format),metalnessMap:at,roughnessMap:ot,anisotropy:J,anisotropyMap:he,clearcoat:dt,clearcoatMap:Ue,clearcoatNormalMap:Oe,clearcoatRoughnessMap:V,dispersion:ct,retroreflection:O,iridescence:A,iridescenceMap:j,iridescenceThicknessMap:me,sheen:ne,sheenColorMap:_e,sheenRoughnessMap:pe,specularMap:xe,specularColorMap:Se,specularIntensityMap:we,transmission:de,transmissionMap:Ie,thicknessMap:q,gradientMap:Te,opaque:x.transparent===!1&&x.blending===Dr&&x.alphaToCoverage===!1,alphaMap:Me,alphaTest:ue,alphaHash:Re,combine:x.combine,mapUv:ie&&g(x.map.channel),aoMapUv:ce&&g(x.aoMap.channel),lightMapUv:ve&&g(x.lightMap.channel),bumpMapUv:Ee&&g(x.bumpMap.channel),normalMapUv:qe&&g(x.normalMap.channel),displacementMapUv:Ye&&g(x.displacementMap.channel),emissiveMapUv:st&&g(x.emissiveMap.channel),metalnessMapUv:at&&g(x.metalnessMap.channel),roughnessMapUv:ot&&g(x.roughnessMap.channel),anisotropyMapUv:he&&g(x.anisotropyMap.channel),clearcoatMapUv:Ue&&g(x.clearcoatMap.channel),clearcoatNormalMapUv:Oe&&g(x.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:V&&g(x.clearcoatRoughnessMap.channel),iridescenceMapUv:j&&g(x.iridescenceMap.channel),iridescenceThicknessMapUv:me&&g(x.iridescenceThicknessMap.channel),sheenColorMapUv:_e&&g(x.sheenColorMap.channel),sheenRoughnessMapUv:pe&&g(x.sheenRoughnessMap.channel),specularMapUv:xe&&g(x.specularMap.channel),specularColorMapUv:Se&&g(x.specularColorMap.channel),specularIntensityMapUv:we&&g(x.specularIntensityMap.channel),transmissionMapUv:Ie&&g(x.transmissionMap.channel),thicknessMapUv:q&&g(x.thicknessMap.channel),alphaMapUv:Me&&g(x.alphaMap.channel),vertexTangents:!!R.attributes.tangent&&(qe||J),vertexNormals:!!R.attributes.normal,vertexColors:x.vertexColors,vertexAlphas:x.vertexColors===!0&&!!R.attributes.color&&R.attributes.color.itemSize===4,pointsUvs:y.isPoints===!0&&!!R.attributes.uv&&(ie||Me),fog:!!D,useFog:x.fog===!0,fogExp2:!!D&&D.isFogExp2,flatShading:x.wireframe===!1&&(x.flatShading===!0||R.attributes.normal===void 0&&qe===!1&&(x.isMeshLambertMaterial||x.isMeshPhongMaterial||x.isMeshStandardMaterial||x.isMeshPhysicalMaterial)),sizeAttenuation:x.sizeAttenuation===!0,logarithmicDepthBuffer:u,reversedDepthBuffer:Pe,skinning:y.isSkinnedMesh===!0,hasPositionAttribute:R.attributes.position!==void 0,morphTargets:R.morphAttributes.position!==void 0,morphNormals:R.morphAttributes.normal!==void 0,morphColors:R.morphAttributes.color!==void 0,morphTargetsCount:H,morphTextureStride:oe,numSunLights:C.sun.length,numDirLights:C.directional.length,numPointLights:C.point.length,numSpotLights:C.spot.length,numSpotLightMaps:C.spotLightMap.length,numRectAreaLights:C.rectArea.length,numHemiLights:C.hemi.length,numSunLightShadows:C.sunShadowMap.length,numDirLightShadows:C.directionalShadowMap.length,numPointLightShadows:C.pointShadowMap.length,numSpotLightShadows:C.spotShadowMap.length,numSpotLightShadowsWithMaps:C.numSpotLightShadowsWithMaps,numLightProbes:C.numLightProbes,numLightProbeGrids:N.length,numClippingPlanes:r.numPlanes,numClipIntersection:r.numIntersection,dithering:x.dithering,shadowMapEnabled:s.shadowMap.enabled&&w.length>0,shadowMapType:s.shadowMap.type,toneMapping:je,decodeVideoTexture:ie&&x.map.isVideoTexture===!0&&_t.getTransfer(x.map.colorSpace)===Dt,decodeVideoTextureEmissive:st&&x.emissiveMap.isVideoTexture===!0&&_t.getTransfer(x.emissiveMap.colorSpace)===Dt,premultipliedAlpha:x.premultipliedAlpha,doubleSided:x.side===Ot,flipSided:x.side===on,useDepthPacking:x.depthPacking>=0,depthPacking:x.depthPacking||0,index0AttributeName:x.index0AttributeName,extensionClipCullDistance:ye&&x.extensions.clipCullDistance===!0&&t.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(ye&&x.extensions.multiDraw===!0||ge)&&t.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:t.has("KHR_parallel_shader_compile"),customProgramCacheKey:x.customProgramCacheKey()};return Ke.vertexUv1s=l.has(1),Ke.vertexUv2s=l.has(2),Ke.vertexUv3s=l.has(3),l.clear(),Ke}function m(x){let C=[];if(x.shaderID?C.push(x.shaderID):(C.push(x.customVertexShaderID),C.push(x.customFragmentShaderID)),x.defines!==void 0)for(let w in x.defines)C.push(w),C.push(x.defines[w]);return x.isRawShaderMaterial===!1&&(f(C,x),v(C,x),C.push(s.outputColorSpace)),C.push(x.customProgramCacheKey),C.join()}function f(x,C){x.push(C.precision),x.push(C.outputColorSpace),x.push(C.envMapMode),x.push(C.envMapCubeUVHeight),x.push(C.mapUv),x.push(C.alphaMapUv),x.push(C.lightMapUv),x.push(C.aoMapUv),x.push(C.bumpMapUv),x.push(C.normalMapUv),x.push(C.displacementMapUv),x.push(C.emissiveMapUv),x.push(C.metalnessMapUv),x.push(C.roughnessMapUv),x.push(C.anisotropyMapUv),x.push(C.clearcoatMapUv),x.push(C.clearcoatNormalMapUv),x.push(C.clearcoatRoughnessMapUv),x.push(C.iridescenceMapUv),x.push(C.iridescenceThicknessMapUv),x.push(C.sheenColorMapUv),x.push(C.sheenRoughnessMapUv),x.push(C.specularMapUv),x.push(C.specularColorMapUv),x.push(C.specularIntensityMapUv),x.push(C.transmissionMapUv),x.push(C.thicknessMapUv),x.push(C.combine),x.push(C.fogExp2),x.push(C.sizeAttenuation),x.push(C.morphTargetsCount),x.push(C.morphAttributeCount),x.push(C.numSunLights),x.push(C.numDirLights),x.push(C.numPointLights),x.push(C.numSpotLights),x.push(C.numSpotLightMaps),x.push(C.numHemiLights),x.push(C.numRectAreaLights),x.push(C.numSunLightShadows),x.push(C.numDirLightShadows),x.push(C.numPointLightShadows),x.push(C.numSpotLightShadows),x.push(C.numSpotLightShadowsWithMaps),x.push(C.numLightProbes),x.push(C.shadowMapType),x.push(C.toneMapping),x.push(C.numClippingPlanes),x.push(C.numClipIntersection),x.push(C.depthPacking)}function v(x,C){a.disableAll(),C.instancing&&a.enable(0),C.instancingColor&&a.enable(1),C.instancingMorph&&a.enable(2),C.matcap&&a.enable(3),C.envMap&&a.enable(4),C.normalMapObjectSpace&&a.enable(5),C.normalMapTangentSpace&&a.enable(6),C.clearcoat&&a.enable(7),C.iridescence&&a.enable(8),C.alphaTest&&a.enable(9),C.vertexColors&&a.enable(10),C.vertexAlphas&&a.enable(11),C.vertexUv1s&&a.enable(12),C.vertexUv2s&&a.enable(13),C.vertexUv3s&&a.enable(14),C.vertexTangents&&a.enable(15),C.anisotropy&&a.enable(16),C.alphaHash&&a.enable(17),C.batching&&a.enable(18),C.dispersion&&a.enable(19),C.retroreflection&&a.enable(24),C.batchingColor&&a.enable(20),C.gradientMap&&a.enable(21),C.packedNormalMap&&a.enable(22),C.vertexNormals&&a.enable(23),x.push(a.mask),a.disableAll(),C.fog&&a.enable(0),C.useFog&&a.enable(1),C.flatShading&&a.enable(2),C.logarithmicDepthBuffer&&a.enable(3),C.reversedDepthBuffer&&a.enable(4),C.skinning&&a.enable(5),C.morphTargets&&a.enable(6),C.morphNormals&&a.enable(7),C.morphColors&&a.enable(8),C.premultipliedAlpha&&a.enable(9),C.shadowMapEnabled&&a.enable(10),C.doubleSided&&a.enable(11),C.flipSided&&a.enable(12),C.useDepthPacking&&a.enable(13),C.dithering&&a.enable(14),C.transmission&&a.enable(15),C.sheen&&a.enable(16),C.opaque&&a.enable(17),C.pointsUvs&&a.enable(18),C.decodeVideoTexture&&a.enable(19),C.decodeVideoTextureEmissive&&a.enable(20),C.alphaToCoverage&&a.enable(21),C.numLightProbeGrids>0&&a.enable(22),C.hasPositionAttribute&&a.enable(23),x.push(a.mask)}function S(x){let C=p[x.type],w;if(C){let I=wi[C];w=ci.clone(I.uniforms)}else w=x.uniforms;return w}function b(x,C){let w=h.get(C);return w!==void 0?++w.usedTimes:(w=new Pv(s,C,x,i),c.push(w),h.set(C,w)),w}function M(x){if(--x.usedTimes===0){let C=c.indexOf(x);c[C]=c[c.length-1],c.pop(),h.delete(x.cacheKey),x.destroy()}}function E(x){o.remove(x)}function P(){o.dispose()}return{getParameters:_,getProgramCacheKey:m,getUniforms:S,acquireProgram:b,releaseProgram:M,releaseShaderCache:E,programs:c,dispose:P}}function Fv(){let s=new WeakMap;function e(a){return s.has(a)}function t(a){let o=s.get(a);return o===void 0&&(o={},s.set(a,o)),o}function n(a){s.delete(a)}function i(a,o,l){s.get(a)[o]=l}function r(){s=new WeakMap}return{has:e,get:t,remove:n,update:i,dispose:r}}function Nv(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.material.id!==e.material.id?s.material.id-e.material.id:s.materialVariant!==e.materialVariant?s.materialVariant-e.materialVariant:s.z!==e.z?s.z-e.z:s.id-e.id}function kf(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.z!==e.z?e.z-s.z:s.id-e.id}function Hf(){let s=[],e=0,t=[],n=[],i=[];function r(){e=0,t.length=0,n.length=0,i.length=0}function a(d){let p=0;return d.isInstancedMesh&&(p+=2),d.isSkinnedMesh&&(p+=1),p}function o(d,p,g,_,m,f){let v=s[e];return v===void 0?(v={id:d.id,object:d,geometry:p,material:g,materialVariant:a(d),groupOrder:_,renderOrder:d.renderOrder,z:m,group:f},s[e]=v):(v.id=d.id,v.object=d,v.geometry=p,v.material=g,v.materialVariant=a(d),v.groupOrder=_,v.renderOrder=d.renderOrder,v.z=m,v.group=f),e++,v}function l(d,p,g,_,m,f,v){v.reversedDepth===!0&&(m=-m);let S=o(d,p,g,_,m,f);g.transmission>0?n.push(S):g.transparent===!0?i.push(S):t.push(S)}function c(d,p,g,_,m,f){let v=o(d,p,g,_,m,f);g.transmission>0?n.unshift(v):g.transparent===!0?i.unshift(v):t.unshift(v)}function h(d,p){t.length>1&&t.sort(d||Nv),n.length>1&&n.sort(p||kf),i.length>1&&i.sort(p||kf)}function u(){for(let d=e,p=s.length;d<p;d++){let g=s[d];if(g.id===null)break;g.id=null,g.object=null,g.geometry=null,g.material=null,g.group=null}}return{opaque:t,transmissive:n,transparent:i,init:r,push:l,unshift:c,finish:u,sort:h}}function Uv(){let s=new WeakMap;function e(n,i){let r=s.get(n),a;return r===void 0?(a=new Hf,s.set(n,[a])):i>=r.length?(a=new Hf,r.push(a)):a=r[i],a}function t(){s=new WeakMap}return{get:e,dispose:t}}function Ov(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"SunLight":case"DirectionalLight":t={direction:new k,color:new Le};break;case"SpotLight":t={position:new k,direction:new k,color:new Le,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":t={position:new k,color:new Le,distance:0,decay:0};break;case"HemisphereLight":t={direction:new k,skyColor:new Le,groundColor:new Le};break;case"RectAreaLight":t={color:new Le,position:new k,halfWidth:new k,halfHeight:new k};break}return s[e.id]=t,t}}}function zv(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"SunLight":case"DirectionalLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Be};break;case"SpotLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Be};break;case"PointLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Be,shadowCameraNear:1,shadowCameraFar:1e3};break}return s[e.id]=t,t}}}var Bv=0;function kv(s,e){return(e.castShadow?2:0)-(s.castShadow?2:0)+(e.map?1:0)-(s.map?1:0)}function Hv(s){let e=new Ov,t=zv(),n={version:0,hash:{sunLength:-1,directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numSunShadows:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],sun:[],sunShadow:[],sunShadowMap:[],sunShadowMatrix:[],sunShadowCascade:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let c=0;c<9;c++)n.probe.push(new k);let i=new k,r=new pt,a=new pt;function o(c){let h=0,u=0,d=0;for(let y=0;y<9;y++)n.probe[y].set(0,0,0);let p=0,g=0,_=0,m=0,f=0,v=0,S=0,b=0,M=0,E=0,P=0,x=0,C=0,w=0;c.sort(kv);for(let y=0,N=c.length;y<N;y++){let D=c[y],R=D.color,U=D.intensity,z=D.distance,T=null;if(D.shadow&&D.shadow.map&&(D.shadow.map.texture.format===fs?T=D.shadow.map.texture:T=D.shadow.map.depthTexture||D.shadow.map.texture),D.isAmbientLight)h+=R.r*U,u+=R.g*U,d+=R.b*U;else if(D.isLightProbe){for(let G=0;G<9;G++)n.probe[G].addScaledVector(D.sh.coefficients[G],U);w++}else if(D.isSunLight){let G=e.get(D);if(G.color.copy(D.color).multiplyScalar(D.intensity),D.castShadow){let X=D.shadow,Z=t.get(D);Z.shadowIntensity=X.intensity,Z.shadowBias=X.bias,Z.shadowNormalBias=X.normalBias,Z.shadowRadius=X.radius,Z.shadowMapSize.copy(X.mapSize).multiply(X.getFrameExtents()),n.sunShadow[g]=Z,n.sunShadowMap[g]=T;let H=X.getViewportCount();for(let oe=0;oe<H;oe++)n.sunShadowMatrix[_+oe]=X.getMatrix(oe),n.sunShadowCascade[_+oe]=X._cascadeData[oe];_+=H,g++}n.sun[p]=G,p++}else if(D.isDirectionalLight){let G=e.get(D);if(G.color.copy(D.color).multiplyScalar(D.intensity),D.castShadow){let X=D.shadow,Z=t.get(D);Z.shadowIntensity=X.intensity,Z.shadowBias=X.bias,Z.shadowNormalBias=X.normalBias,Z.shadowRadius=X.radius,Z.shadowMapSize=X.mapSize,n.directionalShadow[m]=Z,n.directionalShadowMap[m]=T,n.directionalShadowMatrix[m]=D.shadow.matrix,M++}n.directional[m]=G,m++}else if(D.isSpotLight){let G=e.get(D);G.position.setFromMatrixPosition(D.matrixWorld),G.color.copy(R).multiplyScalar(U),G.distance=z,G.coneCos=Math.cos(D.angle),G.penumbraCos=Math.cos(D.angle*(1-D.penumbra)),G.decay=D.decay,n.spot[v]=G;let X=D.shadow;if(D.map&&(n.spotLightMap[x]=D.map,x++,X.updateMatrices(D),D.castShadow&&C++),n.spotLightMatrix[v]=X.matrix,D.castShadow){let Z=t.get(D);Z.shadowIntensity=X.intensity,Z.shadowBias=X.bias,Z.shadowNormalBias=X.normalBias,Z.shadowRadius=X.radius,Z.shadowMapSize=X.mapSize,n.spotShadow[v]=Z,n.spotShadowMap[v]=T,P++}v++}else if(D.isRectAreaLight){let G=e.get(D);G.color.copy(R).multiplyScalar(U),G.halfWidth.set(D.width*.5,0,0),G.halfHeight.set(0,D.height*.5,0),n.rectArea[S]=G,S++}else if(D.isPointLight){let G=e.get(D);if(G.color.copy(D.color).multiplyScalar(D.intensity),G.distance=D.distance,G.decay=D.decay,D.castShadow){let X=D.shadow,Z=t.get(D);Z.shadowIntensity=X.intensity,Z.shadowBias=X.bias,Z.shadowNormalBias=X.normalBias,Z.shadowRadius=X.radius,Z.shadowMapSize=X.mapSize,Z.shadowCameraNear=X.camera.near,Z.shadowCameraFar=X.camera.far,n.pointShadow[f]=Z,n.pointShadowMap[f]=T,n.pointShadowMatrix[f]=D.shadow.matrix,E++}n.point[f]=G,f++}else if(D.isHemisphereLight){let G=e.get(D);G.skyColor.copy(D.color).multiplyScalar(U),G.groundColor.copy(D.groundColor).multiplyScalar(U),n.hemi[b]=G,b++}}S>0&&(s.has("OES_texture_float_linear")===!0?(n.rectAreaLTC1=Je.LTC_FLOAT_1,n.rectAreaLTC2=Je.LTC_FLOAT_2):(n.rectAreaLTC1=Je.LTC_HALF_1,n.rectAreaLTC2=Je.LTC_HALF_2)),n.ambient[0]=h,n.ambient[1]=u,n.ambient[2]=d;let I=n.hash;(I.sunLength!==p||I.directionalLength!==m||I.pointLength!==f||I.spotLength!==v||I.rectAreaLength!==S||I.hemiLength!==b||I.numSunShadows!==g||I.numDirectionalShadows!==M||I.numPointShadows!==E||I.numSpotShadows!==P||I.numSpotMaps!==x||I.numLightProbes!==w)&&(n.sun.length=p,n.directional.length=m,n.spot.length=v,n.rectArea.length=S,n.point.length=f,n.hemi.length=b,n.sunShadow.length=g,n.sunShadowMap.length=g,n.sunShadowMatrix.length=_,n.sunShadowCascade.length=_,n.directionalShadow.length=M,n.directionalShadowMap.length=M,n.directionalShadowMatrix.length=M,n.pointShadow.length=E,n.pointShadowMap.length=E,n.pointShadowMatrix.length=E,n.spotShadow.length=P,n.spotShadowMap.length=P,n.spotLightMatrix.length=P+x-C,n.spotLightMap.length=x,n.numSpotLightShadowsWithMaps=C,n.numLightProbes=w,I.sunLength=p,I.directionalLength=m,I.pointLength=f,I.spotLength=v,I.rectAreaLength=S,I.hemiLength=b,I.numSunShadows=g,I.numDirectionalShadows=M,I.numPointShadows=E,I.numSpotShadows=P,I.numSpotMaps=x,I.numLightProbes=w,n.version=Bv++)}function l(c,h){let u=0,d=0,p=0,g=0,_=0,m=0,f=h.matrixWorldInverse;for(let v=0,S=c.length;v<S;v++){let b=c[v];if(b.isSunLight){let M=n.sun[u];M.direction.setFromMatrixPosition(b.matrixWorld),M.direction.transformDirection(f),u++}else if(b.isDirectionalLight){let M=n.directional[d];M.direction.setFromMatrixPosition(b.matrixWorld),i.setFromMatrixPosition(b.target.matrixWorld),M.direction.sub(i),M.direction.transformDirection(f),d++}else if(b.isSpotLight){let M=n.spot[g];M.position.setFromMatrixPosition(b.matrixWorld),M.position.applyMatrix4(f),M.direction.setFromMatrixPosition(b.matrixWorld),i.setFromMatrixPosition(b.target.matrixWorld),M.direction.sub(i),M.direction.transformDirection(f),g++}else if(b.isRectAreaLight){let M=n.rectArea[_];M.position.setFromMatrixPosition(b.matrixWorld),M.position.applyMatrix4(f),a.identity(),r.copy(b.matrixWorld),r.premultiply(f),a.extractRotation(r),M.halfWidth.set(b.width*.5,0,0),M.halfHeight.set(0,b.height*.5,0),M.halfWidth.applyMatrix4(a),M.halfHeight.applyMatrix4(a),_++}else if(b.isPointLight){let M=n.point[p];M.position.setFromMatrixPosition(b.matrixWorld),M.position.applyMatrix4(f),p++}else if(b.isHemisphereLight){let M=n.hemi[m];M.direction.setFromMatrixPosition(b.matrixWorld),M.direction.transformDirection(f),m++}}}return{setup:o,setupView:l,state:n}}function Gf(s){let e=new Hv(s),t=[],n=[],i=[];function r(d){u.camera=d,t.length=0,n.length=0,i.length=0}function a(d){t.push(d)}function o(d){n.push(d)}function l(d){i.push(d)}function c(){e.setup(t)}function h(d){e.setupView(t,d)}let u={lightsArray:t,shadowsArray:n,lightProbeGridArray:i,camera:null,lights:e,transmissionRenderTarget:{},textureUnits:0};return{init:r,state:u,setupLights:c,setupLightsView:h,pushLight:a,pushShadow:o,pushLightProbeGrid:l}}function Gv(s){let e=new WeakMap;function t(i,r=0){let a=e.get(i),o;return a===void 0?(o=new Gf(s),e.set(i,[o])):r>=a.length?(o=new Gf(s),a.push(o)):o=a[r],o}function n(){e=new WeakMap}return{get:t,dispose:n}}var Vv=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,Wv=`uniform sampler2D shadow_pass;
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
}`,qv=[new k(1,0,0),new k(-1,0,0),new k(0,1,0),new k(0,-1,0),new k(0,0,1),new k(0,0,-1)],Xv=[new k(0,-1,0),new k(0,-1,0),new k(0,0,1),new k(0,0,-1),new k(0,-1,0),new k(0,-1,0)],Vf=new pt,$a=new k,Hh=new k;function jv(s,e,t){let n=new yr,i=new Be,r=new Be,a=new Pt,o=new rl,l=new al,c={},h=t.maxTextureSize,u={[jn]:on,[on]:jn,[Ot]:Ot},d=new vt({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new Be},radius:{value:4}},vertexShader:Vv,fragmentShader:Wv}),p=d.clone();p.defines.HORIZONTAL_PASS=1;let g=new bt;g.setAttribute("position",new Mt(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let _=new it(g,d),m=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=Ns;let f=this.type;this.render=function(E,P,x){if(m.enabled===!1||m.autoUpdate===!1&&m.needsUpdate===!1||E.length===0)return;this.type===Nd&&(nt("WebGLShadowMap: PCFSoftShadowMap has been removed. Using PCFShadowMap instead."),this.type=Ns);let C=s.getRenderTarget(),w=s.getActiveCubeFace(),I=s.getActiveMipmapLevel(),y=s.state;y.setBlending(Kn),y.buffers.depth.getReversed()===!0?y.buffers.color.setClear(0,0,0,0):y.buffers.color.setClear(1,1,1,1),y.buffers.depth.setTest(!0),y.setScissorTest(!1);let N=f!==this.type;N&&P.traverse(function(D){D.material&&(Array.isArray(D.material)?D.material.forEach(R=>R.needsUpdate=!0):D.material.needsUpdate=!0)});for(let D=0,R=E.length;D<R;D++){let U=E[D],z=U.shadow;if(z===void 0){nt("WebGLShadowMap:",U,"has no shadow.");continue}if(z.autoUpdate===!1&&z.needsUpdate===!1)continue;i.copy(z.mapSize);let T=z.getFrameExtents();i.multiply(T),r.copy(z.mapSize),(i.x>h||i.y>h)&&(i.x>h&&(r.x=Math.floor(h/T.x),i.x=r.x*T.x,z.mapSize.x=r.x),i.y>h&&(r.y=Math.floor(h/T.y),i.y=r.y*T.y,z.mapSize.y=r.y));let G=s.state.buffers.depth.getReversed();if(z.camera._reversedDepth=G,z.map===null||N===!0){if(z.map!==null&&(z.map.depthTexture!==null&&(z.map.depthTexture.dispose(),z.map.depthTexture=null),z.map.dispose()),this.type===Ir){if(U.isPointLight){nt("WebGLShadowMap: VSM shadow maps are not supported for PointLights. Use PCF or BasicShadowMap instead.");continue}z.map=new Kt(i.x,i.y,{format:fs,type:rn,minFilter:jt,magFilter:jt,generateMipmaps:!1}),z.map.texture.name=U.name+".shadowMap",z.map.depthTexture=new rs(i.x,i.y,kn),z.map.depthTexture.name=U.name+".shadowMapDepth",z.map.depthTexture.format=mi,z.map.depthTexture.compareFunction=null,z.map.depthTexture.minFilter=$t,z.map.depthTexture.magFilter=$t}else U.isPointLight?(z.map=new hc(i.x),z.map.depthTexture=new Zo(i.x,li)):(z.map=new Kt(i.x,i.y),z.map.depthTexture=new rs(i.x,i.y,li)),z.map.depthTexture.name=U.name+".shadowMap",z.map.depthTexture.format=mi,this.type===Ns?(z.map.depthTexture.compareFunction=G?oc:ac,z.map.depthTexture.minFilter=jt,z.map.depthTexture.magFilter=jt):(z.map.depthTexture.compareFunction=null,z.map.depthTexture.minFilter=$t,z.map.depthTexture.magFilter=$t);z.camera.updateProjectionMatrix()}z.map.isWebGLCubeRenderTarget!==!0&&(z.map.width!==i.x||z.map.height!==i.y)&&z.map.setSize(i.x,i.y);let X=z.map.isWebGLCubeRenderTarget?6:z.getViewportCount();U.isPointLight!==!0&&z.updateMatrices(U,x);for(let Z=0;Z<X;Z++){let H=z.getCamera(Z);if(U.isPointLight){let oe=z.camera,ke=z.matrix,Fe=U.distance||oe.far;Fe!==oe.far&&(oe.far=Fe,oe.updateProjectionMatrix()),$a.setFromMatrixPosition(U.matrixWorld),oe.position.copy($a),Hh.copy(oe.position),Hh.add(qv[Z]),oe.up.copy(Xv[Z]),oe.lookAt(Hh),oe.updateMatrixWorld(),ke.makeTranslation(-$a.x,-$a.y,-$a.z),Vf.multiplyMatrices(oe.projectionMatrix,oe.matrixWorldInverse),z._frustum.setFromProjectionMatrix(Vf,oe.coordinateSystem,oe.reversedDepth)}if(z.map.isWebGLCubeRenderTarget)s.setRenderTarget(z.map,Z),s.clear();else{Z===0&&(s.setRenderTarget(z.map),s.clear());let oe=z.getViewport(Z);a.set(r.x*oe.x,r.y*oe.y,r.x*oe.z,r.y*oe.w),y.viewport(a)}n=z.getFrustum(Z),b(P,x,H,U,this.type)}z.isPointLightShadow!==!0&&this.type===Ir&&v(z,x),z.needsUpdate=!1}f=this.type,m.needsUpdate=!1,s.setRenderTarget(C,w,I)};function v(E,P){let x=e.update(_);d.defines.VSM_SAMPLES!==E.blurSamples&&(d.defines.VSM_SAMPLES=E.blurSamples,p.defines.VSM_SAMPLES=E.blurSamples,d.needsUpdate=!0,p.needsUpdate=!0),E.mapPass===null?E.mapPass=new Kt(i.x,i.y,{format:fs,type:rn}):(E.mapPass.width!==E.map.width||E.mapPass.height!==E.map.height)&&E.mapPass.setSize(E.map.width,E.map.height),d.uniforms.shadow_pass.value=E.map.depthTexture,d.uniforms.resolution.value.set(E.map.width,E.map.height),d.uniforms.radius.value=E.radius,s.setRenderTarget(E.mapPass),s.clear(),s.renderBufferDirect(P,null,x,d,_,null),p.uniforms.shadow_pass.value=E.mapPass.texture,p.uniforms.resolution.value.set(E.map.width,E.map.height),p.uniforms.radius.value=E.radius,s.setRenderTarget(E.map),s.clear(),s.renderBufferDirect(P,null,x,p,_,null)}function S(E,P,x,C){let w=null,I=x.isPointLight===!0?E.customDistanceMaterial:E.customDepthMaterial;if(I!==void 0)w=I;else if(w=x.isPointLight===!0?l:o,s.localClippingEnabled&&P.clipShadows===!0&&Array.isArray(P.clippingPlanes)&&P.clippingPlanes.length!==0||P.displacementMap&&P.displacementScale!==0||P.alphaMap&&P.alphaTest>0||P.map&&P.alphaTest>0||P.alphaToCoverage===!0){let y=w.uuid,N=P.uuid,D=c[y];D===void 0&&(D={},c[y]=D);let R=D[N];R===void 0&&(R=w.clone(),D[N]=R,P.addEventListener("dispose",M)),w=R}if(w.visible=P.visible,w.wireframe=P.wireframe,C===Ir?w.side=P.shadowSide!==null?P.shadowSide:P.side:w.side=P.shadowSide!==null?P.shadowSide:u[P.side],w.alphaMap=P.alphaMap,w.alphaTest=P.alphaToCoverage===!0?.5:P.alphaTest,w.map=P.map,w.clipShadows=P.clipShadows,w.clippingPlanes=P.clippingPlanes,w.clipIntersection=P.clipIntersection,w.displacementMap=P.displacementMap,w.displacementScale=P.displacementScale,w.displacementBias=P.displacementBias,w.wireframeLinewidth=P.wireframeLinewidth,w.linewidth=P.linewidth,x.isPointLight===!0&&w.isMeshDistanceMaterial===!0){let y=s.properties.get(w);y.light=x}return w}function b(E,P,x,C,w){if(E.visible===!1)return;if(E.layers.test(P.layers)&&(E.isMesh||E.isLine||E.isPoints)&&(E.castShadow||E.receiveShadow&&w===Ir)&&(!E.frustumCulled||E.intersectsFrustum(n))){E.modelViewMatrix.multiplyMatrices(x.matrixWorldInverse,E.matrixWorld);let N=e.update(E),D=E.material;if(Array.isArray(D)){let R=N.groups;for(let U=0,z=R.length;U<z;U++){let T=R[U],G=D[T.materialIndex];if(G&&G.visible){let X=S(E,G,C,w);E.onBeforeShadow(s,E,P,x,N,X,T),s.renderBufferDirect(x,null,N,X,E,T),E.onAfterShadow(s,E,P,x,N,X,T)}}}else if(D.visible){let R=S(E,D,C,w);E.onBeforeShadow(s,E,P,x,N,R,null),s.renderBufferDirect(x,null,N,R,E,null),E.onAfterShadow(s,E,P,x,N,R,null)}}let y=E.children;for(let N=0,D=y.length;N<D;N++)b(y[N],P,x,C,w)}function M(E){E.target.removeEventListener("dispose",M);for(let x in c){let C=c[x],w=E.target.uuid;w in C&&(C[w].dispose(),delete C[w])}}}function Kv(s,e){function t(){let q=!1,Te=new Pt,Me=null,ue=new Pt(0,0,0,0);return{setMask:function(Re){Me!==Re&&!q&&(s.colorMask(Re,Re,Re,Re),Me=Re)},setLocked:function(Re){q=Re},setClear:function(Re,ye,je,Ke,B){B===!0&&(Re*=Ke,ye*=Ke,je*=Ke),Te.set(Re,ye,je,Ke),ue.equals(Te)===!1&&(s.clearColor(Re,ye,je,Ke),ue.copy(Te))},reset:function(){q=!1,Me=null,ue.set(-1,0,0,0)}}}function n(){let q=!1,Te=!1,Me=null,ue=null,Re=null;return{setReversed:function(ye){if(Te!==ye){let je=e.get("EXT_clip_control");ye?je.clipControlEXT(je.LOWER_LEFT_EXT,je.ZERO_TO_ONE_EXT):je.clipControlEXT(je.LOWER_LEFT_EXT,je.NEGATIVE_ONE_TO_ONE_EXT),Te=ye;let Ke=Re;Re=null,this.setClear(Ke)}},getReversed:function(){return Te},setTest:function(ye){ye?re(s.DEPTH_TEST):Pe(s.DEPTH_TEST)},setMask:function(ye){Me!==ye&&!q&&(s.depthMask(ye),Me=ye)},setFunc:function(ye){if(Te&&(ye=vf[ye]),ue!==ye){switch(ye){case Bo:s.depthFunc(s.NEVER);break;case ko:s.depthFunc(s.ALWAYS);break;case Ho:s.depthFunc(s.LESS);break;case hr:s.depthFunc(s.LEQUAL);break;case Go:s.depthFunc(s.EQUAL);break;case Vo:s.depthFunc(s.GEQUAL);break;case Wo:s.depthFunc(s.GREATER);break;case qo:s.depthFunc(s.NOTEQUAL);break;default:s.depthFunc(s.LEQUAL)}ue=ye}},setLocked:function(ye){q=ye},setClear:function(ye){Re!==ye&&(Re=ye,Te&&(ye=1-ye),s.clearDepth(ye))},reset:function(){q=!1,Me=null,ue=null,Re=null,Te=!1}}}function i(){let q=!1,Te=null,Me=null,ue=null,Re=null,ye=null,je=null,Ke=null,B=null;return{setTest:function(W){q||(W?re(s.STENCIL_TEST):Pe(s.STENCIL_TEST))},setMask:function(W){Te!==W&&!q&&(s.stencilMask(W),Te=W)},setFunc:function(W,ee,ae){(Me!==W||ue!==ee||Re!==ae)&&(s.stencilFunc(W,ee,ae),Me=W,ue=ee,Re=ae)},setOp:function(W,ee,ae){(ye!==W||je!==ee||Ke!==ae)&&(s.stencilOp(W,ee,ae),ye=W,je=ee,Ke=ae)},setLocked:function(W){q=W},setClear:function(W){B!==W&&(s.clearStencil(W),B=W)},reset:function(){q=!1,Te=null,Me=null,ue=null,Re=null,ye=null,je=null,Ke=null,B=null}}}let r=new t,a=new n,o=new i,l=new WeakMap,c=new WeakMap,h={},u={},d={},p=new WeakMap,g=[],_=null,m=!1,f=null,v=null,S=null,b=null,M=null,E=null,P=null,x=new Le(0,0,0),C=0,w=!1,I=null,y=null,N=null,D=null,R=null,U=s.getParameter(s.MAX_COMBINED_TEXTURE_IMAGE_UNITS),z=!1,T=0,G=s.getParameter(s.VERSION);G.indexOf("WebGL")!==-1?(T=parseFloat(/^WebGL (\d)/.exec(G)[1]),z=T>=1):G.indexOf("OpenGL ES")!==-1&&(T=parseFloat(/^OpenGL ES (\d)/.exec(G)[1]),z=T>=2);let X=null,Z={},H=s.getParameter(s.SCISSOR_BOX),oe=s.getParameter(s.VIEWPORT),ke=new Pt().fromArray(H),Fe=new Pt().fromArray(oe);function We(q,Te,Me,ue){let Re=new Uint8Array(4),ye=s.createTexture();s.bindTexture(q,ye),s.texParameteri(q,s.TEXTURE_MIN_FILTER,s.NEAREST),s.texParameteri(q,s.TEXTURE_MAG_FILTER,s.NEAREST);for(let je=0;je<Me;je++)q===s.TEXTURE_3D||q===s.TEXTURE_2D_ARRAY?s.texImage3D(Te,0,s.RGBA,1,1,ue,0,s.RGBA,s.UNSIGNED_BYTE,Re):s.texImage2D(Te+je,0,s.RGBA,1,1,0,s.RGBA,s.UNSIGNED_BYTE,Re);return ye}let fe={};fe[s.TEXTURE_2D]=We(s.TEXTURE_2D,s.TEXTURE_2D,1),fe[s.TEXTURE_CUBE_MAP]=We(s.TEXTURE_CUBE_MAP,s.TEXTURE_CUBE_MAP_POSITIVE_X,6),fe[s.TEXTURE_2D_ARRAY]=We(s.TEXTURE_2D_ARRAY,s.TEXTURE_2D_ARRAY,1,1),fe[s.TEXTURE_3D]=We(s.TEXTURE_3D,s.TEXTURE_3D,1,1),r.setClear(0,0,0,1),a.setClear(1),o.setClear(0),re(s.DEPTH_TEST),a.setFunc(hr),Ee(!1),qe(mh),re(s.CULL_FACE),ce(Kn);function re(q){h[q]!==!0&&(s.enable(q),h[q]=!0)}function Pe(q){h[q]!==!1&&(s.disable(q),h[q]=!1)}function be(q,Te){return d[q]!==Te?(s.bindFramebuffer(q,Te),d[q]=Te,q===s.DRAW_FRAMEBUFFER&&(d[s.FRAMEBUFFER]=Te),q===s.FRAMEBUFFER&&(d[s.DRAW_FRAMEBUFFER]=Te),!0):!1}function ge(q,Te){let Me=g,ue=!1;if(q){Me=p.get(Te),Me===void 0&&(Me=[],p.set(Te,Me));let Re=q.textures;if(Me.length!==Re.length||Me[0]!==s.COLOR_ATTACHMENT0){for(let ye=0,je=Re.length;ye<je;ye++)Me[ye]=s.COLOR_ATTACHMENT0+ye;Me.length=Re.length,ue=!0}}else Me[0]!==s.BACK&&(Me[0]=s.BACK,ue=!0);ue&&s.drawBuffers(Me)}function ie(q){return _!==q?(s.useProgram(q),_=q,!0):!1}let le={[Us]:s.FUNC_ADD,[Od]:s.FUNC_SUBTRACT,[zd]:s.FUNC_REVERSE_SUBTRACT};le[Bd]=s.MIN,le[kd]=s.MAX;let se={[Hd]:s.ZERO,[Gd]:s.ONE,[Vd]:s.SRC_COLOR,[xh]:s.SRC_ALPHA,[Yd]:s.SRC_ALPHA_SATURATE,[jd]:s.DST_COLOR,[qd]:s.DST_ALPHA,[Wd]:s.ONE_MINUS_SRC_COLOR,[vh]:s.ONE_MINUS_SRC_ALPHA,[Kd]:s.ONE_MINUS_DST_COLOR,[Xd]:s.ONE_MINUS_DST_ALPHA,[Jd]:s.CONSTANT_COLOR,[Zd]:s.ONE_MINUS_CONSTANT_COLOR,[$d]:s.CONSTANT_ALPHA,[Qd]:s.ONE_MINUS_CONSTANT_ALPHA};function ce(q,Te,Me,ue,Re,ye,je,Ke,B,W){if(q===Kn){m===!0&&(Pe(s.BLEND),m=!1);return}if(m===!1&&(re(s.BLEND),m=!0),q!==Ud){if(q!==f||W!==w){if((v!==Us||M!==Us)&&(s.blendEquation(s.FUNC_ADD),v=Us,M=Us),W)switch(q){case Dr:s.blendFuncSeparate(s.ONE,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case qt:s.blendFunc(s.ONE,s.ONE);break;case gh:s.blendFuncSeparate(s.ZERO,s.ONE_MINUS_SRC_COLOR,s.ZERO,s.ONE);break;case bh:s.blendFuncSeparate(s.DST_COLOR,s.ONE_MINUS_SRC_ALPHA,s.ZERO,s.ONE);break;default:ht("WebGLState: Invalid blending: ",q);break}else switch(q){case Dr:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case qt:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE,s.ONE,s.ONE);break;case gh:ht("WebGLState: SubtractiveBlending requires material.premultipliedAlpha = true");break;case bh:ht("WebGLState: MultiplyBlending requires material.premultipliedAlpha = true");break;default:ht("WebGLState: Invalid blending: ",q);break}S=null,b=null,E=null,P=null,x.set(0,0,0),C=0,f=q,w=W}return}Re=Re||Te,ye=ye||Me,je=je||ue,(Te!==v||Re!==M)&&(s.blendEquationSeparate(le[Te],le[Re]),v=Te,M=Re),(Me!==S||ue!==b||ye!==E||je!==P)&&(s.blendFuncSeparate(se[Me],se[ue],se[ye],se[je]),S=Me,b=ue,E=ye,P=je),(Ke.equals(x)===!1||B!==C)&&(s.blendColor(Ke.r,Ke.g,Ke.b,B),x.copy(Ke),C=B),f=q,w=!1}function ve(q,Te){q.side===Ot?Pe(s.CULL_FACE):re(s.CULL_FACE);let Me=q.side===on;Te&&(Me=!Me),Ee(Me),q.blending===Dr&&q.transparent===!1?ce(Kn):ce(q.blending,q.blendEquation,q.blendSrc,q.blendDst,q.blendEquationAlpha,q.blendSrcAlpha,q.blendDstAlpha,q.blendColor,q.blendAlpha,q.premultipliedAlpha),a.setFunc(q.depthFunc),a.setTest(q.depthTest),a.setMask(q.depthWrite),r.setMask(q.colorWrite);let ue=q.stencilWrite;o.setTest(ue),ue&&(o.setMask(q.stencilWriteMask),o.setFunc(q.stencilFunc,q.stencilRef,q.stencilFuncMask),o.setOp(q.stencilFail,q.stencilZFail,q.stencilZPass)),st(q.polygonOffset,q.polygonOffsetFactor,q.polygonOffsetUnits),q.alphaToCoverage===!0?re(s.SAMPLE_ALPHA_TO_COVERAGE):Pe(s.SAMPLE_ALPHA_TO_COVERAGE)}function Ee(q){I!==q&&(q?s.frontFace(s.CW):s.frontFace(s.CCW),I=q)}function qe(q){q!==Ld?(re(s.CULL_FACE),q!==y&&(q===mh?s.cullFace(s.BACK):q===Fd?s.cullFace(s.FRONT):s.cullFace(s.FRONT_AND_BACK))):Pe(s.CULL_FACE),y=q}function Ye(q){q!==N&&(z&&s.lineWidth(q),N=q)}function st(q,Te,Me){q?(re(s.POLYGON_OFFSET_FILL),(D!==Te||R!==Me)&&(D=Te,R=Me,a.getReversed()&&(Te=-Te),s.polygonOffset(Te,Me))):Pe(s.POLYGON_OFFSET_FILL)}function at(q){q?re(s.SCISSOR_TEST):Pe(s.SCISSOR_TEST)}function ot(q){q===void 0&&(q=s.TEXTURE0+U-1),X!==q&&(s.activeTexture(q),X=q)}function J(q,Te,Me){Me===void 0&&(X===null?Me=s.TEXTURE0+U-1:Me=X);let ue=Z[Me];ue===void 0&&(ue={type:void 0,texture:void 0},Z[Me]=ue),(ue.type!==q||ue.texture!==Te)&&(X!==Me&&(s.activeTexture(Me),X=Me),s.bindTexture(q,Te||fe[q]),ue.type=q,ue.texture=Te)}function dt(){let q=Z[X];q!==void 0&&q.type!==void 0&&(s.bindTexture(q.type,null),q.type=void 0,q.texture=void 0)}function ct(){try{s.compressedTexImage2D(...arguments)}catch(q){ht("WebGLState:",q)}}function O(){try{s.compressedTexImage3D(...arguments)}catch(q){ht("WebGLState:",q)}}function A(){try{s.texSubImage2D(...arguments)}catch(q){ht("WebGLState:",q)}}function ne(){try{s.texSubImage3D(...arguments)}catch(q){ht("WebGLState:",q)}}function de(){try{s.compressedTexSubImage2D(...arguments)}catch(q){ht("WebGLState:",q)}}function he(){try{s.compressedTexSubImage3D(...arguments)}catch(q){ht("WebGLState:",q)}}function Ue(){try{s.texStorage2D(...arguments)}catch(q){ht("WebGLState:",q)}}function Oe(){try{s.texStorage3D(...arguments)}catch(q){ht("WebGLState:",q)}}function V(){try{s.texImage2D(...arguments)}catch(q){ht("WebGLState:",q)}}function j(){try{s.texImage3D(...arguments)}catch(q){ht("WebGLState:",q)}}function me(q){return u[q]!==void 0?u[q]:s.getParameter(q)}function _e(q,Te){u[q]!==Te&&(s.pixelStorei(q,Te),u[q]=Te)}function pe(q){ke.equals(q)===!1&&(s.scissor(q.x,q.y,q.z,q.w),ke.copy(q))}function xe(q){Fe.equals(q)===!1&&(s.viewport(q.x,q.y,q.z,q.w),Fe.copy(q))}function Se(q,Te){let Me=c.get(Te);Me===void 0&&(Me=new WeakMap,c.set(Te,Me));let ue=Me.get(q);ue===void 0&&(ue=s.getUniformBlockIndex(Te,q.name),Me.set(q,ue))}function we(q,Te){let ue=c.get(Te).get(q);l.get(Te)!==ue&&(s.uniformBlockBinding(Te,ue,q.__bindingPointIndex),l.set(Te,ue))}function Ie(){s.disable(s.BLEND),s.disable(s.CULL_FACE),s.disable(s.DEPTH_TEST),s.disable(s.POLYGON_OFFSET_FILL),s.disable(s.SCISSOR_TEST),s.disable(s.STENCIL_TEST),s.disable(s.SAMPLE_ALPHA_TO_COVERAGE),s.blendEquation(s.FUNC_ADD),s.blendFunc(s.ONE,s.ZERO),s.blendFuncSeparate(s.ONE,s.ZERO,s.ONE,s.ZERO),s.blendColor(0,0,0,0),s.colorMask(!0,!0,!0,!0),s.clearColor(0,0,0,0),s.depthMask(!0),s.depthFunc(s.LESS),a.setReversed(!1),s.clearDepth(1),s.stencilMask(4294967295),s.stencilFunc(s.ALWAYS,0,4294967295),s.stencilOp(s.KEEP,s.KEEP,s.KEEP),s.clearStencil(0),s.cullFace(s.BACK),s.frontFace(s.CCW),s.polygonOffset(0,0),s.activeTexture(s.TEXTURE0),s.bindFramebuffer(s.FRAMEBUFFER,null),s.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),s.bindFramebuffer(s.READ_FRAMEBUFFER,null),s.useProgram(null),s.lineWidth(1),s.scissor(0,0,s.canvas.width,s.canvas.height),s.viewport(0,0,s.canvas.width,s.canvas.height),s.pixelStorei(s.PACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,!1),s.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!1),s.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,s.BROWSER_DEFAULT_WEBGL),s.pixelStorei(s.PACK_ROW_LENGTH,0),s.pixelStorei(s.PACK_SKIP_PIXELS,0),s.pixelStorei(s.PACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_ROW_LENGTH,0),s.pixelStorei(s.UNPACK_IMAGE_HEIGHT,0),s.pixelStorei(s.UNPACK_SKIP_PIXELS,0),s.pixelStorei(s.UNPACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_SKIP_IMAGES,0),h={},u={},X=null,Z={},d={},p=new WeakMap,g=[],_=null,m=!1,f=null,v=null,S=null,b=null,M=null,E=null,P=null,x=new Le(0,0,0),C=0,w=!1,I=null,y=null,N=null,D=null,R=null,ke.set(0,0,s.canvas.width,s.canvas.height),Fe.set(0,0,s.canvas.width,s.canvas.height),r.reset(),a.reset(),o.reset()}return{buffers:{color:r,depth:a,stencil:o},enable:re,disable:Pe,bindFramebuffer:be,drawBuffers:ge,useProgram:ie,setBlending:ce,setMaterial:ve,setFlipSided:Ee,setCullFace:qe,setLineWidth:Ye,setPolygonOffset:st,setScissorTest:at,activeTexture:ot,bindTexture:J,unbindTexture:dt,compressedTexImage2D:ct,compressedTexImage3D:O,texImage2D:V,texImage3D:j,pixelStorei:_e,getParameter:me,updateUBOMapping:Se,uniformBlockBinding:we,texStorage2D:Ue,texStorage3D:Oe,texSubImage2D:A,texSubImage3D:ne,compressedTexSubImage2D:de,compressedTexSubImage3D:he,scissor:pe,viewport:xe,reset:Ie}}function Yv(s,e,t,n,i,r,a){let o=e.has("WEBGL_multisampled_render_to_texture")?e.get("WEBGL_multisampled_render_to_texture"):null,l=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),c=new Be,h=new WeakMap,u=new Set,d,p=new WeakMap,g=!1;try{g=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function _(O,A){return g?new OffscreenCanvas(O,A):fr("canvas")}function m(O,A,ne){let de=1,he=ct(O);if((he.width>ne||he.height>ne)&&(de=ne/Math.max(he.width,he.height)),de<1)if(typeof HTMLImageElement<"u"&&O instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&O instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&O instanceof ImageBitmap||typeof VideoFrame<"u"&&O instanceof VideoFrame){let Ue=Math.floor(de*he.width),Oe=Math.floor(de*he.height);d===void 0&&(d=_(Ue,Oe));let V=A?_(Ue,Oe):d;return V.width=Ue,V.height=Oe,V.getContext("2d").drawImage(O,0,0,Ue,Oe),nt("WebGLRenderer: Texture has been resized from ("+he.width+"x"+he.height+") to ("+Ue+"x"+Oe+")."),V}else return"data"in O&&nt("WebGLRenderer: Image in DataTexture is too big ("+he.width+"x"+he.height+")."),O;return O}function f(O){return O.generateMipmaps}function v(O){s.generateMipmap(O)}function S(O){return O.isWebGLCubeRenderTarget?s.TEXTURE_CUBE_MAP:O.isWebGL3DRenderTarget?s.TEXTURE_3D:O.isWebGLArrayRenderTarget||O.isCompressedArrayTexture?s.TEXTURE_2D_ARRAY:s.TEXTURE_2D}function b(O,A,ne,de,he,Ue=!1){if(O!==null){if(s[O]!==void 0)return s[O];nt("WebGLRenderer: Attempt to use non-existing WebGL internal format '"+O+"'")}let Oe;de&&(Oe=e.get("EXT_texture_norm16"),Oe||nt("WebGLRenderer: Unable to use normalized textures without EXT_texture_norm16 extension"));let V=A;if(A===s.RED&&(ne===s.FLOAT&&(V=s.R32F),ne===s.HALF_FLOAT&&(V=s.R16F),ne===s.UNSIGNED_BYTE&&(V=s.R8),ne===s.UNSIGNED_SHORT&&Oe&&(V=Oe.R16_EXT),ne===s.SHORT&&Oe&&(V=Oe.R16_SNORM_EXT)),A===s.RED_INTEGER&&(ne===s.UNSIGNED_BYTE&&(V=s.R8UI),ne===s.UNSIGNED_SHORT&&(V=s.R16UI),ne===s.UNSIGNED_INT&&(V=s.R32UI),ne===s.BYTE&&(V=s.R8I),ne===s.SHORT&&(V=s.R16I),ne===s.INT&&(V=s.R32I)),A===s.RG&&(ne===s.FLOAT&&(V=s.RG32F),ne===s.HALF_FLOAT&&(V=s.RG16F),ne===s.UNSIGNED_BYTE&&(V=s.RG8),ne===s.UNSIGNED_SHORT&&Oe&&(V=Oe.RG16_EXT),ne===s.SHORT&&Oe&&(V=Oe.RG16_SNORM_EXT)),A===s.RG_INTEGER&&(ne===s.UNSIGNED_BYTE&&(V=s.RG8UI),ne===s.UNSIGNED_SHORT&&(V=s.RG16UI),ne===s.UNSIGNED_INT&&(V=s.RG32UI),ne===s.BYTE&&(V=s.RG8I),ne===s.SHORT&&(V=s.RG16I),ne===s.INT&&(V=s.RG32I)),A===s.RGB_INTEGER&&(ne===s.UNSIGNED_BYTE&&(V=s.RGB8UI),ne===s.UNSIGNED_SHORT&&(V=s.RGB16UI),ne===s.UNSIGNED_INT&&(V=s.RGB32UI),ne===s.BYTE&&(V=s.RGB8I),ne===s.SHORT&&(V=s.RGB16I),ne===s.INT&&(V=s.RGB32I)),A===s.RGBA_INTEGER&&(ne===s.UNSIGNED_BYTE&&(V=s.RGBA8UI),ne===s.UNSIGNED_SHORT&&(V=s.RGBA16UI),ne===s.UNSIGNED_INT&&(V=s.RGBA32UI),ne===s.BYTE&&(V=s.RGBA8I),ne===s.SHORT&&(V=s.RGBA16I),ne===s.INT&&(V=s.RGBA32I)),A===s.RGB&&(ne===s.UNSIGNED_SHORT&&Oe&&(V=Oe.RGB16_EXT),ne===s.SHORT&&Oe&&(V=Oe.RGB16_SNORM_EXT),ne===s.UNSIGNED_INT_5_9_9_9_REV&&(V=s.RGB9_E5),ne===s.UNSIGNED_INT_10F_11F_11F_REV&&(V=s.R11F_G11F_B10F)),A===s.RGBA){let j=Ue?ca:_t.getTransfer(he);ne===s.FLOAT&&(V=s.RGBA32F),ne===s.HALF_FLOAT&&(V=s.RGBA16F),ne===s.UNSIGNED_BYTE&&(V=j===Dt?s.SRGB8_ALPHA8:s.RGBA8),ne===s.UNSIGNED_SHORT&&Oe&&(V=Oe.RGBA16_EXT),ne===s.SHORT&&Oe&&(V=Oe.RGBA16_SNORM_EXT),ne===s.UNSIGNED_SHORT_4_4_4_4&&(V=s.RGBA4),ne===s.UNSIGNED_SHORT_5_5_5_1&&(V=s.RGB5_A1)}return(V===s.R16F||V===s.R32F||V===s.RG16F||V===s.RG32F||V===s.RGBA16F||V===s.RGBA32F)&&e.get("EXT_color_buffer_float"),V}function M(O,A){let ne;return O?A===null||A===li||A===Nr?ne=s.DEPTH24_STENCIL8:A===kn?ne=s.DEPTH32F_STENCIL8:A===Fr&&(ne=s.DEPTH24_STENCIL8,nt("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):A===null||A===li||A===Nr?ne=s.DEPTH_COMPONENT24:A===kn?ne=s.DEPTH_COMPONENT32F:A===Fr&&(ne=s.DEPTH_COMPONENT16),ne}function E(O,A){return f(O)===!0||O.isFramebufferTexture&&O.minFilter!==$t&&O.minFilter!==jt?Math.log2(Math.max(A.width,A.height))+1:O.mipmaps!==void 0&&O.mipmaps.length>0?O.mipmaps.length:O.isCompressedTexture&&Array.isArray(O.image)?A.mipmaps.length:1}function P(O){let A=O.target;A.removeEventListener("dispose",P),C(A),A.isVideoTexture&&h.delete(A),A.isHTMLTexture&&u.delete(A)}function x(O){let A=O.target;A.removeEventListener("dispose",x),I(A)}function C(O){let A=n.get(O);if(A.__webglInit===void 0)return;let ne=O.source,de=p.get(ne);if(de){let he=de[A.__cacheKey];he.usedTimes--,he.usedTimes===0&&w(O),Object.keys(de).length===0&&p.delete(ne)}n.remove(O)}function w(O){let A=n.get(O);s.deleteTexture(A.__webglTexture);let ne=O.source,de=p.get(ne);delete de[A.__cacheKey],a.memory.textures--}function I(O){let A=n.get(O);if(O.depthTexture&&(O.depthTexture.dispose(),n.remove(O.depthTexture)),O.isWebGLCubeRenderTarget)for(let de=0;de<6;de++){if(Array.isArray(A.__webglFramebuffer[de]))for(let he=0;he<A.__webglFramebuffer[de].length;he++)s.deleteFramebuffer(A.__webglFramebuffer[de][he]);else s.deleteFramebuffer(A.__webglFramebuffer[de]);A.__webglDepthbuffer&&s.deleteRenderbuffer(A.__webglDepthbuffer[de])}else{if(Array.isArray(A.__webglFramebuffer))for(let de=0;de<A.__webglFramebuffer.length;de++)s.deleteFramebuffer(A.__webglFramebuffer[de]);else s.deleteFramebuffer(A.__webglFramebuffer);if(A.__webglDepthbuffer&&s.deleteRenderbuffer(A.__webglDepthbuffer),A.__webglMultisampledFramebuffer&&s.deleteFramebuffer(A.__webglMultisampledFramebuffer),A.__webglColorRenderbuffer)for(let de=0;de<A.__webglColorRenderbuffer.length;de++)A.__webglColorRenderbuffer[de]&&s.deleteRenderbuffer(A.__webglColorRenderbuffer[de]);A.__webglDepthRenderbuffer&&s.deleteRenderbuffer(A.__webglDepthRenderbuffer)}let ne=O.textures;for(let de=0,he=ne.length;de<he;de++){let Ue=n.get(ne[de]);Ue.__webglTexture&&(s.deleteTexture(Ue.__webglTexture),a.memory.textures--),n.remove(ne[de])}n.remove(O)}let y=0;function N(){y=0}function D(){return y}function R(O){y=O}function U(){let O=y;return O>=i.maxTextures&&nt("WebGLTextures: Trying to use "+(O+1)+" texture units while this GPU supports only "+i.maxTextures),y+=1,O}function z(O){let A=[];return A.push(O.wrapS),A.push(O.wrapT),A.push(O.wrapR||0),A.push(O.magFilter),A.push(O.minFilter),A.push(O.anisotropy),A.push(O.internalFormat),A.push(O.format),A.push(O.type),A.push(O.generateMipmaps),A.push(O.premultiplyAlpha),A.push(O.flipY),A.push(O.unpackAlignment),A.push(O.colorSpace),A.join()}function T(O,A){let ne=n.get(O);if(O.isVideoTexture&&J(O),O.isRenderTargetTexture===!1&&O.isExternalTexture!==!0&&O.version>0&&ne.__version!==O.version){let de=O.image;if(de===null)nt("WebGLRenderer: Texture marked for update but no image data found.");else if(de.complete===!1)nt("WebGLRenderer: Texture marked for update but image is incomplete");else{Pe(ne,O,A);return}}else O.isExternalTexture&&(ne.__webglTexture=O.sourceTexture?O.sourceTexture:null);t.bindTexture(s.TEXTURE_2D,ne.__webglTexture,s.TEXTURE0+A)}function G(O,A){let ne=n.get(O);if(O.isRenderTargetTexture===!1&&O.version>0&&ne.__version!==O.version){Pe(ne,O,A);return}else O.isExternalTexture&&(ne.__webglTexture=O.sourceTexture?O.sourceTexture:null);t.bindTexture(s.TEXTURE_2D_ARRAY,ne.__webglTexture,s.TEXTURE0+A)}function X(O,A){let ne=n.get(O);if(O.isRenderTargetTexture===!1&&O.version>0&&ne.__version!==O.version){Pe(ne,O,A);return}t.bindTexture(s.TEXTURE_3D,ne.__webglTexture,s.TEXTURE0+A)}function Z(O,A){let ne=n.get(O);if(O.isCubeDepthTexture!==!0&&O.version>0&&ne.__version!==O.version){be(ne,O,A);return}t.bindTexture(s.TEXTURE_CUBE_MAP,ne.__webglTexture,s.TEXTURE0+A)}let H={[pi]:s.REPEAT,[qn]:s.CLAMP_TO_EDGE,[ur]:s.MIRRORED_REPEAT},oe={[$t]:s.NEAREST,[_l]:s.NEAREST_MIPMAP_NEAREST,[Bs]:s.NEAREST_MIPMAP_LINEAR,[jt]:s.LINEAR,[Lr]:s.LINEAR_MIPMAP_NEAREST,[oi]:s.LINEAR_MIPMAP_LINEAR},ke={[hf]:s.NEVER,[mf]:s.ALWAYS,[uf]:s.LESS,[ac]:s.LEQUAL,[df]:s.EQUAL,[oc]:s.GEQUAL,[ff]:s.GREATER,[pf]:s.NOTEQUAL};function Fe(O,A){if(A.type===kn&&e.has("OES_texture_float_linear")===!1&&(A.magFilter===jt||A.magFilter===Lr||A.magFilter===Bs||A.magFilter===oi||A.minFilter===jt||A.minFilter===Lr||A.minFilter===Bs||A.minFilter===oi)&&nt("WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),s.texParameteri(O,s.TEXTURE_WRAP_S,H[A.wrapS]),s.texParameteri(O,s.TEXTURE_WRAP_T,H[A.wrapT]),(O===s.TEXTURE_3D||O===s.TEXTURE_2D_ARRAY)&&s.texParameteri(O,s.TEXTURE_WRAP_R,H[A.wrapR]),s.texParameteri(O,s.TEXTURE_MAG_FILTER,oe[A.magFilter]),s.texParameteri(O,s.TEXTURE_MIN_FILTER,oe[A.minFilter]),A.compareFunction&&(s.texParameteri(O,s.TEXTURE_COMPARE_MODE,s.COMPARE_REF_TO_TEXTURE),s.texParameteri(O,s.TEXTURE_COMPARE_FUNC,ke[A.compareFunction])),e.has("EXT_texture_filter_anisotropic")===!0){if(A.magFilter===$t||A.minFilter!==Bs&&A.minFilter!==oi||A.type===kn&&e.has("OES_texture_float_linear")===!1)return;if(A.anisotropy>1||n.get(A).__currentAnisotropy){let ne=e.get("EXT_texture_filter_anisotropic");s.texParameterf(O,ne.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(A.anisotropy,i.getMaxAnisotropy())),n.get(A).__currentAnisotropy=A.anisotropy}}}function We(O,A){let ne=!1;O.__webglInit===void 0&&(O.__webglInit=!0,A.addEventListener("dispose",P));let de=A.source,he=p.get(de);he===void 0&&(he={},p.set(de,he));let Ue=z(A);if(Ue!==O.__cacheKey){he[Ue]===void 0&&(he[Ue]={texture:s.createTexture(),usedTimes:0},a.memory.textures++,ne=!0),he[Ue].usedTimes++;let Oe=he[O.__cacheKey];Oe!==void 0&&(he[O.__cacheKey].usedTimes--,Oe.usedTimes===0&&w(A)),O.__cacheKey=Ue,O.__webglTexture=he[Ue].texture}return ne}function fe(O,A,ne){return Math.floor(Math.floor(O/ne)/A)}function re(O,A,ne,de){let Ue=O.updateRanges;if(Ue.length===0)t.texSubImage2D(s.TEXTURE_2D,0,0,0,A.width,A.height,ne,de,A.data);else{Ue.sort((_e,pe)=>_e.start-pe.start);let Oe=0;for(let _e=1;_e<Ue.length;_e++){let pe=Ue[Oe],xe=Ue[_e],Se=pe.start+pe.count,we=fe(xe.start,A.width,4),Ie=fe(pe.start,A.width,4);xe.start<=Se+1&&we===Ie&&fe(xe.start+xe.count-1,A.width,4)===we?pe.count=Math.max(pe.count,xe.start+xe.count-pe.start):(++Oe,Ue[Oe]=xe)}Ue.length=Oe+1;let V=t.getParameter(s.UNPACK_ROW_LENGTH),j=t.getParameter(s.UNPACK_SKIP_PIXELS),me=t.getParameter(s.UNPACK_SKIP_ROWS);t.pixelStorei(s.UNPACK_ROW_LENGTH,A.width);for(let _e=0,pe=Ue.length;_e<pe;_e++){let xe=Ue[_e],Se=Math.floor(xe.start/4),we=Math.ceil(xe.count/4),Ie=Se%A.width,q=Math.floor(Se/A.width),Te=we,Me=1;t.pixelStorei(s.UNPACK_SKIP_PIXELS,Ie),t.pixelStorei(s.UNPACK_SKIP_ROWS,q),t.texSubImage2D(s.TEXTURE_2D,0,Ie,q,Te,Me,ne,de,A.data)}O.clearUpdateRanges(),t.pixelStorei(s.UNPACK_ROW_LENGTH,V),t.pixelStorei(s.UNPACK_SKIP_PIXELS,j),t.pixelStorei(s.UNPACK_SKIP_ROWS,me)}}function Pe(O,A,ne){let de=s.TEXTURE_2D;(A.isDataArrayTexture||A.isCompressedArrayTexture)&&(de=s.TEXTURE_2D_ARRAY),A.isData3DTexture&&(de=s.TEXTURE_3D);let he=We(O,A),Ue=A.source;t.bindTexture(de,O.__webglTexture,s.TEXTURE0+ne);let Oe=n.get(Ue);if(Ue.version!==Oe.__version||he===!0){if(t.activeTexture(s.TEXTURE0+ne),(typeof ImageBitmap<"u"&&A.image instanceof ImageBitmap)===!1){let Me=_t.getPrimaries(_t.workingColorSpace),ue=A.colorSpace===Yn?null:_t.getPrimaries(A.colorSpace),Re=A.colorSpace===Yn||Me===ue?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,A.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,A.premultiplyAlpha),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,Re)}t.pixelStorei(s.UNPACK_ALIGNMENT,A.unpackAlignment);let j=m(A.image,!1,i.maxTextureSize);j=dt(A,j);let me=r.convert(A.format,A.colorSpace),_e=r.convert(A.type),pe=b(A.internalFormat,me,_e,A.normalized,A.colorSpace,A.isVideoTexture);Fe(de,A);let xe,Se=A.mipmaps,we=A.isVideoTexture!==!0,Ie=Oe.__version===void 0||he===!0,q=Ue.dataReady,Te=E(A,j);if(A.isDepthTexture)pe=M(A.format===ds,A.type),Ie&&(we?t.texStorage2D(s.TEXTURE_2D,1,pe,j.width,j.height):t.texImage2D(s.TEXTURE_2D,0,pe,j.width,j.height,0,me,_e,null));else if(A.isDataTexture)if(Se.length>0){we&&Ie&&t.texStorage2D(s.TEXTURE_2D,Te,pe,Se[0].width,Se[0].height);for(let Me=0,ue=Se.length;Me<ue;Me++)xe=Se[Me],we?q&&t.texSubImage2D(s.TEXTURE_2D,Me,0,0,xe.width,xe.height,me,_e,xe.data):t.texImage2D(s.TEXTURE_2D,Me,pe,xe.width,xe.height,0,me,_e,xe.data);A.generateMipmaps=!1}else we?(Ie&&t.texStorage2D(s.TEXTURE_2D,Te,pe,j.width,j.height),q&&re(A,j,me,_e)):t.texImage2D(s.TEXTURE_2D,0,pe,j.width,j.height,0,me,_e,j.data);else if(A.isCompressedTexture)if(A.isCompressedArrayTexture){we&&Ie&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Te,pe,Se[0].width,Se[0].height,j.depth);for(let Me=0,ue=Se.length;Me<ue;Me++)if(xe=Se[Me],A.format!==Hn)if(me!==null)if(we){if(q)if(A.layerUpdates.size>0){let Re=Nh(xe.width,xe.height,A.format,A.type);for(let ye of A.layerUpdates){let je=xe.data.subarray(ye*Re/xe.data.BYTES_PER_ELEMENT,(ye+1)*Re/xe.data.BYTES_PER_ELEMENT);t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,Me,0,0,ye,xe.width,xe.height,1,me,je)}}else t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,Me,0,0,0,xe.width,xe.height,j.depth,me,xe.data)}else t.compressedTexImage3D(s.TEXTURE_2D_ARRAY,Me,pe,xe.width,xe.height,j.depth,0,xe.data,0,0);else nt("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else we?q&&t.texSubImage3D(s.TEXTURE_2D_ARRAY,Me,0,0,0,xe.width,xe.height,j.depth,me,_e,xe.data):t.texImage3D(s.TEXTURE_2D_ARRAY,Me,pe,xe.width,xe.height,j.depth,0,me,_e,xe.data);A.layerUpdates.size>0&&A.clearLayerUpdates()}else{we&&Ie&&t.texStorage2D(s.TEXTURE_2D,Te,pe,Se[0].width,Se[0].height);for(let Me=0,ue=Se.length;Me<ue;Me++)xe=Se[Me],A.format!==Hn?me!==null?we?q&&t.compressedTexSubImage2D(s.TEXTURE_2D,Me,0,0,xe.width,xe.height,me,xe.data):t.compressedTexImage2D(s.TEXTURE_2D,Me,pe,xe.width,xe.height,0,xe.data):nt("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):we?q&&t.texSubImage2D(s.TEXTURE_2D,Me,0,0,xe.width,xe.height,me,_e,xe.data):t.texImage2D(s.TEXTURE_2D,Me,pe,xe.width,xe.height,0,me,_e,xe.data)}else if(A.isDataArrayTexture)if(we){if(Ie&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Te,pe,j.width,j.height,j.depth),q)if(A.layerUpdates.size>0){let Me=Nh(j.width,j.height,A.format,A.type);for(let ue of A.layerUpdates){let Re=j.data.subarray(ue*Me/j.data.BYTES_PER_ELEMENT,(ue+1)*Me/j.data.BYTES_PER_ELEMENT);t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,ue,j.width,j.height,1,me,_e,Re)}A.clearLayerUpdates()}else t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,0,j.width,j.height,j.depth,me,_e,j.data)}else t.texImage3D(s.TEXTURE_2D_ARRAY,0,pe,j.width,j.height,j.depth,0,me,_e,j.data);else if(A.isData3DTexture)we?(Ie&&t.texStorage3D(s.TEXTURE_3D,Te,pe,j.width,j.height,j.depth),q&&t.texSubImage3D(s.TEXTURE_3D,0,0,0,0,j.width,j.height,j.depth,me,_e,j.data)):t.texImage3D(s.TEXTURE_3D,0,pe,j.width,j.height,j.depth,0,me,_e,j.data);else if(A.isFramebufferTexture){if(Ie)if(we)t.texStorage2D(s.TEXTURE_2D,Te,pe,j.width,j.height);else{let Me=j.width,ue=j.height;for(let Re=0;Re<Te;Re++)t.texImage2D(s.TEXTURE_2D,Re,pe,Me,ue,0,me,_e,null),Me>>=1,ue>>=1}}else if(A.isHTMLTexture){if("texElementImage2D"in s){let Me=s.canvas;if(Me.hasAttribute("layoutsubtree")||Me.setAttribute("layoutsubtree","true"),j.parentNode!==Me){Me.appendChild(j),u.add(A),Me.onpaint=ue=>{let Re=ue.changedElements;for(let ye of u)Re.includes(ye.image)&&(ye.needsUpdate=!0)},Me.requestPaint();return}if(s.texElementImage2D.length===3)s.texElementImage2D(s.TEXTURE_2D,s.RGBA8,j);else{let Re=s.RGBA,ye=s.RGBA,je=s.UNSIGNED_BYTE;s.texElementImage2D(s.TEXTURE_2D,0,Re,ye,je,j)}s.texParameteri(s.TEXTURE_2D,s.TEXTURE_MIN_FILTER,s.LINEAR),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_S,s.CLAMP_TO_EDGE),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_T,s.CLAMP_TO_EDGE)}}else if(Se.length>0){if(we&&Ie){let Me=ct(Se[0]);t.texStorage2D(s.TEXTURE_2D,Te,pe,Me.width,Me.height)}for(let Me=0,ue=Se.length;Me<ue;Me++)xe=Se[Me],we?q&&t.texSubImage2D(s.TEXTURE_2D,Me,0,0,me,_e,xe):t.texImage2D(s.TEXTURE_2D,Me,pe,me,_e,xe);A.generateMipmaps=!1}else if(we){if(Ie){let Me=ct(j);t.texStorage2D(s.TEXTURE_2D,Te,pe,Me.width,Me.height)}q&&t.texSubImage2D(s.TEXTURE_2D,0,0,0,me,_e,j)}else t.texImage2D(s.TEXTURE_2D,0,pe,me,_e,j);f(A)&&v(de),Oe.__version=Ue.version,A.onUpdate&&A.onUpdate(A)}O.__version=A.version}function be(O,A,ne){if(A.image.length!==6)return;let de=We(O,A),he=A.source;t.bindTexture(s.TEXTURE_CUBE_MAP,O.__webglTexture,s.TEXTURE0+ne);let Ue=n.get(he);if(he.version!==Ue.__version||de===!0){t.activeTexture(s.TEXTURE0+ne);let Oe=_t.getPrimaries(_t.workingColorSpace),V=A.colorSpace===Yn?null:_t.getPrimaries(A.colorSpace),j=A.colorSpace===Yn||Oe===V?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,A.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,A.premultiplyAlpha),t.pixelStorei(s.UNPACK_ALIGNMENT,A.unpackAlignment),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,j);let me=A.isCompressedTexture||A.image[0].isCompressedTexture,_e=A.image[0]&&A.image[0].isDataTexture,pe=[];for(let ye=0;ye<6;ye++)!me&&!_e?pe[ye]=m(A.image[ye],!0,i.maxCubemapSize):pe[ye]=_e?A.image[ye].image:A.image[ye],pe[ye]=dt(A,pe[ye]);let xe=pe[0],Se=r.convert(A.format,A.colorSpace),we=r.convert(A.type),Ie=b(A.internalFormat,Se,we,A.normalized,A.colorSpace),q=A.isVideoTexture!==!0,Te=Ue.__version===void 0||de===!0,Me=he.dataReady,ue=E(A,xe);Fe(s.TEXTURE_CUBE_MAP,A);let Re;if(me){q&&Te&&t.texStorage2D(s.TEXTURE_CUBE_MAP,ue,Ie,xe.width,xe.height);for(let ye=0;ye<6;ye++){Re=pe[ye].mipmaps;for(let je=0;je<Re.length;je++){let Ke=Re[je];A.format!==Hn?Se!==null?q?Me&&t.compressedTexSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,je,0,0,Ke.width,Ke.height,Se,Ke.data):t.compressedTexImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,je,Ie,Ke.width,Ke.height,0,Ke.data):nt("WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):q?Me&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,je,0,0,Ke.width,Ke.height,Se,we,Ke.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,je,Ie,Ke.width,Ke.height,0,Se,we,Ke.data)}}}else{if(Re=A.mipmaps,q&&Te){Re.length>0&&ue++;let ye=ct(pe[0]);t.texStorage2D(s.TEXTURE_CUBE_MAP,ue,Ie,ye.width,ye.height)}for(let ye=0;ye<6;ye++)if(_e){q?Me&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,0,0,0,pe[ye].width,pe[ye].height,Se,we,pe[ye].data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,0,Ie,pe[ye].width,pe[ye].height,0,Se,we,pe[ye].data);for(let je=0;je<Re.length;je++){let B=Re[je].image[ye].image;q?Me&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,je+1,0,0,B.width,B.height,Se,we,B.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,je+1,Ie,B.width,B.height,0,Se,we,B.data)}}else{q?Me&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,0,0,0,Se,we,pe[ye]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,0,Ie,Se,we,pe[ye]);for(let je=0;je<Re.length;je++){let Ke=Re[je];q?Me&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,je+1,0,0,Se,we,Ke.image[ye]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,je+1,Ie,Se,we,Ke.image[ye])}}}f(A)&&v(s.TEXTURE_CUBE_MAP),Ue.__version=he.version,A.onUpdate&&A.onUpdate(A)}O.__version=A.version}function ge(O,A,ne,de,he,Ue){let Oe=r.convert(ne.format,ne.colorSpace),V=r.convert(ne.type),j=b(ne.internalFormat,Oe,V,ne.normalized,ne.colorSpace),me=n.get(A),_e=n.get(ne);if(_e.__renderTarget=A,!me.__hasExternalTextures){let pe=Math.max(1,A.width>>Ue),xe=Math.max(1,A.height>>Ue);he===s.TEXTURE_3D||he===s.TEXTURE_2D_ARRAY?t.texImage3D(he,Ue,j,pe,xe,A.depth,0,Oe,V,null):t.texImage2D(he,Ue,j,pe,xe,0,Oe,V,null)}t.bindFramebuffer(s.FRAMEBUFFER,O),ot(A)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,de,he,_e.__webglTexture,0,at(A)):(he===s.TEXTURE_2D||he>=s.TEXTURE_CUBE_MAP_POSITIVE_X&&he<=s.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&s.framebufferTexture2D(s.FRAMEBUFFER,de,he,_e.__webglTexture,Ue),t.bindFramebuffer(s.FRAMEBUFFER,null)}function ie(O,A,ne){if(s.bindRenderbuffer(s.RENDERBUFFER,O),A.depthBuffer){let de=A.depthTexture,he=de&&de.isDepthTexture?de.type:null,Ue=M(A.stencilBuffer,he),Oe=A.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;ot(A)?o.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,at(A),Ue,A.width,A.height):ne?s.renderbufferStorageMultisample(s.RENDERBUFFER,at(A),Ue,A.width,A.height):s.renderbufferStorage(s.RENDERBUFFER,Ue,A.width,A.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,Oe,s.RENDERBUFFER,O)}else{let de=A.textures;for(let he=0;he<de.length;he++){let Ue=de[he],Oe=r.convert(Ue.format,Ue.colorSpace),V=r.convert(Ue.type),j=b(Ue.internalFormat,Oe,V,Ue.normalized,Ue.colorSpace);ot(A)?o.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,at(A),j,A.width,A.height):ne?s.renderbufferStorageMultisample(s.RENDERBUFFER,at(A),j,A.width,A.height):s.renderbufferStorage(s.RENDERBUFFER,j,A.width,A.height)}}s.bindRenderbuffer(s.RENDERBUFFER,null)}function le(O,A,ne){let de=A.isWebGLCubeRenderTarget===!0;if(t.bindFramebuffer(s.FRAMEBUFFER,O),!(A.depthTexture&&A.depthTexture.isDepthTexture))throw new Error("THREE.WebGLTextures: renderTarget.depthTexture must be an instance of THREE.DepthTexture.");let he=n.get(A.depthTexture);if(he.__renderTarget=A,(!he.__webglTexture||A.depthTexture.image.width!==A.width||A.depthTexture.image.height!==A.height)&&(A.depthTexture.image.width=A.width,A.depthTexture.image.height=A.height,A.depthTexture.needsUpdate=!0),de){if(he.__webglInit===void 0&&(he.__webglInit=!0,A.depthTexture.addEventListener("dispose",P)),he.__webglTexture===void 0){he.__webglTexture=s.createTexture(),t.bindTexture(s.TEXTURE_CUBE_MAP,he.__webglTexture),Fe(s.TEXTURE_CUBE_MAP,A.depthTexture);let me=r.convert(A.depthTexture.format),_e=r.convert(A.depthTexture.type),pe;A.depthTexture.format===mi?pe=s.DEPTH_COMPONENT24:A.depthTexture.format===ds&&(pe=s.DEPTH24_STENCIL8);for(let xe=0;xe<6;xe++)s.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+xe,0,pe,A.width,A.height,0,me,_e,null)}}else T(A.depthTexture,0);let Ue=he.__webglTexture,Oe=at(A),V=de?s.TEXTURE_CUBE_MAP_POSITIVE_X+ne:s.TEXTURE_2D,j=A.depthTexture.format===ds?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;if(A.depthTexture.format===mi)ot(A)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,j,V,Ue,0,Oe):s.framebufferTexture2D(s.FRAMEBUFFER,j,V,Ue,0);else if(A.depthTexture.format===ds)ot(A)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,j,V,Ue,0,Oe):s.framebufferTexture2D(s.FRAMEBUFFER,j,V,Ue,0);else throw new Error("THREE.WebGLTextures: Unknown depthTexture format.")}function se(O){let A=n.get(O),ne=O.isWebGLCubeRenderTarget===!0;if(A.__boundDepthTexture!==O.depthTexture){let de=O.depthTexture;if(A.__depthDisposeCallback&&A.__depthDisposeCallback(),de){let he=()=>{delete A.__boundDepthTexture,delete A.__depthDisposeCallback,de.removeEventListener("dispose",he)};de.addEventListener("dispose",he),A.__depthDisposeCallback=he}A.__boundDepthTexture=de}if(O.depthTexture&&!A.__autoAllocateDepthBuffer)if(ne)for(let de=0;de<6;de++)le(A.__webglFramebuffer[de],O,de);else{let de=O.texture.mipmaps;de&&de.length>0?le(A.__webglFramebuffer[0],O,0):le(A.__webglFramebuffer,O,0)}else if(ne){A.__webglDepthbuffer=[];for(let de=0;de<6;de++)if(t.bindFramebuffer(s.FRAMEBUFFER,A.__webglFramebuffer[de]),A.__webglDepthbuffer[de]===void 0)A.__webglDepthbuffer[de]=s.createRenderbuffer(),ie(A.__webglDepthbuffer[de],O,!1);else{let he=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Ue=A.__webglDepthbuffer[de];s.bindRenderbuffer(s.RENDERBUFFER,Ue),s.framebufferRenderbuffer(s.FRAMEBUFFER,he,s.RENDERBUFFER,Ue)}}else{let de=O.texture.mipmaps;if(de&&de.length>0?t.bindFramebuffer(s.FRAMEBUFFER,A.__webglFramebuffer[0]):t.bindFramebuffer(s.FRAMEBUFFER,A.__webglFramebuffer),A.__webglDepthbuffer===void 0)A.__webglDepthbuffer=s.createRenderbuffer(),ie(A.__webglDepthbuffer,O,!1);else{let he=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Ue=A.__webglDepthbuffer;s.bindRenderbuffer(s.RENDERBUFFER,Ue),s.framebufferRenderbuffer(s.FRAMEBUFFER,he,s.RENDERBUFFER,Ue)}}t.bindFramebuffer(s.FRAMEBUFFER,null)}function ce(O,A,ne){let de=n.get(O);A!==void 0&&ge(de.__webglFramebuffer,O,O.texture,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,0),ne!==void 0&&se(O)}function ve(O){let A=O.texture,ne=n.get(O),de=n.get(A);O.addEventListener("dispose",x);let he=O.textures,Ue=O.isWebGLCubeRenderTarget===!0,Oe=he.length>1;if(Oe||(de.__webglTexture===void 0&&(de.__webglTexture=s.createTexture()),de.__version=A.version,a.memory.textures++),Ue){ne.__webglFramebuffer=[];for(let V=0;V<6;V++)if(A.mipmaps&&A.mipmaps.length>0){ne.__webglFramebuffer[V]=[];for(let j=0;j<A.mipmaps.length;j++)ne.__webglFramebuffer[V][j]=s.createFramebuffer()}else ne.__webglFramebuffer[V]=s.createFramebuffer()}else{if(A.mipmaps&&A.mipmaps.length>0){ne.__webglFramebuffer=[];for(let V=0;V<A.mipmaps.length;V++)ne.__webglFramebuffer[V]=s.createFramebuffer()}else ne.__webglFramebuffer=s.createFramebuffer();if(Oe)for(let V=0,j=he.length;V<j;V++){let me=n.get(he[V]);me.__webglTexture===void 0&&(me.__webglTexture=s.createTexture(),a.memory.textures++)}if(O.samples>0&&ot(O)===!1){ne.__webglMultisampledFramebuffer=s.createFramebuffer(),ne.__webglColorRenderbuffer=[],t.bindFramebuffer(s.FRAMEBUFFER,ne.__webglMultisampledFramebuffer);for(let V=0;V<he.length;V++){let j=he[V];ne.__webglColorRenderbuffer[V]=s.createRenderbuffer(),s.bindRenderbuffer(s.RENDERBUFFER,ne.__webglColorRenderbuffer[V]);let me=r.convert(j.format,j.colorSpace),_e=r.convert(j.type),pe=b(j.internalFormat,me,_e,j.normalized,j.colorSpace,O.isXRRenderTarget===!0),xe=at(O);s.renderbufferStorageMultisample(s.RENDERBUFFER,xe,pe,O.width,O.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+V,s.RENDERBUFFER,ne.__webglColorRenderbuffer[V])}s.bindRenderbuffer(s.RENDERBUFFER,null),O.depthBuffer&&(ne.__webglDepthRenderbuffer=s.createRenderbuffer(),ie(ne.__webglDepthRenderbuffer,O,!0)),t.bindFramebuffer(s.FRAMEBUFFER,null)}}if(Ue){t.bindTexture(s.TEXTURE_CUBE_MAP,de.__webglTexture),Fe(s.TEXTURE_CUBE_MAP,A);for(let V=0;V<6;V++)if(A.mipmaps&&A.mipmaps.length>0)for(let j=0;j<A.mipmaps.length;j++)ge(ne.__webglFramebuffer[V][j],O,A,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+V,j);else ge(ne.__webglFramebuffer[V],O,A,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+V,0);f(A)&&v(s.TEXTURE_CUBE_MAP),t.unbindTexture()}else if(Oe){for(let V=0,j=he.length;V<j;V++){let me=he[V],_e=n.get(me),pe=s.TEXTURE_2D;(O.isWebGL3DRenderTarget||O.isWebGLArrayRenderTarget)&&(pe=O.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(pe,_e.__webglTexture),Fe(pe,me),ge(ne.__webglFramebuffer,O,me,s.COLOR_ATTACHMENT0+V,pe,0),f(me)&&v(pe)}t.unbindTexture()}else{let V=s.TEXTURE_2D;if((O.isWebGL3DRenderTarget||O.isWebGLArrayRenderTarget)&&(V=O.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(V,de.__webglTexture),Fe(V,A),A.mipmaps&&A.mipmaps.length>0)for(let j=0;j<A.mipmaps.length;j++)ge(ne.__webglFramebuffer[j],O,A,s.COLOR_ATTACHMENT0,V,j);else ge(ne.__webglFramebuffer,O,A,s.COLOR_ATTACHMENT0,V,0);f(A)&&v(V),t.unbindTexture()}O.depthBuffer&&se(O)}function Ee(O){let A=O.textures;for(let ne=0,de=A.length;ne<de;ne++){let he=A[ne];if(f(he)){let Ue=S(O),Oe=n.get(he).__webglTexture;t.bindTexture(Ue,Oe),v(Ue),t.unbindTexture()}}}let qe=[],Ye=[];function st(O){if(O.samples>0){if(ot(O)===!1){let A=O.textures,ne=O.width,de=O.height,he=s.COLOR_BUFFER_BIT,Ue=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Oe=n.get(O),V=A.length>1;if(V)for(let me=0;me<A.length;me++)t.bindFramebuffer(s.FRAMEBUFFER,Oe.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+me,s.RENDERBUFFER,null),t.bindFramebuffer(s.FRAMEBUFFER,Oe.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+me,s.TEXTURE_2D,null,0);t.bindFramebuffer(s.READ_FRAMEBUFFER,Oe.__webglMultisampledFramebuffer);let j=O.texture.mipmaps;j&&j.length>0?t.bindFramebuffer(s.DRAW_FRAMEBUFFER,Oe.__webglFramebuffer[0]):t.bindFramebuffer(s.DRAW_FRAMEBUFFER,Oe.__webglFramebuffer);for(let me=0;me<A.length;me++){if(O.resolveDepthBuffer&&(O.depthBuffer&&(he|=s.DEPTH_BUFFER_BIT),O.stencilBuffer&&O.resolveStencilBuffer&&(he|=s.STENCIL_BUFFER_BIT)),V){s.framebufferRenderbuffer(s.READ_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.RENDERBUFFER,Oe.__webglColorRenderbuffer[me]);let _e=n.get(A[me]).__webglTexture;s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,_e,0)}s.blitFramebuffer(0,0,ne,de,0,0,ne,de,he,s.NEAREST),l===!0&&(qe.length=0,Ye.length=0,qe.push(s.COLOR_ATTACHMENT0+me),O.depthBuffer&&O.storeMultisampledDepthBuffer===!1&&(qe.push(Ue),Ye.push(Ue),s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,Ye)),s.invalidateFramebuffer(s.READ_FRAMEBUFFER,qe))}if(t.bindFramebuffer(s.READ_FRAMEBUFFER,null),t.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),V)for(let me=0;me<A.length;me++){t.bindFramebuffer(s.FRAMEBUFFER,Oe.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+me,s.RENDERBUFFER,Oe.__webglColorRenderbuffer[me]);let _e=n.get(A[me]).__webglTexture;t.bindFramebuffer(s.FRAMEBUFFER,Oe.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+me,s.TEXTURE_2D,_e,0)}t.bindFramebuffer(s.DRAW_FRAMEBUFFER,Oe.__webglMultisampledFramebuffer)}else if(O.depthBuffer&&O.storeMultisampledDepthBuffer===!1&&l){let A=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,[A])}}}function at(O){return Math.min(i.maxSamples,O.samples)}function ot(O){let A=n.get(O);return O.samples>0&&e.has("WEBGL_multisampled_render_to_texture")===!0&&A.__useRenderToTexture!==!1}function J(O){let A=a.render.frame;h.get(O)!==A&&(h.set(O,A),O.update())}function dt(O,A){let ne=O.colorSpace,de=O.format,he=O.type;return O.isCompressedTexture===!0||O.isVideoTexture===!0||ne!==wn&&ne!==Yn&&(_t.getTransfer(ne)===Dt?(de!==Hn||he!==In)&&nt("WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):ht("WebGLTextures: Unsupported texture color space:",ne)),A}function ct(O){return typeof HTMLImageElement<"u"&&O instanceof HTMLImageElement?(c.width=O.naturalWidth||O.width,c.height=O.naturalHeight||O.height):typeof VideoFrame<"u"&&O instanceof VideoFrame?(c.width=O.displayWidth,c.height=O.displayHeight):(c.width=O.width,c.height=O.height),c}this.allocateTextureUnit=U,this.resetTextureUnits=N,this.getTextureUnits=D,this.setTextureUnits=R,this.setTexture2D=T,this.setTexture2DArray=G,this.setTexture3D=X,this.setTextureCube=Z,this.rebindTextures=ce,this.setupRenderTarget=ve,this.updateRenderTargetMipmap=Ee,this.updateMultisampleRenderTarget=st,this.setupDepthRenderbuffer=se,this.setupFrameBufferTexture=ge,this.useMultisampledRTT=ot,this.isReversedDepthBuffer=function(){return t.buffers.depth.getReversed()}}function Jv(s,e){function t(n,i=Yn){let r,a=_t.getTransfer(i);if(n===In)return s.UNSIGNED_BYTE;if(n===Ml)return s.UNSIGNED_SHORT_4_4_4_4;if(n===Sl)return s.UNSIGNED_SHORT_5_5_5_1;if(n===Sh)return s.UNSIGNED_INT_5_9_9_9_REV;if(n===wh)return s.UNSIGNED_INT_10F_11F_11F_REV;if(n===yh)return s.BYTE;if(n===Mh)return s.SHORT;if(n===Fr)return s.UNSIGNED_SHORT;if(n===yl)return s.INT;if(n===li)return s.UNSIGNED_INT;if(n===kn)return s.FLOAT;if(n===rn)return s.HALF_FLOAT;if(n===Eh)return s.ALPHA;if(n===Th)return s.RGB;if(n===Hn)return s.RGBA;if(n===mi)return s.DEPTH_COMPONENT;if(n===ds)return s.DEPTH_STENCIL;if(n===wl)return s.RED;if(n===El)return s.RED_INTEGER;if(n===fs)return s.RG;if(n===Tl)return s.RG_INTEGER;if(n===Al)return s.RGBA_INTEGER;if(n===Va||n===Wa||n===qa||n===Xa)if(a===Dt)if(r=e.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(n===Va)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(n===Wa)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(n===qa)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(n===Xa)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=e.get("WEBGL_compressed_texture_s3tc"),r!==null){if(n===Va)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(n===Wa)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(n===qa)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(n===Xa)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(n===Rl||n===Cl||n===Pl||n===Il)if(r=e.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(n===Rl)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(n===Cl)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(n===Pl)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(n===Il)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(n===Dl||n===Ll||n===Fl||n===Nl||n===Ul||n===ja||n===Ol)if(r=e.get("WEBGL_compressed_texture_etc"),r!==null){if(n===Dl||n===Ll)return a===Dt?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(n===Fl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC;if(n===Nl)return r.COMPRESSED_R11_EAC;if(n===Ul)return r.COMPRESSED_SIGNED_R11_EAC;if(n===ja)return r.COMPRESSED_RG11_EAC;if(n===Ol)return r.COMPRESSED_SIGNED_RG11_EAC}else return null;if(n===zl||n===Bl||n===kl||n===Hl||n===Gl||n===Vl||n===Wl||n===ql||n===Xl||n===jl||n===Kl||n===Yl||n===Jl||n===Zl)if(r=e.get("WEBGL_compressed_texture_astc"),r!==null){if(n===zl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(n===Bl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(n===kl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(n===Hl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(n===Gl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(n===Vl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(n===Wl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(n===ql)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(n===Xl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(n===jl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(n===Kl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(n===Yl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(n===Jl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(n===Zl)return a===Dt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(n===$l||n===Ql||n===ec)if(r=e.get("EXT_texture_compression_bptc"),r!==null){if(n===$l)return a===Dt?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(n===Ql)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(n===ec)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(n===tc||n===nc||n===Ka||n===ic)if(r=e.get("EXT_texture_compression_rgtc"),r!==null){if(n===tc)return r.COMPRESSED_RED_RGTC1_EXT;if(n===nc)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(n===Ka)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(n===ic)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return n===Nr?s.UNSIGNED_INT_24_8:s[n]!==void 0?s[n]:null}return{convert:t}}var Zv=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,$v=`
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

}`,Yh=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(e,t){if(this.texture===null){let n=new va(e.texture);(e.depthNear!==t.depthNear||e.depthFar!==t.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=n}}getMesh(e){if(this.texture!==null&&this.mesh===null){let t=e.cameras[0].viewport,n=new vt({vertexShader:Zv,fragmentShader:$v,uniforms:{depthColor:{value:this.texture},depthWidth:{value:t.z},depthHeight:{value:t.w}}});this.mesh=new it(new pn(20,20),n)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},Jh=class extends Xn{constructor(e,t){super();let n=this,i=null,r=1,a=null,o="local-floor",l=1,c=null,h=null,u=null,d=null,p=null,g=null,_=typeof XRWebGLBinding<"u",m=new Yh,f={},v=t.getContextAttributes(),S=null,b=null,M=[],E=[],P=new Be,x=null,C=null,w=new sn;w.viewport=new Pt;let I=new sn;I.viewport=new Pt;let y=[w,I],N=new pl,D=null,R=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(fe){let re=M[fe];return re===void 0&&(re=new br,M[fe]=re),re.getTargetRaySpace()},this.getControllerGrip=function(fe){let re=M[fe];return re===void 0&&(re=new br,M[fe]=re),re.getGripSpace()},this.getHand=function(fe){let re=M[fe];return re===void 0&&(re=new br,M[fe]=re),re.getHandSpace()};function U(fe){let re=E.indexOf(fe.inputSource);if(re===-1)return;let Pe=M[re];Pe!==void 0&&(Pe.update(fe.inputSource,fe.frame,c||a),Pe.dispatchEvent({type:fe.type,data:fe.inputSource}))}function z(){i.removeEventListener("select",U),i.removeEventListener("selectstart",U),i.removeEventListener("selectend",U),i.removeEventListener("squeeze",U),i.removeEventListener("squeezestart",U),i.removeEventListener("squeezeend",U),i.removeEventListener("end",z),i.removeEventListener("inputsourceschange",T);for(let fe=0;fe<M.length;fe++){let re=E[fe];re!==null&&(E[fe]=null,M[fe].disconnect(re))}D=null,R=null,m.reset();for(let fe in f)delete f[fe];if(e.setRenderTarget(S),p=null,d=null,u=null,i=null,b=null,We.stop(),n.isPresenting=!1,e.setPixelRatio(x),e.setSize(P.width,P.height,!1),C!==null){let fe=C.camera;fe.fov=C.fov,fe.zoom=C.zoom,fe.updateProjectionMatrix(),C=null}n.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(fe){r=fe,n.isPresenting===!0&&nt("WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(fe){o=fe,n.isPresenting===!0&&nt("WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return c||a},this.setReferenceSpace=function(fe){c=fe},this.getBaseLayer=function(){return d!==null?d:p},this.getBinding=function(){return u===null&&_&&(u=new XRWebGLBinding(i,t)),u},this.getFrame=function(){return g},this.getSession=function(){return i},this.setSession=async function(fe){if(i=fe,i!==null){if(S=e.getRenderTarget(),i.addEventListener("select",U),i.addEventListener("selectstart",U),i.addEventListener("selectend",U),i.addEventListener("squeeze",U),i.addEventListener("squeezestart",U),i.addEventListener("squeezeend",U),i.addEventListener("end",z),i.addEventListener("inputsourceschange",T),v.xrCompatible!==!0&&await t.makeXRCompatible(),x=e.getPixelRatio(),e.getSize(P),_&&"createProjectionLayer"in XRWebGLBinding.prototype){let Pe=null,be=null,ge=null;v.depth&&(ge=v.stencil?t.DEPTH24_STENCIL8:t.DEPTH_COMPONENT24,Pe=v.stencil?ds:mi,be=v.stencil?Nr:li);let ie={colorFormat:t.RGBA8,depthFormat:ge,scaleFactor:r};u=this.getBinding(),d=u.createProjectionLayer(ie),i.updateRenderState({layers:[d]}),e.setPixelRatio(1),e.setSize(d.textureWidth,d.textureHeight,!1),b=new Kt(d.textureWidth,d.textureHeight,{format:Hn,type:In,depthTexture:new rs(d.textureWidth,d.textureHeight,be,void 0,void 0,void 0,void 0,void 0,void 0,Pe),stencilBuffer:v.stencil,colorSpace:e.outputColorSpace,samples:v.antialias?4:0,resolveDepthBuffer:d.ignoreDepthValues===!1,resolveStencilBuffer:d.ignoreDepthValues===!1,storeMultisampledDepthBuffer:d.ignoreDepthValues===!1,storeMultisampledStencilBuffer:d.ignoreDepthValues===!1})}else{let Pe={antialias:v.antialias,alpha:!0,depth:v.depth,stencil:v.stencil,framebufferScaleFactor:r};p=new XRWebGLLayer(i,t,Pe),i.updateRenderState({baseLayer:p}),e.setPixelRatio(1),e.setSize(p.framebufferWidth,p.framebufferHeight,!1),b=new Kt(p.framebufferWidth,p.framebufferHeight,{format:Hn,type:In,colorSpace:e.outputColorSpace,stencilBuffer:v.stencil,resolveDepthBuffer:p.ignoreDepthValues===!1,resolveStencilBuffer:p.ignoreDepthValues===!1,storeMultisampledDepthBuffer:p.ignoreDepthValues===!1,storeMultisampledStencilBuffer:p.ignoreDepthValues===!1})}b.isXRRenderTarget=!0,this.setFoveation(l),c=null,a=await i.requestReferenceSpace(o),We.setContext(i),We.start(),n.isPresenting=!0,n.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(i!==null)return i.environmentBlendMode},this.getDepthTexture=function(){return m.getDepthTexture()};function T(fe){for(let re=0;re<fe.removed.length;re++){let Pe=fe.removed[re],be=E.indexOf(Pe);be>=0&&(E[be]=null,M[be].disconnect(Pe))}for(let re=0;re<fe.added.length;re++){let Pe=fe.added[re],be=E.indexOf(Pe);if(be===-1){for(let ie=0;ie<M.length;ie++)if(ie>=E.length){E.push(Pe),be=ie;break}else if(E[ie]===null){E[ie]=Pe,be=ie;break}if(be===-1)break}let ge=M[be];ge&&ge.connect(Pe)}}let G=new k,X=new k;function Z(fe,re,Pe){G.setFromMatrixPosition(re.matrixWorld),X.setFromMatrixPosition(Pe.matrixWorld);let be=G.distanceTo(X),ge=re.projectionMatrix.elements,ie=Pe.projectionMatrix.elements,le=ge[14]/(ge[10]-1),se=ge[14]/(ge[10]+1),ce=(ge[9]+1)/ge[5],ve=(ge[9]-1)/ge[5],Ee=(ge[8]-1)/ge[0],qe=(ie[8]+1)/ie[0],Ye=le*Ee,st=le*qe,at=be/(-Ee+qe),ot=at*-Ee;if(re.matrixWorld.decompose(fe.position,fe.quaternion,fe.scale),fe.translateX(ot),fe.translateZ(at),fe.matrixWorld.compose(fe.position,fe.quaternion,fe.scale),fe.matrixWorldInverse.copy(fe.matrixWorld).invert(),ge[10]===-1)fe.projectionMatrix.copy(re.projectionMatrix),fe.projectionMatrixInverse.copy(re.projectionMatrixInverse);else{let J=le+at,dt=se+at,ct=Ye-ot,O=st+(be-ot),A=ce*se/dt*J,ne=ve*se/dt*J;fe.projectionMatrix.makePerspective(ct,O,A,ne,J,dt),fe.projectionMatrixInverse.copy(fe.projectionMatrix).invert()}}function H(fe,re){re===null?fe.matrixWorld.copy(fe.matrix):fe.matrixWorld.multiplyMatrices(re.matrixWorld,fe.matrix),fe.matrixWorldInverse.copy(fe.matrixWorld).invert()}this.updateCamera=function(fe){if(i===null)return;let re=fe.near,Pe=fe.far;m.texture!==null&&(m.depthNear>0&&(re=m.depthNear),m.depthFar>0&&(Pe=m.depthFar)),N.near=I.near=w.near=re,N.far=I.far=w.far=Pe,(D!==N.near||R!==N.far)&&(i.updateRenderState({depthNear:N.near,depthFar:N.far}),D=N.near,R=N.far),N.layers.mask=fe.layers.mask|6,w.layers.mask=N.layers.mask&-5,I.layers.mask=N.layers.mask&-3;let be=fe.parent,ge=N.cameras;H(N,be);for(let ie=0;ie<ge.length;ie++)H(ge[ie],be);ge.length===2?Z(N,w,I):N.projectionMatrix.copy(w.projectionMatrix),C===null&&fe.isPerspectiveCamera&&(C={camera:fe,fov:fe.fov,zoom:fe.zoom}),oe(fe,N,be)};function oe(fe,re,Pe){Pe===null?fe.matrix.copy(re.matrixWorld):(fe.matrix.copy(Pe.matrixWorld),fe.matrix.invert(),fe.matrix.multiply(re.matrixWorld)),fe.matrix.decompose(fe.position,fe.quaternion,fe.scale),fe.updateMatrixWorld(!0),fe.projectionMatrix.copy(re.projectionMatrix),fe.projectionMatrixInverse.copy(re.projectionMatrixInverse),fe.isPerspectiveCamera&&(fe.fov=Rs*2*Math.atan(1/fe.projectionMatrix.elements[5]),fe.zoom=1)}this.getCamera=function(){return N},this.getFoveation=function(){if(!(d===null&&p===null))return l},this.setFoveation=function(fe){l=fe,d!==null&&(d.fixedFoveation=fe),p!==null&&p.fixedFoveation!==void 0&&(p.fixedFoveation=fe)},this.hasDepthSensing=function(){return m.texture!==null},this.getDepthSensingMesh=function(){return m.getMesh(N)},this.getCameraTexture=function(fe){return f[fe]};let ke=null;function Fe(fe,re){if(h=re.getViewerPose(c||a),g=re,h!==null){let Pe=h.views;p!==null&&(e.setRenderTargetFramebuffer(b,p.framebuffer),e.setRenderTarget(b));let be=!1;Pe.length!==N.cameras.length&&(N.cameras.length=0,be=!0);for(let se=0;se<Pe.length;se++){let ce=Pe[se],ve=null;if(p!==null)ve=p.getViewport(ce);else{let qe=u.getViewSubImage(d,ce);ve=qe.viewport,se===0&&(e.setRenderTargetTextures(b,qe.colorTexture,qe.depthStencilTexture),e.setRenderTarget(b))}let Ee=y[se];Ee===void 0&&(Ee=new sn,Ee.layers.enable(se),Ee.viewport=new Pt,y[se]=Ee),Ee.matrix.fromArray(ce.transform.matrix),Ee.matrix.decompose(Ee.position,Ee.quaternion,Ee.scale),Ee.projectionMatrix.fromArray(ce.projectionMatrix),Ee.projectionMatrixInverse.copy(Ee.projectionMatrix).invert(),Ee.viewport.set(ve.x,ve.y,ve.width,ve.height),se===0&&(N.matrix.copy(Ee.matrix),N.matrix.decompose(N.position,N.quaternion,N.scale)),be===!0&&N.cameras.push(Ee)}let ge=i.enabledFeatures;if(ge&&ge.includes("depth-sensing")&&i.depthUsage=="gpu-optimized"&&_){u=n.getBinding();let se=u.getDepthInformation(Pe[0]);se&&se.isValid&&se.texture&&m.init(se,i.renderState)}if(ge&&ge.includes("camera-access")&&_){e.state.unbindTexture(),u=n.getBinding();for(let se=0;se<Pe.length;se++){let ce=Pe[se].camera;if(ce){let ve=f[ce];ve||(ve=new va,f[ce]=ve);let Ee=u.getCameraImage(ce);ve.sourceTexture=Ee}}}}for(let Pe=0;Pe<M.length;Pe++){let be=E[Pe],ge=M[Pe];be!==null&&ge!==void 0&&ge.update(be,re,c||a)}ke&&ke(fe,re),re.detectedPlanes&&n.dispatchEvent({type:"planesdetected",data:re}),g=null}let We=new Wf;We.setAnimationLoop(Fe),this.setAnimationLoop=function(fe){ke=fe},this.dispose=function(){}}},Qv=new pt,Jf=new mt;Jf.set(-1,0,0,0,1,0,0,0,1);function e_(s,e){function t(m,f){m.matrixAutoUpdate===!0&&m.updateMatrix(),f.value.copy(m.matrix)}function n(m,f){f.color.getRGB(m.fogColor.value,Dh(s)),f.isFog?(m.fogNear.value=f.near,m.fogFar.value=f.far):f.isFogExp2&&(m.fogDensity.value=f.density)}function i(m,f,v,S,b){f.isNodeMaterial?f.uniformsNeedUpdate=!1:f.isMeshBasicMaterial?r(m,f):f.isMeshLambertMaterial?(r(m,f),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)):f.isMeshToonMaterial?(r(m,f),u(m,f)):f.isMeshPhongMaterial?(r(m,f),h(m,f),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)):f.isMeshStandardMaterial?(r(m,f),d(m,f),f.isMeshPhysicalMaterial&&p(m,f,b)):f.isMeshMatcapMaterial?(r(m,f),g(m,f)):f.isMeshDepthMaterial?r(m,f):f.isMeshDistanceMaterial?(r(m,f),_(m,f)):f.isMeshNormalMaterial?r(m,f):f.isLineBasicMaterial?(a(m,f),f.isLineDashedMaterial&&o(m,f)):f.isPointsMaterial?l(m,f,v,S):f.isSpriteMaterial?c(m,f):f.isShadowMaterial?(m.color.value.copy(f.color),m.opacity.value=f.opacity):f.isShaderMaterial&&(f.uniformsNeedUpdate=!1)}function r(m,f){m.opacity.value=f.opacity,f.color&&m.diffuse.value.copy(f.color),f.emissive&&m.emissive.value.copy(f.emissive).multiplyScalar(f.emissiveIntensity),f.map&&(m.map.value=f.map,t(f.map,m.mapTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.bumpMap&&(m.bumpMap.value=f.bumpMap,t(f.bumpMap,m.bumpMapTransform),m.bumpScale.value=f.bumpScale,f.side===on&&(m.bumpScale.value*=-1)),f.normalMap&&(m.normalMap.value=f.normalMap,t(f.normalMap,m.normalMapTransform),m.normalScale.value.copy(f.normalScale),f.side===on&&m.normalScale.value.negate()),f.displacementMap&&(m.displacementMap.value=f.displacementMap,t(f.displacementMap,m.displacementMapTransform),m.displacementScale.value=f.displacementScale,m.displacementBias.value=f.displacementBias),f.emissiveMap&&(m.emissiveMap.value=f.emissiveMap,t(f.emissiveMap,m.emissiveMapTransform)),f.specularMap&&(m.specularMap.value=f.specularMap,t(f.specularMap,m.specularMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest);let v=e.get(f),S=v.envMap,b=v.envMapRotation;S&&(m.envMap.value=S,m.envMapRotation.value.setFromMatrix4(Qv.makeRotationFromEuler(b)).transpose(),S.isCubeTexture&&S.isRenderTargetTexture===!1&&m.envMapRotation.value.premultiply(Jf),m.reflectivity.value=f.reflectivity,m.ior.value=f.ior,m.refractionRatio.value=f.refractionRatio),f.lightMap&&(m.lightMap.value=f.lightMap,m.lightMapIntensity.value=f.lightMapIntensity,t(f.lightMap,m.lightMapTransform)),f.aoMap&&(m.aoMap.value=f.aoMap,m.aoMapIntensity.value=f.aoMapIntensity,t(f.aoMap,m.aoMapTransform))}function a(m,f){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,f.map&&(m.map.value=f.map,t(f.map,m.mapTransform))}function o(m,f){m.dashSize.value=f.dashSize,m.totalSize.value=f.dashSize+f.gapSize,m.scale.value=f.scale}function l(m,f,v,S){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,m.size.value=f.size*v,m.scale.value=S*.5,f.map&&(m.map.value=f.map,t(f.map,m.uvTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest)}function c(m,f){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,m.rotation.value=f.rotation,f.map&&(m.map.value=f.map,t(f.map,m.mapTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest)}function h(m,f){m.specular.value.copy(f.specular),m.shininess.value=Math.max(f.shininess,1e-4)}function u(m,f){f.gradientMap&&(m.gradientMap.value=f.gradientMap)}function d(m,f){m.metalness.value=f.metalness,f.metalnessMap&&(m.metalnessMap.value=f.metalnessMap,t(f.metalnessMap,m.metalnessMapTransform)),m.roughness.value=f.roughness,f.roughnessMap&&(m.roughnessMap.value=f.roughnessMap,t(f.roughnessMap,m.roughnessMapTransform)),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)}function p(m,f,v){m.ior.value=f.ior,f.sheen>0&&(m.sheenColor.value.copy(f.sheenColor).multiplyScalar(f.sheen),m.sheenRoughness.value=f.sheenRoughness,f.sheenColorMap&&(m.sheenColorMap.value=f.sheenColorMap,t(f.sheenColorMap,m.sheenColorMapTransform)),f.sheenRoughnessMap&&(m.sheenRoughnessMap.value=f.sheenRoughnessMap,t(f.sheenRoughnessMap,m.sheenRoughnessMapTransform))),f.clearcoat>0&&(m.clearcoat.value=f.clearcoat,m.clearcoatRoughness.value=f.clearcoatRoughness,f.clearcoatMap&&(m.clearcoatMap.value=f.clearcoatMap,t(f.clearcoatMap,m.clearcoatMapTransform)),f.clearcoatRoughnessMap&&(m.clearcoatRoughnessMap.value=f.clearcoatRoughnessMap,t(f.clearcoatRoughnessMap,m.clearcoatRoughnessMapTransform)),f.clearcoatNormalMap&&(m.clearcoatNormalMap.value=f.clearcoatNormalMap,t(f.clearcoatNormalMap,m.clearcoatNormalMapTransform),m.clearcoatNormalScale.value.copy(f.clearcoatNormalScale),f.side===on&&m.clearcoatNormalScale.value.negate())),f.dispersion>0&&(m.dispersion.value=f.dispersion),f.retroreflectivity>0&&(m.retroreflectivity.value=f.retroreflectivity),f.iridescence>0&&(m.iridescence.value=f.iridescence,m.iridescenceIOR.value=f.iridescenceIOR,m.iridescenceThicknessMinimum.value=f.iridescenceThicknessRange[0],m.iridescenceThicknessMaximum.value=f.iridescenceThicknessRange[1],f.iridescenceMap&&(m.iridescenceMap.value=f.iridescenceMap,t(f.iridescenceMap,m.iridescenceMapTransform)),f.iridescenceThicknessMap&&(m.iridescenceThicknessMap.value=f.iridescenceThicknessMap,t(f.iridescenceThicknessMap,m.iridescenceThicknessMapTransform))),f.transmission>0&&(m.transmission.value=f.transmission,m.transmissionSamplerMap.value=v.texture,m.transmissionSamplerSize.value.set(v.width,v.height),f.transmissionMap&&(m.transmissionMap.value=f.transmissionMap,t(f.transmissionMap,m.transmissionMapTransform)),m.thickness.value=f.thickness,f.thicknessMap&&(m.thicknessMap.value=f.thicknessMap,t(f.thicknessMap,m.thicknessMapTransform)),m.attenuationDistance.value=f.attenuationDistance,m.attenuationColor.value.copy(f.attenuationColor)),f.anisotropy>0&&(m.anisotropyVector.value.set(f.anisotropy*Math.cos(f.anisotropyRotation),f.anisotropy*Math.sin(f.anisotropyRotation)),f.anisotropyMap&&(m.anisotropyMap.value=f.anisotropyMap,t(f.anisotropyMap,m.anisotropyMapTransform))),m.specularIntensity.value=f.specularIntensity,m.specularColor.value.copy(f.specularColor),f.specularColorMap&&(m.specularColorMap.value=f.specularColorMap,t(f.specularColorMap,m.specularColorMapTransform)),f.specularIntensityMap&&(m.specularIntensityMap.value=f.specularIntensityMap,t(f.specularIntensityMap,m.specularIntensityMapTransform))}function g(m,f){f.matcap&&(m.matcap.value=f.matcap)}function _(m,f){let v=e.get(f).light;m.referencePosition.value.setFromMatrixPosition(v.matrixWorld),m.nearDistance.value=v.shadow.camera.near,m.farDistance.value=v.shadow.camera.far}return{refreshFogUniforms:n,refreshMaterialUniforms:i}}function t_(s,e,t,n){let i={},r={},a=[],o=s.getParameter(s.MAX_UNIFORM_BUFFER_BINDINGS);function l(b,M){let E=M.program;n.uniformBlockBinding(b,E)}function c(b,M){let E=i[b.id];E===void 0&&(m(b),E=h(b),i[b.id]=E,b.addEventListener("dispose",v));let P=M.program;n.updateUBOMapping(b,P);let x=e.render.frame;r[b.id]!==x&&(d(b),r[b.id]=x)}function h(b){let M=u();b.__bindingPointIndex=M;let E=s.createBuffer(),P=b.__size,x=b.usage;return s.bindBuffer(s.UNIFORM_BUFFER,E),s.bufferData(s.UNIFORM_BUFFER,P,x),s.bindBuffer(s.UNIFORM_BUFFER,null),s.bindBufferBase(s.UNIFORM_BUFFER,M,E),E}function u(){for(let b=0;b<o;b++)if(a.indexOf(b)===-1)return a.push(b),b;return ht("WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function d(b){let M=i[b.id],E=b.uniforms,P=b.__cache;s.bindBuffer(s.UNIFORM_BUFFER,M);for(let x=0,C=E.length;x<C;x++){let w=E[x];if(Array.isArray(w))for(let I=0,y=w.length;I<y;I++)p(w[I],x,I,P);else p(w,x,0,P)}s.bindBuffer(s.UNIFORM_BUFFER,null)}function p(b,M,E,P){if(_(b,M,E,P)===!0){let x=b.__offset,C=b.value;if(Array.isArray(C)){let w=0;for(let I=0;I<C.length;I++){let y=C[I],N=f(y);g(y,b.__data,w),typeof y!="number"&&typeof y!="boolean"&&!y.isMatrix3&&!ArrayBuffer.isView(y)&&(w+=N.storage/Float32Array.BYTES_PER_ELEMENT)}}else g(C,b.__data,0);s.bufferSubData(s.UNIFORM_BUFFER,x,b.__data)}}function g(b,M,E){typeof b=="number"||typeof b=="boolean"?M[0]=b:b.isMatrix3?(M[0]=b.elements[0],M[1]=b.elements[1],M[2]=b.elements[2],M[3]=0,M[4]=b.elements[3],M[5]=b.elements[4],M[6]=b.elements[5],M[7]=0,M[8]=b.elements[6],M[9]=b.elements[7],M[10]=b.elements[8],M[11]=0):ArrayBuffer.isView(b)?M.set(new b.constructor(b.buffer,b.byteOffset,M.length)):b.toArray(M,E)}function _(b,M,E,P){let x=b.value,C=M+"_"+E;if(P[C]===void 0)return typeof x=="number"||typeof x=="boolean"?P[C]=x:ArrayBuffer.isView(x)?P[C]=x.slice():P[C]=x.clone(),!0;{let w=P[C];if(typeof x=="number"||typeof x=="boolean"){if(w!==x)return P[C]=x,!0}else{if(ArrayBuffer.isView(x))return!0;if(w.equals(x)===!1)return w.copy(x),!0}}return!1}function m(b){let M=b.uniforms,E=0,P=16;for(let C=0,w=M.length;C<w;C++){let I=Array.isArray(M[C])?M[C]:[M[C]];for(let y=0,N=I.length;y<N;y++){let D=I[y],R=Array.isArray(D.value)?D.value:[D.value];for(let U=0,z=R.length;U<z;U++){let T=R[U],G=f(T),X=E%P,Z=X%G.boundary,H=X+Z;E+=Z,H!==0&&P-H<G.storage&&(E+=P-H),D.__data=new Float32Array(G.storage/Float32Array.BYTES_PER_ELEMENT),D.__offset=E,E+=G.storage}}}let x=E%P;return x>0&&(E+=P-x),b.__size=E,b.__cache={},this}function f(b){let M={boundary:0,storage:0};return typeof b=="number"||typeof b=="boolean"?(M.boundary=4,M.storage=4):b.isVector2?(M.boundary=8,M.storage=8):b.isVector3||b.isColor?(M.boundary=16,M.storage=12):b.isVector4?(M.boundary=16,M.storage=16):b.isMatrix3?(M.boundary=48,M.storage=48):b.isMatrix4?(M.boundary=64,M.storage=64):b.isTexture?nt("WebGLRenderer: Texture samplers can not be part of an uniforms group."):ArrayBuffer.isView(b)?(M.boundary=16,M.storage=b.byteLength):nt("WebGLRenderer: Unsupported uniform value type.",b),M}function v(b){let M=b.target;M.removeEventListener("dispose",v);let E=a.indexOf(M.__bindingPointIndex);a.splice(E,1),s.deleteBuffer(i[M.id]),delete i[M.id],delete r[M.id]}function S(){for(let b in i)s.deleteBuffer(i[b]);a=[],i={},r={}}return{bind:l,update:c,dispose:S}}var n_=new Uint16Array([12469,15057,12620,14925,13266,14620,13807,14376,14323,13990,14545,13625,14713,13328,14840,12882,14931,12528,14996,12233,15039,11829,15066,11525,15080,11295,15085,10976,15082,10705,15073,10495,13880,14564,13898,14542,13977,14430,14158,14124,14393,13732,14556,13410,14702,12996,14814,12596,14891,12291,14937,11834,14957,11489,14958,11194,14943,10803,14921,10506,14893,10278,14858,9960,14484,14039,14487,14025,14499,13941,14524,13740,14574,13468,14654,13106,14743,12678,14818,12344,14867,11893,14889,11509,14893,11180,14881,10751,14852,10428,14812,10128,14765,9754,14712,9466,14764,13480,14764,13475,14766,13440,14766,13347,14769,13070,14786,12713,14816,12387,14844,11957,14860,11549,14868,11215,14855,10751,14825,10403,14782,10044,14729,9651,14666,9352,14599,9029,14967,12835,14966,12831,14963,12804,14954,12723,14936,12564,14917,12347,14900,11958,14886,11569,14878,11247,14859,10765,14828,10401,14784,10011,14727,9600,14660,9289,14586,8893,14508,8533,15111,12234,15110,12234,15104,12216,15092,12156,15067,12010,15028,11776,14981,11500,14942,11205,14902,10752,14861,10393,14812,9991,14752,9570,14682,9252,14603,8808,14519,8445,14431,8145,15209,11449,15208,11451,15202,11451,15190,11438,15163,11384,15117,11274,15055,10979,14994,10648,14932,10343,14871,9936,14803,9532,14729,9218,14645,8742,14556,8381,14461,8020,14365,7603,15273,10603,15272,10607,15267,10619,15256,10631,15231,10614,15182,10535,15118,10389,15042,10167,14963,9787,14883,9447,14800,9115,14710,8665,14615,8318,14514,7911,14411,7507,14279,7198,15314,9675,15313,9683,15309,9712,15298,9759,15277,9797,15229,9773,15166,9668,15084,9487,14995,9274,14898,8910,14800,8539,14697,8234,14590,7790,14479,7409,14367,7067,14178,6621,15337,8619,15337,8631,15333,8677,15325,8769,15305,8871,15264,8940,15202,8909,15119,8775,15022,8565,14916,8328,14804,8009,14688,7614,14569,7287,14448,6888,14321,6483,14088,6171,15350,7402,15350,7419,15347,7480,15340,7613,15322,7804,15287,7973,15229,8057,15148,8012,15046,7846,14933,7611,14810,7357,14682,7069,14552,6656,14421,6316,14251,5948,14007,5528,15356,5942,15356,5977,15353,6119,15348,6294,15332,6551,15302,6824,15249,7044,15171,7122,15070,7050,14949,6861,14818,6611,14679,6349,14538,6067,14398,5651,14189,5311,13935,4958,15359,4123,15359,4153,15356,4296,15353,4646,15338,5160,15311,5508,15263,5829,15188,6042,15088,6094,14966,6001,14826,5796,14678,5543,14527,5287,14377,4985,14133,4586,13869,4257,15360,1563,15360,1642,15358,2076,15354,2636,15341,3350,15317,4019,15273,4429,15203,4732,15105,4911,14981,4932,14836,4818,14679,4621,14517,4386,14359,4156,14083,3795,13808,3437,15360,122,15360,137,15358,285,15355,636,15344,1274,15322,2177,15281,2765,15215,3223,15120,3451,14995,3569,14846,3567,14681,3466,14511,3305,14344,3121,14037,2800,13753,2467,15360,0,15360,1,15359,21,15355,89,15346,253,15325,479,15287,796,15225,1148,15133,1492,15008,1749,14856,1882,14685,1886,14506,1783,14324,1608,13996,1398,13702,1183]),Si=null;function i_(){return Si===null&&(Si=new ss(n_,16,16,fs,rn),Si.name="DFG_LUT",Si.minFilter=jt,Si.magFilter=jt,Si.wrapS=qn,Si.wrapT=qn,Si.generateMipmaps=!1,Si.needsUpdate=!0),Si}var uc=class{constructor(e={}){let{canvas:t=gf(),context:n=null,depth:i=!0,stencil:r=!1,alpha:a=!1,antialias:o=!1,premultipliedAlpha:l=!0,preserveDrawingBuffer:c=!1,powerPreference:h="default",failIfMajorPerformanceCaveat:u=!1,reversedDepthBuffer:d=!1,outputBufferType:p=In}=e;this.isWebGLRenderer=!0;let g;if(n!==null){if(typeof WebGLRenderingContext<"u"&&n instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");g=n.getContextAttributes().alpha}else g=a;let _=p,m=new Set([Al,Tl,El]),f=new Set([In,li,Fr,Nr,Ml,Sl]),v=new Uint32Array(4),S=new Int32Array(4),b=new k,M=null,E=null,P=[],x=[],C=null;this.domElement=t,this.debug={checkShaderErrors:!0,diagnostics:{keywords:!1},onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this.toneMapping=ai,this.toneMappingExposure=1,this.transmissionResolutionScale=1;let w=this,I=!1,y=null,N=null,D=null,R=null;this._outputColorSpace=Xt;let U=0,z=0,T=null,G=-1,X=null,Z=new Pt,H=new Pt,oe=null,ke=new Le(0),Fe=0,We=t.width,fe=t.height,re=1,Pe=null,be=null,ge=new Pt(0,0,We,fe),ie=new Pt(0,0,We,fe),le=!1,se=new yr,ce=!1,ve=!1,Ee=new pt,qe=new k,Ye=new Pt,st={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},at=!1;function ot(){return T===null?re:1}let J=n;function dt(F,te){return t.getContext(F,te)}let ct,O,A,ne,de,he,Ue,Oe,V,j,me,_e,pe,xe,Se,we,Ie,q,Te,Me,ue,Re,ye;try{let F={alpha:!0,depth:i,stencil:r,antialias:o,premultipliedAlpha:l,preserveDrawingBuffer:c,powerPreference:h,failIfMajorPerformanceCaveat:u};if("setAttribute"in t&&t.setAttribute("data-engine",`three.js r${"186"}`),t.addEventListener("webglcontextlost",B,!1),t.addEventListener("webglcontextrestored",W,!1),t.addEventListener("webglcontextcreationerror",ee,!1),J===null){let te="webgl2";if(J=dt(te,F),J===null)throw dt(te)?new Error("THREE.WebGLRenderer: Error creating WebGL context with your selected attributes."):new Error("THREE.WebGLRenderer: Error creating WebGL context.")}je()}catch(F){throw t.removeEventListener("webglcontextlost",B,!1),t.removeEventListener("webglcontextrestored",W,!1),t.removeEventListener("webglcontextcreationerror",ee,!1),ht("WebGLRenderer: "+F.message),F}function je(){ct=new hx(J),ct.init(),ue=new Jv(J,ct),O=new ex(J,ct,e,ue),A=new Kv(J,ct),O.reversedDepthBuffer&&d&&A.buffers.depth.setReversed(!0),N=J.createFramebuffer(),D=J.createFramebuffer(),R=J.createFramebuffer(),ne=new fx(J),de=new Fv,he=new Yv(J,ct,A,de,O,ue,ne),Ue=new cx(w),Oe=new mg(J),Re=new $b(J,Oe),V=new ux(J,Oe,ne,Re),j=new mx(J,V,Oe,Re,ne),q=new px(J,O,he),Se=new tx(de),me=new Lv(w,Ue,ct,O,Re,Se),_e=new e_(w,de),pe=new Uv,xe=new Gv(ct),Ie=new Zb(w,Ue,A,j,g,l),we=new jv(w,j,O),ye=new t_(J,ne,O,A),Te=new Qb(J,ct,ne),Me=new dx(J,ct,ne),ne.programs=me.programs,w.capabilities=O,w.extensions=ct,w.properties=de,w.renderLists=pe,w.shadowMap=we,w.state=A,w.info=ne}_!==In&&(C=new bx(_,t.width,t.height,o,i,r));let Ke=new Jh(w,J);this.xr=Ke,this.getContext=function(){return J},this.getContextAttributes=function(){return J.getContextAttributes()},this.forceContextLoss=function(){let F=ct.get("WEBGL_lose_context");F&&F.loseContext()},this.forceContextRestore=function(){let F=ct.get("WEBGL_lose_context");F&&F.restoreContext()},this.getPixelRatio=function(){return re},this.setPixelRatio=function(F){F!==void 0&&(re=F,this.setSize(We,fe,!1))},this.getSize=function(F){return F.set(We,fe)},this.setSize=function(F,te,L=!0){if(Ke.isPresenting){nt("WebGLRenderer: Can't change size while VR device is presenting.");return}We=F,fe=te,t.width=Math.floor(F*re),t.height=Math.floor(te*re),L===!0&&(t.style.width=F+"px",t.style.height=te+"px"),C!==null&&C.setSize(t.width,t.height),this.setViewport(0,0,F,te)},this.getDrawingBufferSize=function(F){return F.set(We*re,fe*re).floor()},this.setDrawingBufferSize=function(F,te,L){We=F,fe=te,re=L,t.width=Math.floor(F*L),t.height=Math.floor(te*L),this.setViewport(0,0,F,te)},this.setEffects=function(F){if(_===In){ht("WebGLRenderer: setEffects() requires outputBufferType set to HalfFloatType or FloatType.");return}if(F){for(let te=0;te<F.length;te++)if(F[te].isOutputPass===!0){nt("WebGLRenderer: OutputPass is not needed in setEffects(). Tone mapping and color space conversion are applied automatically.");break}}C.setEffects(F||[])},this.getCurrentViewport=function(F){return F.copy(Z)},this.getViewport=function(F){return F.copy(ge)},this.setViewport=function(F,te,L,K){F.isVector4?ge.set(F.x,F.y,F.z,F.w):ge.set(F,te,L,K),A.viewport(Z.copy(ge).multiplyScalar(re).round())},this.getScissor=function(F){return F.copy(ie)},this.setScissor=function(F,te,L,K){F.isVector4?ie.set(F.x,F.y,F.z,F.w):ie.set(F,te,L,K),A.scissor(H.copy(ie).multiplyScalar(re).round())},this.getScissorTest=function(){return le},this.setScissorTest=function(F){A.setScissorTest(le=F)},this.setOpaqueSort=function(F){Pe=F},this.setTransparentSort=function(F){be=F},this.getClearColor=function(F){return F.copy(Ie.getClearColor())},this.setClearColor=function(){Ie.setClearColor(...arguments)},this.getClearAlpha=function(){return Ie.getClearAlpha()},this.setClearAlpha=function(){Ie.setClearAlpha(...arguments)},this.clear=function(F=!0,te=!0,L=!0){let K=0;if(F){let $=!1;if(T!==null){let De=T.texture.format;$=m.has(De)}if($){let De=T.texture.type,ze=f.has(De),Ne=Ie.getClearColor(),Ge=Ie.getClearAlpha(),He=Ne.r,tt=Ne.g,Xe=Ne.b;ze?(v[0]=He,v[1]=tt,v[2]=Xe,v[3]=Ge,J.clearBufferuiv(J.COLOR,0,v)):(S[0]=He,S[1]=tt,S[2]=Xe,S[3]=Ge,J.clearBufferiv(J.COLOR,0,S))}else K|=J.COLOR_BUFFER_BIT}te&&(K|=J.DEPTH_BUFFER_BIT,this.state.buffers.depth.setMask(!0)),L&&(K|=J.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),K!==0&&J.clear(K)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.setNodesHandler=function(F){F.setRenderer(this),y=F},this.dispose=function(){t.removeEventListener("webglcontextlost",B,!1),t.removeEventListener("webglcontextrestored",W,!1),t.removeEventListener("webglcontextcreationerror",ee,!1),Ie.dispose(),pe.dispose(),xe.dispose(),de.dispose(),Ue.dispose(),j.dispose(),Re.dispose(),ye.dispose(),me.dispose(),Ke.dispose(),Ke.removeEventListener("sessionstart",Ze),Ke.removeEventListener("sessionend",Ht),Et.stop()};function B(F){F.preventDefault(),ha("WebGLRenderer: Context Lost."),I=!0}function W(){ha("WebGLRenderer: Context Restored."),I=!1;let F=ne.autoReset,te=we.enabled,L=we.autoUpdate,K=we.needsUpdate,$=we.type;je(),ne.autoReset=F,we.enabled=te,we.autoUpdate=L,we.needsUpdate=K,we.type=$}function ee(F){ht("WebGLRenderer: A WebGL context could not be created. Reason: ",F.statusMessage)}function ae(F){let te=F.target;te.removeEventListener("dispose",ae),Y(te)}function Y(F){Q(F),de.remove(F)}function Q(F){let te=de.get(F).programs;te!==void 0&&(te.forEach(function(L){me.releaseProgram(L)}),F.isShaderMaterial&&me.releaseShaderCache(F))}this.renderBufferDirect=function(F,te,L,K,$,De){te===null&&(te=st);let ze=$.isMesh&&$.matrixWorld.determinantAffine()<0,Ne=Yr(F,te,L,K,$);A.setMaterial(K,ze);let Ge=L.index,He=1;if(K.wireframe===!0){if(Ge=V.getWireframeAttribute(L),Ge===void 0)return;He=2}let tt=L.drawRange,Xe=L.attributes.position,Ce=tt.start*He,et=(tt.start+tt.count)*He;De!==null&&(Ce=Math.max(Ce,De.start*He),et=Math.min(et,(De.start+De.count)*He)),Ge!==null?(Ce=Math.max(Ce,0),et=Math.min(et,Ge.count)):Xe!=null&&(Ce=Math.max(Ce,0),et=Math.min(et,Xe.count));let Qe=et-Ce;if(Qe<0||Qe===1/0)return;Re.setup($,K,Ne,L,Ge);let ft,Ct=Te;if(Ge!==null&&(ft=Oe.get(Ge),Ct=Me,Ct.setIndex(ft)),$.isMesh)K.wireframe===!0?(A.setLineWidth(K.wireframeLinewidth*ot()),Ct.setMode(J.LINES)):Ct.setMode(J.TRIANGLES);else if($.isLine){let Wt=K.linewidth;Wt===void 0&&(Wt=1),A.setLineWidth(Wt*ot()),$.isLineSegments?Ct.setMode(J.LINES):$.isLineLoop?Ct.setMode(J.LINE_LOOP):Ct.setMode(J.LINE_STRIP)}else $.isPoints?Ct.setMode(J.POINTS):$.isSprite&&Ct.setMode(J.TRIANGLES);if($.isBatchedMesh)if(ct.get("WEBGL_multi_draw"))Ct.renderMultiDraw($._multiDrawStarts,$._multiDrawCounts,$._multiDrawCount);else{let Wt=$._multiDrawStarts,$e=$._multiDrawCounts,Mn=$._multiDrawCount,It=Ge?Oe.get(Ge).bytesPerElement:1,Vn=de.get(K).currentProgram.getUniforms();for(let ui=0;ui<Mn;ui++)Vn.setValue(J,"_gl_DrawID",ui),Ct.render(Wt[ui]/It,$e[ui])}else if($.isInstancedMesh)Ct.renderInstances(Ce,Qe,$.count);else if(L.isInstancedBufferGeometry){let Wt=L._maxInstanceCount!==void 0?L._maxInstanceCount:1/0,$e=Math.min(L.instanceCount,Wt);Ct.renderInstances(Ce,Qe,$e)}else Ct.render(Ce,Qe)};function Ae(F,te,L,K){y!==null&&F.isNodeMaterial&&y.setObject(K,F),ce===!0&&Se.setState(F,L,!1),F.transparent===!0&&F.side===Ot&&F.forceSinglePass===!1?(F.side=on,F.needsUpdate=!0,mn(F,te,K),F.side=jn,F.needsUpdate=!0,mn(F,te,K),F.side=Ot):mn(F,te,K)}this.compile=function(F,te,L=null){L===null&&(L=F),y!==null&&y.renderStart(F,te,L),E=xe.get(L),E.init(te),x.push(E),L.traverseVisible(function($){$.isLight&&$.layers.test(te.layers)&&(E.pushLight($),$.castShadow&&E.pushShadow($))}),F!==L&&F.traverseVisible(function($){$.isLight&&$.layers.test(te.layers)&&(E.pushLight($),$.castShadow&&E.pushShadow($))}),E.setupLights(),y!==null&&y.updateLights(E.state.lightsArray),ve=this.localClippingEnabled,ce=Se.init(this.clippingPlanes,ve),ce===!0&&Se.setGlobalState(this.clippingPlanes,te),y!==null&&we.render(E.state.shadowsArray,L,te);let K=new Set;return F.traverse(function($){if(!($.isMesh||$.isPoints||$.isLine||$.isSprite))return;let De=$.material;if(De)if(Array.isArray(De))for(let ze=0;ze<De.length;ze++){let Ne=De[ze];Ae(Ne,L,te,$),K.add(Ne)}else Ae(De,L,te,$),K.add(De)}),E=x.pop(),y!==null&&y.renderEnd(),K},this.compileAsync=function(F,te,L=null){let K=this.compile(F,te,L);return new Promise($=>{function De(){if(K.forEach(function(ze){let Ge=de.get(ze).currentProgram;(Ge===void 0||Ge.isReady())&&K.delete(ze)}),K.size===0){$(F);return}setTimeout(De,10)}ct.get("KHR_parallel_shader_compile")!==null?De():setTimeout(De,10)})};let Ve=null;function rt(F){Ve&&Ve(F)}function Ze(){Et.stop()}function Ht(){Et.start()}let Et=new Wf;Et.setAnimationLoop(rt),typeof self<"u"&&Et.setContext(self),this.setAnimationLoop=function(F){Ve=F,Ke.setAnimationLoop(F),F===null?Et.stop():Et.start()},Ke.addEventListener("sessionstart",Ze),Ke.addEventListener("sessionend",Ht),this.render=function(F,te){if(te!==void 0&&te.isCamera!==!0){ht("WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(I===!0)return;y!==null&&y.renderStart(F,te);let L=Ke.enabled===!0&&Ke.isPresenting===!0,K=C!==null&&(T===null||L)&&C.begin(w,T);if(F.matrixWorldAutoUpdate===!0&&F.updateMatrixWorld(),te.parent===null&&te.matrixWorldAutoUpdate===!0&&te.updateMatrixWorld(),Ke.enabled===!0&&Ke.isPresenting===!0&&(C===null||C.isCompositing()===!1)&&(Ke.cameraAutoUpdate===!0&&Ke.updateCamera(te),te=Ke.getCamera()),F.isScene===!0&&F.onBeforeRender(w,F,te,T),E=xe.get(F,x.length),E.init(te),E.state.textureUnits=he.getTextureUnits(),x.push(E),Ee.multiplyMatrices(te.projectionMatrix,te.matrixWorldInverse),se.setFromProjectionMatrix(Ee,ni,te.reversedDepth),ve=this.localClippingEnabled,ce=Se.init(this.clippingPlanes,ve),M=pe.get(F,P.length),M.init(),P.push(M),Ke.enabled===!0&&Ke.isPresenting===!0){let ze=w.xr.getDepthSensingMesh();ze!==null&&lt(ze,te,-1/0,w.sortObjects)}lt(F,te,0,w.sortObjects),M.finish(),y!==null&&y.updateLights(E.state.lightsArray),w.sortObjects===!0&&M.sort(Pe,be),at=Ke.enabled===!1||Ke.isPresenting===!1||Ke.hasDepthSensing()===!1,at&&Ie.addToRenderList(M,F),this.info.render.frame++,this.info.autoReset===!0&&this.info.reset(),ce===!0&&Se.beginShadows();let $=E.state.shadowsArray;if(we.render($,F,te),ce===!0&&Se.endShadows(),(K&&C.hasRenderPass())===!1){let ze=M.opaque,Ne=M.transmissive;if(E.setupLights(),te.isArrayCamera){let Ge=te.cameras;if(Ne.length>0)for(let He=0,tt=Ge.length;He<tt;He++){let Xe=Ge[He];Rt(ze,Ne,F,Xe)}at&&Ie.render(F);for(let He=0,tt=Ge.length;He<tt;He++){let Xe=Ge[He];an(M,F,Xe,Xe.viewport)}}else Ne.length>0&&Rt(ze,Ne,F,te),at&&Ie.render(F),an(M,F,te)}T!==null&&z===0&&(he.updateMultisampleRenderTarget(T),he.updateRenderTargetMipmap(T)),K&&C.end(w),F.isScene===!0&&F.onAfterRender(w,F,te),Re.resetDefaultState(),G=-1,X=null,x.pop(),x.length>0?(E=x[x.length-1],he.setTextureUnits(E.state.textureUnits),ce===!0&&Se.setGlobalState(w.clippingPlanes,E.state.camera)):E=null,P.pop(),P.length>0?M=P[P.length-1]:M=null,y!==null&&y.renderEnd()};function lt(F,te,L,K){if(F.visible===!1)return;if(F.layers.test(te.layers)){if(F.isGroup)L=F.renderOrder;else if(F.isLOD)F.autoUpdate===!0&&F.update(te);else if(F.isLightProbeGrid)E.pushLightProbeGrid(F);else if(F.isLight)E.pushLight(F),F.castShadow&&E.pushShadow(F);else if(F.isSprite){if(!F.frustumCulled||F.intersectsFrustum(se)){K&&Ye.setFromMatrixPosition(F.matrixWorld).applyMatrix4(Ee);let ze=j.update(F),Ne=F.material;Ne.visible&&M.push(F,ze,Ne,L,Ye.z,null,te)}}else if((F.isMesh||F.isLine||F.isPoints)&&(!F.frustumCulled||F.intersectsFrustum(se))){let ze=j.update(F),Ne=F.material;if(K&&(F.boundingSphere!==void 0?(F.boundingSphere===null&&F.computeBoundingSphere(),Ye.copy(F.boundingSphere.center)):(ze.boundingSphere===null&&ze.computeBoundingSphere(),Ye.copy(ze.boundingSphere.center)),Ye.applyMatrix4(F.matrixWorld).applyMatrix4(Ee)),Array.isArray(Ne)){let Ge=ze.groups;for(let He=0,tt=Ge.length;He<tt;He++){let Xe=Ge[He],Ce=Ne[Xe.materialIndex];Ce&&Ce.visible&&M.push(F,ze,Ce,L,Ye.z,Xe,te)}}else Ne.visible&&M.push(F,ze,Ne,L,Ye.z,null,te)}}let De=F.children;for(let ze=0,Ne=De.length;ze<Ne;ze++)lt(De[ze],te,L,K)}function an(F,te,L,K){let{opaque:$,transmissive:De,transparent:ze}=F;E.setupLightsView(L),ce===!0&&Se.setGlobalState(w.clippingPlanes,L),K&&A.viewport(Z.copy(K)),$.length>0&&Yt($,te,L),De.length>0&&Yt(De,te,L),ze.length>0&&Yt(ze,te,L),A.buffers.depth.setTest(!0),A.buffers.depth.setMask(!0),A.buffers.color.setMask(!0),A.setPolygonOffset(!1)}function Rt(F,te,L,K){if((L.isScene===!0?L.overrideMaterial:null)!==null)return;if(E.state.transmissionRenderTarget[K.id]===void 0){let Ce=ct.has("EXT_color_buffer_half_float")||ct.has("EXT_color_buffer_float");E.state.transmissionRenderTarget[K.id]=new Kt(1,1,{generateMipmaps:!0,type:Ce?rn:In,minFilter:oi,samples:Math.max(4,O.samples),stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,storeMultisampledDepthBuffer:!1,storeMultisampledStencilBuffer:!1,colorSpace:_t.workingColorSpace})}let De=E.state.transmissionRenderTarget[K.id],ze=K.viewport||Z;De.setSize(ze.z*w.transmissionResolutionScale,ze.w*w.transmissionResolutionScale);let Ne=w.getRenderTarget(),Ge=w.getActiveCubeFace(),He=w.getActiveMipmapLevel();w.setRenderTarget(De),w.getClearColor(ke),Fe=w.getClearAlpha(),Fe<1&&w.setClearColor(16777215,.5),w.clear(),at&&Ie.render(L);let tt=w.toneMapping;w.toneMapping=ai;let Xe=K.viewport;if(K.viewport!==void 0&&(K.viewport=void 0),E.setupLightsView(K),ce===!0&&Se.setGlobalState(w.clippingPlanes,K),Yt(F,L,K),he.updateMultisampleRenderTarget(De),he.updateRenderTargetMipmap(De),ct.has("WEBGL_multisampled_render_to_texture")===!1){let Ce=!1;for(let et=0,Qe=te.length;et<Qe;et++){let ft=te[et],{object:Ct,geometry:Wt,material:$e,group:Mn}=ft;if($e.side===Ot&&Ct.layers.test(K.layers)){let It=$e.side;$e.side=on,$e.needsUpdate=!0,Gt(Ct,L,K,Wt,$e,Mn),$e.side=It,$e.needsUpdate=!0,Ce=!0}}Ce===!0&&(he.updateMultisampleRenderTarget(De),he.updateRenderTargetMipmap(De))}w.setRenderTarget(Ne,Ge,He),w.setClearColor(ke,Fe),Xe!==void 0&&(K.viewport=Xe),w.toneMapping=tt}function Yt(F,te,L){let K=te.isScene===!0?te.overrideMaterial:null;for(let $=0,De=F.length;$<De;$++){let ze=F[$],{object:Ne,geometry:Ge,group:He}=ze,tt=ze.material;tt.allowOverride===!0&&K!==null&&(tt=K),Ne.layers.test(L.layers)&&Gt(Ne,te,L,Ge,tt,He)}}function Gt(F,te,L,K,$,De){y!==null&&$.isNodeMaterial&&y.setObject(F,$),F.onBeforeRender(w,te,L,K,$,De),F.modelViewMatrix.multiplyMatrices(L.matrixWorldInverse,F.matrixWorld),F.normalMatrix.getNormalMatrix(F.modelViewMatrix),$.onBeforeRender(w,te,L,K,F,De),$.transparent===!0&&$.side===Ot&&$.forceSinglePass===!1?($.side=on,$.needsUpdate=!0,w.renderBufferDirect(L,te,K,$,F,De),$.side=jn,$.needsUpdate=!0,w.renderBufferDirect(L,te,K,$,F,De),$.side=Ot):w.renderBufferDirect(L,te,K,$,F,De),F.onAfterRender(w,te,L,K,$,De)}function mn(F,te,L){te.isScene!==!0&&(te=st);let K=de.get(F),$=E.state.lights,De=E.state.shadowsArray,ze=$.state.version,Ne=me.getParameters(F,$.state,De,te,L,E.state.lightProbeGridArray),Ge=me.getProgramCacheKey(Ne),He=K.programs;K.environment=F.isMeshStandardMaterial||F.isMeshLambertMaterial||F.isMeshPhongMaterial?te.environment:null,K.fog=te.fog;let tt=F.isMeshStandardMaterial||F.isMeshLambertMaterial&&!F.envMap||F.isMeshPhongMaterial&&!F.envMap;K.envMap=Ue.get(F.envMap||K.environment,tt),K.envMapRotation=K.environment!==null&&F.envMap===null?te.environmentRotation:F.envMapRotation,He===void 0&&(F.addEventListener("dispose",ae),He=new Map,K.programs=He);let Xe=He.get(Ge);if(Xe!==void 0){if(K.currentProgram===Xe&&K.lightsStateVersion===ze)return yn(F,Ne),Xe}else Ne.uniforms=me.getUniforms(F),y!==null&&F.isNodeMaterial&&y.build(F,L,Ne),F.onBeforeCompile(Ne,w),Xe=me.acquireProgram(Ne,Ge),He.set(Ge,Xe),K.uniforms=Ne.uniforms;let Ce=K.uniforms;return(!F.isShaderMaterial&&!F.isRawShaderMaterial||F.clipping===!0)&&(Ce.clippingPlanes=Se.uniform),yn(F,Ne),K.needsLights=lo(F),K.lightsStateVersion=ze,K.needsLights&&(Ce.ambientLightColor.value=$.state.ambient,Ce.lightProbe.value=$.state.probe,Ce.sunLights.value=$.state.sun,Ce.sunLightShadows.value=$.state.sunShadow,Ce.directionalLights.value=$.state.directional,Ce.directionalLightShadows.value=$.state.directionalShadow,Ce.spotLights.value=$.state.spot,Ce.spotLightShadows.value=$.state.spotShadow,Ce.rectAreaLights.value=$.state.rectArea,Ce.ltc_1.value=$.state.rectAreaLTC1,Ce.ltc_2.value=$.state.rectAreaLTC2,Ce.pointLights.value=$.state.point,Ce.pointLightShadows.value=$.state.pointShadow,Ce.hemisphereLights.value=$.state.hemi,Ce.sunShadowMatrix.value=$.state.sunShadowMatrix,Ce.sunShadowCascade.value=$.state.sunShadowCascade,Ce.directionalShadowMatrix.value=$.state.directionalShadowMatrix,Ce.spotLightMatrix.value=$.state.spotLightMatrix,Ce.spotLightMap.value=$.state.spotLightMap,Ce.pointShadowMatrix.value=$.state.pointShadowMatrix),K.lightProbeGrid=E.state.lightProbeGridArray.length>0,K.currentProgram=Xe,K.uniformsList=null,Xe}function ji(F){if(F.uniformsList===null){let te=F.currentProgram.getUniforms();F.uniformsList=Br.seqWithValue(te.seq,F.uniforms)}return F.uniformsList}function yn(F,te){let L=de.get(F);L.outputColorSpace=te.outputColorSpace,L.batching=te.batching,L.batchingColor=te.batchingColor,L.instancing=te.instancing,L.instancingColor=te.instancingColor,L.instancingMorph=te.instancingMorph,L.skinning=te.skinning,L.morphTargets=te.morphTargets,L.morphNormals=te.morphNormals,L.morphColors=te.morphColors,L.morphTargetsCount=te.morphTargetsCount,L.numClippingPlanes=te.numClippingPlanes,L.numIntersection=te.numClipIntersection,L.vertexAlphas=te.vertexAlphas,L.vertexTangents=te.vertexTangents,L.toneMapping=te.toneMapping}function Xs(F,te){if(F.length===0)return null;if(F.length===1)return F[0].texture!==null?F[0]:null;b.setFromMatrixPosition(te.matrixWorld);for(let L=0,K=F.length;L<K;L++){let $=F[L];if($.texture!==null&&$.boundingBox.containsPoint(b))return $}return null}function Yr(F,te,L,K,$){te.isScene!==!0&&(te=st),he.resetTextureUnits();let De=te.fog,ze=K.isMeshStandardMaterial||K.isMeshLambertMaterial||K.isMeshPhongMaterial?te.environment:null,Ne=T===null?w.outputColorSpace:T.isXRRenderTarget===!0?T.texture.colorSpace:_t.workingColorSpace,Ge=K.isMeshStandardMaterial||K.isMeshLambertMaterial&&!K.envMap||K.isMeshPhongMaterial&&!K.envMap,He=Ue.get(K.envMap||ze,Ge),tt=K.vertexColors===!0&&!!L.attributes.color&&L.attributes.color.itemSize===4,Xe=!!L.attributes.tangent&&(!!K.normalMap||K.anisotropy>0),Ce=!!L.morphAttributes.position,et=!!L.morphAttributes.normal,Qe=!!L.morphAttributes.color,ft=ai;K.toneMapped&&(T===null||T.isXRRenderTarget===!0)&&(ft=w.toneMapping);let Ct=L.morphAttributes.position||L.morphAttributes.normal||L.morphAttributes.color,Wt=Ct!==void 0?Ct.length:0,$e=de.get(K),Mn=E.state.lights;if(ce===!0&&(ve===!0||F!==X)){let Vt=F===X&&K.id===G;Se.setState(K,F,Vt)}let It=!1;K.version===$e.__version?($e.needsLights&&$e.lightsStateVersion!==Mn.state.version||$e.outputColorSpace!==Ne||$.isBatchedMesh&&$e.batching===!1||!$.isBatchedMesh&&$e.batching===!0||$.isBatchedMesh&&$e.batchingColor===!0&&$._colorsTexture===null||$.isBatchedMesh&&$e.batchingColor===!1&&$._colorsTexture!==null||$.isInstancedMesh&&$e.instancing===!1||!$.isInstancedMesh&&$e.instancing===!0||$.isSkinnedMesh&&$e.skinning===!1||!$.isSkinnedMesh&&$e.skinning===!0||$.isInstancedMesh&&$e.instancingColor===!0&&$.instanceColor===null||$.isInstancedMesh&&$e.instancingColor===!1&&$.instanceColor!==null||$.isInstancedMesh&&$e.instancingMorph===!0&&$.morphTexture===null||$.isInstancedMesh&&$e.instancingMorph===!1&&$.morphTexture!==null||$e.envMap!==He||K.fog===!0&&$e.fog!==De||$e.numClippingPlanes!==void 0&&($e.numClippingPlanes!==Se.numPlanes||$e.numIntersection!==Se.numIntersection)||$e.vertexAlphas!==tt||$e.vertexTangents!==Xe||$e.morphTargets!==Ce||$e.morphNormals!==et||$e.morphColors!==Qe||$e.toneMapping!==ft||$e.morphTargetsCount!==Wt||!!$e.lightProbeGrid!=E.state.lightProbeGridArray.length>0)&&(It=!0):(It=!0,$e.__version=K.version);let Vn=$e.currentProgram;It===!0&&(Vn=mn(K,te,$),y&&K.isNodeMaterial&&y.onUpdateProgram(K,Vn,$e));let ui=!1,Ki=!1,js=!1,Bt=Vn.getUniforms(),Jt=$e.uniforms;if(A.useProgram(Vn.program)&&(ui=!0,Ki=!0,js=!0),K.id!==G&&(G=K.id,Ki=!0),$e.needsLights){let Vt=Xs(E.state.lightProbeGridArray,$);$e.lightProbeGrid!==Vt&&($e.lightProbeGrid=Vt,Ki=!0)}if(ui||X!==F){A.buffers.depth.getReversed()&&F.reversedDepth!==!0&&(F._reversedDepth=!0,F.updateProjectionMatrix()),Bt.setValue(J,"projectionMatrix",F.projectionMatrix),Bt.setValue(J,"viewMatrix",F.matrixWorldInverse);let Ji=Bt.map.cameraPosition;Ji!==void 0&&Ji.setValue(J,qe.setFromMatrixPosition(F.matrixWorld)),O.logarithmicDepthBuffer&&Bt.setValue(J,"logDepthBufFC",2/(Math.log(F.far+1)/Math.LN2)),(K.isMeshPhongMaterial||K.isMeshToonMaterial||K.isMeshLambertMaterial||K.isMeshBasicMaterial||K.isMeshStandardMaterial||K.isShaderMaterial)&&Bt.setValue(J,"isOrthographic",F.isOrthographicCamera===!0),X!==F&&(X=F,Ki=!0,js=!0)}if($e.needsLights&&(Mn.state.sunShadowMap.length>0&&Bt.setValue(J,"sunShadowMap",Mn.state.sunShadowMap,he),Mn.state.directionalShadowMap.length>0&&Bt.setValue(J,"directionalShadowMap",Mn.state.directionalShadowMap,he),Mn.state.spotShadowMap.length>0&&Bt.setValue(J,"spotShadowMap",Mn.state.spotShadowMap,he),Mn.state.pointShadowMap.length>0&&Bt.setValue(J,"pointShadowMap",Mn.state.pointShadowMap,he)),$.isSkinnedMesh){Bt.setOptional(J,$,"bindMatrix"),Bt.setOptional(J,$,"bindMatrixInverse");let Vt=$.skeleton;Vt&&(Vt.boneTexture===null&&Vt.computeBoneTexture(),Bt.setValue(J,"boneTexture",Vt.boneTexture,he))}$.isBatchedMesh&&(Bt.setOptional(J,$,"batchingTexture"),Bt.setValue(J,"batchingTexture",$._matricesTexture,he),Bt.setOptional(J,$,"batchingIdTexture"),Bt.setValue(J,"batchingIdTexture",$._indirectTexture,he),Bt.setOptional(J,$,"batchingColorTexture"),$._colorsTexture!==null&&Bt.setValue(J,"batchingColorTexture",$._colorsTexture,he));let Yi=L.morphAttributes;if((Yi.position!==void 0||Yi.normal!==void 0||Yi.color!==void 0)&&q.update($,L,Vn),(Ki||$e.receiveShadow!==$.receiveShadow)&&($e.receiveShadow=$.receiveShadow,Bt.setValue(J,"receiveShadow",$.receiveShadow)),(K.isMeshStandardMaterial||K.isMeshLambertMaterial||K.isMeshPhongMaterial)&&K.envMap===null&&te.environment!==null&&(Jt.envMapIntensity.value=te.environmentIntensity),Jt.dfgLUT!==void 0&&(Jt.dfgLUT.value=i_()),Ki){if(Bt.setValue(J,"toneMappingExposure",w.toneMappingExposure),$e.needsLights&&oo(Jt,js),De&&K.fog===!0&&_e.refreshFogUniforms(Jt,De),_e.refreshMaterialUniforms(Jt,K,re,fe,E.state.transmissionRenderTarget[F.id]),$e.needsLights&&$e.lightProbeGrid){let Vt=$e.lightProbeGrid;Jt.probesSH.value=Vt.texture,Jt.probesMin.value.copy(Vt.boundingBox.min),Jt.probesMax.value.copy(Vt.boundingBox.max),Jt.probesResolution.value.copy(Vt.resolution)}Br.upload(J,ji($e),Jt,he)}if(K.isShaderMaterial&&K.uniformsNeedUpdate===!0&&(Br.upload(J,ji($e),Jt,he),K.uniformsNeedUpdate=!1),K.isSpriteMaterial&&Bt.setValue(J,"center",$.center),Bt.setValue(J,"modelViewMatrix",$.modelViewMatrix),Bt.setValue(J,"normalMatrix",$.normalMatrix),Bt.setValue(J,"modelMatrix",$.matrixWorld),K.uniformsGroups!==void 0){let Vt=K.uniformsGroups;for(let Ji=0,Ks=Vt.length;Ji<Ks;Ji++){let Vu=Vt[Ji];ye.update(Vu,Vn),ye.bind(Vu,Vn)}}return Vn}function oo(F,te){F.ambientLightColor.needsUpdate=te,F.lightProbe.needsUpdate=te,F.sunLights.needsUpdate=te,F.sunLightShadows.needsUpdate=te,F.directionalLights.needsUpdate=te,F.directionalLightShadows.needsUpdate=te,F.pointLights.needsUpdate=te,F.pointLightShadows.needsUpdate=te,F.spotLights.needsUpdate=te,F.spotLightShadows.needsUpdate=te,F.rectAreaLights.needsUpdate=te,F.hemisphereLights.needsUpdate=te}function lo(F){return F.isMeshLambertMaterial||F.isMeshToonMaterial||F.isMeshPhongMaterial||F.isMeshStandardMaterial||F.isShadowMaterial||F.isShaderMaterial&&F.lights===!0}this.getActiveCubeFace=function(){return U},this.getActiveMipmapLevel=function(){return z},this.getRenderTarget=function(){return T},this.setRenderTargetTextures=function(F,te,L){let K=de.get(F);K.__autoAllocateDepthBuffer=F.resolveDepthBuffer===!1,K.__autoAllocateDepthBuffer===!1&&(K.__useRenderToTexture=!1),de.get(F.texture).__webglTexture=te,de.get(F.depthTexture).__webglTexture=K.__autoAllocateDepthBuffer?void 0:L,K.__hasExternalTextures=!0},this.setRenderTargetFramebuffer=function(F,te){let L=de.get(F);L.__webglFramebuffer=te,L.__useDefaultFramebuffer=te===void 0},this.setRenderTarget=function(F,te=0,L=0){T=F,U=te,z=L;let K=null,$=!1,De=!1;if(F){let Ne=de.get(F);if(Ne.__useDefaultFramebuffer!==void 0){A.bindFramebuffer(J.FRAMEBUFFER,Ne.__webglFramebuffer),Z.copy(F.viewport),H.copy(F.scissor),oe=F.scissorTest,A.viewport(Z),A.scissor(H),A.setScissorTest(oe),G=-1;return}else if(Ne.__webglFramebuffer===void 0)he.setupRenderTarget(F);else if(Ne.__hasExternalTextures)he.rebindTextures(F,de.get(F.texture).__webglTexture,de.get(F.depthTexture).__webglTexture);else if(F.depthBuffer){let tt=F.depthTexture;if(Ne.__boundDepthTexture!==tt){if(tt!==null&&de.has(tt)&&(F.width!==tt.image.width||F.height!==tt.image.height))throw new Error("THREE.WebGLRenderer: Attached DepthTexture is initialized to the incorrect size.");he.setupDepthRenderbuffer(F)}}let Ge=F.texture;(Ge.isData3DTexture||Ge.isDataArrayTexture||Ge.isCompressedArrayTexture)&&(De=!0);let He=de.get(F).__webglFramebuffer;F.isWebGLCubeRenderTarget?(Array.isArray(He[te])?K=He[te][L]:K=He[te],$=!0):F.samples>0&&he.useMultisampledRTT(F)===!1?K=de.get(F).__webglMultisampledFramebuffer:Array.isArray(He)?K=He[L]:K=He,Z.copy(F.viewport),H.copy(F.scissor),oe=F.scissorTest}else Z.copy(ge).multiplyScalar(re).floor(),H.copy(ie).multiplyScalar(re).floor(),oe=le;if(L!==0&&(K=N),A.bindFramebuffer(J.FRAMEBUFFER,K)&&A.drawBuffers(F,K),A.viewport(Z),A.scissor(H),A.setScissorTest(oe),$){let Ne=de.get(F.texture);J.framebufferTexture2D(J.FRAMEBUFFER,J.COLOR_ATTACHMENT0,J.TEXTURE_CUBE_MAP_POSITIVE_X+te,Ne.__webglTexture,L)}else if(De){let Ne=te;for(let Ge=0;Ge<F.textures.length;Ge++){let He=de.get(F.textures[Ge]);J.framebufferTextureLayer(J.FRAMEBUFFER,J.COLOR_ATTACHMENT0+Ge,He.__webglTexture,L,Ne)}}else if(F!==null&&L!==0){let Ne=de.get(F.texture);J.framebufferTexture2D(J.FRAMEBUFFER,J.COLOR_ATTACHMENT0,J.TEXTURE_2D,Ne.__webglTexture,L)}G=-1};function co(F){let te=de.get(F);return(te.__readFormat!==F.format||te.__readType!==F.type)&&(te.__readFormat=F.format,te.__readType=F.type,te.__formatReadable=O.textureFormatReadable(F.format),te.__typeReadable=O.textureTypeReadable(F.type)),te}this.readRenderTargetPixels=function(F,te,L,K,$,De,ze,Ne=0){if(!(F&&F.isWebGLRenderTarget)){ht("WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let Ge=de.get(F).__webglFramebuffer;if(F.isWebGLCubeRenderTarget&&ze!==void 0&&(Ge=Ge[ze]),Ge){A.bindFramebuffer(J.FRAMEBUFFER,Ge);try{let He=F.textures[Ne],tt=He.format,Xe=He.type;F.textures.length>1&&J.readBuffer(J.COLOR_ATTACHMENT0+Ne);let Ce=co(He);if(Ce.__formatReadable===!1){ht("WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if(Ce.__typeReadable===!1){ht("WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}te>=0&&te<=F.width-K&&L>=0&&L<=F.height-$&&J.readPixels(te,L,K,$,ue.convert(tt),ue.convert(Xe),De)}finally{let He=T!==null?de.get(T).__webglFramebuffer:null;A.bindFramebuffer(J.FRAMEBUFFER,He)}}},this.readRenderTargetPixelsAsync=async function(F,te,L,K,$,De,ze,Ne=0){if(!(F&&F.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let Ge=de.get(F).__webglFramebuffer;if(F.isWebGLCubeRenderTarget&&ze!==void 0&&(Ge=Ge[ze]),Ge)if(te>=0&&te<=F.width-K&&L>=0&&L<=F.height-$){A.bindFramebuffer(J.FRAMEBUFFER,Ge);let He=F.textures[Ne],tt=He.format,Xe=He.type;F.textures.length>1&&J.readBuffer(J.COLOR_ATTACHMENT0+Ne);let Ce=co(He);if(Ce.__formatReadable===!1)throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if(Ce.__typeReadable===!1)throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");let et=J.createBuffer();J.bindBuffer(J.PIXEL_PACK_BUFFER,et),J.bufferData(J.PIXEL_PACK_BUFFER,De.byteLength,J.STREAM_READ),J.readPixels(te,L,K,$,ue.convert(tt),ue.convert(Xe),0),J.bindBuffer(J.PIXEL_PACK_BUFFER,null);let Qe=T!==null?de.get(T).__webglFramebuffer:null;A.bindFramebuffer(J.FRAMEBUFFER,Qe);let ft=J.fenceSync(J.SYNC_GPU_COMMANDS_COMPLETE,0);return J.flush(),await xf(J,ft,4),J.bindBuffer(J.PIXEL_PACK_BUFFER,et),J.getBufferSubData(J.PIXEL_PACK_BUFFER,0,De),J.bindBuffer(J.PIXEL_PACK_BUFFER,null),J.deleteBuffer(et),J.deleteSync(ft),De}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")},this.copyFramebufferToTexture=function(F,te=null,L=0){let K=Math.pow(2,-L),$=Math.floor(F.image.width*K),De=Math.floor(F.image.height*K),ze=te!==null?te.x:0,Ne=te!==null?te.y:0;he.setTexture2D(F,0),J.copyTexSubImage2D(J.TEXTURE_2D,L,0,0,ze,Ne,$,De),A.unbindTexture()},this.copyTextureToTexture=function(F,te,L=null,K=null,$=0,De=0){let ze,Ne,Ge,He,tt,Xe,Ce,et,Qe,ft=F.isCompressedTexture?F.mipmaps[De]:F.image;if(L!==null)ze=L.max.x-L.min.x,Ne=L.max.y-L.min.y,Ge=L.isBox3?L.max.z-L.min.z:1,He=L.min.x,tt=L.min.y,Xe=L.isBox3?L.min.z:0;else{let Jt=Math.pow(2,-$);ze=Math.floor(ft.width*Jt),Ne=Math.floor(ft.height*Jt),F.isDataArrayTexture?Ge=ft.depth:F.isData3DTexture?Ge=Math.floor(ft.depth*Jt):Ge=1,He=0,tt=0,Xe=0}K!==null?(Ce=K.x,et=K.y,Qe=K.z):(Ce=0,et=0,Qe=0);let Ct=ue.convert(te.format),Wt=ue.convert(te.type),$e;te.isData3DTexture?(he.setTexture3D(te,0),$e=J.TEXTURE_3D):te.isDataArrayTexture||te.isCompressedArrayTexture?(he.setTexture2DArray(te,0),$e=J.TEXTURE_2D_ARRAY):(he.setTexture2D(te,0),$e=J.TEXTURE_2D),A.activeTexture(J.TEXTURE0),A.pixelStorei(J.UNPACK_FLIP_Y_WEBGL,te.flipY),A.pixelStorei(J.UNPACK_PREMULTIPLY_ALPHA_WEBGL,te.premultiplyAlpha),A.pixelStorei(J.UNPACK_ALIGNMENT,te.unpackAlignment);let Mn=A.getParameter(J.UNPACK_ROW_LENGTH),It=A.getParameter(J.UNPACK_IMAGE_HEIGHT),Vn=A.getParameter(J.UNPACK_SKIP_PIXELS),ui=A.getParameter(J.UNPACK_SKIP_ROWS),Ki=A.getParameter(J.UNPACK_SKIP_IMAGES);A.pixelStorei(J.UNPACK_ROW_LENGTH,ft.width),A.pixelStorei(J.UNPACK_IMAGE_HEIGHT,ft.height),A.pixelStorei(J.UNPACK_SKIP_PIXELS,He),A.pixelStorei(J.UNPACK_SKIP_ROWS,tt),A.pixelStorei(J.UNPACK_SKIP_IMAGES,Xe);let js=F.isDataArrayTexture||F.isData3DTexture,Bt=te.isDataArrayTexture||te.isData3DTexture;if(F.isDepthTexture){let Jt=de.get(F),Yi=de.get(te),Vt=de.get(Jt.__renderTarget),Ji=de.get(Yi.__renderTarget);A.bindFramebuffer(J.READ_FRAMEBUFFER,Vt.__webglFramebuffer),A.bindFramebuffer(J.DRAW_FRAMEBUFFER,Ji.__webglFramebuffer);for(let Ks=0;Ks<Ge;Ks++)js&&(J.framebufferTextureLayer(J.READ_FRAMEBUFFER,J.COLOR_ATTACHMENT0,de.get(F).__webglTexture,$,Xe+Ks),J.framebufferTextureLayer(J.DRAW_FRAMEBUFFER,J.COLOR_ATTACHMENT0,de.get(te).__webglTexture,De,Qe+Ks)),J.blitFramebuffer(He,tt,ze,Ne,Ce,et,ze,Ne,J.DEPTH_BUFFER_BIT,J.NEAREST);A.bindFramebuffer(J.READ_FRAMEBUFFER,null),A.bindFramebuffer(J.DRAW_FRAMEBUFFER,null)}else if($!==0||F.isRenderTargetTexture||de.has(F)){let Jt=de.get(F),Yi=de.get(te);A.bindFramebuffer(J.READ_FRAMEBUFFER,D),A.bindFramebuffer(J.DRAW_FRAMEBUFFER,R);for(let Vt=0;Vt<Ge;Vt++)js?J.framebufferTextureLayer(J.READ_FRAMEBUFFER,J.COLOR_ATTACHMENT0,Jt.__webglTexture,$,Xe+Vt):J.framebufferTexture2D(J.READ_FRAMEBUFFER,J.COLOR_ATTACHMENT0,J.TEXTURE_2D,Jt.__webglTexture,$),Bt?J.framebufferTextureLayer(J.DRAW_FRAMEBUFFER,J.COLOR_ATTACHMENT0,Yi.__webglTexture,De,Qe+Vt):J.framebufferTexture2D(J.DRAW_FRAMEBUFFER,J.COLOR_ATTACHMENT0,J.TEXTURE_2D,Yi.__webglTexture,De),$!==0?J.blitFramebuffer(He,tt,ze,Ne,Ce,et,ze,Ne,J.COLOR_BUFFER_BIT,J.NEAREST):Bt?J.copyTexSubImage3D($e,De,Ce,et,Qe+Vt,He,tt,ze,Ne):J.copyTexSubImage2D($e,De,Ce,et,He,tt,ze,Ne);A.bindFramebuffer(J.READ_FRAMEBUFFER,null),A.bindFramebuffer(J.DRAW_FRAMEBUFFER,null)}else Bt?F.isDataTexture||F.isData3DTexture?J.texSubImage3D($e,De,Ce,et,Qe,ze,Ne,Ge,Ct,Wt,ft.data):te.isCompressedArrayTexture?J.compressedTexSubImage3D($e,De,Ce,et,Qe,ze,Ne,Ge,Ct,ft.data):J.texSubImage3D($e,De,Ce,et,Qe,ze,Ne,Ge,Ct,Wt,ft):F.isDataTexture?J.texSubImage2D(J.TEXTURE_2D,De,Ce,et,ze,Ne,Ct,Wt,ft.data):F.isCompressedTexture?J.compressedTexSubImage2D(J.TEXTURE_2D,De,Ce,et,ft.width,ft.height,Ct,ft.data):J.texSubImage2D(J.TEXTURE_2D,De,Ce,et,ze,Ne,Ct,Wt,ft);A.pixelStorei(J.UNPACK_ROW_LENGTH,Mn),A.pixelStorei(J.UNPACK_IMAGE_HEIGHT,It),A.pixelStorei(J.UNPACK_SKIP_PIXELS,Vn),A.pixelStorei(J.UNPACK_SKIP_ROWS,ui),A.pixelStorei(J.UNPACK_SKIP_IMAGES,Ki),De===0&&te.generateMipmaps&&J.generateMipmap($e),A.unbindTexture()},this.initRenderTarget=function(F){de.get(F).__webglFramebuffer===void 0&&he.setupRenderTarget(F)},this.initTexture=function(F){F.isCubeTexture?he.setTextureCube(F,0):F.isData3DTexture?he.setTexture3D(F,0):F.isDataArrayTexture||F.isCompressedArrayTexture?he.setTexture2DArray(F,0):he.setTexture2D(F,0),A.unbindTexture()},this.resetState=function(){U=0,z=0,T=null,A.reset(),Re.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return ni}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(e){this._outputColorSpace=e;let t=this.getContext();t.drawingBufferColorSpace=_t._getDrawingBufferColorSpace(e),t.unpackColorSpace=_t._getUnpackColorSpace()}};function Zh(s,e){if(e===Ah)return console.warn("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Geometry already defined as triangles."),s;if(e===Ur||e===Ya){let t=s.getIndex();if(t===null){let r=[],a=s.getAttribute("position");if(a!==void 0){for(let o=0;o<a.count;o++)r.push(o);s.setIndex(r),t=s.getIndex()}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Undefined position attribute. Processing not possible."),s}let n=t.count-2,i=[];if(e===Ur)for(let r=1;r<=n;r++)i.push(t.getX(0)),i.push(t.getX(r)),i.push(t.getX(r+1));else for(let r=0;r<n;r++)r%2===0?(i.push(t.getX(r)),i.push(t.getX(r+1)),i.push(t.getX(r+2))):(i.push(t.getX(r+2)),i.push(t.getX(r+1)),i.push(t.getX(r)));return i.length/3!==n&&console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unable to generate correct amount of triangles."),s.setIndex(i),s.clearGroups(),s}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unknown draw mode:",e),s}function Zf(s){let e=new Map,t=new Map,n=s.clone();return $f(s,n,function(i,r){e.set(r,i),t.set(i,r)}),n.traverse(function(i){if(!i.isSkinnedMesh)return;let r=i,a=e.get(i),o=a.skeleton.bones;r.skeleton=a.skeleton.clone(),r.bindMatrix.copy(a.bindMatrix),r.skeleton.bones=o.map(function(l){return t.get(l)}),r.bind(r.skeleton,r.bindMatrix)}),n}function $f(s,e,t){t(s,e);for(let n=0;n<s.children.length;n++)$f(s.children[n],e.children[n],t)}var Ti=class extends yi{constructor(e){super(e),this.dracoLoader=null,this.ktx2Loader=null,this.meshoptDecoder=null,this.pluginCallbacks=[],this.register(function(t){return new su(t)}),this.register(function(t){return new ru(t)}),this.register(function(t){return new pu(t)}),this.register(function(t){return new mu(t)}),this.register(function(t){return new gu(t)}),this.register(function(t){return new ou(t)}),this.register(function(t){return new lu(t)}),this.register(function(t){return new cu(t)}),this.register(function(t){return new hu(t)}),this.register(function(t){return new iu(t)}),this.register(function(t){return new uu(t)}),this.register(function(t){return new au(t)}),this.register(function(t){return new fu(t)}),this.register(function(t){return new du(t)}),this.register(function(t){return new tu(t)}),this.register(function(t){return new pc(t,wt.EXT_MESHOPT_COMPRESSION)}),this.register(function(t){return new pc(t,wt.KHR_MESHOPT_COMPRESSION)}),this.register(function(t){return new bu(t)})}load(e,t,n,i){let r=this,a;if(this.resourcePath!=="")a=this.resourcePath;else if(this.path!==""){let c=Vi.extractUrlBase(e);a=Vi.resolveURL(c,this.path)}else a=Vi.extractUrlBase(e);this.manager.itemStart(e);let o=function(c){i?i(c):console.error(c),r.manager.itemError(e),r.manager.itemEnd(e)},l=new Rr(this.manager);l.setPath(this.path),l.setResponseType("arraybuffer"),l.setRequestHeader(this.requestHeader),l.setWithCredentials(this.withCredentials),l.load(e,function(c){try{r.parse(c,a,function(h){t(h),r.manager.itemEnd(e)},o)}catch(h){o(h)}},n,o)}setDRACOLoader(e){return this.dracoLoader=e,this}setKTX2Loader(e){return this.ktx2Loader=e,this}setMeshoptDecoder(e){return this.meshoptDecoder=e,this}register(e){return this.pluginCallbacks.indexOf(e)===-1&&this.pluginCallbacks.push(e),this}unregister(e){return this.pluginCallbacks.indexOf(e)!==-1&&this.pluginCallbacks.splice(this.pluginCallbacks.indexOf(e),1),this}parse(e,t,n,i){let r,a={},o={},l=new TextDecoder;if(typeof e=="string")r=JSON.parse(e);else if(e instanceof ArrayBuffer)if(l.decode(new Uint8Array(e,0,4))===ip){try{a[wt.KHR_BINARY_GLTF]=new xu(e)}catch(u){i&&i(u);return}r=JSON.parse(a[wt.KHR_BINARY_GLTF].content)}else r=JSON.parse(l.decode(e));else r=e;if(r.asset===void 0||r.asset.version[0]<2){i&&i(new Error("THREE.GLTFLoader: Unsupported asset. glTF versions >=2.0 are supported."));return}let c=new Eu(r,{path:t||this.resourcePath||"",crossOrigin:this.crossOrigin,requestHeader:this.requestHeader,manager:this.manager,ktx2Loader:this.ktx2Loader,meshoptDecoder:this.meshoptDecoder});c.fileLoader.setRequestHeader(this.requestHeader);for(let h=0;h<this.pluginCallbacks.length;h++){let u=this.pluginCallbacks[h](c);u.name||console.error("THREE.GLTFLoader: Invalid plugin found: missing name"),o[u.name]=u,a[u.name]=!0}if(r.extensionsUsed)for(let h=0;h<r.extensionsUsed.length;++h){let u=r.extensionsUsed[h],d=r.extensionsRequired||[];switch(u){case wt.KHR_MATERIALS_UNLIT:a[u]=new nu;break;case wt.KHR_DRACO_MESH_COMPRESSION:a[u]=new vu(r,this.dracoLoader);break;case wt.KHR_TEXTURE_TRANSFORM:a[u]=new _u;break;case wt.KHR_MESH_QUANTIZATION:a[u]=new yu;break;default:d.indexOf(u)>=0&&o[u]===void 0&&console.warn('THREE.GLTFLoader: Unknown extension "'+u+'".')}}c.setExtensions(a),c.setPlugins(o),c.parse(n,i)}parseAsync(e,t){let n=this;return new Promise(function(i,r){n.parse(e,t,i,r)})}};function s_(){let s={};return{get:function(e){return s[e]},add:function(e,t){s[e]=t},remove:function(e){delete s[e]},removeAll:function(){s={}}}}function tn(s,e,t){let n=s.json.materials[e];return n.extensions&&n.extensions[t]?n.extensions[t]:null}var wt={KHR_BINARY_GLTF:"KHR_binary_glTF",KHR_DRACO_MESH_COMPRESSION:"KHR_draco_mesh_compression",KHR_LIGHTS_PUNCTUAL:"KHR_lights_punctual",KHR_MATERIALS_CLEARCOAT:"KHR_materials_clearcoat",KHR_MATERIALS_DISPERSION:"KHR_materials_dispersion",KHR_MATERIALS_IOR:"KHR_materials_ior",KHR_MATERIALS_SHEEN:"KHR_materials_sheen",KHR_MATERIALS_SPECULAR:"KHR_materials_specular",KHR_MATERIALS_TRANSMISSION:"KHR_materials_transmission",KHR_MATERIALS_IRIDESCENCE:"KHR_materials_iridescence",KHR_MATERIALS_ANISOTROPY:"KHR_materials_anisotropy",KHR_MATERIALS_UNLIT:"KHR_materials_unlit",KHR_MATERIALS_VOLUME:"KHR_materials_volume",KHR_TEXTURE_BASISU:"KHR_texture_basisu",KHR_TEXTURE_TRANSFORM:"KHR_texture_transform",KHR_MESH_QUANTIZATION:"KHR_mesh_quantization",KHR_MATERIALS_EMISSIVE_STRENGTH:"KHR_materials_emissive_strength",EXT_MATERIALS_BUMP:"EXT_materials_bump",EXT_TEXTURE_WEBP:"EXT_texture_webp",EXT_TEXTURE_AVIF:"EXT_texture_avif",EXT_MESHOPT_COMPRESSION:"EXT_meshopt_compression",KHR_MESHOPT_COMPRESSION:"KHR_meshopt_compression",EXT_MESH_GPU_INSTANCING:"EXT_mesh_gpu_instancing"},tu=class{constructor(e){this.parser=e,this.name=wt.KHR_LIGHTS_PUNCTUAL,this.cache={refs:{},uses:{}}}_markDefs(){let e=this.parser,t=this.parser.json.nodes||[];for(let n=0,i=t.length;n<i;n++){let r=t[n];r.extensions&&r.extensions[this.name]&&r.extensions[this.name].light!==void 0&&e._addNodeRef(this.cache,r.extensions[this.name].light)}}_loadLight(e){let t=this.parser,n="light:"+e,i=t.cache.get(n);if(i)return i;let r=t.json,l=((r.extensions&&r.extensions[this.name]||{}).lights||[])[e],c,h=new Le(16777215);l.color!==void 0&&h.setRGB(l.color[0],l.color[1],l.color[2],wn);let u=l.range!==void 0?l.range:0;switch(l.type){case"directional":c=new Gi(h),c.target.position.set(0,0,-1),c.add(c.target);break;case"point":c=new ri(h),c.distance=u;break;case"spot":c=new Pa(h),c.distance=u,l.spot=l.spot||{},l.spot.innerConeAngle=l.spot.innerConeAngle!==void 0?l.spot.innerConeAngle:0,l.spot.outerConeAngle=l.spot.outerConeAngle!==void 0?l.spot.outerConeAngle:Math.PI/4,c.angle=l.spot.outerConeAngle,c.penumbra=1-l.spot.innerConeAngle/l.spot.outerConeAngle,c.target.position.set(0,0,-1),c.add(c.target);break;default:throw new Error("THREE.GLTFLoader: Unexpected light type: "+l.type)}return c.position.set(0,0,0),Ei(c,l),l.intensity!==void 0&&(c.intensity=l.intensity),c.name=t.createUniqueName(l.name||"light_"+e),i=Promise.resolve(c),t.cache.add(n,i),i}getDependency(e,t){if(e==="light")return this._loadLight(t)}createNodeAttachment(e){let t=this,n=this.parser,r=n.json.nodes[e],o=(r.extensions&&r.extensions[this.name]||{}).light;return o===void 0?null:this._loadLight(o).then(function(l){return n._getNodeRef(t.cache,o,l)})}},nu=class{constructor(){this.name=wt.KHR_MATERIALS_UNLIT}getMaterialType(){return Lt}extendParams(e,t,n){let i=[];e.color=new Le(1,1,1),e.opacity=1;let r=t.pbrMetallicRoughness;if(r){if(Array.isArray(r.baseColorFactor)){let a=r.baseColorFactor;e.color.setRGB(a[0],a[1],a[2],wn),e.opacity=a[3]}r.baseColorTexture!==void 0&&i.push(n.assignTexture(e,"map",r.baseColorTexture,Xt))}return Promise.all(i)}},iu=class{constructor(e){this.parser=e,this.name=wt.KHR_MATERIALS_EMISSIVE_STRENGTH}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);return n===null||n.emissiveStrength!==void 0&&(t.emissiveIntensity=n.emissiveStrength),Promise.resolve()}},su=class{constructor(e){this.parser=e,this.name=wt.KHR_MATERIALS_CLEARCOAT}getMaterialType(e){return tn(this.parser,e,this.name)!==null?Cn:null}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(n.clearcoatFactor!==void 0&&(t.clearcoat=n.clearcoatFactor),n.clearcoatTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatMap",n.clearcoatTexture)),n.clearcoatRoughnessFactor!==void 0&&(t.clearcoatRoughness=n.clearcoatRoughnessFactor),n.clearcoatRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatRoughnessMap",n.clearcoatRoughnessTexture)),n.clearcoatNormalTexture!==void 0&&(i.push(this.parser.assignTexture(t,"clearcoatNormalMap",n.clearcoatNormalTexture)),n.clearcoatNormalTexture.scale!==void 0)){let r=n.clearcoatNormalTexture.scale;t.clearcoatNormalScale=new Be(r,r)}return Promise.all(i)}},ru=class{constructor(e){this.parser=e,this.name=wt.KHR_MATERIALS_DISPERSION}getMaterialType(e){return tn(this.parser,e,this.name)!==null?Cn:null}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);return n===null||(t.dispersion=n.dispersion!==void 0?n.dispersion:0),Promise.resolve()}},au=class{constructor(e){this.parser=e,this.name=wt.KHR_MATERIALS_IRIDESCENCE}getMaterialType(e){return tn(this.parser,e,this.name)!==null?Cn:null}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.iridescenceFactor!==void 0&&(t.iridescence=n.iridescenceFactor),n.iridescenceTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceMap",n.iridescenceTexture)),n.iridescenceIor!==void 0&&(t.iridescenceIOR=n.iridescenceIor),t.iridescenceThicknessRange===void 0&&(t.iridescenceThicknessRange=[100,400]),n.iridescenceThicknessMinimum!==void 0&&(t.iridescenceThicknessRange[0]=n.iridescenceThicknessMinimum),n.iridescenceThicknessMaximum!==void 0&&(t.iridescenceThicknessRange[1]=n.iridescenceThicknessMaximum),n.iridescenceThicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceThicknessMap",n.iridescenceThicknessTexture)),Promise.all(i)}},ou=class{constructor(e){this.parser=e,this.name=wt.KHR_MATERIALS_SHEEN}getMaterialType(e){return tn(this.parser,e,this.name)!==null?Cn:null}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(t.sheenColor=new Le(0,0,0),t.sheenRoughness=0,t.sheen=1,n.sheenColorFactor!==void 0){let r=n.sheenColorFactor;t.sheenColor.setRGB(r[0],r[1],r[2],wn)}return n.sheenRoughnessFactor!==void 0&&(t.sheenRoughness=n.sheenRoughnessFactor),n.sheenColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenColorMap",n.sheenColorTexture,Xt)),n.sheenRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenRoughnessMap",n.sheenRoughnessTexture)),Promise.all(i)}},lu=class{constructor(e){this.parser=e,this.name=wt.KHR_MATERIALS_TRANSMISSION}getMaterialType(e){return tn(this.parser,e,this.name)!==null?Cn:null}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.transmissionFactor!==void 0&&(t.transmission=n.transmissionFactor),n.transmissionTexture!==void 0&&i.push(this.parser.assignTexture(t,"transmissionMap",n.transmissionTexture)),Promise.all(i)}},cu=class{constructor(e){this.parser=e,this.name=wt.KHR_MATERIALS_VOLUME}getMaterialType(e){return tn(this.parser,e,this.name)!==null?Cn:null}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.thickness=n.thicknessFactor!==void 0?n.thicknessFactor:0,n.thicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"thicknessMap",n.thicknessTexture)),t.attenuationDistance=n.attenuationDistance||1/0;let r=n.attenuationColor||[1,1,1];return t.attenuationColor=new Le().setRGB(r[0],r[1],r[2],wn),Promise.all(i)}},hu=class{constructor(e){this.parser=e,this.name=wt.KHR_MATERIALS_IOR}getMaterialType(e){return tn(this.parser,e,this.name)!==null?Cn:null}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);return n===null||(t.ior=n.ior!==void 0?n.ior:1.5,t.ior===0&&(t.ior=1e3)),Promise.resolve()}},uu=class{constructor(e){this.parser=e,this.name=wt.KHR_MATERIALS_SPECULAR}getMaterialType(e){return tn(this.parser,e,this.name)!==null?Cn:null}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.specularIntensity=n.specularFactor!==void 0?n.specularFactor:1,n.specularTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularIntensityMap",n.specularTexture));let r=n.specularColorFactor||[1,1,1];return t.specularColor=new Le().setRGB(r[0],r[1],r[2],wn),n.specularColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularColorMap",n.specularColorTexture,Xt)),Promise.all(i)}},du=class{constructor(e){this.parser=e,this.name=wt.EXT_MATERIALS_BUMP}getMaterialType(e){return tn(this.parser,e,this.name)!==null?Cn:null}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return t.bumpScale=n.bumpFactor!==void 0?n.bumpFactor:1,n.bumpTexture!==void 0&&i.push(this.parser.assignTexture(t,"bumpMap",n.bumpTexture)),Promise.all(i)}},fu=class{constructor(e){this.parser=e,this.name=wt.KHR_MATERIALS_ANISOTROPY}getMaterialType(e){return tn(this.parser,e,this.name)!==null?Cn:null}extendMaterialParams(e,t){let n=tn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.anisotropyStrength!==void 0&&(t.anisotropy=n.anisotropyStrength),n.anisotropyRotation!==void 0&&(t.anisotropyRotation=n.anisotropyRotation),n.anisotropyTexture!==void 0&&i.push(this.parser.assignTexture(t,"anisotropyMap",n.anisotropyTexture)),Promise.all(i)}},pu=class{constructor(e){this.parser=e,this.name=wt.KHR_TEXTURE_BASISU}loadTexture(e){let t=this.parser,n=t.json,i=n.textures[e];if(!i.extensions||!i.extensions[this.name])return null;let r=i.extensions[this.name],a=t.options.ktx2Loader;if(!a){if(n.extensionsRequired&&n.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setKTX2Loader must be called before loading KTX2 textures");return null}return t.loadTextureImage(e,r.source,a)}},mu=class{constructor(e){this.parser=e,this.name=wt.EXT_TEXTURE_WEBP}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let a=r.extensions[t],o=i.images[a.source],l=n.textureLoader;if(o.uri){let c=n.options.manager.getHandler(o.uri);c!==null&&(l=c)}return n.loadTextureImage(e,a.source,l)}},gu=class{constructor(e){this.parser=e,this.name=wt.EXT_TEXTURE_AVIF}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let a=r.extensions[t],o=i.images[a.source],l=n.textureLoader;if(o.uri){let c=n.options.manager.getHandler(o.uri);c!==null&&(l=c)}return n.loadTextureImage(e,a.source,l)}},pc=class{constructor(e,t){this.name=t,this.parser=e}loadBufferView(e){let t=this.parser.json,n=t.bufferViews[e];if(n.extensions&&n.extensions[this.name]){let i=n.extensions[this.name],r=this.parser.getDependency("buffer",i.buffer),a=this.parser.options.meshoptDecoder;if(!a||!a.supported){if(t.extensionsRequired&&t.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setMeshoptDecoder must be called before loading compressed files");return null}return r.then(function(o){let l=i.byteOffset||0,c=i.byteLength||0,h=i.count,u=i.byteStride,d=new Uint8Array(o,l,c);return a.decodeGltfBufferAsync?a.decodeGltfBufferAsync(h,u,d,i.mode,i.filter).then(function(p){return p.buffer}):a.ready.then(function(){let p=new ArrayBuffer(h*u);return a.decodeGltfBuffer(new Uint8Array(p),h,u,d,i.mode,i.filter),p})})}else return null}},bu=class{constructor(e){this.name=wt.EXT_MESH_GPU_INSTANCING,this.parser=e}createNodeMesh(e){let t=this.parser.json,n=t.nodes[e];if(!n.extensions||!n.extensions[this.name]||n.mesh===void 0)return null;let i=t.meshes[n.mesh];for(let c of i.primitives)if(c.mode!==Jn.TRIANGLES&&c.mode!==Jn.TRIANGLE_STRIP&&c.mode!==Jn.TRIANGLE_FAN&&c.mode!==void 0)return null;let a=n.extensions[this.name].attributes,o=[],l={};for(let c in a)o.push(this.parser.getDependency("accessor",a[c]).then(h=>(l[c]=h,l[c])));return o.length<1?null:(o.push(this.parser.createNodeMesh(e)),Promise.all(o).then(c=>{let h=c.pop(),u=h.isGroup?h.children:[h],d=c[0].count,p=[];for(let g of u){let _=new pt,m=new k,f=new Zt,v=new k(1,1,1),S=new An(g.geometry,g.material,d);for(let M=0;M<d;M++)l.TRANSLATION&&m.fromBufferAttribute(l.TRANSLATION,M),l.ROTATION&&f.fromBufferAttribute(l.ROTATION,M),l.SCALE&&v.fromBufferAttribute(l.SCALE,M),S.setMatrixAt(M,_.compose(m,f,v));let b=null;for(let M in l)if(M==="_COLOR_0"){let E=l[M];S.instanceColor=new Ui(E.array,E.itemSize,E.normalized)}else if(M!=="TRANSLATION"&&M!=="ROTATION"&&M!=="SCALE"){if(b===null){let P=S.geometry;b=new bt,b.name=P.name;for(let x in P.attributes)b.setAttribute(x,P.attributes[x]);for(let x in P.morphAttributes)b.morphAttributes[x]=P.morphAttributes[x];P.index!==null&&b.setIndex(P.index),b.morphTargetsRelative=P.morphTargetsRelative;for(let x of P.groups)b.addGroup(x.start,x.count,x.materialIndex);P.boundingBox!==null&&(b.boundingBox=P.boundingBox.clone()),P.boundingSphere!==null&&(b.boundingSphere=P.boundingSphere.clone()),b.drawRange.start=P.drawRange.start,b.drawRange.count=P.drawRange.count,b.userData=Object.assign({},P.userData),S.geometry=b}let E=l[M];b.setAttribute(M,new Ui(E.array,E.itemSize,E.normalized))}Ft.prototype.copy.call(S,g),this.parser.assignFinalMaterial(S),p.push(S)}return h.isGroup?(h.clear(),h.add(...p),h):p[0]}))}},ip="glTF",eo=12,Qf={JSON:1313821514,BIN:5130562},xu=class{constructor(e){this.name=wt.KHR_BINARY_GLTF,this.content=null,this.body=null;let t=new DataView(e,0,eo),n=new TextDecoder;if(this.header={magic:n.decode(new Uint8Array(e.slice(0,4))),version:t.getUint32(4,!0),length:t.getUint32(8,!0)},this.header.magic!==ip)throw new Error("THREE.GLTFLoader: Unsupported glTF-Binary header.");if(this.header.version<2)throw new Error("THREE.GLTFLoader: Legacy binary file detected.");let i=this.header.length-eo,r=new DataView(e,eo),a=0;for(;a<i;){let o=r.getUint32(a,!0);a+=4;let l=r.getUint32(a,!0);if(a+=4,l===Qf.JSON){let c=new Uint8Array(e,eo+a,o);this.content=n.decode(c)}else if(l===Qf.BIN){let c=eo+a;this.body=e.slice(c,c+o)}a+=o}if(this.content===null)throw new Error("THREE.GLTFLoader: JSON content not found.")}},vu=class{constructor(e,t){if(!t)throw new Error("THREE.GLTFLoader: No DRACOLoader instance provided.");this.name=wt.KHR_DRACO_MESH_COMPRESSION,this.json=e,this.dracoLoader=t,this.dracoLoader.preload()}decodePrimitive(e,t){let n=this.json,i=this.dracoLoader,r=e.extensions[this.name].bufferView,a=e.extensions[this.name].attributes,o={},l={},c={};for(let h in a){let u=Su[h]||h.toLowerCase();o[u]=a[h]}for(let h in e.attributes){let u=Su[h]||h.toLowerCase();if(a[h]!==void 0){let d=n.accessors[e.attributes[h]],p=Gr[d.componentType];c[u]=p.name,l[u]=d.normalized===!0}}return t.getDependency("bufferView",r).then(function(h){return new Promise(function(u,d){i.decodeDracoFile(h,function(p){for(let g in p.attributes){let _=p.attributes[g],m=l[g];m!==void 0&&(_.normalized=m)}u(p)},o,c,wn,d)})})}},_u=class{constructor(){this.name=wt.KHR_TEXTURE_TRANSFORM}extendTexture(e,t){if((t.texCoord===void 0||t.texCoord===e.channel)&&t.offset===void 0&&t.rotation===void 0&&t.scale===void 0)return e;if(e=e.clone(),t.texCoord!==void 0&&(e.channel=t.texCoord),t.offset!==void 0&&e.offset.fromArray(t.offset),t.rotation!==void 0&&(e.rotation=t.rotation),t.scale!==void 0&&e.repeat.fromArray(t.scale),t.rotation!==void 0){let n=Math.cos(e.rotation),i=Math.sin(e.rotation);e.matrix.set(e.repeat.x*n,e.repeat.y*i,e.offset.x,-e.repeat.x*i,e.repeat.y*n,e.offset.y,0,0,1),e.matrixAutoUpdate=!1}return e.needsUpdate=!0,e}},yu=class{constructor(){this.name=wt.KHR_MESH_QUANTIZATION}},mc=class extends _i{constructor(e,t,n,i){super(e,t,n,i)}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i*3+i;for(let a=0;a!==i;a++)t[a]=n[r+a];return t}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=o*2,c=o*3,h=i-t,u=(n-t)/h,d=u*u,p=d*u,g=e*c,_=g-c,m=-2*p+3*d,f=p-d,v=1-m,S=f-d+u;for(let b=0;b!==o;b++){let M=a[_+b+o],E=a[_+b+l]*h,P=a[g+b+o],x=a[g+b]*h;r[b]=v*M+S*E+m*P+f*x}return r}},r_=new Zt,Mu=class extends mc{interpolate_(e,t,n,i){let r=super.interpolate_(e,t,n,i);return r_.fromArray(r).normalize().toArray(r),r}},Jn={FLOAT:5126,FLOAT_MAT3:35675,FLOAT_MAT4:35676,FLOAT_VEC2:35664,FLOAT_VEC3:35665,FLOAT_VEC4:35666,LINEAR:9729,REPEAT:10497,SAMPLER_2D:35678,POINTS:0,LINES:1,LINE_LOOP:2,LINE_STRIP:3,TRIANGLES:4,TRIANGLE_STRIP:5,TRIANGLE_FAN:6,UNSIGNED_BYTE:5121,UNSIGNED_SHORT:5123},Gr={5120:Int8Array,5121:Uint8Array,5122:Int16Array,5123:Uint16Array,5125:Uint32Array,5126:Float32Array},ep={9728:$t,9729:jt,9984:_l,9985:Lr,9986:Bs,9987:oi},tp={33071:qn,33648:ur,10497:pi},$h={SCALAR:1,VEC2:2,VEC3:3,VEC4:4,MAT2:4,MAT3:9,MAT4:16},Su={POSITION:"position",NORMAL:"normal",TANGENT:"tangent",TEXCOORD_0:"uv",TEXCOORD_1:"uv1",TEXCOORD_2:"uv2",TEXCOORD_3:"uv3",COLOR_0:"color",WEIGHTS_0:"skinWeight",JOINTS_0:"skinIndex"},ps={scale:"scale",translation:"position",rotation:"quaternion",weights:"morphTargetInfluences"},a_={CUBICSPLINE:void 0,LINEAR:As,STEP:Ts},Qh={OPAQUE:"OPAQUE",MASK:"MASK",BLEND:"BLEND"};function o_(s){return s.DefaultMaterial===void 0&&(s.DefaultMaterial=new Rn({color:16777215,emissive:0,metalness:1,roughness:1,transparent:!1,depthTest:!0,side:jn})),s.DefaultMaterial}function Gs(s,e,t){for(let n in t.extensions)s[n]===void 0&&(e.userData.gltfExtensions=e.userData.gltfExtensions||{},e.userData.gltfExtensions[n]=t.extensions[n])}function Ei(s,e){e.extras!==void 0&&(typeof e.extras=="object"?Object.assign(s.userData,e.extras):console.warn("THREE.GLTFLoader: Ignoring primitive type .extras, "+e.extras))}function l_(s,e,t){let n=!1,i=!1,r=!1;for(let c=0,h=e.length;c<h;c++){let u=e[c];if(u.POSITION!==void 0&&(n=!0),u.NORMAL!==void 0&&(i=!0),u.COLOR_0!==void 0&&(r=!0),n&&i&&r)break}if(!n&&!i&&!r)return Promise.resolve(s);let a=[],o=[],l=[];for(let c=0,h=e.length;c<h;c++){let u=e[c];if(n){let d=u.POSITION!==void 0?t.getDependency("accessor",u.POSITION):s.attributes.position;a.push(d)}if(i){let d=u.NORMAL!==void 0?t.getDependency("accessor",u.NORMAL):s.attributes.normal;o.push(d)}if(r){let d=u.COLOR_0!==void 0?t.getDependency("accessor",u.COLOR_0):s.attributes.color;l.push(d)}}return Promise.all([Promise.all(a),Promise.all(o),Promise.all(l)]).then(function(c){let h=c[0],u=c[1],d=c[2];return n&&(s.morphAttributes.position=h),i&&(s.morphAttributes.normal=u),r&&(s.morphAttributes.color=d),s.morphTargetsRelative=!0,s})}function c_(s,e){if(s.updateMorphTargets(),e.weights!==void 0)for(let t=0,n=e.weights.length;t<n;t++)s.morphTargetInfluences[t]=e.weights[t];if(e.extras&&Array.isArray(e.extras.targetNames)){let t=e.extras.targetNames;if(s.morphTargetInfluences.length===t.length){s.morphTargetDictionary={};for(let n=0,i=t.length;n<i;n++)s.morphTargetDictionary[t[n]]=n}else console.warn("THREE.GLTFLoader: Invalid extras.targetNames length. Ignoring names.")}}function h_(s){let e,t=s.extensions&&s.extensions[wt.KHR_DRACO_MESH_COMPRESSION];if(t?e="draco:"+t.bufferView+":"+t.indices+":"+eu(t.attributes):e=s.indices+":"+eu(s.attributes)+":"+s.mode,s.targets!==void 0)for(let n=0,i=s.targets.length;n<i;n++)e+=":"+eu(s.targets[n]);return e}function eu(s){let e="",t=Object.keys(s).sort();for(let n=0,i=t.length;n<i;n++)e+=t[n]+":"+s[t[n]]+";";return e}function wu(s){switch(s){case Int8Array:return 1/127;case Uint8Array:return 1/255;case Int16Array:return 1/32767;case Uint16Array:return 1/65535;default:throw new Error("THREE.GLTFLoader: Unsupported normalized accessor component type.")}}function u_(s){return s.search(/\.jpe?g($|\?)/i)>0||s.search(/^data\:image\/jpeg/)===0?"image/jpeg":s.search(/\.webp($|\?)/i)>0||s.search(/^data\:image\/webp/)===0?"image/webp":s.search(/\.ktx2($|\?)/i)>0||s.search(/^data\:image\/ktx2/)===0?"image/ktx2":"image/png"}var d_=new pt,Eu=class{constructor(e={},t={}){this.json=e,this.extensions={},this.plugins={},this.options=t,this.cache=new s_,this.associations=new Map,this.primitiveCache={},this.nodeCache={},this.meshCache={refs:{},uses:{}},this.cameraCache={refs:{},uses:{}},this.lightCache={refs:{},uses:{}},this.sourceCache={},this.textureCache={},this.nodeNamesUsed={};let n=!1,i=-1,r=!1,a=-1;if(typeof navigator<"u"&&typeof navigator.userAgent<"u"){let o=navigator.userAgent;n=/^((?!chrome|android).)*safari/i.test(o)===!0;let l=o.match(/Version\/(\d+)/);i=n&&l?parseInt(l[1],10):-1,r=o.indexOf("Firefox")>-1,a=r?o.match(/Firefox\/([0-9]+)\./)[1]:-1}typeof createImageBitmap>"u"||n&&i<17||r&&a<98?this.textureLoader=new Aa(this.options.manager):this.textureLoader=new Ia(this.options.manager),this.textureLoader.setCrossOrigin(this.options.crossOrigin),this.textureLoader.setRequestHeader(this.options.requestHeader),this.fileLoader=new Rr(this.options.manager),this.fileLoader.setResponseType("arraybuffer"),this.options.crossOrigin==="use-credentials"&&this.fileLoader.setWithCredentials(!0)}setExtensions(e){this.extensions=e}setPlugins(e){this.plugins=e}parse(e,t){let n=this,i=this.json,r=this.extensions;this.cache.removeAll(),this.nodeCache={},this._invokeAll(function(a){return a._markDefs&&a._markDefs()}),Promise.all(this._invokeAll(function(a){return a.beforeRoot&&a.beforeRoot()})).then(function(){return Promise.all([n.getDependencies("scene"),n.getDependencies("animation"),n.getDependencies("camera")])}).then(function(a){let o={scene:a[0][i.scene||0],scenes:a[0],animations:a[1],cameras:a[2],asset:i.asset,parser:n,userData:{}};return Gs(r,o,i),Ei(o,i),Promise.all(n._invokeAll(function(l){return l.afterRoot&&l.afterRoot(o)})).then(function(){for(let l of o.scenes)l.updateMatrixWorld();e(o)})}).catch(t)}_markDefs(){let e=this.json.nodes||[],t=this.json.skins||[],n=this.json.meshes||[];for(let i=0,r=t.length;i<r;i++){let a=t[i].joints;for(let o=0,l=a.length;o<l;o++)e[a[o]].isBone=!0}for(let i=0,r=e.length;i<r;i++){let a=e[i];a.mesh!==void 0&&(this._addNodeRef(this.meshCache,a.mesh),a.skin!==void 0&&(n[a.mesh].isSkinnedMesh=!0)),a.camera!==void 0&&this._addNodeRef(this.cameraCache,a.camera)}}_addNodeRef(e,t){t!==void 0&&(e.refs[t]===void 0&&(e.refs[t]=e.uses[t]=0),e.refs[t]++)}_getNodeRef(e,t,n){if(e.refs[t]<=1)return n;let i=n.clone(),r=(a,o)=>{let l=this.associations.get(a);l!=null&&this.associations.set(o,l);for(let[c,h]of a.children.entries())r(h,o.children[c])};return r(n,i),i.name+="_instance_"+e.uses[t]++,i}_invokeOne(e){let t=Object.values(this.plugins);t.push(this);for(let n=0;n<t.length;n++){let i=e(t[n]);if(i)return i}return null}_invokeAll(e){let t=Object.values(this.plugins);t.unshift(this);let n=[];for(let i=0;i<t.length;i++){let r=e(t[i]);r&&n.push(r)}return n}getDependency(e,t){let n=e+":"+t,i=this.cache.get(n);if(!i){switch(e){case"scene":i=this.loadScene(t);break;case"node":i=this._invokeOne(function(r){return r.loadNode&&r.loadNode(t)});break;case"mesh":i=this._invokeOne(function(r){return r.loadMesh&&r.loadMesh(t)});break;case"accessor":i=this.loadAccessor(t);break;case"bufferView":i=this._invokeOne(function(r){return r.loadBufferView&&r.loadBufferView(t)});break;case"buffer":i=this.loadBuffer(t);break;case"material":i=this._invokeOne(function(r){return r.loadMaterial&&r.loadMaterial(t)});break;case"texture":i=this._invokeOne(function(r){return r.loadTexture&&r.loadTexture(t)});break;case"skin":i=this.loadSkin(t);break;case"animation":i=this._invokeOne(function(r){return r.loadAnimation&&r.loadAnimation(t)});break;case"camera":i=this.loadCamera(t);break;default:if(i=this._invokeOne(function(r){return r!=this&&r.getDependency&&r.getDependency(e,t)}),!i)throw new Error("Unknown type: "+e);break}this.cache.add(n,i)}return i}getDependencies(e){let t=this.cache.get(e);if(!t){let n=this,i=this.json[e+(e==="mesh"?"es":"s")]||[];t=Promise.all(i.map(function(r,a){return n.getDependency(e,a)})),this.cache.add(e,t)}return t}loadBuffer(e){let t=this.json.buffers[e],n=this.fileLoader;if(t.type&&t.type!=="arraybuffer")throw new Error("THREE.GLTFLoader: "+t.type+" buffer type is not supported.");if(t.uri===void 0&&e===0)return Promise.resolve(this.extensions[wt.KHR_BINARY_GLTF].body);let i=this.options;return new Promise(function(r,a){n.load(Vi.resolveURL(t.uri,i.path),r,void 0,function(){a(new Error('THREE.GLTFLoader: Failed to load buffer "'+t.uri+'".'))})})}loadBufferView(e){let t=this.json.bufferViews[e];return this.getDependency("buffer",t.buffer).then(function(n){let i=t.byteLength||0,r=t.byteOffset||0;return n.slice(r,r+i)})}loadAccessor(e){let t=this,n=this.json,i=this.json.accessors[e];if(i.bufferView===void 0&&i.sparse===void 0){let a=$h[i.type],o=Gr[i.componentType],l=i.normalized===!0,c=new o(i.count*a);return Promise.resolve(new Mt(c,a,l))}let r=[];return i.bufferView!==void 0?r.push(this.getDependency("bufferView",i.bufferView)):r.push(null),i.sparse!==void 0&&(r.push(this.getDependency("bufferView",i.sparse.indices.bufferView)),r.push(this.getDependency("bufferView",i.sparse.values.bufferView))),Promise.all(r).then(function(a){let o=a[0],l=$h[i.type],c=Gr[i.componentType],h=c.BYTES_PER_ELEMENT,u=h*l,d=i.byteOffset||0,p=i.bufferView!==void 0?n.bufferViews[i.bufferView].byteStride:void 0,g=i.normalized===!0,_,m;if(p&&p!==u){let f=Math.floor(d/p),v="InterleavedBuffer:"+i.bufferView+":"+i.componentType+":"+f+":"+i.count,S=t.cache.get(v);S||(_=new c(o,f*p,i.count*p/h),S=new xr(_,p/h),t.cache.add(v,S)),m=new vr(S,l,d%p/h,g)}else o===null?_=new c(i.count*l):_=new c(o,d,i.count*l),m=new Mt(_,l,g);if(i.sparse!==void 0){let f=$h.SCALAR,v=Gr[i.sparse.indices.componentType],S=i.sparse.indices.byteOffset||0,b=i.sparse.values.byteOffset||0,M=new v(a[1],S,i.sparse.count*f),E=new c(a[2],b,i.sparse.count*l);o!==null&&(m=new Mt(m.array.slice(),m.itemSize,m.normalized)),m.normalized=!1;for(let P=0,x=M.length;P<x;P++){let C=M[P];if(m.setX(C,E[P*l]),l>=2&&m.setY(C,E[P*l+1]),l>=3&&m.setZ(C,E[P*l+2]),l>=4&&m.setW(C,E[P*l+3]),l>=5)throw new Error("THREE.GLTFLoader: Unsupported itemSize in sparse BufferAttribute.")}m.normalized=g}return m})}loadTexture(e){let t=this.json,n=this.options,r=t.textures[e].source,a=t.images[r],o=this.textureLoader;if(a.uri){let l=n.manager.getHandler(a.uri);l!==null&&(o=l)}return this.loadTextureImage(e,r,o)}loadTextureImage(e,t,n){let i=this,r=this.json,a=r.textures[e],o=r.images[t],l=(o.uri||o.bufferView)+":"+a.sampler;if(this.textureCache[l])return this.textureCache[l];let c=this.loadImageSource(t,n).then(function(h){h.flipY=!1,h.name=a.name||o.name||"",h.name===""&&typeof o.uri=="string"&&o.uri.startsWith("data:image/")===!1&&(h.name=o.uri);let d=(r.samplers||{})[a.sampler]||{};return h.magFilter=ep[d.magFilter]||jt,h.minFilter=ep[d.minFilter]||oi,h.wrapS=tp[d.wrapS]||pi,h.wrapT=tp[d.wrapT]||pi,h.generateMipmaps=!h.isCompressedTexture&&h.minFilter!==$t&&h.minFilter!==jt,i.associations.set(h,{textures:e}),h}).catch(function(){return null});return this.textureCache[l]=c,c}loadImageSource(e,t){let n=this,i=this.json,r=this.options;if(this.sourceCache[e]!==void 0)return this.sourceCache[e].then(u=>u.clone());let a=i.images[e],o=self.URL||self.webkitURL,l=a.uri||"",c=!1;if(a.bufferView!==void 0)l=n.getDependency("bufferView",a.bufferView).then(function(u){c=!0;let d=new Blob([u],{type:a.mimeType});return l=o.createObjectURL(d),l});else if(a.uri===void 0)throw new Error("THREE.GLTFLoader: Image "+e+" is missing URI and bufferView");let h=Promise.resolve(l).then(function(u){return new Promise(function(d,p){let g=d;t.isImageBitmapLoader===!0&&(g=function(_){let m=new Qt(_);m.needsUpdate=!0,d(m)}),t.load(Vi.resolveURL(u,r.path),g,void 0,p)})}).then(function(u){return c===!0&&o.revokeObjectURL(l),Ei(u,a),u.userData.mimeType=a.mimeType||u_(a.uri),u}).catch(function(u){throw console.error("THREE.GLTFLoader: Couldn't load texture",l),u});return this.sourceCache[e]=h,h}assignTexture(e,t,n,i){let r=this;return this.getDependency("texture",n.index).then(function(a){if(!a)return null;if(n.texCoord!==void 0&&n.texCoord>0&&(a=a.clone(),a.channel=n.texCoord),r.extensions[wt.KHR_TEXTURE_TRANSFORM]){let o=n.extensions!==void 0?n.extensions[wt.KHR_TEXTURE_TRANSFORM]:void 0;if(o){let l=r.associations.get(a);a=r.extensions[wt.KHR_TEXTURE_TRANSFORM].extendTexture(a,o),r.associations.set(a,l)}}return i!==void 0&&(a.colorSpace=i),e[t]=a,a})}assignFinalMaterial(e){let t=e.geometry,n=e.material,i=t.attributes.tangent===void 0,r=t.attributes.color!==void 0,a=t.attributes.normal===void 0;if(e.isPoints){let o="PointsMaterial:"+n.uuid,l=this.cache.get(o);l||(l=new Mr,En.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,l.sizeAttenuation=!1,this.cache.add(o,l)),n=l}else if(e.isLine){let o="LineBasicMaterial:"+n.uuid,l=this.cache.get(o);l||(l=new bi,En.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,this.cache.add(o,l)),n=l}if(i||r||a){let o="ClonedMaterial:"+n.uuid+":";i&&(o+="derivative-tangents:"),r&&(o+="vertex-colors:"),a&&(o+="flat-shading:");let l=this.cache.get(o);l||(l=n.clone(),r&&(l.vertexColors=!0),a&&(l.flatShading=!0),i&&(l.normalScale&&(l.normalScale.y*=-1),l.clearcoatNormalScale&&(l.clearcoatNormalScale.y*=-1)),this.cache.add(o,l),this.associations.set(l,this.associations.get(n))),n=l}e.material=n}getMaterialType(){return Rn}loadMaterial(e){let t=this,n=this.json,i=this.extensions,r=n.materials[e],a,o={},l=r.extensions||{},c=[];if(l[wt.KHR_MATERIALS_UNLIT]){let u=i[wt.KHR_MATERIALS_UNLIT];a=u.getMaterialType(),c.push(u.extendParams(o,r,t))}else{let u=r.pbrMetallicRoughness||{};if(o.color=new Le(1,1,1),o.opacity=1,Array.isArray(u.baseColorFactor)){let d=u.baseColorFactor;o.color.setRGB(d[0],d[1],d[2],wn),o.opacity=d[3]}u.baseColorTexture!==void 0&&c.push(t.assignTexture(o,"map",u.baseColorTexture,Xt)),o.metalness=u.metallicFactor!==void 0?u.metallicFactor:1,o.roughness=u.roughnessFactor!==void 0?u.roughnessFactor:1,u.metallicRoughnessTexture!==void 0&&(c.push(t.assignTexture(o,"metalnessMap",u.metallicRoughnessTexture)),c.push(t.assignTexture(o,"roughnessMap",u.metallicRoughnessTexture))),a=this._invokeOne(function(d){return d.getMaterialType&&d.getMaterialType(e)}),c.push(Promise.all(this._invokeAll(function(d){return d.extendMaterialParams&&d.extendMaterialParams(e,o)})))}r.doubleSided===!0&&(o.side=Ot);let h=r.alphaMode||Qh.OPAQUE;if(h===Qh.BLEND?(o.transparent=!0,o.depthWrite=!1):(o.transparent=!1,h===Qh.MASK&&(o.alphaTest=r.alphaCutoff!==void 0?r.alphaCutoff:.5)),r.normalTexture!==void 0&&a!==Lt&&(c.push(t.assignTexture(o,"normalMap",r.normalTexture)),o.normalScale=new Be(1,1),r.normalTexture.scale!==void 0)){let u=r.normalTexture.scale;o.normalScale.set(u,u)}if(r.occlusionTexture!==void 0&&a!==Lt&&(c.push(t.assignTexture(o,"aoMap",r.occlusionTexture)),r.occlusionTexture.strength!==void 0&&(o.aoMapIntensity=r.occlusionTexture.strength)),r.emissiveFactor!==void 0&&a!==Lt){let u=r.emissiveFactor;o.emissive=new Le().setRGB(u[0],u[1],u[2],wn)}return r.emissiveTexture!==void 0&&a!==Lt&&c.push(t.assignTexture(o,"emissiveMap",r.emissiveTexture,Xt)),Promise.all(c).then(function(){let u=new a(o);return r.name&&(u.name=r.name),Ei(u,r),t.associations.set(u,{materials:e}),r.extensions&&Gs(i,u,r),u})}createUniqueName(e){let t=kt.sanitizeNodeName(e||"");return t in this.nodeNamesUsed?t+"_"+ ++this.nodeNamesUsed[t]:(this.nodeNamesUsed[t]=0,t)}loadGeometries(e){let t=this,n=this.extensions,i=this.primitiveCache;function r(o){return n[wt.KHR_DRACO_MESH_COMPRESSION].decodePrimitive(o,t).then(function(l){return np(l,o,t)})}let a=[];for(let o=0,l=e.length;o<l;o++){let c=e[o],h=h_(c),u=i[h];if(u)a.push(u.promise);else{let d;c.extensions&&c.extensions[wt.KHR_DRACO_MESH_COMPRESSION]?d=r(c):d=np(new bt,c,t),c.mode===Jn.TRIANGLE_STRIP?d=d.then(p=>Zh(p,Ya)):c.mode===Jn.TRIANGLE_FAN&&(d=d.then(p=>Zh(p,Ur))),i[h]={primitive:c,promise:d},a.push(d)}}return Promise.all(a)}loadMesh(e){let t=this,n=this.json,i=this.extensions,r=n.meshes[e],a=r.primitives,o=[];for(let l=0,c=a.length;l<c;l++){let h=a[l].material===void 0?o_(this.cache):this.getDependency("material",a[l].material);o.push(h)}return o.push(t.loadGeometries(a)),Promise.all(o).then(async function(l){let c=l.slice(0,l.length-1),h=l[l.length-1],u=[];for(let p=0,g=h.length;p<g;p++){let _=h[p],m=a[p],f,v=c[p];if(m.mode===Jn.TRIANGLES||m.mode===Jn.TRIANGLE_STRIP||m.mode===Jn.TRIANGLE_FAN||m.mode===void 0){let S=r.isSkinnedMesh===!0,b=_.hasAttribute("skinIndex")&&_.hasAttribute("skinWeight");S&&b===!1&&console.warn("THREE.GLTFLoader: Missing skinIndex or skinWeight attributes. Skinning disabled."),f=S&&b?new ma(_,v):new it(_,v),f.isSkinnedMesh===!0&&f.normalizeSkinWeights()}else if(m.mode===Jn.LINES)f=new Ps(_,v);else if(m.mode===Jn.LINE_STRIP)f=new Oi(_,v);else if(m.mode===Jn.LINE_LOOP)f=new ba(_,v);else if(m.mode===Jn.POINTS)f=new fn(_,v);else throw new Error("THREE.GLTFLoader: Primitive mode unsupported: "+m.mode);Object.keys(f.geometry.morphAttributes).length>0&&c_(f,r),f.name=t.createUniqueName(r.name||"mesh_"+e),Ei(f,r),m.extensions&&Gs(i,f,m),t.assignFinalMaterial(f),u.push(f)}for(let p=0,g=u.length;p<g;p++)t.associations.set(u[p],{meshes:e,primitives:p});if(u.length===1)return r.extensions&&Gs(i,u[0],r),u[0];let d=new gt;r.extensions&&Gs(i,d,r),t.associations.set(d,{meshes:e});for(let p=0,g=u.length;p<g;p++)d.add(u[p]);return d})}loadCamera(e){let t,n=this.json.cameras[e],i=n[n.type];if(!i){console.warn("THREE.GLTFLoader: Missing camera parameters.");return}return n.type==="perspective"?t=new sn(Tt.radToDeg(i.yfov),i.aspectRatio||1,i.znear||1,i.zfar||2e6):n.type==="orthographic"&&(t=new Mi(-i.xmag,i.xmag,i.ymag,-i.ymag,i.znear,i.zfar)),n.name&&(t.name=this.createUniqueName(n.name)),Ei(t,n),Promise.resolve(t)}loadSkin(e){let t=this.json.skins[e],n=[];for(let i=0,r=t.joints.length;i<r;i++)n.push(this._loadNodeShallow(t.joints[i]));return t.inverseBindMatrices!==void 0?n.push(this.getDependency("accessor",t.inverseBindMatrices)):n.push(null),Promise.all(n).then(function(i){let r=i.pop(),a=i,o=[],l=[];for(let c=0,h=a.length;c<h;c++){let u=a[c];if(u){o.push(u);let d=new pt;r!==null&&d.fromArray(r.array,c*16),l.push(d)}else console.warn('THREE.GLTFLoader: Joint "%s" could not be found.',t.joints[c])}return new ga(o,l)})}loadAnimation(e){let t=this.json,n=this,i=t.animations[e],r=i.name?i.name:"animation_"+e,a=[],o=[],l=[],c=[],h=[];for(let u=0,d=i.channels.length;u<d;u++){let p=i.channels[u],g=i.samplers[p.sampler],_=p.target,m=_.node,f=i.parameters!==void 0?i.parameters[g.input]:g.input,v=i.parameters!==void 0?i.parameters[g.output]:g.output;_.node!==void 0&&(a.push(this.getDependency("node",m)),o.push(this.getDependency("accessor",f)),l.push(this.getDependency("accessor",v)),c.push(g),h.push(_))}return Promise.all([Promise.all(a),Promise.all(o),Promise.all(l),Promise.all(c),Promise.all(h)]).then(function(u){let d=u[0],p=u[1],g=u[2],_=u[3],m=u[4],f=[];for(let S=0,b=d.length;S<b;S++){let M=d[S],E=p[S],P=g[S],x=_[S],C=m[S];if(M===void 0)continue;M.updateMatrix&&M.updateMatrix();let w=n._createAnimationTracks(M,E,P,x,C);if(w)for(let I=0;I<w.length;I++)f.push(w[I])}let v=new Ls(r,void 0,f);return Ei(v,i),v})}createNodeMesh(e){let t=this.json,n=this,i=t.nodes[e];return i.mesh===void 0?null:n.getDependency("mesh",i.mesh).then(function(r){let a=n._getNodeRef(n.meshCache,i.mesh,r);return i.weights!==void 0&&a.traverse(function(o){if(o.isMesh)for(let l=0,c=i.weights.length;l<c;l++)o.morphTargetInfluences[l]=i.weights[l]}),a})}loadNode(e){let t=this.json,n=this,i=t.nodes[e],r=n._loadNodeShallow(e),a=[],o=i.children||[];for(let c=0,h=o.length;c<h;c++)a.push(n.getDependency("node",o[c]));let l=i.skin===void 0?Promise.resolve(null):n.getDependency("skin",i.skin);return Promise.all([r,Promise.all(a),l]).then(function(c){let h=c[0],u=c[1],d=c[2];d!==null&&h.traverse(function(p){p.isSkinnedMesh&&p.bind(d,d_)});for(let p=0,g=u.length;p<g;p++)h.add(u[p]);if(h.userData.pivot!==void 0&&u.length>0){let p=h.userData.pivot,g=u[0];h.pivot=new k().fromArray(p),h.position.x-=p[0],h.position.y-=p[1],h.position.z-=p[2],g.position.set(0,0,0),delete h.userData.pivot}return h})}_loadNodeShallow(e){let t=this.json,n=this.extensions,i=this;if(this.nodeCache[e]!==void 0)return this.nodeCache[e];let r=t.nodes[e],a=r.name?i.createUniqueName(r.name):"",o=[],l=i._invokeOne(function(c){return c.createNodeMesh&&c.createNodeMesh(e)});return l&&o.push(l),r.camera!==void 0&&o.push(i.getDependency("camera",r.camera).then(function(c){return i._getNodeRef(i.cameraCache,r.camera,c)})),i._invokeAll(function(c){return c.createNodeAttachment&&c.createNodeAttachment(e)}).forEach(function(c){o.push(c)}),this.nodeCache[e]=Promise.all(o).then(function(c){let h;if(r.isBone===!0?h=new _r:c.length>1?h=new gt:c.length===1?h=c[0]:h=new Ft,h!==c[0])for(let u=0,d=c.length;u<d;u++)h.add(c[u]);if(r.name&&(h.userData.name=r.name,h.name=a),Ei(h,r),r.extensions&&Gs(n,h,r),r.matrix!==void 0){let u=new pt;u.fromArray(r.matrix),h.applyMatrix4(u)}else r.translation!==void 0&&h.position.fromArray(r.translation),r.rotation!==void 0&&h.quaternion.fromArray(r.rotation),r.scale!==void 0&&h.scale.fromArray(r.scale);if(!i.associations.has(h))i.associations.set(h,{});else if(r.mesh!==void 0&&i.meshCache.refs[r.mesh]>1){let u=i.associations.get(h);i.associations.set(h,{...u})}return i.associations.get(h).nodes=e,h}),this.nodeCache[e]}loadScene(e){let t=this.extensions,n=this.json.scenes[e],i=this,r=new gt;n.name&&(r.name=i.createUniqueName(n.name)),Ei(r,n),n.extensions&&Gs(t,r,n);let a=n.nodes||[],o=[];for(let l=0,c=a.length;l<c;l++)o.push(i.getDependency("node",a[l]));return Promise.all(o).then(function(l){for(let h=0,u=l.length;h<u;h++){let d=l[h];d.parent!==null?r.add(Zf(d)):r.add(d)}let c=h=>{let u=new Map;for(let[d,p]of i.associations)(d instanceof En||d instanceof Qt)&&u.set(d,p);return h.traverse(d=>{let p=i.associations.get(d);p!=null&&u.set(d,p)}),u};return i.associations=c(r),r})}_createAnimationTracks(e,t,n,i,r){let a=[],o=e.name?e.name:e.uuid,l=[];function c(p){p.morphTargetInfluences&&l.push(p.name?p.name:p.uuid)}ps[r.path]===ps.weights?(c(e),e.isGroup&&e.children.forEach(c)):l.push(o);let h;switch(ps[r.path]){case ps.weights:h=Bi;break;case ps.rotation:h=ki;break;case ps.translation:case ps.scale:h=ls;break;default:n.itemSize===1?h=Bi:h=ls;break}let u=i.interpolation!==void 0?a_[i.interpolation]:As,d=this._getArrayFromAccessor(n);for(let p=0,g=l.length;p<g;p++){let _=new h(l[p]+"."+ps[r.path],t.array,d,u);i.interpolation==="CUBICSPLINE"&&this._createCubicSplineTrackInterpolant(_),a.push(_)}return a}_getArrayFromAccessor(e){let t=e.array;if(e.normalized){let n=wu(t.constructor),i=new Float32Array(t.length);for(let r=0,a=t.length;r<a;r++)i[r]=t[r]*n;t=i}return t}_createCubicSplineTrackInterpolant(e){e.createInterpolant=function(n){let i=this instanceof ki?Mu:mc;return new i(this.times,this.values,this.getValueSize()/3,n)},e.createInterpolant.isInterpolantFactoryMethodGLTFCubicSpline=!0}};function f_(s,e,t){let n=e.attributes,i=new en;if(n.POSITION!==void 0){let o=t.json.accessors[n.POSITION],l=o.min,c=o.max;if(l!==void 0&&c!==void 0){if(i.set(new k(l[0],l[1],l[2]),new k(c[0],c[1],c[2])),o.normalized){let h=wu(Gr[o.componentType]);i.min.multiplyScalar(h),i.max.multiplyScalar(h)}}else{console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.");return}}else return;let r=e.targets;if(r!==void 0){let o=new k,l=new k;for(let c=0,h=r.length;c<h;c++){let u=r[c];if(u.POSITION!==void 0){let d=t.json.accessors[u.POSITION],p=d.min,g=d.max;if(p!==void 0&&g!==void 0){if(l.setX(Math.max(Math.abs(p[0]),Math.abs(g[0]))),l.setY(Math.max(Math.abs(p[1]),Math.abs(g[1]))),l.setZ(Math.max(Math.abs(p[2]),Math.abs(g[2]))),d.normalized){let _=wu(Gr[d.componentType]);l.multiplyScalar(_)}o.max(l)}else console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.")}}i.expandByVector(o)}s.boundingBox=i;let a=new xn;i.getCenter(a.center),a.radius=i.min.distanceTo(i.max)/2,s.boundingSphere=a}function np(s,e,t){let n=e.attributes,i=[];function r(a,o){return t.getDependency("accessor",a).then(function(l){s.setAttribute(o,l)})}for(let a in n){let o=Su[a]||a.toLowerCase();o in s.attributes||i.push(r(n[a],o))}if(e.indices!==void 0&&!s.index){let a=t.getDependency("accessor",e.indices).then(function(o){s.setIndex(o)});i.push(a)}return _t.workingColorSpace!==wn&&"COLOR_0"in n&&console.warn(`THREE.GLTFLoader: Converting vertex colors from "srgb-linear" to "${_t.workingColorSpace}" not supported.`),Ei(s,e),f_(s,e,t),Promise.all(i).then(function(){return e.targets!==void 0?l_(s,e.targets,t):s})}var sp={type:"change"},Au={type:"start"},ap={type:"end"},gc=new gi,rp=new On,p_=Math.cos(70*Tt.DEG2RAD),hn=new k,Ln=2*Math.PI,zt={NONE:-1,ROTATE:0,DOLLY:1,PAN:2,TOUCH_ROTATE:3,TOUCH_PAN:4,TOUCH_DOLLY_PAN:5,TOUCH_DOLLY_ROTATE:6},Tu=1e-6,bc=class extends Na{constructor(e,t=null){super(e,t),this.state=zt.NONE,this.target=new k,this.cursor=new k,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.minTargetRadius=0,this.maxTargetRadius=1/0,this.minPolarAngle=0,this.maxPolarAngle=Math.PI,this.minAzimuthAngle=-1/0,this.maxAzimuthAngle=1/0,this.enableDamping=!1,this.dampingFactor=.05,this.enableZoom=!0,this.zoomSpeed=1,this.enableRotate=!0,this.rotateSpeed=1,this.keyRotateSpeed=1,this.enablePan=!0,this.panSpeed=1,this.screenSpacePanning=!0,this.keyPanSpeed=7,this.zoomToCursor=!1,this.autoRotate=!1,this.autoRotateSpeed=2,this.keys={LEFT:"ArrowLeft",UP:"ArrowUp",RIGHT:"ArrowRight",BOTTOM:"ArrowDown"},this.mouseButtons={LEFT:cs.ROTATE,MIDDLE:cs.DOLLY,RIGHT:cs.PAN},this.touches={ONE:hs.ROTATE,TWO:hs.DOLLY_PAN},this.target0=this.target.clone(),this.position0=this.object.position.clone(),this.zoom0=this.object.zoom,this._cursorStyle="auto",this._domElementKeyEvents=null,this._lastPosition=new k,this._lastQuaternion=new Zt,this._lastTargetPosition=new k,this._quat=new Zt().setFromUnitVectors(e.up,new k(0,1,0)),this._quatInverse=this._quat.clone().invert(),this._spherical=new Pr,this._sphericalDelta=new Pr,this._scale=1,this._panOffset=new k,this._rotateStart=new Be,this._rotateEnd=new Be,this._rotateDelta=new Be,this._panStart=new Be,this._panEnd=new Be,this._panDelta=new Be,this._dollyStart=new Be,this._dollyEnd=new Be,this._dollyDelta=new Be,this._dollyDirection=new k,this._mouse=new Be,this._performCursorZoom=!1,this._pointers=[],this._pointerPositions={},this._controlActive=!1,this._onPointerMove=g_.bind(this),this._onPointerDown=m_.bind(this),this._onPointerUp=b_.bind(this),this._onContextMenu=w_.bind(this),this._onMouseWheel=__.bind(this),this._onKeyDown=y_.bind(this),this._onTouchStart=M_.bind(this),this._onTouchMove=S_.bind(this),this._onMouseDown=x_.bind(this),this._onMouseMove=v_.bind(this),this._interceptControlDown=E_.bind(this),this._interceptControlUp=T_.bind(this),this.domElement!==null&&this.connect(this.domElement),this.update()}set cursorStyle(e){this._cursorStyle=e,e==="grab"?this.domElement.style.cursor="grab":this.domElement.style.cursor="auto"}get cursorStyle(){return this._cursorStyle}connect(e){super.connect(e),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerUp),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.getRootNode().addEventListener("keydown",this._interceptControlDown,{passive:!0,capture:!0}),this.domElement.style.touchAction="none"}disconnect(){this.state=zt.NONE,this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerUp),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.stopListenToKeyEvents();let e=this.domElement.getRootNode();e.removeEventListener("keydown",this._interceptControlDown,{capture:!0}),e.removeEventListener("keyup",this._interceptControlUp,{capture:!0}),this._controlActive=!1,this._pointers.length=0,this._pointerPositions={},this.domElement.style.touchAction="",this.domElement.style.cursor="auto"}dispose(){this.disconnect()}getPolarAngle(){return this._spherical.phi}getAzimuthalAngle(){return this._spherical.theta}getDistance(){return this.object.position.distanceTo(this.target)}listenToKeyEvents(e){e.addEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=e}stopListenToKeyEvents(){this._domElementKeyEvents!==null&&(this._domElementKeyEvents.removeEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=null)}saveState(){this.target0.copy(this.target),this.position0.copy(this.object.position),this.zoom0=this.object.zoom}reset(){this.target.copy(this.target0),this.object.position.copy(this.position0),this.object.zoom=this.zoom0,this.object.updateProjectionMatrix(),this.dispatchEvent(sp),this.update(),this.state=zt.NONE}pan(e,t){this._pan(e,t),this.update()}dollyIn(e){this._dollyIn(e),this.update()}dollyOut(e){this._dollyOut(e),this.update()}rotateLeft(e){this._rotateLeft(e),this.update()}rotateUp(e){this._rotateUp(e),this.update()}update(e=null){let t=this.object.position;hn.copy(t).sub(this.target),hn.applyQuaternion(this._quat),this._spherical.setFromVector3(hn),this.autoRotate&&this.state===zt.NONE&&this._rotateLeft(this._getAutoRotationAngle(e)),this.enableDamping?(this._spherical.theta+=this._sphericalDelta.theta*this.dampingFactor,this._spherical.phi+=this._sphericalDelta.phi*this.dampingFactor):(this._spherical.theta+=this._sphericalDelta.theta,this._spherical.phi+=this._sphericalDelta.phi);let n=this.minAzimuthAngle,i=this.maxAzimuthAngle;isFinite(n)&&isFinite(i)&&(n<-Math.PI?n+=Ln:n>Math.PI&&(n-=Ln),i<-Math.PI?i+=Ln:i>Math.PI&&(i-=Ln),n<=i?this._spherical.theta=Math.max(n,Math.min(i,this._spherical.theta)):this._spherical.theta=this._spherical.theta>(n+i)/2?Math.max(n,this._spherical.theta):Math.min(i,this._spherical.theta)),this._spherical.phi=Math.max(this.minPolarAngle,Math.min(this.maxPolarAngle,this._spherical.phi)),this._spherical.makeSafe(),this.enableDamping===!0?this.target.addScaledVector(this._panOffset,this.dampingFactor):this.target.add(this._panOffset),this.target.sub(this.cursor),this.target.clampLength(this.minTargetRadius,this.maxTargetRadius),this.target.add(this.cursor);let r=!1;if(this.zoomToCursor&&this._performCursorZoom||this.object.isOrthographicCamera)this._spherical.radius=this._clampDistance(this._spherical.radius);else{let a=this._spherical.radius;this._spherical.radius=this._clampDistance(this._spherical.radius*this._scale),r=a!=this._spherical.radius}if(hn.setFromSpherical(this._spherical),hn.applyQuaternion(this._quatInverse),t.copy(this.target).add(hn),this.object.lookAt(this.target),this.enableDamping===!0?(this._sphericalDelta.theta*=1-this.dampingFactor,this._sphericalDelta.phi*=1-this.dampingFactor,this._panOffset.multiplyScalar(1-this.dampingFactor)):(this._sphericalDelta.set(0,0,0),this._panOffset.set(0,0,0)),this.zoomToCursor&&this._performCursorZoom){let a=null;if(this.object.isPerspectiveCamera){let o=hn.length();a=this._clampDistance(o*this._scale);let l=o-a;this.object.position.addScaledVector(this._dollyDirection,l),this.object.updateMatrixWorld(),r=!!l}else if(this.object.isOrthographicCamera){let o=new k(this._mouse.x,this._mouse.y,0);o.unproject(this.object);let l=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),this.object.updateProjectionMatrix(),r=l!==this.object.zoom;let c=new k(this._mouse.x,this._mouse.y,0);c.unproject(this.object),this.object.position.sub(c).add(o),this.object.updateMatrixWorld(),a=hn.length()}else console.warn("WARNING: OrbitControls.js encountered an unknown camera type - zoom to cursor disabled."),this.zoomToCursor=!1;a!==null&&(this.screenSpacePanning?this.target.set(0,0,-1).transformDirection(this.object.matrix).multiplyScalar(a).add(this.object.position):(gc.origin.copy(this.object.position),gc.direction.set(0,0,-1).transformDirection(this.object.matrix),Math.abs(this.object.up.dot(gc.direction))<p_?this.object.lookAt(this.target):(rp.setFromNormalAndCoplanarPoint(this.object.up,this.target),gc.intersectPlane(rp,this.target))))}else if(this.object.isOrthographicCamera){let a=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),a!==this.object.zoom&&(this.object.updateProjectionMatrix(),r=!0)}return this._scale=1,this._performCursorZoom=!1,r||this._lastPosition.distanceToSquared(this.object.position)>Tu||8*(1-this._lastQuaternion.dot(this.object.quaternion))>Tu||this._lastTargetPosition.distanceToSquared(this.target)>Tu?(this.dispatchEvent(sp),this._lastPosition.copy(this.object.position),this._lastQuaternion.copy(this.object.quaternion),this._lastTargetPosition.copy(this.target),!0):!1}_getAutoRotationAngle(e){return e!==null?Ln/60*this.autoRotateSpeed*e:Ln/60/60*this.autoRotateSpeed}_getZoomScale(e){let t=Math.abs(e*.01);return Math.pow(.95,this.zoomSpeed*t)}_rotateLeft(e){this._sphericalDelta.theta-=e}_rotateUp(e){this._sphericalDelta.phi-=e}_panLeft(e,t){hn.setFromMatrixColumn(t,0),hn.multiplyScalar(-e),this._panOffset.add(hn)}_panUp(e,t){this.screenSpacePanning===!0?hn.setFromMatrixColumn(t,1):(hn.setFromMatrixColumn(t,0),hn.crossVectors(this.object.up,hn)),hn.multiplyScalar(e),this._panOffset.add(hn)}_pan(e,t){let n=this.domElement;if(this.object.isPerspectiveCamera){let i=this.object.position;hn.copy(i).sub(this.target);let r=hn.length();r*=Math.tan(this.object.fov/2*Math.PI/180),this._panLeft(2*e*r/n.clientHeight,this.object.matrix),this._panUp(2*t*r/n.clientHeight,this.object.matrix)}else this.object.isOrthographicCamera?(this._panLeft(e*(this.object.right-this.object.left)/this.object.zoom/n.clientWidth,this.object.matrix),this._panUp(t*(this.object.top-this.object.bottom)/this.object.zoom/n.clientHeight,this.object.matrix)):(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - pan disabled."),this.enablePan=!1)}_dollyOut(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale/=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_dollyIn(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale*=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_updateZoomParameters(e,t){if(!this.zoomToCursor)return;this._performCursorZoom=!0;let n=this.domElement.getBoundingClientRect(),i=e-n.left,r=t-n.top,a=n.width,o=n.height;this._mouse.x=i/a*2-1,this._mouse.y=-(r/o)*2+1,this._dollyDirection.set(this._mouse.x,this._mouse.y,1).unproject(this.object).sub(this.object.position).normalize()}_clampDistance(e){return Math.max(this.minDistance,Math.min(this.maxDistance,e))}_handleMouseDownRotate(e){this._rotateStart.set(e.clientX,e.clientY)}_handleMouseDownDolly(e){this._updateZoomParameters(e.clientX,e.clientX),this._dollyStart.set(e.clientX,e.clientY)}_handleMouseDownPan(e){this._panStart.set(e.clientX,e.clientY)}_handleMouseMoveRotate(e){this._rotateEnd.set(e.clientX,e.clientY),this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Ln*this._rotateDelta.x/t.clientHeight),this._rotateUp(Ln*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd),this.update()}_handleMouseMoveDolly(e){this._dollyEnd.set(e.clientX,e.clientY),this._dollyDelta.subVectors(this._dollyEnd,this._dollyStart),this._dollyDelta.y>0?this._dollyOut(this._getZoomScale(this._dollyDelta.y)):this._dollyDelta.y<0&&this._dollyIn(this._getZoomScale(this._dollyDelta.y)),this._dollyStart.copy(this._dollyEnd),this.update()}_handleMouseMovePan(e){this._panEnd.set(e.clientX,e.clientY),this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd),this.update()}_handleMouseWheel(e){this._updateZoomParameters(e.clientX,e.clientY),e.deltaY<0?this._dollyIn(this._getZoomScale(e.deltaY)):e.deltaY>0&&this._dollyOut(this._getZoomScale(e.deltaY)),this.update()}_handleKeyDown(e){let t=!1;switch(e.code){case this.keys.UP:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(Ln*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,this.keyPanSpeed),t=!0;break;case this.keys.BOTTOM:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(-Ln*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,-this.keyPanSpeed),t=!0;break;case this.keys.LEFT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(Ln*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(this.keyPanSpeed,0),t=!0;break;case this.keys.RIGHT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(-Ln*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(-this.keyPanSpeed,0),t=!0;break}t&&(e.preventDefault(),this.update())}_handleTouchStartRotate(e){if(this._pointers.length===1)this._rotateStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._rotateStart.set(n,i)}}_handleTouchStartPan(e){if(this._pointers.length===1)this._panStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panStart.set(n,i)}}_handleTouchStartDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyStart.set(0,r)}_handleTouchStartDollyPan(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enablePan&&this._handleTouchStartPan(e)}_handleTouchStartDollyRotate(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enableRotate&&this._handleTouchStartRotate(e)}_handleTouchMoveRotate(e){if(this._pointers.length==1)this._rotateEnd.set(e.pageX,e.pageY);else{let n=this._getSecondPointerPosition(e),i=.5*(e.pageX+n.x),r=.5*(e.pageY+n.y);this._rotateEnd.set(i,r)}this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Ln*this._rotateDelta.x/t.clientHeight),this._rotateUp(Ln*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd)}_handleTouchMovePan(e){if(this._pointers.length===1)this._panEnd.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panEnd.set(n,i)}this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd)}_handleTouchMoveDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyEnd.set(0,r),this._dollyDelta.set(0,Math.pow(this._dollyEnd.y/this._dollyStart.y,this.zoomSpeed)),this._dollyOut(this._dollyDelta.y),this._dollyStart.copy(this._dollyEnd);let a=(e.pageX+t.x)*.5,o=(e.pageY+t.y)*.5;this._updateZoomParameters(a,o)}_handleTouchMoveDollyPan(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enablePan&&this._handleTouchMovePan(e)}_handleTouchMoveDollyRotate(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enableRotate&&this._handleTouchMoveRotate(e)}_addPointer(e){this._pointers.push(e.pointerId)}_removePointer(e){delete this._pointerPositions[e.pointerId];for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId){this._pointers.splice(t,1);return}}_isTrackingPointer(e){for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId)return!0;return!1}_trackPointer(e){let t=this._pointerPositions[e.pointerId];t===void 0&&(t=new Be,this._pointerPositions[e.pointerId]=t),t.set(e.pageX,e.pageY)}_getSecondPointerPosition(e){let t=e.pointerId===this._pointers[0]?this._pointers[1]:this._pointers[0];return this._pointerPositions[t]}_customWheelEvent(e){let t=e.deltaMode,n={clientX:e.clientX,clientY:e.clientY,deltaY:e.deltaY};switch(t){case 1:n.deltaY*=16;break;case 2:n.deltaY*=100;break}return e.ctrlKey&&!this._controlActive&&(n.deltaY*=10),n}};function m_(s){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(s.pointerId),this.domElement.ownerDocument.addEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.addEventListener("pointerup",this._onPointerUp)),!this._isTrackingPointer(s)&&(this._addPointer(s),s.pointerType==="touch"?this._onTouchStart(s):this._onMouseDown(s),this._cursorStyle==="grab"&&(this.domElement.style.cursor="grabbing")))}function g_(s){this.enabled!==!1&&(s.pointerType==="touch"?this._onTouchMove(s):this._onMouseMove(s))}function b_(s){switch(this._removePointer(s),this._pointers.length){case 0:this.domElement.releasePointerCapture(s.pointerId),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.dispatchEvent(ap),this.state=zt.NONE,this._cursorStyle==="grab"&&(this.domElement.style.cursor="grab");break;case 1:let e=this._pointers[0],t=this._pointerPositions[e];this._onTouchStart({pointerId:e,pageX:t.x,pageY:t.y});break}}function x_(s){let e;switch(s.button){case 0:e=this.mouseButtons.LEFT;break;case 1:e=this.mouseButtons.MIDDLE;break;case 2:e=this.mouseButtons.RIGHT;break;default:e=-1}switch(e){case cs.DOLLY:if(this.enableZoom===!1)return;this._handleMouseDownDolly(s),this.state=zt.DOLLY;break;case cs.ROTATE:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=zt.PAN}else{if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=zt.ROTATE}break;case cs.PAN:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=zt.ROTATE}else{if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=zt.PAN}break;default:this.state=zt.NONE}this.state!==zt.NONE&&this.dispatchEvent(Au)}function v_(s){switch(this.state){case zt.ROTATE:if(this.enableRotate===!1)return;this._handleMouseMoveRotate(s);break;case zt.DOLLY:if(this.enableZoom===!1)return;this._handleMouseMoveDolly(s);break;case zt.PAN:if(this.enablePan===!1)return;this._handleMouseMovePan(s);break}}function __(s){this.enabled===!1||this.enableZoom===!1||this.state!==zt.NONE||(s.preventDefault(),this.dispatchEvent(Au),this._handleMouseWheel(this._customWheelEvent(s)),this.dispatchEvent(ap))}function y_(s){this.enabled!==!1&&this._handleKeyDown(s)}function M_(s){switch(this._trackPointer(s),this._pointers.length){case 1:switch(this.touches.ONE){case hs.ROTATE:if(this.enableRotate===!1)return;this._handleTouchStartRotate(s),this.state=zt.TOUCH_ROTATE;break;case hs.PAN:if(this.enablePan===!1)return;this._handleTouchStartPan(s),this.state=zt.TOUCH_PAN;break;default:this.state=zt.NONE}break;case 2:switch(this.touches.TWO){case hs.DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchStartDollyPan(s),this.state=zt.TOUCH_DOLLY_PAN;break;case hs.DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchStartDollyRotate(s),this.state=zt.TOUCH_DOLLY_ROTATE;break;default:this.state=zt.NONE}break;default:this.state=zt.NONE}this.state!==zt.NONE&&this.dispatchEvent(Au)}function S_(s){switch(this._trackPointer(s),this.state){case zt.TOUCH_ROTATE:if(this.enableRotate===!1)return;this._handleTouchMoveRotate(s),this.update();break;case zt.TOUCH_PAN:if(this.enablePan===!1)return;this._handleTouchMovePan(s),this.update();break;case zt.TOUCH_DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchMoveDollyPan(s),this.update();break;case zt.TOUCH_DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchMoveDollyRotate(s),this.update();break;default:this.state=zt.NONE}}function w_(s){this.enabled!==!1&&s.preventDefault()}function E_(s){s.key==="Control"&&(this._controlActive=!0,this.domElement.getRootNode().addEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}function T_(s){s.key==="Control"&&(this._controlActive=!1,this.domElement.getRootNode().removeEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}var xc=class extends Cs{constructor(){super(),this.name="RoomEnvironment",this.position.y=-3.5;let e=new xi;e.deleteAttribute("uv");let t=new Rn({side:on}),n=new Rn,i=new ri(16777215,900,28,2);i.position.set(.418,16.199,.3),this.add(i);let r=new it(e,t);r.position.set(-.757,13.219,.717),r.scale.set(31.713,28.305,28.591),this.add(r);let a=new An(e,n,6),o=new Ft;o.position.set(-10.906,2.009,1.846),o.rotation.set(0,-.195,0),o.scale.set(2.328,7.905,4.651),o.updateMatrix(),a.setMatrixAt(0,o.matrix),o.position.set(-5.607,-.754,-.758),o.rotation.set(0,.994,0),o.scale.set(1.97,1.534,3.955),o.updateMatrix(),a.setMatrixAt(1,o.matrix),o.position.set(6.167,.857,7.803),o.rotation.set(0,.561,0),o.scale.set(3.927,6.285,3.687),o.updateMatrix(),a.setMatrixAt(2,o.matrix),o.position.set(-2.017,.018,6.124),o.rotation.set(0,.333,0),o.scale.set(2.002,4.566,2.064),o.updateMatrix(),a.setMatrixAt(3,o.matrix),o.position.set(2.291,-.756,-2.621),o.rotation.set(0,-.286,0),o.scale.set(1.546,1.552,1.496),o.updateMatrix(),a.setMatrixAt(4,o.matrix),o.position.set(-2.193,-.369,-5.547),o.rotation.set(0,.516,0),o.scale.set(3.875,3.487,2.986),o.updateMatrix(),a.setMatrixAt(5,o.matrix),this.add(a);let l=new it(e,Vr(50));l.position.set(-16.116,14.37,8.208),l.scale.set(.1,2.428,2.739),this.add(l);let c=new it(e,Vr(50));c.position.set(-16.109,18.021,-8.207),c.scale.set(.1,2.425,2.751),this.add(c);let h=new it(e,Vr(17));h.position.set(14.904,12.198,-1.832),h.scale.set(.15,4.265,6.331),this.add(h);let u=new it(e,Vr(43));u.position.set(-.462,8.89,14.52),u.scale.set(4.38,5.441,.088),this.add(u);let d=new it(e,Vr(20));d.position.set(3.235,11.486,-12.541),d.scale.set(2.5,2,.1),this.add(d);let p=new it(e,Vr(100));p.position.set(0,20,0),p.scale.set(1,.1,1),this.add(p)}dispose(){let e=new Set;this.traverse(t=>{t.isMesh&&(e.add(t.geometry),e.add(t.material))});for(let t of e)t.dispose()}};function Vr(s){return new wa({color:0,emissive:16777215,emissiveIntensity:s})}var Wi={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};var Gn=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},A_=new Mi(-1,1,1,-1,0,1),Ru=class extends bt{constructor(){super(),this.setAttribute("position",new ut([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new ut([0,2,0,0,2,0],2))}},R_=new Ru,Ai=class{constructor(e){this._mesh=new it(R_,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,A_)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var Wr=class extends Gn{constructor(e,t="tDiffuse"){super(),this.textureID=t,this.uniforms=null,this.material=null,e instanceof vt?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=ci.clone(e.uniforms),this.material=new vt({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new Ai(this.material)}render(e,t,n){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=n.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var to=class extends Gn{constructor(e,t){super(),this.scene=e,this.camera=t,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,t,n){let i=e.getContext(),r=e.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let a,o;this.inverse?(a=0,o=1):(a=1,o=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(i.REPLACE,i.REPLACE,i.REPLACE),r.buffers.stencil.setFunc(i.ALWAYS,a,4294967295),r.buffers.stencil.setClear(o),r.buffers.stencil.setLocked(!0),e.setRenderTarget(n),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(t),this.clear&&e.clear(),e.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(i.EQUAL,1,4294967295),r.buffers.stencil.setOp(i.KEEP,i.KEEP,i.KEEP),r.buffers.stencil.setLocked(!0)}},vc=class extends Gn{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var _c=class{constructor(e,t){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),t===void 0){let n=e.getSize(new Be);this._width=n.width,this._height=n.height,t=new Kt(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:rn}),t.texture.name="EffectComposer.rt1"}else this._width=t.width,this._height=t.height;this.renderTarget1=t,this.renderTarget2=t.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Wr(Wi),this.copyPass.material.blending=Kn,this.timer=new Da}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,t){this.passes.splice(t,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let t=this.passes.indexOf(e);t!==-1&&this.passes.splice(t,1)}isLastEnabledPass(e){for(let t=e+1;t<this.passes.length;t++)if(this.passes[t].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let t=this.renderer.getRenderTarget(),n=!1;for(let i=0,r=this.passes.length;i<r;i++){let a=this.passes[i];if(a.enabled!==!1){if(a.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(i),a.render(this.renderer,this.writeBuffer,this.readBuffer,e,n),a.needsSwap){if(n){let o=this.renderer.getContext(),l=this.renderer.state.buffers.stencil;l.setFunc(o.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),l.setFunc(o.EQUAL,1,4294967295)}this.swapBuffers()}to!==void 0&&(a instanceof to?n=!0:a instanceof vc&&(n=!1))}}this.renderer.setRenderTarget(t)}reset(e){if(e===void 0){let t=this.renderer.getSize(new Be);this._pixelRatio=this.renderer.getPixelRatio(),this._width=t.width,this._height=t.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,t){this._width=e,this._height=t;let n=this._width*this._pixelRatio,i=this._height*this._pixelRatio;this.renderTarget1.setSize(n,i),this.renderTarget2.setSize(n,i);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(n,i)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var op={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Le(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var qr=class s extends Gn{constructor(e,t=1,n,i){super(),this.strength=t,this.radius=n,this.threshold=i,this.resolution=e!==void 0?new Be(e.x,e.y):new Be(256,256),this.clearColor=new Le(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),a=Math.round(this.resolution.y/2);this.renderTargetBright=new Kt(r,a,{type:rn,depthBuffer:!1}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let h=0;h<this.nMips;h++){let u=new Kt(r,a,{type:rn,depthBuffer:!1});u.texture.name="UnrealBloomPass.h"+h,u.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(u);let d=new Kt(r,a,{type:rn,depthBuffer:!1});d.texture.name="UnrealBloomPass.v"+h,d.texture.generateMipmaps=!1,this.renderTargetsVertical.push(d),r=Math.round(r/2),a=Math.round(a/2)}let o=op;this.highPassUniforms=ci.clone(o.uniforms),this.highPassUniforms.luminosityThreshold.value=i,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new vt({uniforms:this.highPassUniforms,vertexShader:o.vertexShader,fragmentShader:o.fragmentShader}),this.separableBlurMaterials=[];let l=[6,10,14,18,22];r=Math.round(this.resolution.x/2),a=Math.round(this.resolution.y/2);for(let h=0;h<this.nMips;h++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(l[h])),this.separableBlurMaterials[h].uniforms.invSize.value=new Be(1/r,1/a),r=Math.round(r/2),a=Math.round(a/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=t,this.compositeMaterial.uniforms.bloomRadius.value=.1;let c=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=c,this.bloomTintColors=[new k(1,1,1),new k(1,1,1),new k(1,1,1),new k(1,1,1),new k(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=ci.clone(Wi.uniforms),this.blendMaterial=new vt({uniforms:this.copyUniforms,vertexShader:Wi.vertexShader,fragmentShader:Wi.fragmentShader,premultipliedAlpha:!0,blending:qt,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new Le,this._oldClearAlpha=1,this._basic=new Lt,this._fsQuad=new Ai(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,t){let n=Math.round(e/2),i=Math.round(t/2);this.renderTargetBright.setSize(n,i);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(n,i),this.renderTargetsVertical[r].setSize(n,i),this.separableBlurMaterials[r].uniforms.invSize.value=new Be(1/n,1/i),n=Math.round(n/2),i=Math.round(i/2)}render(e,t,n,i,r){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let a=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),r&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=n.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=n.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let o=this.renderTargetBright;for(let l=0;l<this.nMips;l++)this._fsQuad.material=this.separableBlurMaterials[l],this.separableBlurMaterials[l].uniforms.colorTexture.value=o.texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[l]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[l].uniforms.colorTexture.value=this.renderTargetsHorizontal[l].texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[l]),e.clear(),this._fsQuad.render(e),o=this.renderTargetsVertical[l];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(n),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=a}_getSeparableBlurMaterial(e){let t=[],n=e/3;for(let a=0;a<e;a++)t.push(.39894*Math.exp(-.5*a*a/(n*n))/n);let i=[],r=[];for(let a=1;a<e;a+=2){let o=t[a],l=a+1<e?t[a+1]:0,c=o+l;i.push((a*o+(a+1)*l)/c),r.push(c)}return new vt({defines:{KERNEL_PAIRS:i.length},uniforms:{colorTexture:{value:null},invSize:{value:new Be(.5,.5)},direction:{value:new Be(.5,.5)},centerWeight:{value:t[0]},gaussianOffsets:{value:i},gaussianWeights:{value:r}},vertexShader:`

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

				}`})}_getCompositeMaterial(e){return new vt({defines:{NUM_MIPS:e},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`

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

				}`})}};qr.BlurDirectionX=new Be(1,0);qr.BlurDirectionY=new Be(0,1);var no={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var yc=class extends Gn{constructor(){super(),this.isOutputPass=!0,this.uniforms=ci.clone(no.uniforms),this.material=new Ar({name:no.name,uniforms:this.uniforms,vertexShader:no.vertexShader,fragmentShader:no.fragmentShader}),this._fsQuad=new Ai(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,t,n){this.uniforms.tDiffuse.value=n.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},_t.getTransfer(this._outputColorSpace)===Dt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Ua?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===Oa?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===za?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===Os?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===ka?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Ha?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Ba&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var hi=512,C_=7.5;function Cu(s,e=C_){let t=new ya,n=s.map(([o,l])=>new k(o,0,l)),i=[],r=[],a=n.length;for(let o=0;o<a;o++){let l=n[o],c=n[(o+a-1)%a],h=n[(o+1)%a],u=Math.min(e,l.distanceTo(c)/2,l.distanceTo(h)/2);i.push(l.clone().addScaledVector(c.clone().sub(l).normalize(),u)),r.push(l.clone().addScaledVector(h.clone().sub(l).normalize(),u))}for(let o=0;o<a;o++)t.add(new si(i[o],n[o],r[o])),t.add(new Er(r[o],i[(o+1)%a]));return t}function P_(s){let e=Cu(s),t=e.getSpacedPoints(hi),n=[];for(let i=0;i<hi;i++){let r=t[(i+hi-1)%hi],a=t[(i+1)%hi];n.push(new k(a.x-r.x,0,a.z-r.z).normalize())}return{points:t,tangents:n,length:e.getLength()}}function lp(s){let e=[],t=(n,i,r,a)=>{let o=Math.cos(n.angle),l=Math.sin(n.angle);e.push({x:n.x+i*o+r*l,z:n.z-i*l+r*o,r:a,asset:n.asset})};for(let n of s)if(!(n.z<-100))if(n.asset==="data-tram")for(let i of[-2.4,0,2.4])t(n,i,0,1.5);else n.asset==="server-rack"?t(n,0,0,.8):n.asset==="street-lamp"?t(n,0,0,.5):n.asset==="planter"&&(t(n,-.9,0,.85),t(n,.9,0,.85));return e}function cp({routes:s,obstacles:e=[],robotRadius:t=1.3,lanes:n=[2.2,0,-2.2,3.9,-3.9],defaultLane:i=2.2,speeds:r=[],blocked:a=()=>!1}){let o=s.map(P_),l=s.map((w,I)=>({route:I,s:o[I].length*(I*.173%1),dir:1,lane:i,laneTarget:i,speed:0,target:0,laneVel:0,cruise:r[I]??5.1+I*.43,state:"cruise",heading:0,wait:0,cooldown:0,x:0,z:0,fx:0,fz:1,rx:1,rz:0,vx:0,vz:0,turns:0})),c=new k,h=new k,u=new k;function d(w,I,y,N,D){let R=(I/w.length%1+1)%1*hi,U=Math.floor(R)%hi,z=R-Math.floor(R);N.lerpVectors(w.points[U],w.points[(U+1)%hi],z),D.lerpVectors(w.tangents[U],w.tangents[(U+1)%hi],z).normalize().multiplyScalar(y)}let p=w=>Math.atan2(Math.sin(w),Math.cos(w));function g(w){d(o[w.route],w.s,w.dir,c,h),w.fx=h.x,w.fz=h.z,w.rx=-h.z,w.rz=h.x,w.x=c.x+w.rx*w.lane,w.z=c.z+w.rz*w.lane}function _(w,I){for(let y of e){let N=y.r+t;if((w-y.x)**2+(I-y.z)**2<N*N)return!0}return!1}function m(w,I){let y=o[w.route];for(let N of[-2.5,0,1,3,6,9,12,15,18])if(d(y,w.s+w.dir*N,w.dir,c,h),u.set(c.x-h.z*I,0,c.z+h.x*I),_(u.x,u.z)||N>=0&&a(w,u.x,u.z))return!1;return!0}function f(w){w.dir=-w.dir,w.lane=-w.lane,w.laneTarget=i,w.laneVel=0,w.state="turn",w.cooldown=4,w.wait=0,w.target=0,w.turns++,w.blocked=0}function v(w,I){if(w.held){w.target=0,w.speed=0,w.laneVel=0;return}if(w.cooldown>0&&(w.cooldown-=I),w.state==="turn"){if(w.target=0,w.speed<.05){let R=Math.atan2(w.fx,w.fz),U=p(R-w.heading);w.heading+=U*Math.min(1,I*3.2),Math.abs(U)<.1&&(w.heading=R,w.state="cruise")}return}let y=null,N=[...n].sort((R,U)=>Math.abs(R-i)-Math.abs(U-i)||U-R);for(let R of N)if(m(w,R)){y=R;break}if(y===null){w.blocked=(w.blocked||0)+I,w.target=0,w.blocked>2.5&&w.cooldown<=0&&(w.blocked=0,f(w));return}w.blocked=0,w.laneTarget=y,w.target=w.cruise,Math.abs(w.laneTarget-w.lane)>.5&&!m(w,w.lane)&&(w.target=Math.min(w.target,2.4));let D=!1;for(let R of l){if(R===w)continue;let U=P(w,w.laneTarget,R);if(!U)continue;D=!0;let z=R.fx*w.fx+R.fz*w.fz,T=R.speed<.3;if(T||z<-.2){let G=b.get(R)[S.length-1],X=T?0:Math.sign((G.x-w.x)*w.rx+(G.z-w.z)*w.rz)||-1,Z=null;for(let H of N)if(!(X&&Math.sign(H-w.lane)===X)&&m(w,H)&&!P(w,H,R)){Z=H;break}Z!==null?w.laneTarget=Z:w.target=0}else z>.5?U.da>=U.db&&(w.target=Math.min(w.target,U.da<3.8?0:R.speed*.9)):(U.da>U.db||U.da===U.db&&w.route>R.route)&&(w.target=Math.min(w.target,U.da<=4?0:2))}D&&w.target===0?w.wait+=I:w.wait=0,w.wait>2.5&&w.cooldown<=0&&f(w)}let S=[0,2,4,6,8,10,12],b=new Map,M=new k;function E(w,I,y,N){d(o[w.route],w.s+w.dir*I,w.dir,c,h),N.set(c.x-h.z*y,0,c.z+h.x*y)}function P(w,I,y){let N=b.get(y),D=(t*2+.6)**2,R=y.speed<.3;for(let U=0;U<S.length;U++){E(w,S[U],I,M);let z=S[U]/Math.max(w.speed,1.5);for(let T=0;T<S.length&&!(R&&T>0);T++){let G=S[T]/Math.max(y.speed,1.5);if(!(S[U]>4&&Math.abs(z-G)>1.6)&&M.distanceToSquared(N[T])<D)return{da:S[U],db:S[T]}}}return null}function x(w){if(w>0){for(let I of l){g(I),b.has(I)||b.set(I,S.map(()=>new k));let y=b.get(I);S.forEach((N,D)=>E(I,I.state==="cruise"?N:0,I.lane,y[D]))}for(let I of l)v(I,w);for(let I=0;I<l.length;I++)for(let y=I+1;y<l.length;y++){let N=l[I],D=l[y],R=D.x-N.x,U=D.z-N.z,z=Math.hypot(R,U),T=t*2+.2;if(z>=T)continue;let G=N.state==="turn"||N.speed<.3,X=D.state==="turn"||D.speed<.3,Z=G===X?.5:G?0:1,H=Math.min(T-Math.max(z,.01),2.5*w),oe=R*N.rx+U*N.rz,ke=-(R*D.rx+U*D.rz);N.lane=Tt.clamp(N.lane-Math.sign(oe||1)*H*Z,-4.2,4.2),D.lane=Tt.clamp(D.lane-Math.sign(ke||1)*H*(1-Z),-4.2,4.2),N.target=Math.min(N.target,.5),D.target=Math.min(D.target,.5)}for(let I of l){let y=I.x,N=I.z,D=I.target>I.speed?6:9;if(I.held){I.vx=I.vz=0;continue}I.speed+=Tt.clamp(I.target-I.speed,-D*w,D*w),I.speed<0&&(I.speed=0),I.s+=I.dir*I.speed*w;let R=I.laneTarget-I.lane,U=I.state==="turn"?0:Math.min(3,I.speed*.6),z=Math.sign(R)*Math.min(U,Math.sqrt(8*Math.abs(R)),Math.abs(R)/w);if(I.laneVel+=Tt.clamp(z-I.laneVel,-4*w,4*w),I.lane+=I.laneVel*w,g(I),I.vx=(I.x-y)/w,I.vz=(I.z-N)/w,I.state==="cruise"){let T=I.speed>.25?Math.atan2(I.vx,I.vz):Math.atan2(I.fx,I.fz);I.heading+=p(T-I.heading)*Math.min(1,w*(I.speed>.25?8:3.2))}}}}let C=[];for(let w of l){for(let I=0;I<600&&(g(w),!(C.every(N=>Math.hypot(w.x-N.x,w.z-N.z)>t*2+6)&&m(w,w.lane)));I++)w.s+=1;w.heading=Math.atan2(w.fx,w.fz),C.push(w)}return{agents:l,step:x,hitsObstacle:_,seek(w,I){w.s=I,g(w),w.heading=Math.atan2(w.fx,w.fz)},adopt(w,I){let y=o[w.route],N=0,D=1/0;for(let R=0;R<hi;R++){let U=y.points[R],z=Math.hypot(U.x-I.x,U.z-I.z);z<D&&(D=z,N=R)}w.s=N/hi*y.length,d(y,w.s,w.dir,c,h),w.lane=(I.x-c.x)*-h.z+(I.z-c.z)*h.x,w.laneTarget=i,w.speed=0,w.laneVel=0,w.state="cruise",w.heading=I.heading,g(w)},stats:()=>({states:l.map(w=>w.state),turns:l.reduce((w,I)=>w+I.turns,0),lanes:l.map(w=>Math.round(w.lane*10)/10)})}}var I_=(s,e,t)=>Math.max(e,Math.min(t,s)),wc=(s,e)=>Math.atan2(Math.sin(e-s),Math.cos(e-s)),io=s=>({x:s.x,y:s.y||0,z:s.z,heading:s.heading||0});function qi(s,e=1){let t=s.min.map(d=>d*e),n=s.max.map(d=>d*e),i=(n[0]-t[0])/2,r=(n[2]-t[2])/2,a=i>r,o=Math.max(i,r),l=Math.min(i,r),c=Math.min(7,Math.max(1,Math.ceil(o/Math.max(.3,l)))),h=o/c,u=[];for(let d=0;d<c;d++){let p=-o+h+d*h*2;u.push({x:(t[0]+n[0])/2+(a?p:0),z:(t[2]+n[2])/2+(a?0:p),r:Math.hypot(l,h)})}return{circles:u,minY:t[1],maxY:n[1],reach:Math.max(...u.map(d=>Math.hypot(d.x,d.z)+d.r))}}function Xr(s,e){let t=Math.cos(e.heading),n=Math.sin(e.heading);return{x:e.x+s.x*t+s.z*n,z:e.z-s.x*n+s.z*t,r:s.r}}function Sc(s,e,t,n,i){let r=e-s;if(Math.abs(r)<1e-10)return s>=t&&s<=n;let a=(t-s)/r,o=(n-s)/r;return i[0]=Math.max(i[0],Math.min(a,o)),i[1]=Math.min(i[1],Math.max(a,o)),i[0]<=i[1]}function D_(s,e,t,n,i,r){let a=[0,1];return Sc(e.y-i.y,t.y-r.y,n.minY-s.maxY+.06,n.maxY-s.minY-.06,a)?a:null}function Mc(s,e,t,n,i,r){let a=s.reach+n.reach+.06;if(Math.min(e.x,t.x)-Math.max(i.x,r.x)>a||Math.min(i.x,r.x)-Math.max(e.x,t.x)>a||Math.min(e.z,t.z)-Math.max(i.z,r.z)>a||Math.min(i.z,r.z)-Math.max(e.z,t.z)>a)return!1;let o=D_(s,e,t,n,i,r);if(!o)return!1;for(let l of s.circles)for(let c of n.circles){let h=.06+Math.abs(wc(e.heading,t.heading))*Math.hypot(l.x,l.z)+Math.abs(wc(i.heading,r.heading))*Math.hypot(c.x,c.z),u=Xr(l,e),d=Xr(l,t),p=Xr(c,i),g=Xr(c,r),_=u.x-p.x,m=u.z-p.z,f=d.x-g.x-_,v=d.z-g.z-m,S=I_(-(_*f+m*v)/Math.max(1e-12,f*f+v*v),...o),b=l.r+c.r+h;if((_+f*S)**2+(m+v*S)**2<b*b)return!0}return!1}function Pu(s,e,t,n){let i=s.reach+.06;if(n.world&&(Math.min(e.x,t.x)-i>n.world[2]||Math.max(e.x,t.x)+i<n.world[0]||Math.min(e.z,t.z)-i>n.world[3]||Math.max(e.z,t.z)+i<n.world[1])||Math.min(e.y,t.y)+s.minY>=n.max[1]||Math.max(e.y,t.y)+s.maxY<=n.min[1])return!1;let r=Math.cos(n.heading||0),a=Math.sin(n.heading||0),o=l=>({x:(l.x-n.x)*r-(l.z-n.z)*a,z:(l.x-n.x)*a+(l.z-n.z)*r});for(let l of s.circles){let c=o(Xr(l,e)),h=o(Xr(l,t)),u=l.r+.06+Math.abs(wc(e.heading,t.heading))*Math.hypot(l.x,l.z),d=[0,1];if(Sc(e.y,t.y,n.min[1]-s.maxY+.06,n.max[1]-s.minY-.06,d)&&Sc(c.x,h.x,n.min[0]-u,n.max[0]+u,d)&&Sc(c.z,h.z,n.min[2]-u,n.max[2]+u,d))return!0}return!1}function Ec(){let s=new Map,e=new Map,t=new Map,n=0,i=0,r=0,a=0,o=(g,_)=>g.id===_.id||g.ignore===_.id||_.ignore===g.id||g.enabled===!1||_.enabled===!1;function l(g,_,m,f=!0){if(![m.x,m.y,m.z,m.heading].every(Number.isFinite))return"invalid-pose";for(let v of e.values())if(v.enabled!==!1&&!(v.owner&&(v.owner===g.ignore||v.owner===g.id))&&Pu(g,_,m,v))return v.id;if(f){for(let v of s.values())if(!o(g,v)&&Mc(g,_,m,v,v,v))return v.id}return null}let c=(g,_,m,f=!0)=>!l(g,_,m,f);function h(g,_,m,f=0){if(s.has(g))return s.get(g);let v={id:g,..._,...io(m),priority:f,enabled:!0,blocked:0};return c(v,v,v)?(s.set(g,v),v):null}function u(g,_,m=12){let f={...g,enabled:!0};for(let v=0;v<=m;v+=.5)for(let S=0;S<(v?24:1);S++){let b={...io(_),x:_.x+Math.cos(S*Math.PI/12)*v,z:_.z+Math.sin(S*Math.PI/12)*v};if(c(f,b,b))return b}return null}function d(g,_,m=12){let f=u(g,_,m);return f?(Object.assign(g,f),!0):!1}function p(g){let _=[...s.values()].filter(m=>m.enabled!==!1).map(m=>{let f=t.get(m.id),v=f?.next||io(m);return{body:m,from:io(m),next:v,request:f,accepted:c(m,m,v,!1)}});for(let m=0;m<=_.length;m++){let f=!1;for(let v of _)v.accepted&&v.body.follow&&_.some(S=>S.body.id===v.body.follow&&!S.accepted)&&(v.accepted=!1,f=!0);for(let v=0;v<_.length;v++)for(let S=v+1;S<_.length;S++){let b=_[v],M=_[S];if(o(b.body,M.body)||!b.accepted&&!M.accepted)continue;let E=b.accepted?b.next:b.from,P=M.accepted?M.next:M.from;if(!Mc(b.body,b.from,E,M.body,M.from,P))continue;let x=G=>G.request&&G.accepted&&(Math.hypot(G.next.x-G.from.x,G.next.y-G.from.y,G.next.z-G.from.z)>1e-8||Math.abs(wc(G.from.heading,G.next.heading))>1e-8),C=x(b),w=x(M);if(!C&&!w)continue;let I=b.next.x-b.from.x,y=b.next.z-b.from.z,N=M.next.x-M.from.x,D=M.next.z-M.from.z,R=I*N+y*D>.8*Math.hypot(I,y)*Math.hypot(N,D),U=C&&!Mc(b.body,b.from,b.next,M.body,M.from,M.from),z=w&&!Mc(b.body,b.from,b.from,M.body,M.from,M.next),T=C?w?U&&!z?M:z&&!U?b:b.body.priority!==M.body.priority?b.body.priority<M.body.priority?b:M:R?(M.from.x-b.from.x)*I+(M.from.z-b.from.z)*y>0?b:M:b.body.id>M.body.id?b:M:b:M;T.accepted=!1,f=!0}if(!f)break}for(let m of _)m.accepted?(Object.assign(m.body,m.next),m.body.blocked=0,m.body.obstruction=null):m.request&&(m.body.blocked+=g,m.body.obstruction=l(m.body,m.from,m.next),n++);for(let m of _)m.request?.commit?.(m.accepted,m.body);t.clear(),i++,r+=g}return{register:h,relocate:d,findFree:u,clear:c,obstruction:l,solve:p,begin(){t.clear()},propose(g,_,m){g?.enabled!==!1&&g&&t.set(g.id,{next:io(_),commit:m})},remove(g){s.delete(g),t.delete(g)},solid(g,_){let m=e.get(g),f={id:g,x:0,z:0,heading:0,..._};if(m&&m.x===f.x&&m.z===f.z&&m.heading===f.heading&&m.enabled===f.enabled&&m.min.every((E,P)=>E===f.min[P])&&m.max.every((E,P)=>E===f.max[P]))return;let v=Math.cos(f.heading),S=Math.sin(f.heading),b=[],M=[];for(let E of[f.min[0],f.max[0]])for(let P of[f.min[2],f.max[2]])b.push(f.x+E*v+P*S),M.push(f.z-E*S+P*v);f.world=[Math.min(...b),Math.min(...M),Math.max(...b),Math.max(...M)],e.set(g,f),a++},removeSolid(g){e.delete(g)&&a++},removeOwner(g){for(let[_,m]of e)m.owner===g&&(e.delete(_),a++)},revision:()=>a,solidClear(g){return[...s.values()].every(_=>_.enabled===!1||g.owner&&_.ignore===g.owner||!Pu(_,_,_,{x:0,z:0,heading:0,...g}))},ceiling(g,_,m){let f=0;for(let v of e.values())v.enabled!==!1&&Pu(g,{x:_,y:v.min[1],z:m,heading:0},{x:_,y:v.min[1],z:m,heading:0},v)&&(f=Math.max(f,v.max[1]-g.minY+1));return f},body:g=>s.get(g),neighbors(g,_=12){return[...s.values()].filter(m=>!o(g,m)&&Math.abs(m.y-g.y)<4&&Math.hypot(m.x-g.x,m.z-g.z)<_)},stats:()=>({bodies:s.size,solids:e.size,stops:n,steps:i,time:r,poses:[...s.values()].filter(g=>g.enabled!==!1).map(g=>({id:g.id,x:g.x,y:g.y,z:g.z,heading:g.heading,blocked:g.blocked,obstruction:g.obstruction}))}),dispose(){s.clear(),e.clear(),t.clear()}}}function hp(s,e=1/60){let t=0;return n=>{if(!(n>0))return t=0,0;t=Math.min(t+n,e*6);let i=0;for(;t>=e-1e-9;)s(e),t-=e,i++;return i}}var up=[[[-67,-77],[-18,-77],[-18,-32],[-67,-32]],[[-18,-77],[18,-77],[18,-32],[-18,-32]],[[18,-77],[67,-77],[67,-32],[18,-32]],[[18,13],[18,-32],[67,-32],[67,13]],[[-67,-77],[-18,-77],[-18,-32],[-67,-32]]];function dp(s,e,t){let n=!1,i=0,r=!1,a=!1,o=0,l=!1,c=new gt;c.name="city-life",s.add(c);let h=t.traffic,u=[],d=cp({routes:up,obstacles:t.obstacles||[],blocked:(be,ge,ie)=>{let le=u[be.route],se=le?{...le,x:ge,z:ie}:null;return le?!h.clear(le,se,se):!1}}),p=new Set,g=new Set,_=new Set,m=[],f=new AbortController,v=[],S=[],b=be=>(p.add(be),be),M=be=>(g.add(be),be),E=b(new Bn(1,.013,5,64));for(let be of e){let ge=M(new Lt({color:7452878,transparent:!0,opacity:.08,depthWrite:!1,toneMapped:!1})),ie=new it(E,ge);ie.rotation.x=Math.PI/2,ie.position.set(be.x,.6,be.z),ie.scale.setScalar(be.radius*1.12),c.add(ie),S.push({id:be.id,ring:ie,material:ge,state:"unknown",eventUntil:0,color:new Le(7438733)})}let P=new Map,x=[],C=new Map,w=Date.now(),I=3e3,y=e.find(be=>be.id==="agent"),N=new k(0,0,1),D=new k,R={memory:12690431,graph:16765844,infra:7978495,integrations:7728086,missions:10268415,operations:16758915};if(y){for(let be of e)if(be!==y){let ge=new k(y.x,y.height+3,y.z),ie=new k(be.x,be.height+4,be.z),le=ge.clone().lerp(ie,.5);le.y=Math.max(ge.y,ie.y)+12;let se=new si(ge,le,ie),ce=M(new bi({color:R[be.id],transparent:!0,opacity:0,depthWrite:!1,blending:qt,toneMapped:!1})),ve=new Oi(b(new bt().setFromPoints(se.getPoints(48))),ce);ve.visible=!1,c.add(ve),P.set(be.id,{curve:se,line:ve})}}let U=b(new Bn(1,.06,5,40,Math.PI*.95));for(let be=0;be<12;be++){let ge=[];for(let ie=0;ie<4;ie++){let le=M(new Lt({color:10345983,transparent:!0,opacity:0,depthWrite:!1,blending:qt,toneMapped:!1})),se=new it(ie===3?E:U,le);se.visible=!1,c.add(se),ge.push(se)}x.push({at:-1/0,waves:ge,link:null})}function z(be,ge){if(!l||t.active?.()===!1||be.at<w||ge-be.at>I||be.at>ge||!["started","succeeded","failed","sanitized","progress"].includes(be.state))return;let ie=be.to==="agent",le=ie?be.from:be.to;if(!ie&&be.from!=="agent"||!P.has(le))return;let se=le+":"+ie;if(be.at-(C.get(se)??-1/0)<450)return;C.set(se,be.at);let ce=x.reduce((ve,Ee)=>ve.at<Ee.at?ve:Ee);Object.assign(ce,{at:be.at,link:P.get(le),incoming:ie,from:be.from,to:be.to,state:be.state}),ce.waves.forEach(ve=>ve.material.color.setHex(be.state==="failed"?16737881:R[le]))}function T(be){let ge=Date.now();P.forEach(ie=>{ie.line.visible=!1});for(let ie of x){(!be||t.active?.()===!1)&&(ie.at=-1/0);let le=(ge-ie.at)/I,se=le>=0&&le<1;if(ie.waves.forEach(ve=>{ve.visible=!1}),!se||!ie.link)continue;ie.link.line.visible=!0,ie.link.line.material.opacity=.12*Math.sin(le*Math.PI);for(let ve=0;ve<3;ve++){let Ee=(le-ve*.075)/.75;if(Ee<0||Ee>1)continue;let qe=ie.incoming?1-Ee:Ee,Ye=ie.waves[ve];ie.link.curve.getPoint(qe,Ye.position),ie.link.curve.getTangent(qe,D),ie.incoming&&D.negate(),Ye.quaternion.setFromUnitVectors(N,D),Ye.rotateZ(Math.PI*.525),Ye.scale.setScalar(1.2+Math.sin(Ee*Math.PI)*3.5),Ye.material.opacity=Math.sin(Ee*Math.PI)*.8,Ye.visible=!0}let ce=(le-.74)/.26;if(ce>0){let ve=ie.waves[3];ie.link.curve.getPoint(ie.incoming?0:1,ve.position),ve.rotation.set(-Math.PI/2,0,0),ve.scale.setScalar(1+ce*8),ve.material.opacity=(1-ce)*.6,ve.visible=!0}}}let G=b(new pn(7,7)),X=M(new vt({transparent:!0,depthWrite:!1,blending:qt,uniforms:{color:{value:new Le(5495284)}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"varying vec2 vUv;uniform vec3 color;void main(){float r=length(vUv-.5)*2.;float a=exp(-r*r*5.)*.23*(1.-smoothstep(.65,1.,r));gl_FragColor=vec4(color,a);}"})),Z=b(new Ds(.42,1.3,12,1,!0)),H=M(new Lt({color:9629439,transparent:!0,opacity:.22,depthWrite:!1,toneMapped:!1,blending:qt}));up.forEach((be,ge)=>{let ie=new gt;ie.name="city-white-robot-"+(ge+1);let le=new gt;ie.add(le),c.add(ie);let se=new it(G,X);se.rotation.x=-Math.PI/2,se.position.y=.58,c.add(se);let ce=new it(Z,H);ce.rotation.z=Math.PI,ce.position.y=-.45,ie.add(ce),ie.visible=!1,se.visible=!1,v.push({root:ie,body:le,glow:se,jet:ce,agent:d.agents[ge],lastX:0,lastZ:0,phase:0,gait:0})});function oe(be){be.traverse(ge=>{if(ge.isMesh){p.add(ge.geometry);for(let ie of Array.isArray(ge.material)?ge.material:[ge.material]){g.add(ie);for(let le of Object.values(ie))le?.isTexture&&_.add(le)}}})}function ke(){p.forEach(ge=>ge.dispose()),g.forEach(ge=>ge.dispose());let be=new Set;_.forEach(ge=>{ge.image&&be.add(ge.image),ge.dispose()}),be.forEach(ge=>ge.close?.()),p.clear(),g.clear(),_.clear()}async function Fe(){let be=setTimeout(()=>f.abort(),15e3);try{let ge=await fetch(t.robotURL,{signal:f.signal});if(!ge.ok)throw Error("Robot unavailable");let ie=await ge.arrayBuffer();if(n)return;let{scene:le}=await new Ti().parseAsync(ie,"");if(oe(le),n){ke();return}let se=new en().setFromObject(le),ce=se.getSize(new k),ve=Math.max(ce.x,ce.y,ce.z);if(!Number.isFinite(ve)||ve<=0)throw Error("Invalid robot bounds");le.scale.setScalar(6/ve),le.updateMatrixWorld(!0),se.setFromObject(le);let Ee=se.getCenter(new k);le.position.set(-Ee.x,-se.min.y,-Ee.z),le.rotation.y=-Math.PI/2,le.updateMatrixWorld(!0),se.setFromObject(le),se.expandByScalar(.4);let qe=qi({min:se.min.toArray(),max:se.max.toArray()});le.traverse(Ye=>{Ye.isMesh&&(Ye.castShadow=!1,Ye.receiveShadow=!0)}),v.forEach((Ye,st)=>{if(Ye.body.add(le.clone(!0)),Ye.root.visible=!0,Ye.glow.visible=!0,h){let at=Ye.agent,ot=null;for(let J=0;J<600&&!ot;J++)ot=h.register("patrol-"+st,qe,{x:at.x,y:1.6,z:at.z,heading:at.heading},1),ot||d.seek(at,at.s+1);ot?(u[st]=ot,t.society?.patrol(ot.id,ot,Ye.root,(J,dt)=>{at.held=J,dt&&d.adopt(at,dt)})):(Ye.root.visible=!1,Ye.glow.visible=!1,at.held=!0)}}),r=!0,re(0,!1)}catch(ge){n||(a=!0,t.onError?.(ge))}finally{clearTimeout(be)}}function We(be){m.forEach(ie=>{ie.dispose(),g.delete(ie)}),m.length=0,be.children.find(ie=>ie.userData.district==="operations")?.getObjectByName("signal")?.traverse(ie=>{if(!ie.isMesh)return;let le=se=>{let ce=M(se.clone());return m.push(ce),ce};ie.material=Array.isArray(ie.material)?ie.material.map(le):le(ie.material)})}function fe(be,ge=[]){for(let le of S){let se=be.find(ce=>ce.id===le.id);le.state=!se||se.stale?"unknown":se.state,le.color.setHex(le.state==="error"?16730430:le.state==="running"?7400403:le.state==="unknown"?7438733:7452878),le.material.color.copy(le.color)}let ie=Date.now();for(let le of[...ge].reverse())if(le.id>o&&(z(le,ie),l&&le.at>=w&&t.active?.()!==!1&&t.society?.event(le),ie-le.at<6e3)){let se=S.find(ce=>ce.id===le.district);se&&(se.eventUntil=le.at+5e3)}o=Math.max(o,...ge.map(le=>le.id))}function re(be,ge){if(n)return;l=ge,T(ge);let ie=ge?Math.min(.1,Math.max(0,be)):0,le=h?d.agents.map(se=>({...se})):null;ge&&ie>0&&(i+=ie,d.step(ie));for(let[se,ce]of v.entries()){let ve=ce.agent,Ee=Math.hypot(ve.vx,ve.vz),qe=(!ge||ve.held)&&u[se]?u[se]:ve,Ye=Math.hypot(qe.x-ce.lastX,qe.z-ce.lastZ),st=Ye>.5?0:Ye;ce.lastX=qe.x,ce.lastZ=qe.z,ce.phase+=st*1.4,ce.gait=Tt.damp(ce.gait,Math.min(1,st/Math.max(ie,1e-4)/1.2),6,ie);let at=ce.gait,ot=1-at;ce.root.position.set(qe.x,1.6+Math.sin(ce.phase)*.14*at+Math.sin(i*.7+se*2.1)*.03*ot,qe.z),ce.root.rotation.y=qe.heading;let J=(ve.vx*ve.rx+ve.vz*ve.rz)/Math.max(1,Ee);if(ce.body.rotation.z=Math.sin(i*.85+se)*.035*at-J*.08,ce.body.rotation.x=-.035+Math.sin(i*1.1+se)*.018*at-Math.min(.09,Ee*.015),ce.glow.position.x=qe.x,ce.glow.position.z=qe.z,ce.jet.scale.y=1+Math.sin(i*4+se)*.12+Ee*.04,ge&&u[se]&&!ve.held){let dt=u[se],ct={x:ve.x,y:1.6,z:ve.z,heading:ve.heading};ce.escape&&i<ce.escape.until?(ct.x=dt.x+ce.escape.x*ie*1.8,ct.z=dt.z+ce.escape.z*ie*1.8,ct.heading=dt.heading):ce.escape=null,!h.clear(dt,dt,ct,!1)&&h.clear(dt,dt,{...ct,heading:dt.heading},!1)&&(ct.heading=dt.heading);let O=Math.hypot(ct.x-dt.x,ct.z-dt.z);h.propose(dt,ct,(A,ne)=>{if(ce.wait=A&&O>.002?0:(ce.wait||0)+ie,ce.escape&&A&&d.adopt(ve,ne),ce.wait>2.5){let de=Math.sin(ne.heading),he=Math.cos(ne.heading);for(let[Ue,Oe]of[[he,-de],[-he,de],[de,he],[-de,-he]])if(h.clear(ne,ne,{...ne,x:ne.x+Ue*2.5,z:ne.z+Oe*2.5})){ce.escape={x:Ue,z:Oe,until:i+2};break}ce.wait=0}if(A)ve.trafficWait=0,ve.heading=ne.heading;else{let de=(ve.trafficWait||0)+ie;Object.assign(ve,le[se]),ve.speed=0,ve.vx=ve.vz=0,ve.trafficWait=de,de>2.5&&(ve.state!=="turn"?(ve.dir=-ve.dir,ve.lane=-ve.lane,ve.laneTarget=2.2,ve.state="turn",ve.turns++):ve.state="cruise",ve.trafficWait=0)}ce.root.position.x=ne.x,ce.root.position.z=ne.z,ce.root.rotation.y=ne.heading,ce.glow.position.x=ne.x,ce.glow.position.z=ne.z})}}for(let se of S){let ce=ge?.5+.5*Math.sin(i*(se.state==="error"?3.2:1.8)):.5,ve=se.state==="error"||se.state==="running"||se.eventUntil>Date.now();if(se.material.opacity=se.state==="unknown"?.03:ve?.2+ce*.46:.065,se.ring.scale.setScalar(e.find(Ee=>Ee.id===se.id).radius*1.12*(1+(ve&&ge?ce*.035:0))),se.id==="operations")for(let Ee of m)Ee.color.copy(se.color),Ee.emissive?.copy(se.color),Ee.emissiveIntensity=se.state==="error"?1.1+ce*2.2:se.state==="unknown"?.08:.55}}function Pe(){n||(n=!0,f.abort(),t.signal?.removeEventListener("abort",Pe),c.removeFromParent(),ke(),v.length=0,m.length=0,u.forEach(be=>h.remove(be.id)))}return t.signal?.addEventListener("abort",Pe,{once:!0}),t.signal?.aborted?Pe():Fe(),{update:re,setData:fe,attachLandmarks:We,dispose:Pe,stats:()=>({robots:r?v.filter(be=>be.root.visible).length:0,robotError:a,time:i,navigation:{...d.stats(),gaits:v.map(be=>+(be.gait||0).toFixed(2)),held:v.map(be=>!!be.agent.held),enabled:v.map(be=>!!u[v.indexOf(be)]?.enabled)},positions:v.map(be=>be.root.position.toArray()),transmissions:x.filter(be=>be.waves.some(ge=>ge.visible)).map(be=>({from:be.from,to:be.to,state:be.state,age:Date.now()-be.at})),signals:S.map(be=>({id:be.id,state:be.state,intensity:be.material.opacity}))})}}var so='Geist, "Segoe UI", system-ui, sans-serif',Tc=13,L_=s=>{let e=2166136261;for(let t=0;t<s.length;t++)e=Math.imul(e^s.charCodeAt(t),16777619);return(e>>>0).toString(16).padStart(8,"0")},Iu={vertex:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:`uniform sampler2D map;uniform float time,glitch,fade,gain;uniform vec3 tint;varying vec2 vUv;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){vec2 uv=vUv;float row=floor(uv.y*44.0),frame=floor(time*24.0);
float g=glitch*step(0.7,hash(row+frame*0.37));uv.x+=(hash(row*3.1+frame)-0.5)*0.14*g;
float split=0.0025+glitch*0.02;vec4 c=texture2D(map,uv);
float r=texture2D(map,uv+vec2(split,0.0)).r,b=texture2D(map,uv-vec2(split,0.0)).b;
float a=max(c.a,max(texture2D(map,uv+vec2(split,0.0)).a,texture2D(map,uv-vec2(split,0.0)).a));
float lines=0.84+0.16*sin(uv.y*420.0-time*9.0);float flicker=0.95+0.05*sin(time*31.0)*sin(time*7.3);
float bx=(fract(uv.y*0.55-time*0.11)-0.5)*22.0;float band=0.35*exp(-bx*bx);
float edge=smoothstep(0.0,0.05,uv.x)*smoothstep(0.0,0.05,1.0-uv.x)*smoothstep(0.0,0.09,uv.y)*smoothstep(0.0,0.09,1.0-uv.y);
vec3 col=vec3(r,c.g,b)*tint*(lines*flicker+band)*gain;gl_FragColor=vec4(col*a*fade*edge,1.0);}`},F_={vertex:"varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",fragment:`uniform float time,strength;uniform vec3 base,top;varying vec2 vUv;varying vec3 vNormal,vView;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){float h=vUv.y;float rim=pow(clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0),1.5);
float fall=pow(clamp(1.0-h,0.0,1.0),1.35)*0.75+0.25*clamp(1.0-h,0.0,1.0);float scan=0.7+0.3*sin(h*64.0-time*5.5);
float noise=0.85+0.15*hash(floor(h*96.0)+floor(time*18.0));float shimmer=0.8+0.2*sin(vUv.x*40.0+time*2.0);
float a=(0.28+0.72*rim)*fall*scan*noise*shimmer*strength;gl_FragColor=vec4(mix(base,top,h)*a,1.0);}`},N_={vertex:Iu.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float r=length(vUv-0.5)*2.0;float rings=smoothstep(0.35,1.0,sin(r*16.0-time*2.6));
float core=exp(-r*r*7.0);float a=(core*1.2+pow(max(0.0,1.0-r),1.8)*(0.25+0.75*rings)*0.6)*strength*(1.0-smoothstep(0.85,1.0,r));
gl_FragColor=vec4(color*a,1.0);}`},fp={vertex:Iu.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float dash=step(0.45,fract(vUv.x*28.0+time*0.35));float pulse=0.75+0.25*sin(vUv.x*6.2831*3.0-time*4.0);
gl_FragColor=vec4(color*dash*pulse*strength,1.0);}`},U_={vertex:`attribute float seed;uniform float time,pointScale;varying float vLife;
void main(){float speed=0.55+seed*0.9;float life=fract(seed*3.17+time*speed/13.0);float y=life*13.0;
float ang=seed*6.2831+time*(0.35+seed*0.4)+life*2.2;float rad=mix(1.8,6.8,life)*(0.3+0.7*fract(seed*7.13));
vec4 mv=modelViewMatrix*vec4(cos(ang)*rad,y,sin(ang)*rad,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.09+0.16*fract(seed*3.7))*pointScale/max(1.0,-mv.z);vLife=life;}`,fragment:`uniform vec3 color;uniform float strength;varying float vLife;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,2.0)*sin(vLife*3.1416)*strength;gl_FragColor=vec4(color*a,1.0);}`};function jr(s,e,t={}){return new vt({uniforms:e,vertexShader:s.vertex,fragmentShader:s.fragment,transparent:!0,depthWrite:!1,blending:qt,side:Ot,...t})}function O_(s,e=24){let t=[],n=new Set;for(let i of Array.isArray(s)?s:[]){if(typeof i!="string")continue;let r=i.replace(/[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u2028-\u202e\u2066-\u2069]/g," ").replace(/\s+/g," ").trim().slice(0,140);if(!(r.length<4||n.has(r))&&(n.add(r),t.push(r),t.length>=e))break}return t}function pp(s,e,t,n){let i=[],r=c=>s.measureText(c).width<=t,a="";for(let c of e.split(" ")){if(i.length>=n)break;let h=a?a+" "+c:c;if(r(h)){a=h;continue}if(a&&(i.push(a),a="",i.length>=n))break;let u=c;for(;!r(u)&&i.length<n;){let d=u.length-1;for(;d>1&&!r(u.slice(0,d));)d--;i.push(u.slice(0,d)),u=u.slice(d)}a=u}a&&i.length<n&&(i.push(a),a="");let o=i.join("").replace(/\s/g,"").length,l=e.replace(/\s/g,"").length;if(o<l&&i.length){let c=i[i.length-1].replace(/[\s,.;:]+$/,"");for(;c.length&&!r(c+"\u2026");)c=c.slice(0,-1);i[i.length-1]=c+"\u2026"}return i.length?i:[""]}function mp(s,e,t={}){let n=t.roof??21,i=String(t.label||"MEMORY").toUpperCase(),r=new gt;r.name="memory-hologram",r.position.set(e.x,n,e.z),s.add(r);let a=[],o=[],l=[],c=(ie,le)=>(a.push(ie),o.push(le),new it(ie,le)),h=new Le(6544639),u=new Le(10980351),d=new Le(14218751),p={time:{value:0}},g=!1,_=0,m=[],f=[],v=0,S="none",b=!1,M=c(new pn(11,11),jr(N_,{time:p.time,strength:{value:.9},color:{value:h}}));M.rotation.x=-Math.PI/2,M.position.y=.18,r.add(M);let E=c(new as(7.4,2.3,Tc,56,1,!0),jr(F_,{time:p.time,strength:{value:.32},base:{value:h},top:{value:u}}));E.position.y=Tc/2+.3,r.add(E);let P=c(new Bn(7.5,.055,6,128),jr(fp,{time:p.time,strength:{value:1.6},color:{value:h}}));P.rotation.x=Math.PI/2,P.position.y=Tc+.3,r.add(P);let x=c(new Bn(2.6,.05,6,96),jr(fp,{time:p.time,strength:{value:2},color:{value:d}}));x.rotation.x=Math.PI/2,x.position.y=.55,r.add(x);let C=new Tr(1.35,1),w=new Sa(C),I=new bi({color:10217983,transparent:!0,opacity:.75,blending:qt,depthWrite:!1,toneMapped:!1}),y=new Ps(w,I);y.position.y=4.6,r.add(y),a.push(C,w),o.push(I);let N=new Lt({color:4175871,transparent:!0,opacity:.12,blending:qt,depthWrite:!1,toneMapped:!1}),D=c(new Tr(1.1,1),N);y.add(D);let R=160,U=new Float32Array(R);for(let ie=0;ie<R;ie++)U[ie]=ie*.618033988749895%1;let z=new bt;z.setAttribute("position",new Mt(new Float32Array(R*3),3)),z.setAttribute("seed",new Mt(U,1)),z.boundingSphere=new xn(new k(0,Tc/2,0),12);let T=jr(U_,{time:p.time,pointScale:{value:800},color:{value:d},strength:{value:.9}}),G=new fn(z,T);G.frustumCulled=!1,r.add(G),a.push(z),o.push(T);let X=new ri(6478079,26,46,2);X.position.y=6,r.add(X);function Z(ie,le,se,ce){let ve=document.createElement("canvas");ve.width=se,ve.height=ce;let Ee=ve.getContext("2d"),qe=new Is(ve);qe.colorSpace=Xt,qe.generateMipmaps=!1,qe.minFilter=jt,l.push(qe);let Ye=jr(Iu,{map:{value:qe},time:p.time,glitch:{value:0},fade:{value:0},gain:{value:1.35},tint:{value:new Le(16777215)}},{side:jn}),st=c(new pn(ie,le),Ye);return{canvas:ve,context:Ee,texture:qe,material:Ye,mesh:st,text:"",targetFade:1,next:0}}let H=Z(17.5,8.55,1024,500),oe=[Z(8.6,2.1,640,156),Z(8.6,2.1,640,156)];H.mesh.position.y=10.4,r.add(H.mesh),oe.forEach((ie,le)=>{ie.mesh.position.y=le?14.4:5.9,ie.orbit=le?-.22:.27,ie.angle=le*2.3,r.add(ie.mesh)});function ke(ie,le,se){let{context:ce,canvas:ve}=H,Ee=ve.width,qe=ve.height;ce.clearRect(0,0,Ee,qe),ce.fillStyle="rgba(48,150,214,0.14)",ce.beginPath(),ce.roundRect(14,14,Ee-28,qe-28,22),ce.fill(),ce.strokeStyle="rgba(150,230,255,0.85)",ce.lineWidth=3;for(let[Ye,st,at,ot]of[[18,18,1,1],[Ee-18,18,-1,1],[18,qe-18,1,-1],[Ee-18,qe-18,-1,-1]])ce.beginPath(),ce.moveTo(Ye,st+ot*42),ce.lineTo(Ye,st),ce.lineTo(Ye+at*42,st),ce.stroke();ce.fillStyle="rgba(150,230,255,0.55)",ce.fillRect(48,108,Ee-96,2),ce.font="600 27px "+so,ce.textBaseline="middle",ce.fillStyle="rgba(160,232,255,0.92)",ce.textAlign="left",ce.fillText("\u258C "+i+(se?"  \xB7  "+String(le+1).padStart(2,"0")+" / "+String(se).padStart(2,"0"):""),50,72),ce.textAlign="right",ce.font="500 25px "+so,ce.fillStyle="rgba(190,150,255,0.85)",ce.fillText("0x"+L_(ie||i).toUpperCase(),Ee-52,72),ce.textAlign="left",ce.shadowColor="rgba(120,225,255,0.9)",ce.shadowBlur=16,ie?(ce.font="600 54px "+so,ce.fillStyle="rgba(232,250,255,0.97)",pp(ce,ie,Ee-110,4).forEach((st,at)=>ce.fillText(st,54,172+at*72))):(ce.font="600 40px "+so,ce.fillStyle="rgba(180,235,255,0.7)",ce.fillText("\u25AE \u25AE \u25AF \u25AE \u25AF \u25AF \u25AE \u25AF \u25AE \u25AE \u25AF \u25AE",54,250)),ce.shadowBlur=0,H.texture.needsUpdate=!0}function Fe(ie,le){let{context:se,canvas:ce}=ie,ve=ce.width,Ee=ce.height;se.clearRect(0,0,ve,Ee),se.fillStyle="rgba(48,150,214,0.12)",se.beginPath(),se.roundRect(6,6,ve-12,Ee-12,14),se.fill(),se.fillStyle="rgba(190,150,255,0.8)",se.fillRect(20,26,6,Ee-52),se.font="600 42px "+so,se.textBaseline="middle",se.textAlign="left",se.shadowColor="rgba(160,140,255,0.9)",se.shadowBlur=12,se.fillStyle="rgba(236,240,255,0.95)",se.fillText(le?pp(se,le,ve-70,1)[0]:"\u25AF \u25AE \u25AF \u25AE \u25AF",42,Ee/2),se.shadowBlur=0,ie.texture.needsUpdate=!0}function We(){return m.length?(v>=f.length&&(f=m.map((ie,le)=>le).sort(()=>Math.random()-.5),v=0),m[f[v++]]):""}let fe=.8,re=[3.5,6.5];function Pe(ie){let le=We();H.text=le,_++,ke(le,m.indexOf(le),m.length),H.material.uniforms.glitch.value=ie?1:0,H.material.uniforms.fade.value=ie?.35:1,fe=g?12:4.5+Math.random()*3}function be(ie,le){let se=oe[ie],ce=We();se.text=ce,Fe(se,ce.length>46?ce.slice(0,44).replace(/\s+\S*$/,"")+"\u2026":ce),se.material.uniforms.glitch.value=le?.7:0,se.material.uniforms.fade.value=le?.3:1,re[ie]=g?15:6+Math.random()*4}ke("",0,0),oe.forEach(ie=>Fe(ie,"")),document.fonts?.ready?.then(()=>{b||(ke(H.text,m.indexOf(H.text),m.length),oe.forEach(ie=>Fe(ie,ie.text)))});let ge=new k;return{group:r,setTexts(ie,le="live"){let se=O_(ie),ce=se.length!==m.length||se.some((ve,Ee)=>ve!==m[Ee]);m=se,S=se.length?le:"none",ce&&(f=[],v=0,(!H.text||!m.includes(H.text))&&(fe=Math.min(fe,.6)))},setPointScale(ie){T.uniforms.pointScale.value=ie},setReducedMotion(ie){g=!!ie},update(ie,le,se){if(b)return;let ce=se&&!g,ve=ce?Math.min(.1,Math.max(0,ie)):0;p.time.value+=ve,fe-=ie,fe<=0&&Pe(ce),re.forEach((Ee,qe)=>{re[qe]=Ee-ie,re[qe]<=0&&be(qe,ce)});for(let Ee of[H,...oe]){let qe=Ee.material.uniforms;qe.glitch.value=Math.max(0,qe.glitch.value-ie*2.4),qe.fade.value=Math.min(1,qe.fade.value+ie*1.8),Ee.mesh.quaternion.copy(le.quaternion)}oe.forEach(Ee=>{Ee.angle+=Ee.orbit*ve,Ee.mesh.position.x=Math.cos(Ee.angle)*6.4,Ee.mesh.position.z=Math.sin(Ee.angle)*6.4,ge.copy(Ee.mesh.position).add(r.position).sub(le.position).normalize();let qe=-(ge.x*Math.cos(Ee.angle)+ge.z*Math.sin(Ee.angle));Ee.material.uniforms.gain.value=1.35*Tt.clamp(.35+qe*.9,.15,1.2)}),H.mesh.position.y=9.6+Math.sin(p.time.value*.9)*.22,y.rotation.y+=ve*.7,y.rotation.x+=ve*.31,D.rotation.y-=ve*1.1,P.rotation.z+=ve*.18,x.rotation.z-=ve*.42,X.intensity=26+Math.sin(p.time.value*2.1)*6+H.material.uniforms.glitch.value*22},stats:()=>({artifacts:m.length,source:S,switches:_,shown:H.text?m.indexOf(H.text):-1}),dispose(){b||(b=!0,r.removeFromParent(),a.forEach(ie=>ie.dispose()),o.forEach(ie=>ie.dispose()),l.forEach(ie=>ie.dispose()),X.dispose())}}}var z_=`uniform sampler2D tDiffuse;uniform float time,vignette,grain,aberration,streak,shafts,letterbox,aspect,contrast,saturation;
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
gl_FragColor=vec4(c,1.0);}`;function gp(s){let e={tDiffuse:{value:null},time:s,vignette:{value:.5},grain:{value:.03},aberration:{value:.0024},streak:{value:.16},shafts:{value:0},letterbox:{value:0},aspect:{value:1.7777777777777777},contrast:{value:.22},saturation:{value:1.08},lightPos:{value:new Be(.5,.5)},shaftColor:{value:new Le(12376319)},shadowTint:{value:new Le(6686)},highlightTint:{value:new Le(1444864)}},t=new Wr(new vt({uniforms:e,vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:z_}));t.enabled=!1;let n=new k,i=new Le(16761992),r=new Le(12376319),a=!1;return{pass:t,setCinematic(o){a=!!o},update(o,l,c,{day:h,dusk:u,cloud:d,animated:p}){e.aspect.value=l.aspect;let g=a?1:0;e.letterbox.value=p?e.letterbox.value+(g-e.letterbox.value)*Math.min(1,o*2.2):g,n.copy(c).multiplyScalar(1e3).add(l.position).project(l);let _=(n.x+1)/2,m=(n.y+1)/2,f=n.z<1?Math.max(0,1-Math.max(0,Math.abs(_-.5)-.5,Math.abs(m-.5)-.5)*4):0;e.lightPos.value.set(_,m),e.shafts.value=f*(.06+u*.14+(1-h)*.03)*(1-d*.75),e.shaftColor.value.copy(r).lerp(i,u),e.shadowTint.value.setRGB(0,.018+.01*(1-h),.024+.01*(1-h)),e.highlightTint.value.setRGB(.02+u*.03,.01+u*.006,0)},stats:()=>({enabled:t.enabled,letterbox:+e.letterbox.value.toFixed(2),shafts:+e.shafts.value.toFixed(2)}),dispose(){t.material.dispose(),t.dispose?.()}}}var B_=new Le(16767392),k_=new Le(16758903),Lu=`float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`,Du="varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}",H_="float fbm(vec2 p){float v=0.0,a=0.5;for(int i=0;i<5;i++){if(float(i)>=octaves)break;v+=a*vnoise(p);p=p*2.03+vec2(1.7,9.2);a*=0.5;}return v;}",G_=`uniform float time,day,dusk,cloud,flash,octaves;uniform vec3 zenith,horizon,haze,warm,sunDir,glow;varying vec3 vWorld;${Lu}${H_}
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
gl_FragColor=vec4(col,1.0);}`,bp={vertex:`attribute float phase,speed,size;uniform float time,pointScale,cloud;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0)*(1.0-cloud*0.85);
float k=fract(phase*3.1);vTint=k>0.86?vec3(1.0,0.84,0.66):k>0.7?vec3(0.7,0.82,1.0):vec3(0.82,0.9,1.0);}`,fragment:"varying float vAlpha;varying vec3 vTint;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vTint*a,1.0);}"},xp={vertex:`varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,fragment:`uniform float time,day;uniform vec3 deep,shallow,sky,sunDir,sunColor,glow;uniform vec4 island;varying vec3 vWorld;
#include <fog_pars_fragment>
${Lu}
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
}`},V_=`uniform float time,strength;uniform vec3 color;uniform vec4 island;varying vec3 vWorld;${Lu}
void main(){vec2 p=vWorld.xz;float n=vnoise(p*0.012+vec2(time*0.017,-time*0.011))*0.6+vnoise(p*0.031-vec2(time*0.02,time*0.013))*0.4;
float mist=smoothstep(0.32,0.82,n);vec2 d2=max(abs(p-island.xy)-island.zw,0.0);float outside=smoothstep(0.0,70.0,length(d2));
float far=1.0-smoothstep(500.0,850.0,length(p));gl_FragColor=vec4(color,mist*outside*far*strength);}`,vp={vertex:`attribute vec3 seed;uniform float time,pointScale;varying float vAlpha;
void main(){vec3 p=position;p.x+=sin(time*0.11*seed.x+seed.y*6.28)*9.0+time*0.35*(seed.z-0.5);p.y+=sin(time*0.13+seed.z*6.28)*3.0;
p.z+=cos(time*0.09*seed.y+seed.x*6.28)*9.0;p.x=mod(p.x+90.0,180.0)-90.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.06+0.11*seed.x)*pointScale/max(1.0,-mv.z);vAlpha=0.55+0.45*sin(time*(1.0+seed.y)+seed.z*6.28);}`,fragment:`uniform vec3 color;uniform float strength;varying float vAlpha;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,2.2)*vAlpha*strength,1.0);}`},W_=`uniform float strength;uniform vec3 color;varying vec2 vUv;
void main(){float along=pow(clamp(1.0-vUv.x,0.0,1.0),2.4);float across=pow(clamp(1.0-abs(vUv.y-0.5)*2.0,0.0,1.0),1.7);gl_FragColor=vec4(color*along*across*strength,1.0);}`,q_=`uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;
void main(){float rim=clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0);float a=pow(clamp(vUv.y,0.0,1.0),1.6)*(0.35+0.65*rim)*strength;gl_FragColor=vec4(color*a,1.0);}`,_p={vertex:"attribute float lift;varying float vLift;void main(){vLift=lift;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:"uniform vec3 haze;uniform float shade;varying float vLift;void main(){gl_FragColor=vec4(haze*mix(1.0,shade,smoothstep(0.0,0.6,vLift)),1.0);}"},yp={vertex:`attribute float phase;uniform float time,pointScale,strength;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=strength*(0.75+0.25*sin(time*(0.7+fract(phase)*1.3)+phase));gl_PointSize=clamp(2.2*pointScale/max(1.0,-mv.z),1.5,4.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(1.4,0.95,0.55)*pow(1.0-d,1.5)*vA,1.0);}"},Mp={vertex:"varying vec2 vUv;varying float vRim,vNear;void main(){vUv=uv;vec3 n=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vRim=abs(dot(n,normalize(-mv.xyz)));vNear=smoothstep(40.0,160.0,-mv.z);gl_Position=projectionMatrix*mv;}",fragment:"uniform vec3 color;uniform float strength;varying vec2 vUv;varying float vRim,vNear;void main(){float along=pow(clamp(1.0-vUv.y,0.0,1.0),1.6)*smoothstep(0.0,0.03,vUv.y);gl_FragColor=vec4(color*along*pow(clamp(vRim,0.0,1.0),2.5)*vNear*strength,1.0);}"},Sp={vertex:`attribute float phase;uniform float time,pointScale;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=0.22+0.78*pow(max(0.0,sin(time*1.9+phase)),10.0);gl_PointSize=clamp(1.5*pointScale/max(1.0,-mv.z),2.0,14.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(3.2,0.35,0.25)*pow(1.0-d,2.0)*vA,1.0);}"};function wp(s,e={}){let t=[],n=[],i=new gt;i.name="city-atmosphere",s.add(i);let r=(ue,Re)=>(t.push(ue),n.push(Re),new it(ue,Re)),a={value:0},o={value:900},l={value:0},c={value:0},h={value:.25},u={value:0},d={value:5},p=new Le(4860440),g=(e.sunDirection||new k(-70,145,85)).clone().normalize(),_=new Le(330010),m=new Le(1323596),f=new Le(8011050),v=new Le(s.fog?.color||528926),S=(ue,Re,ye,je={})=>new vt({uniforms:ye,vertexShader:ue,fragmentShader:Re,...je}),b={transparent:!0,depthWrite:!1,blending:qt},M=r(new vi(1500,48,24),S(Du,G_,{time:a,day:l,dusk:c,cloud:h,flash:u,octaves:d,glow:{value:p},zenith:{value:_},horizon:{value:m},haze:{value:v},warm:{value:f},sunDir:{value:g}},{side:on,depthWrite:!1,fog:!1}));M.frustumCulled=!1,M.renderOrder=-10,i.add(M);let E=1400,P=new Float32Array(E*3),x=new Float32Array(E),C=new Float32Array(E),w=new Float32Array(E),I=12345,y=()=>(I=Math.imul(I,1664525)+1013904223>>>0)/4294967296;for(let ue=0;ue<E;ue++){let Re=y()*Math.PI*2,ye=.03+y()*.97,je=Math.sqrt(1-ye*ye);P.set([Math.cos(Re)*je*1450,ye*1450,Math.sin(Re)*je*1450],ue*3),x[ue]=y()*Math.PI*2,C[ue]=.4+y()*1.6,w[ue]=8e-4+y()*y()*.0022}let N=new bt;N.setAttribute("position",new Mt(P,3)),N.setAttribute("phase",new Mt(x,1)),N.setAttribute("speed",new Mt(C,1)),N.setAttribute("size",new Mt(w,1));let D=S(bp.vertex,bp.fragment,{time:a,pointScale:o,cloud:h},{...b,fog:!1}),R=new fn(N,D);R.frustumCulled=!1,R.renderOrder=-9,i.add(R),t.push(N),n.push(D);let U=r(new pn(2800,2800),S(xp.vertex,xp.fragment,{time:a,day:l,glow:{value:p},island:{value:new Pt(0,-7,85,87)},deep:{value:new Le(398368)},shallow:{value:new Le(930640)},sky:{value:m.clone().multiplyScalar(.7)},sunDir:{value:g},sunColor:{value:new Le(14676223)},fogColor:{value:new Le},fogDensity:{value:0},fogNear:{value:1},fogFar:{value:1e3}},{fog:!0}));U.rotation.x=-Math.PI/2,U.position.y=-3.2,i.add(U);let z=r(new pn(1900,1900),S(Du,V_,{time:a,strength:{value:.55},color:{value:new Le(1717320)},island:{value:new Pt(0,-7,85,87)}},{transparent:!0,depthWrite:!1,fog:!1}));z.rotation.x=-Math.PI/2,z.position.y=-2.4,i.add(z);let T=500,G=new Float32Array(T*3),X=new Float32Array(T*3);for(let ue=0;ue<T;ue++)G.set([(y()-.5)*180,2+y()*y()*75,-95+y()*170],ue*3),X.set([y(),y(),y()],ue*3);let Z=new bt;Z.setAttribute("position",new Mt(G,3)),Z.setAttribute("seed",new Mt(X,3));let H=S(vp.vertex,vp.fragment,{time:a,pointScale:o,color:{value:new Le(10475775)},strength:{value:.32}},b),oe=new fn(Z,H);oe.frustumCulled=!1,i.add(oe),t.push(Z),n.push(H);let ke=new gt;ke.position.set(0,86.5,-12),i.add(ke);let Fe=S(Du,W_,{strength:{value:.45},color:{value:new Le(9430783)}},{...b,side:Ot}),We=new pn(100,5.2);We.translate(50,0,0),t.push(We),n.push(Fe);let fe=new it(We,Fe),re=new it(We,Fe);re.rotation.x=Math.PI/2;let Pe=new gt;Pe.add(fe,re),Pe.rotation.z=-.07,ke.add(Pe);let be=new Lt({color:12579583,toneMapped:!1}),ge=r(new vi(.9,16,12),be);ke.add(ge);let ie=(e.lamps||[]).slice(0,64),le=S("varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",q_,{color:{value:new Le(16767392)},strength:{value:.3}},{...b,side:Ot}),se=new Ds(2.7,6.1,18,1,!0),ce=new An(se,le,Math.max(1,ie.length));t.push(se),n.push(le);let ve=new Ft;ie.forEach((ue,Re)=>{ve.position.set(ue.x+1.9*Math.cos(ue.angle||0),3.35,ue.z-1.9*Math.sin(ue.angle||0)),ve.updateMatrix(),ce.setMatrixAt(Re,ve.matrix)}),ce.count=ie.length,ce.instanceMatrix.needsUpdate=!0,i.add(ce);let Ee=[],qe=[],Ye=[],st=[],at=[],ot=360,J=ue=>Math.max(0,.45+.3*Math.sin(ue*5+1.3)+.2*Math.sin(ue*13+.4)+.12*Math.sin(ue*31+2.1))*(.35+.65*Math.max(0,Math.sin(ue*2+.6)));for(let ue=0;ue<=ot;ue++){let Re=Math.PI*.8+Math.PI*1.45*ue/ot,ye=1180+40*Math.sin(Re*7),je=6+74*J(Re);if(Ee.push(Math.cos(Re)*ye,-3.5,Math.sin(Re)*ye,Math.cos(Re)*ye,je,Math.sin(Re)*ye),qe.push(0,je/80),ue<ot){let Ke=ue*2;Ye.push(Ke,Ke+1,Ke+2,Ke+1,Ke+3,Ke+2)}}for(let ue=0;ue<90;ue++){let Re=Math.PI*.82+Math.PI*1.41*y(),ye=1170+40*Math.sin(Re*7);J(Re)<.08||(st.push(Math.cos(Re)*ye,1+y()*y()*28,Math.sin(Re)*ye),at.push(y()*20))}let dt=new bt;dt.setAttribute("position",new ut(Ee,3)),dt.setAttribute("lift",new ut(qe,1)),dt.setIndex(Ye);let ct={value:.8},O=r(dt,S(_p.vertex,_p.fragment,{haze:{value:v},shade:ct},{fog:!1,side:Ot}));O.frustumCulled=!1,O.renderOrder=-8,i.add(O);let A=new bt;A.setAttribute("position",new ut(st,3)),A.setAttribute("phase",new ut(at,1));let ne={value:0},de=S(yp.vertex,yp.fragment,{time:a,pointScale:o,strength:ne},{...b,fog:!1}),he=new fn(A,de);he.frustumCulled=!1,he.renderOrder=-7,i.add(he),t.push(A),n.push(de);let Ue=S(Mp.vertex,Mp.fragment,{color:{value:new Le(11128063)},strength:{value:0}},{...b,side:Ot,fog:!1}),Oe=new as(11,.7,280,20,1,!0);Oe.translate(0,140,0),t.push(Oe),n.push(Ue);let V=[[-72,-168,0],[64,-186,2.1],[4,-232,4.2]].map(([ue,Re,ye])=>{let je=new it(Oe,Ue);return je.position.set(ue,-3,Re),je.rotation.order="YXZ",je.userData.offset=ye,je.frustumCulled=!1,i.add(je),je}),j=96,me=new Float32Array(j*3),_e=new Float32Array(j);for(let ue=0;ue<j;ue++)_e[ue]=y()*Math.PI*2;let pe=new bt;pe.setAttribute("position",new Mt(me,3)),pe.setAttribute("phase",new Mt(_e,1)),pe.setDrawRange(0,0);let xe=S(Sp.vertex,Sp.fragment,{time:a,pointScale:o},{...b,fog:!1}),Se=new fn(pe,xe);Se.frustumCulled=!1,i.add(Se),t.push(pe),n.push(xe);let we=gp(a),Ie=we.pass,q=!1,Te="high",Me=0;return{post:Ie,sea:U,setLighting(ue,Re,ye){g.copy(ye).normalize(),l.value=ue,c.value=Math.min(1,Re)*(1-ue),_.set(330010).lerp(new Le(3108776),ue).lerp(new Le(1909062),c.value*.8),m.set(1323596).lerp(new Le(11126484),ue).lerp(new Le(12609598),c.value*.85),f.set(Re>.1?16747077:8011050),le.uniforms.color.value.copy(B_).lerp(k_,c.value),p.set(4860440).lerp(new Le(7027234),c.value),v.copy(s.fog.color),R.visible=ue<.35,U.material.uniforms.sky.value.copy(m).lerp(_,c.value*.65).multiplyScalar(.7),U.material.uniforms.deep.value.set(398368).lerp(new Le(669270),ue),U.material.uniforms.shallow.value.set(930640).lerp(new Le(1929112),ue),z.material.uniforms.strength.value=.55*(1-ue*.85),ct.value=.82-.22*(1-ue)-.2*c.value,ne.value=Math.max(0,1-ue*1.6)*(1-c.value*.5),U.material.uniforms.sunColor.value.set(14676223).lerp(new Le(16752736),c.value)},setWeather(ue){h.value=ue==="rain"?.95:ue==="fog"?.7:.25},setFlash(ue){u.value=Math.max(0,Math.min(1,ue))},setCinematic(ue){we.setCinematic(ue)},setAviation(ue){let Re=Math.min(j,ue.length);for(let ye=0;ye<Re;ye++)me.set([ue[ye].x,ue[ye].y,ue[ye].z],ye*3);pe.attributes.position.needsUpdate=!0,pe.setDrawRange(0,Re)},setTier(ue){Te=ue;let Re=Te!=="low";z.visible=Re,oe.visible=Re,ce.visible=Re,V.forEach(ye=>{ye.visible=Re}),d.value=Te==="low"?3:Te==="medium"?4:5,Ie.enabled=Te==="high"||Te==="ultra"},setBusy(ue){q=!!ue},setPointScale(ue){o.value=ue},update(ue,Re,ye,je){let Ke=je?Math.min(.1,Math.max(0,ue)):0;a.value+=Ke,s.fog&&(U.material.uniforms.fogColor.value.copy(s.fog.color),s.fog.isFogExp2&&(U.material.uniforms.fogDensity.value=s.fog.density)),U.position.x=ye.position.x,U.position.z=ye.position.z;let B=q?1.15:.32,W=ke.userData.rate??B;ke.userData.rate=W+(B-W)*Math.min(1,Ke*1.5),Pe.rotation.y+=Ke*ke.userData.rate,Fe.uniforms.strength.value=q?.85:.45,ge.scale.setScalar(1+Math.sin(a.value*(q?6:2.2))*.18),le.uniforms.strength.value=.3*(1+.035*Math.sin(a.value*1.6)+.015*Math.sin(a.value*4.1+1.2)),Me+=Ke;let ee=1-l.value;Ue.uniforms.strength.value=ee*ee*.075*(1-c.value*.7)*(1-h.value*.45),V.forEach((ae,Y)=>{let Q=ae.userData.offset;ae.rotation.y=Math.PI+Math.sin(Me*(.11+Y*.025)+Q)*.95,ae.rotation.x=.22+.14*Math.sin(Me*.13+Q*1.7)}),Ie.enabled&&we.update(Ke,ye,g,{day:l.value,dusk:c.value,cloud:h.value,animated:je})},stats:()=>({tier:Te,busy:q,stars:E,dust:T,lamps:ie.length,post:Ie.enabled,coast:Ye.length/6,shoreLights:at.length,clouds:+h.value.toFixed(2),searchlights:V.length,aviation:pe.drawRange.count,cinema:we.stats()}),dispose(){i.removeFromParent(),ce.dispose(),t.forEach(ue=>ue.dispose()),n.forEach(ue=>ue.dispose()),we.dispose()}}}var ro=3,X_=3,j_=192,Ep=[9365742,7730385,16763274,12166399];function Tp(s,e={}){let t=e.anchors||[],n=new gt;n.name="city-flow",s.add(n);let i=[];for(let M=0;M<t.length;M++)for(let E of[1,3]){let P=t[M],x=t[(M+E)%t.length];!P||!x||P.id===x.id||i.push({from:P,to:x,lift:22+(M+E)%4*11})}if(i.length<2)return n.removeFromParent(),{group:n,update(){},setTier(){},stats:()=>({routes:0,points:0,visible:!1}),dispose(){}};let r=Math.min(j_/ro,i.length*X_),a=r*ro,o=new Float32Array(a*3),l=new Float32Array(a),c=new Float32Array(a),h=new Float32Array(a*3),u=new bt;u.setAttribute("position",new Mt(o,3)),u.setAttribute("alpha",new Mt(l,1)),u.setAttribute("size",new Mt(c,1)),u.setAttribute("tint",new Mt(h,3));let d=new vt({transparent:!0,depthWrite:!1,blending:qt,fog:!1,uniforms:{pointScale:{value:900},strength:{value:.4}},vertexShader:`attribute float alpha,size;attribute vec3 tint;uniform float pointScale;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=clamp(size*pointScale/max(1.0,-mv.z),1.0,12.0);vAlpha=alpha;vTint=tint;}`,fragmentShader:`uniform float strength;varying float vAlpha;varying vec3 vTint;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;
gl_FragColor=vec4(vTint*pow(1.0-d,2.4)*vAlpha*strength,1.0);}`}),p=new fn(u,d);p.frustumCulled=!1,p.renderOrder=2,n.add(p);let g=[],_=new Le;for(let M=0;M<r;M++){let E=i[M%i.length];_.setHex(Ep[M%Ep.length]),g.push({route:E,t:M*.3819%1,speed:.045+M*7%9/190,tint:[_.r,_.g,_.b],size:.42+M*3%5*.08})}function m(M,E,P){let x=M.from,C=M.to,w=1-E,I=(x.x+C.x)/2,y=(x.z+C.z)/2,N=Math.max(x.height,C.height)+M.lift,D=x.height+4,R=C.height+4;return P.x=w*w*x.x+2*w*E*I+E*E*C.x,P.y=w*w*D+2*w*E*N+E*E*R,P.z=w*w*x.z+2*w*E*y+E*E*C.z,P}let f={x:0,y:0,z:0},v="high",S=.4;function b(M,E,P){if(!p.visible)return;let x=E?Math.min(M,.05):0;for(let C=0;C<g.length;C++){let w=g[C];w.t=(w.t+x*w.speed)%1;for(let I=0;I<ro;I++){let y=C*ro+I,N=(w.t-I*.012+1)%1;m(w.route,N,f),o[y*3]=f.x,o[y*3+1]=f.y,o[y*3+2]=f.z;let D=Math.min(1,Math.min(N,1-N)*7);l[y]=D*(1-I/ro)*.9,c[y]=w.size*(1-I*.22),h[y*3]=w.tint[0],h[y*3+1]=w.tint[1],h[y*3+2]=w.tint[2]}}u.attributes.position.needsUpdate=!0,u.attributes.alpha.needsUpdate=!0,u.attributes.size.needsUpdate=!0,u.attributes.tint.needsUpdate=!0,S+=(.18+P*.82-S)*Math.min(1,Math.max(0,M)*2),d.uniforms.strength.value=S}return{group:n,update:b,setPointScale(M){d.uniforms.pointScale.value=M},setTier(M){v=M,p.visible=v!=="low",n.visible=v!=="low"},stats:()=>({routes:i.length,points:a,visible:p.visible,tier:v}),dispose(){n.removeFromParent(),u.dispose(),d.dispose()}}}var Ap=[{points:[[-62,34,-62],[8,44,-74],[62,38,-22],[56,47,42],[-8,41,62],[-64,36,12]],speed:9.5},{points:[[30,56,-12],[0,62,18],[-30,58,-12],[0,66,-42]],speed:8},{points:[[42,29,-56],[-28,33,-42],[-52,27,20],[18,31,52],[60,33,8]],speed:11}];function Rp(s,e={}){let t=new gt;t.name="city-drones",s.add(t);let n=[],i=[],r=[],a=new k,o=new k,l=new k,c=new vi(.16,8,6);n.push(c);let h=b=>{let M=new Lt({color:b,toneMapped:!1});return i.push(M),M},u=h(16730684),d=h(5046154),p=h(16777215);[...Ap,...Ap].forEach((b,M)=>{let E=new wr(b.points.map(w=>new k(w[0],w[1]+(M>=3?8:0),w[2])),!0,"centripetal",.6),P=new gt;P.name="city-drone-"+(M+1),P.visible=!1;let x=new gt;P.add(x);let C=[new it(c,u),new it(c,d),new it(c,p)];C[0].position.set(-2.6,.8,0),C[1].position.set(2.6,.8,0),C[2].position.set(0,2.1,-.4),P.add(...C),t.add(P),r.push({root:P,body:x,lights:C,curve:E,length:E.getLength(),speed:b.speed,t:M*.37%1,rotors:[],roll:0,dir:1,waiting:0})});let g=!0,_="high",m=!1,f=new Map,v=b=>{if(!f.has(b)){let M=b.clone();e.surfaces?.apply(M,"object",b.name),f.set(b,M)}return f.get(b)};function S(b,M){let{curve:E,root:P}=b,x=b.t;if(b.t=(b.t+M*b.speed*b.dir/b.length+1)%1,E.getPointAt(b.t,P.position),E.getTangentAt(b.t,a),a.multiplyScalar(b.dir),b.collider&&e.traffic){let I=b.collider,y=e.traffic;E.getPointAt((b.t+b.dir*20/b.length+1)%1,l),b.avoidTime=Math.max(0,(b.avoidTime||0)-M);let N=Math.max(P.position.y,y.ceiling(I,P.position.x,P.position.z),y.ceiling(I,l.x,l.z),b.avoidTime?b.avoidHeight:0);P.position.y=I.y+Tt.clamp(N-I.y,-M*5,M*7);let D=Math.atan2(a.x,a.z),R=I.heading+Math.atan2(Math.sin(D-I.heading),Math.cos(D-I.heading))*Math.min(1,M*3),U={...P.position,heading:R};if(b.obstruction=y.obstruction(I,I,U),b.obstruction){let z=y.body(b.obstruction),T=b.obstruction.startsWith("drone-")&&(Math.abs(I.y-z.y)>1?I.y<z.y:I.id<z.id);T||(b.avoidHeight=Math.min(170,I.y+8),b.avoidTime=8),U={...I,y:T?I.y:Math.min(170,I.y+M*5)},b.t=x}y.propose(I,U,(z,T)=>{z?b.waiting=0:(b.t=x,b.waiting+=M,b.waiting>2.5&&(b.dir=-b.dir,b.waiting=0)),P.position.set(T.x,T.y,T.z),P.rotation.set(0,T.heading,0)})}E.getTangentAt((b.t+.015)%1,o),b.collider||P.rotation.set(0,Math.atan2(a.x,a.z),0);let C=Math.atan2(o.x,o.z)-Math.atan2(a.x,a.z),w=Math.atan2(Math.sin(C),Math.cos(C));b.roll+=(Tt.clamp(w*12,-.55,.55)-b.roll)*(M>0?Math.min(1,M*3):1),b.body.rotation.z=b.roll}return{setTier(b){_=b},setTemplate(b){if(!b)return;m=!0;let M=f;f=new Map;for(let[E,P]of r.entries()){P.body.clear(),P.rotors.length=0;let x=b.clone(!0);x.scale.setScalar(1.9),x.updateMatrixWorld(!0);let C=new en().setFromObject(x);if(C.expandByScalar(.5),C.min.y-=1,C.max.y+=1,x.traverse(w=>{w.isMesh&&(w.castShadow=!1,w.receiveShadow=!1,w.material=Array.isArray(w.material)?w.material.map(v):v(w.material)),/^rotor_/.test(w.name)&&P.rotors.push(w)}),P.body.add(x),P.root.visible=E<(_==="low"?2:_==="medium"?3:6),e.traffic&&!P.collider){S(P,0);let w=qi({min:C.min.toArray(),max:C.max.toArray()});P.root.position.y=Math.max(P.root.position.y,e.traffic.ceiling(w,P.root.position.x,P.root.position.z)),P.collider=e.traffic.register("drone-"+E,w,{...P.root.position,heading:P.root.rotation.y},0),P.collider||(P.root.visible=!1)}}M.forEach(E=>E.dispose())},update(b,M,E){g=E;let P=E?Math.min(.1,Math.max(0,b)):0;r.forEach((x,C)=>{if(x.root.visible=m&&C<(_==="low"?2:_==="medium"?3:6)&&(!e.traffic||!!x.collider),x.collider&&(x.root.visible&&!x.collider.enabled&&!e.traffic.relocate(x.collider,x.collider)&&(x.root.visible=!1),x.collider.enabled=x.root.visible),!x.root.visible)return;S(x,P),x.rotors.forEach((I,y)=>{I.rotation.y+=P*(y%2?-46:46)});let w=Math.sin(M*5+C*1.7);x.lights[0].visible=x.lights[1].visible=!E||w>-.2,x.lights[2].visible=!E||w>.93,x.lights[2].scale.setScalar(E?1.6:1)})},stats:()=>({drones:r.filter(b=>b.root.visible).length,animated:g,positions:r.map(b=>b.root.position.toArray().map(M=>Math.round(M))),obstructions:r.map(b=>b.obstruction)}),dispose(){t.removeFromParent(),n.forEach(b=>b.dispose()),i.forEach(b=>b.dispose()),f.forEach(b=>b.dispose()),r.forEach(b=>{b.collider&&e.traffic.remove(b.collider.id)}),r.length=0}}}var Fn={xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80},xt={ground:0,road:.12,pavement:.5,quay:.08,room:.16,gallery:4.16},_n=["agent","memory","missions"].map((s,e)=>{let t=[0,-43,43][e],n=73;return{id:s,x:t,z:n,width:12,depth:12,front:-1,doorZ:n-6,liftX:t+4,liftZ:n+3}}),Zn=[{id:"infra",x:-67,z:-53,platformX:-74,platformZ:-53,angle:Math.PI/2},{id:"memory",x:-67,z:-10,platformX:-74,platformZ:-10,angle:Math.PI/2},{id:"graph",x:-43,z:59,platformX:-43,platformZ:64.5,angle:0},{id:"operations",x:0,z:59,platformX:0,platformZ:64.5,angle:0},{id:"missions",x:43,z:59,platformX:43,platformZ:64.5,angle:0},{id:"integrations",x:67,z:-10,platformX:74,platformZ:-10,angle:-Math.PI/2},{id:"agent",x:0,z:-77,platformX:0,platformZ:-84,angle:Math.PI}],Fu=[[-67,-77],[-67,59],[67,59],[67,-77]],Cp=(s,e)=>{let t=s*Math.PI/180;return{x:Math.sin(t)*e,z:-12-Math.cos(t)*e,angle:Math.PI-t}},Nt={x:0,z:-12,floor:72.2,inner:4.2,outer:8,bridge:1,lift:{x:9.8,z:-12,base:.88,cab:.03,travel:71.32},telescopes:[45,135,225,315].map(s=>({id:"deck-"+s,...Cp(s,7.25)})),benches:[0,180,270].map(s=>Cp(s,6.6))};function Ip(s,e,t){let n=Math.hypot(s-Nt.x,e-Nt.z);return n>=Nt.inner&&n<=Nt.outer?!0:t&&s>=Nt.outer-.5&&s<=Nt.lift.x+1&&Math.abs(e-Nt.lift.z)<Nt.bridge}var Ri={x:78,z:30},Dp=[{id:"south",x:-11,z:64.3,angle:Math.PI},{id:"harbour",x:23.3,z:75,angle:-Math.PI/2},{id:"west",x:-72.3,z:-22,angle:Math.PI/2},{id:"east",x:72.3,z:21,angle:-Math.PI/2},{id:"north",x:-8,z:-26.7,angle:Math.PI}],Ac=[{x:30.5,z:-55,scale:1},{x:49,z:-55,scale:1.25},{x:-3,z:-57,scale:1.2}];function Pp(s,e,t=Fn.halfWidth){return s>=Fn.minX&&s<=Fn.maxX&&e>=Fn.minZ&&e<=Fn.maxZ&&(Fn.xs.some(n=>Math.abs(s-n)<=t)||Fn.zs.some(n=>Math.abs(e-n)<=t))}function Lp(s,e){return Pp(s,e)?Pp(s,e,Fn.laneHalfWidth)?xt.road:xt.pavement:xt.ground}function K_(s,e,t,n=0){return e>=s.x-5.85+n&&e<=s.x+2.4-n&&t>=s.z+n&&t<=s.z+5.85-n}function Fp(s,e,t,n=0){return Math.abs(e-s.liftX)<=1.6-n&&Math.abs(t-s.liftZ)<=1.6-n}function Np(){let s=[],e=[2.4,1.4,5.6,4.6],t=(n,i,r,a)=>{r>n&&a>i&&s.push({x:(n+r)/2,z:(i+a)/2,sx:(r-n)/4,sz:(a-i)/4})};for(let n=-4;n<=4;n+=4)for(let i=-4;i<=4;i+=4){let[r,a,o,l]=[n-2,i-2,n+2,i+2],[c,h,u,d]=[Math.max(r,e[0]),Math.max(a,e[1]),Math.min(o,e[2]),Math.min(l,e[3])];if(c>=u||h>=d){t(r,a,o,l);continue}t(r,a,c,l),t(u,a,o,l),t(c,a,u,h),t(c,d,u,l)}return s}function Nu(s,e,t,n){return K_(s,e,t,.25)?!0:n?e>=s.x+1.8&&e<=s.liftX+1.25&&Math.abs(t-s.liftZ)<1.25:!1}var Uu={agent:[[-8,-8,8,8]],memory:[[-11.4,-10.4,11.4,8.4]],integrations:[[-11.4,-4.9,-4.6,4.9],[4.6,-4.9,11.4,4.9],[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]],missions:[[-13.4,-8.9,13.4,6.9]],infra:[[-10.7,-9.4,10.7,3.4],[-10,-2.6,10,8.5]]},Y_={agent:[23,23],memory:[26,22],integrations:[26,16],missions:[30,21],infra:[25,23],graph:[24,24],operations:[9,9]},Ou={agent:[[5.9,8,13],[5.4,13,29],[4.7,29,42.5],[4.2,42.5,61.5],[3.6,61.5,72.2],[2.6,72.2,87]]};function Up(s,e,t){for(let n of t){let i=Y_[n.id];if(i&&Math.abs(s-n.x)<i[0]/2&&Math.abs(e-n.z)<i[1]/2)return .88}return 0}function J_(s,e,t){for(let n of t){let i=s-n.x,r=e-n.z;if(n.id==="graph"&&Math.hypot(i,r)<10.2||n.id==="operations"&&Math.hypot(i,r)<3||(Uu[n.id]||[]).some(([a,o,l,c])=>i>a&&i<l&&r>o&&r<c))return!0}return Ac.some(n=>Math.abs(s-n.x)<6&&Math.abs(e-n.z)<5)}function ms(s,e){return _n.find(t=>Math.abs(s-t.x)<t.width/2&&Math.abs(e-t.z)<t.depth/2)}function gs(s,e){if(s>=-81.5&&s<=-78.5&&e>=46&&e<=52)return(52-e)/3;if(s>=-78.3&&s<=-75.3&&e>=46&&e<=52)return Math.min(2,Math.ceil((52-e)*2)/6);if(s>=-82&&s<=-75&&e>=34&&e<46)return 2;if(s>=-80&&s<=-77&&e>=28&&e<34)return(e-28)/3;if(ms(s,e))return xt.room;for(let t of Zn){let n=!!t.angle&&Math.abs(t.angle)!==Math.PI;if(Math.abs(s-t.platformX)<(n?2:4.5)&&Math.abs(e-t.platformZ)<(n?4.5:2))return xt.pavement+.3}return Math.max(Lp(s,e),e>=74&&e<=80&&s>=-80&&s<=80?xt.quay:xt.ground)}function zu(s,e,t,n,i){if(Math.abs(s)>84||e<-90||e>79)return!1;let r=ms(s,e),a=ms(t.x,t.z);if(r||a){let o=r||a;return!(r?.id!==a?.id&&(Math.abs(s-o.x)>1.15||Math.abs(t.x-o.x)>1.15||Math.min((e-o.doorZ)*o.front,(t.z-o.doorZ)*o.front)<-.9||!n(o.id))||r&&(Math.abs(s-o.x)>5.55||(e-o.z)*o.front<-5.55))}return!J_(s,e,i)}function Rc(s,e=new Date){let t=s==="day"?12:s==="evening"?18.5:s==="night"?0:e.getHours()+e.getMinutes()/60;return{hour:t,amount:Math.max(0,Math.min(1,Math.sin((t-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(t-18.5)/2)}}var bs=[{id:"repair-bay",district:"infra",x:-28,z:-62,role:"technician"},{id:"parcel-sorter",district:"missions",x:35,z:49,role:"courier"},{id:"relay-mast",district:"integrations",x:57,z:-20,role:"archivist"},{id:"kinetic-fountain",district:"graph",x:-28,z:47,role:"archivist"},{id:"glass-garden",district:"graph",x:40,z:-68,role:"technician"},{id:"meeting-charge",district:"operations",x:8,z:46,role:"courier"}],Op=[{id:"data-monument",x:0,z:24,y:0,angle:0,clip:"spin"},{id:"neon-gate",x:-18,z:0,y:.5,angle:0,clip:"pulse"},{id:"sky-pod",x:56,z:47,y:0,angle:0,clip:"scan"},{id:"tidal-turbine",x:0,z:96,y:-3.2,angle:Math.PI,clip:"spin"}];function zp(s,e){let t=[],n=new Map,i=-1;for(let a of[-73,-61,-24,-12,12,24,61,73])for(let o of[-83,-71,-38,-26,7,19,53,65])t.push({x:a,z:o,y:e(a,o),heading:0});function r(a,o){i!==s.revision()&&(n.clear(),i=s.revision());let l=a.circles.map(v=>[v.x,v.z,v.r].join(",")).join(";")+":"+a.maxY;if(!n.has(l)){let v=t.filter(b=>s.clear(a,b,b,!1)),S=v.map(()=>[]);for(let b=0;b<v.length;b++)for(let M=b+1;M<v.length;M++){let E=v[b],P=v[M],x=Math.hypot(E.x-P.x,E.z-P.z);x<48&&s.clear(a,E,P,!1)&&(S[b].push([M,x]),S[M].push([b,x]))}n.set(l,{nodes:v,edges:S})}let c=n.get(l),h=[...c.nodes,{x:a.x,y:a.y,z:a.z,heading:a.heading},{...o,heading:0}],u=h.length-2,d=h.length-1,p=c.edges.map(v=>[...v]);p.push([],[]);for(let v of[u,d])for(let S=0;S<v;S++){let b=h[v],M=h[S],E=Math.hypot(b.x-M.x,b.z-M.z);(E<48||S===u)&&s.clear(a,b,M,!1)&&(p[v].push([S,E]),p[S].push([v,E]))}let g=h.map(()=>1/0),_=h.map(()=>-1),m=new Set(h.map((v,S)=>S));for(g[u]=0;m.size;){let v=-1;for(let S of m)(v<0||g[S]<g[v])&&(v=S);if(!Number.isFinite(g[v])||v===d)break;m.delete(v);for(let[S,b]of p[v])g[v]+b<g[S]&&(g[S]=g[v]+b,_[S]=v)}if(!Number.isFinite(g[d]))return[];let f=[];for(let v=d;v!==u;v=_[v]){if(v<0)return[];f.unshift(h[v])}return f}return{route:r,points:t}}function Bp(s){let e=new gt;e.name="city-conversations",s.add(e);let t=new Bn(1,.025,5,40,Math.PI*1.45),n=new os(.92,1,48),i=new k(0,0,1),r=new k,a=[],o=0;for(let l=0;l<8;l++){let c=[];for(let h=0;h<4;h++){let u=new Lt({color:8445392,transparent:!0,opacity:0,blending:qt,depthWrite:!1,side:Ot,toneMapped:!1}),d=new it(h===3?n:t,u);d.visible=!1,e.add(d),c.push(d)}a.push({meshes:c,age:10,from:null,to:null})}return{send(l,c,h="ambient"){let u=a.find(d=>d.age>=2.4);return u?(Object.assign(u,{from:l,to:c,age:0,source:h}),o++,u.meshes.forEach(d=>d.material.color.setHex(h==="live"?16105851:8445392)),!0):!1},update(l,c=!0){for(let h of a){if((!c||h.from?.enabled===!1||h.to?.enabled===!1)&&(h.age=10),h.age+=l,h.meshes.forEach(m=>{m.visible=!1}),h.age>=2.4)continue;let u=h.from,d=h.to,p=d.x-u.x,g=d.z-u.z;r.set(p,d.y+Math.min(2,d.maxY*.7)-(u.y+Math.min(2,u.maxY*.7)),g).normalize();for(let m=0;m<3;m++){let f=(h.age-m*.18)/1.55;if(f<0||f>1)continue;let v=h.meshes[m];v.visible=!0,v.position.set(u.x+p*f,Tt.lerp(u.y+Math.min(2,u.maxY*.7),d.y+Math.min(2,d.maxY*.7),f)+Math.sin(f*Math.PI)*.65,u.z+g*f),v.quaternion.setFromUnitVectors(i,r),v.rotateZ(m*.65),v.scale.setScalar(.25+Math.sin(f*Math.PI)*.65),v.material.opacity=Math.sin(Math.PI*f)*.75}let _=(h.age-1.55)/.85;if(_>0){let m=h.meshes[3];m.visible=!0,m.position.set(d.x,d.y+Math.min(2,d.maxY*.7),d.z),m.quaternion.setFromUnitVectors(i,r),m.scale.setScalar(1.25*(1-_)+.12),m.material.opacity=Math.sin(_*Math.PI)*.85}}},clear(){a.forEach(l=>{l.age=10,l.meshes.forEach(c=>{c.visible=!1})})},stats:()=>({sent:o,active:a.filter(l=>l.age<2.4).length,sources:a.filter(l=>l.age<2.4).map(l=>l.source)}),dispose(){e.removeFromParent(),t.dispose(),n.dispose(),a.forEach(l=>l.meshes.forEach(c=>c.material.dispose()))}}}var kp=(s,e,t)=>s+Math.atan2(Math.sin(e-s),Math.cos(e-s))*Math.min(1,t*5),Hp=(s,e,t)=>{let n=s.play?.(e);n&&(n.timeScale=t/1.65)};function Gp(s,{traffic:e,camera:t,floor:n,onSelect:i,onDemonstrate:r,active:a,onGait:o}){let l=[],c=new Map,h=[],u=[],d=zp(e,n),p=Bp(s),g=0,_=null,m=!1,f="high",v=!1,S=0,b="",M=()=>f==="low"?3:f==="medium"?10:19;function E(R){R.slot&&(R.slot.owner=null,R.slot=null),R.path=[],R.goal=null}function P(R){return R?{id:R.id,role:R.role,state:R.state,source:R.source||"ambient",guided:!!R.guide}:null}function x(R){_=R?.id||null,b=R?.state||"",i?.(P(R))}function C(R,U,z){let T="resident-"+U,G=qi(z,1.15),X=["courier","technician","archivist"][U%3],Z=null;G.circles=[{x:0,z:0,r:G.reach}];let oe=[...h.filter(Fe=>Fe.role===X).map(Fe=>({x:Fe.x,z:Fe.z+2.2,y:n(Fe.x,Fe.z+2.2),heading:Math.PI})),...d.points.map((Fe,We)=>d.points[(U*7+We)%d.points.length])];for(let Fe of oe)if(Z=e.register(T,G,Fe,1),Z)break;if(!Z)return R.node.visible=!1,null;let ke={...R,id:T,index:U,body:Z,role:X,state:"idle",path:[],until:U*.3,trip:U,cooldown:8+U*.7,source:"ambient",carrying:!1,parcels:[],soundX:Z.x,soundZ:Z.z};return R.node.traverse(Fe=>{Fe.isMesh&&/parcel/.test(Fe.name)&&(ke.parcels.push(Fe),Fe.visible=!1)}),l.push(ke),R.node.position.set(Z.x,Z.y,Z.z),ke}function w(R,U,z,T){let G={id:R,body:U,node:z,pause:T,role:"patrol",state:"patrol",cooldown:7+l.length,path:[],soundX:U.x,soundZ:U.z};return l.push(G),G}function I(R){E(R),R.source="ambient",R.state="idle",R.until=g+2;let U=h.filter(z=>!z.owner&&(R.role==="courier"?z.place===(R.carrying?"meeting-charge":"parcel-sorter"):z.role===R.role||z.place==="meeting-charge"));for(let z=0;z<U.length;z++){let T=U[(R.trip+z)%U.length],G=d.route(R.body,T);if(G.length){T.owner=R.id,R.slot=T,R.path=G,R.goal=T,R.trip++,R.state=R.carrying?"carry":"walk";return}}for(let z=0;z<8;z++){let T=d.points[(R.trip++*13+R.index)%d.points.length],G=d.route(R.body,T);if(G.length){R.path=G,R.state="walk";return}}}function y(R){for(let U of[R.a,R.b])E(U),U.meeting=null,U.cooldown=g+12+(U.index||0),U.pause?.(!1),U.state=U.pause?"patrol":"idle",U.until=g+1}function N(){u.forEach(y),u.length=0,p.clear(),_=null,i?.(null);for(let R of l)R.guide=null,R.pause?.(!1,R.body),R.trafficYield=!1,R.yieldTram=null,R.pause||(E(R),R.state="idle",R.until=g+1)}function D(R,U){if(v)return;U&&(g+=R);for(let T of l){let G=T.pause||T.index<M();if(G&&!T.body.enabled){if(!e.relocate(T.body,T.body)){T.node.visible=!1;continue}T.body.enabled=!0,T.node.position.set(T.body.x,T.body.y,T.body.z)}!G&&T.body.enabled&&(E(T),T.body.enabled=!1,T.meeting&&y(T.meeting)),T.node.visible=!!G,T.soundX==null&&(T.soundX=T.body.x,T.soundZ=T.body.z);let X=Math.hypot(T.body.x-T.soundX,T.body.z-T.soundZ);if(T.soundX=T.body.x,T.soundZ=T.body.z,o?.(T.id,T.body.x,T.body.y,T.body.z,X>.004,!!T.carrying,T.role==="patrol"?"glide":""),!G||!U)continue;let Z=e.neighbors(T.body,22).sort((H,oe)=>Math.hypot(H.x-T.body.x,H.z-T.body.z)-Math.hypot(oe.x-T.body.x,oe.z-T.body.z)).find(H=>{if(!H.id.startsWith("tram-"))return!1;let oe=T.body.x-H.x,ke=T.body.z-H.z,Fe=Math.cos(H.heading),We=Math.sin(H.heading);return Math.abs(oe*Fe-ke*We)<T.body.reach+(T.trafficYield?4:2.3)&&oe*We+ke*Fe>-8-T.body.reach&&oe*We+ke*Fe<14});if(!Z&&T.yieldTram){let H=e.body(T.yieldTram),oe=H?T.body.x-H.x:0,ke=H?T.body.z-H.z:0;if(H?.enabled&&Math.hypot(oe,ke)<26&&oe*Math.sin(H.heading)+ke*Math.cos(H.heading)>-H.reach-T.body.reach-1){T.pause?.(!0),T.play?.("idle");continue}T.yieldTram=null}if(Z){if(T.trafficYield=!0,T.yieldTram=Z.id,T.meeting){let le=T.meeting;y(le),u.splice(u.indexOf(le),1)}T.pause?.(!0);let H=Math.cos(Z.heading),oe=Math.sin(Z.heading),ke=Math.sign((T.body.x-Z.x)*H-(T.body.z-Z.z)*oe)||1,Fe=(T.body.x-Z.x)*H-(T.body.z-Z.z)*oe,We=Math.sign((T.body.x-Z.x)*oe+(T.body.z-Z.z)*H)||1,fe=[[H*ke,-oe*ke,T.body.reach+4.5-Math.abs(Fe)],[-H*ke,oe*ke,T.body.reach+4.5+Math.abs(Fe)],[oe*We,H*We,1.2]],re=Array.from({length:16},(le,se)=>[Math.cos(se*Math.PI/8),Math.sin(se*Math.PI/8),2]),Pe=([le,se])=>le*(T.body.x-Z.x)+se*(T.body.z-Z.z);fe.push(...re.filter(le=>Pe(le)>.2).sort((le,se)=>Pe(se)-Pe(le)));let be=T.escape&&g<T.escape.until?T.escape:null,ge=be?fe.filter(le=>le[0]===be.x&&le[1]===be.z).concat(fe.filter(le=>!(le[0]===be.x&&le[1]===be.z))):fe,ie=!1;for(let[le,se,ce]of ge){let ve={x:T.body.x+le*ce,y:T.body.y,z:T.body.z+se*ce,heading:T.body.heading};if(!e.clear(T.body,T.body,ve))continue;let Ee={...ve,x:T.body.x+le*R*1.8,z:T.body.z+se*R*1.8};Ee.y=T.pause?T.body.y:n(Ee.x,Ee.z),e.propose(T.body,Ee,(qe,Ye)=>{T.node.position.set(Ye.x,Ye.y,Ye.z),Hp(T,"walk",1.8)}),T.escape={x:le,z:se,until:g+.8},T.state="yield",T.cooldown=g+5,ie=!0;break}ie||(T.play?.("idle"),T.state="yield");continue}if(T.trafficYield&&(T.trafficYield=!1,T.pause?.(!1,T.body),T.state=T.pause?"patrol":T.path.length?"walk":"idle"),!T.meeting){if(T.pause){T.state==="greet"&&g>T.until&&(T.pause(!1),T.state="patrol");continue}if(T.guide&&Math.hypot(t.position.x-T.body.x,t.position.z-T.body.z)>9){T.play("idle");continue}if(T.path.length){let H=T.path[0],oe=H.x-T.body.x,ke=H.z-T.body.z,Fe=Math.hypot(oe,ke),We=T.guide?1.9:1.65;if(Fe<.18){T.path.shift();continue}let fe=Math.min(Fe,We*R),re={x:oe/Fe,z:ke/Fe},Pe=T.body.x+re.x*fe,be=T.body.z+re.z*fe,ge=kp(T.body.heading,Math.atan2(oe,ke),R),ie={x:T.body.x+re.x*1.8,y:n(Pe,be),z:T.body.z+re.z*1.8,heading:ge};if(e.clear(T.body,T.body,ie))T.detour=null;else{let se=[[re.z,-re.x],[-re.z,re.x]],ce=T.detour&&g<T.detour.until?se.findIndex(qe=>qe[0]===T.detour.x&&qe[1]===T.detour.z):-1,ve=ce>=0?[se[ce],...se.filter((qe,Ye)=>Ye!==ce)]:se,Ee=!1;for(let[qe,Ye]of ve){let st={x:T.body.x+qe*1.3,y:T.body.y,z:T.body.z+Ye*1.3,heading:T.body.heading};if(!e.clear(T.body,T.body,st))continue;let at={x:st.x+re.x*1.8,y:n(st.x,st.z),z:st.z+re.z*1.8,heading:T.body.heading};if(e.clear(T.body,st,at)){Pe=T.body.x+qe*fe,be=T.body.z+Ye*fe,T.detour={x:qe,z:Ye,until:g+1.2},Ee=!0;break}}Ee||(Pe=T.body.x,be=T.body.z)}e.clear(T.body,T.body,{x:Pe,y:n(Pe,be),z:be,heading:ge},!1)||(ge=T.body.heading);let le=Math.hypot(Pe-T.body.x,be-T.body.z)>.001;e.propose(T.body,{x:Pe,y:n(Pe,be),z:be,heading:ge},(se,ce)=>{T.node.position.set(ce.x,ce.y,ce.z),T.node.rotation.y=ce.heading,!se||!le?(T.wait=(T.wait||0)+R,T.play("idle"),T.state="yield"):(T.wait=0,T.state=T.guide?"guide":T.carrying?"carry":"walk",Hp(T,T.carrying?"carry":"walk",We)),T.wait>2.5&&(T.wait=0,E(T),T.state=T.guide?"guide":"idle",T.until=g+.5,T.guide&&(T.path=d.route(T.body,T.guide)))})}else if(T.state==="walk"||T.state==="carry"||T.state==="guide"||T.state==="yield"){if(T.guide&&Math.hypot(T.body.x-T.guide.x,T.body.z-T.guide.z)>.5){T.play("idle"),g>T.until&&(T.path=d.route(T.body,T.guide),T.until=g+2.5);continue}T.guide=null,T.state=T.slot?.place==="meeting-charge"?"charge":T.slot?"work":"idle",T.until=g+5+T.index%4,T.play(T.state==="work"?"work":"idle"),T.slot&&r?.(T.slot.place,5),S++}else g>T.until&&(T.role==="courier"&&T.slot&&(T.state==="work"||T.state==="charge")&&(T.carrying=T.slot.place==="parcel-sorter",T.parcels.forEach(H=>{H.visible=T.carrying})),I(T))}}if(!U){p.update(0,!1);return}for(let T of l){if(!T.body.enabled||T.trafficYield||T.meeting||T.guide||T.state==="greet"||g<T.cooldown)continue;let G=l.find(Z=>Z!==T&&Z.body.enabled&&!Z.trafficYield&&Z.state!=="greet"&&!Z.meeting&&!Z.guide&&g>=Z.cooldown&&Math.abs(T.body.y-Z.body.y)<2&&Math.hypot(T.body.x-Z.body.x,T.body.z-Z.body.z)>2.8&&Math.hypot(T.body.x-Z.body.x,T.body.z-Z.body.z)<6);if(!G)continue;let X={a:T,b:G,at:g,sent:!1,replied:!1};u.push(X);for(let Z of[T,G])Z.meeting=X,Z.state="exchange",Z.pause?.(!0),Z.play?.("greet")}for(let T=u.length-1;T>=0;T--){let G=u[T],X=g-G.at;if(X>4.8||!G.a.body.enabled||!G.b.body.enabled){y(G),u.splice(T,1);continue}for(let[Z,H]of[[G.a,G.b],[G.b,G.a]])e.propose(Z.body,{...Z.body,heading:kp(Z.body.heading,Math.atan2(H.body.x-Z.body.x,H.body.z-Z.body.z),R)},(oe,ke)=>{Z.node.rotation.y=ke.heading});X>.6&&!G.sent&&(p.send(G.a.body,G.b.body),G.sent=!0),X>2.4&&!G.replied&&(p.send(G.b.body,G.a.body),G.replied=!0)}p.update(R,!0);let z=l.find(T=>T.id===_);z&&z.state!==b&&(b=z.state,i?.({...P(z),refresh:!0}))}return{add:C,patrol:w,update:D,suspend:N,addPlace(R,U){if(!c.has(R.id)){c.set(R.id,R);for(let z of U.navigation.workpoints||[])h.push({id:R.id+":"+z.id,place:R.id,role:R.role,x:R.x+z.position[0],y:n(R.x,R.z),z:R.z+z.position[2],owner:null})}},nearby(){let R=[];for(let U of l)if(U.body.enabled){let z=Math.hypot(t.position.x-U.body.x,t.position.z-U.body.z);z<5&&Math.abs(t.position.y-U.body.y)<8&&R.push({kind:"resident",id:U.id,distance:z,x:U.body.x,z:U.body.z})}for(let U of c.values()){let z=Math.hypot(t.position.x-U.x,t.position.z-U.z);z<7&&R.push({kind:"demonstrate",id:U.id,distance:z,x:U.x,z:U.z})}return R},inspect(R){let U=l.find(z=>z.id===R);return x(U),P(U)},action(R,U){let z=l.find(T=>T.id===_);if(!z)return null;if(z.meeting){let T=z.meeting,G=u.indexOf(T);y(T),G>=0&&u.splice(G,1)}if(R==="greet"&&(E(z),z.guide=null,z.pause?.(!0),z.state="greet",z.until=g+3,z.cooldown=g+5,z.play?.("greet")),R==="guide"&&!z.pause&&U){E(z);for(let T of[0,1.5,3]){for(let G=0;G<(T?8:1)&&!z.path.length;G++){let X=U.x+Math.cos(G*Math.PI/4)*T,Z=U.z+Math.sin(G*Math.PI/4)*T;z.path=d.route(z.body,{x:X,z:Z,y:n(X,Z)})}if(z.path.length)break}z.guide=z.path.length?z.path[z.path.length-1]:null,z.state=z.guide?"guide":"idle"}return R==="cancel"&&(E(z),z.guide=null,z.pause?.(!1),z.state=z.pause?"patrol":"idle",z.until=g+1),x(z),P(z)},demonstrate(R,U=!1){c.has(R)&&r?.(R,U?0:6)},setTier(R){f=R},setReplay(R){m!==R&&(m=R,N())},event(R){if(m||!a()||!["started","succeeded","failed","sanitized","progress"].includes(R.state)||!Number.isFinite(R.at)||Date.now()-R.at>2500||R.at>Date.now())return;let U=bs.find(z=>z.district===(R.to==="agent"?R.from:R.to));U&&c.has(U.id)&&r?.(U.id,2,R.state)},stats:()=>({completed:S,meetings:u.length,reservations:h.filter(R=>R.owner).length,signals:p.stats(),residents:l.filter(R=>R.body.enabled).map(R=>({...P(R),x:R.body.x,z:R.body.z,heading:R.body.heading})),places:[...c.keys()]}),dispose(){v=!0,N(),l.forEach(R=>e.remove(R.id)),l.length=0,c.clear(),h.length=0,p.dispose()}}}function Vp(s){let e=new gt;e.name="living-machinery",s.add(e);let t=new Set,n=new Set,i=new Map,r=0,a="high",o=S=>(t.add(S),S),l=S=>(n.add(S),S),c=o(new Bn(1,.025,6,64)),h=o(new Sr(1,48)),u=o(new bt),d=new Float32Array(288),p=new Float32Array(96);for(let S=0;S<96;S++)d[S*3]=Math.cos(S*2.399)*Math.sqrt(S/96),d[S*3+2]=Math.sin(S*2.399)*Math.sqrt(S/96),p[S]=S/96;u.setAttribute("position",new Mt(d,3)),u.setAttribute("phase",new Mt(p,1));for(let S of bs){let b=new gt;b.name=S.id,b.position.set(S.x,.12,S.z),b.visible=!1,e.add(b);let M=l(new Lt({color:7529695,transparent:!0,opacity:.15,depthWrite:!1,blending:qt,toneMapped:!1})),E=[];for(let C=0;C<3;C++){let w=new it(c,l(M.clone()));w.rotation.x=-Math.PI/2,b.add(w),E.push(w)}let P=new it(h,l(M.clone()));P.rotation.x=-Math.PI/2,P.scale.setScalar(2.5),P.position.y=.2,b.add(P);let x=new fn(u,l(new vt({transparent:!0,depthWrite:!1,blending:qt,uniforms:{time:{value:0},strength:{value:0},color:{value:new Le(10414816)}},vertexShader:"attribute float phase;uniform float time;uniform float strength;varying float a;void main(){float t=fract(phase+time*.31);vec3 p=position*vec3(2.,1.,2.);p.y=t*3.;p.xz*=1.-t*.7;vec4 v=modelViewMatrix*vec4(p,1.);a=sin(t*3.14159)*strength;gl_PointSize=clamp(65./max(1.,-v.z),1.,5.);gl_Position=projectionMatrix*v;}",fragmentShader:"uniform vec3 color;varying float a;void main(){float d=length(gl_PointCoord-.5);gl_FragColor=vec4(color,a*(1.-smoothstep(.05,.5,d)));}"})));b.add(x),i.set(S.id,{group:b,waves:E,base:P,motes:x,district:S.district,started:0,until:0,source:"ambient",model:null})}let g=new it(o(new pn(3.9,2.3)),l(new vt({transparent:!0,depthWrite:!1,side:Ot,blending:qt,uniforms:{time:{value:0},strength:{value:0}},vertexShader:"varying vec2 q;void main(){q=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 q;uniform float time;uniform float strength;void main(){float line=pow(max(0.,1.-abs(q.y-fract(time*.4))*14.),3.);float grid=step(.94,fract(q.x*24.))*step(.9,fract(q.y*18.));gl_FragColor=vec4(.18,.9,.8,(line*.36+grid*.1)*sin(q.x*3.14159)*strength);}"})));g.position.set(0,1.4,.8),i.get("repair-bay").group.add(g);let _=i.get("kinetic-fountain"),m=l(new vt({transparent:!0,depthWrite:!1,side:Ot,uniforms:{time:{value:0}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 vUv;uniform float time;void main(){float pulse=.5+.5*sin(vUv.x*95.-time*9.);float edge=.35+.65*pow(max(0.,sin(vUv.y*3.14159)),2.);gl_FragColor=vec4(mix(vec3(.09,.35,.42),vec3(.6,.95,1.),pow(max(0.,pulse),5.)),edge*.56);}"}));for(let S=0;S<8;S++){let b=S*Math.PI/4,M=new k(Math.cos(b)*2.3,.5,Math.sin(b)*2.3),E=new k(Math.cos(b+.5)*.7,.55,Math.sin(b+.5)*.7),P=new si(M,new k(Math.cos(b)*1.5,3.7,Math.sin(b)*1.5),E);_.group.add(new it(o(new Ma(P,24,.045,5,!1)),m))}let f=l(new vt({transparent:!0,depthWrite:!1,uniforms:{time:{value:0}},vertexShader:"varying vec2 q;uniform float time;void main(){q=position.xy;vec3 p=position;p.z+=sin(length(q)*18.-time*4.)*.012;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.);}",fragmentShader:"varying vec2 q;uniform float time;void main(){float r=length(q);float a=.5+.5*sin(r*16.-time*3.+sin(q.x*3.+q.y*2.+time*.7)*.35);float rim=1.-smoothstep(2.35,2.58,r);gl_FragColor=vec4(mix(vec3(.03,.13,.17),vec3(.22,.55,.58),pow(max(0.,a),9.)*.3),.78*rim);}"})),v=new it(o(new Sr(2.58,64)),f);return v.rotation.x=-Math.PI/2,v.position.y=.3,_.group.add(v),{attach(S,b){let M=i.get(S);M&&(M.model=b,M.group.visible=!0)},demonstrate(S,b=6,M=null){let E=i.get(S);E&&(E.until<=r&&(E.started=r),E.until=r+b,E.source=M?"live":"ambient",E.status=M,E.model?.play("operate"))},clear(){i.forEach(S=>{S.until=0,S.source="ambient",S.status=null})},setTier(S){a=S},update(S,b){b&&(r+=S),m.uniforms.time.value=f.uniforms.time.value=g.material.uniforms.time.value=r;for(let[M,E]of i){let P=E.until>r,x=M==="kinetic-fountain",C=P?Tt.smoothstep(r-E.started,0,.35)*Tt.smoothstep(E.until-r,0,.65):0,w=x?Math.max(.25,C):C;P||(E.source="ambient",E.status=null);let I=E.status==="failed"?16737881:E.source==="live"?{infra:7978495,integrations:7728086,missions:10268415,graph:16765844,operations:16758915}[E.district]:7529695;if(E.waves.forEach((y,N)=>{y.visible=P;let D=(r*.6+N/3)%1;y.position.y=.4+D*2,y.scale.setScalar(.4+D*2.6),y.material.opacity=(1-D)*.22*C,y.material.color.setHex(I)}),M==="repair-bay"&&(g.visible=P,g.material.uniforms.strength.value=C),M==="relay-mast"&&P){let y=E.model?.node.getObjectByName("antenna");y&&(y.rotation.y=Tt.damp(y.rotation.y,Math.atan2(-E.group.position.x,-12-E.group.position.z),3,b?S:0))}E.base.material.opacity=.08*C,E.motes.visible=a!=="low"&&w>0,E.motes.material.uniforms.time.value=r,E.motes.material.uniforms.strength.value=w,E.model?.mixer&&(E.model.mixer.timeScale=b&&(P||x)?.65:0)}},stats:()=>({places:[...i].filter(([,S])=>S.model).map(([S,b])=>({id:S,operating:b.until>r,source:b.source})),time:r}),dispose(){e.removeFromParent(),t.forEach(S=>S.dispose()),n.forEach(S=>S.dispose()),i.clear()}}}var Z_={cruise:12,boost:26,climb:7,sink:7,ease:5,eye:2.4,takeoff:1.5,takeoffTime:.7,ceiling:150,minX:-140,maxX:140,minZ:-150,maxZ:130,glide:8,descend:6,reach:40,bob:.03,slack:15},At=Z_,Vs=(s,e,t)=>Math.max(e,Math.min(t,s)),$_=s=>s<=0?0:s>=1?1:s*s*(3-2*s);function Wp({clear:s,ground:e,walkable:t,reduced:n=()=>!1}){let i="idle",r=!1,a=0,o=0,l=0,c=0,h=0,u=0,d=0,p=0,g=0,_=null,m=0,f=!1,v=0,S=0,b=(y,N)=>e(y,N)+At.eye,M=()=>({x:a,y:o+m,z:l}),E=()=>{c=h=u=0},P=(y,N,D)=>(y+=(N-y)*(1-Math.exp(-At.ease*D)),Math.abs(y)<.01&&!N?0:y);function x(y,N=[]){for(let D=0;D<=Math.max(At.maxX-At.minX,At.maxZ-At.minZ);D+=D<At.reach?1.5:4){let R=D?Math.ceil(D*Math.PI*2/(D<At.reach?1.5:4)):1;for(let U=0;U<R;U++){let z=U/R*Math.PI*2,T=a+Math.cos(z)*D,G=l+Math.sin(z)*D;if(T<At.minX||T>At.maxX||G<At.minZ||G>At.maxZ||N.some(Z=>Math.hypot(Z.x-T,Z.z-G)<1)||!t(T,G))continue;let X=b(T,G);if(!(y&&!s({x:T,y:Math.max(o,X),z:G},{x:T,y:X,z:G})))return{x:T,y:X,z:G,blocked:0,skipped:N}}}return null}let C=y=>{a=y.x,o=y.y,l=y.z,E(),m=0,i="idle",r=!0,_=null};function w(y){let N=a,D=l,R=Vs(a+c*y,At.minX,At.maxX),U=Vs(l+u*y,At.minZ,At.maxZ);R!==a+c*y&&(c=0),U!==l+u*y&&(u=0),R!==a&&(s({x:a,y:o,z:l},{x:R,y:o,z:l})?a=R:c=0),U!==l&&(s({x:a,y:o,z:l},{x:a,y:o,z:U})?l=U:u=0);let z=b(a,l);o<z&&(s({x:a,y:o,z:l},{x:a,y:z,z:l})?o=z:(a=N,l=D,c=u=0,z=b(a,l)));let T=Vs(o+h*y,z,At.ceiling);T!==o+h*y&&(h=0),T!==o&&(s({x:a,y:o,z:l},{x:a,y:T,z:l})?o=T:h=0)}function I(y){let N=_,D=N.x-a,R=N.z-l,U=Math.hypot(D,R),z=o-N.y;if(v+=y,v>S){let ke=x(!1);ke?C(ke):(i="flying",_=null);return}if(U<.05&&z<.02){C(N);return}let T=U>.05?Math.min(At.glide,Math.max(.8,U*1.4)):0,G=z>.02?Math.min(At.descend,Math.max(.6,z*1.8)):0,X=U>.05?D/U*Math.min(T*y,U):0,Z=U>.05?R/U*Math.min(T*y,U):0,H=-Math.min(G*y,Math.max(0,z)),oe={x:a,y:o,z:l};if(s(oe,{x:a+X,y:o+H,z:l+Z}))a+=X,o+=H,l+=Z,c=X/y,u=Z/y,h=H/y,N.blocked=0;else if((X||Z)&&s(oe,{x:a+X,y:o,z:l+Z}))a+=X,l+=Z,c=X/y,u=Z/y,h=0,N.blocked=0;else if(E(),U>=.05&&o<At.ceiling&&s(oe,{x:a,y:o+At.descend*y,z:l}))o+=At.descend*y,h=At.descend,N.blocked=0;else if((N.blocked+=y)>1){let ke=x(!0,[...N.skipped,N]);ke?_=ke:(i="flying",_=null)}}return{start(y){return a=y.x,o=Math.max(y.y,b(y.x,y.z)),l=y.z,E(),i="takeoff",r=!1,d=0,p=o,g=0,_=null,m=0,M()},place(y){a=y.x,o=y.y,l=y.z,E(),m=0},step(y,N={},D=0){if(y=Vs(y,0,.1),i==="landing"&&(+N.forward||+N.right||+N.up?f&&(i="flying",_=null):f=!0),i==="takeoff"){d=n()?At.takeoffTime:Math.min(At.takeoffTime,d+y);let R=p+At.takeoff*$_(d/At.takeoffTime);R>o&&s({x:a,y:o,z:l},{x:a,y:R,z:l})&&(o=R),d>=At.takeoffTime&&(i="flying")}else if(i==="landing")y>0&&I(y);else if(i==="flying"){let R=Vs(+N.forward||0,-1,1),U=Vs(+N.right||0,-1,1),z=Math.hypot(R,U);z>1&&(R/=z,U/=z);let T=N.boost?At.boost:At.cruise,G=Math.sin(D),X=Math.cos(D),Z=Vs(+N.up||0,-1,1);c=P(c,(-G*R+X*U)*T,y),u=P(u,(-X*R-G*U)*T,y),h=P(h,Z*(Z>0?At.climb:At.sink),y),w(y),g+=y,m=n()?0:Math.min(At.bob,Math.max(0,o-b(a,l)),At.ceiling-o)*Math.sin(g*2.1)}return M()},land(y=!1){let N=y||n();if(i==="idle")return!1;if(i==="landing"&&!N)return!0;let D=x(!N);return D?N?(C(D),!0):(_=D,i="landing",m=0,f=!1,v=0,S=At.slack+Math.max(Math.hypot(D.x-a,D.z-l)/At.glide,Math.max(0,o-D.y)/At.descend),!0):!1},cancel(){return i!=="landing"?!1:(i="flying",_=null,!0)},reset(){i="idle",r=!1,_=null,E(),m=0},state(){return{phase:i,flying:i!=="idle",landing:i==="landing",target:_?{x:_.x,z:_.z}:null,done:r,x:a,y:o+m,z:l,altitude:Math.max(0,o-b(a,l)),speed:Math.hypot(c,u),climb:h}}}}function qp(s,e){let t=new gt;t.name="world-2",s.add(t);let n=new Map,i=new Set,r=new Set,a=[],o=[],l=[],c=new Map,h=new Map,u=[],d=new Map,p=new Map,g=new Set,_=[],m=new Set,f=[],v=new Map,S=new AbortController,b=[],M=0,E=!1,P=!1,x=e.tier||"high",C=null,w=0,I=0,y=null,N=null,D=0,R=new Set,U=null,z=0,T="",G=new k,X=new k,Z=new k,H=e.camera,oe={ready:!1,lift:null,cab:null,value:0,motion:null,idle:0},ke=[],Fe=new Tn(0,0,0,"YXZ"),We=B=>Math.atan2(Math.sin(B),Math.cos(B)),fe=B=>B<=0?0:B>=1?1:B*B*(3-2*B),re=e.traffic||Ec(),Pe=Vp(s),be=new Map;function ge(B,W){let ee=gs(B,W);for(let ae of bs)for(let Y of C?.assets.find(Q=>Q.id===ae.id)?.navigation.surfaces||[]){let[Q,Ae,Ve,rt]=Y.rect;B>=ae.x+Q&&B<=ae.x+Ve&&W>=ae.z+Ae&&W<=ae.z+rt&&(ee=Math.max(ee,gs(ae.x,ae.z)+Y.height))}return ee}let ie=(B,W)=>Math.max(ge(B,W),Up(B,W,e.districts||[])),le=[],se=()=>re.body("visitor"),ce=new k,ve=Wp({ground:ie,reduced:()=>e.reduced(),clear:(B,W)=>e.clear?e.clear(B,W):!se()||re.clear(se(),{...B,heading:0},{...W,heading:0},!1),walkable:(B,W)=>{if(ms(B,W)||!zu(B,W,{x:B,z:W},()=>!1,e.districts||[]))return!1;let ee={x:B,y:ie(B,W)+2.4,z:W,heading:0};return!se()||re.clear(se(),ee,ee)}}),Ee=Gp(s,{traffic:re,camera:H,floor:ge,onSelect:e.onSociety,onDemonstrate:Pe.demonstrate,active:e.active,onGait:e.onGait}),qe=0,Ye=0;try{R=new Set(JSON.parse(localStorage.getItem("aurago.desktop.sysworld.discoveries")||"[]").filter(B=>Zn.some(W=>W.id===B)))}catch{}let st=Cu(Fu),at=st.getLength(),ot=Zn.map(B=>{let W=1/0,ee=0;for(let ae=0;ae<3e3;ae++){st.getPointAt(ae/3e3,G);let Y=Math.hypot(G.x-B.x,G.z-B.z);Y<W&&(W=Y,ee=ae/3e3)}return{...B,u:ee}}),J=(B,W,ee)=>e.onSound?.(B,W?.x||0,W?.y||0,W?.z||0,ee),dt=new Rn({color:11834208,metalness:.8,roughness:.35});r.add(dt);for(let B of[-1,1]){let W=[],ee=[];for(let Q=0;Q<=512;Q++){st.getPointAt(Q/512,G),st.getTangentAt(Q/512,X);for(let Ae of[-.045,.045]){let Ve=B*.85+Ae;W.push(G.x-X.z*Ve,.21,G.z+X.x*Ve)}if(Q<512){let Ae=Q*2;ee.push(Ae,Ae+1,Ae+2,Ae+1,Ae+3,Ae+2)}}let Y=new bt;Y.setAttribute("position",new ut(W,3)),Y.setIndex(ee),Y.computeVertexNormals(),i.add(Y),t.add(new it(Y,dt))}async function ct(B,W=x==="low"?2:x==="medium"?1:0){let ee=B+":"+W;return n.has(ee)||n.set(ee,(async()=>{let ae=C.assets.find(Ze=>Ze.id===B),Y=ae?.lods.find(Ze=>Ze.level===W);if(!Y||!/^[\w-]+\.lod[0-2]\.glb$/.test(Y.file))throw Error("Invalid world asset");let Q=await fetch(e.assetURL(Y.file),{signal:S.signal});if(!Q.ok)throw Error("World asset unavailable");let Ae=await Q.arrayBuffer();if(P)throw Error("Disposed");w+=Ae.byteLength;let Ve=await new Ti().parseAsync(Ae,""),rt=Ve.animations.length>0||/^(tram|service-cart|robot-|door|lift|sky-lift|telescope)/.test(B);if(Ve.scene.traverse(Ze=>{if(Ze.isMesh){i.add(Ze.geometry);for(let Ht of Array.isArray(Ze.material)?Ze.material:[Ze.material])r.add(Ht);Ze.castShadow=!0,Ze.receiveShadow=!0,e.surfaces?.prepare(Ze,rt)}}),P)throw i.forEach(Ze=>Ze.dispose()),r.forEach(Ze=>Ze.dispose()),Error("Disposed");return g.add(B),Ve})()),n.get(ee)}async function O(B,W,ee,ae=0,Y=0,Q=t,Ae=[1,1,1],Ve){let rt=await ct(B,Ve);if(P)return null;let Ze=rt.scene.clone(!0);Ze.position.set(W,ae,ee),Ze.rotation.y=Y,Ze.scale.set(...Ae),Q.add(Ze),Ze.traverse(Gt=>{Gt.isMesh&&(Gt.userData.worldAsset=B,Gt.userData.worldPart=Gt.name,_.push(Gt))});let Ht=C.assets.find(Gt=>Gt.id===B),Et="furnishing:"+qe++;for(let[Gt,mn]of(Ht.navigation.colliders||[]).entries())re.solid(Et+":"+Gt,{owner:Et,x:W,z:ee,heading:Y,min:[mn[0]*Ae[0],ae+mn[1]*Ae[1],mn[2]*Ae[2]],max:[mn[3]*Ae[0],ae+mn[4]*Ae[1],mn[5]*Ae[2]]});let lt=rt.animations.length?new La(Ze):null;lt&&a.push(lt);let an=null,Rt=null,Yt=/^robot-/.test(B);return{node:Ze,mixer:lt,owner:Et,bounds:Ht.motion_bounds||Ht.lods[0].bounds,clips:rt.animations,play(Gt,mn=!1){if(!lt)return;let ji=rt.animations.find(Yr=>Yr.name===Gt);if(!ji)return;let yn=lt.clipAction(ji);if(an===yn)return yn;let Xs=Rt;return yn.reset(),mn&&(yn.setLoop(sc,1),yn.clampWhenFinished=!0),yn.play(),an&&yn.crossFadeFrom(an,.25,!1),an=yn,Rt=Gt,Yt&&Xs&&Gt!==Xs&&Gt!=="walk"&&e.onSound?.("robot_"+Gt,Ze.position.x,Ze.position.y,Ze.position.z,Et),yn}}}function A(B){B.updateMatrixWorld(!0);let W=new Map;B.traverse(ae=>{if(!ae.isMesh)return;let Y=ae.geometry.uuid+":"+(Array.isArray(ae.material)?ae.material.map(Q=>Q.uuid).join(","):ae.material.uuid);W.has(Y)||W.set(Y,{n:ae,matrices:[]}),W.get(Y).matrices.push(ae.matrixWorld.clone())});let ee=new gt;t.add(ee);for(let{n:ae,matrices:Y}of W.values()){let Q=new An(ae.geometry,ae.material,Y.length);Y.forEach((Ae,Ve)=>Q.setMatrixAt(Ve,Ae)),Q.castShadow=!0,Q.receiveShadow=!0,Q.userData={...ae.userData},_.push(Q),ee.add(Q)}return B.traverse(ae=>{let Y=_.indexOf(ae);Y>=0&&_.splice(Y,1)}),B.removeFromParent(),d.set(ee.uuid,ee),ee}function ne(B){if(p.has(B.id))return p.get(B.id);let W=(async()=>{let ee=new gt;t.add(ee);let ae=[];for(let Y of Np())ae.push(O("floor",B.x+Y.x,B.z+Y.z,xt.room,0,ee,[Y.sx,1,Y.sz]));for(let Y=-4;Y<=4;Y+=4)for(let Q=-4;Q<=4;Q+=4)ae.push(O("ceiling",B.x+Y,B.z+Q,8.15+xt.room,0,ee));for(let Y=-4;Y<=4;Y+=4)ae.push(O("wall",B.x+Y,B.z+6,xt.room,0,ee));for(let Y=-4;Y<=4;Y+=4)for(let Q of[-1,1])for(let Ae of[0,4])ae.push(O("window",B.x+Q*6,B.z+Y,xt.room+Ae,Math.PI/2,ee));for(let Y=-4;Y<=4;Y+=4)for(let Q of[-1,1])ae.push(O("window",B.x+Y,B.z+Q*6,xt.gallery,0,ee));for(let Y of[-4,4])ae.push(O("wall",B.x+Y,B.doorZ,xt.room,0,ee));for(let[Y,Q]of[[-4,1],[.2,1.1]])ae.push(O("floor",B.x+Y,B.z+3,xt.gallery,0,ee,[Q,1,1.5])),ae.push(O("railing",B.x+Y,B.z,xt.gallery,0,ee,[Q,1,1]));for(let Y of[.7,5.3])ae.push(O("railing",B.x+2.4,B.z+Y,xt.gallery,Math.PI/2,ee,[.35,1,1]));for(let Y of[-5.6,1.9])ae.push(O("wall",B.x+Y,B.z+.15,xt.room,0,ee,[.05,1,.6]));if(await Promise.all(ae),!P)return A(ee)})();return p.set(B.id,W),W}async function de(B){if(!(c.has(B.id)||P)){c.set(B.id,{ready:!1,lift:null,liftValue:0,liftTarget:0}),z++;try{await ne(B);let W=new gt;t.add(W);let ee=[];for(let Y of[-3,0])ee.push(O(B.id==="missions"?"cargo":B.id==="memory"?"archive-shelf":"console",B.x+Y,B.z+3,xt.room,Math.PI,W));if(ee.push(O("bench",B.x-4,B.z-1,xt.room,Math.PI/2,W)),ee.push(O("bench",B.x-4,B.z+4.8,xt.gallery,0,W)),ee.push(O("console",B.x,B.z+4.8,xt.gallery,Math.PI,W)),await Promise.all(ee),P)return;A(W);let ae=c.get(B.id);ae.hologram=await O("hologram",B.x-1,B.z,xt.room),ae.hologram?.play("operate"),B.id==="memory"&&Oe(),ae.lift=await O("lift",B.liftX,B.liftZ,xt.room),ae.liftAction=ae.lift?.play("operate",!0),ae.liftAction&&(ae.liftAction.paused=!0),await V(B.id,B.x-2.2,B.z+.9,xt.gallery,Math.PI,{skip:null,limit:.9,pitch:[-.14,.18],eye:0}),ae.ready=!0}catch(W){P||e.onError?.(W)}finally{z--}}}async function he(){try{let B=await fetch(e.assetURL("manifest.json"),{signal:S.signal});if(!B.ok)throw Error("World manifest unavailable");C=await B.json();let W=new gt;t.add(W);let ee=[];for(let Q of Zn)ee.push(O("station",Q.platformX,Q.platformZ,xt.pavement,Q.angle,W));for(let Q of _n){ee.push(ne(Q));let Ae=await O("door",Q.x,Q.doorZ,xt.room,Math.PI);if(P)return;let Ve=Ae.play("open",!0);Ve.paused=!0,h.set(Q.id,{...Ae,action:Ve,value:0,open:!1})}for(let Q=-76;Q<=76;Q+=8)ee.push(O("quay",Q,77,xt.quay,0,W));for(let Q of[-68,-52,-28,28,52,68])ee.push(O("quay-lantern",Q,75.5,xt.quay,0,W));for(let[Q,Ae]of[[-79,68],[-79,-67],[55,2],[8,40],[55,-24]])ee.push(O("garden",Q,Ae,0,0,W)),ee.push(O("bench",Q+3,Ae,0,Math.PI/2,W));for(let[Q,Ae]of[[-30,23],[30,23],[-28,72]])ee.push(O("arcade",Q,Ae,0,0,W));ee.push(O("bridge",-78.5,40,2,0,W,[1.75,1,1]),O("ramp",-80,49,0,Math.PI,W),O("stairs",-76.8,49,0,Math.PI,W),O("ramp",-78.5,31,0,0,W)),ee.push(O("pad",Ri.x,Ri.z,0,0,W)),ee.push(O("sky-deck",Nt.x,Nt.z,Nt.floor,0,W));for(let Q of Nt.benches)ee.push(O("bench",Q.x,Q.z,Nt.floor,Q.angle,W));for(let[Q,Ae]of[[-60,-56],[-60,-46],[51,49]])ee.push(O("charger",Q,Ae,0,0,W));if(await Promise.all(ee),P||(A(W),oe.lift=await O("sky-lift",Nt.lift.x,Nt.lift.z,Nt.lift.base),P))return;oe.cab=oe.lift?.node.getObjectByName("cab")||null,oe.ready=!!oe.cab;for(let Q of Nt.telescopes)await V(Q.id,Q.x,Q.z,Nt.floor,Q.angle,{skip:"agent",limit:1.45,pitch:[-1.1,.3],eye:1.25});for(let[Q,Ae]of[[-57,-61],[-57,-52]])(await O("cooler",Q,Ae))?.play("operate");for(let Q of bs){let Ae=await O(Q.id,Q.x,Q.z,gs(Q.x,Q.z),0,t,[1,1,1],2);if(P)return;Ae&&(Ae.level=2,be.set(Q.id,Ae),Pe.attach(Q.id,Ae),Ee.addPlace(Q,C.assets.find(Ve=>Ve.id===Q.id)),Ae.play("operate"))}for(let Q of Op){let Ae=await O(Q.id,Q.x,Q.z,Q.y,Q.angle||0,t);if(!Ae){if(P)return;continue}Ae.play(Q.clip),u.push({id:Q.id,model:Ae})}let ae=C.assets.find(Q=>Q.id==="hover-pack");if(ae)try{let Q=ae.navigation.interaction?.[0]?.position||[0,0,1.2];for(let Ae of Dp){let Ve=await O("hover-pack",Ae.x,Ae.z,ie(Ae.x,Ae.z),Ae.angle);if(P)return;if(!Ve)continue;Ve.play("idle");let rt=Math.sin(Ae.angle),Ze=Math.cos(Ae.angle);le.push({...Ae,model:Ve,pack:Ve.node.getObjectByName("pack")||null,docked:!0,ix:Ae.x+Q[0]*Ze+Q[2]*rt,iz:Ae.z-Q[0]*rt+Q[2]*Ze})}}catch(Q){P||e.onError?.(Q)}let Y=await O("service-cart",55,49,xt.ground,Math.PI/2);Y&&Y.play("open",!0);for(let Q=0;Q<19;Q++){let Ae=["courier","technician","archivist"][Q%3],Ve=await O("robot-"+Ae,0,0);if(!Ve)return;Ve.node.scale.setScalar(1.15),o.push(Ve),Ee.add(Ve,Q,Ve.bounds),Ve.play("idle")}for(let Q=0;Q<2;Q++){let Ae=await O("tram",0,0);if(!Ae)return;let Ve=Ae.play("open",!0);Ve.paused=!0;let rt=ot[Q?3:0].u;st.getPointAt(rt,G),st.getTangentAt(rt,X);let Ze=re.register("tram-"+Q,qi(Ae.bounds),{x:G.x,y:xt.road+.03,z:G.z,heading:Math.atan2(X.x,X.z)},3);l.push({...Ae,body:Ze,u:rt,speed:0,dwell:5,stop:Q?3:0,door:Ve})}for(let Q=0;Q<3;Q++){let Ae=await O("cargo",43,70);Ae&&(re.removeOwner(Ae.owner),Ae.node.visible=!1,f.push({...Ae,body:null,elapsed:9,direction:1}))}e.onReady?.()}catch(B){P||e.onError?.(B)}}he();async function Ue(B,W,ee){if(!(W.level===ee||W.requested===ee)){W.requested=ee;try{let ae=await ct(B,ee);if(P||W.requested!==ee)return;let Y=new Map;ae.scene.traverse(Q=>{Q.isMesh&&Y.set(Q.name,Q)}),W.node.traverse(Q=>{let Ae=Y.get(Q.name);Q.isMesh&&Ae&&(Q.geometry=Ae.geometry,Q.material=Ae.material)}),W.level=ee,W.requested=null,e.onReady?.()}catch(ae){W.requested=null,P||e.onError?.(ae)}}}function Oe(){let B=c.get("memory");if(!B?.hologram||!B.text&&!b.length)return;if(!B.text){let Q=document.createElement("canvas");Q.width=1024,Q.height=512;let Ae=new Is(Q);Ae.colorSpace=Xt,m.add(Ae);let Ve=new pn(4.8,2.4),rt=new Lt({map:Ae,transparent:!0,depthWrite:!1,side:Ot,toneMapped:!1});i.add(Ve),r.add(rt);let Ze=new it(Ve,rt),Ht=_n.find(Et=>Et.id==="memory");Ze.position.set(Ht.x-1,3.6+xt.room,Ht.z+.3),Ze.rotation.y=Math.PI,t.add(Ze),B.text={canvas:Q,texture:Ae,mesh:Ze}}let{canvas:W,texture:ee,mesh:ae}=B.text,Y=W.getContext("2d");Y.clearRect(0,0,1024,512),ae.visible=b.length>0,Y.fillStyle="rgba(5,24,32,.87)",Y.fillRect(0,0,1024,512),Y.fillStyle="#b9f4ef",Y.font="28px sans-serif",b.slice(0,4).forEach((Q,Ae)=>{let Ve=Array.from(Q).slice(0,96);Y.fillText(Ve.slice(0,48).join(""),30,55+Ae*118),Y.fillText(Ve.slice(48).join(""),30,94+Ae*118)}),ee.needsUpdate=!0}async function V(B,W,ee,ae,Y,Q){let Ae=await O("telescope",W,ee,ae,Y);!Ae||P||ke.push({id:B,model:Ae,tube:Ae.node.getObjectByName("tube"),x:W,z:ee,floor:ae,angle:Y,...Q,ex:W+Math.sin(Y)*Q.eye,ez:ee+Math.cos(Y)*Q.eye})}function j(B){let W=ke.find(Ve=>Ve.id===B);if(!W)return;let ee=We(W.angle+Math.PI),ae=W.floor+2.05,Y=ee,Q=-.08,Ae=1/0;for(let Ve of e.districts||[]){if(Ve.id===W.skip)continue;let rt=Ve.x-W.ex,Ze=Ve.z-W.ez,Ht=Math.hypot(rt,Ze),Et=Math.atan2(-rt,-Ze),lt=Math.abs(We(Et-ee));Ht>12&&lt<W.limit&&lt<Ae&&(Ae=lt,Y=Et,Q=Tt.clamp(Math.atan2(Ve.height*.45-ae,Ht),...W.pitch))}y={kind:"scope",id:B,scope:W,yaw0:ee,fov:14,sent:"",target:null,back:{position:H.position.clone(),quaternion:H.quaternion.clone(),fov:H.fov}},W.model.node.visible=!1,H.position.set(W.ex,ae,W.ez),H.quaternion.setFromEuler(Fe.set(Q,Y,0,"YXZ")),e.reduced()&&(H.fov=y.fov,H.updateProjectionMatrix()),e.onRide?.("scope"),J("door",H.position)}function me(B,W,ee){let ae=ee.radius*.6,Y=B.x-ee.x,Q=B.z-ee.z,Ae=W.x*W.x+W.z*W.z;if(Ae<1e-9)return 1/0;let Ve=2*(Y*W.x+Q*W.z),rt=Ve*Ve-4*Ae*(Y*Y+Q*Q-ae*ae);if(rt<0)return 1/0;let Ze=(-Ve-Math.sqrt(rt))/(2*Ae);if(Ze<0&&(Ze=(-Ve+Math.sqrt(rt))/(2*Ae)),Ze<0)return 1/0;let Ht=B.y+W.y*Ze;return Ht>=0&&Ht<=ee.height?Ze:1/0}function _e(B){let W=y.scope;Fe.setFromQuaternion(H.quaternion,"YXZ"),Fe.y=y.yaw0+Tt.clamp(We(Fe.y-y.yaw0),-W.limit,W.limit),Fe.x=Tt.clamp(Fe.x,...W.pitch),Fe.z=0,H.quaternion.setFromEuler(Fe),H.position.set(W.ex,W.floor+2.05,W.ez);let ee=e.reduced()?y.fov:Tt.damp(H.fov,y.fov,9,Math.min(B,.1));Math.abs(ee-y.fov)<.02&&(ee=y.fov),ee!==H.fov&&(H.fov=ee,H.updateProjectionMatrix()),H.getWorldDirection(G);let ae=null,Y=1/0;for(let rt of e.districts||[]){if(rt.id===W.skip)continue;let Ze=me(H.position,G,rt);Ze>12&&Ze<Y&&(Y=Ze,ae=rt.id)}y.target=ae;let Q=y.back.fov/H.fov,Ae=(Math.atan2(G.x,-G.z)*180/Math.PI+360)%360,Ve=[ae,ae?Math.round(Y):0,Q.toFixed(1),Math.round(Ae)].join();Ve!==y.sent&&(y.sent=Ve,e.onScope?.({id:y.id,target:ae,distance:ae?Math.round(Y):null,zoom:+Q.toFixed(1),bearing:Math.round(Ae)%360}))}function pe(B,W,ee){oe.motion={from:oe.value,to:B,t:0,duration:e.reduced()?0:W},oe.idle=0,ee&&J("lift",oe.lift.node.position)}function xe(){let B=Nt.lift;H.position.set(B.x,B.base+B.cab+oe.value+2.4,B.z)}function Se(B){pe(B,15,!0),y={kind:"sky",to:B},xe(),H.quaternion.setFromEuler(Fe.set(-.42,Math.PI,0,"YXZ")),e.onRide?.("sky")}function we(B,W){let ee=Nt.lift;if(!oe.motion&&y?.kind!=="sky"&&W>0&&Math.hypot(H.position.x-ee.x,H.position.z-ee.z)>30&&(oe.idle+=W,oe.idle>28&&pe(oe.value>1?0:ee.travel,15,!1)),oe.motion){let ae=oe.motion;ae.t+=Math.min(B,.1);let Y=ae.duration?ae.t/ae.duration:1;oe.value=ae.from+(ae.to-ae.from)*fe(Y),Y>=1&&(oe.value=ae.to,oe.motion=null)}oe.cab.position.y=oe.value,y?.kind==="sky"&&(xe(),oe.motion||(y=null,e.onRide?.(null),J("lift",H.position)))}function Ie(B){let W=le.find(ee=>ee.id===B);!W?.docked||e.reduced()||(W.docked=!1,W.pack&&(W.pack.visible=!1),y={kind:"hover",id:B},T="",ce.copy(H.position),ve.start(H.position),e.onRide?.("hover"),J("hover",H.position),Me())}function q(B){let W=le.find(ee=>ee.id===y?.id);W&&(W.docked=!0,W.pack&&(W.pack.visible=!0)),y=null,T="",e.onRide?.(null),e.onFlight?.(null),B&&J("hover",H.position)}function Te(){let B=se();B&&re.relocate(B,H.position)&&H.position.set(B.x,B.y,B.z)}function Me(){let B=ve.state();if(B.done){H.position.set(B.x,B.y,B.z),Te(),q(!0);return}let W=[Math.round(B.altitude),Math.round(B.speed*3.6),B.landing].join();W!==T&&(T=W,e.onFlight?.({altitude:Math.round(B.altitude),speed:Math.round(B.speed*3.6),landing:B.landing}))}function ue(){if(y?.kind==="hover"){ve.state().landing?ve.cancel():ve.land(),Me();return}if(y){if(y.kind==="tram"&&l[y.index].dwell<=1){y.exitRequested=!0;return}Re();return}if(N){if(N.kind==="telescope"){j(N.id);return}if(N.kind==="skycall"){pe(H.position.y>40?Nt.lift.travel:0,7,!0);return}if(N.kind==="skyup"||N.kind==="skydown"){Se(N.kind==="skyup"?Nt.lift.travel:0);return}if(N.kind==="resident"){Ee.inspect(N.id);return}if(N.kind==="demonstrate"){Ee.demonstrate(N.id,e.reduced()),e.onSociety?.({id:N.id,role:"installation",state:e.reduced()?"idle":"work",source:"ambient"});return}if(N.kind==="door"){let B=h.get(N.id);if(B){B.open=!B.open,J("door",B.node.position);let W=_n.find(ee=>ee.id===N.id);de(W)}}if(N.kind==="discover"){R.add(N.id);try{localStorage.setItem("aurago.desktop.sysworld.discoveries",JSON.stringify([...R]))}catch{}e.onDiscover?.(N.id),J("discover",H.position)}if(N.kind==="tram"&&!e.reduced()&&(y={kind:"waiting",station:N.id},e.onRide?.("waiting")),N.kind==="terminal"&&e.onTerminal?.(N.id),N.kind==="hoverpack"){Ie(N.id);return}if(N.kind==="drone"&&!e.reduced()&&(y={kind:"drone",elapsed:0,origin:H.position.clone()},e.onRide?.("drone"),J("tram",H.position)),N.kind==="lift"){let B=c.get(N.id);if(B){let W=H.position.y>5?4:0;if(Math.abs(B.liftValue-W)>.02){B.liftTarget=W;return}B.liftTarget=W?0:4,y={kind:"lift",id:N.id},e.onRide?.("lift"),J("lift",H.position)}}}}function Re(){if(y){if(y.kind==="hover"){let B=le.find(W=>W.id===y.id);if(ve.land(!0)){let W=ve.state();H.position.set(W.x,W.y,W.z)}else B&&H.position.set(B.ix,ie(B.ix,B.iz)+2.4,B.iz);Te(),ve.reset(),q(!1);return}if(y.kind==="lift"){let B=c.get(y.id),W=_n.find(ee=>ee.id===y.id);B.liftTarget=B.liftValue<2?0:4,H.position.set(W.x+1.5,2.4+xt.room+B.liftTarget,W.liftZ)}if(y.kind==="tram"){let B=Zn.find(W=>W.id===y.station)||Zn[0];H.position.set(B.platformX,2.4+xt.pavement+.3,B.platformZ)}if(y.kind==="drone"&&H.position.copy(y.origin),y.kind==="sky"&&(oe.motion=null,oe.value=y.to,oe.cab.position.y=oe.value,xe()),y.kind==="scope"){let B=y.scope;Fe.setFromQuaternion(H.quaternion,"YXZ"),B.tube?.rotation.set(-Fe.x,We(Fe.y+Math.PI-B.angle),0,"YXZ");let W=Fe.y;B.model.node.visible=!0,H.position.copy(y.back.position),Fe.setFromQuaternion(y.back.quaternion,"YXZ"),Fe.y=W,H.quaternion.setFromEuler(Fe),H.fov=y.back.fov,H.updateProjectionMatrix(),e.onScope?.(null)}y=null,e.onRide?.(null)}}function ye(){if(y?.kind==="hover")return ve.state().landing?[]:[{kind:"land",id:y.id,distance:0}];if(y)return[{kind:y.kind==="scope"?"unscope":"exit",id:y.kind,distance:0}];H.getWorldDirection(G);let B=Ee.nearby().filter(ee=>(ee.x-H.position.x)*G.x+(ee.z-H.position.z)*G.z>ee.distance*.3),W=(ee,ae,Y,Q,Ae)=>{let Ve=Math.hypot(H.position.x-Y,H.position.z-Q);Ve<Ae&&B.push({kind:ee,id:ae,distance:Ve})};for(let ee of _n)H.position.y<5&&W("door",ee.id,ee.x,ee.doorZ,3),c.get(ee.id)?.ready&&(W("lift",ee.id,ee.liftX,ee.liftZ,2.3),H.position.y<5&&W("terminal",ee.id,ee.x-2,ee.z+3,2.5));if(H.position.y<5)for(let ee of Zn)W("tram",ee.id,ee.platformX,ee.platformZ,4),R.has(ee.id)||W("discover",ee.id,ee.platformX+(ee.angle===0?5:0),ee.platformZ+(ee.angle===0?0:5),3);if(W("drone","drone",Ri.x,Ri.z,5),H.position.y<5&&!e.reduced())for(let ee of le)ee.docked&&W("hoverpack",ee.id,ee.ix,ee.iz,2.4);if(oe.ready){let ee=Nt.lift,ae=H.position.y>40,Y=Math.hypot(H.position.x-ee.x,H.position.z-ee.z),Q=ae?ee.travel:0;Y<(ae?3.6:2.4)&&(ae||H.position.y<10)&&B.push({kind:oe.motion||Math.abs(oe.value-Q)>.05?"skycall":ae?"skydown":"skyup",id:"sky",distance:Y})}for(let ee of ke)Math.abs(H.position.y-(ee.floor+2.4))<1.5&&W("telescope",ee.id,ee.x,ee.z,1.7);return B.sort((ee,ae)=>ee.distance-ae.distance)}function je(B,W,ee){if(P||!C)return;let ae=W?Math.min(B,.05):0;if(I+=ae,U=(y?.kind==="hover"&&H.position.y>8.3?null:ms(H.position.x,H.position.z)?.id)||null,Ye+=B,Ye>.5){Ye=0;for(let[Y,Q]of be){let Ae=H.position.distanceTo(Q.node.position),Ve=x==="low"||Ae>55?2:x==="medium"||Ae>32?1:0;Ue(Y,Q,Ve)}}e.traffic||re.begin();for(let Y of _n)Math.hypot(H.position.x-Y.x,H.position.z-Y.z)<32&&de(Y);a.forEach(Y=>{Y.getRoot().visible&&Y.update(ae)});for(let Y of h.values()){let Q=Tt.damp(Y.value,Y.open?1:0,5,Math.min(B,.1));Math.abs(Q-(Y.open?1:0))<.001&&(Q=Y.open?1:0);let Ae=Ve=>[-1,1].map(rt=>({owner:Y.owner,x:Y.node.position.x,z:Y.node.position.z,min:[rt*.8+rt*1.6*Ve-.8,xt.room,-.13],max:[rt*.8+rt*1.6*Ve+.8,xt.room+3.6,.13]}));(Y.open||Ae(Q).every(Ve=>re.solidClear(Ve)))&&(Y.value=Q),Ae(Y.value).forEach((Ve,rt)=>re.solid(Y.owner+":panel:"+rt,Ve)),Y.action.time=Y.value*Y.action.getClip().duration,Y.mixer.update(0)}oe.ready&&we(B,ae),y?.kind==="scope"&&_e(B);for(let[Y,Q]of c)if(Q.liftAction&&(Q.liftValue=Tt.damp(Q.liftValue,Q.liftTarget,1.8,Math.min(B,.1)),Math.abs(Q.liftValue-Q.liftTarget)<.02&&(Q.liftValue=Q.liftTarget),Q.liftAction.time=Q.liftValue/4*Q.liftAction.getClip().duration,Q.lift.mixer.update(0),y?.kind==="lift"&&y.id===Y)){let Ae=_n.find(Ve=>Ve.id===Y);H.position.set(Ae.liftX,2.4+xt.room+Q.liftValue,Ae.liftZ),Q.liftValue===Q.liftTarget&&(y=null,e.onRide?.(null))}if(Ee.update(ae,W),Pe.update(ae,W),l.forEach((Y,Q)=>{if(Y.node.visible=!!Y.body&&Q<(x==="low"?1:2),Y.body&&(Y.node.visible&&!Y.body.enabled&&!re.clear({...Y.body,enabled:!0},Y.body,Y.body)&&(Y.node.visible=!1),Y.body.enabled=Y.node.visible),!Y.node.visible)return;let Ae={u:Y.u,dwell:Y.dwell,stop:Y.stop},Ve=!1;if(Y.dwell>0)Y.dwell-=ae,Y.speed=0;else{let rt=Rt=>(Rt.u-Y.u+1)%1*at,Ze=ot.reduce((Rt,Yt)=>rt(Yt)>1e-5&&rt(Yt)<rt(Rt)?Yt:Rt,ot.find(Rt=>rt(Rt)>1e-5)||ot[0]),Ht=Math.min(11,Math.sqrt(rt(Ze)*8));for(let Rt=1;Rt<=3;Rt++){let Yt=(Y.u+(1.2+Y.speed*.7)*Rt/3/at)%1;st.getPointAt(Yt,G),st.getTangentAt(Yt,X);let Gt={x:G.x,y:xt.road+.03,z:G.z,heading:Math.atan2(X.x,X.z)};if(!re.clear(Y.body,Gt,Gt)){Ht=0;break}}Y.speed+=Tt.clamp(Ht-Y.speed,-ae*8,ae*3.5);let Et=ae*Y.speed/at,lt=Y.u;Y.u=(Y.u+Et)%1;let an=ot.findIndex(Rt=>(Rt.u-lt+1)%1>0&&(Rt.u-lt+1)%1<=Et+1e-5);an>=0&&(Y.stop=an,Y.u=ot[an].u,Y.dwell=6,Y.speed=0,Ve=!0)}if(st.getPointAt(Y.u,Y.node.position),Y.node.position.y=xt.road+.03,st.getTangentAt(Y.u,X),Y.node.rotation.y=Math.atan2(X.x,X.z),Y.body&&(Y.body.enabled=Y.node.visible,re.propose(Y.body,{...Y.node.position,heading:Y.node.rotation.y},(rt,Ze)=>{rt||(Object.assign(Y,Ae),Y.speed=0),Y.node.position.set(Ze.x,Ze.y,Ze.z),Y.node.rotation.y=Ze.heading,rt&&Ve&&J("tram",Y.node.position)})),Y.door.time=(Y.dwell>1?1:0)*Y.door.getClip().duration,Y.mixer.update(0),y?.kind==="waiting"&&ot[Y.stop].id===y.station&&Y.dwell>1&&(y={kind:"tram",index:Q,station:y.station,offset:0},e.onRide?.("tram")),y?.kind==="tram"&&y.index===Q){if(Y.dwell>1&&(y.station=ot[Y.stop].id,y.exitRequested)){Re();return}H.position.copy(Y.node.position).addScaledVector(X,y.offset),H.position.y+=.9+1.4,H.lookAt(Y.node.position.x+X.x*16,H.position.y,Y.node.position.z+X.z*16)}}),y?.kind==="drone"){y.elapsed+=Math.min(B,.1);let Y=y.elapsed/40*Math.PI*2;H.position.set(Math.cos(Y)*100,45+Math.sin(Y*2)*12,Math.sin(Y)*95-10),H.lookAt(0,22,-12),y.elapsed>=40&&Re()}if(N=ee==="street"&&ye()[0]||null,ee==="street"&&!y){let Y=H.position.distanceTo(Z);Y<3&&(D+=Y),D>1.8&&(J(U?"step_inside":"step",H.position),D=0)}Z.copy(H.position),e.onEnvironment?.(!!U);for(let[Y,Q]of f.entries()){if(Q.elapsed+=ae,Q.node.visible=W&&Q.elapsed<8,!Q.node.visible){Q.body&&re.remove(Q.body.id),Q.body=null;continue}let Ae=Q.direction>0?Q.elapsed/8:1-Q.elapsed/8,Ve={x:40+Ae*3,y:xt.room,z:70,heading:0};if(Q.body||(Q.body=re.register("freight-"+Y,qi(Q.bounds),Ve,0)),!Q.body){Q.elapsed=9,Q.node.visible=!1;continue}re.propose(Q.body,Ve,(rt,Ze)=>{rt||(Q.elapsed-=ae),Q.node.position.set(Ze.x,Ze.y,Ze.z)})}e.traffic||re.solve(ae),e.onInteraction?.(N,R.size,U)}async function Ke(B){if(x=B,!C)return;let W=B==="low"?2:B==="medium"?1:0;Ee.setTier(B),Pe.setTier(B);try{let ee=new Map(await Promise.all([...g].filter(ae=>!be.has(ae)).map(async ae=>{let Y=await ct(ae,W),Q=new Map;return Y.scene.traverse(Ae=>{Ae.isMesh&&Q.set(Ae.name,Ae)}),[ae,Q]})));if(P||x!==B)return;for(let ae of _){let Y=ee.get(ae.userData.worldAsset)?.get(ae.userData.worldPart);Y&&(ae.geometry=Y.geometry,ae.material=Y.material)}e.onReady?.()}catch(ee){P||e.onError?.(ee)}}return{update:je,interact:ue,endRide:Re,setTier:Ke,society:Ee,suspend(){Ee.suspend(),Pe.clear(),E=!1;for(let B of f)B.elapsed=9,B.node.visible=!1,B.body&&re.remove(B.body.id),B.body=null},syncRide(){if(y?.kind==="tram"){let B=l[y.index];st.getTangentAt(B.u,X),H.position.copy(B.node.position).addScaledVector(X,y.offset),H.position.y+=2.3,H.lookAt(B.node.position.x+X.x*16,H.position.y,B.node.position.z+X.z*16)}},socialAction(B,W){let ee=Zn.find(ae=>ae.id===W);return Ee.action(e.reduced()&&B==="guide"?"cancel":B,ee?{x:ee.platformX,z:ee.platformZ}:null)},setMemory(B){b=(Array.isArray(B)?B:[]).filter(W=>typeof W=="string").slice(0,8),Oe()},setWorld(B,W=!1){W?(Ee.setReplay(!0),Pe.clear()):Ee.setReplay(!1);let ee=B?.entities?.filter(Y=>Y.kind==="mission")||[],ae=new Set;for(let Y of ee){ae.add(Y.id);let Q=v.get(Y.id);if(!W&&e.active()&&E&&Q!=null&&Q!==Y.state&&["running","completed","failed","cancelled"].includes(Y.state)&&Date.now()-Y.at<3e4){let Ae=f.find(Ve=>Ve.elapsed>=8);Ae&&(Ae.elapsed=0,Ae.direction=Y.state==="running"?1:-1,M++)}v.set(Y.id,Y.state)}for(let Y of v.keys())ae.has(Y)||v.delete(Y);if(E=!W,W)for(let Y of f)Y.elapsed=9,Y.node.visible=!1},walkRide(B,W,ee=0){if(!y)return!1;if(y.kind==="tram"&&(y.offset=Tt.clamp(y.offset+B*W*3,-2.5,2.5)),y.kind==="scope"){Fe.setFromQuaternion(H.quaternion,"YXZ");let ae=W*.9*H.fov/y.back.fov;Fe.x+=B*ae,Fe.y-=ee*ae,H.quaternion.setFromEuler(Fe)}return!0},hovering:()=>y?.kind==="hover",fly(B,W){if(y?.kind!=="hover")return!1;H.position.distanceToSquared(ce)>.01&&ve.place(H.position),H.getWorldDirection(G);let ee=ve.step(B,W,Math.atan2(-G.x,-G.z));return ce.set(ee.x,ee.y,ee.z),H.position.copy(ce),Me(),!0},zoom(B){return y?.kind!=="scope"?!1:(y.fov=Tt.clamp(y.fov*Math.exp(B*.0012),5,26),!0)},scoping:()=>y?.kind==="scope",move(B,W,ee){if(y)return!1;if(ee.y>40)return Ip(B,W,!oe.motion&&oe.value===Nt.lift.travel);let ae=ms(ee.x,ee.z);return ae&&ee.y>5?Math.abs(W-ae.z-4.8)<.85&&(Math.abs(B-ae.x)<1.1||Math.abs(B-ae.x+4)<1.75)?!1:Nu(ae,B,W,c.get(ae.id)?.liftValue===4):ae&&ee.y<5&&([-5.6,1.9].some(Y=>Math.abs(B-ae.x-Y)<.25&&Math.abs(W-ae.z-.15)<.25)||Fp(ae,B,W)&&!(B<ae.liftX+1.25&&Math.abs(W-ae.liftZ)<1.25&&c.get(ae.id)?.liftValue===0)||Math.hypot(B-(ae.x-1),W-ae.z)<1.55||[-3,0].some(Y=>Math.abs(B-ae.x-Y)<1.1&&Math.abs(W-ae.z-3)<.8))?!1:zu(B,W,ee,Y=>h.get(Y)?.value>.85&&!!c.get(Y)?.ready,e.districts)},floor(B,W){if(H.position.y>40)return Nt.floor;let ee=ms(B,W);return ee&&H.position.y>5&&Nu(ee,B,W,c.get(ee.id)?.liftValue===4)?xt.gallery:ie(B,W)},visit(B){Re(),y=null;let W=bs.find(ae=>ae.id===B);if(W){let ae=C?.assets.find(Y=>Y.id===B)?.navigation.interaction[0]?.position||[0,1.5,4];H.position.set(W.x,2.4+gs(W.x,W.z),W.z+ae[2]+1),H.lookAt(W.x,2,W.z);return}if(B==="drone"){H.position.set(Ri.x,2.7,Ri.z+3),H.lookAt(Ri.x,2,Ri.z);return}if(B==="skydeck"){let ae=57*Math.PI/180,Y=Nt.x+Math.sin(ae)*7.2,Q=Nt.z-Math.cos(ae)*7.2;H.position.set(Y,Nt.floor+2.4,Q),H.lookAt(Y,Nt.floor+.4,Q-10);return}let ee=Zn.find(ae=>ae.id===B);if(ee){let ae=R.has(B)?0:5,Y=ee.platformX+(ee.angle===0?ae:0),Q=ee.platformZ+(ee.angle===0?0:ae);H.position.set(Y,2.4+gs(Y,Q),Q),H.lookAt(ee.platformX,2,ee.platformZ===Q?ee.platformZ+1:ee.platformZ)}},destination(B){let W=_n.find(ee=>ee.id===B);if(W){Re(),y=null;let ee=W.doorZ+W.front*4;H.position.set(W.x,2.4+gs(W.x,ee),ee),H.lookAt(W.x,2.4+gs(W.x,ee),W.z)}},isRiding:()=>!!y,interaction:()=>N,rideBody:()=>y?.kind==="tram"?"tram-"+y.index:y?.kind==="scope"?y.scope.model.owner:y?.kind==="sky"&&oe.lift?.owner||null,stats:()=>({society:Ee.stats(),machinery:Pe.stats(),details:[...be].map(([B,W])=>({id:B,level:W.level})),loaded:[...g],bytes:w,freightEvents:M,residents:o.filter(B=>B.node.visible).length,trams:l.filter(B=>B.node.visible).length,rooms:[...c].filter(([,B])=>B.ready).map(([B])=>B),inside:U,ride:y?.kind||null,station:y?.station||null,discovered:[...R],loading:z,interactions:N?{kind:N.kind,id:N.id}:null,landmarks:u.map(B=>B.id),sky:{ready:oe.ready,value:+oe.value.toFixed(2),moving:!!oe.motion},telescopes:ke.length,hover:(B=>({pads:le.length,docked:le.filter(W=>W.docked).length,flying:!!B,altitude:B?+B.altitude.toFixed(1):0,speed:B?+(B.speed*3.6).toFixed(1):0,landing:!!B?.landing}))(y?.kind==="hover"?ve.state():null),scope:y?.kind==="scope"?{id:y.id,target:y.target,fov:+H.fov.toFixed(1)}:null}),dispose(){P||(P=!0,S.abort(),Ee.dispose(),Pe.dispose(),e.traffic||re.dispose(),b=[],v.clear(),t.removeFromParent(),a.forEach(B=>{B.stopAllAction(),B.uncacheRoot(B.getRoot())}),t.traverse(B=>{B.isInstancedMesh&&B.dispose()}),i.forEach(B=>B.dispose()),r.forEach(B=>B.dispose()),m.forEach(B=>B.dispose()),n.clear())}}}var Q_=s=>.5+(s-6)/12*3.62,Xp=new k;function jp(s,{sun:e,rim:t,hemisphere:n,atmosphere:i,surfaces:r,onThunder:a}){let o="local",l="clear",c="high",h=!1,u=-100,d=!1,p=n.intensity,g=14,_=-1,m=0,f={value:0},v=new bt,S=[],b=317,M=()=>(b=Math.imul(b,1664525)+1013904223>>>0)/4294967296;for(let D=0;D<1200;D++)S.push((M()-.5)*170,M()*60,(M()-.5)*174-7);v.setAttribute("position",new ut(S,3));let E=new vt({transparent:!0,depthWrite:!1,uniforms:{time:f,flash:{value:0}},vertexShader:"uniform float time;varying float vFade;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;gl_PointSize=clamp(260.0/max(1.0,-mv.z),2.0,26.0);vFade=smoothstep(0.0,4.0,p.y)*smoothstep(460.0,60.0,-mv.z);}",fragmentShader:"uniform float flash;varying float vFade;void main(){vec2 c=gl_PointCoord-.5;float a=(1.0-smoothstep(0.0,0.07,abs(c.x)))*(1.0-smoothstep(0.2,0.5,abs(c.y)))*.42*vFade;if(a<.004)discard;gl_FragColor=vec4(mix(vec3(.6,.78,.86),vec3(.95,.97,1.0),flash),a*(1.0+flash));}"}),P=new fn(v,E);P.frustumCulled=!1,s.add(P);let x=new Le(9417679),C=new Le(528926),w=new Le(2762296);function I(){let D=Rc(o),R=D.amount,U=Q_(D.hour);e.intensity=.8+R*2.6,e.color.set(D.evening>.1?16757370:13164287),e.position.set(Math.cos(D.hour/24*Math.PI*2)*120,40+R*115,85),Xp.set(Math.cos(U)*120,40-D.evening*18+R*115,Math.sin(U)*120),t.intensity=.7+D.evening*1.6,p=n.intensity=.45+R*.45,s.fog.color.copy(C).lerp(x,R).lerp(w,D.evening*(1-R)*.5),s.fog.density=l==="fog"?.018:l==="rain"?.008:.003-R*.0016,i.setLighting?.(R,D.evening,Xp),i.setWeather?.(l),r?.setLighting(R,D.evening,l==="clear"?0:1,s.fog.color),r?.setWeather(l),P.visible=l==="rain"&&!h,v.setDrawRange(0,c==="low"?200:1200)}function y(D,R){if(l!=="rain"||h||!R){_>=0&&(_=-1,N(0));return}if(g-=D,_<0&&g<=0&&(_=0,m++,g=9+M()*16,a?.(.6+M()*3.2)),_<0)return;_+=D;let U=_<.09?1:_<.16?.25:_<.24?.75:Math.max(0,1-(_-.24)/.35)*.5;N(U),_>.6&&(_=-1,N(0))}function N(D){n.intensity=p+D*2.4,E.uniforms.flash.value=D,i.setFlash?.(D)}return I(),{set(D={}){["local","day","evening","night"].includes(D.time)&&(o=D.time),["clear","rain","fog"].includes(D.weather)&&(l=D.weather),I()},setTier(D){c=D,I()},setIndoor(D){h!==D&&(h=D,I())},update(D,R,U){if(!d)return P.visible=l==="rain"&&!h&&U,U&&(f.value+=Math.min(D,.1)),y(Math.min(D,.1),U),R-u>30?(u=R,I(),!0):!1},mood(){let D=Rc(o);return{day:D.amount,evening:D.evening,weather:l,indoor:h}},stats:()=>({time:o,weather:l,indoor:h,daylight:Rc(o).amount,strikes:m}),dispose(){d=!0,P.removeFromParent(),v.dispose(),E.dispose()}}}var Bu=(function(){var s="b9H79Tebbbe8Fv9Gbb9Gvuuuuueu9Giuuub9Geueu9Giuuueuixkbeeeddddillviebeoweuecj:Gdkr;Neqo9TW9T9VV95dbH9F9F939H79T9F9J9H229F9Jt9VV7bb8A9TW79O9V9Wt9F9KW9J9V9KW9wWVtW949c919M9MWVbeY9TW79O9V9Wt9F9KW9J9V9KW69U9KW949c919M9MWVbdE9TW79O9V9Wt9F9KW9J9V9KW69U9KW949tWG91W9U9JWbiL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9p9JtblK9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9r919HtbvL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVT949WboY9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVJ9V29VVbrl79IV9Rbwq:VZkdbk:XYi5ud9:du8Jjjjjbcj;kb9Rgv8Kjjjjbc9:hodnalTmbcuhoaiRbbgrc;WeGc:Ge9hmbarcsGgwce0mbc9:hoalcufadcd4cbawEgDadfgrcKcaawEgqaraq0Egk6mbaicefhxcj;abad9Uc;WFbGcjdadca0EhmaialfgPar9Rgoadfhsavaoadz:jjjjbgzceVhHcbhOdndninaeaO9nmeaPax9RaD6mdamaeaO9RaOamfgoae6EgAcsfglc9WGhCabaOad2fhXaAcethQaxaDfhiaOaeaoaeao6E9RhLalcl4cifcd4hKazcj;cbfaAfhYcbh8AazcjdfhEaHh3incbh5dnawTmbaxa8Acd4fRbbh5kcbh8Eazcj;cbfhqinaih8Fdndndndna5a8Ecet4ciGgoc9:fPdebdkaPa8F9RaA6mrazcj;cbfa8EaA2fa8FaAz:jjjjb8Aa8FaAfhixdkazcj;cbfa8EaA2fcbaAz:kjjjb8Aa8FhixekaPa8F9RaK6mva8FaKfhidnaCTmbaPai9RcK6mbaocdtc:q:G:cjbfcj:G:cjbawEhaczhrcbhlinargoc9Wfghaqfhrdndndndndndnaaa8Fahco4fRbbalcoG4ciGcdtfydbPDbedvivvvlvkar9cb83bwar9cb83bbxlkarcbaiRbdai8Xbb9c:c:qj:bw9:9c:q;c1:I1e:d9c:b:c:e1z9:gg9cjjjjjz:dg8J9qE86bbaqaofgrcGfcbaicdfa8J9c8N1:NfghRbbag9cjjjjjw:dg8J9qE86bbarcVfcbaha8J9c8M1:NfghRbbag9cjjjjjl:dg8J9qE86bbarc7fcbaha8J9c8L1:NfghRbbag9cjjjjjd:dg8J9qE86bbarctfcbaha8J9c8K1:NfghRbbag9cjjjjje:dg8J9qE86bbarc91fcbaha8J9c8J1:NfghRbbag9cjjjj;ab:dg8J9qE86bbarc4fcbaha8J9cg1:NfghRbbag9cjjjja:dg8J9qE86bbarc93fcbaha8J9ch1:NfghRbbag9cjjjjz:dgg9qE86bbarc94fcbahag9ca1:NfghRbbai8Xbe9c:c:qj:bw9:9c:q;c1:I1e:d9c:b:c:e1z9:gg9cjjjjjz:dg8J9qE86bbarc95fcbaha8J9c8N1:NfgiRbbag9cjjjjjw:dg8J9qE86bbarc96fcbaia8J9c8M1:NfgiRbbag9cjjjjjl:dg8J9qE86bbarc97fcbaia8J9c8L1:NfgiRbbag9cjjjjjd:dg8J9qE86bbarc98fcbaia8J9c8K1:NfgiRbbag9cjjjjje:dg8J9qE86bbarc99fcbaia8J9c8J1:NfgiRbbag9cjjjj;ab:dg8J9qE86bbarc9:fcbaia8J9cg1:NfgiRbbag9cjjjja:dg8J9qE86bbarcufcbaia8J9ch1:NfgiRbbag9cjjjjz:dgg9qE86bbaiag9ca1:NfhixikaraiRblaiRbbghco4g8Ka8KciSg8KE86bbaqaofgrcGfaiclfa8Kfg8KRbbahcl4ciGg8La8LciSg8LE86bbarcVfa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc7fa8Ka8Lfg8KRbbahciGghahciSghE86bbarctfa8Kahfg8KRbbaiRbeghco4g8La8LciSg8LE86bbarc91fa8Ka8Lfg8KRbbahcl4ciGg8La8LciSg8LE86bbarc4fa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc93fa8Ka8Lfg8KRbbahciGghahciSghE86bbarc94fa8Kahfg8KRbbaiRbdghco4g8La8LciSg8LE86bbarc95fa8Ka8Lfg8KRbbahcl4ciGg8La8LciSg8LE86bbarc96fa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc97fa8Ka8Lfg8KRbbahciGghahciSghE86bbarc98fa8KahfghRbbaiRbigico4g8Ka8KciSg8KE86bbarc99faha8KfghRbbaicl4ciGg8Ka8KciSg8KE86bbarc9:faha8KfghRbbaicd4ciGg8Ka8KciSg8KE86bbarcufaha8KfgrRbbaiciGgiaiciSgiE86bbaraifhixdkaraiRbwaiRbbghcl4g8Ka8KcsSg8KE86bbaqaofgrcGfaicwfa8Kfg8KRbbahcsGghahcsSghE86bbarcVfa8KahfghRbbaiRbeg8Kcl4g8La8LcsSg8LE86bbarc7faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarctfaha8KfghRbbaiRbdg8Kcl4g8La8LcsSg8LE86bbarc91faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc4faha8KfghRbbaiRbig8Kcl4g8La8LcsSg8LE86bbarc93faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc94faha8KfghRbbaiRblg8Kcl4g8La8LcsSg8LE86bbarc95faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc96faha8KfghRbbaiRbvg8Kcl4g8La8LcsSg8LE86bbarc97faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc98faha8KfghRbbaiRbog8Kcl4g8La8LcsSg8LE86bbarc99faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc9:faha8KfghRbbaiRbrgicl4g8Ka8KcsSg8KE86bbarcufaha8KfgrRbbaicsGgiaicsSgiE86bbaraifhixekarai8Pbw83bwarai8Pbb83bbaiczfhikdnaoaC9pmbalcdfhlaoczfhraPai9RcL0mekkaoaC6moaimexokaCmva8FTmvkaqaAfhqa8Ecefg8Ecl9hmbkdndndndnawTmbasa8Acd4fRbbgociGPlbedrbkaATmdaza8Afh8Fazcj;cbfhhcbh8EaEhaina8FRbbhraahocbhlinaoahalfRbbgqce4cbaqceG9R7arfgr86bbaoadfhoaAalcefgl9hmbkaacefhaa8Fcefh8FahaAfhha8Ecefg8Ecl9hmbxikkaATmeaza8Afhaazcj;cbfhhcbhoceh8EaYh8FinaEaofhlaa8Vbbhrcbhoinala8FaofRbbcwtahaofRbbgqVc;:FiGce4cbaqceG9R7arfgr87bbaladfhlaLaocefgofmbka8FaQfh8FcdhoaacdfhaahaQfhha8EceGhlcbh8EalmbxdkkaATmbaocl4h8Eaza8AfRbbhqcwhoa3hlinalRbbaotaqVhqalcefhlaocwfgoca9hmbkcbhhaEh8FaYhainazcj;cbfahfRbbhrcwhoaahlinalRbbaotarVhralaAfhlaocwfgoca9hmbkara8E94aq7hqcbhoa8Fhlinalaqao486bbalcefhlaocwfgoca9hmbka8Fadfh8FaacefhaahcefghaA9hmbkkaEclfhEa3clfh3a8Aclfg8Aad6mbkaXazcjdfaAad2z:jjjjb8AazazcjdfaAcufad2fadz:jjjjb8AaAaOfhOaihxaimbkc9:hoxdkcbc99aPax9RakSEhoxekc9:hokavcj;kbf8Kjjjjbaok:ysezu8Jjjjjbc;ae9Rgv8Kjjjjbc9:hodnalaeci9UgrcHf6mbcuhoaiRbbgwc;WeGc;Ge9hmbawcsGgDce0mbavc;abfcFecjez:kjjjb8Aav9cu83iUav9cu83i8Wav9cu83iyav9cu83iaav9cu83iKav9cu83izav9cu83iwav9cu83ibaialfc9WfhqaicefgwarfhldnaeTmbcmcsaDceSEhkcbhxcbhmcbhrcbhicbhoindnalaq9nmbc9:hoxikdndnawRbbgDc;Ve0mbavc;abfaoaDcu7gPcl4fcsGcitfgsydlhzasydbhHdndnaDcsGgsak9pmbavaiaPfcsGcdtfydbaxasEhDaxasTgOfhxxekdndnascsSmbcehOasc987asamffcefhDxekalcefhDal8SbbgscFeGhPdndnascu9mmbaDhlxekalcvfhlaPcFbGhPcrhsdninaD8SbbgOcFbGastaPVhPaOcu9kmeaDcefhDascrfgsc8J9hmbxdkkaDcefhlkcehOaPce4cbaPceG9R7amfhDkaDhmkavc;abfaocitfgsaDBdbasazBdlavaicdtfaDBdbavc;abfaocefcsGcitfgsaHBdbasaDBdlaocdfhoaOaifhidnadcd9hmbabarcetfgsaH87ebasclfaD87ebascdfaz87ebxdkabarcdtfgsaHBdbascwfaDBdbasclfazBdbxekdnaDcpe0mbavaiaqaDcsGfRbbgscl4gP9RcsGcdtfydbaxcefgOaPEhDavaias9RcsGcdtfydbaOaPTgzfgOascsGgPEhsaPThPdndnadcd9hmbabarcetfgHax87ebaHclfas87ebaHcdfaD87ebxekabarcdtfgHaxBdbaHcwfasBdbaHclfaDBdbkavaicdtfaxBdbavc;abfaocitfgHaDBdbaHaxBdlavaicefgicsGcdtfaDBdbavc;abfaocefcsGcitfgHasBdbaHaDBdlavaiazfgicsGcdtfasBdbavc;abfaocdfcsGcitfgDaxBdbaDasBdlaocifhoaiaPfhiaOaPfhxxekaxcbalRbbgsEgHaDc;:eSgDfhOascsGhAdndnascl4gCmbaOcefhzxekaOhzavaiaC9RcsGcdtfydbhOkdndnaAmbazcefhxxekazhxavaias9RcsGcdtfydbhzkdndnaDTmbalcefhDxekalcdfhDal8SbegPcFeGhsdnaPcu9kmbalcofhHascFbGhscrhldninaD8SbbgPcFbGaltasVhsaPcu9kmeaDcefhDalcrfglc8J9hmbkaHhDxekaDcefhDkasce4cbasceG9R7amfgmhHkdndnaCcsSmbaDhsxekaDcefhsaD8SbbglcFeGhPdnalcu9kmbaDcvfhOaPcFbGhPcrhldninas8SbbgDcFbGaltaPVhPaDcu9kmeascefhsalcrfglc8J9hmbkaOhsxekascefhskaPce4cbaPceG9R7amfgmhOkdndnaAcsSmbashlxekascefhlas8SbbgDcFeGhPdnaDcu9kmbascvfhzaPcFbGhPcrhDdninal8SbbgscFbGaDtaPVhPascu9kmealcefhlaDcrfgDc8J9hmbkazhlxekalcefhlkaPce4cbaPceG9R7amfgmhzkdndnadcd9hmbabarcetfgDaH87ebaDclfaz87ebaDcdfaO87ebxekabarcdtfgDaHBdbaDcwfazBdbaDclfaOBdbkavc;abfaocitfgDaOBdbaDaHBdlavaicdtfaHBdbavc;abfaocefcsGcitfgDazBdbaDaOBdlavaicefgicsGcdtfaOBdbavc;abfaocdfcsGcitfgDaHBdbaDazBdlavaiaCTaCcsSVfgicsGcdtfazBdbaiaATaAcsSVfhiaocifhokawcefhwaocsGhoaicsGhiarcifgrae6mbkkcbc99alaqSEhokavc;aef8Kjjjjbaok:clevu8Jjjjjbcz9Rhvdnalaecvf9pmbc9:skdnaiRbbc;:eGc;qeSmbcuskav9cb83iwaicefhoaialfc98fhrdnaeTmbdnadcdSmbcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcdtfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgiBdbalaiBdbawcefgwae9hmbxdkkcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcetfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgi87ebalaiBdbawcefgwae9hmbkkcbc99aoarSEk:Lvoeue99dud99eud99dndnadcl9hmbaeTmeindndnabcdfgd8Sbb:Yab8Sbbgi:Ygl:l:tabcefgv8Sbbgo:Ygr:l:tgwJbb;:9cawawNJbbbbawawJbbbb9GgDEgq:mgkaqaicb9iEalMgwawNakaqaocb9iEarMgqaqNMM:r:vglNJbbbZJbbb:;aDEMgr:lJbbb9p9DTmbar:Ohixekcjjjj94hikadai86bbdndnaqalNJbbbZJbbb:;aqJbbbb9GEMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkavad86bbdndnawalNJbbbZJbbb:;awJbbbb9GEMgw:lJbbb9p9DTmbaw:Ohdxekcjjjj94hdkabad86bbabclfhbaecufgembxdkkaeTmbindndnabclfgd8Ueb:Yab8Uebgi:Ygl:l:tabcdfgv8Uebgo:Ygr:l:tgwJb;:FSawawNJbbbbawawJbbbb9GgDEgq:mgkaqaicb9iEalMgwawNakaqaocb9iEarMgqaqNMM:r:vglNJbbbZJbbb:;aDEMgr:lJbbb9p9DTmbar:Ohixekcjjjj94hikadai87ebdndnaqalNJbbbZJbbb:;aqJbbbb9GEMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkavad87ebdndnawalNJbbbZJbbb:;awJbbbb9GEMgw:lJbbb9p9DTmbaw:Ohdxekcjjjj94hdkabad87ebabcwfhbaecufgembkkk:4ioiue99dud99dud99dnaeTmbcbhiabhlindndnal8Uebgv:YgoJ:ji:1Salcof8UebgrciVgw:Y:vgDNJbbbZJbbb:;avcu9kEMgq:lJbbb9p9DTmbaq:Ohkxekcjjjj94hkkalclf8Uebhvalcdf8UebhxalarcefciGcetfak87ebdndnax:YgqaDNJbbbZJbbb:;axcu9kEMgm:lJbbb9p9DTmbam:Ohxxekcjjjj94hxkabaiarciGgkfcd7cetfax87ebdndnav:YgmaDNJbbbZJbbb:;avcu9kEMgP:lJbbb9p9DTmbaP:Ohvxekcjjjj94hvkalarcufciGcetfav87ebdndnawaw2:ZgPaPMaoaoN:taqaqN:tamamN:tgoJbbbbaoJbbbb9GE:raDNJbbbZMgD:lJbbb9p9DTmbaD:Ohrxekcjjjj94hrkalakcetfar87ebalcwfhlaiclfhiaecufgembkkk9mbdnadcd4ae2gdTmbinababydbgecwtcw91:Yaece91cjjj98Gcjjj;8if::NUdbabclfhbadcufgdmbkkk:Tvirud99eudndnadcl9hmbaeTmeindndnabRbbgiabcefgl8Sbbgvabcdfgo8Sbbgrf9R:YJbbuJabcifgwRbbgdce4adVgDcd4aDVgDcl4aDVgD:Z:vgqNJbbbZMgk:lJbbb9p9DTmbak:Ohxxekcjjjj94hxkaoax86bbdndnaraif:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohoxekcjjjj94hokalao86bbdndnavaifar9R:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohixekcjjjj94hikabai86bbdndnaDadcetGadceGV:ZaqNJbbbZMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkawad86bbabclfhbaecufgembxdkkaeTmbindndnab8Vebgiabcdfgl8Uebgvabclfgo8Uebgrf9R:YJbFu9habcofgw8Vebgdce4adVgDcd4aDVgDcl4aDVgDcw4aDVgD:Z:vgqNJbbbZMgk:lJbbb9p9DTmbak:Ohxxekcjjjj94hxkaoax87ebdndnaraif:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohoxekcjjjj94hokalao87ebdndnavaifar9R:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohixekcjjjj94hikabai87ebdndnaDadcetGadceGV:ZaqNJbbbZMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkawad87ebabcwfhbaecufgembkkk9teiucbcbyd:K:G:cjbgeabcifc98GfgbBd:K:G:cjbdndnabZbcztgd9nmbcuhiabad9RcFFifcz4nbcuSmekaehikaik;LeeeudndnaeabVciGTmbabhixekdndnadcz9pmbabhixekabhiinaiaeydbBdbaiclfaeclfydbBdbaicwfaecwfydbBdbaicxfaecxfydbBdbaeczfheaiczfhiadc9Wfgdcs0mbkkadcl6mbinaiaeydbBdbaeclfheaiclfhiadc98fgdci0mbkkdnadTmbinaiaeRbb86bbaicefhiaecefheadcufgdmbkkabk;aeedudndnabciGTmbabhixekaecFeGc:b:c:ew2hldndnadcz9pmbabhixekabhiinaialBdbaicxfalBdbaicwfalBdbaiclfalBdbaiczfhiadc9Wfgdcs0mbkkadcl6mbinaialBdbaiclfhiadc98fgdci0mbkkdnadTmbinaiae86bbaicefhiadcufgdmbkkabkk83dbcj:Gdk8Kbbbbdbbblbbbwbbbbbbbebbbdbbblbbbwbbbbc:K:Gdkl8W:qbb",e="b9H79TebbbeKl9Gbb9Gvuuuuueu9Giuuub9Geueuixkbbebeeddddilve9Weeeviebeoweuecj:Gdkr;Neqo9TW9T9VV95dbH9F9F939H79T9F9J9H229F9Jt9VV7bb8A9TW79O9V9Wt9F9KW9J9V9KW9wWVtW949c919M9MWVbdY9TW79O9V9Wt9F9KW9J9V9KW69U9KW949c919M9MWVblE9TW79O9V9Wt9F9KW9J9V9KW69U9KW949tWG91W9U9JWbvL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9p9JtboK9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9r919HtbrL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVT949WbwY9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVJ9V29VVbDl79IV9Rbqq:W9Dklbzik94evu8Jjjjjbcz9Rhbcbheincbhdcbhiinabcwfadfaicjuaead4ceGglE86bbaialfhiadcefgdcw9hmbkaeai86b:q:W:cjbaecitab8Piw83i:q:G:cjbaecefgecjd9hmbkk:JBl8Aud97dur978Jjjjjbcj;kb9Rgv8Kjjjjbc9:hodnalTmbcuhoaiRbbgrc;WeGc:Ge9hmbarcsGgwce0mbc9:hoalcufadcd4cbawEgDadfgrcKcaawEgqaraq0Egk6mbaialfgxar9RhodnadTgmmbavaoad;8qbbkaicefhPcj;abad9Uc;WFbGcjdadca0EhsdndndnadTmbaoadfhzcbhHinaeaH9nmdaxaP9RaD6miabaHad2fhOaPaDfhAasaeaH9RaHasfae6EgCcsfgocl4cifcd4hXavcj;cbfaoc9WGgQcetfhLavcj;cbfaQci2fhKavcj;cbfaQfhYcbh8Aaoc;ab6hEincbh3dnawTmbaPa8Acd4fRbbh3kcbh5avcj;cbfh8Eindndndndna3a5cet4ciGgoc9:fPdebdkaxaA9RaQ6mwdnaQTmbavcj;cbfa5aQ2faAaQ;8qbbkaAaCfhAxdkaQTmeavcj;cbfa5aQ2fcbaQ;8kbxekaxaA9RaX6moaoclVcbawEhraAaXfhocbhidnaEmbaxao9Rc;Gb6mbcbhlina8EalfhidndndndndndnaAalco4fRbbgqciGarfPDbedibledibkaipxbbbbbbbbbbbbbbbbpklbxlkaiaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaiaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaiaopbbbpklbaoczfhoxekaiaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqcd4ciGarfPDbedibledibkaiczfpxbbbbbbbbbbbbbbbbpklbxlkaiczfaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaiczfaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaiczfaopbbbpklbaoczfhoxekaiczfaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqcl4ciGarfPDbedibledibkaicafpxbbbbbbbbbbbbbbbbpklbxlkaicafaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaicafaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaicafaopbbbpklbaoczfhoxekaicafaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqco4arfPDbedibledibkaic8Wfpxbbbbbbbbbbbbbbbbpklbxlkaic8Wfaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Ngicitpbi:q:G:cjbaiRb:q:W:cjbgipsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Ngqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbaiaoclffaqRb:q:W:cjbfhoxikaic8Wfaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Ngicitpbi:q:G:cjbaiRb:q:W:cjbgipsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Ngqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbaiaocwffaqRb:q:W:cjbfhoxdkaic8Wfaopbbbpklbaoczfhoxekaic8WfaopbbdaoRbbgicitpbi:q:G:cjbaiRb:q:W:cjbgipsaoRbegqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbaiaocdffaqRb:q:W:cjbfhokalc;abfhialcjefaQ0meaihlaxao9Rc;Fb0mbkkdnaiaQ9pmbaici4hlinaxao9RcK6mwa8EaifhqdndndndndndnaAaico4fRbbalcoG4ciGarfPDbedibledibkaqpxbbbbbbbbbbbbbbbbpkbbxlkaqaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spkbbahaoclffagRb:q:W:cjbfhoxikaqaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spkbbahaocwffagRb:q:W:cjbfhoxdkaqaopbbbpkbbaoczfhoxekaqaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpkbbahaocdffagRb:q:W:cjbfhokalcdfhlaiczfgiaQ6mbkkaohAaoTmoka8EaQfh8Ea5cefg5cl9hmbkdndndndnawTmbaza8Acd4fRbbglciGPlbedwbkaQTmdavcjdfa8Afhlava8Afpbdbh8Jcbhoinalavcj;cbfaofpblbg8KaYaofpblbg8LpmbzeHdOiAlCvXoQrLg8MaLaofpblbg8NaKaofpblbgypmbzeHdOiAlCvXoQrLg8PpmbezHdiOAlvCXorQLg8Fcep9Ta8Fpxeeeeeeeeeeeeeeeegap9op9Hp9rg8Fa8Jp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ma8PpmwDKYqk8AExm35Ps8E8Fg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ka8LpmwKDYq8AkEx3m5P8Es8Fg8Ka8NaypmwKDYq8AkEx3m5P8Es8Fg8LpmbezHdiOAlvCXorQLg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ka8LpmwDKYqk8AExm35Ps8E8Fg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ugap9Abbbaladfglaaa8Fa8Fpmlvorlvorlvorlvorp9Ugap9Abbbaladfglaaa8Fa8FpmwDqkwDqkwDqkwDqkp9Ugap9Abbbaladfglaaa8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9AbbbaladfhlaoczfgoaQ6mbxikkaQTmeavcjdfa8Afhlava8Afpbdbh8Jcbhoinalavcj;cbfaofpblbg8KaYaofpblbg8LpmbzeHdOiAlCvXoQrLg8MaLaofpblbg8NaKaofpblbgypmbzeHdOiAlCvXoQrLg8PpmbezHdiOAlvCXorQLg8Fcep:nea8Fpxebebebebebebebebgap9op:bep9rg8Fa8Jp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ma8PpmwDKYqk8AExm35Ps8E8Fg8Fcep:nea8Faap9op:bep9rg8Fp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ka8LpmwKDYq8AkEx3m5P8Es8Fg8Ka8NaypmwKDYq8AkEx3m5P8Es8Fg8LpmbezHdiOAlvCXorQLg8Fcep:nea8Faap9op:bep9rg8Fp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ka8LpmwDKYqk8AExm35Ps8E8Fg8Fcep:nea8Faap9op:bep9rg8Fp:oegap9Abbbaladfglaaa8Fa8Fpmlvorlvorlvorlvorp:oegap9Abbbaladfglaaa8Fa8FpmwDqkwDqkwDqkwDqkp:oegap9Abbbaladfglaaa8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9AbbbaladfhlaoczfgoaQ6mbxdkkaQTmbcbhocbalcl4gl9Rc8FGhiavcjdfa8Afhrava8Afpbdbhainaravcj;cbfaofpblbg8JaYaofpblbg8KpmbzeHdOiAlCvXoQrLg8LaLaofpblbg8MaKaofpblbg8NpmbzeHdOiAlCvXoQrLgypmbezHdiOAlvCXorQLg8Faip:Rea8Falp:Tep9qg8Faap9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8LaypmwDKYqk8AExm35Ps8E8Fg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8Ja8KpmwKDYq8AkEx3m5P8Es8Fg8Ja8Ma8NpmwKDYq8AkEx3m5P8Es8Fg8KpmbezHdiOAlvCXorQLg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8Ja8KpmwDKYqk8AExm35Ps8E8Fg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9AbbbaradfhraoczfgoaQ6mbkka8Aclfg8Aad6mbkdnaCad2goTmbaOavcjdfao;8qbbkdnammbavavcjdfaCcufad2fad;8qbbkaCaHfhHc9:hoaAhPaAmbxlkkaeTmbaDalfhrcbhocuhlinaralaD9RglfaD6mdasaeao9Raoasfae6Eaofgoae6mbkaial9RhPkcbc99axaP9RakSEhoxekc9:hokavcj;kbf8Kjjjjbaokwbz:bjjjbkNsezu8Jjjjjbc;ae9Rgv8Kjjjjbc9:hodnalaeci9UgrcHf6mbcuhoaiRbbgwc;WeGc;Ge9hmbawcsGgDce0mbavc;abfcFecje;8kbav9cu83iUav9cu83i8Wav9cu83iyav9cu83iaav9cu83iKav9cu83izav9cu83iwav9cu83ibaialfc9WfhqaicefgwarfhldnaeTmbcmcsaDceSEhkcbhxcbhmcbhrcbhicbhoindnalaq9nmbc9:hoxikdndnawRbbgDc;Ve0mbavc;abfaoaDcu7gPcl4fcsGcitfgsydlhzasydbhHdndnaDcsGgsak9pmbavaiaPfcsGcdtfydbaxasEhDaxasTgOfhxxekdndnascsSmbcehOasc987asamffcefhDxekalcefhDal8SbbgscFeGhPdndnascu9mmbaDhlxekalcvfhlaPcFbGhPcrhsdninaD8SbbgOcFbGastaPVhPaOcu9kmeaDcefhDascrfgsc8J9hmbxdkkaDcefhlkcehOaPce4cbaPceG9R7amfhDkaDhmkavc;abfaocitfgsaDBdbasazBdlavaicdtfaDBdbavc;abfaocefcsGcitfgsaHBdbasaDBdlaocdfhoaOaifhidnadcd9hmbabarcetfgsaH87ebasclfaD87ebascdfaz87ebxdkabarcdtfgsaHBdbascwfaDBdbasclfazBdbxekdnaDcpe0mbavaiaqaDcsGfRbbgscl4gP9RcsGcdtfydbaxcefgOaPEhDavaias9RcsGcdtfydbaOaPTgzfgOascsGgPEhsaPThPdndnadcd9hmbabarcetfgHax87ebaHclfas87ebaHcdfaD87ebxekabarcdtfgHaxBdbaHcwfasBdbaHclfaDBdbkavaicdtfaxBdbavc;abfaocitfgHaDBdbaHaxBdlavaicefgicsGcdtfaDBdbavc;abfaocefcsGcitfgHasBdbaHaDBdlavaiazfgicsGcdtfasBdbavc;abfaocdfcsGcitfgDaxBdbaDasBdlaocifhoaiaPfhiaOaPfhxxekaxcbalRbbgsEgHaDc;:eSgDfhOascsGhAdndnascl4gCmbaOcefhzxekaOhzavaiaC9RcsGcdtfydbhOkdndnaAmbazcefhxxekazhxavaias9RcsGcdtfydbhzkdndnaDTmbalcefhDxekalcdfhDal8SbegPcFeGhsdnaPcu9kmbalcofhHascFbGhscrhldninaD8SbbgPcFbGaltasVhsaPcu9kmeaDcefhDalcrfglc8J9hmbkaHhDxekaDcefhDkasce4cbasceG9R7amfgmhHkdndnaCcsSmbaDhsxekaDcefhsaD8SbbglcFeGhPdnalcu9kmbaDcvfhOaPcFbGhPcrhldninas8SbbgDcFbGaltaPVhPaDcu9kmeascefhsalcrfglc8J9hmbkaOhsxekascefhskaPce4cbaPceG9R7amfgmhOkdndnaAcsSmbashlxekascefhlas8SbbgDcFeGhPdnaDcu9kmbascvfhzaPcFbGhPcrhDdninal8SbbgscFbGaDtaPVhPascu9kmealcefhlaDcrfgDc8J9hmbkazhlxekalcefhlkaPce4cbaPceG9R7amfgmhzkdndnadcd9hmbabarcetfgDaH87ebaDclfaz87ebaDcdfaO87ebxekabarcdtfgDaHBdbaDcwfazBdbaDclfaOBdbkavc;abfaocitfgDaOBdbaDaHBdlavaicdtfaHBdbavc;abfaocefcsGcitfgDazBdbaDaOBdlavaicefgicsGcdtfaOBdbavc;abfaocdfcsGcitfgDaHBdbaDazBdlavaiaCTaCcsSVfgicsGcdtfazBdbaiaATaAcsSVfhiaocifhokawcefhwaocsGhoaicsGhiarcifgrae6mbkkcbc99alaqSEhokavc;aef8Kjjjjbaok:clevu8Jjjjjbcz9Rhvdnalaecvf9pmbc9:skdnaiRbbc;:eGc;qeSmbcuskav9cb83iwaicefhoaialfc98fhrdnaeTmbdnadcdSmbcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcdtfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgiBdbalaiBdbawcefgwae9hmbxdkkcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcetfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgi87ebalaiBdbawcefgwae9hmbkkcbc99aoarSEk;Toio97eue97aec98Ghedndnadcl9hmbaeTmecbhdinababpbbbgicKp:RecKp:Sep;6eglaicwp:RecKp:Sep;6ealp;Geaiczp:RecKp:Sep;6egvp;Gep;Kep;Legopxbbbbbbbbbbbbbbbbp:2egralpxbbbjbbbjbbbjbbbjgwp9op9rp;Keglpxbb;:9cbb;:9cbb;:9cbb;:9calalp;Meaoaop;Meavaravawp9op9rp;Keglalp;Mep;Kep;Kep;Jep;Negvp;Mepxbbn0bbn0bbn0bbn0grp;KepxFbbbFbbbFbbbFbbbp9oaipxbbbFbbbFbbbFbbbFp9op9qalavp;Mearp;Kecwp:RepxbFbbbFbbbFbbbFbbp9op9qaoavp;Mearp;Keczp:RepxbbFbbbFbbbFbbbFbp9op9qpkbbabczfhbadclfgdae6mbxdkkaeTmbcbhdinabczfgDaDpbbbgipxbbbbbbFFbbbbbbFFgwp9oabpbbbgoaipmbediwDqkzHOAKY8AEgvczp:Reczp:Sep;6eglaoaipmlvorxmPsCXQL358E8FpxFubbFubbFubbFubbp9op;6eavczp:Sep;6egvp;Gealp;Gep;Kep;Legipxbbbbbbbbbbbbbbbbp:2egralpxbbbjbbbjbbbjbbbjgqp9op9rp;Keglpxb;:FSb;:FSb;:FSb;:FSalalp;Meaiaip;Meavaravaqp9op9rp;Keglalp;Mep;Kep;Kep;Jep;Negvp;Mepxbbn0bbn0bbn0bbn0grp;KepxFFbbFFbbFFbbFFbbp9oaiavp;Mearp;Keczp:Rep9qgialavp;Mearp;KepxFFbbFFbbFFbbFFbbp9oglpmwDKYqk8AExm35Ps8E8Fp9qpkbbabaoawp9oaialpmbezHdiOAlvCXorQLp9qpkbbabcafhbadclfgdae6mbkkk;2ileue97euo97dnaec98GgiTmbcbheinabcKfpx:ji:1S:ji:1S:ji:1S:ji:1SabpbbbglabczfgvpbbbgopmlvorxmPsCXQL358E8Fgrczp:Segwpxibbbibbbibbbibbbp9qp;6egDp;NegqaDaDp;MegDaDp;KealaopmbediwDqkzHOAKY8AEgDczp:Reczp:Sep;6eglalp;MeaDczp:Sep;6egoaop;Mearczp:Reczp:Sep;6egrarp;Mep;Kep;Kep;Lepxbbbbbbbbbbbbbbbbp:4ep;Jep;Mepxbbn0bbn0bbn0bbn0gDp;KepxFFbbFFbbFFbbFFbbgkp9oaqaop;MeaDp;Keczp:Rep9qgoaqalp;MeaDp;Keakp9oaqarp;MeaDp;Keczp:Rep9qgDpmwDKYqk8AExm35Ps8E8Fglp5eawclp:RegqpEi:T:j83ibavalp5baqpEd:T:j83ibabcwfaoaDpmbezHdiOAlvCXorQLgDp5eaqpEe:T:j83ibabaDp5baqpEb:T:j83ibabcafhbaeclfgeai6mbkkkuee97dnadcd4ae2c98GgeTmbcbhdinababpbbbgicwp:Recwp:Sep;6eaicep:SepxbbjFbbjFbbjFbbjFp9opxbbjZbbjZbbjZbbjZp:Uep;Mepkbbabczfhbadclfgdae6mbkkk:Sodw97euaec98Ghedndnadcl9hmbaeTmecbhdinabpxbbuJbbuJbbuJbbuJabpbbbgicKp:TeglaicYp:Tep9qgvcdp:Teavp9qgvclp:Teavp9qgop;6ep;Negvaicwp:RecKp:SegraipxFbbbFbbbFbbbFbbbgwp9ogDp:Uep;6ep;Mepxbbn0bbn0bbn0bbn0gqp;Kecwp:RepxbFbbbFbbbFbbbFbbp9oavaDarp:Xeaiczp:RecKp:Segip:Uep;6ep;Meaqp;Keawp9op9qavaDaraip:Uep:Xep;6ep;Meaqp;Keczp:RepxbbFbbbFbbbFbbbFbp9op9qavaoalcep:Rep9oalpxebbbebbbebbbebbbp9op9qp;6ep;Meaqp;KecKp:Rep9qpkbbabczfhbadclfgdae6mbxdkkaeTmbcbhdinabczfgkpxbFu9hbFu9hbFu9hbFu9habpbbbglakpbbbgrpmlvorxmPsCXQL358E8Fgvczp:TegqavcHp:Tep9qgicdp:Teaip9qgiclp:Teaip9qgicwp:Teaip9qgop;6ep;NegialarpmbediwDqkzHOAKY8AEgDpxFFbbFFbbFFbbFFbbglp9ograDczp:Segwp:Ueavczp:Reczp:SegDp:Xep;6ep;Mepxbbn0bbn0bbn0bbn0gvp;Kealp9oaiarawaDp:Uep:Xep;6ep;Meavp;Keczp:Rep9qgwaiaoaqcep:Rep9oaqpxebbbebbbebbbebbbp9op9qp;6ep;Meavp;Keczp:ReaiaDarp:Uep;6ep;Meavp;Kealp9op9qgipmwDKYqk8AExm35Ps8E8FpkbbabawaipmbezHdiOAlvCXorQLpkbbabcafhbadclfgdae6mbkkk9teiucbcbydj:G:cjbgeabcifc98GfgbBdj:G:cjbdndnabZbcztgd9nmbcuhiabad9RcFFifcz4nbcuSmekaehikaikkxebcj:Gdklz:zbb",t=new Uint8Array([0,97,115,109,1,0,0,0,1,4,1,96,0,0,3,3,2,0,0,5,3,1,0,1,12,1,0,10,22,2,12,0,65,0,65,0,65,0,252,10,0,0,11,7,0,65,0,253,15,26,11]),n=new Uint8Array([32,0,65,2,1,106,34,33,3,128,11,4,13,64,6,253,10,7,15,116,127,5,8,12,40,16,19,54,20,9,27,255,113,17,42,67,24,23,146,148,18,14,22,45,70,69,56,114,101,21,25,63,75,136,108,28,118,29,73,115]);if(typeof WebAssembly!="object")return{supported:!1};var i=WebAssembly.validate(t)?o(e):o(s),r,a=WebAssembly.instantiate(i,{}).then(function(f){r=f.instance,r.exports.__wasm_call_ctors()});function o(f){for(var v=new Uint8Array(f.length),S=0;S<f.length;++S){var b=f.charCodeAt(S);v[S]=b>96?b-97:b>64?b-39:b+4}for(var M=0,S=0;S<f.length;++S)v[M++]=v[S]<60?n[v[S]]:(v[S]-60)*64+v[++S];return v.buffer.slice(0,M)}function l(f,v,S,b,M,E,P){var x=f.exports.sbrk,C=b+3&-4,w=x(C*M),I=x(E.length),y=new Uint8Array(f.exports.memory.buffer);y.set(E,I);var N=v(w,b,M,I,E.length);if(N==0&&P&&P(w,C,M),S.set(y.subarray(w,w+b*M)),x(w-x(0)),N!=0)throw new Error("Malformed buffer data: "+N)}var c={NONE:"",OCTAHEDRAL:"meshopt_decodeFilterOct",QUATERNION:"meshopt_decodeFilterQuat",EXPONENTIAL:"meshopt_decodeFilterExp",COLOR:"meshopt_decodeFilterColor"},h={ATTRIBUTES:"meshopt_decodeVertexBuffer",TRIANGLES:"meshopt_decodeIndexBuffer",INDICES:"meshopt_decodeIndexSequence"},u=[],d=0;function p(f){var v={object:new Worker(f),pending:0,requests:{}};return v.object.onmessage=function(S){var b=S.data;v.pending-=b.count,v.requests[b.id][b.action](b.value),delete v.requests[b.id]},v}function g(f){for(var v="self.ready = WebAssembly.instantiate(new Uint8Array(["+new Uint8Array(i)+"]), {}).then(function(result) { result.instance.exports.__wasm_call_ctors(); return result.instance; });self.onmessage = "+m.name+";"+l.toString()+m.toString(),S=new Blob([v],{type:"text/javascript"}),b=URL.createObjectURL(S),M=u.length;M<f;++M)u[M]=p(b);for(var M=f;M<u.length;++M)u[M].object.postMessage({});u.length=f,URL.revokeObjectURL(b)}function _(f,v,S,b,M){for(var E=u[0],P=1;P<u.length;++P)u[P].pending<E.pending&&(E=u[P]);return new Promise(function(x,C){var w=new Uint8Array(S),I=++d;E.pending+=f,E.requests[I]={resolve:x,reject:C},E.object.postMessage({id:I,count:f,size:v,source:w,mode:b,filter:M},[w.buffer])})}function m(f){var v=f.data;self.ready.then(function(S){if(!v.id)return self.close();try{var b=new Uint8Array(v.count*v.size);l(S,S.exports[v.mode],b,v.count,v.size,v.source,S.exports[v.filter]),self.postMessage({id:v.id,count:v.count,action:"resolve",value:b},[b.buffer])}catch(M){self.postMessage({id:v.id,count:v.count,action:"reject",value:M})}})}return{ready:a,supported:!0,useWorkers:function(f){g(f)},decodeVertexBuffer:function(f,v,S,b,M){l(r,r.exports.meshopt_decodeVertexBuffer,f,v,S,b,r.exports[c[M]])},decodeIndexBuffer:function(f,v,S,b){l(r,r.exports.meshopt_decodeIndexBuffer,f,v,S,b)},decodeIndexSequence:function(f,v,S,b){l(r,r.exports.meshopt_decodeIndexSequence,f,v,S,b)},decodeGltfBuffer:function(f,v,S,b,M,E){l(r,r.exports[h[M]],f,v,S,b,r.exports[c[E]])},decodeGltfBufferAsync:function(f,v,S,b,M){return u.length>0?_(f,v,S,h[b],c[M]):a.then(function(){var E=new Uint8Array(f*v);return l(r,r.exports[h[b]],E,f,v,S,r.exports[c[M]]),E})}}})();var xs={cx:20,cz:69.8,rx:9,rz:2,seconds:24,gaitHz:1.7},ey=["body","fl_hip","fr_hip","hl_hip","hr_hip","fl_uleg","fr_uleg","hl_uleg","hr_uleg","fl_lleg","fr_lleg","hl_lleg","hr_lleg"],ty={fl:0,hr:0,fr:Math.PI,hl:Math.PI};function ny(s){let e=s/xs.seconds*Math.PI*2,t=xs.cx+xs.rx*Math.cos(e),n=xs.cz+xs.rz*Math.sin(e),i=-xs.rx*Math.sin(e),r=xs.rz*Math.cos(e);return{x:t,z:n,heading:Math.atan2(-r,i),phase:s*xs.gaitHz*Math.PI*2}}function iy(s){let e={};for(let[t,n]of Object.entries(ty))e[t]={hip:.26*Math.sin(s+n),uleg:.13*Math.sin(s+n+.9)};return e}function Kp({scene:s,resourceURL:e,signal:t,groundY:n=0,onError:i}){let r=new gt;r.visible=!1,s.add(r);let a=Object.create(null),o=new en,l=null,c=0,h=0,u=!1,d=!1,p=new Ti;p.setMeshoptDecoder(Bu);let g=(async()=>{try{await Bu.ready;let m=await fetch(e("/3d/system-world/spot.glb"),{signal:t});if(!m.ok)throw Error("Companion asset unavailable: "+m.status);let f=await m.arrayBuffer();if(d)return!1;let v=await p.parseAsync(f,"");if(d)return!1;l=v.scene,l.traverse(S=>{S.isMesh&&(S.castShadow=!0,S.receiveShadow=!0)});for(let S of ey)a[S]=l.getObjectByName(S)||null;if(!a.body)throw Error("Companion joint hierarchy missing");return o.setFromObject(l),c=n-o.min.y,r.add(l),r.visible=!0,!0}catch(m){return u=!0,i?.(m),!1}})();function _(m,f){if(!l||d)return;f&&(h+=Math.min(.1,Math.max(0,m)));let v=ny(h);r.position.set(v.x,c+Math.abs(Math.sin(v.phase))*.012,v.z),r.rotation.y=v.heading;for(let[S,b]of Object.entries(iy(v.phase)))a[S+"_hip"]&&(a[S+"_hip"].rotation.y=b.hip),a[S+"_uleg"]&&(a[S+"_uleg"].rotation.y=b.uleg)}return{ready:g,update:_,stats:()=>({loaded:!!l,failed:u,time:+h.toFixed(2),joints:Object.keys(a).filter(m=>a[m]).length,position:[+r.position.x.toFixed(2),+r.position.z.toFixed(2)]}),dispose(){d||(d=!0,l&&l.traverse(m=>{if(m.isMesh){m.geometry.dispose();for(let f of Array.isArray(m.material)?m.material:[m.material])f.dispose()}}),r.removeFromParent(),r.clear())}}}function Yp(s,e,t,n){for(let i of t){let r=e.get(i.asset).lods[0].bounds;(Uu[i.id]||(i.id==="graph"?[[-10.2,-10.2,10.2,10.2]]:[[-3,-3,3,3]])).forEach(([o,l,c,h],u)=>s.solid("district:"+i.id+":"+u,{x:i.x,z:i.z,min:[o,0,l],max:[c,8,h]})),Ou[i.id]?Ou[i.id].forEach(([o,l,c],h)=>s.solid("district:"+i.id+":upper:"+h,{x:i.x,z:i.z,min:[-o,l,-o],max:[o,c,o]})):s.solid("district:"+i.id+":upper",{x:i.x,z:i.z,min:[r.min[0],8,r.min[2]],max:r.max})}n.forEach((i,r)=>{if(i.asset==="street-tile"||i.asset==="street-crossing")return;let a=e.get(i.asset)?.lods[0].bounds;if(!a)return;let o=a.min.map((c,h)=>c*i.scale[h]),l=a.max.map((c,h)=>c*i.scale[h]);o[1]+=i.y,l[1]+=i.y,i.asset==="street-lamp"&&(s.solid("city:"+r+":pole",{x:i.x,z:i.z,min:[-.2,i.y,-.2],max:[.2,l[1],.2]}),o[1]=Math.max(o[1],l[1]-1)),s.solid("city:"+r,{x:i.x,z:i.z,heading:i.angle,min:o,max:l})})}var ao={"city.road":{top:"asphalt",side:"concrete",albedo:.55,rough:.7,bump:.012,wet:1,puddle:[.447,.442]},"city.stone":{top:"pavers",side:"concrete",albedo:.5,rough:.6,bump:.014,wet:1,puddle:[.523,.516]},"city.graphite":{top:"gravel",side:"panels",albedo:.42,rough:.5,bump:.016,wet:.8,metalTop:.3},"city.titanium":{top:"panels",side:"panels",albedo:.2,rough:.45,bump:.006,wet:.6},"city.bronze":{top:"panels",side:"panels",albedo:.18,rough:.4,bump:.006,wet:.6},"city.robot-rubber":{top:"panels",side:"panels",albedo:.3,rough:.75,bump:.008,wet:.25},"city.ceramic":{top:"concrete",side:"concrete",albedo:.3,rough:.4,bump:.008,wet:.8,puddle:[.464,.46]},"city.leaf":{top:"foliage",side:"foliage",albedo:.7,rough:.3,bump:.03,wet:.5},ground:{top:"pavers",side:"concrete",albedo:.55,rough:.6,bump:.014,wet:1,topScale:.5,puddle:[.523,.516],waterline:!0}},Jp=new Set(["city.ivory","city.warm"]),sy={low:0,medium:1,high:2,ultra:2},Ws=s=>Number(s).toFixed(4),ry=`varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
attribute float swSeed;varying float vSwSeed;
#endif`,ay=`
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
#endif`,oy=`uniform float swWet;uniform float swRain;uniform float swTime;uniform vec4 swWindow;uniform vec2 swFade;uniform vec4 swShelter[3];uniform vec3 swSky;
varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
varying float vSwSeed;
#endif
#ifdef SW_SURFACE
uniform sampler2D swTop;uniform sampler2D swSide;uniform vec2 swScale;
float swHash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
vec3 swPerturb(vec3 pos,vec3 n,vec2 dh,float face){vec3 sx=dFdx(pos),sy=dFdy(pos),r1=cross(sy,n),r2=cross(n,sx);float det=dot(sx,r1)*face;return normalize(abs(det)*n-sign(det)*(dh.x*r1+dh.y*r2));}
#endif`,ly=`
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
#endif`,cy=`
#ifdef SW_SURFACE
diffuseColor.rgb*=swAlbedoF*mix(1.0,0.56,swWetness)*mix(1.0,0.7,swPuddle);
#endif`,hy=`
#ifdef SW_SURFACE
roughnessFactor=clamp(roughnessFactor*swRoughF,0.04,1.0);roughnessFactor=mix(roughnessFactor,roughnessFactor*0.6,swWetness);roughnessFactor=mix(roughnessFactor,0.04,swPuddle);
#endif`,uy=`
#ifdef SW_SURFACE
metalnessFactor*=mix(1.0,SW_METAL_TOP,swW.y);
#endif`,dy=`
#if SW_LEVEL>1
normal=swPerturb(-vViewPosition,normal,vec2(dFdx(swH),dFdy(swH))*SW_BUMP*swDetail,faceDirection);
#endif`,fy=`
#ifdef SW_SURFACE
float swFresnel=pow(1.0-clamp(dot(normal,normalize(vViewPosition)),0.0,1.0),5.0)*0.95+0.05;
totalEmissiveRadiance+=swSky*swFresnel*(swPuddle*0.6+swWetness*0.1);
#endif
#ifdef SW_WINDOWS
float swPane=(1.0-smoothstep(0.35,0.65,abs(swN.y)))*(1.0-step(1.5,vSwSeed));
totalEmissiveRadiance*=mix(swWindow.w,mix(swWindow.z,swWindow.y,step(vSwSeed,swWindow.x)),swPane);
#endif`;function py(s){let e=s?.getAttribute("position");if(!e||s.getAttribute("swSeed"))return;let t=e.count,n=new Int32Array(t),i=s.getIndex();for(let u=0;u<t;u++)n[u]=u;let r=u=>{for(;n[u]!==u;)n[u]=n[n[u]],u=n[u];return u},a=u=>i?i.getX(u):u,o=i?i.count:t;for(let u=0;u+2<o;u+=3){let d=r(a(u));n[r(a(u+1))]=d,n[r(a(u+2))]=d}let l=new Map,c=new Map;for(let u=0;u<t;u++){let d=r(u),p=c.get(d)||[0,0,0];l.set(d,(l.get(d)||0)+1),p[0]+=e.getX(u),p[1]+=e.getY(u),p[2]+=e.getZ(u),c.set(d,p)}let h=new Float32Array(t);for(let u=0;u<t;u++){let d=r(u),p=l.get(d),g=c.get(d);if(p>6){h[u]=2;continue}let _=Math.sin(g[0]/p*12.9898+g[1]/p*78.233+g[2]/p*37.719)*43758.5453;h[u]=_-Math.floor(_)}s.setAttribute("swSeed",new Mt(h,1))}function Zp(s,e={}){let t=new AbortController,n=new Map,i=[],r=new ss(new Uint8Array([128,128,128,255]),1,1);r.colorSpace=Yn,r.needsUpdate=!0;let a={},o={},l={};for(let y of Object.values(ao))for(let N of[y.top,y.side])a[N]||={value:r};for(let y of Object.keys(ao))l[y]={value:new Be(1/4,1/4)};let c=(e.shelters||[]).slice(0,3).map(y=>new Pt(...y));for(;c.length<3;)c.push(new Pt(1e6,1e6,-1e6,-1e6));let h={swWet:{value:0},swRain:{value:0},swTime:{value:0},swShelter:{value:c},swSky:{value:new Le(528926)},swWindow:{value:new Pt(.8,1,.06,1)},swFade:{value:new Be(35,140)}},u=2,d=null,p=!1,g=0,_=0,m=0,f=0,v=!1,S=s.capabilities.getMaxAnisotropy(),b=()=>u>1?Math.min(8,S):Math.min(4,S);function M(y,N){let{kind:D,windows:R,space:U}=n.get(N),z=ao[D];Object.assign(y.uniforms,h);let T=["#define SW_LEVEL "+(z?u:0)];U==="world"&&T.push("#define SW_WORLD"),R&&T.push("#define SW_WINDOWS"),z&&(y.uniforms.swTop=a[z.top],y.uniforms.swSide=a[z.side],y.uniforms.swScale=l[D],T.push("#define SW_SURFACE","#define SW_ALBEDO "+Ws(z.albedo),"#define SW_ROUGH "+Ws(z.rough),"#define SW_BUMP "+Ws(z.bump),"#define SW_WETK "+Ws(U==="world"?z.wet:z.wet*.5),"#define SW_METAL_TOP "+Ws(z.metalTop??1),"#define SW_PUDDLE_DRY "+Ws(z.puddle?.[0]??-1),"#define SW_PUDDLE_FULL "+Ws(z.puddle?.[1]??-2)),z.waterline&&T.push("#define SW_WATERLINE"));let G=T.join(`
`)+`
`;y.vertexShader=G+y.vertexShader.replace("#include <common>",`#include <common>
`+ry).replace("#include <begin_vertex>","#include <begin_vertex>"+ay),y.fragmentShader=G+y.fragmentShader.replace("#include <common>",`#include <common>
`+oy).replace("#include <map_fragment>","#include <map_fragment>"+ly).replace("#include <color_fragment>","#include <color_fragment>"+cy).replace("#include <roughnessmap_fragment>","#include <roughnessmap_fragment>"+hy).replace("#include <metalnessmap_fragment>","#include <metalnessmap_fragment>"+uy).replace("#include <normal_fragment_maps>","#include <normal_fragment_maps>"+dy).replace("#include <emissivemap_fragment>","#include <emissivemap_fragment>"+fy)}function E(y,N="world",D=y?.name){if(!y?.isMeshStandardMaterial||n.has(y))return;let R=ao[D]?D:"",U=Jp.has(y.name);!R&&!U||(n.set(y,{kind:R,windows:U,space:N}),y.addEventListener("dispose",()=>n.delete(y)),y.onBeforeCompile=z=>M(z,y),y.customProgramCacheKey=()=>["sw",R,U,N,R?u:0].join("|"),y.needsUpdate=!0)}function P(y,N=!1){if(y?.isMesh)for(let D of Array.isArray(y.material)?y.material:[y.material])E(D,N?"object":"world"),Jp.has(D?.name)&&py(y.geometry)}let x=y=>e.url?.(y),C=null;async function w(y){let N=await fetch(x(y.file),{signal:t.signal});if(!N.ok)throw Error("Surface texture unavailable");let D=await N.blob();if(p)return;let R=await createImageBitmap(D,{premultiplyAlpha:"none",colorSpaceConversion:"none"});if(p){R.close?.();return}let U=new Qt(R);U.wrapS=U.wrapT=pi,U.colorSpace=Yn,U.flipY=!1,U.anisotropy=b(),U.needsUpdate=!0,i.push(U),a[y.id].value=U,m++,g+=D.size,e.onProgress?.(g,_)}function I(){return d||p||!e.url||(d=(async()=>{if(!C){let D=await fetch(x("manifest.json"),{signal:t.signal});if(!D.ok)throw Error("Surface manifest unavailable");C=await D.json();let R=(C.textures||[]).filter(U=>a[U.id]&&/^[a-z-]+\.webp$/.test(U.file)&&U.tile_metres>0);C.entries=R,_=R.reduce((U,z)=>U+(z.bytes||0),0);for(let U of R)o[U.id]=U.tile_metres;for(let[U,z]of Object.entries(ao))l[U].value.set(1/((o[z.top]||4)/(z.topScale||1)),1/(o[z.side]||4))}let y=C.entries.filter(D=>a[D.id].value===r),N=await Promise.allSettled(y.map(w));f+=N.filter(D=>D.status==="rejected").length})().catch(()=>{f++}).finally(()=>{d=null})),d}return{apply:E,prepare:P,setTier(y){let N=sy[y]??2;if(N!==u){u=N;for(let[D,R]of n)R.kind&&(D.needsUpdate=!0)}for(let D of i)D.anisotropy!==b()&&(D.anisotropy=b(),D.needsUpdate=!0);u>0&&I()},setLighting(y,N,D=0,R=null){R&&h.swSky.value.copy(R).multiplyScalar(1.25);let U=1-y,z=Tt.smoothstep(U,.1,.9);h.swWindow.value.set(Math.min(.9,.12+.72*z+.12*D),.22+U*.98+Math.min(1,N)*.1,.05,.55+.45*U)},setWeather(y){v=y==="rain",h.swRain.value=v?1:0},update(y,N){let D=v?1:0,R=h.swWet.value;if(!N){h.swWet.value=D;return}let U=Math.min(.1,Math.max(0,y));h.swTime.value+=U,h.swWet.value=R+(D-R)*Math.min(1,U/(D>R?14:70))},progress:()=>({bytes:g,expected:_}),stats:()=>({detail:u,textures:m,failures:f,bytes:g,materials:n.size,wet:+h.swWet.value.toFixed(2),windows:+h.swWindow.value.x.toFixed(2)}),dispose(){p=!0,t.abort();for(let y of i)y.dispose(),y.source.data?.close?.();i.length=0,r.dispose()}}}function my(s){let e=Math.max(64,Math.round(s.sampleRate*.03)||1440),t=[],n=0;for(let a=0;a<s.numberOfChannels;a++){let o=s.getChannelData(a);for(let l=0;l<o.length;l+=e){let c=0,h=Math.min(l+e,o.length);for(let u=l;u<h;u++){let d=o[u];if(!Number.isFinite(d))throw Error("Invalid voice");n=Math.max(n,Math.abs(d)),c+=d*d}t.push(Math.sqrt(c/(h-l)))}}if(n<1e-4)throw Error("Silent voice");let i=t.filter(a=>a>Math.max(.003,n*.003)).sort((a,o)=>a-o),r=i[Math.floor((i.length-1)*.7)]||n*.5;return Math.min(8,Math.max(1/n,.26/Math.max(r,.001)))}function $p(s,e){let t=!1,n=!1,i=0,r=null,a=null,o=[],l=null,c=null,h=null,u=250,d=0,p=0,g=0,_="idle",m=null,f=()=>.04+.46/(1+(u/110)**2);function v(){let x=s.currentTime;l?.gain.setTargetAtTime(f(),x,.18),c?.frequency.setTargetAtTime(1800+4400/(1+u/95),x,.18),h?.pan.setTargetAtTime(d,x,.18)}function S(){if(a){a.onended=null;try{a.stop()}catch{}a=null}o.forEach(x=>x.disconnect()),o=[],l=c=h=null}function b(){p++,clearTimeout(i),i=0,r?.abort(),r=null,S(),_="idle"}function M(x=2e4+Math.random()*2e4){clearTimeout(i),t&&!n&&(i=setTimeout(()=>{i=0,P()},x))}function E(){if(m)return m;m=s.createBuffer(2,Math.ceil(s.sampleRate*.85),s.sampleRate);let x=42;for(let C=0;C<2;C++){let w=m.getChannelData(C),I=Math.round(s.sampleRate*.015),y=0;for(let D=0;D<w.length;D++){x=Math.imul(x,1664525)+1013904223>>>0;let R=(D-I)/(w.length-I);w[D]=D<I?0:(x/4294967296*2-1)*Math.pow(1-R,2.25),y+=w[D]*w[D]}let N=.65/Math.sqrt(y||1);for(let D=0;D<w.length;D++)w[D]*=N}return m}async function P(){if(!t||n||r||a||s.state!=="running"){M();return}let x=p,C=new AbortController;r=C,_="loading";let w=setTimeout(()=>C.abort(),32e3);try{let I=await fetch("/api/desktop/system-world/voice",{method:"POST",credentials:"same-origin",cache:"no-store",signal:C.signal});if(I.status===204){_="idle";return}if(!I.ok||!I.headers.get("Content-Type")?.startsWith("audio/"))throw Error("Voice unavailable");if(Number(I.headers.get("Content-Length"))>8*1024*1024)throw Error("Voice too large");let y=await I.arrayBuffer();if(y.byteLength>8*1024*1024)throw Error("Voice too large");if(x!==p||!t||n)return;let N=await s.decodeAudioData(y);if(x!==p||!t||n)return;if(!Number.isFinite(N.duration)||N.duration<=0||N.duration>25||N.numberOfChannels>2)throw Error("Invalid voice");let D=my(N),R=We=>(o.push(We),We);a=R(s.createBufferSource()),a.buffer=N;let U=R(s.createGain()),z=R(s.createBiquadFilter());U.gain.value=D,z.type="highpass",z.frequency.value=90,c=R(s.createBiquadFilter()),c.type="lowpass",c.Q.value=.55;let T=R(s.createGain()),G=R(s.createGain()),X=R(s.createGain()),Z=R(s.createGain());T.gain.value=.76,G.gain.value=.22,X.gain.value=.28;let H=R(s.createDelay(.2)),oe=R(s.createConvolver());H.delayTime.value=.085,oe.normalize=!1,oe.buffer=E();let ke=R(s.createWaveShaper()),Fe=new Float32Array(1024);for(let We=0;We<Fe.length;We++)Fe[We]=Math.max(-.75,Math.min(.75,We*2/(Fe.length-1)-1));ke.curve=Fe,l=R(s.createGain()),l.gain.value=f(),h=R(s.createStereoPanner()),h.pan.value=d,a.connect(U).connect(z).connect(c),c.connect(T).connect(Z),c.connect(H).connect(G).connect(Z),c.connect(oe).connect(X).connect(Z),Z.connect(ke).connect(l).connect(h).connect(e),v(),_="speaking",g++,a.onended=()=>{x!==p||!t||n||(_="tail",i=setTimeout(()=>{S(),_="idle",M()},900))},a.start()}catch{x===p&&!n&&(_=C.signal.aborted?"idle":"unavailable")}finally{clearTimeout(w),r===C&&(r=null),x===p&&!n&&!a&&M(_==="unavailable"?6e4:void 0)}}return{setActive(x){t===x||n||(t=x,t?M(3e3+Math.random()*2e3):b())},setListener(x,C,w,I,y){if(!Number.isFinite(x)||!Number.isFinite(C)||!Number.isFinite(w)||!Number.isFinite(I)||!Number.isFinite(y))return;let N=-x,D=-12-w,R=Math.max(0,Math.hypot(N,D)-13);u=Math.hypot(R,Math.max(0,C-87,-C));let U=Math.hypot(N,D),z=Math.hypot(I,y);d=U&&z?Math.max(-.8,Math.min(.8,(D*I-N*y)/U/z))*.8:0,v()},stats(){return{state:_,phrases:g,distance:u,gain:f(),pan:d,pending:!!r,scheduled:!!i}},dispose(){n||(n=!0,t=!1,b(),m=null)}}}function Qp(s,e,t=e){let n=new Set,i={x:0,z:0,fx:0,fz:-1},r=!1,a=!1,o="clear",l=s.createBuffer(1,s.sampleRate*2,s.sampleRate),c=31,h=l.getChannelData(0);for(let f=0;f<h.length;f++)c=Math.imul(c,1664525)+1013904223|0,h[f]=(c>>>0)/2147483648-1;let u=s.createBufferSource(),d=s.createBiquadFilter(),p=s.createGain();u.buffer=l,u.loop=!0,d.type="lowpass",d.frequency.value=450,p.gain.value=0,u.connect(d).connect(p).connect(t),u.start();let g=[{x:-57,z:-57,f:94,g:.055},{x:43,z:62,f:143,g:.035},{x:0,z:14,f:220,g:.016},{x:0,z:79,f:340,g:.06}].map((f,v)=>{let S=v===3?s.createBufferSource():s.createOscillator(),b=s.createGain(),M=s.createStereoPanner(),E=s.createBiquadFilter();return v===3?(S.buffer=l,S.loop=!0):(S.type="sine",S.frequency.value=f.f),E.type="lowpass",E.frequency.value=f.f*2,b.gain.value=0,S.connect(E).connect(b).connect(M).connect(t),S.start(),{...f,source:S,level:b,pan:M,low:E}});function _(){p.gain.setTargetAtTime(r?(o==="rain"?.07:.015)*(a?.12:1):0,s.currentTime,.1),d.frequency.setTargetAtTime(a?280:o==="rain"?2100:600,s.currentTime,.2)}function m(){for(let f of[...n]){try{f.source.stop()}catch{}f.release()}}return{setActive(f){if(r=f,_(),!f){m();for(let v of g)v.level.gain.setTargetAtTime(0,s.currentTime,.05)}},environment(f,v){a=f,v&&(o=v),_()},listener(f,v,S,b,M){Object.assign(i,{x:f,z:S,fx:b,fz:M});for(let E of g){let P=Math.hypot(f-E.x,S-E.z);E.level.gain.setTargetAtTime(r?E.g/(1+P*P*.025)*(a?.25:1):0,s.currentTime,.1),E.pan.pan.value=Math.max(-1,Math.min(1,((E.x-f)*-M+(E.z-S)*b)/Math.max(1,P)))}},play(f,v=0,S=0,b=0){if(!r||n.size>=16)return;let M=Math.hypot(v-i.x,b-i.z);if(M>90)return;let E={step:[.12,240,.16],step_inside:[.1,550,.12],door:[.7,850,.06],lift:[1.5,120,.05],tram:[.8,330,.08],discover:[.45,880,.08],hover:[1.2,160,.07]}[f];if(!E)return;let[P,x,C]=E,w=f.startsWith("step")||f==="door"||f==="hover"?s.createBufferSource():s.createOscillator();"buffer"in w?w.buffer=l:(w.type="sine",w.frequency.value=x);let I=s.createBiquadFilter(),y=s.createGain(),N=s.createStereoPanner();I.type="lowpass",I.frequency.value=a?x*.7:x,y.gain.setValueAtTime(0,s.currentTime),y.gain.linearRampToValueAtTime(C/(1+M*.08),s.currentTime+.015),y.gain.exponentialRampToValueAtTime(1e-4,s.currentTime+P),N.pan.value=Math.max(-1,Math.min(1,((v-i.x)*-i.fz+(b-i.z)*i.fx)/Math.max(M,1))),w.connect(I).connect(y).connect(N).connect(e);let D={source:w,release(){if(n.delete(D))for(let R of[w,I,y,N])R.disconnect()}};n.add(D),w.onended=D.release,w.start(),w.stop(s.currentTime+P+.02)},stats:()=>({voices:n.size,weather:o,inside:a}),dispose(){r=!1,m(),u.stop();for(let f of g){f.source.stop();for(let v of[f.source,f.level,f.pan,f.low])v.disconnect()}for(let f of[u,d,p])f.disconnect()}}}function em(s,e,t=e){let o=new Set,l=new Map,c=new Map,h={x:0,y:0,z:0,fx:0,fz:-1},u={x:0,z:0},d=!1,p=!1,g=0,_=0,m=0,f={kind:0,inactive:0,distant:0,pool:0,retrigger:0,quiet:0},v=s.createBuffer(1,s.sampleRate*2,s.sampleRate),S=9371,b=v.getChannelData(0);for(let C=0;C<b.length;C++)S=Math.imul(S,1664525)+1013904223|0,b[C]=(S>>>0)/2147483648-1;let M={step:{gain:.07,layers:[{wave:"noise",f:1900,q:1.7,dur:.05,lvl:.5},{wave:"noise",f:520,q:.9,dur:.1,lvl:.4},{wave:"tone",f:104,f2:74,dur:.12,lvl:.3}]},step_carry:{gain:.075,layers:[{wave:"noise",f:1250,q:1.2,dur:.06,lvl:.45},{wave:"noise",f:340,q:.8,dur:.13,lvl:.5},{wave:"tone",f:88,f2:62,dur:.15,lvl:.36}]},glide:{gain:.028,layers:[{wave:"noise",f:700,q:.6,dur:.26,lvl:.8},{wave:"tone",f:150,f2:120,dur:.24,lvl:.18}]},greet:{gain:.05,layers:[{wave:"tone",f:520,f2:740,dur:.11,lvl:.5,type:"triangle"},{wave:"tone",f:790,f2:960,dur:.13,lvl:.3,type:"triangle",delay:.1}]},work:{gain:.045,layers:[{wave:"noise",f:2700,q:2.6,dur:.05,lvl:.6},{wave:"tone",f:240,f2:175,dur:.22,lvl:.26,type:"sawtooth"}]},carry:{gain:.034,layers:[{wave:"noise",f:950,q:.7,dur:.12,lvl:.7},{wave:"noise",f:2300,q:1.5,dur:.07,lvl:.25}]},idle:{gain:.02,layers:[{wave:"noise",f:3400,q:3.2,dur:.028,lvl:.7}]},turn:{gain:.026,layers:[{wave:"tone",f:900,f2:1440,dur:.18,lvl:.4,type:"sawtooth"}]}};function E(C,w,I,y,N){let D=C.startsWith("robot_")?C.slice(6):C,R=M[D];if(!R)return f.kind++,!1;if(!d)return f.inactive++,!1;let U=w-u.x,z=y-u.z,T=Math.hypot(U,z);if(T>48)return f.distant++,!1;if(o.size>=10){let le=null;for(let se of o)(!le||se.distance>le.distance)&&(le=se);if(!le||T>=le.distance)return f.pool++,!1;le.release()}let G=s.currentTime;if(N!=null){let le=c.get(N);if(le!=null&&G-le<.25)return f.retrigger++,!1;c.set(N,G)}let X=Math.abs(I-h.y),Z=R.gain*4.5/(1+T*.12)/(1+X*.0045)*(p?.5:1);if(Z<.002)return f.quiet++,!1;let H=w-h.x,oe=y-h.z,ke=Math.max(-1,Math.min(1,(H*-h.fz+oe*h.fx)/Math.max(1,Math.hypot(H,oe)))),Fe=s.createGain(),We=s.createStereoPanner(),fe=s.createGain();Fe.gain.value=Z,We.pan.value=ke,fe.gain.value=p?.14:.26;let re=s.createBiquadFilter();re.type="lowpass",re.Q.value=.6,re.frequency.value=Math.max(700,Math.min(11e3,11e3-T*130)),re.connect(Fe),Fe.connect(We).connect(e),We.connect(fe).connect(t);let Pe=[re],be={distance:T,release(){if(o.delete(be)){for(let le of Pe){try{le.stop?.()}catch{}try{le.disconnect()}catch{}}for(let le of[Fe,We,fe])try{le.disconnect()}catch{}}}};o.add(be);let ge=null,ie=0;for(let le of R.layers){let se=G+(le.delay||0),ce=Math.max(le.dur,(le.attack||.006)+.02),ve=s.createGain();ve.gain.setValueAtTime(0,se),ve.gain.linearRampToValueAtTime(le.lvl,se+(le.attack||.006)),ve.gain.exponentialRampToValueAtTime(1e-4,se+ce),ve.connect(re);let Ee;if(le.wave==="noise"){Ee=s.createBufferSource(),Ee.buffer=v,Ee.loop=!0;let Ye=s.createBiquadFilter();Ye.type="bandpass",Ye.frequency.value=le.f,Ye.Q.value=le.q||1,Ee.connect(Ye).connect(ve),Pe.push(Ye)}else Ee=s.createOscillator(),Ee.type=le.type||"sine",Ee.frequency.setValueAtTime(le.f,se),le.f2&&Ee.frequency.exponentialRampToValueAtTime(Math.max(20,le.f2),se+ce),Ee.connect(ve);Pe.push(Ee,ve);let qe=se+ce+.02;qe>ie&&(ie=qe,ge=Ee),Ee.start(se),Ee.stop(qe)}return ge&&(ge.onended=be.release),g++,Z>m&&(m=Z),!0}function P(C,w,I,y,N,D,R){if(!d)return;let U=l.get(C);if(!U){l.set(C,{x:w,z:y,phase:0});return}let z=w-U.x,T=y-U.z,G=Math.hypot(z,T);if(U.x=w,U.z=y,!N){U.phase=0;return}if(G>8)return;U.phase+=G;let X=R==="glide"?1.6:D?1.05:.8;U.phase<X||(U.phase-=X,E(R==="glide"?"glide":D?"step_carry":"step",w,I,y,C),_++)}function x(){for(let C of[...o])C.release();for(let[,C]of l)C.x=h.x,C.z=h.z,C.phase=0;c.clear()}return{setActive(C){d!==C&&(d=C,C||x())},environment(C){p=!!C},listener(C,w,I,y,N,D,R){Object.assign(h,{x:C,y:w,z:I,fx:y,fz:N}),u.x=D??C,u.z=R??I},action(C,w,I,y,N){return E(C,w,I,y,N)},gait:P,reset(){l.clear(),c.clear()},stats:()=>({voices:o.size,played:g,steps:_,gaits:l.size,inside:p,peak:+m.toFixed(4),rejected:{...f}}),dispose(){d=!1,x()}}}var ku=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]],tm=[440,523.25,587.33,659.25,783.99,880,1046.5],Xi=12,Cc={harbour:{x:0,z:79},quay:{x:0,z:72},tower:{x:0,z:-12},square:{x:8,z:40}},gy=[{kind:"chime",weight:15},{kind:"swoosh",weight:13},{kind:"gust",weight:7},{kind:"birds",weight:8,when:s=>s.day>.45&&s.weather==="clear"},{kind:"gull",weight:6,when:s=>s.day>.2&&s.weather==="clear"},{kind:"pigeon",weight:4,when:s=>s.day>.6&&!s.busy},{kind:"cricket",weight:6,when:s=>s.day<.25&&s.weather!=="rain"},{kind:"horn",weight:4,when:s=>s.day<.45&&s.weather!=="rain"},{kind:"bell",weight:3,when:s=>s.day<.7},{kind:"hammer",weight:4,when:s=>s.day>.3},{kind:"drip",weight:10,when:s=>s.weather==="rain"},{kind:"windchime",weight:4,when:s=>s.weather==="clear"&&!s.busy},{kind:"siren",weight:2,when:s=>s.evening>.3&&s.busy}];function by(s,e,t){let n=Math.floor(s.sampleRate*e),i=s.createBuffer(2,n,s.sampleRate),r=977;for(let a=0;a<2;a++){let o=i.getChannelData(a);for(let l=0;l<n;l++)r=Math.imul(r,1664525)+1013904223|0,o[l]=((r>>>0)/2147483648-1)*Math.pow(1-l/n,t)}return i}function xy(s=()=>{}){let e=null,t=null,n=null,i=!1,r=!1,a=!1,o=0,l=null,c=[],h=[],u=new Float32Array(256),d=new Set,p=.18,g=null,_=null,m=null,f=null,v=null,S=!1,b="clear",M=[],E=null,P=null,x=null,C=null,w=null,I=null,y=null,N=null,D=null,R="0.000",U="0.000",z="0.000",T=0,G=0,X=0,Z=0,H={x:0,y:2.4,z:0,fx:0,fz:-1,origin:{x:0,z:0}},oe={busy:!1,day:0,evening:0,weather:"clear"},ke={ambience:1,effects:1,voice:1};try{a=localStorage.getItem("aurago.desktop.sysworld.sound")==="true"}catch{}function Fe(){if(e||i)return;e=new(window.AudioContext||window.webkitAudioContext);let V=lt=>(c.push(lt),lt),j=lt=>(h.push(lt),V(lt));t=V(e.createGain()),t.gain.value=0,n=V(e.createAnalyser()),n.fftSize=512,t.connect(n),n.connect(e.destination),m=V(e.createGain()),m.gain.value=ke.ambience,m.connect(t),f=V(e.createGain()),f.gain.value=ke.effects,f.connect(t),v=V(e.createGain()),v.gain.value=ke.voice,v.connect(t),w=V(e.createConvolver()),w.buffer=by(e,3.4,2.6);let me=V(e.createGain());me.gain.value=.55,w.connect(me).connect(m),l=$p(e,v),g=Qp(e,f,m),g.environment(S,b),_=em(e,f,m),_.environment(S),_.listener(H.x,H.y,H.z,H.fx,H.fz,H.origin.x,H.origin.z);for(let[lt,an]of[[55,.07],[82.4069,.022],[110,.01]]){let Rt=j(e.createOscillator()),Yt=V(e.createGain());Rt.frequency.value=lt,Yt.gain.value=an,Rt.connect(Yt).connect(m),Rt.start()}E=V(e.createBiquadFilter()),E.type="lowpass",E.frequency.value=650,E.Q.value=.6,P=V(e.createGain()),P.gain.value=.042;let _e=V(e.createGain());_e.gain.value=.9,E.connect(P).connect(m),P.connect(_e).connect(w),M=ku[0].map(lt=>[-6,6].map(an=>{let Rt=j(e.createOscillator()),Yt=V(e.createGain());return Rt.type="sawtooth",Rt.frequency.value=lt,Rt.detune.value=an,Yt.gain.value=.22,Rt.connect(Yt).connect(E),Rt.start(),Rt}));let pe=j(e.createOscillator()),xe=V(e.createGain());pe.frequency.value=.05,xe.gain.value=180,pe.connect(xe).connect(E.frequency),pe.start();let Se=e.createBuffer(1,e.sampleRate*4,e.sampleRate),we=Se.getChannelData(0),Ie=0;for(let lt=0;lt<we.length;lt++)Ie=(Ie+(Math.random()*2-1)*.018)/1.018,we[lt]=Ie;let q=we[we.length-1]-we[0];for(let lt=0;lt<we.length;lt++)we[lt]-=q*lt/(we.length-1);let Te=j(e.createBufferSource());x=V(e.createBiquadFilter()),C=V(e.createGain()),Te.buffer=Se,Te.loop=!0,x.type="lowpass",x.frequency.value=680,x.Q.value=.25,C.gain.value=.32,Te.connect(x).connect(C).connect(m),Te.start();let Me=j(e.createOscillator()),ue=V(e.createGain());Me.frequency.value=.075,ue.gain.value=160,Me.connect(ue).connect(x.frequency),Me.start(),I=e.createBuffer(1,e.sampleRate*2,e.sampleRate);let Re=I.getChannelData(0);for(let lt=0;lt<Re.length;lt++)Re[lt]=Math.random()*2-1;let ye=j(e.createBufferSource());ye.buffer=I,ye.loop=!0;let je=V(e.createBiquadFilter());je.type="bandpass",je.frequency.value=620,je.Q.value=.5;let Ke=j(e.createOscillator()),B=V(e.createGain());Ke.frequency.value=.08,B.gain.value=140,Ke.connect(B).connect(je.frequency),Ke.start(),y=V(e.createGain()),y.gain.value=0,ye.connect(je).connect(y).connect(m),ye.start();let W=j(e.createBufferSource());W.buffer=I,W.loop=!0;let ee=V(e.createBiquadFilter());ee.type="bandpass",ee.frequency.value=320,ee.Q.value=.7;let ae=j(e.createOscillator()),Y=V(e.createGain());ae.frequency.value=.13,Y.gain.value=90,ae.connect(Y).connect(ee.frequency),ae.start(),N=V(e.createGain()),N.gain.value=0,W.connect(ee).connect(N).connect(m),W.start();let Q=j(e.createOscillator()),Ae=j(e.createOscillator());Q.type="sine",Q.frequency.value=118,Ae.type="sine",Ae.frequency.value=236;let Ve=V(e.createGain());Ve.gain.value=.26;let rt=V(e.createGain());rt.gain.value=.9;let Ze=j(e.createOscillator()),Ht=V(e.createGain());Ze.frequency.value=.19,Ht.gain.value=.08,Ze.connect(Ht).connect(rt.gain),Ze.start();let Et=V(e.createBiquadFilter());Et.type="lowpass",Et.frequency.value=420,Et.Q.value=.4,D=V(e.createGain()),D.gain.value=0,Q.connect(rt),Ae.connect(Ve).connect(rt),rt.connect(Et).connect(D).connect(m),Q.start(),Ae.start(),he(),Ue()}let We=()=>!!e&&a&&r&&!i&&p>0;function fe(V){let j={release(){if(d.delete(j))for(let me of V){try{me.stop?.()}catch{}try{me.disconnect()}catch{}}}};return d.add(j),V[0].onended=j.release,Z++,j}function re(V,j,me){let _e=e.createStereoPanner(),pe=e.createGain();return _e.pan.value=j,pe.gain.value=me,V.connect(_e).connect(m),_e.connect(pe).connect(w),[_e,pe]}function Pe(V){let j=V.x-H.x,me=V.z-H.z,_e=V.x-H.origin.x,pe=V.z-H.origin.z,xe=Math.hypot(_e,pe),Se=Math.max(1,Math.hypot(j,me));return{distance:xe,pan:Math.max(-1,Math.min(1,(j*-H.fz+me*H.fx)/Se)),fade:1/(1+xe*.02)}}function be(V){if(V==="horn"||V==="gull")return Cc.harbour;if(V==="windchime")return Cc.quay;if(V==="bell")return Cc.tower;if(V==="pigeon")return Cc.square;let j=Math.random()*Math.PI*2,me=24+Math.random()*44;return{x:H.origin.x+Math.sin(j)*me,z:H.origin.z+Math.cos(j)*me}}function ge(V){let j=e.currentTime,me=tm[Math.floor(Math.random()*tm.length)],_e=e.createOscillator(),pe=e.createOscillator(),xe=e.createGain(),Se=e.createGain(),{pan:we,fade:Ie}=Pe(V);_e.frequency.value=me,pe.frequency.value=me*2.76,xe.gain.value=.22,Se.gain.setValueAtTime(0,j),Se.gain.linearRampToValueAtTime(oe.busy?.02:.014,j+.008),Se.gain.exponentialRampToValueAtTime(1e-4,j+2.8),_e.connect(Se),pe.connect(xe).connect(Se),fe([_e,pe,xe,Se,...re(Se,we,1.4)]),_e.start(j),pe.start(j),_e.stop(j+2.9),pe.stop(j+2.9)}function ie(V){let j=e.currentTime,me=e.createBufferSource(),_e=e.createBiquadFilter(),pe=e.createGain(),{pan:xe,fade:Se}=Pe(V);me.buffer=I,me.loop=!0,_e.type="bandpass",_e.Q.value=1.4,_e.frequency.setValueAtTime(260,j),_e.frequency.exponentialRampToValueAtTime(1100,j+1.6),_e.frequency.exponentialRampToValueAtTime(380,j+3.4),pe.gain.setValueAtTime(0,j),pe.gain.linearRampToValueAtTime(.03*Se,j+1.6),pe.gain.exponentialRampToValueAtTime(1e-4,j+3.6),me.connect(_e).connect(pe);let[we,Ie]=re(pe,xe,.5),q=Math.max(-1,xe-.6),Te=Math.min(1,xe+.6);we.pan.setValueAtTime(q,j),we.pan.linearRampToValueAtTime(Te,j+3.4),fe([me,_e,pe,we,Ie]),me.start(j),me.stop(j+3.7)}function le(){let V=e.currentTime,j=b==="rain"?.36:.32;x.frequency.cancelScheduledValues(V),C.gain.cancelScheduledValues(V),x.frequency.setTargetAtTime(1200+Math.random()*700,V,.8),x.frequency.setTargetAtTime(680,V+2.6,1.4),C.gain.setTargetAtTime(j*1.7,V,.7),C.gain.setTargetAtTime(j,V+2.6,1.5),Z++}function se(V){let j=e.currentTime,me=e.createBiquadFilter(),_e=e.createGain(),{pan:pe,fade:xe}=Pe(V);me.type="lowpass",me.frequency.value=420,_e.gain.setValueAtTime(0,j),_e.gain.linearRampToValueAtTime(.028*xe,j+.9),_e.gain.setValueAtTime(.028*xe,j+3.2),_e.gain.exponentialRampToValueAtTime(1e-4,j+5.5);let Se=[69.3,103.83].map(we=>{let Ie=e.createOscillator();return Ie.type="sawtooth",Ie.frequency.value=we,Ie.detune.value=Math.random()*8-4,Ie.connect(me),Ie});me.connect(_e),fe([...Se,me,_e,...re(_e,pe,1.6)]),Se.forEach(we=>{we.start(j),we.stop(j+5.6)})}function ce(V){let j=e.currentTime,me=2+Math.floor(Math.random()*3),{pan:_e,fade:pe}=Pe(V);for(let xe=0;xe<me&&d.size<Xi;xe++){let Se=j+xe*(.12+Math.random()*.1),we=e.createOscillator(),Ie=e.createGain(),q=2600+Math.random()*900;we.frequency.setValueAtTime(q,Se),we.frequency.exponentialRampToValueAtTime(q*1.5,Se+.07),Ie.gain.setValueAtTime(0,Se),Ie.gain.linearRampToValueAtTime(.006*pe,Se+.015),Ie.gain.exponentialRampToValueAtTime(1e-4,Se+.13),we.connect(Ie),fe([we,Ie,...re(Ie,_e,.6)]),we.start(Se),we.stop(Se+.15)}}function ve(V){let j=e.currentTime,me=2+Math.floor(Math.random()*3),_e=760+Math.random()*280,{pan:pe,fade:xe}=Pe(V);for(let Se=0;Se<me&&d.size<Xi;Se++){let we=j+Se*(.2+Math.random()*.14),Ie=e.createOscillator(),q=e.createBiquadFilter(),Te=e.createGain();Ie.type="sawtooth",Ie.frequency.setValueAtTime(_e,we),Ie.frequency.exponentialRampToValueAtTime(_e*.5,we+.26),q.type="bandpass",q.frequency.value=1500,q.Q.value=3.6,Te.gain.setValueAtTime(0,we),Te.gain.linearRampToValueAtTime(.012*xe,we+.04),Te.gain.exponentialRampToValueAtTime(1e-4,we+.34),Ie.connect(q).connect(Te),fe([Ie,q,Te,...re(Te,pe,.9)]),Ie.start(we),Ie.stop(we+.36)}}function Ee(V){let j=e.currentTime,me=2+Math.floor(Math.random()*2),_e=420+Math.random()*90,{pan:pe,fade:xe}=Pe(V);for(let Se=0;Se<me&&d.size<Xi;Se++){let we=j+Se*(.17+Math.random()*.08),Ie=e.createOscillator(),q=e.createBiquadFilter(),Te=e.createGain();Ie.type="triangle",Ie.frequency.setValueAtTime(_e,we),Ie.frequency.linearRampToValueAtTime(_e*.72,we+.12),q.type="lowpass",q.frequency.value=1100,Te.gain.setValueAtTime(0,we),Te.gain.linearRampToValueAtTime(.01*xe,we+.03),Te.gain.exponentialRampToValueAtTime(1e-4,we+.16),Ie.connect(q).connect(Te),fe([Ie,q,Te,...re(Te,pe,.5)]),Ie.start(we),Ie.stop(we+.18)}}function qe(V){let j=e.currentTime,me=3+Math.floor(Math.random()*4),_e=3900+Math.random()*900,{pan:pe,fade:xe}=Pe(V);for(let Se=0;Se<me&&d.size<Xi;Se++){let we=j+Se*.055,Ie=e.createOscillator(),q=e.createGain();Ie.type="square",Ie.frequency.value=_e,q.gain.setValueAtTime(0,we),q.gain.linearRampToValueAtTime(.005*xe,we+.008),q.gain.exponentialRampToValueAtTime(1e-4,we+.035),Ie.connect(q),fe([Ie,q,...re(q,pe,.35)]),Ie.start(we),Ie.stop(we+.05)}}function Ye(V){let j=e.currentTime,me=196,{pan:_e,fade:pe}=Pe(V),xe=e.createGain();xe.gain.setValueAtTime(0,j),xe.gain.linearRampToValueAtTime(.02*pe,j+.012),xe.gain.exponentialRampToValueAtTime(1e-4,j+4.2);let Se=[1,2.02,2.99,4.18,5.43].map((we,Ie)=>{let q=e.createOscillator(),Te=e.createGain();return q.type="sine",q.frequency.value=me*we,Te.gain.value=.5/(Ie+1),q.connect(Te).connect(xe),q});fe([...Se,xe,...re(xe,_e,1.8)]),Se.forEach(we=>{we.start(j),we.stop(j+4.3)})}function st(V){let j=e.currentTime,me=2+Math.floor(Math.random()*3),{pan:_e,fade:pe}=Pe(V);for(let xe=0;xe<me&&d.size<Xi;xe++){let Se=j+xe*(.5+Math.random()*.35),we=e.createBufferSource(),Ie=e.createBiquadFilter(),q=e.createGain(),Te=e.createOscillator(),Me=e.createGain();we.buffer=I,we.loop=!0,Ie.type="bandpass",Ie.frequency.value=420,Ie.Q.value=1.1,q.gain.setValueAtTime(0,Se),q.gain.linearRampToValueAtTime(.02*pe,Se+.006),q.gain.exponentialRampToValueAtTime(1e-4,Se+.42),Te.type="sine",Te.frequency.value=1180,Me.gain.value=.2,we.connect(Ie).connect(q),Te.connect(Me).connect(q),fe([we,Ie,q,Te,Me,...re(q,_e,.9)]),we.start(Se),we.stop(Se+.44),Te.start(Se),Te.stop(Se+.44)}}function at(V){let j=e.currentTime,me=2+Math.floor(Math.random()*4),{pan:_e,fade:pe}=Pe(V);for(let xe=0;xe<me&&d.size<Xi;xe++){let Se=j+xe*(.06+Math.random()*.14),we=e.createOscillator(),Ie=e.createGain();we.type="sine",we.frequency.setValueAtTime(900+Math.random()*700,Se),we.frequency.exponentialRampToValueAtTime(320,Se+.06),Ie.gain.setValueAtTime(0,Se),Ie.gain.linearRampToValueAtTime(.008*pe,Se+.005),Ie.gain.exponentialRampToValueAtTime(1e-4,Se+.12),we.connect(Ie),fe([we,Ie,...re(Ie,_e,.4)]),we.start(Se),we.stop(Se+.14)}}function ot(V){let j=e.currentTime,me=3+Math.floor(Math.random()*4),_e=[1046.5,1174.7,1396.9,1567.98,1760,2093],{pan:pe,fade:xe}=Pe(V);for(let Se=0;Se<me&&d.size<Xi;Se++){let we=j+Se*(.09+Math.random()*.13),Ie=_e[Math.floor(Math.random()*_e.length)],q=e.createOscillator(),Te=e.createGain();q.type="sine",q.frequency.value=Ie,Te.gain.setValueAtTime(0,we),Te.gain.linearRampToValueAtTime(.006*xe,we+.004),Te.gain.exponentialRampToValueAtTime(1e-4,we+.9),q.connect(Te),fe([q,Te,...re(Te,pe,1.5)]),q.start(we),q.stop(we+.95)}}function J(V){let j=e.currentTime,{pan:me,fade:_e}=Pe(V),pe=e.createOscillator(),xe=e.createGain();pe.type="sine",pe.frequency.setValueAtTime(420,j),pe.frequency.linearRampToValueAtTime(620,j+1.8),pe.frequency.linearRampToValueAtTime(430,j+3.6),xe.gain.setValueAtTime(0,j),xe.gain.linearRampToValueAtTime(.01*_e,j+.5),xe.gain.setValueAtTime(.01*_e,j+2.8),xe.gain.exponentialRampToValueAtTime(1e-4,j+4.2),pe.connect(xe),fe([pe,xe,...re(xe,me,1.2)]),pe.start(j),pe.stop(j+4.3)}let dt={chime:ge,swoosh:ie,gust:le,horn:se,birds:ce,gull:ve,pigeon:Ee,cricket:qe,bell:Ye,hammer:st,drip:at,windchime:ot,siren:J};function ct(){let V=gy.filter(_e=>!_e.when||_e.when(oe));if(!V.length)return"chime";let j=0;for(let _e of V)j+=_e.weight;let me=Math.random()*j;for(let _e of V)if(me-=_e.weight,me<=0)return _e.kind;return V[V.length-1].kind}function O(){if(d.size>=Xi)return;let V=ct(),j=be(V);dt[V]?.(j)}function A(){clearTimeout(T),T=0,We()&&(T=setTimeout(()=>{T=0,We()&&(O(),A())},(oe.busy?1900:3600)+Math.random()*9e3))}function ne(){clearTimeout(G),G=0,We()&&(G=setTimeout(()=>{G=0,We()&&(X=(X+1)%ku.length,M.forEach((V,j)=>V.forEach(me=>me.frequency.setTargetAtTime(ku[X][j],e.currentTime,1.8))),ne())},16e3))}function de(){clearTimeout(T),clearTimeout(G),T=G=0;for(let V of[...d])V.release()}function he(){if(!e)return;let V=e.currentTime;E.frequency.setTargetAtTime(oe.busy?1500:650+oe.day*450+oe.evening*150,V,2.5),P.gain.setTargetAtTime((oe.busy?.055:.042)*(oe.weather==="rain"?.75:1),V,2)}function Ue(){if(!e||!y||!N||!D)return;let V=Math.max(-76,Math.min(76,H.origin.x)),j=Math.hypot(H.origin.x-V,H.origin.z-77),me=Math.max(0,1-(j-6)/42),_e=Math.max(0,1-(j-25)/110)*.18,pe=(S?0:Math.min(1,me+_e)*.05*(b==="rain"?1.2:1)).toFixed(3),xe=((oe.busy?.03:.012)*(.35+oe.day*.65)*(S?.25:1)).toFixed(3),Se=Math.hypot(H.origin.x-V,H.origin.z-75.5),we=Math.max(0,1-(Se-8)/46),Ie=(S?0:Math.min(1,we)*.012*(1-oe.day)*(b==="rain"?.6:1)).toFixed(3);if(pe===R&&xe===U&&Ie===z)return;R=pe,U=xe,z=Ie;let q=e.currentTime;y.gain.setTargetAtTime(Number(pe),q,.5),N.gain.setTargetAtTime(Number(xe),q,.8),D.gain.setTargetAtTime(Number(Ie),q,.8)}function Oe(V=!1){if(clearTimeout(o),!i){if(V&&a)try{Fe()}catch{a=!1,s(!1);return}e&&(l?.setActive(a&&r&&p>0&&ke.voice>0),g?.setActive(a&&r&&p>0),_?.setActive(a&&r&&p>0&&ke.effects>0),a&&r?(e.resume().catch(()=>{}),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(p,e.currentTime,.25),T||A(),G||ne()):(de(),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(0,e.currentTime,.035),o=setTimeout(()=>{!i&&(!a||!r)&&e.suspend().catch(()=>{})},180)))}}return{enabled:()=>a,toggle(){a=!a;try{localStorage.setItem("aurago.desktop.sysworld.sound",String(a))}catch{}Oe(!0),s(a)},unlock(){Oe(!0)},setActive(V){r!==V&&(r=V,Oe())},setVolume(V){Number.isFinite(V)&&(p=Math.max(0,Math.min(.35,V)),Oe())},setListener(V,j,me,_e,pe,xe,Se){Object.assign(H,{x:V,y:j,z:me,fx:_e,fz:pe}),H.origin.x=xe??V,H.origin.z=Se??me,l?.setListener(V,j,me,_e,pe),g?.listener(V,j,me,_e,pe),_?.listener(V,j,me,_e,pe,H.origin.x,H.origin.z),Ue()},setChannel(V,j){if(!(V in ke)||!Number.isFinite(j))return;ke[V]=Math.max(0,Math.min(1,j));let me={ambience:m,effects:f,voice:v}[V];me&&me.gain.setTargetAtTime(ke[V],e.currentTime,.05),Oe()},effect(V,j,me,_e,pe){if(String(V).startsWith("robot_")){_?.action(V,j,me,_e,pe);return}g?.play(V,j,me,_e)},gait(V,j,me,_e,pe,xe,Se){_?.gait(V,j,me,_e,pe,xe,Se)},setEnvironment(V,j){S=!!V,["clear","rain","fog"].includes(j)&&(b=j),g?.environment(S,b),_?.environment(S),Ue()},setMood(V={}){Object.assign(oe,{busy:!!V.busy,day:Number.isFinite(V.day)?V.day:oe.day,evening:Number.isFinite(V.evening)?V.evening:oe.evening,weather:["clear","rain","fog"].includes(V.weather)?V.weather:oe.weather}),he(),Ue()},thunder(V=1){if(!We()||S||d.size>=Xi)return;let j=Math.max(0,Math.min(5,V)),me=e.currentTime+j,_e=Math.min(1,j/4),pe=e.createBufferSource(),xe=e.createBiquadFilter(),Se=e.createGain();pe.buffer=I,pe.loop=!0,xe.type="lowpass",xe.Q.value=.7,xe.frequency.setValueAtTime(900-_e*500,me),xe.frequency.exponentialRampToValueAtTime(70,me+3.5),Se.gain.setValueAtTime(0,me),Se.gain.linearRampToValueAtTime(.09*(1-_e*.5),me+.06+_e*.25),Se.gain.exponentialRampToValueAtTime(.025,me+1.3),Se.gain.exponentialRampToValueAtTime(1e-4,me+5.5),pe.connect(xe).connect(Se),fe([pe,xe,Se,...re(Se,Math.random()*.8-.4,.8)]),pe.start(me),pe.stop(me+5.6)},stats(){let V=0;if(n&&e.state==="running"){n.getFloatTimeDomainData(u);for(let j of u)V+=j*j}return{voice:l?.stats(),effects:g?.stats(),foley:_?.stats(),channels:{...ke},enabled:a,active:r,state:e?.state||"uninitialized",volume:p,rms:Math.sqrt(V/u.length),soundscape:{chord:X,events:Z,transient:d.size,scheduled:!!T,mood:{...oe},beds:{water:Number(R),murmur:Number(U),lamp:Number(z)},listener:{x:H.x,z:H.z}}}},dispose(){i||(i=!0,de(),_?.dispose(),g?.dispose(),l?.dispose(),clearTimeout(o),h.forEach(V=>{try{V.stop()}catch{}}),c.forEach(V=>V.disconnect()),e&&e.close().catch(()=>{}))}}}var un=[{id:"agent",asset:"agent-spire",x:0,z:-12,height:87,radius:13},{id:"infra",asset:"compute-foundry",x:-43,z:-53,height:24,radius:18},{id:"integrations",asset:"integration-gate",x:43,z:-10,height:34,radius:14},{id:"missions",asset:"mission-terminal",x:43,z:36,height:23,radius:15},{id:"memory",asset:"memory-archive",x:-43,z:-10,height:37,radius:14},{id:"graph",asset:"knowledge-atrium",x:-43,z:36,height:28,radius:17},{id:"operations",asset:"operations-beacon",x:0,z:36,height:22,radius:6}],Pc={low:{lod:2,dpr:1,shadow:0,bloom:!1},medium:{lod:1,dpr:1.25,shadow:1024,bloom:!1},high:{lod:0,dpr:1.5,shadow:2048,bloom:!0},ultra:{lod:0,dpr:2,shadow:4096,bloom:!0}},Hu=new k(122,106,183),Gu=class extends Gn{constructor(e,t,n){super(),this.scene=e,this.camera=t,this.needsSwap=!1,this.target=new Kt(1,1,{type:rn,samples:n}),this.quad=new Ai(new vt({uniforms:ci.clone(Wi.uniforms),vertexShader:Wi.vertexShader,depthTest:!1,depthWrite:!1,fragmentShader:"uniform sampler2D tDiffuse;varying vec2 vUv;void main(){vec4 c=texture2D(tDiffuse,vUv);if(any(isnan(c))||any(isinf(c)))c=vec4(0.0,0.0,0.0,1.0);gl_FragColor=c;}"}))}render(e,t,n){let i=e.autoClear;e.autoClear=!1,e.setRenderTarget(this.target),e.clear(),e.render(this.scene,this.camera),this.quad.material.uniforms.tDiffuse.value=this.target.texture,e.setRenderTarget(this.renderToScreen?null:n),this.quad.render(e),e.autoClear=i}setSize(e,t){this.target.setSize(e,t)}dispose(){this.target.dispose(),this.quad.material.dispose(),this.quad.dispose()}},Kr=new k(0,22,-12),qs=[];{let s=(r,a,o,l=0,c=0,h=[1,1,1])=>qs.push({asset:r,x:a,z:o,y:l,angle:c,scale:h}),{xs:e,zs:t}=Fn;for(let r of t){for(let o of e)s("street-crossing",o,r);let a=[Fn.minX,...e,Fn.maxX];for(let o=0;o<a.length-1;o++){let l=a[o]+(o?6:0),c=a[o+1]-(o<a.length-2?6:0);s("street-tile",(l+c)/2,r,0,0,[(c-l)/16,1,1])}}for(let r of e){let a=[Fn.minZ,...t,Fn.maxZ];for(let o=0;o<a.length-1;o++){let l=a[o]+(o?6:0),c=a[o+1]-(o<a.length-2?6:0);s("street-tile",r,(l+c)/2,0,Math.PI/2,[(c-l)/16,1,1])}}for(let r of t)for(let a of[-55,-31,31,55])s("street-lamp",a,r-4.5,.45),s("planter",a+4,r-4.5,.45);for(let{x:r,z:a,scale:o}of Ac)s("data-tower-a",r,a,0,0,[1,o,1]);s("skybridge",39.75,-55,15),s("server-rack",-55,-36.7,.5),s("server-rack",-49,-36.7,.5);let n=7919,i=()=>(n=Math.imul(n,1664525)+1013904223>>>0)/4294967296;for(let r=0;r<4;r++)for(let a=0;a<12;a++){let o=i(),l=i(),c=i(),h=i(),u=i(),d=i();if(h<.12)continue;let p=(a-5.5)*22+(o-.5)*12,g=-143-r*29+(l-.5)*14,_=1-Math.min(1,Math.abs(p)/150),m=(.5+c*c+.45*_)*(r?1:.85),f=.7+.34*u;s(o>.5?"data-tower-a":"data-tower-b",p,g,-3,d<.3?Math.PI/2:0,[f,m,f])}for(let r=0;r<16;r++){let a=(r-7.5)*34+(i()-.5)*16,o=-268-i()*46;s(i()<.5?"data-tower-a":"data-tower-b",a,o,-3,0,[1.1,.55+i()*1.1,1.1])}}var vy=[...lp(qs),..._n.flatMap(s=>[-3,3].flatMap(e=>[-3,3].map(t=>({x:s.x+e,z:s.z+t,r:4.25,asset:"interior"}))))];async function K1(s,e){let t=!1,n=!0,i="orbit",r=e.quality||"auto",a=r==="auto"?"high":r,o=0,l=null,c=0,h=0,u=0,d=0,p=0,g=0,_=0,m=null,f=e.reducedMotion,v=1,S=1,b=[],M=!1,E=null,P=null,x=null,C=new AbortController,w=new Set,I=new Map,y=new Map,N=new Set,D=new Set,R=new Set,U=[],z=new uc({antialias:!0,alpha:!1});z.toneMapping=Os,z.toneMappingExposure=.98,z.shadowMap.type=Ns,z.shadowMap.autoUpdate=!1,z.info.autoReset=!1;let T=z.domElement;T.className="sysworld-gl",T.tabIndex=0,T.setAttribute("aria-label",e.label),s.append(T);let G=new Cs;G.background=new Le(528926),G.fog=new da(528926,.004);let X=new sn(43,1,.3,1800);X.position.copy(Hu);let Z=Ec(),H=Z.register("visitor",{circles:[{x:0,z:0,r:.38}],minY:-.4,maxY:.4,reach:.38},X.position,100),oe=new bc(X,T);oe.target.copy(Kr),oe.enableDamping=!0,oe.dampingFactor=.085,oe.minDistance=12,oe.maxDistance=410,oe.maxPolarAngle=Math.PI*.485,oe.update();let ke=0,Fe=()=>{let L=We?.progress();e.onProgress?.(p+(L?.bytes||0),ke+(L?.expected||0))},We=Zp(z,{url:L=>e.resourceURL("/3d/system-world/textures/v1/"+L),onProgress:()=>Fe(),shelters:_n.map(L=>[L.x-L.width/2,L.z-L.depth/2,L.x+L.width/2,L.z+L.depth/2])}),fe=new kr(z),re=new xc,Pe=fe.fromScene(re,.05);G.environment=Pe.texture,G.environmentIntensity=.48,re.dispose(),fe.dispose();let be=new Ra(12639487,1056813,.8);G.add(be);let ge=new Gi(13164287,3.2);ge.position.set(-70,145,85),ge.castShadow=!0,Object.assign(ge.shadow.camera,{left:-108,right:108,top:108,bottom:-108,near:1,far:360}),ge.shadow.normalBias=.09,ge.shadow.bias=-15e-5,ge.shadow.radius=2.4,G.add(ge);let ie=new Gi(16758130,2.4);ie.position.set(90,80,-120),G.add(ie);let le=new Gi(14195306,.42);le.position.set(-10,-40,30),G.add(le);let se=new ri(6479871,0,75,2);se.position.set(0,34,-12),G.add(se);let ce=new Kt(1,1,{type:rn}),ve=new _c(z,ce);ve.addPass(new Gu(G,X,Math.min(4,z.capabilities.maxSamples)));let Ee=new qr(new Be(1,1),.28,.68,1.06);ve.addPass(Ee),ve.addPass(new yc);let qe=new gt,Ye=new gt,st=new gt;G.add(qe),qe.add(Ye,st);function at(L,K){return N.add(L),D.add(K),new it(L,K)}let ot=at(new xi(170,3,174),new Rn({color:1911345,metalness:.05,roughness:.82}));ot.position.set(0,-1.5,-7),ot.receiveShadow=!0,qe.add(ot),We.apply(ot.material,"world","ground");let J=wp(G,{sunDirection:ge.position,lamps:qs.filter(L=>L.asset==="street-lamp")});ve.addPass(J.post);let dt=Tp(G,{anchors:un.map(L=>({id:L.id,x:L.x,z:L.z,height:L.height}))}),ct=un.find(L=>L.id==="memory"),O=mp(G,ct,{roof:21,label:e.memoryLabel}),A=Rp(G,{traffic:Z,surfaces:We}),ne=jp(G,{sun:ge,rim:ie,hemisphere:be,atmosphere:J,surfaces:We,onThunder:L=>e.onThunder?.(L)}),de="",he=null,Ue={value:0},Oe=new os(1,1.045,96);N.add(Oe);let V=(L,K)=>{let $=new vt({transparent:!0,depthWrite:!1,side:Ot,uniforms:{color:{value:new Le(9365742)},time:Ue,opacity:{value:L},dashes:{value:K}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float time,opacity,dashes;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;float dash=dashes>0.5?step(0.3,fract(a*dashes-time*0.35)):1.0;gl_FragColor=vec4(color,opacity*dash*(0.8+0.2*sin(time*2.2)));}"});return D.add($),$},j=new it(Oe,V(.9,18)),me=new it(Oe,V(.38,0));for(let L of[j,me])L.rotation.x=-Math.PI/2,L.visible=!1,qe.add(L);let _e=new vt({transparent:!0,depthWrite:!1,side:Ot,uniforms:{color:{value:new Le(9365742)},alpha:{value:0}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float alpha;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;gl_FragColor=vec4(color,alpha*(0.55+0.45*sin(a*3.0)));}"});D.add(_e);let pe=new it(new os(.9,1,96),_e);N.add(pe.geometry),pe.rotation.x=-Math.PI/2,pe.visible=!1,qe.add(pe);let xe=2,Se={error:16742504,running:7730385,stale:8096667,idle:8175587},we={},Ie=()=>j.material.uniforms.color.value.setHex(Se[we[m]]??9365742),q=new Lt({color:6742230,toneMapped:!1}),Te=new An(new vi(.65,10,6),q,un.length);N.add(Te.geometry),D.add(q),qe.add(Te);let Me=new Ft,ue=new k,Re=new Fa,ye=new Be;un.forEach((L,K)=>{Me.position.set(L.x,L.height+2,L.z),Me.updateMatrix(),Te.setMatrixAt(K,Me.matrix),Te.setColorAt(K,new Le(8425630))}),e.signal?.addEventListener("abort",te,{once:!0});let je;try{if(e.signal?.aborted)throw Error("Disposed");let L=await fetch(e.assetURL("manifest.json"),{signal:C.signal});if(!L.ok)throw Error("City manifest unavailable");je=await L.json()}catch(L){throw te(),L}let Ke=new Map(je.assets.map(L=>[L.id,L]));Yp(Z,Ke,un,qs),he=qp(G,{camera:X,districts:un,tier:a,traffic:Z,reduced:()=>f,active:()=>n&&!M&&i!=="map"&&!f&&!e.replaying?.(),assetURL:L=>e.resourceURL("/3d/system-world/v2/"+L),surfaces:We,clear:(L,K)=>Z.clear(H,{...L,heading:0},{...K,heading:0},!1),onFlight:L=>e.onFlight?.(L),onInteraction:e.onInteraction,onScope:L=>e.onScope?.(L),onDiscover:e.onDiscover,onSound:e.onSound,onGait:e.onGait,onTerminal:e.onTerminal,onSociety:e.onSociety,onEnvironment:L=>{ne.setIndoor(L),e.onEnvironment?.(L)},onError:e.onError,onReady:()=>{z.shadowMap.needsUpdate=!0}});async function B(L,K){let $=L+":"+K;return I.has($)||I.set($,(async()=>{let De=Ke.get(L)?.lods.find(Ge=>Ge.level===K);if(!De||!/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(De.file))throw Error("Invalid city asset");let ze=new AbortController;w.add(ze);let Ne=setTimeout(()=>ze.abort(),15e3);try{let Ge=await fetch(e.assetURL(De.file),{signal:ze.signal});if(!Ge.ok)throw Error("City asset unavailable");let He=await Ge.arrayBuffer();if(t)throw Error("Disposed");let tt=await new Ti().parseAsync(He,"");if(t)throw tt.scene.traverse(Xe=>{Xe.isMesh&&(Xe.geometry.dispose(),Xe.material.dispose())}),Error("Disposed");return p+=He.byteLength,Fe(),tt.scene.traverse(Xe=>{if(!Xe.isMesh)return;N.add(Xe.geometry),Xe.castShadow=!0,Xe.receiveShadow=!0;let et=(Array.isArray(Xe.material)?Xe.material:[Xe.material]).map(Qe=>{if(y.has(Qe.name)){let ft=Qe;Qe=y.get(Qe.name),ft.dispose()}else y.set(Qe.name,Qe),D.add(Qe);return Qe});Xe.material=Array.isArray(Xe.material)?et:et[0],We.prepare(Xe)}),tt.scene}finally{clearTimeout(Ne),w.delete(ze)}})().catch(De=>{throw I.delete($),De})),I.get($)}function W(L){L.traverse(K=>{K.isInstancedMesh&&K.dispose()}),L.clear()}async function ee(){let L=++o,K=Pc[a],$=K.lod,De=[...new Set([...qs.map(Ne=>Ne.asset),...un.map(Ne=>Ne.asset),"service-drone"])],ze=(Ne,Ge)=>Ke.get(Ne)?.lods.find(He=>He.level===Ge)?.bytes||0;ke=De.reduce((Ne,Ge)=>Ne+(I.has(Ge+":"+$)?0:ze(Ge,$)),p)+($===2?0:["data-tower-a","data-tower-b"].reduce((Ne,Ge)=>Ne+(I.has(Ge+":2")?0:ze(Ge,2)),0));try{let Ne=new Map(await Promise.all(De.map(async Ce=>[Ce,await B(Ce,$)]))),Ge=new Map(await Promise.all(["data-tower-a","data-tower-b"].map(async Ce=>[Ce,await B(Ce,2)])));if(t||L!==o)return;W(Ye),W(st),b=[];let He=new Map;for(let Ce of qs){let et=(Ce.z<-100?Ge:Ne).get(Ce.asset);et.updateMatrixWorld(!0);let Qe=new pt().compose(new k(Ce.x,Ce.y,Ce.z),new Zt().setFromAxisAngle(new k(0,1,0),Ce.angle),new k(...Ce.scale));et.traverse(ft=>{if(!ft.isMesh)return;let Ct=ft.uuid;He.has(Ct)||He.set(Ct,{node:ft,matrices:[]}),He.get(Ct).matrices.push(new pt().multiplyMatrices(Qe,ft.matrixWorld))})}for(let{node:Ce,matrices:et}of He.values()){let Qe=new An(Ce.geometry,Ce.material,et.length);et.forEach((ft,Ct)=>Qe.setMatrixAt(Ct,ft)),Qe.castShadow=!0,Qe.receiveShadow=!0,Ye.add(Qe)}for(let Ce of un){let et=Ne.get(Ce.asset).clone(!0);et.position.set(Ce.x,0,Ce.z),et.userData.district=Ce.id,st.add(et),b.push(et)}P?.attachLandmarks(st),A.setTemplate(Ne.get("service-drone"));let tt=new en,Xe=new Map;for(let Ce of["data-tower-a","data-tower-b"])Xe.set(Ce,tt.setFromObject(Ge.get(Ce)).max.y);J.setAviation(qs.filter(Ce=>Xe.has(Ce.asset)&&(Ce.z<-100||Ce.scale[1]>1.1)).map(Ce=>({x:Ce.x,y:Ce.y+Xe.get(Ce.asset)*Ce.scale[1]+.6,z:Ce.z}))),z.shadowMap.needsUpdate=!0,e.onReady?.()}catch(Ne){!t&&L===o&&e.onError?.(Ne)}}function ae(){if(t)return;v=Math.max(1,s.clientWidth),S=Math.max(1,s.clientHeight);let L=Math.min(devicePixelRatio||1,Pc[a].dpr,Math.sqrt(3840*2160/(v*S)));z.setPixelRatio(L),z.setSize(v,S,!1),ve.setPixelRatio(L),ve.setSize(v,S),X.aspect=v/S,X.updateProjectionMatrix(),O.setPointScale(S*L),J.setPointScale(S*L),dt.setPointScale(S*L)}function Y(L){return r=L in Pc||L==="auto"?L:"auto",a=r==="auto"?"high":r,Q(),ee()}function Q(){let L=Pc[a];z.shadowMap.enabled=L.shadow>0,L.shadow&&ge.shadow.mapSize.x!==L.shadow&&(ge.shadow.mapSize.set(L.shadow,L.shadow),ge.shadow.map?.dispose(),ge.shadow.map=null),z.shadowMap.needsUpdate=!0,Ee.enabled=L.bloom,We.setTier(a),J.setTier(a),he.setTier(a),ne.setTier(a),A.setTier(a),dt.setTier(a),ae(),e.onQuality?.(r,a)}function Ae(L,K){l=null;let $=Z.findFree(H,L);if(!$)return;if(L=new k($.x,$.y,$.z),f){X.position.copy(L),Object.assign(H,$),oe.target.copy(K),oe.update(),l=null;return}let De=(He,tt)=>Z.clear(H,{...He,heading:0},{...tt,heading:0},!1),ze=X.position.clone(),Ne=Math.max(ze.y,L.y,110),Ge=[ze,L];if(!De(ze,L)){let He=[ze],tt=new k(L.x,Ne,L.z),Xe=[-1],Ce=[0],et=-1;if(!De(tt,L))return;for(let Qe of[3,8,16])for(let ft=0;ft<8;ft++)He.push(new k(ze.x+Math.cos(ft*Math.PI/4)*Qe,ze.y,ze.z+Math.sin(ft*Math.PI/4)*Qe));for(let Qe of _n)if(Math.hypot(ze.x-Qe.x,ze.z-Qe.z)<24)for(let ft of[Qe.z,Qe.doorZ-Qe.front*2,Qe.doorZ+Qe.front*3])He.push(new k(Qe.x,ze.y,ft));for(;Ce.length;){let Qe=Ce.shift(),ft=He[Qe],Ct=new k(ft.x,Ne,ft.z);if(De(ft,Ct)&&De(Ct,tt)){et=Qe;break}for(let Wt=1;Wt<He.length;Wt++)Xe[Wt]===void 0&&ft.distanceTo(He[Wt])<=20&&De(ft,He[Wt])&&(Xe[Wt]=Qe,Ce.push(Wt))}if(et<0)return;Ge=[];for(let Qe=et;Qe>=0;Qe=Xe[Qe])Ge.unshift(He[Qe]);Ge.push(new k(He[et].x,Ne,He[et].z),tt,L)}l={start:X.position.clone(),targetStart:oe.target.clone(),end:L.clone(),target:K.clone(),time:0,waypoints:Ge}}function Ve(){Z.relocate(H,X.position),X.position.set(H.x,H.y,H.z)}function rt(L){let K=un.find($=>$.id===L);K&&(m=L,j.position.set(K.x,.55,K.z),j.scale.setScalar(K.radius*1.3),j.visible=!0,Ie(),pe.position.set(K.x,.62,K.z),pe.scale.setScalar(K.radius*1.3*.35),_e.uniforms.color.value.setHex(Se[we[L]]??9365742),_e.uniforms.alpha.value=0,xe=0,pe.visible=!f,me.visible=!!Gt&&Gt!==m,i==="street"?(he.endRide(),e.onCut?.(),X.position.set(K.x,2.4,K.z+K.radius+7),X.lookAt(K.x,K.height*.4,K.z),Ve()):Ae(new k(K.x+K.radius*2.7,K.height*.7+18,K.z+K.radius*4),new k(K.x,K.height*.4,K.z)))}function Ze(L){he.endRide(),he.society.suspend(),R.clear(),l=null,i=L,oe.enabled=i==="orbit"||i==="tour",H.minY=i==="street"?-2.3:-.4,H.ignore=H.follow=null,H.circles[0].r=H.reach=i==="street"?.24:.38,document.pointerLockElement===T&&document.exitPointerLock(),i==="street"?(e.onCut?.(),yn(),X.position.set(18,2.4,57),X.lookAt(0,26,-12),Ve()):i!=="map"&&(Ve(),Ae(Hu.clone().multiplyScalar(X.aspect<1?1.3:1),Kr)),g=0,_=0,e.onMode?.(i)}let Ht=()=>{l=null,i==="tour"&&(i="orbit",e.onMode?.(i))};oe.addEventListener("start",Ht);function Et(L,K,$,De){L.addEventListener(K,$,De),U.push(()=>L.removeEventListener(K,$,De))}let lt=null,an=!1,Rt=new Tn(0,0,0,"YXZ");function Yt(L,K){let $=.0025*X.fov/43;Rt.setFromQuaternion(X.quaternion),Rt.y-=L*$,Rt.x=Tt.clamp(Rt.x-K*$,-1.35,1.35),X.quaternion.setFromEuler(Rt)}Et(T,"pointerdown",L=>{T.focus({preventScroll:!0}),Ht(),lt=[L.clientX,L.clientY],an=!1,i==="street"&&T.setPointerCapture(L.pointerId)});let Gt=null,mn=-1e9;function ji(L){if(L===Gt)return;Gt=L,T.style.cursor=L?"pointer":"";let K=un.find($=>$.id===L);me.visible=!!K&&L!==m,K&&(me.position.set(K.x,.56,K.z),me.scale.setScalar(K.radius*1.3)),e.onHover?.(L)}function yn(){ji(null)}function Xs(L){if(i==="street"||i==="map"||lt||L.timeStamp-mn<80)return;mn=L.timeStamp;let K=T.getBoundingClientRect();ye.set((L.clientX-K.left)/K.width*2-1,1-(L.clientY-K.top)/K.height*2),Re.setFromCamera(ye,X);let $=Re.intersectObjects(b,!0)[0]?.object;for(;$&&!$.userData.district;)$=$.parent;ji($?.userData.district||null)}Et(T,"pointerleave",yn),Et(T,"pointermove",L=>{i==="street"&&(document.pointerLockElement===T||lt)&&Yt(L.movementX,L.movementY),Xs(L),lt&&Math.hypot(L.clientX-lt[0],L.clientY-lt[1])>5&&(an=!0)}),Et(T,"pointerup",L=>{if(lt&&!an&&i!=="street"){let K=T.getBoundingClientRect();ye.set((L.clientX-K.left)/K.width*2-1,1-(L.clientY-K.top)/K.height*2),Re.setFromCamera(ye,X);let $=Re.intersectObjects(b,!0)[0];if($){let De=$.object;for(;De&&!De.userData.district;)De=De.parent;De&&e.onSelect?.(De.userData.district)}}lt=null}),Et(T,"pointercancel",()=>{lt=null,R.clear()}),Et(T,"keydown",L=>{if(L.key==="Escape"){he.scoping()?he.endRide():Ze("orbit"),L.preventDefault();return}if(i==="street"&&["+","=","-"].includes(L.key)&&he.zoom(L.key==="-"?240:-240)){L.preventDefault();return}if(i==="street"&&L.code==="KeyE"&&!L.repeat){he.interact(),L.preventDefault();return}i==="street"&&["KeyW","KeyA","KeyS","KeyD","ArrowUp","ArrowDown","ArrowLeft","ArrowRight","ShiftLeft"].includes(L.code)&&(R.add(L.code),L.preventDefault(),L.stopPropagation()),i==="street"&&["Space","KeyC"].includes(L.code)&&!L.ctrlKey&&!L.metaKey&&!L.altKey&&(R.add(L.code),L.preventDefault(),L.stopPropagation())}),Et(T,"keyup",L=>R.delete(L.code)),Et(T,"wheel",L=>{i==="street"&&he.zoom(L.deltaY)&&L.preventDefault()},{passive:!1}),Et(T,"blur",()=>R.clear()),Et(window,"blur",()=>{R.clear(),lt=null}),Et(T,"webglcontextlost",L=>{L.preventDefault(),M=!0,R.clear(),e.onContextLost?.()}),E=new ResizeObserver(ae),E.observe(s);function Yr(L){let K=(R.has("ShiftLeft")?25:11)*L,$=Number(R.has("KeyW")||R.has("ArrowUp"))-Number(R.has("KeyS")||R.has("ArrowDown")),De=Number(R.has("KeyD")||R.has("ArrowRight"))-Number(R.has("KeyA")||R.has("ArrowLeft"));if(he.hovering()){he.fly(L,{forward:$,right:De,up:Number(R.has("Space"))-Number(R.has("KeyC")),boost:R.has("ShiftLeft")});return}if(!$&&!De||he.walkRide($,L,De))return;let ze=K/Math.hypot($,De);X.getWorldDirection(ue),ue.y=0,ue.normalize();let Ne=(ue.x*$-ue.z*De)*ze,Ge=(ue.z*$+ue.x*De)*ze,He=(tt,Xe)=>he.move(tt,Xe,X.position)&&Z.clear(H,H,{x:tt,y:2.4+he.floor(tt,Xe),z:Xe,heading:0});He(X.position.x+Ne,X.position.z)&&(X.position.x+=Ne),He(X.position.x,X.position.z+Ge)&&(X.position.z+=Ge),he.isRiding()||(X.position.y=2.4+he.floor(X.position.x,X.position.z))}let oo=0,lo=hp(L=>{Z.begin(),oo+=f?0:L,i==="street"&&Yr(L),he.update(L,!f,i);let K=H.follow;H.follow=he.rideBody(),H.ignore=H.follow||K,P?.update(L,!f),A.update(L,oo,!f),Z.propose(H,{...X.position,heading:0},($,De)=>X.position.set(De.x,De.y,De.z)),Z.solve(L),he.syncRide?.(),H.follow&&Object.assign(H,{x:X.position.x,y:X.position.y,z:X.position.z}),H.ignore=H.follow});function co(L,K){if(t||!n||M||i==="map")return;let $=null,De=0,ze=null;if(i!=="street"){if(i==="tour"&&(g-=L,g<=0)){let Xe=un[_++%un.length].id;rt(Xe),e.onTourFocus?.(Xe),g=7}if(l){$=l,De=l.time,l.time+=L;let Xe=Math.min(1,l.time/(l.duration||(l.waypoints.length===2?1.1:3.2))),Ce=Xe*Xe*(3-2*Xe),et=Math.min(l.waypoints.length-1-1e-6,Ce*(l.waypoints.length-1)),Qe=Math.floor(et);X.position.lerpVectors(l.waypoints[Qe],l.waypoints[Qe+1],et-Qe),oe.target.lerpVectors(l.targetStart,l.target,Ce),X.lookAt(oe.target),ze=X.position.clone(),Xe===1&&(l=null)}else oe.update()}lo(Math.min(.1,Math.max(0,L)))||(Z.begin(),Z.propose(H,{...X.position,heading:0},(Xe,Ce)=>X.position.set(Ce.x,Ce.y,Ce.z)),Z.solve(0)),$&&(X.position.distanceToSquared(ze)>1e-6?($.wait=($.wait||0)+L,$.time=De,l=$.wait<6?$:null):$.wait=0),X.getWorldDirection(ue);let Ne=i!=="street"&&X.position.y>15;e.onListener?.(X.position.x,X.position.y,X.position.z,ue.x,ue.z,Ne?oe.target.x:X.position.x,Ne?oe.target.z:X.position.z);let Ge=!!e.busy?.();ne.update(L,K,!f)&&(z.shadowMap.needsUpdate=!0),se.intensity=f?0:Ge?180+Math.sin(K*2)*35:12,J.setBusy(Ge),J.setCinematic(i==="tour"),J.update(L,K,X,!f),We.update(L,!f),x?.update(L,!f&&n&&i!=="map"&&!e.replaying?.());let He=ne.mood();if(dt.update(L,!f,1-(He.day||0)),!f){if(xe<1.4){xe+=L;let Xe=Math.min(1,xe/1.4),Ce=un.find(et=>et.id===m);Ce&&(pe.scale.setScalar(Ce.radius*1.3*(.35+Xe*1.05)),_e.uniforms.alpha.value=(1-Xe)*.55),Xe>=1&&(pe.visible=!1)}for(let Xe=0;Xe<un.length;Xe++){let Ce=un[Xe];Me.position.set(Ce.x,Ce.height+2,Ce.z),Me.scale.setScalar(1+Math.sin(K*1.7+Xe*1.13)*.13),Me.updateMatrix(),Te.setMatrixAt(Xe,Me.matrix)}Te.instanceMatrix.needsUpdate=!0}f||(Ue.value+=Math.min(.1,Math.max(0,L)));let tt=[Ge,He.day.toFixed(1),He.evening.toFixed(1),He.weather].join();if(Ee.strength=.2+.15*(1-He.day),z.toneMappingExposure=.95+.05*(1-He.day),tt!==de&&(de=tt,e.onMood?.({busy:Ge,day:He.day,evening:He.evening,weather:He.weather})),O.update(L,X,!f),z.info.reset(),ve.render(),c++,r==="auto"&&L>0&&L<.2&&K-d>12&&(h+=L,u++,u>=180)){let Xe=h/u,Ce=["low","medium","high"],et=Ce.indexOf(a),Qe=Xe>.028&&et>0?Ce[et-1]:Xe<.017&&et<2?Ce[et+1]:a;u=0,h=0,Qe!==a&&(a=Qe,d=K,Q(),ee())}}function F(L,K){P?.setData(L,K),un.forEach(($,De)=>{let ze=L.find(Ne=>Ne.id===$.id);we[$.id]=!ze||ze.stale?"stale":ze.state==="error"?"error":ze.state==="running"?"running":"idle",Te.setColorAt(De,new Le(Se[we[$.id]]))}),Te.instanceColor.needsUpdate=!0,Ie()}function te(){t||(t=!0,o++,C.abort(),w.forEach(L=>L.abort()),R.clear(),document.pointerLockElement===T&&document.exitPointerLock(),e.signal?.removeEventListener("abort",te),U.forEach(L=>L()),E?.disconnect(),oe.dispose(),P?.dispose(),x?.dispose(),he?.dispose(),ne.dispose(),O.dispose(),A.dispose(),Z.dispose(),J.dispose(),We.dispose(),dt.dispose(),W(Ye),W(st),Te.dispose(),N.forEach(L=>L.dispose()),D.forEach(L=>L.dispose()),ve.passes.forEach(L=>L.dispose?.()),ve.dispose(),Pe.dispose(),ge.shadow.dispose(),z.dispose(),z.forceContextLoss(),T.remove(),I.clear(),y.clear())}P=dp(G,un,{traffic:Z,society:he.society,robotURL:e.resourceURL("/3d/system-world/white-robot.glb"),signal:e.signal,onError:e.onRobotError,active:()=>n&&!M&&i!=="map"&&!e.replaying?.(),obstacles:vy}),x=Kp({scene:G,resourceURL:e.resourceURL,signal:C.signal,groundY:xt.quay,onError:e.onCompanionError}),O.setReducedMotion(!!f);try{Q(),await ee()}catch(L){throw te(),L}return{districts:un,canvas:T,update:co,focus(L){Ht(),rt(L)},setMode:Ze,setQuality:Y,setData:F,dispose:te,interact(){he.interact()},socialAction:he.socialAction,visit(L){Ze("street"),he.visit(L),Ve()},enter(L){Ze("street"),he.destination(L),Ve()},setEnvironment(L){ne.set(L),z.shadowMap.needsUpdate=!0},setVisible(L){n=L,L||(lo(0),he.suspend(),P?.update(0,!1),R.clear(),lt=null,document.pointerLockElement===T&&document.exitPointerLock())},setReducedMotion(L){f=L,O.setReducedMotion(!!L),L&&(l=null,he.endRide(),he.society.suspend()),L&&i==="tour"&&Ze("orbit")},setHologram(L,K){O.setTexts(L,K),he.setMemory(L)},setWorld(L,K){he.setWorld(L,K)},moveKey(L,K){K?R.add(L):R.delete(L)},lockPointer(){if(i==="street")return T.requestPointerLock()},project(L){let K=un.find(De=>De.id===L);if(!K)return null;ue.set(K.x,K.height+4,K.z);let $=X.position.distanceTo(ue);return ue.project(X),{x:(ue.x+1)*v/2,y:(1-ue.y)*S/2,visible:ue.z<1&&ue.z>-1,distance:$}},capture(){return t||M?Promise.resolve(null):(ve.render(),new Promise(L=>T.toBlob(L,"image/png")))},intro(){if(f||i!=="orbit"||t)return;let L=Hu.clone().multiplyScalar(X.aspect<1?1.3:1);X.position.copy(L).sub(Kr).applyAxisAngle(new k(0,1,0),-.55).multiplyScalar(1.22).add(Kr),X.position.y+=34,X.lookAt(Kr),Ve(),Ae(L,Kr),l&&(l.duration=3.6)},stats(){return{flying:!!l,traffic:Z.stats(),experience:he.stats(),weather:ne.stats(),life:P?.stats(),hologram:O.stats(),atmosphere:J.stats(),surfaces:We.stats(),drones:A.stats(),flow:dt.stats(),companion:x?.stats(),frames:c,tier:a,mode:i,focusedDistrict:m,hovered:Gt,loadedBytes:p+he.stats().bytes+We.stats().bytes,cachedModels:I.size,calls:z.info.render.calls,triangles:z.info.render.triangles,geometries:z.info.memory.geometries,position:X.position.toArray(),renderer:z.getContext().getParameter(z.getContext().getExtension("WEBGL_debug_renderer_info")?.UNMASKED_RENDERER_WEBGL||z.getContext().RENDERER),disposed:t}}}}export{K1 as createCity,xy as createCityAmbience,un as districts,vy as obstacles,qs as placements};
/*! Bundled license information:

three/build/three.core.js:
three/build/three.module.js:
  (**
   * @license
   * Copyright 2010-2026 Three.js Authors
   * SPDX-License-Identifier: MIT
   *)
*/
