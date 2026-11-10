var Ts={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2},As={ROTATE:0,PAN:1,DOLLY_PAN:2,DOLLY_ROTATE:3},Cf=0,tu=1,Pf=2;var tr=1,If=2,Zr=3,oi=0,yn=1,Kt=2,li=0,$r=1,rn=2,nu=3,iu=4,Df=5;var nr=100,Lf=101,Ff=102,Nf=103,Uf=104,zf=200,Of=201,kf=202,Bf=203,su=204,ru=205,Hf=206,Gf=207,Vf=208,Wf=209,qf=210,Xf=211,jf=212,Kf=213,Yf=214,ml=0,gl=1,xl=2,Pr=3,bl=4,vl=5,yl=6,_l=7,jl=0,Jf=1,Zf=2,Si=0,ao=1,oo=2,lo=3,ir=4,co=5,ho=6,uo=7,Wh="attached",$f="detached",au=300,Rs=301,sr=302,Kl=303,Yl=304,fo=306,Di=1e3,ri=1001,Ir=1002,cn=1003,Jl=1004;var rr=1005;var on=1006,Qr=1007;var Ei=1008;var Xn=1009,ou=1010,lu=1011,ea=1012,Zl=1013,Ti=1014,ei=1015,xn=1016,$l=1017,Ql=1018,ta=1020,cu=35902,hu=35899,uu=1021,du=1022,ti=1023,Li=1026,Cs=1027,ec=1028,tc=1029,Ps=1030,nc=1031;var ic=1033,po=33776,mo=33777,go=33778,xo=33779,sc=35840,rc=35841,ac=35842,oc=35843,lc=36196,cc=37492,hc=37496,uc=37488,dc=37489,bo=37490,fc=37491,pc=37808,mc=37809,gc=37810,xc=37811,bc=37812,vc=37813,yc=37814,_c=37815,Mc=37816,wc=37817,Sc=37818,Ec=37819,Tc=37820,Ac=37821,Rc=36492,Cc=36494,Pc=36495,Ic=36283,Dc=36284,vo=36285,Lc=36286,Fc=2200,Qf=2201,ep=2202,Xs=2300,js=2301,dl=2302,qh=2303,Vs=2400,Ws=2401,Da=2402,Nc=2500,tp=2501,fu=0,yo=1,na=2,np=3200;var _o=0,ip=1,ci="",an="srgb",Hn="srgb-linear",La="linear",Xt="srgb";var fl=7680;var sp=519,rp=512,ap=513,op=514,Uc=515,lp=516,cp=517,zc=518,hp=519,pu=35044;var mu="300 es",bi=2e3,Dr=2001;function Fg(s){for(let e=s.length-1;e>=0;--e)if(s[e]>=65535)return!0;return!1}function Ng(s){return ArrayBuffer.isView(s)&&!(s instanceof DataView)}function Lr(s){return document.createElementNS("http://www.w3.org/1999/xhtml",s)}function up(){let s=Lr("canvas");return s.style.display="block",s}var kd={},Fr=null;function Fa(...s){let e="THREE."+s.shift();Fr?Fr("log",e,...s):console.log(e,...s)}function dp(s){let e=s[0];if(typeof e=="string"&&e.startsWith("TSL:")){let t=s[1];t&&t.isStackTrace?s[0]+=" "+t.getLocation():s[1]='Stack trace not available. Enable "THREE.Node.captureStackTrace" to capture stack traces.'}return s}function dt(...s){s=dp(s);let e="THREE."+s.shift();if(Fr)Fr("warn",e,...s);else{let t=s[0];t&&t.isStackTrace?console.warn(t.getError(e)):console.warn(e,...s)}}function _t(...s){s=dp(s);let e="THREE."+s.shift();if(Fr)Fr("error",e,...s);else{let t=s[0];t&&t.isStackTrace?console.error(t.getError(e)):console.error(e,...s)}}function qs(...s){let e=s.join(" ");e in kd||(kd[e]=!0,dt(...s))}function fp(s,e,t){return new Promise(function(n,i){function r(){switch(s.clientWaitSync(e,s.SYNC_FLUSH_COMMANDS_BIT,0)){case s.WAIT_FAILED:i();break;case s.TIMEOUT_EXPIRED:setTimeout(r,t);break;default:n()}}setTimeout(r,t)})}var pp={[ml]:gl,[xl]:yl,[bl]:_l,[Pr]:vl,[gl]:ml,[yl]:xl,[_l]:bl,[vl]:Pr},ai=class{addEventListener(e,t){this._listeners===void 0&&(this._listeners={});let n=this._listeners;n[e]===void 0&&(n[e]=[]),n[e].indexOf(t)===-1&&n[e].push(t)}hasEventListener(e,t){let n=this._listeners;return n===void 0?!1:n[e]!==void 0&&n[e].indexOf(t)!==-1}removeEventListener(e,t){let n=this._listeners;if(n===void 0)return;let i=n[e];if(i!==void 0){let r=i.indexOf(t);r!==-1&&i.splice(r,1)}}dispatchEvent(e){let t=this._listeners;if(t===void 0)return;let n=t[e.type];if(n!==void 0){e.target=this;let i=n.slice(0);for(let r=0,a=i.length;r<a;r++)i[r].call(this,e);e.target=null}}},Fn=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],Bd=1234567,Ra=Math.PI/180,Ks=180/Math.PI;function vi(){let s=Math.random()*4294967295|0,e=Math.random()*4294967295|0,t=Math.random()*4294967295|0,n=Math.random()*4294967295|0;return(Fn[s&255]+Fn[s>>8&255]+Fn[s>>16&255]+Fn[s>>24&255]+"-"+Fn[e&255]+Fn[e>>8&255]+"-"+Fn[e>>16&15|64]+Fn[e>>24&255]+"-"+Fn[t&63|128]+Fn[t>>8&255]+"-"+Fn[t>>16&255]+Fn[t>>24&255]+Fn[n&255]+Fn[n>>8&255]+Fn[n>>16&255]+Fn[n>>24&255]).toLowerCase()}function Lt(s,e,t){return Math.max(e,Math.min(t,s))}function gu(s,e){return(s%e+e)%e}function Ug(s,e,t,n,i){return n+(s-e)*(i-n)/(t-e)}function zg(s,e,t){return s!==e?(t-s)/(e-s):0}function Ca(s,e,t){return(1-t)*s+t*e}function Og(s,e,t,n){return Ca(s,e,1-Math.exp(-t*n))}function kg(s,e=1){return e-Math.abs(gu(s,e*2)-e)}function Bg(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*(3-2*s))}function Hg(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*s*(s*(s*6-15)+10))}function Gg(s,e){return s+Math.floor(Math.random()*(e-s+1))}function Vg(s,e){return s+Math.random()*(e-s)}function Wg(s){return s*(.5-Math.random())}function qg(s){s!==void 0&&(Bd=s);let e=Bd+=1831565813;return e=Math.imul(e^e>>>15,e|1),e^=e+Math.imul(e^e>>>7,e|61),((e^e>>>14)>>>0)/4294967296}function Xg(s){return s*Ra}function jg(s){return s*Ks}function Kg(s){return s>0&&Number.isInteger(s)&&2**Math.round(Math.log2(s))===s}function Yg(s){return Math.pow(2,Math.ceil(Math.log(s)/Math.LN2))}function Jg(s){return Math.pow(2,Math.floor(Math.log(s)/Math.LN2))}function Zg(s,e,t,n,i){let r=Math.cos,a=Math.sin,o=r(t/2),l=a(t/2),c=r((e+n)/2),h=a((e+n)/2),d=r((e-n)/2),u=a((e-n)/2),p=r((n-e)/2),x=a((n-e)/2);switch(i){case"XYX":s.set(o*h,l*d,l*u,o*c);break;case"YZY":s.set(l*u,o*h,l*d,o*c);break;case"ZXZ":s.set(l*d,l*u,o*h,o*c);break;case"XZX":s.set(o*h,l*x,l*p,o*c);break;case"YXY":s.set(l*p,o*h,l*x,o*c);break;case"ZYZ":s.set(l*x,l*p,o*h,o*c);break;default:dt("MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+i)}}function xi(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return s/4294967295;case Uint16Array:return s/65535;case Uint8Array:case Uint8ClampedArray:return s/255;case Int32Array:return Math.max(s/2147483647,-1);case Int16Array:return Math.max(s/32767,-1);case Int8Array:return Math.max(s/127,-1);default:throw new Error("THREE.MathUtils: Invalid component type.")}}function $t(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return Math.round(s*4294967295);case Uint16Array:return Math.round(s*65535);case Uint8Array:case Uint8ClampedArray:return Math.round(s*255);case Int32Array:return Math.round(s*2147483647);case Int16Array:return Math.round(s*32767);case Int8Array:return Math.round(s*127);default:throw new Error("THREE.MathUtils: Invalid component type.")}}var It={DEG2RAD:Ra,RAD2DEG:Ks,generateUUID:vi,clamp:Lt,euclideanModulo:gu,mapLinear:Ug,inverseLerp:zg,lerp:Ca,damp:Og,pingpong:kg,smoothstep:Bg,smootherstep:Hg,randInt:Gg,randFloat:Vg,randFloatSpread:Wg,seededRandom:qg,degToRad:Xg,radToDeg:jg,isPowerOfTwo:Kg,ceilPowerOfTwo:Yg,floorPowerOfTwo:Jg,setQuaternionFromProperEuler:Zg,normalize:$t,denormalize:xi},Ge=class s{static{s.prototype.isVector2=!0}constructor(e=0,t=0){this.x=e,this.y=t}get width(){return this.x}set width(e){this.x=e}get height(){return this.y}set height(e){this.y=e}set(e,t){return this.x=e,this.y=t,this}setScalar(e){return this.x=e,this.y=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;default:throw new Error("THREE.Vector2: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;default:throw new Error("THREE.Vector2: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y)}copy(e){return this.x=e.x,this.y=e.y,this}add(e){return this.x+=e.x,this.y+=e.y,this}addScalar(e){return this.x+=e,this.y+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this}subScalar(e){return this.x-=e,this.y-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this}multiply(e){return this.x*=e.x,this.y*=e.y,this}multiplyScalar(e){return this.x*=e,this.y*=e,this}divide(e){return this.x/=e.x,this.y/=e.y,this}divideScalar(e){return this.multiplyScalar(1/e)}applyMatrix3(e){let t=this.x,n=this.y,i=e.elements;return this.x=i[0]*t+i[3]*n+i[6],this.y=i[1]*t+i[4]*n+i[7],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this}clamp(e,t){return this.x=Lt(this.x,e.x,t.x),this.y=Lt(this.y,e.y,t.y),this}clampScalar(e,t){return this.x=Lt(this.x,e,t),this.y=Lt(this.y,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(Lt(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(e){return this.x*e.x+this.y*e.y}cross(e){return this.x*e.y-this.y*e.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(Lt(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y;return t*t+n*n}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this}equals(e){return e.x===this.x&&e.y===this.y}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this}rotateAround(e,t){let n=Math.cos(t),i=Math.sin(t),r=this.x-e.x,a=this.y-e.y;return this.x=r*n-a*i+e.x,this.y=r*i+a*n+e.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},nn=class{constructor(e=0,t=0,n=0,i=1){this.isQuaternion=!0,this._x=e,this._y=t,this._z=n,this._w=i}static slerpFlat(e,t,n,i,r,a,o){let l=n[i+0],c=n[i+1],h=n[i+2],d=n[i+3],u=r[a+0],p=r[a+1],x=r[a+2],M=r[a+3];if(d!==M||l!==u||c!==p||h!==x){let m=l*u+c*p+h*x+d*M;m<0&&(u=-u,p=-p,x=-x,M=-M,m=-m);let f=1-o;if(m<.9995){let b=Math.acos(m),E=Math.sin(b);f=Math.sin(f*b)/E,o=Math.sin(o*b)/E,l=l*f+u*o,c=c*f+p*o,h=h*f+x*o,d=d*f+M*o}else{l=l*f+u*o,c=c*f+p*o,h=h*f+x*o,d=d*f+M*o;let b=1/Math.sqrt(l*l+c*c+h*h+d*d);l*=b,c*=b,h*=b,d*=b}}e[t]=l,e[t+1]=c,e[t+2]=h,e[t+3]=d}static multiplyQuaternionsFlat(e,t,n,i,r,a){let o=n[i],l=n[i+1],c=n[i+2],h=n[i+3],d=r[a],u=r[a+1],p=r[a+2],x=r[a+3];return e[t]=o*x+h*d+l*p-c*u,e[t+1]=l*x+h*u+c*d-o*p,e[t+2]=c*x+h*p+o*u-l*d,e[t+3]=h*x-o*d-l*u-c*p,e}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get w(){return this._w}set w(e){this._w=e,this._onChangeCallback()}set(e,t,n,i){return this._x=e,this._y=t,this._z=n,this._w=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(e){return this._x=e.x,this._y=e.y,this._z=e.z,this._w=e.w,this._onChangeCallback(),this}setFromEuler(e,t=!0){let n=e._x,i=e._y,r=e._z,a=e._order,o=Math.cos,l=Math.sin,c=o(n/2),h=o(i/2),d=o(r/2),u=l(n/2),p=l(i/2),x=l(r/2);switch(a){case"XYZ":this._x=u*h*d+c*p*x,this._y=c*p*d-u*h*x,this._z=c*h*x+u*p*d,this._w=c*h*d-u*p*x;break;case"YXZ":this._x=u*h*d+c*p*x,this._y=c*p*d-u*h*x,this._z=c*h*x-u*p*d,this._w=c*h*d+u*p*x;break;case"ZXY":this._x=u*h*d-c*p*x,this._y=c*p*d+u*h*x,this._z=c*h*x+u*p*d,this._w=c*h*d-u*p*x;break;case"ZYX":this._x=u*h*d-c*p*x,this._y=c*p*d+u*h*x,this._z=c*h*x-u*p*d,this._w=c*h*d+u*p*x;break;case"YZX":this._x=u*h*d+c*p*x,this._y=c*p*d+u*h*x,this._z=c*h*x-u*p*d,this._w=c*h*d-u*p*x;break;case"XZY":this._x=u*h*d-c*p*x,this._y=c*p*d-u*h*x,this._z=c*h*x+u*p*d,this._w=c*h*d+u*p*x;break;default:dt("Quaternion: .setFromEuler() encountered an unknown order: "+a)}return t===!0&&this._onChangeCallback(),this}setFromAxisAngle(e,t){let n=t/2,i=Math.sin(n);return this._x=e.x*i,this._y=e.y*i,this._z=e.z*i,this._w=Math.cos(n),this._onChangeCallback(),this}setFromRotationMatrix(e){let t=e.elements,n=t[0],i=t[4],r=t[8],a=t[1],o=t[5],l=t[9],c=t[2],h=t[6],d=t[10],u=n+o+d;if(u>0){let p=.5/Math.sqrt(u+1);this._w=.25/p,this._x=(h-l)*p,this._y=(r-c)*p,this._z=(a-i)*p}else if(n>o&&n>d){let p=2*Math.sqrt(1+n-o-d);this._w=(h-l)/p,this._x=.25*p,this._y=(i+a)/p,this._z=(r+c)/p}else if(o>d){let p=2*Math.sqrt(1+o-n-d);this._w=(r-c)/p,this._x=(i+a)/p,this._y=.25*p,this._z=(l+h)/p}else{let p=2*Math.sqrt(1+d-n-o);this._w=(a-i)/p,this._x=(r+c)/p,this._y=(l+h)/p,this._z=.25*p}return this._onChangeCallback(),this}setFromUnitVectors(e,t){let n=e.dot(t)+1;return n<1e-8?(n=0,Math.abs(e.x)>Math.abs(e.z)?(this._x=-e.y,this._y=e.x,this._z=0,this._w=n):(this._x=0,this._y=-e.z,this._z=e.y,this._w=n)):(this._x=e.y*t.z-e.z*t.y,this._y=e.z*t.x-e.x*t.z,this._z=e.x*t.y-e.y*t.x,this._w=n),this.normalize()}angleTo(e){return 2*Math.acos(Math.abs(Lt(this.dot(e),-1,1)))}rotateTowards(e,t){let n=this.angleTo(e);if(n===0)return this;let i=Math.min(1,t/n);return this.slerp(e,i),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(e){return this._x*e._x+this._y*e._y+this._z*e._z+this._w*e._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let e=this.length();return e===0?(this._x=0,this._y=0,this._z=0,this._w=1):(e=1/e,this._x=this._x*e,this._y=this._y*e,this._z=this._z*e,this._w=this._w*e),this._onChangeCallback(),this}multiply(e){return this.multiplyQuaternions(this,e)}premultiply(e){return this.multiplyQuaternions(e,this)}multiplyQuaternions(e,t){let n=e._x,i=e._y,r=e._z,a=e._w,o=t._x,l=t._y,c=t._z,h=t._w;return this._x=n*h+a*o+i*c-r*l,this._y=i*h+a*l+r*o-n*c,this._z=r*h+a*c+n*l-i*o,this._w=a*h-n*o-i*l-r*c,this._onChangeCallback(),this}slerp(e,t){let n=e._x,i=e._y,r=e._z,a=e._w,o=this.dot(e);o<0&&(n=-n,i=-i,r=-r,a=-a,o=-o);let l=1-t;if(o<.9995){let c=Math.acos(o),h=Math.sin(c);l=Math.sin(l*c)/h,t=Math.sin(t*c)/h,this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+a*t,this._onChangeCallback()}else this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+a*t,this.normalize();return this}slerpQuaternions(e,t,n){return this.copy(e).slerp(t,n)}random(){let e=2*Math.PI*Math.random(),t=2*Math.PI*Math.random(),n=Math.random(),i=Math.sqrt(1-n),r=Math.sqrt(n);return this.set(i*Math.sin(e),i*Math.cos(e),r*Math.sin(t),r*Math.cos(t))}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._w===this._w}fromArray(e,t=0){return this._x=e[t],this._y=e[t+1],this._z=e[t+2],this._w=e[t+3],this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._w,e}fromBufferAttribute(e,t){return this._x=e.getX(t),this._y=e.getY(t),this._z=e.getZ(t),this._w=e.getW(t),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},U=class s{static{s.prototype.isVector3=!0}constructor(e=0,t=0,n=0){this.x=e,this.y=t,this.z=n}set(e,t,n){return n===void 0&&(n=this.z),this.x=e,this.y=t,this.z=n,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;default:throw new Error("THREE.Vector3: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("THREE.Vector3: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this}multiplyVectors(e,t){return this.x=e.x*t.x,this.y=e.y*t.y,this.z=e.z*t.z,this}applyEuler(e){return this.applyQuaternion(Hd.setFromEuler(e))}applyAxisAngle(e,t){return this.applyQuaternion(Hd.setFromAxisAngle(e,t))}applyMatrix3(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[3]*n+r[6]*i,this.y=r[1]*t+r[4]*n+r[7]*i,this.z=r[2]*t+r[5]*n+r[8]*i,this}applyNormalMatrix(e){return this.applyMatrix3(e).normalize()}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=e.elements,a=1/(r[3]*t+r[7]*n+r[11]*i+r[15]);return this.x=(r[0]*t+r[4]*n+r[8]*i+r[12])*a,this.y=(r[1]*t+r[5]*n+r[9]*i+r[13])*a,this.z=(r[2]*t+r[6]*n+r[10]*i+r[14])*a,this}applyQuaternion(e){let t=this.x,n=this.y,i=this.z,r=e.x,a=e.y,o=e.z,l=e.w,c=2*(a*i-o*n),h=2*(o*t-r*i),d=2*(r*n-a*t);return this.x=t+l*c+a*d-o*h,this.y=n+l*h+o*c-r*d,this.z=i+l*d+r*h-a*c,this}project(e){return this.applyMatrix4(e.matrixWorldInverse).applyMatrix4(e.projectionMatrix)}unproject(e){return this.applyMatrix4(e.projectionMatrixInverse).applyMatrix4(e.matrixWorld)}transformDirection(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[4]*n+r[8]*i,this.y=r[1]*t+r[5]*n+r[9]*i,this.z=r[2]*t+r[6]*n+r[10]*i,this.normalize()}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this}divideScalar(e){return this.multiplyScalar(1/e)}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this}clamp(e,t){return this.x=Lt(this.x,e.x,t.x),this.y=Lt(this.y,e.y,t.y),this.z=Lt(this.z,e.z,t.z),this}clampScalar(e,t){return this.x=Lt(this.x,e,t),this.y=Lt(this.y,e,t),this.z=Lt(this.z,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(Lt(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this}cross(e){return this.crossVectors(this,e)}crossVectors(e,t){let n=e.x,i=e.y,r=e.z,a=t.x,o=t.y,l=t.z;return this.x=i*l-r*o,this.y=r*a-n*l,this.z=n*o-i*a,this}projectOnVector(e){let t=e.lengthSq();if(t===0)return this.set(0,0,0);let n=e.dot(this)/t;return this.copy(e).multiplyScalar(n)}projectOnPlane(e){return xh.copy(this).projectOnVector(e),this.sub(xh)}reflect(e){return this.sub(xh.copy(e).multiplyScalar(2*this.dot(e)))}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(Lt(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y,i=this.z-e.z;return t*t+n*n+i*i}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)+Math.abs(this.z-e.z)}setFromSpherical(e){return this.setFromSphericalCoords(e.radius,e.phi,e.theta)}setFromSphericalCoords(e,t,n){let i=Math.sin(t)*e;return this.x=i*Math.sin(n),this.y=Math.cos(t)*e,this.z=i*Math.cos(n),this}setFromCylindrical(e){return this.setFromCylindricalCoords(e.radius,e.theta,e.y)}setFromCylindricalCoords(e,t,n){return this.x=e*Math.sin(t),this.y=n,this.z=e*Math.cos(t),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this}setFromMatrixScale(e){let t=this.setFromMatrixColumn(e,0).length(),n=this.setFromMatrixColumn(e,1).length(),i=this.setFromMatrixColumn(e,2).length();return this.x=t,this.y=n,this.z=i,this}setFromMatrixColumn(e,t){return this.fromArray(e.elements,t*4)}setFromMatrix3Column(e,t){return this.fromArray(e.elements,t*3)}setFromEuler(e){return this.x=e._x,this.y=e._y,this.z=e._z,this}setFromColor(e){return this.x=e.r,this.y=e.g,this.z=e.b,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let e=Math.random()*Math.PI*2,t=Math.random()*2-1,n=Math.sqrt(1-t*t);return this.x=n*Math.cos(e),this.y=t,this.z=n*Math.sin(e),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},xh=new U,Hd=new nn,Tt=class s{static{s.prototype.isMatrix3=!0}constructor(e,t,n,i,r,a,o,l,c){this.elements=[1,0,0,0,1,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,a,o,l,c)}set(e,t,n,i,r,a,o,l,c){let h=this.elements;return h[0]=e,h[1]=i,h[2]=o,h[3]=t,h[4]=r,h[5]=l,h[6]=n,h[7]=a,h[8]=c,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],this}extractBasis(e,t,n){return e.setFromMatrix3Column(this,0),t.setFromMatrix3Column(this,1),n.setFromMatrix3Column(this,2),this}setFromMatrix4(e){let t=e.elements;return this.set(t[0],t[4],t[8],t[1],t[5],t[9],t[2],t[6],t[10]),this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,a=n[0],o=n[3],l=n[6],c=n[1],h=n[4],d=n[7],u=n[2],p=n[5],x=n[8],M=i[0],m=i[3],f=i[6],b=i[1],E=i[4],g=i[7],_=i[2],w=i[5],T=i[8];return r[0]=a*M+o*b+l*_,r[3]=a*m+o*E+l*w,r[6]=a*f+o*g+l*T,r[1]=c*M+h*b+d*_,r[4]=c*m+h*E+d*w,r[7]=c*f+h*g+d*T,r[2]=u*M+p*b+x*_,r[5]=u*m+p*E+x*w,r[8]=u*f+p*g+x*T,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[3]*=e,t[6]*=e,t[1]*=e,t[4]*=e,t[7]*=e,t[2]*=e,t[5]*=e,t[8]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8];return t*a*h-t*o*c-n*r*h+n*o*l+i*r*c-i*a*l}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8],d=h*a-o*c,u=o*l-h*r,p=c*r-a*l,x=t*d+n*u+i*p;if(x===0)return this.set(0,0,0,0,0,0,0,0,0);let M=1/x;return e[0]=d*M,e[1]=(i*c-h*n)*M,e[2]=(o*n-i*a)*M,e[3]=u*M,e[4]=(h*t-i*l)*M,e[5]=(i*r-o*t)*M,e[6]=p*M,e[7]=(n*l-c*t)*M,e[8]=(a*t-n*r)*M,this}transpose(){let e,t=this.elements;return e=t[1],t[1]=t[3],t[3]=e,e=t[2],t[2]=t[6],t[6]=e,e=t[5],t[5]=t[7],t[7]=e,this}getNormalMatrix(e){return this.setFromMatrix4(e).invert().transpose()}transposeIntoArray(e){let t=this.elements;return e[0]=t[0],e[1]=t[3],e[2]=t[6],e[3]=t[1],e[4]=t[4],e[5]=t[7],e[6]=t[2],e[7]=t[5],e[8]=t[8],this}setUvTransform(e,t,n,i,r,a,o){let l=Math.cos(r),c=Math.sin(r);return this.set(n*l,n*c,-n*(l*a+c*o)+a+e,-i*c,i*l,-i*(-c*a+l*o)+o+t,0,0,1),this}scale(e,t){return qs("Matrix3: .scale() is deprecated. Use .makeScale() instead."),this.premultiply(bh.makeScale(e,t)),this}rotate(e){return qs("Matrix3: .rotate() is deprecated. Use .makeRotation() instead."),this.premultiply(bh.makeRotation(-e)),this}translate(e,t){return qs("Matrix3: .translate() is deprecated. Use .makeTranslation() instead."),this.premultiply(bh.makeTranslation(e,t)),this}makeTranslation(e,t){return e.isVector2?this.set(1,0,e.x,0,1,e.y,0,0,1):this.set(1,0,e,0,1,t,0,0,1),this}makeRotation(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,n,t,0,0,0,1),this}makeScale(e,t){return this.set(e,0,0,0,t,0,0,0,1),this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<9;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<9;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e}clone(){return new this.constructor().fromArray(this.elements)}},bh=new Tt,Gd=new Tt().set(.4123908,.3575843,.1804808,.212639,.7151687,.0721923,.0193308,.1191948,.9505322),Vd=new Tt().set(3.2409699,-1.5373832,-.4986108,-.9692436,1.8759675,.0415551,.0556301,-.203977,1.0569715);function $g(){let s={enabled:!0,workingColorSpace:Hn,spaces:{},convert:function(i,r,a){return this.enabled===!1||r===a||!r||!a||(this.spaces[r].transfer===Xt&&(i.r=es(i.r),i.g=es(i.g),i.b=es(i.b)),this.spaces[r].primaries!==this.spaces[a].primaries&&(i.applyMatrix3(this.spaces[r].toXYZ),i.applyMatrix3(this.spaces[a].fromXYZ)),this.spaces[a].transfer===Xt&&(i.r=Cr(i.r),i.g=Cr(i.g),i.b=Cr(i.b))),i},workingToColorSpace:function(i,r){return this.convert(i,this.workingColorSpace,r)},colorSpaceToWorking:function(i,r){return this.convert(i,r,this.workingColorSpace)},getPrimaries:function(i){return this.spaces[i].primaries},getTransfer:function(i){return i===ci?La:this.spaces[i].transfer},getToneMappingMode:function(i){return this.spaces[i].outputColorSpaceConfig.toneMappingMode||"standard"},getLuminanceCoefficients:function(i,r=this.workingColorSpace){return i.fromArray(this.spaces[r].luminanceCoefficients)},define:function(i){Object.assign(this.spaces,i)},_getMatrix:function(i,r,a){return i.copy(this.spaces[r].toXYZ).multiply(this.spaces[a].fromXYZ)},_getDrawingBufferColorSpace:function(i){return this.spaces[i].outputColorSpaceConfig.drawingBufferColorSpace},_getUnpackColorSpace:function(i=this.workingColorSpace){return this.spaces[i].workingColorSpaceConfig.unpackColorSpace},fromWorkingColorSpace:function(i,r){return qs("ColorManagement: .fromWorkingColorSpace() has been renamed to .workingToColorSpace()."),s.workingToColorSpace(i,r)},toWorkingColorSpace:function(i,r){return qs("ColorManagement: .toWorkingColorSpace() has been renamed to .colorSpaceToWorking()."),s.colorSpaceToWorking(i,r)}},e=[.64,.33,.3,.6,.15,.06],t=[.2126,.7152,.0722],n=[.3127,.329];return s.define({[Hn]:{primaries:e,whitePoint:n,transfer:La,toXYZ:Gd,fromXYZ:Vd,luminanceCoefficients:t,workingColorSpaceConfig:{unpackColorSpace:an},outputColorSpaceConfig:{drawingBufferColorSpace:an}},[an]:{primaries:e,whitePoint:n,transfer:Xt,toXYZ:Gd,fromXYZ:Vd,luminanceCoefficients:t,outputColorSpaceConfig:{drawingBufferColorSpace:an}}}),s}var Dt=$g();function es(s){return s<.04045?s*.0773993808:Math.pow(s*.9478672986+.0521327014,2.4)}function Cr(s){return s<.0031308?s*12.92:1.055*Math.pow(s,.41666)-.055}var mr,Ml=class{static getDataURL(e,t="image/png"){if(/^data:/i.test(e.src)||typeof HTMLCanvasElement>"u")return e.src;let n;if(e instanceof HTMLCanvasElement)n=e;else{mr===void 0&&(mr=Lr("canvas")),mr.width=e.width,mr.height=e.height;let i=mr.getContext("2d");e instanceof ImageData?i.putImageData(e,0,0):i.drawImage(e,0,0,e.width,e.height),n=mr}return n.toDataURL(t)}static sRGBToLinear(e){if(typeof HTMLImageElement<"u"&&e instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&e instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&e instanceof ImageBitmap){let t=Lr("canvas");t.width=e.width,t.height=e.height;let n=t.getContext("2d");n.drawImage(e,0,0,e.width,e.height);let i=n.getImageData(0,0,e.width,e.height),r=i.data;for(let a=0;a<r.length;a++)r[a]=es(r[a]/255)*255;return n.putImageData(i,0,0),t}else if(e.data){let t=e.data.slice(0);for(let n=0;n<t.length;n++)t instanceof Uint8Array||t instanceof Uint8ClampedArray?t[n]=Math.floor(es(t[n]/255)*255):t[n]=es(t[n]);return{data:t,width:e.width,height:e.height}}else return dt("ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),e}},Qg=0,Nr=class{constructor(e=null){this.isTextureSource=!0,Object.defineProperty(this,"id",{value:Qg++}),this.uuid=vi(),this.data=e,this.dataReady=!0,this.version=0}getSize(e){let t=this.data;return typeof HTMLVideoElement<"u"&&t instanceof HTMLVideoElement?e.set(t.videoWidth,t.videoHeight,0):typeof VideoFrame<"u"&&t instanceof VideoFrame?e.set(t.displayWidth,t.displayHeight,0):t!==null?e.set(t.width,t.height,t.depth||0):e.set(0,0,0),e}set needsUpdate(e){e===!0&&this.version++}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.images[this.uuid]!==void 0)return e.images[this.uuid];let n={uuid:this.uuid,url:""},i=this.data;if(i!==null){let r;if(Array.isArray(i)){r=[];for(let a=0,o=i.length;a<o;a++)i[a].isDataTexture?r.push(vh(i[a].image)):r.push(vh(i[a]))}else r=vh(i);n.url=r}return t||(e.images[this.uuid]=n),n}};function vh(s){return typeof HTMLImageElement<"u"&&s instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&s instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&s instanceof ImageBitmap?Ml.getDataURL(s):s.data?{data:Array.from(s.data),width:s.width,height:s.height,type:s.data.constructor.name}:(dt("Texture: Unable to serialize Texture."),{})}var e0=0,yh=new U,hn=class s extends ai{constructor(e=s.DEFAULT_IMAGE,t=s.DEFAULT_MAPPING,n=ri,i=ri,r=on,a=Ei,o=ti,l=Xn,c=s.DEFAULT_ANISOTROPY,h=ci){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:e0++}),this.uuid=vi(),this.name="",this.source=new Nr(e),this.mipmaps=[],this.mapping=t,this.channel=0,this.wrapS=n,this.wrapT=i,this.magFilter=r,this.minFilter=a,this.anisotropy=c,this.format=o,this.internalFormat=null,this.type=l,this.offset=new Ge(0,0),this.repeat=new Ge(1,1),this.center=new Ge(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new Tt,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=h,this.userData={},this.updateRanges=[],this.version=0,this.onUpdate=null,this.renderTarget=null,this.isRenderTargetTexture=!1,this.isArrayTexture=!!(e&&e.depth&&e.depth>1),this.pmremVersion=0,this.normalized=!1}get width(){return this.source.getSize(yh).x}get height(){return this.source.getSize(yh).y}get depth(){return this.source.getSize(yh).z}get image(){return this.source.data}set image(e){this.source.data=e}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}clone(){return new this.constructor().copy(this)}copy(e){return this.name=e.name,this.source=e.source,this.mipmaps=e.mipmaps.slice(0),this.mapping=e.mapping,this.channel=e.channel,this.wrapS=e.wrapS,this.wrapT=e.wrapT,this.magFilter=e.magFilter,this.minFilter=e.minFilter,this.anisotropy=e.anisotropy,this.format=e.format,this.internalFormat=e.internalFormat,this.type=e.type,this.normalized=e.normalized,this.offset.copy(e.offset),this.repeat.copy(e.repeat),this.center.copy(e.center),this.rotation=e.rotation,this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrix.copy(e.matrix),this.generateMipmaps=e.generateMipmaps,this.premultiplyAlpha=e.premultiplyAlpha,this.flipY=e.flipY,this.unpackAlignment=e.unpackAlignment,this.colorSpace=e.colorSpace,this.renderTarget=e.renderTarget,this.isRenderTargetTexture=e.isRenderTargetTexture,this.isArrayTexture=e.isArrayTexture,this.userData=JSON.parse(JSON.stringify(e.userData)),this.needsUpdate=!0,this}setValues(e){for(let t in e){let n=e[t];if(n===void 0){dt(`Texture.setValues(): parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){dt(`Texture.setValues(): property '${t}' does not exist.`);continue}i&&n&&i.isVector2&&n.isVector2||i&&n&&i.isVector3&&n.isVector3||i&&n&&i.isMatrix3&&n.isMatrix3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.textures[this.uuid]!==void 0)return e.textures[this.uuid];let n={metadata:{version:4.7,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(e).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,normalized:this.normalized,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(n.userData=this.userData),t||(e.textures[this.uuid]=n),n}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(e){if(this.mapping!==au)return e;if(e.applyMatrix3(this.matrix),e.x<0||e.x>1)switch(this.wrapS){case Di:e.x=e.x-Math.floor(e.x);break;case ri:e.x=e.x<0?0:1;break;case Ir:Math.abs(Math.floor(e.x)%2)===1?e.x=Math.ceil(e.x)-e.x:e.x=e.x-Math.floor(e.x);break}if(e.y<0||e.y>1)switch(this.wrapT){case Di:e.y=e.y-Math.floor(e.y);break;case ri:e.y=e.y<0?0:1;break;case Ir:Math.abs(Math.floor(e.y)%2)===1?e.y=Math.ceil(e.y)-e.y:e.y=e.y-Math.floor(e.y);break}return this.flipY&&(e.y=1-e.y),e}set needsUpdate(e){e===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(e){e===!0&&this.pmremVersion++}};hn.DEFAULT_IMAGE=null;hn.DEFAULT_MAPPING=au;hn.DEFAULT_ANISOTROPY=1;var Ot=class s{static{s.prototype.isVector4=!0}constructor(e=0,t=0,n=0,i=1){this.x=e,this.y=t,this.z=n,this.w=i}get width(){return this.z}set width(e){this.z=e}get height(){return this.w}set height(e){this.w=e}set(e,t,n,i){return this.x=e,this.y=t,this.z=n,this.w=i,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this.w=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setW(e){return this.w=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;case 3:this.w=t;break;default:throw new Error("THREE.Vector4: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("THREE.Vector4: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this.w=e.w!==void 0?e.w:1,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this.w+=e.w,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this.w+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this.w=e.w+t.w,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this.w+=e.w*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this.w-=e.w,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this.w-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this.w=e.w-t.w,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this.w*=e.w,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this.w*=e,this}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=this.w,a=e.elements;return this.x=a[0]*t+a[4]*n+a[8]*i+a[12]*r,this.y=a[1]*t+a[5]*n+a[9]*i+a[13]*r,this.z=a[2]*t+a[6]*n+a[10]*i+a[14]*r,this.w=a[3]*t+a[7]*n+a[11]*i+a[15]*r,this}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this.w/=e.w,this}divideScalar(e){return this.multiplyScalar(1/e)}setAxisAngleFromQuaternion(e){this.w=2*Math.acos(e.w);let t=Math.sqrt(1-e.w*e.w);return t<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=e.x/t,this.y=e.y/t,this.z=e.z/t),this}setAxisAngleFromRotationMatrix(e){let t,n,i,r,l=e.elements,c=l[0],h=l[4],d=l[8],u=l[1],p=l[5],x=l[9],M=l[2],m=l[6],f=l[10];if(Math.abs(h-u)<.01&&Math.abs(d-M)<.01&&Math.abs(x-m)<.01){if(Math.abs(h+u)<.1&&Math.abs(d+M)<.1&&Math.abs(x+m)<.1&&Math.abs(c+p+f-3)<.1)return this.set(1,0,0,0),this;t=Math.PI;let E=(c+1)/2,g=(p+1)/2,_=(f+1)/2,w=(h+u)/4,T=(d+M)/4,y=(x+m)/4;return E>g&&E>_?E<.01?(n=0,i=.707106781,r=.707106781):(n=Math.sqrt(E),i=w/n,r=T/n):g>_?g<.01?(n=.707106781,i=0,r=.707106781):(i=Math.sqrt(g),n=w/i,r=y/i):_<.01?(n=.707106781,i=.707106781,r=0):(r=Math.sqrt(_),n=T/r,i=y/r),this.set(n,i,r,t),this}let b=Math.sqrt((m-x)*(m-x)+(d-M)*(d-M)+(u-h)*(u-h));return Math.abs(b)<.001&&(b=1),this.x=(m-x)/b,this.y=(d-M)/b,this.z=(u-h)/b,this.w=Math.acos((c+p+f-1)/2),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this.w=t[15],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this.w=Math.min(this.w,e.w),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this.w=Math.max(this.w,e.w),this}clamp(e,t){return this.x=Lt(this.x,e.x,t.x),this.y=Lt(this.y,e.y,t.y),this.z=Lt(this.z,e.z,t.z),this.w=Lt(this.w,e.w,t.w),this}clampScalar(e,t){return this.x=Lt(this.x,e,t),this.y=Lt(this.y,e,t),this.z=Lt(this.z,e,t),this.w=Lt(this.w,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(Lt(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z+this.w*e.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this.w+=(e.w-this.w)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this.w=e.w+(t.w-e.w)*n,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z&&e.w===this.w}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this.w=e[t+3],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e[t+3]=this.w,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this.w=e.getW(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},wl=class extends ai{constructor(e=1,t=1,n={}){super(),n=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:on,depthBuffer:!0,stencilBuffer:!1,resolveColorBuffer:!0,resolveDepthBuffer:!0,resolveStencilBuffer:!0,storeMultisampledColorBuffer:!0,storeMultisampledDepthBuffer:!0,storeMultisampledStencilBuffer:!0,depthTexture:null,samples:0,count:1,depth:1,multiview:!1,useArrayDepthTexture:!1},n),this.isRenderTarget=!0,this.width=e,this.height=t,this.depth=n.depth,this.scissor=new Ot(0,0,e,t),this.scissorTest=!1,this.viewport=new Ot(0,0,e,t),this.textures=[];let i={width:e,height:t,depth:n.depth},r=new hn(i),a=n.count;for(let o=0;o<a;o++)this.textures[o]=r.clone(),this.textures[o].isRenderTargetTexture=!0,this.textures[o].renderTarget=this;this._setTextureOptions(n),this.depthBuffer=n.depthBuffer,this.stencilBuffer=n.stencilBuffer,this.resolveColorBuffer=n.resolveColorBuffer,this.resolveDepthBuffer=n.resolveDepthBuffer,this.resolveStencilBuffer=n.resolveStencilBuffer,this.storeMultisampledColorBuffer=n.storeMultisampledColorBuffer,this.storeMultisampledDepthBuffer=n.storeMultisampledDepthBuffer,this.storeMultisampledStencilBuffer=n.storeMultisampledStencilBuffer,this._depthTexture=null,this.depthTexture=n.depthTexture,this.samples=n.samples,this.multiview=n.multiview,this.useArrayDepthTexture=n.useArrayDepthTexture}_setTextureOptions(e={}){let t={minFilter:on,generateMipmaps:!1,flipY:!1,internalFormat:null};e.mapping!==void 0&&(t.mapping=e.mapping),e.wrapS!==void 0&&(t.wrapS=e.wrapS),e.wrapT!==void 0&&(t.wrapT=e.wrapT),e.wrapR!==void 0&&(t.wrapR=e.wrapR),e.magFilter!==void 0&&(t.magFilter=e.magFilter),e.minFilter!==void 0&&(t.minFilter=e.minFilter),e.format!==void 0&&(t.format=e.format),e.type!==void 0&&(t.type=e.type),e.anisotropy!==void 0&&(t.anisotropy=e.anisotropy),e.colorSpace!==void 0&&(t.colorSpace=e.colorSpace),e.flipY!==void 0&&(t.flipY=e.flipY),e.generateMipmaps!==void 0&&(t.generateMipmaps=e.generateMipmaps),e.internalFormat!==void 0&&(t.internalFormat=e.internalFormat);for(let n=0;n<this.textures.length;n++)this.textures[n].setValues(t)}get texture(){return this.textures[0]}set texture(e){this.textures[0]=e}set depthTexture(e){this._depthTexture!==null&&this._depthTexture.renderTarget===this&&(this._depthTexture.renderTarget=null),e!==null&&e.renderTarget===null&&(e.renderTarget=this),this._depthTexture=e}get depthTexture(){return this._depthTexture}setSize(e,t,n=1){if(this.width!==e||this.height!==t||this.depth!==n){this.width=e,this.height=t,this.depth=n;for(let i=0,r=this.textures.length;i<r;i++)this.textures[i].image.width=e,this.textures[i].image.height=t,this.textures[i].image.depth=n,this.textures[i].isData3DTexture!==!0&&(this.textures[i].isArrayTexture=this.textures[i].image.depth>1);this.dispose()}this.viewport.set(0,0,e,t),this.scissor.set(0,0,e,t)}clone(){return new this.constructor().copy(this)}copy(e){this.width=e.width,this.height=e.height,this.depth=e.depth,this.scissor.copy(e.scissor),this.scissorTest=e.scissorTest,this.viewport.copy(e.viewport),this.textures.length=0;for(let t=0,n=e.textures.length;t<n;t++){this.textures[t]=e.textures[t].clone(),this.textures[t].isRenderTargetTexture=!0,this.textures[t].renderTarget=this;let i=Object.assign({},e.textures[t].image);this.textures[t].source=new Nr(i)}if(this.depthBuffer=e.depthBuffer,this.stencilBuffer=e.stencilBuffer,this.resolveColorBuffer=e.resolveColorBuffer,this.resolveDepthBuffer=e.resolveDepthBuffer,this.resolveStencilBuffer=e.resolveStencilBuffer,this.storeMultisampledColorBuffer=e.storeMultisampledColorBuffer,this.storeMultisampledDepthBuffer=e.storeMultisampledDepthBuffer,this.storeMultisampledStencilBuffer=e.storeMultisampledStencilBuffer,e.depthTexture!==null)if(e.depthTexture.renderTarget===e){let t=e.depthTexture.clone();t.renderTarget=null,this.depthTexture=t}else this.depthTexture=e.depthTexture;return this.samples=e.samples,this.multiview=e.multiview,this.useArrayDepthTexture=e.useArrayDepthTexture,this}dispose(){this.dispatchEvent({type:"dispose"})}},ln=class extends wl{constructor(e=1,t=1,n={}){super(e,t,n),this.isWebGLRenderTarget=!0}},Na=class extends hn{constructor(e=null,t=1,n=1,i=1){super(null),this.isDataArrayTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=cn,this.minFilter=cn,this.wrapR=ri,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}copy(e){return super.copy(e),this.wrapR=e.wrapR,this}addLayerUpdate(e){this.layerUpdates.add(e)}clearLayerUpdates(){this.layerUpdates.clear()}};var Sl=class extends hn{constructor(e=null,t=1,n=1,i=1){super(null),this.isData3DTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=cn,this.minFilter=cn,this.wrapR=ri,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}copy(e){return super.copy(e),this.wrapR=e.wrapR,this}};var yt=class s{static{s.prototype.isMatrix4=!0}constructor(e,t,n,i,r,a,o,l,c,h,d,u,p,x,M,m){this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,a,o,l,c,h,d,u,p,x,M,m)}set(e,t,n,i,r,a,o,l,c,h,d,u,p,x,M,m){let f=this.elements;return f[0]=e,f[4]=t,f[8]=n,f[12]=i,f[1]=r,f[5]=a,f[9]=o,f[13]=l,f[2]=c,f[6]=h,f[10]=d,f[14]=u,f[3]=p,f[7]=x,f[11]=M,f[15]=m,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new s().fromArray(this.elements)}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],t[9]=n[9],t[10]=n[10],t[11]=n[11],t[12]=n[12],t[13]=n[13],t[14]=n[14],t[15]=n[15],this}copyPosition(e){let t=this.elements,n=e.elements;return t[12]=n[12],t[13]=n[13],t[14]=n[14],this}setFromMatrix3(e){let t=e.elements;return this.set(t[0],t[3],t[6],0,t[1],t[4],t[7],0,t[2],t[5],t[8],0,0,0,0,1),this}extractBasis(e,t,n){return this.determinantAffine()===0?(e.set(1,0,0),t.set(0,1,0),n.set(0,0,1),this):(e.setFromMatrixColumn(this,0),t.setFromMatrixColumn(this,1),n.setFromMatrixColumn(this,2),this)}makeBasis(e,t,n){return this.set(e.x,t.x,n.x,0,e.y,t.y,n.y,0,e.z,t.z,n.z,0,0,0,0,1),this}extractRotation(e){if(e.determinantAffine()===0)return this.identity();let t=this.elements,n=e.elements,i=1/gr.setFromMatrixColumn(e,0).length(),r=1/gr.setFromMatrixColumn(e,1).length(),a=1/gr.setFromMatrixColumn(e,2).length();return t[0]=n[0]*i,t[1]=n[1]*i,t[2]=n[2]*i,t[3]=0,t[4]=n[4]*r,t[5]=n[5]*r,t[6]=n[6]*r,t[7]=0,t[8]=n[8]*a,t[9]=n[9]*a,t[10]=n[10]*a,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromEuler(e){let t=this.elements,n=e.x,i=e.y,r=e.z,a=Math.cos(n),o=Math.sin(n),l=Math.cos(i),c=Math.sin(i),h=Math.cos(r),d=Math.sin(r);if(e.order==="XYZ"){let u=a*h,p=a*d,x=o*h,M=o*d;t[0]=l*h,t[4]=-l*d,t[8]=c,t[1]=p+x*c,t[5]=u-M*c,t[9]=-o*l,t[2]=M-u*c,t[6]=x+p*c,t[10]=a*l}else if(e.order==="YXZ"){let u=l*h,p=l*d,x=c*h,M=c*d;t[0]=u+M*o,t[4]=x*o-p,t[8]=a*c,t[1]=a*d,t[5]=a*h,t[9]=-o,t[2]=p*o-x,t[6]=M+u*o,t[10]=a*l}else if(e.order==="ZXY"){let u=l*h,p=l*d,x=c*h,M=c*d;t[0]=u-M*o,t[4]=-a*d,t[8]=x+p*o,t[1]=p+x*o,t[5]=a*h,t[9]=M-u*o,t[2]=-a*c,t[6]=o,t[10]=a*l}else if(e.order==="ZYX"){let u=a*h,p=a*d,x=o*h,M=o*d;t[0]=l*h,t[4]=x*c-p,t[8]=u*c+M,t[1]=l*d,t[5]=M*c+u,t[9]=p*c-x,t[2]=-c,t[6]=o*l,t[10]=a*l}else if(e.order==="YZX"){let u=a*l,p=a*c,x=o*l,M=o*c;t[0]=l*h,t[4]=M-u*d,t[8]=x*d+p,t[1]=d,t[5]=a*h,t[9]=-o*h,t[2]=-c*h,t[6]=p*d+x,t[10]=u-M*d}else if(e.order==="XZY"){let u=a*l,p=a*c,x=o*l,M=o*c;t[0]=l*h,t[4]=-d,t[8]=c*h,t[1]=u*d+M,t[5]=a*h,t[9]=p*d-x,t[2]=x*d-p,t[6]=o*h,t[10]=M*d+u}return t[3]=0,t[7]=0,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromQuaternion(e){return this.compose(t0,e,n0)}lookAt(e,t,n){let i=this.elements;return Yn.subVectors(e,t),Yn.lengthSq()===0&&(Yn.z=1),Yn.normalize(),ps.crossVectors(n,Yn),ps.lengthSq()===0&&(Math.abs(n.z)===1?Yn.x+=1e-4:Yn.z+=1e-4,Yn.normalize(),ps.crossVectors(n,Yn)),ps.normalize(),Ho.crossVectors(Yn,ps),i[0]=ps.x,i[4]=Ho.x,i[8]=Yn.x,i[1]=ps.y,i[5]=Ho.y,i[9]=Yn.y,i[2]=ps.z,i[6]=Ho.z,i[10]=Yn.z,this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,a=n[0],o=n[4],l=n[8],c=n[12],h=n[1],d=n[5],u=n[9],p=n[13],x=n[2],M=n[6],m=n[10],f=n[14],b=n[3],E=n[7],g=n[11],_=n[15],w=i[0],T=i[4],y=i[8],C=i[12],S=i[1],D=i[5],v=i[9],I=i[13],P=i[2],N=i[6],z=i[10],X=i[14],j=i[3],V=i[7],W=i[11],ie=i[15];return r[0]=a*w+o*S+l*P+c*j,r[4]=a*T+o*D+l*N+c*V,r[8]=a*y+o*v+l*z+c*W,r[12]=a*C+o*I+l*X+c*ie,r[1]=h*w+d*S+u*P+p*j,r[5]=h*T+d*D+u*N+p*V,r[9]=h*y+d*v+u*z+p*W,r[13]=h*C+d*I+u*X+p*ie,r[2]=x*w+M*S+m*P+f*j,r[6]=x*T+M*D+m*N+f*V,r[10]=x*y+M*v+m*z+f*W,r[14]=x*C+M*I+m*X+f*ie,r[3]=b*w+E*S+g*P+_*j,r[7]=b*T+E*D+g*N+_*V,r[11]=b*y+E*v+g*z+_*W,r[15]=b*C+E*I+g*X+_*ie,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[4]*=e,t[8]*=e,t[12]*=e,t[1]*=e,t[5]*=e,t[9]*=e,t[13]*=e,t[2]*=e,t[6]*=e,t[10]*=e,t[14]*=e,t[3]*=e,t[7]*=e,t[11]*=e,t[15]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[12],a=e[1],o=e[5],l=e[9],c=e[13],h=e[2],d=e[6],u=e[10],p=e[14],x=e[3],M=e[7],m=e[11],f=e[15],b=l*p-c*u,E=o*p-c*d,g=o*u-l*d,_=a*p-c*h,w=a*u-l*h,T=a*d-o*h;return t*(M*b-m*E+f*g)-n*(x*b-m*_+f*w)+i*(x*E-M*_+f*T)-r*(x*g-M*w+m*T)}determinantAffine(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[1],a=e[5],o=e[9],l=e[2],c=e[6],h=e[10];return t*(a*h-o*c)-n*(r*h-o*l)+i*(r*c-a*l)}transpose(){let e=this.elements,t;return t=e[1],e[1]=e[4],e[4]=t,t=e[2],e[2]=e[8],e[8]=t,t=e[6],e[6]=e[9],e[9]=t,t=e[3],e[3]=e[12],e[12]=t,t=e[7],e[7]=e[13],e[13]=t,t=e[11],e[11]=e[14],e[14]=t,this}setPosition(e,t,n){let i=this.elements;return e.isVector3?(i[12]=e.x,i[13]=e.y,i[14]=e.z):(i[12]=e,i[13]=t,i[14]=n),this}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],a=e[4],o=e[5],l=e[6],c=e[7],h=e[8],d=e[9],u=e[10],p=e[11],x=e[12],M=e[13],m=e[14],f=e[15],b=t*o-n*a,E=t*l-i*a,g=t*c-r*a,_=n*l-i*o,w=n*c-r*o,T=i*c-r*l,y=h*M-d*x,C=h*m-u*x,S=h*f-p*x,D=d*m-u*M,v=d*f-p*M,I=u*f-p*m,P=b*I-E*v+g*D+_*S-w*C+T*y;if(P===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let N=1/P;return e[0]=(o*I-l*v+c*D)*N,e[1]=(i*v-n*I-r*D)*N,e[2]=(M*T-m*w+f*_)*N,e[3]=(u*w-d*T-p*_)*N,e[4]=(l*S-a*I-c*C)*N,e[5]=(t*I-i*S+r*C)*N,e[6]=(m*g-x*T-f*E)*N,e[7]=(h*T-u*g+p*E)*N,e[8]=(a*v-o*S+c*y)*N,e[9]=(n*S-t*v-r*y)*N,e[10]=(x*w-M*g+f*b)*N,e[11]=(d*g-h*w-p*b)*N,e[12]=(o*C-a*D-l*y)*N,e[13]=(t*D-n*C+i*y)*N,e[14]=(M*E-x*_-m*b)*N,e[15]=(h*_-d*E+u*b)*N,this}scale(e){let t=this.elements,n=e.x,i=e.y,r=e.z;return t[0]*=n,t[4]*=i,t[8]*=r,t[1]*=n,t[5]*=i,t[9]*=r,t[2]*=n,t[6]*=i,t[10]*=r,t[3]*=n,t[7]*=i,t[11]*=r,this}getMaxScaleOnAxis(){let e=this.elements,t=e[0]*e[0]+e[1]*e[1]+e[2]*e[2],n=e[4]*e[4]+e[5]*e[5]+e[6]*e[6],i=e[8]*e[8]+e[9]*e[9]+e[10]*e[10];return Math.sqrt(Math.max(t,n,i))}makeTranslation(e,t,n){return e.isVector3?this.set(1,0,0,e.x,0,1,0,e.y,0,0,1,e.z,0,0,0,1):this.set(1,0,0,e,0,1,0,t,0,0,1,n,0,0,0,1),this}makeRotationX(e){let t=Math.cos(e),n=Math.sin(e);return this.set(1,0,0,0,0,t,-n,0,0,n,t,0,0,0,0,1),this}makeRotationY(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,0,n,0,0,1,0,0,-n,0,t,0,0,0,0,1),this}makeRotationZ(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,0,n,t,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(e,t){let n=Math.cos(t),i=Math.sin(t),r=1-n,a=e.x,o=e.y,l=e.z,c=r*a,h=r*o;return this.set(c*a+n,c*o-i*l,c*l+i*o,0,c*o+i*l,h*o+n,h*l-i*a,0,c*l-i*o,h*l+i*a,r*l*l+n,0,0,0,0,1),this}makeScale(e,t,n){return this.set(e,0,0,0,0,t,0,0,0,0,n,0,0,0,0,1),this}makeShear(e,t,n,i,r,a){return this.set(1,n,r,0,e,1,a,0,t,i,1,0,0,0,0,1),this}compose(e,t,n){let i=this.elements,r=t._x,a=t._y,o=t._z,l=t._w,c=r+r,h=a+a,d=o+o,u=r*c,p=r*h,x=r*d,M=a*h,m=a*d,f=o*d,b=l*c,E=l*h,g=l*d,_=n.x,w=n.y,T=n.z;return i[0]=(1-(M+f))*_,i[1]=(p+g)*_,i[2]=(x-E)*_,i[3]=0,i[4]=(p-g)*w,i[5]=(1-(u+f))*w,i[6]=(m+b)*w,i[7]=0,i[8]=(x+E)*T,i[9]=(m-b)*T,i[10]=(1-(u+M))*T,i[11]=0,i[12]=e.x,i[13]=e.y,i[14]=e.z,i[15]=1,this}decompose(e,t,n){let i=this.elements;e.x=i[12],e.y=i[13],e.z=i[14];let r=this.determinantAffine();if(r===0)return n.set(1,1,1),t.identity(),this;let a=gr.set(i[0],i[1],i[2]).length(),o=gr.set(i[4],i[5],i[6]).length(),l=gr.set(i[8],i[9],i[10]).length();r<0&&(a=-a),pi.copy(this);let c=1/a,h=1/o,d=1/l;return pi.elements[0]*=c,pi.elements[1]*=c,pi.elements[2]*=c,pi.elements[4]*=h,pi.elements[5]*=h,pi.elements[6]*=h,pi.elements[8]*=d,pi.elements[9]*=d,pi.elements[10]*=d,t.setFromRotationMatrix(pi),n.x=a,n.y=o,n.z=l,this}makePerspective(e,t,n,i,r,a,o=bi,l=!1){let c=this.elements,h=2*r/(t-e),d=2*r/(n-i),u=(t+e)/(t-e),p=(n+i)/(n-i),x,M;if(l)x=r/(a-r),M=a*r/(a-r);else if(o===bi)x=-(a+r)/(a-r),M=-2*a*r/(a-r);else if(o===Dr)x=-a/(a-r),M=-a*r/(a-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+o);return c[0]=h,c[4]=0,c[8]=u,c[12]=0,c[1]=0,c[5]=d,c[9]=p,c[13]=0,c[2]=0,c[6]=0,c[10]=x,c[14]=M,c[3]=0,c[7]=0,c[11]=-1,c[15]=0,this}makeOrthographic(e,t,n,i,r,a,o=bi,l=!1){let c=this.elements,h=2/(t-e),d=2/(n-i),u=-(t+e)/(t-e),p=-(n+i)/(n-i),x,M;if(l)x=1/(a-r),M=a/(a-r);else if(o===bi)x=-2/(a-r),M=-(a+r)/(a-r);else if(o===Dr)x=-1/(a-r),M=-r/(a-r);else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+o);return c[0]=h,c[4]=0,c[8]=0,c[12]=u,c[1]=0,c[5]=d,c[9]=0,c[13]=p,c[2]=0,c[6]=0,c[10]=x,c[14]=M,c[3]=0,c[7]=0,c[11]=0,c[15]=1,this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<16;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<16;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e[t+9]=n[9],e[t+10]=n[10],e[t+11]=n[11],e[t+12]=n[12],e[t+13]=n[13],e[t+14]=n[14],e[t+15]=n[15],e}},gr=new U,pi=new yt,t0=new U(0,0,0),n0=new U(1,1,1),ps=new U,Ho=new U,Yn=new U,Wd=new yt,qd=new nn,Vn=class s{constructor(e=0,t=0,n=0,i=s.DEFAULT_ORDER){this.isEuler=!0,this._x=e,this._y=t,this._z=n,this._order=i}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get order(){return this._order}set order(e){this._order=e,this._onChangeCallback()}set(e,t,n,i=this._order){return this._x=e,this._y=t,this._z=n,this._order=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(e){return this._x=e._x,this._y=e._y,this._z=e._z,this._order=e._order,this._onChangeCallback(),this}setFromRotationMatrix(e,t=this._order,n=!0){let i=e.elements,r=i[0],a=i[4],o=i[8],l=i[1],c=i[5],h=i[9],d=i[2],u=i[6],p=i[10];switch(t){case"XYZ":this._y=Math.asin(Lt(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(-h,p),this._z=Math.atan2(-a,r)):(this._x=Math.atan2(u,c),this._z=0);break;case"YXZ":this._x=Math.asin(-Lt(h,-1,1)),Math.abs(h)<.9999999?(this._y=Math.atan2(o,p),this._z=Math.atan2(l,c)):(this._y=Math.atan2(-d,r),this._z=0);break;case"ZXY":this._x=Math.asin(Lt(u,-1,1)),Math.abs(u)<.9999999?(this._y=Math.atan2(-d,p),this._z=Math.atan2(-a,c)):(this._y=0,this._z=Math.atan2(l,r));break;case"ZYX":this._y=Math.asin(-Lt(d,-1,1)),Math.abs(d)<.9999999?(this._x=Math.atan2(u,p),this._z=Math.atan2(l,r)):(this._x=0,this._z=Math.atan2(-a,c));break;case"YZX":this._z=Math.asin(Lt(l,-1,1)),Math.abs(l)<.9999999?(this._x=Math.atan2(-h,c),this._y=Math.atan2(-d,r)):(this._x=0,this._y=Math.atan2(o,p));break;case"XZY":this._z=Math.asin(-Lt(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(u,c),this._y=Math.atan2(o,r)):(this._x=Math.atan2(-h,p),this._y=0);break;default:dt("Euler: .setFromRotationMatrix() encountered an unknown order: "+t)}return this._order=t,n===!0&&this._onChangeCallback(),this}setFromQuaternion(e,t,n){return Wd.makeRotationFromQuaternion(e),this.setFromRotationMatrix(Wd,t,n)}setFromVector3(e,t=this._order){return this.set(e.x,e.y,e.z,t)}reorder(e){return qd.setFromEuler(this),this.setFromQuaternion(qd,e)}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._order===this._order}fromArray(e){return this._x=e[0],this._y=e[1],this._z=e[2],e[3]!==void 0&&(this._order=e[3]),this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._order,e}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};Vn.DEFAULT_ORDER="XYZ";var Ur=class{constructor(){this.mask=1}set(e){this.mask=(1<<e|0)>>>0}enable(e){this.mask|=1<<e|0}enableAll(){this.mask=-1}toggle(e){this.mask^=1<<e|0}disable(e){this.mask&=~(1<<e|0)}disableAll(){this.mask=0}test(e){return(this.mask&e.mask)!==0}isEnabled(e){return(this.mask&(1<<e|0))!==0}},i0=0,Xd=new U,xr=new nn,Ki=new yt,Go=new U,ya=new U,s0=new U,r0=new nn,jd=new U(1,0,0),Kd=new U(0,1,0),Yd=new U(0,0,1),Jd={type:"added"},a0={type:"removed"},br={type:"childadded",child:null},_h={type:"childremoved",child:null},Yt=class s extends ai{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:i0++}),this.uuid=vi(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=s.DEFAULT_UP.clone();let e=new U,t=new Vn,n=new nn,i=new U(1,1,1);function r(){n.setFromEuler(t,!1)}function a(){t.setFromQuaternion(n,void 0,!1)}t._onChange(r),n._onChange(a),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:e},rotation:{configurable:!0,enumerable:!0,value:t},quaternion:{configurable:!0,enumerable:!0,value:n},scale:{configurable:!0,enumerable:!0,value:i},modelViewMatrix:{value:new yt},normalMatrix:{value:new Tt}}),this.matrix=new yt,this.matrixWorld=new yt,this.matrixAutoUpdate=s.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=s.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new Ur,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.customDepthMaterial=void 0,this.customDistanceMaterial=void 0,this.static=!1,this.userData={},this.pivot=null}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(e){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(e),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(e){return this.quaternion.premultiply(e),this}setRotationFromAxisAngle(e,t){this.quaternion.setFromAxisAngle(e,t)}setRotationFromEuler(e){this.quaternion.setFromEuler(e,!0)}setRotationFromMatrix(e){this.quaternion.setFromRotationMatrix(e)}setRotationFromQuaternion(e){this.quaternion.copy(e)}rotateOnAxis(e,t){return xr.setFromAxisAngle(e,t),this.quaternion.multiply(xr),this}rotateOnWorldAxis(e,t){return xr.setFromAxisAngle(e,t),this.quaternion.premultiply(xr),this}rotateX(e){return this.rotateOnAxis(jd,e)}rotateY(e){return this.rotateOnAxis(Kd,e)}rotateZ(e){return this.rotateOnAxis(Yd,e)}translateOnAxis(e,t){return Xd.copy(e).applyQuaternion(this.quaternion),this.position.add(Xd.multiplyScalar(t)),this}translateX(e){return this.translateOnAxis(jd,e)}translateY(e){return this.translateOnAxis(Kd,e)}translateZ(e){return this.translateOnAxis(Yd,e)}localToWorld(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(this.matrixWorld)}worldToLocal(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(Ki.copy(this.matrixWorld).invert())}lookAt(e,t,n){e.isVector3?Go.copy(e):Go.set(e,t,n);let i=this.parent;this.updateWorldMatrix(!0,!1),ya.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?Ki.lookAt(ya,Go,this.up):Ki.lookAt(Go,ya,this.up),this.quaternion.setFromRotationMatrix(Ki),i&&(Ki.extractRotation(i.matrixWorld),xr.setFromRotationMatrix(Ki),this.quaternion.premultiply(xr.invert()))}add(e){if(arguments.length>1){for(let t=0;t<arguments.length;t++)this.add(arguments[t]);return this}return e===this?(_t("Object3D.add: object can't be added as a child of itself.",e),this):(e&&e.isObject3D?(e.removeFromParent(),e.parent=this,this.children.push(e),e.dispatchEvent(Jd),br.child=e,this.dispatchEvent(br),br.child=null):_t("Object3D.add: object not an instance of THREE.Object3D.",e),this)}remove(e){if(arguments.length>1){for(let n=0;n<arguments.length;n++)this.remove(arguments[n]);return this}let t=this.children.indexOf(e);return t!==-1&&(e.parent=null,this.children.splice(t,1),e.dispatchEvent(a0),_h.child=e,this.dispatchEvent(_h),_h.child=null),this}removeFromParent(){let e=this.parent;return e!==null&&e.remove(this),this}clear(){return this.remove(...this.children)}attach(e){return this.updateWorldMatrix(!0,!1),Ki.copy(this.matrixWorld).invert(),e.parent!==null&&(e.parent.updateWorldMatrix(!0,!1),Ki.multiply(e.parent.matrixWorld)),e.applyMatrix4(Ki),e.removeFromParent(),e.parent=this,this.children.push(e),e.updateWorldMatrix(!1,!0),e.dispatchEvent(Jd),br.child=e,this.dispatchEvent(br),br.child=null,this}getObjectById(e){return this.getObjectByProperty("id",e)}getObjectByName(e){return this.getObjectByProperty("name",e)}getObjectByProperty(e,t){if(this[e]===t)return this;for(let n=0,i=this.children.length;n<i;n++){let a=this.children[n].getObjectByProperty(e,t);if(a!==void 0)return a}}getObjectsByProperty(e,t,n=[]){this[e]===t&&n.push(this);let i=this.children;for(let r=0,a=i.length;r<a;r++)i[r].getObjectsByProperty(e,t,n);return n}getWorldPosition(e){return this.updateWorldMatrix(!0,!1),e.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(ya,e,s0),e}getWorldScale(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(ya,r0,e),e}getWorldDirection(e){this.updateWorldMatrix(!0,!1);let t=this.matrixWorld.elements;return e.set(t[8],t[9],t[10]).normalize()}raycast(){}intersectsFrustum(){}traverse(e){e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverse(e)}traverseVisible(e){if(this.visible===!1)return;e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverseVisible(e)}traverseAncestors(e){let t=this.parent;t!==null&&(e(t),t.traverseAncestors(e))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale);let e=this.pivot;if(e!==null){let t=e.x,n=e.y,i=e.z,r=this.matrix.elements;r[12]+=t-r[0]*t-r[4]*n-r[8]*i,r[13]+=n-r[1]*t-r[5]*n-r[9]*i,r[14]+=i-r[2]*t-r[6]*n-r[10]*i}this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(e){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||e)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,e=!0);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].updateMatrixWorld(e)}updateWorldMatrix(e,t,n=!1){let i=this.parent;if(e===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||n)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,n=!0),t===!0){let r=this.children;for(let a=0,o=r.length;a<o;a++)r[a].updateWorldMatrix(!1,!0,n)}}toJSON(e){let t=e===void 0||typeof e=="string",n={};t&&(e={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},n.metadata={version:4.7,type:"Object",generator:"Object3D.toJSON"});let i={};i.uuid=this.uuid,i.type=this.type,i.name=this.name,i.castShadow=this.castShadow,i.receiveShadow=this.receiveShadow,i.visible=this.visible,i.frustumCulled=this.frustumCulled,i.renderOrder=this.renderOrder,i.static=this.static,i.matrixAutoUpdate=this.matrixAutoUpdate,Object.keys(this.userData).length>0&&(i.userData=this.userData),i.layers=this.layers.mask,i.matrix=this.matrix.toArray(),i.up=this.up.toArray(),this.pivot!==null&&(i.pivot=this.pivot.toArray()),this.morphTargetDictionary!==void 0&&(i.morphTargetDictionary=Object.assign({},this.morphTargetDictionary)),this.morphTargetInfluences!==void 0&&(i.morphTargetInfluences=this.morphTargetInfluences.slice()),this.isInstancedMesh&&(i.type="InstancedMesh",i.count=this.count,i.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(i.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(i.type="BatchedMesh",i.perObjectFrustumCulled=this.perObjectFrustumCulled,i.sortObjects=this.sortObjects,i.drawRanges=this._drawRanges,i.reservedRanges=this._reservedRanges,i.geometryInfo=this._geometryInfo.map(o=>({...o,boundingBox:o.boundingBox?o.boundingBox.toJSON():void 0,boundingSphere:o.boundingSphere?o.boundingSphere.toJSON():void 0})),i.instanceInfo=this._instanceInfo.map(o=>({...o})),i.availableInstanceIds=this._availableInstanceIds.slice(),i.availableGeometryIds=this._availableGeometryIds.slice(),i.nextIndexStart=this._nextIndexStart,i.nextVertexStart=this._nextVertexStart,i.geometryCount=this._geometryCount,i.maxInstanceCount=this._maxInstanceCount,i.maxVertexCount=this._maxVertexCount,i.maxIndexCount=this._maxIndexCount,i.geometryInitialized=this._geometryInitialized,i.matricesTexture=this._matricesTexture.toJSON(e),i.indirectTexture=this._indirectTexture.toJSON(e),this._colorsTexture!==null&&(i.colorsTexture=this._colorsTexture.toJSON(e)),this.boundingSphere!==null&&(i.boundingSphere=this.boundingSphere.toJSON()),this.boundingBox!==null&&(i.boundingBox=this.boundingBox.toJSON()));function r(o,l){return o[l.uuid]===void 0&&(o[l.uuid]=l.toJSON(e)),l.uuid}if(this.isScene)this.background&&(this.background.isColor?i.background=this.background.toJSON():this.background.isTexture&&(i.background=this.background.toJSON(e).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(i.environment=this.environment.toJSON(e).uuid);else if(this.isMesh||this.isLine||this.isPoints){i.geometry=r(e.geometries,this.geometry);let o=this.geometry.parameters;if(o!==void 0&&o.shapes!==void 0){let l=o.shapes;if(Array.isArray(l))for(let c=0,h=l.length;c<h;c++){let d=l[c];r(e.shapes,d)}else r(e.shapes,l)}}if(this.isSkinnedMesh&&(i.bindMode=this.bindMode,i.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(e.skeletons,this.skeleton),i.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let o=[];for(let l=0,c=this.material.length;l<c;l++)o.push(r(e.materials,this.material[l]));i.material=o}else i.material=r(e.materials,this.material);if(this.children.length>0){i.children=[];for(let o=0;o<this.children.length;o++)i.children.push(this.children[o].toJSON(e).object)}if(this.animations.length>0){i.animations=[];for(let o=0;o<this.animations.length;o++){let l=this.animations[o];i.animations.push(r(e.animations,l))}}if(t){let o=a(e.geometries),l=a(e.materials),c=a(e.textures),h=a(e.images),d=a(e.shapes),u=a(e.skeletons),p=a(e.animations),x=a(e.nodes);o.length>0&&(n.geometries=o),l.length>0&&(n.materials=l),c.length>0&&(n.textures=c),h.length>0&&(n.images=h),d.length>0&&(n.shapes=d),u.length>0&&(n.skeletons=u),p.length>0&&(n.animations=p),x.length>0&&(n.nodes=x)}return n.object=i,n;function a(o){let l=[];for(let c in o){let h=o[c];delete h.metadata,l.push(h)}return l}}clone(e){return new this.constructor().copy(this,e)}copy(e,t=!0){if(this.name=e.name,this.up.copy(e.up),this.position.copy(e.position),this.rotation.order=e.rotation.order,this.quaternion.copy(e.quaternion),this.scale.copy(e.scale),this.pivot=e.pivot!==null?e.pivot.clone():null,this.matrix.copy(e.matrix),this.matrixWorld.copy(e.matrixWorld),this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrixWorldAutoUpdate=e.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=e.matrixWorldNeedsUpdate,this.layers.mask=e.layers.mask,this.visible=e.visible,this.castShadow=e.castShadow,this.receiveShadow=e.receiveShadow,this.frustumCulled=e.frustumCulled,this.renderOrder=e.renderOrder,this.static=e.static,this.animations=e.animations.slice(),this.userData=JSON.parse(JSON.stringify(e.userData)),t===!0)for(let n=0;n<e.children.length;n++){let i=e.children[n];this.add(i.clone())}return this}dispose(){this.dispatchEvent({type:"dispose"})}};Yt.DEFAULT_UP=new U(0,1,0);Yt.DEFAULT_MATRIX_AUTO_UPDATE=!0;Yt.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var Et=class extends Yt{constructor(){super(),this.isGroup=!0,this.type="Group"}},o0={type:"move"},zr=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new Et,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new Et,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new U,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new U),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new Et,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new U,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new U,this._grip.eventsEnabled=!1),this._grip}dispatchEvent(e){return this._targetRay!==null&&this._targetRay.dispatchEvent(e),this._grip!==null&&this._grip.dispatchEvent(e),this._hand!==null&&this._hand.dispatchEvent(e),this}connect(e){if(e&&e.hand){let t=this._hand;if(t)for(let n of e.hand.values())this._getHandJoint(t,n)}return this.dispatchEvent({type:"connected",data:e}),this}disconnect(e){return this.dispatchEvent({type:"disconnected",data:e}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(e,t,n){let i=null,r=null,a=null,o=this._targetRay,l=this._grip,c=this._hand;if(e&&t.session.visibilityState!=="visible-blurred"){if(c&&e.hand){a=!0;for(let M of e.hand.values()){let m=t.getJointPose(M,n),f=this._getHandJoint(c,M);m!==null&&(f.matrix.fromArray(m.transform.matrix),f.matrix.decompose(f.position,f.rotation,f.scale),f.matrixWorldNeedsUpdate=!0,f.jointRadius=m.radius),f.visible=m!==null}let h=c.joints["index-finger-tip"],d=c.joints["thumb-tip"],u=h.position.distanceTo(d.position),p=.02,x=.005;c.inputState.pinching&&u>p+x?(c.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:e.handedness,target:this})):!c.inputState.pinching&&u<=p-x&&(c.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:e.handedness,target:this}))}else l!==null&&e.gripSpace&&(r=t.getPose(e.gripSpace,n),r!==null&&(l.matrix.fromArray(r.transform.matrix),l.matrix.decompose(l.position,l.rotation,l.scale),l.matrixWorldNeedsUpdate=!0,r.linearVelocity?(l.hasLinearVelocity=!0,l.linearVelocity.copy(r.linearVelocity)):l.hasLinearVelocity=!1,r.angularVelocity?(l.hasAngularVelocity=!0,l.angularVelocity.copy(r.angularVelocity)):l.hasAngularVelocity=!1,l.eventsEnabled&&l.dispatchEvent({type:"gripUpdated",data:e,target:this})));o!==null&&(i=t.getPose(e.targetRaySpace,n),i===null&&r!==null&&(i=r),i!==null&&(o.matrix.fromArray(i.transform.matrix),o.matrix.decompose(o.position,o.rotation,o.scale),o.matrixWorldNeedsUpdate=!0,i.linearVelocity?(o.hasLinearVelocity=!0,o.linearVelocity.copy(i.linearVelocity)):o.hasLinearVelocity=!1,i.angularVelocity?(o.hasAngularVelocity=!0,o.angularVelocity.copy(i.angularVelocity)):o.hasAngularVelocity=!1,this.dispatchEvent(o0)))}return o!==null&&(o.visible=i!==null),l!==null&&(l.visible=r!==null),c!==null&&(c.visible=a!==null),this}_getHandJoint(e,t){if(e.joints[t.jointName]===void 0){let n=new Et;n.matrixAutoUpdate=!1,n.visible=!1,e.joints[t.jointName]=n,e.add(n)}return e.joints[t.jointName]}},mp={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},ms={h:0,s:0,l:0},Vo={h:0,s:0,l:0};function Mh(s,e,t){return t<0&&(t+=1),t>1&&(t-=1),t<1/6?s+(e-s)*6*t:t<1/2?e:t<2/3?s+(e-s)*6*(2/3-t):s}var Le=class{constructor(e,t,n){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(e,t,n)}set(e,t,n){if(t===void 0&&n===void 0){let i=e;i&&i.isColor?this.copy(i):typeof i=="number"?this.setHex(i):typeof i=="string"&&this.setStyle(i)}else this.setRGB(e,t,n);return this}setScalar(e){return this.r=e,this.g=e,this.b=e,this}setHex(e,t=an){return e=Math.floor(e),this.r=(e>>16&255)/255,this.g=(e>>8&255)/255,this.b=(e&255)/255,Dt.colorSpaceToWorking(this,t),this}setRGB(e,t,n,i=Dt.workingColorSpace){return this.r=e,this.g=t,this.b=n,Dt.colorSpaceToWorking(this,i),this}setHSL(e,t,n,i=Dt.workingColorSpace){if(e=gu(e,1),t=Lt(t,0,1),n=Lt(n,0,1),t===0)this.r=this.g=this.b=n;else{let r=n<=.5?n*(1+t):n+t-n*t,a=2*n-r;this.r=Mh(a,r,e+1/3),this.g=Mh(a,r,e),this.b=Mh(a,r,e-1/3)}return Dt.colorSpaceToWorking(this,i),this}setStyle(e,t=an){function n(r){r!==void 0&&parseFloat(r)<1&&dt("Color: Alpha component of "+e+" will be ignored.")}let i;if(i=/^(\w+)\(([^\)]*)\)/.exec(e)){let r,a=i[1],o=i[2];switch(a){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,t);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,t);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(o))return n(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,t);break;default:dt("Color: Unknown color model "+e)}}else if(i=/^\#([A-Fa-f\d]+)$/.exec(e)){let r=i[1],a=r.length;if(a===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,t);if(a===6)return this.setHex(parseInt(r,16),t);dt("Color: Invalid hex color "+e)}else if(e&&e.length>0)return this.setColorName(e,t);return this}setColorName(e,t=an){let n=mp[e.toLowerCase()];return n!==void 0?this.setHex(n,t):dt("Color: Unknown color "+e),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(e){return this.r=e.r,this.g=e.g,this.b=e.b,this}copySRGBToLinear(e){return this.r=es(e.r),this.g=es(e.g),this.b=es(e.b),this}copyLinearToSRGB(e){return this.r=Cr(e.r),this.g=Cr(e.g),this.b=Cr(e.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(e=an){return Dt.workingToColorSpace(Nn.copy(this),e),Math.round(Lt(Nn.r*255,0,255))*65536+Math.round(Lt(Nn.g*255,0,255))*256+Math.round(Lt(Nn.b*255,0,255))}getHexString(e=an){return("000000"+this.getHex(e).toString(16)).slice(-6)}getHSL(e,t=Dt.workingColorSpace){Dt.workingToColorSpace(Nn.copy(this),t);let n=Nn.r,i=Nn.g,r=Nn.b,a=Math.max(n,i,r),o=Math.min(n,i,r),l,c,h=(o+a)/2;if(o===a)l=0,c=0;else{let d=a-o;switch(c=h<=.5?d/(a+o):d/(2-a-o),a){case n:l=(i-r)/d+(i<r?6:0);break;case i:l=(r-n)/d+2;break;case r:l=(n-i)/d+4;break}l/=6}return e.h=l,e.s=c,e.l=h,e}getRGB(e,t=Dt.workingColorSpace){return Dt.workingToColorSpace(Nn.copy(this),t),e.r=Nn.r,e.g=Nn.g,e.b=Nn.b,e}getStyle(e=an){Dt.workingToColorSpace(Nn.copy(this),e);let t=Nn.r,n=Nn.g,i=Nn.b;return e!==an?`color(${e} ${t.toFixed(3)} ${n.toFixed(3)} ${i.toFixed(3)})`:`rgb(${Math.round(t*255)},${Math.round(n*255)},${Math.round(i*255)})`}offsetHSL(e,t,n){return this.getHSL(ms),this.setHSL(ms.h+e,ms.s+t,ms.l+n)}add(e){return this.r+=e.r,this.g+=e.g,this.b+=e.b,this}addColors(e,t){return this.r=e.r+t.r,this.g=e.g+t.g,this.b=e.b+t.b,this}addScalar(e){return this.r+=e,this.g+=e,this.b+=e,this}sub(e){return this.r=Math.max(0,this.r-e.r),this.g=Math.max(0,this.g-e.g),this.b=Math.max(0,this.b-e.b),this}multiply(e){return this.r*=e.r,this.g*=e.g,this.b*=e.b,this}multiplyScalar(e){return this.r*=e,this.g*=e,this.b*=e,this}lerp(e,t){return this.r+=(e.r-this.r)*t,this.g+=(e.g-this.g)*t,this.b+=(e.b-this.b)*t,this}lerpColors(e,t,n){return this.r=e.r+(t.r-e.r)*n,this.g=e.g+(t.g-e.g)*n,this.b=e.b+(t.b-e.b)*n,this}lerpHSL(e,t){this.getHSL(ms),e.getHSL(Vo);let n=Ca(ms.h,Vo.h,t),i=Ca(ms.s,Vo.s,t),r=Ca(ms.l,Vo.l,t);return this.setHSL(n,i,r),this}setFromVector3(e){return this.r=e.x,this.g=e.y,this.b=e.z,this}applyMatrix3(e){let t=this.r,n=this.g,i=this.b,r=e.elements;return this.r=r[0]*t+r[3]*n+r[6]*i,this.g=r[1]*t+r[4]*n+r[7]*i,this.b=r[2]*t+r[5]*n+r[8]*i,this}equals(e){return e.r===this.r&&e.g===this.g&&e.b===this.b}fromArray(e,t=0){return this.r=e[t],this.g=e[t+1],this.b=e[t+2],this}toArray(e=[],t=0){return e[t]=this.r,e[t+1]=this.g,e[t+2]=this.b,e}fromBufferAttribute(e,t){return this.r=e.getX(t),this.g=e.getY(t),this.b=e.getZ(t),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},Nn=new Le;Le.NAMES=mp;var Ua=class s{constructor(e,t=25e-5){this.isFogExp2=!0,this.name="",this.color=new Le(e),this.density=t}clone(){return new s(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var Ys=class extends Yt{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new Vn,this.environmentIntensity=1,this.environmentRotation=new Vn,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(e,t){return super.copy(e,t),e.background!==null&&(this.background=e.background.clone()),e.environment!==null&&(this.environment=e.environment.clone()),e.fog!==null&&(this.fog=e.fog.clone()),this.backgroundBlurriness=e.backgroundBlurriness,this.backgroundIntensity=e.backgroundIntensity,this.backgroundRotation.copy(e.backgroundRotation),this.environmentIntensity=e.environmentIntensity,this.environmentRotation.copy(e.environmentRotation),e.overrideMaterial!==null&&(this.overrideMaterial=e.overrideMaterial.clone()),this.matrixAutoUpdate=e.matrixAutoUpdate,this}toJSON(e){let t=super.toJSON(e);return this.fog!==null&&(t.object.fog=this.fog.toJSON()),t.object.backgroundBlurriness=this.backgroundBlurriness,t.object.backgroundIntensity=this.backgroundIntensity,t.object.backgroundRotation=this.backgroundRotation.toArray(),t.object.environmentIntensity=this.environmentIntensity,t.object.environmentRotation=this.environmentRotation.toArray(),t}},mi=new U,Yi=new U,wh=new U,Ji=new U,vr=new U,yr=new U,Zd=new U,Sh=new U,Eh=new U,Th=new U,Ah=new Ot,Rh=new Ot,Ch=new Ot,ys=class s{constructor(e=new U,t=new U,n=new U){this.a=e,this.b=t,this.c=n}static getNormal(e,t,n,i){i.subVectors(n,t),mi.subVectors(e,t),i.cross(mi);let r=i.lengthSq();return r>0?i.multiplyScalar(1/Math.sqrt(r)):i.set(0,0,0)}static getBarycoord(e,t,n,i,r){mi.subVectors(i,t),Yi.subVectors(n,t),wh.subVectors(e,t);let a=mi.dot(mi),o=mi.dot(Yi),l=mi.dot(wh),c=Yi.dot(Yi),h=Yi.dot(wh),d=a*c-o*o;if(d===0)return r.set(0,0,0),null;let u=1/d,p=(c*l-o*h)*u,x=(a*h-o*l)*u;return r.set(1-p-x,x,p)}static containsPoint(e,t,n,i){return this.getBarycoord(e,t,n,i,Ji)===null?!1:Ji.x>=0&&Ji.y>=0&&Ji.x+Ji.y<=1}static getInterpolation(e,t,n,i,r,a,o,l){return this.getBarycoord(e,t,n,i,Ji)===null?(l.x=0,l.y=0,"z"in l&&(l.z=0),"w"in l&&(l.w=0),null):(l.setScalar(0),l.addScaledVector(r,Ji.x),l.addScaledVector(a,Ji.y),l.addScaledVector(o,Ji.z),l)}static getInterpolatedAttribute(e,t,n,i,r,a){return Ah.setScalar(0),Rh.setScalar(0),Ch.setScalar(0),Ah.fromBufferAttribute(e,t),Rh.fromBufferAttribute(e,n),Ch.fromBufferAttribute(e,i),a.setScalar(0),a.addScaledVector(Ah,r.x),a.addScaledVector(Rh,r.y),a.addScaledVector(Ch,r.z),a}static isFrontFacing(e,t,n,i){return mi.subVectors(n,t),Yi.subVectors(e,t),mi.cross(Yi).dot(i)<0}set(e,t,n){return this.a.copy(e),this.b.copy(t),this.c.copy(n),this}setFromPointsAndIndices(e,t,n,i){return this.a.copy(e[t]),this.b.copy(e[n]),this.c.copy(e[i]),this}setFromAttributeAndIndices(e,t,n,i){return this.a.fromBufferAttribute(e,t),this.b.fromBufferAttribute(e,n),this.c.fromBufferAttribute(e,i),this}clone(){return new this.constructor().copy(this)}copy(e){return this.a.copy(e.a),this.b.copy(e.b),this.c.copy(e.c),this}getArea(){return mi.subVectors(this.c,this.b),Yi.subVectors(this.a,this.b),mi.cross(Yi).length()*.5}getMidpoint(e){return e.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(e){return s.getNormal(this.a,this.b,this.c,e)}getPlane(e){return e.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(e,t){return s.getBarycoord(e,this.a,this.b,this.c,t)}getInterpolation(e,t,n,i,r){return s.getInterpolation(e,this.a,this.b,this.c,t,n,i,r)}containsPoint(e){return s.containsPoint(e,this.a,this.b,this.c)}isFrontFacing(e){return s.isFrontFacing(this.a,this.b,this.c,e)}intersectsBox(e){return e.intersectsTriangle(this)}closestPointToPoint(e,t){let n=this.a,i=this.b,r=this.c,a,o;vr.subVectors(i,n),yr.subVectors(r,n),Sh.subVectors(e,n);let l=vr.dot(Sh),c=yr.dot(Sh);if(l<=0&&c<=0)return t.copy(n);Eh.subVectors(e,i);let h=vr.dot(Eh),d=yr.dot(Eh);if(h>=0&&d<=h)return t.copy(i);let u=l*d-h*c;if(u<=0&&l>=0&&h<=0)return a=l/(l-h),t.copy(n).addScaledVector(vr,a);Th.subVectors(e,r);let p=vr.dot(Th),x=yr.dot(Th);if(x>=0&&p<=x)return t.copy(r);let M=p*c-l*x;if(M<=0&&c>=0&&x<=0)return o=c/(c-x),t.copy(n).addScaledVector(yr,o);let m=h*x-p*d;if(m<=0&&d-h>=0&&p-x>=0)return Zd.subVectors(r,i),o=(d-h)/(d-h+(p-x)),t.copy(i).addScaledVector(Zd,o);let f=1/(m+M+u);return a=M*f,o=u*f,t.copy(n).addScaledVector(vr,a).addScaledVector(yr,o)}equals(e){return e.a.equals(this.a)&&e.b.equals(this.b)&&e.c.equals(this.c)}},un=class{constructor(e=new U(1/0,1/0,1/0),t=new U(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=e,this.max=t}set(e,t){return this.min.copy(e),this.max.copy(t),this}setFromArray(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t+=3)this.expandByPoint(gi.fromArray(e,t));return this}setFromBufferAttribute(e){this.makeEmpty();for(let t=0,n=e.count;t<n;t++)this.expandByPoint(gi.fromBufferAttribute(e,t));return this}setFromPoints(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t++)this.expandByPoint(e[t]);return this}setFromCenterAndSize(e,t){let n=gi.copy(t).multiplyScalar(.5);return this.min.copy(e).sub(n),this.max.copy(e).add(n),this}setFromObject(e,t=!1){return this.makeEmpty(),this.expandByObject(e,t)}clone(){return new this.constructor().copy(this)}copy(e){return this.min.copy(e.min),this.max.copy(e.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(e){return this.isEmpty()?e.set(0,0,0):e.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(e){return this.isEmpty()?e.set(0,0,0):e.subVectors(this.max,this.min)}expandByPoint(e){return this.min.min(e),this.max.max(e),this}expandByVector(e){return this.min.sub(e),this.max.add(e),this}expandByScalar(e){return this.min.addScalar(-e),this.max.addScalar(e),this}expandByObject(e,t=!1){e.updateWorldMatrix(!1,!1);let n=e.geometry;if(n!==void 0){let r=n.getAttribute("position");if(t===!0&&r!==void 0&&e.isInstancedMesh!==!0)for(let a=0,o=r.count;a<o;a++)e.isMesh===!0?e.getVertexPosition(a,gi):gi.fromBufferAttribute(r,a),gi.applyMatrix4(e.matrixWorld),this.expandByPoint(gi);else e.boundingBox!==void 0?(e.boundingBox===null&&e.computeBoundingBox(),Wo.copy(e.boundingBox)):(n.boundingBox===null&&n.computeBoundingBox(),Wo.copy(n.boundingBox)),Wo.applyMatrix4(e.matrixWorld),this.union(Wo)}let i=e.children;for(let r=0,a=i.length;r<a;r++)this.expandByObject(i[r],t);return this}containsPoint(e){return e.x>=this.min.x&&e.x<=this.max.x&&e.y>=this.min.y&&e.y<=this.max.y&&e.z>=this.min.z&&e.z<=this.max.z}containsBox(e){return this.min.x<=e.min.x&&e.max.x<=this.max.x&&this.min.y<=e.min.y&&e.max.y<=this.max.y&&this.min.z<=e.min.z&&e.max.z<=this.max.z}getParameter(e,t){return t.set((e.x-this.min.x)/(this.max.x-this.min.x),(e.y-this.min.y)/(this.max.y-this.min.y),(e.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(e){return e.max.x>=this.min.x&&e.min.x<=this.max.x&&e.max.y>=this.min.y&&e.min.y<=this.max.y&&e.max.z>=this.min.z&&e.min.z<=this.max.z}intersectsSphere(e){return this.clampPoint(e.center,gi),gi.distanceToSquared(e.center)<=e.radius*e.radius}intersectsPlane(e){let t,n;return e.normal.x>0?(t=e.normal.x*this.min.x,n=e.normal.x*this.max.x):(t=e.normal.x*this.max.x,n=e.normal.x*this.min.x),e.normal.y>0?(t+=e.normal.y*this.min.y,n+=e.normal.y*this.max.y):(t+=e.normal.y*this.max.y,n+=e.normal.y*this.min.y),e.normal.z>0?(t+=e.normal.z*this.min.z,n+=e.normal.z*this.max.z):(t+=e.normal.z*this.max.z,n+=e.normal.z*this.min.z),t<=-e.constant&&n>=-e.constant}intersectsTriangle(e){if(this.isEmpty())return!1;this.getCenter(_a),qo.subVectors(this.max,_a),_r.subVectors(e.a,_a),Mr.subVectors(e.b,_a),wr.subVectors(e.c,_a),gs.subVectors(Mr,_r),xs.subVectors(wr,Mr),ks.subVectors(_r,wr);let t=[0,-gs.z,gs.y,0,-xs.z,xs.y,0,-ks.z,ks.y,gs.z,0,-gs.x,xs.z,0,-xs.x,ks.z,0,-ks.x,-gs.y,gs.x,0,-xs.y,xs.x,0,-ks.y,ks.x,0];return!Ph(t,_r,Mr,wr,qo)||(t=[1,0,0,0,1,0,0,0,1],!Ph(t,_r,Mr,wr,qo))?!1:(Xo.crossVectors(gs,xs),t=[Xo.x,Xo.y,Xo.z],Ph(t,_r,Mr,wr,qo))}clampPoint(e,t){return t.copy(e).clamp(this.min,this.max)}distanceToPoint(e){return this.clampPoint(e,gi).distanceTo(e)}getBoundingSphere(e){return this.isEmpty()?e.makeEmpty():(this.getCenter(e.center),e.radius=this.getSize(gi).length()*.5),e}intersect(e){return this.min.max(e.min),this.max.min(e.max),this.isEmpty()&&this.makeEmpty(),this}union(e){return this.min.min(e.min),this.max.max(e.max),this}applyMatrix4(e){return this.isEmpty()?this:(Zi[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(e),Zi[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(e),Zi[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(e),Zi[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(e),Zi[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(e),Zi[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(e),Zi[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(e),Zi[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(e),this.setFromPoints(Zi),this)}translate(e){return this.min.add(e),this.max.add(e),this}equals(e){return e.min.equals(this.min)&&e.max.equals(this.max)}toJSON(){return{min:this.min.toArray(),max:this.max.toArray()}}fromJSON(e){return this.min.fromArray(e.min),this.max.fromArray(e.max),this}},Zi=[new U,new U,new U,new U,new U,new U,new U,new U],gi=new U,Wo=new un,_r=new U,Mr=new U,wr=new U,gs=new U,xs=new U,ks=new U,_a=new U,qo=new U,Xo=new U,Bs=new U;function Ph(s,e,t,n,i){for(let r=0,a=s.length-3;r<=a;r+=3){Bs.fromArray(s,r);let o=i.x*Math.abs(Bs.x)+i.y*Math.abs(Bs.y)+i.z*Math.abs(Bs.z),l=e.dot(Bs),c=t.dot(Bs),h=n.dot(Bs);if(Math.max(-Math.max(l,c,h),Math.min(l,c,h))>o)return!1}return!0}var mn=new U,jo=new Ge,l0=0,Pt=class extends ai{constructor(e,t,n=!1){if(super(),Array.isArray(e))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,Object.defineProperty(this,"id",{value:l0++}),this.name="",this.array=e,this.itemSize=t,this.count=e!==void 0?e.length/t:0,this.normalized=n,this.usage=pu,this.updateRanges=[],this.gpuType=ei,this.version=0}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.name=e.name,this.array=new e.array.constructor(e.array),this.itemSize=e.itemSize,this.count=e.count,this.normalized=e.normalized,this.usage=e.usage,this.gpuType=e.gpuType,this}copyAt(e,t,n){e*=this.itemSize,n*=t.itemSize;for(let i=0,r=this.itemSize;i<r;i++)this.array[e+i]=t.array[n+i];return this}copyArray(e){return this.array.set(e),this}applyMatrix3(e){if(this.itemSize===2)for(let t=0,n=this.count;t<n;t++)jo.fromBufferAttribute(this,t),jo.applyMatrix3(e),this.setXY(t,jo.x,jo.y);else if(this.itemSize===3)for(let t=0,n=this.count;t<n;t++)mn.fromBufferAttribute(this,t),mn.applyMatrix3(e),this.setXYZ(t,mn.x,mn.y,mn.z);return this}applyMatrix4(e){for(let t=0,n=this.count;t<n;t++)mn.fromBufferAttribute(this,t),mn.applyMatrix4(e),this.setXYZ(t,mn.x,mn.y,mn.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)mn.fromBufferAttribute(this,t),mn.applyNormalMatrix(e),this.setXYZ(t,mn.x,mn.y,mn.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)mn.fromBufferAttribute(this,t),mn.transformDirection(e),this.setXYZ(t,mn.x,mn.y,mn.z);return this}set(e,t=0){return this.array.set(e,t),this}getComponent(e,t){let n=this.array[e*this.itemSize+t];return this.normalized&&(n=xi(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=$t(n,this.array)),this.array[e*this.itemSize+t]=n,this}getX(e){let t=this.array[e*this.itemSize];return this.normalized&&(t=xi(t,this.array)),t}setX(e,t){return this.normalized&&(t=$t(t,this.array)),this.array[e*this.itemSize]=t,this}getY(e){let t=this.array[e*this.itemSize+1];return this.normalized&&(t=xi(t,this.array)),t}setY(e,t){return this.normalized&&(t=$t(t,this.array)),this.array[e*this.itemSize+1]=t,this}getZ(e){let t=this.array[e*this.itemSize+2];return this.normalized&&(t=xi(t,this.array)),t}setZ(e,t){return this.normalized&&(t=$t(t,this.array)),this.array[e*this.itemSize+2]=t,this}getW(e){let t=this.array[e*this.itemSize+3];return this.normalized&&(t=xi(t,this.array)),t}setW(e,t){return this.normalized&&(t=$t(t,this.array)),this.array[e*this.itemSize+3]=t,this}setXY(e,t,n){return e*=this.itemSize,this.normalized&&(t=$t(t,this.array),n=$t(n,this.array)),this.array[e+0]=t,this.array[e+1]=n,this}setXYZ(e,t,n,i){return e*=this.itemSize,this.normalized&&(t=$t(t,this.array),n=$t(n,this.array),i=$t(i,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e*=this.itemSize,this.normalized&&(t=$t(t,this.array),n=$t(n,this.array),i=$t(i,this.array),r=$t(r,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this.array[e+3]=r,this}onUpload(e){return this.onUploadCallback=e,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let e={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return e.name=this.name,e.usage=this.usage,e.gpuType=this.gpuType,e}dispose(){this.dispatchEvent({type:"dispose"})}};var za=class extends Pt{constructor(e,t,n){super(new Uint16Array(e),t,n)}};var Oa=class extends Pt{constructor(e,t,n){super(new Uint32Array(e),t,n)}};var Mt=class extends Pt{constructor(e,t,n){super(new Float32Array(e),t,n)}},c0=new un,Ma=new U,Ih=new U,Un=class{constructor(e=new U,t=-1){this.isSphere=!0,this.center=e,this.radius=t}set(e,t){return this.center.copy(e),this.radius=t,this}setFromPoints(e,t){let n=this.center;t!==void 0?n.copy(t):c0.setFromPoints(e).getCenter(n);let i=0;for(let r=0,a=e.length;r<a;r++)i=Math.max(i,n.distanceToSquared(e[r]));return this.radius=Math.sqrt(i),this}copy(e){return this.center.copy(e.center),this.radius=e.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(e){return e.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(e){return e.distanceTo(this.center)-this.radius}intersectsSphere(e){let t=this.radius+e.radius;return e.center.distanceToSquared(this.center)<=t*t}intersectsBox(e){return e.intersectsSphere(this)}intersectsPlane(e){return Math.abs(e.distanceToPoint(this.center))<=this.radius}clampPoint(e,t){let n=this.center.distanceToSquared(e);return t.copy(e),n>this.radius*this.radius&&(t.sub(this.center).normalize(),t.multiplyScalar(this.radius).add(this.center)),t}getBoundingBox(e){return this.isEmpty()?(e.makeEmpty(),e):(e.set(this.center,this.center),e.expandByScalar(this.radius),e)}applyMatrix4(e){return this.center.applyMatrix4(e),this.radius=this.radius*e.getMaxScaleOnAxis(),this}translate(e){return this.center.add(e),this}expandByPoint(e){if(this.isEmpty())return this.center.copy(e),this.radius=0,this;Ma.subVectors(e,this.center);let t=Ma.lengthSq();if(t>this.radius*this.radius){let n=Math.sqrt(t),i=(n-this.radius)*.5;this.center.addScaledVector(Ma,i/n),this.radius+=i}return this}union(e){return e.isEmpty()?this:this.isEmpty()?(this.copy(e),this):(this.center.equals(e.center)===!0?this.radius=Math.max(this.radius,e.radius):(Ih.subVectors(e.center,this.center).setLength(e.radius),this.expandByPoint(Ma.copy(e.center).add(Ih)),this.expandByPoint(Ma.copy(e.center).sub(Ih))),this)}equals(e){return e.center.equals(this.center)&&e.radius===this.radius}clone(){return new this.constructor().copy(this)}toJSON(){return{radius:this.radius,center:this.center.toArray()}}fromJSON(e){return this.radius=e.radius,this.center.fromArray(e.center),this}},h0=0,si=new yt,Dh=new Yt,Sr=new U,Jn=new un,wa=new un,Tn=new U,At=class s extends ai{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:h0++}),this.uuid=vi(),this.name="",this.type="BufferGeometry",this.index=null,this.indirect=null,this.indirectOffset=0,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={},this._transformed=!1}getIndex(){return this.index}setIndex(e){return Array.isArray(e)?this.index=new(Fg(e)?Oa:za)(e,1):this.index=e,this}setIndirect(e,t=0){return this.indirect=e,this.indirectOffset=t,this}getIndirect(){return this.indirect}getAttribute(e){return this.attributes[e]}setAttribute(e,t){return this.attributes[e]=t,this}deleteAttribute(e){return delete this.attributes[e],this}hasAttribute(e){return this.attributes[e]!==void 0}addGroup(e,t,n=0){this.groups.push({start:e,count:t,materialIndex:n})}clearGroups(){this.groups=[]}setDrawRange(e,t){this.drawRange.start=e,this.drawRange.count=t}applyMatrix4(e){let t=this.attributes.position;t!==void 0&&(t.applyMatrix4(e),t.needsUpdate=!0);let n=this.attributes.normal;if(n!==void 0){let r=new Tt().getNormalMatrix(e);n.applyNormalMatrix(r),n.needsUpdate=!0}let i=this.attributes.tangent;return i!==void 0&&(i.transformDirection(e),i.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this._transformed=!0,this}applyQuaternion(e){return si.makeRotationFromQuaternion(e),this.applyMatrix4(si),this}rotateX(e){return si.makeRotationX(e),this.applyMatrix4(si),this}rotateY(e){return si.makeRotationY(e),this.applyMatrix4(si),this}rotateZ(e){return si.makeRotationZ(e),this.applyMatrix4(si),this}translate(e,t,n){return si.makeTranslation(e,t,n),this.applyMatrix4(si),this}scale(e,t,n){return si.makeScale(e,t,n),this.applyMatrix4(si),this}lookAt(e){return Dh.lookAt(e),Dh.updateMatrix(),this.applyMatrix4(Dh.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(Sr).negate(),this.translate(Sr.x,Sr.y,Sr.z),this}setFromPoints(e){let t=this.getAttribute("position");if(t===void 0){let n=[];for(let i=0,r=e.length;i<r;i++){let a=e[i];n.push(a.x,a.y,a.z||0)}this.setAttribute("position",new Mt(n,3))}else{let n=Math.min(e.length,t.count);for(let i=0;i<n;i++){let r=e[i];t.setXYZ(i,r.x,r.y,r.z||0)}e.length>t.count&&dt("BufferGeometry: Buffer size too small for points data. Use .dispose() and create a new geometry."),t.needsUpdate=!0}return this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new un);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){_t("BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new U(-1/0,-1/0,-1/0),new U(1/0,1/0,1/0));return}if(e!==void 0){if(this.boundingBox.setFromBufferAttribute(e),t)for(let n=0,i=t.length;n<i;n++){let r=t[n];Jn.setFromBufferAttribute(r),this.morphTargetsRelative?(Tn.addVectors(this.boundingBox.min,Jn.min),this.boundingBox.expandByPoint(Tn),Tn.addVectors(this.boundingBox.max,Jn.max),this.boundingBox.expandByPoint(Tn)):(this.boundingBox.expandByPoint(Jn.min),this.boundingBox.expandByPoint(Jn.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&_t('BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new Un);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){_t("BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new U,1/0);return}if(e){let n=this.boundingSphere.center;if(Jn.setFromBufferAttribute(e),t)for(let r=0,a=t.length;r<a;r++){let o=t[r];wa.setFromBufferAttribute(o),this.morphTargetsRelative?(Tn.addVectors(Jn.min,wa.min),Jn.expandByPoint(Tn),Tn.addVectors(Jn.max,wa.max),Jn.expandByPoint(Tn)):(Jn.expandByPoint(wa.min),Jn.expandByPoint(wa.max))}Jn.getCenter(n);let i=0;for(let r=0,a=e.count;r<a;r++)Tn.fromBufferAttribute(e,r),i=Math.max(i,n.distanceToSquared(Tn));if(t)for(let r=0,a=t.length;r<a;r++){let o=t[r],l=this.morphTargetsRelative;for(let c=0,h=o.count;c<h;c++)Tn.fromBufferAttribute(o,c),l&&(Sr.fromBufferAttribute(e,c),Tn.add(Sr)),i=Math.max(i,n.distanceToSquared(Tn))}this.boundingSphere.radius=Math.sqrt(i),isNaN(this.boundingSphere.radius)&&_t('BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let e=this.index,t=this.attributes;if(e===null||t.position===void 0||t.normal===void 0||t.uv===void 0){_t("BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let n=t.position,i=t.normal,r=t.uv,a=this.getAttribute("tangent");(a===void 0||a.count!==n.count)&&(a=new Pt(new Float32Array(4*n.count),4),this.setAttribute("tangent",a));let o=[],l=[];for(let y=0;y<n.count;y++)o[y]=new U,l[y]=new U;let c=new U,h=new U,d=new U,u=new Ge,p=new Ge,x=new Ge,M=new U,m=new U;function f(y,C,S){c.fromBufferAttribute(n,y),h.fromBufferAttribute(n,C),d.fromBufferAttribute(n,S),u.fromBufferAttribute(r,y),p.fromBufferAttribute(r,C),x.fromBufferAttribute(r,S),h.sub(c),d.sub(c),p.sub(u),x.sub(u);let D=1/(p.x*x.y-x.x*p.y);isFinite(D)&&(M.copy(h).multiplyScalar(x.y).addScaledVector(d,-p.y).multiplyScalar(D),m.copy(d).multiplyScalar(p.x).addScaledVector(h,-x.x).multiplyScalar(D),o[y].add(M),o[C].add(M),o[S].add(M),l[y].add(m),l[C].add(m),l[S].add(m))}let b=this.groups;b.length===0&&(b=[{start:0,count:e.count}]);for(let y=0,C=b.length;y<C;++y){let S=b[y],D=S.start,v=S.count;for(let I=D,P=D+v;I<P;I+=3)f(e.getX(I+0),e.getX(I+1),e.getX(I+2))}let E=new U,g=new U,_=new U,w=new U;function T(y){_.fromBufferAttribute(i,y),w.copy(_);let C=o[y];E.copy(C),E.sub(_.multiplyScalar(_.dot(C))).normalize(),g.crossVectors(w,C);let D=g.dot(l[y])<0?-1:1;a.setXYZW(y,E.x,E.y,E.z,D)}for(let y=0,C=b.length;y<C;++y){let S=b[y],D=S.start,v=S.count;for(let I=D,P=D+v;I<P;I+=3)T(e.getX(I+0)),T(e.getX(I+1)),T(e.getX(I+2))}this._transformed=!0}computeVertexNormals(){let e=this.index,t=this.getAttribute("position");if(t!==void 0){let n=this.getAttribute("normal");if(n===void 0||n.count!==t.count)n=new Pt(new Float32Array(t.count*3),3),this.setAttribute("normal",n);else for(let u=0,p=n.count;u<p;u++)n.setXYZ(u,0,0,0);let i=new U,r=new U,a=new U,o=new U,l=new U,c=new U,h=new U,d=new U;if(e)for(let u=0,p=e.count;u<p;u+=3){let x=e.getX(u+0),M=e.getX(u+1),m=e.getX(u+2);i.fromBufferAttribute(t,x),r.fromBufferAttribute(t,M),a.fromBufferAttribute(t,m),h.subVectors(a,r),d.subVectors(i,r),h.cross(d),o.fromBufferAttribute(n,x),l.fromBufferAttribute(n,M),c.fromBufferAttribute(n,m),o.add(h),l.add(h),c.add(h),n.setXYZ(x,o.x,o.y,o.z),n.setXYZ(M,l.x,l.y,l.z),n.setXYZ(m,c.x,c.y,c.z)}else for(let u=0,p=t.count;u<p;u+=3)i.fromBufferAttribute(t,u+0),r.fromBufferAttribute(t,u+1),a.fromBufferAttribute(t,u+2),h.subVectors(a,r),d.subVectors(i,r),h.cross(d),n.setXYZ(u+0,h.x,h.y,h.z),n.setXYZ(u+1,h.x,h.y,h.z),n.setXYZ(u+2,h.x,h.y,h.z);this.normalizeNormals(),n.needsUpdate=!0}}normalizeNormals(){let e=this.attributes.normal;for(let t=0,n=e.count;t<n;t++)Tn.fromBufferAttribute(e,t),Tn.normalize(),e.setXYZ(t,Tn.x,Tn.y,Tn.z)}toNonIndexed(){function e(o,l){let c=o.array,h=o.itemSize,d=o.normalized,u=new c.constructor(l.length*h),p=0,x=0;for(let M=0,m=l.length;M<m;M++){o.isInterleavedBufferAttribute?p=l[M]*o.data.stride+o.offset:p=l[M]*h;for(let f=0;f<h;f++)u[x++]=c[p++]}return new Pt(u,h,d)}if(this.index===null)return dt("BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let t=new s,n=this.index.array,i=this.attributes;for(let o in i){let l=i[o],c=e(l,n);t.setAttribute(o,c)}let r=this.morphAttributes;for(let o in r){let l=[],c=r[o];for(let h=0,d=c.length;h<d;h++){let u=c[h],p=e(u,n);l.push(p)}t.morphAttributes[o]=l}t.morphTargetsRelative=this.morphTargetsRelative;let a=this.groups;for(let o=0,l=a.length;o<l;o++){let c=a[o];t.addGroup(c.start,c.count,c.materialIndex)}return t}toJSON(){let e={metadata:{version:4.7,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(e.uuid=this.uuid,e.type=this.parameters!==void 0&&this._transformed===!0?"BufferGeometry":this.type,e.name=this.name,Object.keys(this.userData).length>0&&(e.userData=this.userData),this.parameters!==void 0&&this._transformed!==!0){let l=this.parameters;for(let c in l)l[c]!==void 0&&(e[c]=l[c]);return e}e.data={attributes:{}};let t=this.index;t!==null&&(e.data.index={type:t.array.constructor.name,array:Array.prototype.slice.call(t.array)});let n=this.attributes;for(let l in n){let c=n[l];e.data.attributes[l]=c.toJSON(e.data)}let i={},r=!1;for(let l in this.morphAttributes){let c=this.morphAttributes[l],h=[];for(let d=0,u=c.length;d<u;d++){let p=c[d];h.push(p.toJSON(e.data))}h.length>0&&(i[l]=h,r=!0)}r&&(e.data.morphAttributes=i,e.data.morphTargetsRelative=this.morphTargetsRelative);let a=this.groups;a.length>0&&(e.data.groups=JSON.parse(JSON.stringify(a)));let o=this.boundingSphere;return o!==null&&(e.data.boundingSphere=o.toJSON()),e}clone(){return new this.constructor().copy(this)}copy(e){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let t={};this.name=e.name;let n=e.index;n!==null&&this.setIndex(n.clone());let i=e.attributes;for(let c in i){let h=i[c];this.setAttribute(c,h.clone(t))}let r=e.morphAttributes;for(let c in r){let h=[],d=r[c];for(let u=0,p=d.length;u<p;u++)h.push(d[u].clone(t));this.morphAttributes[c]=h}this.morphTargetsRelative=e.morphTargetsRelative;let a=e.groups;for(let c=0,h=a.length;c<h;c++){let d=a[c];this.addGroup(d.start,d.count,d.materialIndex)}let o=e.boundingBox;o!==null&&(this.boundingBox=o.clone());let l=e.boundingSphere;return l!==null&&(this.boundingSphere=l.clone()),this.drawRange.start=e.drawRange.start,this.drawRange.count=e.drawRange.count,this.userData=e.userData,this._transformed=e._transformed,this}dispose(){this.dispatchEvent({type:"dispose"})}},Or=class{constructor(e,t){this.isInterleavedBuffer=!0,this.array=e,this.stride=t,this.count=e!==void 0?e.length/t:0,this.usage=pu,this.updateRanges=[],this.version=0,this.uuid=vi()}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.array=new e.array.constructor(e.array),this.count=e.count,this.stride=e.stride,this.usage=e.usage,this}copyAt(e,t,n){e*=this.stride,n*=t.stride;for(let i=0,r=this.stride;i<r;i++)this.array[e+i]=t.array[n+i];return this}set(e,t=0){return this.array.set(e,t),this}clone(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=vi()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=this.array.slice(0).buffer);let t=new this.array.constructor(e.arrayBuffers[this.array.buffer._uuid]),n=new this.constructor(t,this.stride);return n.setUsage(this.usage),n}onUpload(e){return this.onUploadCallback=e,this}toJSON(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=vi()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=Array.from(new Uint32Array(this.array.buffer)));let t={uuid:this.uuid,buffer:this.array.buffer._uuid,type:this.array.constructor.name,stride:this.stride};return t.usage=this.usage,t}},Bn=new U,kr=class s{constructor(e,t,n,i=!1){this.isInterleavedBufferAttribute=!0,this.name="",this.data=e,this.itemSize=t,this.offset=n,this.normalized=i}get count(){return this.data.count}get array(){return this.data.array}set needsUpdate(e){this.data.needsUpdate=e}applyMatrix4(e){for(let t=0,n=this.data.count;t<n;t++)Bn.fromBufferAttribute(this,t),Bn.applyMatrix4(e),this.setXYZ(t,Bn.x,Bn.y,Bn.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)Bn.fromBufferAttribute(this,t),Bn.applyNormalMatrix(e),this.setXYZ(t,Bn.x,Bn.y,Bn.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)Bn.fromBufferAttribute(this,t),Bn.transformDirection(e),this.setXYZ(t,Bn.x,Bn.y,Bn.z);return this}getComponent(e,t){let n=this.array[e*this.data.stride+this.offset+t];return this.normalized&&(n=xi(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=$t(n,this.array)),this.data.array[e*this.data.stride+this.offset+t]=n,this}setX(e,t){return this.normalized&&(t=$t(t,this.array)),this.data.array[e*this.data.stride+this.offset]=t,this}setY(e,t){return this.normalized&&(t=$t(t,this.array)),this.data.array[e*this.data.stride+this.offset+1]=t,this}setZ(e,t){return this.normalized&&(t=$t(t,this.array)),this.data.array[e*this.data.stride+this.offset+2]=t,this}setW(e,t){return this.normalized&&(t=$t(t,this.array)),this.data.array[e*this.data.stride+this.offset+3]=t,this}getX(e){let t=this.data.array[e*this.data.stride+this.offset];return this.normalized&&(t=xi(t,this.array)),t}getY(e){let t=this.data.array[e*this.data.stride+this.offset+1];return this.normalized&&(t=xi(t,this.array)),t}getZ(e){let t=this.data.array[e*this.data.stride+this.offset+2];return this.normalized&&(t=xi(t,this.array)),t}getW(e){let t=this.data.array[e*this.data.stride+this.offset+3];return this.normalized&&(t=xi(t,this.array)),t}setXY(e,t,n){return e=e*this.data.stride+this.offset,this.normalized&&(t=$t(t,this.array),n=$t(n,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this}setXYZ(e,t,n,i){return e=e*this.data.stride+this.offset,this.normalized&&(t=$t(t,this.array),n=$t(n,this.array),i=$t(i,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e=e*this.data.stride+this.offset,this.normalized&&(t=$t(t,this.array),n=$t(n,this.array),i=$t(i,this.array),r=$t(r,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this.data.array[e+3]=r,this}clone(e){if(e===void 0){Fa("InterleavedBufferAttribute.clone(): Cloning an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return new Pt(new this.array.constructor(t),this.itemSize,this.normalized)}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.clone(e)),new s(e.interleavedBuffers[this.data.uuid],this.itemSize,this.offset,this.normalized)}toJSON(e){if(e===void 0){Fa("InterleavedBufferAttribute.toJSON(): Serializing an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return{itemSize:this.itemSize,type:this.array.constructor.name,array:t,normalized:this.normalized}}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.toJSON(e)),{isInterleavedBufferAttribute:!0,itemSize:this.itemSize,data:this.data.uuid,offset:this.offset,normalized:this.normalized}}},Lh=new U,u0=new U,d0=new Tt,Zn=class{constructor(e=new U(1,0,0),t=0){this.isPlane=!0,this.normal=e,this.constant=t}set(e,t){return this.normal.copy(e),this.constant=t,this}setComponents(e,t,n,i){return this.normal.set(e,t,n),this.constant=i,this}setFromNormalAndCoplanarPoint(e,t){return this.normal.copy(e),this.constant=-t.dot(this.normal),this}setFromCoplanarPoints(e,t,n){let i=Lh.subVectors(n,t).cross(u0.subVectors(e,t)).normalize();return this.setFromNormalAndCoplanarPoint(i,e),this}copy(e){return this.normal.copy(e.normal),this.constant=e.constant,this}normalize(){let e=1/this.normal.length();return this.normal.multiplyScalar(e),this.constant*=e,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(e){return this.normal.dot(e)+this.constant}distanceToSphere(e){return this.distanceToPoint(e.center)-e.radius}projectPoint(e,t){return t.copy(e).addScaledVector(this.normal,-this.distanceToPoint(e))}intersectLine(e,t,n=!0){let i=e.delta(Lh),r=this.normal.dot(i);if(r===0)return this.distanceToPoint(e.start)===0?t.copy(e.start):null;let a=-(e.start.dot(this.normal)+this.constant)/r;return n===!0&&(a<0||a>1)?null:t.copy(e.start).addScaledVector(i,a)}intersectsLine(e){let t=this.distanceToPoint(e.start),n=this.distanceToPoint(e.end);return t<0&&n>0||n<0&&t>0}intersectsBox(e){return e.intersectsPlane(this)}intersectsSphere(e){return e.intersectsPlane(this)}coplanarPoint(e){return e.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(e,t){let n=t||d0.getNormalMatrix(e),i=this.coplanarPoint(Lh).applyMatrix4(e),r=this.normal.applyMatrix3(n).normalize();return this.constant=-i.dot(r),this}translate(e){return this.constant-=e.dot(this.normal),this}equals(e){return e.normal.equals(this.normal)&&e.constant===this.constant}clone(){return new this.constructor().copy(this)}toJSON(){return{normal:this.normal.toArray(),constant:this.constant}}fromJSON(e){return this.normal.fromArray(e.normal),this.constant=e.constant,this}},f0=0,Gn=class extends ai{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:f0++}),this.uuid=vi(),this.name="",this.type="Material",this.blending=$r,this.side=oi,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=su,this.blendDst=ru,this.blendEquation=nr,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new Le(0,0,0),this.blendAlpha=0,this.depthFunc=Pr,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=sp,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=fl,this.stencilZFail=fl,this.stencilZPass=fl,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.allowOverride=!0,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(e){this._alphaTest>0!=e>0&&this.version++,this._alphaTest=e}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(e){if(e!==void 0)for(let t in e){let n=e[t];if(n===void 0){dt(`Material: parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){dt(`Material: '${t}' is not a property of THREE.${this.type}.`);continue}i&&i.isColor?i.set(n):i&&i.isVector2&&n&&n.isVector2||i&&i.isEuler&&n&&n.isEuler||i&&i.isVector3&&n&&n.isVector3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";t&&(e={textures:{},images:{}});let n={metadata:{version:4.7,type:"Material",generator:"Material.toJSON"}};n.uuid=this.uuid,n.type=this.type,n.blending=this.blending,n.side=this.side,n.shadowSide=this.shadowSide,n.vertexColors=this.vertexColors,n.opacity=this.opacity,n.transparent=this.transparent,n.blendSrc=this.blendSrc,n.blendDst=this.blendDst,n.blendEquation=this.blendEquation,n.blendSrcAlpha=this.blendSrcAlpha,n.blendDstAlpha=this.blendDstAlpha,n.blendEquationAlpha=this.blendEquationAlpha,n.blendColor=this.blendColor.getHex(),n.blendAlpha=this.blendAlpha,n.depthFunc=this.depthFunc,n.depthTest=this.depthTest,n.depthWrite=this.depthWrite,n.colorWrite=this.colorWrite,n.clipIntersection=this.clipIntersection,n.clipShadows=this.clipShadows,n.stencilWriteMask=this.stencilWriteMask,n.stencilFunc=this.stencilFunc,n.stencilRef=this.stencilRef,n.stencilFuncMask=this.stencilFuncMask,n.stencilFail=this.stencilFail,n.stencilZFail=this.stencilZFail,n.stencilZPass=this.stencilZPass,n.stencilWrite=this.stencilWrite,n.polygonOffset=this.polygonOffset,n.polygonOffsetFactor=this.polygonOffsetFactor,n.polygonOffsetUnits=this.polygonOffsetUnits,n.dithering=this.dithering,n.alphaTest=this.alphaTest,n.alphaHash=this.alphaHash,n.alphaToCoverage=this.alphaToCoverage,n.premultipliedAlpha=this.premultipliedAlpha,n.forceSinglePass=this.forceSinglePass,n.allowOverride=this.allowOverride,n.visible=this.visible,n.toneMapped=this.toneMapped,n.name=this.name,this.color&&this.color.isColor&&(n.color=this.color.getHex()),this.roughness!==void 0&&(n.roughness=this.roughness),this.metalness!==void 0&&(n.metalness=this.metalness),this.sheen!==void 0&&(n.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(n.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(n.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(n.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&(n.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(n.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(n.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(n.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(n.shininess=this.shininess),this.clearcoat!==void 0&&(n.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(n.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(n.clearcoatMap=this.clearcoatMap.toJSON(e).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(n.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(e).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(n.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(e).uuid,n.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.sheenColorMap&&this.sheenColorMap.isTexture&&(n.sheenColorMap=this.sheenColorMap.toJSON(e).uuid),this.sheenRoughnessMap&&this.sheenRoughnessMap.isTexture&&(n.sheenRoughnessMap=this.sheenRoughnessMap.toJSON(e).uuid),this.dispersion!==void 0&&(n.dispersion=this.dispersion),this.retroreflectivity!==void 0&&(n.retroreflectivity=this.retroreflectivity),this.iridescence!==void 0&&(n.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(n.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(n.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(n.iridescenceMap=this.iridescenceMap.toJSON(e).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(n.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(e).uuid),this.anisotropy!==void 0&&(n.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(n.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(n.anisotropyMap=this.anisotropyMap.toJSON(e).uuid),this.map&&this.map.isTexture&&(n.map=this.map.toJSON(e).uuid),this.matcap&&this.matcap.isTexture&&(n.matcap=this.matcap.toJSON(e).uuid),this.alphaMap&&this.alphaMap.isTexture&&(n.alphaMap=this.alphaMap.toJSON(e).uuid),this.lightMap&&this.lightMap.isTexture&&(n.lightMap=this.lightMap.toJSON(e).uuid,n.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(n.aoMap=this.aoMap.toJSON(e).uuid,n.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(n.bumpMap=this.bumpMap.toJSON(e).uuid,n.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(n.normalMap=this.normalMap.toJSON(e).uuid,n.normalMapType=this.normalMapType,n.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(n.displacementMap=this.displacementMap.toJSON(e).uuid,n.displacementScale=this.displacementScale,n.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(n.roughnessMap=this.roughnessMap.toJSON(e).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(n.metalnessMap=this.metalnessMap.toJSON(e).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(n.emissiveMap=this.emissiveMap.toJSON(e).uuid),this.specularMap&&this.specularMap.isTexture&&(n.specularMap=this.specularMap.toJSON(e).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(n.specularIntensityMap=this.specularIntensityMap.toJSON(e).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(n.specularColorMap=this.specularColorMap.toJSON(e).uuid),this.envMap&&this.envMap.isTexture&&(n.envMap=this.envMap.toJSON(e).uuid,this.combine!==void 0&&(n.combine=this.combine)),this.envMapRotation!==void 0&&(n.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(n.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(n.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(n.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(n.gradientMap=this.gradientMap.toJSON(e).uuid),this.transmission!==void 0&&(n.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(n.transmissionMap=this.transmissionMap.toJSON(e).uuid),this.thickness!==void 0&&(n.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(n.thicknessMap=this.thicknessMap.toJSON(e).uuid),this.attenuationDistance!==void 0&&(n.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(n.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(n.size=this.size),this.sizeAttenuation!==void 0&&(n.sizeAttenuation=this.sizeAttenuation),Array.isArray(this.clippingPlanes)&&this.clippingPlanes.length>0&&(n.clippingPlanes=this.clippingPlanes.map(r=>r.toJSON())),this.rotation!==void 0&&(n.rotation=this.rotation),this.depthPacking!==void 0&&(n.depthPacking=this.depthPacking),this.linewidth!==void 0&&(n.linewidth=this.linewidth),this.linecap!==void 0&&(n.linecap=this.linecap),this.linejoin!==void 0&&(n.linejoin=this.linejoin),this.dashSize!==void 0&&(n.dashSize=this.dashSize),this.gapSize!==void 0&&(n.gapSize=this.gapSize),this.scale!==void 0&&(n.scale=this.scale),this.wireframe!==void 0&&(n.wireframe=this.wireframe),this.wireframeLinewidth!==void 0&&(n.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!==void 0&&(n.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!==void 0&&(n.wireframeLinejoin=this.wireframeLinejoin),this.flatShading!==void 0&&(n.flatShading=this.flatShading),this.fog!==void 0&&(n.fog=this.fog),Object.keys(this.userData).length>0&&(n.userData=this.userData);function i(r){let a=[];for(let o in r){let l=r[o];delete l.metadata,a.push(l)}return a}if(t){let r=i(e.textures),a=i(e.images);r.length>0&&(n.textures=r),a.length>0&&(n.images=a)}return n}fromJSON(e,t){if(e.uuid!==void 0&&(this.uuid=e.uuid),e.name!==void 0&&(this.name=e.name),e.color!==void 0&&this.color!==void 0&&this.color.setHex(e.color),e.roughness!==void 0&&(this.roughness=e.roughness),e.metalness!==void 0&&(this.metalness=e.metalness),e.sheen!==void 0&&(this.sheen=e.sheen),e.sheenColor!==void 0&&(this.sheenColor=new Le().setHex(e.sheenColor)),e.sheenRoughness!==void 0&&(this.sheenRoughness=e.sheenRoughness),e.emissive!==void 0&&this.emissive!==void 0&&this.emissive.setHex(e.emissive),e.specular!==void 0&&this.specular!==void 0&&this.specular.setHex(e.specular),e.specularIntensity!==void 0&&(this.specularIntensity=e.specularIntensity),e.specularColor!==void 0&&this.specularColor!==void 0&&this.specularColor.setHex(e.specularColor),e.shininess!==void 0&&(this.shininess=e.shininess),e.clearcoat!==void 0&&(this.clearcoat=e.clearcoat),e.clearcoatRoughness!==void 0&&(this.clearcoatRoughness=e.clearcoatRoughness),e.dispersion!==void 0&&(this.dispersion=e.dispersion),e.retroreflectivity!==void 0&&(this.retroreflectivity=e.retroreflectivity),e.iridescence!==void 0&&(this.iridescence=e.iridescence),e.iridescenceIOR!==void 0&&(this.iridescenceIOR=e.iridescenceIOR),e.iridescenceThicknessRange!==void 0&&(this.iridescenceThicknessRange=e.iridescenceThicknessRange),e.transmission!==void 0&&(this.transmission=e.transmission),e.thickness!==void 0&&(this.thickness=e.thickness),e.attenuationDistance!==void 0&&(this.attenuationDistance=e.attenuationDistance),e.attenuationColor!==void 0&&this.attenuationColor!==void 0&&this.attenuationColor.setHex(e.attenuationColor),e.anisotropy!==void 0&&(this.anisotropy=e.anisotropy),e.anisotropyRotation!==void 0&&(this.anisotropyRotation=e.anisotropyRotation),e.fog!==void 0&&(this.fog=e.fog),e.flatShading!==void 0&&(this.flatShading=e.flatShading),e.blending!==void 0&&(this.blending=e.blending),e.combine!==void 0&&(this.combine=e.combine),e.side!==void 0&&(this.side=e.side),e.shadowSide!==void 0&&(this.shadowSide=e.shadowSide),e.opacity!==void 0&&(this.opacity=e.opacity),e.transparent!==void 0&&(this.transparent=e.transparent),e.alphaTest!==void 0&&(this.alphaTest=e.alphaTest),e.alphaHash!==void 0&&(this.alphaHash=e.alphaHash),e.depthFunc!==void 0&&(this.depthFunc=e.depthFunc),e.depthTest!==void 0&&(this.depthTest=e.depthTest),e.depthWrite!==void 0&&(this.depthWrite=e.depthWrite),e.colorWrite!==void 0&&(this.colorWrite=e.colorWrite),e.clippingPlanes!==void 0&&(this.clippingPlanes=e.clippingPlanes.map(n=>new Zn().fromJSON(n))),e.clipIntersection!==void 0&&(this.clipIntersection=e.clipIntersection),e.clipShadows!==void 0&&(this.clipShadows=e.clipShadows),e.depthPacking!==void 0&&(this.depthPacking=e.depthPacking),e.blendSrc!==void 0&&(this.blendSrc=e.blendSrc),e.blendDst!==void 0&&(this.blendDst=e.blendDst),e.blendEquation!==void 0&&(this.blendEquation=e.blendEquation),e.blendSrcAlpha!==void 0&&(this.blendSrcAlpha=e.blendSrcAlpha),e.blendDstAlpha!==void 0&&(this.blendDstAlpha=e.blendDstAlpha),e.blendEquationAlpha!==void 0&&(this.blendEquationAlpha=e.blendEquationAlpha),e.blendColor!==void 0&&this.blendColor!==void 0&&this.blendColor.setHex(e.blendColor),e.blendAlpha!==void 0&&(this.blendAlpha=e.blendAlpha),e.stencilWriteMask!==void 0&&(this.stencilWriteMask=e.stencilWriteMask),e.stencilFunc!==void 0&&(this.stencilFunc=e.stencilFunc),e.stencilRef!==void 0&&(this.stencilRef=e.stencilRef),e.stencilFuncMask!==void 0&&(this.stencilFuncMask=e.stencilFuncMask),e.stencilFail!==void 0&&(this.stencilFail=e.stencilFail),e.stencilZFail!==void 0&&(this.stencilZFail=e.stencilZFail),e.stencilZPass!==void 0&&(this.stencilZPass=e.stencilZPass),e.stencilWrite!==void 0&&(this.stencilWrite=e.stencilWrite),e.wireframe!==void 0&&(this.wireframe=e.wireframe),e.wireframeLinewidth!==void 0&&(this.wireframeLinewidth=e.wireframeLinewidth),e.wireframeLinecap!==void 0&&(this.wireframeLinecap=e.wireframeLinecap),e.wireframeLinejoin!==void 0&&(this.wireframeLinejoin=e.wireframeLinejoin),e.rotation!==void 0&&(this.rotation=e.rotation),e.linewidth!==void 0&&(this.linewidth=e.linewidth),e.linecap!==void 0&&(this.linecap=e.linecap),e.linejoin!==void 0&&(this.linejoin=e.linejoin),e.dashSize!==void 0&&(this.dashSize=e.dashSize),e.gapSize!==void 0&&(this.gapSize=e.gapSize),e.scale!==void 0&&(this.scale=e.scale),e.polygonOffset!==void 0&&(this.polygonOffset=e.polygonOffset),e.polygonOffsetFactor!==void 0&&(this.polygonOffsetFactor=e.polygonOffsetFactor),e.polygonOffsetUnits!==void 0&&(this.polygonOffsetUnits=e.polygonOffsetUnits),e.dithering!==void 0&&(this.dithering=e.dithering),e.alphaToCoverage!==void 0&&(this.alphaToCoverage=e.alphaToCoverage),e.premultipliedAlpha!==void 0&&(this.premultipliedAlpha=e.premultipliedAlpha),e.forceSinglePass!==void 0&&(this.forceSinglePass=e.forceSinglePass),e.allowOverride!==void 0&&(this.allowOverride=e.allowOverride),e.visible!==void 0&&(this.visible=e.visible),e.toneMapped!==void 0&&(this.toneMapped=e.toneMapped),e.userData!==void 0&&(this.userData=e.userData),e.vertexColors!==void 0&&(typeof e.vertexColors=="number"?this.vertexColors=e.vertexColors>0:this.vertexColors=e.vertexColors),e.size!==void 0&&(this.size=e.size),e.sizeAttenuation!==void 0&&(this.sizeAttenuation=e.sizeAttenuation),e.map!==void 0&&(this.map=t[e.map]||null),e.matcap!==void 0&&(this.matcap=t[e.matcap]||null),e.alphaMap!==void 0&&(this.alphaMap=t[e.alphaMap]||null),e.bumpMap!==void 0&&(this.bumpMap=t[e.bumpMap]||null),e.bumpScale!==void 0&&(this.bumpScale=e.bumpScale),e.normalMap!==void 0&&(this.normalMap=t[e.normalMap]||null),e.normalMapType!==void 0&&(this.normalMapType=e.normalMapType),e.normalScale!==void 0){let n=e.normalScale;Array.isArray(n)===!1&&(n=[n,n]),this.normalScale=new Ge().fromArray(n)}return e.displacementMap!==void 0&&(this.displacementMap=t[e.displacementMap]||null),e.displacementScale!==void 0&&(this.displacementScale=e.displacementScale),e.displacementBias!==void 0&&(this.displacementBias=e.displacementBias),e.roughnessMap!==void 0&&(this.roughnessMap=t[e.roughnessMap]||null),e.metalnessMap!==void 0&&(this.metalnessMap=t[e.metalnessMap]||null),e.emissiveMap!==void 0&&(this.emissiveMap=t[e.emissiveMap]||null),e.emissiveIntensity!==void 0&&(this.emissiveIntensity=e.emissiveIntensity),e.specularMap!==void 0&&(this.specularMap=t[e.specularMap]||null),e.specularIntensityMap!==void 0&&(this.specularIntensityMap=t[e.specularIntensityMap]||null),e.specularColorMap!==void 0&&(this.specularColorMap=t[e.specularColorMap]||null),e.envMap!==void 0&&(this.envMap=t[e.envMap]||null),e.envMapRotation!==void 0&&this.envMapRotation.fromArray(e.envMapRotation),e.envMapIntensity!==void 0&&(this.envMapIntensity=e.envMapIntensity),e.reflectivity!==void 0&&(this.reflectivity=e.reflectivity),e.refractionRatio!==void 0&&(this.refractionRatio=e.refractionRatio),e.lightMap!==void 0&&(this.lightMap=t[e.lightMap]||null),e.lightMapIntensity!==void 0&&(this.lightMapIntensity=e.lightMapIntensity),e.aoMap!==void 0&&(this.aoMap=t[e.aoMap]||null),e.aoMapIntensity!==void 0&&(this.aoMapIntensity=e.aoMapIntensity),e.gradientMap!==void 0&&(this.gradientMap=t[e.gradientMap]||null),e.clearcoatMap!==void 0&&(this.clearcoatMap=t[e.clearcoatMap]||null),e.clearcoatRoughnessMap!==void 0&&(this.clearcoatRoughnessMap=t[e.clearcoatRoughnessMap]||null),e.clearcoatNormalMap!==void 0&&(this.clearcoatNormalMap=t[e.clearcoatNormalMap]||null),e.clearcoatNormalScale!==void 0&&(this.clearcoatNormalScale=new Ge().fromArray(e.clearcoatNormalScale)),e.iridescenceMap!==void 0&&(this.iridescenceMap=t[e.iridescenceMap]||null),e.iridescenceThicknessMap!==void 0&&(this.iridescenceThicknessMap=t[e.iridescenceThicknessMap]||null),e.transmissionMap!==void 0&&(this.transmissionMap=t[e.transmissionMap]||null),e.thicknessMap!==void 0&&(this.thicknessMap=t[e.thicknessMap]||null),e.anisotropyMap!==void 0&&(this.anisotropyMap=t[e.anisotropyMap]||null),e.sheenColorMap!==void 0&&(this.sheenColorMap=t[e.sheenColorMap]||null),e.sheenRoughnessMap!==void 0&&(this.sheenRoughnessMap=t[e.sheenRoughnessMap]||null),this}clone(){return new this.constructor().copy(this)}copy(e){this.name=e.name,this.blending=e.blending,this.side=e.side,this.vertexColors=e.vertexColors,this.opacity=e.opacity,this.transparent=e.transparent,this.blendSrc=e.blendSrc,this.blendDst=e.blendDst,this.blendEquation=e.blendEquation,this.blendSrcAlpha=e.blendSrcAlpha,this.blendDstAlpha=e.blendDstAlpha,this.blendEquationAlpha=e.blendEquationAlpha,this.blendColor.copy(e.blendColor),this.blendAlpha=e.blendAlpha,this.depthFunc=e.depthFunc,this.depthTest=e.depthTest,this.depthWrite=e.depthWrite,this.stencilWriteMask=e.stencilWriteMask,this.stencilFunc=e.stencilFunc,this.stencilRef=e.stencilRef,this.stencilFuncMask=e.stencilFuncMask,this.stencilFail=e.stencilFail,this.stencilZFail=e.stencilZFail,this.stencilZPass=e.stencilZPass,this.stencilWrite=e.stencilWrite;let t=e.clippingPlanes,n=null;if(t!==null){let i=t.length;n=new Array(i);for(let r=0;r!==i;++r)n[r]=t[r].clone()}return this.clippingPlanes=n,this.clipIntersection=e.clipIntersection,this.clipShadows=e.clipShadows,this.shadowSide=e.shadowSide,this.colorWrite=e.colorWrite,this.precision=e.precision,this.polygonOffset=e.polygonOffset,this.polygonOffsetFactor=e.polygonOffsetFactor,this.polygonOffsetUnits=e.polygonOffsetUnits,this.dithering=e.dithering,this.alphaTest=e.alphaTest,this.alphaHash=e.alphaHash,this.alphaToCoverage=e.alphaToCoverage,this.premultipliedAlpha=e.premultipliedAlpha,this.forceSinglePass=e.forceSinglePass,this.allowOverride=e.allowOverride,this.visible=e.visible,this.toneMapped=e.toneMapped,this.userData=JSON.parse(JSON.stringify(e.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(e){e===!0&&this.version++}};var $i=new U,Fh=new U,Ko=new U,Yo=new U,Fi=class{constructor(e=new U,t=new U(0,0,-1)){this.origin=e,this.direction=t}set(e,t){return this.origin.copy(e),this.direction.copy(t),this}copy(e){return this.origin.copy(e.origin),this.direction.copy(e.direction),this}at(e,t){return t.copy(this.origin).addScaledVector(this.direction,e)}lookAt(e){return this.direction.copy(e).sub(this.origin).normalize(),this}recast(e){return this.origin.copy(this.at(e,$i)),this}closestPointToPoint(e,t){t.subVectors(e,this.origin);let n=t.dot(this.direction);return n<0?t.copy(this.origin):t.copy(this.origin).addScaledVector(this.direction,n)}distanceToPoint(e){return Math.sqrt(this.distanceSqToPoint(e))}distanceSqToPoint(e){let t=$i.subVectors(e,this.origin).dot(this.direction);return t<0?this.origin.distanceToSquared(e):($i.copy(this.origin).addScaledVector(this.direction,t),$i.distanceToSquared(e))}distanceSqToSegment(e,t,n,i){Fh.copy(e).add(t).multiplyScalar(.5),Ko.copy(t).sub(e).normalize(),Yo.copy(this.origin).sub(Fh);let r=e.distanceTo(t)*.5,a=-this.direction.dot(Ko),o=Yo.dot(this.direction),l=-Yo.dot(Ko),c=Yo.lengthSq(),h=Math.abs(1-a*a),d,u,p,x;if(h>0)if(d=a*l-o,u=a*o-l,x=r*h,d>=0)if(u>=-x)if(u<=x){let M=1/h;d*=M,u*=M,p=d*(d+a*u+2*o)+u*(a*d+u+2*l)+c}else u=r,d=Math.max(0,-(a*u+o)),p=-d*d+u*(u+2*l)+c;else u=-r,d=Math.max(0,-(a*u+o)),p=-d*d+u*(u+2*l)+c;else u<=-x?(d=Math.max(0,-(-a*r+o)),u=d>0?-r:Math.min(Math.max(-r,-l),r),p=-d*d+u*(u+2*l)+c):u<=x?(d=0,u=Math.min(Math.max(-r,-l),r),p=u*(u+2*l)+c):(d=Math.max(0,-(a*r+o)),u=d>0?r:Math.min(Math.max(-r,-l),r),p=-d*d+u*(u+2*l)+c);else u=a>0?-r:r,d=Math.max(0,-(a*u+o)),p=-d*d+u*(u+2*l)+c;return n&&n.copy(this.origin).addScaledVector(this.direction,d),i&&i.copy(Fh).addScaledVector(Ko,u),p}intersectSphere(e,t){if(e.radius<0)return null;$i.subVectors(e.center,this.origin);let n=$i.dot(this.direction),i=$i.dot($i)-n*n,r=e.radius*e.radius;if(i>r)return null;let a=Math.sqrt(r-i),o=n-a,l=n+a;return l<0?null:o<0?this.at(l,t):this.at(o,t)}intersectsSphere(e){return e.radius<0?!1:this.distanceSqToPoint(e.center)<=e.radius*e.radius}distanceToPlane(e){let t=e.normal.dot(this.direction);if(t===0)return e.distanceToPoint(this.origin)===0?0:null;let n=-(this.origin.dot(e.normal)+e.constant)/t;return n>=0?n:null}intersectPlane(e,t){let n=this.distanceToPlane(e);return n===null?null:this.at(n,t)}intersectsPlane(e){let t=e.distanceToPoint(this.origin);return t===0||e.normal.dot(this.direction)*t<0}intersectBox(e,t){let n,i,r,a,o,l,c=1/this.direction.x,h=1/this.direction.y,d=1/this.direction.z,u=this.origin;return c>=0?(n=(e.min.x-u.x)*c,i=(e.max.x-u.x)*c):(n=(e.max.x-u.x)*c,i=(e.min.x-u.x)*c),h>=0?(r=(e.min.y-u.y)*h,a=(e.max.y-u.y)*h):(r=(e.max.y-u.y)*h,a=(e.min.y-u.y)*h),n>a||r>i||((r>n||isNaN(n))&&(n=r),(a<i||isNaN(i))&&(i=a),d>=0?(o=(e.min.z-u.z)*d,l=(e.max.z-u.z)*d):(o=(e.max.z-u.z)*d,l=(e.min.z-u.z)*d),n>l||o>i)||((o>n||n!==n)&&(n=o),(l<i||i!==i)&&(i=l),i<0)?null:this.at(n>=0?n:i,t)}intersectsBox(e){return this.intersectBox(e,$i)!==null}intersectTriangle(e,t,n,i,r){let a=this.origin,o=this.direction,l=o.x,c=o.y,h=o.z,d=e.x-a.x,u=e.y-a.y,p=e.z-a.z,x=t.x-a.x,M=t.y-a.y,m=t.z-a.z,f=n.x-a.x,b=n.y-a.y,E=n.z-a.z,g=Math.abs(l),_=Math.abs(c),w=Math.abs(h),T,y,C,S,D,v,I,P,N,z,X,j;if(g>=_&&g>=w?(C=l,v=d,N=x,j=f,l>=0?(T=c,y=h,S=u,D=p,I=M,P=m,z=b,X=E):(T=h,y=c,S=p,D=u,I=m,P=M,z=E,X=b)):_>=w?(C=c,v=u,N=M,j=b,c>=0?(T=h,y=l,S=p,D=d,I=m,P=x,z=E,X=f):(T=l,y=h,S=d,D=p,I=x,P=m,z=f,X=E)):(C=h,v=p,N=m,j=E,h>=0?(T=l,y=c,S=d,D=u,I=x,P=M,z=f,X=b):(T=c,y=l,S=u,D=d,I=M,P=x,z=b,X=f)),C===0)return null;let V=T/C,W=y/C,ie=1/C,H=S-V*v,ge=D-W*v,Ze=I-V*N,G=P-W*N,ye=z-V*j,J=X-W*j,A=ye*G-J*Ze,me=H*J-ge*ye,ce=Ze*ge-G*H;if(i){if(A<0||me<0||ce<0)return null}else if((A<0||me<0||ce<0)&&(A>0||me>0||ce>0))return null;let ne=A+me+ce;if(ne===0)return null;let K=ie*(A*v+me*N+ce*j);return(ne>0?K<0:K>0)?null:this.at(K/ne,r)}applyMatrix4(e){return this.origin.applyMatrix4(e),this.direction.transformDirection(e),this}equals(e){return e.origin.equals(this.origin)&&e.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},jt=class extends Gn{constructor(e){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new Le(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Vn,this.combine=jl,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.fog=e.fog,this}},$d=new yt,Hs=new Fi,Jo=new Un,Qd=new U,Zo=new U,$o=new U,Qo=new U,Nh=new U,el=new U,ef=new U,tl=new U,ht=class extends Yt{constructor(e=new At,t=new jt){super(),this.isMesh=!0,this.type="Mesh",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.count=1,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),e.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=e.morphTargetInfluences.slice()),e.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},e.morphTargetDictionary)),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}getVertexPosition(e,t){let n=this.geometry,i=n.attributes.position,r=n.morphAttributes.position,a=n.morphTargetsRelative;t.fromBufferAttribute(i,e);let o=this.morphTargetInfluences;if(r&&o){el.set(0,0,0);for(let l=0,c=r.length;l<c;l++){let h=o[l],d=r[l];h!==0&&(Nh.fromBufferAttribute(d,e),a?el.addScaledVector(Nh,h):el.addScaledVector(Nh.sub(t),h))}t.add(el)}return t}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.material,r=this.matrixWorld;i!==void 0&&(n.boundingSphere===null&&n.computeBoundingSphere(),Jo.copy(n.boundingSphere),Jo.applyMatrix4(r),Hs.copy(e.ray).recast(e.near),!(Jo.containsPoint(Hs.origin)===!1&&(Hs.intersectSphere(Jo,Qd)===null||Hs.origin.distanceToSquared(Qd)>(e.far-e.near)**2))&&($d.copy(r).invert(),Hs.copy(e.ray).applyMatrix4($d),!(n.boundingBox!==null&&Hs.intersectsBox(n.boundingBox)===!1)&&this._computeIntersections(e,t,Hs)))}_computeIntersections(e,t,n){let i,r=this.geometry,a=this.material,o=r.index,l=r.attributes.position,c=r.attributes.uv,h=r.attributes.uv1,d=r.attributes.normal,u=r.groups,p=r.drawRange;if(o!==null)if(Array.isArray(a))for(let x=0,M=u.length;x<M;x++){let m=u[x],f=a[m.materialIndex],b=Math.max(m.start,p.start),E=Math.min(o.count,Math.min(m.start+m.count,p.start+p.count));for(let g=b,_=E;g<_;g+=3){let w=o.getX(g),T=o.getX(g+1),y=o.getX(g+2);i=nl(this,f,e,n,c,h,d,w,T,y),i&&(i.faceIndex=Math.floor(g/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let x=Math.max(0,p.start),M=Math.min(o.count,p.start+p.count);for(let m=x,f=M;m<f;m+=3){let b=o.getX(m),E=o.getX(m+1),g=o.getX(m+2);i=nl(this,a,e,n,c,h,d,b,E,g),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}else if(l!==void 0)if(Array.isArray(a))for(let x=0,M=u.length;x<M;x++){let m=u[x],f=a[m.materialIndex],b=Math.max(m.start,p.start),E=Math.min(l.count,Math.min(m.start+m.count,p.start+p.count));for(let g=b,_=E;g<_;g+=3){let w=g,T=g+1,y=g+2;i=nl(this,f,e,n,c,h,d,w,T,y),i&&(i.faceIndex=Math.floor(g/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let x=Math.max(0,p.start),M=Math.min(l.count,p.start+p.count);for(let m=x,f=M;m<f;m+=3){let b=m,E=m+1,g=m+2;i=nl(this,a,e,n,c,h,d,b,E,g),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}}};function p0(s,e,t,n,i,r,a,o){let l;if(e.side===yn?l=n.intersectTriangle(a,r,i,!0,o):l=n.intersectTriangle(i,r,a,e.side===oi,o),l===null)return null;tl.copy(o),tl.applyMatrix4(s.matrixWorld);let c=t.ray.origin.distanceTo(tl);return c<t.near||c>t.far?null:{distance:c,point:tl.clone(),object:s}}function nl(s,e,t,n,i,r,a,o,l,c){s.getVertexPosition(o,Zo),s.getVertexPosition(l,$o),s.getVertexPosition(c,Qo);let h=p0(s,e,t,n,Zo,$o,Qo,ef);if(h){let d=new U;ys.getBarycoord(ef,Zo,$o,Qo,d),i&&(h.uv=ys.getInterpolatedAttribute(i,o,l,c,d,new Ge)),r&&(h.uv1=ys.getInterpolatedAttribute(r,o,l,c,d,new Ge)),a&&(h.normal=ys.getInterpolatedAttribute(a,o,l,c,d,new U),h.normal.dot(n.direction)>0&&h.normal.multiplyScalar(-1));let u={a:o,b:l,c,normal:new U,materialIndex:0};ys.getNormal(Zo,$o,Qo,u.normal),h.face=u,h.barycoord=d}return h}var Sa=new Ot,tf=new Ot,nf=new Ot,m0=new Ot,sf=new yt,il=new U,Uh=new Un,rf=new yt,zh=new Fi,ka=class extends ht{constructor(e,t){super(e,t),this.isSkinnedMesh=!0,this.type="SkinnedMesh",this.bindMode=Wh,this.bindMatrix=new yt,this.bindMatrixInverse=new yt,this.boundingBox=null,this.boundingSphere=null}computeBoundingBox(){let e=this.geometry;this.boundingBox===null&&(this.boundingBox=new un),this.boundingBox.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,il),this.boundingBox.expandByPoint(il)}computeBoundingSphere(){let e=this.geometry;this.boundingSphere===null&&(this.boundingSphere=new Un),this.boundingSphere.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,il),this.boundingSphere.expandByPoint(il)}copy(e,t){return super.copy(e,t),this.bindMode=e.bindMode,this.bindMatrix.copy(e.bindMatrix),this.bindMatrixInverse.copy(e.bindMatrixInverse),this.skeleton=e.skeleton,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}raycast(e,t){let n=this.material,i=this.matrixWorld;n!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Uh.copy(this.boundingSphere),Uh.applyMatrix4(i),e.ray.intersectsSphere(Uh)!==!1&&(rf.copy(i).invert(),zh.copy(e.ray).applyMatrix4(rf),!(this.boundingBox!==null&&zh.intersectsBox(this.boundingBox)===!1)&&this._computeIntersections(e,t,zh)))}getVertexPosition(e,t){return super.getVertexPosition(e,t),this.applyBoneTransform(e,t),t}bind(e,t){this.skeleton=e,t===void 0&&(this.updateMatrixWorld(!0),this.skeleton.calculateInverses(),t=this.matrixWorld),this.bindMatrix.copy(t),this.bindMatrixInverse.copy(t).invert()}pose(){this.skeleton.pose()}normalizeSkinWeights(){let e=new Ot,t=this.geometry.attributes.skinWeight;for(let n=0,i=t.count;n<i;n++){e.fromBufferAttribute(t,n);let r=1/e.manhattanLength();r!==1/0?e.multiplyScalar(r):e.set(1,0,0,0),t.setXYZW(n,e.x,e.y,e.z,e.w)}}updateMatrixWorld(e){super.updateMatrixWorld(e),this.bindMode===Wh?this.bindMatrixInverse.copy(this.matrixWorld).invert():this.bindMode===$f?this.bindMatrixInverse.copy(this.bindMatrix).invert():dt("SkinnedMesh: Unrecognized bindMode: "+this.bindMode)}applyBoneTransform(e,t){let n=this.skeleton,i=this.geometry;tf.fromBufferAttribute(i.attributes.skinIndex,e),nf.fromBufferAttribute(i.attributes.skinWeight,e),t.isVector4?(Sa.copy(t),t.set(0,0,0,0)):(Sa.set(...t,1),t.set(0,0,0)),Sa.applyMatrix4(this.bindMatrix);for(let r=0;r<4;r++){let a=nf.getComponent(r);if(a!==0){let o=tf.getComponent(r);sf.multiplyMatrices(n.bones[o].matrixWorld,n.boneInverses[o]),t.addScaledVector(m0.copy(Sa).applyMatrix4(sf),a)}}return t.isVector4&&(t.w=Sa.w),t.applyMatrix4(this.bindMatrixInverse)}},Br=class extends Yt{constructor(){super(),this.isBone=!0,this.type="Bone"}},_s=class extends hn{constructor(e=null,t=1,n=1,i,r,a,o,l,c=cn,h=cn,d,u){super(null,a,o,l,c,h,i,r,d,u),this.isDataTexture=!0,this.image={data:e,width:t,height:n},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}},af=new yt,g0=new yt,Ba=class s{constructor(e=[],t=[]){this.uuid=vi(),this.bones=e.slice(0),this.boneInverses=t,this.boneMatrices=null,this.boneTexture=null,this.init()}init(){let e=this.bones,t=this.boneInverses;if(this.boneMatrices=new Float32Array(e.length*16),t.length===0)this.calculateInverses();else if(e.length!==t.length){dt("Skeleton: Number of inverse bone matrices does not match amount of bones."),this.boneInverses=[];for(let n=0,i=this.bones.length;n<i;n++)this.boneInverses.push(new yt)}}calculateInverses(){this.boneInverses.length=0;for(let e=0,t=this.bones.length;e<t;e++){let n=new yt;this.bones[e]&&n.copy(this.bones[e].matrixWorld).invert(),this.boneInverses.push(n)}}pose(){for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&n.matrixWorld.copy(this.boneInverses[e]).invert()}for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&(n.parent&&n.parent.isBone?(n.matrix.copy(n.parent.matrixWorld).invert(),n.matrix.multiply(n.matrixWorld)):n.matrix.copy(n.matrixWorld),n.matrix.decompose(n.position,n.quaternion,n.scale))}}update(){let e=this.bones,t=this.boneInverses,n=this.boneMatrices,i=this.boneTexture;for(let r=0,a=e.length;r<a;r++){let o=e[r]?e[r].matrixWorld:g0;af.multiplyMatrices(o,t[r]),af.toArray(n,r*16)}i!==null&&(i.needsUpdate=!0)}clone(){return new s(this.bones,this.boneInverses)}computeBoneTexture(){let e=Math.sqrt(this.bones.length*4);e=Math.ceil(e/4)*4,e=Math.max(e,4);let t=new Float32Array(e*e*4);t.set(this.boneMatrices);let n=new _s(t,e,e,ti,ei);return n.needsUpdate=!0,this.boneMatrices=t,this.boneTexture=n,this}getBoneByName(e){for(let t=0,n=this.bones.length;t<n;t++){let i=this.bones[t];if(i.name===e)return i}}dispose(){this.boneTexture!==null&&(this.boneTexture.dispose(),this.boneTexture=null)}fromJSON(e,t){this.uuid=e.uuid;for(let n=0,i=e.bones.length;n<i;n++){let r=e.bones[n],a=t[r];a===void 0&&(dt("Skeleton: No bone found with UUID:",r),a=new Br),this.bones.push(a),this.boneInverses.push(new yt().fromArray(e.boneInverses[n]))}return this.init(),this}toJSON(){let e={metadata:{version:4.7,type:"Skeleton",generator:"Skeleton.toJSON"},bones:[],boneInverses:[]};e.uuid=this.uuid;let t=this.bones,n=this.boneInverses;for(let i=0,r=t.length;i<r;i++){let a=t[i];e.bones.push(a.uuid);let o=n[i];e.boneInverses.push(o.toArray())}return e}},yi=class extends Pt{constructor(e,t,n,i=1){super(e,t,n),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=i}copy(e){return super.copy(e),this.meshPerAttribute=e.meshPerAttribute,this}toJSON(){let e=super.toJSON();return e.meshPerAttribute=this.meshPerAttribute,e.isInstancedBufferAttribute=!0,e}},Er=new yt,of=new yt,sl=[],lf=new un,x0=new yt,Ea=new ht,Ta=new Un,An=class extends ht{constructor(e,t,n){super(e,t),this.isInstancedMesh=!0,this.instanceMatrix=new yi(new Float32Array(n*16),16),this.instanceColor=null,this.morphTexture=null,this.count=n,this.boundingBox=null,this.boundingSphere=null;for(let i=0;i<n;i++)this.setMatrixAt(i,x0)}computeBoundingBox(){let e=this.geometry,t=this.count;this.boundingBox===null&&(this.boundingBox=new un),e.boundingBox===null&&e.computeBoundingBox(),this.boundingBox.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,Er),lf.copy(e.boundingBox).applyMatrix4(Er),this.boundingBox.union(lf)}computeBoundingSphere(){let e=this.geometry,t=this.count;this.boundingSphere===null&&(this.boundingSphere=new Un),e.boundingSphere===null&&e.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,Er),Ta.copy(e.boundingSphere).applyMatrix4(Er),this.boundingSphere.union(Ta)}copy(e,t){return super.copy(e,t),this.instanceMatrix.copy(e.instanceMatrix),e.morphTexture!==null&&(this.morphTexture=e.morphTexture.clone()),e.instanceColor!==null&&(this.instanceColor=e.instanceColor.clone()),this.count=e.count,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}getColorAt(e,t){return this.instanceColor===null?t.setRGB(1,1,1):t.fromArray(this.instanceColor.array,e*3)}getMatrixAt(e,t){return t.fromArray(this.instanceMatrix.array,e*16)}getMorphAt(e,t){let n=t.morphTargetInfluences,i=this.morphTexture.source.data.data,r=n.length+1,a=e*r+1;for(let o=0;o<n.length;o++)n[o]=i[a+o]}raycast(e,t){let n=this.matrixWorld,i=this.count;if(Ea.geometry=this.geometry,Ea.material=this.material,Ea.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Ta.copy(this.boundingSphere),Ta.applyMatrix4(n),e.ray.intersectsSphere(Ta)!==!1))for(let r=0;r<i;r++){this.getMatrixAt(r,Er),of.multiplyMatrices(n,Er),Ea.matrixWorld=of,Ea.raycast(e,sl);for(let a=0,o=sl.length;a<o;a++){let l=sl[a];l.instanceId=r,l.object=this,t.push(l)}sl.length=0}}setColorAt(e,t){return this.instanceColor===null&&(this.instanceColor=new yi(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),t.toArray(this.instanceColor.array,e*3),this}setMatrixAt(e,t){return t.toArray(this.instanceMatrix.array,e*16),this}setMorphAt(e,t){let n=t.morphTargetInfluences,i=n.length+1;this.morphTexture===null&&(this.morphTexture=new _s(new Float32Array(i*this.count),i,this.count,ec,ei));let r=this.morphTexture.source.data.data,a=0;for(let c=0;c<n.length;c++)a+=n[c];let o=this.geometry.morphTargetsRelative?1:1-a,l=i*e;return r[l]=o,r.set(n,l+1),this}updateMorphTargets(){}dispose(){super.dispose(),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null)}},Gs=new Un,b0=new Ge(.5,.5),rl=new U,Hr=class{constructor(e=new Zn,t=new Zn,n=new Zn,i=new Zn,r=new Zn,a=new Zn){this.planes=[e,t,n,i,r,a]}set(e,t,n,i,r,a){let o=this.planes;return o[0].copy(e),o[1].copy(t),o[2].copy(n),o[3].copy(i),o[4].copy(r),o[5].copy(a),this}copy(e){let t=this.planes;for(let n=0;n<6;n++)t[n].copy(e.planes[n]);return this}setFromProjectionMatrix(e,t=bi,n=!1){let i=this.planes,r=e.elements,a=r[0],o=r[1],l=r[2],c=r[3],h=r[4],d=r[5],u=r[6],p=r[7],x=r[8],M=r[9],m=r[10],f=r[11],b=r[12],E=r[13],g=r[14],_=r[15];if(i[0].setComponents(c-a,p-h,f-x,_-b).normalize(),i[1].setComponents(c+a,p+h,f+x,_+b).normalize(),i[2].setComponents(c+o,p+d,f+M,_+E).normalize(),i[3].setComponents(c-o,p-d,f-M,_-E).normalize(),n)i[4].setComponents(l,u,m,g).normalize(),i[5].setComponents(c-l,p-u,f-m,_-g).normalize();else if(i[4].setComponents(c-l,p-u,f-m,_-g).normalize(),t===bi)i[5].setComponents(c+l,p+u,f+m,_+g).normalize();else if(t===Dr)i[5].setComponents(l,u,m,g).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+t);return this}intersectsObject(e){if(e.boundingSphere!==void 0)e.boundingSphere===null&&e.computeBoundingSphere(),Gs.copy(e.boundingSphere).applyMatrix4(e.matrixWorld);else{let t=e.geometry;t.boundingSphere===null&&t.computeBoundingSphere(),Gs.copy(t.boundingSphere).applyMatrix4(e.matrixWorld)}return this.intersectsSphere(Gs)}intersectsSprite(e){Gs.center.set(0,0,0);let t=b0.distanceTo(e.center);return Gs.radius=.7071067811865476+t,Gs.applyMatrix4(e.matrixWorld),this.intersectsSphere(Gs)}intersectsSphere(e){let t=this.planes,n=e.center,i=-e.radius;for(let r=0;r<6;r++)if(t[r].distanceToPoint(n)<i)return!1;return!0}intersectsBox(e){let t=this.planes;for(let n=0;n<6;n++){let i=t[n];if(rl.x=i.normal.x>0?e.max.x:e.min.x,rl.y=i.normal.y>0?e.max.y:e.min.y,rl.z=i.normal.z>0?e.max.z:e.min.z,i.distanceToPoint(rl)<0)return!1}return!0}containsPoint(e){let t=this.planes;for(let n=0;n<6;n++)if(t[n].distanceToPoint(e)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};var Ni=class extends Gn{constructor(e){super(),this.isLineBasicMaterial=!0,this.type="LineBasicMaterial",this.color=new Le(16777215),this.map=null,this.linewidth=1,this.linecap="round",this.linejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.linewidth=e.linewidth,this.linecap=e.linecap,this.linejoin=e.linejoin,this.fog=e.fog,this}},El=new U,Tl=new U,cf=new yt,Aa=new Fi,al=new Un,Oh=new U,hf=new U,ts=class extends Yt{constructor(e=new At,t=new Ni){super(),this.isLine=!0,this.type="Line",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[0];for(let i=1,r=t.count;i<r;i++)El.fromBufferAttribute(t,i-1),Tl.fromBufferAttribute(t,i),n[i]=n[i-1],n[i]+=El.distanceTo(Tl);e.setAttribute("lineDistance",new Mt(n,1))}else dt("Line.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Line.threshold,a=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),al.copy(n.boundingSphere),al.applyMatrix4(i),al.radius+=r,e.ray.intersectsSphere(al)===!1)return;cf.copy(i).invert(),Aa.copy(e.ray).applyMatrix4(cf);let o=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=o*o,c=this.isLineSegments?2:1,h=n.index,u=n.attributes.position;if(h!==null){let p=Math.max(0,a.start),x=Math.min(h.count,a.start+a.count);for(let M=p,m=x-1;M<m;M+=c){let f=h.getX(M),b=h.getX(M+1),E=ol(this,e,Aa,l,f,b,M);E&&t.push(E)}if(this.isLineLoop){let M=h.getX(x-1),m=h.getX(p),f=ol(this,e,Aa,l,M,m,x-1);f&&t.push(f)}}else{let p=Math.max(0,a.start),x=Math.min(u.count,a.start+a.count);for(let M=p,m=x-1;M<m;M+=c){let f=ol(this,e,Aa,l,M,M+1,M);f&&t.push(f)}if(this.isLineLoop){let M=ol(this,e,Aa,l,x-1,p,x-1);M&&t.push(M)}}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}};function ol(s,e,t,n,i,r,a){let o=s.geometry.attributes.position;if(El.fromBufferAttribute(o,i),Tl.fromBufferAttribute(o,r),t.distanceSqToSegment(El,Tl,Oh,hf)>n)return;Oh.applyMatrix4(s.matrixWorld);let c=e.ray.origin.distanceTo(Oh);if(!(c<e.near||c>e.far))return{distance:c,point:hf.clone().applyMatrix4(s.matrixWorld),index:a,face:null,faceIndex:null,barycoord:null,object:s}}var uf=new U,df=new U,Js=class extends ts{constructor(e,t){super(e,t),this.isLineSegments=!0,this.type="LineSegments"}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[];for(let i=0,r=t.count;i<r;i+=2)uf.fromBufferAttribute(t,i),df.fromBufferAttribute(t,i+1),n[i]=i===0?0:n[i-1],n[i+1]=n[i]+uf.distanceTo(df);e.setAttribute("lineDistance",new Mt(n,1))}else dt("LineSegments.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}},Ha=class extends ts{constructor(e,t){super(e,t),this.isLineLoop=!0,this.type="LineLoop"}},Gr=class extends Gn{constructor(e){super(),this.isPointsMaterial=!0,this.type="PointsMaterial",this.color=new Le(16777215),this.map=null,this.alphaMap=null,this.size=1,this.sizeAttenuation=!0,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.alphaMap=e.alphaMap,this.size=e.size,this.sizeAttenuation=e.sizeAttenuation,this.fog=e.fog,this}},ff=new yt,Xh=new Fi,ll=new Un,cl=new U,Rn=class extends Yt{constructor(e=new At,t=new Gr){super(),this.isPoints=!0,this.type="Points",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Points.threshold,a=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),ll.copy(n.boundingSphere),ll.applyMatrix4(i),ll.radius+=r,e.ray.intersectsSphere(ll)===!1)return;ff.copy(i).invert(),Xh.copy(e.ray).applyMatrix4(ff);let o=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=o*o,c=n.index,d=n.attributes.position;if(c!==null){let u=Math.max(0,a.start),p=Math.min(c.count,a.start+a.count);for(let x=u,M=p;x<M;x++){let m=c.getX(x);cl.fromBufferAttribute(d,m),pf(cl,m,l,i,e,t,this)}}else{let u=Math.max(0,a.start),p=Math.min(d.count,a.start+a.count);for(let x=u,M=p;x<M;x++)cl.fromBufferAttribute(d,x),pf(cl,x,l,i,e,t,this)}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,a=i.length;r<a;r++){let o=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[o]=r}}}}};function pf(s,e,t,n,i,r,a){let o=Xh.distanceSqToPoint(s);if(o<t){let l=new U;Xh.closestPointToPoint(s,l),l.applyMatrix4(n);let c=i.ray.origin.distanceTo(l);if(c<i.near||c>i.far)return;r.push({distance:c,distanceToRay:Math.sqrt(o),point:l,index:e,face:null,faceIndex:null,barycoord:null,object:a})}}var Ga=class extends hn{constructor(e=[],t=Rs,n,i,r,a,o,l,c,h){super(e,t,n,i,r,a,o,l,c,h),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(e){this.image=e}},Zs=class extends hn{constructor(e,t,n,i,r,a,o,l,c){super(e,t,n,i,r,a,o,l,c),this.isCanvasTexture=!0,this.needsUpdate=!0}};var Ms=class extends hn{constructor(e,t,n=Ti,i,r,a,o=cn,l=cn,c,h=Li,d=1){if(h!==Li&&h!==Cs)throw new Error("THREE.DepthTexture: format must be either THREE.DepthFormat or THREE.DepthStencilFormat");let u={width:e,height:t,depth:d};super(u,i,r,a,o,l,h,n,c),this.isDepthTexture=!0,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(e){return super.copy(e),this.source=new Nr(Object.assign({},e.image)),this.compareFunction=e.compareFunction,this}toJSON(e){let t=super.toJSON(e);return t.compareFunction=this.compareFunction,t}},Al=class extends Ms{constructor(e,t=Ti,n=Rs,i,r,a=cn,o=cn,l,c=Li){let h={width:e,height:e,depth:1},d=[h,h,h,h,h,h];super(e,e,t,n,i,r,a,o,l,c),this.image=d,this.isCubeDepthTexture=!0,this.isCubeTexture=!0}get images(){return this.image}set images(e){this.image=e}},Va=class extends hn{constructor(e=null){super(),this.sourceTexture=e,this.isExternalTexture=!0}copy(e){return super.copy(e),this.sourceTexture=e.sourceTexture,this}},_i=class s extends At{constructor(e=1,t=1,n=1,i=1,r=1,a=1){super(),this.type="BoxGeometry",this.parameters={width:e,height:t,depth:n,widthSegments:i,heightSegments:r,depthSegments:a};let o=this;i=Math.floor(i),r=Math.floor(r),a=Math.floor(a);let l=[],c=[],h=[],d=[],u=0,p=0;x("z","y","x",-1,-1,n,t,e,a,r,0),x("z","y","x",1,-1,n,t,-e,a,r,1),x("x","z","y",1,1,e,n,t,i,a,2),x("x","z","y",1,-1,e,n,-t,i,a,3),x("x","y","z",1,-1,e,t,n,i,r,4),x("x","y","z",-1,-1,e,t,-n,i,r,5),this.setIndex(l),this.setAttribute("position",new Mt(c,3)),this.setAttribute("normal",new Mt(h,3)),this.setAttribute("uv",new Mt(d,2));function x(M,m,f,b,E,g,_,w,T,y,C){let S=g/T,D=_/y,v=g/2,I=_/2,P=w/2,N=T+1,z=y+1,X=0,j=0,V=new U;for(let W=0;W<z;W++){let ie=W*D-I;for(let H=0;H<N;H++){let ge=H*S-v;V[M]=ge*b,V[m]=ie*E,V[f]=P,c.push(V.x,V.y,V.z),V[M]=0,V[m]=0,V[f]=w>0?1:-1,h.push(V.x,V.y,V.z),d.push(H/T),d.push(1-W/y),X+=1}}for(let W=0;W<y;W++)for(let ie=0;ie<T;ie++){let H=u+ie+N*W,ge=u+ie+N*(W+1),Ze=u+(ie+1)+N*(W+1),G=u+(ie+1)+N*W;l.push(H,ge,G),l.push(ge,Ze,G),j+=6}o.addGroup(p,j,C),p+=j,u+=X}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.depth,e.widthSegments,e.heightSegments,e.depthSegments)}};var Vr=class s extends At{constructor(e=1,t=32,n=0,i=Math.PI*2){super(),this.type="CircleGeometry",this.parameters={radius:e,segments:t,thetaStart:n,thetaLength:i},t=Math.max(3,t);let r=[],a=[],o=[],l=[],c=new U,h=new Ge;a.push(0,0,0),o.push(0,0,1),l.push(.5,.5);for(let d=0,u=3;d<=t;d++,u+=3){let p=n+d/t*i;c.x=e*Math.cos(p),c.y=e*Math.sin(p),a.push(c.x,c.y,c.z),o.push(0,0,1),h.x=(a[u]/e+1)/2,h.y=(a[u+1]/e+1)/2,l.push(h.x,h.y)}for(let d=1;d<=t;d++)r.push(d,d+1,0);this.setIndex(r),this.setAttribute("position",new Mt(a,3)),this.setAttribute("normal",new Mt(o,3)),this.setAttribute("uv",new Mt(l,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.segments,e.thetaStart,e.thetaLength)}},ws=class s extends At{constructor(e=1,t=1,n=1,i=32,r=1,a=!1,o=0,l=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:e,radiusBottom:t,height:n,radialSegments:i,heightSegments:r,openEnded:a,thetaStart:o,thetaLength:l};let c=this;i=Math.floor(i),r=Math.floor(r);let h=[],d=[],u=[],p=[],x=0,M=[],m=n/2,f=0;b(),a===!1&&(e>0&&E(!0),t>0&&E(!1)),this.setIndex(h),this.setAttribute("position",new Mt(d,3)),this.setAttribute("normal",new Mt(u,3)),this.setAttribute("uv",new Mt(p,2));function b(){let g=new U,_=new U,w=0,T=(t-e)/n;for(let y=0;y<=r;y++){let C=[],S=y/r,D=S*(t-e)+e;for(let v=0;v<=i;v++){let I=v/i,P=I*l+o,N=Math.sin(P),z=Math.cos(P);_.x=D*N,_.y=-S*n+m,_.z=D*z,d.push(_.x,_.y,_.z),g.set(N,T,z).normalize(),u.push(g.x,g.y,g.z),p.push(I,1-S),C.push(x++)}M.push(C)}for(let y=0;y<i;y++)for(let C=0;C<r;C++){let S=M[C][y],D=M[C+1][y],v=M[C+1][y+1],I=M[C][y+1];(e>0||C!==0)&&(h.push(S,D,I),w+=3),(t>0||C!==r-1)&&(h.push(D,v,I),w+=3)}c.addGroup(f,w,0),f+=w}function E(g){let _=x,w=new Ge,T=new U,y=0,C=g===!0?e:t,S=g===!0?1:-1;for(let v=1;v<=i;v++)d.push(0,m*S,0),u.push(0,S,0),p.push(.5,.5),x++;let D=x;for(let v=0;v<=i;v++){let P=v/i*l+o,N=Math.cos(P),z=Math.sin(P);T.x=C*z,T.y=m*S,T.z=C*N,d.push(T.x,T.y,T.z),u.push(0,S,0),w.x=N*.5+.5,w.y=z*.5*S+.5,p.push(w.x,w.y),x++}for(let v=0;v<i;v++){let I=_+v,P=D+v;g===!0?h.push(P,P+1,I):h.push(P+1,P,I),y+=3}c.addGroup(f,y,g===!0?1:2),f+=y}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radiusTop,e.radiusBottom,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},$s=class s extends ws{constructor(e=1,t=1,n=32,i=1,r=!1,a=0,o=Math.PI*2){super(0,e,t,n,i,r,a,o),this.type="ConeGeometry",this.parameters={radius:e,height:t,radialSegments:n,heightSegments:i,openEnded:r,thetaStart:a,thetaLength:o}}static fromJSON(e){return new s(e.radius,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Rl=class s extends At{constructor(e=[],t=[],n=1,i=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:e,indices:t,radius:n,detail:i};let r=[],a=[];o(i),c(n),h(),this.setAttribute("position",new Mt(r,3)),this.setAttribute("normal",new Mt(r.slice(),3)),this.setAttribute("uv",new Mt(a,2)),i===0?this.computeVertexNormals():this.normalizeNormals();function o(b){let E=new U,g=new U,_=new U;for(let w=0;w<t.length;w+=3)p(t[w+0],E),p(t[w+1],g),p(t[w+2],_),l(E,g,_,b)}function l(b,E,g,_){let w=_+1,T=[];for(let y=0;y<=w;y++){T[y]=[];let C=b.clone().lerp(g,y/w),S=E.clone().lerp(g,y/w),D=w-y;for(let v=0;v<=D;v++)v===0&&y===w?T[y][v]=C:T[y][v]=C.clone().lerp(S,v/D)}for(let y=0;y<w;y++)for(let C=0;C<2*(w-y)-1;C++){let S=Math.floor(C/2);C%2===0?(u(T[y][S+1]),u(T[y+1][S]),u(T[y][S])):(u(T[y][S+1]),u(T[y+1][S+1]),u(T[y+1][S]))}}function c(b){let E=new U;for(let g=0;g<r.length;g+=3)E.x=r[g+0],E.y=r[g+1],E.z=r[g+2],E.normalize().multiplyScalar(b),r[g+0]=E.x,r[g+1]=E.y,r[g+2]=E.z}function h(){let b=new U;for(let E=0;E<r.length;E+=3){b.x=r[E+0],b.y=r[E+1],b.z=r[E+2];let g=m(b)/2/Math.PI+.5,_=f(b)/Math.PI+.5;a.push(g,1-_)}x(),d()}function d(){for(let b=0;b<a.length;b+=6){let E=a[b+0],g=a[b+2],_=a[b+4],w=Math.max(E,g,_),T=Math.min(E,g,_);w>.9&&T<.1&&(E<.2&&(a[b+0]+=1),g<.2&&(a[b+2]+=1),_<.2&&(a[b+4]+=1))}}function u(b){r.push(b.x,b.y,b.z)}function p(b,E){let g=b*3;E.x=e[g+0],E.y=e[g+1],E.z=e[g+2]}function x(){let b=new U,E=new U,g=new U,_=new U,w=new Ge,T=new Ge,y=new Ge;for(let C=0,S=0;C<r.length;C+=9,S+=6){b.set(r[C+0],r[C+1],r[C+2]),E.set(r[C+3],r[C+4],r[C+5]),g.set(r[C+6],r[C+7],r[C+8]),w.set(a[S+0],a[S+1]),T.set(a[S+2],a[S+3]),y.set(a[S+4],a[S+5]),_.copy(b).add(E).add(g).divideScalar(3);let D=m(_);M(w,S+0,b,D),M(T,S+2,E,D),M(y,S+4,g,D)}}function M(b,E,g,_){_<0&&b.x===1&&(a[E]=b.x-1),g.x===0&&g.z===0&&(a[E]=_/2/Math.PI+.5)}function m(b){return Math.atan2(b.z,-b.x)}function f(b){return Math.atan2(-b.y,Math.sqrt(b.x*b.x+b.z*b.z))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.vertices,e.indices,e.radius,e.detail)}};var $n=class{constructor(){this.type="Curve",this.arcLengthDivisions=200,this.needsUpdate=!1,this.cacheArcLengths=null}getPoint(){dt("Curve: .getPoint() not implemented.")}getPointAt(e,t){let n=this.getUtoTmapping(e);return this.getPoint(n,t)}getPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return t}getSpacedPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPointAt(n/e));return t}getLength(){let e=this.getLengths();return e[e.length-1]}getLengths(e=this.arcLengthDivisions){if(this.cacheArcLengths&&this.cacheArcLengths.length===e+1&&!this.needsUpdate)return this.cacheArcLengths;this.needsUpdate=!1;let t=[],n,i=this.getPoint(0),r=0;t.push(0);for(let a=1;a<=e;a++)n=this.getPoint(a/e),r+=n.distanceTo(i),t.push(r),i=n;return this.cacheArcLengths=t,t}updateArcLengths(){this.needsUpdate=!0,this.getLengths()}getUtoTmapping(e,t=null){let n=this.getLengths(),i=0,r=n.length,a;t?a=t:a=e*n[r-1];let o=0,l=r-1,c;for(;o<=l;)if(i=Math.floor(o+(l-o)/2),c=n[i]-a,c<0)o=i+1;else if(c>0)l=i-1;else{l=i;break}if(i=l,n[i]===a)return i/(r-1);let h=n[i],u=n[i+1]-h,p=(a-h)/u;return(i+p)/(r-1)}getTangent(e,t){let i=e-1e-4,r=e+1e-4;i<0&&(i=0),r>1&&(r=1);let a=this.getPoint(i),o=this.getPoint(r),l=t||(a.isVector2?new Ge:new U);return l.copy(o).sub(a).normalize(),l}getTangentAt(e,t){let n=this.getUtoTmapping(e);return this.getTangent(n,t)}computeFrenetFrames(e,t=!1){let n=new U,i=[],r=[],a=[],o=new U,l=new yt;for(let p=0;p<=e;p++){let x=p/e;i[p]=this.getTangentAt(x,new U)}r[0]=new U,a[0]=new U;let c=Number.MAX_VALUE,h=Math.abs(i[0].x),d=Math.abs(i[0].y),u=Math.abs(i[0].z);h<=c&&(c=h,n.set(1,0,0)),d<=c&&(c=d,n.set(0,1,0)),u<=c&&n.set(0,0,1),o.crossVectors(i[0],n).normalize(),r[0].crossVectors(i[0],o),a[0].crossVectors(i[0],r[0]);for(let p=1;p<=e;p++){if(r[p]=r[p-1].clone(),a[p]=a[p-1].clone(),o.crossVectors(i[p-1],i[p]),o.length()>Number.EPSILON){o.normalize();let x=Math.acos(Lt(i[p-1].dot(i[p]),-1,1));r[p].applyMatrix4(l.makeRotationAxis(o,x))}a[p].crossVectors(i[p],r[p])}if(t===!0){let p=Math.acos(Lt(r[0].dot(r[e]),-1,1));p/=e,i[0].dot(o.crossVectors(r[0],r[e]))>0&&(p=-p);for(let x=1;x<=e;x++)r[x].applyMatrix4(l.makeRotationAxis(i[x],p*x)),a[x].crossVectors(i[x],r[x])}return{tangents:i,normals:r,binormals:a}}clone(){return new this.constructor().copy(this)}copy(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}toJSON(){let e={metadata:{version:4.7,type:"Curve",generator:"Curve.toJSON"}};return e.arcLengthDivisions=this.arcLengthDivisions,e.type=this.type,e}fromJSON(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}},Wa=class extends $n{constructor(e=0,t=0,n=1,i=1,r=0,a=Math.PI*2,o=!1,l=0){super(),this.isEllipseCurve=!0,this.type="EllipseCurve",this.aX=e,this.aY=t,this.xRadius=n,this.yRadius=i,this.aStartAngle=r,this.aEndAngle=a,this.aClockwise=o,this.aRotation=l}getPoint(e,t=new Ge){let n=t,i=Math.PI*2,r=this.aEndAngle-this.aStartAngle,a=Math.abs(r)<Number.EPSILON;for(;r<0;)r+=i;for(;r>i;)r-=i;r<Number.EPSILON&&(a?r=0:r=i),this.aClockwise===!0&&!a&&(r===i?r=-i:r=r-i);let o=this.aStartAngle+e*r,l=this.aX+this.xRadius*Math.cos(o),c=this.aY+this.yRadius*Math.sin(o);if(this.aRotation!==0){let h=Math.cos(this.aRotation),d=Math.sin(this.aRotation),u=l-this.aX,p=c-this.aY;l=u*h-p*d+this.aX,c=u*d+p*h+this.aY}return n.set(l,c)}copy(e){return super.copy(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}toJSON(){let e=super.toJSON();return e.aX=this.aX,e.aY=this.aY,e.xRadius=this.xRadius,e.yRadius=this.yRadius,e.aStartAngle=this.aStartAngle,e.aEndAngle=this.aEndAngle,e.aClockwise=this.aClockwise,e.aRotation=this.aRotation,e}fromJSON(e){return super.fromJSON(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}},Cl=class extends Wa{constructor(e,t,n,i,r,a){super(e,t,n,n,i,r,a),this.isArcCurve=!0,this.type="ArcCurve"}};function xu(){let s=0,e=0,t=0,n=0;function i(r,a,o,l){s=r,e=o,t=-3*r+3*a-2*o-l,n=2*r-2*a+o+l}return{initCatmullRom:function(r,a,o,l,c){i(a,o,c*(o-r),c*(l-a))},initNonuniformCatmullRom:function(r,a,o,l,c,h,d){let u=(a-r)/c-(o-r)/(c+h)+(o-a)/h,p=(o-a)/h-(l-a)/(h+d)+(l-o)/d;u*=h,p*=h,i(a,o,u,p)},calc:function(r){let a=r*r,o=a*r;return s+e*r+t*a+n*o}}}var mf=new U,gf=new U,kh=new xu,Bh=new xu,Hh=new xu,Wr=class extends $n{constructor(e=[],t=!1,n="centripetal",i=.5){super(),this.isCatmullRomCurve3=!0,this.type="CatmullRomCurve3",this.points=e,this.closed=t,this.curveType=n,this.tension=i}getPoint(e,t=new U){let n=t,i=this.points,r=i.length,a=(r-(this.closed?0:1))*e,o=Math.floor(a),l=a-o;this.closed?o+=o>0?0:(Math.floor(Math.abs(o)/r)+1)*r:l===0&&o===r-1&&(o=r-2,l=1);let c,h;this.closed||o>0?c=i[(o-1)%r]:(gf.subVectors(i[0],i[1]).add(i[0]),c=gf);let d=i[o%r],u=i[(o+1)%r];if(this.closed||o+2<r?h=i[(o+2)%r]:(mf.subVectors(i[r-1],i[r-2]).add(i[r-1]),h=mf),this.curveType==="centripetal"||this.curveType==="chordal"){let p=this.curveType==="chordal"?.5:.25,x=Math.pow(c.distanceToSquared(d),p),M=Math.pow(d.distanceToSquared(u),p),m=Math.pow(u.distanceToSquared(h),p);M<1e-4&&(M=1),x<1e-4&&(x=M),m<1e-4&&(m=M),kh.initNonuniformCatmullRom(c.x,d.x,u.x,h.x,x,M,m),Bh.initNonuniformCatmullRom(c.y,d.y,u.y,h.y,x,M,m),Hh.initNonuniformCatmullRom(c.z,d.z,u.z,h.z,x,M,m)}else this.curveType==="catmullrom"&&(kh.initCatmullRom(c.x,d.x,u.x,h.x,this.tension),Bh.initCatmullRom(c.y,d.y,u.y,h.y,this.tension),Hh.initCatmullRom(c.z,d.z,u.z,h.z,this.tension));return n.set(kh.calc(l),Bh.calc(l),Hh.calc(l)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e.closed=this.closed,e.curveType=this.curveType,e.tension=this.tension,e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new U().fromArray(i))}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}};function xf(s,e,t,n,i){let r=(n-e)*.5,a=(i-t)*.5,o=s*s,l=s*o;return(2*t-2*n+r+a)*l+(-3*t+3*n-2*r-a)*o+r*s+t}function v0(s,e){let t=1-s;return t*t*e}function y0(s,e){return 2*(1-s)*s*e}function _0(s,e){return s*s*e}function Pa(s,e,t,n){return v0(s,e)+y0(s,t)+_0(s,n)}function M0(s,e){let t=1-s;return t*t*t*e}function w0(s,e){let t=1-s;return 3*t*t*s*e}function S0(s,e){return 3*(1-s)*s*s*e}function E0(s,e){return s*s*s*e}function Ia(s,e,t,n,i){return M0(s,e)+w0(s,t)+S0(s,n)+E0(s,i)}var Pl=class extends $n{constructor(e=new Ge,t=new Ge,n=new Ge,i=new Ge){super(),this.isCubicBezierCurve=!0,this.type="CubicBezierCurve",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new Ge){let n=t,i=this.v0,r=this.v1,a=this.v2,o=this.v3;return n.set(Ia(e,i.x,r.x,a.x,o.x),Ia(e,i.y,r.y,a.y,o.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},Il=class extends $n{constructor(e=new U,t=new U,n=new U,i=new U){super(),this.isCubicBezierCurve3=!0,this.type="CubicBezierCurve3",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new U){let n=t,i=this.v0,r=this.v1,a=this.v2,o=this.v3;return n.set(Ia(e,i.x,r.x,a.x,o.x),Ia(e,i.y,r.y,a.y,o.y),Ia(e,i.z,r.z,a.z,o.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},Dl=class extends $n{constructor(e=new Ge,t=new Ge){super(),this.isLineCurve=!0,this.type="LineCurve",this.v1=e,this.v2=t}getPoint(e,t=new Ge){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new Ge){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},qr=class extends $n{constructor(e=new U,t=new U){super(),this.isLineCurve3=!0,this.type="LineCurve3",this.v1=e,this.v2=t}getPoint(e,t=new U){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new U){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Ll=class extends $n{constructor(e=new Ge,t=new Ge,n=new Ge){super(),this.isQuadraticBezierCurve=!0,this.type="QuadraticBezierCurve",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new Ge){let n=t,i=this.v0,r=this.v1,a=this.v2;return n.set(Pa(e,i.x,r.x,a.x),Pa(e,i.y,r.y,a.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Mi=class extends $n{constructor(e=new U,t=new U,n=new U){super(),this.isQuadraticBezierCurve3=!0,this.type="QuadraticBezierCurve3",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new U){let n=t,i=this.v0,r=this.v1,a=this.v2;return n.set(Pa(e,i.x,r.x,a.x),Pa(e,i.y,r.y,a.y),Pa(e,i.z,r.z,a.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Fl=class extends $n{constructor(e=[]){super(),this.isSplineCurve=!0,this.type="SplineCurve",this.points=e}getPoint(e,t=new Ge){let n=t,i=this.points,r=(i.length-1)*e,a=Math.floor(r),o=r-a,l=i[a===0?a:a-1],c=i[a],h=i[a>i.length-2?i.length-1:a+1],d=i[a>i.length-3?i.length-1:a+2];return n.set(xf(o,l.x,c.x,h.x,d.x),xf(o,l.y,c.y,h.y,d.y)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new Ge().fromArray(i))}return this}},jh=Object.freeze({__proto__:null,ArcCurve:Cl,CatmullRomCurve3:Wr,CubicBezierCurve:Pl,CubicBezierCurve3:Il,EllipseCurve:Wa,LineCurve:Dl,LineCurve3:qr,QuadraticBezierCurve:Ll,QuadraticBezierCurve3:Mi,SplineCurve:Fl}),qa=class extends $n{constructor(){super(),this.type="CurvePath",this.curves=[],this.autoClose=!1}add(e){this.curves.push(e)}closePath(){let e=this.curves[0].getPoint(0),t=this.curves[this.curves.length-1].getPoint(1);if(!e.equals(t)){let n=e.isVector2===!0?"LineCurve":"LineCurve3";this.curves.push(new jh[n](t,e))}return this}getPoint(e,t){let n=e*this.getLength(),i=this.getCurveLengths(),r=0;for(;r<i.length;){if(i[r]>=n){let a=i[r]-n,o=this.curves[r],l=o.getLength(),c=l===0?0:1-a/l;return o.getPointAt(c,t)}r++}return null}getLength(){let e=this.getCurveLengths();return e[e.length-1]}updateArcLengths(){this.needsUpdate=!0,this.cacheLengths=null,this.getCurveLengths()}getCurveLengths(){if(this.cacheLengths&&this.cacheLengths.length===this.curves.length)return this.cacheLengths;let e=[],t=0;for(let n=0,i=this.curves.length;n<i;n++)t+=this.curves[n].getLength(),e.push(t);return this.cacheLengths=e,e}getSpacedPoints(e=40){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return this.autoClose&&t.push(t[0]),t}getPoints(e=12){let t=[],n;for(let i=0,r=this.curves;i<r.length;i++){let a=r[i],o=a.isEllipseCurve?e*2:a.isLineCurve||a.isLineCurve3?1:a.isSplineCurve?e*a.points.length:e,l=a.getPoints(o);for(let c=0;c<l.length;c++){let h=l[c];n&&n.equals(h)||(t.push(h),n=h)}}return this.autoClose&&t.length>1&&!t[t.length-1].equals(t[0])&&t.push(t[0]),t}copy(e){super.copy(e),this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(i.clone())}return this.autoClose=e.autoClose,this}toJSON(){let e=super.toJSON();e.autoClose=this.autoClose,e.curves=[];for(let t=0,n=this.curves.length;t<n;t++){let i=this.curves[t];e.curves.push(i.toJSON())}return e}fromJSON(e){super.fromJSON(e),this.autoClose=e.autoClose,this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(new jh[i.type]().fromJSON(i))}return this}};var Xr=class s extends Rl{constructor(e=1,t=0){let n=(1+Math.sqrt(5))/2,i=[-1,n,0,1,n,0,-1,-n,0,1,-n,0,0,-1,n,0,1,n,0,-1,-n,0,1,-n,n,0,-1,n,0,1,-n,0,-1,-n,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(i,r,e,t),this.type="IcosahedronGeometry",this.parameters={radius:e,detail:t}}static fromJSON(e){return new s(e.radius,e.detail)}};var dn=class s extends At{constructor(e=1,t=1,n=1,i=1){super(),this.type="PlaneGeometry",this.parameters={width:e,height:t,widthSegments:n,heightSegments:i};let r=e/2,a=t/2,o=Math.floor(n),l=Math.floor(i),c=o+1,h=l+1,d=e/o,u=t/l,p=[],x=[],M=[],m=[];for(let f=0;f<h;f++){let b=f*u-a;for(let E=0;E<c;E++){let g=E*d-r;x.push(g,-b,0),M.push(0,0,1),m.push(E/o),m.push(1-f/l)}}for(let f=0;f<l;f++)for(let b=0;b<o;b++){let E=b+c*f,g=b+c*(f+1),_=b+1+c*(f+1),w=b+1+c*f;p.push(E,g,w),p.push(g,_,w)}this.setIndex(p),this.setAttribute("position",new Mt(x,3)),this.setAttribute("normal",new Mt(M,3)),this.setAttribute("uv",new Mt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.widthSegments,e.heightSegments)}},Ss=class s extends At{constructor(e=.5,t=1,n=32,i=1,r=0,a=Math.PI*2){super(),this.type="RingGeometry",this.parameters={innerRadius:e,outerRadius:t,thetaSegments:n,phiSegments:i,thetaStart:r,thetaLength:a},n=Math.max(3,n),i=Math.max(1,i);let o=[],l=[],c=[],h=[],d=e,u=(t-e)/i,p=new U,x=new Ge;for(let M=0;M<=i;M++){for(let m=0;m<=n;m++){let f=r+m/n*a;p.x=d*Math.cos(f),p.y=d*Math.sin(f),l.push(p.x,p.y,p.z),c.push(0,0,1),x.x=(p.x/t+1)/2,x.y=(p.y/t+1)/2,h.push(x.x,x.y)}d+=u}for(let M=0;M<i;M++){let m=M*(n+1);for(let f=0;f<n;f++){let b=f+m,E=b,g=b+n+1,_=b+n+2,w=b+1;o.push(E,g,w),o.push(g,_,w)}}this.setIndex(o),this.setAttribute("position",new Mt(l,3)),this.setAttribute("normal",new Mt(c,3)),this.setAttribute("uv",new Mt(h,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.innerRadius,e.outerRadius,e.thetaSegments,e.phiSegments,e.thetaStart,e.thetaLength)}};var Ui=class s extends At{constructor(e=1,t=32,n=16,i=0,r=Math.PI*2,a=0,o=Math.PI){super(),this.type="SphereGeometry",this.parameters={radius:e,widthSegments:t,heightSegments:n,phiStart:i,phiLength:r,thetaStart:a,thetaLength:o},t=Math.max(3,Math.floor(t)),n=Math.max(2,Math.floor(n));let l=Math.min(a+o,Math.PI),c=0,h=[],d=new U,u=new U,p=[],x=[],M=[],m=[];for(let f=0;f<=n;f++){let b=[],E=f/n,g=a+E*o,_=e*Math.cos(g),w=Math.sqrt(e*e-_*_),T=0;f===0&&a===0?T=.5/t:f===n&&l===Math.PI&&(T=-.5/t);for(let y=0;y<=t;y++){let C=y/t,S=i+C*r;d.x=-w*Math.cos(S),d.y=_,d.z=w*Math.sin(S),x.push(d.x,d.y,d.z),u.copy(d).normalize(),M.push(u.x,u.y,u.z),m.push(C+T,1-E),b.push(c++)}h.push(b)}for(let f=0;f<n;f++)for(let b=0;b<t;b++){let E=h[f][b+1],g=h[f][b],_=h[f+1][b],w=h[f+1][b+1];(f!==0||a>0)&&p.push(E,g,w),(f!==n-1||l<Math.PI)&&p.push(g,_,w)}this.setIndex(p),this.setAttribute("position",new Mt(x,3)),this.setAttribute("normal",new Mt(M,3)),this.setAttribute("uv",new Mt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.widthSegments,e.heightSegments,e.phiStart,e.phiLength,e.thetaStart,e.thetaLength)}};var Qn=class s extends At{constructor(e=1,t=.4,n=12,i=48,r=Math.PI*2,a=0,o=Math.PI*2){super(),this.type="TorusGeometry",this.parameters={radius:e,tube:t,radialSegments:n,tubularSegments:i,arc:r,thetaStart:a,thetaLength:o},n=Math.floor(n),i=Math.floor(i);let l=[],c=[],h=[],d=[],u=new U,p=new U,x=new U;for(let M=0;M<=n;M++){let m=a+M/n*o;for(let f=0;f<=i;f++){let b=f/i*r;p.x=(e+t*Math.cos(m))*Math.cos(b),p.y=(e+t*Math.cos(m))*Math.sin(b),p.z=t*Math.sin(m),c.push(p.x,p.y,p.z),u.x=e*Math.cos(b),u.y=e*Math.sin(b),x.subVectors(p,u).normalize(),h.push(x.x,x.y,x.z),d.push(f/i),d.push(M/n)}}for(let M=1;M<=n;M++)for(let m=1;m<=i;m++){let f=(i+1)*M+m-1,b=(i+1)*(M-1)+m-1,E=(i+1)*(M-1)+m,g=(i+1)*M+m;l.push(f,b,g),l.push(b,E,g)}this.setIndex(l),this.setAttribute("position",new Mt(c,3)),this.setAttribute("normal",new Mt(h,3)),this.setAttribute("uv",new Mt(d,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.tube,e.radialSegments,e.tubularSegments,e.arc,e.thetaStart,e.thetaLength)}};var Xa=class s extends At{constructor(e=new Mi(new U(-1,-1,0),new U(-1,1,0),new U(1,1,0)),t=64,n=1,i=8,r=!1){super(),this.type="TubeGeometry",this.parameters={path:e,tubularSegments:t,radius:n,radialSegments:i,closed:r};let a=e.computeFrenetFrames(t,r);this.tangents=a.tangents,this.normals=a.normals,this.binormals=a.binormals;let o=new U,l=new U,c=new Ge,h=new U,d=[],u=[],p=[],x=[];M(),this.setIndex(x),this.setAttribute("position",new Mt(d,3)),this.setAttribute("normal",new Mt(u,3)),this.setAttribute("uv",new Mt(p,2));function M(){for(let E=0;E<t;E++)m(E);m(r===!1?t:0),b(),f()}function m(E){h=e.getPointAt(E/t,h);let g=a.normals[E],_=a.binormals[E];for(let w=0;w<=i;w++){let T=w/i*Math.PI*2,y=Math.sin(T),C=-Math.cos(T);l.x=C*g.x+y*_.x,l.y=C*g.y+y*_.y,l.z=C*g.z+y*_.z,l.normalize(),u.push(l.x,l.y,l.z),o.x=h.x+n*l.x,o.y=h.y+n*l.y,o.z=h.z+n*l.z,d.push(o.x,o.y,o.z)}}function f(){for(let E=1;E<=t;E++)for(let g=1;g<=i;g++){let _=(i+1)*(E-1)+(g-1),w=(i+1)*E+(g-1),T=(i+1)*E+g,y=(i+1)*(E-1)+g;x.push(_,w,y),x.push(w,T,y)}}function b(){for(let E=0;E<=t;E++)for(let g=0;g<=i;g++)c.x=E/t,c.y=g/i,p.push(c.x,c.y)}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}toJSON(){let e=super.toJSON();return e.path=this.parameters.path.toJSON(),e}static fromJSON(e){return new s(new jh[e.path.type]().fromJSON(e.path),e.tubularSegments,e.radius,e.radialSegments,e.closed)}},ja=class extends At{constructor(e=null){if(super(),this.type="WireframeGeometry",this.parameters={geometry:e},e!==null){let t=[],n=new Set,i=new U,r=new U;if(e.index!==null){let a=e.attributes.position,o=e.index,l=e.groups;l.length===0&&(l=[{start:0,count:o.count,materialIndex:0}]);for(let c=0,h=l.length;c<h;++c){let d=l[c],u=d.start,p=d.count;for(let x=u,M=u+p;x<M;x+=3)for(let m=0;m<3;m++){let f=o.getX(x+m),b=o.getX(x+(m+1)%3);i.fromBufferAttribute(a,f),r.fromBufferAttribute(a,b),bf(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}}else{let a=e.attributes.position;for(let o=0,l=a.count/3;o<l;o++)for(let c=0;c<3;c++){let h=3*o+c,d=3*o+(c+1)%3;i.fromBufferAttribute(a,h),r.fromBufferAttribute(a,d),bf(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}this.setAttribute("position",new Mt(t,3))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}};function bf(s,e,t){let n=`${s.x},${s.y},${s.z}-${e.x},${e.y},${e.z}`,i=`${e.x},${e.y},${e.z}-${s.x},${s.y},${s.z}`;return t.has(n)===!0||t.has(i)===!0?!1:(t.add(n),t.add(i),!0)}function ar(s){let e={};for(let t in s){e[t]={};for(let n in s[t]){let i=s[t][n];if(vf(i))i.isRenderTargetTexture?(dt("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),e[t][n]=null):e[t][n]=i.clone();else if(Array.isArray(i))if(vf(i[0])){let r=[];for(let a=0,o=i.length;a<o;a++)r[a]=i[a].clone();e[t][n]=r}else e[t][n]=i.slice();else e[t][n]=i}}return e}function zn(s){let e={};for(let t=0;t<s.length;t++){let n=ar(s[t]);for(let i in n)e[i]=n[i]}return e}function vf(s){return s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)}function T0(s){let e=[];for(let t=0;t<s.length;t++)e.push(s[t].clone());return e}function bu(s){let e=s.getRenderTarget();return e===null?s.outputColorSpace:e.isXRRenderTarget===!0?e.texture.colorSpace:Dt.workingColorSpace}var Ai={clone:ar,merge:zn},A0=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,R0=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,Rt=class extends Gn{constructor(e){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=A0,this.fragmentShader=R0,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,e!==void 0&&this.setValues(e)}copy(e){return super.copy(e),this.fragmentShader=e.fragmentShader,this.vertexShader=e.vertexShader,this.uniforms=ar(e.uniforms),this.uniformsGroups=T0(e.uniformsGroups),this.defines=Object.assign({},e.defines),this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.fog=e.fog,this.lights=e.lights,this.clipping=e.clipping,this.extensions=Object.assign({},e.extensions),this.glslVersion=e.glslVersion,this.defaultAttributeValues=Object.assign({},e.defaultAttributeValues),this.index0AttributeName=e.index0AttributeName,this.uniformsNeedUpdate=e.uniformsNeedUpdate,this}toJSON(e){let t=super.toJSON(e);t.glslVersion=this.glslVersion,t.uniforms={};for(let i in this.uniforms){let a=this.uniforms[i].value;a&&a.isTexture?t.uniforms[i]={type:"t",value:a.toJSON(e).uuid}:a&&a.isColor?t.uniforms[i]={type:"c",value:a.getHex()}:a&&a.isVector2?t.uniforms[i]={type:"v2",value:a.toArray()}:a&&a.isVector3?t.uniforms[i]={type:"v3",value:a.toArray()}:a&&a.isVector4?t.uniforms[i]={type:"v4",value:a.toArray()}:a&&a.isMatrix3?t.uniforms[i]={type:"m3",value:a.toArray()}:a&&a.isMatrix4?t.uniforms[i]={type:"m4",value:a.toArray()}:t.uniforms[i]={value:a}}Object.keys(this.defines).length>0&&(t.defines=this.defines),t.vertexShader=this.vertexShader,t.fragmentShader=this.fragmentShader,t.lights=this.lights,t.clipping=this.clipping;let n={};for(let i in this.extensions)this.extensions[i]===!0&&(n[i]=!0);return Object.keys(n).length>0&&(t.extensions=n),t}fromJSON(e,t){if(super.fromJSON(e,t),e.uniforms!==void 0)for(let n in e.uniforms){let i=e.uniforms[n];switch(this.uniforms[n]={},i.type){case"t":this.uniforms[n].value=t[i.value]||null;break;case"c":this.uniforms[n].value=new Le().setHex(i.value);break;case"v2":this.uniforms[n].value=new Ge().fromArray(i.value);break;case"v3":this.uniforms[n].value=new U().fromArray(i.value);break;case"v4":this.uniforms[n].value=new Ot().fromArray(i.value);break;case"m3":this.uniforms[n].value=new Tt().fromArray(i.value);break;case"m4":this.uniforms[n].value=new yt().fromArray(i.value);break;default:this.uniforms[n].value=i.value}}if(e.defines!==void 0&&(this.defines=e.defines),e.vertexShader!==void 0&&(this.vertexShader=e.vertexShader),e.fragmentShader!==void 0&&(this.fragmentShader=e.fragmentShader),e.glslVersion!==void 0&&(this.glslVersion=e.glslVersion),e.extensions!==void 0)for(let n in e.extensions)this.extensions[n]=e.extensions[n];return e.lights!==void 0&&(this.lights=e.lights),e.clipping!==void 0&&(this.clipping=e.clipping),this}},jr=class extends Rt{constructor(e){super(e),this.isRawShaderMaterial=!0,this.type="RawShaderMaterial"}},vn=class extends Gn{constructor(e){super(),this.isMeshStandardMaterial=!0,this.type="MeshStandardMaterial",this.defines={STANDARD:""},this.color=new Le(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Le(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=_o,this.normalScale=new Ge(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Vn,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.defines={STANDARD:""},this.color.copy(e.color),this.roughness=e.roughness,this.metalness=e.metalness,this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.roughnessMap=e.roughnessMap,this.metalnessMap=e.metalnessMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.envMapIntensity=e.envMapIntensity,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},Wn=class extends vn{constructor(e){super(),this.isMeshPhysicalMaterial=!0,this.defines={STANDARD:"",PHYSICAL:""},this.type="MeshPhysicalMaterial",this.anisotropyRotation=0,this.anisotropyMap=null,this.clearcoatMap=null,this.clearcoatRoughness=0,this.clearcoatRoughnessMap=null,this.clearcoatNormalScale=new Ge(1,1),this.clearcoatNormalMap=null,this.ior=1.5,Object.defineProperty(this,"reflectivity",{get:function(){return Lt(2.5*(this.ior-1)/(this.ior+1),0,1)},set:function(t){this.ior=(1+.4*t)/(1-.4*t)}}),this.iridescenceMap=null,this.iridescenceIOR=1.3,this.iridescenceThicknessRange=[100,400],this.iridescenceThicknessMap=null,this.sheenColor=new Le(0),this.sheenColorMap=null,this.sheenRoughness=1,this.sheenRoughnessMap=null,this.transmissionMap=null,this.thickness=0,this.thicknessMap=null,this.attenuationDistance=1/0,this.attenuationColor=new Le(1,1,1),this.specularIntensity=1,this.specularIntensityMap=null,this.specularColor=new Le(1,1,1),this.specularColorMap=null,this._anisotropy=0,this._clearcoat=0,this._dispersion=0,this._iridescence=0,this._retroreflectivity=0,this._sheen=0,this._transmission=0,this.setValues(e)}get anisotropy(){return this._anisotropy}set anisotropy(e){this._anisotropy>0!=e>0&&this.version++,this._anisotropy=e}get clearcoat(){return this._clearcoat}set clearcoat(e){this._clearcoat>0!=e>0&&this.version++,this._clearcoat=e}get iridescence(){return this._iridescence}set iridescence(e){this._iridescence>0!=e>0&&this.version++,this._iridescence=e}get dispersion(){return this._dispersion}set dispersion(e){this._dispersion>0!=e>0&&this.version++,this._dispersion=e}get retroreflectivity(){return this._retroreflectivity}set retroreflectivity(e){this._retroreflectivity>0!=e>0&&this.version++,this._retroreflectivity=e}get sheen(){return this._sheen}set sheen(e){this._sheen>0!=e>0&&this.version++,this._sheen=e}get transmission(){return this._transmission}set transmission(e){this._transmission>0!=e>0&&this.version++,this._transmission=e}copy(e){return super.copy(e),this.defines={STANDARD:"",PHYSICAL:""},this.anisotropy=e.anisotropy,this.anisotropyRotation=e.anisotropyRotation,this.anisotropyMap=e.anisotropyMap,this.clearcoat=e.clearcoat,this.clearcoatMap=e.clearcoatMap,this.clearcoatRoughness=e.clearcoatRoughness,this.clearcoatRoughnessMap=e.clearcoatRoughnessMap,this.clearcoatNormalMap=e.clearcoatNormalMap,this.clearcoatNormalScale.copy(e.clearcoatNormalScale),this.dispersion=e.dispersion,this.ior=e.ior,this.iridescence=e.iridescence,this.iridescenceMap=e.iridescenceMap,this.iridescenceIOR=e.iridescenceIOR,this.iridescenceThicknessRange=[...e.iridescenceThicknessRange],this.iridescenceThicknessMap=e.iridescenceThicknessMap,this.retroreflectivity=e.retroreflectivity,this.sheen=e.sheen,this.sheenColor.copy(e.sheenColor),this.sheenColorMap=e.sheenColorMap,this.sheenRoughness=e.sheenRoughness,this.sheenRoughnessMap=e.sheenRoughnessMap,this.transmission=e.transmission,this.transmissionMap=e.transmissionMap,this.thickness=e.thickness,this.thicknessMap=e.thicknessMap,this.attenuationDistance=e.attenuationDistance,this.attenuationColor.copy(e.attenuationColor),this.specularIntensity=e.specularIntensity,this.specularIntensityMap=e.specularIntensityMap,this.specularColor.copy(e.specularColor),this.specularColorMap=e.specularColorMap,this}};var Ka=class extends Gn{constructor(e){super(),this.isMeshLambertMaterial=!0,this.type="MeshLambertMaterial",this.color=new Le(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Le(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=_o,this.normalScale=new Ge(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Vn,this.combine=jl,this.reflectivity=1,this.envMapIntensity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.envMapIntensity=e.envMapIntensity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},Nl=class extends Gn{constructor(e){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=np,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(e)}copy(e){return super.copy(e),this.depthPacking=e.depthPacking,this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this}},Ul=class extends Gn{constructor(e){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(e)}copy(e){return super.copy(e),this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this}};function vs(s,e){return!s||s.constructor===e?s:typeof e.BYTES_PER_ELEMENT=="number"?new e(s):Array.prototype.slice.call(s)}function pl(s){return s!==void 0&&s.inTangents!==void 0&&s.outTangents!==void 0}function C0(s){function e(i,r){return s[i]-s[r]}let t=s.length,n=new Array(t);for(let i=0;i!==t;++i)n[i]=i;return n.sort(e),n}function yf(s,e,t){let n=s.length,i=new s.constructor(n);for(let r=0,a=0;a!==n;++r){let o=t[r]*e;for(let l=0;l!==e;++l)i[a++]=s[o+l]}return i}function P0(s,e,t,n){let i=1,r=s[0];for(;r!==void 0&&r[n]===void 0;)r=s[i++];if(r===void 0)return;let a=r[n];if(a!==void 0)if(Array.isArray(a))do a=r[n],a!==void 0&&(e.push(r.time),t.push(...a)),r=s[i++];while(r!==void 0);else if(a.toArray!==void 0)do a=r[n],a!==void 0&&(e.push(r.time),a.toArray(t,t.length)),r=s[i++];while(r!==void 0);else do a=r[n],a!==void 0&&(e.push(r.time),t.push(a)),r=s[i++];while(r!==void 0)}var zi=class{constructor(e,t,n,i){this.parameterPositions=e,this._cachedIndex=0,this.resultBuffer=i!==void 0?i:new t.constructor(n),this.sampleValues=t,this.valueSize=n,this.settings=null,this.DefaultSettings_={}}evaluate(e){let t=this.parameterPositions,n=this._cachedIndex,i=t[n],r=t[n-1];e:{t:{let a;n:{i:if(!(e<i)){for(let o=n+2;;){if(i===void 0){if(e<r)break i;return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}if(n===o)break;if(r=i,i=t[++n],e<i)break t}a=t.length;break n}if(!(e>=r)){let o=t[1];e<o&&(n=2,r=o);for(let l=n-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(n===l)break;if(i=r,r=t[--n-1],e>=r)break t}a=n,n=0;break n}break e}for(;n<a;){let o=n+a>>>1;e<t[o]?a=o:n=o+1}if(i=t[n],r=t[n-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===void 0)return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}this._cachedIndex=n,this.intervalChanged_(n,r,i)}return this.interpolate_(n,r,e,i)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i;for(let a=0;a!==i;++a)t[a]=n[r+a];return t}interpolate_(){throw new Error("THREE.Interpolant: Call to abstract method.")}intervalChanged_(){}},zl=class extends zi{constructor(e,t,n,i){super(e,t,n,i),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:Vs,endingEnd:Vs}}intervalChanged_(e,t,n){let i=this.parameterPositions,r=e-2,a=e+1,o=i[r],l=i[a];if(o===void 0)switch(this.getSettings_().endingStart){case Ws:r=e,o=2*t-n;break;case Da:r=i.length-2,o=t+i[r]-i[r+1];break;default:r=e,o=n}if(l===void 0)switch(this.getSettings_().endingEnd){case Ws:a=e,l=2*n-t;break;case Da:a=1,l=n+i[1]-i[0];break;default:a=e-1,l=t}let c=(n-t)*.5,h=this.valueSize;this._weightPrev=c/(t-o),this._weightNext=c/(l-n),this._offsetPrev=r*h,this._offsetNext=a*h}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=this._offsetPrev,d=this._offsetNext,u=this._weightPrev,p=this._weightNext,x=(n-t)/(i-t),M=x*x,m=M*x,f=-u*m+2*u*M-u*x,b=(1+u)*m+(-1.5-2*u)*M+(-.5+u)*x+1,E=(-1-p)*m+(1.5+p)*M+.5*x,g=p*m-p*M;for(let _=0;_!==o;++_)r[_]=f*a[h+_]+b*a[c+_]+E*a[l+_]+g*a[d+_];return r}},Ya=class extends zi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=(n-t)/(i-t),d=1-h;for(let u=0;u!==o;++u)r[u]=a[c+u]*d+a[l+u]*h;return r}},Ol=class extends zi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e){return this.copySampleValue_(e-1)}},kl=class extends zi{interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=e*o,c=l-o,h=this.inTangents,d=this.outTangents;if(!h||!d){let x=(n-t)/(i-t),M=1-x;for(let m=0;m!==o;++m)r[m]=a[c+m]*M+a[l+m]*x;return r}let u=o*2,p=e-1;for(let x=0;x!==o;++x){let M=a[c+x],m=a[l+x],f=p*u+x*2,b=d[f],E=d[f+1],g=e*u+x*2,_=h[g],w=h[g+1],T=D0(n,t,b,_,i);r[x]=gp(T,M,E,w,m)}return r}};function gp(s,e,t,n,i){let r=1-s;return r*r*r*e+3*r*r*s*t+3*r*s*s*n+s*s*s*i}function I0(s,e,t,n,i){let r=1-s;return 3*r*r*(t-e)+6*r*s*(n-t)+3*s*s*(i-n)}function D0(s,e,t,n,i){let r=(s-e)/(i-e);for(let a=0;a<8;a++){let o=gp(r,e,t,n,i)-s;if(Math.abs(o)<1e-10)break;let l=I0(r,e,t,n,i);if(Math.abs(l)<1e-10)break;r=Math.max(0,Math.min(1,r-o/l))}return r}var qn=class{constructor(e,t,n,i){if(e===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(t===void 0||t.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+e);this.name=e,this.times=vs(t,this.TimeBufferType),this.values=vs(n,this.ValueBufferType),this.setInterpolation(i||this.DefaultInterpolation)}static toJSON(e){let t=e.constructor,n;if(t.toJSON!==this.toJSON)n=t.toJSON(e);else{n={name:e.name,times:vs(e.times,Array),values:vs(e.values,Array)};let i=e.getInterpolation();i!==e.DefaultInterpolation&&(n.interpolation=i),pl(e.settings)&&(n.settings={inTangents:vs(e.settings.inTangents,Array),outTangents:vs(e.settings.outTangents,Array)})}return n.type=e.ValueTypeName,n}InterpolantFactoryMethodDiscrete(e){return new Ol(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodLinear(e){return new Ya(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodSmooth(e){return new zl(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodBezier(e){let t=new kl(this.times,this.values,this.getValueSize(),e);return this.settings&&(t.inTangents=this.settings.inTangents,t.outTangents=this.settings.outTangents),t}setInterpolation(e){let t;switch(e){case Xs:t=this.InterpolantFactoryMethodDiscrete;break;case js:t=this.InterpolantFactoryMethodLinear;break;case dl:t=this.InterpolantFactoryMethodSmooth;break;case qh:t=this.InterpolantFactoryMethodBezier;break}if(t===void 0){let n="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(e!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(n);return dt("KeyframeTrack:",n),this}return this.createInterpolant=t,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return Xs;case this.InterpolantFactoryMethodLinear:return js;case this.InterpolantFactoryMethodSmooth:return dl;case this.InterpolantFactoryMethodBezier:return qh}}getValueSize(){return this.values.length/this.times.length}shift(e){if(e!==0){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]+=e}return this}scale(e){if(e!==1){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]*=e;pl(this.settings)&&(_f(this.settings.inTangents,e),_f(this.settings.outTangents,e))}return this}trim(e,t){let n=this.times,i=n.length,r=0,a=i-1;for(;r!==i&&n[r]<e;)++r;for(;a!==-1&&n[a]>t;)--a;if(++a,r!==0||a!==i){r>=a&&(a=Math.max(a,1),r=a-1);let o=this.getValueSize();this.times=n.slice(r,a),this.values=this.values.slice(r*o,a*o)}return this}validate(){let e=!0,t=this.getValueSize();t-Math.floor(t)!==0&&(_t("KeyframeTrack: Invalid value size in track.",this),e=!1);let n=this.times,i=this.values,r=n.length;r===0&&(_t("KeyframeTrack: Track is empty.",this),e=!1);let a=null;for(let o=0;o!==r;o++){let l=n[o];if(typeof l=="number"&&isNaN(l)){_t("KeyframeTrack: Time is not a valid number.",this,o,l),e=!1;break}if(a!==null&&a>l){_t("KeyframeTrack: Out of order keys.",this,o,l,a),e=!1;break}a=l}if(i!==void 0&&Ng(i))for(let o=0,l=i.length;o!==l;++o){let c=i[o];if(isNaN(c)){_t("KeyframeTrack: Value is not a valid number.",this,o,c),e=!1;break}}return e}optimize(){let e=this.times.slice(),t=this.values.slice(),n=this.getValueSize(),i=this.getInterpolation()===dl,r=e.length-1,a=1;for(let o=1;o<r;++o){let l=!1,c=e[o],h=e[o+1];if(c!==h&&(o!==1||c!==e[0]))if(i)l=!0;else{let d=o*n,u=d-n,p=d+n;for(let x=0;x!==n;++x){let M=t[d+x];if(M!==t[u+x]||M!==t[p+x]){l=!0;break}}}if(l){if(o!==a){e[a]=e[o];let d=o*n,u=a*n;for(let p=0;p!==n;++p)t[u+p]=t[d+p]}++a}}if(r>0){e[a]=e[r];for(let o=r*n,l=a*n,c=0;c!==n;++c)t[l+c]=t[o+c];++a}return a!==e.length?(this.times=e.slice(0,a),this.values=t.slice(0,a*n)):(this.times=e,this.values=t),this}clone(){let e=this.times.slice(),t=this.values.slice(),n=this.constructor,i=new n(this.name,e,t);return i.createInterpolant=this.createInterpolant,pl(this.settings)&&(i.settings={inTangents:this.settings.inTangents.slice(),outTangents:this.settings.outTangents.slice()}),i}};function _f(s,e){for(let t=0,n=s.length;t!==n;t+=2)s[t]*=e}qn.prototype.ValueTypeName="";qn.prototype.TimeBufferType=Float32Array;qn.prototype.ValueBufferType=Float32Array;qn.prototype.DefaultInterpolation=js;var ns=class extends qn{constructor(e,t,n){super(e,t,n)}};ns.prototype.ValueTypeName="bool";ns.prototype.ValueBufferType=Array;ns.prototype.DefaultInterpolation=Xs;ns.prototype.InterpolantFactoryMethodLinear=void 0;ns.prototype.InterpolantFactoryMethodSmooth=void 0;var Ja=class extends qn{constructor(e,t,n,i){super(e,t,n,i)}};Ja.prototype.ValueTypeName="color";var is=class extends qn{constructor(e,t,n,i){super(e,t,n,i)}};is.prototype.ValueTypeName="number";var Bl=class extends zi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=(n-t)/(i-t),c=e*o;for(let h=c+o;c!==h;c+=4)nn.slerpFlat(r,0,a,c-o,a,c,l);return r}},ss=class extends qn{constructor(e,t,n,i){super(e,t,n,i)}InterpolantFactoryMethodLinear(e){return new Bl(this.times,this.values,this.getValueSize(),e)}};ss.prototype.ValueTypeName="quaternion";ss.prototype.InterpolantFactoryMethodSmooth=void 0;var rs=class extends qn{constructor(e,t,n){super(e,t,n)}};rs.prototype.ValueTypeName="string";rs.prototype.ValueBufferType=Array;rs.prototype.DefaultInterpolation=Xs;rs.prototype.InterpolantFactoryMethodLinear=void 0;rs.prototype.InterpolantFactoryMethodSmooth=void 0;var Es=class extends qn{constructor(e,t,n,i){super(e,t,n,i)}};Es.prototype.ValueTypeName="vector";var Qs=class{constructor(e="",t=-1,n=[],i=Nc){this.name=e,this.tracks=n,this.duration=t,this.blendMode=i,this.uuid=vi(),this.userData={},this.duration<0&&this.resetDuration()}static parse(e){let t=[],n=e.tracks,i=1/(e.fps||1);for(let a=0,o=n.length;a!==o;++a)t.push(F0(n[a]).scale(i));let r=new this(e.name,e.duration,t,e.blendMode);return r.uuid=e.uuid,r.userData=JSON.parse(e.userData||"{}"),r}static toJSON(e){let t=[],n=e.tracks,i={name:e.name,duration:e.duration,tracks:t,uuid:e.uuid,blendMode:e.blendMode,userData:JSON.stringify(e.userData)};for(let r=0,a=n.length;r!==a;++r)t.push(qn.toJSON(n[r]));return i}static CreateFromMorphTargetSequence(e,t,n,i){let r=t.length,a=[];for(let o=0;o<r;o++){let l=[],c=[];l.push((o+r-1)%r,o,(o+1)%r),c.push(0,1,0);let h=C0(l);l=yf(l,1,h),c=yf(c,1,h),!i&&l[0]===0&&(l.push(r),c.push(c[0])),a.push(new is(".morphTargetInfluences["+t[o].name+"]",l,c).scale(1/n))}return new this(e,-1,a)}static findByName(e,t){let n=e;if(!Array.isArray(e)){let i=e;n=i.geometry&&i.geometry.animations||i.animations}for(let i=0;i<n.length;i++)if(n[i].name===t)return n[i];return null}static CreateClipsFromMorphTargetSequences(e,t,n){let i={},r=/^([\w-]*?)([\d]+)$/;for(let o=0,l=e.length;o<l;o++){let c=e[o],h=c.name.match(r);if(h&&h.length>1){let d=h[1],u=i[d];u||(i[d]=u=[]),u.push(c)}}let a=[];for(let o in i)a.push(this.CreateFromMorphTargetSequence(o,i[o],t,n));return a}resetDuration(){let e=this.tracks,t=0;for(let n=0,i=e.length;n!==i;++n){let r=this.tracks[n];t=Math.max(t,r.times[r.times.length-1])}return this.duration=t,this}trim(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].trim(0,this.duration);return this}validate(){let e=!0;for(let t=0;t<this.tracks.length;t++)e=e&&this.tracks[t].validate();return e}optimize(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].optimize();return this}clone(){let e=[];for(let n=0;n<this.tracks.length;n++)e.push(this.tracks[n].clone());let t=new this.constructor(this.name,this.duration,e,this.blendMode);return t.userData=JSON.parse(JSON.stringify(this.userData)),t}toJSON(){return this.constructor.toJSON(this)}};function L0(s){switch(s.toLowerCase()){case"scalar":case"double":case"float":case"number":case"integer":return is;case"vector":case"vector2":case"vector3":case"vector4":return Es;case"color":return Ja;case"quaternion":return ss;case"bool":case"boolean":return ns;case"string":return rs}throw new Error("THREE.KeyframeTrack: Unsupported typeName: "+s)}function F0(s){if(s.type===void 0)throw new Error("THREE.KeyframeTrack: track type undefined, can not parse");let e=L0(s.type);if(s.times===void 0){let n=[],i=[];P0(s.keys,n,i,"value"),s.times=n,s.values=i}let t;return e.parse!==void 0?t=e.parse(s):t=new e(s.name,s.times,s.values,s.interpolation),pl(s.settings)&&(t.settings={inTangents:vs(s.settings.inTangents,Float32Array),outTangents:vs(s.settings.outTangents,Float32Array)}),t}var Ii={enabled:!1,files:{},add:function(s,e){this.enabled!==!1&&(Mf(s)||(this.files[s]=e))},get:function(s){if(this.enabled!==!1&&!Mf(s))return this.files[s]},remove:function(s){delete this.files[s]},clear:function(){this.files={}}};function Mf(s){try{let e=s.slice(s.indexOf(":")+1);return new URL(e).protocol==="blob:"}catch{return!1}}var Hl=class{constructor(e,t,n){let i=this,r=!1,a=0,o=0,l,c=[];this.onStart=void 0,this.onLoad=e,this.onProgress=t,this.onError=n,this._abortController=null,this.itemStart=function(h){o++,r===!1&&i.onStart!==void 0&&i.onStart(h,a,o),r=!0},this.itemEnd=function(h){a++,i.onProgress!==void 0&&i.onProgress(h,a,o),a===o&&(r=!1,i.onLoad!==void 0&&i.onLoad())},this.itemError=function(h){i.onError!==void 0&&i.onError(h)},this.resolveURL=function(h){return h=h.normalize("NFC"),l?l(h):h},this.setURLModifier=function(h){return l=h,this},this.addHandler=function(h,d){return c.push(h,d),this},this.removeHandler=function(h){let d=c.indexOf(h);return d!==-1&&c.splice(d,2),this},this.getHandler=function(h){for(let d=0,u=c.length;d<u;d+=2){let p=c[d],x=c[d+1];if(p.global&&(p.lastIndex=0),p.test(h))return x}return null},this.abort=function(){return this.abortController.abort(),this._abortController=null,this}}get abortController(){return this._abortController||(this._abortController=new AbortController),this._abortController}},xp=new Hl,Oi=class{constructor(e){this.manager=e!==void 0?e:xp,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}load(){}loadAsync(e,t){let n=this;return new Promise(function(i,r){n.load(e,i,t,r)})}parse(){}setCrossOrigin(e){return this.crossOrigin=e,this}setWithCredentials(e){return this.withCredentials=e,this}setPath(e){return this.path=e,this}setResourcePath(e){return this.resourcePath=e,this}setRequestHeader(e){return this.requestHeader=e,this}abort(){return this}};Oi.DEFAULT_MATERIAL_NAME="__DEFAULT";var Qi={},Kh=class extends Error{constructor(e,t){super(e),this.response=t}},Kr=class extends Oi{constructor(e){super(e),this.mimeType="",this.responseType="",this._abortController=new AbortController}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=Ii.get(`file:${e}`);if(r!==void 0){this.manager.itemStart(e),setTimeout(()=>{t&&t(r),this.manager.itemEnd(e)},0);return}if(Qi[e]!==void 0){Qi[e].push({onLoad:t,onProgress:n,onError:i});return}Qi[e]=[],Qi[e].push({onLoad:t,onProgress:n,onError:i});let a=new Request(e,{headers:new Headers(this.requestHeader),credentials:this.withCredentials?"include":"same-origin",signal:typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal}),o=this.mimeType,l=this.responseType;fetch(a).then(c=>{if(c.status===200||c.status===0){if(c.status===0&&dt("FileLoader: HTTP Status 0 received."),typeof ReadableStream>"u"||c.body===void 0||c.body.getReader===void 0)return c;let h=Qi[e],d=c.body.getReader(),u=c.headers.get("X-File-Size")||c.headers.get("Content-Length"),p=u?parseInt(u):0,x=p!==0,M=0,m=new ReadableStream({start(f){b();function b(){d.read().then(({done:E,value:g})=>{if(E)f.close();else{M+=g.byteLength;let _=new ProgressEvent("progress",{lengthComputable:x,loaded:M,total:p});for(let w=0,T=h.length;w<T;w++){let y=h[w];y.onProgress&&y.onProgress(_)}f.enqueue(g),b()}},E=>{f.error(E)})}}});return new Response(m)}else throw new Kh(`fetch for "${c.url}" responded with ${c.status}: ${c.statusText}`,c)}).then(c=>{switch(l){case"arraybuffer":return c.arrayBuffer();case"blob":return c.blob();case"document":return c.text().then(h=>new DOMParser().parseFromString(h,o));case"json":return c.json();default:if(o==="")return c.text();{let d=/charset="?([^;"\s]*)"?/i.exec(o),u=d&&d[1]?d[1].toLowerCase():void 0,p=new TextDecoder(u);return c.arrayBuffer().then(x=>p.decode(x))}}}).then(c=>{Ii.add(`file:${e}`,c);let h=Qi[e];delete Qi[e];for(let d=0,u=h.length;d<u;d++){let p=h[d];p.onLoad&&p.onLoad(c)}}).catch(c=>{let h=Qi[e];if(h===void 0)throw this.manager.itemError(e),c;delete Qi[e];for(let d=0,u=h.length;d<u;d++){let p=h[d];p.onError&&p.onError(c)}this.manager.itemError(e)}).finally(()=>{this.manager.itemEnd(e)}),this.manager.itemStart(e)}setResponseType(e){return this.responseType=e,this}setMimeType(e){return this.mimeType=e,this}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var Tr=new WeakMap,Gl=class extends Oi{constructor(e){super(e)}load(e,t,n,i){this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,a=Ii.get(`image:${e}`);if(a!==void 0){if(a.complete===!0)r.manager.itemStart(e),setTimeout(function(){t&&t(a),r.manager.itemEnd(e)},0);else{let d=Tr.get(a);d===void 0&&(d=[],Tr.set(a,d)),d.push({onLoad:t,onError:i})}return a}let o=Lr("img");function l(){h(),t&&t(this);let d=Tr.get(this)||[];for(let u=0;u<d.length;u++){let p=d[u];p.onLoad&&p.onLoad(this)}Tr.delete(this),r.manager.itemEnd(e)}function c(d){h(),i&&i(d),Ii.remove(`image:${e}`);let u=Tr.get(this)||[];for(let p=0;p<u.length;p++){let x=u[p];x.onError&&x.onError(d)}Tr.delete(this),r.manager.itemError(e),r.manager.itemEnd(e)}function h(){o.removeEventListener("load",l,!1),o.removeEventListener("error",c,!1)}return o.addEventListener("load",l,!1),o.addEventListener("error",c,!1),e.slice(0,5)!=="data:"&&this.crossOrigin!==void 0&&(o.crossOrigin=this.crossOrigin),Ii.add(`image:${e}`,o),r.manager.itemStart(e),o.src=e,o}};var Za=class extends Oi{constructor(e){super(e)}load(e,t,n,i){let r=new hn,a=new Gl(this.manager);return a.setCrossOrigin(this.crossOrigin),a.setPath(this.path),a.load(e,function(o){r.image=o,r.needsUpdate=!0,t!==void 0&&t(r)},n,i),r}},er=class extends Yt{constructor(e,t=1){super(),this.isLight=!0,this.type="Light",this.color=new Le(e),this.intensity=t}copy(e,t){return super.copy(e,t),this.color.copy(e.color),this.intensity=e.intensity,this}toJSON(e){let t=super.toJSON(e);return t.object.color=this.color.getHex(),t.object.intensity=this.intensity,t}},$a=class extends er{constructor(e,t,n){super(e,n),this.isHemisphereLight=!0,this.type="HemisphereLight",this.position.copy(Yt.DEFAULT_UP),this.updateMatrix(),this.groundColor=new Le(t)}copy(e,t){return super.copy(e,t),this.groundColor.copy(e.groundColor),this}toJSON(e){let t=super.toJSON(e);return t.object.groundColor=this.groundColor.getHex(),t}},Gh=new yt,wf=new U,Sf=new U,Yr=class{constructor(e){this.camera=e,this.intensity=1,this.bias=0,this.biasNode=null,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new Ge(512,512),this.mapType=Xn,this.map=null,this.mapPass=null,this.matrix=new yt,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new Hr,this._frameExtents=new Ge(1,1),this._viewportCount=1,this._viewports=[new Ot(0,0,1,1)]}getViewportCount(){return this._viewportCount}getCamera(){return this.camera}getFrustum(){return this._frustum}updateMatrices(e){let t=this.camera;wf.setFromMatrixPosition(e.matrixWorld),t.position.copy(wf),Sf.setFromMatrixPosition(e.target.matrixWorld),t.lookAt(Sf),t.updateMatrixWorld(),this._updateMatrix(t,this.matrix,this._frustum)}_updateMatrix(e,t,n,i){Gh.multiplyMatrices(e.projectionMatrix,e.matrixWorldInverse),n.setFromProjectionMatrix(Gh,e.coordinateSystem,e.reversedDepth);let r=this._frameExtents,a=i?i.z/r.x:1,o=i?i.w/r.y:1,l=i?i.x/r.x:0,c=i?i.y/r.y:0;e.coordinateSystem===Dr||e.reversedDepth?t.set(.5*a,0,0,.5*a+l,0,.5*o,0,.5*o+c,0,0,1,0,0,0,0,1):t.set(.5*a,0,0,.5*a+l,0,.5*o,0,.5*o+c,0,0,.5,.5,0,0,0,1),t.multiply(Gh)}getViewport(e){return this._viewports[e]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(e){return this.camera=e.camera.clone(),this.intensity=e.intensity,this.bias=e.bias,this.radius=e.radius,this.autoUpdate=e.autoUpdate,this.needsUpdate=e.needsUpdate,this.normalBias=e.normalBias,this.blurSamples=e.blurSamples,this.mapSize.copy(e.mapSize),this.biasNode=e.biasNode,this}clone(){return new this.constructor().copy(this)}toJSON(){let e={};return e.intensity=this.intensity,e.bias=this.bias,e.normalBias=this.normalBias,e.radius=this.radius,e.blurSamples=this.blurSamples,e.mapSize=this.mapSize.toArray(),e.camera=this.camera.toJSON(!1).object,delete e.camera.matrix,e}},hl=new U,ul=new nn,Pi=new U,Qa=class extends Yt{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new yt,this.projectionMatrix=new yt,this.projectionMatrixInverse=new yt,this.coordinateSystem=bi,this._reversedDepth=!1}get reversedDepth(){return this._reversedDepth}copy(e,t){return super.copy(e,t),this.matrixWorldInverse.copy(e.matrixWorldInverse),this.projectionMatrix.copy(e.projectionMatrix),this.projectionMatrixInverse.copy(e.projectionMatrixInverse),this.coordinateSystem=e.coordinateSystem,this}getWorldDirection(e){return super.getWorldDirection(e).negate()}updateMatrixWorld(e){super.updateMatrixWorld(e),this.matrixWorld.decompose(hl,ul,Pi),Pi.x===1&&Pi.y===1&&Pi.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(hl,ul,Pi.set(1,1,1)).invert()}updateWorldMatrix(e,t,n=!1){super.updateWorldMatrix(e,t,n),this.matrixWorld.decompose(hl,ul,Pi),Pi.x===1&&Pi.y===1&&Pi.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(hl,ul,Pi.set(1,1,1)).invert()}clone(){return new this.constructor().copy(this)}},bs=new U,Ef=new Ge,Tf=new Ge,gn=class extends Qa{constructor(e=50,t=1,n=.1,i=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=e,this.zoom=1,this.near=n,this.far=i,this.focus=10,this.aspect=t,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.fov=e.fov,this.zoom=e.zoom,this.near=e.near,this.far=e.far,this.focus=e.focus,this.aspect=e.aspect,this.view=e.view===null?null:Object.assign({},e.view),this.filmGauge=e.filmGauge,this.filmOffset=e.filmOffset,this}setFocalLength(e){let t=.5*this.getFilmHeight()/e;this.fov=Ks*2*Math.atan(t),this.updateProjectionMatrix()}getFocalLength(){let e=Math.tan(Ra*.5*this.fov);return .5*this.getFilmHeight()/e}getEffectiveFOV(){return Ks*2*Math.atan(Math.tan(Ra*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(e,t,n){bs.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),t.set(bs.x,bs.y).multiplyScalar(-e/bs.z),bs.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),n.set(bs.x,bs.y).multiplyScalar(-e/bs.z)}getViewSize(e,t){return this.getViewBounds(e,Ef,Tf),t.subVectors(Tf,Ef)}setViewOffset(e,t,n,i,r,a){this.aspect=e/t,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=a,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=this.near,t=e*Math.tan(Ra*.5*this.fov)/this.zoom,n=2*t,i=this.aspect*n,r=-.5*i,a=this.view;if(this.view!==null&&this.view.enabled){let l=a.fullWidth,c=a.fullHeight;r+=a.offsetX*i/l,t-=a.offsetY*n/c,i*=a.width/l,n*=a.height/c}let o=this.filmOffset;o!==0&&(r+=e*o/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+i,t,t-n,e,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.fov=this.fov,t.object.zoom=this.zoom,t.object.near=this.near,t.object.far=this.far,t.object.focus=this.focus,t.object.aspect=this.aspect,this.view!==null&&(t.object.view=Object.assign({},this.view)),t.object.filmGauge=this.filmGauge,t.object.filmOffset=this.filmOffset,t}},Yh=class extends Yr{constructor(){super(new gn(50,1,.5,500)),this.isSpotLightShadow=!0,this.focus=1,this.aspect=1}updateMatrices(e){let t=this.camera,n=Ks*2*e.angle*this.focus,i=this.mapSize.width/this.mapSize.height*this.aspect,r=e.distance||t.far;(n!==t.fov||i!==t.aspect||r!==t.far)&&(t.fov=n,t.aspect=i,t.far=r,t.updateProjectionMatrix()),super.updateMatrices(e)}copy(e){return super.copy(e),this.focus=e.focus,this.aspect=e.aspect,this}toJSON(){let e=super.toJSON();return e.focus=this.focus,e.aspect=this.aspect,e}},eo=class extends er{constructor(e,t,n=0,i=Math.PI/3,r=0,a=2){super(e,t),this.isSpotLight=!0,this.type="SpotLight",this.position.copy(Yt.DEFAULT_UP),this.updateMatrix(),this.target=new Yt,this.distance=n,this.angle=i,this.penumbra=r,this.decay=a,this.map=null,this.shadow=new Yh}get power(){return this.intensity*Math.PI}set power(e){this.intensity=e/Math.PI}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.angle=e.angle,this.penumbra=e.penumbra,this.decay=e.decay,this.target=e.target.clone(),this.map=e.map,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.angle=this.angle,t.object.decay=this.decay,t.object.penumbra=this.penumbra,t.object.target=this.target.uuid,this.map&&this.map.isTexture&&(t.object.map=this.map.toJSON(e).uuid),t.object.shadow=this.shadow.toJSON(),t}},Jh=class extends Yr{constructor(){super(new gn(90,1,.5,500)),this.isPointLightShadow=!0}},wi=class extends er{constructor(e,t,n=0,i=2){super(e,t),this.isPointLight=!0,this.type="PointLight",this.distance=n,this.decay=i,this.shadow=new Jh}get power(){return this.intensity*4*Math.PI}set power(e){this.intensity=e/(4*Math.PI)}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.decay=e.decay,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.decay=this.decay,t.object.shadow=this.shadow.toJSON(),t}},ki=class extends Qa{constructor(e=-1,t=1,n=1,i=-1,r=.1,a=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=e,this.right=t,this.top=n,this.bottom=i,this.near=r,this.far=a,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.left=e.left,this.right=e.right,this.top=e.top,this.bottom=e.bottom,this.near=e.near,this.far=e.far,this.zoom=e.zoom,this.view=e.view===null?null:Object.assign({},e.view),this}setViewOffset(e,t,n,i,r,a){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=a,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=(this.right-this.left)/(2*this.zoom),t=(this.top-this.bottom)/(2*this.zoom),n=(this.right+this.left)/2,i=(this.top+this.bottom)/2,r=n-e,a=n+e,o=i+t,l=i-t;if(this.view!==null&&this.view.enabled){let c=(this.right-this.left)/this.view.fullWidth/this.zoom,h=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=c*this.view.offsetX,a=r+c*this.view.width,o-=h*this.view.offsetY,l=o-h*this.view.height}this.projectionMatrix.makeOrthographic(r,a,o,l,this.near,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.zoom=this.zoom,t.object.left=this.left,t.object.right=this.right,t.object.top=this.top,t.object.bottom=this.bottom,t.object.near=this.near,t.object.far=this.far,this.view!==null&&(t.object.view=Object.assign({},this.view)),t}},Zh=class extends Yr{constructor(){super(new ki(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},as=class extends er{constructor(e,t){super(e,t),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(Yt.DEFAULT_UP),this.updateMatrix(),this.target=new Yt,this.shadow=new Zh}dispose(){super.dispose(),this.shadow.dispose()}copy(e){return super.copy(e),this.target=e.target.clone(),this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.shadow=this.shadow.toJSON(),t.object.target=this.target.uuid,t}};var os=class{static extractUrlBase(e){let t=e.lastIndexOf("/");return t===-1?"./":e.slice(0,t+1)}static resolveURL(e,t){return typeof e!="string"||e===""?"":(/^https?:\/\//i.test(t)&&/^\//.test(e)&&(t=t.replace(/(^https?:\/\/[^\/]+).*/i,"$1")),/^(https?:)?\/\//i.test(e)||/^data:.*,.*$/i.test(e)||/^blob:.*$/i.test(e)?e:t+e)}};var Vh=new WeakMap,to=class extends Oi{constructor(e){super(e),this.isImageBitmapLoader=!0,typeof createImageBitmap>"u"&&dt("ImageBitmapLoader: createImageBitmap() not supported."),typeof fetch>"u"&&dt("ImageBitmapLoader: fetch() not supported."),this.options={premultiplyAlpha:"none"},this._abortController=new AbortController}setOptions(e){return this.options=e,this}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,a=Ii.get(`image-bitmap:${e}`);if(a!==void 0){if(r.manager.itemStart(e),a.then){a.then(c=>{Vh.has(a)===!0?(i&&i(Vh.get(a)),r.manager.itemError(e),r.manager.itemEnd(e)):(t&&t(c),r.manager.itemEnd(e))});return}setTimeout(function(){t&&t(a),r.manager.itemEnd(e)},0);return}let o={};o.credentials=this.crossOrigin==="anonymous"?"same-origin":"include",o.headers=this.requestHeader,o.signal=typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal;let l=fetch(e,o).then(function(c){return c.blob()}).then(function(c){return createImageBitmap(c,Object.assign({},r.options,{colorSpaceConversion:"none"}))}).then(function(c){return Ii.add(`image-bitmap:${e}`,c),t&&t(c),r.manager.itemEnd(e),c}).catch(function(c){i&&i(c),Vh.set(l,c),Ii.remove(`image-bitmap:${e}`),r.manager.itemError(e),r.manager.itemEnd(e)});Ii.add(`image-bitmap:${e}`,l),r.manager.itemStart(e)}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var Ar=-90,Rr=1,Vl=class extends Yt{constructor(e,t,n){super(),this.type="CubeCamera",this.renderTarget=n,this.coordinateSystem=null,this.activeMipmapLevel=0;let i=new gn(Ar,Rr,e,t);i.layers=this.layers,this.add(i);let r=new gn(Ar,Rr,e,t);r.layers=this.layers,this.add(r);let a=new gn(Ar,Rr,e,t);a.layers=this.layers,this.add(a);let o=new gn(Ar,Rr,e,t);o.layers=this.layers,this.add(o);let l=new gn(Ar,Rr,e,t);l.layers=this.layers,this.add(l);let c=new gn(Ar,Rr,e,t);c.layers=this.layers,this.add(c)}updateCoordinateSystem(){let e=this.coordinateSystem,t=this.children.concat(),[n,i,r,a,o,l]=t;for(let c of t)this.remove(c);if(e===bi)n.up.set(0,1,0),n.lookAt(1,0,0),i.up.set(0,1,0),i.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),a.up.set(0,0,1),a.lookAt(0,-1,0),o.up.set(0,1,0),o.lookAt(0,0,1),l.up.set(0,1,0),l.lookAt(0,0,-1);else if(e===Dr)n.up.set(0,-1,0),n.lookAt(-1,0,0),i.up.set(0,-1,0),i.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),a.up.set(0,0,-1),a.lookAt(0,-1,0),o.up.set(0,-1,0),o.lookAt(0,0,1),l.up.set(0,-1,0),l.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+e);for(let c of t)this.add(c),c.updateMatrixWorld()}update(e,t){this.parent===null&&this.updateMatrixWorld();let{renderTarget:n,activeMipmapLevel:i}=this;this.coordinateSystem!==e.coordinateSystem&&(this.coordinateSystem=e.coordinateSystem,this.updateCoordinateSystem());let[r,a,o,l,c,h]=this.children,d=e.getRenderTarget(),u=e.getActiveCubeFace(),p=e.getActiveMipmapLevel(),x=e.xr.enabled;e.xr.enabled=!1;let M=n.texture.generateMipmaps;n.texture.generateMipmaps=!1;let m=!1;e.isWebGLRenderer===!0?m=e.state.buffers.depth.getReversed():m=e.reversedDepthBuffer,e.setRenderTarget(n,0,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,r),e.setRenderTarget(n,1,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,a),e.setRenderTarget(n,2,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,o),e.setRenderTarget(n,3,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,l),e.setRenderTarget(n,4,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,c),n.texture.generateMipmaps=M,e.setRenderTarget(n,5,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,h),e.setRenderTarget(d,u,p),e.xr.enabled=x,n.texture.needsPMREMUpdate=!0}},Wl=class extends gn{constructor(e=[]){super(),this.isArrayCamera=!0,this.isMultiViewCamera=!1,this.cameras=e}},no=class{constructor(){this._previousTime=0,this._currentTime=0,this._startTime=performance.now(),this._delta=0,this._elapsed=0,this._timescale=1,this._document=null,this._pageVisibilityHandler=null}connect(e){this._document=e,e.hidden!==void 0&&(this._pageVisibilityHandler=N0.bind(this),e.addEventListener("visibilitychange",this._pageVisibilityHandler,!1))}disconnect(){this._pageVisibilityHandler!==null&&(this._document.removeEventListener("visibilitychange",this._pageVisibilityHandler),this._pageVisibilityHandler=null),this._document=null}getDelta(){return this._delta/1e3}getElapsed(){return this._elapsed/1e3}getTimescale(){return this._timescale}setTimescale(e){return this._timescale=e,this}reset(){return this._currentTime=performance.now()-this._startTime,this}dispose(){this.disconnect()}update(e){return this._pageVisibilityHandler!==null&&this._document.hidden===!0?this._delta=0:(this._previousTime=this._currentTime,this._currentTime=(e!==void 0?e:performance.now())-this._startTime,this._delta=(this._currentTime-this._previousTime)*this._timescale,this._elapsed+=this._delta),this}};function N0(){this._document.hidden===!1&&this.reset()}var ql=class{constructor(e,t,n){this.binding=e,this.valueSize=n;let i,r,a;switch(t){case"quaternion":i=this._slerp,r=this._slerpAdditive,a=this._setAdditiveIdentityQuaternion,this.buffer=new Float64Array(n*6),this._workIndex=5;break;case"string":case"bool":i=this._select,r=this._select,a=this._setAdditiveIdentityOther,this.buffer=new Array(n*5);break;default:i=this._lerp,r=this._lerpAdditive,a=this._setAdditiveIdentityNumeric,this.buffer=new Float64Array(n*5)}this._mixBufferRegion=i,this._mixBufferRegionAdditive=r,this._setIdentity=a,this._origIndex=3,this._addIndex=4,this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,this.useCount=0,this.referenceCount=0}accumulate(e,t){let n=this.buffer,i=this.valueSize,r=e*i+i,a=this.cumulativeWeight;if(a===0){for(let o=0;o!==i;++o)n[r+o]=n[o];a=t}else{a+=t;let o=t/a;this._mixBufferRegion(n,r,0,o,i)}this.cumulativeWeight=a}accumulateAdditive(e){let t=this.buffer,n=this.valueSize,i=n*this._addIndex;this.cumulativeWeightAdditive===0&&this._setIdentity(),this._mixBufferRegionAdditive(t,i,0,e,n),this.cumulativeWeightAdditive+=e}apply(e){let t=this.valueSize,n=this.buffer,i=e*t+t,r=this.cumulativeWeight,a=this.cumulativeWeightAdditive,o=this.binding;if(this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,r<1){let l=t*this._origIndex;this._mixBufferRegion(n,i,l,1-r,t)}a>0&&this._mixBufferRegionAdditive(n,i,this._addIndex*t,1,t);for(let l=t,c=t+t;l!==c;++l)if(n[l]!==n[l+t]){o.setValue(n,i);break}}saveOriginalState(){let e=this.binding,t=this.buffer,n=this.valueSize,i=n*this._origIndex;e.getValue(t,i);for(let r=n,a=i;r!==a;++r)t[r]=t[i+r%n];this._setIdentity(),this.cumulativeWeight=0,this.cumulativeWeightAdditive=0}restoreOriginalState(){let e=this.valueSize*3;this.binding.setValue(this.buffer,e)}_setAdditiveIdentityNumeric(){let e=this._addIndex*this.valueSize,t=e+this.valueSize;for(let n=e;n<t;n++)this.buffer[n]=0}_setAdditiveIdentityQuaternion(){this._setAdditiveIdentityNumeric(),this.buffer[this._addIndex*this.valueSize+3]=1}_setAdditiveIdentityOther(){let e=this._origIndex*this.valueSize,t=this._addIndex*this.valueSize;for(let n=0;n<this.valueSize;n++)this.buffer[t+n]=this.buffer[e+n]}_select(e,t,n,i,r){if(i>=.5)for(let a=0;a!==r;++a)e[t+a]=e[n+a]}_slerp(e,t,n,i){nn.slerpFlat(e,t,e,t,e,n,i)}_slerpAdditive(e,t,n,i,r){let a=this._workIndex*r;nn.multiplyQuaternionsFlat(e,a,e,t,e,n),nn.slerpFlat(e,t,e,t,e,a,i)}_lerp(e,t,n,i,r){let a=1-i;for(let o=0;o!==r;++o){let l=t+o;e[l]=e[l]*a+e[n+o]*i}}_lerpAdditive(e,t,n,i,r){for(let a=0;a!==r;++a){let o=t+a;e[o]=e[o]+e[n+a]*i}}},vu="\\[\\]\\.:\\/",U0=new RegExp("["+vu+"]","g"),yu="[^"+vu+"]",z0="[^"+vu.replace("\\.","")+"]",O0=/((?:WC+[\/:])*)/.source.replace("WC",yu),k0=/(WCOD+)?/.source.replace("WCOD",z0),B0=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",yu),H0=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",yu),G0=new RegExp("^"+O0+k0+B0+H0+"$"),V0=["material","materials","bones","map"],$h=class{constructor(e,t,n){let i=n||tn.parseTrackName(t);this._targetGroup=e,this._bindings=e.subscribe_(t,i)}getValue(e,t){this.bind();let n=this._targetGroup.nCachedObjects_,i=this._bindings[n];i!==void 0&&i.getValue(e,t)}setValue(e,t){let n=this._bindings;for(let i=this._targetGroup.nCachedObjects_,r=n.length;i!==r;++i)n[i].setValue(e,t)}bind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].bind()}unbind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].unbind()}},tn=class s{constructor(e,t,n){this.path=t,this.parsedPath=n||s.parseTrackName(t),this.node=s.findNode(e,this.parsedPath.nodeName),this.rootNode=e,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(e,t,n){return e&&e.isAnimationObjectGroup?new s.Composite(e,t,n):new s(e,t,n)}static sanitizeNodeName(e){return e.replace(/\s/g,"_").replace(U0,"")}static parseTrackName(e){let t=G0.exec(e);if(t===null)throw new Error("THREE.PropertyBinding: Cannot parse trackName: "+e);let n={nodeName:t[2],objectName:t[3],objectIndex:t[4],propertyName:t[5],propertyIndex:t[6]},i=n.nodeName&&n.nodeName.lastIndexOf(".");if(i!==void 0&&i!==-1){let r=n.nodeName.substring(i+1);V0.indexOf(r)!==-1&&(n.nodeName=n.nodeName.substring(0,i),n.objectName=r)}if(n.propertyName===null||n.propertyName.length===0)throw new Error("THREE.PropertyBinding: can not parse propertyName from trackName: "+e);return n}static findNode(e,t){if(t===void 0||t===""||t==="."||t===-1||t===e.name||t===e.uuid)return e;if(e.skeleton){let n=e.skeleton.getBoneByName(t);if(n!==void 0)return n}if(e.children){let n=function(r){for(let a=0;a<r.length;a++){let o=r[a];if(o.name===t||o.uuid===t)return o;let l=n(o.children);if(l)return l}return null},i=n(e.children);if(i)return i}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(e,t){e[t]=this.targetObject[this.propertyName]}_getValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)e[t++]=n[i]}_getValue_arrayElement(e,t){e[t]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(e,t){this.resolvedProperty.toArray(e,t)}_setValue_direct(e,t){this.targetObject[this.propertyName]=e[t]}_setValue_direct_setNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++]}_setValue_array_setNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(e,t){this.resolvedProperty[this.propertyIndex]=e[t]}_setValue_arrayElement_setNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(e,t){this.resolvedProperty.fromArray(e,t)}_setValue_fromArray_setNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(e,t){this.bind(),this.getValue(e,t)}_setValue_unbound(e,t){this.bind(),this.setValue(e,t)}bind(){let e=this.node,t=this.parsedPath,n=t.objectName,i=t.propertyName,r=t.propertyIndex;if(e||(e=s.findNode(this.rootNode,t.nodeName),this.node=e),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!e){dt("PropertyBinding: No target node found for track: "+this.path+".");return}if(n){let c=t.objectIndex;switch(n){case"materials":if(!e.material){_t("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.materials){_t("PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}e=e.material.materials;break;case"bones":if(!e.skeleton){_t("PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}e=e.skeleton.bones;for(let h=0;h<e.length;h++)if(e[h].name===c){c=h;break}break;case"map":if("map"in e){e=e.map;break}if(!e.material){_t("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.map){_t("PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}e=e.material.map;break;default:if(e[n]===void 0){_t("PropertyBinding: Can not bind to objectName of node undefined.",this);return}e=e[n]}if(c!==void 0){if(e[c]===void 0){_t("PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,e);return}e=e[c]}}let a=e[i];if(a===void 0){let c=t.nodeName;_t("PropertyBinding: Trying to update property for track: "+c+"."+i+" but it wasn't found.",e);return}let o=this.Versioning.None;this.targetObject=e,e.isMaterial===!0?o=this.Versioning.NeedsUpdate:e.isObject3D===!0&&(o=this.Versioning.MatrixWorldNeedsUpdate);let l=this.BindingType.Direct;if(r!==void 0){if(i==="morphTargetInfluences"){if(!e.geometry){_t("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!e.geometry.morphAttributes){_t("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}e.morphTargetDictionary[r]!==void 0&&(r=e.morphTargetDictionary[r])}l=this.BindingType.ArrayElement,this.resolvedProperty=a,this.propertyIndex=r}else a.fromArray!==void 0&&a.toArray!==void 0?(l=this.BindingType.HasFromToArray,this.resolvedProperty=a):Array.isArray(a)?(l=this.BindingType.EntireArray,this.resolvedProperty=a):this.propertyName=i;this.getValue=this.GetterByBindingType[l],this.setValue=this.SetterByBindingTypeAndVersioning[l][o]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};tn.Composite=$h;tn.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};tn.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};tn.prototype.GetterByBindingType=[tn.prototype._getValue_direct,tn.prototype._getValue_array,tn.prototype._getValue_arrayElement,tn.prototype._getValue_toArray];tn.prototype.SetterByBindingTypeAndVersioning=[[tn.prototype._setValue_direct,tn.prototype._setValue_direct_setNeedsUpdate,tn.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[tn.prototype._setValue_array,tn.prototype._setValue_array_setNeedsUpdate,tn.prototype._setValue_array_setMatrixWorldNeedsUpdate],[tn.prototype._setValue_arrayElement,tn.prototype._setValue_arrayElement_setNeedsUpdate,tn.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[tn.prototype._setValue_fromArray,tn.prototype._setValue_fromArray_setNeedsUpdate,tn.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var Xl=class{constructor(e,t,n=null,i=t.blendMode){this._mixer=e,this._clip=t,this._localRoot=n,this.blendMode=i;let r=t.tracks,a=r.length,o=new Array(a),l={endingStart:Vs,endingEnd:Vs};for(let c=0;c!==a;++c){let h=r[c].createInterpolant(null);o[c]=h,h.settings=l}this._interpolantSettings=l,this._interpolants=o,this._propertyBindings=new Array(a),this._cacheIndex=null,this._byClipCacheIndex=null,this._timeScaleInterpolant=null,this._restoreTimeScale=null,this._weightInterpolant=null,this.loop=Qf,this._loopCount=-1,this._startTime=null,this.time=0,this.timeScale=1,this._effectiveTimeScale=1,this.weight=1,this._effectiveWeight=1,this.repetitions=1/0,this.paused=!1,this.enabled=!0,this.clampWhenFinished=!1,this.zeroSlopeAtStart=!0,this.zeroSlopeAtEnd=!0}play(){return this._mixer._activateAction(this),this}stop(){return this._mixer._deactivateAction(this),this.reset()}reset(){return this.paused=!1,this.enabled=!0,this.time=0,this._loopCount=-1,this._startTime=null,this.stopFading().stopWarping()}isRunning(){return this.enabled&&!this.paused&&this.timeScale!==0&&this._startTime===null&&this._mixer._isActiveAction(this)}isScheduled(){return this._mixer._isActiveAction(this)}startAt(e){return this._startTime=e,this}setLoop(e,t){return this.loop=e,this.repetitions=t,this}setEffectiveWeight(e){return this.weight=e,this._effectiveWeight=this.enabled?e:0,this.stopFading()}getEffectiveWeight(){return this._effectiveWeight}fadeIn(e){return this._scheduleFading(e,0,1)}fadeOut(e){return this._scheduleFading(e,1,0)}crossFadeFrom(e,t,n=!1){if(e.fadeOut(t),this.fadeIn(t),n===!0){let i=this._clip.duration,r=e._clip.duration,a=r/i,o=i/r;e._restoreTimeScale=e.timeScale,this._restoreTimeScale=this.timeScale,e.warp(1,a,t),this.warp(o,1,t)}return this}crossFadeTo(e,t,n=!1){return e.crossFadeFrom(this,t,n)}stopFading(){let e=this._weightInterpolant;return e!==null&&(this._weightInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this}setEffectiveTimeScale(e){return this.timeScale=e,this._effectiveTimeScale=this.paused?0:e,this.stopWarping()}getEffectiveTimeScale(){return this._effectiveTimeScale}setDuration(e){return this.timeScale=this._clip.duration/e,this.stopWarping()}syncWith(e){return this.time=e.time,this.timeScale=e.timeScale,this.stopWarping()}halt(e){return this.warp(this._effectiveTimeScale,0,e)}warp(e,t,n){let i=this._mixer,r=i.time,a=this.timeScale,o=this._timeScaleInterpolant;o===null&&(o=i._lendControlInterpolant(),this._timeScaleInterpolant=o);let l=o.parameterPositions,c=o.sampleValues;return l[0]=r,l[1]=r+n,c[0]=e/a,c[1]=t/a,this}stopWarping(){let e=this._timeScaleInterpolant;return e!==null&&(this._timeScaleInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this._restoreTimeScale=null,this}getMixer(){return this._mixer}getClip(){return this._clip}getRoot(){return this._localRoot||this._mixer._root}_update(e,t,n,i){if(!this.enabled){this._updateWeight(e);return}let r=this._startTime;if(r!==null){let l=(e-r)*n;l<0||n===0?t=0:(this._startTime=null,t=n*l)}t*=this._updateTimeScale(e);let a=this._updateTime(t),o=this._updateWeight(e);if(o>0){let l=this._interpolants,c=this._propertyBindings;switch(this.blendMode){case tp:for(let h=0,d=l.length;h!==d;++h)l[h].evaluate(a),c[h].accumulateAdditive(o);break;case Nc:default:for(let h=0,d=l.length;h!==d;++h)l[h].evaluate(a),c[h].accumulate(i,o)}}}_updateWeight(e){let t=0;if(this.enabled){t=this.weight;let n=this._weightInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(this.stopFading(),i===0&&(this.enabled=!1))}}return this._effectiveWeight=t,t}_updateTimeScale(e){let t=0;if(!this.paused){t=this.timeScale;let n=this._timeScaleInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(t===0?this.paused=!0:(this._restoreTimeScale!==null&&(t=this._restoreTimeScale),this.timeScale=t),this.stopWarping())}}return this._effectiveTimeScale=t,t}_updateTime(e){let t=this._clip.duration,n=this.loop,i=this.time+e,r=this._loopCount,a=n===ep;if(e===0)return r===-1?i:a&&(r&1)===1?t-i:i;if(n===Fc){r===-1&&(this._loopCount=0,this._setEndings(!0,!0,!1));e:{if(i>=t)i=t;else if(i<0)i=0;else{this.time=i;break e}this.clampWhenFinished?this.paused=!0:this.enabled=!1,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e<0?-1:1})}}else{if(r===-1&&(e>=0?(r=0,this._setEndings(!0,this.repetitions===0,a)):this._setEndings(this.repetitions===0,!0,a)),i>=t||i<0){let o=Math.floor(i/t);i-=t*o,r+=Math.abs(o);let l=this.repetitions-r;if(l<=0)this.clampWhenFinished?this.paused=!0:this.enabled=!1,i=e>0?t:0,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e>0?1:-1});else{if(l===1){let c=e<0;this._setEndings(c,!c,a)}else this._setEndings(!1,!1,a);this._loopCount=r,this.time=i,this._mixer.dispatchEvent({type:"loop",action:this,loopDelta:o})}}else this._loopCount=r,this.time=i;if(a&&(r&1)===1)return t-i}return i}_setEndings(e,t,n){let i=this._interpolantSettings;n?(i.endingStart=Ws,i.endingEnd=Ws):(e?i.endingStart=this.zeroSlopeAtStart?Ws:Vs:i.endingStart=Da,t?i.endingEnd=this.zeroSlopeAtEnd?Ws:Vs:i.endingEnd=Da)}_scheduleFading(e,t,n){let i=this._mixer,r=i.time,a=this._weightInterpolant;a===null&&(a=i._lendControlInterpolant(),this._weightInterpolant=a);let o=a.parameterPositions,l=a.sampleValues;return o[0]=r,l[0]=t,o[1]=r+e,l[1]=n,this}},W0=new Float32Array(1),io=class extends ai{constructor(e){super(),this._root=e,this._initMemoryManager(),this._accuIndex=0,this.time=0,this.timeScale=1,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}_bindAction(e,t){let n=e._localRoot||this._root,i=e._clip.tracks,r=i.length,a=e._propertyBindings,o=e._interpolants,l=n.uuid,c=this._bindingsByRootAndName,h=c[l];h===void 0&&(h={},c[l]=h);for(let d=0;d!==r;++d){let u=i[d],p=u.name,x=h[p];if(x!==void 0)++x.referenceCount,a[d]=x;else{if(x=a[d],x!==void 0){x._cacheIndex===null&&(++x.referenceCount,this._addInactiveBinding(x,l,p));continue}let M=t&&t._propertyBindings[d].binding.parsedPath;x=new ql(tn.create(n,p,M),u.ValueTypeName,u.getValueSize()),++x.referenceCount,this._addInactiveBinding(x,l,p),a[d]=x}o[d].resultBuffer=x.buffer}}_activateAction(e){if(!this._isActiveAction(e)){if(e._cacheIndex===null){let n=(e._localRoot||this._root).uuid,i=e._clip.uuid,r=this._actionsByClip[i];this._bindAction(e,r&&r.knownActions[0]),this._addInactiveAction(e,i,n)}let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];r.useCount++===0&&(this._lendBinding(r),r.saveOriginalState())}this._lendAction(e)}}_deactivateAction(e){if(this._isActiveAction(e)){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.useCount===0&&(r.restoreOriginalState(),this._takeBackBinding(r))}this._takeBackAction(e)}}_initMemoryManager(){this._actions=[],this._nActiveActions=0,this._actionsByClip={},this._bindings=[],this._nActiveBindings=0,this._bindingsByRootAndName={},this._controlInterpolants=[],this._nActiveControlInterpolants=0;let e=this;this.stats={actions:{get total(){return e._actions.length},get inUse(){return e._nActiveActions}},bindings:{get total(){return e._bindings.length},get inUse(){return e._nActiveBindings}},controlInterpolants:{get total(){return e._controlInterpolants.length},get inUse(){return e._nActiveControlInterpolants}}}}_isActiveAction(e){let t=e._cacheIndex;return t!==null&&t<this._nActiveActions}_addInactiveAction(e,t,n){let i=this._actions,r=this._actionsByClip,a=r[t];if(a===void 0)a={knownActions:[e],actionByRoot:{}},e._byClipCacheIndex=0,r[t]=a;else{let o=a.knownActions;e._byClipCacheIndex=o.length,o.push(e)}e._cacheIndex=i.length,i.push(e),a.actionByRoot[n]=e}_removeInactiveAction(e){let t=this._actions,n=t[t.length-1],i=e._cacheIndex;n._cacheIndex=i,t[i]=n,t.pop(),e._cacheIndex=null;let r=e._clip.uuid,a=this._actionsByClip,o=a[r],l=o.knownActions,c=l[l.length-1],h=e._byClipCacheIndex;c._byClipCacheIndex=h,l[h]=c,l.pop(),e._byClipCacheIndex=null;let d=o.actionByRoot,u=(e._localRoot||this._root).uuid;delete d[u],l.length===0&&delete a[r],this._removeInactiveBindingsForAction(e)}_removeInactiveBindingsForAction(e){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.referenceCount===0&&this._removeInactiveBinding(r)}}_lendAction(e){let t=this._actions,n=e._cacheIndex,i=this._nActiveActions++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackAction(e){let t=this._actions,n=e._cacheIndex,i=--this._nActiveActions,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_addInactiveBinding(e,t,n){let i=this._bindingsByRootAndName,r=this._bindings,a=i[t];a===void 0&&(a={},i[t]=a),a[n]=e,e._cacheIndex=r.length,r.push(e)}_removeInactiveBinding(e){let t=this._bindings,n=e.binding,i=n.rootNode.uuid,r=n.path,a=this._bindingsByRootAndName,o=a[i],l=t[t.length-1],c=e._cacheIndex;l._cacheIndex=c,t[c]=l,t.pop(),delete o[r],Object.keys(o).length===0&&delete a[i]}_lendBinding(e){let t=this._bindings,n=e._cacheIndex,i=this._nActiveBindings++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackBinding(e){let t=this._bindings,n=e._cacheIndex,i=--this._nActiveBindings,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_lendControlInterpolant(){let e=this._controlInterpolants,t=this._nActiveControlInterpolants++,n=e[t];return n===void 0&&(n=new Ya(new Float32Array(2),new Float32Array(2),1,W0),n.__cacheIndex=t,e[t]=n),n}_takeBackControlInterpolant(e){let t=this._controlInterpolants,n=e.__cacheIndex,i=--this._nActiveControlInterpolants,r=t[i];e.__cacheIndex=i,t[i]=e,r.__cacheIndex=n,t[n]=r}clipAction(e,t,n){let i=t||this._root,r=i.uuid,a=typeof e=="string"?Qs.findByName(i,e):e,o=a!==null?a.uuid:e,l=this._actionsByClip[o],c=null;if(n===void 0&&(a!==null?n=a.blendMode:n=Nc),l!==void 0){let d=l.actionByRoot[r];if(d!==void 0&&d.blendMode===n)return d;c=l.knownActions[0],a===null&&(a=c._clip)}if(a===null)return null;let h=new Xl(this,a,t,n);return this._bindAction(h,c),this._addInactiveAction(h,o,r),h}existingAction(e,t){let n=t||this._root,i=n.uuid,r=typeof e=="string"?Qs.findByName(n,e):e,a=r?r.uuid:e,o=this._actionsByClip[a];return o!==void 0&&o.actionByRoot[i]||null}stopAllAction(){let e=this._actions,t=this._nActiveActions;for(let n=t-1;n>=0;--n)e[n].stop();return this}update(e){e*=this.timeScale;let t=this._actions,n=this._nActiveActions,i=this.time+=e,r=Math.sign(e),a=this._accuIndex^=1;for(let c=0;c!==n;++c)t[c]._update(i,e,r,a);let o=this._bindings,l=this._nActiveBindings;for(let c=0;c!==l;++c)o[c].apply(a);return this}setTime(e){this.time=0;for(let t=0;t<this._actions.length;t++)this._actions[t].time=0;return this.update(e)}getRoot(){return this._root}uncacheClip(e){let t=this._actions,n=e.uuid,i=this._actionsByClip,r=i[n];if(r!==void 0){let a=r.knownActions;for(let o=0,l=a.length;o!==l;++o){let c=a[o];this._deactivateAction(c);let h=c._cacheIndex,d=t[t.length-1];c._cacheIndex=null,c._byClipCacheIndex=null,d._cacheIndex=h,t[h]=d,t.pop(),this._removeInactiveBindingsForAction(c)}delete i[n]}}uncacheRoot(e){let t=e.uuid,n=this._actionsByClip;for(let a in n){let o=n[a].actionByRoot,l=o[t];l!==void 0&&(this._deactivateAction(l),this._removeInactiveAction(l))}let i=this._bindingsByRootAndName,r=i[t];if(r!==void 0)for(let a in r){let o=r[a];o.restoreOriginalState(),this._removeInactiveBinding(o)}}uncacheAction(e,t){let n=this.existingAction(e,t);n!==null&&(this._deactivateAction(n),this._removeInactiveAction(n))}};var Af=new yt,so=class{constructor(e,t,n=0,i=1/0){this.ray=new Fi(e,t),this.near=n,this.far=i,this.camera=null,this.layers=new Ur,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(e,t){this.ray.set(e,t)}setFromCamera(e,t){t.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(e.x,e.y,.5).unproject(t).sub(this.ray.origin).normalize(),this.camera=t):t.isOrthographicCamera?(this.ray.origin.set(e.x,e.y,t.projectionMatrix.elements[14]).unproject(t),this.ray.direction.set(0,0,-1).transformDirection(t.matrixWorld),this.camera=t):_t("Raycaster: Unsupported camera type: "+t.type)}setFromXRController(e){return Af.identity().extractRotation(e.matrixWorld),this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(Af),this}intersectObject(e,t=!0,n=[]){return Qh(e,this,n,t),n.sort(Rf),n}intersectObjects(e,t=!0,n=[]){for(let i=0,r=e.length;i<r;i++)Qh(e[i],this,n,t);return n.sort(Rf),n}};function Rf(s,e){return s.distance-e.distance}function Qh(s,e,t,n){let i=!0;if(s.layers.test(e.layers)&&s.raycast(e,t)===!1&&(i=!1),i===!0&&n===!0){let r=s.children;for(let a=0,o=r.length;a<o;a++)Qh(r[a],e,t,!0)}}var Jr=class{constructor(e=1,t=0,n=0){this.radius=e,this.phi=t,this.theta=n}set(e,t,n){return this.radius=e,this.phi=t,this.theta=n,this}copy(e){return this.radius=e.radius,this.phi=e.phi,this.theta=e.theta,this}makeSafe(){return this.phi=Lt(this.phi,1e-6,Math.PI-1e-6),this}setFromVector3(e){return this.setFromCartesianCoords(e.x,e.y,e.z)}setFromCartesianCoords(e,t,n){return this.radius=Math.sqrt(e*e+t*t+n*n),this.radius===0?(this.theta=0,this.phi=0):(this.theta=Math.atan2(e,n),this.phi=Math.acos(Lt(t/this.radius,-1,1))),this}clone(){return new this.constructor().copy(this)}};var eu=class s{static{s.prototype.isMatrix2=!0}constructor(e,t,n,i){this.elements=[1,0,0,1],e!==void 0&&this.set(e,t,n,i)}identity(){return this.set(1,0,0,1),this}fromArray(e,t=0){for(let n=0;n<4;n++)this.elements[n]=e[n+t];return this}set(e,t,n,i){let r=this.elements;return r[0]=e,r[2]=t,r[1]=n,r[3]=i,this}};var ro=class extends ai{constructor(e,t=null){super(),this.object=e,this.domElement=t,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(e){this.domElement!==null&&this.disconnect(),this.domElement=e}disconnect(){}dispose(){}update(){}};function _u(s,e,t,n){let i=q0(n);switch(t){case uu:return s*e;case ec:return s*e/i.components*i.byteLength;case tc:return s*e/i.components*i.byteLength;case Ps:return s*e*2/i.components*i.byteLength;case nc:return s*e*2/i.components*i.byteLength;case du:return s*e*3/i.components*i.byteLength;case ti:return s*e*4/i.components*i.byteLength;case ic:return s*e*4/i.components*i.byteLength;case po:case mo:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case go:case xo:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case rc:case oc:return Math.max(s,16)*Math.max(e,8)/4;case sc:case ac:return Math.max(s,8)*Math.max(e,8)/2;case lc:case cc:case uc:case dc:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case hc:case bo:case fc:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case pc:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case mc:return Math.floor((s+4)/5)*Math.floor((e+3)/4)*16;case gc:return Math.floor((s+4)/5)*Math.floor((e+4)/5)*16;case xc:return Math.floor((s+5)/6)*Math.floor((e+4)/5)*16;case bc:return Math.floor((s+5)/6)*Math.floor((e+5)/6)*16;case vc:return Math.floor((s+7)/8)*Math.floor((e+4)/5)*16;case yc:return Math.floor((s+7)/8)*Math.floor((e+5)/6)*16;case _c:return Math.floor((s+7)/8)*Math.floor((e+7)/8)*16;case Mc:return Math.floor((s+9)/10)*Math.floor((e+4)/5)*16;case wc:return Math.floor((s+9)/10)*Math.floor((e+5)/6)*16;case Sc:return Math.floor((s+9)/10)*Math.floor((e+7)/8)*16;case Ec:return Math.floor((s+9)/10)*Math.floor((e+9)/10)*16;case Tc:return Math.floor((s+11)/12)*Math.floor((e+9)/10)*16;case Ac:return Math.floor((s+11)/12)*Math.floor((e+11)/12)*16;case Rc:case Cc:case Pc:return Math.ceil(s/4)*Math.ceil(e/4)*16;case Ic:case Dc:return Math.ceil(s/4)*Math.ceil(e/4)*8;case vo:case Lc:return Math.ceil(s/4)*Math.ceil(e/4)*16}throw new Error(`Unable to determine texture byte length for ${t} format.`)}function q0(s){switch(s){case Xn:case ou:return{byteLength:1,components:1};case ea:case lu:case xn:return{byteLength:2,components:1};case $l:case Ql:return{byteLength:2,components:4};case Ti:case Zl:case ei:return{byteLength:4,components:1};case cu:case hu:return{byteLength:4,components:3}}throw new Error(`THREE.TextureUtils: Unknown texture type ${s}.`)}typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"186"}}));typeof window<"u"&&(window.__THREE__?dt("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="186");function kp(){let s=null,e=!1,t=null,n=null;function i(r,a){n=s.requestAnimationFrame(i),t(r,a)}return{start:function(){e!==!0&&t!==null&&s!==null&&(n=s.requestAnimationFrame(i),e=!0)},stop:function(){s!==null&&s.cancelAnimationFrame(n),e=!1},setAnimationLoop:function(r){t=r},setContext:function(r){s=r}}}function j0(s){let e=new WeakMap;function t(o,l){let c=o.array,h=o.usage,d=c.byteLength,u=s.createBuffer();s.bindBuffer(l,u),s.bufferData(l,c,h),o.onUploadCallback();let p;if(c instanceof Float32Array)p=s.FLOAT;else if(typeof Float16Array<"u"&&c instanceof Float16Array)p=s.HALF_FLOAT;else if(c instanceof Uint16Array)o.isFloat16BufferAttribute?p=s.HALF_FLOAT:p=s.UNSIGNED_SHORT;else if(c instanceof Int16Array)p=s.SHORT;else if(c instanceof Uint32Array)p=s.UNSIGNED_INT;else if(c instanceof Int32Array)p=s.INT;else if(c instanceof Int8Array)p=s.BYTE;else if(c instanceof Uint8Array)p=s.UNSIGNED_BYTE;else if(c instanceof Uint8ClampedArray)p=s.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+c);return{buffer:u,type:p,bytesPerElement:c.BYTES_PER_ELEMENT,version:o.version,size:d}}function n(o,l,c){let h=l.array,d=l.updateRanges;if(s.bindBuffer(c,o),d.length===0)s.bufferSubData(c,0,h);else{d.sort((p,x)=>p.start-x.start);let u=0;for(let p=1;p<d.length;p++){let x=d[u],M=d[p];M.start<=x.start+x.count+1?x.count=Math.max(x.count,M.start+M.count-x.start):(++u,d[u]=M)}d.length=u+1;for(let p=0,x=d.length;p<x;p++){let M=d[p];s.bufferSubData(c,M.start*h.BYTES_PER_ELEMENT,h,M.start,M.count)}l.clearUpdateRanges()}l.onUploadCallback()}function i(o){return o.isInterleavedBufferAttribute&&(o=o.data),e.get(o)}function r(o){o.isInterleavedBufferAttribute&&(o=o.data);let l=e.get(o);l&&(s.deleteBuffer(l.buffer),e.delete(o))}function a(o,l){if(o.isInterleavedBufferAttribute&&(o=o.data),o.isGLBufferAttribute){let h=e.get(o);(!h||h.version<o.version)&&e.set(o,{buffer:o.buffer,type:o.type,bytesPerElement:o.elementSize,version:o.version});return}let c=e.get(o);if(c===void 0)e.set(o,t(o,l));else if(c.version<o.version){if(c.size!==o.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");n(c.buffer,o,l),c.version=o.version}}return{get:i,remove:r,update:a}}var K0=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,Y0=`#ifdef USE_ALPHAHASH
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
#endif`,J0=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,Z0=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,$0=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,Q0=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,ex=`#ifdef USE_AOMAP
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
#endif`,tx=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,nx=`#ifdef USE_BATCHING
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
#endif`,ix=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,sx=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,rx=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,ax=`float G_BlinnPhong_Implicit( ) {
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
} // validated`,ox=`#ifdef USE_IRIDESCENCE
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
#endif`,lx=`#ifdef USE_BUMPMAP
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
#endif`,cx=`#if NUM_CLIPPING_PLANES > 0
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
#endif`,hx=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,ux=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,dx=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,fx=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#endif`,px=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#endif`,mx=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec4 vColor;
#endif`,gx=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
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
#endif`,xx=`#define PI 3.141592653589793
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
} // validated`,bx=`#ifdef ENVMAP_TYPE_CUBE_UV
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
#endif`,vx=`vec3 transformedNormal = objectNormal;
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
#endif`,yx=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,_x=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,Mx=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	#ifdef DECODE_VIDEO_TEXTURE_EMISSIVE
		emissiveColor = sRGBTransferEOTF( emissiveColor );
	#endif
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,wx=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,Sx="gl_FragColor = linearToOutputTexel( gl_FragColor );",Ex=`vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferEOTF( in vec4 value ) {
	return vec4( mix( pow( value.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), value.rgb * 0.0773993808, vec3( lessThanEqual( value.rgb, vec3( 0.04045 ) ) ) ), value.a );
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,Tx=`#ifdef USE_ENVMAP
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
#endif`,Ax=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
#endif`,Rx=`#ifdef USE_ENVMAP
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
#endif`,Cx=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,Px=`#ifdef USE_ENVMAP
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
#endif`,Ix=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,Dx=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,Lx=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,Fx=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,Nx=`#ifdef USE_GRADIENTMAP
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
}`,Ux=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,zx=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,Ox=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,kx=`uniform bool receiveShadow;
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
#include <lightprobes_pars_fragment>`,Bx=`#ifdef USE_ENVMAP
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
#endif`,Hx=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,Gx=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,Vx=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,Wx=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,qx=`PhysicalMaterial material;
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
#endif`,Xx=`uniform sampler2D dfgLUT;
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
}`,jx=`
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
#endif`,Kx=`#if defined( RE_IndirectDiffuse )
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
#endif`,Yx=`#if defined( RE_IndirectDiffuse )
	#if defined( LAMBERT ) || defined( PHONG )
		irradiance += iblIrradiance;
	#endif
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,Jx=`#ifdef USE_LIGHT_PROBES_GRID
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
#endif`,Zx=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,$x=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,Qx=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,eb=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,tb=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = sRGBTransferEOTF( sampledDiffuseColor );
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,nb=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,ib=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
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
#endif`,sb=`#if defined( USE_POINTS_UV )
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
#endif`,rb=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,ab=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,ob=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,lb=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,cb=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,hb=`#ifdef USE_MORPHTARGETS
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
#endif`,ub=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,db=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
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
vec3 nonPerturbedNormal = normal;`,fb=`#ifdef USE_NORMALMAP_OBJECTSPACE
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
#endif`,pb=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,mb=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,gb=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
		#ifdef FLIP_SIDED
			vBitangent = - vBitangent;
		#endif
	#endif
#endif`,xb=`#ifdef USE_NORMALMAP
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
#endif`,bb=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,vb=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,yb=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,_b=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,Mb=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,wb=`vec3 packNormalToRGB( const in vec3 normal ) {
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
}`,Sb=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,Eb=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,Tb=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,Ab=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,Rb=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,Cb=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,Pb=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,Ib=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,Db=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_SUN_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
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
#endif`,Lb=`float getShadowMask() {
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
}`,Fb=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,Nb=`#ifdef USE_SKINNING
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
#endif`,Ub=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,zb=`#ifdef USE_SKINNING
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
#endif`,Ob=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,kb=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,Bb=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,Hb=`#ifndef saturate
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
vec3 CustomToneMapping( vec3 color ) { return color; }`,Gb=`#ifdef USE_TRANSMISSION
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
#endif`,Vb=`#ifdef USE_TRANSMISSION
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
#endif`,Wb=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,qb=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,Xb=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,jb=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,Kb=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,Yb=`uniform sampler2D t2D;
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
}`,Jb=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,Zb=`#ifdef ENVMAP_TYPE_CUBE
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
}`,$b=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,Qb=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,ev=`#include <common>
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
}`,tv=`#if DEPTH_PACKING == 3200
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
}`,nv=`#define DISTANCE
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
}`,iv=`#define DISTANCE
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
}`,sv=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,rv=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,av=`uniform float scale;
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
}`,ov=`uniform vec3 diffuse;
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
}`,lv=`#include <common>
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
}`,cv=`uniform vec3 diffuse;
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
}`,hv=`#define LAMBERT
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
}`,uv=`#define LAMBERT
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
}`,dv=`#define MATCAP
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
}`,fv=`#define MATCAP
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
}`,pv=`#define NORMAL
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
}`,mv=`#define NORMAL
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
}`,gv=`#define PHONG
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
}`,xv=`#define PHONG
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
}`,bv=`#define STANDARD
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
}`,vv=`#define STANDARD
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
}`,yv=`#define TOON
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
}`,_v=`#define TOON
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
}`,Mv=`uniform float size;
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
}`,wv=`uniform vec3 diffuse;
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
}`,Sv=`#include <common>
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
}`,Ev=`uniform vec3 color;
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
}`,Tv=`uniform float rotation;
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
}`,Av=`uniform vec3 diffuse;
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
}`,Ft={alphahash_fragment:K0,alphahash_pars_fragment:Y0,alphamap_fragment:J0,alphamap_pars_fragment:Z0,alphatest_fragment:$0,alphatest_pars_fragment:Q0,aomap_fragment:ex,aomap_pars_fragment:tx,batching_pars_vertex:nx,batching_vertex:ix,begin_vertex:sx,beginnormal_vertex:rx,bsdfs:ax,iridescence_fragment:ox,bumpmap_pars_fragment:lx,clipping_planes_fragment:cx,clipping_planes_pars_fragment:hx,clipping_planes_pars_vertex:ux,clipping_planes_vertex:dx,color_fragment:fx,color_pars_fragment:px,color_pars_vertex:mx,color_vertex:gx,common:xx,cube_uv_reflection_fragment:bx,defaultnormal_vertex:vx,displacementmap_pars_vertex:yx,displacementmap_vertex:_x,emissivemap_fragment:Mx,emissivemap_pars_fragment:wx,colorspace_fragment:Sx,colorspace_pars_fragment:Ex,envmap_fragment:Tx,envmap_common_pars_fragment:Ax,envmap_pars_fragment:Rx,envmap_pars_vertex:Cx,envmap_physical_pars_fragment:Bx,envmap_vertex:Px,fog_vertex:Ix,fog_pars_vertex:Dx,fog_fragment:Lx,fog_pars_fragment:Fx,gradientmap_pars_fragment:Nx,lightmap_pars_fragment:Ux,lights_lambert_fragment:zx,lights_lambert_pars_fragment:Ox,lights_pars_begin:kx,lights_toon_fragment:Hx,lights_toon_pars_fragment:Gx,lights_phong_fragment:Vx,lights_phong_pars_fragment:Wx,lights_physical_fragment:qx,lights_physical_pars_fragment:Xx,lights_fragment_begin:jx,lights_fragment_maps:Kx,lights_fragment_end:Yx,lightprobes_pars_fragment:Jx,logdepthbuf_fragment:Zx,logdepthbuf_pars_fragment:$x,logdepthbuf_pars_vertex:Qx,logdepthbuf_vertex:eb,map_fragment:tb,map_pars_fragment:nb,map_particle_fragment:ib,map_particle_pars_fragment:sb,metalnessmap_fragment:rb,metalnessmap_pars_fragment:ab,morphinstance_vertex:ob,morphcolor_vertex:lb,morphnormal_vertex:cb,morphtarget_pars_vertex:hb,morphtarget_vertex:ub,normal_fragment_begin:db,normal_fragment_maps:fb,normal_pars_fragment:pb,normal_pars_vertex:mb,normal_vertex:gb,normalmap_pars_fragment:xb,clearcoat_normal_fragment_begin:bb,clearcoat_normal_fragment_maps:vb,clearcoat_pars_fragment:yb,iridescence_pars_fragment:_b,opaque_fragment:Mb,packing:wb,premultiplied_alpha_fragment:Sb,project_vertex:Eb,dithering_fragment:Tb,dithering_pars_fragment:Ab,roughnessmap_fragment:Rb,roughnessmap_pars_fragment:Cb,shadowmap_pars_fragment:Pb,shadowmap_pars_vertex:Ib,shadowmap_vertex:Db,shadowmask_pars_fragment:Lb,skinbase_vertex:Fb,skinning_pars_vertex:Nb,skinning_vertex:Ub,skinnormal_vertex:zb,specularmap_fragment:Ob,specularmap_pars_fragment:kb,tonemapping_fragment:Bb,tonemapping_pars_fragment:Hb,transmission_fragment:Gb,transmission_pars_fragment:Vb,uv_pars_fragment:Wb,uv_pars_vertex:qb,uv_vertex:Xb,worldpos_vertex:jb,background_vert:Kb,background_frag:Yb,backgroundCube_vert:Jb,backgroundCube_frag:Zb,cube_vert:$b,cube_frag:Qb,depth_vert:ev,depth_frag:tv,distance_vert:nv,distance_frag:iv,equirect_vert:sv,equirect_frag:rv,linedashed_vert:av,linedashed_frag:ov,meshbasic_vert:lv,meshbasic_frag:cv,meshlambert_vert:hv,meshlambert_frag:uv,meshmatcap_vert:dv,meshmatcap_frag:fv,meshnormal_vert:pv,meshnormal_frag:mv,meshphong_vert:gv,meshphong_frag:xv,meshphysical_vert:bv,meshphysical_frag:vv,meshtoon_vert:yv,meshtoon_frag:_v,points_vert:Mv,points_frag:wv,shadow_vert:Sv,shadow_frag:Ev,sprite_vert:Tv,sprite_frag:Av},Qe={common:{diffuse:{value:new Le(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new Tt},alphaMap:{value:null},alphaMapTransform:{value:new Tt},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new Tt}},envmap:{envMap:{value:null},envMapRotation:{value:new Tt},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98},dfgLUT:{value:null}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new Tt}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new Tt}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new Tt},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new Tt},normalScale:{value:new Ge(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new Tt},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new Tt}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new Tt}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new Tt}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new Le(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},sunLights:{value:[],properties:{direction:{},color:{}}},sunLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},sunShadowMatrix:{value:[]},sunShadowCascade:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null},probesSH:{value:null},probesMin:{value:new U},probesMax:{value:new U},probesResolution:{value:new U}},points:{diffuse:{value:new Le(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new Tt},alphaTest:{value:0},uvTransform:{value:new Tt}},sprite:{diffuse:{value:new Le(16777215)},opacity:{value:1},center:{value:new Ge(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new Tt},alphaMap:{value:null},alphaMapTransform:{value:new Tt},alphaTest:{value:0}}},Hi={basic:{uniforms:zn([Qe.common,Qe.specularmap,Qe.envmap,Qe.aomap,Qe.lightmap,Qe.fog]),vertexShader:Ft.meshbasic_vert,fragmentShader:Ft.meshbasic_frag},lambert:{uniforms:zn([Qe.common,Qe.specularmap,Qe.envmap,Qe.aomap,Qe.lightmap,Qe.emissivemap,Qe.bumpmap,Qe.normalmap,Qe.displacementmap,Qe.fog,Qe.lights,{emissive:{value:new Le(0)},envMapIntensity:{value:1}}]),vertexShader:Ft.meshlambert_vert,fragmentShader:Ft.meshlambert_frag},phong:{uniforms:zn([Qe.common,Qe.specularmap,Qe.envmap,Qe.aomap,Qe.lightmap,Qe.emissivemap,Qe.bumpmap,Qe.normalmap,Qe.displacementmap,Qe.fog,Qe.lights,{emissive:{value:new Le(0)},specular:{value:new Le(1118481)},shininess:{value:30},envMapIntensity:{value:1}}]),vertexShader:Ft.meshphong_vert,fragmentShader:Ft.meshphong_frag},standard:{uniforms:zn([Qe.common,Qe.envmap,Qe.aomap,Qe.lightmap,Qe.emissivemap,Qe.bumpmap,Qe.normalmap,Qe.displacementmap,Qe.roughnessmap,Qe.metalnessmap,Qe.fog,Qe.lights,{emissive:{value:new Le(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:Ft.meshphysical_vert,fragmentShader:Ft.meshphysical_frag},toon:{uniforms:zn([Qe.common,Qe.aomap,Qe.lightmap,Qe.emissivemap,Qe.bumpmap,Qe.normalmap,Qe.displacementmap,Qe.gradientmap,Qe.fog,Qe.lights,{emissive:{value:new Le(0)}}]),vertexShader:Ft.meshtoon_vert,fragmentShader:Ft.meshtoon_frag},matcap:{uniforms:zn([Qe.common,Qe.bumpmap,Qe.normalmap,Qe.displacementmap,Qe.fog,{matcap:{value:null}}]),vertexShader:Ft.meshmatcap_vert,fragmentShader:Ft.meshmatcap_frag},points:{uniforms:zn([Qe.points,Qe.fog]),vertexShader:Ft.points_vert,fragmentShader:Ft.points_frag},dashed:{uniforms:zn([Qe.common,Qe.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:Ft.linedashed_vert,fragmentShader:Ft.linedashed_frag},depth:{uniforms:zn([Qe.common,Qe.displacementmap]),vertexShader:Ft.depth_vert,fragmentShader:Ft.depth_frag},normal:{uniforms:zn([Qe.common,Qe.bumpmap,Qe.normalmap,Qe.displacementmap,{opacity:{value:1}}]),vertexShader:Ft.meshnormal_vert,fragmentShader:Ft.meshnormal_frag},sprite:{uniforms:zn([Qe.sprite,Qe.fog]),vertexShader:Ft.sprite_vert,fragmentShader:Ft.sprite_frag},background:{uniforms:{uvTransform:{value:new Tt},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:Ft.background_vert,fragmentShader:Ft.background_frag},backgroundCube:{uniforms:{envMap:{value:null},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new Tt}},vertexShader:Ft.backgroundCube_vert,fragmentShader:Ft.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:Ft.cube_vert,fragmentShader:Ft.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:Ft.equirect_vert,fragmentShader:Ft.equirect_frag},distance:{uniforms:zn([Qe.common,Qe.displacementmap,{referencePosition:{value:new U},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:Ft.distance_vert,fragmentShader:Ft.distance_frag},shadow:{uniforms:zn([Qe.lights,Qe.fog,{color:{value:new Le(0)},opacity:{value:1}}]),vertexShader:Ft.shadow_vert,fragmentShader:Ft.shadow_frag}};Hi.physical={uniforms:zn([Hi.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new Tt},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new Tt},clearcoatNormalScale:{value:new Ge(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new Tt},dispersion:{value:0},retroreflectivity:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new Tt},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new Tt},sheen:{value:0},sheenColor:{value:new Le(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new Tt},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new Tt},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new Tt},transmissionSamplerSize:{value:new Ge},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new Tt},attenuationDistance:{value:0},attenuationColor:{value:new Le(0)},specularColor:{value:new Le(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new Tt},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new Tt},anisotropyVector:{value:new Ge},anisotropyMap:{value:null},anisotropyMapTransform:{value:new Tt}}]),vertexShader:Ft.meshphysical_vert,fragmentShader:Ft.meshphysical_frag};var Oc={r:0,b:0,g:0},Rv=new yt,Bp=new Tt;Bp.set(-1,0,0,0,1,0,0,0,1);function Cv(s,e,t,n,i,r){let a=new Le(0),o=i===!0?0:1,l,c,h=null,d=0,u=null;function p(b){let E=b.isScene===!0?b.background:null;if(E&&E.isTexture){let g=b.backgroundBlurriness>0;E=e.get(E,g)}return E}function x(b){let E=!1,g=p(b);g===null?m(a,o):g&&g.isColor&&(m(g,1),E=!0);let _=s.xr.getEnvironmentBlendMode();_==="additive"?t.buffers.color.setClear(0,0,0,1,r):_==="alpha-blend"&&t.buffers.color.setClear(0,0,0,0,r),(s.autoClear||E)&&(t.buffers.depth.setTest(!0),t.buffers.depth.setMask(!0),t.buffers.color.setMask(!0),s.clear(s.autoClearColor,s.autoClearDepth,s.autoClearStencil))}function M(b,E){let g=p(E);g&&(g.isCubeTexture||g.mapping===fo)?(c===void 0&&(c=new ht(new _i(1,1,1),new Rt({name:"BackgroundCubeMaterial",uniforms:ar(Hi.backgroundCube.uniforms),vertexShader:Hi.backgroundCube.vertexShader,fragmentShader:Hi.backgroundCube.fragmentShader,side:yn,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),c.geometry.deleteAttribute("normal"),c.geometry.deleteAttribute("uv"),c.onBeforeRender=function(_,w,T){this.matrixWorld.copyPosition(T.matrixWorld)},Object.defineProperty(c.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),n.update(c)),c.material.uniforms.envMap.value=g,c.material.uniforms.backgroundBlurriness.value=E.backgroundBlurriness,c.material.uniforms.backgroundIntensity.value=E.backgroundIntensity,c.material.uniforms.backgroundRotation.value.setFromMatrix4(Rv.makeRotationFromEuler(E.backgroundRotation)).transpose(),g.isCubeTexture&&g.isRenderTargetTexture===!1&&c.material.uniforms.backgroundRotation.value.premultiply(Bp),c.material.toneMapped=Dt.getTransfer(g.colorSpace)!==Xt,(h!==g||d!==g.version||u!==s.toneMapping)&&(c.material.needsUpdate=!0,h=g,d=g.version,u=s.toneMapping),c.layers.enableAll(),b.unshift(c,c.geometry,c.material,0,0,null)):g&&g.isTexture&&(l===void 0&&(l=new ht(new dn(2,2),new Rt({name:"BackgroundMaterial",uniforms:ar(Hi.background.uniforms),vertexShader:Hi.background.vertexShader,fragmentShader:Hi.background.fragmentShader,side:oi,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),l.geometry.deleteAttribute("normal"),Object.defineProperty(l.material,"map",{get:function(){return this.uniforms.t2D.value}}),n.update(l)),l.material.uniforms.t2D.value=g,l.material.uniforms.backgroundIntensity.value=E.backgroundIntensity,l.material.toneMapped=Dt.getTransfer(g.colorSpace)!==Xt,g.matrixAutoUpdate===!0&&g.updateMatrix(),l.material.uniforms.uvTransform.value.copy(g.matrix),(h!==g||d!==g.version||u!==s.toneMapping)&&(l.material.needsUpdate=!0,h=g,d=g.version,u=s.toneMapping),l.layers.enableAll(),b.unshift(l,l.geometry,l.material,0,0,null))}function m(b,E){b.getRGB(Oc,bu(s)),t.buffers.color.setClear(Oc.r,Oc.g,Oc.b,E,r)}function f(){c!==void 0&&(c.geometry.dispose(),c.material.dispose(),c=void 0),l!==void 0&&(l.geometry.dispose(),l.material.dispose(),l=void 0)}return{getClearColor:function(){return a},setClearColor:function(b,E=1){a.set(b),o=E,m(a,o)},getClearAlpha:function(){return o},setClearAlpha:function(b){o=b,m(a,o)},render:x,addToRenderList:M,dispose:f}}function Pv(s,e){let t=s.getParameter(s.MAX_VERTEX_ATTRIBS),n={},i=u(null),r=i,a=!1;function o(D,v,I,P,N){let z=!1,X=d(D,P,I,v);r!==X&&(r=X,c(r.object)),z=p(D,P,I,N),z&&x(D,P,I,N),N!==null&&e.update(N,s.ELEMENT_ARRAY_BUFFER),(z||a)&&(a=!1,g(D,v,I,P),N!==null&&s.bindBuffer(s.ELEMENT_ARRAY_BUFFER,e.get(N).buffer))}function l(){return s.createVertexArray()}function c(D){return s.bindVertexArray(D)}function h(D){return s.deleteVertexArray(D)}function d(D,v,I,P){let N=P.wireframe===!0,z=n[v.id];z===void 0&&(z={},n[v.id]=z);let X=D.isInstancedMesh===!0?D.id:0,j=z[X];j===void 0&&(j={},z[X]=j);let V=j[I.id];V===void 0&&(V={},j[I.id]=V);let W=V[N];return W===void 0&&(W=u(l()),V[N]=W),W}function u(D){let v=[],I=[],P=[];for(let N=0;N<t;N++)v[N]=0,I[N]=0,P[N]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:v,enabledAttributes:I,attributeDivisors:P,object:D,attributes:{},index:null}}function p(D,v,I,P){let N=r.attributes,z=v.attributes,X=0,j=I.getAttributes();for(let V in j)if(j[V].location>=0){let ie=N[V],H=z[V];if(H===void 0&&(V==="instanceMatrix"&&D.instanceMatrix&&(H=D.instanceMatrix),V==="instanceColor"&&D.instanceColor&&(H=D.instanceColor)),ie===void 0||ie.attribute!==H||H&&ie.data!==H.data)return!0;X++}return r.attributesNum!==X||r.index!==P}function x(D,v,I,P){let N={},z=v.attributes,X=0,j=I.getAttributes();for(let V in j)if(j[V].location>=0){let ie=z[V];ie===void 0&&(V==="instanceMatrix"&&D.instanceMatrix&&(ie=D.instanceMatrix),V==="instanceColor"&&D.instanceColor&&(ie=D.instanceColor));let H={};H.attribute=ie,ie&&ie.data&&(H.data=ie.data),N[V]=H,X++}r.attributes=N,r.attributesNum=X,r.index=P}function M(){let D=r.newAttributes;for(let v=0,I=D.length;v<I;v++)D[v]=0}function m(D){f(D,0)}function f(D,v){let I=r.newAttributes,P=r.enabledAttributes,N=r.attributeDivisors;I[D]=1,P[D]===0&&(s.enableVertexAttribArray(D),P[D]=1),N[D]!==v&&(s.vertexAttribDivisor(D,v),N[D]=v)}function b(){let D=r.newAttributes,v=r.enabledAttributes;for(let I=0,P=v.length;I<P;I++)v[I]!==D[I]&&(s.disableVertexAttribArray(I),v[I]=0)}function E(D,v,I,P,N,z,X){X===!0?s.vertexAttribIPointer(D,v,I,N,z):s.vertexAttribPointer(D,v,I,P,N,z)}function g(D,v,I,P){M();let N=P.attributes,z=I.getAttributes(),X=v.defaultAttributeValues;for(let j in z){let V=z[j];if(V.location>=0){let W=N[j];if(W===void 0&&(j==="instanceMatrix"&&D.instanceMatrix&&(W=D.instanceMatrix),j==="instanceColor"&&D.instanceColor&&(W=D.instanceColor)),W!==void 0){let ie=W.normalized,H=W.itemSize,ge=e.get(W);if(ge===void 0)continue;let Ze=ge.buffer,G=ge.type,ye=ge.bytesPerElement,J=G===s.INT||G===s.UNSIGNED_INT||W.gpuType===Zl;if(W.isInterleavedBufferAttribute){let A=W.data,me=A.stride,ce=W.offset;if(A.isInstancedInterleavedBuffer){for(let ne=0;ne<V.locationSize;ne++)f(V.location+ne,A.meshPerAttribute);D.isInstancedMesh!==!0&&P._maxInstanceCount===void 0&&(P._maxInstanceCount=A.meshPerAttribute*A.count)}else for(let ne=0;ne<V.locationSize;ne++)m(V.location+ne);s.bindBuffer(s.ARRAY_BUFFER,Ze);for(let ne=0;ne<V.locationSize;ne++)E(V.location+ne,H/V.locationSize,G,ie,me*ye,(ce+H/V.locationSize*ne)*ye,J)}else{if(W.isInstancedBufferAttribute){for(let A=0;A<V.locationSize;A++)f(V.location+A,W.meshPerAttribute);D.isInstancedMesh!==!0&&P._maxInstanceCount===void 0&&(P._maxInstanceCount=W.meshPerAttribute*W.count)}else for(let A=0;A<V.locationSize;A++)m(V.location+A);s.bindBuffer(s.ARRAY_BUFFER,Ze);for(let A=0;A<V.locationSize;A++)E(V.location+A,H/V.locationSize,G,ie,H*ye,H/V.locationSize*A*ye,J)}}else if(X!==void 0){let ie=X[j];if(ie!==void 0)switch(ie.length){case 2:s.vertexAttrib2fv(V.location,ie);break;case 3:s.vertexAttrib3fv(V.location,ie);break;case 4:s.vertexAttrib4fv(V.location,ie);break;default:s.vertexAttrib1fv(V.location,ie)}}}}b()}function _(){C();for(let D in n){let v=n[D];for(let I in v){let P=v[I];for(let N in P){let z=P[N];for(let X in z)h(z[X].object),delete z[X];delete P[N]}}delete n[D]}}function w(D){if(n[D.id]===void 0)return;let v=n[D.id];for(let I in v){let P=v[I];for(let N in P){let z=P[N];for(let X in z)h(z[X].object),delete z[X];delete P[N]}}delete n[D.id]}function T(D){for(let v in n){let I=n[v];for(let P in I){let N=I[P];if(N[D.id]===void 0)continue;let z=N[D.id];for(let X in z)h(z[X].object),delete z[X];delete N[D.id]}}}function y(D){for(let v in n){let I=n[v],P=D.isInstancedMesh===!0?D.id:0,N=I[P];if(N!==void 0){for(let z in N){let X=N[z];for(let j in X)h(X[j].object),delete X[j];delete N[z]}delete I[P],Object.keys(I).length===0&&delete n[v]}}}function C(){S(),a=!0,r!==i&&(r=i,c(r.object))}function S(){i.geometry=null,i.program=null,i.wireframe=!1}return{setup:o,reset:C,resetDefaultState:S,dispose:_,releaseStatesOfGeometry:w,releaseStatesOfObject:y,releaseStatesOfProgram:T,initAttributes:M,enableAttribute:m,disableUnusedAttributes:b}}function Iv(s,e,t){let n;function i(l){n=l}function r(l,c){s.drawArrays(n,l,c),t.update(c,n,1)}function a(l,c,h){h!==0&&(s.drawArraysInstanced(n,l,c,h),t.update(c,n,h))}function o(l,c,h){if(h===0)return;e.get("WEBGL_multi_draw").multiDrawArraysWEBGL(n,l,0,c,0,h);let u=0;for(let p=0;p<h;p++)u+=c[p];t.update(u,n,1)}this.setMode=i,this.render=r,this.renderInstances=a,this.renderMultiDraw=o}function Dv(s,e,t,n){let i;function r(){if(i!==void 0)return i;if(e.has("EXT_texture_filter_anisotropic")===!0){let T=e.get("EXT_texture_filter_anisotropic");i=s.getParameter(T.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else i=0;return i}function a(T){return!(T!==ti&&n.convert(T)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_FORMAT))}function o(T){let y=T===xn&&(e.has("EXT_color_buffer_half_float")||e.has("EXT_color_buffer_float"));return!(T!==Xn&&T!==ei&&!y&&n.convert(T)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_TYPE))}function l(T){if(T==="highp"){if(s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.HIGH_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.HIGH_FLOAT).precision>0)return"highp";T="mediump"}return T==="mediump"&&s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.MEDIUM_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let c=t.precision!==void 0?t.precision:"highp",h=l(c);h!==c&&(dt("WebGLRenderer:",c,"not supported, using",h,"instead."),c=h);let d=t.logarithmicDepthBuffer===!0,u=t.reversedDepthBuffer===!0&&e.has("EXT_clip_control");t.reversedDepthBuffer===!0&&u===!1&&dt("WebGLRenderer: Unable to use reversed depth buffer due to missing EXT_clip_control extension. Fallback to default depth buffer.");let p=s.getParameter(s.MAX_TEXTURE_IMAGE_UNITS),x=s.getParameter(s.MAX_VERTEX_TEXTURE_IMAGE_UNITS),M=s.getParameter(s.MAX_TEXTURE_SIZE),m=s.getParameter(s.MAX_CUBE_MAP_TEXTURE_SIZE),f=s.getParameter(s.MAX_VERTEX_ATTRIBS),b=s.getParameter(s.MAX_VERTEX_UNIFORM_VECTORS),E=s.getParameter(s.MAX_VARYING_VECTORS),g=s.getParameter(s.MAX_FRAGMENT_UNIFORM_VECTORS),_=s.getParameter(s.MAX_SAMPLES),w=s.getParameter(s.SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:l,textureFormatReadable:a,textureTypeReadable:o,precision:c,logarithmicDepthBuffer:d,reversedDepthBuffer:u,maxTextures:p,maxVertexTextures:x,maxTextureSize:M,maxCubemapSize:m,maxAttributes:f,maxVertexUniforms:b,maxVaryings:E,maxFragmentUniforms:g,maxSamples:_,samples:w}}function Lv(s){let e=this,t=null,n=0,i=!1,r=!1,a=new Zn,o=new Tt,l={value:null,needsUpdate:!1};this.uniform=l,this.numPlanes=0,this.numIntersection=0,this.init=function(d,u){let p=d.length!==0||u||n!==0||i;return i=u,n=d.length,p},this.beginShadows=function(){r=!0,h(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(d,u){t=h(d,u,0)},this.setState=function(d,u,p){let x=d.clippingPlanes,M=d.clipIntersection,m=d.clipShadows,f=s.get(d);if(!i||x===null||x.length===0||r&&!m)r?h(null):c();else{let b=r?0:n,E=b*4,g=f.clippingState||null;l.value=g,g=h(x,u,E,p);for(let _=0;_!==E;++_)g[_]=t[_];f.clippingState=g,this.numIntersection=M?this.numPlanes:0,this.numPlanes+=b}};function c(){l.value!==t&&(l.value=t,l.needsUpdate=n>0),e.numPlanes=n,e.numIntersection=0}function h(d,u,p,x){let M=d!==null?d.length:0,m=null;if(M!==0){if(m=l.value,x!==!0||m===null){let f=p+M*4,b=u.matrixWorldInverse;o.getNormalMatrix(b),(m===null||m.length<f)&&(m=new Float32Array(f));for(let E=0,g=p;E!==M;++E,g+=4)a.copy(d[E]).applyMatrix4(b,o),a.normal.toArray(m,g),m[g+3]=a.constant}l.value=m,l.needsUpdate=!0}return e.numPlanes=M,e.numIntersection=0,m}}var sa=4,Fv=6,Nv=20,Uv=256,Mo=new ki,bp=new Le,Mu=null,wu=0,Su=0,Eu=!1,zv=new U,or=new U,aa=class{constructor(e){this._renderer=e,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._sizeLods=[],this._lodMeshes=[],this._backgroundBox=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._blurMaterial=null,this._ggxMaterial=null}fromScene(e,t=0,n=.1,i=100,r={}){let{size:a=256,position:o=zv}=r;Mu=this._renderer.getRenderTarget(),wu=this._renderer.getActiveCubeFace(),Su=this._renderer.getActiveMipmapLevel(),Eu=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(a);let l=this._allocateTargets();return l.depthBuffer=!0,this._sceneToCubeUV(e,n,i,l,o),t>0&&this._blur(l,0,0,t),this._applyPMREM(l),this._cleanup(l),l}fromEquirectangular(e,t=null){return this._fromTexture(e,t)}fromCubemap(e,t=null){return this._fromTexture(e,t)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=_p(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=yp(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose(),this._backgroundBox!==null&&(this._backgroundBox.geometry.dispose(),this._backgroundBox.material.dispose())}_setSize(e){this._lodMax=Math.floor(Math.log2(e)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._ggxMaterial!==null&&this._ggxMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let e=0;e<this._lodMeshes.length;e++)this._lodMeshes[e].geometry.dispose()}_cleanup(e){this._renderer.setRenderTarget(Mu,wu,Su),this._renderer.xr.enabled=Eu,e.scissorTest=!1,ia(e,0,0,e.width,e.height)}_fromTexture(e,t){e.mapping===Rs||e.mapping===sr?this._setSize(e.image.length===0?16:e.image[0].width||e.image[0].image.width):this._setSize(e.image.width/4),Mu=this._renderer.getRenderTarget(),wu=this._renderer.getActiveCubeFace(),Su=this._renderer.getActiveMipmapLevel(),Eu=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let n=t||this._allocateTargets();return this._textureToCubeUV(e,n),this._applyPMREM(n),this._cleanup(n),n}_allocateTargets(){let e=3*Math.max(this._cubeSize,112),t=4*this._cubeSize,n={magFilter:on,minFilter:on,generateMipmaps:!1,type:xn,format:ti,colorSpace:Hn,depthBuffer:!1},i=vp(e,t,n);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==e||this._pingPongRenderTarget.height!==t){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=vp(e,t,n);let{_lodMax:r}=this;({lodMeshes:this._lodMeshes,sizeLods:this._sizeLods}=Ov(r)),this._blurMaterial=Bv(r,e,t),this._ggxMaterial=kv(r,e,t)}return i}_compileMaterial(e){let t=new ht(new At,e);this._renderer.compile(t,Mo)}_sceneToCubeUV(e,t,n,i,r){let l=new gn(90,1,t,n),c=[1,-1,1,1,1,1],h=[1,1,1,-1,-1,-1],d=this._renderer,u=d.autoClear,p=d.toneMapping;d.getClearColor(bp),d.toneMapping=Si,d.autoClear=!1,d.state.buffers.depth.getReversed()&&(d.setRenderTarget(i),d.clearDepth(),d.setRenderTarget(null)),this._backgroundBox===null&&(this._backgroundBox=new ht(new _i,new jt({name:"PMREM.Background",side:yn,depthWrite:!1,depthTest:!1})));let M=this._backgroundBox,m=M.material,f=!1,b=e.background;b?b.isColor&&(m.color.copy(b),e.background=null,f=!0):(m.color.copy(bp),f=!0);for(let E=0;E<6;E++){let g=E%3;g===0?(l.up.set(0,c[E],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x+h[E],r.y,r.z)):g===1?(l.up.set(0,0,c[E]),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y+h[E],r.z)):(l.up.set(0,c[E],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y,r.z+h[E]));let _=this._cubeSize;ia(i,g*_,E>2?_:0,_,_),d.setRenderTarget(i),f&&d.render(M,l),d.render(e,l)}d.toneMapping=p,d.autoClear=u,e.background=b}_textureToCubeUV(e,t){let n=this._renderer,i=e.mapping===Rs||e.mapping===sr;i?(this._cubemapMaterial===null&&(this._cubemapMaterial=_p()),this._cubemapMaterial.uniforms.flipEnvMap.value=e.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=yp());let r=i?this._cubemapMaterial:this._equirectMaterial,a=this._lodMeshes[0];a.material=r;let o=r.uniforms;o.envMap.value=e;let l=this._cubeSize;ia(t,0,0,3*l,2*l),n.setRenderTarget(t),n.render(a,Mo)}_applyPMREM(e){let t=this._renderer,n=t.autoClear;t.autoClear=!1;let i=this._lodMeshes.length;for(let r=1;r<i;r++)this._applyGGXFilter(e,r-1,r);t.autoClear=n}_applyGGXFilter(e,t,n){let i=this._renderer,r=this._pingPongRenderTarget,a=this._ggxMaterial,o=this._lodMeshes[n];o.material=a;let l=a.uniforms,c=n/(this._lodMeshes.length-1),h=t/(this._lodMeshes.length-1),d=Math.sqrt(c*c-h*h),u=c*1.25,p=d*u,{_lodMax:x}=this,M=this._sizeLods[n],m=3*M*(n>x-sa?n-x+sa:0),f=4*(this._cubeSize-M);l.envMap.value=e.texture,l.roughness.value=p,l.mipInt.value=x-t,ia(r,m,f,3*M,2*M),i.setRenderTarget(r),i.render(o,Mo),l.envMap.value=r.texture,l.roughness.value=0,l.mipInt.value=x-n,ia(e,m,f,3*M,2*M),i.setRenderTarget(e),i.render(o,Mo)}_blur(e,t,n,i){let r=this._pingPongRenderTarget,a=Math.min(i,Math.PI)/Math.SQRT2;this._blurPass(e,r,t,n,a),this._blurPass(r,e,n,n,a)}_blurPass(e,t,n,i,r){let a=this._renderer,o=this._blurMaterial,l=this._lodMeshes[i];l.material=o;let c=o.uniforms;c.envMap.value=e.texture,c.sigma.value=r,c.mipInt.value=this._lodMax-n;let h=this._sizeLods[i],d=3*h*(i>this._lodMax-sa?i-this._lodMax+sa:0),u=4*(this._cubeSize-h);ia(t,d,u,3*h,2*h),a.setRenderTarget(t),a.render(l,Mo)}};function Ov(s){let e=[],t=[],n=s,i=s-sa+1+Fv;for(let r=0;r<i;r++){let a=Math.pow(2,n);e.push(a);let o=1/(a-2),l=-o,c=1+o,h=[l,l,c,l,c,c,l,l,c,c,l,c],d=6,u=6,p=3,x=new Float32Array(p*u*d),M=new Float32Array(p*u*d);for(let f=0;f<d;f++){let b=f%3*2/3-1,E=f>2?0:-1,g=[b,E,0,b+2/3,E,0,b+2/3,E+1,0,b,E,0,b+2/3,E+1,0,b,E+1,0];x.set(g,p*u*f);for(let _=0;_<u;_++){let w=h[_*2]*2-1,T=h[_*2+1]*2-1;f===0?or.set(1,T,w):f===1?or.set(-w,1,-T):f===2?or.set(-w,T,1):f===3?or.set(-1,T,-w):f===4?or.set(-w,-1,T):or.set(w,T,-1),or.toArray(M,(f*u+_)*p)}}let m=new At;m.setAttribute("position",new Pt(x,p)),m.setAttribute("outputDirection",new Pt(M,p)),t.push(new ht(m,null)),n>sa&&n--}return{lodMeshes:t,sizeLods:e}}function vp(s,e,t){let n=new ln(s,e,t);return n.texture.mapping=fo,n.texture.name="PMREM.cubeUv",n.scissorTest=!0,n}function ia(s,e,t,n,i){s.viewport.set(e,t,n,i),s.scissor.set(e,t,n,i)}function kv(s,e,t){return new Rt({name:"PMREMGGXConvolution",defines:{GGX_SAMPLES:Uv,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},roughness:{value:0},mipInt:{value:0}},vertexShader:Gc(),fragmentShader:`

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
		`,blending:li,depthTest:!1,depthWrite:!1})}function Bv(s,e,t){return new Rt({name:"SphericalGaussianBlur",defines:{SAMPLES:Nv,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},sigma:{value:0},mipInt:{value:0}},vertexShader:Gc(),fragmentShader:`

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
		`,blending:li,depthTest:!1,depthWrite:!1})}function yp(){return new Rt({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:Gc(),fragmentShader:`

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
		`,blending:li,depthTest:!1,depthWrite:!1})}function _p(){return new Rt({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:Gc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:li,depthTest:!1,depthWrite:!1})}function Gc(){return`

		precision mediump float;
		precision mediump int;

		attribute vec3 outputDirection;

		varying vec3 vOutputDirection;

		void main() {

			vOutputDirection = outputDirection;
			gl_Position = vec4( position, 1.0 );

		}
	`}var Bc=class extends ln{constructor(e=1,t={}){super(e,e,t),this.isWebGLCubeRenderTarget=!0;let n={width:e,height:e,depth:1},i=[n,n,n,n,n,n];this.texture=new Ga(i),this._setTextureOptions(t),this.texture.isRenderTargetTexture=!0}fromEquirectangularTexture(e,t){this.texture.type=t.type,this.texture.colorSpace=t.colorSpace,this.texture.generateMipmaps=t.generateMipmaps,this.texture.minFilter=t.minFilter,this.texture.magFilter=t.magFilter;let n={uniforms:{tEquirect:{value:null}},vertexShader:`

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
			`},i=new _i(5,5,5),r=new Rt({name:"CubemapFromEquirect",uniforms:ar(n.uniforms),vertexShader:n.vertexShader,fragmentShader:n.fragmentShader,side:yn,blending:li});r.uniforms.tEquirect.value=t;let a=new ht(i,r),o=t.minFilter;return t.minFilter===Ei&&(t.minFilter=on),new Vl(1,10,this).update(e,a),t.minFilter=o,a.geometry.dispose(),a.material.dispose(),this}clear(e,t=!0,n=!0,i=!0){let r=e.getRenderTarget();for(let a=0;a<6;a++)e.setRenderTarget(this,a),e.clear(t,n,i);e.setRenderTarget(r)}};function Hv(s){let e=new WeakMap,t=new WeakMap,n=null;function i(u,p=!1){return u==null?null:p?a(u):r(u)}function r(u){if(u&&u.isTexture){let p=u.mapping;if(p===Kl||p===Yl)if(e.has(u)){let x=e.get(u).texture;return o(x,u.mapping)}else{let x=u.image;if(x&&x.height>0){let M=new Bc(x.height);return M.fromEquirectangularTexture(s,u),e.set(u,M),u.addEventListener("dispose",c),o(M.texture,u.mapping)}else return null}}return u}function a(u){if(u&&u.isTexture){let p=u.mapping,x=p===Kl||p===Yl,M=p===Rs||p===sr;if(x||M){let m=t.get(u),f=m!==void 0?m.texture.pmremVersion:0;if(u.isRenderTargetTexture&&u.pmremVersion!==f)return n===null&&(n=new aa(s)),m=x?n.fromEquirectangular(u,m):n.fromCubemap(u,m),m.texture.pmremVersion=u.pmremVersion,t.set(u,m),m.texture;if(m!==void 0)return m.texture;{let b=u.image;return x&&b&&b.height>0||M&&b&&l(b)?(n===null&&(n=new aa(s)),m=x?n.fromEquirectangular(u):n.fromCubemap(u),m.texture.pmremVersion=u.pmremVersion,t.set(u,m),u.addEventListener("dispose",h),m.texture):null}}}return u}function o(u,p){return p===Kl?u.mapping=Rs:p===Yl&&(u.mapping=sr),u}function l(u){let p=0,x=6;for(let M=0;M<x;M++)u[M]!==void 0&&p++;return p===x}function c(u){let p=u.target;p.removeEventListener("dispose",c);let x=e.get(p);x!==void 0&&(e.delete(p),x.dispose())}function h(u){let p=u.target;p.removeEventListener("dispose",h);let x=t.get(p);x!==void 0&&(t.delete(p),x.dispose())}function d(){e=new WeakMap,t=new WeakMap,n!==null&&(n.dispose(),n=null)}return{get:i,dispose:d}}function Gv(s){let e={};function t(n){if(e[n]!==void 0)return e[n];let i=s.getExtension(n);return e[n]=i,i}return{has:function(n){return t(n)!==null},init:function(){t("EXT_color_buffer_float"),t("WEBGL_clip_cull_distance"),t("OES_texture_float_linear"),t("EXT_color_buffer_half_float"),t("WEBGL_multisampled_render_to_texture"),t("WEBGL_render_shared_exponent")},get:function(n){let i=t(n);return i===null&&qs("WebGLRenderer: "+n+" extension not supported."),i}}}function Vv(s,e,t,n){let i={},r=new WeakMap;function a(d){let u=d.target;u.index!==null&&e.remove(u.index);for(let x in u.attributes)e.remove(u.attributes[x]);u.removeEventListener("dispose",a),delete i[u.id];let p=r.get(u);p&&(e.remove(p),r.delete(u)),n.releaseStatesOfGeometry(u),u.isInstancedBufferGeometry===!0&&delete u._maxInstanceCount,t.memory.geometries--}function o(d,u){return i[u.id]===!0||(u.addEventListener("dispose",a),i[u.id]=!0,t.memory.geometries++),u}function l(d){let u=d.attributes;for(let p in u)e.update(u[p],s.ARRAY_BUFFER)}function c(d){let u=[],p=d.index,x=d.attributes.position,M=0;if(x===void 0)return;if(p!==null){let b=p.array;M=p.version;for(let E=0,g=b.length;E<g;E+=3){let _=b[E+0],w=b[E+1],T=b[E+2];u.push(_,w,w,T,T,_)}}else{let b=x.array;M=x.version;for(let E=0,g=b.length/3-1;E<g;E+=3){let _=E+0,w=E+1,T=E+2;u.push(_,w,w,T,T,_)}}let m=new(x.count>=65535?Oa:za)(u,1);m.version=M;let f=r.get(d);f&&e.remove(f),r.set(d,m)}function h(d){let u=r.get(d);if(u){let p=d.index;p!==null&&u.version<p.version&&c(d)}else c(d);return r.get(d)}return{get:o,update:l,getWireframeAttribute:h}}function Wv(s,e,t){let n;function i(d){n=d}let r,a;function o(d){r=d.type,a=d.bytesPerElement}function l(d,u){s.drawElements(n,u,r,d*a),t.update(u,n,1)}function c(d,u,p){p!==0&&(s.drawElementsInstanced(n,u,r,d*a,p),t.update(u,n,p))}function h(d,u,p){if(p===0)return;e.get("WEBGL_multi_draw").multiDrawElementsWEBGL(n,u,0,r,d,0,p);let M=0;for(let m=0;m<p;m++)M+=u[m];t.update(M,n,1)}this.setMode=i,this.setIndex=o,this.render=l,this.renderInstances=c,this.renderMultiDraw=h}function qv(s){let e={geometries:0,textures:0},t={frame:0,calls:0,triangles:0,points:0,lines:0};function n(r,a,o){switch(t.calls++,a){case s.TRIANGLES:t.triangles+=o*(r/3);break;case s.LINES:t.lines+=o*(r/2);break;case s.LINE_STRIP:t.lines+=o*(r-1);break;case s.LINE_LOOP:t.lines+=o*r;break;case s.POINTS:t.points+=o*r;break;default:_t("WebGLInfo: Unknown draw mode:",a);break}}function i(){t.calls=0,t.triangles=0,t.points=0,t.lines=0}return{memory:e,render:t,programs:null,autoReset:!0,reset:i,update:n}}function Xv(s,e,t){let n=new WeakMap,i=new Ot;function r(a,o,l){let c=a.morphTargetInfluences,h=o.morphAttributes.position||o.morphAttributes.normal||o.morphAttributes.color,d=h!==void 0?h.length:0,u=n.get(o);if(u===void 0||u.count!==d){let C=function(){T.dispose(),n.delete(o),o.removeEventListener("dispose",C)};u!==void 0&&u.texture.dispose();let p=o.morphAttributes.position!==void 0,x=o.morphAttributes.normal!==void 0,M=o.morphAttributes.color!==void 0,m=o.morphAttributes.position||[],f=o.morphAttributes.normal||[],b=o.morphAttributes.color||[],E=0;p===!0&&(E=1),x===!0&&(E=2),M===!0&&(E=3);let g=o.attributes.position.count*E,_=1;g>e.maxTextureSize&&(_=Math.ceil(g/e.maxTextureSize),g=e.maxTextureSize);let w=new Float32Array(g*_*4*d),T=new Na(w,g,_,d);T.type=ei,T.needsUpdate=!0;let y=E*4;for(let S=0;S<d;S++){let D=m[S],v=f[S],I=b[S],P=g*_*4*S;for(let N=0;N<D.count;N++){let z=N*y;p===!0&&(i.fromBufferAttribute(D,N),w[P+z+0]=i.x,w[P+z+1]=i.y,w[P+z+2]=i.z,w[P+z+3]=0),x===!0&&(i.fromBufferAttribute(v,N),w[P+z+4]=i.x,w[P+z+5]=i.y,w[P+z+6]=i.z,w[P+z+7]=0),M===!0&&(i.fromBufferAttribute(I,N),w[P+z+8]=i.x,w[P+z+9]=i.y,w[P+z+10]=i.z,w[P+z+11]=I.itemSize===4?i.w:1)}}u={count:d,texture:T,size:new Ge(g,_)},n.set(o,u),o.addEventListener("dispose",C)}if(a.isInstancedMesh===!0&&a.morphTexture!==null)l.getUniforms().setValue(s,"morphTexture",a.morphTexture,t);else{let p=0;for(let M=0;M<c.length;M++)p+=c[M];let x=o.morphTargetsRelative?1:1-p;l.getUniforms().setValue(s,"morphTargetBaseInfluence",x),l.getUniforms().setValue(s,"morphTargetInfluences",c)}l.getUniforms().setValue(s,"morphTargetsTexture",u.texture,t),l.getUniforms().setValue(s,"morphTargetsTextureSize",u.size)}return{update:r}}function jv(s,e,t,n,i){let r=new WeakMap;function a(c){let h=i.render.frame,d=c.geometry,u=e.get(c,d);if(r.get(u)!==h&&(e.update(u),r.set(u,h)),c.isInstancedMesh&&(c.hasEventListener("dispose",l)===!1&&c.addEventListener("dispose",l),r.get(c)!==h&&(t.update(c.instanceMatrix,s.ARRAY_BUFFER),c.instanceColor!==null&&t.update(c.instanceColor,s.ARRAY_BUFFER),r.set(c,h))),c.isSkinnedMesh){let p=c.skeleton;r.get(p)!==h&&(p.update(),r.set(p,h))}return u}function o(){r=new WeakMap}function l(c){let h=c.target;h.removeEventListener("dispose",l),n.releaseStatesOfObject(h),t.remove(h.instanceMatrix),h.instanceColor!==null&&t.remove(h.instanceColor)}return{update:a,dispose:o}}var Kv={[ao]:"LINEAR_TONE_MAPPING",[oo]:"REINHARD_TONE_MAPPING",[lo]:"CINEON_TONE_MAPPING",[ir]:"ACES_FILMIC_TONE_MAPPING",[ho]:"AGX_TONE_MAPPING",[uo]:"NEUTRAL_TONE_MAPPING",[co]:"CUSTOM_TONE_MAPPING"};function Yv(s,e,t,n,i,r){let a=new ln(e,t,{type:s,depthBuffer:i,stencilBuffer:r,samples:n?4:0,storeMultisampledDepthBuffer:!1,storeMultisampledStencilBuffer:!1,resolveDepthBuffer:!1,resolveStencilBuffer:!1}),o=null,l=null,c=new At;c.setAttribute("position",new Mt([-1,3,0,-1,-1,0,3,-1,0],3)),c.setAttribute("uv",new Mt([0,2,0,0,2,0],2));let h=new jr({uniforms:{tDiffuse:{value:null}},vertexShader:`
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
			}`,depthTest:!1,depthWrite:!1}),d=new ht(c,h),u=new ki(-1,1,1,-1,0,1),p=null,x=null,M=!1,m,f=null,b=[],E=!1;this.setSize=function(g,_){a.setSize(g,_),o!==null&&o.setSize(g,_),l!==null&&l.setSize(g,_);for(let w=0;w<b.length;w++){let T=b[w];T.setSize&&T.setSize(g,_)}},this.setEffects=function(g){b=g,E=b.length>0&&b[0].isRenderPass===!0;let _=a.width,w=a.height;b.length>0&&o===null&&(o=new ln(_,w,{type:xn,depthBuffer:!1,stencilBuffer:!1}),l=new ln(_,w,{type:xn,depthBuffer:!1,stencilBuffer:!1}));for(let T=0;T<b.length;T++){let y=b[T];y.setSize&&y.setSize(_,w)}},this.begin=function(g,_){if(M||g.toneMapping===Si&&b.length===0)return!1;if(f=_,_!==null){let w=_.width,T=_.height;(a.width!==w||a.height!==T)&&this.setSize(w,T)}return E===!1&&g.setRenderTarget(a),m=g.toneMapping,g.toneMapping=Si,!0},this.hasRenderPass=function(){return E},this.end=function(g,_){g.toneMapping=m,M=!0;let w=a,T=o;for(let y=0;y<b.length;y++){let C=b[y];C.enabled!==!1&&(C.render(g,T,w,_),C.needsSwap!==!1&&(w=T,T=T===o?l:o))}if(p!==g.outputColorSpace||x!==g.toneMapping){p=g.outputColorSpace,x=g.toneMapping,h.defines={},Dt.getTransfer(p)===Xt&&(h.defines.SRGB_TRANSFER="");let y=Kv[x];y&&(h.defines[y]=""),h.needsUpdate=!0}h.uniforms.tDiffuse.value=w.texture,g.setRenderTarget(f),g.render(d,u),f=null,M=!1},this.isCompositing=function(){return M},this.dispose=function(){a.dispose(),o!==null&&o.dispose(),l!==null&&l.dispose(),c.dispose(),h.dispose()}}var Hp=new hn,Ru=new Ms(1,1),Gp=new Na,Vp=new Sl,Wp=new Ga,Mp=[],wp=[],Sp=new Float32Array(16),Ep=new Float32Array(9),Tp=new Float32Array(4);function oa(s,e,t){let n=s[0];if(n<=0||n>0)return s;let i=e*t,r=Mp[i];if(r===void 0&&(r=new Float32Array(i),Mp[i]=r),e!==0){n.toArray(r,0);for(let a=1,o=0;a!==e;++a)o+=t,s[a].toArray(r,o)}return r}function _n(s,e){if(s.length!==e.length)return!1;for(let t=0,n=s.length;t<n;t++)if(s[t]!==e[t])return!1;return!0}function Mn(s,e){for(let t=0,n=e.length;t<n;t++)s[t]=e[t]}function Vc(s,e){let t=wp[e];t===void 0&&(t=new Int32Array(e),wp[e]=t);for(let n=0;n!==e;++n)t[n]=s.allocateTextureUnit();return t}function Jv(s,e){let t=this.cache;t[0]!==e&&(s.uniform1f(this.addr,e),t[0]=e)}function Zv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2f(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(_n(t,e))return;s.uniform2fv(this.addr,e),Mn(t,e)}}function $v(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3f(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else if(e.r!==void 0)(t[0]!==e.r||t[1]!==e.g||t[2]!==e.b)&&(s.uniform3f(this.addr,e.r,e.g,e.b),t[0]=e.r,t[1]=e.g,t[2]=e.b);else{if(_n(t,e))return;s.uniform3fv(this.addr,e),Mn(t,e)}}function Qv(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4f(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(_n(t,e))return;s.uniform4fv(this.addr,e),Mn(t,e)}}function ey(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(_n(t,e))return;s.uniformMatrix2fv(this.addr,!1,e),Mn(t,e)}else{if(_n(t,n))return;Tp.set(n),s.uniformMatrix2fv(this.addr,!1,Tp),Mn(t,n)}}function ty(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(_n(t,e))return;s.uniformMatrix3fv(this.addr,!1,e),Mn(t,e)}else{if(_n(t,n))return;Ep.set(n),s.uniformMatrix3fv(this.addr,!1,Ep),Mn(t,n)}}function ny(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(_n(t,e))return;s.uniformMatrix4fv(this.addr,!1,e),Mn(t,e)}else{if(_n(t,n))return;Sp.set(n),s.uniformMatrix4fv(this.addr,!1,Sp),Mn(t,n)}}function iy(s,e){let t=this.cache;t[0]!==e&&(s.uniform1i(this.addr,e),t[0]=e)}function sy(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2i(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(_n(t,e))return;s.uniform2iv(this.addr,e),Mn(t,e)}}function ry(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3i(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(_n(t,e))return;s.uniform3iv(this.addr,e),Mn(t,e)}}function ay(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4i(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(_n(t,e))return;s.uniform4iv(this.addr,e),Mn(t,e)}}function oy(s,e){let t=this.cache;t[0]!==e&&(s.uniform1ui(this.addr,e),t[0]=e)}function ly(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2ui(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(_n(t,e))return;s.uniform2uiv(this.addr,e),Mn(t,e)}}function cy(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3ui(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(_n(t,e))return;s.uniform3uiv(this.addr,e),Mn(t,e)}}function hy(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4ui(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(_n(t,e))return;s.uniform4uiv(this.addr,e),Mn(t,e)}}function uy(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i);let r;this.type===s.SAMPLER_2D_SHADOW?(Ru.compareFunction=t.isReversedDepthBuffer()?zc:Uc,r=Ru):r=Hp,t.setTexture2D(e||r,i)}function dy(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture3D(e||Vp,i)}function fy(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTextureCube(e||Wp,i)}function py(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture2DArray(e||Gp,i)}function my(s){switch(s){case 5126:return Jv;case 35664:return Zv;case 35665:return $v;case 35666:return Qv;case 35674:return ey;case 35675:return ty;case 35676:return ny;case 5124:case 35670:return iy;case 35667:case 35671:return sy;case 35668:case 35672:return ry;case 35669:case 35673:return ay;case 5125:return oy;case 36294:return ly;case 36295:return cy;case 36296:return hy;case 35678:case 36198:case 36298:case 36306:case 35682:return uy;case 35679:case 36299:case 36307:return dy;case 35680:case 36300:case 36308:case 36293:return fy;case 36289:case 36303:case 36311:case 36292:return py}}function gy(s,e){s.uniform1fv(this.addr,e)}function xy(s,e){let t=oa(e,this.size,2);s.uniform2fv(this.addr,t)}function by(s,e){let t=oa(e,this.size,3);s.uniform3fv(this.addr,t)}function vy(s,e){let t=oa(e,this.size,4);s.uniform4fv(this.addr,t)}function yy(s,e){let t=oa(e,this.size,4);s.uniformMatrix2fv(this.addr,!1,t)}function _y(s,e){let t=oa(e,this.size,9);s.uniformMatrix3fv(this.addr,!1,t)}function My(s,e){let t=oa(e,this.size,16);s.uniformMatrix4fv(this.addr,!1,t)}function wy(s,e){s.uniform1iv(this.addr,e)}function Sy(s,e){s.uniform2iv(this.addr,e)}function Ey(s,e){s.uniform3iv(this.addr,e)}function Ty(s,e){s.uniform4iv(this.addr,e)}function Ay(s,e){s.uniform1uiv(this.addr,e)}function Ry(s,e){s.uniform2uiv(this.addr,e)}function Cy(s,e){s.uniform3uiv(this.addr,e)}function Py(s,e){s.uniform4uiv(this.addr,e)}function Iy(s,e,t){let n=this.cache,i=e.length,r=Vc(t,i);_n(n,r)||(s.uniform1iv(this.addr,r),Mn(n,r));let a;this.type===s.SAMPLER_2D_SHADOW?a=Ru:a=Hp;for(let o=0;o!==i;++o)t.setTexture2D(e[o]||a,r[o])}function Dy(s,e,t){let n=this.cache,i=e.length,r=Vc(t,i);_n(n,r)||(s.uniform1iv(this.addr,r),Mn(n,r));for(let a=0;a!==i;++a)t.setTexture3D(e[a]||Vp,r[a])}function Ly(s,e,t){let n=this.cache,i=e.length,r=Vc(t,i);_n(n,r)||(s.uniform1iv(this.addr,r),Mn(n,r));for(let a=0;a!==i;++a)t.setTextureCube(e[a]||Wp,r[a])}function Fy(s,e,t){let n=this.cache,i=e.length,r=Vc(t,i);_n(n,r)||(s.uniform1iv(this.addr,r),Mn(n,r));for(let a=0;a!==i;++a)t.setTexture2DArray(e[a]||Gp,r[a])}function Ny(s){switch(s){case 5126:return gy;case 35664:return xy;case 35665:return by;case 35666:return vy;case 35674:return yy;case 35675:return _y;case 35676:return My;case 5124:case 35670:return wy;case 35667:case 35671:return Sy;case 35668:case 35672:return Ey;case 35669:case 35673:return Ty;case 5125:return Ay;case 36294:return Ry;case 36295:return Cy;case 36296:return Py;case 35678:case 36198:case 36298:case 36306:case 35682:return Iy;case 35679:case 36299:case 36307:return Dy;case 35680:case 36300:case 36308:case 36293:return Ly;case 36289:case 36303:case 36311:case 36292:return Fy}}var Cu=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.setValue=my(t.type)}},Pu=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.size=t.size,this.setValue=Ny(t.type)}},Iu=class{constructor(e){this.id=e,this.seq=[],this.map={}}setValue(e,t,n){let i=this.seq;for(let r=0,a=i.length;r!==a;++r){let o=i[r];o.setValue(e,t[o.id],n)}}},Tu=/(\w+)(\])?(\[|\.)?/g;function Ap(s,e){s.seq.push(e),s.map[e.id]=e}function Uy(s,e,t){let n=s.name,i=n.length;for(Tu.lastIndex=0;;){let r=Tu.exec(n),a=Tu.lastIndex,o=r[1],l=r[2]==="]",c=r[3];if(l&&(o=o|0),c===void 0||c==="["&&a+2===i){Ap(t,c===void 0?new Cu(o,s,e):new Pu(o,s,e));break}else{let d=t.map[o];d===void 0&&(d=new Iu(o),Ap(t,d)),t=d}}}var ra=class{constructor(e,t){this.seq=[],this.map={};let n=e.getProgramParameter(t,e.ACTIVE_UNIFORMS);for(let a=0;a<n;++a){let o=e.getActiveUniform(t,a),l=e.getUniformLocation(t,o.name);Uy(o,l,this)}let i=[],r=[];for(let a of this.seq)a.type===e.SAMPLER_2D_SHADOW||a.type===e.SAMPLER_CUBE_SHADOW||a.type===e.SAMPLER_2D_ARRAY_SHADOW?i.push(a):r.push(a);i.length>0&&(this.seq=i.concat(r))}setValue(e,t,n,i){let r=this.map[t];r!==void 0&&r.setValue(e,n,i)}setOptional(e,t,n){let i=t[n];i!==void 0&&this.setValue(e,n,i)}static upload(e,t,n,i){for(let r=0,a=t.length;r!==a;++r){let o=t[r],l=n[o.id];l.needsUpdate!==!1&&o.setValue(e,l.value,i)}}static seqWithValue(e,t){let n=[];for(let i=0,r=e.length;i!==r;++i){let a=e[i];a.id in t&&n.push(a)}return n}};function Rp(s,e,t){let n=s.createShader(e);return s.shaderSource(n,t),s.compileShader(n),n}var zy=37297,Oy=0;function ky(s,e){let t=s.split(`
`),n=[],i=Math.max(e-6,0),r=Math.min(e+6,t.length);for(let a=i;a<r;a++){let o=a+1;n.push(`${o===e?">":" "} ${o}: ${t[a]}`)}return n.join(`
`)}var Cp=new Tt;function By(s){Dt._getMatrix(Cp,Dt.workingColorSpace,s);let e=`mat3( ${Cp.elements.map(t=>t.toFixed(4))} )`;switch(Dt.getTransfer(s)){case La:return[e,"LinearTransferOETF"];case Xt:return[e,"sRGBTransferOETF"];default:return dt("WebGLProgram: Unsupported color space: ",s),[e,"LinearTransferOETF"]}}function Pp(s,e,t){let n=s.getShaderParameter(e,s.COMPILE_STATUS),r=(s.getShaderInfoLog(e)||"").trim();if(n&&r==="")return"";let a=/ERROR: 0:(\d+)/.exec(r);if(a){let o=parseInt(a[1]);return t.toUpperCase()+`

`+r+`

`+ky(s.getShaderSource(e),o)}else return r}function Hy(s,e){let t=By(e);return[`vec4 ${s}( vec4 value ) {`,`	return ${t[1]}( vec4( value.rgb * ${t[0]}, value.a ) );`,"}"].join(`
`)}var Gy={[ao]:"Linear",[oo]:"Reinhard",[lo]:"Cineon",[ir]:"ACESFilmic",[ho]:"AgX",[uo]:"Neutral",[co]:"Custom"};function Vy(s,e){let t=Gy[e];return t===void 0?(dt("WebGLProgram: Unsupported toneMapping:",e),"vec3 "+s+"( vec3 color ) { return LinearToneMapping( color ); }"):"vec3 "+s+"( vec3 color ) { return "+t+"ToneMapping( color ); }"}var kc=new U;function Wy(){Dt.getLuminanceCoefficients(kc);let s=kc.x.toFixed(4),e=kc.y.toFixed(4),t=kc.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${s}, ${e}, ${t} );`,"	return dot( weights, rgb );","}"].join(`
`)}function qy(s){return[s.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",s.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(So).join(`
`)}function Xy(s){let e=[];for(let t in s){let n=s[t];n!==!1&&e.push("#define "+t+" "+n)}return e.join(`
`)}function jy(s,e){let t={},n=s.getProgramParameter(e,s.ACTIVE_ATTRIBUTES);for(let i=0;i<n;i++){let r=s.getActiveAttrib(e,i),a=r.name,o=1;r.type===s.FLOAT_MAT2&&(o=2),r.type===s.FLOAT_MAT3&&(o=3),r.type===s.FLOAT_MAT4&&(o=4),t[a]={type:r.type,location:s.getAttribLocation(e,a),locationSize:o}}return t}function So(s){return s!==""}function Ip(s,e){let t=e.numSpotLightShadows+e.numSpotLightMaps-e.numSpotLightShadowsWithMaps;return s.replace(/NUM_SUN_LIGHTS/g,e.numSunLights).replace(/NUM_DIR_LIGHTS/g,e.numDirLights).replace(/NUM_SPOT_LIGHTS/g,e.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,e.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,t).replace(/NUM_RECT_AREA_LIGHTS/g,e.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,e.numPointLights).replace(/NUM_HEMI_LIGHTS/g,e.numHemiLights).replace(/NUM_SUN_LIGHT_SHADOWS/g,e.numSunLightShadows).replace(/NUM_DIR_LIGHT_SHADOWS/g,e.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,e.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,e.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,e.numPointLightShadows)}function Dp(s,e){return s.replace(/NUM_CLIPPING_PLANES/g,e.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,e.numClippingPlanes-e.numClipIntersection)}var Ky=/^[ \t]*#include +<([\w\d./]+)>/gm;function Du(s){return s.replace(Ky,Jy)}var Yy=new Map;function Jy(s,e){let t=Ft[e];if(t===void 0){let n=Yy.get(e);if(n!==void 0)t=Ft[n],dt('WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',e,n);else throw new Error("THREE.WebGLProgram: Can not resolve #include <"+e+">")}return Du(t)}var Zy=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function Lp(s){return s.replace(Zy,$y)}function $y(s,e,t,n){let i="";for(let r=parseInt(e);r<parseInt(t);r++)i+=n.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return i}function Fp(s){let e=`precision ${s.precision} float;
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
#define LOW_PRECISION`),e}var Qy={[tr]:"SHADOWMAP_TYPE_PCF",[Zr]:"SHADOWMAP_TYPE_VSM"};function e_(s){return Qy[s.shadowMapType]||"SHADOWMAP_TYPE_BASIC"}var t_={[Rs]:"ENVMAP_TYPE_CUBE",[sr]:"ENVMAP_TYPE_CUBE",[fo]:"ENVMAP_TYPE_CUBE_UV"};function n_(s){return s.envMap===!1?"ENVMAP_TYPE_CUBE":t_[s.envMapMode]||"ENVMAP_TYPE_CUBE"}var i_={[sr]:"ENVMAP_MODE_REFRACTION"};function s_(s){return s.envMap===!1?"ENVMAP_MODE_REFLECTION":i_[s.envMapMode]||"ENVMAP_MODE_REFLECTION"}var r_={[jl]:"ENVMAP_BLENDING_MULTIPLY",[Jf]:"ENVMAP_BLENDING_MIX",[Zf]:"ENVMAP_BLENDING_ADD"};function a_(s){return s.envMap===!1?"ENVMAP_BLENDING_NONE":r_[s.combine]||"ENVMAP_BLENDING_NONE"}function o_(s){let e=s.envMapCubeUVHeight;if(e===null)return null;let t=Math.log2(e)-2,n=1/e;return{texelWidth:1/(3*Math.max(Math.pow(2,t),112)),texelHeight:n,maxMip:t}}function l_(s,e,t,n){let i=s.getContext(),r=t.defines,a=t.vertexShader,o=t.fragmentShader,l=e_(t),c=n_(t),h=s_(t),d=a_(t),u=o_(t),p=qy(t),x=Xy(r),M=i.createProgram(),m,f,b=t.glslVersion?"#version "+t.glslVersion+`
`:"";t.isRawShaderMaterial?(m=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x].filter(So).join(`
`),m.length>0&&(m+=`
`),f=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x].filter(So).join(`
`),f.length>0&&(f+=`
`)):(m=[Fp(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x,t.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",t.batching?"#define USE_BATCHING":"",t.batchingColor?"#define USE_BATCHING_COLOR":"",t.instancing?"#define USE_INSTANCING":"",t.instancingColor?"#define USE_INSTANCING_COLOR":"",t.instancingMorph?"#define USE_INSTANCING_MORPH":"",t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.map?"#define USE_MAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+h:"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.displacementMap?"#define USE_DISPLACEMENTMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.mapUv?"#define MAP_UV "+t.mapUv:"",t.alphaMapUv?"#define ALPHAMAP_UV "+t.alphaMapUv:"",t.lightMapUv?"#define LIGHTMAP_UV "+t.lightMapUv:"",t.aoMapUv?"#define AOMAP_UV "+t.aoMapUv:"",t.emissiveMapUv?"#define EMISSIVEMAP_UV "+t.emissiveMapUv:"",t.bumpMapUv?"#define BUMPMAP_UV "+t.bumpMapUv:"",t.normalMapUv?"#define NORMALMAP_UV "+t.normalMapUv:"",t.displacementMapUv?"#define DISPLACEMENTMAP_UV "+t.displacementMapUv:"",t.metalnessMapUv?"#define METALNESSMAP_UV "+t.metalnessMapUv:"",t.roughnessMapUv?"#define ROUGHNESSMAP_UV "+t.roughnessMapUv:"",t.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+t.anisotropyMapUv:"",t.clearcoatMapUv?"#define CLEARCOATMAP_UV "+t.clearcoatMapUv:"",t.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+t.clearcoatNormalMapUv:"",t.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+t.clearcoatRoughnessMapUv:"",t.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+t.iridescenceMapUv:"",t.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+t.iridescenceThicknessMapUv:"",t.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+t.sheenColorMapUv:"",t.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+t.sheenRoughnessMapUv:"",t.specularMapUv?"#define SPECULARMAP_UV "+t.specularMapUv:"",t.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+t.specularColorMapUv:"",t.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+t.specularIntensityMapUv:"",t.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+t.transmissionMapUv:"",t.thicknessMapUv?"#define THICKNESSMAP_UV "+t.thicknessMapUv:"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexNormals?"#define HAS_NORMAL":"",t.vertexColors?"#define USE_COLOR":"",t.vertexAlphas?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.flatShading?"#define FLAT_SHADED":"",t.skinning?"#define USE_SKINNING":"",t.morphTargets?"#define USE_MORPHTARGETS":"",t.morphNormals&&t.flatShading===!1?"#define USE_MORPHNORMALS":"",t.morphColors?"#define USE_MORPHCOLORS":"",t.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+t.morphTextureStride:"",t.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+t.morphTargetsCount:"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.sizeAttenuation?"#define USE_SIZEATTENUATION":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(So).join(`
`),f=[Fp(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,x,t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",t.map?"#define USE_MAP":"",t.matcap?"#define USE_MATCAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+c:"",t.envMap?"#define "+h:"",t.envMap?"#define "+d:"",u?"#define CUBEUV_TEXEL_WIDTH "+u.texelWidth:"",u?"#define CUBEUV_TEXEL_HEIGHT "+u.texelHeight:"",u?"#define CUBEUV_MAX_MIP "+u.maxMip+".0":"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.packedNormalMap?"#define USE_PACKED_NORMALMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoat?"#define USE_CLEARCOAT":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.dispersion?"#define USE_DISPERSION":"",t.retroreflection?"#define USE_RETROREFLECTION":"",t.iridescence?"#define USE_IRIDESCENCE":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaTest?"#define USE_ALPHATEST":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.sheen?"#define USE_SHEEN":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexColors||t.instancingColor?"#define USE_COLOR":"",t.vertexAlphas||t.batchingColor?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.gradientMap?"#define USE_GRADIENTMAP":"",t.flatShading?"#define FLAT_SHADED":"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.numLightProbeGrids>0?"#define USE_LIGHT_PROBES_GRID":"",t.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",t.decodeVideoTextureEmissive?"#define DECODE_VIDEO_TEXTURE_EMISSIVE":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",t.toneMapping!==Si?"#define TONE_MAPPING":"",t.toneMapping!==Si?Ft.tonemapping_pars_fragment:"",t.toneMapping!==Si?Vy("toneMapping",t.toneMapping):"",t.dithering?"#define DITHERING":"",t.opaque?"#define OPAQUE":"",Ft.colorspace_pars_fragment,Hy("linearToOutputTexel",t.outputColorSpace),Wy(),t.useDepthPacking?"#define DEPTH_PACKING "+t.depthPacking:"",`
`].filter(So).join(`
`)),a=Du(a),a=Ip(a,t),a=Dp(a,t),o=Du(o),o=Ip(o,t),o=Dp(o,t),a=Lp(a),o=Lp(o),t.isRawShaderMaterial!==!0&&(b=`#version 300 es
`,m=[p,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+m,f=["#define varying in",t.glslVersion===mu?"":"layout(location = 0) out highp vec4 pc_fragColor;",t.glslVersion===mu?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+f);let E=b+m+a,g=b+f+o,_=Rp(i,i.VERTEX_SHADER,E),w=Rp(i,i.FRAGMENT_SHADER,g);i.attachShader(M,_),i.attachShader(M,w),t.index0AttributeName!==void 0?i.bindAttribLocation(M,0,t.index0AttributeName):t.hasPositionAttribute===!0&&i.bindAttribLocation(M,0,"position"),i.linkProgram(M);function T(D){if(s.debug.checkShaderErrors){let v=i.getProgramInfoLog(M)||"",I=i.getShaderInfoLog(_)||"",P=i.getShaderInfoLog(w)||"",N=v.trim(),z=I.trim(),X=P.trim(),j=!0,V=!0;if(i.getProgramParameter(M,i.LINK_STATUS)===!1)if(j=!1,typeof s.debug.onShaderError=="function")s.debug.onShaderError(i,M,_,w);else{let W=Pp(i,_,"vertex"),ie=Pp(i,w,"fragment");_t("WebGLProgram: Shader Error "+i.getError()+" - VALIDATE_STATUS "+i.getProgramParameter(M,i.VALIDATE_STATUS)+`

Material Name: `+D.name+`
Material Type: `+D.type+`

Program Info Log: `+N+`
`+W+`
`+ie)}else N!==""?dt("WebGLProgram: Program Info Log:",N):(z===""||X==="")&&(V=!1);V&&(D.diagnostics={runnable:j,programLog:N,vertexShader:{log:z,prefix:m},fragmentShader:{log:X,prefix:f}})}i.deleteShader(_),i.deleteShader(w),y=new ra(i,M),C=jy(i,M)}let y;this.getUniforms=function(){return y===void 0&&T(this),y};let C;this.getAttributes=function(){return C===void 0&&T(this),C};let S=t.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return S===!1&&(S=i.getProgramParameter(M,zy)),S},this.destroy=function(){n.releaseStatesOfProgram(this),i.deleteProgram(M),this.program=void 0},this.type=t.shaderType,this.name=t.shaderName,this.id=Oy++,this.cacheKey=e,this.usedTimes=1,this.program=M,this.vertexShader=_,this.fragmentShader=w,this}var c_=0,Lu=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(e,t,n){let i=this._getShaderCacheForMaterial(e);return i.has(t)===!1&&(i.add(t),t.usedTimes++),i.has(n)===!1&&(i.add(n),n.usedTimes++),this}remove(e){let t=this.materialCache.get(e);for(let n of t)n.usedTimes--,n.usedTimes===0&&this.shaderCache.delete(n.code);return this.materialCache.delete(e),this}getVertexShaderStage(e){return this._getShaderStage(e.vertexShader)}getFragmentShaderStage(e){return this._getShaderStage(e.fragmentShader)}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(e){let t=this.materialCache,n=t.get(e);return n===void 0&&(n=new Set,t.set(e,n)),n}_getShaderStage(e){let t=this.shaderCache,n=t.get(e);return n===void 0&&(n=new Fu(e),t.set(e,n)),n}},Fu=class{constructor(e){this.id=c_++,this.code=e,this.usedTimes=0}};function h_(s){return s===Ps||s===bo||s===vo}function u_(s,e,t,n,i,r){let a=new Ur,o=new Lu,l=new Set,c=[],h=new Map,d=n.logarithmicDepthBuffer,u=n.precision,p={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distance",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function x(y){return l.add(y),y===0?"uv":`uv${y}`}function M(y,C,S,D,v,I){let P=D.fog,N=v.geometry,z=y.isMeshStandardMaterial||y.isMeshLambertMaterial||y.isMeshPhongMaterial?D.environment:null,X=y.isMeshStandardMaterial||y.isMeshLambertMaterial&&!y.envMap||y.isMeshPhongMaterial&&!y.envMap,j=e.get(y.envMap||z,X),V=j&&j.mapping===fo?j.image.height:null,W=p[y.type];y.precision!==null&&(u=n.getMaxPrecision(y.precision),u!==y.precision&&dt("WebGLProgram.getParameters:",y.precision,"not supported, using",u,"instead."));let ie=N.morphAttributes.position||N.morphAttributes.normal||N.morphAttributes.color,H=ie!==void 0?ie.length:0,ge=0;N.morphAttributes.position!==void 0&&(ge=1),N.morphAttributes.normal!==void 0&&(ge=2),N.morphAttributes.color!==void 0&&(ge=3);let Ze,G,ye,J;if(W){let Fe=Hi[W];Ze=Fe.vertexShader,G=Fe.fragmentShader}else{Ze=y.vertexShader,G=y.fragmentShader;let Fe=o.getVertexShaderStage(y),Je=o.getFragmentShaderStage(y);o.update(y,Fe,Je),ye=Fe.id,J=Je.id}let A=s.getRenderTarget(),me=s.state.buffers.depth.getReversed(),ce=v.isInstancedMesh===!0,ne=v.isBatchedMesh===!0,K=!!y.map,oe=!!y.matcap,re=!!j,te=!!y.aoMap,fe=!!y.lightMap,Ee=!!y.bumpMap&&y.wireframe===!1,Ne=!!y.normalMap,Oe=!!y.displacementMap,ut=!!y.emissiveMap,rt=!!y.metalnessMap,at=!!y.roughnessMap,Q=y.anisotropy>0,vt=y.clearcoat>0,je=y.dispersion>0,O=y.retroreflectivity>0,R=y.iridescence>0,ee=y.sheen>0,ae=y.transmission>0,_e=Q&&!!y.anisotropyMap,Ue=vt&&!!y.clearcoatMap,ke=vt&&!!y.clearcoatNormalMap,B=vt&&!!y.clearcoatRoughnessMap,Z=R&&!!y.iridescenceMap,pe=R&&!!y.iridescenceThicknessMap,Te=ee&&!!y.sheenColorMap,ve=ee&&!!y.sheenRoughnessMap,we=!!y.specularMap,ue=!!y.specularColorMap,Ae=!!y.specularIntensityMap,ze=ae&&!!y.transmissionMap,Y=ae&&!!y.thicknessMap,Ce=!!y.gradientMap,Re=!!y.alphaMap,Ve=y.alphaTest>0,He=!!y.alphaHash,De=!!y.extensions,Ye=Si;y.toneMapped&&(A===null||A.isXRRenderTarget===!0)&&(Ye=s.toneMapping);let et={shaderID:W,shaderType:y.type,shaderName:y.name,vertexShader:Ze,fragmentShader:G,defines:y.defines,customVertexShaderID:ye,customFragmentShaderID:J,isRawShaderMaterial:y.isRawShaderMaterial===!0,glslVersion:y.glslVersion,precision:u,batching:ne,batchingColor:ne&&v._colorsTexture!==null,instancing:ce,instancingColor:ce&&v.instanceColor!==null,instancingMorph:ce&&v.morphTexture!==null,outputColorSpace:A===null?s.outputColorSpace:A.isXRRenderTarget===!0?A.texture.colorSpace:Dt.workingColorSpace,alphaToCoverage:!!y.alphaToCoverage,map:K,matcap:oe,envMap:re,envMapMode:re&&j.mapping,envMapCubeUVHeight:V,aoMap:te,lightMap:fe,bumpMap:Ee,normalMap:Ne,displacementMap:Oe,emissiveMap:ut,normalMapObjectSpace:Ne&&y.normalMapType===ip,normalMapTangentSpace:Ne&&y.normalMapType===_o,packedNormalMap:Ne&&y.normalMapType===_o&&h_(y.normalMap.format),metalnessMap:rt,roughnessMap:at,anisotropy:Q,anisotropyMap:_e,clearcoat:vt,clearcoatMap:Ue,clearcoatNormalMap:ke,clearcoatRoughnessMap:B,dispersion:je,retroreflection:O,iridescence:R,iridescenceMap:Z,iridescenceThicknessMap:pe,sheen:ee,sheenColorMap:Te,sheenRoughnessMap:ve,specularMap:we,specularColorMap:ue,specularIntensityMap:Ae,transmission:ae,transmissionMap:ze,thicknessMap:Y,gradientMap:Ce,opaque:y.transparent===!1&&y.blending===$r&&y.alphaToCoverage===!1,alphaMap:Re,alphaTest:Ve,alphaHash:He,combine:y.combine,mapUv:K&&x(y.map.channel),aoMapUv:te&&x(y.aoMap.channel),lightMapUv:fe&&x(y.lightMap.channel),bumpMapUv:Ee&&x(y.bumpMap.channel),normalMapUv:Ne&&x(y.normalMap.channel),displacementMapUv:Oe&&x(y.displacementMap.channel),emissiveMapUv:ut&&x(y.emissiveMap.channel),metalnessMapUv:rt&&x(y.metalnessMap.channel),roughnessMapUv:at&&x(y.roughnessMap.channel),anisotropyMapUv:_e&&x(y.anisotropyMap.channel),clearcoatMapUv:Ue&&x(y.clearcoatMap.channel),clearcoatNormalMapUv:ke&&x(y.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:B&&x(y.clearcoatRoughnessMap.channel),iridescenceMapUv:Z&&x(y.iridescenceMap.channel),iridescenceThicknessMapUv:pe&&x(y.iridescenceThicknessMap.channel),sheenColorMapUv:Te&&x(y.sheenColorMap.channel),sheenRoughnessMapUv:ve&&x(y.sheenRoughnessMap.channel),specularMapUv:we&&x(y.specularMap.channel),specularColorMapUv:ue&&x(y.specularColorMap.channel),specularIntensityMapUv:Ae&&x(y.specularIntensityMap.channel),transmissionMapUv:ze&&x(y.transmissionMap.channel),thicknessMapUv:Y&&x(y.thicknessMap.channel),alphaMapUv:Re&&x(y.alphaMap.channel),vertexTangents:!!N.attributes.tangent&&(Ne||Q),vertexNormals:!!N.attributes.normal,vertexColors:y.vertexColors,vertexAlphas:y.vertexColors===!0&&!!N.attributes.color&&N.attributes.color.itemSize===4,pointsUvs:v.isPoints===!0&&!!N.attributes.uv&&(K||Re),fog:!!P,useFog:y.fog===!0,fogExp2:!!P&&P.isFogExp2,flatShading:y.wireframe===!1&&(y.flatShading===!0||N.attributes.normal===void 0&&Ne===!1&&(y.isMeshLambertMaterial||y.isMeshPhongMaterial||y.isMeshStandardMaterial||y.isMeshPhysicalMaterial)),sizeAttenuation:y.sizeAttenuation===!0,logarithmicDepthBuffer:d,reversedDepthBuffer:me,skinning:v.isSkinnedMesh===!0,hasPositionAttribute:N.attributes.position!==void 0,morphTargets:N.morphAttributes.position!==void 0,morphNormals:N.morphAttributes.normal!==void 0,morphColors:N.morphAttributes.color!==void 0,morphTargetsCount:H,morphTextureStride:ge,numSunLights:C.sun.length,numDirLights:C.directional.length,numPointLights:C.point.length,numSpotLights:C.spot.length,numSpotLightMaps:C.spotLightMap.length,numRectAreaLights:C.rectArea.length,numHemiLights:C.hemi.length,numSunLightShadows:C.sunShadowMap.length,numDirLightShadows:C.directionalShadowMap.length,numPointLightShadows:C.pointShadowMap.length,numSpotLightShadows:C.spotShadowMap.length,numSpotLightShadowsWithMaps:C.numSpotLightShadowsWithMaps,numLightProbes:C.numLightProbes,numLightProbeGrids:I.length,numClippingPlanes:r.numPlanes,numClipIntersection:r.numIntersection,dithering:y.dithering,shadowMapEnabled:s.shadowMap.enabled&&S.length>0,shadowMapType:s.shadowMap.type,toneMapping:Ye,decodeVideoTexture:K&&y.map.isVideoTexture===!0&&Dt.getTransfer(y.map.colorSpace)===Xt,decodeVideoTextureEmissive:ut&&y.emissiveMap.isVideoTexture===!0&&Dt.getTransfer(y.emissiveMap.colorSpace)===Xt,premultipliedAlpha:y.premultipliedAlpha,doubleSided:y.side===Kt,flipSided:y.side===yn,useDepthPacking:y.depthPacking>=0,depthPacking:y.depthPacking||0,index0AttributeName:y.index0AttributeName,extensionClipCullDistance:De&&y.extensions.clipCullDistance===!0&&t.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(De&&y.extensions.multiDraw===!0||ne)&&t.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:t.has("KHR_parallel_shader_compile"),customProgramCacheKey:y.customProgramCacheKey()};return et.vertexUv1s=l.has(1),et.vertexUv2s=l.has(2),et.vertexUv3s=l.has(3),l.clear(),et}function m(y){let C=[];if(y.shaderID?C.push(y.shaderID):(C.push(y.customVertexShaderID),C.push(y.customFragmentShaderID)),y.defines!==void 0)for(let S in y.defines)C.push(S),C.push(y.defines[S]);return y.isRawShaderMaterial===!1&&(f(C,y),b(C,y),C.push(s.outputColorSpace)),C.push(y.customProgramCacheKey),C.join()}function f(y,C){y.push(C.precision),y.push(C.outputColorSpace),y.push(C.envMapMode),y.push(C.envMapCubeUVHeight),y.push(C.mapUv),y.push(C.alphaMapUv),y.push(C.lightMapUv),y.push(C.aoMapUv),y.push(C.bumpMapUv),y.push(C.normalMapUv),y.push(C.displacementMapUv),y.push(C.emissiveMapUv),y.push(C.metalnessMapUv),y.push(C.roughnessMapUv),y.push(C.anisotropyMapUv),y.push(C.clearcoatMapUv),y.push(C.clearcoatNormalMapUv),y.push(C.clearcoatRoughnessMapUv),y.push(C.iridescenceMapUv),y.push(C.iridescenceThicknessMapUv),y.push(C.sheenColorMapUv),y.push(C.sheenRoughnessMapUv),y.push(C.specularMapUv),y.push(C.specularColorMapUv),y.push(C.specularIntensityMapUv),y.push(C.transmissionMapUv),y.push(C.thicknessMapUv),y.push(C.combine),y.push(C.fogExp2),y.push(C.sizeAttenuation),y.push(C.morphTargetsCount),y.push(C.morphAttributeCount),y.push(C.numSunLights),y.push(C.numDirLights),y.push(C.numPointLights),y.push(C.numSpotLights),y.push(C.numSpotLightMaps),y.push(C.numHemiLights),y.push(C.numRectAreaLights),y.push(C.numSunLightShadows),y.push(C.numDirLightShadows),y.push(C.numPointLightShadows),y.push(C.numSpotLightShadows),y.push(C.numSpotLightShadowsWithMaps),y.push(C.numLightProbes),y.push(C.shadowMapType),y.push(C.toneMapping),y.push(C.numClippingPlanes),y.push(C.numClipIntersection),y.push(C.depthPacking)}function b(y,C){a.disableAll(),C.instancing&&a.enable(0),C.instancingColor&&a.enable(1),C.instancingMorph&&a.enable(2),C.matcap&&a.enable(3),C.envMap&&a.enable(4),C.normalMapObjectSpace&&a.enable(5),C.normalMapTangentSpace&&a.enable(6),C.clearcoat&&a.enable(7),C.iridescence&&a.enable(8),C.alphaTest&&a.enable(9),C.vertexColors&&a.enable(10),C.vertexAlphas&&a.enable(11),C.vertexUv1s&&a.enable(12),C.vertexUv2s&&a.enable(13),C.vertexUv3s&&a.enable(14),C.vertexTangents&&a.enable(15),C.anisotropy&&a.enable(16),C.alphaHash&&a.enable(17),C.batching&&a.enable(18),C.dispersion&&a.enable(19),C.retroreflection&&a.enable(24),C.batchingColor&&a.enable(20),C.gradientMap&&a.enable(21),C.packedNormalMap&&a.enable(22),C.vertexNormals&&a.enable(23),y.push(a.mask),a.disableAll(),C.fog&&a.enable(0),C.useFog&&a.enable(1),C.flatShading&&a.enable(2),C.logarithmicDepthBuffer&&a.enable(3),C.reversedDepthBuffer&&a.enable(4),C.skinning&&a.enable(5),C.morphTargets&&a.enable(6),C.morphNormals&&a.enable(7),C.morphColors&&a.enable(8),C.premultipliedAlpha&&a.enable(9),C.shadowMapEnabled&&a.enable(10),C.doubleSided&&a.enable(11),C.flipSided&&a.enable(12),C.useDepthPacking&&a.enable(13),C.dithering&&a.enable(14),C.transmission&&a.enable(15),C.sheen&&a.enable(16),C.opaque&&a.enable(17),C.pointsUvs&&a.enable(18),C.decodeVideoTexture&&a.enable(19),C.decodeVideoTextureEmissive&&a.enable(20),C.alphaToCoverage&&a.enable(21),C.numLightProbeGrids>0&&a.enable(22),C.hasPositionAttribute&&a.enable(23),y.push(a.mask)}function E(y){let C=p[y.type],S;if(C){let D=Hi[C];S=Ai.clone(D.uniforms)}else S=y.uniforms;return S}function g(y,C){let S=h.get(C);return S!==void 0?++S.usedTimes:(S=new l_(s,C,y,i),c.push(S),h.set(C,S)),S}function _(y){if(--y.usedTimes===0){let C=c.indexOf(y);c[C]=c[c.length-1],c.pop(),h.delete(y.cacheKey),y.destroy()}}function w(y){o.remove(y)}function T(){o.dispose()}return{getParameters:M,getProgramCacheKey:m,getUniforms:E,acquireProgram:g,releaseProgram:_,releaseShaderCache:w,programs:c,dispose:T}}function d_(){let s=new WeakMap;function e(a){return s.has(a)}function t(a){let o=s.get(a);return o===void 0&&(o={},s.set(a,o)),o}function n(a){s.delete(a)}function i(a,o,l){s.get(a)[o]=l}function r(){s=new WeakMap}return{has:e,get:t,remove:n,update:i,dispose:r}}function f_(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.material.id!==e.material.id?s.material.id-e.material.id:s.materialVariant!==e.materialVariant?s.materialVariant-e.materialVariant:s.z!==e.z?s.z-e.z:s.id-e.id}function Np(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.z!==e.z?e.z-s.z:s.id-e.id}function Up(){let s=[],e=0,t=[],n=[],i=[];function r(){e=0,t.length=0,n.length=0,i.length=0}function a(u){let p=0;return u.isInstancedMesh&&(p+=2),u.isSkinnedMesh&&(p+=1),p}function o(u,p,x,M,m,f){let b=s[e];return b===void 0?(b={id:u.id,object:u,geometry:p,material:x,materialVariant:a(u),groupOrder:M,renderOrder:u.renderOrder,z:m,group:f},s[e]=b):(b.id=u.id,b.object=u,b.geometry=p,b.material=x,b.materialVariant=a(u),b.groupOrder=M,b.renderOrder=u.renderOrder,b.z=m,b.group=f),e++,b}function l(u,p,x,M,m,f,b){b.reversedDepth===!0&&(m=-m);let E=o(u,p,x,M,m,f);x.transmission>0?n.push(E):x.transparent===!0?i.push(E):t.push(E)}function c(u,p,x,M,m,f){let b=o(u,p,x,M,m,f);x.transmission>0?n.unshift(b):x.transparent===!0?i.unshift(b):t.unshift(b)}function h(u,p){t.length>1&&t.sort(u||f_),n.length>1&&n.sort(p||Np),i.length>1&&i.sort(p||Np)}function d(){for(let u=e,p=s.length;u<p;u++){let x=s[u];if(x.id===null)break;x.id=null,x.object=null,x.geometry=null,x.material=null,x.group=null}}return{opaque:t,transmissive:n,transparent:i,init:r,push:l,unshift:c,finish:d,sort:h}}function p_(){let s=new WeakMap;function e(n,i){let r=s.get(n),a;return r===void 0?(a=new Up,s.set(n,[a])):i>=r.length?(a=new Up,r.push(a)):a=r[i],a}function t(){s=new WeakMap}return{get:e,dispose:t}}function m_(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"SunLight":case"DirectionalLight":t={direction:new U,color:new Le};break;case"SpotLight":t={position:new U,direction:new U,color:new Le,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":t={position:new U,color:new Le,distance:0,decay:0};break;case"HemisphereLight":t={direction:new U,skyColor:new Le,groundColor:new Le};break;case"RectAreaLight":t={color:new Le,position:new U,halfWidth:new U,halfHeight:new U};break}return s[e.id]=t,t}}}function g_(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"SunLight":case"DirectionalLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Ge};break;case"SpotLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Ge};break;case"PointLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Ge,shadowCameraNear:1,shadowCameraFar:1e3};break}return s[e.id]=t,t}}}var x_=0;function b_(s,e){return(e.castShadow?2:0)-(s.castShadow?2:0)+(e.map?1:0)-(s.map?1:0)}function v_(s){let e=new m_,t=g_(),n={version:0,hash:{sunLength:-1,directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numSunShadows:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],sun:[],sunShadow:[],sunShadowMap:[],sunShadowMatrix:[],sunShadowCascade:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let c=0;c<9;c++)n.probe.push(new U);let i=new U,r=new yt,a=new yt;function o(c){let h=0,d=0,u=0;for(let v=0;v<9;v++)n.probe[v].set(0,0,0);let p=0,x=0,M=0,m=0,f=0,b=0,E=0,g=0,_=0,w=0,T=0,y=0,C=0,S=0;c.sort(b_);for(let v=0,I=c.length;v<I;v++){let P=c[v],N=P.color,z=P.intensity,X=P.distance,j=null;if(P.shadow&&P.shadow.map&&(P.shadow.map.texture.format===Ps?j=P.shadow.map.texture:j=P.shadow.map.depthTexture||P.shadow.map.texture),P.isAmbientLight)h+=N.r*z,d+=N.g*z,u+=N.b*z;else if(P.isLightProbe){for(let V=0;V<9;V++)n.probe[V].addScaledVector(P.sh.coefficients[V],z);S++}else if(P.isSunLight){let V=e.get(P);if(V.color.copy(P.color).multiplyScalar(P.intensity),P.castShadow){let W=P.shadow,ie=t.get(P);ie.shadowIntensity=W.intensity,ie.shadowBias=W.bias,ie.shadowNormalBias=W.normalBias,ie.shadowRadius=W.radius,ie.shadowMapSize.copy(W.mapSize).multiply(W.getFrameExtents()),n.sunShadow[x]=ie,n.sunShadowMap[x]=j;let H=W.getViewportCount();for(let ge=0;ge<H;ge++)n.sunShadowMatrix[M+ge]=W.getMatrix(ge),n.sunShadowCascade[M+ge]=W._cascadeData[ge];M+=H,x++}n.sun[p]=V,p++}else if(P.isDirectionalLight){let V=e.get(P);if(V.color.copy(P.color).multiplyScalar(P.intensity),P.castShadow){let W=P.shadow,ie=t.get(P);ie.shadowIntensity=W.intensity,ie.shadowBias=W.bias,ie.shadowNormalBias=W.normalBias,ie.shadowRadius=W.radius,ie.shadowMapSize=W.mapSize,n.directionalShadow[m]=ie,n.directionalShadowMap[m]=j,n.directionalShadowMatrix[m]=P.shadow.matrix,_++}n.directional[m]=V,m++}else if(P.isSpotLight){let V=e.get(P);V.position.setFromMatrixPosition(P.matrixWorld),V.color.copy(N).multiplyScalar(z),V.distance=X,V.coneCos=Math.cos(P.angle),V.penumbraCos=Math.cos(P.angle*(1-P.penumbra)),V.decay=P.decay,n.spot[b]=V;let W=P.shadow;if(P.map&&(n.spotLightMap[y]=P.map,y++,W.updateMatrices(P),P.castShadow&&C++),n.spotLightMatrix[b]=W.matrix,P.castShadow){let ie=t.get(P);ie.shadowIntensity=W.intensity,ie.shadowBias=W.bias,ie.shadowNormalBias=W.normalBias,ie.shadowRadius=W.radius,ie.shadowMapSize=W.mapSize,n.spotShadow[b]=ie,n.spotShadowMap[b]=j,T++}b++}else if(P.isRectAreaLight){let V=e.get(P);V.color.copy(N).multiplyScalar(z),V.halfWidth.set(P.width*.5,0,0),V.halfHeight.set(0,P.height*.5,0),n.rectArea[E]=V,E++}else if(P.isPointLight){let V=e.get(P);if(V.color.copy(P.color).multiplyScalar(P.intensity),V.distance=P.distance,V.decay=P.decay,P.castShadow){let W=P.shadow,ie=t.get(P);ie.shadowIntensity=W.intensity,ie.shadowBias=W.bias,ie.shadowNormalBias=W.normalBias,ie.shadowRadius=W.radius,ie.shadowMapSize=W.mapSize,ie.shadowCameraNear=W.camera.near,ie.shadowCameraFar=W.camera.far,n.pointShadow[f]=ie,n.pointShadowMap[f]=j,n.pointShadowMatrix[f]=P.shadow.matrix,w++}n.point[f]=V,f++}else if(P.isHemisphereLight){let V=e.get(P);V.skyColor.copy(P.color).multiplyScalar(z),V.groundColor.copy(P.groundColor).multiplyScalar(z),n.hemi[g]=V,g++}}E>0&&(s.has("OES_texture_float_linear")===!0?(n.rectAreaLTC1=Qe.LTC_FLOAT_1,n.rectAreaLTC2=Qe.LTC_FLOAT_2):(n.rectAreaLTC1=Qe.LTC_HALF_1,n.rectAreaLTC2=Qe.LTC_HALF_2)),n.ambient[0]=h,n.ambient[1]=d,n.ambient[2]=u;let D=n.hash;(D.sunLength!==p||D.directionalLength!==m||D.pointLength!==f||D.spotLength!==b||D.rectAreaLength!==E||D.hemiLength!==g||D.numSunShadows!==x||D.numDirectionalShadows!==_||D.numPointShadows!==w||D.numSpotShadows!==T||D.numSpotMaps!==y||D.numLightProbes!==S)&&(n.sun.length=p,n.directional.length=m,n.spot.length=b,n.rectArea.length=E,n.point.length=f,n.hemi.length=g,n.sunShadow.length=x,n.sunShadowMap.length=x,n.sunShadowMatrix.length=M,n.sunShadowCascade.length=M,n.directionalShadow.length=_,n.directionalShadowMap.length=_,n.directionalShadowMatrix.length=_,n.pointShadow.length=w,n.pointShadowMap.length=w,n.pointShadowMatrix.length=w,n.spotShadow.length=T,n.spotShadowMap.length=T,n.spotLightMatrix.length=T+y-C,n.spotLightMap.length=y,n.numSpotLightShadowsWithMaps=C,n.numLightProbes=S,D.sunLength=p,D.directionalLength=m,D.pointLength=f,D.spotLength=b,D.rectAreaLength=E,D.hemiLength=g,D.numSunShadows=x,D.numDirectionalShadows=_,D.numPointShadows=w,D.numSpotShadows=T,D.numSpotMaps=y,D.numLightProbes=S,n.version=x_++)}function l(c,h){let d=0,u=0,p=0,x=0,M=0,m=0,f=h.matrixWorldInverse;for(let b=0,E=c.length;b<E;b++){let g=c[b];if(g.isSunLight){let _=n.sun[d];_.direction.setFromMatrixPosition(g.matrixWorld),_.direction.transformDirection(f),d++}else if(g.isDirectionalLight){let _=n.directional[u];_.direction.setFromMatrixPosition(g.matrixWorld),i.setFromMatrixPosition(g.target.matrixWorld),_.direction.sub(i),_.direction.transformDirection(f),u++}else if(g.isSpotLight){let _=n.spot[x];_.position.setFromMatrixPosition(g.matrixWorld),_.position.applyMatrix4(f),_.direction.setFromMatrixPosition(g.matrixWorld),i.setFromMatrixPosition(g.target.matrixWorld),_.direction.sub(i),_.direction.transformDirection(f),x++}else if(g.isRectAreaLight){let _=n.rectArea[M];_.position.setFromMatrixPosition(g.matrixWorld),_.position.applyMatrix4(f),a.identity(),r.copy(g.matrixWorld),r.premultiply(f),a.extractRotation(r),_.halfWidth.set(g.width*.5,0,0),_.halfHeight.set(0,g.height*.5,0),_.halfWidth.applyMatrix4(a),_.halfHeight.applyMatrix4(a),M++}else if(g.isPointLight){let _=n.point[p];_.position.setFromMatrixPosition(g.matrixWorld),_.position.applyMatrix4(f),p++}else if(g.isHemisphereLight){let _=n.hemi[m];_.direction.setFromMatrixPosition(g.matrixWorld),_.direction.transformDirection(f),m++}}}return{setup:o,setupView:l,state:n}}function zp(s){let e=new v_(s),t=[],n=[],i=[];function r(u){d.camera=u,t.length=0,n.length=0,i.length=0}function a(u){t.push(u)}function o(u){n.push(u)}function l(u){i.push(u)}function c(){e.setup(t)}function h(u){e.setupView(t,u)}let d={lightsArray:t,shadowsArray:n,lightProbeGridArray:i,camera:null,lights:e,transmissionRenderTarget:{},textureUnits:0};return{init:r,state:d,setupLights:c,setupLightsView:h,pushLight:a,pushShadow:o,pushLightProbeGrid:l}}function y_(s){let e=new WeakMap;function t(i,r=0){let a=e.get(i),o;return a===void 0?(o=new zp(s),e.set(i,[o])):r>=a.length?(o=new zp(s),a.push(o)):o=a[r],o}function n(){e=new WeakMap}return{get:t,dispose:n}}var __=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,M_=`uniform sampler2D shadow_pass;
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
}`,w_=[new U(1,0,0),new U(-1,0,0),new U(0,1,0),new U(0,-1,0),new U(0,0,1),new U(0,0,-1)],S_=[new U(0,-1,0),new U(0,-1,0),new U(0,0,1),new U(0,0,-1),new U(0,-1,0),new U(0,-1,0)],Op=new yt,wo=new U,Au=new U;function E_(s,e,t){let n=new Hr,i=new Ge,r=new Ge,a=new Ot,o=new Nl,l=new Ul,c={},h=t.maxTextureSize,d={[oi]:yn,[yn]:oi,[Kt]:Kt},u=new Rt({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new Ge},radius:{value:4}},vertexShader:__,fragmentShader:M_}),p=u.clone();p.defines.HORIZONTAL_PASS=1;let x=new At;x.setAttribute("position",new Pt(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let M=new ht(x,u),m=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=tr;let f=this.type;this.render=function(w,T,y){if(m.enabled===!1||m.autoUpdate===!1&&m.needsUpdate===!1||w.length===0)return;this.type===If&&(dt("WebGLShadowMap: PCFSoftShadowMap has been removed. Using PCFShadowMap instead."),this.type=tr);let C=s.getRenderTarget(),S=s.getActiveCubeFace(),D=s.getActiveMipmapLevel(),v=s.state;v.setBlending(li),v.buffers.depth.getReversed()===!0?v.buffers.color.setClear(0,0,0,0):v.buffers.color.setClear(1,1,1,1),v.buffers.depth.setTest(!0),v.setScissorTest(!1);let I=f!==this.type;I&&T.traverse(function(P){P.material&&(Array.isArray(P.material)?P.material.forEach(N=>N.needsUpdate=!0):P.material.needsUpdate=!0)});for(let P=0,N=w.length;P<N;P++){let z=w[P],X=z.shadow;if(X===void 0){dt("WebGLShadowMap:",z,"has no shadow.");continue}if(X.autoUpdate===!1&&X.needsUpdate===!1)continue;i.copy(X.mapSize);let j=X.getFrameExtents();i.multiply(j),r.copy(X.mapSize),(i.x>h||i.y>h)&&(i.x>h&&(r.x=Math.floor(h/j.x),i.x=r.x*j.x,X.mapSize.x=r.x),i.y>h&&(r.y=Math.floor(h/j.y),i.y=r.y*j.y,X.mapSize.y=r.y));let V=s.state.buffers.depth.getReversed();if(X.camera._reversedDepth=V,X.map===null||I===!0){if(X.map!==null&&(X.map.depthTexture!==null&&(X.map.depthTexture.dispose(),X.map.depthTexture=null),X.map.dispose()),this.type===Zr){if(z.isPointLight){dt("WebGLShadowMap: VSM shadow maps are not supported for PointLights. Use PCF or BasicShadowMap instead.");continue}X.map=new ln(i.x,i.y,{format:Ps,type:xn,minFilter:on,magFilter:on,generateMipmaps:!1}),X.map.texture.name=z.name+".shadowMap",X.map.depthTexture=new Ms(i.x,i.y,ei),X.map.depthTexture.name=z.name+".shadowMapDepth",X.map.depthTexture.format=Li,X.map.depthTexture.compareFunction=null,X.map.depthTexture.minFilter=cn,X.map.depthTexture.magFilter=cn}else z.isPointLight?(X.map=new Bc(i.x),X.map.depthTexture=new Al(i.x,Ti)):(X.map=new ln(i.x,i.y),X.map.depthTexture=new Ms(i.x,i.y,Ti)),X.map.depthTexture.name=z.name+".shadowMap",X.map.depthTexture.format=Li,this.type===tr?(X.map.depthTexture.compareFunction=V?zc:Uc,X.map.depthTexture.minFilter=on,X.map.depthTexture.magFilter=on):(X.map.depthTexture.compareFunction=null,X.map.depthTexture.minFilter=cn,X.map.depthTexture.magFilter=cn);X.camera.updateProjectionMatrix()}X.map.isWebGLCubeRenderTarget!==!0&&(X.map.width!==i.x||X.map.height!==i.y)&&X.map.setSize(i.x,i.y);let W=X.map.isWebGLCubeRenderTarget?6:X.getViewportCount();z.isPointLight!==!0&&X.updateMatrices(z,y);for(let ie=0;ie<W;ie++){let H=X.getCamera(ie);if(z.isPointLight){let ge=X.camera,Ze=X.matrix,G=z.distance||ge.far;G!==ge.far&&(ge.far=G,ge.updateProjectionMatrix()),wo.setFromMatrixPosition(z.matrixWorld),ge.position.copy(wo),Au.copy(ge.position),Au.add(w_[ie]),ge.up.copy(S_[ie]),ge.lookAt(Au),ge.updateMatrixWorld(),Ze.makeTranslation(-wo.x,-wo.y,-wo.z),Op.multiplyMatrices(ge.projectionMatrix,ge.matrixWorldInverse),X._frustum.setFromProjectionMatrix(Op,ge.coordinateSystem,ge.reversedDepth)}if(X.map.isWebGLCubeRenderTarget)s.setRenderTarget(X.map,ie),s.clear();else{ie===0&&(s.setRenderTarget(X.map),s.clear());let ge=X.getViewport(ie);a.set(r.x*ge.x,r.y*ge.y,r.x*ge.z,r.y*ge.w),v.viewport(a)}n=X.getFrustum(ie),g(T,y,H,z,this.type)}X.isPointLightShadow!==!0&&this.type===Zr&&b(X,y),X.needsUpdate=!1}f=this.type,m.needsUpdate=!1,s.setRenderTarget(C,S,D)};function b(w,T){let y=e.update(M);u.defines.VSM_SAMPLES!==w.blurSamples&&(u.defines.VSM_SAMPLES=w.blurSamples,p.defines.VSM_SAMPLES=w.blurSamples,u.needsUpdate=!0,p.needsUpdate=!0),w.mapPass===null?w.mapPass=new ln(i.x,i.y,{format:Ps,type:xn}):(w.mapPass.width!==w.map.width||w.mapPass.height!==w.map.height)&&w.mapPass.setSize(w.map.width,w.map.height),u.uniforms.shadow_pass.value=w.map.depthTexture,u.uniforms.resolution.value.set(w.map.width,w.map.height),u.uniforms.radius.value=w.radius,s.setRenderTarget(w.mapPass),s.clear(),s.renderBufferDirect(T,null,y,u,M,null),p.uniforms.shadow_pass.value=w.mapPass.texture,p.uniforms.resolution.value.set(w.map.width,w.map.height),p.uniforms.radius.value=w.radius,s.setRenderTarget(w.map),s.clear(),s.renderBufferDirect(T,null,y,p,M,null)}function E(w,T,y,C){let S=null,D=y.isPointLight===!0?w.customDistanceMaterial:w.customDepthMaterial;if(D!==void 0)S=D;else if(S=y.isPointLight===!0?l:o,s.localClippingEnabled&&T.clipShadows===!0&&Array.isArray(T.clippingPlanes)&&T.clippingPlanes.length!==0||T.displacementMap&&T.displacementScale!==0||T.alphaMap&&T.alphaTest>0||T.map&&T.alphaTest>0||T.alphaToCoverage===!0){let v=S.uuid,I=T.uuid,P=c[v];P===void 0&&(P={},c[v]=P);let N=P[I];N===void 0&&(N=S.clone(),P[I]=N,T.addEventListener("dispose",_)),S=N}if(S.visible=T.visible,S.wireframe=T.wireframe,C===Zr?S.side=T.shadowSide!==null?T.shadowSide:T.side:S.side=T.shadowSide!==null?T.shadowSide:d[T.side],S.alphaMap=T.alphaMap,S.alphaTest=T.alphaToCoverage===!0?.5:T.alphaTest,S.map=T.map,S.clipShadows=T.clipShadows,S.clippingPlanes=T.clippingPlanes,S.clipIntersection=T.clipIntersection,S.displacementMap=T.displacementMap,S.displacementScale=T.displacementScale,S.displacementBias=T.displacementBias,S.wireframeLinewidth=T.wireframeLinewidth,S.linewidth=T.linewidth,y.isPointLight===!0&&S.isMeshDistanceMaterial===!0){let v=s.properties.get(S);v.light=y}return S}function g(w,T,y,C,S){if(w.visible===!1)return;if(w.layers.test(T.layers)&&(w.isMesh||w.isLine||w.isPoints)&&(w.castShadow||w.receiveShadow&&S===Zr)&&(!w.frustumCulled||w.intersectsFrustum(n))){w.modelViewMatrix.multiplyMatrices(y.matrixWorldInverse,w.matrixWorld);let I=e.update(w),P=w.material;if(Array.isArray(P)){let N=I.groups;for(let z=0,X=N.length;z<X;z++){let j=N[z],V=P[j.materialIndex];if(V&&V.visible){let W=E(w,V,C,S);w.onBeforeShadow(s,w,T,y,I,W,j),s.renderBufferDirect(y,null,I,W,w,j),w.onAfterShadow(s,w,T,y,I,W,j)}}}else if(P.visible){let N=E(w,P,C,S);w.onBeforeShadow(s,w,T,y,I,N,null),s.renderBufferDirect(y,null,I,N,w,null),w.onAfterShadow(s,w,T,y,I,N,null)}}let v=w.children;for(let I=0,P=v.length;I<P;I++)g(v[I],T,y,C,S)}function _(w){w.target.removeEventListener("dispose",_);for(let y in c){let C=c[y],S=w.target.uuid;S in C&&(C[S].dispose(),delete C[S])}}}function T_(s,e){function t(){let Y=!1,Ce=new Ot,Re=null,Ve=new Ot(0,0,0,0);return{setMask:function(He){Re!==He&&!Y&&(s.colorMask(He,He,He,He),Re=He)},setLocked:function(He){Y=He},setClear:function(He,De,Ye,et,Fe){Fe===!0&&(He*=et,De*=et,Ye*=et),Ce.set(He,De,Ye,et),Ve.equals(Ce)===!1&&(s.clearColor(He,De,Ye,et),Ve.copy(Ce))},reset:function(){Y=!1,Re=null,Ve.set(-1,0,0,0)}}}function n(){let Y=!1,Ce=!1,Re=null,Ve=null,He=null;return{setReversed:function(De){if(Ce!==De){let Ye=e.get("EXT_clip_control");De?Ye.clipControlEXT(Ye.LOWER_LEFT_EXT,Ye.ZERO_TO_ONE_EXT):Ye.clipControlEXT(Ye.LOWER_LEFT_EXT,Ye.NEGATIVE_ONE_TO_ONE_EXT),Ce=De;let et=He;He=null,this.setClear(et)}},getReversed:function(){return Ce},setTest:function(De){De?A(s.DEPTH_TEST):me(s.DEPTH_TEST)},setMask:function(De){Re!==De&&!Y&&(s.depthMask(De),Re=De)},setFunc:function(De){if(Ce&&(De=pp[De]),Ve!==De){switch(De){case ml:s.depthFunc(s.NEVER);break;case gl:s.depthFunc(s.ALWAYS);break;case xl:s.depthFunc(s.LESS);break;case Pr:s.depthFunc(s.LEQUAL);break;case bl:s.depthFunc(s.EQUAL);break;case vl:s.depthFunc(s.GEQUAL);break;case yl:s.depthFunc(s.GREATER);break;case _l:s.depthFunc(s.NOTEQUAL);break;default:s.depthFunc(s.LEQUAL)}Ve=De}},setLocked:function(De){Y=De},setClear:function(De){He!==De&&(He=De,Ce&&(De=1-De),s.clearDepth(De))},reset:function(){Y=!1,Re=null,Ve=null,He=null,Ce=!1}}}function i(){let Y=!1,Ce=null,Re=null,Ve=null,He=null,De=null,Ye=null,et=null,Fe=null;return{setTest:function(Je){Y||(Je?A(s.STENCIL_TEST):me(s.STENCIL_TEST))},setMask:function(Je){Ce!==Je&&!Y&&(s.stencilMask(Je),Ce=Je)},setFunc:function(Je,F,q){(Re!==Je||Ve!==F||He!==q)&&(s.stencilFunc(Je,F,q),Re=Je,Ve=F,He=q)},setOp:function(Je,F,q){(De!==Je||Ye!==F||et!==q)&&(s.stencilOp(Je,F,q),De=Je,Ye=F,et=q)},setLocked:function(Je){Y=Je},setClear:function(Je){Fe!==Je&&(s.clearStencil(Je),Fe=Je)},reset:function(){Y=!1,Ce=null,Re=null,Ve=null,He=null,De=null,Ye=null,et=null,Fe=null}}}let r=new t,a=new n,o=new i,l=new WeakMap,c=new WeakMap,h={},d={},u={},p=new WeakMap,x=[],M=null,m=!1,f=null,b=null,E=null,g=null,_=null,w=null,T=null,y=new Le(0,0,0),C=0,S=!1,D=null,v=null,I=null,P=null,N=null,z=s.getParameter(s.MAX_COMBINED_TEXTURE_IMAGE_UNITS),X=!1,j=0,V=s.getParameter(s.VERSION);V.indexOf("WebGL")!==-1?(j=parseFloat(/^WebGL (\d)/.exec(V)[1]),X=j>=1):V.indexOf("OpenGL ES")!==-1&&(j=parseFloat(/^OpenGL ES (\d)/.exec(V)[1]),X=j>=2);let W=null,ie={},H=s.getParameter(s.SCISSOR_BOX),ge=s.getParameter(s.VIEWPORT),Ze=new Ot().fromArray(H),G=new Ot().fromArray(ge);function ye(Y,Ce,Re,Ve){let He=new Uint8Array(4),De=s.createTexture();s.bindTexture(Y,De),s.texParameteri(Y,s.TEXTURE_MIN_FILTER,s.NEAREST),s.texParameteri(Y,s.TEXTURE_MAG_FILTER,s.NEAREST);for(let Ye=0;Ye<Re;Ye++)Y===s.TEXTURE_3D||Y===s.TEXTURE_2D_ARRAY?s.texImage3D(Ce,0,s.RGBA,1,1,Ve,0,s.RGBA,s.UNSIGNED_BYTE,He):s.texImage2D(Ce+Ye,0,s.RGBA,1,1,0,s.RGBA,s.UNSIGNED_BYTE,He);return De}let J={};J[s.TEXTURE_2D]=ye(s.TEXTURE_2D,s.TEXTURE_2D,1),J[s.TEXTURE_CUBE_MAP]=ye(s.TEXTURE_CUBE_MAP,s.TEXTURE_CUBE_MAP_POSITIVE_X,6),J[s.TEXTURE_2D_ARRAY]=ye(s.TEXTURE_2D_ARRAY,s.TEXTURE_2D_ARRAY,1,1),J[s.TEXTURE_3D]=ye(s.TEXTURE_3D,s.TEXTURE_3D,1,1),r.setClear(0,0,0,1),a.setClear(1),o.setClear(0),A(s.DEPTH_TEST),a.setFunc(Pr),Ee(!1),Ne(tu),A(s.CULL_FACE),te(li);function A(Y){h[Y]!==!0&&(s.enable(Y),h[Y]=!0)}function me(Y){h[Y]!==!1&&(s.disable(Y),h[Y]=!1)}function ce(Y,Ce){return u[Y]!==Ce?(s.bindFramebuffer(Y,Ce),u[Y]=Ce,Y===s.DRAW_FRAMEBUFFER&&(u[s.FRAMEBUFFER]=Ce),Y===s.FRAMEBUFFER&&(u[s.DRAW_FRAMEBUFFER]=Ce),!0):!1}function ne(Y,Ce){let Re=x,Ve=!1;if(Y){Re=p.get(Ce),Re===void 0&&(Re=[],p.set(Ce,Re));let He=Y.textures;if(Re.length!==He.length||Re[0]!==s.COLOR_ATTACHMENT0){for(let De=0,Ye=He.length;De<Ye;De++)Re[De]=s.COLOR_ATTACHMENT0+De;Re.length=He.length,Ve=!0}}else Re[0]!==s.BACK&&(Re[0]=s.BACK,Ve=!0);Ve&&s.drawBuffers(Re)}function K(Y){return M!==Y?(s.useProgram(Y),M=Y,!0):!1}let oe={[nr]:s.FUNC_ADD,[Lf]:s.FUNC_SUBTRACT,[Ff]:s.FUNC_REVERSE_SUBTRACT};oe[Nf]=s.MIN,oe[Uf]=s.MAX;let re={[zf]:s.ZERO,[Of]:s.ONE,[kf]:s.SRC_COLOR,[su]:s.SRC_ALPHA,[qf]:s.SRC_ALPHA_SATURATE,[Vf]:s.DST_COLOR,[Hf]:s.DST_ALPHA,[Bf]:s.ONE_MINUS_SRC_COLOR,[ru]:s.ONE_MINUS_SRC_ALPHA,[Wf]:s.ONE_MINUS_DST_COLOR,[Gf]:s.ONE_MINUS_DST_ALPHA,[Xf]:s.CONSTANT_COLOR,[jf]:s.ONE_MINUS_CONSTANT_COLOR,[Kf]:s.CONSTANT_ALPHA,[Yf]:s.ONE_MINUS_CONSTANT_ALPHA};function te(Y,Ce,Re,Ve,He,De,Ye,et,Fe,Je){if(Y===li){m===!0&&(me(s.BLEND),m=!1);return}if(m===!1&&(A(s.BLEND),m=!0),Y!==Df){if(Y!==f||Je!==S){if((b!==nr||_!==nr)&&(s.blendEquation(s.FUNC_ADD),b=nr,_=nr),Je)switch(Y){case $r:s.blendFuncSeparate(s.ONE,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case rn:s.blendFunc(s.ONE,s.ONE);break;case nu:s.blendFuncSeparate(s.ZERO,s.ONE_MINUS_SRC_COLOR,s.ZERO,s.ONE);break;case iu:s.blendFuncSeparate(s.DST_COLOR,s.ONE_MINUS_SRC_ALPHA,s.ZERO,s.ONE);break;default:_t("WebGLState: Invalid blending: ",Y);break}else switch(Y){case $r:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case rn:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE,s.ONE,s.ONE);break;case nu:_t("WebGLState: SubtractiveBlending requires material.premultipliedAlpha = true");break;case iu:_t("WebGLState: MultiplyBlending requires material.premultipliedAlpha = true");break;default:_t("WebGLState: Invalid blending: ",Y);break}E=null,g=null,w=null,T=null,y.set(0,0,0),C=0,f=Y,S=Je}return}He=He||Ce,De=De||Re,Ye=Ye||Ve,(Ce!==b||He!==_)&&(s.blendEquationSeparate(oe[Ce],oe[He]),b=Ce,_=He),(Re!==E||Ve!==g||De!==w||Ye!==T)&&(s.blendFuncSeparate(re[Re],re[Ve],re[De],re[Ye]),E=Re,g=Ve,w=De,T=Ye),(et.equals(y)===!1||Fe!==C)&&(s.blendColor(et.r,et.g,et.b,Fe),y.copy(et),C=Fe),f=Y,S=!1}function fe(Y,Ce){Y.side===Kt?me(s.CULL_FACE):A(s.CULL_FACE);let Re=Y.side===yn;Ce&&(Re=!Re),Ee(Re),Y.blending===$r&&Y.transparent===!1?te(li):te(Y.blending,Y.blendEquation,Y.blendSrc,Y.blendDst,Y.blendEquationAlpha,Y.blendSrcAlpha,Y.blendDstAlpha,Y.blendColor,Y.blendAlpha,Y.premultipliedAlpha),a.setFunc(Y.depthFunc),a.setTest(Y.depthTest),a.setMask(Y.depthWrite),r.setMask(Y.colorWrite);let Ve=Y.stencilWrite;o.setTest(Ve),Ve&&(o.setMask(Y.stencilWriteMask),o.setFunc(Y.stencilFunc,Y.stencilRef,Y.stencilFuncMask),o.setOp(Y.stencilFail,Y.stencilZFail,Y.stencilZPass)),ut(Y.polygonOffset,Y.polygonOffsetFactor,Y.polygonOffsetUnits),Y.alphaToCoverage===!0?A(s.SAMPLE_ALPHA_TO_COVERAGE):me(s.SAMPLE_ALPHA_TO_COVERAGE)}function Ee(Y){D!==Y&&(Y?s.frontFace(s.CW):s.frontFace(s.CCW),D=Y)}function Ne(Y){Y!==Cf?(A(s.CULL_FACE),Y!==v&&(Y===tu?s.cullFace(s.BACK):Y===Pf?s.cullFace(s.FRONT):s.cullFace(s.FRONT_AND_BACK))):me(s.CULL_FACE),v=Y}function Oe(Y){Y!==I&&(X&&s.lineWidth(Y),I=Y)}function ut(Y,Ce,Re){Y?(A(s.POLYGON_OFFSET_FILL),(P!==Ce||N!==Re)&&(P=Ce,N=Re,a.getReversed()&&(Ce=-Ce),s.polygonOffset(Ce,Re))):me(s.POLYGON_OFFSET_FILL)}function rt(Y){Y?A(s.SCISSOR_TEST):me(s.SCISSOR_TEST)}function at(Y){Y===void 0&&(Y=s.TEXTURE0+z-1),W!==Y&&(s.activeTexture(Y),W=Y)}function Q(Y,Ce,Re){Re===void 0&&(W===null?Re=s.TEXTURE0+z-1:Re=W);let Ve=ie[Re];Ve===void 0&&(Ve={type:void 0,texture:void 0},ie[Re]=Ve),(Ve.type!==Y||Ve.texture!==Ce)&&(W!==Re&&(s.activeTexture(Re),W=Re),s.bindTexture(Y,Ce||J[Y]),Ve.type=Y,Ve.texture=Ce)}function vt(){let Y=ie[W];Y!==void 0&&Y.type!==void 0&&(s.bindTexture(Y.type,null),Y.type=void 0,Y.texture=void 0)}function je(){try{s.compressedTexImage2D(...arguments)}catch(Y){_t("WebGLState:",Y)}}function O(){try{s.compressedTexImage3D(...arguments)}catch(Y){_t("WebGLState:",Y)}}function R(){try{s.texSubImage2D(...arguments)}catch(Y){_t("WebGLState:",Y)}}function ee(){try{s.texSubImage3D(...arguments)}catch(Y){_t("WebGLState:",Y)}}function ae(){try{s.compressedTexSubImage2D(...arguments)}catch(Y){_t("WebGLState:",Y)}}function _e(){try{s.compressedTexSubImage3D(...arguments)}catch(Y){_t("WebGLState:",Y)}}function Ue(){try{s.texStorage2D(...arguments)}catch(Y){_t("WebGLState:",Y)}}function ke(){try{s.texStorage3D(...arguments)}catch(Y){_t("WebGLState:",Y)}}function B(){try{s.texImage2D(...arguments)}catch(Y){_t("WebGLState:",Y)}}function Z(){try{s.texImage3D(...arguments)}catch(Y){_t("WebGLState:",Y)}}function pe(Y){return d[Y]!==void 0?d[Y]:s.getParameter(Y)}function Te(Y,Ce){d[Y]!==Ce&&(s.pixelStorei(Y,Ce),d[Y]=Ce)}function ve(Y){Ze.equals(Y)===!1&&(s.scissor(Y.x,Y.y,Y.z,Y.w),Ze.copy(Y))}function we(Y){G.equals(Y)===!1&&(s.viewport(Y.x,Y.y,Y.z,Y.w),G.copy(Y))}function ue(Y,Ce){let Re=c.get(Ce);Re===void 0&&(Re=new WeakMap,c.set(Ce,Re));let Ve=Re.get(Y);Ve===void 0&&(Ve=s.getUniformBlockIndex(Ce,Y.name),Re.set(Y,Ve))}function Ae(Y,Ce){let Ve=c.get(Ce).get(Y);l.get(Ce)!==Ve&&(s.uniformBlockBinding(Ce,Ve,Y.__bindingPointIndex),l.set(Ce,Ve))}function ze(){s.disable(s.BLEND),s.disable(s.CULL_FACE),s.disable(s.DEPTH_TEST),s.disable(s.POLYGON_OFFSET_FILL),s.disable(s.SCISSOR_TEST),s.disable(s.STENCIL_TEST),s.disable(s.SAMPLE_ALPHA_TO_COVERAGE),s.blendEquation(s.FUNC_ADD),s.blendFunc(s.ONE,s.ZERO),s.blendFuncSeparate(s.ONE,s.ZERO,s.ONE,s.ZERO),s.blendColor(0,0,0,0),s.colorMask(!0,!0,!0,!0),s.clearColor(0,0,0,0),s.depthMask(!0),s.depthFunc(s.LESS),a.setReversed(!1),s.clearDepth(1),s.stencilMask(4294967295),s.stencilFunc(s.ALWAYS,0,4294967295),s.stencilOp(s.KEEP,s.KEEP,s.KEEP),s.clearStencil(0),s.cullFace(s.BACK),s.frontFace(s.CCW),s.polygonOffset(0,0),s.activeTexture(s.TEXTURE0),s.bindFramebuffer(s.FRAMEBUFFER,null),s.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),s.bindFramebuffer(s.READ_FRAMEBUFFER,null),s.useProgram(null),s.lineWidth(1),s.scissor(0,0,s.canvas.width,s.canvas.height),s.viewport(0,0,s.canvas.width,s.canvas.height),s.pixelStorei(s.PACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,!1),s.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!1),s.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,s.BROWSER_DEFAULT_WEBGL),s.pixelStorei(s.PACK_ROW_LENGTH,0),s.pixelStorei(s.PACK_SKIP_PIXELS,0),s.pixelStorei(s.PACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_ROW_LENGTH,0),s.pixelStorei(s.UNPACK_IMAGE_HEIGHT,0),s.pixelStorei(s.UNPACK_SKIP_PIXELS,0),s.pixelStorei(s.UNPACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_SKIP_IMAGES,0),h={},d={},W=null,ie={},u={},p=new WeakMap,x=[],M=null,m=!1,f=null,b=null,E=null,g=null,_=null,w=null,T=null,y=new Le(0,0,0),C=0,S=!1,D=null,v=null,I=null,P=null,N=null,Ze.set(0,0,s.canvas.width,s.canvas.height),G.set(0,0,s.canvas.width,s.canvas.height),r.reset(),a.reset(),o.reset()}return{buffers:{color:r,depth:a,stencil:o},enable:A,disable:me,bindFramebuffer:ce,drawBuffers:ne,useProgram:K,setBlending:te,setMaterial:fe,setFlipSided:Ee,setCullFace:Ne,setLineWidth:Oe,setPolygonOffset:ut,setScissorTest:rt,activeTexture:at,bindTexture:Q,unbindTexture:vt,compressedTexImage2D:je,compressedTexImage3D:O,texImage2D:B,texImage3D:Z,pixelStorei:Te,getParameter:pe,updateUBOMapping:ue,uniformBlockBinding:Ae,texStorage2D:Ue,texStorage3D:ke,texSubImage2D:R,texSubImage3D:ee,compressedTexSubImage2D:ae,compressedTexSubImage3D:_e,scissor:ve,viewport:we,reset:ze}}function A_(s,e,t,n,i,r,a){let o=e.has("WEBGL_multisampled_render_to_texture")?e.get("WEBGL_multisampled_render_to_texture"):null,l=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),c=new Ge,h=new WeakMap,d=new Set,u,p=new WeakMap,x=!1;try{x=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function M(O,R){return x?new OffscreenCanvas(O,R):Lr("canvas")}function m(O,R,ee){let ae=1,_e=je(O);if((_e.width>ee||_e.height>ee)&&(ae=ee/Math.max(_e.width,_e.height)),ae<1)if(typeof HTMLImageElement<"u"&&O instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&O instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&O instanceof ImageBitmap||typeof VideoFrame<"u"&&O instanceof VideoFrame){let Ue=Math.floor(ae*_e.width),ke=Math.floor(ae*_e.height);u===void 0&&(u=M(Ue,ke));let B=R?M(Ue,ke):u;return B.width=Ue,B.height=ke,B.getContext("2d").drawImage(O,0,0,Ue,ke),dt("WebGLRenderer: Texture has been resized from ("+_e.width+"x"+_e.height+") to ("+Ue+"x"+ke+")."),B}else return"data"in O&&dt("WebGLRenderer: Image in DataTexture is too big ("+_e.width+"x"+_e.height+")."),O;return O}function f(O){return O.generateMipmaps}function b(O){s.generateMipmap(O)}function E(O){return O.isWebGLCubeRenderTarget?s.TEXTURE_CUBE_MAP:O.isWebGL3DRenderTarget?s.TEXTURE_3D:O.isWebGLArrayRenderTarget||O.isCompressedArrayTexture?s.TEXTURE_2D_ARRAY:s.TEXTURE_2D}function g(O,R,ee,ae,_e,Ue=!1){if(O!==null){if(s[O]!==void 0)return s[O];dt("WebGLRenderer: Attempt to use non-existing WebGL internal format '"+O+"'")}let ke;ae&&(ke=e.get("EXT_texture_norm16"),ke||dt("WebGLRenderer: Unable to use normalized textures without EXT_texture_norm16 extension"));let B=R;if(R===s.RED&&(ee===s.FLOAT&&(B=s.R32F),ee===s.HALF_FLOAT&&(B=s.R16F),ee===s.UNSIGNED_BYTE&&(B=s.R8),ee===s.UNSIGNED_SHORT&&ke&&(B=ke.R16_EXT),ee===s.SHORT&&ke&&(B=ke.R16_SNORM_EXT)),R===s.RED_INTEGER&&(ee===s.UNSIGNED_BYTE&&(B=s.R8UI),ee===s.UNSIGNED_SHORT&&(B=s.R16UI),ee===s.UNSIGNED_INT&&(B=s.R32UI),ee===s.BYTE&&(B=s.R8I),ee===s.SHORT&&(B=s.R16I),ee===s.INT&&(B=s.R32I)),R===s.RG&&(ee===s.FLOAT&&(B=s.RG32F),ee===s.HALF_FLOAT&&(B=s.RG16F),ee===s.UNSIGNED_BYTE&&(B=s.RG8),ee===s.UNSIGNED_SHORT&&ke&&(B=ke.RG16_EXT),ee===s.SHORT&&ke&&(B=ke.RG16_SNORM_EXT)),R===s.RG_INTEGER&&(ee===s.UNSIGNED_BYTE&&(B=s.RG8UI),ee===s.UNSIGNED_SHORT&&(B=s.RG16UI),ee===s.UNSIGNED_INT&&(B=s.RG32UI),ee===s.BYTE&&(B=s.RG8I),ee===s.SHORT&&(B=s.RG16I),ee===s.INT&&(B=s.RG32I)),R===s.RGB_INTEGER&&(ee===s.UNSIGNED_BYTE&&(B=s.RGB8UI),ee===s.UNSIGNED_SHORT&&(B=s.RGB16UI),ee===s.UNSIGNED_INT&&(B=s.RGB32UI),ee===s.BYTE&&(B=s.RGB8I),ee===s.SHORT&&(B=s.RGB16I),ee===s.INT&&(B=s.RGB32I)),R===s.RGBA_INTEGER&&(ee===s.UNSIGNED_BYTE&&(B=s.RGBA8UI),ee===s.UNSIGNED_SHORT&&(B=s.RGBA16UI),ee===s.UNSIGNED_INT&&(B=s.RGBA32UI),ee===s.BYTE&&(B=s.RGBA8I),ee===s.SHORT&&(B=s.RGBA16I),ee===s.INT&&(B=s.RGBA32I)),R===s.RGB&&(ee===s.UNSIGNED_SHORT&&ke&&(B=ke.RGB16_EXT),ee===s.SHORT&&ke&&(B=ke.RGB16_SNORM_EXT),ee===s.UNSIGNED_INT_5_9_9_9_REV&&(B=s.RGB9_E5),ee===s.UNSIGNED_INT_10F_11F_11F_REV&&(B=s.R11F_G11F_B10F)),R===s.RGBA){let Z=Ue?La:Dt.getTransfer(_e);ee===s.FLOAT&&(B=s.RGBA32F),ee===s.HALF_FLOAT&&(B=s.RGBA16F),ee===s.UNSIGNED_BYTE&&(B=Z===Xt?s.SRGB8_ALPHA8:s.RGBA8),ee===s.UNSIGNED_SHORT&&ke&&(B=ke.RGBA16_EXT),ee===s.SHORT&&ke&&(B=ke.RGBA16_SNORM_EXT),ee===s.UNSIGNED_SHORT_4_4_4_4&&(B=s.RGBA4),ee===s.UNSIGNED_SHORT_5_5_5_1&&(B=s.RGB5_A1)}return(B===s.R16F||B===s.R32F||B===s.RG16F||B===s.RG32F||B===s.RGBA16F||B===s.RGBA32F)&&e.get("EXT_color_buffer_float"),B}function _(O,R){let ee;return O?R===null||R===Ti||R===ta?ee=s.DEPTH24_STENCIL8:R===ei?ee=s.DEPTH32F_STENCIL8:R===ea&&(ee=s.DEPTH24_STENCIL8,dt("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):R===null||R===Ti||R===ta?ee=s.DEPTH_COMPONENT24:R===ei?ee=s.DEPTH_COMPONENT32F:R===ea&&(ee=s.DEPTH_COMPONENT16),ee}function w(O,R){return f(O)===!0||O.isFramebufferTexture&&O.minFilter!==cn&&O.minFilter!==on?Math.log2(Math.max(R.width,R.height))+1:O.mipmaps!==void 0&&O.mipmaps.length>0?O.mipmaps.length:O.isCompressedTexture&&Array.isArray(O.image)?R.mipmaps.length:1}function T(O){let R=O.target;R.removeEventListener("dispose",T),C(R),R.isVideoTexture&&h.delete(R),R.isHTMLTexture&&d.delete(R)}function y(O){let R=O.target;R.removeEventListener("dispose",y),D(R)}function C(O){let R=n.get(O);if(R.__webglInit===void 0)return;let ee=O.source,ae=p.get(ee);if(ae){let _e=ae[R.__cacheKey];_e.usedTimes--,_e.usedTimes===0&&S(O),Object.keys(ae).length===0&&p.delete(ee)}n.remove(O)}function S(O){let R=n.get(O);s.deleteTexture(R.__webglTexture);let ee=O.source,ae=p.get(ee);delete ae[R.__cacheKey],a.memory.textures--}function D(O){let R=n.get(O);if(O.depthTexture&&(O.depthTexture.dispose(),n.remove(O.depthTexture)),O.isWebGLCubeRenderTarget)for(let ae=0;ae<6;ae++){if(Array.isArray(R.__webglFramebuffer[ae]))for(let _e=0;_e<R.__webglFramebuffer[ae].length;_e++)s.deleteFramebuffer(R.__webglFramebuffer[ae][_e]);else s.deleteFramebuffer(R.__webglFramebuffer[ae]);R.__webglDepthbuffer&&s.deleteRenderbuffer(R.__webglDepthbuffer[ae])}else{if(Array.isArray(R.__webglFramebuffer))for(let ae=0;ae<R.__webglFramebuffer.length;ae++)s.deleteFramebuffer(R.__webglFramebuffer[ae]);else s.deleteFramebuffer(R.__webglFramebuffer);if(R.__webglDepthbuffer&&s.deleteRenderbuffer(R.__webglDepthbuffer),R.__webglMultisampledFramebuffer&&s.deleteFramebuffer(R.__webglMultisampledFramebuffer),R.__webglColorRenderbuffer)for(let ae=0;ae<R.__webglColorRenderbuffer.length;ae++)R.__webglColorRenderbuffer[ae]&&s.deleteRenderbuffer(R.__webglColorRenderbuffer[ae]);R.__webglDepthRenderbuffer&&s.deleteRenderbuffer(R.__webglDepthRenderbuffer)}let ee=O.textures;for(let ae=0,_e=ee.length;ae<_e;ae++){let Ue=n.get(ee[ae]);Ue.__webglTexture&&(s.deleteTexture(Ue.__webglTexture),a.memory.textures--),n.remove(ee[ae])}n.remove(O)}let v=0;function I(){v=0}function P(){return v}function N(O){v=O}function z(){let O=v;return O>=i.maxTextures&&dt("WebGLTextures: Trying to use "+(O+1)+" texture units while this GPU supports only "+i.maxTextures),v+=1,O}function X(O){let R=[];return R.push(O.wrapS),R.push(O.wrapT),R.push(O.wrapR||0),R.push(O.magFilter),R.push(O.minFilter),R.push(O.anisotropy),R.push(O.internalFormat),R.push(O.format),R.push(O.type),R.push(O.generateMipmaps),R.push(O.premultiplyAlpha),R.push(O.flipY),R.push(O.unpackAlignment),R.push(O.colorSpace),R.join()}function j(O,R){let ee=n.get(O);if(O.isVideoTexture&&Q(O),O.isRenderTargetTexture===!1&&O.isExternalTexture!==!0&&O.version>0&&ee.__version!==O.version){let ae=O.image;if(ae===null)dt("WebGLRenderer: Texture marked for update but no image data found.");else if(ae.complete===!1)dt("WebGLRenderer: Texture marked for update but image is incomplete");else{me(ee,O,R);return}}else O.isExternalTexture&&(ee.__webglTexture=O.sourceTexture?O.sourceTexture:null);t.bindTexture(s.TEXTURE_2D,ee.__webglTexture,s.TEXTURE0+R)}function V(O,R){let ee=n.get(O);if(O.isRenderTargetTexture===!1&&O.version>0&&ee.__version!==O.version){me(ee,O,R);return}else O.isExternalTexture&&(ee.__webglTexture=O.sourceTexture?O.sourceTexture:null);t.bindTexture(s.TEXTURE_2D_ARRAY,ee.__webglTexture,s.TEXTURE0+R)}function W(O,R){let ee=n.get(O);if(O.isRenderTargetTexture===!1&&O.version>0&&ee.__version!==O.version){me(ee,O,R);return}t.bindTexture(s.TEXTURE_3D,ee.__webglTexture,s.TEXTURE0+R)}function ie(O,R){let ee=n.get(O);if(O.isCubeDepthTexture!==!0&&O.version>0&&ee.__version!==O.version){ce(ee,O,R);return}t.bindTexture(s.TEXTURE_CUBE_MAP,ee.__webglTexture,s.TEXTURE0+R)}let H={[Di]:s.REPEAT,[ri]:s.CLAMP_TO_EDGE,[Ir]:s.MIRRORED_REPEAT},ge={[cn]:s.NEAREST,[Jl]:s.NEAREST_MIPMAP_NEAREST,[rr]:s.NEAREST_MIPMAP_LINEAR,[on]:s.LINEAR,[Qr]:s.LINEAR_MIPMAP_NEAREST,[Ei]:s.LINEAR_MIPMAP_LINEAR},Ze={[rp]:s.NEVER,[hp]:s.ALWAYS,[ap]:s.LESS,[Uc]:s.LEQUAL,[op]:s.EQUAL,[zc]:s.GEQUAL,[lp]:s.GREATER,[cp]:s.NOTEQUAL};function G(O,R){if(R.type===ei&&e.has("OES_texture_float_linear")===!1&&(R.magFilter===on||R.magFilter===Qr||R.magFilter===rr||R.magFilter===Ei||R.minFilter===on||R.minFilter===Qr||R.minFilter===rr||R.minFilter===Ei)&&dt("WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),s.texParameteri(O,s.TEXTURE_WRAP_S,H[R.wrapS]),s.texParameteri(O,s.TEXTURE_WRAP_T,H[R.wrapT]),(O===s.TEXTURE_3D||O===s.TEXTURE_2D_ARRAY)&&s.texParameteri(O,s.TEXTURE_WRAP_R,H[R.wrapR]),s.texParameteri(O,s.TEXTURE_MAG_FILTER,ge[R.magFilter]),s.texParameteri(O,s.TEXTURE_MIN_FILTER,ge[R.minFilter]),R.compareFunction&&(s.texParameteri(O,s.TEXTURE_COMPARE_MODE,s.COMPARE_REF_TO_TEXTURE),s.texParameteri(O,s.TEXTURE_COMPARE_FUNC,Ze[R.compareFunction])),e.has("EXT_texture_filter_anisotropic")===!0){if(R.magFilter===cn||R.minFilter!==rr&&R.minFilter!==Ei||R.type===ei&&e.has("OES_texture_float_linear")===!1)return;if(R.anisotropy>1||n.get(R).__currentAnisotropy){let ee=e.get("EXT_texture_filter_anisotropic");s.texParameterf(O,ee.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(R.anisotropy,i.getMaxAnisotropy())),n.get(R).__currentAnisotropy=R.anisotropy}}}function ye(O,R){let ee=!1;O.__webglInit===void 0&&(O.__webglInit=!0,R.addEventListener("dispose",T));let ae=R.source,_e=p.get(ae);_e===void 0&&(_e={},p.set(ae,_e));let Ue=X(R);if(Ue!==O.__cacheKey){_e[Ue]===void 0&&(_e[Ue]={texture:s.createTexture(),usedTimes:0},a.memory.textures++,ee=!0),_e[Ue].usedTimes++;let ke=_e[O.__cacheKey];ke!==void 0&&(_e[O.__cacheKey].usedTimes--,ke.usedTimes===0&&S(R)),O.__cacheKey=Ue,O.__webglTexture=_e[Ue].texture}return ee}function J(O,R,ee){return Math.floor(Math.floor(O/ee)/R)}function A(O,R,ee,ae){let Ue=O.updateRanges;if(Ue.length===0)t.texSubImage2D(s.TEXTURE_2D,0,0,0,R.width,R.height,ee,ae,R.data);else{Ue.sort((Te,ve)=>Te.start-ve.start);let ke=0;for(let Te=1;Te<Ue.length;Te++){let ve=Ue[ke],we=Ue[Te],ue=ve.start+ve.count,Ae=J(we.start,R.width,4),ze=J(ve.start,R.width,4);we.start<=ue+1&&Ae===ze&&J(we.start+we.count-1,R.width,4)===Ae?ve.count=Math.max(ve.count,we.start+we.count-ve.start):(++ke,Ue[ke]=we)}Ue.length=ke+1;let B=t.getParameter(s.UNPACK_ROW_LENGTH),Z=t.getParameter(s.UNPACK_SKIP_PIXELS),pe=t.getParameter(s.UNPACK_SKIP_ROWS);t.pixelStorei(s.UNPACK_ROW_LENGTH,R.width);for(let Te=0,ve=Ue.length;Te<ve;Te++){let we=Ue[Te],ue=Math.floor(we.start/4),Ae=Math.ceil(we.count/4),ze=ue%R.width,Y=Math.floor(ue/R.width),Ce=Ae,Re=1;t.pixelStorei(s.UNPACK_SKIP_PIXELS,ze),t.pixelStorei(s.UNPACK_SKIP_ROWS,Y),t.texSubImage2D(s.TEXTURE_2D,0,ze,Y,Ce,Re,ee,ae,R.data)}O.clearUpdateRanges(),t.pixelStorei(s.UNPACK_ROW_LENGTH,B),t.pixelStorei(s.UNPACK_SKIP_PIXELS,Z),t.pixelStorei(s.UNPACK_SKIP_ROWS,pe)}}function me(O,R,ee){let ae=s.TEXTURE_2D;(R.isDataArrayTexture||R.isCompressedArrayTexture)&&(ae=s.TEXTURE_2D_ARRAY),R.isData3DTexture&&(ae=s.TEXTURE_3D);let _e=ye(O,R),Ue=R.source;t.bindTexture(ae,O.__webglTexture,s.TEXTURE0+ee);let ke=n.get(Ue);if(Ue.version!==ke.__version||_e===!0){if(t.activeTexture(s.TEXTURE0+ee),(typeof ImageBitmap<"u"&&R.image instanceof ImageBitmap)===!1){let Re=Dt.getPrimaries(Dt.workingColorSpace),Ve=R.colorSpace===ci?null:Dt.getPrimaries(R.colorSpace),He=R.colorSpace===ci||Re===Ve?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,R.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,R.premultiplyAlpha),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,He)}t.pixelStorei(s.UNPACK_ALIGNMENT,R.unpackAlignment);let Z=m(R.image,!1,i.maxTextureSize);Z=vt(R,Z);let pe=r.convert(R.format,R.colorSpace),Te=r.convert(R.type),ve=g(R.internalFormat,pe,Te,R.normalized,R.colorSpace,R.isVideoTexture);G(ae,R);let we,ue=R.mipmaps,Ae=R.isVideoTexture!==!0,ze=ke.__version===void 0||_e===!0,Y=Ue.dataReady,Ce=w(R,Z);if(R.isDepthTexture)ve=_(R.format===Cs,R.type),ze&&(Ae?t.texStorage2D(s.TEXTURE_2D,1,ve,Z.width,Z.height):t.texImage2D(s.TEXTURE_2D,0,ve,Z.width,Z.height,0,pe,Te,null));else if(R.isDataTexture)if(ue.length>0){Ae&&ze&&t.texStorage2D(s.TEXTURE_2D,Ce,ve,ue[0].width,ue[0].height);for(let Re=0,Ve=ue.length;Re<Ve;Re++)we=ue[Re],Ae?Y&&t.texSubImage2D(s.TEXTURE_2D,Re,0,0,we.width,we.height,pe,Te,we.data):t.texImage2D(s.TEXTURE_2D,Re,ve,we.width,we.height,0,pe,Te,we.data);R.generateMipmaps=!1}else Ae?(ze&&t.texStorage2D(s.TEXTURE_2D,Ce,ve,Z.width,Z.height),Y&&A(R,Z,pe,Te)):t.texImage2D(s.TEXTURE_2D,0,ve,Z.width,Z.height,0,pe,Te,Z.data);else if(R.isCompressedTexture)if(R.isCompressedArrayTexture){Ae&&ze&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Ce,ve,ue[0].width,ue[0].height,Z.depth);for(let Re=0,Ve=ue.length;Re<Ve;Re++)if(we=ue[Re],R.format!==ti)if(pe!==null)if(Ae){if(Y)if(R.layerUpdates.size>0){let He=_u(we.width,we.height,R.format,R.type);for(let De of R.layerUpdates){let Ye=we.data.subarray(De*He/we.data.BYTES_PER_ELEMENT,(De+1)*He/we.data.BYTES_PER_ELEMENT);t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,Re,0,0,De,we.width,we.height,1,pe,Ye)}}else t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,Re,0,0,0,we.width,we.height,Z.depth,pe,we.data)}else t.compressedTexImage3D(s.TEXTURE_2D_ARRAY,Re,ve,we.width,we.height,Z.depth,0,we.data,0,0);else dt("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else Ae?Y&&t.texSubImage3D(s.TEXTURE_2D_ARRAY,Re,0,0,0,we.width,we.height,Z.depth,pe,Te,we.data):t.texImage3D(s.TEXTURE_2D_ARRAY,Re,ve,we.width,we.height,Z.depth,0,pe,Te,we.data);R.layerUpdates.size>0&&R.clearLayerUpdates()}else{Ae&&ze&&t.texStorage2D(s.TEXTURE_2D,Ce,ve,ue[0].width,ue[0].height);for(let Re=0,Ve=ue.length;Re<Ve;Re++)we=ue[Re],R.format!==ti?pe!==null?Ae?Y&&t.compressedTexSubImage2D(s.TEXTURE_2D,Re,0,0,we.width,we.height,pe,we.data):t.compressedTexImage2D(s.TEXTURE_2D,Re,ve,we.width,we.height,0,we.data):dt("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):Ae?Y&&t.texSubImage2D(s.TEXTURE_2D,Re,0,0,we.width,we.height,pe,Te,we.data):t.texImage2D(s.TEXTURE_2D,Re,ve,we.width,we.height,0,pe,Te,we.data)}else if(R.isDataArrayTexture)if(Ae){if(ze&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Ce,ve,Z.width,Z.height,Z.depth),Y)if(R.layerUpdates.size>0){let Re=_u(Z.width,Z.height,R.format,R.type);for(let Ve of R.layerUpdates){let He=Z.data.subarray(Ve*Re/Z.data.BYTES_PER_ELEMENT,(Ve+1)*Re/Z.data.BYTES_PER_ELEMENT);t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,Ve,Z.width,Z.height,1,pe,Te,He)}R.clearLayerUpdates()}else t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,0,Z.width,Z.height,Z.depth,pe,Te,Z.data)}else t.texImage3D(s.TEXTURE_2D_ARRAY,0,ve,Z.width,Z.height,Z.depth,0,pe,Te,Z.data);else if(R.isData3DTexture)Ae?(ze&&t.texStorage3D(s.TEXTURE_3D,Ce,ve,Z.width,Z.height,Z.depth),Y&&t.texSubImage3D(s.TEXTURE_3D,0,0,0,0,Z.width,Z.height,Z.depth,pe,Te,Z.data)):t.texImage3D(s.TEXTURE_3D,0,ve,Z.width,Z.height,Z.depth,0,pe,Te,Z.data);else if(R.isFramebufferTexture){if(ze)if(Ae)t.texStorage2D(s.TEXTURE_2D,Ce,ve,Z.width,Z.height);else{let Re=Z.width,Ve=Z.height;for(let He=0;He<Ce;He++)t.texImage2D(s.TEXTURE_2D,He,ve,Re,Ve,0,pe,Te,null),Re>>=1,Ve>>=1}}else if(R.isHTMLTexture){if("texElementImage2D"in s){let Re=s.canvas;if(Re.hasAttribute("layoutsubtree")||Re.setAttribute("layoutsubtree","true"),Z.parentNode!==Re){Re.appendChild(Z),d.add(R),Re.onpaint=Ve=>{let He=Ve.changedElements;for(let De of d)He.includes(De.image)&&(De.needsUpdate=!0)},Re.requestPaint();return}if(s.texElementImage2D.length===3)s.texElementImage2D(s.TEXTURE_2D,s.RGBA8,Z);else{let He=s.RGBA,De=s.RGBA,Ye=s.UNSIGNED_BYTE;s.texElementImage2D(s.TEXTURE_2D,0,He,De,Ye,Z)}s.texParameteri(s.TEXTURE_2D,s.TEXTURE_MIN_FILTER,s.LINEAR),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_S,s.CLAMP_TO_EDGE),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_T,s.CLAMP_TO_EDGE)}}else if(ue.length>0){if(Ae&&ze){let Re=je(ue[0]);t.texStorage2D(s.TEXTURE_2D,Ce,ve,Re.width,Re.height)}for(let Re=0,Ve=ue.length;Re<Ve;Re++)we=ue[Re],Ae?Y&&t.texSubImage2D(s.TEXTURE_2D,Re,0,0,pe,Te,we):t.texImage2D(s.TEXTURE_2D,Re,ve,pe,Te,we);R.generateMipmaps=!1}else if(Ae){if(ze){let Re=je(Z);t.texStorage2D(s.TEXTURE_2D,Ce,ve,Re.width,Re.height)}Y&&t.texSubImage2D(s.TEXTURE_2D,0,0,0,pe,Te,Z)}else t.texImage2D(s.TEXTURE_2D,0,ve,pe,Te,Z);f(R)&&b(ae),ke.__version=Ue.version,R.onUpdate&&R.onUpdate(R)}O.__version=R.version}function ce(O,R,ee){if(R.image.length!==6)return;let ae=ye(O,R),_e=R.source;t.bindTexture(s.TEXTURE_CUBE_MAP,O.__webglTexture,s.TEXTURE0+ee);let Ue=n.get(_e);if(_e.version!==Ue.__version||ae===!0){t.activeTexture(s.TEXTURE0+ee);let ke=Dt.getPrimaries(Dt.workingColorSpace),B=R.colorSpace===ci?null:Dt.getPrimaries(R.colorSpace),Z=R.colorSpace===ci||ke===B?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,R.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,R.premultiplyAlpha),t.pixelStorei(s.UNPACK_ALIGNMENT,R.unpackAlignment),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,Z);let pe=R.isCompressedTexture||R.image[0].isCompressedTexture,Te=R.image[0]&&R.image[0].isDataTexture,ve=[];for(let De=0;De<6;De++)!pe&&!Te?ve[De]=m(R.image[De],!0,i.maxCubemapSize):ve[De]=Te?R.image[De].image:R.image[De],ve[De]=vt(R,ve[De]);let we=ve[0],ue=r.convert(R.format,R.colorSpace),Ae=r.convert(R.type),ze=g(R.internalFormat,ue,Ae,R.normalized,R.colorSpace),Y=R.isVideoTexture!==!0,Ce=Ue.__version===void 0||ae===!0,Re=_e.dataReady,Ve=w(R,we);G(s.TEXTURE_CUBE_MAP,R);let He;if(pe){Y&&Ce&&t.texStorage2D(s.TEXTURE_CUBE_MAP,Ve,ze,we.width,we.height);for(let De=0;De<6;De++){He=ve[De].mipmaps;for(let Ye=0;Ye<He.length;Ye++){let et=He[Ye];R.format!==ti?ue!==null?Y?Re&&t.compressedTexSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,Ye,0,0,et.width,et.height,ue,et.data):t.compressedTexImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,Ye,ze,et.width,et.height,0,et.data):dt("WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):Y?Re&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,Ye,0,0,et.width,et.height,ue,Ae,et.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,Ye,ze,et.width,et.height,0,ue,Ae,et.data)}}}else{if(He=R.mipmaps,Y&&Ce){He.length>0&&Ve++;let De=je(ve[0]);t.texStorage2D(s.TEXTURE_CUBE_MAP,Ve,ze,De.width,De.height)}for(let De=0;De<6;De++)if(Te){Y?Re&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,0,0,0,ve[De].width,ve[De].height,ue,Ae,ve[De].data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,0,ze,ve[De].width,ve[De].height,0,ue,Ae,ve[De].data);for(let Ye=0;Ye<He.length;Ye++){let Fe=He[Ye].image[De].image;Y?Re&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,Ye+1,0,0,Fe.width,Fe.height,ue,Ae,Fe.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,Ye+1,ze,Fe.width,Fe.height,0,ue,Ae,Fe.data)}}else{Y?Re&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,0,0,0,ue,Ae,ve[De]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,0,ze,ue,Ae,ve[De]);for(let Ye=0;Ye<He.length;Ye++){let et=He[Ye];Y?Re&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,Ye+1,0,0,ue,Ae,et.image[De]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+De,Ye+1,ze,ue,Ae,et.image[De])}}}f(R)&&b(s.TEXTURE_CUBE_MAP),Ue.__version=_e.version,R.onUpdate&&R.onUpdate(R)}O.__version=R.version}function ne(O,R,ee,ae,_e,Ue){let ke=r.convert(ee.format,ee.colorSpace),B=r.convert(ee.type),Z=g(ee.internalFormat,ke,B,ee.normalized,ee.colorSpace),pe=n.get(R),Te=n.get(ee);if(Te.__renderTarget=R,!pe.__hasExternalTextures){let ve=Math.max(1,R.width>>Ue),we=Math.max(1,R.height>>Ue);_e===s.TEXTURE_3D||_e===s.TEXTURE_2D_ARRAY?t.texImage3D(_e,Ue,Z,ve,we,R.depth,0,ke,B,null):t.texImage2D(_e,Ue,Z,ve,we,0,ke,B,null)}t.bindFramebuffer(s.FRAMEBUFFER,O),at(R)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,ae,_e,Te.__webglTexture,0,rt(R)):(_e===s.TEXTURE_2D||_e>=s.TEXTURE_CUBE_MAP_POSITIVE_X&&_e<=s.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&s.framebufferTexture2D(s.FRAMEBUFFER,ae,_e,Te.__webglTexture,Ue),t.bindFramebuffer(s.FRAMEBUFFER,null)}function K(O,R,ee){if(s.bindRenderbuffer(s.RENDERBUFFER,O),R.depthBuffer){let ae=R.depthTexture,_e=ae&&ae.isDepthTexture?ae.type:null,Ue=_(R.stencilBuffer,_e),ke=R.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;at(R)?o.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,rt(R),Ue,R.width,R.height):ee?s.renderbufferStorageMultisample(s.RENDERBUFFER,rt(R),Ue,R.width,R.height):s.renderbufferStorage(s.RENDERBUFFER,Ue,R.width,R.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,ke,s.RENDERBUFFER,O)}else{let ae=R.textures;for(let _e=0;_e<ae.length;_e++){let Ue=ae[_e],ke=r.convert(Ue.format,Ue.colorSpace),B=r.convert(Ue.type),Z=g(Ue.internalFormat,ke,B,Ue.normalized,Ue.colorSpace);at(R)?o.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,rt(R),Z,R.width,R.height):ee?s.renderbufferStorageMultisample(s.RENDERBUFFER,rt(R),Z,R.width,R.height):s.renderbufferStorage(s.RENDERBUFFER,Z,R.width,R.height)}}s.bindRenderbuffer(s.RENDERBUFFER,null)}function oe(O,R,ee){let ae=R.isWebGLCubeRenderTarget===!0;if(t.bindFramebuffer(s.FRAMEBUFFER,O),!(R.depthTexture&&R.depthTexture.isDepthTexture))throw new Error("THREE.WebGLTextures: renderTarget.depthTexture must be an instance of THREE.DepthTexture.");let _e=n.get(R.depthTexture);if(_e.__renderTarget=R,(!_e.__webglTexture||R.depthTexture.image.width!==R.width||R.depthTexture.image.height!==R.height)&&(R.depthTexture.image.width=R.width,R.depthTexture.image.height=R.height,R.depthTexture.needsUpdate=!0),ae){if(_e.__webglInit===void 0&&(_e.__webglInit=!0,R.depthTexture.addEventListener("dispose",T)),_e.__webglTexture===void 0){_e.__webglTexture=s.createTexture(),t.bindTexture(s.TEXTURE_CUBE_MAP,_e.__webglTexture),G(s.TEXTURE_CUBE_MAP,R.depthTexture);let pe=r.convert(R.depthTexture.format),Te=r.convert(R.depthTexture.type),ve;R.depthTexture.format===Li?ve=s.DEPTH_COMPONENT24:R.depthTexture.format===Cs&&(ve=s.DEPTH24_STENCIL8);for(let we=0;we<6;we++)s.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+we,0,ve,R.width,R.height,0,pe,Te,null)}}else j(R.depthTexture,0);let Ue=_e.__webglTexture,ke=rt(R),B=ae?s.TEXTURE_CUBE_MAP_POSITIVE_X+ee:s.TEXTURE_2D,Z=R.depthTexture.format===Cs?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;if(R.depthTexture.format===Li)at(R)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,Z,B,Ue,0,ke):s.framebufferTexture2D(s.FRAMEBUFFER,Z,B,Ue,0);else if(R.depthTexture.format===Cs)at(R)?o.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,Z,B,Ue,0,ke):s.framebufferTexture2D(s.FRAMEBUFFER,Z,B,Ue,0);else throw new Error("THREE.WebGLTextures: Unknown depthTexture format.")}function re(O){let R=n.get(O),ee=O.isWebGLCubeRenderTarget===!0;if(R.__boundDepthTexture!==O.depthTexture){let ae=O.depthTexture;if(R.__depthDisposeCallback&&R.__depthDisposeCallback(),ae){let _e=()=>{delete R.__boundDepthTexture,delete R.__depthDisposeCallback,ae.removeEventListener("dispose",_e)};ae.addEventListener("dispose",_e),R.__depthDisposeCallback=_e}R.__boundDepthTexture=ae}if(O.depthTexture&&!R.__autoAllocateDepthBuffer)if(ee)for(let ae=0;ae<6;ae++)oe(R.__webglFramebuffer[ae],O,ae);else{let ae=O.texture.mipmaps;ae&&ae.length>0?oe(R.__webglFramebuffer[0],O,0):oe(R.__webglFramebuffer,O,0)}else if(ee){R.__webglDepthbuffer=[];for(let ae=0;ae<6;ae++)if(t.bindFramebuffer(s.FRAMEBUFFER,R.__webglFramebuffer[ae]),R.__webglDepthbuffer[ae]===void 0)R.__webglDepthbuffer[ae]=s.createRenderbuffer(),K(R.__webglDepthbuffer[ae],O,!1);else{let _e=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Ue=R.__webglDepthbuffer[ae];s.bindRenderbuffer(s.RENDERBUFFER,Ue),s.framebufferRenderbuffer(s.FRAMEBUFFER,_e,s.RENDERBUFFER,Ue)}}else{let ae=O.texture.mipmaps;if(ae&&ae.length>0?t.bindFramebuffer(s.FRAMEBUFFER,R.__webglFramebuffer[0]):t.bindFramebuffer(s.FRAMEBUFFER,R.__webglFramebuffer),R.__webglDepthbuffer===void 0)R.__webglDepthbuffer=s.createRenderbuffer(),K(R.__webglDepthbuffer,O,!1);else{let _e=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Ue=R.__webglDepthbuffer;s.bindRenderbuffer(s.RENDERBUFFER,Ue),s.framebufferRenderbuffer(s.FRAMEBUFFER,_e,s.RENDERBUFFER,Ue)}}t.bindFramebuffer(s.FRAMEBUFFER,null)}function te(O,R,ee){let ae=n.get(O);R!==void 0&&ne(ae.__webglFramebuffer,O,O.texture,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,0),ee!==void 0&&re(O)}function fe(O){let R=O.texture,ee=n.get(O),ae=n.get(R);O.addEventListener("dispose",y);let _e=O.textures,Ue=O.isWebGLCubeRenderTarget===!0,ke=_e.length>1;if(ke||(ae.__webglTexture===void 0&&(ae.__webglTexture=s.createTexture()),ae.__version=R.version,a.memory.textures++),Ue){ee.__webglFramebuffer=[];for(let B=0;B<6;B++)if(R.mipmaps&&R.mipmaps.length>0){ee.__webglFramebuffer[B]=[];for(let Z=0;Z<R.mipmaps.length;Z++)ee.__webglFramebuffer[B][Z]=s.createFramebuffer()}else ee.__webglFramebuffer[B]=s.createFramebuffer()}else{if(R.mipmaps&&R.mipmaps.length>0){ee.__webglFramebuffer=[];for(let B=0;B<R.mipmaps.length;B++)ee.__webglFramebuffer[B]=s.createFramebuffer()}else ee.__webglFramebuffer=s.createFramebuffer();if(ke)for(let B=0,Z=_e.length;B<Z;B++){let pe=n.get(_e[B]);pe.__webglTexture===void 0&&(pe.__webglTexture=s.createTexture(),a.memory.textures++)}if(O.samples>0&&at(O)===!1){ee.__webglMultisampledFramebuffer=s.createFramebuffer(),ee.__webglColorRenderbuffer=[],t.bindFramebuffer(s.FRAMEBUFFER,ee.__webglMultisampledFramebuffer);for(let B=0;B<_e.length;B++){let Z=_e[B];ee.__webglColorRenderbuffer[B]=s.createRenderbuffer(),s.bindRenderbuffer(s.RENDERBUFFER,ee.__webglColorRenderbuffer[B]);let pe=r.convert(Z.format,Z.colorSpace),Te=r.convert(Z.type),ve=g(Z.internalFormat,pe,Te,Z.normalized,Z.colorSpace,O.isXRRenderTarget===!0),we=rt(O);s.renderbufferStorageMultisample(s.RENDERBUFFER,we,ve,O.width,O.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+B,s.RENDERBUFFER,ee.__webglColorRenderbuffer[B])}s.bindRenderbuffer(s.RENDERBUFFER,null),O.depthBuffer&&(ee.__webglDepthRenderbuffer=s.createRenderbuffer(),K(ee.__webglDepthRenderbuffer,O,!0)),t.bindFramebuffer(s.FRAMEBUFFER,null)}}if(Ue){t.bindTexture(s.TEXTURE_CUBE_MAP,ae.__webglTexture),G(s.TEXTURE_CUBE_MAP,R);for(let B=0;B<6;B++)if(R.mipmaps&&R.mipmaps.length>0)for(let Z=0;Z<R.mipmaps.length;Z++)ne(ee.__webglFramebuffer[B][Z],O,R,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+B,Z);else ne(ee.__webglFramebuffer[B],O,R,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+B,0);f(R)&&b(s.TEXTURE_CUBE_MAP),t.unbindTexture()}else if(ke){for(let B=0,Z=_e.length;B<Z;B++){let pe=_e[B],Te=n.get(pe),ve=s.TEXTURE_2D;(O.isWebGL3DRenderTarget||O.isWebGLArrayRenderTarget)&&(ve=O.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(ve,Te.__webglTexture),G(ve,pe),ne(ee.__webglFramebuffer,O,pe,s.COLOR_ATTACHMENT0+B,ve,0),f(pe)&&b(ve)}t.unbindTexture()}else{let B=s.TEXTURE_2D;if((O.isWebGL3DRenderTarget||O.isWebGLArrayRenderTarget)&&(B=O.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(B,ae.__webglTexture),G(B,R),R.mipmaps&&R.mipmaps.length>0)for(let Z=0;Z<R.mipmaps.length;Z++)ne(ee.__webglFramebuffer[Z],O,R,s.COLOR_ATTACHMENT0,B,Z);else ne(ee.__webglFramebuffer,O,R,s.COLOR_ATTACHMENT0,B,0);f(R)&&b(B),t.unbindTexture()}O.depthBuffer&&re(O)}function Ee(O){let R=O.textures;for(let ee=0,ae=R.length;ee<ae;ee++){let _e=R[ee];if(f(_e)){let Ue=E(O),ke=n.get(_e).__webglTexture;t.bindTexture(Ue,ke),b(Ue),t.unbindTexture()}}}let Ne=[],Oe=[];function ut(O){if(O.samples>0){if(at(O)===!1){let R=O.textures,ee=O.width,ae=O.height,_e=s.COLOR_BUFFER_BIT,Ue=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,ke=n.get(O),B=R.length>1;if(B)for(let pe=0;pe<R.length;pe++)t.bindFramebuffer(s.FRAMEBUFFER,ke.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+pe,s.RENDERBUFFER,null),t.bindFramebuffer(s.FRAMEBUFFER,ke.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+pe,s.TEXTURE_2D,null,0);t.bindFramebuffer(s.READ_FRAMEBUFFER,ke.__webglMultisampledFramebuffer);let Z=O.texture.mipmaps;Z&&Z.length>0?t.bindFramebuffer(s.DRAW_FRAMEBUFFER,ke.__webglFramebuffer[0]):t.bindFramebuffer(s.DRAW_FRAMEBUFFER,ke.__webglFramebuffer);for(let pe=0;pe<R.length;pe++){if(O.resolveDepthBuffer&&(O.depthBuffer&&(_e|=s.DEPTH_BUFFER_BIT),O.stencilBuffer&&O.resolveStencilBuffer&&(_e|=s.STENCIL_BUFFER_BIT)),B){s.framebufferRenderbuffer(s.READ_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.RENDERBUFFER,ke.__webglColorRenderbuffer[pe]);let Te=n.get(R[pe]).__webglTexture;s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,Te,0)}s.blitFramebuffer(0,0,ee,ae,0,0,ee,ae,_e,s.NEAREST),l===!0&&(Ne.length=0,Oe.length=0,Ne.push(s.COLOR_ATTACHMENT0+pe),O.depthBuffer&&O.storeMultisampledDepthBuffer===!1&&(Ne.push(Ue),Oe.push(Ue),s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,Oe)),s.invalidateFramebuffer(s.READ_FRAMEBUFFER,Ne))}if(t.bindFramebuffer(s.READ_FRAMEBUFFER,null),t.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),B)for(let pe=0;pe<R.length;pe++){t.bindFramebuffer(s.FRAMEBUFFER,ke.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+pe,s.RENDERBUFFER,ke.__webglColorRenderbuffer[pe]);let Te=n.get(R[pe]).__webglTexture;t.bindFramebuffer(s.FRAMEBUFFER,ke.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+pe,s.TEXTURE_2D,Te,0)}t.bindFramebuffer(s.DRAW_FRAMEBUFFER,ke.__webglMultisampledFramebuffer)}else if(O.depthBuffer&&O.storeMultisampledDepthBuffer===!1&&l){let R=O.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,[R])}}}function rt(O){return Math.min(i.maxSamples,O.samples)}function at(O){let R=n.get(O);return O.samples>0&&e.has("WEBGL_multisampled_render_to_texture")===!0&&R.__useRenderToTexture!==!1}function Q(O){let R=a.render.frame;h.get(O)!==R&&(h.set(O,R),O.update())}function vt(O,R){let ee=O.colorSpace,ae=O.format,_e=O.type;return O.isCompressedTexture===!0||O.isVideoTexture===!0||ee!==Hn&&ee!==ci&&(Dt.getTransfer(ee)===Xt?(ae!==ti||_e!==Xn)&&dt("WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):_t("WebGLTextures: Unsupported texture color space:",ee)),R}function je(O){return typeof HTMLImageElement<"u"&&O instanceof HTMLImageElement?(c.width=O.naturalWidth||O.width,c.height=O.naturalHeight||O.height):typeof VideoFrame<"u"&&O instanceof VideoFrame?(c.width=O.displayWidth,c.height=O.displayHeight):(c.width=O.width,c.height=O.height),c}this.allocateTextureUnit=z,this.resetTextureUnits=I,this.getTextureUnits=P,this.setTextureUnits=N,this.setTexture2D=j,this.setTexture2DArray=V,this.setTexture3D=W,this.setTextureCube=ie,this.rebindTextures=te,this.setupRenderTarget=fe,this.updateRenderTargetMipmap=Ee,this.updateMultisampleRenderTarget=ut,this.setupDepthRenderbuffer=re,this.setupFrameBufferTexture=ne,this.useMultisampledRTT=at,this.isReversedDepthBuffer=function(){return t.buffers.depth.getReversed()}}function R_(s,e){function t(n,i=ci){let r,a=Dt.getTransfer(i);if(n===Xn)return s.UNSIGNED_BYTE;if(n===$l)return s.UNSIGNED_SHORT_4_4_4_4;if(n===Ql)return s.UNSIGNED_SHORT_5_5_5_1;if(n===cu)return s.UNSIGNED_INT_5_9_9_9_REV;if(n===hu)return s.UNSIGNED_INT_10F_11F_11F_REV;if(n===ou)return s.BYTE;if(n===lu)return s.SHORT;if(n===ea)return s.UNSIGNED_SHORT;if(n===Zl)return s.INT;if(n===Ti)return s.UNSIGNED_INT;if(n===ei)return s.FLOAT;if(n===xn)return s.HALF_FLOAT;if(n===uu)return s.ALPHA;if(n===du)return s.RGB;if(n===ti)return s.RGBA;if(n===Li)return s.DEPTH_COMPONENT;if(n===Cs)return s.DEPTH_STENCIL;if(n===ec)return s.RED;if(n===tc)return s.RED_INTEGER;if(n===Ps)return s.RG;if(n===nc)return s.RG_INTEGER;if(n===ic)return s.RGBA_INTEGER;if(n===po||n===mo||n===go||n===xo)if(a===Xt)if(r=e.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(n===po)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(n===mo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(n===go)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(n===xo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=e.get("WEBGL_compressed_texture_s3tc"),r!==null){if(n===po)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(n===mo)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(n===go)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(n===xo)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(n===sc||n===rc||n===ac||n===oc)if(r=e.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(n===sc)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(n===rc)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(n===ac)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(n===oc)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(n===lc||n===cc||n===hc||n===uc||n===dc||n===bo||n===fc)if(r=e.get("WEBGL_compressed_texture_etc"),r!==null){if(n===lc||n===cc)return a===Xt?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(n===hc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC;if(n===uc)return r.COMPRESSED_R11_EAC;if(n===dc)return r.COMPRESSED_SIGNED_R11_EAC;if(n===bo)return r.COMPRESSED_RG11_EAC;if(n===fc)return r.COMPRESSED_SIGNED_RG11_EAC}else return null;if(n===pc||n===mc||n===gc||n===xc||n===bc||n===vc||n===yc||n===_c||n===Mc||n===wc||n===Sc||n===Ec||n===Tc||n===Ac)if(r=e.get("WEBGL_compressed_texture_astc"),r!==null){if(n===pc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(n===mc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(n===gc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(n===xc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(n===bc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(n===vc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(n===yc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(n===_c)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(n===Mc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(n===wc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(n===Sc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(n===Ec)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(n===Tc)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(n===Ac)return a===Xt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(n===Rc||n===Cc||n===Pc)if(r=e.get("EXT_texture_compression_bptc"),r!==null){if(n===Rc)return a===Xt?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(n===Cc)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(n===Pc)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(n===Ic||n===Dc||n===vo||n===Lc)if(r=e.get("EXT_texture_compression_rgtc"),r!==null){if(n===Ic)return r.COMPRESSED_RED_RGTC1_EXT;if(n===Dc)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(n===vo)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(n===Lc)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return n===ta?s.UNSIGNED_INT_24_8:s[n]!==void 0?s[n]:null}return{convert:t}}var C_=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,P_=`
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

}`,Nu=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(e,t){if(this.texture===null){let n=new Va(e.texture);(e.depthNear!==t.depthNear||e.depthFar!==t.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=n}}getMesh(e){if(this.texture!==null&&this.mesh===null){let t=e.cameras[0].viewport,n=new Rt({vertexShader:C_,fragmentShader:P_,uniforms:{depthColor:{value:this.texture},depthWidth:{value:t.z},depthHeight:{value:t.w}}});this.mesh=new ht(new dn(20,20),n)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},Uu=class extends ai{constructor(e,t){super();let n=this,i=null,r=1,a=null,o="local-floor",l=1,c=null,h=null,d=null,u=null,p=null,x=null,M=typeof XRWebGLBinding<"u",m=new Nu,f={},b=t.getContextAttributes(),E=null,g=null,_=[],w=[],T=new Ge,y=null,C=null,S=new gn;S.viewport=new Ot;let D=new gn;D.viewport=new Ot;let v=[S,D],I=new Wl,P=null,N=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(J){let A=_[J];return A===void 0&&(A=new zr,_[J]=A),A.getTargetRaySpace()},this.getControllerGrip=function(J){let A=_[J];return A===void 0&&(A=new zr,_[J]=A),A.getGripSpace()},this.getHand=function(J){let A=_[J];return A===void 0&&(A=new zr,_[J]=A),A.getHandSpace()};function z(J){let A=w.indexOf(J.inputSource);if(A===-1)return;let me=_[A];me!==void 0&&(me.update(J.inputSource,J.frame,c||a),me.dispatchEvent({type:J.type,data:J.inputSource}))}function X(){i.removeEventListener("select",z),i.removeEventListener("selectstart",z),i.removeEventListener("selectend",z),i.removeEventListener("squeeze",z),i.removeEventListener("squeezestart",z),i.removeEventListener("squeezeend",z),i.removeEventListener("end",X),i.removeEventListener("inputsourceschange",j);for(let J=0;J<_.length;J++){let A=w[J];A!==null&&(w[J]=null,_[J].disconnect(A))}P=null,N=null,m.reset();for(let J in f)delete f[J];if(e.setRenderTarget(E),p=null,u=null,d=null,i=null,g=null,ye.stop(),n.isPresenting=!1,e.setPixelRatio(y),e.setSize(T.width,T.height,!1),C!==null){let J=C.camera;J.fov=C.fov,J.zoom=C.zoom,J.updateProjectionMatrix(),C=null}n.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(J){r=J,n.isPresenting===!0&&dt("WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(J){o=J,n.isPresenting===!0&&dt("WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return c||a},this.setReferenceSpace=function(J){c=J},this.getBaseLayer=function(){return u!==null?u:p},this.getBinding=function(){return d===null&&M&&(d=new XRWebGLBinding(i,t)),d},this.getFrame=function(){return x},this.getSession=function(){return i},this.setSession=async function(J){if(i=J,i!==null){if(E=e.getRenderTarget(),i.addEventListener("select",z),i.addEventListener("selectstart",z),i.addEventListener("selectend",z),i.addEventListener("squeeze",z),i.addEventListener("squeezestart",z),i.addEventListener("squeezeend",z),i.addEventListener("end",X),i.addEventListener("inputsourceschange",j),b.xrCompatible!==!0&&await t.makeXRCompatible(),y=e.getPixelRatio(),e.getSize(T),M&&"createProjectionLayer"in XRWebGLBinding.prototype){let me=null,ce=null,ne=null;b.depth&&(ne=b.stencil?t.DEPTH24_STENCIL8:t.DEPTH_COMPONENT24,me=b.stencil?Cs:Li,ce=b.stencil?ta:Ti);let K={colorFormat:t.RGBA8,depthFormat:ne,scaleFactor:r};d=this.getBinding(),u=d.createProjectionLayer(K),i.updateRenderState({layers:[u]}),e.setPixelRatio(1),e.setSize(u.textureWidth,u.textureHeight,!1),g=new ln(u.textureWidth,u.textureHeight,{format:ti,type:Xn,depthTexture:new Ms(u.textureWidth,u.textureHeight,ce,void 0,void 0,void 0,void 0,void 0,void 0,me),stencilBuffer:b.stencil,colorSpace:e.outputColorSpace,samples:b.antialias?4:0,resolveDepthBuffer:u.ignoreDepthValues===!1,resolveStencilBuffer:u.ignoreDepthValues===!1,storeMultisampledDepthBuffer:u.ignoreDepthValues===!1,storeMultisampledStencilBuffer:u.ignoreDepthValues===!1})}else{let me={antialias:b.antialias,alpha:!0,depth:b.depth,stencil:b.stencil,framebufferScaleFactor:r};p=new XRWebGLLayer(i,t,me),i.updateRenderState({baseLayer:p}),e.setPixelRatio(1),e.setSize(p.framebufferWidth,p.framebufferHeight,!1),g=new ln(p.framebufferWidth,p.framebufferHeight,{format:ti,type:Xn,colorSpace:e.outputColorSpace,stencilBuffer:b.stencil,resolveDepthBuffer:p.ignoreDepthValues===!1,resolveStencilBuffer:p.ignoreDepthValues===!1,storeMultisampledDepthBuffer:p.ignoreDepthValues===!1,storeMultisampledStencilBuffer:p.ignoreDepthValues===!1})}g.isXRRenderTarget=!0,this.setFoveation(l),c=null,a=await i.requestReferenceSpace(o),ye.setContext(i),ye.start(),n.isPresenting=!0,n.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(i!==null)return i.environmentBlendMode},this.getDepthTexture=function(){return m.getDepthTexture()};function j(J){for(let A=0;A<J.removed.length;A++){let me=J.removed[A],ce=w.indexOf(me);ce>=0&&(w[ce]=null,_[ce].disconnect(me))}for(let A=0;A<J.added.length;A++){let me=J.added[A],ce=w.indexOf(me);if(ce===-1){for(let K=0;K<_.length;K++)if(K>=w.length){w.push(me),ce=K;break}else if(w[K]===null){w[K]=me,ce=K;break}if(ce===-1)break}let ne=_[ce];ne&&ne.connect(me)}}let V=new U,W=new U;function ie(J,A,me){V.setFromMatrixPosition(A.matrixWorld),W.setFromMatrixPosition(me.matrixWorld);let ce=V.distanceTo(W),ne=A.projectionMatrix.elements,K=me.projectionMatrix.elements,oe=ne[14]/(ne[10]-1),re=ne[14]/(ne[10]+1),te=(ne[9]+1)/ne[5],fe=(ne[9]-1)/ne[5],Ee=(ne[8]-1)/ne[0],Ne=(K[8]+1)/K[0],Oe=oe*Ee,ut=oe*Ne,rt=ce/(-Ee+Ne),at=rt*-Ee;if(A.matrixWorld.decompose(J.position,J.quaternion,J.scale),J.translateX(at),J.translateZ(rt),J.matrixWorld.compose(J.position,J.quaternion,J.scale),J.matrixWorldInverse.copy(J.matrixWorld).invert(),ne[10]===-1)J.projectionMatrix.copy(A.projectionMatrix),J.projectionMatrixInverse.copy(A.projectionMatrixInverse);else{let Q=oe+rt,vt=re+rt,je=Oe-at,O=ut+(ce-at),R=te*re/vt*Q,ee=fe*re/vt*Q;J.projectionMatrix.makePerspective(je,O,R,ee,Q,vt),J.projectionMatrixInverse.copy(J.projectionMatrix).invert()}}function H(J,A){A===null?J.matrixWorld.copy(J.matrix):J.matrixWorld.multiplyMatrices(A.matrixWorld,J.matrix),J.matrixWorldInverse.copy(J.matrixWorld).invert()}this.updateCamera=function(J){if(i===null)return;let A=J.near,me=J.far;m.texture!==null&&(m.depthNear>0&&(A=m.depthNear),m.depthFar>0&&(me=m.depthFar)),I.near=D.near=S.near=A,I.far=D.far=S.far=me,(P!==I.near||N!==I.far)&&(i.updateRenderState({depthNear:I.near,depthFar:I.far}),P=I.near,N=I.far),I.layers.mask=J.layers.mask|6,S.layers.mask=I.layers.mask&-5,D.layers.mask=I.layers.mask&-3;let ce=J.parent,ne=I.cameras;H(I,ce);for(let K=0;K<ne.length;K++)H(ne[K],ce);ne.length===2?ie(I,S,D):I.projectionMatrix.copy(S.projectionMatrix),C===null&&J.isPerspectiveCamera&&(C={camera:J,fov:J.fov,zoom:J.zoom}),ge(J,I,ce)};function ge(J,A,me){me===null?J.matrix.copy(A.matrixWorld):(J.matrix.copy(me.matrixWorld),J.matrix.invert(),J.matrix.multiply(A.matrixWorld)),J.matrix.decompose(J.position,J.quaternion,J.scale),J.updateMatrixWorld(!0),J.projectionMatrix.copy(A.projectionMatrix),J.projectionMatrixInverse.copy(A.projectionMatrixInverse),J.isPerspectiveCamera&&(J.fov=Ks*2*Math.atan(1/J.projectionMatrix.elements[5]),J.zoom=1)}this.getCamera=function(){return I},this.getFoveation=function(){if(!(u===null&&p===null))return l},this.setFoveation=function(J){l=J,u!==null&&(u.fixedFoveation=J),p!==null&&p.fixedFoveation!==void 0&&(p.fixedFoveation=J)},this.hasDepthSensing=function(){return m.texture!==null},this.getDepthSensingMesh=function(){return m.getMesh(I)},this.getCameraTexture=function(J){return f[J]};let Ze=null;function G(J,A){if(h=A.getViewerPose(c||a),x=A,h!==null){let me=h.views;p!==null&&(e.setRenderTargetFramebuffer(g,p.framebuffer),e.setRenderTarget(g));let ce=!1;me.length!==I.cameras.length&&(I.cameras.length=0,ce=!0);for(let re=0;re<me.length;re++){let te=me[re],fe=null;if(p!==null)fe=p.getViewport(te);else{let Ne=d.getViewSubImage(u,te);fe=Ne.viewport,re===0&&(e.setRenderTargetTextures(g,Ne.colorTexture,Ne.depthStencilTexture),e.setRenderTarget(g))}let Ee=v[re];Ee===void 0&&(Ee=new gn,Ee.layers.enable(re),Ee.viewport=new Ot,v[re]=Ee),Ee.matrix.fromArray(te.transform.matrix),Ee.matrix.decompose(Ee.position,Ee.quaternion,Ee.scale),Ee.projectionMatrix.fromArray(te.projectionMatrix),Ee.projectionMatrixInverse.copy(Ee.projectionMatrix).invert(),Ee.viewport.set(fe.x,fe.y,fe.width,fe.height),re===0&&(I.matrix.copy(Ee.matrix),I.matrix.decompose(I.position,I.quaternion,I.scale)),ce===!0&&I.cameras.push(Ee)}let ne=i.enabledFeatures;if(ne&&ne.includes("depth-sensing")&&i.depthUsage=="gpu-optimized"&&M){d=n.getBinding();let re=d.getDepthInformation(me[0]);re&&re.isValid&&re.texture&&m.init(re,i.renderState)}if(ne&&ne.includes("camera-access")&&M){e.state.unbindTexture(),d=n.getBinding();for(let re=0;re<me.length;re++){let te=me[re].camera;if(te){let fe=f[te];fe||(fe=new Va,f[te]=fe);let Ee=d.getCameraImage(te);fe.sourceTexture=Ee}}}}for(let me=0;me<_.length;me++){let ce=w[me],ne=_[me];ce!==null&&ne!==void 0&&ne.update(ce,A,c||a)}Ze&&Ze(J,A),A.detectedPlanes&&n.dispatchEvent({type:"planesdetected",data:A}),x=null}let ye=new kp;ye.setAnimationLoop(G),this.setAnimationLoop=function(J){Ze=J},this.dispose=function(){}}},I_=new yt,qp=new Tt;qp.set(-1,0,0,0,1,0,0,0,1);function D_(s,e){function t(m,f){m.matrixAutoUpdate===!0&&m.updateMatrix(),f.value.copy(m.matrix)}function n(m,f){f.color.getRGB(m.fogColor.value,bu(s)),f.isFog?(m.fogNear.value=f.near,m.fogFar.value=f.far):f.isFogExp2&&(m.fogDensity.value=f.density)}function i(m,f,b,E,g){f.isNodeMaterial?f.uniformsNeedUpdate=!1:f.isMeshBasicMaterial?r(m,f):f.isMeshLambertMaterial?(r(m,f),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)):f.isMeshToonMaterial?(r(m,f),d(m,f)):f.isMeshPhongMaterial?(r(m,f),h(m,f),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)):f.isMeshStandardMaterial?(r(m,f),u(m,f),f.isMeshPhysicalMaterial&&p(m,f,g)):f.isMeshMatcapMaterial?(r(m,f),x(m,f)):f.isMeshDepthMaterial?r(m,f):f.isMeshDistanceMaterial?(r(m,f),M(m,f)):f.isMeshNormalMaterial?r(m,f):f.isLineBasicMaterial?(a(m,f),f.isLineDashedMaterial&&o(m,f)):f.isPointsMaterial?l(m,f,b,E):f.isSpriteMaterial?c(m,f):f.isShadowMaterial?(m.color.value.copy(f.color),m.opacity.value=f.opacity):f.isShaderMaterial&&(f.uniformsNeedUpdate=!1)}function r(m,f){m.opacity.value=f.opacity,f.color&&m.diffuse.value.copy(f.color),f.emissive&&m.emissive.value.copy(f.emissive).multiplyScalar(f.emissiveIntensity),f.map&&(m.map.value=f.map,t(f.map,m.mapTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.bumpMap&&(m.bumpMap.value=f.bumpMap,t(f.bumpMap,m.bumpMapTransform),m.bumpScale.value=f.bumpScale,f.side===yn&&(m.bumpScale.value*=-1)),f.normalMap&&(m.normalMap.value=f.normalMap,t(f.normalMap,m.normalMapTransform),m.normalScale.value.copy(f.normalScale),f.side===yn&&m.normalScale.value.negate()),f.displacementMap&&(m.displacementMap.value=f.displacementMap,t(f.displacementMap,m.displacementMapTransform),m.displacementScale.value=f.displacementScale,m.displacementBias.value=f.displacementBias),f.emissiveMap&&(m.emissiveMap.value=f.emissiveMap,t(f.emissiveMap,m.emissiveMapTransform)),f.specularMap&&(m.specularMap.value=f.specularMap,t(f.specularMap,m.specularMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest);let b=e.get(f),E=b.envMap,g=b.envMapRotation;E&&(m.envMap.value=E,m.envMapRotation.value.setFromMatrix4(I_.makeRotationFromEuler(g)).transpose(),E.isCubeTexture&&E.isRenderTargetTexture===!1&&m.envMapRotation.value.premultiply(qp),m.reflectivity.value=f.reflectivity,m.ior.value=f.ior,m.refractionRatio.value=f.refractionRatio),f.lightMap&&(m.lightMap.value=f.lightMap,m.lightMapIntensity.value=f.lightMapIntensity,t(f.lightMap,m.lightMapTransform)),f.aoMap&&(m.aoMap.value=f.aoMap,m.aoMapIntensity.value=f.aoMapIntensity,t(f.aoMap,m.aoMapTransform))}function a(m,f){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,f.map&&(m.map.value=f.map,t(f.map,m.mapTransform))}function o(m,f){m.dashSize.value=f.dashSize,m.totalSize.value=f.dashSize+f.gapSize,m.scale.value=f.scale}function l(m,f,b,E){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,m.size.value=f.size*b,m.scale.value=E*.5,f.map&&(m.map.value=f.map,t(f.map,m.uvTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest)}function c(m,f){m.diffuse.value.copy(f.color),m.opacity.value=f.opacity,m.rotation.value=f.rotation,f.map&&(m.map.value=f.map,t(f.map,m.mapTransform)),f.alphaMap&&(m.alphaMap.value=f.alphaMap,t(f.alphaMap,m.alphaMapTransform)),f.alphaTest>0&&(m.alphaTest.value=f.alphaTest)}function h(m,f){m.specular.value.copy(f.specular),m.shininess.value=Math.max(f.shininess,1e-4)}function d(m,f){f.gradientMap&&(m.gradientMap.value=f.gradientMap)}function u(m,f){m.metalness.value=f.metalness,f.metalnessMap&&(m.metalnessMap.value=f.metalnessMap,t(f.metalnessMap,m.metalnessMapTransform)),m.roughness.value=f.roughness,f.roughnessMap&&(m.roughnessMap.value=f.roughnessMap,t(f.roughnessMap,m.roughnessMapTransform)),f.envMap&&(m.envMapIntensity.value=f.envMapIntensity)}function p(m,f,b){m.ior.value=f.ior,f.sheen>0&&(m.sheenColor.value.copy(f.sheenColor).multiplyScalar(f.sheen),m.sheenRoughness.value=f.sheenRoughness,f.sheenColorMap&&(m.sheenColorMap.value=f.sheenColorMap,t(f.sheenColorMap,m.sheenColorMapTransform)),f.sheenRoughnessMap&&(m.sheenRoughnessMap.value=f.sheenRoughnessMap,t(f.sheenRoughnessMap,m.sheenRoughnessMapTransform))),f.clearcoat>0&&(m.clearcoat.value=f.clearcoat,m.clearcoatRoughness.value=f.clearcoatRoughness,f.clearcoatMap&&(m.clearcoatMap.value=f.clearcoatMap,t(f.clearcoatMap,m.clearcoatMapTransform)),f.clearcoatRoughnessMap&&(m.clearcoatRoughnessMap.value=f.clearcoatRoughnessMap,t(f.clearcoatRoughnessMap,m.clearcoatRoughnessMapTransform)),f.clearcoatNormalMap&&(m.clearcoatNormalMap.value=f.clearcoatNormalMap,t(f.clearcoatNormalMap,m.clearcoatNormalMapTransform),m.clearcoatNormalScale.value.copy(f.clearcoatNormalScale),f.side===yn&&m.clearcoatNormalScale.value.negate())),f.dispersion>0&&(m.dispersion.value=f.dispersion),f.retroreflectivity>0&&(m.retroreflectivity.value=f.retroreflectivity),f.iridescence>0&&(m.iridescence.value=f.iridescence,m.iridescenceIOR.value=f.iridescenceIOR,m.iridescenceThicknessMinimum.value=f.iridescenceThicknessRange[0],m.iridescenceThicknessMaximum.value=f.iridescenceThicknessRange[1],f.iridescenceMap&&(m.iridescenceMap.value=f.iridescenceMap,t(f.iridescenceMap,m.iridescenceMapTransform)),f.iridescenceThicknessMap&&(m.iridescenceThicknessMap.value=f.iridescenceThicknessMap,t(f.iridescenceThicknessMap,m.iridescenceThicknessMapTransform))),f.transmission>0&&(m.transmission.value=f.transmission,m.transmissionSamplerMap.value=b.texture,m.transmissionSamplerSize.value.set(b.width,b.height),f.transmissionMap&&(m.transmissionMap.value=f.transmissionMap,t(f.transmissionMap,m.transmissionMapTransform)),m.thickness.value=f.thickness,f.thicknessMap&&(m.thicknessMap.value=f.thicknessMap,t(f.thicknessMap,m.thicknessMapTransform)),m.attenuationDistance.value=f.attenuationDistance,m.attenuationColor.value.copy(f.attenuationColor)),f.anisotropy>0&&(m.anisotropyVector.value.set(f.anisotropy*Math.cos(f.anisotropyRotation),f.anisotropy*Math.sin(f.anisotropyRotation)),f.anisotropyMap&&(m.anisotropyMap.value=f.anisotropyMap,t(f.anisotropyMap,m.anisotropyMapTransform))),m.specularIntensity.value=f.specularIntensity,m.specularColor.value.copy(f.specularColor),f.specularColorMap&&(m.specularColorMap.value=f.specularColorMap,t(f.specularColorMap,m.specularColorMapTransform)),f.specularIntensityMap&&(m.specularIntensityMap.value=f.specularIntensityMap,t(f.specularIntensityMap,m.specularIntensityMapTransform))}function x(m,f){f.matcap&&(m.matcap.value=f.matcap)}function M(m,f){let b=e.get(f).light;m.referencePosition.value.setFromMatrixPosition(b.matrixWorld),m.nearDistance.value=b.shadow.camera.near,m.farDistance.value=b.shadow.camera.far}return{refreshFogUniforms:n,refreshMaterialUniforms:i}}function L_(s,e,t,n){let i={},r={},a=[],o=s.getParameter(s.MAX_UNIFORM_BUFFER_BINDINGS);function l(g,_){let w=_.program;n.uniformBlockBinding(g,w)}function c(g,_){let w=i[g.id];w===void 0&&(m(g),w=h(g),i[g.id]=w,g.addEventListener("dispose",b));let T=_.program;n.updateUBOMapping(g,T);let y=e.render.frame;r[g.id]!==y&&(u(g),r[g.id]=y)}function h(g){let _=d();g.__bindingPointIndex=_;let w=s.createBuffer(),T=g.__size,y=g.usage;return s.bindBuffer(s.UNIFORM_BUFFER,w),s.bufferData(s.UNIFORM_BUFFER,T,y),s.bindBuffer(s.UNIFORM_BUFFER,null),s.bindBufferBase(s.UNIFORM_BUFFER,_,w),w}function d(){for(let g=0;g<o;g++)if(a.indexOf(g)===-1)return a.push(g),g;return _t("WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function u(g){let _=i[g.id],w=g.uniforms,T=g.__cache;s.bindBuffer(s.UNIFORM_BUFFER,_);for(let y=0,C=w.length;y<C;y++){let S=w[y];if(Array.isArray(S))for(let D=0,v=S.length;D<v;D++)p(S[D],y,D,T);else p(S,y,0,T)}s.bindBuffer(s.UNIFORM_BUFFER,null)}function p(g,_,w,T){if(M(g,_,w,T)===!0){let y=g.__offset,C=g.value;if(Array.isArray(C)){let S=0;for(let D=0;D<C.length;D++){let v=C[D],I=f(v);x(v,g.__data,S),typeof v!="number"&&typeof v!="boolean"&&!v.isMatrix3&&!ArrayBuffer.isView(v)&&(S+=I.storage/Float32Array.BYTES_PER_ELEMENT)}}else x(C,g.__data,0);s.bufferSubData(s.UNIFORM_BUFFER,y,g.__data)}}function x(g,_,w){typeof g=="number"||typeof g=="boolean"?_[0]=g:g.isMatrix3?(_[0]=g.elements[0],_[1]=g.elements[1],_[2]=g.elements[2],_[3]=0,_[4]=g.elements[3],_[5]=g.elements[4],_[6]=g.elements[5],_[7]=0,_[8]=g.elements[6],_[9]=g.elements[7],_[10]=g.elements[8],_[11]=0):ArrayBuffer.isView(g)?_.set(new g.constructor(g.buffer,g.byteOffset,_.length)):g.toArray(_,w)}function M(g,_,w,T){let y=g.value,C=_+"_"+w;if(T[C]===void 0)return typeof y=="number"||typeof y=="boolean"?T[C]=y:ArrayBuffer.isView(y)?T[C]=y.slice():T[C]=y.clone(),!0;{let S=T[C];if(typeof y=="number"||typeof y=="boolean"){if(S!==y)return T[C]=y,!0}else{if(ArrayBuffer.isView(y))return!0;if(S.equals(y)===!1)return S.copy(y),!0}}return!1}function m(g){let _=g.uniforms,w=0,T=16;for(let C=0,S=_.length;C<S;C++){let D=Array.isArray(_[C])?_[C]:[_[C]];for(let v=0,I=D.length;v<I;v++){let P=D[v],N=Array.isArray(P.value)?P.value:[P.value];for(let z=0,X=N.length;z<X;z++){let j=N[z],V=f(j),W=w%T,ie=W%V.boundary,H=W+ie;w+=ie,H!==0&&T-H<V.storage&&(w+=T-H),P.__data=new Float32Array(V.storage/Float32Array.BYTES_PER_ELEMENT),P.__offset=w,w+=V.storage}}}let y=w%T;return y>0&&(w+=T-y),g.__size=w,g.__cache={},this}function f(g){let _={boundary:0,storage:0};return typeof g=="number"||typeof g=="boolean"?(_.boundary=4,_.storage=4):g.isVector2?(_.boundary=8,_.storage=8):g.isVector3||g.isColor?(_.boundary=16,_.storage=12):g.isVector4?(_.boundary=16,_.storage=16):g.isMatrix3?(_.boundary=48,_.storage=48):g.isMatrix4?(_.boundary=64,_.storage=64):g.isTexture?dt("WebGLRenderer: Texture samplers can not be part of an uniforms group."):ArrayBuffer.isView(g)?(_.boundary=16,_.storage=g.byteLength):dt("WebGLRenderer: Unsupported uniform value type.",g),_}function b(g){let _=g.target;_.removeEventListener("dispose",b);let w=a.indexOf(_.__bindingPointIndex);a.splice(w,1),s.deleteBuffer(i[_.id]),delete i[_.id],delete r[_.id]}function E(){for(let g in i)s.deleteBuffer(i[g]);a=[],i={},r={}}return{bind:l,update:c,dispose:E}}var F_=new Uint16Array([12469,15057,12620,14925,13266,14620,13807,14376,14323,13990,14545,13625,14713,13328,14840,12882,14931,12528,14996,12233,15039,11829,15066,11525,15080,11295,15085,10976,15082,10705,15073,10495,13880,14564,13898,14542,13977,14430,14158,14124,14393,13732,14556,13410,14702,12996,14814,12596,14891,12291,14937,11834,14957,11489,14958,11194,14943,10803,14921,10506,14893,10278,14858,9960,14484,14039,14487,14025,14499,13941,14524,13740,14574,13468,14654,13106,14743,12678,14818,12344,14867,11893,14889,11509,14893,11180,14881,10751,14852,10428,14812,10128,14765,9754,14712,9466,14764,13480,14764,13475,14766,13440,14766,13347,14769,13070,14786,12713,14816,12387,14844,11957,14860,11549,14868,11215,14855,10751,14825,10403,14782,10044,14729,9651,14666,9352,14599,9029,14967,12835,14966,12831,14963,12804,14954,12723,14936,12564,14917,12347,14900,11958,14886,11569,14878,11247,14859,10765,14828,10401,14784,10011,14727,9600,14660,9289,14586,8893,14508,8533,15111,12234,15110,12234,15104,12216,15092,12156,15067,12010,15028,11776,14981,11500,14942,11205,14902,10752,14861,10393,14812,9991,14752,9570,14682,9252,14603,8808,14519,8445,14431,8145,15209,11449,15208,11451,15202,11451,15190,11438,15163,11384,15117,11274,15055,10979,14994,10648,14932,10343,14871,9936,14803,9532,14729,9218,14645,8742,14556,8381,14461,8020,14365,7603,15273,10603,15272,10607,15267,10619,15256,10631,15231,10614,15182,10535,15118,10389,15042,10167,14963,9787,14883,9447,14800,9115,14710,8665,14615,8318,14514,7911,14411,7507,14279,7198,15314,9675,15313,9683,15309,9712,15298,9759,15277,9797,15229,9773,15166,9668,15084,9487,14995,9274,14898,8910,14800,8539,14697,8234,14590,7790,14479,7409,14367,7067,14178,6621,15337,8619,15337,8631,15333,8677,15325,8769,15305,8871,15264,8940,15202,8909,15119,8775,15022,8565,14916,8328,14804,8009,14688,7614,14569,7287,14448,6888,14321,6483,14088,6171,15350,7402,15350,7419,15347,7480,15340,7613,15322,7804,15287,7973,15229,8057,15148,8012,15046,7846,14933,7611,14810,7357,14682,7069,14552,6656,14421,6316,14251,5948,14007,5528,15356,5942,15356,5977,15353,6119,15348,6294,15332,6551,15302,6824,15249,7044,15171,7122,15070,7050,14949,6861,14818,6611,14679,6349,14538,6067,14398,5651,14189,5311,13935,4958,15359,4123,15359,4153,15356,4296,15353,4646,15338,5160,15311,5508,15263,5829,15188,6042,15088,6094,14966,6001,14826,5796,14678,5543,14527,5287,14377,4985,14133,4586,13869,4257,15360,1563,15360,1642,15358,2076,15354,2636,15341,3350,15317,4019,15273,4429,15203,4732,15105,4911,14981,4932,14836,4818,14679,4621,14517,4386,14359,4156,14083,3795,13808,3437,15360,122,15360,137,15358,285,15355,636,15344,1274,15322,2177,15281,2765,15215,3223,15120,3451,14995,3569,14846,3567,14681,3466,14511,3305,14344,3121,14037,2800,13753,2467,15360,0,15360,1,15359,21,15355,89,15346,253,15325,479,15287,796,15225,1148,15133,1492,15008,1749,14856,1882,14685,1886,14506,1783,14324,1608,13996,1398,13702,1183]),Bi=null;function N_(){return Bi===null&&(Bi=new _s(F_,16,16,Ps,xn),Bi.name="DFG_LUT",Bi.minFilter=on,Bi.magFilter=on,Bi.wrapS=ri,Bi.wrapT=ri,Bi.generateMipmaps=!1,Bi.needsUpdate=!0),Bi}var Hc=class{constructor(e={}){let{canvas:t=up(),context:n=null,depth:i=!0,stencil:r=!1,alpha:a=!1,antialias:o=!1,premultipliedAlpha:l=!0,preserveDrawingBuffer:c=!1,powerPreference:h="default",failIfMajorPerformanceCaveat:d=!1,reversedDepthBuffer:u=!1,outputBufferType:p=Xn}=e;this.isWebGLRenderer=!0;let x;if(n!==null){if(typeof WebGLRenderingContext<"u"&&n instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");x=n.getContextAttributes().alpha}else x=a;let M=p,m=new Set([ic,nc,tc]),f=new Set([Xn,Ti,ea,ta,$l,Ql]),b=new Uint32Array(4),E=new Int32Array(4),g=new U,_=null,w=null,T=[],y=[],C=null;this.domElement=t,this.debug={checkShaderErrors:!0,diagnostics:{keywords:!1},onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this.toneMapping=Si,this.toneMappingExposure=1,this.transmissionResolutionScale=1;let S=this,D=!1,v=null,I=null,P=null,N=null;this._outputColorSpace=an;let z=0,X=0,j=null,V=-1,W=null,ie=new Ot,H=new Ot,ge=null,Ze=new Le(0),G=0,ye=t.width,J=t.height,A=1,me=null,ce=null,ne=new Ot(0,0,ye,J),K=new Ot(0,0,ye,J),oe=!1,re=new Hr,te=!1,fe=!1,Ee=new yt,Ne=new U,Oe=new Ot,ut={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},rt=!1;function at(){return j===null?A:1}let Q=n;function vt(L,se){return t.getContext(L,se)}let je,O,R,ee,ae,_e,Ue,ke,B,Z,pe,Te,ve,we,ue,Ae,ze,Y,Ce,Re,Ve,He,De;try{let L={alpha:!0,depth:i,stencil:r,antialias:o,premultipliedAlpha:l,preserveDrawingBuffer:c,powerPreference:h,failIfMajorPerformanceCaveat:d};if("setAttribute"in t&&t.setAttribute("data-engine",`three.js r${"186"}`),t.addEventListener("webglcontextlost",Fe,!1),t.addEventListener("webglcontextrestored",Je,!1),t.addEventListener("webglcontextcreationerror",F,!1),Q===null){let se="webgl2";if(Q=vt(se,L),Q===null)throw vt(se)?new Error("THREE.WebGLRenderer: Error creating WebGL context with your selected attributes."):new Error("THREE.WebGLRenderer: Error creating WebGL context.")}Ye()}catch(L){throw t.removeEventListener("webglcontextlost",Fe,!1),t.removeEventListener("webglcontextrestored",Je,!1),t.removeEventListener("webglcontextcreationerror",F,!1),_t("WebGLRenderer: "+L.message),L}function Ye(){je=new Gv(Q),je.init(),Ve=new R_(Q,je),O=new Dv(Q,je,e,Ve),R=new T_(Q,je),O.reversedDepthBuffer&&u&&R.buffers.depth.setReversed(!0),I=Q.createFramebuffer(),P=Q.createFramebuffer(),N=Q.createFramebuffer(),ee=new qv(Q),ae=new d_,_e=new A_(Q,je,R,ae,O,Ve,ee),Ue=new Hv(S),ke=new j0(Q),He=new Pv(Q,ke),B=new Vv(Q,ke,ee,He),Z=new jv(Q,B,ke,He,ee),Y=new Xv(Q,O,_e),ue=new Lv(ae),pe=new u_(S,Ue,je,O,He,ue),Te=new D_(S,ae),ve=new p_,we=new y_(je),ze=new Cv(S,Ue,R,Z,x,l),Ae=new E_(S,Z,O),De=new L_(Q,ee,O,R),Ce=new Iv(Q,je,ee),Re=new Wv(Q,je,ee),ee.programs=pe.programs,S.capabilities=O,S.extensions=je,S.properties=ae,S.renderLists=ve,S.shadowMap=Ae,S.state=R,S.info=ee}M!==Xn&&(C=new Yv(M,t.width,t.height,o,i,r));let et=new Uu(S,Q);this.xr=et,this.getContext=function(){return Q},this.getContextAttributes=function(){return Q.getContextAttributes()},this.forceContextLoss=function(){let L=je.get("WEBGL_lose_context");L&&L.loseContext()},this.forceContextRestore=function(){let L=je.get("WEBGL_lose_context");L&&L.restoreContext()},this.getPixelRatio=function(){return A},this.setPixelRatio=function(L){L!==void 0&&(A=L,this.setSize(ye,J,!1))},this.getSize=function(L){return L.set(ye,J)},this.setSize=function(L,se,Se=!0){if(et.isPresenting){dt("WebGLRenderer: Can't change size while VR device is presenting.");return}ye=L,J=se,t.width=Math.floor(L*A),t.height=Math.floor(se*A),Se===!0&&(t.style.width=L+"px",t.style.height=se+"px"),C!==null&&C.setSize(t.width,t.height),this.setViewport(0,0,L,se)},this.getDrawingBufferSize=function(L){return L.set(ye*A,J*A).floor()},this.setDrawingBufferSize=function(L,se,Se){ye=L,J=se,A=Se,t.width=Math.floor(L*Se),t.height=Math.floor(se*Se),this.setViewport(0,0,L,se)},this.setEffects=function(L){if(M===Xn){_t("WebGLRenderer: setEffects() requires outputBufferType set to HalfFloatType or FloatType.");return}if(L){for(let se=0;se<L.length;se++)if(L[se].isOutputPass===!0){dt("WebGLRenderer: OutputPass is not needed in setEffects(). Tone mapping and color space conversion are applied automatically.");break}}C.setEffects(L||[])},this.getCurrentViewport=function(L){return L.copy(ie)},this.getViewport=function(L){return L.copy(ne)},this.setViewport=function(L,se,Se,xe){L.isVector4?ne.set(L.x,L.y,L.z,L.w):ne.set(L,se,Se,xe),R.viewport(ie.copy(ne).multiplyScalar(A).round())},this.getScissor=function(L){return L.copy(K)},this.setScissor=function(L,se,Se,xe){L.isVector4?K.set(L.x,L.y,L.z,L.w):K.set(L,se,Se,xe),R.scissor(H.copy(K).multiplyScalar(A).round())},this.getScissorTest=function(){return oe},this.setScissorTest=function(L){R.setScissorTest(oe=L)},this.setOpaqueSort=function(L){me=L},this.setTransparentSort=function(L){ce=L},this.getClearColor=function(L){return L.copy(ze.getClearColor())},this.setClearColor=function(){ze.setClearColor(...arguments)},this.getClearAlpha=function(){return ze.getClearAlpha()},this.setClearAlpha=function(){ze.setClearAlpha(...arguments)},this.clear=function(L=!0,se=!0,Se=!0){let xe=0;if(L){let be=!1;if(j!==null){let $e=j.texture.format;be=m.has($e)}if(be){let $e=j.texture.type,tt=f.has($e),Ke=ze.getClearColor(),ot=ze.getClearAlpha(),ct=Ke.r,Ct=Ke.g,Nt=Ke.b;tt?(b[0]=ct,b[1]=Ct,b[2]=Nt,b[3]=ot,Q.clearBufferuiv(Q.COLOR,0,b)):(E[0]=ct,E[1]=Ct,E[2]=Nt,E[3]=ot,Q.clearBufferiv(Q.COLOR,0,E))}else xe|=Q.COLOR_BUFFER_BIT}se&&(xe|=Q.DEPTH_BUFFER_BIT,this.state.buffers.depth.setMask(!0)),Se&&(xe|=Q.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),xe!==0&&Q.clear(xe)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.setNodesHandler=function(L){L.setRenderer(this),v=L},this.dispose=function(){t.removeEventListener("webglcontextlost",Fe,!1),t.removeEventListener("webglcontextrestored",Je,!1),t.removeEventListener("webglcontextcreationerror",F,!1),ze.dispose(),ve.dispose(),we.dispose(),ae.dispose(),Ue.dispose(),Z.dispose(),He.dispose(),De.dispose(),pe.dispose(),et.dispose(),et.removeEventListener("sessionstart",le),et.removeEventListener("sessionend",qe),Xe.stop()};function Fe(L){L.preventDefault(),Fa("WebGLRenderer: Context Lost."),D=!0}function Je(){Fa("WebGLRenderer: Context Restored."),D=!1;let L=ee.autoReset,se=Ae.enabled,Se=Ae.autoUpdate,xe=Ae.needsUpdate,be=Ae.type;Ye(),ee.autoReset=L,Ae.enabled=se,Ae.autoUpdate=Se,Ae.needsUpdate=xe,Ae.type=be}function F(L){_t("WebGLRenderer: A WebGL context could not be created. Reason: ",L.statusMessage)}function q(L){let se=L.target;se.removeEventListener("dispose",q),$(se)}function $(L){de(L),ae.remove(L)}function de(L){let se=ae.get(L).programs;se!==void 0&&(se.forEach(function(Se){pe.releaseProgram(Se)}),L.isShaderMaterial&&pe.releaseShaderCache(L))}this.renderBufferDirect=function(L,se,Se,xe,be,$e){se===null&&(se=ut);let tt=be.isMesh&&be.matrixWorld.determinantAffine()<0,Ke=En(L,se,Se,xe,be);R.setMaterial(xe,tt);let ot=Se.index,ct=1;if(xe.wireframe===!0){if(ot=B.getWireframeAttribute(Se),ot===void 0)return;ct=2}let Ct=Se.drawRange,Nt=Se.attributes.position,lt=Ct.start*ct,Ht=(Ct.start+Ct.count)*ct;$e!==null&&(lt=Math.max(lt,$e.start*ct),Ht=Math.min(Ht,($e.start+$e.count)*ct)),ot!==null?(lt=Math.max(lt,0),Ht=Math.min(Ht,ot.count)):Nt!=null&&(lt=Math.max(lt,0),Ht=Math.min(Ht,Nt.count));let k=Ht-lt;if(k<0||k===1/0)return;He.setup(be,xe,Ke,Se,ot);let Ie,We=Ce;if(ot!==null&&(Ie=ke.get(ot),We=Re,We.setIndex(Ie)),be.isMesh)xe.wireframe===!0?(R.setLineWidth(xe.wireframeLinewidth*at()),We.setMode(Q.LINES)):We.setMode(Q.TRIANGLES);else if(be.isLine){let ft=xe.linewidth;ft===void 0&&(ft=1),R.setLineWidth(ft*at()),be.isLineSegments?We.setMode(Q.LINES):be.isLineLoop?We.setMode(Q.LINE_LOOP):We.setMode(Q.LINE_STRIP)}else be.isPoints?We.setMode(Q.POINTS):be.isSprite&&We.setMode(Q.TRIANGLES);if(be.isBatchedMesh)if(je.get("WEBGL_multi_draw"))We.renderMultiDraw(be._multiDrawStarts,be._multiDrawCounts,be._multiDrawCount);else{let ft=be._multiDrawStarts,Be=be._multiDrawCounts,Vt=be._multiDrawCount,gt=ot?ke.get(ot).bytesPerElement:1,wt=ae.get(xe).currentProgram.getUniforms();for(let pt=0;pt<Vt;pt++)wt.setValue(Q,"_gl_DrawID",pt),We.render(ft[pt]/gt,Be[pt])}else if(be.isInstancedMesh)We.renderInstances(lt,k,be.count);else if(Se.isInstancedBufferGeometry){let ft=Se._maxInstanceCount!==void 0?Se._maxInstanceCount:1/0,Be=Math.min(Se.instanceCount,ft);We.renderInstances(lt,k,Be)}else We.render(lt,k)};function Me(L,se,Se,xe){v!==null&&L.isNodeMaterial&&v.setObject(xe,L),te===!0&&ue.setState(L,Se,!1),L.transparent===!0&&L.side===Kt&&L.forceSinglePass===!1?(L.side=yn,L.needsUpdate=!0,Kn(L,se,xe),L.side=oi,L.needsUpdate=!0,Kn(L,se,xe),L.side=Kt):Kn(L,se,xe)}this.compile=function(L,se,Se=null){Se===null&&(Se=L),v!==null&&v.renderStart(L,se,Se),w=we.get(Se),w.init(se),y.push(w),Se.traverseVisible(function(be){be.isLight&&be.layers.test(se.layers)&&(w.pushLight(be),be.castShadow&&w.pushShadow(be))}),L!==Se&&L.traverseVisible(function(be){be.isLight&&be.layers.test(se.layers)&&(w.pushLight(be),be.castShadow&&w.pushShadow(be))}),w.setupLights(),v!==null&&v.updateLights(w.state.lightsArray),fe=this.localClippingEnabled,te=ue.init(this.clippingPlanes,fe),te===!0&&ue.setGlobalState(this.clippingPlanes,se),v!==null&&Ae.render(w.state.shadowsArray,Se,se);let xe=new Set;return L.traverse(function(be){if(!(be.isMesh||be.isPoints||be.isLine||be.isSprite))return;let $e=be.material;if($e)if(Array.isArray($e))for(let tt=0;tt<$e.length;tt++){let Ke=$e[tt];Me(Ke,Se,se,be),xe.add(Ke)}else Me($e,Se,se,be),xe.add($e)}),w=y.pop(),v!==null&&v.renderEnd(),xe},this.compileAsync=function(L,se,Se=null){let xe=this.compile(L,se,Se);return new Promise(be=>{function $e(){if(xe.forEach(function(tt){let ot=ae.get(tt).currentProgram;(ot===void 0||ot.isReady())&&xe.delete(tt)}),xe.size===0){be(L);return}setTimeout($e,10)}je.get("KHR_parallel_shader_compile")!==null?$e():setTimeout($e,10)})};let he=null;function Pe(L){he&&he(L)}function le(){Xe.stop()}function qe(){Xe.start()}let Xe=new kp;Xe.setAnimationLoop(Pe),typeof self<"u"&&Xe.setContext(self),this.setAnimationLoop=function(L){he=L,et.setAnimationLoop(L),L===null?Xe.stop():Xe.start()},et.addEventListener("sessionstart",le),et.addEventListener("sessionend",qe),this.render=function(L,se){if(se!==void 0&&se.isCamera!==!0){_t("WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(D===!0)return;v!==null&&v.renderStart(L,se);let Se=et.enabled===!0&&et.isPresenting===!0,xe=C!==null&&(j===null||Se)&&C.begin(S,j);if(L.matrixWorldAutoUpdate===!0&&L.updateMatrixWorld(),se.parent===null&&se.matrixWorldAutoUpdate===!0&&se.updateMatrixWorld(),et.enabled===!0&&et.isPresenting===!0&&(C===null||C.isCompositing()===!1)&&(et.cameraAutoUpdate===!0&&et.updateCamera(se),se=et.getCamera()),L.isScene===!0&&L.onBeforeRender(S,L,se,j),w=we.get(L,y.length),w.init(se),w.state.textureUnits=_e.getTextureUnits(),y.push(w),Ee.multiplyMatrices(se.projectionMatrix,se.matrixWorldInverse),re.setFromProjectionMatrix(Ee,bi,se.reversedDepth),fe=this.localClippingEnabled,te=ue.init(this.clippingPlanes,fe),_=ve.get(L,T.length),_.init(),T.push(_),et.enabled===!0&&et.isPresenting===!0){let tt=S.xr.getDepthSensingMesh();tt!==null&&it(tt,se,-1/0,S.sortObjects)}it(L,se,0,S.sortObjects),_.finish(),v!==null&&v.updateLights(w.state.lightsArray),S.sortObjects===!0&&_.sort(me,ce),rt=et.enabled===!1||et.isPresenting===!1||et.hasDepthSensing()===!1,rt&&ze.addToRenderList(_,L),this.info.render.frame++,this.info.autoReset===!0&&this.info.reset(),te===!0&&ue.beginShadows();let be=w.state.shadowsArray;if(Ae.render(be,L,se),te===!0&&ue.endShadows(),(xe&&C.hasRenderPass())===!1){let tt=_.opaque,Ke=_.transmissive;if(w.setupLights(),se.isArrayCamera){let ot=se.cameras;if(Ke.length>0)for(let ct=0,Ct=ot.length;ct<Ct;ct++){let Nt=ot[ct];Bt(tt,Ke,L,Nt)}rt&&ze.render(L);for(let ct=0,Ct=ot.length;ct<Ct;ct++){let Nt=ot[ct];zt(_,L,Nt,Nt.viewport)}}else Ke.length>0&&Bt(tt,Ke,L,se),rt&&ze.render(L),zt(_,L,se)}j!==null&&X===0&&(_e.updateMultisampleRenderTarget(j),_e.updateRenderTargetMipmap(j)),xe&&C.end(S),L.isScene===!0&&L.onAfterRender(S,L,se),He.resetDefaultState(),V=-1,W=null,y.pop(),y.length>0?(w=y[y.length-1],_e.setTextureUnits(w.state.textureUnits),te===!0&&ue.setGlobalState(S.clippingPlanes,w.state.camera)):w=null,T.pop(),T.length>0?_=T[T.length-1]:_=null,v!==null&&v.renderEnd()};function it(L,se,Se,xe){if(L.visible===!1)return;if(L.layers.test(se.layers)){if(L.isGroup)Se=L.renderOrder;else if(L.isLOD)L.autoUpdate===!0&&L.update(se);else if(L.isLightProbeGrid)w.pushLightProbeGrid(L);else if(L.isLight)w.pushLight(L),L.castShadow&&w.pushShadow(L);else if(L.isSprite){if(!L.frustumCulled||L.intersectsFrustum(re)){xe&&Oe.setFromMatrixPosition(L.matrixWorld).applyMatrix4(Ee);let tt=Z.update(L),Ke=L.material;Ke.visible&&_.push(L,tt,Ke,Se,Oe.z,null,se)}}else if((L.isMesh||L.isLine||L.isPoints)&&(!L.frustumCulled||L.intersectsFrustum(re))){let tt=Z.update(L),Ke=L.material;if(xe&&(L.boundingSphere!==void 0?(L.boundingSphere===null&&L.computeBoundingSphere(),Oe.copy(L.boundingSphere.center)):(tt.boundingSphere===null&&tt.computeBoundingSphere(),Oe.copy(tt.boundingSphere.center)),Oe.applyMatrix4(L.matrixWorld).applyMatrix4(Ee)),Array.isArray(Ke)){let ot=tt.groups;for(let ct=0,Ct=ot.length;ct<Ct;ct++){let Nt=ot[ct],lt=Ke[Nt.materialIndex];lt&&lt.visible&&_.push(L,tt,lt,Se,Oe.z,Nt,se)}}else Ke.visible&&_.push(L,tt,Ke,Se,Oe.z,null,se)}}let $e=L.children;for(let tt=0,Ke=$e.length;tt<Ke;tt++)it($e[tt],se,Se,xe)}function zt(L,se,Se,xe){let{opaque:be,transmissive:$e,transparent:tt}=L;w.setupLightsView(Se),te===!0&&ue.setGlobalState(S.clippingPlanes,Se),xe&&R.viewport(ie.copy(xe)),be.length>0&&pn(be,se,Se),$e.length>0&&pn($e,se,Se),tt.length>0&&pn(tt,se,Se),R.buffers.depth.setTest(!0),R.buffers.depth.setMask(!0),R.buffers.color.setMask(!0),R.setPolygonOffset(!1)}function Bt(L,se,Se,xe){if((Se.isScene===!0?Se.overrideMaterial:null)!==null)return;if(w.state.transmissionRenderTarget[xe.id]===void 0){let lt=je.has("EXT_color_buffer_half_float")||je.has("EXT_color_buffer_float");w.state.transmissionRenderTarget[xe.id]=new ln(1,1,{generateMipmaps:!0,type:lt?xn:Xn,minFilter:Ei,samples:Math.max(4,O.samples),stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,storeMultisampledDepthBuffer:!1,storeMultisampledStencilBuffer:!1,colorSpace:Dt.workingColorSpace})}let $e=w.state.transmissionRenderTarget[xe.id],tt=xe.viewport||ie;$e.setSize(tt.z*S.transmissionResolutionScale,tt.w*S.transmissionResolutionScale);let Ke=S.getRenderTarget(),ot=S.getActiveCubeFace(),ct=S.getActiveMipmapLevel();S.setRenderTarget($e),S.getClearColor(Ze),G=S.getClearAlpha(),G<1&&S.setClearColor(16777215,.5),S.clear(),rt&&ze.render(Se);let Ct=S.toneMapping;S.toneMapping=Si;let Nt=xe.viewport;if(xe.viewport!==void 0&&(xe.viewport=void 0),w.setupLightsView(xe),te===!0&&ue.setGlobalState(S.clippingPlanes,xe),pn(L,Se,xe),_e.updateMultisampleRenderTarget($e),_e.updateRenderTargetMipmap($e),je.has("WEBGL_multisampled_render_to_texture")===!1){let lt=!1;for(let Ht=0,k=se.length;Ht<k;Ht++){let Ie=se[Ht],{object:We,geometry:ft,material:Be,group:Vt}=Ie;if(Be.side===Kt&&We.layers.test(xe.layers)){let gt=Be.side;Be.side=yn,Be.needsUpdate=!0,ii(We,Se,xe,ft,Be,Vt),Be.side=gt,Be.needsUpdate=!0,lt=!0}}lt===!0&&(_e.updateMultisampleRenderTarget($e),_e.updateRenderTargetMipmap($e))}S.setRenderTarget(Ke,ot,ct),S.setClearColor(Ze,G),Nt!==void 0&&(xe.viewport=Nt),S.toneMapping=Ct}function pn(L,se,Se){let xe=se.isScene===!0?se.overrideMaterial:null;for(let be=0,$e=L.length;be<$e;be++){let tt=L[be],{object:Ke,geometry:ot,group:ct}=tt,Ct=tt.material;Ct.allowOverride===!0&&xe!==null&&(Ct=xe),Ke.layers.test(Se.layers)&&ii(Ke,se,Se,ot,Ct,ct)}}function ii(L,se,Se,xe,be,$e){v!==null&&be.isNodeMaterial&&v.setObject(L,be),L.onBeforeRender(S,se,Se,xe,be,$e),L.modelViewMatrix.multiplyMatrices(Se.matrixWorldInverse,L.matrixWorld),L.normalMatrix.getNormalMatrix(L.modelViewMatrix),be.onBeforeRender(S,se,Se,xe,L,$e),be.transparent===!0&&be.side===Kt&&be.forceSinglePass===!1?(be.side=yn,be.needsUpdate=!0,S.renderBufferDirect(Se,se,xe,be,L,$e),be.side=oi,be.needsUpdate=!0,S.renderBufferDirect(Se,se,xe,be,L,$e),be.side=Kt):S.renderBufferDirect(Se,se,xe,be,L,$e),L.onAfterRender(S,se,Se,xe,be,$e)}function Kn(L,se,Se){se.isScene!==!0&&(se=ut);let xe=ae.get(L),be=w.state.lights,$e=w.state.shadowsArray,tt=be.state.version,Ke=pe.getParameters(L,be.state,$e,se,Se,w.state.lightProbeGridArray),ot=pe.getProgramCacheKey(Ke),ct=xe.programs;xe.environment=L.isMeshStandardMaterial||L.isMeshLambertMaterial||L.isMeshPhongMaterial?se.environment:null,xe.fog=se.fog;let Ct=L.isMeshStandardMaterial||L.isMeshLambertMaterial&&!L.envMap||L.isMeshPhongMaterial&&!L.envMap;xe.envMap=Ue.get(L.envMap||xe.environment,Ct),xe.envMapRotation=xe.environment!==null&&L.envMap===null?se.environmentRotation:L.envMapRotation,ct===void 0&&(L.addEventListener("dispose",q),ct=new Map,xe.programs=ct);let Nt=ct.get(ot);if(Nt!==void 0){if(xe.currentProgram===Nt&&xe.lightsStateVersion===tt)return en(L,Ke),Nt}else Ke.uniforms=pe.getUniforms(L),v!==null&&L.isNodeMaterial&&v.build(L,Se,Ke),L.onBeforeCompile(Ke,S),Nt=pe.acquireProgram(Ke,ot),ct.set(ot,Nt),xe.uniforms=Ke.uniforms;let lt=xe.uniforms;return(!L.isShaderMaterial&&!L.isRawShaderMaterial||L.clipping===!0)&&(lt.clippingPlanes=ue.uniform),en(L,Ke),xe.needsLights=kn(L),xe.lightsStateVersion=tt,xe.needsLights&&(lt.ambientLightColor.value=be.state.ambient,lt.lightProbe.value=be.state.probe,lt.sunLights.value=be.state.sun,lt.sunLightShadows.value=be.state.sunShadow,lt.directionalLights.value=be.state.directional,lt.directionalLightShadows.value=be.state.directionalShadow,lt.spotLights.value=be.state.spot,lt.spotLightShadows.value=be.state.spotShadow,lt.rectAreaLights.value=be.state.rectArea,lt.ltc_1.value=be.state.rectAreaLTC1,lt.ltc_2.value=be.state.rectAreaLTC2,lt.pointLights.value=be.state.point,lt.pointLightShadows.value=be.state.pointShadow,lt.hemisphereLights.value=be.state.hemi,lt.sunShadowMatrix.value=be.state.sunShadowMatrix,lt.sunShadowCascade.value=be.state.sunShadowCascade,lt.directionalShadowMatrix.value=be.state.directionalShadowMatrix,lt.spotLightMatrix.value=be.state.spotLightMatrix,lt.spotLightMap.value=be.state.spotLightMap,lt.pointShadowMatrix.value=be.state.pointShadowMatrix),xe.lightProbeGrid=w.state.lightProbeGridArray.length>0,xe.currentProgram=Nt,xe.uniformsList=null,Nt}function Zt(L){if(L.uniformsList===null){let se=L.currentProgram.getUniforms();L.uniformsList=ra.seqWithValue(se.seq,L.uniforms)}return L.uniformsList}function en(L,se){let Se=ae.get(L);Se.outputColorSpace=se.outputColorSpace,Se.batching=se.batching,Se.batchingColor=se.batchingColor,Se.instancing=se.instancing,Se.instancingColor=se.instancingColor,Se.instancingMorph=se.instancingMorph,Se.skinning=se.skinning,Se.morphTargets=se.morphTargets,Se.morphNormals=se.morphNormals,Se.morphColors=se.morphColors,Se.morphTargetsCount=se.morphTargetsCount,Se.numClippingPlanes=se.numClippingPlanes,Se.numIntersection=se.numClipIntersection,Se.vertexAlphas=se.vertexAlphas,Se.vertexTangents=se.vertexTangents,Se.toneMapping=se.toneMapping}function Pn(L,se){if(L.length===0)return null;if(L.length===1)return L[0].texture!==null?L[0]:null;g.setFromMatrixPosition(se.matrixWorld);for(let Se=0,xe=L.length;Se<xe;Se++){let be=L[Se];if(be.texture!==null&&be.boundingBox.containsPoint(g))return be}return null}function En(L,se,Se,xe,be){se.isScene!==!0&&(se=ut),_e.resetTextureUnits();let $e=se.fog,tt=xe.isMeshStandardMaterial||xe.isMeshLambertMaterial||xe.isMeshPhongMaterial?se.environment:null,Ke=j===null?S.outputColorSpace:j.isXRRenderTarget===!0?j.texture.colorSpace:Dt.workingColorSpace,ot=xe.isMeshStandardMaterial||xe.isMeshLambertMaterial&&!xe.envMap||xe.isMeshPhongMaterial&&!xe.envMap,ct=Ue.get(xe.envMap||tt,ot),Ct=xe.vertexColors===!0&&!!Se.attributes.color&&Se.attributes.color.itemSize===4,Nt=!!Se.attributes.tangent&&(!!xe.normalMap||xe.anisotropy>0),lt=!!Se.morphAttributes.position,Ht=!!Se.morphAttributes.normal,k=!!Se.morphAttributes.color,Ie=Si;xe.toneMapped&&(j===null||j.isXRRenderTarget===!0)&&(Ie=S.toneMapping);let We=Se.morphAttributes.position||Se.morphAttributes.normal||Se.morphAttributes.color,ft=We!==void 0?We.length:0,Be=ae.get(xe),Vt=w.state.lights;if(te===!0&&(fe===!0||L!==W)){let St=L===W&&xe.id===V;ue.setState(xe,L,St)}let gt=!1;xe.version===Be.__version?(Be.needsLights&&Be.lightsStateVersion!==Vt.state.version||Be.outputColorSpace!==Ke||be.isBatchedMesh&&Be.batching===!1||!be.isBatchedMesh&&Be.batching===!0||be.isBatchedMesh&&Be.batchingColor===!0&&be._colorsTexture===null||be.isBatchedMesh&&Be.batchingColor===!1&&be._colorsTexture!==null||be.isInstancedMesh&&Be.instancing===!1||!be.isInstancedMesh&&Be.instancing===!0||be.isSkinnedMesh&&Be.skinning===!1||!be.isSkinnedMesh&&Be.skinning===!0||be.isInstancedMesh&&Be.instancingColor===!0&&be.instanceColor===null||be.isInstancedMesh&&Be.instancingColor===!1&&be.instanceColor!==null||be.isInstancedMesh&&Be.instancingMorph===!0&&be.morphTexture===null||be.isInstancedMesh&&Be.instancingMorph===!1&&be.morphTexture!==null||Be.envMap!==ct||xe.fog===!0&&Be.fog!==$e||Be.numClippingPlanes!==void 0&&(Be.numClippingPlanes!==ue.numPlanes||Be.numIntersection!==ue.numIntersection)||Be.vertexAlphas!==Ct||Be.vertexTangents!==Nt||Be.morphTargets!==lt||Be.morphNormals!==Ht||Be.morphColors!==k||Be.toneMapping!==Ie||Be.morphTargetsCount!==ft||!!Be.lightProbeGrid!=w.state.lightProbeGridArray.length>0)&&(gt=!0):(gt=!0,Be.__version=xe.version);let wt=Be.currentProgram;gt===!0&&(wt=Kn(xe,se,be),v&&xe.isNodeMaterial&&v.onUpdateProgram(xe,wt,Be));let pt=!1,mt=!1,Ut=!1,nt=wt.getUniforms(),xt=Be.uniforms;if(R.useProgram(wt.program)&&(pt=!0,mt=!0,Ut=!0),xe.id!==V&&(V=xe.id,mt=!0),Be.needsLights){let St=Pn(w.state.lightProbeGridArray,be);Be.lightProbeGrid!==St&&(Be.lightProbeGrid=St,mt=!0)}if(pt||W!==L){R.buffers.depth.getReversed()&&L.reversedDepth!==!0&&(L._reversedDepth=!0,L.updateProjectionMatrix()),nt.setValue(Q,"projectionMatrix",L.projectionMatrix),nt.setValue(Q,"viewMatrix",L.matrixWorldInverse);let st=nt.map.cameraPosition;st!==void 0&&st.setValue(Q,Ne.setFromMatrixPosition(L.matrixWorld)),O.logarithmicDepthBuffer&&nt.setValue(Q,"logDepthBufFC",2/(Math.log(L.far+1)/Math.LN2)),(xe.isMeshPhongMaterial||xe.isMeshToonMaterial||xe.isMeshLambertMaterial||xe.isMeshBasicMaterial||xe.isMeshStandardMaterial||xe.isShaderMaterial)&&nt.setValue(Q,"isOrthographic",L.isOrthographicCamera===!0),W!==L&&(W=L,mt=!0,Ut=!0)}if(Be.needsLights&&(Vt.state.sunShadowMap.length>0&&nt.setValue(Q,"sunShadowMap",Vt.state.sunShadowMap,_e),Vt.state.directionalShadowMap.length>0&&nt.setValue(Q,"directionalShadowMap",Vt.state.directionalShadowMap,_e),Vt.state.spotShadowMap.length>0&&nt.setValue(Q,"spotShadowMap",Vt.state.spotShadowMap,_e),Vt.state.pointShadowMap.length>0&&nt.setValue(Q,"pointShadowMap",Vt.state.pointShadowMap,_e)),be.isSkinnedMesh){nt.setOptional(Q,be,"bindMatrix"),nt.setOptional(Q,be,"bindMatrixInverse");let St=be.skeleton;St&&(St.boneTexture===null&&St.computeBoneTexture(),nt.setValue(Q,"boneTexture",St.boneTexture,_e))}be.isBatchedMesh&&(nt.setOptional(Q,be,"batchingTexture"),nt.setValue(Q,"batchingTexture",be._matricesTexture,_e),nt.setOptional(Q,be,"batchingIdTexture"),nt.setValue(Q,"batchingIdTexture",be._indirectTexture,_e),nt.setOptional(Q,be,"batchingColorTexture"),be._colorsTexture!==null&&nt.setValue(Q,"batchingColorTexture",be._colorsTexture,_e));let sn=Se.morphAttributes;if((sn.position!==void 0||sn.normal!==void 0||sn.color!==void 0)&&Y.update(be,Se,wt),(mt||Be.receiveShadow!==be.receiveShadow)&&(Be.receiveShadow=be.receiveShadow,nt.setValue(Q,"receiveShadow",be.receiveShadow)),(xe.isMeshStandardMaterial||xe.isMeshLambertMaterial||xe.isMeshPhongMaterial)&&xe.envMap===null&&se.environment!==null&&(xt.envMapIntensity.value=se.environmentIntensity),xt.dfgLUT!==void 0&&(xt.dfgLUT.value=N_()),mt){if(nt.setValue(Q,"toneMappingExposure",S.toneMappingExposure),Be.needsLights&&Ln(xt,Ut),$e&&xe.fog===!0&&Te.refreshFogUniforms(xt,$e),Te.refreshMaterialUniforms(xt,xe,A,J,w.state.transmissionRenderTarget[L.id]),Be.needsLights&&Be.lightProbeGrid){let St=Be.lightProbeGrid;xt.probesSH.value=St.texture,xt.probesMin.value.copy(St.boundingBox.min),xt.probesMax.value.copy(St.boundingBox.max),xt.probesResolution.value.copy(St.resolution)}ra.upload(Q,Zt(Be),xt,_e)}if(xe.isShaderMaterial&&xe.uniformsNeedUpdate===!0&&(ra.upload(Q,Zt(Be),xt,_e),xe.uniformsNeedUpdate=!1),xe.isSpriteMaterial&&nt.setValue(Q,"center",be.center),nt.setValue(Q,"modelViewMatrix",be.modelViewMatrix),nt.setValue(Q,"normalMatrix",be.normalMatrix),nt.setValue(Q,"modelMatrix",be.matrixWorld),xe.uniformsGroups!==void 0){let St=xe.uniformsGroups;for(let st=0,bn=St.length;st<bn;st++){let ji=St[st];De.update(ji,wt),De.bind(ji,wt)}}return wt}function Ln(L,se){L.ambientLightColor.needsUpdate=se,L.lightProbe.needsUpdate=se,L.sunLights.needsUpdate=se,L.sunLightShadows.needsUpdate=se,L.directionalLights.needsUpdate=se,L.directionalLightShadows.needsUpdate=se,L.pointLights.needsUpdate=se,L.pointLightShadows.needsUpdate=se,L.spotLights.needsUpdate=se,L.spotLightShadows.needsUpdate=se,L.rectAreaLights.needsUpdate=se,L.hemisphereLights.needsUpdate=se}function kn(L){return L.isMeshLambertMaterial||L.isMeshToonMaterial||L.isMeshPhongMaterial||L.isMeshStandardMaterial||L.isShadowMaterial||L.isShaderMaterial&&L.lights===!0}this.getActiveCubeFace=function(){return z},this.getActiveMipmapLevel=function(){return X},this.getRenderTarget=function(){return j},this.setRenderTargetTextures=function(L,se,Se){let xe=ae.get(L);xe.__autoAllocateDepthBuffer=L.resolveDepthBuffer===!1,xe.__autoAllocateDepthBuffer===!1&&(xe.__useRenderToTexture=!1),ae.get(L.texture).__webglTexture=se,ae.get(L.depthTexture).__webglTexture=xe.__autoAllocateDepthBuffer?void 0:Se,xe.__hasExternalTextures=!0},this.setRenderTargetFramebuffer=function(L,se){let Se=ae.get(L);Se.__webglFramebuffer=se,Se.__useDefaultFramebuffer=se===void 0},this.setRenderTarget=function(L,se=0,Se=0){j=L,z=se,X=Se;let xe=null,be=!1,$e=!1;if(L){let Ke=ae.get(L);if(Ke.__useDefaultFramebuffer!==void 0){R.bindFramebuffer(Q.FRAMEBUFFER,Ke.__webglFramebuffer),ie.copy(L.viewport),H.copy(L.scissor),ge=L.scissorTest,R.viewport(ie),R.scissor(H),R.setScissorTest(ge),V=-1;return}else if(Ke.__webglFramebuffer===void 0)_e.setupRenderTarget(L);else if(Ke.__hasExternalTextures)_e.rebindTextures(L,ae.get(L.texture).__webglTexture,ae.get(L.depthTexture).__webglTexture);else if(L.depthBuffer){let Ct=L.depthTexture;if(Ke.__boundDepthTexture!==Ct){if(Ct!==null&&ae.has(Ct)&&(L.width!==Ct.image.width||L.height!==Ct.image.height))throw new Error("THREE.WebGLRenderer: Attached DepthTexture is initialized to the incorrect size.");_e.setupDepthRenderbuffer(L)}}let ot=L.texture;(ot.isData3DTexture||ot.isDataArrayTexture||ot.isCompressedArrayTexture)&&($e=!0);let ct=ae.get(L).__webglFramebuffer;L.isWebGLCubeRenderTarget?(Array.isArray(ct[se])?xe=ct[se][Se]:xe=ct[se],be=!0):L.samples>0&&_e.useMultisampledRTT(L)===!1?xe=ae.get(L).__webglMultisampledFramebuffer:Array.isArray(ct)?xe=ct[Se]:xe=ct,ie.copy(L.viewport),H.copy(L.scissor),ge=L.scissorTest}else ie.copy(ne).multiplyScalar(A).floor(),H.copy(K).multiplyScalar(A).floor(),ge=oe;if(Se!==0&&(xe=I),R.bindFramebuffer(Q.FRAMEBUFFER,xe)&&R.drawBuffers(L,xe),R.viewport(ie),R.scissor(H),R.setScissorTest(ge),be){let Ke=ae.get(L.texture);Q.framebufferTexture2D(Q.FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_CUBE_MAP_POSITIVE_X+se,Ke.__webglTexture,Se)}else if($e){let Ke=se;for(let ot=0;ot<L.textures.length;ot++){let ct=ae.get(L.textures[ot]);Q.framebufferTextureLayer(Q.FRAMEBUFFER,Q.COLOR_ATTACHMENT0+ot,ct.__webglTexture,Se,Ke)}}else if(L!==null&&Se!==0){let Ke=ae.get(L.texture);Q.framebufferTexture2D(Q.FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_2D,Ke.__webglTexture,Se)}V=-1};function pr(L){let se=ae.get(L);return(se.__readFormat!==L.format||se.__readType!==L.type)&&(se.__readFormat=L.format,se.__readType=L.type,se.__formatReadable=O.textureFormatReadable(L.format),se.__typeReadable=O.textureTypeReadable(L.type)),se}this.readRenderTargetPixels=function(L,se,Se,xe,be,$e,tt,Ke=0){if(!(L&&L.isWebGLRenderTarget)){_t("WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let ot=ae.get(L).__webglFramebuffer;if(L.isWebGLCubeRenderTarget&&tt!==void 0&&(ot=ot[tt]),ot){R.bindFramebuffer(Q.FRAMEBUFFER,ot);try{let ct=L.textures[Ke],Ct=ct.format,Nt=ct.type;L.textures.length>1&&Q.readBuffer(Q.COLOR_ATTACHMENT0+Ke);let lt=pr(ct);if(lt.__formatReadable===!1){_t("WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if(lt.__typeReadable===!1){_t("WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}se>=0&&se<=L.width-xe&&Se>=0&&Se<=L.height-be&&Q.readPixels(se,Se,xe,be,Ve.convert(Ct),Ve.convert(Nt),$e)}finally{let ct=j!==null?ae.get(j).__webglFramebuffer:null;R.bindFramebuffer(Q.FRAMEBUFFER,ct)}}},this.readRenderTargetPixelsAsync=async function(L,se,Se,xe,be,$e,tt,Ke=0){if(!(L&&L.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let ot=ae.get(L).__webglFramebuffer;if(L.isWebGLCubeRenderTarget&&tt!==void 0&&(ot=ot[tt]),ot)if(se>=0&&se<=L.width-xe&&Se>=0&&Se<=L.height-be){R.bindFramebuffer(Q.FRAMEBUFFER,ot);let ct=L.textures[Ke],Ct=ct.format,Nt=ct.type;L.textures.length>1&&Q.readBuffer(Q.COLOR_ATTACHMENT0+Ke);let lt=pr(ct);if(lt.__formatReadable===!1)throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if(lt.__typeReadable===!1)throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");let Ht=Q.createBuffer();Q.bindBuffer(Q.PIXEL_PACK_BUFFER,Ht),Q.bufferData(Q.PIXEL_PACK_BUFFER,$e.byteLength,Q.STREAM_READ),Q.readPixels(se,Se,xe,be,Ve.convert(Ct),Ve.convert(Nt),0),Q.bindBuffer(Q.PIXEL_PACK_BUFFER,null);let k=j!==null?ae.get(j).__webglFramebuffer:null;R.bindFramebuffer(Q.FRAMEBUFFER,k);let Ie=Q.fenceSync(Q.SYNC_GPU_COMMANDS_COMPLETE,0);return Q.flush(),await fp(Q,Ie,4),Q.bindBuffer(Q.PIXEL_PACK_BUFFER,Ht),Q.getBufferSubData(Q.PIXEL_PACK_BUFFER,0,$e),Q.bindBuffer(Q.PIXEL_PACK_BUFFER,null),Q.deleteBuffer(Ht),Q.deleteSync(Ie),$e}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")},this.copyFramebufferToTexture=function(L,se=null,Se=0){let xe=Math.pow(2,-Se),be=Math.floor(L.image.width*xe),$e=Math.floor(L.image.height*xe),tt=se!==null?se.x:0,Ke=se!==null?se.y:0;_e.setTexture2D(L,0),Q.copyTexSubImage2D(Q.TEXTURE_2D,Se,0,0,tt,Ke,be,$e),R.unbindTexture()},this.copyTextureToTexture=function(L,se,Se=null,xe=null,be=0,$e=0){let tt,Ke,ot,ct,Ct,Nt,lt,Ht,k,Ie=L.isCompressedTexture?L.mipmaps[$e]:L.image;if(Se!==null)tt=Se.max.x-Se.min.x,Ke=Se.max.y-Se.min.y,ot=Se.isBox3?Se.max.z-Se.min.z:1,ct=Se.min.x,Ct=Se.min.y,Nt=Se.isBox3?Se.min.z:0;else{let xt=Math.pow(2,-be);tt=Math.floor(Ie.width*xt),Ke=Math.floor(Ie.height*xt),L.isDataArrayTexture?ot=Ie.depth:L.isData3DTexture?ot=Math.floor(Ie.depth*xt):ot=1,ct=0,Ct=0,Nt=0}xe!==null?(lt=xe.x,Ht=xe.y,k=xe.z):(lt=0,Ht=0,k=0);let We=Ve.convert(se.format),ft=Ve.convert(se.type),Be;se.isData3DTexture?(_e.setTexture3D(se,0),Be=Q.TEXTURE_3D):se.isDataArrayTexture||se.isCompressedArrayTexture?(_e.setTexture2DArray(se,0),Be=Q.TEXTURE_2D_ARRAY):(_e.setTexture2D(se,0),Be=Q.TEXTURE_2D),R.activeTexture(Q.TEXTURE0),R.pixelStorei(Q.UNPACK_FLIP_Y_WEBGL,se.flipY),R.pixelStorei(Q.UNPACK_PREMULTIPLY_ALPHA_WEBGL,se.premultiplyAlpha),R.pixelStorei(Q.UNPACK_ALIGNMENT,se.unpackAlignment);let Vt=R.getParameter(Q.UNPACK_ROW_LENGTH),gt=R.getParameter(Q.UNPACK_IMAGE_HEIGHT),wt=R.getParameter(Q.UNPACK_SKIP_PIXELS),pt=R.getParameter(Q.UNPACK_SKIP_ROWS),mt=R.getParameter(Q.UNPACK_SKIP_IMAGES);R.pixelStorei(Q.UNPACK_ROW_LENGTH,Ie.width),R.pixelStorei(Q.UNPACK_IMAGE_HEIGHT,Ie.height),R.pixelStorei(Q.UNPACK_SKIP_PIXELS,ct),R.pixelStorei(Q.UNPACK_SKIP_ROWS,Ct),R.pixelStorei(Q.UNPACK_SKIP_IMAGES,Nt);let Ut=L.isDataArrayTexture||L.isData3DTexture,nt=se.isDataArrayTexture||se.isData3DTexture;if(L.isDepthTexture){let xt=ae.get(L),sn=ae.get(se),St=ae.get(xt.__renderTarget),st=ae.get(sn.__renderTarget);R.bindFramebuffer(Q.READ_FRAMEBUFFER,St.__webglFramebuffer),R.bindFramebuffer(Q.DRAW_FRAMEBUFFER,st.__webglFramebuffer);for(let bn=0;bn<ot;bn++)Ut&&(Q.framebufferTextureLayer(Q.READ_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,ae.get(L).__webglTexture,be,Nt+bn),Q.framebufferTextureLayer(Q.DRAW_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,ae.get(se).__webglTexture,$e,k+bn)),Q.blitFramebuffer(ct,Ct,tt,Ke,lt,Ht,tt,Ke,Q.DEPTH_BUFFER_BIT,Q.NEAREST);R.bindFramebuffer(Q.READ_FRAMEBUFFER,null),R.bindFramebuffer(Q.DRAW_FRAMEBUFFER,null)}else if(be!==0||L.isRenderTargetTexture||ae.has(L)){let xt=ae.get(L),sn=ae.get(se);R.bindFramebuffer(Q.READ_FRAMEBUFFER,P),R.bindFramebuffer(Q.DRAW_FRAMEBUFFER,N);for(let St=0;St<ot;St++)Ut?Q.framebufferTextureLayer(Q.READ_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,xt.__webglTexture,be,Nt+St):Q.framebufferTexture2D(Q.READ_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_2D,xt.__webglTexture,be),nt?Q.framebufferTextureLayer(Q.DRAW_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,sn.__webglTexture,$e,k+St):Q.framebufferTexture2D(Q.DRAW_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_2D,sn.__webglTexture,$e),be!==0?Q.blitFramebuffer(ct,Ct,tt,Ke,lt,Ht,tt,Ke,Q.COLOR_BUFFER_BIT,Q.NEAREST):nt?Q.copyTexSubImage3D(Be,$e,lt,Ht,k+St,ct,Ct,tt,Ke):Q.copyTexSubImage2D(Be,$e,lt,Ht,ct,Ct,tt,Ke);R.bindFramebuffer(Q.READ_FRAMEBUFFER,null),R.bindFramebuffer(Q.DRAW_FRAMEBUFFER,null)}else nt?L.isDataTexture||L.isData3DTexture?Q.texSubImage3D(Be,$e,lt,Ht,k,tt,Ke,ot,We,ft,Ie.data):se.isCompressedArrayTexture?Q.compressedTexSubImage3D(Be,$e,lt,Ht,k,tt,Ke,ot,We,Ie.data):Q.texSubImage3D(Be,$e,lt,Ht,k,tt,Ke,ot,We,ft,Ie):L.isDataTexture?Q.texSubImage2D(Q.TEXTURE_2D,$e,lt,Ht,tt,Ke,We,ft,Ie.data):L.isCompressedTexture?Q.compressedTexSubImage2D(Q.TEXTURE_2D,$e,lt,Ht,Ie.width,Ie.height,We,Ie.data):Q.texSubImage2D(Q.TEXTURE_2D,$e,lt,Ht,tt,Ke,We,ft,Ie);R.pixelStorei(Q.UNPACK_ROW_LENGTH,Vt),R.pixelStorei(Q.UNPACK_IMAGE_HEIGHT,gt),R.pixelStorei(Q.UNPACK_SKIP_PIXELS,wt),R.pixelStorei(Q.UNPACK_SKIP_ROWS,pt),R.pixelStorei(Q.UNPACK_SKIP_IMAGES,mt),$e===0&&se.generateMipmaps&&Q.generateMipmap(Be),R.unbindTexture()},this.initRenderTarget=function(L){ae.get(L).__webglFramebuffer===void 0&&_e.setupRenderTarget(L)},this.initTexture=function(L){L.isCubeTexture?_e.setTextureCube(L,0):L.isData3DTexture?_e.setTexture3D(L,0):L.isDataArrayTexture||L.isCompressedArrayTexture?_e.setTexture2DArray(L,0):_e.setTexture2D(L,0),R.unbindTexture()},this.resetState=function(){z=0,X=0,j=null,R.reset(),He.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return bi}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(e){this._outputColorSpace=e;let t=this.getContext();t.drawingBufferColorSpace=Dt._getDrawingBufferColorSpace(e),t.unpackColorSpace=Dt._getUnpackColorSpace()}};function zu(s,e){if(e===fu)return console.warn("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Geometry already defined as triangles."),s;if(e===na||e===yo){let t=s.getIndex();if(t===null){let r=[],a=s.getAttribute("position");if(a!==void 0){for(let o=0;o<a.count;o++)r.push(o);s.setIndex(r),t=s.getIndex()}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Undefined position attribute. Processing not possible."),s}let n=t.count-2,i=[];if(e===na)for(let r=1;r<=n;r++)i.push(t.getX(0)),i.push(t.getX(r)),i.push(t.getX(r+1));else for(let r=0;r<n;r++)r%2===0?(i.push(t.getX(r)),i.push(t.getX(r+1)),i.push(t.getX(r+2))):(i.push(t.getX(r+2)),i.push(t.getX(r+1)),i.push(t.getX(r)));return i.length/3!==n&&console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unable to generate correct amount of triangles."),s.setIndex(i),s.clearGroups(),s}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unknown draw mode:",e),s}function Xp(s){let e=new Map,t=new Map,n=s.clone();return jp(s,n,function(i,r){e.set(r,i),t.set(i,r)}),n.traverse(function(i){if(!i.isSkinnedMesh)return;let r=i,a=e.get(i),o=a.skeleton.bones;r.skeleton=a.skeleton.clone(),r.bindMatrix.copy(a.bindMatrix),r.skeleton.bones=o.map(function(l){return t.get(l)}),r.bind(r.skeleton,r.bindMatrix)}),n}function jp(s,e,t){t(s,e);for(let n=0;n<s.children.length;n++)jp(s.children[n],e.children[n],t)}var Ri=class extends Oi{constructor(e){super(e),this.dracoLoader=null,this.ktx2Loader=null,this.meshoptDecoder=null,this.pluginCallbacks=[],this.register(function(t){return new Wu(t)}),this.register(function(t){return new qu(t)}),this.register(function(t){return new ed(t)}),this.register(function(t){return new td(t)}),this.register(function(t){return new nd(t)}),this.register(function(t){return new ju(t)}),this.register(function(t){return new Ku(t)}),this.register(function(t){return new Yu(t)}),this.register(function(t){return new Ju(t)}),this.register(function(t){return new Vu(t)}),this.register(function(t){return new Zu(t)}),this.register(function(t){return new Xu(t)}),this.register(function(t){return new Qu(t)}),this.register(function(t){return new $u(t)}),this.register(function(t){return new Hu(t)}),this.register(function(t){return new Wc(t,kt.EXT_MESHOPT_COMPRESSION)}),this.register(function(t){return new Wc(t,kt.KHR_MESHOPT_COMPRESSION)}),this.register(function(t){return new id(t)})}load(e,t,n,i){let r=this,a;if(this.resourcePath!=="")a=this.resourcePath;else if(this.path!==""){let c=os.extractUrlBase(e);a=os.resolveURL(c,this.path)}else a=os.extractUrlBase(e);this.manager.itemStart(e);let o=function(c){i?i(c):console.error(c),r.manager.itemError(e),r.manager.itemEnd(e)},l=new Kr(this.manager);l.setPath(this.path),l.setResponseType("arraybuffer"),l.setRequestHeader(this.requestHeader),l.setWithCredentials(this.withCredentials),l.load(e,function(c){try{r.parse(c,a,function(h){t(h),r.manager.itemEnd(e)},o)}catch(h){o(h)}},n,o)}setDRACOLoader(e){return this.dracoLoader=e,this}setKTX2Loader(e){return this.ktx2Loader=e,this}setMeshoptDecoder(e){return this.meshoptDecoder=e,this}register(e){return this.pluginCallbacks.indexOf(e)===-1&&this.pluginCallbacks.push(e),this}unregister(e){return this.pluginCallbacks.indexOf(e)!==-1&&this.pluginCallbacks.splice(this.pluginCallbacks.indexOf(e),1),this}parse(e,t,n,i){let r,a={},o={},l=new TextDecoder;if(typeof e=="string")r=JSON.parse(e);else if(e instanceof ArrayBuffer)if(l.decode(new Uint8Array(e,0,4))===$p){try{a[kt.KHR_BINARY_GLTF]=new sd(e)}catch(d){i&&i(d);return}r=JSON.parse(a[kt.KHR_BINARY_GLTF].content)}else r=JSON.parse(l.decode(e));else r=e;if(r.asset===void 0||r.asset.version[0]<2){i&&i(new Error("THREE.GLTFLoader: Unsupported asset. glTF versions >=2.0 are supported."));return}let c=new ud(r,{path:t||this.resourcePath||"",crossOrigin:this.crossOrigin,requestHeader:this.requestHeader,manager:this.manager,ktx2Loader:this.ktx2Loader,meshoptDecoder:this.meshoptDecoder});c.fileLoader.setRequestHeader(this.requestHeader);for(let h=0;h<this.pluginCallbacks.length;h++){let d=this.pluginCallbacks[h](c);d.name||console.error("THREE.GLTFLoader: Invalid plugin found: missing name"),o[d.name]=d,a[d.name]=!0}if(r.extensionsUsed)for(let h=0;h<r.extensionsUsed.length;++h){let d=r.extensionsUsed[h],u=r.extensionsRequired||[];switch(d){case kt.KHR_MATERIALS_UNLIT:a[d]=new Gu;break;case kt.KHR_DRACO_MESH_COMPRESSION:a[d]=new rd(r,this.dracoLoader);break;case kt.KHR_TEXTURE_TRANSFORM:a[d]=new ad;break;case kt.KHR_MESH_QUANTIZATION:a[d]=new od;break;default:u.indexOf(d)>=0&&o[d]===void 0&&console.warn('THREE.GLTFLoader: Unknown extension "'+d+'".')}}c.setExtensions(a),c.setPlugins(o),c.parse(n,i)}parseAsync(e,t){let n=this;return new Promise(function(i,r){n.parse(e,t,i,r)})}};function U_(){let s={};return{get:function(e){return s[e]},add:function(e,t){s[e]=t},remove:function(e){delete s[e]},removeAll:function(){s={}}}}function fn(s,e,t){let n=s.json.materials[e];return n.extensions&&n.extensions[t]?n.extensions[t]:null}var kt={KHR_BINARY_GLTF:"KHR_binary_glTF",KHR_DRACO_MESH_COMPRESSION:"KHR_draco_mesh_compression",KHR_LIGHTS_PUNCTUAL:"KHR_lights_punctual",KHR_MATERIALS_CLEARCOAT:"KHR_materials_clearcoat",KHR_MATERIALS_DISPERSION:"KHR_materials_dispersion",KHR_MATERIALS_IOR:"KHR_materials_ior",KHR_MATERIALS_SHEEN:"KHR_materials_sheen",KHR_MATERIALS_SPECULAR:"KHR_materials_specular",KHR_MATERIALS_TRANSMISSION:"KHR_materials_transmission",KHR_MATERIALS_IRIDESCENCE:"KHR_materials_iridescence",KHR_MATERIALS_ANISOTROPY:"KHR_materials_anisotropy",KHR_MATERIALS_UNLIT:"KHR_materials_unlit",KHR_MATERIALS_VOLUME:"KHR_materials_volume",KHR_TEXTURE_BASISU:"KHR_texture_basisu",KHR_TEXTURE_TRANSFORM:"KHR_texture_transform",KHR_MESH_QUANTIZATION:"KHR_mesh_quantization",KHR_MATERIALS_EMISSIVE_STRENGTH:"KHR_materials_emissive_strength",EXT_MATERIALS_BUMP:"EXT_materials_bump",EXT_TEXTURE_WEBP:"EXT_texture_webp",EXT_TEXTURE_AVIF:"EXT_texture_avif",EXT_MESHOPT_COMPRESSION:"EXT_meshopt_compression",KHR_MESHOPT_COMPRESSION:"KHR_meshopt_compression",EXT_MESH_GPU_INSTANCING:"EXT_mesh_gpu_instancing"},Hu=class{constructor(e){this.parser=e,this.name=kt.KHR_LIGHTS_PUNCTUAL,this.cache={refs:{},uses:{}}}_markDefs(){let e=this.parser,t=this.parser.json.nodes||[];for(let n=0,i=t.length;n<i;n++){let r=t[n];r.extensions&&r.extensions[this.name]&&r.extensions[this.name].light!==void 0&&e._addNodeRef(this.cache,r.extensions[this.name].light)}}_loadLight(e){let t=this.parser,n="light:"+e,i=t.cache.get(n);if(i)return i;let r=t.json,l=((r.extensions&&r.extensions[this.name]||{}).lights||[])[e],c,h=new Le(16777215);l.color!==void 0&&h.setRGB(l.color[0],l.color[1],l.color[2],Hn);let d=l.range!==void 0?l.range:0;switch(l.type){case"directional":c=new as(h),c.target.position.set(0,0,-1),c.add(c.target);break;case"point":c=new wi(h),c.distance=d;break;case"spot":c=new eo(h),c.distance=d,l.spot=l.spot||{},l.spot.innerConeAngle=l.spot.innerConeAngle!==void 0?l.spot.innerConeAngle:0,l.spot.outerConeAngle=l.spot.outerConeAngle!==void 0?l.spot.outerConeAngle:Math.PI/4,c.angle=l.spot.outerConeAngle,c.penumbra=1-l.spot.innerConeAngle/l.spot.outerConeAngle,c.target.position.set(0,0,-1),c.add(c.target);break;default:throw new Error("THREE.GLTFLoader: Unexpected light type: "+l.type)}return c.position.set(0,0,0),Gi(c,l),l.intensity!==void 0&&(c.intensity=l.intensity),c.name=t.createUniqueName(l.name||"light_"+e),i=Promise.resolve(c),t.cache.add(n,i),i}getDependency(e,t){if(e==="light")return this._loadLight(t)}createNodeAttachment(e){let t=this,n=this.parser,r=n.json.nodes[e],o=(r.extensions&&r.extensions[this.name]||{}).light;return o===void 0?null:this._loadLight(o).then(function(l){return n._getNodeRef(t.cache,o,l)})}},Gu=class{constructor(){this.name=kt.KHR_MATERIALS_UNLIT}getMaterialType(){return jt}extendParams(e,t,n){let i=[];e.color=new Le(1,1,1),e.opacity=1;let r=t.pbrMetallicRoughness;if(r){if(Array.isArray(r.baseColorFactor)){let a=r.baseColorFactor;e.color.setRGB(a[0],a[1],a[2],Hn),e.opacity=a[3]}r.baseColorTexture!==void 0&&i.push(n.assignTexture(e,"map",r.baseColorTexture,an))}return Promise.all(i)}},Vu=class{constructor(e){this.parser=e,this.name=kt.KHR_MATERIALS_EMISSIVE_STRENGTH}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);return n===null||n.emissiveStrength!==void 0&&(t.emissiveIntensity=n.emissiveStrength),Promise.resolve()}},Wu=class{constructor(e){this.parser=e,this.name=kt.KHR_MATERIALS_CLEARCOAT}getMaterialType(e){return fn(this.parser,e,this.name)!==null?Wn:null}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(n.clearcoatFactor!==void 0&&(t.clearcoat=n.clearcoatFactor),n.clearcoatTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatMap",n.clearcoatTexture)),n.clearcoatRoughnessFactor!==void 0&&(t.clearcoatRoughness=n.clearcoatRoughnessFactor),n.clearcoatRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatRoughnessMap",n.clearcoatRoughnessTexture)),n.clearcoatNormalTexture!==void 0&&(i.push(this.parser.assignTexture(t,"clearcoatNormalMap",n.clearcoatNormalTexture)),n.clearcoatNormalTexture.scale!==void 0)){let r=n.clearcoatNormalTexture.scale;t.clearcoatNormalScale=new Ge(r,r)}return Promise.all(i)}},qu=class{constructor(e){this.parser=e,this.name=kt.KHR_MATERIALS_DISPERSION}getMaterialType(e){return fn(this.parser,e,this.name)!==null?Wn:null}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);return n===null||(t.dispersion=n.dispersion!==void 0?n.dispersion:0),Promise.resolve()}},Xu=class{constructor(e){this.parser=e,this.name=kt.KHR_MATERIALS_IRIDESCENCE}getMaterialType(e){return fn(this.parser,e,this.name)!==null?Wn:null}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.iridescenceFactor!==void 0&&(t.iridescence=n.iridescenceFactor),n.iridescenceTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceMap",n.iridescenceTexture)),n.iridescenceIor!==void 0&&(t.iridescenceIOR=n.iridescenceIor),t.iridescenceThicknessRange===void 0&&(t.iridescenceThicknessRange=[100,400]),n.iridescenceThicknessMinimum!==void 0&&(t.iridescenceThicknessRange[0]=n.iridescenceThicknessMinimum),n.iridescenceThicknessMaximum!==void 0&&(t.iridescenceThicknessRange[1]=n.iridescenceThicknessMaximum),n.iridescenceThicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceThicknessMap",n.iridescenceThicknessTexture)),Promise.all(i)}},ju=class{constructor(e){this.parser=e,this.name=kt.KHR_MATERIALS_SHEEN}getMaterialType(e){return fn(this.parser,e,this.name)!==null?Wn:null}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(t.sheenColor=new Le(0,0,0),t.sheenRoughness=0,t.sheen=1,n.sheenColorFactor!==void 0){let r=n.sheenColorFactor;t.sheenColor.setRGB(r[0],r[1],r[2],Hn)}return n.sheenRoughnessFactor!==void 0&&(t.sheenRoughness=n.sheenRoughnessFactor),n.sheenColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenColorMap",n.sheenColorTexture,an)),n.sheenRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenRoughnessMap",n.sheenRoughnessTexture)),Promise.all(i)}},Ku=class{constructor(e){this.parser=e,this.name=kt.KHR_MATERIALS_TRANSMISSION}getMaterialType(e){return fn(this.parser,e,this.name)!==null?Wn:null}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.transmissionFactor!==void 0&&(t.transmission=n.transmissionFactor),n.transmissionTexture!==void 0&&i.push(this.parser.assignTexture(t,"transmissionMap",n.transmissionTexture)),Promise.all(i)}},Yu=class{constructor(e){this.parser=e,this.name=kt.KHR_MATERIALS_VOLUME}getMaterialType(e){return fn(this.parser,e,this.name)!==null?Wn:null}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.thickness=n.thicknessFactor!==void 0?n.thicknessFactor:0,n.thicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"thicknessMap",n.thicknessTexture)),t.attenuationDistance=n.attenuationDistance||1/0;let r=n.attenuationColor||[1,1,1];return t.attenuationColor=new Le().setRGB(r[0],r[1],r[2],Hn),Promise.all(i)}},Ju=class{constructor(e){this.parser=e,this.name=kt.KHR_MATERIALS_IOR}getMaterialType(e){return fn(this.parser,e,this.name)!==null?Wn:null}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);return n===null||(t.ior=n.ior!==void 0?n.ior:1.5,t.ior===0&&(t.ior=1e3)),Promise.resolve()}},Zu=class{constructor(e){this.parser=e,this.name=kt.KHR_MATERIALS_SPECULAR}getMaterialType(e){return fn(this.parser,e,this.name)!==null?Wn:null}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.specularIntensity=n.specularFactor!==void 0?n.specularFactor:1,n.specularTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularIntensityMap",n.specularTexture));let r=n.specularColorFactor||[1,1,1];return t.specularColor=new Le().setRGB(r[0],r[1],r[2],Hn),n.specularColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularColorMap",n.specularColorTexture,an)),Promise.all(i)}},$u=class{constructor(e){this.parser=e,this.name=kt.EXT_MATERIALS_BUMP}getMaterialType(e){return fn(this.parser,e,this.name)!==null?Wn:null}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return t.bumpScale=n.bumpFactor!==void 0?n.bumpFactor:1,n.bumpTexture!==void 0&&i.push(this.parser.assignTexture(t,"bumpMap",n.bumpTexture)),Promise.all(i)}},Qu=class{constructor(e){this.parser=e,this.name=kt.KHR_MATERIALS_ANISOTROPY}getMaterialType(e){return fn(this.parser,e,this.name)!==null?Wn:null}extendMaterialParams(e,t){let n=fn(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.anisotropyStrength!==void 0&&(t.anisotropy=n.anisotropyStrength),n.anisotropyRotation!==void 0&&(t.anisotropyRotation=n.anisotropyRotation),n.anisotropyTexture!==void 0&&i.push(this.parser.assignTexture(t,"anisotropyMap",n.anisotropyTexture)),Promise.all(i)}},ed=class{constructor(e){this.parser=e,this.name=kt.KHR_TEXTURE_BASISU}loadTexture(e){let t=this.parser,n=t.json,i=n.textures[e];if(!i.extensions||!i.extensions[this.name])return null;let r=i.extensions[this.name],a=t.options.ktx2Loader;if(!a){if(n.extensionsRequired&&n.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setKTX2Loader must be called before loading KTX2 textures");return null}return t.loadTextureImage(e,r.source,a)}},td=class{constructor(e){this.parser=e,this.name=kt.EXT_TEXTURE_WEBP}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let a=r.extensions[t],o=i.images[a.source],l=n.textureLoader;if(o.uri){let c=n.options.manager.getHandler(o.uri);c!==null&&(l=c)}return n.loadTextureImage(e,a.source,l)}},nd=class{constructor(e){this.parser=e,this.name=kt.EXT_TEXTURE_AVIF}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let a=r.extensions[t],o=i.images[a.source],l=n.textureLoader;if(o.uri){let c=n.options.manager.getHandler(o.uri);c!==null&&(l=c)}return n.loadTextureImage(e,a.source,l)}},Wc=class{constructor(e,t){this.name=t,this.parser=e}loadBufferView(e){let t=this.parser.json,n=t.bufferViews[e];if(n.extensions&&n.extensions[this.name]){let i=n.extensions[this.name],r=this.parser.getDependency("buffer",i.buffer),a=this.parser.options.meshoptDecoder;if(!a||!a.supported){if(t.extensionsRequired&&t.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setMeshoptDecoder must be called before loading compressed files");return null}return r.then(function(o){let l=i.byteOffset||0,c=i.byteLength||0,h=i.count,d=i.byteStride,u=new Uint8Array(o,l,c);return a.decodeGltfBufferAsync?a.decodeGltfBufferAsync(h,d,u,i.mode,i.filter).then(function(p){return p.buffer}):a.ready.then(function(){let p=new ArrayBuffer(h*d);return a.decodeGltfBuffer(new Uint8Array(p),h,d,u,i.mode,i.filter),p})})}else return null}},id=class{constructor(e){this.name=kt.EXT_MESH_GPU_INSTANCING,this.parser=e}createNodeMesh(e){let t=this.parser.json,n=t.nodes[e];if(!n.extensions||!n.extensions[this.name]||n.mesh===void 0)return null;let i=t.meshes[n.mesh];for(let c of i.primitives)if(c.mode!==hi.TRIANGLES&&c.mode!==hi.TRIANGLE_STRIP&&c.mode!==hi.TRIANGLE_FAN&&c.mode!==void 0)return null;let a=n.extensions[this.name].attributes,o=[],l={};for(let c in a)o.push(this.parser.getDependency("accessor",a[c]).then(h=>(l[c]=h,l[c])));return o.length<1?null:(o.push(this.parser.createNodeMesh(e)),Promise.all(o).then(c=>{let h=c.pop(),d=h.isGroup?h.children:[h],u=c[0].count,p=[];for(let x of d){let M=new yt,m=new U,f=new nn,b=new U(1,1,1),E=new An(x.geometry,x.material,u);for(let _=0;_<u;_++)l.TRANSLATION&&m.fromBufferAttribute(l.TRANSLATION,_),l.ROTATION&&f.fromBufferAttribute(l.ROTATION,_),l.SCALE&&b.fromBufferAttribute(l.SCALE,_),E.setMatrixAt(_,M.compose(m,f,b));let g=null;for(let _ in l)if(_==="_COLOR_0"){let w=l[_];E.instanceColor=new yi(w.array,w.itemSize,w.normalized)}else if(_!=="TRANSLATION"&&_!=="ROTATION"&&_!=="SCALE"){if(g===null){let T=E.geometry;g=new At,g.name=T.name;for(let y in T.attributes)g.setAttribute(y,T.attributes[y]);for(let y in T.morphAttributes)g.morphAttributes[y]=T.morphAttributes[y];T.index!==null&&g.setIndex(T.index),g.morphTargetsRelative=T.morphTargetsRelative;for(let y of T.groups)g.addGroup(y.start,y.count,y.materialIndex);T.boundingBox!==null&&(g.boundingBox=T.boundingBox.clone()),T.boundingSphere!==null&&(g.boundingSphere=T.boundingSphere.clone()),g.drawRange.start=T.drawRange.start,g.drawRange.count=T.drawRange.count,g.userData=Object.assign({},T.userData),E.geometry=g}let w=l[_];g.setAttribute(_,new yi(w.array,w.itemSize,w.normalized))}Yt.prototype.copy.call(E,x),this.parser.assignFinalMaterial(E),p.push(E)}return h.isGroup?(h.clear(),h.add(...p),h):p[0]}))}},$p="glTF",Eo=12,Kp={JSON:1313821514,BIN:5130562},sd=class{constructor(e){this.name=kt.KHR_BINARY_GLTF,this.content=null,this.body=null;let t=new DataView(e,0,Eo),n=new TextDecoder;if(this.header={magic:n.decode(new Uint8Array(e.slice(0,4))),version:t.getUint32(4,!0),length:t.getUint32(8,!0)},this.header.magic!==$p)throw new Error("THREE.GLTFLoader: Unsupported glTF-Binary header.");if(this.header.version<2)throw new Error("THREE.GLTFLoader: Legacy binary file detected.");let i=this.header.length-Eo,r=new DataView(e,Eo),a=0;for(;a<i;){let o=r.getUint32(a,!0);a+=4;let l=r.getUint32(a,!0);if(a+=4,l===Kp.JSON){let c=new Uint8Array(e,Eo+a,o);this.content=n.decode(c)}else if(l===Kp.BIN){let c=Eo+a;this.body=e.slice(c,c+o)}a+=o}if(this.content===null)throw new Error("THREE.GLTFLoader: JSON content not found.")}},rd=class{constructor(e,t){if(!t)throw new Error("THREE.GLTFLoader: No DRACOLoader instance provided.");this.name=kt.KHR_DRACO_MESH_COMPRESSION,this.json=e,this.dracoLoader=t,this.dracoLoader.preload()}decodePrimitive(e,t){let n=this.json,i=this.dracoLoader,r=e.extensions[this.name].bufferView,a=e.extensions[this.name].attributes,o={},l={},c={};for(let h in a){let d=cd[h]||h.toLowerCase();o[d]=a[h]}for(let h in e.attributes){let d=cd[h]||h.toLowerCase();if(a[h]!==void 0){let u=n.accessors[e.attributes[h]],p=la[u.componentType];c[d]=p.name,l[d]=u.normalized===!0}}return t.getDependency("bufferView",r).then(function(h){return new Promise(function(d,u){i.decodeDracoFile(h,function(p){for(let x in p.attributes){let M=p.attributes[x],m=l[x];m!==void 0&&(M.normalized=m)}d(p)},o,c,Hn,u)})})}},ad=class{constructor(){this.name=kt.KHR_TEXTURE_TRANSFORM}extendTexture(e,t){if((t.texCoord===void 0||t.texCoord===e.channel)&&t.offset===void 0&&t.rotation===void 0&&t.scale===void 0)return e;if(e=e.clone(),t.texCoord!==void 0&&(e.channel=t.texCoord),t.offset!==void 0&&e.offset.fromArray(t.offset),t.rotation!==void 0&&(e.rotation=t.rotation),t.scale!==void 0&&e.repeat.fromArray(t.scale),t.rotation!==void 0){let n=Math.cos(e.rotation),i=Math.sin(e.rotation);e.matrix.set(e.repeat.x*n,e.repeat.y*i,e.offset.x,-e.repeat.x*i,e.repeat.y*n,e.offset.y,0,0,1),e.matrixAutoUpdate=!1}return e.needsUpdate=!0,e}},od=class{constructor(){this.name=kt.KHR_MESH_QUANTIZATION}},qc=class extends zi{constructor(e,t,n,i){super(e,t,n,i)}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i*3+i;for(let a=0;a!==i;a++)t[a]=n[r+a];return t}interpolate_(e,t,n,i){let r=this.resultBuffer,a=this.sampleValues,o=this.valueSize,l=o*2,c=o*3,h=i-t,d=(n-t)/h,u=d*d,p=u*d,x=e*c,M=x-c,m=-2*p+3*u,f=p-u,b=1-m,E=f-u+d;for(let g=0;g!==o;g++){let _=a[M+g+o],w=a[M+g+l]*h,T=a[x+g+o],y=a[x+g]*h;r[g]=b*_+E*w+m*T+f*y}return r}},z_=new nn,ld=class extends qc{interpolate_(e,t,n,i){let r=super.interpolate_(e,t,n,i);return z_.fromArray(r).normalize().toArray(r),r}},hi={FLOAT:5126,FLOAT_MAT3:35675,FLOAT_MAT4:35676,FLOAT_VEC2:35664,FLOAT_VEC3:35665,FLOAT_VEC4:35666,LINEAR:9729,REPEAT:10497,SAMPLER_2D:35678,POINTS:0,LINES:1,LINE_LOOP:2,LINE_STRIP:3,TRIANGLES:4,TRIANGLE_STRIP:5,TRIANGLE_FAN:6,UNSIGNED_BYTE:5121,UNSIGNED_SHORT:5123},la={5120:Int8Array,5121:Uint8Array,5122:Int16Array,5123:Uint16Array,5125:Uint32Array,5126:Float32Array},Yp={9728:cn,9729:on,9984:Jl,9985:Qr,9986:rr,9987:Ei},Jp={33071:ri,33648:Ir,10497:Di},Ou={SCALAR:1,VEC2:2,VEC3:3,VEC4:4,MAT2:4,MAT3:9,MAT4:16},cd={POSITION:"position",NORMAL:"normal",TANGENT:"tangent",TEXCOORD_0:"uv",TEXCOORD_1:"uv1",TEXCOORD_2:"uv2",TEXCOORD_3:"uv3",COLOR_0:"color",WEIGHTS_0:"skinWeight",JOINTS_0:"skinIndex"},Is={scale:"scale",translation:"position",rotation:"quaternion",weights:"morphTargetInfluences"},O_={CUBICSPLINE:void 0,LINEAR:js,STEP:Xs},ku={OPAQUE:"OPAQUE",MASK:"MASK",BLEND:"BLEND"};function k_(s){return s.DefaultMaterial===void 0&&(s.DefaultMaterial=new vn({color:16777215,emissive:0,metalness:1,roughness:1,transparent:!1,depthTest:!0,side:oi})),s.DefaultMaterial}function lr(s,e,t){for(let n in t.extensions)s[n]===void 0&&(e.userData.gltfExtensions=e.userData.gltfExtensions||{},e.userData.gltfExtensions[n]=t.extensions[n])}function Gi(s,e){e.extras!==void 0&&(typeof e.extras=="object"?Object.assign(s.userData,e.extras):console.warn("THREE.GLTFLoader: Ignoring primitive type .extras, "+e.extras))}function B_(s,e,t){let n=!1,i=!1,r=!1;for(let c=0,h=e.length;c<h;c++){let d=e[c];if(d.POSITION!==void 0&&(n=!0),d.NORMAL!==void 0&&(i=!0),d.COLOR_0!==void 0&&(r=!0),n&&i&&r)break}if(!n&&!i&&!r)return Promise.resolve(s);let a=[],o=[],l=[];for(let c=0,h=e.length;c<h;c++){let d=e[c];if(n){let u=d.POSITION!==void 0?t.getDependency("accessor",d.POSITION):s.attributes.position;a.push(u)}if(i){let u=d.NORMAL!==void 0?t.getDependency("accessor",d.NORMAL):s.attributes.normal;o.push(u)}if(r){let u=d.COLOR_0!==void 0?t.getDependency("accessor",d.COLOR_0):s.attributes.color;l.push(u)}}return Promise.all([Promise.all(a),Promise.all(o),Promise.all(l)]).then(function(c){let h=c[0],d=c[1],u=c[2];return n&&(s.morphAttributes.position=h),i&&(s.morphAttributes.normal=d),r&&(s.morphAttributes.color=u),s.morphTargetsRelative=!0,s})}function H_(s,e){if(s.updateMorphTargets(),e.weights!==void 0)for(let t=0,n=e.weights.length;t<n;t++)s.morphTargetInfluences[t]=e.weights[t];if(e.extras&&Array.isArray(e.extras.targetNames)){let t=e.extras.targetNames;if(s.morphTargetInfluences.length===t.length){s.morphTargetDictionary={};for(let n=0,i=t.length;n<i;n++)s.morphTargetDictionary[t[n]]=n}else console.warn("THREE.GLTFLoader: Invalid extras.targetNames length. Ignoring names.")}}function G_(s){let e,t=s.extensions&&s.extensions[kt.KHR_DRACO_MESH_COMPRESSION];if(t?e="draco:"+t.bufferView+":"+t.indices+":"+Bu(t.attributes):e=s.indices+":"+Bu(s.attributes)+":"+s.mode,s.targets!==void 0)for(let n=0,i=s.targets.length;n<i;n++)e+=":"+Bu(s.targets[n]);return e}function Bu(s){let e="",t=Object.keys(s).sort();for(let n=0,i=t.length;n<i;n++)e+=t[n]+":"+s[t[n]]+";";return e}function hd(s){switch(s){case Int8Array:return 1/127;case Uint8Array:return 1/255;case Int16Array:return 1/32767;case Uint16Array:return 1/65535;default:throw new Error("THREE.GLTFLoader: Unsupported normalized accessor component type.")}}function V_(s){return s.search(/\.jpe?g($|\?)/i)>0||s.search(/^data\:image\/jpeg/)===0?"image/jpeg":s.search(/\.webp($|\?)/i)>0||s.search(/^data\:image\/webp/)===0?"image/webp":s.search(/\.ktx2($|\?)/i)>0||s.search(/^data\:image\/ktx2/)===0?"image/ktx2":"image/png"}var W_=new yt,ud=class{constructor(e={},t={}){this.json=e,this.extensions={},this.plugins={},this.options=t,this.cache=new U_,this.associations=new Map,this.primitiveCache={},this.nodeCache={},this.meshCache={refs:{},uses:{}},this.cameraCache={refs:{},uses:{}},this.lightCache={refs:{},uses:{}},this.sourceCache={},this.textureCache={},this.nodeNamesUsed={};let n=!1,i=-1,r=!1,a=-1;if(typeof navigator<"u"&&typeof navigator.userAgent<"u"){let o=navigator.userAgent;n=/^((?!chrome|android).)*safari/i.test(o)===!0;let l=o.match(/Version\/(\d+)/);i=n&&l?parseInt(l[1],10):-1,r=o.indexOf("Firefox")>-1,a=r?o.match(/Firefox\/([0-9]+)\./)[1]:-1}typeof createImageBitmap>"u"||n&&i<17||r&&a<98?this.textureLoader=new Za(this.options.manager):this.textureLoader=new to(this.options.manager),this.textureLoader.setCrossOrigin(this.options.crossOrigin),this.textureLoader.setRequestHeader(this.options.requestHeader),this.fileLoader=new Kr(this.options.manager),this.fileLoader.setResponseType("arraybuffer"),this.options.crossOrigin==="use-credentials"&&this.fileLoader.setWithCredentials(!0)}setExtensions(e){this.extensions=e}setPlugins(e){this.plugins=e}parse(e,t){let n=this,i=this.json,r=this.extensions;this.cache.removeAll(),this.nodeCache={},this._invokeAll(function(a){return a._markDefs&&a._markDefs()}),Promise.all(this._invokeAll(function(a){return a.beforeRoot&&a.beforeRoot()})).then(function(){return Promise.all([n.getDependencies("scene"),n.getDependencies("animation"),n.getDependencies("camera")])}).then(function(a){let o={scene:a[0][i.scene||0],scenes:a[0],animations:a[1],cameras:a[2],asset:i.asset,parser:n,userData:{}};return lr(r,o,i),Gi(o,i),Promise.all(n._invokeAll(function(l){return l.afterRoot&&l.afterRoot(o)})).then(function(){for(let l of o.scenes)l.updateMatrixWorld();e(o)})}).catch(t)}_markDefs(){let e=this.json.nodes||[],t=this.json.skins||[],n=this.json.meshes||[];for(let i=0,r=t.length;i<r;i++){let a=t[i].joints;for(let o=0,l=a.length;o<l;o++)e[a[o]].isBone=!0}for(let i=0,r=e.length;i<r;i++){let a=e[i];a.mesh!==void 0&&(this._addNodeRef(this.meshCache,a.mesh),a.skin!==void 0&&(n[a.mesh].isSkinnedMesh=!0)),a.camera!==void 0&&this._addNodeRef(this.cameraCache,a.camera)}}_addNodeRef(e,t){t!==void 0&&(e.refs[t]===void 0&&(e.refs[t]=e.uses[t]=0),e.refs[t]++)}_getNodeRef(e,t,n){if(e.refs[t]<=1)return n;let i=n.clone(),r=(a,o)=>{let l=this.associations.get(a);l!=null&&this.associations.set(o,l);for(let[c,h]of a.children.entries())r(h,o.children[c])};return r(n,i),i.name+="_instance_"+e.uses[t]++,i}_invokeOne(e){let t=Object.values(this.plugins);t.push(this);for(let n=0;n<t.length;n++){let i=e(t[n]);if(i)return i}return null}_invokeAll(e){let t=Object.values(this.plugins);t.unshift(this);let n=[];for(let i=0;i<t.length;i++){let r=e(t[i]);r&&n.push(r)}return n}getDependency(e,t){let n=e+":"+t,i=this.cache.get(n);if(!i){switch(e){case"scene":i=this.loadScene(t);break;case"node":i=this._invokeOne(function(r){return r.loadNode&&r.loadNode(t)});break;case"mesh":i=this._invokeOne(function(r){return r.loadMesh&&r.loadMesh(t)});break;case"accessor":i=this.loadAccessor(t);break;case"bufferView":i=this._invokeOne(function(r){return r.loadBufferView&&r.loadBufferView(t)});break;case"buffer":i=this.loadBuffer(t);break;case"material":i=this._invokeOne(function(r){return r.loadMaterial&&r.loadMaterial(t)});break;case"texture":i=this._invokeOne(function(r){return r.loadTexture&&r.loadTexture(t)});break;case"skin":i=this.loadSkin(t);break;case"animation":i=this._invokeOne(function(r){return r.loadAnimation&&r.loadAnimation(t)});break;case"camera":i=this.loadCamera(t);break;default:if(i=this._invokeOne(function(r){return r!=this&&r.getDependency&&r.getDependency(e,t)}),!i)throw new Error("Unknown type: "+e);break}this.cache.add(n,i)}return i}getDependencies(e){let t=this.cache.get(e);if(!t){let n=this,i=this.json[e+(e==="mesh"?"es":"s")]||[];t=Promise.all(i.map(function(r,a){return n.getDependency(e,a)})),this.cache.add(e,t)}return t}loadBuffer(e){let t=this.json.buffers[e],n=this.fileLoader;if(t.type&&t.type!=="arraybuffer")throw new Error("THREE.GLTFLoader: "+t.type+" buffer type is not supported.");if(t.uri===void 0&&e===0)return Promise.resolve(this.extensions[kt.KHR_BINARY_GLTF].body);let i=this.options;return new Promise(function(r,a){n.load(os.resolveURL(t.uri,i.path),r,void 0,function(){a(new Error('THREE.GLTFLoader: Failed to load buffer "'+t.uri+'".'))})})}loadBufferView(e){let t=this.json.bufferViews[e];return this.getDependency("buffer",t.buffer).then(function(n){let i=t.byteLength||0,r=t.byteOffset||0;return n.slice(r,r+i)})}loadAccessor(e){let t=this,n=this.json,i=this.json.accessors[e];if(i.bufferView===void 0&&i.sparse===void 0){let a=Ou[i.type],o=la[i.componentType],l=i.normalized===!0,c=new o(i.count*a);return Promise.resolve(new Pt(c,a,l))}let r=[];return i.bufferView!==void 0?r.push(this.getDependency("bufferView",i.bufferView)):r.push(null),i.sparse!==void 0&&(r.push(this.getDependency("bufferView",i.sparse.indices.bufferView)),r.push(this.getDependency("bufferView",i.sparse.values.bufferView))),Promise.all(r).then(function(a){let o=a[0],l=Ou[i.type],c=la[i.componentType],h=c.BYTES_PER_ELEMENT,d=h*l,u=i.byteOffset||0,p=i.bufferView!==void 0?n.bufferViews[i.bufferView].byteStride:void 0,x=i.normalized===!0,M,m;if(p&&p!==d){let f=Math.floor(u/p),b="InterleavedBuffer:"+i.bufferView+":"+i.componentType+":"+f+":"+i.count,E=t.cache.get(b);E||(M=new c(o,f*p,i.count*p/h),E=new Or(M,p/h),t.cache.add(b,E)),m=new kr(E,l,u%p/h,x)}else o===null?M=new c(i.count*l):M=new c(o,u,i.count*l),m=new Pt(M,l,x);if(i.sparse!==void 0){let f=Ou.SCALAR,b=la[i.sparse.indices.componentType],E=i.sparse.indices.byteOffset||0,g=i.sparse.values.byteOffset||0,_=new b(a[1],E,i.sparse.count*f),w=new c(a[2],g,i.sparse.count*l);o!==null&&(m=new Pt(m.array.slice(),m.itemSize,m.normalized)),m.normalized=!1;for(let T=0,y=_.length;T<y;T++){let C=_[T];if(m.setX(C,w[T*l]),l>=2&&m.setY(C,w[T*l+1]),l>=3&&m.setZ(C,w[T*l+2]),l>=4&&m.setW(C,w[T*l+3]),l>=5)throw new Error("THREE.GLTFLoader: Unsupported itemSize in sparse BufferAttribute.")}m.normalized=x}return m})}loadTexture(e){let t=this.json,n=this.options,r=t.textures[e].source,a=t.images[r],o=this.textureLoader;if(a.uri){let l=n.manager.getHandler(a.uri);l!==null&&(o=l)}return this.loadTextureImage(e,r,o)}loadTextureImage(e,t,n){let i=this,r=this.json,a=r.textures[e],o=r.images[t],l=(o.uri||o.bufferView)+":"+a.sampler;if(this.textureCache[l])return this.textureCache[l];let c=this.loadImageSource(t,n).then(function(h){h.flipY=!1,h.name=a.name||o.name||"",h.name===""&&typeof o.uri=="string"&&o.uri.startsWith("data:image/")===!1&&(h.name=o.uri);let u=(r.samplers||{})[a.sampler]||{};return h.magFilter=Yp[u.magFilter]||on,h.minFilter=Yp[u.minFilter]||Ei,h.wrapS=Jp[u.wrapS]||Di,h.wrapT=Jp[u.wrapT]||Di,h.generateMipmaps=!h.isCompressedTexture&&h.minFilter!==cn&&h.minFilter!==on,i.associations.set(h,{textures:e}),h}).catch(function(){return null});return this.textureCache[l]=c,c}loadImageSource(e,t){let n=this,i=this.json,r=this.options;if(this.sourceCache[e]!==void 0)return this.sourceCache[e].then(d=>d.clone());let a=i.images[e],o=self.URL||self.webkitURL,l=a.uri||"",c=!1;if(a.bufferView!==void 0)l=n.getDependency("bufferView",a.bufferView).then(function(d){c=!0;let u=new Blob([d],{type:a.mimeType});return l=o.createObjectURL(u),l});else if(a.uri===void 0)throw new Error("THREE.GLTFLoader: Image "+e+" is missing URI and bufferView");let h=Promise.resolve(l).then(function(d){return new Promise(function(u,p){let x=u;t.isImageBitmapLoader===!0&&(x=function(M){let m=new hn(M);m.needsUpdate=!0,u(m)}),t.load(os.resolveURL(d,r.path),x,void 0,p)})}).then(function(d){return c===!0&&o.revokeObjectURL(l),Gi(d,a),d.userData.mimeType=a.mimeType||V_(a.uri),d}).catch(function(d){throw console.error("THREE.GLTFLoader: Couldn't load texture",l),d});return this.sourceCache[e]=h,h}assignTexture(e,t,n,i){let r=this;return this.getDependency("texture",n.index).then(function(a){if(!a)return null;if(n.texCoord!==void 0&&n.texCoord>0&&(a=a.clone(),a.channel=n.texCoord),r.extensions[kt.KHR_TEXTURE_TRANSFORM]){let o=n.extensions!==void 0?n.extensions[kt.KHR_TEXTURE_TRANSFORM]:void 0;if(o){let l=r.associations.get(a);a=r.extensions[kt.KHR_TEXTURE_TRANSFORM].extendTexture(a,o),r.associations.set(a,l)}}return i!==void 0&&(a.colorSpace=i),e[t]=a,a})}assignFinalMaterial(e){let t=e.geometry,n=e.material,i=t.attributes.tangent===void 0,r=t.attributes.color!==void 0,a=t.attributes.normal===void 0;if(e.isPoints){let o="PointsMaterial:"+n.uuid,l=this.cache.get(o);l||(l=new Gr,Gn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,l.sizeAttenuation=!1,this.cache.add(o,l)),n=l}else if(e.isLine){let o="LineBasicMaterial:"+n.uuid,l=this.cache.get(o);l||(l=new Ni,Gn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,this.cache.add(o,l)),n=l}if(i||r||a){let o="ClonedMaterial:"+n.uuid+":";i&&(o+="derivative-tangents:"),r&&(o+="vertex-colors:"),a&&(o+="flat-shading:");let l=this.cache.get(o);l||(l=n.clone(),r&&(l.vertexColors=!0),a&&(l.flatShading=!0),i&&(l.normalScale&&(l.normalScale.y*=-1),l.clearcoatNormalScale&&(l.clearcoatNormalScale.y*=-1)),this.cache.add(o,l),this.associations.set(l,this.associations.get(n))),n=l}e.material=n}getMaterialType(){return vn}loadMaterial(e){let t=this,n=this.json,i=this.extensions,r=n.materials[e],a,o={},l=r.extensions||{},c=[];if(l[kt.KHR_MATERIALS_UNLIT]){let d=i[kt.KHR_MATERIALS_UNLIT];a=d.getMaterialType(),c.push(d.extendParams(o,r,t))}else{let d=r.pbrMetallicRoughness||{};if(o.color=new Le(1,1,1),o.opacity=1,Array.isArray(d.baseColorFactor)){let u=d.baseColorFactor;o.color.setRGB(u[0],u[1],u[2],Hn),o.opacity=u[3]}d.baseColorTexture!==void 0&&c.push(t.assignTexture(o,"map",d.baseColorTexture,an)),o.metalness=d.metallicFactor!==void 0?d.metallicFactor:1,o.roughness=d.roughnessFactor!==void 0?d.roughnessFactor:1,d.metallicRoughnessTexture!==void 0&&(c.push(t.assignTexture(o,"metalnessMap",d.metallicRoughnessTexture)),c.push(t.assignTexture(o,"roughnessMap",d.metallicRoughnessTexture))),a=this._invokeOne(function(u){return u.getMaterialType&&u.getMaterialType(e)}),c.push(Promise.all(this._invokeAll(function(u){return u.extendMaterialParams&&u.extendMaterialParams(e,o)})))}r.doubleSided===!0&&(o.side=Kt);let h=r.alphaMode||ku.OPAQUE;if(h===ku.BLEND?(o.transparent=!0,o.depthWrite=!1):(o.transparent=!1,h===ku.MASK&&(o.alphaTest=r.alphaCutoff!==void 0?r.alphaCutoff:.5)),r.normalTexture!==void 0&&a!==jt&&(c.push(t.assignTexture(o,"normalMap",r.normalTexture)),o.normalScale=new Ge(1,1),r.normalTexture.scale!==void 0)){let d=r.normalTexture.scale;o.normalScale.set(d,d)}if(r.occlusionTexture!==void 0&&a!==jt&&(c.push(t.assignTexture(o,"aoMap",r.occlusionTexture)),r.occlusionTexture.strength!==void 0&&(o.aoMapIntensity=r.occlusionTexture.strength)),r.emissiveFactor!==void 0&&a!==jt){let d=r.emissiveFactor;o.emissive=new Le().setRGB(d[0],d[1],d[2],Hn)}return r.emissiveTexture!==void 0&&a!==jt&&c.push(t.assignTexture(o,"emissiveMap",r.emissiveTexture,an)),Promise.all(c).then(function(){let d=new a(o);return r.name&&(d.name=r.name),Gi(d,r),t.associations.set(d,{materials:e}),r.extensions&&lr(i,d,r),d})}createUniqueName(e){let t=tn.sanitizeNodeName(e||"");return t in this.nodeNamesUsed?t+"_"+ ++this.nodeNamesUsed[t]:(this.nodeNamesUsed[t]=0,t)}loadGeometries(e){let t=this,n=this.extensions,i=this.primitiveCache;function r(o){return n[kt.KHR_DRACO_MESH_COMPRESSION].decodePrimitive(o,t).then(function(l){return Zp(l,o,t)})}let a=[];for(let o=0,l=e.length;o<l;o++){let c=e[o],h=G_(c),d=i[h];if(d)a.push(d.promise);else{let u;c.extensions&&c.extensions[kt.KHR_DRACO_MESH_COMPRESSION]?u=r(c):u=Zp(new At,c,t),c.mode===hi.TRIANGLE_STRIP?u=u.then(p=>zu(p,yo)):c.mode===hi.TRIANGLE_FAN&&(u=u.then(p=>zu(p,na))),i[h]={primitive:c,promise:u},a.push(u)}}return Promise.all(a)}loadMesh(e){let t=this,n=this.json,i=this.extensions,r=n.meshes[e],a=r.primitives,o=[];for(let l=0,c=a.length;l<c;l++){let h=a[l].material===void 0?k_(this.cache):this.getDependency("material",a[l].material);o.push(h)}return o.push(t.loadGeometries(a)),Promise.all(o).then(async function(l){let c=l.slice(0,l.length-1),h=l[l.length-1],d=[];for(let p=0,x=h.length;p<x;p++){let M=h[p],m=a[p],f,b=c[p];if(m.mode===hi.TRIANGLES||m.mode===hi.TRIANGLE_STRIP||m.mode===hi.TRIANGLE_FAN||m.mode===void 0){let E=r.isSkinnedMesh===!0,g=M.hasAttribute("skinIndex")&&M.hasAttribute("skinWeight");E&&g===!1&&console.warn("THREE.GLTFLoader: Missing skinIndex or skinWeight attributes. Skinning disabled."),f=E&&g?new ka(M,b):new ht(M,b),f.isSkinnedMesh===!0&&f.normalizeSkinWeights()}else if(m.mode===hi.LINES)f=new Js(M,b);else if(m.mode===hi.LINE_STRIP)f=new ts(M,b);else if(m.mode===hi.LINE_LOOP)f=new Ha(M,b);else if(m.mode===hi.POINTS)f=new Rn(M,b);else throw new Error("THREE.GLTFLoader: Primitive mode unsupported: "+m.mode);Object.keys(f.geometry.morphAttributes).length>0&&H_(f,r),f.name=t.createUniqueName(r.name||"mesh_"+e),Gi(f,r),m.extensions&&lr(i,f,m),t.assignFinalMaterial(f),d.push(f)}for(let p=0,x=d.length;p<x;p++)t.associations.set(d[p],{meshes:e,primitives:p});if(d.length===1)return r.extensions&&lr(i,d[0],r),d[0];let u=new Et;r.extensions&&lr(i,u,r),t.associations.set(u,{meshes:e});for(let p=0,x=d.length;p<x;p++)u.add(d[p]);return u})}loadCamera(e){let t,n=this.json.cameras[e],i=n[n.type];if(!i){console.warn("THREE.GLTFLoader: Missing camera parameters.");return}return n.type==="perspective"?t=new gn(It.radToDeg(i.yfov),i.aspectRatio||1,i.znear||1,i.zfar||2e6):n.type==="orthographic"&&(t=new ki(-i.xmag,i.xmag,i.ymag,-i.ymag,i.znear,i.zfar)),n.name&&(t.name=this.createUniqueName(n.name)),Gi(t,n),Promise.resolve(t)}loadSkin(e){let t=this.json.skins[e],n=[];for(let i=0,r=t.joints.length;i<r;i++)n.push(this._loadNodeShallow(t.joints[i]));return t.inverseBindMatrices!==void 0?n.push(this.getDependency("accessor",t.inverseBindMatrices)):n.push(null),Promise.all(n).then(function(i){let r=i.pop(),a=i,o=[],l=[];for(let c=0,h=a.length;c<h;c++){let d=a[c];if(d){o.push(d);let u=new yt;r!==null&&u.fromArray(r.array,c*16),l.push(u)}else console.warn('THREE.GLTFLoader: Joint "%s" could not be found.',t.joints[c])}return new Ba(o,l)})}loadAnimation(e){let t=this.json,n=this,i=t.animations[e],r=i.name?i.name:"animation_"+e,a=[],o=[],l=[],c=[],h=[];for(let d=0,u=i.channels.length;d<u;d++){let p=i.channels[d],x=i.samplers[p.sampler],M=p.target,m=M.node,f=i.parameters!==void 0?i.parameters[x.input]:x.input,b=i.parameters!==void 0?i.parameters[x.output]:x.output;M.node!==void 0&&(a.push(this.getDependency("node",m)),o.push(this.getDependency("accessor",f)),l.push(this.getDependency("accessor",b)),c.push(x),h.push(M))}return Promise.all([Promise.all(a),Promise.all(o),Promise.all(l),Promise.all(c),Promise.all(h)]).then(function(d){let u=d[0],p=d[1],x=d[2],M=d[3],m=d[4],f=[];for(let E=0,g=u.length;E<g;E++){let _=u[E],w=p[E],T=x[E],y=M[E],C=m[E];if(_===void 0)continue;_.updateMatrix&&_.updateMatrix();let S=n._createAnimationTracks(_,w,T,y,C);if(S)for(let D=0;D<S.length;D++)f.push(S[D])}let b=new Qs(r,void 0,f);return Gi(b,i),b})}createNodeMesh(e){let t=this.json,n=this,i=t.nodes[e];return i.mesh===void 0?null:n.getDependency("mesh",i.mesh).then(function(r){let a=n._getNodeRef(n.meshCache,i.mesh,r);return i.weights!==void 0&&a.traverse(function(o){if(o.isMesh)for(let l=0,c=i.weights.length;l<c;l++)o.morphTargetInfluences[l]=i.weights[l]}),a})}loadNode(e){let t=this.json,n=this,i=t.nodes[e],r=n._loadNodeShallow(e),a=[],o=i.children||[];for(let c=0,h=o.length;c<h;c++)a.push(n.getDependency("node",o[c]));let l=i.skin===void 0?Promise.resolve(null):n.getDependency("skin",i.skin);return Promise.all([r,Promise.all(a),l]).then(function(c){let h=c[0],d=c[1],u=c[2];u!==null&&h.traverse(function(p){p.isSkinnedMesh&&p.bind(u,W_)});for(let p=0,x=d.length;p<x;p++)h.add(d[p]);if(h.userData.pivot!==void 0&&d.length>0){let p=h.userData.pivot,x=d[0];h.pivot=new U().fromArray(p),h.position.x-=p[0],h.position.y-=p[1],h.position.z-=p[2],x.position.set(0,0,0),delete h.userData.pivot}return h})}_loadNodeShallow(e){let t=this.json,n=this.extensions,i=this;if(this.nodeCache[e]!==void 0)return this.nodeCache[e];let r=t.nodes[e],a=r.name?i.createUniqueName(r.name):"",o=[],l=i._invokeOne(function(c){return c.createNodeMesh&&c.createNodeMesh(e)});return l&&o.push(l),r.camera!==void 0&&o.push(i.getDependency("camera",r.camera).then(function(c){return i._getNodeRef(i.cameraCache,r.camera,c)})),i._invokeAll(function(c){return c.createNodeAttachment&&c.createNodeAttachment(e)}).forEach(function(c){o.push(c)}),this.nodeCache[e]=Promise.all(o).then(function(c){let h;if(r.isBone===!0?h=new Br:c.length>1?h=new Et:c.length===1?h=c[0]:h=new Yt,h!==c[0])for(let d=0,u=c.length;d<u;d++)h.add(c[d]);if(r.name&&(h.userData.name=r.name,h.name=a),Gi(h,r),r.extensions&&lr(n,h,r),r.matrix!==void 0){let d=new yt;d.fromArray(r.matrix),h.applyMatrix4(d)}else r.translation!==void 0&&h.position.fromArray(r.translation),r.rotation!==void 0&&h.quaternion.fromArray(r.rotation),r.scale!==void 0&&h.scale.fromArray(r.scale);if(!i.associations.has(h))i.associations.set(h,{});else if(r.mesh!==void 0&&i.meshCache.refs[r.mesh]>1){let d=i.associations.get(h);i.associations.set(h,{...d})}return i.associations.get(h).nodes=e,h}),this.nodeCache[e]}loadScene(e){let t=this.extensions,n=this.json.scenes[e],i=this,r=new Et;n.name&&(r.name=i.createUniqueName(n.name)),Gi(r,n),n.extensions&&lr(t,r,n);let a=n.nodes||[],o=[];for(let l=0,c=a.length;l<c;l++)o.push(i.getDependency("node",a[l]));return Promise.all(o).then(function(l){for(let h=0,d=l.length;h<d;h++){let u=l[h];u.parent!==null?r.add(Xp(u)):r.add(u)}let c=h=>{let d=new Map;for(let[u,p]of i.associations)(u instanceof Gn||u instanceof hn)&&d.set(u,p);return h.traverse(u=>{let p=i.associations.get(u);p!=null&&d.set(u,p)}),d};return i.associations=c(r),r})}_createAnimationTracks(e,t,n,i,r){let a=[],o=e.name?e.name:e.uuid,l=[];function c(p){p.morphTargetInfluences&&l.push(p.name?p.name:p.uuid)}Is[r.path]===Is.weights?(c(e),e.isGroup&&e.children.forEach(c)):l.push(o);let h;switch(Is[r.path]){case Is.weights:h=is;break;case Is.rotation:h=ss;break;case Is.translation:case Is.scale:h=Es;break;default:n.itemSize===1?h=is:h=Es;break}let d=i.interpolation!==void 0?O_[i.interpolation]:js,u=this._getArrayFromAccessor(n);for(let p=0,x=l.length;p<x;p++){let M=new h(l[p]+"."+Is[r.path],t.array,u,d);i.interpolation==="CUBICSPLINE"&&this._createCubicSplineTrackInterpolant(M),a.push(M)}return a}_getArrayFromAccessor(e){let t=e.array;if(e.normalized){let n=hd(t.constructor),i=new Float32Array(t.length);for(let r=0,a=t.length;r<a;r++)i[r]=t[r]*n;t=i}return t}_createCubicSplineTrackInterpolant(e){e.createInterpolant=function(n){let i=this instanceof ss?ld:qc;return new i(this.times,this.values,this.getValueSize()/3,n)},e.createInterpolant.isInterpolantFactoryMethodGLTFCubicSpline=!0}};function q_(s,e,t){let n=e.attributes,i=new un;if(n.POSITION!==void 0){let o=t.json.accessors[n.POSITION],l=o.min,c=o.max;if(l!==void 0&&c!==void 0){if(i.set(new U(l[0],l[1],l[2]),new U(c[0],c[1],c[2])),o.normalized){let h=hd(la[o.componentType]);i.min.multiplyScalar(h),i.max.multiplyScalar(h)}}else{console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.");return}}else return;let r=e.targets;if(r!==void 0){let o=new U,l=new U;for(let c=0,h=r.length;c<h;c++){let d=r[c];if(d.POSITION!==void 0){let u=t.json.accessors[d.POSITION],p=u.min,x=u.max;if(p!==void 0&&x!==void 0){if(l.setX(Math.max(Math.abs(p[0]),Math.abs(x[0]))),l.setY(Math.max(Math.abs(p[1]),Math.abs(x[1]))),l.setZ(Math.max(Math.abs(p[2]),Math.abs(x[2]))),u.normalized){let M=hd(la[u.componentType]);l.multiplyScalar(M)}o.max(l)}else console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.")}}i.expandByVector(o)}s.boundingBox=i;let a=new Un;i.getCenter(a.center),a.radius=i.min.distanceTo(i.max)/2,s.boundingSphere=a}function Zp(s,e,t){let n=e.attributes,i=[];function r(a,o){return t.getDependency("accessor",a).then(function(l){s.setAttribute(o,l)})}for(let a in n){let o=cd[a]||a.toLowerCase();o in s.attributes||i.push(r(n[a],o))}if(e.indices!==void 0&&!s.index){let a=t.getDependency("accessor",e.indices).then(function(o){s.setIndex(o)});i.push(a)}return Dt.workingColorSpace!==Hn&&"COLOR_0"in n&&console.warn(`THREE.GLTFLoader: Converting vertex colors from "srgb-linear" to "${Dt.workingColorSpace}" not supported.`),Gi(s,e),q_(s,e,t),Promise.all(i).then(function(){return e.targets!==void 0?B_(s,e.targets,t):s})}var Qp={type:"change"},fd={type:"start"},tm={type:"end"},Xc=new Fi,em=new Zn,X_=Math.cos(70*It.DEG2RAD),wn=new U,jn=2*Math.PI,Qt={NONE:-1,ROTATE:0,DOLLY:1,PAN:2,TOUCH_ROTATE:3,TOUCH_PAN:4,TOUCH_DOLLY_PAN:5,TOUCH_DOLLY_ROTATE:6},dd=1e-6,jc=class extends ro{constructor(e,t=null){super(e,t),this.state=Qt.NONE,this.target=new U,this.cursor=new U,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.minTargetRadius=0,this.maxTargetRadius=1/0,this.minPolarAngle=0,this.maxPolarAngle=Math.PI,this.minAzimuthAngle=-1/0,this.maxAzimuthAngle=1/0,this.enableDamping=!1,this.dampingFactor=.05,this.enableZoom=!0,this.zoomSpeed=1,this.enableRotate=!0,this.rotateSpeed=1,this.keyRotateSpeed=1,this.enablePan=!0,this.panSpeed=1,this.screenSpacePanning=!0,this.keyPanSpeed=7,this.zoomToCursor=!1,this.autoRotate=!1,this.autoRotateSpeed=2,this.keys={LEFT:"ArrowLeft",UP:"ArrowUp",RIGHT:"ArrowRight",BOTTOM:"ArrowDown"},this.mouseButtons={LEFT:Ts.ROTATE,MIDDLE:Ts.DOLLY,RIGHT:Ts.PAN},this.touches={ONE:As.ROTATE,TWO:As.DOLLY_PAN},this.target0=this.target.clone(),this.position0=this.object.position.clone(),this.zoom0=this.object.zoom,this._cursorStyle="auto",this._domElementKeyEvents=null,this._lastPosition=new U,this._lastQuaternion=new nn,this._lastTargetPosition=new U,this._quat=new nn().setFromUnitVectors(e.up,new U(0,1,0)),this._quatInverse=this._quat.clone().invert(),this._spherical=new Jr,this._sphericalDelta=new Jr,this._scale=1,this._panOffset=new U,this._rotateStart=new Ge,this._rotateEnd=new Ge,this._rotateDelta=new Ge,this._panStart=new Ge,this._panEnd=new Ge,this._panDelta=new Ge,this._dollyStart=new Ge,this._dollyEnd=new Ge,this._dollyDelta=new Ge,this._dollyDirection=new U,this._mouse=new Ge,this._performCursorZoom=!1,this._pointers=[],this._pointerPositions={},this._controlActive=!1,this._onPointerMove=K_.bind(this),this._onPointerDown=j_.bind(this),this._onPointerUp=Y_.bind(this),this._onContextMenu=nM.bind(this),this._onMouseWheel=$_.bind(this),this._onKeyDown=Q_.bind(this),this._onTouchStart=eM.bind(this),this._onTouchMove=tM.bind(this),this._onMouseDown=J_.bind(this),this._onMouseMove=Z_.bind(this),this._interceptControlDown=iM.bind(this),this._interceptControlUp=sM.bind(this),this.domElement!==null&&this.connect(this.domElement),this.update()}set cursorStyle(e){this._cursorStyle=e,e==="grab"?this.domElement.style.cursor="grab":this.domElement.style.cursor="auto"}get cursorStyle(){return this._cursorStyle}connect(e){super.connect(e),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerUp),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.getRootNode().addEventListener("keydown",this._interceptControlDown,{passive:!0,capture:!0}),this.domElement.style.touchAction="none"}disconnect(){this.state=Qt.NONE,this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerUp),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.stopListenToKeyEvents();let e=this.domElement.getRootNode();e.removeEventListener("keydown",this._interceptControlDown,{capture:!0}),e.removeEventListener("keyup",this._interceptControlUp,{capture:!0}),this._controlActive=!1,this._pointers.length=0,this._pointerPositions={},this.domElement.style.touchAction="",this.domElement.style.cursor="auto"}dispose(){this.disconnect()}getPolarAngle(){return this._spherical.phi}getAzimuthalAngle(){return this._spherical.theta}getDistance(){return this.object.position.distanceTo(this.target)}listenToKeyEvents(e){e.addEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=e}stopListenToKeyEvents(){this._domElementKeyEvents!==null&&(this._domElementKeyEvents.removeEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=null)}saveState(){this.target0.copy(this.target),this.position0.copy(this.object.position),this.zoom0=this.object.zoom}reset(){this.target.copy(this.target0),this.object.position.copy(this.position0),this.object.zoom=this.zoom0,this.object.updateProjectionMatrix(),this.dispatchEvent(Qp),this.update(),this.state=Qt.NONE}pan(e,t){this._pan(e,t),this.update()}dollyIn(e){this._dollyIn(e),this.update()}dollyOut(e){this._dollyOut(e),this.update()}rotateLeft(e){this._rotateLeft(e),this.update()}rotateUp(e){this._rotateUp(e),this.update()}update(e=null){let t=this.object.position;wn.copy(t).sub(this.target),wn.applyQuaternion(this._quat),this._spherical.setFromVector3(wn),this.autoRotate&&this.state===Qt.NONE&&this._rotateLeft(this._getAutoRotationAngle(e)),this.enableDamping?(this._spherical.theta+=this._sphericalDelta.theta*this.dampingFactor,this._spherical.phi+=this._sphericalDelta.phi*this.dampingFactor):(this._spherical.theta+=this._sphericalDelta.theta,this._spherical.phi+=this._sphericalDelta.phi);let n=this.minAzimuthAngle,i=this.maxAzimuthAngle;isFinite(n)&&isFinite(i)&&(n<-Math.PI?n+=jn:n>Math.PI&&(n-=jn),i<-Math.PI?i+=jn:i>Math.PI&&(i-=jn),n<=i?this._spherical.theta=Math.max(n,Math.min(i,this._spherical.theta)):this._spherical.theta=this._spherical.theta>(n+i)/2?Math.max(n,this._spherical.theta):Math.min(i,this._spherical.theta)),this._spherical.phi=Math.max(this.minPolarAngle,Math.min(this.maxPolarAngle,this._spherical.phi)),this._spherical.makeSafe(),this.enableDamping===!0?this.target.addScaledVector(this._panOffset,this.dampingFactor):this.target.add(this._panOffset),this.target.sub(this.cursor),this.target.clampLength(this.minTargetRadius,this.maxTargetRadius),this.target.add(this.cursor);let r=!1;if(this.zoomToCursor&&this._performCursorZoom||this.object.isOrthographicCamera)this._spherical.radius=this._clampDistance(this._spherical.radius);else{let a=this._spherical.radius;this._spherical.radius=this._clampDistance(this._spherical.radius*this._scale),r=a!=this._spherical.radius}if(wn.setFromSpherical(this._spherical),wn.applyQuaternion(this._quatInverse),t.copy(this.target).add(wn),this.object.lookAt(this.target),this.enableDamping===!0?(this._sphericalDelta.theta*=1-this.dampingFactor,this._sphericalDelta.phi*=1-this.dampingFactor,this._panOffset.multiplyScalar(1-this.dampingFactor)):(this._sphericalDelta.set(0,0,0),this._panOffset.set(0,0,0)),this.zoomToCursor&&this._performCursorZoom){let a=null;if(this.object.isPerspectiveCamera){let o=wn.length();a=this._clampDistance(o*this._scale);let l=o-a;this.object.position.addScaledVector(this._dollyDirection,l),this.object.updateMatrixWorld(),r=!!l}else if(this.object.isOrthographicCamera){let o=new U(this._mouse.x,this._mouse.y,0);o.unproject(this.object);let l=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),this.object.updateProjectionMatrix(),r=l!==this.object.zoom;let c=new U(this._mouse.x,this._mouse.y,0);c.unproject(this.object),this.object.position.sub(c).add(o),this.object.updateMatrixWorld(),a=wn.length()}else console.warn("WARNING: OrbitControls.js encountered an unknown camera type - zoom to cursor disabled."),this.zoomToCursor=!1;a!==null&&(this.screenSpacePanning?this.target.set(0,0,-1).transformDirection(this.object.matrix).multiplyScalar(a).add(this.object.position):(Xc.origin.copy(this.object.position),Xc.direction.set(0,0,-1).transformDirection(this.object.matrix),Math.abs(this.object.up.dot(Xc.direction))<X_?this.object.lookAt(this.target):(em.setFromNormalAndCoplanarPoint(this.object.up,this.target),Xc.intersectPlane(em,this.target))))}else if(this.object.isOrthographicCamera){let a=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),a!==this.object.zoom&&(this.object.updateProjectionMatrix(),r=!0)}return this._scale=1,this._performCursorZoom=!1,r||this._lastPosition.distanceToSquared(this.object.position)>dd||8*(1-this._lastQuaternion.dot(this.object.quaternion))>dd||this._lastTargetPosition.distanceToSquared(this.target)>dd?(this.dispatchEvent(Qp),this._lastPosition.copy(this.object.position),this._lastQuaternion.copy(this.object.quaternion),this._lastTargetPosition.copy(this.target),!0):!1}_getAutoRotationAngle(e){return e!==null?jn/60*this.autoRotateSpeed*e:jn/60/60*this.autoRotateSpeed}_getZoomScale(e){let t=Math.abs(e*.01);return Math.pow(.95,this.zoomSpeed*t)}_rotateLeft(e){this._sphericalDelta.theta-=e}_rotateUp(e){this._sphericalDelta.phi-=e}_panLeft(e,t){wn.setFromMatrixColumn(t,0),wn.multiplyScalar(-e),this._panOffset.add(wn)}_panUp(e,t){this.screenSpacePanning===!0?wn.setFromMatrixColumn(t,1):(wn.setFromMatrixColumn(t,0),wn.crossVectors(this.object.up,wn)),wn.multiplyScalar(e),this._panOffset.add(wn)}_pan(e,t){let n=this.domElement;if(this.object.isPerspectiveCamera){let i=this.object.position;wn.copy(i).sub(this.target);let r=wn.length();r*=Math.tan(this.object.fov/2*Math.PI/180),this._panLeft(2*e*r/n.clientHeight,this.object.matrix),this._panUp(2*t*r/n.clientHeight,this.object.matrix)}else this.object.isOrthographicCamera?(this._panLeft(e*(this.object.right-this.object.left)/this.object.zoom/n.clientWidth,this.object.matrix),this._panUp(t*(this.object.top-this.object.bottom)/this.object.zoom/n.clientHeight,this.object.matrix)):(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - pan disabled."),this.enablePan=!1)}_dollyOut(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale/=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_dollyIn(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale*=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_updateZoomParameters(e,t){if(!this.zoomToCursor)return;this._performCursorZoom=!0;let n=this.domElement.getBoundingClientRect(),i=e-n.left,r=t-n.top,a=n.width,o=n.height;this._mouse.x=i/a*2-1,this._mouse.y=-(r/o)*2+1,this._dollyDirection.set(this._mouse.x,this._mouse.y,1).unproject(this.object).sub(this.object.position).normalize()}_clampDistance(e){return Math.max(this.minDistance,Math.min(this.maxDistance,e))}_handleMouseDownRotate(e){this._rotateStart.set(e.clientX,e.clientY)}_handleMouseDownDolly(e){this._updateZoomParameters(e.clientX,e.clientX),this._dollyStart.set(e.clientX,e.clientY)}_handleMouseDownPan(e){this._panStart.set(e.clientX,e.clientY)}_handleMouseMoveRotate(e){this._rotateEnd.set(e.clientX,e.clientY),this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(jn*this._rotateDelta.x/t.clientHeight),this._rotateUp(jn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd),this.update()}_handleMouseMoveDolly(e){this._dollyEnd.set(e.clientX,e.clientY),this._dollyDelta.subVectors(this._dollyEnd,this._dollyStart),this._dollyDelta.y>0?this._dollyOut(this._getZoomScale(this._dollyDelta.y)):this._dollyDelta.y<0&&this._dollyIn(this._getZoomScale(this._dollyDelta.y)),this._dollyStart.copy(this._dollyEnd),this.update()}_handleMouseMovePan(e){this._panEnd.set(e.clientX,e.clientY),this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd),this.update()}_handleMouseWheel(e){this._updateZoomParameters(e.clientX,e.clientY),e.deltaY<0?this._dollyIn(this._getZoomScale(e.deltaY)):e.deltaY>0&&this._dollyOut(this._getZoomScale(e.deltaY)),this.update()}_handleKeyDown(e){let t=!1;switch(e.code){case this.keys.UP:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(jn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,this.keyPanSpeed),t=!0;break;case this.keys.BOTTOM:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(-jn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,-this.keyPanSpeed),t=!0;break;case this.keys.LEFT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(jn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(this.keyPanSpeed,0),t=!0;break;case this.keys.RIGHT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(-jn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(-this.keyPanSpeed,0),t=!0;break}t&&(e.preventDefault(),this.update())}_handleTouchStartRotate(e){if(this._pointers.length===1)this._rotateStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._rotateStart.set(n,i)}}_handleTouchStartPan(e){if(this._pointers.length===1)this._panStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panStart.set(n,i)}}_handleTouchStartDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyStart.set(0,r)}_handleTouchStartDollyPan(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enablePan&&this._handleTouchStartPan(e)}_handleTouchStartDollyRotate(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enableRotate&&this._handleTouchStartRotate(e)}_handleTouchMoveRotate(e){if(this._pointers.length==1)this._rotateEnd.set(e.pageX,e.pageY);else{let n=this._getSecondPointerPosition(e),i=.5*(e.pageX+n.x),r=.5*(e.pageY+n.y);this._rotateEnd.set(i,r)}this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(jn*this._rotateDelta.x/t.clientHeight),this._rotateUp(jn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd)}_handleTouchMovePan(e){if(this._pointers.length===1)this._panEnd.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panEnd.set(n,i)}this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd)}_handleTouchMoveDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyEnd.set(0,r),this._dollyDelta.set(0,Math.pow(this._dollyEnd.y/this._dollyStart.y,this.zoomSpeed)),this._dollyOut(this._dollyDelta.y),this._dollyStart.copy(this._dollyEnd);let a=(e.pageX+t.x)*.5,o=(e.pageY+t.y)*.5;this._updateZoomParameters(a,o)}_handleTouchMoveDollyPan(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enablePan&&this._handleTouchMovePan(e)}_handleTouchMoveDollyRotate(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enableRotate&&this._handleTouchMoveRotate(e)}_addPointer(e){this._pointers.push(e.pointerId)}_removePointer(e){delete this._pointerPositions[e.pointerId];for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId){this._pointers.splice(t,1);return}}_isTrackingPointer(e){for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId)return!0;return!1}_trackPointer(e){let t=this._pointerPositions[e.pointerId];t===void 0&&(t=new Ge,this._pointerPositions[e.pointerId]=t),t.set(e.pageX,e.pageY)}_getSecondPointerPosition(e){let t=e.pointerId===this._pointers[0]?this._pointers[1]:this._pointers[0];return this._pointerPositions[t]}_customWheelEvent(e){let t=e.deltaMode,n={clientX:e.clientX,clientY:e.clientY,deltaY:e.deltaY};switch(t){case 1:n.deltaY*=16;break;case 2:n.deltaY*=100;break}return e.ctrlKey&&!this._controlActive&&(n.deltaY*=10),n}};function j_(s){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(s.pointerId),this.domElement.ownerDocument.addEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.addEventListener("pointerup",this._onPointerUp)),!this._isTrackingPointer(s)&&(this._addPointer(s),s.pointerType==="touch"?this._onTouchStart(s):this._onMouseDown(s),this._cursorStyle==="grab"&&(this.domElement.style.cursor="grabbing")))}function K_(s){this.enabled!==!1&&(s.pointerType==="touch"?this._onTouchMove(s):this._onMouseMove(s))}function Y_(s){switch(this._removePointer(s),this._pointers.length){case 0:this.domElement.releasePointerCapture(s.pointerId),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.dispatchEvent(tm),this.state=Qt.NONE,this._cursorStyle==="grab"&&(this.domElement.style.cursor="grab");break;case 1:let e=this._pointers[0],t=this._pointerPositions[e];this._onTouchStart({pointerId:e,pageX:t.x,pageY:t.y});break}}function J_(s){let e;switch(s.button){case 0:e=this.mouseButtons.LEFT;break;case 1:e=this.mouseButtons.MIDDLE;break;case 2:e=this.mouseButtons.RIGHT;break;default:e=-1}switch(e){case Ts.DOLLY:if(this.enableZoom===!1)return;this._handleMouseDownDolly(s),this.state=Qt.DOLLY;break;case Ts.ROTATE:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Qt.PAN}else{if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Qt.ROTATE}break;case Ts.PAN:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Qt.ROTATE}else{if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Qt.PAN}break;default:this.state=Qt.NONE}this.state!==Qt.NONE&&this.dispatchEvent(fd)}function Z_(s){switch(this.state){case Qt.ROTATE:if(this.enableRotate===!1)return;this._handleMouseMoveRotate(s);break;case Qt.DOLLY:if(this.enableZoom===!1)return;this._handleMouseMoveDolly(s);break;case Qt.PAN:if(this.enablePan===!1)return;this._handleMouseMovePan(s);break}}function $_(s){this.enabled===!1||this.enableZoom===!1||this.state!==Qt.NONE||(s.preventDefault(),this.dispatchEvent(fd),this._handleMouseWheel(this._customWheelEvent(s)),this.dispatchEvent(tm))}function Q_(s){this.enabled!==!1&&this._handleKeyDown(s)}function eM(s){switch(this._trackPointer(s),this._pointers.length){case 1:switch(this.touches.ONE){case As.ROTATE:if(this.enableRotate===!1)return;this._handleTouchStartRotate(s),this.state=Qt.TOUCH_ROTATE;break;case As.PAN:if(this.enablePan===!1)return;this._handleTouchStartPan(s),this.state=Qt.TOUCH_PAN;break;default:this.state=Qt.NONE}break;case 2:switch(this.touches.TWO){case As.DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchStartDollyPan(s),this.state=Qt.TOUCH_DOLLY_PAN;break;case As.DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchStartDollyRotate(s),this.state=Qt.TOUCH_DOLLY_ROTATE;break;default:this.state=Qt.NONE}break;default:this.state=Qt.NONE}this.state!==Qt.NONE&&this.dispatchEvent(fd)}function tM(s){switch(this._trackPointer(s),this.state){case Qt.TOUCH_ROTATE:if(this.enableRotate===!1)return;this._handleTouchMoveRotate(s),this.update();break;case Qt.TOUCH_PAN:if(this.enablePan===!1)return;this._handleTouchMovePan(s),this.update();break;case Qt.TOUCH_DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchMoveDollyPan(s),this.update();break;case Qt.TOUCH_DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchMoveDollyRotate(s),this.update();break;default:this.state=Qt.NONE}}function nM(s){this.enabled!==!1&&s.preventDefault()}function iM(s){s.key==="Control"&&(this._controlActive=!0,this.domElement.getRootNode().addEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}function sM(s){s.key==="Control"&&(this._controlActive=!1,this.domElement.getRootNode().removeEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}var ca=(function(){var s="b9H79Tebbbe8Fv9Gbb9Gvuuuuueu9Giuuub9Geueu9Giuuueuixkbeeeddddillviebeoweuecj:Gdkr;Neqo9TW9T9VV95dbH9F9F939H79T9F9J9H229F9Jt9VV7bb8A9TW79O9V9Wt9F9KW9J9V9KW9wWVtW949c919M9MWVbeY9TW79O9V9Wt9F9KW9J9V9KW69U9KW949c919M9MWVbdE9TW79O9V9Wt9F9KW9J9V9KW69U9KW949tWG91W9U9JWbiL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9p9JtblK9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9r919HtbvL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVT949WboY9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVJ9V29VVbrl79IV9Rbwq:VZkdbk:XYi5ud9:du8Jjjjjbcj;kb9Rgv8Kjjjjbc9:hodnalTmbcuhoaiRbbgrc;WeGc:Ge9hmbarcsGgwce0mbc9:hoalcufadcd4cbawEgDadfgrcKcaawEgqaraq0Egk6mbaicefhxcj;abad9Uc;WFbGcjdadca0EhmaialfgPar9Rgoadfhsavaoadz:jjjjbgzceVhHcbhOdndninaeaO9nmeaPax9RaD6mdamaeaO9RaOamfgoae6EgAcsfglc9WGhCabaOad2fhXaAcethQaxaDfhiaOaeaoaeao6E9RhLalcl4cifcd4hKazcj;cbfaAfhYcbh8AazcjdfhEaHh3incbh5dnawTmbaxa8Acd4fRbbh5kcbh8Eazcj;cbfhqinaih8Fdndndndna5a8Ecet4ciGgoc9:fPdebdkaPa8F9RaA6mrazcj;cbfa8EaA2fa8FaAz:jjjjb8Aa8FaAfhixdkazcj;cbfa8EaA2fcbaAz:kjjjb8Aa8FhixekaPa8F9RaK6mva8FaKfhidnaCTmbaPai9RcK6mbaocdtc:q:G:cjbfcj:G:cjbawEhaczhrcbhlinargoc9Wfghaqfhrdndndndndndnaaa8Fahco4fRbbalcoG4ciGcdtfydbPDbedvivvvlvkar9cb83bwar9cb83bbxlkarcbaiRbdai8Xbb9c:c:qj:bw9:9c:q;c1:I1e:d9c:b:c:e1z9:gg9cjjjjjz:dg8J9qE86bbaqaofgrcGfcbaicdfa8J9c8N1:NfghRbbag9cjjjjjw:dg8J9qE86bbarcVfcbaha8J9c8M1:NfghRbbag9cjjjjjl:dg8J9qE86bbarc7fcbaha8J9c8L1:NfghRbbag9cjjjjjd:dg8J9qE86bbarctfcbaha8J9c8K1:NfghRbbag9cjjjjje:dg8J9qE86bbarc91fcbaha8J9c8J1:NfghRbbag9cjjjj;ab:dg8J9qE86bbarc4fcbaha8J9cg1:NfghRbbag9cjjjja:dg8J9qE86bbarc93fcbaha8J9ch1:NfghRbbag9cjjjjz:dgg9qE86bbarc94fcbahag9ca1:NfghRbbai8Xbe9c:c:qj:bw9:9c:q;c1:I1e:d9c:b:c:e1z9:gg9cjjjjjz:dg8J9qE86bbarc95fcbaha8J9c8N1:NfgiRbbag9cjjjjjw:dg8J9qE86bbarc96fcbaia8J9c8M1:NfgiRbbag9cjjjjjl:dg8J9qE86bbarc97fcbaia8J9c8L1:NfgiRbbag9cjjjjjd:dg8J9qE86bbarc98fcbaia8J9c8K1:NfgiRbbag9cjjjjje:dg8J9qE86bbarc99fcbaia8J9c8J1:NfgiRbbag9cjjjj;ab:dg8J9qE86bbarc9:fcbaia8J9cg1:NfgiRbbag9cjjjja:dg8J9qE86bbarcufcbaia8J9ch1:NfgiRbbag9cjjjjz:dgg9qE86bbaiag9ca1:NfhixikaraiRblaiRbbghco4g8Ka8KciSg8KE86bbaqaofgrcGfaiclfa8Kfg8KRbbahcl4ciGg8La8LciSg8LE86bbarcVfa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc7fa8Ka8Lfg8KRbbahciGghahciSghE86bbarctfa8Kahfg8KRbbaiRbeghco4g8La8LciSg8LE86bbarc91fa8Ka8Lfg8KRbbahcl4ciGg8La8LciSg8LE86bbarc4fa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc93fa8Ka8Lfg8KRbbahciGghahciSghE86bbarc94fa8Kahfg8KRbbaiRbdghco4g8La8LciSg8LE86bbarc95fa8Ka8Lfg8KRbbahcl4ciGg8La8LciSg8LE86bbarc96fa8Ka8Lfg8KRbbahcd4ciGg8La8LciSg8LE86bbarc97fa8Ka8Lfg8KRbbahciGghahciSghE86bbarc98fa8KahfghRbbaiRbigico4g8Ka8KciSg8KE86bbarc99faha8KfghRbbaicl4ciGg8Ka8KciSg8KE86bbarc9:faha8KfghRbbaicd4ciGg8Ka8KciSg8KE86bbarcufaha8KfgrRbbaiciGgiaiciSgiE86bbaraifhixdkaraiRbwaiRbbghcl4g8Ka8KcsSg8KE86bbaqaofgrcGfaicwfa8Kfg8KRbbahcsGghahcsSghE86bbarcVfa8KahfghRbbaiRbeg8Kcl4g8La8LcsSg8LE86bbarc7faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarctfaha8KfghRbbaiRbdg8Kcl4g8La8LcsSg8LE86bbarc91faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc4faha8KfghRbbaiRbig8Kcl4g8La8LcsSg8LE86bbarc93faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc94faha8KfghRbbaiRblg8Kcl4g8La8LcsSg8LE86bbarc95faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc96faha8KfghRbbaiRbvg8Kcl4g8La8LcsSg8LE86bbarc97faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc98faha8KfghRbbaiRbog8Kcl4g8La8LcsSg8LE86bbarc99faha8LfghRbba8KcsGg8Ka8KcsSg8KE86bbarc9:faha8KfghRbbaiRbrgicl4g8Ka8KcsSg8KE86bbarcufaha8KfgrRbbaicsGgiaicsSgiE86bbaraifhixekarai8Pbw83bwarai8Pbb83bbaiczfhikdnaoaC9pmbalcdfhlaoczfhraPai9RcL0mekkaoaC6moaimexokaCmva8FTmvkaqaAfhqa8Ecefg8Ecl9hmbkdndndndnawTmbasa8Acd4fRbbgociGPlbedrbkaATmdaza8Afh8Fazcj;cbfhhcbh8EaEhaina8FRbbhraahocbhlinaoahalfRbbgqce4cbaqceG9R7arfgr86bbaoadfhoaAalcefgl9hmbkaacefhaa8Fcefh8FahaAfhha8Ecefg8Ecl9hmbxikkaATmeaza8Afhaazcj;cbfhhcbhoceh8EaYh8FinaEaofhlaa8Vbbhrcbhoinala8FaofRbbcwtahaofRbbgqVc;:FiGce4cbaqceG9R7arfgr87bbaladfhlaLaocefgofmbka8FaQfh8FcdhoaacdfhaahaQfhha8EceGhlcbh8EalmbxdkkaATmbaocl4h8Eaza8AfRbbhqcwhoa3hlinalRbbaotaqVhqalcefhlaocwfgoca9hmbkcbhhaEh8FaYhainazcj;cbfahfRbbhrcwhoaahlinalRbbaotarVhralaAfhlaocwfgoca9hmbkara8E94aq7hqcbhoa8Fhlinalaqao486bbalcefhlaocwfgoca9hmbka8Fadfh8FaacefhaahcefghaA9hmbkkaEclfhEa3clfh3a8Aclfg8Aad6mbkaXazcjdfaAad2z:jjjjb8AazazcjdfaAcufad2fadz:jjjjb8AaAaOfhOaihxaimbkc9:hoxdkcbc99aPax9RakSEhoxekc9:hokavcj;kbf8Kjjjjbaok:ysezu8Jjjjjbc;ae9Rgv8Kjjjjbc9:hodnalaeci9UgrcHf6mbcuhoaiRbbgwc;WeGc;Ge9hmbawcsGgDce0mbavc;abfcFecjez:kjjjb8Aav9cu83iUav9cu83i8Wav9cu83iyav9cu83iaav9cu83iKav9cu83izav9cu83iwav9cu83ibaialfc9WfhqaicefgwarfhldnaeTmbcmcsaDceSEhkcbhxcbhmcbhrcbhicbhoindnalaq9nmbc9:hoxikdndnawRbbgDc;Ve0mbavc;abfaoaDcu7gPcl4fcsGcitfgsydlhzasydbhHdndnaDcsGgsak9pmbavaiaPfcsGcdtfydbaxasEhDaxasTgOfhxxekdndnascsSmbcehOasc987asamffcefhDxekalcefhDal8SbbgscFeGhPdndnascu9mmbaDhlxekalcvfhlaPcFbGhPcrhsdninaD8SbbgOcFbGastaPVhPaOcu9kmeaDcefhDascrfgsc8J9hmbxdkkaDcefhlkcehOaPce4cbaPceG9R7amfhDkaDhmkavc;abfaocitfgsaDBdbasazBdlavaicdtfaDBdbavc;abfaocefcsGcitfgsaHBdbasaDBdlaocdfhoaOaifhidnadcd9hmbabarcetfgsaH87ebasclfaD87ebascdfaz87ebxdkabarcdtfgsaHBdbascwfaDBdbasclfazBdbxekdnaDcpe0mbavaiaqaDcsGfRbbgscl4gP9RcsGcdtfydbaxcefgOaPEhDavaias9RcsGcdtfydbaOaPTgzfgOascsGgPEhsaPThPdndnadcd9hmbabarcetfgHax87ebaHclfas87ebaHcdfaD87ebxekabarcdtfgHaxBdbaHcwfasBdbaHclfaDBdbkavaicdtfaxBdbavc;abfaocitfgHaDBdbaHaxBdlavaicefgicsGcdtfaDBdbavc;abfaocefcsGcitfgHasBdbaHaDBdlavaiazfgicsGcdtfasBdbavc;abfaocdfcsGcitfgDaxBdbaDasBdlaocifhoaiaPfhiaOaPfhxxekaxcbalRbbgsEgHaDc;:eSgDfhOascsGhAdndnascl4gCmbaOcefhzxekaOhzavaiaC9RcsGcdtfydbhOkdndnaAmbazcefhxxekazhxavaias9RcsGcdtfydbhzkdndnaDTmbalcefhDxekalcdfhDal8SbegPcFeGhsdnaPcu9kmbalcofhHascFbGhscrhldninaD8SbbgPcFbGaltasVhsaPcu9kmeaDcefhDalcrfglc8J9hmbkaHhDxekaDcefhDkasce4cbasceG9R7amfgmhHkdndnaCcsSmbaDhsxekaDcefhsaD8SbbglcFeGhPdnalcu9kmbaDcvfhOaPcFbGhPcrhldninas8SbbgDcFbGaltaPVhPaDcu9kmeascefhsalcrfglc8J9hmbkaOhsxekascefhskaPce4cbaPceG9R7amfgmhOkdndnaAcsSmbashlxekascefhlas8SbbgDcFeGhPdnaDcu9kmbascvfhzaPcFbGhPcrhDdninal8SbbgscFbGaDtaPVhPascu9kmealcefhlaDcrfgDc8J9hmbkazhlxekalcefhlkaPce4cbaPceG9R7amfgmhzkdndnadcd9hmbabarcetfgDaH87ebaDclfaz87ebaDcdfaO87ebxekabarcdtfgDaHBdbaDcwfazBdbaDclfaOBdbkavc;abfaocitfgDaOBdbaDaHBdlavaicdtfaHBdbavc;abfaocefcsGcitfgDazBdbaDaOBdlavaicefgicsGcdtfaOBdbavc;abfaocdfcsGcitfgDaHBdbaDazBdlavaiaCTaCcsSVfgicsGcdtfazBdbaiaATaAcsSVfhiaocifhokawcefhwaocsGhoaicsGhiarcifgrae6mbkkcbc99alaqSEhokavc;aef8Kjjjjbaok:clevu8Jjjjjbcz9Rhvdnalaecvf9pmbc9:skdnaiRbbc;:eGc;qeSmbcuskav9cb83iwaicefhoaialfc98fhrdnaeTmbdnadcdSmbcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcdtfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgiBdbalaiBdbawcefgwae9hmbxdkkcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcetfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgi87ebalaiBdbawcefgwae9hmbkkcbc99aoarSEk:Lvoeue99dud99eud99dndnadcl9hmbaeTmeindndnabcdfgd8Sbb:Yab8Sbbgi:Ygl:l:tabcefgv8Sbbgo:Ygr:l:tgwJbb;:9cawawNJbbbbawawJbbbb9GgDEgq:mgkaqaicb9iEalMgwawNakaqaocb9iEarMgqaqNMM:r:vglNJbbbZJbbb:;aDEMgr:lJbbb9p9DTmbar:Ohixekcjjjj94hikadai86bbdndnaqalNJbbbZJbbb:;aqJbbbb9GEMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkavad86bbdndnawalNJbbbZJbbb:;awJbbbb9GEMgw:lJbbb9p9DTmbaw:Ohdxekcjjjj94hdkabad86bbabclfhbaecufgembxdkkaeTmbindndnabclfgd8Ueb:Yab8Uebgi:Ygl:l:tabcdfgv8Uebgo:Ygr:l:tgwJb;:FSawawNJbbbbawawJbbbb9GgDEgq:mgkaqaicb9iEalMgwawNakaqaocb9iEarMgqaqNMM:r:vglNJbbbZJbbb:;aDEMgr:lJbbb9p9DTmbar:Ohixekcjjjj94hikadai87ebdndnaqalNJbbbZJbbb:;aqJbbbb9GEMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkavad87ebdndnawalNJbbbZJbbb:;awJbbbb9GEMgw:lJbbb9p9DTmbaw:Ohdxekcjjjj94hdkabad87ebabcwfhbaecufgembkkk:4ioiue99dud99dud99dnaeTmbcbhiabhlindndnal8Uebgv:YgoJ:ji:1Salcof8UebgrciVgw:Y:vgDNJbbbZJbbb:;avcu9kEMgq:lJbbb9p9DTmbaq:Ohkxekcjjjj94hkkalclf8Uebhvalcdf8UebhxalarcefciGcetfak87ebdndnax:YgqaDNJbbbZJbbb:;axcu9kEMgm:lJbbb9p9DTmbam:Ohxxekcjjjj94hxkabaiarciGgkfcd7cetfax87ebdndnav:YgmaDNJbbbZJbbb:;avcu9kEMgP:lJbbb9p9DTmbaP:Ohvxekcjjjj94hvkalarcufciGcetfav87ebdndnawaw2:ZgPaPMaoaoN:taqaqN:tamamN:tgoJbbbbaoJbbbb9GE:raDNJbbbZMgD:lJbbb9p9DTmbaD:Ohrxekcjjjj94hrkalakcetfar87ebalcwfhlaiclfhiaecufgembkkk9mbdnadcd4ae2gdTmbinababydbgecwtcw91:Yaece91cjjj98Gcjjj;8if::NUdbabclfhbadcufgdmbkkk:Tvirud99eudndnadcl9hmbaeTmeindndnabRbbgiabcefgl8Sbbgvabcdfgo8Sbbgrf9R:YJbbuJabcifgwRbbgdce4adVgDcd4aDVgDcl4aDVgD:Z:vgqNJbbbZMgk:lJbbb9p9DTmbak:Ohxxekcjjjj94hxkaoax86bbdndnaraif:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohoxekcjjjj94hokalao86bbdndnavaifar9R:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohixekcjjjj94hikabai86bbdndnaDadcetGadceGV:ZaqNJbbbZMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkawad86bbabclfhbaecufgembxdkkaeTmbindndnab8Vebgiabcdfgl8Uebgvabclfgo8Uebgrf9R:YJbFu9habcofgw8Vebgdce4adVgDcd4aDVgDcl4aDVgDcw4aDVgD:Z:vgqNJbbbZMgk:lJbbb9p9DTmbak:Ohxxekcjjjj94hxkaoax87ebdndnaraif:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohoxekcjjjj94hokalao87ebdndnavaifar9R:YaqNJbbbZMgk:lJbbb9p9DTmbak:Ohixekcjjjj94hikabai87ebdndnaDadcetGadceGV:ZaqNJbbbZMgq:lJbbb9p9DTmbaq:Ohdxekcjjjj94hdkawad87ebabcwfhbaecufgembkkk9teiucbcbyd:K:G:cjbgeabcifc98GfgbBd:K:G:cjbdndnabZbcztgd9nmbcuhiabad9RcFFifcz4nbcuSmekaehikaik;LeeeudndnaeabVciGTmbabhixekdndnadcz9pmbabhixekabhiinaiaeydbBdbaiclfaeclfydbBdbaicwfaecwfydbBdbaicxfaecxfydbBdbaeczfheaiczfhiadc9Wfgdcs0mbkkadcl6mbinaiaeydbBdbaeclfheaiclfhiadc98fgdci0mbkkdnadTmbinaiaeRbb86bbaicefhiaecefheadcufgdmbkkabk;aeedudndnabciGTmbabhixekaecFeGc:b:c:ew2hldndnadcz9pmbabhixekabhiinaialBdbaicxfalBdbaicwfalBdbaiclfalBdbaiczfhiadc9Wfgdcs0mbkkadcl6mbinaialBdbaiclfhiadc98fgdci0mbkkdnadTmbinaiae86bbaicefhiadcufgdmbkkabkk83dbcj:Gdk8Kbbbbdbbblbbbwbbbbbbbebbbdbbblbbbwbbbbc:K:Gdkl8W:qbb",e="b9H79TebbbeKl9Gbb9Gvuuuuueu9Giuuub9Geueuixkbbebeeddddilve9Weeeviebeoweuecj:Gdkr;Neqo9TW9T9VV95dbH9F9F939H79T9F9J9H229F9Jt9VV7bb8A9TW79O9V9Wt9F9KW9J9V9KW9wWVtW949c919M9MWVbdY9TW79O9V9Wt9F9KW9J9V9KW69U9KW949c919M9MWVblE9TW79O9V9Wt9F9KW9J9V9KW69U9KW949tWG91W9U9JWbvL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9p9JtboK9TW79O9V9Wt9F9KW9J9V9KWS9P2tWV9r919HtbrL9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVT949WbwY9TW79O9V9Wt9F9KW9J9V9KWS9P2tWVJ9V29VVbDl79IV9Rbqq:W9Dklbzik94evu8Jjjjjbcz9Rhbcbheincbhdcbhiinabcwfadfaicjuaead4ceGglE86bbaialfhiadcefgdcw9hmbkaeai86b:q:W:cjbaecitab8Piw83i:q:G:cjbaecefgecjd9hmbkk:JBl8Aud97dur978Jjjjjbcj;kb9Rgv8Kjjjjbc9:hodnalTmbcuhoaiRbbgrc;WeGc:Ge9hmbarcsGgwce0mbc9:hoalcufadcd4cbawEgDadfgrcKcaawEgqaraq0Egk6mbaialfgxar9RhodnadTgmmbavaoad;8qbbkaicefhPcj;abad9Uc;WFbGcjdadca0EhsdndndnadTmbaoadfhzcbhHinaeaH9nmdaxaP9RaD6miabaHad2fhOaPaDfhAasaeaH9RaHasfae6EgCcsfgocl4cifcd4hXavcj;cbfaoc9WGgQcetfhLavcj;cbfaQci2fhKavcj;cbfaQfhYcbh8Aaoc;ab6hEincbh3dnawTmbaPa8Acd4fRbbh3kcbh5avcj;cbfh8Eindndndndna3a5cet4ciGgoc9:fPdebdkaxaA9RaQ6mwdnaQTmbavcj;cbfa5aQ2faAaQ;8qbbkaAaCfhAxdkaQTmeavcj;cbfa5aQ2fcbaQ;8kbxekaxaA9RaX6moaoclVcbawEhraAaXfhocbhidnaEmbaxao9Rc;Gb6mbcbhlina8EalfhidndndndndndnaAalco4fRbbgqciGarfPDbedibledibkaipxbbbbbbbbbbbbbbbbpklbxlkaiaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaiaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaiaopbbbpklbaoczfhoxekaiaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqcd4ciGarfPDbedibledibkaiczfpxbbbbbbbbbbbbbbbbpklbxlkaiczfaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaiczfaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaiczfaopbbbpklbaoczfhoxekaiczfaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqcl4ciGarfPDbedibledibkaicafpxbbbbbbbbbbbbbbbbpklbxlkaicafaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaoclffagRb:q:W:cjbfhoxikaicafaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbahaocwffagRb:q:W:cjbfhoxdkaicafaopbbbpklbaoczfhoxekaicafaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbahaocdffagRb:q:W:cjbfhokdndndndndndnaqco4arfPDbedibledibkaic8Wfpxbbbbbbbbbbbbbbbbpklbxlkaic8Wfaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Ngicitpbi:q:G:cjbaiRb:q:W:cjbgipsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Ngqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbaiaoclffaqRb:q:W:cjbfhoxikaic8Wfaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Ngicitpbi:q:G:cjbaiRb:q:W:cjbgipsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Ngqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spklbaiaocwffaqRb:q:W:cjbfhoxdkaic8Wfaopbbbpklbaoczfhoxekaic8WfaopbbdaoRbbgicitpbi:q:G:cjbaiRb:q:W:cjbgipsaoRbegqcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpklbaiaocdffaqRb:q:W:cjbfhokalc;abfhialcjefaQ0meaihlaxao9Rc;Fb0mbkkdnaiaQ9pmbaici4hlinaxao9RcK6mwa8EaifhqdndndndndndnaAaico4fRbbalcoG4ciGarfPDbedibledibkaqpxbbbbbbbbbbbbbbbbpkbbxlkaqaopbblaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLg8Fcdp:mea8FpmbzeHdOiAlCvXoQrLpxiiiiiiiiiiiiiiiip9ogapxiiiiiiiiiiiiiiiip8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spkbbahaoclffagRb:q:W:cjbfhoxikaqaopbbwaopbbbg8Fclp:mea8FpmbzeHdOiAlCvXoQrLpxssssssssssssssssp9ogapxssssssssssssssssp8Jg8Fp5b9cjF;8;4;W;G;ab9:9cU1:Nghcitpbi:q:G:cjbahRb:q:W:cjbghpsa8Fp5e9cjF;8;4;W;G;ab9:9cU1:Nggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPaaa8Fp9spkbbahaocwffagRb:q:W:cjbfhoxdkaqaopbbbpkbbaoczfhoxekaqaopbbdaoRbbghcitpbi:q:G:cjbahRb:q:W:cjbghpsaoRbeggcitpbi:q:G:cjbp9UpmbedilvorzHOACXQLpPpkbbahaocdffagRb:q:W:cjbfhokalcdfhlaiczfgiaQ6mbkkaohAaoTmoka8EaQfh8Ea5cefg5cl9hmbkdndndndnawTmbaza8Acd4fRbbglciGPlbedwbkaQTmdavcjdfa8Afhlava8Afpbdbh8Jcbhoinalavcj;cbfaofpblbg8KaYaofpblbg8LpmbzeHdOiAlCvXoQrLg8MaLaofpblbg8NaKaofpblbgypmbzeHdOiAlCvXoQrLg8PpmbezHdiOAlvCXorQLg8Fcep9Ta8Fpxeeeeeeeeeeeeeeeegap9op9Hp9rg8Fa8Jp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ma8PpmwDKYqk8AExm35Ps8E8Fg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ka8LpmwKDYq8AkEx3m5P8Es8Fg8Ka8NaypmwKDYq8AkEx3m5P8Es8Fg8LpmbezHdiOAlvCXorQLg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ug8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp9Ug8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9Abbbaladfgla8Ja8Ka8LpmwDKYqk8AExm35Ps8E8Fg8Fcep9Ta8Faap9op9Hp9rg8Fp9Ugap9Abbbaladfglaaa8Fa8Fpmlvorlvorlvorlvorp9Ugap9Abbbaladfglaaa8Fa8FpmwDqkwDqkwDqkwDqkp9Ugap9Abbbaladfglaaa8Fa8FpmxmPsxmPsxmPsxmPsp9Ug8Jp9AbbbaladfhlaoczfgoaQ6mbxikkaQTmeavcjdfa8Afhlava8Afpbdbh8Jcbhoinalavcj;cbfaofpblbg8KaYaofpblbg8LpmbzeHdOiAlCvXoQrLg8MaLaofpblbg8NaKaofpblbgypmbzeHdOiAlCvXoQrLg8PpmbezHdiOAlvCXorQLg8Fcep:nea8Fpxebebebebebebebebgap9op:bep9rg8Fa8Jp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ma8PpmwDKYqk8AExm35Ps8E8Fg8Fcep:nea8Faap9op:bep9rg8Fp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ka8LpmwKDYq8AkEx3m5P8Es8Fg8Ka8NaypmwKDYq8AkEx3m5P8Es8Fg8LpmbezHdiOAlvCXorQLg8Fcep:nea8Faap9op:bep9rg8Fp:oeg8Jp9Abbbaladfgla8Ja8Fa8Fpmlvorlvorlvorlvorp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmwDqkwDqkwDqkwDqkp:oeg8Jp9Abbbaladfgla8Ja8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9Abbbaladfgla8Ja8Ka8LpmwDKYqk8AExm35Ps8E8Fg8Fcep:nea8Faap9op:bep9rg8Fp:oegap9Abbbaladfglaaa8Fa8Fpmlvorlvorlvorlvorp:oegap9Abbbaladfglaaa8Fa8FpmwDqkwDqkwDqkwDqkp:oegap9Abbbaladfglaaa8Fa8FpmxmPsxmPsxmPsxmPsp:oeg8Jp9AbbbaladfhlaoczfgoaQ6mbxdkkaQTmbcbhocbalcl4gl9Rc8FGhiavcjdfa8Afhrava8Afpbdbhainaravcj;cbfaofpblbg8JaYaofpblbg8KpmbzeHdOiAlCvXoQrLg8LaLaofpblbg8MaKaofpblbg8NpmbzeHdOiAlCvXoQrLgypmbezHdiOAlvCXorQLg8Faip:Rea8Falp:Tep9qg8Faap9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8LaypmwDKYqk8AExm35Ps8E8Fg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8Ja8KpmwKDYq8AkEx3m5P8Es8Fg8Ja8Ma8NpmwKDYq8AkEx3m5P8Es8Fg8KpmbezHdiOAlvCXorQLg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9Abbbaradfgraaa8Ja8KpmwDKYqk8AExm35Ps8E8Fg8Faip:Rea8Falp:Tep9qg8Fp9rgap9Abbbaradfgraaa8Fa8Fpmlvorlvorlvorlvorp9rgap9Abbbaradfgraaa8Fa8FpmwDqkwDqkwDqkwDqkp9rgap9Abbbaradfgraaa8Fa8FpmxmPsxmPsxmPsxmPsp9rgap9AbbbaradfhraoczfgoaQ6mbkka8Aclfg8Aad6mbkdnaCad2goTmbaOavcjdfao;8qbbkdnammbavavcjdfaCcufad2fad;8qbbkaCaHfhHc9:hoaAhPaAmbxlkkaeTmbaDalfhrcbhocuhlinaralaD9RglfaD6mdasaeao9Raoasfae6Eaofgoae6mbkaial9RhPkcbc99axaP9RakSEhoxekc9:hokavcj;kbf8Kjjjjbaokwbz:bjjjbkNsezu8Jjjjjbc;ae9Rgv8Kjjjjbc9:hodnalaeci9UgrcHf6mbcuhoaiRbbgwc;WeGc;Ge9hmbawcsGgDce0mbavc;abfcFecje;8kbav9cu83iUav9cu83i8Wav9cu83iyav9cu83iaav9cu83iKav9cu83izav9cu83iwav9cu83ibaialfc9WfhqaicefgwarfhldnaeTmbcmcsaDceSEhkcbhxcbhmcbhrcbhicbhoindnalaq9nmbc9:hoxikdndnawRbbgDc;Ve0mbavc;abfaoaDcu7gPcl4fcsGcitfgsydlhzasydbhHdndnaDcsGgsak9pmbavaiaPfcsGcdtfydbaxasEhDaxasTgOfhxxekdndnascsSmbcehOasc987asamffcefhDxekalcefhDal8SbbgscFeGhPdndnascu9mmbaDhlxekalcvfhlaPcFbGhPcrhsdninaD8SbbgOcFbGastaPVhPaOcu9kmeaDcefhDascrfgsc8J9hmbxdkkaDcefhlkcehOaPce4cbaPceG9R7amfhDkaDhmkavc;abfaocitfgsaDBdbasazBdlavaicdtfaDBdbavc;abfaocefcsGcitfgsaHBdbasaDBdlaocdfhoaOaifhidnadcd9hmbabarcetfgsaH87ebasclfaD87ebascdfaz87ebxdkabarcdtfgsaHBdbascwfaDBdbasclfazBdbxekdnaDcpe0mbavaiaqaDcsGfRbbgscl4gP9RcsGcdtfydbaxcefgOaPEhDavaias9RcsGcdtfydbaOaPTgzfgOascsGgPEhsaPThPdndnadcd9hmbabarcetfgHax87ebaHclfas87ebaHcdfaD87ebxekabarcdtfgHaxBdbaHcwfasBdbaHclfaDBdbkavaicdtfaxBdbavc;abfaocitfgHaDBdbaHaxBdlavaicefgicsGcdtfaDBdbavc;abfaocefcsGcitfgHasBdbaHaDBdlavaiazfgicsGcdtfasBdbavc;abfaocdfcsGcitfgDaxBdbaDasBdlaocifhoaiaPfhiaOaPfhxxekaxcbalRbbgsEgHaDc;:eSgDfhOascsGhAdndnascl4gCmbaOcefhzxekaOhzavaiaC9RcsGcdtfydbhOkdndnaAmbazcefhxxekazhxavaias9RcsGcdtfydbhzkdndnaDTmbalcefhDxekalcdfhDal8SbegPcFeGhsdnaPcu9kmbalcofhHascFbGhscrhldninaD8SbbgPcFbGaltasVhsaPcu9kmeaDcefhDalcrfglc8J9hmbkaHhDxekaDcefhDkasce4cbasceG9R7amfgmhHkdndnaCcsSmbaDhsxekaDcefhsaD8SbbglcFeGhPdnalcu9kmbaDcvfhOaPcFbGhPcrhldninas8SbbgDcFbGaltaPVhPaDcu9kmeascefhsalcrfglc8J9hmbkaOhsxekascefhskaPce4cbaPceG9R7amfgmhOkdndnaAcsSmbashlxekascefhlas8SbbgDcFeGhPdnaDcu9kmbascvfhzaPcFbGhPcrhDdninal8SbbgscFbGaDtaPVhPascu9kmealcefhlaDcrfgDc8J9hmbkazhlxekalcefhlkaPce4cbaPceG9R7amfgmhzkdndnadcd9hmbabarcetfgDaH87ebaDclfaz87ebaDcdfaO87ebxekabarcdtfgDaHBdbaDcwfazBdbaDclfaOBdbkavc;abfaocitfgDaOBdbaDaHBdlavaicdtfaHBdbavc;abfaocefcsGcitfgDazBdbaDaOBdlavaicefgicsGcdtfaOBdbavc;abfaocdfcsGcitfgDaHBdbaDazBdlavaiaCTaCcsSVfgicsGcdtfazBdbaiaATaAcsSVfhiaocifhokawcefhwaocsGhoaicsGhiarcifgrae6mbkkcbc99alaqSEhokavc;aef8Kjjjjbaok:clevu8Jjjjjbcz9Rhvdnalaecvf9pmbc9:skdnaiRbbc;:eGc;qeSmbcuskav9cb83iwaicefhoaialfc98fhrdnaeTmbdnadcdSmbcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcdtfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgiBdbalaiBdbawcefgwae9hmbxdkkcbhwindnaoar6mbc9:skaocefhlao8SbbgicFeGhddndnaicu9mmbalhoxekaocvfhoadcFbGhdcrhidninal8SbbgDcFbGaitadVhdaDcu9kmealcefhlaicrfgic8J9hmbxdkkalcefhokabawcetfadc8Etc8F91adcd47avcwfadceGcdtVglydbfgi87ebalaiBdbawcefgwae9hmbkkcbc99aoarSEk;Toio97eue97aec98Ghedndnadcl9hmbaeTmecbhdinababpbbbgicKp:RecKp:Sep;6eglaicwp:RecKp:Sep;6ealp;Geaiczp:RecKp:Sep;6egvp;Gep;Kep;Legopxbbbbbbbbbbbbbbbbp:2egralpxbbbjbbbjbbbjbbbjgwp9op9rp;Keglpxbb;:9cbb;:9cbb;:9cbb;:9calalp;Meaoaop;Meavaravawp9op9rp;Keglalp;Mep;Kep;Kep;Jep;Negvp;Mepxbbn0bbn0bbn0bbn0grp;KepxFbbbFbbbFbbbFbbbp9oaipxbbbFbbbFbbbFbbbFp9op9qalavp;Mearp;Kecwp:RepxbFbbbFbbbFbbbFbbp9op9qaoavp;Mearp;Keczp:RepxbbFbbbFbbbFbbbFbp9op9qpkbbabczfhbadclfgdae6mbxdkkaeTmbcbhdinabczfgDaDpbbbgipxbbbbbbFFbbbbbbFFgwp9oabpbbbgoaipmbediwDqkzHOAKY8AEgvczp:Reczp:Sep;6eglaoaipmlvorxmPsCXQL358E8FpxFubbFubbFubbFubbp9op;6eavczp:Sep;6egvp;Gealp;Gep;Kep;Legipxbbbbbbbbbbbbbbbbp:2egralpxbbbjbbbjbbbjbbbjgqp9op9rp;Keglpxb;:FSb;:FSb;:FSb;:FSalalp;Meaiaip;Meavaravaqp9op9rp;Keglalp;Mep;Kep;Kep;Jep;Negvp;Mepxbbn0bbn0bbn0bbn0grp;KepxFFbbFFbbFFbbFFbbp9oaiavp;Mearp;Keczp:Rep9qgialavp;Mearp;KepxFFbbFFbbFFbbFFbbp9oglpmwDKYqk8AExm35Ps8E8Fp9qpkbbabaoawp9oaialpmbezHdiOAlvCXorQLp9qpkbbabcafhbadclfgdae6mbkkk;2ileue97euo97dnaec98GgiTmbcbheinabcKfpx:ji:1S:ji:1S:ji:1S:ji:1SabpbbbglabczfgvpbbbgopmlvorxmPsCXQL358E8Fgrczp:Segwpxibbbibbbibbbibbbp9qp;6egDp;NegqaDaDp;MegDaDp;KealaopmbediwDqkzHOAKY8AEgDczp:Reczp:Sep;6eglalp;MeaDczp:Sep;6egoaop;Mearczp:Reczp:Sep;6egrarp;Mep;Kep;Kep;Lepxbbbbbbbbbbbbbbbbp:4ep;Jep;Mepxbbn0bbn0bbn0bbn0gDp;KepxFFbbFFbbFFbbFFbbgkp9oaqaop;MeaDp;Keczp:Rep9qgoaqalp;MeaDp;Keakp9oaqarp;MeaDp;Keczp:Rep9qgDpmwDKYqk8AExm35Ps8E8Fglp5eawclp:RegqpEi:T:j83ibavalp5baqpEd:T:j83ibabcwfaoaDpmbezHdiOAlvCXorQLgDp5eaqpEe:T:j83ibabaDp5baqpEb:T:j83ibabcafhbaeclfgeai6mbkkkuee97dnadcd4ae2c98GgeTmbcbhdinababpbbbgicwp:Recwp:Sep;6eaicep:SepxbbjFbbjFbbjFbbjFp9opxbbjZbbjZbbjZbbjZp:Uep;Mepkbbabczfhbadclfgdae6mbkkk:Sodw97euaec98Ghedndnadcl9hmbaeTmecbhdinabpxbbuJbbuJbbuJbbuJabpbbbgicKp:TeglaicYp:Tep9qgvcdp:Teavp9qgvclp:Teavp9qgop;6ep;Negvaicwp:RecKp:SegraipxFbbbFbbbFbbbFbbbgwp9ogDp:Uep;6ep;Mepxbbn0bbn0bbn0bbn0gqp;Kecwp:RepxbFbbbFbbbFbbbFbbp9oavaDarp:Xeaiczp:RecKp:Segip:Uep;6ep;Meaqp;Keawp9op9qavaDaraip:Uep:Xep;6ep;Meaqp;Keczp:RepxbbFbbbFbbbFbbbFbp9op9qavaoalcep:Rep9oalpxebbbebbbebbbebbbp9op9qp;6ep;Meaqp;KecKp:Rep9qpkbbabczfhbadclfgdae6mbxdkkaeTmbcbhdinabczfgkpxbFu9hbFu9hbFu9hbFu9habpbbbglakpbbbgrpmlvorxmPsCXQL358E8Fgvczp:TegqavcHp:Tep9qgicdp:Teaip9qgiclp:Teaip9qgicwp:Teaip9qgop;6ep;NegialarpmbediwDqkzHOAKY8AEgDpxFFbbFFbbFFbbFFbbglp9ograDczp:Segwp:Ueavczp:Reczp:SegDp:Xep;6ep;Mepxbbn0bbn0bbn0bbn0gvp;Kealp9oaiarawaDp:Uep:Xep;6ep;Meavp;Keczp:Rep9qgwaiaoaqcep:Rep9oaqpxebbbebbbebbbebbbp9op9qp;6ep;Meavp;Keczp:ReaiaDarp:Uep;6ep;Meavp;Kealp9op9qgipmwDKYqk8AExm35Ps8E8FpkbbabawaipmbezHdiOAlvCXorQLpkbbabcafhbadclfgdae6mbkkk9teiucbcbydj:G:cjbgeabcifc98GfgbBdj:G:cjbdndnabZbcztgd9nmbcuhiabad9RcFFifcz4nbcuSmekaehikaikkxebcj:Gdklz:zbb",t=new Uint8Array([0,97,115,109,1,0,0,0,1,4,1,96,0,0,3,3,2,0,0,5,3,1,0,1,12,1,0,10,22,2,12,0,65,0,65,0,65,0,252,10,0,0,11,7,0,65,0,253,15,26,11]),n=new Uint8Array([32,0,65,2,1,106,34,33,3,128,11,4,13,64,6,253,10,7,15,116,127,5,8,12,40,16,19,54,20,9,27,255,113,17,42,67,24,23,146,148,18,14,22,45,70,69,56,114,101,21,25,63,75,136,108,28,118,29,73,115]);if(typeof WebAssembly!="object")return{supported:!1};var i=WebAssembly.validate(t)?o(e):o(s),r,a=WebAssembly.instantiate(i,{}).then(function(f){r=f.instance,r.exports.__wasm_call_ctors()});function o(f){for(var b=new Uint8Array(f.length),E=0;E<f.length;++E){var g=f.charCodeAt(E);b[E]=g>96?g-97:g>64?g-39:g+4}for(var _=0,E=0;E<f.length;++E)b[_++]=b[E]<60?n[b[E]]:(b[E]-60)*64+b[++E];return b.buffer.slice(0,_)}function l(f,b,E,g,_,w,T){var y=f.exports.sbrk,C=g+3&-4,S=y(C*_),D=y(w.length),v=new Uint8Array(f.exports.memory.buffer);v.set(w,D);var I=b(S,g,_,D,w.length);if(I==0&&T&&T(S,C,_),E.set(v.subarray(S,S+g*_)),y(S-y(0)),I!=0)throw new Error("Malformed buffer data: "+I)}var c={NONE:"",OCTAHEDRAL:"meshopt_decodeFilterOct",QUATERNION:"meshopt_decodeFilterQuat",EXPONENTIAL:"meshopt_decodeFilterExp",COLOR:"meshopt_decodeFilterColor"},h={ATTRIBUTES:"meshopt_decodeVertexBuffer",TRIANGLES:"meshopt_decodeIndexBuffer",INDICES:"meshopt_decodeIndexSequence"},d=[],u=0;function p(f){var b={object:new Worker(f),pending:0,requests:{}};return b.object.onmessage=function(E){var g=E.data;b.pending-=g.count,b.requests[g.id][g.action](g.value),delete b.requests[g.id]},b}function x(f){for(var b="self.ready = WebAssembly.instantiate(new Uint8Array(["+new Uint8Array(i)+"]), {}).then(function(result) { result.instance.exports.__wasm_call_ctors(); return result.instance; });self.onmessage = "+m.name+";"+l.toString()+m.toString(),E=new Blob([b],{type:"text/javascript"}),g=URL.createObjectURL(E),_=d.length;_<f;++_)d[_]=p(g);for(var _=f;_<d.length;++_)d[_].object.postMessage({});d.length=f,URL.revokeObjectURL(g)}function M(f,b,E,g,_){for(var w=d[0],T=1;T<d.length;++T)d[T].pending<w.pending&&(w=d[T]);return new Promise(function(y,C){var S=new Uint8Array(E),D=++u;w.pending+=f,w.requests[D]={resolve:y,reject:C},w.object.postMessage({id:D,count:f,size:b,source:S,mode:g,filter:_},[S.buffer])})}function m(f){var b=f.data;self.ready.then(function(E){if(!b.id)return self.close();try{var g=new Uint8Array(b.count*b.size);l(E,E.exports[b.mode],g,b.count,b.size,b.source,E.exports[b.filter]),self.postMessage({id:b.id,count:b.count,action:"resolve",value:g},[g.buffer])}catch(_){self.postMessage({id:b.id,count:b.count,action:"reject",value:_})}})}return{ready:a,supported:!0,useWorkers:function(f){x(f)},decodeVertexBuffer:function(f,b,E,g,_){l(r,r.exports.meshopt_decodeVertexBuffer,f,b,E,g,r.exports[c[_]])},decodeIndexBuffer:function(f,b,E,g){l(r,r.exports.meshopt_decodeIndexBuffer,f,b,E,g)},decodeIndexSequence:function(f,b,E,g){l(r,r.exports.meshopt_decodeIndexSequence,f,b,E,g)},decodeGltfBuffer:function(f,b,E,g,_,w){l(r,r.exports[h[_]],f,b,E,g,r.exports[c[w]])},decodeGltfBufferAsync:function(f,b,E,g,_){return d.length>0?M(f,b,E,h[g],c[_]):a.then(function(){var w=new Uint8Array(f*b);return l(r,r.exports[h[g]],w,f,b,E,r.exports[c[_]]),w})}}})();var Kc=class extends Ys{constructor(){super(),this.name="RoomEnvironment",this.position.y=-3.5;let e=new _i;e.deleteAttribute("uv");let t=new vn({side:yn}),n=new vn,i=new wi(16777215,900,28,2);i.position.set(.418,16.199,.3),this.add(i);let r=new ht(e,t);r.position.set(-.757,13.219,.717),r.scale.set(31.713,28.305,28.591),this.add(r);let a=new An(e,n,6),o=new Yt;o.position.set(-10.906,2.009,1.846),o.rotation.set(0,-.195,0),o.scale.set(2.328,7.905,4.651),o.updateMatrix(),a.setMatrixAt(0,o.matrix),o.position.set(-5.607,-.754,-.758),o.rotation.set(0,.994,0),o.scale.set(1.97,1.534,3.955),o.updateMatrix(),a.setMatrixAt(1,o.matrix),o.position.set(6.167,.857,7.803),o.rotation.set(0,.561,0),o.scale.set(3.927,6.285,3.687),o.updateMatrix(),a.setMatrixAt(2,o.matrix),o.position.set(-2.017,.018,6.124),o.rotation.set(0,.333,0),o.scale.set(2.002,4.566,2.064),o.updateMatrix(),a.setMatrixAt(3,o.matrix),o.position.set(2.291,-.756,-2.621),o.rotation.set(0,-.286,0),o.scale.set(1.546,1.552,1.496),o.updateMatrix(),a.setMatrixAt(4,o.matrix),o.position.set(-2.193,-.369,-5.547),o.rotation.set(0,.516,0),o.scale.set(3.875,3.487,2.986),o.updateMatrix(),a.setMatrixAt(5,o.matrix),this.add(a);let l=new ht(e,ha(50));l.position.set(-16.116,14.37,8.208),l.scale.set(.1,2.428,2.739),this.add(l);let c=new ht(e,ha(50));c.position.set(-16.109,18.021,-8.207),c.scale.set(.1,2.425,2.751),this.add(c);let h=new ht(e,ha(17));h.position.set(14.904,12.198,-1.832),h.scale.set(.15,4.265,6.331),this.add(h);let d=new ht(e,ha(43));d.position.set(-.462,8.89,14.52),d.scale.set(4.38,5.441,.088),this.add(d);let u=new ht(e,ha(20));u.position.set(3.235,11.486,-12.541),u.scale.set(2.5,2,.1),this.add(u);let p=new ht(e,ha(100));p.position.set(0,20,0),p.scale.set(1,.1,1),this.add(p)}dispose(){let e=new Set;this.traverse(t=>{t.isMesh&&(e.add(t.geometry),e.add(t.material))});for(let t of e)t.dispose()}};function ha(s){return new Ka({color:0,emissive:16777215,emissiveIntensity:s})}var ls={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};var ni=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},rM=new ki(-1,1,1,-1,0,1),pd=class extends At{constructor(){super(),this.setAttribute("position",new Mt([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new Mt([0,2,0,0,2,0],2))}},aM=new pd,Vi=class{constructor(e){this._mesh=new ht(aM,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,rM)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var ua=class extends ni{constructor(e,t="tDiffuse"){super(),this.textureID=t,this.uniforms=null,this.material=null,e instanceof Rt?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=Ai.clone(e.uniforms),this.material=new Rt({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new Vi(this.material)}render(e,t,n){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=n.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var To=class extends ni{constructor(e,t){super(),this.scene=e,this.camera=t,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,t,n){let i=e.getContext(),r=e.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let a,o;this.inverse?(a=0,o=1):(a=1,o=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(i.REPLACE,i.REPLACE,i.REPLACE),r.buffers.stencil.setFunc(i.ALWAYS,a,4294967295),r.buffers.stencil.setClear(o),r.buffers.stencil.setLocked(!0),e.setRenderTarget(n),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(t),this.clear&&e.clear(),e.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(i.EQUAL,1,4294967295),r.buffers.stencil.setOp(i.KEEP,i.KEEP,i.KEEP),r.buffers.stencil.setLocked(!0)}},Yc=class extends ni{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var Jc=class{constructor(e,t){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),t===void 0){let n=e.getSize(new Ge);this._width=n.width,this._height=n.height,t=new ln(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:xn}),t.texture.name="EffectComposer.rt1"}else this._width=t.width,this._height=t.height;this.renderTarget1=t,this.renderTarget2=t.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new ua(ls),this.copyPass.material.blending=li,this.timer=new no}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,t){this.passes.splice(t,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let t=this.passes.indexOf(e);t!==-1&&this.passes.splice(t,1)}isLastEnabledPass(e){for(let t=e+1;t<this.passes.length;t++)if(this.passes[t].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let t=this.renderer.getRenderTarget(),n=!1;for(let i=0,r=this.passes.length;i<r;i++){let a=this.passes[i];if(a.enabled!==!1){if(a.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(i),a.render(this.renderer,this.writeBuffer,this.readBuffer,e,n),a.needsSwap){if(n){let o=this.renderer.getContext(),l=this.renderer.state.buffers.stencil;l.setFunc(o.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),l.setFunc(o.EQUAL,1,4294967295)}this.swapBuffers()}To!==void 0&&(a instanceof To?n=!0:a instanceof Yc&&(n=!1))}}this.renderer.setRenderTarget(t)}reset(e){if(e===void 0){let t=this.renderer.getSize(new Ge);this._pixelRatio=this.renderer.getPixelRatio(),this._width=t.width,this._height=t.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,t){this._width=e,this._height=t;let n=this._width*this._pixelRatio,i=this._height*this._pixelRatio;this.renderTarget1.setSize(n,i),this.renderTarget2.setSize(n,i);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(n,i)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var nm={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Le(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var da=class s extends ni{constructor(e,t=1,n,i){super(),this.strength=t,this.radius=n,this.threshold=i,this.resolution=e!==void 0?new Ge(e.x,e.y):new Ge(256,256),this.clearColor=new Le(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),a=Math.round(this.resolution.y/2);this.renderTargetBright=new ln(r,a,{type:xn,depthBuffer:!1}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let h=0;h<this.nMips;h++){let d=new ln(r,a,{type:xn,depthBuffer:!1});d.texture.name="UnrealBloomPass.h"+h,d.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(d);let u=new ln(r,a,{type:xn,depthBuffer:!1});u.texture.name="UnrealBloomPass.v"+h,u.texture.generateMipmaps=!1,this.renderTargetsVertical.push(u),r=Math.round(r/2),a=Math.round(a/2)}let o=nm;this.highPassUniforms=Ai.clone(o.uniforms),this.highPassUniforms.luminosityThreshold.value=i,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new Rt({uniforms:this.highPassUniforms,vertexShader:o.vertexShader,fragmentShader:o.fragmentShader}),this.separableBlurMaterials=[];let l=[6,10,14,18,22];r=Math.round(this.resolution.x/2),a=Math.round(this.resolution.y/2);for(let h=0;h<this.nMips;h++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(l[h])),this.separableBlurMaterials[h].uniforms.invSize.value=new Ge(1/r,1/a),r=Math.round(r/2),a=Math.round(a/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=t,this.compositeMaterial.uniforms.bloomRadius.value=.1;let c=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=c,this.bloomTintColors=[new U(1,1,1),new U(1,1,1),new U(1,1,1),new U(1,1,1),new U(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=Ai.clone(ls.uniforms),this.blendMaterial=new Rt({uniforms:this.copyUniforms,vertexShader:ls.vertexShader,fragmentShader:ls.fragmentShader,premultipliedAlpha:!0,blending:rn,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new Le,this._oldClearAlpha=1,this._basic=new jt,this._fsQuad=new Vi(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,t){let n=Math.round(e/2),i=Math.round(t/2);this.renderTargetBright.setSize(n,i);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(n,i),this.renderTargetsVertical[r].setSize(n,i),this.separableBlurMaterials[r].uniforms.invSize.value=new Ge(1/n,1/i),n=Math.round(n/2),i=Math.round(i/2)}render(e,t,n,i,r){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let a=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),r&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=n.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=n.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let o=this.renderTargetBright;for(let l=0;l<this.nMips;l++)this._fsQuad.material=this.separableBlurMaterials[l],this.separableBlurMaterials[l].uniforms.colorTexture.value=o.texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[l]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[l].uniforms.colorTexture.value=this.renderTargetsHorizontal[l].texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[l]),e.clear(),this._fsQuad.render(e),o=this.renderTargetsVertical[l];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(n),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=a}_getSeparableBlurMaterial(e){let t=[],n=e/3;for(let a=0;a<e;a++)t.push(.39894*Math.exp(-.5*a*a/(n*n))/n);let i=[],r=[];for(let a=1;a<e;a+=2){let o=t[a],l=a+1<e?t[a+1]:0,c=o+l;i.push((a*o+(a+1)*l)/c),r.push(c)}return new Rt({defines:{KERNEL_PAIRS:i.length},uniforms:{colorTexture:{value:null},invSize:{value:new Ge(.5,.5)},direction:{value:new Ge(.5,.5)},centerWeight:{value:t[0]},gaussianOffsets:{value:i},gaussianWeights:{value:r}},vertexShader:`

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

				}`})}_getCompositeMaterial(e){return new Rt({defines:{NUM_MIPS:e},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`

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

				}`})}};da.BlurDirectionX=new Ge(1,0);da.BlurDirectionY=new Ge(0,1);var Ao={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var Zc=class extends ni{constructor(){super(),this.isOutputPass=!0,this.uniforms=Ai.clone(Ao.uniforms),this.material=new jr({name:Ao.name,uniforms:this.uniforms,vertexShader:Ao.vertexShader,fragmentShader:Ao.fragmentShader}),this._fsQuad=new Vi(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,t,n){this.uniforms.tDiffuse.value=n.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},Dt.getTransfer(this._outputColorSpace)===Xt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===ao?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===oo?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===lo?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===ir?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===ho?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===uo?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===co&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Ci=512,oM=7.5;function md(s,e=oM){let t=new qa,n=s.map(([o,l])=>new U(o,0,l)),i=[],r=[],a=n.length;for(let o=0;o<a;o++){let l=n[o],c=n[(o+a-1)%a],h=n[(o+1)%a],d=Math.min(e,l.distanceTo(c)/2,l.distanceTo(h)/2);i.push(l.clone().addScaledVector(c.clone().sub(l).normalize(),d)),r.push(l.clone().addScaledVector(h.clone().sub(l).normalize(),d))}for(let o=0;o<a;o++)t.add(new Mi(i[o],n[o],r[o])),t.add(new qr(r[o],i[(o+1)%a]));return t}function lM(s){let e=md(s),t=e.getSpacedPoints(Ci),n=[];for(let i=0;i<Ci;i++){let r=t[(i+Ci-1)%Ci],a=t[(i+1)%Ci];n.push(new U(a.x-r.x,0,a.z-r.z).normalize())}return{points:t,tangents:n,length:e.getLength()}}function im(s){let e=[],t=(n,i,r,a)=>{let o=Math.cos(n.angle),l=Math.sin(n.angle);e.push({x:n.x+i*o+r*l,z:n.z-i*l+r*o,r:a,asset:n.asset})};for(let n of s)if(!(n.z<-100))if(n.asset==="data-tram")for(let i of[-2.4,0,2.4])t(n,i,0,1.5);else n.asset==="server-rack"?t(n,0,0,.8):n.asset==="street-lamp"?t(n,0,0,.5):n.asset==="planter"&&(t(n,-.9,0,.85),t(n,.9,0,.85));return e}function sm({routes:s,obstacles:e=[],robotRadius:t=1.3,lanes:n=[2.2,0,-2.2,3.9,-3.9],defaultLane:i=2.2,speeds:r=[],blocked:a=()=>!1}){let o=s.map(lM),l=s.map((S,D)=>({route:D,s:o[D].length*(D*.173%1),dir:1,lane:i,laneTarget:i,speed:0,target:0,laneVel:0,cruise:r[D]??5.1+D*.43,state:"cruise",heading:0,wait:0,cooldown:0,x:0,z:0,fx:0,fz:1,rx:1,rz:0,vx:0,vz:0,turns:0})),c=new U,h=new U,d=new U;function u(S,D,v,I,P){let N=(D/S.length%1+1)%1*Ci,z=Math.floor(N)%Ci,X=N-Math.floor(N);I.lerpVectors(S.points[z],S.points[(z+1)%Ci],X),P.lerpVectors(S.tangents[z],S.tangents[(z+1)%Ci],X).normalize().multiplyScalar(v)}let p=S=>Math.atan2(Math.sin(S),Math.cos(S));function x(S){u(o[S.route],S.s,S.dir,c,h),S.fx=h.x,S.fz=h.z,S.rx=-h.z,S.rz=h.x,S.x=c.x+S.rx*S.lane,S.z=c.z+S.rz*S.lane}function M(S,D){for(let v of e){let I=v.r+t;if((S-v.x)**2+(D-v.z)**2<I*I)return!0}return!1}function m(S,D){let v=o[S.route];for(let I of[-2.5,0,1,3,6,9,12,15,18])if(u(v,S.s+S.dir*I,S.dir,c,h),d.set(c.x-h.z*D,0,c.z+h.x*D),M(d.x,d.z)||I>=0&&a(S,d.x,d.z))return!1;return!0}function f(S){S.dir=-S.dir,S.lane=-S.lane,S.laneTarget=i,S.laneVel=0,S.state="turn",S.cooldown=4,S.wait=0,S.target=0,S.turns++,S.blocked=0}function b(S,D){if(S.held){S.target=0,S.speed=0,S.laneVel=0;return}if(S.cooldown>0&&(S.cooldown-=D),S.state==="turn"){if(S.target=0,S.speed<.05){let N=Math.atan2(S.fx,S.fz),z=p(N-S.heading);S.heading+=z*Math.min(1,D*3.2),Math.abs(z)<.1&&(S.heading=N,S.state="cruise")}return}let v=null,I=[...n].sort((N,z)=>Math.abs(N-i)-Math.abs(z-i)||z-N);for(let N of I)if(m(S,N)){v=N;break}if(v===null){S.blocked=(S.blocked||0)+D,S.target=0,S.blocked>2.5&&S.cooldown<=0&&(S.blocked=0,f(S));return}S.blocked=0,S.laneTarget=v,S.target=S.cruise,Math.abs(S.laneTarget-S.lane)>.5&&!m(S,S.lane)&&(S.target=Math.min(S.target,2.4));let P=!1;for(let N of l){if(N===S)continue;let z=T(S,S.laneTarget,N);if(!z)continue;P=!0;let X=N.fx*S.fx+N.fz*S.fz,j=N.speed<.3;if(j||X<-.2){let V=g.get(N)[E.length-1],W=j?0:Math.sign((V.x-S.x)*S.rx+(V.z-S.z)*S.rz)||-1,ie=null;for(let H of I)if(!(W&&Math.sign(H-S.lane)===W)&&m(S,H)&&!T(S,H,N)){ie=H;break}ie!==null?S.laneTarget=ie:S.target=0}else X>.5?z.da>=z.db&&(S.target=Math.min(S.target,z.da<3.8?0:N.speed*.9)):(z.da>z.db||z.da===z.db&&S.route>N.route)&&(S.target=Math.min(S.target,z.da<=4?0:2))}P&&S.target===0?S.wait+=D:S.wait=0,S.wait>2.5&&S.cooldown<=0&&f(S)}let E=[0,2,4,6,8,10,12],g=new Map,_=new U;function w(S,D,v,I){u(o[S.route],S.s+S.dir*D,S.dir,c,h),I.set(c.x-h.z*v,0,c.z+h.x*v)}function T(S,D,v){let I=g.get(v),P=(t*2+.6)**2,N=v.speed<.3;for(let z=0;z<E.length;z++){w(S,E[z],D,_);let X=E[z]/Math.max(S.speed,1.5);for(let j=0;j<E.length&&!(N&&j>0);j++){let V=E[j]/Math.max(v.speed,1.5);if(!(E[z]>4&&Math.abs(X-V)>1.6)&&_.distanceToSquared(I[j])<P)return{da:E[z],db:E[j]}}}return null}function y(S){if(S>0){for(let D of l){x(D),g.has(D)||g.set(D,E.map(()=>new U));let v=g.get(D);E.forEach((I,P)=>w(D,D.state==="cruise"?I:0,D.lane,v[P]))}for(let D of l)b(D,S);for(let D=0;D<l.length;D++)for(let v=D+1;v<l.length;v++){let I=l[D],P=l[v],N=P.x-I.x,z=P.z-I.z,X=Math.hypot(N,z),j=t*2+.2;if(X>=j)continue;let V=I.state==="turn"||I.speed<.3,W=P.state==="turn"||P.speed<.3,ie=V===W?.5:V?0:1,H=Math.min(j-Math.max(X,.01),2.5*S),ge=N*I.rx+z*I.rz,Ze=-(N*P.rx+z*P.rz);I.lane=It.clamp(I.lane-Math.sign(ge||1)*H*ie,-4.2,4.2),P.lane=It.clamp(P.lane-Math.sign(Ze||1)*H*(1-ie),-4.2,4.2),I.target=Math.min(I.target,.5),P.target=Math.min(P.target,.5)}for(let D of l){let v=D.x,I=D.z,P=D.target>D.speed?6:9;if(D.held){D.vx=D.vz=0;continue}D.speed+=It.clamp(D.target-D.speed,-P*S,P*S),D.speed<0&&(D.speed=0),D.s+=D.dir*D.speed*S;let N=D.laneTarget-D.lane,z=D.state==="turn"?0:Math.min(3,D.speed*.6),X=Math.sign(N)*Math.min(z,Math.sqrt(8*Math.abs(N)),Math.abs(N)/S);if(D.laneVel+=It.clamp(X-D.laneVel,-4*S,4*S),D.lane+=D.laneVel*S,x(D),D.vx=(D.x-v)/S,D.vz=(D.z-I)/S,D.state==="cruise"){let j=D.speed>.25?Math.atan2(D.vx,D.vz):Math.atan2(D.fx,D.fz);D.heading+=p(j-D.heading)*Math.min(1,S*(D.speed>.25?8:3.2))}}}}let C=[];for(let S of l){for(let D=0;D<600&&(x(S),!(C.every(I=>Math.hypot(S.x-I.x,S.z-I.z)>t*2+6)&&m(S,S.lane)));D++)S.s+=1;S.heading=Math.atan2(S.fx,S.fz),C.push(S)}return{agents:l,step:y,hitsObstacle:M,seek(S,D){S.s=D,x(S),S.heading=Math.atan2(S.fx,S.fz)},adopt(S,D){let v=o[S.route],I=0,P=1/0;for(let N=0;N<Ci;N++){let z=v.points[N],X=Math.hypot(z.x-D.x,z.z-D.z);X<P&&(P=X,I=N)}S.s=I/Ci*v.length,u(v,S.s,S.dir,c,h),S.lane=(D.x-c.x)*-h.z+(D.z-c.z)*h.x,S.laneTarget=i,S.speed=0,S.laneVel=0,S.state="cruise",S.heading=D.heading,x(S)},stats:()=>({states:l.map(S=>S.state),turns:l.reduce((S,D)=>S+D.turns,0),lanes:l.map(S=>Math.round(S.lane*10)/10)})}}var cM=(s,e,t)=>Math.max(e,Math.min(t,s)),eh=(s,e)=>Math.atan2(Math.sin(e-s),Math.cos(e-s)),Ro=s=>({x:s.x,y:s.y||0,z:s.z,heading:s.heading||0});function cs(s,e=1){let t=s.min.map(u=>u*e),n=s.max.map(u=>u*e),i=(n[0]-t[0])/2,r=(n[2]-t[2])/2,a=i>r,o=Math.max(i,r),l=Math.min(i,r),c=Math.min(7,Math.max(1,Math.ceil(o/Math.max(.3,l)))),h=o/c,d=[];for(let u=0;u<c;u++){let p=-o+h+u*h*2;d.push({x:(t[0]+n[0])/2+(a?p:0),z:(t[2]+n[2])/2+(a?0:p),r:Math.hypot(l,h)})}return{circles:d,minY:t[1],maxY:n[1],reach:Math.max(...d.map(u=>Math.hypot(u.x,u.z)+u.r))}}function fa(s,e){let t=Math.cos(e.heading),n=Math.sin(e.heading);return{x:e.x+s.x*t+s.z*n,z:e.z-s.x*n+s.z*t,r:s.r}}function Qc(s,e,t,n,i){let r=e-s;if(Math.abs(r)<1e-10)return s>=t&&s<=n;let a=(t-s)/r,o=(n-s)/r;return i[0]=Math.max(i[0],Math.min(a,o)),i[1]=Math.min(i[1],Math.max(a,o)),i[0]<=i[1]}function hM(s,e,t,n,i,r){let a=[0,1];return Qc(e.y-i.y,t.y-r.y,n.minY-s.maxY+.06,n.maxY-s.minY-.06,a)?a:null}function $c(s,e,t,n,i,r){let a=s.reach+n.reach+.06;if(Math.min(e.x,t.x)-Math.max(i.x,r.x)>a||Math.min(i.x,r.x)-Math.max(e.x,t.x)>a||Math.min(e.z,t.z)-Math.max(i.z,r.z)>a||Math.min(i.z,r.z)-Math.max(e.z,t.z)>a)return!1;let o=hM(s,e,t,n,i,r);if(!o)return!1;for(let l of s.circles)for(let c of n.circles){let h=.06+Math.abs(eh(e.heading,t.heading))*Math.hypot(l.x,l.z)+Math.abs(eh(i.heading,r.heading))*Math.hypot(c.x,c.z),d=fa(l,e),u=fa(l,t),p=fa(c,i),x=fa(c,r),M=d.x-p.x,m=d.z-p.z,f=u.x-x.x-M,b=u.z-x.z-m,E=cM(-(M*f+m*b)/Math.max(1e-12,f*f+b*b),...o),g=l.r+c.r+h;if((M+f*E)**2+(m+b*E)**2<g*g)return!0}return!1}function gd(s,e,t,n){let i=s.reach+.06;if(n.world&&(Math.min(e.x,t.x)-i>n.world[2]||Math.max(e.x,t.x)+i<n.world[0]||Math.min(e.z,t.z)-i>n.world[3]||Math.max(e.z,t.z)+i<n.world[1])||Math.min(e.y,t.y)+s.minY>=n.max[1]||Math.max(e.y,t.y)+s.maxY<=n.min[1])return!1;let r=Math.cos(n.heading||0),a=Math.sin(n.heading||0),o=l=>({x:(l.x-n.x)*r-(l.z-n.z)*a,z:(l.x-n.x)*a+(l.z-n.z)*r});for(let l of s.circles){let c=o(fa(l,e)),h=o(fa(l,t)),d=l.r+.06+Math.abs(eh(e.heading,t.heading))*Math.hypot(l.x,l.z),u=[0,1];if(Qc(e.y,t.y,n.min[1]-s.maxY+.06,n.max[1]-s.minY-.06,u)&&Qc(c.x,h.x,n.min[0]-d,n.max[0]+d,u)&&Qc(c.z,h.z,n.min[2]-d,n.max[2]+d,u))return!0}return!1}function th(){let s=new Map,e=new Map,t=new Map,n=0,i=0,r=0,a=0,o=(x,M)=>x.id===M.id||x.ignore===M.id||M.ignore===x.id||x.enabled===!1||M.enabled===!1;function l(x,M,m,f=!0){if(![m.x,m.y,m.z,m.heading].every(Number.isFinite))return"invalid-pose";for(let b of e.values())if(b.enabled!==!1&&!(b.owner&&(b.owner===x.ignore||b.owner===x.id))&&gd(x,M,m,b))return b.id;if(f){for(let b of s.values())if(!o(x,b)&&$c(x,M,m,b,b,b))return b.id}return null}let c=(x,M,m,f=!0)=>!l(x,M,m,f);function h(x,M,m,f=0){if(s.has(x))return s.get(x);let b={id:x,...M,...Ro(m),priority:f,enabled:!0,blocked:0};return c(b,b,b)?(s.set(x,b),b):null}function d(x,M,m=12){let f={...x,enabled:!0};for(let b=0;b<=m;b+=.5)for(let E=0;E<(b?24:1);E++){let g={...Ro(M),x:M.x+Math.cos(E*Math.PI/12)*b,z:M.z+Math.sin(E*Math.PI/12)*b};if(c(f,g,g))return g}return null}function u(x,M,m=12){let f=d(x,M,m);return f?(Object.assign(x,f),!0):!1}function p(x){let M=[...s.values()].filter(m=>m.enabled!==!1).map(m=>{let f=t.get(m.id),b=f?.next||Ro(m);return{body:m,from:Ro(m),next:b,request:f,accepted:c(m,m,b,!1)}});for(let m=0;m<=M.length;m++){let f=!1;for(let b of M)b.accepted&&b.body.follow&&M.some(E=>E.body.id===b.body.follow&&!E.accepted)&&(b.accepted=!1,f=!0);for(let b=0;b<M.length;b++)for(let E=b+1;E<M.length;E++){let g=M[b],_=M[E];if(o(g.body,_.body)||!g.accepted&&!_.accepted)continue;let w=g.accepted?g.next:g.from,T=_.accepted?_.next:_.from;if(!$c(g.body,g.from,w,_.body,_.from,T))continue;let y=V=>V.request&&V.accepted&&(Math.hypot(V.next.x-V.from.x,V.next.y-V.from.y,V.next.z-V.from.z)>1e-8||Math.abs(eh(V.from.heading,V.next.heading))>1e-8),C=y(g),S=y(_);if(!C&&!S)continue;let D=g.next.x-g.from.x,v=g.next.z-g.from.z,I=_.next.x-_.from.x,P=_.next.z-_.from.z,N=D*I+v*P>.8*Math.hypot(D,v)*Math.hypot(I,P),z=C&&!$c(g.body,g.from,g.next,_.body,_.from,_.from),X=S&&!$c(g.body,g.from,g.from,_.body,_.from,_.next),j=C?S?z&&!X?_:X&&!z?g:g.body.priority!==_.body.priority?g.body.priority<_.body.priority?g:_:N?(_.from.x-g.from.x)*D+(_.from.z-g.from.z)*v>0?g:_:g.body.id>_.body.id?g:_:g:_;j.accepted=!1,f=!0}if(!f)break}for(let m of M)m.accepted?(Object.assign(m.body,m.next),m.body.blocked=0,m.body.obstruction=null):m.request&&(m.body.blocked+=x,m.body.obstruction=l(m.body,m.from,m.next),n++);for(let m of M)m.request?.commit?.(m.accepted,m.body);t.clear(),i++,r+=x}return{register:h,relocate:u,findFree:d,clear:c,obstruction:l,solve:p,begin(){t.clear()},propose(x,M,m){x?.enabled!==!1&&x&&t.set(x.id,{next:Ro(M),commit:m})},remove(x){s.delete(x),t.delete(x)},solid(x,M){let m=e.get(x),f={id:x,x:0,z:0,heading:0,...M};if(m&&m.x===f.x&&m.z===f.z&&m.heading===f.heading&&m.enabled===f.enabled&&m.min.every((w,T)=>w===f.min[T])&&m.max.every((w,T)=>w===f.max[T]))return;let b=Math.cos(f.heading),E=Math.sin(f.heading),g=[],_=[];for(let w of[f.min[0],f.max[0]])for(let T of[f.min[2],f.max[2]])g.push(f.x+w*b+T*E),_.push(f.z-w*E+T*b);f.world=[Math.min(...g),Math.min(..._),Math.max(...g),Math.max(..._)],e.set(x,f),a++},removeSolid(x){e.delete(x)&&a++},removeOwner(x){for(let[M,m]of e)m.owner===x&&(e.delete(M),a++)},revision:()=>a,solidClear(x){return[...s.values()].every(M=>M.enabled===!1||x.owner&&M.ignore===x.owner||!gd(M,M,M,{x:0,z:0,heading:0,...x}))},ceiling(x,M,m){let f=0;for(let b of e.values())b.enabled!==!1&&gd(x,{x:M,y:b.min[1],z:m,heading:0},{x:M,y:b.min[1],z:m,heading:0},b)&&(f=Math.max(f,b.max[1]-x.minY+1));return f},body:x=>s.get(x),neighbors(x,M=12){return[...s.values()].filter(m=>!o(x,m)&&Math.abs(m.y-x.y)<4&&Math.hypot(m.x-x.x,m.z-x.z)<M)},stats:()=>({bodies:s.size,solids:e.size,stops:n,steps:i,time:r,poses:[...s.values()].filter(x=>x.enabled!==!1).map(x=>({id:x.id,x:x.x,y:x.y,z:x.z,heading:x.heading,blocked:x.blocked,obstruction:x.obstruction}))}),dispose(){s.clear(),e.clear(),t.clear()}}}function rm(s,e=1/60){let t=0;return n=>{if(!(n>0))return t=0,0;t=Math.min(t+n,e*6);let i=0;for(;t>=e-1e-9;)s(e),t-=e,i++;return i}}var Wt={xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80,quayZ:74},bt={ground:0,road:-.14,pavement:0,quay:.08,room:.16,gallery:4.16},On=["agent","memory","missions"].map((s,e)=>{let t=[0,-43,43][e],n=73;return{id:s,x:t,z:n,width:12,depth:12,front:-1,doorZ:n-6,liftX:t+4,liftZ:n+3}}),ui=[{id:"infra",x:-67,z:-53,platformX:-74,platformZ:-53,angle:Math.PI/2},{id:"memory",x:-67,z:-10,platformX:-74,platformZ:-10,angle:Math.PI/2},{id:"graph",x:-43,z:59,platformX:-43,platformZ:64.5,angle:0},{id:"operations",x:0,z:59,platformX:0,platformZ:64.5,angle:0},{id:"missions",x:43,z:59,platformX:43,platformZ:64.5,angle:0},{id:"integrations",x:67,z:-10,platformX:74,platformZ:-10,angle:-Math.PI/2},{id:"agent",x:0,z:-77,platformX:0,platformZ:-84,angle:Math.PI}],xd=[[-67,-77],[-67,59],[67,59],[67,-77]],am=(s,e)=>{let t=s*Math.PI/180;return{x:Math.sin(t)*e,z:-12-Math.cos(t)*e,angle:Math.PI-t}},Jt={x:0,z:-12,floor:72.2,inner:4.2,outer:8,bridge:1,lift:{x:9.8,z:-12,base:.88,cab:.03,travel:71.32},telescopes:[45,135,225,315].map(s=>({id:"deck-"+s,...am(s,7.25)})),benches:[0,180,270].map(s=>am(s,6.6))};function om(s,e,t){let n=Math.hypot(s-Jt.x,e-Jt.z);return n>=Jt.inner&&n<=Jt.outer?!0:t&&s>=Jt.outer-.5&&s<=Jt.lift.x+1&&Math.abs(e-Jt.lift.z)<Jt.bridge}var Wi={x:78,z:30},lm=[{id:"south",x:-7.8,z:63.75,angle:Math.PI},{id:"harbour",x:23.3,z:75,angle:-Math.PI/2},{id:"west",x:-72.3,z:-22,angle:Math.PI/2},{id:"east",x:72.3,z:42,angle:-Math.PI/2},{id:"north",x:-8,z:-26.7,angle:Math.PI}],nh=[{x:30.5,z:-55,scale:1},{x:49,z:-55,scale:1.25},{x:-3,z:-57,scale:1.2}];function uM(s,e,t,n=0){return e>=s.x-5.85+n&&e<=s.x+2.4-n&&t>=s.z+n&&t<=s.z+5.85-n}function cm(s,e,t,n=0){return Math.abs(e-s.liftX)<=1.6-n&&Math.abs(t-s.liftZ)<=1.6-n}function hm(){let s=[],e=[2.4,1.4,5.6,4.6],t=(n,i,r,a)=>{r>n&&a>i&&s.push({x:(n+r)/2,z:(i+a)/2,sx:(r-n)/4,sz:(a-i)/4})};for(let n=-4;n<=4;n+=4)for(let i=-4;i<=4;i+=4){let[r,a,o,l]=[n-2,i-2,n+2,i+2],[c,h,d,u]=[Math.max(r,e[0]),Math.max(a,e[1]),Math.min(o,e[2]),Math.min(l,e[3])];if(c>=d||h>=u){t(r,a,o,l);continue}t(r,a,c,l),t(d,a,o,l),t(c,a,d,h),t(c,u,d,l)}return s}function bd(s,e,t,n){return uM(s,e,t,.25)?!0:n?e>=s.x+1.8&&e<=s.liftX+1.25&&Math.abs(t-s.liftZ)<1.25:!1}var qt={line:3.3,run:.14,top:.14,rise:.14,droppedRun:1.2,opening:1.2,flare:.8,zebraOffset:8,cornerRadius:1.2,endDepth:.5,pavementLine:4.75},qi=4,dM=1,rh=48,cr={top:-.2,bottom:-3},dm=.15,fM=.1,um=s=>s<0?0:s>1?1:s,vd=[...Wt.zs.map(s=>({axis:"x",line:s,from:Wt.minX,to:Wt.maxX,crossings:Wt.xs})),...Wt.xs.map(s=>({axis:"z",line:s,from:Wt.minZ,to:Wt.quayZ,crossings:Wt.zs}))];var ah=[];for(let s of vd)for(let e of s.crossings)for(let t of[-1,1]){let n=e+t*qt.zebraOffset;n>=s.from&&n<=s.to&&ah.push({axis:s.axis,line:s.line,at:n,x:s.axis==="x"?n:s.line,z:s.axis==="x"?s.line:n})}function fm(){return ah.map(s=>({...s}))}function pM(s){return s<=qt.opening?qt.droppedRun:s>=qt.opening+qt.flare?qt.run:qt.droppedRun+(qt.run-qt.droppedRun)*(s-qt.opening)/qt.flare}function oh(s,e){let{xs:t,zs:n,halfWidth:i,minX:r,maxX:a,minZ:o,quayZ:l}=Wt,c=1/0,h=1/0;if(e>=o&&e<=l)for(let g of t)c=Math.min(c,Math.abs(s-g));if(s>=r&&s<=a)for(let g of n)h=Math.min(h,Math.abs(e-g));let d=c<=i,u=h<=i;if(!d&&!u)return null;let p=bt.road,x,M=qt.run;if(d&&u)if(c<=qt.line||h<=qt.line)x=-1;else{let g=c-qt.line,_=h-qt.line,w=qt.cornerRadius;x=g<w&&_<w?w-Math.hypot(w-g,w-_):Math.min(g,_)}else{let g=u?s:e,_=u?t:n,w=1/0;for(let T of _)w=Math.min(w,Math.abs(g-T-qt.zebraOffset),Math.abs(g-T+qt.zebraOffset));x=(u?h:c)-qt.line,M=pM(w)}let m=p+qt.rise*um(x/M),f=x<=0?"road":x>=M?"pavement":"kerb",b=Math.min(d?e-o:1/0,d?l-e:1/0,u?s-r:1/0,u?a-s:1/0),E=qt.endDepth+qt.run;if(b<E){let g=p+qt.rise*um((E-b)/qt.run);g>m&&(m=g,f="kerb")}return{height:m,kind:f,d:x,run:M}}function mM(){let s=[];for(let e of vd){let t=[{at:e.from,kind:"end"},...e.crossings.map(n=>({at:n,kind:"crossing"})),{at:e.to,kind:"end"}];for(let n=0;n+1<t.length;n++){let i=t[n],r=t[n+1];s.push({axis:e.axis,line:e.line,start:i.kind,end:r.kind,from:i.kind==="crossing"?i.at+Wt.halfWidth:i.at,to:r.kind==="crossing"?r.at-Wt.halfWidth:r.at})}}return s}var gM={x:0,z:Math.PI/2};function Co(s,e,t,n,i){let r=(n+i)/2;return{id:s,axis:e,line:t,from:n,to:i,x:e==="x"?r:t,z:e==="x"?t:r,angle:gM[e]}}function pm(){let s=[];for(let e of Wt.xs)for(let t of Wt.zs)s.push({id:"street-crossing",axis:null,line:null,from:null,to:null,x:e,z:t,angle:0});for(let e of mM()){let t=e.from,n=e.to;e.start==="crossing"&&(s.push(Co("street-zebra",e.axis,e.line,t,t+qi)),t+=qi),e.end==="crossing"&&(s.push(Co("street-zebra",e.axis,e.line,n-qi,n)),n-=qi);let i=e.start==="crossing",r=Math.floor((n-t)/qi+1e-9),a=Math.round(n-t-r*qi);for(let o=0;o<r;o++){let l=o%3===2?"street-drain":"street-plain";s.push(i?Co(l,e.axis,e.line,t+o*qi,t+(o+1)*qi):Co(l,e.axis,e.line,n-(o+1)*qi,n-o*qi))}for(let o=0;o<a;o++){let l=i?n-a+o:t+o;s.push(Co("street-fill",e.axis,e.line,l,l+dM))}for(let[o,l,c]of[[e.start,e.from,-1],[e.end,e.to,1]])o==="end"&&s.push({id:"street-end",axis:e.axis,line:e.line,from:l,to:l,x:e.axis==="x"?l:e.line,z:e.axis==="x"?e.line:l,angle:e.axis==="x"?c>0?0:Math.PI:c>0?-Math.PI/2:Math.PI/2})}return s}function mm(){let{xs:s,zs:e,halfWidth:t,minX:n,maxX:i,minZ:r,maxZ:a,quayZ:o}=Wt,l=(h,d,u)=>{let p=[],x=d;for(let M of h)p.push([x,M-t]),x=M+t;return p.push([x,u]),p},c=[];for(let[h,d]of l(s,n,i))for(let[u,p]of l(e,r,o))c.push({kind:"plot",x0:h,z0:u,x1:d,z1:p,top:bt.ground,bottom:cr.top});return c.push({kind:"quay",x0:n,z0:o,x1:i,z1:a,top:bt.ground,bottom:cr.top}),c}var ih=qt.line+qt.droppedRun+dm;function xM(s,e){let t=Math.min(s.x,e.x),n=Math.max(s.x,e.x),i=Math.min(s.z,e.z),r=Math.max(s.z,e.z);return vd.some(a=>a.axis==="x"?i<=a.line+ih&&r>=a.line-ih&&n>=a.from&&t<=a.to:t<=a.line+ih&&n>=a.line-ih&&r>=a.from&&i<=a.to)}function gm(s,e){return ah.some(t=>t.axis==="x"?Math.abs(s-t.at)<=qt.opening&&Math.abs(e-t.line)<=Wt.halfWidth:Math.abs(e-t.at)<=qt.opening&&Math.abs(s-t.line)<=Wt.halfWidth)}function hs(s,e,t=!1,n=!1){if(!xM(s,e))return!0;let i=n,r=Math.max(1,Math.ceil(Math.hypot(e.x-s.x,e.z-s.z)/fM));for(let a=0;a<=r;a++){let o=a/r,l=s.x+(e.x-s.x)*o,c=s.z+(e.z-s.z)*o,h=oh(l,c);if(!h||h.kind==="pavement"&&h.d-h.run>=dm){i=!1;continue}if(!(t&&gm(l,c))&&!i)return!1}return!0}function Ds(s,e){let t=oh(s,e);return!!t&&t.kind==="road"&&!gm(s,e)}var sh=null;function xm(){if(sh)return sh;let s=qt.pavementLine,e=[],t=[];for(let n of Wt.xs)for(let i of Wt.zs)for(let[r,a]of[[1,1],[1,-1],[-1,1],[-1,-1]])e.push({x:n+r*s,z:i+a*s,kind:"corner"});for(let n of ah){let i=e.length;for(let r of[-1,1])e.push(n.axis==="x"?{x:n.at,z:n.line+r*s,kind:"zebra"}:{x:n.line+r*s,z:n.at,kind:"zebra"});e[i].pair=i+1,e[i+1].pair=i,t.push({a:i,b:i+1,zebra:!0,length:2*s})}for(let n=0;n<e.length;n++)for(let i=n+1;i<e.length;i++){if(e[n].pair===i)continue;let r=Math.hypot(e[n].x-e[i].x,e[n].z-e[i].z);r<rh&&hs(e[n],e[i])&&t.push({a:n,b:i,zebra:!1,length:r})}return e.forEach(Object.freeze),t.forEach(Object.freeze),sh=Object.freeze({nodes:Object.freeze(e),links:Object.freeze(t)}),sh}var bM=.14666666666666667,QE=.88+.02,In=.6,ma=.25,pa=1.2,vM=.35,yM=.25,hr=8,_M=.35,ym=.25,yd=3.2,MM=.6,bm=.85,_m=45,Mm=70,di=.5,lh=.9,vm=1.2,wM=3,Io={sides:12,radius:9.45,wall:4.7,cap:.4,portal:.1},Ls={radius:2.7,pipe:.4,reach:4.1,top:2.4},Po={from:7.3,to:9.3,half:.45,top:5.9},Do=Object.freeze([{id:"agent",x0:-11.5,z0:-23.5,x1:11.5,z1:-.5,stairs:[{x:0,width:7.5}]},{id:"memory",x0:-56,z0:-21,x1:-30,z1:1,stairs:[{x:-48,width:4.2},{x:-38,width:4.2}]},{id:"integrations",x0:30,z0:-18,x1:56,z1:-2,stairs:[{x:35,width:3.4},{x:51,width:3.4}]},{id:"missions",x0:28,z0:25.5,x1:58,z1:46.5,stairs:[{x:43,width:23}]},{id:"infra",x0:-55.5,z0:-64.5,x1:-30.5,z1:-41.5,stairs:[{x:-46.75,width:3},{x:-39.25,width:3}]},{id:"graph",x0:-55,z0:24,x1:-31,z1:48,stairs:[{x:-43,width:4.4}]},{id:"operations",x0:-4.5,z0:31.5,x1:4.5,z1:40.5,stairs:[{x:0,width:2.2}]}].map(s=>Object.freeze(s))),SM=[{id:"agent",district:"agent",asset:"lobby-agent",role:"technician",origin:[0,-12],rect:[-7.5,-19.5,7.5,-4.5],ceiling:5,doors:[[0,0,4,3.5]],struts:!0},{id:"memory",district:"memory",asset:"lobby-memory",role:"archivist",origin:[-43,-10],rect:[-54,-20,-32,-2],ceiling:6.1,doors:[[0,-48,2.8,3.5],[1,-38,2.8,3.5]]},{id:"integrations-west",district:"integrations",asset:"lobby-integrations",role:"technician",origin:[43,-10],rect:[32,-14.5,38,-5.5],ceiling:4.3,doors:[[0,35,2,2.92]]},{id:"integrations-east",district:"integrations",asset:"lobby-integrations",role:"technician",origin:[43,-10],rect:[48,-14.5,54,-5.5],ceiling:4.3,doors:[[1,51,2,2.92]]},{id:"missions",district:"missions",asset:"lobby-missions",role:"archivist",origin:[43,36],rect:[30,27.5,56,42.5],ceiling:6.1,doors:[[0,34,4,3.5],[1,40,4,3.5],[2,46,4,3.5],[3,52,4,3.5]]},{id:"infra",district:"infra",asset:"lobby-infra",role:"technician",origin:[-43,-53],rect:[-53,-62,-33,-50],ceiling:4.5,doors:[[0,-46.75,2.4,3.12],[1,-39.25,2.4,3.12]],towers:[[-50.5,-48],[-43,-48],[-35.5,-48]]},{id:"graph",district:"graph",asset:"lobby-graph",role:"archivist",origin:[-43,36],circle:{x:-43,z:36,...Io},ceiling:null,open:!0,doors:[[0,-43,3.2,3.1]]}],_d=s=>s.radius*Math.cos(Math.PI/s.sides),fi=Object.freeze(SM.map(s=>{let e=s.circle?s.circle.z+_d(s.circle):s.rect[3];return Object.freeze({...s,open:!!s.open,front:e,leaf:e-yM+(s.circle?Io.portal:0),doors:Object.freeze(s.doors.map(([t,n,i,r])=>Object.freeze({n:t,x:n,width:i,height:r})))})})),ga=Object.freeze(fi.flatMap(s=>s.doors.map(e=>Object.freeze({...e,key:"lobby:"+s.id+":"+e.n,lobby:s.id,asset:s.asset,district:s.district,z:s.leaf})))),wm=new Set(fi.map(s=>s.district)),EM=Array.from({length:Io.sides},(s,e)=>[Math.sin(e*2*Math.PI/Io.sides),Math.cos(e*2*Math.PI/Io.sides)]);function ch(s,e,t){if(s.circle){let o=e-s.circle.x,l=t-s.circle.z,c=-1/0;for(let[h,d]of EM)c=Math.max(c,o*h+l*d);return c-_d(s.circle)}let[n,i,r,a]=s.rect;return Math.max(n-e,e-r,i-t,t-a)}var TM=(s,e,t)=>{let[n,i,r,a]=s.rect;return Math.min(e-n,r-e)<pa&&Math.min(t-i,a-t)<pa},AM=(s,e,t)=>{let n=t-s.front;return n>=-In&&n<=ma&&s.doors.some(i=>Math.abs(e-i.x)<i.width/2)},RM=s=>s.rect?[...s.rect]:[s.circle.x-s.circle.radius,s.circle.z-s.circle.radius,s.circle.x+s.circle.radius,s.circle.z+s.circle.radius];function Md(s,e){return Do.find(t=>s>=t.x0&&s<=t.x1&&e>=t.z0&&e<=t.z1)||null}function Fs(s,e){if(Md(s,e))return .88;for(let t of Do){let n=e-t.z1;if(!(n<=0||n>=1.6)){for(let i of t.stairs)if(Math.abs(s-i.x)<=i.width/2)return .88-bM*(Math.min(4,Math.floor(n/.32))+1)}}return null}function Ns(s,e){return fi.find(t=>ch(t,s,e)<=-In)||null}function Sm(s,e,t){return e<hr&&!!Ns(s,t)}function CM(s,e){for(let t of fi){let n=ch(t,s,e);if(!(n>ma)){if(n>-In){if(!AM(t,s,e))return!0;continue}if(t.rect&&TM(t,s,e))return!0}}return!1}var PM=(s,e,t)=>Do.some(n=>Math.max(s.x,e.x)>=n.x0-t&&Math.min(s.x,e.x)<=n.x1+t&&Math.max(s.z,e.z)>=n.z0-t&&Math.min(s.z,e.z)<=n.z1+1.6+t);function Xi(s,e,t){if(!PM(s,e,1))return!0;let n=Math.max(1,Math.ceil(Math.hypot(e.x-s.x,e.z-s.z)/ym)),i=t(s.x,s.z);for(let r=1;r<=n;r++){let a=r/n,o=t(s.x+(e.x-s.x)*a,s.z+(e.z-s.z)*a);if(Math.abs(o-i)>_M)return!1;i=o}return!0}function Em(s,e,t){let n=Math.max(1,Math.ceil(Math.hypot(e.x-s.x,e.z-s.z)/ym));for(let i=0;i<=n;i++){let r=i/n;if(CM(s.x+(e.x-s.x)*r,s.z+(e.z-s.z)*r))return!1}for(let i of ga){if(Math.sign(s.z-i.z)===Math.sign(e.z-i.z))continue;let r=e.z===s.z?0:(i.z-s.z)/(e.z-s.z),a=s.x+(e.x-s.x)*r;if(Math.abs(a-i.x)<i.width/2&&!t(i.key))return!1}return!0}function Tm(s){let e=[],t=(n,i,r,a,o=0,l=hr)=>e.push({x:0,z:0,heading:0,min:[n,o,i],max:[r,l,a]});for(let n of fi)if(n.district===s){if(n.rect){let[i,r,a,o]=n.rect,l=ma;t(i-l,r-l,a+l,r+In),t(i-l,r-l,i+In,o+l),t(a-In,r-l,a+l,o+l);for(let[h,d]of[[i,1],[a,-1]])for(let[u,p]of[[r,1],[o,-1]])t(Math.min(h,h+d*pa),Math.min(u,u+p*pa),Math.max(h,h+d*pa),Math.max(u,u+p*pa));let c=i-l;for(let h of n.doors)t(c,o-In,h.x-h.width/2,o+l),t(h.x-h.width/2,o-In,h.x+h.width/2,o+l,.88+h.height),c=h.x+h.width/2;t(c,o-In,a+l,o+l),t(i-l,r-l,a+l,o+l,n.ceiling)}else{let i=n.circle,r=_d(i),a=i.radius*Math.sin(Math.PI/i.sides)+vM,o=n.doors[0],l=o.x-i.x;for(let h=0;h<i.sides;h++){let d=h*2*Math.PI/i.sides,u=(p,x,M=0)=>e.push({x:i.x,z:i.z,heading:d,min:[p,M,r-In],max:[x,hr,r+ma]});if(h){u(-a,a);continue}u(-a,l-o.width/2),u(l-o.width/2,l+o.width/2,.88+o.height),u(l+o.width/2,a)}let c=i.radius+ma;e.push({x:i.x,z:i.z,heading:0,min:[-c,i.wall+i.cap,-c],max:[c,hr,c]})}for(let[i,r]of n.towers||[])t(i-Ls.radius,r-Ls.radius,i+Ls.radius,r+Ls.radius),t(i-Ls.pipe,r,i+Ls.pipe,r+Ls.reach,0,Ls.top);if(n.struts)for(let i of[1,3,5,7])e.push({x:n.origin[0],z:n.origin[1],heading:i*Math.PI/4,min:[-Po.half,0,Po.from],max:[Po.half,Po.top,Po.to]})}return e}var IM=s=>s*s*(3-2*s);function Am(){let s=new Map(ga.map(r=>[r.key,{door:r,t:0,target:0,held:!1}])),e=new Set,t="high",n=r=>t!=="low"&&e.has(r.lobby),i=(r,a)=>!!a&&Math.hypot(a.x-r.x,a.z-r.z)<yd;return{setTier(r){t=r},setReady(r,a){a?e.add(r):e.delete(r)},usable(r){let a=s.get(r);return!!a&&n(a.door)},hold(r){let a=s.get(r);return!a||!n(a.door)?!1:(a.held=!a.held,!0)},update(r,a,o=[],l=!1){let c=[];for(let h of s.values()){let d=h.door;n(d)||(h.held=!1);let u=n(d)&&(h.held||i(d,a))||o.some(p=>i(d,p))?1:0;u!==h.target&&(h.target=u,c.push(d)),h.t=l?u:Math.min(1,Math.max(0,h.t+(u?1:-1)*Math.max(0,r||0)/MM))}return c},value(r){let a=s.get(r);return a?IM(a.t):0},passable(r){let a=s.get(r);return!!a&&n(a.door)&&a.t>=bm},stats:()=>({open:[...s.values()].filter(r=>r.t>=bm).map(r=>r.door.key),ready:[...e],tier:t})}}var Rm=(s,e)=>s instanceof Map?s.get(e):s?.assets?.find(t=>t.id===e);function DM(s,e){return(Rm(e,s.asset)?.navigation?.workpoints||[]).map(t=>({id:t.id,x:s.origin[0]+t.position[0],z:s.origin[1]+t.position[2]})).filter(t=>Ns(t.x,t.z)?.id===s.id)}function Cm(s){let e=[],t=new Set;for(let n of fi){let i=Rm(s,n.asset);t.has(n.asset)||!i?.navigation?.workpoints?.length||(t.add(n.asset),e.push({place:{id:n.asset,district:n.district,x:n.origin[0],z:n.origin[1],role:n.role,lobby:!0},entry:i}))}return e}function Pm(s=null){let e=[],t=[],n=a=>e.push(Object.freeze(a))-1,i=(a,o)=>t.push([a,o]),r=new Map(Do.map(a=>[a.id,[]]));for(let a of Do)for(let o of a.stairs){let l=n({x:o.x,z:a.z1+1.6+lh,kind:"foot",lobby:null}),c=n({x:o.x,z:a.z1-lh,kind:"landing",lobby:null});i(l,c),r.get(a.id).push(c)}for(let a of fi){let o=[],l=new Map;for(let f of a.doors){let b=n({x:f.x,z:a.front+ma+lh,kind:"porch",lobby:null}),E=n({x:f.x,z:a.front-In-lh,kind:"entry",lobby:a.id});i(b,E),r.get(a.district).push(b),o.push(E)}let[c,h,d,u]=RM(a),p=Math.floor((d-c-2*In-di)/di+1e-9),x=Math.floor((u-h-2*In-di)/di+1e-9);for(let f=0;f<=p;f++)for(let b=0;b<=x;b++){let E=c+In+di/2+f*di,g=h+In+di/2+b*di;ch(a,E,g)<=-In-.3&&l.set(f+","+b,n({x:E,z:g,kind:"grid",lobby:a.id}))}for(let[f,b]of l){let[E,g]=f.split(",").map(Number);for(let[_,w]of[[1,0],[0,1],[1,1],[1,-1]]){let T=l.get(E+_+","+(g+w));T!==void 0&&i(b,T)}}for(let f of DM(a,s))o.push(n({x:f.x,z:f.z,id:f.id,kind:"work",lobby:a.id}));let M=[...l.values()],m=[];for(let f of o)for(let[b,E]of[[1,0],[-1,0],[0,1],[0,-1]]){let g=f;for(let _=1;_*di<=wM+1e-9;_++){let w=e[f].x+b*_*di,T=e[f].z+E*_*di;if(ch(a,w,T)>-In-.3)break;let y=n({x:w,z:T,kind:"grid",lobby:a.id,spur:!0});i(g,y),m.push(y),g=y}}for(let f of o)for(let b of M)Math.hypot(e[f].x-e[b].x,e[f].z-e[b].z)<=vm+1e-9&&i(f,b);for(let f of m)for(let b of M)Math.hypot(e[f].x-e[b].x,e[f].z-e[b].z)<=di*.75+1e-9&&i(f,b);for(let f=0;f<o.length;f++)for(let b=f+1;b<o.length;b++)Math.hypot(e[o[f]].x-e[o[b]].x,e[o[f]].z-e[o[b]].z)<=vm+1e-9&&i(o[f],o[b])}for(let a of r.values())for(let o=0;o<a.length;o++)for(let l=o+1;l<a.length;l++)i(a[o],a[l]);return{nodes:e,links:t}}var wd={integrations:[[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]]},Sd={agent:[[5.9,8,13],[5.4,13,29],[4.7,29,42.5],[4.2,42.5,61.5],[3.6,61.5,72.2],[2.6,72.2,87]]};function Im(s,e){return Md(s,e)?.88:0}function LM(s,e,t){for(let n of t){let i=s-n.x,r=e-n.z;if(n.id==="operations"&&Math.hypot(i,r)<3||(wd[n.id]||[]).some(([a,o,l,c])=>i>a&&i<l&&r>o&&r<c))return!0}return nh.some(n=>Math.abs(s-n.x)<6&&Math.abs(e-n.z)<5)}function Us(s,e){return On.find(t=>Math.abs(s-t.x)<t.width/2&&Math.abs(e-t.z)<t.depth/2)}function Dn(s,e){if(s>=-81.5&&s<=-78.5&&e>=46&&e<=52)return(52-e)/3;if(s>=-78.3&&s<=-75.3&&e>=46&&e<=52)return Math.min(2,Math.ceil((52-e)*2)/6);if(s>=-82&&s<=-75&&e>=34&&e<46)return 2;if(s>=-80&&s<=-77&&e>=28&&e<34)return(e-28)/3;if(Us(s,e))return bt.room;let t=Fs(s,e);if(t!==null)return t;for(let i of ui){let r=!!i.angle&&Math.abs(i.angle)!==Math.PI;if(Math.abs(s-i.platformX)<(r?2:4.5)&&Math.abs(e-i.platformZ)<(r?4.5:2))return bt.pavement+.3}let n=oh(s,e);return n?n.height:e>=74&&e<=80&&s>=-80&&s<=80?bt.quay:bt.ground}function Ed(s,e,t,n,i){if(Math.abs(s)>84||e<-90||e>79)return!1;let r=Us(s,e),a=Us(t.x,t.z);if(r||a){let o=r||a;return!(r?.id!==a?.id&&(Math.abs(s-o.x)>1.15||Math.abs(t.x-o.x)>1.15||Math.min((e-o.doorZ)*o.front,(t.z-o.doorZ)*o.front)<-.9||!n(o.id))||r&&(Math.abs(s-o.x)>5.55||(e-o.z)*o.front<-5.55))}return!Xi(t,{x:s,z:e},Dn)||!Em(t,{x:s,z:e},n)?!1:!LM(s,e,i)}function hh(s,e=new Date){let t=s==="day"?12:s==="evening"?18.5:s==="night"?0:e.getHours()+e.getMinutes()/60;return{hour:t,amount:Math.max(0,Math.min(1,Math.sin((t-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(t-18.5)/2)}}var Dm=[[[-67,-77],[-18,-77],[-18,-32],[-67,-32]],[[-18,-77],[18,-77],[18,-32],[-18,-32]],[[18,-77],[67,-77],[67,-32],[18,-32]],[[18,13],[18,-32],[67,-32],[67,13]],[[-67,-77],[-18,-77],[-18,-32],[-67,-32]]];function Lm(s,e,t){let n=!1,i=0,r=!1,a=!1,o=0,l=!1,c=new Et;c.name="city-life",s.add(c);let h=t.traffic,d=[],u=sm({routes:Dm,obstacles:t.obstacles||[],blocked:(ce,ne,K)=>{let oe=d[ce.route],re=oe?{...oe,x:ne,z:K}:null;return oe?!h.clear(oe,re,re):!1}}),p=new Set,x=new Set,M=new Set,m=[],f=new AbortController,b=[],E=[],g=ce=>(p.add(ce),ce),_=ce=>(x.add(ce),ce),w=g(new Qn(1,.013,5,64));for(let ce of e){let ne=_(new jt({color:7452878,transparent:!0,opacity:.08,depthWrite:!1,toneMapped:!1})),K=new ht(w,ne);K.rotation.x=Math.PI/2,K.position.set(ce.x,.6,ce.z),K.scale.setScalar(ce.radius*1.12),c.add(K),E.push({id:ce.id,ring:K,material:ne,state:"unknown",eventUntil:0,color:new Le(7438733)})}let T=new Map,y=[],C=new Map,S=Date.now(),D=3e3,v=e.find(ce=>ce.id==="agent"),I=new U(0,0,1),P=new U,N={memory:12690431,graph:16765844,infra:7978495,integrations:7728086,missions:10268415,operations:16758915};if(v){for(let ce of e)if(ce!==v){let ne=new U(v.x,v.height+3,v.z),K=new U(ce.x,ce.height+4,ce.z),oe=ne.clone().lerp(K,.5);oe.y=Math.max(ne.y,K.y)+12;let re=new Mi(ne,oe,K),te=_(new Ni({color:N[ce.id],transparent:!0,opacity:0,depthWrite:!1,blending:rn,toneMapped:!1})),fe=new ts(g(new At().setFromPoints(re.getPoints(48))),te);fe.visible=!1,c.add(fe),T.set(ce.id,{curve:re,line:fe})}}let z=g(new Qn(1,.06,5,40,Math.PI*.95));for(let ce=0;ce<12;ce++){let ne=[];for(let K=0;K<4;K++){let oe=_(new jt({color:10345983,transparent:!0,opacity:0,depthWrite:!1,blending:rn,toneMapped:!1})),re=new ht(K===3?w:z,oe);re.visible=!1,c.add(re),ne.push(re)}y.push({at:-1/0,waves:ne,link:null})}function X(ce,ne){if(!l||t.active?.()===!1||ce.at<S||ne-ce.at>D||ce.at>ne||!["started","succeeded","failed","sanitized","progress"].includes(ce.state))return;let K=ce.to==="agent",oe=K?ce.from:ce.to;if(!K&&ce.from!=="agent"||!T.has(oe))return;let re=oe+":"+K;if(ce.at-(C.get(re)??-1/0)<450)return;C.set(re,ce.at);let te=y.reduce((fe,Ee)=>fe.at<Ee.at?fe:Ee);Object.assign(te,{at:ce.at,link:T.get(oe),incoming:K,from:ce.from,to:ce.to,state:ce.state}),te.waves.forEach(fe=>fe.material.color.setHex(ce.state==="failed"?16737881:N[oe]))}function j(ce){let ne=Date.now();T.forEach(K=>{K.line.visible=!1});for(let K of y){(!ce||t.active?.()===!1)&&(K.at=-1/0);let oe=(ne-K.at)/D,re=oe>=0&&oe<1;if(K.waves.forEach(fe=>{fe.visible=!1}),!re||!K.link)continue;K.link.line.visible=!0,K.link.line.material.opacity=.12*Math.sin(oe*Math.PI);for(let fe=0;fe<3;fe++){let Ee=(oe-fe*.075)/.75;if(Ee<0||Ee>1)continue;let Ne=K.incoming?1-Ee:Ee,Oe=K.waves[fe];K.link.curve.getPoint(Ne,Oe.position),K.link.curve.getTangent(Ne,P),K.incoming&&P.negate(),Oe.quaternion.setFromUnitVectors(I,P),Oe.rotateZ(Math.PI*.525),Oe.scale.setScalar(1.2+Math.sin(Ee*Math.PI)*3.5),Oe.material.opacity=Math.sin(Ee*Math.PI)*.8,Oe.visible=!0}let te=(oe-.74)/.26;if(te>0){let fe=K.waves[3];K.link.curve.getPoint(K.incoming?0:1,fe.position),fe.rotation.set(-Math.PI/2,0,0),fe.scale.setScalar(1+te*8),fe.material.opacity=(1-te)*.6,fe.visible=!0}}}let V=g(new dn(7,7)),W=_(new Rt({transparent:!0,depthWrite:!1,blending:rn,uniforms:{color:{value:new Le(5495284)}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"varying vec2 vUv;uniform vec3 color;void main(){float r=length(vUv-.5)*2.;float a=exp(-r*r*5.)*.23*(1.-smoothstep(.65,1.,r));gl_FragColor=vec4(color,a);}"})),ie=g(new $s(.42,1.3,12,1,!0)),H=_(new jt({color:9629439,transparent:!0,opacity:.22,depthWrite:!1,toneMapped:!1,blending:rn}));Dm.forEach((ce,ne)=>{let K=new Et;K.name="city-white-robot-"+(ne+1);let oe=new Et;K.add(oe),c.add(K);let re=new ht(V,W);re.rotation.x=-Math.PI/2,re.position.y=.08,c.add(re);let te=new ht(ie,H);te.rotation.z=Math.PI,te.position.y=-.45,K.add(te),K.visible=!1,re.visible=!1,b.push({shadow:t.shadows?.claim()||null,root:K,body:oe,glow:re,jet:te,agent:u.agents[ne],lastX:0,lastZ:0,phase:0,gait:0})});function ge(ce){ce.traverse(ne=>{if(ne.isMesh){p.add(ne.geometry);for(let K of Array.isArray(ne.material)?ne.material:[ne.material]){x.add(K);for(let oe of Object.values(K))oe?.isTexture&&M.add(oe)}}})}function Ze(){p.forEach(ne=>ne.dispose()),x.forEach(ne=>ne.dispose());let ce=new Set;M.forEach(ne=>{ne.image&&ce.add(ne.image),ne.dispose()}),ce.forEach(ne=>ne.close?.()),p.clear(),x.clear(),M.clear()}async function G(){let ce=setTimeout(()=>f.abort(),15e3);try{let ne=await fetch(t.robotURL,{signal:f.signal});if(!ne.ok)throw Error("Robot unavailable");let K=await ne.arrayBuffer();if(n)return;let{scene:oe}=await new Ri().parseAsync(K,"");if(ge(oe),n){Ze();return}let re=new un().setFromObject(oe),te=re.getSize(new U),fe=Math.max(te.x,te.y,te.z);if(!Number.isFinite(fe)||fe<=0)throw Error("Invalid robot bounds");oe.scale.setScalar(6/fe),oe.updateMatrixWorld(!0),re.setFromObject(oe);let Ee=re.getCenter(new U);oe.position.set(-Ee.x,-re.min.y,-Ee.z),oe.rotation.y=-Math.PI/2,oe.updateMatrixWorld(!0),re.setFromObject(oe),re.expandByScalar(.4);let Ne=cs({min:re.min.toArray(),max:re.max.toArray()});oe.traverse(Oe=>{Oe.isMesh&&(Oe.castShadow=!1,Oe.receiveShadow=!0)}),b.forEach((Oe,ut)=>{if(Oe.body.add(oe.clone(!0)),Oe.root.visible=!0,Oe.glow.visible=!0,h){let rt=Oe.agent,at=null;for(let Q=0;Q<600&&!at;Q++)at=h.register("patrol-"+ut,Ne,{x:rt.x,y:1.6,z:rt.z,heading:rt.heading},1),at||u.seek(rt,rt.s+1);at?(d[ut]=at,t.society?.patrol(at.id,at,Oe.root,(Q,vt)=>{rt.held=Q,vt&&u.adopt(rt,vt)})):(Oe.root.visible=!1,Oe.glow.visible=!1,rt.held=!0)}}),r=!0,A(0,!1)}catch(ne){n||(a=!0,t.onError?.(ne))}finally{clearTimeout(ce)}}function ye(ce){m.forEach(K=>{K.dispose(),x.delete(K)}),m.length=0,ce.children.find(K=>K.userData.district==="operations")?.getObjectByName("signal")?.traverse(K=>{if(!K.isMesh)return;let oe=re=>{let te=_(re.clone());return m.push(te),te};K.material=Array.isArray(K.material)?K.material.map(oe):oe(K.material)})}function J(ce,ne=[]){for(let oe of E){let re=ce.find(te=>te.id===oe.id);oe.state=!re||re.stale?"unknown":re.state,oe.color.setHex(oe.state==="error"?16730430:oe.state==="running"?7400403:oe.state==="unknown"?7438733:7452878),oe.material.color.copy(oe.color)}let K=Date.now();for(let oe of[...ne].reverse())if(oe.id>o&&(X(oe,K),l&&oe.at>=S&&t.active?.()!==!1&&t.society?.event(oe),K-oe.at<6e3)){let re=E.find(te=>te.id===oe.district);re&&(re.eventUntil=oe.at+5e3)}o=Math.max(o,...ne.map(oe=>oe.id))}function A(ce,ne){if(n)return;l=ne,j(ne);let K=ne?Math.min(.1,Math.max(0,ce)):0,oe=h?u.agents.map(re=>({...re})):null;ne&&K>0&&(i+=K,u.step(K));for(let[re,te]of b.entries()){let fe=te.agent,Ee=Math.hypot(fe.vx,fe.vz),Ne=(!ne||fe.held)&&d[re]?d[re]:fe,Oe=Math.hypot(Ne.x-te.lastX,Ne.z-te.lastZ),ut=Oe>.5?0:Oe;te.lastX=Ne.x,te.lastZ=Ne.z,te.phase+=ut*1.4,te.gait=It.damp(te.gait,Math.min(1,ut/Math.max(K,1e-4)/1.2),6,K);let rt=te.gait,at=1-rt;te.root.position.set(Ne.x,1.6+Math.sin(te.phase)*.14*rt+Math.sin(i*.7+re*2.1)*.03*at,Ne.z),te.root.rotation.y=Ne.heading;let Q=(fe.vx*fe.rx+fe.vz*fe.rz)/Math.max(1,Ee);te.body.rotation.z=Math.sin(i*.85+re)*.035*rt-Q*.08,te.body.rotation.x=-.035+Math.sin(i*1.1+re)*.018*rt-Math.min(.09,Ee*.015),te.glow.position.x=Ne.x,te.glow.position.z=Ne.z,te.jet.scale.y=1+Math.sin(i*4+re)*.12+Ee*.04;let vt=Dn(te.glow.position.x,te.glow.position.z);if(te.glow.position.y=vt+.08,te.shadow&&(te.root.visible?te.shadow.set(te.glow.position.x,vt,te.glow.position.z,1.1,.3):te.shadow.hide()),ne&&d[re]&&!fe.held){let je=d[re],O={x:fe.x,y:1.6,z:fe.z,heading:fe.heading};te.escape&&i<te.escape.until?(O.x=je.x+te.escape.x*K*1.8,O.z=je.z+te.escape.z*K*1.8,O.heading=je.heading):te.escape=null,!h.clear(je,je,O,!1)&&h.clear(je,je,{...O,heading:je.heading},!1)&&(O.heading=je.heading);let R=Math.hypot(O.x-je.x,O.z-je.z);h.propose(je,O,(ee,ae)=>{if(te.wait=ee&&R>.002?0:(te.wait||0)+K,te.escape&&ee&&u.adopt(fe,ae),te.wait>2.5){let _e=Math.sin(ae.heading),Ue=Math.cos(ae.heading);for(let[ke,B]of[[Ue,-_e],[-Ue,_e],[_e,Ue],[-_e,-Ue]])if(h.clear(ae,ae,{...ae,x:ae.x+ke*2.5,z:ae.z+B*2.5})){te.escape={x:ke,z:B,until:i+2};break}te.wait=0}if(ee)fe.trafficWait=0,fe.heading=ae.heading;else{let _e=(fe.trafficWait||0)+K;Object.assign(fe,oe[re]),fe.speed=0,fe.vx=fe.vz=0,fe.trafficWait=_e,_e>2.5&&(fe.state!=="turn"?(fe.dir=-fe.dir,fe.lane=-fe.lane,fe.laneTarget=2.2,fe.state="turn",fe.turns++):fe.state="cruise",fe.trafficWait=0)}te.root.position.x=ae.x,te.root.position.z=ae.z,te.root.rotation.y=ae.heading,te.glow.position.x=ae.x,te.glow.position.z=ae.z})}}for(let re of E){let te=ne?.5+.5*Math.sin(i*(re.state==="error"?3.2:1.8)):.5,fe=re.state==="error"||re.state==="running"||re.eventUntil>Date.now();if(re.material.opacity=re.state==="unknown"?.03:fe?.2+te*.46:.065,re.ring.scale.setScalar(e.find(Ee=>Ee.id===re.id).radius*1.12*(1+(fe&&ne?te*.035:0))),re.id==="operations")for(let Ee of m)Ee.color.copy(re.color),Ee.emissive?.copy(re.color),Ee.emissiveIntensity=re.state==="error"?1.1+te*2.2:re.state==="unknown"?.08:.55}}function me(){n||(n=!0,f.abort(),t.signal?.removeEventListener("abort",me),c.removeFromParent(),Ze(),b.length=0,m.length=0,d.forEach(ce=>h.remove(ce.id)))}return t.signal?.addEventListener("abort",me,{once:!0}),t.signal?.aborted?me():G(),{update:A,setData:J,attachLandmarks:ye,dispose:me,stats:()=>({robots:r?b.filter(ce=>ce.root.visible).length:0,robotError:a,time:i,navigation:{...u.stats(),gaits:b.map(ce=>+(ce.gait||0).toFixed(2)),held:b.map(ce=>!!ce.agent.held),enabled:b.map(ce=>!!d[b.indexOf(ce)]?.enabled)},positions:b.map(ce=>ce.root.position.toArray()),transmissions:y.filter(ce=>ce.waves.some(ne=>ne.visible)).map(ce=>({from:ce.from,to:ce.to,state:ce.state,age:Date.now()-ce.at})),signals:E.map(ce=>({id:ce.id,state:ce.state,intensity:ce.material.opacity}))})}}var Fo='Geist, "Segoe UI", system-ui, sans-serif',uh=13,FM=s=>{let e=2166136261;for(let t=0;t<s.length;t++)e=Math.imul(e^s.charCodeAt(t),16777619);return(e>>>0).toString(16).padStart(8,"0")},Td={vertex:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:`uniform sampler2D map;uniform float time,glitch,fade,gain;uniform vec3 tint;varying vec2 vUv;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){vec2 uv=vUv;float row=floor(uv.y*44.0),frame=floor(time*24.0);
float g=glitch*step(0.7,hash(row+frame*0.37));uv.x+=(hash(row*3.1+frame)-0.5)*0.14*g;
float split=0.0025+glitch*0.02;vec4 c=texture2D(map,uv);
float r=texture2D(map,uv+vec2(split,0.0)).r,b=texture2D(map,uv-vec2(split,0.0)).b;
float a=max(c.a,max(texture2D(map,uv+vec2(split,0.0)).a,texture2D(map,uv-vec2(split,0.0)).a));
float lines=0.84+0.16*sin(uv.y*420.0-time*9.0);float flicker=0.95+0.05*sin(time*31.0)*sin(time*7.3);
float bx=(fract(uv.y*0.55-time*0.11)-0.5)*22.0;float band=0.35*exp(-bx*bx);
float edge=smoothstep(0.0,0.05,uv.x)*smoothstep(0.0,0.05,1.0-uv.x)*smoothstep(0.0,0.09,uv.y)*smoothstep(0.0,0.09,1.0-uv.y);
vec3 col=vec3(r,c.g,b)*tint*(lines*flicker+band)*gain;gl_FragColor=vec4(col*a*fade*edge,1.0);}`},NM={vertex:"varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",fragment:`uniform float time,strength;uniform vec3 base,top;varying vec2 vUv;varying vec3 vNormal,vView;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){float h=vUv.y;float rim=pow(clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0),1.5);
float fall=pow(clamp(1.0-h,0.0,1.0),1.35)*0.75+0.25*clamp(1.0-h,0.0,1.0);float scan=0.7+0.3*sin(h*64.0-time*5.5);
float noise=0.85+0.15*hash(floor(h*96.0)+floor(time*18.0));float shimmer=0.8+0.2*sin(vUv.x*40.0+time*2.0);
float a=(0.28+0.72*rim)*fall*scan*noise*shimmer*strength;gl_FragColor=vec4(mix(base,top,h)*a,1.0);}`},UM={vertex:Td.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float r=length(vUv-0.5)*2.0;float rings=smoothstep(0.35,1.0,sin(r*16.0-time*2.6));
float core=exp(-r*r*7.0);float a=(core*1.2+pow(max(0.0,1.0-r),1.8)*(0.25+0.75*rings)*0.6)*strength*(1.0-smoothstep(0.85,1.0,r));
gl_FragColor=vec4(color*a,1.0);}`},Fm={vertex:Td.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float dash=step(0.45,fract(vUv.x*28.0+time*0.35));float pulse=0.75+0.25*sin(vUv.x*6.2831*3.0-time*4.0);
gl_FragColor=vec4(color*dash*pulse*strength,1.0);}`},zM={vertex:`attribute float seed;uniform float time,pointScale;varying float vLife;
void main(){float speed=0.55+seed*0.9;float life=fract(seed*3.17+time*speed/13.0);float y=life*13.0;
float ang=seed*6.2831+time*(0.35+seed*0.4)+life*2.2;float rad=mix(1.8,6.8,life)*(0.3+0.7*fract(seed*7.13));
vec4 mv=modelViewMatrix*vec4(cos(ang)*rad,y,sin(ang)*rad,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.09+0.16*fract(seed*3.7))*pointScale/max(1.0,-mv.z);vLife=life;}`,fragment:`uniform vec3 color;uniform float strength;varying float vLife;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,2.0)*sin(vLife*3.1416)*strength;gl_FragColor=vec4(color*a,1.0);}`};function xa(s,e,t={}){return new Rt({uniforms:e,vertexShader:s.vertex,fragmentShader:s.fragment,transparent:!0,depthWrite:!1,blending:rn,side:Kt,...t})}function OM(s,e=24){let t=[],n=new Set;for(let i of Array.isArray(s)?s:[]){if(typeof i!="string")continue;let r=i.replace(/[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u2028-\u202e\u2066-\u2069]/g," ").replace(/\s+/g," ").trim().slice(0,140);if(!(r.length<4||n.has(r))&&(n.add(r),t.push(r),t.length>=e))break}return t}function Nm(s,e,t,n){let i=[],r=c=>s.measureText(c).width<=t,a="";for(let c of e.split(" ")){if(i.length>=n)break;let h=a?a+" "+c:c;if(r(h)){a=h;continue}if(a&&(i.push(a),a="",i.length>=n))break;let d=c;for(;!r(d)&&i.length<n;){let u=d.length-1;for(;u>1&&!r(d.slice(0,u));)u--;i.push(d.slice(0,u)),d=d.slice(u)}a=d}a&&i.length<n&&(i.push(a),a="");let o=i.join("").replace(/\s/g,"").length,l=e.replace(/\s/g,"").length;if(o<l&&i.length){let c=i[i.length-1].replace(/[\s,.;:]+$/,"");for(;c.length&&!r(c+"\u2026");)c=c.slice(0,-1);i[i.length-1]=c+"\u2026"}return i.length?i:[""]}function Um(s,e,t={}){let n=t.roof??21,i=String(t.label||"MEMORY").toUpperCase(),r=new Et;r.name="memory-hologram",r.position.set(e.x,n,e.z),s.add(r);let a=[],o=[],l=[],c=(K,oe)=>(a.push(K),o.push(oe),new ht(K,oe)),h=new Le(6544639),d=new Le(10980351),u=new Le(14218751),p={time:{value:0}},x=!1,M=0,m=[],f=[],b=0,E="none",g=!1,_=c(new dn(11,11),xa(UM,{time:p.time,strength:{value:.9},color:{value:h}}));_.rotation.x=-Math.PI/2,_.position.y=.18,r.add(_);let w=c(new ws(7.4,2.3,uh,56,1,!0),xa(NM,{time:p.time,strength:{value:.32},base:{value:h},top:{value:d}}));w.position.y=uh/2+.3,r.add(w);let T=c(new Qn(7.5,.055,6,128),xa(Fm,{time:p.time,strength:{value:1.6},color:{value:h}}));T.rotation.x=Math.PI/2,T.position.y=uh+.3,r.add(T);let y=c(new Qn(2.6,.05,6,96),xa(Fm,{time:p.time,strength:{value:2},color:{value:u}}));y.rotation.x=Math.PI/2,y.position.y=.55,r.add(y);let C=new Xr(1.35,1),S=new ja(C),D=new Ni({color:10217983,transparent:!0,opacity:.75,blending:rn,depthWrite:!1,toneMapped:!1}),v=new Js(S,D);v.position.y=4.6,r.add(v),a.push(C,S),o.push(D);let I=new jt({color:4175871,transparent:!0,opacity:.12,blending:rn,depthWrite:!1,toneMapped:!1}),P=c(new Xr(1.1,1),I);v.add(P);let N=160,z=new Float32Array(N);for(let K=0;K<N;K++)z[K]=K*.618033988749895%1;let X=new At;X.setAttribute("position",new Pt(new Float32Array(N*3),3)),X.setAttribute("seed",new Pt(z,1)),X.boundingSphere=new Un(new U(0,uh/2,0),12);let j=xa(zM,{time:p.time,pointScale:{value:800},color:{value:u},strength:{value:.9}}),V=new Rn(X,j);V.frustumCulled=!1,r.add(V),a.push(X),o.push(j);let W=new wi(6478079,26,46,2);W.position.y=6,r.add(W);function ie(K,oe,re,te){let fe=document.createElement("canvas");fe.width=re,fe.height=te;let Ee=fe.getContext("2d"),Ne=new Zs(fe);Ne.colorSpace=an,Ne.generateMipmaps=!1,Ne.minFilter=on,l.push(Ne);let Oe=xa(Td,{map:{value:Ne},time:p.time,glitch:{value:0},fade:{value:0},gain:{value:1.35},tint:{value:new Le(16777215)}},{side:oi}),ut=c(new dn(K,oe),Oe);return{canvas:fe,context:Ee,texture:Ne,material:Oe,mesh:ut,text:"",targetFade:1,next:0}}let H=ie(17.5,8.55,1024,500),ge=[ie(8.6,2.1,640,156),ie(8.6,2.1,640,156)];H.mesh.position.y=10.4,r.add(H.mesh),ge.forEach((K,oe)=>{K.mesh.position.y=oe?14.4:5.9,K.orbit=oe?-.22:.27,K.angle=oe*2.3,r.add(K.mesh)});function Ze(K,oe,re){let{context:te,canvas:fe}=H,Ee=fe.width,Ne=fe.height;te.clearRect(0,0,Ee,Ne),te.fillStyle="rgba(48,150,214,0.14)",te.beginPath(),te.roundRect(14,14,Ee-28,Ne-28,22),te.fill(),te.strokeStyle="rgba(150,230,255,0.85)",te.lineWidth=3;for(let[Oe,ut,rt,at]of[[18,18,1,1],[Ee-18,18,-1,1],[18,Ne-18,1,-1],[Ee-18,Ne-18,-1,-1]])te.beginPath(),te.moveTo(Oe,ut+at*42),te.lineTo(Oe,ut),te.lineTo(Oe+rt*42,ut),te.stroke();te.fillStyle="rgba(150,230,255,0.55)",te.fillRect(48,108,Ee-96,2),te.font="600 27px "+Fo,te.textBaseline="middle",te.fillStyle="rgba(160,232,255,0.92)",te.textAlign="left",te.fillText("\u258C "+i+(re?"  \xB7  "+String(oe+1).padStart(2,"0")+" / "+String(re).padStart(2,"0"):""),50,72),te.textAlign="right",te.font="500 25px "+Fo,te.fillStyle="rgba(190,150,255,0.85)",te.fillText("0x"+FM(K||i).toUpperCase(),Ee-52,72),te.textAlign="left",te.shadowColor="rgba(120,225,255,0.9)",te.shadowBlur=16,K?(te.font="600 54px "+Fo,te.fillStyle="rgba(232,250,255,0.97)",Nm(te,K,Ee-110,4).forEach((ut,rt)=>te.fillText(ut,54,172+rt*72))):(te.font="600 40px "+Fo,te.fillStyle="rgba(180,235,255,0.7)",te.fillText("\u25AE \u25AE \u25AF \u25AE \u25AF \u25AF \u25AE \u25AF \u25AE \u25AE \u25AF \u25AE",54,250)),te.shadowBlur=0,H.texture.needsUpdate=!0}function G(K,oe){let{context:re,canvas:te}=K,fe=te.width,Ee=te.height;re.clearRect(0,0,fe,Ee),re.fillStyle="rgba(48,150,214,0.12)",re.beginPath(),re.roundRect(6,6,fe-12,Ee-12,14),re.fill(),re.fillStyle="rgba(190,150,255,0.8)",re.fillRect(20,26,6,Ee-52),re.font="600 42px "+Fo,re.textBaseline="middle",re.textAlign="left",re.shadowColor="rgba(160,140,255,0.9)",re.shadowBlur=12,re.fillStyle="rgba(236,240,255,0.95)",re.fillText(oe?Nm(re,oe,fe-70,1)[0]:"\u25AF \u25AE \u25AF \u25AE \u25AF",42,Ee/2),re.shadowBlur=0,K.texture.needsUpdate=!0}function ye(){return m.length?(b>=f.length&&(f=m.map((K,oe)=>oe).sort(()=>Math.random()-.5),b=0),m[f[b++]]):""}let J=.8,A=[3.5,6.5];function me(K){let oe=ye();H.text=oe,M++,Ze(oe,m.indexOf(oe),m.length),H.material.uniforms.glitch.value=K?1:0,H.material.uniforms.fade.value=K?.35:1,J=x?12:4.5+Math.random()*3}function ce(K,oe){let re=ge[K],te=ye();re.text=te,G(re,te.length>46?te.slice(0,44).replace(/\s+\S*$/,"")+"\u2026":te),re.material.uniforms.glitch.value=oe?.7:0,re.material.uniforms.fade.value=oe?.3:1,A[K]=x?15:6+Math.random()*4}Ze("",0,0),ge.forEach(K=>G(K,"")),document.fonts?.ready?.then(()=>{g||(Ze(H.text,m.indexOf(H.text),m.length),ge.forEach(K=>G(K,K.text)))});let ne=new U;return{group:r,setTexts(K,oe="live"){let re=OM(K),te=re.length!==m.length||re.some((fe,Ee)=>fe!==m[Ee]);m=re,E=re.length?oe:"none",te&&(f=[],b=0,(!H.text||!m.includes(H.text))&&(J=Math.min(J,.6)))},setPointScale(K){j.uniforms.pointScale.value=K},setReducedMotion(K){x=!!K},update(K,oe,re){if(g)return;let te=re&&!x,fe=te?Math.min(.1,Math.max(0,K)):0;p.time.value+=fe,J-=K,J<=0&&me(te),A.forEach((Ee,Ne)=>{A[Ne]=Ee-K,A[Ne]<=0&&ce(Ne,te)});for(let Ee of[H,...ge]){let Ne=Ee.material.uniforms;Ne.glitch.value=Math.max(0,Ne.glitch.value-K*2.4),Ne.fade.value=Math.min(1,Ne.fade.value+K*1.8),Ee.mesh.quaternion.copy(oe.quaternion)}ge.forEach(Ee=>{Ee.angle+=Ee.orbit*fe,Ee.mesh.position.x=Math.cos(Ee.angle)*6.4,Ee.mesh.position.z=Math.sin(Ee.angle)*6.4,ne.copy(Ee.mesh.position).add(r.position).sub(oe.position).normalize();let Ne=-(ne.x*Math.cos(Ee.angle)+ne.z*Math.sin(Ee.angle));Ee.material.uniforms.gain.value=1.35*It.clamp(.35+Ne*.9,.15,1.2)}),H.mesh.position.y=9.6+Math.sin(p.time.value*.9)*.22,v.rotation.y+=fe*.7,v.rotation.x+=fe*.31,P.rotation.y-=fe*1.1,T.rotation.z+=fe*.18,y.rotation.z-=fe*.42,W.intensity=26+Math.sin(p.time.value*2.1)*6+H.material.uniforms.glitch.value*22},stats:()=>({artifacts:m.length,source:E,switches:M,shown:H.text?m.indexOf(H.text):-1}),dispose(){g||(g=!0,r.removeFromParent(),a.forEach(K=>K.dispose()),o.forEach(K=>K.dispose()),l.forEach(K=>K.dispose()),W.dispose())}}}var kM=`uniform sampler2D tDiffuse;uniform float time,vignette,grain,aberration,streak,shafts,letterbox,aspect,contrast,saturation;
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
gl_FragColor=vec4(c,1.0);}`;function zm(s){let e={tDiffuse:{value:null},time:s,vignette:{value:.5},grain:{value:.03},aberration:{value:.0024},streak:{value:.16},shafts:{value:0},letterbox:{value:0},aspect:{value:1.7777777777777777},contrast:{value:.22},saturation:{value:1.08},lightPos:{value:new Ge(.5,.5)},shaftColor:{value:new Le(12376319)},shadowTint:{value:new Le(6686)},highlightTint:{value:new Le(1444864)}},t=new ua(new Rt({uniforms:e,vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:kM}));t.enabled=!1;let n=new U,i=new Le(16761992),r=new Le(12376319),a=!1;return{pass:t,setCinematic(o){a=!!o},update(o,l,c,{day:h,dusk:d,cloud:u,animated:p}){e.aspect.value=l.aspect;let x=a?1:0;e.letterbox.value=p?e.letterbox.value+(x-e.letterbox.value)*Math.min(1,o*2.2):x,n.copy(c).multiplyScalar(1e3).add(l.position).project(l);let M=(n.x+1)/2,m=(n.y+1)/2,f=n.z<1?Math.max(0,1-Math.max(0,Math.abs(M-.5)-.5,Math.abs(m-.5)-.5)*4):0;e.lightPos.value.set(M,m),e.shafts.value=f*(.06+d*.14+(1-h)*.03)*(1-u*.75),e.shaftColor.value.copy(r).lerp(i,d),e.shadowTint.value.setRGB(0,.018+.01*(1-h),.024+.01*(1-h)),e.highlightTint.value.setRGB(.02+d*.03,.01+d*.006,0)},stats:()=>({enabled:t.enabled,letterbox:+e.letterbox.value.toFixed(2),shafts:+e.shafts.value.toFixed(2)}),dispose(){t.material.dispose(),t.dispose?.()}}}var BM=new Le(16767392),HM=new Le(16758903),Rd=`float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`,Ad="varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}",GM="float fbm(vec2 p){float v=0.0,a=0.5;for(int i=0;i<5;i++){if(float(i)>=octaves)break;v+=a*vnoise(p);p=p*2.03+vec2(1.7,9.2);a*=0.5;}return v;}",VM=`uniform float time,day,dusk,cloud,flash,octaves;uniform vec3 zenith,horizon,haze,warm,sunDir,glow;varying vec3 vWorld;${Rd}${GM}
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
gl_FragColor=vec4(col,1.0);}`,Om={vertex:`attribute float phase,speed,size;uniform float time,pointScale,cloud;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0)*(1.0-cloud*0.85);
float k=fract(phase*3.1);vTint=k>0.86?vec3(1.0,0.84,0.66):k>0.7?vec3(0.7,0.82,1.0):vec3(0.82,0.9,1.0);}`,fragment:"varying float vAlpha;varying vec3 vTint;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vTint*a,1.0);}"},km={vertex:`varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,fragment:`uniform float time,day;uniform vec3 deep,shallow,sky,sunDir,sunColor,glow;uniform vec4 island;varying vec3 vWorld;
#include <fog_pars_fragment>
${Rd}
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
}`},WM=`uniform float time,strength;uniform vec3 color;uniform vec4 island;varying vec3 vWorld;${Rd}
void main(){vec2 p=vWorld.xz;float n=vnoise(p*0.012+vec2(time*0.017,-time*0.011))*0.6+vnoise(p*0.031-vec2(time*0.02,time*0.013))*0.4;
float mist=smoothstep(0.32,0.82,n);vec2 d2=max(abs(p-island.xy)-island.zw,0.0);float outside=smoothstep(0.0,70.0,length(d2));
float far=1.0-smoothstep(500.0,850.0,length(p));gl_FragColor=vec4(color,mist*outside*far*strength);}`,Bm={vertex:`attribute vec3 seed;uniform float time,pointScale;varying float vAlpha;
void main(){vec3 p=position;p.x+=sin(time*0.11*seed.x+seed.y*6.28)*9.0+time*0.35*(seed.z-0.5);p.y+=sin(time*0.13+seed.z*6.28)*3.0;
p.z+=cos(time*0.09*seed.y+seed.x*6.28)*9.0;p.x=mod(p.x+90.0,180.0)-90.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.06+0.11*seed.x)*pointScale/max(1.0,-mv.z);vAlpha=0.55+0.45*sin(time*(1.0+seed.y)+seed.z*6.28);}`,fragment:`uniform vec3 color;uniform float strength;varying float vAlpha;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,2.2)*vAlpha*strength,1.0);}`},qM=`uniform float strength;uniform vec3 color;varying vec2 vUv;
void main(){float along=pow(clamp(1.0-vUv.x,0.0,1.0),2.4);float across=pow(clamp(1.0-abs(vUv.y-0.5)*2.0,0.0,1.0),1.7);gl_FragColor=vec4(color*along*across*strength,1.0);}`,XM=`uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;
void main(){float facing=clamp(abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0);float h=clamp(vUv.y,0.0,1.0);
float a=pow(h,2.4)*pow(facing,1.5)*strength;gl_FragColor=vec4(color*a,1.0);}`,jM=`uniform vec3 color;uniform float strength;varying vec2 vUv;
void main(){float d=clamp(length(vUv-0.5)*2.0,0.0,1.0);float f=1.0-smoothstep(0.0,1.0,d);f=f*(0.35+0.65*f);gl_FragColor=vec4(color*f*strength,1.0);}`,Hm={vertex:"attribute float lift;varying float vLift;void main(){vLift=lift;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:"uniform vec3 haze;uniform float shade;varying float vLift;void main(){gl_FragColor=vec4(haze*mix(1.0,shade,smoothstep(0.0,0.6,vLift)),1.0);}"},Gm={vertex:`attribute float phase;uniform float time,pointScale,strength;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=strength*(0.75+0.25*sin(time*(0.7+fract(phase)*1.3)+phase));gl_PointSize=clamp(2.2*pointScale/max(1.0,-mv.z),1.5,4.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(1.4,0.95,0.55)*pow(1.0-d,1.5)*vA,1.0);}"},Vm={vertex:"varying vec2 vUv;varying float vRim,vNear;void main(){vUv=uv;vec3 n=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vRim=abs(dot(n,normalize(-mv.xyz)));vNear=smoothstep(40.0,160.0,-mv.z);gl_Position=projectionMatrix*mv;}",fragment:"uniform vec3 color;uniform float strength;varying vec2 vUv;varying float vRim,vNear;void main(){float along=pow(clamp(1.0-vUv.y,0.0,1.0),1.6)*smoothstep(0.0,0.03,vUv.y);gl_FragColor=vec4(color*along*pow(clamp(vRim,0.0,1.0),2.5)*vNear*strength,1.0);}"},Wm={vertex:`attribute float phase;uniform float time,pointScale;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=0.22+0.78*pow(max(0.0,sin(time*1.9+phase)),10.0);gl_PointSize=clamp(1.5*pointScale/max(1.0,-mv.z),2.0,14.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(3.2,0.35,0.25)*pow(1.0-d,2.0)*vA,1.0);}"};function qm(s,e={}){let t=[],n=[],i=new Et;i.name="city-atmosphere",s.add(i);let r=(Fe,Je)=>(t.push(Fe),n.push(Je),new ht(Fe,Je)),a={value:0},o={value:900},l={value:0},c={value:0},h={value:.25},d={value:0},u={value:5},p=new Le(4860440),x=(e.sunDirection||new U(-70,145,85)).clone().normalize(),M=new Le(330010),m=new Le(1323596),f=new Le(8011050),b=new Le(s.fog?.color||528926),E=(Fe,Je,F,q={})=>new Rt({uniforms:F,vertexShader:Fe,fragmentShader:Je,...q}),g={transparent:!0,depthWrite:!1,blending:rn},_=r(new Ui(1500,48,24),E(Ad,VM,{time:a,day:l,dusk:c,cloud:h,flash:d,octaves:u,glow:{value:p},zenith:{value:M},horizon:{value:m},haze:{value:b},warm:{value:f},sunDir:{value:x}},{side:yn,depthWrite:!1,fog:!1}));_.frustumCulled=!1,_.renderOrder=-10,i.add(_);let w=1400,T=new Float32Array(w*3),y=new Float32Array(w),C=new Float32Array(w),S=new Float32Array(w),D=12345,v=()=>(D=Math.imul(D,1664525)+1013904223>>>0)/4294967296;for(let Fe=0;Fe<w;Fe++){let Je=v()*Math.PI*2,F=.03+v()*.97,q=Math.sqrt(1-F*F);T.set([Math.cos(Je)*q*1450,F*1450,Math.sin(Je)*q*1450],Fe*3),y[Fe]=v()*Math.PI*2,C[Fe]=.4+v()*1.6,S[Fe]=8e-4+v()*v()*.0022}let I=new At;I.setAttribute("position",new Pt(T,3)),I.setAttribute("phase",new Pt(y,1)),I.setAttribute("speed",new Pt(C,1)),I.setAttribute("size",new Pt(S,1));let P=E(Om.vertex,Om.fragment,{time:a,pointScale:o,cloud:h},{...g,fog:!1}),N=new Rn(I,P);N.frustumCulled=!1,N.renderOrder=-9,i.add(N),t.push(I),n.push(P);let z=r(new dn(2800,2800),E(km.vertex,km.fragment,{time:a,day:l,glow:{value:p},island:{value:new Ot(0,-7,85,87)},deep:{value:new Le(398368)},shallow:{value:new Le(930640)},sky:{value:m.clone().multiplyScalar(.7)},sunDir:{value:x},sunColor:{value:new Le(14676223)},fogColor:{value:new Le},fogDensity:{value:0},fogNear:{value:1},fogFar:{value:1e3}},{fog:!0}));z.rotation.x=-Math.PI/2,z.position.y=-3.2,i.add(z);let X=r(new dn(1900,1900),E(Ad,WM,{time:a,strength:{value:.55},color:{value:new Le(1717320)},island:{value:new Ot(0,-7,85,87)}},{transparent:!0,depthWrite:!1,fog:!1}));X.rotation.x=-Math.PI/2,X.position.y=-2.4,i.add(X);let j=500,V=new Float32Array(j*3),W=new Float32Array(j*3);for(let Fe=0;Fe<j;Fe++)V.set([(v()-.5)*180,2+v()*v()*75,-95+v()*170],Fe*3),W.set([v(),v(),v()],Fe*3);let ie=new At;ie.setAttribute("position",new Pt(V,3)),ie.setAttribute("seed",new Pt(W,3));let H=E(Bm.vertex,Bm.fragment,{time:a,pointScale:o,color:{value:new Le(10475775)},strength:{value:.32}},g),ge=new Rn(ie,H);ge.frustumCulled=!1,i.add(ge),t.push(ie),n.push(H);let Ze=new Et;Ze.position.set(0,86.5,-12),i.add(Ze);let G=E(Ad,qM,{strength:{value:.45},color:{value:new Le(9430783)}},{...g,side:Kt}),ye=new dn(100,5.2);ye.translate(50,0,0),t.push(ye),n.push(G);let J=new ht(ye,G),A=new ht(ye,G);A.rotation.x=Math.PI/2;let me=new Et;me.add(J,A),me.rotation.z=-.07,Ze.add(me);let ce=new jt({color:12579583,toneMapped:!1}),ne=r(new Ui(.9,16,12),ce);Ze.add(ne);let K=(e.lamps||[]).slice(0,64),oe=E(`varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 p=vec4(position,1.0);
#ifdef USE_INSTANCING
p=instanceMatrix*p;
#endif
vec4 mv=modelViewMatrix*p;vView=-mv.xyz;gl_Position=projectionMatrix*mv;}`,XM,{color:{value:new Le(16767392)},strength:{value:.3}},{...g,side:Kt}),re=new $s(2.7,6.5,18,1,!0),te=new An(re,oe,Math.max(1,K.length));t.push(re),n.push(oe);let fe=new Yt;K.forEach((Fe,Je)=>{{let F=(Fe.y||0)+6.5,q=F-bt.pavement;fe.position.set(Fe.x+1.55*Math.cos(Fe.angle||0),bt.pavement+q/2,Fe.z-1.55*Math.sin(Fe.angle||0)),fe.scale.set(1,q/6.5,1),fe.updateMatrix(),fe.scale.set(1,1,1)}te.setMatrixAt(Je,fe.matrix)}),te.count=K.length,te.instanceMatrix.needsUpdate=!0,i.add(te);let Ee=E(`varying vec2 vUv;void main(){vUv=uv;vec4 p=vec4(position,1.0);
#ifdef USE_INSTANCING
p=instanceMatrix*p;
#endif
gl_Position=projectionMatrix*modelViewMatrix*p;}`,jM,{color:{value:new Le(16767392)},strength:{value:.3}},{...g,polygonOffset:!0,polygonOffsetFactor:-2,polygonOffsetUnits:-2}),Ne=new dn(1,1);Ne.rotateX(-Math.PI/2);let Oe=new An(Ne,Ee,Math.max(1,K.length));Oe.name="lamp-pools",Oe.renderOrder=1,Oe.frustumCulled=!1,t.push(Ne),n.push(Ee),K.forEach((Fe,Je)=>{fe.position.set(Fe.x+1.55*Math.cos(Fe.angle||0),bt.pavement+.02,Fe.z-1.55*Math.sin(Fe.angle||0)),fe.scale.set(6.8,1,6.8),fe.updateMatrix(),Oe.setMatrixAt(Je,fe.matrix)}),fe.scale.set(1,1,1),Oe.count=K.length,Oe.instanceMatrix.needsUpdate=!0,i.add(Oe);let ut=[],rt=[],at=[],Q=[],vt=[],je=360,O=Fe=>Math.max(0,.45+.3*Math.sin(Fe*5+1.3)+.2*Math.sin(Fe*13+.4)+.12*Math.sin(Fe*31+2.1))*(.35+.65*Math.max(0,Math.sin(Fe*2+.6)));for(let Fe=0;Fe<=je;Fe++){let Je=Math.PI*.8+Math.PI*1.45*Fe/je,F=1180+40*Math.sin(Je*7),q=6+74*O(Je);if(ut.push(Math.cos(Je)*F,-3.5,Math.sin(Je)*F,Math.cos(Je)*F,q,Math.sin(Je)*F),rt.push(0,q/80),Fe<je){let $=Fe*2;at.push($,$+1,$+2,$+1,$+3,$+2)}}for(let Fe=0;Fe<90;Fe++){let Je=Math.PI*.82+Math.PI*1.41*v(),F=1170+40*Math.sin(Je*7);O(Je)<.08||(Q.push(Math.cos(Je)*F,1+v()*v()*28,Math.sin(Je)*F),vt.push(v()*20))}let R=new At;R.setAttribute("position",new Mt(ut,3)),R.setAttribute("lift",new Mt(rt,1)),R.setIndex(at);let ee={value:.8},ae=r(R,E(Hm.vertex,Hm.fragment,{haze:{value:b},shade:ee},{fog:!1,side:Kt}));ae.frustumCulled=!1,ae.renderOrder=-8,i.add(ae);let _e=new At;_e.setAttribute("position",new Mt(Q,3)),_e.setAttribute("phase",new Mt(vt,1));let Ue={value:0},ke=E(Gm.vertex,Gm.fragment,{time:a,pointScale:o,strength:Ue},{...g,fog:!1}),B=new Rn(_e,ke);B.frustumCulled=!1,B.renderOrder=-7,i.add(B),t.push(_e),n.push(ke);let Z=E(Vm.vertex,Vm.fragment,{color:{value:new Le(11128063)},strength:{value:0}},{...g,side:Kt,fog:!1}),pe=new ws(11,.7,280,20,1,!0);pe.translate(0,140,0),t.push(pe),n.push(Z);let Te=[[-72,-168,0],[64,-186,2.1],[4,-232,4.2]].map(([Fe,Je,F])=>{let q=new ht(pe,Z);return q.position.set(Fe,-3,Je),q.rotation.order="YXZ",q.userData.offset=F,q.frustumCulled=!1,i.add(q),q}),ve=96,we=new Float32Array(ve*3),ue=new Float32Array(ve);for(let Fe=0;Fe<ve;Fe++)ue[Fe]=v()*Math.PI*2;let Ae=new At;Ae.setAttribute("position",new Pt(we,3)),Ae.setAttribute("phase",new Pt(ue,1)),Ae.setDrawRange(0,0);let ze=E(Wm.vertex,Wm.fragment,{time:a,pointScale:o},{...g,fog:!1}),Y=new Rn(Ae,ze);Y.frustumCulled=!1,i.add(Y),t.push(Ae),n.push(ze);let Ce=zm(a),Re=Ce.pass,Ve=!1,He="high",De=0,Ye=1,et=()=>{te.visible=Oe.visible=He!=="low"&&Ye>.01};return{post:Re,sea:z,setLighting(Fe,Je,F){x.copy(F).normalize(),l.value=Fe,c.value=Math.min(1,Je)*(1-Fe),M.set(330010).lerp(new Le(3108776),Fe).lerp(new Le(1909062),c.value*.8),m.set(1323596).lerp(new Le(11126484),Fe).lerp(new Le(12609598),c.value*.85),f.set(Je>.1?16747077:8011050),oe.uniforms.color.value.copy(BM).lerp(HM,c.value),Ee.uniforms.color.value.copy(oe.uniforms.color.value),p.set(4860440).lerp(new Le(7027234),c.value),b.copy(s.fog.color),N.visible=Fe<.35,z.material.uniforms.sky.value.copy(m).lerp(M,c.value*.65).multiplyScalar(.7),z.material.uniforms.deep.value.set(398368).lerp(new Le(669270),Fe),z.material.uniforms.shallow.value.set(930640).lerp(new Le(1929112),Fe),X.material.uniforms.strength.value=.55*(1-Fe*.85),ee.value=.82-.22*(1-Fe)-.2*c.value,Ue.value=Math.max(0,1-Fe*1.6)*(1-c.value*.5),z.material.uniforms.sunColor.value.set(14676223).lerp(new Le(16752736),c.value),Ye=1-It.smoothstep(Fe,.12,.55),et()},setWeather(Fe){h.value=Fe==="rain"?.95:Fe==="fog"?.7:.25},setFlash(Fe){d.value=Math.max(0,Math.min(1,Fe))},setCinematic(Fe){Ce.setCinematic(Fe)},setAviation(Fe){let Je=Math.min(ve,Fe.length);for(let F=0;F<Je;F++)we.set([Fe[F].x,Fe[F].y,Fe[F].z],F*3);Ae.attributes.position.needsUpdate=!0,Ae.setDrawRange(0,Je)},setTier(Fe){He=Fe;let Je=He!=="low";X.visible=Je,ge.visible=Je,et(),Te.forEach(F=>{F.visible=Je}),u.value=He==="low"?3:He==="medium"?4:5,Re.enabled=He==="high"||He==="ultra"},setBusy(Fe){Ve=!!Fe},setPointScale(Fe){o.value=Fe},update(Fe,Je,F,q){let $=q?Math.min(.1,Math.max(0,Fe)):0;a.value+=$,s.fog&&(z.material.uniforms.fogColor.value.copy(s.fog.color),s.fog.isFogExp2&&(z.material.uniforms.fogDensity.value=s.fog.density)),z.position.x=F.position.x,z.position.z=F.position.z;let de=Ve?1.15:.32,Me=Ze.userData.rate??de;Ze.userData.rate=Me+(de-Me)*Math.min(1,$*1.5),me.rotation.y+=$*Ze.userData.rate,G.uniforms.strength.value=Ve?.85:.45,ne.scale.setScalar(1+Math.sin(a.value*(Ve?6:2.2))*.18);let he=Ye*(1+.035*Math.sin(a.value*1.6)+.015*Math.sin(a.value*4.1+1.2));oe.uniforms.strength.value=.3*he,Ee.uniforms.strength.value=.6*he,De+=$;let Pe=1-l.value;Z.uniforms.strength.value=Pe*Pe*.075*(1-c.value*.7)*(1-h.value*.45),Te.forEach((le,qe)=>{let Xe=le.userData.offset;le.rotation.y=Math.PI+Math.sin(De*(.11+qe*.025)+Xe)*.95,le.rotation.x=.22+.14*Math.sin(De*.13+Xe*1.7)}),Re.enabled&&Ce.update($,F,x,{day:l.value,dusk:c.value,cloud:h.value,animated:q})},stats:()=>({tier:He,busy:Ve,stars:w,dust:j,lamps:K.length,lampLevel:+Ye.toFixed(2),post:Re.enabled,coast:at.length/6,shoreLights:vt.length,clouds:+h.value.toFixed(2),searchlights:Te.length,aviation:Ae.drawRange.count,cinema:Ce.stats()}),dispose(){i.removeFromParent(),te.dispose(),Oe.dispose(),t.forEach(Fe=>Fe.dispose()),n.forEach(Fe=>Fe.dispose()),Ce.dispose()}}}var No=3,KM=3,YM=192,Xm=[9365742,7730385,16763274,12166399];function jm(s,e={}){let t=e.anchors||[],n=new Et;n.name="city-flow",s.add(n);let i=[];for(let _=0;_<t.length;_++)for(let w of[1,3]){let T=t[_],y=t[(_+w)%t.length];!T||!y||T.id===y.id||i.push({from:T,to:y,lift:22+(_+w)%4*11})}if(i.length<2)return n.removeFromParent(),{group:n,update(){},setTier(){},stats:()=>({routes:0,points:0,visible:!1}),dispose(){}};let r=Math.min(YM/No,i.length*KM),a=r*No,o=new Float32Array(a*3),l=new Float32Array(a),c=new Float32Array(a),h=new Float32Array(a*3),d=new At;d.setAttribute("position",new Pt(o,3)),d.setAttribute("alpha",new Pt(l,1)),d.setAttribute("size",new Pt(c,1)),d.setAttribute("tint",new Pt(h,3));let u=new Rt({transparent:!0,depthWrite:!1,blending:rn,fog:!1,uniforms:{pointScale:{value:900},strength:{value:.4}},vertexShader:`attribute float alpha,size;attribute vec3 tint;uniform float pointScale;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=clamp(size*pointScale/max(1.0,-mv.z),1.0,12.0);vAlpha=alpha;vTint=tint;}`,fragmentShader:`uniform float strength;varying float vAlpha;varying vec3 vTint;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;
gl_FragColor=vec4(vTint*pow(1.0-d,2.4)*vAlpha*strength,1.0);}`}),p=new Rn(d,u);p.frustumCulled=!1,p.renderOrder=2,n.add(p);let x=[],M=new Le;for(let _=0;_<r;_++){let w=i[_%i.length];M.setHex(Xm[_%Xm.length]),x.push({route:w,t:_*.3819%1,speed:.045+_*7%9/190,tint:[M.r,M.g,M.b],size:.42+_*3%5*.08})}function m(_,w,T){let y=_.from,C=_.to,S=1-w,D=(y.x+C.x)/2,v=(y.z+C.z)/2,I=Math.max(y.height,C.height)+_.lift,P=y.height+4,N=C.height+4;return T.x=S*S*y.x+2*S*w*D+w*w*C.x,T.y=S*S*P+2*S*w*I+w*w*N,T.z=S*S*y.z+2*S*w*v+w*w*C.z,T}let f={x:0,y:0,z:0},b="high",E=.4;function g(_,w,T){if(!p.visible)return;let y=w?Math.min(_,.05):0;for(let C=0;C<x.length;C++){let S=x[C];S.t=(S.t+y*S.speed)%1;for(let D=0;D<No;D++){let v=C*No+D,I=(S.t-D*.012+1)%1;m(S.route,I,f),o[v*3]=f.x,o[v*3+1]=f.y,o[v*3+2]=f.z;let P=Math.min(1,Math.min(I,1-I)*7);l[v]=P*(1-D/No)*.9,c[v]=S.size*(1-D*.22),h[v*3]=S.tint[0],h[v*3+1]=S.tint[1],h[v*3+2]=S.tint[2]}}d.attributes.position.needsUpdate=!0,d.attributes.alpha.needsUpdate=!0,d.attributes.size.needsUpdate=!0,d.attributes.tint.needsUpdate=!0,E+=(.18+T*.82-E)*Math.min(1,Math.max(0,_)*2),u.uniforms.strength.value=E}return{group:n,update:g,setPointScale(_){u.uniforms.pointScale.value=_},setTier(_){b=_,p.visible=b!=="low",n.visible=b!=="low"},stats:()=>({routes:i.length,points:a,visible:p.visible,tier:b}),dispose(){n.removeFromParent(),d.dispose(),u.dispose()}}}var Km=[{points:[[-62,34,-62],[8,44,-74],[62,38,-22],[56,47,42],[-8,41,62],[-64,36,12]],speed:9.5},{points:[[30,56,-12],[0,62,18],[-30,58,-12],[0,66,-42]],speed:8},{points:[[42,29,-56],[-28,33,-42],[-52,27,20],[18,31,52],[60,33,8]],speed:11}];function Ym(s,e={}){let t=new Et;t.name="city-drones",s.add(t);let n=[],i=[],r=[],a=new U,o=new U,l=new U,c=new Ui(.16,8,6);n.push(c);let h=g=>{let _=new jt({color:g,toneMapped:!1});return i.push(_),_},d=h(16730684),u=h(5046154),p=h(16777215);[...Km,...Km].forEach((g,_)=>{let w=new Wr(g.points.map(S=>new U(S[0],S[1]+(_>=3?8:0),S[2])),!0,"centripetal",.6),T=new Et;T.name="city-drone-"+(_+1),T.visible=!1;let y=new Et;T.add(y);let C=[new ht(c,d),new ht(c,u),new ht(c,p)];C[0].position.set(-2.6,.8,0),C[1].position.set(2.6,.8,0),C[2].position.set(0,2.1,-.4),T.add(...C),t.add(T),r.push({root:T,body:y,lights:C,curve:w,length:w.getLength(),speed:g.speed,t:_*.37%1,rotors:[],roll:0,dir:1,waiting:0})});let x=!0,M="high",m=!1,f=new Map,b=g=>{if(!f.has(g)){let _=g.clone();e.surfaces?.apply(_,"object",g.name),f.set(g,_)}return f.get(g)};function E(g,_){let{curve:w,root:T}=g,y=g.t;if(g.t=(g.t+_*g.speed*g.dir/g.length+1)%1,w.getPointAt(g.t,T.position),w.getTangentAt(g.t,a),a.multiplyScalar(g.dir),g.collider&&e.traffic){let D=g.collider,v=e.traffic;w.getPointAt((g.t+g.dir*20/g.length+1)%1,l),g.avoidTime=Math.max(0,(g.avoidTime||0)-_);let I=Math.max(T.position.y,v.ceiling(D,T.position.x,T.position.z),v.ceiling(D,l.x,l.z),g.avoidTime?g.avoidHeight:0);T.position.y=D.y+It.clamp(I-D.y,-_*5,_*7);let P=Math.atan2(a.x,a.z),N=D.heading+Math.atan2(Math.sin(P-D.heading),Math.cos(P-D.heading))*Math.min(1,_*3),z={...T.position,heading:N};if(g.obstruction=v.obstruction(D,D,z),g.obstruction){let X=v.body(g.obstruction),j=g.obstruction.startsWith("drone-")&&(Math.abs(D.y-X.y)>1?D.y<X.y:D.id<X.id);j||(g.avoidHeight=Math.min(170,D.y+8),g.avoidTime=8),z={...D,y:j?D.y:Math.min(170,D.y+_*5)},g.t=y}v.propose(D,z,(X,j)=>{X?g.waiting=0:(g.t=y,g.waiting+=_,g.waiting>2.5&&(g.dir=-g.dir,g.waiting=0)),T.position.set(j.x,j.y,j.z),T.rotation.set(0,j.heading,0)})}w.getTangentAt((g.t+.015)%1,o),g.collider||T.rotation.set(0,Math.atan2(a.x,a.z),0);let C=Math.atan2(o.x,o.z)-Math.atan2(a.x,a.z),S=Math.atan2(Math.sin(C),Math.cos(C));g.roll+=(It.clamp(S*12,-.55,.55)-g.roll)*(_>0?Math.min(1,_*3):1),g.body.rotation.z=g.roll}return{setTier(g){M=g},setTemplate(g){if(!g)return;m=!0;let _=f;f=new Map;for(let[w,T]of r.entries()){T.body.clear(),T.rotors.length=0;let y=g.clone(!0);y.scale.setScalar(1.9),y.updateMatrixWorld(!0);let C=new un().setFromObject(y);if(C.expandByScalar(.5),C.min.y-=1,C.max.y+=1,y.traverse(S=>{S.isMesh&&(S.castShadow=!1,S.receiveShadow=!1,S.material=Array.isArray(S.material)?S.material.map(b):b(S.material)),/^rotor_/.test(S.name)&&T.rotors.push(S)}),T.body.add(y),T.root.visible=w<(M==="low"?2:M==="medium"?3:6),e.traffic&&!T.collider){E(T,0);let S=cs({min:C.min.toArray(),max:C.max.toArray()});T.root.position.y=Math.max(T.root.position.y,e.traffic.ceiling(S,T.root.position.x,T.root.position.z)),T.collider=e.traffic.register("drone-"+w,S,{...T.root.position,heading:T.root.rotation.y},0),T.collider||(T.root.visible=!1)}}_.forEach(w=>w.dispose())},update(g,_,w){x=w;let T=w?Math.min(.1,Math.max(0,g)):0;r.forEach((y,C)=>{if(y.root.visible=m&&C<(M==="low"?2:M==="medium"?3:6)&&(!e.traffic||!!y.collider),y.collider&&(y.root.visible&&!y.collider.enabled&&!e.traffic.relocate(y.collider,y.collider)&&(y.root.visible=!1),y.collider.enabled=y.root.visible),!y.root.visible)return;E(y,T),y.rotors.forEach((D,v)=>{D.rotation.y+=T*(v%2?-46:46)});let S=Math.sin(_*5+C*1.7);y.lights[0].visible=y.lights[1].visible=!w||S>-.2,y.lights[2].visible=!w||S>.93,y.lights[2].scale.setScalar(w?1.6:1)})},stats:()=>({drones:r.filter(g=>g.root.visible).length,animated:x,positions:r.map(g=>g.root.position.toArray().map(_=>Math.round(_))),obstructions:r.map(g=>g.obstruction)}),dispose(){t.removeFromParent(),n.forEach(g=>g.dispose()),i.forEach(g=>g.dispose()),f.forEach(g=>g.dispose()),r.forEach(g=>{g.collider&&e.traffic.remove(g.collider.id)}),r.length=0}}}var us=[{id:"repair-bay",district:"infra",x:-26,z:-62,role:"technician"},{id:"parcel-sorter",district:"missions",x:27.6,z:49.4,role:"courier"},{id:"relay-mast",district:"integrations",x:57.5,z:-23,role:"archivist"},{id:"kinetic-fountain",district:"graph",x:-26,z:47,role:"archivist"},{id:"glass-garden",district:"graph",x:40,z:-68,role:"technician"},{id:"meeting-charge",district:"operations",x:8,z:46,role:"courier"}],Jm=[{id:"data-monument",x:0,z:24,y:0,angle:0,clip:"spin"},{id:"neon-gate",x:-18,z:0,y:bt.pavement,angle:0,clip:"pulse"},{id:"sky-pod",x:56,z:47,y:0,angle:0,clip:"scan"},{id:"tidal-turbine",x:0,z:96,y:-3.2,angle:Math.PI,clip:"spin"}],JM=2*Wt.halfWidth;function ZM(s){return fm().some(e=>e.axis==="x"?Math.abs(s.x-e.at)<=qt.opening&&Math.abs(s.z-e.line)<=Wt.halfWidth:Math.abs(s.z-e.at)<=qt.opening&&Math.abs(s.x-e.line)<=Wt.halfWidth)}function Zm(s,e,t=null){let n=xm(),i=Pm(t),r=n.nodes.length,a=[...n.nodes,...i.nodes].map(m=>({x:m.x,z:m.z,y:e(m.x,m.z),heading:0,lobby:m.lobby||null,kind:m.kind,cell:m.kind==="grid"||void 0})),o=new Map,l=-1,c=null;function h(){if(c)return c;let m=(f,b)=>Math.hypot(a[f].x-a[b].x,a[f].z-a[b].z);c=n.links.filter(f=>Xi(a[f.a],a[f.b],e)).map(f=>({a:f.a,b:f.b,length:f.length}));for(let[f,b]of i.links)Xi(a[r+f],a[r+b],e)&&c.push({a:r+f,b:r+b,length:m(r+f,r+b)});return i.nodes.forEach((f,b)=>{if(f.kind!=="foot")return;let E=r+b;for(let g=0;g<r;g++){let _=m(E,g);_<rh&&hs(a[E],a[g])&&Xi(a[E],a[g],e)&&c.push({a:E,b:g,length:_})}}),c}let d=2,u=22;function p(m,f,b,E){let g=[];E.forEach((w,T)=>{T<r&&g.push(w)});let _=[];for(let w of us){let T=new Map;for(let v=-5;v<=5;v++)for(let I=-5;I<=8;I++){let P=w.x+v*d,N=w.z+I*d,z={x:P,z:N,y:e(P,N),heading:0,lobby:null,cell:!0};Fs(P,N)!==null||Ns(P,N)||!hs(z,z)||!s.clear(m,z,z,!1)||T.set(v+","+I,{c:z,i:v,j:I,links:[],exit:!1})}let y=(v,I)=>{let P=Math.hypot(v.c.x-I.c.x,v.c.z-I.c.z);hs(v.c,I.c)&&Xi(v.c,I.c,e)&&s.clear(m,v.c,I.c,!1)&&(v.links.push([I,P]),I.links.push([v,P]))};for(let v of T.values())for(let[I,P]of[[1,0],[0,1],[1,1],[1,-1]]){let N=T.get(v.i+I+","+(v.j+P));N&&y(v,N)}let C=[];for(let v of T.values())for(let I of g){let P=Math.hypot(v.c.x-f[I].x,v.c.z-f[I].z);P<u&&hs(v.c,f[I])&&Xi(v.c,f[I],e)&&s.clear(m,v.c,f[I],!1)&&(v.exit=!0,C.push([v,I,P]))}let S=new Set,D=[...new Set(C.map(v=>v[0]))];for(D.forEach(v=>S.add(v));D.length;){let v=D.pop();for(let[I]of v.links)S.has(I)||(S.add(I),D.push(I))}for(let v of S)v.place=w.id,_.push(v),v.index=f.length,f.push(v.c),b.push([]);for(let v of S)for(let[I,P]of v.links)v.index<I.index&&(b[v.index].push([I.index,P]),b[I.index].push([v.index,P]));for(let[v,I,P]of C)b[v.index].push([I,P]),b[I].push([v.index,P])}for(let w of _)for(let T of _){let y=Math.hypot(w.c.x-T.c.x,w.c.z-T.c.z);w.place<T.place&&y<3&&hs(w.c,T.c)&&Xi(w.c,T.c,e)&&s.clear(m,w.c,T.c,!1)&&(b[w.index].push([T.index,y]),b[T.index].push([w.index,y]))}}function x(m,f){let b=new Set,E=[];for(m.forEach((g,_)=>{g.kind==="entry"&&(b.add(_),E.push(_))});E.length;){let g=E.pop();for(let[_]of f[g])b.has(_)||(b.add(_),E.push(_))}m.forEach((g,_)=>{if(g.lobby&&!b.has(_)){for(let[w]of f[_])f[w]=f[w].filter(T=>T[0]!==_);f[_]=[]}})}function M(m,f){l!==s.revision()&&(o.clear(),l=s.revision());let b=m.circles.map(j=>[j.x,j.z,j.r].join(",")).join(";")+":"+m.maxY;if(!o.has(b)){let j=new Map,V=[];a.forEach((ie,H)=>{s.clear(m,ie,ie,!1)&&(j.set(H,V.length),V.push(ie))});let W=V.map(()=>[]);for(let ie of h()){let H=j.get(ie.a),ge=j.get(ie.b);H===void 0||ge===void 0||s.clear(m,V[H],V[ge],!1)&&(W[H].push([ge,ie.length]),W[ge].push([H,ie.length]))}p(m,V,W,j),x(V,W),o.set(b,{nodes:V,edges:W})}let E=o.get(b),g=Ns(m.x,m.z)?.id||null,_=Ns(f.x,f.z)?.id||null,w=[...E.nodes,{x:m.x,y:m.y,z:m.z,heading:m.heading,lobby:g},{...f,heading:0,lobby:_}],T=w.length-2,y=w.length-1,C=ZM(w[T]),S=new Map,D=(j,V,W)=>{for(let[ie,H]of[[j,V],[V,j]]){let ge=S.get(ie);ge?ge.push([H,W]):S.set(ie,[[H,W]])}};for(let j of[T,y])for(let V=0;V<j;V++){let W=w[j],ie=w[V],H=Math.hypot(W.x-ie.x,W.z-ie.z);if(W.lobby!==ie.lobby||ie.cell&&H>=(ie.lobby?4:10))continue;let ge=j===T,G=V===T?hs(ie,W,!1,!0):hs(W,ie,ge&&C&&H<JM,ge);(H<rh||V===T)&&G&&Xi(W,ie,e)&&s.clear(m,W,ie,!1)&&D(j,V,H)}let v=w.map(()=>1/0),I=w.map(()=>-1),P=[[0,T]];v[T]=0;let N=j=>{let V=P.push(j)-1;for(;V>0;){let W=V-1>>1;if(P[W][0]<=j[0])break;P[V]=P[W],V=W}P[V]=j},z=()=>{let j=P[0],V=P.pop();if(P.length){let W=0;for(;;){let ie=2*W+1;if(ie>=P.length||(ie+1<P.length&&P[ie+1][0]<P[ie][0]&&ie++,P[ie][0]>=V[0]))break;P[W]=P[ie],W=ie}P[W]=V}return j};for(;P.length;){let[j,V]=z();if(j>v[V])continue;if(V===y)break;let W=([ie,H])=>{j+H<v[ie]&&(v[ie]=j+H,I[ie]=V,N([v[ie],ie]))};V<E.edges.length&&E.edges[V].forEach(W),S.get(V)?.forEach(W)}if(!Number.isFinite(v[y]))return[];let X=[];for(let j=y;j!==T;j=I[j]){if(j<0)return[];X.unshift(w[j])}return X}return{route:M,points:a,streetPoints:a.filter(m=>!m.lobby&&!["foot","landing","porch"].includes(m.kind))}}function $m(s){let e=new Et;e.name="city-conversations",s.add(e);let t=new Qn(1,.025,5,40,Math.PI*1.45),n=new Ss(.92,1,48),i=new U(0,0,1),r=new U,a=[],o=0;for(let l=0;l<8;l++){let c=[];for(let h=0;h<4;h++){let d=new jt({color:8445392,transparent:!0,opacity:0,blending:rn,depthWrite:!1,side:Kt,toneMapped:!1}),u=new ht(h===3?n:t,d);u.visible=!1,e.add(u),c.push(u)}a.push({meshes:c,age:10,from:null,to:null})}return{send(l,c,h="ambient"){let d=a.find(u=>u.age>=2.4);return d?(Object.assign(d,{from:l,to:c,age:0,source:h}),o++,d.meshes.forEach(u=>u.material.color.setHex(h==="live"?16105851:8445392)),!0):!1},update(l,c=!0){for(let h of a){if((!c||h.from?.enabled===!1||h.to?.enabled===!1)&&(h.age=10),h.age+=l,h.meshes.forEach(m=>{m.visible=!1}),h.age>=2.4)continue;let d=h.from,u=h.to,p=u.x-d.x,x=u.z-d.z;r.set(p,u.y+Math.min(2,u.maxY*.7)-(d.y+Math.min(2,d.maxY*.7)),x).normalize();for(let m=0;m<3;m++){let f=(h.age-m*.18)/1.55;if(f<0||f>1)continue;let b=h.meshes[m];b.visible=!0,b.position.set(d.x+p*f,It.lerp(d.y+Math.min(2,d.maxY*.7),u.y+Math.min(2,u.maxY*.7),f)+Math.sin(f*Math.PI)*.65,d.z+x*f),b.quaternion.setFromUnitVectors(i,r),b.rotateZ(m*.65),b.scale.setScalar(.25+Math.sin(f*Math.PI)*.65),b.material.opacity=Math.sin(Math.PI*f)*.75}let M=(h.age-1.55)/.85;if(M>0){let m=h.meshes[3];m.visible=!0,m.position.set(u.x,u.y+Math.min(2,u.maxY*.7),u.z),m.quaternion.setFromUnitVectors(i,r),m.scale.setScalar(1.25*(1-M)+.12),m.material.opacity=Math.sin(M*Math.PI)*.85}}},clear(){a.forEach(l=>{l.age=10,l.meshes.forEach(c=>{c.visible=!1})})},stats:()=>({sent:o,active:a.filter(l=>l.age<2.4).length,sources:a.filter(l=>l.age<2.4).map(l=>l.source)}),dispose(){e.removeFromParent(),t.dispose(),n.dispose(),a.forEach(l=>l.meshes.forEach(c=>c.material.dispose()))}}}var $M=[["leg_l","shin_l","foot_l"],["leg_r","shin_r","foot_r"]],Qm=[[0,-.13,-.085],[0,-.13,.27]];function zs(s,e,t,n=14,i=.6){if(!(t>0)||!Number.isFinite(s.y)||Math.abs(e-s.y)>i)return s.y=e,s.v=0,e;let r=s.y-e,a=Math.exp(-n*t),o=(s.v+n*r)*t;return s.v=(s.v-n*o)*a,s.y=e+(r+o)*a,s.y}function QM(s,e=.025,t=.07){let n=Math.min(1,Math.max(0,(s-e)/(t-e)));return 1-n*n*(3-2*n)}var Uo=new U,zo=new U,Cd=new U,ew=new U,Pd=new U,Id=new U,dh={hip:new nn,knee:new nn};function tw(s,e,t,n,i){Uo.subVectors(e,s),zo.subVectors(t,e);let r=Uo.length(),a=zo.length();if(r<1e-6||a<1e-6||i.lengthSq()<1e-12)return null;Cd.copy(i).normalize();let o=Math.min(r+a-1e-4,Math.max(Math.abs(r-a)+1e-4,n.distanceTo(s))),l=Math.atan2(ew.crossVectors(Uo,zo).dot(Cd),Uo.dot(zo)),c=Math.acos(Math.max(-1,Math.min(1,(r*r+a*a-o*o)/(2*r*a)))),h=(Math.PI-c)*(l<0?-1:1);return dh.knee.setFromAxisAngle(Cd,h-l),Pd.copy(zo).applyQuaternion(dh.knee).add(Uo),Id.subVectors(n,s),Pd.lengthSq()<1e-12||Id.lengthSq()<1e-12?null:(dh.hip.setFromUnitVectors(Pd.normalize(),Id.normalize()),dh)}var Oo=new U,ko=new U,eg=new U,tg=new U,Dd=new U,Ld=new U,ng=new U,Fd=new nn,ig=new nn,sg=new nn;function ag(s){Oo.fromArray(Qm[0]).applyMatrix4(s.matrixWorld),ko.fromArray(Qm[1]).applyMatrix4(s.matrixWorld)}function og(s,e){return ag(s),Math.min(Oo.y,ko.y)-e.matrixWorld.elements[13]}function nw(s,e){return ag(s),Math.max(e(Oo.x,Oo.z)-Oo.y,e(ko.x,ko.z)-ko.y)}function rg(s,e){s.parent.getWorldQuaternion(Fd),s.quaternion.premultiply(Fd).premultiply(e).premultiply(Fd.invert())}function lg(s){let e=s.getObjectByName("structure");if(!e)return null;let t=[];for(let n of $M){let[i,r,a]=n.map(o=>s.getObjectByName(o));if(!i||!r||!a||r.parent!==i||a.parent!==r)return null;t.push({leg:i,shin:r,foot:a})}s.updateMatrixWorld(!0);for(let n of t)if(Math.abs(og(n.foot,s))>.04)return null;return{structure:e,legs:t}}function cg(s,e,t,{maxDrop:n=.25,maxRaise:i=.35}={}){e.updateMatrixWorld(!0);let r=s.legs.map(l=>({l,lift:og(l.foot,e),pen:nw(l.foot,t)})),a=0;for(let l of r)a=Math.max(a,QM(l.lift)*Math.max(0,-l.pen));a=Math.min(a,n),a<1e-4&&(a=0),a&&(e.position.y-=a,e.updateMatrixWorld(!0));let o=[];for(let l of r){let c=Math.min(i,l.pen+a);if(o.push(Math.max(0,c)),c<=5e-4)continue;let{leg:h,shin:d,foot:u}=l.l;h.getWorldPosition(eg),d.getWorldPosition(tg),u.getWorldPosition(Dd),Ld.copy(Dd),Ld.y+=c,ng.setFromMatrixColumn(h.matrixWorld,0);let p=tw(eg,tg,Dd,Ld,ng);p&&(u.getWorldQuaternion(sg),rg(d,p.knee),rg(h,p.hip),h.updateMatrixWorld(!0),d.getWorldQuaternion(ig),u.quaternion.copy(ig.invert().multiply(sg)),u.updateMatrixWorld(!0))}return{drop:a,raised:o}}var iw=.35,sw=s=>{let e=[0,2].map(t=>Math.min(iw/1.15,(s.max[t]-s.min[t])/2));return{min:[-e[0],s.min[1],-e[1]],max:[e[0],s.max[1],e[1]]}};function rw(s){let e=cs(sw(s),1.15);return e.circles=[{x:0,z:0,r:e.reach}],e}var hg=(s,e,t)=>s+Math.atan2(Math.sin(e-s),Math.cos(e-s))*Math.min(1,t*5),Nd=(s,e,t)=>{let n=s.play?.(e);n&&(n.timeScale=t/1.65)};function ug(s,{traffic:e,camera:t,floor:n,catalog:i,onSelect:r,onDemonstrate:a,active:o,onGait:l,shadows:c}){let h=[],d=new Map,u=[],p=[],x=Zm(e,n,i),M=$m(s),m=0,f=null,b=!1,E="high",g=!1,_=0,w="",T=0,y={},C={},S=new Map;function D(G){let ye=S.get(G.id);return!ye||Math.hypot(G.x-ye.x,G.z-ye.z)>.05||Math.abs(G.heading-ye.heading)>.01?(S.set(G.id,{x:G.x,z:G.z,heading:G.heading,since:m}),!1):m-ye.since>8}let v=()=>E==="low"?3:E==="medium"?10:19;function I(G){G.slot&&(G.slot.owner=null,G.slot=null),G.path=[],G.goal=null}function P(G){return G?{id:G.id,role:G.role,state:G.state,source:G.source||"ambient",guided:!!G.guide}:null}function N(G){f=G?.id||null,w=G?.state||"",r?.(P(G))}function z(G,ye,J){let A="resident-"+ye,me=rw(J),ce=["courier","technician","archivist"][ye%3],ne=null,K=u.filter(te=>te.role===ce).map(te=>({x:te.x,z:te.z+2.2,y:n(te.x,te.z+2.2),heading:Math.PI}));K.push(...K.splice(0,Math.floor(ye/3)*2%Math.max(1,K.length)));let oe=[...K,...x.points.map((te,fe)=>x.points[(ye*7+fe)%x.points.length])];for(let te of oe)if(Fs(te.x,te.z)===null&&(ne=e.register(A,me,te,1),ne))break;if(!ne)return G.node.visible=!1,null;let re={...G,id:A,index:ye,body:ne,role:ce,state:"idle",path:[],until:ye*.3,trip:ye,cooldown:8+ye*.7,source:"ambient",carrying:!1,parcels:[],soundX:ne.x,soundZ:ne.z};return G.node.traverse(te=>{te.isMesh&&/parcel/.test(te.name)&&(re.parcels.push(te),te.visible=!1)}),h.push(re),G.node.position.set(ne.x,ne.y,ne.z),re.height={y:ne.y,v:0},re.pitch={y:0,v:0},G.node.rotation.order="YXZ",re.rig=lg(G.node),re.shadow=c?.claim()||null,re}function X(G,ye,J,A){let me={id:G,body:ye,node:J,pause:A,role:"patrol",state:"patrol",cooldown:7+h.length,path:[],soundX:ye.x,soundZ:ye.z};return h.push(me),me}function j(G,ye){let J=ye.find(K=>Math.hypot(K.x-G.body.x,K.z-G.body.z)>.5);if(!J)return!0;let A=J.x-G.body.x,me=J.z-G.body.z,ce=Math.hypot(A,me),ne=Math.min(1.8,ce);return e.clear(G.body,G.body,{x:G.body.x+A/ce*ne,y:G.body.y,z:G.body.z+me/ce*ne,heading:G.body.heading})}function V(G){I(G),G.source="ambient",G.state="idle",G.until=m+2;let ye=u.filter(me=>!me.owner&&!(me.lobby&&E==="low")&&(G.role==="courier"?me.place===(G.carrying?"meeting-charge":"parcel-sorter"):me.role===G.role||me.place==="meeting-charge")),J=G.trip===G.index;J&&ye.sort((me,ce)=>Math.hypot(me.x-G.body.x,me.z-G.body.z)-Math.hypot(ce.x-G.body.x,ce.z-G.body.z));let A=()=>{for(let me=0;me<24;me++){let ce=x.streetPoints[(G.trip++*13+G.index)%x.streetPoints.length];if(Math.hypot(ce.x-G.body.x,ce.z-G.body.z)>30)continue;let ne=x.route(G.body,ce);if(ne.length&&j(G,ne))return G.path=ne,G.state="walk",!0}return!1};for(let me of[0,1]){if(me===1&&G.avoidLeft>0&&A()){G.avoidLeft--;return}for(let ce=0;ce<ye.length;ce++){let ne=ye[((J?0:G.trip)+ce)%ye.length];if(me===0&&G.avoidLeft>0&&ne.place===G.avoid)continue;let K=x.route(G.body,ne);if(!(!K.length||!j(G,K))){G.avoidLeft>0&&G.avoidLeft--,ne.owner=G.id,G.slot=ne,G.path=K,G.goal=ne,G.trip++,G.state=G.carrying?"carry":"walk";return}}}A()}function W(G,ye,J){let A=e.obstruction(G.body,G.body,J),me=null;for(let fe of h){if(fe===G||fe.index===void 0||fe.index>=G.index||!fe.body.enabled)continue;let Ee=fe.body.x-G.body.x,Ne=fe.body.z-G.body.z;if(fe.id===A||Ee*Ee+Ne*Ne<2.56&&Ee*ye.x+Ne*ye.z>0){me=fe;break}}if(!me)return!1;let ce=me.body.x-G.body.x,ne=me.body.z-G.body.z,K=Ds(G.body.x,G.body.z),oe=G.body.x,re=G.body.z,te=1/0;for(let[fe,Ee]of[[ye.z,-ye.x],[-ye.z,ye.x]]){let Ne={x:G.body.x+fe*.8,y:G.body.y,z:G.body.z+Ee*.8,heading:G.body.heading},Oe=fe*ce+Ee*ne;Oe>=te||!e.clear(G.body,G.body,Ne)||!K&&Ds(Ne.x,Ne.z)||(te=Oe,oe=Ne.x,re=Ne.z)}if(te===1/0){let fe={x:G.body.x-ye.x*.8,y:G.body.y,z:G.body.z-ye.z*.8,heading:G.body.heading};e.clear(G.body,G.body,fe)&&(K||!Ds(fe.x,fe.z))&&(oe=fe.x,re=fe.z)}return G.back={x:oe,z:re,until:m+1.5+G.index%5*.25},G.wait=0,G.state="idle",!0}function ie(G){for(let ye of[G.a,G.b])I(ye),ye.meeting=null,ye.cooldown=m+12+(ye.index||0),ye.pause?.(!1),ye.state=ye.pause?"patrol":"idle",ye.until=m+1}function H(){p.forEach(ie),p.length=0,M.clear(),f=null,r?.(null);for(let G of h)G.guide=null,G.pause?.(!1,G.body),G.trafficYield=!1,G.yieldTram=null,G.pause||(I(G),G.state="idle",G.until=m+1)}function ge(G,ye){if(g)return;ye&&(m+=G);for(let A of h){let me=A.pause||A.index<v();if(me&&!A.body.enabled){if(!e.relocate(A.body,A.body)){A.node.visible=!1;continue}A.body.enabled=!0,A.node.position.set(A.body.x,A.body.y,A.body.z)}!me&&A.body.enabled&&(I(A),A.body.enabled=!1,A.meeting&&ie(A.meeting)),A.node.visible=!!me,A.soundX==null&&(A.soundX=A.body.x,A.soundZ=A.body.z);let ce=Math.hypot(A.body.x-A.soundX,A.body.z-A.soundZ);if(A.soundX=A.body.x,A.soundZ=A.body.z,l?.(A.id,A.body.x,A.body.y,A.body.z,ce>.004,!!A.carrying,A.role==="patrol"?"glide":""),!me||!ye)continue;let ne=e.neighbors(A.body,22).sort((K,oe)=>Math.hypot(K.x-A.body.x,K.z-A.body.z)-Math.hypot(oe.x-A.body.x,oe.z-A.body.z)).find(K=>{if(!K.id.startsWith("tram-")||D(K))return!1;let oe=A.body.x-K.x,re=A.body.z-K.z,te=Math.cos(K.heading),fe=Math.sin(K.heading);return Math.abs(oe*te-re*fe)<A.body.reach+(A.trafficYield?4:2.3)&&oe*fe+re*te>-8-A.body.reach&&oe*fe+re*te<14});if(!ne&&A.yieldTram){let K=e.body(A.yieldTram),oe=K?A.body.x-K.x:0,re=K?A.body.z-K.z:0;if(K?.enabled&&!D(K)&&Math.hypot(oe,re)<26&&oe*Math.sin(K.heading)+re*Math.cos(K.heading)>-K.reach-A.body.reach-1){A.pause?.(!0),A.play?.("idle");continue}A.yieldTram=null}if(ne){if(A.trafficYield=!0,A.yieldTram=ne.id,A.meeting){let Q=A.meeting;ie(Q),p.splice(p.indexOf(Q),1)}A.pause?.(!0);let K=Math.cos(ne.heading),oe=Math.sin(ne.heading),re=Math.sign((A.body.x-ne.x)*K-(A.body.z-ne.z)*oe)||1,te=(A.body.x-ne.x)*K-(A.body.z-ne.z)*oe,fe=Math.sign((A.body.x-ne.x)*oe+(A.body.z-ne.z)*K)||1,Ee=[[K*re,-oe*re,A.body.reach+4.5-Math.abs(te)],[-K*re,oe*re,A.body.reach+4.5+Math.abs(te)],[oe*fe,K*fe,1.2]],Ne=Array.from({length:16},(Q,vt)=>[Math.cos(vt*Math.PI/8),Math.sin(vt*Math.PI/8),2]),Oe=([Q,vt])=>Q*(A.body.x-ne.x)+vt*(A.body.z-ne.z);Ee.push(...Ne.filter(Q=>Oe(Q)>.2).sort((Q,vt)=>Oe(vt)-Oe(Q)));let ut=A.escape&&m<A.escape.until?A.escape:null,rt=ut?Ee.filter(Q=>Q[0]===ut.x&&Q[1]===ut.z).concat(Ee.filter(Q=>!(Q[0]===ut.x&&Q[1]===ut.z))):Ee,at=!1;for(let[Q,vt,je]of rt){let O={x:A.body.x+Q*je,y:A.body.y,z:A.body.z+vt*je,heading:A.body.heading};if(!e.clear(A.body,A.body,O))continue;let R={...O,x:A.body.x+Q*G*1.8,z:A.body.z+vt*G*1.8};R.y=A.pause?A.body.y:n(R.x,R.z),e.propose(A.body,R,(ee,ae)=>{A.node.position.set(ae.x,ae.y,ae.z),Nd(A,"walk",1.8)}),A.escape={x:Q,z:vt,until:m+.8},A.state="yield",A.cooldown=m+5,at=!0;break}at||(A.play?.("idle"),A.state="yield");continue}if(A.trafficYield&&(A.trafficYield=!1,A.pause?.(!1,A.body),A.state=A.pause?"patrol":A.path.length?"walk":"idle"),!A.meeting){if(A.pause){A.state==="greet"&&m>A.until&&(A.pause(!1),A.state="patrol");continue}if(A.guide&&Math.hypot(t.position.x-A.body.x,t.position.z-A.body.z)>9){A.play("idle");continue}if(A.back){let K=A.back,oe=K.x-A.body.x,re=K.z-A.body.z,te=Math.hypot(oe,re);if(m>=K.until)A.back=null;else{if(te>.03){let fe=Math.min(te,1.2*G),Ee=A.body.x+oe/te*fe,Ne=A.body.z+re/te*fe;e.propose(A.body,{x:Ee,y:n(Ee,Ne),z:Ne,heading:A.body.heading},(Oe,ut)=>{A.node.position.set(ut.x,ut.y,ut.z),Oe?Nd(A,"walk",1.2):A.play("idle")})}else A.play("idle");continue}}if(A.path.length){let K=A.path[0],oe=K.x-A.body.x,re=K.z-A.body.z,te=Math.hypot(oe,re),fe=A.guide?1.9:1.65;if(te<.18){A.path.shift();continue}let Ee=Math.min(te,fe*G),Ne={x:oe/te,z:re/te},Oe=A.body.x+Ne.x*Ee,ut=A.body.z+Ne.z*Ee,rt=hg(A.body.heading,Math.atan2(oe,re),G),at=Math.min(1.8,te),Q={x:A.body.x+Ne.x*at,y:n(Oe,ut),z:A.body.z+Ne.z*at,heading:rt};if(e.clear(A.body,A.body,Q))A.detour=null;else{let je=[[Ne.z,-Ne.x],[-Ne.z,Ne.x]],O=A.detour&&m<A.detour.until?je.findIndex(ae=>ae[0]===A.detour.x&&ae[1]===A.detour.z):-1,R=O>=0?[je[O],...je.filter((ae,_e)=>_e!==O)]:je,ee=!1;for(let[ae,_e]of R){let Ue={x:A.body.x+ae*1.3,y:A.body.y,z:A.body.z+_e*1.3,heading:A.body.heading};if(!e.clear(A.body,A.body,Ue)||Ds(Ue.x,Ue.z)&&!Ds(A.body.x,A.body.z))continue;let ke={x:Ue.x+Ne.x*1.8,y:n(Ue.x,Ue.z),z:Ue.z+Ne.z*1.8,heading:A.body.heading};if(e.clear(A.body,Ue,ke)){Oe=A.body.x+ae*Ee,ut=A.body.z+_e*Ee,A.detour={x:ae,z:_e,until:m+1.2},ee=!0;break}}ee||(Oe=A.body.x,ut=A.body.z)}e.clear(A.body,A.body,{x:Oe,y:n(Oe,ut),z:ut,heading:rt},!1)||(rt=A.body.heading);let vt=Math.hypot(Oe-A.body.x,ut-A.body.z)>.001;e.propose(A.body,{x:Oe,y:n(Oe,ut),z:ut,heading:rt},(je,O)=>{A.node.position.set(O.x,O.y,O.z),A.node.rotation.y=O.heading,!je||!vt?(A.wait=(A.wait||0)+G,A.play("idle"),A.state="yield"):(A.wait=0,A.state=A.guide?"guide":A.carrying?"carry":"walk",Nd(A,A.carrying?"carry":"walk",fe)),A.wait>1.2&&!A.back&&W(A,Ne,Q)?A.play("idle"):A.wait>1.2&&A.slot&&A.path.length===1&&te<2.2&&(A.path.length=0,A.wait=0),A.wait>2.5&&(A.wait=0,I(A),A.state=A.guide?"guide":"idle",A.until=m+.5,A.guide&&(A.path=x.route(A.body,A.guide)))})}else if(A.state==="walk"||A.state==="carry"||A.state==="guide"||A.state==="yield"){if(A.guide&&Math.hypot(A.body.x-A.guide.x,A.body.z-A.guide.z)>.5){A.play("idle"),m>A.until&&(A.path=x.route(A.body,A.guide),A.until=m+2.5);continue}A.guide=null,A.state=A.slot?.place==="meeting-charge"?"charge":A.slot?"work":"idle",A.until=m+5+A.index%4,A.play(A.state==="work"?"work":"idle"),A.slot&&!A.slot.lobby&&a?.(A.slot.place,5),A.slot?.lobby&&T++,A.avoid=A.slot?.place||null,A.avoidLeft=A.avoid?2:0,_++,C[A.id]=(C[A.id]||0)+1,A.slot&&(y[A.slot.place]=(y[A.slot.place]||0)+1)}else m>A.until&&(A.role==="courier"&&A.slot&&(A.state==="work"||A.state==="charge")&&(A.carrying=A.slot.place==="parcel-sorter",A.parcels.forEach(K=>{K.visible=A.carrying})),V(A))}}if(!ye){M.update(0,!1);return}for(let A of h){if(!A.body.enabled||A.trafficYield||A.meeting||A.guide||A.state==="greet"||m<A.cooldown||Ds(A.body.x,A.body.z))continue;let me=h.find(ne=>ne!==A&&ne.body.enabled&&!ne.trafficYield&&!Ds(ne.body.x,ne.body.z)&&ne.state!=="greet"&&!ne.meeting&&!ne.guide&&m>=ne.cooldown&&Math.abs(A.body.y-ne.body.y)<2&&Math.hypot(A.body.x-ne.body.x,A.body.z-ne.body.z)>2.8&&Math.hypot(A.body.x-ne.body.x,A.body.z-ne.body.z)<6);if(!me)continue;let ce={a:A,b:me,at:m,sent:!1,replied:!1};p.push(ce);for(let ne of[A,me])ne.meeting=ce,ne.state="exchange",ne.pause?.(!0),ne.play?.("greet")}for(let A=p.length-1;A>=0;A--){let me=p[A],ce=m-me.at;if(ce>4.8||!me.a.body.enabled||!me.b.body.enabled){ie(me),p.splice(A,1);continue}for(let[ne,K]of[[me.a,me.b],[me.b,me.a]])e.propose(ne.body,{...ne.body,heading:hg(ne.body.heading,Math.atan2(K.body.x-ne.body.x,K.body.z-ne.body.z),G)},(oe,re)=>{ne.node.rotation.y=re.heading});ce>.6&&!me.sent&&(M.send(me.a.body,me.b.body),me.sent=!0),ce>2.4&&!me.replied&&(M.send(me.b.body,me.a.body),me.replied=!0)}M.update(G,!0);let J=h.find(A=>A.id===f);J&&J.state!==w&&(w=J.state,r?.({...P(J),refresh:!0}))}function Ze(G){if(g)return;let ye=E!=="low";for(let J of h){if(!J.height)continue;if(!J.body.enabled||!J.node.visible){J.shadow?.hide();continue}let A=J.body,me=zs(J.height,A.y,G),ce=Math.sin(A.heading),ne=Math.cos(A.heading),K=n(A.x+ce*.35,A.z+ne*.35)-n(A.x-ce*.35,A.z-ne*.35),oe=zs(J.pitch,Math.max(-.08,Math.min(.08,Math.atan2(K,.7)*.5)),G);J.node.position.set(A.x,me,A.z),J.node.rotation.y=A.heading,J.node.rotation.x=-oe,ye&&J.rig&&cg(J.rig,J.node,n),J.shadow?.set(A.x,n(A.x,A.z),A.z,.62,.5)}}return{add:z,patrol:X,update:ge,suspend:H,settle:Ze,addPlace(G,ye){if(!d.has(G.id)){d.set(G.id,G);for(let J of ye.navigation.workpoints||[])u.push({id:G.id+":"+J.id,place:G.id,role:G.role,x:G.x+J.position[0],y:n(G.x,G.z),z:G.z+J.position[2],owner:null,lobby:!!G.lobby})}},nearby(){let G=[];for(let ye of h)if(ye.body.enabled){let J=Math.hypot(t.position.x-ye.body.x,t.position.z-ye.body.z);J<5&&Math.abs(t.position.y-ye.body.y)<8&&G.push({kind:"resident",id:ye.id,distance:J,x:ye.body.x,z:ye.body.z})}for(let ye of d.values()){if(ye.lobby)continue;let J=Math.hypot(t.position.x-ye.x,t.position.z-ye.z);J<7&&G.push({kind:"demonstrate",id:ye.id,distance:J,x:ye.x,z:ye.z})}return G},inspect(G){let ye=h.find(J=>J.id===G);return N(ye),P(ye)},action(G,ye){let J=h.find(A=>A.id===f);if(!J)return null;if(J.meeting){let A=J.meeting,me=p.indexOf(A);ie(A),me>=0&&p.splice(me,1)}if(G==="greet"&&(I(J),J.guide=null,J.pause?.(!0),J.state="greet",J.until=m+3,J.cooldown=m+5,J.play?.("greet")),G==="guide"&&!J.pause&&ye){I(J);for(let A of[0,1.5,3]){for(let me=0;me<(A?8:1)&&!J.path.length;me++){let ce=ye.x+Math.cos(me*Math.PI/4)*A,ne=ye.z+Math.sin(me*Math.PI/4)*A;J.path=x.route(J.body,{x:ce,z:ne,y:n(ce,ne)})}if(J.path.length)break}J.guide=J.path.length?J.path[J.path.length-1]:null,J.state=J.guide?"guide":"idle"}return G==="cancel"&&(I(J),J.guide=null,J.pause?.(!1),J.state=J.pause?"patrol":"idle",J.until=m+1),N(J),P(J)},demonstrate(G,ye=!1){d.has(G)&&a?.(G,ye?0:6)},setTier(G){E=G},setReplay(G){b!==G&&(b=G,H())},event(G){if(b||!o()||!["started","succeeded","failed","sanitized","progress"].includes(G.state)||!Number.isFinite(G.at)||Date.now()-G.at>2500||G.at>Date.now())return;let ye=us.find(J=>J.district===(G.to==="agent"?G.from:G.to));ye&&d.has(ye.id)&&a?.(ye.id,2,G.state)},stats:()=>({completed:_,lobbyVisits:T,byPlace:{...y},byResident:{...C},rigs:h.filter(G=>G.rig).length,meetings:p.length,reservations:u.filter(G=>G.owner).length,signals:M.stats(),residents:h.filter(G=>G.body.enabled).map(G=>({...P(G),x:G.body.x,z:G.body.z,heading:G.body.heading})),places:[...d.keys()]}),dispose(){g=!0,H(),h.forEach(G=>e.remove(G.id)),h.length=0,d.clear(),u.length=0,M.dispose()}}}function dg(s){let e=new Et;e.name="living-machinery",s.add(e);let t=new Set,n=new Set,i=new Map,r=0,a="high",o=E=>(t.add(E),E),l=E=>(n.add(E),E),c=o(new Qn(1,.025,6,64)),h=o(new Vr(1,48)),d=o(new At),u=new Float32Array(288),p=new Float32Array(96);for(let E=0;E<96;E++)u[E*3]=Math.cos(E*2.399)*Math.sqrt(E/96),u[E*3+2]=Math.sin(E*2.399)*Math.sqrt(E/96),p[E]=E/96;d.setAttribute("position",new Pt(u,3)),d.setAttribute("phase",new Pt(p,1));for(let E of us){let g=new Et;g.name=E.id,g.position.set(E.x,.12,E.z),g.visible=!1,e.add(g);let _=l(new jt({color:7529695,transparent:!0,opacity:.15,depthWrite:!1,blending:rn,toneMapped:!1})),w=[];for(let C=0;C<3;C++){let S=new ht(c,l(_.clone()));S.rotation.x=-Math.PI/2,g.add(S),w.push(S)}let T=new ht(h,l(_.clone()));T.rotation.x=-Math.PI/2,T.scale.setScalar(2.5),T.position.y=.2,g.add(T);let y=new Rn(d,l(new Rt({transparent:!0,depthWrite:!1,blending:rn,uniforms:{time:{value:0},strength:{value:0},color:{value:new Le(10414816)}},vertexShader:"attribute float phase;uniform float time;uniform float strength;varying float a;void main(){float t=fract(phase+time*.31);vec3 p=position*vec3(2.,1.,2.);p.y=t*3.;p.xz*=1.-t*.7;vec4 v=modelViewMatrix*vec4(p,1.);a=sin(t*3.14159)*strength;gl_PointSize=clamp(65./max(1.,-v.z),1.,5.);gl_Position=projectionMatrix*v;}",fragmentShader:"uniform vec3 color;varying float a;void main(){float d=length(gl_PointCoord-.5);gl_FragColor=vec4(color,a*(1.-smoothstep(.05,.5,d)));}"})));g.add(y),i.set(E.id,{group:g,waves:w,base:T,motes:y,district:E.district,started:0,until:0,source:"ambient",model:null})}let x=new ht(o(new dn(3.9,2.3)),l(new Rt({transparent:!0,depthWrite:!1,side:Kt,blending:rn,uniforms:{time:{value:0},strength:{value:0}},vertexShader:"varying vec2 q;void main(){q=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 q;uniform float time;uniform float strength;void main(){float line=pow(max(0.,1.-abs(q.y-fract(time*.4))*14.),3.);float grid=step(.94,fract(q.x*24.))*step(.9,fract(q.y*18.));gl_FragColor=vec4(.18,.9,.8,(line*.36+grid*.1)*sin(q.x*3.14159)*strength);}"})));x.position.set(0,1.4,.8),i.get("repair-bay").group.add(x);let M=i.get("kinetic-fountain"),m=l(new Rt({transparent:!0,depthWrite:!1,side:Kt,uniforms:{time:{value:0}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 vUv;uniform float time;void main(){float pulse=.5+.5*sin(vUv.x*95.-time*9.);float edge=.35+.65*pow(max(0.,sin(vUv.y*3.14159)),2.);gl_FragColor=vec4(mix(vec3(.09,.35,.42),vec3(.6,.95,1.),pow(max(0.,pulse),5.)),edge*.56);}"}));for(let E=0;E<8;E++){let g=E*Math.PI/4,_=new U(Math.cos(g)*2.3,.5,Math.sin(g)*2.3),w=new U(Math.cos(g+.5)*.7,.55,Math.sin(g+.5)*.7),T=new Mi(_,new U(Math.cos(g)*1.5,3.7,Math.sin(g)*1.5),w);M.group.add(new ht(o(new Xa(T,24,.045,5,!1)),m))}let f=l(new Rt({transparent:!0,depthWrite:!1,uniforms:{time:{value:0}},vertexShader:"varying vec2 q;uniform float time;void main(){q=position.xy;vec3 p=position;p.z+=sin(length(q)*18.-time*4.)*.012;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.);}",fragmentShader:"varying vec2 q;uniform float time;void main(){float r=length(q);float a=.5+.5*sin(r*16.-time*3.+sin(q.x*3.+q.y*2.+time*.7)*.35);float rim=1.-smoothstep(2.35,2.58,r);gl_FragColor=vec4(mix(vec3(.03,.13,.17),vec3(.22,.55,.58),pow(max(0.,a),9.)*.3),.78*rim);}"})),b=new ht(o(new Vr(2.58,64)),f);return b.rotation.x=-Math.PI/2,b.position.y=.3,M.group.add(b),{attach(E,g){let _=i.get(E);_&&(_.model=g,_.group.visible=!0)},demonstrate(E,g=6,_=null){let w=i.get(E);w&&(w.until<=r&&(w.started=r),w.until=r+g,w.source=_?"live":"ambient",w.status=_,w.model?.play("operate"))},clear(){i.forEach(E=>{E.until=0,E.source="ambient",E.status=null})},setTier(E){a=E},update(E,g){g&&(r+=E),m.uniforms.time.value=f.uniforms.time.value=x.material.uniforms.time.value=r;for(let[_,w]of i){let T=w.until>r,y=_==="kinetic-fountain",C=T?It.smoothstep(r-w.started,0,.35)*It.smoothstep(w.until-r,0,.65):0,S=y?Math.max(.25,C):C;T||(w.source="ambient",w.status=null);let D=w.status==="failed"?16737881:w.source==="live"?{infra:7978495,integrations:7728086,missions:10268415,graph:16765844,operations:16758915}[w.district]:7529695;if(w.waves.forEach((v,I)=>{v.visible=T;let P=(r*.6+I/3)%1;v.position.y=.4+P*2,v.scale.setScalar(.4+P*2.6),v.material.opacity=(1-P)*.22*C,v.material.color.setHex(D)}),_==="repair-bay"&&(x.visible=T,x.material.uniforms.strength.value=C),_==="relay-mast"&&T){let v=w.model?.node.getObjectByName("antenna");v&&(v.rotation.y=It.damp(v.rotation.y,Math.atan2(-w.group.position.x,-12-w.group.position.z),3,g?E:0))}w.base.material.opacity=.08*C,w.motes.visible=a!=="low"&&S>0,w.motes.material.uniforms.time.value=r,w.motes.material.uniforms.strength.value=S,w.model?.mixer&&(w.model.mixer.timeScale=g&&(T||y)?.65:0)}},stats:()=>({places:[...i].filter(([,E])=>E.model).map(([E,g])=>({id:E,operating:g.until>r,source:g.source})),time:r}),dispose(){e.removeFromParent(),t.forEach(E=>E.dispose()),n.forEach(E=>E.dispose()),i.clear()}}}var aw={cruise:12,boost:26,climb:7,sink:7,ease:5,eye:2.4,takeoff:1.5,takeoffTime:.7,ceiling:150,minX:-140,maxX:140,minZ:-150,maxZ:130,glide:8,descend:6,reach:40,bob:.03,slack:15},Gt=aw,ur=(s,e,t)=>Math.max(e,Math.min(t,s)),ow=s=>s<=0?0:s>=1?1:s*s*(3-2*s);function fg({clear:s,ground:e,walkable:t,reduced:n=()=>!1}){let i="idle",r=!1,a=0,o=0,l=0,c=0,h=0,d=0,u=0,p=0,x=0,M=null,m=0,f=!1,b=0,E=0,g=(v,I)=>e(v,I)+Gt.eye,_=()=>({x:a,y:o+m,z:l}),w=()=>{c=h=d=0},T=(v,I,P)=>(v+=(I-v)*(1-Math.exp(-Gt.ease*P)),Math.abs(v)<.01&&!I?0:v);function y(v,I=[]){for(let P=0;P<=Math.max(Gt.maxX-Gt.minX,Gt.maxZ-Gt.minZ);P+=P<Gt.reach?1.5:4){let N=P?Math.ceil(P*Math.PI*2/(P<Gt.reach?1.5:4)):1;for(let z=0;z<N;z++){let X=z/N*Math.PI*2,j=a+Math.cos(X)*P,V=l+Math.sin(X)*P;if(j<Gt.minX||j>Gt.maxX||V<Gt.minZ||V>Gt.maxZ||I.some(ie=>Math.hypot(ie.x-j,ie.z-V)<1)||!t(j,V))continue;let W=g(j,V);if(!(v&&!s({x:j,y:Math.max(o,W),z:V},{x:j,y:W,z:V})))return{x:j,y:W,z:V,blocked:0,skipped:I}}}return null}let C=v=>{a=v.x,o=v.y,l=v.z,w(),m=0,i="idle",r=!0,M=null};function S(v){let I=a,P=l,N=ur(a+c*v,Gt.minX,Gt.maxX),z=ur(l+d*v,Gt.minZ,Gt.maxZ);N!==a+c*v&&(c=0),z!==l+d*v&&(d=0),N!==a&&(s({x:a,y:o,z:l},{x:N,y:o,z:l})?a=N:c=0),z!==l&&(s({x:a,y:o,z:l},{x:a,y:o,z})?l=z:d=0);let X=g(a,l);o<X&&(s({x:a,y:o,z:l},{x:a,y:X,z:l})?o=X:(a=I,l=P,c=d=0,X=g(a,l)));let j=ur(o+h*v,X,Gt.ceiling);j!==o+h*v&&(h=0),j!==o&&(s({x:a,y:o,z:l},{x:a,y:j,z:l})?o=j:h=0)}function D(v){let I=M,P=I.x-a,N=I.z-l,z=Math.hypot(P,N),X=o-I.y;if(b+=v,b>E){let Ze=y(!1);Ze?C(Ze):(i="flying",M=null);return}if(z<.05&&X<.02){C(I);return}let j=z>.05?Math.min(Gt.glide,Math.max(.8,z*1.4)):0,V=X>.02?Math.min(Gt.descend,Math.max(.6,X*1.8)):0,W=z>.05?P/z*Math.min(j*v,z):0,ie=z>.05?N/z*Math.min(j*v,z):0,H=-Math.min(V*v,Math.max(0,X)),ge={x:a,y:o,z:l};if(s(ge,{x:a+W,y:o+H,z:l+ie}))a+=W,o+=H,l+=ie,c=W/v,d=ie/v,h=H/v,I.blocked=0;else if((W||ie)&&s(ge,{x:a+W,y:o,z:l+ie}))a+=W,l+=ie,c=W/v,d=ie/v,h=0,I.blocked=0;else if(w(),z>=.05&&o<Gt.ceiling&&s(ge,{x:a,y:o+Gt.descend*v,z:l}))o+=Gt.descend*v,h=Gt.descend,I.blocked=0;else if((I.blocked+=v)>1){let Ze=y(!0,[...I.skipped,I]);Ze?M=Ze:(i="flying",M=null)}}return{start(v){return a=v.x,o=Math.max(v.y,g(v.x,v.z)),l=v.z,w(),i="takeoff",r=!1,u=0,p=o,x=0,M=null,m=0,_()},place(v){a=v.x,o=v.y,l=v.z,w(),m=0},step(v,I={},P=0){if(v=ur(v,0,.1),i==="landing"&&(+I.forward||+I.right||+I.up?f&&(i="flying",M=null):f=!0),i==="takeoff"){u=n()?Gt.takeoffTime:Math.min(Gt.takeoffTime,u+v);let N=p+Gt.takeoff*ow(u/Gt.takeoffTime);N>o&&s({x:a,y:o,z:l},{x:a,y:N,z:l})&&(o=N),u>=Gt.takeoffTime&&(i="flying")}else if(i==="landing")v>0&&D(v);else if(i==="flying"){let N=ur(+I.forward||0,-1,1),z=ur(+I.right||0,-1,1),X=Math.hypot(N,z);X>1&&(N/=X,z/=X);let j=I.boost?Gt.boost:Gt.cruise,V=Math.sin(P),W=Math.cos(P),ie=ur(+I.up||0,-1,1);c=T(c,(-V*N+W*z)*j,v),d=T(d,(-W*N-V*z)*j,v),h=T(h,ie*(ie>0?Gt.climb:Gt.sink),v),S(v),x+=v,m=n()?0:Math.min(Gt.bob,Math.max(0,o-g(a,l)),Gt.ceiling-o)*Math.sin(x*2.1)}return _()},land(v=!1){let I=v||n();if(i==="idle")return!1;if(i==="landing"&&!I)return!0;let P=y(!I);return P?I?(C(P),!0):(M=P,i="landing",m=0,f=!1,b=0,E=Gt.slack+Math.max(Math.hypot(P.x-a,P.z-l)/Gt.glide,Math.max(0,o-P.y)/Gt.descend),!0):!1},cancel(){return i!=="landing"?!1:(i="flying",M=null,!0)},reset(){i="idle",r=!1,M=null,w(),m=0},state(){return{phase:i,flying:i!=="idle",landing:i==="landing",target:M?{x:M.x,z:M.z}:null,done:r,x:a,y:o+m,z:l,altitude:Math.max(0,o-g(a,l)),speed:Math.hypot(c,d),climb:h}}}}function pg(s,e){let t=new Et;t.name="world-2",s.add(t);let n=new Map,i=new Set,r=new Set,a=[],o=[],l=[],c=new Map,h=new Map,d=[],u=new Map,p=new Map,x=new Set,M=[],m=new Set,f=[],b=new Map,E=new AbortController,g=[],_=0,w=!1,T=!1,y=e.tier||"high",C=null,S=0,D=0,v=null,I=null,P=0,N=new Set,z=null,X=0,j="",V=new U,W=new U,ie=new U,H=e.camera,ge={ready:!1,lift:null,cab:null,value:0,motion:null,idle:0},Ze=[],G=new Vn(0,0,0,"YXZ"),ye=F=>Math.atan2(Math.sin(F),Math.cos(F)),J=F=>F<=0?0:F>=1?1:F*F*(3-2*F),A=e.traffic||th(),me=dg(s),ce=new Map,ne=Am(),K=null;function oe(F,q){let $=Dn(F,q);for(let de of us)for(let Me of C?.assets.find(he=>he.id===de.id)?.navigation.surfaces||[]){let[he,Pe,le,qe]=Me.rect;F>=de.x+he&&F<=de.x+le&&q>=de.z+Pe&&q<=de.z+qe&&($=Math.max($,Dn(de.x,de.z)+Me.height))}return $}let re=(F,q)=>{let $=Im(F,q,e.districts||[]);return $>0?Math.max(oe(F,q),$):oe(F,q)},te=[],fe=()=>A.body("visitor"),Ee=new U,Ne=fg({ground:re,reduced:()=>e.reduced(),clear:(F,q)=>!Sm(q.x,q.y,q.z)&&(e.clear?e.clear(F,q):!fe()||A.clear(fe(),{...F,heading:0},{...q,heading:0},!1)),walkable:(F,q)=>{if(Us(F,q)||Fs(F,q)!==null||!Ed(F,q,{x:F,z:q},()=>!1,e.districts||[]))return!1;let $={x:F,y:re(F,q)+2.4,z:q,heading:0};return!fe()||A.clear(fe(),$,$)}}),Oe=ug(s,{traffic:A,camera:H,floor:oe,catalog:e.kit3,onSelect:e.onSociety,onDemonstrate:me.demonstrate,active:e.active,onGait:e.onGait,shadows:e.shadows}),ut=0,rt=0;try{N=new Set(JSON.parse(localStorage.getItem("aurago.desktop.sysworld.discoveries")||"[]").filter(F=>ui.some(q=>q.id===F)))}catch{}let at=md(xd),Q=at.getLength(),vt=ui.map(F=>{let q=1/0,$=0;for(let de=0;de<3e3;de++){at.getPointAt(de/3e3,V);let Me=Math.hypot(V.x-F.x,V.z-F.z);Me<q&&(q=Me,$=de/3e3)}return{...F,u:$}}),je=(F,q,$)=>e.onSound?.(F,q?.x||0,q?.y||0,q?.z||0,$),O=new vn({color:11834208,metalness:.8,roughness:.35});r.add(O);for(let F of[-1,1]){let q=[],$=[];for(let he=0;he<=512;he++){at.getPointAt(he/512,V),at.getTangentAt(he/512,W);for(let Pe of[-.045,.045]){let le=F*.85+Pe;q.push(V.x-W.z*le,bt.road+.09,V.z+W.x*le)}if(he<512){let Pe=he*2;$.push(Pe,Pe+1,Pe+2,Pe+1,Pe+3,Pe+2)}}let Me=new At;Me.setAttribute("position",new Mt(q,3)),Me.setIndex($),Me.computeVertexNormals(),i.add(Me),t.add(new ht(Me,O))}async function R(F,q=y==="low"?2:y==="medium"?1:0){let $=F+":"+q;return n.has($)||n.set($,(async()=>{let de=C.assets.find(Xe=>Xe.id===F),Me=de?.lods.find(Xe=>Xe.level===q);if(!Me||!/^[\w-]+\.lod[0-2]\.glb$/.test(Me.file))throw Error("Invalid world asset");let he=await fetch(e.assetURL(Me.file),{signal:E.signal});if(!he.ok)throw Error("World asset unavailable");let Pe=await he.arrayBuffer();if(T)throw Error("Disposed");S+=Pe.byteLength;let le=await new Ri().parseAsync(Pe,""),qe=le.animations.length>0||/^(tram|service-cart|robot-|door|lift|sky-lift|telescope)/.test(F);if(le.scene.traverse(Xe=>{if(Xe.isMesh){i.add(Xe.geometry);for(let it of Array.isArray(Xe.material)?Xe.material:[Xe.material])it.isMeshStandardMaterial&&it.metalness>.5&&(it.roughness=Math.max(it.roughness,.45)),r.add(it);Xe.castShadow=!0,Xe.receiveShadow=!0,e.surfaces?.prepare(Xe,qe)}}),T)throw i.forEach(Xe=>Xe.dispose()),r.forEach(Xe=>Xe.dispose()),Error("Disposed");return x.add(F),le})()),n.get($)}async function ee(F,q,$,de=0,Me=0,he=t,Pe=[1,1,1],le){let qe=await R(F,le);if(T)return null;let Xe=qe.scene.clone(!0);Xe.position.set(q,de,$),Xe.rotation.y=Me,Xe.scale.set(...Pe),he.add(Xe),Xe.traverse(Zt=>{Zt.isMesh&&(Zt.userData.worldAsset=F,Zt.userData.worldPart=Zt.name,M.push(Zt))});let it=C.assets.find(Zt=>Zt.id===F),zt="furnishing:"+ut++;for(let[Zt,en]of(it.navigation.colliders||[]).entries())A.solid(zt+":"+Zt,{owner:zt,x:q,z:$,heading:Me,min:[en[0]*Pe[0],de+en[1]*Pe[1],en[2]*Pe[2]],max:[en[3]*Pe[0],de+en[4]*Pe[1],en[5]*Pe[2]]});let Bt=qe.animations.length?new io(Xe):null;Bt&&a.push(Bt);let pn=null,ii=null,Kn=/^robot-/.test(F);return{node:Xe,mixer:Bt,owner:zt,bounds:it.motion_bounds||it.lods[0].bounds,clips:qe.animations,play(Zt,en=!1){if(!Bt)return;let Pn=qe.animations.find(kn=>kn.name===Zt);if(!Pn)return;let En=Bt.clipAction(Pn);if(pn===En)return En;let Ln=ii;return En.reset(),en&&(En.setLoop(Fc,1),En.clampWhenFinished=!0),En.play(),pn&&En.crossFadeFrom(pn,.25,!1),pn=En,ii=Zt,Kn&&Ln&&Zt!==Ln&&Zt!=="walk"&&e.onSound?.("robot_"+Zt,Xe.position.x,Xe.position.y,Xe.position.z,zt),En}}}function ae(F){F.updateMatrixWorld(!0);let q=new Map;F.traverse(de=>{if(!de.isMesh)return;let Me=de.geometry.uuid+":"+(Array.isArray(de.material)?de.material.map(he=>he.uuid).join(","):de.material.uuid);q.has(Me)||q.set(Me,{n:de,matrices:[]}),q.get(Me).matrices.push(de.matrixWorld.clone())});let $=new Et;t.add($);for(let{n:de,matrices:Me}of q.values()){let he=new An(de.geometry,de.material,Me.length);Me.forEach((Pe,le)=>he.setMatrixAt(le,Pe)),he.castShadow=!0,he.receiveShadow=!0,he.userData={...de.userData},M.push(he),$.add(he)}return F.traverse(de=>{let Me=M.indexOf(de);Me>=0&&M.splice(Me,1)}),F.removeFromParent(),u.set($.uuid,$),$}function _e(F){if(p.has(F.id))return p.get(F.id);let q=(async()=>{let $=new Et;t.add($);let de=[];for(let Me of hm())de.push(ee("floor",F.x+Me.x,F.z+Me.z,bt.room,0,$,[Me.sx,1,Me.sz]));for(let Me=-4;Me<=4;Me+=4)for(let he=-4;he<=4;he+=4)de.push(ee("ceiling",F.x+Me,F.z+he,8.15+bt.room,0,$));for(let Me=-4;Me<=4;Me+=4)de.push(ee("wall",F.x+Me,F.z+6,bt.room,0,$));for(let Me=-4;Me<=4;Me+=4)for(let he of[-1,1])for(let Pe of[0,4])de.push(ee("window",F.x+he*6,F.z+Me,bt.room+Pe,Math.PI/2,$));for(let Me=-4;Me<=4;Me+=4)for(let he of[-1,1])de.push(ee("window",F.x+Me,F.z+he*6,bt.gallery,0,$));for(let Me of[-4,4])de.push(ee("wall",F.x+Me,F.doorZ,bt.room,0,$));for(let[Me,he]of[[-4,1],[.2,1.1]])de.push(ee("floor",F.x+Me,F.z+3,bt.gallery,0,$,[he,1,1.5])),de.push(ee("railing",F.x+Me,F.z,bt.gallery,0,$,[he,1,1]));for(let Me of[.7,5.3])de.push(ee("railing",F.x+2.4,F.z+Me,bt.gallery,Math.PI/2,$,[.35,1,1]));for(let Me of[-5.6,1.9])de.push(ee("wall",F.x+Me,F.z+.15,bt.room,0,$,[.05,1,.6]));if(await Promise.all(de),!T)return ae($)})();return p.set(F.id,q),q}async function Ue(F){if(!(c.has(F.id)||T)){c.set(F.id,{ready:!1,lift:null,liftValue:0,liftTarget:0}),X++;try{await _e(F);let q=new Et;t.add(q);let $=[];for(let Me of[-3,0])$.push(ee(F.id==="missions"?"cargo":F.id==="memory"?"archive-shelf":"console",F.x+Me,F.z+3,bt.room,Math.PI,q));if($.push(ee("bench",F.x-4,F.z-1,bt.room,Math.PI/2,q)),$.push(ee("bench",F.x-4,F.z+4.8,bt.gallery,0,q)),$.push(ee("console",F.x,F.z+4.8,bt.gallery,Math.PI,q)),await Promise.all($),T)return;ae(q);let de=c.get(F.id);de.hologram=await ee("hologram",F.x-1,F.z,bt.room),de.hologram?.play("operate"),F.id==="memory"&&Z(),de.lift=await ee("lift",F.liftX,F.liftZ,bt.room),de.liftAction=de.lift?.play("operate",!0),de.liftAction&&(de.liftAction.paused=!0),await pe(F.id,F.x-2.2,F.z+.9,bt.gallery,Math.PI,{skip:null,limit:.9,pitch:[-.14,.18],eye:0}),de.ready=!0}catch(q){T||e.onError?.(q)}finally{X--}}}async function ke(){try{let F=await fetch(e.assetURL("manifest.json"),{signal:E.signal});if(!F.ok)throw Error("World manifest unavailable");C=await F.json();let q=new Et;t.add(q);let $=[];for(let he of ui)$.push(ee("station",he.platformX,he.platformZ,bt.pavement,he.angle,q));for(let he of On){$.push(_e(he));let Pe=await ee("door",he.x,he.doorZ,bt.room,Math.PI);if(T)return;let le=Pe.play("open",!0);le.paused=!0,h.set(he.id,{...Pe,action:le,value:0,open:!1})}for(let he=-76;he<=76;he+=8)$.push(ee("quay",he,77,bt.quay,0,q));for(let he of[-68,-52,-28,28,52,68])$.push(ee("quay-lantern",he,75.5,bt.quay,0,q));for(let[he,Pe]of[[-79,68],[-79,-67],[55,4],[8,40],[45,-23]])$.push(ee("garden",he,Pe,0,0,q)),$.push(ee("bench",he+3,Pe,0,Math.PI/2,q));for(let[he,Pe]of[[-26,23],[30,23],[-28,72]])$.push(ee("arcade",he,Pe,0,0,q));$.push(ee("bridge",-78.5,40,2,0,q,[1.75,1,1]),ee("ramp",-80,49,0,Math.PI,q),ee("stairs",-76.8,49,0,Math.PI,q),ee("ramp",-78.5,31,0,0,q)),$.push(ee("pad",Wi.x,Wi.z,0,0,q)),$.push(ee("sky-deck",Jt.x,Jt.z,Jt.floor,0,q));for(let he of Jt.benches)$.push(ee("bench",he.x,he.z,Jt.floor,he.angle,q));for(let[he,Pe]of[[-60,-56],[-60,-46],[51,51]])$.push(ee("charger",he,Pe,0,0,q));if(await Promise.all($),T||(ae(q),ge.lift=await ee("sky-lift",Jt.lift.x,Jt.lift.z,Jt.lift.base),T))return;ge.cab=ge.lift?.node.getObjectByName("cab")||null,ge.ready=!!ge.cab;for(let he of Jt.telescopes)await pe(he.id,he.x,he.z,Jt.floor,he.angle,{skip:"agent",limit:1.45,pitch:[-1.1,.3],eye:1.25});for(let[he,Pe]of[[-59,-61],[-59,-52]])(await ee("cooler",he,Pe))?.play("operate");for(let he of us){let Pe=await ee(he.id,he.x,he.z,Dn(he.x,he.z),0,t,[1,1,1],2);if(T)return;Pe&&(Pe.level=2,ce.set(he.id,Pe),me.attach(he.id,Pe),Oe.addPlace(he,C.assets.find(le=>le.id===he.id)),Pe.play("operate"))}for(let he of Jm){let Pe=await ee(he.id,he.x,he.z,he.y,he.angle||0,t);if(!Pe){if(T)return;continue}Pe.play(he.clip),d.push({id:he.id,model:Pe})}let de=C.assets.find(he=>he.id==="hover-pack");if(de)try{let he=de.navigation.interaction?.[0]?.position||[0,0,1.2];for(let Pe of lm){let le=await ee("hover-pack",Pe.x,Pe.z,re(Pe.x,Pe.z),Pe.angle);if(T)return;if(!le)continue;le.play("idle");let qe=Math.sin(Pe.angle),Xe=Math.cos(Pe.angle);te.push({...Pe,model:le,pack:le.node.getObjectByName("pack")||null,docked:!0,ix:Pe.x+he[0]*Xe+he[2]*qe,iz:Pe.z-he[0]*qe+he[2]*Xe})}}catch(he){T||e.onError?.(he)}let Me=await ee("service-cart",55,51.2,bt.ground,Math.PI/2);Me&&Me.play("open",!0);for(let he=0;he<19;he++){let Pe=["courier","technician","archivist"][he%3],le=await ee("robot-"+Pe,0,0);if(!le)return;le.node.scale.setScalar(1.15),o.push(le),Oe.add(le,he,le.bounds),le.play("idle")}for(let he=0;he<2;he++){let Pe=await ee("tram",0,0);if(!Pe)return;let le=Pe.play("open",!0);le.paused=!0;let qe=vt[he?3:0].u;at.getPointAt(qe,V),at.getTangentAt(qe,W);let Xe=A.register("tram-"+he,cs(Pe.bounds),{x:V.x,y:bt.road+.03,z:V.z,heading:Math.atan2(W.x,W.z)},3);l.push({...Pe,body:Xe,u:qe,speed:0,dwell:5,stop:he?3:0,door:le})}for(let he=0;he<3;he++){let Pe=await ee("cargo",43,70);Pe&&(A.removeOwner(Pe.owner),Pe.node.visible=!1,f.push({...Pe,body:null,elapsed:9,direction:1}))}for(let{place:he,entry:Pe}of Cm(e.kit3))Oe.addPlace(he,Pe);e.onReady?.()}catch(F){T||e.onError?.(F)}}ke();async function B(F,q,$){if(!(q.level===$||q.requested===$)){q.requested=$;try{let de=await R(F,$);if(T||q.requested!==$)return;let Me=new Map;de.scene.traverse(he=>{he.isMesh&&Me.set(he.name,he)}),q.node.traverse(he=>{let Pe=Me.get(he.name);he.isMesh&&Pe&&(he.geometry=Pe.geometry,he.material=Pe.material)}),q.level=$,q.requested=null,e.onReady?.()}catch(de){q.requested=null,T||e.onError?.(de)}}}function Z(){let F=c.get("memory");if(!F?.hologram||!F.text&&!g.length)return;if(!F.text){let he=document.createElement("canvas");he.width=1024,he.height=512;let Pe=new Zs(he);Pe.colorSpace=an,m.add(Pe);let le=new dn(4.8,2.4),qe=new jt({map:Pe,transparent:!0,depthWrite:!1,side:Kt,toneMapped:!1});i.add(le),r.add(qe);let Xe=new ht(le,qe),it=On.find(zt=>zt.id==="memory");Xe.position.set(it.x-1,3.6+bt.room,it.z+.3),Xe.rotation.y=Math.PI,t.add(Xe),F.text={canvas:he,texture:Pe,mesh:Xe}}let{canvas:q,texture:$,mesh:de}=F.text,Me=q.getContext("2d");Me.clearRect(0,0,1024,512),de.visible=g.length>0,Me.fillStyle="rgba(5,24,32,.87)",Me.fillRect(0,0,1024,512),Me.fillStyle="#b9f4ef",Me.font="28px sans-serif",g.slice(0,4).forEach((he,Pe)=>{let le=Array.from(he).slice(0,96);Me.fillText(le.slice(0,48).join(""),30,55+Pe*118),Me.fillText(le.slice(48).join(""),30,94+Pe*118)}),$.needsUpdate=!0}async function pe(F,q,$,de,Me,he){let Pe=await ee("telescope",q,$,de,Me);!Pe||T||Ze.push({id:F,model:Pe,tube:Pe.node.getObjectByName("tube"),x:q,z:$,floor:de,angle:Me,...he,ex:q+Math.sin(Me)*he.eye,ez:$+Math.cos(Me)*he.eye})}function Te(F){let q=Ze.find(le=>le.id===F);if(!q)return;let $=ye(q.angle+Math.PI),de=q.floor+2.05,Me=$,he=-.08,Pe=1/0;for(let le of e.districts||[]){if(le.id===q.skip)continue;let qe=le.x-q.ex,Xe=le.z-q.ez,it=Math.hypot(qe,Xe),zt=Math.atan2(-qe,-Xe),Bt=Math.abs(ye(zt-$));it>12&&Bt<q.limit&&Bt<Pe&&(Pe=Bt,Me=zt,he=It.clamp(Math.atan2(le.height*.45-de,it),...q.pitch))}v={kind:"scope",id:F,scope:q,yaw0:$,fov:14,sent:"",target:null,back:{position:H.position.clone(),quaternion:H.quaternion.clone(),fov:H.fov}},q.model.node.visible=!1,H.position.set(q.ex,de,q.ez),H.quaternion.setFromEuler(G.set(he,Me,0,"YXZ")),e.reduced()&&(H.fov=v.fov,H.updateProjectionMatrix()),e.onRide?.("scope"),je("door",H.position)}function ve(F,q,$){let de=$.radius*.6,Me=F.x-$.x,he=F.z-$.z,Pe=q.x*q.x+q.z*q.z;if(Pe<1e-9)return 1/0;let le=2*(Me*q.x+he*q.z),qe=le*le-4*Pe*(Me*Me+he*he-de*de);if(qe<0)return 1/0;let Xe=(-le-Math.sqrt(qe))/(2*Pe);if(Xe<0&&(Xe=(-le+Math.sqrt(qe))/(2*Pe)),Xe<0)return 1/0;let it=F.y+q.y*Xe;return it>=0&&it<=$.height?Xe:1/0}function we(F){let q=v.scope;G.setFromQuaternion(H.quaternion,"YXZ"),G.y=v.yaw0+It.clamp(ye(G.y-v.yaw0),-q.limit,q.limit),G.x=It.clamp(G.x,...q.pitch),G.z=0,H.quaternion.setFromEuler(G),H.position.set(q.ex,q.floor+2.05,q.ez);let $=e.reduced()?v.fov:It.damp(H.fov,v.fov,9,Math.min(F,.1));Math.abs($-v.fov)<.02&&($=v.fov),$!==H.fov&&(H.fov=$,H.updateProjectionMatrix()),H.getWorldDirection(V);let de=null,Me=1/0;for(let qe of e.districts||[]){if(qe.id===q.skip)continue;let Xe=ve(H.position,V,qe);Xe>12&&Xe<Me&&(Me=Xe,de=qe.id)}v.target=de;let he=v.back.fov/H.fov,Pe=(Math.atan2(V.x,-V.z)*180/Math.PI+360)%360,le=[de,de?Math.round(Me):0,he.toFixed(1),Math.round(Pe)].join();le!==v.sent&&(v.sent=le,e.onScope?.({id:v.id,target:de,distance:de?Math.round(Me):null,zoom:+he.toFixed(1),bearing:Math.round(Pe)%360}))}function ue(F,q,$){ge.motion={from:ge.value,to:F,t:0,duration:e.reduced()?0:q},ge.idle=0,$&&je("lift",ge.lift.node.position)}function Ae(){let F=Jt.lift;H.position.set(F.x,F.base+F.cab+ge.value+2.4,F.z)}function ze(F){ue(F,15,!0),v={kind:"sky",to:F},Ae(),H.quaternion.setFromEuler(G.set(-.42,Math.PI,0,"YXZ")),e.onRide?.("sky")}function Y(F,q){let $=Jt.lift;if(!ge.motion&&v?.kind!=="sky"&&q>0&&Math.hypot(H.position.x-$.x,H.position.z-$.z)>30&&(ge.idle+=q,ge.idle>28&&ue(ge.value>1?0:$.travel,15,!1)),ge.motion){let de=ge.motion;de.t+=Math.min(F,.1);let Me=de.duration?de.t/de.duration:1;ge.value=de.from+(de.to-de.from)*J(Me),Me>=1&&(ge.value=de.to,ge.motion=null)}ge.cab.position.y=ge.value,v?.kind==="sky"&&(Ae(),ge.motion||(v=null,e.onRide?.(null),je("lift",H.position)))}function Ce(F){let q=te.find($=>$.id===F);!q?.docked||e.reduced()||(q.docked=!1,q.pack&&(q.pack.visible=!1),v={kind:"hover",id:F},j="",Ee.copy(H.position),Ne.start(H.position),e.onRide?.("hover"),je("hover",H.position),He())}function Re(F){let q=te.find($=>$.id===v?.id);q&&(q.docked=!0,q.pack&&(q.pack.visible=!0)),v=null,j="",e.onRide?.(null),e.onFlight?.(null),F&&je("hover",H.position)}function Ve(){let F=fe();F&&A.relocate(F,H.position)&&H.position.set(F.x,F.y,F.z)}function He(){let F=Ne.state();if(F.done){H.position.set(F.x,F.y,F.z),Ve(),Re(!0);return}let q=[Math.round(F.altitude),Math.round(F.speed*3.6),F.landing].join();q!==j&&(j=q,e.onFlight?.({altitude:Math.round(F.altitude),speed:Math.round(F.speed*3.6),landing:F.landing}))}function De(){if(v?.kind==="hover"){Ne.state().landing?Ne.cancel():Ne.land(),He();return}if(v){if(v.kind==="tram"&&l[v.index].dwell<=1){v.exitRequested=!0;return}Ye();return}if(I){if(I.kind==="telescope"){Te(I.id);return}if(I.kind==="skycall"){ue(H.position.y>40?Jt.lift.travel:0,7,!0);return}if(I.kind==="skyup"||I.kind==="skydown"){ze(I.kind==="skyup"?Jt.lift.travel:0);return}if(I.kind==="resident"){Oe.inspect(I.id);return}if(I.kind==="demonstrate"){Oe.demonstrate(I.id,e.reduced()),e.onSociety?.({id:I.id,role:"installation",state:e.reduced()?"idle":"work",source:"ambient"});return}if(I.kind==="door"&&I.id.startsWith("lobby:")){ne.hold(I.id)&&je("door",H.position,I.id);return}if(I.kind==="door"){let F=h.get(I.id);if(F){F.open=!F.open,je("door",F.node.position);let q=On.find($=>$.id===I.id);Ue(q)}}if(I.kind==="discover"){N.add(I.id);try{localStorage.setItem("aurago.desktop.sysworld.discoveries",JSON.stringify([...N]))}catch{}e.onDiscover?.(I.id),je("discover",H.position)}if(I.kind==="tram"&&!e.reduced()&&(v={kind:"waiting",station:I.id},e.onRide?.("waiting")),I.kind==="terminal"&&e.onTerminal?.(I.id),I.kind==="hoverpack"){Ce(I.id);return}if(I.kind==="drone"&&!e.reduced()&&(v={kind:"drone",elapsed:0,origin:H.position.clone()},e.onRide?.("drone"),je("tram",H.position)),I.kind==="lift"){let F=c.get(I.id);if(F){let q=H.position.y>5?4:0;if(Math.abs(F.liftValue-q)>.02){F.liftTarget=q;return}F.liftTarget=q?0:4,v={kind:"lift",id:I.id},e.onRide?.("lift"),je("lift",H.position)}}}}function Ye(){if(v){if(v.kind==="hover"){let F=te.find(q=>q.id===v.id);if(Ne.land(!0)){let q=Ne.state();H.position.set(q.x,q.y,q.z)}else F&&H.position.set(F.ix,re(F.ix,F.iz)+2.4,F.iz);Ve(),Ne.reset(),Re(!1);return}if(v.kind==="lift"){let F=c.get(v.id),q=On.find($=>$.id===v.id);F.liftTarget=F.liftValue<2?0:4,H.position.set(q.x+1.5,2.4+bt.room+F.liftTarget,q.liftZ)}if(v.kind==="tram"){let F=ui.find(q=>q.id===v.station)||ui[0];H.position.set(F.platformX,2.4+Dn(F.platformX,F.platformZ),F.platformZ)}if(v.kind==="drone"&&H.position.copy(v.origin),v.kind==="sky"&&(ge.motion=null,ge.value=v.to,ge.cab.position.y=ge.value,Ae()),v.kind==="scope"){let F=v.scope;G.setFromQuaternion(H.quaternion,"YXZ"),F.tube?.rotation.set(-G.x,ye(G.y+Math.PI-F.angle),0,"YXZ");let q=G.y;F.model.node.visible=!0,H.position.copy(v.back.position),G.setFromQuaternion(v.back.quaternion,"YXZ"),G.y=q,H.quaternion.setFromEuler(G),H.fov=v.back.fov,H.updateProjectionMatrix(),e.onScope?.(null)}v=null,e.onRide?.(null)}}function et(){if(v?.kind==="hover")return Ne.state().landing?[]:[{kind:"land",id:v.id,distance:0}];if(v)return[{kind:v.kind==="scope"?"unscope":"exit",id:v.kind,distance:0}];H.getWorldDirection(V);let F=Oe.nearby().filter($=>($.x-H.position.x)*V.x+($.z-H.position.z)*V.z>$.distance*.3),q=($,de,Me,he,Pe)=>{let le=Math.hypot(H.position.x-Me,H.position.z-he);le<Pe&&F.push({kind:$,id:de,distance:le})};for(let $ of On)H.position.y<5&&q("door",$.id,$.x,$.doorZ,3),c.get($.id)?.ready&&(q("lift",$.id,$.liftX,$.liftZ,2.3),H.position.y<5&&q("terminal",$.id,$.x-2,$.z+3,2.5));if(H.position.y<5)for(let $ of ui)q("tram",$.id,$.platformX,$.platformZ,4),N.has($.id)||q("discover",$.id,$.platformX+($.angle===0?5:0),$.platformZ+($.angle===0?0:5),3);if(q("drone","drone",Wi.x,Wi.z,5),H.position.y<5&&!e.reduced())for(let $ of te)$.docked&&q("hoverpack",$.id,$.ix,$.iz,2.4);if(ge.ready){let $=Jt.lift,de=H.position.y>40,Me=Math.hypot(H.position.x-$.x,H.position.z-$.z),he=de?$.travel:0;Me<(de?3.6:2.4)&&(de||H.position.y<10)&&F.push({kind:ge.motion||Math.abs(ge.value-he)>.05?"skycall":de?"skydown":"skyup",id:"sky",distance:Me})}for(let $ of Ze)Math.abs(H.position.y-($.floor+2.4))<1.5&&q("telescope",$.id,$.x,$.z,1.7);if(H.position.y<hr)for(let $ of ga)ne.usable($.key)&&q("door",$.key,$.x,$.z,yd*.75);return F.sort(($,de)=>$.distance-de.distance)}function Fe(F,q,$){if(T||!C)return;let de=q?Math.min(F,.05):0;D+=de;let Me=v?.kind==="hover"&&H.position.y>8.3,he=Me||H.position.y>hr?null:Ns(H.position.x,H.position.z);if(K=he?.id||null,z=(Me?null:Us(H.position.x,H.position.z)?.id)||(he&&!he.open?he.id:null)||null,rt+=F,rt>.5){rt=0;for(let[le,qe]of ce){let Xe=H.position.distanceTo(qe.node.position),it=y==="low"||Xe>55?2:y==="medium"||Xe>32?1:0;B(le,qe,it)}}e.traffic||A.begin();for(let le of On)Math.hypot(H.position.x-le.x,H.position.z-le.z)<32&&Ue(le);a.forEach(le=>{le.getRoot().visible&&le.update(de)});for(let le of h.values()){let qe=It.damp(le.value,le.open?1:0,5,Math.min(F,.1));Math.abs(qe-(le.open?1:0))<.001&&(qe=le.open?1:0);let Xe=it=>[-1,1].map(zt=>({owner:le.owner,x:le.node.position.x,z:le.node.position.z,min:[zt*.8+zt*1.6*it-.8,bt.room,-.13],max:[zt*.8+zt*1.6*it+.8,bt.room+3.6,.13]}));(le.open||Xe(qe).every(it=>A.solidClear(it)))&&(le.value=qe),Xe(le.value).forEach((it,zt)=>A.solid(le.owner+":panel:"+zt,it)),le.action.time=le.value*le.action.getClip().duration,le.mixer.update(0)}ge.ready&&Y(F,de),v?.kind==="scope"&&we(F);for(let[le,qe]of c)if(qe.liftAction&&(qe.liftValue=It.damp(qe.liftValue,qe.liftTarget,1.8,Math.min(F,.1)),Math.abs(qe.liftValue-qe.liftTarget)<.02&&(qe.liftValue=qe.liftTarget),qe.liftAction.time=qe.liftValue/4*qe.liftAction.getClip().duration,qe.lift.mixer.update(0),v?.kind==="lift"&&v.id===le)){let Xe=On.find(it=>it.id===le);H.position.set(Xe.liftX,2.4+bt.room+qe.liftValue,Xe.liftZ),qe.liftValue===qe.liftTarget&&(v=null,e.onRide?.(null))}Oe.update(de,q),me.update(de,q);let Pe=[];for(let le=0;le<o.length;le++){let qe=A.body("resident-"+le);qe?.enabled&&Pe.push(qe)}for(let le of ne.update(Math.min(F,.1),$==="street"&&!v?H.position:null,Pe,!q))Math.hypot(H.position.x-le.x,H.position.z-le.z)<20&&je("door",{x:le.x,y:.88+1.5,z:le.z},le.key);if(l.forEach((le,qe)=>{if(le.node.visible=!!le.body&&qe<(y==="low"?1:2),le.body&&(le.node.visible&&!le.body.enabled&&!A.clear({...le.body,enabled:!0},le.body,le.body)&&(le.node.visible=!1),le.body.enabled=le.node.visible),!le.node.visible)return;let Xe={u:le.u,dwell:le.dwell,stop:le.stop},it=!1;if(le.dwell>0)le.dwell-=de,le.speed=0;else{let zt=en=>(en.u-le.u+1)%1*Q,Bt=vt.reduce((en,Pn)=>zt(Pn)>1e-5&&zt(Pn)<zt(en)?Pn:en,vt.find(en=>zt(en)>1e-5)||vt[0]),pn=Math.min(11,Math.sqrt(zt(Bt)*8));for(let en=1;en<=3;en++){let Pn=(le.u+(1.2+le.speed*.7)*en/3/Q)%1;at.getPointAt(Pn,V),at.getTangentAt(Pn,W);let En={x:V.x,y:bt.road+.03,z:V.z,heading:Math.atan2(W.x,W.z)};if(!A.clear(le.body,En,En)){pn=0;break}}le.speed+=It.clamp(pn-le.speed,-de*8,de*3.5);let ii=de*le.speed/Q,Kn=le.u;le.u=(le.u+ii)%1;let Zt=vt.findIndex(en=>(en.u-Kn+1)%1>0&&(en.u-Kn+1)%1<=ii+1e-5);Zt>=0&&(le.stop=Zt,le.u=vt[Zt].u,le.dwell=6,le.speed=0,it=!0)}if(at.getPointAt(le.u,le.node.position),le.node.position.y=bt.road+.03,at.getTangentAt(le.u,W),le.node.rotation.y=Math.atan2(W.x,W.z),le.body&&(le.body.enabled=le.node.visible,A.propose(le.body,{...le.node.position,heading:le.node.rotation.y},(zt,Bt)=>{zt||(Object.assign(le,Xe),le.speed=0),le.node.position.set(Bt.x,Bt.y,Bt.z),le.node.rotation.y=Bt.heading,zt&&it&&je("tram",le.node.position)})),le.door.time=(le.dwell>1?1:0)*le.door.getClip().duration,le.mixer.update(0),v?.kind==="waiting"&&vt[le.stop].id===v.station&&le.dwell>1&&(v={kind:"tram",index:qe,station:v.station,offset:0},e.onRide?.("tram")),v?.kind==="tram"&&v.index===qe){if(le.dwell>1&&(v.station=vt[le.stop].id,v.exitRequested)){Ye();return}H.position.copy(le.node.position).addScaledVector(W,v.offset),H.position.y+=.9+1.4,H.lookAt(le.node.position.x+W.x*16,H.position.y,le.node.position.z+W.z*16)}}),v?.kind==="drone"){v.elapsed+=Math.min(F,.1);let le=v.elapsed/40*Math.PI*2;H.position.set(Math.cos(le)*100,45+Math.sin(le*2)*12,Math.sin(le)*95-10),H.lookAt(0,22,-12),v.elapsed>=40&&Ye()}if(I=$==="street"&&et()[0]||null,$==="street"&&!v){let le=H.position.distanceTo(ie);le<3&&(P+=le),P>1.8&&(je(z?"step_inside":"step",H.position),P=0)}ie.copy(H.position),e.onEnvironment?.(!!z);for(let[le,qe]of f.entries()){if(qe.elapsed+=de,qe.node.visible=q&&qe.elapsed<8,!qe.node.visible){qe.body&&A.remove(qe.body.id),qe.body=null;continue}let Xe=qe.direction>0?qe.elapsed/8:1-qe.elapsed/8,it={x:40+Xe*3,y:bt.room,z:70,heading:0};if(qe.body||(qe.body=A.register("freight-"+le,cs(qe.bounds),it,0)),!qe.body){qe.elapsed=9,qe.node.visible=!1;continue}A.propose(qe.body,it,(zt,Bt)=>{zt||(qe.elapsed-=de),qe.node.position.set(Bt.x,Bt.y,Bt.z)})}e.traffic||(A.solve(de),Oe.settle(de)),e.onInteraction?.(I,N.size,z)}async function Je(F){if(y=F,ne.setTier(F),!C)return;let q=F==="low"?2:F==="medium"?1:0;Oe.setTier(F),me.setTier(F);try{let $=new Map(await Promise.all([...x].filter(de=>!ce.has(de)).map(async de=>{let Me=await R(de,q),he=new Map;return Me.scene.traverse(Pe=>{Pe.isMesh&&he.set(Pe.name,Pe)}),[de,he]})));if(T||y!==F)return;for(let de of M){let Me=$.get(de.userData.worldAsset)?.get(de.userData.worldPart);Me&&(de.geometry=Me.geometry,de.material=Me.material)}e.onReady?.()}catch($){T||e.onError?.($)}}return{update:Fe,interact:De,endRide:Ye,setTier:Je,society:Oe,lobbies:ne,settle(F){!T&&C&&Oe.settle(F)},suspend(){Oe.suspend(),me.clear(),w=!1;for(let F of f)F.elapsed=9,F.node.visible=!1,F.body&&A.remove(F.body.id),F.body=null},syncRide(){if(v?.kind==="tram"){let F=l[v.index];at.getTangentAt(F.u,W),H.position.copy(F.node.position).addScaledVector(W,v.offset),H.position.y+=2.3,H.lookAt(F.node.position.x+W.x*16,H.position.y,F.node.position.z+W.z*16)}},socialAction(F,q){let $=ui.find(de=>de.id===q);return Oe.action(e.reduced()&&F==="guide"?"cancel":F,$?{x:$.platformX,z:$.platformZ}:null)},setMemory(F){g=(Array.isArray(F)?F:[]).filter(q=>typeof q=="string").slice(0,8),Z()},setWorld(F,q=!1){q?(Oe.setReplay(!0),me.clear()):Oe.setReplay(!1);let $=F?.entities?.filter(Me=>Me.kind==="mission")||[],de=new Set;for(let Me of $){de.add(Me.id);let he=b.get(Me.id);if(!q&&e.active()&&w&&he!=null&&he!==Me.state&&["running","completed","failed","cancelled"].includes(Me.state)&&Date.now()-Me.at<3e4){let Pe=f.find(le=>le.elapsed>=8);Pe&&(Pe.elapsed=0,Pe.direction=Me.state==="running"?1:-1,_++)}b.set(Me.id,Me.state)}for(let Me of b.keys())de.has(Me)||b.delete(Me);if(w=!q,q)for(let Me of f)Me.elapsed=9,Me.node.visible=!1},walkRide(F,q,$=0){if(!v)return!1;if(v.kind==="tram"&&(v.offset=It.clamp(v.offset+F*q*3,-2.5,2.5)),v.kind==="scope"){G.setFromQuaternion(H.quaternion,"YXZ");let de=q*.9*H.fov/v.back.fov;G.x+=F*de,G.y-=$*de,H.quaternion.setFromEuler(G)}return!0},hovering:()=>v?.kind==="hover",fly(F,q){if(v?.kind!=="hover")return!1;H.position.distanceToSquared(Ee)>.01&&Ne.place(H.position),H.getWorldDirection(V);let $=Ne.step(F,q,Math.atan2(-V.x,-V.z));return Ee.set($.x,$.y,$.z),H.position.copy(Ee),He(),!0},zoom(F){return v?.kind!=="scope"?!1:(v.fov=It.clamp(v.fov*Math.exp(F*.0012),5,26),!0)},scoping:()=>v?.kind==="scope",move(F,q,$){if(v)return!1;if($.y>40)return om(F,q,!ge.motion&&ge.value===Jt.lift.travel);let de=Us($.x,$.z);return de&&$.y>5?Math.abs(q-de.z-4.8)<.85&&(Math.abs(F-de.x)<1.1||Math.abs(F-de.x+4)<1.75)?!1:bd(de,F,q,c.get(de.id)?.liftValue===4):de&&$.y<5&&([-5.6,1.9].some(Me=>Math.abs(F-de.x-Me)<.25&&Math.abs(q-de.z-.15)<.25)||cm(de,F,q)&&!(F<de.liftX+1.25&&Math.abs(q-de.liftZ)<1.25&&c.get(de.id)?.liftValue===0)||Math.hypot(F-(de.x-1),q-de.z)<1.55||[-3,0].some(Me=>Math.abs(F-de.x-Me)<1.1&&Math.abs(q-de.z-3)<.8))?!1:Ed(F,q,$,Me=>Me.startsWith("lobby:")?ne.passable(Me):h.get(Me)?.value>.85&&!!c.get(Me)?.ready,e.districts)},floor(F,q){if(H.position.y>40)return Jt.floor;let $=Us(F,q);return $&&H.position.y>5&&bd($,F,q,c.get($.id)?.liftValue===4)?bt.gallery:re(F,q)},visit(F){Ye(),v=null;let q=us.find(de=>de.id===F);if(q){let de=C?.assets.find(Me=>Me.id===F)?.navigation.interaction[0]?.position||[0,1.5,4];H.position.set(q.x,2.4+Dn(q.x,q.z),q.z+de[2]+1),H.lookAt(q.x,2,q.z);return}if(F==="drone"){H.position.set(Wi.x,2.7,Wi.z+3),H.lookAt(Wi.x,2,Wi.z);return}if(F==="skydeck"){let de=57*Math.PI/180,Me=Jt.x+Math.sin(de)*7.2,he=Jt.z-Math.cos(de)*7.2;H.position.set(Me,Jt.floor+2.4,he),H.lookAt(Me,Jt.floor+.4,he-10);return}let $=ui.find(de=>de.id===F);if($){let de=N.has(F)?0:5,Me=$.platformX+($.angle===0?de:0),he=$.platformZ+($.angle===0?0:de);H.position.set(Me,2.4+Dn(Me,he),he),H.lookAt($.platformX,2,$.platformZ===he?$.platformZ+1:$.platformZ)}},destination(F){let q=On.find($=>$.id===F);if(q){Ye(),v=null;let $=q.doorZ+q.front*4;H.position.set(q.x,2.4+Dn(q.x,$),$),H.lookAt(q.x,2.4+Dn(q.x,$),q.z)}},isRiding:()=>!!v,interaction:()=>I,rideBody:()=>v?.kind==="tram"?"tram-"+v.index:v?.kind==="scope"?v.scope.model.owner:v?.kind==="sky"&&ge.lift?.owner||null,stats:()=>({society:Oe.stats(),machinery:me.stats(),details:[...ce].map(([F,q])=>({id:F,level:q.level})),loaded:[...x],bytes:S,freightEvents:_,residents:o.filter(F=>F.node.visible).length,trams:l.filter(F=>F.node.visible).length,rooms:[...c].filter(([,F])=>F.ready).map(([F])=>F),inside:z,lobby:K,lobbies:ne.stats(),ride:v?.kind||null,station:v?.station||null,discovered:[...N],loading:X,interactions:I?{kind:I.kind,id:I.id}:null,landmarks:d.map(F=>F.id),sky:{ready:ge.ready,value:+ge.value.toFixed(2),moving:!!ge.motion},telescopes:Ze.length,hover:(F=>({pads:te.length,docked:te.filter(q=>q.docked).length,flying:!!F,altitude:F?+F.altitude.toFixed(1):0,speed:F?+(F.speed*3.6).toFixed(1):0,landing:!!F?.landing}))(v?.kind==="hover"?Ne.state():null),scope:v?.kind==="scope"?{id:v.id,target:v.target,fov:+H.fov.toFixed(1)}:null}),dispose(){T||(T=!0,E.abort(),Oe.dispose(),me.dispose(),e.traffic||A.dispose(),g=[],b.clear(),t.removeFromParent(),a.forEach(F=>{F.stopAllAction(),F.uncacheRoot(F.getRoot())}),t.traverse(F=>{F.isInstancedMesh&&F.dispose()}),i.forEach(F=>F.dispose()),r.forEach(F=>F.dispose()),m.forEach(F=>F.dispose()),n.clear())}}}var lw=s=>.5+(s-6)/12*3.62,mg=new U;function gg(s,{sun:e,rim:t,hemisphere:n,atmosphere:i,surfaces:r,onThunder:a}){let o="local",l="clear",c="high",h=!1,d=-100,u=!1,p=n.intensity,x=14,M=-1,m=0,f={value:0},b=new At,E=[],g=317,_=()=>(g=Math.imul(g,1664525)+1013904223>>>0)/4294967296;for(let P=0;P<1200;P++)E.push((_()-.5)*170,_()*60,(_()-.5)*174-7);b.setAttribute("position",new Mt(E,3));let w=new Rt({transparent:!0,depthWrite:!1,uniforms:{time:f,flash:{value:0}},vertexShader:"uniform float time;varying float vFade;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;gl_PointSize=clamp(260.0/max(1.0,-mv.z),2.0,26.0);vFade=smoothstep(0.0,4.0,p.y)*smoothstep(460.0,60.0,-mv.z);}",fragmentShader:"uniform float flash;varying float vFade;void main(){vec2 c=gl_PointCoord-.5;float a=(1.0-smoothstep(0.0,0.07,abs(c.x)))*(1.0-smoothstep(0.2,0.5,abs(c.y)))*.42*vFade;if(a<.004)discard;gl_FragColor=vec4(mix(vec3(.6,.78,.86),vec3(.95,.97,1.0),flash),a*(1.0+flash));}"}),T=new Rn(b,w);T.frustumCulled=!1,s.add(T);let y=new Le(9417679),C=new Le(528926),S=new Le(2762296);function D(){let P=hh(o),N=P.amount,z=lw(P.hour);e.intensity=.8+N*2.6,e.color.set(P.evening>.1?16757370:13164287),e.position.set(Math.cos(P.hour/24*Math.PI*2)*120,40+N*115,85),mg.set(Math.cos(z)*120,40-P.evening*18+N*115,Math.sin(z)*120),t.intensity=.7+P.evening*1.6,p=n.intensity=.45+N*.45,s.fog.color.copy(C).lerp(y,N).lerp(S,P.evening*(1-N)*.5),s.fog.density=l==="fog"?.018:l==="rain"?.008:.003-N*.0016,i.setLighting?.(N,P.evening,mg),i.setWeather?.(l),r?.setLighting(N,P.evening,l==="clear"?0:1,s.fog.color),r?.setWeather(l),T.visible=l==="rain"&&!h,b.setDrawRange(0,c==="low"?200:1200)}function v(P,N){if(l!=="rain"||h||!N){M>=0&&(M=-1,I(0));return}if(x-=P,M<0&&x<=0&&(M=0,m++,x=9+_()*16,a?.(.6+_()*3.2)),M<0)return;M+=P;let z=M<.09?1:M<.16?.25:M<.24?.75:Math.max(0,1-(M-.24)/.35)*.5;I(z),M>.6&&(M=-1,I(0))}function I(P){n.intensity=p+P*2.4,w.uniforms.flash.value=P,i.setFlash?.(P)}return D(),{set(P={}){["local","day","evening","night"].includes(P.time)&&(o=P.time),["clear","rain","fog"].includes(P.weather)&&(l=P.weather),D()},setTier(P){c=P,D()},setIndoor(P){h!==P&&(h=P,D())},update(P,N,z){if(!u)return T.visible=l==="rain"&&!h&&z,z&&(f.value+=Math.min(P,.1)),v(Math.min(P,.1),z),N-d>30?(d=N,D(),!0):!1},mood(){let P=hh(o);return{day:P.amount,evening:P.evening,weather:l,indoor:h}},stats:()=>({time:o,weather:l,indoor:h,daylight:hh(o).amount,strikes:m}),dispose(){u=!0,T.removeFromParent(),b.dispose(),w.dispose()}}}var Os={cx:30,cz:69.5,rx:4.5,rz:2.5,seconds:16,gaitHz:1.7},cw=["body","fl_hip","fr_hip","hl_hip","hr_hip","fl_uleg","fr_uleg","hl_uleg","hr_uleg","fl_lleg","fr_lleg","hl_lleg","hr_lleg"],hw={fl:0,hr:0,fr:Math.PI,hl:Math.PI};function uw(s){let e=s/Os.seconds*Math.PI*2,t=Os.cx+Os.rx*Math.cos(e),n=Os.cz+Os.rz*Math.sin(e),i=-Os.rx*Math.sin(e),r=Os.rz*Math.cos(e);return{x:t,z:n,heading:Math.atan2(-r,i),phase:s*Os.gaitHz*Math.PI*2}}function dw(s){let e={};for(let[t,n]of Object.entries(hw))e[t]={hip:.26*Math.sin(s+n),uleg:.13*Math.sin(s+n+.9)};return e}function xg({scene:s,resourceURL:e,signal:t,ground:n=Dn,shadows:i=null,onError:r}){let a=new Et;a.visible=!1,s.add(a);let o=Object.create(null),l=new un,c={y:NaN,v:0},h={y:0,v:0},d=i?.claim()||null,u=null,p=0,x=0,M=!1,m=!1,f=new Ri;f.setMeshoptDecoder(ca);let b=(async()=>{try{await ca.ready;let g=await fetch(e("/3d/system-world/spot.glb"),{signal:t});if(!g.ok)throw Error("Companion asset unavailable: "+g.status);let _=await g.arrayBuffer();if(m)return!1;let w=await f.parseAsync(_,"");if(m)return!1;u=w.scene,u.traverse(T=>{T.isMesh&&(T.castShadow=!0,T.receiveShadow=!0)});for(let T of cw)o[T]=u.getObjectByName(T)||null;if(!o.body)throw Error("Companion joint hierarchy missing");return l.setFromObject(u),p=-l.min.y,a.add(u),a.visible=!0,!0}catch(g){return M=!0,r?.(g),!1}})();function E(g,_){if(!u||m)return;let w=_?Math.min(.1,Math.max(0,g)):0;x+=w;let T=uw(x),y=Math.cos(T.heading),C=-Math.sin(T.heading),S=.55,D=n(T.x,T.z),v=zs(c,D,w),I=n(T.x+y*S,T.z+C*S)-n(T.x-y*S,T.z-C*S),P=zs(h,Math.atan2(I,2*S),w);a.position.set(T.x,v+p+Math.abs(Math.sin(T.phase))*.012,T.z),a.rotation.set(0,T.heading,P,"YZX"),d?.set(T.x,D,T.z,.6,.45);for(let[N,z]of Object.entries(dw(T.phase)))o[N+"_hip"]&&(o[N+"_hip"].rotation.y=z.hip),o[N+"_uleg"]&&(o[N+"_uleg"].rotation.y=z.uleg)}return{ready:b,update:E,stats:()=>({loaded:!!u,failed:M,time:+x.toFixed(2),joints:Object.keys(o).filter(g=>o[g]).length,position:[+a.position.x.toFixed(2),+a.position.z.toFixed(2)]}),dispose(){m||(m=!0,d?.hide(),u&&u.traverse(g=>{if(g.isMesh){g.geometry.dispose();for(let _ of Array.isArray(g.material)?g.material:[g.material])_.dispose()}}),a.removeFromParent(),a.clear())}}}function bg(s,{capacity:e=40}={}){let t=new dn(1,1);t.rotateX(-Math.PI/2);let n=new yi(new Float32Array(e),1);t.setAttribute("swShadow",n);let i=new Rt({transparent:!0,depthWrite:!1,polygonOffset:!0,polygonOffsetFactor:-2,polygonOffsetUnits:-2,vertexShader:`attribute float swShadow;varying vec2 vUv;varying float vA;void main(){vUv=uv;vA=swShadow;vec4 p=vec4(position,1.0);
#ifdef USE_INSTANCING
p=instanceMatrix*p;
#endif
gl_Position=projectionMatrix*modelViewMatrix*p;}`,fragmentShader:"varying vec2 vUv;varying float vA;void main(){float r=length(vUv-.5)*2.;gl_FragColor=vec4(0.,0.,0.,vA*(1.-smoothstep(.1,1.,r)));}"}),r=new An(t,i,e),a=new yt;r.name="contact-shadows",r.frustumCulled=!1,r.renderOrder=1,r.castShadow=!1,r.receiveShadow=!1;for(let l=0;l<e;l++)r.setMatrixAt(l,a.makeScale(0,0,0));s.add(r);let o=0;return{claim(){if(o>=e)return null;let l=o++;return{set(c,h,d,u,p){r.setMatrixAt(l,a.makeScale(u*2,1,u*2).setPosition(c,h+.015,d)),n.setX(l,p),r.instanceMatrix.needsUpdate=!0,n.needsUpdate=!0},hide(){r.setMatrixAt(l,a.makeScale(0,0,0)),n.setX(l,0),r.instanceMatrix.needsUpdate=!0,n.needsUpdate=!0}}},stats:()=>({used:o,capacity:e}),dispose(){r.removeFromParent(),r.dispose(),t.dispose(),i.dispose()}}}function vg(...s){let e=new Map;for(let t of s)for(let[n,i]of t instanceof Map?t:(t?.assets||[]).map(r=>[r.id,r]))e.set(n,i);return e}function yg(s,e,t,n){for(let i of t){let r=e.get(i.asset).lods[0].bounds;(wd[i.id]||(wm.has(i.id)?[]:[[-3,-3,3,3]])).forEach(([o,l,c,h],d)=>s.solid("district:"+i.id+":"+d,{x:i.x,z:i.z,min:[o,0,l],max:[c,8,h]})),Tm(i.id).forEach((o,l)=>s.solid("district:"+i.id+":lobby:"+l,o));for(let o of new Set(fi.filter(l=>l.district===i.id).map(l=>l.asset)))(e.get(o)?.navigation?.colliders||[]).forEach((l,c)=>s.solid("district:"+i.id+":"+o+":"+c,{x:i.x,z:i.z,min:[l[0],l[1],l[2]],max:[l[3],l[4],l[5]]}));Sd[i.id]?Sd[i.id].forEach(([o,l,c],h)=>s.solid("district:"+i.id+":upper:"+h,{x:i.x,z:i.z,min:[-o,l,-o],max:[o,c,o]})):s.solid("district:"+i.id+":upper",{x:i.x,z:i.z,min:[r.min[0],8,r.min[2]],max:r.max})}n.forEach((i,r)=>{if(i.asset==="street-tile"||i.asset==="street-crossing")return;let a=e.get(i.asset)?.lods[0].bounds;if(!a)return;let o=a.min.map((c,h)=>c*i.scale[h]),l=a.max.map((c,h)=>c*i.scale[h]);o[1]+=i.y,l[1]+=i.y,i.asset==="street-lamp"&&(s.solid("city:"+r+":pole",{x:i.x,z:i.z,min:[-.2,i.y,-.2],max:[.2,l[1],.2]}),o[1]=Math.max(o[1],l[1]-1)),s.solid("city:"+r,{x:i.x,z:i.z,heading:i.angle,min:o,max:l})})}var Bo={"city.road":{top:"asphalt",side:"concrete",albedo:.55,rough:.7,bump:.012,wet:1,puddle:[.447,.442]},"city.stone":{top:"pavers",side:"concrete",albedo:.5,rough:.6,bump:.014,wet:1,puddle:[.523,.516]},"city.graphite":{top:"gravel",side:"panels",albedo:.42,rough:.5,bump:.016,wet:.8,metalTop:.3},"city.titanium":{top:"panels",side:"panels",albedo:.2,rough:.45,bump:.006,wet:.6},"city.bronze":{top:"panels",side:"panels",albedo:.18,rough:.4,bump:.006,wet:.6},"city.robot-rubber":{top:"panels",side:"panels",albedo:.3,rough:.75,bump:.008,wet:.25},"city.ceramic":{top:"concrete",side:"concrete",albedo:.3,rough:.4,bump:.008,wet:.8,puddle:[.464,.46]},"city.leaf":{top:"foliage",side:"foliage",albedo:.7,rough:.3,bump:.03,wet:.5},"city.kerb":{top:"concrete",side:"concrete",albedo:.45,rough:.55,bump:.01,wet:1},"city.setts":{top:"pavers",side:"concrete",albedo:.5,rough:.6,bump:.014,wet:1,topScale:2,puddle:[.523,.516]},"city.iron":{top:"panels",side:"panels",albedo:.25,rough:.45,bump:.008,wet:.9},"city.marking":{top:"asphalt",side:"concrete",albedo:.25,rough:.5,bump:.006,wet:1,puddle:[.447,.442]},"city.tactile":{top:"concrete",side:"concrete",albedo:.4,rough:.6,bump:.02,wet:1},"city.glass":{top:"panels",side:"panels",albedo:.15,rough:.3,bump:.002,wet:.6},ground:{top:"pavers",side:"concrete",albedo:.55,rough:.6,bump:.014,wet:1,topScale:.5,puddle:[.523,.516],waterline:!0}},_g=new Set(["city.ivory","city.warm"]),fw="city.window",pw={low:0,medium:1,high:2,ultra:2},dr=s=>Number(s).toFixed(4),mw=`varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
attribute float swSeed;varying float vSwSeed;
#endif`,gw=`
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
#endif`,xw=`uniform float swWet;uniform float swRain;uniform float swTime;uniform vec4 swWindow;uniform vec2 swFade;uniform vec4 swShelter[3];uniform vec3 swSky;
varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
varying float vSwSeed;
#endif
#ifdef SW_SURFACE
uniform sampler2D swTop;uniform sampler2D swSide;uniform vec2 swScale;
float swHash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
vec3 swPerturb(vec3 pos,vec3 n,vec2 dh,float face){vec3 sx=dFdx(pos),sy=dFdy(pos),r1=cross(sy,n),r2=cross(n,sx);float det=dot(sx,r1)*face;return normalize(abs(det)*n-sign(det)*(dh.x*r1+dh.y*r2));}
#endif`,bw=`
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
#endif`,vw=`
#ifdef SW_SURFACE
diffuseColor.rgb*=swAlbedoF*mix(1.0,0.56,swWetness)*mix(1.0,0.7,swPuddle);
#endif`,yw=`
#ifdef SW_SURFACE
roughnessFactor=clamp(roughnessFactor*swRoughF,0.04,1.0);roughnessFactor=mix(roughnessFactor,roughnessFactor*0.6,swWetness);roughnessFactor=mix(roughnessFactor,0.04,swPuddle);
#endif`,_w=`
#ifdef SW_SURFACE
metalnessFactor*=mix(1.0,SW_METAL_TOP,swW.y);
#endif`,Mw=`
#if SW_LEVEL>1
normal=swPerturb(-vViewPosition,normal,vec2(dFdx(swH),dFdy(swH))*SW_BUMP*swDetail,faceDirection);
#endif`,ww=`
#ifdef SW_SURFACE
float swFresnel=pow(1.0-clamp(dot(normal,normalize(vViewPosition)),0.0,1.0),5.0)*0.95+0.05;
totalEmissiveRadiance+=swSky*swFresnel*(swPuddle*0.6+swWetness*0.1);
#endif
#ifdef SW_WINDOWS
float swPane=(1.0-smoothstep(0.35,0.65,abs(swN.y)))*(1.0-step(1.5,vSwSeed));
totalEmissiveRadiance*=mix(swWindow.w,mix(swWindow.z,swWindow.y,step(vSwSeed,swWindow.x)),swPane);
#endif`;function Sw(s,e=!1){let t=s?.getAttribute("position");if(!t||s.getAttribute("swSeed"))return;if(e){s.setAttribute("swSeed",new Pt(new Float32Array(t.count).fill(2),1));return}let n=t.count,i=new Int32Array(n),r=s.getIndex();for(let u=0;u<n;u++)i[u]=u;let a=u=>{for(;i[u]!==u;)i[u]=i[i[u]],u=i[u];return u},o=u=>r?r.getX(u):u,l=r?r.count:n;for(let u=0;u+2<l;u+=3){let p=a(o(u));i[a(o(u+1))]=p,i[a(o(u+2))]=p}let c=new Map,h=new Map;for(let u=0;u<n;u++){let p=a(u),x=h.get(p)||[0,0,0];c.set(p,(c.get(p)||0)+1),x[0]+=t.getX(u),x[1]+=t.getY(u),x[2]+=t.getZ(u),h.set(p,x)}let d=new Float32Array(n);for(let u=0;u<n;u++){let p=a(u),x=c.get(p),M=h.get(p);if(x>6){d[u]=2;continue}let m=Math.sin(M[0]/x*12.9898+M[1]/x*78.233+M[2]/x*37.719)*43758.5453;d[u]=m-Math.floor(m)}s.setAttribute("swSeed",new Pt(d,1))}function Mg(s,e={}){let t=new AbortController,n=new Map,i=[],r=new _s(new Uint8Array([128,128,128,255]),1,1);r.colorSpace=ci,r.needsUpdate=!0;let a={},o={},l={};for(let v of Object.values(Bo))for(let I of[v.top,v.side])a[I]||={value:r};for(let v of Object.keys(Bo))l[v]={value:new Ge(1/4,1/4)};let c=(e.shelters||[]).slice(0,3).map(v=>new Ot(...v));for(;c.length<3;)c.push(new Ot(1e6,1e6,-1e6,-1e6));let h={swWet:{value:0},swRain:{value:0},swTime:{value:0},swShelter:{value:c},swSky:{value:new Le(528926)},swWindow:{value:new Ot(.8,1,.06,1)},swFade:{value:new Ge(35,140)}},d=2,u=null,p=!1,x=0,M=0,m=0,f=0,b=!1,E=s.capabilities.getMaxAnisotropy(),g=()=>d>1?Math.min(8,E):Math.min(4,E);function _(v,I){let{kind:P,windows:N,space:z}=n.get(I),X=Bo[P];Object.assign(v.uniforms,h);let j=["#define SW_LEVEL "+(X?d:0)];z==="world"&&j.push("#define SW_WORLD"),N&&j.push("#define SW_WINDOWS"),X&&(v.uniforms.swTop=a[X.top],v.uniforms.swSide=a[X.side],v.uniforms.swScale=l[P],j.push("#define SW_SURFACE","#define SW_ALBEDO "+dr(X.albedo),"#define SW_ROUGH "+dr(X.rough),"#define SW_BUMP "+dr(X.bump),"#define SW_WETK "+dr(z==="world"?X.wet:X.wet*.5),"#define SW_METAL_TOP "+dr(X.metalTop??1),"#define SW_PUDDLE_DRY "+dr(X.puddle?.[0]??-1),"#define SW_PUDDLE_FULL "+dr(X.puddle?.[1]??-2)),X.waterline&&j.push("#define SW_WATERLINE"));let V=j.join(`
`)+`
`;v.vertexShader=V+v.vertexShader.replace("#include <common>",`#include <common>
`+mw).replace("#include <begin_vertex>","#include <begin_vertex>"+gw),v.fragmentShader=V+v.fragmentShader.replace("#include <common>",`#include <common>
`+xw).replace("#include <map_fragment>","#include <map_fragment>"+bw).replace("#include <color_fragment>","#include <color_fragment>"+vw).replace("#include <roughnessmap_fragment>","#include <roughnessmap_fragment>"+yw).replace("#include <metalnessmap_fragment>","#include <metalnessmap_fragment>"+_w).replace("#include <normal_fragment_maps>","#include <normal_fragment_maps>"+Mw).replace("#include <emissivemap_fragment>","#include <emissivemap_fragment>"+ww)}function w(v,I="world",P=v?.name){if(!v?.isMeshStandardMaterial||n.has(v)||v.name===fw)return;let N=Bo[P]?P:"",z=_g.has(v.name);!N&&!z||(n.set(v,{kind:N,windows:z,space:I}),v.addEventListener("dispose",()=>n.delete(v)),v.onBeforeCompile=X=>_(X,v),v.customProgramCacheKey=()=>["sw",N,z,I,N?d:0].join("|"),v.needsUpdate=!0)}function T(v,I=!1){if(v?.isMesh)for(let P of Array.isArray(v.material)?v.material:[v.material])w(P,I?"object":"world"),_g.has(P?.name)&&Sw(v.geometry,!!P.vertexColors)}let y=v=>e.url?.(v),C=null;async function S(v){let I=await fetch(y(v.file),{signal:t.signal});if(!I.ok)throw Error("Surface texture unavailable");let P=await I.blob();if(p)return;let N=await createImageBitmap(P,{premultiplyAlpha:"none",colorSpaceConversion:"none"});if(p){N.close?.();return}let z=new hn(N);z.wrapS=z.wrapT=Di,z.colorSpace=ci,z.flipY=!1,z.anisotropy=g(),z.needsUpdate=!0,i.push(z),a[v.id].value=z,m++,x+=P.size,e.onProgress?.(x,M)}function D(){return u||p||!e.url||(u=(async()=>{if(!C){let P=await fetch(y("manifest.json"),{signal:t.signal});if(!P.ok)throw Error("Surface manifest unavailable");C=await P.json();let N=(C.textures||[]).filter(z=>a[z.id]&&/^[a-z-]+\.webp$/.test(z.file)&&z.tile_metres>0);C.entries=N,M=N.reduce((z,X)=>z+(X.bytes||0),0);for(let z of N)o[z.id]=z.tile_metres;for(let[z,X]of Object.entries(Bo))l[z].value.set(1/((o[X.top]||4)/(X.topScale||1)),1/(o[X.side]||4))}let v=C.entries.filter(P=>a[P.id].value===r),I=await Promise.allSettled(v.map(S));f+=I.filter(P=>P.status==="rejected").length})().catch(()=>{f++}).finally(()=>{u=null})),u}return{apply:w,prepare:T,setTier(v){let I=pw[v]??2;if(I!==d){d=I;for(let[P,N]of n)N.kind&&(P.needsUpdate=!0)}for(let P of i)P.anisotropy!==g()&&(P.anisotropy=g(),P.needsUpdate=!0);d>0&&D()},setLighting(v,I,P=0,N=null){N&&h.swSky.value.copy(N).multiplyScalar(1.25);let z=1-v,X=It.smoothstep(z,.1,.9);h.swWindow.value.set(Math.min(.9,.12+.72*X+.12*P),.22+z*.98+Math.min(1,I)*.1,.05,.55+.45*z)},setWeather(v){b=v==="rain",h.swRain.value=b?1:0},update(v,I){let P=b?1:0,N=h.swWet.value;if(!I){h.swWet.value=P;return}let z=Math.min(.1,Math.max(0,v));h.swTime.value+=z,h.swWet.value=N+(P-N)*Math.min(1,z/(P>N?14:70))},progress:()=>({bytes:x,expected:M}),stats:()=>({detail:d,textures:m,failures:f,bytes:x,materials:n.size,wet:+h.swWet.value.toFixed(2),windows:+h.swWindow.value.x.toFixed(2)}),dispose(){p=!0,t.abort();for(let v of i)v.dispose(),v.source.data?.close?.();i.length=0,r.dispose()}}}var wg="city.window",Ew={low:0,medium:1,high:2,ultra:2},Tw=s=>Ew[s]??2,fh=s=>s>0?s<1?s:1:0,Aw=(s,e,t)=>{let n=fh((s-e)/(t-e));return n*n*(3-2*n)};function Rw(s,e,t=0){let n=fh(s),i=Aw(1-n,.1,.9);return{share:Math.min(.9,.12+.38*i+.1*fh(e)*i+.08*fh(t)),ambient:.012+.1*n,lobby:i,glow:.6+.9*i}}var ba={vertexPars:"varying vec3 vWinPos;varying vec3 vWinNrm;varying vec2 vWinRoom;varying float vWinSeed;",vertexMain:`
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
totalEmissiveRadiance+=winGlow;`};function Sg(){let s={winTime:{value:0},winState:{value:new Ot(.12,.112,0,.6)},winSky:{value:new Le(528926)}},e=2,t="high",n=!1,i=0,r=!1,a=new vn({name:wg,color:new Le(.018,.032,.045),roughness:.08,metalness:0,vertexColors:!0});return a.onBeforeCompile=o=>{Object.assign(o.uniforms,s);let l=`#define WIN_LEVEL ${e}
#define WIN_DEPTH 3.0
#define WIN_BAY 1.9
#define WIN_STOREY 3.05
`;o.vertexShader=l+o.vertexShader.replace("#include <common>",`#include <common>
`+ba.vertexPars).replace("#include <begin_vertex>","#include <begin_vertex>"+ba.vertexMain),o.fragmentShader=l+o.fragmentShader.replace("#include <common>",`#include <common>
`+ba.fragmentPars).replace("#include <color_fragment>","#include <color_fragment>"+ba.fragmentMain).replace("#include <roughnessmap_fragment>","#include <roughnessmap_fragment>"+ba.roughness).replace("#include <emissivemap_fragment>","#include <emissivemap_fragment>"+ba.emissive)},a.customProgramCacheKey=()=>"sw-windows|"+e,{material:a,uniforms:s,owns:o=>o===a,adopt(o){if(!o?.isMesh||r)return!1;let l=Array.isArray(o.material)?o.material:[o.material],c=!1,h=l.map(d=>d===a||d?.name!==wg?d:(c=!0,d.dispose(),a));return c?(o.material=Array.isArray(o.material)?h:h[0],i++,!0):!1},setLighting(o,l,c=0,h=null){let d=Rw(o,l,c);s.winState.value.set(d.share,d.ambient,d.lobby,d.glow),h&&s.winSky.value.copy(h)},setTier(o){t=o;let l=Tw(o);l!==e&&(e=l,a.needsUpdate=!0)},setReducedMotion(o){n=!!o},update(o,l){!l||n||r||(s.winTime.value=(s.winTime.value+Math.min(.1,Math.max(0,o||0)))%7200)},stats:()=>({level:e,tier:t,reduced:n,meshes:i,share:+s.winState.value.x.toFixed(2),lobby:+s.winState.value.z.toFixed(2),glow:+s.winState.value.w.toFixed(2),time:+s.winTime.value.toFixed(1)}),dispose(){r=!0,a.dispose()}}}var Eg="city.pane",Cw=new Set(["city.ivory","city.warm","city.cyan","city.jade"]),Tg={glow:.28,opacity:.26},ph={glow:1,opacity:.2},Pw={medium:1,high:0,ultra:0},Iw=/^door_(\d+)_([lr])$/,Dw=s=>Pw[s]??null;function Ag({group:s,load:e,doors:t=()=>null}){let n=new Et;n.name="lobby-interiors",s.add(n);let i=new vn({name:Eg,color:new Le(.03,.09,.1),roughness:.05,metalness:0,envMapIntensity:2.4,transparent:!0,opacity:ph.opacity,depthWrite:!1,side:Kt}),r=[...new Set(fi.map(p=>p.asset))].map(p=>{let x=fi.filter(M=>M.asset===p);return{asset:p,halls:x,district:x[0].district,origin:x[0].origin,doors:ga.filter(M=>M.asset===p),node:null,level:null,pending:null,failed:!1,leaves:[],kit:[]}}),a=0,o=!1,l=ph.glow,c=p=>{let x=p.clone();return x.onBeforeCompile=p.onBeforeCompile,x.customProgramCacheKey=p.customProgramCacheKey,x.userData.glow=p.emissiveIntensity,x.emissiveIntensity=p.emissiveIntensity*l,x},h=(p,x)=>{for(let M of p.halls)t()?.setReady(M.id,x)};function d(p,x,M){let m=x.clone(!0);m.name=p.asset,m.position.set(p.origin[0],0,p.origin[1]),m.traverse(b=>{let E=Iw.exec(b.name),g=E&&p.doors.find(_=>_.n===Number(E[1]));g&&p.leaves.push({node:b,key:g.key,base:b.position.x,shift:(E[2]==="l"?-1:1)*g.width/2})});let f=new Map;m.traverse(b=>{if(!b.isMesh)return;let g=(Array.isArray(b.material)?b.material:[b.material]).map(_=>{if(!Cw.has(_?.name))return _;if(!f.has(_)){let w=c(_);f.set(_,w),p.kit.push(w)}return f.get(_)});b.material=Array.isArray(b.material)?g:g[0]}),n.add(m),p.node=m,p.level=M,h(p,!0)}function u(p){if(p.node){n.remove(p.node),p.node=null,p.level=null,p.leaves=[];for(let x of p.kit)x.dispose();p.kit=[],h(p,!1)}}return{material:i,owns:p=>p===i,adopt(p){if(!p?.isMesh||o)return!1;let x=Array.isArray(p.material)?p.material:[p.material],M=!1,m=x.map(f=>f===i||f?.name!==Eg?f:(M=!0,f.dispose(),i));return M?(p.material=Array.isArray(p.material)?m:m[0],p.castShadow=!1,!0):!1},setLighting(p){let x=It.smoothstep(Number.isFinite(p)?p:1,0,1),M=(m,f,b=x)=>m+(f-m)*b;i.opacity=M(Tg.opacity,ph.opacity),l=M(Tg.glow,ph.glow,x*x);for(let m of r)for(let f of m.kit)f.emissiveIntensity=f.userData.glow*l},setTier(p){a=Dw(p);for(let x of r)x.failed=!1},update(p,x){if(o)return!1;let M=!1,m=t();for(let f of r){let b=Math.min(...f.doors.map(_=>Math.hypot(p.x-_.x,p.y-.88-1.5,p.z-_.z))),E=a!==null&&(b<=_m||!!f.node&&b<=Mm);if(f.node&&(!E||f.level!==a)&&(u(f),M=!0),E&&!f.node&&!f.failed&&f.pending!==a){let _=a;f.pending=_,e(f.asset,_).then(w=>{f.pending===_&&(f.pending=null,!o&&a===_&&!f.node&&d(f,w,_))},()=>{f.pending===_&&(f.pending=null,f.failed=!0)})}let g=x?.children.find(_=>_.userData.district===f.district)?.getObjectByName("lobby");if(g&&g.visible===!!f.node&&(g.visible=!f.node,M=!0),m)for(let _ of f.leaves)_.node.position.x=_.base+_.shift*m.value(_.key)}return M},stats:()=>({level:a,shown:r.filter(p=>p.node).map(p=>p.asset),loading:r.filter(p=>p.pending!==null).length,failed:r.filter(p=>p.failed).map(p=>p.asset)}),dispose(){if(!o){for(let p of r)u(p);o=!0,n.removeFromParent(),i.dispose()}}}}function Lw(s){let e=Math.max(64,Math.round(s.sampleRate*.03)||1440),t=[],n=0;for(let a=0;a<s.numberOfChannels;a++){let o=s.getChannelData(a);for(let l=0;l<o.length;l+=e){let c=0,h=Math.min(l+e,o.length);for(let d=l;d<h;d++){let u=o[d];if(!Number.isFinite(u))throw Error("Invalid voice");n=Math.max(n,Math.abs(u)),c+=u*u}t.push(Math.sqrt(c/(h-l)))}}if(n<1e-4)throw Error("Silent voice");let i=t.filter(a=>a>Math.max(.003,n*.003)).sort((a,o)=>a-o),r=i[Math.floor((i.length-1)*.7)]||n*.5;return Math.min(8,Math.max(1/n,.26/Math.max(r,.001)))}function Rg(s,e){let t=!1,n=!1,i=0,r=null,a=null,o=[],l=null,c=null,h=null,d=250,u=0,p=0,x=0,M="idle",m=null,f=()=>.04+.46/(1+(d/110)**2);function b(){let y=s.currentTime;l?.gain.setTargetAtTime(f(),y,.18),c?.frequency.setTargetAtTime(1800+4400/(1+d/95),y,.18),h?.pan.setTargetAtTime(u,y,.18)}function E(){if(a){a.onended=null;try{a.stop()}catch{}a=null}o.forEach(y=>y.disconnect()),o=[],l=c=h=null}function g(){p++,clearTimeout(i),i=0,r?.abort(),r=null,E(),M="idle"}function _(y=2e4+Math.random()*2e4){clearTimeout(i),t&&!n&&(i=setTimeout(()=>{i=0,T()},y))}function w(){if(m)return m;m=s.createBuffer(2,Math.ceil(s.sampleRate*.85),s.sampleRate);let y=42;for(let C=0;C<2;C++){let S=m.getChannelData(C),D=Math.round(s.sampleRate*.015),v=0;for(let P=0;P<S.length;P++){y=Math.imul(y,1664525)+1013904223>>>0;let N=(P-D)/(S.length-D);S[P]=P<D?0:(y/4294967296*2-1)*Math.pow(1-N,2.25),v+=S[P]*S[P]}let I=.65/Math.sqrt(v||1);for(let P=0;P<S.length;P++)S[P]*=I}return m}async function T(){if(!t||n||r||a||s.state!=="running"){_();return}let y=p,C=new AbortController;r=C,M="loading";let S=setTimeout(()=>C.abort(),32e3);try{let D=await fetch("/api/desktop/system-world/voice",{method:"POST",credentials:"same-origin",cache:"no-store",signal:C.signal});if(D.status===204){M="idle";return}if(!D.ok||!D.headers.get("Content-Type")?.startsWith("audio/"))throw Error("Voice unavailable");if(Number(D.headers.get("Content-Length"))>8*1024*1024)throw Error("Voice too large");let v=await D.arrayBuffer();if(v.byteLength>8*1024*1024)throw Error("Voice too large");if(y!==p||!t||n)return;let I=await s.decodeAudioData(v);if(y!==p||!t||n)return;if(!Number.isFinite(I.duration)||I.duration<=0||I.duration>25||I.numberOfChannels>2)throw Error("Invalid voice");let P=Lw(I),N=ye=>(o.push(ye),ye);a=N(s.createBufferSource()),a.buffer=I;let z=N(s.createGain()),X=N(s.createBiquadFilter());z.gain.value=P,X.type="highpass",X.frequency.value=90,c=N(s.createBiquadFilter()),c.type="lowpass",c.Q.value=.55;let j=N(s.createGain()),V=N(s.createGain()),W=N(s.createGain()),ie=N(s.createGain());j.gain.value=.76,V.gain.value=.22,W.gain.value=.28;let H=N(s.createDelay(.2)),ge=N(s.createConvolver());H.delayTime.value=.085,ge.normalize=!1,ge.buffer=w();let Ze=N(s.createWaveShaper()),G=new Float32Array(1024);for(let ye=0;ye<G.length;ye++)G[ye]=Math.max(-.75,Math.min(.75,ye*2/(G.length-1)-1));Ze.curve=G,l=N(s.createGain()),l.gain.value=f(),h=N(s.createStereoPanner()),h.pan.value=u,a.connect(z).connect(X).connect(c),c.connect(j).connect(ie),c.connect(H).connect(V).connect(ie),c.connect(ge).connect(W).connect(ie),ie.connect(Ze).connect(l).connect(h).connect(e),b(),M="speaking",x++,a.onended=()=>{y!==p||!t||n||(M="tail",i=setTimeout(()=>{E(),M="idle",_()},900))},a.start()}catch{y===p&&!n&&(M=C.signal.aborted?"idle":"unavailable")}finally{clearTimeout(S),r===C&&(r=null),y===p&&!n&&!a&&_(M==="unavailable"?6e4:void 0)}}return{setActive(y){t===y||n||(t=y,t?_(3e3+Math.random()*2e3):g())},setListener(y,C,S,D,v){if(!Number.isFinite(y)||!Number.isFinite(C)||!Number.isFinite(S)||!Number.isFinite(D)||!Number.isFinite(v))return;let I=-y,P=-12-S,N=Math.max(0,Math.hypot(I,P)-13);d=Math.hypot(N,Math.max(0,C-87,-C));let z=Math.hypot(I,P),X=Math.hypot(D,v);u=z&&X?Math.max(-.8,Math.min(.8,(P*D-I*v)/z/X))*.8:0,b()},stats(){return{state:M,phrases:x,distance:d,gain:f(),pan:u,pending:!!r,scheduled:!!i}},dispose(){n||(n=!0,t=!1,g(),m=null)}}}function Cg(s,e,t=e){let n=new Set,i={x:0,z:0,fx:0,fz:-1},r=!1,a=!1,o="clear",l=s.createBuffer(1,s.sampleRate*2,s.sampleRate),c=31,h=l.getChannelData(0);for(let f=0;f<h.length;f++)c=Math.imul(c,1664525)+1013904223|0,h[f]=(c>>>0)/2147483648-1;let d=s.createBufferSource(),u=s.createBiquadFilter(),p=s.createGain();d.buffer=l,d.loop=!0,u.type="lowpass",u.frequency.value=450,p.gain.value=0,d.connect(u).connect(p).connect(t),d.start();let x=[{x:-57,z:-57,f:94,g:.055},{x:43,z:62,f:143,g:.035},{x:0,z:14,f:220,g:.016},{x:0,z:79,f:340,g:.06}].map((f,b)=>{let E=b===3?s.createBufferSource():s.createOscillator(),g=s.createGain(),_=s.createStereoPanner(),w=s.createBiquadFilter();return b===3?(E.buffer=l,E.loop=!0):(E.type="sine",E.frequency.value=f.f),w.type="lowpass",w.frequency.value=f.f*2,g.gain.value=0,E.connect(w).connect(g).connect(_).connect(t),E.start(),{...f,source:E,level:g,pan:_,low:w}});function M(){p.gain.setTargetAtTime(r?(o==="rain"?.07:.015)*(a?.12:1):0,s.currentTime,.1),u.frequency.setTargetAtTime(a?280:o==="rain"?2100:600,s.currentTime,.2)}function m(){for(let f of[...n]){try{f.source.stop()}catch{}f.release()}}return{setActive(f){if(r=f,M(),!f){m();for(let b of x)b.level.gain.setTargetAtTime(0,s.currentTime,.05)}},environment(f,b){a=f,b&&(o=b),M()},listener(f,b,E,g,_){Object.assign(i,{x:f,z:E,fx:g,fz:_});for(let w of x){let T=Math.hypot(f-w.x,E-w.z);w.level.gain.setTargetAtTime(r?w.g/(1+T*T*.025)*(a?.25:1):0,s.currentTime,.1),w.pan.pan.value=Math.max(-1,Math.min(1,((w.x-f)*-_+(w.z-E)*g)/Math.max(1,T)))}},play(f,b=0,E=0,g=0){if(!r||n.size>=16)return;let _=Math.hypot(b-i.x,g-i.z);if(_>90)return;let w={step:[.12,240,.16],step_inside:[.1,550,.12],door:[.7,850,.06],lift:[1.5,120,.05],tram:[.8,330,.08],discover:[.45,880,.08],hover:[1.2,160,.07]}[f];if(!w)return;let[T,y,C]=w,S=f.startsWith("step")||f==="door"||f==="hover"?s.createBufferSource():s.createOscillator();"buffer"in S?S.buffer=l:(S.type="sine",S.frequency.value=y);let D=s.createBiquadFilter(),v=s.createGain(),I=s.createStereoPanner();D.type="lowpass",D.frequency.value=a?y*.7:y,v.gain.setValueAtTime(0,s.currentTime),v.gain.linearRampToValueAtTime(C/(1+_*.08),s.currentTime+.015),v.gain.exponentialRampToValueAtTime(1e-4,s.currentTime+T),I.pan.value=Math.max(-1,Math.min(1,((b-i.x)*-i.fz+(g-i.z)*i.fx)/Math.max(_,1))),S.connect(D).connect(v).connect(I).connect(e);let P={source:S,release(){if(n.delete(P))for(let N of[S,D,v,I])N.disconnect()}};n.add(P),S.onended=P.release,S.start(),S.stop(s.currentTime+T+.02)},stats:()=>({voices:n.size,weather:o,inside:a}),dispose(){r=!1,m(),d.stop();for(let f of x){f.source.stop();for(let b of[f.source,f.level,f.pan,f.low])b.disconnect()}for(let f of[d,u,p])f.disconnect()}}}function Pg(s,e,t=e){let o=new Set,l=new Map,c=new Map,h={x:0,y:0,z:0,fx:0,fz:-1},d={x:0,z:0},u=!1,p=!1,x=0,M=0,m=0,f={kind:0,inactive:0,distant:0,pool:0,retrigger:0,quiet:0},b=s.createBuffer(1,s.sampleRate*2,s.sampleRate),E=9371,g=b.getChannelData(0);for(let C=0;C<g.length;C++)E=Math.imul(E,1664525)+1013904223|0,g[C]=(E>>>0)/2147483648-1;let _={step:{gain:.07,layers:[{wave:"noise",f:1900,q:1.7,dur:.05,lvl:.5},{wave:"noise",f:520,q:.9,dur:.1,lvl:.4},{wave:"tone",f:104,f2:74,dur:.12,lvl:.3}]},step_carry:{gain:.075,layers:[{wave:"noise",f:1250,q:1.2,dur:.06,lvl:.45},{wave:"noise",f:340,q:.8,dur:.13,lvl:.5},{wave:"tone",f:88,f2:62,dur:.15,lvl:.36}]},glide:{gain:.028,layers:[{wave:"noise",f:700,q:.6,dur:.26,lvl:.8},{wave:"tone",f:150,f2:120,dur:.24,lvl:.18}]},greet:{gain:.05,layers:[{wave:"tone",f:520,f2:740,dur:.11,lvl:.5,type:"triangle"},{wave:"tone",f:790,f2:960,dur:.13,lvl:.3,type:"triangle",delay:.1}]},work:{gain:.045,layers:[{wave:"noise",f:2700,q:2.6,dur:.05,lvl:.6},{wave:"tone",f:240,f2:175,dur:.22,lvl:.26,type:"sawtooth"}]},carry:{gain:.034,layers:[{wave:"noise",f:950,q:.7,dur:.12,lvl:.7},{wave:"noise",f:2300,q:1.5,dur:.07,lvl:.25}]},idle:{gain:.02,layers:[{wave:"noise",f:3400,q:3.2,dur:.028,lvl:.7}]},turn:{gain:.026,layers:[{wave:"tone",f:900,f2:1440,dur:.18,lvl:.4,type:"sawtooth"}]}};function w(C,S,D,v,I){let P=C.startsWith("robot_")?C.slice(6):C,N=_[P];if(!N)return f.kind++,!1;if(!u)return f.inactive++,!1;let z=S-d.x,X=v-d.z,j=Math.hypot(z,X);if(j>48)return f.distant++,!1;if(o.size>=10){let oe=null;for(let re of o)(!oe||re.distance>oe.distance)&&(oe=re);if(!oe||j>=oe.distance)return f.pool++,!1;oe.release()}let V=s.currentTime;if(I!=null){let oe=c.get(I);if(oe!=null&&V-oe<.25)return f.retrigger++,!1;c.set(I,V)}let W=Math.abs(D-h.y),ie=N.gain*4.5/(1+j*.12)/(1+W*.0045)*(p?.5:1);if(ie<.002)return f.quiet++,!1;let H=S-h.x,ge=v-h.z,Ze=Math.max(-1,Math.min(1,(H*-h.fz+ge*h.fx)/Math.max(1,Math.hypot(H,ge)))),G=s.createGain(),ye=s.createStereoPanner(),J=s.createGain();G.gain.value=ie,ye.pan.value=Ze,J.gain.value=p?.14:.26;let A=s.createBiquadFilter();A.type="lowpass",A.Q.value=.6,A.frequency.value=Math.max(700,Math.min(11e3,11e3-j*130)),A.connect(G),G.connect(ye).connect(e),ye.connect(J).connect(t);let me=[A],ce={distance:j,release(){if(o.delete(ce)){for(let oe of me){try{oe.stop?.()}catch{}try{oe.disconnect()}catch{}}for(let oe of[G,ye,J])try{oe.disconnect()}catch{}}}};o.add(ce);let ne=null,K=0;for(let oe of N.layers){let re=V+(oe.delay||0),te=Math.max(oe.dur,(oe.attack||.006)+.02),fe=s.createGain();fe.gain.setValueAtTime(0,re),fe.gain.linearRampToValueAtTime(oe.lvl,re+(oe.attack||.006)),fe.gain.exponentialRampToValueAtTime(1e-4,re+te),fe.connect(A);let Ee;if(oe.wave==="noise"){Ee=s.createBufferSource(),Ee.buffer=b,Ee.loop=!0;let Oe=s.createBiquadFilter();Oe.type="bandpass",Oe.frequency.value=oe.f,Oe.Q.value=oe.q||1,Ee.connect(Oe).connect(fe),me.push(Oe)}else Ee=s.createOscillator(),Ee.type=oe.type||"sine",Ee.frequency.setValueAtTime(oe.f,re),oe.f2&&Ee.frequency.exponentialRampToValueAtTime(Math.max(20,oe.f2),re+te),Ee.connect(fe);me.push(Ee,fe);let Ne=re+te+.02;Ne>K&&(K=Ne,ne=Ee),Ee.start(re),Ee.stop(Ne)}return ne&&(ne.onended=ce.release),x++,ie>m&&(m=ie),!0}function T(C,S,D,v,I,P,N){if(!u)return;let z=l.get(C);if(!z){l.set(C,{x:S,z:v,phase:0});return}let X=S-z.x,j=v-z.z,V=Math.hypot(X,j);if(z.x=S,z.z=v,!I){z.phase=0;return}if(V>8)return;z.phase+=V;let W=N==="glide"?1.6:P?1.05:.8;z.phase<W||(z.phase-=W,w(N==="glide"?"glide":P?"step_carry":"step",S,D,v,C),M++)}function y(){for(let C of[...o])C.release();for(let[,C]of l)C.x=h.x,C.z=h.z,C.phase=0;c.clear()}return{setActive(C){u!==C&&(u=C,C||y())},environment(C){p=!!C},listener(C,S,D,v,I,P,N){Object.assign(h,{x:C,y:S,z:D,fx:v,fz:I}),d.x=P??C,d.z=N??D},action(C,S,D,v,I){return w(C,S,D,v,I)},gait:T,reset(){l.clear(),c.clear()},stats:()=>({voices:o.size,played:x,steps:M,gaits:l.size,inside:p,peak:+m.toFixed(4),rejected:{...f}}),dispose(){u=!1,y()}}}var Ud=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]],Ig=[440,523.25,587.33,659.25,783.99,880,1046.5],ds=12,mh={harbour:{x:0,z:79},quay:{x:0,z:72},tower:{x:0,z:-12},square:{x:8,z:40}},Fw=[{kind:"chime",weight:15},{kind:"swoosh",weight:13},{kind:"gust",weight:7},{kind:"birds",weight:8,when:s=>s.day>.45&&s.weather==="clear"},{kind:"gull",weight:6,when:s=>s.day>.2&&s.weather==="clear"},{kind:"pigeon",weight:4,when:s=>s.day>.6&&!s.busy},{kind:"cricket",weight:6,when:s=>s.day<.25&&s.weather!=="rain"},{kind:"horn",weight:4,when:s=>s.day<.45&&s.weather!=="rain"},{kind:"bell",weight:3,when:s=>s.day<.7},{kind:"hammer",weight:4,when:s=>s.day>.3},{kind:"drip",weight:10,when:s=>s.weather==="rain"},{kind:"windchime",weight:4,when:s=>s.weather==="clear"&&!s.busy},{kind:"siren",weight:2,when:s=>s.evening>.3&&s.busy}];function Nw(s,e,t){let n=Math.floor(s.sampleRate*e),i=s.createBuffer(2,n,s.sampleRate),r=977;for(let a=0;a<2;a++){let o=i.getChannelData(a);for(let l=0;l<n;l++)r=Math.imul(r,1664525)+1013904223|0,o[l]=((r>>>0)/2147483648-1)*Math.pow(1-l/n,t)}return i}function Uw(s=()=>{}){let e=null,t=null,n=null,i=!1,r=!1,a=!1,o=0,l=null,c=[],h=[],d=new Float32Array(256),u=new Set,p=.18,x=null,M=null,m=null,f=null,b=null,E=!1,g="clear",_=[],w=null,T=null,y=null,C=null,S=null,D=null,v=null,I=null,P=null,N="0.000",z="0.000",X="0.000",j=0,V=0,W=0,ie=0,H={x:0,y:2.4,z:0,fx:0,fz:-1,origin:{x:0,z:0}},ge={busy:!1,day:0,evening:0,weather:"clear"},Ze={ambience:1,effects:1,voice:1};try{a=localStorage.getItem("aurago.desktop.sysworld.sound")==="true"}catch{}function G(){if(e||i)return;e=new(window.AudioContext||window.webkitAudioContext);let B=it=>(c.push(it),it),Z=it=>(h.push(it),B(it));t=B(e.createGain()),t.gain.value=0,n=B(e.createAnalyser()),n.fftSize=512,t.connect(n),n.connect(e.destination),m=B(e.createGain()),m.gain.value=Ze.ambience,m.connect(t),f=B(e.createGain()),f.gain.value=Ze.effects,f.connect(t),b=B(e.createGain()),b.gain.value=Ze.voice,b.connect(t),S=B(e.createConvolver()),S.buffer=Nw(e,3.4,2.6);let pe=B(e.createGain());pe.gain.value=.55,S.connect(pe).connect(m),l=Rg(e,b),x=Cg(e,f,m),x.environment(E,g),M=Pg(e,f,m),M.environment(E),M.listener(H.x,H.y,H.z,H.fx,H.fz,H.origin.x,H.origin.z);for(let[it,zt]of[[55,.07],[82.4069,.022],[110,.01]]){let Bt=Z(e.createOscillator()),pn=B(e.createGain());Bt.frequency.value=it,pn.gain.value=zt,Bt.connect(pn).connect(m),Bt.start()}w=B(e.createBiquadFilter()),w.type="lowpass",w.frequency.value=650,w.Q.value=.6,T=B(e.createGain()),T.gain.value=.042;let Te=B(e.createGain());Te.gain.value=.9,w.connect(T).connect(m),T.connect(Te).connect(S),_=Ud[0].map(it=>[-6,6].map(zt=>{let Bt=Z(e.createOscillator()),pn=B(e.createGain());return Bt.type="sawtooth",Bt.frequency.value=it,Bt.detune.value=zt,pn.gain.value=.22,Bt.connect(pn).connect(w),Bt.start(),Bt}));let ve=Z(e.createOscillator()),we=B(e.createGain());ve.frequency.value=.05,we.gain.value=180,ve.connect(we).connect(w.frequency),ve.start();let ue=e.createBuffer(1,e.sampleRate*4,e.sampleRate),Ae=ue.getChannelData(0),ze=0;for(let it=0;it<Ae.length;it++)ze=(ze+(Math.random()*2-1)*.018)/1.018,Ae[it]=ze;let Y=Ae[Ae.length-1]-Ae[0];for(let it=0;it<Ae.length;it++)Ae[it]-=Y*it/(Ae.length-1);let Ce=Z(e.createBufferSource());y=B(e.createBiquadFilter()),C=B(e.createGain()),Ce.buffer=ue,Ce.loop=!0,y.type="lowpass",y.frequency.value=680,y.Q.value=.25,C.gain.value=.32,Ce.connect(y).connect(C).connect(m),Ce.start();let Re=Z(e.createOscillator()),Ve=B(e.createGain());Re.frequency.value=.075,Ve.gain.value=160,Re.connect(Ve).connect(y.frequency),Re.start(),D=e.createBuffer(1,e.sampleRate*2,e.sampleRate);let He=D.getChannelData(0);for(let it=0;it<He.length;it++)He[it]=Math.random()*2-1;let De=Z(e.createBufferSource());De.buffer=D,De.loop=!0;let Ye=B(e.createBiquadFilter());Ye.type="bandpass",Ye.frequency.value=620,Ye.Q.value=.5;let et=Z(e.createOscillator()),Fe=B(e.createGain());et.frequency.value=.08,Fe.gain.value=140,et.connect(Fe).connect(Ye.frequency),et.start(),v=B(e.createGain()),v.gain.value=0,De.connect(Ye).connect(v).connect(m),De.start();let Je=Z(e.createBufferSource());Je.buffer=D,Je.loop=!0;let F=B(e.createBiquadFilter());F.type="bandpass",F.frequency.value=320,F.Q.value=.7;let q=Z(e.createOscillator()),$=B(e.createGain());q.frequency.value=.13,$.gain.value=90,q.connect($).connect(F.frequency),q.start(),I=B(e.createGain()),I.gain.value=0,Je.connect(F).connect(I).connect(m),Je.start();let de=Z(e.createOscillator()),Me=Z(e.createOscillator());de.type="sine",de.frequency.value=118,Me.type="sine",Me.frequency.value=236;let he=B(e.createGain());he.gain.value=.26;let Pe=B(e.createGain());Pe.gain.value=.9;let le=Z(e.createOscillator()),qe=B(e.createGain());le.frequency.value=.19,qe.gain.value=.08,le.connect(qe).connect(Pe.gain),le.start();let Xe=B(e.createBiquadFilter());Xe.type="lowpass",Xe.frequency.value=420,Xe.Q.value=.4,P=B(e.createGain()),P.gain.value=0,de.connect(Pe),Me.connect(he).connect(Pe),Pe.connect(Xe).connect(P).connect(m),de.start(),Me.start(),_e(),Ue()}let ye=()=>!!e&&a&&r&&!i&&p>0;function J(B){let Z={release(){if(u.delete(Z))for(let pe of B){try{pe.stop?.()}catch{}try{pe.disconnect()}catch{}}}};return u.add(Z),B[0].onended=Z.release,ie++,Z}function A(B,Z,pe){let Te=e.createStereoPanner(),ve=e.createGain();return Te.pan.value=Z,ve.gain.value=pe,B.connect(Te).connect(m),Te.connect(ve).connect(S),[Te,ve]}function me(B){let Z=B.x-H.x,pe=B.z-H.z,Te=B.x-H.origin.x,ve=B.z-H.origin.z,we=Math.hypot(Te,ve),ue=Math.max(1,Math.hypot(Z,pe));return{distance:we,pan:Math.max(-1,Math.min(1,(Z*-H.fz+pe*H.fx)/ue)),fade:1/(1+we*.02)}}function ce(B){if(B==="horn"||B==="gull")return mh.harbour;if(B==="windchime")return mh.quay;if(B==="bell")return mh.tower;if(B==="pigeon")return mh.square;let Z=Math.random()*Math.PI*2,pe=24+Math.random()*44;return{x:H.origin.x+Math.sin(Z)*pe,z:H.origin.z+Math.cos(Z)*pe}}function ne(B){let Z=e.currentTime,pe=Ig[Math.floor(Math.random()*Ig.length)],Te=e.createOscillator(),ve=e.createOscillator(),we=e.createGain(),ue=e.createGain(),{pan:Ae,fade:ze}=me(B);Te.frequency.value=pe,ve.frequency.value=pe*2.76,we.gain.value=.22,ue.gain.setValueAtTime(0,Z),ue.gain.linearRampToValueAtTime(ge.busy?.02:.014,Z+.008),ue.gain.exponentialRampToValueAtTime(1e-4,Z+2.8),Te.connect(ue),ve.connect(we).connect(ue),J([Te,ve,we,ue,...A(ue,Ae,1.4)]),Te.start(Z),ve.start(Z),Te.stop(Z+2.9),ve.stop(Z+2.9)}function K(B){let Z=e.currentTime,pe=e.createBufferSource(),Te=e.createBiquadFilter(),ve=e.createGain(),{pan:we,fade:ue}=me(B);pe.buffer=D,pe.loop=!0,Te.type="bandpass",Te.Q.value=1.4,Te.frequency.setValueAtTime(260,Z),Te.frequency.exponentialRampToValueAtTime(1100,Z+1.6),Te.frequency.exponentialRampToValueAtTime(380,Z+3.4),ve.gain.setValueAtTime(0,Z),ve.gain.linearRampToValueAtTime(.03*ue,Z+1.6),ve.gain.exponentialRampToValueAtTime(1e-4,Z+3.6),pe.connect(Te).connect(ve);let[Ae,ze]=A(ve,we,.5),Y=Math.max(-1,we-.6),Ce=Math.min(1,we+.6);Ae.pan.setValueAtTime(Y,Z),Ae.pan.linearRampToValueAtTime(Ce,Z+3.4),J([pe,Te,ve,Ae,ze]),pe.start(Z),pe.stop(Z+3.7)}function oe(){let B=e.currentTime,Z=g==="rain"?.36:.32;y.frequency.cancelScheduledValues(B),C.gain.cancelScheduledValues(B),y.frequency.setTargetAtTime(1200+Math.random()*700,B,.8),y.frequency.setTargetAtTime(680,B+2.6,1.4),C.gain.setTargetAtTime(Z*1.7,B,.7),C.gain.setTargetAtTime(Z,B+2.6,1.5),ie++}function re(B){let Z=e.currentTime,pe=e.createBiquadFilter(),Te=e.createGain(),{pan:ve,fade:we}=me(B);pe.type="lowpass",pe.frequency.value=420,Te.gain.setValueAtTime(0,Z),Te.gain.linearRampToValueAtTime(.028*we,Z+.9),Te.gain.setValueAtTime(.028*we,Z+3.2),Te.gain.exponentialRampToValueAtTime(1e-4,Z+5.5);let ue=[69.3,103.83].map(Ae=>{let ze=e.createOscillator();return ze.type="sawtooth",ze.frequency.value=Ae,ze.detune.value=Math.random()*8-4,ze.connect(pe),ze});pe.connect(Te),J([...ue,pe,Te,...A(Te,ve,1.6)]),ue.forEach(Ae=>{Ae.start(Z),Ae.stop(Z+5.6)})}function te(B){let Z=e.currentTime,pe=2+Math.floor(Math.random()*3),{pan:Te,fade:ve}=me(B);for(let we=0;we<pe&&u.size<ds;we++){let ue=Z+we*(.12+Math.random()*.1),Ae=e.createOscillator(),ze=e.createGain(),Y=2600+Math.random()*900;Ae.frequency.setValueAtTime(Y,ue),Ae.frequency.exponentialRampToValueAtTime(Y*1.5,ue+.07),ze.gain.setValueAtTime(0,ue),ze.gain.linearRampToValueAtTime(.006*ve,ue+.015),ze.gain.exponentialRampToValueAtTime(1e-4,ue+.13),Ae.connect(ze),J([Ae,ze,...A(ze,Te,.6)]),Ae.start(ue),Ae.stop(ue+.15)}}function fe(B){let Z=e.currentTime,pe=2+Math.floor(Math.random()*3),Te=760+Math.random()*280,{pan:ve,fade:we}=me(B);for(let ue=0;ue<pe&&u.size<ds;ue++){let Ae=Z+ue*(.2+Math.random()*.14),ze=e.createOscillator(),Y=e.createBiquadFilter(),Ce=e.createGain();ze.type="sawtooth",ze.frequency.setValueAtTime(Te,Ae),ze.frequency.exponentialRampToValueAtTime(Te*.5,Ae+.26),Y.type="bandpass",Y.frequency.value=1500,Y.Q.value=3.6,Ce.gain.setValueAtTime(0,Ae),Ce.gain.linearRampToValueAtTime(.012*we,Ae+.04),Ce.gain.exponentialRampToValueAtTime(1e-4,Ae+.34),ze.connect(Y).connect(Ce),J([ze,Y,Ce,...A(Ce,ve,.9)]),ze.start(Ae),ze.stop(Ae+.36)}}function Ee(B){let Z=e.currentTime,pe=2+Math.floor(Math.random()*2),Te=420+Math.random()*90,{pan:ve,fade:we}=me(B);for(let ue=0;ue<pe&&u.size<ds;ue++){let Ae=Z+ue*(.17+Math.random()*.08),ze=e.createOscillator(),Y=e.createBiquadFilter(),Ce=e.createGain();ze.type="triangle",ze.frequency.setValueAtTime(Te,Ae),ze.frequency.linearRampToValueAtTime(Te*.72,Ae+.12),Y.type="lowpass",Y.frequency.value=1100,Ce.gain.setValueAtTime(0,Ae),Ce.gain.linearRampToValueAtTime(.01*we,Ae+.03),Ce.gain.exponentialRampToValueAtTime(1e-4,Ae+.16),ze.connect(Y).connect(Ce),J([ze,Y,Ce,...A(Ce,ve,.5)]),ze.start(Ae),ze.stop(Ae+.18)}}function Ne(B){let Z=e.currentTime,pe=3+Math.floor(Math.random()*4),Te=3900+Math.random()*900,{pan:ve,fade:we}=me(B);for(let ue=0;ue<pe&&u.size<ds;ue++){let Ae=Z+ue*.055,ze=e.createOscillator(),Y=e.createGain();ze.type="square",ze.frequency.value=Te,Y.gain.setValueAtTime(0,Ae),Y.gain.linearRampToValueAtTime(.005*we,Ae+.008),Y.gain.exponentialRampToValueAtTime(1e-4,Ae+.035),ze.connect(Y),J([ze,Y,...A(Y,ve,.35)]),ze.start(Ae),ze.stop(Ae+.05)}}function Oe(B){let Z=e.currentTime,pe=196,{pan:Te,fade:ve}=me(B),we=e.createGain();we.gain.setValueAtTime(0,Z),we.gain.linearRampToValueAtTime(.02*ve,Z+.012),we.gain.exponentialRampToValueAtTime(1e-4,Z+4.2);let ue=[1,2.02,2.99,4.18,5.43].map((Ae,ze)=>{let Y=e.createOscillator(),Ce=e.createGain();return Y.type="sine",Y.frequency.value=pe*Ae,Ce.gain.value=.5/(ze+1),Y.connect(Ce).connect(we),Y});J([...ue,we,...A(we,Te,1.8)]),ue.forEach(Ae=>{Ae.start(Z),Ae.stop(Z+4.3)})}function ut(B){let Z=e.currentTime,pe=2+Math.floor(Math.random()*3),{pan:Te,fade:ve}=me(B);for(let we=0;we<pe&&u.size<ds;we++){let ue=Z+we*(.5+Math.random()*.35),Ae=e.createBufferSource(),ze=e.createBiquadFilter(),Y=e.createGain(),Ce=e.createOscillator(),Re=e.createGain();Ae.buffer=D,Ae.loop=!0,ze.type="bandpass",ze.frequency.value=420,ze.Q.value=1.1,Y.gain.setValueAtTime(0,ue),Y.gain.linearRampToValueAtTime(.02*ve,ue+.006),Y.gain.exponentialRampToValueAtTime(1e-4,ue+.42),Ce.type="sine",Ce.frequency.value=1180,Re.gain.value=.2,Ae.connect(ze).connect(Y),Ce.connect(Re).connect(Y),J([Ae,ze,Y,Ce,Re,...A(Y,Te,.9)]),Ae.start(ue),Ae.stop(ue+.44),Ce.start(ue),Ce.stop(ue+.44)}}function rt(B){let Z=e.currentTime,pe=2+Math.floor(Math.random()*4),{pan:Te,fade:ve}=me(B);for(let we=0;we<pe&&u.size<ds;we++){let ue=Z+we*(.06+Math.random()*.14),Ae=e.createOscillator(),ze=e.createGain();Ae.type="sine",Ae.frequency.setValueAtTime(900+Math.random()*700,ue),Ae.frequency.exponentialRampToValueAtTime(320,ue+.06),ze.gain.setValueAtTime(0,ue),ze.gain.linearRampToValueAtTime(.008*ve,ue+.005),ze.gain.exponentialRampToValueAtTime(1e-4,ue+.12),Ae.connect(ze),J([Ae,ze,...A(ze,Te,.4)]),Ae.start(ue),Ae.stop(ue+.14)}}function at(B){let Z=e.currentTime,pe=3+Math.floor(Math.random()*4),Te=[1046.5,1174.7,1396.9,1567.98,1760,2093],{pan:ve,fade:we}=me(B);for(let ue=0;ue<pe&&u.size<ds;ue++){let Ae=Z+ue*(.09+Math.random()*.13),ze=Te[Math.floor(Math.random()*Te.length)],Y=e.createOscillator(),Ce=e.createGain();Y.type="sine",Y.frequency.value=ze,Ce.gain.setValueAtTime(0,Ae),Ce.gain.linearRampToValueAtTime(.006*we,Ae+.004),Ce.gain.exponentialRampToValueAtTime(1e-4,Ae+.9),Y.connect(Ce),J([Y,Ce,...A(Ce,ve,1.5)]),Y.start(Ae),Y.stop(Ae+.95)}}function Q(B){let Z=e.currentTime,{pan:pe,fade:Te}=me(B),ve=e.createOscillator(),we=e.createGain();ve.type="sine",ve.frequency.setValueAtTime(420,Z),ve.frequency.linearRampToValueAtTime(620,Z+1.8),ve.frequency.linearRampToValueAtTime(430,Z+3.6),we.gain.setValueAtTime(0,Z),we.gain.linearRampToValueAtTime(.01*Te,Z+.5),we.gain.setValueAtTime(.01*Te,Z+2.8),we.gain.exponentialRampToValueAtTime(1e-4,Z+4.2),ve.connect(we),J([ve,we,...A(we,pe,1.2)]),ve.start(Z),ve.stop(Z+4.3)}let vt={chime:ne,swoosh:K,gust:oe,horn:re,birds:te,gull:fe,pigeon:Ee,cricket:Ne,bell:Oe,hammer:ut,drip:rt,windchime:at,siren:Q};function je(){let B=Fw.filter(Te=>!Te.when||Te.when(ge));if(!B.length)return"chime";let Z=0;for(let Te of B)Z+=Te.weight;let pe=Math.random()*Z;for(let Te of B)if(pe-=Te.weight,pe<=0)return Te.kind;return B[B.length-1].kind}function O(){if(u.size>=ds)return;let B=je(),Z=ce(B);vt[B]?.(Z)}function R(){clearTimeout(j),j=0,ye()&&(j=setTimeout(()=>{j=0,ye()&&(O(),R())},(ge.busy?1900:3600)+Math.random()*9e3))}function ee(){clearTimeout(V),V=0,ye()&&(V=setTimeout(()=>{V=0,ye()&&(W=(W+1)%Ud.length,_.forEach((B,Z)=>B.forEach(pe=>pe.frequency.setTargetAtTime(Ud[W][Z],e.currentTime,1.8))),ee())},16e3))}function ae(){clearTimeout(j),clearTimeout(V),j=V=0;for(let B of[...u])B.release()}function _e(){if(!e)return;let B=e.currentTime;w.frequency.setTargetAtTime(ge.busy?1500:650+ge.day*450+ge.evening*150,B,2.5),T.gain.setTargetAtTime((ge.busy?.055:.042)*(ge.weather==="rain"?.75:1),B,2)}function Ue(){if(!e||!v||!I||!P)return;let B=Math.max(-76,Math.min(76,H.origin.x)),Z=Math.hypot(H.origin.x-B,H.origin.z-77),pe=Math.max(0,1-(Z-6)/42),Te=Math.max(0,1-(Z-25)/110)*.18,ve=(E?0:Math.min(1,pe+Te)*.05*(g==="rain"?1.2:1)).toFixed(3),we=((ge.busy?.03:.012)*(.35+ge.day*.65)*(E?.25:1)).toFixed(3),ue=Math.hypot(H.origin.x-B,H.origin.z-75.5),Ae=Math.max(0,1-(ue-8)/46),ze=(E?0:Math.min(1,Ae)*.012*(1-ge.day)*(g==="rain"?.6:1)).toFixed(3);if(ve===N&&we===z&&ze===X)return;N=ve,z=we,X=ze;let Y=e.currentTime;v.gain.setTargetAtTime(Number(ve),Y,.5),I.gain.setTargetAtTime(Number(we),Y,.8),P.gain.setTargetAtTime(Number(ze),Y,.8)}function ke(B=!1){if(clearTimeout(o),!i){if(B&&a)try{G()}catch{a=!1,s(!1);return}e&&(l?.setActive(a&&r&&p>0&&Ze.voice>0),x?.setActive(a&&r&&p>0),M?.setActive(a&&r&&p>0&&Ze.effects>0),a&&r?(e.resume().catch(()=>{}),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(p,e.currentTime,.25),j||R(),V||ee()):(ae(),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(0,e.currentTime,.035),o=setTimeout(()=>{!i&&(!a||!r)&&e.suspend().catch(()=>{})},180)))}}return{enabled:()=>a,toggle(){a=!a;try{localStorage.setItem("aurago.desktop.sysworld.sound",String(a))}catch{}ke(!0),s(a)},unlock(){ke(!0)},setActive(B){r!==B&&(r=B,ke())},setVolume(B){Number.isFinite(B)&&(p=Math.max(0,Math.min(.35,B)),ke())},setListener(B,Z,pe,Te,ve,we,ue){Object.assign(H,{x:B,y:Z,z:pe,fx:Te,fz:ve}),H.origin.x=we??B,H.origin.z=ue??pe,l?.setListener(B,Z,pe,Te,ve),x?.listener(B,Z,pe,Te,ve),M?.listener(B,Z,pe,Te,ve,H.origin.x,H.origin.z),Ue()},setChannel(B,Z){if(!(B in Ze)||!Number.isFinite(Z))return;Ze[B]=Math.max(0,Math.min(1,Z));let pe={ambience:m,effects:f,voice:b}[B];pe&&pe.gain.setTargetAtTime(Ze[B],e.currentTime,.05),ke()},effect(B,Z,pe,Te,ve){if(String(B).startsWith("robot_")){M?.action(B,Z,pe,Te,ve);return}x?.play(B,Z,pe,Te)},gait(B,Z,pe,Te,ve,we,ue){M?.gait(B,Z,pe,Te,ve,we,ue)},setEnvironment(B,Z){E=!!B,["clear","rain","fog"].includes(Z)&&(g=Z),x?.environment(E,g),M?.environment(E),Ue()},setMood(B={}){Object.assign(ge,{busy:!!B.busy,day:Number.isFinite(B.day)?B.day:ge.day,evening:Number.isFinite(B.evening)?B.evening:ge.evening,weather:["clear","rain","fog"].includes(B.weather)?B.weather:ge.weather}),_e(),Ue()},thunder(B=1){if(!ye()||E||u.size>=ds)return;let Z=Math.max(0,Math.min(5,B)),pe=e.currentTime+Z,Te=Math.min(1,Z/4),ve=e.createBufferSource(),we=e.createBiquadFilter(),ue=e.createGain();ve.buffer=D,ve.loop=!0,we.type="lowpass",we.Q.value=.7,we.frequency.setValueAtTime(900-Te*500,pe),we.frequency.exponentialRampToValueAtTime(70,pe+3.5),ue.gain.setValueAtTime(0,pe),ue.gain.linearRampToValueAtTime(.09*(1-Te*.5),pe+.06+Te*.25),ue.gain.exponentialRampToValueAtTime(.025,pe+1.3),ue.gain.exponentialRampToValueAtTime(1e-4,pe+5.5),ve.connect(we).connect(ue),J([ve,we,ue,...A(ue,Math.random()*.8-.4,.8)]),ve.start(pe),ve.stop(pe+5.6)},stats(){let B=0;if(n&&e.state==="running"){n.getFloatTimeDomainData(d);for(let Z of d)B+=Z*Z}return{voice:l?.stats(),effects:x?.stats(),foley:M?.stats(),channels:{...Ze},enabled:a,active:r,state:e?.state||"uninitialized",volume:p,rms:Math.sqrt(B/d.length),soundscape:{chord:W,events:ie,transient:u.size,scheduled:!!j,mood:{...ge},beds:{water:Number(N),murmur:Number(z),lamp:Number(X)},listener:{x:H.x,z:H.z}}}},dispose(){i||(i=!0,ae(),M?.dispose(),x?.dispose(),l?.dispose(),clearTimeout(o),h.forEach(B=>{try{B.stop()}catch{}}),c.forEach(B=>B.disconnect()),e&&e.close().catch(()=>{}))}}}var Sn=[{id:"agent",asset:"agent-spire",x:0,z:-12,height:87,radius:13},{id:"infra",asset:"compute-foundry",x:-43,z:-53,height:24,radius:18},{id:"integrations",asset:"integration-gate",x:43,z:-10,height:34,radius:14},{id:"missions",asset:"mission-terminal",x:43,z:36,height:23,radius:15},{id:"memory",asset:"memory-archive",x:-43,z:-10,height:37,radius:14},{id:"graph",asset:"knowledge-atrium",x:-43,z:36,height:28,radius:17},{id:"operations",asset:"operations-beacon",x:0,z:36,height:22,radius:6}],gh={low:{lod:2,dpr:1,shadow:0,bloom:!1},medium:{lod:1,dpr:1.25,shadow:1024,bloom:!1},high:{lod:0,dpr:1.5,shadow:2048,bloom:!0},ultra:{lod:0,dpr:2,shadow:4096,bloom:!0}},zd=new U(122,106,183),Od=class extends ni{constructor(e,t,n){super(),this.scene=e,this.camera=t,this.needsSwap=!1,this.target=new ln(1,1,{type:xn,samples:n}),this.quad=new Vi(new Rt({uniforms:Ai.clone(ls.uniforms),vertexShader:ls.vertexShader,depthTest:!1,depthWrite:!1,fragmentShader:"uniform sampler2D tDiffuse;varying vec2 vUv;void main(){vec4 c=texture2D(tDiffuse,vUv);if(any(isnan(c))||any(isinf(c)))c=vec4(0.0,0.0,0.0,1.0);gl_FragColor=c;}"}))}render(e,t,n){let i=e.autoClear;e.autoClear=!1,e.setRenderTarget(this.target),e.clear(),e.render(this.scene,this.camera),this.quad.material.uniforms.tDiffuse.value=this.target.texture,e.setRenderTarget(this.renderToScreen?null:n),this.quad.render(e),e.autoClear=i}setSize(e,t){this.target.setSize(e,t)}dispose(){this.target.dispose(),this.quad.material.dispose(),this.quad.dispose()}},va=new U(0,22,-12);function zw(s){return s.name+(s.vertexColors?"|vertex-colours":"")}var Dg=pm().map(s=>({asset:s.id,x:s.x,z:s.z,y:0,angle:s.angle,scale:[1,1,1]})),VA=["agent-spire","memory-archive","integration-gate","mission-terminal","compute-foundry","knowledge-atrium","operations-beacon","data-tower-a","data-tower-b","skybridge","street-lamp","planter"];function Ow(s,e){return e?.has(s)?"v3":"v1"}var fr=[];{let s=(a,o,l,c=0,h=0,d=[1,1,1])=>fr.push({asset:a,x:o,z:l,y:c,angle:h,scale:d}),{xs:e,zs:t}=Wt,n=bt.pavement-.05;for(let a of t)for(let o of[-55,-31,31,55])s("street-lamp",o,a-4.5,n),s("planter",o+4*Math.sign(Math.sign(o)*43-o),a-4.5,n);for(let{x:a,z:o,scale:l}of nh)s("data-tower-a",a,o,0,0,[1,l,1]);s("skybridge",39.75,-55,15),s("server-rack",-55,-36.7,bt.pavement),s("server-rack",-49,-36.7,bt.pavement);let i=7919,r=()=>(i=Math.imul(i,1664525)+1013904223>>>0)/4294967296;for(let a=0;a<4;a++)for(let o=0;o<12;o++){let l=r(),c=r(),h=r(),d=r(),u=r(),p=r();if(d<.12)continue;let x=(o-5.5)*22+(l-.5)*12,M=-143-a*29+(c-.5)*14,m=1-Math.min(1,Math.abs(x)/150),f=(.5+h*h+.45*m)*(a?1:.85),b=.7+.34*u;s(l>.5?"data-tower-a":"data-tower-b",x,M,-3,p<.3?Math.PI/2:0,[b,f,b])}for(let a=0;a<16;a++){let o=(a-7.5)*34+(r()-.5)*16,l=-268-r()*46;s(r()<.5?"data-tower-a":"data-tower-b",o,l,-3,0,[1.1,.55+r()*1.1,1.1])}}var kw=[...im(fr),...On.flatMap(s=>[-3,3].flatMap(e=>[-3,3].map(t=>({x:s.x+e,z:s.z+t,r:4.25,asset:"interior"}))))];async function WA(s,e){let t=!1,n=!0,i="orbit",r=e.quality||"auto",a=r==="auto"?"high":r,o=0,l=null,c=0,h=0,d=0,u=0,p=0,x=0,M=0,m=null,f=e.reducedMotion,b=1,E=1,g=[],_=!1,w=null,T=null,y=null,C=new AbortController,S=new Set,D=new Map,v=new Map,I=new Set,P=new Set,N=new Set,z=[],X=new Hc({antialias:!0,alpha:!1});X.toneMapping=ir,X.toneMappingExposure=.98,X.shadowMap.type=tr,X.shadowMap.autoUpdate=!1,X.info.autoReset=!1;let j=X.domElement;j.className="sysworld-gl",j.tabIndex=0,j.setAttribute("aria-label",e.label),s.append(j);let V=new Ys;V.background=new Le(528926),V.fog=new Ua(528926,.004);let W=new gn(43,1,.3,1800);W.position.copy(zd);let ie=th(),H=ie.register("visitor",{circles:[{x:0,z:0,r:.38}],minY:-.4,maxY:.4,reach:.38},W.position,100),ge=new jc(W,j);ge.target.copy(va),ge.enableDamping=!0,ge.dampingFactor=.085,ge.minDistance=12,ge.maxDistance=410,ge.maxPolarAngle=Math.PI*.485,ge.update();let Ze=0,G=()=>{let k=ye?.progress();e.onProgress?.(p+(k?.bytes||0),Ze+(k?.expected||0))},ye=Mg(X,{url:k=>e.resourceURL("/3d/system-world/textures/v1/"+k),onProgress:()=>G(),shelters:On.map(k=>[k.x-k.width/2,k.z-k.depth/2,k.x+k.width/2,k.z+k.depth/2])}),J=Sg(),A=new aa(X),me=new Kc,ce=A.fromScene(me,.05);V.environment=ce.texture,V.environmentIntensity=.48,me.dispose(),A.dispose();let ne=new $a(12639487,1056813,.8);V.add(ne);let K=new as(13164287,3.2);K.position.set(-70,145,85),K.castShadow=!0,Object.assign(K.shadow.camera,{left:-108,right:108,top:108,bottom:-108,near:1,far:360}),K.shadow.normalBias=.09,K.shadow.bias=-15e-5,K.shadow.radius=2.4,V.add(K);let oe=new as(16758130,2.4);oe.position.set(90,80,-120),V.add(oe);let re=new as(14195306,.42);re.position.set(-10,-40,30),V.add(re);let te=new wi(6479871,0,75,2);te.position.set(0,34,-12),V.add(te);let fe=new ln(1,1,{type:xn}),Ee=new Jc(X,fe);Ee.addPass(new Od(V,W,Math.min(4,X.capabilities.maxSamples)));let Ne=new da(new Ge(1,1),.28,.68,1.06);Ee.addPass(Ne),Ee.addPass(new Zc);let Oe=new Et,ut=new Et,rt=new Et;V.add(Oe),Oe.add(ut,rt);let at=Ag({group:Oe,doors:()=>ue?.lobbies,load:(k,Ie)=>(D.has("v3:"+k+":"+Ie)||(Ze+=le.get(k)?.lods.find(We=>We.level===Ie)?.bytes||0),Xe(k,Ie,"v3"))}),Q=bg(V);function vt(k,Ie){return I.add(k),P.add(Ie),new ht(k,Ie)}let je=new vn({color:1911345,metalness:.05,roughness:.82}),O=new vn({color:1382685,metalness:.05,roughness:.9});P.add(je),P.add(O);let R=new _i(Wt.maxX-Wt.minX,cr.top-cr.bottom,Wt.maxZ-Wt.minZ);I.add(R);let ee=new ht(R,[je,je,O,je,je,je]);ee.position.set((Wt.minX+Wt.maxX)/2,(cr.top+cr.bottom)/2,(Wt.minZ+Wt.maxZ)/2),ee.receiveShadow=!0,Oe.add(ee),ye.apply(je,"world","ground"),ye.apply(O,"world","city.road");let ae=mm(),_e=new _i(1,1,1);I.add(_e);let Ue=new An(_e,je,ae.length);Ue.name="plots",Ue.receiveShadow=!0,ae.forEach((k,Ie)=>Ue.setMatrixAt(Ie,new yt().compose(new U((k.x0+k.x1)/2,(k.top+k.bottom)/2,(k.z0+k.z1)/2),new nn,new U(k.x1-k.x0,k.top-k.bottom,k.z1-k.z0)))),Oe.add(Ue);let ke=qm(V,{sunDirection:K.position,lamps:fr.filter(k=>k.asset==="street-lamp")});Ee.addPass(ke.post);let B=jm(V,{anchors:Sn.map(k=>({id:k.id,x:k.x,z:k.z,height:k.height}))}),Z=Sn.find(k=>k.id==="memory"),pe=Um(V,Z,{roof:21,label:e.memoryLabel}),Te=Ym(V,{traffic:ie,surfaces:ye}),ve=gg(V,{sun:K,rim:oe,hemisphere:ne,atmosphere:ke,surfaces:ye,onThunder:k=>e.onThunder?.(k)}),we="",ue=null,Ae={value:0},ze=new Ss(1,1.045,96);I.add(ze);let Y=(k,Ie)=>{let We=new Rt({transparent:!0,depthWrite:!1,side:Kt,uniforms:{color:{value:new Le(9365742)},time:Ae,opacity:{value:k},dashes:{value:Ie}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float time,opacity,dashes;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;float dash=dashes>0.5?step(0.3,fract(a*dashes-time*0.35)):1.0;gl_FragColor=vec4(color,opacity*dash*(0.8+0.2*sin(time*2.2)));}"});return P.add(We),We},Ce=new ht(ze,Y(.9,18)),Re=new ht(ze,Y(.38,0));for(let k of[Ce,Re])k.rotation.x=-Math.PI/2,k.visible=!1,Oe.add(k);let Ve=new Rt({transparent:!0,depthWrite:!1,side:Kt,uniforms:{color:{value:new Le(9365742)},alpha:{value:0}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float alpha;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;gl_FragColor=vec4(color,alpha*(0.55+0.45*sin(a*3.0)));}"});P.add(Ve);let He=new ht(new Ss(.9,1,96),Ve);I.add(He.geometry),He.rotation.x=-Math.PI/2,He.visible=!1,Oe.add(He);let De=2,Ye={error:16742504,running:7730385,stale:8096667,idle:8175587},et={},Fe=()=>Ce.material.uniforms.color.value.setHex(Ye[et[m]]??9365742),Je=new jt({color:6742230,toneMapped:!1}),F=new An(new Ui(.65,10,6),Je,Sn.length);I.add(F.geometry),P.add(Je),Oe.add(F);let q=new Yt,$=new U,de=new so,Me=new Ge;Sn.forEach((k,Ie)=>{q.position.set(k.x,k.height+2,k.z),q.updateMatrix(),F.setMatrixAt(Ie,q.matrix),F.setColorAt(Ie,new Le(8425630))}),e.signal?.addEventListener("abort",Ht,{once:!0});let he;try{if(e.signal?.aborted)throw Error("Disposed");let k=await fetch(e.assetURL("manifest.json"),{signal:C.signal});if(!k.ok)throw Error("City manifest unavailable");he=await k.json()}catch(k){throw Ht(),k}let Pe=new Map(he.assets.map(k=>[k.id,k])),le=new Map;try{let k=await fetch(e.resourceURL("/3d/system-world/v3/manifest.json"),{signal:C.signal}),Ie=k.ok?await k.json():null;if(Ie?.schema_version===3&&Array.isArray(Ie.assets))for(let We of Ie.assets)le.set(We.id,We)}catch(k){if(C.signal.aborted)throw Ht(),k}yg(ie,vg(Pe,le),Sn,fr),ue=pg(V,{shadows:Q,camera:W,districts:Sn,tier:a,traffic:ie,kit3:le,reduced:()=>f,active:()=>n&&!_&&i!=="map"&&!f&&!e.replaying?.(),assetURL:k=>e.resourceURL("/3d/system-world/v2/"+k),surfaces:ye,clear:(k,Ie)=>ie.clear(H,{...k,heading:0},{...Ie,heading:0},!1),onFlight:k=>e.onFlight?.(k),onInteraction:e.onInteraction,onScope:k=>e.onScope?.(k),onDiscover:e.onDiscover,onSound:e.onSound,onGait:e.onGait,onTerminal:e.onTerminal,onSociety:e.onSociety,onEnvironment:k=>{ve.setIndoor(k),e.onEnvironment?.(k)},onError:e.onError,onReady:()=>{X.shadowMap.needsUpdate=!0}});let qe={v1:{catalog:Pe,url:k=>e.assetURL(k),loader:()=>new Ri},v3:{catalog:le,url:k=>e.resourceURL("/3d/system-world/v3/"+k),loader:()=>new Ri().setMeshoptDecoder(ca)}};async function Xe(k,Ie,We="v1"){let ft=We+":"+k+":"+Ie;return D.has(ft)||D.set(ft,(async()=>{let Be=qe[We],Vt=Be.catalog.get(k)?.lods.find(pt=>pt.level===Ie);if(!Vt||!/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(Vt.file))throw Error("Invalid city asset");let gt=new AbortController;S.add(gt);let wt=setTimeout(()=>gt.abort(),15e3);try{let pt=await fetch(Be.url(Vt.file),{signal:gt.signal});if(!pt.ok)throw Error("City asset unavailable");let mt=await pt.arrayBuffer();if(t)throw Error("Disposed");We==="v3"&&await ca.ready;let Ut=await Be.loader().parseAsync(mt,"");if(t)throw Ut.scene.traverse(nt=>{nt.isMesh&&(nt.geometry.dispose(),nt.material.dispose())}),Error("Disposed");return p+=mt.byteLength,G(),Ut.scene.traverse(nt=>{if(!nt.isMesh)return;I.add(nt.geometry),nt.castShadow=!0,nt.receiveShadow=!0,J.adopt(nt),at.adopt(nt);let sn=(Array.isArray(nt.material)?nt.material:[nt.material]).map(St=>{if(J.owns(St)||at.owns(St))return St;let st=zw(St);if(v.has(st)){let bn=St;St=v.get(st),bn.dispose()}else St.isMeshStandardMaterial&&St.metalness>.5&&(St.roughness=Math.max(St.roughness,.45)),v.set(st,St),P.add(St);return St});nt.material=Array.isArray(nt.material)?sn:sn[0],ye.prepare(nt)}),Ut.scene}finally{clearTimeout(wt),S.delete(gt)}})().catch(Be=>{throw D.delete(ft),Be})),D.get(ft)}function it(k){k.traverse(Ie=>{Ie.isInstancedMesh&&Ie.dispose()}),k.clear()}async function zt(){let k=++o,Ie=gh[a],We=Ie.lod,ft=[...new Set([...fr.map(pt=>pt.asset),...Sn.map(pt=>pt.asset),"service-drone"])],Be=[...new Set(Dg.map(pt=>pt.asset))],Vt=pt=>Ow(pt,le),gt=(pt,mt,Ut=Vt(pt))=>(Ut==="v3"?le:Pe).get(pt)?.lods.find(nt=>nt.level===mt)?.bytes||0,wt=(pt,mt,Ut=Vt(pt))=>D.has(Ut+":"+pt+":"+mt)?0:gt(pt,mt,Ut);Ze=ft.reduce((pt,mt)=>pt+wt(mt,We),p)+(We===2?0:["data-tower-a","data-tower-b"].reduce((pt,mt)=>pt+wt(mt,2),0))+Be.reduce((pt,mt)=>pt+wt(mt,We,"v3"),0);try{let pt=new Map(await Promise.all(ft.map(async st=>[st,await Xe(st,We,Vt(st))]))),mt=new Map(await Promise.all(["data-tower-a","data-tower-b"].map(async st=>[st,await Xe(st,2,Vt(st))]))),Ut=new Map(await Promise.all(Be.map(async st=>[st,await Xe(st,We,"v3")])));if(t||k!==o)return;it(ut),it(rt),g=[];let nt=new Map,xt=(st,bn)=>{bn.updateMatrixWorld(!0);let ji=new yt().compose(new U(st.x,st.y,st.z),new nn().setFromAxisAngle(new U(0,1,0),st.angle),new U(...st.scale));bn.traverse(fs=>{fs.isMesh&&(nt.has(fs.uuid)||nt.set(fs.uuid,{node:fs,matrices:[]}),nt.get(fs.uuid).matrices.push(new yt().multiplyMatrices(ji,fs.matrixWorld)))})};for(let st of fr)xt(st,(st.z<-100?mt:pt).get(st.asset));for(let st of Dg)xt(st,Ut.get(st.asset));for(let{node:st,matrices:bn}of nt.values()){let ji=new An(st.geometry,st.material,bn.length);bn.forEach((fs,Lg)=>ji.setMatrixAt(Lg,fs)),ji.castShadow=!0,ji.receiveShadow=!0,ut.add(ji)}for(let st of Sn){let bn=pt.get(st.asset).clone(!0);bn.position.set(st.x,0,st.z),bn.userData.district=st.id,rt.add(bn),g.push(bn)}T?.attachLandmarks(rt),Te.setTemplate(pt.get("service-drone"));let sn=new un,St=new Map;for(let st of["data-tower-a","data-tower-b"])St.set(st,sn.setFromObject(mt.get(st)).max.y);ke.setAviation(fr.filter(st=>St.has(st.asset)&&(st.z<-100||st.scale[1]>1.1)).map(st=>({x:st.x,y:st.y+St.get(st.asset)*st.scale[1]+.6,z:st.z}))),X.shadowMap.needsUpdate=!0,e.onReady?.()}catch(pt){!t&&k===o&&e.onError?.(pt)}}function Bt(){if(t)return;b=Math.max(1,s.clientWidth),E=Math.max(1,s.clientHeight);let k=Math.min(devicePixelRatio||1,gh[a].dpr,Math.sqrt(3840*2160/(b*E)));X.setPixelRatio(k),X.setSize(b,E,!1),Ee.setPixelRatio(k),Ee.setSize(b,E),W.aspect=b/E,W.updateProjectionMatrix(),pe.setPointScale(E*k),ke.setPointScale(E*k),B.setPointScale(E*k)}function pn(k){return r=k in gh||k==="auto"?k:"auto",a=r==="auto"?"high":r,ii(),zt()}function ii(){let k=gh[a];X.shadowMap.enabled=k.shadow>0,k.shadow&&K.shadow.mapSize.x!==k.shadow&&(K.shadow.mapSize.set(k.shadow,k.shadow),K.shadow.map?.dispose(),K.shadow.map=null),X.shadowMap.needsUpdate=!0,Ne.enabled=k.bloom,ye.setTier(a),J.setTier(a),at.setTier(a),ke.setTier(a),ue.setTier(a),ve.setTier(a),Te.setTier(a),B.setTier(a),Bt(),e.onQuality?.(r,a)}function Kn(k,Ie){l=null;let We=ie.findFree(H,k);if(!We)return;if(k=new U(We.x,We.y,We.z),f){W.position.copy(k),Object.assign(H,We),ge.target.copy(Ie),ge.update(),l=null;return}let ft=(wt,pt)=>ie.clear(H,{...wt,heading:0},{...pt,heading:0},!1),Be=W.position.clone(),Vt=Math.max(Be.y,k.y,110),gt=[Be,k];if(!ft(Be,k)){let wt=[Be],pt=new U(k.x,Vt,k.z),mt=[-1],Ut=[0],nt=-1;if(!ft(pt,k))return;for(let xt of[3,8,16])for(let sn=0;sn<8;sn++)wt.push(new U(Be.x+Math.cos(sn*Math.PI/4)*xt,Be.y,Be.z+Math.sin(sn*Math.PI/4)*xt));for(let xt of On)if(Math.hypot(Be.x-xt.x,Be.z-xt.z)<24)for(let sn of[xt.z,xt.doorZ-xt.front*2,xt.doorZ+xt.front*3])wt.push(new U(xt.x,Be.y,sn));for(;Ut.length;){let xt=Ut.shift(),sn=wt[xt],St=new U(sn.x,Vt,sn.z);if(ft(sn,St)&&ft(St,pt)){nt=xt;break}for(let st=1;st<wt.length;st++)mt[st]===void 0&&sn.distanceTo(wt[st])<=20&&ft(sn,wt[st])&&(mt[st]=xt,Ut.push(st))}if(nt<0)return;gt=[];for(let xt=nt;xt>=0;xt=mt[xt])gt.unshift(wt[xt]);gt.push(new U(wt[nt].x,Vt,wt[nt].z),pt,k)}l={start:W.position.clone(),targetStart:ge.target.clone(),end:k.clone(),target:Ie.clone(),time:0,waypoints:gt}}function Zt(){ie.relocate(H,W.position),W.position.set(H.x,H.y,H.z)}function en(k){let Ie=Sn.find(We=>We.id===k);Ie&&(m=k,Ce.position.set(Ie.x,.55,Ie.z),Ce.scale.setScalar(Ie.radius*1.3),Ce.visible=!0,Fe(),He.position.set(Ie.x,.62,Ie.z),He.scale.setScalar(Ie.radius*1.3*.35),Ve.uniforms.color.value.setHex(Ye[et[k]]??9365742),Ve.uniforms.alpha.value=0,De=0,He.visible=!f,Re.visible=!!Se&&Se!==m,i==="street"?(ue.endRide(),e.onCut?.(),W.position.set(Ie.x,2.4+Dn(Ie.x,Ie.z+Ie.radius+7),Ie.z+Ie.radius+7),W.lookAt(Ie.x,Ie.height*.4,Ie.z),Zt()):Kn(new U(Ie.x+Ie.radius*2.7,Ie.height*.7+18,Ie.z+Ie.radius*4),new U(Ie.x,Ie.height*.4,Ie.z)))}function Pn(k){ue.endRide(),ue.society.suspend(),N.clear(),l=null,i=k,ge.enabled=i==="orbit"||i==="tour",H.minY=i==="street"?-2.3:-.4,H.ignore=H.follow=null,H.circles[0].r=H.reach=i==="street"?.24:.38,document.pointerLockElement===j&&document.exitPointerLock(),i==="street"?(e.onCut?.(),$e(),W.position.set(18,2.4+Dn(18,57),57),W.lookAt(0,26,-12),Zt()):i!=="map"&&(Zt(),Kn(zd.clone().multiplyScalar(W.aspect<1?1.3:1),va)),x=0,M=0,e.onMode?.(i)}let En=()=>{l=null,i==="tour"&&(i="orbit",e.onMode?.(i))};ge.addEventListener("start",En);function Ln(k,Ie,We,ft){k.addEventListener(Ie,We,ft),z.push(()=>k.removeEventListener(Ie,We,ft))}let kn=null,pr=!1,L=new Vn(0,0,0,"YXZ");function se(k,Ie){let We=.0025*W.fov/43;L.setFromQuaternion(W.quaternion),L.y-=k*We,L.x=It.clamp(L.x-Ie*We,-1.35,1.35),W.quaternion.setFromEuler(L)}Ln(j,"pointerdown",k=>{j.focus({preventScroll:!0}),En(),kn=[k.clientX,k.clientY],pr=!1,i==="street"&&j.setPointerCapture(k.pointerId)});let Se=null,xe=-1e9;function be(k){if(k===Se)return;Se=k,j.style.cursor=k?"pointer":"";let Ie=Sn.find(We=>We.id===k);Re.visible=!!Ie&&k!==m,Ie&&(Re.position.set(Ie.x,.56,Ie.z),Re.scale.setScalar(Ie.radius*1.3)),e.onHover?.(k)}function $e(){be(null)}function tt(k){if(i==="street"||i==="map"||kn||k.timeStamp-xe<80)return;xe=k.timeStamp;let Ie=j.getBoundingClientRect();Me.set((k.clientX-Ie.left)/Ie.width*2-1,1-(k.clientY-Ie.top)/Ie.height*2),de.setFromCamera(Me,W);let We=de.intersectObjects(g,!0)[0]?.object;for(;We&&!We.userData.district;)We=We.parent;be(We?.userData.district||null)}Ln(j,"pointerleave",$e),Ln(j,"pointermove",k=>{i==="street"&&(document.pointerLockElement===j||kn)&&se(k.movementX,k.movementY),tt(k),kn&&Math.hypot(k.clientX-kn[0],k.clientY-kn[1])>5&&(pr=!0)}),Ln(j,"pointerup",k=>{if(kn&&!pr&&i!=="street"){let Ie=j.getBoundingClientRect();Me.set((k.clientX-Ie.left)/Ie.width*2-1,1-(k.clientY-Ie.top)/Ie.height*2),de.setFromCamera(Me,W);let We=de.intersectObjects(g,!0)[0];if(We){let ft=We.object;for(;ft&&!ft.userData.district;)ft=ft.parent;ft&&e.onSelect?.(ft.userData.district)}}kn=null}),Ln(j,"pointercancel",()=>{kn=null,N.clear()}),Ln(j,"keydown",k=>{if(k.key==="Escape"){ue.scoping()?ue.endRide():Pn("orbit"),k.preventDefault();return}if(i==="street"&&["+","=","-"].includes(k.key)&&ue.zoom(k.key==="-"?240:-240)){k.preventDefault();return}if(i==="street"&&k.code==="KeyE"&&!k.repeat){ue.interact(),k.preventDefault();return}i==="street"&&["KeyW","KeyA","KeyS","KeyD","ArrowUp","ArrowDown","ArrowLeft","ArrowRight","ShiftLeft"].includes(k.code)&&(N.add(k.code),k.preventDefault(),k.stopPropagation()),i==="street"&&["Space","KeyC"].includes(k.code)&&!k.ctrlKey&&!k.metaKey&&!k.altKey&&(N.add(k.code),k.preventDefault(),k.stopPropagation())}),Ln(j,"keyup",k=>N.delete(k.code)),Ln(j,"wheel",k=>{i==="street"&&ue.zoom(k.deltaY)&&k.preventDefault()},{passive:!1}),Ln(j,"blur",()=>N.clear()),Ln(window,"blur",()=>{N.clear(),kn=null}),Ln(j,"webglcontextlost",k=>{k.preventDefault(),_=!0,N.clear(),e.onContextLost?.()}),w=new ResizeObserver(Bt),w.observe(s);let Ke={y:NaN,v:0};function ot(k){let Ie=(N.has("ShiftLeft")?25:11)*k,We=Number(N.has("KeyW")||N.has("ArrowUp"))-Number(N.has("KeyS")||N.has("ArrowDown")),ft=Number(N.has("KeyD")||N.has("ArrowRight"))-Number(N.has("KeyA")||N.has("ArrowLeft"));if(ue.hovering()){ue.fly(k,{forward:We,right:ft,up:Number(N.has("Space"))-Number(N.has("KeyC")),boost:N.has("ShiftLeft")}),Ke.y=NaN;return}if(We||ft){if(ue.walkRide(We,k,ft))return;let Be=Ie/Math.hypot(We,ft);W.getWorldDirection($),$.y=0,$.normalize();let Vt=($.x*We-$.z*ft)*Be,gt=($.z*We+$.x*ft)*Be,wt=(pt,mt)=>ue.move(pt,mt,W.position)&&ie.clear(H,H,{x:pt,y:2.4+ue.floor(pt,mt),z:mt,heading:0});wt(W.position.x+Vt,W.position.z)&&(W.position.x+=Vt),wt(W.position.x,W.position.z+gt)&&(W.position.z+=gt)}if(ue.isRiding()){Ke.y=NaN;return}Math.abs(W.position.y-Ke.y)<1e-6||(Ke.y=W.position.y,Ke.v=0),W.position.y=zs(Ke,2.4+ue.floor(W.position.x,W.position.z),f?0:k)}let ct=0,Ct=rm(k=>{ie.begin(),ct+=f?0:k,i==="street"&&ot(k),ue.update(k,!f,i);let Ie=H.follow;H.follow=ue.rideBody(),H.ignore=H.follow||Ie,T?.update(k,!f),Te.update(k,ct,!f),ie.propose(H,{...W.position,heading:0},(We,ft)=>W.position.set(ft.x,ft.y,ft.z)),ie.solve(k),ue.settle(f?0:k),ue.syncRide?.(),H.follow&&Object.assign(H,{x:W.position.x,y:W.position.y,z:W.position.z}),H.ignore=H.follow});function Nt(k,Ie){if(t||!n||_||i==="map")return;let We=null,ft=0,Be=null;if(i!=="street"){if(i==="tour"&&(x-=k,x<=0)){let mt=Sn[M++%Sn.length].id;en(mt),e.onTourFocus?.(mt),x=7}if(l){We=l,ft=l.time,l.time+=k;let mt=Math.min(1,l.time/(l.duration||(l.waypoints.length===2?1.1:3.2))),Ut=mt*mt*(3-2*mt),nt=Math.min(l.waypoints.length-1-1e-6,Ut*(l.waypoints.length-1)),xt=Math.floor(nt);W.position.lerpVectors(l.waypoints[xt],l.waypoints[xt+1],nt-xt),ge.target.lerpVectors(l.targetStart,l.target,Ut),W.lookAt(ge.target),Be=W.position.clone(),mt===1&&(l=null)}else ge.update()}Ct(Math.min(.1,Math.max(0,k)))||(ie.begin(),ie.propose(H,{...W.position,heading:0},(mt,Ut)=>W.position.set(Ut.x,Ut.y,Ut.z)),ie.solve(0)),at.update(W.position,rt)&&(X.shadowMap.needsUpdate=!0),We&&(W.position.distanceToSquared(Be)>1e-6?(We.wait=(We.wait||0)+k,We.time=ft,l=We.wait<6?We:null):We.wait=0),W.getWorldDirection($);let Vt=i!=="street"&&W.position.y>15;e.onListener?.(W.position.x,W.position.y,W.position.z,$.x,$.z,Vt?ge.target.x:W.position.x,Vt?ge.target.z:W.position.z);let gt=!!e.busy?.();ve.update(k,Ie,!f)&&(X.shadowMap.needsUpdate=!0),te.intensity=f?0:gt?180+Math.sin(Ie*2)*35:12,ke.setBusy(gt),ke.setCinematic(i==="tour"),ke.update(k,Ie,W,!f),ye.update(k,!f),y?.update(k,!f&&n&&i!=="map"&&!e.replaying?.());let wt=ve.mood();if(B.update(k,!f,1-(wt.day||0)),J.setLighting(wt.day,wt.evening,wt.weather==="clear"?0:1,V.fog.color),at.setLighting(wt.day),J.update(k,!f),!f){if(De<1.4){De+=k;let mt=Math.min(1,De/1.4),Ut=Sn.find(nt=>nt.id===m);Ut&&(He.scale.setScalar(Ut.radius*1.3*(.35+mt*1.05)),Ve.uniforms.alpha.value=(1-mt)*.55),mt>=1&&(He.visible=!1)}for(let mt=0;mt<Sn.length;mt++){let Ut=Sn[mt];q.position.set(Ut.x,Ut.height+2,Ut.z),q.scale.setScalar(1+Math.sin(Ie*1.7+mt*1.13)*.13),q.updateMatrix(),F.setMatrixAt(mt,q.matrix)}F.instanceMatrix.needsUpdate=!0}f||(Ae.value+=Math.min(.1,Math.max(0,k)));let pt=[gt,wt.day.toFixed(1),wt.evening.toFixed(1),wt.weather].join();if(Ne.strength=.2+.15*(1-wt.day),Ne.threshold=1.06+.26*It.smoothstep(wt.day,0,1),X.toneMappingExposure=.95+.05*(1-wt.day),pt!==we&&(we=pt,e.onMood?.({busy:gt,day:wt.day,evening:wt.evening,weather:wt.weather})),pe.update(k,W,!f),X.info.reset(),Ee.render(),c++,r==="auto"&&k>0&&k<.2&&Ie-u>12&&(h+=k,d++,d>=180)){let mt=h/d,Ut=["low","medium","high"],nt=Ut.indexOf(a),xt=mt>.028&&nt>0?Ut[nt-1]:mt<.017&&nt<2?Ut[nt+1]:a;d=0,h=0,xt!==a&&(a=xt,u=Ie,ii(),zt())}}function lt(k,Ie){T?.setData(k,Ie),Sn.forEach((We,ft)=>{let Be=k.find(Vt=>Vt.id===We.id);et[We.id]=!Be||Be.stale?"stale":Be.state==="error"?"error":Be.state==="running"?"running":"idle",F.setColorAt(ft,new Le(Ye[et[We.id]]))}),F.instanceColor.needsUpdate=!0,Fe()}function Ht(){t||(t=!0,o++,C.abort(),S.forEach(k=>k.abort()),N.clear(),document.pointerLockElement===j&&document.exitPointerLock(),e.signal?.removeEventListener("abort",Ht),z.forEach(k=>k()),w?.disconnect(),ge.dispose(),T?.dispose(),y?.dispose(),ue?.dispose(),ve.dispose(),pe.dispose(),Te.dispose(),Q.dispose(),ie.dispose(),ke.dispose(),ye.dispose(),J.dispose(),at.dispose(),B.dispose(),it(ut),it(rt),F.dispose(),Ue.dispose(),I.forEach(k=>k.dispose()),P.forEach(k=>k.dispose()),Ee.passes.forEach(k=>k.dispose?.()),Ee.dispose(),ce.dispose(),K.shadow.dispose(),X.dispose(),X.forceContextLoss(),j.remove(),D.clear(),v.clear())}T=Lm(V,Sn,{traffic:ie,society:ue.society,robotURL:e.resourceURL("/3d/system-world/white-robot.glb"),signal:e.signal,onError:e.onRobotError,active:()=>n&&!_&&i!=="map"&&!e.replaying?.(),obstacles:kw,shadows:Q}),y=xg({scene:V,resourceURL:e.resourceURL,signal:C.signal,shadows:Q,onError:e.onCompanionError}),pe.setReducedMotion(!!f),J.setReducedMotion(!!f);try{ii(),await zt()}catch(k){throw Ht(),k}return{districts:Sn,canvas:j,update:Nt,focus(k){En(),en(k)},setMode:Pn,setQuality:pn,setData:lt,dispose:Ht,interact(){ue.interact()},socialAction:ue.socialAction,visit(k){Pn("street"),ue.visit(k),Zt()},enter(k){Pn("street"),ue.destination(k),Zt()},setEnvironment(k){ve.set(k),X.shadowMap.needsUpdate=!0},setVisible(k){n=k,k||(Ct(0),ue.suspend(),T?.update(0,!1),N.clear(),kn=null,document.pointerLockElement===j&&document.exitPointerLock())},setReducedMotion(k){f=k,pe.setReducedMotion(!!k),J.setReducedMotion(!!k),k&&(l=null,ue.endRide(),ue.society.suspend()),k&&i==="tour"&&Pn("orbit")},setHologram(k,Ie){pe.setTexts(k,Ie),ue.setMemory(k)},setWorld(k,Ie){ue.setWorld(k,Ie)},moveKey(k,Ie){Ie?N.add(k):N.delete(k)},lockPointer(){if(i==="street")return j.requestPointerLock()},project(k){let Ie=Sn.find(ft=>ft.id===k);if(!Ie)return null;$.set(Ie.x,Ie.height+4,Ie.z);let We=W.position.distanceTo($);return $.project(W),{x:($.x+1)*b/2,y:(1-$.y)*E/2,visible:$.z<1&&$.z>-1,distance:We}},capture(){return t||_?Promise.resolve(null):(Ee.render(),new Promise(k=>j.toBlob(k,"image/png")))},intro(){if(f||i!=="orbit"||t)return;let k=zd.clone().multiplyScalar(W.aspect<1?1.3:1);W.position.copy(k).sub(va).applyAxisAngle(new U(0,1,0),-.55).multiplyScalar(1.22).add(va),W.position.y+=34,W.lookAt(va),Zt(),Kn(k,va),l&&(l.duration=3.6)},stats(){return{flying:!!l,traffic:ie.stats(),experience:ue.stats(),weather:ve.stats(),life:T?.stats(),hologram:pe.stats(),atmosphere:ke.stats(),surfaces:ye.stats(),windows:J.stats(),lobbies:at.stats(),drones:Te.stats(),flow:B.stats(),companion:y?.stats(),frames:c,tier:a,mode:i,focusedDistrict:m,hovered:Se,loadedBytes:p+ue.stats().bytes+ye.stats().bytes,cachedModels:D.size,kitModels:{v1:[...D.keys()].filter(k=>k.startsWith("v1:")).length,v3:[...D.keys()].filter(k=>k.startsWith("v3:")).length},calls:X.info.render.calls,triangles:X.info.render.triangles,geometries:X.info.memory.geometries,position:W.position.toArray(),renderer:X.getContext().getParameter(X.getContext().getExtension("WEBGL_debug_renderer_info")?.UNMASKED_RENDERER_WEBGL||X.getContext().RENDERER),disposed:t}}}}export{WA as createCity,Uw as createCityAmbience,Sn as districts,Ow as kitFor,Dg as kitPlacements,kw as obstacles,fr as placements,zw as sharedMaterialKey,VA as supersededV1};
/*! Bundled license information:

three/build/three.core.js:
three/build/three.module.js:
  (**
   * @license
   * Copyright 2010-2026 Three.js Authors
   * SPDX-License-Identifier: MIT
   *)
*/
