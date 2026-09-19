var Zi={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2},Ki={ROTATE:0,PAN:1,DOLLY_PAN:2,DOLLY_ROTATE:3},sf=0,qc=1,rf=2;var mo=1,Ja=2,fr=3,Sn=0,Wt=1,Bt=2,On=0,us=1,rn=2,Yc=3,Zc=4,of=5;var Vi=100,af=101,lf=102,cf=103,hf=104,uf=200,ff=201,df=202,pf=203,ma=204,ga=205,mf=206,gf=207,_f=208,xf=209,vf=210,yf=211,Mf=212,bf=213,Sf=214,_a=0,xa=1,va=2,fs=3,ya=4,Ma=5,ba=6,Sa=7,ja=0,Tf=1,Ef=2,qn=0,go=1,_o=2,xo=3,bs=4,vo=5,yo=6,Mo=7,Uc="attached",wf="detached",Kc=300,Ji=301,Ss=302,$a=303,Qa=304,bo=306,Gi=1e3,Dn=1001,qs=1002,Ot=1003,el=1004;var Ts=1005;var Lt=1006,dr=1007;var Yn=1008;var gn=1009,Jc=1010,jc=1011,pr=1012,tl=1013,Zn=1014,En=1015,Xt=1016,nl=1017,il=1018,mr=1020,$c=35902,Qc=35899,eh=1021,th=1022,wn=1023,ti=1026,ji=1027,sl=1028,rl=1029,$i=1030,ol=1031;var al=1033,So=33776,To=33777,Eo=33778,wo=33779,ll=35840,cl=35841,hl=35842,ul=35843,fl=36196,dl=37492,pl=37496,ml=37488,gl=37489,Ao=37490,_l=37491,xl=37808,vl=37809,yl=37810,Ml=37811,bl=37812,Sl=37813,Tl=37814,El=37815,wl=37816,Al=37817,Rl=37818,Cl=37819,Pl=37820,Il=37821,Ll=36492,Dl=36494,Nl=36495,Ul=36283,Fl=36284,Ro=36285,Ol=36286,Bl=2200,Af=2201,Rf=2202,ds=2300,ps=2301,pa=2302,Fc=2303,ls=2400,cs=2401,zr=2402,zl=2500,Cf=2501,nh=0,Co=1,gr=2,Pf=3200;var Po=0,If=1,Ci="",Pt="srgb",cn="srgb-linear",kr="linear",pt="srgb";var as=7680;var Oc=519,Lf=512,Df=513,Nf=514,kl=515,Uf=516,Ff=517,Hl=518,Of=519,Ta=35044;var ih="300 es",Gn=2e3,Ys=2001;function rp(s){for(let e=s.length-1;e>=0;--e)if(s[e]>=65535)return!0;return!1}function op(s){return ArrayBuffer.isView(s)&&!(s instanceof DataView)}function Zs(s){return document.createElementNS("http://www.w3.org/1999/xhtml",s)}function Bf(){let s=Zs("canvas");return s.style.display="block",s}var pu={},Ks=null;function Hr(...s){let e="THREE."+s.shift();Ks?Ks("log",e,...s):console.log(e,...s)}function zf(s){let e=s[0];if(typeof e=="string"&&e.startsWith("TSL:")){let t=s[1];t&&t.isStackTrace?s[0]+=" "+t.getLocation():s[1]='Stack trace not available. Enable "THREE.Node.captureStackTrace" to capture stack traces.'}return s}function Be(...s){s=zf(s);let e="THREE."+s.shift();if(Ks)Ks("warn",e,...s);else{let t=s[0];t&&t.isStackTrace?console.warn(t.getError(e)):console.warn(e,...s)}}function Ze(...s){s=zf(s);let e="THREE."+s.shift();if(Ks)Ks("error",e,...s);else{let t=s[0];t&&t.isStackTrace?console.error(t.getError(e)):console.error(e,...s)}}function hs(...s){let e=s.join(" ");e in pu||(pu[e]=!0,Be(...s))}function kf(s,e,t){return new Promise(function(n,i){function r(){switch(s.clientWaitSync(e,s.SYNC_FLUSH_COMMANDS_BIT,0)){case s.WAIT_FAILED:i();break;case s.TIMEOUT_EXPIRED:setTimeout(r,t);break;default:n()}}setTimeout(r,t)})}var Hf={[_a]:xa,[va]:ba,[ya]:Sa,[fs]:Ma,[xa]:_a,[ba]:va,[Sa]:ya,[Ma]:fs},Nn=class{addEventListener(e,t){this._listeners===void 0&&(this._listeners={});let n=this._listeners;n[e]===void 0&&(n[e]=[]),n[e].indexOf(t)===-1&&n[e].push(t)}hasEventListener(e,t){let n=this._listeners;return n===void 0?!1:n[e]!==void 0&&n[e].indexOf(t)!==-1}removeEventListener(e,t){let n=this._listeners;if(n===void 0)return;let i=n[e];if(i!==void 0){let r=i.indexOf(t);r!==-1&&i.splice(r,1)}}dispatchEvent(e){let t=this._listeners;if(t===void 0)return;let n=t[e.type];if(n!==void 0){e.target=this;let i=n.slice(0);for(let r=0,o=i.length;r<o;r++)i[r].call(this,e);e.target=null}}},en=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],mu=1234567,Ur=Math.PI/180,ms=180/Math.PI;function Wn(){let s=Math.random()*4294967295|0,e=Math.random()*4294967295|0,t=Math.random()*4294967295|0,n=Math.random()*4294967295|0;return(en[s&255]+en[s>>8&255]+en[s>>16&255]+en[s>>24&255]+"-"+en[e&255]+en[e>>8&255]+"-"+en[e>>16&15|64]+en[e>>24&255]+"-"+en[t&63|128]+en[t>>8&255]+"-"+en[t>>16&255]+en[t>>24&255]+en[n&255]+en[n>>8&255]+en[n>>16&255]+en[n>>24&255]).toLowerCase()}function tt(s,e,t){return Math.max(e,Math.min(t,s))}function sh(s,e){return(s%e+e)%e}function ap(s,e,t,n,i){return n+(s-e)*(i-n)/(t-e)}function lp(s,e,t){return s!==e?(t-s)/(e-s):0}function Fr(s,e,t){return(1-t)*s+t*e}function cp(s,e,t,n){return Fr(s,e,1-Math.exp(-t*n))}function hp(s,e=1){return e-Math.abs(sh(s,e*2)-e)}function up(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*(3-2*s))}function fp(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*s*(s*(s*6-15)+10))}function dp(s,e){return s+Math.floor(Math.random()*(e-s+1))}function pp(s,e){return s+Math.random()*(e-s)}function mp(s){return s*(.5-Math.random())}function gp(s){s!==void 0&&(mu=s);let e=mu+=1831565813;return e=Math.imul(e^e>>>15,e|1),e^=e+Math.imul(e^e>>>7,e|61),((e^e>>>14)>>>0)/4294967296}function _p(s){return s*Ur}function xp(s){return s*ms}function vp(s){return(s&s-1)===0&&s!==0}function yp(s){return Math.pow(2,Math.ceil(Math.log(s)/Math.LN2))}function Mp(s){return Math.pow(2,Math.floor(Math.log(s)/Math.LN2))}function bp(s,e,t,n,i){let r=Math.cos,o=Math.sin,a=r(t/2),l=o(t/2),c=r((e+n)/2),h=o((e+n)/2),u=r((e-n)/2),f=o((e-n)/2),d=r((n-e)/2),g=o((n-e)/2);switch(i){case"XYX":s.set(a*h,l*u,l*f,a*c);break;case"YZY":s.set(l*f,a*h,l*u,a*c);break;case"ZXZ":s.set(l*u,l*f,a*h,a*c);break;case"XZX":s.set(a*h,l*g,l*d,a*c);break;case"YXY":s.set(l*d,a*h,l*g,a*c);break;case"ZYZ":s.set(l*g,l*d,a*h,a*c);break;default:Be("MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+i)}}function Vn(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return s/4294967295;case Uint16Array:return s/65535;case Uint8Array:return s/255;case Int32Array:return Math.max(s/2147483647,-1);case Int16Array:return Math.max(s/32767,-1);case Int8Array:return Math.max(s/127,-1);default:throw new Error("THREE.MathUtils: Invalid component type.")}}function Mt(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return Math.round(s*4294967295);case Uint16Array:return Math.round(s*65535);case Uint8Array:return Math.round(s*255);case Int32Array:return Math.round(s*2147483647);case Int16Array:return Math.round(s*32767);case Int8Array:return Math.round(s*127);default:throw new Error("THREE.MathUtils: Invalid component type.")}}var Vt={DEG2RAD:Ur,RAD2DEG:ms,generateUUID:Wn,clamp:tt,euclideanModulo:sh,mapLinear:ap,inverseLerp:lp,lerp:Fr,damp:cp,pingpong:hp,smoothstep:up,smootherstep:fp,randInt:dp,randFloat:pp,randFloatSpread:mp,seededRandom:gp,degToRad:_p,radToDeg:xp,isPowerOfTwo:vp,ceilPowerOfTwo:yp,floorPowerOfTwo:Mp,setQuaternionFromProperEuler:bp,normalize:Mt,denormalize:Vn},ye=class s{static{s.prototype.isVector2=!0}constructor(e=0,t=0){this.x=e,this.y=t}get width(){return this.x}set width(e){this.x=e}get height(){return this.y}set height(e){this.y=e}set(e,t){return this.x=e,this.y=t,this}setScalar(e){return this.x=e,this.y=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;default:throw new Error("THREE.Vector2: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;default:throw new Error("THREE.Vector2: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y)}copy(e){return this.x=e.x,this.y=e.y,this}add(e){return this.x+=e.x,this.y+=e.y,this}addScalar(e){return this.x+=e,this.y+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this}subScalar(e){return this.x-=e,this.y-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this}multiply(e){return this.x*=e.x,this.y*=e.y,this}multiplyScalar(e){return this.x*=e,this.y*=e,this}divide(e){return this.x/=e.x,this.y/=e.y,this}divideScalar(e){return this.multiplyScalar(1/e)}applyMatrix3(e){let t=this.x,n=this.y,i=e.elements;return this.x=i[0]*t+i[3]*n+i[6],this.y=i[1]*t+i[4]*n+i[7],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this}clamp(e,t){return this.x=tt(this.x,e.x,t.x),this.y=tt(this.y,e.y,t.y),this}clampScalar(e,t){return this.x=tt(this.x,e,t),this.y=tt(this.y,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(tt(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(e){return this.x*e.x+this.y*e.y}cross(e){return this.x*e.y-this.y*e.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(tt(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y;return t*t+n*n}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this}equals(e){return e.x===this.x&&e.y===this.y}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this}rotateAround(e,t){let n=Math.cos(t),i=Math.sin(t),r=this.x-e.x,o=this.y-e.y;return this.x=r*n-o*i+e.x,this.y=r*i+o*n+e.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},It=class{constructor(e=0,t=0,n=0,i=1){this.isQuaternion=!0,this._x=e,this._y=t,this._z=n,this._w=i}static slerpFlat(e,t,n,i,r,o,a){let l=n[i+0],c=n[i+1],h=n[i+2],u=n[i+3],f=r[o+0],d=r[o+1],g=r[o+2],M=r[o+3];if(u!==M||l!==f||c!==d||h!==g){let m=l*f+c*d+h*g+u*M;m<0&&(f=-f,d=-d,g=-g,M=-M,m=-m);let p=1-a;if(m<.9995){let S=Math.acos(m),R=Math.sin(S);p=Math.sin(p*S)/R,a=Math.sin(a*S)/R,l=l*p+f*a,c=c*p+d*a,h=h*p+g*a,u=u*p+M*a}else{l=l*p+f*a,c=c*p+d*a,h=h*p+g*a,u=u*p+M*a;let S=1/Math.sqrt(l*l+c*c+h*h+u*u);l*=S,c*=S,h*=S,u*=S}}e[t]=l,e[t+1]=c,e[t+2]=h,e[t+3]=u}static multiplyQuaternionsFlat(e,t,n,i,r,o){let a=n[i],l=n[i+1],c=n[i+2],h=n[i+3],u=r[o],f=r[o+1],d=r[o+2],g=r[o+3];return e[t]=a*g+h*u+l*d-c*f,e[t+1]=l*g+h*f+c*u-a*d,e[t+2]=c*g+h*d+a*f-l*u,e[t+3]=h*g-a*u-l*f-c*d,e}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get w(){return this._w}set w(e){this._w=e,this._onChangeCallback()}set(e,t,n,i){return this._x=e,this._y=t,this._z=n,this._w=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(e){return this._x=e.x,this._y=e.y,this._z=e.z,this._w=e.w,this._onChangeCallback(),this}setFromEuler(e,t=!0){let n=e._x,i=e._y,r=e._z,o=e._order,a=Math.cos,l=Math.sin,c=a(n/2),h=a(i/2),u=a(r/2),f=l(n/2),d=l(i/2),g=l(r/2);switch(o){case"XYZ":this._x=f*h*u+c*d*g,this._y=c*d*u-f*h*g,this._z=c*h*g+f*d*u,this._w=c*h*u-f*d*g;break;case"YXZ":this._x=f*h*u+c*d*g,this._y=c*d*u-f*h*g,this._z=c*h*g-f*d*u,this._w=c*h*u+f*d*g;break;case"ZXY":this._x=f*h*u-c*d*g,this._y=c*d*u+f*h*g,this._z=c*h*g+f*d*u,this._w=c*h*u-f*d*g;break;case"ZYX":this._x=f*h*u-c*d*g,this._y=c*d*u+f*h*g,this._z=c*h*g-f*d*u,this._w=c*h*u+f*d*g;break;case"YZX":this._x=f*h*u+c*d*g,this._y=c*d*u+f*h*g,this._z=c*h*g-f*d*u,this._w=c*h*u-f*d*g;break;case"XZY":this._x=f*h*u-c*d*g,this._y=c*d*u-f*h*g,this._z=c*h*g+f*d*u,this._w=c*h*u+f*d*g;break;default:Be("Quaternion: .setFromEuler() encountered an unknown order: "+o)}return t===!0&&this._onChangeCallback(),this}setFromAxisAngle(e,t){let n=t/2,i=Math.sin(n);return this._x=e.x*i,this._y=e.y*i,this._z=e.z*i,this._w=Math.cos(n),this._onChangeCallback(),this}setFromRotationMatrix(e){let t=e.elements,n=t[0],i=t[4],r=t[8],o=t[1],a=t[5],l=t[9],c=t[2],h=t[6],u=t[10],f=n+a+u;if(f>0){let d=.5/Math.sqrt(f+1);this._w=.25/d,this._x=(h-l)*d,this._y=(r-c)*d,this._z=(o-i)*d}else if(n>a&&n>u){let d=2*Math.sqrt(1+n-a-u);this._w=(h-l)/d,this._x=.25*d,this._y=(i+o)/d,this._z=(r+c)/d}else if(a>u){let d=2*Math.sqrt(1+a-n-u);this._w=(r-c)/d,this._x=(i+o)/d,this._y=.25*d,this._z=(l+h)/d}else{let d=2*Math.sqrt(1+u-n-a);this._w=(o-i)/d,this._x=(r+c)/d,this._y=(l+h)/d,this._z=.25*d}return this._onChangeCallback(),this}setFromUnitVectors(e,t){let n=e.dot(t)+1;return n<1e-8?(n=0,Math.abs(e.x)>Math.abs(e.z)?(this._x=-e.y,this._y=e.x,this._z=0,this._w=n):(this._x=0,this._y=-e.z,this._z=e.y,this._w=n)):(this._x=e.y*t.z-e.z*t.y,this._y=e.z*t.x-e.x*t.z,this._z=e.x*t.y-e.y*t.x,this._w=n),this.normalize()}angleTo(e){return 2*Math.acos(Math.abs(tt(this.dot(e),-1,1)))}rotateTowards(e,t){let n=this.angleTo(e);if(n===0)return this;let i=Math.min(1,t/n);return this.slerp(e,i),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(e){return this._x*e._x+this._y*e._y+this._z*e._z+this._w*e._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let e=this.length();return e===0?(this._x=0,this._y=0,this._z=0,this._w=1):(e=1/e,this._x=this._x*e,this._y=this._y*e,this._z=this._z*e,this._w=this._w*e),this._onChangeCallback(),this}multiply(e){return this.multiplyQuaternions(this,e)}premultiply(e){return this.multiplyQuaternions(e,this)}multiplyQuaternions(e,t){let n=e._x,i=e._y,r=e._z,o=e._w,a=t._x,l=t._y,c=t._z,h=t._w;return this._x=n*h+o*a+i*c-r*l,this._y=i*h+o*l+r*a-n*c,this._z=r*h+o*c+n*l-i*a,this._w=o*h-n*a-i*l-r*c,this._onChangeCallback(),this}slerp(e,t){let n=e._x,i=e._y,r=e._z,o=e._w,a=this.dot(e);a<0&&(n=-n,i=-i,r=-r,o=-o,a=-a);let l=1-t;if(a<.9995){let c=Math.acos(a),h=Math.sin(c);l=Math.sin(l*c)/h,t=Math.sin(t*c)/h,this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+o*t,this._onChangeCallback()}else this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+o*t,this.normalize();return this}slerpQuaternions(e,t,n){return this.copy(e).slerp(t,n)}random(){let e=2*Math.PI*Math.random(),t=2*Math.PI*Math.random(),n=Math.random(),i=Math.sqrt(1-n),r=Math.sqrt(n);return this.set(i*Math.sin(e),i*Math.cos(e),r*Math.sin(t),r*Math.cos(t))}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._w===this._w}fromArray(e,t=0){return this._x=e[t],this._y=e[t+1],this._z=e[t+2],this._w=e[t+3],this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._w,e}fromBufferAttribute(e,t){return this._x=e.getX(t),this._y=e.getY(t),this._z=e.getZ(t),this._w=e.getW(t),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},D=class s{static{s.prototype.isVector3=!0}constructor(e=0,t=0,n=0){this.x=e,this.y=t,this.z=n}set(e,t,n){return n===void 0&&(n=this.z),this.x=e,this.y=t,this.z=n,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;default:throw new Error("THREE.Vector3: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("THREE.Vector3: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this}multiplyVectors(e,t){return this.x=e.x*t.x,this.y=e.y*t.y,this.z=e.z*t.z,this}applyEuler(e){return this.applyQuaternion(gu.setFromEuler(e))}applyAxisAngle(e,t){return this.applyQuaternion(gu.setFromAxisAngle(e,t))}applyMatrix3(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[3]*n+r[6]*i,this.y=r[1]*t+r[4]*n+r[7]*i,this.z=r[2]*t+r[5]*n+r[8]*i,this}applyNormalMatrix(e){return this.applyMatrix3(e).normalize()}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=e.elements,o=1/(r[3]*t+r[7]*n+r[11]*i+r[15]);return this.x=(r[0]*t+r[4]*n+r[8]*i+r[12])*o,this.y=(r[1]*t+r[5]*n+r[9]*i+r[13])*o,this.z=(r[2]*t+r[6]*n+r[10]*i+r[14])*o,this}applyQuaternion(e){let t=this.x,n=this.y,i=this.z,r=e.x,o=e.y,a=e.z,l=e.w,c=2*(o*i-a*n),h=2*(a*t-r*i),u=2*(r*n-o*t);return this.x=t+l*c+o*u-a*h,this.y=n+l*h+a*c-r*u,this.z=i+l*u+r*h-o*c,this}project(e){return this.applyMatrix4(e.matrixWorldInverse).applyMatrix4(e.projectionMatrix)}unproject(e){return this.applyMatrix4(e.projectionMatrixInverse).applyMatrix4(e.matrixWorld)}transformDirection(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[4]*n+r[8]*i,this.y=r[1]*t+r[5]*n+r[9]*i,this.z=r[2]*t+r[6]*n+r[10]*i,this.normalize()}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this}divideScalar(e){return this.multiplyScalar(1/e)}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this}clamp(e,t){return this.x=tt(this.x,e.x,t.x),this.y=tt(this.y,e.y,t.y),this.z=tt(this.z,e.z,t.z),this}clampScalar(e,t){return this.x=tt(this.x,e,t),this.y=tt(this.y,e,t),this.z=tt(this.z,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(tt(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this}cross(e){return this.crossVectors(this,e)}crossVectors(e,t){let n=e.x,i=e.y,r=e.z,o=t.x,a=t.y,l=t.z;return this.x=i*l-r*a,this.y=r*o-n*l,this.z=n*a-i*o,this}projectOnVector(e){let t=e.lengthSq();if(t===0)return this.set(0,0,0);let n=e.dot(this)/t;return this.copy(e).multiplyScalar(n)}projectOnPlane(e){return oc.copy(this).projectOnVector(e),this.sub(oc)}reflect(e){return this.sub(oc.copy(e).multiplyScalar(2*this.dot(e)))}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(tt(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y,i=this.z-e.z;return t*t+n*n+i*i}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)+Math.abs(this.z-e.z)}setFromSpherical(e){return this.setFromSphericalCoords(e.radius,e.phi,e.theta)}setFromSphericalCoords(e,t,n){let i=Math.sin(t)*e;return this.x=i*Math.sin(n),this.y=Math.cos(t)*e,this.z=i*Math.cos(n),this}setFromCylindrical(e){return this.setFromCylindricalCoords(e.radius,e.theta,e.y)}setFromCylindricalCoords(e,t,n){return this.x=e*Math.sin(t),this.y=n,this.z=e*Math.cos(t),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this}setFromMatrixScale(e){let t=this.setFromMatrixColumn(e,0).length(),n=this.setFromMatrixColumn(e,1).length(),i=this.setFromMatrixColumn(e,2).length();return this.x=t,this.y=n,this.z=i,this}setFromMatrixColumn(e,t){return this.fromArray(e.elements,t*4)}setFromMatrix3Column(e,t){return this.fromArray(e.elements,t*3)}setFromEuler(e){return this.x=e._x,this.y=e._y,this.z=e._z,this}setFromColor(e){return this.x=e.r,this.y=e.g,this.z=e.b,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let e=Math.random()*Math.PI*2,t=Math.random()*2-1,n=Math.sqrt(1-t*t);return this.x=n*Math.cos(e),this.y=t,this.z=n*Math.sin(e),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},oc=new D,gu=new It,$e=class s{static{s.prototype.isMatrix3=!0}constructor(e,t,n,i,r,o,a,l,c){this.elements=[1,0,0,0,1,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,o,a,l,c)}set(e,t,n,i,r,o,a,l,c){let h=this.elements;return h[0]=e,h[1]=i,h[2]=a,h[3]=t,h[4]=r,h[5]=l,h[6]=n,h[7]=o,h[8]=c,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],this}extractBasis(e,t,n){return e.setFromMatrix3Column(this,0),t.setFromMatrix3Column(this,1),n.setFromMatrix3Column(this,2),this}setFromMatrix4(e){let t=e.elements;return this.set(t[0],t[4],t[8],t[1],t[5],t[9],t[2],t[6],t[10]),this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,o=n[0],a=n[3],l=n[6],c=n[1],h=n[4],u=n[7],f=n[2],d=n[5],g=n[8],M=i[0],m=i[3],p=i[6],S=i[1],R=i[4],b=i[7],w=i[2],y=i[5],C=i[8];return r[0]=o*M+a*S+l*w,r[3]=o*m+a*R+l*y,r[6]=o*p+a*b+l*C,r[1]=c*M+h*S+u*w,r[4]=c*m+h*R+u*y,r[7]=c*p+h*b+u*C,r[2]=f*M+d*S+g*w,r[5]=f*m+d*R+g*y,r[8]=f*p+d*b+g*C,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[3]*=e,t[6]*=e,t[1]*=e,t[4]*=e,t[7]*=e,t[2]*=e,t[5]*=e,t[8]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8];return t*o*h-t*a*c-n*r*h+n*a*l+i*r*c-i*o*l}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8],u=h*o-a*c,f=a*l-h*r,d=c*r-o*l,g=t*u+n*f+i*d;if(g===0)return this.set(0,0,0,0,0,0,0,0,0);let M=1/g;return e[0]=u*M,e[1]=(i*c-h*n)*M,e[2]=(a*n-i*o)*M,e[3]=f*M,e[4]=(h*t-i*l)*M,e[5]=(i*r-a*t)*M,e[6]=d*M,e[7]=(n*l-c*t)*M,e[8]=(o*t-n*r)*M,this}transpose(){let e,t=this.elements;return e=t[1],t[1]=t[3],t[3]=e,e=t[2],t[2]=t[6],t[6]=e,e=t[5],t[5]=t[7],t[7]=e,this}getNormalMatrix(e){return this.setFromMatrix4(e).invert().transpose()}transposeIntoArray(e){let t=this.elements;return e[0]=t[0],e[1]=t[3],e[2]=t[6],e[3]=t[1],e[4]=t[4],e[5]=t[7],e[6]=t[2],e[7]=t[5],e[8]=t[8],this}setUvTransform(e,t,n,i,r,o,a){let l=Math.cos(r),c=Math.sin(r);return this.set(n*l,n*c,-n*(l*o+c*a)+o+e,-i*c,i*l,-i*(-c*o+l*a)+a+t,0,0,1),this}scale(e,t){return hs("Matrix3: .scale() is deprecated. Use .makeScale() instead."),this.premultiply(ac.makeScale(e,t)),this}rotate(e){return hs("Matrix3: .rotate() is deprecated. Use .makeRotation() instead."),this.premultiply(ac.makeRotation(-e)),this}translate(e,t){return hs("Matrix3: .translate() is deprecated. Use .makeTranslation() instead."),this.premultiply(ac.makeTranslation(e,t)),this}makeTranslation(e,t){return e.isVector2?this.set(1,0,e.x,0,1,e.y,0,0,1):this.set(1,0,e,0,1,t,0,0,1),this}makeRotation(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,n,t,0,0,0,1),this}makeScale(e,t){return this.set(e,0,0,0,t,0,0,0,1),this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<9;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<9;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e}clone(){return new this.constructor().fromArray(this.elements)}},ac=new $e,_u=new $e().set(.4123908,.3575843,.1804808,.212639,.7151687,.0721923,.0193308,.1191948,.9505322),xu=new $e().set(3.2409699,-1.5373832,-.4986108,-.9692436,1.8759675,.0415551,.0556301,-.203977,1.0569715);function Sp(){let s={enabled:!0,workingColorSpace:cn,spaces:{},convert:function(i,r,o){return this.enabled===!1||r===o||!r||!o||(this.spaces[r].transfer===pt&&(i.r=yi(i.r),i.g=yi(i.g),i.b=yi(i.b)),this.spaces[r].primaries!==this.spaces[o].primaries&&(i.applyMatrix3(this.spaces[r].toXYZ),i.applyMatrix3(this.spaces[o].fromXYZ)),this.spaces[o].transfer===pt&&(i.r=Xs(i.r),i.g=Xs(i.g),i.b=Xs(i.b))),i},workingToColorSpace:function(i,r){return this.convert(i,this.workingColorSpace,r)},colorSpaceToWorking:function(i,r){return this.convert(i,r,this.workingColorSpace)},getPrimaries:function(i){return this.spaces[i].primaries},getTransfer:function(i){return i===Ci?kr:this.spaces[i].transfer},getToneMappingMode:function(i){return this.spaces[i].outputColorSpaceConfig.toneMappingMode||"standard"},getLuminanceCoefficients:function(i,r=this.workingColorSpace){return i.fromArray(this.spaces[r].luminanceCoefficients)},define:function(i){Object.assign(this.spaces,i)},_getMatrix:function(i,r,o){return i.copy(this.spaces[r].toXYZ).multiply(this.spaces[o].fromXYZ)},_getDrawingBufferColorSpace:function(i){return this.spaces[i].outputColorSpaceConfig.drawingBufferColorSpace},_getUnpackColorSpace:function(i=this.workingColorSpace){return this.spaces[i].workingColorSpaceConfig.unpackColorSpace},fromWorkingColorSpace:function(i,r){return hs("ColorManagement: .fromWorkingColorSpace() has been renamed to .workingToColorSpace()."),s.workingToColorSpace(i,r)},toWorkingColorSpace:function(i,r){return hs("ColorManagement: .toWorkingColorSpace() has been renamed to .colorSpaceToWorking()."),s.colorSpaceToWorking(i,r)}},e=[.64,.33,.3,.6,.15,.06],t=[.2126,.7152,.0722],n=[.3127,.329];return s.define({[cn]:{primaries:e,whitePoint:n,transfer:kr,toXYZ:_u,fromXYZ:xu,luminanceCoefficients:t,workingColorSpaceConfig:{unpackColorSpace:Pt},outputColorSpaceConfig:{drawingBufferColorSpace:Pt}},[Pt]:{primaries:e,whitePoint:n,transfer:pt,toXYZ:_u,fromXYZ:xu,luminanceCoefficients:t,outputColorSpaceConfig:{drawingBufferColorSpace:Pt}}}),s}var et=Sp();function yi(s){return s<.04045?s*.0773993808:Math.pow(s*.9478672986+.0521327014,2.4)}function Xs(s){return s<.0031308?s*12.92:1.055*Math.pow(s,.41666)-.055}var Is,Ea=class{static getDataURL(e,t="image/png"){if(/^data:/i.test(e.src)||typeof HTMLCanvasElement>"u")return e.src;let n;if(e instanceof HTMLCanvasElement)n=e;else{Is===void 0&&(Is=Zs("canvas")),Is.width=e.width,Is.height=e.height;let i=Is.getContext("2d");e instanceof ImageData?i.putImageData(e,0,0):i.drawImage(e,0,0,e.width,e.height),n=Is}return n.toDataURL(t)}static sRGBToLinear(e){if(typeof HTMLImageElement<"u"&&e instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&e instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&e instanceof ImageBitmap){let t=Zs("canvas");t.width=e.width,t.height=e.height;let n=t.getContext("2d");n.drawImage(e,0,0,e.width,e.height);let i=n.getImageData(0,0,e.width,e.height),r=i.data;for(let o=0;o<r.length;o++)r[o]=yi(r[o]/255)*255;return n.putImageData(i,0,0),t}else if(e.data){let t=e.data.slice(0);for(let n=0;n<t.length;n++)t instanceof Uint8Array||t instanceof Uint8ClampedArray?t[n]=Math.floor(yi(t[n]/255)*255):t[n]=yi(t[n]);return{data:t,width:e.width,height:e.height}}else return Be("ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),e}},Tp=0,Js=class{constructor(e=null){this.isSource=!0,Object.defineProperty(this,"id",{value:Tp++}),this.uuid=Wn(),this.data=e,this.dataReady=!0,this.version=0}getSize(e){let t=this.data;return typeof HTMLVideoElement<"u"&&t instanceof HTMLVideoElement?e.set(t.videoWidth,t.videoHeight,0):typeof VideoFrame<"u"&&t instanceof VideoFrame?e.set(t.displayWidth,t.displayHeight,0):t!==null?e.set(t.width,t.height,t.depth||0):e.set(0,0,0),e}set needsUpdate(e){e===!0&&this.version++}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.images[this.uuid]!==void 0)return e.images[this.uuid];let n={uuid:this.uuid,url:""},i=this.data;if(i!==null){let r;if(Array.isArray(i)){r=[];for(let o=0,a=i.length;o<a;o++)i[o].isDataTexture?r.push(lc(i[o].image)):r.push(lc(i[o]))}else r=lc(i);n.url=r}return t||(e.images[this.uuid]=n),n}};function lc(s){return typeof HTMLImageElement<"u"&&s instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&s instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&s instanceof ImageBitmap?Ea.getDataURL(s):s.data?{data:Array.from(s.data),width:s.width,height:s.height,type:s.data.constructor.name}:(Be("Texture: Unable to serialize Texture."),{})}var Ep=0,cc=new D,Gt=class s extends Nn{constructor(e=s.DEFAULT_IMAGE,t=s.DEFAULT_MAPPING,n=Dn,i=Dn,r=Lt,o=Yn,a=wn,l=gn,c=s.DEFAULT_ANISOTROPY,h=Ci){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:Ep++}),this.uuid=Wn(),this.name="",this.source=new Js(e),this.mipmaps=[],this.mapping=t,this.channel=0,this.wrapS=n,this.wrapT=i,this.magFilter=r,this.minFilter=o,this.anisotropy=c,this.format=a,this.internalFormat=null,this.type=l,this.offset=new ye(0,0),this.repeat=new ye(1,1),this.center=new ye(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new $e,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=h,this.userData={},this.updateRanges=[],this.version=0,this.onUpdate=null,this.renderTarget=null,this.isRenderTargetTexture=!1,this.isArrayTexture=!!(e&&e.depth&&e.depth>1),this.pmremVersion=0,this.normalized=!1}get width(){return this.source.getSize(cc).x}get height(){return this.source.getSize(cc).y}get depth(){return this.source.getSize(cc).z}get image(){return this.source.data}set image(e){this.source.data=e}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}clone(){return new this.constructor().copy(this)}copy(e){return this.name=e.name,this.source=e.source,this.mipmaps=e.mipmaps.slice(0),this.mapping=e.mapping,this.channel=e.channel,this.wrapS=e.wrapS,this.wrapT=e.wrapT,this.magFilter=e.magFilter,this.minFilter=e.minFilter,this.anisotropy=e.anisotropy,this.format=e.format,this.internalFormat=e.internalFormat,this.type=e.type,this.normalized=e.normalized,this.offset.copy(e.offset),this.repeat.copy(e.repeat),this.center.copy(e.center),this.rotation=e.rotation,this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrix.copy(e.matrix),this.generateMipmaps=e.generateMipmaps,this.premultiplyAlpha=e.premultiplyAlpha,this.flipY=e.flipY,this.unpackAlignment=e.unpackAlignment,this.colorSpace=e.colorSpace,this.renderTarget=e.renderTarget,this.isRenderTargetTexture=e.isRenderTargetTexture,this.isArrayTexture=e.isArrayTexture,this.userData=JSON.parse(JSON.stringify(e.userData)),this.needsUpdate=!0,this}setValues(e){for(let t in e){let n=e[t];if(n===void 0){Be(`Texture.setValues(): parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){Be(`Texture.setValues(): property '${t}' does not exist.`);continue}i&&n&&i.isVector2&&n.isVector2||i&&n&&i.isVector3&&n.isVector3||i&&n&&i.isMatrix3&&n.isMatrix3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.textures[this.uuid]!==void 0)return e.textures[this.uuid];let n={metadata:{version:4.7,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(e).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,normalized:this.normalized,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(n.userData=this.userData),t||(e.textures[this.uuid]=n),n}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(e){if(this.mapping!==Kc)return e;if(e.applyMatrix3(this.matrix),e.x<0||e.x>1)switch(this.wrapS){case Gi:e.x=e.x-Math.floor(e.x);break;case Dn:e.x=e.x<0?0:1;break;case qs:Math.abs(Math.floor(e.x)%2)===1?e.x=Math.ceil(e.x)-e.x:e.x=e.x-Math.floor(e.x);break}if(e.y<0||e.y>1)switch(this.wrapT){case Gi:e.y=e.y-Math.floor(e.y);break;case Dn:e.y=e.y<0?0:1;break;case qs:Math.abs(Math.floor(e.y)%2)===1?e.y=Math.ceil(e.y)-e.y:e.y=e.y-Math.floor(e.y);break}return this.flipY&&(e.y=1-e.y),e}set needsUpdate(e){e===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(e){e===!0&&this.pmremVersion++}};Gt.DEFAULT_IMAGE=null;Gt.DEFAULT_MAPPING=Kc;Gt.DEFAULT_ANISOTROPY=1;var xt=class s{static{s.prototype.isVector4=!0}constructor(e=0,t=0,n=0,i=1){this.x=e,this.y=t,this.z=n,this.w=i}get width(){return this.z}set width(e){this.z=e}get height(){return this.w}set height(e){this.w=e}set(e,t,n,i){return this.x=e,this.y=t,this.z=n,this.w=i,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this.w=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setW(e){return this.w=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;case 3:this.w=t;break;default:throw new Error("THREE.Vector4: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("THREE.Vector4: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this.w=e.w!==void 0?e.w:1,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this.w+=e.w,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this.w+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this.w=e.w+t.w,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this.w+=e.w*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this.w-=e.w,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this.w-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this.w=e.w-t.w,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this.w*=e.w,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this.w*=e,this}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=this.w,o=e.elements;return this.x=o[0]*t+o[4]*n+o[8]*i+o[12]*r,this.y=o[1]*t+o[5]*n+o[9]*i+o[13]*r,this.z=o[2]*t+o[6]*n+o[10]*i+o[14]*r,this.w=o[3]*t+o[7]*n+o[11]*i+o[15]*r,this}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this.w/=e.w,this}divideScalar(e){return this.multiplyScalar(1/e)}setAxisAngleFromQuaternion(e){this.w=2*Math.acos(e.w);let t=Math.sqrt(1-e.w*e.w);return t<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=e.x/t,this.y=e.y/t,this.z=e.z/t),this}setAxisAngleFromRotationMatrix(e){let t,n,i,r,l=e.elements,c=l[0],h=l[4],u=l[8],f=l[1],d=l[5],g=l[9],M=l[2],m=l[6],p=l[10];if(Math.abs(h-f)<.01&&Math.abs(u-M)<.01&&Math.abs(g-m)<.01){if(Math.abs(h+f)<.1&&Math.abs(u+M)<.1&&Math.abs(g+m)<.1&&Math.abs(c+d+p-3)<.1)return this.set(1,0,0,0),this;t=Math.PI;let R=(c+1)/2,b=(d+1)/2,w=(p+1)/2,y=(h+f)/4,C=(u+M)/4,_=(g+m)/4;return R>b&&R>w?R<.01?(n=0,i=.707106781,r=.707106781):(n=Math.sqrt(R),i=y/n,r=C/n):b>w?b<.01?(n=.707106781,i=0,r=.707106781):(i=Math.sqrt(b),n=y/i,r=_/i):w<.01?(n=.707106781,i=.707106781,r=0):(r=Math.sqrt(w),n=C/r,i=_/r),this.set(n,i,r,t),this}let S=Math.sqrt((m-g)*(m-g)+(u-M)*(u-M)+(f-h)*(f-h));return Math.abs(S)<.001&&(S=1),this.x=(m-g)/S,this.y=(u-M)/S,this.z=(f-h)/S,this.w=Math.acos((c+d+p-1)/2),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this.w=t[15],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this.w=Math.min(this.w,e.w),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this.w=Math.max(this.w,e.w),this}clamp(e,t){return this.x=tt(this.x,e.x,t.x),this.y=tt(this.y,e.y,t.y),this.z=tt(this.z,e.z,t.z),this.w=tt(this.w,e.w,t.w),this}clampScalar(e,t){return this.x=tt(this.x,e,t),this.y=tt(this.y,e,t),this.z=tt(this.z,e,t),this.w=tt(this.w,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(tt(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z+this.w*e.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this.w+=(e.w-this.w)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this.w=e.w+(t.w-e.w)*n,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z&&e.w===this.w}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this.w=e[t+3],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e[t+3]=this.w,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this.w=e.getW(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},wa=class extends Nn{constructor(e=1,t=1,n={}){super(),n=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:Lt,depthBuffer:!0,stencilBuffer:!1,resolveDepthBuffer:!0,resolveStencilBuffer:!0,depthTexture:null,samples:0,count:1,depth:1,multiview:!1,useArrayDepthTexture:!1},n),this.isRenderTarget=!0,this.width=e,this.height=t,this.depth=n.depth,this.scissor=new xt(0,0,e,t),this.scissorTest=!1,this.viewport=new xt(0,0,e,t),this.textures=[];let i={width:e,height:t,depth:n.depth},r=new Gt(i),o=n.count;for(let a=0;a<o;a++)this.textures[a]=r.clone(),this.textures[a].isRenderTargetTexture=!0,this.textures[a].renderTarget=this;this._setTextureOptions(n),this.depthBuffer=n.depthBuffer,this.stencilBuffer=n.stencilBuffer,this.resolveDepthBuffer=n.resolveDepthBuffer,this.resolveStencilBuffer=n.resolveStencilBuffer,this._depthTexture=null,this.depthTexture=n.depthTexture,this.samples=n.samples,this.multiview=n.multiview,this.useArrayDepthTexture=n.useArrayDepthTexture}_setTextureOptions(e={}){let t={minFilter:Lt,generateMipmaps:!1,flipY:!1,internalFormat:null};e.mapping!==void 0&&(t.mapping=e.mapping),e.wrapS!==void 0&&(t.wrapS=e.wrapS),e.wrapT!==void 0&&(t.wrapT=e.wrapT),e.wrapR!==void 0&&(t.wrapR=e.wrapR),e.magFilter!==void 0&&(t.magFilter=e.magFilter),e.minFilter!==void 0&&(t.minFilter=e.minFilter),e.format!==void 0&&(t.format=e.format),e.type!==void 0&&(t.type=e.type),e.anisotropy!==void 0&&(t.anisotropy=e.anisotropy),e.colorSpace!==void 0&&(t.colorSpace=e.colorSpace),e.flipY!==void 0&&(t.flipY=e.flipY),e.generateMipmaps!==void 0&&(t.generateMipmaps=e.generateMipmaps),e.internalFormat!==void 0&&(t.internalFormat=e.internalFormat);for(let n=0;n<this.textures.length;n++)this.textures[n].setValues(t)}get texture(){return this.textures[0]}set texture(e){this.textures[0]=e}set depthTexture(e){this._depthTexture!==null&&(this._depthTexture.renderTarget=null),e!==null&&(e.renderTarget=this),this._depthTexture=e}get depthTexture(){return this._depthTexture}setSize(e,t,n=1){if(this.width!==e||this.height!==t||this.depth!==n){this.width=e,this.height=t,this.depth=n;for(let i=0,r=this.textures.length;i<r;i++)this.textures[i].image.width=e,this.textures[i].image.height=t,this.textures[i].image.depth=n,this.textures[i].isData3DTexture!==!0&&(this.textures[i].isArrayTexture=this.textures[i].image.depth>1);this.dispose()}this.viewport.set(0,0,e,t),this.scissor.set(0,0,e,t)}clone(){return new this.constructor().copy(this)}copy(e){this.width=e.width,this.height=e.height,this.depth=e.depth,this.scissor.copy(e.scissor),this.scissorTest=e.scissorTest,this.viewport.copy(e.viewport),this.textures.length=0;for(let t=0,n=e.textures.length;t<n;t++){this.textures[t]=e.textures[t].clone(),this.textures[t].isRenderTargetTexture=!0,this.textures[t].renderTarget=this;let i=Object.assign({},e.textures[t].image);this.textures[t].source=new Js(i)}return this.depthBuffer=e.depthBuffer,this.stencilBuffer=e.stencilBuffer,this.resolveDepthBuffer=e.resolveDepthBuffer,this.resolveStencilBuffer=e.resolveStencilBuffer,e.depthTexture!==null&&(this.depthTexture=e.depthTexture.clone()),this.samples=e.samples,this.multiview=e.multiview,this.useArrayDepthTexture=e.useArrayDepthTexture,this}dispose(){this.dispatchEvent({type:"dispose"})}},Nt=class extends wa{constructor(e=1,t=1,n={}){super(e,t,n),this.isWebGLRenderTarget=!0}},Vr=class extends Gt{constructor(e=null,t=1,n=1,i=1){super(null),this.isDataArrayTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=Ot,this.minFilter=Ot,this.wrapR=Dn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}addLayerUpdate(e){this.layerUpdates.add(e)}clearLayerUpdates(){this.layerUpdates.clear()}};var Aa=class extends Gt{constructor(e=null,t=1,n=1,i=1){super(null),this.isData3DTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=Ot,this.minFilter=Ot,this.wrapR=Dn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}};var Ke=class s{static{s.prototype.isMatrix4=!0}constructor(e,t,n,i,r,o,a,l,c,h,u,f,d,g,M,m){this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,o,a,l,c,h,u,f,d,g,M,m)}set(e,t,n,i,r,o,a,l,c,h,u,f,d,g,M,m){let p=this.elements;return p[0]=e,p[4]=t,p[8]=n,p[12]=i,p[1]=r,p[5]=o,p[9]=a,p[13]=l,p[2]=c,p[6]=h,p[10]=u,p[14]=f,p[3]=d,p[7]=g,p[11]=M,p[15]=m,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new s().fromArray(this.elements)}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],t[9]=n[9],t[10]=n[10],t[11]=n[11],t[12]=n[12],t[13]=n[13],t[14]=n[14],t[15]=n[15],this}copyPosition(e){let t=this.elements,n=e.elements;return t[12]=n[12],t[13]=n[13],t[14]=n[14],this}setFromMatrix3(e){let t=e.elements;return this.set(t[0],t[3],t[6],0,t[1],t[4],t[7],0,t[2],t[5],t[8],0,0,0,0,1),this}extractBasis(e,t,n){return this.determinantAffine()===0?(e.set(1,0,0),t.set(0,1,0),n.set(0,0,1),this):(e.setFromMatrixColumn(this,0),t.setFromMatrixColumn(this,1),n.setFromMatrixColumn(this,2),this)}makeBasis(e,t,n){return this.set(e.x,t.x,n.x,0,e.y,t.y,n.y,0,e.z,t.z,n.z,0,0,0,0,1),this}extractRotation(e){if(e.determinantAffine()===0)return this.identity();let t=this.elements,n=e.elements,i=1/Ls.setFromMatrixColumn(e,0).length(),r=1/Ls.setFromMatrixColumn(e,1).length(),o=1/Ls.setFromMatrixColumn(e,2).length();return t[0]=n[0]*i,t[1]=n[1]*i,t[2]=n[2]*i,t[3]=0,t[4]=n[4]*r,t[5]=n[5]*r,t[6]=n[6]*r,t[7]=0,t[8]=n[8]*o,t[9]=n[9]*o,t[10]=n[10]*o,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromEuler(e){let t=this.elements,n=e.x,i=e.y,r=e.z,o=Math.cos(n),a=Math.sin(n),l=Math.cos(i),c=Math.sin(i),h=Math.cos(r),u=Math.sin(r);if(e.order==="XYZ"){let f=o*h,d=o*u,g=a*h,M=a*u;t[0]=l*h,t[4]=-l*u,t[8]=c,t[1]=d+g*c,t[5]=f-M*c,t[9]=-a*l,t[2]=M-f*c,t[6]=g+d*c,t[10]=o*l}else if(e.order==="YXZ"){let f=l*h,d=l*u,g=c*h,M=c*u;t[0]=f+M*a,t[4]=g*a-d,t[8]=o*c,t[1]=o*u,t[5]=o*h,t[9]=-a,t[2]=d*a-g,t[6]=M+f*a,t[10]=o*l}else if(e.order==="ZXY"){let f=l*h,d=l*u,g=c*h,M=c*u;t[0]=f-M*a,t[4]=-o*u,t[8]=g+d*a,t[1]=d+g*a,t[5]=o*h,t[9]=M-f*a,t[2]=-o*c,t[6]=a,t[10]=o*l}else if(e.order==="ZYX"){let f=o*h,d=o*u,g=a*h,M=a*u;t[0]=l*h,t[4]=g*c-d,t[8]=f*c+M,t[1]=l*u,t[5]=M*c+f,t[9]=d*c-g,t[2]=-c,t[6]=a*l,t[10]=o*l}else if(e.order==="YZX"){let f=o*l,d=o*c,g=a*l,M=a*c;t[0]=l*h,t[4]=M-f*u,t[8]=g*u+d,t[1]=u,t[5]=o*h,t[9]=-a*h,t[2]=-c*h,t[6]=d*u+g,t[10]=f-M*u}else if(e.order==="XZY"){let f=o*l,d=o*c,g=a*l,M=a*c;t[0]=l*h,t[4]=-u,t[8]=c*h,t[1]=f*u+M,t[5]=o*h,t[9]=d*u-g,t[2]=g*u-d,t[6]=a*h,t[10]=M*u+f}return t[3]=0,t[7]=0,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromQuaternion(e){return this.compose(wp,e,Ap)}lookAt(e,t,n){let i=this.elements;return Mn.subVectors(e,t),Mn.lengthSq()===0&&(Mn.z=1),Mn.normalize(),Ui.crossVectors(n,Mn),Ui.lengthSq()===0&&(Math.abs(n.z)===1?Mn.x+=1e-4:Mn.z+=1e-4,Mn.normalize(),Ui.crossVectors(n,Mn)),Ui.normalize(),Vo.crossVectors(Mn,Ui),i[0]=Ui.x,i[4]=Vo.x,i[8]=Mn.x,i[1]=Ui.y,i[5]=Vo.y,i[9]=Mn.y,i[2]=Ui.z,i[6]=Vo.z,i[10]=Mn.z,this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,o=n[0],a=n[4],l=n[8],c=n[12],h=n[1],u=n[5],f=n[9],d=n[13],g=n[2],M=n[6],m=n[10],p=n[14],S=n[3],R=n[7],b=n[11],w=n[15],y=i[0],C=i[4],_=i[8],v=i[12],E=i[1],P=i[5],L=i[9],H=i[13],O=i[2],U=i[6],V=i[10],F=i[14],Y=i[3],J=i[7],j=i[11],me=i[15];return r[0]=o*y+a*E+l*O+c*Y,r[4]=o*C+a*P+l*U+c*J,r[8]=o*_+a*L+l*V+c*j,r[12]=o*v+a*H+l*F+c*me,r[1]=h*y+u*E+f*O+d*Y,r[5]=h*C+u*P+f*U+d*J,r[9]=h*_+u*L+f*V+d*j,r[13]=h*v+u*H+f*F+d*me,r[2]=g*y+M*E+m*O+p*Y,r[6]=g*C+M*P+m*U+p*J,r[10]=g*_+M*L+m*V+p*j,r[14]=g*v+M*H+m*F+p*me,r[3]=S*y+R*E+b*O+w*Y,r[7]=S*C+R*P+b*U+w*J,r[11]=S*_+R*L+b*V+w*j,r[15]=S*v+R*H+b*F+w*me,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[4]*=e,t[8]*=e,t[12]*=e,t[1]*=e,t[5]*=e,t[9]*=e,t[13]*=e,t[2]*=e,t[6]*=e,t[10]*=e,t[14]*=e,t[3]*=e,t[7]*=e,t[11]*=e,t[15]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[12],o=e[1],a=e[5],l=e[9],c=e[13],h=e[2],u=e[6],f=e[10],d=e[14],g=e[3],M=e[7],m=e[11],p=e[15],S=l*d-c*f,R=a*d-c*u,b=a*f-l*u,w=o*d-c*h,y=o*f-l*h,C=o*u-a*h;return t*(M*S-m*R+p*b)-n*(g*S-m*w+p*y)+i*(g*R-M*w+p*C)-r*(g*b-M*y+m*C)}determinantAffine(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[1],o=e[5],a=e[9],l=e[2],c=e[6],h=e[10];return t*(o*h-a*c)-n*(r*h-a*l)+i*(r*c-o*l)}transpose(){let e=this.elements,t;return t=e[1],e[1]=e[4],e[4]=t,t=e[2],e[2]=e[8],e[8]=t,t=e[6],e[6]=e[9],e[9]=t,t=e[3],e[3]=e[12],e[12]=t,t=e[7],e[7]=e[13],e[13]=t,t=e[11],e[11]=e[14],e[14]=t,this}setPosition(e,t,n){let i=this.elements;return e.isVector3?(i[12]=e.x,i[13]=e.y,i[14]=e.z):(i[12]=e,i[13]=t,i[14]=n),this}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8],u=e[9],f=e[10],d=e[11],g=e[12],M=e[13],m=e[14],p=e[15],S=t*a-n*o,R=t*l-i*o,b=t*c-r*o,w=n*l-i*a,y=n*c-r*a,C=i*c-r*l,_=h*M-u*g,v=h*m-f*g,E=h*p-d*g,P=u*m-f*M,L=u*p-d*M,H=f*p-d*m,O=S*H-R*L+b*P+w*E-y*v+C*_;if(O===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let U=1/O;return e[0]=(a*H-l*L+c*P)*U,e[1]=(i*L-n*H-r*P)*U,e[2]=(M*C-m*y+p*w)*U,e[3]=(f*y-u*C-d*w)*U,e[4]=(l*E-o*H-c*v)*U,e[5]=(t*H-i*E+r*v)*U,e[6]=(m*b-g*C-p*R)*U,e[7]=(h*C-f*b+d*R)*U,e[8]=(o*L-a*E+c*_)*U,e[9]=(n*E-t*L-r*_)*U,e[10]=(g*y-M*b+p*S)*U,e[11]=(u*b-h*y-d*S)*U,e[12]=(a*v-o*P-l*_)*U,e[13]=(t*P-n*v+i*_)*U,e[14]=(M*R-g*w-m*S)*U,e[15]=(h*w-u*R+f*S)*U,this}scale(e){let t=this.elements,n=e.x,i=e.y,r=e.z;return t[0]*=n,t[4]*=i,t[8]*=r,t[1]*=n,t[5]*=i,t[9]*=r,t[2]*=n,t[6]*=i,t[10]*=r,t[3]*=n,t[7]*=i,t[11]*=r,this}getMaxScaleOnAxis(){let e=this.elements,t=e[0]*e[0]+e[1]*e[1]+e[2]*e[2],n=e[4]*e[4]+e[5]*e[5]+e[6]*e[6],i=e[8]*e[8]+e[9]*e[9]+e[10]*e[10];return Math.sqrt(Math.max(t,n,i))}makeTranslation(e,t,n){return e.isVector3?this.set(1,0,0,e.x,0,1,0,e.y,0,0,1,e.z,0,0,0,1):this.set(1,0,0,e,0,1,0,t,0,0,1,n,0,0,0,1),this}makeRotationX(e){let t=Math.cos(e),n=Math.sin(e);return this.set(1,0,0,0,0,t,-n,0,0,n,t,0,0,0,0,1),this}makeRotationY(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,0,n,0,0,1,0,0,-n,0,t,0,0,0,0,1),this}makeRotationZ(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,0,n,t,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(e,t){let n=Math.cos(t),i=Math.sin(t),r=1-n,o=e.x,a=e.y,l=e.z,c=r*o,h=r*a;return this.set(c*o+n,c*a-i*l,c*l+i*a,0,c*a+i*l,h*a+n,h*l-i*o,0,c*l-i*a,h*l+i*o,r*l*l+n,0,0,0,0,1),this}makeScale(e,t,n){return this.set(e,0,0,0,0,t,0,0,0,0,n,0,0,0,0,1),this}makeShear(e,t,n,i,r,o){return this.set(1,n,r,0,e,1,o,0,t,i,1,0,0,0,0,1),this}compose(e,t,n){let i=this.elements,r=t._x,o=t._y,a=t._z,l=t._w,c=r+r,h=o+o,u=a+a,f=r*c,d=r*h,g=r*u,M=o*h,m=o*u,p=a*u,S=l*c,R=l*h,b=l*u,w=n.x,y=n.y,C=n.z;return i[0]=(1-(M+p))*w,i[1]=(d+b)*w,i[2]=(g-R)*w,i[3]=0,i[4]=(d-b)*y,i[5]=(1-(f+p))*y,i[6]=(m+S)*y,i[7]=0,i[8]=(g+R)*C,i[9]=(m-S)*C,i[10]=(1-(f+M))*C,i[11]=0,i[12]=e.x,i[13]=e.y,i[14]=e.z,i[15]=1,this}decompose(e,t,n){let i=this.elements;e.x=i[12],e.y=i[13],e.z=i[14];let r=this.determinantAffine();if(r===0)return n.set(1,1,1),t.identity(),this;let o=Ls.set(i[0],i[1],i[2]).length(),a=Ls.set(i[4],i[5],i[6]).length(),l=Ls.set(i[8],i[9],i[10]).length();r<0&&(o=-o),zn.copy(this);let c=1/o,h=1/a,u=1/l;return zn.elements[0]*=c,zn.elements[1]*=c,zn.elements[2]*=c,zn.elements[4]*=h,zn.elements[5]*=h,zn.elements[6]*=h,zn.elements[8]*=u,zn.elements[9]*=u,zn.elements[10]*=u,t.setFromRotationMatrix(zn),n.x=o,n.y=a,n.z=l,this}makePerspective(e,t,n,i,r,o,a=Gn,l=!1){let c=this.elements,h=2*r/(t-e),u=2*r/(n-i),f=(t+e)/(t-e),d=(n+i)/(n-i),g,M;if(l)g=r/(o-r),M=o*r/(o-r);else if(a===Gn)g=-(o+r)/(o-r),M=-2*o*r/(o-r);else if(a===Ys)g=-o/(o-r),M=-o*r/(o-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+a);return c[0]=h,c[4]=0,c[8]=f,c[12]=0,c[1]=0,c[5]=u,c[9]=d,c[13]=0,c[2]=0,c[6]=0,c[10]=g,c[14]=M,c[3]=0,c[7]=0,c[11]=-1,c[15]=0,this}makeOrthographic(e,t,n,i,r,o,a=Gn,l=!1){let c=this.elements,h=2/(t-e),u=2/(n-i),f=-(t+e)/(t-e),d=-(n+i)/(n-i),g,M;if(l)g=1/(o-r),M=o/(o-r);else if(a===Gn)g=-2/(o-r),M=-(o+r)/(o-r);else if(a===Ys)g=-1/(o-r),M=-r/(o-r);else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+a);return c[0]=h,c[4]=0,c[8]=0,c[12]=f,c[1]=0,c[5]=u,c[9]=0,c[13]=d,c[2]=0,c[6]=0,c[10]=g,c[14]=M,c[3]=0,c[7]=0,c[11]=0,c[15]=1,this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<16;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<16;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e[t+9]=n[9],e[t+10]=n[10],e[t+11]=n[11],e[t+12]=n[12],e[t+13]=n[13],e[t+14]=n[14],e[t+15]=n[15],e}},Ls=new D,zn=new Ke,wp=new D(0,0,0),Ap=new D(1,1,1),Ui=new D,Vo=new D,Mn=new D,vu=new Ke,yu=new It,Un=class s{constructor(e=0,t=0,n=0,i=s.DEFAULT_ORDER){this.isEuler=!0,this._x=e,this._y=t,this._z=n,this._order=i}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get order(){return this._order}set order(e){this._order=e,this._onChangeCallback()}set(e,t,n,i=this._order){return this._x=e,this._y=t,this._z=n,this._order=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(e){return this._x=e._x,this._y=e._y,this._z=e._z,this._order=e._order,this._onChangeCallback(),this}setFromRotationMatrix(e,t=this._order,n=!0){let i=e.elements,r=i[0],o=i[4],a=i[8],l=i[1],c=i[5],h=i[9],u=i[2],f=i[6],d=i[10];switch(t){case"XYZ":this._y=Math.asin(tt(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(-h,d),this._z=Math.atan2(-o,r)):(this._x=Math.atan2(f,c),this._z=0);break;case"YXZ":this._x=Math.asin(-tt(h,-1,1)),Math.abs(h)<.9999999?(this._y=Math.atan2(a,d),this._z=Math.atan2(l,c)):(this._y=Math.atan2(-u,r),this._z=0);break;case"ZXY":this._x=Math.asin(tt(f,-1,1)),Math.abs(f)<.9999999?(this._y=Math.atan2(-u,d),this._z=Math.atan2(-o,c)):(this._y=0,this._z=Math.atan2(l,r));break;case"ZYX":this._y=Math.asin(-tt(u,-1,1)),Math.abs(u)<.9999999?(this._x=Math.atan2(f,d),this._z=Math.atan2(l,r)):(this._x=0,this._z=Math.atan2(-o,c));break;case"YZX":this._z=Math.asin(tt(l,-1,1)),Math.abs(l)<.9999999?(this._x=Math.atan2(-h,c),this._y=Math.atan2(-u,r)):(this._x=0,this._y=Math.atan2(a,d));break;case"XZY":this._z=Math.asin(-tt(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(f,c),this._y=Math.atan2(a,r)):(this._x=Math.atan2(-h,d),this._y=0);break;default:Be("Euler: .setFromRotationMatrix() encountered an unknown order: "+t)}return this._order=t,n===!0&&this._onChangeCallback(),this}setFromQuaternion(e,t,n){return vu.makeRotationFromQuaternion(e),this.setFromRotationMatrix(vu,t,n)}setFromVector3(e,t=this._order){return this.set(e.x,e.y,e.z,t)}reorder(e){return yu.setFromEuler(this),this.setFromQuaternion(yu,e)}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._order===this._order}fromArray(e){return this._x=e[0],this._y=e[1],this._z=e[2],e[3]!==void 0&&(this._order=e[3]),this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._order,e}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};Un.DEFAULT_ORDER="XYZ";var js=class{constructor(){this.mask=1}set(e){this.mask=(1<<e|0)>>>0}enable(e){this.mask|=1<<e|0}enableAll(){this.mask=-1}toggle(e){this.mask^=1<<e|0}disable(e){this.mask&=~(1<<e|0)}disableAll(){this.mask=0}test(e){return(this.mask&e.mask)!==0}isEnabled(e){return(this.mask&(1<<e|0))!==0}},Rp=0,Mu=new D,Ds=new It,pi=new Ke,Go=new D,Ar=new D,Cp=new D,Pp=new It,bu=new D(1,0,0),Su=new D(0,1,0),Tu=new D(0,0,1),Eu={type:"added"},Ip={type:"removed"},Ns={type:"childadded",child:null},hc={type:"childremoved",child:null},yt=class s extends Nn{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:Rp++}),this.uuid=Wn(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=s.DEFAULT_UP.clone();let e=new D,t=new Un,n=new It,i=new D(1,1,1);function r(){n.setFromEuler(t,!1)}function o(){t.setFromQuaternion(n,void 0,!1)}t._onChange(r),n._onChange(o),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:e},rotation:{configurable:!0,enumerable:!0,value:t},quaternion:{configurable:!0,enumerable:!0,value:n},scale:{configurable:!0,enumerable:!0,value:i},modelViewMatrix:{value:new Ke},normalMatrix:{value:new $e}}),this.matrix=new Ke,this.matrixWorld=new Ke,this.matrixAutoUpdate=s.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=s.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new js,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.customDepthMaterial=void 0,this.customDistanceMaterial=void 0,this.static=!1,this.userData={},this.pivot=null}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(e){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(e),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(e){return this.quaternion.premultiply(e),this}setRotationFromAxisAngle(e,t){this.quaternion.setFromAxisAngle(e,t)}setRotationFromEuler(e){this.quaternion.setFromEuler(e,!0)}setRotationFromMatrix(e){this.quaternion.setFromRotationMatrix(e)}setRotationFromQuaternion(e){this.quaternion.copy(e)}rotateOnAxis(e,t){return Ds.setFromAxisAngle(e,t),this.quaternion.multiply(Ds),this}rotateOnWorldAxis(e,t){return Ds.setFromAxisAngle(e,t),this.quaternion.premultiply(Ds),this}rotateX(e){return this.rotateOnAxis(bu,e)}rotateY(e){return this.rotateOnAxis(Su,e)}rotateZ(e){return this.rotateOnAxis(Tu,e)}translateOnAxis(e,t){return Mu.copy(e).applyQuaternion(this.quaternion),this.position.add(Mu.multiplyScalar(t)),this}translateX(e){return this.translateOnAxis(bu,e)}translateY(e){return this.translateOnAxis(Su,e)}translateZ(e){return this.translateOnAxis(Tu,e)}localToWorld(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(this.matrixWorld)}worldToLocal(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(pi.copy(this.matrixWorld).invert())}lookAt(e,t,n){e.isVector3?Go.copy(e):Go.set(e,t,n);let i=this.parent;this.updateWorldMatrix(!0,!1),Ar.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?pi.lookAt(Ar,Go,this.up):pi.lookAt(Go,Ar,this.up),this.quaternion.setFromRotationMatrix(pi),i&&(pi.extractRotation(i.matrixWorld),Ds.setFromRotationMatrix(pi),this.quaternion.premultiply(Ds.invert()))}add(e){if(arguments.length>1){for(let t=0;t<arguments.length;t++)this.add(arguments[t]);return this}return e===this?(Ze("Object3D.add: object can't be added as a child of itself.",e),this):(e&&e.isObject3D?(e.removeFromParent(),e.parent=this,this.children.push(e),e.dispatchEvent(Eu),Ns.child=e,this.dispatchEvent(Ns),Ns.child=null):Ze("Object3D.add: object not an instance of THREE.Object3D.",e),this)}remove(e){if(arguments.length>1){for(let n=0;n<arguments.length;n++)this.remove(arguments[n]);return this}let t=this.children.indexOf(e);return t!==-1&&(e.parent=null,this.children.splice(t,1),e.dispatchEvent(Ip),hc.child=e,this.dispatchEvent(hc),hc.child=null),this}removeFromParent(){let e=this.parent;return e!==null&&e.remove(this),this}clear(){return this.remove(...this.children)}attach(e){return this.updateWorldMatrix(!0,!1),pi.copy(this.matrixWorld).invert(),e.parent!==null&&(e.parent.updateWorldMatrix(!0,!1),pi.multiply(e.parent.matrixWorld)),e.applyMatrix4(pi),e.removeFromParent(),e.parent=this,this.children.push(e),e.updateWorldMatrix(!1,!0),e.dispatchEvent(Eu),Ns.child=e,this.dispatchEvent(Ns),Ns.child=null,this}getObjectById(e){return this.getObjectByProperty("id",e)}getObjectByName(e){return this.getObjectByProperty("name",e)}getObjectByProperty(e,t){if(this[e]===t)return this;for(let n=0,i=this.children.length;n<i;n++){let o=this.children[n].getObjectByProperty(e,t);if(o!==void 0)return o}}getObjectsByProperty(e,t,n=[]){this[e]===t&&n.push(this);let i=this.children;for(let r=0,o=i.length;r<o;r++)i[r].getObjectsByProperty(e,t,n);return n}getWorldPosition(e){return this.updateWorldMatrix(!0,!1),e.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Ar,e,Cp),e}getWorldScale(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Ar,Pp,e),e}getWorldDirection(e){this.updateWorldMatrix(!0,!1);let t=this.matrixWorld.elements;return e.set(t[8],t[9],t[10]).normalize()}raycast(){}traverse(e){e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverse(e)}traverseVisible(e){if(this.visible===!1)return;e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverseVisible(e)}traverseAncestors(e){let t=this.parent;t!==null&&(e(t),t.traverseAncestors(e))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale);let e=this.pivot;if(e!==null){let t=e.x,n=e.y,i=e.z,r=this.matrix.elements;r[12]+=t-r[0]*t-r[4]*n-r[8]*i,r[13]+=n-r[1]*t-r[5]*n-r[9]*i,r[14]+=i-r[2]*t-r[6]*n-r[10]*i}this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(e){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||e)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,e=!0);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].updateMatrixWorld(e)}updateWorldMatrix(e,t,n=!1){let i=this.parent;if(e===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||n)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,n=!0),t===!0){let r=this.children;for(let o=0,a=r.length;o<a;o++)r[o].updateWorldMatrix(!1,!0,n)}}toJSON(e){let t=e===void 0||typeof e=="string",n={};t&&(e={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},n.metadata={version:4.7,type:"Object",generator:"Object3D.toJSON"});let i={};i.uuid=this.uuid,i.type=this.type,this.name!==""&&(i.name=this.name),this.castShadow===!0&&(i.castShadow=!0),this.receiveShadow===!0&&(i.receiveShadow=!0),this.visible===!1&&(i.visible=!1),this.frustumCulled===!1&&(i.frustumCulled=!1),this.renderOrder!==0&&(i.renderOrder=this.renderOrder),this.static!==!1&&(i.static=this.static),Object.keys(this.userData).length>0&&(i.userData=this.userData),i.layers=this.layers.mask,i.matrix=this.matrix.toArray(),i.up=this.up.toArray(),this.pivot!==null&&(i.pivot=this.pivot.toArray()),this.matrixAutoUpdate===!1&&(i.matrixAutoUpdate=!1),this.morphTargetDictionary!==void 0&&(i.morphTargetDictionary=Object.assign({},this.morphTargetDictionary)),this.morphTargetInfluences!==void 0&&(i.morphTargetInfluences=this.morphTargetInfluences.slice()),this.isInstancedMesh&&(i.type="InstancedMesh",i.count=this.count,i.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(i.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(i.type="BatchedMesh",i.perObjectFrustumCulled=this.perObjectFrustumCulled,i.sortObjects=this.sortObjects,i.drawRanges=this._drawRanges,i.reservedRanges=this._reservedRanges,i.geometryInfo=this._geometryInfo.map(a=>({...a,boundingBox:a.boundingBox?a.boundingBox.toJSON():void 0,boundingSphere:a.boundingSphere?a.boundingSphere.toJSON():void 0})),i.instanceInfo=this._instanceInfo.map(a=>({...a})),i.availableInstanceIds=this._availableInstanceIds.slice(),i.availableGeometryIds=this._availableGeometryIds.slice(),i.nextIndexStart=this._nextIndexStart,i.nextVertexStart=this._nextVertexStart,i.geometryCount=this._geometryCount,i.maxInstanceCount=this._maxInstanceCount,i.maxVertexCount=this._maxVertexCount,i.maxIndexCount=this._maxIndexCount,i.geometryInitialized=this._geometryInitialized,i.matricesTexture=this._matricesTexture.toJSON(e),i.indirectTexture=this._indirectTexture.toJSON(e),this._colorsTexture!==null&&(i.colorsTexture=this._colorsTexture.toJSON(e)),this.boundingSphere!==null&&(i.boundingSphere=this.boundingSphere.toJSON()),this.boundingBox!==null&&(i.boundingBox=this.boundingBox.toJSON()));function r(a,l){return a[l.uuid]===void 0&&(a[l.uuid]=l.toJSON(e)),l.uuid}if(this.isScene)this.background&&(this.background.isColor?i.background=this.background.toJSON():this.background.isTexture&&(i.background=this.background.toJSON(e).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(i.environment=this.environment.toJSON(e).uuid);else if(this.isMesh||this.isLine||this.isPoints){i.geometry=r(e.geometries,this.geometry);let a=this.geometry.parameters;if(a!==void 0&&a.shapes!==void 0){let l=a.shapes;if(Array.isArray(l))for(let c=0,h=l.length;c<h;c++){let u=l[c];r(e.shapes,u)}else r(e.shapes,l)}}if(this.isSkinnedMesh&&(i.bindMode=this.bindMode,i.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(e.skeletons,this.skeleton),i.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let a=[];for(let l=0,c=this.material.length;l<c;l++)a.push(r(e.materials,this.material[l]));i.material=a}else i.material=r(e.materials,this.material);if(this.children.length>0){i.children=[];for(let a=0;a<this.children.length;a++)i.children.push(this.children[a].toJSON(e).object)}if(this.animations.length>0){i.animations=[];for(let a=0;a<this.animations.length;a++){let l=this.animations[a];i.animations.push(r(e.animations,l))}}if(t){let a=o(e.geometries),l=o(e.materials),c=o(e.textures),h=o(e.images),u=o(e.shapes),f=o(e.skeletons),d=o(e.animations),g=o(e.nodes);a.length>0&&(n.geometries=a),l.length>0&&(n.materials=l),c.length>0&&(n.textures=c),h.length>0&&(n.images=h),u.length>0&&(n.shapes=u),f.length>0&&(n.skeletons=f),d.length>0&&(n.animations=d),g.length>0&&(n.nodes=g)}return n.object=i,n;function o(a){let l=[];for(let c in a){let h=a[c];delete h.metadata,l.push(h)}return l}}clone(e){return new this.constructor().copy(this,e)}copy(e,t=!0){if(this.name=e.name,this.up.copy(e.up),this.position.copy(e.position),this.rotation.order=e.rotation.order,this.quaternion.copy(e.quaternion),this.scale.copy(e.scale),this.pivot=e.pivot!==null?e.pivot.clone():null,this.matrix.copy(e.matrix),this.matrixWorld.copy(e.matrixWorld),this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrixWorldAutoUpdate=e.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=e.matrixWorldNeedsUpdate,this.layers.mask=e.layers.mask,this.visible=e.visible,this.castShadow=e.castShadow,this.receiveShadow=e.receiveShadow,this.frustumCulled=e.frustumCulled,this.renderOrder=e.renderOrder,this.static=e.static,this.animations=e.animations.slice(),this.userData=JSON.parse(JSON.stringify(e.userData)),t===!0)for(let n=0;n<e.children.length;n++){let i=e.children[n];this.add(i.clone())}return this}};yt.DEFAULT_UP=new D(0,1,0);yt.DEFAULT_MATRIX_AUTO_UPDATE=!0;yt.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var ut=class extends yt{constructor(){super(),this.isGroup=!0,this.type="Group"}},Lp={type:"move"},$s=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new ut,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new ut,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new D,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new D),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new ut,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new D,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new D,this._grip.eventsEnabled=!1),this._grip}dispatchEvent(e){return this._targetRay!==null&&this._targetRay.dispatchEvent(e),this._grip!==null&&this._grip.dispatchEvent(e),this._hand!==null&&this._hand.dispatchEvent(e),this}connect(e){if(e&&e.hand){let t=this._hand;if(t)for(let n of e.hand.values())this._getHandJoint(t,n)}return this.dispatchEvent({type:"connected",data:e}),this}disconnect(e){return this.dispatchEvent({type:"disconnected",data:e}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(e,t,n){let i=null,r=null,o=null,a=this._targetRay,l=this._grip,c=this._hand;if(e&&t.session.visibilityState!=="visible-blurred"){if(c&&e.hand){o=!0;for(let M of e.hand.values()){let m=t.getJointPose(M,n),p=this._getHandJoint(c,M);m!==null&&(p.matrix.fromArray(m.transform.matrix),p.matrix.decompose(p.position,p.rotation,p.scale),p.matrixWorldNeedsUpdate=!0,p.jointRadius=m.radius),p.visible=m!==null}let h=c.joints["index-finger-tip"],u=c.joints["thumb-tip"],f=h.position.distanceTo(u.position),d=.02,g=.005;c.inputState.pinching&&f>d+g?(c.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:e.handedness,target:this})):!c.inputState.pinching&&f<=d-g&&(c.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:e.handedness,target:this}))}else l!==null&&e.gripSpace&&(r=t.getPose(e.gripSpace,n),r!==null&&(l.matrix.fromArray(r.transform.matrix),l.matrix.decompose(l.position,l.rotation,l.scale),l.matrixWorldNeedsUpdate=!0,r.linearVelocity?(l.hasLinearVelocity=!0,l.linearVelocity.copy(r.linearVelocity)):l.hasLinearVelocity=!1,r.angularVelocity?(l.hasAngularVelocity=!0,l.angularVelocity.copy(r.angularVelocity)):l.hasAngularVelocity=!1,l.eventsEnabled&&l.dispatchEvent({type:"gripUpdated",data:e,target:this})));a!==null&&(i=t.getPose(e.targetRaySpace,n),i===null&&r!==null&&(i=r),i!==null&&(a.matrix.fromArray(i.transform.matrix),a.matrix.decompose(a.position,a.rotation,a.scale),a.matrixWorldNeedsUpdate=!0,i.linearVelocity?(a.hasLinearVelocity=!0,a.linearVelocity.copy(i.linearVelocity)):a.hasLinearVelocity=!1,i.angularVelocity?(a.hasAngularVelocity=!0,a.angularVelocity.copy(i.angularVelocity)):a.hasAngularVelocity=!1,this.dispatchEvent(Lp)))}return a!==null&&(a.visible=i!==null),l!==null&&(l.visible=r!==null),c!==null&&(c.visible=o!==null),this}_getHandJoint(e,t){if(e.joints[t.jointName]===void 0){let n=new ut;n.matrixAutoUpdate=!1,n.visible=!1,e.joints[t.jointName]=n,e.add(n)}return e.joints[t.jointName]}},Vf={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},Fi={h:0,s:0,l:0},Wo={h:0,s:0,l:0};function uc(s,e,t){return t<0&&(t+=1),t>1&&(t-=1),t<1/6?s+(e-s)*6*t:t<1/2?e:t<2/3?s+(e-s)*6*(2/3-t):s}var Me=class{constructor(e,t,n){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(e,t,n)}set(e,t,n){if(t===void 0&&n===void 0){let i=e;i&&i.isColor?this.copy(i):typeof i=="number"?this.setHex(i):typeof i=="string"&&this.setStyle(i)}else this.setRGB(e,t,n);return this}setScalar(e){return this.r=e,this.g=e,this.b=e,this}setHex(e,t=Pt){return e=Math.floor(e),this.r=(e>>16&255)/255,this.g=(e>>8&255)/255,this.b=(e&255)/255,et.colorSpaceToWorking(this,t),this}setRGB(e,t,n,i=et.workingColorSpace){return this.r=e,this.g=t,this.b=n,et.colorSpaceToWorking(this,i),this}setHSL(e,t,n,i=et.workingColorSpace){if(e=sh(e,1),t=tt(t,0,1),n=tt(n,0,1),t===0)this.r=this.g=this.b=n;else{let r=n<=.5?n*(1+t):n+t-n*t,o=2*n-r;this.r=uc(o,r,e+1/3),this.g=uc(o,r,e),this.b=uc(o,r,e-1/3)}return et.colorSpaceToWorking(this,i),this}setStyle(e,t=Pt){function n(r){r!==void 0&&parseFloat(r)<1&&Be("Color: Alpha component of "+e+" will be ignored.")}let i;if(i=/^(\w+)\(([^\)]*)\)/.exec(e)){let r,o=i[1],a=i[2];switch(o){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,t);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,t);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,t);break;default:Be("Color: Unknown color model "+e)}}else if(i=/^\#([A-Fa-f\d]+)$/.exec(e)){let r=i[1],o=r.length;if(o===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,t);if(o===6)return this.setHex(parseInt(r,16),t);Be("Color: Invalid hex color "+e)}else if(e&&e.length>0)return this.setColorName(e,t);return this}setColorName(e,t=Pt){let n=Vf[e.toLowerCase()];return n!==void 0?this.setHex(n,t):Be("Color: Unknown color "+e),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(e){return this.r=e.r,this.g=e.g,this.b=e.b,this}copySRGBToLinear(e){return this.r=yi(e.r),this.g=yi(e.g),this.b=yi(e.b),this}copyLinearToSRGB(e){return this.r=Xs(e.r),this.g=Xs(e.g),this.b=Xs(e.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(e=Pt){return et.workingToColorSpace(tn.copy(this),e),Math.round(tt(tn.r*255,0,255))*65536+Math.round(tt(tn.g*255,0,255))*256+Math.round(tt(tn.b*255,0,255))}getHexString(e=Pt){return("000000"+this.getHex(e).toString(16)).slice(-6)}getHSL(e,t=et.workingColorSpace){et.workingToColorSpace(tn.copy(this),t);let n=tn.r,i=tn.g,r=tn.b,o=Math.max(n,i,r),a=Math.min(n,i,r),l,c,h=(a+o)/2;if(a===o)l=0,c=0;else{let u=o-a;switch(c=h<=.5?u/(o+a):u/(2-o-a),o){case n:l=(i-r)/u+(i<r?6:0);break;case i:l=(r-n)/u+2;break;case r:l=(n-i)/u+4;break}l/=6}return e.h=l,e.s=c,e.l=h,e}getRGB(e,t=et.workingColorSpace){return et.workingToColorSpace(tn.copy(this),t),e.r=tn.r,e.g=tn.g,e.b=tn.b,e}getStyle(e=Pt){et.workingToColorSpace(tn.copy(this),e);let t=tn.r,n=tn.g,i=tn.b;return e!==Pt?`color(${e} ${t.toFixed(3)} ${n.toFixed(3)} ${i.toFixed(3)})`:`rgb(${Math.round(t*255)},${Math.round(n*255)},${Math.round(i*255)})`}offsetHSL(e,t,n){return this.getHSL(Fi),this.setHSL(Fi.h+e,Fi.s+t,Fi.l+n)}add(e){return this.r+=e.r,this.g+=e.g,this.b+=e.b,this}addColors(e,t){return this.r=e.r+t.r,this.g=e.g+t.g,this.b=e.b+t.b,this}addScalar(e){return this.r+=e,this.g+=e,this.b+=e,this}sub(e){return this.r=Math.max(0,this.r-e.r),this.g=Math.max(0,this.g-e.g),this.b=Math.max(0,this.b-e.b),this}multiply(e){return this.r*=e.r,this.g*=e.g,this.b*=e.b,this}multiplyScalar(e){return this.r*=e,this.g*=e,this.b*=e,this}lerp(e,t){return this.r+=(e.r-this.r)*t,this.g+=(e.g-this.g)*t,this.b+=(e.b-this.b)*t,this}lerpColors(e,t,n){return this.r=e.r+(t.r-e.r)*n,this.g=e.g+(t.g-e.g)*n,this.b=e.b+(t.b-e.b)*n,this}lerpHSL(e,t){this.getHSL(Fi),e.getHSL(Wo);let n=Fr(Fi.h,Wo.h,t),i=Fr(Fi.s,Wo.s,t),r=Fr(Fi.l,Wo.l,t);return this.setHSL(n,i,r),this}setFromVector3(e){return this.r=e.x,this.g=e.y,this.b=e.z,this}applyMatrix3(e){let t=this.r,n=this.g,i=this.b,r=e.elements;return this.r=r[0]*t+r[3]*n+r[6]*i,this.g=r[1]*t+r[4]*n+r[7]*i,this.b=r[2]*t+r[5]*n+r[8]*i,this}equals(e){return e.r===this.r&&e.g===this.g&&e.b===this.b}fromArray(e,t=0){return this.r=e[t],this.g=e[t+1],this.b=e[t+2],this}toArray(e=[],t=0){return e[t]=this.r,e[t+1]=this.g,e[t+2]=this.b,e}fromBufferAttribute(e,t){return this.r=e.getX(t),this.g=e.getY(t),this.b=e.getZ(t),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},tn=new Me;Me.NAMES=Vf;var Gr=class s{constructor(e,t=25e-5){this.isFogExp2=!0,this.name="",this.color=new Me(e),this.density=t}clone(){return new s(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var gs=class extends yt{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new Un,this.environmentIntensity=1,this.environmentRotation=new Un,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(e,t){return super.copy(e,t),e.background!==null&&(this.background=e.background.clone()),e.environment!==null&&(this.environment=e.environment.clone()),e.fog!==null&&(this.fog=e.fog.clone()),this.backgroundBlurriness=e.backgroundBlurriness,this.backgroundIntensity=e.backgroundIntensity,this.backgroundRotation.copy(e.backgroundRotation),this.environmentIntensity=e.environmentIntensity,this.environmentRotation.copy(e.environmentRotation),e.overrideMaterial!==null&&(this.overrideMaterial=e.overrideMaterial.clone()),this.matrixAutoUpdate=e.matrixAutoUpdate,this}toJSON(e){let t=super.toJSON(e);return this.fog!==null&&(t.object.fog=this.fog.toJSON()),this.backgroundBlurriness>0&&(t.object.backgroundBlurriness=this.backgroundBlurriness),this.backgroundIntensity!==1&&(t.object.backgroundIntensity=this.backgroundIntensity),t.object.backgroundRotation=this.backgroundRotation.toArray(),this.environmentIntensity!==1&&(t.object.environmentIntensity=this.environmentIntensity),t.object.environmentRotation=this.environmentRotation.toArray(),t}},kn=new D,mi=new D,fc=new D,gi=new D,Us=new D,Fs=new D,wu=new D,dc=new D,pc=new D,mc=new D,gc=new xt,_c=new xt,xc=new xt,Hi=class s{constructor(e=new D,t=new D,n=new D){this.a=e,this.b=t,this.c=n}static getNormal(e,t,n,i){i.subVectors(n,t),kn.subVectors(e,t),i.cross(kn);let r=i.lengthSq();return r>0?i.multiplyScalar(1/Math.sqrt(r)):i.set(0,0,0)}static getBarycoord(e,t,n,i,r){kn.subVectors(i,t),mi.subVectors(n,t),fc.subVectors(e,t);let o=kn.dot(kn),a=kn.dot(mi),l=kn.dot(fc),c=mi.dot(mi),h=mi.dot(fc),u=o*c-a*a;if(u===0)return r.set(0,0,0),null;let f=1/u,d=(c*l-a*h)*f,g=(o*h-a*l)*f;return r.set(1-d-g,g,d)}static containsPoint(e,t,n,i){return this.getBarycoord(e,t,n,i,gi)===null?!1:gi.x>=0&&gi.y>=0&&gi.x+gi.y<=1}static getInterpolation(e,t,n,i,r,o,a,l){return this.getBarycoord(e,t,n,i,gi)===null?(l.x=0,l.y=0,"z"in l&&(l.z=0),"w"in l&&(l.w=0),null):(l.setScalar(0),l.addScaledVector(r,gi.x),l.addScaledVector(o,gi.y),l.addScaledVector(a,gi.z),l)}static getInterpolatedAttribute(e,t,n,i,r,o){return gc.setScalar(0),_c.setScalar(0),xc.setScalar(0),gc.fromBufferAttribute(e,t),_c.fromBufferAttribute(e,n),xc.fromBufferAttribute(e,i),o.setScalar(0),o.addScaledVector(gc,r.x),o.addScaledVector(_c,r.y),o.addScaledVector(xc,r.z),o}static isFrontFacing(e,t,n,i){return kn.subVectors(n,t),mi.subVectors(e,t),kn.cross(mi).dot(i)<0}set(e,t,n){return this.a.copy(e),this.b.copy(t),this.c.copy(n),this}setFromPointsAndIndices(e,t,n,i){return this.a.copy(e[t]),this.b.copy(e[n]),this.c.copy(e[i]),this}setFromAttributeAndIndices(e,t,n,i){return this.a.fromBufferAttribute(e,t),this.b.fromBufferAttribute(e,n),this.c.fromBufferAttribute(e,i),this}clone(){return new this.constructor().copy(this)}copy(e){return this.a.copy(e.a),this.b.copy(e.b),this.c.copy(e.c),this}getArea(){return kn.subVectors(this.c,this.b),mi.subVectors(this.a,this.b),kn.cross(mi).length()*.5}getMidpoint(e){return e.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(e){return s.getNormal(this.a,this.b,this.c,e)}getPlane(e){return e.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(e,t){return s.getBarycoord(e,this.a,this.b,this.c,t)}getInterpolation(e,t,n,i,r){return s.getInterpolation(e,this.a,this.b,this.c,t,n,i,r)}containsPoint(e){return s.containsPoint(e,this.a,this.b,this.c)}isFrontFacing(e){return s.isFrontFacing(this.a,this.b,this.c,e)}intersectsBox(e){return e.intersectsTriangle(this)}closestPointToPoint(e,t){let n=this.a,i=this.b,r=this.c,o,a;Us.subVectors(i,n),Fs.subVectors(r,n),dc.subVectors(e,n);let l=Us.dot(dc),c=Fs.dot(dc);if(l<=0&&c<=0)return t.copy(n);pc.subVectors(e,i);let h=Us.dot(pc),u=Fs.dot(pc);if(h>=0&&u<=h)return t.copy(i);let f=l*u-h*c;if(f<=0&&l>=0&&h<=0)return o=l/(l-h),t.copy(n).addScaledVector(Us,o);mc.subVectors(e,r);let d=Us.dot(mc),g=Fs.dot(mc);if(g>=0&&d<=g)return t.copy(r);let M=d*c-l*g;if(M<=0&&c>=0&&g<=0)return a=c/(c-g),t.copy(n).addScaledVector(Fs,a);let m=h*g-d*u;if(m<=0&&u-h>=0&&d-g>=0)return wu.subVectors(r,i),a=(u-h)/(u-h+(d-g)),t.copy(i).addScaledVector(wu,a);let p=1/(m+M+f);return o=M*p,a=f*p,t.copy(n).addScaledVector(Us,o).addScaledVector(Fs,a)}equals(e){return e.a.equals(this.a)&&e.b.equals(this.b)&&e.c.equals(this.c)}},hn=class{constructor(e=new D(1/0,1/0,1/0),t=new D(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=e,this.max=t}set(e,t){return this.min.copy(e),this.max.copy(t),this}setFromArray(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t+=3)this.expandByPoint(Hn.fromArray(e,t));return this}setFromBufferAttribute(e){this.makeEmpty();for(let t=0,n=e.count;t<n;t++)this.expandByPoint(Hn.fromBufferAttribute(e,t));return this}setFromPoints(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t++)this.expandByPoint(e[t]);return this}setFromCenterAndSize(e,t){let n=Hn.copy(t).multiplyScalar(.5);return this.min.copy(e).sub(n),this.max.copy(e).add(n),this}setFromObject(e,t=!1){return this.makeEmpty(),this.expandByObject(e,t)}clone(){return new this.constructor().copy(this)}copy(e){return this.min.copy(e.min),this.max.copy(e.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(e){return this.isEmpty()?e.set(0,0,0):e.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(e){return this.isEmpty()?e.set(0,0,0):e.subVectors(this.max,this.min)}expandByPoint(e){return this.min.min(e),this.max.max(e),this}expandByVector(e){return this.min.sub(e),this.max.add(e),this}expandByScalar(e){return this.min.addScalar(-e),this.max.addScalar(e),this}expandByObject(e,t=!1){e.updateWorldMatrix(!1,!1);let n=e.geometry;if(n!==void 0){let r=n.getAttribute("position");if(t===!0&&r!==void 0&&e.isInstancedMesh!==!0)for(let o=0,a=r.count;o<a;o++)e.isMesh===!0?e.getVertexPosition(o,Hn):Hn.fromBufferAttribute(r,o),Hn.applyMatrix4(e.matrixWorld),this.expandByPoint(Hn);else e.boundingBox!==void 0?(e.boundingBox===null&&e.computeBoundingBox(),Xo.copy(e.boundingBox)):(n.boundingBox===null&&n.computeBoundingBox(),Xo.copy(n.boundingBox)),Xo.applyMatrix4(e.matrixWorld),this.union(Xo)}let i=e.children;for(let r=0,o=i.length;r<o;r++)this.expandByObject(i[r],t);return this}containsPoint(e){return e.x>=this.min.x&&e.x<=this.max.x&&e.y>=this.min.y&&e.y<=this.max.y&&e.z>=this.min.z&&e.z<=this.max.z}containsBox(e){return this.min.x<=e.min.x&&e.max.x<=this.max.x&&this.min.y<=e.min.y&&e.max.y<=this.max.y&&this.min.z<=e.min.z&&e.max.z<=this.max.z}getParameter(e,t){return t.set((e.x-this.min.x)/(this.max.x-this.min.x),(e.y-this.min.y)/(this.max.y-this.min.y),(e.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(e){return e.max.x>=this.min.x&&e.min.x<=this.max.x&&e.max.y>=this.min.y&&e.min.y<=this.max.y&&e.max.z>=this.min.z&&e.min.z<=this.max.z}intersectsSphere(e){return this.clampPoint(e.center,Hn),Hn.distanceToSquared(e.center)<=e.radius*e.radius}intersectsPlane(e){let t,n;return e.normal.x>0?(t=e.normal.x*this.min.x,n=e.normal.x*this.max.x):(t=e.normal.x*this.max.x,n=e.normal.x*this.min.x),e.normal.y>0?(t+=e.normal.y*this.min.y,n+=e.normal.y*this.max.y):(t+=e.normal.y*this.max.y,n+=e.normal.y*this.min.y),e.normal.z>0?(t+=e.normal.z*this.min.z,n+=e.normal.z*this.max.z):(t+=e.normal.z*this.max.z,n+=e.normal.z*this.min.z),t<=-e.constant&&n>=-e.constant}intersectsTriangle(e){if(this.isEmpty())return!1;this.getCenter(Rr),qo.subVectors(this.max,Rr),Os.subVectors(e.a,Rr),Bs.subVectors(e.b,Rr),zs.subVectors(e.c,Rr),Oi.subVectors(Bs,Os),Bi.subVectors(zs,Bs),is.subVectors(Os,zs);let t=[0,-Oi.z,Oi.y,0,-Bi.z,Bi.y,0,-is.z,is.y,Oi.z,0,-Oi.x,Bi.z,0,-Bi.x,is.z,0,-is.x,-Oi.y,Oi.x,0,-Bi.y,Bi.x,0,-is.y,is.x,0];return!vc(t,Os,Bs,zs,qo)||(t=[1,0,0,0,1,0,0,0,1],!vc(t,Os,Bs,zs,qo))?!1:(Yo.crossVectors(Oi,Bi),t=[Yo.x,Yo.y,Yo.z],vc(t,Os,Bs,zs,qo))}clampPoint(e,t){return t.copy(e).clamp(this.min,this.max)}distanceToPoint(e){return this.clampPoint(e,Hn).distanceTo(e)}getBoundingSphere(e){return this.isEmpty()?e.makeEmpty():(this.getCenter(e.center),e.radius=this.getSize(Hn).length()*.5),e}intersect(e){return this.min.max(e.min),this.max.min(e.max),this.isEmpty()&&this.makeEmpty(),this}union(e){return this.min.min(e.min),this.max.max(e.max),this}applyMatrix4(e){return this.isEmpty()?this:(_i[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(e),_i[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(e),_i[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(e),_i[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(e),_i[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(e),_i[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(e),_i[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(e),_i[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(e),this.setFromPoints(_i),this)}translate(e){return this.min.add(e),this.max.add(e),this}equals(e){return e.min.equals(this.min)&&e.max.equals(this.max)}toJSON(){return{min:this.min.toArray(),max:this.max.toArray()}}fromJSON(e){return this.min.fromArray(e.min),this.max.fromArray(e.max),this}},_i=[new D,new D,new D,new D,new D,new D,new D,new D],Hn=new D,Xo=new hn,Os=new D,Bs=new D,zs=new D,Oi=new D,Bi=new D,is=new D,Rr=new D,qo=new D,Yo=new D,ss=new D;function vc(s,e,t,n,i){for(let r=0,o=s.length-3;r<=o;r+=3){ss.fromArray(s,r);let a=i.x*Math.abs(ss.x)+i.y*Math.abs(ss.y)+i.z*Math.abs(ss.z),l=e.dot(ss),c=t.dot(ss),h=n.dot(ss);if(Math.max(-Math.max(l,c,h),Math.min(l,c,h))>a)return!1}return!0}var kt=new D,Zo=new ye,Dp=0,vt=class extends Nn{constructor(e,t,n=!1){if(super(),Array.isArray(e))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,Object.defineProperty(this,"id",{value:Dp++}),this.name="",this.array=e,this.itemSize=t,this.count=e!==void 0?e.length/t:0,this.normalized=n,this.usage=Ta,this.updateRanges=[],this.gpuType=En,this.version=0}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.name=e.name,this.array=new e.array.constructor(e.array),this.itemSize=e.itemSize,this.count=e.count,this.normalized=e.normalized,this.usage=e.usage,this.gpuType=e.gpuType,this}copyAt(e,t,n){e*=this.itemSize,n*=t.itemSize;for(let i=0,r=this.itemSize;i<r;i++)this.array[e+i]=t.array[n+i];return this}copyArray(e){return this.array.set(e),this}applyMatrix3(e){if(this.itemSize===2)for(let t=0,n=this.count;t<n;t++)Zo.fromBufferAttribute(this,t),Zo.applyMatrix3(e),this.setXY(t,Zo.x,Zo.y);else if(this.itemSize===3)for(let t=0,n=this.count;t<n;t++)kt.fromBufferAttribute(this,t),kt.applyMatrix3(e),this.setXYZ(t,kt.x,kt.y,kt.z);return this}applyMatrix4(e){for(let t=0,n=this.count;t<n;t++)kt.fromBufferAttribute(this,t),kt.applyMatrix4(e),this.setXYZ(t,kt.x,kt.y,kt.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)kt.fromBufferAttribute(this,t),kt.applyNormalMatrix(e),this.setXYZ(t,kt.x,kt.y,kt.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)kt.fromBufferAttribute(this,t),kt.transformDirection(e),this.setXYZ(t,kt.x,kt.y,kt.z);return this}set(e,t=0){return this.array.set(e,t),this}getComponent(e,t){let n=this.array[e*this.itemSize+t];return this.normalized&&(n=Vn(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=Mt(n,this.array)),this.array[e*this.itemSize+t]=n,this}getX(e){let t=this.array[e*this.itemSize];return this.normalized&&(t=Vn(t,this.array)),t}setX(e,t){return this.normalized&&(t=Mt(t,this.array)),this.array[e*this.itemSize]=t,this}getY(e){let t=this.array[e*this.itemSize+1];return this.normalized&&(t=Vn(t,this.array)),t}setY(e,t){return this.normalized&&(t=Mt(t,this.array)),this.array[e*this.itemSize+1]=t,this}getZ(e){let t=this.array[e*this.itemSize+2];return this.normalized&&(t=Vn(t,this.array)),t}setZ(e,t){return this.normalized&&(t=Mt(t,this.array)),this.array[e*this.itemSize+2]=t,this}getW(e){let t=this.array[e*this.itemSize+3];return this.normalized&&(t=Vn(t,this.array)),t}setW(e,t){return this.normalized&&(t=Mt(t,this.array)),this.array[e*this.itemSize+3]=t,this}setXY(e,t,n){return e*=this.itemSize,this.normalized&&(t=Mt(t,this.array),n=Mt(n,this.array)),this.array[e+0]=t,this.array[e+1]=n,this}setXYZ(e,t,n,i){return e*=this.itemSize,this.normalized&&(t=Mt(t,this.array),n=Mt(n,this.array),i=Mt(i,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e*=this.itemSize,this.normalized&&(t=Mt(t,this.array),n=Mt(n,this.array),i=Mt(i,this.array),r=Mt(r,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this.array[e+3]=r,this}onUpload(e){return this.onUploadCallback=e,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let e={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return this.name!==""&&(e.name=this.name),this.usage!==Ta&&(e.usage=this.usage),e}dispose(){this.dispatchEvent({type:"dispose"})}};var Wr=class extends vt{constructor(e,t,n){super(new Uint16Array(e),t,n)}};var Xr=class extends vt{constructor(e,t,n){super(new Uint32Array(e),t,n)}};var ot=class extends vt{constructor(e,t,n){super(new Float32Array(e),t,n)}},Np=new hn,Cr=new D,yc=new D,nn=class{constructor(e=new D,t=-1){this.isSphere=!0,this.center=e,this.radius=t}set(e,t){return this.center.copy(e),this.radius=t,this}setFromPoints(e,t){let n=this.center;t!==void 0?n.copy(t):Np.setFromPoints(e).getCenter(n);let i=0;for(let r=0,o=e.length;r<o;r++)i=Math.max(i,n.distanceToSquared(e[r]));return this.radius=Math.sqrt(i),this}copy(e){return this.center.copy(e.center),this.radius=e.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(e){return e.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(e){return e.distanceTo(this.center)-this.radius}intersectsSphere(e){let t=this.radius+e.radius;return e.center.distanceToSquared(this.center)<=t*t}intersectsBox(e){return e.intersectsSphere(this)}intersectsPlane(e){return Math.abs(e.distanceToPoint(this.center))<=this.radius}clampPoint(e,t){let n=this.center.distanceToSquared(e);return t.copy(e),n>this.radius*this.radius&&(t.sub(this.center).normalize(),t.multiplyScalar(this.radius).add(this.center)),t}getBoundingBox(e){return this.isEmpty()?(e.makeEmpty(),e):(e.set(this.center,this.center),e.expandByScalar(this.radius),e)}applyMatrix4(e){return this.center.applyMatrix4(e),this.radius=this.radius*e.getMaxScaleOnAxis(),this}translate(e){return this.center.add(e),this}expandByPoint(e){if(this.isEmpty())return this.center.copy(e),this.radius=0,this;Cr.subVectors(e,this.center);let t=Cr.lengthSq();if(t>this.radius*this.radius){let n=Math.sqrt(t),i=(n-this.radius)*.5;this.center.addScaledVector(Cr,i/n),this.radius+=i}return this}union(e){return e.isEmpty()?this:this.isEmpty()?(this.copy(e),this):(this.center.equals(e.center)===!0?this.radius=Math.max(this.radius,e.radius):(yc.subVectors(e.center,this.center).setLength(e.radius),this.expandByPoint(Cr.copy(e.center).add(yc)),this.expandByPoint(Cr.copy(e.center).sub(yc))),this)}equals(e){return e.center.equals(this.center)&&e.radius===this.radius}clone(){return new this.constructor().copy(this)}toJSON(){return{radius:this.radius,center:this.center.toArray()}}fromJSON(e){return this.radius=e.radius,this.center.fromArray(e.center),this}},Up=0,In=new Ke,Mc=new yt,ks=new D,bn=new hn,Pr=new hn,jt=new D,ft=class s extends Nn{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:Up++}),this.uuid=Wn(),this.name="",this.type="BufferGeometry",this.index=null,this.indirect=null,this.indirectOffset=0,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={},this._transformed=!1}getIndex(){return this.index}setIndex(e){return Array.isArray(e)?this.index=new(rp(e)?Xr:Wr)(e,1):this.index=e,this}setIndirect(e,t=0){return this.indirect=e,this.indirectOffset=t,this}getIndirect(){return this.indirect}getAttribute(e){return this.attributes[e]}setAttribute(e,t){return this.attributes[e]=t,this}deleteAttribute(e){return delete this.attributes[e],this}hasAttribute(e){return this.attributes[e]!==void 0}addGroup(e,t,n=0){this.groups.push({start:e,count:t,materialIndex:n})}clearGroups(){this.groups=[]}setDrawRange(e,t){this.drawRange.start=e,this.drawRange.count=t}applyMatrix4(e){let t=this.attributes.position;t!==void 0&&(t.applyMatrix4(e),t.needsUpdate=!0);let n=this.attributes.normal;if(n!==void 0){let r=new $e().getNormalMatrix(e);n.applyNormalMatrix(r),n.needsUpdate=!0}let i=this.attributes.tangent;return i!==void 0&&(i.transformDirection(e),i.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this._transformed=!0,this}applyQuaternion(e){return In.makeRotationFromQuaternion(e),this.applyMatrix4(In),this}rotateX(e){return In.makeRotationX(e),this.applyMatrix4(In),this}rotateY(e){return In.makeRotationY(e),this.applyMatrix4(In),this}rotateZ(e){return In.makeRotationZ(e),this.applyMatrix4(In),this}translate(e,t,n){return In.makeTranslation(e,t,n),this.applyMatrix4(In),this}scale(e,t,n){return In.makeScale(e,t,n),this.applyMatrix4(In),this}lookAt(e){return Mc.lookAt(e),Mc.updateMatrix(),this.applyMatrix4(Mc.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(ks).negate(),this.translate(ks.x,ks.y,ks.z),this}setFromPoints(e){let t=this.getAttribute("position");if(t===void 0){let n=[];for(let i=0,r=e.length;i<r;i++){let o=e[i];n.push(o.x,o.y,o.z||0)}this.setAttribute("position",new ot(n,3))}else{let n=Math.min(e.length,t.count);for(let i=0;i<n;i++){let r=e[i];t.setXYZ(i,r.x,r.y,r.z||0)}e.length>t.count&&Be("BufferGeometry: Buffer size too small for points data. Use .dispose() and create a new geometry."),t.needsUpdate=!0}return this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new hn);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){Ze("BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new D(-1/0,-1/0,-1/0),new D(1/0,1/0,1/0));return}if(e!==void 0){if(this.boundingBox.setFromBufferAttribute(e),t)for(let n=0,i=t.length;n<i;n++){let r=t[n];bn.setFromBufferAttribute(r),this.morphTargetsRelative?(jt.addVectors(this.boundingBox.min,bn.min),this.boundingBox.expandByPoint(jt),jt.addVectors(this.boundingBox.max,bn.max),this.boundingBox.expandByPoint(jt)):(this.boundingBox.expandByPoint(bn.min),this.boundingBox.expandByPoint(bn.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&Ze('BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new nn);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){Ze("BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new D,1/0);return}if(e){let n=this.boundingSphere.center;if(bn.setFromBufferAttribute(e),t)for(let r=0,o=t.length;r<o;r++){let a=t[r];Pr.setFromBufferAttribute(a),this.morphTargetsRelative?(jt.addVectors(bn.min,Pr.min),bn.expandByPoint(jt),jt.addVectors(bn.max,Pr.max),bn.expandByPoint(jt)):(bn.expandByPoint(Pr.min),bn.expandByPoint(Pr.max))}bn.getCenter(n);let i=0;for(let r=0,o=e.count;r<o;r++)jt.fromBufferAttribute(e,r),i=Math.max(i,n.distanceToSquared(jt));if(t)for(let r=0,o=t.length;r<o;r++){let a=t[r],l=this.morphTargetsRelative;for(let c=0,h=a.count;c<h;c++)jt.fromBufferAttribute(a,c),l&&(ks.fromBufferAttribute(e,c),jt.add(ks)),i=Math.max(i,n.distanceToSquared(jt))}this.boundingSphere.radius=Math.sqrt(i),isNaN(this.boundingSphere.radius)&&Ze('BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let e=this.index,t=this.attributes;if(e===null||t.position===void 0||t.normal===void 0||t.uv===void 0){Ze("BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let n=t.position,i=t.normal,r=t.uv,o=this.getAttribute("tangent");(o===void 0||o.count!==n.count)&&(o=new vt(new Float32Array(4*n.count),4),this.setAttribute("tangent",o));let a=[],l=[];for(let _=0;_<n.count;_++)a[_]=new D,l[_]=new D;let c=new D,h=new D,u=new D,f=new ye,d=new ye,g=new ye,M=new D,m=new D;function p(_,v,E){c.fromBufferAttribute(n,_),h.fromBufferAttribute(n,v),u.fromBufferAttribute(n,E),f.fromBufferAttribute(r,_),d.fromBufferAttribute(r,v),g.fromBufferAttribute(r,E),h.sub(c),u.sub(c),d.sub(f),g.sub(f);let P=1/(d.x*g.y-g.x*d.y);isFinite(P)&&(M.copy(h).multiplyScalar(g.y).addScaledVector(u,-d.y).multiplyScalar(P),m.copy(u).multiplyScalar(d.x).addScaledVector(h,-g.x).multiplyScalar(P),a[_].add(M),a[v].add(M),a[E].add(M),l[_].add(m),l[v].add(m),l[E].add(m))}let S=this.groups;S.length===0&&(S=[{start:0,count:e.count}]);for(let _=0,v=S.length;_<v;++_){let E=S[_],P=E.start,L=E.count;for(let H=P,O=P+L;H<O;H+=3)p(e.getX(H+0),e.getX(H+1),e.getX(H+2))}let R=new D,b=new D,w=new D,y=new D;function C(_){w.fromBufferAttribute(i,_),y.copy(w);let v=a[_];R.copy(v),R.sub(w.multiplyScalar(w.dot(v))).normalize(),b.crossVectors(y,v);let P=b.dot(l[_])<0?-1:1;o.setXYZW(_,R.x,R.y,R.z,P)}for(let _=0,v=S.length;_<v;++_){let E=S[_],P=E.start,L=E.count;for(let H=P,O=P+L;H<O;H+=3)C(e.getX(H+0)),C(e.getX(H+1)),C(e.getX(H+2))}this._transformed=!0}computeVertexNormals(){let e=this.index,t=this.getAttribute("position");if(t!==void 0){let n=this.getAttribute("normal");if(n===void 0||n.count!==t.count)n=new vt(new Float32Array(t.count*3),3),this.setAttribute("normal",n);else for(let f=0,d=n.count;f<d;f++)n.setXYZ(f,0,0,0);let i=new D,r=new D,o=new D,a=new D,l=new D,c=new D,h=new D,u=new D;if(e)for(let f=0,d=e.count;f<d;f+=3){let g=e.getX(f+0),M=e.getX(f+1),m=e.getX(f+2);i.fromBufferAttribute(t,g),r.fromBufferAttribute(t,M),o.fromBufferAttribute(t,m),h.subVectors(o,r),u.subVectors(i,r),h.cross(u),a.fromBufferAttribute(n,g),l.fromBufferAttribute(n,M),c.fromBufferAttribute(n,m),a.add(h),l.add(h),c.add(h),n.setXYZ(g,a.x,a.y,a.z),n.setXYZ(M,l.x,l.y,l.z),n.setXYZ(m,c.x,c.y,c.z)}else for(let f=0,d=t.count;f<d;f+=3)i.fromBufferAttribute(t,f+0),r.fromBufferAttribute(t,f+1),o.fromBufferAttribute(t,f+2),h.subVectors(o,r),u.subVectors(i,r),h.cross(u),n.setXYZ(f+0,h.x,h.y,h.z),n.setXYZ(f+1,h.x,h.y,h.z),n.setXYZ(f+2,h.x,h.y,h.z);this.normalizeNormals(),n.needsUpdate=!0}}normalizeNormals(){let e=this.attributes.normal;for(let t=0,n=e.count;t<n;t++)jt.fromBufferAttribute(e,t),jt.normalize(),e.setXYZ(t,jt.x,jt.y,jt.z)}toNonIndexed(){function e(a,l){let c=a.array,h=a.itemSize,u=a.normalized,f=new c.constructor(l.length*h),d=0,g=0;for(let M=0,m=l.length;M<m;M++){a.isInterleavedBufferAttribute?d=l[M]*a.data.stride+a.offset:d=l[M]*h;for(let p=0;p<h;p++)f[g++]=c[d++]}return new vt(f,h,u)}if(this.index===null)return Be("BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let t=new s,n=this.index.array,i=this.attributes;for(let a in i){let l=i[a],c=e(l,n);t.setAttribute(a,c)}let r=this.morphAttributes;for(let a in r){let l=[],c=r[a];for(let h=0,u=c.length;h<u;h++){let f=c[h],d=e(f,n);l.push(d)}t.morphAttributes[a]=l}t.morphTargetsRelative=this.morphTargetsRelative;let o=this.groups;for(let a=0,l=o.length;a<l;a++){let c=o[a];t.addGroup(c.start,c.count,c.materialIndex)}return t}toJSON(){let e={metadata:{version:4.7,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(e.uuid=this.uuid,e.type=this.parameters!==void 0&&this._transformed===!0?"BufferGeometry":this.type,this.name!==""&&(e.name=this.name),Object.keys(this.userData).length>0&&(e.userData=this.userData),this.parameters!==void 0&&this._transformed!==!0){let l=this.parameters;for(let c in l)l[c]!==void 0&&(e[c]=l[c]);return e}e.data={attributes:{}};let t=this.index;t!==null&&(e.data.index={type:t.array.constructor.name,array:Array.prototype.slice.call(t.array)});let n=this.attributes;for(let l in n){let c=n[l];e.data.attributes[l]=c.toJSON(e.data)}let i={},r=!1;for(let l in this.morphAttributes){let c=this.morphAttributes[l],h=[];for(let u=0,f=c.length;u<f;u++){let d=c[u];h.push(d.toJSON(e.data))}h.length>0&&(i[l]=h,r=!0)}r&&(e.data.morphAttributes=i,e.data.morphTargetsRelative=this.morphTargetsRelative);let o=this.groups;o.length>0&&(e.data.groups=JSON.parse(JSON.stringify(o)));let a=this.boundingSphere;return a!==null&&(e.data.boundingSphere=a.toJSON()),e}clone(){return new this.constructor().copy(this)}copy(e){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let t={};this.name=e.name;let n=e.index;n!==null&&this.setIndex(n.clone());let i=e.attributes;for(let c in i){let h=i[c];this.setAttribute(c,h.clone(t))}let r=e.morphAttributes;for(let c in r){let h=[],u=r[c];for(let f=0,d=u.length;f<d;f++)h.push(u[f].clone(t));this.morphAttributes[c]=h}this.morphTargetsRelative=e.morphTargetsRelative;let o=e.groups;for(let c=0,h=o.length;c<h;c++){let u=o[c];this.addGroup(u.start,u.count,u.materialIndex)}let a=e.boundingBox;a!==null&&(this.boundingBox=a.clone());let l=e.boundingSphere;return l!==null&&(this.boundingSphere=l.clone()),this.drawRange.start=e.drawRange.start,this.drawRange.count=e.drawRange.count,this.userData=e.userData,this._transformed=e._transformed,this}dispose(){this.dispatchEvent({type:"dispose"})}},Qs=class{constructor(e,t){this.isInterleavedBuffer=!0,this.array=e,this.stride=t,this.count=e!==void 0?e.length/t:0,this.usage=Ta,this.updateRanges=[],this.version=0,this.uuid=Wn()}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.array=new e.array.constructor(e.array),this.count=e.count,this.stride=e.stride,this.usage=e.usage,this}copyAt(e,t,n){e*=this.stride,n*=t.stride;for(let i=0,r=this.stride;i<r;i++)this.array[e+i]=t.array[n+i];return this}set(e,t=0){return this.array.set(e,t),this}clone(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=Wn()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=this.array.slice(0).buffer);let t=new this.array.constructor(e.arrayBuffers[this.array.buffer._uuid]),n=new this.constructor(t,this.stride);return n.setUsage(this.usage),n}onUpload(e){return this.onUploadCallback=e,this}toJSON(e){return e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=Wn()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=Array.from(new Uint32Array(this.array.buffer))),{uuid:this.uuid,buffer:this.array.buffer._uuid,type:this.array.constructor.name,stride:this.stride}}},ln=new D,er=class s{constructor(e,t,n,i=!1){this.isInterleavedBufferAttribute=!0,this.name="",this.data=e,this.itemSize=t,this.offset=n,this.normalized=i}get count(){return this.data.count}get array(){return this.data.array}set needsUpdate(e){this.data.needsUpdate=e}applyMatrix4(e){for(let t=0,n=this.data.count;t<n;t++)ln.fromBufferAttribute(this,t),ln.applyMatrix4(e),this.setXYZ(t,ln.x,ln.y,ln.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)ln.fromBufferAttribute(this,t),ln.applyNormalMatrix(e),this.setXYZ(t,ln.x,ln.y,ln.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)ln.fromBufferAttribute(this,t),ln.transformDirection(e),this.setXYZ(t,ln.x,ln.y,ln.z);return this}getComponent(e,t){let n=this.array[e*this.data.stride+this.offset+t];return this.normalized&&(n=Vn(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=Mt(n,this.array)),this.data.array[e*this.data.stride+this.offset+t]=n,this}setX(e,t){return this.normalized&&(t=Mt(t,this.array)),this.data.array[e*this.data.stride+this.offset]=t,this}setY(e,t){return this.normalized&&(t=Mt(t,this.array)),this.data.array[e*this.data.stride+this.offset+1]=t,this}setZ(e,t){return this.normalized&&(t=Mt(t,this.array)),this.data.array[e*this.data.stride+this.offset+2]=t,this}setW(e,t){return this.normalized&&(t=Mt(t,this.array)),this.data.array[e*this.data.stride+this.offset+3]=t,this}getX(e){let t=this.data.array[e*this.data.stride+this.offset];return this.normalized&&(t=Vn(t,this.array)),t}getY(e){let t=this.data.array[e*this.data.stride+this.offset+1];return this.normalized&&(t=Vn(t,this.array)),t}getZ(e){let t=this.data.array[e*this.data.stride+this.offset+2];return this.normalized&&(t=Vn(t,this.array)),t}getW(e){let t=this.data.array[e*this.data.stride+this.offset+3];return this.normalized&&(t=Vn(t,this.array)),t}setXY(e,t,n){return e=e*this.data.stride+this.offset,this.normalized&&(t=Mt(t,this.array),n=Mt(n,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this}setXYZ(e,t,n,i){return e=e*this.data.stride+this.offset,this.normalized&&(t=Mt(t,this.array),n=Mt(n,this.array),i=Mt(i,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e=e*this.data.stride+this.offset,this.normalized&&(t=Mt(t,this.array),n=Mt(n,this.array),i=Mt(i,this.array),r=Mt(r,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this.data.array[e+3]=r,this}clone(e){if(e===void 0){Hr("InterleavedBufferAttribute.clone(): Cloning an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return new vt(new this.array.constructor(t),this.itemSize,this.normalized)}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.clone(e)),new s(e.interleavedBuffers[this.data.uuid],this.itemSize,this.offset,this.normalized)}toJSON(e){if(e===void 0){Hr("InterleavedBufferAttribute.toJSON(): Serializing an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return{itemSize:this.itemSize,type:this.array.constructor.name,array:t,normalized:this.normalized}}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.toJSON(e)),{isInterleavedBufferAttribute:!0,itemSize:this.itemSize,data:this.data.uuid,offset:this.offset,normalized:this.normalized}}},Fp=0,un=class extends Nn{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:Fp++}),this.uuid=Wn(),this.name="",this.type="Material",this.blending=us,this.side=Sn,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=ma,this.blendDst=ga,this.blendEquation=Vi,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new Me(0,0,0),this.blendAlpha=0,this.depthFunc=fs,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=Oc,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=as,this.stencilZFail=as,this.stencilZPass=as,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.allowOverride=!0,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(e){this._alphaTest>0!=e>0&&this.version++,this._alphaTest=e}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(e){if(e!==void 0)for(let t in e){let n=e[t];if(n===void 0){Be(`Material: parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){Be(`Material: '${t}' is not a property of THREE.${this.type}.`);continue}i&&i.isColor?i.set(n):i&&i.isVector2&&n&&n.isVector2||i&&i.isEuler&&n&&n.isEuler||i&&i.isVector3&&n&&n.isVector3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";t&&(e={textures:{},images:{}});let n={metadata:{version:4.7,type:"Material",generator:"Material.toJSON"}};n.uuid=this.uuid,n.type=this.type,this.name!==""&&(n.name=this.name),this.color&&this.color.isColor&&(n.color=this.color.getHex()),this.roughness!==void 0&&(n.roughness=this.roughness),this.metalness!==void 0&&(n.metalness=this.metalness),this.sheen!==void 0&&(n.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(n.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(n.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(n.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&this.emissiveIntensity!==1&&(n.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(n.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(n.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(n.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(n.shininess=this.shininess),this.clearcoat!==void 0&&(n.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(n.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(n.clearcoatMap=this.clearcoatMap.toJSON(e).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(n.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(e).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(n.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(e).uuid,n.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.sheenColorMap&&this.sheenColorMap.isTexture&&(n.sheenColorMap=this.sheenColorMap.toJSON(e).uuid),this.sheenRoughnessMap&&this.sheenRoughnessMap.isTexture&&(n.sheenRoughnessMap=this.sheenRoughnessMap.toJSON(e).uuid),this.dispersion!==void 0&&(n.dispersion=this.dispersion),this.iridescence!==void 0&&(n.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(n.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(n.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(n.iridescenceMap=this.iridescenceMap.toJSON(e).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(n.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(e).uuid),this.anisotropy!==void 0&&(n.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(n.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(n.anisotropyMap=this.anisotropyMap.toJSON(e).uuid),this.map&&this.map.isTexture&&(n.map=this.map.toJSON(e).uuid),this.matcap&&this.matcap.isTexture&&(n.matcap=this.matcap.toJSON(e).uuid),this.alphaMap&&this.alphaMap.isTexture&&(n.alphaMap=this.alphaMap.toJSON(e).uuid),this.lightMap&&this.lightMap.isTexture&&(n.lightMap=this.lightMap.toJSON(e).uuid,n.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(n.aoMap=this.aoMap.toJSON(e).uuid,n.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(n.bumpMap=this.bumpMap.toJSON(e).uuid,n.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(n.normalMap=this.normalMap.toJSON(e).uuid,n.normalMapType=this.normalMapType,n.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(n.displacementMap=this.displacementMap.toJSON(e).uuid,n.displacementScale=this.displacementScale,n.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(n.roughnessMap=this.roughnessMap.toJSON(e).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(n.metalnessMap=this.metalnessMap.toJSON(e).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(n.emissiveMap=this.emissiveMap.toJSON(e).uuid),this.specularMap&&this.specularMap.isTexture&&(n.specularMap=this.specularMap.toJSON(e).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(n.specularIntensityMap=this.specularIntensityMap.toJSON(e).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(n.specularColorMap=this.specularColorMap.toJSON(e).uuid),this.envMap&&this.envMap.isTexture&&(n.envMap=this.envMap.toJSON(e).uuid,this.combine!==void 0&&(n.combine=this.combine)),this.envMapRotation!==void 0&&(n.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(n.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(n.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(n.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(n.gradientMap=this.gradientMap.toJSON(e).uuid),this.transmission!==void 0&&(n.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(n.transmissionMap=this.transmissionMap.toJSON(e).uuid),this.thickness!==void 0&&(n.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(n.thicknessMap=this.thicknessMap.toJSON(e).uuid),this.attenuationDistance!==void 0&&this.attenuationDistance!==1/0&&(n.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(n.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(n.size=this.size),this.shadowSide!==null&&(n.shadowSide=this.shadowSide),this.sizeAttenuation!==void 0&&(n.sizeAttenuation=this.sizeAttenuation),this.blending!==us&&(n.blending=this.blending),this.side!==Sn&&(n.side=this.side),this.vertexColors===!0&&(n.vertexColors=!0),this.opacity<1&&(n.opacity=this.opacity),this.transparent===!0&&(n.transparent=!0),this.blendSrc!==ma&&(n.blendSrc=this.blendSrc),this.blendDst!==ga&&(n.blendDst=this.blendDst),this.blendEquation!==Vi&&(n.blendEquation=this.blendEquation),this.blendSrcAlpha!==null&&(n.blendSrcAlpha=this.blendSrcAlpha),this.blendDstAlpha!==null&&(n.blendDstAlpha=this.blendDstAlpha),this.blendEquationAlpha!==null&&(n.blendEquationAlpha=this.blendEquationAlpha),this.blendColor&&this.blendColor.isColor&&(n.blendColor=this.blendColor.getHex()),this.blendAlpha!==0&&(n.blendAlpha=this.blendAlpha),this.depthFunc!==fs&&(n.depthFunc=this.depthFunc),this.depthTest===!1&&(n.depthTest=this.depthTest),this.depthWrite===!1&&(n.depthWrite=this.depthWrite),this.colorWrite===!1&&(n.colorWrite=this.colorWrite),this.stencilWriteMask!==255&&(n.stencilWriteMask=this.stencilWriteMask),this.stencilFunc!==Oc&&(n.stencilFunc=this.stencilFunc),this.stencilRef!==0&&(n.stencilRef=this.stencilRef),this.stencilFuncMask!==255&&(n.stencilFuncMask=this.stencilFuncMask),this.stencilFail!==as&&(n.stencilFail=this.stencilFail),this.stencilZFail!==as&&(n.stencilZFail=this.stencilZFail),this.stencilZPass!==as&&(n.stencilZPass=this.stencilZPass),this.stencilWrite===!0&&(n.stencilWrite=this.stencilWrite),this.rotation!==void 0&&this.rotation!==0&&(n.rotation=this.rotation),this.polygonOffset===!0&&(n.polygonOffset=!0),this.polygonOffsetFactor!==0&&(n.polygonOffsetFactor=this.polygonOffsetFactor),this.polygonOffsetUnits!==0&&(n.polygonOffsetUnits=this.polygonOffsetUnits),this.linewidth!==void 0&&this.linewidth!==1&&(n.linewidth=this.linewidth),this.dashSize!==void 0&&(n.dashSize=this.dashSize),this.gapSize!==void 0&&(n.gapSize=this.gapSize),this.scale!==void 0&&(n.scale=this.scale),this.dithering===!0&&(n.dithering=!0),this.alphaTest>0&&(n.alphaTest=this.alphaTest),this.alphaHash===!0&&(n.alphaHash=!0),this.alphaToCoverage===!0&&(n.alphaToCoverage=!0),this.premultipliedAlpha===!0&&(n.premultipliedAlpha=!0),this.forceSinglePass===!0&&(n.forceSinglePass=!0),this.allowOverride===!1&&(n.allowOverride=!1),this.wireframe===!0&&(n.wireframe=!0),this.wireframeLinewidth>1&&(n.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!=="round"&&(n.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!=="round"&&(n.wireframeLinejoin=this.wireframeLinejoin),this.flatShading===!0&&(n.flatShading=!0),this.visible===!1&&(n.visible=!1),this.toneMapped===!1&&(n.toneMapped=!1),this.fog===!1&&(n.fog=!1),Object.keys(this.userData).length>0&&(n.userData=this.userData);function i(r){let o=[];for(let a in r){let l=r[a];delete l.metadata,o.push(l)}return o}if(t){let r=i(e.textures),o=i(e.images);r.length>0&&(n.textures=r),o.length>0&&(n.images=o)}return n}fromJSON(e,t){if(e.uuid!==void 0&&(this.uuid=e.uuid),e.name!==void 0&&(this.name=e.name),e.color!==void 0&&this.color!==void 0&&this.color.setHex(e.color),e.roughness!==void 0&&(this.roughness=e.roughness),e.metalness!==void 0&&(this.metalness=e.metalness),e.sheen!==void 0&&(this.sheen=e.sheen),e.sheenColor!==void 0&&(this.sheenColor=new Me().setHex(e.sheenColor)),e.sheenRoughness!==void 0&&(this.sheenRoughness=e.sheenRoughness),e.emissive!==void 0&&this.emissive!==void 0&&this.emissive.setHex(e.emissive),e.specular!==void 0&&this.specular!==void 0&&this.specular.setHex(e.specular),e.specularIntensity!==void 0&&(this.specularIntensity=e.specularIntensity),e.specularColor!==void 0&&this.specularColor!==void 0&&this.specularColor.setHex(e.specularColor),e.shininess!==void 0&&(this.shininess=e.shininess),e.clearcoat!==void 0&&(this.clearcoat=e.clearcoat),e.clearcoatRoughness!==void 0&&(this.clearcoatRoughness=e.clearcoatRoughness),e.dispersion!==void 0&&(this.dispersion=e.dispersion),e.iridescence!==void 0&&(this.iridescence=e.iridescence),e.iridescenceIOR!==void 0&&(this.iridescenceIOR=e.iridescenceIOR),e.iridescenceThicknessRange!==void 0&&(this.iridescenceThicknessRange=e.iridescenceThicknessRange),e.transmission!==void 0&&(this.transmission=e.transmission),e.thickness!==void 0&&(this.thickness=e.thickness),e.attenuationDistance!==void 0&&(this.attenuationDistance=e.attenuationDistance),e.attenuationColor!==void 0&&this.attenuationColor!==void 0&&this.attenuationColor.setHex(e.attenuationColor),e.anisotropy!==void 0&&(this.anisotropy=e.anisotropy),e.anisotropyRotation!==void 0&&(this.anisotropyRotation=e.anisotropyRotation),e.fog!==void 0&&(this.fog=e.fog),e.flatShading!==void 0&&(this.flatShading=e.flatShading),e.blending!==void 0&&(this.blending=e.blending),e.combine!==void 0&&(this.combine=e.combine),e.side!==void 0&&(this.side=e.side),e.shadowSide!==void 0&&(this.shadowSide=e.shadowSide),e.opacity!==void 0&&(this.opacity=e.opacity),e.transparent!==void 0&&(this.transparent=e.transparent),e.alphaTest!==void 0&&(this.alphaTest=e.alphaTest),e.alphaHash!==void 0&&(this.alphaHash=e.alphaHash),e.depthFunc!==void 0&&(this.depthFunc=e.depthFunc),e.depthTest!==void 0&&(this.depthTest=e.depthTest),e.depthWrite!==void 0&&(this.depthWrite=e.depthWrite),e.colorWrite!==void 0&&(this.colorWrite=e.colorWrite),e.blendSrc!==void 0&&(this.blendSrc=e.blendSrc),e.blendDst!==void 0&&(this.blendDst=e.blendDst),e.blendEquation!==void 0&&(this.blendEquation=e.blendEquation),e.blendSrcAlpha!==void 0&&(this.blendSrcAlpha=e.blendSrcAlpha),e.blendDstAlpha!==void 0&&(this.blendDstAlpha=e.blendDstAlpha),e.blendEquationAlpha!==void 0&&(this.blendEquationAlpha=e.blendEquationAlpha),e.blendColor!==void 0&&this.blendColor!==void 0&&this.blendColor.setHex(e.blendColor),e.blendAlpha!==void 0&&(this.blendAlpha=e.blendAlpha),e.stencilWriteMask!==void 0&&(this.stencilWriteMask=e.stencilWriteMask),e.stencilFunc!==void 0&&(this.stencilFunc=e.stencilFunc),e.stencilRef!==void 0&&(this.stencilRef=e.stencilRef),e.stencilFuncMask!==void 0&&(this.stencilFuncMask=e.stencilFuncMask),e.stencilFail!==void 0&&(this.stencilFail=e.stencilFail),e.stencilZFail!==void 0&&(this.stencilZFail=e.stencilZFail),e.stencilZPass!==void 0&&(this.stencilZPass=e.stencilZPass),e.stencilWrite!==void 0&&(this.stencilWrite=e.stencilWrite),e.wireframe!==void 0&&(this.wireframe=e.wireframe),e.wireframeLinewidth!==void 0&&(this.wireframeLinewidth=e.wireframeLinewidth),e.wireframeLinecap!==void 0&&(this.wireframeLinecap=e.wireframeLinecap),e.wireframeLinejoin!==void 0&&(this.wireframeLinejoin=e.wireframeLinejoin),e.rotation!==void 0&&(this.rotation=e.rotation),e.linewidth!==void 0&&(this.linewidth=e.linewidth),e.dashSize!==void 0&&(this.dashSize=e.dashSize),e.gapSize!==void 0&&(this.gapSize=e.gapSize),e.scale!==void 0&&(this.scale=e.scale),e.polygonOffset!==void 0&&(this.polygonOffset=e.polygonOffset),e.polygonOffsetFactor!==void 0&&(this.polygonOffsetFactor=e.polygonOffsetFactor),e.polygonOffsetUnits!==void 0&&(this.polygonOffsetUnits=e.polygonOffsetUnits),e.dithering!==void 0&&(this.dithering=e.dithering),e.alphaToCoverage!==void 0&&(this.alphaToCoverage=e.alphaToCoverage),e.premultipliedAlpha!==void 0&&(this.premultipliedAlpha=e.premultipliedAlpha),e.forceSinglePass!==void 0&&(this.forceSinglePass=e.forceSinglePass),e.allowOverride!==void 0&&(this.allowOverride=e.allowOverride),e.visible!==void 0&&(this.visible=e.visible),e.toneMapped!==void 0&&(this.toneMapped=e.toneMapped),e.userData!==void 0&&(this.userData=e.userData),e.vertexColors!==void 0&&(typeof e.vertexColors=="number"?this.vertexColors=e.vertexColors>0:this.vertexColors=e.vertexColors),e.size!==void 0&&(this.size=e.size),e.sizeAttenuation!==void 0&&(this.sizeAttenuation=e.sizeAttenuation),e.map!==void 0&&(this.map=t[e.map]||null),e.matcap!==void 0&&(this.matcap=t[e.matcap]||null),e.alphaMap!==void 0&&(this.alphaMap=t[e.alphaMap]||null),e.bumpMap!==void 0&&(this.bumpMap=t[e.bumpMap]||null),e.bumpScale!==void 0&&(this.bumpScale=e.bumpScale),e.normalMap!==void 0&&(this.normalMap=t[e.normalMap]||null),e.normalMapType!==void 0&&(this.normalMapType=e.normalMapType),e.normalScale!==void 0){let n=e.normalScale;Array.isArray(n)===!1&&(n=[n,n]),this.normalScale=new ye().fromArray(n)}return e.displacementMap!==void 0&&(this.displacementMap=t[e.displacementMap]||null),e.displacementScale!==void 0&&(this.displacementScale=e.displacementScale),e.displacementBias!==void 0&&(this.displacementBias=e.displacementBias),e.roughnessMap!==void 0&&(this.roughnessMap=t[e.roughnessMap]||null),e.metalnessMap!==void 0&&(this.metalnessMap=t[e.metalnessMap]||null),e.emissiveMap!==void 0&&(this.emissiveMap=t[e.emissiveMap]||null),e.emissiveIntensity!==void 0&&(this.emissiveIntensity=e.emissiveIntensity),e.specularMap!==void 0&&(this.specularMap=t[e.specularMap]||null),e.specularIntensityMap!==void 0&&(this.specularIntensityMap=t[e.specularIntensityMap]||null),e.specularColorMap!==void 0&&(this.specularColorMap=t[e.specularColorMap]||null),e.envMap!==void 0&&(this.envMap=t[e.envMap]||null),e.envMapRotation!==void 0&&this.envMapRotation.fromArray(e.envMapRotation),e.envMapIntensity!==void 0&&(this.envMapIntensity=e.envMapIntensity),e.reflectivity!==void 0&&(this.reflectivity=e.reflectivity),e.refractionRatio!==void 0&&(this.refractionRatio=e.refractionRatio),e.lightMap!==void 0&&(this.lightMap=t[e.lightMap]||null),e.lightMapIntensity!==void 0&&(this.lightMapIntensity=e.lightMapIntensity),e.aoMap!==void 0&&(this.aoMap=t[e.aoMap]||null),e.aoMapIntensity!==void 0&&(this.aoMapIntensity=e.aoMapIntensity),e.gradientMap!==void 0&&(this.gradientMap=t[e.gradientMap]||null),e.clearcoatMap!==void 0&&(this.clearcoatMap=t[e.clearcoatMap]||null),e.clearcoatRoughnessMap!==void 0&&(this.clearcoatRoughnessMap=t[e.clearcoatRoughnessMap]||null),e.clearcoatNormalMap!==void 0&&(this.clearcoatNormalMap=t[e.clearcoatNormalMap]||null),e.clearcoatNormalScale!==void 0&&(this.clearcoatNormalScale=new ye().fromArray(e.clearcoatNormalScale)),e.iridescenceMap!==void 0&&(this.iridescenceMap=t[e.iridescenceMap]||null),e.iridescenceThicknessMap!==void 0&&(this.iridescenceThicknessMap=t[e.iridescenceThicknessMap]||null),e.transmissionMap!==void 0&&(this.transmissionMap=t[e.transmissionMap]||null),e.thicknessMap!==void 0&&(this.thicknessMap=t[e.thicknessMap]||null),e.anisotropyMap!==void 0&&(this.anisotropyMap=t[e.anisotropyMap]||null),e.sheenColorMap!==void 0&&(this.sheenColorMap=t[e.sheenColorMap]||null),e.sheenRoughnessMap!==void 0&&(this.sheenRoughnessMap=t[e.sheenRoughnessMap]||null),this}clone(){return new this.constructor().copy(this)}copy(e){this.name=e.name,this.blending=e.blending,this.side=e.side,this.vertexColors=e.vertexColors,this.opacity=e.opacity,this.transparent=e.transparent,this.blendSrc=e.blendSrc,this.blendDst=e.blendDst,this.blendEquation=e.blendEquation,this.blendSrcAlpha=e.blendSrcAlpha,this.blendDstAlpha=e.blendDstAlpha,this.blendEquationAlpha=e.blendEquationAlpha,this.blendColor.copy(e.blendColor),this.blendAlpha=e.blendAlpha,this.depthFunc=e.depthFunc,this.depthTest=e.depthTest,this.depthWrite=e.depthWrite,this.stencilWriteMask=e.stencilWriteMask,this.stencilFunc=e.stencilFunc,this.stencilRef=e.stencilRef,this.stencilFuncMask=e.stencilFuncMask,this.stencilFail=e.stencilFail,this.stencilZFail=e.stencilZFail,this.stencilZPass=e.stencilZPass,this.stencilWrite=e.stencilWrite;let t=e.clippingPlanes,n=null;if(t!==null){let i=t.length;n=new Array(i);for(let r=0;r!==i;++r)n[r]=t[r].clone()}return this.clippingPlanes=n,this.clipIntersection=e.clipIntersection,this.clipShadows=e.clipShadows,this.shadowSide=e.shadowSide,this.colorWrite=e.colorWrite,this.precision=e.precision,this.polygonOffset=e.polygonOffset,this.polygonOffsetFactor=e.polygonOffsetFactor,this.polygonOffsetUnits=e.polygonOffsetUnits,this.dithering=e.dithering,this.alphaTest=e.alphaTest,this.alphaHash=e.alphaHash,this.alphaToCoverage=e.alphaToCoverage,this.premultipliedAlpha=e.premultipliedAlpha,this.forceSinglePass=e.forceSinglePass,this.allowOverride=e.allowOverride,this.visible=e.visible,this.toneMapped=e.toneMapped,this.userData=JSON.parse(JSON.stringify(e.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(e){e===!0&&this.version++}};var xi=new D,bc=new D,Ko=new D,zi=new D,Sc=new D,Jo=new D,Tc=new D,ni=class{constructor(e=new D,t=new D(0,0,-1)){this.origin=e,this.direction=t}set(e,t){return this.origin.copy(e),this.direction.copy(t),this}copy(e){return this.origin.copy(e.origin),this.direction.copy(e.direction),this}at(e,t){return t.copy(this.origin).addScaledVector(this.direction,e)}lookAt(e){return this.direction.copy(e).sub(this.origin).normalize(),this}recast(e){return this.origin.copy(this.at(e,xi)),this}closestPointToPoint(e,t){t.subVectors(e,this.origin);let n=t.dot(this.direction);return n<0?t.copy(this.origin):t.copy(this.origin).addScaledVector(this.direction,n)}distanceToPoint(e){return Math.sqrt(this.distanceSqToPoint(e))}distanceSqToPoint(e){let t=xi.subVectors(e,this.origin).dot(this.direction);return t<0?this.origin.distanceToSquared(e):(xi.copy(this.origin).addScaledVector(this.direction,t),xi.distanceToSquared(e))}distanceSqToSegment(e,t,n,i){bc.copy(e).add(t).multiplyScalar(.5),Ko.copy(t).sub(e).normalize(),zi.copy(this.origin).sub(bc);let r=e.distanceTo(t)*.5,o=-this.direction.dot(Ko),a=zi.dot(this.direction),l=-zi.dot(Ko),c=zi.lengthSq(),h=Math.abs(1-o*o),u,f,d,g;if(h>0)if(u=o*l-a,f=o*a-l,g=r*h,u>=0)if(f>=-g)if(f<=g){let M=1/h;u*=M,f*=M,d=u*(u+o*f+2*a)+f*(o*u+f+2*l)+c}else f=r,u=Math.max(0,-(o*f+a)),d=-u*u+f*(f+2*l)+c;else f=-r,u=Math.max(0,-(o*f+a)),d=-u*u+f*(f+2*l)+c;else f<=-g?(u=Math.max(0,-(-o*r+a)),f=u>0?-r:Math.min(Math.max(-r,-l),r),d=-u*u+f*(f+2*l)+c):f<=g?(u=0,f=Math.min(Math.max(-r,-l),r),d=f*(f+2*l)+c):(u=Math.max(0,-(o*r+a)),f=u>0?r:Math.min(Math.max(-r,-l),r),d=-u*u+f*(f+2*l)+c);else f=o>0?-r:r,u=Math.max(0,-(o*f+a)),d=-u*u+f*(f+2*l)+c;return n&&n.copy(this.origin).addScaledVector(this.direction,u),i&&i.copy(bc).addScaledVector(Ko,f),d}intersectSphere(e,t){xi.subVectors(e.center,this.origin);let n=xi.dot(this.direction),i=xi.dot(xi)-n*n,r=e.radius*e.radius;if(i>r)return null;let o=Math.sqrt(r-i),a=n-o,l=n+o;return l<0?null:a<0?this.at(l,t):this.at(a,t)}intersectsSphere(e){return e.radius<0?!1:this.distanceSqToPoint(e.center)<=e.radius*e.radius}distanceToPlane(e){let t=e.normal.dot(this.direction);if(t===0)return e.distanceToPoint(this.origin)===0?0:null;let n=-(this.origin.dot(e.normal)+e.constant)/t;return n>=0?n:null}intersectPlane(e,t){let n=this.distanceToPlane(e);return n===null?null:this.at(n,t)}intersectsPlane(e){let t=e.distanceToPoint(this.origin);return t===0||e.normal.dot(this.direction)*t<0}intersectBox(e,t){let n,i,r,o,a,l,c=1/this.direction.x,h=1/this.direction.y,u=1/this.direction.z,f=this.origin;return c>=0?(n=(e.min.x-f.x)*c,i=(e.max.x-f.x)*c):(n=(e.max.x-f.x)*c,i=(e.min.x-f.x)*c),h>=0?(r=(e.min.y-f.y)*h,o=(e.max.y-f.y)*h):(r=(e.max.y-f.y)*h,o=(e.min.y-f.y)*h),n>o||r>i||((r>n||isNaN(n))&&(n=r),(o<i||isNaN(i))&&(i=o),u>=0?(a=(e.min.z-f.z)*u,l=(e.max.z-f.z)*u):(a=(e.max.z-f.z)*u,l=(e.min.z-f.z)*u),n>l||a>i)||((a>n||n!==n)&&(n=a),(l<i||i!==i)&&(i=l),i<0)?null:this.at(n>=0?n:i,t)}intersectsBox(e){return this.intersectBox(e,xi)!==null}intersectTriangle(e,t,n,i,r){Sc.subVectors(t,e),Jo.subVectors(n,e),Tc.crossVectors(Sc,Jo);let o=this.direction.dot(Tc),a;if(o>0){if(i)return null;a=1}else if(o<0)a=-1,o=-o;else return null;zi.subVectors(this.origin,e);let l=a*this.direction.dot(Jo.crossVectors(zi,Jo));if(l<0)return null;let c=a*this.direction.dot(Sc.cross(zi));if(c<0||l+c>o)return null;let h=-a*zi.dot(Tc);return h<0?null:this.at(h/o,r)}applyMatrix4(e){return this.origin.applyMatrix4(e),this.direction.transformDirection(e),this}equals(e){return e.origin.equals(this.origin)&&e.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},Tt=class extends un{constructor(e){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new Me(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Un,this.combine=ja,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.fog=e.fog,this}},Au=new Ke,rs=new ni,jo=new nn,Ru=new D,$o=new D,Qo=new D,ea=new D,Ec=new D,ta=new D,Cu=new D,na=new D,Je=class extends yt{constructor(e=new ft,t=new Tt){super(),this.isMesh=!0,this.type="Mesh",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.count=1,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),e.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=e.morphTargetInfluences.slice()),e.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},e.morphTargetDictionary)),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}getVertexPosition(e,t){let n=this.geometry,i=n.attributes.position,r=n.morphAttributes.position,o=n.morphTargetsRelative;t.fromBufferAttribute(i,e);let a=this.morphTargetInfluences;if(r&&a){ta.set(0,0,0);for(let l=0,c=r.length;l<c;l++){let h=a[l],u=r[l];h!==0&&(Ec.fromBufferAttribute(u,e),o?ta.addScaledVector(Ec,h):ta.addScaledVector(Ec.sub(t),h))}t.add(ta)}return t}raycast(e,t){let n=this.geometry,i=this.material,r=this.matrixWorld;i!==void 0&&(n.boundingSphere===null&&n.computeBoundingSphere(),jo.copy(n.boundingSphere),jo.applyMatrix4(r),rs.copy(e.ray).recast(e.near),!(jo.containsPoint(rs.origin)===!1&&(rs.intersectSphere(jo,Ru)===null||rs.origin.distanceToSquared(Ru)>(e.far-e.near)**2))&&(Au.copy(r).invert(),rs.copy(e.ray).applyMatrix4(Au),!(n.boundingBox!==null&&rs.intersectsBox(n.boundingBox)===!1)&&this._computeIntersections(e,t,rs)))}_computeIntersections(e,t,n){let i,r=this.geometry,o=this.material,a=r.index,l=r.attributes.position,c=r.attributes.uv,h=r.attributes.uv1,u=r.attributes.normal,f=r.groups,d=r.drawRange;if(a!==null)if(Array.isArray(o))for(let g=0,M=f.length;g<M;g++){let m=f[g],p=o[m.materialIndex],S=Math.max(m.start,d.start),R=Math.min(a.count,Math.min(m.start+m.count,d.start+d.count));for(let b=S,w=R;b<w;b+=3){let y=a.getX(b),C=a.getX(b+1),_=a.getX(b+2);i=ia(this,p,e,n,c,h,u,y,C,_),i&&(i.faceIndex=Math.floor(b/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,d.start),M=Math.min(a.count,d.start+d.count);for(let m=g,p=M;m<p;m+=3){let S=a.getX(m),R=a.getX(m+1),b=a.getX(m+2);i=ia(this,o,e,n,c,h,u,S,R,b),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}else if(l!==void 0)if(Array.isArray(o))for(let g=0,M=f.length;g<M;g++){let m=f[g],p=o[m.materialIndex],S=Math.max(m.start,d.start),R=Math.min(l.count,Math.min(m.start+m.count,d.start+d.count));for(let b=S,w=R;b<w;b+=3){let y=b,C=b+1,_=b+2;i=ia(this,p,e,n,c,h,u,y,C,_),i&&(i.faceIndex=Math.floor(b/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,d.start),M=Math.min(l.count,d.start+d.count);for(let m=g,p=M;m<p;m+=3){let S=m,R=m+1,b=m+2;i=ia(this,o,e,n,c,h,u,S,R,b),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}}};function Op(s,e,t,n,i,r,o,a){let l;if(e.side===Wt?l=n.intersectTriangle(o,r,i,!0,a):l=n.intersectTriangle(i,r,o,e.side===Sn,a),l===null)return null;na.copy(a),na.applyMatrix4(s.matrixWorld);let c=t.ray.origin.distanceTo(na);return c<t.near||c>t.far?null:{distance:c,point:na.clone(),object:s}}function ia(s,e,t,n,i,r,o,a,l,c){s.getVertexPosition(a,$o),s.getVertexPosition(l,Qo),s.getVertexPosition(c,ea);let h=Op(s,e,t,n,$o,Qo,ea,Cu);if(h){let u=new D;Hi.getBarycoord(Cu,$o,Qo,ea,u),i&&(h.uv=Hi.getInterpolatedAttribute(i,a,l,c,u,new ye)),r&&(h.uv1=Hi.getInterpolatedAttribute(r,a,l,c,u,new ye)),o&&(h.normal=Hi.getInterpolatedAttribute(o,a,l,c,u,new D),h.normal.dot(n.direction)>0&&h.normal.multiplyScalar(-1));let f={a,b:l,c,normal:new D,materialIndex:0};Hi.getNormal($o,Qo,ea,f.normal),h.face=f,h.barycoord=u}return h}var Ir=new xt,Pu=new xt,Iu=new xt,Bp=new xt,Lu=new Ke,sa=new D,wc=new nn,Du=new Ke,Ac=new ni,qr=class extends Je{constructor(e,t){super(e,t),this.isSkinnedMesh=!0,this.type="SkinnedMesh",this.bindMode=Uc,this.bindMatrix=new Ke,this.bindMatrixInverse=new Ke,this.boundingBox=null,this.boundingSphere=null}computeBoundingBox(){let e=this.geometry;this.boundingBox===null&&(this.boundingBox=new hn),this.boundingBox.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,sa),this.boundingBox.expandByPoint(sa)}computeBoundingSphere(){let e=this.geometry;this.boundingSphere===null&&(this.boundingSphere=new nn),this.boundingSphere.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,sa),this.boundingSphere.expandByPoint(sa)}copy(e,t){return super.copy(e,t),this.bindMode=e.bindMode,this.bindMatrix.copy(e.bindMatrix),this.bindMatrixInverse.copy(e.bindMatrixInverse),this.skeleton=e.skeleton,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}raycast(e,t){let n=this.material,i=this.matrixWorld;n!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),wc.copy(this.boundingSphere),wc.applyMatrix4(i),e.ray.intersectsSphere(wc)!==!1&&(Du.copy(i).invert(),Ac.copy(e.ray).applyMatrix4(Du),!(this.boundingBox!==null&&Ac.intersectsBox(this.boundingBox)===!1)&&this._computeIntersections(e,t,Ac)))}getVertexPosition(e,t){return super.getVertexPosition(e,t),this.applyBoneTransform(e,t),t}bind(e,t){this.skeleton=e,t===void 0&&(this.updateMatrixWorld(!0),this.skeleton.calculateInverses(),t=this.matrixWorld),this.bindMatrix.copy(t),this.bindMatrixInverse.copy(t).invert()}pose(){this.skeleton.pose()}normalizeSkinWeights(){let e=new xt,t=this.geometry.attributes.skinWeight;for(let n=0,i=t.count;n<i;n++){e.fromBufferAttribute(t,n);let r=1/e.manhattanLength();r!==1/0?e.multiplyScalar(r):e.set(1,0,0,0),t.setXYZW(n,e.x,e.y,e.z,e.w)}}updateMatrixWorld(e){super.updateMatrixWorld(e),this.bindMode===Uc?this.bindMatrixInverse.copy(this.matrixWorld).invert():this.bindMode===wf?this.bindMatrixInverse.copy(this.bindMatrix).invert():Be("SkinnedMesh: Unrecognized bindMode: "+this.bindMode)}applyBoneTransform(e,t){let n=this.skeleton,i=this.geometry;Pu.fromBufferAttribute(i.attributes.skinIndex,e),Iu.fromBufferAttribute(i.attributes.skinWeight,e),t.isVector4?(Ir.copy(t),t.set(0,0,0,0)):(Ir.set(...t,1),t.set(0,0,0)),Ir.applyMatrix4(this.bindMatrix);for(let r=0;r<4;r++){let o=Iu.getComponent(r);if(o!==0){let a=Pu.getComponent(r);Lu.multiplyMatrices(n.bones[a].matrixWorld,n.boneInverses[a]),t.addScaledVector(Bp.copy(Ir).applyMatrix4(Lu),o)}}return t.isVector4&&(t.w=Ir.w),t.applyMatrix4(this.bindMatrixInverse)}},tr=class extends yt{constructor(){super(),this.isBone=!0,this.type="Bone"}},nr=class extends Gt{constructor(e=null,t=1,n=1,i,r,o,a,l,c=Ot,h=Ot,u,f){super(null,o,a,l,c,h,i,r,u,f),this.isDataTexture=!0,this.image={data:e,width:t,height:n},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}},Nu=new Ke,zp=new Ke,Yr=class s{constructor(e=[],t=[]){this.uuid=Wn(),this.bones=e.slice(0),this.boneInverses=t,this.boneMatrices=null,this.boneTexture=null,this.init()}init(){let e=this.bones,t=this.boneInverses;if(this.boneMatrices=new Float32Array(e.length*16),t.length===0)this.calculateInverses();else if(e.length!==t.length){Be("Skeleton: Number of inverse bone matrices does not match amount of bones."),this.boneInverses=[];for(let n=0,i=this.bones.length;n<i;n++)this.boneInverses.push(new Ke)}}calculateInverses(){this.boneInverses.length=0;for(let e=0,t=this.bones.length;e<t;e++){let n=new Ke;this.bones[e]&&n.copy(this.bones[e].matrixWorld).invert(),this.boneInverses.push(n)}}pose(){for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&n.matrixWorld.copy(this.boneInverses[e]).invert()}for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&(n.parent&&n.parent.isBone?(n.matrix.copy(n.parent.matrixWorld).invert(),n.matrix.multiply(n.matrixWorld)):n.matrix.copy(n.matrixWorld),n.matrix.decompose(n.position,n.quaternion,n.scale))}}update(){let e=this.bones,t=this.boneInverses,n=this.boneMatrices,i=this.boneTexture;for(let r=0,o=e.length;r<o;r++){let a=e[r]?e[r].matrixWorld:zp;Nu.multiplyMatrices(a,t[r]),Nu.toArray(n,r*16)}i!==null&&(i.needsUpdate=!0)}clone(){return new s(this.bones,this.boneInverses)}computeBoneTexture(){let e=Math.sqrt(this.bones.length*4);e=Math.ceil(e/4)*4,e=Math.max(e,4);let t=new Float32Array(e*e*4);t.set(this.boneMatrices);let n=new nr(t,e,e,wn,En);return n.needsUpdate=!0,this.boneMatrices=t,this.boneTexture=n,this}getBoneByName(e){for(let t=0,n=this.bones.length;t<n;t++){let i=this.bones[t];if(i.name===e)return i}}dispose(){this.boneTexture!==null&&(this.boneTexture.dispose(),this.boneTexture=null)}fromJSON(e,t){this.uuid=e.uuid;for(let n=0,i=e.bones.length;n<i;n++){let r=e.bones[n],o=t[r];o===void 0&&(Be("Skeleton: No bone found with UUID:",r),o=new tr),this.bones.push(o),this.boneInverses.push(new Ke().fromArray(e.boneInverses[n]))}return this.init(),this}toJSON(){let e={metadata:{version:4.7,type:"Skeleton",generator:"Skeleton.toJSON"},bones:[],boneInverses:[]};e.uuid=this.uuid;let t=this.bones,n=this.boneInverses;for(let i=0,r=t.length;i<r;i++){let o=t[i];e.bones.push(o.uuid);let a=n[i];e.boneInverses.push(a.toArray())}return e}},Wi=class extends vt{constructor(e,t,n,i=1){super(e,t,n),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=i}copy(e){return super.copy(e),this.meshPerAttribute=e.meshPerAttribute,this}toJSON(){let e=super.toJSON();return e.meshPerAttribute=this.meshPerAttribute,e.isInstancedBufferAttribute=!0,e}},Hs=new Ke,Uu=new Ke,ra=[],Fu=new hn,kp=new Ke,Lr=new Je,Dr=new nn,fn=class extends Je{constructor(e,t,n){super(e,t),this.isInstancedMesh=!0,this.instanceMatrix=new Wi(new Float32Array(n*16),16),this.instanceColor=null,this.morphTexture=null,this.count=n,this.boundingBox=null,this.boundingSphere=null;for(let i=0;i<n;i++)this.setMatrixAt(i,kp)}computeBoundingBox(){let e=this.geometry,t=this.count;this.boundingBox===null&&(this.boundingBox=new hn),e.boundingBox===null&&e.computeBoundingBox(),this.boundingBox.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,Hs),Fu.copy(e.boundingBox).applyMatrix4(Hs),this.boundingBox.union(Fu)}computeBoundingSphere(){let e=this.geometry,t=this.count;this.boundingSphere===null&&(this.boundingSphere=new nn),e.boundingSphere===null&&e.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,Hs),Dr.copy(e.boundingSphere).applyMatrix4(Hs),this.boundingSphere.union(Dr)}copy(e,t){return super.copy(e,t),this.instanceMatrix.copy(e.instanceMatrix),e.morphTexture!==null&&(this.morphTexture=e.morphTexture.clone()),e.instanceColor!==null&&(this.instanceColor=e.instanceColor.clone()),this.count=e.count,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}getColorAt(e,t){return this.instanceColor===null?t.setRGB(1,1,1):t.fromArray(this.instanceColor.array,e*3)}getMatrixAt(e,t){return t.fromArray(this.instanceMatrix.array,e*16)}getMorphAt(e,t){let n=t.morphTargetInfluences,i=this.morphTexture.source.data.data,r=n.length+1,o=e*r+1;for(let a=0;a<n.length;a++)n[a]=i[o+a]}raycast(e,t){let n=this.matrixWorld,i=this.count;if(Lr.geometry=this.geometry,Lr.material=this.material,Lr.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Dr.copy(this.boundingSphere),Dr.applyMatrix4(n),e.ray.intersectsSphere(Dr)!==!1))for(let r=0;r<i;r++){this.getMatrixAt(r,Hs),Uu.multiplyMatrices(n,Hs),Lr.matrixWorld=Uu,Lr.raycast(e,ra);for(let o=0,a=ra.length;o<a;o++){let l=ra[o];l.instanceId=r,l.object=this,t.push(l)}ra.length=0}}setColorAt(e,t){return this.instanceColor===null&&(this.instanceColor=new Wi(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),t.toArray(this.instanceColor.array,e*3),this}setMatrixAt(e,t){return t.toArray(this.instanceMatrix.array,e*16),this}setMorphAt(e,t){let n=t.morphTargetInfluences,i=n.length+1;this.morphTexture===null&&(this.morphTexture=new nr(new Float32Array(i*this.count),i,this.count,sl,En));let r=this.morphTexture.source.data.data,o=0;for(let c=0;c<n.length;c++)o+=n[c];let a=this.geometry.morphTargetsRelative?1:1-o,l=i*e;return r[l]=a,r.set(n,l+1),this}updateMorphTargets(){}dispose(){this.dispatchEvent({type:"dispose"}),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null)}},Rc=new D,Hp=new D,Vp=new $e,Ln=class{constructor(e=new D(1,0,0),t=0){this.isPlane=!0,this.normal=e,this.constant=t}set(e,t){return this.normal.copy(e),this.constant=t,this}setComponents(e,t,n,i){return this.normal.set(e,t,n),this.constant=i,this}setFromNormalAndCoplanarPoint(e,t){return this.normal.copy(e),this.constant=-t.dot(this.normal),this}setFromCoplanarPoints(e,t,n){let i=Rc.subVectors(n,t).cross(Hp.subVectors(e,t)).normalize();return this.setFromNormalAndCoplanarPoint(i,e),this}copy(e){return this.normal.copy(e.normal),this.constant=e.constant,this}normalize(){let e=1/this.normal.length();return this.normal.multiplyScalar(e),this.constant*=e,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(e){return this.normal.dot(e)+this.constant}distanceToSphere(e){return this.distanceToPoint(e.center)-e.radius}projectPoint(e,t){return t.copy(e).addScaledVector(this.normal,-this.distanceToPoint(e))}intersectLine(e,t,n=!0){let i=e.delta(Rc),r=this.normal.dot(i);if(r===0)return this.distanceToPoint(e.start)===0?t.copy(e.start):null;let o=-(e.start.dot(this.normal)+this.constant)/r;return n===!0&&(o<0||o>1)?null:t.copy(e.start).addScaledVector(i,o)}intersectsLine(e){let t=this.distanceToPoint(e.start),n=this.distanceToPoint(e.end);return t<0&&n>0||n<0&&t>0}intersectsBox(e){return e.intersectsPlane(this)}intersectsSphere(e){return e.intersectsPlane(this)}coplanarPoint(e){return e.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(e,t){let n=t||Vp.getNormalMatrix(e),i=this.coplanarPoint(Rc).applyMatrix4(e),r=this.normal.applyMatrix3(n).normalize();return this.constant=-i.dot(r),this}translate(e){return this.constant-=e.dot(this.normal),this}equals(e){return e.normal.equals(this.normal)&&e.constant===this.constant}clone(){return new this.constructor().copy(this)}},os=new nn,Gp=new ye(.5,.5),oa=new D,ir=class{constructor(e=new Ln,t=new Ln,n=new Ln,i=new Ln,r=new Ln,o=new Ln){this.planes=[e,t,n,i,r,o]}set(e,t,n,i,r,o){let a=this.planes;return a[0].copy(e),a[1].copy(t),a[2].copy(n),a[3].copy(i),a[4].copy(r),a[5].copy(o),this}copy(e){let t=this.planes;for(let n=0;n<6;n++)t[n].copy(e.planes[n]);return this}setFromProjectionMatrix(e,t=Gn,n=!1){let i=this.planes,r=e.elements,o=r[0],a=r[1],l=r[2],c=r[3],h=r[4],u=r[5],f=r[6],d=r[7],g=r[8],M=r[9],m=r[10],p=r[11],S=r[12],R=r[13],b=r[14],w=r[15];if(i[0].setComponents(c-o,d-h,p-g,w-S).normalize(),i[1].setComponents(c+o,d+h,p+g,w+S).normalize(),i[2].setComponents(c+a,d+u,p+M,w+R).normalize(),i[3].setComponents(c-a,d-u,p-M,w-R).normalize(),n)i[4].setComponents(l,f,m,b).normalize(),i[5].setComponents(c-l,d-f,p-m,w-b).normalize();else if(i[4].setComponents(c-l,d-f,p-m,w-b).normalize(),t===Gn)i[5].setComponents(c+l,d+f,p+m,w+b).normalize();else if(t===Ys)i[5].setComponents(l,f,m,b).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+t);return this}intersectsObject(e){if(e.boundingSphere!==void 0)e.boundingSphere===null&&e.computeBoundingSphere(),os.copy(e.boundingSphere).applyMatrix4(e.matrixWorld);else{let t=e.geometry;t.boundingSphere===null&&t.computeBoundingSphere(),os.copy(t.boundingSphere).applyMatrix4(e.matrixWorld)}return this.intersectsSphere(os)}intersectsSprite(e){os.center.set(0,0,0);let t=Gp.distanceTo(e.center);return os.radius=.7071067811865476+t,os.applyMatrix4(e.matrixWorld),this.intersectsSphere(os)}intersectsSphere(e){let t=this.planes,n=e.center,i=-e.radius;for(let r=0;r<6;r++)if(t[r].distanceToPoint(n)<i)return!1;return!0}intersectsBox(e){let t=this.planes;for(let n=0;n<6;n++){let i=t[n];if(oa.x=i.normal.x>0?e.max.x:e.min.x,oa.y=i.normal.y>0?e.max.y:e.min.y,oa.z=i.normal.z>0?e.max.z:e.min.z,i.distanceToPoint(oa)<0)return!1}return!0}containsPoint(e){let t=this.planes;for(let n=0;n<6;n++)if(t[n].distanceToPoint(e)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};var ii=class extends un{constructor(e){super(),this.isLineBasicMaterial=!0,this.type="LineBasicMaterial",this.color=new Me(16777215),this.map=null,this.linewidth=1,this.linecap="round",this.linejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.linewidth=e.linewidth,this.linecap=e.linecap,this.linejoin=e.linejoin,this.fog=e.fog,this}},Ra=new D,Ca=new D,Ou=new Ke,Nr=new ni,aa=new nn,Cc=new D,Bu=new D,Mi=class extends yt{constructor(e=new ft,t=new ii){super(),this.isLine=!0,this.type="Line",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[0];for(let i=1,r=t.count;i<r;i++)Ra.fromBufferAttribute(t,i-1),Ca.fromBufferAttribute(t,i),n[i]=n[i-1],n[i]+=Ra.distanceTo(Ca);e.setAttribute("lineDistance",new ot(n,1))}else Be("Line.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Line.threshold,o=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),aa.copy(n.boundingSphere),aa.applyMatrix4(i),aa.radius+=r,e.ray.intersectsSphere(aa)===!1)return;Ou.copy(i).invert(),Nr.copy(e.ray).applyMatrix4(Ou);let a=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=a*a,c=this.isLineSegments?2:1,h=n.index,f=n.attributes.position;if(h!==null){let d=Math.max(0,o.start),g=Math.min(h.count,o.start+o.count);for(let M=d,m=g-1;M<m;M+=c){let p=h.getX(M),S=h.getX(M+1),R=la(this,e,Nr,l,p,S,M);R&&t.push(R)}if(this.isLineLoop){let M=h.getX(g-1),m=h.getX(d),p=la(this,e,Nr,l,M,m,g-1);p&&t.push(p)}}else{let d=Math.max(0,o.start),g=Math.min(f.count,o.start+o.count);for(let M=d,m=g-1;M<m;M+=c){let p=la(this,e,Nr,l,M,M+1,M);p&&t.push(p)}if(this.isLineLoop){let M=la(this,e,Nr,l,g-1,d,g-1);M&&t.push(M)}}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}};function la(s,e,t,n,i,r,o){let a=s.geometry.attributes.position;if(Ra.fromBufferAttribute(a,i),Ca.fromBufferAttribute(a,r),t.distanceSqToSegment(Ra,Ca,Cc,Bu)>n)return;Cc.applyMatrix4(s.matrixWorld);let c=e.ray.origin.distanceTo(Cc);if(!(c<e.near||c>e.far))return{distance:c,point:Bu.clone().applyMatrix4(s.matrixWorld),index:o,face:null,faceIndex:null,barycoord:null,object:s}}var zu=new D,ku=new D,_s=class extends Mi{constructor(e,t){super(e,t),this.isLineSegments=!0,this.type="LineSegments"}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[];for(let i=0,r=t.count;i<r;i+=2)zu.fromBufferAttribute(t,i),ku.fromBufferAttribute(t,i+1),n[i]=i===0?0:n[i-1],n[i+1]=n[i]+zu.distanceTo(ku);e.setAttribute("lineDistance",new ot(n,1))}else Be("LineSegments.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}},Zr=class extends Mi{constructor(e,t){super(e,t),this.isLineLoop=!0,this.type="LineLoop"}},sr=class extends un{constructor(e){super(),this.isPointsMaterial=!0,this.type="PointsMaterial",this.color=new Me(16777215),this.map=null,this.alphaMap=null,this.size=1,this.sizeAttenuation=!0,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.alphaMap=e.alphaMap,this.size=e.size,this.sizeAttenuation=e.sizeAttenuation,this.fog=e.fog,this}},Hu=new Ke,Bc=new ni,ca=new nn,ha=new D,Fn=class extends yt{constructor(e=new ft,t=new sr){super(),this.isPoints=!0,this.type="Points",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Points.threshold,o=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),ca.copy(n.boundingSphere),ca.applyMatrix4(i),ca.radius+=r,e.ray.intersectsSphere(ca)===!1)return;Hu.copy(i).invert(),Bc.copy(e.ray).applyMatrix4(Hu);let a=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=a*a,c=n.index,u=n.attributes.position;if(c!==null){let f=Math.max(0,o.start),d=Math.min(c.count,o.start+o.count);for(let g=f,M=d;g<M;g++){let m=c.getX(g);ha.fromBufferAttribute(u,m),Vu(ha,m,l,i,e,t,this)}}else{let f=Math.max(0,o.start),d=Math.min(u.count,o.start+o.count);for(let g=f,M=d;g<M;g++)ha.fromBufferAttribute(u,g),Vu(ha,g,l,i,e,t,this)}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}};function Vu(s,e,t,n,i,r,o){let a=Bc.distanceSqToPoint(s);if(a<t){let l=new D;Bc.closestPointToPoint(s,l),l.applyMatrix4(n);let c=i.ray.origin.distanceTo(l);if(c<i.near||c>i.far)return;r.push({distance:c,distanceToRay:Math.sqrt(a),point:l,index:e,face:null,faceIndex:null,barycoord:null,object:o})}}var Kr=class extends Gt{constructor(e=[],t=Ji,n,i,r,o,a,l,c,h){super(e,t,n,i,r,o,a,l,c,h),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(e){this.image=e}},xs=class extends Gt{constructor(e,t,n,i,r,o,a,l,c){super(e,t,n,i,r,o,a,l,c),this.isCanvasTexture=!0,this.needsUpdate=!0}};var bi=class extends Gt{constructor(e,t,n=Zn,i,r,o,a=Ot,l=Ot,c,h=ti,u=1){if(h!==ti&&h!==ji)throw new Error("THREE.DepthTexture: format must be either THREE.DepthFormat or THREE.DepthStencilFormat");let f={width:e,height:t,depth:u};super(f,i,r,o,a,l,h,n,c),this.isDepthTexture=!0,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(e){return super.copy(e),this.source=new Js(Object.assign({},e.image)),this.compareFunction=e.compareFunction,this}toJSON(e){let t=super.toJSON(e);return this.compareFunction!==null&&(t.compareFunction=this.compareFunction),t}},Pa=class extends bi{constructor(e,t=Zn,n=Ji,i,r,o=Ot,a=Ot,l,c=ti){let h={width:e,height:e,depth:1},u=[h,h,h,h,h,h];super(e,e,t,n,i,r,o,a,l,c),this.image=u,this.isCubeDepthTexture=!0,this.isCubeTexture=!0}get images(){return this.image}set images(e){this.image=e}},Jr=class extends Gt{constructor(e=null){super(),this.sourceTexture=e,this.isExternalTexture=!0}copy(e){return super.copy(e),this.sourceTexture=e.sourceTexture,this}},si=class s extends ft{constructor(e=1,t=1,n=1,i=1,r=1,o=1){super(),this.type="BoxGeometry",this.parameters={width:e,height:t,depth:n,widthSegments:i,heightSegments:r,depthSegments:o};let a=this;i=Math.floor(i),r=Math.floor(r),o=Math.floor(o);let l=[],c=[],h=[],u=[],f=0,d=0;g("z","y","x",-1,-1,n,t,e,o,r,0),g("z","y","x",1,-1,n,t,-e,o,r,1),g("x","z","y",1,1,e,n,t,i,o,2),g("x","z","y",1,-1,e,n,-t,i,o,3),g("x","y","z",1,-1,e,t,n,i,r,4),g("x","y","z",-1,-1,e,t,-n,i,r,5),this.setIndex(l),this.setAttribute("position",new ot(c,3)),this.setAttribute("normal",new ot(h,3)),this.setAttribute("uv",new ot(u,2));function g(M,m,p,S,R,b,w,y,C,_,v){let E=b/C,P=w/_,L=b/2,H=w/2,O=y/2,U=C+1,V=_+1,F=0,Y=0,J=new D;for(let j=0;j<V;j++){let me=j*P-H;for(let _e=0;_e<U;_e++){let Ve=_e*E-L;J[M]=Ve*S,J[m]=me*R,J[p]=O,c.push(J.x,J.y,J.z),J[M]=0,J[m]=0,J[p]=y>0?1:-1,h.push(J.x,J.y,J.z),u.push(_e/C),u.push(1-j/_),F+=1}}for(let j=0;j<_;j++)for(let me=0;me<C;me++){let _e=f+me+U*j,Ve=f+me+U*(j+1),it=f+(me+1)+U*(j+1),Fe=f+(me+1)+U*j;l.push(_e,Ve,Fe),l.push(Ve,it,Fe),Y+=6}a.addGroup(d,Y,v),d+=Y,f+=F}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.depth,e.widthSegments,e.heightSegments,e.depthSegments)}};var rr=class s extends ft{constructor(e=1,t=1,n=1,i=32,r=1,o=!1,a=0,l=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:e,radiusBottom:t,height:n,radialSegments:i,heightSegments:r,openEnded:o,thetaStart:a,thetaLength:l};let c=this;i=Math.floor(i),r=Math.floor(r);let h=[],u=[],f=[],d=[],g=0,M=[],m=n/2,p=0;S(),o===!1&&(e>0&&R(!0),t>0&&R(!1)),this.setIndex(h),this.setAttribute("position",new ot(u,3)),this.setAttribute("normal",new ot(f,3)),this.setAttribute("uv",new ot(d,2));function S(){let b=new D,w=new D,y=0,C=(t-e)/n;for(let _=0;_<=r;_++){let v=[],E=_/r,P=E*(t-e)+e;for(let L=0;L<=i;L++){let H=L/i,O=H*l+a,U=Math.sin(O),V=Math.cos(O);w.x=P*U,w.y=-E*n+m,w.z=P*V,u.push(w.x,w.y,w.z),b.set(U,C,V).normalize(),f.push(b.x,b.y,b.z),d.push(H,1-E),v.push(g++)}M.push(v)}for(let _=0;_<i;_++)for(let v=0;v<r;v++){let E=M[v][_],P=M[v+1][_],L=M[v+1][_+1],H=M[v][_+1];(e>0||v!==0)&&(h.push(E,P,H),y+=3),(t>0||v!==r-1)&&(h.push(P,L,H),y+=3)}c.addGroup(p,y,0),p+=y}function R(b){let w=g,y=new ye,C=new D,_=0,v=b===!0?e:t,E=b===!0?1:-1;for(let L=1;L<=i;L++)u.push(0,m*E,0),f.push(0,E,0),d.push(.5,.5),g++;let P=g;for(let L=0;L<=i;L++){let O=L/i*l+a,U=Math.cos(O),V=Math.sin(O);C.x=v*V,C.y=m*E,C.z=v*U,u.push(C.x,C.y,C.z),f.push(0,E,0),y.x=U*.5+.5,y.y=V*.5*E+.5,d.push(y.x,y.y),g++}for(let L=0;L<i;L++){let H=w+L,O=P+L;b===!0?h.push(O,O+1,H):h.push(O+1,O,H),_+=3}c.addGroup(p,_,b===!0?1:2),p+=_}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radiusTop,e.radiusBottom,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},vs=class s extends rr{constructor(e=1,t=1,n=32,i=1,r=!1,o=0,a=Math.PI*2){super(0,e,t,n,i,r,o,a),this.type="ConeGeometry",this.parameters={radius:e,height:t,radialSegments:n,heightSegments:i,openEnded:r,thetaStart:o,thetaLength:a}}static fromJSON(e){return new s(e.radius,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Ia=class s extends ft{constructor(e=[],t=[],n=1,i=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:e,indices:t,radius:n,detail:i};let r=[],o=[];a(i),c(n),h(),this.setAttribute("position",new ot(r,3)),this.setAttribute("normal",new ot(r.slice(),3)),this.setAttribute("uv",new ot(o,2)),i===0?this.computeVertexNormals():this.normalizeNormals();function a(S){let R=new D,b=new D,w=new D;for(let y=0;y<t.length;y+=3)d(t[y+0],R),d(t[y+1],b),d(t[y+2],w),l(R,b,w,S)}function l(S,R,b,w){let y=w+1,C=[];for(let _=0;_<=y;_++){C[_]=[];let v=S.clone().lerp(b,_/y),E=R.clone().lerp(b,_/y),P=y-_;for(let L=0;L<=P;L++)L===0&&_===y?C[_][L]=v:C[_][L]=v.clone().lerp(E,L/P)}for(let _=0;_<y;_++)for(let v=0;v<2*(y-_)-1;v++){let E=Math.floor(v/2);v%2===0?(f(C[_][E+1]),f(C[_+1][E]),f(C[_][E])):(f(C[_][E+1]),f(C[_+1][E+1]),f(C[_+1][E]))}}function c(S){let R=new D;for(let b=0;b<r.length;b+=3)R.x=r[b+0],R.y=r[b+1],R.z=r[b+2],R.normalize().multiplyScalar(S),r[b+0]=R.x,r[b+1]=R.y,r[b+2]=R.z}function h(){let S=new D;for(let R=0;R<r.length;R+=3){S.x=r[R+0],S.y=r[R+1],S.z=r[R+2];let b=m(S)/2/Math.PI+.5,w=p(S)/Math.PI+.5;o.push(b,1-w)}g(),u()}function u(){for(let S=0;S<o.length;S+=6){let R=o[S+0],b=o[S+2],w=o[S+4],y=Math.max(R,b,w),C=Math.min(R,b,w);y>.9&&C<.1&&(R<.2&&(o[S+0]+=1),b<.2&&(o[S+2]+=1),w<.2&&(o[S+4]+=1))}}function f(S){r.push(S.x,S.y,S.z)}function d(S,R){let b=S*3;R.x=e[b+0],R.y=e[b+1],R.z=e[b+2]}function g(){let S=new D,R=new D,b=new D,w=new D,y=new ye,C=new ye,_=new ye;for(let v=0,E=0;v<r.length;v+=9,E+=6){S.set(r[v+0],r[v+1],r[v+2]),R.set(r[v+3],r[v+4],r[v+5]),b.set(r[v+6],r[v+7],r[v+8]),y.set(o[E+0],o[E+1]),C.set(o[E+2],o[E+3]),_.set(o[E+4],o[E+5]),w.copy(S).add(R).add(b).divideScalar(3);let P=m(w);M(y,E+0,S,P),M(C,E+2,R,P),M(_,E+4,b,P)}}function M(S,R,b,w){w<0&&S.x===1&&(o[R]=S.x-1),b.x===0&&b.z===0&&(o[R]=w/2/Math.PI+.5)}function m(S){return Math.atan2(S.z,-S.x)}function p(S){return Math.atan2(-S.y,Math.sqrt(S.x*S.x+S.z*S.z))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.vertices,e.indices,e.radius,e.detail)}};var Tn=class{constructor(){this.type="Curve",this.arcLengthDivisions=200,this.needsUpdate=!1,this.cacheArcLengths=null}getPoint(){Be("Curve: .getPoint() not implemented.")}getPointAt(e,t){let n=this.getUtoTmapping(e);return this.getPoint(n,t)}getPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return t}getSpacedPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPointAt(n/e));return t}getLength(){let e=this.getLengths();return e[e.length-1]}getLengths(e=this.arcLengthDivisions){if(this.cacheArcLengths&&this.cacheArcLengths.length===e+1&&!this.needsUpdate)return this.cacheArcLengths;this.needsUpdate=!1;let t=[],n,i=this.getPoint(0),r=0;t.push(0);for(let o=1;o<=e;o++)n=this.getPoint(o/e),r+=n.distanceTo(i),t.push(r),i=n;return this.cacheArcLengths=t,t}updateArcLengths(){this.needsUpdate=!0,this.getLengths()}getUtoTmapping(e,t=null){let n=this.getLengths(),i=0,r=n.length,o;t?o=t:o=e*n[r-1];let a=0,l=r-1,c;for(;a<=l;)if(i=Math.floor(a+(l-a)/2),c=n[i]-o,c<0)a=i+1;else if(c>0)l=i-1;else{l=i;break}if(i=l,n[i]===o)return i/(r-1);let h=n[i],f=n[i+1]-h,d=(o-h)/f;return(i+d)/(r-1)}getTangent(e,t){let i=e-1e-4,r=e+1e-4;i<0&&(i=0),r>1&&(r=1);let o=this.getPoint(i),a=this.getPoint(r),l=t||(o.isVector2?new ye:new D);return l.copy(a).sub(o).normalize(),l}getTangentAt(e,t){let n=this.getUtoTmapping(e);return this.getTangent(n,t)}computeFrenetFrames(e,t=!1){let n=new D,i=[],r=[],o=[],a=new D,l=new Ke;for(let d=0;d<=e;d++){let g=d/e;i[d]=this.getTangentAt(g,new D)}r[0]=new D,o[0]=new D;let c=Number.MAX_VALUE,h=Math.abs(i[0].x),u=Math.abs(i[0].y),f=Math.abs(i[0].z);h<=c&&(c=h,n.set(1,0,0)),u<=c&&(c=u,n.set(0,1,0)),f<=c&&n.set(0,0,1),a.crossVectors(i[0],n).normalize(),r[0].crossVectors(i[0],a),o[0].crossVectors(i[0],r[0]);for(let d=1;d<=e;d++){if(r[d]=r[d-1].clone(),o[d]=o[d-1].clone(),a.crossVectors(i[d-1],i[d]),a.length()>Number.EPSILON){a.normalize();let g=Math.acos(tt(i[d-1].dot(i[d]),-1,1));r[d].applyMatrix4(l.makeRotationAxis(a,g))}o[d].crossVectors(i[d],r[d])}if(t===!0){let d=Math.acos(tt(r[0].dot(r[e]),-1,1));d/=e,i[0].dot(a.crossVectors(r[0],r[e]))>0&&(d=-d);for(let g=1;g<=e;g++)r[g].applyMatrix4(l.makeRotationAxis(i[g],d*g)),o[g].crossVectors(i[g],r[g])}return{tangents:i,normals:r,binormals:o}}clone(){return new this.constructor().copy(this)}copy(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}toJSON(){let e={metadata:{version:4.7,type:"Curve",generator:"Curve.toJSON"}};return e.arcLengthDivisions=this.arcLengthDivisions,e.type=this.type,e}fromJSON(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}},jr=class extends Tn{constructor(e=0,t=0,n=1,i=1,r=0,o=Math.PI*2,a=!1,l=0){super(),this.isEllipseCurve=!0,this.type="EllipseCurve",this.aX=e,this.aY=t,this.xRadius=n,this.yRadius=i,this.aStartAngle=r,this.aEndAngle=o,this.aClockwise=a,this.aRotation=l}getPoint(e,t=new ye){let n=t,i=Math.PI*2,r=this.aEndAngle-this.aStartAngle,o=Math.abs(r)<Number.EPSILON;for(;r<0;)r+=i;for(;r>i;)r-=i;r<Number.EPSILON&&(o?r=0:r=i),this.aClockwise===!0&&!o&&(r===i?r=-i:r=r-i);let a=this.aStartAngle+e*r,l=this.aX+this.xRadius*Math.cos(a),c=this.aY+this.yRadius*Math.sin(a);if(this.aRotation!==0){let h=Math.cos(this.aRotation),u=Math.sin(this.aRotation),f=l-this.aX,d=c-this.aY;l=f*h-d*u+this.aX,c=f*u+d*h+this.aY}return n.set(l,c)}copy(e){return super.copy(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}toJSON(){let e=super.toJSON();return e.aX=this.aX,e.aY=this.aY,e.xRadius=this.xRadius,e.yRadius=this.yRadius,e.aStartAngle=this.aStartAngle,e.aEndAngle=this.aEndAngle,e.aClockwise=this.aClockwise,e.aRotation=this.aRotation,e}fromJSON(e){return super.fromJSON(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}},La=class extends jr{constructor(e,t,n,i,r,o){super(e,t,n,n,i,r,o),this.isArcCurve=!0,this.type="ArcCurve"}};function rh(){let s=0,e=0,t=0,n=0;function i(r,o,a,l){s=r,e=a,t=-3*r+3*o-2*a-l,n=2*r-2*o+a+l}return{initCatmullRom:function(r,o,a,l,c){i(o,a,c*(a-r),c*(l-o))},initNonuniformCatmullRom:function(r,o,a,l,c,h,u){let f=(o-r)/c-(a-r)/(c+h)+(a-o)/h,d=(a-o)/h-(l-o)/(h+u)+(l-a)/u;f*=h,d*=h,i(o,a,f,d)},calc:function(r){let o=r*r,a=o*r;return s+e*r+t*o+n*a}}}var Gu=new D,Wu=new D,Pc=new rh,Ic=new rh,Lc=new rh,or=class extends Tn{constructor(e=[],t=!1,n="centripetal",i=.5){super(),this.isCatmullRomCurve3=!0,this.type="CatmullRomCurve3",this.points=e,this.closed=t,this.curveType=n,this.tension=i}getPoint(e,t=new D){let n=t,i=this.points,r=i.length,o=(r-(this.closed?0:1))*e,a=Math.floor(o),l=o-a;this.closed?a+=a>0?0:(Math.floor(Math.abs(a)/r)+1)*r:l===0&&a===r-1&&(a=r-2,l=1);let c,h;this.closed||a>0?c=i[(a-1)%r]:(Wu.subVectors(i[0],i[1]).add(i[0]),c=Wu);let u=i[a%r],f=i[(a+1)%r];if(this.closed||a+2<r?h=i[(a+2)%r]:(Gu.subVectors(i[r-1],i[r-2]).add(i[r-1]),h=Gu),this.curveType==="centripetal"||this.curveType==="chordal"){let d=this.curveType==="chordal"?.5:.25,g=Math.pow(c.distanceToSquared(u),d),M=Math.pow(u.distanceToSquared(f),d),m=Math.pow(f.distanceToSquared(h),d);M<1e-4&&(M=1),g<1e-4&&(g=M),m<1e-4&&(m=M),Pc.initNonuniformCatmullRom(c.x,u.x,f.x,h.x,g,M,m),Ic.initNonuniformCatmullRom(c.y,u.y,f.y,h.y,g,M,m),Lc.initNonuniformCatmullRom(c.z,u.z,f.z,h.z,g,M,m)}else this.curveType==="catmullrom"&&(Pc.initCatmullRom(c.x,u.x,f.x,h.x,this.tension),Ic.initCatmullRom(c.y,u.y,f.y,h.y,this.tension),Lc.initCatmullRom(c.z,u.z,f.z,h.z,this.tension));return n.set(Pc.calc(l),Ic.calc(l),Lc.calc(l)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e.closed=this.closed,e.curveType=this.curveType,e.tension=this.tension,e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new D().fromArray(i))}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}};function Xu(s,e,t,n,i){let r=(n-e)*.5,o=(i-t)*.5,a=s*s,l=s*a;return(2*t-2*n+r+o)*l+(-3*t+3*n-2*r-o)*a+r*s+t}function Wp(s,e){let t=1-s;return t*t*e}function Xp(s,e){return 2*(1-s)*s*e}function qp(s,e){return s*s*e}function Or(s,e,t,n){return Wp(s,e)+Xp(s,t)+qp(s,n)}function Yp(s,e){let t=1-s;return t*t*t*e}function Zp(s,e){let t=1-s;return 3*t*t*s*e}function Kp(s,e){return 3*(1-s)*s*s*e}function Jp(s,e){return s*s*s*e}function Br(s,e,t,n,i){return Yp(s,e)+Zp(s,t)+Kp(s,n)+Jp(s,i)}var Da=class extends Tn{constructor(e=new ye,t=new ye,n=new ye,i=new ye){super(),this.isCubicBezierCurve=!0,this.type="CubicBezierCurve",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new ye){let n=t,i=this.v0,r=this.v1,o=this.v2,a=this.v3;return n.set(Br(e,i.x,r.x,o.x,a.x),Br(e,i.y,r.y,o.y,a.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},Na=class extends Tn{constructor(e=new D,t=new D,n=new D,i=new D){super(),this.isCubicBezierCurve3=!0,this.type="CubicBezierCurve3",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new D){let n=t,i=this.v0,r=this.v1,o=this.v2,a=this.v3;return n.set(Br(e,i.x,r.x,o.x,a.x),Br(e,i.y,r.y,o.y,a.y),Br(e,i.z,r.z,o.z,a.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},Ua=class extends Tn{constructor(e=new ye,t=new ye){super(),this.isLineCurve=!0,this.type="LineCurve",this.v1=e,this.v2=t}getPoint(e,t=new ye){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new ye){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},ar=class extends Tn{constructor(e=new D,t=new D){super(),this.isLineCurve3=!0,this.type="LineCurve3",this.v1=e,this.v2=t}getPoint(e,t=new D){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new D){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Fa=class extends Tn{constructor(e=new ye,t=new ye,n=new ye){super(),this.isQuadraticBezierCurve=!0,this.type="QuadraticBezierCurve",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new ye){let n=t,i=this.v0,r=this.v1,o=this.v2;return n.set(Or(e,i.x,r.x,o.x),Or(e,i.y,r.y,o.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Xi=class extends Tn{constructor(e=new D,t=new D,n=new D){super(),this.isQuadraticBezierCurve3=!0,this.type="QuadraticBezierCurve3",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new D){let n=t,i=this.v0,r=this.v1,o=this.v2;return n.set(Or(e,i.x,r.x,o.x),Or(e,i.y,r.y,o.y),Or(e,i.z,r.z,o.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Oa=class extends Tn{constructor(e=[]){super(),this.isSplineCurve=!0,this.type="SplineCurve",this.points=e}getPoint(e,t=new ye){let n=t,i=this.points,r=(i.length-1)*e,o=Math.floor(r),a=r-o,l=i[o===0?o:o-1],c=i[o],h=i[o>i.length-2?i.length-1:o+1],u=i[o>i.length-3?i.length-1:o+2];return n.set(Xu(a,l.x,c.x,h.x,u.x),Xu(a,l.y,c.y,h.y,u.y)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new ye().fromArray(i))}return this}},qu=Object.freeze({__proto__:null,ArcCurve:La,CatmullRomCurve3:or,CubicBezierCurve:Da,CubicBezierCurve3:Na,EllipseCurve:jr,LineCurve:Ua,LineCurve3:ar,QuadraticBezierCurve:Fa,QuadraticBezierCurve3:Xi,SplineCurve:Oa}),$r=class extends Tn{constructor(){super(),this.type="CurvePath",this.curves=[],this.autoClose=!1}add(e){this.curves.push(e)}closePath(){let e=this.curves[0].getPoint(0),t=this.curves[this.curves.length-1].getPoint(1);if(!e.equals(t)){let n=e.isVector2===!0?"LineCurve":"LineCurve3";this.curves.push(new qu[n](t,e))}return this}getPoint(e,t){let n=e*this.getLength(),i=this.getCurveLengths(),r=0;for(;r<i.length;){if(i[r]>=n){let o=i[r]-n,a=this.curves[r],l=a.getLength(),c=l===0?0:1-o/l;return a.getPointAt(c,t)}r++}return null}getLength(){let e=this.getCurveLengths();return e[e.length-1]}updateArcLengths(){this.needsUpdate=!0,this.cacheLengths=null,this.getCurveLengths()}getCurveLengths(){if(this.cacheLengths&&this.cacheLengths.length===this.curves.length)return this.cacheLengths;let e=[],t=0;for(let n=0,i=this.curves.length;n<i;n++)t+=this.curves[n].getLength(),e.push(t);return this.cacheLengths=e,e}getSpacedPoints(e=40){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return this.autoClose&&t.push(t[0]),t}getPoints(e=12){let t=[],n;for(let i=0,r=this.curves;i<r.length;i++){let o=r[i],a=o.isEllipseCurve?e*2:o.isLineCurve||o.isLineCurve3?1:o.isSplineCurve?e*o.points.length:e,l=o.getPoints(a);for(let c=0;c<l.length;c++){let h=l[c];n&&n.equals(h)||(t.push(h),n=h)}}return this.autoClose&&t.length>1&&!t[t.length-1].equals(t[0])&&t.push(t[0]),t}copy(e){super.copy(e),this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(i.clone())}return this.autoClose=e.autoClose,this}toJSON(){let e=super.toJSON();e.autoClose=this.autoClose,e.curves=[];for(let t=0,n=this.curves.length;t<n;t++){let i=this.curves[t];e.curves.push(i.toJSON())}return e}fromJSON(e){super.fromJSON(e),this.autoClose=e.autoClose,this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(new qu[i.type]().fromJSON(i))}return this}};var lr=class s extends Ia{constructor(e=1,t=0){let n=(1+Math.sqrt(5))/2,i=[-1,n,0,1,n,0,-1,-n,0,1,-n,0,0,-1,n,0,1,n,0,-1,-n,0,1,-n,n,0,-1,n,0,1,-n,0,-1,-n,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(i,r,e,t),this.type="IcosahedronGeometry",this.parameters={radius:e,detail:t}}static fromJSON(e){return new s(e.radius,e.detail)}};var sn=class s extends ft{constructor(e=1,t=1,n=1,i=1){super(),this.type="PlaneGeometry",this.parameters={width:e,height:t,widthSegments:n,heightSegments:i};let r=e/2,o=t/2,a=Math.floor(n),l=Math.floor(i),c=a+1,h=l+1,u=e/a,f=t/l,d=[],g=[],M=[],m=[];for(let p=0;p<h;p++){let S=p*f-o;for(let R=0;R<c;R++){let b=R*u-r;g.push(b,-S,0),M.push(0,0,1),m.push(R/a),m.push(1-p/l)}}for(let p=0;p<l;p++)for(let S=0;S<a;S++){let R=S+c*p,b=S+c*(p+1),w=S+1+c*(p+1),y=S+1+c*p;d.push(R,b,y),d.push(b,w,y)}this.setIndex(d),this.setAttribute("position",new ot(g,3)),this.setAttribute("normal",new ot(M,3)),this.setAttribute("uv",new ot(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.widthSegments,e.heightSegments)}},Qr=class s extends ft{constructor(e=.5,t=1,n=32,i=1,r=0,o=Math.PI*2){super(),this.type="RingGeometry",this.parameters={innerRadius:e,outerRadius:t,thetaSegments:n,phiSegments:i,thetaStart:r,thetaLength:o},n=Math.max(3,n),i=Math.max(1,i);let a=[],l=[],c=[],h=[],u=e,f=(t-e)/i,d=new D,g=new ye;for(let M=0;M<=i;M++){for(let m=0;m<=n;m++){let p=r+m/n*o;d.x=u*Math.cos(p),d.y=u*Math.sin(p),l.push(d.x,d.y,d.z),c.push(0,0,1),g.x=(d.x/t+1)/2,g.y=(d.y/t+1)/2,h.push(g.x,g.y)}u+=f}for(let M=0;M<i;M++){let m=M*(n+1);for(let p=0;p<n;p++){let S=p+m,R=S,b=S+n+1,w=S+n+2,y=S+1;a.push(R,b,y),a.push(b,w,y)}}this.setIndex(a),this.setAttribute("position",new ot(l,3)),this.setAttribute("normal",new ot(c,3)),this.setAttribute("uv",new ot(h,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.innerRadius,e.outerRadius,e.thetaSegments,e.phiSegments,e.thetaStart,e.thetaLength)}};var ri=class s extends ft{constructor(e=1,t=32,n=16,i=0,r=Math.PI*2,o=0,a=Math.PI){super(),this.type="SphereGeometry",this.parameters={radius:e,widthSegments:t,heightSegments:n,phiStart:i,phiLength:r,thetaStart:o,thetaLength:a},t=Math.max(3,Math.floor(t)),n=Math.max(2,Math.floor(n));let l=Math.min(o+a,Math.PI),c=0,h=[],u=new D,f=new D,d=[],g=[],M=[],m=[];for(let p=0;p<=n;p++){let S=[],R=p/n,b=o+R*a,w=e*Math.cos(b),y=Math.sqrt(e*e-w*w),C=0;p===0&&o===0?C=.5/t:p===n&&l===Math.PI&&(C=-.5/t);for(let _=0;_<=t;_++){let v=_/t,E=i+v*r;u.x=-y*Math.cos(E),u.y=w,u.z=y*Math.sin(E),g.push(u.x,u.y,u.z),f.copy(u).normalize(),M.push(f.x,f.y,f.z),m.push(v+C,1-R),S.push(c++)}h.push(S)}for(let p=0;p<n;p++)for(let S=0;S<t;S++){let R=h[p][S+1],b=h[p][S],w=h[p+1][S],y=h[p+1][S+1];(p!==0||o>0)&&d.push(R,b,y),(p!==n-1||l<Math.PI)&&d.push(b,w,y)}this.setIndex(d),this.setAttribute("position",new ot(g,3)),this.setAttribute("normal",new ot(M,3)),this.setAttribute("uv",new ot(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.widthSegments,e.heightSegments,e.phiStart,e.phiLength,e.thetaStart,e.thetaLength)}};var Si=class s extends ft{constructor(e=1,t=.4,n=12,i=48,r=Math.PI*2,o=0,a=Math.PI*2){super(),this.type="TorusGeometry",this.parameters={radius:e,tube:t,radialSegments:n,tubularSegments:i,arc:r,thetaStart:o,thetaLength:a},n=Math.floor(n),i=Math.floor(i);let l=[],c=[],h=[],u=[],f=new D,d=new D,g=new D;for(let M=0;M<=n;M++){let m=o+M/n*a;for(let p=0;p<=i;p++){let S=p/i*r;d.x=(e+t*Math.cos(m))*Math.cos(S),d.y=(e+t*Math.cos(m))*Math.sin(S),d.z=t*Math.sin(m),c.push(d.x,d.y,d.z),f.x=e*Math.cos(S),f.y=e*Math.sin(S),g.subVectors(d,f).normalize(),h.push(g.x,g.y,g.z),u.push(p/i),u.push(M/n)}}for(let M=1;M<=n;M++)for(let m=1;m<=i;m++){let p=(i+1)*M+m-1,S=(i+1)*(M-1)+m-1,R=(i+1)*(M-1)+m,b=(i+1)*M+m;l.push(p,S,b),l.push(S,R,b)}this.setIndex(l),this.setAttribute("position",new ot(c,3)),this.setAttribute("normal",new ot(h,3)),this.setAttribute("uv",new ot(u,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.tube,e.radialSegments,e.tubularSegments,e.arc)}};var eo=class extends ft{constructor(e=null){if(super(),this.type="WireframeGeometry",this.parameters={geometry:e},e!==null){let t=[],n=new Set,i=new D,r=new D;if(e.index!==null){let o=e.attributes.position,a=e.index,l=e.groups;l.length===0&&(l=[{start:0,count:a.count,materialIndex:0}]);for(let c=0,h=l.length;c<h;++c){let u=l[c],f=u.start,d=u.count;for(let g=f,M=f+d;g<M;g+=3)for(let m=0;m<3;m++){let p=a.getX(g+m),S=a.getX(g+(m+1)%3);i.fromBufferAttribute(o,p),r.fromBufferAttribute(o,S),Yu(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}}else{let o=e.attributes.position;for(let a=0,l=o.count/3;a<l;a++)for(let c=0;c<3;c++){let h=3*a+c,u=3*a+(c+1)%3;i.fromBufferAttribute(o,h),r.fromBufferAttribute(o,u),Yu(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}this.setAttribute("position",new ot(t,3))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}};function Yu(s,e,t){let n=`${s.x},${s.y},${s.z}-${e.x},${e.y},${e.z}`,i=`${e.x},${e.y},${e.z}-${s.x},${s.y},${s.z}`;return t.has(n)===!0||t.has(i)===!0?!1:(t.add(n),t.add(i),!0)}function Es(s){let e={};for(let t in s){e[t]={};for(let n in s[t]){let i=s[t][n];if(Zu(i))i.isRenderTargetTexture?(Be("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),e[t][n]=null):e[t][n]=i.clone();else if(Array.isArray(i))if(Zu(i[0])){let r=[];for(let o=0,a=i.length;o<a;o++)r[o]=i[o].clone();e[t][n]=r}else e[t][n]=i.slice();else e[t][n]=i}}return e}function on(s){let e={};for(let t=0;t<s.length;t++){let n=Es(s[t]);for(let i in n)e[i]=n[i]}return e}function Zu(s){return s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)}function jp(s){let e=[];for(let t=0;t<s.length;t++)e.push(s[t].clone());return e}function oh(s){let e=s.getRenderTarget();return e===null?s.outputColorSpace:e.isXRRenderTarget===!0?e.texture.colorSpace:et.workingColorSpace}var Kn={clone:Es,merge:on},$p=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,Qp=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,mt=class extends un{constructor(e){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=$p,this.fragmentShader=Qp,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,e!==void 0&&this.setValues(e)}copy(e){return super.copy(e),this.fragmentShader=e.fragmentShader,this.vertexShader=e.vertexShader,this.uniforms=Es(e.uniforms),this.uniformsGroups=jp(e.uniformsGroups),this.defines=Object.assign({},e.defines),this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.fog=e.fog,this.lights=e.lights,this.clipping=e.clipping,this.extensions=Object.assign({},e.extensions),this.glslVersion=e.glslVersion,this.defaultAttributeValues=Object.assign({},e.defaultAttributeValues),this.index0AttributeName=e.index0AttributeName,this.uniformsNeedUpdate=e.uniformsNeedUpdate,this}toJSON(e){let t=super.toJSON(e);t.glslVersion=this.glslVersion,t.uniforms={};for(let i in this.uniforms){let o=this.uniforms[i].value;o&&o.isTexture?t.uniforms[i]={type:"t",value:o.toJSON(e).uuid}:o&&o.isColor?t.uniforms[i]={type:"c",value:o.getHex()}:o&&o.isVector2?t.uniforms[i]={type:"v2",value:o.toArray()}:o&&o.isVector3?t.uniforms[i]={type:"v3",value:o.toArray()}:o&&o.isVector4?t.uniforms[i]={type:"v4",value:o.toArray()}:o&&o.isMatrix3?t.uniforms[i]={type:"m3",value:o.toArray()}:o&&o.isMatrix4?t.uniforms[i]={type:"m4",value:o.toArray()}:t.uniforms[i]={value:o}}Object.keys(this.defines).length>0&&(t.defines=this.defines),t.vertexShader=this.vertexShader,t.fragmentShader=this.fragmentShader,t.lights=this.lights,t.clipping=this.clipping;let n={};for(let i in this.extensions)this.extensions[i]===!0&&(n[i]=!0);return Object.keys(n).length>0&&(t.extensions=n),t}fromJSON(e,t){if(super.fromJSON(e,t),e.uniforms!==void 0)for(let n in e.uniforms){let i=e.uniforms[n];switch(this.uniforms[n]={},i.type){case"t":this.uniforms[n].value=t[i.value]||null;break;case"c":this.uniforms[n].value=new Me().setHex(i.value);break;case"v2":this.uniforms[n].value=new ye().fromArray(i.value);break;case"v3":this.uniforms[n].value=new D().fromArray(i.value);break;case"v4":this.uniforms[n].value=new xt().fromArray(i.value);break;case"m3":this.uniforms[n].value=new $e().fromArray(i.value);break;case"m4":this.uniforms[n].value=new Ke().fromArray(i.value);break;default:this.uniforms[n].value=i.value}}if(e.defines!==void 0&&(this.defines=e.defines),e.vertexShader!==void 0&&(this.vertexShader=e.vertexShader),e.fragmentShader!==void 0&&(this.fragmentShader=e.fragmentShader),e.glslVersion!==void 0&&(this.glslVersion=e.glslVersion),e.extensions!==void 0)for(let n in e.extensions)this.extensions[n]=e.extensions[n];return e.lights!==void 0&&(this.lights=e.lights),e.clipping!==void 0&&(this.clipping=e.clipping),this}},cr=class extends mt{constructor(e){super(e),this.isRawShaderMaterial=!0,this.type="RawShaderMaterial"}},dn=class extends un{constructor(e){super(),this.isMeshStandardMaterial=!0,this.type="MeshStandardMaterial",this.defines={STANDARD:""},this.color=new Me(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Me(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Po,this.normalScale=new ye(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Un,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.defines={STANDARD:""},this.color.copy(e.color),this.roughness=e.roughness,this.metalness=e.metalness,this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.roughnessMap=e.roughnessMap,this.metalnessMap=e.metalnessMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.envMapIntensity=e.envMapIntensity,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},pn=class extends dn{constructor(e){super(),this.isMeshPhysicalMaterial=!0,this.defines={STANDARD:"",PHYSICAL:""},this.type="MeshPhysicalMaterial",this.anisotropyRotation=0,this.anisotropyMap=null,this.clearcoatMap=null,this.clearcoatRoughness=0,this.clearcoatRoughnessMap=null,this.clearcoatNormalScale=new ye(1,1),this.clearcoatNormalMap=null,this.ior=1.5,Object.defineProperty(this,"reflectivity",{get:function(){return tt(2.5*(this.ior-1)/(this.ior+1),0,1)},set:function(t){this.ior=(1+.4*t)/(1-.4*t)}}),this.iridescenceMap=null,this.iridescenceIOR=1.3,this.iridescenceThicknessRange=[100,400],this.iridescenceThicknessMap=null,this.sheenColor=new Me(0),this.sheenColorMap=null,this.sheenRoughness=1,this.sheenRoughnessMap=null,this.transmissionMap=null,this.thickness=0,this.thicknessMap=null,this.attenuationDistance=1/0,this.attenuationColor=new Me(1,1,1),this.specularIntensity=1,this.specularIntensityMap=null,this.specularColor=new Me(1,1,1),this.specularColorMap=null,this._anisotropy=0,this._clearcoat=0,this._dispersion=0,this._iridescence=0,this._sheen=0,this._transmission=0,this.setValues(e)}get anisotropy(){return this._anisotropy}set anisotropy(e){this._anisotropy>0!=e>0&&this.version++,this._anisotropy=e}get clearcoat(){return this._clearcoat}set clearcoat(e){this._clearcoat>0!=e>0&&this.version++,this._clearcoat=e}get iridescence(){return this._iridescence}set iridescence(e){this._iridescence>0!=e>0&&this.version++,this._iridescence=e}get dispersion(){return this._dispersion}set dispersion(e){this._dispersion>0!=e>0&&this.version++,this._dispersion=e}get sheen(){return this._sheen}set sheen(e){this._sheen>0!=e>0&&this.version++,this._sheen=e}get transmission(){return this._transmission}set transmission(e){this._transmission>0!=e>0&&this.version++,this._transmission=e}copy(e){return super.copy(e),this.defines={STANDARD:"",PHYSICAL:""},this.anisotropy=e.anisotropy,this.anisotropyRotation=e.anisotropyRotation,this.anisotropyMap=e.anisotropyMap,this.clearcoat=e.clearcoat,this.clearcoatMap=e.clearcoatMap,this.clearcoatRoughness=e.clearcoatRoughness,this.clearcoatRoughnessMap=e.clearcoatRoughnessMap,this.clearcoatNormalMap=e.clearcoatNormalMap,this.clearcoatNormalScale.copy(e.clearcoatNormalScale),this.dispersion=e.dispersion,this.ior=e.ior,this.iridescence=e.iridescence,this.iridescenceMap=e.iridescenceMap,this.iridescenceIOR=e.iridescenceIOR,this.iridescenceThicknessRange=[...e.iridescenceThicknessRange],this.iridescenceThicknessMap=e.iridescenceThicknessMap,this.sheen=e.sheen,this.sheenColor.copy(e.sheenColor),this.sheenColorMap=e.sheenColorMap,this.sheenRoughness=e.sheenRoughness,this.sheenRoughnessMap=e.sheenRoughnessMap,this.transmission=e.transmission,this.transmissionMap=e.transmissionMap,this.thickness=e.thickness,this.thicknessMap=e.thicknessMap,this.attenuationDistance=e.attenuationDistance,this.attenuationColor.copy(e.attenuationColor),this.specularIntensity=e.specularIntensity,this.specularIntensityMap=e.specularIntensityMap,this.specularColor.copy(e.specularColor),this.specularColorMap=e.specularColorMap,this}};var to=class extends un{constructor(e){super(),this.isMeshLambertMaterial=!0,this.type="MeshLambertMaterial",this.color=new Me(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Me(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Po,this.normalScale=new ye(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Un,this.combine=ja,this.reflectivity=1,this.envMapIntensity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.envMapIntensity=e.envMapIntensity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},Ba=class extends un{constructor(e){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=Pf,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(e)}copy(e){return super.copy(e),this.depthPacking=e.depthPacking,this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this}},za=class extends un{constructor(e){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(e)}copy(e){return super.copy(e),this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this}};function ua(s,e){return!s||s.constructor===e?s:typeof e.BYTES_PER_ELEMENT=="number"?new e(s):Array.prototype.slice.call(s)}function em(s){function e(i,r){return s[i]-s[r]}let t=s.length,n=new Array(t);for(let i=0;i!==t;++i)n[i]=i;return n.sort(e),n}function Ku(s,e,t){let n=s.length,i=new s.constructor(n);for(let r=0,o=0;o!==n;++r){let a=t[r]*e;for(let l=0;l!==e;++l)i[o++]=s[a+l]}return i}function tm(s,e,t,n){let i=1,r=s[0];for(;r!==void 0&&r[n]===void 0;)r=s[i++];if(r===void 0)return;let o=r[n];if(o!==void 0)if(Array.isArray(o))do o=r[n],o!==void 0&&(e.push(r.time),t.push(...o)),r=s[i++];while(r!==void 0);else if(o.toArray!==void 0)do o=r[n],o!==void 0&&(e.push(r.time),o.toArray(t,t.length)),r=s[i++];while(r!==void 0);else do o=r[n],o!==void 0&&(e.push(r.time),t.push(o)),r=s[i++];while(r!==void 0)}var oi=class{constructor(e,t,n,i){this.parameterPositions=e,this._cachedIndex=0,this.resultBuffer=i!==void 0?i:new t.constructor(n),this.sampleValues=t,this.valueSize=n,this.settings=null,this.DefaultSettings_={}}evaluate(e){let t=this.parameterPositions,n=this._cachedIndex,i=t[n],r=t[n-1];e:{t:{let o;n:{i:if(!(e<i)){for(let a=n+2;;){if(i===void 0){if(e<r)break i;return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}if(n===a)break;if(r=i,i=t[++n],e<i)break t}o=t.length;break n}if(!(e>=r)){let a=t[1];e<a&&(n=2,r=a);for(let l=n-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(n===l)break;if(i=r,r=t[--n-1],e>=r)break t}o=n,n=0;break n}break e}for(;n<o;){let a=n+o>>>1;e<t[a]?o=a:n=a+1}if(i=t[n],r=t[n-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===void 0)return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}this._cachedIndex=n,this.intervalChanged_(n,r,i)}return this.interpolate_(n,r,e,i)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i;for(let o=0;o!==i;++o)t[o]=n[r+o];return t}interpolate_(){throw new Error("THREE.Interpolant: Call to abstract method.")}intervalChanged_(){}},ka=class extends oi{constructor(e,t,n,i){super(e,t,n,i),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:ls,endingEnd:ls}}intervalChanged_(e,t,n){let i=this.parameterPositions,r=e-2,o=e+1,a=i[r],l=i[o];if(a===void 0)switch(this.getSettings_().endingStart){case cs:r=e,a=2*t-n;break;case zr:r=i.length-2,a=t+i[r]-i[r+1];break;default:r=e,a=n}if(l===void 0)switch(this.getSettings_().endingEnd){case cs:o=e,l=2*n-t;break;case zr:o=1,l=n+i[1]-i[0];break;default:o=e-1,l=t}let c=(n-t)*.5,h=this.valueSize;this._weightPrev=c/(t-a),this._weightNext=c/(l-n),this._offsetPrev=r*h,this._offsetNext=o*h}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=this._offsetPrev,u=this._offsetNext,f=this._weightPrev,d=this._weightNext,g=(n-t)/(i-t),M=g*g,m=M*g,p=-f*m+2*f*M-f*g,S=(1+f)*m+(-1.5-2*f)*M+(-.5+f)*g+1,R=(-1-d)*m+(1.5+d)*M+.5*g,b=d*m-d*M;for(let w=0;w!==a;++w)r[w]=p*o[h+w]+S*o[c+w]+R*o[l+w]+b*o[u+w];return r}},no=class extends oi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=(n-t)/(i-t),u=1-h;for(let f=0;f!==a;++f)r[f]=o[c+f]*u+o[l+f]*h;return r}},Ha=class extends oi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e){return this.copySampleValue_(e-1)}},Va=class extends oi{interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=this.inTangents,u=this.outTangents;if(!h||!u){let g=(n-t)/(i-t),M=1-g;for(let m=0;m!==a;++m)r[m]=o[c+m]*M+o[l+m]*g;return r}let f=a*2,d=e-1;for(let g=0;g!==a;++g){let M=o[c+g],m=o[l+g],p=d*f+g*2,S=u[p],R=u[p+1],b=e*f+g*2,w=h[b],y=h[b+1],C=(n-t)/(i-t),_,v,E,P,L;for(let H=0;H<8;H++){_=C*C,v=_*C,E=1-C,P=E*E,L=P*E;let U=L*t+3*P*C*S+3*E*_*w+v*i-n;if(Math.abs(U)<1e-10)break;let V=3*P*(S-t)+6*E*C*(w-S)+3*_*(i-w);if(Math.abs(V)<1e-10)break;C=C-U/V,C=Math.max(0,Math.min(1,C))}r[g]=L*M+3*P*C*R+3*E*_*y+v*m}return r}},mn=class{constructor(e,t,n,i){if(e===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(t===void 0||t.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+e);this.name=e,this.times=ua(t,this.TimeBufferType),this.values=ua(n,this.ValueBufferType),this.setInterpolation(i||this.DefaultInterpolation)}static toJSON(e){let t=e.constructor,n;if(t.toJSON!==this.toJSON)n=t.toJSON(e);else{n={name:e.name,times:ua(e.times,Array),values:ua(e.values,Array)};let i=e.getInterpolation();i!==e.DefaultInterpolation&&(n.interpolation=i)}return n.type=e.ValueTypeName,n}InterpolantFactoryMethodDiscrete(e){return new Ha(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodLinear(e){return new no(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodSmooth(e){return new ka(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodBezier(e){let t=new Va(this.times,this.values,this.getValueSize(),e);return this.settings&&(t.inTangents=this.settings.inTangents,t.outTangents=this.settings.outTangents),t}setInterpolation(e){let t;switch(e){case ds:t=this.InterpolantFactoryMethodDiscrete;break;case ps:t=this.InterpolantFactoryMethodLinear;break;case pa:t=this.InterpolantFactoryMethodSmooth;break;case Fc:t=this.InterpolantFactoryMethodBezier;break}if(t===void 0){let n="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(e!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(n);return Be("KeyframeTrack:",n),this}return this.createInterpolant=t,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return ds;case this.InterpolantFactoryMethodLinear:return ps;case this.InterpolantFactoryMethodSmooth:return pa;case this.InterpolantFactoryMethodBezier:return Fc}}getValueSize(){return this.values.length/this.times.length}shift(e){if(e!==0){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]+=e}return this}scale(e){if(e!==1){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]*=e}return this}trim(e,t){let n=this.times,i=n.length,r=0,o=i-1;for(;r!==i&&n[r]<e;)++r;for(;o!==-1&&n[o]>t;)--o;if(++o,r!==0||o!==i){r>=o&&(o=Math.max(o,1),r=o-1);let a=this.getValueSize();this.times=n.slice(r,o),this.values=this.values.slice(r*a,o*a)}return this}validate(){let e=!0,t=this.getValueSize();t-Math.floor(t)!==0&&(Ze("KeyframeTrack: Invalid value size in track.",this),e=!1);let n=this.times,i=this.values,r=n.length;r===0&&(Ze("KeyframeTrack: Track is empty.",this),e=!1);let o=null;for(let a=0;a!==r;a++){let l=n[a];if(typeof l=="number"&&isNaN(l)){Ze("KeyframeTrack: Time is not a valid number.",this,a,l),e=!1;break}if(o!==null&&o>l){Ze("KeyframeTrack: Out of order keys.",this,a,l,o),e=!1;break}o=l}if(i!==void 0&&op(i))for(let a=0,l=i.length;a!==l;++a){let c=i[a];if(isNaN(c)){Ze("KeyframeTrack: Value is not a valid number.",this,a,c),e=!1;break}}return e}optimize(){let e=this.times.slice(),t=this.values.slice(),n=this.getValueSize(),i=this.getInterpolation()===pa,r=e.length-1,o=1;for(let a=1;a<r;++a){let l=!1,c=e[a],h=e[a+1];if(c!==h&&(a!==1||c!==e[0]))if(i)l=!0;else{let u=a*n,f=u-n,d=u+n;for(let g=0;g!==n;++g){let M=t[u+g];if(M!==t[f+g]||M!==t[d+g]){l=!0;break}}}if(l){if(a!==o){e[o]=e[a];let u=a*n,f=o*n;for(let d=0;d!==n;++d)t[f+d]=t[u+d]}++o}}if(r>0){e[o]=e[r];for(let a=r*n,l=o*n,c=0;c!==n;++c)t[l+c]=t[a+c];++o}return o!==e.length?(this.times=e.slice(0,o),this.values=t.slice(0,o*n)):(this.times=e,this.values=t),this}clone(){let e=this.times.slice(),t=this.values.slice(),n=this.constructor,i=new n(this.name,e,t);return i.createInterpolant=this.createInterpolant,i}};mn.prototype.ValueTypeName="";mn.prototype.TimeBufferType=Float32Array;mn.prototype.ValueBufferType=Float32Array;mn.prototype.DefaultInterpolation=ps;var Ti=class extends mn{constructor(e,t,n){super(e,t,n)}};Ti.prototype.ValueTypeName="bool";Ti.prototype.ValueBufferType=Array;Ti.prototype.DefaultInterpolation=ds;Ti.prototype.InterpolantFactoryMethodLinear=void 0;Ti.prototype.InterpolantFactoryMethodSmooth=void 0;var io=class extends mn{constructor(e,t,n,i){super(e,t,n,i)}};io.prototype.ValueTypeName="color";var Ei=class extends mn{constructor(e,t,n,i){super(e,t,n,i)}};Ei.prototype.ValueTypeName="number";var Ga=class extends oi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=(n-t)/(i-t),c=e*a;for(let h=c+a;c!==h;c+=4)It.slerpFlat(r,0,o,c-a,o,c,l);return r}},wi=class extends mn{constructor(e,t,n,i){super(e,t,n,i)}InterpolantFactoryMethodLinear(e){return new Ga(this.times,this.values,this.getValueSize(),e)}};wi.prototype.ValueTypeName="quaternion";wi.prototype.InterpolantFactoryMethodSmooth=void 0;var Ai=class extends mn{constructor(e,t,n){super(e,t,n)}};Ai.prototype.ValueTypeName="string";Ai.prototype.ValueBufferType=Array;Ai.prototype.DefaultInterpolation=ds;Ai.prototype.InterpolantFactoryMethodLinear=void 0;Ai.prototype.InterpolantFactoryMethodSmooth=void 0;var qi=class extends mn{constructor(e,t,n,i){super(e,t,n,i)}};qi.prototype.ValueTypeName="vector";var ys=class{constructor(e="",t=-1,n=[],i=zl){this.name=e,this.tracks=n,this.duration=t,this.blendMode=i,this.uuid=Wn(),this.userData={},this.duration<0&&this.resetDuration()}static parse(e){let t=[],n=e.tracks,i=1/(e.fps||1);for(let o=0,a=n.length;o!==a;++o)t.push(im(n[o]).scale(i));let r=new this(e.name,e.duration,t,e.blendMode);return r.uuid=e.uuid,r.userData=JSON.parse(e.userData||"{}"),r}static toJSON(e){let t=[],n=e.tracks,i={name:e.name,duration:e.duration,tracks:t,uuid:e.uuid,blendMode:e.blendMode,userData:JSON.stringify(e.userData)};for(let r=0,o=n.length;r!==o;++r)t.push(mn.toJSON(n[r]));return i}static CreateFromMorphTargetSequence(e,t,n,i){let r=t.length,o=[];for(let a=0;a<r;a++){let l=[],c=[];l.push((a+r-1)%r,a,(a+1)%r),c.push(0,1,0);let h=em(l);l=Ku(l,1,h),c=Ku(c,1,h),!i&&l[0]===0&&(l.push(r),c.push(c[0])),o.push(new Ei(".morphTargetInfluences["+t[a].name+"]",l,c).scale(1/n))}return new this(e,-1,o)}static findByName(e,t){let n=e;if(!Array.isArray(e)){let i=e;n=i.geometry&&i.geometry.animations||i.animations}for(let i=0;i<n.length;i++)if(n[i].name===t)return n[i];return null}static CreateClipsFromMorphTargetSequences(e,t,n){let i={},r=/^([\w-]*?)([\d]+)$/;for(let a=0,l=e.length;a<l;a++){let c=e[a],h=c.name.match(r);if(h&&h.length>1){let u=h[1],f=i[u];f||(i[u]=f=[]),f.push(c)}}let o=[];for(let a in i)o.push(this.CreateFromMorphTargetSequence(a,i[a],t,n));return o}resetDuration(){let e=this.tracks,t=0;for(let n=0,i=e.length;n!==i;++n){let r=this.tracks[n];t=Math.max(t,r.times[r.times.length-1])}return this.duration=t,this}trim(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].trim(0,this.duration);return this}validate(){let e=!0;for(let t=0;t<this.tracks.length;t++)e=e&&this.tracks[t].validate();return e}optimize(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].optimize();return this}clone(){let e=[];for(let n=0;n<this.tracks.length;n++)e.push(this.tracks[n].clone());let t=new this.constructor(this.name,this.duration,e,this.blendMode);return t.userData=JSON.parse(JSON.stringify(this.userData)),t}toJSON(){return this.constructor.toJSON(this)}};function nm(s){switch(s.toLowerCase()){case"scalar":case"double":case"float":case"number":case"integer":return Ei;case"vector":case"vector2":case"vector3":case"vector4":return qi;case"color":return io;case"quaternion":return wi;case"bool":case"boolean":return Ti;case"string":return Ai}throw new Error("THREE.KeyframeTrack: Unsupported typeName: "+s)}function im(s){if(s.type===void 0)throw new Error("THREE.KeyframeTrack: track type undefined, can not parse");let e=nm(s.type);if(s.times===void 0){let t=[],n=[];tm(s.keys,t,n,"value"),s.times=t,s.values=n}return e.parse!==void 0?e.parse(s):new e(s.name,s.times,s.values,s.interpolation)}var ei={enabled:!1,files:{},add:function(s,e){this.enabled!==!1&&(Ju(s)||(this.files[s]=e))},get:function(s){if(this.enabled!==!1&&!Ju(s))return this.files[s]},remove:function(s){delete this.files[s]},clear:function(){this.files={}}};function Ju(s){try{let e=s.slice(s.indexOf(":")+1);return new URL(e).protocol==="blob:"}catch{return!1}}var Wa=class{constructor(e,t,n){let i=this,r=!1,o=0,a=0,l,c=[];this.onStart=void 0,this.onLoad=e,this.onProgress=t,this.onError=n,this._abortController=null,this.itemStart=function(h){a++,r===!1&&i.onStart!==void 0&&i.onStart(h,o,a),r=!0},this.itemEnd=function(h){o++,i.onProgress!==void 0&&i.onProgress(h,o,a),o===a&&(r=!1,i.onLoad!==void 0&&i.onLoad())},this.itemError=function(h){i.onError!==void 0&&i.onError(h)},this.resolveURL=function(h){return h=h.normalize("NFC"),l?l(h):h},this.setURLModifier=function(h){return l=h,this},this.addHandler=function(h,u){return c.push(h,u),this},this.removeHandler=function(h){let u=c.indexOf(h);return u!==-1&&c.splice(u,2),this},this.getHandler=function(h){for(let u=0,f=c.length;u<f;u+=2){let d=c[u],g=c[u+1];if(d.global&&(d.lastIndex=0),d.test(h))return g}return null},this.abort=function(){return this.abortController.abort(),this._abortController=null,this}}get abortController(){return this._abortController||(this._abortController=new AbortController),this._abortController}},Gf=new Wa,ai=class{constructor(e){this.manager=e!==void 0?e:Gf,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}load(){}loadAsync(e,t){let n=this;return new Promise(function(i,r){n.load(e,i,t,r)})}parse(){}setCrossOrigin(e){return this.crossOrigin=e,this}setWithCredentials(e){return this.withCredentials=e,this}setPath(e){return this.path=e,this}setResourcePath(e){return this.resourcePath=e,this}setRequestHeader(e){return this.requestHeader=e,this}abort(){return this}};ai.DEFAULT_MATERIAL_NAME="__DEFAULT";var vi={},zc=class extends Error{constructor(e,t){super(e),this.response=t}},hr=class extends ai{constructor(e){super(e),this.mimeType="",this.responseType="",this._abortController=new AbortController}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=ei.get(`file:${e}`);if(r!==void 0){this.manager.itemStart(e),setTimeout(()=>{t&&t(r),this.manager.itemEnd(e)},0);return}if(vi[e]!==void 0){vi[e].push({onLoad:t,onProgress:n,onError:i});return}vi[e]=[],vi[e].push({onLoad:t,onProgress:n,onError:i});let o=new Request(e,{headers:new Headers(this.requestHeader),credentials:this.withCredentials?"include":"same-origin",signal:typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal}),a=this.mimeType,l=this.responseType;fetch(o).then(c=>{if(c.status===200||c.status===0){if(c.status===0&&Be("FileLoader: HTTP Status 0 received."),typeof ReadableStream>"u"||c.body===void 0||c.body.getReader===void 0)return c;let h=vi[e],u=c.body.getReader(),f=c.headers.get("X-File-Size")||c.headers.get("Content-Length"),d=f?parseInt(f):0,g=d!==0,M=0,m=new ReadableStream({start(p){S();function S(){u.read().then(({done:R,value:b})=>{if(R)p.close();else{M+=b.byteLength;let w=new ProgressEvent("progress",{lengthComputable:g,loaded:M,total:d});for(let y=0,C=h.length;y<C;y++){let _=h[y];_.onProgress&&_.onProgress(w)}p.enqueue(b),S()}},R=>{p.error(R)})}}});return new Response(m)}else throw new zc(`fetch for "${c.url}" responded with ${c.status}: ${c.statusText}`,c)}).then(c=>{switch(l){case"arraybuffer":return c.arrayBuffer();case"blob":return c.blob();case"document":return c.text().then(h=>new DOMParser().parseFromString(h,a));case"json":return c.json();default:if(a==="")return c.text();{let u=/charset="?([^;"\s]*)"?/i.exec(a),f=u&&u[1]?u[1].toLowerCase():void 0,d=new TextDecoder(f);return c.arrayBuffer().then(g=>d.decode(g))}}}).then(c=>{ei.add(`file:${e}`,c);let h=vi[e];delete vi[e];for(let u=0,f=h.length;u<f;u++){let d=h[u];d.onLoad&&d.onLoad(c)}}).catch(c=>{let h=vi[e];if(h===void 0)throw this.manager.itemError(e),c;delete vi[e];for(let u=0,f=h.length;u<f;u++){let d=h[u];d.onError&&d.onError(c)}this.manager.itemError(e)}).finally(()=>{this.manager.itemEnd(e)}),this.manager.itemStart(e)}setResponseType(e){return this.responseType=e,this}setMimeType(e){return this.mimeType=e,this}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var Vs=new WeakMap,Xa=class extends ai{constructor(e){super(e)}load(e,t,n,i){this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,o=ei.get(`image:${e}`);if(o!==void 0){if(o.complete===!0)r.manager.itemStart(e),setTimeout(function(){t&&t(o),r.manager.itemEnd(e)},0);else{let u=Vs.get(o);u===void 0&&(u=[],Vs.set(o,u)),u.push({onLoad:t,onError:i})}return o}let a=Zs("img");function l(){h(),t&&t(this);let u=Vs.get(this)||[];for(let f=0;f<u.length;f++){let d=u[f];d.onLoad&&d.onLoad(this)}Vs.delete(this),r.manager.itemEnd(e)}function c(u){h(),i&&i(u),ei.remove(`image:${e}`);let f=Vs.get(this)||[];for(let d=0;d<f.length;d++){let g=f[d];g.onError&&g.onError(u)}Vs.delete(this),r.manager.itemError(e),r.manager.itemEnd(e)}function h(){a.removeEventListener("load",l,!1),a.removeEventListener("error",c,!1)}return a.addEventListener("load",l,!1),a.addEventListener("error",c,!1),e.slice(0,5)!=="data:"&&this.crossOrigin!==void 0&&(a.crossOrigin=this.crossOrigin),ei.add(`image:${e}`,a),r.manager.itemStart(e),a.src=e,a}};var so=class extends ai{constructor(e){super(e)}load(e,t,n,i){let r=new Gt,o=new Xa(this.manager);return o.setCrossOrigin(this.crossOrigin),o.setPath(this.path),o.load(e,function(a){r.image=a,r.needsUpdate=!0,t!==void 0&&t(r)},n,i),r}},Ms=class extends yt{constructor(e,t=1){super(),this.isLight=!0,this.type="Light",this.color=new Me(e),this.intensity=t}dispose(){this.dispatchEvent({type:"dispose"})}copy(e,t){return super.copy(e,t),this.color.copy(e.color),this.intensity=e.intensity,this}toJSON(e){let t=super.toJSON(e);return t.object.color=this.color.getHex(),t.object.intensity=this.intensity,t}},ro=class extends Ms{constructor(e,t,n){super(e,n),this.isHemisphereLight=!0,this.type="HemisphereLight",this.position.copy(yt.DEFAULT_UP),this.updateMatrix(),this.groundColor=new Me(t)}copy(e,t){return super.copy(e,t),this.groundColor.copy(e.groundColor),this}toJSON(e){let t=super.toJSON(e);return t.object.groundColor=this.groundColor.getHex(),t}},Dc=new Ke,ju=new D,$u=new D,oo=class{constructor(e){this.camera=e,this.intensity=1,this.bias=0,this.biasNode=null,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new ye(512,512),this.mapType=gn,this.map=null,this.mapPass=null,this.matrix=new Ke,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new ir,this._frameExtents=new ye(1,1),this._viewportCount=1,this._viewports=[new xt(0,0,1,1)]}getViewportCount(){return this._viewportCount}getFrustum(){return this._frustum}updateMatrices(e){let t=this.camera,n=this.matrix;ju.setFromMatrixPosition(e.matrixWorld),t.position.copy(ju),$u.setFromMatrixPosition(e.target.matrixWorld),t.lookAt($u),t.updateMatrixWorld(),Dc.multiplyMatrices(t.projectionMatrix,t.matrixWorldInverse),this._frustum.setFromProjectionMatrix(Dc,t.coordinateSystem,t.reversedDepth),t.coordinateSystem===Ys||t.reversedDepth?n.set(.5,0,0,.5,0,.5,0,.5,0,0,1,0,0,0,0,1):n.set(.5,0,0,.5,0,.5,0,.5,0,0,.5,.5,0,0,0,1),n.multiply(Dc)}getViewport(e){return this._viewports[e]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(e){return this.camera=e.camera.clone(),this.intensity=e.intensity,this.bias=e.bias,this.radius=e.radius,this.autoUpdate=e.autoUpdate,this.needsUpdate=e.needsUpdate,this.normalBias=e.normalBias,this.blurSamples=e.blurSamples,this.mapSize.copy(e.mapSize),this.biasNode=e.biasNode,this}clone(){return new this.constructor().copy(this)}toJSON(){let e={};return this.intensity!==1&&(e.intensity=this.intensity),this.bias!==0&&(e.bias=this.bias),this.normalBias!==0&&(e.normalBias=this.normalBias),this.radius!==1&&(e.radius=this.radius),(this.mapSize.x!==512||this.mapSize.y!==512)&&(e.mapSize=this.mapSize.toArray()),e.camera=this.camera.toJSON(!1).object,delete e.camera.matrix,e}},fa=new D,da=new It,Qn=new D,ao=class extends yt{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new Ke,this.projectionMatrix=new Ke,this.projectionMatrixInverse=new Ke,this.coordinateSystem=Gn,this._reversedDepth=!1}get reversedDepth(){return this._reversedDepth}copy(e,t){return super.copy(e,t),this.matrixWorldInverse.copy(e.matrixWorldInverse),this.projectionMatrix.copy(e.projectionMatrix),this.projectionMatrixInverse.copy(e.projectionMatrixInverse),this.coordinateSystem=e.coordinateSystem,this}getWorldDirection(e){return super.getWorldDirection(e).negate()}updateMatrixWorld(e){super.updateMatrixWorld(e),this.matrixWorld.decompose(fa,da,Qn),Qn.x===1&&Qn.y===1&&Qn.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(fa,da,Qn.set(1,1,1)).invert()}updateWorldMatrix(e,t,n=!1){super.updateWorldMatrix(e,t,n),this.matrixWorld.decompose(fa,da,Qn),Qn.x===1&&Qn.y===1&&Qn.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(fa,da,Qn.set(1,1,1)).invert()}clone(){return new this.constructor().copy(this)}},ki=new D,Qu=new ye,ef=new ye,Ht=class extends ao{constructor(e=50,t=1,n=.1,i=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=e,this.zoom=1,this.near=n,this.far=i,this.focus=10,this.aspect=t,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.fov=e.fov,this.zoom=e.zoom,this.near=e.near,this.far=e.far,this.focus=e.focus,this.aspect=e.aspect,this.view=e.view===null?null:Object.assign({},e.view),this.filmGauge=e.filmGauge,this.filmOffset=e.filmOffset,this}setFocalLength(e){let t=.5*this.getFilmHeight()/e;this.fov=ms*2*Math.atan(t),this.updateProjectionMatrix()}getFocalLength(){let e=Math.tan(Ur*.5*this.fov);return .5*this.getFilmHeight()/e}getEffectiveFOV(){return ms*2*Math.atan(Math.tan(Ur*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(e,t,n){ki.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),t.set(ki.x,ki.y).multiplyScalar(-e/ki.z),ki.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),n.set(ki.x,ki.y).multiplyScalar(-e/ki.z)}getViewSize(e,t){return this.getViewBounds(e,Qu,ef),t.subVectors(ef,Qu)}setViewOffset(e,t,n,i,r,o){this.aspect=e/t,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=this.near,t=e*Math.tan(Ur*.5*this.fov)/this.zoom,n=2*t,i=this.aspect*n,r=-.5*i,o=this.view;if(this.view!==null&&this.view.enabled){let l=o.fullWidth,c=o.fullHeight;r+=o.offsetX*i/l,t-=o.offsetY*n/c,i*=o.width/l,n*=o.height/c}let a=this.filmOffset;a!==0&&(r+=e*a/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+i,t,t-n,e,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.fov=this.fov,t.object.zoom=this.zoom,t.object.near=this.near,t.object.far=this.far,t.object.focus=this.focus,t.object.aspect=this.aspect,this.view!==null&&(t.object.view=Object.assign({},this.view)),t.object.filmGauge=this.filmGauge,t.object.filmOffset=this.filmOffset,t}},kc=class extends oo{constructor(){super(new Ht(50,1,.5,500)),this.isSpotLightShadow=!0,this.focus=1,this.aspect=1}updateMatrices(e){let t=this.camera,n=ms*2*e.angle*this.focus,i=this.mapSize.width/this.mapSize.height*this.aspect,r=e.distance||t.far;(n!==t.fov||i!==t.aspect||r!==t.far)&&(t.fov=n,t.aspect=i,t.far=r,t.updateProjectionMatrix()),super.updateMatrices(e)}copy(e){return super.copy(e),this.focus=e.focus,this}},lo=class extends Ms{constructor(e,t,n=0,i=Math.PI/3,r=0,o=2){super(e,t),this.isSpotLight=!0,this.type="SpotLight",this.position.copy(yt.DEFAULT_UP),this.updateMatrix(),this.target=new yt,this.distance=n,this.angle=i,this.penumbra=r,this.decay=o,this.map=null,this.shadow=new kc}get power(){return this.intensity*Math.PI}set power(e){this.intensity=e/Math.PI}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.angle=e.angle,this.penumbra=e.penumbra,this.decay=e.decay,this.target=e.target.clone(),this.map=e.map,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.angle=this.angle,t.object.decay=this.decay,t.object.penumbra=this.penumbra,t.object.target=this.target.uuid,this.map&&this.map.isTexture&&(t.object.map=this.map.toJSON(e).uuid),t.object.shadow=this.shadow.toJSON(),t}},Hc=class extends oo{constructor(){super(new Ht(90,1,.5,500)),this.isPointLightShadow=!0}},Xn=class extends Ms{constructor(e,t,n=0,i=2){super(e,t),this.isPointLight=!0,this.type="PointLight",this.distance=n,this.decay=i,this.shadow=new Hc}get power(){return this.intensity*4*Math.PI}set power(e){this.intensity=e/(4*Math.PI)}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.decay=e.decay,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.decay=this.decay,t.object.shadow=this.shadow.toJSON(),t}},li=class extends ao{constructor(e=-1,t=1,n=1,i=-1,r=.1,o=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=e,this.right=t,this.top=n,this.bottom=i,this.near=r,this.far=o,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.left=e.left,this.right=e.right,this.top=e.top,this.bottom=e.bottom,this.near=e.near,this.far=e.far,this.zoom=e.zoom,this.view=e.view===null?null:Object.assign({},e.view),this}setViewOffset(e,t,n,i,r,o){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=(this.right-this.left)/(2*this.zoom),t=(this.top-this.bottom)/(2*this.zoom),n=(this.right+this.left)/2,i=(this.top+this.bottom)/2,r=n-e,o=n+e,a=i+t,l=i-t;if(this.view!==null&&this.view.enabled){let c=(this.right-this.left)/this.view.fullWidth/this.zoom,h=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=c*this.view.offsetX,o=r+c*this.view.width,a-=h*this.view.offsetY,l=a-h*this.view.height}this.projectionMatrix.makeOrthographic(r,o,a,l,this.near,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.zoom=this.zoom,t.object.left=this.left,t.object.right=this.right,t.object.top=this.top,t.object.bottom=this.bottom,t.object.near=this.near,t.object.far=this.far,this.view!==null&&(t.object.view=Object.assign({},this.view)),t}},Vc=class extends oo{constructor(){super(new li(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},Yi=class extends Ms{constructor(e,t){super(e,t),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(yt.DEFAULT_UP),this.updateMatrix(),this.target=new yt,this.shadow=new Vc}dispose(){super.dispose(),this.shadow.dispose()}copy(e){return super.copy(e),this.target=e.target.clone(),this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.shadow=this.shadow.toJSON(),t.object.target=this.target.uuid,t}};var Ri=class{static extractUrlBase(e){let t=e.lastIndexOf("/");return t===-1?"./":e.slice(0,t+1)}static resolveURL(e,t){return typeof e!="string"||e===""?"":(/^https?:\/\//i.test(t)&&/^\//.test(e)&&(t=t.replace(/(^https?:\/\/[^\/]+).*/i,"$1")),/^(https?:)?\/\//i.test(e)||/^data:.*,.*$/i.test(e)||/^blob:.*$/i.test(e)?e:t+e)}};var Nc=new WeakMap,co=class extends ai{constructor(e){super(e),this.isImageBitmapLoader=!0,typeof createImageBitmap>"u"&&Be("ImageBitmapLoader: createImageBitmap() not supported."),typeof fetch>"u"&&Be("ImageBitmapLoader: fetch() not supported."),this.options={premultiplyAlpha:"none"},this._abortController=new AbortController}setOptions(e){return this.options=e,this}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,o=ei.get(`image-bitmap:${e}`);if(o!==void 0){if(r.manager.itemStart(e),o.then){o.then(c=>{Nc.has(o)===!0?(i&&i(Nc.get(o)),r.manager.itemError(e),r.manager.itemEnd(e)):(t&&t(c),r.manager.itemEnd(e))});return}setTimeout(function(){t&&t(o),r.manager.itemEnd(e)},0);return}let a={};a.credentials=this.crossOrigin==="anonymous"?"same-origin":"include",a.headers=this.requestHeader,a.signal=typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal;let l=fetch(e,a).then(function(c){return c.blob()}).then(function(c){return createImageBitmap(c,Object.assign(r.options,{colorSpaceConversion:"none"}))}).then(function(c){ei.add(`image-bitmap:${e}`,c),t&&t(c),r.manager.itemEnd(e)}).catch(function(c){i&&i(c),Nc.set(l,c),ei.remove(`image-bitmap:${e}`),r.manager.itemError(e),r.manager.itemEnd(e)});ei.add(`image-bitmap:${e}`,l),r.manager.itemStart(e)}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var Gs=-90,Ws=1,qa=class extends yt{constructor(e,t,n){super(),this.type="CubeCamera",this.renderTarget=n,this.coordinateSystem=null,this.activeMipmapLevel=0;let i=new Ht(Gs,Ws,e,t);i.layers=this.layers,this.add(i);let r=new Ht(Gs,Ws,e,t);r.layers=this.layers,this.add(r);let o=new Ht(Gs,Ws,e,t);o.layers=this.layers,this.add(o);let a=new Ht(Gs,Ws,e,t);a.layers=this.layers,this.add(a);let l=new Ht(Gs,Ws,e,t);l.layers=this.layers,this.add(l);let c=new Ht(Gs,Ws,e,t);c.layers=this.layers,this.add(c)}updateCoordinateSystem(){let e=this.coordinateSystem,t=this.children.concat(),[n,i,r,o,a,l]=t;for(let c of t)this.remove(c);if(e===Gn)n.up.set(0,1,0),n.lookAt(1,0,0),i.up.set(0,1,0),i.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),o.up.set(0,0,1),o.lookAt(0,-1,0),a.up.set(0,1,0),a.lookAt(0,0,1),l.up.set(0,1,0),l.lookAt(0,0,-1);else if(e===Ys)n.up.set(0,-1,0),n.lookAt(-1,0,0),i.up.set(0,-1,0),i.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),o.up.set(0,0,-1),o.lookAt(0,-1,0),a.up.set(0,-1,0),a.lookAt(0,0,1),l.up.set(0,-1,0),l.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+e);for(let c of t)this.add(c),c.updateMatrixWorld()}update(e,t){this.parent===null&&this.updateMatrixWorld();let{renderTarget:n,activeMipmapLevel:i}=this;this.coordinateSystem!==e.coordinateSystem&&(this.coordinateSystem=e.coordinateSystem,this.updateCoordinateSystem());let[r,o,a,l,c,h]=this.children,u=e.getRenderTarget(),f=e.getActiveCubeFace(),d=e.getActiveMipmapLevel(),g=e.xr.enabled;e.xr.enabled=!1;let M=n.texture.generateMipmaps;n.texture.generateMipmaps=!1;let m=!1;e.isWebGLRenderer===!0?m=e.state.buffers.depth.getReversed():m=e.reversedDepthBuffer,e.setRenderTarget(n,0,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,r),e.setRenderTarget(n,1,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,o),e.setRenderTarget(n,2,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,a),e.setRenderTarget(n,3,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,l),e.setRenderTarget(n,4,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,c),n.texture.generateMipmaps=M,e.setRenderTarget(n,5,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,h),e.setRenderTarget(u,f,d),e.xr.enabled=g,n.texture.needsPMREMUpdate=!0}},Ya=class extends Ht{constructor(e=[]){super(),this.isArrayCamera=!0,this.isMultiViewCamera=!1,this.cameras=e}},ho=class{constructor(){this._previousTime=0,this._currentTime=0,this._startTime=performance.now(),this._delta=0,this._elapsed=0,this._timescale=1,this._document=null,this._pageVisibilityHandler=null}connect(e){this._document=e,e.hidden!==void 0&&(this._pageVisibilityHandler=sm.bind(this),e.addEventListener("visibilitychange",this._pageVisibilityHandler,!1))}disconnect(){this._pageVisibilityHandler!==null&&(this._document.removeEventListener("visibilitychange",this._pageVisibilityHandler),this._pageVisibilityHandler=null),this._document=null}getDelta(){return this._delta/1e3}getElapsed(){return this._elapsed/1e3}getTimescale(){return this._timescale}setTimescale(e){return this._timescale=e,this}reset(){return this._currentTime=performance.now()-this._startTime,this}dispose(){this.disconnect()}update(e){return this._pageVisibilityHandler!==null&&this._document.hidden===!0?this._delta=0:(this._previousTime=this._currentTime,this._currentTime=(e!==void 0?e:performance.now())-this._startTime,this._delta=(this._currentTime-this._previousTime)*this._timescale,this._elapsed+=this._delta),this}};function sm(){this._document.hidden===!1&&this.reset()}var Za=class{constructor(e,t,n){this.binding=e,this.valueSize=n;let i,r,o;switch(t){case"quaternion":i=this._slerp,r=this._slerpAdditive,o=this._setAdditiveIdentityQuaternion,this.buffer=new Float64Array(n*6),this._workIndex=5;break;case"string":case"bool":i=this._select,r=this._select,o=this._setAdditiveIdentityOther,this.buffer=new Array(n*5);break;default:i=this._lerp,r=this._lerpAdditive,o=this._setAdditiveIdentityNumeric,this.buffer=new Float64Array(n*5)}this._mixBufferRegion=i,this._mixBufferRegionAdditive=r,this._setIdentity=o,this._origIndex=3,this._addIndex=4,this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,this.useCount=0,this.referenceCount=0}accumulate(e,t){let n=this.buffer,i=this.valueSize,r=e*i+i,o=this.cumulativeWeight;if(o===0){for(let a=0;a!==i;++a)n[r+a]=n[a];o=t}else{o+=t;let a=t/o;this._mixBufferRegion(n,r,0,a,i)}this.cumulativeWeight=o}accumulateAdditive(e){let t=this.buffer,n=this.valueSize,i=n*this._addIndex;this.cumulativeWeightAdditive===0&&this._setIdentity(),this._mixBufferRegionAdditive(t,i,0,e,n),this.cumulativeWeightAdditive+=e}apply(e){let t=this.valueSize,n=this.buffer,i=e*t+t,r=this.cumulativeWeight,o=this.cumulativeWeightAdditive,a=this.binding;if(this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,r<1){let l=t*this._origIndex;this._mixBufferRegion(n,i,l,1-r,t)}o>0&&this._mixBufferRegionAdditive(n,i,this._addIndex*t,1,t);for(let l=t,c=t+t;l!==c;++l)if(n[l]!==n[l+t]){a.setValue(n,i);break}}saveOriginalState(){let e=this.binding,t=this.buffer,n=this.valueSize,i=n*this._origIndex;e.getValue(t,i);for(let r=n,o=i;r!==o;++r)t[r]=t[i+r%n];this._setIdentity(),this.cumulativeWeight=0,this.cumulativeWeightAdditive=0}restoreOriginalState(){let e=this.valueSize*3;this.binding.setValue(this.buffer,e)}_setAdditiveIdentityNumeric(){let e=this._addIndex*this.valueSize,t=e+this.valueSize;for(let n=e;n<t;n++)this.buffer[n]=0}_setAdditiveIdentityQuaternion(){this._setAdditiveIdentityNumeric(),this.buffer[this._addIndex*this.valueSize+3]=1}_setAdditiveIdentityOther(){let e=this._origIndex*this.valueSize,t=this._addIndex*this.valueSize;for(let n=0;n<this.valueSize;n++)this.buffer[t+n]=this.buffer[e+n]}_select(e,t,n,i,r){if(i>=.5)for(let o=0;o!==r;++o)e[t+o]=e[n+o]}_slerp(e,t,n,i){It.slerpFlat(e,t,e,t,e,n,i)}_slerpAdditive(e,t,n,i,r){let o=this._workIndex*r;It.multiplyQuaternionsFlat(e,o,e,t,e,n),It.slerpFlat(e,t,e,t,e,o,i)}_lerp(e,t,n,i,r){let o=1-i;for(let a=0;a!==r;++a){let l=t+a;e[l]=e[l]*o+e[n+a]*i}}_lerpAdditive(e,t,n,i,r){for(let o=0;o!==r;++o){let a=t+o;e[a]=e[a]+e[n+o]*i}}},ah="\\[\\]\\.:\\/",rm=new RegExp("["+ah+"]","g"),lh="[^"+ah+"]",om="[^"+ah.replace("\\.","")+"]",am=/((?:WC+[\/:])*)/.source.replace("WC",lh),lm=/(WCOD+)?/.source.replace("WCOD",om),cm=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",lh),hm=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",lh),um=new RegExp("^"+am+lm+cm+hm+"$"),fm=["material","materials","bones","map"],Gc=class{constructor(e,t,n){let i=n||St.parseTrackName(t);this._targetGroup=e,this._bindings=e.subscribe_(t,i)}getValue(e,t){this.bind();let n=this._targetGroup.nCachedObjects_,i=this._bindings[n];i!==void 0&&i.getValue(e,t)}setValue(e,t){let n=this._bindings;for(let i=this._targetGroup.nCachedObjects_,r=n.length;i!==r;++i)n[i].setValue(e,t)}bind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].bind()}unbind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].unbind()}},St=class s{constructor(e,t,n){this.path=t,this.parsedPath=n||s.parseTrackName(t),this.node=s.findNode(e,this.parsedPath.nodeName),this.rootNode=e,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(e,t,n){return e&&e.isAnimationObjectGroup?new s.Composite(e,t,n):new s(e,t,n)}static sanitizeNodeName(e){return e.replace(/\s/g,"_").replace(rm,"")}static parseTrackName(e){let t=um.exec(e);if(t===null)throw new Error("THREE.PropertyBinding: Cannot parse trackName: "+e);let n={nodeName:t[2],objectName:t[3],objectIndex:t[4],propertyName:t[5],propertyIndex:t[6]},i=n.nodeName&&n.nodeName.lastIndexOf(".");if(i!==void 0&&i!==-1){let r=n.nodeName.substring(i+1);fm.indexOf(r)!==-1&&(n.nodeName=n.nodeName.substring(0,i),n.objectName=r)}if(n.propertyName===null||n.propertyName.length===0)throw new Error("THREE.PropertyBinding: can not parse propertyName from trackName: "+e);return n}static findNode(e,t){if(t===void 0||t===""||t==="."||t===-1||t===e.name||t===e.uuid)return e;if(e.skeleton){let n=e.skeleton.getBoneByName(t);if(n!==void 0)return n}if(e.children){let n=function(r){for(let o=0;o<r.length;o++){let a=r[o];if(a.name===t||a.uuid===t)return a;let l=n(a.children);if(l)return l}return null},i=n(e.children);if(i)return i}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(e,t){e[t]=this.targetObject[this.propertyName]}_getValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)e[t++]=n[i]}_getValue_arrayElement(e,t){e[t]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(e,t){this.resolvedProperty.toArray(e,t)}_setValue_direct(e,t){this.targetObject[this.propertyName]=e[t]}_setValue_direct_setNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++]}_setValue_array_setNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(e,t){this.resolvedProperty[this.propertyIndex]=e[t]}_setValue_arrayElement_setNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(e,t){this.resolvedProperty.fromArray(e,t)}_setValue_fromArray_setNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(e,t){this.bind(),this.getValue(e,t)}_setValue_unbound(e,t){this.bind(),this.setValue(e,t)}bind(){let e=this.node,t=this.parsedPath,n=t.objectName,i=t.propertyName,r=t.propertyIndex;if(e||(e=s.findNode(this.rootNode,t.nodeName),this.node=e),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!e){Be("PropertyBinding: No target node found for track: "+this.path+".");return}if(n){let c=t.objectIndex;switch(n){case"materials":if(!e.material){Ze("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.materials){Ze("PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}e=e.material.materials;break;case"bones":if(!e.skeleton){Ze("PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}e=e.skeleton.bones;for(let h=0;h<e.length;h++)if(e[h].name===c){c=h;break}break;case"map":if("map"in e){e=e.map;break}if(!e.material){Ze("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.map){Ze("PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}e=e.material.map;break;default:if(e[n]===void 0){Ze("PropertyBinding: Can not bind to objectName of node undefined.",this);return}e=e[n]}if(c!==void 0){if(e[c]===void 0){Ze("PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,e);return}e=e[c]}}let o=e[i];if(o===void 0){let c=t.nodeName;Ze("PropertyBinding: Trying to update property for track: "+c+"."+i+" but it wasn't found.",e);return}let a=this.Versioning.None;this.targetObject=e,e.isMaterial===!0?a=this.Versioning.NeedsUpdate:e.isObject3D===!0&&(a=this.Versioning.MatrixWorldNeedsUpdate);let l=this.BindingType.Direct;if(r!==void 0){if(i==="morphTargetInfluences"){if(!e.geometry){Ze("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!e.geometry.morphAttributes){Ze("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}e.morphTargetDictionary[r]!==void 0&&(r=e.morphTargetDictionary[r])}l=this.BindingType.ArrayElement,this.resolvedProperty=o,this.propertyIndex=r}else o.fromArray!==void 0&&o.toArray!==void 0?(l=this.BindingType.HasFromToArray,this.resolvedProperty=o):Array.isArray(o)?(l=this.BindingType.EntireArray,this.resolvedProperty=o):this.propertyName=i;this.getValue=this.GetterByBindingType[l],this.setValue=this.SetterByBindingTypeAndVersioning[l][a]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};St.Composite=Gc;St.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};St.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};St.prototype.GetterByBindingType=[St.prototype._getValue_direct,St.prototype._getValue_array,St.prototype._getValue_arrayElement,St.prototype._getValue_toArray];St.prototype.SetterByBindingTypeAndVersioning=[[St.prototype._setValue_direct,St.prototype._setValue_direct_setNeedsUpdate,St.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[St.prototype._setValue_array,St.prototype._setValue_array_setNeedsUpdate,St.prototype._setValue_array_setMatrixWorldNeedsUpdate],[St.prototype._setValue_arrayElement,St.prototype._setValue_arrayElement_setNeedsUpdate,St.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[St.prototype._setValue_fromArray,St.prototype._setValue_fromArray_setNeedsUpdate,St.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var Ka=class{constructor(e,t,n=null,i=t.blendMode){this._mixer=e,this._clip=t,this._localRoot=n,this.blendMode=i;let r=t.tracks,o=r.length,a=new Array(o),l={endingStart:ls,endingEnd:ls};for(let c=0;c!==o;++c){let h=r[c].createInterpolant(null);a[c]=h,h.settings=l}this._interpolantSettings=l,this._interpolants=a,this._propertyBindings=new Array(o),this._cacheIndex=null,this._byClipCacheIndex=null,this._timeScaleInterpolant=null,this._restoreTimeScale=null,this._weightInterpolant=null,this.loop=Af,this._loopCount=-1,this._startTime=null,this.time=0,this.timeScale=1,this._effectiveTimeScale=1,this.weight=1,this._effectiveWeight=1,this.repetitions=1/0,this.paused=!1,this.enabled=!0,this.clampWhenFinished=!1,this.zeroSlopeAtStart=!0,this.zeroSlopeAtEnd=!0}play(){return this._mixer._activateAction(this),this}stop(){return this._mixer._deactivateAction(this),this.reset()}reset(){return this.paused=!1,this.enabled=!0,this.time=0,this._loopCount=-1,this._startTime=null,this.stopFading().stopWarping()}isRunning(){return this.enabled&&!this.paused&&this.timeScale!==0&&this._startTime===null&&this._mixer._isActiveAction(this)}isScheduled(){return this._mixer._isActiveAction(this)}startAt(e){return this._startTime=e,this}setLoop(e,t){return this.loop=e,this.repetitions=t,this}setEffectiveWeight(e){return this.weight=e,this._effectiveWeight=this.enabled?e:0,this.stopFading()}getEffectiveWeight(){return this._effectiveWeight}fadeIn(e){return this._scheduleFading(e,0,1)}fadeOut(e){return this._scheduleFading(e,1,0)}crossFadeFrom(e,t,n=!1){if(e.fadeOut(t),this.fadeIn(t),n===!0){let i=this._clip.duration,r=e._clip.duration,o=r/i,a=i/r;e._restoreTimeScale=e.timeScale,this._restoreTimeScale=this.timeScale,e.warp(1,o,t),this.warp(a,1,t)}return this}crossFadeTo(e,t,n=!1){return e.crossFadeFrom(this,t,n)}stopFading(){let e=this._weightInterpolant;return e!==null&&(this._weightInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this}setEffectiveTimeScale(e){return this.timeScale=e,this._effectiveTimeScale=this.paused?0:e,this.stopWarping()}getEffectiveTimeScale(){return this._effectiveTimeScale}setDuration(e){return this.timeScale=this._clip.duration/e,this.stopWarping()}syncWith(e){return this.time=e.time,this.timeScale=e.timeScale,this.stopWarping()}halt(e){return this.warp(this._effectiveTimeScale,0,e)}warp(e,t,n){let i=this._mixer,r=i.time,o=this.timeScale,a=this._timeScaleInterpolant;a===null&&(a=i._lendControlInterpolant(),this._timeScaleInterpolant=a);let l=a.parameterPositions,c=a.sampleValues;return l[0]=r,l[1]=r+n,c[0]=e/o,c[1]=t/o,this}stopWarping(){let e=this._timeScaleInterpolant;return e!==null&&(this._timeScaleInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this._restoreTimeScale=null,this}getMixer(){return this._mixer}getClip(){return this._clip}getRoot(){return this._localRoot||this._mixer._root}_update(e,t,n,i){if(!this.enabled){this._updateWeight(e);return}let r=this._startTime;if(r!==null){let l=(e-r)*n;l<0||n===0?t=0:(this._startTime=null,t=n*l)}t*=this._updateTimeScale(e);let o=this._updateTime(t),a=this._updateWeight(e);if(a>0){let l=this._interpolants,c=this._propertyBindings;switch(this.blendMode){case Cf:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(o),c[h].accumulateAdditive(a);break;case zl:default:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(o),c[h].accumulate(i,a)}}}_updateWeight(e){let t=0;if(this.enabled){t=this.weight;let n=this._weightInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(this.stopFading(),i===0&&(this.enabled=!1))}}return this._effectiveWeight=t,t}_updateTimeScale(e){let t=0;if(!this.paused){t=this.timeScale;let n=this._timeScaleInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(t===0?this.paused=!0:(this._restoreTimeScale!==null&&(t=this._restoreTimeScale),this.timeScale=t),this.stopWarping())}}return this._effectiveTimeScale=t,t}_updateTime(e){let t=this._clip.duration,n=this.loop,i=this.time+e,r=this._loopCount,o=n===Rf;if(e===0)return r===-1?i:o&&(r&1)===1?t-i:i;if(n===Bl){r===-1&&(this._loopCount=0,this._setEndings(!0,!0,!1));e:{if(i>=t)i=t;else if(i<0)i=0;else{this.time=i;break e}this.clampWhenFinished?this.paused=!0:this.enabled=!1,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e<0?-1:1})}}else{if(r===-1&&(e>=0?(r=0,this._setEndings(!0,this.repetitions===0,o)):this._setEndings(this.repetitions===0,!0,o)),i>=t||i<0){let a=Math.floor(i/t);i-=t*a,r+=Math.abs(a);let l=this.repetitions-r;if(l<=0)this.clampWhenFinished?this.paused=!0:this.enabled=!1,i=e>0?t:0,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e>0?1:-1});else{if(l===1){let c=e<0;this._setEndings(c,!c,o)}else this._setEndings(!1,!1,o);this._loopCount=r,this.time=i,this._mixer.dispatchEvent({type:"loop",action:this,loopDelta:a})}}else this._loopCount=r,this.time=i;if(o&&(r&1)===1)return t-i}return i}_setEndings(e,t,n){let i=this._interpolantSettings;n?(i.endingStart=cs,i.endingEnd=cs):(e?i.endingStart=this.zeroSlopeAtStart?cs:ls:i.endingStart=zr,t?i.endingEnd=this.zeroSlopeAtEnd?cs:ls:i.endingEnd=zr)}_scheduleFading(e,t,n){let i=this._mixer,r=i.time,o=this._weightInterpolant;o===null&&(o=i._lendControlInterpolant(),this._weightInterpolant=o);let a=o.parameterPositions,l=o.sampleValues;return a[0]=r,l[0]=t,a[1]=r+e,l[1]=n,this}},dm=new Float32Array(1),uo=class extends Nn{constructor(e){super(),this._root=e,this._initMemoryManager(),this._accuIndex=0,this.time=0,this.timeScale=1,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}_bindAction(e,t){let n=e._localRoot||this._root,i=e._clip.tracks,r=i.length,o=e._propertyBindings,a=e._interpolants,l=n.uuid,c=this._bindingsByRootAndName,h=c[l];h===void 0&&(h={},c[l]=h);for(let u=0;u!==r;++u){let f=i[u],d=f.name,g=h[d];if(g!==void 0)++g.referenceCount,o[u]=g;else{if(g=o[u],g!==void 0){g._cacheIndex===null&&(++g.referenceCount,this._addInactiveBinding(g,l,d));continue}let M=t&&t._propertyBindings[u].binding.parsedPath;g=new Za(St.create(n,d,M),f.ValueTypeName,f.getValueSize()),++g.referenceCount,this._addInactiveBinding(g,l,d),o[u]=g}a[u].resultBuffer=g.buffer}}_activateAction(e){if(!this._isActiveAction(e)){if(e._cacheIndex===null){let n=(e._localRoot||this._root).uuid,i=e._clip.uuid,r=this._actionsByClip[i];this._bindAction(e,r&&r.knownActions[0]),this._addInactiveAction(e,i,n)}let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];r.useCount++===0&&(this._lendBinding(r),r.saveOriginalState())}this._lendAction(e)}}_deactivateAction(e){if(this._isActiveAction(e)){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.useCount===0&&(r.restoreOriginalState(),this._takeBackBinding(r))}this._takeBackAction(e)}}_initMemoryManager(){this._actions=[],this._nActiveActions=0,this._actionsByClip={},this._bindings=[],this._nActiveBindings=0,this._bindingsByRootAndName={},this._controlInterpolants=[],this._nActiveControlInterpolants=0;let e=this;this.stats={actions:{get total(){return e._actions.length},get inUse(){return e._nActiveActions}},bindings:{get total(){return e._bindings.length},get inUse(){return e._nActiveBindings}},controlInterpolants:{get total(){return e._controlInterpolants.length},get inUse(){return e._nActiveControlInterpolants}}}}_isActiveAction(e){let t=e._cacheIndex;return t!==null&&t<this._nActiveActions}_addInactiveAction(e,t,n){let i=this._actions,r=this._actionsByClip,o=r[t];if(o===void 0)o={knownActions:[e],actionByRoot:{}},e._byClipCacheIndex=0,r[t]=o;else{let a=o.knownActions;e._byClipCacheIndex=a.length,a.push(e)}e._cacheIndex=i.length,i.push(e),o.actionByRoot[n]=e}_removeInactiveAction(e){let t=this._actions,n=t[t.length-1],i=e._cacheIndex;n._cacheIndex=i,t[i]=n,t.pop(),e._cacheIndex=null;let r=e._clip.uuid,o=this._actionsByClip,a=o[r],l=a.knownActions,c=l[l.length-1],h=e._byClipCacheIndex;c._byClipCacheIndex=h,l[h]=c,l.pop(),e._byClipCacheIndex=null;let u=a.actionByRoot,f=(e._localRoot||this._root).uuid;delete u[f],l.length===0&&delete o[r],this._removeInactiveBindingsForAction(e)}_removeInactiveBindingsForAction(e){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.referenceCount===0&&this._removeInactiveBinding(r)}}_lendAction(e){let t=this._actions,n=e._cacheIndex,i=this._nActiveActions++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackAction(e){let t=this._actions,n=e._cacheIndex,i=--this._nActiveActions,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_addInactiveBinding(e,t,n){let i=this._bindingsByRootAndName,r=this._bindings,o=i[t];o===void 0&&(o={},i[t]=o),o[n]=e,e._cacheIndex=r.length,r.push(e)}_removeInactiveBinding(e){let t=this._bindings,n=e.binding,i=n.rootNode.uuid,r=n.path,o=this._bindingsByRootAndName,a=o[i],l=t[t.length-1],c=e._cacheIndex;l._cacheIndex=c,t[c]=l,t.pop(),delete a[r],Object.keys(a).length===0&&delete o[i]}_lendBinding(e){let t=this._bindings,n=e._cacheIndex,i=this._nActiveBindings++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackBinding(e){let t=this._bindings,n=e._cacheIndex,i=--this._nActiveBindings,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_lendControlInterpolant(){let e=this._controlInterpolants,t=this._nActiveControlInterpolants++,n=e[t];return n===void 0&&(n=new no(new Float32Array(2),new Float32Array(2),1,dm),n.__cacheIndex=t,e[t]=n),n}_takeBackControlInterpolant(e){let t=this._controlInterpolants,n=e.__cacheIndex,i=--this._nActiveControlInterpolants,r=t[i];e.__cacheIndex=i,t[i]=e,r.__cacheIndex=n,t[n]=r}clipAction(e,t,n){let i=t||this._root,r=i.uuid,o=typeof e=="string"?ys.findByName(i,e):e,a=o!==null?o.uuid:e,l=this._actionsByClip[a],c=null;if(n===void 0&&(o!==null?n=o.blendMode:n=zl),l!==void 0){let u=l.actionByRoot[r];if(u!==void 0&&u.blendMode===n)return u;c=l.knownActions[0],o===null&&(o=c._clip)}if(o===null)return null;let h=new Ka(this,o,t,n);return this._bindAction(h,c),this._addInactiveAction(h,a,r),h}existingAction(e,t){let n=t||this._root,i=n.uuid,r=typeof e=="string"?ys.findByName(n,e):e,o=r?r.uuid:e,a=this._actionsByClip[o];return a!==void 0&&a.actionByRoot[i]||null}stopAllAction(){let e=this._actions,t=this._nActiveActions;for(let n=t-1;n>=0;--n)e[n].stop();return this}update(e){e*=this.timeScale;let t=this._actions,n=this._nActiveActions,i=this.time+=e,r=Math.sign(e),o=this._accuIndex^=1;for(let c=0;c!==n;++c)t[c]._update(i,e,r,o);let a=this._bindings,l=this._nActiveBindings;for(let c=0;c!==l;++c)a[c].apply(o);return this}setTime(e){this.time=0;for(let t=0;t<this._actions.length;t++)this._actions[t].time=0;return this.update(e)}getRoot(){return this._root}uncacheClip(e){let t=this._actions,n=e.uuid,i=this._actionsByClip,r=i[n];if(r!==void 0){let o=r.knownActions;for(let a=0,l=o.length;a!==l;++a){let c=o[a];this._deactivateAction(c);let h=c._cacheIndex,u=t[t.length-1];c._cacheIndex=null,c._byClipCacheIndex=null,u._cacheIndex=h,t[h]=u,t.pop(),this._removeInactiveBindingsForAction(c)}delete i[n]}}uncacheRoot(e){let t=e.uuid,n=this._actionsByClip;for(let o in n){let a=n[o].actionByRoot,l=a[t];l!==void 0&&(this._deactivateAction(l),this._removeInactiveAction(l))}let i=this._bindingsByRootAndName,r=i[t];if(r!==void 0)for(let o in r){let a=r[o];a.restoreOriginalState(),this._removeInactiveBinding(a)}}uncacheAction(e,t){let n=this.existingAction(e,t);n!==null&&(this._deactivateAction(n),this._removeInactiveAction(n))}};var tf=new Ke,fo=class{constructor(e,t,n=0,i=1/0){this.ray=new ni(e,t),this.near=n,this.far=i,this.camera=null,this.layers=new js,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(e,t){this.ray.set(e,t)}setFromCamera(e,t){t.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(e.x,e.y,.5).unproject(t).sub(this.ray.origin).normalize(),this.camera=t):t.isOrthographicCamera?(this.ray.origin.set(e.x,e.y,t.projectionMatrix.elements[14]).unproject(t),this.ray.direction.set(0,0,-1).transformDirection(t.matrixWorld),this.camera=t):Ze("Raycaster: Unsupported camera type: "+t.type)}setFromXRController(e){return tf.identity().extractRotation(e.matrixWorld),this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(tf),this}intersectObject(e,t=!0,n=[]){return Wc(e,this,n,t),n.sort(nf),n}intersectObjects(e,t=!0,n=[]){for(let i=0,r=e.length;i<r;i++)Wc(e[i],this,n,t);return n.sort(nf),n}};function nf(s,e){return s.distance-e.distance}function Wc(s,e,t,n){let i=!0;if(s.layers.test(e.layers)&&s.raycast(e,t)===!1&&(i=!1),i===!0&&n===!0){let r=s.children;for(let o=0,a=r.length;o<a;o++)Wc(r[o],e,t,!0)}}var ur=class{constructor(e=1,t=0,n=0){this.radius=e,this.phi=t,this.theta=n}set(e,t,n){return this.radius=e,this.phi=t,this.theta=n,this}copy(e){return this.radius=e.radius,this.phi=e.phi,this.theta=e.theta,this}makeSafe(){return this.phi=tt(this.phi,1e-6,Math.PI-1e-6),this}setFromVector3(e){return this.setFromCartesianCoords(e.x,e.y,e.z)}setFromCartesianCoords(e,t,n){return this.radius=Math.sqrt(e*e+t*t+n*n),this.radius===0?(this.theta=0,this.phi=0):(this.theta=Math.atan2(e,n),this.phi=Math.acos(tt(t/this.radius,-1,1))),this}clone(){return new this.constructor().copy(this)}};var Xc=class s{static{s.prototype.isMatrix2=!0}constructor(e,t,n,i){this.elements=[1,0,0,1],e!==void 0&&this.set(e,t,n,i)}identity(){return this.set(1,0,0,1),this}fromArray(e,t=0){for(let n=0;n<4;n++)this.elements[n]=e[n+t];return this}set(e,t,n,i){let r=this.elements;return r[0]=e,r[2]=t,r[1]=n,r[3]=i,this}};var po=class extends Nn{constructor(e,t=null){super(),this.object=e,this.domElement=t,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(e){if(e===void 0){Be("Controls: connect() now requires an element.");return}this.domElement!==null&&this.disconnect(),this.domElement=e}disconnect(){}dispose(){}update(){}};function ch(s,e,t,n){let i=pm(n);switch(t){case eh:return s*e;case sl:return s*e/i.components*i.byteLength;case rl:return s*e/i.components*i.byteLength;case $i:return s*e*2/i.components*i.byteLength;case ol:return s*e*2/i.components*i.byteLength;case th:return s*e*3/i.components*i.byteLength;case wn:return s*e*4/i.components*i.byteLength;case al:return s*e*4/i.components*i.byteLength;case So:case To:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case Eo:case wo:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case cl:case ul:return Math.max(s,16)*Math.max(e,8)/4;case ll:case hl:return Math.max(s,8)*Math.max(e,8)/2;case fl:case dl:case ml:case gl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case pl:case Ao:case _l:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case xl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case vl:return Math.floor((s+4)/5)*Math.floor((e+3)/4)*16;case yl:return Math.floor((s+4)/5)*Math.floor((e+4)/5)*16;case Ml:return Math.floor((s+5)/6)*Math.floor((e+4)/5)*16;case bl:return Math.floor((s+5)/6)*Math.floor((e+5)/6)*16;case Sl:return Math.floor((s+7)/8)*Math.floor((e+4)/5)*16;case Tl:return Math.floor((s+7)/8)*Math.floor((e+5)/6)*16;case El:return Math.floor((s+7)/8)*Math.floor((e+7)/8)*16;case wl:return Math.floor((s+9)/10)*Math.floor((e+4)/5)*16;case Al:return Math.floor((s+9)/10)*Math.floor((e+5)/6)*16;case Rl:return Math.floor((s+9)/10)*Math.floor((e+7)/8)*16;case Cl:return Math.floor((s+9)/10)*Math.floor((e+9)/10)*16;case Pl:return Math.floor((s+11)/12)*Math.floor((e+9)/10)*16;case Il:return Math.floor((s+11)/12)*Math.floor((e+11)/12)*16;case Ll:case Dl:case Nl:return Math.ceil(s/4)*Math.ceil(e/4)*16;case Ul:case Fl:return Math.ceil(s/4)*Math.ceil(e/4)*8;case Ro:case Ol:return Math.ceil(s/4)*Math.ceil(e/4)*16}throw new Error(`Unable to determine texture byte length for ${t} format.`)}function pm(s){switch(s){case gn:case Jc:return{byteLength:1,components:1};case pr:case jc:case Xt:return{byteLength:2,components:1};case nl:case il:return{byteLength:2,components:4};case Zn:case tl:case En:return{byteLength:4,components:1};case $c:case Qc:return{byteLength:4,components:3}}throw new Error(`THREE.TextureUtils: Unknown texture type ${s}.`)}typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"185"}}));typeof window<"u"&&(window.__THREE__?Be("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="185");function fd(){let s=null,e=!1,t=null,n=null;function i(r,o){t(r,o),n=s.requestAnimationFrame(i)}return{start:function(){e!==!0&&t!==null&&s!==null&&(n=s.requestAnimationFrame(i),e=!0)},stop:function(){s!==null&&s.cancelAnimationFrame(n),e=!1},setAnimationLoop:function(r){t=r},setContext:function(r){s=r}}}function gm(s){let e=new WeakMap;function t(a,l){let c=a.array,h=a.usage,u=c.byteLength,f=s.createBuffer();s.bindBuffer(l,f),s.bufferData(l,c,h),a.onUploadCallback();let d;if(c instanceof Float32Array)d=s.FLOAT;else if(typeof Float16Array<"u"&&c instanceof Float16Array)d=s.HALF_FLOAT;else if(c instanceof Uint16Array)a.isFloat16BufferAttribute?d=s.HALF_FLOAT:d=s.UNSIGNED_SHORT;else if(c instanceof Int16Array)d=s.SHORT;else if(c instanceof Uint32Array)d=s.UNSIGNED_INT;else if(c instanceof Int32Array)d=s.INT;else if(c instanceof Int8Array)d=s.BYTE;else if(c instanceof Uint8Array)d=s.UNSIGNED_BYTE;else if(c instanceof Uint8ClampedArray)d=s.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+c);return{buffer:f,type:d,bytesPerElement:c.BYTES_PER_ELEMENT,version:a.version,size:u}}function n(a,l,c){let h=l.array,u=l.updateRanges;if(s.bindBuffer(c,a),u.length===0)s.bufferSubData(c,0,h);else{u.sort((d,g)=>d.start-g.start);let f=0;for(let d=1;d<u.length;d++){let g=u[f],M=u[d];M.start<=g.start+g.count+1?g.count=Math.max(g.count,M.start+M.count-g.start):(++f,u[f]=M)}u.length=f+1;for(let d=0,g=u.length;d<g;d++){let M=u[d];s.bufferSubData(c,M.start*h.BYTES_PER_ELEMENT,h,M.start,M.count)}l.clearUpdateRanges()}l.onUploadCallback()}function i(a){return a.isInterleavedBufferAttribute&&(a=a.data),e.get(a)}function r(a){a.isInterleavedBufferAttribute&&(a=a.data);let l=e.get(a);l&&(s.deleteBuffer(l.buffer),e.delete(a))}function o(a,l){if(a.isInterleavedBufferAttribute&&(a=a.data),a.isGLBufferAttribute){let h=e.get(a);(!h||h.version<a.version)&&e.set(a,{buffer:a.buffer,type:a.type,bytesPerElement:a.elementSize,version:a.version});return}let c=e.get(a);if(c===void 0)e.set(a,t(a,l));else if(c.version<a.version){if(c.size!==a.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");n(c.buffer,a,l),c.version=a.version}}return{get:i,remove:r,update:o}}var _m=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,xm=`#ifdef USE_ALPHAHASH
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
#endif`,vm=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,ym=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,Mm=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,bm=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,Sm=`#ifdef USE_AOMAP
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
#endif`,Tm=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,Em=`#ifdef USE_BATCHING
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
#endif`,wm=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,Am=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,Rm=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,Cm=`float G_BlinnPhong_Implicit( ) {
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
} // validated`,Pm=`#ifdef USE_IRIDESCENCE
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
#endif`,Im=`#ifdef USE_BUMPMAP
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
#endif`,Lm=`#if NUM_CLIPPING_PLANES > 0
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
#endif`,Dm=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,Nm=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,Um=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,Fm=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#endif`,Om=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#endif`,Bm=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec4 vColor;
#endif`,zm=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
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
#endif`,km=`#define PI 3.141592653589793
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
} // validated`,Hm=`#ifdef ENVMAP_TYPE_CUBE_UV
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
#endif`,Vm=`vec3 transformedNormal = objectNormal;
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
#endif`,Gm=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,Wm=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,Xm=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	#ifdef DECODE_VIDEO_TEXTURE_EMISSIVE
		emissiveColor = sRGBTransferEOTF( emissiveColor );
	#endif
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,qm=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,Ym="gl_FragColor = linearToOutputTexel( gl_FragColor );",Zm=`vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferEOTF( in vec4 value ) {
	return vec4( mix( pow( value.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), value.rgb * 0.0773993808, vec3( lessThanEqual( value.rgb, vec3( 0.04045 ) ) ) ), value.a );
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,Km=`#ifdef USE_ENVMAP
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
#endif`,Jm=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
#endif`,jm=`#ifdef USE_ENVMAP
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
#endif`,$m=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,Qm=`#ifdef USE_ENVMAP
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
#endif`,eg=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,tg=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,ng=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,ig=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,sg=`#ifdef USE_GRADIENTMAP
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
}`,rg=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,og=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,ag=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,lg=`uniform bool receiveShadow;
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
#include <lightprobes_pars_fragment>`,cg=`#ifdef USE_ENVMAP
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
	#endif
#endif`,hg=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,ug=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,fg=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,dg=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,pg=`PhysicalMaterial material;
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
#endif`,mg=`uniform sampler2D dfgLUT;
struct PhysicalMaterial {
	vec3 diffuseColor;
	vec3 diffuseContribution;
	vec3 specularColor;
	vec3 specularColorBlended;
	float roughness;
	float metalness;
	float specularF90;
	float dispersion;
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
		vec3 iridescenceF0;
		vec3 iridescenceFresnelDielectric;
		vec3 iridescenceFresnelMetallic;
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
void computeMultiscatteringIridescence( const in vec3 normal, const in vec3 viewDir, const in vec3 specularColor, const in float specularF90, const in float iridescence, const in vec3 iridescenceF0, const in float roughness, inout vec3 singleScatter, inout vec3 multiScatter ) {
#else
void computeMultiscattering( const in vec3 normal, const in vec3 viewDir, const in vec3 specularColor, const in float specularF90, const in float roughness, inout vec3 singleScatter, inout vec3 multiScatter ) {
#endif
	float dotNV = saturate( dot( normal, viewDir ) );
	vec2 fab = texture2D( dfgLUT, vec2( roughness, dotNV ) ).rg;
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
vec3 BRDF_GGX_Multiscatter( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, const in PhysicalMaterial material ) {
	vec3 singleScatter = BRDF_GGX( lightDir, viewDir, normal, material );
	float dotNL = saturate( dot( normal, lightDir ) );
	float dotNV = saturate( dot( normal, viewDir ) );
	vec2 dfgV = texture2D( dfgLUT, vec2( material.roughness, dotNV ) ).rg;
	vec2 dfgL = texture2D( dfgLUT, vec2( material.roughness, dotNL ) ).rg;
	vec3 FssEss_V = material.specularColorBlended * dfgV.x + material.specularF90 * dfgV.y;
	vec3 FssEss_L = material.specularColorBlended * dfgL.x + material.specularF90 * dfgL.y;
	float Ess_V = dfgV.x + dfgV.y;
	float Ess_L = dfgL.x + dfgL.y;
	float Ems_V = 1.0 - Ess_V;
	float Ems_L = 1.0 - Ess_L;
	vec3 Favg = material.specularColorBlended + ( 1.0 - material.specularColorBlended ) * 0.047619;
	vec3 Fms = FssEss_V * FssEss_L * Favg / ( 1.0 - Ems_V * Ems_L * Favg + EPSILON );
	float compensationFactor = Ems_V * Ems_L;
	vec3 multiScatter = Fms * compensationFactor;
	return singleScatter + multiScatter;
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
	reflectedLight.directSpecular += irradiance * BRDF_GGX_Multiscatter( directLight.direction, geometryViewDir, geometryNormal, material );
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseContribution );
}
void RE_IndirectDiffuse_Physical( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight ) {
	vec3 diffuse = irradiance * BRDF_Lambert( material.diffuseContribution );
	#ifdef USE_SHEEN
		float sheenAlbedo = IBLSheenBRDF( geometryNormal, geometryViewDir, material.sheenRoughness );
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
		computeMultiscatteringIridescence( geometryNormal, geometryViewDir, material.specularColor, material.specularF90, material.iridescence, material.iridescenceFresnelDielectric, material.roughness, singleScatteringDielectric, multiScatteringDielectric );
		computeMultiscatteringIridescence( geometryNormal, geometryViewDir, material.diffuseColor, material.specularF90, material.iridescence, material.iridescenceFresnelMetallic, material.roughness, singleScatteringMetallic, multiScatteringMetallic );
	#else
		computeMultiscattering( geometryNormal, geometryViewDir, material.specularColor, material.specularF90, material.roughness, singleScatteringDielectric, multiScatteringDielectric );
		computeMultiscattering( geometryNormal, geometryViewDir, material.diffuseColor, material.specularF90, material.roughness, singleScatteringMetallic, multiScatteringMetallic );
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
}`,gg=`
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
		material.iridescenceFresnelDielectric = evalIridescence( 1.0, material.iridescenceIOR, dotNVi, material.iridescenceThickness, material.specularColor );
		material.iridescenceFresnelMetallic = evalIridescence( 1.0, material.iridescenceIOR, dotNVi, material.iridescenceThickness, material.diffuseColor );
		material.iridescenceFresnel = mix( material.iridescenceFresnelDielectric, material.iridescenceFresnelMetallic, material.metalness );
		material.iridescenceF0 = Schlick_to_F0( material.iridescenceFresnel, 1.0, dotNVi );
	}
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
#endif`,_g=`#if defined( RE_IndirectDiffuse )
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
		radiance += getIBLAnisotropyRadiance( geometryViewDir, geometryNormal, material.roughness, material.anisotropyB, material.anisotropy );
	#else
		radiance += getIBLRadiance( geometryViewDir, geometryNormal, material.roughness );
	#endif
	#ifdef USE_CLEARCOAT
		clearcoatRadiance += getIBLRadiance( geometryViewDir, geometryClearcoatNormal, material.clearcoatRoughness );
	#endif
#endif`,xg=`#if defined( RE_IndirectDiffuse )
	#if defined( LAMBERT ) || defined( PHONG )
		irradiance += iblIrradiance;
	#endif
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,vg=`#ifdef USE_LIGHT_PROBES_GRID
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
#endif`,yg=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,Mg=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,bg=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,Sg=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,Tg=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = sRGBTransferEOTF( sampledDiffuseColor );
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,Eg=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,wg=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
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
#endif`,Ag=`#if defined( USE_POINTS_UV )
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
#endif`,Rg=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,Cg=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,Pg=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,Ig=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,Lg=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,Dg=`#ifdef USE_MORPHTARGETS
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
#endif`,Ng=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,Ug=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
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
vec3 nonPerturbedNormal = normal;`,Fg=`#ifdef USE_NORMALMAP_OBJECTSPACE
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
#endif`,Og=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,Bg=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,zg=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
		#ifdef FLIP_SIDED
			vBitangent = - vBitangent;
		#endif
	#endif
#endif`,kg=`#ifdef USE_NORMALMAP
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
#endif`,Hg=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,Vg=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,Gg=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,Wg=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,Xg=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,qg=`vec3 packNormalToRGB( const in vec3 normal ) {
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
}`,Yg=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,Zg=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,Kg=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,Jg=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,jg=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,$g=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,Qg=`#if NUM_SPOT_LIGHT_COORDS > 0
	varying vec4 vSpotLightCoord[ NUM_SPOT_LIGHT_COORDS ];
#endif
#if NUM_SPOT_LIGHT_MAPS > 0
	uniform sampler2D spotLightMap[ NUM_SPOT_LIGHT_MAPS ];
#endif
#ifdef USE_SHADOWMAP
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
#endif`,e0=`#if NUM_SPOT_LIGHT_COORDS > 0
	uniform mat4 spotLightMatrix[ NUM_SPOT_LIGHT_COORDS ];
	varying vec4 vSpotLightCoord[ NUM_SPOT_LIGHT_COORDS ];
#endif
#ifdef USE_SHADOWMAP
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
#endif`,t0=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
	#ifdef HAS_NORMAL
		vec3 shadowWorldNormal = transformNormalByInverseViewMatrix( transformedNormal, viewMatrix );
	#else
		vec3 shadowWorldNormal = vec3( 0.0 );
	#endif
	vec4 shadowWorldPosition;
#endif
#if defined( USE_SHADOWMAP )
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
#endif`,n0=`float getShadowMask() {
	float shadow = 1.0;
	#ifdef USE_SHADOWMAP
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
}`,i0=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,s0=`#ifdef USE_SKINNING
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
#endif`,r0=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,o0=`#ifdef USE_SKINNING
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
#endif`,a0=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,l0=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,c0=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,h0=`#ifndef saturate
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
vec3 CustomToneMapping( vec3 color ) { return color; }`,u0=`#ifdef USE_TRANSMISSION
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
#endif`,f0=`#ifdef USE_TRANSMISSION
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
#endif`,d0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,p0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,m0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,g0=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,_0=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,x0=`uniform sampler2D t2D;
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
}`,v0=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,y0=`#ifdef ENVMAP_TYPE_CUBE
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
}`,M0=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,b0=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,S0=`#include <common>
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
}`,T0=`#if DEPTH_PACKING == 3200
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
}`,E0=`#define DISTANCE
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
}`,w0=`#define DISTANCE
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
}`,A0=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,R0=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,C0=`uniform float scale;
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
}`,P0=`uniform vec3 diffuse;
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
}`,I0=`#include <common>
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
}`,L0=`uniform vec3 diffuse;
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
}`,D0=`#define LAMBERT
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
}`,N0=`#define LAMBERT
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
}`,U0=`#define MATCAP
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
}`,F0=`#define MATCAP
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
}`,O0=`#define NORMAL
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
}`,B0=`#define NORMAL
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
}`,z0=`#define PHONG
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
}`,k0=`#define PHONG
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
}`,H0=`#define STANDARD
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
}`,V0=`#define STANDARD
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
}`,G0=`#define TOON
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
}`,W0=`#define TOON
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
}`,X0=`uniform float size;
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
}`,q0=`uniform vec3 diffuse;
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
}`,Y0=`#include <common>
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
}`,Z0=`uniform vec3 color;
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
}`,K0=`uniform float rotation;
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
}`,J0=`uniform vec3 diffuse;
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
}`,nt={alphahash_fragment:_m,alphahash_pars_fragment:xm,alphamap_fragment:vm,alphamap_pars_fragment:ym,alphatest_fragment:Mm,alphatest_pars_fragment:bm,aomap_fragment:Sm,aomap_pars_fragment:Tm,batching_pars_vertex:Em,batching_vertex:wm,begin_vertex:Am,beginnormal_vertex:Rm,bsdfs:Cm,iridescence_fragment:Pm,bumpmap_pars_fragment:Im,clipping_planes_fragment:Lm,clipping_planes_pars_fragment:Dm,clipping_planes_pars_vertex:Nm,clipping_planes_vertex:Um,color_fragment:Fm,color_pars_fragment:Om,color_pars_vertex:Bm,color_vertex:zm,common:km,cube_uv_reflection_fragment:Hm,defaultnormal_vertex:Vm,displacementmap_pars_vertex:Gm,displacementmap_vertex:Wm,emissivemap_fragment:Xm,emissivemap_pars_fragment:qm,colorspace_fragment:Ym,colorspace_pars_fragment:Zm,envmap_fragment:Km,envmap_common_pars_fragment:Jm,envmap_pars_fragment:jm,envmap_pars_vertex:$m,envmap_physical_pars_fragment:cg,envmap_vertex:Qm,fog_vertex:eg,fog_pars_vertex:tg,fog_fragment:ng,fog_pars_fragment:ig,gradientmap_pars_fragment:sg,lightmap_pars_fragment:rg,lights_lambert_fragment:og,lights_lambert_pars_fragment:ag,lights_pars_begin:lg,lights_toon_fragment:hg,lights_toon_pars_fragment:ug,lights_phong_fragment:fg,lights_phong_pars_fragment:dg,lights_physical_fragment:pg,lights_physical_pars_fragment:mg,lights_fragment_begin:gg,lights_fragment_maps:_g,lights_fragment_end:xg,lightprobes_pars_fragment:vg,logdepthbuf_fragment:yg,logdepthbuf_pars_fragment:Mg,logdepthbuf_pars_vertex:bg,logdepthbuf_vertex:Sg,map_fragment:Tg,map_pars_fragment:Eg,map_particle_fragment:wg,map_particle_pars_fragment:Ag,metalnessmap_fragment:Rg,metalnessmap_pars_fragment:Cg,morphinstance_vertex:Pg,morphcolor_vertex:Ig,morphnormal_vertex:Lg,morphtarget_pars_vertex:Dg,morphtarget_vertex:Ng,normal_fragment_begin:Ug,normal_fragment_maps:Fg,normal_pars_fragment:Og,normal_pars_vertex:Bg,normal_vertex:zg,normalmap_pars_fragment:kg,clearcoat_normal_fragment_begin:Hg,clearcoat_normal_fragment_maps:Vg,clearcoat_pars_fragment:Gg,iridescence_pars_fragment:Wg,opaque_fragment:Xg,packing:qg,premultiplied_alpha_fragment:Yg,project_vertex:Zg,dithering_fragment:Kg,dithering_pars_fragment:Jg,roughnessmap_fragment:jg,roughnessmap_pars_fragment:$g,shadowmap_pars_fragment:Qg,shadowmap_pars_vertex:e0,shadowmap_vertex:t0,shadowmask_pars_fragment:n0,skinbase_vertex:i0,skinning_pars_vertex:s0,skinning_vertex:r0,skinnormal_vertex:o0,specularmap_fragment:a0,specularmap_pars_fragment:l0,tonemapping_fragment:c0,tonemapping_pars_fragment:h0,transmission_fragment:u0,transmission_pars_fragment:f0,uv_pars_fragment:d0,uv_pars_vertex:p0,uv_vertex:m0,worldpos_vertex:g0,background_vert:_0,background_frag:x0,backgroundCube_vert:v0,backgroundCube_frag:y0,cube_vert:M0,cube_frag:b0,depth_vert:S0,depth_frag:T0,distance_vert:E0,distance_frag:w0,equirect_vert:A0,equirect_frag:R0,linedashed_vert:C0,linedashed_frag:P0,meshbasic_vert:I0,meshbasic_frag:L0,meshlambert_vert:D0,meshlambert_frag:N0,meshmatcap_vert:U0,meshmatcap_frag:F0,meshnormal_vert:O0,meshnormal_frag:B0,meshphong_vert:z0,meshphong_frag:k0,meshphysical_vert:H0,meshphysical_frag:V0,meshtoon_vert:G0,meshtoon_frag:W0,points_vert:X0,points_frag:q0,shadow_vert:Y0,shadow_frag:Z0,sprite_vert:K0,sprite_frag:J0},Ce={common:{diffuse:{value:new Me(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new $e},alphaMap:{value:null},alphaMapTransform:{value:new $e},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new $e}},envmap:{envMap:{value:null},envMapRotation:{value:new $e},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98},dfgLUT:{value:null}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new $e}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new $e}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new $e},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new $e},normalScale:{value:new ye(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new $e},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new $e}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new $e}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new $e}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new Me(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null},probesSH:{value:null},probesMin:{value:new D},probesMax:{value:new D},probesResolution:{value:new D}},points:{diffuse:{value:new Me(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new $e},alphaTest:{value:0},uvTransform:{value:new $e}},sprite:{diffuse:{value:new Me(16777215)},opacity:{value:1},center:{value:new ye(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new $e},alphaMap:{value:null},alphaMapTransform:{value:new $e},alphaTest:{value:0}}},hi={basic:{uniforms:on([Ce.common,Ce.specularmap,Ce.envmap,Ce.aomap,Ce.lightmap,Ce.fog]),vertexShader:nt.meshbasic_vert,fragmentShader:nt.meshbasic_frag},lambert:{uniforms:on([Ce.common,Ce.specularmap,Ce.envmap,Ce.aomap,Ce.lightmap,Ce.emissivemap,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,Ce.fog,Ce.lights,{emissive:{value:new Me(0)},envMapIntensity:{value:1}}]),vertexShader:nt.meshlambert_vert,fragmentShader:nt.meshlambert_frag},phong:{uniforms:on([Ce.common,Ce.specularmap,Ce.envmap,Ce.aomap,Ce.lightmap,Ce.emissivemap,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,Ce.fog,Ce.lights,{emissive:{value:new Me(0)},specular:{value:new Me(1118481)},shininess:{value:30},envMapIntensity:{value:1}}]),vertexShader:nt.meshphong_vert,fragmentShader:nt.meshphong_frag},standard:{uniforms:on([Ce.common,Ce.envmap,Ce.aomap,Ce.lightmap,Ce.emissivemap,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,Ce.roughnessmap,Ce.metalnessmap,Ce.fog,Ce.lights,{emissive:{value:new Me(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:nt.meshphysical_vert,fragmentShader:nt.meshphysical_frag},toon:{uniforms:on([Ce.common,Ce.aomap,Ce.lightmap,Ce.emissivemap,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,Ce.gradientmap,Ce.fog,Ce.lights,{emissive:{value:new Me(0)}}]),vertexShader:nt.meshtoon_vert,fragmentShader:nt.meshtoon_frag},matcap:{uniforms:on([Ce.common,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,Ce.fog,{matcap:{value:null}}]),vertexShader:nt.meshmatcap_vert,fragmentShader:nt.meshmatcap_frag},points:{uniforms:on([Ce.points,Ce.fog]),vertexShader:nt.points_vert,fragmentShader:nt.points_frag},dashed:{uniforms:on([Ce.common,Ce.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:nt.linedashed_vert,fragmentShader:nt.linedashed_frag},depth:{uniforms:on([Ce.common,Ce.displacementmap]),vertexShader:nt.depth_vert,fragmentShader:nt.depth_frag},normal:{uniforms:on([Ce.common,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,{opacity:{value:1}}]),vertexShader:nt.meshnormal_vert,fragmentShader:nt.meshnormal_frag},sprite:{uniforms:on([Ce.sprite,Ce.fog]),vertexShader:nt.sprite_vert,fragmentShader:nt.sprite_frag},background:{uniforms:{uvTransform:{value:new $e},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:nt.background_vert,fragmentShader:nt.background_frag},backgroundCube:{uniforms:{envMap:{value:null},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new $e}},vertexShader:nt.backgroundCube_vert,fragmentShader:nt.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:nt.cube_vert,fragmentShader:nt.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:nt.equirect_vert,fragmentShader:nt.equirect_frag},distance:{uniforms:on([Ce.common,Ce.displacementmap,{referencePosition:{value:new D},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:nt.distance_vert,fragmentShader:nt.distance_frag},shadow:{uniforms:on([Ce.lights,Ce.fog,{color:{value:new Me(0)},opacity:{value:1}}]),vertexShader:nt.shadow_vert,fragmentShader:nt.shadow_frag}};hi.physical={uniforms:on([hi.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new $e},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new $e},clearcoatNormalScale:{value:new ye(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new $e},dispersion:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new $e},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new $e},sheen:{value:0},sheenColor:{value:new Me(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new $e},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new $e},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new $e},transmissionSamplerSize:{value:new ye},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new $e},attenuationDistance:{value:0},attenuationColor:{value:new Me(0)},specularColor:{value:new Me(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new $e},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new $e},anisotropyVector:{value:new ye},anisotropyMap:{value:null},anisotropyMapTransform:{value:new $e}}]),vertexShader:nt.meshphysical_vert,fragmentShader:nt.meshphysical_frag};var Vl={r:0,b:0,g:0},j0=new Ke,dd=new $e;dd.set(-1,0,0,0,1,0,0,0,1);function $0(s,e,t,n,i,r){let o=new Me(0),a=i===!0?0:1,l,c,h=null,u=0,f=null;function d(S){let R=S.isScene===!0?S.background:null;if(R&&R.isTexture){let b=S.backgroundBlurriness>0;R=e.get(R,b)}return R}function g(S){let R=!1,b=d(S);b===null?m(o,a):b&&b.isColor&&(m(b,1),R=!0);let w=s.xr.getEnvironmentBlendMode();w==="additive"?t.buffers.color.setClear(0,0,0,1,r):w==="alpha-blend"&&t.buffers.color.setClear(0,0,0,0,r),(s.autoClear||R)&&(t.buffers.depth.setTest(!0),t.buffers.depth.setMask(!0),t.buffers.color.setMask(!0),s.clear(s.autoClearColor,s.autoClearDepth,s.autoClearStencil))}function M(S,R){let b=d(R);b&&(b.isCubeTexture||b.mapping===bo)?(c===void 0&&(c=new Je(new si(1,1,1),new mt({name:"BackgroundCubeMaterial",uniforms:Es(hi.backgroundCube.uniforms),vertexShader:hi.backgroundCube.vertexShader,fragmentShader:hi.backgroundCube.fragmentShader,side:Wt,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),c.geometry.deleteAttribute("normal"),c.geometry.deleteAttribute("uv"),c.onBeforeRender=function(w,y,C){this.matrixWorld.copyPosition(C.matrixWorld)},Object.defineProperty(c.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),n.update(c)),c.material.uniforms.envMap.value=b,c.material.uniforms.backgroundBlurriness.value=R.backgroundBlurriness,c.material.uniforms.backgroundIntensity.value=R.backgroundIntensity,c.material.uniforms.backgroundRotation.value.setFromMatrix4(j0.makeRotationFromEuler(R.backgroundRotation)).transpose(),b.isCubeTexture&&b.isRenderTargetTexture===!1&&c.material.uniforms.backgroundRotation.value.premultiply(dd),c.material.toneMapped=et.getTransfer(b.colorSpace)!==pt,(h!==b||u!==b.version||f!==s.toneMapping)&&(c.material.needsUpdate=!0,h=b,u=b.version,f=s.toneMapping),c.layers.enableAll(),S.unshift(c,c.geometry,c.material,0,0,null)):b&&b.isTexture&&(l===void 0&&(l=new Je(new sn(2,2),new mt({name:"BackgroundMaterial",uniforms:Es(hi.background.uniforms),vertexShader:hi.background.vertexShader,fragmentShader:hi.background.fragmentShader,side:Sn,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),l.geometry.deleteAttribute("normal"),Object.defineProperty(l.material,"map",{get:function(){return this.uniforms.t2D.value}}),n.update(l)),l.material.uniforms.t2D.value=b,l.material.uniforms.backgroundIntensity.value=R.backgroundIntensity,l.material.toneMapped=et.getTransfer(b.colorSpace)!==pt,b.matrixAutoUpdate===!0&&b.updateMatrix(),l.material.uniforms.uvTransform.value.copy(b.matrix),(h!==b||u!==b.version||f!==s.toneMapping)&&(l.material.needsUpdate=!0,h=b,u=b.version,f=s.toneMapping),l.layers.enableAll(),S.unshift(l,l.geometry,l.material,0,0,null))}function m(S,R){S.getRGB(Vl,oh(s)),t.buffers.color.setClear(Vl.r,Vl.g,Vl.b,R,r)}function p(){c!==void 0&&(c.geometry.dispose(),c.material.dispose(),c=void 0),l!==void 0&&(l.geometry.dispose(),l.material.dispose(),l=void 0)}return{getClearColor:function(){return o},setClearColor:function(S,R=1){o.set(S),a=R,m(o,a)},getClearAlpha:function(){return a},setClearAlpha:function(S){a=S,m(o,a)},render:g,addToRenderList:M,dispose:p}}function Q0(s,e){let t=s.getParameter(s.MAX_VERTEX_ATTRIBS),n={},i=f(null),r=i,o=!1;function a(P,L,H,O,U){let V=!1,F=u(P,O,H,L);r!==F&&(r=F,c(r.object)),V=d(P,O,H,U),V&&g(P,O,H,U),U!==null&&e.update(U,s.ELEMENT_ARRAY_BUFFER),(V||o)&&(o=!1,b(P,L,H,O),U!==null&&s.bindBuffer(s.ELEMENT_ARRAY_BUFFER,e.get(U).buffer))}function l(){return s.createVertexArray()}function c(P){return s.bindVertexArray(P)}function h(P){return s.deleteVertexArray(P)}function u(P,L,H,O){let U=O.wireframe===!0,V=n[L.id];V===void 0&&(V={},n[L.id]=V);let F=P.isInstancedMesh===!0?P.id:0,Y=V[F];Y===void 0&&(Y={},V[F]=Y);let J=Y[H.id];J===void 0&&(J={},Y[H.id]=J);let j=J[U];return j===void 0&&(j=f(l()),J[U]=j),j}function f(P){let L=[],H=[],O=[];for(let U=0;U<t;U++)L[U]=0,H[U]=0,O[U]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:L,enabledAttributes:H,attributeDivisors:O,object:P,attributes:{},index:null}}function d(P,L,H,O){let U=r.attributes,V=L.attributes,F=0,Y=H.getAttributes();for(let J in Y)if(Y[J].location>=0){let me=U[J],_e=V[J];if(_e===void 0&&(J==="instanceMatrix"&&P.instanceMatrix&&(_e=P.instanceMatrix),J==="instanceColor"&&P.instanceColor&&(_e=P.instanceColor)),me===void 0||me.attribute!==_e||_e&&me.data!==_e.data)return!0;F++}return r.attributesNum!==F||r.index!==O}function g(P,L,H,O){let U={},V=L.attributes,F=0,Y=H.getAttributes();for(let J in Y)if(Y[J].location>=0){let me=V[J];me===void 0&&(J==="instanceMatrix"&&P.instanceMatrix&&(me=P.instanceMatrix),J==="instanceColor"&&P.instanceColor&&(me=P.instanceColor));let _e={};_e.attribute=me,me&&me.data&&(_e.data=me.data),U[J]=_e,F++}r.attributes=U,r.attributesNum=F,r.index=O}function M(){let P=r.newAttributes;for(let L=0,H=P.length;L<H;L++)P[L]=0}function m(P){p(P,0)}function p(P,L){let H=r.newAttributes,O=r.enabledAttributes,U=r.attributeDivisors;H[P]=1,O[P]===0&&(s.enableVertexAttribArray(P),O[P]=1),U[P]!==L&&(s.vertexAttribDivisor(P,L),U[P]=L)}function S(){let P=r.newAttributes,L=r.enabledAttributes;for(let H=0,O=L.length;H<O;H++)L[H]!==P[H]&&(s.disableVertexAttribArray(H),L[H]=0)}function R(P,L,H,O,U,V,F){F===!0?s.vertexAttribIPointer(P,L,H,U,V):s.vertexAttribPointer(P,L,H,O,U,V)}function b(P,L,H,O){M();let U=O.attributes,V=H.getAttributes(),F=L.defaultAttributeValues;for(let Y in V){let J=V[Y];if(J.location>=0){let j=U[Y];if(j===void 0&&(Y==="instanceMatrix"&&P.instanceMatrix&&(j=P.instanceMatrix),Y==="instanceColor"&&P.instanceColor&&(j=P.instanceColor)),j!==void 0){let me=j.normalized,_e=j.itemSize,Ve=e.get(j);if(Ve===void 0)continue;let it=Ve.buffer,Fe=Ve.type,Q=Ve.bytesPerElement,ge=Fe===s.INT||Fe===s.UNSIGNED_INT||j.gpuType===tl;if(j.isInterleavedBufferAttribute){let X=j.data,he=X.stride,$=j.offset;if(X.isInstancedInterleavedBuffer){for(let ie=0;ie<J.locationSize;ie++)p(J.location+ie,X.meshPerAttribute);P.isInstancedMesh!==!0&&O._maxInstanceCount===void 0&&(O._maxInstanceCount=X.meshPerAttribute*X.count)}else for(let ie=0;ie<J.locationSize;ie++)m(J.location+ie);s.bindBuffer(s.ARRAY_BUFFER,it);for(let ie=0;ie<J.locationSize;ie++)R(J.location+ie,_e/J.locationSize,Fe,me,he*Q,($+_e/J.locationSize*ie)*Q,ge)}else{if(j.isInstancedBufferAttribute){for(let X=0;X<J.locationSize;X++)p(J.location+X,j.meshPerAttribute);P.isInstancedMesh!==!0&&O._maxInstanceCount===void 0&&(O._maxInstanceCount=j.meshPerAttribute*j.count)}else for(let X=0;X<J.locationSize;X++)m(J.location+X);s.bindBuffer(s.ARRAY_BUFFER,it);for(let X=0;X<J.locationSize;X++)R(J.location+X,_e/J.locationSize,Fe,me,_e*Q,_e/J.locationSize*X*Q,ge)}}else if(F!==void 0){let me=F[Y];if(me!==void 0)switch(me.length){case 2:s.vertexAttrib2fv(J.location,me);break;case 3:s.vertexAttrib3fv(J.location,me);break;case 4:s.vertexAttrib4fv(J.location,me);break;default:s.vertexAttrib1fv(J.location,me)}}}}S()}function w(){v();for(let P in n){let L=n[P];for(let H in L){let O=L[H];for(let U in O){let V=O[U];for(let F in V)h(V[F].object),delete V[F];delete O[U]}}delete n[P]}}function y(P){if(n[P.id]===void 0)return;let L=n[P.id];for(let H in L){let O=L[H];for(let U in O){let V=O[U];for(let F in V)h(V[F].object),delete V[F];delete O[U]}}delete n[P.id]}function C(P){for(let L in n){let H=n[L];for(let O in H){let U=H[O];if(U[P.id]===void 0)continue;let V=U[P.id];for(let F in V)h(V[F].object),delete V[F];delete U[P.id]}}}function _(P){for(let L in n){let H=n[L],O=P.isInstancedMesh===!0?P.id:0,U=H[O];if(U!==void 0){for(let V in U){let F=U[V];for(let Y in F)h(F[Y].object),delete F[Y];delete U[V]}delete H[O],Object.keys(H).length===0&&delete n[L]}}}function v(){E(),o=!0,r!==i&&(r=i,c(r.object))}function E(){i.geometry=null,i.program=null,i.wireframe=!1}return{setup:a,reset:v,resetDefaultState:E,dispose:w,releaseStatesOfGeometry:y,releaseStatesOfObject:_,releaseStatesOfProgram:C,initAttributes:M,enableAttribute:m,disableUnusedAttributes:S}}function e_(s,e,t){let n;function i(l){n=l}function r(l,c){s.drawArrays(n,l,c),t.update(c,n,1)}function o(l,c,h){h!==0&&(s.drawArraysInstanced(n,l,c,h),t.update(c,n,h))}function a(l,c,h){if(h===0)return;e.get("WEBGL_multi_draw").multiDrawArraysWEBGL(n,l,0,c,0,h);let f=0;for(let d=0;d<h;d++)f+=c[d];t.update(f,n,1)}this.setMode=i,this.render=r,this.renderInstances=o,this.renderMultiDraw=a}function t_(s,e,t,n){let i;function r(){if(i!==void 0)return i;if(e.has("EXT_texture_filter_anisotropic")===!0){let C=e.get("EXT_texture_filter_anisotropic");i=s.getParameter(C.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else i=0;return i}function o(C){return!(C!==wn&&n.convert(C)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_FORMAT))}function a(C){let _=C===Xt&&(e.has("EXT_color_buffer_half_float")||e.has("EXT_color_buffer_float"));return!(C!==gn&&n.convert(C)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_TYPE)&&C!==En&&!_)}function l(C){if(C==="highp"){if(s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.HIGH_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.HIGH_FLOAT).precision>0)return"highp";C="mediump"}return C==="mediump"&&s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.MEDIUM_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let c=t.precision!==void 0?t.precision:"highp",h=l(c);h!==c&&(Be("WebGLRenderer:",c,"not supported, using",h,"instead."),c=h);let u=t.logarithmicDepthBuffer===!0,f=t.reversedDepthBuffer===!0&&e.has("EXT_clip_control");t.reversedDepthBuffer===!0&&f===!1&&Be("WebGLRenderer: Unable to use reversed depth buffer due to missing EXT_clip_control extension. Fallback to default depth buffer.");let d=s.getParameter(s.MAX_TEXTURE_IMAGE_UNITS),g=s.getParameter(s.MAX_VERTEX_TEXTURE_IMAGE_UNITS),M=s.getParameter(s.MAX_TEXTURE_SIZE),m=s.getParameter(s.MAX_CUBE_MAP_TEXTURE_SIZE),p=s.getParameter(s.MAX_VERTEX_ATTRIBS),S=s.getParameter(s.MAX_VERTEX_UNIFORM_VECTORS),R=s.getParameter(s.MAX_VARYING_VECTORS),b=s.getParameter(s.MAX_FRAGMENT_UNIFORM_VECTORS),w=s.getParameter(s.MAX_SAMPLES),y=s.getParameter(s.SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:l,textureFormatReadable:o,textureTypeReadable:a,precision:c,logarithmicDepthBuffer:u,reversedDepthBuffer:f,maxTextures:d,maxVertexTextures:g,maxTextureSize:M,maxCubemapSize:m,maxAttributes:p,maxVertexUniforms:S,maxVaryings:R,maxFragmentUniforms:b,maxSamples:w,samples:y}}function n_(s){let e=this,t=null,n=0,i=!1,r=!1,o=new Ln,a=new $e,l={value:null,needsUpdate:!1};this.uniform=l,this.numPlanes=0,this.numIntersection=0,this.init=function(u,f){let d=u.length!==0||f||n!==0||i;return i=f,n=u.length,d},this.beginShadows=function(){r=!0,h(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(u,f){t=h(u,f,0)},this.setState=function(u,f,d){let g=u.clippingPlanes,M=u.clipIntersection,m=u.clipShadows,p=s.get(u);if(!i||g===null||g.length===0||r&&!m)r?h(null):c();else{let S=r?0:n,R=S*4,b=p.clippingState||null;l.value=b,b=h(g,f,R,d);for(let w=0;w!==R;++w)b[w]=t[w];p.clippingState=b,this.numIntersection=M?this.numPlanes:0,this.numPlanes+=S}};function c(){l.value!==t&&(l.value=t,l.needsUpdate=n>0),e.numPlanes=n,e.numIntersection=0}function h(u,f,d,g){let M=u!==null?u.length:0,m=null;if(M!==0){if(m=l.value,g!==!0||m===null){let p=d+M*4,S=f.matrixWorldInverse;a.getNormalMatrix(S),(m===null||m.length<p)&&(m=new Float32Array(p));for(let R=0,b=d;R!==M;++R,b+=4)o.copy(u[R]).applyMatrix4(S,a),o.normal.toArray(m,b),m[b+3]=o.constant}l.value=m,l.needsUpdate=!0}return e.numPlanes=M,e.numIntersection=0,m}}var Qi=4,Wf=[.125,.215,.35,.446,.526,.582],ws=20,i_=256,Io=new li,Xf=new Me,hh=null,uh=0,fh=0,dh=!1,s_=new D,vr=class{constructor(e){this._renderer=e,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._sizeLods=[],this._sigmas=[],this._lodMeshes=[],this._backgroundBox=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._blurMaterial=null,this._ggxMaterial=null}fromScene(e,t=0,n=.1,i=100,r={}){let{size:o=256,position:a=s_}=r;hh=this._renderer.getRenderTarget(),uh=this._renderer.getActiveCubeFace(),fh=this._renderer.getActiveMipmapLevel(),dh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(o);let l=this._allocateTargets();return l.depthBuffer=!0,this._sceneToCubeUV(e,n,i,l,a),t>0&&this._blur(l,0,0,t),this._applyPMREM(l),this._cleanup(l),l}fromEquirectangular(e,t=null){return this._fromTexture(e,t)}fromCubemap(e,t=null){return this._fromTexture(e,t)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=Zf(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=Yf(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose(),this._backgroundBox!==null&&(this._backgroundBox.geometry.dispose(),this._backgroundBox.material.dispose())}_setSize(e){this._lodMax=Math.floor(Math.log2(e)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._ggxMaterial!==null&&this._ggxMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let e=0;e<this._lodMeshes.length;e++)this._lodMeshes[e].geometry.dispose()}_cleanup(e){this._renderer.setRenderTarget(hh,uh,fh),this._renderer.xr.enabled=dh,e.scissorTest=!1,_r(e,0,0,e.width,e.height)}_fromTexture(e,t){e.mapping===Ji||e.mapping===Ss?this._setSize(e.image.length===0?16:e.image[0].width||e.image[0].image.width):this._setSize(e.image.width/4),hh=this._renderer.getRenderTarget(),uh=this._renderer.getActiveCubeFace(),fh=this._renderer.getActiveMipmapLevel(),dh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let n=t||this._allocateTargets();return this._textureToCubeUV(e,n),this._applyPMREM(n),this._cleanup(n),n}_allocateTargets(){let e=3*Math.max(this._cubeSize,112),t=4*this._cubeSize,n={magFilter:Lt,minFilter:Lt,generateMipmaps:!1,type:Xt,format:wn,colorSpace:cn,depthBuffer:!1},i=qf(e,t,n);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==e||this._pingPongRenderTarget.height!==t){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=qf(e,t,n);let{_lodMax:r}=this;({lodMeshes:this._lodMeshes,sizeLods:this._sizeLods,sigmas:this._sigmas}=r_(r)),this._blurMaterial=a_(r,e,t),this._ggxMaterial=o_(r,e,t)}return i}_compileMaterial(e){let t=new Je(new ft,e);this._renderer.compile(t,Io)}_sceneToCubeUV(e,t,n,i,r){let l=new Ht(90,1,t,n),c=[1,-1,1,1,1,1],h=[1,1,1,-1,-1,-1],u=this._renderer,f=u.autoClear,d=u.toneMapping;u.getClearColor(Xf),u.toneMapping=qn,u.autoClear=!1,u.state.buffers.depth.getReversed()&&(u.setRenderTarget(i),u.clearDepth(),u.setRenderTarget(null)),this._backgroundBox===null&&(this._backgroundBox=new Je(new si,new Tt({name:"PMREM.Background",side:Wt,depthWrite:!1,depthTest:!1})));let M=this._backgroundBox,m=M.material,p=!1,S=e.background;S?S.isColor&&(m.color.copy(S),e.background=null,p=!0):(m.color.copy(Xf),p=!0);for(let R=0;R<6;R++){let b=R%3;b===0?(l.up.set(0,c[R],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x+h[R],r.y,r.z)):b===1?(l.up.set(0,0,c[R]),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y+h[R],r.z)):(l.up.set(0,c[R],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y,r.z+h[R]));let w=this._cubeSize;_r(i,b*w,R>2?w:0,w,w),u.setRenderTarget(i),p&&u.render(M,l),u.render(e,l)}u.toneMapping=d,u.autoClear=f,e.background=S}_textureToCubeUV(e,t){let n=this._renderer,i=e.mapping===Ji||e.mapping===Ss;i?(this._cubemapMaterial===null&&(this._cubemapMaterial=Zf()),this._cubemapMaterial.uniforms.flipEnvMap.value=e.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=Yf());let r=i?this._cubemapMaterial:this._equirectMaterial,o=this._lodMeshes[0];o.material=r;let a=r.uniforms;a.envMap.value=e;let l=this._cubeSize;_r(t,0,0,3*l,2*l),n.setRenderTarget(t),n.render(o,Io)}_applyPMREM(e){let t=this._renderer,n=t.autoClear;t.autoClear=!1;let i=this._lodMeshes.length;for(let r=1;r<i;r++)this._applyGGXFilter(e,r-1,r);t.autoClear=n}_applyGGXFilter(e,t,n){let i=this._renderer,r=this._pingPongRenderTarget,o=this._ggxMaterial,a=this._lodMeshes[n];a.material=o;let l=o.uniforms,c=n/(this._lodMeshes.length-1),h=t/(this._lodMeshes.length-1),u=Math.sqrt(c*c-h*h),f=0+c*1.25,d=u*f,{_lodMax:g}=this,M=this._sizeLods[n],m=3*M*(n>g-Qi?n-g+Qi:0),p=4*(this._cubeSize-M);l.envMap.value=e.texture,l.roughness.value=d,l.mipInt.value=g-t,_r(r,m,p,3*M,2*M),i.setRenderTarget(r),i.render(a,Io),l.envMap.value=r.texture,l.roughness.value=0,l.mipInt.value=g-n,_r(e,m,p,3*M,2*M),i.setRenderTarget(e),i.render(a,Io)}_blur(e,t,n,i,r){let o=this._pingPongRenderTarget;this._halfBlur(e,o,t,n,i,"latitudinal",r),this._halfBlur(o,e,n,n,i,"longitudinal",r)}_halfBlur(e,t,n,i,r,o,a){let l=this._renderer,c=this._blurMaterial;o!=="latitudinal"&&o!=="longitudinal"&&Ze("blur direction must be either latitudinal or longitudinal!");let h=3,u=this._lodMeshes[i];u.material=c;let f=c.uniforms,d=this._sizeLods[n]-1,g=isFinite(r)?Math.PI/(2*d):2*Math.PI/(2*ws-1),M=r/g,m=isFinite(r)?1+Math.floor(h*M):ws;m>ws&&Be(`sigmaRadians, ${r}, is too large and will clip, as it requested ${m} samples when the maximum is set to ${ws}`);let p=[],S=0;for(let C=0;C<ws;++C){let _=C/M,v=Math.exp(-_*_/2);p.push(v),C===0?S+=v:C<m&&(S+=2*v)}for(let C=0;C<p.length;C++)p[C]=p[C]/S;f.envMap.value=e.texture,f.samples.value=m,f.weights.value=p,f.latitudinal.value=o==="latitudinal",a&&(f.poleAxis.value=a);let{_lodMax:R}=this;f.dTheta.value=g,f.mipInt.value=R-n;let b=this._sizeLods[i],w=3*b*(i>R-Qi?i-R+Qi:0),y=4*(this._cubeSize-b);_r(t,w,y,3*b,2*b),l.setRenderTarget(t),l.render(u,Io)}};function r_(s){let e=[],t=[],n=[],i=s,r=s-Qi+1+Wf.length;for(let o=0;o<r;o++){let a=Math.pow(2,i);e.push(a);let l=1/a;o>s-Qi?l=Wf[o-s+Qi-1]:o===0&&(l=0),t.push(l);let c=1/(a-2),h=-c,u=1+c,f=[h,h,u,h,u,u,h,h,u,u,h,u],d=6,g=6,M=3,m=2,p=1,S=new Float32Array(M*g*d),R=new Float32Array(m*g*d),b=new Float32Array(p*g*d);for(let y=0;y<d;y++){let C=y%3*2/3-1,_=y>2?0:-1,v=[C,_,0,C+2/3,_,0,C+2/3,_+1,0,C,_,0,C+2/3,_+1,0,C,_+1,0];S.set(v,M*g*y),R.set(f,m*g*y);let E=[y,y,y,y,y,y];b.set(E,p*g*y)}let w=new ft;w.setAttribute("position",new vt(S,M)),w.setAttribute("uv",new vt(R,m)),w.setAttribute("faceIndex",new vt(b,p)),n.push(new Je(w,null)),i>Qi&&i--}return{lodMeshes:n,sizeLods:e,sigmas:t}}function qf(s,e,t){let n=new Nt(s,e,t);return n.texture.mapping=bo,n.texture.name="PMREM.cubeUv",n.scissorTest=!0,n}function _r(s,e,t,n,i){s.viewport.set(e,t,n,i),s.scissor.set(e,t,n,i)}function o_(s,e,t){return new mt({name:"PMREMGGXConvolution",defines:{GGX_SAMPLES:i_,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},roughness:{value:0},mipInt:{value:0}},vertexShader:ql(),fragmentShader:`

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
		`,blending:On,depthTest:!1,depthWrite:!1})}function a_(s,e,t){let n=new Float32Array(ws),i=new D(0,1,0);return new mt({name:"SphericalGaussianBlur",defines:{n:ws,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},samples:{value:1},weights:{value:n},latitudinal:{value:!1},dTheta:{value:0},mipInt:{value:0},poleAxis:{value:i}},vertexShader:ql(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			varying vec3 vOutputDirection;

			uniform sampler2D envMap;
			uniform int samples;
			uniform float weights[ n ];
			uniform bool latitudinal;
			uniform float dTheta;
			uniform float mipInt;
			uniform vec3 poleAxis;

			#define ENVMAP_TYPE_CUBE_UV
			#include <cube_uv_reflection_fragment>

			vec3 getSample( float theta, vec3 axis ) {

				float cosTheta = cos( theta );
				// Rodrigues' axis-angle rotation
				vec3 sampleDirection = vOutputDirection * cosTheta
					+ cross( axis, vOutputDirection ) * sin( theta )
					+ axis * dot( axis, vOutputDirection ) * ( 1.0 - cosTheta );

				return bilinearCubeUV( envMap, sampleDirection, mipInt );

			}

			void main() {

				vec3 axis = latitudinal ? poleAxis : cross( poleAxis, vOutputDirection );

				if ( all( equal( axis, vec3( 0.0 ) ) ) ) {

					axis = vec3( vOutputDirection.z, 0.0, - vOutputDirection.x );

				}

				axis = normalize( axis );

				gl_FragColor = vec4( 0.0, 0.0, 0.0, 1.0 );
				gl_FragColor.rgb += weights[ 0 ] * getSample( 0.0, axis );

				for ( int i = 1; i < n; i++ ) {

					if ( i >= samples ) {

						break;

					}

					float theta = dTheta * float( i );
					gl_FragColor.rgb += weights[ i ] * getSample( -1.0 * theta, axis );
					gl_FragColor.rgb += weights[ i ] * getSample( theta, axis );

				}

			}
		`,blending:On,depthTest:!1,depthWrite:!1})}function Yf(){return new mt({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:ql(),fragmentShader:`

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
		`,blending:On,depthTest:!1,depthWrite:!1})}function Zf(){return new mt({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:ql(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:On,depthTest:!1,depthWrite:!1})}function ql(){return`

		precision mediump float;
		precision mediump int;

		attribute float faceIndex;

		varying vec3 vOutputDirection;

		// RH coordinate system; PMREM face-indexing convention
		vec3 getDirection( vec2 uv, float face ) {

			uv = 2.0 * uv - 1.0;

			vec3 direction = vec3( uv, 1.0 );

			if ( face == 0.0 ) {

				direction = direction.zyx; // ( 1, v, u ) pos x

			} else if ( face == 1.0 ) {

				direction = direction.xzy;
				direction.xz *= -1.0; // ( -u, 1, -v ) pos y

			} else if ( face == 2.0 ) {

				direction.x *= -1.0; // ( -u, v, 1 ) pos z

			} else if ( face == 3.0 ) {

				direction = direction.zyx;
				direction.xz *= -1.0; // ( -1, v, -u ) neg x

			} else if ( face == 4.0 ) {

				direction = direction.xzy;
				direction.xy *= -1.0; // ( -u, -1, v ) neg y

			} else if ( face == 5.0 ) {

				direction.z *= -1.0; // ( u, v, -1 ) neg z

			}

			return direction;

		}

		void main() {

			vOutputDirection = getDirection( uv, faceIndex );
			gl_Position = vec4( position, 1.0 );

		}
	`}var Wl=class extends Nt{constructor(e=1,t={}){super(e,e,t),this.isWebGLCubeRenderTarget=!0;let n={width:e,height:e,depth:1},i=[n,n,n,n,n,n];this.texture=new Kr(i),this._setTextureOptions(t),this.texture.isRenderTargetTexture=!0}fromEquirectangularTexture(e,t){this.texture.type=t.type,this.texture.colorSpace=t.colorSpace,this.texture.generateMipmaps=t.generateMipmaps,this.texture.minFilter=t.minFilter,this.texture.magFilter=t.magFilter;let n={uniforms:{tEquirect:{value:null}},vertexShader:`

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
			`},i=new si(5,5,5),r=new mt({name:"CubemapFromEquirect",uniforms:Es(n.uniforms),vertexShader:n.vertexShader,fragmentShader:n.fragmentShader,side:Wt,blending:On});r.uniforms.tEquirect.value=t;let o=new Je(i,r),a=t.minFilter;return t.minFilter===Yn&&(t.minFilter=Lt),new qa(1,10,this).update(e,o),t.minFilter=a,o.geometry.dispose(),o.material.dispose(),this}clear(e,t=!0,n=!0,i=!0){let r=e.getRenderTarget();for(let o=0;o<6;o++)e.setRenderTarget(this,o),e.clear(t,n,i);e.setRenderTarget(r)}};function l_(s){let e=new WeakMap,t=new WeakMap,n=null;function i(f,d=!1){return f==null?null:d?o(f):r(f)}function r(f){if(f&&f.isTexture){let d=f.mapping;if(d===$a||d===Qa)if(e.has(f)){let g=e.get(f).texture;return a(g,f.mapping)}else{let g=f.image;if(g&&g.height>0){let M=new Wl(g.height);return M.fromEquirectangularTexture(s,f),e.set(f,M),f.addEventListener("dispose",c),a(M.texture,f.mapping)}else return null}}return f}function o(f){if(f&&f.isTexture){let d=f.mapping,g=d===$a||d===Qa,M=d===Ji||d===Ss;if(g||M){let m=t.get(f),p=m!==void 0?m.texture.pmremVersion:0;if(f.isRenderTargetTexture&&f.pmremVersion!==p)return n===null&&(n=new vr(s)),m=g?n.fromEquirectangular(f,m):n.fromCubemap(f,m),m.texture.pmremVersion=f.pmremVersion,t.set(f,m),m.texture;if(m!==void 0)return m.texture;{let S=f.image;return g&&S&&S.height>0||M&&S&&l(S)?(n===null&&(n=new vr(s)),m=g?n.fromEquirectangular(f):n.fromCubemap(f),m.texture.pmremVersion=f.pmremVersion,t.set(f,m),f.addEventListener("dispose",h),m.texture):null}}}return f}function a(f,d){return d===$a?f.mapping=Ji:d===Qa&&(f.mapping=Ss),f}function l(f){let d=0,g=6;for(let M=0;M<g;M++)f[M]!==void 0&&d++;return d===g}function c(f){let d=f.target;d.removeEventListener("dispose",c);let g=e.get(d);g!==void 0&&(e.delete(d),g.dispose())}function h(f){let d=f.target;d.removeEventListener("dispose",h);let g=t.get(d);g!==void 0&&(t.delete(d),g.dispose())}function u(){e=new WeakMap,t=new WeakMap,n!==null&&(n.dispose(),n=null)}return{get:i,dispose:u}}function c_(s){let e={};function t(n){if(e[n]!==void 0)return e[n];let i=s.getExtension(n);return e[n]=i,i}return{has:function(n){return t(n)!==null},init:function(){t("EXT_color_buffer_float"),t("WEBGL_clip_cull_distance"),t("OES_texture_float_linear"),t("EXT_color_buffer_half_float"),t("WEBGL_multisampled_render_to_texture"),t("WEBGL_render_shared_exponent")},get:function(n){let i=t(n);return i===null&&hs("WebGLRenderer: "+n+" extension not supported."),i}}}function h_(s,e,t,n){let i={},r=new WeakMap;function o(u){let f=u.target;f.index!==null&&e.remove(f.index);for(let g in f.attributes)e.remove(f.attributes[g]);f.removeEventListener("dispose",o),delete i[f.id];let d=r.get(f);d&&(e.remove(d),r.delete(f)),n.releaseStatesOfGeometry(f),f.isInstancedBufferGeometry===!0&&delete f._maxInstanceCount,t.memory.geometries--}function a(u,f){return i[f.id]===!0||(f.addEventListener("dispose",o),i[f.id]=!0,t.memory.geometries++),f}function l(u){let f=u.attributes;for(let d in f)e.update(f[d],s.ARRAY_BUFFER)}function c(u){let f=[],d=u.index,g=u.attributes.position,M=0;if(g===void 0)return;if(d!==null){let S=d.array;M=d.version;for(let R=0,b=S.length;R<b;R+=3){let w=S[R+0],y=S[R+1],C=S[R+2];f.push(w,y,y,C,C,w)}}else{let S=g.array;M=g.version;for(let R=0,b=S.length/3-1;R<b;R+=3){let w=R+0,y=R+1,C=R+2;f.push(w,y,y,C,C,w)}}let m=new(g.count>=65535?Xr:Wr)(f,1);m.version=M;let p=r.get(u);p&&e.remove(p),r.set(u,m)}function h(u){let f=r.get(u);if(f){let d=u.index;d!==null&&f.version<d.version&&c(u)}else c(u);return r.get(u)}return{get:a,update:l,getWireframeAttribute:h}}function u_(s,e,t){let n;function i(u){n=u}let r,o;function a(u){r=u.type,o=u.bytesPerElement}function l(u,f){s.drawElements(n,f,r,u*o),t.update(f,n,1)}function c(u,f,d){d!==0&&(s.drawElementsInstanced(n,f,r,u*o,d),t.update(f,n,d))}function h(u,f,d){if(d===0)return;e.get("WEBGL_multi_draw").multiDrawElementsWEBGL(n,f,0,r,u,0,d);let M=0;for(let m=0;m<d;m++)M+=f[m];t.update(M,n,1)}this.setMode=i,this.setIndex=a,this.render=l,this.renderInstances=c,this.renderMultiDraw=h}function f_(s){let e={geometries:0,textures:0},t={frame:0,calls:0,triangles:0,points:0,lines:0};function n(r,o,a){switch(t.calls++,o){case s.TRIANGLES:t.triangles+=a*(r/3);break;case s.LINES:t.lines+=a*(r/2);break;case s.LINE_STRIP:t.lines+=a*(r-1);break;case s.LINE_LOOP:t.lines+=a*r;break;case s.POINTS:t.points+=a*r;break;default:Ze("WebGLInfo: Unknown draw mode:",o);break}}function i(){t.calls=0,t.triangles=0,t.points=0,t.lines=0}return{memory:e,render:t,programs:null,autoReset:!0,reset:i,update:n}}function d_(s,e,t){let n=new WeakMap,i=new xt;function r(o,a,l){let c=o.morphTargetInfluences,h=a.morphAttributes.position||a.morphAttributes.normal||a.morphAttributes.color,u=h!==void 0?h.length:0,f=n.get(a);if(f===void 0||f.count!==u){let v=function(){C.dispose(),n.delete(a),a.removeEventListener("dispose",v)};f!==void 0&&f.texture.dispose();let d=a.morphAttributes.position!==void 0,g=a.morphAttributes.normal!==void 0,M=a.morphAttributes.color!==void 0,m=a.morphAttributes.position||[],p=a.morphAttributes.normal||[],S=a.morphAttributes.color||[],R=0;d===!0&&(R=1),g===!0&&(R=2),M===!0&&(R=3);let b=a.attributes.position.count*R,w=1;b>e.maxTextureSize&&(w=Math.ceil(b/e.maxTextureSize),b=e.maxTextureSize);let y=new Float32Array(b*w*4*u),C=new Vr(y,b,w,u);C.type=En,C.needsUpdate=!0;let _=R*4;for(let E=0;E<u;E++){let P=m[E],L=p[E],H=S[E],O=b*w*4*E;for(let U=0;U<P.count;U++){let V=U*_;d===!0&&(i.fromBufferAttribute(P,U),y[O+V+0]=i.x,y[O+V+1]=i.y,y[O+V+2]=i.z,y[O+V+3]=0),g===!0&&(i.fromBufferAttribute(L,U),y[O+V+4]=i.x,y[O+V+5]=i.y,y[O+V+6]=i.z,y[O+V+7]=0),M===!0&&(i.fromBufferAttribute(H,U),y[O+V+8]=i.x,y[O+V+9]=i.y,y[O+V+10]=i.z,y[O+V+11]=H.itemSize===4?i.w:1)}}f={count:u,texture:C,size:new ye(b,w)},n.set(a,f),a.addEventListener("dispose",v)}if(o.isInstancedMesh===!0&&o.morphTexture!==null)l.getUniforms().setValue(s,"morphTexture",o.morphTexture,t);else{let d=0;for(let M=0;M<c.length;M++)d+=c[M];let g=a.morphTargetsRelative?1:1-d;l.getUniforms().setValue(s,"morphTargetBaseInfluence",g),l.getUniforms().setValue(s,"morphTargetInfluences",c)}l.getUniforms().setValue(s,"morphTargetsTexture",f.texture,t),l.getUniforms().setValue(s,"morphTargetsTextureSize",f.size)}return{update:r}}function p_(s,e,t,n,i){let r=new WeakMap;function o(c){let h=i.render.frame,u=c.geometry,f=e.get(c,u);if(r.get(f)!==h&&(e.update(f),r.set(f,h)),c.isInstancedMesh&&(c.hasEventListener("dispose",l)===!1&&c.addEventListener("dispose",l),r.get(c)!==h&&(t.update(c.instanceMatrix,s.ARRAY_BUFFER),c.instanceColor!==null&&t.update(c.instanceColor,s.ARRAY_BUFFER),r.set(c,h))),c.isSkinnedMesh){let d=c.skeleton;r.get(d)!==h&&(d.update(),r.set(d,h))}return f}function a(){r=new WeakMap}function l(c){let h=c.target;h.removeEventListener("dispose",l),n.releaseStatesOfObject(h),t.remove(h.instanceMatrix),h.instanceColor!==null&&t.remove(h.instanceColor)}return{update:o,dispose:a}}var m_={[go]:"LINEAR_TONE_MAPPING",[_o]:"REINHARD_TONE_MAPPING",[xo]:"CINEON_TONE_MAPPING",[bs]:"ACES_FILMIC_TONE_MAPPING",[yo]:"AGX_TONE_MAPPING",[Mo]:"NEUTRAL_TONE_MAPPING",[vo]:"CUSTOM_TONE_MAPPING"};function g_(s,e,t,n,i,r){let o=new Nt(e,t,{type:s,depthBuffer:i,stencilBuffer:r,samples:n?4:0,depthTexture:i?new bi(e,t):void 0}),a=new Nt(e,t,{type:Xt,depthBuffer:!1,stencilBuffer:!1}),l=new ft;l.setAttribute("position",new ot([-1,3,0,-1,-1,0,3,-1,0],3)),l.setAttribute("uv",new ot([0,2,0,0,2,0],2));let c=new cr({uniforms:{tDiffuse:{value:null}},vertexShader:`
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
			}`,depthTest:!1,depthWrite:!1}),h=new Je(l,c),u=new li(-1,1,1,-1,0,1),f=null,d=null,g=!1,M,m=null,p=[],S=!1;this.setSize=function(R,b){o.setSize(R,b),a.setSize(R,b);for(let w=0;w<p.length;w++){let y=p[w];y.setSize&&y.setSize(R,b)}},this.setEffects=function(R){p=R,S=p.length>0&&p[0].isRenderPass===!0;let b=o.width,w=o.height;for(let y=0;y<p.length;y++){let C=p[y];C.setSize&&C.setSize(b,w)}},this.begin=function(R,b){if(g||R.toneMapping===qn&&p.length===0)return!1;if(m=b,b!==null){let w=b.width,y=b.height;(o.width!==w||o.height!==y)&&this.setSize(w,y)}return S===!1&&R.setRenderTarget(o),M=R.toneMapping,R.toneMapping=qn,!0},this.hasRenderPass=function(){return S},this.end=function(R,b){R.toneMapping=M,g=!0;let w=o,y=a;for(let C=0;C<p.length;C++){let _=p[C];if(_.enabled!==!1&&(_.render(R,y,w,b),_.needsSwap!==!1)){let v=w;w=y,y=v}}if(f!==R.outputColorSpace||d!==R.toneMapping){f=R.outputColorSpace,d=R.toneMapping,c.defines={},et.getTransfer(f)===pt&&(c.defines.SRGB_TRANSFER="");let C=m_[d];C&&(c.defines[C]=""),c.needsUpdate=!0}c.uniforms.tDiffuse.value=w.texture,R.setRenderTarget(m),R.render(h,u),m=null,g=!1},this.isCompositing=function(){return g},this.dispose=function(){o.depthTexture&&o.depthTexture.dispose(),o.dispose(),a.dispose(),l.dispose(),c.dispose()}}var pd=new Gt,gh=new bi(1,1),md=new Vr,gd=new Aa,_d=new Kr,Kf=[],Jf=[],jf=new Float32Array(16),$f=new Float32Array(9),Qf=new Float32Array(4);function yr(s,e,t){let n=s[0];if(n<=0||n>0)return s;let i=e*t,r=Kf[i];if(r===void 0&&(r=new Float32Array(i),Kf[i]=r),e!==0){n.toArray(r,0);for(let o=1,a=0;o!==e;++o)a+=t,s[o].toArray(r,a)}return r}function qt(s,e){if(s.length!==e.length)return!1;for(let t=0,n=s.length;t<n;t++)if(s[t]!==e[t])return!1;return!0}function Yt(s,e){for(let t=0,n=e.length;t<n;t++)s[t]=e[t]}function Yl(s,e){let t=Jf[e];t===void 0&&(t=new Int32Array(e),Jf[e]=t);for(let n=0;n!==e;++n)t[n]=s.allocateTextureUnit();return t}function __(s,e){let t=this.cache;t[0]!==e&&(s.uniform1f(this.addr,e),t[0]=e)}function x_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2f(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(qt(t,e))return;s.uniform2fv(this.addr,e),Yt(t,e)}}function v_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3f(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else if(e.r!==void 0)(t[0]!==e.r||t[1]!==e.g||t[2]!==e.b)&&(s.uniform3f(this.addr,e.r,e.g,e.b),t[0]=e.r,t[1]=e.g,t[2]=e.b);else{if(qt(t,e))return;s.uniform3fv(this.addr,e),Yt(t,e)}}function y_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4f(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(qt(t,e))return;s.uniform4fv(this.addr,e),Yt(t,e)}}function M_(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(qt(t,e))return;s.uniformMatrix2fv(this.addr,!1,e),Yt(t,e)}else{if(qt(t,n))return;Qf.set(n),s.uniformMatrix2fv(this.addr,!1,Qf),Yt(t,n)}}function b_(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(qt(t,e))return;s.uniformMatrix3fv(this.addr,!1,e),Yt(t,e)}else{if(qt(t,n))return;$f.set(n),s.uniformMatrix3fv(this.addr,!1,$f),Yt(t,n)}}function S_(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(qt(t,e))return;s.uniformMatrix4fv(this.addr,!1,e),Yt(t,e)}else{if(qt(t,n))return;jf.set(n),s.uniformMatrix4fv(this.addr,!1,jf),Yt(t,n)}}function T_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1i(this.addr,e),t[0]=e)}function E_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2i(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(qt(t,e))return;s.uniform2iv(this.addr,e),Yt(t,e)}}function w_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3i(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(qt(t,e))return;s.uniform3iv(this.addr,e),Yt(t,e)}}function A_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4i(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(qt(t,e))return;s.uniform4iv(this.addr,e),Yt(t,e)}}function R_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1ui(this.addr,e),t[0]=e)}function C_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2ui(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(qt(t,e))return;s.uniform2uiv(this.addr,e),Yt(t,e)}}function P_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3ui(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(qt(t,e))return;s.uniform3uiv(this.addr,e),Yt(t,e)}}function I_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4ui(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(qt(t,e))return;s.uniform4uiv(this.addr,e),Yt(t,e)}}function L_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i);let r;this.type===s.SAMPLER_2D_SHADOW?(gh.compareFunction=t.isReversedDepthBuffer()?Hl:kl,r=gh):r=pd,t.setTexture2D(e||r,i)}function D_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture3D(e||gd,i)}function N_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTextureCube(e||_d,i)}function U_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture2DArray(e||md,i)}function F_(s){switch(s){case 5126:return __;case 35664:return x_;case 35665:return v_;case 35666:return y_;case 35674:return M_;case 35675:return b_;case 35676:return S_;case 5124:case 35670:return T_;case 35667:case 35671:return E_;case 35668:case 35672:return w_;case 35669:case 35673:return A_;case 5125:return R_;case 36294:return C_;case 36295:return P_;case 36296:return I_;case 35678:case 36198:case 36298:case 36306:case 35682:return L_;case 35679:case 36299:case 36307:return D_;case 35680:case 36300:case 36308:case 36293:return N_;case 36289:case 36303:case 36311:case 36292:return U_}}function O_(s,e){s.uniform1fv(this.addr,e)}function B_(s,e){let t=yr(e,this.size,2);s.uniform2fv(this.addr,t)}function z_(s,e){let t=yr(e,this.size,3);s.uniform3fv(this.addr,t)}function k_(s,e){let t=yr(e,this.size,4);s.uniform4fv(this.addr,t)}function H_(s,e){let t=yr(e,this.size,4);s.uniformMatrix2fv(this.addr,!1,t)}function V_(s,e){let t=yr(e,this.size,9);s.uniformMatrix3fv(this.addr,!1,t)}function G_(s,e){let t=yr(e,this.size,16);s.uniformMatrix4fv(this.addr,!1,t)}function W_(s,e){s.uniform1iv(this.addr,e)}function X_(s,e){s.uniform2iv(this.addr,e)}function q_(s,e){s.uniform3iv(this.addr,e)}function Y_(s,e){s.uniform4iv(this.addr,e)}function Z_(s,e){s.uniform1uiv(this.addr,e)}function K_(s,e){s.uniform2uiv(this.addr,e)}function J_(s,e){s.uniform3uiv(this.addr,e)}function j_(s,e){s.uniform4uiv(this.addr,e)}function $_(s,e,t){let n=this.cache,i=e.length,r=Yl(t,i);qt(n,r)||(s.uniform1iv(this.addr,r),Yt(n,r));let o;this.type===s.SAMPLER_2D_SHADOW?o=gh:o=pd;for(let a=0;a!==i;++a)t.setTexture2D(e[a]||o,r[a])}function Q_(s,e,t){let n=this.cache,i=e.length,r=Yl(t,i);qt(n,r)||(s.uniform1iv(this.addr,r),Yt(n,r));for(let o=0;o!==i;++o)t.setTexture3D(e[o]||gd,r[o])}function ex(s,e,t){let n=this.cache,i=e.length,r=Yl(t,i);qt(n,r)||(s.uniform1iv(this.addr,r),Yt(n,r));for(let o=0;o!==i;++o)t.setTextureCube(e[o]||_d,r[o])}function tx(s,e,t){let n=this.cache,i=e.length,r=Yl(t,i);qt(n,r)||(s.uniform1iv(this.addr,r),Yt(n,r));for(let o=0;o!==i;++o)t.setTexture2DArray(e[o]||md,r[o])}function nx(s){switch(s){case 5126:return O_;case 35664:return B_;case 35665:return z_;case 35666:return k_;case 35674:return H_;case 35675:return V_;case 35676:return G_;case 5124:case 35670:return W_;case 35667:case 35671:return X_;case 35668:case 35672:return q_;case 35669:case 35673:return Y_;case 5125:return Z_;case 36294:return K_;case 36295:return J_;case 36296:return j_;case 35678:case 36198:case 36298:case 36306:case 35682:return $_;case 35679:case 36299:case 36307:return Q_;case 35680:case 36300:case 36308:case 36293:return ex;case 36289:case 36303:case 36311:case 36292:return tx}}var _h=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.setValue=F_(t.type)}},xh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.size=t.size,this.setValue=nx(t.type)}},vh=class{constructor(e){this.id=e,this.seq=[],this.map={}}setValue(e,t,n){let i=this.seq;for(let r=0,o=i.length;r!==o;++r){let a=i[r];a.setValue(e,t[a.id],n)}}},ph=/(\w+)(\])?(\[|\.)?/g;function ed(s,e){s.seq.push(e),s.map[e.id]=e}function ix(s,e,t){let n=s.name,i=n.length;for(ph.lastIndex=0;;){let r=ph.exec(n),o=ph.lastIndex,a=r[1],l=r[2]==="]",c=r[3];if(l&&(a=a|0),c===void 0||c==="["&&o+2===i){ed(t,c===void 0?new _h(a,s,e):new xh(a,s,e));break}else{let u=t.map[a];u===void 0&&(u=new vh(a),ed(t,u)),t=u}}}var xr=class{constructor(e,t){this.seq=[],this.map={};let n=e.getProgramParameter(t,e.ACTIVE_UNIFORMS);for(let o=0;o<n;++o){let a=e.getActiveUniform(t,o),l=e.getUniformLocation(t,a.name);ix(a,l,this)}let i=[],r=[];for(let o of this.seq)o.type===e.SAMPLER_2D_SHADOW||o.type===e.SAMPLER_CUBE_SHADOW||o.type===e.SAMPLER_2D_ARRAY_SHADOW?i.push(o):r.push(o);i.length>0&&(this.seq=i.concat(r))}setValue(e,t,n,i){let r=this.map[t];r!==void 0&&r.setValue(e,n,i)}setOptional(e,t,n){let i=t[n];i!==void 0&&this.setValue(e,n,i)}static upload(e,t,n,i){for(let r=0,o=t.length;r!==o;++r){let a=t[r],l=n[a.id];l.needsUpdate!==!1&&a.setValue(e,l.value,i)}}static seqWithValue(e,t){let n=[];for(let i=0,r=e.length;i!==r;++i){let o=e[i];o.id in t&&n.push(o)}return n}};function td(s,e,t){let n=s.createShader(e);return s.shaderSource(n,t),s.compileShader(n),n}var sx=37297,rx=0;function ox(s,e){let t=s.split(`
`),n=[],i=Math.max(e-6,0),r=Math.min(e+6,t.length);for(let o=i;o<r;o++){let a=o+1;n.push(`${a===e?">":" "} ${a}: ${t[o]}`)}return n.join(`
`)}var nd=new $e;function ax(s){et._getMatrix(nd,et.workingColorSpace,s);let e=`mat3( ${nd.elements.map(t=>t.toFixed(4))} )`;switch(et.getTransfer(s)){case kr:return[e,"LinearTransferOETF"];case pt:return[e,"sRGBTransferOETF"];default:return Be("WebGLProgram: Unsupported color space: ",s),[e,"LinearTransferOETF"]}}function id(s,e,t){let n=s.getShaderParameter(e,s.COMPILE_STATUS),r=(s.getShaderInfoLog(e)||"").trim();if(n&&r==="")return"";let o=/ERROR: 0:(\d+)/.exec(r);if(o){let a=parseInt(o[1]);return t.toUpperCase()+`

`+r+`

`+ox(s.getShaderSource(e),a)}else return r}function lx(s,e){let t=ax(e);return[`vec4 ${s}( vec4 value ) {`,`	return ${t[1]}( vec4( value.rgb * ${t[0]}, value.a ) );`,"}"].join(`
`)}var cx={[go]:"Linear",[_o]:"Reinhard",[xo]:"Cineon",[bs]:"ACESFilmic",[yo]:"AgX",[Mo]:"Neutral",[vo]:"Custom"};function hx(s,e){let t=cx[e];return t===void 0?(Be("WebGLProgram: Unsupported toneMapping:",e),"vec3 "+s+"( vec3 color ) { return LinearToneMapping( color ); }"):"vec3 "+s+"( vec3 color ) { return "+t+"ToneMapping( color ); }"}var Gl=new D;function ux(){et.getLuminanceCoefficients(Gl);let s=Gl.x.toFixed(4),e=Gl.y.toFixed(4),t=Gl.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${s}, ${e}, ${t} );`,"	return dot( weights, rgb );","}"].join(`
`)}function fx(s){return[s.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",s.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(Do).join(`
`)}function dx(s){let e=[];for(let t in s){let n=s[t];n!==!1&&e.push("#define "+t+" "+n)}return e.join(`
`)}function px(s,e){let t={},n=s.getProgramParameter(e,s.ACTIVE_ATTRIBUTES);for(let i=0;i<n;i++){let r=s.getActiveAttrib(e,i),o=r.name,a=1;r.type===s.FLOAT_MAT2&&(a=2),r.type===s.FLOAT_MAT3&&(a=3),r.type===s.FLOAT_MAT4&&(a=4),t[o]={type:r.type,location:s.getAttribLocation(e,o),locationSize:a}}return t}function Do(s){return s!==""}function sd(s,e){let t=e.numSpotLightShadows+e.numSpotLightMaps-e.numSpotLightShadowsWithMaps;return s.replace(/NUM_DIR_LIGHTS/g,e.numDirLights).replace(/NUM_SPOT_LIGHTS/g,e.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,e.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,t).replace(/NUM_RECT_AREA_LIGHTS/g,e.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,e.numPointLights).replace(/NUM_HEMI_LIGHTS/g,e.numHemiLights).replace(/NUM_DIR_LIGHT_SHADOWS/g,e.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,e.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,e.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,e.numPointLightShadows)}function rd(s,e){return s.replace(/NUM_CLIPPING_PLANES/g,e.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,e.numClippingPlanes-e.numClipIntersection)}var mx=/^[ \t]*#include +<([\w\d./]+)>/gm;function yh(s){return s.replace(mx,_x)}var gx=new Map;function _x(s,e){let t=nt[e];if(t===void 0){let n=gx.get(e);if(n!==void 0)t=nt[n],Be('WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',e,n);else throw new Error("THREE.WebGLProgram: Can not resolve #include <"+e+">")}return yh(t)}var xx=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function od(s){return s.replace(xx,vx)}function vx(s,e,t,n){let i="";for(let r=parseInt(e);r<parseInt(t);r++)i+=n.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return i}function ad(s){let e=`precision ${s.precision} float;
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
#define LOW_PRECISION`),e}var yx={[mo]:"SHADOWMAP_TYPE_PCF",[fr]:"SHADOWMAP_TYPE_VSM"};function Mx(s){return yx[s.shadowMapType]||"SHADOWMAP_TYPE_BASIC"}var bx={[Ji]:"ENVMAP_TYPE_CUBE",[Ss]:"ENVMAP_TYPE_CUBE",[bo]:"ENVMAP_TYPE_CUBE_UV"};function Sx(s){return s.envMap===!1?"ENVMAP_TYPE_CUBE":bx[s.envMapMode]||"ENVMAP_TYPE_CUBE"}var Tx={[Ss]:"ENVMAP_MODE_REFRACTION"};function Ex(s){return s.envMap===!1?"ENVMAP_MODE_REFLECTION":Tx[s.envMapMode]||"ENVMAP_MODE_REFLECTION"}var wx={[ja]:"ENVMAP_BLENDING_MULTIPLY",[Tf]:"ENVMAP_BLENDING_MIX",[Ef]:"ENVMAP_BLENDING_ADD"};function Ax(s){return s.envMap===!1?"ENVMAP_BLENDING_NONE":wx[s.combine]||"ENVMAP_BLENDING_NONE"}function Rx(s){let e=s.envMapCubeUVHeight;if(e===null)return null;let t=Math.log2(e)-2,n=1/e;return{texelWidth:1/(3*Math.max(Math.pow(2,t),112)),texelHeight:n,maxMip:t}}function Cx(s,e,t,n){let i=s.getContext(),r=t.defines,o=t.vertexShader,a=t.fragmentShader,l=Mx(t),c=Sx(t),h=Ex(t),u=Ax(t),f=Rx(t),d=fx(t),g=dx(r),M=i.createProgram(),m,p,S=t.glslVersion?"#version "+t.glslVersion+`
`:"";t.isRawShaderMaterial?(m=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(Do).join(`
`),m.length>0&&(m+=`
`),p=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(Do).join(`
`),p.length>0&&(p+=`
`)):(m=[ad(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",t.batching?"#define USE_BATCHING":"",t.batchingColor?"#define USE_BATCHING_COLOR":"",t.instancing?"#define USE_INSTANCING":"",t.instancingColor?"#define USE_INSTANCING_COLOR":"",t.instancingMorph?"#define USE_INSTANCING_MORPH":"",t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.map?"#define USE_MAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+h:"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.displacementMap?"#define USE_DISPLACEMENTMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.mapUv?"#define MAP_UV "+t.mapUv:"",t.alphaMapUv?"#define ALPHAMAP_UV "+t.alphaMapUv:"",t.lightMapUv?"#define LIGHTMAP_UV "+t.lightMapUv:"",t.aoMapUv?"#define AOMAP_UV "+t.aoMapUv:"",t.emissiveMapUv?"#define EMISSIVEMAP_UV "+t.emissiveMapUv:"",t.bumpMapUv?"#define BUMPMAP_UV "+t.bumpMapUv:"",t.normalMapUv?"#define NORMALMAP_UV "+t.normalMapUv:"",t.displacementMapUv?"#define DISPLACEMENTMAP_UV "+t.displacementMapUv:"",t.metalnessMapUv?"#define METALNESSMAP_UV "+t.metalnessMapUv:"",t.roughnessMapUv?"#define ROUGHNESSMAP_UV "+t.roughnessMapUv:"",t.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+t.anisotropyMapUv:"",t.clearcoatMapUv?"#define CLEARCOATMAP_UV "+t.clearcoatMapUv:"",t.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+t.clearcoatNormalMapUv:"",t.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+t.clearcoatRoughnessMapUv:"",t.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+t.iridescenceMapUv:"",t.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+t.iridescenceThicknessMapUv:"",t.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+t.sheenColorMapUv:"",t.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+t.sheenRoughnessMapUv:"",t.specularMapUv?"#define SPECULARMAP_UV "+t.specularMapUv:"",t.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+t.specularColorMapUv:"",t.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+t.specularIntensityMapUv:"",t.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+t.transmissionMapUv:"",t.thicknessMapUv?"#define THICKNESSMAP_UV "+t.thicknessMapUv:"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexNormals?"#define HAS_NORMAL":"",t.vertexColors?"#define USE_COLOR":"",t.vertexAlphas?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.flatShading?"#define FLAT_SHADED":"",t.skinning?"#define USE_SKINNING":"",t.morphTargets?"#define USE_MORPHTARGETS":"",t.morphNormals&&t.flatShading===!1?"#define USE_MORPHNORMALS":"",t.morphColors?"#define USE_MORPHCOLORS":"",t.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+t.morphTextureStride:"",t.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+t.morphTargetsCount:"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.sizeAttenuation?"#define USE_SIZEATTENUATION":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(Do).join(`
`),p=[ad(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",t.map?"#define USE_MAP":"",t.matcap?"#define USE_MATCAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+c:"",t.envMap?"#define "+h:"",t.envMap?"#define "+u:"",f?"#define CUBEUV_TEXEL_WIDTH "+f.texelWidth:"",f?"#define CUBEUV_TEXEL_HEIGHT "+f.texelHeight:"",f?"#define CUBEUV_MAX_MIP "+f.maxMip+".0":"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.packedNormalMap?"#define USE_PACKED_NORMALMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoat?"#define USE_CLEARCOAT":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.dispersion?"#define USE_DISPERSION":"",t.iridescence?"#define USE_IRIDESCENCE":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaTest?"#define USE_ALPHATEST":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.sheen?"#define USE_SHEEN":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexColors||t.instancingColor?"#define USE_COLOR":"",t.vertexAlphas||t.batchingColor?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.gradientMap?"#define USE_GRADIENTMAP":"",t.flatShading?"#define FLAT_SHADED":"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.numLightProbeGrids>0?"#define USE_LIGHT_PROBES_GRID":"",t.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",t.decodeVideoTextureEmissive?"#define DECODE_VIDEO_TEXTURE_EMISSIVE":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",t.toneMapping!==qn?"#define TONE_MAPPING":"",t.toneMapping!==qn?nt.tonemapping_pars_fragment:"",t.toneMapping!==qn?hx("toneMapping",t.toneMapping):"",t.dithering?"#define DITHERING":"",t.opaque?"#define OPAQUE":"",nt.colorspace_pars_fragment,lx("linearToOutputTexel",t.outputColorSpace),ux(),t.useDepthPacking?"#define DEPTH_PACKING "+t.depthPacking:"",`
`].filter(Do).join(`
`)),o=yh(o),o=sd(o,t),o=rd(o,t),a=yh(a),a=sd(a,t),a=rd(a,t),o=od(o),a=od(a),t.isRawShaderMaterial!==!0&&(S=`#version 300 es
`,m=[d,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+m,p=["#define varying in",t.glslVersion===ih?"":"layout(location = 0) out highp vec4 pc_fragColor;",t.glslVersion===ih?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+p);let R=S+m+o,b=S+p+a,w=td(i,i.VERTEX_SHADER,R),y=td(i,i.FRAGMENT_SHADER,b);i.attachShader(M,w),i.attachShader(M,y),t.index0AttributeName!==void 0?i.bindAttribLocation(M,0,t.index0AttributeName):t.hasPositionAttribute===!0&&i.bindAttribLocation(M,0,"position"),i.linkProgram(M);function C(P){if(s.debug.checkShaderErrors){let L=i.getProgramInfoLog(M)||"",H=i.getShaderInfoLog(w)||"",O=i.getShaderInfoLog(y)||"",U=L.trim(),V=H.trim(),F=O.trim(),Y=!0,J=!0;if(i.getProgramParameter(M,i.LINK_STATUS)===!1)if(Y=!1,typeof s.debug.onShaderError=="function")s.debug.onShaderError(i,M,w,y);else{let j=id(i,w,"vertex"),me=id(i,y,"fragment");Ze("WebGLProgram: Shader Error "+i.getError()+" - VALIDATE_STATUS "+i.getProgramParameter(M,i.VALIDATE_STATUS)+`

Material Name: `+P.name+`
Material Type: `+P.type+`

Program Info Log: `+U+`
`+j+`
`+me)}else U!==""?Be("WebGLProgram: Program Info Log:",U):(V===""||F==="")&&(J=!1);J&&(P.diagnostics={runnable:Y,programLog:U,vertexShader:{log:V,prefix:m},fragmentShader:{log:F,prefix:p}})}i.deleteShader(w),i.deleteShader(y),_=new xr(i,M),v=px(i,M)}let _;this.getUniforms=function(){return _===void 0&&C(this),_};let v;this.getAttributes=function(){return v===void 0&&C(this),v};let E=t.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return E===!1&&(E=i.getProgramParameter(M,sx)),E},this.destroy=function(){n.releaseStatesOfProgram(this),i.deleteProgram(M),this.program=void 0},this.type=t.shaderType,this.name=t.shaderName,this.id=rx++,this.cacheKey=e,this.usedTimes=1,this.program=M,this.vertexShader=w,this.fragmentShader=y,this}var Px=0,Mh=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(e,t,n){let i=this._getShaderCacheForMaterial(e);return i.has(t)===!1&&(i.add(t),t.usedTimes++),i.has(n)===!1&&(i.add(n),n.usedTimes++),this}remove(e){let t=this.materialCache.get(e);for(let n of t)n.usedTimes--,n.usedTimes===0&&this.shaderCache.delete(n.code);return this.materialCache.delete(e),this}getVertexShaderStage(e){return this._getShaderStage(e.vertexShader)}getFragmentShaderStage(e){return this._getShaderStage(e.fragmentShader)}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(e){let t=this.materialCache,n=t.get(e);return n===void 0&&(n=new Set,t.set(e,n)),n}_getShaderStage(e){let t=this.shaderCache,n=t.get(e);return n===void 0&&(n=new bh(e),t.set(e,n)),n}},bh=class{constructor(e){this.id=Px++,this.code=e,this.usedTimes=0}};function Ix(s){return s===$i||s===Ao||s===Ro}function Lx(s,e,t,n,i,r){let o=new js,a=new Mh,l=new Set,c=[],h=new Map,u=n.logarithmicDepthBuffer,f=n.precision,d={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distance",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function g(_){return l.add(_),_===0?"uv":`uv${_}`}function M(_,v,E,P,L,H){let O=P.fog,U=L.geometry,V=_.isMeshStandardMaterial||_.isMeshLambertMaterial||_.isMeshPhongMaterial?P.environment:null,F=_.isMeshStandardMaterial||_.isMeshLambertMaterial&&!_.envMap||_.isMeshPhongMaterial&&!_.envMap,Y=e.get(_.envMap||V,F),J=Y&&Y.mapping===bo?Y.image.height:null,j=d[_.type];_.precision!==null&&(f=n.getMaxPrecision(_.precision),f!==_.precision&&Be("WebGLProgram.getParameters:",_.precision,"not supported, using",f,"instead."));let me=U.morphAttributes.position||U.morphAttributes.normal||U.morphAttributes.color,_e=me!==void 0?me.length:0,Ve=0;U.morphAttributes.position!==void 0&&(Ve=1),U.morphAttributes.normal!==void 0&&(Ve=2),U.morphAttributes.color!==void 0&&(Ve=3);let it,Fe,Q,ge;if(j){let De=hi[j];it=De.vertexShader,Fe=De.fragmentShader}else{it=_.vertexShader,Fe=_.fragmentShader;let De=a.getVertexShaderStage(_),q=a.getFragmentShaderStage(_);a.update(_,De,q),Q=De.id,ge=q.id}let X=s.getRenderTarget(),he=s.state.buffers.depth.getReversed(),$=L.isInstancedMesh===!0,ie=L.isBatchedMesh===!0,oe=!!_.map,ue=!!_.matcap,se=!!Y,ce=!!_.aoMap,ve=!!_.lightMap,Re=!!_.bumpMap&&_.wireframe===!1,We=!!_.normalMap,lt=!!_.displacementMap,ct=!!_.emissiveMap,k=!!_.metalnessMap,ne=!!_.roughnessMap,I=_.anisotropy>0,fe=_.clearcoat>0,N=_.dispersion>0,T=_.iridescence>0,x=_.sheen>0,B=_.transmission>0,G=I&&!!_.anisotropyMap,ee=fe&&!!_.clearcoatMap,pe=fe&&!!_.clearcoatNormalMap,be=fe&&!!_.clearcoatRoughnessMap,re=T&&!!_.iridescenceMap,ae=T&&!!_.iridescenceThicknessMap,Te=x&&!!_.sheenColorMap,Oe=x&&!!_.sheenRoughnessMap,Ae=!!_.specularMap,Ee=!!_.specularColorMap,ze=!!_.specularIntensityMap,qe=B&&!!_.transmissionMap,Ye=B&&!!_.thicknessMap,z=!!_.gradientMap,Se=!!_.alphaMap,le=_.alphaTest>0,we=!!_.alphaHash,Pe=!!_.extensions,de=qn;_.toneMapped&&(X===null||X.isXRRenderTarget===!0)&&(de=s.toneMapping);let ke={shaderID:j,shaderType:_.type,shaderName:_.name,vertexShader:it,fragmentShader:Fe,defines:_.defines,customVertexShaderID:Q,customFragmentShaderID:ge,isRawShaderMaterial:_.isRawShaderMaterial===!0,glslVersion:_.glslVersion,precision:f,batching:ie,batchingColor:ie&&L._colorsTexture!==null,instancing:$,instancingColor:$&&L.instanceColor!==null,instancingMorph:$&&L.morphTexture!==null,outputColorSpace:X===null?s.outputColorSpace:X.isXRRenderTarget===!0?X.texture.colorSpace:et.workingColorSpace,alphaToCoverage:!!_.alphaToCoverage,map:oe,matcap:ue,envMap:se,envMapMode:se&&Y.mapping,envMapCubeUVHeight:J,aoMap:ce,lightMap:ve,bumpMap:Re,normalMap:We,displacementMap:lt,emissiveMap:ct,normalMapObjectSpace:We&&_.normalMapType===If,normalMapTangentSpace:We&&_.normalMapType===Po,packedNormalMap:We&&_.normalMapType===Po&&Ix(_.normalMap.format),metalnessMap:k,roughnessMap:ne,anisotropy:I,anisotropyMap:G,clearcoat:fe,clearcoatMap:ee,clearcoatNormalMap:pe,clearcoatRoughnessMap:be,dispersion:N,iridescence:T,iridescenceMap:re,iridescenceThicknessMap:ae,sheen:x,sheenColorMap:Te,sheenRoughnessMap:Oe,specularMap:Ae,specularColorMap:Ee,specularIntensityMap:ze,transmission:B,transmissionMap:qe,thicknessMap:Ye,gradientMap:z,opaque:_.transparent===!1&&_.blending===us&&_.alphaToCoverage===!1,alphaMap:Se,alphaTest:le,alphaHash:we,combine:_.combine,mapUv:oe&&g(_.map.channel),aoMapUv:ce&&g(_.aoMap.channel),lightMapUv:ve&&g(_.lightMap.channel),bumpMapUv:Re&&g(_.bumpMap.channel),normalMapUv:We&&g(_.normalMap.channel),displacementMapUv:lt&&g(_.displacementMap.channel),emissiveMapUv:ct&&g(_.emissiveMap.channel),metalnessMapUv:k&&g(_.metalnessMap.channel),roughnessMapUv:ne&&g(_.roughnessMap.channel),anisotropyMapUv:G&&g(_.anisotropyMap.channel),clearcoatMapUv:ee&&g(_.clearcoatMap.channel),clearcoatNormalMapUv:pe&&g(_.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:be&&g(_.clearcoatRoughnessMap.channel),iridescenceMapUv:re&&g(_.iridescenceMap.channel),iridescenceThicknessMapUv:ae&&g(_.iridescenceThicknessMap.channel),sheenColorMapUv:Te&&g(_.sheenColorMap.channel),sheenRoughnessMapUv:Oe&&g(_.sheenRoughnessMap.channel),specularMapUv:Ae&&g(_.specularMap.channel),specularColorMapUv:Ee&&g(_.specularColorMap.channel),specularIntensityMapUv:ze&&g(_.specularIntensityMap.channel),transmissionMapUv:qe&&g(_.transmissionMap.channel),thicknessMapUv:Ye&&g(_.thicknessMap.channel),alphaMapUv:Se&&g(_.alphaMap.channel),vertexTangents:!!U.attributes.tangent&&(We||I),vertexNormals:!!U.attributes.normal,vertexColors:_.vertexColors,vertexAlphas:_.vertexColors===!0&&!!U.attributes.color&&U.attributes.color.itemSize===4,pointsUvs:L.isPoints===!0&&!!U.attributes.uv&&(oe||Se),fog:!!O,useFog:_.fog===!0,fogExp2:!!O&&O.isFogExp2,flatShading:_.wireframe===!1&&(_.flatShading===!0||U.attributes.normal===void 0&&We===!1&&(_.isMeshLambertMaterial||_.isMeshPhongMaterial||_.isMeshStandardMaterial||_.isMeshPhysicalMaterial)),sizeAttenuation:_.sizeAttenuation===!0,logarithmicDepthBuffer:u,reversedDepthBuffer:he,skinning:L.isSkinnedMesh===!0,hasPositionAttribute:U.attributes.position!==void 0,morphTargets:U.morphAttributes.position!==void 0,morphNormals:U.morphAttributes.normal!==void 0,morphColors:U.morphAttributes.color!==void 0,morphTargetsCount:_e,morphTextureStride:Ve,numDirLights:v.directional.length,numPointLights:v.point.length,numSpotLights:v.spot.length,numSpotLightMaps:v.spotLightMap.length,numRectAreaLights:v.rectArea.length,numHemiLights:v.hemi.length,numDirLightShadows:v.directionalShadowMap.length,numPointLightShadows:v.pointShadowMap.length,numSpotLightShadows:v.spotShadowMap.length,numSpotLightShadowsWithMaps:v.numSpotLightShadowsWithMaps,numLightProbes:v.numLightProbes,numLightProbeGrids:H.length,numClippingPlanes:r.numPlanes,numClipIntersection:r.numIntersection,dithering:_.dithering,shadowMapEnabled:s.shadowMap.enabled&&E.length>0,shadowMapType:s.shadowMap.type,toneMapping:de,decodeVideoTexture:oe&&_.map.isVideoTexture===!0&&et.getTransfer(_.map.colorSpace)===pt,decodeVideoTextureEmissive:ct&&_.emissiveMap.isVideoTexture===!0&&et.getTransfer(_.emissiveMap.colorSpace)===pt,premultipliedAlpha:_.premultipliedAlpha,doubleSided:_.side===Bt,flipSided:_.side===Wt,useDepthPacking:_.depthPacking>=0,depthPacking:_.depthPacking||0,index0AttributeName:_.index0AttributeName,extensionClipCullDistance:Pe&&_.extensions.clipCullDistance===!0&&t.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(Pe&&_.extensions.multiDraw===!0||ie)&&t.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:t.has("KHR_parallel_shader_compile"),customProgramCacheKey:_.customProgramCacheKey()};return ke.vertexUv1s=l.has(1),ke.vertexUv2s=l.has(2),ke.vertexUv3s=l.has(3),l.clear(),ke}function m(_){let v=[];if(_.shaderID?v.push(_.shaderID):(v.push(_.customVertexShaderID),v.push(_.customFragmentShaderID)),_.defines!==void 0)for(let E in _.defines)v.push(E),v.push(_.defines[E]);return _.isRawShaderMaterial===!1&&(p(v,_),S(v,_),v.push(s.outputColorSpace)),v.push(_.customProgramCacheKey),v.join()}function p(_,v){_.push(v.precision),_.push(v.outputColorSpace),_.push(v.envMapMode),_.push(v.envMapCubeUVHeight),_.push(v.mapUv),_.push(v.alphaMapUv),_.push(v.lightMapUv),_.push(v.aoMapUv),_.push(v.bumpMapUv),_.push(v.normalMapUv),_.push(v.displacementMapUv),_.push(v.emissiveMapUv),_.push(v.metalnessMapUv),_.push(v.roughnessMapUv),_.push(v.anisotropyMapUv),_.push(v.clearcoatMapUv),_.push(v.clearcoatNormalMapUv),_.push(v.clearcoatRoughnessMapUv),_.push(v.iridescenceMapUv),_.push(v.iridescenceThicknessMapUv),_.push(v.sheenColorMapUv),_.push(v.sheenRoughnessMapUv),_.push(v.specularMapUv),_.push(v.specularColorMapUv),_.push(v.specularIntensityMapUv),_.push(v.transmissionMapUv),_.push(v.thicknessMapUv),_.push(v.combine),_.push(v.fogExp2),_.push(v.sizeAttenuation),_.push(v.morphTargetsCount),_.push(v.morphAttributeCount),_.push(v.numDirLights),_.push(v.numPointLights),_.push(v.numSpotLights),_.push(v.numSpotLightMaps),_.push(v.numHemiLights),_.push(v.numRectAreaLights),_.push(v.numDirLightShadows),_.push(v.numPointLightShadows),_.push(v.numSpotLightShadows),_.push(v.numSpotLightShadowsWithMaps),_.push(v.numLightProbes),_.push(v.shadowMapType),_.push(v.toneMapping),_.push(v.numClippingPlanes),_.push(v.numClipIntersection),_.push(v.depthPacking)}function S(_,v){o.disableAll(),v.instancing&&o.enable(0),v.instancingColor&&o.enable(1),v.instancingMorph&&o.enable(2),v.matcap&&o.enable(3),v.envMap&&o.enable(4),v.normalMapObjectSpace&&o.enable(5),v.normalMapTangentSpace&&o.enable(6),v.clearcoat&&o.enable(7),v.iridescence&&o.enable(8),v.alphaTest&&o.enable(9),v.vertexColors&&o.enable(10),v.vertexAlphas&&o.enable(11),v.vertexUv1s&&o.enable(12),v.vertexUv2s&&o.enable(13),v.vertexUv3s&&o.enable(14),v.vertexTangents&&o.enable(15),v.anisotropy&&o.enable(16),v.alphaHash&&o.enable(17),v.batching&&o.enable(18),v.dispersion&&o.enable(19),v.batchingColor&&o.enable(20),v.gradientMap&&o.enable(21),v.packedNormalMap&&o.enable(22),v.vertexNormals&&o.enable(23),_.push(o.mask),o.disableAll(),v.fog&&o.enable(0),v.useFog&&o.enable(1),v.flatShading&&o.enable(2),v.logarithmicDepthBuffer&&o.enable(3),v.reversedDepthBuffer&&o.enable(4),v.skinning&&o.enable(5),v.morphTargets&&o.enable(6),v.morphNormals&&o.enable(7),v.morphColors&&o.enable(8),v.premultipliedAlpha&&o.enable(9),v.shadowMapEnabled&&o.enable(10),v.doubleSided&&o.enable(11),v.flipSided&&o.enable(12),v.useDepthPacking&&o.enable(13),v.dithering&&o.enable(14),v.transmission&&o.enable(15),v.sheen&&o.enable(16),v.opaque&&o.enable(17),v.pointsUvs&&o.enable(18),v.decodeVideoTexture&&o.enable(19),v.decodeVideoTextureEmissive&&o.enable(20),v.alphaToCoverage&&o.enable(21),v.numLightProbeGrids>0&&o.enable(22),v.hasPositionAttribute&&o.enable(23),_.push(o.mask)}function R(_){let v=d[_.type],E;if(v){let P=hi[v];E=Kn.clone(P.uniforms)}else E=_.uniforms;return E}function b(_,v){let E=h.get(v);return E!==void 0?++E.usedTimes:(E=new Cx(s,v,_,i),c.push(E),h.set(v,E)),E}function w(_){if(--_.usedTimes===0){let v=c.indexOf(_);c[v]=c[c.length-1],c.pop(),h.delete(_.cacheKey),_.destroy()}}function y(_){a.remove(_)}function C(){a.dispose()}return{getParameters:M,getProgramCacheKey:m,getUniforms:R,acquireProgram:b,releaseProgram:w,releaseShaderCache:y,programs:c,dispose:C}}function Dx(){let s=new WeakMap;function e(o){return s.has(o)}function t(o){let a=s.get(o);return a===void 0&&(a={},s.set(o,a)),a}function n(o){s.delete(o)}function i(o,a,l){s.get(o)[a]=l}function r(){s=new WeakMap}return{has:e,get:t,remove:n,update:i,dispose:r}}function Nx(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.material.id!==e.material.id?s.material.id-e.material.id:s.materialVariant!==e.materialVariant?s.materialVariant-e.materialVariant:s.z!==e.z?s.z-e.z:s.id-e.id}function ld(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.z!==e.z?e.z-s.z:s.id-e.id}function cd(){let s=[],e=0,t=[],n=[],i=[];function r(){e=0,t.length=0,n.length=0,i.length=0}function o(f){let d=0;return f.isInstancedMesh&&(d+=2),f.isSkinnedMesh&&(d+=1),d}function a(f,d,g,M,m,p){let S=s[e];return S===void 0?(S={id:f.id,object:f,geometry:d,material:g,materialVariant:o(f),groupOrder:M,renderOrder:f.renderOrder,z:m,group:p},s[e]=S):(S.id=f.id,S.object=f,S.geometry=d,S.material=g,S.materialVariant=o(f),S.groupOrder=M,S.renderOrder=f.renderOrder,S.z=m,S.group=p),e++,S}function l(f,d,g,M,m,p){let S=a(f,d,g,M,m,p);g.transmission>0?n.push(S):g.transparent===!0?i.push(S):t.push(S)}function c(f,d,g,M,m,p){let S=a(f,d,g,M,m,p);g.transmission>0?n.unshift(S):g.transparent===!0?i.unshift(S):t.unshift(S)}function h(f,d,g){t.length>1&&t.sort(f||Nx),n.length>1&&n.sort(d||ld),i.length>1&&i.sort(d||ld),g&&(t.reverse(),n.reverse(),i.reverse())}function u(){for(let f=e,d=s.length;f<d;f++){let g=s[f];if(g.id===null)break;g.id=null,g.object=null,g.geometry=null,g.material=null,g.group=null}}return{opaque:t,transmissive:n,transparent:i,init:r,push:l,unshift:c,finish:u,sort:h}}function Ux(){let s=new WeakMap;function e(n,i){let r=s.get(n),o;return r===void 0?(o=new cd,s.set(n,[o])):i>=r.length?(o=new cd,r.push(o)):o=r[i],o}function t(){s=new WeakMap}return{get:e,dispose:t}}function Fx(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"DirectionalLight":t={direction:new D,color:new Me};break;case"SpotLight":t={position:new D,direction:new D,color:new Me,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":t={position:new D,color:new Me,distance:0,decay:0};break;case"HemisphereLight":t={direction:new D,skyColor:new Me,groundColor:new Me};break;case"RectAreaLight":t={color:new Me,position:new D,halfWidth:new D,halfHeight:new D};break}return s[e.id]=t,t}}}function Ox(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"DirectionalLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new ye};break;case"SpotLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new ye};break;case"PointLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new ye,shadowCameraNear:1,shadowCameraFar:1e3};break}return s[e.id]=t,t}}}var Bx=0;function zx(s,e){return(e.castShadow?2:0)-(s.castShadow?2:0)+(e.map?1:0)-(s.map?1:0)}function kx(s){let e=new Fx,t=Ox(),n={version:0,hash:{directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let c=0;c<9;c++)n.probe.push(new D);let i=new D,r=new Ke,o=new Ke;function a(c){let h=0,u=0,f=0;for(let v=0;v<9;v++)n.probe[v].set(0,0,0);let d=0,g=0,M=0,m=0,p=0,S=0,R=0,b=0,w=0,y=0,C=0;c.sort(zx);for(let v=0,E=c.length;v<E;v++){let P=c[v],L=P.color,H=P.intensity,O=P.distance,U=null;if(P.shadow&&P.shadow.map&&(P.shadow.map.texture.format===$i?U=P.shadow.map.texture:U=P.shadow.map.depthTexture||P.shadow.map.texture),P.isAmbientLight)h+=L.r*H,u+=L.g*H,f+=L.b*H;else if(P.isLightProbe){for(let V=0;V<9;V++)n.probe[V].addScaledVector(P.sh.coefficients[V],H);C++}else if(P.isDirectionalLight){let V=e.get(P);if(V.color.copy(P.color).multiplyScalar(P.intensity),P.castShadow){let F=P.shadow,Y=t.get(P);Y.shadowIntensity=F.intensity,Y.shadowBias=F.bias,Y.shadowNormalBias=F.normalBias,Y.shadowRadius=F.radius,Y.shadowMapSize=F.mapSize,n.directionalShadow[d]=Y,n.directionalShadowMap[d]=U,n.directionalShadowMatrix[d]=P.shadow.matrix,S++}n.directional[d]=V,d++}else if(P.isSpotLight){let V=e.get(P);V.position.setFromMatrixPosition(P.matrixWorld),V.color.copy(L).multiplyScalar(H),V.distance=O,V.coneCos=Math.cos(P.angle),V.penumbraCos=Math.cos(P.angle*(1-P.penumbra)),V.decay=P.decay,n.spot[M]=V;let F=P.shadow;if(P.map&&(n.spotLightMap[w]=P.map,w++,F.updateMatrices(P),P.castShadow&&y++),n.spotLightMatrix[M]=F.matrix,P.castShadow){let Y=t.get(P);Y.shadowIntensity=F.intensity,Y.shadowBias=F.bias,Y.shadowNormalBias=F.normalBias,Y.shadowRadius=F.radius,Y.shadowMapSize=F.mapSize,n.spotShadow[M]=Y,n.spotShadowMap[M]=U,b++}M++}else if(P.isRectAreaLight){let V=e.get(P);V.color.copy(L).multiplyScalar(H),V.halfWidth.set(P.width*.5,0,0),V.halfHeight.set(0,P.height*.5,0),n.rectArea[m]=V,m++}else if(P.isPointLight){let V=e.get(P);if(V.color.copy(P.color).multiplyScalar(P.intensity),V.distance=P.distance,V.decay=P.decay,P.castShadow){let F=P.shadow,Y=t.get(P);Y.shadowIntensity=F.intensity,Y.shadowBias=F.bias,Y.shadowNormalBias=F.normalBias,Y.shadowRadius=F.radius,Y.shadowMapSize=F.mapSize,Y.shadowCameraNear=F.camera.near,Y.shadowCameraFar=F.camera.far,n.pointShadow[g]=Y,n.pointShadowMap[g]=U,n.pointShadowMatrix[g]=P.shadow.matrix,R++}n.point[g]=V,g++}else if(P.isHemisphereLight){let V=e.get(P);V.skyColor.copy(P.color).multiplyScalar(H),V.groundColor.copy(P.groundColor).multiplyScalar(H),n.hemi[p]=V,p++}}m>0&&(s.has("OES_texture_float_linear")===!0?(n.rectAreaLTC1=Ce.LTC_FLOAT_1,n.rectAreaLTC2=Ce.LTC_FLOAT_2):(n.rectAreaLTC1=Ce.LTC_HALF_1,n.rectAreaLTC2=Ce.LTC_HALF_2)),n.ambient[0]=h,n.ambient[1]=u,n.ambient[2]=f;let _=n.hash;(_.directionalLength!==d||_.pointLength!==g||_.spotLength!==M||_.rectAreaLength!==m||_.hemiLength!==p||_.numDirectionalShadows!==S||_.numPointShadows!==R||_.numSpotShadows!==b||_.numSpotMaps!==w||_.numLightProbes!==C)&&(n.directional.length=d,n.spot.length=M,n.rectArea.length=m,n.point.length=g,n.hemi.length=p,n.directionalShadow.length=S,n.directionalShadowMap.length=S,n.pointShadow.length=R,n.pointShadowMap.length=R,n.spotShadow.length=b,n.spotShadowMap.length=b,n.directionalShadowMatrix.length=S,n.pointShadowMatrix.length=R,n.spotLightMatrix.length=b+w-y,n.spotLightMap.length=w,n.numSpotLightShadowsWithMaps=y,n.numLightProbes=C,_.directionalLength=d,_.pointLength=g,_.spotLength=M,_.rectAreaLength=m,_.hemiLength=p,_.numDirectionalShadows=S,_.numPointShadows=R,_.numSpotShadows=b,_.numSpotMaps=w,_.numLightProbes=C,n.version=Bx++)}function l(c,h){let u=0,f=0,d=0,g=0,M=0,m=h.matrixWorldInverse;for(let p=0,S=c.length;p<S;p++){let R=c[p];if(R.isDirectionalLight){let b=n.directional[u];b.direction.setFromMatrixPosition(R.matrixWorld),i.setFromMatrixPosition(R.target.matrixWorld),b.direction.sub(i),b.direction.transformDirection(m),u++}else if(R.isSpotLight){let b=n.spot[d];b.position.setFromMatrixPosition(R.matrixWorld),b.position.applyMatrix4(m),b.direction.setFromMatrixPosition(R.matrixWorld),i.setFromMatrixPosition(R.target.matrixWorld),b.direction.sub(i),b.direction.transformDirection(m),d++}else if(R.isRectAreaLight){let b=n.rectArea[g];b.position.setFromMatrixPosition(R.matrixWorld),b.position.applyMatrix4(m),o.identity(),r.copy(R.matrixWorld),r.premultiply(m),o.extractRotation(r),b.halfWidth.set(R.width*.5,0,0),b.halfHeight.set(0,R.height*.5,0),b.halfWidth.applyMatrix4(o),b.halfHeight.applyMatrix4(o),g++}else if(R.isPointLight){let b=n.point[f];b.position.setFromMatrixPosition(R.matrixWorld),b.position.applyMatrix4(m),f++}else if(R.isHemisphereLight){let b=n.hemi[M];b.direction.setFromMatrixPosition(R.matrixWorld),b.direction.transformDirection(m),M++}}}return{setup:a,setupView:l,state:n}}function hd(s){let e=new kx(s),t=[],n=[],i=[];function r(f){u.camera=f,t.length=0,n.length=0,i.length=0}function o(f){t.push(f)}function a(f){n.push(f)}function l(f){i.push(f)}function c(){e.setup(t)}function h(f){e.setupView(t,f)}let u={lightsArray:t,shadowsArray:n,lightProbeGridArray:i,camera:null,lights:e,transmissionRenderTarget:{},textureUnits:0};return{init:r,state:u,setupLights:c,setupLightsView:h,pushLight:o,pushShadow:a,pushLightProbeGrid:l}}function Hx(s){let e=new WeakMap;function t(i,r=0){let o=e.get(i),a;return o===void 0?(a=new hd(s),e.set(i,[a])):r>=o.length?(a=new hd(s),o.push(a)):a=o[r],a}function n(){e=new WeakMap}return{get:t,dispose:n}}var Vx=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,Gx=`uniform sampler2D shadow_pass;
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
}`,Wx=[new D(1,0,0),new D(-1,0,0),new D(0,1,0),new D(0,-1,0),new D(0,0,1),new D(0,0,-1)],Xx=[new D(0,-1,0),new D(0,-1,0),new D(0,0,1),new D(0,0,-1),new D(0,-1,0),new D(0,-1,0)],ud=new Ke,Lo=new D,mh=new D;function qx(s,e,t){let n=new ir,i=new ye,r=new ye,o=new xt,a=new Ba,l=new za,c={},h=t.maxTextureSize,u={[Sn]:Wt,[Wt]:Sn,[Bt]:Bt},f=new mt({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new ye},radius:{value:4}},vertexShader:Vx,fragmentShader:Gx}),d=f.clone();d.defines.HORIZONTAL_PASS=1;let g=new ft;g.setAttribute("position",new vt(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let M=new Je(g,f),m=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=mo;let p=this.type;this.render=function(y,C,_){if(m.enabled===!1||m.autoUpdate===!1&&m.needsUpdate===!1||y.length===0)return;this.type===Ja&&(Be("WebGLShadowMap: PCFSoftShadowMap has been deprecated. Using PCFShadowMap instead."),this.type=mo);let v=s.getRenderTarget(),E=s.getActiveCubeFace(),P=s.getActiveMipmapLevel(),L=s.state;L.setBlending(On),L.buffers.depth.getReversed()===!0?L.buffers.color.setClear(0,0,0,0):L.buffers.color.setClear(1,1,1,1),L.buffers.depth.setTest(!0),L.setScissorTest(!1);let H=p!==this.type;H&&C.traverse(function(O){O.material&&(Array.isArray(O.material)?O.material.forEach(U=>U.needsUpdate=!0):O.material.needsUpdate=!0)});for(let O=0,U=y.length;O<U;O++){let V=y[O],F=V.shadow;if(F===void 0){Be("WebGLShadowMap:",V,"has no shadow.");continue}if(F.autoUpdate===!1&&F.needsUpdate===!1)continue;i.copy(F.mapSize);let Y=F.getFrameExtents();i.multiply(Y),r.copy(F.mapSize),(i.x>h||i.y>h)&&(i.x>h&&(r.x=Math.floor(h/Y.x),i.x=r.x*Y.x,F.mapSize.x=r.x),i.y>h&&(r.y=Math.floor(h/Y.y),i.y=r.y*Y.y,F.mapSize.y=r.y));let J=s.state.buffers.depth.getReversed();if(F.camera._reversedDepth=J,F.map===null||H===!0){if(F.map!==null&&(F.map.depthTexture!==null&&(F.map.depthTexture.dispose(),F.map.depthTexture=null),F.map.dispose()),this.type===fr){if(V.isPointLight){Be("WebGLShadowMap: VSM shadow maps are not supported for PointLights. Use PCF or BasicShadowMap instead.");continue}F.map=new Nt(i.x,i.y,{format:$i,type:Xt,minFilter:Lt,magFilter:Lt,generateMipmaps:!1}),F.map.texture.name=V.name+".shadowMap",F.map.depthTexture=new bi(i.x,i.y,En),F.map.depthTexture.name=V.name+".shadowMapDepth",F.map.depthTexture.format=ti,F.map.depthTexture.compareFunction=null,F.map.depthTexture.minFilter=Ot,F.map.depthTexture.magFilter=Ot}else V.isPointLight?(F.map=new Wl(i.x),F.map.depthTexture=new Pa(i.x,Zn)):(F.map=new Nt(i.x,i.y),F.map.depthTexture=new bi(i.x,i.y,Zn)),F.map.depthTexture.name=V.name+".shadowMap",F.map.depthTexture.format=ti,this.type===mo?(F.map.depthTexture.compareFunction=J?Hl:kl,F.map.depthTexture.minFilter=Lt,F.map.depthTexture.magFilter=Lt):(F.map.depthTexture.compareFunction=null,F.map.depthTexture.minFilter=Ot,F.map.depthTexture.magFilter=Ot);F.camera.updateProjectionMatrix()}let j=F.map.isWebGLCubeRenderTarget?6:1;for(let me=0;me<j;me++){if(F.map.isWebGLCubeRenderTarget)s.setRenderTarget(F.map,me),s.clear();else{me===0&&(s.setRenderTarget(F.map),s.clear());let _e=F.getViewport(me);o.set(r.x*_e.x,r.y*_e.y,r.x*_e.z,r.y*_e.w),L.viewport(o)}if(V.isPointLight){let _e=F.camera,Ve=F.matrix,it=V.distance||_e.far;it!==_e.far&&(_e.far=it,_e.updateProjectionMatrix()),Lo.setFromMatrixPosition(V.matrixWorld),_e.position.copy(Lo),mh.copy(_e.position),mh.add(Wx[me]),_e.up.copy(Xx[me]),_e.lookAt(mh),_e.updateMatrixWorld(),Ve.makeTranslation(-Lo.x,-Lo.y,-Lo.z),ud.multiplyMatrices(_e.projectionMatrix,_e.matrixWorldInverse),F._frustum.setFromProjectionMatrix(ud,_e.coordinateSystem,_e.reversedDepth)}else F.updateMatrices(V);n=F.getFrustum(),b(C,_,F.camera,V,this.type)}F.isPointLightShadow!==!0&&this.type===fr&&S(F,_),F.needsUpdate=!1}p=this.type,m.needsUpdate=!1,s.setRenderTarget(v,E,P)};function S(y,C){let _=e.update(M);f.defines.VSM_SAMPLES!==y.blurSamples&&(f.defines.VSM_SAMPLES=y.blurSamples,d.defines.VSM_SAMPLES=y.blurSamples,f.needsUpdate=!0,d.needsUpdate=!0),y.mapPass===null&&(y.mapPass=new Nt(i.x,i.y,{format:$i,type:Xt})),f.uniforms.shadow_pass.value=y.map.depthTexture,f.uniforms.resolution.value=y.mapSize,f.uniforms.radius.value=y.radius,s.setRenderTarget(y.mapPass),s.clear(),s.renderBufferDirect(C,null,_,f,M,null),d.uniforms.shadow_pass.value=y.mapPass.texture,d.uniforms.resolution.value=y.mapSize,d.uniforms.radius.value=y.radius,s.setRenderTarget(y.map),s.clear(),s.renderBufferDirect(C,null,_,d,M,null)}function R(y,C,_,v){let E=null,P=_.isPointLight===!0?y.customDistanceMaterial:y.customDepthMaterial;if(P!==void 0)E=P;else if(E=_.isPointLight===!0?l:a,s.localClippingEnabled&&C.clipShadows===!0&&Array.isArray(C.clippingPlanes)&&C.clippingPlanes.length!==0||C.displacementMap&&C.displacementScale!==0||C.alphaMap&&C.alphaTest>0||C.map&&C.alphaTest>0||C.alphaToCoverage===!0){let L=E.uuid,H=C.uuid,O=c[L];O===void 0&&(O={},c[L]=O);let U=O[H];U===void 0&&(U=E.clone(),O[H]=U,C.addEventListener("dispose",w)),E=U}if(E.visible=C.visible,E.wireframe=C.wireframe,v===fr?E.side=C.shadowSide!==null?C.shadowSide:C.side:E.side=C.shadowSide!==null?C.shadowSide:u[C.side],E.alphaMap=C.alphaMap,E.alphaTest=C.alphaToCoverage===!0?.5:C.alphaTest,E.map=C.map,E.clipShadows=C.clipShadows,E.clippingPlanes=C.clippingPlanes,E.clipIntersection=C.clipIntersection,E.displacementMap=C.displacementMap,E.displacementScale=C.displacementScale,E.displacementBias=C.displacementBias,E.wireframeLinewidth=C.wireframeLinewidth,E.linewidth=C.linewidth,_.isPointLight===!0&&E.isMeshDistanceMaterial===!0){let L=s.properties.get(E);L.light=_}return E}function b(y,C,_,v,E){if(y.visible===!1)return;if(y.layers.test(C.layers)&&(y.isMesh||y.isLine||y.isPoints)&&(y.castShadow||y.receiveShadow&&E===fr)&&(!y.frustumCulled||n.intersectsObject(y))){y.modelViewMatrix.multiplyMatrices(_.matrixWorldInverse,y.matrixWorld);let H=e.update(y),O=y.material;if(Array.isArray(O)){let U=H.groups;for(let V=0,F=U.length;V<F;V++){let Y=U[V],J=O[Y.materialIndex];if(J&&J.visible){let j=R(y,J,v,E);y.onBeforeShadow(s,y,C,_,H,j,Y),s.renderBufferDirect(_,null,H,j,y,Y),y.onAfterShadow(s,y,C,_,H,j,Y)}}}else if(O.visible){let U=R(y,O,v,E);y.onBeforeShadow(s,y,C,_,H,U,null),s.renderBufferDirect(_,null,H,U,y,null),y.onAfterShadow(s,y,C,_,H,U,null)}}let L=y.children;for(let H=0,O=L.length;H<O;H++)b(L[H],C,_,v,E)}function w(y){y.target.removeEventListener("dispose",w);for(let _ in c){let v=c[_],E=y.target.uuid;E in v&&(v[E].dispose(),delete v[E])}}}function Yx(s,e){function t(){let z=!1,Se=new xt,le=null,we=new xt(0,0,0,0);return{setMask:function(Pe){le!==Pe&&!z&&(s.colorMask(Pe,Pe,Pe,Pe),le=Pe)},setLocked:function(Pe){z=Pe},setClear:function(Pe,de,ke,De,q){q===!0&&(Pe*=De,de*=De,ke*=De),Se.set(Pe,de,ke,De),we.equals(Se)===!1&&(s.clearColor(Pe,de,ke,De),we.copy(Se))},reset:function(){z=!1,le=null,we.set(-1,0,0,0)}}}function n(){let z=!1,Se=!1,le=null,we=null,Pe=null;return{setReversed:function(de){if(Se!==de){let ke=e.get("EXT_clip_control");de?ke.clipControlEXT(ke.LOWER_LEFT_EXT,ke.ZERO_TO_ONE_EXT):ke.clipControlEXT(ke.LOWER_LEFT_EXT,ke.NEGATIVE_ONE_TO_ONE_EXT),Se=de;let De=Pe;Pe=null,this.setClear(De)}},getReversed:function(){return Se},setTest:function(de){de?X(s.DEPTH_TEST):he(s.DEPTH_TEST)},setMask:function(de){le!==de&&!z&&(s.depthMask(de),le=de)},setFunc:function(de){if(Se&&(de=Hf[de]),we!==de){switch(de){case _a:s.depthFunc(s.NEVER);break;case xa:s.depthFunc(s.ALWAYS);break;case va:s.depthFunc(s.LESS);break;case fs:s.depthFunc(s.LEQUAL);break;case ya:s.depthFunc(s.EQUAL);break;case Ma:s.depthFunc(s.GEQUAL);break;case ba:s.depthFunc(s.GREATER);break;case Sa:s.depthFunc(s.NOTEQUAL);break;default:s.depthFunc(s.LEQUAL)}we=de}},setLocked:function(de){z=de},setClear:function(de){Pe!==de&&(Pe=de,Se&&(de=1-de),s.clearDepth(de))},reset:function(){z=!1,le=null,we=null,Pe=null,Se=!1}}}function i(){let z=!1,Se=null,le=null,we=null,Pe=null,de=null,ke=null,De=null,q=null;return{setTest:function(xe){z||(xe?X(s.STENCIL_TEST):he(s.STENCIL_TEST))},setMask:function(xe){Se!==xe&&!z&&(s.stencilMask(xe),Se=xe)},setFunc:function(xe,rt,je){(le!==xe||we!==rt||Pe!==je)&&(s.stencilFunc(xe,rt,je),le=xe,we=rt,Pe=je)},setOp:function(xe,rt,je){(de!==xe||ke!==rt||De!==je)&&(s.stencilOp(xe,rt,je),de=xe,ke=rt,De=je)},setLocked:function(xe){z=xe},setClear:function(xe){q!==xe&&(s.clearStencil(xe),q=xe)},reset:function(){z=!1,Se=null,le=null,we=null,Pe=null,de=null,ke=null,De=null,q=null}}}let r=new t,o=new n,a=new i,l=new WeakMap,c=new WeakMap,h={},u={},f={},d=new WeakMap,g=[],M=null,m=!1,p=null,S=null,R=null,b=null,w=null,y=null,C=null,_=new Me(0,0,0),v=0,E=!1,P=null,L=null,H=null,O=null,U=null,V=s.getParameter(s.MAX_COMBINED_TEXTURE_IMAGE_UNITS),F=!1,Y=0,J=s.getParameter(s.VERSION);J.indexOf("WebGL")!==-1?(Y=parseFloat(/^WebGL (\d)/.exec(J)[1]),F=Y>=1):J.indexOf("OpenGL ES")!==-1&&(Y=parseFloat(/^OpenGL ES (\d)/.exec(J)[1]),F=Y>=2);let j=null,me={},_e=s.getParameter(s.SCISSOR_BOX),Ve=s.getParameter(s.VIEWPORT),it=new xt().fromArray(_e),Fe=new xt().fromArray(Ve);function Q(z,Se,le,we){let Pe=new Uint8Array(4),de=s.createTexture();s.bindTexture(z,de),s.texParameteri(z,s.TEXTURE_MIN_FILTER,s.NEAREST),s.texParameteri(z,s.TEXTURE_MAG_FILTER,s.NEAREST);for(let ke=0;ke<le;ke++)z===s.TEXTURE_3D||z===s.TEXTURE_2D_ARRAY?s.texImage3D(Se,0,s.RGBA,1,1,we,0,s.RGBA,s.UNSIGNED_BYTE,Pe):s.texImage2D(Se+ke,0,s.RGBA,1,1,0,s.RGBA,s.UNSIGNED_BYTE,Pe);return de}let ge={};ge[s.TEXTURE_2D]=Q(s.TEXTURE_2D,s.TEXTURE_2D,1),ge[s.TEXTURE_CUBE_MAP]=Q(s.TEXTURE_CUBE_MAP,s.TEXTURE_CUBE_MAP_POSITIVE_X,6),ge[s.TEXTURE_2D_ARRAY]=Q(s.TEXTURE_2D_ARRAY,s.TEXTURE_2D_ARRAY,1,1),ge[s.TEXTURE_3D]=Q(s.TEXTURE_3D,s.TEXTURE_3D,1,1),r.setClear(0,0,0,1),o.setClear(1),a.setClear(0),X(s.DEPTH_TEST),o.setFunc(fs),Re(!1),We(qc),X(s.CULL_FACE),ce(On);function X(z){h[z]!==!0&&(s.enable(z),h[z]=!0)}function he(z){h[z]!==!1&&(s.disable(z),h[z]=!1)}function $(z,Se){return f[z]!==Se?(s.bindFramebuffer(z,Se),f[z]=Se,z===s.DRAW_FRAMEBUFFER&&(f[s.FRAMEBUFFER]=Se),z===s.FRAMEBUFFER&&(f[s.DRAW_FRAMEBUFFER]=Se),!0):!1}function ie(z,Se){let le=g,we=!1;if(z){le=d.get(Se),le===void 0&&(le=[],d.set(Se,le));let Pe=z.textures;if(le.length!==Pe.length||le[0]!==s.COLOR_ATTACHMENT0){for(let de=0,ke=Pe.length;de<ke;de++)le[de]=s.COLOR_ATTACHMENT0+de;le.length=Pe.length,we=!0}}else le[0]!==s.BACK&&(le[0]=s.BACK,we=!0);we&&s.drawBuffers(le)}function oe(z){return M!==z?(s.useProgram(z),M=z,!0):!1}let ue={[Vi]:s.FUNC_ADD,[af]:s.FUNC_SUBTRACT,[lf]:s.FUNC_REVERSE_SUBTRACT};ue[cf]=s.MIN,ue[hf]=s.MAX;let se={[uf]:s.ZERO,[ff]:s.ONE,[df]:s.SRC_COLOR,[ma]:s.SRC_ALPHA,[vf]:s.SRC_ALPHA_SATURATE,[_f]:s.DST_COLOR,[mf]:s.DST_ALPHA,[pf]:s.ONE_MINUS_SRC_COLOR,[ga]:s.ONE_MINUS_SRC_ALPHA,[xf]:s.ONE_MINUS_DST_COLOR,[gf]:s.ONE_MINUS_DST_ALPHA,[yf]:s.CONSTANT_COLOR,[Mf]:s.ONE_MINUS_CONSTANT_COLOR,[bf]:s.CONSTANT_ALPHA,[Sf]:s.ONE_MINUS_CONSTANT_ALPHA};function ce(z,Se,le,we,Pe,de,ke,De,q,xe){if(z===On){m===!0&&(he(s.BLEND),m=!1);return}if(m===!1&&(X(s.BLEND),m=!0),z!==of){if(z!==p||xe!==E){if((S!==Vi||w!==Vi)&&(s.blendEquation(s.FUNC_ADD),S=Vi,w=Vi),xe)switch(z){case us:s.blendFuncSeparate(s.ONE,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case rn:s.blendFunc(s.ONE,s.ONE);break;case Yc:s.blendFuncSeparate(s.ZERO,s.ONE_MINUS_SRC_COLOR,s.ZERO,s.ONE);break;case Zc:s.blendFuncSeparate(s.DST_COLOR,s.ONE_MINUS_SRC_ALPHA,s.ZERO,s.ONE);break;default:Ze("WebGLState: Invalid blending: ",z);break}else switch(z){case us:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case rn:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE,s.ONE,s.ONE);break;case Yc:Ze("WebGLState: SubtractiveBlending requires material.premultipliedAlpha = true");break;case Zc:Ze("WebGLState: MultiplyBlending requires material.premultipliedAlpha = true");break;default:Ze("WebGLState: Invalid blending: ",z);break}R=null,b=null,y=null,C=null,_.set(0,0,0),v=0,p=z,E=xe}return}Pe=Pe||Se,de=de||le,ke=ke||we,(Se!==S||Pe!==w)&&(s.blendEquationSeparate(ue[Se],ue[Pe]),S=Se,w=Pe),(le!==R||we!==b||de!==y||ke!==C)&&(s.blendFuncSeparate(se[le],se[we],se[de],se[ke]),R=le,b=we,y=de,C=ke),(De.equals(_)===!1||q!==v)&&(s.blendColor(De.r,De.g,De.b,q),_.copy(De),v=q),p=z,E=!1}function ve(z,Se){z.side===Bt?he(s.CULL_FACE):X(s.CULL_FACE);let le=z.side===Wt;Se&&(le=!le),Re(le),z.blending===us&&z.transparent===!1?ce(On):ce(z.blending,z.blendEquation,z.blendSrc,z.blendDst,z.blendEquationAlpha,z.blendSrcAlpha,z.blendDstAlpha,z.blendColor,z.blendAlpha,z.premultipliedAlpha),o.setFunc(z.depthFunc),o.setTest(z.depthTest),o.setMask(z.depthWrite),r.setMask(z.colorWrite);let we=z.stencilWrite;a.setTest(we),we&&(a.setMask(z.stencilWriteMask),a.setFunc(z.stencilFunc,z.stencilRef,z.stencilFuncMask),a.setOp(z.stencilFail,z.stencilZFail,z.stencilZPass)),ct(z.polygonOffset,z.polygonOffsetFactor,z.polygonOffsetUnits),z.alphaToCoverage===!0?X(s.SAMPLE_ALPHA_TO_COVERAGE):he(s.SAMPLE_ALPHA_TO_COVERAGE)}function Re(z){P!==z&&(z?s.frontFace(s.CW):s.frontFace(s.CCW),P=z)}function We(z){z!==sf?(X(s.CULL_FACE),z!==L&&(z===qc?s.cullFace(s.BACK):z===rf?s.cullFace(s.FRONT):s.cullFace(s.FRONT_AND_BACK))):he(s.CULL_FACE),L=z}function lt(z){z!==H&&(F&&s.lineWidth(z),H=z)}function ct(z,Se,le){z?(X(s.POLYGON_OFFSET_FILL),(O!==Se||U!==le)&&(O=Se,U=le,o.getReversed()&&(Se=-Se),s.polygonOffset(Se,le))):he(s.POLYGON_OFFSET_FILL)}function k(z){z?X(s.SCISSOR_TEST):he(s.SCISSOR_TEST)}function ne(z){z===void 0&&(z=s.TEXTURE0+V-1),j!==z&&(s.activeTexture(z),j=z)}function I(z,Se,le){le===void 0&&(j===null?le=s.TEXTURE0+V-1:le=j);let we=me[le];we===void 0&&(we={type:void 0,texture:void 0},me[le]=we),(we.type!==z||we.texture!==Se)&&(j!==le&&(s.activeTexture(le),j=le),s.bindTexture(z,Se||ge[z]),we.type=z,we.texture=Se)}function fe(){let z=me[j];z!==void 0&&z.type!==void 0&&(s.bindTexture(z.type,null),z.type=void 0,z.texture=void 0)}function N(){try{s.compressedTexImage2D(...arguments)}catch(z){Ze("WebGLState:",z)}}function T(){try{s.compressedTexImage3D(...arguments)}catch(z){Ze("WebGLState:",z)}}function x(){try{s.texSubImage2D(...arguments)}catch(z){Ze("WebGLState:",z)}}function B(){try{s.texSubImage3D(...arguments)}catch(z){Ze("WebGLState:",z)}}function G(){try{s.compressedTexSubImage2D(...arguments)}catch(z){Ze("WebGLState:",z)}}function ee(){try{s.compressedTexSubImage3D(...arguments)}catch(z){Ze("WebGLState:",z)}}function pe(){try{s.texStorage2D(...arguments)}catch(z){Ze("WebGLState:",z)}}function be(){try{s.texStorage3D(...arguments)}catch(z){Ze("WebGLState:",z)}}function re(){try{s.texImage2D(...arguments)}catch(z){Ze("WebGLState:",z)}}function ae(){try{s.texImage3D(...arguments)}catch(z){Ze("WebGLState:",z)}}function Te(z){return u[z]!==void 0?u[z]:s.getParameter(z)}function Oe(z,Se){u[z]!==Se&&(s.pixelStorei(z,Se),u[z]=Se)}function Ae(z){it.equals(z)===!1&&(s.scissor(z.x,z.y,z.z,z.w),it.copy(z))}function Ee(z){Fe.equals(z)===!1&&(s.viewport(z.x,z.y,z.z,z.w),Fe.copy(z))}function ze(z,Se){let le=c.get(Se);le===void 0&&(le=new WeakMap,c.set(Se,le));let we=le.get(z);we===void 0&&(we=s.getUniformBlockIndex(Se,z.name),le.set(z,we))}function qe(z,Se){let we=c.get(Se).get(z);l.get(Se)!==we&&(s.uniformBlockBinding(Se,we,z.__bindingPointIndex),l.set(Se,we))}function Ye(){s.disable(s.BLEND),s.disable(s.CULL_FACE),s.disable(s.DEPTH_TEST),s.disable(s.POLYGON_OFFSET_FILL),s.disable(s.SCISSOR_TEST),s.disable(s.STENCIL_TEST),s.disable(s.SAMPLE_ALPHA_TO_COVERAGE),s.blendEquation(s.FUNC_ADD),s.blendFunc(s.ONE,s.ZERO),s.blendFuncSeparate(s.ONE,s.ZERO,s.ONE,s.ZERO),s.blendColor(0,0,0,0),s.colorMask(!0,!0,!0,!0),s.clearColor(0,0,0,0),s.depthMask(!0),s.depthFunc(s.LESS),o.setReversed(!1),s.clearDepth(1),s.stencilMask(4294967295),s.stencilFunc(s.ALWAYS,0,4294967295),s.stencilOp(s.KEEP,s.KEEP,s.KEEP),s.clearStencil(0),s.cullFace(s.BACK),s.frontFace(s.CCW),s.polygonOffset(0,0),s.activeTexture(s.TEXTURE0),s.bindFramebuffer(s.FRAMEBUFFER,null),s.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),s.bindFramebuffer(s.READ_FRAMEBUFFER,null),s.useProgram(null),s.lineWidth(1),s.scissor(0,0,s.canvas.width,s.canvas.height),s.viewport(0,0,s.canvas.width,s.canvas.height),s.pixelStorei(s.PACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,!1),s.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!1),s.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,s.BROWSER_DEFAULT_WEBGL),s.pixelStorei(s.PACK_ROW_LENGTH,0),s.pixelStorei(s.PACK_SKIP_PIXELS,0),s.pixelStorei(s.PACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_ROW_LENGTH,0),s.pixelStorei(s.UNPACK_IMAGE_HEIGHT,0),s.pixelStorei(s.UNPACK_SKIP_PIXELS,0),s.pixelStorei(s.UNPACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_SKIP_IMAGES,0),h={},u={},j=null,me={},f={},d=new WeakMap,g=[],M=null,m=!1,p=null,S=null,R=null,b=null,w=null,y=null,C=null,_=new Me(0,0,0),v=0,E=!1,P=null,L=null,H=null,O=null,U=null,it.set(0,0,s.canvas.width,s.canvas.height),Fe.set(0,0,s.canvas.width,s.canvas.height),r.reset(),o.reset(),a.reset()}return{buffers:{color:r,depth:o,stencil:a},enable:X,disable:he,bindFramebuffer:$,drawBuffers:ie,useProgram:oe,setBlending:ce,setMaterial:ve,setFlipSided:Re,setCullFace:We,setLineWidth:lt,setPolygonOffset:ct,setScissorTest:k,activeTexture:ne,bindTexture:I,unbindTexture:fe,compressedTexImage2D:N,compressedTexImage3D:T,texImage2D:re,texImage3D:ae,pixelStorei:Oe,getParameter:Te,updateUBOMapping:ze,uniformBlockBinding:qe,texStorage2D:pe,texStorage3D:be,texSubImage2D:x,texSubImage3D:B,compressedTexSubImage2D:G,compressedTexSubImage3D:ee,scissor:Ae,viewport:Ee,reset:Ye}}function Zx(s,e,t,n,i,r,o){let a=e.has("WEBGL_multisampled_render_to_texture")?e.get("WEBGL_multisampled_render_to_texture"):null,l=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),c=new ye,h=new WeakMap,u=new Set,f,d=new WeakMap,g=!1;try{g=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function M(T,x){return g?new OffscreenCanvas(T,x):Zs("canvas")}function m(T,x,B){let G=1,ee=N(T);if((ee.width>B||ee.height>B)&&(G=B/Math.max(ee.width,ee.height)),G<1)if(typeof HTMLImageElement<"u"&&T instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&T instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&T instanceof ImageBitmap||typeof VideoFrame<"u"&&T instanceof VideoFrame){let pe=Math.floor(G*ee.width),be=Math.floor(G*ee.height);f===void 0&&(f=M(pe,be));let re=x?M(pe,be):f;return re.width=pe,re.height=be,re.getContext("2d").drawImage(T,0,0,pe,be),Be("WebGLRenderer: Texture has been resized from ("+ee.width+"x"+ee.height+") to ("+pe+"x"+be+")."),re}else return"data"in T&&Be("WebGLRenderer: Image in DataTexture is too big ("+ee.width+"x"+ee.height+")."),T;return T}function p(T){return T.generateMipmaps}function S(T){s.generateMipmap(T)}function R(T){return T.isWebGLCubeRenderTarget?s.TEXTURE_CUBE_MAP:T.isWebGL3DRenderTarget?s.TEXTURE_3D:T.isWebGLArrayRenderTarget||T.isCompressedArrayTexture?s.TEXTURE_2D_ARRAY:s.TEXTURE_2D}function b(T,x,B,G,ee,pe=!1){if(T!==null){if(s[T]!==void 0)return s[T];Be("WebGLRenderer: Attempt to use non-existing WebGL internal format '"+T+"'")}let be;G&&(be=e.get("EXT_texture_norm16"),be||Be("WebGLRenderer: Unable to use normalized textures without EXT_texture_norm16 extension"));let re=x;if(x===s.RED&&(B===s.FLOAT&&(re=s.R32F),B===s.HALF_FLOAT&&(re=s.R16F),B===s.UNSIGNED_BYTE&&(re=s.R8),B===s.UNSIGNED_SHORT&&be&&(re=be.R16_EXT),B===s.SHORT&&be&&(re=be.R16_SNORM_EXT)),x===s.RED_INTEGER&&(B===s.UNSIGNED_BYTE&&(re=s.R8UI),B===s.UNSIGNED_SHORT&&(re=s.R16UI),B===s.UNSIGNED_INT&&(re=s.R32UI),B===s.BYTE&&(re=s.R8I),B===s.SHORT&&(re=s.R16I),B===s.INT&&(re=s.R32I)),x===s.RG&&(B===s.FLOAT&&(re=s.RG32F),B===s.HALF_FLOAT&&(re=s.RG16F),B===s.UNSIGNED_BYTE&&(re=s.RG8),B===s.UNSIGNED_SHORT&&be&&(re=be.RG16_EXT),B===s.SHORT&&be&&(re=be.RG16_SNORM_EXT)),x===s.RG_INTEGER&&(B===s.UNSIGNED_BYTE&&(re=s.RG8UI),B===s.UNSIGNED_SHORT&&(re=s.RG16UI),B===s.UNSIGNED_INT&&(re=s.RG32UI),B===s.BYTE&&(re=s.RG8I),B===s.SHORT&&(re=s.RG16I),B===s.INT&&(re=s.RG32I)),x===s.RGB_INTEGER&&(B===s.UNSIGNED_BYTE&&(re=s.RGB8UI),B===s.UNSIGNED_SHORT&&(re=s.RGB16UI),B===s.UNSIGNED_INT&&(re=s.RGB32UI),B===s.BYTE&&(re=s.RGB8I),B===s.SHORT&&(re=s.RGB16I),B===s.INT&&(re=s.RGB32I)),x===s.RGBA_INTEGER&&(B===s.UNSIGNED_BYTE&&(re=s.RGBA8UI),B===s.UNSIGNED_SHORT&&(re=s.RGBA16UI),B===s.UNSIGNED_INT&&(re=s.RGBA32UI),B===s.BYTE&&(re=s.RGBA8I),B===s.SHORT&&(re=s.RGBA16I),B===s.INT&&(re=s.RGBA32I)),x===s.RGB&&(B===s.UNSIGNED_SHORT&&be&&(re=be.RGB16_EXT),B===s.SHORT&&be&&(re=be.RGB16_SNORM_EXT),B===s.UNSIGNED_INT_5_9_9_9_REV&&(re=s.RGB9_E5),B===s.UNSIGNED_INT_10F_11F_11F_REV&&(re=s.R11F_G11F_B10F)),x===s.RGBA){let ae=pe?kr:et.getTransfer(ee);B===s.FLOAT&&(re=s.RGBA32F),B===s.HALF_FLOAT&&(re=s.RGBA16F),B===s.UNSIGNED_BYTE&&(re=ae===pt?s.SRGB8_ALPHA8:s.RGBA8),B===s.UNSIGNED_SHORT&&be&&(re=be.RGBA16_EXT),B===s.SHORT&&be&&(re=be.RGBA16_SNORM_EXT),B===s.UNSIGNED_SHORT_4_4_4_4&&(re=s.RGBA4),B===s.UNSIGNED_SHORT_5_5_5_1&&(re=s.RGB5_A1)}return(re===s.R16F||re===s.R32F||re===s.RG16F||re===s.RG32F||re===s.RGBA16F||re===s.RGBA32F)&&e.get("EXT_color_buffer_float"),re}function w(T,x){let B;return T?x===null||x===Zn||x===mr?B=s.DEPTH24_STENCIL8:x===En?B=s.DEPTH32F_STENCIL8:x===pr&&(B=s.DEPTH24_STENCIL8,Be("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):x===null||x===Zn||x===mr?B=s.DEPTH_COMPONENT24:x===En?B=s.DEPTH_COMPONENT32F:x===pr&&(B=s.DEPTH_COMPONENT16),B}function y(T,x){return p(T)===!0||T.isFramebufferTexture&&T.minFilter!==Ot&&T.minFilter!==Lt?Math.log2(Math.max(x.width,x.height))+1:T.mipmaps!==void 0&&T.mipmaps.length>0?T.mipmaps.length:T.isCompressedTexture&&Array.isArray(T.image)?x.mipmaps.length:1}function C(T){let x=T.target;x.removeEventListener("dispose",C),v(x),x.isVideoTexture&&h.delete(x),x.isHTMLTexture&&u.delete(x)}function _(T){let x=T.target;x.removeEventListener("dispose",_),P(x)}function v(T){let x=n.get(T);if(x.__webglInit===void 0)return;let B=T.source,G=d.get(B);if(G){let ee=G[x.__cacheKey];ee.usedTimes--,ee.usedTimes===0&&E(T),Object.keys(G).length===0&&d.delete(B)}n.remove(T)}function E(T){let x=n.get(T);s.deleteTexture(x.__webglTexture);let B=T.source,G=d.get(B);delete G[x.__cacheKey],o.memory.textures--}function P(T){let x=n.get(T);if(T.depthTexture&&(T.depthTexture.dispose(),n.remove(T.depthTexture)),T.isWebGLCubeRenderTarget)for(let G=0;G<6;G++){if(Array.isArray(x.__webglFramebuffer[G]))for(let ee=0;ee<x.__webglFramebuffer[G].length;ee++)s.deleteFramebuffer(x.__webglFramebuffer[G][ee]);else s.deleteFramebuffer(x.__webglFramebuffer[G]);x.__webglDepthbuffer&&s.deleteRenderbuffer(x.__webglDepthbuffer[G])}else{if(Array.isArray(x.__webglFramebuffer))for(let G=0;G<x.__webglFramebuffer.length;G++)s.deleteFramebuffer(x.__webglFramebuffer[G]);else s.deleteFramebuffer(x.__webglFramebuffer);if(x.__webglDepthbuffer&&s.deleteRenderbuffer(x.__webglDepthbuffer),x.__webglMultisampledFramebuffer&&s.deleteFramebuffer(x.__webglMultisampledFramebuffer),x.__webglColorRenderbuffer)for(let G=0;G<x.__webglColorRenderbuffer.length;G++)x.__webglColorRenderbuffer[G]&&s.deleteRenderbuffer(x.__webglColorRenderbuffer[G]);x.__webglDepthRenderbuffer&&s.deleteRenderbuffer(x.__webglDepthRenderbuffer)}let B=T.textures;for(let G=0,ee=B.length;G<ee;G++){let pe=n.get(B[G]);pe.__webglTexture&&(s.deleteTexture(pe.__webglTexture),o.memory.textures--),n.remove(B[G])}n.remove(T)}let L=0;function H(){L=0}function O(){return L}function U(T){L=T}function V(){let T=L;return T>=i.maxTextures&&Be("WebGLTextures: Trying to use "+T+" texture units while this GPU supports only "+i.maxTextures),L+=1,T}function F(T){let x=[];return x.push(T.wrapS),x.push(T.wrapT),x.push(T.wrapR||0),x.push(T.magFilter),x.push(T.minFilter),x.push(T.anisotropy),x.push(T.internalFormat),x.push(T.format),x.push(T.type),x.push(T.generateMipmaps),x.push(T.premultiplyAlpha),x.push(T.flipY),x.push(T.unpackAlignment),x.push(T.colorSpace),x.join()}function Y(T,x){let B=n.get(T);if(T.isVideoTexture&&I(T),T.isRenderTargetTexture===!1&&T.isExternalTexture!==!0&&T.version>0&&B.__version!==T.version){let G=T.image;if(G===null)Be("WebGLRenderer: Texture marked for update but no image data found.");else if(G.complete===!1)Be("WebGLRenderer: Texture marked for update but image is incomplete");else{he(B,T,x);return}}else T.isExternalTexture&&(B.__webglTexture=T.sourceTexture?T.sourceTexture:null);t.bindTexture(s.TEXTURE_2D,B.__webglTexture,s.TEXTURE0+x)}function J(T,x){let B=n.get(T);if(T.isRenderTargetTexture===!1&&T.version>0&&B.__version!==T.version){he(B,T,x);return}else T.isExternalTexture&&(B.__webglTexture=T.sourceTexture?T.sourceTexture:null);t.bindTexture(s.TEXTURE_2D_ARRAY,B.__webglTexture,s.TEXTURE0+x)}function j(T,x){let B=n.get(T);if(T.isRenderTargetTexture===!1&&T.version>0&&B.__version!==T.version){he(B,T,x);return}t.bindTexture(s.TEXTURE_3D,B.__webglTexture,s.TEXTURE0+x)}function me(T,x){let B=n.get(T);if(T.isCubeDepthTexture!==!0&&T.version>0&&B.__version!==T.version){$(B,T,x);return}t.bindTexture(s.TEXTURE_CUBE_MAP,B.__webglTexture,s.TEXTURE0+x)}let _e={[Gi]:s.REPEAT,[Dn]:s.CLAMP_TO_EDGE,[qs]:s.MIRRORED_REPEAT},Ve={[Ot]:s.NEAREST,[el]:s.NEAREST_MIPMAP_NEAREST,[Ts]:s.NEAREST_MIPMAP_LINEAR,[Lt]:s.LINEAR,[dr]:s.LINEAR_MIPMAP_NEAREST,[Yn]:s.LINEAR_MIPMAP_LINEAR},it={[Lf]:s.NEVER,[Of]:s.ALWAYS,[Df]:s.LESS,[kl]:s.LEQUAL,[Nf]:s.EQUAL,[Hl]:s.GEQUAL,[Uf]:s.GREATER,[Ff]:s.NOTEQUAL};function Fe(T,x){if(x.type===En&&e.has("OES_texture_float_linear")===!1&&(x.magFilter===Lt||x.magFilter===dr||x.magFilter===Ts||x.magFilter===Yn||x.minFilter===Lt||x.minFilter===dr||x.minFilter===Ts||x.minFilter===Yn)&&Be("WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),s.texParameteri(T,s.TEXTURE_WRAP_S,_e[x.wrapS]),s.texParameteri(T,s.TEXTURE_WRAP_T,_e[x.wrapT]),(T===s.TEXTURE_3D||T===s.TEXTURE_2D_ARRAY)&&s.texParameteri(T,s.TEXTURE_WRAP_R,_e[x.wrapR]),s.texParameteri(T,s.TEXTURE_MAG_FILTER,Ve[x.magFilter]),s.texParameteri(T,s.TEXTURE_MIN_FILTER,Ve[x.minFilter]),x.compareFunction&&(s.texParameteri(T,s.TEXTURE_COMPARE_MODE,s.COMPARE_REF_TO_TEXTURE),s.texParameteri(T,s.TEXTURE_COMPARE_FUNC,it[x.compareFunction])),e.has("EXT_texture_filter_anisotropic")===!0){if(x.magFilter===Ot||x.minFilter!==Ts&&x.minFilter!==Yn||x.type===En&&e.has("OES_texture_float_linear")===!1)return;if(x.anisotropy>1||n.get(x).__currentAnisotropy){let B=e.get("EXT_texture_filter_anisotropic");s.texParameterf(T,B.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(x.anisotropy,i.getMaxAnisotropy())),n.get(x).__currentAnisotropy=x.anisotropy}}}function Q(T,x){let B=!1;T.__webglInit===void 0&&(T.__webglInit=!0,x.addEventListener("dispose",C));let G=x.source,ee=d.get(G);ee===void 0&&(ee={},d.set(G,ee));let pe=F(x);if(pe!==T.__cacheKey){ee[pe]===void 0&&(ee[pe]={texture:s.createTexture(),usedTimes:0},o.memory.textures++,B=!0),ee[pe].usedTimes++;let be=ee[T.__cacheKey];be!==void 0&&(ee[T.__cacheKey].usedTimes--,be.usedTimes===0&&E(x)),T.__cacheKey=pe,T.__webglTexture=ee[pe].texture}return B}function ge(T,x,B){return Math.floor(Math.floor(T/B)/x)}function X(T,x,B,G){let pe=T.updateRanges;if(pe.length===0)t.texSubImage2D(s.TEXTURE_2D,0,0,0,x.width,x.height,B,G,x.data);else{pe.sort((Oe,Ae)=>Oe.start-Ae.start);let be=0;for(let Oe=1;Oe<pe.length;Oe++){let Ae=pe[be],Ee=pe[Oe],ze=Ae.start+Ae.count,qe=ge(Ee.start,x.width,4),Ye=ge(Ae.start,x.width,4);Ee.start<=ze+1&&qe===Ye&&ge(Ee.start+Ee.count-1,x.width,4)===qe?Ae.count=Math.max(Ae.count,Ee.start+Ee.count-Ae.start):(++be,pe[be]=Ee)}pe.length=be+1;let re=t.getParameter(s.UNPACK_ROW_LENGTH),ae=t.getParameter(s.UNPACK_SKIP_PIXELS),Te=t.getParameter(s.UNPACK_SKIP_ROWS);t.pixelStorei(s.UNPACK_ROW_LENGTH,x.width);for(let Oe=0,Ae=pe.length;Oe<Ae;Oe++){let Ee=pe[Oe],ze=Math.floor(Ee.start/4),qe=Math.ceil(Ee.count/4),Ye=ze%x.width,z=Math.floor(ze/x.width),Se=qe,le=1;t.pixelStorei(s.UNPACK_SKIP_PIXELS,Ye),t.pixelStorei(s.UNPACK_SKIP_ROWS,z),t.texSubImage2D(s.TEXTURE_2D,0,Ye,z,Se,le,B,G,x.data)}T.clearUpdateRanges(),t.pixelStorei(s.UNPACK_ROW_LENGTH,re),t.pixelStorei(s.UNPACK_SKIP_PIXELS,ae),t.pixelStorei(s.UNPACK_SKIP_ROWS,Te)}}function he(T,x,B){let G=s.TEXTURE_2D;(x.isDataArrayTexture||x.isCompressedArrayTexture)&&(G=s.TEXTURE_2D_ARRAY),x.isData3DTexture&&(G=s.TEXTURE_3D);let ee=Q(T,x),pe=x.source;t.bindTexture(G,T.__webglTexture,s.TEXTURE0+B);let be=n.get(pe);if(pe.version!==be.__version||ee===!0){if(t.activeTexture(s.TEXTURE0+B),(typeof ImageBitmap<"u"&&x.image instanceof ImageBitmap)===!1){let le=et.getPrimaries(et.workingColorSpace),we=x.colorSpace===Ci?null:et.getPrimaries(x.colorSpace),Pe=x.colorSpace===Ci||le===we?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,x.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,x.premultiplyAlpha),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,Pe)}t.pixelStorei(s.UNPACK_ALIGNMENT,x.unpackAlignment);let ae=m(x.image,!1,i.maxTextureSize);ae=fe(x,ae);let Te=r.convert(x.format,x.colorSpace),Oe=r.convert(x.type),Ae=b(x.internalFormat,Te,Oe,x.normalized,x.colorSpace,x.isVideoTexture);Fe(G,x);let Ee,ze=x.mipmaps,qe=x.isVideoTexture!==!0,Ye=be.__version===void 0||ee===!0,z=pe.dataReady,Se=y(x,ae);if(x.isDepthTexture)Ae=w(x.format===ji,x.type),Ye&&(qe?t.texStorage2D(s.TEXTURE_2D,1,Ae,ae.width,ae.height):t.texImage2D(s.TEXTURE_2D,0,Ae,ae.width,ae.height,0,Te,Oe,null));else if(x.isDataTexture)if(ze.length>0){qe&&Ye&&t.texStorage2D(s.TEXTURE_2D,Se,Ae,ze[0].width,ze[0].height);for(let le=0,we=ze.length;le<we;le++)Ee=ze[le],qe?z&&t.texSubImage2D(s.TEXTURE_2D,le,0,0,Ee.width,Ee.height,Te,Oe,Ee.data):t.texImage2D(s.TEXTURE_2D,le,Ae,Ee.width,Ee.height,0,Te,Oe,Ee.data);x.generateMipmaps=!1}else qe?(Ye&&t.texStorage2D(s.TEXTURE_2D,Se,Ae,ae.width,ae.height),z&&X(x,ae,Te,Oe)):t.texImage2D(s.TEXTURE_2D,0,Ae,ae.width,ae.height,0,Te,Oe,ae.data);else if(x.isCompressedTexture)if(x.isCompressedArrayTexture){qe&&Ye&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Se,Ae,ze[0].width,ze[0].height,ae.depth);for(let le=0,we=ze.length;le<we;le++)if(Ee=ze[le],x.format!==wn)if(Te!==null)if(qe){if(z)if(x.layerUpdates.size>0){let Pe=ch(Ee.width,Ee.height,x.format,x.type);for(let de of x.layerUpdates){let ke=Ee.data.subarray(de*Pe/Ee.data.BYTES_PER_ELEMENT,(de+1)*Pe/Ee.data.BYTES_PER_ELEMENT);t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,le,0,0,de,Ee.width,Ee.height,1,Te,ke)}x.clearLayerUpdates()}else t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,le,0,0,0,Ee.width,Ee.height,ae.depth,Te,Ee.data)}else t.compressedTexImage3D(s.TEXTURE_2D_ARRAY,le,Ae,Ee.width,Ee.height,ae.depth,0,Ee.data,0,0);else Be("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else qe?z&&t.texSubImage3D(s.TEXTURE_2D_ARRAY,le,0,0,0,Ee.width,Ee.height,ae.depth,Te,Oe,Ee.data):t.texImage3D(s.TEXTURE_2D_ARRAY,le,Ae,Ee.width,Ee.height,ae.depth,0,Te,Oe,Ee.data)}else{qe&&Ye&&t.texStorage2D(s.TEXTURE_2D,Se,Ae,ze[0].width,ze[0].height);for(let le=0,we=ze.length;le<we;le++)Ee=ze[le],x.format!==wn?Te!==null?qe?z&&t.compressedTexSubImage2D(s.TEXTURE_2D,le,0,0,Ee.width,Ee.height,Te,Ee.data):t.compressedTexImage2D(s.TEXTURE_2D,le,Ae,Ee.width,Ee.height,0,Ee.data):Be("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):qe?z&&t.texSubImage2D(s.TEXTURE_2D,le,0,0,Ee.width,Ee.height,Te,Oe,Ee.data):t.texImage2D(s.TEXTURE_2D,le,Ae,Ee.width,Ee.height,0,Te,Oe,Ee.data)}else if(x.isDataArrayTexture)if(qe){if(Ye&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Se,Ae,ae.width,ae.height,ae.depth),z)if(x.layerUpdates.size>0){let le=ch(ae.width,ae.height,x.format,x.type);for(let we of x.layerUpdates){let Pe=ae.data.subarray(we*le/ae.data.BYTES_PER_ELEMENT,(we+1)*le/ae.data.BYTES_PER_ELEMENT);t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,we,ae.width,ae.height,1,Te,Oe,Pe)}x.clearLayerUpdates()}else t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,0,ae.width,ae.height,ae.depth,Te,Oe,ae.data)}else t.texImage3D(s.TEXTURE_2D_ARRAY,0,Ae,ae.width,ae.height,ae.depth,0,Te,Oe,ae.data);else if(x.isData3DTexture)qe?(Ye&&t.texStorage3D(s.TEXTURE_3D,Se,Ae,ae.width,ae.height,ae.depth),z&&t.texSubImage3D(s.TEXTURE_3D,0,0,0,0,ae.width,ae.height,ae.depth,Te,Oe,ae.data)):t.texImage3D(s.TEXTURE_3D,0,Ae,ae.width,ae.height,ae.depth,0,Te,Oe,ae.data);else if(x.isFramebufferTexture){if(Ye)if(qe)t.texStorage2D(s.TEXTURE_2D,Se,Ae,ae.width,ae.height);else{let le=ae.width,we=ae.height;for(let Pe=0;Pe<Se;Pe++)t.texImage2D(s.TEXTURE_2D,Pe,Ae,le,we,0,Te,Oe,null),le>>=1,we>>=1}}else if(x.isHTMLTexture){if("texElementImage2D"in s){let le=s.canvas;if(le.hasAttribute("layoutsubtree")||le.setAttribute("layoutsubtree","true"),ae.parentNode!==le){le.appendChild(ae),u.add(x),le.onpaint=we=>{let Pe=we.changedElements;for(let de of u)Pe.includes(de.image)&&(de.needsUpdate=!0)},le.requestPaint();return}if(s.texElementImage2D.length===3)s.texElementImage2D(s.TEXTURE_2D,s.RGBA8,ae);else{let Pe=s.RGBA,de=s.RGBA,ke=s.UNSIGNED_BYTE;s.texElementImage2D(s.TEXTURE_2D,0,Pe,de,ke,ae)}s.texParameteri(s.TEXTURE_2D,s.TEXTURE_MIN_FILTER,s.LINEAR),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_S,s.CLAMP_TO_EDGE),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_T,s.CLAMP_TO_EDGE)}}else if(ze.length>0){if(qe&&Ye){let le=N(ze[0]);t.texStorage2D(s.TEXTURE_2D,Se,Ae,le.width,le.height)}for(let le=0,we=ze.length;le<we;le++)Ee=ze[le],qe?z&&t.texSubImage2D(s.TEXTURE_2D,le,0,0,Te,Oe,Ee):t.texImage2D(s.TEXTURE_2D,le,Ae,Te,Oe,Ee);x.generateMipmaps=!1}else if(qe){if(Ye){let le=N(ae);t.texStorage2D(s.TEXTURE_2D,Se,Ae,le.width,le.height)}z&&t.texSubImage2D(s.TEXTURE_2D,0,0,0,Te,Oe,ae)}else t.texImage2D(s.TEXTURE_2D,0,Ae,Te,Oe,ae);p(x)&&S(G),be.__version=pe.version,x.onUpdate&&x.onUpdate(x)}T.__version=x.version}function $(T,x,B){if(x.image.length!==6)return;let G=Q(T,x),ee=x.source;t.bindTexture(s.TEXTURE_CUBE_MAP,T.__webglTexture,s.TEXTURE0+B);let pe=n.get(ee);if(ee.version!==pe.__version||G===!0){t.activeTexture(s.TEXTURE0+B);let be=et.getPrimaries(et.workingColorSpace),re=x.colorSpace===Ci?null:et.getPrimaries(x.colorSpace),ae=x.colorSpace===Ci||be===re?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,x.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,x.premultiplyAlpha),t.pixelStorei(s.UNPACK_ALIGNMENT,x.unpackAlignment),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,ae);let Te=x.isCompressedTexture||x.image[0].isCompressedTexture,Oe=x.image[0]&&x.image[0].isDataTexture,Ae=[];for(let de=0;de<6;de++)!Te&&!Oe?Ae[de]=m(x.image[de],!0,i.maxCubemapSize):Ae[de]=Oe?x.image[de].image:x.image[de],Ae[de]=fe(x,Ae[de]);let Ee=Ae[0],ze=r.convert(x.format,x.colorSpace),qe=r.convert(x.type),Ye=b(x.internalFormat,ze,qe,x.normalized,x.colorSpace),z=x.isVideoTexture!==!0,Se=pe.__version===void 0||G===!0,le=ee.dataReady,we=y(x,Ee);Fe(s.TEXTURE_CUBE_MAP,x);let Pe;if(Te){z&&Se&&t.texStorage2D(s.TEXTURE_CUBE_MAP,we,Ye,Ee.width,Ee.height);for(let de=0;de<6;de++){Pe=Ae[de].mipmaps;for(let ke=0;ke<Pe.length;ke++){let De=Pe[ke];x.format!==wn?ze!==null?z?le&&t.compressedTexSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,ke,0,0,De.width,De.height,ze,De.data):t.compressedTexImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,ke,Ye,De.width,De.height,0,De.data):Be("WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):z?le&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,ke,0,0,De.width,De.height,ze,qe,De.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,ke,Ye,De.width,De.height,0,ze,qe,De.data)}}}else{if(Pe=x.mipmaps,z&&Se){Pe.length>0&&we++;let de=N(Ae[0]);t.texStorage2D(s.TEXTURE_CUBE_MAP,we,Ye,de.width,de.height)}for(let de=0;de<6;de++)if(Oe){z?le&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,0,0,0,Ae[de].width,Ae[de].height,ze,qe,Ae[de].data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,0,Ye,Ae[de].width,Ae[de].height,0,ze,qe,Ae[de].data);for(let ke=0;ke<Pe.length;ke++){let q=Pe[ke].image[de].image;z?le&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,ke+1,0,0,q.width,q.height,ze,qe,q.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,ke+1,Ye,q.width,q.height,0,ze,qe,q.data)}}else{z?le&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,0,0,0,ze,qe,Ae[de]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,0,Ye,ze,qe,Ae[de]);for(let ke=0;ke<Pe.length;ke++){let De=Pe[ke];z?le&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,ke+1,0,0,ze,qe,De.image[de]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+de,ke+1,Ye,ze,qe,De.image[de])}}}p(x)&&S(s.TEXTURE_CUBE_MAP),pe.__version=ee.version,x.onUpdate&&x.onUpdate(x)}T.__version=x.version}function ie(T,x,B,G,ee,pe){let be=r.convert(B.format,B.colorSpace),re=r.convert(B.type),ae=b(B.internalFormat,be,re,B.normalized,B.colorSpace),Te=n.get(x),Oe=n.get(B);if(Oe.__renderTarget=x,!Te.__hasExternalTextures){let Ae=Math.max(1,x.width>>pe),Ee=Math.max(1,x.height>>pe);ee===s.TEXTURE_3D||ee===s.TEXTURE_2D_ARRAY?t.texImage3D(ee,pe,ae,Ae,Ee,x.depth,0,be,re,null):t.texImage2D(ee,pe,ae,Ae,Ee,0,be,re,null)}t.bindFramebuffer(s.FRAMEBUFFER,T),ne(x)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,G,ee,Oe.__webglTexture,0,k(x)):(ee===s.TEXTURE_2D||ee>=s.TEXTURE_CUBE_MAP_POSITIVE_X&&ee<=s.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&s.framebufferTexture2D(s.FRAMEBUFFER,G,ee,Oe.__webglTexture,pe),t.bindFramebuffer(s.FRAMEBUFFER,null)}function oe(T,x,B){if(s.bindRenderbuffer(s.RENDERBUFFER,T),x.depthBuffer){let G=x.depthTexture,ee=G&&G.isDepthTexture?G.type:null,pe=w(x.stencilBuffer,ee),be=x.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;ne(x)?a.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,k(x),pe,x.width,x.height):B?s.renderbufferStorageMultisample(s.RENDERBUFFER,k(x),pe,x.width,x.height):s.renderbufferStorage(s.RENDERBUFFER,pe,x.width,x.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,be,s.RENDERBUFFER,T)}else{let G=x.textures;for(let ee=0;ee<G.length;ee++){let pe=G[ee],be=r.convert(pe.format,pe.colorSpace),re=r.convert(pe.type),ae=b(pe.internalFormat,be,re,pe.normalized,pe.colorSpace);ne(x)?a.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,k(x),ae,x.width,x.height):B?s.renderbufferStorageMultisample(s.RENDERBUFFER,k(x),ae,x.width,x.height):s.renderbufferStorage(s.RENDERBUFFER,ae,x.width,x.height)}}s.bindRenderbuffer(s.RENDERBUFFER,null)}function ue(T,x,B){let G=x.isWebGLCubeRenderTarget===!0;if(t.bindFramebuffer(s.FRAMEBUFFER,T),!(x.depthTexture&&x.depthTexture.isDepthTexture))throw new Error("THREE.WebGLTextures: renderTarget.depthTexture must be an instance of THREE.DepthTexture.");let ee=n.get(x.depthTexture);if(ee.__renderTarget=x,(!ee.__webglTexture||x.depthTexture.image.width!==x.width||x.depthTexture.image.height!==x.height)&&(x.depthTexture.image.width=x.width,x.depthTexture.image.height=x.height,x.depthTexture.needsUpdate=!0),G){if(ee.__webglInit===void 0&&(ee.__webglInit=!0,x.depthTexture.addEventListener("dispose",C)),ee.__webglTexture===void 0){ee.__webglTexture=s.createTexture(),t.bindTexture(s.TEXTURE_CUBE_MAP,ee.__webglTexture),Fe(s.TEXTURE_CUBE_MAP,x.depthTexture);let Te=r.convert(x.depthTexture.format),Oe=r.convert(x.depthTexture.type),Ae;x.depthTexture.format===ti?Ae=s.DEPTH_COMPONENT24:x.depthTexture.format===ji&&(Ae=s.DEPTH24_STENCIL8);for(let Ee=0;Ee<6;Ee++)s.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Ee,0,Ae,x.width,x.height,0,Te,Oe,null)}}else Y(x.depthTexture,0);let pe=ee.__webglTexture,be=k(x),re=G?s.TEXTURE_CUBE_MAP_POSITIVE_X+B:s.TEXTURE_2D,ae=x.depthTexture.format===ji?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;if(x.depthTexture.format===ti)ne(x)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,ae,re,pe,0,be):s.framebufferTexture2D(s.FRAMEBUFFER,ae,re,pe,0);else if(x.depthTexture.format===ji)ne(x)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,ae,re,pe,0,be):s.framebufferTexture2D(s.FRAMEBUFFER,ae,re,pe,0);else throw new Error("THREE.WebGLTextures: Unknown depthTexture format.")}function se(T){let x=n.get(T),B=T.isWebGLCubeRenderTarget===!0;if(x.__boundDepthTexture!==T.depthTexture){let G=T.depthTexture;if(x.__depthDisposeCallback&&x.__depthDisposeCallback(),G){let ee=()=>{delete x.__boundDepthTexture,delete x.__depthDisposeCallback,G.removeEventListener("dispose",ee)};G.addEventListener("dispose",ee),x.__depthDisposeCallback=ee}x.__boundDepthTexture=G}if(T.depthTexture&&!x.__autoAllocateDepthBuffer)if(B)for(let G=0;G<6;G++)ue(x.__webglFramebuffer[G],T,G);else{let G=T.texture.mipmaps;G&&G.length>0?ue(x.__webglFramebuffer[0],T,0):ue(x.__webglFramebuffer,T,0)}else if(B){x.__webglDepthbuffer=[];for(let G=0;G<6;G++)if(t.bindFramebuffer(s.FRAMEBUFFER,x.__webglFramebuffer[G]),x.__webglDepthbuffer[G]===void 0)x.__webglDepthbuffer[G]=s.createRenderbuffer(),oe(x.__webglDepthbuffer[G],T,!1);else{let ee=T.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,pe=x.__webglDepthbuffer[G];s.bindRenderbuffer(s.RENDERBUFFER,pe),s.framebufferRenderbuffer(s.FRAMEBUFFER,ee,s.RENDERBUFFER,pe)}}else{let G=T.texture.mipmaps;if(G&&G.length>0?t.bindFramebuffer(s.FRAMEBUFFER,x.__webglFramebuffer[0]):t.bindFramebuffer(s.FRAMEBUFFER,x.__webglFramebuffer),x.__webglDepthbuffer===void 0)x.__webglDepthbuffer=s.createRenderbuffer(),oe(x.__webglDepthbuffer,T,!1);else{let ee=T.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,pe=x.__webglDepthbuffer;s.bindRenderbuffer(s.RENDERBUFFER,pe),s.framebufferRenderbuffer(s.FRAMEBUFFER,ee,s.RENDERBUFFER,pe)}}t.bindFramebuffer(s.FRAMEBUFFER,null)}function ce(T,x,B){let G=n.get(T);x!==void 0&&ie(G.__webglFramebuffer,T,T.texture,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,0),B!==void 0&&se(T)}function ve(T){let x=T.texture,B=n.get(T),G=n.get(x);T.addEventListener("dispose",_);let ee=T.textures,pe=T.isWebGLCubeRenderTarget===!0,be=ee.length>1;if(be||(G.__webglTexture===void 0&&(G.__webglTexture=s.createTexture()),G.__version=x.version,o.memory.textures++),pe){B.__webglFramebuffer=[];for(let re=0;re<6;re++)if(x.mipmaps&&x.mipmaps.length>0){B.__webglFramebuffer[re]=[];for(let ae=0;ae<x.mipmaps.length;ae++)B.__webglFramebuffer[re][ae]=s.createFramebuffer()}else B.__webglFramebuffer[re]=s.createFramebuffer()}else{if(x.mipmaps&&x.mipmaps.length>0){B.__webglFramebuffer=[];for(let re=0;re<x.mipmaps.length;re++)B.__webglFramebuffer[re]=s.createFramebuffer()}else B.__webglFramebuffer=s.createFramebuffer();if(be)for(let re=0,ae=ee.length;re<ae;re++){let Te=n.get(ee[re]);Te.__webglTexture===void 0&&(Te.__webglTexture=s.createTexture(),o.memory.textures++)}if(T.samples>0&&ne(T)===!1){B.__webglMultisampledFramebuffer=s.createFramebuffer(),B.__webglColorRenderbuffer=[],t.bindFramebuffer(s.FRAMEBUFFER,B.__webglMultisampledFramebuffer);for(let re=0;re<ee.length;re++){let ae=ee[re];B.__webglColorRenderbuffer[re]=s.createRenderbuffer(),s.bindRenderbuffer(s.RENDERBUFFER,B.__webglColorRenderbuffer[re]);let Te=r.convert(ae.format,ae.colorSpace),Oe=r.convert(ae.type),Ae=b(ae.internalFormat,Te,Oe,ae.normalized,ae.colorSpace,T.isXRRenderTarget===!0),Ee=k(T);s.renderbufferStorageMultisample(s.RENDERBUFFER,Ee,Ae,T.width,T.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+re,s.RENDERBUFFER,B.__webglColorRenderbuffer[re])}s.bindRenderbuffer(s.RENDERBUFFER,null),T.depthBuffer&&(B.__webglDepthRenderbuffer=s.createRenderbuffer(),oe(B.__webglDepthRenderbuffer,T,!0)),t.bindFramebuffer(s.FRAMEBUFFER,null)}}if(pe){t.bindTexture(s.TEXTURE_CUBE_MAP,G.__webglTexture),Fe(s.TEXTURE_CUBE_MAP,x);for(let re=0;re<6;re++)if(x.mipmaps&&x.mipmaps.length>0)for(let ae=0;ae<x.mipmaps.length;ae++)ie(B.__webglFramebuffer[re][ae],T,x,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+re,ae);else ie(B.__webglFramebuffer[re],T,x,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+re,0);p(x)&&S(s.TEXTURE_CUBE_MAP),t.unbindTexture()}else if(be){for(let re=0,ae=ee.length;re<ae;re++){let Te=ee[re],Oe=n.get(Te),Ae=s.TEXTURE_2D;(T.isWebGL3DRenderTarget||T.isWebGLArrayRenderTarget)&&(Ae=T.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(Ae,Oe.__webglTexture),Fe(Ae,Te),ie(B.__webglFramebuffer,T,Te,s.COLOR_ATTACHMENT0+re,Ae,0),p(Te)&&S(Ae)}t.unbindTexture()}else{let re=s.TEXTURE_2D;if((T.isWebGL3DRenderTarget||T.isWebGLArrayRenderTarget)&&(re=T.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(re,G.__webglTexture),Fe(re,x),x.mipmaps&&x.mipmaps.length>0)for(let ae=0;ae<x.mipmaps.length;ae++)ie(B.__webglFramebuffer[ae],T,x,s.COLOR_ATTACHMENT0,re,ae);else ie(B.__webglFramebuffer,T,x,s.COLOR_ATTACHMENT0,re,0);p(x)&&S(re),t.unbindTexture()}T.depthBuffer&&se(T)}function Re(T){let x=T.textures;for(let B=0,G=x.length;B<G;B++){let ee=x[B];if(p(ee)){let pe=R(T),be=n.get(ee).__webglTexture;t.bindTexture(pe,be),S(pe),t.unbindTexture()}}}let We=[],lt=[];function ct(T){if(T.samples>0){if(ne(T)===!1){let x=T.textures,B=T.width,G=T.height,ee=s.COLOR_BUFFER_BIT,pe=T.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,be=n.get(T),re=x.length>1;if(re)for(let Te=0;Te<x.length;Te++)t.bindFramebuffer(s.FRAMEBUFFER,be.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+Te,s.RENDERBUFFER,null),t.bindFramebuffer(s.FRAMEBUFFER,be.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+Te,s.TEXTURE_2D,null,0);t.bindFramebuffer(s.READ_FRAMEBUFFER,be.__webglMultisampledFramebuffer);let ae=T.texture.mipmaps;ae&&ae.length>0?t.bindFramebuffer(s.DRAW_FRAMEBUFFER,be.__webglFramebuffer[0]):t.bindFramebuffer(s.DRAW_FRAMEBUFFER,be.__webglFramebuffer);for(let Te=0;Te<x.length;Te++){if(T.resolveDepthBuffer&&(T.depthBuffer&&(ee|=s.DEPTH_BUFFER_BIT),T.stencilBuffer&&T.resolveStencilBuffer&&(ee|=s.STENCIL_BUFFER_BIT)),re){s.framebufferRenderbuffer(s.READ_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.RENDERBUFFER,be.__webglColorRenderbuffer[Te]);let Oe=n.get(x[Te]).__webglTexture;s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,Oe,0)}s.blitFramebuffer(0,0,B,G,0,0,B,G,ee,s.NEAREST),l===!0&&(We.length=0,lt.length=0,We.push(s.COLOR_ATTACHMENT0+Te),T.depthBuffer&&T.resolveDepthBuffer===!1&&(We.push(pe),lt.push(pe),s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,lt)),s.invalidateFramebuffer(s.READ_FRAMEBUFFER,We))}if(t.bindFramebuffer(s.READ_FRAMEBUFFER,null),t.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),re)for(let Te=0;Te<x.length;Te++){t.bindFramebuffer(s.FRAMEBUFFER,be.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+Te,s.RENDERBUFFER,be.__webglColorRenderbuffer[Te]);let Oe=n.get(x[Te]).__webglTexture;t.bindFramebuffer(s.FRAMEBUFFER,be.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+Te,s.TEXTURE_2D,Oe,0)}t.bindFramebuffer(s.DRAW_FRAMEBUFFER,be.__webglMultisampledFramebuffer)}else if(T.depthBuffer&&T.resolveDepthBuffer===!1&&l){let x=T.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,[x])}}}function k(T){return Math.min(i.maxSamples,T.samples)}function ne(T){let x=n.get(T);return T.samples>0&&e.has("WEBGL_multisampled_render_to_texture")===!0&&x.__useRenderToTexture!==!1}function I(T){let x=o.render.frame;h.get(T)!==x&&(h.set(T,x),T.update())}function fe(T,x){let B=T.colorSpace,G=T.format,ee=T.type;return T.isCompressedTexture===!0||T.isVideoTexture===!0||B!==cn&&B!==Ci&&(et.getTransfer(B)===pt?(G!==wn||ee!==gn)&&Be("WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):Ze("WebGLTextures: Unsupported texture color space:",B)),x}function N(T){return typeof HTMLImageElement<"u"&&T instanceof HTMLImageElement?(c.width=T.naturalWidth||T.width,c.height=T.naturalHeight||T.height):typeof VideoFrame<"u"&&T instanceof VideoFrame?(c.width=T.displayWidth,c.height=T.displayHeight):(c.width=T.width,c.height=T.height),c}this.allocateTextureUnit=V,this.resetTextureUnits=H,this.getTextureUnits=O,this.setTextureUnits=U,this.setTexture2D=Y,this.setTexture2DArray=J,this.setTexture3D=j,this.setTextureCube=me,this.rebindTextures=ce,this.setupRenderTarget=ve,this.updateRenderTargetMipmap=Re,this.updateMultisampleRenderTarget=ct,this.setupDepthRenderbuffer=se,this.setupFrameBufferTexture=ie,this.useMultisampledRTT=ne,this.isReversedDepthBuffer=function(){return t.buffers.depth.getReversed()}}function Kx(s,e){function t(n,i=Ci){let r,o=et.getTransfer(i);if(n===gn)return s.UNSIGNED_BYTE;if(n===nl)return s.UNSIGNED_SHORT_4_4_4_4;if(n===il)return s.UNSIGNED_SHORT_5_5_5_1;if(n===$c)return s.UNSIGNED_INT_5_9_9_9_REV;if(n===Qc)return s.UNSIGNED_INT_10F_11F_11F_REV;if(n===Jc)return s.BYTE;if(n===jc)return s.SHORT;if(n===pr)return s.UNSIGNED_SHORT;if(n===tl)return s.INT;if(n===Zn)return s.UNSIGNED_INT;if(n===En)return s.FLOAT;if(n===Xt)return s.HALF_FLOAT;if(n===eh)return s.ALPHA;if(n===th)return s.RGB;if(n===wn)return s.RGBA;if(n===ti)return s.DEPTH_COMPONENT;if(n===ji)return s.DEPTH_STENCIL;if(n===sl)return s.RED;if(n===rl)return s.RED_INTEGER;if(n===$i)return s.RG;if(n===ol)return s.RG_INTEGER;if(n===al)return s.RGBA_INTEGER;if(n===So||n===To||n===Eo||n===wo)if(o===pt)if(r=e.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(n===So)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(n===To)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(n===Eo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(n===wo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=e.get("WEBGL_compressed_texture_s3tc"),r!==null){if(n===So)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(n===To)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(n===Eo)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(n===wo)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(n===ll||n===cl||n===hl||n===ul)if(r=e.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(n===ll)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(n===cl)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(n===hl)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(n===ul)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(n===fl||n===dl||n===pl||n===ml||n===gl||n===Ao||n===_l)if(r=e.get("WEBGL_compressed_texture_etc"),r!==null){if(n===fl||n===dl)return o===pt?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(n===pl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC;if(n===ml)return r.COMPRESSED_R11_EAC;if(n===gl)return r.COMPRESSED_SIGNED_R11_EAC;if(n===Ao)return r.COMPRESSED_RG11_EAC;if(n===_l)return r.COMPRESSED_SIGNED_RG11_EAC}else return null;if(n===xl||n===vl||n===yl||n===Ml||n===bl||n===Sl||n===Tl||n===El||n===wl||n===Al||n===Rl||n===Cl||n===Pl||n===Il)if(r=e.get("WEBGL_compressed_texture_astc"),r!==null){if(n===xl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(n===vl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(n===yl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(n===Ml)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(n===bl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(n===Sl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(n===Tl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(n===El)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(n===wl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(n===Al)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(n===Rl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(n===Cl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(n===Pl)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(n===Il)return o===pt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(n===Ll||n===Dl||n===Nl)if(r=e.get("EXT_texture_compression_bptc"),r!==null){if(n===Ll)return o===pt?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(n===Dl)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(n===Nl)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(n===Ul||n===Fl||n===Ro||n===Ol)if(r=e.get("EXT_texture_compression_rgtc"),r!==null){if(n===Ul)return r.COMPRESSED_RED_RGTC1_EXT;if(n===Fl)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(n===Ro)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(n===Ol)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return n===mr?s.UNSIGNED_INT_24_8:s[n]!==void 0?s[n]:null}return{convert:t}}var Jx=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,jx=`
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

}`,Sh=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(e,t){if(this.texture===null){let n=new Jr(e.texture);(e.depthNear!==t.depthNear||e.depthFar!==t.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=n}}getMesh(e){if(this.texture!==null&&this.mesh===null){let t=e.cameras[0].viewport,n=new mt({vertexShader:Jx,fragmentShader:jx,uniforms:{depthColor:{value:this.texture},depthWidth:{value:t.z},depthHeight:{value:t.w}}});this.mesh=new Je(new sn(20,20),n)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},Th=class extends Nn{constructor(e,t){super();let n=this,i=null,r=1,o=null,a="local-floor",l=1,c=null,h=null,u=null,f=null,d=null,g=null,M=typeof XRWebGLBinding<"u",m=new Sh,p={},S=t.getContextAttributes(),R=null,b=null,w=[],y=[],C=new ye,_=null,v=new Ht;v.viewport=new xt;let E=new Ht;E.viewport=new xt;let P=[v,E],L=new Ya,H=null,O=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(Q){let ge=w[Q];return ge===void 0&&(ge=new $s,w[Q]=ge),ge.getTargetRaySpace()},this.getControllerGrip=function(Q){let ge=w[Q];return ge===void 0&&(ge=new $s,w[Q]=ge),ge.getGripSpace()},this.getHand=function(Q){let ge=w[Q];return ge===void 0&&(ge=new $s,w[Q]=ge),ge.getHandSpace()};function U(Q){let ge=y.indexOf(Q.inputSource);if(ge===-1)return;let X=w[ge];X!==void 0&&(X.update(Q.inputSource,Q.frame,c||o),X.dispatchEvent({type:Q.type,data:Q.inputSource}))}function V(){i.removeEventListener("select",U),i.removeEventListener("selectstart",U),i.removeEventListener("selectend",U),i.removeEventListener("squeeze",U),i.removeEventListener("squeezestart",U),i.removeEventListener("squeezeend",U),i.removeEventListener("end",V),i.removeEventListener("inputsourceschange",F);for(let Q=0;Q<w.length;Q++){let ge=y[Q];ge!==null&&(y[Q]=null,w[Q].disconnect(ge))}H=null,O=null,m.reset();for(let Q in p)delete p[Q];e.setRenderTarget(R),d=null,f=null,u=null,i=null,b=null,Fe.stop(),n.isPresenting=!1,e.setPixelRatio(_),e.setSize(C.width,C.height,!1),n.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(Q){r=Q,n.isPresenting===!0&&Be("WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(Q){a=Q,n.isPresenting===!0&&Be("WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return c||o},this.setReferenceSpace=function(Q){c=Q},this.getBaseLayer=function(){return f!==null?f:d},this.getBinding=function(){return u===null&&M&&(u=new XRWebGLBinding(i,t)),u},this.getFrame=function(){return g},this.getSession=function(){return i},this.setSession=async function(Q){if(i=Q,i!==null){if(R=e.getRenderTarget(),i.addEventListener("select",U),i.addEventListener("selectstart",U),i.addEventListener("selectend",U),i.addEventListener("squeeze",U),i.addEventListener("squeezestart",U),i.addEventListener("squeezeend",U),i.addEventListener("end",V),i.addEventListener("inputsourceschange",F),S.xrCompatible!==!0&&await t.makeXRCompatible(),_=e.getPixelRatio(),e.getSize(C),M&&"createProjectionLayer"in XRWebGLBinding.prototype){let X=null,he=null,$=null;S.depth&&($=S.stencil?t.DEPTH24_STENCIL8:t.DEPTH_COMPONENT24,X=S.stencil?ji:ti,he=S.stencil?mr:Zn);let ie={colorFormat:t.RGBA8,depthFormat:$,scaleFactor:r};u=this.getBinding(),f=u.createProjectionLayer(ie),i.updateRenderState({layers:[f]}),e.setPixelRatio(1),e.setSize(f.textureWidth,f.textureHeight,!1),b=new Nt(f.textureWidth,f.textureHeight,{format:wn,type:gn,depthTexture:new bi(f.textureWidth,f.textureHeight,he,void 0,void 0,void 0,void 0,void 0,void 0,X),stencilBuffer:S.stencil,colorSpace:e.outputColorSpace,samples:S.antialias?4:0,resolveDepthBuffer:f.ignoreDepthValues===!1,resolveStencilBuffer:f.ignoreDepthValues===!1})}else{let X={antialias:S.antialias,alpha:!0,depth:S.depth,stencil:S.stencil,framebufferScaleFactor:r};d=new XRWebGLLayer(i,t,X),i.updateRenderState({baseLayer:d}),e.setPixelRatio(1),e.setSize(d.framebufferWidth,d.framebufferHeight,!1),b=new Nt(d.framebufferWidth,d.framebufferHeight,{format:wn,type:gn,colorSpace:e.outputColorSpace,stencilBuffer:S.stencil,resolveDepthBuffer:d.ignoreDepthValues===!1,resolveStencilBuffer:d.ignoreDepthValues===!1})}b.isXRRenderTarget=!0,this.setFoveation(l),c=null,o=await i.requestReferenceSpace(a),Fe.setContext(i),Fe.start(),n.isPresenting=!0,n.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(i!==null)return i.environmentBlendMode},this.getDepthTexture=function(){return m.getDepthTexture()};function F(Q){for(let ge=0;ge<Q.removed.length;ge++){let X=Q.removed[ge],he=y.indexOf(X);he>=0&&(y[he]=null,w[he].disconnect(X))}for(let ge=0;ge<Q.added.length;ge++){let X=Q.added[ge],he=y.indexOf(X);if(he===-1){for(let ie=0;ie<w.length;ie++)if(ie>=y.length){y.push(X),he=ie;break}else if(y[ie]===null){y[ie]=X,he=ie;break}if(he===-1)break}let $=w[he];$&&$.connect(X)}}let Y=new D,J=new D;function j(Q,ge,X){Y.setFromMatrixPosition(ge.matrixWorld),J.setFromMatrixPosition(X.matrixWorld);let he=Y.distanceTo(J),$=ge.projectionMatrix.elements,ie=X.projectionMatrix.elements,oe=$[14]/($[10]-1),ue=$[14]/($[10]+1),se=($[9]+1)/$[5],ce=($[9]-1)/$[5],ve=($[8]-1)/$[0],Re=(ie[8]+1)/ie[0],We=oe*ve,lt=oe*Re,ct=he/(-ve+Re),k=ct*-ve;if(ge.matrixWorld.decompose(Q.position,Q.quaternion,Q.scale),Q.translateX(k),Q.translateZ(ct),Q.matrixWorld.compose(Q.position,Q.quaternion,Q.scale),Q.matrixWorldInverse.copy(Q.matrixWorld).invert(),$[10]===-1)Q.projectionMatrix.copy(ge.projectionMatrix),Q.projectionMatrixInverse.copy(ge.projectionMatrixInverse);else{let ne=oe+ct,I=ue+ct,fe=We-k,N=lt+(he-k),T=se*ue/I*ne,x=ce*ue/I*ne;Q.projectionMatrix.makePerspective(fe,N,T,x,ne,I),Q.projectionMatrixInverse.copy(Q.projectionMatrix).invert()}}function me(Q,ge){ge===null?Q.matrixWorld.copy(Q.matrix):Q.matrixWorld.multiplyMatrices(ge.matrixWorld,Q.matrix),Q.matrixWorldInverse.copy(Q.matrixWorld).invert()}this.updateCamera=function(Q){if(i===null)return;let ge=Q.near,X=Q.far;m.texture!==null&&(m.depthNear>0&&(ge=m.depthNear),m.depthFar>0&&(X=m.depthFar)),L.near=E.near=v.near=ge,L.far=E.far=v.far=X,(H!==L.near||O!==L.far)&&(i.updateRenderState({depthNear:L.near,depthFar:L.far}),H=L.near,O=L.far),L.layers.mask=Q.layers.mask|6,v.layers.mask=L.layers.mask&-5,E.layers.mask=L.layers.mask&-3;let he=Q.parent,$=L.cameras;me(L,he);for(let ie=0;ie<$.length;ie++)me($[ie],he);$.length===2?j(L,v,E):L.projectionMatrix.copy(v.projectionMatrix),_e(Q,L,he)};function _e(Q,ge,X){X===null?Q.matrix.copy(ge.matrixWorld):(Q.matrix.copy(X.matrixWorld),Q.matrix.invert(),Q.matrix.multiply(ge.matrixWorld)),Q.matrix.decompose(Q.position,Q.quaternion,Q.scale),Q.updateMatrixWorld(!0),Q.projectionMatrix.copy(ge.projectionMatrix),Q.projectionMatrixInverse.copy(ge.projectionMatrixInverse),Q.isPerspectiveCamera&&(Q.fov=ms*2*Math.atan(1/Q.projectionMatrix.elements[5]),Q.zoom=1)}this.getCamera=function(){return L},this.getFoveation=function(){if(!(f===null&&d===null))return l},this.setFoveation=function(Q){l=Q,f!==null&&(f.fixedFoveation=Q),d!==null&&d.fixedFoveation!==void 0&&(d.fixedFoveation=Q)},this.hasDepthSensing=function(){return m.texture!==null},this.getDepthSensingMesh=function(){return m.getMesh(L)},this.getCameraTexture=function(Q){return p[Q]};let Ve=null;function it(Q,ge){if(h=ge.getViewerPose(c||o),g=ge,h!==null){let X=h.views;d!==null&&(e.setRenderTargetFramebuffer(b,d.framebuffer),e.setRenderTarget(b));let he=!1;X.length!==L.cameras.length&&(L.cameras.length=0,he=!0);for(let ue=0;ue<X.length;ue++){let se=X[ue],ce=null;if(d!==null)ce=d.getViewport(se);else{let Re=u.getViewSubImage(f,se);ce=Re.viewport,ue===0&&(e.setRenderTargetTextures(b,Re.colorTexture,Re.depthStencilTexture),e.setRenderTarget(b))}let ve=P[ue];ve===void 0&&(ve=new Ht,ve.layers.enable(ue),ve.viewport=new xt,P[ue]=ve),ve.matrix.fromArray(se.transform.matrix),ve.matrix.decompose(ve.position,ve.quaternion,ve.scale),ve.projectionMatrix.fromArray(se.projectionMatrix),ve.projectionMatrixInverse.copy(ve.projectionMatrix).invert(),ve.viewport.set(ce.x,ce.y,ce.width,ce.height),ue===0&&(L.matrix.copy(ve.matrix),L.matrix.decompose(L.position,L.quaternion,L.scale)),he===!0&&L.cameras.push(ve)}let $=i.enabledFeatures;if($&&$.includes("depth-sensing")&&i.depthUsage=="gpu-optimized"&&M){u=n.getBinding();let ue=u.getDepthInformation(X[0]);ue&&ue.isValid&&ue.texture&&m.init(ue,i.renderState)}if($&&$.includes("camera-access")&&M){e.state.unbindTexture(),u=n.getBinding();for(let ue=0;ue<X.length;ue++){let se=X[ue].camera;if(se){let ce=p[se];ce||(ce=new Jr,p[se]=ce);let ve=u.getCameraImage(se);ce.sourceTexture=ve}}}}for(let X=0;X<w.length;X++){let he=y[X],$=w[X];he!==null&&$!==void 0&&$.update(he,ge,c||o)}Ve&&Ve(Q,ge),ge.detectedPlanes&&n.dispatchEvent({type:"planesdetected",data:ge}),g=null}let Fe=new fd;Fe.setAnimationLoop(it),this.setAnimationLoop=function(Q){Ve=Q},this.dispose=function(){}}},$x=new Ke,xd=new $e;xd.set(-1,0,0,0,1,0,0,0,1);function Qx(s,e){function t(m,p){m.matrixAutoUpdate===!0&&m.updateMatrix(),p.value.copy(m.matrix)}function n(m,p){p.color.getRGB(m.fogColor.value,oh(s)),p.isFog?(m.fogNear.value=p.near,m.fogFar.value=p.far):p.isFogExp2&&(m.fogDensity.value=p.density)}function i(m,p,S,R,b){p.isNodeMaterial?p.uniformsNeedUpdate=!1:p.isMeshBasicMaterial?r(m,p):p.isMeshLambertMaterial?(r(m,p),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)):p.isMeshToonMaterial?(r(m,p),u(m,p)):p.isMeshPhongMaterial?(r(m,p),h(m,p),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)):p.isMeshStandardMaterial?(r(m,p),f(m,p),p.isMeshPhysicalMaterial&&d(m,p,b)):p.isMeshMatcapMaterial?(r(m,p),g(m,p)):p.isMeshDepthMaterial?r(m,p):p.isMeshDistanceMaterial?(r(m,p),M(m,p)):p.isMeshNormalMaterial?r(m,p):p.isLineBasicMaterial?(o(m,p),p.isLineDashedMaterial&&a(m,p)):p.isPointsMaterial?l(m,p,S,R):p.isSpriteMaterial?c(m,p):p.isShadowMaterial?(m.color.value.copy(p.color),m.opacity.value=p.opacity):p.isShaderMaterial&&(p.uniformsNeedUpdate=!1)}function r(m,p){m.opacity.value=p.opacity,p.color&&m.diffuse.value.copy(p.color),p.emissive&&m.emissive.value.copy(p.emissive).multiplyScalar(p.emissiveIntensity),p.map&&(m.map.value=p.map,t(p.map,m.mapTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.bumpMap&&(m.bumpMap.value=p.bumpMap,t(p.bumpMap,m.bumpMapTransform),m.bumpScale.value=p.bumpScale,p.side===Wt&&(m.bumpScale.value*=-1)),p.normalMap&&(m.normalMap.value=p.normalMap,t(p.normalMap,m.normalMapTransform),m.normalScale.value.copy(p.normalScale),p.side===Wt&&m.normalScale.value.negate()),p.displacementMap&&(m.displacementMap.value=p.displacementMap,t(p.displacementMap,m.displacementMapTransform),m.displacementScale.value=p.displacementScale,m.displacementBias.value=p.displacementBias),p.emissiveMap&&(m.emissiveMap.value=p.emissiveMap,t(p.emissiveMap,m.emissiveMapTransform)),p.specularMap&&(m.specularMap.value=p.specularMap,t(p.specularMap,m.specularMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest);let S=e.get(p),R=S.envMap,b=S.envMapRotation;R&&(m.envMap.value=R,m.envMapRotation.value.setFromMatrix4($x.makeRotationFromEuler(b)).transpose(),R.isCubeTexture&&R.isRenderTargetTexture===!1&&m.envMapRotation.value.premultiply(xd),m.reflectivity.value=p.reflectivity,m.ior.value=p.ior,m.refractionRatio.value=p.refractionRatio),p.lightMap&&(m.lightMap.value=p.lightMap,m.lightMapIntensity.value=p.lightMapIntensity,t(p.lightMap,m.lightMapTransform)),p.aoMap&&(m.aoMap.value=p.aoMap,m.aoMapIntensity.value=p.aoMapIntensity,t(p.aoMap,m.aoMapTransform))}function o(m,p){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,p.map&&(m.map.value=p.map,t(p.map,m.mapTransform))}function a(m,p){m.dashSize.value=p.dashSize,m.totalSize.value=p.dashSize+p.gapSize,m.scale.value=p.scale}function l(m,p,S,R){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,m.size.value=p.size*S,m.scale.value=R*.5,p.map&&(m.map.value=p.map,t(p.map,m.uvTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest)}function c(m,p){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,m.rotation.value=p.rotation,p.map&&(m.map.value=p.map,t(p.map,m.mapTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest)}function h(m,p){m.specular.value.copy(p.specular),m.shininess.value=Math.max(p.shininess,1e-4)}function u(m,p){p.gradientMap&&(m.gradientMap.value=p.gradientMap)}function f(m,p){m.metalness.value=p.metalness,p.metalnessMap&&(m.metalnessMap.value=p.metalnessMap,t(p.metalnessMap,m.metalnessMapTransform)),m.roughness.value=p.roughness,p.roughnessMap&&(m.roughnessMap.value=p.roughnessMap,t(p.roughnessMap,m.roughnessMapTransform)),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)}function d(m,p,S){m.ior.value=p.ior,p.sheen>0&&(m.sheenColor.value.copy(p.sheenColor).multiplyScalar(p.sheen),m.sheenRoughness.value=p.sheenRoughness,p.sheenColorMap&&(m.sheenColorMap.value=p.sheenColorMap,t(p.sheenColorMap,m.sheenColorMapTransform)),p.sheenRoughnessMap&&(m.sheenRoughnessMap.value=p.sheenRoughnessMap,t(p.sheenRoughnessMap,m.sheenRoughnessMapTransform))),p.clearcoat>0&&(m.clearcoat.value=p.clearcoat,m.clearcoatRoughness.value=p.clearcoatRoughness,p.clearcoatMap&&(m.clearcoatMap.value=p.clearcoatMap,t(p.clearcoatMap,m.clearcoatMapTransform)),p.clearcoatRoughnessMap&&(m.clearcoatRoughnessMap.value=p.clearcoatRoughnessMap,t(p.clearcoatRoughnessMap,m.clearcoatRoughnessMapTransform)),p.clearcoatNormalMap&&(m.clearcoatNormalMap.value=p.clearcoatNormalMap,t(p.clearcoatNormalMap,m.clearcoatNormalMapTransform),m.clearcoatNormalScale.value.copy(p.clearcoatNormalScale),p.side===Wt&&m.clearcoatNormalScale.value.negate())),p.dispersion>0&&(m.dispersion.value=p.dispersion),p.iridescence>0&&(m.iridescence.value=p.iridescence,m.iridescenceIOR.value=p.iridescenceIOR,m.iridescenceThicknessMinimum.value=p.iridescenceThicknessRange[0],m.iridescenceThicknessMaximum.value=p.iridescenceThicknessRange[1],p.iridescenceMap&&(m.iridescenceMap.value=p.iridescenceMap,t(p.iridescenceMap,m.iridescenceMapTransform)),p.iridescenceThicknessMap&&(m.iridescenceThicknessMap.value=p.iridescenceThicknessMap,t(p.iridescenceThicknessMap,m.iridescenceThicknessMapTransform))),p.transmission>0&&(m.transmission.value=p.transmission,m.transmissionSamplerMap.value=S.texture,m.transmissionSamplerSize.value.set(S.width,S.height),p.transmissionMap&&(m.transmissionMap.value=p.transmissionMap,t(p.transmissionMap,m.transmissionMapTransform)),m.thickness.value=p.thickness,p.thicknessMap&&(m.thicknessMap.value=p.thicknessMap,t(p.thicknessMap,m.thicknessMapTransform)),m.attenuationDistance.value=p.attenuationDistance,m.attenuationColor.value.copy(p.attenuationColor)),p.anisotropy>0&&(m.anisotropyVector.value.set(p.anisotropy*Math.cos(p.anisotropyRotation),p.anisotropy*Math.sin(p.anisotropyRotation)),p.anisotropyMap&&(m.anisotropyMap.value=p.anisotropyMap,t(p.anisotropyMap,m.anisotropyMapTransform))),m.specularIntensity.value=p.specularIntensity,m.specularColor.value.copy(p.specularColor),p.specularColorMap&&(m.specularColorMap.value=p.specularColorMap,t(p.specularColorMap,m.specularColorMapTransform)),p.specularIntensityMap&&(m.specularIntensityMap.value=p.specularIntensityMap,t(p.specularIntensityMap,m.specularIntensityMapTransform))}function g(m,p){p.matcap&&(m.matcap.value=p.matcap)}function M(m,p){let S=e.get(p).light;m.referencePosition.value.setFromMatrixPosition(S.matrixWorld),m.nearDistance.value=S.shadow.camera.near,m.farDistance.value=S.shadow.camera.far}return{refreshFogUniforms:n,refreshMaterialUniforms:i}}function ev(s,e,t,n){let i={},r={},o=[],a=s.getParameter(s.MAX_UNIFORM_BUFFER_BINDINGS);function l(b,w){let y=w.program;n.uniformBlockBinding(b,y)}function c(b,w){let y=i[b.id];y===void 0&&(m(b),y=h(b),i[b.id]=y,b.addEventListener("dispose",S));let C=w.program;n.updateUBOMapping(b,C);let _=e.render.frame;r[b.id]!==_&&(f(b),r[b.id]=_)}function h(b){let w=u();b.__bindingPointIndex=w;let y=s.createBuffer(),C=b.__size,_=b.usage;return s.bindBuffer(s.UNIFORM_BUFFER,y),s.bufferData(s.UNIFORM_BUFFER,C,_),s.bindBuffer(s.UNIFORM_BUFFER,null),s.bindBufferBase(s.UNIFORM_BUFFER,w,y),y}function u(){for(let b=0;b<a;b++)if(o.indexOf(b)===-1)return o.push(b),b;return Ze("WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function f(b){let w=i[b.id],y=b.uniforms,C=b.__cache;s.bindBuffer(s.UNIFORM_BUFFER,w);for(let _=0,v=y.length;_<v;_++){let E=y[_];if(Array.isArray(E))for(let P=0,L=E.length;P<L;P++)d(E[P],_,P,C);else d(E,_,0,C)}s.bindBuffer(s.UNIFORM_BUFFER,null)}function d(b,w,y,C){if(M(b,w,y,C)===!0){let _=b.__offset,v=b.value;if(Array.isArray(v)){let E=0;for(let P=0;P<v.length;P++){let L=v[P],H=p(L);g(L,b.__data,E),typeof L!="number"&&typeof L!="boolean"&&!L.isMatrix3&&!ArrayBuffer.isView(L)&&(E+=H.storage/Float32Array.BYTES_PER_ELEMENT)}}else g(v,b.__data,0);s.bufferSubData(s.UNIFORM_BUFFER,_,b.__data)}}function g(b,w,y){typeof b=="number"||typeof b=="boolean"?w[0]=b:b.isMatrix3?(w[0]=b.elements[0],w[1]=b.elements[1],w[2]=b.elements[2],w[3]=0,w[4]=b.elements[3],w[5]=b.elements[4],w[6]=b.elements[5],w[7]=0,w[8]=b.elements[6],w[9]=b.elements[7],w[10]=b.elements[8],w[11]=0):ArrayBuffer.isView(b)?w.set(new b.constructor(b.buffer,b.byteOffset,w.length)):b.toArray(w,y)}function M(b,w,y,C){let _=b.value,v=w+"_"+y;if(C[v]===void 0)return typeof _=="number"||typeof _=="boolean"?C[v]=_:ArrayBuffer.isView(_)?C[v]=_.slice():C[v]=_.clone(),!0;{let E=C[v];if(typeof _=="number"||typeof _=="boolean"){if(E!==_)return C[v]=_,!0}else{if(ArrayBuffer.isView(_))return!0;if(E.equals(_)===!1)return E.copy(_),!0}}return!1}function m(b){let w=b.uniforms,y=0,C=16;for(let v=0,E=w.length;v<E;v++){let P=Array.isArray(w[v])?w[v]:[w[v]];for(let L=0,H=P.length;L<H;L++){let O=P[L],U=Array.isArray(O.value)?O.value:[O.value];for(let V=0,F=U.length;V<F;V++){let Y=U[V],J=p(Y),j=y%C,me=j%J.boundary,_e=j+me;y+=me,_e!==0&&C-_e<J.storage&&(y+=C-_e),O.__data=new Float32Array(J.storage/Float32Array.BYTES_PER_ELEMENT),O.__offset=y,y+=J.storage}}}let _=y%C;return _>0&&(y+=C-_),b.__size=y,b.__cache={},this}function p(b){let w={boundary:0,storage:0};return typeof b=="number"||typeof b=="boolean"?(w.boundary=4,w.storage=4):b.isVector2?(w.boundary=8,w.storage=8):b.isVector3||b.isColor?(w.boundary=16,w.storage=12):b.isVector4?(w.boundary=16,w.storage=16):b.isMatrix3?(w.boundary=48,w.storage=48):b.isMatrix4?(w.boundary=64,w.storage=64):b.isTexture?Be("WebGLRenderer: Texture samplers can not be part of an uniforms group."):ArrayBuffer.isView(b)?(w.boundary=16,w.storage=b.byteLength):Be("WebGLRenderer: Unsupported uniform value type.",b),w}function S(b){let w=b.target;w.removeEventListener("dispose",S);let y=o.indexOf(w.__bindingPointIndex);o.splice(y,1),s.deleteBuffer(i[w.id]),delete i[w.id],delete r[w.id]}function R(){for(let b in i)s.deleteBuffer(i[b]);o=[],i={},r={}}return{bind:l,update:c,dispose:R}}var tv=new Uint16Array([12469,15057,12620,14925,13266,14620,13807,14376,14323,13990,14545,13625,14713,13328,14840,12882,14931,12528,14996,12233,15039,11829,15066,11525,15080,11295,15085,10976,15082,10705,15073,10495,13880,14564,13898,14542,13977,14430,14158,14124,14393,13732,14556,13410,14702,12996,14814,12596,14891,12291,14937,11834,14957,11489,14958,11194,14943,10803,14921,10506,14893,10278,14858,9960,14484,14039,14487,14025,14499,13941,14524,13740,14574,13468,14654,13106,14743,12678,14818,12344,14867,11893,14889,11509,14893,11180,14881,10751,14852,10428,14812,10128,14765,9754,14712,9466,14764,13480,14764,13475,14766,13440,14766,13347,14769,13070,14786,12713,14816,12387,14844,11957,14860,11549,14868,11215,14855,10751,14825,10403,14782,10044,14729,9651,14666,9352,14599,9029,14967,12835,14966,12831,14963,12804,14954,12723,14936,12564,14917,12347,14900,11958,14886,11569,14878,11247,14859,10765,14828,10401,14784,10011,14727,9600,14660,9289,14586,8893,14508,8533,15111,12234,15110,12234,15104,12216,15092,12156,15067,12010,15028,11776,14981,11500,14942,11205,14902,10752,14861,10393,14812,9991,14752,9570,14682,9252,14603,8808,14519,8445,14431,8145,15209,11449,15208,11451,15202,11451,15190,11438,15163,11384,15117,11274,15055,10979,14994,10648,14932,10343,14871,9936,14803,9532,14729,9218,14645,8742,14556,8381,14461,8020,14365,7603,15273,10603,15272,10607,15267,10619,15256,10631,15231,10614,15182,10535,15118,10389,15042,10167,14963,9787,14883,9447,14800,9115,14710,8665,14615,8318,14514,7911,14411,7507,14279,7198,15314,9675,15313,9683,15309,9712,15298,9759,15277,9797,15229,9773,15166,9668,15084,9487,14995,9274,14898,8910,14800,8539,14697,8234,14590,7790,14479,7409,14367,7067,14178,6621,15337,8619,15337,8631,15333,8677,15325,8769,15305,8871,15264,8940,15202,8909,15119,8775,15022,8565,14916,8328,14804,8009,14688,7614,14569,7287,14448,6888,14321,6483,14088,6171,15350,7402,15350,7419,15347,7480,15340,7613,15322,7804,15287,7973,15229,8057,15148,8012,15046,7846,14933,7611,14810,7357,14682,7069,14552,6656,14421,6316,14251,5948,14007,5528,15356,5942,15356,5977,15353,6119,15348,6294,15332,6551,15302,6824,15249,7044,15171,7122,15070,7050,14949,6861,14818,6611,14679,6349,14538,6067,14398,5651,14189,5311,13935,4958,15359,4123,15359,4153,15356,4296,15353,4646,15338,5160,15311,5508,15263,5829,15188,6042,15088,6094,14966,6001,14826,5796,14678,5543,14527,5287,14377,4985,14133,4586,13869,4257,15360,1563,15360,1642,15358,2076,15354,2636,15341,3350,15317,4019,15273,4429,15203,4732,15105,4911,14981,4932,14836,4818,14679,4621,14517,4386,14359,4156,14083,3795,13808,3437,15360,122,15360,137,15358,285,15355,636,15344,1274,15322,2177,15281,2765,15215,3223,15120,3451,14995,3569,14846,3567,14681,3466,14511,3305,14344,3121,14037,2800,13753,2467,15360,0,15360,1,15359,21,15355,89,15346,253,15325,479,15287,796,15225,1148,15133,1492,15008,1749,14856,1882,14685,1886,14506,1783,14324,1608,13996,1398,13702,1183]),ci=null;function nv(){return ci===null&&(ci=new nr(tv,16,16,$i,Xt),ci.name="DFG_LUT",ci.minFilter=Lt,ci.magFilter=Lt,ci.wrapS=Dn,ci.wrapT=Dn,ci.generateMipmaps=!1,ci.needsUpdate=!0),ci}var Xl=class{constructor(e={}){let{canvas:t=Bf(),context:n=null,depth:i=!0,stencil:r=!1,alpha:o=!1,antialias:a=!1,premultipliedAlpha:l=!0,preserveDrawingBuffer:c=!1,powerPreference:h="default",failIfMajorPerformanceCaveat:u=!1,reversedDepthBuffer:f=!1,outputBufferType:d=gn}=e;this.isWebGLRenderer=!0;let g;if(n!==null){if(typeof WebGLRenderingContext<"u"&&n instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");g=n.getContextAttributes().alpha}else g=o;let M=d,m=new Set([al,ol,rl]),p=new Set([gn,Zn,pr,mr,nl,il]),S=new Uint32Array(4),R=new Int32Array(4),b=new D,w=null,y=null,C=[],_=[],v=null;this.domElement=t,this.debug={checkShaderErrors:!0,onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this.toneMapping=qn,this.toneMappingExposure=1,this.transmissionResolutionScale=1;let E=this,P=!1,L=null,H=null,O=null,U=null;this._outputColorSpace=Pt;let V=0,F=0,Y=null,J=-1,j=null,me=new xt,_e=new xt,Ve=null,it=new Me(0),Fe=0,Q=t.width,ge=t.height,X=1,he=null,$=null,ie=new xt(0,0,Q,ge),oe=new xt(0,0,Q,ge),ue=!1,se=new ir,ce=!1,ve=!1,Re=new Ke,We=new D,lt=new xt,ct={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},k=!1;function ne(){return Y===null?X:1}let I=n;function fe(A,W){return t.getContext(A,W)}try{let A={alpha:!0,depth:i,stencil:r,antialias:a,premultipliedAlpha:l,preserveDrawingBuffer:c,powerPreference:h,failIfMajorPerformanceCaveat:u};if("setAttribute"in t&&t.setAttribute("data-engine",`three.js r${"185"}`),t.addEventListener("webglcontextlost",q,!1),t.addEventListener("webglcontextrestored",xe,!1),t.addEventListener("webglcontextcreationerror",rt,!1),I===null){let W="webgl2";if(I=fe(W,A),I===null)throw fe(W)?new Error("THREE.WebGLRenderer: Error creating WebGL context with your selected attributes."):new Error("THREE.WebGLRenderer: Error creating WebGL context.")}}catch(A){throw Ze("WebGLRenderer: "+A.message),A}let N,T,x,B,G,ee,pe,be,re,ae,Te,Oe,Ae,Ee,ze,qe,Ye,z,Se,le,we,Pe,de;function ke(){N=new c_(I),N.init(),we=new Kx(I,N),T=new t_(I,N,e,we),x=new Yx(I,N),T.reversedDepthBuffer&&f&&x.buffers.depth.setReversed(!0),H=I.createFramebuffer(),O=I.createFramebuffer(),U=I.createFramebuffer(),B=new f_(I),G=new Dx,ee=new Zx(I,N,x,G,T,we,B),pe=new l_(E),be=new gm(I),Pe=new Q0(I,be),re=new h_(I,be,B,Pe),ae=new p_(I,re,be,Pe,B),z=new d_(I,T,ee),ze=new n_(G),Te=new Lx(E,pe,N,T,Pe,ze),Oe=new Qx(E,G),Ae=new Ux,Ee=new Hx(N),Ye=new $0(E,pe,x,ae,g,l),qe=new qx(E,ae,T),de=new ev(I,B,T,x),Se=new e_(I,N,B),le=new u_(I,N,B),B.programs=Te.programs,E.capabilities=T,E.extensions=N,E.properties=G,E.renderLists=Ae,E.shadowMap=qe,E.state=x,E.info=B}ke(),M!==gn&&(v=new g_(M,t.width,t.height,a,i,r));let De=new Th(E,I);this.xr=De,this.getContext=function(){return I},this.getContextAttributes=function(){return I.getContextAttributes()},this.forceContextLoss=function(){let A=N.get("WEBGL_lose_context");A&&A.loseContext()},this.forceContextRestore=function(){let A=N.get("WEBGL_lose_context");A&&A.restoreContext()},this.getPixelRatio=function(){return X},this.setPixelRatio=function(A){A!==void 0&&(X=A,this.setSize(Q,ge,!1))},this.getSize=function(A){return A.set(Q,ge)},this.setSize=function(A,W,te=!0){if(De.isPresenting){Be("WebGLRenderer: Can't change size while VR device is presenting.");return}Q=A,ge=W,t.width=Math.floor(A*X),t.height=Math.floor(W*X),te===!0&&(t.style.width=A+"px",t.style.height=W+"px"),v!==null&&v.setSize(t.width,t.height),this.setViewport(0,0,A,W)},this.getDrawingBufferSize=function(A){return A.set(Q*X,ge*X).floor()},this.setDrawingBufferSize=function(A,W,te){Q=A,ge=W,X=te,t.width=Math.floor(A*te),t.height=Math.floor(W*te),this.setViewport(0,0,A,W)},this.setEffects=function(A){if(M===gn){Ze("WebGLRenderer: setEffects() requires outputBufferType set to HalfFloatType or FloatType.");return}if(A){for(let W=0;W<A.length;W++)if(A[W].isOutputPass===!0){Be("WebGLRenderer: OutputPass is not needed in setEffects(). Tone mapping and color space conversion are applied automatically.");break}}v.setEffects(A||[])},this.getCurrentViewport=function(A){return A.copy(me)},this.getViewport=function(A){return A.copy(ie)},this.setViewport=function(A,W,te,Z){A.isVector4?ie.set(A.x,A.y,A.z,A.w):ie.set(A,W,te,Z),x.viewport(me.copy(ie).multiplyScalar(X).round())},this.getScissor=function(A){return A.copy(oe)},this.setScissor=function(A,W,te,Z){A.isVector4?oe.set(A.x,A.y,A.z,A.w):oe.set(A,W,te,Z),x.scissor(_e.copy(oe).multiplyScalar(X).round())},this.getScissorTest=function(){return ue},this.setScissorTest=function(A){x.setScissorTest(ue=A)},this.setOpaqueSort=function(A){he=A},this.setTransparentSort=function(A){$=A},this.getClearColor=function(A){return A.copy(Ye.getClearColor())},this.setClearColor=function(){Ye.setClearColor(...arguments)},this.getClearAlpha=function(){return Ye.getClearAlpha()},this.setClearAlpha=function(){Ye.setClearAlpha(...arguments)},this.clear=function(A=!0,W=!0,te=!0){let Z=0;if(A){let K=!1;if(Y!==null){let Le=Y.texture.format;K=m.has(Le)}if(K){let Le=Y.texture.type,Ue=p.has(Le),Ie=Ye.getClearColor(),He=Ye.getClearAlpha(),Ge=Ie.r,Qe=Ie.g,st=Ie.b;Ue?(S[0]=Ge,S[1]=Qe,S[2]=st,S[3]=He,I.clearBufferuiv(I.COLOR,0,S)):(R[0]=Ge,R[1]=Qe,R[2]=st,R[3]=He,I.clearBufferiv(I.COLOR,0,R))}else Z|=I.COLOR_BUFFER_BIT}W&&(Z|=I.DEPTH_BUFFER_BIT,this.state.buffers.depth.setMask(!0)),te&&(Z|=I.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),Z!==0&&I.clear(Z)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.setNodesHandler=function(A){A.setRenderer(this),L=A},this.dispose=function(){t.removeEventListener("webglcontextlost",q,!1),t.removeEventListener("webglcontextrestored",xe,!1),t.removeEventListener("webglcontextcreationerror",rt,!1),Ye.dispose(),Ae.dispose(),Ee.dispose(),G.dispose(),pe.dispose(),ae.dispose(),Pe.dispose(),de.dispose(),Te.dispose(),De.dispose(),De.removeEventListener("sessionstart",Ct),De.removeEventListener("sessionend",jn),$t.stop()};function q(A){A.preventDefault(),Hr("WebGLRenderer: Context Lost."),P=!0}function xe(){Hr("WebGLRenderer: Context Restored."),P=!1;let A=B.autoReset,W=qe.enabled,te=qe.autoUpdate,Z=qe.needsUpdate,K=qe.type;ke(),B.autoReset=A,qe.enabled=W,qe.autoUpdate=te,qe.needsUpdate=Z,qe.type=K}function rt(A){Ze("WebGLRenderer: A WebGL context could not be created. Reason: ",A.statusMessage)}function je(A){let W=A.target;W.removeEventListener("dispose",je),gt(W)}function gt(A){an(A),G.remove(A)}function an(A){let W=G.get(A).programs;W!==void 0&&(W.forEach(function(te){Te.releaseProgram(te)}),A.isShaderMaterial&&Te.releaseShaderCache(A))}this.renderBufferDirect=function(A,W,te,Z,K,Le){W===null&&(W=ct);let Ue=K.isMesh&&K.matrixWorld.determinantAffine()<0,Ie=np(A,W,te,Z,K);x.setMaterial(Z,Ue);let He=te.index,Ge=1;if(Z.wireframe===!0){if(He=re.getWireframeAttribute(te),He===void 0)return;Ge=2}let Qe=te.drawRange,st=te.attributes.position,Xe=Qe.start*Ge,bt=(Qe.start+Qe.count)*Ge;Le!==null&&(Xe=Math.max(Xe,Le.start*Ge),bt=Math.min(bt,(Le.start+Le.count)*Ge)),He!==null?(Xe=Math.max(Xe,0),bt=Math.min(bt,He.count)):st!=null&&(Xe=Math.max(Xe,0),bt=Math.min(bt,st.count));let Ut=bt-Xe;if(Ut<0||Ut===1/0)return;Pe.setup(K,Z,Ie,te,He);let Dt,wt=Se;if(He!==null&&(Dt=be.get(He),wt=le,wt.setIndex(Dt)),K.isMesh)Z.wireframe===!0?(x.setLineWidth(Z.wireframeLinewidth*ne()),wt.setMode(I.LINES)):wt.setMode(I.TRIANGLES);else if(K.isLine){let Qt=Z.linewidth;Qt===void 0&&(Qt=1),x.setLineWidth(Qt*ne()),K.isLineSegments?wt.setMode(I.LINES):K.isLineLoop?wt.setMode(I.LINE_LOOP):wt.setMode(I.LINE_STRIP)}else K.isPoints?wt.setMode(I.POINTS):K.isSprite&&wt.setMode(I.TRIANGLES);if(K.isBatchedMesh)if(N.get("WEBGL_multi_draw"))wt.renderMultiDraw(K._multiDrawStarts,K._multiDrawCounts,K._multiDrawCount);else{let Qt=K._multiDrawStarts,Ne=K._multiDrawCounts,yn=K._multiDrawCount,dt=He?be.get(He).bytesPerElement:1,Pn=G.get(Z).currentProgram.getUniforms();for(let $n=0;$n<yn;$n++)Pn.setValue(I,"_gl_DrawID",$n),wt.render(Qt[$n]/dt,Ne[$n])}else if(K.isInstancedMesh)wt.renderInstances(Xe,Ut,K.count);else if(te.isInstancedBufferGeometry){let Qt=te._maxInstanceCount!==void 0?te._maxInstanceCount:1/0,Ne=Math.min(te.instanceCount,Qt);wt.renderInstances(Xe,Ut,Ne)}else wt.render(Xe,Ut)};function Kt(A,W,te){A.transparent===!0&&A.side===Bt&&A.forceSinglePass===!1?(A.side=Wt,A.needsUpdate=!0,Ho(A,W,te),A.side=Sn,A.needsUpdate=!0,Ho(A,W,te),A.side=Bt):Ho(A,W,te)}this.compile=function(A,W,te=null){te===null&&(te=A),y=Ee.get(te),y.init(W),_.push(y),te.traverseVisible(function(K){K.isLight&&K.layers.test(W.layers)&&(y.pushLight(K),K.castShadow&&y.pushShadow(K))}),A!==te&&A.traverseVisible(function(K){K.isLight&&K.layers.test(W.layers)&&(y.pushLight(K),K.castShadow&&y.pushShadow(K))}),y.setupLights();let Z=new Set;return A.traverse(function(K){if(!(K.isMesh||K.isPoints||K.isLine||K.isSprite))return;let Le=K.material;if(Le)if(Array.isArray(Le))for(let Ue=0;Ue<Le.length;Ue++){let Ie=Le[Ue];Kt(Ie,te,K),Z.add(Ie)}else Kt(Le,te,K),Z.add(Le)}),y=_.pop(),Z},this.compileAsync=function(A,W,te=null){let Z=this.compile(A,W,te);return new Promise(K=>{function Le(){if(Z.forEach(function(Ue){G.get(Ue).currentProgram.isReady()&&Z.delete(Ue)}),Z.size===0){K(A);return}setTimeout(Le,10)}N.get("KHR_parallel_shader_compile")!==null?Le():setTimeout(Le,10)})};let _t=null;function Jt(A){_t&&_t(A)}function Ct(){$t.stop()}function jn(){$t.start()}let $t=new fd;$t.setAnimationLoop(Jt),typeof self<"u"&&$t.setContext(self),this.setAnimationLoop=function(A){_t=A,De.setAnimationLoop(A),A===null?$t.stop():$t.start()},De.addEventListener("sessionstart",Ct),De.addEventListener("sessionend",jn),this.render=function(A,W){if(W!==void 0&&W.isCamera!==!0){Ze("WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(P===!0)return;L!==null&&L.renderStart(A,W);let te=De.enabled===!0&&De.isPresenting===!0,Z=v!==null&&(Y===null||te)&&v.begin(E,Y);if(A.matrixWorldAutoUpdate===!0&&A.updateMatrixWorld(),W.parent===null&&W.matrixWorldAutoUpdate===!0&&W.updateMatrixWorld(),De.enabled===!0&&De.isPresenting===!0&&(v===null||v.isCompositing()===!1)&&(De.cameraAutoUpdate===!0&&De.updateCamera(W),W=De.getCamera()),A.isScene===!0&&A.onBeforeRender(E,A,W,Y),y=Ee.get(A,_.length),y.init(W),y.state.textureUnits=ee.getTextureUnits(),_.push(y),Re.multiplyMatrices(W.projectionMatrix,W.matrixWorldInverse),se.setFromProjectionMatrix(Re,Gn,W.reversedDepth),ve=this.localClippingEnabled,ce=ze.init(this.clippingPlanes,ve),w=Ae.get(A,C.length),w.init(),C.push(w),De.enabled===!0&&De.isPresenting===!0){let Ue=E.xr.getDepthSensingMesh();Ue!==null&&Cn(Ue,W,-1/0,E.sortObjects)}Cn(A,W,0,E.sortObjects),w.finish(),E.sortObjects===!0&&w.sort(he,$,W.reversedDepth),k=De.enabled===!1||De.isPresenting===!1||De.hasDepthSensing()===!1,k&&Ye.addToRenderList(w,A),this.info.render.frame++,this.info.autoReset===!0&&this.info.reset(),ce===!0&&ze.beginShadows();let K=y.state.shadowsArray;if(qe.render(K,A,W),ce===!0&&ze.endShadows(),(Z&&v.hasRenderPass())===!1){let Ue=w.opaque,Ie=w.transmissive;if(y.setupLights(),W.isArrayCamera){let He=W.cameras;if(Ie.length>0)for(let Ge=0,Qe=He.length;Ge<Qe;Ge++){let st=He[Ge];cu(Ue,Ie,A,st)}k&&Ye.render(A);for(let Ge=0,Qe=He.length;Ge<Qe;Ge++){let st=He[Ge];zo(w,A,st,st.viewport)}}else Ie.length>0&&cu(Ue,Ie,A,W),k&&Ye.render(A),zo(w,A,W)}Y!==null&&F===0&&(ee.updateMultisampleRenderTarget(Y),ee.updateRenderTargetMipmap(Y)),Z&&v.end(E),A.isScene===!0&&A.onAfterRender(E,A,W),Pe.resetDefaultState(),J=-1,j=null,_.pop(),_.length>0?(y=_[_.length-1],ee.setTextureUnits(y.state.textureUnits),ce===!0&&ze.setGlobalState(E.clippingPlanes,y.state.camera)):y=null,C.pop(),C.length>0?w=C[C.length-1]:w=null,L!==null&&L.renderEnd()};function Cn(A,W,te,Z){if(A.visible===!1)return;if(A.layers.test(W.layers)){if(A.isGroup)te=A.renderOrder;else if(A.isLOD)A.autoUpdate===!0&&A.update(W);else if(A.isLightProbeGrid)y.pushLightProbeGrid(A);else if(A.isLight)y.pushLight(A),A.castShadow&&y.pushShadow(A);else if(A.isSprite){if(!A.frustumCulled||se.intersectsSprite(A)){Z&&lt.setFromMatrixPosition(A.matrixWorld).applyMatrix4(Re);let Ue=ae.update(A),Ie=A.material;Ie.visible&&w.push(A,Ue,Ie,te,lt.z,null)}}else if((A.isMesh||A.isLine||A.isPoints)&&(!A.frustumCulled||se.intersectsObject(A))){let Ue=ae.update(A),Ie=A.material;if(Z&&(A.boundingSphere!==void 0?(A.boundingSphere===null&&A.computeBoundingSphere(),lt.copy(A.boundingSphere.center)):(Ue.boundingSphere===null&&Ue.computeBoundingSphere(),lt.copy(Ue.boundingSphere.center)),lt.applyMatrix4(A.matrixWorld).applyMatrix4(Re)),Array.isArray(Ie)){let He=Ue.groups;for(let Ge=0,Qe=He.length;Ge<Qe;Ge++){let st=He[Ge],Xe=Ie[st.materialIndex];Xe&&Xe.visible&&w.push(A,Ue,Xe,te,lt.z,st)}}else Ie.visible&&w.push(A,Ue,Ie,te,lt.z,null)}}let Le=A.children;for(let Ue=0,Ie=Le.length;Ue<Ie;Ue++)Cn(Le[Ue],W,te,Z)}function zo(A,W,te,Z){let{opaque:K,transmissive:Le,transparent:Ue}=A;y.setupLightsView(te),ce===!0&&ze.setGlobalState(E.clippingPlanes,te),Z&&x.viewport(me.copy(Z)),K.length>0&&ko(K,W,te),Le.length>0&&ko(Le,W,te),Ue.length>0&&ko(Ue,W,te),x.buffers.depth.setTest(!0),x.buffers.depth.setMask(!0),x.buffers.color.setMask(!0),x.setPolygonOffset(!1)}function cu(A,W,te,Z){if((te.isScene===!0?te.overrideMaterial:null)!==null)return;if(y.state.transmissionRenderTarget[Z.id]===void 0){let Xe=N.has("EXT_color_buffer_half_float")||N.has("EXT_color_buffer_float");y.state.transmissionRenderTarget[Z.id]=new Nt(1,1,{generateMipmaps:!0,type:Xe?Xt:gn,minFilter:Yn,samples:Math.max(4,T.samples),stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,colorSpace:et.workingColorSpace})}let Le=y.state.transmissionRenderTarget[Z.id],Ue=Z.viewport||me;Le.setSize(Ue.z*E.transmissionResolutionScale,Ue.w*E.transmissionResolutionScale);let Ie=E.getRenderTarget(),He=E.getActiveCubeFace(),Ge=E.getActiveMipmapLevel();E.setRenderTarget(Le),E.getClearColor(it),Fe=E.getClearAlpha(),Fe<1&&E.setClearColor(16777215,.5),E.clear(),k&&Ye.render(te);let Qe=E.toneMapping;E.toneMapping=qn;let st=Z.viewport;if(Z.viewport!==void 0&&(Z.viewport=void 0),y.setupLightsView(Z),ce===!0&&ze.setGlobalState(E.clippingPlanes,Z),ko(A,te,Z),ee.updateMultisampleRenderTarget(Le),ee.updateRenderTargetMipmap(Le),N.has("WEBGL_multisampled_render_to_texture")===!1){let Xe=!1;for(let bt=0,Ut=W.length;bt<Ut;bt++){let Dt=W[bt],{object:wt,geometry:Qt,material:Ne,group:yn}=Dt;if(Ne.side===Bt&&wt.layers.test(Z.layers)){let dt=Ne.side;Ne.side=Wt,Ne.needsUpdate=!0,hu(wt,te,Z,Qt,Ne,yn),Ne.side=dt,Ne.needsUpdate=!0,Xe=!0}}Xe===!0&&(ee.updateMultisampleRenderTarget(Le),ee.updateRenderTargetMipmap(Le))}E.setRenderTarget(Ie,He,Ge),E.setClearColor(it,Fe),st!==void 0&&(Z.viewport=st),E.toneMapping=Qe}function ko(A,W,te){let Z=W.isScene===!0?W.overrideMaterial:null;for(let K=0,Le=A.length;K<Le;K++){let Ue=A[K],{object:Ie,geometry:He,group:Ge}=Ue,Qe=Ue.material;Qe.allowOverride===!0&&Z!==null&&(Qe=Z),Ie.layers.test(te.layers)&&hu(Ie,W,te,He,Qe,Ge)}}function hu(A,W,te,Z,K,Le){A.onBeforeRender(E,W,te,Z,K,Le),A.modelViewMatrix.multiplyMatrices(te.matrixWorldInverse,A.matrixWorld),A.normalMatrix.getNormalMatrix(A.modelViewMatrix),K.onBeforeRender(E,W,te,Z,A,Le),K.transparent===!0&&K.side===Bt&&K.forceSinglePass===!1?(K.side=Wt,K.needsUpdate=!0,E.renderBufferDirect(te,W,Z,K,A,Le),K.side=Sn,K.needsUpdate=!0,E.renderBufferDirect(te,W,Z,K,A,Le),K.side=Bt):E.renderBufferDirect(te,W,Z,K,A,Le),A.onAfterRender(E,W,te,Z,K,Le)}function Ho(A,W,te){W.isScene!==!0&&(W=ct);let Z=G.get(A),K=y.state.lights,Le=y.state.shadowsArray,Ue=K.state.version,Ie=Te.getParameters(A,K.state,Le,W,te,y.state.lightProbeGridArray),He=Te.getProgramCacheKey(Ie),Ge=Z.programs;Z.environment=A.isMeshStandardMaterial||A.isMeshLambertMaterial||A.isMeshPhongMaterial?W.environment:null,Z.fog=W.fog;let Qe=A.isMeshStandardMaterial||A.isMeshLambertMaterial&&!A.envMap||A.isMeshPhongMaterial&&!A.envMap;Z.envMap=pe.get(A.envMap||Z.environment,Qe),Z.envMapRotation=Z.environment!==null&&A.envMap===null?W.environmentRotation:A.envMapRotation,Ge===void 0&&(A.addEventListener("dispose",je),Ge=new Map,Z.programs=Ge);let st=Ge.get(He);if(st!==void 0){if(Z.currentProgram===st&&Z.lightsStateVersion===Ue)return fu(A,Ie),st}else Ie.uniforms=Te.getUniforms(A),L!==null&&A.isNodeMaterial&&L.build(A,te,Ie),A.onBeforeCompile(Ie,E),st=Te.acquireProgram(Ie,He),Ge.set(He,st),Z.uniforms=Ie.uniforms;let Xe=Z.uniforms;return(!A.isShaderMaterial&&!A.isRawShaderMaterial||A.clipping===!0)&&(Xe.clippingPlanes=ze.uniform),fu(A,Ie),Z.needsLights=sp(A),Z.lightsStateVersion=Ue,Z.needsLights&&(Xe.ambientLightColor.value=K.state.ambient,Xe.lightProbe.value=K.state.probe,Xe.directionalLights.value=K.state.directional,Xe.directionalLightShadows.value=K.state.directionalShadow,Xe.spotLights.value=K.state.spot,Xe.spotLightShadows.value=K.state.spotShadow,Xe.rectAreaLights.value=K.state.rectArea,Xe.ltc_1.value=K.state.rectAreaLTC1,Xe.ltc_2.value=K.state.rectAreaLTC2,Xe.pointLights.value=K.state.point,Xe.pointLightShadows.value=K.state.pointShadow,Xe.hemisphereLights.value=K.state.hemi,Xe.directionalShadowMatrix.value=K.state.directionalShadowMatrix,Xe.spotLightMatrix.value=K.state.spotLightMatrix,Xe.spotLightMap.value=K.state.spotLightMap,Xe.pointShadowMatrix.value=K.state.pointShadowMatrix),Z.lightProbeGrid=y.state.lightProbeGridArray.length>0,Z.currentProgram=st,Z.uniformsList=null,st}function uu(A){if(A.uniformsList===null){let W=A.currentProgram.getUniforms();A.uniformsList=xr.seqWithValue(W.seq,A.uniforms)}return A.uniformsList}function fu(A,W){let te=G.get(A);te.outputColorSpace=W.outputColorSpace,te.batching=W.batching,te.batchingColor=W.batchingColor,te.instancing=W.instancing,te.instancingColor=W.instancingColor,te.instancingMorph=W.instancingMorph,te.skinning=W.skinning,te.morphTargets=W.morphTargets,te.morphNormals=W.morphNormals,te.morphColors=W.morphColors,te.morphTargetsCount=W.morphTargetsCount,te.numClippingPlanes=W.numClippingPlanes,te.numIntersection=W.numClipIntersection,te.vertexAlphas=W.vertexAlphas,te.vertexTangents=W.vertexTangents,te.toneMapping=W.toneMapping}function tp(A,W){if(A.length===0)return null;if(A.length===1)return A[0].texture!==null?A[0]:null;b.setFromMatrixPosition(W.matrixWorld);for(let te=0,Z=A.length;te<Z;te++){let K=A[te];if(K.texture!==null&&K.boundingBox.containsPoint(b))return K}return null}function np(A,W,te,Z,K){W.isScene!==!0&&(W=ct),ee.resetTextureUnits();let Le=W.fog,Ue=Z.isMeshStandardMaterial||Z.isMeshLambertMaterial||Z.isMeshPhongMaterial?W.environment:null,Ie=Y===null?E.outputColorSpace:Y.isXRRenderTarget===!0?Y.texture.colorSpace:et.workingColorSpace,He=Z.isMeshStandardMaterial||Z.isMeshLambertMaterial&&!Z.envMap||Z.isMeshPhongMaterial&&!Z.envMap,Ge=pe.get(Z.envMap||Ue,He),Qe=Z.vertexColors===!0&&!!te.attributes.color&&te.attributes.color.itemSize===4,st=!!te.attributes.tangent&&(!!Z.normalMap||Z.anisotropy>0),Xe=!!te.morphAttributes.position,bt=!!te.morphAttributes.normal,Ut=!!te.morphAttributes.color,Dt=qn;Z.toneMapped&&(Y===null||Y.isXRRenderTarget===!0)&&(Dt=E.toneMapping);let wt=te.morphAttributes.position||te.morphAttributes.normal||te.morphAttributes.color,Qt=wt!==void 0?wt.length:0,Ne=G.get(Z),yn=y.state.lights;if(ce===!0&&(ve===!0||A!==j)){let Rt=A===j&&Z.id===J;ze.setState(Z,A,Rt)}let dt=!1;Z.version===Ne.__version?(Ne.needsLights&&Ne.lightsStateVersion!==yn.state.version||Ne.outputColorSpace!==Ie||K.isBatchedMesh&&Ne.batching===!1||!K.isBatchedMesh&&Ne.batching===!0||K.isBatchedMesh&&Ne.batchingColor===!0&&K.colorTexture===null||K.isBatchedMesh&&Ne.batchingColor===!1&&K.colorTexture!==null||K.isInstancedMesh&&Ne.instancing===!1||!K.isInstancedMesh&&Ne.instancing===!0||K.isSkinnedMesh&&Ne.skinning===!1||!K.isSkinnedMesh&&Ne.skinning===!0||K.isInstancedMesh&&Ne.instancingColor===!0&&K.instanceColor===null||K.isInstancedMesh&&Ne.instancingColor===!1&&K.instanceColor!==null||K.isInstancedMesh&&Ne.instancingMorph===!0&&K.morphTexture===null||K.isInstancedMesh&&Ne.instancingMorph===!1&&K.morphTexture!==null||Ne.envMap!==Ge||Z.fog===!0&&Ne.fog!==Le||Ne.numClippingPlanes!==void 0&&(Ne.numClippingPlanes!==ze.numPlanes||Ne.numIntersection!==ze.numIntersection)||Ne.vertexAlphas!==Qe||Ne.vertexTangents!==st||Ne.morphTargets!==Xe||Ne.morphNormals!==bt||Ne.morphColors!==Ut||Ne.toneMapping!==Dt||Ne.morphTargetsCount!==Qt||!!Ne.lightProbeGrid!=y.state.lightProbeGridArray.length>0)&&(dt=!0):(dt=!0,Ne.__version=Z.version);let Pn=Ne.currentProgram;dt===!0&&(Pn=Ho(Z,W,K),L&&Z.isNodeMaterial&&L.onUpdateProgram(Z,Pn,Ne));let $n=!1,Li=!1,Cs=!1,At=Pn.getUniforms(),Ft=Ne.uniforms;if(x.useProgram(Pn.program)&&($n=!0,Li=!0,Cs=!0),Z.id!==J&&(J=Z.id,Li=!0),Ne.needsLights){let Rt=tp(y.state.lightProbeGridArray,K);Ne.lightProbeGrid!==Rt&&(Ne.lightProbeGrid=Rt,Li=!0)}if($n||j!==A){x.buffers.depth.getReversed()&&A.reversedDepth!==!0&&(A._reversedDepth=!0,A.updateProjectionMatrix()),At.setValue(I,"projectionMatrix",A.projectionMatrix),At.setValue(I,"viewMatrix",A.matrixWorldInverse);let Ni=At.map.cameraPosition;Ni!==void 0&&Ni.setValue(I,We.setFromMatrixPosition(A.matrixWorld)),T.logarithmicDepthBuffer&&At.setValue(I,"logDepthBufFC",2/(Math.log(A.far+1)/Math.LN2)),(Z.isMeshPhongMaterial||Z.isMeshToonMaterial||Z.isMeshLambertMaterial||Z.isMeshBasicMaterial||Z.isMeshStandardMaterial||Z.isShaderMaterial)&&At.setValue(I,"isOrthographic",A.isOrthographicCamera===!0),j!==A&&(j=A,Li=!0,Cs=!0)}if(Ne.needsLights&&(yn.state.directionalShadowMap.length>0&&At.setValue(I,"directionalShadowMap",yn.state.directionalShadowMap,ee),yn.state.spotShadowMap.length>0&&At.setValue(I,"spotShadowMap",yn.state.spotShadowMap,ee),yn.state.pointShadowMap.length>0&&At.setValue(I,"pointShadowMap",yn.state.pointShadowMap,ee)),K.isSkinnedMesh){At.setOptional(I,K,"bindMatrix"),At.setOptional(I,K,"bindMatrixInverse");let Rt=K.skeleton;Rt&&(Rt.boneTexture===null&&Rt.computeBoneTexture(),At.setValue(I,"boneTexture",Rt.boneTexture,ee))}K.isBatchedMesh&&(At.setOptional(I,K,"batchingTexture"),At.setValue(I,"batchingTexture",K._matricesTexture,ee),At.setOptional(I,K,"batchingIdTexture"),At.setValue(I,"batchingIdTexture",K._indirectTexture,ee),At.setOptional(I,K,"batchingColorTexture"),K._colorsTexture!==null&&At.setValue(I,"batchingColorTexture",K._colorsTexture,ee));let Di=te.morphAttributes;if((Di.position!==void 0||Di.normal!==void 0||Di.color!==void 0)&&z.update(K,te,Pn),(Li||Ne.receiveShadow!==K.receiveShadow)&&(Ne.receiveShadow=K.receiveShadow,At.setValue(I,"receiveShadow",K.receiveShadow)),(Z.isMeshStandardMaterial||Z.isMeshLambertMaterial||Z.isMeshPhongMaterial)&&Z.envMap===null&&W.environment!==null&&(Ft.envMapIntensity.value=W.environmentIntensity),Ft.dfgLUT!==void 0&&(Ft.dfgLUT.value=nv()),Li){if(At.setValue(I,"toneMappingExposure",E.toneMappingExposure),Ne.needsLights&&ip(Ft,Cs),Le&&Z.fog===!0&&Oe.refreshFogUniforms(Ft,Le),Oe.refreshMaterialUniforms(Ft,Z,X,ge,y.state.transmissionRenderTarget[A.id]),Ne.needsLights&&Ne.lightProbeGrid){let Rt=Ne.lightProbeGrid;Ft.probesSH.value=Rt.texture,Ft.probesMin.value.copy(Rt.boundingBox.min),Ft.probesMax.value.copy(Rt.boundingBox.max),Ft.probesResolution.value.copy(Rt.resolution)}xr.upload(I,uu(Ne),Ft,ee)}if(Z.isShaderMaterial&&Z.uniformsNeedUpdate===!0&&(xr.upload(I,uu(Ne),Ft,ee),Z.uniformsNeedUpdate=!1),Z.isSpriteMaterial&&At.setValue(I,"center",K.center),At.setValue(I,"modelViewMatrix",K.modelViewMatrix),At.setValue(I,"normalMatrix",K.normalMatrix),At.setValue(I,"modelMatrix",K.matrixWorld),Z.uniformsGroups!==void 0){let Rt=Z.uniformsGroups;for(let Ni=0,Ps=Rt.length;Ni<Ps;Ni++){let du=Rt[Ni];de.update(du,Pn),de.bind(du,Pn)}}return Pn}function ip(A,W){A.ambientLightColor.needsUpdate=W,A.lightProbe.needsUpdate=W,A.directionalLights.needsUpdate=W,A.directionalLightShadows.needsUpdate=W,A.pointLights.needsUpdate=W,A.pointLightShadows.needsUpdate=W,A.spotLights.needsUpdate=W,A.spotLightShadows.needsUpdate=W,A.rectAreaLights.needsUpdate=W,A.hemisphereLights.needsUpdate=W}function sp(A){return A.isMeshLambertMaterial||A.isMeshToonMaterial||A.isMeshPhongMaterial||A.isMeshStandardMaterial||A.isShadowMaterial||A.isShaderMaterial&&A.lights===!0}this.getActiveCubeFace=function(){return V},this.getActiveMipmapLevel=function(){return F},this.getRenderTarget=function(){return Y},this.setRenderTargetTextures=function(A,W,te){let Z=G.get(A);Z.__autoAllocateDepthBuffer=A.resolveDepthBuffer===!1,Z.__autoAllocateDepthBuffer===!1&&(Z.__useRenderToTexture=!1),G.get(A.texture).__webglTexture=W,G.get(A.depthTexture).__webglTexture=Z.__autoAllocateDepthBuffer?void 0:te,Z.__hasExternalTextures=!0},this.setRenderTargetFramebuffer=function(A,W){let te=G.get(A);te.__webglFramebuffer=W,te.__useDefaultFramebuffer=W===void 0},this.setRenderTarget=function(A,W=0,te=0){Y=A,V=W,F=te;let Z=null,K=!1,Le=!1;if(A){let Ie=G.get(A);if(Ie.__useDefaultFramebuffer!==void 0){x.bindFramebuffer(I.FRAMEBUFFER,Ie.__webglFramebuffer),me.copy(A.viewport),_e.copy(A.scissor),Ve=A.scissorTest,x.viewport(me),x.scissor(_e),x.setScissorTest(Ve),J=-1;return}else if(Ie.__webglFramebuffer===void 0)ee.setupRenderTarget(A);else if(Ie.__hasExternalTextures)ee.rebindTextures(A,G.get(A.texture).__webglTexture,G.get(A.depthTexture).__webglTexture);else if(A.depthBuffer){let Qe=A.depthTexture;if(Ie.__boundDepthTexture!==Qe){if(Qe!==null&&G.has(Qe)&&(A.width!==Qe.image.width||A.height!==Qe.image.height))throw new Error("THREE.WebGLRenderer: Attached DepthTexture is initialized to the incorrect size.");ee.setupDepthRenderbuffer(A)}}let He=A.texture;(He.isData3DTexture||He.isDataArrayTexture||He.isCompressedArrayTexture)&&(Le=!0);let Ge=G.get(A).__webglFramebuffer;A.isWebGLCubeRenderTarget?(Array.isArray(Ge[W])?Z=Ge[W][te]:Z=Ge[W],K=!0):A.samples>0&&ee.useMultisampledRTT(A)===!1?Z=G.get(A).__webglMultisampledFramebuffer:Array.isArray(Ge)?Z=Ge[te]:Z=Ge,me.copy(A.viewport),_e.copy(A.scissor),Ve=A.scissorTest}else me.copy(ie).multiplyScalar(X).floor(),_e.copy(oe).multiplyScalar(X).floor(),Ve=ue;if(te!==0&&(Z=H),x.bindFramebuffer(I.FRAMEBUFFER,Z)&&x.drawBuffers(A,Z),x.viewport(me),x.scissor(_e),x.setScissorTest(Ve),K){let Ie=G.get(A.texture);I.framebufferTexture2D(I.FRAMEBUFFER,I.COLOR_ATTACHMENT0,I.TEXTURE_CUBE_MAP_POSITIVE_X+W,Ie.__webglTexture,te)}else if(Le){let Ie=W;for(let He=0;He<A.textures.length;He++){let Ge=G.get(A.textures[He]);I.framebufferTextureLayer(I.FRAMEBUFFER,I.COLOR_ATTACHMENT0+He,Ge.__webglTexture,te,Ie)}}else if(A!==null&&te!==0){let Ie=G.get(A.texture);I.framebufferTexture2D(I.FRAMEBUFFER,I.COLOR_ATTACHMENT0,I.TEXTURE_2D,Ie.__webglTexture,te)}J=-1},this.readRenderTargetPixels=function(A,W,te,Z,K,Le,Ue,Ie=0){if(!(A&&A.isWebGLRenderTarget)){Ze("WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let He=G.get(A).__webglFramebuffer;if(A.isWebGLCubeRenderTarget&&Ue!==void 0&&(He=He[Ue]),He){x.bindFramebuffer(I.FRAMEBUFFER,He);try{let Ge=A.textures[Ie],Qe=Ge.format,st=Ge.type;if(A.textures.length>1&&I.readBuffer(I.COLOR_ATTACHMENT0+Ie),!T.textureFormatReadable(Qe)){Ze("WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if(!T.textureTypeReadable(st)){Ze("WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}W>=0&&W<=A.width-Z&&te>=0&&te<=A.height-K&&I.readPixels(W,te,Z,K,we.convert(Qe),we.convert(st),Le)}finally{let Ge=Y!==null?G.get(Y).__webglFramebuffer:null;x.bindFramebuffer(I.FRAMEBUFFER,Ge)}}},this.readRenderTargetPixelsAsync=async function(A,W,te,Z,K,Le,Ue,Ie=0){if(!(A&&A.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let He=G.get(A).__webglFramebuffer;if(A.isWebGLCubeRenderTarget&&Ue!==void 0&&(He=He[Ue]),He)if(W>=0&&W<=A.width-Z&&te>=0&&te<=A.height-K){x.bindFramebuffer(I.FRAMEBUFFER,He);let Ge=A.textures[Ie],Qe=Ge.format,st=Ge.type;if(A.textures.length>1&&I.readBuffer(I.COLOR_ATTACHMENT0+Ie),!T.textureFormatReadable(Qe))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if(!T.textureTypeReadable(st))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");let Xe=I.createBuffer();I.bindBuffer(I.PIXEL_PACK_BUFFER,Xe),I.bufferData(I.PIXEL_PACK_BUFFER,Le.byteLength,I.STREAM_READ),I.readPixels(W,te,Z,K,we.convert(Qe),we.convert(st),0);let bt=Y!==null?G.get(Y).__webglFramebuffer:null;x.bindFramebuffer(I.FRAMEBUFFER,bt);let Ut=I.fenceSync(I.SYNC_GPU_COMMANDS_COMPLETE,0);return I.flush(),await kf(I,Ut,4),I.bindBuffer(I.PIXEL_PACK_BUFFER,Xe),I.getBufferSubData(I.PIXEL_PACK_BUFFER,0,Le),I.deleteBuffer(Xe),I.deleteSync(Ut),Le}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")},this.copyFramebufferToTexture=function(A,W=null,te=0){let Z=Math.pow(2,-te),K=Math.floor(A.image.width*Z),Le=Math.floor(A.image.height*Z),Ue=W!==null?W.x:0,Ie=W!==null?W.y:0;ee.setTexture2D(A,0),I.copyTexSubImage2D(I.TEXTURE_2D,te,0,0,Ue,Ie,K,Le),x.unbindTexture()},this.copyTextureToTexture=function(A,W,te=null,Z=null,K=0,Le=0){let Ue,Ie,He,Ge,Qe,st,Xe,bt,Ut,Dt=A.isCompressedTexture?A.mipmaps[Le]:A.image;if(te!==null)Ue=te.max.x-te.min.x,Ie=te.max.y-te.min.y,He=te.isBox3?te.max.z-te.min.z:1,Ge=te.min.x,Qe=te.min.y,st=te.isBox3?te.min.z:0;else{let Ft=Math.pow(2,-K);Ue=Math.floor(Dt.width*Ft),Ie=Math.floor(Dt.height*Ft),A.isDataArrayTexture?He=Dt.depth:A.isData3DTexture?He=Math.floor(Dt.depth*Ft):He=1,Ge=0,Qe=0,st=0}Z!==null?(Xe=Z.x,bt=Z.y,Ut=Z.z):(Xe=0,bt=0,Ut=0);let wt=we.convert(W.format),Qt=we.convert(W.type),Ne;W.isData3DTexture?(ee.setTexture3D(W,0),Ne=I.TEXTURE_3D):W.isDataArrayTexture||W.isCompressedArrayTexture?(ee.setTexture2DArray(W,0),Ne=I.TEXTURE_2D_ARRAY):(ee.setTexture2D(W,0),Ne=I.TEXTURE_2D),x.activeTexture(I.TEXTURE0),x.pixelStorei(I.UNPACK_FLIP_Y_WEBGL,W.flipY),x.pixelStorei(I.UNPACK_PREMULTIPLY_ALPHA_WEBGL,W.premultiplyAlpha),x.pixelStorei(I.UNPACK_ALIGNMENT,W.unpackAlignment);let yn=x.getParameter(I.UNPACK_ROW_LENGTH),dt=x.getParameter(I.UNPACK_IMAGE_HEIGHT),Pn=x.getParameter(I.UNPACK_SKIP_PIXELS),$n=x.getParameter(I.UNPACK_SKIP_ROWS),Li=x.getParameter(I.UNPACK_SKIP_IMAGES);x.pixelStorei(I.UNPACK_ROW_LENGTH,Dt.width),x.pixelStorei(I.UNPACK_IMAGE_HEIGHT,Dt.height),x.pixelStorei(I.UNPACK_SKIP_PIXELS,Ge),x.pixelStorei(I.UNPACK_SKIP_ROWS,Qe),x.pixelStorei(I.UNPACK_SKIP_IMAGES,st);let Cs=A.isDataArrayTexture||A.isData3DTexture,At=W.isDataArrayTexture||W.isData3DTexture;if(A.isDepthTexture){let Ft=G.get(A),Di=G.get(W),Rt=G.get(Ft.__renderTarget),Ni=G.get(Di.__renderTarget);x.bindFramebuffer(I.READ_FRAMEBUFFER,Rt.__webglFramebuffer),x.bindFramebuffer(I.DRAW_FRAMEBUFFER,Ni.__webglFramebuffer);for(let Ps=0;Ps<He;Ps++)Cs&&(I.framebufferTextureLayer(I.READ_FRAMEBUFFER,I.COLOR_ATTACHMENT0,G.get(A).__webglTexture,K,st+Ps),I.framebufferTextureLayer(I.DRAW_FRAMEBUFFER,I.COLOR_ATTACHMENT0,G.get(W).__webglTexture,Le,Ut+Ps)),I.blitFramebuffer(Ge,Qe,Ue,Ie,Xe,bt,Ue,Ie,I.DEPTH_BUFFER_BIT,I.NEAREST);x.bindFramebuffer(I.READ_FRAMEBUFFER,null),x.bindFramebuffer(I.DRAW_FRAMEBUFFER,null)}else if(K!==0||A.isRenderTargetTexture||G.has(A)){let Ft=G.get(A),Di=G.get(W);x.bindFramebuffer(I.READ_FRAMEBUFFER,O),x.bindFramebuffer(I.DRAW_FRAMEBUFFER,U);for(let Rt=0;Rt<He;Rt++)Cs?I.framebufferTextureLayer(I.READ_FRAMEBUFFER,I.COLOR_ATTACHMENT0,Ft.__webglTexture,K,st+Rt):I.framebufferTexture2D(I.READ_FRAMEBUFFER,I.COLOR_ATTACHMENT0,I.TEXTURE_2D,Ft.__webglTexture,K),At?I.framebufferTextureLayer(I.DRAW_FRAMEBUFFER,I.COLOR_ATTACHMENT0,Di.__webglTexture,Le,Ut+Rt):I.framebufferTexture2D(I.DRAW_FRAMEBUFFER,I.COLOR_ATTACHMENT0,I.TEXTURE_2D,Di.__webglTexture,Le),K!==0?I.blitFramebuffer(Ge,Qe,Ue,Ie,Xe,bt,Ue,Ie,I.COLOR_BUFFER_BIT,I.NEAREST):At?I.copyTexSubImage3D(Ne,Le,Xe,bt,Ut+Rt,Ge,Qe,Ue,Ie):I.copyTexSubImage2D(Ne,Le,Xe,bt,Ge,Qe,Ue,Ie);x.bindFramebuffer(I.READ_FRAMEBUFFER,null),x.bindFramebuffer(I.DRAW_FRAMEBUFFER,null)}else At?A.isDataTexture||A.isData3DTexture?I.texSubImage3D(Ne,Le,Xe,bt,Ut,Ue,Ie,He,wt,Qt,Dt.data):W.isCompressedArrayTexture?I.compressedTexSubImage3D(Ne,Le,Xe,bt,Ut,Ue,Ie,He,wt,Dt.data):I.texSubImage3D(Ne,Le,Xe,bt,Ut,Ue,Ie,He,wt,Qt,Dt):A.isDataTexture?I.texSubImage2D(I.TEXTURE_2D,Le,Xe,bt,Ue,Ie,wt,Qt,Dt.data):A.isCompressedTexture?I.compressedTexSubImage2D(I.TEXTURE_2D,Le,Xe,bt,Dt.width,Dt.height,wt,Dt.data):I.texSubImage2D(I.TEXTURE_2D,Le,Xe,bt,Ue,Ie,wt,Qt,Dt);x.pixelStorei(I.UNPACK_ROW_LENGTH,yn),x.pixelStorei(I.UNPACK_IMAGE_HEIGHT,dt),x.pixelStorei(I.UNPACK_SKIP_PIXELS,Pn),x.pixelStorei(I.UNPACK_SKIP_ROWS,$n),x.pixelStorei(I.UNPACK_SKIP_IMAGES,Li),Le===0&&W.generateMipmaps&&I.generateMipmap(Ne),x.unbindTexture()},this.initRenderTarget=function(A){G.get(A).__webglFramebuffer===void 0&&ee.setupRenderTarget(A)},this.initTexture=function(A){A.isCubeTexture?ee.setTextureCube(A,0):A.isData3DTexture?ee.setTexture3D(A,0):A.isDataArrayTexture||A.isCompressedArrayTexture?ee.setTexture2DArray(A,0):ee.setTexture2D(A,0),x.unbindTexture()},this.resetState=function(){V=0,F=0,Y=null,x.reset(),Pe.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return Gn}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(e){this._outputColorSpace=e;let t=this.getContext();t.drawingBufferColorSpace=et._getDrawingBufferColorSpace(e),t.unpackColorSpace=et._getUnpackColorSpace()}};function Eh(s,e){if(e===nh)return console.warn("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Geometry already defined as triangles."),s;if(e===gr||e===Co){let t=s.getIndex();if(t===null){let o=[],a=s.getAttribute("position");if(a!==void 0){for(let l=0;l<a.count;l++)o.push(l);s.setIndex(o),t=s.getIndex()}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Undefined position attribute. Processing not possible."),s}let n=t.count-2,i=[];if(e===gr)for(let o=1;o<=n;o++)i.push(t.getX(0)),i.push(t.getX(o)),i.push(t.getX(o+1));else for(let o=0;o<n;o++)o%2===0?(i.push(t.getX(o)),i.push(t.getX(o+1)),i.push(t.getX(o+2))):(i.push(t.getX(o+2)),i.push(t.getX(o+1)),i.push(t.getX(o)));i.length/3!==n&&console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unable to generate correct amount of triangles.");let r=s.clone();return r.setIndex(i),r.clearGroups(),r}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unknown draw mode:",e),s}function vd(s){let e=new Map,t=new Map,n=s.clone();return yd(s,n,function(i,r){e.set(r,i),t.set(i,r)}),n.traverse(function(i){if(!i.isSkinnedMesh)return;let r=i,o=e.get(i),a=o.skeleton.bones;r.skeleton=o.skeleton.clone(),r.bindMatrix.copy(o.bindMatrix),r.skeleton.bones=a.map(function(l){return t.get(l)}),r.bind(r.skeleton,r.bindMatrix)}),n}function yd(s,e,t){t(s,e);for(let n=0;n<s.children.length;n++)yd(s.children[n],e.children[n],t)}var ns=class extends ai{constructor(e){super(e),this.dracoLoader=null,this.ktx2Loader=null,this.meshoptDecoder=null,this.pluginCallbacks=[],this.register(function(t){return new Lh(t)}),this.register(function(t){return new Dh(t)}),this.register(function(t){return new Vh(t)}),this.register(function(t){return new Gh(t)}),this.register(function(t){return new Wh(t)}),this.register(function(t){return new Uh(t)}),this.register(function(t){return new Fh(t)}),this.register(function(t){return new Oh(t)}),this.register(function(t){return new Bh(t)}),this.register(function(t){return new Ih(t)}),this.register(function(t){return new zh(t)}),this.register(function(t){return new Nh(t)}),this.register(function(t){return new Hh(t)}),this.register(function(t){return new kh(t)}),this.register(function(t){return new Ch(t)}),this.register(function(t){return new Zl(t,at.EXT_MESHOPT_COMPRESSION)}),this.register(function(t){return new Zl(t,at.KHR_MESHOPT_COMPRESSION)}),this.register(function(t){return new Xh(t)})}load(e,t,n,i){let r=this,o;if(this.resourcePath!=="")o=this.resourcePath;else if(this.path!==""){let c=Ri.extractUrlBase(e);o=Ri.resolveURL(c,this.path)}else o=Ri.extractUrlBase(e);this.manager.itemStart(e);let a=function(c){i?i(c):console.error(c),r.manager.itemError(e),r.manager.itemEnd(e)},l=new hr(this.manager);l.setPath(this.path),l.setResponseType("arraybuffer"),l.setRequestHeader(this.requestHeader),l.setWithCredentials(this.withCredentials),l.load(e,function(c){try{r.parse(c,o,function(h){t(h),r.manager.itemEnd(e)},a)}catch(h){a(h)}},n,a)}setDRACOLoader(e){return this.dracoLoader=e,this}setKTX2Loader(e){return this.ktx2Loader=e,this}setMeshoptDecoder(e){return this.meshoptDecoder=e,this}register(e){return this.pluginCallbacks.indexOf(e)===-1&&this.pluginCallbacks.push(e),this}unregister(e){return this.pluginCallbacks.indexOf(e)!==-1&&this.pluginCallbacks.splice(this.pluginCallbacks.indexOf(e),1),this}parse(e,t,n,i){let r,o={},a={},l=new TextDecoder;if(typeof e=="string")r=JSON.parse(e);else if(e instanceof ArrayBuffer)if(l.decode(new Uint8Array(e,0,4))===Ed){try{o[at.KHR_BINARY_GLTF]=new qh(e)}catch(u){i&&i(u);return}r=JSON.parse(o[at.KHR_BINARY_GLTF].content)}else r=JSON.parse(l.decode(e));else r=e;if(r.asset===void 0||r.asset.version[0]<2){i&&i(new Error("THREE.GLTFLoader: Unsupported asset. glTF versions >=2.0 are supported."));return}let c=new Qh(r,{path:t||this.resourcePath||"",crossOrigin:this.crossOrigin,requestHeader:this.requestHeader,manager:this.manager,ktx2Loader:this.ktx2Loader,meshoptDecoder:this.meshoptDecoder});c.fileLoader.setRequestHeader(this.requestHeader);for(let h=0;h<this.pluginCallbacks.length;h++){let u=this.pluginCallbacks[h](c);u.name||console.error("THREE.GLTFLoader: Invalid plugin found: missing name"),a[u.name]=u,o[u.name]=!0}if(r.extensionsUsed)for(let h=0;h<r.extensionsUsed.length;++h){let u=r.extensionsUsed[h],f=r.extensionsRequired||[];switch(u){case at.KHR_MATERIALS_UNLIT:o[u]=new Ph;break;case at.KHR_DRACO_MESH_COMPRESSION:o[u]=new Yh(r,this.dracoLoader);break;case at.KHR_TEXTURE_TRANSFORM:o[u]=new Zh;break;case at.KHR_MESH_QUANTIZATION:o[u]=new Kh;break;default:f.indexOf(u)>=0&&a[u]===void 0&&console.warn('THREE.GLTFLoader: Unknown extension "'+u+'".')}}c.setExtensions(o),c.setPlugins(a),c.parse(n,i)}parseAsync(e,t){let n=this;return new Promise(function(i,r){n.parse(e,t,i,r)})}};function iv(){let s={};return{get:function(e){return s[e]},add:function(e,t){s[e]=t},remove:function(e){delete s[e]},removeAll:function(){s={}}}}function zt(s,e,t){let n=s.json.materials[e];return n.extensions&&n.extensions[t]?n.extensions[t]:null}var at={KHR_BINARY_GLTF:"KHR_binary_glTF",KHR_DRACO_MESH_COMPRESSION:"KHR_draco_mesh_compression",KHR_LIGHTS_PUNCTUAL:"KHR_lights_punctual",KHR_MATERIALS_CLEARCOAT:"KHR_materials_clearcoat",KHR_MATERIALS_DISPERSION:"KHR_materials_dispersion",KHR_MATERIALS_IOR:"KHR_materials_ior",KHR_MATERIALS_SHEEN:"KHR_materials_sheen",KHR_MATERIALS_SPECULAR:"KHR_materials_specular",KHR_MATERIALS_TRANSMISSION:"KHR_materials_transmission",KHR_MATERIALS_IRIDESCENCE:"KHR_materials_iridescence",KHR_MATERIALS_ANISOTROPY:"KHR_materials_anisotropy",KHR_MATERIALS_UNLIT:"KHR_materials_unlit",KHR_MATERIALS_VOLUME:"KHR_materials_volume",KHR_TEXTURE_BASISU:"KHR_texture_basisu",KHR_TEXTURE_TRANSFORM:"KHR_texture_transform",KHR_MESH_QUANTIZATION:"KHR_mesh_quantization",KHR_MATERIALS_EMISSIVE_STRENGTH:"KHR_materials_emissive_strength",EXT_MATERIALS_BUMP:"EXT_materials_bump",EXT_TEXTURE_WEBP:"EXT_texture_webp",EXT_TEXTURE_AVIF:"EXT_texture_avif",EXT_MESHOPT_COMPRESSION:"EXT_meshopt_compression",KHR_MESHOPT_COMPRESSION:"KHR_meshopt_compression",EXT_MESH_GPU_INSTANCING:"EXT_mesh_gpu_instancing"},Ch=class{constructor(e){this.parser=e,this.name=at.KHR_LIGHTS_PUNCTUAL,this.cache={refs:{},uses:{}}}_markDefs(){let e=this.parser,t=this.parser.json.nodes||[];for(let n=0,i=t.length;n<i;n++){let r=t[n];r.extensions&&r.extensions[this.name]&&r.extensions[this.name].light!==void 0&&e._addNodeRef(this.cache,r.extensions[this.name].light)}}_loadLight(e){let t=this.parser,n="light:"+e,i=t.cache.get(n);if(i)return i;let r=t.json,l=((r.extensions&&r.extensions[this.name]||{}).lights||[])[e],c,h=new Me(16777215);l.color!==void 0&&h.setRGB(l.color[0],l.color[1],l.color[2],cn);let u=l.range!==void 0?l.range:0;switch(l.type){case"directional":c=new Yi(h),c.target.position.set(0,0,-1),c.add(c.target);break;case"point":c=new Xn(h),c.distance=u;break;case"spot":c=new lo(h),c.distance=u,l.spot=l.spot||{},l.spot.innerConeAngle=l.spot.innerConeAngle!==void 0?l.spot.innerConeAngle:0,l.spot.outerConeAngle=l.spot.outerConeAngle!==void 0?l.spot.outerConeAngle:Math.PI/4,c.angle=l.spot.outerConeAngle,c.penumbra=1-l.spot.innerConeAngle/l.spot.outerConeAngle,c.target.position.set(0,0,-1),c.add(c.target);break;default:throw new Error("THREE.GLTFLoader: Unexpected light type: "+l.type)}return c.position.set(0,0,0),ui(c,l),l.intensity!==void 0&&(c.intensity=l.intensity),c.name=t.createUniqueName(l.name||"light_"+e),i=Promise.resolve(c),t.cache.add(n,i),i}getDependency(e,t){if(e==="light")return this._loadLight(t)}createNodeAttachment(e){let t=this,n=this.parser,r=n.json.nodes[e],a=(r.extensions&&r.extensions[this.name]||{}).light;return a===void 0?null:this._loadLight(a).then(function(l){return n._getNodeRef(t.cache,a,l)})}},Ph=class{constructor(){this.name=at.KHR_MATERIALS_UNLIT}getMaterialType(){return Tt}extendParams(e,t,n){let i=[];e.color=new Me(1,1,1),e.opacity=1;let r=t.pbrMetallicRoughness;if(r){if(Array.isArray(r.baseColorFactor)){let o=r.baseColorFactor;e.color.setRGB(o[0],o[1],o[2],cn),e.opacity=o[3]}r.baseColorTexture!==void 0&&i.push(n.assignTexture(e,"map",r.baseColorTexture,Pt))}return Promise.all(i)}},Ih=class{constructor(e){this.parser=e,this.name=at.KHR_MATERIALS_EMISSIVE_STRENGTH}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);return n===null||n.emissiveStrength!==void 0&&(t.emissiveIntensity=n.emissiveStrength),Promise.resolve()}},Lh=class{constructor(e){this.parser=e,this.name=at.KHR_MATERIALS_CLEARCOAT}getMaterialType(e){return zt(this.parser,e,this.name)!==null?pn:null}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(n.clearcoatFactor!==void 0&&(t.clearcoat=n.clearcoatFactor),n.clearcoatTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatMap",n.clearcoatTexture)),n.clearcoatRoughnessFactor!==void 0&&(t.clearcoatRoughness=n.clearcoatRoughnessFactor),n.clearcoatRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatRoughnessMap",n.clearcoatRoughnessTexture)),n.clearcoatNormalTexture!==void 0&&(i.push(this.parser.assignTexture(t,"clearcoatNormalMap",n.clearcoatNormalTexture)),n.clearcoatNormalTexture.scale!==void 0)){let r=n.clearcoatNormalTexture.scale;t.clearcoatNormalScale=new ye(r,r)}return Promise.all(i)}},Dh=class{constructor(e){this.parser=e,this.name=at.KHR_MATERIALS_DISPERSION}getMaterialType(e){return zt(this.parser,e,this.name)!==null?pn:null}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);return n===null||(t.dispersion=n.dispersion!==void 0?n.dispersion:0),Promise.resolve()}},Nh=class{constructor(e){this.parser=e,this.name=at.KHR_MATERIALS_IRIDESCENCE}getMaterialType(e){return zt(this.parser,e,this.name)!==null?pn:null}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.iridescenceFactor!==void 0&&(t.iridescence=n.iridescenceFactor),n.iridescenceTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceMap",n.iridescenceTexture)),n.iridescenceIor!==void 0&&(t.iridescenceIOR=n.iridescenceIor),t.iridescenceThicknessRange===void 0&&(t.iridescenceThicknessRange=[100,400]),n.iridescenceThicknessMinimum!==void 0&&(t.iridescenceThicknessRange[0]=n.iridescenceThicknessMinimum),n.iridescenceThicknessMaximum!==void 0&&(t.iridescenceThicknessRange[1]=n.iridescenceThicknessMaximum),n.iridescenceThicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceThicknessMap",n.iridescenceThicknessTexture)),Promise.all(i)}},Uh=class{constructor(e){this.parser=e,this.name=at.KHR_MATERIALS_SHEEN}getMaterialType(e){return zt(this.parser,e,this.name)!==null?pn:null}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(t.sheenColor=new Me(0,0,0),t.sheenRoughness=0,t.sheen=1,n.sheenColorFactor!==void 0){let r=n.sheenColorFactor;t.sheenColor.setRGB(r[0],r[1],r[2],cn)}return n.sheenRoughnessFactor!==void 0&&(t.sheenRoughness=n.sheenRoughnessFactor),n.sheenColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenColorMap",n.sheenColorTexture,Pt)),n.sheenRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenRoughnessMap",n.sheenRoughnessTexture)),Promise.all(i)}},Fh=class{constructor(e){this.parser=e,this.name=at.KHR_MATERIALS_TRANSMISSION}getMaterialType(e){return zt(this.parser,e,this.name)!==null?pn:null}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.transmissionFactor!==void 0&&(t.transmission=n.transmissionFactor),n.transmissionTexture!==void 0&&i.push(this.parser.assignTexture(t,"transmissionMap",n.transmissionTexture)),Promise.all(i)}},Oh=class{constructor(e){this.parser=e,this.name=at.KHR_MATERIALS_VOLUME}getMaterialType(e){return zt(this.parser,e,this.name)!==null?pn:null}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.thickness=n.thicknessFactor!==void 0?n.thicknessFactor:0,n.thicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"thicknessMap",n.thicknessTexture)),t.attenuationDistance=n.attenuationDistance||1/0;let r=n.attenuationColor||[1,1,1];return t.attenuationColor=new Me().setRGB(r[0],r[1],r[2],cn),Promise.all(i)}},Bh=class{constructor(e){this.parser=e,this.name=at.KHR_MATERIALS_IOR}getMaterialType(e){return zt(this.parser,e,this.name)!==null?pn:null}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);return n===null||(t.ior=n.ior!==void 0?n.ior:1.5,t.ior===0&&(t.ior=1e3)),Promise.resolve()}},zh=class{constructor(e){this.parser=e,this.name=at.KHR_MATERIALS_SPECULAR}getMaterialType(e){return zt(this.parser,e,this.name)!==null?pn:null}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.specularIntensity=n.specularFactor!==void 0?n.specularFactor:1,n.specularTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularIntensityMap",n.specularTexture));let r=n.specularColorFactor||[1,1,1];return t.specularColor=new Me().setRGB(r[0],r[1],r[2],cn),n.specularColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularColorMap",n.specularColorTexture,Pt)),Promise.all(i)}},kh=class{constructor(e){this.parser=e,this.name=at.EXT_MATERIALS_BUMP}getMaterialType(e){return zt(this.parser,e,this.name)!==null?pn:null}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return t.bumpScale=n.bumpFactor!==void 0?n.bumpFactor:1,n.bumpTexture!==void 0&&i.push(this.parser.assignTexture(t,"bumpMap",n.bumpTexture)),Promise.all(i)}},Hh=class{constructor(e){this.parser=e,this.name=at.KHR_MATERIALS_ANISOTROPY}getMaterialType(e){return zt(this.parser,e,this.name)!==null?pn:null}extendMaterialParams(e,t){let n=zt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.anisotropyStrength!==void 0&&(t.anisotropy=n.anisotropyStrength),n.anisotropyRotation!==void 0&&(t.anisotropyRotation=n.anisotropyRotation),n.anisotropyTexture!==void 0&&i.push(this.parser.assignTexture(t,"anisotropyMap",n.anisotropyTexture)),Promise.all(i)}},Vh=class{constructor(e){this.parser=e,this.name=at.KHR_TEXTURE_BASISU}loadTexture(e){let t=this.parser,n=t.json,i=n.textures[e];if(!i.extensions||!i.extensions[this.name])return null;let r=i.extensions[this.name],o=t.options.ktx2Loader;if(!o){if(n.extensionsRequired&&n.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setKTX2Loader must be called before loading KTX2 textures");return null}return t.loadTextureImage(e,r.source,o)}},Gh=class{constructor(e){this.parser=e,this.name=at.EXT_TEXTURE_WEBP}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let o=r.extensions[t],a=i.images[o.source],l=n.textureLoader;if(a.uri){let c=n.options.manager.getHandler(a.uri);c!==null&&(l=c)}return n.loadTextureImage(e,o.source,l)}},Wh=class{constructor(e){this.parser=e,this.name=at.EXT_TEXTURE_AVIF}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let o=r.extensions[t],a=i.images[o.source],l=n.textureLoader;if(a.uri){let c=n.options.manager.getHandler(a.uri);c!==null&&(l=c)}return n.loadTextureImage(e,o.source,l)}},Zl=class{constructor(e,t){this.name=t,this.parser=e}loadBufferView(e){let t=this.parser.json,n=t.bufferViews[e];if(n.extensions&&n.extensions[this.name]){let i=n.extensions[this.name],r=this.parser.getDependency("buffer",i.buffer),o=this.parser.options.meshoptDecoder;if(!o||!o.supported){if(t.extensionsRequired&&t.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setMeshoptDecoder must be called before loading compressed files");return null}return r.then(function(a){let l=i.byteOffset||0,c=i.byteLength||0,h=i.count,u=i.byteStride,f=new Uint8Array(a,l,c);return o.decodeGltfBufferAsync?o.decodeGltfBufferAsync(h,u,f,i.mode,i.filter).then(function(d){return d.buffer}):o.ready.then(function(){let d=new ArrayBuffer(h*u);return o.decodeGltfBuffer(new Uint8Array(d),h,u,f,i.mode,i.filter),d})})}else return null}},Xh=class{constructor(e){this.name=at.EXT_MESH_GPU_INSTANCING,this.parser=e}createNodeMesh(e){let t=this.parser.json,n=t.nodes[e];if(!n.extensions||!n.extensions[this.name]||n.mesh===void 0)return null;let i=t.meshes[n.mesh];for(let c of i.primitives)if(c.mode!==Bn.TRIANGLES&&c.mode!==Bn.TRIANGLE_STRIP&&c.mode!==Bn.TRIANGLE_FAN&&c.mode!==void 0)return null;let o=n.extensions[this.name].attributes,a=[],l={};for(let c in o)a.push(this.parser.getDependency("accessor",o[c]).then(h=>(l[c]=h,l[c])));return a.length<1?null:(a.push(this.parser.createNodeMesh(e)),Promise.all(a).then(c=>{let h=c.pop(),u=h.isGroup?h.children:[h],f=c[0].count,d=[];for(let g of u){let M=new Ke,m=new D,p=new It,S=new D(1,1,1),R=new fn(g.geometry,g.material,f);for(let b=0;b<f;b++)l.TRANSLATION&&m.fromBufferAttribute(l.TRANSLATION,b),l.ROTATION&&p.fromBufferAttribute(l.ROTATION,b),l.SCALE&&S.fromBufferAttribute(l.SCALE,b),R.setMatrixAt(b,M.compose(m,p,S));for(let b in l)if(b==="_COLOR_0"){let w=l[b];R.instanceColor=new Wi(w.array,w.itemSize,w.normalized)}else b!=="TRANSLATION"&&b!=="ROTATION"&&b!=="SCALE"&&g.geometry.setAttribute(b,l[b]);yt.prototype.copy.call(R,g),this.parser.assignFinalMaterial(R),d.push(R)}return h.isGroup?(h.clear(),h.add(...d),h):d[0]}))}},Ed="glTF",No=12,Md={JSON:1313821514,BIN:5130562},qh=class{constructor(e){this.name=at.KHR_BINARY_GLTF,this.content=null,this.body=null;let t=new DataView(e,0,No),n=new TextDecoder;if(this.header={magic:n.decode(new Uint8Array(e.slice(0,4))),version:t.getUint32(4,!0),length:t.getUint32(8,!0)},this.header.magic!==Ed)throw new Error("THREE.GLTFLoader: Unsupported glTF-Binary header.");if(this.header.version<2)throw new Error("THREE.GLTFLoader: Legacy binary file detected.");let i=this.header.length-No,r=new DataView(e,No),o=0;for(;o<i;){let a=r.getUint32(o,!0);o+=4;let l=r.getUint32(o,!0);if(o+=4,l===Md.JSON){let c=new Uint8Array(e,No+o,a);this.content=n.decode(c)}else if(l===Md.BIN){let c=No+o;this.body=e.slice(c,c+a)}o+=a}if(this.content===null)throw new Error("THREE.GLTFLoader: JSON content not found.")}},Yh=class{constructor(e,t){if(!t)throw new Error("THREE.GLTFLoader: No DRACOLoader instance provided.");this.name=at.KHR_DRACO_MESH_COMPRESSION,this.json=e,this.dracoLoader=t,this.dracoLoader.preload()}decodePrimitive(e,t){let n=this.json,i=this.dracoLoader,r=e.extensions[this.name].bufferView,o=e.extensions[this.name].attributes,a={},l={},c={};for(let h in o){let u=jh[h]||h.toLowerCase();a[u]=o[h]}for(let h in e.attributes){let u=jh[h]||h.toLowerCase();if(o[h]!==void 0){let f=n.accessors[e.attributes[h]],d=Mr[f.componentType];c[u]=d.name,l[u]=f.normalized===!0}}return t.getDependency("bufferView",r).then(function(h){return new Promise(function(u,f){i.decodeDracoFile(h,function(d){for(let g in d.attributes){let M=d.attributes[g],m=l[g];m!==void 0&&(M.normalized=m)}u(d)},a,c,cn,f)})})}},Zh=class{constructor(){this.name=at.KHR_TEXTURE_TRANSFORM}extendTexture(e,t){return(t.texCoord===void 0||t.texCoord===e.channel)&&t.offset===void 0&&t.rotation===void 0&&t.scale===void 0||(e=e.clone(),t.texCoord!==void 0&&(e.channel=t.texCoord),t.offset!==void 0&&e.offset.fromArray(t.offset),t.rotation!==void 0&&(e.rotation=t.rotation),t.scale!==void 0&&e.repeat.fromArray(t.scale),e.needsUpdate=!0),e}},Kh=class{constructor(){this.name=at.KHR_MESH_QUANTIZATION}},Kl=class extends oi{constructor(e,t,n,i){super(e,t,n,i)}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i*3+i;for(let o=0;o!==i;o++)t[o]=n[r+o];return t}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=a*2,c=a*3,h=i-t,u=(n-t)/h,f=u*u,d=f*u,g=e*c,M=g-c,m=-2*d+3*f,p=d-f,S=1-m,R=p-f+u;for(let b=0;b!==a;b++){let w=o[M+b+a],y=o[M+b+l]*h,C=o[g+b+a],_=o[g+b]*h;r[b]=S*w+R*y+m*C+p*_}return r}},sv=new It,Jh=class extends Kl{interpolate_(e,t,n,i){let r=super.interpolate_(e,t,n,i);return sv.fromArray(r).normalize().toArray(r),r}},Bn={FLOAT:5126,FLOAT_MAT3:35675,FLOAT_MAT4:35676,FLOAT_VEC2:35664,FLOAT_VEC3:35665,FLOAT_VEC4:35666,LINEAR:9729,REPEAT:10497,SAMPLER_2D:35678,POINTS:0,LINES:1,LINE_LOOP:2,LINE_STRIP:3,TRIANGLES:4,TRIANGLE_STRIP:5,TRIANGLE_FAN:6,UNSIGNED_BYTE:5121,UNSIGNED_SHORT:5123},Mr={5120:Int8Array,5121:Uint8Array,5122:Int16Array,5123:Uint16Array,5125:Uint32Array,5126:Float32Array},bd={9728:Ot,9729:Lt,9984:el,9985:dr,9986:Ts,9987:Yn},Sd={33071:Dn,33648:qs,10497:Gi},wh={SCALAR:1,VEC2:2,VEC3:3,VEC4:4,MAT2:4,MAT3:9,MAT4:16},jh={POSITION:"position",NORMAL:"normal",TANGENT:"tangent",TEXCOORD_0:"uv",TEXCOORD_1:"uv1",TEXCOORD_2:"uv2",TEXCOORD_3:"uv3",COLOR_0:"color",WEIGHTS_0:"skinWeight",JOINTS_0:"skinIndex"},ts={scale:"scale",translation:"position",rotation:"quaternion",weights:"morphTargetInfluences"},rv={CUBICSPLINE:void 0,LINEAR:ps,STEP:ds},Ah={OPAQUE:"OPAQUE",MASK:"MASK",BLEND:"BLEND"};function ov(s){return s.DefaultMaterial===void 0&&(s.DefaultMaterial=new dn({color:16777215,emissive:0,metalness:1,roughness:1,transparent:!1,depthTest:!0,side:Sn})),s.DefaultMaterial}function As(s,e,t){for(let n in t.extensions)s[n]===void 0&&(e.userData.gltfExtensions=e.userData.gltfExtensions||{},e.userData.gltfExtensions[n]=t.extensions[n])}function ui(s,e){e.extras!==void 0&&(typeof e.extras=="object"?Object.assign(s.userData,e.extras):console.warn("THREE.GLTFLoader: Ignoring primitive type .extras, "+e.extras))}function av(s,e,t){let n=!1,i=!1,r=!1;for(let c=0,h=e.length;c<h;c++){let u=e[c];if(u.POSITION!==void 0&&(n=!0),u.NORMAL!==void 0&&(i=!0),u.COLOR_0!==void 0&&(r=!0),n&&i&&r)break}if(!n&&!i&&!r)return Promise.resolve(s);let o=[],a=[],l=[];for(let c=0,h=e.length;c<h;c++){let u=e[c];if(n){let f=u.POSITION!==void 0?t.getDependency("accessor",u.POSITION):s.attributes.position;o.push(f)}if(i){let f=u.NORMAL!==void 0?t.getDependency("accessor",u.NORMAL):s.attributes.normal;a.push(f)}if(r){let f=u.COLOR_0!==void 0?t.getDependency("accessor",u.COLOR_0):s.attributes.color;l.push(f)}}return Promise.all([Promise.all(o),Promise.all(a),Promise.all(l)]).then(function(c){let h=c[0],u=c[1],f=c[2];return n&&(s.morphAttributes.position=h),i&&(s.morphAttributes.normal=u),r&&(s.morphAttributes.color=f),s.morphTargetsRelative=!0,s})}function lv(s,e){if(s.updateMorphTargets(),e.weights!==void 0)for(let t=0,n=e.weights.length;t<n;t++)s.morphTargetInfluences[t]=e.weights[t];if(e.extras&&Array.isArray(e.extras.targetNames)){let t=e.extras.targetNames;if(s.morphTargetInfluences.length===t.length){s.morphTargetDictionary={};for(let n=0,i=t.length;n<i;n++)s.morphTargetDictionary[t[n]]=n}else console.warn("THREE.GLTFLoader: Invalid extras.targetNames length. Ignoring names.")}}function cv(s){let e,t=s.extensions&&s.extensions[at.KHR_DRACO_MESH_COMPRESSION];if(t?e="draco:"+t.bufferView+":"+t.indices+":"+Rh(t.attributes):e=s.indices+":"+Rh(s.attributes)+":"+s.mode,s.targets!==void 0)for(let n=0,i=s.targets.length;n<i;n++)e+=":"+Rh(s.targets[n]);return e}function Rh(s){let e="",t=Object.keys(s).sort();for(let n=0,i=t.length;n<i;n++)e+=t[n]+":"+s[t[n]]+";";return e}function $h(s){switch(s){case Int8Array:return 1/127;case Uint8Array:return 1/255;case Int16Array:return 1/32767;case Uint16Array:return 1/65535;default:throw new Error("THREE.GLTFLoader: Unsupported normalized accessor component type.")}}function hv(s){return s.search(/\.jpe?g($|\?)/i)>0||s.search(/^data\:image\/jpeg/)===0?"image/jpeg":s.search(/\.webp($|\?)/i)>0||s.search(/^data\:image\/webp/)===0?"image/webp":s.search(/\.ktx2($|\?)/i)>0||s.search(/^data\:image\/ktx2/)===0?"image/ktx2":"image/png"}var uv=new Ke,Qh=class{constructor(e={},t={}){this.json=e,this.extensions={},this.plugins={},this.options=t,this.cache=new iv,this.associations=new Map,this.primitiveCache={},this.nodeCache={},this.meshCache={refs:{},uses:{}},this.cameraCache={refs:{},uses:{}},this.lightCache={refs:{},uses:{}},this.sourceCache={},this.textureCache={},this.nodeNamesUsed={};let n=!1,i=-1,r=!1,o=-1;if(typeof navigator<"u"&&typeof navigator.userAgent<"u"){let a=navigator.userAgent;n=/^((?!chrome|android).)*safari/i.test(a)===!0;let l=a.match(/Version\/(\d+)/);i=n&&l?parseInt(l[1],10):-1,r=a.indexOf("Firefox")>-1,o=r?a.match(/Firefox\/([0-9]+)\./)[1]:-1}typeof createImageBitmap>"u"||n&&i<17||r&&o<98?this.textureLoader=new so(this.options.manager):this.textureLoader=new co(this.options.manager),this.textureLoader.setCrossOrigin(this.options.crossOrigin),this.textureLoader.setRequestHeader(this.options.requestHeader),this.fileLoader=new hr(this.options.manager),this.fileLoader.setResponseType("arraybuffer"),this.options.crossOrigin==="use-credentials"&&this.fileLoader.setWithCredentials(!0)}setExtensions(e){this.extensions=e}setPlugins(e){this.plugins=e}parse(e,t){let n=this,i=this.json,r=this.extensions;this.cache.removeAll(),this.nodeCache={},this._invokeAll(function(o){return o._markDefs&&o._markDefs()}),Promise.all(this._invokeAll(function(o){return o.beforeRoot&&o.beforeRoot()})).then(function(){return Promise.all([n.getDependencies("scene"),n.getDependencies("animation"),n.getDependencies("camera")])}).then(function(o){let a={scene:o[0][i.scene||0],scenes:o[0],animations:o[1],cameras:o[2],asset:i.asset,parser:n,userData:{}};return As(r,a,i),ui(a,i),Promise.all(n._invokeAll(function(l){return l.afterRoot&&l.afterRoot(a)})).then(function(){for(let l of a.scenes)l.updateMatrixWorld();e(a)})}).catch(t)}_markDefs(){let e=this.json.nodes||[],t=this.json.skins||[],n=this.json.meshes||[];for(let i=0,r=t.length;i<r;i++){let o=t[i].joints;for(let a=0,l=o.length;a<l;a++)e[o[a]].isBone=!0}for(let i=0,r=e.length;i<r;i++){let o=e[i];o.mesh!==void 0&&(this._addNodeRef(this.meshCache,o.mesh),o.skin!==void 0&&(n[o.mesh].isSkinnedMesh=!0)),o.camera!==void 0&&this._addNodeRef(this.cameraCache,o.camera)}}_addNodeRef(e,t){t!==void 0&&(e.refs[t]===void 0&&(e.refs[t]=e.uses[t]=0),e.refs[t]++)}_getNodeRef(e,t,n){if(e.refs[t]<=1)return n;let i=n.clone(),r=(o,a)=>{let l=this.associations.get(o);l!=null&&this.associations.set(a,l);for(let[c,h]of o.children.entries())r(h,a.children[c])};return r(n,i),i.name+="_instance_"+e.uses[t]++,i}_invokeOne(e){let t=Object.values(this.plugins);t.push(this);for(let n=0;n<t.length;n++){let i=e(t[n]);if(i)return i}return null}_invokeAll(e){let t=Object.values(this.plugins);t.unshift(this);let n=[];for(let i=0;i<t.length;i++){let r=e(t[i]);r&&n.push(r)}return n}getDependency(e,t){let n=e+":"+t,i=this.cache.get(n);if(!i){switch(e){case"scene":i=this.loadScene(t);break;case"node":i=this._invokeOne(function(r){return r.loadNode&&r.loadNode(t)});break;case"mesh":i=this._invokeOne(function(r){return r.loadMesh&&r.loadMesh(t)});break;case"accessor":i=this.loadAccessor(t);break;case"bufferView":i=this._invokeOne(function(r){return r.loadBufferView&&r.loadBufferView(t)});break;case"buffer":i=this.loadBuffer(t);break;case"material":i=this._invokeOne(function(r){return r.loadMaterial&&r.loadMaterial(t)});break;case"texture":i=this._invokeOne(function(r){return r.loadTexture&&r.loadTexture(t)});break;case"skin":i=this.loadSkin(t);break;case"animation":i=this._invokeOne(function(r){return r.loadAnimation&&r.loadAnimation(t)});break;case"camera":i=this.loadCamera(t);break;default:if(i=this._invokeOne(function(r){return r!=this&&r.getDependency&&r.getDependency(e,t)}),!i)throw new Error("Unknown type: "+e);break}this.cache.add(n,i)}return i}getDependencies(e){let t=this.cache.get(e);if(!t){let n=this,i=this.json[e+(e==="mesh"?"es":"s")]||[];t=Promise.all(i.map(function(r,o){return n.getDependency(e,o)})),this.cache.add(e,t)}return t}loadBuffer(e){let t=this.json.buffers[e],n=this.fileLoader;if(t.type&&t.type!=="arraybuffer")throw new Error("THREE.GLTFLoader: "+t.type+" buffer type is not supported.");if(t.uri===void 0&&e===0)return Promise.resolve(this.extensions[at.KHR_BINARY_GLTF].body);let i=this.options;return new Promise(function(r,o){n.load(Ri.resolveURL(t.uri,i.path),r,void 0,function(){o(new Error('THREE.GLTFLoader: Failed to load buffer "'+t.uri+'".'))})})}loadBufferView(e){let t=this.json.bufferViews[e];return this.getDependency("buffer",t.buffer).then(function(n){let i=t.byteLength||0,r=t.byteOffset||0;return n.slice(r,r+i)})}loadAccessor(e){let t=this,n=this.json,i=this.json.accessors[e];if(i.bufferView===void 0&&i.sparse===void 0){let o=wh[i.type],a=Mr[i.componentType],l=i.normalized===!0,c=new a(i.count*o);return Promise.resolve(new vt(c,o,l))}let r=[];return i.bufferView!==void 0?r.push(this.getDependency("bufferView",i.bufferView)):r.push(null),i.sparse!==void 0&&(r.push(this.getDependency("bufferView",i.sparse.indices.bufferView)),r.push(this.getDependency("bufferView",i.sparse.values.bufferView))),Promise.all(r).then(function(o){let a=o[0],l=wh[i.type],c=Mr[i.componentType],h=c.BYTES_PER_ELEMENT,u=h*l,f=i.byteOffset||0,d=i.bufferView!==void 0?n.bufferViews[i.bufferView].byteStride:void 0,g=i.normalized===!0,M,m;if(d&&d!==u){let p=Math.floor(f/d),S="InterleavedBuffer:"+i.bufferView+":"+i.componentType+":"+p+":"+i.count,R=t.cache.get(S);R||(M=new c(a,p*d,i.count*d/h),R=new Qs(M,d/h),t.cache.add(S,R)),m=new er(R,l,f%d/h,g)}else a===null?M=new c(i.count*l):M=new c(a,f,i.count*l),m=new vt(M,l,g);if(i.sparse!==void 0){let p=wh.SCALAR,S=Mr[i.sparse.indices.componentType],R=i.sparse.indices.byteOffset||0,b=i.sparse.values.byteOffset||0,w=new S(o[1],R,i.sparse.count*p),y=new c(o[2],b,i.sparse.count*l);a!==null&&(m=new vt(m.array.slice(),m.itemSize,m.normalized)),m.normalized=!1;for(let C=0,_=w.length;C<_;C++){let v=w[C];if(m.setX(v,y[C*l]),l>=2&&m.setY(v,y[C*l+1]),l>=3&&m.setZ(v,y[C*l+2]),l>=4&&m.setW(v,y[C*l+3]),l>=5)throw new Error("THREE.GLTFLoader: Unsupported itemSize in sparse BufferAttribute.")}m.normalized=g}return m})}loadTexture(e){let t=this.json,n=this.options,r=t.textures[e].source,o=t.images[r],a=this.textureLoader;if(o.uri){let l=n.manager.getHandler(o.uri);l!==null&&(a=l)}return this.loadTextureImage(e,r,a)}loadTextureImage(e,t,n){let i=this,r=this.json,o=r.textures[e],a=r.images[t],l=(a.uri||a.bufferView)+":"+o.sampler;if(this.textureCache[l])return this.textureCache[l];let c=this.loadImageSource(t,n).then(function(h){h.flipY=!1,h.name=o.name||a.name||"",h.name===""&&typeof a.uri=="string"&&a.uri.startsWith("data:image/")===!1&&(h.name=a.uri);let f=(r.samplers||{})[o.sampler]||{};return h.magFilter=bd[f.magFilter]||Lt,h.minFilter=bd[f.minFilter]||Yn,h.wrapS=Sd[f.wrapS]||Gi,h.wrapT=Sd[f.wrapT]||Gi,h.generateMipmaps=!h.isCompressedTexture&&h.minFilter!==Ot&&h.minFilter!==Lt,i.associations.set(h,{textures:e}),h}).catch(function(){return null});return this.textureCache[l]=c,c}loadImageSource(e,t){let n=this,i=this.json,r=this.options;if(this.sourceCache[e]!==void 0)return this.sourceCache[e].then(u=>u.clone());let o=i.images[e],a=self.URL||self.webkitURL,l=o.uri||"",c=!1;if(o.bufferView!==void 0)l=n.getDependency("bufferView",o.bufferView).then(function(u){c=!0;let f=new Blob([u],{type:o.mimeType});return l=a.createObjectURL(f),l});else if(o.uri===void 0)throw new Error("THREE.GLTFLoader: Image "+e+" is missing URI and bufferView");let h=Promise.resolve(l).then(function(u){return new Promise(function(f,d){let g=f;t.isImageBitmapLoader===!0&&(g=function(M){let m=new Gt(M);m.needsUpdate=!0,f(m)}),t.load(Ri.resolveURL(u,r.path),g,void 0,d)})}).then(function(u){return c===!0&&a.revokeObjectURL(l),ui(u,o),u.userData.mimeType=o.mimeType||hv(o.uri),u}).catch(function(u){throw console.error("THREE.GLTFLoader: Couldn't load texture",l),u});return this.sourceCache[e]=h,h}assignTexture(e,t,n,i){let r=this;return this.getDependency("texture",n.index).then(function(o){if(!o)return null;if(n.texCoord!==void 0&&n.texCoord>0&&(o=o.clone(),o.channel=n.texCoord),r.extensions[at.KHR_TEXTURE_TRANSFORM]){let a=n.extensions!==void 0?n.extensions[at.KHR_TEXTURE_TRANSFORM]:void 0;if(a){let l=r.associations.get(o);o=r.extensions[at.KHR_TEXTURE_TRANSFORM].extendTexture(o,a),r.associations.set(o,l)}}return i!==void 0&&(o.colorSpace=i),e[t]=o,o})}assignFinalMaterial(e){let t=e.geometry,n=e.material,i=t.attributes.tangent===void 0,r=t.attributes.color!==void 0,o=t.attributes.normal===void 0;if(e.isPoints){let a="PointsMaterial:"+n.uuid,l=this.cache.get(a);l||(l=new sr,un.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,l.sizeAttenuation=!1,this.cache.add(a,l)),n=l}else if(e.isLine){let a="LineBasicMaterial:"+n.uuid,l=this.cache.get(a);l||(l=new ii,un.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,this.cache.add(a,l)),n=l}if(i||r||o){let a="ClonedMaterial:"+n.uuid+":";i&&(a+="derivative-tangents:"),r&&(a+="vertex-colors:"),o&&(a+="flat-shading:");let l=this.cache.get(a);l||(l=n.clone(),r&&(l.vertexColors=!0),o&&(l.flatShading=!0),i&&(l.normalScale&&(l.normalScale.y*=-1),l.clearcoatNormalScale&&(l.clearcoatNormalScale.y*=-1)),this.cache.add(a,l),this.associations.set(l,this.associations.get(n))),n=l}e.material=n}getMaterialType(){return dn}loadMaterial(e){let t=this,n=this.json,i=this.extensions,r=n.materials[e],o,a={},l=r.extensions||{},c=[];if(l[at.KHR_MATERIALS_UNLIT]){let u=i[at.KHR_MATERIALS_UNLIT];o=u.getMaterialType(),c.push(u.extendParams(a,r,t))}else{let u=r.pbrMetallicRoughness||{};if(a.color=new Me(1,1,1),a.opacity=1,Array.isArray(u.baseColorFactor)){let f=u.baseColorFactor;a.color.setRGB(f[0],f[1],f[2],cn),a.opacity=f[3]}u.baseColorTexture!==void 0&&c.push(t.assignTexture(a,"map",u.baseColorTexture,Pt)),a.metalness=u.metallicFactor!==void 0?u.metallicFactor:1,a.roughness=u.roughnessFactor!==void 0?u.roughnessFactor:1,u.metallicRoughnessTexture!==void 0&&(c.push(t.assignTexture(a,"metalnessMap",u.metallicRoughnessTexture)),c.push(t.assignTexture(a,"roughnessMap",u.metallicRoughnessTexture))),o=this._invokeOne(function(f){return f.getMaterialType&&f.getMaterialType(e)}),c.push(Promise.all(this._invokeAll(function(f){return f.extendMaterialParams&&f.extendMaterialParams(e,a)})))}r.doubleSided===!0&&(a.side=Bt);let h=r.alphaMode||Ah.OPAQUE;if(h===Ah.BLEND?(a.transparent=!0,a.depthWrite=!1):(a.transparent=!1,h===Ah.MASK&&(a.alphaTest=r.alphaCutoff!==void 0?r.alphaCutoff:.5)),r.normalTexture!==void 0&&o!==Tt&&(c.push(t.assignTexture(a,"normalMap",r.normalTexture)),a.normalScale=new ye(1,1),r.normalTexture.scale!==void 0)){let u=r.normalTexture.scale;a.normalScale.set(u,u)}if(r.occlusionTexture!==void 0&&o!==Tt&&(c.push(t.assignTexture(a,"aoMap",r.occlusionTexture)),r.occlusionTexture.strength!==void 0&&(a.aoMapIntensity=r.occlusionTexture.strength)),r.emissiveFactor!==void 0&&o!==Tt){let u=r.emissiveFactor;a.emissive=new Me().setRGB(u[0],u[1],u[2],cn)}return r.emissiveTexture!==void 0&&o!==Tt&&c.push(t.assignTexture(a,"emissiveMap",r.emissiveTexture,Pt)),Promise.all(c).then(function(){let u=new o(a);return r.name&&(u.name=r.name),ui(u,r),t.associations.set(u,{materials:e}),r.extensions&&As(i,u,r),u})}createUniqueName(e){let t=St.sanitizeNodeName(e||"");return t in this.nodeNamesUsed?t+"_"+ ++this.nodeNamesUsed[t]:(this.nodeNamesUsed[t]=0,t)}loadGeometries(e){let t=this,n=this.extensions,i=this.primitiveCache;function r(a){return n[at.KHR_DRACO_MESH_COMPRESSION].decodePrimitive(a,t).then(function(l){return Td(l,a,t)})}let o=[];for(let a=0,l=e.length;a<l;a++){let c=e[a],h=cv(c),u=i[h];if(u)o.push(u.promise);else{let f;c.extensions&&c.extensions[at.KHR_DRACO_MESH_COMPRESSION]?f=r(c):f=Td(new ft,c,t),i[h]={primitive:c,promise:f},o.push(f)}}return Promise.all(o)}loadMesh(e){let t=this,n=this.json,i=this.extensions,r=n.meshes[e],o=r.primitives,a=[];for(let l=0,c=o.length;l<c;l++){let h=o[l].material===void 0?ov(this.cache):this.getDependency("material",o[l].material);a.push(h)}return a.push(t.loadGeometries(o)),Promise.all(a).then(function(l){let c=l.slice(0,l.length-1),h=l[l.length-1],u=[];for(let d=0,g=h.length;d<g;d++){let M=h[d],m=o[d],p,S=c[d];if(m.mode===Bn.TRIANGLES||m.mode===Bn.TRIANGLE_STRIP||m.mode===Bn.TRIANGLE_FAN||m.mode===void 0)p=r.isSkinnedMesh===!0?new qr(M,S):new Je(M,S),p.isSkinnedMesh===!0&&p.normalizeSkinWeights(),m.mode===Bn.TRIANGLE_STRIP?p.geometry=Eh(p.geometry,Co):m.mode===Bn.TRIANGLE_FAN&&(p.geometry=Eh(p.geometry,gr));else if(m.mode===Bn.LINES)p=new _s(M,S);else if(m.mode===Bn.LINE_STRIP)p=new Mi(M,S);else if(m.mode===Bn.LINE_LOOP)p=new Zr(M,S);else if(m.mode===Bn.POINTS)p=new Fn(M,S);else throw new Error("THREE.GLTFLoader: Primitive mode unsupported: "+m.mode);Object.keys(p.geometry.morphAttributes).length>0&&lv(p,r),p.name=t.createUniqueName(r.name||"mesh_"+e),ui(p,r),m.extensions&&As(i,p,m),t.assignFinalMaterial(p),u.push(p)}for(let d=0,g=u.length;d<g;d++)t.associations.set(u[d],{meshes:e,primitives:d});if(u.length===1)return r.extensions&&As(i,u[0],r),u[0];let f=new ut;r.extensions&&As(i,f,r),t.associations.set(f,{meshes:e});for(let d=0,g=u.length;d<g;d++)f.add(u[d]);return f})}loadCamera(e){let t,n=this.json.cameras[e],i=n[n.type];if(!i){console.warn("THREE.GLTFLoader: Missing camera parameters.");return}return n.type==="perspective"?t=new Ht(Vt.radToDeg(i.yfov),i.aspectRatio||1,i.znear||1,i.zfar||2e6):n.type==="orthographic"&&(t=new li(-i.xmag,i.xmag,i.ymag,-i.ymag,i.znear,i.zfar)),n.name&&(t.name=this.createUniqueName(n.name)),ui(t,n),Promise.resolve(t)}loadSkin(e){let t=this.json.skins[e],n=[];for(let i=0,r=t.joints.length;i<r;i++)n.push(this._loadNodeShallow(t.joints[i]));return t.inverseBindMatrices!==void 0?n.push(this.getDependency("accessor",t.inverseBindMatrices)):n.push(null),Promise.all(n).then(function(i){let r=i.pop(),o=i,a=[],l=[];for(let c=0,h=o.length;c<h;c++){let u=o[c];if(u){a.push(u);let f=new Ke;r!==null&&f.fromArray(r.array,c*16),l.push(f)}else console.warn('THREE.GLTFLoader: Joint "%s" could not be found.',t.joints[c])}return new Yr(a,l)})}loadAnimation(e){let t=this.json,n=this,i=t.animations[e],r=i.name?i.name:"animation_"+e,o=[],a=[],l=[],c=[],h=[];for(let u=0,f=i.channels.length;u<f;u++){let d=i.channels[u],g=i.samplers[d.sampler],M=d.target,m=M.node,p=i.parameters!==void 0?i.parameters[g.input]:g.input,S=i.parameters!==void 0?i.parameters[g.output]:g.output;M.node!==void 0&&(o.push(this.getDependency("node",m)),a.push(this.getDependency("accessor",p)),l.push(this.getDependency("accessor",S)),c.push(g),h.push(M))}return Promise.all([Promise.all(o),Promise.all(a),Promise.all(l),Promise.all(c),Promise.all(h)]).then(function(u){let f=u[0],d=u[1],g=u[2],M=u[3],m=u[4],p=[];for(let R=0,b=f.length;R<b;R++){let w=f[R],y=d[R],C=g[R],_=M[R],v=m[R];if(w===void 0)continue;w.updateMatrix&&w.updateMatrix();let E=n._createAnimationTracks(w,y,C,_,v);if(E)for(let P=0;P<E.length;P++)p.push(E[P])}let S=new ys(r,void 0,p);return ui(S,i),S})}createNodeMesh(e){let t=this.json,n=this,i=t.nodes[e];return i.mesh===void 0?null:n.getDependency("mesh",i.mesh).then(function(r){let o=n._getNodeRef(n.meshCache,i.mesh,r);return i.weights!==void 0&&o.traverse(function(a){if(a.isMesh)for(let l=0,c=i.weights.length;l<c;l++)a.morphTargetInfluences[l]=i.weights[l]}),o})}loadNode(e){let t=this.json,n=this,i=t.nodes[e],r=n._loadNodeShallow(e),o=[],a=i.children||[];for(let c=0,h=a.length;c<h;c++)o.push(n.getDependency("node",a[c]));let l=i.skin===void 0?Promise.resolve(null):n.getDependency("skin",i.skin);return Promise.all([r,Promise.all(o),l]).then(function(c){let h=c[0],u=c[1],f=c[2];f!==null&&h.traverse(function(d){d.isSkinnedMesh&&d.bind(f,uv)});for(let d=0,g=u.length;d<g;d++)h.add(u[d]);if(h.userData.pivot!==void 0&&u.length>0){let d=h.userData.pivot,g=u[0];h.pivot=new D().fromArray(d),h.position.x-=d[0],h.position.y-=d[1],h.position.z-=d[2],g.position.set(0,0,0),delete h.userData.pivot}return h})}_loadNodeShallow(e){let t=this.json,n=this.extensions,i=this;if(this.nodeCache[e]!==void 0)return this.nodeCache[e];let r=t.nodes[e],o=r.name?i.createUniqueName(r.name):"",a=[],l=i._invokeOne(function(c){return c.createNodeMesh&&c.createNodeMesh(e)});return l&&a.push(l),r.camera!==void 0&&a.push(i.getDependency("camera",r.camera).then(function(c){return i._getNodeRef(i.cameraCache,r.camera,c)})),i._invokeAll(function(c){return c.createNodeAttachment&&c.createNodeAttachment(e)}).forEach(function(c){a.push(c)}),this.nodeCache[e]=Promise.all(a).then(function(c){let h;if(r.isBone===!0?h=new tr:c.length>1?h=new ut:c.length===1?h=c[0]:h=new yt,h!==c[0])for(let u=0,f=c.length;u<f;u++)h.add(c[u]);if(r.name&&(h.userData.name=r.name,h.name=o),ui(h,r),r.extensions&&As(n,h,r),r.matrix!==void 0){let u=new Ke;u.fromArray(r.matrix),h.applyMatrix4(u)}else r.translation!==void 0&&h.position.fromArray(r.translation),r.rotation!==void 0&&h.quaternion.fromArray(r.rotation),r.scale!==void 0&&h.scale.fromArray(r.scale);if(!i.associations.has(h))i.associations.set(h,{});else if(r.mesh!==void 0&&i.meshCache.refs[r.mesh]>1){let u=i.associations.get(h);i.associations.set(h,{...u})}return i.associations.get(h).nodes=e,h}),this.nodeCache[e]}loadScene(e){let t=this.extensions,n=this.json.scenes[e],i=this,r=new ut;n.name&&(r.name=i.createUniqueName(n.name)),ui(r,n),n.extensions&&As(t,r,n);let o=n.nodes||[],a=[];for(let l=0,c=o.length;l<c;l++)a.push(i.getDependency("node",o[l]));return Promise.all(a).then(function(l){for(let h=0,u=l.length;h<u;h++){let f=l[h];f.parent!==null?r.add(vd(f)):r.add(f)}let c=h=>{let u=new Map;for(let[f,d]of i.associations)(f instanceof un||f instanceof Gt)&&u.set(f,d);return h.traverse(f=>{let d=i.associations.get(f);d!=null&&u.set(f,d)}),u};return i.associations=c(r),r})}_createAnimationTracks(e,t,n,i,r){let o=[],a=e.name?e.name:e.uuid,l=[];function c(d){d.morphTargetInfluences&&l.push(d.name?d.name:d.uuid)}ts[r.path]===ts.weights?(c(e),e.isGroup&&e.children.forEach(c)):l.push(a);let h;switch(ts[r.path]){case ts.weights:h=Ei;break;case ts.rotation:h=wi;break;case ts.translation:case ts.scale:h=qi;break;default:switch(n.itemSize){case 1:h=Ei;break;case 2:case 3:default:h=qi;break}break}let u=i.interpolation!==void 0?rv[i.interpolation]:ps,f=this._getArrayFromAccessor(n);for(let d=0,g=l.length;d<g;d++){let M=new h(l[d]+"."+ts[r.path],t.array,f,u);i.interpolation==="CUBICSPLINE"&&this._createCubicSplineTrackInterpolant(M),o.push(M)}return o}_getArrayFromAccessor(e){let t=e.array;if(e.normalized){let n=$h(t.constructor),i=new Float32Array(t.length);for(let r=0,o=t.length;r<o;r++)i[r]=t[r]*n;t=i}return t}_createCubicSplineTrackInterpolant(e){e.createInterpolant=function(n){let i=this instanceof wi?Jh:Kl;return new i(this.times,this.values,this.getValueSize()/3,n)},e.createInterpolant.isInterpolantFactoryMethodGLTFCubicSpline=!0}};function fv(s,e,t){let n=e.attributes,i=new hn;if(n.POSITION!==void 0){let a=t.json.accessors[n.POSITION],l=a.min,c=a.max;if(l!==void 0&&c!==void 0){if(i.set(new D(l[0],l[1],l[2]),new D(c[0],c[1],c[2])),a.normalized){let h=$h(Mr[a.componentType]);i.min.multiplyScalar(h),i.max.multiplyScalar(h)}}else{console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.");return}}else return;let r=e.targets;if(r!==void 0){let a=new D,l=new D;for(let c=0,h=r.length;c<h;c++){let u=r[c];if(u.POSITION!==void 0){let f=t.json.accessors[u.POSITION],d=f.min,g=f.max;if(d!==void 0&&g!==void 0){if(l.setX(Math.max(Math.abs(d[0]),Math.abs(g[0]))),l.setY(Math.max(Math.abs(d[1]),Math.abs(g[1]))),l.setZ(Math.max(Math.abs(d[2]),Math.abs(g[2]))),f.normalized){let M=$h(Mr[f.componentType]);l.multiplyScalar(M)}a.max(l)}else console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.")}}i.expandByVector(a)}s.boundingBox=i;let o=new nn;i.getCenter(o.center),o.radius=i.min.distanceTo(i.max)/2,s.boundingSphere=o}function Td(s,e,t){let n=e.attributes,i=[];function r(o,a){return t.getDependency("accessor",o).then(function(l){s.setAttribute(a,l)})}for(let o in n){let a=jh[o]||o.toLowerCase();a in s.attributes||i.push(r(n[o],a))}if(e.indices!==void 0&&!s.index){let o=t.getDependency("accessor",e.indices).then(function(a){s.setIndex(a)});i.push(o)}return et.workingColorSpace!==cn&&"COLOR_0"in n&&console.warn(`THREE.GLTFLoader: Converting vertex colors from "srgb-linear" to "${et.workingColorSpace}" not supported.`),ui(s,e),fv(s,e,t),Promise.all(i).then(function(){return e.targets!==void 0?av(s,e.targets,t):s})}var wd={type:"change"},tu={type:"start"},Rd={type:"end"},Jl=new ni,Ad=new Ln,dv=Math.cos(70*Vt.DEG2RAD),Zt=new D,_n=2*Math.PI,Et={NONE:-1,ROTATE:0,DOLLY:1,PAN:2,TOUCH_ROTATE:3,TOUCH_PAN:4,TOUCH_DOLLY_PAN:5,TOUCH_DOLLY_ROTATE:6},eu=1e-6,jl=class extends po{constructor(e,t=null){super(e,t),this.state=Et.NONE,this.target=new D,this.cursor=new D,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.minTargetRadius=0,this.maxTargetRadius=1/0,this.minPolarAngle=0,this.maxPolarAngle=Math.PI,this.minAzimuthAngle=-1/0,this.maxAzimuthAngle=1/0,this.enableDamping=!1,this.dampingFactor=.05,this.enableZoom=!0,this.zoomSpeed=1,this.enableRotate=!0,this.rotateSpeed=1,this.keyRotateSpeed=1,this.enablePan=!0,this.panSpeed=1,this.screenSpacePanning=!0,this.keyPanSpeed=7,this.zoomToCursor=!1,this.autoRotate=!1,this.autoRotateSpeed=2,this.keys={LEFT:"ArrowLeft",UP:"ArrowUp",RIGHT:"ArrowRight",BOTTOM:"ArrowDown"},this.mouseButtons={LEFT:Zi.ROTATE,MIDDLE:Zi.DOLLY,RIGHT:Zi.PAN},this.touches={ONE:Ki.ROTATE,TWO:Ki.DOLLY_PAN},this.target0=this.target.clone(),this.position0=this.object.position.clone(),this.zoom0=this.object.zoom,this._cursorStyle="auto",this._domElementKeyEvents=null,this._lastPosition=new D,this._lastQuaternion=new It,this._lastTargetPosition=new D,this._quat=new It().setFromUnitVectors(e.up,new D(0,1,0)),this._quatInverse=this._quat.clone().invert(),this._spherical=new ur,this._sphericalDelta=new ur,this._scale=1,this._panOffset=new D,this._rotateStart=new ye,this._rotateEnd=new ye,this._rotateDelta=new ye,this._panStart=new ye,this._panEnd=new ye,this._panDelta=new ye,this._dollyStart=new ye,this._dollyEnd=new ye,this._dollyDelta=new ye,this._dollyDirection=new D,this._mouse=new ye,this._performCursorZoom=!1,this._pointers=[],this._pointerPositions={},this._controlActive=!1,this._onPointerMove=mv.bind(this),this._onPointerDown=pv.bind(this),this._onPointerUp=gv.bind(this),this._onContextMenu=Sv.bind(this),this._onMouseWheel=vv.bind(this),this._onKeyDown=yv.bind(this),this._onTouchStart=Mv.bind(this),this._onTouchMove=bv.bind(this),this._onMouseDown=_v.bind(this),this._onMouseMove=xv.bind(this),this._interceptControlDown=Tv.bind(this),this._interceptControlUp=Ev.bind(this),this.domElement!==null&&this.connect(this.domElement),this.update()}set cursorStyle(e){this._cursorStyle=e,e==="grab"?this.domElement.style.cursor="grab":this.domElement.style.cursor="auto"}get cursorStyle(){return this._cursorStyle}connect(e){super.connect(e),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerUp),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.getRootNode().addEventListener("keydown",this._interceptControlDown,{passive:!0,capture:!0}),this.domElement.style.touchAction="none"}disconnect(){this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerUp),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.stopListenToKeyEvents(),this.domElement.getRootNode().removeEventListener("keydown",this._interceptControlDown,{capture:!0}),this.domElement.style.touchAction=""}dispose(){this.disconnect()}getPolarAngle(){return this._spherical.phi}getAzimuthalAngle(){return this._spherical.theta}getDistance(){return this.object.position.distanceTo(this.target)}listenToKeyEvents(e){e.addEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=e}stopListenToKeyEvents(){this._domElementKeyEvents!==null&&(this._domElementKeyEvents.removeEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=null)}saveState(){this.target0.copy(this.target),this.position0.copy(this.object.position),this.zoom0=this.object.zoom}reset(){this.target.copy(this.target0),this.object.position.copy(this.position0),this.object.zoom=this.zoom0,this.object.updateProjectionMatrix(),this.dispatchEvent(wd),this.update(),this.state=Et.NONE}pan(e,t){this._pan(e,t),this.update()}dollyIn(e){this._dollyIn(e),this.update()}dollyOut(e){this._dollyOut(e),this.update()}rotateLeft(e){this._rotateLeft(e),this.update()}rotateUp(e){this._rotateUp(e),this.update()}update(e=null){let t=this.object.position;Zt.copy(t).sub(this.target),Zt.applyQuaternion(this._quat),this._spherical.setFromVector3(Zt),this.autoRotate&&this.state===Et.NONE&&this._rotateLeft(this._getAutoRotationAngle(e)),this.enableDamping?(this._spherical.theta+=this._sphericalDelta.theta*this.dampingFactor,this._spherical.phi+=this._sphericalDelta.phi*this.dampingFactor):(this._spherical.theta+=this._sphericalDelta.theta,this._spherical.phi+=this._sphericalDelta.phi);let n=this.minAzimuthAngle,i=this.maxAzimuthAngle;isFinite(n)&&isFinite(i)&&(n<-Math.PI?n+=_n:n>Math.PI&&(n-=_n),i<-Math.PI?i+=_n:i>Math.PI&&(i-=_n),n<=i?this._spherical.theta=Math.max(n,Math.min(i,this._spherical.theta)):this._spherical.theta=this._spherical.theta>(n+i)/2?Math.max(n,this._spherical.theta):Math.min(i,this._spherical.theta)),this._spherical.phi=Math.max(this.minPolarAngle,Math.min(this.maxPolarAngle,this._spherical.phi)),this._spherical.makeSafe(),this.enableDamping===!0?this.target.addScaledVector(this._panOffset,this.dampingFactor):this.target.add(this._panOffset),this.target.sub(this.cursor),this.target.clampLength(this.minTargetRadius,this.maxTargetRadius),this.target.add(this.cursor);let r=!1;if(this.zoomToCursor&&this._performCursorZoom||this.object.isOrthographicCamera)this._spherical.radius=this._clampDistance(this._spherical.radius);else{let o=this._spherical.radius;this._spherical.radius=this._clampDistance(this._spherical.radius*this._scale),r=o!=this._spherical.radius}if(Zt.setFromSpherical(this._spherical),Zt.applyQuaternion(this._quatInverse),t.copy(this.target).add(Zt),this.object.lookAt(this.target),this.enableDamping===!0?(this._sphericalDelta.theta*=1-this.dampingFactor,this._sphericalDelta.phi*=1-this.dampingFactor,this._panOffset.multiplyScalar(1-this.dampingFactor)):(this._sphericalDelta.set(0,0,0),this._panOffset.set(0,0,0)),this.zoomToCursor&&this._performCursorZoom){let o=null;if(this.object.isPerspectiveCamera){let a=Zt.length();o=this._clampDistance(a*this._scale);let l=a-o;this.object.position.addScaledVector(this._dollyDirection,l),this.object.updateMatrixWorld(),r=!!l}else if(this.object.isOrthographicCamera){let a=new D(this._mouse.x,this._mouse.y,0);a.unproject(this.object);let l=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),this.object.updateProjectionMatrix(),r=l!==this.object.zoom;let c=new D(this._mouse.x,this._mouse.y,0);c.unproject(this.object),this.object.position.sub(c).add(a),this.object.updateMatrixWorld(),o=Zt.length()}else console.warn("WARNING: OrbitControls.js encountered an unknown camera type - zoom to cursor disabled."),this.zoomToCursor=!1;o!==null&&(this.screenSpacePanning?this.target.set(0,0,-1).transformDirection(this.object.matrix).multiplyScalar(o).add(this.object.position):(Jl.origin.copy(this.object.position),Jl.direction.set(0,0,-1).transformDirection(this.object.matrix),Math.abs(this.object.up.dot(Jl.direction))<dv?this.object.lookAt(this.target):(Ad.setFromNormalAndCoplanarPoint(this.object.up,this.target),Jl.intersectPlane(Ad,this.target))))}else if(this.object.isOrthographicCamera){let o=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),o!==this.object.zoom&&(this.object.updateProjectionMatrix(),r=!0)}return this._scale=1,this._performCursorZoom=!1,r||this._lastPosition.distanceToSquared(this.object.position)>eu||8*(1-this._lastQuaternion.dot(this.object.quaternion))>eu||this._lastTargetPosition.distanceToSquared(this.target)>eu?(this.dispatchEvent(wd),this._lastPosition.copy(this.object.position),this._lastQuaternion.copy(this.object.quaternion),this._lastTargetPosition.copy(this.target),!0):!1}_getAutoRotationAngle(e){return e!==null?_n/60*this.autoRotateSpeed*e:_n/60/60*this.autoRotateSpeed}_getZoomScale(e){let t=Math.abs(e*.01);return Math.pow(.95,this.zoomSpeed*t)}_rotateLeft(e){this._sphericalDelta.theta-=e}_rotateUp(e){this._sphericalDelta.phi-=e}_panLeft(e,t){Zt.setFromMatrixColumn(t,0),Zt.multiplyScalar(-e),this._panOffset.add(Zt)}_panUp(e,t){this.screenSpacePanning===!0?Zt.setFromMatrixColumn(t,1):(Zt.setFromMatrixColumn(t,0),Zt.crossVectors(this.object.up,Zt)),Zt.multiplyScalar(e),this._panOffset.add(Zt)}_pan(e,t){let n=this.domElement;if(this.object.isPerspectiveCamera){let i=this.object.position;Zt.copy(i).sub(this.target);let r=Zt.length();r*=Math.tan(this.object.fov/2*Math.PI/180),this._panLeft(2*e*r/n.clientHeight,this.object.matrix),this._panUp(2*t*r/n.clientHeight,this.object.matrix)}else this.object.isOrthographicCamera?(this._panLeft(e*(this.object.right-this.object.left)/this.object.zoom/n.clientWidth,this.object.matrix),this._panUp(t*(this.object.top-this.object.bottom)/this.object.zoom/n.clientHeight,this.object.matrix)):(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - pan disabled."),this.enablePan=!1)}_dollyOut(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale/=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_dollyIn(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale*=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_updateZoomParameters(e,t){if(!this.zoomToCursor)return;this._performCursorZoom=!0;let n=this.domElement.getBoundingClientRect(),i=e-n.left,r=t-n.top,o=n.width,a=n.height;this._mouse.x=i/o*2-1,this._mouse.y=-(r/a)*2+1,this._dollyDirection.set(this._mouse.x,this._mouse.y,1).unproject(this.object).sub(this.object.position).normalize()}_clampDistance(e){return Math.max(this.minDistance,Math.min(this.maxDistance,e))}_handleMouseDownRotate(e){this._rotateStart.set(e.clientX,e.clientY)}_handleMouseDownDolly(e){this._updateZoomParameters(e.clientX,e.clientX),this._dollyStart.set(e.clientX,e.clientY)}_handleMouseDownPan(e){this._panStart.set(e.clientX,e.clientY)}_handleMouseMoveRotate(e){this._rotateEnd.set(e.clientX,e.clientY),this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(_n*this._rotateDelta.x/t.clientHeight),this._rotateUp(_n*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd),this.update()}_handleMouseMoveDolly(e){this._dollyEnd.set(e.clientX,e.clientY),this._dollyDelta.subVectors(this._dollyEnd,this._dollyStart),this._dollyDelta.y>0?this._dollyOut(this._getZoomScale(this._dollyDelta.y)):this._dollyDelta.y<0&&this._dollyIn(this._getZoomScale(this._dollyDelta.y)),this._dollyStart.copy(this._dollyEnd),this.update()}_handleMouseMovePan(e){this._panEnd.set(e.clientX,e.clientY),this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd),this.update()}_handleMouseWheel(e){this._updateZoomParameters(e.clientX,e.clientY),e.deltaY<0?this._dollyIn(this._getZoomScale(e.deltaY)):e.deltaY>0&&this._dollyOut(this._getZoomScale(e.deltaY)),this.update()}_handleKeyDown(e){let t=!1;switch(e.code){case this.keys.UP:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(_n*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,this.keyPanSpeed),t=!0;break;case this.keys.BOTTOM:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(-_n*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,-this.keyPanSpeed),t=!0;break;case this.keys.LEFT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(_n*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(this.keyPanSpeed,0),t=!0;break;case this.keys.RIGHT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(-_n*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(-this.keyPanSpeed,0),t=!0;break}t&&(e.preventDefault(),this.update())}_handleTouchStartRotate(e){if(this._pointers.length===1)this._rotateStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._rotateStart.set(n,i)}}_handleTouchStartPan(e){if(this._pointers.length===1)this._panStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panStart.set(n,i)}}_handleTouchStartDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyStart.set(0,r)}_handleTouchStartDollyPan(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enablePan&&this._handleTouchStartPan(e)}_handleTouchStartDollyRotate(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enableRotate&&this._handleTouchStartRotate(e)}_handleTouchMoveRotate(e){if(this._pointers.length==1)this._rotateEnd.set(e.pageX,e.pageY);else{let n=this._getSecondPointerPosition(e),i=.5*(e.pageX+n.x),r=.5*(e.pageY+n.y);this._rotateEnd.set(i,r)}this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(_n*this._rotateDelta.x/t.clientHeight),this._rotateUp(_n*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd)}_handleTouchMovePan(e){if(this._pointers.length===1)this._panEnd.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panEnd.set(n,i)}this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd)}_handleTouchMoveDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyEnd.set(0,r),this._dollyDelta.set(0,Math.pow(this._dollyEnd.y/this._dollyStart.y,this.zoomSpeed)),this._dollyOut(this._dollyDelta.y),this._dollyStart.copy(this._dollyEnd);let o=(e.pageX+t.x)*.5,a=(e.pageY+t.y)*.5;this._updateZoomParameters(o,a)}_handleTouchMoveDollyPan(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enablePan&&this._handleTouchMovePan(e)}_handleTouchMoveDollyRotate(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enableRotate&&this._handleTouchMoveRotate(e)}_addPointer(e){this._pointers.push(e.pointerId)}_removePointer(e){delete this._pointerPositions[e.pointerId];for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId){this._pointers.splice(t,1);return}}_isTrackingPointer(e){for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId)return!0;return!1}_trackPointer(e){let t=this._pointerPositions[e.pointerId];t===void 0&&(t=new ye,this._pointerPositions[e.pointerId]=t),t.set(e.pageX,e.pageY)}_getSecondPointerPosition(e){let t=e.pointerId===this._pointers[0]?this._pointers[1]:this._pointers[0];return this._pointerPositions[t]}_customWheelEvent(e){let t=e.deltaMode,n={clientX:e.clientX,clientY:e.clientY,deltaY:e.deltaY};switch(t){case 1:n.deltaY*=16;break;case 2:n.deltaY*=100;break}return e.ctrlKey&&!this._controlActive&&(n.deltaY*=10),n}};function pv(s){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(s.pointerId),this.domElement.ownerDocument.addEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.addEventListener("pointerup",this._onPointerUp)),!this._isTrackingPointer(s)&&(this._addPointer(s),s.pointerType==="touch"?this._onTouchStart(s):this._onMouseDown(s),this._cursorStyle==="grab"&&(this.domElement.style.cursor="grabbing")))}function mv(s){this.enabled!==!1&&(s.pointerType==="touch"?this._onTouchMove(s):this._onMouseMove(s))}function gv(s){switch(this._removePointer(s),this._pointers.length){case 0:this.domElement.releasePointerCapture(s.pointerId),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.dispatchEvent(Rd),this.state=Et.NONE,this._cursorStyle==="grab"&&(this.domElement.style.cursor="grab");break;case 1:let e=this._pointers[0],t=this._pointerPositions[e];this._onTouchStart({pointerId:e,pageX:t.x,pageY:t.y});break}}function _v(s){let e;switch(s.button){case 0:e=this.mouseButtons.LEFT;break;case 1:e=this.mouseButtons.MIDDLE;break;case 2:e=this.mouseButtons.RIGHT;break;default:e=-1}switch(e){case Zi.DOLLY:if(this.enableZoom===!1)return;this._handleMouseDownDolly(s),this.state=Et.DOLLY;break;case Zi.ROTATE:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Et.PAN}else{if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Et.ROTATE}break;case Zi.PAN:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Et.ROTATE}else{if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Et.PAN}break;default:this.state=Et.NONE}this.state!==Et.NONE&&this.dispatchEvent(tu)}function xv(s){switch(this.state){case Et.ROTATE:if(this.enableRotate===!1)return;this._handleMouseMoveRotate(s);break;case Et.DOLLY:if(this.enableZoom===!1)return;this._handleMouseMoveDolly(s);break;case Et.PAN:if(this.enablePan===!1)return;this._handleMouseMovePan(s);break}}function vv(s){this.enabled===!1||this.enableZoom===!1||this.state!==Et.NONE||(s.preventDefault(),this.dispatchEvent(tu),this._handleMouseWheel(this._customWheelEvent(s)),this.dispatchEvent(Rd))}function yv(s){this.enabled!==!1&&this._handleKeyDown(s)}function Mv(s){switch(this._trackPointer(s),this._pointers.length){case 1:switch(this.touches.ONE){case Ki.ROTATE:if(this.enableRotate===!1)return;this._handleTouchStartRotate(s),this.state=Et.TOUCH_ROTATE;break;case Ki.PAN:if(this.enablePan===!1)return;this._handleTouchStartPan(s),this.state=Et.TOUCH_PAN;break;default:this.state=Et.NONE}break;case 2:switch(this.touches.TWO){case Ki.DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchStartDollyPan(s),this.state=Et.TOUCH_DOLLY_PAN;break;case Ki.DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchStartDollyRotate(s),this.state=Et.TOUCH_DOLLY_ROTATE;break;default:this.state=Et.NONE}break;default:this.state=Et.NONE}this.state!==Et.NONE&&this.dispatchEvent(tu)}function bv(s){switch(this._trackPointer(s),this.state){case Et.TOUCH_ROTATE:if(this.enableRotate===!1)return;this._handleTouchMoveRotate(s),this.update();break;case Et.TOUCH_PAN:if(this.enablePan===!1)return;this._handleTouchMovePan(s),this.update();break;case Et.TOUCH_DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchMoveDollyPan(s),this.update();break;case Et.TOUCH_DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchMoveDollyRotate(s),this.update();break;default:this.state=Et.NONE}}function Sv(s){this.enabled!==!1&&s.preventDefault()}function Tv(s){s.key==="Control"&&(this._controlActive=!0,this.domElement.getRootNode().addEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}function Ev(s){s.key==="Control"&&(this._controlActive=!1,this.domElement.getRootNode().removeEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}var $l=class extends gs{constructor(){super(),this.name="RoomEnvironment",this.position.y=-3.5;let e=new si;e.deleteAttribute("uv");let t=new dn({side:Wt}),n=new dn,i=new Xn(16777215,900,28,2);i.position.set(.418,16.199,.3),this.add(i);let r=new Je(e,t);r.position.set(-.757,13.219,.717),r.scale.set(31.713,28.305,28.591),this.add(r);let o=new fn(e,n,6),a=new yt;a.position.set(-10.906,2.009,1.846),a.rotation.set(0,-.195,0),a.scale.set(2.328,7.905,4.651),a.updateMatrix(),o.setMatrixAt(0,a.matrix),a.position.set(-5.607,-.754,-.758),a.rotation.set(0,.994,0),a.scale.set(1.97,1.534,3.955),a.updateMatrix(),o.setMatrixAt(1,a.matrix),a.position.set(6.167,.857,7.803),a.rotation.set(0,.561,0),a.scale.set(3.927,6.285,3.687),a.updateMatrix(),o.setMatrixAt(2,a.matrix),a.position.set(-2.017,.018,6.124),a.rotation.set(0,.333,0),a.scale.set(2.002,4.566,2.064),a.updateMatrix(),o.setMatrixAt(3,a.matrix),a.position.set(2.291,-.756,-2.621),a.rotation.set(0,-.286,0),a.scale.set(1.546,1.552,1.496),a.updateMatrix(),o.setMatrixAt(4,a.matrix),a.position.set(-2.193,-.369,-5.547),a.rotation.set(0,.516,0),a.scale.set(3.875,3.487,2.986),a.updateMatrix(),o.setMatrixAt(5,a.matrix),this.add(o);let l=new Je(e,br(50));l.position.set(-16.116,14.37,8.208),l.scale.set(.1,2.428,2.739),this.add(l);let c=new Je(e,br(50));c.position.set(-16.109,18.021,-8.207),c.scale.set(.1,2.425,2.751),this.add(c);let h=new Je(e,br(17));h.position.set(14.904,12.198,-1.832),h.scale.set(.15,4.265,6.331),this.add(h);let u=new Je(e,br(43));u.position.set(-.462,8.89,14.52),u.scale.set(4.38,5.441,.088),this.add(u);let f=new Je(e,br(20));f.position.set(3.235,11.486,-12.541),f.scale.set(2.5,2,.1),this.add(f);let d=new Je(e,br(100));d.position.set(0,20,0),d.scale.set(1,.1,1),this.add(d)}dispose(){let e=new Set;this.traverse(t=>{t.isMesh&&(e.add(t.geometry),e.add(t.material))});for(let t of e)t.dispose()}};function br(s){return new to({color:0,emissive:16777215,emissiveIntensity:s})}var Pi={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};var An=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},wv=new li(-1,1,1,-1,0,1),nu=class extends ft{constructor(){super(),this.setAttribute("position",new ot([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new ot([0,2,0,0,2,0],2))}},Av=new nu,fi=class{constructor(e){this._mesh=new Je(Av,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,wv)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var Sr=class extends An{constructor(e,t="tDiffuse"){super(),this.textureID=t,this.uniforms=null,this.material=null,e instanceof mt?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=Kn.clone(e.uniforms),this.material=new mt({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new fi(this.material)}render(e,t,n){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=n.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Uo=class extends An{constructor(e,t){super(),this.scene=e,this.camera=t,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,t,n){let i=e.getContext(),r=e.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let o,a;this.inverse?(o=0,a=1):(o=1,a=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(i.REPLACE,i.REPLACE,i.REPLACE),r.buffers.stencil.setFunc(i.ALWAYS,o,4294967295),r.buffers.stencil.setClear(a),r.buffers.stencil.setLocked(!0),e.setRenderTarget(n),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(t),this.clear&&e.clear(),e.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(i.EQUAL,1,4294967295),r.buffers.stencil.setOp(i.KEEP,i.KEEP,i.KEEP),r.buffers.stencil.setLocked(!0)}},Ql=class extends An{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var ec=class{constructor(e,t){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),t===void 0){let n=e.getSize(new ye);this._width=n.width,this._height=n.height,t=new Nt(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:Xt}),t.texture.name="EffectComposer.rt1"}else this._width=t.width,this._height=t.height;this.renderTarget1=t,this.renderTarget2=t.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Sr(Pi),this.copyPass.material.blending=On,this.timer=new ho}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,t){this.passes.splice(t,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let t=this.passes.indexOf(e);t!==-1&&this.passes.splice(t,1)}isLastEnabledPass(e){for(let t=e+1;t<this.passes.length;t++)if(this.passes[t].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let t=this.renderer.getRenderTarget(),n=!1;for(let i=0,r=this.passes.length;i<r;i++){let o=this.passes[i];if(o.enabled!==!1){if(o.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(i),o.render(this.renderer,this.writeBuffer,this.readBuffer,e,n),o.needsSwap){if(n){let a=this.renderer.getContext(),l=this.renderer.state.buffers.stencil;l.setFunc(a.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),l.setFunc(a.EQUAL,1,4294967295)}this.swapBuffers()}Uo!==void 0&&(o instanceof Uo?n=!0:o instanceof Ql&&(n=!1))}}this.renderer.setRenderTarget(t)}reset(e){if(e===void 0){let t=this.renderer.getSize(new ye);this._pixelRatio=this.renderer.getPixelRatio(),this._width=t.width,this._height=t.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,t){this._width=e,this._height=t;let n=this._width*this._pixelRatio,i=this._height*this._pixelRatio;this.renderTarget1.setSize(n,i),this.renderTarget2.setSize(n,i);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(n,i)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var Cd={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Me(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var Tr=class s extends An{constructor(e,t=1,n,i){super(),this.strength=t,this.radius=n,this.threshold=i,this.resolution=e!==void 0?new ye(e.x,e.y):new ye(256,256),this.clearColor=new Me(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);this.renderTargetBright=new Nt(r,o,{type:Xt}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let h=0;h<this.nMips;h++){let u=new Nt(r,o,{type:Xt});u.texture.name="UnrealBloomPass.h"+h,u.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(u);let f=new Nt(r,o,{type:Xt});f.texture.name="UnrealBloomPass.v"+h,f.texture.generateMipmaps=!1,this.renderTargetsVertical.push(f),r=Math.round(r/2),o=Math.round(o/2)}let a=Cd;this.highPassUniforms=Kn.clone(a.uniforms),this.highPassUniforms.luminosityThreshold.value=i,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new mt({uniforms:this.highPassUniforms,vertexShader:a.vertexShader,fragmentShader:a.fragmentShader}),this.separableBlurMaterials=[];let l=[6,10,14,18,22];r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);for(let h=0;h<this.nMips;h++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(l[h])),this.separableBlurMaterials[h].uniforms.invSize.value=new ye(1/r,1/o),r=Math.round(r/2),o=Math.round(o/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=t,this.compositeMaterial.uniforms.bloomRadius.value=.1;let c=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=c,this.bloomTintColors=[new D(1,1,1),new D(1,1,1),new D(1,1,1),new D(1,1,1),new D(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=Kn.clone(Pi.uniforms),this.blendMaterial=new mt({uniforms:this.copyUniforms,vertexShader:Pi.vertexShader,fragmentShader:Pi.fragmentShader,premultipliedAlpha:!0,blending:rn,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new Me,this._oldClearAlpha=1,this._basic=new Tt,this._fsQuad=new fi(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,t){let n=Math.round(e/2),i=Math.round(t/2);this.renderTargetBright.setSize(n,i);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(n,i),this.renderTargetsVertical[r].setSize(n,i),this.separableBlurMaterials[r].uniforms.invSize.value=new ye(1/n,1/i),n=Math.round(n/2),i=Math.round(i/2)}render(e,t,n,i,r){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let o=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),r&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=n.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=n.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let a=this.renderTargetBright;for(let l=0;l<this.nMips;l++)this._fsQuad.material=this.separableBlurMaterials[l],this.separableBlurMaterials[l].uniforms.colorTexture.value=a.texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[l]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[l].uniforms.colorTexture.value=this.renderTargetsHorizontal[l].texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[l]),e.clear(),this._fsQuad.render(e),a=this.renderTargetsVertical[l];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(n),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=o}_getSeparableBlurMaterial(e){let t=[],n=e/3;for(let i=0;i<e;i++)t.push(.39894*Math.exp(-.5*i*i/(n*n))/n);return new mt({defines:{KERNEL_RADIUS:e},uniforms:{colorTexture:{value:null},invSize:{value:new ye(.5,.5)},direction:{value:new ye(.5,.5)},gaussianCoefficients:{value:t}},vertexShader:`

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
				uniform float gaussianCoefficients[KERNEL_RADIUS];

				void main() {

					float weightSum = gaussianCoefficients[0];
					vec3 diffuseSum = texture2D( colorTexture, vUv ).rgb * weightSum;

					for ( int i = 1; i < KERNEL_RADIUS; i ++ ) {

						float x = float( i );
						float w = gaussianCoefficients[i];
						vec2 uvOffset = direction * invSize * x;
						vec3 sample1 = texture2D( colorTexture, vUv + uvOffset ).rgb;
						vec3 sample2 = texture2D( colorTexture, vUv - uvOffset ).rgb;
						diffuseSum += ( sample1 + sample2 ) * w;

					}

					gl_FragColor = vec4( diffuseSum, 1.0 );

				}`})}_getCompositeMaterial(e){return new mt({defines:{NUM_MIPS:e},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`

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

				}`})}};Tr.BlurDirectionX=new ye(1,0);Tr.BlurDirectionY=new ye(0,1);var Fo={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var tc=class extends An{constructor(){super(),this.isOutputPass=!0,this.uniforms=Kn.clone(Fo.uniforms),this.material=new cr({name:Fo.name,uniforms:this.uniforms,vertexShader:Fo.vertexShader,fragmentShader:Fo.fragmentShader}),this._fsQuad=new fi(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,t,n){this.uniforms.tDiffuse.value=n.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},et.getTransfer(this._outputColorSpace)===pt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===go?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===_o?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===xo?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===bs?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===yo?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Mo?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===vo&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Ii=512,Rv=7.5;function nc(s,e=Rv){let t=new $r,n=s.map(([a,l])=>new D(a,0,l)),i=[],r=[],o=n.length;for(let a=0;a<o;a++){let l=n[a],c=n[(a+o-1)%o],h=n[(a+1)%o],u=Math.min(e,l.distanceTo(c)/2,l.distanceTo(h)/2);i.push(l.clone().addScaledVector(c.clone().sub(l).normalize(),u)),r.push(l.clone().addScaledVector(h.clone().sub(l).normalize(),u))}for(let a=0;a<o;a++)t.add(new Xi(i[a],n[a],r[a])),t.add(new ar(r[a],i[(a+1)%o]));return t}function Cv(s){let e=nc(s),t=e.getSpacedPoints(Ii),n=[];for(let i=0;i<Ii;i++){let r=t[(i+Ii-1)%Ii],o=t[(i+1)%Ii];n.push(new D(o.x-r.x,0,o.z-r.z).normalize())}return{points:t,tangents:n,length:e.getLength()}}function Pd(s){let e=[],t=(n,i,r,o)=>{let a=Math.cos(n.angle),l=Math.sin(n.angle);e.push({x:n.x+i*a+r*l,z:n.z-i*l+r*a,r:o,asset:n.asset})};for(let n of s)if(!(n.z<-100))if(n.asset==="data-tram")for(let i of[-2.4,0,2.4])t(n,i,0,1.5);else n.asset==="server-rack"?t(n,0,0,.8):n.asset==="street-lamp"?t(n,0,0,.5):n.asset==="planter"&&(t(n,-.9,0,.85),t(n,.9,0,.85));return e}function Id({routes:s,obstacles:e=[],robotRadius:t=1.3,lanes:n=[2.2,0,-2.2,3.9,-3.9],defaultLane:i=2.2,speeds:r=[]}){let o=s.map(Cv),a=s.map((v,E)=>({route:E,s:o[E].length*(E*.173%1),dir:1,lane:i,laneTarget:i,speed:0,target:0,laneVel:0,cruise:r[E]??5.1+E*.43,state:"cruise",heading:0,wait:0,cooldown:0,x:0,z:0,fx:0,fz:1,rx:1,rz:0,vx:0,vz:0,turns:0})),l=new D,c=new D,h=new D;function u(v,E,P,L,H){let O=(E/v.length%1+1)%1*Ii,U=Math.floor(O)%Ii,V=O-Math.floor(O);L.lerpVectors(v.points[U],v.points[(U+1)%Ii],V),H.lerpVectors(v.tangents[U],v.tangents[(U+1)%Ii],V).normalize().multiplyScalar(P)}let f=v=>Math.atan2(Math.sin(v),Math.cos(v));function d(v){u(o[v.route],v.s,v.dir,l,c),v.fx=c.x,v.fz=c.z,v.rx=-c.z,v.rz=c.x,v.x=l.x+v.rx*v.lane,v.z=l.z+v.rz*v.lane}function g(v,E){for(let P of e){let L=P.r+t;if((v-P.x)**2+(E-P.z)**2<L*L)return!0}return!1}function M(v,E){let P=o[v.route];for(let L of[-2.5,0,1,3,6,9,12,15,18])if(u(P,v.s+v.dir*L,v.dir,l,c),h.set(l.x-c.z*E,0,l.z+c.x*E),g(h.x,h.z))return!1;return!0}function m(v){v.dir=-v.dir,v.lane=-v.lane,v.laneTarget=i,v.laneVel=0,v.state="turn",v.cooldown=4,v.wait=0,v.target=0,v.turns++}function p(v,E){if(v.cooldown>0&&(v.cooldown-=E),v.state==="turn"){if(v.target=0,v.speed<.05){let O=Math.atan2(v.fx,v.fz),U=f(O-v.heading);v.heading+=U*Math.min(1,E*3.2),Math.abs(U)<.1&&(v.heading=O,v.state="cruise")}return}let P=null,L=[...n].sort((O,U)=>Math.abs(O-i)-Math.abs(U-i)||U-O);for(let O of L)if(M(v,O)){P=O;break}if(P===null){v.cooldown<=0?m(v):v.target=0;return}v.laneTarget=P,v.target=v.cruise,Math.abs(v.laneTarget-v.lane)>.5&&!M(v,v.lane)&&(v.target=Math.min(v.target,2.4));let H=!1;for(let O of a){if(O===v)continue;let U=y(v,v.laneTarget,O);if(!U)continue;H=!0;let V=O.fx*v.fx+O.fz*v.fz,F=O.speed<.3;if(F||V<-.2){let Y=R.get(O)[S.length-1],J=F?0:Math.sign((Y.x-v.x)*v.rx+(Y.z-v.z)*v.rz)||-1,j=null;for(let me of L)if(!(J&&Math.sign(me-v.lane)===J)&&M(v,me)&&!y(v,me,O)){j=me;break}j!==null?v.laneTarget=j:v.target=0}else V>.5?U.da>=U.db&&(v.target=Math.min(v.target,U.da<3.8?0:O.speed*.9)):(U.da>U.db||U.da===U.db&&v.route>O.route)&&(v.target=Math.min(v.target,U.da<=4?0:2))}H&&v.target===0?v.wait+=E:v.wait=0,v.wait>2.5&&v.cooldown<=0&&m(v)}let S=[0,2,4,6,8,10,12],R=new Map,b=new D;function w(v,E,P,L){u(o[v.route],v.s+v.dir*E,v.dir,l,c),L.set(l.x-c.z*P,0,l.z+c.x*P)}function y(v,E,P){let L=R.get(P),H=(t*2+.6)**2,O=P.speed<.3;for(let U=0;U<S.length;U++){w(v,S[U],E,b);let V=S[U]/Math.max(v.speed,1.5);for(let F=0;F<S.length&&!(O&&F>0);F++){let Y=S[F]/Math.max(P.speed,1.5);if(!(S[U]>4&&Math.abs(V-Y)>1.6)&&b.distanceToSquared(L[F])<H)return{da:S[U],db:S[F]}}}return null}function C(v){if(v>0){for(let E of a){d(E),R.has(E)||R.set(E,S.map(()=>new D));let P=R.get(E);S.forEach((L,H)=>w(E,E.state==="cruise"?L:0,E.lane,P[H]))}for(let E of a)p(E,v);for(let E=0;E<a.length;E++)for(let P=E+1;P<a.length;P++){let L=a[E],H=a[P],O=H.x-L.x,U=H.z-L.z,V=Math.hypot(O,U),F=t*2+.2;if(V>=F)continue;let Y=L.state==="turn"||L.speed<.3,J=H.state==="turn"||H.speed<.3,j=Y===J?.5:Y?0:1,me=Math.min(F-Math.max(V,.01),2.5*v),_e=O*L.rx+U*L.rz,Ve=-(O*H.rx+U*H.rz);L.lane=Vt.clamp(L.lane-Math.sign(_e||1)*me*j,-4.2,4.2),H.lane=Vt.clamp(H.lane-Math.sign(Ve||1)*me*(1-j),-4.2,4.2),L.target=Math.min(L.target,.5),H.target=Math.min(H.target,.5)}for(let E of a){let P=E.x,L=E.z,H=E.target>E.speed?6:9;E.speed+=Vt.clamp(E.target-E.speed,-H*v,H*v),E.speed<0&&(E.speed=0),E.s+=E.dir*E.speed*v;let O=E.laneTarget-E.lane,U=E.state==="turn"?0:Math.min(3,E.speed*.6),V=Math.sign(O)*Math.min(U,Math.sqrt(8*Math.abs(O)),Math.abs(O)/v);if(E.laneVel+=Vt.clamp(V-E.laneVel,-4*v,4*v),E.lane+=E.laneVel*v,d(E),E.vx=(E.x-P)/v,E.vz=(E.z-L)/v,E.state==="cruise"){let F=E.speed>.25?Math.atan2(E.vx,E.vz):Math.atan2(E.fx,E.fz);E.heading+=f(F-E.heading)*Math.min(1,v*(E.speed>.25?8:3.2))}}}}let _=[];for(let v of a){for(let E=0;E<600&&(d(v),!(_.every(L=>Math.hypot(v.x-L.x,v.z-L.z)>t*2+6)&&M(v,v.lane)));E++)v.s+=1;v.heading=Math.atan2(v.fx,v.fz),_.push(v)}return{agents:a,step:C,hitsObstacle:g,stats:()=>({states:a.map(v=>v.state),turns:a.reduce((v,E)=>v+E.turns,0),lanes:a.map(v=>Math.round(v.lane*10)/10)})}}var Ld=[[[-67,-77],[-18,-77],[-18,-32],[-67,-32]],[[-18,-77],[18,-77],[18,-32],[-18,-32]],[[18,-77],[67,-77],[67,-32],[18,-32]],[[18,13],[18,-32],[67,-32],[67,13]],[[-67,-77],[-18,-77],[-18,-32],[-67,-32]]];function Dd(s,e,t){let n=!1,i=0,r=!1,o=!1,a=0,l=!1,c=new ut;c.name="city-life",s.add(c);let h=Id({routes:Ld,obstacles:t.obstacles||[]}),u=new Set,f=new Set,d=new Set,g=[],M=new AbortController,m=[],p=[],S=X=>(u.add(X),X),R=X=>(f.add(X),X),b=S(new Si(1,.013,5,64));for(let X of e){let he=R(new Tt({color:7452878,transparent:!0,opacity:.08,depthWrite:!1,toneMapped:!1})),$=new Je(b,he);$.rotation.x=Math.PI/2,$.position.set(X.x,.6,X.z),$.scale.setScalar(X.radius*1.12),c.add($),p.push({id:X.id,ring:$,material:he,state:"unknown",eventUntil:0,color:new Me(7438733)})}let w=new Map,y=[],C=new Map,_=Date.now(),v=3e3,E=e.find(X=>X.id==="agent"),P=new D(0,0,1),L=new D,H={memory:12690431,graph:16765844,infra:7978495,integrations:7728086,missions:10268415,operations:16758915};if(E){for(let X of e)if(X!==E){let he=new D(E.x,E.height+3,E.z),$=new D(X.x,X.height+4,X.z),ie=he.clone().lerp($,.5);ie.y=Math.max(he.y,$.y)+12;let oe=new Xi(he,ie,$),ue=R(new ii({color:H[X.id],transparent:!0,opacity:0,depthWrite:!1,blending:rn,toneMapped:!1})),se=new Mi(S(new ft().setFromPoints(oe.getPoints(48))),ue);se.visible=!1,c.add(se),w.set(X.id,{curve:oe,line:se})}}let O=S(new Si(1,.06,5,40,Math.PI*.95));for(let X=0;X<12;X++){let he=[];for(let $=0;$<4;$++){let ie=R(new Tt({color:10345983,transparent:!0,opacity:0,depthWrite:!1,blending:rn,toneMapped:!1})),oe=new Je($===3?b:O,ie);oe.visible=!1,c.add(oe),he.push(oe)}y.push({at:-1/0,waves:he,link:null})}function U(X,he){if(!l||t.active?.()===!1||X.at<_||he-X.at>v||X.at>he||!["started","succeeded","failed","sanitized","progress"].includes(X.state))return;let $=X.to==="agent",ie=$?X.from:X.to;if(!$&&X.from!=="agent"||!w.has(ie))return;let oe=ie+":"+$;if(X.at-(C.get(oe)??-1/0)<450)return;C.set(oe,X.at);let ue=y.reduce((se,ce)=>se.at<ce.at?se:ce);Object.assign(ue,{at:X.at,link:w.get(ie),incoming:$,from:X.from,to:X.to,state:X.state}),ue.waves.forEach(se=>se.material.color.setHex(X.state==="failed"?16737881:H[ie]))}function V(X){let he=Date.now();w.forEach($=>{$.line.visible=!1});for(let $ of y){(!X||t.active?.()===!1)&&($.at=-1/0);let ie=(he-$.at)/v,oe=ie>=0&&ie<1;if($.waves.forEach(se=>{se.visible=!1}),!oe||!$.link)continue;$.link.line.visible=!0,$.link.line.material.opacity=.12*Math.sin(ie*Math.PI);for(let se=0;se<3;se++){let ce=(ie-se*.075)/.75;if(ce<0||ce>1)continue;let ve=$.incoming?1-ce:ce,Re=$.waves[se];$.link.curve.getPoint(ve,Re.position),$.link.curve.getTangent(ve,L),$.incoming&&L.negate(),Re.quaternion.setFromUnitVectors(P,L),Re.rotateZ(Math.PI*.525),Re.scale.setScalar(1.2+Math.sin(ce*Math.PI)*3.5),Re.material.opacity=Math.sin(ce*Math.PI)*.8,Re.visible=!0}let ue=(ie-.74)/.26;if(ue>0){let se=$.waves[3];$.link.curve.getPoint($.incoming?0:1,se.position),se.rotation.set(-Math.PI/2,0,0),se.scale.setScalar(1+ue*8),se.material.opacity=(1-ue)*.6,se.visible=!0}}}let F=S(new sn(7,7)),Y=R(new mt({transparent:!0,depthWrite:!1,blending:rn,uniforms:{color:{value:new Me(5495284)}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"varying vec2 vUv;uniform vec3 color;void main(){float r=length(vUv-.5)*2.;float a=exp(-r*r*5.)*.23*(1.-smoothstep(.65,1.,r));gl_FragColor=vec4(color,a);}"})),J=S(new vs(.42,1.3,12,1,!0)),j=R(new Tt({color:9629439,transparent:!0,opacity:.22,depthWrite:!1,toneMapped:!1,blending:rn}));Ld.forEach((X,he)=>{let $=new ut;$.name="city-white-robot-"+(he+1);let ie=new ut;$.add(ie),c.add($);let oe=new Je(F,Y);oe.rotation.x=-Math.PI/2,oe.position.y=.58,c.add(oe);let ue=new Je(J,j);ue.rotation.z=Math.PI,ue.position.y=-.45,$.add(ue),$.visible=!1,oe.visible=!1,m.push({root:$,body:ie,glow:oe,jet:ue,agent:h.agents[he]})});function me(X){X.traverse(he=>{if(he.isMesh){u.add(he.geometry);for(let $ of Array.isArray(he.material)?he.material:[he.material]){f.add($);for(let ie of Object.values($))ie?.isTexture&&d.add(ie)}}})}function _e(){u.forEach(he=>he.dispose()),f.forEach(he=>he.dispose());let X=new Set;d.forEach(he=>{he.image&&X.add(he.image),he.dispose()}),X.forEach(he=>he.close?.()),u.clear(),f.clear(),d.clear()}async function Ve(){let X=setTimeout(()=>M.abort(),15e3);try{let he=await fetch(t.robotURL,{signal:M.signal});if(!he.ok)throw Error("Robot unavailable");let $=await he.arrayBuffer();if(n)return;let{scene:ie}=await new ns().parseAsync($,"");if(me(ie),n){_e();return}let oe=new hn().setFromObject(ie),ue=oe.getSize(new D),se=Math.max(ue.x,ue.y,ue.z);if(!Number.isFinite(se)||se<=0)throw Error("Invalid robot bounds");ie.scale.setScalar(6/se),ie.updateMatrixWorld(!0),oe.setFromObject(ie);let ce=oe.getCenter(new D);ie.position.set(-ce.x,-oe.min.y,-ce.z),ie.rotation.y=-Math.PI/2,ie.traverse(ve=>{ve.isMesh&&(ve.castShadow=!1,ve.receiveShadow=!0)}),m.forEach(ve=>{ve.body.add(ie.clone(!0)),ve.root.visible=!0,ve.glow.visible=!0}),r=!0,Q(0,!1)}catch(he){n||(o=!0,t.onError?.(he))}finally{clearTimeout(X)}}function it(X){g.forEach($=>{$.dispose(),f.delete($)}),g.length=0,X.children.find($=>$.userData.district==="operations")?.getObjectByName("signal")?.traverse($=>{if(!$.isMesh)return;let ie=oe=>{let ue=R(oe.clone());return g.push(ue),ue};$.material=Array.isArray($.material)?$.material.map(ie):ie($.material)})}function Fe(X,he=[]){for(let ie of p){let oe=X.find(ue=>ue.id===ie.id);ie.state=!oe||oe.stale?"unknown":oe.state,ie.color.setHex(ie.state==="error"?16730430:ie.state==="running"?7400403:ie.state==="unknown"?7438733:7452878),ie.material.color.copy(ie.color)}let $=Date.now();for(let ie of[...he].reverse())if(ie.id>a&&(U(ie,$),$-ie.at<6e3)){let oe=p.find(ue=>ue.id===ie.district);oe&&(oe.eventUntil=ie.at+5e3)}a=Math.max(a,...he.map(ie=>ie.id))}function Q(X,he){if(n)return;l=he,V(he);let $=Math.min(.1,Math.max(0,X));he&&$>0&&(i+=$,h.step($));for(let[ie,oe]of m.entries()){let ue=oe.agent,se=Math.hypot(ue.vx,ue.vz);oe.root.position.set(ue.x,1.6+Math.sin(i*1.6+ie*1.9)*.18,ue.z),oe.root.rotation.y=ue.heading;let ce=(ue.vx*ue.rx+ue.vz*ue.rz)/Math.max(1,se);oe.body.rotation.z=Math.sin(i*.85+ie)*.035-ce*.08,oe.body.rotation.x=-.035+Math.sin(i*1.1+ie)*.018-Math.min(.09,se*.015),oe.glow.position.x=ue.x,oe.glow.position.z=ue.z,oe.jet.scale.y=1+Math.sin(i*4+ie)*.12+se*.04}for(let ie of p){let oe=he?.5+.5*Math.sin(i*(ie.state==="error"?3.2:1.8)):.5,ue=ie.state==="error"||ie.state==="running"||ie.eventUntil>Date.now();if(ie.material.opacity=ie.state==="unknown"?.03:ue?.2+oe*.46:.065,ie.ring.scale.setScalar(e.find(se=>se.id===ie.id).radius*1.12*(1+(ue&&he?oe*.035:0))),ie.id==="operations")for(let se of g)se.color.copy(ie.color),se.emissive?.copy(ie.color),se.emissiveIntensity=ie.state==="error"?1.1+oe*2.2:ie.state==="unknown"?.08:.55}}function ge(){n||(n=!0,M.abort(),t.signal?.removeEventListener("abort",ge),c.removeFromParent(),_e(),m.length=0,g.length=0)}return t.signal?.addEventListener("abort",ge,{once:!0}),t.signal?.aborted?ge():Ve(),{update:Q,setData:Fe,attachLandmarks:it,dispose:ge,stats:()=>({robots:r?m.length:0,robotError:o,time:i,navigation:h.stats(),positions:m.map(X=>X.root.position.toArray()),transmissions:y.filter(X=>X.waves.some(he=>he.visible)).map(X=>({from:X.from,to:X.to,state:X.state,age:Date.now()-X.at})),signals:p.map(X=>({id:X.id,state:X.state,intensity:X.material.opacity}))})}}var Oo='Geist, "Segoe UI", system-ui, sans-serif',ic=13,Pv=s=>{let e=2166136261;for(let t=0;t<s.length;t++)e=Math.imul(e^s.charCodeAt(t),16777619);return(e>>>0).toString(16).padStart(8,"0")},iu={vertex:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:`uniform sampler2D map;uniform float time,glitch,fade,gain;uniform vec3 tint;varying vec2 vUv;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){vec2 uv=vUv;float row=floor(uv.y*44.0),frame=floor(time*24.0);
float g=glitch*step(0.7,hash(row+frame*0.37));uv.x+=(hash(row*3.1+frame)-0.5)*0.14*g;
float split=0.0025+glitch*0.02;vec4 c=texture2D(map,uv);
float r=texture2D(map,uv+vec2(split,0.0)).r,b=texture2D(map,uv-vec2(split,0.0)).b;
float a=max(c.a,max(texture2D(map,uv+vec2(split,0.0)).a,texture2D(map,uv-vec2(split,0.0)).a));
float lines=0.84+0.16*sin(uv.y*420.0-time*9.0);float flicker=0.95+0.05*sin(time*31.0)*sin(time*7.3);
float bx=(fract(uv.y*0.55-time*0.11)-0.5)*22.0;float band=0.35*exp(-bx*bx);
float edge=smoothstep(0.0,0.05,uv.x)*smoothstep(0.0,0.05,1.0-uv.x)*smoothstep(0.0,0.09,uv.y)*smoothstep(0.0,0.09,1.0-uv.y);
vec3 col=vec3(r,c.g,b)*tint*(lines*flicker+band)*gain;gl_FragColor=vec4(col*a*fade*edge,1.0);}`},Iv={vertex:"varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",fragment:`uniform float time,strength;uniform vec3 base,top;varying vec2 vUv;varying vec3 vNormal,vView;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){float h=vUv.y;float rim=pow(clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0),1.5);
float fall=pow(clamp(1.0-h,0.0,1.0),1.35)*0.75+0.25*clamp(1.0-h,0.0,1.0);float scan=0.7+0.3*sin(h*64.0-time*5.5);
float noise=0.85+0.15*hash(floor(h*96.0)+floor(time*18.0));float shimmer=0.8+0.2*sin(vUv.x*40.0+time*2.0);
float a=(0.28+0.72*rim)*fall*scan*noise*shimmer*strength;gl_FragColor=vec4(mix(base,top,h)*a,1.0);}`},Lv={vertex:iu.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float r=length(vUv-0.5)*2.0;float rings=smoothstep(0.35,1.0,sin(r*16.0-time*2.6));
float core=exp(-r*r*7.0);float a=(core*1.2+pow(max(0.0,1.0-r),1.8)*(0.25+0.75*rings)*0.6)*strength*(1.0-smoothstep(0.85,1.0,r));
gl_FragColor=vec4(color*a,1.0);}`},Nd={vertex:iu.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float dash=step(0.45,fract(vUv.x*28.0+time*0.35));float pulse=0.75+0.25*sin(vUv.x*6.2831*3.0-time*4.0);
gl_FragColor=vec4(color*dash*pulse*strength,1.0);}`},Dv={vertex:`attribute float seed;uniform float time,pointScale;varying float vLife;
void main(){float speed=0.55+seed*0.9;float life=fract(seed*3.17+time*speed/13.0);float y=life*13.0;
float ang=seed*6.2831+time*(0.35+seed*0.4)+life*2.2;float rad=mix(1.8,6.8,life)*(0.3+0.7*fract(seed*7.13));
vec4 mv=modelViewMatrix*vec4(cos(ang)*rad,y,sin(ang)*rad,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.09+0.16*fract(seed*3.7))*pointScale/max(1.0,-mv.z);vLife=life;}`,fragment:`uniform vec3 color;uniform float strength;varying float vLife;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,2.0)*sin(vLife*3.1416)*strength;gl_FragColor=vec4(color*a,1.0);}`};function Er(s,e,t={}){return new mt({uniforms:e,vertexShader:s.vertex,fragmentShader:s.fragment,transparent:!0,depthWrite:!1,blending:rn,side:Bt,...t})}function Nv(s,e=24){let t=[],n=new Set;for(let i of Array.isArray(s)?s:[]){if(typeof i!="string")continue;let r=i.replace(/[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u2028-\u202e\u2066-\u2069]/g," ").replace(/\s+/g," ").trim().slice(0,140);if(!(r.length<4||n.has(r))&&(n.add(r),t.push(r),t.length>=e))break}return t}function Ud(s,e,t,n){let i=[],r=c=>s.measureText(c).width<=t,o="";for(let c of e.split(" ")){if(i.length>=n)break;let h=o?o+" "+c:c;if(r(h)){o=h;continue}if(o&&(i.push(o),o="",i.length>=n))break;let u=c;for(;!r(u)&&i.length<n;){let f=u.length-1;for(;f>1&&!r(u.slice(0,f));)f--;i.push(u.slice(0,f)),u=u.slice(f)}o=u}o&&i.length<n&&(i.push(o),o="");let a=i.join("").replace(/\s/g,"").length,l=e.replace(/\s/g,"").length;if(a<l&&i.length){let c=i[i.length-1].replace(/[\s,.;:]+$/,"");for(;c.length&&!r(c+"\u2026");)c=c.slice(0,-1);i[i.length-1]=c+"\u2026"}return i.length?i:[""]}function Fd(s,e,t={}){let n=t.roof??21,i=String(t.label||"MEMORY").toUpperCase(),r=new ut;r.name="memory-hologram",r.position.set(e.x,n,e.z),s.add(r);let o=[],a=[],l=[],c=(oe,ue)=>(o.push(oe),a.push(ue),new Je(oe,ue)),h=new Me(6544639),u=new Me(10980351),f=new Me(14218751),d={time:{value:0}},g=!1,M=0,m=[],p=[],S=0,R="none",b=!1,w=c(new sn(11,11),Er(Lv,{time:d.time,strength:{value:.9},color:{value:h}}));w.rotation.x=-Math.PI/2,w.position.y=.18,r.add(w);let y=c(new rr(7.4,2.3,ic,56,1,!0),Er(Iv,{time:d.time,strength:{value:.32},base:{value:h},top:{value:u}}));y.position.y=ic/2+.3,r.add(y);let C=c(new Si(7.5,.055,6,128),Er(Nd,{time:d.time,strength:{value:1.6},color:{value:h}}));C.rotation.x=Math.PI/2,C.position.y=ic+.3,r.add(C);let _=c(new Si(2.6,.05,6,96),Er(Nd,{time:d.time,strength:{value:2},color:{value:f}}));_.rotation.x=Math.PI/2,_.position.y=.55,r.add(_);let v=new lr(1.35,1),E=new eo(v),P=new ii({color:10217983,transparent:!0,opacity:.75,blending:rn,depthWrite:!1,toneMapped:!1}),L=new _s(E,P);L.position.y=4.6,r.add(L),o.push(v,E),a.push(P);let H=new Tt({color:4175871,transparent:!0,opacity:.12,blending:rn,depthWrite:!1,toneMapped:!1}),O=c(new lr(1.1,1),H);L.add(O);let U=160,V=new Float32Array(U);for(let oe=0;oe<U;oe++)V[oe]=oe*.618033988749895%1;let F=new ft;F.setAttribute("position",new vt(new Float32Array(U*3),3)),F.setAttribute("seed",new vt(V,1)),F.boundingSphere=new nn(new D(0,ic/2,0),12);let Y=Er(Dv,{time:d.time,pointScale:{value:800},color:{value:f},strength:{value:.9}}),J=new Fn(F,Y);J.frustumCulled=!1,r.add(J),o.push(F),a.push(Y);let j=new Xn(6478079,26,46,2);j.position.y=6,r.add(j);function me(oe,ue,se,ce){let ve=document.createElement("canvas");ve.width=se,ve.height=ce;let Re=ve.getContext("2d"),We=new xs(ve);We.colorSpace=Pt,We.generateMipmaps=!1,We.minFilter=Lt,l.push(We);let lt=Er(iu,{map:{value:We},time:d.time,glitch:{value:0},fade:{value:0},gain:{value:1.35},tint:{value:new Me(16777215)}},{side:Sn}),ct=c(new sn(oe,ue),lt);return{canvas:ve,context:Re,texture:We,material:lt,mesh:ct,text:"",targetFade:1,next:0}}let _e=me(17.5,8.55,1024,500),Ve=[me(8.6,2.1,640,156),me(8.6,2.1,640,156)];_e.mesh.position.y=10.4,r.add(_e.mesh),Ve.forEach((oe,ue)=>{oe.mesh.position.y=ue?14.4:5.9,oe.orbit=ue?-.22:.27,oe.angle=ue*2.3,r.add(oe.mesh)});function it(oe,ue,se){let{context:ce,canvas:ve}=_e,Re=ve.width,We=ve.height;ce.clearRect(0,0,Re,We),ce.fillStyle="rgba(48,150,214,0.14)",ce.beginPath(),ce.roundRect(14,14,Re-28,We-28,22),ce.fill(),ce.strokeStyle="rgba(150,230,255,0.85)",ce.lineWidth=3;for(let[lt,ct,k,ne]of[[18,18,1,1],[Re-18,18,-1,1],[18,We-18,1,-1],[Re-18,We-18,-1,-1]])ce.beginPath(),ce.moveTo(lt,ct+ne*42),ce.lineTo(lt,ct),ce.lineTo(lt+k*42,ct),ce.stroke();ce.fillStyle="rgba(150,230,255,0.55)",ce.fillRect(48,108,Re-96,2),ce.font="600 27px "+Oo,ce.textBaseline="middle",ce.fillStyle="rgba(160,232,255,0.92)",ce.textAlign="left",ce.fillText("\u258C "+i+(se?"  \xB7  "+String(ue+1).padStart(2,"0")+" / "+String(se).padStart(2,"0"):""),50,72),ce.textAlign="right",ce.font="500 25px "+Oo,ce.fillStyle="rgba(190,150,255,0.85)",ce.fillText("0x"+Pv(oe||i).toUpperCase(),Re-52,72),ce.textAlign="left",ce.shadowColor="rgba(120,225,255,0.9)",ce.shadowBlur=16,oe?(ce.font="600 54px "+Oo,ce.fillStyle="rgba(232,250,255,0.97)",Ud(ce,oe,Re-110,4).forEach((ct,k)=>ce.fillText(ct,54,172+k*72))):(ce.font="600 40px "+Oo,ce.fillStyle="rgba(180,235,255,0.7)",ce.fillText("\u25AE \u25AE \u25AF \u25AE \u25AF \u25AF \u25AE \u25AF \u25AE \u25AE \u25AF \u25AE",54,250)),ce.shadowBlur=0,_e.texture.needsUpdate=!0}function Fe(oe,ue){let{context:se,canvas:ce}=oe,ve=ce.width,Re=ce.height;se.clearRect(0,0,ve,Re),se.fillStyle="rgba(48,150,214,0.12)",se.beginPath(),se.roundRect(6,6,ve-12,Re-12,14),se.fill(),se.fillStyle="rgba(190,150,255,0.8)",se.fillRect(20,26,6,Re-52),se.font="600 42px "+Oo,se.textBaseline="middle",se.textAlign="left",se.shadowColor="rgba(160,140,255,0.9)",se.shadowBlur=12,se.fillStyle="rgba(236,240,255,0.95)",se.fillText(ue?Ud(se,ue,ve-70,1)[0]:"\u25AF \u25AE \u25AF \u25AE \u25AF",42,Re/2),se.shadowBlur=0,oe.texture.needsUpdate=!0}function Q(){return m.length?(S>=p.length&&(p=m.map((oe,ue)=>ue).sort(()=>Math.random()-.5),S=0),m[p[S++]]):""}let ge=.8,X=[3.5,6.5];function he(oe){let ue=Q();_e.text=ue,M++,it(ue,m.indexOf(ue),m.length),_e.material.uniforms.glitch.value=oe?1:0,_e.material.uniforms.fade.value=oe?.35:1,ge=g?12:4.5+Math.random()*3}function $(oe,ue){let se=Ve[oe],ce=Q();se.text=ce,Fe(se,ce.length>46?ce.slice(0,44).replace(/\s+\S*$/,"")+"\u2026":ce),se.material.uniforms.glitch.value=ue?.7:0,se.material.uniforms.fade.value=ue?.3:1,X[oe]=g?15:6+Math.random()*4}it("",0,0),Ve.forEach(oe=>Fe(oe,"")),document.fonts?.ready?.then(()=>{b||(it(_e.text,m.indexOf(_e.text),m.length),Ve.forEach(oe=>Fe(oe,oe.text)))});let ie=new D;return{group:r,setTexts(oe,ue="live"){let se=Nv(oe),ce=se.length!==m.length||se.some((ve,Re)=>ve!==m[Re]);m=se,R=se.length?ue:"none",ce&&(p=[],S=0,(!_e.text||!m.includes(_e.text))&&(ge=Math.min(ge,.6)))},setPointScale(oe){Y.uniforms.pointScale.value=oe},setReducedMotion(oe){g=!!oe},update(oe,ue,se){if(b)return;let ce=se&&!g,ve=ce?Math.min(.1,Math.max(0,oe)):0;d.time.value+=ve,ge-=oe,ge<=0&&he(ce),X.forEach((Re,We)=>{X[We]=Re-oe,X[We]<=0&&$(We,ce)});for(let Re of[_e,...Ve]){let We=Re.material.uniforms;We.glitch.value=Math.max(0,We.glitch.value-oe*2.4),We.fade.value=Math.min(1,We.fade.value+oe*1.8),Re.mesh.quaternion.copy(ue.quaternion)}Ve.forEach(Re=>{Re.angle+=Re.orbit*ve,Re.mesh.position.x=Math.cos(Re.angle)*6.4,Re.mesh.position.z=Math.sin(Re.angle)*6.4,ie.copy(Re.mesh.position).add(r.position).sub(ue.position).normalize();let We=-(ie.x*Math.cos(Re.angle)+ie.z*Math.sin(Re.angle));Re.material.uniforms.gain.value=1.35*Vt.clamp(.35+We*.9,.15,1.2)}),_e.mesh.position.y=9.6+Math.sin(d.time.value*.9)*.22,L.rotation.y+=ve*.7,L.rotation.x+=ve*.31,O.rotation.y-=ve*1.1,C.rotation.z+=ve*.18,_.rotation.z-=ve*.42,j.intensity=26+Math.sin(d.time.value*2.1)*6+_e.material.uniforms.glitch.value*22},stats:()=>({artifacts:m.length,source:R,switches:M,shown:_e.text?m.indexOf(_e.text):-1}),dispose(){b||(b=!0,r.removeFromParent(),o.forEach(oe=>oe.dispose()),a.forEach(oe=>oe.dispose()),l.forEach(oe=>oe.dispose()),j.dispose())}}}var Uv=`float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`,su="varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}",Fv=`uniform float time;uniform vec3 zenith,horizon,haze,warm,sunDir;varying vec3 vWorld;
void main(){vec3 d=normalize(vWorld);float h=clamp(d.y,-0.08,1.0);
vec3 col=mix(horizon,zenith,pow(smoothstep(0.0,0.62,h),0.55));
vec3 level=normalize(vec3(d.x,0.0,d.z)+vec3(0.0,0.0,1e-4));
float az=max(0.0,dot(level,normalize(vec3(sunDir.x,0.0,sunDir.z))));
col+=warm*pow(az,5.0)*exp(-max(h,0.0)*8.0)*0.85;
float bend=atan(d.x,d.z);
float b1=(h-0.2+0.05*sin(bend*3.0+time*0.07))*13.0,b2=(h-0.33+0.04*sin(bend*2.5-time*0.05+1.7))*17.0;
float a1=exp(-b1*b1),a2=exp(-b2*b2);
float ripple=0.5+0.5*sin(bend*7.0+time*0.16+sin(bend*3.0)*1.3);
col+=vec3(0.08,0.32,0.34)*a1*ripple*0.5+vec3(0.26,0.12,0.4)*a2*(1.0-ripple)*0.42;
col=mix(haze,col,smoothstep(0.0,0.14,h));
col=mix(col,haze*0.6,smoothstep(0.0,0.08,-d.y));
vec3 s=normalize(sunDir);float mu=max(0.0,dot(d,s));
float disc=smoothstep(0.99955,0.99982,mu);
float halo=pow(mu,220.0)*0.55+pow(mu,48.0)*0.12;
col+=vec3(0.92,0.95,1.06)*disc*2.4+vec3(0.45,0.62,1.0)*halo;
gl_FragColor=vec4(col,1.0);}`,Od={vertex:`attribute float phase,speed,size;uniform float time,pointScale;varying float vAlpha;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0);}`,fragment:"varying float vAlpha;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vec3(0.82,0.9,1.0)*a,1.0);}"},Bd={vertex:`varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,fragment:`uniform float time;uniform vec3 deep,shallow,sky,sunDir,sunColor;varying vec3 vWorld;
#include <fog_pars_fragment>
float height(vec2 p){return 0.36*sin(p.x*0.09+time*0.9+sin(p.y*0.07)*1.5)+0.26*sin(p.y*0.11-time*0.7+p.x*0.03)
+0.12*sin((p.x+p.y)*0.21+time*1.4)+0.07*sin(p.x*0.52-p.y*0.37+time*2.2);}
void main(){vec2 p=vWorld.xz;float e=0.6;
vec3 n=normalize(vec3(height(p-vec2(e,0.0))-height(p+vec2(e,0.0)),2.0*e*0.9,height(p-vec2(0.0,e))-height(p+vec2(0.0,e))));
vec3 view=normalize(cameraPosition-vWorld);float fresnel=pow(clamp(1.0-dot(n,view),0.0,1.0),4.0);
vec3 col=mix(deep,shallow,clamp(0.5+n.x*2.5,0.0,1.0));col=mix(col,sky,0.22+0.62*fresnel);
vec3 refl=reflect(-view,n);float spec=pow(max(dot(refl,sunDir),0.0),260.0)*2.6+pow(max(dot(refl,sunDir),0.0),28.0)*0.22;
float sparkle=pow(max(0.0,sin(p.x*3.7+time*3.0)*sin(p.y*4.1-time*2.3)),40.0)*0.35*max(0.0,dot(refl,sunDir));
col+=sunColor*(spec+sparkle);gl_FragColor=vec4(col,1.0);
#include <fog_fragment>
}`},Ov=`uniform float time,strength;uniform vec3 color;uniform vec4 island;varying vec3 vWorld;${Uv}
void main(){vec2 p=vWorld.xz;float n=vnoise(p*0.012+vec2(time*0.017,-time*0.011))*0.6+vnoise(p*0.031-vec2(time*0.02,time*0.013))*0.4;
float mist=smoothstep(0.32,0.82,n);vec2 d2=max(abs(p-island.xy)-island.zw,0.0);float outside=smoothstep(0.0,70.0,length(d2));
float far=1.0-smoothstep(500.0,850.0,length(p));gl_FragColor=vec4(color,mist*outside*far*strength);}`,zd={vertex:`attribute vec3 seed;uniform float time,pointScale;varying float vAlpha;
void main(){vec3 p=position;p.x+=sin(time*0.11*seed.x+seed.y*6.28)*9.0+time*0.35*(seed.z-0.5);p.y+=sin(time*0.13+seed.z*6.28)*3.0;
p.z+=cos(time*0.09*seed.y+seed.x*6.28)*9.0;p.x=mod(p.x+90.0,180.0)-90.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.06+0.11*seed.x)*pointScale/max(1.0,-mv.z);vAlpha=0.55+0.45*sin(time*(1.0+seed.y)+seed.z*6.28);}`,fragment:`uniform vec3 color;uniform float strength;varying float vAlpha;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,2.2)*vAlpha*strength,1.0);}`},Bv=`uniform float strength;uniform vec3 color;varying vec2 vUv;
void main(){float along=pow(clamp(1.0-vUv.x,0.0,1.0),2.4);float across=pow(clamp(1.0-abs(vUv.y-0.5)*2.0,0.0,1.0),1.7);gl_FragColor=vec4(color*along*across*strength,1.0);}`,zv=`uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;
void main(){float rim=clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0);float a=pow(clamp(vUv.y,0.0,1.0),1.6)*(0.35+0.65*rim)*strength;gl_FragColor=vec4(color*a,1.0);}`,kv=`uniform sampler2D tDiffuse;uniform float time,vignette,grain;varying vec2 vUv;
float hash2(vec2 p){return fract(sin(dot(p,vec2(12.9898,78.233)))*43758.5453);}
void main(){vec4 c=texture2D(tDiffuse,vUv);vec2 d=vUv-0.5;float v=1.0-vignette*smoothstep(0.28,1.05,dot(d,d)*2.4);
float g=(hash2(vUv*vec2(1920.0,1080.0)+fract(time)*7.0)-0.5)*grain;gl_FragColor=vec4(c.rgb*v+g,c.a);}`;function kd(s,e={}){let t=[],n=[],i=new ut;i.name="city-atmosphere",s.add(i);let r=(se,ce)=>(t.push(se),n.push(ce),new Je(se,ce)),o={value:0},a={value:900},l=(e.sunDirection||new D(-70,145,85)).clone().normalize(),c=new Me(330010),h=new Me(1323596),u=new Me(8011050),f=new Me(s.fog?.color||528926),d=(se,ce,ve,Re={})=>new mt({uniforms:ve,vertexShader:se,fragmentShader:ce,...Re}),g={transparent:!0,depthWrite:!1,blending:rn},M=r(new ri(1500,48,24),d(su,Fv,{time:o,zenith:{value:c},horizon:{value:h},haze:{value:f},warm:{value:u},sunDir:{value:l}},{side:Wt,depthWrite:!1,fog:!1}));M.frustumCulled=!1,M.renderOrder=-10,i.add(M);let m=1400,p=new Float32Array(m*3),S=new Float32Array(m),R=new Float32Array(m),b=new Float32Array(m),w=12345,y=()=>(w=Math.imul(w,1664525)+1013904223>>>0)/4294967296;for(let se=0;se<m;se++){let ce=y()*Math.PI*2,ve=.03+y()*.97,Re=Math.sqrt(1-ve*ve);p.set([Math.cos(ce)*Re*1450,ve*1450,Math.sin(ce)*Re*1450],se*3),S[se]=y()*Math.PI*2,R[se]=.4+y()*1.6,b[se]=8e-4+y()*y()*.0022}let C=new ft;C.setAttribute("position",new vt(p,3)),C.setAttribute("phase",new vt(S,1)),C.setAttribute("speed",new vt(R,1)),C.setAttribute("size",new vt(b,1));let _=d(Od.vertex,Od.fragment,{time:o,pointScale:a},{...g,fog:!1}),v=new Fn(C,_);v.frustumCulled=!1,v.renderOrder=-9,i.add(v),t.push(C),n.push(_);let E=r(new sn(1800,1800),d(Bd.vertex,Bd.fragment,{time:o,deep:{value:new Me(398368)},shallow:{value:new Me(930640)},sky:{value:h.clone().multiplyScalar(.7)},sunDir:{value:l},sunColor:{value:new Me(14676223)},fogColor:{value:new Me},fogDensity:{value:0},fogNear:{value:1},fogFar:{value:1e3}},{fog:!0}));E.rotation.x=-Math.PI/2,E.position.y=-3.2,i.add(E);let P=r(new sn(1900,1900),d(su,Ov,{time:o,strength:{value:.55},color:{value:new Me(1717320)},island:{value:new xt(0,-7,85,87)}},{transparent:!0,depthWrite:!1,fog:!1}));P.rotation.x=-Math.PI/2,P.position.y=-2.4,i.add(P);let L=500,H=new Float32Array(L*3),O=new Float32Array(L*3);for(let se=0;se<L;se++)H.set([(y()-.5)*180,2+y()*y()*75,-95+y()*170],se*3),O.set([y(),y(),y()],se*3);let U=new ft;U.setAttribute("position",new vt(H,3)),U.setAttribute("seed",new vt(O,3));let V=d(zd.vertex,zd.fragment,{time:o,pointScale:a,color:{value:new Me(10475775)},strength:{value:.32}},g),F=new Fn(U,V);F.frustumCulled=!1,i.add(F),t.push(U),n.push(V);let Y=new ut;Y.position.set(0,86.5,-12),i.add(Y);let J=d(su,Bv,{strength:{value:.45},color:{value:new Me(9430783)}},{...g,side:Bt}),j=new sn(100,5.2);j.translate(50,0,0),t.push(j),n.push(J);let me=new Je(j,J),_e=new Je(j,J);_e.rotation.x=Math.PI/2;let Ve=new ut;Ve.add(me,_e),Ve.rotation.z=-.07,Y.add(Ve);let it=new Tt({color:12579583,toneMapped:!1}),Fe=r(new ri(.9,16,12),it);Y.add(Fe);let Q=(e.lamps||[]).slice(0,64),ge=d("varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",zv,{color:{value:new Me(16767392)},strength:{value:.3}},{...g,side:Bt}),X=new vs(2.7,6.1,18,1,!0),he=new fn(X,ge,Math.max(1,Q.length));t.push(X),n.push(ge);let $=new yt;Q.forEach((se,ce)=>{$.position.set(se.x+1.9*Math.cos(se.angle||0),3.35,se.z-1.9*Math.sin(se.angle||0)),$.updateMatrix(),he.setMatrixAt(ce,$.matrix)}),he.count=Q.length,he.instanceMatrix.needsUpdate=!0,i.add(he);let ie=new Sr(new mt({uniforms:{tDiffuse:{value:null},time:o,vignette:{value:.42},grain:{value:.028}},vertexShader:Hv(),fragmentShader:kv}));ie.enabled=!1,n.push(ie.material);let oe=!1,ue="high";return{post:ie,sea:E,setLighting(se,ce,ve){l.copy(ve).normalize(),c.set(330010).lerp(new Me(2647428),se),h.set(1323596).lerp(new Me(10467773),se),u.set(ce>.1?14315834:8011050),f.copy(s.fog.color),v.visible=se<.35,E.material.uniforms.sky.value.copy(h).multiplyScalar(.7)},setTier(se){ue=se;let ce=ue!=="low";P.visible=ce,F.visible=ce,he.visible=ce,ie.enabled=ue==="high"||ue==="ultra"},setBusy(se){oe=!!se},setPointScale(se){a.value=se},update(se,ce,ve,Re){let We=Re?Math.min(.1,Math.max(0,se)):0;o.value+=We,s.fog&&(E.material.uniforms.fogColor.value.copy(s.fog.color),s.fog.isFogExp2&&(E.material.uniforms.fogDensity.value=s.fog.density)),E.position.x=ve.position.x,E.position.z=ve.position.z;let lt=oe?1.15:.32,ct=Y.userData.rate??lt;Y.userData.rate=ct+(lt-ct)*Math.min(1,We*1.5),Ve.rotation.y+=We*Y.userData.rate,J.uniforms.strength.value=oe?.85:.45,Fe.scale.setScalar(1+Math.sin(o.value*(oe?6:2.2))*.18)},stats:()=>({tier:ue,busy:oe,stars:m,dust:L,lamps:Q.length,post:ie.enabled}),dispose(){i.removeFromParent(),he.dispose(),t.forEach(se=>se.dispose()),n.forEach(se=>se.dispose()),ie.dispose?.()}}}function Hv(){return"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}"}var Hd=[{points:[[-62,34,-62],[8,44,-74],[62,38,-22],[56,47,42],[-8,41,62],[-64,36,12]],speed:9.5},{points:[[30,56,-12],[0,62,18],[-30,58,-12],[0,66,-42]],speed:8},{points:[[42,29,-56],[-28,33,-42],[-52,27,20],[18,31,52],[60,33,8]],speed:11}];function Vd(s,e={}){let t=new ut;t.name="city-drones",s.add(t);let n=[],i=[],r=[],o=new D,a=new D,l=new D(0,1,0),c=new Ke,h=new It,u=new D,f=new ri(.16,8,6);n.push(f);let d=w=>{let y=new Tt({color:w,toneMapped:!1});return i.push(y),y},g=d(16730684),M=d(5046154),m=d(16777215);[...Hd,...Hd].forEach((w,y)=>{let C=new or(w.points.map(P=>new D(P[0],P[1]+(y>=3?8:0),P[2])),!0,"centripetal",.6),_=new ut;_.name="city-drone-"+(y+1),_.visible=!1;let v=new ut;_.add(v);let E=[new Je(f,g),new Je(f,M),new Je(f,m)];E[0].position.set(-2.6,.8,0),E[1].position.set(2.6,.8,0),E[2].position.set(0,2.1,-.4),_.add(...E),t.add(_),r.push({root:_,body:v,lights:E,curve:C,length:C.getLength(),speed:w.speed,t:y*.37,rotors:[],roll:0})});let p=!0,S="high",R=!1;function b(w,y){let{curve:C,root:_}=w;w.t=(w.t+y*w.speed/w.length)%1,C.getPointAt(w.t,_.position),C.getTangentAt(w.t,o),C.getTangentAt((w.t+.015)%1,a),c.lookAt(o,u,l),h.setFromRotationMatrix(c),_.quaternion.slerp(h,y>0?Math.min(1,y*4):1);let v=Math.atan2(a.x,a.z)-Math.atan2(o.x,o.z),E=Math.atan2(Math.sin(v),Math.cos(v));w.roll+=(Vt.clamp(E*12,-.55,.55)-w.roll)*(y>0?Math.min(1,y*3):1),w.body.rotation.z=w.roll}return{setTier(w){S=w},setTemplate(w){if(w){R=!0;for(let[y,C]of r.entries()){C.body.clear(),C.rotors.length=0;let _=w.clone(!0);_.scale.setScalar(1.9),_.traverse(v=>{v.isMesh&&(v.castShadow=!1,v.receiveShadow=!1),/^rotor_/.test(v.name)&&C.rotors.push(v)}),C.body.add(_),C.root.visible=y<(S==="low"?2:S==="medium"?3:6)}}},update(w,y,C){p=C;let _=C?Math.min(.1,Math.max(0,w)):0;r.forEach((v,E)=>{if(v.root.visible=R&&E<(S==="low"?2:S==="medium"?3:6),!v.root.visible)return;b(v,_),v.rotors.forEach((L,H)=>{L.rotation.y+=_*(H%2?-46:46)});let P=Math.sin(y*5+E*1.7);v.lights[0].visible=v.lights[1].visible=!C||P>-.2,v.lights[2].visible=!C||P>.93,v.lights[2].scale.setScalar(C?1.6:1)})},stats:()=>({drones:r.filter(w=>w.root.visible).length,animated:p,positions:r.map(w=>w.root.position.toArray().map(y=>Math.round(y)))}),dispose(){t.removeFromParent(),n.forEach(w=>w.dispose()),i.forEach(w=>w.dispose()),r.length=0}}}var xn={xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80},ht={ground:0,road:.12,pavement:.5,quay:.08,room:.16,gallery:4.16},vn=["agent","memory","missions"].map((s,e)=>{let t=[0,-43,43][e],n=73;return{id:s,x:t,z:n,width:12,depth:12,front:-1,doorZ:n-6,liftX:t+4,liftZ:n+3}}),Jn=[{id:"infra",x:-67,z:-53,platformX:-74,platformZ:-53,angle:Math.PI/2},{id:"memory",x:-67,z:-10,platformX:-74,platformZ:-10,angle:Math.PI/2},{id:"graph",x:-43,z:59,platformX:-43,platformZ:64.5,angle:0},{id:"operations",x:0,z:59,platformX:0,platformZ:64.5,angle:0},{id:"missions",x:43,z:59,platformX:43,platformZ:64.5,angle:0},{id:"integrations",x:67,z:-10,platformX:74,platformZ:-10,angle:-Math.PI/2},{id:"agent",x:0,z:-77,platformX:0,platformZ:-84,angle:Math.PI}],ru=[[-67,-77],[-67,59],[67,59],[67,-77]],di={x:78,z:30},sc=[{x:30.5,z:-55,scale:1},{x:49,z:-55,scale:1.25},{x:-3,z:-57,scale:1.2}];function Gd(s,e,t=xn.halfWidth){return s>=xn.minX&&s<=xn.maxX&&e>=xn.minZ&&e<=xn.maxZ&&(xn.xs.some(n=>Math.abs(s-n)<=t)||xn.zs.some(n=>Math.abs(e-n)<=t))}function Wd(s,e){return Gd(s,e)?Gd(s,e,xn.laneHalfWidth)?ht.road:ht.pavement:ht.ground}function Vv(s,e,t,n=0){return e>=s.x-5.85+n&&e<=s.x+2.4-n&&t>=s.z+n&&t<=s.z+5.85-n}function Xd(s,e,t,n=0){return Math.abs(e-s.liftX)<=1.6-n&&Math.abs(t-s.liftZ)<=1.6-n}function qd(){let s=[],e=[2.4,1.4,5.6,4.6],t=(n,i,r,o)=>{r>n&&o>i&&s.push({x:(n+r)/2,z:(i+o)/2,sx:(r-n)/4,sz:(o-i)/4})};for(let n=-4;n<=4;n+=4)for(let i=-4;i<=4;i+=4){let[r,o,a,l]=[n-2,i-2,n+2,i+2],[c,h,u,f]=[Math.max(r,e[0]),Math.max(o,e[1]),Math.min(a,e[2]),Math.min(l,e[3])];if(c>=u||h>=f){t(r,o,a,l);continue}t(r,o,c,l),t(u,o,a,l),t(c,o,u,h),t(c,f,u,l)}return s}function ou(s,e,t,n){return Vv(s,e,t,.25)?!0:n?e>=s.x+1.8&&e<=s.liftX+1.25&&Math.abs(t-s.liftZ)<1.25:!1}var Gv={agent:[[-8,-8,8,8]],memory:[[-11.4,-10.4,11.4,8.4]],integrations:[[-11.4,-4.9,-4.6,4.9],[4.6,-4.9,11.4,4.9],[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]],missions:[[-13.4,-8.9,13.4,6.9]],infra:[[-10.7,-9.4,10.7,3.4],[-10,-2.6,10,8.5]]},Wv={agent:[23,23],memory:[26,22],integrations:[26,16],missions:[30,21],infra:[25,23],graph:[24,24],operations:[9,9]};function Yd(s,e,t){for(let n of t){let i=Wv[n.id];if(i&&Math.abs(s-n.x)<i[0]/2&&Math.abs(e-n.z)<i[1]/2)return .88}return 0}function Xv(s,e,t){for(let n of t){let i=s-n.x,r=e-n.z;if(n.id==="graph"&&Math.hypot(i,r)<10.2||n.id==="operations"&&Math.hypot(i,r)<3||(Gv[n.id]||[]).some(([o,a,l,c])=>i>o&&i<l&&r>a&&r<c))return!0}return sc.some(n=>Math.abs(s-n.x)<6&&Math.abs(e-n.z)<5)}function Rs(s,e){return vn.find(t=>Math.abs(s-t.x)<t.width/2&&Math.abs(e-t.z)<t.depth/2)}function wr(s,e){if(s>=-81.5&&s<=-78.5&&e>=46&&e<=52)return(52-e)/3;if(s>=-78.3&&s<=-75.3&&e>=46&&e<=52)return Math.min(2,Math.ceil((52-e)*2)/6);if(s>=-82&&s<=-75&&e>=34&&e<46)return 2;if(s>=-80&&s<=-77&&e>=28&&e<34)return(e-28)/3;if(Rs(s,e))return ht.room;for(let t of Jn){let n=!!t.angle&&Math.abs(t.angle)!==Math.PI;if(Math.abs(s-t.platformX)<(n?2:4.5)&&Math.abs(e-t.platformZ)<(n?4.5:2))return ht.pavement+.3}return Math.max(Wd(s,e),e>=74&&e<=80&&s>=-80&&s<=80?ht.quay:ht.ground)}function Zd(s,e,t,n,i){if(Math.abs(s)>84||e<-90||e>79)return!1;let r=Rs(s,e),o=Rs(t.x,t.z);if(r||o){let a=r||o;return!(r?.id!==o?.id&&(Math.abs(s-a.x)>1.15||Math.abs(t.x-a.x)>1.15||Math.min((e-a.doorZ)*a.front,(t.z-a.doorZ)*a.front)<-.9||!n(a.id))||r&&(Math.abs(s-a.x)>5.55||(e-a.z)*a.front<-5.55))}return!Xv(s,e,i)}function au(s,e=new Date){let t=s==="day"?12:s==="evening"?18.5:s==="night"?0:e.getHours()+e.getMinutes()/60;return{hour:t,amount:Math.max(0,Math.min(1,Math.sin((t-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(t-18.5)/2)}}function Kd(s,e){let t=new ut;t.name="world-2",s.add(t);let n=new Map,i=new Set,r=new Set,o=[],a=[],l=[],c=new Map,h=new Map,u=new Map,f=new Map,d=new Set,g=[],M=new Set,m=[],p=new Map,S=new AbortController,R=[],b=0,w=!1,y=!1,C=e.tier||"high",_=null,v=0,E=0,P=null,L=null,H=0,O=new Set,U=null,V=0,F=new D,Y=new D,J=new D,j=e.camera;try{O=new Set(JSON.parse(localStorage.getItem("aurago.desktop.sysworld.discoveries")||"[]").filter(k=>Jn.some(ne=>ne.id===k)))}catch{}let me=nc(ru),_e=me.getLength(),Ve=nc([[-71.65,-81.65],[-71.65,63.65],[71.65,63.65],[71.65,-81.65]],2),it=Ve.getLength(),Fe=Jn.map(k=>{let ne=1/0,I=0;for(let fe=0;fe<3e3;fe++){me.getPointAt(fe/3e3,F);let N=Math.hypot(F.x-k.x,F.z-k.z);N<ne&&(ne=N,I=fe/3e3)}return{...k,u:I}}),Q=(k,ne)=>e.onSound?.(k,ne?.x||0,ne?.y||0,ne?.z||0,!!U),ge=new dn({color:11834208,metalness:.8,roughness:.35});r.add(ge);for(let k of[-1,1]){let ne=[],I=[];for(let T=0;T<=512;T++){me.getPointAt(T/512,F),me.getTangentAt(T/512,Y);for(let x of[-.045,.045]){let B=k*.85+x;ne.push(F.x-Y.z*B,.21,F.z+Y.x*B)}if(T<512){let x=T*2;I.push(x,x+1,x+2,x+1,x+3,x+2)}}let N=new ft;N.setAttribute("position",new ot(ne,3)),N.setIndex(I),N.computeVertexNormals(),i.add(N),t.add(new Je(N,ge))}function X(){return C==="low"?3:C==="medium"?10:19}async function he(k,ne=C==="low"?2:C==="medium"?1:0){let I=k+":"+ne;return n.has(I)||n.set(I,(async()=>{let fe=_.assets.find(G=>G.id===k),N=fe?.lods.find(G=>G.level===ne);if(!N||!/^[\w-]+\.lod[0-2]\.glb$/.test(N.file))throw Error("Invalid world asset");let T=await fetch(e.assetURL(N.file),{signal:S.signal});if(!T.ok)throw Error("World asset unavailable");let x=await T.arrayBuffer();if(y)throw Error("Disposed");v+=x.byteLength;let B=await new ns().parseAsync(x,"");if(B.scene.traverse(G=>{if(G.isMesh){i.add(G.geometry);for(let ee of Array.isArray(G.material)?G.material:[G.material])r.add(ee);G.castShadow=!0,G.receiveShadow=!0}}),y)throw i.forEach(G=>G.dispose()),r.forEach(G=>G.dispose()),Error("Disposed");return d.add(k),B})()),n.get(I)}async function $(k,ne,I,fe=0,N=0,T=t,x=[1,1,1]){let B=await he(k);if(y)return null;let G=B.scene.clone(!0);G.position.set(ne,fe,I),G.rotation.y=N,G.scale.set(...x),T.add(G),G.traverse(pe=>{pe.isMesh&&(pe.userData.worldAsset=k,pe.userData.worldPart=pe.name,g.push(pe))});let ee=B.animations.length?new uo(G):null;return ee&&o.push(ee),{node:G,mixer:ee,clips:B.animations,play(pe,be=!1){if(!ee)return;let re=B.animations.find(Te=>Te.name===pe);if(!re)return;ee.stopAllAction();let ae=ee.clipAction(re).reset();return be&&(ae.setLoop(Bl,1),ae.clampWhenFinished=!0),ae.play(),ae}}}function ie(k){k.updateMatrixWorld(!0);let ne=new Map;k.traverse(fe=>{if(!fe.isMesh)return;let N=fe.geometry.uuid+":"+(Array.isArray(fe.material)?fe.material.map(T=>T.uuid).join(","):fe.material.uuid);ne.has(N)||ne.set(N,{n:fe,matrices:[]}),ne.get(N).matrices.push(fe.matrixWorld.clone())});let I=new ut;t.add(I);for(let{n:fe,matrices:N}of ne.values()){let T=new fn(fe.geometry,fe.material,N.length);N.forEach((x,B)=>T.setMatrixAt(B,x)),T.castShadow=!0,T.receiveShadow=!0,T.userData={...fe.userData},g.push(T),I.add(T)}return k.traverse(fe=>{let N=g.indexOf(fe);N>=0&&g.splice(N,1)}),k.removeFromParent(),u.set(I.uuid,I),I}function oe(k){if(f.has(k.id))return f.get(k.id);let ne=(async()=>{let I=new ut;t.add(I);let fe=[];for(let N of qd())fe.push($("floor",k.x+N.x,k.z+N.z,ht.room,0,I,[N.sx,1,N.sz]));for(let N=-4;N<=4;N+=4)for(let T=-4;T<=4;T+=4)fe.push($("ceiling",k.x+N,k.z+T,8.15+ht.room,0,I));for(let N=-4;N<=4;N+=4)fe.push($("wall",k.x+N,k.z+6,ht.room,0,I));for(let N=-4;N<=4;N+=4)for(let T of[-1,1])for(let x of[0,4])fe.push($("window",k.x+T*6,k.z+N,ht.room+x,Math.PI/2,I));for(let N=-4;N<=4;N+=4)for(let T of[-1,1])fe.push($("window",k.x+N,k.z+T*6,ht.gallery,0,I));for(let N of[-4,4])fe.push($("wall",k.x+N,k.doorZ,ht.room,0,I));for(let[N,T]of[[-4,1],[.2,1.1]])fe.push($("floor",k.x+N,k.z+3,ht.gallery,0,I,[T,1,1.5])),fe.push($("railing",k.x+N,k.z,ht.gallery,0,I,[T,1,1]));for(let N of[.7,5.3])fe.push($("railing",k.x+2.4,k.z+N,ht.gallery,Math.PI/2,I,[.35,1,1]));for(let N of[-5.6,1.9])fe.push($("wall",k.x+N,k.z+.15,ht.room,0,I,[.05,1,.6]));if(await Promise.all(fe),!y)return ie(I)})();return f.set(k.id,ne),ne}async function ue(k){if(!(c.has(k.id)||y)){c.set(k.id,{ready:!1,lift:null,liftValue:0,liftTarget:0}),V++;try{await oe(k);let ne=new ut;t.add(ne);let I=[];for(let N of[-3,0])I.push($(k.id==="missions"?"cargo":k.id==="memory"?"archive-shelf":"console",k.x+N,k.z+3,ht.room,Math.PI,ne));if(I.push($("bench",k.x-4,k.z-1,ht.room,Math.PI/2,ne)),I.push($("bench",k.x-4,k.z+4.8,ht.gallery,0,ne)),I.push($("console",k.x,k.z+4.8,ht.gallery,Math.PI,ne)),await Promise.all(I),y)return;ie(ne);let fe=c.get(k.id);fe.hologram=await $("hologram",k.x-1,k.z,ht.room),fe.hologram?.play("operate"),k.id==="memory"&&ce(),fe.lift=await $("lift",k.liftX,k.liftZ,ht.room),fe.liftAction=fe.lift?.play("operate",!0),fe.liftAction&&(fe.liftAction.paused=!0),fe.ready=!0}catch(ne){y||e.onError?.(ne)}finally{V--}}}async function se(){try{let k=await fetch(e.assetURL("manifest.json"),{signal:S.signal});if(!k.ok)throw Error("World manifest unavailable");_=await k.json();let ne=new ut;t.add(ne);let I=[];for(let N of Jn)I.push($("station",N.platformX,N.platformZ,ht.pavement,N.angle,ne));for(let N of vn){I.push(oe(N));let T=await $("door",N.x,N.doorZ,ht.room,Math.PI);if(y)return;let x=T.play("open",!0);x.paused=!0,h.set(N.id,{...T,action:x,value:0,open:!1})}for(let N=-76;N<=76;N+=8)I.push($("quay",N,77,ht.quay,0,ne));for(let[N,T]of[[-79,68],[-79,-67],[55,2],[8,40],[55,-24]])I.push($("garden",N,T,0,0,ne)),I.push($("bench",N+3,T,0,Math.PI/2,ne));for(let[N,T]of[[-30,23],[30,23],[-28,72]])I.push($("arcade",N,T,0,0,ne));I.push($("bridge",-78.5,40,2,0,ne,[1.75,1,1]),$("ramp",-80,49,0,Math.PI,ne),$("stairs",-76.8,49,0,Math.PI,ne),$("ramp",-78.5,31,0,0,ne)),I.push($("pad",di.x,di.z,0,0,ne));for(let[N,T]of[[-60,-56],[-60,-46],[51,49]])I.push($("charger",N,T,0,0,ne));if(await Promise.all(I),y)return;ie(ne);for(let[N,T]of[[-57,-61],[-57,-52]])(await $("cooler",N,T))?.play("operate");for(let N=0;N<19;N++){let T=["courier","technician","archivist"][N%3],x=await $("robot-"+T,0,0);if(!x)return;x.node.scale.setScalar(1.15),a.push({...x,u:N/19,state:"idle",type:T,phase:N*.53}),x.play("walk")}for(let N=0;N<2;N++){let T=await $("tram",0,0);if(!T)return;let x=T.play("open",!0);x.paused=!0,l.push({...T,u:Fe[N?3:0].u,dwell:5,stop:N?3:0,door:x})}let fe=await $("service-cart",55,49,ht.ground);fe&&(fe.node.rotation.y=Math.PI/2,fe.play("open",!0));for(let N=0;N<3;N++){let T=await $("cargo",43,70);T&&(T.node.visible=!1,m.push({...T,elapsed:9,direction:1}))}e.onReady?.()}catch(k){y||e.onError?.(k)}}se();function ce(){let k=c.get("memory");if(!k?.hologram||!k.text&&!R.length)return;if(!k.text){let T=document.createElement("canvas");T.width=1024,T.height=512;let x=new xs(T);x.colorSpace=Pt,M.add(x);let B=new sn(4.8,2.4),G=new Tt({map:x,transparent:!0,depthWrite:!1,side:Bt,toneMapped:!1});i.add(B),r.add(G);let ee=new Je(B,G),pe=vn.find(be=>be.id==="memory");ee.position.set(pe.x-1,3.6+ht.room,pe.z+.3),ee.rotation.y=Math.PI,t.add(ee),k.text={canvas:T,texture:x,mesh:ee}}let{canvas:ne,texture:I,mesh:fe}=k.text,N=ne.getContext("2d");N.clearRect(0,0,1024,512),fe.visible=R.length>0,N.fillStyle="rgba(5,24,32,.87)",N.fillRect(0,0,1024,512),N.fillStyle="#b9f4ef",N.font="28px sans-serif",R.slice(0,4).forEach((T,x)=>{let B=Array.from(T).slice(0,96);N.fillText(B.slice(0,48).join(""),30,55+x*118),N.fillText(B.slice(48).join(""),30,94+x*118)}),I.needsUpdate=!0}function ve(){if(P){if(P.kind==="tram"&&l[P.index].dwell<=1){P.exitRequested=!0;return}Re();return}if(L){if(L.kind==="door"){let k=h.get(L.id);if(k){k.open=!k.open,Q("door",k.node.position);let ne=vn.find(I=>I.id===L.id);ue(ne)}}if(L.kind==="discover"){O.add(L.id);try{localStorage.setItem("aurago.desktop.sysworld.discoveries",JSON.stringify([...O]))}catch{}e.onDiscover?.(L.id),Q("discover",j.position)}if(L.kind==="tram"&&!e.reduced()&&(P={kind:"waiting",station:L.id},e.onRide?.("waiting")),L.kind==="terminal"&&e.onTerminal?.(L.id),L.kind==="drone"&&!e.reduced()&&(P={kind:"drone",elapsed:0,origin:j.position.clone()},e.onRide?.("drone"),Q("tram",j.position)),L.kind==="lift"){let k=c.get(L.id);if(k){let ne=j.position.y>5?4:0;if(Math.abs(k.liftValue-ne)>.02){k.liftTarget=ne;return}k.liftTarget=ne?0:4,P={kind:"lift",id:L.id},e.onRide?.("lift"),Q("lift",j.position)}}}}function Re(){if(P){if(P.kind==="lift"){let k=c.get(P.id),ne=vn.find(I=>I.id===P.id);k.liftTarget=k.liftValue<2?0:4,j.position.set(ne.x+1.5,2.4+ht.room+k.liftTarget,ne.liftZ)}if(P.kind==="tram"){let k=Jn.find(ne=>ne.id===P.station)||Jn[0];j.position.set(k.platformX,2.4+ht.pavement+.3,k.platformZ)}P.kind==="drone"&&j.position.copy(P.origin),P=null,e.onRide?.(null)}}function We(){if(P)return[{kind:"exit",id:P.kind,distance:0}];let k=[],ne=(I,fe,N,T,x)=>{let B=Math.hypot(j.position.x-N,j.position.z-T);B<x&&k.push({kind:I,id:fe,distance:B})};for(let I of vn)j.position.y<5&&ne("door",I.id,I.x,I.doorZ,3),c.get(I.id)?.ready&&(ne("lift",I.id,I.liftX,I.liftZ,2.3),j.position.y<5&&ne("terminal",I.id,I.x-2,I.z+3,2.5));if(j.position.y<5)for(let I of Jn)ne("tram",I.id,I.platformX,I.platformZ,4),O.has(I.id)||ne("discover",I.id,I.platformX+(I.angle===0?5:0),I.platformZ+(I.angle===0?0:5),3);return ne("drone","drone",di.x,di.z,5),k.sort((I,fe)=>I.distance-fe.distance)}function lt(k,ne,I){if(y||!_)return;let fe=ne?Math.min(k,.05):0;E+=fe,U=Rs(j.position.x,j.position.z)?.id||null;for(let N of vn)Math.hypot(j.position.x-N.x,j.position.z-N.z)<32&&ue(N);o.forEach(N=>{N.getRoot().visible&&N.update(fe)});for(let N of h.values())N.value=Vt.damp(N.value,N.open?1:0,5,Math.min(k,.1)),N.action.time=N.value*N.action.getClip().duration,N.mixer.update(0);for(let[N,T]of c)if(T.liftAction&&(T.liftValue=Vt.damp(T.liftValue,T.liftTarget,1.8,Math.min(k,.1)),Math.abs(T.liftValue-T.liftTarget)<.02&&(T.liftValue=T.liftTarget),T.liftAction.time=T.liftValue/4*T.liftAction.getClip().duration,T.lift.mixer.update(0),P?.kind==="lift"&&P.id===N)){let x=vn.find(B=>B.id===N);j.position.set(x.liftX,2.4+ht.room+T.liftValue,x.liftZ),T.liftValue===T.liftTarget&&(P=null,e.onRide?.(null))}if(a.forEach((N,T)=>{if(N.node.visible=T<X(),!N.node.visible)return;let x=j.position.distanceTo(N.node.position),B=(E+N.phase*6)%28,G=x<4?"greet":B>24?"work":B>20?"idle":N.type==="courier"?"carry":"walk";if(G!==N.state&&(N.state=G,N.play(G)),G==="walk"||G==="carry"){let ee=(N.u+fe*1.5/it)%1;a.some(pe=>pe!==N&&pe.node.visible&&(pe.u-N.u+1)%1*it<2.5)||(N.u=ee)}Ve.getPointAt(N.u,F),Ve.getTangentAt(N.u,Y),N.node.position.set(F.x,wr(F.x,F.z),F.z),N.node.rotation.y=Math.atan2(Y.x,Y.z),G==="greet"&&N.node.lookAt(j.position.x,N.node.position.y,j.position.z)}),l.forEach((N,T)=>{if(N.node.visible=T<(C==="low"?1:2),!!N.node.visible){if(N.dwell>0)N.dwell-=fe;else{let x=N.u;N.u=(N.u+fe*11/_e)%1;let B=Fe.findIndex(G=>(G.u-x+1)%1>0&&(G.u-x+1)%1<=fe*11/_e+1e-5);B>=0&&(N.stop=B,N.u=Fe[B].u,N.dwell=6,Q("tram",N.node.position))}if(me.getPointAt(N.u,N.node.position),N.node.position.y=ht.road+.03,me.getTangentAt(N.u,Y),N.node.rotation.y=Math.atan2(Y.x,Y.z),N.door.time=(N.dwell>1?1:0)*N.door.getClip().duration,N.mixer.update(0),P?.kind==="waiting"&&Fe[N.stop].id===P.station&&N.dwell>1&&(P={kind:"tram",index:T,station:P.station,offset:0},e.onRide?.("tram")),P?.kind==="tram"&&P.index===T){if(N.dwell>1&&(P.station=Fe[N.stop].id,P.exitRequested)){Re();return}j.position.copy(N.node.position).addScaledVector(Y,P.offset),j.position.y+=.9+1.4,j.lookAt(N.node.position.x+Y.x*16,j.position.y,N.node.position.z+Y.z*16)}}}),P?.kind==="drone"){P.elapsed+=Math.min(k,.1);let N=P.elapsed/40*Math.PI*2;j.position.set(Math.cos(N)*100,45+Math.sin(N*2)*12,Math.sin(N)*95-10),j.lookAt(0,22,-12),P.elapsed>=40&&Re()}if(L=I==="street"&&We()[0]||null,I==="street"&&!P){let N=j.position.distanceTo(J);N<3&&(H+=N),H>1.8&&(Q(U?"step_inside":"step",j.position),H=0)}J.copy(j.position),e.onEnvironment?.(!!U);for(let N of m){if(N.elapsed+=fe,N.node.visible=ne&&N.elapsed<8,!N.node.visible)continue;let T=N.direction>0?N.elapsed/8:1-N.elapsed/8;N.node.position.set(40+T*3,ht.room,70)}e.onInteraction?.(L,O.size,U)}async function ct(k){if(C=k,!_)return;let ne=k==="low"?2:k==="medium"?1:0;try{let I=new Map(await Promise.all([...d].map(async fe=>{let N=await he(fe,ne),T=new Map;return N.scene.traverse(x=>{x.isMesh&&T.set(x.name,x)}),[fe,T]})));if(y||C!==k)return;for(let fe of g){let N=I.get(fe.userData.worldAsset)?.get(fe.userData.worldPart);N&&(fe.geometry=N.geometry,fe.material=N.material)}e.onReady?.()}catch(I){y||e.onError?.(I)}}return{update:lt,interact:ve,endRide:Re,setTier:ct,setMemory(k){R=(Array.isArray(k)?k:[]).filter(ne=>typeof ne=="string").slice(0,8),ce()},setWorld(k,ne=!1){let I=k?.entities?.filter(N=>N.kind==="mission")||[],fe=new Set;for(let N of I){fe.add(N.id);let T=p.get(N.id);if(!ne&&e.active()&&w&&T!=null&&T!==N.state&&["running","completed","failed","cancelled"].includes(N.state)&&Date.now()-N.at<3e4){let x=m.find(B=>B.elapsed>=8);x&&(x.elapsed=0,x.direction=N.state==="running"?1:-1,b++)}p.set(N.id,N.state)}for(let N of p.keys())fe.has(N)||p.delete(N);if(w=!ne,ne)for(let N of m)N.elapsed=9,N.node.visible=!1},walkRide(k,ne){return P?(P.kind==="tram"&&(P.offset=Vt.clamp(P.offset+k*ne*3,-2.5,2.5)),!0):!1},move(k,ne,I){if(P)return!1;let fe=Rs(I.x,I.z);return fe&&I.y>5?Math.abs(ne-fe.z-4.8)<.85&&(Math.abs(k-fe.x)<1.1||Math.abs(k-fe.x+4)<1.75)?!1:ou(fe,k,ne,c.get(fe.id)?.liftValue===4):fe&&I.y<5&&([-5.6,1.9].some(N=>Math.abs(k-fe.x-N)<.25&&Math.abs(ne-fe.z-.15)<.25)||Xd(fe,k,ne)&&!(k<fe.liftX+1.25&&Math.abs(ne-fe.liftZ)<1.25&&c.get(fe.id)?.liftValue===0)||Math.hypot(k-(fe.x-1),ne-fe.z)<1.55||[-3,0].some(N=>Math.abs(k-fe.x-N)<1.1&&Math.abs(ne-fe.z-3)<.8))?!1:Zd(k,ne,I,N=>h.get(N)?.value>.85&&!!c.get(N)?.ready,e.districts)},floor(k,ne){let I=Rs(k,ne);return I&&j.position.y>5&&ou(I,k,ne,c.get(I.id)?.liftValue===4)?ht.gallery:Math.max(wr(k,ne),Yd(k,ne,e.districts))},visit(k){if(Re(),P=null,k==="drone"){j.position.set(di.x,2.7,di.z+3),j.lookAt(di.x,2,di.z);return}let ne=Jn.find(I=>I.id===k);if(ne){let I=O.has(k)?0:5,fe=ne.platformX+(ne.angle===0?I:0),N=ne.platformZ+(ne.angle===0?0:I);j.position.set(fe,2.4+wr(fe,N),N),j.lookAt(ne.platformX,2,ne.platformZ===N?ne.platformZ+1:ne.platformZ)}},destination(k){let ne=vn.find(I=>I.id===k);if(ne){Re(),P=null;let I=ne.doorZ+ne.front*4;j.position.set(ne.x,2.4+wr(ne.x,I),I),j.lookAt(ne.x,2.4+wr(ne.x,I),ne.z)}},isRiding:()=>!!P,interaction:()=>L,stats:()=>({loaded:[...d],bytes:v,freightEvents:b,residents:a.filter(k=>k.node.visible).length,trams:l.filter(k=>k.node.visible).length,rooms:[...c].filter(([,k])=>k.ready).map(([k])=>k),inside:U,ride:P?.kind||null,station:P?.station||null,discovered:[...O],loading:V,interactions:L?{kind:L.kind,id:L.id}:null}),dispose(){y||(y=!0,S.abort(),R=[],p.clear(),t.removeFromParent(),o.forEach(k=>{k.stopAllAction(),k.uncacheRoot(k.getRoot())}),t.traverse(k=>{k.isInstancedMesh&&k.dispose()}),i.forEach(k=>k.dispose()),r.forEach(k=>k.dispose()),M.forEach(k=>k.dispose()),n.clear())}}}function Jd(s,{sun:e,rim:t,hemisphere:n,atmosphere:i,ground:r}){let o="local",a="clear",l="high",c=!1,h=-100,u=!1,f={value:0},d=new ft,g=[],M=317,m=()=>(M=Math.imul(M,1664525)+1013904223>>>0)/4294967296;for(let y=0;y<1200;y++)g.push((m()-.5)*170,m()*60,(m()-.5)*174-7);d.setAttribute("position",new ot(g,3));let p=new mt({transparent:!0,depthWrite:!1,uniforms:{time:f},vertexShader:"uniform float time;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.0);gl_PointSize=3.0;}",fragmentShader:"void main(){float a=(1.0-abs(gl_PointCoord.x-.5)*2.0)*.35;gl_FragColor=vec4(.6,.78,.86,a);}"}),S=new Fn(d,p);S.frustumCulled=!1,s.add(S);let R=new Me(8559007),b=new Me(528926);function w(){let y=au(o),C=y.amount;e.intensity=.8+C*2.6,e.color.set(y.evening>.1?16765345:13164287),e.position.set(Math.cos(y.hour/24*Math.PI*2)*120,40+C*115,85),t.intensity=.7+y.evening*1.6,n.intensity=.5+C*.65,s.fog.color.copy(b).lerp(R,C),s.fog.density=a==="fog"?.018:a==="rain"?.008:.0035,i.setLighting?.(C,y.evening,e.position),r.material.roughness=a==="rain"?.14:.44,S.visible=a==="rain"&&!c,d.setDrawRange(0,l==="low"?200:1200)}return w(),{set(y={}){["local","day","evening","night"].includes(y.time)&&(o=y.time),["clear","rain","fog"].includes(y.weather)&&(a=y.weather),w()},setTier(y){l=y,w()},setIndoor(y){c!==y&&(c=y,w())},update(y,C,_){if(!u)return S.visible=a==="rain"&&!c&&_,_&&(f.value+=Math.min(y,.1)),C-h>30?(h=C,w(),!0):!1},stats:()=>({time:o,weather:a,indoor:c,daylight:au(o).amount}),dispose(){u=!0,S.removeFromParent(),d.dispose(),p.dispose()}}}function jd(s,e){let t=!1,n=!1,i=0,r=null,o=null,a=[],l=null,c=null,h=null,u=250,f=0,d=0,g=0,M="idle",m=null,p=()=>.03+.36/(1+(u/48)**2);function S(){let _=s.currentTime;l?.gain.setTargetAtTime(p(),_,.18),c?.frequency.setTargetAtTime(1800+4400/(1+u/95),_,.18),h?.pan.setTargetAtTime(f,_,.18)}function R(){if(o){o.onended=null;try{o.stop()}catch{}o=null}a.forEach(_=>_.disconnect()),a=[],l=c=h=null}function b(){d++,clearTimeout(i),i=0,r?.abort(),r=null,R(),M="idle"}function w(_=2e4+Math.random()*2e4){clearTimeout(i),t&&!n&&(i=setTimeout(()=>{i=0,C()},_))}function y(){if(m)return m;m=s.createBuffer(2,Math.ceil(s.sampleRate*.85),s.sampleRate);let _=42;for(let v=0;v<2;v++){let E=m.getChannelData(v);for(let P=0;P<E.length;P++)_=Math.imul(_,1664525)+1013904223>>>0,E[P]=(_/4294967296*2-1)*Math.pow(1-P/E.length,3)}return m}async function C(){if(!t||n||r||o||s.state!=="running"){w();return}let _=d,v=new AbortController;r=v,M="loading";let E=setTimeout(()=>v.abort(),32e3);try{let P=await fetch("/api/desktop/system-world/voice",{method:"POST",credentials:"same-origin",cache:"no-store",signal:v.signal});if(P.status===204){M="idle";return}if(!P.ok||!P.headers.get("Content-Type")?.startsWith("audio/"))throw Error("Voice unavailable");if(Number(P.headers.get("Content-Length"))>8*1024*1024)throw Error("Voice too large");let L=await P.arrayBuffer();if(L.byteLength>8*1024*1024)throw Error("Voice too large");if(_!==d||!t||n)return;let H=await s.decodeAudioData(L);if(_!==d||!t||n)return;if(!Number.isFinite(H.duration)||H.duration<=0||H.duration>25||H.numberOfChannels>2)throw Error("Invalid voice");let O=0;for(let Q=0;Q<H.numberOfChannels;Q++){let ge=H.getChannelData(Q);for(let X of ge)O=Math.max(O,Math.abs(X))}if(!Number.isFinite(O)||O<1e-4)throw Error("Silent voice");let U=Q=>(a.push(Q),Q);o=U(s.createBufferSource()),o.buffer=H;let V=U(s.createGain()),F=U(s.createBiquadFilter());V.gain.value=1/O,F.type="highpass",F.frequency.value=170,c=U(s.createBiquadFilter()),c.type="lowpass",c.Q.value=.55;let Y=U(s.createGain()),J=U(s.createGain()),j=U(s.createGain()),me=U(s.createGain());Y.gain.value=.74,J.gain.value=.17,j.gain.value=.12;let _e=U(s.createDelay(.2)),Ve=U(s.createConvolver());_e.delayTime.value=.085,Ve.buffer=y();let it=U(s.createWaveShaper()),Fe=new Float32Array(1024);for(let Q=0;Q<Fe.length;Q++)Fe[Q]=Math.max(-.75,Math.min(.75,Q*2/(Fe.length-1)-1));it.curve=Fe,l=U(s.createGain()),l.gain.value=p(),h=U(s.createStereoPanner()),h.pan.value=f,o.connect(V).connect(F).connect(c),c.connect(Y).connect(me),c.connect(_e).connect(J).connect(me),c.connect(Ve).connect(j).connect(me),me.connect(it).connect(l).connect(h).connect(e),S(),M="speaking",g++,o.onended=()=>{_!==d||!t||n||(M="tail",i=setTimeout(()=>{R(),M="idle",w()},900))},o.start()}catch{_===d&&!n&&(M=v.signal.aborted?"idle":"unavailable")}finally{clearTimeout(E),r===v&&(r=null),_===d&&!n&&!o&&w(M==="unavailable"?6e4:void 0)}}return{setActive(_){t===_||n||(t=_,t?w(3e3+Math.random()*2e3):b())},setListener(_,v,E,P,L){if(!Number.isFinite(_)||!Number.isFinite(v)||!Number.isFinite(E)||!Number.isFinite(P)||!Number.isFinite(L))return;let H=-_,O=-12-E,U=Math.max(0,Math.hypot(H,O)-13);u=Math.hypot(U,Math.max(0,v-87,-v));let V=Math.hypot(H,O),F=Math.hypot(P,L);f=V&&F?Math.max(-.8,Math.min(.8,(O*P-H*L)/V/F))*.8:0,S()},stats(){return{state:M,phrases:g,distance:u,gain:p(),pan:f,pending:!!r,scheduled:!!i}},dispose(){n||(n=!0,t=!1,b(),m=null)}}}function $d(s,e,t=e){let n=new Set,i={x:0,z:0,fx:0,fz:-1},r=!1,o=!1,a="clear",l=s.createBuffer(1,s.sampleRate*2,s.sampleRate),c=31,h=l.getChannelData(0);for(let p=0;p<h.length;p++)c=Math.imul(c,1664525)+1013904223|0,h[p]=(c>>>0)/2147483648-1;let u=s.createBufferSource(),f=s.createBiquadFilter(),d=s.createGain();u.buffer=l,u.loop=!0,f.type="lowpass",f.frequency.value=450,d.gain.value=0,u.connect(f).connect(d).connect(t),u.start();let g=[{x:-57,z:-57,f:94,g:.055},{x:43,z:62,f:143,g:.035},{x:0,z:14,f:220,g:.016},{x:0,z:79,f:340,g:.06}].map((p,S)=>{let R=S===3?s.createBufferSource():s.createOscillator(),b=s.createGain(),w=s.createStereoPanner(),y=s.createBiquadFilter();return S===3?(R.buffer=l,R.loop=!0):(R.type="sine",R.frequency.value=p.f),y.type="lowpass",y.frequency.value=p.f*2,b.gain.value=0,R.connect(y).connect(b).connect(w).connect(t),R.start(),{...p,source:R,level:b,pan:w,low:y}});function M(){d.gain.setTargetAtTime(r?(a==="rain"?.07:.015)*(o?.12:1):0,s.currentTime,.1),f.frequency.setTargetAtTime(o?280:a==="rain"?2100:600,s.currentTime,.2)}function m(){for(let p of[...n]){try{p.source.stop()}catch{}p.release()}}return{setActive(p){if(r=p,M(),!p){m();for(let S of g)S.level.gain.setTargetAtTime(0,s.currentTime,.05)}},environment(p,S){o=p,S&&(a=S),M()},listener(p,S,R,b,w){Object.assign(i,{x:p,z:R,fx:b,fz:w});for(let y of g){let C=Math.hypot(p-y.x,R-y.z);y.level.gain.setTargetAtTime(r?y.g/(1+C*C*.025)*(o?.25:1):0,s.currentTime,.1),y.pan.pan.value=Math.max(-1,Math.min(1,((y.x-p)*-w+(y.z-R)*b)/Math.max(1,C)))}},play(p,S=0,R=0,b=0){if(!r||n.size>=16)return;let w=Math.hypot(S-i.x,b-i.z);if(w>90)return;let y={step:[.12,240,.16],step_inside:[.1,550,.12],door:[.7,850,.06],lift:[1.5,120,.05],tram:[.8,330,.08],discover:[.45,880,.08]}[p];if(!y)return;let[C,_,v]=y,E=p.startsWith("step")||p==="door"?s.createBufferSource():s.createOscillator();"buffer"in E?E.buffer=l:(E.type="sine",E.frequency.value=_);let P=s.createBiquadFilter(),L=s.createGain(),H=s.createStereoPanner();P.type="lowpass",P.frequency.value=o?_*.7:_,L.gain.setValueAtTime(0,s.currentTime),L.gain.linearRampToValueAtTime(v/(1+w*.08),s.currentTime+.015),L.gain.exponentialRampToValueAtTime(1e-4,s.currentTime+C),H.pan.value=Math.max(-1,Math.min(1,((S-i.x)*-i.fz+(b-i.z)*i.fx)/Math.max(w,1))),E.connect(P).connect(L).connect(H).connect(e);let O={source:E,release(){if(n.delete(O))for(let U of[E,P,L,H])U.disconnect()}};n.add(O),E.onended=O.release,E.start(),E.stop(s.currentTime+C+.02)},stats:()=>({voices:n.size,weather:a,inside:o}),dispose(){r=!1,m(),u.stop();for(let p of g){p.source.stop();for(let S of[p.source,p.level,p.pan,p.low])S.disconnect()}for(let p of[u,f,d])p.disconnect()}}}function qv(s=()=>{}){let e=null,t=null,n=null,i=!1,r=!1,o=!1,a=0,l=null,c=[],h=[],u=new Float32Array(256),f=.18,d=null,g=null,M=null,m=null,p=!1,S="clear",R={ambience:1,effects:1,voice:1};try{o=localStorage.getItem("aurago.desktop.sysworld.sound")==="true"}catch{}function b(){if(e||i)return;e=new(window.AudioContext||window.webkitAudioContext);let y=F=>(c.push(F),F),C=F=>(h.push(F),y(F));t=y(e.createGain()),t.gain.value=0,n=y(e.createAnalyser()),n.fftSize=512,t.connect(n),n.connect(e.destination),g=y(e.createGain()),g.gain.value=R.ambience,g.connect(t),M=y(e.createGain()),M.gain.value=R.effects,M.connect(t),m=y(e.createGain()),m.gain.value=R.voice,m.connect(t),l=jd(e,m),d=$d(e,M,g),d.environment(p,S);for(let[F,Y]of[[55,.1],[82.4069,.032],[110,.017],[164.8138,.009]]){let J=C(e.createOscillator()),j=y(e.createGain());J.frequency.value=F,j.gain.value=Y,J.connect(j).connect(g),J.start()}let _=e.createBuffer(1,e.sampleRate*4,e.sampleRate),v=_.getChannelData(0),E=0;for(let F=0;F<v.length;F++)E=(E+(Math.random()*2-1)*.018)/1.018,v[F]=E;let P=v[v.length-1]-v[0];for(let F=0;F<v.length;F++)v[F]-=P*F/(v.length-1);let L=C(e.createBufferSource()),H=y(e.createBiquadFilter()),O=y(e.createGain());L.buffer=_,L.loop=!0,H.type="lowpass",H.frequency.value=680,H.Q.value=.25,O.gain.value=.32,L.connect(H).connect(O).connect(g),L.start();let U=C(e.createOscillator()),V=y(e.createGain());U.frequency.value=.075,V.gain.value=160,U.connect(V).connect(H.frequency),U.start()}function w(y=!1){if(clearTimeout(a),!i){if(y&&o)try{b()}catch{o=!1,s(!1);return}e&&(l?.setActive(o&&r&&f>0&&R.voice>0),d?.setActive(o&&r&&f>0),o&&r?(e.resume().catch(()=>{}),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(f,e.currentTime,.25)):(t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(0,e.currentTime,.035),a=setTimeout(()=>{!i&&(!o||!r)&&e.suspend().catch(()=>{})},180)))}}return{enabled:()=>o,toggle(){o=!o;try{localStorage.setItem("aurago.desktop.sysworld.sound",String(o))}catch{}w(!0),s(o)},unlock(){w(!0)},setActive(y){r!==y&&(r=y,w())},setVolume(y){Number.isFinite(y)&&(f=Math.max(0,Math.min(.35,y)),w())},setListener(y,C,_,v,E){l?.setListener(y,C,_,v,E),d?.listener(y,C,_,v,E)},setChannel(y,C){if(!(y in R)||!Number.isFinite(C))return;R[y]=Math.max(0,Math.min(1,C));let _={ambience:g,effects:M,voice:m}[y];_&&_.gain.setTargetAtTime(R[y],e.currentTime,.05),w()},effect(y,C,_,v){d?.play(y,C,_,v)},setEnvironment(y,C){p=!!y,["clear","rain","fog"].includes(C)&&(S=C),d?.environment(p,S)},stats(){let y=0;if(n&&e.state==="running"){n.getFloatTimeDomainData(u);for(let C of u)y+=C*C}return{voice:l?.stats(),effects:d?.stats(),channels:{...R},enabled:o,active:r,state:e?.state||"uninitialized",volume:f,rms:Math.sqrt(y/u.length)}},dispose(){i||(i=!0,d?.dispose(),l?.dispose(),clearTimeout(a),h.forEach(y=>{try{y.stop()}catch{}}),c.forEach(y=>y.disconnect()),e&&e.close().catch(()=>{}))}}}var Rn=[{id:"agent",asset:"agent-spire",x:0,z:-12,height:87,radius:13},{id:"infra",asset:"compute-foundry",x:-43,z:-53,height:24,radius:18},{id:"integrations",asset:"integration-gate",x:43,z:-10,height:34,radius:14},{id:"missions",asset:"mission-terminal",x:43,z:36,height:23,radius:15},{id:"memory",asset:"memory-archive",x:-43,z:-10,height:37,radius:14},{id:"graph",asset:"knowledge-atrium",x:-43,z:36,height:28,radius:17},{id:"operations",asset:"operations-beacon",x:0,z:36,height:22,radius:6}],rc={low:{lod:2,dpr:1,shadow:0,bloom:!1},medium:{lod:1,dpr:1.25,shadow:1024,bloom:!1},high:{lod:0,dpr:1.5,shadow:2048,bloom:!0},ultra:{lod:0,dpr:2,shadow:4096,bloom:!0}},Qd=new D(122,106,183),lu=class extends An{constructor(e,t,n){super(),this.scene=e,this.camera=t,this.needsSwap=!1,this.target=new Nt(1,1,{type:Xt,samples:n}),this.quad=new fi(new mt({uniforms:Kn.clone(Pi.uniforms),vertexShader:Pi.vertexShader,depthTest:!1,depthWrite:!1,fragmentShader:"uniform sampler2D tDiffuse;varying vec2 vUv;void main(){vec4 c=texture2D(tDiffuse,vUv);if(any(isnan(c))||any(isinf(c)))c=vec4(0.0,0.0,0.0,1.0);gl_FragColor=c;}"}))}render(e,t,n){let i=e.autoClear;e.autoClear=!1,e.setRenderTarget(this.target),e.clear(),e.render(this.scene,this.camera),this.quad.material.uniforms.tDiffuse.value=this.target.texture,e.setRenderTarget(this.renderToScreen?null:n),this.quad.render(e),e.autoClear=i}setSize(e,t){this.target.setSize(e,t)}dispose(){this.target.dispose(),this.quad.material.dispose(),this.quad.dispose()}},ep=new D(0,22,-12),Bo=[];{let s=(n,i,r,o=0,a=0,l=[1,1,1])=>Bo.push({asset:n,x:i,z:r,y:o,angle:a,scale:l}),{xs:e,zs:t}=xn;for(let n of t){for(let r of e)s("street-crossing",r,n);let i=[xn.minX,...e,xn.maxX];for(let r=0;r<i.length-1;r++){let o=i[r]+(r?6:0),a=i[r+1]-(r<i.length-2?6:0);s("street-tile",(o+a)/2,n,0,0,[(a-o)/16,1,1])}}for(let n of e){let i=[xn.minZ,...t,xn.maxZ];for(let r=0;r<i.length-1;r++){let o=i[r]+(r?6:0),a=i[r+1]-(r<i.length-2?6:0);s("street-tile",n,(o+a)/2,0,Math.PI/2,[(a-o)/16,1,1])}}for(let n of t)for(let i of[-55,-31,31,55])s("street-lamp",i,n-4.5,.45),s("planter",i+4,n-4.5,.45);for(let{x:n,z:i,scale:r}of sc)s("data-tower-a",n,i,0,0,[1,r,1]);s("skybridge",39.75,-55,15),s("server-rack",-55,-36.7,.5),s("server-rack",-49,-36.7,.5);for(let n=0;n<48;n++){let i=(n%12-5.5)*22,r=-143-Math.floor(n/12)*29,o=.7+n*17%13/12;s(n%2?"data-tower-a":"data-tower-b",i,r,-3,0,[.8,o,.8])}}var Yv=[...Pd(Bo),...vn.flatMap(s=>[-3,3].flatMap(e=>[-3,3].map(t=>({x:s.x+e,z:s.z+t,r:4.25,asset:"interior"}))))];async function VS(s,e){let t=!1,n=!0,i="orbit",r=e.quality||"auto",o=r==="auto"?"high":r,a=0,l=null,c=0,h=0,u=0,f=0,d=0,g=0,M=0,m=null,p=e.reducedMotion,S=1,R=1,b=[],w=!1,y=null,C=null,_=new AbortController,v=new Set,E=new Map,P=new Map,L=new Set,H=new Set,O=new Set,U=[],V=new Xl({antialias:!0,alpha:!1});V.toneMapping=bs,V.toneMappingExposure=.98,V.shadowMap.type=Ja,V.shadowMap.autoUpdate=!1,V.info.autoReset=!1;let F=V.domElement;F.className="sysworld-gl",F.tabIndex=0,F.setAttribute("aria-label",e.label),s.append(F);let Y=new gs;Y.background=new Me(528926),Y.fog=new Gr(528926,.004);let J=new Ht(43,1,.3,1800);J.position.copy(Qd);let j=new jl(J,F);j.target.copy(ep),j.enableDamping=!0,j.dampingFactor=.085,j.minDistance=12,j.maxDistance=410,j.maxPolarAngle=Math.PI*.485,j.update();let me=new vr(V),_e=new $l,Ve=me.fromScene(_e,.05);Y.environment=Ve.texture,Y.environmentIntensity=.48,_e.dispose(),me.dispose();let it=new ro(12639487,1056813,.8);Y.add(it);let Fe=new Yi(13164287,3.2);Fe.position.set(-70,145,85),Fe.castShadow=!0,Object.assign(Fe.shadow.camera,{left:-120,right:120,top:120,bottom:-120,near:1,far:360}),Fe.shadow.normalBias=.09,Fe.shadow.bias=-15e-5,Y.add(Fe);let Q=new Yi(16758130,2.4);Q.position.set(90,80,-120),Y.add(Q);let ge=new Xn(6479871,0,75,2);ge.position.set(0,34,-12),Y.add(ge);let X=new Nt(1,1,{type:Xt}),he=new ec(V,X);he.addPass(new lu(Y,J,Math.min(4,V.capabilities.maxSamples)));let $=new Tr(new ye(1,1),.24,.5,1.25);he.addPass($),he.addPass(new tc);let ie=new ut,oe=new ut,ue=new ut;Y.add(ie),ie.add(oe,ue);function se(q,xe){return L.add(q),H.add(xe),new Je(q,xe)}let ce=se(new si(170,3,174),new dn({color:1451051,metalness:.55,roughness:.38}));ce.position.set(0,-1.5,-7),ce.receiveShadow=!0,ie.add(ce);let ve=kd(Y,{sunDirection:Fe.position,lamps:Bo.filter(q=>q.asset==="street-lamp")});he.addPass(ve.post);let Re=Rn.find(q=>q.id==="memory"),We=Fd(Y,Re,{roof:21,label:e.memoryLabel}),lt=Vd(Y),ct=Jd(Y,{sun:Fe,rim:Q,hemisphere:it,atmosphere:ve,ground:ce}),k=Kd(Y,{camera:J,districts:Rn,tier:o,reduced:()=>p,active:()=>n&&!w&&i!=="map"&&!p,assetURL:q=>e.resourceURL("/3d/system-world/v2/"+q),onInteraction:e.onInteraction,onDiscover:e.onDiscover,onSound:e.onSound,onTerminal:e.onTerminal,onEnvironment:q=>{ct.setIndoor(q),e.onEnvironment?.(q)},onError:e.onError,onReady:()=>{V.shadowMap.needsUpdate=!0}}),ne=se(new Qr(1,1.035,64),new Tt({color:9365742,transparent:!0,opacity:.85,depthWrite:!1,side:Bt}));ne.rotation.x=-Math.PI/2,ne.visible=!1,ie.add(ne);let I=new Tt({color:6742230,toneMapped:!1}),fe=new fn(new ri(.65,10,6),I,Rn.length);L.add(fe.geometry),H.add(I),ie.add(fe);let N=new yt,T=new D,x=new fo,B=new ye;Rn.forEach((q,xe)=>{N.position.set(q.x,q.height+2,q.z),N.updateMatrix(),fe.setMatrixAt(xe,N.matrix),fe.setColorAt(xe,new Me(8425630))}),e.signal?.addEventListener("abort",De,{once:!0});let G;try{if(e.signal?.aborted)throw Error("Disposed");let q=await fetch(e.assetURL("manifest.json"),{signal:_.signal});if(!q.ok)throw Error("City manifest unavailable");G=await q.json()}catch(q){throw De(),q}let ee=new Map(G.assets.map(q=>[q.id,q]));async function pe(q,xe){let rt=q+":"+xe;return E.has(rt)||E.set(rt,(async()=>{let je=ee.get(q)?.lods.find(Kt=>Kt.level===xe);if(!je||!/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(je.file))throw Error("Invalid city asset");let gt=new AbortController;v.add(gt);let an=setTimeout(()=>gt.abort(),15e3);try{let Kt=await fetch(e.assetURL(je.file),{signal:gt.signal});if(!Kt.ok)throw Error("City asset unavailable");let _t=await Kt.arrayBuffer();if(t)throw Error("Disposed");let Jt=await new ns().parseAsync(_t,"");if(t)throw Jt.scene.traverse(Ct=>{Ct.isMesh&&(Ct.geometry.dispose(),Ct.material.dispose())}),Error("Disposed");return d+=_t.byteLength,Jt.scene.traverse(Ct=>{if(!Ct.isMesh)return;L.add(Ct.geometry),Ct.castShadow=!0,Ct.receiveShadow=!0;let $t=(Array.isArray(Ct.material)?Ct.material:[Ct.material]).map(Cn=>{if(P.has(Cn.name)){let zo=Cn;Cn=P.get(Cn.name),zo.dispose()}else P.set(Cn.name,Cn),H.add(Cn);return Cn});Ct.material=Array.isArray(Ct.material)?$t:$t[0]}),Jt.scene}finally{clearTimeout(an),v.delete(gt)}})().catch(je=>{throw E.delete(rt),je})),E.get(rt)}function be(q){q.traverse(xe=>{xe.isInstancedMesh&&xe.dispose()}),q.clear()}async function re(){let q=++a,xe=rc[o],rt=xe.lod,je=[...new Set([...Bo.map(gt=>gt.asset),...Rn.map(gt=>gt.asset),"service-drone"])];try{let gt=new Map(await Promise.all(je.map(async _t=>[_t,await pe(_t,rt)]))),an=new Map(await Promise.all(["data-tower-a","data-tower-b"].map(async _t=>[_t,await pe(_t,2)])));if(t||q!==a)return;be(oe),be(ue),b=[];let Kt=new Map;for(let _t of Bo){let Jt=(_t.z<-100?an:gt).get(_t.asset);Jt.updateMatrixWorld(!0);let Ct=new Ke().compose(new D(_t.x,_t.y,_t.z),new It().setFromAxisAngle(new D(0,1,0),_t.angle),new D(..._t.scale));Jt.traverse(jn=>{if(!jn.isMesh)return;let $t=jn.uuid;Kt.has($t)||Kt.set($t,{node:jn,matrices:[]}),Kt.get($t).matrices.push(new Ke().multiplyMatrices(Ct,jn.matrixWorld))})}for(let{node:_t,matrices:Jt}of Kt.values()){let Ct=new fn(_t.geometry,_t.material,Jt.length);Jt.forEach((jn,$t)=>Ct.setMatrixAt($t,jn)),Ct.castShadow=!0,Ct.receiveShadow=!0,oe.add(Ct)}for(let _t of Rn){let Jt=gt.get(_t.asset).clone(!0);Jt.position.set(_t.x,0,_t.z),Jt.userData.district=_t.id,ue.add(Jt),b.push(Jt)}C?.attachLandmarks(ue),lt.setTemplate(gt.get("service-drone")),V.shadowMap.needsUpdate=!0,e.onReady?.()}catch(gt){!t&&q===a&&e.onError?.(gt)}}function ae(){if(t)return;S=Math.max(1,s.clientWidth),R=Math.max(1,s.clientHeight);let q=Math.min(devicePixelRatio||1,rc[o].dpr,Math.sqrt(3840*2160/(S*R)));V.setPixelRatio(q),V.setSize(S,R,!1),he.setPixelRatio(q),he.setSize(S,R),J.aspect=S/R,J.updateProjectionMatrix(),We.setPointScale(R*q),ve.setPointScale(R*q)}function Te(q){return r=q in rc||q==="auto"?q:"auto",o=r==="auto"?"high":r,Oe(),re()}function Oe(){let q=rc[o];V.shadowMap.enabled=q.shadow>0,q.shadow&&Fe.shadow.mapSize.x!==q.shadow&&(Fe.shadow.mapSize.set(q.shadow,q.shadow),Fe.shadow.map?.dispose(),Fe.shadow.map=null),V.shadowMap.needsUpdate=!0,$.enabled=q.bloom,ve.setTier(o),k.setTier(o),ct.setTier(o),lt.setTier(o),ae(),e.onQuality?.(r,o)}function Ae(q,xe){if(p){J.position.copy(q),j.target.copy(xe),j.update(),l=null;return}l={start:J.position.clone(),targetStart:j.target.clone(),end:q.clone(),target:xe.clone(),time:0}}function Ee(q){let xe=Rn.find(rt=>rt.id===q);xe&&(m=q,ne.position.set(xe.x,.55,xe.z),ne.scale.setScalar(xe.radius*1.3),ne.visible=!0,i==="street"?(J.position.set(xe.x,2.4,xe.z+xe.radius+7),J.lookAt(xe.x,xe.height*.4,xe.z)):Ae(new D(xe.x+xe.radius*2.7,xe.height*.7+18,xe.z+xe.radius*4),new D(xe.x,xe.height*.4,xe.z)))}function ze(q){k.endRide(),O.clear(),l=null,i=q,j.enabled=i==="orbit"||i==="tour",document.pointerLockElement===F&&document.exitPointerLock(),i==="street"?(J.position.set(18,2.4,57),J.lookAt(0,26,-12)):i!=="map"&&Ae(Qd.clone().multiplyScalar(J.aspect<1?1.3:1),ep),g=0,M=0,e.onMode?.(i)}let qe=()=>{i==="tour"&&(i="orbit",l=null,e.onMode?.(i))};j.addEventListener("start",qe);function Ye(q,xe,rt,je){q.addEventListener(xe,rt,je),U.push(()=>q.removeEventListener(xe,rt,je))}let z=null,Se=!1,le=new Un(0,0,0,"YXZ");function we(q,xe){le.setFromQuaternion(J.quaternion),le.y-=q*.0025,le.x=Vt.clamp(le.x-xe*.0025,-1.35,1.35),J.quaternion.setFromEuler(le)}Ye(F,"pointerdown",q=>{F.focus({preventScroll:!0}),qe(),z=[q.clientX,q.clientY],Se=!1,i==="street"&&F.setPointerCapture(q.pointerId)}),Ye(F,"pointermove",q=>{i==="street"&&(document.pointerLockElement===F||z)&&we(q.movementX,q.movementY),z&&Math.hypot(q.clientX-z[0],q.clientY-z[1])>5&&(Se=!0)}),Ye(F,"pointerup",q=>{if(z&&!Se&&i!=="street"){let xe=F.getBoundingClientRect();B.set((q.clientX-xe.left)/xe.width*2-1,1-(q.clientY-xe.top)/xe.height*2),x.setFromCamera(B,J);let rt=x.intersectObjects(b,!0)[0];if(rt){let je=rt.object;for(;je&&!je.userData.district;)je=je.parent;je&&e.onSelect?.(je.userData.district)}}z=null}),Ye(F,"pointercancel",()=>{z=null,O.clear()}),Ye(F,"keydown",q=>{if(q.key==="Escape"){ze("orbit"),q.preventDefault();return}if(i==="street"&&q.code==="KeyE"&&!q.repeat){k.interact(),q.preventDefault();return}i==="street"&&["KeyW","KeyA","KeyS","KeyD","ArrowUp","ArrowDown","ArrowLeft","ArrowRight","ShiftLeft"].includes(q.code)&&(O.add(q.code),q.preventDefault(),q.stopPropagation())}),Ye(F,"keyup",q=>O.delete(q.code)),Ye(F,"blur",()=>O.clear()),Ye(window,"blur",()=>{O.clear(),z=null}),Ye(F,"webglcontextlost",q=>{q.preventDefault(),w=!0,O.clear(),e.onContextLost?.()}),y=new ResizeObserver(ae),y.observe(s);function Pe(q){let xe=(O.has("ShiftLeft")?25:11)*q,rt=Number(O.has("KeyW")||O.has("ArrowUp"))-Number(O.has("KeyS")||O.has("ArrowDown")),je=Number(O.has("KeyD")||O.has("ArrowRight"))-Number(O.has("KeyA")||O.has("ArrowLeft"));if(!rt&&!je||k.walkRide(rt,q))return;let gt=xe/Math.hypot(rt,je);J.getWorldDirection(T),T.y=0,T.normalize();let an=(T.x*rt-T.z*je)*gt,Kt=(T.z*rt+T.x*je)*gt,_t=(Jt,Ct)=>k.move(Jt,Ct,J.position);_t(J.position.x+an,J.position.z)&&(J.position.x+=an),_t(J.position.x,J.position.z+Kt)&&(J.position.z+=Kt),k.isRiding()||(J.position.y=2.4+k.floor(J.position.x,J.position.z))}function de(q,xe){if(t||!n||w||i==="map")return;if(i==="street")Pe(Math.min(q,.05));else{if(i==="tour"&&(g-=q,g<=0)){let je=Rn[M++%Rn.length].id;Ee(je),e.onTourFocus?.(je),g=7}if(l){l.time+=q;let je=Math.min(1,l.time/1.1),gt=je*je*(3-2*je);J.position.lerpVectors(l.start,l.end,gt),j.target.lerpVectors(l.targetStart,l.target,gt),je===1&&(l=null)}j.update()}J.getWorldDirection(T),e.onListener?.(J.position.x,J.position.y,J.position.z,T.x,T.z);let rt=!!e.busy?.();if(k.update(q,!p,i),ct.update(q,xe,!p)&&(V.shadowMap.needsUpdate=!0),ge.intensity=p?0:rt?180+Math.sin(xe*2)*35:0,C?.update(q,!p),ve.setBusy(rt),ve.update(q,xe,J,!p),We.update(q,J,!p),lt.update(q,xe,!p),V.info.reset(),he.render(),c++,r==="auto"&&q>0&&q<.2&&xe-f>12&&(h+=q,u++,u>=180)){let je=h/u,gt=["low","medium","high"],an=gt.indexOf(o),Kt=je>.028&&an>0?gt[an-1]:je<.017&&an<2?gt[an+1]:o;u=0,h=0,Kt!==o&&(o=Kt,f=xe,Oe(),re())}}function ke(q,xe){C?.setData(q,xe),Rn.forEach((rt,je)=>{let gt=q.find(an=>an.id===rt.id);fe.setColorAt(je,new Me(!gt||gt.stale?8096667:gt.state==="error"?16742504:gt.state==="running"?7730385:8175587))}),fe.instanceColor.needsUpdate=!0}function De(){t||(t=!0,a++,_.abort(),v.forEach(q=>q.abort()),O.clear(),document.pointerLockElement===F&&document.exitPointerLock(),e.signal?.removeEventListener("abort",De),U.forEach(q=>q()),y?.disconnect(),j.dispose(),C?.dispose(),k.dispose(),ct.dispose(),We.dispose(),lt.dispose(),ve.dispose(),be(oe),be(ue),fe.dispose(),L.forEach(q=>q.dispose()),H.forEach(q=>q.dispose()),he.passes.forEach(q=>q.dispose?.()),he.dispose(),Ve.dispose(),Fe.shadow.dispose(),V.dispose(),V.forceContextLoss(),F.remove(),E.clear(),P.clear())}C=Dd(Y,Rn,{robotURL:e.resourceURL("/3d/system-world/white-robot.glb"),signal:e.signal,onError:e.onRobotError,active:()=>n&&!w&&i!=="map",obstacles:Yv}),We.setReducedMotion(!!p);try{Oe(),await re()}catch(q){throw De(),q}return{districts:Rn,canvas:F,update:de,focus(q){qe(),Ee(q)},setMode:ze,setQuality:Te,setData:ke,dispose:De,interact(){k.interact()},visit(q){ze("street"),k.visit(q)},enter(q){ze("street"),k.destination(q)},setEnvironment(q){ct.set(q),V.shadowMap.needsUpdate=!0},setVisible(q){n=q,q||(O.clear(),z=null,document.pointerLockElement===F&&document.exitPointerLock())},setReducedMotion(q){p=q,We.setReducedMotion(!!q),q&&k.endRide(),q&&i==="tour"&&ze("orbit")},setHologram(q,xe){We.setTexts(q,xe),k.setMemory(q)},setWorld(q,xe){k.setWorld(q,xe)},moveKey(q,xe){xe?O.add(q):O.delete(q)},lockPointer(){if(i==="street")return F.requestPointerLock()},project(q){let xe=Rn.find(rt=>rt.id===q);return xe?(T.set(xe.x,xe.height+4,xe.z).project(J),{x:(T.x+1)*S/2,y:(1-T.y)*R/2,visible:T.z<1&&T.z>-1}):null},stats(){return{experience:k.stats(),weather:ct.stats(),life:C?.stats(),hologram:We.stats(),atmosphere:ve.stats(),drones:lt.stats(),frames:c,tier:o,mode:i,focusedDistrict:m,loadedBytes:d+k.stats().bytes,cachedModels:E.size,calls:V.info.render.calls,triangles:V.info.render.triangles,geometries:V.info.memory.geometries,position:J.position.toArray(),renderer:V.getContext().getParameter(V.getContext().getExtension("WEBGL_debug_renderer_info")?.UNMASKED_RENDERER_WEBGL||V.getContext().RENDERER),disposed:t}}}}export{VS as createCity,qv as createCityAmbience,Rn as districts,Yv as obstacles,Bo as placements};
/*! Bundled license information:

three/build/three.core.js:
three/build/three.module.js:
  (**
   * @license
   * Copyright 2010-2026 Three.js Authors
   * SPDX-License-Identifier: MIT
   *)
*/
