var ts={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2},ns={ROTATE:0,PAN:1,DOLLY_PAN:2,DOLLY_ROTATE:3},Pd=0,fh=1,Id=2;var Co=1,dl=2,br=3,Bn=0,Kt=1,Rt=2,zn=0,Sr=1,Ut=2,ph=3,mh=4,Ld=5;var Rs=100,Dd=101,Nd=102,Ud=103,Fd=104,Od=200,Bd=201,zd=202,kd=203,gh=204,xh=205,Hd=206,Vd=207,Gd=208,Wd=209,Xd=210,qd=211,Yd=212,Zd=213,Kd=214,Na=0,Ua=1,Fa=2,tr=3,Oa=4,Ba=5,za=6,ka=7,fl=0,jd=1,Jd=2,Jn=0,Po=1,Io=2,Lo=3,Cs=4,Do=5,No=6,Uo=7,nh="attached",$d="detached",_h=300,is=301,Ps=302,pl=303,ml=304,Fo=306,ai=1e3,Fn=1001,nr=1002,Vt=1003,gl=1004;var Is=1005;var Ot=1006,wr=1007;var $n=1008;var bn=1009,vh=1010,yh=1011,Tr=1012,xl=1013,Qn=1014,Pn=1015,Yt=1016,_l=1017,vl=1018,Er=1020,Mh=35902,bh=35899,Sh=1021,wh=1022,In=1023,li=1026,ss=1027,yl=1028,Ml=1029,rs=1030,bl=1031;var Sl=1033,Oo=33776,Bo=33777,zo=33778,ko=33779,wl=35840,Tl=35841,El=35842,Al=35843,Rl=36196,Cl=37492,Pl=37496,Il=37488,Ll=37489,Ho=37490,Dl=37491,Nl=37808,Ul=37809,Fl=37810,Ol=37811,Bl=37812,zl=37813,kl=37814,Hl=37815,Vl=37816,Gl=37817,Wl=37818,Xl=37819,ql=37820,Yl=37821,Zl=36492,Kl=36494,jl=36495,Jl=36283,$l=36284,Vo=36285,Ql=36286,ec=2200,Qd=2201,ef=2202,vs=2300,ys=2301,Ia=2302,ih=2303,gs=2400,xs=2401,eo=2402,tc=2500,tf=2501,Th=0,Go=1,Ar=2,nf=3200;var Wo=0,sf=1,kn="",Ft="srgb",mn="srgb-linear",to="linear",St="srgb";var La=7680;var rf=519,of=512,af=513,lf=514,nc=515,cf=516,hf=517,ic=518,uf=519,Eh=35044;var Ah="300 es",Yn=2e3,ir=2001;function Zp(s){for(let e=s.length-1;e>=0;--e)if(s[e]>=65535)return!0;return!1}function Kp(s){return ArrayBuffer.isView(s)&&!(s instanceof DataView)}function sr(s){return document.createElementNS("http://www.w3.org/1999/xhtml",s)}function df(){let s=sr("canvas");return s.style.display="block",s}var Hu={},rr=null;function no(...s){let e="THREE."+s.shift();rr?rr("log",e,...s):console.log(e,...s)}function ff(s){let e=s[0];if(typeof e=="string"&&e.startsWith("TSL:")){let t=s[1];t&&t.isStackTrace?s[0]+=" "+t.getLocation():s[1]='Stack trace not available. Enable "THREE.Node.captureStackTrace" to capture stack traces.'}return s}function Ze(...s){s=ff(s);let e="THREE."+s.shift();if(rr)rr("warn",e,...s);else{let t=s[0];t&&t.isStackTrace?console.warn(t.getError(e)):console.warn(e,...s)}}function et(...s){s=ff(s);let e="THREE."+s.shift();if(rr)rr("error",e,...s);else{let t=s[0];t&&t.isStackTrace?console.error(t.getError(e)):console.error(e,...s)}}function _s(...s){let e=s.join(" ");e in Hu||(Hu[e]=!0,Ze(...s))}function pf(s,e,t){return new Promise(function(n,i){function r(){switch(s.clientWaitSync(e,s.SYNC_FLUSH_COMMANDS_BIT,0)){case s.WAIT_FAILED:i();break;case s.TIMEOUT_EXPIRED:setTimeout(r,t);break;default:n()}}setTimeout(r,t)})}var mf={[Na]:Ua,[Fa]:za,[Oa]:ka,[tr]:Ba,[Ua]:Na,[za]:Fa,[ka]:Oa,[Ba]:tr},On=class{addEventListener(e,t){this._listeners===void 0&&(this._listeners={});let n=this._listeners;n[e]===void 0&&(n[e]=[]),n[e].indexOf(t)===-1&&n[e].push(t)}hasEventListener(e,t){let n=this._listeners;return n===void 0?!1:n[e]!==void 0&&n[e].indexOf(t)!==-1}removeEventListener(e,t){let n=this._listeners;if(n===void 0)return;let i=n[e];if(i!==void 0){let r=i.indexOf(t);r!==-1&&i.splice(r,1)}}dispatchEvent(e){let t=this._listeners;if(t===void 0)return;let n=t[e.type];if(n!==void 0){e.target=this;let i=n.slice(0);for(let r=0,o=i.length;r<o;r++)i[r].call(this,e);e.target=null}}},an=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],Vu=1234567,jr=Math.PI/180,Ms=180/Math.PI;function Zn(){let s=Math.random()*4294967295|0,e=Math.random()*4294967295|0,t=Math.random()*4294967295|0,n=Math.random()*4294967295|0;return(an[s&255]+an[s>>8&255]+an[s>>16&255]+an[s>>24&255]+"-"+an[e&255]+an[e>>8&255]+"-"+an[e>>16&15|64]+an[e>>24&255]+"-"+an[t&63|128]+an[t>>8&255]+"-"+an[t>>16&255]+an[t>>24&255]+an[n&255]+an[n>>8&255]+an[n>>16&255]+an[n>>24&255]).toLowerCase()}function ut(s,e,t){return Math.max(e,Math.min(t,s))}function Rh(s,e){return(s%e+e)%e}function jp(s,e,t,n,i){return n+(s-e)*(i-n)/(t-e)}function Jp(s,e,t){return s!==e?(t-s)/(e-s):0}function Jr(s,e,t){return(1-t)*s+t*e}function $p(s,e,t,n){return Jr(s,e,1-Math.exp(-t*n))}function Qp(s,e=1){return e-Math.abs(Rh(s,e*2)-e)}function em(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*(3-2*s))}function tm(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*s*(s*(s*6-15)+10))}function nm(s,e){return s+Math.floor(Math.random()*(e-s+1))}function im(s,e){return s+Math.random()*(e-s)}function sm(s){return s*(.5-Math.random())}function rm(s){s!==void 0&&(Vu=s);let e=Vu+=1831565813;return e=Math.imul(e^e>>>15,e|1),e^=e+Math.imul(e^e>>>7,e|61),((e^e>>>14)>>>0)/4294967296}function om(s){return s*jr}function am(s){return s*Ms}function lm(s){return s>0&&Number.isInteger(s)&&2**Math.round(Math.log2(s))===s}function cm(s){return Math.pow(2,Math.ceil(Math.log(s)/Math.LN2))}function hm(s){return Math.pow(2,Math.floor(Math.log(s)/Math.LN2))}function um(s,e,t,n,i){let r=Math.cos,o=Math.sin,a=r(t/2),l=o(t/2),c=r((e+n)/2),h=o((e+n)/2),u=r((e-n)/2),d=o((e-n)/2),f=r((n-e)/2),g=o((n-e)/2);switch(i){case"XYX":s.set(a*h,l*u,l*d,a*c);break;case"YZY":s.set(l*d,a*h,l*u,a*c);break;case"ZXZ":s.set(l*u,l*d,a*h,a*c);break;case"XZX":s.set(a*h,l*g,l*f,a*c);break;case"YXY":s.set(l*f,a*h,l*g,a*c);break;case"ZYZ":s.set(l*g,l*f,a*h,a*c);break;default:Ze("MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+i)}}function qn(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return s/4294967295;case Uint16Array:return s/65535;case Uint8Array:case Uint8ClampedArray:return s/255;case Int32Array:return Math.max(s/2147483647,-1);case Int16Array:return Math.max(s/32767,-1);case Int8Array:return Math.max(s/127,-1);default:throw new Error("THREE.MathUtils: Invalid component type.")}}function At(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return Math.round(s*4294967295);case Uint16Array:return Math.round(s*65535);case Uint8Array:case Uint8ClampedArray:return Math.round(s*255);case Int32Array:return Math.round(s*2147483647);case Int16Array:return Math.round(s*32767);case Int8Array:return Math.round(s*127);default:throw new Error("THREE.MathUtils: Invalid component type.")}}var _t={DEG2RAD:jr,RAD2DEG:Ms,generateUUID:Zn,clamp:ut,euclideanModulo:Rh,mapLinear:jp,inverseLerp:Jp,lerp:Jr,damp:$p,pingpong:Qp,smoothstep:em,smootherstep:tm,randInt:nm,randFloat:im,randFloatSpread:sm,seededRandom:rm,degToRad:om,radToDeg:am,isPowerOfTwo:lm,ceilPowerOfTwo:cm,floorPowerOfTwo:hm,setQuaternionFromProperEuler:um,normalize:At,denormalize:qn},Ue=class s{static{s.prototype.isVector2=!0}constructor(e=0,t=0){this.x=e,this.y=t}get width(){return this.x}set width(e){this.x=e}get height(){return this.y}set height(e){this.y=e}set(e,t){return this.x=e,this.y=t,this}setScalar(e){return this.x=e,this.y=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;default:throw new Error("THREE.Vector2: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;default:throw new Error("THREE.Vector2: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y)}copy(e){return this.x=e.x,this.y=e.y,this}add(e){return this.x+=e.x,this.y+=e.y,this}addScalar(e){return this.x+=e,this.y+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this}subScalar(e){return this.x-=e,this.y-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this}multiply(e){return this.x*=e.x,this.y*=e.y,this}multiplyScalar(e){return this.x*=e,this.y*=e,this}divide(e){return this.x/=e.x,this.y/=e.y,this}divideScalar(e){return this.multiplyScalar(1/e)}applyMatrix3(e){let t=this.x,n=this.y,i=e.elements;return this.x=i[0]*t+i[3]*n+i[6],this.y=i[1]*t+i[4]*n+i[7],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this}clamp(e,t){return this.x=ut(this.x,e.x,t.x),this.y=ut(this.y,e.y,t.y),this}clampScalar(e,t){return this.x=ut(this.x,e,t),this.y=ut(this.y,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(ut(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(e){return this.x*e.x+this.y*e.y}cross(e){return this.x*e.y-this.y*e.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(ut(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y;return t*t+n*n}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this}equals(e){return e.x===this.x&&e.y===this.y}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this}rotateAround(e,t){let n=Math.cos(t),i=Math.sin(t),r=this.x-e.x,o=this.y-e.y;return this.x=r*n-o*i+e.x,this.y=r*i+o*n+e.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},Ht=class{constructor(e=0,t=0,n=0,i=1){this.isQuaternion=!0,this._x=e,this._y=t,this._z=n,this._w=i}static slerpFlat(e,t,n,i,r,o,a){let l=n[i+0],c=n[i+1],h=n[i+2],u=n[i+3],d=r[o+0],f=r[o+1],g=r[o+2],v=r[o+3];if(u!==v||l!==d||c!==f||h!==g){let m=l*d+c*f+h*g+u*v;m<0&&(d=-d,f=-f,g=-g,v=-v,m=-m);let p=1-a;if(m<.9995){let M=Math.acos(m),T=Math.sin(M);p=Math.sin(p*M)/T,a=Math.sin(a*M)/T,l=l*p+d*a,c=c*p+f*a,h=h*p+g*a,u=u*p+v*a}else{l=l*p+d*a,c=c*p+f*a,h=h*p+g*a,u=u*p+v*a;let M=1/Math.sqrt(l*l+c*c+h*h+u*u);l*=M,c*=M,h*=M,u*=M}}e[t]=l,e[t+1]=c,e[t+2]=h,e[t+3]=u}static multiplyQuaternionsFlat(e,t,n,i,r,o){let a=n[i],l=n[i+1],c=n[i+2],h=n[i+3],u=r[o],d=r[o+1],f=r[o+2],g=r[o+3];return e[t]=a*g+h*u+l*f-c*d,e[t+1]=l*g+h*d+c*u-a*f,e[t+2]=c*g+h*f+a*d-l*u,e[t+3]=h*g-a*u-l*d-c*f,e}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get w(){return this._w}set w(e){this._w=e,this._onChangeCallback()}set(e,t,n,i){return this._x=e,this._y=t,this._z=n,this._w=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(e){return this._x=e.x,this._y=e.y,this._z=e.z,this._w=e.w,this._onChangeCallback(),this}setFromEuler(e,t=!0){let n=e._x,i=e._y,r=e._z,o=e._order,a=Math.cos,l=Math.sin,c=a(n/2),h=a(i/2),u=a(r/2),d=l(n/2),f=l(i/2),g=l(r/2);switch(o){case"XYZ":this._x=d*h*u+c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u-d*f*g;break;case"YXZ":this._x=d*h*u+c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u+d*f*g;break;case"ZXY":this._x=d*h*u-c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u-d*f*g;break;case"ZYX":this._x=d*h*u-c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u+d*f*g;break;case"YZX":this._x=d*h*u+c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u-d*f*g;break;case"XZY":this._x=d*h*u-c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u+d*f*g;break;default:Ze("Quaternion: .setFromEuler() encountered an unknown order: "+o)}return t===!0&&this._onChangeCallback(),this}setFromAxisAngle(e,t){let n=t/2,i=Math.sin(n);return this._x=e.x*i,this._y=e.y*i,this._z=e.z*i,this._w=Math.cos(n),this._onChangeCallback(),this}setFromRotationMatrix(e){let t=e.elements,n=t[0],i=t[4],r=t[8],o=t[1],a=t[5],l=t[9],c=t[2],h=t[6],u=t[10],d=n+a+u;if(d>0){let f=.5/Math.sqrt(d+1);this._w=.25/f,this._x=(h-l)*f,this._y=(r-c)*f,this._z=(o-i)*f}else if(n>a&&n>u){let f=2*Math.sqrt(1+n-a-u);this._w=(h-l)/f,this._x=.25*f,this._y=(i+o)/f,this._z=(r+c)/f}else if(a>u){let f=2*Math.sqrt(1+a-n-u);this._w=(r-c)/f,this._x=(i+o)/f,this._y=.25*f,this._z=(l+h)/f}else{let f=2*Math.sqrt(1+u-n-a);this._w=(o-i)/f,this._x=(r+c)/f,this._y=(l+h)/f,this._z=.25*f}return this._onChangeCallback(),this}setFromUnitVectors(e,t){let n=e.dot(t)+1;return n<1e-8?(n=0,Math.abs(e.x)>Math.abs(e.z)?(this._x=-e.y,this._y=e.x,this._z=0,this._w=n):(this._x=0,this._y=-e.z,this._z=e.y,this._w=n)):(this._x=e.y*t.z-e.z*t.y,this._y=e.z*t.x-e.x*t.z,this._z=e.x*t.y-e.y*t.x,this._w=n),this.normalize()}angleTo(e){return 2*Math.acos(Math.abs(ut(this.dot(e),-1,1)))}rotateTowards(e,t){let n=this.angleTo(e);if(n===0)return this;let i=Math.min(1,t/n);return this.slerp(e,i),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(e){return this._x*e._x+this._y*e._y+this._z*e._z+this._w*e._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let e=this.length();return e===0?(this._x=0,this._y=0,this._z=0,this._w=1):(e=1/e,this._x=this._x*e,this._y=this._y*e,this._z=this._z*e,this._w=this._w*e),this._onChangeCallback(),this}multiply(e){return this.multiplyQuaternions(this,e)}premultiply(e){return this.multiplyQuaternions(e,this)}multiplyQuaternions(e,t){let n=e._x,i=e._y,r=e._z,o=e._w,a=t._x,l=t._y,c=t._z,h=t._w;return this._x=n*h+o*a+i*c-r*l,this._y=i*h+o*l+r*a-n*c,this._z=r*h+o*c+n*l-i*a,this._w=o*h-n*a-i*l-r*c,this._onChangeCallback(),this}slerp(e,t){let n=e._x,i=e._y,r=e._z,o=e._w,a=this.dot(e);a<0&&(n=-n,i=-i,r=-r,o=-o,a=-a);let l=1-t;if(a<.9995){let c=Math.acos(a),h=Math.sin(c);l=Math.sin(l*c)/h,t=Math.sin(t*c)/h,this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+o*t,this._onChangeCallback()}else this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+o*t,this.normalize();return this}slerpQuaternions(e,t,n){return this.copy(e).slerp(t,n)}random(){let e=2*Math.PI*Math.random(),t=2*Math.PI*Math.random(),n=Math.random(),i=Math.sqrt(1-n),r=Math.sqrt(n);return this.set(i*Math.sin(e),i*Math.cos(e),r*Math.sin(t),r*Math.cos(t))}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._w===this._w}fromArray(e,t=0){return this._x=e[t],this._y=e[t+1],this._z=e[t+2],this._w=e[t+3],this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._w,e}fromBufferAttribute(e,t){return this._x=e.getX(t),this._y=e.getY(t),this._z=e.getZ(t),this._w=e.getW(t),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},W=class s{static{s.prototype.isVector3=!0}constructor(e=0,t=0,n=0){this.x=e,this.y=t,this.z=n}set(e,t,n){return n===void 0&&(n=this.z),this.x=e,this.y=t,this.z=n,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;default:throw new Error("THREE.Vector3: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("THREE.Vector3: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this}multiplyVectors(e,t){return this.x=e.x*t.x,this.y=e.y*t.y,this.z=e.z*t.z,this}applyEuler(e){return this.applyQuaternion(Gu.setFromEuler(e))}applyAxisAngle(e,t){return this.applyQuaternion(Gu.setFromAxisAngle(e,t))}applyMatrix3(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[3]*n+r[6]*i,this.y=r[1]*t+r[4]*n+r[7]*i,this.z=r[2]*t+r[5]*n+r[8]*i,this}applyNormalMatrix(e){return this.applyMatrix3(e).normalize()}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=e.elements,o=1/(r[3]*t+r[7]*n+r[11]*i+r[15]);return this.x=(r[0]*t+r[4]*n+r[8]*i+r[12])*o,this.y=(r[1]*t+r[5]*n+r[9]*i+r[13])*o,this.z=(r[2]*t+r[6]*n+r[10]*i+r[14])*o,this}applyQuaternion(e){let t=this.x,n=this.y,i=this.z,r=e.x,o=e.y,a=e.z,l=e.w,c=2*(o*i-a*n),h=2*(a*t-r*i),u=2*(r*n-o*t);return this.x=t+l*c+o*u-a*h,this.y=n+l*h+a*c-r*u,this.z=i+l*u+r*h-o*c,this}project(e){return this.applyMatrix4(e.matrixWorldInverse).applyMatrix4(e.projectionMatrix)}unproject(e){return this.applyMatrix4(e.projectionMatrixInverse).applyMatrix4(e.matrixWorld)}transformDirection(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[4]*n+r[8]*i,this.y=r[1]*t+r[5]*n+r[9]*i,this.z=r[2]*t+r[6]*n+r[10]*i,this.normalize()}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this}divideScalar(e){return this.multiplyScalar(1/e)}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this}clamp(e,t){return this.x=ut(this.x,e.x,t.x),this.y=ut(this.y,e.y,t.y),this.z=ut(this.z,e.z,t.z),this}clampScalar(e,t){return this.x=ut(this.x,e,t),this.y=ut(this.y,e,t),this.z=ut(this.z,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(ut(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this}cross(e){return this.crossVectors(this,e)}crossVectors(e,t){let n=e.x,i=e.y,r=e.z,o=t.x,a=t.y,l=t.z;return this.x=i*l-r*a,this.y=r*o-n*l,this.z=n*a-i*o,this}projectOnVector(e){let t=e.lengthSq();if(t===0)return this.set(0,0,0);let n=e.dot(this)/t;return this.copy(e).multiplyScalar(n)}projectOnPlane(e){return Cc.copy(this).projectOnVector(e),this.sub(Cc)}reflect(e){return this.sub(Cc.copy(e).multiplyScalar(2*this.dot(e)))}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(ut(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y,i=this.z-e.z;return t*t+n*n+i*i}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)+Math.abs(this.z-e.z)}setFromSpherical(e){return this.setFromSphericalCoords(e.radius,e.phi,e.theta)}setFromSphericalCoords(e,t,n){let i=Math.sin(t)*e;return this.x=i*Math.sin(n),this.y=Math.cos(t)*e,this.z=i*Math.cos(n),this}setFromCylindrical(e){return this.setFromCylindricalCoords(e.radius,e.theta,e.y)}setFromCylindricalCoords(e,t,n){return this.x=e*Math.sin(t),this.y=n,this.z=e*Math.cos(t),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this}setFromMatrixScale(e){let t=this.setFromMatrixColumn(e,0).length(),n=this.setFromMatrixColumn(e,1).length(),i=this.setFromMatrixColumn(e,2).length();return this.x=t,this.y=n,this.z=i,this}setFromMatrixColumn(e,t){return this.fromArray(e.elements,t*4)}setFromMatrix3Column(e,t){return this.fromArray(e.elements,t*3)}setFromEuler(e){return this.x=e._x,this.y=e._y,this.z=e._z,this}setFromColor(e){return this.x=e.r,this.y=e.g,this.z=e.b,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let e=Math.random()*Math.PI*2,t=Math.random()*2-1,n=Math.sqrt(1-t*t);return this.x=n*Math.cos(e),this.y=t,this.z=n*Math.sin(e),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},Cc=new W,Gu=new Ht,rt=class s{static{s.prototype.isMatrix3=!0}constructor(e,t,n,i,r,o,a,l,c){this.elements=[1,0,0,0,1,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,o,a,l,c)}set(e,t,n,i,r,o,a,l,c){let h=this.elements;return h[0]=e,h[1]=i,h[2]=a,h[3]=t,h[4]=r,h[5]=l,h[6]=n,h[7]=o,h[8]=c,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],this}extractBasis(e,t,n){return e.setFromMatrix3Column(this,0),t.setFromMatrix3Column(this,1),n.setFromMatrix3Column(this,2),this}setFromMatrix4(e){let t=e.elements;return this.set(t[0],t[4],t[8],t[1],t[5],t[9],t[2],t[6],t[10]),this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,o=n[0],a=n[3],l=n[6],c=n[1],h=n[4],u=n[7],d=n[2],f=n[5],g=n[8],v=i[0],m=i[3],p=i[6],M=i[1],T=i[4],x=i[7],y=i[2],S=i[5],C=i[8];return r[0]=o*v+a*M+l*y,r[3]=o*m+a*T+l*S,r[6]=o*p+a*x+l*C,r[1]=c*v+h*M+u*y,r[4]=c*m+h*T+u*S,r[7]=c*p+h*x+u*C,r[2]=d*v+f*M+g*y,r[5]=d*m+f*T+g*S,r[8]=d*p+f*x+g*C,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[3]*=e,t[6]*=e,t[1]*=e,t[4]*=e,t[7]*=e,t[2]*=e,t[5]*=e,t[8]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8];return t*o*h-t*a*c-n*r*h+n*a*l+i*r*c-i*o*l}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8],u=h*o-a*c,d=a*l-h*r,f=c*r-o*l,g=t*u+n*d+i*f;if(g===0)return this.set(0,0,0,0,0,0,0,0,0);let v=1/g;return e[0]=u*v,e[1]=(i*c-h*n)*v,e[2]=(a*n-i*o)*v,e[3]=d*v,e[4]=(h*t-i*l)*v,e[5]=(i*r-a*t)*v,e[6]=f*v,e[7]=(n*l-c*t)*v,e[8]=(o*t-n*r)*v,this}transpose(){let e,t=this.elements;return e=t[1],t[1]=t[3],t[3]=e,e=t[2],t[2]=t[6],t[6]=e,e=t[5],t[5]=t[7],t[7]=e,this}getNormalMatrix(e){return this.setFromMatrix4(e).invert().transpose()}transposeIntoArray(e){let t=this.elements;return e[0]=t[0],e[1]=t[3],e[2]=t[6],e[3]=t[1],e[4]=t[4],e[5]=t[7],e[6]=t[2],e[7]=t[5],e[8]=t[8],this}setUvTransform(e,t,n,i,r,o,a){let l=Math.cos(r),c=Math.sin(r);return this.set(n*l,n*c,-n*(l*o+c*a)+o+e,-i*c,i*l,-i*(-c*o+l*a)+a+t,0,0,1),this}scale(e,t){return _s("Matrix3: .scale() is deprecated. Use .makeScale() instead."),this.premultiply(Pc.makeScale(e,t)),this}rotate(e){return _s("Matrix3: .rotate() is deprecated. Use .makeRotation() instead."),this.premultiply(Pc.makeRotation(-e)),this}translate(e,t){return _s("Matrix3: .translate() is deprecated. Use .makeTranslation() instead."),this.premultiply(Pc.makeTranslation(e,t)),this}makeTranslation(e,t){return e.isVector2?this.set(1,0,e.x,0,1,e.y,0,0,1):this.set(1,0,e,0,1,t,0,0,1),this}makeRotation(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,n,t,0,0,0,1),this}makeScale(e,t){return this.set(e,0,0,0,t,0,0,0,1),this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<9;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<9;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e}clone(){return new this.constructor().fromArray(this.elements)}},Pc=new rt,Wu=new rt().set(.4123908,.3575843,.1804808,.212639,.7151687,.0721923,.0193308,.1191948,.9505322),Xu=new rt().set(3.2409699,-1.5373832,-.4986108,-.9692436,1.8759675,.0415551,.0556301,-.203977,1.0569715);function dm(){let s={enabled:!0,workingColorSpace:mn,spaces:{},convert:function(i,r,o){return this.enabled===!1||r===o||!r||!o||(this.spaces[r].transfer===St&&(i.r=Ai(i.r),i.g=Ai(i.g),i.b=Ai(i.b)),this.spaces[r].primaries!==this.spaces[o].primaries&&(i.applyMatrix3(this.spaces[r].toXYZ),i.applyMatrix3(this.spaces[o].fromXYZ)),this.spaces[o].transfer===St&&(i.r=er(i.r),i.g=er(i.g),i.b=er(i.b))),i},workingToColorSpace:function(i,r){return this.convert(i,this.workingColorSpace,r)},colorSpaceToWorking:function(i,r){return this.convert(i,r,this.workingColorSpace)},getPrimaries:function(i){return this.spaces[i].primaries},getTransfer:function(i){return i===kn?to:this.spaces[i].transfer},getToneMappingMode:function(i){return this.spaces[i].outputColorSpaceConfig.toneMappingMode||"standard"},getLuminanceCoefficients:function(i,r=this.workingColorSpace){return i.fromArray(this.spaces[r].luminanceCoefficients)},define:function(i){Object.assign(this.spaces,i)},_getMatrix:function(i,r,o){return i.copy(this.spaces[r].toXYZ).multiply(this.spaces[o].fromXYZ)},_getDrawingBufferColorSpace:function(i){return this.spaces[i].outputColorSpaceConfig.drawingBufferColorSpace},_getUnpackColorSpace:function(i=this.workingColorSpace){return this.spaces[i].workingColorSpaceConfig.unpackColorSpace},fromWorkingColorSpace:function(i,r){return _s("ColorManagement: .fromWorkingColorSpace() has been renamed to .workingToColorSpace()."),s.workingToColorSpace(i,r)},toWorkingColorSpace:function(i,r){return _s("ColorManagement: .toWorkingColorSpace() has been renamed to .colorSpaceToWorking()."),s.colorSpaceToWorking(i,r)}},e=[.64,.33,.3,.6,.15,.06],t=[.2126,.7152,.0722],n=[.3127,.329];return s.define({[mn]:{primaries:e,whitePoint:n,transfer:to,toXYZ:Wu,fromXYZ:Xu,luminanceCoefficients:t,workingColorSpaceConfig:{unpackColorSpace:Ft},outputColorSpaceConfig:{drawingBufferColorSpace:Ft}},[Ft]:{primaries:e,whitePoint:n,transfer:St,toXYZ:Wu,fromXYZ:Xu,luminanceCoefficients:t,outputColorSpaceConfig:{drawingBufferColorSpace:Ft}}}),s}var ht=dm();function Ai(s){return s<.04045?s*.0773993808:Math.pow(s*.9478672986+.0521327014,2.4)}function er(s){return s<.0031308?s*12.92:1.055*Math.pow(s,.41666)-.055}var ks,Ha=class{static getDataURL(e,t="image/png"){if(/^data:/i.test(e.src)||typeof HTMLCanvasElement>"u")return e.src;let n;if(e instanceof HTMLCanvasElement)n=e;else{ks===void 0&&(ks=sr("canvas")),ks.width=e.width,ks.height=e.height;let i=ks.getContext("2d");e instanceof ImageData?i.putImageData(e,0,0):i.drawImage(e,0,0,e.width,e.height),n=ks}return n.toDataURL(t)}static sRGBToLinear(e){if(typeof HTMLImageElement<"u"&&e instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&e instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&e instanceof ImageBitmap){let t=sr("canvas");t.width=e.width,t.height=e.height;let n=t.getContext("2d");n.drawImage(e,0,0,e.width,e.height);let i=n.getImageData(0,0,e.width,e.height),r=i.data;for(let o=0;o<r.length;o++)r[o]=Ai(r[o]/255)*255;return n.putImageData(i,0,0),t}else if(e.data){let t=e.data.slice(0);for(let n=0;n<t.length;n++)t instanceof Uint8Array||t instanceof Uint8ClampedArray?t[n]=Math.floor(Ai(t[n]/255)*255):t[n]=Ai(t[n]);return{data:t,width:e.width,height:e.height}}else return Ze("ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),e}},fm=0,or=class{constructor(e=null){this.isTextureSource=!0,Object.defineProperty(this,"id",{value:fm++}),this.uuid=Zn(),this.data=e,this.dataReady=!0,this.version=0}getSize(e){let t=this.data;return typeof HTMLVideoElement<"u"&&t instanceof HTMLVideoElement?e.set(t.videoWidth,t.videoHeight,0):typeof VideoFrame<"u"&&t instanceof VideoFrame?e.set(t.displayWidth,t.displayHeight,0):t!==null?e.set(t.width,t.height,t.depth||0):e.set(0,0,0),e}set needsUpdate(e){e===!0&&this.version++}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.images[this.uuid]!==void 0)return e.images[this.uuid];let n={uuid:this.uuid,url:""},i=this.data;if(i!==null){let r;if(Array.isArray(i)){r=[];for(let o=0,a=i.length;o<a;o++)i[o].isDataTexture?r.push(Ic(i[o].image)):r.push(Ic(i[o]))}else r=Ic(i);n.url=r}return t||(e.images[this.uuid]=n),n}};function Ic(s){return typeof HTMLImageElement<"u"&&s instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&s instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&s instanceof ImageBitmap?Ha.getDataURL(s):s.data?{data:Array.from(s.data),width:s.width,height:s.height,type:s.data.constructor.name}:(Ze("Texture: Unable to serialize Texture."),{})}var pm=0,Lc=new W,Gt=class s extends On{constructor(e=s.DEFAULT_IMAGE,t=s.DEFAULT_MAPPING,n=Fn,i=Fn,r=Ot,o=$n,a=In,l=bn,c=s.DEFAULT_ANISOTROPY,h=kn){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:pm++}),this.uuid=Zn(),this.name="",this.source=new or(e),this.mipmaps=[],this.mapping=t,this.channel=0,this.wrapS=n,this.wrapT=i,this.magFilter=r,this.minFilter=o,this.anisotropy=c,this.format=a,this.internalFormat=null,this.type=l,this.offset=new Ue(0,0),this.repeat=new Ue(1,1),this.center=new Ue(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new rt,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=h,this.userData={},this.updateRanges=[],this.version=0,this.onUpdate=null,this.renderTarget=null,this.isRenderTargetTexture=!1,this.isArrayTexture=!!(e&&e.depth&&e.depth>1),this.pmremVersion=0,this.normalized=!1}get width(){return this.source.getSize(Lc).x}get height(){return this.source.getSize(Lc).y}get depth(){return this.source.getSize(Lc).z}get image(){return this.source.data}set image(e){this.source.data=e}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}clone(){return new this.constructor().copy(this)}copy(e){return this.name=e.name,this.source=e.source,this.mipmaps=e.mipmaps.slice(0),this.mapping=e.mapping,this.channel=e.channel,this.wrapS=e.wrapS,this.wrapT=e.wrapT,this.magFilter=e.magFilter,this.minFilter=e.minFilter,this.anisotropy=e.anisotropy,this.format=e.format,this.internalFormat=e.internalFormat,this.type=e.type,this.normalized=e.normalized,this.offset.copy(e.offset),this.repeat.copy(e.repeat),this.center.copy(e.center),this.rotation=e.rotation,this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrix.copy(e.matrix),this.generateMipmaps=e.generateMipmaps,this.premultiplyAlpha=e.premultiplyAlpha,this.flipY=e.flipY,this.unpackAlignment=e.unpackAlignment,this.colorSpace=e.colorSpace,this.renderTarget=e.renderTarget,this.isRenderTargetTexture=e.isRenderTargetTexture,this.isArrayTexture=e.isArrayTexture,this.userData=JSON.parse(JSON.stringify(e.userData)),this.needsUpdate=!0,this}setValues(e){for(let t in e){let n=e[t];if(n===void 0){Ze(`Texture.setValues(): parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){Ze(`Texture.setValues(): property '${t}' does not exist.`);continue}i&&n&&i.isVector2&&n.isVector2||i&&n&&i.isVector3&&n.isVector3||i&&n&&i.isMatrix3&&n.isMatrix3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.textures[this.uuid]!==void 0)return e.textures[this.uuid];let n={metadata:{version:4.7,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(e).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,normalized:this.normalized,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(n.userData=this.userData),t||(e.textures[this.uuid]=n),n}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(e){if(this.mapping!==_h)return e;if(e.applyMatrix3(this.matrix),e.x<0||e.x>1)switch(this.wrapS){case ai:e.x=e.x-Math.floor(e.x);break;case Fn:e.x=e.x<0?0:1;break;case nr:Math.abs(Math.floor(e.x)%2)===1?e.x=Math.ceil(e.x)-e.x:e.x=e.x-Math.floor(e.x);break}if(e.y<0||e.y>1)switch(this.wrapT){case ai:e.y=e.y-Math.floor(e.y);break;case Fn:e.y=e.y<0?0:1;break;case nr:Math.abs(Math.floor(e.y)%2)===1?e.y=Math.ceil(e.y)-e.y:e.y=e.y-Math.floor(e.y);break}return this.flipY&&(e.y=1-e.y),e}set needsUpdate(e){e===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(e){e===!0&&this.pmremVersion++}};Gt.DEFAULT_IMAGE=null;Gt.DEFAULT_MAPPING=_h;Gt.DEFAULT_ANISOTROPY=1;var gt=class s{static{s.prototype.isVector4=!0}constructor(e=0,t=0,n=0,i=1){this.x=e,this.y=t,this.z=n,this.w=i}get width(){return this.z}set width(e){this.z=e}get height(){return this.w}set height(e){this.w=e}set(e,t,n,i){return this.x=e,this.y=t,this.z=n,this.w=i,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this.w=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setW(e){return this.w=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;case 3:this.w=t;break;default:throw new Error("THREE.Vector4: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("THREE.Vector4: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this.w=e.w!==void 0?e.w:1,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this.w+=e.w,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this.w+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this.w=e.w+t.w,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this.w+=e.w*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this.w-=e.w,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this.w-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this.w=e.w-t.w,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this.w*=e.w,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this.w*=e,this}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=this.w,o=e.elements;return this.x=o[0]*t+o[4]*n+o[8]*i+o[12]*r,this.y=o[1]*t+o[5]*n+o[9]*i+o[13]*r,this.z=o[2]*t+o[6]*n+o[10]*i+o[14]*r,this.w=o[3]*t+o[7]*n+o[11]*i+o[15]*r,this}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this.w/=e.w,this}divideScalar(e){return this.multiplyScalar(1/e)}setAxisAngleFromQuaternion(e){this.w=2*Math.acos(e.w);let t=Math.sqrt(1-e.w*e.w);return t<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=e.x/t,this.y=e.y/t,this.z=e.z/t),this}setAxisAngleFromRotationMatrix(e){let t,n,i,r,l=e.elements,c=l[0],h=l[4],u=l[8],d=l[1],f=l[5],g=l[9],v=l[2],m=l[6],p=l[10];if(Math.abs(h-d)<.01&&Math.abs(u-v)<.01&&Math.abs(g-m)<.01){if(Math.abs(h+d)<.1&&Math.abs(u+v)<.1&&Math.abs(g+m)<.1&&Math.abs(c+f+p-3)<.1)return this.set(1,0,0,0),this;t=Math.PI;let T=(c+1)/2,x=(f+1)/2,y=(p+1)/2,S=(h+d)/4,C=(u+v)/4,_=(g+m)/4;return T>x&&T>y?T<.01?(n=0,i=.707106781,r=.707106781):(n=Math.sqrt(T),i=S/n,r=C/n):x>y?x<.01?(n=.707106781,i=0,r=.707106781):(i=Math.sqrt(x),n=S/i,r=_/i):y<.01?(n=.707106781,i=.707106781,r=0):(r=Math.sqrt(y),n=C/r,i=_/r),this.set(n,i,r,t),this}let M=Math.sqrt((m-g)*(m-g)+(u-v)*(u-v)+(d-h)*(d-h));return Math.abs(M)<.001&&(M=1),this.x=(m-g)/M,this.y=(u-v)/M,this.z=(d-h)/M,this.w=Math.acos((c+f+p-1)/2),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this.w=t[15],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this.w=Math.min(this.w,e.w),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this.w=Math.max(this.w,e.w),this}clamp(e,t){return this.x=ut(this.x,e.x,t.x),this.y=ut(this.y,e.y,t.y),this.z=ut(this.z,e.z,t.z),this.w=ut(this.w,e.w,t.w),this}clampScalar(e,t){return this.x=ut(this.x,e,t),this.y=ut(this.y,e,t),this.z=ut(this.z,e,t),this.w=ut(this.w,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(ut(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z+this.w*e.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this.w+=(e.w-this.w)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this.w=e.w+(t.w-e.w)*n,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z&&e.w===this.w}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this.w=e[t+3],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e[t+3]=this.w,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this.w=e.getW(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},Va=class extends On{constructor(e=1,t=1,n={}){super(),n=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:Ot,depthBuffer:!0,stencilBuffer:!1,resolveColorBuffer:!0,resolveDepthBuffer:!0,resolveStencilBuffer:!0,storeMultisampledColorBuffer:!0,storeMultisampledDepthBuffer:!0,storeMultisampledStencilBuffer:!0,depthTexture:null,samples:0,count:1,depth:1,multiview:!1,useArrayDepthTexture:!1},n),this.isRenderTarget=!0,this.width=e,this.height=t,this.depth=n.depth,this.scissor=new gt(0,0,e,t),this.scissorTest=!1,this.viewport=new gt(0,0,e,t),this.textures=[];let i={width:e,height:t,depth:n.depth},r=new Gt(i),o=n.count;for(let a=0;a<o;a++)this.textures[a]=r.clone(),this.textures[a].isRenderTargetTexture=!0,this.textures[a].renderTarget=this;this._setTextureOptions(n),this.depthBuffer=n.depthBuffer,this.stencilBuffer=n.stencilBuffer,this.resolveColorBuffer=n.resolveColorBuffer,this.resolveDepthBuffer=n.resolveDepthBuffer,this.resolveStencilBuffer=n.resolveStencilBuffer,this.storeMultisampledColorBuffer=n.storeMultisampledColorBuffer,this.storeMultisampledDepthBuffer=n.storeMultisampledDepthBuffer,this.storeMultisampledStencilBuffer=n.storeMultisampledStencilBuffer,this._depthTexture=null,this.depthTexture=n.depthTexture,this.samples=n.samples,this.multiview=n.multiview,this.useArrayDepthTexture=n.useArrayDepthTexture}_setTextureOptions(e={}){let t={minFilter:Ot,generateMipmaps:!1,flipY:!1,internalFormat:null};e.mapping!==void 0&&(t.mapping=e.mapping),e.wrapS!==void 0&&(t.wrapS=e.wrapS),e.wrapT!==void 0&&(t.wrapT=e.wrapT),e.wrapR!==void 0&&(t.wrapR=e.wrapR),e.magFilter!==void 0&&(t.magFilter=e.magFilter),e.minFilter!==void 0&&(t.minFilter=e.minFilter),e.format!==void 0&&(t.format=e.format),e.type!==void 0&&(t.type=e.type),e.anisotropy!==void 0&&(t.anisotropy=e.anisotropy),e.colorSpace!==void 0&&(t.colorSpace=e.colorSpace),e.flipY!==void 0&&(t.flipY=e.flipY),e.generateMipmaps!==void 0&&(t.generateMipmaps=e.generateMipmaps),e.internalFormat!==void 0&&(t.internalFormat=e.internalFormat);for(let n=0;n<this.textures.length;n++)this.textures[n].setValues(t)}get texture(){return this.textures[0]}set texture(e){this.textures[0]=e}set depthTexture(e){this._depthTexture!==null&&this._depthTexture.renderTarget===this&&(this._depthTexture.renderTarget=null),e!==null&&e.renderTarget===null&&(e.renderTarget=this),this._depthTexture=e}get depthTexture(){return this._depthTexture}setSize(e,t,n=1){if(this.width!==e||this.height!==t||this.depth!==n){this.width=e,this.height=t,this.depth=n;for(let i=0,r=this.textures.length;i<r;i++)this.textures[i].image.width=e,this.textures[i].image.height=t,this.textures[i].image.depth=n,this.textures[i].isData3DTexture!==!0&&(this.textures[i].isArrayTexture=this.textures[i].image.depth>1);this.dispose()}this.viewport.set(0,0,e,t),this.scissor.set(0,0,e,t)}clone(){return new this.constructor().copy(this)}copy(e){this.width=e.width,this.height=e.height,this.depth=e.depth,this.scissor.copy(e.scissor),this.scissorTest=e.scissorTest,this.viewport.copy(e.viewport),this.textures.length=0;for(let t=0,n=e.textures.length;t<n;t++){this.textures[t]=e.textures[t].clone(),this.textures[t].isRenderTargetTexture=!0,this.textures[t].renderTarget=this;let i=Object.assign({},e.textures[t].image);this.textures[t].source=new or(i)}if(this.depthBuffer=e.depthBuffer,this.stencilBuffer=e.stencilBuffer,this.resolveColorBuffer=e.resolveColorBuffer,this.resolveDepthBuffer=e.resolveDepthBuffer,this.resolveStencilBuffer=e.resolveStencilBuffer,this.storeMultisampledColorBuffer=e.storeMultisampledColorBuffer,this.storeMultisampledDepthBuffer=e.storeMultisampledDepthBuffer,this.storeMultisampledStencilBuffer=e.storeMultisampledStencilBuffer,e.depthTexture!==null)if(e.depthTexture.renderTarget===e){let t=e.depthTexture.clone();t.renderTarget=null,this.depthTexture=t}else this.depthTexture=e.depthTexture;return this.samples=e.samples,this.multiview=e.multiview,this.useArrayDepthTexture=e.useArrayDepthTexture,this}dispose(){this.dispatchEvent({type:"dispose"})}},Bt=class extends Va{constructor(e=1,t=1,n={}){super(e,t,n),this.isWebGLRenderTarget=!0}},io=class extends Gt{constructor(e=null,t=1,n=1,i=1){super(null),this.isDataArrayTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=Vt,this.minFilter=Vt,this.wrapR=Fn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}copy(e){return super.copy(e),this.wrapR=e.wrapR,this}addLayerUpdate(e){this.layerUpdates.add(e)}clearLayerUpdates(){this.layerUpdates.clear()}};var Ga=class extends Gt{constructor(e=null,t=1,n=1,i=1){super(null),this.isData3DTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=Vt,this.minFilter=Vt,this.wrapR=Fn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}copy(e){return super.copy(e),this.wrapR=e.wrapR,this}};var it=class s{static{s.prototype.isMatrix4=!0}constructor(e,t,n,i,r,o,a,l,c,h,u,d,f,g,v,m){this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,o,a,l,c,h,u,d,f,g,v,m)}set(e,t,n,i,r,o,a,l,c,h,u,d,f,g,v,m){let p=this.elements;return p[0]=e,p[4]=t,p[8]=n,p[12]=i,p[1]=r,p[5]=o,p[9]=a,p[13]=l,p[2]=c,p[6]=h,p[10]=u,p[14]=d,p[3]=f,p[7]=g,p[11]=v,p[15]=m,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new s().fromArray(this.elements)}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],t[9]=n[9],t[10]=n[10],t[11]=n[11],t[12]=n[12],t[13]=n[13],t[14]=n[14],t[15]=n[15],this}copyPosition(e){let t=this.elements,n=e.elements;return t[12]=n[12],t[13]=n[13],t[14]=n[14],this}setFromMatrix3(e){let t=e.elements;return this.set(t[0],t[3],t[6],0,t[1],t[4],t[7],0,t[2],t[5],t[8],0,0,0,0,1),this}extractBasis(e,t,n){return this.determinantAffine()===0?(e.set(1,0,0),t.set(0,1,0),n.set(0,0,1),this):(e.setFromMatrixColumn(this,0),t.setFromMatrixColumn(this,1),n.setFromMatrixColumn(this,2),this)}makeBasis(e,t,n){return this.set(e.x,t.x,n.x,0,e.y,t.y,n.y,0,e.z,t.z,n.z,0,0,0,0,1),this}extractRotation(e){if(e.determinantAffine()===0)return this.identity();let t=this.elements,n=e.elements,i=1/Hs.setFromMatrixColumn(e,0).length(),r=1/Hs.setFromMatrixColumn(e,1).length(),o=1/Hs.setFromMatrixColumn(e,2).length();return t[0]=n[0]*i,t[1]=n[1]*i,t[2]=n[2]*i,t[3]=0,t[4]=n[4]*r,t[5]=n[5]*r,t[6]=n[6]*r,t[7]=0,t[8]=n[8]*o,t[9]=n[9]*o,t[10]=n[10]*o,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromEuler(e){let t=this.elements,n=e.x,i=e.y,r=e.z,o=Math.cos(n),a=Math.sin(n),l=Math.cos(i),c=Math.sin(i),h=Math.cos(r),u=Math.sin(r);if(e.order==="XYZ"){let d=o*h,f=o*u,g=a*h,v=a*u;t[0]=l*h,t[4]=-l*u,t[8]=c,t[1]=f+g*c,t[5]=d-v*c,t[9]=-a*l,t[2]=v-d*c,t[6]=g+f*c,t[10]=o*l}else if(e.order==="YXZ"){let d=l*h,f=l*u,g=c*h,v=c*u;t[0]=d+v*a,t[4]=g*a-f,t[8]=o*c,t[1]=o*u,t[5]=o*h,t[9]=-a,t[2]=f*a-g,t[6]=v+d*a,t[10]=o*l}else if(e.order==="ZXY"){let d=l*h,f=l*u,g=c*h,v=c*u;t[0]=d-v*a,t[4]=-o*u,t[8]=g+f*a,t[1]=f+g*a,t[5]=o*h,t[9]=v-d*a,t[2]=-o*c,t[6]=a,t[10]=o*l}else if(e.order==="ZYX"){let d=o*h,f=o*u,g=a*h,v=a*u;t[0]=l*h,t[4]=g*c-f,t[8]=d*c+v,t[1]=l*u,t[5]=v*c+d,t[9]=f*c-g,t[2]=-c,t[6]=a*l,t[10]=o*l}else if(e.order==="YZX"){let d=o*l,f=o*c,g=a*l,v=a*c;t[0]=l*h,t[4]=v-d*u,t[8]=g*u+f,t[1]=u,t[5]=o*h,t[9]=-a*h,t[2]=-c*h,t[6]=f*u+g,t[10]=d-v*u}else if(e.order==="XZY"){let d=o*l,f=o*c,g=a*l,v=a*c;t[0]=l*h,t[4]=-u,t[8]=c*h,t[1]=d*u+v,t[5]=o*h,t[9]=f*u-g,t[2]=g*u-f,t[6]=a*h,t[10]=v*u+d}return t[3]=0,t[7]=0,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromQuaternion(e){return this.compose(mm,e,gm)}lookAt(e,t,n){let i=this.elements;return Tn.subVectors(e,t),Tn.lengthSq()===0&&(Tn.z=1),Tn.normalize(),Gi.crossVectors(n,Tn),Gi.lengthSq()===0&&(Math.abs(n.z)===1?Tn.x+=1e-4:Tn.z+=1e-4,Tn.normalize(),Gi.crossVectors(n,Tn)),Gi.normalize(),oa.crossVectors(Tn,Gi),i[0]=Gi.x,i[4]=oa.x,i[8]=Tn.x,i[1]=Gi.y,i[5]=oa.y,i[9]=Tn.y,i[2]=Gi.z,i[6]=oa.z,i[10]=Tn.z,this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,o=n[0],a=n[4],l=n[8],c=n[12],h=n[1],u=n[5],d=n[9],f=n[13],g=n[2],v=n[6],m=n[10],p=n[14],M=n[3],T=n[7],x=n[11],y=n[15],S=i[0],C=i[4],_=i[8],I=i[12],b=i[1],N=i[5],w=i[9],H=i[13],F=i[2],L=i[6],O=i[10],V=i[14],R=i[3],G=i[7],oe=i[11],k=i[15];return r[0]=o*S+a*b+l*F+c*R,r[4]=o*C+a*N+l*L+c*G,r[8]=o*_+a*w+l*O+c*oe,r[12]=o*I+a*H+l*V+c*k,r[1]=h*S+u*b+d*F+f*R,r[5]=h*C+u*N+d*L+f*G,r[9]=h*_+u*w+d*O+f*oe,r[13]=h*I+u*H+d*V+f*k,r[2]=g*S+v*b+m*F+p*R,r[6]=g*C+v*N+m*L+p*G,r[10]=g*_+v*w+m*O+p*oe,r[14]=g*I+v*H+m*V+p*k,r[3]=M*S+T*b+x*F+y*R,r[7]=M*C+T*N+x*L+y*G,r[11]=M*_+T*w+x*O+y*oe,r[15]=M*I+T*H+x*V+y*k,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[4]*=e,t[8]*=e,t[12]*=e,t[1]*=e,t[5]*=e,t[9]*=e,t[13]*=e,t[2]*=e,t[6]*=e,t[10]*=e,t[14]*=e,t[3]*=e,t[7]*=e,t[11]*=e,t[15]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[12],o=e[1],a=e[5],l=e[9],c=e[13],h=e[2],u=e[6],d=e[10],f=e[14],g=e[3],v=e[7],m=e[11],p=e[15],M=l*f-c*d,T=a*f-c*u,x=a*d-l*u,y=o*f-c*h,S=o*d-l*h,C=o*u-a*h;return t*(v*M-m*T+p*x)-n*(g*M-m*y+p*S)+i*(g*T-v*y+p*C)-r*(g*x-v*S+m*C)}determinantAffine(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[1],o=e[5],a=e[9],l=e[2],c=e[6],h=e[10];return t*(o*h-a*c)-n*(r*h-a*l)+i*(r*c-o*l)}transpose(){let e=this.elements,t;return t=e[1],e[1]=e[4],e[4]=t,t=e[2],e[2]=e[8],e[8]=t,t=e[6],e[6]=e[9],e[9]=t,t=e[3],e[3]=e[12],e[12]=t,t=e[7],e[7]=e[13],e[13]=t,t=e[11],e[11]=e[14],e[14]=t,this}setPosition(e,t,n){let i=this.elements;return e.isVector3?(i[12]=e.x,i[13]=e.y,i[14]=e.z):(i[12]=e,i[13]=t,i[14]=n),this}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8],u=e[9],d=e[10],f=e[11],g=e[12],v=e[13],m=e[14],p=e[15],M=t*a-n*o,T=t*l-i*o,x=t*c-r*o,y=n*l-i*a,S=n*c-r*a,C=i*c-r*l,_=h*v-u*g,I=h*m-d*g,b=h*p-f*g,N=u*m-d*v,w=u*p-f*v,H=d*p-f*m,F=M*H-T*w+x*N+y*b-S*I+C*_;if(F===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let L=1/F;return e[0]=(a*H-l*w+c*N)*L,e[1]=(i*w-n*H-r*N)*L,e[2]=(v*C-m*S+p*y)*L,e[3]=(d*S-u*C-f*y)*L,e[4]=(l*b-o*H-c*I)*L,e[5]=(t*H-i*b+r*I)*L,e[6]=(m*x-g*C-p*T)*L,e[7]=(h*C-d*x+f*T)*L,e[8]=(o*w-a*b+c*_)*L,e[9]=(n*b-t*w-r*_)*L,e[10]=(g*S-v*x+p*M)*L,e[11]=(u*x-h*S-f*M)*L,e[12]=(a*I-o*N-l*_)*L,e[13]=(t*N-n*I+i*_)*L,e[14]=(v*T-g*y-m*M)*L,e[15]=(h*y-u*T+d*M)*L,this}scale(e){let t=this.elements,n=e.x,i=e.y,r=e.z;return t[0]*=n,t[4]*=i,t[8]*=r,t[1]*=n,t[5]*=i,t[9]*=r,t[2]*=n,t[6]*=i,t[10]*=r,t[3]*=n,t[7]*=i,t[11]*=r,this}getMaxScaleOnAxis(){let e=this.elements,t=e[0]*e[0]+e[1]*e[1]+e[2]*e[2],n=e[4]*e[4]+e[5]*e[5]+e[6]*e[6],i=e[8]*e[8]+e[9]*e[9]+e[10]*e[10];return Math.sqrt(Math.max(t,n,i))}makeTranslation(e,t,n){return e.isVector3?this.set(1,0,0,e.x,0,1,0,e.y,0,0,1,e.z,0,0,0,1):this.set(1,0,0,e,0,1,0,t,0,0,1,n,0,0,0,1),this}makeRotationX(e){let t=Math.cos(e),n=Math.sin(e);return this.set(1,0,0,0,0,t,-n,0,0,n,t,0,0,0,0,1),this}makeRotationY(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,0,n,0,0,1,0,0,-n,0,t,0,0,0,0,1),this}makeRotationZ(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,0,n,t,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(e,t){let n=Math.cos(t),i=Math.sin(t),r=1-n,o=e.x,a=e.y,l=e.z,c=r*o,h=r*a;return this.set(c*o+n,c*a-i*l,c*l+i*a,0,c*a+i*l,h*a+n,h*l-i*o,0,c*l-i*a,h*l+i*o,r*l*l+n,0,0,0,0,1),this}makeScale(e,t,n){return this.set(e,0,0,0,0,t,0,0,0,0,n,0,0,0,0,1),this}makeShear(e,t,n,i,r,o){return this.set(1,n,r,0,e,1,o,0,t,i,1,0,0,0,0,1),this}compose(e,t,n){let i=this.elements,r=t._x,o=t._y,a=t._z,l=t._w,c=r+r,h=o+o,u=a+a,d=r*c,f=r*h,g=r*u,v=o*h,m=o*u,p=a*u,M=l*c,T=l*h,x=l*u,y=n.x,S=n.y,C=n.z;return i[0]=(1-(v+p))*y,i[1]=(f+x)*y,i[2]=(g-T)*y,i[3]=0,i[4]=(f-x)*S,i[5]=(1-(d+p))*S,i[6]=(m+M)*S,i[7]=0,i[8]=(g+T)*C,i[9]=(m-M)*C,i[10]=(1-(d+v))*C,i[11]=0,i[12]=e.x,i[13]=e.y,i[14]=e.z,i[15]=1,this}decompose(e,t,n){let i=this.elements;e.x=i[12],e.y=i[13],e.z=i[14];let r=this.determinantAffine();if(r===0)return n.set(1,1,1),t.identity(),this;let o=Hs.set(i[0],i[1],i[2]).length(),a=Hs.set(i[4],i[5],i[6]).length(),l=Hs.set(i[8],i[9],i[10]).length();r<0&&(o=-o),Gn.copy(this);let c=1/o,h=1/a,u=1/l;return Gn.elements[0]*=c,Gn.elements[1]*=c,Gn.elements[2]*=c,Gn.elements[4]*=h,Gn.elements[5]*=h,Gn.elements[6]*=h,Gn.elements[8]*=u,Gn.elements[9]*=u,Gn.elements[10]*=u,t.setFromRotationMatrix(Gn),n.x=o,n.y=a,n.z=l,this}makePerspective(e,t,n,i,r,o,a=Yn,l=!1){let c=this.elements,h=2*r/(t-e),u=2*r/(n-i),d=(t+e)/(t-e),f=(n+i)/(n-i),g,v;if(l)g=r/(o-r),v=o*r/(o-r);else if(a===Yn)g=-(o+r)/(o-r),v=-2*o*r/(o-r);else if(a===ir)g=-o/(o-r),v=-o*r/(o-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+a);return c[0]=h,c[4]=0,c[8]=d,c[12]=0,c[1]=0,c[5]=u,c[9]=f,c[13]=0,c[2]=0,c[6]=0,c[10]=g,c[14]=v,c[3]=0,c[7]=0,c[11]=-1,c[15]=0,this}makeOrthographic(e,t,n,i,r,o,a=Yn,l=!1){let c=this.elements,h=2/(t-e),u=2/(n-i),d=-(t+e)/(t-e),f=-(n+i)/(n-i),g,v;if(l)g=1/(o-r),v=o/(o-r);else if(a===Yn)g=-2/(o-r),v=-(o+r)/(o-r);else if(a===ir)g=-1/(o-r),v=-r/(o-r);else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+a);return c[0]=h,c[4]=0,c[8]=0,c[12]=d,c[1]=0,c[5]=u,c[9]=0,c[13]=f,c[2]=0,c[6]=0,c[10]=g,c[14]=v,c[3]=0,c[7]=0,c[11]=0,c[15]=1,this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<16;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<16;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e[t+9]=n[9],e[t+10]=n[10],e[t+11]=n[11],e[t+12]=n[12],e[t+13]=n[13],e[t+14]=n[14],e[t+15]=n[15],e}},Hs=new W,Gn=new it,mm=new W(0,0,0),gm=new W(1,1,1),Gi=new W,oa=new W,Tn=new W,qu=new it,Yu=new Ht,xn=class s{constructor(e=0,t=0,n=0,i=s.DEFAULT_ORDER){this.isEuler=!0,this._x=e,this._y=t,this._z=n,this._order=i}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get order(){return this._order}set order(e){this._order=e,this._onChangeCallback()}set(e,t,n,i=this._order){return this._x=e,this._y=t,this._z=n,this._order=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(e){return this._x=e._x,this._y=e._y,this._z=e._z,this._order=e._order,this._onChangeCallback(),this}setFromRotationMatrix(e,t=this._order,n=!0){let i=e.elements,r=i[0],o=i[4],a=i[8],l=i[1],c=i[5],h=i[9],u=i[2],d=i[6],f=i[10];switch(t){case"XYZ":this._y=Math.asin(ut(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(-h,f),this._z=Math.atan2(-o,r)):(this._x=Math.atan2(d,c),this._z=0);break;case"YXZ":this._x=Math.asin(-ut(h,-1,1)),Math.abs(h)<.9999999?(this._y=Math.atan2(a,f),this._z=Math.atan2(l,c)):(this._y=Math.atan2(-u,r),this._z=0);break;case"ZXY":this._x=Math.asin(ut(d,-1,1)),Math.abs(d)<.9999999?(this._y=Math.atan2(-u,f),this._z=Math.atan2(-o,c)):(this._y=0,this._z=Math.atan2(l,r));break;case"ZYX":this._y=Math.asin(-ut(u,-1,1)),Math.abs(u)<.9999999?(this._x=Math.atan2(d,f),this._z=Math.atan2(l,r)):(this._x=0,this._z=Math.atan2(-o,c));break;case"YZX":this._z=Math.asin(ut(l,-1,1)),Math.abs(l)<.9999999?(this._x=Math.atan2(-h,c),this._y=Math.atan2(-u,r)):(this._x=0,this._y=Math.atan2(a,f));break;case"XZY":this._z=Math.asin(-ut(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(d,c),this._y=Math.atan2(a,r)):(this._x=Math.atan2(-h,f),this._y=0);break;default:Ze("Euler: .setFromRotationMatrix() encountered an unknown order: "+t)}return this._order=t,n===!0&&this._onChangeCallback(),this}setFromQuaternion(e,t,n){return qu.makeRotationFromQuaternion(e),this.setFromRotationMatrix(qu,t,n)}setFromVector3(e,t=this._order){return this.set(e.x,e.y,e.z,t)}reorder(e){return Yu.setFromEuler(this),this.setFromQuaternion(Yu,e)}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._order===this._order}fromArray(e){return this._x=e[0],this._y=e[1],this._z=e[2],e[3]!==void 0&&(this._order=e[3]),this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._order,e}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};xn.DEFAULT_ORDER="XYZ";var ar=class{constructor(){this.mask=1}set(e){this.mask=(1<<e|0)>>>0}enable(e){this.mask|=1<<e|0}enableAll(){this.mask=-1}toggle(e){this.mask^=1<<e|0}disable(e){this.mask&=~(1<<e|0)}disableAll(){this.mask=0}test(e){return(this.mask&e.mask)!==0}isEnabled(e){return(this.mask&(1<<e|0))!==0}},xm=0,Zu=new W,Vs=new Ht,Mi=new it,aa=new W,Vr=new W,_m=new W,vm=new Ht,Ku=new W(1,0,0),ju=new W(0,1,0),Ju=new W(0,0,1),$u={type:"added"},ym={type:"removed"},Gs={type:"childadded",child:null},Dc={type:"childremoved",child:null},Tt=class s extends On{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:xm++}),this.uuid=Zn(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=s.DEFAULT_UP.clone();let e=new W,t=new xn,n=new Ht,i=new W(1,1,1);function r(){n.setFromEuler(t,!1)}function o(){t.setFromQuaternion(n,void 0,!1)}t._onChange(r),n._onChange(o),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:e},rotation:{configurable:!0,enumerable:!0,value:t},quaternion:{configurable:!0,enumerable:!0,value:n},scale:{configurable:!0,enumerable:!0,value:i},modelViewMatrix:{value:new it},normalMatrix:{value:new rt}}),this.matrix=new it,this.matrixWorld=new it,this.matrixAutoUpdate=s.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=s.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new ar,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.customDepthMaterial=void 0,this.customDistanceMaterial=void 0,this.static=!1,this.userData={},this.pivot=null}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(e){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(e),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(e){return this.quaternion.premultiply(e),this}setRotationFromAxisAngle(e,t){this.quaternion.setFromAxisAngle(e,t)}setRotationFromEuler(e){this.quaternion.setFromEuler(e,!0)}setRotationFromMatrix(e){this.quaternion.setFromRotationMatrix(e)}setRotationFromQuaternion(e){this.quaternion.copy(e)}rotateOnAxis(e,t){return Vs.setFromAxisAngle(e,t),this.quaternion.multiply(Vs),this}rotateOnWorldAxis(e,t){return Vs.setFromAxisAngle(e,t),this.quaternion.premultiply(Vs),this}rotateX(e){return this.rotateOnAxis(Ku,e)}rotateY(e){return this.rotateOnAxis(ju,e)}rotateZ(e){return this.rotateOnAxis(Ju,e)}translateOnAxis(e,t){return Zu.copy(e).applyQuaternion(this.quaternion),this.position.add(Zu.multiplyScalar(t)),this}translateX(e){return this.translateOnAxis(Ku,e)}translateY(e){return this.translateOnAxis(ju,e)}translateZ(e){return this.translateOnAxis(Ju,e)}localToWorld(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(this.matrixWorld)}worldToLocal(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(Mi.copy(this.matrixWorld).invert())}lookAt(e,t,n){e.isVector3?aa.copy(e):aa.set(e,t,n);let i=this.parent;this.updateWorldMatrix(!0,!1),Vr.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?Mi.lookAt(Vr,aa,this.up):Mi.lookAt(aa,Vr,this.up),this.quaternion.setFromRotationMatrix(Mi),i&&(Mi.extractRotation(i.matrixWorld),Vs.setFromRotationMatrix(Mi),this.quaternion.premultiply(Vs.invert()))}add(e){if(arguments.length>1){for(let t=0;t<arguments.length;t++)this.add(arguments[t]);return this}return e===this?(et("Object3D.add: object can't be added as a child of itself.",e),this):(e&&e.isObject3D?(e.removeFromParent(),e.parent=this,this.children.push(e),e.dispatchEvent($u),Gs.child=e,this.dispatchEvent(Gs),Gs.child=null):et("Object3D.add: object not an instance of THREE.Object3D.",e),this)}remove(e){if(arguments.length>1){for(let n=0;n<arguments.length;n++)this.remove(arguments[n]);return this}let t=this.children.indexOf(e);return t!==-1&&(e.parent=null,this.children.splice(t,1),e.dispatchEvent(ym),Dc.child=e,this.dispatchEvent(Dc),Dc.child=null),this}removeFromParent(){let e=this.parent;return e!==null&&e.remove(this),this}clear(){return this.remove(...this.children)}attach(e){return this.updateWorldMatrix(!0,!1),Mi.copy(this.matrixWorld).invert(),e.parent!==null&&(e.parent.updateWorldMatrix(!0,!1),Mi.multiply(e.parent.matrixWorld)),e.applyMatrix4(Mi),e.removeFromParent(),e.parent=this,this.children.push(e),e.updateWorldMatrix(!1,!0),e.dispatchEvent($u),Gs.child=e,this.dispatchEvent(Gs),Gs.child=null,this}getObjectById(e){return this.getObjectByProperty("id",e)}getObjectByName(e){return this.getObjectByProperty("name",e)}getObjectByProperty(e,t){if(this[e]===t)return this;for(let n=0,i=this.children.length;n<i;n++){let o=this.children[n].getObjectByProperty(e,t);if(o!==void 0)return o}}getObjectsByProperty(e,t,n=[]){this[e]===t&&n.push(this);let i=this.children;for(let r=0,o=i.length;r<o;r++)i[r].getObjectsByProperty(e,t,n);return n}getWorldPosition(e){return this.updateWorldMatrix(!0,!1),e.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Vr,e,_m),e}getWorldScale(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Vr,vm,e),e}getWorldDirection(e){this.updateWorldMatrix(!0,!1);let t=this.matrixWorld.elements;return e.set(t[8],t[9],t[10]).normalize()}raycast(){}intersectsFrustum(){}traverse(e){e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverse(e)}traverseVisible(e){if(this.visible===!1)return;e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverseVisible(e)}traverseAncestors(e){let t=this.parent;t!==null&&(e(t),t.traverseAncestors(e))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale);let e=this.pivot;if(e!==null){let t=e.x,n=e.y,i=e.z,r=this.matrix.elements;r[12]+=t-r[0]*t-r[4]*n-r[8]*i,r[13]+=n-r[1]*t-r[5]*n-r[9]*i,r[14]+=i-r[2]*t-r[6]*n-r[10]*i}this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(e){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||e)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,e=!0);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].updateMatrixWorld(e)}updateWorldMatrix(e,t,n=!1){let i=this.parent;if(e===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||n)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,n=!0),t===!0){let r=this.children;for(let o=0,a=r.length;o<a;o++)r[o].updateWorldMatrix(!1,!0,n)}}toJSON(e){let t=e===void 0||typeof e=="string",n={};t&&(e={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},n.metadata={version:4.7,type:"Object",generator:"Object3D.toJSON"});let i={};i.uuid=this.uuid,i.type=this.type,i.name=this.name,i.castShadow=this.castShadow,i.receiveShadow=this.receiveShadow,i.visible=this.visible,i.frustumCulled=this.frustumCulled,i.renderOrder=this.renderOrder,i.static=this.static,i.matrixAutoUpdate=this.matrixAutoUpdate,Object.keys(this.userData).length>0&&(i.userData=this.userData),i.layers=this.layers.mask,i.matrix=this.matrix.toArray(),i.up=this.up.toArray(),this.pivot!==null&&(i.pivot=this.pivot.toArray()),this.morphTargetDictionary!==void 0&&(i.morphTargetDictionary=Object.assign({},this.morphTargetDictionary)),this.morphTargetInfluences!==void 0&&(i.morphTargetInfluences=this.morphTargetInfluences.slice()),this.isInstancedMesh&&(i.type="InstancedMesh",i.count=this.count,i.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(i.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(i.type="BatchedMesh",i.perObjectFrustumCulled=this.perObjectFrustumCulled,i.sortObjects=this.sortObjects,i.drawRanges=this._drawRanges,i.reservedRanges=this._reservedRanges,i.geometryInfo=this._geometryInfo.map(a=>({...a,boundingBox:a.boundingBox?a.boundingBox.toJSON():void 0,boundingSphere:a.boundingSphere?a.boundingSphere.toJSON():void 0})),i.instanceInfo=this._instanceInfo.map(a=>({...a})),i.availableInstanceIds=this._availableInstanceIds.slice(),i.availableGeometryIds=this._availableGeometryIds.slice(),i.nextIndexStart=this._nextIndexStart,i.nextVertexStart=this._nextVertexStart,i.geometryCount=this._geometryCount,i.maxInstanceCount=this._maxInstanceCount,i.maxVertexCount=this._maxVertexCount,i.maxIndexCount=this._maxIndexCount,i.geometryInitialized=this._geometryInitialized,i.matricesTexture=this._matricesTexture.toJSON(e),i.indirectTexture=this._indirectTexture.toJSON(e),this._colorsTexture!==null&&(i.colorsTexture=this._colorsTexture.toJSON(e)),this.boundingSphere!==null&&(i.boundingSphere=this.boundingSphere.toJSON()),this.boundingBox!==null&&(i.boundingBox=this.boundingBox.toJSON()));function r(a,l){return a[l.uuid]===void 0&&(a[l.uuid]=l.toJSON(e)),l.uuid}if(this.isScene)this.background&&(this.background.isColor?i.background=this.background.toJSON():this.background.isTexture&&(i.background=this.background.toJSON(e).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(i.environment=this.environment.toJSON(e).uuid);else if(this.isMesh||this.isLine||this.isPoints){i.geometry=r(e.geometries,this.geometry);let a=this.geometry.parameters;if(a!==void 0&&a.shapes!==void 0){let l=a.shapes;if(Array.isArray(l))for(let c=0,h=l.length;c<h;c++){let u=l[c];r(e.shapes,u)}else r(e.shapes,l)}}if(this.isSkinnedMesh&&(i.bindMode=this.bindMode,i.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(e.skeletons,this.skeleton),i.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let a=[];for(let l=0,c=this.material.length;l<c;l++)a.push(r(e.materials,this.material[l]));i.material=a}else i.material=r(e.materials,this.material);if(this.children.length>0){i.children=[];for(let a=0;a<this.children.length;a++)i.children.push(this.children[a].toJSON(e).object)}if(this.animations.length>0){i.animations=[];for(let a=0;a<this.animations.length;a++){let l=this.animations[a];i.animations.push(r(e.animations,l))}}if(t){let a=o(e.geometries),l=o(e.materials),c=o(e.textures),h=o(e.images),u=o(e.shapes),d=o(e.skeletons),f=o(e.animations),g=o(e.nodes);a.length>0&&(n.geometries=a),l.length>0&&(n.materials=l),c.length>0&&(n.textures=c),h.length>0&&(n.images=h),u.length>0&&(n.shapes=u),d.length>0&&(n.skeletons=d),f.length>0&&(n.animations=f),g.length>0&&(n.nodes=g)}return n.object=i,n;function o(a){let l=[];for(let c in a){let h=a[c];delete h.metadata,l.push(h)}return l}}clone(e){return new this.constructor().copy(this,e)}copy(e,t=!0){if(this.name=e.name,this.up.copy(e.up),this.position.copy(e.position),this.rotation.order=e.rotation.order,this.quaternion.copy(e.quaternion),this.scale.copy(e.scale),this.pivot=e.pivot!==null?e.pivot.clone():null,this.matrix.copy(e.matrix),this.matrixWorld.copy(e.matrixWorld),this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrixWorldAutoUpdate=e.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=e.matrixWorldNeedsUpdate,this.layers.mask=e.layers.mask,this.visible=e.visible,this.castShadow=e.castShadow,this.receiveShadow=e.receiveShadow,this.frustumCulled=e.frustumCulled,this.renderOrder=e.renderOrder,this.static=e.static,this.animations=e.animations.slice(),this.userData=JSON.parse(JSON.stringify(e.userData)),t===!0)for(let n=0;n<e.children.length;n++){let i=e.children[n];this.add(i.clone())}return this}dispose(){this.dispatchEvent({type:"dispose"})}};Tt.DEFAULT_UP=new W(0,1,0);Tt.DEFAULT_MATRIX_AUTO_UPDATE=!0;Tt.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var lt=class extends Tt{constructor(){super(),this.isGroup=!0,this.type="Group"}},Mm={type:"move"},lr=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new lt,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new lt,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new W,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new W),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new lt,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new W,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new W,this._grip.eventsEnabled=!1),this._grip}dispatchEvent(e){return this._targetRay!==null&&this._targetRay.dispatchEvent(e),this._grip!==null&&this._grip.dispatchEvent(e),this._hand!==null&&this._hand.dispatchEvent(e),this}connect(e){if(e&&e.hand){let t=this._hand;if(t)for(let n of e.hand.values())this._getHandJoint(t,n)}return this.dispatchEvent({type:"connected",data:e}),this}disconnect(e){return this.dispatchEvent({type:"disconnected",data:e}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(e,t,n){let i=null,r=null,o=null,a=this._targetRay,l=this._grip,c=this._hand;if(e&&t.session.visibilityState!=="visible-blurred"){if(c&&e.hand){o=!0;for(let v of e.hand.values()){let m=t.getJointPose(v,n),p=this._getHandJoint(c,v);m!==null&&(p.matrix.fromArray(m.transform.matrix),p.matrix.decompose(p.position,p.rotation,p.scale),p.matrixWorldNeedsUpdate=!0,p.jointRadius=m.radius),p.visible=m!==null}let h=c.joints["index-finger-tip"],u=c.joints["thumb-tip"],d=h.position.distanceTo(u.position),f=.02,g=.005;c.inputState.pinching&&d>f+g?(c.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:e.handedness,target:this})):!c.inputState.pinching&&d<=f-g&&(c.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:e.handedness,target:this}))}else l!==null&&e.gripSpace&&(r=t.getPose(e.gripSpace,n),r!==null&&(l.matrix.fromArray(r.transform.matrix),l.matrix.decompose(l.position,l.rotation,l.scale),l.matrixWorldNeedsUpdate=!0,r.linearVelocity?(l.hasLinearVelocity=!0,l.linearVelocity.copy(r.linearVelocity)):l.hasLinearVelocity=!1,r.angularVelocity?(l.hasAngularVelocity=!0,l.angularVelocity.copy(r.angularVelocity)):l.hasAngularVelocity=!1,l.eventsEnabled&&l.dispatchEvent({type:"gripUpdated",data:e,target:this})));a!==null&&(i=t.getPose(e.targetRaySpace,n),i===null&&r!==null&&(i=r),i!==null&&(a.matrix.fromArray(i.transform.matrix),a.matrix.decompose(a.position,a.rotation,a.scale),a.matrixWorldNeedsUpdate=!0,i.linearVelocity?(a.hasLinearVelocity=!0,a.linearVelocity.copy(i.linearVelocity)):a.hasLinearVelocity=!1,i.angularVelocity?(a.hasAngularVelocity=!0,a.angularVelocity.copy(i.angularVelocity)):a.hasAngularVelocity=!1,this.dispatchEvent(Mm)))}return a!==null&&(a.visible=i!==null),l!==null&&(l.visible=r!==null),c!==null&&(c.visible=o!==null),this}_getHandJoint(e,t){if(e.joints[t.jointName]===void 0){let n=new lt;n.matrixAutoUpdate=!1,n.visible=!1,e.joints[t.jointName]=n,e.add(n)}return e.joints[t.jointName]}},gf={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},Wi={h:0,s:0,l:0},la={h:0,s:0,l:0};function Nc(s,e,t){return t<0&&(t+=1),t>1&&(t-=1),t<1/6?s+(e-s)*6*t:t<1/2?e:t<2/3?s+(e-s)*6*(2/3-t):s}var Ce=class{constructor(e,t,n){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(e,t,n)}set(e,t,n){if(t===void 0&&n===void 0){let i=e;i&&i.isColor?this.copy(i):typeof i=="number"?this.setHex(i):typeof i=="string"&&this.setStyle(i)}else this.setRGB(e,t,n);return this}setScalar(e){return this.r=e,this.g=e,this.b=e,this}setHex(e,t=Ft){return e=Math.floor(e),this.r=(e>>16&255)/255,this.g=(e>>8&255)/255,this.b=(e&255)/255,ht.colorSpaceToWorking(this,t),this}setRGB(e,t,n,i=ht.workingColorSpace){return this.r=e,this.g=t,this.b=n,ht.colorSpaceToWorking(this,i),this}setHSL(e,t,n,i=ht.workingColorSpace){if(e=Rh(e,1),t=ut(t,0,1),n=ut(n,0,1),t===0)this.r=this.g=this.b=n;else{let r=n<=.5?n*(1+t):n+t-n*t,o=2*n-r;this.r=Nc(o,r,e+1/3),this.g=Nc(o,r,e),this.b=Nc(o,r,e-1/3)}return ht.colorSpaceToWorking(this,i),this}setStyle(e,t=Ft){function n(r){r!==void 0&&parseFloat(r)<1&&Ze("Color: Alpha component of "+e+" will be ignored.")}let i;if(i=/^(\w+)\(([^\)]*)\)/.exec(e)){let r,o=i[1],a=i[2];switch(o){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,t);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,t);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,t);break;default:Ze("Color: Unknown color model "+e)}}else if(i=/^\#([A-Fa-f\d]+)$/.exec(e)){let r=i[1],o=r.length;if(o===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,t);if(o===6)return this.setHex(parseInt(r,16),t);Ze("Color: Invalid hex color "+e)}else if(e&&e.length>0)return this.setColorName(e,t);return this}setColorName(e,t=Ft){let n=gf[e.toLowerCase()];return n!==void 0?this.setHex(n,t):Ze("Color: Unknown color "+e),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(e){return this.r=e.r,this.g=e.g,this.b=e.b,this}copySRGBToLinear(e){return this.r=Ai(e.r),this.g=Ai(e.g),this.b=Ai(e.b),this}copyLinearToSRGB(e){return this.r=er(e.r),this.g=er(e.g),this.b=er(e.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(e=Ft){return ht.workingToColorSpace(ln.copy(this),e),Math.round(ut(ln.r*255,0,255))*65536+Math.round(ut(ln.g*255,0,255))*256+Math.round(ut(ln.b*255,0,255))}getHexString(e=Ft){return("000000"+this.getHex(e).toString(16)).slice(-6)}getHSL(e,t=ht.workingColorSpace){ht.workingToColorSpace(ln.copy(this),t);let n=ln.r,i=ln.g,r=ln.b,o=Math.max(n,i,r),a=Math.min(n,i,r),l,c,h=(a+o)/2;if(a===o)l=0,c=0;else{let u=o-a;switch(c=h<=.5?u/(o+a):u/(2-o-a),o){case n:l=(i-r)/u+(i<r?6:0);break;case i:l=(r-n)/u+2;break;case r:l=(n-i)/u+4;break}l/=6}return e.h=l,e.s=c,e.l=h,e}getRGB(e,t=ht.workingColorSpace){return ht.workingToColorSpace(ln.copy(this),t),e.r=ln.r,e.g=ln.g,e.b=ln.b,e}getStyle(e=Ft){ht.workingToColorSpace(ln.copy(this),e);let t=ln.r,n=ln.g,i=ln.b;return e!==Ft?`color(${e} ${t.toFixed(3)} ${n.toFixed(3)} ${i.toFixed(3)})`:`rgb(${Math.round(t*255)},${Math.round(n*255)},${Math.round(i*255)})`}offsetHSL(e,t,n){return this.getHSL(Wi),this.setHSL(Wi.h+e,Wi.s+t,Wi.l+n)}add(e){return this.r+=e.r,this.g+=e.g,this.b+=e.b,this}addColors(e,t){return this.r=e.r+t.r,this.g=e.g+t.g,this.b=e.b+t.b,this}addScalar(e){return this.r+=e,this.g+=e,this.b+=e,this}sub(e){return this.r=Math.max(0,this.r-e.r),this.g=Math.max(0,this.g-e.g),this.b=Math.max(0,this.b-e.b),this}multiply(e){return this.r*=e.r,this.g*=e.g,this.b*=e.b,this}multiplyScalar(e){return this.r*=e,this.g*=e,this.b*=e,this}lerp(e,t){return this.r+=(e.r-this.r)*t,this.g+=(e.g-this.g)*t,this.b+=(e.b-this.b)*t,this}lerpColors(e,t,n){return this.r=e.r+(t.r-e.r)*n,this.g=e.g+(t.g-e.g)*n,this.b=e.b+(t.b-e.b)*n,this}lerpHSL(e,t){this.getHSL(Wi),e.getHSL(la);let n=Jr(Wi.h,la.h,t),i=Jr(Wi.s,la.s,t),r=Jr(Wi.l,la.l,t);return this.setHSL(n,i,r),this}setFromVector3(e){return this.r=e.x,this.g=e.y,this.b=e.z,this}applyMatrix3(e){let t=this.r,n=this.g,i=this.b,r=e.elements;return this.r=r[0]*t+r[3]*n+r[6]*i,this.g=r[1]*t+r[4]*n+r[7]*i,this.b=r[2]*t+r[5]*n+r[8]*i,this}equals(e){return e.r===this.r&&e.g===this.g&&e.b===this.b}fromArray(e,t=0){return this.r=e[t],this.g=e[t+1],this.b=e[t+2],this}toArray(e=[],t=0){return e[t]=this.r,e[t+1]=this.g,e[t+2]=this.b,e}fromBufferAttribute(e,t){return this.r=e.getX(t),this.g=e.getY(t),this.b=e.getZ(t),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},ln=new Ce;Ce.NAMES=gf;var so=class s{constructor(e,t=25e-5){this.isFogExp2=!0,this.name="",this.color=new Ce(e),this.density=t}clone(){return new s(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var bs=class extends Tt{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new xn,this.environmentIntensity=1,this.environmentRotation=new xn,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(e,t){return super.copy(e,t),e.background!==null&&(this.background=e.background.clone()),e.environment!==null&&(this.environment=e.environment.clone()),e.fog!==null&&(this.fog=e.fog.clone()),this.backgroundBlurriness=e.backgroundBlurriness,this.backgroundIntensity=e.backgroundIntensity,this.backgroundRotation.copy(e.backgroundRotation),this.environmentIntensity=e.environmentIntensity,this.environmentRotation.copy(e.environmentRotation),e.overrideMaterial!==null&&(this.overrideMaterial=e.overrideMaterial.clone()),this.matrixAutoUpdate=e.matrixAutoUpdate,this}toJSON(e){let t=super.toJSON(e);return this.fog!==null&&(t.object.fog=this.fog.toJSON()),t.object.backgroundBlurriness=this.backgroundBlurriness,t.object.backgroundIntensity=this.backgroundIntensity,t.object.backgroundRotation=this.backgroundRotation.toArray(),t.object.environmentIntensity=this.environmentIntensity,t.object.environmentRotation=this.environmentRotation.toArray(),t}},Wn=new W,bi=new W,Uc=new W,Si=new W,Ws=new W,Xs=new W,Qu=new W,Fc=new W,Oc=new W,Bc=new W,zc=new gt,kc=new gt,Hc=new gt,Ki=class s{constructor(e=new W,t=new W,n=new W){this.a=e,this.b=t,this.c=n}static getNormal(e,t,n,i){i.subVectors(n,t),Wn.subVectors(e,t),i.cross(Wn);let r=i.lengthSq();return r>0?i.multiplyScalar(1/Math.sqrt(r)):i.set(0,0,0)}static getBarycoord(e,t,n,i,r){Wn.subVectors(i,t),bi.subVectors(n,t),Uc.subVectors(e,t);let o=Wn.dot(Wn),a=Wn.dot(bi),l=Wn.dot(Uc),c=bi.dot(bi),h=bi.dot(Uc),u=o*c-a*a;if(u===0)return r.set(0,0,0),null;let d=1/u,f=(c*l-a*h)*d,g=(o*h-a*l)*d;return r.set(1-f-g,g,f)}static containsPoint(e,t,n,i){return this.getBarycoord(e,t,n,i,Si)===null?!1:Si.x>=0&&Si.y>=0&&Si.x+Si.y<=1}static getInterpolation(e,t,n,i,r,o,a,l){return this.getBarycoord(e,t,n,i,Si)===null?(l.x=0,l.y=0,"z"in l&&(l.z=0),"w"in l&&(l.w=0),null):(l.setScalar(0),l.addScaledVector(r,Si.x),l.addScaledVector(o,Si.y),l.addScaledVector(a,Si.z),l)}static getInterpolatedAttribute(e,t,n,i,r,o){return zc.setScalar(0),kc.setScalar(0),Hc.setScalar(0),zc.fromBufferAttribute(e,t),kc.fromBufferAttribute(e,n),Hc.fromBufferAttribute(e,i),o.setScalar(0),o.addScaledVector(zc,r.x),o.addScaledVector(kc,r.y),o.addScaledVector(Hc,r.z),o}static isFrontFacing(e,t,n,i){return Wn.subVectors(n,t),bi.subVectors(e,t),Wn.cross(bi).dot(i)<0}set(e,t,n){return this.a.copy(e),this.b.copy(t),this.c.copy(n),this}setFromPointsAndIndices(e,t,n,i){return this.a.copy(e[t]),this.b.copy(e[n]),this.c.copy(e[i]),this}setFromAttributeAndIndices(e,t,n,i){return this.a.fromBufferAttribute(e,t),this.b.fromBufferAttribute(e,n),this.c.fromBufferAttribute(e,i),this}clone(){return new this.constructor().copy(this)}copy(e){return this.a.copy(e.a),this.b.copy(e.b),this.c.copy(e.c),this}getArea(){return Wn.subVectors(this.c,this.b),bi.subVectors(this.a,this.b),Wn.cross(bi).length()*.5}getMidpoint(e){return e.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(e){return s.getNormal(this.a,this.b,this.c,e)}getPlane(e){return e.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(e,t){return s.getBarycoord(e,this.a,this.b,this.c,t)}getInterpolation(e,t,n,i,r){return s.getInterpolation(e,this.a,this.b,this.c,t,n,i,r)}containsPoint(e){return s.containsPoint(e,this.a,this.b,this.c)}isFrontFacing(e){return s.isFrontFacing(this.a,this.b,this.c,e)}intersectsBox(e){return e.intersectsTriangle(this)}closestPointToPoint(e,t){let n=this.a,i=this.b,r=this.c,o,a;Ws.subVectors(i,n),Xs.subVectors(r,n),Fc.subVectors(e,n);let l=Ws.dot(Fc),c=Xs.dot(Fc);if(l<=0&&c<=0)return t.copy(n);Oc.subVectors(e,i);let h=Ws.dot(Oc),u=Xs.dot(Oc);if(h>=0&&u<=h)return t.copy(i);let d=l*u-h*c;if(d<=0&&l>=0&&h<=0)return o=l/(l-h),t.copy(n).addScaledVector(Ws,o);Bc.subVectors(e,r);let f=Ws.dot(Bc),g=Xs.dot(Bc);if(g>=0&&f<=g)return t.copy(r);let v=f*c-l*g;if(v<=0&&c>=0&&g<=0)return a=c/(c-g),t.copy(n).addScaledVector(Xs,a);let m=h*g-f*u;if(m<=0&&u-h>=0&&f-g>=0)return Qu.subVectors(r,i),a=(u-h)/(u-h+(f-g)),t.copy(i).addScaledVector(Qu,a);let p=1/(m+v+d);return o=v*p,a=d*p,t.copy(n).addScaledVector(Ws,o).addScaledVector(Xs,a)}equals(e){return e.a.equals(this.a)&&e.b.equals(this.b)&&e.c.equals(this.c)}},Zt=class{constructor(e=new W(1/0,1/0,1/0),t=new W(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=e,this.max=t}set(e,t){return this.min.copy(e),this.max.copy(t),this}setFromArray(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t+=3)this.expandByPoint(Xn.fromArray(e,t));return this}setFromBufferAttribute(e){this.makeEmpty();for(let t=0,n=e.count;t<n;t++)this.expandByPoint(Xn.fromBufferAttribute(e,t));return this}setFromPoints(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t++)this.expandByPoint(e[t]);return this}setFromCenterAndSize(e,t){let n=Xn.copy(t).multiplyScalar(.5);return this.min.copy(e).sub(n),this.max.copy(e).add(n),this}setFromObject(e,t=!1){return this.makeEmpty(),this.expandByObject(e,t)}clone(){return new this.constructor().copy(this)}copy(e){return this.min.copy(e.min),this.max.copy(e.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(e){return this.isEmpty()?e.set(0,0,0):e.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(e){return this.isEmpty()?e.set(0,0,0):e.subVectors(this.max,this.min)}expandByPoint(e){return this.min.min(e),this.max.max(e),this}expandByVector(e){return this.min.sub(e),this.max.add(e),this}expandByScalar(e){return this.min.addScalar(-e),this.max.addScalar(e),this}expandByObject(e,t=!1){e.updateWorldMatrix(!1,!1);let n=e.geometry;if(n!==void 0){let r=n.getAttribute("position");if(t===!0&&r!==void 0&&e.isInstancedMesh!==!0)for(let o=0,a=r.count;o<a;o++)e.isMesh===!0?e.getVertexPosition(o,Xn):Xn.fromBufferAttribute(r,o),Xn.applyMatrix4(e.matrixWorld),this.expandByPoint(Xn);else e.boundingBox!==void 0?(e.boundingBox===null&&e.computeBoundingBox(),ca.copy(e.boundingBox)):(n.boundingBox===null&&n.computeBoundingBox(),ca.copy(n.boundingBox)),ca.applyMatrix4(e.matrixWorld),this.union(ca)}let i=e.children;for(let r=0,o=i.length;r<o;r++)this.expandByObject(i[r],t);return this}containsPoint(e){return e.x>=this.min.x&&e.x<=this.max.x&&e.y>=this.min.y&&e.y<=this.max.y&&e.z>=this.min.z&&e.z<=this.max.z}containsBox(e){return this.min.x<=e.min.x&&e.max.x<=this.max.x&&this.min.y<=e.min.y&&e.max.y<=this.max.y&&this.min.z<=e.min.z&&e.max.z<=this.max.z}getParameter(e,t){return t.set((e.x-this.min.x)/(this.max.x-this.min.x),(e.y-this.min.y)/(this.max.y-this.min.y),(e.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(e){return e.max.x>=this.min.x&&e.min.x<=this.max.x&&e.max.y>=this.min.y&&e.min.y<=this.max.y&&e.max.z>=this.min.z&&e.min.z<=this.max.z}intersectsSphere(e){return this.clampPoint(e.center,Xn),Xn.distanceToSquared(e.center)<=e.radius*e.radius}intersectsPlane(e){let t,n;return e.normal.x>0?(t=e.normal.x*this.min.x,n=e.normal.x*this.max.x):(t=e.normal.x*this.max.x,n=e.normal.x*this.min.x),e.normal.y>0?(t+=e.normal.y*this.min.y,n+=e.normal.y*this.max.y):(t+=e.normal.y*this.max.y,n+=e.normal.y*this.min.y),e.normal.z>0?(t+=e.normal.z*this.min.z,n+=e.normal.z*this.max.z):(t+=e.normal.z*this.max.z,n+=e.normal.z*this.min.z),t<=-e.constant&&n>=-e.constant}intersectsTriangle(e){if(this.isEmpty())return!1;this.getCenter(Gr),ha.subVectors(this.max,Gr),qs.subVectors(e.a,Gr),Ys.subVectors(e.b,Gr),Zs.subVectors(e.c,Gr),Xi.subVectors(Ys,qs),qi.subVectors(Zs,Ys),ds.subVectors(qs,Zs);let t=[0,-Xi.z,Xi.y,0,-qi.z,qi.y,0,-ds.z,ds.y,Xi.z,0,-Xi.x,qi.z,0,-qi.x,ds.z,0,-ds.x,-Xi.y,Xi.x,0,-qi.y,qi.x,0,-ds.y,ds.x,0];return!Vc(t,qs,Ys,Zs,ha)||(t=[1,0,0,0,1,0,0,0,1],!Vc(t,qs,Ys,Zs,ha))?!1:(ua.crossVectors(Xi,qi),t=[ua.x,ua.y,ua.z],Vc(t,qs,Ys,Zs,ha))}clampPoint(e,t){return t.copy(e).clamp(this.min,this.max)}distanceToPoint(e){return this.clampPoint(e,Xn).distanceTo(e)}getBoundingSphere(e){return this.isEmpty()?e.makeEmpty():(this.getCenter(e.center),e.radius=this.getSize(Xn).length()*.5),e}intersect(e){return this.min.max(e.min),this.max.min(e.max),this.isEmpty()&&this.makeEmpty(),this}union(e){return this.min.min(e.min),this.max.max(e.max),this}applyMatrix4(e){return this.isEmpty()?this:(wi[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(e),wi[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(e),wi[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(e),wi[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(e),wi[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(e),wi[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(e),wi[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(e),wi[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(e),this.setFromPoints(wi),this)}translate(e){return this.min.add(e),this.max.add(e),this}equals(e){return e.min.equals(this.min)&&e.max.equals(this.max)}toJSON(){return{min:this.min.toArray(),max:this.max.toArray()}}fromJSON(e){return this.min.fromArray(e.min),this.max.fromArray(e.max),this}},wi=[new W,new W,new W,new W,new W,new W,new W,new W],Xn=new W,ca=new Zt,qs=new W,Ys=new W,Zs=new W,Xi=new W,qi=new W,ds=new W,Gr=new W,ha=new W,ua=new W,fs=new W;function Vc(s,e,t,n,i){for(let r=0,o=s.length-3;r<=o;r+=3){fs.fromArray(s,r);let a=i.x*Math.abs(fs.x)+i.y*Math.abs(fs.y)+i.z*Math.abs(fs.z),l=e.dot(fs),c=t.dot(fs),h=n.dot(fs);if(Math.max(-Math.max(l,c,h),Math.min(l,c,h))>a)return!1}return!0}var Xt=new W,da=new Ue,bm=0,dt=class extends On{constructor(e,t,n=!1){if(super(),Array.isArray(e))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,Object.defineProperty(this,"id",{value:bm++}),this.name="",this.array=e,this.itemSize=t,this.count=e!==void 0?e.length/t:0,this.normalized=n,this.usage=Eh,this.updateRanges=[],this.gpuType=Pn,this.version=0}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.name=e.name,this.array=new e.array.constructor(e.array),this.itemSize=e.itemSize,this.count=e.count,this.normalized=e.normalized,this.usage=e.usage,this.gpuType=e.gpuType,this}copyAt(e,t,n){e*=this.itemSize,n*=t.itemSize;for(let i=0,r=this.itemSize;i<r;i++)this.array[e+i]=t.array[n+i];return this}copyArray(e){return this.array.set(e),this}applyMatrix3(e){if(this.itemSize===2)for(let t=0,n=this.count;t<n;t++)da.fromBufferAttribute(this,t),da.applyMatrix3(e),this.setXY(t,da.x,da.y);else if(this.itemSize===3)for(let t=0,n=this.count;t<n;t++)Xt.fromBufferAttribute(this,t),Xt.applyMatrix3(e),this.setXYZ(t,Xt.x,Xt.y,Xt.z);return this}applyMatrix4(e){for(let t=0,n=this.count;t<n;t++)Xt.fromBufferAttribute(this,t),Xt.applyMatrix4(e),this.setXYZ(t,Xt.x,Xt.y,Xt.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)Xt.fromBufferAttribute(this,t),Xt.applyNormalMatrix(e),this.setXYZ(t,Xt.x,Xt.y,Xt.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)Xt.fromBufferAttribute(this,t),Xt.transformDirection(e),this.setXYZ(t,Xt.x,Xt.y,Xt.z);return this}set(e,t=0){return this.array.set(e,t),this}getComponent(e,t){let n=this.array[e*this.itemSize+t];return this.normalized&&(n=qn(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=At(n,this.array)),this.array[e*this.itemSize+t]=n,this}getX(e){let t=this.array[e*this.itemSize];return this.normalized&&(t=qn(t,this.array)),t}setX(e,t){return this.normalized&&(t=At(t,this.array)),this.array[e*this.itemSize]=t,this}getY(e){let t=this.array[e*this.itemSize+1];return this.normalized&&(t=qn(t,this.array)),t}setY(e,t){return this.normalized&&(t=At(t,this.array)),this.array[e*this.itemSize+1]=t,this}getZ(e){let t=this.array[e*this.itemSize+2];return this.normalized&&(t=qn(t,this.array)),t}setZ(e,t){return this.normalized&&(t=At(t,this.array)),this.array[e*this.itemSize+2]=t,this}getW(e){let t=this.array[e*this.itemSize+3];return this.normalized&&(t=qn(t,this.array)),t}setW(e,t){return this.normalized&&(t=At(t,this.array)),this.array[e*this.itemSize+3]=t,this}setXY(e,t,n){return e*=this.itemSize,this.normalized&&(t=At(t,this.array),n=At(n,this.array)),this.array[e+0]=t,this.array[e+1]=n,this}setXYZ(e,t,n,i){return e*=this.itemSize,this.normalized&&(t=At(t,this.array),n=At(n,this.array),i=At(i,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e*=this.itemSize,this.normalized&&(t=At(t,this.array),n=At(n,this.array),i=At(i,this.array),r=At(r,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this.array[e+3]=r,this}onUpload(e){return this.onUploadCallback=e,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let e={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return e.name=this.name,e.usage=this.usage,e.gpuType=this.gpuType,e}dispose(){this.dispatchEvent({type:"dispose"})}};var ro=class extends dt{constructor(e,t,n){super(new Uint16Array(e),t,n)}};var oo=class extends dt{constructor(e,t,n){super(new Uint32Array(e),t,n)}};var tt=class extends dt{constructor(e,t,n){super(new Float32Array(e),t,n)}},Sm=new Zt,Wr=new W,Gc=new W,cn=class{constructor(e=new W,t=-1){this.isSphere=!0,this.center=e,this.radius=t}set(e,t){return this.center.copy(e),this.radius=t,this}setFromPoints(e,t){let n=this.center;t!==void 0?n.copy(t):Sm.setFromPoints(e).getCenter(n);let i=0;for(let r=0,o=e.length;r<o;r++)i=Math.max(i,n.distanceToSquared(e[r]));return this.radius=Math.sqrt(i),this}copy(e){return this.center.copy(e.center),this.radius=e.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(e){return e.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(e){return e.distanceTo(this.center)-this.radius}intersectsSphere(e){let t=this.radius+e.radius;return e.center.distanceToSquared(this.center)<=t*t}intersectsBox(e){return e.intersectsSphere(this)}intersectsPlane(e){return Math.abs(e.distanceToPoint(this.center))<=this.radius}clampPoint(e,t){let n=this.center.distanceToSquared(e);return t.copy(e),n>this.radius*this.radius&&(t.sub(this.center).normalize(),t.multiplyScalar(this.radius).add(this.center)),t}getBoundingBox(e){return this.isEmpty()?(e.makeEmpty(),e):(e.set(this.center,this.center),e.expandByScalar(this.radius),e)}applyMatrix4(e){return this.center.applyMatrix4(e),this.radius=this.radius*e.getMaxScaleOnAxis(),this}translate(e){return this.center.add(e),this}expandByPoint(e){if(this.isEmpty())return this.center.copy(e),this.radius=0,this;Wr.subVectors(e,this.center);let t=Wr.lengthSq();if(t>this.radius*this.radius){let n=Math.sqrt(t),i=(n-this.radius)*.5;this.center.addScaledVector(Wr,i/n),this.radius+=i}return this}union(e){return e.isEmpty()?this:this.isEmpty()?(this.copy(e),this):(this.center.equals(e.center)===!0?this.radius=Math.max(this.radius,e.radius):(Gc.subVectors(e.center,this.center).setLength(e.radius),this.expandByPoint(Wr.copy(e.center).add(Gc)),this.expandByPoint(Wr.copy(e.center).sub(Gc))),this)}equals(e){return e.center.equals(this.center)&&e.radius===this.radius}clone(){return new this.constructor().copy(this)}toJSON(){return{radius:this.radius,center:this.center.toArray()}}fromJSON(e){return this.radius=e.radius,this.center.fromArray(e.center),this}},wm=0,Un=new it,Wc=new Tt,Ks=new W,En=new Zt,Xr=new Zt,en=new W,ot=class s extends On{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:wm++}),this.uuid=Zn(),this.name="",this.type="BufferGeometry",this.index=null,this.indirect=null,this.indirectOffset=0,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={},this._transformed=!1}getIndex(){return this.index}setIndex(e){return Array.isArray(e)?this.index=new(Zp(e)?oo:ro)(e,1):this.index=e,this}setIndirect(e,t=0){return this.indirect=e,this.indirectOffset=t,this}getIndirect(){return this.indirect}getAttribute(e){return this.attributes[e]}setAttribute(e,t){return this.attributes[e]=t,this}deleteAttribute(e){return delete this.attributes[e],this}hasAttribute(e){return this.attributes[e]!==void 0}addGroup(e,t,n=0){this.groups.push({start:e,count:t,materialIndex:n})}clearGroups(){this.groups=[]}setDrawRange(e,t){this.drawRange.start=e,this.drawRange.count=t}applyMatrix4(e){let t=this.attributes.position;t!==void 0&&(t.applyMatrix4(e),t.needsUpdate=!0);let n=this.attributes.normal;if(n!==void 0){let r=new rt().getNormalMatrix(e);n.applyNormalMatrix(r),n.needsUpdate=!0}let i=this.attributes.tangent;return i!==void 0&&(i.transformDirection(e),i.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this._transformed=!0,this}applyQuaternion(e){return Un.makeRotationFromQuaternion(e),this.applyMatrix4(Un),this}rotateX(e){return Un.makeRotationX(e),this.applyMatrix4(Un),this}rotateY(e){return Un.makeRotationY(e),this.applyMatrix4(Un),this}rotateZ(e){return Un.makeRotationZ(e),this.applyMatrix4(Un),this}translate(e,t,n){return Un.makeTranslation(e,t,n),this.applyMatrix4(Un),this}scale(e,t,n){return Un.makeScale(e,t,n),this.applyMatrix4(Un),this}lookAt(e){return Wc.lookAt(e),Wc.updateMatrix(),this.applyMatrix4(Wc.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(Ks).negate(),this.translate(Ks.x,Ks.y,Ks.z),this}setFromPoints(e){let t=this.getAttribute("position");if(t===void 0){let n=[];for(let i=0,r=e.length;i<r;i++){let o=e[i];n.push(o.x,o.y,o.z||0)}this.setAttribute("position",new tt(n,3))}else{let n=Math.min(e.length,t.count);for(let i=0;i<n;i++){let r=e[i];t.setXYZ(i,r.x,r.y,r.z||0)}e.length>t.count&&Ze("BufferGeometry: Buffer size too small for points data. Use .dispose() and create a new geometry."),t.needsUpdate=!0}return this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new Zt);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){et("BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new W(-1/0,-1/0,-1/0),new W(1/0,1/0,1/0));return}if(e!==void 0){if(this.boundingBox.setFromBufferAttribute(e),t)for(let n=0,i=t.length;n<i;n++){let r=t[n];En.setFromBufferAttribute(r),this.morphTargetsRelative?(en.addVectors(this.boundingBox.min,En.min),this.boundingBox.expandByPoint(en),en.addVectors(this.boundingBox.max,En.max),this.boundingBox.expandByPoint(en)):(this.boundingBox.expandByPoint(En.min),this.boundingBox.expandByPoint(En.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&et('BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new cn);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){et("BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new W,1/0);return}if(e){let n=this.boundingSphere.center;if(En.setFromBufferAttribute(e),t)for(let r=0,o=t.length;r<o;r++){let a=t[r];Xr.setFromBufferAttribute(a),this.morphTargetsRelative?(en.addVectors(En.min,Xr.min),En.expandByPoint(en),en.addVectors(En.max,Xr.max),En.expandByPoint(en)):(En.expandByPoint(Xr.min),En.expandByPoint(Xr.max))}En.getCenter(n);let i=0;for(let r=0,o=e.count;r<o;r++)en.fromBufferAttribute(e,r),i=Math.max(i,n.distanceToSquared(en));if(t)for(let r=0,o=t.length;r<o;r++){let a=t[r],l=this.morphTargetsRelative;for(let c=0,h=a.count;c<h;c++)en.fromBufferAttribute(a,c),l&&(Ks.fromBufferAttribute(e,c),en.add(Ks)),i=Math.max(i,n.distanceToSquared(en))}this.boundingSphere.radius=Math.sqrt(i),isNaN(this.boundingSphere.radius)&&et('BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let e=this.index,t=this.attributes;if(e===null||t.position===void 0||t.normal===void 0||t.uv===void 0){et("BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let n=t.position,i=t.normal,r=t.uv,o=this.getAttribute("tangent");(o===void 0||o.count!==n.count)&&(o=new dt(new Float32Array(4*n.count),4),this.setAttribute("tangent",o));let a=[],l=[];for(let _=0;_<n.count;_++)a[_]=new W,l[_]=new W;let c=new W,h=new W,u=new W,d=new Ue,f=new Ue,g=new Ue,v=new W,m=new W;function p(_,I,b){c.fromBufferAttribute(n,_),h.fromBufferAttribute(n,I),u.fromBufferAttribute(n,b),d.fromBufferAttribute(r,_),f.fromBufferAttribute(r,I),g.fromBufferAttribute(r,b),h.sub(c),u.sub(c),f.sub(d),g.sub(d);let N=1/(f.x*g.y-g.x*f.y);isFinite(N)&&(v.copy(h).multiplyScalar(g.y).addScaledVector(u,-f.y).multiplyScalar(N),m.copy(u).multiplyScalar(f.x).addScaledVector(h,-g.x).multiplyScalar(N),a[_].add(v),a[I].add(v),a[b].add(v),l[_].add(m),l[I].add(m),l[b].add(m))}let M=this.groups;M.length===0&&(M=[{start:0,count:e.count}]);for(let _=0,I=M.length;_<I;++_){let b=M[_],N=b.start,w=b.count;for(let H=N,F=N+w;H<F;H+=3)p(e.getX(H+0),e.getX(H+1),e.getX(H+2))}let T=new W,x=new W,y=new W,S=new W;function C(_){y.fromBufferAttribute(i,_),S.copy(y);let I=a[_];T.copy(I),T.sub(y.multiplyScalar(y.dot(I))).normalize(),x.crossVectors(S,I);let N=x.dot(l[_])<0?-1:1;o.setXYZW(_,T.x,T.y,T.z,N)}for(let _=0,I=M.length;_<I;++_){let b=M[_],N=b.start,w=b.count;for(let H=N,F=N+w;H<F;H+=3)C(e.getX(H+0)),C(e.getX(H+1)),C(e.getX(H+2))}this._transformed=!0}computeVertexNormals(){let e=this.index,t=this.getAttribute("position");if(t!==void 0){let n=this.getAttribute("normal");if(n===void 0||n.count!==t.count)n=new dt(new Float32Array(t.count*3),3),this.setAttribute("normal",n);else for(let d=0,f=n.count;d<f;d++)n.setXYZ(d,0,0,0);let i=new W,r=new W,o=new W,a=new W,l=new W,c=new W,h=new W,u=new W;if(e)for(let d=0,f=e.count;d<f;d+=3){let g=e.getX(d+0),v=e.getX(d+1),m=e.getX(d+2);i.fromBufferAttribute(t,g),r.fromBufferAttribute(t,v),o.fromBufferAttribute(t,m),h.subVectors(o,r),u.subVectors(i,r),h.cross(u),a.fromBufferAttribute(n,g),l.fromBufferAttribute(n,v),c.fromBufferAttribute(n,m),a.add(h),l.add(h),c.add(h),n.setXYZ(g,a.x,a.y,a.z),n.setXYZ(v,l.x,l.y,l.z),n.setXYZ(m,c.x,c.y,c.z)}else for(let d=0,f=t.count;d<f;d+=3)i.fromBufferAttribute(t,d+0),r.fromBufferAttribute(t,d+1),o.fromBufferAttribute(t,d+2),h.subVectors(o,r),u.subVectors(i,r),h.cross(u),n.setXYZ(d+0,h.x,h.y,h.z),n.setXYZ(d+1,h.x,h.y,h.z),n.setXYZ(d+2,h.x,h.y,h.z);this.normalizeNormals(),n.needsUpdate=!0}}normalizeNormals(){let e=this.attributes.normal;for(let t=0,n=e.count;t<n;t++)en.fromBufferAttribute(e,t),en.normalize(),e.setXYZ(t,en.x,en.y,en.z)}toNonIndexed(){function e(a,l){let c=a.array,h=a.itemSize,u=a.normalized,d=new c.constructor(l.length*h),f=0,g=0;for(let v=0,m=l.length;v<m;v++){a.isInterleavedBufferAttribute?f=l[v]*a.data.stride+a.offset:f=l[v]*h;for(let p=0;p<h;p++)d[g++]=c[f++]}return new dt(d,h,u)}if(this.index===null)return Ze("BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let t=new s,n=this.index.array,i=this.attributes;for(let a in i){let l=i[a],c=e(l,n);t.setAttribute(a,c)}let r=this.morphAttributes;for(let a in r){let l=[],c=r[a];for(let h=0,u=c.length;h<u;h++){let d=c[h],f=e(d,n);l.push(f)}t.morphAttributes[a]=l}t.morphTargetsRelative=this.morphTargetsRelative;let o=this.groups;for(let a=0,l=o.length;a<l;a++){let c=o[a];t.addGroup(c.start,c.count,c.materialIndex)}return t}toJSON(){let e={metadata:{version:4.7,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(e.uuid=this.uuid,e.type=this.parameters!==void 0&&this._transformed===!0?"BufferGeometry":this.type,e.name=this.name,Object.keys(this.userData).length>0&&(e.userData=this.userData),this.parameters!==void 0&&this._transformed!==!0){let l=this.parameters;for(let c in l)l[c]!==void 0&&(e[c]=l[c]);return e}e.data={attributes:{}};let t=this.index;t!==null&&(e.data.index={type:t.array.constructor.name,array:Array.prototype.slice.call(t.array)});let n=this.attributes;for(let l in n){let c=n[l];e.data.attributes[l]=c.toJSON(e.data)}let i={},r=!1;for(let l in this.morphAttributes){let c=this.morphAttributes[l],h=[];for(let u=0,d=c.length;u<d;u++){let f=c[u];h.push(f.toJSON(e.data))}h.length>0&&(i[l]=h,r=!0)}r&&(e.data.morphAttributes=i,e.data.morphTargetsRelative=this.morphTargetsRelative);let o=this.groups;o.length>0&&(e.data.groups=JSON.parse(JSON.stringify(o)));let a=this.boundingSphere;return a!==null&&(e.data.boundingSphere=a.toJSON()),e}clone(){return new this.constructor().copy(this)}copy(e){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let t={};this.name=e.name;let n=e.index;n!==null&&this.setIndex(n.clone());let i=e.attributes;for(let c in i){let h=i[c];this.setAttribute(c,h.clone(t))}let r=e.morphAttributes;for(let c in r){let h=[],u=r[c];for(let d=0,f=u.length;d<f;d++)h.push(u[d].clone(t));this.morphAttributes[c]=h}this.morphTargetsRelative=e.morphTargetsRelative;let o=e.groups;for(let c=0,h=o.length;c<h;c++){let u=o[c];this.addGroup(u.start,u.count,u.materialIndex)}let a=e.boundingBox;a!==null&&(this.boundingBox=a.clone());let l=e.boundingSphere;return l!==null&&(this.boundingSphere=l.clone()),this.drawRange.start=e.drawRange.start,this.drawRange.count=e.drawRange.count,this.userData=e.userData,this._transformed=e._transformed,this}dispose(){this.dispatchEvent({type:"dispose"})}},cr=class{constructor(e,t){this.isInterleavedBuffer=!0,this.array=e,this.stride=t,this.count=e!==void 0?e.length/t:0,this.usage=Eh,this.updateRanges=[],this.version=0,this.uuid=Zn()}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.array=new e.array.constructor(e.array),this.count=e.count,this.stride=e.stride,this.usage=e.usage,this}copyAt(e,t,n){e*=this.stride,n*=t.stride;for(let i=0,r=this.stride;i<r;i++)this.array[e+i]=t.array[n+i];return this}set(e,t=0){return this.array.set(e,t),this}clone(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=Zn()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=this.array.slice(0).buffer);let t=new this.array.constructor(e.arrayBuffers[this.array.buffer._uuid]),n=new this.constructor(t,this.stride);return n.setUsage(this.usage),n}onUpload(e){return this.onUploadCallback=e,this}toJSON(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=Zn()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=Array.from(new Uint32Array(this.array.buffer)));let t={uuid:this.uuid,buffer:this.array.buffer._uuid,type:this.array.constructor.name,stride:this.stride};return t.usage=this.usage,t}},pn=new W,hr=class s{constructor(e,t,n,i=!1){this.isInterleavedBufferAttribute=!0,this.name="",this.data=e,this.itemSize=t,this.offset=n,this.normalized=i}get count(){return this.data.count}get array(){return this.data.array}set needsUpdate(e){this.data.needsUpdate=e}applyMatrix4(e){for(let t=0,n=this.data.count;t<n;t++)pn.fromBufferAttribute(this,t),pn.applyMatrix4(e),this.setXYZ(t,pn.x,pn.y,pn.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)pn.fromBufferAttribute(this,t),pn.applyNormalMatrix(e),this.setXYZ(t,pn.x,pn.y,pn.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)pn.fromBufferAttribute(this,t),pn.transformDirection(e),this.setXYZ(t,pn.x,pn.y,pn.z);return this}getComponent(e,t){let n=this.array[e*this.data.stride+this.offset+t];return this.normalized&&(n=qn(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=At(n,this.array)),this.data.array[e*this.data.stride+this.offset+t]=n,this}setX(e,t){return this.normalized&&(t=At(t,this.array)),this.data.array[e*this.data.stride+this.offset]=t,this}setY(e,t){return this.normalized&&(t=At(t,this.array)),this.data.array[e*this.data.stride+this.offset+1]=t,this}setZ(e,t){return this.normalized&&(t=At(t,this.array)),this.data.array[e*this.data.stride+this.offset+2]=t,this}setW(e,t){return this.normalized&&(t=At(t,this.array)),this.data.array[e*this.data.stride+this.offset+3]=t,this}getX(e){let t=this.data.array[e*this.data.stride+this.offset];return this.normalized&&(t=qn(t,this.array)),t}getY(e){let t=this.data.array[e*this.data.stride+this.offset+1];return this.normalized&&(t=qn(t,this.array)),t}getZ(e){let t=this.data.array[e*this.data.stride+this.offset+2];return this.normalized&&(t=qn(t,this.array)),t}getW(e){let t=this.data.array[e*this.data.stride+this.offset+3];return this.normalized&&(t=qn(t,this.array)),t}setXY(e,t,n){return e=e*this.data.stride+this.offset,this.normalized&&(t=At(t,this.array),n=At(n,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this}setXYZ(e,t,n,i){return e=e*this.data.stride+this.offset,this.normalized&&(t=At(t,this.array),n=At(n,this.array),i=At(i,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e=e*this.data.stride+this.offset,this.normalized&&(t=At(t,this.array),n=At(n,this.array),i=At(i,this.array),r=At(r,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this.data.array[e+3]=r,this}clone(e){if(e===void 0){no("InterleavedBufferAttribute.clone(): Cloning an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return new dt(new this.array.constructor(t),this.itemSize,this.normalized)}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.clone(e)),new s(e.interleavedBuffers[this.data.uuid],this.itemSize,this.offset,this.normalized)}toJSON(e){if(e===void 0){no("InterleavedBufferAttribute.toJSON(): Serializing an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return{itemSize:this.itemSize,type:this.array.constructor.name,array:t,normalized:this.normalized}}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.toJSON(e)),{isInterleavedBufferAttribute:!0,itemSize:this.itemSize,data:this.data.uuid,offset:this.offset,normalized:this.normalized}}},Xc=new W,Tm=new W,Em=new rt,An=class{constructor(e=new W(1,0,0),t=0){this.isPlane=!0,this.normal=e,this.constant=t}set(e,t){return this.normal.copy(e),this.constant=t,this}setComponents(e,t,n,i){return this.normal.set(e,t,n),this.constant=i,this}setFromNormalAndCoplanarPoint(e,t){return this.normal.copy(e),this.constant=-t.dot(this.normal),this}setFromCoplanarPoints(e,t,n){let i=Xc.subVectors(n,t).cross(Tm.subVectors(e,t)).normalize();return this.setFromNormalAndCoplanarPoint(i,e),this}copy(e){return this.normal.copy(e.normal),this.constant=e.constant,this}normalize(){let e=1/this.normal.length();return this.normal.multiplyScalar(e),this.constant*=e,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(e){return this.normal.dot(e)+this.constant}distanceToSphere(e){return this.distanceToPoint(e.center)-e.radius}projectPoint(e,t){return t.copy(e).addScaledVector(this.normal,-this.distanceToPoint(e))}intersectLine(e,t,n=!0){let i=e.delta(Xc),r=this.normal.dot(i);if(r===0)return this.distanceToPoint(e.start)===0?t.copy(e.start):null;let o=-(e.start.dot(this.normal)+this.constant)/r;return n===!0&&(o<0||o>1)?null:t.copy(e.start).addScaledVector(i,o)}intersectsLine(e){let t=this.distanceToPoint(e.start),n=this.distanceToPoint(e.end);return t<0&&n>0||n<0&&t>0}intersectsBox(e){return e.intersectsPlane(this)}intersectsSphere(e){return e.intersectsPlane(this)}coplanarPoint(e){return e.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(e,t){let n=t||Em.getNormalMatrix(e),i=this.coplanarPoint(Xc).applyMatrix4(e),r=this.normal.applyMatrix3(n).normalize();return this.constant=-i.dot(r),this}translate(e){return this.constant-=e.dot(this.normal),this}equals(e){return e.normal.equals(this.normal)&&e.constant===this.constant}clone(){return new this.constructor().copy(this)}toJSON(){return{normal:this.normal.toArray(),constant:this.constant}}fromJSON(e){return this.normal.fromArray(e.normal),this.constant=e.constant,this}},Am=0,gn=class extends On{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:Am++}),this.uuid=Zn(),this.name="",this.type="Material",this.blending=Sr,this.side=Bn,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=gh,this.blendDst=xh,this.blendEquation=Rs,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new Ce(0,0,0),this.blendAlpha=0,this.depthFunc=tr,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=rf,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=La,this.stencilZFail=La,this.stencilZPass=La,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.allowOverride=!0,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(e){this._alphaTest>0!=e>0&&this.version++,this._alphaTest=e}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(e){if(e!==void 0)for(let t in e){let n=e[t];if(n===void 0){Ze(`Material: parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){Ze(`Material: '${t}' is not a property of THREE.${this.type}.`);continue}i&&i.isColor?i.set(n):i&&i.isVector2&&n&&n.isVector2||i&&i.isEuler&&n&&n.isEuler||i&&i.isVector3&&n&&n.isVector3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";t&&(e={textures:{},images:{}});let n={metadata:{version:4.7,type:"Material",generator:"Material.toJSON"}};n.uuid=this.uuid,n.type=this.type,n.blending=this.blending,n.side=this.side,n.shadowSide=this.shadowSide,n.vertexColors=this.vertexColors,n.opacity=this.opacity,n.transparent=this.transparent,n.blendSrc=this.blendSrc,n.blendDst=this.blendDst,n.blendEquation=this.blendEquation,n.blendSrcAlpha=this.blendSrcAlpha,n.blendDstAlpha=this.blendDstAlpha,n.blendEquationAlpha=this.blendEquationAlpha,n.blendColor=this.blendColor.getHex(),n.blendAlpha=this.blendAlpha,n.depthFunc=this.depthFunc,n.depthTest=this.depthTest,n.depthWrite=this.depthWrite,n.colorWrite=this.colorWrite,n.clipIntersection=this.clipIntersection,n.clipShadows=this.clipShadows,n.stencilWriteMask=this.stencilWriteMask,n.stencilFunc=this.stencilFunc,n.stencilRef=this.stencilRef,n.stencilFuncMask=this.stencilFuncMask,n.stencilFail=this.stencilFail,n.stencilZFail=this.stencilZFail,n.stencilZPass=this.stencilZPass,n.stencilWrite=this.stencilWrite,n.polygonOffset=this.polygonOffset,n.polygonOffsetFactor=this.polygonOffsetFactor,n.polygonOffsetUnits=this.polygonOffsetUnits,n.dithering=this.dithering,n.alphaTest=this.alphaTest,n.alphaHash=this.alphaHash,n.alphaToCoverage=this.alphaToCoverage,n.premultipliedAlpha=this.premultipliedAlpha,n.forceSinglePass=this.forceSinglePass,n.allowOverride=this.allowOverride,n.visible=this.visible,n.toneMapped=this.toneMapped,n.name=this.name,this.color&&this.color.isColor&&(n.color=this.color.getHex()),this.roughness!==void 0&&(n.roughness=this.roughness),this.metalness!==void 0&&(n.metalness=this.metalness),this.sheen!==void 0&&(n.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(n.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(n.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(n.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&(n.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(n.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(n.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(n.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(n.shininess=this.shininess),this.clearcoat!==void 0&&(n.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(n.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(n.clearcoatMap=this.clearcoatMap.toJSON(e).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(n.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(e).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(n.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(e).uuid,n.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.sheenColorMap&&this.sheenColorMap.isTexture&&(n.sheenColorMap=this.sheenColorMap.toJSON(e).uuid),this.sheenRoughnessMap&&this.sheenRoughnessMap.isTexture&&(n.sheenRoughnessMap=this.sheenRoughnessMap.toJSON(e).uuid),this.dispersion!==void 0&&(n.dispersion=this.dispersion),this.retroreflectivity!==void 0&&(n.retroreflectivity=this.retroreflectivity),this.iridescence!==void 0&&(n.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(n.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(n.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(n.iridescenceMap=this.iridescenceMap.toJSON(e).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(n.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(e).uuid),this.anisotropy!==void 0&&(n.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(n.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(n.anisotropyMap=this.anisotropyMap.toJSON(e).uuid),this.map&&this.map.isTexture&&(n.map=this.map.toJSON(e).uuid),this.matcap&&this.matcap.isTexture&&(n.matcap=this.matcap.toJSON(e).uuid),this.alphaMap&&this.alphaMap.isTexture&&(n.alphaMap=this.alphaMap.toJSON(e).uuid),this.lightMap&&this.lightMap.isTexture&&(n.lightMap=this.lightMap.toJSON(e).uuid,n.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(n.aoMap=this.aoMap.toJSON(e).uuid,n.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(n.bumpMap=this.bumpMap.toJSON(e).uuid,n.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(n.normalMap=this.normalMap.toJSON(e).uuid,n.normalMapType=this.normalMapType,n.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(n.displacementMap=this.displacementMap.toJSON(e).uuid,n.displacementScale=this.displacementScale,n.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(n.roughnessMap=this.roughnessMap.toJSON(e).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(n.metalnessMap=this.metalnessMap.toJSON(e).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(n.emissiveMap=this.emissiveMap.toJSON(e).uuid),this.specularMap&&this.specularMap.isTexture&&(n.specularMap=this.specularMap.toJSON(e).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(n.specularIntensityMap=this.specularIntensityMap.toJSON(e).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(n.specularColorMap=this.specularColorMap.toJSON(e).uuid),this.envMap&&this.envMap.isTexture&&(n.envMap=this.envMap.toJSON(e).uuid,this.combine!==void 0&&(n.combine=this.combine)),this.envMapRotation!==void 0&&(n.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(n.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(n.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(n.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(n.gradientMap=this.gradientMap.toJSON(e).uuid),this.transmission!==void 0&&(n.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(n.transmissionMap=this.transmissionMap.toJSON(e).uuid),this.thickness!==void 0&&(n.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(n.thicknessMap=this.thicknessMap.toJSON(e).uuid),this.attenuationDistance!==void 0&&(n.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(n.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(n.size=this.size),this.sizeAttenuation!==void 0&&(n.sizeAttenuation=this.sizeAttenuation),Array.isArray(this.clippingPlanes)&&this.clippingPlanes.length>0&&(n.clippingPlanes=this.clippingPlanes.map(r=>r.toJSON())),this.rotation!==void 0&&(n.rotation=this.rotation),this.depthPacking!==void 0&&(n.depthPacking=this.depthPacking),this.linewidth!==void 0&&(n.linewidth=this.linewidth),this.linecap!==void 0&&(n.linecap=this.linecap),this.linejoin!==void 0&&(n.linejoin=this.linejoin),this.dashSize!==void 0&&(n.dashSize=this.dashSize),this.gapSize!==void 0&&(n.gapSize=this.gapSize),this.scale!==void 0&&(n.scale=this.scale),this.wireframe!==void 0&&(n.wireframe=this.wireframe),this.wireframeLinewidth!==void 0&&(n.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!==void 0&&(n.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!==void 0&&(n.wireframeLinejoin=this.wireframeLinejoin),this.flatShading!==void 0&&(n.flatShading=this.flatShading),this.fog!==void 0&&(n.fog=this.fog),Object.keys(this.userData).length>0&&(n.userData=this.userData);function i(r){let o=[];for(let a in r){let l=r[a];delete l.metadata,o.push(l)}return o}if(t){let r=i(e.textures),o=i(e.images);r.length>0&&(n.textures=r),o.length>0&&(n.images=o)}return n}fromJSON(e,t){if(e.uuid!==void 0&&(this.uuid=e.uuid),e.name!==void 0&&(this.name=e.name),e.color!==void 0&&this.color!==void 0&&this.color.setHex(e.color),e.roughness!==void 0&&(this.roughness=e.roughness),e.metalness!==void 0&&(this.metalness=e.metalness),e.sheen!==void 0&&(this.sheen=e.sheen),e.sheenColor!==void 0&&(this.sheenColor=new Ce().setHex(e.sheenColor)),e.sheenRoughness!==void 0&&(this.sheenRoughness=e.sheenRoughness),e.emissive!==void 0&&this.emissive!==void 0&&this.emissive.setHex(e.emissive),e.specular!==void 0&&this.specular!==void 0&&this.specular.setHex(e.specular),e.specularIntensity!==void 0&&(this.specularIntensity=e.specularIntensity),e.specularColor!==void 0&&this.specularColor!==void 0&&this.specularColor.setHex(e.specularColor),e.shininess!==void 0&&(this.shininess=e.shininess),e.clearcoat!==void 0&&(this.clearcoat=e.clearcoat),e.clearcoatRoughness!==void 0&&(this.clearcoatRoughness=e.clearcoatRoughness),e.dispersion!==void 0&&(this.dispersion=e.dispersion),e.retroreflectivity!==void 0&&(this.retroreflectivity=e.retroreflectivity),e.iridescence!==void 0&&(this.iridescence=e.iridescence),e.iridescenceIOR!==void 0&&(this.iridescenceIOR=e.iridescenceIOR),e.iridescenceThicknessRange!==void 0&&(this.iridescenceThicknessRange=e.iridescenceThicknessRange),e.transmission!==void 0&&(this.transmission=e.transmission),e.thickness!==void 0&&(this.thickness=e.thickness),e.attenuationDistance!==void 0&&(this.attenuationDistance=e.attenuationDistance),e.attenuationColor!==void 0&&this.attenuationColor!==void 0&&this.attenuationColor.setHex(e.attenuationColor),e.anisotropy!==void 0&&(this.anisotropy=e.anisotropy),e.anisotropyRotation!==void 0&&(this.anisotropyRotation=e.anisotropyRotation),e.fog!==void 0&&(this.fog=e.fog),e.flatShading!==void 0&&(this.flatShading=e.flatShading),e.blending!==void 0&&(this.blending=e.blending),e.combine!==void 0&&(this.combine=e.combine),e.side!==void 0&&(this.side=e.side),e.shadowSide!==void 0&&(this.shadowSide=e.shadowSide),e.opacity!==void 0&&(this.opacity=e.opacity),e.transparent!==void 0&&(this.transparent=e.transparent),e.alphaTest!==void 0&&(this.alphaTest=e.alphaTest),e.alphaHash!==void 0&&(this.alphaHash=e.alphaHash),e.depthFunc!==void 0&&(this.depthFunc=e.depthFunc),e.depthTest!==void 0&&(this.depthTest=e.depthTest),e.depthWrite!==void 0&&(this.depthWrite=e.depthWrite),e.colorWrite!==void 0&&(this.colorWrite=e.colorWrite),e.clippingPlanes!==void 0&&(this.clippingPlanes=e.clippingPlanes.map(n=>new An().fromJSON(n))),e.clipIntersection!==void 0&&(this.clipIntersection=e.clipIntersection),e.clipShadows!==void 0&&(this.clipShadows=e.clipShadows),e.depthPacking!==void 0&&(this.depthPacking=e.depthPacking),e.blendSrc!==void 0&&(this.blendSrc=e.blendSrc),e.blendDst!==void 0&&(this.blendDst=e.blendDst),e.blendEquation!==void 0&&(this.blendEquation=e.blendEquation),e.blendSrcAlpha!==void 0&&(this.blendSrcAlpha=e.blendSrcAlpha),e.blendDstAlpha!==void 0&&(this.blendDstAlpha=e.blendDstAlpha),e.blendEquationAlpha!==void 0&&(this.blendEquationAlpha=e.blendEquationAlpha),e.blendColor!==void 0&&this.blendColor!==void 0&&this.blendColor.setHex(e.blendColor),e.blendAlpha!==void 0&&(this.blendAlpha=e.blendAlpha),e.stencilWriteMask!==void 0&&(this.stencilWriteMask=e.stencilWriteMask),e.stencilFunc!==void 0&&(this.stencilFunc=e.stencilFunc),e.stencilRef!==void 0&&(this.stencilRef=e.stencilRef),e.stencilFuncMask!==void 0&&(this.stencilFuncMask=e.stencilFuncMask),e.stencilFail!==void 0&&(this.stencilFail=e.stencilFail),e.stencilZFail!==void 0&&(this.stencilZFail=e.stencilZFail),e.stencilZPass!==void 0&&(this.stencilZPass=e.stencilZPass),e.stencilWrite!==void 0&&(this.stencilWrite=e.stencilWrite),e.wireframe!==void 0&&(this.wireframe=e.wireframe),e.wireframeLinewidth!==void 0&&(this.wireframeLinewidth=e.wireframeLinewidth),e.wireframeLinecap!==void 0&&(this.wireframeLinecap=e.wireframeLinecap),e.wireframeLinejoin!==void 0&&(this.wireframeLinejoin=e.wireframeLinejoin),e.rotation!==void 0&&(this.rotation=e.rotation),e.linewidth!==void 0&&(this.linewidth=e.linewidth),e.linecap!==void 0&&(this.linecap=e.linecap),e.linejoin!==void 0&&(this.linejoin=e.linejoin),e.dashSize!==void 0&&(this.dashSize=e.dashSize),e.gapSize!==void 0&&(this.gapSize=e.gapSize),e.scale!==void 0&&(this.scale=e.scale),e.polygonOffset!==void 0&&(this.polygonOffset=e.polygonOffset),e.polygonOffsetFactor!==void 0&&(this.polygonOffsetFactor=e.polygonOffsetFactor),e.polygonOffsetUnits!==void 0&&(this.polygonOffsetUnits=e.polygonOffsetUnits),e.dithering!==void 0&&(this.dithering=e.dithering),e.alphaToCoverage!==void 0&&(this.alphaToCoverage=e.alphaToCoverage),e.premultipliedAlpha!==void 0&&(this.premultipliedAlpha=e.premultipliedAlpha),e.forceSinglePass!==void 0&&(this.forceSinglePass=e.forceSinglePass),e.allowOverride!==void 0&&(this.allowOverride=e.allowOverride),e.visible!==void 0&&(this.visible=e.visible),e.toneMapped!==void 0&&(this.toneMapped=e.toneMapped),e.userData!==void 0&&(this.userData=e.userData),e.vertexColors!==void 0&&(typeof e.vertexColors=="number"?this.vertexColors=e.vertexColors>0:this.vertexColors=e.vertexColors),e.size!==void 0&&(this.size=e.size),e.sizeAttenuation!==void 0&&(this.sizeAttenuation=e.sizeAttenuation),e.map!==void 0&&(this.map=t[e.map]||null),e.matcap!==void 0&&(this.matcap=t[e.matcap]||null),e.alphaMap!==void 0&&(this.alphaMap=t[e.alphaMap]||null),e.bumpMap!==void 0&&(this.bumpMap=t[e.bumpMap]||null),e.bumpScale!==void 0&&(this.bumpScale=e.bumpScale),e.normalMap!==void 0&&(this.normalMap=t[e.normalMap]||null),e.normalMapType!==void 0&&(this.normalMapType=e.normalMapType),e.normalScale!==void 0){let n=e.normalScale;Array.isArray(n)===!1&&(n=[n,n]),this.normalScale=new Ue().fromArray(n)}return e.displacementMap!==void 0&&(this.displacementMap=t[e.displacementMap]||null),e.displacementScale!==void 0&&(this.displacementScale=e.displacementScale),e.displacementBias!==void 0&&(this.displacementBias=e.displacementBias),e.roughnessMap!==void 0&&(this.roughnessMap=t[e.roughnessMap]||null),e.metalnessMap!==void 0&&(this.metalnessMap=t[e.metalnessMap]||null),e.emissiveMap!==void 0&&(this.emissiveMap=t[e.emissiveMap]||null),e.emissiveIntensity!==void 0&&(this.emissiveIntensity=e.emissiveIntensity),e.specularMap!==void 0&&(this.specularMap=t[e.specularMap]||null),e.specularIntensityMap!==void 0&&(this.specularIntensityMap=t[e.specularIntensityMap]||null),e.specularColorMap!==void 0&&(this.specularColorMap=t[e.specularColorMap]||null),e.envMap!==void 0&&(this.envMap=t[e.envMap]||null),e.envMapRotation!==void 0&&this.envMapRotation.fromArray(e.envMapRotation),e.envMapIntensity!==void 0&&(this.envMapIntensity=e.envMapIntensity),e.reflectivity!==void 0&&(this.reflectivity=e.reflectivity),e.refractionRatio!==void 0&&(this.refractionRatio=e.refractionRatio),e.lightMap!==void 0&&(this.lightMap=t[e.lightMap]||null),e.lightMapIntensity!==void 0&&(this.lightMapIntensity=e.lightMapIntensity),e.aoMap!==void 0&&(this.aoMap=t[e.aoMap]||null),e.aoMapIntensity!==void 0&&(this.aoMapIntensity=e.aoMapIntensity),e.gradientMap!==void 0&&(this.gradientMap=t[e.gradientMap]||null),e.clearcoatMap!==void 0&&(this.clearcoatMap=t[e.clearcoatMap]||null),e.clearcoatRoughnessMap!==void 0&&(this.clearcoatRoughnessMap=t[e.clearcoatRoughnessMap]||null),e.clearcoatNormalMap!==void 0&&(this.clearcoatNormalMap=t[e.clearcoatNormalMap]||null),e.clearcoatNormalScale!==void 0&&(this.clearcoatNormalScale=new Ue().fromArray(e.clearcoatNormalScale)),e.iridescenceMap!==void 0&&(this.iridescenceMap=t[e.iridescenceMap]||null),e.iridescenceThicknessMap!==void 0&&(this.iridescenceThicknessMap=t[e.iridescenceThicknessMap]||null),e.transmissionMap!==void 0&&(this.transmissionMap=t[e.transmissionMap]||null),e.thicknessMap!==void 0&&(this.thicknessMap=t[e.thicknessMap]||null),e.anisotropyMap!==void 0&&(this.anisotropyMap=t[e.anisotropyMap]||null),e.sheenColorMap!==void 0&&(this.sheenColorMap=t[e.sheenColorMap]||null),e.sheenRoughnessMap!==void 0&&(this.sheenRoughnessMap=t[e.sheenRoughnessMap]||null),this}clone(){return new this.constructor().copy(this)}copy(e){this.name=e.name,this.blending=e.blending,this.side=e.side,this.vertexColors=e.vertexColors,this.opacity=e.opacity,this.transparent=e.transparent,this.blendSrc=e.blendSrc,this.blendDst=e.blendDst,this.blendEquation=e.blendEquation,this.blendSrcAlpha=e.blendSrcAlpha,this.blendDstAlpha=e.blendDstAlpha,this.blendEquationAlpha=e.blendEquationAlpha,this.blendColor.copy(e.blendColor),this.blendAlpha=e.blendAlpha,this.depthFunc=e.depthFunc,this.depthTest=e.depthTest,this.depthWrite=e.depthWrite,this.stencilWriteMask=e.stencilWriteMask,this.stencilFunc=e.stencilFunc,this.stencilRef=e.stencilRef,this.stencilFuncMask=e.stencilFuncMask,this.stencilFail=e.stencilFail,this.stencilZFail=e.stencilZFail,this.stencilZPass=e.stencilZPass,this.stencilWrite=e.stencilWrite;let t=e.clippingPlanes,n=null;if(t!==null){let i=t.length;n=new Array(i);for(let r=0;r!==i;++r)n[r]=t[r].clone()}return this.clippingPlanes=n,this.clipIntersection=e.clipIntersection,this.clipShadows=e.clipShadows,this.shadowSide=e.shadowSide,this.colorWrite=e.colorWrite,this.precision=e.precision,this.polygonOffset=e.polygonOffset,this.polygonOffsetFactor=e.polygonOffsetFactor,this.polygonOffsetUnits=e.polygonOffsetUnits,this.dithering=e.dithering,this.alphaTest=e.alphaTest,this.alphaHash=e.alphaHash,this.alphaToCoverage=e.alphaToCoverage,this.premultipliedAlpha=e.premultipliedAlpha,this.forceSinglePass=e.forceSinglePass,this.allowOverride=e.allowOverride,this.visible=e.visible,this.toneMapped=e.toneMapped,this.userData=JSON.parse(JSON.stringify(e.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(e){e===!0&&this.version++}};var Ti=new W,qc=new W,fa=new W,pa=new W,ci=class{constructor(e=new W,t=new W(0,0,-1)){this.origin=e,this.direction=t}set(e,t){return this.origin.copy(e),this.direction.copy(t),this}copy(e){return this.origin.copy(e.origin),this.direction.copy(e.direction),this}at(e,t){return t.copy(this.origin).addScaledVector(this.direction,e)}lookAt(e){return this.direction.copy(e).sub(this.origin).normalize(),this}recast(e){return this.origin.copy(this.at(e,Ti)),this}closestPointToPoint(e,t){t.subVectors(e,this.origin);let n=t.dot(this.direction);return n<0?t.copy(this.origin):t.copy(this.origin).addScaledVector(this.direction,n)}distanceToPoint(e){return Math.sqrt(this.distanceSqToPoint(e))}distanceSqToPoint(e){let t=Ti.subVectors(e,this.origin).dot(this.direction);return t<0?this.origin.distanceToSquared(e):(Ti.copy(this.origin).addScaledVector(this.direction,t),Ti.distanceToSquared(e))}distanceSqToSegment(e,t,n,i){qc.copy(e).add(t).multiplyScalar(.5),fa.copy(t).sub(e).normalize(),pa.copy(this.origin).sub(qc);let r=e.distanceTo(t)*.5,o=-this.direction.dot(fa),a=pa.dot(this.direction),l=-pa.dot(fa),c=pa.lengthSq(),h=Math.abs(1-o*o),u,d,f,g;if(h>0)if(u=o*l-a,d=o*a-l,g=r*h,u>=0)if(d>=-g)if(d<=g){let v=1/h;u*=v,d*=v,f=u*(u+o*d+2*a)+d*(o*u+d+2*l)+c}else d=r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;else d=-r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;else d<=-g?(u=Math.max(0,-(-o*r+a)),d=u>0?-r:Math.min(Math.max(-r,-l),r),f=-u*u+d*(d+2*l)+c):d<=g?(u=0,d=Math.min(Math.max(-r,-l),r),f=d*(d+2*l)+c):(u=Math.max(0,-(o*r+a)),d=u>0?r:Math.min(Math.max(-r,-l),r),f=-u*u+d*(d+2*l)+c);else d=o>0?-r:r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;return n&&n.copy(this.origin).addScaledVector(this.direction,u),i&&i.copy(qc).addScaledVector(fa,d),f}intersectSphere(e,t){if(e.radius<0)return null;Ti.subVectors(e.center,this.origin);let n=Ti.dot(this.direction),i=Ti.dot(Ti)-n*n,r=e.radius*e.radius;if(i>r)return null;let o=Math.sqrt(r-i),a=n-o,l=n+o;return l<0?null:a<0?this.at(l,t):this.at(a,t)}intersectsSphere(e){return e.radius<0?!1:this.distanceSqToPoint(e.center)<=e.radius*e.radius}distanceToPlane(e){let t=e.normal.dot(this.direction);if(t===0)return e.distanceToPoint(this.origin)===0?0:null;let n=-(this.origin.dot(e.normal)+e.constant)/t;return n>=0?n:null}intersectPlane(e,t){let n=this.distanceToPlane(e);return n===null?null:this.at(n,t)}intersectsPlane(e){let t=e.distanceToPoint(this.origin);return t===0||e.normal.dot(this.direction)*t<0}intersectBox(e,t){let n,i,r,o,a,l,c=1/this.direction.x,h=1/this.direction.y,u=1/this.direction.z,d=this.origin;return c>=0?(n=(e.min.x-d.x)*c,i=(e.max.x-d.x)*c):(n=(e.max.x-d.x)*c,i=(e.min.x-d.x)*c),h>=0?(r=(e.min.y-d.y)*h,o=(e.max.y-d.y)*h):(r=(e.max.y-d.y)*h,o=(e.min.y-d.y)*h),n>o||r>i||((r>n||isNaN(n))&&(n=r),(o<i||isNaN(i))&&(i=o),u>=0?(a=(e.min.z-d.z)*u,l=(e.max.z-d.z)*u):(a=(e.max.z-d.z)*u,l=(e.min.z-d.z)*u),n>l||a>i)||((a>n||n!==n)&&(n=a),(l<i||i!==i)&&(i=l),i<0)?null:this.at(n>=0?n:i,t)}intersectsBox(e){return this.intersectBox(e,Ti)!==null}intersectTriangle(e,t,n,i,r){let o=this.origin,a=this.direction,l=a.x,c=a.y,h=a.z,u=e.x-o.x,d=e.y-o.y,f=e.z-o.z,g=t.x-o.x,v=t.y-o.y,m=t.z-o.z,p=n.x-o.x,M=n.y-o.y,T=n.z-o.z,x=Math.abs(l),y=Math.abs(c),S=Math.abs(h),C,_,I,b,N,w,H,F,L,O,V,R;if(x>=y&&x>=S?(I=l,w=u,L=g,R=p,l>=0?(C=c,_=h,b=d,N=f,H=v,F=m,O=M,V=T):(C=h,_=c,b=f,N=d,H=m,F=v,O=T,V=M)):y>=S?(I=c,w=d,L=v,R=M,c>=0?(C=h,_=l,b=f,N=u,H=m,F=g,O=T,V=p):(C=l,_=h,b=u,N=f,H=g,F=m,O=p,V=T)):(I=h,w=f,L=m,R=T,h>=0?(C=l,_=c,b=u,N=d,H=g,F=v,O=p,V=M):(C=c,_=l,b=d,N=u,H=v,F=g,O=M,V=p)),I===0)return null;let G=C/I,oe=_/I,k=1/I,re=b-G*w,Ae=N-oe*w,Le=H-G*L,be=F-oe*L,Fe=O-G*R,le=V-oe*R,ge=Fe*be-le*Le,Re=re*le-Ae*Fe,de=Le*Ae-be*re;if(i){if(ge<0||Re<0||de<0)return null}else if((ge<0||Re<0||de<0)&&(ge>0||Re>0||de>0))return null;let he=ge+Re+de;if(he===0)return null;let ce=k*(ge*w+Re*L+de*R);return(he>0?ce<0:ce>0)?null:this.at(ce/he,r)}applyMatrix4(e){return this.origin.applyMatrix4(e),this.direction.transformDirection(e),this}equals(e){return e.origin.equals(this.origin)&&e.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},wt=class extends gn{constructor(e){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new Ce(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new xn,this.combine=fl,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.fog=e.fog,this}},ed=new it,ps=new ci,ma=new cn,td=new W,ga=new W,xa=new W,_a=new W,Yc=new W,va=new W,nd=new W,ya=new W,Ke=class extends Tt{constructor(e=new ot,t=new wt){super(),this.isMesh=!0,this.type="Mesh",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.count=1,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),e.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=e.morphTargetInfluences.slice()),e.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},e.morphTargetDictionary)),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}getVertexPosition(e,t){let n=this.geometry,i=n.attributes.position,r=n.morphAttributes.position,o=n.morphTargetsRelative;t.fromBufferAttribute(i,e);let a=this.morphTargetInfluences;if(r&&a){va.set(0,0,0);for(let l=0,c=r.length;l<c;l++){let h=a[l],u=r[l];h!==0&&(Yc.fromBufferAttribute(u,e),o?va.addScaledVector(Yc,h):va.addScaledVector(Yc.sub(t),h))}t.add(va)}return t}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.material,r=this.matrixWorld;i!==void 0&&(n.boundingSphere===null&&n.computeBoundingSphere(),ma.copy(n.boundingSphere),ma.applyMatrix4(r),ps.copy(e.ray).recast(e.near),!(ma.containsPoint(ps.origin)===!1&&(ps.intersectSphere(ma,td)===null||ps.origin.distanceToSquared(td)>(e.far-e.near)**2))&&(ed.copy(r).invert(),ps.copy(e.ray).applyMatrix4(ed),!(n.boundingBox!==null&&ps.intersectsBox(n.boundingBox)===!1)&&this._computeIntersections(e,t,ps)))}_computeIntersections(e,t,n){let i,r=this.geometry,o=this.material,a=r.index,l=r.attributes.position,c=r.attributes.uv,h=r.attributes.uv1,u=r.attributes.normal,d=r.groups,f=r.drawRange;if(a!==null)if(Array.isArray(o))for(let g=0,v=d.length;g<v;g++){let m=d[g],p=o[m.materialIndex],M=Math.max(m.start,f.start),T=Math.min(a.count,Math.min(m.start+m.count,f.start+f.count));for(let x=M,y=T;x<y;x+=3){let S=a.getX(x),C=a.getX(x+1),_=a.getX(x+2);i=Ma(this,p,e,n,c,h,u,S,C,_),i&&(i.faceIndex=Math.floor(x/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,f.start),v=Math.min(a.count,f.start+f.count);for(let m=g,p=v;m<p;m+=3){let M=a.getX(m),T=a.getX(m+1),x=a.getX(m+2);i=Ma(this,o,e,n,c,h,u,M,T,x),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}else if(l!==void 0)if(Array.isArray(o))for(let g=0,v=d.length;g<v;g++){let m=d[g],p=o[m.materialIndex],M=Math.max(m.start,f.start),T=Math.min(l.count,Math.min(m.start+m.count,f.start+f.count));for(let x=M,y=T;x<y;x+=3){let S=x,C=x+1,_=x+2;i=Ma(this,p,e,n,c,h,u,S,C,_),i&&(i.faceIndex=Math.floor(x/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,f.start),v=Math.min(l.count,f.start+f.count);for(let m=g,p=v;m<p;m+=3){let M=m,T=m+1,x=m+2;i=Ma(this,o,e,n,c,h,u,M,T,x),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}}};function Rm(s,e,t,n,i,r,o,a){let l;if(e.side===Kt?l=n.intersectTriangle(o,r,i,!0,a):l=n.intersectTriangle(i,r,o,e.side===Bn,a),l===null)return null;ya.copy(a),ya.applyMatrix4(s.matrixWorld);let c=t.ray.origin.distanceTo(ya);return c<t.near||c>t.far?null:{distance:c,point:ya.clone(),object:s}}function Ma(s,e,t,n,i,r,o,a,l,c){s.getVertexPosition(a,ga),s.getVertexPosition(l,xa),s.getVertexPosition(c,_a);let h=Rm(s,e,t,n,ga,xa,_a,nd);if(h){let u=new W;Ki.getBarycoord(nd,ga,xa,_a,u),i&&(h.uv=Ki.getInterpolatedAttribute(i,a,l,c,u,new Ue)),r&&(h.uv1=Ki.getInterpolatedAttribute(r,a,l,c,u,new Ue)),o&&(h.normal=Ki.getInterpolatedAttribute(o,a,l,c,u,new W),h.normal.dot(n.direction)>0&&h.normal.multiplyScalar(-1));let d={a,b:l,c,normal:new W,materialIndex:0};Ki.getNormal(ga,xa,_a,d.normal),h.face=d,h.barycoord=u}return h}var qr=new gt,id=new gt,sd=new gt,Cm=new gt,rd=new it,ba=new W,Zc=new cn,od=new it,Kc=new ci,ao=class extends Ke{constructor(e,t){super(e,t),this.isSkinnedMesh=!0,this.type="SkinnedMesh",this.bindMode=nh,this.bindMatrix=new it,this.bindMatrixInverse=new it,this.boundingBox=null,this.boundingSphere=null}computeBoundingBox(){let e=this.geometry;this.boundingBox===null&&(this.boundingBox=new Zt),this.boundingBox.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,ba),this.boundingBox.expandByPoint(ba)}computeBoundingSphere(){let e=this.geometry;this.boundingSphere===null&&(this.boundingSphere=new cn),this.boundingSphere.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,ba),this.boundingSphere.expandByPoint(ba)}copy(e,t){return super.copy(e,t),this.bindMode=e.bindMode,this.bindMatrix.copy(e.bindMatrix),this.bindMatrixInverse.copy(e.bindMatrixInverse),this.skeleton=e.skeleton,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}raycast(e,t){let n=this.material,i=this.matrixWorld;n!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Zc.copy(this.boundingSphere),Zc.applyMatrix4(i),e.ray.intersectsSphere(Zc)!==!1&&(od.copy(i).invert(),Kc.copy(e.ray).applyMatrix4(od),!(this.boundingBox!==null&&Kc.intersectsBox(this.boundingBox)===!1)&&this._computeIntersections(e,t,Kc)))}getVertexPosition(e,t){return super.getVertexPosition(e,t),this.applyBoneTransform(e,t),t}bind(e,t){this.skeleton=e,t===void 0&&(this.updateMatrixWorld(!0),this.skeleton.calculateInverses(),t=this.matrixWorld),this.bindMatrix.copy(t),this.bindMatrixInverse.copy(t).invert()}pose(){this.skeleton.pose()}normalizeSkinWeights(){let e=new gt,t=this.geometry.attributes.skinWeight;for(let n=0,i=t.count;n<i;n++){e.fromBufferAttribute(t,n);let r=1/e.manhattanLength();r!==1/0?e.multiplyScalar(r):e.set(1,0,0,0),t.setXYZW(n,e.x,e.y,e.z,e.w)}}updateMatrixWorld(e){super.updateMatrixWorld(e),this.bindMode===nh?this.bindMatrixInverse.copy(this.matrixWorld).invert():this.bindMode===$d?this.bindMatrixInverse.copy(this.bindMatrix).invert():Ze("SkinnedMesh: Unrecognized bindMode: "+this.bindMode)}applyBoneTransform(e,t){let n=this.skeleton,i=this.geometry;id.fromBufferAttribute(i.attributes.skinIndex,e),sd.fromBufferAttribute(i.attributes.skinWeight,e),t.isVector4?(qr.copy(t),t.set(0,0,0,0)):(qr.set(...t,1),t.set(0,0,0)),qr.applyMatrix4(this.bindMatrix);for(let r=0;r<4;r++){let o=sd.getComponent(r);if(o!==0){let a=id.getComponent(r);rd.multiplyMatrices(n.bones[a].matrixWorld,n.boneInverses[a]),t.addScaledVector(Cm.copy(qr).applyMatrix4(rd),o)}}return t.isVector4&&(t.w=qr.w),t.applyMatrix4(this.bindMatrixInverse)}},ur=class extends Tt{constructor(){super(),this.isBone=!0,this.type="Bone"}},ji=class extends Gt{constructor(e=null,t=1,n=1,i,r,o,a,l,c=Vt,h=Vt,u,d){super(null,o,a,l,c,h,i,r,u,d),this.isDataTexture=!0,this.image={data:e,width:t,height:n},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}},ad=new it,Pm=new it,lo=class s{constructor(e=[],t=[]){this.uuid=Zn(),this.bones=e.slice(0),this.boneInverses=t,this.boneMatrices=null,this.boneTexture=null,this.init()}init(){let e=this.bones,t=this.boneInverses;if(this.boneMatrices=new Float32Array(e.length*16),t.length===0)this.calculateInverses();else if(e.length!==t.length){Ze("Skeleton: Number of inverse bone matrices does not match amount of bones."),this.boneInverses=[];for(let n=0,i=this.bones.length;n<i;n++)this.boneInverses.push(new it)}}calculateInverses(){this.boneInverses.length=0;for(let e=0,t=this.bones.length;e<t;e++){let n=new it;this.bones[e]&&n.copy(this.bones[e].matrixWorld).invert(),this.boneInverses.push(n)}}pose(){for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&n.matrixWorld.copy(this.boneInverses[e]).invert()}for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&(n.parent&&n.parent.isBone?(n.matrix.copy(n.parent.matrixWorld).invert(),n.matrix.multiply(n.matrixWorld)):n.matrix.copy(n.matrixWorld),n.matrix.decompose(n.position,n.quaternion,n.scale))}}update(){let e=this.bones,t=this.boneInverses,n=this.boneMatrices,i=this.boneTexture;for(let r=0,o=e.length;r<o;r++){let a=e[r]?e[r].matrixWorld:Pm;ad.multiplyMatrices(a,t[r]),ad.toArray(n,r*16)}i!==null&&(i.needsUpdate=!0)}clone(){return new s(this.bones,this.boneInverses)}computeBoneTexture(){let e=Math.sqrt(this.bones.length*4);e=Math.ceil(e/4)*4,e=Math.max(e,4);let t=new Float32Array(e*e*4);t.set(this.boneMatrices);let n=new ji(t,e,e,In,Pn);return n.needsUpdate=!0,this.boneMatrices=t,this.boneTexture=n,this}getBoneByName(e){for(let t=0,n=this.bones.length;t<n;t++){let i=this.bones[t];if(i.name===e)return i}}dispose(){this.boneTexture!==null&&(this.boneTexture.dispose(),this.boneTexture=null)}fromJSON(e,t){this.uuid=e.uuid;for(let n=0,i=e.bones.length;n<i;n++){let r=e.bones[n],o=t[r];o===void 0&&(Ze("Skeleton: No bone found with UUID:",r),o=new ur),this.bones.push(o),this.boneInverses.push(new it().fromArray(e.boneInverses[n]))}return this.init(),this}toJSON(){let e={metadata:{version:4.7,type:"Skeleton",generator:"Skeleton.toJSON"},bones:[],boneInverses:[]};e.uuid=this.uuid;let t=this.bones,n=this.boneInverses;for(let i=0,r=t.length;i<r;i++){let o=t[i];e.bones.push(o.uuid);let a=n[i];e.boneInverses.push(a.toArray())}return e}},Ri=class extends dt{constructor(e,t,n,i=1){super(e,t,n),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=i}copy(e){return super.copy(e),this.meshPerAttribute=e.meshPerAttribute,this}toJSON(){let e=super.toJSON();return e.meshPerAttribute=this.meshPerAttribute,e.isInstancedBufferAttribute=!0,e}},js=new it,ld=new it,Sa=[],cd=new Zt,Im=new it,Yr=new Ke,Zr=new cn,_n=class extends Ke{constructor(e,t,n){super(e,t),this.isInstancedMesh=!0,this.instanceMatrix=new Ri(new Float32Array(n*16),16),this.instanceColor=null,this.morphTexture=null,this.count=n,this.boundingBox=null,this.boundingSphere=null;for(let i=0;i<n;i++)this.setMatrixAt(i,Im)}computeBoundingBox(){let e=this.geometry,t=this.count;this.boundingBox===null&&(this.boundingBox=new Zt),e.boundingBox===null&&e.computeBoundingBox(),this.boundingBox.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,js),cd.copy(e.boundingBox).applyMatrix4(js),this.boundingBox.union(cd)}computeBoundingSphere(){let e=this.geometry,t=this.count;this.boundingSphere===null&&(this.boundingSphere=new cn),e.boundingSphere===null&&e.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,js),Zr.copy(e.boundingSphere).applyMatrix4(js),this.boundingSphere.union(Zr)}copy(e,t){return super.copy(e,t),this.instanceMatrix.copy(e.instanceMatrix),e.morphTexture!==null&&(this.morphTexture=e.morphTexture.clone()),e.instanceColor!==null&&(this.instanceColor=e.instanceColor.clone()),this.count=e.count,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}getColorAt(e,t){return this.instanceColor===null?t.setRGB(1,1,1):t.fromArray(this.instanceColor.array,e*3)}getMatrixAt(e,t){return t.fromArray(this.instanceMatrix.array,e*16)}getMorphAt(e,t){let n=t.morphTargetInfluences,i=this.morphTexture.source.data.data,r=n.length+1,o=e*r+1;for(let a=0;a<n.length;a++)n[a]=i[o+a]}raycast(e,t){let n=this.matrixWorld,i=this.count;if(Yr.geometry=this.geometry,Yr.material=this.material,Yr.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Zr.copy(this.boundingSphere),Zr.applyMatrix4(n),e.ray.intersectsSphere(Zr)!==!1))for(let r=0;r<i;r++){this.getMatrixAt(r,js),ld.multiplyMatrices(n,js),Yr.matrixWorld=ld,Yr.raycast(e,Sa);for(let o=0,a=Sa.length;o<a;o++){let l=Sa[o];l.instanceId=r,l.object=this,t.push(l)}Sa.length=0}}setColorAt(e,t){return this.instanceColor===null&&(this.instanceColor=new Ri(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),t.toArray(this.instanceColor.array,e*3),this}setMatrixAt(e,t){return t.toArray(this.instanceMatrix.array,e*16),this}setMorphAt(e,t){let n=t.morphTargetInfluences,i=n.length+1;this.morphTexture===null&&(this.morphTexture=new ji(new Float32Array(i*this.count),i,this.count,yl,Pn));let r=this.morphTexture.source.data.data,o=0;for(let c=0;c<n.length;c++)o+=n[c];let a=this.geometry.morphTargetsRelative?1:1-o,l=i*e;return r[l]=a,r.set(n,l+1),this}updateMorphTargets(){}dispose(){super.dispose(),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null)}},ms=new cn,Lm=new Ue(.5,.5),wa=new W,dr=class{constructor(e=new An,t=new An,n=new An,i=new An,r=new An,o=new An){this.planes=[e,t,n,i,r,o]}set(e,t,n,i,r,o){let a=this.planes;return a[0].copy(e),a[1].copy(t),a[2].copy(n),a[3].copy(i),a[4].copy(r),a[5].copy(o),this}copy(e){let t=this.planes;for(let n=0;n<6;n++)t[n].copy(e.planes[n]);return this}setFromProjectionMatrix(e,t=Yn,n=!1){let i=this.planes,r=e.elements,o=r[0],a=r[1],l=r[2],c=r[3],h=r[4],u=r[5],d=r[6],f=r[7],g=r[8],v=r[9],m=r[10],p=r[11],M=r[12],T=r[13],x=r[14],y=r[15];if(i[0].setComponents(c-o,f-h,p-g,y-M).normalize(),i[1].setComponents(c+o,f+h,p+g,y+M).normalize(),i[2].setComponents(c+a,f+u,p+v,y+T).normalize(),i[3].setComponents(c-a,f-u,p-v,y-T).normalize(),n)i[4].setComponents(l,d,m,x).normalize(),i[5].setComponents(c-l,f-d,p-m,y-x).normalize();else if(i[4].setComponents(c-l,f-d,p-m,y-x).normalize(),t===Yn)i[5].setComponents(c+l,f+d,p+m,y+x).normalize();else if(t===ir)i[5].setComponents(l,d,m,x).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+t);return this}intersectsObject(e){if(e.boundingSphere!==void 0)e.boundingSphere===null&&e.computeBoundingSphere(),ms.copy(e.boundingSphere).applyMatrix4(e.matrixWorld);else{let t=e.geometry;t.boundingSphere===null&&t.computeBoundingSphere(),ms.copy(t.boundingSphere).applyMatrix4(e.matrixWorld)}return this.intersectsSphere(ms)}intersectsSprite(e){ms.center.set(0,0,0);let t=Lm.distanceTo(e.center);return ms.radius=.7071067811865476+t,ms.applyMatrix4(e.matrixWorld),this.intersectsSphere(ms)}intersectsSphere(e){let t=this.planes,n=e.center,i=-e.radius;for(let r=0;r<6;r++)if(t[r].distanceToPoint(n)<i)return!1;return!0}intersectsBox(e){let t=this.planes;for(let n=0;n<6;n++){let i=t[n];if(wa.x=i.normal.x>0?e.max.x:e.min.x,wa.y=i.normal.y>0?e.max.y:e.min.y,wa.z=i.normal.z>0?e.max.z:e.min.z,i.distanceToPoint(wa)<0)return!1}return!0}containsPoint(e){let t=this.planes;for(let n=0;n<6;n++)if(t[n].distanceToPoint(e)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};var hi=class extends gn{constructor(e){super(),this.isLineBasicMaterial=!0,this.type="LineBasicMaterial",this.color=new Ce(16777215),this.map=null,this.linewidth=1,this.linecap="round",this.linejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.linewidth=e.linewidth,this.linecap=e.linecap,this.linejoin=e.linejoin,this.fog=e.fog,this}},Wa=new W,Xa=new W,hd=new it,Kr=new ci,Ta=new cn,jc=new W,ud=new W,Ci=class extends Tt{constructor(e=new ot,t=new hi){super(),this.isLine=!0,this.type="Line",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[0];for(let i=1,r=t.count;i<r;i++)Wa.fromBufferAttribute(t,i-1),Xa.fromBufferAttribute(t,i),n[i]=n[i-1],n[i]+=Wa.distanceTo(Xa);e.setAttribute("lineDistance",new tt(n,1))}else Ze("Line.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Line.threshold,o=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),Ta.copy(n.boundingSphere),Ta.applyMatrix4(i),Ta.radius+=r,e.ray.intersectsSphere(Ta)===!1)return;hd.copy(i).invert(),Kr.copy(e.ray).applyMatrix4(hd);let a=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=a*a,c=this.isLineSegments?2:1,h=n.index,d=n.attributes.position;if(h!==null){let f=Math.max(0,o.start),g=Math.min(h.count,o.start+o.count);for(let v=f,m=g-1;v<m;v+=c){let p=h.getX(v),M=h.getX(v+1),T=Ea(this,e,Kr,l,p,M,v);T&&t.push(T)}if(this.isLineLoop){let v=h.getX(g-1),m=h.getX(f),p=Ea(this,e,Kr,l,v,m,g-1);p&&t.push(p)}}else{let f=Math.max(0,o.start),g=Math.min(d.count,o.start+o.count);for(let v=f,m=g-1;v<m;v+=c){let p=Ea(this,e,Kr,l,v,v+1,v);p&&t.push(p)}if(this.isLineLoop){let v=Ea(this,e,Kr,l,g-1,f,g-1);v&&t.push(v)}}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}};function Ea(s,e,t,n,i,r,o){let a=s.geometry.attributes.position;if(Wa.fromBufferAttribute(a,i),Xa.fromBufferAttribute(a,r),t.distanceSqToSegment(Wa,Xa,jc,ud)>n)return;jc.applyMatrix4(s.matrixWorld);let c=e.ray.origin.distanceTo(jc);if(!(c<e.near||c>e.far))return{distance:c,point:ud.clone().applyMatrix4(s.matrixWorld),index:o,face:null,faceIndex:null,barycoord:null,object:s}}var dd=new W,fd=new W,Ss=class extends Ci{constructor(e,t){super(e,t),this.isLineSegments=!0,this.type="LineSegments"}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[];for(let i=0,r=t.count;i<r;i+=2)dd.fromBufferAttribute(t,i),fd.fromBufferAttribute(t,i+1),n[i]=i===0?0:n[i-1],n[i+1]=n[i]+dd.distanceTo(fd);e.setAttribute("lineDistance",new tt(n,1))}else Ze("LineSegments.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}},co=class extends Ci{constructor(e,t){super(e,t),this.isLineLoop=!0,this.type="LineLoop"}},fr=class extends gn{constructor(e){super(),this.isPointsMaterial=!0,this.type="PointsMaterial",this.color=new Ce(16777215),this.map=null,this.alphaMap=null,this.size=1,this.sizeAttenuation=!0,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.alphaMap=e.alphaMap,this.size=e.size,this.sizeAttenuation=e.sizeAttenuation,this.fog=e.fog,this}},pd=new it,sh=new ci,Aa=new cn,Ra=new W,tn=class extends Tt{constructor(e=new ot,t=new fr){super(),this.isPoints=!0,this.type="Points",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}intersectsFrustum(e){return e.intersectsObject(this)}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Points.threshold,o=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),Aa.copy(n.boundingSphere),Aa.applyMatrix4(i),Aa.radius+=r,e.ray.intersectsSphere(Aa)===!1)return;pd.copy(i).invert(),sh.copy(e.ray).applyMatrix4(pd);let a=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=a*a,c=n.index,u=n.attributes.position;if(c!==null){let d=Math.max(0,o.start),f=Math.min(c.count,o.start+o.count);for(let g=d,v=f;g<v;g++){let m=c.getX(g);Ra.fromBufferAttribute(u,m),md(Ra,m,l,i,e,t,this)}}else{let d=Math.max(0,o.start),f=Math.min(u.count,o.start+o.count);for(let g=d,v=f;g<v;g++)Ra.fromBufferAttribute(u,g),md(Ra,g,l,i,e,t,this)}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}};function md(s,e,t,n,i,r,o){let a=sh.distanceSqToPoint(s);if(a<t){let l=new W;sh.closestPointToPoint(s,l),l.applyMatrix4(n);let c=i.ray.origin.distanceTo(l);if(c<i.near||c>i.far)return;r.push({distance:c,distanceToRay:Math.sqrt(a),point:l,index:e,face:null,faceIndex:null,barycoord:null,object:o})}}var ho=class extends Gt{constructor(e=[],t=is,n,i,r,o,a,l,c,h){super(e,t,n,i,r,o,a,l,c,h),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(e){this.image=e}},ws=class extends Gt{constructor(e,t,n,i,r,o,a,l,c){super(e,t,n,i,r,o,a,l,c),this.isCanvasTexture=!0,this.needsUpdate=!0}};var Ji=class extends Gt{constructor(e,t,n=Qn,i,r,o,a=Vt,l=Vt,c,h=li,u=1){if(h!==li&&h!==ss)throw new Error("THREE.DepthTexture: format must be either THREE.DepthFormat or THREE.DepthStencilFormat");let d={width:e,height:t,depth:u};super(d,i,r,o,a,l,h,n,c),this.isDepthTexture=!0,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(e){return super.copy(e),this.source=new or(Object.assign({},e.image)),this.compareFunction=e.compareFunction,this}toJSON(e){let t=super.toJSON(e);return t.compareFunction=this.compareFunction,t}},qa=class extends Ji{constructor(e,t=Qn,n=is,i,r,o=Vt,a=Vt,l,c=li){let h={width:e,height:e,depth:1},u=[h,h,h,h,h,h];super(e,e,t,n,i,r,o,a,l,c),this.image=u,this.isCubeDepthTexture=!0,this.isCubeTexture=!0}get images(){return this.image}set images(e){this.image=e}},uo=class extends Gt{constructor(e=null){super(),this.sourceTexture=e,this.isExternalTexture=!0}copy(e){return super.copy(e),this.sourceTexture=e.sourceTexture,this}},ui=class s extends ot{constructor(e=1,t=1,n=1,i=1,r=1,o=1){super(),this.type="BoxGeometry",this.parameters={width:e,height:t,depth:n,widthSegments:i,heightSegments:r,depthSegments:o};let a=this;i=Math.floor(i),r=Math.floor(r),o=Math.floor(o);let l=[],c=[],h=[],u=[],d=0,f=0;g("z","y","x",-1,-1,n,t,e,o,r,0),g("z","y","x",1,-1,n,t,-e,o,r,1),g("x","z","y",1,1,e,n,t,i,o,2),g("x","z","y",1,-1,e,n,-t,i,o,3),g("x","y","z",1,-1,e,t,n,i,r,4),g("x","y","z",-1,-1,e,t,-n,i,r,5),this.setIndex(l),this.setAttribute("position",new tt(c,3)),this.setAttribute("normal",new tt(h,3)),this.setAttribute("uv",new tt(u,2));function g(v,m,p,M,T,x,y,S,C,_,I){let b=x/C,N=y/_,w=x/2,H=y/2,F=S/2,L=C+1,O=_+1,V=0,R=0,G=new W;for(let oe=0;oe<O;oe++){let k=oe*N-H;for(let re=0;re<L;re++){let Ae=re*b-w;G[v]=Ae*M,G[m]=k*T,G[p]=F,c.push(G.x,G.y,G.z),G[v]=0,G[m]=0,G[p]=S>0?1:-1,h.push(G.x,G.y,G.z),u.push(re/C),u.push(1-oe/_),V+=1}}for(let oe=0;oe<_;oe++)for(let k=0;k<C;k++){let re=d+k+L*oe,Ae=d+k+L*(oe+1),Le=d+(k+1)+L*(oe+1),be=d+(k+1)+L*oe;l.push(re,Ae,be),l.push(Ae,Le,be),R+=6}a.addGroup(f,R,I),f+=R,d+=V}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.depth,e.widthSegments,e.heightSegments,e.depthSegments)}};var pr=class s extends ot{constructor(e=1,t=32,n=0,i=Math.PI*2){super(),this.type="CircleGeometry",this.parameters={radius:e,segments:t,thetaStart:n,thetaLength:i},t=Math.max(3,t);let r=[],o=[],a=[],l=[],c=new W,h=new Ue;o.push(0,0,0),a.push(0,0,1),l.push(.5,.5);for(let u=0,d=3;u<=t;u++,d+=3){let f=n+u/t*i;c.x=e*Math.cos(f),c.y=e*Math.sin(f),o.push(c.x,c.y,c.z),a.push(0,0,1),h.x=(o[d]/e+1)/2,h.y=(o[d+1]/e+1)/2,l.push(h.x,h.y)}for(let u=1;u<=t;u++)r.push(u,u+1,0);this.setIndex(r),this.setAttribute("position",new tt(o,3)),this.setAttribute("normal",new tt(a,3)),this.setAttribute("uv",new tt(l,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.segments,e.thetaStart,e.thetaLength)}},$i=class s extends ot{constructor(e=1,t=1,n=1,i=32,r=1,o=!1,a=0,l=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:e,radiusBottom:t,height:n,radialSegments:i,heightSegments:r,openEnded:o,thetaStart:a,thetaLength:l};let c=this;i=Math.floor(i),r=Math.floor(r);let h=[],u=[],d=[],f=[],g=0,v=[],m=n/2,p=0;M(),o===!1&&(e>0&&T(!0),t>0&&T(!1)),this.setIndex(h),this.setAttribute("position",new tt(u,3)),this.setAttribute("normal",new tt(d,3)),this.setAttribute("uv",new tt(f,2));function M(){let x=new W,y=new W,S=0,C=(t-e)/n;for(let _=0;_<=r;_++){let I=[],b=_/r,N=b*(t-e)+e;for(let w=0;w<=i;w++){let H=w/i,F=H*l+a,L=Math.sin(F),O=Math.cos(F);y.x=N*L,y.y=-b*n+m,y.z=N*O,u.push(y.x,y.y,y.z),x.set(L,C,O).normalize(),d.push(x.x,x.y,x.z),f.push(H,1-b),I.push(g++)}v.push(I)}for(let _=0;_<i;_++)for(let I=0;I<r;I++){let b=v[I][_],N=v[I+1][_],w=v[I+1][_+1],H=v[I][_+1];(e>0||I!==0)&&(h.push(b,N,H),S+=3),(t>0||I!==r-1)&&(h.push(N,w,H),S+=3)}c.addGroup(p,S,0),p+=S}function T(x){let y=g,S=new Ue,C=new W,_=0,I=x===!0?e:t,b=x===!0?1:-1;for(let w=1;w<=i;w++)u.push(0,m*b,0),d.push(0,b,0),f.push(.5,.5),g++;let N=g;for(let w=0;w<=i;w++){let F=w/i*l+a,L=Math.cos(F),O=Math.sin(F);C.x=I*O,C.y=m*b,C.z=I*L,u.push(C.x,C.y,C.z),d.push(0,b,0),S.x=L*.5+.5,S.y=O*.5*b+.5,f.push(S.x,S.y),g++}for(let w=0;w<i;w++){let H=y+w,F=N+w;x===!0?h.push(F,F+1,H):h.push(F+1,F,H),_+=3}c.addGroup(p,_,x===!0?1:2),p+=_}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radiusTop,e.radiusBottom,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Ts=class s extends $i{constructor(e=1,t=1,n=32,i=1,r=!1,o=0,a=Math.PI*2){super(0,e,t,n,i,r,o,a),this.type="ConeGeometry",this.parameters={radius:e,height:t,radialSegments:n,heightSegments:i,openEnded:r,thetaStart:o,thetaLength:a}}static fromJSON(e){return new s(e.radius,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Ya=class s extends ot{constructor(e=[],t=[],n=1,i=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:e,indices:t,radius:n,detail:i};let r=[],o=[];a(i),c(n),h(),this.setAttribute("position",new tt(r,3)),this.setAttribute("normal",new tt(r.slice(),3)),this.setAttribute("uv",new tt(o,2)),i===0?this.computeVertexNormals():this.normalizeNormals();function a(M){let T=new W,x=new W,y=new W;for(let S=0;S<t.length;S+=3)f(t[S+0],T),f(t[S+1],x),f(t[S+2],y),l(T,x,y,M)}function l(M,T,x,y){let S=y+1,C=[];for(let _=0;_<=S;_++){C[_]=[];let I=M.clone().lerp(x,_/S),b=T.clone().lerp(x,_/S),N=S-_;for(let w=0;w<=N;w++)w===0&&_===S?C[_][w]=I:C[_][w]=I.clone().lerp(b,w/N)}for(let _=0;_<S;_++)for(let I=0;I<2*(S-_)-1;I++){let b=Math.floor(I/2);I%2===0?(d(C[_][b+1]),d(C[_+1][b]),d(C[_][b])):(d(C[_][b+1]),d(C[_+1][b+1]),d(C[_+1][b]))}}function c(M){let T=new W;for(let x=0;x<r.length;x+=3)T.x=r[x+0],T.y=r[x+1],T.z=r[x+2],T.normalize().multiplyScalar(M),r[x+0]=T.x,r[x+1]=T.y,r[x+2]=T.z}function h(){let M=new W;for(let T=0;T<r.length;T+=3){M.x=r[T+0],M.y=r[T+1],M.z=r[T+2];let x=m(M)/2/Math.PI+.5,y=p(M)/Math.PI+.5;o.push(x,1-y)}g(),u()}function u(){for(let M=0;M<o.length;M+=6){let T=o[M+0],x=o[M+2],y=o[M+4],S=Math.max(T,x,y),C=Math.min(T,x,y);S>.9&&C<.1&&(T<.2&&(o[M+0]+=1),x<.2&&(o[M+2]+=1),y<.2&&(o[M+4]+=1))}}function d(M){r.push(M.x,M.y,M.z)}function f(M,T){let x=M*3;T.x=e[x+0],T.y=e[x+1],T.z=e[x+2]}function g(){let M=new W,T=new W,x=new W,y=new W,S=new Ue,C=new Ue,_=new Ue;for(let I=0,b=0;I<r.length;I+=9,b+=6){M.set(r[I+0],r[I+1],r[I+2]),T.set(r[I+3],r[I+4],r[I+5]),x.set(r[I+6],r[I+7],r[I+8]),S.set(o[b+0],o[b+1]),C.set(o[b+2],o[b+3]),_.set(o[b+4],o[b+5]),y.copy(M).add(T).add(x).divideScalar(3);let N=m(y);v(S,b+0,M,N),v(C,b+2,T,N),v(_,b+4,x,N)}}function v(M,T,x,y){y<0&&M.x===1&&(o[T]=M.x-1),x.x===0&&x.z===0&&(o[T]=y/2/Math.PI+.5)}function m(M){return Math.atan2(M.z,-M.x)}function p(M){return Math.atan2(-M.y,Math.sqrt(M.x*M.x+M.z*M.z))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.vertices,e.indices,e.radius,e.detail)}};var Rn=class{constructor(){this.type="Curve",this.arcLengthDivisions=200,this.needsUpdate=!1,this.cacheArcLengths=null}getPoint(){Ze("Curve: .getPoint() not implemented.")}getPointAt(e,t){let n=this.getUtoTmapping(e);return this.getPoint(n,t)}getPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return t}getSpacedPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPointAt(n/e));return t}getLength(){let e=this.getLengths();return e[e.length-1]}getLengths(e=this.arcLengthDivisions){if(this.cacheArcLengths&&this.cacheArcLengths.length===e+1&&!this.needsUpdate)return this.cacheArcLengths;this.needsUpdate=!1;let t=[],n,i=this.getPoint(0),r=0;t.push(0);for(let o=1;o<=e;o++)n=this.getPoint(o/e),r+=n.distanceTo(i),t.push(r),i=n;return this.cacheArcLengths=t,t}updateArcLengths(){this.needsUpdate=!0,this.getLengths()}getUtoTmapping(e,t=null){let n=this.getLengths(),i=0,r=n.length,o;t?o=t:o=e*n[r-1];let a=0,l=r-1,c;for(;a<=l;)if(i=Math.floor(a+(l-a)/2),c=n[i]-o,c<0)a=i+1;else if(c>0)l=i-1;else{l=i;break}if(i=l,n[i]===o)return i/(r-1);let h=n[i],d=n[i+1]-h,f=(o-h)/d;return(i+f)/(r-1)}getTangent(e,t){let i=e-1e-4,r=e+1e-4;i<0&&(i=0),r>1&&(r=1);let o=this.getPoint(i),a=this.getPoint(r),l=t||(o.isVector2?new Ue:new W);return l.copy(a).sub(o).normalize(),l}getTangentAt(e,t){let n=this.getUtoTmapping(e);return this.getTangent(n,t)}computeFrenetFrames(e,t=!1){let n=new W,i=[],r=[],o=[],a=new W,l=new it;for(let f=0;f<=e;f++){let g=f/e;i[f]=this.getTangentAt(g,new W)}r[0]=new W,o[0]=new W;let c=Number.MAX_VALUE,h=Math.abs(i[0].x),u=Math.abs(i[0].y),d=Math.abs(i[0].z);h<=c&&(c=h,n.set(1,0,0)),u<=c&&(c=u,n.set(0,1,0)),d<=c&&n.set(0,0,1),a.crossVectors(i[0],n).normalize(),r[0].crossVectors(i[0],a),o[0].crossVectors(i[0],r[0]);for(let f=1;f<=e;f++){if(r[f]=r[f-1].clone(),o[f]=o[f-1].clone(),a.crossVectors(i[f-1],i[f]),a.length()>Number.EPSILON){a.normalize();let g=Math.acos(ut(i[f-1].dot(i[f]),-1,1));r[f].applyMatrix4(l.makeRotationAxis(a,g))}o[f].crossVectors(i[f],r[f])}if(t===!0){let f=Math.acos(ut(r[0].dot(r[e]),-1,1));f/=e,i[0].dot(a.crossVectors(r[0],r[e]))>0&&(f=-f);for(let g=1;g<=e;g++)r[g].applyMatrix4(l.makeRotationAxis(i[g],f*g)),o[g].crossVectors(i[g],r[g])}return{tangents:i,normals:r,binormals:o}}clone(){return new this.constructor().copy(this)}copy(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}toJSON(){let e={metadata:{version:4.7,type:"Curve",generator:"Curve.toJSON"}};return e.arcLengthDivisions=this.arcLengthDivisions,e.type=this.type,e}fromJSON(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}},fo=class extends Rn{constructor(e=0,t=0,n=1,i=1,r=0,o=Math.PI*2,a=!1,l=0){super(),this.isEllipseCurve=!0,this.type="EllipseCurve",this.aX=e,this.aY=t,this.xRadius=n,this.yRadius=i,this.aStartAngle=r,this.aEndAngle=o,this.aClockwise=a,this.aRotation=l}getPoint(e,t=new Ue){let n=t,i=Math.PI*2,r=this.aEndAngle-this.aStartAngle,o=Math.abs(r)<Number.EPSILON;for(;r<0;)r+=i;for(;r>i;)r-=i;r<Number.EPSILON&&(o?r=0:r=i),this.aClockwise===!0&&!o&&(r===i?r=-i:r=r-i);let a=this.aStartAngle+e*r,l=this.aX+this.xRadius*Math.cos(a),c=this.aY+this.yRadius*Math.sin(a);if(this.aRotation!==0){let h=Math.cos(this.aRotation),u=Math.sin(this.aRotation),d=l-this.aX,f=c-this.aY;l=d*h-f*u+this.aX,c=d*u+f*h+this.aY}return n.set(l,c)}copy(e){return super.copy(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}toJSON(){let e=super.toJSON();return e.aX=this.aX,e.aY=this.aY,e.xRadius=this.xRadius,e.yRadius=this.yRadius,e.aStartAngle=this.aStartAngle,e.aEndAngle=this.aEndAngle,e.aClockwise=this.aClockwise,e.aRotation=this.aRotation,e}fromJSON(e){return super.fromJSON(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}},Za=class extends fo{constructor(e,t,n,i,r,o){super(e,t,n,n,i,r,o),this.isArcCurve=!0,this.type="ArcCurve"}};function Ch(){let s=0,e=0,t=0,n=0;function i(r,o,a,l){s=r,e=a,t=-3*r+3*o-2*a-l,n=2*r-2*o+a+l}return{initCatmullRom:function(r,o,a,l,c){i(o,a,c*(a-r),c*(l-o))},initNonuniformCatmullRom:function(r,o,a,l,c,h,u){let d=(o-r)/c-(a-r)/(c+h)+(a-o)/h,f=(a-o)/h-(l-o)/(h+u)+(l-a)/u;d*=h,f*=h,i(o,a,d,f)},calc:function(r){let o=r*r,a=o*r;return s+e*r+t*o+n*a}}}var gd=new W,xd=new W,Jc=new Ch,$c=new Ch,Qc=new Ch,mr=class extends Rn{constructor(e=[],t=!1,n="centripetal",i=.5){super(),this.isCatmullRomCurve3=!0,this.type="CatmullRomCurve3",this.points=e,this.closed=t,this.curveType=n,this.tension=i}getPoint(e,t=new W){let n=t,i=this.points,r=i.length,o=(r-(this.closed?0:1))*e,a=Math.floor(o),l=o-a;this.closed?a+=a>0?0:(Math.floor(Math.abs(a)/r)+1)*r:l===0&&a===r-1&&(a=r-2,l=1);let c,h;this.closed||a>0?c=i[(a-1)%r]:(xd.subVectors(i[0],i[1]).add(i[0]),c=xd);let u=i[a%r],d=i[(a+1)%r];if(this.closed||a+2<r?h=i[(a+2)%r]:(gd.subVectors(i[r-1],i[r-2]).add(i[r-1]),h=gd),this.curveType==="centripetal"||this.curveType==="chordal"){let f=this.curveType==="chordal"?.5:.25,g=Math.pow(c.distanceToSquared(u),f),v=Math.pow(u.distanceToSquared(d),f),m=Math.pow(d.distanceToSquared(h),f);v<1e-4&&(v=1),g<1e-4&&(g=v),m<1e-4&&(m=v),Jc.initNonuniformCatmullRom(c.x,u.x,d.x,h.x,g,v,m),$c.initNonuniformCatmullRom(c.y,u.y,d.y,h.y,g,v,m),Qc.initNonuniformCatmullRom(c.z,u.z,d.z,h.z,g,v,m)}else this.curveType==="catmullrom"&&(Jc.initCatmullRom(c.x,u.x,d.x,h.x,this.tension),$c.initCatmullRom(c.y,u.y,d.y,h.y,this.tension),Qc.initCatmullRom(c.z,u.z,d.z,h.z,this.tension));return n.set(Jc.calc(l),$c.calc(l),Qc.calc(l)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e.closed=this.closed,e.curveType=this.curveType,e.tension=this.tension,e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new W().fromArray(i))}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}};function _d(s,e,t,n,i){let r=(n-e)*.5,o=(i-t)*.5,a=s*s,l=s*a;return(2*t-2*n+r+o)*l+(-3*t+3*n-2*r-o)*a+r*s+t}function Dm(s,e){let t=1-s;return t*t*e}function Nm(s,e){return 2*(1-s)*s*e}function Um(s,e){return s*s*e}function $r(s,e,t,n){return Dm(s,e)+Nm(s,t)+Um(s,n)}function Fm(s,e){let t=1-s;return t*t*t*e}function Om(s,e){let t=1-s;return 3*t*t*s*e}function Bm(s,e){return 3*(1-s)*s*s*e}function zm(s,e){return s*s*s*e}function Qr(s,e,t,n,i){return Fm(s,e)+Om(s,t)+Bm(s,n)+zm(s,i)}var Ka=class extends Rn{constructor(e=new Ue,t=new Ue,n=new Ue,i=new Ue){super(),this.isCubicBezierCurve=!0,this.type="CubicBezierCurve",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new Ue){let n=t,i=this.v0,r=this.v1,o=this.v2,a=this.v3;return n.set(Qr(e,i.x,r.x,o.x,a.x),Qr(e,i.y,r.y,o.y,a.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},ja=class extends Rn{constructor(e=new W,t=new W,n=new W,i=new W){super(),this.isCubicBezierCurve3=!0,this.type="CubicBezierCurve3",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new W){let n=t,i=this.v0,r=this.v1,o=this.v2,a=this.v3;return n.set(Qr(e,i.x,r.x,o.x,a.x),Qr(e,i.y,r.y,o.y,a.y),Qr(e,i.z,r.z,o.z,a.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},Ja=class extends Rn{constructor(e=new Ue,t=new Ue){super(),this.isLineCurve=!0,this.type="LineCurve",this.v1=e,this.v2=t}getPoint(e,t=new Ue){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new Ue){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},gr=class extends Rn{constructor(e=new W,t=new W){super(),this.isLineCurve3=!0,this.type="LineCurve3",this.v1=e,this.v2=t}getPoint(e,t=new W){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new W){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},$a=class extends Rn{constructor(e=new Ue,t=new Ue,n=new Ue){super(),this.isQuadraticBezierCurve=!0,this.type="QuadraticBezierCurve",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new Ue){let n=t,i=this.v0,r=this.v1,o=this.v2;return n.set($r(e,i.x,r.x,o.x),$r(e,i.y,r.y,o.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Kn=class extends Rn{constructor(e=new W,t=new W,n=new W){super(),this.isQuadraticBezierCurve3=!0,this.type="QuadraticBezierCurve3",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new W){let n=t,i=this.v0,r=this.v1,o=this.v2;return n.set($r(e,i.x,r.x,o.x),$r(e,i.y,r.y,o.y),$r(e,i.z,r.z,o.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Qa=class extends Rn{constructor(e=[]){super(),this.isSplineCurve=!0,this.type="SplineCurve",this.points=e}getPoint(e,t=new Ue){let n=t,i=this.points,r=(i.length-1)*e,o=Math.floor(r),a=r-o,l=i[o===0?o:o-1],c=i[o],h=i[o>i.length-2?i.length-1:o+1],u=i[o>i.length-3?i.length-1:o+2];return n.set(_d(a,l.x,c.x,h.x,u.x),_d(a,l.y,c.y,h.y,u.y)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new Ue().fromArray(i))}return this}},rh=Object.freeze({__proto__:null,ArcCurve:Za,CatmullRomCurve3:mr,CubicBezierCurve:Ka,CubicBezierCurve3:ja,EllipseCurve:fo,LineCurve:Ja,LineCurve3:gr,QuadraticBezierCurve:$a,QuadraticBezierCurve3:Kn,SplineCurve:Qa}),po=class extends Rn{constructor(){super(),this.type="CurvePath",this.curves=[],this.autoClose=!1}add(e){this.curves.push(e)}closePath(){let e=this.curves[0].getPoint(0),t=this.curves[this.curves.length-1].getPoint(1);if(!e.equals(t)){let n=e.isVector2===!0?"LineCurve":"LineCurve3";this.curves.push(new rh[n](t,e))}return this}getPoint(e,t){let n=e*this.getLength(),i=this.getCurveLengths(),r=0;for(;r<i.length;){if(i[r]>=n){let o=i[r]-n,a=this.curves[r],l=a.getLength(),c=l===0?0:1-o/l;return a.getPointAt(c,t)}r++}return null}getLength(){let e=this.getCurveLengths();return e[e.length-1]}updateArcLengths(){this.needsUpdate=!0,this.cacheLengths=null,this.getCurveLengths()}getCurveLengths(){if(this.cacheLengths&&this.cacheLengths.length===this.curves.length)return this.cacheLengths;let e=[],t=0;for(let n=0,i=this.curves.length;n<i;n++)t+=this.curves[n].getLength(),e.push(t);return this.cacheLengths=e,e}getSpacedPoints(e=40){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return this.autoClose&&t.push(t[0]),t}getPoints(e=12){let t=[],n;for(let i=0,r=this.curves;i<r.length;i++){let o=r[i],a=o.isEllipseCurve?e*2:o.isLineCurve||o.isLineCurve3?1:o.isSplineCurve?e*o.points.length:e,l=o.getPoints(a);for(let c=0;c<l.length;c++){let h=l[c];n&&n.equals(h)||(t.push(h),n=h)}}return this.autoClose&&t.length>1&&!t[t.length-1].equals(t[0])&&t.push(t[0]),t}copy(e){super.copy(e),this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(i.clone())}return this.autoClose=e.autoClose,this}toJSON(){let e=super.toJSON();e.autoClose=this.autoClose,e.curves=[];for(let t=0,n=this.curves.length;t<n;t++){let i=this.curves[t];e.curves.push(i.toJSON())}return e}fromJSON(e){super.fromJSON(e),this.autoClose=e.autoClose,this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(new rh[i.type]().fromJSON(i))}return this}};var xr=class s extends Ya{constructor(e=1,t=0){let n=(1+Math.sqrt(5))/2,i=[-1,n,0,1,n,0,-1,-n,0,1,-n,0,0,-1,n,0,1,n,0,-1,-n,0,1,-n,n,0,-1,n,0,1,-n,0,-1,-n,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(i,r,e,t),this.type="IcosahedronGeometry",this.parameters={radius:e,detail:t}}static fromJSON(e){return new s(e.radius,e.detail)}};var nn=class s extends ot{constructor(e=1,t=1,n=1,i=1){super(),this.type="PlaneGeometry",this.parameters={width:e,height:t,widthSegments:n,heightSegments:i};let r=e/2,o=t/2,a=Math.floor(n),l=Math.floor(i),c=a+1,h=l+1,u=e/a,d=t/l,f=[],g=[],v=[],m=[];for(let p=0;p<h;p++){let M=p*d-o;for(let T=0;T<c;T++){let x=T*u-r;g.push(x,-M,0),v.push(0,0,1),m.push(T/a),m.push(1-p/l)}}for(let p=0;p<l;p++)for(let M=0;M<a;M++){let T=M+c*p,x=M+c*(p+1),y=M+1+c*(p+1),S=M+1+c*p;f.push(T,x,S),f.push(x,y,S)}this.setIndex(f),this.setAttribute("position",new tt(g,3)),this.setAttribute("normal",new tt(v,3)),this.setAttribute("uv",new tt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.widthSegments,e.heightSegments)}},Qi=class s extends ot{constructor(e=.5,t=1,n=32,i=1,r=0,o=Math.PI*2){super(),this.type="RingGeometry",this.parameters={innerRadius:e,outerRadius:t,thetaSegments:n,phiSegments:i,thetaStart:r,thetaLength:o},n=Math.max(3,n),i=Math.max(1,i);let a=[],l=[],c=[],h=[],u=e,d=(t-e)/i,f=new W,g=new Ue;for(let v=0;v<=i;v++){for(let m=0;m<=n;m++){let p=r+m/n*o;f.x=u*Math.cos(p),f.y=u*Math.sin(p),l.push(f.x,f.y,f.z),c.push(0,0,1),g.x=(f.x/t+1)/2,g.y=(f.y/t+1)/2,h.push(g.x,g.y)}u+=d}for(let v=0;v<i;v++){let m=v*(n+1);for(let p=0;p<n;p++){let M=p+m,T=M,x=M+n+1,y=M+n+2,S=M+1;a.push(T,x,S),a.push(x,y,S)}}this.setIndex(a),this.setAttribute("position",new tt(l,3)),this.setAttribute("normal",new tt(c,3)),this.setAttribute("uv",new tt(h,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.innerRadius,e.outerRadius,e.thetaSegments,e.phiSegments,e.thetaStart,e.thetaLength)}};var di=class s extends ot{constructor(e=1,t=32,n=16,i=0,r=Math.PI*2,o=0,a=Math.PI){super(),this.type="SphereGeometry",this.parameters={radius:e,widthSegments:t,heightSegments:n,phiStart:i,phiLength:r,thetaStart:o,thetaLength:a},t=Math.max(3,Math.floor(t)),n=Math.max(2,Math.floor(n));let l=Math.min(o+a,Math.PI),c=0,h=[],u=new W,d=new W,f=[],g=[],v=[],m=[];for(let p=0;p<=n;p++){let M=[],T=p/n,x=o+T*a,y=e*Math.cos(x),S=Math.sqrt(e*e-y*y),C=0;p===0&&o===0?C=.5/t:p===n&&l===Math.PI&&(C=-.5/t);for(let _=0;_<=t;_++){let I=_/t,b=i+I*r;u.x=-S*Math.cos(b),u.y=y,u.z=S*Math.sin(b),g.push(u.x,u.y,u.z),d.copy(u).normalize(),v.push(d.x,d.y,d.z),m.push(I+C,1-T),M.push(c++)}h.push(M)}for(let p=0;p<n;p++)for(let M=0;M<t;M++){let T=h[p][M+1],x=h[p][M],y=h[p+1][M],S=h[p+1][M+1];(p!==0||o>0)&&f.push(T,x,S),(p!==n-1||l<Math.PI)&&f.push(x,y,S)}this.setIndex(f),this.setAttribute("position",new tt(g,3)),this.setAttribute("normal",new tt(v,3)),this.setAttribute("uv",new tt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.widthSegments,e.heightSegments,e.phiStart,e.phiLength,e.thetaStart,e.thetaLength)}};var Cn=class s extends ot{constructor(e=1,t=.4,n=12,i=48,r=Math.PI*2,o=0,a=Math.PI*2){super(),this.type="TorusGeometry",this.parameters={radius:e,tube:t,radialSegments:n,tubularSegments:i,arc:r,thetaStart:o,thetaLength:a},n=Math.floor(n),i=Math.floor(i);let l=[],c=[],h=[],u=[],d=new W,f=new W,g=new W;for(let v=0;v<=n;v++){let m=o+v/n*a;for(let p=0;p<=i;p++){let M=p/i*r;f.x=(e+t*Math.cos(m))*Math.cos(M),f.y=(e+t*Math.cos(m))*Math.sin(M),f.z=t*Math.sin(m),c.push(f.x,f.y,f.z),d.x=e*Math.cos(M),d.y=e*Math.sin(M),g.subVectors(f,d).normalize(),h.push(g.x,g.y,g.z),u.push(p/i),u.push(v/n)}}for(let v=1;v<=n;v++)for(let m=1;m<=i;m++){let p=(i+1)*v+m-1,M=(i+1)*(v-1)+m-1,T=(i+1)*(v-1)+m,x=(i+1)*v+m;l.push(p,M,x),l.push(M,T,x)}this.setIndex(l),this.setAttribute("position",new tt(c,3)),this.setAttribute("normal",new tt(h,3)),this.setAttribute("uv",new tt(u,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.tube,e.radialSegments,e.tubularSegments,e.arc,e.thetaStart,e.thetaLength)}};var mo=class s extends ot{constructor(e=new Kn(new W(-1,-1,0),new W(-1,1,0),new W(1,1,0)),t=64,n=1,i=8,r=!1){super(),this.type="TubeGeometry",this.parameters={path:e,tubularSegments:t,radius:n,radialSegments:i,closed:r};let o=e.computeFrenetFrames(t,r);this.tangents=o.tangents,this.normals=o.normals,this.binormals=o.binormals;let a=new W,l=new W,c=new Ue,h=new W,u=[],d=[],f=[],g=[];v(),this.setIndex(g),this.setAttribute("position",new tt(u,3)),this.setAttribute("normal",new tt(d,3)),this.setAttribute("uv",new tt(f,2));function v(){for(let T=0;T<t;T++)m(T);m(r===!1?t:0),M(),p()}function m(T){h=e.getPointAt(T/t,h);let x=o.normals[T],y=o.binormals[T];for(let S=0;S<=i;S++){let C=S/i*Math.PI*2,_=Math.sin(C),I=-Math.cos(C);l.x=I*x.x+_*y.x,l.y=I*x.y+_*y.y,l.z=I*x.z+_*y.z,l.normalize(),d.push(l.x,l.y,l.z),a.x=h.x+n*l.x,a.y=h.y+n*l.y,a.z=h.z+n*l.z,u.push(a.x,a.y,a.z)}}function p(){for(let T=1;T<=t;T++)for(let x=1;x<=i;x++){let y=(i+1)*(T-1)+(x-1),S=(i+1)*T+(x-1),C=(i+1)*T+x,_=(i+1)*(T-1)+x;g.push(y,S,_),g.push(S,C,_)}}function M(){for(let T=0;T<=t;T++)for(let x=0;x<=i;x++)c.x=T/t,c.y=x/i,f.push(c.x,c.y)}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}toJSON(){let e=super.toJSON();return e.path=this.parameters.path.toJSON(),e}static fromJSON(e){return new s(new rh[e.path.type]().fromJSON(e.path),e.tubularSegments,e.radius,e.radialSegments,e.closed)}},go=class extends ot{constructor(e=null){if(super(),this.type="WireframeGeometry",this.parameters={geometry:e},e!==null){let t=[],n=new Set,i=new W,r=new W;if(e.index!==null){let o=e.attributes.position,a=e.index,l=e.groups;l.length===0&&(l=[{start:0,count:a.count,materialIndex:0}]);for(let c=0,h=l.length;c<h;++c){let u=l[c],d=u.start,f=u.count;for(let g=d,v=d+f;g<v;g+=3)for(let m=0;m<3;m++){let p=a.getX(g+m),M=a.getX(g+(m+1)%3);i.fromBufferAttribute(o,p),r.fromBufferAttribute(o,M),vd(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}}else{let o=e.attributes.position;for(let a=0,l=o.count/3;a<l;a++)for(let c=0;c<3;c++){let h=3*a+c,u=3*a+(c+1)%3;i.fromBufferAttribute(o,h),r.fromBufferAttribute(o,u),vd(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}this.setAttribute("position",new tt(t,3))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}};function vd(s,e,t){let n=`${s.x},${s.y},${s.z}-${e.x},${e.y},${e.z}`,i=`${e.x},${e.y},${e.z}-${s.x},${s.y},${s.z}`;return t.has(n)===!0||t.has(i)===!0?!1:(t.add(n),t.add(i),!0)}function Ls(s){let e={};for(let t in s){e[t]={};for(let n in s[t]){let i=s[t][n];if(yd(i))i.isRenderTargetTexture?(Ze("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),e[t][n]=null):e[t][n]=i.clone();else if(Array.isArray(i))if(yd(i[0])){let r=[];for(let o=0,a=i.length;o<a;o++)r[o]=i[o].clone();e[t][n]=r}else e[t][n]=i.slice();else e[t][n]=i}}return e}function hn(s){let e={};for(let t=0;t<s.length;t++){let n=Ls(s[t]);for(let i in n)e[i]=n[i]}return e}function yd(s){return s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)}function km(s){let e=[];for(let t=0;t<s.length;t++)e.push(s[t].clone());return e}function Ph(s){let e=s.getRenderTarget();return e===null?s.outputColorSpace:e.isXRRenderTarget===!0?e.texture.colorSpace:ht.workingColorSpace}var ei={clone:Ls,merge:hn},Hm=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,Vm=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,ct=class extends gn{constructor(e){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=Hm,this.fragmentShader=Vm,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,e!==void 0&&this.setValues(e)}copy(e){return super.copy(e),this.fragmentShader=e.fragmentShader,this.vertexShader=e.vertexShader,this.uniforms=Ls(e.uniforms),this.uniformsGroups=km(e.uniformsGroups),this.defines=Object.assign({},e.defines),this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.fog=e.fog,this.lights=e.lights,this.clipping=e.clipping,this.extensions=Object.assign({},e.extensions),this.glslVersion=e.glslVersion,this.defaultAttributeValues=Object.assign({},e.defaultAttributeValues),this.index0AttributeName=e.index0AttributeName,this.uniformsNeedUpdate=e.uniformsNeedUpdate,this}toJSON(e){let t=super.toJSON(e);t.glslVersion=this.glslVersion,t.uniforms={};for(let i in this.uniforms){let o=this.uniforms[i].value;o&&o.isTexture?t.uniforms[i]={type:"t",value:o.toJSON(e).uuid}:o&&o.isColor?t.uniforms[i]={type:"c",value:o.getHex()}:o&&o.isVector2?t.uniforms[i]={type:"v2",value:o.toArray()}:o&&o.isVector3?t.uniforms[i]={type:"v3",value:o.toArray()}:o&&o.isVector4?t.uniforms[i]={type:"v4",value:o.toArray()}:o&&o.isMatrix3?t.uniforms[i]={type:"m3",value:o.toArray()}:o&&o.isMatrix4?t.uniforms[i]={type:"m4",value:o.toArray()}:t.uniforms[i]={value:o}}Object.keys(this.defines).length>0&&(t.defines=this.defines),t.vertexShader=this.vertexShader,t.fragmentShader=this.fragmentShader,t.lights=this.lights,t.clipping=this.clipping;let n={};for(let i in this.extensions)this.extensions[i]===!0&&(n[i]=!0);return Object.keys(n).length>0&&(t.extensions=n),t}fromJSON(e,t){if(super.fromJSON(e,t),e.uniforms!==void 0)for(let n in e.uniforms){let i=e.uniforms[n];switch(this.uniforms[n]={},i.type){case"t":this.uniforms[n].value=t[i.value]||null;break;case"c":this.uniforms[n].value=new Ce().setHex(i.value);break;case"v2":this.uniforms[n].value=new Ue().fromArray(i.value);break;case"v3":this.uniforms[n].value=new W().fromArray(i.value);break;case"v4":this.uniforms[n].value=new gt().fromArray(i.value);break;case"m3":this.uniforms[n].value=new rt().fromArray(i.value);break;case"m4":this.uniforms[n].value=new it().fromArray(i.value);break;default:this.uniforms[n].value=i.value}}if(e.defines!==void 0&&(this.defines=e.defines),e.vertexShader!==void 0&&(this.vertexShader=e.vertexShader),e.fragmentShader!==void 0&&(this.fragmentShader=e.fragmentShader),e.glslVersion!==void 0&&(this.glslVersion=e.glslVersion),e.extensions!==void 0)for(let n in e.extensions)this.extensions[n]=e.extensions[n];return e.lights!==void 0&&(this.lights=e.lights),e.clipping!==void 0&&(this.clipping=e.clipping),this}},_r=class extends ct{constructor(e){super(e),this.isRawShaderMaterial=!0,this.type="RawShaderMaterial"}},vn=class extends gn{constructor(e){super(),this.isMeshStandardMaterial=!0,this.type="MeshStandardMaterial",this.defines={STANDARD:""},this.color=new Ce(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Ce(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Wo,this.normalScale=new Ue(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new xn,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.defines={STANDARD:""},this.color.copy(e.color),this.roughness=e.roughness,this.metalness=e.metalness,this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.roughnessMap=e.roughnessMap,this.metalnessMap=e.metalnessMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.envMapIntensity=e.envMapIntensity,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},yn=class extends vn{constructor(e){super(),this.isMeshPhysicalMaterial=!0,this.defines={STANDARD:"",PHYSICAL:""},this.type="MeshPhysicalMaterial",this.anisotropyRotation=0,this.anisotropyMap=null,this.clearcoatMap=null,this.clearcoatRoughness=0,this.clearcoatRoughnessMap=null,this.clearcoatNormalScale=new Ue(1,1),this.clearcoatNormalMap=null,this.ior=1.5,Object.defineProperty(this,"reflectivity",{get:function(){return ut(2.5*(this.ior-1)/(this.ior+1),0,1)},set:function(t){this.ior=(1+.4*t)/(1-.4*t)}}),this.iridescenceMap=null,this.iridescenceIOR=1.3,this.iridescenceThicknessRange=[100,400],this.iridescenceThicknessMap=null,this.sheenColor=new Ce(0),this.sheenColorMap=null,this.sheenRoughness=1,this.sheenRoughnessMap=null,this.transmissionMap=null,this.thickness=0,this.thicknessMap=null,this.attenuationDistance=1/0,this.attenuationColor=new Ce(1,1,1),this.specularIntensity=1,this.specularIntensityMap=null,this.specularColor=new Ce(1,1,1),this.specularColorMap=null,this._anisotropy=0,this._clearcoat=0,this._dispersion=0,this._iridescence=0,this._retroreflectivity=0,this._sheen=0,this._transmission=0,this.setValues(e)}get anisotropy(){return this._anisotropy}set anisotropy(e){this._anisotropy>0!=e>0&&this.version++,this._anisotropy=e}get clearcoat(){return this._clearcoat}set clearcoat(e){this._clearcoat>0!=e>0&&this.version++,this._clearcoat=e}get iridescence(){return this._iridescence}set iridescence(e){this._iridescence>0!=e>0&&this.version++,this._iridescence=e}get dispersion(){return this._dispersion}set dispersion(e){this._dispersion>0!=e>0&&this.version++,this._dispersion=e}get retroreflectivity(){return this._retroreflectivity}set retroreflectivity(e){this._retroreflectivity>0!=e>0&&this.version++,this._retroreflectivity=e}get sheen(){return this._sheen}set sheen(e){this._sheen>0!=e>0&&this.version++,this._sheen=e}get transmission(){return this._transmission}set transmission(e){this._transmission>0!=e>0&&this.version++,this._transmission=e}copy(e){return super.copy(e),this.defines={STANDARD:"",PHYSICAL:""},this.anisotropy=e.anisotropy,this.anisotropyRotation=e.anisotropyRotation,this.anisotropyMap=e.anisotropyMap,this.clearcoat=e.clearcoat,this.clearcoatMap=e.clearcoatMap,this.clearcoatRoughness=e.clearcoatRoughness,this.clearcoatRoughnessMap=e.clearcoatRoughnessMap,this.clearcoatNormalMap=e.clearcoatNormalMap,this.clearcoatNormalScale.copy(e.clearcoatNormalScale),this.dispersion=e.dispersion,this.ior=e.ior,this.iridescence=e.iridescence,this.iridescenceMap=e.iridescenceMap,this.iridescenceIOR=e.iridescenceIOR,this.iridescenceThicknessRange=[...e.iridescenceThicknessRange],this.iridescenceThicknessMap=e.iridescenceThicknessMap,this.retroreflectivity=e.retroreflectivity,this.sheen=e.sheen,this.sheenColor.copy(e.sheenColor),this.sheenColorMap=e.sheenColorMap,this.sheenRoughness=e.sheenRoughness,this.sheenRoughnessMap=e.sheenRoughnessMap,this.transmission=e.transmission,this.transmissionMap=e.transmissionMap,this.thickness=e.thickness,this.thicknessMap=e.thicknessMap,this.attenuationDistance=e.attenuationDistance,this.attenuationColor.copy(e.attenuationColor),this.specularIntensity=e.specularIntensity,this.specularIntensityMap=e.specularIntensityMap,this.specularColor.copy(e.specularColor),this.specularColorMap=e.specularColorMap,this}};var xo=class extends gn{constructor(e){super(),this.isMeshLambertMaterial=!0,this.type="MeshLambertMaterial",this.color=new Ce(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Ce(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Wo,this.normalScale=new Ue(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new xn,this.combine=fl,this.reflectivity=1,this.envMapIntensity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.envMapIntensity=e.envMapIntensity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},el=class extends gn{constructor(e){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=nf,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(e)}copy(e){return super.copy(e),this.depthPacking=e.depthPacking,this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this}},tl=class extends gn{constructor(e){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(e)}copy(e){return super.copy(e),this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this}};function Zi(s,e){return!s||s.constructor===e?s:typeof e.BYTES_PER_ELEMENT=="number"?new e(s):Array.prototype.slice.call(s)}function Da(s){return s!==void 0&&s.inTangents!==void 0&&s.outTangents!==void 0}function Gm(s){function e(i,r){return s[i]-s[r]}let t=s.length,n=new Array(t);for(let i=0;i!==t;++i)n[i]=i;return n.sort(e),n}function Md(s,e,t){let n=s.length,i=new s.constructor(n);for(let r=0,o=0;o!==n;++r){let a=t[r]*e;for(let l=0;l!==e;++l)i[o++]=s[a+l]}return i}function Wm(s,e,t,n){let i=1,r=s[0];for(;r!==void 0&&r[n]===void 0;)r=s[i++];if(r===void 0)return;let o=r[n];if(o!==void 0)if(Array.isArray(o))do o=r[n],o!==void 0&&(e.push(r.time),t.push(...o)),r=s[i++];while(r!==void 0);else if(o.toArray!==void 0)do o=r[n],o!==void 0&&(e.push(r.time),o.toArray(t,t.length)),r=s[i++];while(r!==void 0);else do o=r[n],o!==void 0&&(e.push(r.time),t.push(o)),r=s[i++];while(r!==void 0)}var fi=class{constructor(e,t,n,i){this.parameterPositions=e,this._cachedIndex=0,this.resultBuffer=i!==void 0?i:new t.constructor(n),this.sampleValues=t,this.valueSize=n,this.settings=null,this.DefaultSettings_={}}evaluate(e){let t=this.parameterPositions,n=this._cachedIndex,i=t[n],r=t[n-1];e:{t:{let o;n:{i:if(!(e<i)){for(let a=n+2;;){if(i===void 0){if(e<r)break i;return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}if(n===a)break;if(r=i,i=t[++n],e<i)break t}o=t.length;break n}if(!(e>=r)){let a=t[1];e<a&&(n=2,r=a);for(let l=n-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(n===l)break;if(i=r,r=t[--n-1],e>=r)break t}o=n,n=0;break n}break e}for(;n<o;){let a=n+o>>>1;e<t[a]?o=a:n=a+1}if(i=t[n],r=t[n-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===void 0)return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}this._cachedIndex=n,this.intervalChanged_(n,r,i)}return this.interpolate_(n,r,e,i)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i;for(let o=0;o!==i;++o)t[o]=n[r+o];return t}interpolate_(){throw new Error("THREE.Interpolant: Call to abstract method.")}intervalChanged_(){}},nl=class extends fi{constructor(e,t,n,i){super(e,t,n,i),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:gs,endingEnd:gs}}intervalChanged_(e,t,n){let i=this.parameterPositions,r=e-2,o=e+1,a=i[r],l=i[o];if(a===void 0)switch(this.getSettings_().endingStart){case xs:r=e,a=2*t-n;break;case eo:r=i.length-2,a=t+i[r]-i[r+1];break;default:r=e,a=n}if(l===void 0)switch(this.getSettings_().endingEnd){case xs:o=e,l=2*n-t;break;case eo:o=1,l=n+i[1]-i[0];break;default:o=e-1,l=t}let c=(n-t)*.5,h=this.valueSize;this._weightPrev=c/(t-a),this._weightNext=c/(l-n),this._offsetPrev=r*h,this._offsetNext=o*h}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=this._offsetPrev,u=this._offsetNext,d=this._weightPrev,f=this._weightNext,g=(n-t)/(i-t),v=g*g,m=v*g,p=-d*m+2*d*v-d*g,M=(1+d)*m+(-1.5-2*d)*v+(-.5+d)*g+1,T=(-1-f)*m+(1.5+f)*v+.5*g,x=f*m-f*v;for(let y=0;y!==a;++y)r[y]=p*o[h+y]+M*o[c+y]+T*o[l+y]+x*o[u+y];return r}},_o=class extends fi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=(n-t)/(i-t),u=1-h;for(let d=0;d!==a;++d)r[d]=o[c+d]*u+o[l+d]*h;return r}},il=class extends fi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e){return this.copySampleValue_(e-1)}},sl=class extends fi{interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=this.inTangents,u=this.outTangents;if(!h||!u){let g=(n-t)/(i-t),v=1-g;for(let m=0;m!==a;++m)r[m]=o[c+m]*v+o[l+m]*g;return r}let d=a*2,f=e-1;for(let g=0;g!==a;++g){let v=o[c+g],m=o[l+g],p=f*d+g*2,M=u[p],T=u[p+1],x=e*d+g*2,y=h[x],S=h[x+1],C=qm(n,t,M,y,i);r[g]=xf(C,v,T,S,m)}return r}};function xf(s,e,t,n,i){let r=1-s;return r*r*r*e+3*r*r*s*t+3*r*s*s*n+s*s*s*i}function Xm(s,e,t,n,i){let r=1-s;return 3*r*r*(t-e)+6*r*s*(n-t)+3*s*s*(i-n)}function qm(s,e,t,n,i){let r=(s-e)/(i-e);for(let o=0;o<8;o++){let a=xf(r,e,t,n,i)-s;if(Math.abs(a)<1e-10)break;let l=Xm(r,e,t,n,i);if(Math.abs(l)<1e-10)break;r=Math.max(0,Math.min(1,r-a/l))}return r}var Mn=class{constructor(e,t,n,i){if(e===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(t===void 0||t.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+e);this.name=e,this.times=Zi(t,this.TimeBufferType),this.values=Zi(n,this.ValueBufferType),this.setInterpolation(i||this.DefaultInterpolation)}static toJSON(e){let t=e.constructor,n;if(t.toJSON!==this.toJSON)n=t.toJSON(e);else{n={name:e.name,times:Zi(e.times,Array),values:Zi(e.values,Array)};let i=e.getInterpolation();i!==e.DefaultInterpolation&&(n.interpolation=i),Da(e.settings)&&(n.settings={inTangents:Zi(e.settings.inTangents,Array),outTangents:Zi(e.settings.outTangents,Array)})}return n.type=e.ValueTypeName,n}InterpolantFactoryMethodDiscrete(e){return new il(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodLinear(e){return new _o(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodSmooth(e){return new nl(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodBezier(e){let t=new sl(this.times,this.values,this.getValueSize(),e);return this.settings&&(t.inTangents=this.settings.inTangents,t.outTangents=this.settings.outTangents),t}setInterpolation(e){let t;switch(e){case vs:t=this.InterpolantFactoryMethodDiscrete;break;case ys:t=this.InterpolantFactoryMethodLinear;break;case Ia:t=this.InterpolantFactoryMethodSmooth;break;case ih:t=this.InterpolantFactoryMethodBezier;break}if(t===void 0){let n="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(e!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(n);return Ze("KeyframeTrack:",n),this}return this.createInterpolant=t,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return vs;case this.InterpolantFactoryMethodLinear:return ys;case this.InterpolantFactoryMethodSmooth:return Ia;case this.InterpolantFactoryMethodBezier:return ih}}getValueSize(){return this.values.length/this.times.length}shift(e){if(e!==0){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]+=e}return this}scale(e){if(e!==1){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]*=e;Da(this.settings)&&(bd(this.settings.inTangents,e),bd(this.settings.outTangents,e))}return this}trim(e,t){let n=this.times,i=n.length,r=0,o=i-1;for(;r!==i&&n[r]<e;)++r;for(;o!==-1&&n[o]>t;)--o;if(++o,r!==0||o!==i){r>=o&&(o=Math.max(o,1),r=o-1);let a=this.getValueSize();this.times=n.slice(r,o),this.values=this.values.slice(r*a,o*a)}return this}validate(){let e=!0,t=this.getValueSize();t-Math.floor(t)!==0&&(et("KeyframeTrack: Invalid value size in track.",this),e=!1);let n=this.times,i=this.values,r=n.length;r===0&&(et("KeyframeTrack: Track is empty.",this),e=!1);let o=null;for(let a=0;a!==r;a++){let l=n[a];if(typeof l=="number"&&isNaN(l)){et("KeyframeTrack: Time is not a valid number.",this,a,l),e=!1;break}if(o!==null&&o>l){et("KeyframeTrack: Out of order keys.",this,a,l,o),e=!1;break}o=l}if(i!==void 0&&Kp(i))for(let a=0,l=i.length;a!==l;++a){let c=i[a];if(isNaN(c)){et("KeyframeTrack: Value is not a valid number.",this,a,c),e=!1;break}}return e}optimize(){let e=this.times.slice(),t=this.values.slice(),n=this.getValueSize(),i=this.getInterpolation()===Ia,r=e.length-1,o=1;for(let a=1;a<r;++a){let l=!1,c=e[a],h=e[a+1];if(c!==h&&(a!==1||c!==e[0]))if(i)l=!0;else{let u=a*n,d=u-n,f=u+n;for(let g=0;g!==n;++g){let v=t[u+g];if(v!==t[d+g]||v!==t[f+g]){l=!0;break}}}if(l){if(a!==o){e[o]=e[a];let u=a*n,d=o*n;for(let f=0;f!==n;++f)t[d+f]=t[u+f]}++o}}if(r>0){e[o]=e[r];for(let a=r*n,l=o*n,c=0;c!==n;++c)t[l+c]=t[a+c];++o}return o!==e.length?(this.times=e.slice(0,o),this.values=t.slice(0,o*n)):(this.times=e,this.values=t),this}clone(){let e=this.times.slice(),t=this.values.slice(),n=this.constructor,i=new n(this.name,e,t);return i.createInterpolant=this.createInterpolant,Da(this.settings)&&(i.settings={inTangents:this.settings.inTangents.slice(),outTangents:this.settings.outTangents.slice()}),i}};function bd(s,e){for(let t=0,n=s.length;t!==n;t+=2)s[t]*=e}Mn.prototype.ValueTypeName="";Mn.prototype.TimeBufferType=Float32Array;Mn.prototype.ValueBufferType=Float32Array;Mn.prototype.DefaultInterpolation=ys;var Pi=class extends Mn{constructor(e,t,n){super(e,t,n)}};Pi.prototype.ValueTypeName="bool";Pi.prototype.ValueBufferType=Array;Pi.prototype.DefaultInterpolation=vs;Pi.prototype.InterpolantFactoryMethodLinear=void 0;Pi.prototype.InterpolantFactoryMethodSmooth=void 0;var vo=class extends Mn{constructor(e,t,n,i){super(e,t,n,i)}};vo.prototype.ValueTypeName="color";var Ii=class extends Mn{constructor(e,t,n,i){super(e,t,n,i)}};Ii.prototype.ValueTypeName="number";var rl=class extends fi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=(n-t)/(i-t),c=e*a;for(let h=c+a;c!==h;c+=4)Ht.slerpFlat(r,0,o,c-a,o,c,l);return r}},Li=class extends Mn{constructor(e,t,n,i){super(e,t,n,i)}InterpolantFactoryMethodLinear(e){return new rl(this.times,this.values,this.getValueSize(),e)}};Li.prototype.ValueTypeName="quaternion";Li.prototype.InterpolantFactoryMethodSmooth=void 0;var Di=class extends Mn{constructor(e,t,n){super(e,t,n)}};Di.prototype.ValueTypeName="string";Di.prototype.ValueBufferType=Array;Di.prototype.DefaultInterpolation=vs;Di.prototype.InterpolantFactoryMethodLinear=void 0;Di.prototype.InterpolantFactoryMethodSmooth=void 0;var es=class extends Mn{constructor(e,t,n,i){super(e,t,n,i)}};es.prototype.ValueTypeName="vector";var Es=class{constructor(e="",t=-1,n=[],i=tc){this.name=e,this.tracks=n,this.duration=t,this.blendMode=i,this.uuid=Zn(),this.userData={},this.duration<0&&this.resetDuration()}static parse(e){let t=[],n=e.tracks,i=1/(e.fps||1);for(let o=0,a=n.length;o!==a;++o)t.push(Zm(n[o]).scale(i));let r=new this(e.name,e.duration,t,e.blendMode);return r.uuid=e.uuid,r.userData=JSON.parse(e.userData||"{}"),r}static toJSON(e){let t=[],n=e.tracks,i={name:e.name,duration:e.duration,tracks:t,uuid:e.uuid,blendMode:e.blendMode,userData:JSON.stringify(e.userData)};for(let r=0,o=n.length;r!==o;++r)t.push(Mn.toJSON(n[r]));return i}static CreateFromMorphTargetSequence(e,t,n,i){let r=t.length,o=[];for(let a=0;a<r;a++){let l=[],c=[];l.push((a+r-1)%r,a,(a+1)%r),c.push(0,1,0);let h=Gm(l);l=Md(l,1,h),c=Md(c,1,h),!i&&l[0]===0&&(l.push(r),c.push(c[0])),o.push(new Ii(".morphTargetInfluences["+t[a].name+"]",l,c).scale(1/n))}return new this(e,-1,o)}static findByName(e,t){let n=e;if(!Array.isArray(e)){let i=e;n=i.geometry&&i.geometry.animations||i.animations}for(let i=0;i<n.length;i++)if(n[i].name===t)return n[i];return null}static CreateClipsFromMorphTargetSequences(e,t,n){let i={},r=/^([\w-]*?)([\d]+)$/;for(let a=0,l=e.length;a<l;a++){let c=e[a],h=c.name.match(r);if(h&&h.length>1){let u=h[1],d=i[u];d||(i[u]=d=[]),d.push(c)}}let o=[];for(let a in i)o.push(this.CreateFromMorphTargetSequence(a,i[a],t,n));return o}resetDuration(){let e=this.tracks,t=0;for(let n=0,i=e.length;n!==i;++n){let r=this.tracks[n];t=Math.max(t,r.times[r.times.length-1])}return this.duration=t,this}trim(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].trim(0,this.duration);return this}validate(){let e=!0;for(let t=0;t<this.tracks.length;t++)e=e&&this.tracks[t].validate();return e}optimize(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].optimize();return this}clone(){let e=[];for(let n=0;n<this.tracks.length;n++)e.push(this.tracks[n].clone());let t=new this.constructor(this.name,this.duration,e,this.blendMode);return t.userData=JSON.parse(JSON.stringify(this.userData)),t}toJSON(){return this.constructor.toJSON(this)}};function Ym(s){switch(s.toLowerCase()){case"scalar":case"double":case"float":case"number":case"integer":return Ii;case"vector":case"vector2":case"vector3":case"vector4":return es;case"color":return vo;case"quaternion":return Li;case"bool":case"boolean":return Pi;case"string":return Di}throw new Error("THREE.KeyframeTrack: Unsupported typeName: "+s)}function Zm(s){if(s.type===void 0)throw new Error("THREE.KeyframeTrack: track type undefined, can not parse");let e=Ym(s.type);if(s.times===void 0){let n=[],i=[];Wm(s.keys,n,i,"value"),s.times=n,s.values=i}let t;return e.parse!==void 0?t=e.parse(s):t=new e(s.name,s.times,s.values,s.interpolation),Da(s.settings)&&(t.settings={inTangents:Zi(s.settings.inTangents,Float32Array),outTangents:Zi(s.settings.outTangents,Float32Array)}),t}var oi={enabled:!1,files:{},add:function(s,e){this.enabled!==!1&&(Sd(s)||(this.files[s]=e))},get:function(s){if(this.enabled!==!1&&!Sd(s))return this.files[s]},remove:function(s){delete this.files[s]},clear:function(){this.files={}}};function Sd(s){try{let e=s.slice(s.indexOf(":")+1);return new URL(e).protocol==="blob:"}catch{return!1}}var ol=class{constructor(e,t,n){let i=this,r=!1,o=0,a=0,l,c=[];this.onStart=void 0,this.onLoad=e,this.onProgress=t,this.onError=n,this._abortController=null,this.itemStart=function(h){a++,r===!1&&i.onStart!==void 0&&i.onStart(h,o,a),r=!0},this.itemEnd=function(h){o++,i.onProgress!==void 0&&i.onProgress(h,o,a),o===a&&(r=!1,i.onLoad!==void 0&&i.onLoad())},this.itemError=function(h){i.onError!==void 0&&i.onError(h)},this.resolveURL=function(h){return h=h.normalize("NFC"),l?l(h):h},this.setURLModifier=function(h){return l=h,this},this.addHandler=function(h,u){return c.push(h,u),this},this.removeHandler=function(h){let u=c.indexOf(h);return u!==-1&&c.splice(u,2),this},this.getHandler=function(h){for(let u=0,d=c.length;u<d;u+=2){let f=c[u],g=c[u+1];if(f.global&&(f.lastIndex=0),f.test(h))return g}return null},this.abort=function(){return this.abortController.abort(),this._abortController=null,this}}get abortController(){return this._abortController||(this._abortController=new AbortController),this._abortController}},_f=new ol,pi=class{constructor(e){this.manager=e!==void 0?e:_f,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}load(){}loadAsync(e,t){let n=this;return new Promise(function(i,r){n.load(e,i,t,r)})}parse(){}setCrossOrigin(e){return this.crossOrigin=e,this}setWithCredentials(e){return this.withCredentials=e,this}setPath(e){return this.path=e,this}setResourcePath(e){return this.resourcePath=e,this}setRequestHeader(e){return this.requestHeader=e,this}abort(){return this}};pi.DEFAULT_MATERIAL_NAME="__DEFAULT";var Ei={},oh=class extends Error{constructor(e,t){super(e),this.response=t}},vr=class extends pi{constructor(e){super(e),this.mimeType="",this.responseType="",this._abortController=new AbortController}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=oi.get(`file:${e}`);if(r!==void 0){this.manager.itemStart(e),setTimeout(()=>{t&&t(r),this.manager.itemEnd(e)},0);return}if(Ei[e]!==void 0){Ei[e].push({onLoad:t,onProgress:n,onError:i});return}Ei[e]=[],Ei[e].push({onLoad:t,onProgress:n,onError:i});let o=new Request(e,{headers:new Headers(this.requestHeader),credentials:this.withCredentials?"include":"same-origin",signal:typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal}),a=this.mimeType,l=this.responseType;fetch(o).then(c=>{if(c.status===200||c.status===0){if(c.status===0&&Ze("FileLoader: HTTP Status 0 received."),typeof ReadableStream>"u"||c.body===void 0||c.body.getReader===void 0)return c;let h=Ei[e],u=c.body.getReader(),d=c.headers.get("X-File-Size")||c.headers.get("Content-Length"),f=d?parseInt(d):0,g=f!==0,v=0,m=new ReadableStream({start(p){M();function M(){u.read().then(({done:T,value:x})=>{if(T)p.close();else{v+=x.byteLength;let y=new ProgressEvent("progress",{lengthComputable:g,loaded:v,total:f});for(let S=0,C=h.length;S<C;S++){let _=h[S];_.onProgress&&_.onProgress(y)}p.enqueue(x),M()}},T=>{p.error(T)})}}});return new Response(m)}else throw new oh(`fetch for "${c.url}" responded with ${c.status}: ${c.statusText}`,c)}).then(c=>{switch(l){case"arraybuffer":return c.arrayBuffer();case"blob":return c.blob();case"document":return c.text().then(h=>new DOMParser().parseFromString(h,a));case"json":return c.json();default:if(a==="")return c.text();{let u=/charset="?([^;"\s]*)"?/i.exec(a),d=u&&u[1]?u[1].toLowerCase():void 0,f=new TextDecoder(d);return c.arrayBuffer().then(g=>f.decode(g))}}}).then(c=>{oi.add(`file:${e}`,c);let h=Ei[e];delete Ei[e];for(let u=0,d=h.length;u<d;u++){let f=h[u];f.onLoad&&f.onLoad(c)}}).catch(c=>{let h=Ei[e];if(h===void 0)throw this.manager.itemError(e),c;delete Ei[e];for(let u=0,d=h.length;u<d;u++){let f=h[u];f.onError&&f.onError(c)}this.manager.itemError(e)}).finally(()=>{this.manager.itemEnd(e)}),this.manager.itemStart(e)}setResponseType(e){return this.responseType=e,this}setMimeType(e){return this.mimeType=e,this}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var Js=new WeakMap,al=class extends pi{constructor(e){super(e)}load(e,t,n,i){this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,o=oi.get(`image:${e}`);if(o!==void 0){if(o.complete===!0)r.manager.itemStart(e),setTimeout(function(){t&&t(o),r.manager.itemEnd(e)},0);else{let u=Js.get(o);u===void 0&&(u=[],Js.set(o,u)),u.push({onLoad:t,onError:i})}return o}let a=sr("img");function l(){h(),t&&t(this);let u=Js.get(this)||[];for(let d=0;d<u.length;d++){let f=u[d];f.onLoad&&f.onLoad(this)}Js.delete(this),r.manager.itemEnd(e)}function c(u){h(),i&&i(u),oi.remove(`image:${e}`);let d=Js.get(this)||[];for(let f=0;f<d.length;f++){let g=d[f];g.onError&&g.onError(u)}Js.delete(this),r.manager.itemError(e),r.manager.itemEnd(e)}function h(){a.removeEventListener("load",l,!1),a.removeEventListener("error",c,!1)}return a.addEventListener("load",l,!1),a.addEventListener("error",c,!1),e.slice(0,5)!=="data:"&&this.crossOrigin!==void 0&&(a.crossOrigin=this.crossOrigin),oi.add(`image:${e}`,a),r.manager.itemStart(e),a.src=e,a}};var yo=class extends pi{constructor(e){super(e)}load(e,t,n,i){let r=new Gt,o=new al(this.manager);return o.setCrossOrigin(this.crossOrigin),o.setPath(this.path),o.load(e,function(a){r.image=a,r.needsUpdate=!0,t!==void 0&&t(r)},n,i),r}},As=class extends Tt{constructor(e,t=1){super(),this.isLight=!0,this.type="Light",this.color=new Ce(e),this.intensity=t}copy(e,t){return super.copy(e,t),this.color.copy(e.color),this.intensity=e.intensity,this}toJSON(e){let t=super.toJSON(e);return t.object.color=this.color.getHex(),t.object.intensity=this.intensity,t}},Mo=class extends As{constructor(e,t,n){super(e,n),this.isHemisphereLight=!0,this.type="HemisphereLight",this.position.copy(Tt.DEFAULT_UP),this.updateMatrix(),this.groundColor=new Ce(t)}copy(e,t){return super.copy(e,t),this.groundColor.copy(e.groundColor),this}toJSON(e){let t=super.toJSON(e);return t.object.groundColor=this.groundColor.getHex(),t}},eh=new it,wd=new W,Td=new W,yr=class{constructor(e){this.camera=e,this.intensity=1,this.bias=0,this.biasNode=null,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new Ue(512,512),this.mapType=bn,this.map=null,this.mapPass=null,this.matrix=new it,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new dr,this._frameExtents=new Ue(1,1),this._viewportCount=1,this._viewports=[new gt(0,0,1,1)]}getViewportCount(){return this._viewportCount}getCamera(){return this.camera}getFrustum(){return this._frustum}updateMatrices(e){let t=this.camera;wd.setFromMatrixPosition(e.matrixWorld),t.position.copy(wd),Td.setFromMatrixPosition(e.target.matrixWorld),t.lookAt(Td),t.updateMatrixWorld(),this._updateMatrix(t,this.matrix,this._frustum)}_updateMatrix(e,t,n,i){eh.multiplyMatrices(e.projectionMatrix,e.matrixWorldInverse),n.setFromProjectionMatrix(eh,e.coordinateSystem,e.reversedDepth);let r=this._frameExtents,o=i?i.z/r.x:1,a=i?i.w/r.y:1,l=i?i.x/r.x:0,c=i?i.y/r.y:0;e.coordinateSystem===ir||e.reversedDepth?t.set(.5*o,0,0,.5*o+l,0,.5*a,0,.5*a+c,0,0,1,0,0,0,0,1):t.set(.5*o,0,0,.5*o+l,0,.5*a,0,.5*a+c,0,0,.5,.5,0,0,0,1),t.multiply(eh)}getViewport(e){return this._viewports[e]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(e){return this.camera=e.camera.clone(),this.intensity=e.intensity,this.bias=e.bias,this.radius=e.radius,this.autoUpdate=e.autoUpdate,this.needsUpdate=e.needsUpdate,this.normalBias=e.normalBias,this.blurSamples=e.blurSamples,this.mapSize.copy(e.mapSize),this.biasNode=e.biasNode,this}clone(){return new this.constructor().copy(this)}toJSON(){let e={};return e.intensity=this.intensity,e.bias=this.bias,e.normalBias=this.normalBias,e.radius=this.radius,e.blurSamples=this.blurSamples,e.mapSize=this.mapSize.toArray(),e.camera=this.camera.toJSON(!1).object,delete e.camera.matrix,e}},Ca=new W,Pa=new Ht,ri=new W,bo=class extends Tt{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new it,this.projectionMatrix=new it,this.projectionMatrixInverse=new it,this.coordinateSystem=Yn,this._reversedDepth=!1}get reversedDepth(){return this._reversedDepth}copy(e,t){return super.copy(e,t),this.matrixWorldInverse.copy(e.matrixWorldInverse),this.projectionMatrix.copy(e.projectionMatrix),this.projectionMatrixInverse.copy(e.projectionMatrixInverse),this.coordinateSystem=e.coordinateSystem,this}getWorldDirection(e){return super.getWorldDirection(e).negate()}updateMatrixWorld(e){super.updateMatrixWorld(e),this.matrixWorld.decompose(Ca,Pa,ri),ri.x===1&&ri.y===1&&ri.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Ca,Pa,ri.set(1,1,1)).invert()}updateWorldMatrix(e,t,n=!1){super.updateWorldMatrix(e,t,n),this.matrixWorld.decompose(Ca,Pa,ri),ri.x===1&&ri.y===1&&ri.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Ca,Pa,ri.set(1,1,1)).invert()}clone(){return new this.constructor().copy(this)}},Yi=new W,Ed=new Ue,Ad=new Ue,qt=class extends bo{constructor(e=50,t=1,n=.1,i=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=e,this.zoom=1,this.near=n,this.far=i,this.focus=10,this.aspect=t,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.fov=e.fov,this.zoom=e.zoom,this.near=e.near,this.far=e.far,this.focus=e.focus,this.aspect=e.aspect,this.view=e.view===null?null:Object.assign({},e.view),this.filmGauge=e.filmGauge,this.filmOffset=e.filmOffset,this}setFocalLength(e){let t=.5*this.getFilmHeight()/e;this.fov=Ms*2*Math.atan(t),this.updateProjectionMatrix()}getFocalLength(){let e=Math.tan(jr*.5*this.fov);return .5*this.getFilmHeight()/e}getEffectiveFOV(){return Ms*2*Math.atan(Math.tan(jr*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(e,t,n){Yi.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),t.set(Yi.x,Yi.y).multiplyScalar(-e/Yi.z),Yi.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),n.set(Yi.x,Yi.y).multiplyScalar(-e/Yi.z)}getViewSize(e,t){return this.getViewBounds(e,Ed,Ad),t.subVectors(Ad,Ed)}setViewOffset(e,t,n,i,r,o){this.aspect=e/t,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=this.near,t=e*Math.tan(jr*.5*this.fov)/this.zoom,n=2*t,i=this.aspect*n,r=-.5*i,o=this.view;if(this.view!==null&&this.view.enabled){let l=o.fullWidth,c=o.fullHeight;r+=o.offsetX*i/l,t-=o.offsetY*n/c,i*=o.width/l,n*=o.height/c}let a=this.filmOffset;a!==0&&(r+=e*a/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+i,t,t-n,e,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.fov=this.fov,t.object.zoom=this.zoom,t.object.near=this.near,t.object.far=this.far,t.object.focus=this.focus,t.object.aspect=this.aspect,this.view!==null&&(t.object.view=Object.assign({},this.view)),t.object.filmGauge=this.filmGauge,t.object.filmOffset=this.filmOffset,t}},ah=class extends yr{constructor(){super(new qt(50,1,.5,500)),this.isSpotLightShadow=!0,this.focus=1,this.aspect=1}updateMatrices(e){let t=this.camera,n=Ms*2*e.angle*this.focus,i=this.mapSize.width/this.mapSize.height*this.aspect,r=e.distance||t.far;(n!==t.fov||i!==t.aspect||r!==t.far)&&(t.fov=n,t.aspect=i,t.far=r,t.updateProjectionMatrix()),super.updateMatrices(e)}copy(e){return super.copy(e),this.focus=e.focus,this.aspect=e.aspect,this}toJSON(){let e=super.toJSON();return e.focus=this.focus,e.aspect=this.aspect,e}},So=class extends As{constructor(e,t,n=0,i=Math.PI/3,r=0,o=2){super(e,t),this.isSpotLight=!0,this.type="SpotLight",this.position.copy(Tt.DEFAULT_UP),this.updateMatrix(),this.target=new Tt,this.distance=n,this.angle=i,this.penumbra=r,this.decay=o,this.map=null,this.shadow=new ah}get power(){return this.intensity*Math.PI}set power(e){this.intensity=e/Math.PI}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.angle=e.angle,this.penumbra=e.penumbra,this.decay=e.decay,this.target=e.target.clone(),this.map=e.map,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.angle=this.angle,t.object.decay=this.decay,t.object.penumbra=this.penumbra,t.object.target=this.target.uuid,this.map&&this.map.isTexture&&(t.object.map=this.map.toJSON(e).uuid),t.object.shadow=this.shadow.toJSON(),t}},lh=class extends yr{constructor(){super(new qt(90,1,.5,500)),this.isPointLightShadow=!0}},jn=class extends As{constructor(e,t,n=0,i=2){super(e,t),this.isPointLight=!0,this.type="PointLight",this.distance=n,this.decay=i,this.shadow=new lh}get power(){return this.intensity*4*Math.PI}set power(e){this.intensity=e/(4*Math.PI)}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.decay=e.decay,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.decay=this.decay,t.object.shadow=this.shadow.toJSON(),t}},mi=class extends bo{constructor(e=-1,t=1,n=1,i=-1,r=.1,o=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=e,this.right=t,this.top=n,this.bottom=i,this.near=r,this.far=o,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.left=e.left,this.right=e.right,this.top=e.top,this.bottom=e.bottom,this.near=e.near,this.far=e.far,this.zoom=e.zoom,this.view=e.view===null?null:Object.assign({},e.view),this}setViewOffset(e,t,n,i,r,o){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=(this.right-this.left)/(2*this.zoom),t=(this.top-this.bottom)/(2*this.zoom),n=(this.right+this.left)/2,i=(this.top+this.bottom)/2,r=n-e,o=n+e,a=i+t,l=i-t;if(this.view!==null&&this.view.enabled){let c=(this.right-this.left)/this.view.fullWidth/this.zoom,h=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=c*this.view.offsetX,o=r+c*this.view.width,a-=h*this.view.offsetY,l=a-h*this.view.height}this.projectionMatrix.makeOrthographic(r,o,a,l,this.near,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.zoom=this.zoom,t.object.left=this.left,t.object.right=this.right,t.object.top=this.top,t.object.bottom=this.bottom,t.object.near=this.near,t.object.far=this.far,this.view!==null&&(t.object.view=Object.assign({},this.view)),t}},ch=class extends yr{constructor(){super(new mi(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},Ni=class extends As{constructor(e,t){super(e,t),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(Tt.DEFAULT_UP),this.updateMatrix(),this.target=new Tt,this.shadow=new ch}dispose(){super.dispose(),this.shadow.dispose()}copy(e){return super.copy(e),this.target=e.target.clone(),this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.shadow=this.shadow.toJSON(),t.object.target=this.target.uuid,t}};var Ui=class{static extractUrlBase(e){let t=e.lastIndexOf("/");return t===-1?"./":e.slice(0,t+1)}static resolveURL(e,t){return typeof e!="string"||e===""?"":(/^https?:\/\//i.test(t)&&/^\//.test(e)&&(t=t.replace(/(^https?:\/\/[^\/]+).*/i,"$1")),/^(https?:)?\/\//i.test(e)||/^data:.*,.*$/i.test(e)||/^blob:.*$/i.test(e)?e:t+e)}};var th=new WeakMap,wo=class extends pi{constructor(e){super(e),this.isImageBitmapLoader=!0,typeof createImageBitmap>"u"&&Ze("ImageBitmapLoader: createImageBitmap() not supported."),typeof fetch>"u"&&Ze("ImageBitmapLoader: fetch() not supported."),this.options={premultiplyAlpha:"none"},this._abortController=new AbortController}setOptions(e){return this.options=e,this}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,o=oi.get(`image-bitmap:${e}`);if(o!==void 0){if(r.manager.itemStart(e),o.then){o.then(c=>{th.has(o)===!0?(i&&i(th.get(o)),r.manager.itemError(e),r.manager.itemEnd(e)):(t&&t(c),r.manager.itemEnd(e))});return}setTimeout(function(){t&&t(o),r.manager.itemEnd(e)},0);return}let a={};a.credentials=this.crossOrigin==="anonymous"?"same-origin":"include",a.headers=this.requestHeader,a.signal=typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal;let l=fetch(e,a).then(function(c){return c.blob()}).then(function(c){return createImageBitmap(c,Object.assign({},r.options,{colorSpaceConversion:"none"}))}).then(function(c){return oi.add(`image-bitmap:${e}`,c),t&&t(c),r.manager.itemEnd(e),c}).catch(function(c){i&&i(c),th.set(l,c),oi.remove(`image-bitmap:${e}`),r.manager.itemError(e),r.manager.itemEnd(e)});oi.add(`image-bitmap:${e}`,l),r.manager.itemStart(e)}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var $s=-90,Qs=1,ll=class extends Tt{constructor(e,t,n){super(),this.type="CubeCamera",this.renderTarget=n,this.coordinateSystem=null,this.activeMipmapLevel=0;let i=new qt($s,Qs,e,t);i.layers=this.layers,this.add(i);let r=new qt($s,Qs,e,t);r.layers=this.layers,this.add(r);let o=new qt($s,Qs,e,t);o.layers=this.layers,this.add(o);let a=new qt($s,Qs,e,t);a.layers=this.layers,this.add(a);let l=new qt($s,Qs,e,t);l.layers=this.layers,this.add(l);let c=new qt($s,Qs,e,t);c.layers=this.layers,this.add(c)}updateCoordinateSystem(){let e=this.coordinateSystem,t=this.children.concat(),[n,i,r,o,a,l]=t;for(let c of t)this.remove(c);if(e===Yn)n.up.set(0,1,0),n.lookAt(1,0,0),i.up.set(0,1,0),i.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),o.up.set(0,0,1),o.lookAt(0,-1,0),a.up.set(0,1,0),a.lookAt(0,0,1),l.up.set(0,1,0),l.lookAt(0,0,-1);else if(e===ir)n.up.set(0,-1,0),n.lookAt(-1,0,0),i.up.set(0,-1,0),i.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),o.up.set(0,0,-1),o.lookAt(0,-1,0),a.up.set(0,-1,0),a.lookAt(0,0,1),l.up.set(0,-1,0),l.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+e);for(let c of t)this.add(c),c.updateMatrixWorld()}update(e,t){this.parent===null&&this.updateMatrixWorld();let{renderTarget:n,activeMipmapLevel:i}=this;this.coordinateSystem!==e.coordinateSystem&&(this.coordinateSystem=e.coordinateSystem,this.updateCoordinateSystem());let[r,o,a,l,c,h]=this.children,u=e.getRenderTarget(),d=e.getActiveCubeFace(),f=e.getActiveMipmapLevel(),g=e.xr.enabled;e.xr.enabled=!1;let v=n.texture.generateMipmaps;n.texture.generateMipmaps=!1;let m=!1;e.isWebGLRenderer===!0?m=e.state.buffers.depth.getReversed():m=e.reversedDepthBuffer,e.setRenderTarget(n,0,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,r),e.setRenderTarget(n,1,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,o),e.setRenderTarget(n,2,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,a),e.setRenderTarget(n,3,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,l),e.setRenderTarget(n,4,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,c),n.texture.generateMipmaps=v,e.setRenderTarget(n,5,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,h),e.setRenderTarget(u,d,f),e.xr.enabled=g,n.texture.needsPMREMUpdate=!0}},cl=class extends qt{constructor(e=[]){super(),this.isArrayCamera=!0,this.isMultiViewCamera=!1,this.cameras=e}},To=class{constructor(){this._previousTime=0,this._currentTime=0,this._startTime=performance.now(),this._delta=0,this._elapsed=0,this._timescale=1,this._document=null,this._pageVisibilityHandler=null}connect(e){this._document=e,e.hidden!==void 0&&(this._pageVisibilityHandler=Km.bind(this),e.addEventListener("visibilitychange",this._pageVisibilityHandler,!1))}disconnect(){this._pageVisibilityHandler!==null&&(this._document.removeEventListener("visibilitychange",this._pageVisibilityHandler),this._pageVisibilityHandler=null),this._document=null}getDelta(){return this._delta/1e3}getElapsed(){return this._elapsed/1e3}getTimescale(){return this._timescale}setTimescale(e){return this._timescale=e,this}reset(){return this._currentTime=performance.now()-this._startTime,this}dispose(){this.disconnect()}update(e){return this._pageVisibilityHandler!==null&&this._document.hidden===!0?this._delta=0:(this._previousTime=this._currentTime,this._currentTime=(e!==void 0?e:performance.now())-this._startTime,this._delta=(this._currentTime-this._previousTime)*this._timescale,this._elapsed+=this._delta),this}};function Km(){this._document.hidden===!1&&this.reset()}var hl=class{constructor(e,t,n){this.binding=e,this.valueSize=n;let i,r,o;switch(t){case"quaternion":i=this._slerp,r=this._slerpAdditive,o=this._setAdditiveIdentityQuaternion,this.buffer=new Float64Array(n*6),this._workIndex=5;break;case"string":case"bool":i=this._select,r=this._select,o=this._setAdditiveIdentityOther,this.buffer=new Array(n*5);break;default:i=this._lerp,r=this._lerpAdditive,o=this._setAdditiveIdentityNumeric,this.buffer=new Float64Array(n*5)}this._mixBufferRegion=i,this._mixBufferRegionAdditive=r,this._setIdentity=o,this._origIndex=3,this._addIndex=4,this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,this.useCount=0,this.referenceCount=0}accumulate(e,t){let n=this.buffer,i=this.valueSize,r=e*i+i,o=this.cumulativeWeight;if(o===0){for(let a=0;a!==i;++a)n[r+a]=n[a];o=t}else{o+=t;let a=t/o;this._mixBufferRegion(n,r,0,a,i)}this.cumulativeWeight=o}accumulateAdditive(e){let t=this.buffer,n=this.valueSize,i=n*this._addIndex;this.cumulativeWeightAdditive===0&&this._setIdentity(),this._mixBufferRegionAdditive(t,i,0,e,n),this.cumulativeWeightAdditive+=e}apply(e){let t=this.valueSize,n=this.buffer,i=e*t+t,r=this.cumulativeWeight,o=this.cumulativeWeightAdditive,a=this.binding;if(this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,r<1){let l=t*this._origIndex;this._mixBufferRegion(n,i,l,1-r,t)}o>0&&this._mixBufferRegionAdditive(n,i,this._addIndex*t,1,t);for(let l=t,c=t+t;l!==c;++l)if(n[l]!==n[l+t]){a.setValue(n,i);break}}saveOriginalState(){let e=this.binding,t=this.buffer,n=this.valueSize,i=n*this._origIndex;e.getValue(t,i);for(let r=n,o=i;r!==o;++r)t[r]=t[i+r%n];this._setIdentity(),this.cumulativeWeight=0,this.cumulativeWeightAdditive=0}restoreOriginalState(){let e=this.valueSize*3;this.binding.setValue(this.buffer,e)}_setAdditiveIdentityNumeric(){let e=this._addIndex*this.valueSize,t=e+this.valueSize;for(let n=e;n<t;n++)this.buffer[n]=0}_setAdditiveIdentityQuaternion(){this._setAdditiveIdentityNumeric(),this.buffer[this._addIndex*this.valueSize+3]=1}_setAdditiveIdentityOther(){let e=this._origIndex*this.valueSize,t=this._addIndex*this.valueSize;for(let n=0;n<this.valueSize;n++)this.buffer[t+n]=this.buffer[e+n]}_select(e,t,n,i,r){if(i>=.5)for(let o=0;o!==r;++o)e[t+o]=e[n+o]}_slerp(e,t,n,i){Ht.slerpFlat(e,t,e,t,e,n,i)}_slerpAdditive(e,t,n,i,r){let o=this._workIndex*r;Ht.multiplyQuaternionsFlat(e,o,e,t,e,n),Ht.slerpFlat(e,t,e,t,e,o,i)}_lerp(e,t,n,i,r){let o=1-i;for(let a=0;a!==r;++a){let l=t+a;e[l]=e[l]*o+e[n+a]*i}}_lerpAdditive(e,t,n,i,r){for(let o=0;o!==r;++o){let a=t+o;e[a]=e[a]+e[n+o]*i}}},Ih="\\[\\]\\.:\\/",jm=new RegExp("["+Ih+"]","g"),Lh="[^"+Ih+"]",Jm="[^"+Ih.replace("\\.","")+"]",$m=/((?:WC+[\/:])*)/.source.replace("WC",Lh),Qm=/(WCOD+)?/.source.replace("WCOD",Jm),eg=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",Lh),tg=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",Lh),ng=new RegExp("^"+$m+Qm+eg+tg+"$"),ig=["material","materials","bones","map"],hh=class{constructor(e,t,n){let i=n||Dt.parseTrackName(t);this._targetGroup=e,this._bindings=e.subscribe_(t,i)}getValue(e,t){this.bind();let n=this._targetGroup.nCachedObjects_,i=this._bindings[n];i!==void 0&&i.getValue(e,t)}setValue(e,t){let n=this._bindings;for(let i=this._targetGroup.nCachedObjects_,r=n.length;i!==r;++i)n[i].setValue(e,t)}bind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].bind()}unbind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].unbind()}},Dt=class s{constructor(e,t,n){this.path=t,this.parsedPath=n||s.parseTrackName(t),this.node=s.findNode(e,this.parsedPath.nodeName),this.rootNode=e,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(e,t,n){return e&&e.isAnimationObjectGroup?new s.Composite(e,t,n):new s(e,t,n)}static sanitizeNodeName(e){return e.replace(/\s/g,"_").replace(jm,"")}static parseTrackName(e){let t=ng.exec(e);if(t===null)throw new Error("THREE.PropertyBinding: Cannot parse trackName: "+e);let n={nodeName:t[2],objectName:t[3],objectIndex:t[4],propertyName:t[5],propertyIndex:t[6]},i=n.nodeName&&n.nodeName.lastIndexOf(".");if(i!==void 0&&i!==-1){let r=n.nodeName.substring(i+1);ig.indexOf(r)!==-1&&(n.nodeName=n.nodeName.substring(0,i),n.objectName=r)}if(n.propertyName===null||n.propertyName.length===0)throw new Error("THREE.PropertyBinding: can not parse propertyName from trackName: "+e);return n}static findNode(e,t){if(t===void 0||t===""||t==="."||t===-1||t===e.name||t===e.uuid)return e;if(e.skeleton){let n=e.skeleton.getBoneByName(t);if(n!==void 0)return n}if(e.children){let n=function(r){for(let o=0;o<r.length;o++){let a=r[o];if(a.name===t||a.uuid===t)return a;let l=n(a.children);if(l)return l}return null},i=n(e.children);if(i)return i}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(e,t){e[t]=this.targetObject[this.propertyName]}_getValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)e[t++]=n[i]}_getValue_arrayElement(e,t){e[t]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(e,t){this.resolvedProperty.toArray(e,t)}_setValue_direct(e,t){this.targetObject[this.propertyName]=e[t]}_setValue_direct_setNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++]}_setValue_array_setNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(e,t){this.resolvedProperty[this.propertyIndex]=e[t]}_setValue_arrayElement_setNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(e,t){this.resolvedProperty.fromArray(e,t)}_setValue_fromArray_setNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(e,t){this.bind(),this.getValue(e,t)}_setValue_unbound(e,t){this.bind(),this.setValue(e,t)}bind(){let e=this.node,t=this.parsedPath,n=t.objectName,i=t.propertyName,r=t.propertyIndex;if(e||(e=s.findNode(this.rootNode,t.nodeName),this.node=e),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!e){Ze("PropertyBinding: No target node found for track: "+this.path+".");return}if(n){let c=t.objectIndex;switch(n){case"materials":if(!e.material){et("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.materials){et("PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}e=e.material.materials;break;case"bones":if(!e.skeleton){et("PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}e=e.skeleton.bones;for(let h=0;h<e.length;h++)if(e[h].name===c){c=h;break}break;case"map":if("map"in e){e=e.map;break}if(!e.material){et("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.map){et("PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}e=e.material.map;break;default:if(e[n]===void 0){et("PropertyBinding: Can not bind to objectName of node undefined.",this);return}e=e[n]}if(c!==void 0){if(e[c]===void 0){et("PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,e);return}e=e[c]}}let o=e[i];if(o===void 0){let c=t.nodeName;et("PropertyBinding: Trying to update property for track: "+c+"."+i+" but it wasn't found.",e);return}let a=this.Versioning.None;this.targetObject=e,e.isMaterial===!0?a=this.Versioning.NeedsUpdate:e.isObject3D===!0&&(a=this.Versioning.MatrixWorldNeedsUpdate);let l=this.BindingType.Direct;if(r!==void 0){if(i==="morphTargetInfluences"){if(!e.geometry){et("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!e.geometry.morphAttributes){et("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}e.morphTargetDictionary[r]!==void 0&&(r=e.morphTargetDictionary[r])}l=this.BindingType.ArrayElement,this.resolvedProperty=o,this.propertyIndex=r}else o.fromArray!==void 0&&o.toArray!==void 0?(l=this.BindingType.HasFromToArray,this.resolvedProperty=o):Array.isArray(o)?(l=this.BindingType.EntireArray,this.resolvedProperty=o):this.propertyName=i;this.getValue=this.GetterByBindingType[l],this.setValue=this.SetterByBindingTypeAndVersioning[l][a]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};Dt.Composite=hh;Dt.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};Dt.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};Dt.prototype.GetterByBindingType=[Dt.prototype._getValue_direct,Dt.prototype._getValue_array,Dt.prototype._getValue_arrayElement,Dt.prototype._getValue_toArray];Dt.prototype.SetterByBindingTypeAndVersioning=[[Dt.prototype._setValue_direct,Dt.prototype._setValue_direct_setNeedsUpdate,Dt.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[Dt.prototype._setValue_array,Dt.prototype._setValue_array_setNeedsUpdate,Dt.prototype._setValue_array_setMatrixWorldNeedsUpdate],[Dt.prototype._setValue_arrayElement,Dt.prototype._setValue_arrayElement_setNeedsUpdate,Dt.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[Dt.prototype._setValue_fromArray,Dt.prototype._setValue_fromArray_setNeedsUpdate,Dt.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var ul=class{constructor(e,t,n=null,i=t.blendMode){this._mixer=e,this._clip=t,this._localRoot=n,this.blendMode=i;let r=t.tracks,o=r.length,a=new Array(o),l={endingStart:gs,endingEnd:gs};for(let c=0;c!==o;++c){let h=r[c].createInterpolant(null);a[c]=h,h.settings=l}this._interpolantSettings=l,this._interpolants=a,this._propertyBindings=new Array(o),this._cacheIndex=null,this._byClipCacheIndex=null,this._timeScaleInterpolant=null,this._restoreTimeScale=null,this._weightInterpolant=null,this.loop=Qd,this._loopCount=-1,this._startTime=null,this.time=0,this.timeScale=1,this._effectiveTimeScale=1,this.weight=1,this._effectiveWeight=1,this.repetitions=1/0,this.paused=!1,this.enabled=!0,this.clampWhenFinished=!1,this.zeroSlopeAtStart=!0,this.zeroSlopeAtEnd=!0}play(){return this._mixer._activateAction(this),this}stop(){return this._mixer._deactivateAction(this),this.reset()}reset(){return this.paused=!1,this.enabled=!0,this.time=0,this._loopCount=-1,this._startTime=null,this.stopFading().stopWarping()}isRunning(){return this.enabled&&!this.paused&&this.timeScale!==0&&this._startTime===null&&this._mixer._isActiveAction(this)}isScheduled(){return this._mixer._isActiveAction(this)}startAt(e){return this._startTime=e,this}setLoop(e,t){return this.loop=e,this.repetitions=t,this}setEffectiveWeight(e){return this.weight=e,this._effectiveWeight=this.enabled?e:0,this.stopFading()}getEffectiveWeight(){return this._effectiveWeight}fadeIn(e){return this._scheduleFading(e,0,1)}fadeOut(e){return this._scheduleFading(e,1,0)}crossFadeFrom(e,t,n=!1){if(e.fadeOut(t),this.fadeIn(t),n===!0){let i=this._clip.duration,r=e._clip.duration,o=r/i,a=i/r;e._restoreTimeScale=e.timeScale,this._restoreTimeScale=this.timeScale,e.warp(1,o,t),this.warp(a,1,t)}return this}crossFadeTo(e,t,n=!1){return e.crossFadeFrom(this,t,n)}stopFading(){let e=this._weightInterpolant;return e!==null&&(this._weightInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this}setEffectiveTimeScale(e){return this.timeScale=e,this._effectiveTimeScale=this.paused?0:e,this.stopWarping()}getEffectiveTimeScale(){return this._effectiveTimeScale}setDuration(e){return this.timeScale=this._clip.duration/e,this.stopWarping()}syncWith(e){return this.time=e.time,this.timeScale=e.timeScale,this.stopWarping()}halt(e){return this.warp(this._effectiveTimeScale,0,e)}warp(e,t,n){let i=this._mixer,r=i.time,o=this.timeScale,a=this._timeScaleInterpolant;a===null&&(a=i._lendControlInterpolant(),this._timeScaleInterpolant=a);let l=a.parameterPositions,c=a.sampleValues;return l[0]=r,l[1]=r+n,c[0]=e/o,c[1]=t/o,this}stopWarping(){let e=this._timeScaleInterpolant;return e!==null&&(this._timeScaleInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this._restoreTimeScale=null,this}getMixer(){return this._mixer}getClip(){return this._clip}getRoot(){return this._localRoot||this._mixer._root}_update(e,t,n,i){if(!this.enabled){this._updateWeight(e);return}let r=this._startTime;if(r!==null){let l=(e-r)*n;l<0||n===0?t=0:(this._startTime=null,t=n*l)}t*=this._updateTimeScale(e);let o=this._updateTime(t),a=this._updateWeight(e);if(a>0){let l=this._interpolants,c=this._propertyBindings;switch(this.blendMode){case tf:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(o),c[h].accumulateAdditive(a);break;case tc:default:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(o),c[h].accumulate(i,a)}}}_updateWeight(e){let t=0;if(this.enabled){t=this.weight;let n=this._weightInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(this.stopFading(),i===0&&(this.enabled=!1))}}return this._effectiveWeight=t,t}_updateTimeScale(e){let t=0;if(!this.paused){t=this.timeScale;let n=this._timeScaleInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(t===0?this.paused=!0:(this._restoreTimeScale!==null&&(t=this._restoreTimeScale),this.timeScale=t),this.stopWarping())}}return this._effectiveTimeScale=t,t}_updateTime(e){let t=this._clip.duration,n=this.loop,i=this.time+e,r=this._loopCount,o=n===ef;if(e===0)return r===-1?i:o&&(r&1)===1?t-i:i;if(n===ec){r===-1&&(this._loopCount=0,this._setEndings(!0,!0,!1));e:{if(i>=t)i=t;else if(i<0)i=0;else{this.time=i;break e}this.clampWhenFinished?this.paused=!0:this.enabled=!1,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e<0?-1:1})}}else{if(r===-1&&(e>=0?(r=0,this._setEndings(!0,this.repetitions===0,o)):this._setEndings(this.repetitions===0,!0,o)),i>=t||i<0){let a=Math.floor(i/t);i-=t*a,r+=Math.abs(a);let l=this.repetitions-r;if(l<=0)this.clampWhenFinished?this.paused=!0:this.enabled=!1,i=e>0?t:0,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e>0?1:-1});else{if(l===1){let c=e<0;this._setEndings(c,!c,o)}else this._setEndings(!1,!1,o);this._loopCount=r,this.time=i,this._mixer.dispatchEvent({type:"loop",action:this,loopDelta:a})}}else this._loopCount=r,this.time=i;if(o&&(r&1)===1)return t-i}return i}_setEndings(e,t,n){let i=this._interpolantSettings;n?(i.endingStart=xs,i.endingEnd=xs):(e?i.endingStart=this.zeroSlopeAtStart?xs:gs:i.endingStart=eo,t?i.endingEnd=this.zeroSlopeAtEnd?xs:gs:i.endingEnd=eo)}_scheduleFading(e,t,n){let i=this._mixer,r=i.time,o=this._weightInterpolant;o===null&&(o=i._lendControlInterpolant(),this._weightInterpolant=o);let a=o.parameterPositions,l=o.sampleValues;return a[0]=r,l[0]=t,a[1]=r+e,l[1]=n,this}},sg=new Float32Array(1),Eo=class extends On{constructor(e){super(),this._root=e,this._initMemoryManager(),this._accuIndex=0,this.time=0,this.timeScale=1,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}_bindAction(e,t){let n=e._localRoot||this._root,i=e._clip.tracks,r=i.length,o=e._propertyBindings,a=e._interpolants,l=n.uuid,c=this._bindingsByRootAndName,h=c[l];h===void 0&&(h={},c[l]=h);for(let u=0;u!==r;++u){let d=i[u],f=d.name,g=h[f];if(g!==void 0)++g.referenceCount,o[u]=g;else{if(g=o[u],g!==void 0){g._cacheIndex===null&&(++g.referenceCount,this._addInactiveBinding(g,l,f));continue}let v=t&&t._propertyBindings[u].binding.parsedPath;g=new hl(Dt.create(n,f,v),d.ValueTypeName,d.getValueSize()),++g.referenceCount,this._addInactiveBinding(g,l,f),o[u]=g}a[u].resultBuffer=g.buffer}}_activateAction(e){if(!this._isActiveAction(e)){if(e._cacheIndex===null){let n=(e._localRoot||this._root).uuid,i=e._clip.uuid,r=this._actionsByClip[i];this._bindAction(e,r&&r.knownActions[0]),this._addInactiveAction(e,i,n)}let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];r.useCount++===0&&(this._lendBinding(r),r.saveOriginalState())}this._lendAction(e)}}_deactivateAction(e){if(this._isActiveAction(e)){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.useCount===0&&(r.restoreOriginalState(),this._takeBackBinding(r))}this._takeBackAction(e)}}_initMemoryManager(){this._actions=[],this._nActiveActions=0,this._actionsByClip={},this._bindings=[],this._nActiveBindings=0,this._bindingsByRootAndName={},this._controlInterpolants=[],this._nActiveControlInterpolants=0;let e=this;this.stats={actions:{get total(){return e._actions.length},get inUse(){return e._nActiveActions}},bindings:{get total(){return e._bindings.length},get inUse(){return e._nActiveBindings}},controlInterpolants:{get total(){return e._controlInterpolants.length},get inUse(){return e._nActiveControlInterpolants}}}}_isActiveAction(e){let t=e._cacheIndex;return t!==null&&t<this._nActiveActions}_addInactiveAction(e,t,n){let i=this._actions,r=this._actionsByClip,o=r[t];if(o===void 0)o={knownActions:[e],actionByRoot:{}},e._byClipCacheIndex=0,r[t]=o;else{let a=o.knownActions;e._byClipCacheIndex=a.length,a.push(e)}e._cacheIndex=i.length,i.push(e),o.actionByRoot[n]=e}_removeInactiveAction(e){let t=this._actions,n=t[t.length-1],i=e._cacheIndex;n._cacheIndex=i,t[i]=n,t.pop(),e._cacheIndex=null;let r=e._clip.uuid,o=this._actionsByClip,a=o[r],l=a.knownActions,c=l[l.length-1],h=e._byClipCacheIndex;c._byClipCacheIndex=h,l[h]=c,l.pop(),e._byClipCacheIndex=null;let u=a.actionByRoot,d=(e._localRoot||this._root).uuid;delete u[d],l.length===0&&delete o[r],this._removeInactiveBindingsForAction(e)}_removeInactiveBindingsForAction(e){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.referenceCount===0&&this._removeInactiveBinding(r)}}_lendAction(e){let t=this._actions,n=e._cacheIndex,i=this._nActiveActions++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackAction(e){let t=this._actions,n=e._cacheIndex,i=--this._nActiveActions,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_addInactiveBinding(e,t,n){let i=this._bindingsByRootAndName,r=this._bindings,o=i[t];o===void 0&&(o={},i[t]=o),o[n]=e,e._cacheIndex=r.length,r.push(e)}_removeInactiveBinding(e){let t=this._bindings,n=e.binding,i=n.rootNode.uuid,r=n.path,o=this._bindingsByRootAndName,a=o[i],l=t[t.length-1],c=e._cacheIndex;l._cacheIndex=c,t[c]=l,t.pop(),delete a[r],Object.keys(a).length===0&&delete o[i]}_lendBinding(e){let t=this._bindings,n=e._cacheIndex,i=this._nActiveBindings++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackBinding(e){let t=this._bindings,n=e._cacheIndex,i=--this._nActiveBindings,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_lendControlInterpolant(){let e=this._controlInterpolants,t=this._nActiveControlInterpolants++,n=e[t];return n===void 0&&(n=new _o(new Float32Array(2),new Float32Array(2),1,sg),n.__cacheIndex=t,e[t]=n),n}_takeBackControlInterpolant(e){let t=this._controlInterpolants,n=e.__cacheIndex,i=--this._nActiveControlInterpolants,r=t[i];e.__cacheIndex=i,t[i]=e,r.__cacheIndex=n,t[n]=r}clipAction(e,t,n){let i=t||this._root,r=i.uuid,o=typeof e=="string"?Es.findByName(i,e):e,a=o!==null?o.uuid:e,l=this._actionsByClip[a],c=null;if(n===void 0&&(o!==null?n=o.blendMode:n=tc),l!==void 0){let u=l.actionByRoot[r];if(u!==void 0&&u.blendMode===n)return u;c=l.knownActions[0],o===null&&(o=c._clip)}if(o===null)return null;let h=new ul(this,o,t,n);return this._bindAction(h,c),this._addInactiveAction(h,a,r),h}existingAction(e,t){let n=t||this._root,i=n.uuid,r=typeof e=="string"?Es.findByName(n,e):e,o=r?r.uuid:e,a=this._actionsByClip[o];return a!==void 0&&a.actionByRoot[i]||null}stopAllAction(){let e=this._actions,t=this._nActiveActions;for(let n=t-1;n>=0;--n)e[n].stop();return this}update(e){e*=this.timeScale;let t=this._actions,n=this._nActiveActions,i=this.time+=e,r=Math.sign(e),o=this._accuIndex^=1;for(let c=0;c!==n;++c)t[c]._update(i,e,r,o);let a=this._bindings,l=this._nActiveBindings;for(let c=0;c!==l;++c)a[c].apply(o);return this}setTime(e){this.time=0;for(let t=0;t<this._actions.length;t++)this._actions[t].time=0;return this.update(e)}getRoot(){return this._root}uncacheClip(e){let t=this._actions,n=e.uuid,i=this._actionsByClip,r=i[n];if(r!==void 0){let o=r.knownActions;for(let a=0,l=o.length;a!==l;++a){let c=o[a];this._deactivateAction(c);let h=c._cacheIndex,u=t[t.length-1];c._cacheIndex=null,c._byClipCacheIndex=null,u._cacheIndex=h,t[h]=u,t.pop(),this._removeInactiveBindingsForAction(c)}delete i[n]}}uncacheRoot(e){let t=e.uuid,n=this._actionsByClip;for(let o in n){let a=n[o].actionByRoot,l=a[t];l!==void 0&&(this._deactivateAction(l),this._removeInactiveAction(l))}let i=this._bindingsByRootAndName,r=i[t];if(r!==void 0)for(let o in r){let a=r[o];a.restoreOriginalState(),this._removeInactiveBinding(a)}}uncacheAction(e,t){let n=this.existingAction(e,t);n!==null&&(this._deactivateAction(n),this._removeInactiveAction(n))}};var Rd=new it,Ao=class{constructor(e,t,n=0,i=1/0){this.ray=new ci(e,t),this.near=n,this.far=i,this.camera=null,this.layers=new ar,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(e,t){this.ray.set(e,t)}setFromCamera(e,t){t.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(e.x,e.y,.5).unproject(t).sub(this.ray.origin).normalize(),this.camera=t):t.isOrthographicCamera?(this.ray.origin.set(e.x,e.y,t.projectionMatrix.elements[14]).unproject(t),this.ray.direction.set(0,0,-1).transformDirection(t.matrixWorld),this.camera=t):et("Raycaster: Unsupported camera type: "+t.type)}setFromXRController(e){return Rd.identity().extractRotation(e.matrixWorld),this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(Rd),this}intersectObject(e,t=!0,n=[]){return uh(e,this,n,t),n.sort(Cd),n}intersectObjects(e,t=!0,n=[]){for(let i=0,r=e.length;i<r;i++)uh(e[i],this,n,t);return n.sort(Cd),n}};function Cd(s,e){return s.distance-e.distance}function uh(s,e,t,n){let i=!0;if(s.layers.test(e.layers)&&s.raycast(e,t)===!1&&(i=!1),i===!0&&n===!0){let r=s.children;for(let o=0,a=r.length;o<a;o++)uh(r[o],e,t,!0)}}var Mr=class{constructor(e=1,t=0,n=0){this.radius=e,this.phi=t,this.theta=n}set(e,t,n){return this.radius=e,this.phi=t,this.theta=n,this}copy(e){return this.radius=e.radius,this.phi=e.phi,this.theta=e.theta,this}makeSafe(){return this.phi=ut(this.phi,1e-6,Math.PI-1e-6),this}setFromVector3(e){return this.setFromCartesianCoords(e.x,e.y,e.z)}setFromCartesianCoords(e,t,n){return this.radius=Math.sqrt(e*e+t*t+n*n),this.radius===0?(this.theta=0,this.phi=0):(this.theta=Math.atan2(e,n),this.phi=Math.acos(ut(t/this.radius,-1,1))),this}clone(){return new this.constructor().copy(this)}};var dh=class s{static{s.prototype.isMatrix2=!0}constructor(e,t,n,i){this.elements=[1,0,0,1],e!==void 0&&this.set(e,t,n,i)}identity(){return this.set(1,0,0,1),this}fromArray(e,t=0){for(let n=0;n<4;n++)this.elements[n]=e[n+t];return this}set(e,t,n,i){let r=this.elements;return r[0]=e,r[2]=t,r[1]=n,r[3]=i,this}};var Ro=class extends On{constructor(e,t=null){super(),this.object=e,this.domElement=t,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(e){this.domElement!==null&&this.disconnect(),this.domElement=e}disconnect(){}dispose(){}update(){}};function Dh(s,e,t,n){let i=rg(n);switch(t){case Sh:return s*e;case yl:return s*e/i.components*i.byteLength;case Ml:return s*e/i.components*i.byteLength;case rs:return s*e*2/i.components*i.byteLength;case bl:return s*e*2/i.components*i.byteLength;case wh:return s*e*3/i.components*i.byteLength;case In:return s*e*4/i.components*i.byteLength;case Sl:return s*e*4/i.components*i.byteLength;case Oo:case Bo:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case zo:case ko:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Tl:case Al:return Math.max(s,16)*Math.max(e,8)/4;case wl:case El:return Math.max(s,8)*Math.max(e,8)/2;case Rl:case Cl:case Il:case Ll:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case Pl:case Ho:case Dl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Nl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Ul:return Math.floor((s+4)/5)*Math.floor((e+3)/4)*16;case Fl:return Math.floor((s+4)/5)*Math.floor((e+4)/5)*16;case Ol:return Math.floor((s+5)/6)*Math.floor((e+4)/5)*16;case Bl:return Math.floor((s+5)/6)*Math.floor((e+5)/6)*16;case zl:return Math.floor((s+7)/8)*Math.floor((e+4)/5)*16;case kl:return Math.floor((s+7)/8)*Math.floor((e+5)/6)*16;case Hl:return Math.floor((s+7)/8)*Math.floor((e+7)/8)*16;case Vl:return Math.floor((s+9)/10)*Math.floor((e+4)/5)*16;case Gl:return Math.floor((s+9)/10)*Math.floor((e+5)/6)*16;case Wl:return Math.floor((s+9)/10)*Math.floor((e+7)/8)*16;case Xl:return Math.floor((s+9)/10)*Math.floor((e+9)/10)*16;case ql:return Math.floor((s+11)/12)*Math.floor((e+9)/10)*16;case Yl:return Math.floor((s+11)/12)*Math.floor((e+11)/12)*16;case Zl:case Kl:case jl:return Math.ceil(s/4)*Math.ceil(e/4)*16;case Jl:case $l:return Math.ceil(s/4)*Math.ceil(e/4)*8;case Vo:case Ql:return Math.ceil(s/4)*Math.ceil(e/4)*16}throw new Error(`Unable to determine texture byte length for ${t} format.`)}function rg(s){switch(s){case bn:case vh:return{byteLength:1,components:1};case Tr:case yh:case Yt:return{byteLength:2,components:1};case _l:case vl:return{byteLength:2,components:4};case Qn:case xl:case Pn:return{byteLength:4,components:1};case Mh:case bh:return{byteLength:4,components:3}}throw new Error(`THREE.TextureUtils: Unknown texture type ${s}.`)}typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"186"}}));typeof window<"u"&&(window.__THREE__?Ze("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="186");function kf(){let s=null,e=!1,t=null,n=null;function i(r,o){n=s.requestAnimationFrame(i),t(r,o)}return{start:function(){e!==!0&&t!==null&&s!==null&&(n=s.requestAnimationFrame(i),e=!0)},stop:function(){s!==null&&s.cancelAnimationFrame(n),e=!1},setAnimationLoop:function(r){t=r},setContext:function(r){s=r}}}function ag(s){let e=new WeakMap;function t(a,l){let c=a.array,h=a.usage,u=c.byteLength,d=s.createBuffer();s.bindBuffer(l,d),s.bufferData(l,c,h),a.onUploadCallback();let f;if(c instanceof Float32Array)f=s.FLOAT;else if(typeof Float16Array<"u"&&c instanceof Float16Array)f=s.HALF_FLOAT;else if(c instanceof Uint16Array)a.isFloat16BufferAttribute?f=s.HALF_FLOAT:f=s.UNSIGNED_SHORT;else if(c instanceof Int16Array)f=s.SHORT;else if(c instanceof Uint32Array)f=s.UNSIGNED_INT;else if(c instanceof Int32Array)f=s.INT;else if(c instanceof Int8Array)f=s.BYTE;else if(c instanceof Uint8Array)f=s.UNSIGNED_BYTE;else if(c instanceof Uint8ClampedArray)f=s.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+c);return{buffer:d,type:f,bytesPerElement:c.BYTES_PER_ELEMENT,version:a.version,size:u}}function n(a,l,c){let h=l.array,u=l.updateRanges;if(s.bindBuffer(c,a),u.length===0)s.bufferSubData(c,0,h);else{u.sort((f,g)=>f.start-g.start);let d=0;for(let f=1;f<u.length;f++){let g=u[d],v=u[f];v.start<=g.start+g.count+1?g.count=Math.max(g.count,v.start+v.count-g.start):(++d,u[d]=v)}u.length=d+1;for(let f=0,g=u.length;f<g;f++){let v=u[f];s.bufferSubData(c,v.start*h.BYTES_PER_ELEMENT,h,v.start,v.count)}l.clearUpdateRanges()}l.onUploadCallback()}function i(a){return a.isInterleavedBufferAttribute&&(a=a.data),e.get(a)}function r(a){a.isInterleavedBufferAttribute&&(a=a.data);let l=e.get(a);l&&(s.deleteBuffer(l.buffer),e.delete(a))}function o(a,l){if(a.isInterleavedBufferAttribute&&(a=a.data),a.isGLBufferAttribute){let h=e.get(a);(!h||h.version<a.version)&&e.set(a,{buffer:a.buffer,type:a.type,bytesPerElement:a.elementSize,version:a.version});return}let c=e.get(a);if(c===void 0)e.set(a,t(a,l));else if(c.version<a.version){if(c.size!==a.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");n(c.buffer,a,l),c.version=a.version}}return{get:i,remove:r,update:o}}var lg=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,cg=`#ifdef USE_ALPHAHASH
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
#endif`,hg=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,ug=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,dg=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,fg=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,pg=`#ifdef USE_AOMAP
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
#endif`,mg=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,gg=`#ifdef USE_BATCHING
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
#endif`,xg=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,_g=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,vg=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,yg=`float G_BlinnPhong_Implicit( ) {
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
} // validated`,Mg=`#ifdef USE_IRIDESCENCE
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
#endif`,bg=`#ifdef USE_BUMPMAP
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
#endif`,Sg=`#if NUM_CLIPPING_PLANES > 0
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
#endif`,wg=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,Tg=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,Eg=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,Ag=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#endif`,Rg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#endif`,Cg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec4 vColor;
#endif`,Pg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
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
#endif`,Ig=`#define PI 3.141592653589793
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
} // validated`,Lg=`#ifdef ENVMAP_TYPE_CUBE_UV
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
#endif`,Dg=`vec3 transformedNormal = objectNormal;
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
#endif`,Ng=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,Ug=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,Fg=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	#ifdef DECODE_VIDEO_TEXTURE_EMISSIVE
		emissiveColor = sRGBTransferEOTF( emissiveColor );
	#endif
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,Og=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,Bg="gl_FragColor = linearToOutputTexel( gl_FragColor );",zg=`vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferEOTF( in vec4 value ) {
	return vec4( mix( pow( value.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), value.rgb * 0.0773993808, vec3( lessThanEqual( value.rgb, vec3( 0.04045 ) ) ) ), value.a );
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,kg=`#ifdef USE_ENVMAP
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
#endif`,Hg=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
#endif`,Vg=`#ifdef USE_ENVMAP
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
#endif`,Gg=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,Wg=`#ifdef USE_ENVMAP
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
#endif`,Xg=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,qg=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,Yg=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,Zg=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,Kg=`#ifdef USE_GRADIENTMAP
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
}`,jg=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,Jg=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,$g=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,Qg=`uniform bool receiveShadow;
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
#include <lightprobes_pars_fragment>`,e0=`#ifdef USE_ENVMAP
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
#endif`,t0=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,n0=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,i0=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,s0=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,r0=`PhysicalMaterial material;
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
#endif`,o0=`uniform sampler2D dfgLUT;
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
}`,a0=`
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
#endif`,l0=`#if defined( RE_IndirectDiffuse )
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
#endif`,c0=`#if defined( RE_IndirectDiffuse )
	#if defined( LAMBERT ) || defined( PHONG )
		irradiance += iblIrradiance;
	#endif
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,h0=`#ifdef USE_LIGHT_PROBES_GRID
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
#endif`,u0=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,d0=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,f0=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,p0=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,m0=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = sRGBTransferEOTF( sampledDiffuseColor );
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,g0=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,x0=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
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
#endif`,_0=`#if defined( USE_POINTS_UV )
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
#endif`,v0=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,y0=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,M0=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,b0=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,S0=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,w0=`#ifdef USE_MORPHTARGETS
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
#endif`,T0=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,E0=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
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
vec3 nonPerturbedNormal = normal;`,A0=`#ifdef USE_NORMALMAP_OBJECTSPACE
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
#endif`,R0=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,C0=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,P0=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
		#ifdef FLIP_SIDED
			vBitangent = - vBitangent;
		#endif
	#endif
#endif`,I0=`#ifdef USE_NORMALMAP
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
#endif`,L0=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,D0=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,N0=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,U0=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,F0=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,O0=`vec3 packNormalToRGB( const in vec3 normal ) {
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
}`,B0=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,z0=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,k0=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,H0=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,V0=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,G0=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,W0=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,X0=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,q0=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_SUN_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
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
#endif`,Y0=`float getShadowMask() {
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
}`,Z0=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,K0=`#ifdef USE_SKINNING
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
#endif`,j0=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,J0=`#ifdef USE_SKINNING
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
#endif`,$0=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,Q0=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,ex=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,tx=`#ifndef saturate
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
vec3 CustomToneMapping( vec3 color ) { return color; }`,nx=`#ifdef USE_TRANSMISSION
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
#endif`,ix=`#ifdef USE_TRANSMISSION
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
#endif`,sx=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,rx=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,ox=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,ax=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,lx=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,cx=`uniform sampler2D t2D;
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
}`,hx=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,ux=`#ifdef ENVMAP_TYPE_CUBE
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
}`,dx=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,fx=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,px=`#include <common>
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
}`,mx=`#if DEPTH_PACKING == 3200
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
}`,gx=`#define DISTANCE
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
}`,xx=`#define DISTANCE
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
}`,_x=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,vx=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,yx=`uniform float scale;
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
}`,Mx=`uniform vec3 diffuse;
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
}`,bx=`#include <common>
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
}`,Sx=`uniform vec3 diffuse;
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
}`,wx=`#define LAMBERT
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
}`,Tx=`#define LAMBERT
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
}`,Ex=`#define MATCAP
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
}`,Ax=`#define MATCAP
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
}`,Rx=`#define NORMAL
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
}`,Cx=`#define NORMAL
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
}`,Px=`#define PHONG
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
}`,Ix=`#define PHONG
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
}`,Lx=`#define STANDARD
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
}`,Dx=`#define STANDARD
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
}`,Nx=`#define TOON
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
}`,Ux=`#define TOON
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
}`,Fx=`uniform float size;
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
}`,Ox=`uniform vec3 diffuse;
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
}`,Bx=`#include <common>
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
}`,zx=`uniform vec3 color;
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
}`,kx=`uniform float rotation;
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
}`,Hx=`uniform vec3 diffuse;
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
}`,pt={alphahash_fragment:lg,alphahash_pars_fragment:cg,alphamap_fragment:hg,alphamap_pars_fragment:ug,alphatest_fragment:dg,alphatest_pars_fragment:fg,aomap_fragment:pg,aomap_pars_fragment:mg,batching_pars_vertex:gg,batching_vertex:xg,begin_vertex:_g,beginnormal_vertex:vg,bsdfs:yg,iridescence_fragment:Mg,bumpmap_pars_fragment:bg,clipping_planes_fragment:Sg,clipping_planes_pars_fragment:wg,clipping_planes_pars_vertex:Tg,clipping_planes_vertex:Eg,color_fragment:Ag,color_pars_fragment:Rg,color_pars_vertex:Cg,color_vertex:Pg,common:Ig,cube_uv_reflection_fragment:Lg,defaultnormal_vertex:Dg,displacementmap_pars_vertex:Ng,displacementmap_vertex:Ug,emissivemap_fragment:Fg,emissivemap_pars_fragment:Og,colorspace_fragment:Bg,colorspace_pars_fragment:zg,envmap_fragment:kg,envmap_common_pars_fragment:Hg,envmap_pars_fragment:Vg,envmap_pars_vertex:Gg,envmap_physical_pars_fragment:e0,envmap_vertex:Wg,fog_vertex:Xg,fog_pars_vertex:qg,fog_fragment:Yg,fog_pars_fragment:Zg,gradientmap_pars_fragment:Kg,lightmap_pars_fragment:jg,lights_lambert_fragment:Jg,lights_lambert_pars_fragment:$g,lights_pars_begin:Qg,lights_toon_fragment:t0,lights_toon_pars_fragment:n0,lights_phong_fragment:i0,lights_phong_pars_fragment:s0,lights_physical_fragment:r0,lights_physical_pars_fragment:o0,lights_fragment_begin:a0,lights_fragment_maps:l0,lights_fragment_end:c0,lightprobes_pars_fragment:h0,logdepthbuf_fragment:u0,logdepthbuf_pars_fragment:d0,logdepthbuf_pars_vertex:f0,logdepthbuf_vertex:p0,map_fragment:m0,map_pars_fragment:g0,map_particle_fragment:x0,map_particle_pars_fragment:_0,metalnessmap_fragment:v0,metalnessmap_pars_fragment:y0,morphinstance_vertex:M0,morphcolor_vertex:b0,morphnormal_vertex:S0,morphtarget_pars_vertex:w0,morphtarget_vertex:T0,normal_fragment_begin:E0,normal_fragment_maps:A0,normal_pars_fragment:R0,normal_pars_vertex:C0,normal_vertex:P0,normalmap_pars_fragment:I0,clearcoat_normal_fragment_begin:L0,clearcoat_normal_fragment_maps:D0,clearcoat_pars_fragment:N0,iridescence_pars_fragment:U0,opaque_fragment:F0,packing:O0,premultiplied_alpha_fragment:B0,project_vertex:z0,dithering_fragment:k0,dithering_pars_fragment:H0,roughnessmap_fragment:V0,roughnessmap_pars_fragment:G0,shadowmap_pars_fragment:W0,shadowmap_pars_vertex:X0,shadowmap_vertex:q0,shadowmask_pars_fragment:Y0,skinbase_vertex:Z0,skinning_pars_vertex:K0,skinning_vertex:j0,skinnormal_vertex:J0,specularmap_fragment:$0,specularmap_pars_fragment:Q0,tonemapping_fragment:ex,tonemapping_pars_fragment:tx,transmission_fragment:nx,transmission_pars_fragment:ix,uv_pars_fragment:sx,uv_pars_vertex:rx,uv_vertex:ox,worldpos_vertex:ax,background_vert:lx,background_frag:cx,backgroundCube_vert:hx,backgroundCube_frag:ux,cube_vert:dx,cube_frag:fx,depth_vert:px,depth_frag:mx,distance_vert:gx,distance_frag:xx,equirect_vert:_x,equirect_frag:vx,linedashed_vert:yx,linedashed_frag:Mx,meshbasic_vert:bx,meshbasic_frag:Sx,meshlambert_vert:wx,meshlambert_frag:Tx,meshmatcap_vert:Ex,meshmatcap_frag:Ax,meshnormal_vert:Rx,meshnormal_frag:Cx,meshphong_vert:Px,meshphong_frag:Ix,meshphysical_vert:Lx,meshphysical_frag:Dx,meshtoon_vert:Nx,meshtoon_frag:Ux,points_vert:Fx,points_frag:Ox,shadow_vert:Bx,shadow_frag:zx,sprite_vert:kx,sprite_frag:Hx},Ve={common:{diffuse:{value:new Ce(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new rt},alphaMap:{value:null},alphaMapTransform:{value:new rt},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new rt}},envmap:{envMap:{value:null},envMapRotation:{value:new rt},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98},dfgLUT:{value:null}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new rt}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new rt}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new rt},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new rt},normalScale:{value:new Ue(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new rt},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new rt}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new rt}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new rt}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new Ce(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},sunLights:{value:[],properties:{direction:{},color:{}}},sunLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},sunShadowMatrix:{value:[]},sunShadowCascade:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null},probesSH:{value:null},probesMin:{value:new W},probesMax:{value:new W},probesResolution:{value:new W}},points:{diffuse:{value:new Ce(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new rt},alphaTest:{value:0},uvTransform:{value:new rt}},sprite:{diffuse:{value:new Ce(16777215)},opacity:{value:1},center:{value:new Ue(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new rt},alphaMap:{value:null},alphaMapTransform:{value:new rt},alphaTest:{value:0}}},xi={basic:{uniforms:hn([Ve.common,Ve.specularmap,Ve.envmap,Ve.aomap,Ve.lightmap,Ve.fog]),vertexShader:pt.meshbasic_vert,fragmentShader:pt.meshbasic_frag},lambert:{uniforms:hn([Ve.common,Ve.specularmap,Ve.envmap,Ve.aomap,Ve.lightmap,Ve.emissivemap,Ve.bumpmap,Ve.normalmap,Ve.displacementmap,Ve.fog,Ve.lights,{emissive:{value:new Ce(0)},envMapIntensity:{value:1}}]),vertexShader:pt.meshlambert_vert,fragmentShader:pt.meshlambert_frag},phong:{uniforms:hn([Ve.common,Ve.specularmap,Ve.envmap,Ve.aomap,Ve.lightmap,Ve.emissivemap,Ve.bumpmap,Ve.normalmap,Ve.displacementmap,Ve.fog,Ve.lights,{emissive:{value:new Ce(0)},specular:{value:new Ce(1118481)},shininess:{value:30},envMapIntensity:{value:1}}]),vertexShader:pt.meshphong_vert,fragmentShader:pt.meshphong_frag},standard:{uniforms:hn([Ve.common,Ve.envmap,Ve.aomap,Ve.lightmap,Ve.emissivemap,Ve.bumpmap,Ve.normalmap,Ve.displacementmap,Ve.roughnessmap,Ve.metalnessmap,Ve.fog,Ve.lights,{emissive:{value:new Ce(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:pt.meshphysical_vert,fragmentShader:pt.meshphysical_frag},toon:{uniforms:hn([Ve.common,Ve.aomap,Ve.lightmap,Ve.emissivemap,Ve.bumpmap,Ve.normalmap,Ve.displacementmap,Ve.gradientmap,Ve.fog,Ve.lights,{emissive:{value:new Ce(0)}}]),vertexShader:pt.meshtoon_vert,fragmentShader:pt.meshtoon_frag},matcap:{uniforms:hn([Ve.common,Ve.bumpmap,Ve.normalmap,Ve.displacementmap,Ve.fog,{matcap:{value:null}}]),vertexShader:pt.meshmatcap_vert,fragmentShader:pt.meshmatcap_frag},points:{uniforms:hn([Ve.points,Ve.fog]),vertexShader:pt.points_vert,fragmentShader:pt.points_frag},dashed:{uniforms:hn([Ve.common,Ve.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:pt.linedashed_vert,fragmentShader:pt.linedashed_frag},depth:{uniforms:hn([Ve.common,Ve.displacementmap]),vertexShader:pt.depth_vert,fragmentShader:pt.depth_frag},normal:{uniforms:hn([Ve.common,Ve.bumpmap,Ve.normalmap,Ve.displacementmap,{opacity:{value:1}}]),vertexShader:pt.meshnormal_vert,fragmentShader:pt.meshnormal_frag},sprite:{uniforms:hn([Ve.sprite,Ve.fog]),vertexShader:pt.sprite_vert,fragmentShader:pt.sprite_frag},background:{uniforms:{uvTransform:{value:new rt},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:pt.background_vert,fragmentShader:pt.background_frag},backgroundCube:{uniforms:{envMap:{value:null},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new rt}},vertexShader:pt.backgroundCube_vert,fragmentShader:pt.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:pt.cube_vert,fragmentShader:pt.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:pt.equirect_vert,fragmentShader:pt.equirect_frag},distance:{uniforms:hn([Ve.common,Ve.displacementmap,{referencePosition:{value:new W},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:pt.distance_vert,fragmentShader:pt.distance_frag},shadow:{uniforms:hn([Ve.lights,Ve.fog,{color:{value:new Ce(0)},opacity:{value:1}}]),vertexShader:pt.shadow_vert,fragmentShader:pt.shadow_frag}};xi.physical={uniforms:hn([xi.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new rt},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new rt},clearcoatNormalScale:{value:new Ue(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new rt},dispersion:{value:0},retroreflectivity:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new rt},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new rt},sheen:{value:0},sheenColor:{value:new Ce(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new rt},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new rt},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new rt},transmissionSamplerSize:{value:new Ue},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new rt},attenuationDistance:{value:0},attenuationColor:{value:new Ce(0)},specularColor:{value:new Ce(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new rt},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new rt},anisotropyVector:{value:new Ue},anisotropyMap:{value:null},anisotropyMapTransform:{value:new rt}}]),vertexShader:pt.meshphysical_vert,fragmentShader:pt.meshphysical_frag};var sc={r:0,b:0,g:0},Vx=new it,Hf=new rt;Hf.set(-1,0,0,0,1,0,0,0,1);function Gx(s,e,t,n,i,r){let o=new Ce(0),a=i===!0?0:1,l,c,h=null,u=0,d=null;function f(M){let T=M.isScene===!0?M.background:null;if(T&&T.isTexture){let x=M.backgroundBlurriness>0;T=e.get(T,x)}return T}function g(M){let T=!1,x=f(M);x===null?m(o,a):x&&x.isColor&&(m(x,1),T=!0);let y=s.xr.getEnvironmentBlendMode();y==="additive"?t.buffers.color.setClear(0,0,0,1,r):y==="alpha-blend"&&t.buffers.color.setClear(0,0,0,0,r),(s.autoClear||T)&&(t.buffers.depth.setTest(!0),t.buffers.depth.setMask(!0),t.buffers.color.setMask(!0),s.clear(s.autoClearColor,s.autoClearDepth,s.autoClearStencil))}function v(M,T){let x=f(T);x&&(x.isCubeTexture||x.mapping===Fo)?(c===void 0&&(c=new Ke(new ui(1,1,1),new ct({name:"BackgroundCubeMaterial",uniforms:Ls(xi.backgroundCube.uniforms),vertexShader:xi.backgroundCube.vertexShader,fragmentShader:xi.backgroundCube.fragmentShader,side:Kt,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),c.geometry.deleteAttribute("normal"),c.geometry.deleteAttribute("uv"),c.onBeforeRender=function(y,S,C){this.matrixWorld.copyPosition(C.matrixWorld)},Object.defineProperty(c.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),n.update(c)),c.material.uniforms.envMap.value=x,c.material.uniforms.backgroundBlurriness.value=T.backgroundBlurriness,c.material.uniforms.backgroundIntensity.value=T.backgroundIntensity,c.material.uniforms.backgroundRotation.value.setFromMatrix4(Vx.makeRotationFromEuler(T.backgroundRotation)).transpose(),x.isCubeTexture&&x.isRenderTargetTexture===!1&&c.material.uniforms.backgroundRotation.value.premultiply(Hf),c.material.toneMapped=ht.getTransfer(x.colorSpace)!==St,(h!==x||u!==x.version||d!==s.toneMapping)&&(c.material.needsUpdate=!0,h=x,u=x.version,d=s.toneMapping),c.layers.enableAll(),M.unshift(c,c.geometry,c.material,0,0,null)):x&&x.isTexture&&(l===void 0&&(l=new Ke(new nn(2,2),new ct({name:"BackgroundMaterial",uniforms:Ls(xi.background.uniforms),vertexShader:xi.background.vertexShader,fragmentShader:xi.background.fragmentShader,side:Bn,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),l.geometry.deleteAttribute("normal"),Object.defineProperty(l.material,"map",{get:function(){return this.uniforms.t2D.value}}),n.update(l)),l.material.uniforms.t2D.value=x,l.material.uniforms.backgroundIntensity.value=T.backgroundIntensity,l.material.toneMapped=ht.getTransfer(x.colorSpace)!==St,x.matrixAutoUpdate===!0&&x.updateMatrix(),l.material.uniforms.uvTransform.value.copy(x.matrix),(h!==x||u!==x.version||d!==s.toneMapping)&&(l.material.needsUpdate=!0,h=x,u=x.version,d=s.toneMapping),l.layers.enableAll(),M.unshift(l,l.geometry,l.material,0,0,null))}function m(M,T){M.getRGB(sc,Ph(s)),t.buffers.color.setClear(sc.r,sc.g,sc.b,T,r)}function p(){c!==void 0&&(c.geometry.dispose(),c.material.dispose(),c=void 0),l!==void 0&&(l.geometry.dispose(),l.material.dispose(),l=void 0)}return{getClearColor:function(){return o},setClearColor:function(M,T=1){o.set(M),a=T,m(o,a)},getClearAlpha:function(){return a},setClearAlpha:function(M){a=M,m(o,a)},render:g,addToRenderList:v,dispose:p}}function Wx(s,e){let t=s.getParameter(s.MAX_VERTEX_ATTRIBS),n={},i=d(null),r=i,o=!1;function a(N,w,H,F,L){let O=!1,V=u(N,F,H,w);r!==V&&(r=V,c(r.object)),O=f(N,F,H,L),O&&g(N,F,H,L),L!==null&&e.update(L,s.ELEMENT_ARRAY_BUFFER),(O||o)&&(o=!1,x(N,w,H,F),L!==null&&s.bindBuffer(s.ELEMENT_ARRAY_BUFFER,e.get(L).buffer))}function l(){return s.createVertexArray()}function c(N){return s.bindVertexArray(N)}function h(N){return s.deleteVertexArray(N)}function u(N,w,H,F){let L=F.wireframe===!0,O=n[w.id];O===void 0&&(O={},n[w.id]=O);let V=N.isInstancedMesh===!0?N.id:0,R=O[V];R===void 0&&(R={},O[V]=R);let G=R[H.id];G===void 0&&(G={},R[H.id]=G);let oe=G[L];return oe===void 0&&(oe=d(l()),G[L]=oe),oe}function d(N){let w=[],H=[],F=[];for(let L=0;L<t;L++)w[L]=0,H[L]=0,F[L]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:w,enabledAttributes:H,attributeDivisors:F,object:N,attributes:{},index:null}}function f(N,w,H,F){let L=r.attributes,O=w.attributes,V=0,R=H.getAttributes();for(let G in R)if(R[G].location>=0){let k=L[G],re=O[G];if(re===void 0&&(G==="instanceMatrix"&&N.instanceMatrix&&(re=N.instanceMatrix),G==="instanceColor"&&N.instanceColor&&(re=N.instanceColor)),k===void 0||k.attribute!==re||re&&k.data!==re.data)return!0;V++}return r.attributesNum!==V||r.index!==F}function g(N,w,H,F){let L={},O=w.attributes,V=0,R=H.getAttributes();for(let G in R)if(R[G].location>=0){let k=O[G];k===void 0&&(G==="instanceMatrix"&&N.instanceMatrix&&(k=N.instanceMatrix),G==="instanceColor"&&N.instanceColor&&(k=N.instanceColor));let re={};re.attribute=k,k&&k.data&&(re.data=k.data),L[G]=re,V++}r.attributes=L,r.attributesNum=V,r.index=F}function v(){let N=r.newAttributes;for(let w=0,H=N.length;w<H;w++)N[w]=0}function m(N){p(N,0)}function p(N,w){let H=r.newAttributes,F=r.enabledAttributes,L=r.attributeDivisors;H[N]=1,F[N]===0&&(s.enableVertexAttribArray(N),F[N]=1),L[N]!==w&&(s.vertexAttribDivisor(N,w),L[N]=w)}function M(){let N=r.newAttributes,w=r.enabledAttributes;for(let H=0,F=w.length;H<F;H++)w[H]!==N[H]&&(s.disableVertexAttribArray(H),w[H]=0)}function T(N,w,H,F,L,O,V){V===!0?s.vertexAttribIPointer(N,w,H,L,O):s.vertexAttribPointer(N,w,H,F,L,O)}function x(N,w,H,F){v();let L=F.attributes,O=H.getAttributes(),V=w.defaultAttributeValues;for(let R in O){let G=O[R];if(G.location>=0){let oe=L[R];if(oe===void 0&&(R==="instanceMatrix"&&N.instanceMatrix&&(oe=N.instanceMatrix),R==="instanceColor"&&N.instanceColor&&(oe=N.instanceColor)),oe!==void 0){let k=oe.normalized,re=oe.itemSize,Ae=e.get(oe);if(Ae===void 0)continue;let Le=Ae.buffer,be=Ae.type,Fe=Ae.bytesPerElement,le=be===s.INT||be===s.UNSIGNED_INT||oe.gpuType===xl;if(oe.isInterleavedBufferAttribute){let ge=oe.data,Re=ge.stride,de=oe.offset;if(ge.isInstancedInterleavedBuffer){for(let he=0;he<G.locationSize;he++)p(G.location+he,ge.meshPerAttribute);N.isInstancedMesh!==!0&&F._maxInstanceCount===void 0&&(F._maxInstanceCount=ge.meshPerAttribute*ge.count)}else for(let he=0;he<G.locationSize;he++)m(G.location+he);s.bindBuffer(s.ARRAY_BUFFER,Le);for(let he=0;he<G.locationSize;he++)T(G.location+he,re/G.locationSize,be,k,Re*Fe,(de+re/G.locationSize*he)*Fe,le)}else{if(oe.isInstancedBufferAttribute){for(let ge=0;ge<G.locationSize;ge++)p(G.location+ge,oe.meshPerAttribute);N.isInstancedMesh!==!0&&F._maxInstanceCount===void 0&&(F._maxInstanceCount=oe.meshPerAttribute*oe.count)}else for(let ge=0;ge<G.locationSize;ge++)m(G.location+ge);s.bindBuffer(s.ARRAY_BUFFER,Le);for(let ge=0;ge<G.locationSize;ge++)T(G.location+ge,re/G.locationSize,be,k,re*Fe,re/G.locationSize*ge*Fe,le)}}else if(V!==void 0){let k=V[R];if(k!==void 0)switch(k.length){case 2:s.vertexAttrib2fv(G.location,k);break;case 3:s.vertexAttrib3fv(G.location,k);break;case 4:s.vertexAttrib4fv(G.location,k);break;default:s.vertexAttrib1fv(G.location,k)}}}}M()}function y(){I();for(let N in n){let w=n[N];for(let H in w){let F=w[H];for(let L in F){let O=F[L];for(let V in O)h(O[V].object),delete O[V];delete F[L]}}delete n[N]}}function S(N){if(n[N.id]===void 0)return;let w=n[N.id];for(let H in w){let F=w[H];for(let L in F){let O=F[L];for(let V in O)h(O[V].object),delete O[V];delete F[L]}}delete n[N.id]}function C(N){for(let w in n){let H=n[w];for(let F in H){let L=H[F];if(L[N.id]===void 0)continue;let O=L[N.id];for(let V in O)h(O[V].object),delete O[V];delete L[N.id]}}}function _(N){for(let w in n){let H=n[w],F=N.isInstancedMesh===!0?N.id:0,L=H[F];if(L!==void 0){for(let O in L){let V=L[O];for(let R in V)h(V[R].object),delete V[R];delete L[O]}delete H[F],Object.keys(H).length===0&&delete n[w]}}}function I(){b(),o=!0,r!==i&&(r=i,c(r.object))}function b(){i.geometry=null,i.program=null,i.wireframe=!1}return{setup:a,reset:I,resetDefaultState:b,dispose:y,releaseStatesOfGeometry:S,releaseStatesOfObject:_,releaseStatesOfProgram:C,initAttributes:v,enableAttribute:m,disableUnusedAttributes:M}}function Xx(s,e,t){let n;function i(l){n=l}function r(l,c){s.drawArrays(n,l,c),t.update(c,n,1)}function o(l,c,h){h!==0&&(s.drawArraysInstanced(n,l,c,h),t.update(c,n,h))}function a(l,c,h){if(h===0)return;e.get("WEBGL_multi_draw").multiDrawArraysWEBGL(n,l,0,c,0,h);let d=0;for(let f=0;f<h;f++)d+=c[f];t.update(d,n,1)}this.setMode=i,this.render=r,this.renderInstances=o,this.renderMultiDraw=a}function qx(s,e,t,n){let i;function r(){if(i!==void 0)return i;if(e.has("EXT_texture_filter_anisotropic")===!0){let C=e.get("EXT_texture_filter_anisotropic");i=s.getParameter(C.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else i=0;return i}function o(C){return!(C!==In&&n.convert(C)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_FORMAT))}function a(C){let _=C===Yt&&(e.has("EXT_color_buffer_half_float")||e.has("EXT_color_buffer_float"));return!(C!==bn&&C!==Pn&&!_&&n.convert(C)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_TYPE))}function l(C){if(C==="highp"){if(s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.HIGH_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.HIGH_FLOAT).precision>0)return"highp";C="mediump"}return C==="mediump"&&s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.MEDIUM_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let c=t.precision!==void 0?t.precision:"highp",h=l(c);h!==c&&(Ze("WebGLRenderer:",c,"not supported, using",h,"instead."),c=h);let u=t.logarithmicDepthBuffer===!0,d=t.reversedDepthBuffer===!0&&e.has("EXT_clip_control");t.reversedDepthBuffer===!0&&d===!1&&Ze("WebGLRenderer: Unable to use reversed depth buffer due to missing EXT_clip_control extension. Fallback to default depth buffer.");let f=s.getParameter(s.MAX_TEXTURE_IMAGE_UNITS),g=s.getParameter(s.MAX_VERTEX_TEXTURE_IMAGE_UNITS),v=s.getParameter(s.MAX_TEXTURE_SIZE),m=s.getParameter(s.MAX_CUBE_MAP_TEXTURE_SIZE),p=s.getParameter(s.MAX_VERTEX_ATTRIBS),M=s.getParameter(s.MAX_VERTEX_UNIFORM_VECTORS),T=s.getParameter(s.MAX_VARYING_VECTORS),x=s.getParameter(s.MAX_FRAGMENT_UNIFORM_VECTORS),y=s.getParameter(s.MAX_SAMPLES),S=s.getParameter(s.SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:l,textureFormatReadable:o,textureTypeReadable:a,precision:c,logarithmicDepthBuffer:u,reversedDepthBuffer:d,maxTextures:f,maxVertexTextures:g,maxTextureSize:v,maxCubemapSize:m,maxAttributes:p,maxVertexUniforms:M,maxVaryings:T,maxFragmentUniforms:x,maxSamples:y,samples:S}}function Yx(s){let e=this,t=null,n=0,i=!1,r=!1,o=new An,a=new rt,l={value:null,needsUpdate:!1};this.uniform=l,this.numPlanes=0,this.numIntersection=0,this.init=function(u,d){let f=u.length!==0||d||n!==0||i;return i=d,n=u.length,f},this.beginShadows=function(){r=!0,h(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(u,d){t=h(u,d,0)},this.setState=function(u,d,f){let g=u.clippingPlanes,v=u.clipIntersection,m=u.clipShadows,p=s.get(u);if(!i||g===null||g.length===0||r&&!m)r?h(null):c();else{let M=r?0:n,T=M*4,x=p.clippingState||null;l.value=x,x=h(g,d,T,f);for(let y=0;y!==T;++y)x[y]=t[y];p.clippingState=x,this.numIntersection=v?this.numPlanes:0,this.numPlanes+=M}};function c(){l.value!==t&&(l.value=t,l.needsUpdate=n>0),e.numPlanes=n,e.numIntersection=0}function h(u,d,f,g){let v=u!==null?u.length:0,m=null;if(v!==0){if(m=l.value,g!==!0||m===null){let p=f+v*4,M=d.matrixWorldInverse;a.getNormalMatrix(M),(m===null||m.length<p)&&(m=new Float32Array(p));for(let T=0,x=f;T!==v;++T,x+=4)o.copy(u[T]).applyMatrix4(M,a),o.normal.toArray(m,x),m[x+3]=o.constant}l.value=m,l.needsUpdate=!0}return e.numPlanes=v,e.numIntersection=0,m}}var Cr=4,Zx=6,Kx=20,jx=256,Xo=new mi,vf=new Ce,Nh=null,Uh=0,Fh=0,Oh=!1,Jx=new W,Ds=new W,Ir=class{constructor(e){this._renderer=e,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._sizeLods=[],this._lodMeshes=[],this._backgroundBox=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._blurMaterial=null,this._ggxMaterial=null}fromScene(e,t=0,n=.1,i=100,r={}){let{size:o=256,position:a=Jx}=r;Nh=this._renderer.getRenderTarget(),Uh=this._renderer.getActiveCubeFace(),Fh=this._renderer.getActiveMipmapLevel(),Oh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(o);let l=this._allocateTargets();return l.depthBuffer=!0,this._sceneToCubeUV(e,n,i,l,a),t>0&&this._blur(l,0,0,t),this._applyPMREM(l),this._cleanup(l),l}fromEquirectangular(e,t=null){return this._fromTexture(e,t)}fromCubemap(e,t=null){return this._fromTexture(e,t)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=bf(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=Mf(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose(),this._backgroundBox!==null&&(this._backgroundBox.geometry.dispose(),this._backgroundBox.material.dispose())}_setSize(e){this._lodMax=Math.floor(Math.log2(e)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._ggxMaterial!==null&&this._ggxMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let e=0;e<this._lodMeshes.length;e++)this._lodMeshes[e].geometry.dispose()}_cleanup(e){this._renderer.setRenderTarget(Nh,Uh,Fh),this._renderer.xr.enabled=Oh,e.scissorTest=!1,Rr(e,0,0,e.width,e.height)}_fromTexture(e,t){e.mapping===is||e.mapping===Ps?this._setSize(e.image.length===0?16:e.image[0].width||e.image[0].image.width):this._setSize(e.image.width/4),Nh=this._renderer.getRenderTarget(),Uh=this._renderer.getActiveCubeFace(),Fh=this._renderer.getActiveMipmapLevel(),Oh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let n=t||this._allocateTargets();return this._textureToCubeUV(e,n),this._applyPMREM(n),this._cleanup(n),n}_allocateTargets(){let e=3*Math.max(this._cubeSize,112),t=4*this._cubeSize,n={magFilter:Ot,minFilter:Ot,generateMipmaps:!1,type:Yt,format:In,colorSpace:mn,depthBuffer:!1},i=yf(e,t,n);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==e||this._pingPongRenderTarget.height!==t){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=yf(e,t,n);let{_lodMax:r}=this;({lodMeshes:this._lodMeshes,sizeLods:this._sizeLods}=$x(r)),this._blurMaterial=e_(r,e,t),this._ggxMaterial=Qx(r,e,t)}return i}_compileMaterial(e){let t=new Ke(new ot,e);this._renderer.compile(t,Xo)}_sceneToCubeUV(e,t,n,i,r){let l=new qt(90,1,t,n),c=[1,-1,1,1,1,1],h=[1,1,1,-1,-1,-1],u=this._renderer,d=u.autoClear,f=u.toneMapping;u.getClearColor(vf),u.toneMapping=Jn,u.autoClear=!1,u.state.buffers.depth.getReversed()&&(u.setRenderTarget(i),u.clearDepth(),u.setRenderTarget(null)),this._backgroundBox===null&&(this._backgroundBox=new Ke(new ui,new wt({name:"PMREM.Background",side:Kt,depthWrite:!1,depthTest:!1})));let v=this._backgroundBox,m=v.material,p=!1,M=e.background;M?M.isColor&&(m.color.copy(M),e.background=null,p=!0):(m.color.copy(vf),p=!0);for(let T=0;T<6;T++){let x=T%3;x===0?(l.up.set(0,c[T],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x+h[T],r.y,r.z)):x===1?(l.up.set(0,0,c[T]),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y+h[T],r.z)):(l.up.set(0,c[T],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y,r.z+h[T]));let y=this._cubeSize;Rr(i,x*y,T>2?y:0,y,y),u.setRenderTarget(i),p&&u.render(v,l),u.render(e,l)}u.toneMapping=f,u.autoClear=d,e.background=M}_textureToCubeUV(e,t){let n=this._renderer,i=e.mapping===is||e.mapping===Ps;i?(this._cubemapMaterial===null&&(this._cubemapMaterial=bf()),this._cubemapMaterial.uniforms.flipEnvMap.value=e.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=Mf());let r=i?this._cubemapMaterial:this._equirectMaterial,o=this._lodMeshes[0];o.material=r;let a=r.uniforms;a.envMap.value=e;let l=this._cubeSize;Rr(t,0,0,3*l,2*l),n.setRenderTarget(t),n.render(o,Xo)}_applyPMREM(e){let t=this._renderer,n=t.autoClear;t.autoClear=!1;let i=this._lodMeshes.length;for(let r=1;r<i;r++)this._applyGGXFilter(e,r-1,r);t.autoClear=n}_applyGGXFilter(e,t,n){let i=this._renderer,r=this._pingPongRenderTarget,o=this._ggxMaterial,a=this._lodMeshes[n];a.material=o;let l=o.uniforms,c=n/(this._lodMeshes.length-1),h=t/(this._lodMeshes.length-1),u=Math.sqrt(c*c-h*h),d=c*1.25,f=u*d,{_lodMax:g}=this,v=this._sizeLods[n],m=3*v*(n>g-Cr?n-g+Cr:0),p=4*(this._cubeSize-v);l.envMap.value=e.texture,l.roughness.value=f,l.mipInt.value=g-t,Rr(r,m,p,3*v,2*v),i.setRenderTarget(r),i.render(a,Xo),l.envMap.value=r.texture,l.roughness.value=0,l.mipInt.value=g-n,Rr(e,m,p,3*v,2*v),i.setRenderTarget(e),i.render(a,Xo)}_blur(e,t,n,i){let r=this._pingPongRenderTarget,o=Math.min(i,Math.PI)/Math.SQRT2;this._blurPass(e,r,t,n,o),this._blurPass(r,e,n,n,o)}_blurPass(e,t,n,i,r){let o=this._renderer,a=this._blurMaterial,l=this._lodMeshes[i];l.material=a;let c=a.uniforms;c.envMap.value=e.texture,c.sigma.value=r,c.mipInt.value=this._lodMax-n;let h=this._sizeLods[i],u=3*h*(i>this._lodMax-Cr?i-this._lodMax+Cr:0),d=4*(this._cubeSize-h);Rr(t,u,d,3*h,2*h),o.setRenderTarget(t),o.render(l,Xo)}};function $x(s){let e=[],t=[],n=s,i=s-Cr+1+Zx;for(let r=0;r<i;r++){let o=Math.pow(2,n);e.push(o);let a=1/(o-2),l=-a,c=1+a,h=[l,l,c,l,c,c,l,l,c,c,l,c],u=6,d=6,f=3,g=new Float32Array(f*d*u),v=new Float32Array(f*d*u);for(let p=0;p<u;p++){let M=p%3*2/3-1,T=p>2?0:-1,x=[M,T,0,M+2/3,T,0,M+2/3,T+1,0,M,T,0,M+2/3,T+1,0,M,T+1,0];g.set(x,f*d*p);for(let y=0;y<d;y++){let S=h[y*2]*2-1,C=h[y*2+1]*2-1;p===0?Ds.set(1,C,S):p===1?Ds.set(-S,1,-C):p===2?Ds.set(-S,C,1):p===3?Ds.set(-1,C,-S):p===4?Ds.set(-S,-1,C):Ds.set(S,C,-1),Ds.toArray(v,(p*d+y)*f)}}let m=new ot;m.setAttribute("position",new dt(g,f)),m.setAttribute("outputDirection",new dt(v,f)),t.push(new Ke(m,null)),n>Cr&&n--}return{lodMeshes:t,sizeLods:e}}function yf(s,e,t){let n=new Bt(s,e,t);return n.texture.mapping=Fo,n.texture.name="PMREM.cubeUv",n.scissorTest=!0,n}function Rr(s,e,t,n,i){s.viewport.set(e,t,n,i),s.scissor.set(e,t,n,i)}function Qx(s,e,t){return new ct({name:"PMREMGGXConvolution",defines:{GGX_SAMPLES:jx,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},roughness:{value:0},mipInt:{value:0}},vertexShader:lc(),fragmentShader:`

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
		`,blending:zn,depthTest:!1,depthWrite:!1})}function e_(s,e,t){return new ct({name:"SphericalGaussianBlur",defines:{SAMPLES:Kx,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},sigma:{value:0},mipInt:{value:0}},vertexShader:lc(),fragmentShader:`

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
		`,blending:zn,depthTest:!1,depthWrite:!1})}function Mf(){return new ct({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:lc(),fragmentShader:`

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
		`,blending:zn,depthTest:!1,depthWrite:!1})}function bf(){return new ct({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:lc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:zn,depthTest:!1,depthWrite:!1})}function lc(){return`

		precision mediump float;
		precision mediump int;

		attribute vec3 outputDirection;

		varying vec3 vOutputDirection;

		void main() {

			vOutputDirection = outputDirection;
			gl_Position = vec4( position, 1.0 );

		}
	`}var oc=class extends Bt{constructor(e=1,t={}){super(e,e,t),this.isWebGLCubeRenderTarget=!0;let n={width:e,height:e,depth:1},i=[n,n,n,n,n,n];this.texture=new ho(i),this._setTextureOptions(t),this.texture.isRenderTargetTexture=!0}fromEquirectangularTexture(e,t){this.texture.type=t.type,this.texture.colorSpace=t.colorSpace,this.texture.generateMipmaps=t.generateMipmaps,this.texture.minFilter=t.minFilter,this.texture.magFilter=t.magFilter;let n={uniforms:{tEquirect:{value:null}},vertexShader:`

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
			`},i=new ui(5,5,5),r=new ct({name:"CubemapFromEquirect",uniforms:Ls(n.uniforms),vertexShader:n.vertexShader,fragmentShader:n.fragmentShader,side:Kt,blending:zn});r.uniforms.tEquirect.value=t;let o=new Ke(i,r),a=t.minFilter;return t.minFilter===$n&&(t.minFilter=Ot),new ll(1,10,this).update(e,o),t.minFilter=a,o.geometry.dispose(),o.material.dispose(),this}clear(e,t=!0,n=!0,i=!0){let r=e.getRenderTarget();for(let o=0;o<6;o++)e.setRenderTarget(this,o),e.clear(t,n,i);e.setRenderTarget(r)}};function t_(s){let e=new WeakMap,t=new WeakMap,n=null;function i(d,f=!1){return d==null?null:f?o(d):r(d)}function r(d){if(d&&d.isTexture){let f=d.mapping;if(f===pl||f===ml)if(e.has(d)){let g=e.get(d).texture;return a(g,d.mapping)}else{let g=d.image;if(g&&g.height>0){let v=new oc(g.height);return v.fromEquirectangularTexture(s,d),e.set(d,v),d.addEventListener("dispose",c),a(v.texture,d.mapping)}else return null}}return d}function o(d){if(d&&d.isTexture){let f=d.mapping,g=f===pl||f===ml,v=f===is||f===Ps;if(g||v){let m=t.get(d),p=m!==void 0?m.texture.pmremVersion:0;if(d.isRenderTargetTexture&&d.pmremVersion!==p)return n===null&&(n=new Ir(s)),m=g?n.fromEquirectangular(d,m):n.fromCubemap(d,m),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),m.texture;if(m!==void 0)return m.texture;{let M=d.image;return g&&M&&M.height>0||v&&M&&l(M)?(n===null&&(n=new Ir(s)),m=g?n.fromEquirectangular(d):n.fromCubemap(d),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),d.addEventListener("dispose",h),m.texture):null}}}return d}function a(d,f){return f===pl?d.mapping=is:f===ml&&(d.mapping=Ps),d}function l(d){let f=0,g=6;for(let v=0;v<g;v++)d[v]!==void 0&&f++;return f===g}function c(d){let f=d.target;f.removeEventListener("dispose",c);let g=e.get(f);g!==void 0&&(e.delete(f),g.dispose())}function h(d){let f=d.target;f.removeEventListener("dispose",h);let g=t.get(f);g!==void 0&&(t.delete(f),g.dispose())}function u(){e=new WeakMap,t=new WeakMap,n!==null&&(n.dispose(),n=null)}return{get:i,dispose:u}}function n_(s){let e={};function t(n){if(e[n]!==void 0)return e[n];let i=s.getExtension(n);return e[n]=i,i}return{has:function(n){return t(n)!==null},init:function(){t("EXT_color_buffer_float"),t("WEBGL_clip_cull_distance"),t("OES_texture_float_linear"),t("EXT_color_buffer_half_float"),t("WEBGL_multisampled_render_to_texture"),t("WEBGL_render_shared_exponent")},get:function(n){let i=t(n);return i===null&&_s("WebGLRenderer: "+n+" extension not supported."),i}}}function i_(s,e,t,n){let i={},r=new WeakMap;function o(u){let d=u.target;d.index!==null&&e.remove(d.index);for(let g in d.attributes)e.remove(d.attributes[g]);d.removeEventListener("dispose",o),delete i[d.id];let f=r.get(d);f&&(e.remove(f),r.delete(d)),n.releaseStatesOfGeometry(d),d.isInstancedBufferGeometry===!0&&delete d._maxInstanceCount,t.memory.geometries--}function a(u,d){return i[d.id]===!0||(d.addEventListener("dispose",o),i[d.id]=!0,t.memory.geometries++),d}function l(u){let d=u.attributes;for(let f in d)e.update(d[f],s.ARRAY_BUFFER)}function c(u){let d=[],f=u.index,g=u.attributes.position,v=0;if(g===void 0)return;if(f!==null){let M=f.array;v=f.version;for(let T=0,x=M.length;T<x;T+=3){let y=M[T+0],S=M[T+1],C=M[T+2];d.push(y,S,S,C,C,y)}}else{let M=g.array;v=g.version;for(let T=0,x=M.length/3-1;T<x;T+=3){let y=T+0,S=T+1,C=T+2;d.push(y,S,S,C,C,y)}}let m=new(g.count>=65535?oo:ro)(d,1);m.version=v;let p=r.get(u);p&&e.remove(p),r.set(u,m)}function h(u){let d=r.get(u);if(d){let f=u.index;f!==null&&d.version<f.version&&c(u)}else c(u);return r.get(u)}return{get:a,update:l,getWireframeAttribute:h}}function s_(s,e,t){let n;function i(u){n=u}let r,o;function a(u){r=u.type,o=u.bytesPerElement}function l(u,d){s.drawElements(n,d,r,u*o),t.update(d,n,1)}function c(u,d,f){f!==0&&(s.drawElementsInstanced(n,d,r,u*o,f),t.update(d,n,f))}function h(u,d,f){if(f===0)return;e.get("WEBGL_multi_draw").multiDrawElementsWEBGL(n,d,0,r,u,0,f);let v=0;for(let m=0;m<f;m++)v+=d[m];t.update(v,n,1)}this.setMode=i,this.setIndex=a,this.render=l,this.renderInstances=c,this.renderMultiDraw=h}function r_(s){let e={geometries:0,textures:0},t={frame:0,calls:0,triangles:0,points:0,lines:0};function n(r,o,a){switch(t.calls++,o){case s.TRIANGLES:t.triangles+=a*(r/3);break;case s.LINES:t.lines+=a*(r/2);break;case s.LINE_STRIP:t.lines+=a*(r-1);break;case s.LINE_LOOP:t.lines+=a*r;break;case s.POINTS:t.points+=a*r;break;default:et("WebGLInfo: Unknown draw mode:",o);break}}function i(){t.calls=0,t.triangles=0,t.points=0,t.lines=0}return{memory:e,render:t,programs:null,autoReset:!0,reset:i,update:n}}function o_(s,e,t){let n=new WeakMap,i=new gt;function r(o,a,l){let c=o.morphTargetInfluences,h=a.morphAttributes.position||a.morphAttributes.normal||a.morphAttributes.color,u=h!==void 0?h.length:0,d=n.get(a);if(d===void 0||d.count!==u){let I=function(){C.dispose(),n.delete(a),a.removeEventListener("dispose",I)};d!==void 0&&d.texture.dispose();let f=a.morphAttributes.position!==void 0,g=a.morphAttributes.normal!==void 0,v=a.morphAttributes.color!==void 0,m=a.morphAttributes.position||[],p=a.morphAttributes.normal||[],M=a.morphAttributes.color||[],T=0;f===!0&&(T=1),g===!0&&(T=2),v===!0&&(T=3);let x=a.attributes.position.count*T,y=1;x>e.maxTextureSize&&(y=Math.ceil(x/e.maxTextureSize),x=e.maxTextureSize);let S=new Float32Array(x*y*4*u),C=new io(S,x,y,u);C.type=Pn,C.needsUpdate=!0;let _=T*4;for(let b=0;b<u;b++){let N=m[b],w=p[b],H=M[b],F=x*y*4*b;for(let L=0;L<N.count;L++){let O=L*_;f===!0&&(i.fromBufferAttribute(N,L),S[F+O+0]=i.x,S[F+O+1]=i.y,S[F+O+2]=i.z,S[F+O+3]=0),g===!0&&(i.fromBufferAttribute(w,L),S[F+O+4]=i.x,S[F+O+5]=i.y,S[F+O+6]=i.z,S[F+O+7]=0),v===!0&&(i.fromBufferAttribute(H,L),S[F+O+8]=i.x,S[F+O+9]=i.y,S[F+O+10]=i.z,S[F+O+11]=H.itemSize===4?i.w:1)}}d={count:u,texture:C,size:new Ue(x,y)},n.set(a,d),a.addEventListener("dispose",I)}if(o.isInstancedMesh===!0&&o.morphTexture!==null)l.getUniforms().setValue(s,"morphTexture",o.morphTexture,t);else{let f=0;for(let v=0;v<c.length;v++)f+=c[v];let g=a.morphTargetsRelative?1:1-f;l.getUniforms().setValue(s,"morphTargetBaseInfluence",g),l.getUniforms().setValue(s,"morphTargetInfluences",c)}l.getUniforms().setValue(s,"morphTargetsTexture",d.texture,t),l.getUniforms().setValue(s,"morphTargetsTextureSize",d.size)}return{update:r}}function a_(s,e,t,n,i){let r=new WeakMap;function o(c){let h=i.render.frame,u=c.geometry,d=e.get(c,u);if(r.get(d)!==h&&(e.update(d),r.set(d,h)),c.isInstancedMesh&&(c.hasEventListener("dispose",l)===!1&&c.addEventListener("dispose",l),r.get(c)!==h&&(t.update(c.instanceMatrix,s.ARRAY_BUFFER),c.instanceColor!==null&&t.update(c.instanceColor,s.ARRAY_BUFFER),r.set(c,h))),c.isSkinnedMesh){let f=c.skeleton;r.get(f)!==h&&(f.update(),r.set(f,h))}return d}function a(){r=new WeakMap}function l(c){let h=c.target;h.removeEventListener("dispose",l),n.releaseStatesOfObject(h),t.remove(h.instanceMatrix),h.instanceColor!==null&&t.remove(h.instanceColor)}return{update:o,dispose:a}}var l_={[Po]:"LINEAR_TONE_MAPPING",[Io]:"REINHARD_TONE_MAPPING",[Lo]:"CINEON_TONE_MAPPING",[Cs]:"ACES_FILMIC_TONE_MAPPING",[No]:"AGX_TONE_MAPPING",[Uo]:"NEUTRAL_TONE_MAPPING",[Do]:"CUSTOM_TONE_MAPPING"};function c_(s,e,t,n,i,r){let o=new Bt(e,t,{type:s,depthBuffer:i,stencilBuffer:r,samples:n?4:0,storeMultisampledDepthBuffer:!1,storeMultisampledStencilBuffer:!1,resolveDepthBuffer:!1,resolveStencilBuffer:!1}),a=null,l=null,c=new ot;c.setAttribute("position",new tt([-1,3,0,-1,-1,0,3,-1,0],3)),c.setAttribute("uv",new tt([0,2,0,0,2,0],2));let h=new _r({uniforms:{tDiffuse:{value:null}},vertexShader:`
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
			}`,depthTest:!1,depthWrite:!1}),u=new Ke(c,h),d=new mi(-1,1,1,-1,0,1),f=null,g=null,v=!1,m,p=null,M=[],T=!1;this.setSize=function(x,y){o.setSize(x,y),a!==null&&a.setSize(x,y),l!==null&&l.setSize(x,y);for(let S=0;S<M.length;S++){let C=M[S];C.setSize&&C.setSize(x,y)}},this.setEffects=function(x){M=x,T=M.length>0&&M[0].isRenderPass===!0;let y=o.width,S=o.height;M.length>0&&a===null&&(a=new Bt(y,S,{type:Yt,depthBuffer:!1,stencilBuffer:!1}),l=new Bt(y,S,{type:Yt,depthBuffer:!1,stencilBuffer:!1}));for(let C=0;C<M.length;C++){let _=M[C];_.setSize&&_.setSize(y,S)}},this.begin=function(x,y){if(v||x.toneMapping===Jn&&M.length===0)return!1;if(p=y,y!==null){let S=y.width,C=y.height;(o.width!==S||o.height!==C)&&this.setSize(S,C)}return T===!1&&x.setRenderTarget(o),m=x.toneMapping,x.toneMapping=Jn,!0},this.hasRenderPass=function(){return T},this.end=function(x,y){x.toneMapping=m,v=!0;let S=o,C=a;for(let _=0;_<M.length;_++){let I=M[_];I.enabled!==!1&&(I.render(x,C,S,y),I.needsSwap!==!1&&(S=C,C=C===a?l:a))}if(f!==x.outputColorSpace||g!==x.toneMapping){f=x.outputColorSpace,g=x.toneMapping,h.defines={},ht.getTransfer(f)===St&&(h.defines.SRGB_TRANSFER="");let _=l_[g];_&&(h.defines[_]=""),h.needsUpdate=!0}h.uniforms.tDiffuse.value=S.texture,x.setRenderTarget(p),x.render(u,d),p=null,v=!1},this.isCompositing=function(){return v},this.dispose=function(){o.dispose(),a!==null&&a.dispose(),l!==null&&l.dispose(),c.dispose(),h.dispose()}}var Vf=new Gt,kh=new Ji(1,1),Gf=new io,Wf=new Ga,Xf=new ho,Sf=[],wf=[],Tf=new Float32Array(16),Ef=new Float32Array(9),Af=new Float32Array(4);function Lr(s,e,t){let n=s[0];if(n<=0||n>0)return s;let i=e*t,r=Sf[i];if(r===void 0&&(r=new Float32Array(i),Sf[i]=r),e!==0){n.toArray(r,0);for(let o=1,a=0;o!==e;++o)a+=t,s[o].toArray(r,a)}return r}function jt(s,e){if(s.length!==e.length)return!1;for(let t=0,n=s.length;t<n;t++)if(s[t]!==e[t])return!1;return!0}function Jt(s,e){for(let t=0,n=e.length;t<n;t++)s[t]=e[t]}function cc(s,e){let t=wf[e];t===void 0&&(t=new Int32Array(e),wf[e]=t);for(let n=0;n!==e;++n)t[n]=s.allocateTextureUnit();return t}function h_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1f(this.addr,e),t[0]=e)}function u_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2f(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(jt(t,e))return;s.uniform2fv(this.addr,e),Jt(t,e)}}function d_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3f(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else if(e.r!==void 0)(t[0]!==e.r||t[1]!==e.g||t[2]!==e.b)&&(s.uniform3f(this.addr,e.r,e.g,e.b),t[0]=e.r,t[1]=e.g,t[2]=e.b);else{if(jt(t,e))return;s.uniform3fv(this.addr,e),Jt(t,e)}}function f_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4f(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(jt(t,e))return;s.uniform4fv(this.addr,e),Jt(t,e)}}function p_(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(jt(t,e))return;s.uniformMatrix2fv(this.addr,!1,e),Jt(t,e)}else{if(jt(t,n))return;Af.set(n),s.uniformMatrix2fv(this.addr,!1,Af),Jt(t,n)}}function m_(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(jt(t,e))return;s.uniformMatrix3fv(this.addr,!1,e),Jt(t,e)}else{if(jt(t,n))return;Ef.set(n),s.uniformMatrix3fv(this.addr,!1,Ef),Jt(t,n)}}function g_(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(jt(t,e))return;s.uniformMatrix4fv(this.addr,!1,e),Jt(t,e)}else{if(jt(t,n))return;Tf.set(n),s.uniformMatrix4fv(this.addr,!1,Tf),Jt(t,n)}}function x_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1i(this.addr,e),t[0]=e)}function __(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2i(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(jt(t,e))return;s.uniform2iv(this.addr,e),Jt(t,e)}}function v_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3i(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(jt(t,e))return;s.uniform3iv(this.addr,e),Jt(t,e)}}function y_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4i(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(jt(t,e))return;s.uniform4iv(this.addr,e),Jt(t,e)}}function M_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1ui(this.addr,e),t[0]=e)}function b_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2ui(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(jt(t,e))return;s.uniform2uiv(this.addr,e),Jt(t,e)}}function S_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3ui(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(jt(t,e))return;s.uniform3uiv(this.addr,e),Jt(t,e)}}function w_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4ui(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(jt(t,e))return;s.uniform4uiv(this.addr,e),Jt(t,e)}}function T_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i);let r;this.type===s.SAMPLER_2D_SHADOW?(kh.compareFunction=t.isReversedDepthBuffer()?ic:nc,r=kh):r=Vf,t.setTexture2D(e||r,i)}function E_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture3D(e||Wf,i)}function A_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTextureCube(e||Xf,i)}function R_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture2DArray(e||Gf,i)}function C_(s){switch(s){case 5126:return h_;case 35664:return u_;case 35665:return d_;case 35666:return f_;case 35674:return p_;case 35675:return m_;case 35676:return g_;case 5124:case 35670:return x_;case 35667:case 35671:return __;case 35668:case 35672:return v_;case 35669:case 35673:return y_;case 5125:return M_;case 36294:return b_;case 36295:return S_;case 36296:return w_;case 35678:case 36198:case 36298:case 36306:case 35682:return T_;case 35679:case 36299:case 36307:return E_;case 35680:case 36300:case 36308:case 36293:return A_;case 36289:case 36303:case 36311:case 36292:return R_}}function P_(s,e){s.uniform1fv(this.addr,e)}function I_(s,e){let t=Lr(e,this.size,2);s.uniform2fv(this.addr,t)}function L_(s,e){let t=Lr(e,this.size,3);s.uniform3fv(this.addr,t)}function D_(s,e){let t=Lr(e,this.size,4);s.uniform4fv(this.addr,t)}function N_(s,e){let t=Lr(e,this.size,4);s.uniformMatrix2fv(this.addr,!1,t)}function U_(s,e){let t=Lr(e,this.size,9);s.uniformMatrix3fv(this.addr,!1,t)}function F_(s,e){let t=Lr(e,this.size,16);s.uniformMatrix4fv(this.addr,!1,t)}function O_(s,e){s.uniform1iv(this.addr,e)}function B_(s,e){s.uniform2iv(this.addr,e)}function z_(s,e){s.uniform3iv(this.addr,e)}function k_(s,e){s.uniform4iv(this.addr,e)}function H_(s,e){s.uniform1uiv(this.addr,e)}function V_(s,e){s.uniform2uiv(this.addr,e)}function G_(s,e){s.uniform3uiv(this.addr,e)}function W_(s,e){s.uniform4uiv(this.addr,e)}function X_(s,e,t){let n=this.cache,i=e.length,r=cc(t,i);jt(n,r)||(s.uniform1iv(this.addr,r),Jt(n,r));let o;this.type===s.SAMPLER_2D_SHADOW?o=kh:o=Vf;for(let a=0;a!==i;++a)t.setTexture2D(e[a]||o,r[a])}function q_(s,e,t){let n=this.cache,i=e.length,r=cc(t,i);jt(n,r)||(s.uniform1iv(this.addr,r),Jt(n,r));for(let o=0;o!==i;++o)t.setTexture3D(e[o]||Wf,r[o])}function Y_(s,e,t){let n=this.cache,i=e.length,r=cc(t,i);jt(n,r)||(s.uniform1iv(this.addr,r),Jt(n,r));for(let o=0;o!==i;++o)t.setTextureCube(e[o]||Xf,r[o])}function Z_(s,e,t){let n=this.cache,i=e.length,r=cc(t,i);jt(n,r)||(s.uniform1iv(this.addr,r),Jt(n,r));for(let o=0;o!==i;++o)t.setTexture2DArray(e[o]||Gf,r[o])}function K_(s){switch(s){case 5126:return P_;case 35664:return I_;case 35665:return L_;case 35666:return D_;case 35674:return N_;case 35675:return U_;case 35676:return F_;case 5124:case 35670:return O_;case 35667:case 35671:return B_;case 35668:case 35672:return z_;case 35669:case 35673:return k_;case 5125:return H_;case 36294:return V_;case 36295:return G_;case 36296:return W_;case 35678:case 36198:case 36298:case 36306:case 35682:return X_;case 35679:case 36299:case 36307:return q_;case 35680:case 36300:case 36308:case 36293:return Y_;case 36289:case 36303:case 36311:case 36292:return Z_}}var Hh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.setValue=C_(t.type)}},Vh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.size=t.size,this.setValue=K_(t.type)}},Gh=class{constructor(e){this.id=e,this.seq=[],this.map={}}setValue(e,t,n){let i=this.seq;for(let r=0,o=i.length;r!==o;++r){let a=i[r];a.setValue(e,t[a.id],n)}}},Bh=/(\w+)(\])?(\[|\.)?/g;function Rf(s,e){s.seq.push(e),s.map[e.id]=e}function j_(s,e,t){let n=s.name,i=n.length;for(Bh.lastIndex=0;;){let r=Bh.exec(n),o=Bh.lastIndex,a=r[1],l=r[2]==="]",c=r[3];if(l&&(a=a|0),c===void 0||c==="["&&o+2===i){Rf(t,c===void 0?new Hh(a,s,e):new Vh(a,s,e));break}else{let u=t.map[a];u===void 0&&(u=new Gh(a),Rf(t,u)),t=u}}}var Pr=class{constructor(e,t){this.seq=[],this.map={};let n=e.getProgramParameter(t,e.ACTIVE_UNIFORMS);for(let o=0;o<n;++o){let a=e.getActiveUniform(t,o),l=e.getUniformLocation(t,a.name);j_(a,l,this)}let i=[],r=[];for(let o of this.seq)o.type===e.SAMPLER_2D_SHADOW||o.type===e.SAMPLER_CUBE_SHADOW||o.type===e.SAMPLER_2D_ARRAY_SHADOW?i.push(o):r.push(o);i.length>0&&(this.seq=i.concat(r))}setValue(e,t,n,i){let r=this.map[t];r!==void 0&&r.setValue(e,n,i)}setOptional(e,t,n){let i=t[n];i!==void 0&&this.setValue(e,n,i)}static upload(e,t,n,i){for(let r=0,o=t.length;r!==o;++r){let a=t[r],l=n[a.id];l.needsUpdate!==!1&&a.setValue(e,l.value,i)}}static seqWithValue(e,t){let n=[];for(let i=0,r=e.length;i!==r;++i){let o=e[i];o.id in t&&n.push(o)}return n}};function Cf(s,e,t){let n=s.createShader(e);return s.shaderSource(n,t),s.compileShader(n),n}var J_=37297,$_=0;function Q_(s,e){let t=s.split(`
`),n=[],i=Math.max(e-6,0),r=Math.min(e+6,t.length);for(let o=i;o<r;o++){let a=o+1;n.push(`${a===e?">":" "} ${a}: ${t[o]}`)}return n.join(`
`)}var Pf=new rt;function ev(s){ht._getMatrix(Pf,ht.workingColorSpace,s);let e=`mat3( ${Pf.elements.map(t=>t.toFixed(4))} )`;switch(ht.getTransfer(s)){case to:return[e,"LinearTransferOETF"];case St:return[e,"sRGBTransferOETF"];default:return Ze("WebGLProgram: Unsupported color space: ",s),[e,"LinearTransferOETF"]}}function If(s,e,t){let n=s.getShaderParameter(e,s.COMPILE_STATUS),r=(s.getShaderInfoLog(e)||"").trim();if(n&&r==="")return"";let o=/ERROR: 0:(\d+)/.exec(r);if(o){let a=parseInt(o[1]);return t.toUpperCase()+`

`+r+`

`+Q_(s.getShaderSource(e),a)}else return r}function tv(s,e){let t=ev(e);return[`vec4 ${s}( vec4 value ) {`,`	return ${t[1]}( vec4( value.rgb * ${t[0]}, value.a ) );`,"}"].join(`
`)}var nv={[Po]:"Linear",[Io]:"Reinhard",[Lo]:"Cineon",[Cs]:"ACESFilmic",[No]:"AgX",[Uo]:"Neutral",[Do]:"Custom"};function iv(s,e){let t=nv[e];return t===void 0?(Ze("WebGLProgram: Unsupported toneMapping:",e),"vec3 "+s+"( vec3 color ) { return LinearToneMapping( color ); }"):"vec3 "+s+"( vec3 color ) { return "+t+"ToneMapping( color ); }"}var rc=new W;function sv(){ht.getLuminanceCoefficients(rc);let s=rc.x.toFixed(4),e=rc.y.toFixed(4),t=rc.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${s}, ${e}, ${t} );`,"	return dot( weights, rgb );","}"].join(`
`)}function rv(s){return[s.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",s.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(Yo).join(`
`)}function ov(s){let e=[];for(let t in s){let n=s[t];n!==!1&&e.push("#define "+t+" "+n)}return e.join(`
`)}function av(s,e){let t={},n=s.getProgramParameter(e,s.ACTIVE_ATTRIBUTES);for(let i=0;i<n;i++){let r=s.getActiveAttrib(e,i),o=r.name,a=1;r.type===s.FLOAT_MAT2&&(a=2),r.type===s.FLOAT_MAT3&&(a=3),r.type===s.FLOAT_MAT4&&(a=4),t[o]={type:r.type,location:s.getAttribLocation(e,o),locationSize:a}}return t}function Yo(s){return s!==""}function Lf(s,e){let t=e.numSpotLightShadows+e.numSpotLightMaps-e.numSpotLightShadowsWithMaps;return s.replace(/NUM_SUN_LIGHTS/g,e.numSunLights).replace(/NUM_DIR_LIGHTS/g,e.numDirLights).replace(/NUM_SPOT_LIGHTS/g,e.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,e.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,t).replace(/NUM_RECT_AREA_LIGHTS/g,e.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,e.numPointLights).replace(/NUM_HEMI_LIGHTS/g,e.numHemiLights).replace(/NUM_SUN_LIGHT_SHADOWS/g,e.numSunLightShadows).replace(/NUM_DIR_LIGHT_SHADOWS/g,e.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,e.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,e.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,e.numPointLightShadows)}function Df(s,e){return s.replace(/NUM_CLIPPING_PLANES/g,e.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,e.numClippingPlanes-e.numClipIntersection)}var lv=/^[ \t]*#include +<([\w\d./]+)>/gm;function Wh(s){return s.replace(lv,hv)}var cv=new Map;function hv(s,e){let t=pt[e];if(t===void 0){let n=cv.get(e);if(n!==void 0)t=pt[n],Ze('WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',e,n);else throw new Error("THREE.WebGLProgram: Can not resolve #include <"+e+">")}return Wh(t)}var uv=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function Nf(s){return s.replace(uv,dv)}function dv(s,e,t,n){let i="";for(let r=parseInt(e);r<parseInt(t);r++)i+=n.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return i}function Uf(s){let e=`precision ${s.precision} float;
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
#define LOW_PRECISION`),e}var fv={[Co]:"SHADOWMAP_TYPE_PCF",[br]:"SHADOWMAP_TYPE_VSM"};function pv(s){return fv[s.shadowMapType]||"SHADOWMAP_TYPE_BASIC"}var mv={[is]:"ENVMAP_TYPE_CUBE",[Ps]:"ENVMAP_TYPE_CUBE",[Fo]:"ENVMAP_TYPE_CUBE_UV"};function gv(s){return s.envMap===!1?"ENVMAP_TYPE_CUBE":mv[s.envMapMode]||"ENVMAP_TYPE_CUBE"}var xv={[Ps]:"ENVMAP_MODE_REFRACTION"};function _v(s){return s.envMap===!1?"ENVMAP_MODE_REFLECTION":xv[s.envMapMode]||"ENVMAP_MODE_REFLECTION"}var vv={[fl]:"ENVMAP_BLENDING_MULTIPLY",[jd]:"ENVMAP_BLENDING_MIX",[Jd]:"ENVMAP_BLENDING_ADD"};function yv(s){return s.envMap===!1?"ENVMAP_BLENDING_NONE":vv[s.combine]||"ENVMAP_BLENDING_NONE"}function Mv(s){let e=s.envMapCubeUVHeight;if(e===null)return null;let t=Math.log2(e)-2,n=1/e;return{texelWidth:1/(3*Math.max(Math.pow(2,t),112)),texelHeight:n,maxMip:t}}function bv(s,e,t,n){let i=s.getContext(),r=t.defines,o=t.vertexShader,a=t.fragmentShader,l=pv(t),c=gv(t),h=_v(t),u=yv(t),d=Mv(t),f=rv(t),g=ov(r),v=i.createProgram(),m,p,M=t.glslVersion?"#version "+t.glslVersion+`
`:"";t.isRawShaderMaterial?(m=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(Yo).join(`
`),m.length>0&&(m+=`
`),p=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(Yo).join(`
`),p.length>0&&(p+=`
`)):(m=[Uf(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",t.batching?"#define USE_BATCHING":"",t.batchingColor?"#define USE_BATCHING_COLOR":"",t.instancing?"#define USE_INSTANCING":"",t.instancingColor?"#define USE_INSTANCING_COLOR":"",t.instancingMorph?"#define USE_INSTANCING_MORPH":"",t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.map?"#define USE_MAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+h:"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.displacementMap?"#define USE_DISPLACEMENTMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.mapUv?"#define MAP_UV "+t.mapUv:"",t.alphaMapUv?"#define ALPHAMAP_UV "+t.alphaMapUv:"",t.lightMapUv?"#define LIGHTMAP_UV "+t.lightMapUv:"",t.aoMapUv?"#define AOMAP_UV "+t.aoMapUv:"",t.emissiveMapUv?"#define EMISSIVEMAP_UV "+t.emissiveMapUv:"",t.bumpMapUv?"#define BUMPMAP_UV "+t.bumpMapUv:"",t.normalMapUv?"#define NORMALMAP_UV "+t.normalMapUv:"",t.displacementMapUv?"#define DISPLACEMENTMAP_UV "+t.displacementMapUv:"",t.metalnessMapUv?"#define METALNESSMAP_UV "+t.metalnessMapUv:"",t.roughnessMapUv?"#define ROUGHNESSMAP_UV "+t.roughnessMapUv:"",t.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+t.anisotropyMapUv:"",t.clearcoatMapUv?"#define CLEARCOATMAP_UV "+t.clearcoatMapUv:"",t.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+t.clearcoatNormalMapUv:"",t.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+t.clearcoatRoughnessMapUv:"",t.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+t.iridescenceMapUv:"",t.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+t.iridescenceThicknessMapUv:"",t.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+t.sheenColorMapUv:"",t.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+t.sheenRoughnessMapUv:"",t.specularMapUv?"#define SPECULARMAP_UV "+t.specularMapUv:"",t.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+t.specularColorMapUv:"",t.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+t.specularIntensityMapUv:"",t.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+t.transmissionMapUv:"",t.thicknessMapUv?"#define THICKNESSMAP_UV "+t.thicknessMapUv:"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexNormals?"#define HAS_NORMAL":"",t.vertexColors?"#define USE_COLOR":"",t.vertexAlphas?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.flatShading?"#define FLAT_SHADED":"",t.skinning?"#define USE_SKINNING":"",t.morphTargets?"#define USE_MORPHTARGETS":"",t.morphNormals&&t.flatShading===!1?"#define USE_MORPHNORMALS":"",t.morphColors?"#define USE_MORPHCOLORS":"",t.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+t.morphTextureStride:"",t.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+t.morphTargetsCount:"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.sizeAttenuation?"#define USE_SIZEATTENUATION":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(Yo).join(`
`),p=[Uf(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",t.map?"#define USE_MAP":"",t.matcap?"#define USE_MATCAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+c:"",t.envMap?"#define "+h:"",t.envMap?"#define "+u:"",d?"#define CUBEUV_TEXEL_WIDTH "+d.texelWidth:"",d?"#define CUBEUV_TEXEL_HEIGHT "+d.texelHeight:"",d?"#define CUBEUV_MAX_MIP "+d.maxMip+".0":"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.packedNormalMap?"#define USE_PACKED_NORMALMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoat?"#define USE_CLEARCOAT":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.dispersion?"#define USE_DISPERSION":"",t.retroreflection?"#define USE_RETROREFLECTION":"",t.iridescence?"#define USE_IRIDESCENCE":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaTest?"#define USE_ALPHATEST":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.sheen?"#define USE_SHEEN":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexColors||t.instancingColor?"#define USE_COLOR":"",t.vertexAlphas||t.batchingColor?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.gradientMap?"#define USE_GRADIENTMAP":"",t.flatShading?"#define FLAT_SHADED":"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.numLightProbeGrids>0?"#define USE_LIGHT_PROBES_GRID":"",t.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",t.decodeVideoTextureEmissive?"#define DECODE_VIDEO_TEXTURE_EMISSIVE":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",t.toneMapping!==Jn?"#define TONE_MAPPING":"",t.toneMapping!==Jn?pt.tonemapping_pars_fragment:"",t.toneMapping!==Jn?iv("toneMapping",t.toneMapping):"",t.dithering?"#define DITHERING":"",t.opaque?"#define OPAQUE":"",pt.colorspace_pars_fragment,tv("linearToOutputTexel",t.outputColorSpace),sv(),t.useDepthPacking?"#define DEPTH_PACKING "+t.depthPacking:"",`
`].filter(Yo).join(`
`)),o=Wh(o),o=Lf(o,t),o=Df(o,t),a=Wh(a),a=Lf(a,t),a=Df(a,t),o=Nf(o),a=Nf(a),t.isRawShaderMaterial!==!0&&(M=`#version 300 es
`,m=[f,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+m,p=["#define varying in",t.glslVersion===Ah?"":"layout(location = 0) out highp vec4 pc_fragColor;",t.glslVersion===Ah?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+p);let T=M+m+o,x=M+p+a,y=Cf(i,i.VERTEX_SHADER,T),S=Cf(i,i.FRAGMENT_SHADER,x);i.attachShader(v,y),i.attachShader(v,S),t.index0AttributeName!==void 0?i.bindAttribLocation(v,0,t.index0AttributeName):t.hasPositionAttribute===!0&&i.bindAttribLocation(v,0,"position"),i.linkProgram(v);function C(N){if(s.debug.checkShaderErrors){let w=i.getProgramInfoLog(v)||"",H=i.getShaderInfoLog(y)||"",F=i.getShaderInfoLog(S)||"",L=w.trim(),O=H.trim(),V=F.trim(),R=!0,G=!0;if(i.getProgramParameter(v,i.LINK_STATUS)===!1)if(R=!1,typeof s.debug.onShaderError=="function")s.debug.onShaderError(i,v,y,S);else{let oe=If(i,y,"vertex"),k=If(i,S,"fragment");et("WebGLProgram: Shader Error "+i.getError()+" - VALIDATE_STATUS "+i.getProgramParameter(v,i.VALIDATE_STATUS)+`

Material Name: `+N.name+`
Material Type: `+N.type+`

Program Info Log: `+L+`
`+oe+`
`+k)}else L!==""?Ze("WebGLProgram: Program Info Log:",L):(O===""||V==="")&&(G=!1);G&&(N.diagnostics={runnable:R,programLog:L,vertexShader:{log:O,prefix:m},fragmentShader:{log:V,prefix:p}})}i.deleteShader(y),i.deleteShader(S),_=new Pr(i,v),I=av(i,v)}let _;this.getUniforms=function(){return _===void 0&&C(this),_};let I;this.getAttributes=function(){return I===void 0&&C(this),I};let b=t.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return b===!1&&(b=i.getProgramParameter(v,J_)),b},this.destroy=function(){n.releaseStatesOfProgram(this),i.deleteProgram(v),this.program=void 0},this.type=t.shaderType,this.name=t.shaderName,this.id=$_++,this.cacheKey=e,this.usedTimes=1,this.program=v,this.vertexShader=y,this.fragmentShader=S,this}var Sv=0,Xh=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(e,t,n){let i=this._getShaderCacheForMaterial(e);return i.has(t)===!1&&(i.add(t),t.usedTimes++),i.has(n)===!1&&(i.add(n),n.usedTimes++),this}remove(e){let t=this.materialCache.get(e);for(let n of t)n.usedTimes--,n.usedTimes===0&&this.shaderCache.delete(n.code);return this.materialCache.delete(e),this}getVertexShaderStage(e){return this._getShaderStage(e.vertexShader)}getFragmentShaderStage(e){return this._getShaderStage(e.fragmentShader)}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(e){let t=this.materialCache,n=t.get(e);return n===void 0&&(n=new Set,t.set(e,n)),n}_getShaderStage(e){let t=this.shaderCache,n=t.get(e);return n===void 0&&(n=new qh(e),t.set(e,n)),n}},qh=class{constructor(e){this.id=Sv++,this.code=e,this.usedTimes=0}};function wv(s){return s===rs||s===Ho||s===Vo}function Tv(s,e,t,n,i,r){let o=new ar,a=new Xh,l=new Set,c=[],h=new Map,u=n.logarithmicDepthBuffer,d=n.precision,f={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distance",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function g(_){return l.add(_),_===0?"uv":`uv${_}`}function v(_,I,b,N,w,H){let F=N.fog,L=w.geometry,O=_.isMeshStandardMaterial||_.isMeshLambertMaterial||_.isMeshPhongMaterial?N.environment:null,V=_.isMeshStandardMaterial||_.isMeshLambertMaterial&&!_.envMap||_.isMeshPhongMaterial&&!_.envMap,R=e.get(_.envMap||O,V),G=R&&R.mapping===Fo?R.image.height:null,oe=f[_.type];_.precision!==null&&(d=n.getMaxPrecision(_.precision),d!==_.precision&&Ze("WebGLProgram.getParameters:",_.precision,"not supported, using",d,"instead."));let k=L.morphAttributes.position||L.morphAttributes.normal||L.morphAttributes.color,re=k!==void 0?k.length:0,Ae=0;L.morphAttributes.position!==void 0&&(Ae=1),L.morphAttributes.normal!==void 0&&(Ae=2),L.morphAttributes.color!==void 0&&(Ae=3);let Le,be,Fe,le;if(oe){let je=xi[oe];Le=je.vertexShader,be=je.fragmentShader}else{Le=_.vertexShader,be=_.fragmentShader;let je=a.getVertexShaderStage(_),Je=a.getFragmentShaderStage(_);a.update(_,je,Je),Fe=je.id,le=Je.id}let ge=s.getRenderTarget(),Re=s.state.buffers.depth.getReversed(),de=w.isInstancedMesh===!0,he=w.isBatchedMesh===!0,ce=!!_.map,Se=!!_.matcap,fe=!!R,ue=!!_.aoMap,Me=!!_.lightMap,Ee=!!_.bumpMap&&_.wireframe===!1,Ge=!!_.normalMap,qe=!!_.displacementMap,ke=!!_.emissiveMap,Ye=!!_.metalnessMap,Qe=!!_.roughnessMap,Q=_.anisotropy>0,st=_.clearcoat>0,$e=_.dispersion>0,X=_.retroreflectivity>0,E=_.iridescence>0,ae=_.sheen>0,se=_.transmission>0,_e=Q&&!!_.anisotropyMap,K=st&&!!_.clearcoatMap,te=st&&!!_.clearcoatNormalMap,J=st&&!!_.clearcoatRoughnessMap,$=E&&!!_.iridescenceMap,xe=E&&!!_.iridescenceThicknessMap,ve=ae&&!!_.sheenColorMap,me=ae&&!!_.sheenRoughnessMap,pe=!!_.specularMap,we=!!_.specularColorMap,B=!!_.specularIntensityMap,q=se&&!!_.transmissionMap,D=se&&!!_.thicknessMap,Y=!!_.gradientMap,A=!!_.alphaMap,z=_.alphaTest>0,ee=!!_.alphaHash,ne=!!_.extensions,Te=Jn;_.toneMapped&&(ge===null||ge.isXRRenderTarget===!0)&&(Te=s.toneMapping);let ye={shaderID:oe,shaderType:_.type,shaderName:_.name,vertexShader:Le,fragmentShader:be,defines:_.defines,customVertexShaderID:Fe,customFragmentShaderID:le,isRawShaderMaterial:_.isRawShaderMaterial===!0,glslVersion:_.glslVersion,precision:d,batching:he,batchingColor:he&&w._colorsTexture!==null,instancing:de,instancingColor:de&&w.instanceColor!==null,instancingMorph:de&&w.morphTexture!==null,outputColorSpace:ge===null?s.outputColorSpace:ge.isXRRenderTarget===!0?ge.texture.colorSpace:ht.workingColorSpace,alphaToCoverage:!!_.alphaToCoverage,map:ce,matcap:Se,envMap:fe,envMapMode:fe&&R.mapping,envMapCubeUVHeight:G,aoMap:ue,lightMap:Me,bumpMap:Ee,normalMap:Ge,displacementMap:qe,emissiveMap:ke,normalMapObjectSpace:Ge&&_.normalMapType===sf,normalMapTangentSpace:Ge&&_.normalMapType===Wo,packedNormalMap:Ge&&_.normalMapType===Wo&&wv(_.normalMap.format),metalnessMap:Ye,roughnessMap:Qe,anisotropy:Q,anisotropyMap:_e,clearcoat:st,clearcoatMap:K,clearcoatNormalMap:te,clearcoatRoughnessMap:J,dispersion:$e,retroreflection:X,iridescence:E,iridescenceMap:$,iridescenceThicknessMap:xe,sheen:ae,sheenColorMap:ve,sheenRoughnessMap:me,specularMap:pe,specularColorMap:we,specularIntensityMap:B,transmission:se,transmissionMap:q,thicknessMap:D,gradientMap:Y,opaque:_.transparent===!1&&_.blending===Sr&&_.alphaToCoverage===!1,alphaMap:A,alphaTest:z,alphaHash:ee,combine:_.combine,mapUv:ce&&g(_.map.channel),aoMapUv:ue&&g(_.aoMap.channel),lightMapUv:Me&&g(_.lightMap.channel),bumpMapUv:Ee&&g(_.bumpMap.channel),normalMapUv:Ge&&g(_.normalMap.channel),displacementMapUv:qe&&g(_.displacementMap.channel),emissiveMapUv:ke&&g(_.emissiveMap.channel),metalnessMapUv:Ye&&g(_.metalnessMap.channel),roughnessMapUv:Qe&&g(_.roughnessMap.channel),anisotropyMapUv:_e&&g(_.anisotropyMap.channel),clearcoatMapUv:K&&g(_.clearcoatMap.channel),clearcoatNormalMapUv:te&&g(_.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:J&&g(_.clearcoatRoughnessMap.channel),iridescenceMapUv:$&&g(_.iridescenceMap.channel),iridescenceThicknessMapUv:xe&&g(_.iridescenceThicknessMap.channel),sheenColorMapUv:ve&&g(_.sheenColorMap.channel),sheenRoughnessMapUv:me&&g(_.sheenRoughnessMap.channel),specularMapUv:pe&&g(_.specularMap.channel),specularColorMapUv:we&&g(_.specularColorMap.channel),specularIntensityMapUv:B&&g(_.specularIntensityMap.channel),transmissionMapUv:q&&g(_.transmissionMap.channel),thicknessMapUv:D&&g(_.thicknessMap.channel),alphaMapUv:A&&g(_.alphaMap.channel),vertexTangents:!!L.attributes.tangent&&(Ge||Q),vertexNormals:!!L.attributes.normal,vertexColors:_.vertexColors,vertexAlphas:_.vertexColors===!0&&!!L.attributes.color&&L.attributes.color.itemSize===4,pointsUvs:w.isPoints===!0&&!!L.attributes.uv&&(ce||A),fog:!!F,useFog:_.fog===!0,fogExp2:!!F&&F.isFogExp2,flatShading:_.wireframe===!1&&(_.flatShading===!0||L.attributes.normal===void 0&&Ge===!1&&(_.isMeshLambertMaterial||_.isMeshPhongMaterial||_.isMeshStandardMaterial||_.isMeshPhysicalMaterial)),sizeAttenuation:_.sizeAttenuation===!0,logarithmicDepthBuffer:u,reversedDepthBuffer:Re,skinning:w.isSkinnedMesh===!0,hasPositionAttribute:L.attributes.position!==void 0,morphTargets:L.morphAttributes.position!==void 0,morphNormals:L.morphAttributes.normal!==void 0,morphColors:L.morphAttributes.color!==void 0,morphTargetsCount:re,morphTextureStride:Ae,numSunLights:I.sun.length,numDirLights:I.directional.length,numPointLights:I.point.length,numSpotLights:I.spot.length,numSpotLightMaps:I.spotLightMap.length,numRectAreaLights:I.rectArea.length,numHemiLights:I.hemi.length,numSunLightShadows:I.sunShadowMap.length,numDirLightShadows:I.directionalShadowMap.length,numPointLightShadows:I.pointShadowMap.length,numSpotLightShadows:I.spotShadowMap.length,numSpotLightShadowsWithMaps:I.numSpotLightShadowsWithMaps,numLightProbes:I.numLightProbes,numLightProbeGrids:H.length,numClippingPlanes:r.numPlanes,numClipIntersection:r.numIntersection,dithering:_.dithering,shadowMapEnabled:s.shadowMap.enabled&&b.length>0,shadowMapType:s.shadowMap.type,toneMapping:Te,decodeVideoTexture:ce&&_.map.isVideoTexture===!0&&ht.getTransfer(_.map.colorSpace)===St,decodeVideoTextureEmissive:ke&&_.emissiveMap.isVideoTexture===!0&&ht.getTransfer(_.emissiveMap.colorSpace)===St,premultipliedAlpha:_.premultipliedAlpha,doubleSided:_.side===Rt,flipSided:_.side===Kt,useDepthPacking:_.depthPacking>=0,depthPacking:_.depthPacking||0,index0AttributeName:_.index0AttributeName,extensionClipCullDistance:ne&&_.extensions.clipCullDistance===!0&&t.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(ne&&_.extensions.multiDraw===!0||he)&&t.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:t.has("KHR_parallel_shader_compile"),customProgramCacheKey:_.customProgramCacheKey()};return ye.vertexUv1s=l.has(1),ye.vertexUv2s=l.has(2),ye.vertexUv3s=l.has(3),l.clear(),ye}function m(_){let I=[];if(_.shaderID?I.push(_.shaderID):(I.push(_.customVertexShaderID),I.push(_.customFragmentShaderID)),_.defines!==void 0)for(let b in _.defines)I.push(b),I.push(_.defines[b]);return _.isRawShaderMaterial===!1&&(p(I,_),M(I,_),I.push(s.outputColorSpace)),I.push(_.customProgramCacheKey),I.join()}function p(_,I){_.push(I.precision),_.push(I.outputColorSpace),_.push(I.envMapMode),_.push(I.envMapCubeUVHeight),_.push(I.mapUv),_.push(I.alphaMapUv),_.push(I.lightMapUv),_.push(I.aoMapUv),_.push(I.bumpMapUv),_.push(I.normalMapUv),_.push(I.displacementMapUv),_.push(I.emissiveMapUv),_.push(I.metalnessMapUv),_.push(I.roughnessMapUv),_.push(I.anisotropyMapUv),_.push(I.clearcoatMapUv),_.push(I.clearcoatNormalMapUv),_.push(I.clearcoatRoughnessMapUv),_.push(I.iridescenceMapUv),_.push(I.iridescenceThicknessMapUv),_.push(I.sheenColorMapUv),_.push(I.sheenRoughnessMapUv),_.push(I.specularMapUv),_.push(I.specularColorMapUv),_.push(I.specularIntensityMapUv),_.push(I.transmissionMapUv),_.push(I.thicknessMapUv),_.push(I.combine),_.push(I.fogExp2),_.push(I.sizeAttenuation),_.push(I.morphTargetsCount),_.push(I.morphAttributeCount),_.push(I.numSunLights),_.push(I.numDirLights),_.push(I.numPointLights),_.push(I.numSpotLights),_.push(I.numSpotLightMaps),_.push(I.numHemiLights),_.push(I.numRectAreaLights),_.push(I.numSunLightShadows),_.push(I.numDirLightShadows),_.push(I.numPointLightShadows),_.push(I.numSpotLightShadows),_.push(I.numSpotLightShadowsWithMaps),_.push(I.numLightProbes),_.push(I.shadowMapType),_.push(I.toneMapping),_.push(I.numClippingPlanes),_.push(I.numClipIntersection),_.push(I.depthPacking)}function M(_,I){o.disableAll(),I.instancing&&o.enable(0),I.instancingColor&&o.enable(1),I.instancingMorph&&o.enable(2),I.matcap&&o.enable(3),I.envMap&&o.enable(4),I.normalMapObjectSpace&&o.enable(5),I.normalMapTangentSpace&&o.enable(6),I.clearcoat&&o.enable(7),I.iridescence&&o.enable(8),I.alphaTest&&o.enable(9),I.vertexColors&&o.enable(10),I.vertexAlphas&&o.enable(11),I.vertexUv1s&&o.enable(12),I.vertexUv2s&&o.enable(13),I.vertexUv3s&&o.enable(14),I.vertexTangents&&o.enable(15),I.anisotropy&&o.enable(16),I.alphaHash&&o.enable(17),I.batching&&o.enable(18),I.dispersion&&o.enable(19),I.retroreflection&&o.enable(24),I.batchingColor&&o.enable(20),I.gradientMap&&o.enable(21),I.packedNormalMap&&o.enable(22),I.vertexNormals&&o.enable(23),_.push(o.mask),o.disableAll(),I.fog&&o.enable(0),I.useFog&&o.enable(1),I.flatShading&&o.enable(2),I.logarithmicDepthBuffer&&o.enable(3),I.reversedDepthBuffer&&o.enable(4),I.skinning&&o.enable(5),I.morphTargets&&o.enable(6),I.morphNormals&&o.enable(7),I.morphColors&&o.enable(8),I.premultipliedAlpha&&o.enable(9),I.shadowMapEnabled&&o.enable(10),I.doubleSided&&o.enable(11),I.flipSided&&o.enable(12),I.useDepthPacking&&o.enable(13),I.dithering&&o.enable(14),I.transmission&&o.enable(15),I.sheen&&o.enable(16),I.opaque&&o.enable(17),I.pointsUvs&&o.enable(18),I.decodeVideoTexture&&o.enable(19),I.decodeVideoTextureEmissive&&o.enable(20),I.alphaToCoverage&&o.enable(21),I.numLightProbeGrids>0&&o.enable(22),I.hasPositionAttribute&&o.enable(23),_.push(o.mask)}function T(_){let I=f[_.type],b;if(I){let N=xi[I];b=ei.clone(N.uniforms)}else b=_.uniforms;return b}function x(_,I){let b=h.get(I);return b!==void 0?++b.usedTimes:(b=new bv(s,I,_,i),c.push(b),h.set(I,b)),b}function y(_){if(--_.usedTimes===0){let I=c.indexOf(_);c[I]=c[c.length-1],c.pop(),h.delete(_.cacheKey),_.destroy()}}function S(_){a.remove(_)}function C(){a.dispose()}return{getParameters:v,getProgramCacheKey:m,getUniforms:T,acquireProgram:x,releaseProgram:y,releaseShaderCache:S,programs:c,dispose:C}}function Ev(){let s=new WeakMap;function e(o){return s.has(o)}function t(o){let a=s.get(o);return a===void 0&&(a={},s.set(o,a)),a}function n(o){s.delete(o)}function i(o,a,l){s.get(o)[a]=l}function r(){s=new WeakMap}return{has:e,get:t,remove:n,update:i,dispose:r}}function Av(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.material.id!==e.material.id?s.material.id-e.material.id:s.materialVariant!==e.materialVariant?s.materialVariant-e.materialVariant:s.z!==e.z?s.z-e.z:s.id-e.id}function Ff(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.z!==e.z?e.z-s.z:s.id-e.id}function Of(){let s=[],e=0,t=[],n=[],i=[];function r(){e=0,t.length=0,n.length=0,i.length=0}function o(d){let f=0;return d.isInstancedMesh&&(f+=2),d.isSkinnedMesh&&(f+=1),f}function a(d,f,g,v,m,p){let M=s[e];return M===void 0?(M={id:d.id,object:d,geometry:f,material:g,materialVariant:o(d),groupOrder:v,renderOrder:d.renderOrder,z:m,group:p},s[e]=M):(M.id=d.id,M.object=d,M.geometry=f,M.material=g,M.materialVariant=o(d),M.groupOrder=v,M.renderOrder=d.renderOrder,M.z=m,M.group=p),e++,M}function l(d,f,g,v,m,p,M){M.reversedDepth===!0&&(m=-m);let T=a(d,f,g,v,m,p);g.transmission>0?n.push(T):g.transparent===!0?i.push(T):t.push(T)}function c(d,f,g,v,m,p){let M=a(d,f,g,v,m,p);g.transmission>0?n.unshift(M):g.transparent===!0?i.unshift(M):t.unshift(M)}function h(d,f){t.length>1&&t.sort(d||Av),n.length>1&&n.sort(f||Ff),i.length>1&&i.sort(f||Ff)}function u(){for(let d=e,f=s.length;d<f;d++){let g=s[d];if(g.id===null)break;g.id=null,g.object=null,g.geometry=null,g.material=null,g.group=null}}return{opaque:t,transmissive:n,transparent:i,init:r,push:l,unshift:c,finish:u,sort:h}}function Rv(){let s=new WeakMap;function e(n,i){let r=s.get(n),o;return r===void 0?(o=new Of,s.set(n,[o])):i>=r.length?(o=new Of,r.push(o)):o=r[i],o}function t(){s=new WeakMap}return{get:e,dispose:t}}function Cv(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"SunLight":case"DirectionalLight":t={direction:new W,color:new Ce};break;case"SpotLight":t={position:new W,direction:new W,color:new Ce,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":t={position:new W,color:new Ce,distance:0,decay:0};break;case"HemisphereLight":t={direction:new W,skyColor:new Ce,groundColor:new Ce};break;case"RectAreaLight":t={color:new Ce,position:new W,halfWidth:new W,halfHeight:new W};break}return s[e.id]=t,t}}}function Pv(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"SunLight":case"DirectionalLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Ue};break;case"SpotLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Ue};break;case"PointLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Ue,shadowCameraNear:1,shadowCameraFar:1e3};break}return s[e.id]=t,t}}}var Iv=0;function Lv(s,e){return(e.castShadow?2:0)-(s.castShadow?2:0)+(e.map?1:0)-(s.map?1:0)}function Dv(s){let e=new Cv,t=Pv(),n={version:0,hash:{sunLength:-1,directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numSunShadows:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],sun:[],sunShadow:[],sunShadowMap:[],sunShadowMatrix:[],sunShadowCascade:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let c=0;c<9;c++)n.probe.push(new W);let i=new W,r=new it,o=new it;function a(c){let h=0,u=0,d=0;for(let w=0;w<9;w++)n.probe[w].set(0,0,0);let f=0,g=0,v=0,m=0,p=0,M=0,T=0,x=0,y=0,S=0,C=0,_=0,I=0,b=0;c.sort(Lv);for(let w=0,H=c.length;w<H;w++){let F=c[w],L=F.color,O=F.intensity,V=F.distance,R=null;if(F.shadow&&F.shadow.map&&(F.shadow.map.texture.format===rs?R=F.shadow.map.texture:R=F.shadow.map.depthTexture||F.shadow.map.texture),F.isAmbientLight)h+=L.r*O,u+=L.g*O,d+=L.b*O;else if(F.isLightProbe){for(let G=0;G<9;G++)n.probe[G].addScaledVector(F.sh.coefficients[G],O);b++}else if(F.isSunLight){let G=e.get(F);if(G.color.copy(F.color).multiplyScalar(F.intensity),F.castShadow){let oe=F.shadow,k=t.get(F);k.shadowIntensity=oe.intensity,k.shadowBias=oe.bias,k.shadowNormalBias=oe.normalBias,k.shadowRadius=oe.radius,k.shadowMapSize.copy(oe.mapSize).multiply(oe.getFrameExtents()),n.sunShadow[g]=k,n.sunShadowMap[g]=R;let re=oe.getViewportCount();for(let Ae=0;Ae<re;Ae++)n.sunShadowMatrix[v+Ae]=oe.getMatrix(Ae),n.sunShadowCascade[v+Ae]=oe._cascadeData[Ae];v+=re,g++}n.sun[f]=G,f++}else if(F.isDirectionalLight){let G=e.get(F);if(G.color.copy(F.color).multiplyScalar(F.intensity),F.castShadow){let oe=F.shadow,k=t.get(F);k.shadowIntensity=oe.intensity,k.shadowBias=oe.bias,k.shadowNormalBias=oe.normalBias,k.shadowRadius=oe.radius,k.shadowMapSize=oe.mapSize,n.directionalShadow[m]=k,n.directionalShadowMap[m]=R,n.directionalShadowMatrix[m]=F.shadow.matrix,y++}n.directional[m]=G,m++}else if(F.isSpotLight){let G=e.get(F);G.position.setFromMatrixPosition(F.matrixWorld),G.color.copy(L).multiplyScalar(O),G.distance=V,G.coneCos=Math.cos(F.angle),G.penumbraCos=Math.cos(F.angle*(1-F.penumbra)),G.decay=F.decay,n.spot[M]=G;let oe=F.shadow;if(F.map&&(n.spotLightMap[_]=F.map,_++,oe.updateMatrices(F),F.castShadow&&I++),n.spotLightMatrix[M]=oe.matrix,F.castShadow){let k=t.get(F);k.shadowIntensity=oe.intensity,k.shadowBias=oe.bias,k.shadowNormalBias=oe.normalBias,k.shadowRadius=oe.radius,k.shadowMapSize=oe.mapSize,n.spotShadow[M]=k,n.spotShadowMap[M]=R,C++}M++}else if(F.isRectAreaLight){let G=e.get(F);G.color.copy(L).multiplyScalar(O),G.halfWidth.set(F.width*.5,0,0),G.halfHeight.set(0,F.height*.5,0),n.rectArea[T]=G,T++}else if(F.isPointLight){let G=e.get(F);if(G.color.copy(F.color).multiplyScalar(F.intensity),G.distance=F.distance,G.decay=F.decay,F.castShadow){let oe=F.shadow,k=t.get(F);k.shadowIntensity=oe.intensity,k.shadowBias=oe.bias,k.shadowNormalBias=oe.normalBias,k.shadowRadius=oe.radius,k.shadowMapSize=oe.mapSize,k.shadowCameraNear=oe.camera.near,k.shadowCameraFar=oe.camera.far,n.pointShadow[p]=k,n.pointShadowMap[p]=R,n.pointShadowMatrix[p]=F.shadow.matrix,S++}n.point[p]=G,p++}else if(F.isHemisphereLight){let G=e.get(F);G.skyColor.copy(F.color).multiplyScalar(O),G.groundColor.copy(F.groundColor).multiplyScalar(O),n.hemi[x]=G,x++}}T>0&&(s.has("OES_texture_float_linear")===!0?(n.rectAreaLTC1=Ve.LTC_FLOAT_1,n.rectAreaLTC2=Ve.LTC_FLOAT_2):(n.rectAreaLTC1=Ve.LTC_HALF_1,n.rectAreaLTC2=Ve.LTC_HALF_2)),n.ambient[0]=h,n.ambient[1]=u,n.ambient[2]=d;let N=n.hash;(N.sunLength!==f||N.directionalLength!==m||N.pointLength!==p||N.spotLength!==M||N.rectAreaLength!==T||N.hemiLength!==x||N.numSunShadows!==g||N.numDirectionalShadows!==y||N.numPointShadows!==S||N.numSpotShadows!==C||N.numSpotMaps!==_||N.numLightProbes!==b)&&(n.sun.length=f,n.directional.length=m,n.spot.length=M,n.rectArea.length=T,n.point.length=p,n.hemi.length=x,n.sunShadow.length=g,n.sunShadowMap.length=g,n.sunShadowMatrix.length=v,n.sunShadowCascade.length=v,n.directionalShadow.length=y,n.directionalShadowMap.length=y,n.directionalShadowMatrix.length=y,n.pointShadow.length=S,n.pointShadowMap.length=S,n.pointShadowMatrix.length=S,n.spotShadow.length=C,n.spotShadowMap.length=C,n.spotLightMatrix.length=C+_-I,n.spotLightMap.length=_,n.numSpotLightShadowsWithMaps=I,n.numLightProbes=b,N.sunLength=f,N.directionalLength=m,N.pointLength=p,N.spotLength=M,N.rectAreaLength=T,N.hemiLength=x,N.numSunShadows=g,N.numDirectionalShadows=y,N.numPointShadows=S,N.numSpotShadows=C,N.numSpotMaps=_,N.numLightProbes=b,n.version=Iv++)}function l(c,h){let u=0,d=0,f=0,g=0,v=0,m=0,p=h.matrixWorldInverse;for(let M=0,T=c.length;M<T;M++){let x=c[M];if(x.isSunLight){let y=n.sun[u];y.direction.setFromMatrixPosition(x.matrixWorld),y.direction.transformDirection(p),u++}else if(x.isDirectionalLight){let y=n.directional[d];y.direction.setFromMatrixPosition(x.matrixWorld),i.setFromMatrixPosition(x.target.matrixWorld),y.direction.sub(i),y.direction.transformDirection(p),d++}else if(x.isSpotLight){let y=n.spot[g];y.position.setFromMatrixPosition(x.matrixWorld),y.position.applyMatrix4(p),y.direction.setFromMatrixPosition(x.matrixWorld),i.setFromMatrixPosition(x.target.matrixWorld),y.direction.sub(i),y.direction.transformDirection(p),g++}else if(x.isRectAreaLight){let y=n.rectArea[v];y.position.setFromMatrixPosition(x.matrixWorld),y.position.applyMatrix4(p),o.identity(),r.copy(x.matrixWorld),r.premultiply(p),o.extractRotation(r),y.halfWidth.set(x.width*.5,0,0),y.halfHeight.set(0,x.height*.5,0),y.halfWidth.applyMatrix4(o),y.halfHeight.applyMatrix4(o),v++}else if(x.isPointLight){let y=n.point[f];y.position.setFromMatrixPosition(x.matrixWorld),y.position.applyMatrix4(p),f++}else if(x.isHemisphereLight){let y=n.hemi[m];y.direction.setFromMatrixPosition(x.matrixWorld),y.direction.transformDirection(p),m++}}}return{setup:a,setupView:l,state:n}}function Bf(s){let e=new Dv(s),t=[],n=[],i=[];function r(d){u.camera=d,t.length=0,n.length=0,i.length=0}function o(d){t.push(d)}function a(d){n.push(d)}function l(d){i.push(d)}function c(){e.setup(t)}function h(d){e.setupView(t,d)}let u={lightsArray:t,shadowsArray:n,lightProbeGridArray:i,camera:null,lights:e,transmissionRenderTarget:{},textureUnits:0};return{init:r,state:u,setupLights:c,setupLightsView:h,pushLight:o,pushShadow:a,pushLightProbeGrid:l}}function Nv(s){let e=new WeakMap;function t(i,r=0){let o=e.get(i),a;return o===void 0?(a=new Bf(s),e.set(i,[a])):r>=o.length?(a=new Bf(s),o.push(a)):a=o[r],a}function n(){e=new WeakMap}return{get:t,dispose:n}}var Uv=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,Fv=`uniform sampler2D shadow_pass;
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
}`,Ov=[new W(1,0,0),new W(-1,0,0),new W(0,1,0),new W(0,-1,0),new W(0,0,1),new W(0,0,-1)],Bv=[new W(0,-1,0),new W(0,-1,0),new W(0,0,1),new W(0,0,-1),new W(0,-1,0),new W(0,-1,0)],zf=new it,qo=new W,zh=new W;function zv(s,e,t){let n=new dr,i=new Ue,r=new Ue,o=new gt,a=new el,l=new tl,c={},h=t.maxTextureSize,u={[Bn]:Kt,[Kt]:Bn,[Rt]:Rt},d=new ct({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new Ue},radius:{value:4}},vertexShader:Uv,fragmentShader:Fv}),f=d.clone();f.defines.HORIZONTAL_PASS=1;let g=new ot;g.setAttribute("position",new dt(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let v=new Ke(g,d),m=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=Co;let p=this.type;this.render=function(S,C,_){if(m.enabled===!1||m.autoUpdate===!1&&m.needsUpdate===!1||S.length===0)return;this.type===dl&&(Ze("WebGLShadowMap: PCFSoftShadowMap has been removed. Using PCFShadowMap instead."),this.type=Co);let I=s.getRenderTarget(),b=s.getActiveCubeFace(),N=s.getActiveMipmapLevel(),w=s.state;w.setBlending(zn),w.buffers.depth.getReversed()===!0?w.buffers.color.setClear(0,0,0,0):w.buffers.color.setClear(1,1,1,1),w.buffers.depth.setTest(!0),w.setScissorTest(!1);let H=p!==this.type;H&&C.traverse(function(F){F.material&&(Array.isArray(F.material)?F.material.forEach(L=>L.needsUpdate=!0):F.material.needsUpdate=!0)});for(let F=0,L=S.length;F<L;F++){let O=S[F],V=O.shadow;if(V===void 0){Ze("WebGLShadowMap:",O,"has no shadow.");continue}if(V.autoUpdate===!1&&V.needsUpdate===!1)continue;i.copy(V.mapSize);let R=V.getFrameExtents();i.multiply(R),r.copy(V.mapSize),(i.x>h||i.y>h)&&(i.x>h&&(r.x=Math.floor(h/R.x),i.x=r.x*R.x,V.mapSize.x=r.x),i.y>h&&(r.y=Math.floor(h/R.y),i.y=r.y*R.y,V.mapSize.y=r.y));let G=s.state.buffers.depth.getReversed();if(V.camera._reversedDepth=G,V.map===null||H===!0){if(V.map!==null&&(V.map.depthTexture!==null&&(V.map.depthTexture.dispose(),V.map.depthTexture=null),V.map.dispose()),this.type===br){if(O.isPointLight){Ze("WebGLShadowMap: VSM shadow maps are not supported for PointLights. Use PCF or BasicShadowMap instead.");continue}V.map=new Bt(i.x,i.y,{format:rs,type:Yt,minFilter:Ot,magFilter:Ot,generateMipmaps:!1}),V.map.texture.name=O.name+".shadowMap",V.map.depthTexture=new Ji(i.x,i.y,Pn),V.map.depthTexture.name=O.name+".shadowMapDepth",V.map.depthTexture.format=li,V.map.depthTexture.compareFunction=null,V.map.depthTexture.minFilter=Vt,V.map.depthTexture.magFilter=Vt}else O.isPointLight?(V.map=new oc(i.x),V.map.depthTexture=new qa(i.x,Qn)):(V.map=new Bt(i.x,i.y),V.map.depthTexture=new Ji(i.x,i.y,Qn)),V.map.depthTexture.name=O.name+".shadowMap",V.map.depthTexture.format=li,this.type===Co?(V.map.depthTexture.compareFunction=G?ic:nc,V.map.depthTexture.minFilter=Ot,V.map.depthTexture.magFilter=Ot):(V.map.depthTexture.compareFunction=null,V.map.depthTexture.minFilter=Vt,V.map.depthTexture.magFilter=Vt);V.camera.updateProjectionMatrix()}V.map.isWebGLCubeRenderTarget!==!0&&(V.map.width!==i.x||V.map.height!==i.y)&&V.map.setSize(i.x,i.y);let oe=V.map.isWebGLCubeRenderTarget?6:V.getViewportCount();O.isPointLight!==!0&&V.updateMatrices(O,_);for(let k=0;k<oe;k++){let re=V.getCamera(k);if(O.isPointLight){let Ae=V.camera,Le=V.matrix,be=O.distance||Ae.far;be!==Ae.far&&(Ae.far=be,Ae.updateProjectionMatrix()),qo.setFromMatrixPosition(O.matrixWorld),Ae.position.copy(qo),zh.copy(Ae.position),zh.add(Ov[k]),Ae.up.copy(Bv[k]),Ae.lookAt(zh),Ae.updateMatrixWorld(),Le.makeTranslation(-qo.x,-qo.y,-qo.z),zf.multiplyMatrices(Ae.projectionMatrix,Ae.matrixWorldInverse),V._frustum.setFromProjectionMatrix(zf,Ae.coordinateSystem,Ae.reversedDepth)}if(V.map.isWebGLCubeRenderTarget)s.setRenderTarget(V.map,k),s.clear();else{k===0&&(s.setRenderTarget(V.map),s.clear());let Ae=V.getViewport(k);o.set(r.x*Ae.x,r.y*Ae.y,r.x*Ae.z,r.y*Ae.w),w.viewport(o)}n=V.getFrustum(k),x(C,_,re,O,this.type)}V.isPointLightShadow!==!0&&this.type===br&&M(V,_),V.needsUpdate=!1}p=this.type,m.needsUpdate=!1,s.setRenderTarget(I,b,N)};function M(S,C){let _=e.update(v);d.defines.VSM_SAMPLES!==S.blurSamples&&(d.defines.VSM_SAMPLES=S.blurSamples,f.defines.VSM_SAMPLES=S.blurSamples,d.needsUpdate=!0,f.needsUpdate=!0),S.mapPass===null?S.mapPass=new Bt(i.x,i.y,{format:rs,type:Yt}):(S.mapPass.width!==S.map.width||S.mapPass.height!==S.map.height)&&S.mapPass.setSize(S.map.width,S.map.height),d.uniforms.shadow_pass.value=S.map.depthTexture,d.uniforms.resolution.value.set(S.map.width,S.map.height),d.uniforms.radius.value=S.radius,s.setRenderTarget(S.mapPass),s.clear(),s.renderBufferDirect(C,null,_,d,v,null),f.uniforms.shadow_pass.value=S.mapPass.texture,f.uniforms.resolution.value.set(S.map.width,S.map.height),f.uniforms.radius.value=S.radius,s.setRenderTarget(S.map),s.clear(),s.renderBufferDirect(C,null,_,f,v,null)}function T(S,C,_,I){let b=null,N=_.isPointLight===!0?S.customDistanceMaterial:S.customDepthMaterial;if(N!==void 0)b=N;else if(b=_.isPointLight===!0?l:a,s.localClippingEnabled&&C.clipShadows===!0&&Array.isArray(C.clippingPlanes)&&C.clippingPlanes.length!==0||C.displacementMap&&C.displacementScale!==0||C.alphaMap&&C.alphaTest>0||C.map&&C.alphaTest>0||C.alphaToCoverage===!0){let w=b.uuid,H=C.uuid,F=c[w];F===void 0&&(F={},c[w]=F);let L=F[H];L===void 0&&(L=b.clone(),F[H]=L,C.addEventListener("dispose",y)),b=L}if(b.visible=C.visible,b.wireframe=C.wireframe,I===br?b.side=C.shadowSide!==null?C.shadowSide:C.side:b.side=C.shadowSide!==null?C.shadowSide:u[C.side],b.alphaMap=C.alphaMap,b.alphaTest=C.alphaToCoverage===!0?.5:C.alphaTest,b.map=C.map,b.clipShadows=C.clipShadows,b.clippingPlanes=C.clippingPlanes,b.clipIntersection=C.clipIntersection,b.displacementMap=C.displacementMap,b.displacementScale=C.displacementScale,b.displacementBias=C.displacementBias,b.wireframeLinewidth=C.wireframeLinewidth,b.linewidth=C.linewidth,_.isPointLight===!0&&b.isMeshDistanceMaterial===!0){let w=s.properties.get(b);w.light=_}return b}function x(S,C,_,I,b){if(S.visible===!1)return;if(S.layers.test(C.layers)&&(S.isMesh||S.isLine||S.isPoints)&&(S.castShadow||S.receiveShadow&&b===br)&&(!S.frustumCulled||S.intersectsFrustum(n))){S.modelViewMatrix.multiplyMatrices(_.matrixWorldInverse,S.matrixWorld);let H=e.update(S),F=S.material;if(Array.isArray(F)){let L=H.groups;for(let O=0,V=L.length;O<V;O++){let R=L[O],G=F[R.materialIndex];if(G&&G.visible){let oe=T(S,G,I,b);S.onBeforeShadow(s,S,C,_,H,oe,R),s.renderBufferDirect(_,null,H,oe,S,R),S.onAfterShadow(s,S,C,_,H,oe,R)}}}else if(F.visible){let L=T(S,F,I,b);S.onBeforeShadow(s,S,C,_,H,L,null),s.renderBufferDirect(_,null,H,L,S,null),S.onAfterShadow(s,S,C,_,H,L,null)}}let w=S.children;for(let H=0,F=w.length;H<F;H++)x(w[H],C,_,I,b)}function y(S){S.target.removeEventListener("dispose",y);for(let _ in c){let I=c[_],b=S.target.uuid;b in I&&(I[b].dispose(),delete I[b])}}}function kv(s,e){function t(){let D=!1,Y=new gt,A=null,z=new gt(0,0,0,0);return{setMask:function(ee){A!==ee&&!D&&(s.colorMask(ee,ee,ee,ee),A=ee)},setLocked:function(ee){D=ee},setClear:function(ee,ne,Te,ye,je){je===!0&&(ee*=ye,ne*=ye,Te*=ye),Y.set(ee,ne,Te,ye),z.equals(Y)===!1&&(s.clearColor(ee,ne,Te,ye),z.copy(Y))},reset:function(){D=!1,A=null,z.set(-1,0,0,0)}}}function n(){let D=!1,Y=!1,A=null,z=null,ee=null;return{setReversed:function(ne){if(Y!==ne){let Te=e.get("EXT_clip_control");ne?Te.clipControlEXT(Te.LOWER_LEFT_EXT,Te.ZERO_TO_ONE_EXT):Te.clipControlEXT(Te.LOWER_LEFT_EXT,Te.NEGATIVE_ONE_TO_ONE_EXT),Y=ne;let ye=ee;ee=null,this.setClear(ye)}},getReversed:function(){return Y},setTest:function(ne){ne?ge(s.DEPTH_TEST):Re(s.DEPTH_TEST)},setMask:function(ne){A!==ne&&!D&&(s.depthMask(ne),A=ne)},setFunc:function(ne){if(Y&&(ne=mf[ne]),z!==ne){switch(ne){case Na:s.depthFunc(s.NEVER);break;case Ua:s.depthFunc(s.ALWAYS);break;case Fa:s.depthFunc(s.LESS);break;case tr:s.depthFunc(s.LEQUAL);break;case Oa:s.depthFunc(s.EQUAL);break;case Ba:s.depthFunc(s.GEQUAL);break;case za:s.depthFunc(s.GREATER);break;case ka:s.depthFunc(s.NOTEQUAL);break;default:s.depthFunc(s.LEQUAL)}z=ne}},setLocked:function(ne){D=ne},setClear:function(ne){ee!==ne&&(ee=ne,Y&&(ne=1-ne),s.clearDepth(ne))},reset:function(){D=!1,A=null,z=null,ee=null,Y=!1}}}function i(){let D=!1,Y=null,A=null,z=null,ee=null,ne=null,Te=null,ye=null,je=null;return{setTest:function(Je){D||(Je?ge(s.STENCIL_TEST):Re(s.STENCIL_TEST))},setMask:function(Je){Y!==Je&&!D&&(s.stencilMask(Je),Y=Je)},setFunc:function(Je,Mt,nt){(A!==Je||z!==Mt||ee!==nt)&&(s.stencilFunc(Je,Mt,nt),A=Je,z=Mt,ee=nt)},setOp:function(Je,Mt,nt){(ne!==Je||Te!==Mt||ye!==nt)&&(s.stencilOp(Je,Mt,nt),ne=Je,Te=Mt,ye=nt)},setLocked:function(Je){D=Je},setClear:function(Je){je!==Je&&(s.clearStencil(Je),je=Je)},reset:function(){D=!1,Y=null,A=null,z=null,ee=null,ne=null,Te=null,ye=null,je=null}}}let r=new t,o=new n,a=new i,l=new WeakMap,c=new WeakMap,h={},u={},d={},f=new WeakMap,g=[],v=null,m=!1,p=null,M=null,T=null,x=null,y=null,S=null,C=null,_=new Ce(0,0,0),I=0,b=!1,N=null,w=null,H=null,F=null,L=null,O=s.getParameter(s.MAX_COMBINED_TEXTURE_IMAGE_UNITS),V=!1,R=0,G=s.getParameter(s.VERSION);G.indexOf("WebGL")!==-1?(R=parseFloat(/^WebGL (\d)/.exec(G)[1]),V=R>=1):G.indexOf("OpenGL ES")!==-1&&(R=parseFloat(/^OpenGL ES (\d)/.exec(G)[1]),V=R>=2);let oe=null,k={},re=s.getParameter(s.SCISSOR_BOX),Ae=s.getParameter(s.VIEWPORT),Le=new gt().fromArray(re),be=new gt().fromArray(Ae);function Fe(D,Y,A,z){let ee=new Uint8Array(4),ne=s.createTexture();s.bindTexture(D,ne),s.texParameteri(D,s.TEXTURE_MIN_FILTER,s.NEAREST),s.texParameteri(D,s.TEXTURE_MAG_FILTER,s.NEAREST);for(let Te=0;Te<A;Te++)D===s.TEXTURE_3D||D===s.TEXTURE_2D_ARRAY?s.texImage3D(Y,0,s.RGBA,1,1,z,0,s.RGBA,s.UNSIGNED_BYTE,ee):s.texImage2D(Y+Te,0,s.RGBA,1,1,0,s.RGBA,s.UNSIGNED_BYTE,ee);return ne}let le={};le[s.TEXTURE_2D]=Fe(s.TEXTURE_2D,s.TEXTURE_2D,1),le[s.TEXTURE_CUBE_MAP]=Fe(s.TEXTURE_CUBE_MAP,s.TEXTURE_CUBE_MAP_POSITIVE_X,6),le[s.TEXTURE_2D_ARRAY]=Fe(s.TEXTURE_2D_ARRAY,s.TEXTURE_2D_ARRAY,1,1),le[s.TEXTURE_3D]=Fe(s.TEXTURE_3D,s.TEXTURE_3D,1,1),r.setClear(0,0,0,1),o.setClear(1),a.setClear(0),ge(s.DEPTH_TEST),o.setFunc(tr),Ee(!1),Ge(fh),ge(s.CULL_FACE),ue(zn);function ge(D){h[D]!==!0&&(s.enable(D),h[D]=!0)}function Re(D){h[D]!==!1&&(s.disable(D),h[D]=!1)}function de(D,Y){return d[D]!==Y?(s.bindFramebuffer(D,Y),d[D]=Y,D===s.DRAW_FRAMEBUFFER&&(d[s.FRAMEBUFFER]=Y),D===s.FRAMEBUFFER&&(d[s.DRAW_FRAMEBUFFER]=Y),!0):!1}function he(D,Y){let A=g,z=!1;if(D){A=f.get(Y),A===void 0&&(A=[],f.set(Y,A));let ee=D.textures;if(A.length!==ee.length||A[0]!==s.COLOR_ATTACHMENT0){for(let ne=0,Te=ee.length;ne<Te;ne++)A[ne]=s.COLOR_ATTACHMENT0+ne;A.length=ee.length,z=!0}}else A[0]!==s.BACK&&(A[0]=s.BACK,z=!0);z&&s.drawBuffers(A)}function ce(D){return v!==D?(s.useProgram(D),v=D,!0):!1}let Se={[Rs]:s.FUNC_ADD,[Dd]:s.FUNC_SUBTRACT,[Nd]:s.FUNC_REVERSE_SUBTRACT};Se[Ud]=s.MIN,Se[Fd]=s.MAX;let fe={[Od]:s.ZERO,[Bd]:s.ONE,[zd]:s.SRC_COLOR,[gh]:s.SRC_ALPHA,[Xd]:s.SRC_ALPHA_SATURATE,[Gd]:s.DST_COLOR,[Hd]:s.DST_ALPHA,[kd]:s.ONE_MINUS_SRC_COLOR,[xh]:s.ONE_MINUS_SRC_ALPHA,[Wd]:s.ONE_MINUS_DST_COLOR,[Vd]:s.ONE_MINUS_DST_ALPHA,[qd]:s.CONSTANT_COLOR,[Yd]:s.ONE_MINUS_CONSTANT_COLOR,[Zd]:s.CONSTANT_ALPHA,[Kd]:s.ONE_MINUS_CONSTANT_ALPHA};function ue(D,Y,A,z,ee,ne,Te,ye,je,Je){if(D===zn){m===!0&&(Re(s.BLEND),m=!1);return}if(m===!1&&(ge(s.BLEND),m=!0),D!==Ld){if(D!==p||Je!==b){if((M!==Rs||y!==Rs)&&(s.blendEquation(s.FUNC_ADD),M=Rs,y=Rs),Je)switch(D){case Sr:s.blendFuncSeparate(s.ONE,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case Ut:s.blendFunc(s.ONE,s.ONE);break;case ph:s.blendFuncSeparate(s.ZERO,s.ONE_MINUS_SRC_COLOR,s.ZERO,s.ONE);break;case mh:s.blendFuncSeparate(s.DST_COLOR,s.ONE_MINUS_SRC_ALPHA,s.ZERO,s.ONE);break;default:et("WebGLState: Invalid blending: ",D);break}else switch(D){case Sr:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case Ut:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE,s.ONE,s.ONE);break;case ph:et("WebGLState: SubtractiveBlending requires material.premultipliedAlpha = true");break;case mh:et("WebGLState: MultiplyBlending requires material.premultipliedAlpha = true");break;default:et("WebGLState: Invalid blending: ",D);break}T=null,x=null,S=null,C=null,_.set(0,0,0),I=0,p=D,b=Je}return}ee=ee||Y,ne=ne||A,Te=Te||z,(Y!==M||ee!==y)&&(s.blendEquationSeparate(Se[Y],Se[ee]),M=Y,y=ee),(A!==T||z!==x||ne!==S||Te!==C)&&(s.blendFuncSeparate(fe[A],fe[z],fe[ne],fe[Te]),T=A,x=z,S=ne,C=Te),(ye.equals(_)===!1||je!==I)&&(s.blendColor(ye.r,ye.g,ye.b,je),_.copy(ye),I=je),p=D,b=!1}function Me(D,Y){D.side===Rt?Re(s.CULL_FACE):ge(s.CULL_FACE);let A=D.side===Kt;Y&&(A=!A),Ee(A),D.blending===Sr&&D.transparent===!1?ue(zn):ue(D.blending,D.blendEquation,D.blendSrc,D.blendDst,D.blendEquationAlpha,D.blendSrcAlpha,D.blendDstAlpha,D.blendColor,D.blendAlpha,D.premultipliedAlpha),o.setFunc(D.depthFunc),o.setTest(D.depthTest),o.setMask(D.depthWrite),r.setMask(D.colorWrite);let z=D.stencilWrite;a.setTest(z),z&&(a.setMask(D.stencilWriteMask),a.setFunc(D.stencilFunc,D.stencilRef,D.stencilFuncMask),a.setOp(D.stencilFail,D.stencilZFail,D.stencilZPass)),ke(D.polygonOffset,D.polygonOffsetFactor,D.polygonOffsetUnits),D.alphaToCoverage===!0?ge(s.SAMPLE_ALPHA_TO_COVERAGE):Re(s.SAMPLE_ALPHA_TO_COVERAGE)}function Ee(D){N!==D&&(D?s.frontFace(s.CW):s.frontFace(s.CCW),N=D)}function Ge(D){D!==Pd?(ge(s.CULL_FACE),D!==w&&(D===fh?s.cullFace(s.BACK):D===Id?s.cullFace(s.FRONT):s.cullFace(s.FRONT_AND_BACK))):Re(s.CULL_FACE),w=D}function qe(D){D!==H&&(V&&s.lineWidth(D),H=D)}function ke(D,Y,A){D?(ge(s.POLYGON_OFFSET_FILL),(F!==Y||L!==A)&&(F=Y,L=A,o.getReversed()&&(Y=-Y),s.polygonOffset(Y,A))):Re(s.POLYGON_OFFSET_FILL)}function Ye(D){D?ge(s.SCISSOR_TEST):Re(s.SCISSOR_TEST)}function Qe(D){D===void 0&&(D=s.TEXTURE0+O-1),oe!==D&&(s.activeTexture(D),oe=D)}function Q(D,Y,A){A===void 0&&(oe===null?A=s.TEXTURE0+O-1:A=oe);let z=k[A];z===void 0&&(z={type:void 0,texture:void 0},k[A]=z),(z.type!==D||z.texture!==Y)&&(oe!==A&&(s.activeTexture(A),oe=A),s.bindTexture(D,Y||le[D]),z.type=D,z.texture=Y)}function st(){let D=k[oe];D!==void 0&&D.type!==void 0&&(s.bindTexture(D.type,null),D.type=void 0,D.texture=void 0)}function $e(){try{s.compressedTexImage2D(...arguments)}catch(D){et("WebGLState:",D)}}function X(){try{s.compressedTexImage3D(...arguments)}catch(D){et("WebGLState:",D)}}function E(){try{s.texSubImage2D(...arguments)}catch(D){et("WebGLState:",D)}}function ae(){try{s.texSubImage3D(...arguments)}catch(D){et("WebGLState:",D)}}function se(){try{s.compressedTexSubImage2D(...arguments)}catch(D){et("WebGLState:",D)}}function _e(){try{s.compressedTexSubImage3D(...arguments)}catch(D){et("WebGLState:",D)}}function K(){try{s.texStorage2D(...arguments)}catch(D){et("WebGLState:",D)}}function te(){try{s.texStorage3D(...arguments)}catch(D){et("WebGLState:",D)}}function J(){try{s.texImage2D(...arguments)}catch(D){et("WebGLState:",D)}}function $(){try{s.texImage3D(...arguments)}catch(D){et("WebGLState:",D)}}function xe(D){return u[D]!==void 0?u[D]:s.getParameter(D)}function ve(D,Y){u[D]!==Y&&(s.pixelStorei(D,Y),u[D]=Y)}function me(D){Le.equals(D)===!1&&(s.scissor(D.x,D.y,D.z,D.w),Le.copy(D))}function pe(D){be.equals(D)===!1&&(s.viewport(D.x,D.y,D.z,D.w),be.copy(D))}function we(D,Y){let A=c.get(Y);A===void 0&&(A=new WeakMap,c.set(Y,A));let z=A.get(D);z===void 0&&(z=s.getUniformBlockIndex(Y,D.name),A.set(D,z))}function B(D,Y){let z=c.get(Y).get(D);l.get(Y)!==z&&(s.uniformBlockBinding(Y,z,D.__bindingPointIndex),l.set(Y,z))}function q(){s.disable(s.BLEND),s.disable(s.CULL_FACE),s.disable(s.DEPTH_TEST),s.disable(s.POLYGON_OFFSET_FILL),s.disable(s.SCISSOR_TEST),s.disable(s.STENCIL_TEST),s.disable(s.SAMPLE_ALPHA_TO_COVERAGE),s.blendEquation(s.FUNC_ADD),s.blendFunc(s.ONE,s.ZERO),s.blendFuncSeparate(s.ONE,s.ZERO,s.ONE,s.ZERO),s.blendColor(0,0,0,0),s.colorMask(!0,!0,!0,!0),s.clearColor(0,0,0,0),s.depthMask(!0),s.depthFunc(s.LESS),o.setReversed(!1),s.clearDepth(1),s.stencilMask(4294967295),s.stencilFunc(s.ALWAYS,0,4294967295),s.stencilOp(s.KEEP,s.KEEP,s.KEEP),s.clearStencil(0),s.cullFace(s.BACK),s.frontFace(s.CCW),s.polygonOffset(0,0),s.activeTexture(s.TEXTURE0),s.bindFramebuffer(s.FRAMEBUFFER,null),s.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),s.bindFramebuffer(s.READ_FRAMEBUFFER,null),s.useProgram(null),s.lineWidth(1),s.scissor(0,0,s.canvas.width,s.canvas.height),s.viewport(0,0,s.canvas.width,s.canvas.height),s.pixelStorei(s.PACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,!1),s.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!1),s.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,s.BROWSER_DEFAULT_WEBGL),s.pixelStorei(s.PACK_ROW_LENGTH,0),s.pixelStorei(s.PACK_SKIP_PIXELS,0),s.pixelStorei(s.PACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_ROW_LENGTH,0),s.pixelStorei(s.UNPACK_IMAGE_HEIGHT,0),s.pixelStorei(s.UNPACK_SKIP_PIXELS,0),s.pixelStorei(s.UNPACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_SKIP_IMAGES,0),h={},u={},oe=null,k={},d={},f=new WeakMap,g=[],v=null,m=!1,p=null,M=null,T=null,x=null,y=null,S=null,C=null,_=new Ce(0,0,0),I=0,b=!1,N=null,w=null,H=null,F=null,L=null,Le.set(0,0,s.canvas.width,s.canvas.height),be.set(0,0,s.canvas.width,s.canvas.height),r.reset(),o.reset(),a.reset()}return{buffers:{color:r,depth:o,stencil:a},enable:ge,disable:Re,bindFramebuffer:de,drawBuffers:he,useProgram:ce,setBlending:ue,setMaterial:Me,setFlipSided:Ee,setCullFace:Ge,setLineWidth:qe,setPolygonOffset:ke,setScissorTest:Ye,activeTexture:Qe,bindTexture:Q,unbindTexture:st,compressedTexImage2D:$e,compressedTexImage3D:X,texImage2D:J,texImage3D:$,pixelStorei:ve,getParameter:xe,updateUBOMapping:we,uniformBlockBinding:B,texStorage2D:K,texStorage3D:te,texSubImage2D:E,texSubImage3D:ae,compressedTexSubImage2D:se,compressedTexSubImage3D:_e,scissor:me,viewport:pe,reset:q}}function Hv(s,e,t,n,i,r,o){let a=e.has("WEBGL_multisampled_render_to_texture")?e.get("WEBGL_multisampled_render_to_texture"):null,l=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),c=new Ue,h=new WeakMap,u=new Set,d,f=new WeakMap,g=!1;try{g=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function v(X,E){return g?new OffscreenCanvas(X,E):sr("canvas")}function m(X,E,ae){let se=1,_e=$e(X);if((_e.width>ae||_e.height>ae)&&(se=ae/Math.max(_e.width,_e.height)),se<1)if(typeof HTMLImageElement<"u"&&X instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&X instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&X instanceof ImageBitmap||typeof VideoFrame<"u"&&X instanceof VideoFrame){let K=Math.floor(se*_e.width),te=Math.floor(se*_e.height);d===void 0&&(d=v(K,te));let J=E?v(K,te):d;return J.width=K,J.height=te,J.getContext("2d").drawImage(X,0,0,K,te),Ze("WebGLRenderer: Texture has been resized from ("+_e.width+"x"+_e.height+") to ("+K+"x"+te+")."),J}else return"data"in X&&Ze("WebGLRenderer: Image in DataTexture is too big ("+_e.width+"x"+_e.height+")."),X;return X}function p(X){return X.generateMipmaps}function M(X){s.generateMipmap(X)}function T(X){return X.isWebGLCubeRenderTarget?s.TEXTURE_CUBE_MAP:X.isWebGL3DRenderTarget?s.TEXTURE_3D:X.isWebGLArrayRenderTarget||X.isCompressedArrayTexture?s.TEXTURE_2D_ARRAY:s.TEXTURE_2D}function x(X,E,ae,se,_e,K=!1){if(X!==null){if(s[X]!==void 0)return s[X];Ze("WebGLRenderer: Attempt to use non-existing WebGL internal format '"+X+"'")}let te;se&&(te=e.get("EXT_texture_norm16"),te||Ze("WebGLRenderer: Unable to use normalized textures without EXT_texture_norm16 extension"));let J=E;if(E===s.RED&&(ae===s.FLOAT&&(J=s.R32F),ae===s.HALF_FLOAT&&(J=s.R16F),ae===s.UNSIGNED_BYTE&&(J=s.R8),ae===s.UNSIGNED_SHORT&&te&&(J=te.R16_EXT),ae===s.SHORT&&te&&(J=te.R16_SNORM_EXT)),E===s.RED_INTEGER&&(ae===s.UNSIGNED_BYTE&&(J=s.R8UI),ae===s.UNSIGNED_SHORT&&(J=s.R16UI),ae===s.UNSIGNED_INT&&(J=s.R32UI),ae===s.BYTE&&(J=s.R8I),ae===s.SHORT&&(J=s.R16I),ae===s.INT&&(J=s.R32I)),E===s.RG&&(ae===s.FLOAT&&(J=s.RG32F),ae===s.HALF_FLOAT&&(J=s.RG16F),ae===s.UNSIGNED_BYTE&&(J=s.RG8),ae===s.UNSIGNED_SHORT&&te&&(J=te.RG16_EXT),ae===s.SHORT&&te&&(J=te.RG16_SNORM_EXT)),E===s.RG_INTEGER&&(ae===s.UNSIGNED_BYTE&&(J=s.RG8UI),ae===s.UNSIGNED_SHORT&&(J=s.RG16UI),ae===s.UNSIGNED_INT&&(J=s.RG32UI),ae===s.BYTE&&(J=s.RG8I),ae===s.SHORT&&(J=s.RG16I),ae===s.INT&&(J=s.RG32I)),E===s.RGB_INTEGER&&(ae===s.UNSIGNED_BYTE&&(J=s.RGB8UI),ae===s.UNSIGNED_SHORT&&(J=s.RGB16UI),ae===s.UNSIGNED_INT&&(J=s.RGB32UI),ae===s.BYTE&&(J=s.RGB8I),ae===s.SHORT&&(J=s.RGB16I),ae===s.INT&&(J=s.RGB32I)),E===s.RGBA_INTEGER&&(ae===s.UNSIGNED_BYTE&&(J=s.RGBA8UI),ae===s.UNSIGNED_SHORT&&(J=s.RGBA16UI),ae===s.UNSIGNED_INT&&(J=s.RGBA32UI),ae===s.BYTE&&(J=s.RGBA8I),ae===s.SHORT&&(J=s.RGBA16I),ae===s.INT&&(J=s.RGBA32I)),E===s.RGB&&(ae===s.UNSIGNED_SHORT&&te&&(J=te.RGB16_EXT),ae===s.SHORT&&te&&(J=te.RGB16_SNORM_EXT),ae===s.UNSIGNED_INT_5_9_9_9_REV&&(J=s.RGB9_E5),ae===s.UNSIGNED_INT_10F_11F_11F_REV&&(J=s.R11F_G11F_B10F)),E===s.RGBA){let $=K?to:ht.getTransfer(_e);ae===s.FLOAT&&(J=s.RGBA32F),ae===s.HALF_FLOAT&&(J=s.RGBA16F),ae===s.UNSIGNED_BYTE&&(J=$===St?s.SRGB8_ALPHA8:s.RGBA8),ae===s.UNSIGNED_SHORT&&te&&(J=te.RGBA16_EXT),ae===s.SHORT&&te&&(J=te.RGBA16_SNORM_EXT),ae===s.UNSIGNED_SHORT_4_4_4_4&&(J=s.RGBA4),ae===s.UNSIGNED_SHORT_5_5_5_1&&(J=s.RGB5_A1)}return(J===s.R16F||J===s.R32F||J===s.RG16F||J===s.RG32F||J===s.RGBA16F||J===s.RGBA32F)&&e.get("EXT_color_buffer_float"),J}function y(X,E){let ae;return X?E===null||E===Qn||E===Er?ae=s.DEPTH24_STENCIL8:E===Pn?ae=s.DEPTH32F_STENCIL8:E===Tr&&(ae=s.DEPTH24_STENCIL8,Ze("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):E===null||E===Qn||E===Er?ae=s.DEPTH_COMPONENT24:E===Pn?ae=s.DEPTH_COMPONENT32F:E===Tr&&(ae=s.DEPTH_COMPONENT16),ae}function S(X,E){return p(X)===!0||X.isFramebufferTexture&&X.minFilter!==Vt&&X.minFilter!==Ot?Math.log2(Math.max(E.width,E.height))+1:X.mipmaps!==void 0&&X.mipmaps.length>0?X.mipmaps.length:X.isCompressedTexture&&Array.isArray(X.image)?E.mipmaps.length:1}function C(X){let E=X.target;E.removeEventListener("dispose",C),I(E),E.isVideoTexture&&h.delete(E),E.isHTMLTexture&&u.delete(E)}function _(X){let E=X.target;E.removeEventListener("dispose",_),N(E)}function I(X){let E=n.get(X);if(E.__webglInit===void 0)return;let ae=X.source,se=f.get(ae);if(se){let _e=se[E.__cacheKey];_e.usedTimes--,_e.usedTimes===0&&b(X),Object.keys(se).length===0&&f.delete(ae)}n.remove(X)}function b(X){let E=n.get(X);s.deleteTexture(E.__webglTexture);let ae=X.source,se=f.get(ae);delete se[E.__cacheKey],o.memory.textures--}function N(X){let E=n.get(X);if(X.depthTexture&&(X.depthTexture.dispose(),n.remove(X.depthTexture)),X.isWebGLCubeRenderTarget)for(let se=0;se<6;se++){if(Array.isArray(E.__webglFramebuffer[se]))for(let _e=0;_e<E.__webglFramebuffer[se].length;_e++)s.deleteFramebuffer(E.__webglFramebuffer[se][_e]);else s.deleteFramebuffer(E.__webglFramebuffer[se]);E.__webglDepthbuffer&&s.deleteRenderbuffer(E.__webglDepthbuffer[se])}else{if(Array.isArray(E.__webglFramebuffer))for(let se=0;se<E.__webglFramebuffer.length;se++)s.deleteFramebuffer(E.__webglFramebuffer[se]);else s.deleteFramebuffer(E.__webglFramebuffer);if(E.__webglDepthbuffer&&s.deleteRenderbuffer(E.__webglDepthbuffer),E.__webglMultisampledFramebuffer&&s.deleteFramebuffer(E.__webglMultisampledFramebuffer),E.__webglColorRenderbuffer)for(let se=0;se<E.__webglColorRenderbuffer.length;se++)E.__webglColorRenderbuffer[se]&&s.deleteRenderbuffer(E.__webglColorRenderbuffer[se]);E.__webglDepthRenderbuffer&&s.deleteRenderbuffer(E.__webglDepthRenderbuffer)}let ae=X.textures;for(let se=0,_e=ae.length;se<_e;se++){let K=n.get(ae[se]);K.__webglTexture&&(s.deleteTexture(K.__webglTexture),o.memory.textures--),n.remove(ae[se])}n.remove(X)}let w=0;function H(){w=0}function F(){return w}function L(X){w=X}function O(){let X=w;return X>=i.maxTextures&&Ze("WebGLTextures: Trying to use "+(X+1)+" texture units while this GPU supports only "+i.maxTextures),w+=1,X}function V(X){let E=[];return E.push(X.wrapS),E.push(X.wrapT),E.push(X.wrapR||0),E.push(X.magFilter),E.push(X.minFilter),E.push(X.anisotropy),E.push(X.internalFormat),E.push(X.format),E.push(X.type),E.push(X.generateMipmaps),E.push(X.premultiplyAlpha),E.push(X.flipY),E.push(X.unpackAlignment),E.push(X.colorSpace),E.join()}function R(X,E){let ae=n.get(X);if(X.isVideoTexture&&Q(X),X.isRenderTargetTexture===!1&&X.isExternalTexture!==!0&&X.version>0&&ae.__version!==X.version){let se=X.image;if(se===null)Ze("WebGLRenderer: Texture marked for update but no image data found.");else if(se.complete===!1)Ze("WebGLRenderer: Texture marked for update but image is incomplete");else{Re(ae,X,E);return}}else X.isExternalTexture&&(ae.__webglTexture=X.sourceTexture?X.sourceTexture:null);t.bindTexture(s.TEXTURE_2D,ae.__webglTexture,s.TEXTURE0+E)}function G(X,E){let ae=n.get(X);if(X.isRenderTargetTexture===!1&&X.version>0&&ae.__version!==X.version){Re(ae,X,E);return}else X.isExternalTexture&&(ae.__webglTexture=X.sourceTexture?X.sourceTexture:null);t.bindTexture(s.TEXTURE_2D_ARRAY,ae.__webglTexture,s.TEXTURE0+E)}function oe(X,E){let ae=n.get(X);if(X.isRenderTargetTexture===!1&&X.version>0&&ae.__version!==X.version){Re(ae,X,E);return}t.bindTexture(s.TEXTURE_3D,ae.__webglTexture,s.TEXTURE0+E)}function k(X,E){let ae=n.get(X);if(X.isCubeDepthTexture!==!0&&X.version>0&&ae.__version!==X.version){de(ae,X,E);return}t.bindTexture(s.TEXTURE_CUBE_MAP,ae.__webglTexture,s.TEXTURE0+E)}let re={[ai]:s.REPEAT,[Fn]:s.CLAMP_TO_EDGE,[nr]:s.MIRRORED_REPEAT},Ae={[Vt]:s.NEAREST,[gl]:s.NEAREST_MIPMAP_NEAREST,[Is]:s.NEAREST_MIPMAP_LINEAR,[Ot]:s.LINEAR,[wr]:s.LINEAR_MIPMAP_NEAREST,[$n]:s.LINEAR_MIPMAP_LINEAR},Le={[of]:s.NEVER,[uf]:s.ALWAYS,[af]:s.LESS,[nc]:s.LEQUAL,[lf]:s.EQUAL,[ic]:s.GEQUAL,[cf]:s.GREATER,[hf]:s.NOTEQUAL};function be(X,E){if(E.type===Pn&&e.has("OES_texture_float_linear")===!1&&(E.magFilter===Ot||E.magFilter===wr||E.magFilter===Is||E.magFilter===$n||E.minFilter===Ot||E.minFilter===wr||E.minFilter===Is||E.minFilter===$n)&&Ze("WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),s.texParameteri(X,s.TEXTURE_WRAP_S,re[E.wrapS]),s.texParameteri(X,s.TEXTURE_WRAP_T,re[E.wrapT]),(X===s.TEXTURE_3D||X===s.TEXTURE_2D_ARRAY)&&s.texParameteri(X,s.TEXTURE_WRAP_R,re[E.wrapR]),s.texParameteri(X,s.TEXTURE_MAG_FILTER,Ae[E.magFilter]),s.texParameteri(X,s.TEXTURE_MIN_FILTER,Ae[E.minFilter]),E.compareFunction&&(s.texParameteri(X,s.TEXTURE_COMPARE_MODE,s.COMPARE_REF_TO_TEXTURE),s.texParameteri(X,s.TEXTURE_COMPARE_FUNC,Le[E.compareFunction])),e.has("EXT_texture_filter_anisotropic")===!0){if(E.magFilter===Vt||E.minFilter!==Is&&E.minFilter!==$n||E.type===Pn&&e.has("OES_texture_float_linear")===!1)return;if(E.anisotropy>1||n.get(E).__currentAnisotropy){let ae=e.get("EXT_texture_filter_anisotropic");s.texParameterf(X,ae.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(E.anisotropy,i.getMaxAnisotropy())),n.get(E).__currentAnisotropy=E.anisotropy}}}function Fe(X,E){let ae=!1;X.__webglInit===void 0&&(X.__webglInit=!0,E.addEventListener("dispose",C));let se=E.source,_e=f.get(se);_e===void 0&&(_e={},f.set(se,_e));let K=V(E);if(K!==X.__cacheKey){_e[K]===void 0&&(_e[K]={texture:s.createTexture(),usedTimes:0},o.memory.textures++,ae=!0),_e[K].usedTimes++;let te=_e[X.__cacheKey];te!==void 0&&(_e[X.__cacheKey].usedTimes--,te.usedTimes===0&&b(E)),X.__cacheKey=K,X.__webglTexture=_e[K].texture}return ae}function le(X,E,ae){return Math.floor(Math.floor(X/ae)/E)}function ge(X,E,ae,se){let K=X.updateRanges;if(K.length===0)t.texSubImage2D(s.TEXTURE_2D,0,0,0,E.width,E.height,ae,se,E.data);else{K.sort((ve,me)=>ve.start-me.start);let te=0;for(let ve=1;ve<K.length;ve++){let me=K[te],pe=K[ve],we=me.start+me.count,B=le(pe.start,E.width,4),q=le(me.start,E.width,4);pe.start<=we+1&&B===q&&le(pe.start+pe.count-1,E.width,4)===B?me.count=Math.max(me.count,pe.start+pe.count-me.start):(++te,K[te]=pe)}K.length=te+1;let J=t.getParameter(s.UNPACK_ROW_LENGTH),$=t.getParameter(s.UNPACK_SKIP_PIXELS),xe=t.getParameter(s.UNPACK_SKIP_ROWS);t.pixelStorei(s.UNPACK_ROW_LENGTH,E.width);for(let ve=0,me=K.length;ve<me;ve++){let pe=K[ve],we=Math.floor(pe.start/4),B=Math.ceil(pe.count/4),q=we%E.width,D=Math.floor(we/E.width),Y=B,A=1;t.pixelStorei(s.UNPACK_SKIP_PIXELS,q),t.pixelStorei(s.UNPACK_SKIP_ROWS,D),t.texSubImage2D(s.TEXTURE_2D,0,q,D,Y,A,ae,se,E.data)}X.clearUpdateRanges(),t.pixelStorei(s.UNPACK_ROW_LENGTH,J),t.pixelStorei(s.UNPACK_SKIP_PIXELS,$),t.pixelStorei(s.UNPACK_SKIP_ROWS,xe)}}function Re(X,E,ae){let se=s.TEXTURE_2D;(E.isDataArrayTexture||E.isCompressedArrayTexture)&&(se=s.TEXTURE_2D_ARRAY),E.isData3DTexture&&(se=s.TEXTURE_3D);let _e=Fe(X,E),K=E.source;t.bindTexture(se,X.__webglTexture,s.TEXTURE0+ae);let te=n.get(K);if(K.version!==te.__version||_e===!0){if(t.activeTexture(s.TEXTURE0+ae),(typeof ImageBitmap<"u"&&E.image instanceof ImageBitmap)===!1){let A=ht.getPrimaries(ht.workingColorSpace),z=E.colorSpace===kn?null:ht.getPrimaries(E.colorSpace),ee=E.colorSpace===kn||A===z?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,E.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,E.premultiplyAlpha),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,ee)}t.pixelStorei(s.UNPACK_ALIGNMENT,E.unpackAlignment);let $=m(E.image,!1,i.maxTextureSize);$=st(E,$);let xe=r.convert(E.format,E.colorSpace),ve=r.convert(E.type),me=x(E.internalFormat,xe,ve,E.normalized,E.colorSpace,E.isVideoTexture);be(se,E);let pe,we=E.mipmaps,B=E.isVideoTexture!==!0,q=te.__version===void 0||_e===!0,D=K.dataReady,Y=S(E,$);if(E.isDepthTexture)me=y(E.format===ss,E.type),q&&(B?t.texStorage2D(s.TEXTURE_2D,1,me,$.width,$.height):t.texImage2D(s.TEXTURE_2D,0,me,$.width,$.height,0,xe,ve,null));else if(E.isDataTexture)if(we.length>0){B&&q&&t.texStorage2D(s.TEXTURE_2D,Y,me,we[0].width,we[0].height);for(let A=0,z=we.length;A<z;A++)pe=we[A],B?D&&t.texSubImage2D(s.TEXTURE_2D,A,0,0,pe.width,pe.height,xe,ve,pe.data):t.texImage2D(s.TEXTURE_2D,A,me,pe.width,pe.height,0,xe,ve,pe.data);E.generateMipmaps=!1}else B?(q&&t.texStorage2D(s.TEXTURE_2D,Y,me,$.width,$.height),D&&ge(E,$,xe,ve)):t.texImage2D(s.TEXTURE_2D,0,me,$.width,$.height,0,xe,ve,$.data);else if(E.isCompressedTexture)if(E.isCompressedArrayTexture){B&&q&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Y,me,we[0].width,we[0].height,$.depth);for(let A=0,z=we.length;A<z;A++)if(pe=we[A],E.format!==In)if(xe!==null)if(B){if(D)if(E.layerUpdates.size>0){let ee=Dh(pe.width,pe.height,E.format,E.type);for(let ne of E.layerUpdates){let Te=pe.data.subarray(ne*ee/pe.data.BYTES_PER_ELEMENT,(ne+1)*ee/pe.data.BYTES_PER_ELEMENT);t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,A,0,0,ne,pe.width,pe.height,1,xe,Te)}}else t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,A,0,0,0,pe.width,pe.height,$.depth,xe,pe.data)}else t.compressedTexImage3D(s.TEXTURE_2D_ARRAY,A,me,pe.width,pe.height,$.depth,0,pe.data,0,0);else Ze("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else B?D&&t.texSubImage3D(s.TEXTURE_2D_ARRAY,A,0,0,0,pe.width,pe.height,$.depth,xe,ve,pe.data):t.texImage3D(s.TEXTURE_2D_ARRAY,A,me,pe.width,pe.height,$.depth,0,xe,ve,pe.data);E.layerUpdates.size>0&&E.clearLayerUpdates()}else{B&&q&&t.texStorage2D(s.TEXTURE_2D,Y,me,we[0].width,we[0].height);for(let A=0,z=we.length;A<z;A++)pe=we[A],E.format!==In?xe!==null?B?D&&t.compressedTexSubImage2D(s.TEXTURE_2D,A,0,0,pe.width,pe.height,xe,pe.data):t.compressedTexImage2D(s.TEXTURE_2D,A,me,pe.width,pe.height,0,pe.data):Ze("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):B?D&&t.texSubImage2D(s.TEXTURE_2D,A,0,0,pe.width,pe.height,xe,ve,pe.data):t.texImage2D(s.TEXTURE_2D,A,me,pe.width,pe.height,0,xe,ve,pe.data)}else if(E.isDataArrayTexture)if(B){if(q&&t.texStorage3D(s.TEXTURE_2D_ARRAY,Y,me,$.width,$.height,$.depth),D)if(E.layerUpdates.size>0){let A=Dh($.width,$.height,E.format,E.type);for(let z of E.layerUpdates){let ee=$.data.subarray(z*A/$.data.BYTES_PER_ELEMENT,(z+1)*A/$.data.BYTES_PER_ELEMENT);t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,z,$.width,$.height,1,xe,ve,ee)}E.clearLayerUpdates()}else t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,0,$.width,$.height,$.depth,xe,ve,$.data)}else t.texImage3D(s.TEXTURE_2D_ARRAY,0,me,$.width,$.height,$.depth,0,xe,ve,$.data);else if(E.isData3DTexture)B?(q&&t.texStorage3D(s.TEXTURE_3D,Y,me,$.width,$.height,$.depth),D&&t.texSubImage3D(s.TEXTURE_3D,0,0,0,0,$.width,$.height,$.depth,xe,ve,$.data)):t.texImage3D(s.TEXTURE_3D,0,me,$.width,$.height,$.depth,0,xe,ve,$.data);else if(E.isFramebufferTexture){if(q)if(B)t.texStorage2D(s.TEXTURE_2D,Y,me,$.width,$.height);else{let A=$.width,z=$.height;for(let ee=0;ee<Y;ee++)t.texImage2D(s.TEXTURE_2D,ee,me,A,z,0,xe,ve,null),A>>=1,z>>=1}}else if(E.isHTMLTexture){if("texElementImage2D"in s){let A=s.canvas;if(A.hasAttribute("layoutsubtree")||A.setAttribute("layoutsubtree","true"),$.parentNode!==A){A.appendChild($),u.add(E),A.onpaint=z=>{let ee=z.changedElements;for(let ne of u)ee.includes(ne.image)&&(ne.needsUpdate=!0)},A.requestPaint();return}if(s.texElementImage2D.length===3)s.texElementImage2D(s.TEXTURE_2D,s.RGBA8,$);else{let ee=s.RGBA,ne=s.RGBA,Te=s.UNSIGNED_BYTE;s.texElementImage2D(s.TEXTURE_2D,0,ee,ne,Te,$)}s.texParameteri(s.TEXTURE_2D,s.TEXTURE_MIN_FILTER,s.LINEAR),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_S,s.CLAMP_TO_EDGE),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_T,s.CLAMP_TO_EDGE)}}else if(we.length>0){if(B&&q){let A=$e(we[0]);t.texStorage2D(s.TEXTURE_2D,Y,me,A.width,A.height)}for(let A=0,z=we.length;A<z;A++)pe=we[A],B?D&&t.texSubImage2D(s.TEXTURE_2D,A,0,0,xe,ve,pe):t.texImage2D(s.TEXTURE_2D,A,me,xe,ve,pe);E.generateMipmaps=!1}else if(B){if(q){let A=$e($);t.texStorage2D(s.TEXTURE_2D,Y,me,A.width,A.height)}D&&t.texSubImage2D(s.TEXTURE_2D,0,0,0,xe,ve,$)}else t.texImage2D(s.TEXTURE_2D,0,me,xe,ve,$);p(E)&&M(se),te.__version=K.version,E.onUpdate&&E.onUpdate(E)}X.__version=E.version}function de(X,E,ae){if(E.image.length!==6)return;let se=Fe(X,E),_e=E.source;t.bindTexture(s.TEXTURE_CUBE_MAP,X.__webglTexture,s.TEXTURE0+ae);let K=n.get(_e);if(_e.version!==K.__version||se===!0){t.activeTexture(s.TEXTURE0+ae);let te=ht.getPrimaries(ht.workingColorSpace),J=E.colorSpace===kn?null:ht.getPrimaries(E.colorSpace),$=E.colorSpace===kn||te===J?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,E.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,E.premultiplyAlpha),t.pixelStorei(s.UNPACK_ALIGNMENT,E.unpackAlignment),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,$);let xe=E.isCompressedTexture||E.image[0].isCompressedTexture,ve=E.image[0]&&E.image[0].isDataTexture,me=[];for(let ne=0;ne<6;ne++)!xe&&!ve?me[ne]=m(E.image[ne],!0,i.maxCubemapSize):me[ne]=ve?E.image[ne].image:E.image[ne],me[ne]=st(E,me[ne]);let pe=me[0],we=r.convert(E.format,E.colorSpace),B=r.convert(E.type),q=x(E.internalFormat,we,B,E.normalized,E.colorSpace),D=E.isVideoTexture!==!0,Y=K.__version===void 0||se===!0,A=_e.dataReady,z=S(E,pe);be(s.TEXTURE_CUBE_MAP,E);let ee;if(xe){D&&Y&&t.texStorage2D(s.TEXTURE_CUBE_MAP,z,q,pe.width,pe.height);for(let ne=0;ne<6;ne++){ee=me[ne].mipmaps;for(let Te=0;Te<ee.length;Te++){let ye=ee[Te];E.format!==In?we!==null?D?A&&t.compressedTexSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,Te,0,0,ye.width,ye.height,we,ye.data):t.compressedTexImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,Te,q,ye.width,ye.height,0,ye.data):Ze("WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):D?A&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,Te,0,0,ye.width,ye.height,we,B,ye.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,Te,q,ye.width,ye.height,0,we,B,ye.data)}}}else{if(ee=E.mipmaps,D&&Y){ee.length>0&&z++;let ne=$e(me[0]);t.texStorage2D(s.TEXTURE_CUBE_MAP,z,q,ne.width,ne.height)}for(let ne=0;ne<6;ne++)if(ve){D?A&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,0,0,0,me[ne].width,me[ne].height,we,B,me[ne].data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,0,q,me[ne].width,me[ne].height,0,we,B,me[ne].data);for(let Te=0;Te<ee.length;Te++){let je=ee[Te].image[ne].image;D?A&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,Te+1,0,0,je.width,je.height,we,B,je.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,Te+1,q,je.width,je.height,0,we,B,je.data)}}else{D?A&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,0,0,0,we,B,me[ne]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,0,q,we,B,me[ne]);for(let Te=0;Te<ee.length;Te++){let ye=ee[Te];D?A&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,Te+1,0,0,we,B,ye.image[ne]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ne,Te+1,q,we,B,ye.image[ne])}}}p(E)&&M(s.TEXTURE_CUBE_MAP),K.__version=_e.version,E.onUpdate&&E.onUpdate(E)}X.__version=E.version}function he(X,E,ae,se,_e,K){let te=r.convert(ae.format,ae.colorSpace),J=r.convert(ae.type),$=x(ae.internalFormat,te,J,ae.normalized,ae.colorSpace),xe=n.get(E),ve=n.get(ae);if(ve.__renderTarget=E,!xe.__hasExternalTextures){let me=Math.max(1,E.width>>K),pe=Math.max(1,E.height>>K);_e===s.TEXTURE_3D||_e===s.TEXTURE_2D_ARRAY?t.texImage3D(_e,K,$,me,pe,E.depth,0,te,J,null):t.texImage2D(_e,K,$,me,pe,0,te,J,null)}t.bindFramebuffer(s.FRAMEBUFFER,X),Qe(E)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,se,_e,ve.__webglTexture,0,Ye(E)):(_e===s.TEXTURE_2D||_e>=s.TEXTURE_CUBE_MAP_POSITIVE_X&&_e<=s.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&s.framebufferTexture2D(s.FRAMEBUFFER,se,_e,ve.__webglTexture,K),t.bindFramebuffer(s.FRAMEBUFFER,null)}function ce(X,E,ae){if(s.bindRenderbuffer(s.RENDERBUFFER,X),E.depthBuffer){let se=E.depthTexture,_e=se&&se.isDepthTexture?se.type:null,K=y(E.stencilBuffer,_e),te=E.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;Qe(E)?a.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,Ye(E),K,E.width,E.height):ae?s.renderbufferStorageMultisample(s.RENDERBUFFER,Ye(E),K,E.width,E.height):s.renderbufferStorage(s.RENDERBUFFER,K,E.width,E.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,te,s.RENDERBUFFER,X)}else{let se=E.textures;for(let _e=0;_e<se.length;_e++){let K=se[_e],te=r.convert(K.format,K.colorSpace),J=r.convert(K.type),$=x(K.internalFormat,te,J,K.normalized,K.colorSpace);Qe(E)?a.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,Ye(E),$,E.width,E.height):ae?s.renderbufferStorageMultisample(s.RENDERBUFFER,Ye(E),$,E.width,E.height):s.renderbufferStorage(s.RENDERBUFFER,$,E.width,E.height)}}s.bindRenderbuffer(s.RENDERBUFFER,null)}function Se(X,E,ae){let se=E.isWebGLCubeRenderTarget===!0;if(t.bindFramebuffer(s.FRAMEBUFFER,X),!(E.depthTexture&&E.depthTexture.isDepthTexture))throw new Error("THREE.WebGLTextures: renderTarget.depthTexture must be an instance of THREE.DepthTexture.");let _e=n.get(E.depthTexture);if(_e.__renderTarget=E,(!_e.__webglTexture||E.depthTexture.image.width!==E.width||E.depthTexture.image.height!==E.height)&&(E.depthTexture.image.width=E.width,E.depthTexture.image.height=E.height,E.depthTexture.needsUpdate=!0),se){if(_e.__webglInit===void 0&&(_e.__webglInit=!0,E.depthTexture.addEventListener("dispose",C)),_e.__webglTexture===void 0){_e.__webglTexture=s.createTexture(),t.bindTexture(s.TEXTURE_CUBE_MAP,_e.__webglTexture),be(s.TEXTURE_CUBE_MAP,E.depthTexture);let xe=r.convert(E.depthTexture.format),ve=r.convert(E.depthTexture.type),me;E.depthTexture.format===li?me=s.DEPTH_COMPONENT24:E.depthTexture.format===ss&&(me=s.DEPTH24_STENCIL8);for(let pe=0;pe<6;pe++)s.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+pe,0,me,E.width,E.height,0,xe,ve,null)}}else R(E.depthTexture,0);let K=_e.__webglTexture,te=Ye(E),J=se?s.TEXTURE_CUBE_MAP_POSITIVE_X+ae:s.TEXTURE_2D,$=E.depthTexture.format===ss?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;if(E.depthTexture.format===li)Qe(E)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,$,J,K,0,te):s.framebufferTexture2D(s.FRAMEBUFFER,$,J,K,0);else if(E.depthTexture.format===ss)Qe(E)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,$,J,K,0,te):s.framebufferTexture2D(s.FRAMEBUFFER,$,J,K,0);else throw new Error("THREE.WebGLTextures: Unknown depthTexture format.")}function fe(X){let E=n.get(X),ae=X.isWebGLCubeRenderTarget===!0;if(E.__boundDepthTexture!==X.depthTexture){let se=X.depthTexture;if(E.__depthDisposeCallback&&E.__depthDisposeCallback(),se){let _e=()=>{delete E.__boundDepthTexture,delete E.__depthDisposeCallback,se.removeEventListener("dispose",_e)};se.addEventListener("dispose",_e),E.__depthDisposeCallback=_e}E.__boundDepthTexture=se}if(X.depthTexture&&!E.__autoAllocateDepthBuffer)if(ae)for(let se=0;se<6;se++)Se(E.__webglFramebuffer[se],X,se);else{let se=X.texture.mipmaps;se&&se.length>0?Se(E.__webglFramebuffer[0],X,0):Se(E.__webglFramebuffer,X,0)}else if(ae){E.__webglDepthbuffer=[];for(let se=0;se<6;se++)if(t.bindFramebuffer(s.FRAMEBUFFER,E.__webglFramebuffer[se]),E.__webglDepthbuffer[se]===void 0)E.__webglDepthbuffer[se]=s.createRenderbuffer(),ce(E.__webglDepthbuffer[se],X,!1);else{let _e=X.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,K=E.__webglDepthbuffer[se];s.bindRenderbuffer(s.RENDERBUFFER,K),s.framebufferRenderbuffer(s.FRAMEBUFFER,_e,s.RENDERBUFFER,K)}}else{let se=X.texture.mipmaps;if(se&&se.length>0?t.bindFramebuffer(s.FRAMEBUFFER,E.__webglFramebuffer[0]):t.bindFramebuffer(s.FRAMEBUFFER,E.__webglFramebuffer),E.__webglDepthbuffer===void 0)E.__webglDepthbuffer=s.createRenderbuffer(),ce(E.__webglDepthbuffer,X,!1);else{let _e=X.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,K=E.__webglDepthbuffer;s.bindRenderbuffer(s.RENDERBUFFER,K),s.framebufferRenderbuffer(s.FRAMEBUFFER,_e,s.RENDERBUFFER,K)}}t.bindFramebuffer(s.FRAMEBUFFER,null)}function ue(X,E,ae){let se=n.get(X);E!==void 0&&he(se.__webglFramebuffer,X,X.texture,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,0),ae!==void 0&&fe(X)}function Me(X){let E=X.texture,ae=n.get(X),se=n.get(E);X.addEventListener("dispose",_);let _e=X.textures,K=X.isWebGLCubeRenderTarget===!0,te=_e.length>1;if(te||(se.__webglTexture===void 0&&(se.__webglTexture=s.createTexture()),se.__version=E.version,o.memory.textures++),K){ae.__webglFramebuffer=[];for(let J=0;J<6;J++)if(E.mipmaps&&E.mipmaps.length>0){ae.__webglFramebuffer[J]=[];for(let $=0;$<E.mipmaps.length;$++)ae.__webglFramebuffer[J][$]=s.createFramebuffer()}else ae.__webglFramebuffer[J]=s.createFramebuffer()}else{if(E.mipmaps&&E.mipmaps.length>0){ae.__webglFramebuffer=[];for(let J=0;J<E.mipmaps.length;J++)ae.__webglFramebuffer[J]=s.createFramebuffer()}else ae.__webglFramebuffer=s.createFramebuffer();if(te)for(let J=0,$=_e.length;J<$;J++){let xe=n.get(_e[J]);xe.__webglTexture===void 0&&(xe.__webglTexture=s.createTexture(),o.memory.textures++)}if(X.samples>0&&Qe(X)===!1){ae.__webglMultisampledFramebuffer=s.createFramebuffer(),ae.__webglColorRenderbuffer=[],t.bindFramebuffer(s.FRAMEBUFFER,ae.__webglMultisampledFramebuffer);for(let J=0;J<_e.length;J++){let $=_e[J];ae.__webglColorRenderbuffer[J]=s.createRenderbuffer(),s.bindRenderbuffer(s.RENDERBUFFER,ae.__webglColorRenderbuffer[J]);let xe=r.convert($.format,$.colorSpace),ve=r.convert($.type),me=x($.internalFormat,xe,ve,$.normalized,$.colorSpace,X.isXRRenderTarget===!0),pe=Ye(X);s.renderbufferStorageMultisample(s.RENDERBUFFER,pe,me,X.width,X.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+J,s.RENDERBUFFER,ae.__webglColorRenderbuffer[J])}s.bindRenderbuffer(s.RENDERBUFFER,null),X.depthBuffer&&(ae.__webglDepthRenderbuffer=s.createRenderbuffer(),ce(ae.__webglDepthRenderbuffer,X,!0)),t.bindFramebuffer(s.FRAMEBUFFER,null)}}if(K){t.bindTexture(s.TEXTURE_CUBE_MAP,se.__webglTexture),be(s.TEXTURE_CUBE_MAP,E);for(let J=0;J<6;J++)if(E.mipmaps&&E.mipmaps.length>0)for(let $=0;$<E.mipmaps.length;$++)he(ae.__webglFramebuffer[J][$],X,E,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+J,$);else he(ae.__webglFramebuffer[J],X,E,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+J,0);p(E)&&M(s.TEXTURE_CUBE_MAP),t.unbindTexture()}else if(te){for(let J=0,$=_e.length;J<$;J++){let xe=_e[J],ve=n.get(xe),me=s.TEXTURE_2D;(X.isWebGL3DRenderTarget||X.isWebGLArrayRenderTarget)&&(me=X.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(me,ve.__webglTexture),be(me,xe),he(ae.__webglFramebuffer,X,xe,s.COLOR_ATTACHMENT0+J,me,0),p(xe)&&M(me)}t.unbindTexture()}else{let J=s.TEXTURE_2D;if((X.isWebGL3DRenderTarget||X.isWebGLArrayRenderTarget)&&(J=X.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(J,se.__webglTexture),be(J,E),E.mipmaps&&E.mipmaps.length>0)for(let $=0;$<E.mipmaps.length;$++)he(ae.__webglFramebuffer[$],X,E,s.COLOR_ATTACHMENT0,J,$);else he(ae.__webglFramebuffer,X,E,s.COLOR_ATTACHMENT0,J,0);p(E)&&M(J),t.unbindTexture()}X.depthBuffer&&fe(X)}function Ee(X){let E=X.textures;for(let ae=0,se=E.length;ae<se;ae++){let _e=E[ae];if(p(_e)){let K=T(X),te=n.get(_e).__webglTexture;t.bindTexture(K,te),M(K),t.unbindTexture()}}}let Ge=[],qe=[];function ke(X){if(X.samples>0){if(Qe(X)===!1){let E=X.textures,ae=X.width,se=X.height,_e=s.COLOR_BUFFER_BIT,K=X.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,te=n.get(X),J=E.length>1;if(J)for(let xe=0;xe<E.length;xe++)t.bindFramebuffer(s.FRAMEBUFFER,te.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+xe,s.RENDERBUFFER,null),t.bindFramebuffer(s.FRAMEBUFFER,te.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+xe,s.TEXTURE_2D,null,0);t.bindFramebuffer(s.READ_FRAMEBUFFER,te.__webglMultisampledFramebuffer);let $=X.texture.mipmaps;$&&$.length>0?t.bindFramebuffer(s.DRAW_FRAMEBUFFER,te.__webglFramebuffer[0]):t.bindFramebuffer(s.DRAW_FRAMEBUFFER,te.__webglFramebuffer);for(let xe=0;xe<E.length;xe++){if(X.resolveDepthBuffer&&(X.depthBuffer&&(_e|=s.DEPTH_BUFFER_BIT),X.stencilBuffer&&X.resolveStencilBuffer&&(_e|=s.STENCIL_BUFFER_BIT)),J){s.framebufferRenderbuffer(s.READ_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.RENDERBUFFER,te.__webglColorRenderbuffer[xe]);let ve=n.get(E[xe]).__webglTexture;s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,ve,0)}s.blitFramebuffer(0,0,ae,se,0,0,ae,se,_e,s.NEAREST),l===!0&&(Ge.length=0,qe.length=0,Ge.push(s.COLOR_ATTACHMENT0+xe),X.depthBuffer&&X.storeMultisampledDepthBuffer===!1&&(Ge.push(K),qe.push(K),s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,qe)),s.invalidateFramebuffer(s.READ_FRAMEBUFFER,Ge))}if(t.bindFramebuffer(s.READ_FRAMEBUFFER,null),t.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),J)for(let xe=0;xe<E.length;xe++){t.bindFramebuffer(s.FRAMEBUFFER,te.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+xe,s.RENDERBUFFER,te.__webglColorRenderbuffer[xe]);let ve=n.get(E[xe]).__webglTexture;t.bindFramebuffer(s.FRAMEBUFFER,te.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+xe,s.TEXTURE_2D,ve,0)}t.bindFramebuffer(s.DRAW_FRAMEBUFFER,te.__webglMultisampledFramebuffer)}else if(X.depthBuffer&&X.storeMultisampledDepthBuffer===!1&&l){let E=X.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,[E])}}}function Ye(X){return Math.min(i.maxSamples,X.samples)}function Qe(X){let E=n.get(X);return X.samples>0&&e.has("WEBGL_multisampled_render_to_texture")===!0&&E.__useRenderToTexture!==!1}function Q(X){let E=o.render.frame;h.get(X)!==E&&(h.set(X,E),X.update())}function st(X,E){let ae=X.colorSpace,se=X.format,_e=X.type;return X.isCompressedTexture===!0||X.isVideoTexture===!0||ae!==mn&&ae!==kn&&(ht.getTransfer(ae)===St?(se!==In||_e!==bn)&&Ze("WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):et("WebGLTextures: Unsupported texture color space:",ae)),E}function $e(X){return typeof HTMLImageElement<"u"&&X instanceof HTMLImageElement?(c.width=X.naturalWidth||X.width,c.height=X.naturalHeight||X.height):typeof VideoFrame<"u"&&X instanceof VideoFrame?(c.width=X.displayWidth,c.height=X.displayHeight):(c.width=X.width,c.height=X.height),c}this.allocateTextureUnit=O,this.resetTextureUnits=H,this.getTextureUnits=F,this.setTextureUnits=L,this.setTexture2D=R,this.setTexture2DArray=G,this.setTexture3D=oe,this.setTextureCube=k,this.rebindTextures=ue,this.setupRenderTarget=Me,this.updateRenderTargetMipmap=Ee,this.updateMultisampleRenderTarget=ke,this.setupDepthRenderbuffer=fe,this.setupFrameBufferTexture=he,this.useMultisampledRTT=Qe,this.isReversedDepthBuffer=function(){return t.buffers.depth.getReversed()}}function Vv(s,e){function t(n,i=kn){let r,o=ht.getTransfer(i);if(n===bn)return s.UNSIGNED_BYTE;if(n===_l)return s.UNSIGNED_SHORT_4_4_4_4;if(n===vl)return s.UNSIGNED_SHORT_5_5_5_1;if(n===Mh)return s.UNSIGNED_INT_5_9_9_9_REV;if(n===bh)return s.UNSIGNED_INT_10F_11F_11F_REV;if(n===vh)return s.BYTE;if(n===yh)return s.SHORT;if(n===Tr)return s.UNSIGNED_SHORT;if(n===xl)return s.INT;if(n===Qn)return s.UNSIGNED_INT;if(n===Pn)return s.FLOAT;if(n===Yt)return s.HALF_FLOAT;if(n===Sh)return s.ALPHA;if(n===wh)return s.RGB;if(n===In)return s.RGBA;if(n===li)return s.DEPTH_COMPONENT;if(n===ss)return s.DEPTH_STENCIL;if(n===yl)return s.RED;if(n===Ml)return s.RED_INTEGER;if(n===rs)return s.RG;if(n===bl)return s.RG_INTEGER;if(n===Sl)return s.RGBA_INTEGER;if(n===Oo||n===Bo||n===zo||n===ko)if(o===St)if(r=e.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(n===Oo)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(n===Bo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(n===zo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(n===ko)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=e.get("WEBGL_compressed_texture_s3tc"),r!==null){if(n===Oo)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(n===Bo)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(n===zo)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(n===ko)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(n===wl||n===Tl||n===El||n===Al)if(r=e.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(n===wl)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(n===Tl)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(n===El)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(n===Al)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(n===Rl||n===Cl||n===Pl||n===Il||n===Ll||n===Ho||n===Dl)if(r=e.get("WEBGL_compressed_texture_etc"),r!==null){if(n===Rl||n===Cl)return o===St?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(n===Pl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC;if(n===Il)return r.COMPRESSED_R11_EAC;if(n===Ll)return r.COMPRESSED_SIGNED_R11_EAC;if(n===Ho)return r.COMPRESSED_RG11_EAC;if(n===Dl)return r.COMPRESSED_SIGNED_RG11_EAC}else return null;if(n===Nl||n===Ul||n===Fl||n===Ol||n===Bl||n===zl||n===kl||n===Hl||n===Vl||n===Gl||n===Wl||n===Xl||n===ql||n===Yl)if(r=e.get("WEBGL_compressed_texture_astc"),r!==null){if(n===Nl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(n===Ul)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(n===Fl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(n===Ol)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(n===Bl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(n===zl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(n===kl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(n===Hl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(n===Vl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(n===Gl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(n===Wl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(n===Xl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(n===ql)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(n===Yl)return o===St?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(n===Zl||n===Kl||n===jl)if(r=e.get("EXT_texture_compression_bptc"),r!==null){if(n===Zl)return o===St?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(n===Kl)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(n===jl)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(n===Jl||n===$l||n===Vo||n===Ql)if(r=e.get("EXT_texture_compression_rgtc"),r!==null){if(n===Jl)return r.COMPRESSED_RED_RGTC1_EXT;if(n===$l)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(n===Vo)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(n===Ql)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return n===Er?s.UNSIGNED_INT_24_8:s[n]!==void 0?s[n]:null}return{convert:t}}var Gv=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,Wv=`
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

}`,Yh=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(e,t){if(this.texture===null){let n=new uo(e.texture);(e.depthNear!==t.depthNear||e.depthFar!==t.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=n}}getMesh(e){if(this.texture!==null&&this.mesh===null){let t=e.cameras[0].viewport,n=new ct({vertexShader:Gv,fragmentShader:Wv,uniforms:{depthColor:{value:this.texture},depthWidth:{value:t.z},depthHeight:{value:t.w}}});this.mesh=new Ke(new nn(20,20),n)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},Zh=class extends On{constructor(e,t){super();let n=this,i=null,r=1,o=null,a="local-floor",l=1,c=null,h=null,u=null,d=null,f=null,g=null,v=typeof XRWebGLBinding<"u",m=new Yh,p={},M=t.getContextAttributes(),T=null,x=null,y=[],S=[],C=new Ue,_=null,I=null,b=new qt;b.viewport=new gt;let N=new qt;N.viewport=new gt;let w=[b,N],H=new cl,F=null,L=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(le){let ge=y[le];return ge===void 0&&(ge=new lr,y[le]=ge),ge.getTargetRaySpace()},this.getControllerGrip=function(le){let ge=y[le];return ge===void 0&&(ge=new lr,y[le]=ge),ge.getGripSpace()},this.getHand=function(le){let ge=y[le];return ge===void 0&&(ge=new lr,y[le]=ge),ge.getHandSpace()};function O(le){let ge=S.indexOf(le.inputSource);if(ge===-1)return;let Re=y[ge];Re!==void 0&&(Re.update(le.inputSource,le.frame,c||o),Re.dispatchEvent({type:le.type,data:le.inputSource}))}function V(){i.removeEventListener("select",O),i.removeEventListener("selectstart",O),i.removeEventListener("selectend",O),i.removeEventListener("squeeze",O),i.removeEventListener("squeezestart",O),i.removeEventListener("squeezeend",O),i.removeEventListener("end",V),i.removeEventListener("inputsourceschange",R);for(let le=0;le<y.length;le++){let ge=S[le];ge!==null&&(S[le]=null,y[le].disconnect(ge))}F=null,L=null,m.reset();for(let le in p)delete p[le];if(e.setRenderTarget(T),f=null,d=null,u=null,i=null,x=null,Fe.stop(),n.isPresenting=!1,e.setPixelRatio(_),e.setSize(C.width,C.height,!1),I!==null){let le=I.camera;le.fov=I.fov,le.zoom=I.zoom,le.updateProjectionMatrix(),I=null}n.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(le){r=le,n.isPresenting===!0&&Ze("WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(le){a=le,n.isPresenting===!0&&Ze("WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return c||o},this.setReferenceSpace=function(le){c=le},this.getBaseLayer=function(){return d!==null?d:f},this.getBinding=function(){return u===null&&v&&(u=new XRWebGLBinding(i,t)),u},this.getFrame=function(){return g},this.getSession=function(){return i},this.setSession=async function(le){if(i=le,i!==null){if(T=e.getRenderTarget(),i.addEventListener("select",O),i.addEventListener("selectstart",O),i.addEventListener("selectend",O),i.addEventListener("squeeze",O),i.addEventListener("squeezestart",O),i.addEventListener("squeezeend",O),i.addEventListener("end",V),i.addEventListener("inputsourceschange",R),M.xrCompatible!==!0&&await t.makeXRCompatible(),_=e.getPixelRatio(),e.getSize(C),v&&"createProjectionLayer"in XRWebGLBinding.prototype){let Re=null,de=null,he=null;M.depth&&(he=M.stencil?t.DEPTH24_STENCIL8:t.DEPTH_COMPONENT24,Re=M.stencil?ss:li,de=M.stencil?Er:Qn);let ce={colorFormat:t.RGBA8,depthFormat:he,scaleFactor:r};u=this.getBinding(),d=u.createProjectionLayer(ce),i.updateRenderState({layers:[d]}),e.setPixelRatio(1),e.setSize(d.textureWidth,d.textureHeight,!1),x=new Bt(d.textureWidth,d.textureHeight,{format:In,type:bn,depthTexture:new Ji(d.textureWidth,d.textureHeight,de,void 0,void 0,void 0,void 0,void 0,void 0,Re),stencilBuffer:M.stencil,colorSpace:e.outputColorSpace,samples:M.antialias?4:0,resolveDepthBuffer:d.ignoreDepthValues===!1,resolveStencilBuffer:d.ignoreDepthValues===!1,storeMultisampledDepthBuffer:d.ignoreDepthValues===!1,storeMultisampledStencilBuffer:d.ignoreDepthValues===!1})}else{let Re={antialias:M.antialias,alpha:!0,depth:M.depth,stencil:M.stencil,framebufferScaleFactor:r};f=new XRWebGLLayer(i,t,Re),i.updateRenderState({baseLayer:f}),e.setPixelRatio(1),e.setSize(f.framebufferWidth,f.framebufferHeight,!1),x=new Bt(f.framebufferWidth,f.framebufferHeight,{format:In,type:bn,colorSpace:e.outputColorSpace,stencilBuffer:M.stencil,resolveDepthBuffer:f.ignoreDepthValues===!1,resolveStencilBuffer:f.ignoreDepthValues===!1,storeMultisampledDepthBuffer:f.ignoreDepthValues===!1,storeMultisampledStencilBuffer:f.ignoreDepthValues===!1})}x.isXRRenderTarget=!0,this.setFoveation(l),c=null,o=await i.requestReferenceSpace(a),Fe.setContext(i),Fe.start(),n.isPresenting=!0,n.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(i!==null)return i.environmentBlendMode},this.getDepthTexture=function(){return m.getDepthTexture()};function R(le){for(let ge=0;ge<le.removed.length;ge++){let Re=le.removed[ge],de=S.indexOf(Re);de>=0&&(S[de]=null,y[de].disconnect(Re))}for(let ge=0;ge<le.added.length;ge++){let Re=le.added[ge],de=S.indexOf(Re);if(de===-1){for(let ce=0;ce<y.length;ce++)if(ce>=S.length){S.push(Re),de=ce;break}else if(S[ce]===null){S[ce]=Re,de=ce;break}if(de===-1)break}let he=y[de];he&&he.connect(Re)}}let G=new W,oe=new W;function k(le,ge,Re){G.setFromMatrixPosition(ge.matrixWorld),oe.setFromMatrixPosition(Re.matrixWorld);let de=G.distanceTo(oe),he=ge.projectionMatrix.elements,ce=Re.projectionMatrix.elements,Se=he[14]/(he[10]-1),fe=he[14]/(he[10]+1),ue=(he[9]+1)/he[5],Me=(he[9]-1)/he[5],Ee=(he[8]-1)/he[0],Ge=(ce[8]+1)/ce[0],qe=Se*Ee,ke=Se*Ge,Ye=de/(-Ee+Ge),Qe=Ye*-Ee;if(ge.matrixWorld.decompose(le.position,le.quaternion,le.scale),le.translateX(Qe),le.translateZ(Ye),le.matrixWorld.compose(le.position,le.quaternion,le.scale),le.matrixWorldInverse.copy(le.matrixWorld).invert(),he[10]===-1)le.projectionMatrix.copy(ge.projectionMatrix),le.projectionMatrixInverse.copy(ge.projectionMatrixInverse);else{let Q=Se+Ye,st=fe+Ye,$e=qe-Qe,X=ke+(de-Qe),E=ue*fe/st*Q,ae=Me*fe/st*Q;le.projectionMatrix.makePerspective($e,X,E,ae,Q,st),le.projectionMatrixInverse.copy(le.projectionMatrix).invert()}}function re(le,ge){ge===null?le.matrixWorld.copy(le.matrix):le.matrixWorld.multiplyMatrices(ge.matrixWorld,le.matrix),le.matrixWorldInverse.copy(le.matrixWorld).invert()}this.updateCamera=function(le){if(i===null)return;let ge=le.near,Re=le.far;m.texture!==null&&(m.depthNear>0&&(ge=m.depthNear),m.depthFar>0&&(Re=m.depthFar)),H.near=N.near=b.near=ge,H.far=N.far=b.far=Re,(F!==H.near||L!==H.far)&&(i.updateRenderState({depthNear:H.near,depthFar:H.far}),F=H.near,L=H.far),H.layers.mask=le.layers.mask|6,b.layers.mask=H.layers.mask&-5,N.layers.mask=H.layers.mask&-3;let de=le.parent,he=H.cameras;re(H,de);for(let ce=0;ce<he.length;ce++)re(he[ce],de);he.length===2?k(H,b,N):H.projectionMatrix.copy(b.projectionMatrix),I===null&&le.isPerspectiveCamera&&(I={camera:le,fov:le.fov,zoom:le.zoom}),Ae(le,H,de)};function Ae(le,ge,Re){Re===null?le.matrix.copy(ge.matrixWorld):(le.matrix.copy(Re.matrixWorld),le.matrix.invert(),le.matrix.multiply(ge.matrixWorld)),le.matrix.decompose(le.position,le.quaternion,le.scale),le.updateMatrixWorld(!0),le.projectionMatrix.copy(ge.projectionMatrix),le.projectionMatrixInverse.copy(ge.projectionMatrixInverse),le.isPerspectiveCamera&&(le.fov=Ms*2*Math.atan(1/le.projectionMatrix.elements[5]),le.zoom=1)}this.getCamera=function(){return H},this.getFoveation=function(){if(!(d===null&&f===null))return l},this.setFoveation=function(le){l=le,d!==null&&(d.fixedFoveation=le),f!==null&&f.fixedFoveation!==void 0&&(f.fixedFoveation=le)},this.hasDepthSensing=function(){return m.texture!==null},this.getDepthSensingMesh=function(){return m.getMesh(H)},this.getCameraTexture=function(le){return p[le]};let Le=null;function be(le,ge){if(h=ge.getViewerPose(c||o),g=ge,h!==null){let Re=h.views;f!==null&&(e.setRenderTargetFramebuffer(x,f.framebuffer),e.setRenderTarget(x));let de=!1;Re.length!==H.cameras.length&&(H.cameras.length=0,de=!0);for(let fe=0;fe<Re.length;fe++){let ue=Re[fe],Me=null;if(f!==null)Me=f.getViewport(ue);else{let Ge=u.getViewSubImage(d,ue);Me=Ge.viewport,fe===0&&(e.setRenderTargetTextures(x,Ge.colorTexture,Ge.depthStencilTexture),e.setRenderTarget(x))}let Ee=w[fe];Ee===void 0&&(Ee=new qt,Ee.layers.enable(fe),Ee.viewport=new gt,w[fe]=Ee),Ee.matrix.fromArray(ue.transform.matrix),Ee.matrix.decompose(Ee.position,Ee.quaternion,Ee.scale),Ee.projectionMatrix.fromArray(ue.projectionMatrix),Ee.projectionMatrixInverse.copy(Ee.projectionMatrix).invert(),Ee.viewport.set(Me.x,Me.y,Me.width,Me.height),fe===0&&(H.matrix.copy(Ee.matrix),H.matrix.decompose(H.position,H.quaternion,H.scale)),de===!0&&H.cameras.push(Ee)}let he=i.enabledFeatures;if(he&&he.includes("depth-sensing")&&i.depthUsage=="gpu-optimized"&&v){u=n.getBinding();let fe=u.getDepthInformation(Re[0]);fe&&fe.isValid&&fe.texture&&m.init(fe,i.renderState)}if(he&&he.includes("camera-access")&&v){e.state.unbindTexture(),u=n.getBinding();for(let fe=0;fe<Re.length;fe++){let ue=Re[fe].camera;if(ue){let Me=p[ue];Me||(Me=new uo,p[ue]=Me);let Ee=u.getCameraImage(ue);Me.sourceTexture=Ee}}}}for(let Re=0;Re<y.length;Re++){let de=S[Re],he=y[Re];de!==null&&he!==void 0&&he.update(de,ge,c||o)}Le&&Le(le,ge),ge.detectedPlanes&&n.dispatchEvent({type:"planesdetected",data:ge}),g=null}let Fe=new kf;Fe.setAnimationLoop(be),this.setAnimationLoop=function(le){Le=le},this.dispose=function(){}}},Xv=new it,qf=new rt;qf.set(-1,0,0,0,1,0,0,0,1);function qv(s,e){function t(m,p){m.matrixAutoUpdate===!0&&m.updateMatrix(),p.value.copy(m.matrix)}function n(m,p){p.color.getRGB(m.fogColor.value,Ph(s)),p.isFog?(m.fogNear.value=p.near,m.fogFar.value=p.far):p.isFogExp2&&(m.fogDensity.value=p.density)}function i(m,p,M,T,x){p.isNodeMaterial?p.uniformsNeedUpdate=!1:p.isMeshBasicMaterial?r(m,p):p.isMeshLambertMaterial?(r(m,p),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)):p.isMeshToonMaterial?(r(m,p),u(m,p)):p.isMeshPhongMaterial?(r(m,p),h(m,p),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)):p.isMeshStandardMaterial?(r(m,p),d(m,p),p.isMeshPhysicalMaterial&&f(m,p,x)):p.isMeshMatcapMaterial?(r(m,p),g(m,p)):p.isMeshDepthMaterial?r(m,p):p.isMeshDistanceMaterial?(r(m,p),v(m,p)):p.isMeshNormalMaterial?r(m,p):p.isLineBasicMaterial?(o(m,p),p.isLineDashedMaterial&&a(m,p)):p.isPointsMaterial?l(m,p,M,T):p.isSpriteMaterial?c(m,p):p.isShadowMaterial?(m.color.value.copy(p.color),m.opacity.value=p.opacity):p.isShaderMaterial&&(p.uniformsNeedUpdate=!1)}function r(m,p){m.opacity.value=p.opacity,p.color&&m.diffuse.value.copy(p.color),p.emissive&&m.emissive.value.copy(p.emissive).multiplyScalar(p.emissiveIntensity),p.map&&(m.map.value=p.map,t(p.map,m.mapTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.bumpMap&&(m.bumpMap.value=p.bumpMap,t(p.bumpMap,m.bumpMapTransform),m.bumpScale.value=p.bumpScale,p.side===Kt&&(m.bumpScale.value*=-1)),p.normalMap&&(m.normalMap.value=p.normalMap,t(p.normalMap,m.normalMapTransform),m.normalScale.value.copy(p.normalScale),p.side===Kt&&m.normalScale.value.negate()),p.displacementMap&&(m.displacementMap.value=p.displacementMap,t(p.displacementMap,m.displacementMapTransform),m.displacementScale.value=p.displacementScale,m.displacementBias.value=p.displacementBias),p.emissiveMap&&(m.emissiveMap.value=p.emissiveMap,t(p.emissiveMap,m.emissiveMapTransform)),p.specularMap&&(m.specularMap.value=p.specularMap,t(p.specularMap,m.specularMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest);let M=e.get(p),T=M.envMap,x=M.envMapRotation;T&&(m.envMap.value=T,m.envMapRotation.value.setFromMatrix4(Xv.makeRotationFromEuler(x)).transpose(),T.isCubeTexture&&T.isRenderTargetTexture===!1&&m.envMapRotation.value.premultiply(qf),m.reflectivity.value=p.reflectivity,m.ior.value=p.ior,m.refractionRatio.value=p.refractionRatio),p.lightMap&&(m.lightMap.value=p.lightMap,m.lightMapIntensity.value=p.lightMapIntensity,t(p.lightMap,m.lightMapTransform)),p.aoMap&&(m.aoMap.value=p.aoMap,m.aoMapIntensity.value=p.aoMapIntensity,t(p.aoMap,m.aoMapTransform))}function o(m,p){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,p.map&&(m.map.value=p.map,t(p.map,m.mapTransform))}function a(m,p){m.dashSize.value=p.dashSize,m.totalSize.value=p.dashSize+p.gapSize,m.scale.value=p.scale}function l(m,p,M,T){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,m.size.value=p.size*M,m.scale.value=T*.5,p.map&&(m.map.value=p.map,t(p.map,m.uvTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest)}function c(m,p){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,m.rotation.value=p.rotation,p.map&&(m.map.value=p.map,t(p.map,m.mapTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest)}function h(m,p){m.specular.value.copy(p.specular),m.shininess.value=Math.max(p.shininess,1e-4)}function u(m,p){p.gradientMap&&(m.gradientMap.value=p.gradientMap)}function d(m,p){m.metalness.value=p.metalness,p.metalnessMap&&(m.metalnessMap.value=p.metalnessMap,t(p.metalnessMap,m.metalnessMapTransform)),m.roughness.value=p.roughness,p.roughnessMap&&(m.roughnessMap.value=p.roughnessMap,t(p.roughnessMap,m.roughnessMapTransform)),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)}function f(m,p,M){m.ior.value=p.ior,p.sheen>0&&(m.sheenColor.value.copy(p.sheenColor).multiplyScalar(p.sheen),m.sheenRoughness.value=p.sheenRoughness,p.sheenColorMap&&(m.sheenColorMap.value=p.sheenColorMap,t(p.sheenColorMap,m.sheenColorMapTransform)),p.sheenRoughnessMap&&(m.sheenRoughnessMap.value=p.sheenRoughnessMap,t(p.sheenRoughnessMap,m.sheenRoughnessMapTransform))),p.clearcoat>0&&(m.clearcoat.value=p.clearcoat,m.clearcoatRoughness.value=p.clearcoatRoughness,p.clearcoatMap&&(m.clearcoatMap.value=p.clearcoatMap,t(p.clearcoatMap,m.clearcoatMapTransform)),p.clearcoatRoughnessMap&&(m.clearcoatRoughnessMap.value=p.clearcoatRoughnessMap,t(p.clearcoatRoughnessMap,m.clearcoatRoughnessMapTransform)),p.clearcoatNormalMap&&(m.clearcoatNormalMap.value=p.clearcoatNormalMap,t(p.clearcoatNormalMap,m.clearcoatNormalMapTransform),m.clearcoatNormalScale.value.copy(p.clearcoatNormalScale),p.side===Kt&&m.clearcoatNormalScale.value.negate())),p.dispersion>0&&(m.dispersion.value=p.dispersion),p.retroreflectivity>0&&(m.retroreflectivity.value=p.retroreflectivity),p.iridescence>0&&(m.iridescence.value=p.iridescence,m.iridescenceIOR.value=p.iridescenceIOR,m.iridescenceThicknessMinimum.value=p.iridescenceThicknessRange[0],m.iridescenceThicknessMaximum.value=p.iridescenceThicknessRange[1],p.iridescenceMap&&(m.iridescenceMap.value=p.iridescenceMap,t(p.iridescenceMap,m.iridescenceMapTransform)),p.iridescenceThicknessMap&&(m.iridescenceThicknessMap.value=p.iridescenceThicknessMap,t(p.iridescenceThicknessMap,m.iridescenceThicknessMapTransform))),p.transmission>0&&(m.transmission.value=p.transmission,m.transmissionSamplerMap.value=M.texture,m.transmissionSamplerSize.value.set(M.width,M.height),p.transmissionMap&&(m.transmissionMap.value=p.transmissionMap,t(p.transmissionMap,m.transmissionMapTransform)),m.thickness.value=p.thickness,p.thicknessMap&&(m.thicknessMap.value=p.thicknessMap,t(p.thicknessMap,m.thicknessMapTransform)),m.attenuationDistance.value=p.attenuationDistance,m.attenuationColor.value.copy(p.attenuationColor)),p.anisotropy>0&&(m.anisotropyVector.value.set(p.anisotropy*Math.cos(p.anisotropyRotation),p.anisotropy*Math.sin(p.anisotropyRotation)),p.anisotropyMap&&(m.anisotropyMap.value=p.anisotropyMap,t(p.anisotropyMap,m.anisotropyMapTransform))),m.specularIntensity.value=p.specularIntensity,m.specularColor.value.copy(p.specularColor),p.specularColorMap&&(m.specularColorMap.value=p.specularColorMap,t(p.specularColorMap,m.specularColorMapTransform)),p.specularIntensityMap&&(m.specularIntensityMap.value=p.specularIntensityMap,t(p.specularIntensityMap,m.specularIntensityMapTransform))}function g(m,p){p.matcap&&(m.matcap.value=p.matcap)}function v(m,p){let M=e.get(p).light;m.referencePosition.value.setFromMatrixPosition(M.matrixWorld),m.nearDistance.value=M.shadow.camera.near,m.farDistance.value=M.shadow.camera.far}return{refreshFogUniforms:n,refreshMaterialUniforms:i}}function Yv(s,e,t,n){let i={},r={},o=[],a=s.getParameter(s.MAX_UNIFORM_BUFFER_BINDINGS);function l(x,y){let S=y.program;n.uniformBlockBinding(x,S)}function c(x,y){let S=i[x.id];S===void 0&&(m(x),S=h(x),i[x.id]=S,x.addEventListener("dispose",M));let C=y.program;n.updateUBOMapping(x,C);let _=e.render.frame;r[x.id]!==_&&(d(x),r[x.id]=_)}function h(x){let y=u();x.__bindingPointIndex=y;let S=s.createBuffer(),C=x.__size,_=x.usage;return s.bindBuffer(s.UNIFORM_BUFFER,S),s.bufferData(s.UNIFORM_BUFFER,C,_),s.bindBuffer(s.UNIFORM_BUFFER,null),s.bindBufferBase(s.UNIFORM_BUFFER,y,S),S}function u(){for(let x=0;x<a;x++)if(o.indexOf(x)===-1)return o.push(x),x;return et("WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function d(x){let y=i[x.id],S=x.uniforms,C=x.__cache;s.bindBuffer(s.UNIFORM_BUFFER,y);for(let _=0,I=S.length;_<I;_++){let b=S[_];if(Array.isArray(b))for(let N=0,w=b.length;N<w;N++)f(b[N],_,N,C);else f(b,_,0,C)}s.bindBuffer(s.UNIFORM_BUFFER,null)}function f(x,y,S,C){if(v(x,y,S,C)===!0){let _=x.__offset,I=x.value;if(Array.isArray(I)){let b=0;for(let N=0;N<I.length;N++){let w=I[N],H=p(w);g(w,x.__data,b),typeof w!="number"&&typeof w!="boolean"&&!w.isMatrix3&&!ArrayBuffer.isView(w)&&(b+=H.storage/Float32Array.BYTES_PER_ELEMENT)}}else g(I,x.__data,0);s.bufferSubData(s.UNIFORM_BUFFER,_,x.__data)}}function g(x,y,S){typeof x=="number"||typeof x=="boolean"?y[0]=x:x.isMatrix3?(y[0]=x.elements[0],y[1]=x.elements[1],y[2]=x.elements[2],y[3]=0,y[4]=x.elements[3],y[5]=x.elements[4],y[6]=x.elements[5],y[7]=0,y[8]=x.elements[6],y[9]=x.elements[7],y[10]=x.elements[8],y[11]=0):ArrayBuffer.isView(x)?y.set(new x.constructor(x.buffer,x.byteOffset,y.length)):x.toArray(y,S)}function v(x,y,S,C){let _=x.value,I=y+"_"+S;if(C[I]===void 0)return typeof _=="number"||typeof _=="boolean"?C[I]=_:ArrayBuffer.isView(_)?C[I]=_.slice():C[I]=_.clone(),!0;{let b=C[I];if(typeof _=="number"||typeof _=="boolean"){if(b!==_)return C[I]=_,!0}else{if(ArrayBuffer.isView(_))return!0;if(b.equals(_)===!1)return b.copy(_),!0}}return!1}function m(x){let y=x.uniforms,S=0,C=16;for(let I=0,b=y.length;I<b;I++){let N=Array.isArray(y[I])?y[I]:[y[I]];for(let w=0,H=N.length;w<H;w++){let F=N[w],L=Array.isArray(F.value)?F.value:[F.value];for(let O=0,V=L.length;O<V;O++){let R=L[O],G=p(R),oe=S%C,k=oe%G.boundary,re=oe+k;S+=k,re!==0&&C-re<G.storage&&(S+=C-re),F.__data=new Float32Array(G.storage/Float32Array.BYTES_PER_ELEMENT),F.__offset=S,S+=G.storage}}}let _=S%C;return _>0&&(S+=C-_),x.__size=S,x.__cache={},this}function p(x){let y={boundary:0,storage:0};return typeof x=="number"||typeof x=="boolean"?(y.boundary=4,y.storage=4):x.isVector2?(y.boundary=8,y.storage=8):x.isVector3||x.isColor?(y.boundary=16,y.storage=12):x.isVector4?(y.boundary=16,y.storage=16):x.isMatrix3?(y.boundary=48,y.storage=48):x.isMatrix4?(y.boundary=64,y.storage=64):x.isTexture?Ze("WebGLRenderer: Texture samplers can not be part of an uniforms group."):ArrayBuffer.isView(x)?(y.boundary=16,y.storage=x.byteLength):Ze("WebGLRenderer: Unsupported uniform value type.",x),y}function M(x){let y=x.target;y.removeEventListener("dispose",M);let S=o.indexOf(y.__bindingPointIndex);o.splice(S,1),s.deleteBuffer(i[y.id]),delete i[y.id],delete r[y.id]}function T(){for(let x in i)s.deleteBuffer(i[x]);o=[],i={},r={}}return{bind:l,update:c,dispose:T}}var Zv=new Uint16Array([12469,15057,12620,14925,13266,14620,13807,14376,14323,13990,14545,13625,14713,13328,14840,12882,14931,12528,14996,12233,15039,11829,15066,11525,15080,11295,15085,10976,15082,10705,15073,10495,13880,14564,13898,14542,13977,14430,14158,14124,14393,13732,14556,13410,14702,12996,14814,12596,14891,12291,14937,11834,14957,11489,14958,11194,14943,10803,14921,10506,14893,10278,14858,9960,14484,14039,14487,14025,14499,13941,14524,13740,14574,13468,14654,13106,14743,12678,14818,12344,14867,11893,14889,11509,14893,11180,14881,10751,14852,10428,14812,10128,14765,9754,14712,9466,14764,13480,14764,13475,14766,13440,14766,13347,14769,13070,14786,12713,14816,12387,14844,11957,14860,11549,14868,11215,14855,10751,14825,10403,14782,10044,14729,9651,14666,9352,14599,9029,14967,12835,14966,12831,14963,12804,14954,12723,14936,12564,14917,12347,14900,11958,14886,11569,14878,11247,14859,10765,14828,10401,14784,10011,14727,9600,14660,9289,14586,8893,14508,8533,15111,12234,15110,12234,15104,12216,15092,12156,15067,12010,15028,11776,14981,11500,14942,11205,14902,10752,14861,10393,14812,9991,14752,9570,14682,9252,14603,8808,14519,8445,14431,8145,15209,11449,15208,11451,15202,11451,15190,11438,15163,11384,15117,11274,15055,10979,14994,10648,14932,10343,14871,9936,14803,9532,14729,9218,14645,8742,14556,8381,14461,8020,14365,7603,15273,10603,15272,10607,15267,10619,15256,10631,15231,10614,15182,10535,15118,10389,15042,10167,14963,9787,14883,9447,14800,9115,14710,8665,14615,8318,14514,7911,14411,7507,14279,7198,15314,9675,15313,9683,15309,9712,15298,9759,15277,9797,15229,9773,15166,9668,15084,9487,14995,9274,14898,8910,14800,8539,14697,8234,14590,7790,14479,7409,14367,7067,14178,6621,15337,8619,15337,8631,15333,8677,15325,8769,15305,8871,15264,8940,15202,8909,15119,8775,15022,8565,14916,8328,14804,8009,14688,7614,14569,7287,14448,6888,14321,6483,14088,6171,15350,7402,15350,7419,15347,7480,15340,7613,15322,7804,15287,7973,15229,8057,15148,8012,15046,7846,14933,7611,14810,7357,14682,7069,14552,6656,14421,6316,14251,5948,14007,5528,15356,5942,15356,5977,15353,6119,15348,6294,15332,6551,15302,6824,15249,7044,15171,7122,15070,7050,14949,6861,14818,6611,14679,6349,14538,6067,14398,5651,14189,5311,13935,4958,15359,4123,15359,4153,15356,4296,15353,4646,15338,5160,15311,5508,15263,5829,15188,6042,15088,6094,14966,6001,14826,5796,14678,5543,14527,5287,14377,4985,14133,4586,13869,4257,15360,1563,15360,1642,15358,2076,15354,2636,15341,3350,15317,4019,15273,4429,15203,4732,15105,4911,14981,4932,14836,4818,14679,4621,14517,4386,14359,4156,14083,3795,13808,3437,15360,122,15360,137,15358,285,15355,636,15344,1274,15322,2177,15281,2765,15215,3223,15120,3451,14995,3569,14846,3567,14681,3466,14511,3305,14344,3121,14037,2800,13753,2467,15360,0,15360,1,15359,21,15355,89,15346,253,15325,479,15287,796,15225,1148,15133,1492,15008,1749,14856,1882,14685,1886,14506,1783,14324,1608,13996,1398,13702,1183]),gi=null;function Kv(){return gi===null&&(gi=new ji(Zv,16,16,rs,Yt),gi.name="DFG_LUT",gi.minFilter=Ot,gi.magFilter=Ot,gi.wrapS=Fn,gi.wrapT=Fn,gi.generateMipmaps=!1,gi.needsUpdate=!0),gi}var ac=class{constructor(e={}){let{canvas:t=df(),context:n=null,depth:i=!0,stencil:r=!1,alpha:o=!1,antialias:a=!1,premultipliedAlpha:l=!0,preserveDrawingBuffer:c=!1,powerPreference:h="default",failIfMajorPerformanceCaveat:u=!1,reversedDepthBuffer:d=!1,outputBufferType:f=bn}=e;this.isWebGLRenderer=!0;let g;if(n!==null){if(typeof WebGLRenderingContext<"u"&&n instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");g=n.getContextAttributes().alpha}else g=o;let v=f,m=new Set([Sl,bl,Ml]),p=new Set([bn,Qn,Tr,Er,_l,vl]),M=new Uint32Array(4),T=new Int32Array(4),x=new W,y=null,S=null,C=[],_=[],I=null;this.domElement=t,this.debug={checkShaderErrors:!0,diagnostics:{keywords:!1},onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this.toneMapping=Jn,this.toneMappingExposure=1,this.transmissionResolutionScale=1;let b=this,N=!1,w=null,H=null,F=null,L=null;this._outputColorSpace=Ft;let O=0,V=0,R=null,G=-1,oe=null,k=new gt,re=new gt,Ae=null,Le=new Ce(0),be=0,Fe=t.width,le=t.height,ge=1,Re=null,de=null,he=new gt(0,0,Fe,le),ce=new gt(0,0,Fe,le),Se=!1,fe=new dr,ue=!1,Me=!1,Ee=new it,Ge=new W,qe=new gt,ke={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},Ye=!1;function Qe(){return R===null?ge:1}let Q=n;function st(U,P){return t.getContext(U,P)}let $e,X,E,ae,se,_e,K,te,J,$,xe,ve,me,pe,we,B,q,D,Y,A,z,ee,ne;try{let U={alpha:!0,depth:i,stencil:r,antialias:a,premultipliedAlpha:l,preserveDrawingBuffer:c,powerPreference:h,failIfMajorPerformanceCaveat:u};if("setAttribute"in t&&t.setAttribute("data-engine",`three.js r${"186"}`),t.addEventListener("webglcontextlost",je,!1),t.addEventListener("webglcontextrestored",Je,!1),t.addEventListener("webglcontextcreationerror",Mt,!1),Q===null){let P="webgl2";if(Q=st(P,U),Q===null)throw st(P)?new Error("THREE.WebGLRenderer: Error creating WebGL context with your selected attributes."):new Error("THREE.WebGLRenderer: Error creating WebGL context.")}Te()}catch(U){throw t.removeEventListener("webglcontextlost",je,!1),t.removeEventListener("webglcontextrestored",Je,!1),t.removeEventListener("webglcontextcreationerror",Mt,!1),et("WebGLRenderer: "+U.message),U}function Te(){$e=new n_(Q),$e.init(),z=new Vv(Q,$e),X=new qx(Q,$e,e,z),E=new kv(Q,$e),X.reversedDepthBuffer&&d&&E.buffers.depth.setReversed(!0),H=Q.createFramebuffer(),F=Q.createFramebuffer(),L=Q.createFramebuffer(),ae=new r_(Q),se=new Ev,_e=new Hv(Q,$e,E,se,X,z,ae),K=new t_(b),te=new ag(Q),ee=new Wx(Q,te),J=new i_(Q,te,ae,ee),$=new a_(Q,J,te,ee,ae),D=new o_(Q,X,_e),we=new Yx(se),xe=new Tv(b,K,$e,X,ee,we),ve=new qv(b,se),me=new Rv,pe=new Nv($e),q=new Gx(b,K,E,$,g,l),B=new zv(b,$,X),ne=new Yv(Q,ae,X,E),Y=new Xx(Q,$e,ae),A=new s_(Q,$e,ae),ae.programs=xe.programs,b.capabilities=X,b.extensions=$e,b.properties=se,b.renderLists=me,b.shadowMap=B,b.state=E,b.info=ae}v!==bn&&(I=new c_(v,t.width,t.height,a,i,r));let ye=new Zh(b,Q);this.xr=ye,this.getContext=function(){return Q},this.getContextAttributes=function(){return Q.getContextAttributes()},this.forceContextLoss=function(){let U=$e.get("WEBGL_lose_context");U&&U.loseContext()},this.forceContextRestore=function(){let U=$e.get("WEBGL_lose_context");U&&U.restoreContext()},this.getPixelRatio=function(){return ge},this.setPixelRatio=function(U){U!==void 0&&(ge=U,this.setSize(Fe,le,!1))},this.getSize=function(U){return U.set(Fe,le)},this.setSize=function(U,P,Z=!0){if(ye.isPresenting){Ze("WebGLRenderer: Can't change size while VR device is presenting.");return}Fe=U,le=P,t.width=Math.floor(U*ge),t.height=Math.floor(P*ge),Z===!0&&(t.style.width=U+"px",t.style.height=P+"px"),I!==null&&I.setSize(t.width,t.height),this.setViewport(0,0,U,P)},this.getDrawingBufferSize=function(U){return U.set(Fe*ge,le*ge).floor()},this.setDrawingBufferSize=function(U,P,Z){Fe=U,le=P,ge=Z,t.width=Math.floor(U*Z),t.height=Math.floor(P*Z),this.setViewport(0,0,U,P)},this.setEffects=function(U){if(v===bn){et("WebGLRenderer: setEffects() requires outputBufferType set to HalfFloatType or FloatType.");return}if(U){for(let P=0;P<U.length;P++)if(U[P].isOutputPass===!0){Ze("WebGLRenderer: OutputPass is not needed in setEffects(). Tone mapping and color space conversion are applied automatically.");break}}I.setEffects(U||[])},this.getCurrentViewport=function(U){return U.copy(k)},this.getViewport=function(U){return U.copy(he)},this.setViewport=function(U,P,Z,j){U.isVector4?he.set(U.x,U.y,U.z,U.w):he.set(U,P,Z,j),E.viewport(k.copy(he).multiplyScalar(ge).round())},this.getScissor=function(U){return U.copy(ce)},this.setScissor=function(U,P,Z,j){U.isVector4?ce.set(U.x,U.y,U.z,U.w):ce.set(U,P,Z,j),E.scissor(re.copy(ce).multiplyScalar(ge).round())},this.getScissorTest=function(){return Se},this.setScissorTest=function(U){E.setScissorTest(Se=U)},this.setOpaqueSort=function(U){Re=U},this.setTransparentSort=function(U){de=U},this.getClearColor=function(U){return U.copy(q.getClearColor())},this.setClearColor=function(){q.setClearColor(...arguments)},this.getClearAlpha=function(){return q.getClearAlpha()},this.setClearAlpha=function(){q.setClearAlpha(...arguments)},this.clear=function(U=!0,P=!0,Z=!0){let j=0;if(U){let ie=!1;if(R!==null){let Ie=R.texture.format;ie=m.has(Ie)}if(ie){let Ie=R.texture.type,De=p.has(Ie),Pe=q.getClearColor(),Oe=q.getClearAlpha(),Ne=Pe.r,He=Pe.g,ze=Pe.b;De?(M[0]=Ne,M[1]=He,M[2]=ze,M[3]=Oe,Q.clearBufferuiv(Q.COLOR,0,M)):(T[0]=Ne,T[1]=He,T[2]=ze,T[3]=Oe,Q.clearBufferiv(Q.COLOR,0,T))}else j|=Q.COLOR_BUFFER_BIT}P&&(j|=Q.DEPTH_BUFFER_BIT,this.state.buffers.depth.setMask(!0)),Z&&(j|=Q.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),j!==0&&Q.clear(j)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.setNodesHandler=function(U){U.setRenderer(this),w=U},this.dispose=function(){t.removeEventListener("webglcontextlost",je,!1),t.removeEventListener("webglcontextrestored",Je,!1),t.removeEventListener("webglcontextcreationerror",Mt,!1),q.dispose(),me.dispose(),pe.dispose(),se.dispose(),K.dispose(),$.dispose(),ee.dispose(),ne.dispose(),xe.dispose(),ye.dispose(),ye.removeEventListener("sessionstart",rn),ye.removeEventListener("sessionend",sn),zt.stop()};function je(U){U.preventDefault(),no("WebGLRenderer: Context Lost."),N=!0}function Je(){no("WebGLRenderer: Context Restored."),N=!1;let U=ae.autoReset,P=B.enabled,Z=B.autoUpdate,j=B.needsUpdate,ie=B.type;Te(),ae.autoReset=U,B.enabled=P,B.autoUpdate=Z,B.needsUpdate=j,B.type=ie}function Mt(U){et("WebGLRenderer: A WebGL context could not be created. Reason: ",U.statusMessage)}function nt(U){let P=U.target;P.removeEventListener("dispose",nt),Pt(P)}function Pt(U){It(U),se.remove(U)}function It(U){let P=se.get(U).programs;P!==void 0&&(P.forEach(function(Z){xe.releaseProgram(Z)}),U.isShaderMaterial&&xe.releaseShaderCache(U))}this.renderBufferDirect=function(U,P,Z,j,ie,Ie){P===null&&(P=ke);let De=ie.isMesh&&ie.matrixWorld.determinantAffine()<0,Pe=ia(U,P,Z,j,ie);E.setMaterial(j,De);let Oe=Z.index,Ne=1;if(j.wireframe===!0){if(Oe=J.getWireframeAttribute(Z),Oe===void 0)return;Ne=2}let He=Z.drawRange,ze=Z.attributes.position,Be=He.start*Ne,We=(He.start+He.count)*Ne;Ie!==null&&(Be=Math.max(Be,Ie.start*Ne),We=Math.min(We,(Ie.start+Ie.count)*Ne)),Oe!==null?(Be=Math.max(Be,0),We=Math.min(We,Oe.count)):ze!=null&&(Be=Math.max(Be,0),We=Math.min(We,ze.count));let at=We-Be;if(at<0||at===1/0)return;ee.setup(ie,j,Pe,Z,Oe);let yt,xt=Y;if(Oe!==null&&(yt=te.get(Oe),xt=A,xt.setIndex(yt)),ie.isMesh)j.wireframe===!0?(E.setLineWidth(j.wireframeLinewidth*Qe()),xt.setMode(Q.LINES)):xt.setMode(Q.TRIANGLES);else if(ie.isLine){let on=j.linewidth;on===void 0&&(on=1),E.setLineWidth(on*Qe()),ie.isLineSegments?xt.setMode(Q.LINES):ie.isLineLoop?xt.setMode(Q.LINE_LOOP):xt.setMode(Q.LINE_STRIP)}else ie.isPoints?xt.setMode(Q.POINTS):ie.isSprite&&xt.setMode(Q.TRIANGLES);if(ie.isBatchedMesh)if($e.get("WEBGL_multi_draw"))xt.renderMultiDraw(ie._multiDrawStarts,ie._multiDrawCounts,ie._multiDrawCount);else{let on=ie._multiDrawStarts,Xe=ie._multiDrawCounts,fn=ie._multiDrawCount,bt=Oe?te.get(Oe).bytesPerElement:1,Nn=se.get(j).currentProgram.getUniforms();for(let si=0;si<fn;si++)Nn.setValue(Q,"_gl_DrawID",si),xt.render(on[si]/bt,Xe[si])}else if(ie.isInstancedMesh)xt.renderInstances(Be,at,ie.count);else if(Z.isInstancedBufferGeometry){let on=Z._maxInstanceCount!==void 0?Z._maxInstanceCount:1/0,Xe=Math.min(Z.instanceCount,on);xt.renderInstances(Be,at,Xe)}else xt.render(Be,at)};function vt(U,P,Z,j){w!==null&&U.isNodeMaterial&&w.setObject(j,U),ue===!0&&we.setState(U,Z,!1),U.transparent===!0&&U.side===Rt&&U.forceSinglePass===!1?(U.side=Kt,U.needsUpdate=!0,us(U,P,j),U.side=Bn,U.needsUpdate=!0,us(U,P,j),U.side=Rt):us(U,P,j)}this.compile=function(U,P,Z=null){Z===null&&(Z=U),w!==null&&w.renderStart(U,P,Z),S=pe.get(Z),S.init(P),_.push(S),Z.traverseVisible(function(ie){ie.isLight&&ie.layers.test(P.layers)&&(S.pushLight(ie),ie.castShadow&&S.pushShadow(ie))}),U!==Z&&U.traverseVisible(function(ie){ie.isLight&&ie.layers.test(P.layers)&&(S.pushLight(ie),ie.castShadow&&S.pushShadow(ie))}),S.setupLights(),w!==null&&w.updateLights(S.state.lightsArray),Me=this.localClippingEnabled,ue=we.init(this.clippingPlanes,Me),ue===!0&&we.setGlobalState(this.clippingPlanes,P),w!==null&&B.render(S.state.shadowsArray,Z,P);let j=new Set;return U.traverse(function(ie){if(!(ie.isMesh||ie.isPoints||ie.isLine||ie.isSprite))return;let Ie=ie.material;if(Ie)if(Array.isArray(Ie))for(let De=0;De<Ie.length;De++){let Pe=Ie[De];vt(Pe,Z,P,ie),j.add(Pe)}else vt(Ie,Z,P,ie),j.add(Ie)}),S=_.pop(),w!==null&&w.renderEnd(),j},this.compileAsync=function(U,P,Z=null){let j=this.compile(U,P,Z);return new Promise(ie=>{function Ie(){if(j.forEach(function(De){let Oe=se.get(De).currentProgram;(Oe===void 0||Oe.isReady())&&j.delete(De)}),j.size===0){ie(U);return}setTimeout(Ie,10)}$e.get("KHR_parallel_shader_compile")!==null?Ie():setTimeout(Ie,10)})};let dn=null;function ni(U){dn&&dn(U)}function rn(){zt.stop()}function sn(){zt.start()}let zt=new kf;zt.setAnimationLoop(ni),typeof self<"u"&&zt.setContext(self),this.setAnimationLoop=function(U){dn=U,ye.setAnimationLoop(U),U===null?zt.stop():zt.start()},ye.addEventListener("sessionstart",rn),ye.addEventListener("sessionend",sn),this.render=function(U,P){if(P!==void 0&&P.isCamera!==!0){et("WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(N===!0)return;w!==null&&w.renderStart(U,P);let Z=ye.enabled===!0&&ye.isPresenting===!0,j=I!==null&&(R===null||Z)&&I.begin(b,R);if(U.matrixWorldAutoUpdate===!0&&U.updateMatrixWorld(),P.parent===null&&P.matrixWorldAutoUpdate===!0&&P.updateMatrixWorld(),ye.enabled===!0&&ye.isPresenting===!0&&(I===null||I.isCompositing()===!1)&&(ye.cameraAutoUpdate===!0&&ye.updateCamera(P),P=ye.getCamera()),U.isScene===!0&&U.onBeforeRender(b,U,P,R),S=pe.get(U,_.length),S.init(P),S.state.textureUnits=_e.getTextureUnits(),_.push(S),Ee.multiplyMatrices(P.projectionMatrix,P.matrixWorldInverse),fe.setFromProjectionMatrix(Ee,Yn,P.reversedDepth),Me=this.localClippingEnabled,ue=we.init(this.clippingPlanes,Me),y=me.get(U,C.length),y.init(),C.push(y),ye.enabled===!0&&ye.isPresenting===!0){let De=b.xr.getDepthSensingMesh();De!==null&&hs(De,P,-1/0,b.sortObjects)}hs(U,P,0,b.sortObjects),y.finish(),w!==null&&w.updateLights(S.state.lightsArray),b.sortObjects===!0&&y.sort(Re,de),Ye=ye.enabled===!1||ye.isPresenting===!1||ye.hasDepthSensing()===!1,Ye&&q.addToRenderList(y,U),this.info.render.frame++,this.info.autoReset===!0&&this.info.reset(),ue===!0&&we.beginShadows();let ie=S.state.shadowsArray;if(B.render(ie,U,P),ue===!0&&we.endShadows(),(j&&I.hasRenderPass())===!1){let De=y.opaque,Pe=y.transmissive;if(S.setupLights(),P.isArrayCamera){let Oe=P.cameras;if(Pe.length>0)for(let Ne=0,He=Oe.length;Ne<He;Ne++){let ze=Oe[Ne];ta(De,Pe,U,ze)}Ye&&q.render(U);for(let Ne=0,He=Oe.length;Ne<He;Ne++){let ze=Oe[Ne];zi(y,U,ze,ze.viewport)}}else Pe.length>0&&ta(De,Pe,U,P),Ye&&q.render(U),zi(y,U,P)}R!==null&&V===0&&(_e.updateMultisampleRenderTarget(R),_e.updateRenderTargetMipmap(R)),j&&I.end(b),U.isScene===!0&&U.onAfterRender(b,U,P),ee.resetDefaultState(),G=-1,oe=null,_.pop(),_.length>0?(S=_[_.length-1],_e.setTextureUnits(S.state.textureUnits),ue===!0&&we.setGlobalState(b.clippingPlanes,S.state.camera)):S=null,C.pop(),C.length>0?y=C[C.length-1]:y=null,w!==null&&w.renderEnd()};function hs(U,P,Z,j){if(U.visible===!1)return;if(U.layers.test(P.layers)){if(U.isGroup)Z=U.renderOrder;else if(U.isLOD)U.autoUpdate===!0&&U.update(P);else if(U.isLightProbeGrid)S.pushLightProbeGrid(U);else if(U.isLight)S.pushLight(U),U.castShadow&&S.pushShadow(U);else if(U.isSprite){if(!U.frustumCulled||U.intersectsFrustum(fe)){j&&qe.setFromMatrixPosition(U.matrixWorld).applyMatrix4(Ee);let De=$.update(U),Pe=U.material;Pe.visible&&y.push(U,De,Pe,Z,qe.z,null,P)}}else if((U.isMesh||U.isLine||U.isPoints)&&(!U.frustumCulled||U.intersectsFrustum(fe))){let De=$.update(U),Pe=U.material;if(j&&(U.boundingSphere!==void 0?(U.boundingSphere===null&&U.computeBoundingSphere(),qe.copy(U.boundingSphere.center)):(De.boundingSphere===null&&De.computeBoundingSphere(),qe.copy(De.boundingSphere.center)),qe.applyMatrix4(U.matrixWorld).applyMatrix4(Ee)),Array.isArray(Pe)){let Oe=De.groups;for(let Ne=0,He=Oe.length;Ne<He;Ne++){let ze=Oe[Ne],Be=Pe[ze.materialIndex];Be&&Be.visible&&y.push(U,De,Be,Z,qe.z,ze,P)}}else Pe.visible&&y.push(U,De,Pe,Z,qe.z,null,P)}}let Ie=U.children;for(let De=0,Pe=Ie.length;De<Pe;De++)hs(Ie[De],P,Z,j)}function zi(U,P,Z,j){let{opaque:ie,transmissive:Ie,transparent:De}=U;S.setupLightsView(Z),ue===!0&&we.setGlobalState(b.clippingPlanes,Z),j&&E.viewport(k.copy(j)),ie.length>0&&ii(ie,P,Z),Ie.length>0&&ii(Ie,P,Z),De.length>0&&ii(De,P,Z),E.buffers.depth.setTest(!0),E.buffers.depth.setMask(!0),E.buffers.color.setMask(!0),E.setPolygonOffset(!1)}function ta(U,P,Z,j){if((Z.isScene===!0?Z.overrideMaterial:null)!==null)return;if(S.state.transmissionRenderTarget[j.id]===void 0){let Be=$e.has("EXT_color_buffer_half_float")||$e.has("EXT_color_buffer_float");S.state.transmissionRenderTarget[j.id]=new Bt(1,1,{generateMipmaps:!0,type:Be?Yt:bn,minFilter:$n,samples:Math.max(4,X.samples),stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,storeMultisampledDepthBuffer:!1,storeMultisampledStencilBuffer:!1,colorSpace:ht.workingColorSpace})}let Ie=S.state.transmissionRenderTarget[j.id],De=j.viewport||k;Ie.setSize(De.z*b.transmissionResolutionScale,De.w*b.transmissionResolutionScale);let Pe=b.getRenderTarget(),Oe=b.getActiveCubeFace(),Ne=b.getActiveMipmapLevel();b.setRenderTarget(Ie),b.getClearColor(Le),be=b.getClearAlpha(),be<1&&b.setClearColor(16777215,.5),b.clear(),Ye&&q.render(Z);let He=b.toneMapping;b.toneMapping=Jn;let ze=j.viewport;if(j.viewport!==void 0&&(j.viewport=void 0),S.setupLightsView(j),ue===!0&&we.setGlobalState(b.clippingPlanes,j),ii(U,Z,j),_e.updateMultisampleRenderTarget(Ie),_e.updateRenderTargetMipmap(Ie),$e.has("WEBGL_multisampled_render_to_texture")===!1){let Be=!1;for(let We=0,at=P.length;We<at;We++){let yt=P[We],{object:xt,geometry:on,material:Xe,group:fn}=yt;if(Xe.side===Rt&&xt.layers.test(j.layers)){let bt=Xe.side;Xe.side=Kt,Xe.needsUpdate=!0,kr(xt,Z,j,on,Xe,fn),Xe.side=bt,Xe.needsUpdate=!0,Be=!0}}Be===!0&&(_e.updateMultisampleRenderTarget(Ie),_e.updateRenderTargetMipmap(Ie))}b.setRenderTarget(Pe,Oe,Ne),b.setClearColor(Le,be),ze!==void 0&&(j.viewport=ze),b.toneMapping=He}function ii(U,P,Z){let j=P.isScene===!0?P.overrideMaterial:null;for(let ie=0,Ie=U.length;ie<Ie;ie++){let De=U[ie],{object:Pe,geometry:Oe,group:Ne}=De,He=De.material;He.allowOverride===!0&&j!==null&&(He=j),Pe.layers.test(Z.layers)&&kr(Pe,P,Z,Oe,He,Ne)}}function kr(U,P,Z,j,ie,Ie){w!==null&&ie.isNodeMaterial&&w.setObject(U,ie),U.onBeforeRender(b,P,Z,j,ie,Ie),U.modelViewMatrix.multiplyMatrices(Z.matrixWorldInverse,U.matrixWorld),U.normalMatrix.getNormalMatrix(U.modelViewMatrix),ie.onBeforeRender(b,P,Z,j,U,Ie),ie.transparent===!0&&ie.side===Rt&&ie.forceSinglePass===!1?(ie.side=Kt,ie.needsUpdate=!0,b.renderBufferDirect(Z,P,j,ie,U,Ie),ie.side=Bn,ie.needsUpdate=!0,b.renderBufferDirect(Z,P,j,ie,U,Ie),ie.side=Rt):b.renderBufferDirect(Z,P,j,ie,U,Ie),U.onAfterRender(b,P,Z,j,ie,Ie)}function us(U,P,Z){P.isScene!==!0&&(P=ke);let j=se.get(U),ie=S.state.lights,Ie=S.state.shadowsArray,De=ie.state.version,Pe=xe.getParameters(U,ie.state,Ie,P,Z,S.state.lightProbeGridArray),Oe=xe.getProgramCacheKey(Pe),Ne=j.programs;j.environment=U.isMeshStandardMaterial||U.isMeshLambertMaterial||U.isMeshPhongMaterial?P.environment:null,j.fog=P.fog;let He=U.isMeshStandardMaterial||U.isMeshLambertMaterial&&!U.envMap||U.isMeshPhongMaterial&&!U.envMap;j.envMap=K.get(U.envMap||j.environment,He),j.envMapRotation=j.environment!==null&&U.envMap===null?P.environmentRotation:U.envMapRotation,Ne===void 0&&(U.addEventListener("dispose",nt),Ne=new Map,j.programs=Ne);let ze=Ne.get(Oe);if(ze!==void 0){if(j.currentProgram===ze&&j.lightsStateVersion===De)return na(U,Pe),ze}else Pe.uniforms=xe.getUniforms(U),w!==null&&U.isNodeMaterial&&w.build(U,Z,Pe),U.onBeforeCompile(Pe,b),ze=xe.acquireProgram(Pe,Oe),Ne.set(Oe,ze),j.uniforms=Pe.uniforms;let Be=j.uniforms;return(!U.isShaderMaterial&&!U.isRawShaderMaterial||U.clipping===!0)&&(Be.clippingPlanes=we.uniform),na(U,Pe),j.needsLights=Rc(U),j.lightsStateVersion=De,j.needsLights&&(Be.ambientLightColor.value=ie.state.ambient,Be.lightProbe.value=ie.state.probe,Be.sunLights.value=ie.state.sun,Be.sunLightShadows.value=ie.state.sunShadow,Be.directionalLights.value=ie.state.directional,Be.directionalLightShadows.value=ie.state.directionalShadow,Be.spotLights.value=ie.state.spot,Be.spotLightShadows.value=ie.state.spotShadow,Be.rectAreaLights.value=ie.state.rectArea,Be.ltc_1.value=ie.state.rectAreaLTC1,Be.ltc_2.value=ie.state.rectAreaLTC2,Be.pointLights.value=ie.state.point,Be.pointLightShadows.value=ie.state.pointShadow,Be.hemisphereLights.value=ie.state.hemi,Be.sunShadowMatrix.value=ie.state.sunShadowMatrix,Be.sunShadowCascade.value=ie.state.sunShadowCascade,Be.directionalShadowMatrix.value=ie.state.directionalShadowMatrix,Be.spotLightMatrix.value=ie.state.spotLightMatrix,Be.spotLightMap.value=ie.state.spotLightMap,Be.pointShadowMatrix.value=ie.state.pointShadowMatrix),j.lightProbeGrid=S.state.lightProbeGridArray.length>0,j.currentProgram=ze,j.uniformsList=null,ze}function Hr(U){if(U.uniformsList===null){let P=U.currentProgram.getUniforms();U.uniformsList=Pr.seqWithValue(P.seq,U.uniforms)}return U.uniformsList}function na(U,P){let Z=se.get(U);Z.outputColorSpace=P.outputColorSpace,Z.batching=P.batching,Z.batchingColor=P.batchingColor,Z.instancing=P.instancing,Z.instancingColor=P.instancingColor,Z.instancingMorph=P.instancingMorph,Z.skinning=P.skinning,Z.morphTargets=P.morphTargets,Z.morphNormals=P.morphNormals,Z.morphColors=P.morphColors,Z.morphTargetsCount=P.morphTargetsCount,Z.numClippingPlanes=P.numClippingPlanes,Z.numIntersection=P.numClipIntersection,Z.vertexAlphas=P.vertexAlphas,Z.vertexTangents=P.vertexTangents,Z.toneMapping=P.toneMapping}function Ac(U,P){if(U.length===0)return null;if(U.length===1)return U[0].texture!==null?U[0]:null;x.setFromMatrixPosition(P.matrixWorld);for(let Z=0,j=U.length;Z<j;Z++){let ie=U[Z];if(ie.texture!==null&&ie.boundingBox.containsPoint(x))return ie}return null}function ia(U,P,Z,j,ie){P.isScene!==!0&&(P=ke),_e.resetTextureUnits();let Ie=P.fog,De=j.isMeshStandardMaterial||j.isMeshLambertMaterial||j.isMeshPhongMaterial?P.environment:null,Pe=R===null?b.outputColorSpace:R.isXRRenderTarget===!0?R.texture.colorSpace:ht.workingColorSpace,Oe=j.isMeshStandardMaterial||j.isMeshLambertMaterial&&!j.envMap||j.isMeshPhongMaterial&&!j.envMap,Ne=K.get(j.envMap||De,Oe),He=j.vertexColors===!0&&!!Z.attributes.color&&Z.attributes.color.itemSize===4,ze=!!Z.attributes.tangent&&(!!j.normalMap||j.anisotropy>0),Be=!!Z.morphAttributes.position,We=!!Z.morphAttributes.normal,at=!!Z.morphAttributes.color,yt=Jn;j.toneMapped&&(R===null||R.isXRRenderTarget===!0)&&(yt=b.toneMapping);let xt=Z.morphAttributes.position||Z.morphAttributes.normal||Z.morphAttributes.color,on=xt!==void 0?xt.length:0,Xe=se.get(j),fn=S.state.lights;if(ue===!0&&(Me===!0||U!==oe)){let Nt=U===oe&&j.id===G;we.setState(j,U,Nt)}let bt=!1;j.version===Xe.__version?(Xe.needsLights&&Xe.lightsStateVersion!==fn.state.version||Xe.outputColorSpace!==Pe||ie.isBatchedMesh&&Xe.batching===!1||!ie.isBatchedMesh&&Xe.batching===!0||ie.isBatchedMesh&&Xe.batchingColor===!0&&ie._colorsTexture===null||ie.isBatchedMesh&&Xe.batchingColor===!1&&ie._colorsTexture!==null||ie.isInstancedMesh&&Xe.instancing===!1||!ie.isInstancedMesh&&Xe.instancing===!0||ie.isSkinnedMesh&&Xe.skinning===!1||!ie.isSkinnedMesh&&Xe.skinning===!0||ie.isInstancedMesh&&Xe.instancingColor===!0&&ie.instanceColor===null||ie.isInstancedMesh&&Xe.instancingColor===!1&&ie.instanceColor!==null||ie.isInstancedMesh&&Xe.instancingMorph===!0&&ie.morphTexture===null||ie.isInstancedMesh&&Xe.instancingMorph===!1&&ie.morphTexture!==null||Xe.envMap!==Ne||j.fog===!0&&Xe.fog!==Ie||Xe.numClippingPlanes!==void 0&&(Xe.numClippingPlanes!==we.numPlanes||Xe.numIntersection!==we.numIntersection)||Xe.vertexAlphas!==He||Xe.vertexTangents!==ze||Xe.morphTargets!==Be||Xe.morphNormals!==We||Xe.morphColors!==at||Xe.toneMapping!==yt||Xe.morphTargetsCount!==on||!!Xe.lightProbeGrid!=S.state.lightProbeGridArray.length>0)&&(bt=!0):(bt=!0,Xe.__version=j.version);let Nn=Xe.currentProgram;bt===!0&&(Nn=us(j,P,ie),w&&j.isNodeMaterial&&w.onUpdateProgram(j,Nn,Xe));let si=!1,ki=!1,Bs=!1,Lt=Nn.getUniforms(),kt=Xe.uniforms;if(E.useProgram(Nn.program)&&(si=!0,ki=!0,Bs=!0),j.id!==G&&(G=j.id,ki=!0),Xe.needsLights){let Nt=Ac(S.state.lightProbeGridArray,ie);Xe.lightProbeGrid!==Nt&&(Xe.lightProbeGrid=Nt,ki=!0)}if(si||oe!==U){E.buffers.depth.getReversed()&&U.reversedDepth!==!0&&(U._reversedDepth=!0,U.updateProjectionMatrix()),Lt.setValue(Q,"projectionMatrix",U.projectionMatrix),Lt.setValue(Q,"viewMatrix",U.matrixWorldInverse);let Vi=Lt.map.cameraPosition;Vi!==void 0&&Vi.setValue(Q,Ge.setFromMatrixPosition(U.matrixWorld)),X.logarithmicDepthBuffer&&Lt.setValue(Q,"logDepthBufFC",2/(Math.log(U.far+1)/Math.LN2)),(j.isMeshPhongMaterial||j.isMeshToonMaterial||j.isMeshLambertMaterial||j.isMeshBasicMaterial||j.isMeshStandardMaterial||j.isShaderMaterial)&&Lt.setValue(Q,"isOrthographic",U.isOrthographicCamera===!0),oe!==U&&(oe=U,ki=!0,Bs=!0)}if(Xe.needsLights&&(fn.state.sunShadowMap.length>0&&Lt.setValue(Q,"sunShadowMap",fn.state.sunShadowMap,_e),fn.state.directionalShadowMap.length>0&&Lt.setValue(Q,"directionalShadowMap",fn.state.directionalShadowMap,_e),fn.state.spotShadowMap.length>0&&Lt.setValue(Q,"spotShadowMap",fn.state.spotShadowMap,_e),fn.state.pointShadowMap.length>0&&Lt.setValue(Q,"pointShadowMap",fn.state.pointShadowMap,_e)),ie.isSkinnedMesh){Lt.setOptional(Q,ie,"bindMatrix"),Lt.setOptional(Q,ie,"bindMatrixInverse");let Nt=ie.skeleton;Nt&&(Nt.boneTexture===null&&Nt.computeBoneTexture(),Lt.setValue(Q,"boneTexture",Nt.boneTexture,_e))}ie.isBatchedMesh&&(Lt.setOptional(Q,ie,"batchingTexture"),Lt.setValue(Q,"batchingTexture",ie._matricesTexture,_e),Lt.setOptional(Q,ie,"batchingIdTexture"),Lt.setValue(Q,"batchingIdTexture",ie._indirectTexture,_e),Lt.setOptional(Q,ie,"batchingColorTexture"),ie._colorsTexture!==null&&Lt.setValue(Q,"batchingColorTexture",ie._colorsTexture,_e));let Hi=Z.morphAttributes;if((Hi.position!==void 0||Hi.normal!==void 0||Hi.color!==void 0)&&D.update(ie,Z,Nn),(ki||Xe.receiveShadow!==ie.receiveShadow)&&(Xe.receiveShadow=ie.receiveShadow,Lt.setValue(Q,"receiveShadow",ie.receiveShadow)),(j.isMeshStandardMaterial||j.isMeshLambertMaterial||j.isMeshPhongMaterial)&&j.envMap===null&&P.environment!==null&&(kt.envMapIntensity.value=P.environmentIntensity),kt.dfgLUT!==void 0&&(kt.dfgLUT.value=Kv()),ki){if(Lt.setValue(Q,"toneMappingExposure",b.toneMappingExposure),Xe.needsLights&&sa(kt,Bs),Ie&&j.fog===!0&&ve.refreshFogUniforms(kt,Ie),ve.refreshMaterialUniforms(kt,j,ge,le,S.state.transmissionRenderTarget[U.id]),Xe.needsLights&&Xe.lightProbeGrid){let Nt=Xe.lightProbeGrid;kt.probesSH.value=Nt.texture,kt.probesMin.value.copy(Nt.boundingBox.min),kt.probesMax.value.copy(Nt.boundingBox.max),kt.probesResolution.value.copy(Nt.resolution)}Pr.upload(Q,Hr(Xe),kt,_e)}if(j.isShaderMaterial&&j.uniformsNeedUpdate===!0&&(Pr.upload(Q,Hr(Xe),kt,_e),j.uniformsNeedUpdate=!1),j.isSpriteMaterial&&Lt.setValue(Q,"center",ie.center),Lt.setValue(Q,"modelViewMatrix",ie.modelViewMatrix),Lt.setValue(Q,"normalMatrix",ie.normalMatrix),Lt.setValue(Q,"modelMatrix",ie.matrixWorld),j.uniformsGroups!==void 0){let Nt=j.uniformsGroups;for(let Vi=0,zs=Nt.length;Vi<zs;Vi++){let ku=Nt[Vi];ne.update(ku,Nn),ne.bind(ku,Nn)}}return Nn}function sa(U,P){U.ambientLightColor.needsUpdate=P,U.lightProbe.needsUpdate=P,U.sunLights.needsUpdate=P,U.sunLightShadows.needsUpdate=P,U.directionalLights.needsUpdate=P,U.directionalLightShadows.needsUpdate=P,U.pointLights.needsUpdate=P,U.pointLightShadows.needsUpdate=P,U.spotLights.needsUpdate=P,U.spotLightShadows.needsUpdate=P,U.rectAreaLights.needsUpdate=P,U.hemisphereLights.needsUpdate=P}function Rc(U){return U.isMeshLambertMaterial||U.isMeshToonMaterial||U.isMeshPhongMaterial||U.isMeshStandardMaterial||U.isShadowMaterial||U.isShaderMaterial&&U.lights===!0}this.getActiveCubeFace=function(){return O},this.getActiveMipmapLevel=function(){return V},this.getRenderTarget=function(){return R},this.setRenderTargetTextures=function(U,P,Z){let j=se.get(U);j.__autoAllocateDepthBuffer=U.resolveDepthBuffer===!1,j.__autoAllocateDepthBuffer===!1&&(j.__useRenderToTexture=!1),se.get(U.texture).__webglTexture=P,se.get(U.depthTexture).__webglTexture=j.__autoAllocateDepthBuffer?void 0:Z,j.__hasExternalTextures=!0},this.setRenderTargetFramebuffer=function(U,P){let Z=se.get(U);Z.__webglFramebuffer=P,Z.__useDefaultFramebuffer=P===void 0},this.setRenderTarget=function(U,P=0,Z=0){R=U,O=P,V=Z;let j=null,ie=!1,Ie=!1;if(U){let Pe=se.get(U);if(Pe.__useDefaultFramebuffer!==void 0){E.bindFramebuffer(Q.FRAMEBUFFER,Pe.__webglFramebuffer),k.copy(U.viewport),re.copy(U.scissor),Ae=U.scissorTest,E.viewport(k),E.scissor(re),E.setScissorTest(Ae),G=-1;return}else if(Pe.__webglFramebuffer===void 0)_e.setupRenderTarget(U);else if(Pe.__hasExternalTextures)_e.rebindTextures(U,se.get(U.texture).__webglTexture,se.get(U.depthTexture).__webglTexture);else if(U.depthBuffer){let He=U.depthTexture;if(Pe.__boundDepthTexture!==He){if(He!==null&&se.has(He)&&(U.width!==He.image.width||U.height!==He.image.height))throw new Error("THREE.WebGLRenderer: Attached DepthTexture is initialized to the incorrect size.");_e.setupDepthRenderbuffer(U)}}let Oe=U.texture;(Oe.isData3DTexture||Oe.isDataArrayTexture||Oe.isCompressedArrayTexture)&&(Ie=!0);let Ne=se.get(U).__webglFramebuffer;U.isWebGLCubeRenderTarget?(Array.isArray(Ne[P])?j=Ne[P][Z]:j=Ne[P],ie=!0):U.samples>0&&_e.useMultisampledRTT(U)===!1?j=se.get(U).__webglMultisampledFramebuffer:Array.isArray(Ne)?j=Ne[Z]:j=Ne,k.copy(U.viewport),re.copy(U.scissor),Ae=U.scissorTest}else k.copy(he).multiplyScalar(ge).floor(),re.copy(ce).multiplyScalar(ge).floor(),Ae=Se;if(Z!==0&&(j=H),E.bindFramebuffer(Q.FRAMEBUFFER,j)&&E.drawBuffers(U,j),E.viewport(k),E.scissor(re),E.setScissorTest(Ae),ie){let Pe=se.get(U.texture);Q.framebufferTexture2D(Q.FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_CUBE_MAP_POSITIVE_X+P,Pe.__webglTexture,Z)}else if(Ie){let Pe=P;for(let Oe=0;Oe<U.textures.length;Oe++){let Ne=se.get(U.textures[Oe]);Q.framebufferTextureLayer(Q.FRAMEBUFFER,Q.COLOR_ATTACHMENT0+Oe,Ne.__webglTexture,Z,Pe)}}else if(U!==null&&Z!==0){let Pe=se.get(U.texture);Q.framebufferTexture2D(Q.FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_2D,Pe.__webglTexture,Z)}G=-1};function ra(U){let P=se.get(U);return(P.__readFormat!==U.format||P.__readType!==U.type)&&(P.__readFormat=U.format,P.__readType=U.type,P.__formatReadable=X.textureFormatReadable(U.format),P.__typeReadable=X.textureTypeReadable(U.type)),P}this.readRenderTargetPixels=function(U,P,Z,j,ie,Ie,De,Pe=0){if(!(U&&U.isWebGLRenderTarget)){et("WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let Oe=se.get(U).__webglFramebuffer;if(U.isWebGLCubeRenderTarget&&De!==void 0&&(Oe=Oe[De]),Oe){E.bindFramebuffer(Q.FRAMEBUFFER,Oe);try{let Ne=U.textures[Pe],He=Ne.format,ze=Ne.type;U.textures.length>1&&Q.readBuffer(Q.COLOR_ATTACHMENT0+Pe);let Be=ra(Ne);if(Be.__formatReadable===!1){et("WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if(Be.__typeReadable===!1){et("WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}P>=0&&P<=U.width-j&&Z>=0&&Z<=U.height-ie&&Q.readPixels(P,Z,j,ie,z.convert(He),z.convert(ze),Ie)}finally{let Ne=R!==null?se.get(R).__webglFramebuffer:null;E.bindFramebuffer(Q.FRAMEBUFFER,Ne)}}},this.readRenderTargetPixelsAsync=async function(U,P,Z,j,ie,Ie,De,Pe=0){if(!(U&&U.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let Oe=se.get(U).__webglFramebuffer;if(U.isWebGLCubeRenderTarget&&De!==void 0&&(Oe=Oe[De]),Oe)if(P>=0&&P<=U.width-j&&Z>=0&&Z<=U.height-ie){E.bindFramebuffer(Q.FRAMEBUFFER,Oe);let Ne=U.textures[Pe],He=Ne.format,ze=Ne.type;U.textures.length>1&&Q.readBuffer(Q.COLOR_ATTACHMENT0+Pe);let Be=ra(Ne);if(Be.__formatReadable===!1)throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if(Be.__typeReadable===!1)throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");let We=Q.createBuffer();Q.bindBuffer(Q.PIXEL_PACK_BUFFER,We),Q.bufferData(Q.PIXEL_PACK_BUFFER,Ie.byteLength,Q.STREAM_READ),Q.readPixels(P,Z,j,ie,z.convert(He),z.convert(ze),0),Q.bindBuffer(Q.PIXEL_PACK_BUFFER,null);let at=R!==null?se.get(R).__webglFramebuffer:null;E.bindFramebuffer(Q.FRAMEBUFFER,at);let yt=Q.fenceSync(Q.SYNC_GPU_COMMANDS_COMPLETE,0);return Q.flush(),await pf(Q,yt,4),Q.bindBuffer(Q.PIXEL_PACK_BUFFER,We),Q.getBufferSubData(Q.PIXEL_PACK_BUFFER,0,Ie),Q.bindBuffer(Q.PIXEL_PACK_BUFFER,null),Q.deleteBuffer(We),Q.deleteSync(yt),Ie}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")},this.copyFramebufferToTexture=function(U,P=null,Z=0){let j=Math.pow(2,-Z),ie=Math.floor(U.image.width*j),Ie=Math.floor(U.image.height*j),De=P!==null?P.x:0,Pe=P!==null?P.y:0;_e.setTexture2D(U,0),Q.copyTexSubImage2D(Q.TEXTURE_2D,Z,0,0,De,Pe,ie,Ie),E.unbindTexture()},this.copyTextureToTexture=function(U,P,Z=null,j=null,ie=0,Ie=0){let De,Pe,Oe,Ne,He,ze,Be,We,at,yt=U.isCompressedTexture?U.mipmaps[Ie]:U.image;if(Z!==null)De=Z.max.x-Z.min.x,Pe=Z.max.y-Z.min.y,Oe=Z.isBox3?Z.max.z-Z.min.z:1,Ne=Z.min.x,He=Z.min.y,ze=Z.isBox3?Z.min.z:0;else{let kt=Math.pow(2,-ie);De=Math.floor(yt.width*kt),Pe=Math.floor(yt.height*kt),U.isDataArrayTexture?Oe=yt.depth:U.isData3DTexture?Oe=Math.floor(yt.depth*kt):Oe=1,Ne=0,He=0,ze=0}j!==null?(Be=j.x,We=j.y,at=j.z):(Be=0,We=0,at=0);let xt=z.convert(P.format),on=z.convert(P.type),Xe;P.isData3DTexture?(_e.setTexture3D(P,0),Xe=Q.TEXTURE_3D):P.isDataArrayTexture||P.isCompressedArrayTexture?(_e.setTexture2DArray(P,0),Xe=Q.TEXTURE_2D_ARRAY):(_e.setTexture2D(P,0),Xe=Q.TEXTURE_2D),E.activeTexture(Q.TEXTURE0),E.pixelStorei(Q.UNPACK_FLIP_Y_WEBGL,P.flipY),E.pixelStorei(Q.UNPACK_PREMULTIPLY_ALPHA_WEBGL,P.premultiplyAlpha),E.pixelStorei(Q.UNPACK_ALIGNMENT,P.unpackAlignment);let fn=E.getParameter(Q.UNPACK_ROW_LENGTH),bt=E.getParameter(Q.UNPACK_IMAGE_HEIGHT),Nn=E.getParameter(Q.UNPACK_SKIP_PIXELS),si=E.getParameter(Q.UNPACK_SKIP_ROWS),ki=E.getParameter(Q.UNPACK_SKIP_IMAGES);E.pixelStorei(Q.UNPACK_ROW_LENGTH,yt.width),E.pixelStorei(Q.UNPACK_IMAGE_HEIGHT,yt.height),E.pixelStorei(Q.UNPACK_SKIP_PIXELS,Ne),E.pixelStorei(Q.UNPACK_SKIP_ROWS,He),E.pixelStorei(Q.UNPACK_SKIP_IMAGES,ze);let Bs=U.isDataArrayTexture||U.isData3DTexture,Lt=P.isDataArrayTexture||P.isData3DTexture;if(U.isDepthTexture){let kt=se.get(U),Hi=se.get(P),Nt=se.get(kt.__renderTarget),Vi=se.get(Hi.__renderTarget);E.bindFramebuffer(Q.READ_FRAMEBUFFER,Nt.__webglFramebuffer),E.bindFramebuffer(Q.DRAW_FRAMEBUFFER,Vi.__webglFramebuffer);for(let zs=0;zs<Oe;zs++)Bs&&(Q.framebufferTextureLayer(Q.READ_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,se.get(U).__webglTexture,ie,ze+zs),Q.framebufferTextureLayer(Q.DRAW_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,se.get(P).__webglTexture,Ie,at+zs)),Q.blitFramebuffer(Ne,He,De,Pe,Be,We,De,Pe,Q.DEPTH_BUFFER_BIT,Q.NEAREST);E.bindFramebuffer(Q.READ_FRAMEBUFFER,null),E.bindFramebuffer(Q.DRAW_FRAMEBUFFER,null)}else if(ie!==0||U.isRenderTargetTexture||se.has(U)){let kt=se.get(U),Hi=se.get(P);E.bindFramebuffer(Q.READ_FRAMEBUFFER,F),E.bindFramebuffer(Q.DRAW_FRAMEBUFFER,L);for(let Nt=0;Nt<Oe;Nt++)Bs?Q.framebufferTextureLayer(Q.READ_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,kt.__webglTexture,ie,ze+Nt):Q.framebufferTexture2D(Q.READ_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_2D,kt.__webglTexture,ie),Lt?Q.framebufferTextureLayer(Q.DRAW_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Hi.__webglTexture,Ie,at+Nt):Q.framebufferTexture2D(Q.DRAW_FRAMEBUFFER,Q.COLOR_ATTACHMENT0,Q.TEXTURE_2D,Hi.__webglTexture,Ie),ie!==0?Q.blitFramebuffer(Ne,He,De,Pe,Be,We,De,Pe,Q.COLOR_BUFFER_BIT,Q.NEAREST):Lt?Q.copyTexSubImage3D(Xe,Ie,Be,We,at+Nt,Ne,He,De,Pe):Q.copyTexSubImage2D(Xe,Ie,Be,We,Ne,He,De,Pe);E.bindFramebuffer(Q.READ_FRAMEBUFFER,null),E.bindFramebuffer(Q.DRAW_FRAMEBUFFER,null)}else Lt?U.isDataTexture||U.isData3DTexture?Q.texSubImage3D(Xe,Ie,Be,We,at,De,Pe,Oe,xt,on,yt.data):P.isCompressedArrayTexture?Q.compressedTexSubImage3D(Xe,Ie,Be,We,at,De,Pe,Oe,xt,yt.data):Q.texSubImage3D(Xe,Ie,Be,We,at,De,Pe,Oe,xt,on,yt):U.isDataTexture?Q.texSubImage2D(Q.TEXTURE_2D,Ie,Be,We,De,Pe,xt,on,yt.data):U.isCompressedTexture?Q.compressedTexSubImage2D(Q.TEXTURE_2D,Ie,Be,We,yt.width,yt.height,xt,yt.data):Q.texSubImage2D(Q.TEXTURE_2D,Ie,Be,We,De,Pe,xt,on,yt);E.pixelStorei(Q.UNPACK_ROW_LENGTH,fn),E.pixelStorei(Q.UNPACK_IMAGE_HEIGHT,bt),E.pixelStorei(Q.UNPACK_SKIP_PIXELS,Nn),E.pixelStorei(Q.UNPACK_SKIP_ROWS,si),E.pixelStorei(Q.UNPACK_SKIP_IMAGES,ki),Ie===0&&P.generateMipmaps&&Q.generateMipmap(Xe),E.unbindTexture()},this.initRenderTarget=function(U){se.get(U).__webglFramebuffer===void 0&&_e.setupRenderTarget(U)},this.initTexture=function(U){U.isCubeTexture?_e.setTextureCube(U,0):U.isData3DTexture?_e.setTexture3D(U,0):U.isDataArrayTexture||U.isCompressedArrayTexture?_e.setTexture2DArray(U,0):_e.setTexture2D(U,0),E.unbindTexture()},this.resetState=function(){O=0,V=0,R=null,E.reset(),ee.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return Yn}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(e){this._outputColorSpace=e;let t=this.getContext();t.drawingBufferColorSpace=ht._getDrawingBufferColorSpace(e),t.unpackColorSpace=ht._getUnpackColorSpace()}};function Kh(s,e){if(e===Th)return console.warn("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Geometry already defined as triangles."),s;if(e===Ar||e===Go){let t=s.getIndex();if(t===null){let r=[],o=s.getAttribute("position");if(o!==void 0){for(let a=0;a<o.count;a++)r.push(a);s.setIndex(r),t=s.getIndex()}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Undefined position attribute. Processing not possible."),s}let n=t.count-2,i=[];if(e===Ar)for(let r=1;r<=n;r++)i.push(t.getX(0)),i.push(t.getX(r)),i.push(t.getX(r+1));else for(let r=0;r<n;r++)r%2===0?(i.push(t.getX(r)),i.push(t.getX(r+1)),i.push(t.getX(r+2))):(i.push(t.getX(r+2)),i.push(t.getX(r+1)),i.push(t.getX(r)));return i.length/3!==n&&console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unable to generate correct amount of triangles."),s.setIndex(i),s.clearGroups(),s}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unknown draw mode:",e),s}function Yf(s){let e=new Map,t=new Map,n=s.clone();return Zf(s,n,function(i,r){e.set(r,i),t.set(i,r)}),n.traverse(function(i){if(!i.isSkinnedMesh)return;let r=i,o=e.get(i),a=o.skeleton.bones;r.skeleton=o.skeleton.clone(),r.bindMatrix.copy(o.bindMatrix),r.skeleton.bones=a.map(function(l){return t.get(l)}),r.bind(r.skeleton,r.bindMatrix)}),n}function Zf(s,e,t){t(s,e);for(let n=0;n<s.children.length;n++)Zf(s.children[n],e.children[n],t)}var as=class extends pi{constructor(e){super(e),this.dracoLoader=null,this.ktx2Loader=null,this.meshoptDecoder=null,this.pluginCallbacks=[],this.register(function(t){return new nu(t)}),this.register(function(t){return new iu(t)}),this.register(function(t){return new du(t)}),this.register(function(t){return new fu(t)}),this.register(function(t){return new pu(t)}),this.register(function(t){return new ru(t)}),this.register(function(t){return new ou(t)}),this.register(function(t){return new au(t)}),this.register(function(t){return new lu(t)}),this.register(function(t){return new tu(t)}),this.register(function(t){return new cu(t)}),this.register(function(t){return new su(t)}),this.register(function(t){return new uu(t)}),this.register(function(t){return new hu(t)}),this.register(function(t){return new Qh(t)}),this.register(function(t){return new hc(t,mt.EXT_MESHOPT_COMPRESSION)}),this.register(function(t){return new hc(t,mt.KHR_MESHOPT_COMPRESSION)}),this.register(function(t){return new mu(t)})}load(e,t,n,i){let r=this,o;if(this.resourcePath!=="")o=this.resourcePath;else if(this.path!==""){let c=Ui.extractUrlBase(e);o=Ui.resolveURL(c,this.path)}else o=Ui.extractUrlBase(e);this.manager.itemStart(e);let a=function(c){i?i(c):console.error(c),r.manager.itemError(e),r.manager.itemEnd(e)},l=new vr(this.manager);l.setPath(this.path),l.setResponseType("arraybuffer"),l.setRequestHeader(this.requestHeader),l.setWithCredentials(this.withCredentials),l.load(e,function(c){try{r.parse(c,o,function(h){t(h),r.manager.itemEnd(e)},a)}catch(h){a(h)}},n,a)}setDRACOLoader(e){return this.dracoLoader=e,this}setKTX2Loader(e){return this.ktx2Loader=e,this}setMeshoptDecoder(e){return this.meshoptDecoder=e,this}register(e){return this.pluginCallbacks.indexOf(e)===-1&&this.pluginCallbacks.push(e),this}unregister(e){return this.pluginCallbacks.indexOf(e)!==-1&&this.pluginCallbacks.splice(this.pluginCallbacks.indexOf(e),1),this}parse(e,t,n,i){let r,o={},a={},l=new TextDecoder;if(typeof e=="string")r=JSON.parse(e);else if(e instanceof ArrayBuffer)if(l.decode(new Uint8Array(e,0,4))===Qf){try{o[mt.KHR_BINARY_GLTF]=new gu(e)}catch(u){i&&i(u);return}r=JSON.parse(o[mt.KHR_BINARY_GLTF].content)}else r=JSON.parse(l.decode(e));else r=e;if(r.asset===void 0||r.asset.version[0]<2){i&&i(new Error("THREE.GLTFLoader: Unsupported asset. glTF versions >=2.0 are supported."));return}let c=new Su(r,{path:t||this.resourcePath||"",crossOrigin:this.crossOrigin,requestHeader:this.requestHeader,manager:this.manager,ktx2Loader:this.ktx2Loader,meshoptDecoder:this.meshoptDecoder});c.fileLoader.setRequestHeader(this.requestHeader);for(let h=0;h<this.pluginCallbacks.length;h++){let u=this.pluginCallbacks[h](c);u.name||console.error("THREE.GLTFLoader: Invalid plugin found: missing name"),a[u.name]=u,o[u.name]=!0}if(r.extensionsUsed)for(let h=0;h<r.extensionsUsed.length;++h){let u=r.extensionsUsed[h],d=r.extensionsRequired||[];switch(u){case mt.KHR_MATERIALS_UNLIT:o[u]=new eu;break;case mt.KHR_DRACO_MESH_COMPRESSION:o[u]=new xu(r,this.dracoLoader);break;case mt.KHR_TEXTURE_TRANSFORM:o[u]=new _u;break;case mt.KHR_MESH_QUANTIZATION:o[u]=new vu;break;default:d.indexOf(u)>=0&&a[u]===void 0&&console.warn('THREE.GLTFLoader: Unknown extension "'+u+'".')}}c.setExtensions(o),c.setPlugins(a),c.parse(n,i)}parseAsync(e,t){let n=this;return new Promise(function(i,r){n.parse(e,t,i,r)})}};function jv(){let s={};return{get:function(e){return s[e]},add:function(e,t){s[e]=t},remove:function(e){delete s[e]},removeAll:function(){s={}}}}function Wt(s,e,t){let n=s.json.materials[e];return n.extensions&&n.extensions[t]?n.extensions[t]:null}var mt={KHR_BINARY_GLTF:"KHR_binary_glTF",KHR_DRACO_MESH_COMPRESSION:"KHR_draco_mesh_compression",KHR_LIGHTS_PUNCTUAL:"KHR_lights_punctual",KHR_MATERIALS_CLEARCOAT:"KHR_materials_clearcoat",KHR_MATERIALS_DISPERSION:"KHR_materials_dispersion",KHR_MATERIALS_IOR:"KHR_materials_ior",KHR_MATERIALS_SHEEN:"KHR_materials_sheen",KHR_MATERIALS_SPECULAR:"KHR_materials_specular",KHR_MATERIALS_TRANSMISSION:"KHR_materials_transmission",KHR_MATERIALS_IRIDESCENCE:"KHR_materials_iridescence",KHR_MATERIALS_ANISOTROPY:"KHR_materials_anisotropy",KHR_MATERIALS_UNLIT:"KHR_materials_unlit",KHR_MATERIALS_VOLUME:"KHR_materials_volume",KHR_TEXTURE_BASISU:"KHR_texture_basisu",KHR_TEXTURE_TRANSFORM:"KHR_texture_transform",KHR_MESH_QUANTIZATION:"KHR_mesh_quantization",KHR_MATERIALS_EMISSIVE_STRENGTH:"KHR_materials_emissive_strength",EXT_MATERIALS_BUMP:"EXT_materials_bump",EXT_TEXTURE_WEBP:"EXT_texture_webp",EXT_TEXTURE_AVIF:"EXT_texture_avif",EXT_MESHOPT_COMPRESSION:"EXT_meshopt_compression",KHR_MESHOPT_COMPRESSION:"KHR_meshopt_compression",EXT_MESH_GPU_INSTANCING:"EXT_mesh_gpu_instancing"},Qh=class{constructor(e){this.parser=e,this.name=mt.KHR_LIGHTS_PUNCTUAL,this.cache={refs:{},uses:{}}}_markDefs(){let e=this.parser,t=this.parser.json.nodes||[];for(let n=0,i=t.length;n<i;n++){let r=t[n];r.extensions&&r.extensions[this.name]&&r.extensions[this.name].light!==void 0&&e._addNodeRef(this.cache,r.extensions[this.name].light)}}_loadLight(e){let t=this.parser,n="light:"+e,i=t.cache.get(n);if(i)return i;let r=t.json,l=((r.extensions&&r.extensions[this.name]||{}).lights||[])[e],c,h=new Ce(16777215);l.color!==void 0&&h.setRGB(l.color[0],l.color[1],l.color[2],mn);let u=l.range!==void 0?l.range:0;switch(l.type){case"directional":c=new Ni(h),c.target.position.set(0,0,-1),c.add(c.target);break;case"point":c=new jn(h),c.distance=u;break;case"spot":c=new So(h),c.distance=u,l.spot=l.spot||{},l.spot.innerConeAngle=l.spot.innerConeAngle!==void 0?l.spot.innerConeAngle:0,l.spot.outerConeAngle=l.spot.outerConeAngle!==void 0?l.spot.outerConeAngle:Math.PI/4,c.angle=l.spot.outerConeAngle,c.penumbra=1-l.spot.innerConeAngle/l.spot.outerConeAngle,c.target.position.set(0,0,-1),c.add(c.target);break;default:throw new Error("THREE.GLTFLoader: Unexpected light type: "+l.type)}return c.position.set(0,0,0),_i(c,l),l.intensity!==void 0&&(c.intensity=l.intensity),c.name=t.createUniqueName(l.name||"light_"+e),i=Promise.resolve(c),t.cache.add(n,i),i}getDependency(e,t){if(e==="light")return this._loadLight(t)}createNodeAttachment(e){let t=this,n=this.parser,r=n.json.nodes[e],a=(r.extensions&&r.extensions[this.name]||{}).light;return a===void 0?null:this._loadLight(a).then(function(l){return n._getNodeRef(t.cache,a,l)})}},eu=class{constructor(){this.name=mt.KHR_MATERIALS_UNLIT}getMaterialType(){return wt}extendParams(e,t,n){let i=[];e.color=new Ce(1,1,1),e.opacity=1;let r=t.pbrMetallicRoughness;if(r){if(Array.isArray(r.baseColorFactor)){let o=r.baseColorFactor;e.color.setRGB(o[0],o[1],o[2],mn),e.opacity=o[3]}r.baseColorTexture!==void 0&&i.push(n.assignTexture(e,"map",r.baseColorTexture,Ft))}return Promise.all(i)}},tu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_EMISSIVE_STRENGTH}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);return n===null||n.emissiveStrength!==void 0&&(t.emissiveIntensity=n.emissiveStrength),Promise.resolve()}},nu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_CLEARCOAT}getMaterialType(e){return Wt(this.parser,e,this.name)!==null?yn:null}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(n.clearcoatFactor!==void 0&&(t.clearcoat=n.clearcoatFactor),n.clearcoatTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatMap",n.clearcoatTexture)),n.clearcoatRoughnessFactor!==void 0&&(t.clearcoatRoughness=n.clearcoatRoughnessFactor),n.clearcoatRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatRoughnessMap",n.clearcoatRoughnessTexture)),n.clearcoatNormalTexture!==void 0&&(i.push(this.parser.assignTexture(t,"clearcoatNormalMap",n.clearcoatNormalTexture)),n.clearcoatNormalTexture.scale!==void 0)){let r=n.clearcoatNormalTexture.scale;t.clearcoatNormalScale=new Ue(r,r)}return Promise.all(i)}},iu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_DISPERSION}getMaterialType(e){return Wt(this.parser,e,this.name)!==null?yn:null}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);return n===null||(t.dispersion=n.dispersion!==void 0?n.dispersion:0),Promise.resolve()}},su=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_IRIDESCENCE}getMaterialType(e){return Wt(this.parser,e,this.name)!==null?yn:null}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.iridescenceFactor!==void 0&&(t.iridescence=n.iridescenceFactor),n.iridescenceTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceMap",n.iridescenceTexture)),n.iridescenceIor!==void 0&&(t.iridescenceIOR=n.iridescenceIor),t.iridescenceThicknessRange===void 0&&(t.iridescenceThicknessRange=[100,400]),n.iridescenceThicknessMinimum!==void 0&&(t.iridescenceThicknessRange[0]=n.iridescenceThicknessMinimum),n.iridescenceThicknessMaximum!==void 0&&(t.iridescenceThicknessRange[1]=n.iridescenceThicknessMaximum),n.iridescenceThicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceThicknessMap",n.iridescenceThicknessTexture)),Promise.all(i)}},ru=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_SHEEN}getMaterialType(e){return Wt(this.parser,e,this.name)!==null?yn:null}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(t.sheenColor=new Ce(0,0,0),t.sheenRoughness=0,t.sheen=1,n.sheenColorFactor!==void 0){let r=n.sheenColorFactor;t.sheenColor.setRGB(r[0],r[1],r[2],mn)}return n.sheenRoughnessFactor!==void 0&&(t.sheenRoughness=n.sheenRoughnessFactor),n.sheenColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenColorMap",n.sheenColorTexture,Ft)),n.sheenRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenRoughnessMap",n.sheenRoughnessTexture)),Promise.all(i)}},ou=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_TRANSMISSION}getMaterialType(e){return Wt(this.parser,e,this.name)!==null?yn:null}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.transmissionFactor!==void 0&&(t.transmission=n.transmissionFactor),n.transmissionTexture!==void 0&&i.push(this.parser.assignTexture(t,"transmissionMap",n.transmissionTexture)),Promise.all(i)}},au=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_VOLUME}getMaterialType(e){return Wt(this.parser,e,this.name)!==null?yn:null}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.thickness=n.thicknessFactor!==void 0?n.thicknessFactor:0,n.thicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"thicknessMap",n.thicknessTexture)),t.attenuationDistance=n.attenuationDistance||1/0;let r=n.attenuationColor||[1,1,1];return t.attenuationColor=new Ce().setRGB(r[0],r[1],r[2],mn),Promise.all(i)}},lu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_IOR}getMaterialType(e){return Wt(this.parser,e,this.name)!==null?yn:null}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);return n===null||(t.ior=n.ior!==void 0?n.ior:1.5,t.ior===0&&(t.ior=1e3)),Promise.resolve()}},cu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_SPECULAR}getMaterialType(e){return Wt(this.parser,e,this.name)!==null?yn:null}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.specularIntensity=n.specularFactor!==void 0?n.specularFactor:1,n.specularTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularIntensityMap",n.specularTexture));let r=n.specularColorFactor||[1,1,1];return t.specularColor=new Ce().setRGB(r[0],r[1],r[2],mn),n.specularColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularColorMap",n.specularColorTexture,Ft)),Promise.all(i)}},hu=class{constructor(e){this.parser=e,this.name=mt.EXT_MATERIALS_BUMP}getMaterialType(e){return Wt(this.parser,e,this.name)!==null?yn:null}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return t.bumpScale=n.bumpFactor!==void 0?n.bumpFactor:1,n.bumpTexture!==void 0&&i.push(this.parser.assignTexture(t,"bumpMap",n.bumpTexture)),Promise.all(i)}},uu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_ANISOTROPY}getMaterialType(e){return Wt(this.parser,e,this.name)!==null?yn:null}extendMaterialParams(e,t){let n=Wt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.anisotropyStrength!==void 0&&(t.anisotropy=n.anisotropyStrength),n.anisotropyRotation!==void 0&&(t.anisotropyRotation=n.anisotropyRotation),n.anisotropyTexture!==void 0&&i.push(this.parser.assignTexture(t,"anisotropyMap",n.anisotropyTexture)),Promise.all(i)}},du=class{constructor(e){this.parser=e,this.name=mt.KHR_TEXTURE_BASISU}loadTexture(e){let t=this.parser,n=t.json,i=n.textures[e];if(!i.extensions||!i.extensions[this.name])return null;let r=i.extensions[this.name],o=t.options.ktx2Loader;if(!o){if(n.extensionsRequired&&n.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setKTX2Loader must be called before loading KTX2 textures");return null}return t.loadTextureImage(e,r.source,o)}},fu=class{constructor(e){this.parser=e,this.name=mt.EXT_TEXTURE_WEBP}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let o=r.extensions[t],a=i.images[o.source],l=n.textureLoader;if(a.uri){let c=n.options.manager.getHandler(a.uri);c!==null&&(l=c)}return n.loadTextureImage(e,o.source,l)}},pu=class{constructor(e){this.parser=e,this.name=mt.EXT_TEXTURE_AVIF}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let o=r.extensions[t],a=i.images[o.source],l=n.textureLoader;if(a.uri){let c=n.options.manager.getHandler(a.uri);c!==null&&(l=c)}return n.loadTextureImage(e,o.source,l)}},hc=class{constructor(e,t){this.name=t,this.parser=e}loadBufferView(e){let t=this.parser.json,n=t.bufferViews[e];if(n.extensions&&n.extensions[this.name]){let i=n.extensions[this.name],r=this.parser.getDependency("buffer",i.buffer),o=this.parser.options.meshoptDecoder;if(!o||!o.supported){if(t.extensionsRequired&&t.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setMeshoptDecoder must be called before loading compressed files");return null}return r.then(function(a){let l=i.byteOffset||0,c=i.byteLength||0,h=i.count,u=i.byteStride,d=new Uint8Array(a,l,c);return o.decodeGltfBufferAsync?o.decodeGltfBufferAsync(h,u,d,i.mode,i.filter).then(function(f){return f.buffer}):o.ready.then(function(){let f=new ArrayBuffer(h*u);return o.decodeGltfBuffer(new Uint8Array(f),h,u,d,i.mode,i.filter),f})})}else return null}},mu=class{constructor(e){this.name=mt.EXT_MESH_GPU_INSTANCING,this.parser=e}createNodeMesh(e){let t=this.parser.json,n=t.nodes[e];if(!n.extensions||!n.extensions[this.name]||n.mesh===void 0)return null;let i=t.meshes[n.mesh];for(let c of i.primitives)if(c.mode!==Hn.TRIANGLES&&c.mode!==Hn.TRIANGLE_STRIP&&c.mode!==Hn.TRIANGLE_FAN&&c.mode!==void 0)return null;let o=n.extensions[this.name].attributes,a=[],l={};for(let c in o)a.push(this.parser.getDependency("accessor",o[c]).then(h=>(l[c]=h,l[c])));return a.length<1?null:(a.push(this.parser.createNodeMesh(e)),Promise.all(a).then(c=>{let h=c.pop(),u=h.isGroup?h.children:[h],d=c[0].count,f=[];for(let g of u){let v=new it,m=new W,p=new Ht,M=new W(1,1,1),T=new _n(g.geometry,g.material,d);for(let y=0;y<d;y++)l.TRANSLATION&&m.fromBufferAttribute(l.TRANSLATION,y),l.ROTATION&&p.fromBufferAttribute(l.ROTATION,y),l.SCALE&&M.fromBufferAttribute(l.SCALE,y),T.setMatrixAt(y,v.compose(m,p,M));let x=null;for(let y in l)if(y==="_COLOR_0"){let S=l[y];T.instanceColor=new Ri(S.array,S.itemSize,S.normalized)}else if(y!=="TRANSLATION"&&y!=="ROTATION"&&y!=="SCALE"){if(x===null){let C=T.geometry;x=new ot,x.name=C.name;for(let _ in C.attributes)x.setAttribute(_,C.attributes[_]);for(let _ in C.morphAttributes)x.morphAttributes[_]=C.morphAttributes[_];C.index!==null&&x.setIndex(C.index),x.morphTargetsRelative=C.morphTargetsRelative;for(let _ of C.groups)x.addGroup(_.start,_.count,_.materialIndex);C.boundingBox!==null&&(x.boundingBox=C.boundingBox.clone()),C.boundingSphere!==null&&(x.boundingSphere=C.boundingSphere.clone()),x.drawRange.start=C.drawRange.start,x.drawRange.count=C.drawRange.count,x.userData=Object.assign({},C.userData),T.geometry=x}let S=l[y];x.setAttribute(y,new Ri(S.array,S.itemSize,S.normalized))}Tt.prototype.copy.call(T,g),this.parser.assignFinalMaterial(T),f.push(T)}return h.isGroup?(h.clear(),h.add(...f),h):f[0]}))}},Qf="glTF",Zo=12,Kf={JSON:1313821514,BIN:5130562},gu=class{constructor(e){this.name=mt.KHR_BINARY_GLTF,this.content=null,this.body=null;let t=new DataView(e,0,Zo),n=new TextDecoder;if(this.header={magic:n.decode(new Uint8Array(e.slice(0,4))),version:t.getUint32(4,!0),length:t.getUint32(8,!0)},this.header.magic!==Qf)throw new Error("THREE.GLTFLoader: Unsupported glTF-Binary header.");if(this.header.version<2)throw new Error("THREE.GLTFLoader: Legacy binary file detected.");let i=this.header.length-Zo,r=new DataView(e,Zo),o=0;for(;o<i;){let a=r.getUint32(o,!0);o+=4;let l=r.getUint32(o,!0);if(o+=4,l===Kf.JSON){let c=new Uint8Array(e,Zo+o,a);this.content=n.decode(c)}else if(l===Kf.BIN){let c=Zo+o;this.body=e.slice(c,c+a)}o+=a}if(this.content===null)throw new Error("THREE.GLTFLoader: JSON content not found.")}},xu=class{constructor(e,t){if(!t)throw new Error("THREE.GLTFLoader: No DRACOLoader instance provided.");this.name=mt.KHR_DRACO_MESH_COMPRESSION,this.json=e,this.dracoLoader=t,this.dracoLoader.preload()}decodePrimitive(e,t){let n=this.json,i=this.dracoLoader,r=e.extensions[this.name].bufferView,o=e.extensions[this.name].attributes,a={},l={},c={};for(let h in o){let u=Mu[h]||h.toLowerCase();a[u]=o[h]}for(let h in e.attributes){let u=Mu[h]||h.toLowerCase();if(o[h]!==void 0){let d=n.accessors[e.attributes[h]],f=Dr[d.componentType];c[u]=f.name,l[u]=d.normalized===!0}}return t.getDependency("bufferView",r).then(function(h){return new Promise(function(u,d){i.decodeDracoFile(h,function(f){for(let g in f.attributes){let v=f.attributes[g],m=l[g];m!==void 0&&(v.normalized=m)}u(f)},a,c,mn,d)})})}},_u=class{constructor(){this.name=mt.KHR_TEXTURE_TRANSFORM}extendTexture(e,t){if((t.texCoord===void 0||t.texCoord===e.channel)&&t.offset===void 0&&t.rotation===void 0&&t.scale===void 0)return e;if(e=e.clone(),t.texCoord!==void 0&&(e.channel=t.texCoord),t.offset!==void 0&&e.offset.fromArray(t.offset),t.rotation!==void 0&&(e.rotation=t.rotation),t.scale!==void 0&&e.repeat.fromArray(t.scale),t.rotation!==void 0){let n=Math.cos(e.rotation),i=Math.sin(e.rotation);e.matrix.set(e.repeat.x*n,e.repeat.y*i,e.offset.x,-e.repeat.x*i,e.repeat.y*n,e.offset.y,0,0,1),e.matrixAutoUpdate=!1}return e.needsUpdate=!0,e}},vu=class{constructor(){this.name=mt.KHR_MESH_QUANTIZATION}},uc=class extends fi{constructor(e,t,n,i){super(e,t,n,i)}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i*3+i;for(let o=0;o!==i;o++)t[o]=n[r+o];return t}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=a*2,c=a*3,h=i-t,u=(n-t)/h,d=u*u,f=d*u,g=e*c,v=g-c,m=-2*f+3*d,p=f-d,M=1-m,T=p-d+u;for(let x=0;x!==a;x++){let y=o[v+x+a],S=o[v+x+l]*h,C=o[g+x+a],_=o[g+x]*h;r[x]=M*y+T*S+m*C+p*_}return r}},Jv=new Ht,yu=class extends uc{interpolate_(e,t,n,i){let r=super.interpolate_(e,t,n,i);return Jv.fromArray(r).normalize().toArray(r),r}},Hn={FLOAT:5126,FLOAT_MAT3:35675,FLOAT_MAT4:35676,FLOAT_VEC2:35664,FLOAT_VEC3:35665,FLOAT_VEC4:35666,LINEAR:9729,REPEAT:10497,SAMPLER_2D:35678,POINTS:0,LINES:1,LINE_LOOP:2,LINE_STRIP:3,TRIANGLES:4,TRIANGLE_STRIP:5,TRIANGLE_FAN:6,UNSIGNED_BYTE:5121,UNSIGNED_SHORT:5123},Dr={5120:Int8Array,5121:Uint8Array,5122:Int16Array,5123:Uint16Array,5125:Uint32Array,5126:Float32Array},jf={9728:Vt,9729:Ot,9984:gl,9985:wr,9986:Is,9987:$n},Jf={33071:Fn,33648:nr,10497:ai},jh={SCALAR:1,VEC2:2,VEC3:3,VEC4:4,MAT2:4,MAT3:9,MAT4:16},Mu={POSITION:"position",NORMAL:"normal",TANGENT:"tangent",TEXCOORD_0:"uv",TEXCOORD_1:"uv1",TEXCOORD_2:"uv2",TEXCOORD_3:"uv3",COLOR_0:"color",WEIGHTS_0:"skinWeight",JOINTS_0:"skinIndex"},os={scale:"scale",translation:"position",rotation:"quaternion",weights:"morphTargetInfluences"},$v={CUBICSPLINE:void 0,LINEAR:ys,STEP:vs},Jh={OPAQUE:"OPAQUE",MASK:"MASK",BLEND:"BLEND"};function Qv(s){return s.DefaultMaterial===void 0&&(s.DefaultMaterial=new vn({color:16777215,emissive:0,metalness:1,roughness:1,transparent:!1,depthTest:!0,side:Bn})),s.DefaultMaterial}function Ns(s,e,t){for(let n in t.extensions)s[n]===void 0&&(e.userData.gltfExtensions=e.userData.gltfExtensions||{},e.userData.gltfExtensions[n]=t.extensions[n])}function _i(s,e){e.extras!==void 0&&(typeof e.extras=="object"?Object.assign(s.userData,e.extras):console.warn("THREE.GLTFLoader: Ignoring primitive type .extras, "+e.extras))}function ey(s,e,t){let n=!1,i=!1,r=!1;for(let c=0,h=e.length;c<h;c++){let u=e[c];if(u.POSITION!==void 0&&(n=!0),u.NORMAL!==void 0&&(i=!0),u.COLOR_0!==void 0&&(r=!0),n&&i&&r)break}if(!n&&!i&&!r)return Promise.resolve(s);let o=[],a=[],l=[];for(let c=0,h=e.length;c<h;c++){let u=e[c];if(n){let d=u.POSITION!==void 0?t.getDependency("accessor",u.POSITION):s.attributes.position;o.push(d)}if(i){let d=u.NORMAL!==void 0?t.getDependency("accessor",u.NORMAL):s.attributes.normal;a.push(d)}if(r){let d=u.COLOR_0!==void 0?t.getDependency("accessor",u.COLOR_0):s.attributes.color;l.push(d)}}return Promise.all([Promise.all(o),Promise.all(a),Promise.all(l)]).then(function(c){let h=c[0],u=c[1],d=c[2];return n&&(s.morphAttributes.position=h),i&&(s.morphAttributes.normal=u),r&&(s.morphAttributes.color=d),s.morphTargetsRelative=!0,s})}function ty(s,e){if(s.updateMorphTargets(),e.weights!==void 0)for(let t=0,n=e.weights.length;t<n;t++)s.morphTargetInfluences[t]=e.weights[t];if(e.extras&&Array.isArray(e.extras.targetNames)){let t=e.extras.targetNames;if(s.morphTargetInfluences.length===t.length){s.morphTargetDictionary={};for(let n=0,i=t.length;n<i;n++)s.morphTargetDictionary[t[n]]=n}else console.warn("THREE.GLTFLoader: Invalid extras.targetNames length. Ignoring names.")}}function ny(s){let e,t=s.extensions&&s.extensions[mt.KHR_DRACO_MESH_COMPRESSION];if(t?e="draco:"+t.bufferView+":"+t.indices+":"+$h(t.attributes):e=s.indices+":"+$h(s.attributes)+":"+s.mode,s.targets!==void 0)for(let n=0,i=s.targets.length;n<i;n++)e+=":"+$h(s.targets[n]);return e}function $h(s){let e="",t=Object.keys(s).sort();for(let n=0,i=t.length;n<i;n++)e+=t[n]+":"+s[t[n]]+";";return e}function bu(s){switch(s){case Int8Array:return 1/127;case Uint8Array:return 1/255;case Int16Array:return 1/32767;case Uint16Array:return 1/65535;default:throw new Error("THREE.GLTFLoader: Unsupported normalized accessor component type.")}}function iy(s){return s.search(/\.jpe?g($|\?)/i)>0||s.search(/^data\:image\/jpeg/)===0?"image/jpeg":s.search(/\.webp($|\?)/i)>0||s.search(/^data\:image\/webp/)===0?"image/webp":s.search(/\.ktx2($|\?)/i)>0||s.search(/^data\:image\/ktx2/)===0?"image/ktx2":"image/png"}var sy=new it,Su=class{constructor(e={},t={}){this.json=e,this.extensions={},this.plugins={},this.options=t,this.cache=new jv,this.associations=new Map,this.primitiveCache={},this.nodeCache={},this.meshCache={refs:{},uses:{}},this.cameraCache={refs:{},uses:{}},this.lightCache={refs:{},uses:{}},this.sourceCache={},this.textureCache={},this.nodeNamesUsed={};let n=!1,i=-1,r=!1,o=-1;if(typeof navigator<"u"&&typeof navigator.userAgent<"u"){let a=navigator.userAgent;n=/^((?!chrome|android).)*safari/i.test(a)===!0;let l=a.match(/Version\/(\d+)/);i=n&&l?parseInt(l[1],10):-1,r=a.indexOf("Firefox")>-1,o=r?a.match(/Firefox\/([0-9]+)\./)[1]:-1}typeof createImageBitmap>"u"||n&&i<17||r&&o<98?this.textureLoader=new yo(this.options.manager):this.textureLoader=new wo(this.options.manager),this.textureLoader.setCrossOrigin(this.options.crossOrigin),this.textureLoader.setRequestHeader(this.options.requestHeader),this.fileLoader=new vr(this.options.manager),this.fileLoader.setResponseType("arraybuffer"),this.options.crossOrigin==="use-credentials"&&this.fileLoader.setWithCredentials(!0)}setExtensions(e){this.extensions=e}setPlugins(e){this.plugins=e}parse(e,t){let n=this,i=this.json,r=this.extensions;this.cache.removeAll(),this.nodeCache={},this._invokeAll(function(o){return o._markDefs&&o._markDefs()}),Promise.all(this._invokeAll(function(o){return o.beforeRoot&&o.beforeRoot()})).then(function(){return Promise.all([n.getDependencies("scene"),n.getDependencies("animation"),n.getDependencies("camera")])}).then(function(o){let a={scene:o[0][i.scene||0],scenes:o[0],animations:o[1],cameras:o[2],asset:i.asset,parser:n,userData:{}};return Ns(r,a,i),_i(a,i),Promise.all(n._invokeAll(function(l){return l.afterRoot&&l.afterRoot(a)})).then(function(){for(let l of a.scenes)l.updateMatrixWorld();e(a)})}).catch(t)}_markDefs(){let e=this.json.nodes||[],t=this.json.skins||[],n=this.json.meshes||[];for(let i=0,r=t.length;i<r;i++){let o=t[i].joints;for(let a=0,l=o.length;a<l;a++)e[o[a]].isBone=!0}for(let i=0,r=e.length;i<r;i++){let o=e[i];o.mesh!==void 0&&(this._addNodeRef(this.meshCache,o.mesh),o.skin!==void 0&&(n[o.mesh].isSkinnedMesh=!0)),o.camera!==void 0&&this._addNodeRef(this.cameraCache,o.camera)}}_addNodeRef(e,t){t!==void 0&&(e.refs[t]===void 0&&(e.refs[t]=e.uses[t]=0),e.refs[t]++)}_getNodeRef(e,t,n){if(e.refs[t]<=1)return n;let i=n.clone(),r=(o,a)=>{let l=this.associations.get(o);l!=null&&this.associations.set(a,l);for(let[c,h]of o.children.entries())r(h,a.children[c])};return r(n,i),i.name+="_instance_"+e.uses[t]++,i}_invokeOne(e){let t=Object.values(this.plugins);t.push(this);for(let n=0;n<t.length;n++){let i=e(t[n]);if(i)return i}return null}_invokeAll(e){let t=Object.values(this.plugins);t.unshift(this);let n=[];for(let i=0;i<t.length;i++){let r=e(t[i]);r&&n.push(r)}return n}getDependency(e,t){let n=e+":"+t,i=this.cache.get(n);if(!i){switch(e){case"scene":i=this.loadScene(t);break;case"node":i=this._invokeOne(function(r){return r.loadNode&&r.loadNode(t)});break;case"mesh":i=this._invokeOne(function(r){return r.loadMesh&&r.loadMesh(t)});break;case"accessor":i=this.loadAccessor(t);break;case"bufferView":i=this._invokeOne(function(r){return r.loadBufferView&&r.loadBufferView(t)});break;case"buffer":i=this.loadBuffer(t);break;case"material":i=this._invokeOne(function(r){return r.loadMaterial&&r.loadMaterial(t)});break;case"texture":i=this._invokeOne(function(r){return r.loadTexture&&r.loadTexture(t)});break;case"skin":i=this.loadSkin(t);break;case"animation":i=this._invokeOne(function(r){return r.loadAnimation&&r.loadAnimation(t)});break;case"camera":i=this.loadCamera(t);break;default:if(i=this._invokeOne(function(r){return r!=this&&r.getDependency&&r.getDependency(e,t)}),!i)throw new Error("Unknown type: "+e);break}this.cache.add(n,i)}return i}getDependencies(e){let t=this.cache.get(e);if(!t){let n=this,i=this.json[e+(e==="mesh"?"es":"s")]||[];t=Promise.all(i.map(function(r,o){return n.getDependency(e,o)})),this.cache.add(e,t)}return t}loadBuffer(e){let t=this.json.buffers[e],n=this.fileLoader;if(t.type&&t.type!=="arraybuffer")throw new Error("THREE.GLTFLoader: "+t.type+" buffer type is not supported.");if(t.uri===void 0&&e===0)return Promise.resolve(this.extensions[mt.KHR_BINARY_GLTF].body);let i=this.options;return new Promise(function(r,o){n.load(Ui.resolveURL(t.uri,i.path),r,void 0,function(){o(new Error('THREE.GLTFLoader: Failed to load buffer "'+t.uri+'".'))})})}loadBufferView(e){let t=this.json.bufferViews[e];return this.getDependency("buffer",t.buffer).then(function(n){let i=t.byteLength||0,r=t.byteOffset||0;return n.slice(r,r+i)})}loadAccessor(e){let t=this,n=this.json,i=this.json.accessors[e];if(i.bufferView===void 0&&i.sparse===void 0){let o=jh[i.type],a=Dr[i.componentType],l=i.normalized===!0,c=new a(i.count*o);return Promise.resolve(new dt(c,o,l))}let r=[];return i.bufferView!==void 0?r.push(this.getDependency("bufferView",i.bufferView)):r.push(null),i.sparse!==void 0&&(r.push(this.getDependency("bufferView",i.sparse.indices.bufferView)),r.push(this.getDependency("bufferView",i.sparse.values.bufferView))),Promise.all(r).then(function(o){let a=o[0],l=jh[i.type],c=Dr[i.componentType],h=c.BYTES_PER_ELEMENT,u=h*l,d=i.byteOffset||0,f=i.bufferView!==void 0?n.bufferViews[i.bufferView].byteStride:void 0,g=i.normalized===!0,v,m;if(f&&f!==u){let p=Math.floor(d/f),M="InterleavedBuffer:"+i.bufferView+":"+i.componentType+":"+p+":"+i.count,T=t.cache.get(M);T||(v=new c(a,p*f,i.count*f/h),T=new cr(v,f/h),t.cache.add(M,T)),m=new hr(T,l,d%f/h,g)}else a===null?v=new c(i.count*l):v=new c(a,d,i.count*l),m=new dt(v,l,g);if(i.sparse!==void 0){let p=jh.SCALAR,M=Dr[i.sparse.indices.componentType],T=i.sparse.indices.byteOffset||0,x=i.sparse.values.byteOffset||0,y=new M(o[1],T,i.sparse.count*p),S=new c(o[2],x,i.sparse.count*l);a!==null&&(m=new dt(m.array.slice(),m.itemSize,m.normalized)),m.normalized=!1;for(let C=0,_=y.length;C<_;C++){let I=y[C];if(m.setX(I,S[C*l]),l>=2&&m.setY(I,S[C*l+1]),l>=3&&m.setZ(I,S[C*l+2]),l>=4&&m.setW(I,S[C*l+3]),l>=5)throw new Error("THREE.GLTFLoader: Unsupported itemSize in sparse BufferAttribute.")}m.normalized=g}return m})}loadTexture(e){let t=this.json,n=this.options,r=t.textures[e].source,o=t.images[r],a=this.textureLoader;if(o.uri){let l=n.manager.getHandler(o.uri);l!==null&&(a=l)}return this.loadTextureImage(e,r,a)}loadTextureImage(e,t,n){let i=this,r=this.json,o=r.textures[e],a=r.images[t],l=(a.uri||a.bufferView)+":"+o.sampler;if(this.textureCache[l])return this.textureCache[l];let c=this.loadImageSource(t,n).then(function(h){h.flipY=!1,h.name=o.name||a.name||"",h.name===""&&typeof a.uri=="string"&&a.uri.startsWith("data:image/")===!1&&(h.name=a.uri);let d=(r.samplers||{})[o.sampler]||{};return h.magFilter=jf[d.magFilter]||Ot,h.minFilter=jf[d.minFilter]||$n,h.wrapS=Jf[d.wrapS]||ai,h.wrapT=Jf[d.wrapT]||ai,h.generateMipmaps=!h.isCompressedTexture&&h.minFilter!==Vt&&h.minFilter!==Ot,i.associations.set(h,{textures:e}),h}).catch(function(){return null});return this.textureCache[l]=c,c}loadImageSource(e,t){let n=this,i=this.json,r=this.options;if(this.sourceCache[e]!==void 0)return this.sourceCache[e].then(u=>u.clone());let o=i.images[e],a=self.URL||self.webkitURL,l=o.uri||"",c=!1;if(o.bufferView!==void 0)l=n.getDependency("bufferView",o.bufferView).then(function(u){c=!0;let d=new Blob([u],{type:o.mimeType});return l=a.createObjectURL(d),l});else if(o.uri===void 0)throw new Error("THREE.GLTFLoader: Image "+e+" is missing URI and bufferView");let h=Promise.resolve(l).then(function(u){return new Promise(function(d,f){let g=d;t.isImageBitmapLoader===!0&&(g=function(v){let m=new Gt(v);m.needsUpdate=!0,d(m)}),t.load(Ui.resolveURL(u,r.path),g,void 0,f)})}).then(function(u){return c===!0&&a.revokeObjectURL(l),_i(u,o),u.userData.mimeType=o.mimeType||iy(o.uri),u}).catch(function(u){throw console.error("THREE.GLTFLoader: Couldn't load texture",l),u});return this.sourceCache[e]=h,h}assignTexture(e,t,n,i){let r=this;return this.getDependency("texture",n.index).then(function(o){if(!o)return null;if(n.texCoord!==void 0&&n.texCoord>0&&(o=o.clone(),o.channel=n.texCoord),r.extensions[mt.KHR_TEXTURE_TRANSFORM]){let a=n.extensions!==void 0?n.extensions[mt.KHR_TEXTURE_TRANSFORM]:void 0;if(a){let l=r.associations.get(o);o=r.extensions[mt.KHR_TEXTURE_TRANSFORM].extendTexture(o,a),r.associations.set(o,l)}}return i!==void 0&&(o.colorSpace=i),e[t]=o,o})}assignFinalMaterial(e){let t=e.geometry,n=e.material,i=t.attributes.tangent===void 0,r=t.attributes.color!==void 0,o=t.attributes.normal===void 0;if(e.isPoints){let a="PointsMaterial:"+n.uuid,l=this.cache.get(a);l||(l=new fr,gn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,l.sizeAttenuation=!1,this.cache.add(a,l)),n=l}else if(e.isLine){let a="LineBasicMaterial:"+n.uuid,l=this.cache.get(a);l||(l=new hi,gn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,this.cache.add(a,l)),n=l}if(i||r||o){let a="ClonedMaterial:"+n.uuid+":";i&&(a+="derivative-tangents:"),r&&(a+="vertex-colors:"),o&&(a+="flat-shading:");let l=this.cache.get(a);l||(l=n.clone(),r&&(l.vertexColors=!0),o&&(l.flatShading=!0),i&&(l.normalScale&&(l.normalScale.y*=-1),l.clearcoatNormalScale&&(l.clearcoatNormalScale.y*=-1)),this.cache.add(a,l),this.associations.set(l,this.associations.get(n))),n=l}e.material=n}getMaterialType(){return vn}loadMaterial(e){let t=this,n=this.json,i=this.extensions,r=n.materials[e],o,a={},l=r.extensions||{},c=[];if(l[mt.KHR_MATERIALS_UNLIT]){let u=i[mt.KHR_MATERIALS_UNLIT];o=u.getMaterialType(),c.push(u.extendParams(a,r,t))}else{let u=r.pbrMetallicRoughness||{};if(a.color=new Ce(1,1,1),a.opacity=1,Array.isArray(u.baseColorFactor)){let d=u.baseColorFactor;a.color.setRGB(d[0],d[1],d[2],mn),a.opacity=d[3]}u.baseColorTexture!==void 0&&c.push(t.assignTexture(a,"map",u.baseColorTexture,Ft)),a.metalness=u.metallicFactor!==void 0?u.metallicFactor:1,a.roughness=u.roughnessFactor!==void 0?u.roughnessFactor:1,u.metallicRoughnessTexture!==void 0&&(c.push(t.assignTexture(a,"metalnessMap",u.metallicRoughnessTexture)),c.push(t.assignTexture(a,"roughnessMap",u.metallicRoughnessTexture))),o=this._invokeOne(function(d){return d.getMaterialType&&d.getMaterialType(e)}),c.push(Promise.all(this._invokeAll(function(d){return d.extendMaterialParams&&d.extendMaterialParams(e,a)})))}r.doubleSided===!0&&(a.side=Rt);let h=r.alphaMode||Jh.OPAQUE;if(h===Jh.BLEND?(a.transparent=!0,a.depthWrite=!1):(a.transparent=!1,h===Jh.MASK&&(a.alphaTest=r.alphaCutoff!==void 0?r.alphaCutoff:.5)),r.normalTexture!==void 0&&o!==wt&&(c.push(t.assignTexture(a,"normalMap",r.normalTexture)),a.normalScale=new Ue(1,1),r.normalTexture.scale!==void 0)){let u=r.normalTexture.scale;a.normalScale.set(u,u)}if(r.occlusionTexture!==void 0&&o!==wt&&(c.push(t.assignTexture(a,"aoMap",r.occlusionTexture)),r.occlusionTexture.strength!==void 0&&(a.aoMapIntensity=r.occlusionTexture.strength)),r.emissiveFactor!==void 0&&o!==wt){let u=r.emissiveFactor;a.emissive=new Ce().setRGB(u[0],u[1],u[2],mn)}return r.emissiveTexture!==void 0&&o!==wt&&c.push(t.assignTexture(a,"emissiveMap",r.emissiveTexture,Ft)),Promise.all(c).then(function(){let u=new o(a);return r.name&&(u.name=r.name),_i(u,r),t.associations.set(u,{materials:e}),r.extensions&&Ns(i,u,r),u})}createUniqueName(e){let t=Dt.sanitizeNodeName(e||"");return t in this.nodeNamesUsed?t+"_"+ ++this.nodeNamesUsed[t]:(this.nodeNamesUsed[t]=0,t)}loadGeometries(e){let t=this,n=this.extensions,i=this.primitiveCache;function r(a){return n[mt.KHR_DRACO_MESH_COMPRESSION].decodePrimitive(a,t).then(function(l){return $f(l,a,t)})}let o=[];for(let a=0,l=e.length;a<l;a++){let c=e[a],h=ny(c),u=i[h];if(u)o.push(u.promise);else{let d;c.extensions&&c.extensions[mt.KHR_DRACO_MESH_COMPRESSION]?d=r(c):d=$f(new ot,c,t),c.mode===Hn.TRIANGLE_STRIP?d=d.then(f=>Kh(f,Go)):c.mode===Hn.TRIANGLE_FAN&&(d=d.then(f=>Kh(f,Ar))),i[h]={primitive:c,promise:d},o.push(d)}}return Promise.all(o)}loadMesh(e){let t=this,n=this.json,i=this.extensions,r=n.meshes[e],o=r.primitives,a=[];for(let l=0,c=o.length;l<c;l++){let h=o[l].material===void 0?Qv(this.cache):this.getDependency("material",o[l].material);a.push(h)}return a.push(t.loadGeometries(o)),Promise.all(a).then(async function(l){let c=l.slice(0,l.length-1),h=l[l.length-1],u=[];for(let f=0,g=h.length;f<g;f++){let v=h[f],m=o[f],p,M=c[f];if(m.mode===Hn.TRIANGLES||m.mode===Hn.TRIANGLE_STRIP||m.mode===Hn.TRIANGLE_FAN||m.mode===void 0){let T=r.isSkinnedMesh===!0,x=v.hasAttribute("skinIndex")&&v.hasAttribute("skinWeight");T&&x===!1&&console.warn("THREE.GLTFLoader: Missing skinIndex or skinWeight attributes. Skinning disabled."),p=T&&x?new ao(v,M):new Ke(v,M),p.isSkinnedMesh===!0&&p.normalizeSkinWeights()}else if(m.mode===Hn.LINES)p=new Ss(v,M);else if(m.mode===Hn.LINE_STRIP)p=new Ci(v,M);else if(m.mode===Hn.LINE_LOOP)p=new co(v,M);else if(m.mode===Hn.POINTS)p=new tn(v,M);else throw new Error("THREE.GLTFLoader: Primitive mode unsupported: "+m.mode);Object.keys(p.geometry.morphAttributes).length>0&&ty(p,r),p.name=t.createUniqueName(r.name||"mesh_"+e),_i(p,r),m.extensions&&Ns(i,p,m),t.assignFinalMaterial(p),u.push(p)}for(let f=0,g=u.length;f<g;f++)t.associations.set(u[f],{meshes:e,primitives:f});if(u.length===1)return r.extensions&&Ns(i,u[0],r),u[0];let d=new lt;r.extensions&&Ns(i,d,r),t.associations.set(d,{meshes:e});for(let f=0,g=u.length;f<g;f++)d.add(u[f]);return d})}loadCamera(e){let t,n=this.json.cameras[e],i=n[n.type];if(!i){console.warn("THREE.GLTFLoader: Missing camera parameters.");return}return n.type==="perspective"?t=new qt(_t.radToDeg(i.yfov),i.aspectRatio||1,i.znear||1,i.zfar||2e6):n.type==="orthographic"&&(t=new mi(-i.xmag,i.xmag,i.ymag,-i.ymag,i.znear,i.zfar)),n.name&&(t.name=this.createUniqueName(n.name)),_i(t,n),Promise.resolve(t)}loadSkin(e){let t=this.json.skins[e],n=[];for(let i=0,r=t.joints.length;i<r;i++)n.push(this._loadNodeShallow(t.joints[i]));return t.inverseBindMatrices!==void 0?n.push(this.getDependency("accessor",t.inverseBindMatrices)):n.push(null),Promise.all(n).then(function(i){let r=i.pop(),o=i,a=[],l=[];for(let c=0,h=o.length;c<h;c++){let u=o[c];if(u){a.push(u);let d=new it;r!==null&&d.fromArray(r.array,c*16),l.push(d)}else console.warn('THREE.GLTFLoader: Joint "%s" could not be found.',t.joints[c])}return new lo(a,l)})}loadAnimation(e){let t=this.json,n=this,i=t.animations[e],r=i.name?i.name:"animation_"+e,o=[],a=[],l=[],c=[],h=[];for(let u=0,d=i.channels.length;u<d;u++){let f=i.channels[u],g=i.samplers[f.sampler],v=f.target,m=v.node,p=i.parameters!==void 0?i.parameters[g.input]:g.input,M=i.parameters!==void 0?i.parameters[g.output]:g.output;v.node!==void 0&&(o.push(this.getDependency("node",m)),a.push(this.getDependency("accessor",p)),l.push(this.getDependency("accessor",M)),c.push(g),h.push(v))}return Promise.all([Promise.all(o),Promise.all(a),Promise.all(l),Promise.all(c),Promise.all(h)]).then(function(u){let d=u[0],f=u[1],g=u[2],v=u[3],m=u[4],p=[];for(let T=0,x=d.length;T<x;T++){let y=d[T],S=f[T],C=g[T],_=v[T],I=m[T];if(y===void 0)continue;y.updateMatrix&&y.updateMatrix();let b=n._createAnimationTracks(y,S,C,_,I);if(b)for(let N=0;N<b.length;N++)p.push(b[N])}let M=new Es(r,void 0,p);return _i(M,i),M})}createNodeMesh(e){let t=this.json,n=this,i=t.nodes[e];return i.mesh===void 0?null:n.getDependency("mesh",i.mesh).then(function(r){let o=n._getNodeRef(n.meshCache,i.mesh,r);return i.weights!==void 0&&o.traverse(function(a){if(a.isMesh)for(let l=0,c=i.weights.length;l<c;l++)a.morphTargetInfluences[l]=i.weights[l]}),o})}loadNode(e){let t=this.json,n=this,i=t.nodes[e],r=n._loadNodeShallow(e),o=[],a=i.children||[];for(let c=0,h=a.length;c<h;c++)o.push(n.getDependency("node",a[c]));let l=i.skin===void 0?Promise.resolve(null):n.getDependency("skin",i.skin);return Promise.all([r,Promise.all(o),l]).then(function(c){let h=c[0],u=c[1],d=c[2];d!==null&&h.traverse(function(f){f.isSkinnedMesh&&f.bind(d,sy)});for(let f=0,g=u.length;f<g;f++)h.add(u[f]);if(h.userData.pivot!==void 0&&u.length>0){let f=h.userData.pivot,g=u[0];h.pivot=new W().fromArray(f),h.position.x-=f[0],h.position.y-=f[1],h.position.z-=f[2],g.position.set(0,0,0),delete h.userData.pivot}return h})}_loadNodeShallow(e){let t=this.json,n=this.extensions,i=this;if(this.nodeCache[e]!==void 0)return this.nodeCache[e];let r=t.nodes[e],o=r.name?i.createUniqueName(r.name):"",a=[],l=i._invokeOne(function(c){return c.createNodeMesh&&c.createNodeMesh(e)});return l&&a.push(l),r.camera!==void 0&&a.push(i.getDependency("camera",r.camera).then(function(c){return i._getNodeRef(i.cameraCache,r.camera,c)})),i._invokeAll(function(c){return c.createNodeAttachment&&c.createNodeAttachment(e)}).forEach(function(c){a.push(c)}),this.nodeCache[e]=Promise.all(a).then(function(c){let h;if(r.isBone===!0?h=new ur:c.length>1?h=new lt:c.length===1?h=c[0]:h=new Tt,h!==c[0])for(let u=0,d=c.length;u<d;u++)h.add(c[u]);if(r.name&&(h.userData.name=r.name,h.name=o),_i(h,r),r.extensions&&Ns(n,h,r),r.matrix!==void 0){let u=new it;u.fromArray(r.matrix),h.applyMatrix4(u)}else r.translation!==void 0&&h.position.fromArray(r.translation),r.rotation!==void 0&&h.quaternion.fromArray(r.rotation),r.scale!==void 0&&h.scale.fromArray(r.scale);if(!i.associations.has(h))i.associations.set(h,{});else if(r.mesh!==void 0&&i.meshCache.refs[r.mesh]>1){let u=i.associations.get(h);i.associations.set(h,{...u})}return i.associations.get(h).nodes=e,h}),this.nodeCache[e]}loadScene(e){let t=this.extensions,n=this.json.scenes[e],i=this,r=new lt;n.name&&(r.name=i.createUniqueName(n.name)),_i(r,n),n.extensions&&Ns(t,r,n);let o=n.nodes||[],a=[];for(let l=0,c=o.length;l<c;l++)a.push(i.getDependency("node",o[l]));return Promise.all(a).then(function(l){for(let h=0,u=l.length;h<u;h++){let d=l[h];d.parent!==null?r.add(Yf(d)):r.add(d)}let c=h=>{let u=new Map;for(let[d,f]of i.associations)(d instanceof gn||d instanceof Gt)&&u.set(d,f);return h.traverse(d=>{let f=i.associations.get(d);f!=null&&u.set(d,f)}),u};return i.associations=c(r),r})}_createAnimationTracks(e,t,n,i,r){let o=[],a=e.name?e.name:e.uuid,l=[];function c(f){f.morphTargetInfluences&&l.push(f.name?f.name:f.uuid)}os[r.path]===os.weights?(c(e),e.isGroup&&e.children.forEach(c)):l.push(a);let h;switch(os[r.path]){case os.weights:h=Ii;break;case os.rotation:h=Li;break;case os.translation:case os.scale:h=es;break;default:n.itemSize===1?h=Ii:h=es;break}let u=i.interpolation!==void 0?$v[i.interpolation]:ys,d=this._getArrayFromAccessor(n);for(let f=0,g=l.length;f<g;f++){let v=new h(l[f]+"."+os[r.path],t.array,d,u);i.interpolation==="CUBICSPLINE"&&this._createCubicSplineTrackInterpolant(v),o.push(v)}return o}_getArrayFromAccessor(e){let t=e.array;if(e.normalized){let n=bu(t.constructor),i=new Float32Array(t.length);for(let r=0,o=t.length;r<o;r++)i[r]=t[r]*n;t=i}return t}_createCubicSplineTrackInterpolant(e){e.createInterpolant=function(n){let i=this instanceof Li?yu:uc;return new i(this.times,this.values,this.getValueSize()/3,n)},e.createInterpolant.isInterpolantFactoryMethodGLTFCubicSpline=!0}};function ry(s,e,t){let n=e.attributes,i=new Zt;if(n.POSITION!==void 0){let a=t.json.accessors[n.POSITION],l=a.min,c=a.max;if(l!==void 0&&c!==void 0){if(i.set(new W(l[0],l[1],l[2]),new W(c[0],c[1],c[2])),a.normalized){let h=bu(Dr[a.componentType]);i.min.multiplyScalar(h),i.max.multiplyScalar(h)}}else{console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.");return}}else return;let r=e.targets;if(r!==void 0){let a=new W,l=new W;for(let c=0,h=r.length;c<h;c++){let u=r[c];if(u.POSITION!==void 0){let d=t.json.accessors[u.POSITION],f=d.min,g=d.max;if(f!==void 0&&g!==void 0){if(l.setX(Math.max(Math.abs(f[0]),Math.abs(g[0]))),l.setY(Math.max(Math.abs(f[1]),Math.abs(g[1]))),l.setZ(Math.max(Math.abs(f[2]),Math.abs(g[2]))),d.normalized){let v=bu(Dr[d.componentType]);l.multiplyScalar(v)}a.max(l)}else console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.")}}i.expandByVector(a)}s.boundingBox=i;let o=new cn;i.getCenter(o.center),o.radius=i.min.distanceTo(i.max)/2,s.boundingSphere=o}function $f(s,e,t){let n=e.attributes,i=[];function r(o,a){return t.getDependency("accessor",o).then(function(l){s.setAttribute(a,l)})}for(let o in n){let a=Mu[o]||o.toLowerCase();a in s.attributes||i.push(r(n[o],a))}if(e.indices!==void 0&&!s.index){let o=t.getDependency("accessor",e.indices).then(function(a){s.setIndex(a)});i.push(o)}return ht.workingColorSpace!==mn&&"COLOR_0"in n&&console.warn(`THREE.GLTFLoader: Converting vertex colors from "srgb-linear" to "${ht.workingColorSpace}" not supported.`),_i(s,e),ry(s,e,t),Promise.all(i).then(function(){return e.targets!==void 0?ey(s,e.targets,t):s})}var ep={type:"change"},Tu={type:"start"},np={type:"end"},dc=new ci,tp=new An,oy=Math.cos(70*_t.DEG2RAD),$t=new W,Sn=2*Math.PI,Ct={NONE:-1,ROTATE:0,DOLLY:1,PAN:2,TOUCH_ROTATE:3,TOUCH_PAN:4,TOUCH_DOLLY_PAN:5,TOUCH_DOLLY_ROTATE:6},wu=1e-6,fc=class extends Ro{constructor(e,t=null){super(e,t),this.state=Ct.NONE,this.target=new W,this.cursor=new W,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.minTargetRadius=0,this.maxTargetRadius=1/0,this.minPolarAngle=0,this.maxPolarAngle=Math.PI,this.minAzimuthAngle=-1/0,this.maxAzimuthAngle=1/0,this.enableDamping=!1,this.dampingFactor=.05,this.enableZoom=!0,this.zoomSpeed=1,this.enableRotate=!0,this.rotateSpeed=1,this.keyRotateSpeed=1,this.enablePan=!0,this.panSpeed=1,this.screenSpacePanning=!0,this.keyPanSpeed=7,this.zoomToCursor=!1,this.autoRotate=!1,this.autoRotateSpeed=2,this.keys={LEFT:"ArrowLeft",UP:"ArrowUp",RIGHT:"ArrowRight",BOTTOM:"ArrowDown"},this.mouseButtons={LEFT:ts.ROTATE,MIDDLE:ts.DOLLY,RIGHT:ts.PAN},this.touches={ONE:ns.ROTATE,TWO:ns.DOLLY_PAN},this.target0=this.target.clone(),this.position0=this.object.position.clone(),this.zoom0=this.object.zoom,this._cursorStyle="auto",this._domElementKeyEvents=null,this._lastPosition=new W,this._lastQuaternion=new Ht,this._lastTargetPosition=new W,this._quat=new Ht().setFromUnitVectors(e.up,new W(0,1,0)),this._quatInverse=this._quat.clone().invert(),this._spherical=new Mr,this._sphericalDelta=new Mr,this._scale=1,this._panOffset=new W,this._rotateStart=new Ue,this._rotateEnd=new Ue,this._rotateDelta=new Ue,this._panStart=new Ue,this._panEnd=new Ue,this._panDelta=new Ue,this._dollyStart=new Ue,this._dollyEnd=new Ue,this._dollyDelta=new Ue,this._dollyDirection=new W,this._mouse=new Ue,this._performCursorZoom=!1,this._pointers=[],this._pointerPositions={},this._controlActive=!1,this._onPointerMove=ly.bind(this),this._onPointerDown=ay.bind(this),this._onPointerUp=cy.bind(this),this._onContextMenu=gy.bind(this),this._onMouseWheel=dy.bind(this),this._onKeyDown=fy.bind(this),this._onTouchStart=py.bind(this),this._onTouchMove=my.bind(this),this._onMouseDown=hy.bind(this),this._onMouseMove=uy.bind(this),this._interceptControlDown=xy.bind(this),this._interceptControlUp=_y.bind(this),this.domElement!==null&&this.connect(this.domElement),this.update()}set cursorStyle(e){this._cursorStyle=e,e==="grab"?this.domElement.style.cursor="grab":this.domElement.style.cursor="auto"}get cursorStyle(){return this._cursorStyle}connect(e){super.connect(e),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerUp),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.getRootNode().addEventListener("keydown",this._interceptControlDown,{passive:!0,capture:!0}),this.domElement.style.touchAction="none"}disconnect(){this.state=Ct.NONE,this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerUp),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.stopListenToKeyEvents();let e=this.domElement.getRootNode();e.removeEventListener("keydown",this._interceptControlDown,{capture:!0}),e.removeEventListener("keyup",this._interceptControlUp,{capture:!0}),this._controlActive=!1,this._pointers.length=0,this._pointerPositions={},this.domElement.style.touchAction="",this.domElement.style.cursor="auto"}dispose(){this.disconnect()}getPolarAngle(){return this._spherical.phi}getAzimuthalAngle(){return this._spherical.theta}getDistance(){return this.object.position.distanceTo(this.target)}listenToKeyEvents(e){e.addEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=e}stopListenToKeyEvents(){this._domElementKeyEvents!==null&&(this._domElementKeyEvents.removeEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=null)}saveState(){this.target0.copy(this.target),this.position0.copy(this.object.position),this.zoom0=this.object.zoom}reset(){this.target.copy(this.target0),this.object.position.copy(this.position0),this.object.zoom=this.zoom0,this.object.updateProjectionMatrix(),this.dispatchEvent(ep),this.update(),this.state=Ct.NONE}pan(e,t){this._pan(e,t),this.update()}dollyIn(e){this._dollyIn(e),this.update()}dollyOut(e){this._dollyOut(e),this.update()}rotateLeft(e){this._rotateLeft(e),this.update()}rotateUp(e){this._rotateUp(e),this.update()}update(e=null){let t=this.object.position;$t.copy(t).sub(this.target),$t.applyQuaternion(this._quat),this._spherical.setFromVector3($t),this.autoRotate&&this.state===Ct.NONE&&this._rotateLeft(this._getAutoRotationAngle(e)),this.enableDamping?(this._spherical.theta+=this._sphericalDelta.theta*this.dampingFactor,this._spherical.phi+=this._sphericalDelta.phi*this.dampingFactor):(this._spherical.theta+=this._sphericalDelta.theta,this._spherical.phi+=this._sphericalDelta.phi);let n=this.minAzimuthAngle,i=this.maxAzimuthAngle;isFinite(n)&&isFinite(i)&&(n<-Math.PI?n+=Sn:n>Math.PI&&(n-=Sn),i<-Math.PI?i+=Sn:i>Math.PI&&(i-=Sn),n<=i?this._spherical.theta=Math.max(n,Math.min(i,this._spherical.theta)):this._spherical.theta=this._spherical.theta>(n+i)/2?Math.max(n,this._spherical.theta):Math.min(i,this._spherical.theta)),this._spherical.phi=Math.max(this.minPolarAngle,Math.min(this.maxPolarAngle,this._spherical.phi)),this._spherical.makeSafe(),this.enableDamping===!0?this.target.addScaledVector(this._panOffset,this.dampingFactor):this.target.add(this._panOffset),this.target.sub(this.cursor),this.target.clampLength(this.minTargetRadius,this.maxTargetRadius),this.target.add(this.cursor);let r=!1;if(this.zoomToCursor&&this._performCursorZoom||this.object.isOrthographicCamera)this._spherical.radius=this._clampDistance(this._spherical.radius);else{let o=this._spherical.radius;this._spherical.radius=this._clampDistance(this._spherical.radius*this._scale),r=o!=this._spherical.radius}if($t.setFromSpherical(this._spherical),$t.applyQuaternion(this._quatInverse),t.copy(this.target).add($t),this.object.lookAt(this.target),this.enableDamping===!0?(this._sphericalDelta.theta*=1-this.dampingFactor,this._sphericalDelta.phi*=1-this.dampingFactor,this._panOffset.multiplyScalar(1-this.dampingFactor)):(this._sphericalDelta.set(0,0,0),this._panOffset.set(0,0,0)),this.zoomToCursor&&this._performCursorZoom){let o=null;if(this.object.isPerspectiveCamera){let a=$t.length();o=this._clampDistance(a*this._scale);let l=a-o;this.object.position.addScaledVector(this._dollyDirection,l),this.object.updateMatrixWorld(),r=!!l}else if(this.object.isOrthographicCamera){let a=new W(this._mouse.x,this._mouse.y,0);a.unproject(this.object);let l=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),this.object.updateProjectionMatrix(),r=l!==this.object.zoom;let c=new W(this._mouse.x,this._mouse.y,0);c.unproject(this.object),this.object.position.sub(c).add(a),this.object.updateMatrixWorld(),o=$t.length()}else console.warn("WARNING: OrbitControls.js encountered an unknown camera type - zoom to cursor disabled."),this.zoomToCursor=!1;o!==null&&(this.screenSpacePanning?this.target.set(0,0,-1).transformDirection(this.object.matrix).multiplyScalar(o).add(this.object.position):(dc.origin.copy(this.object.position),dc.direction.set(0,0,-1).transformDirection(this.object.matrix),Math.abs(this.object.up.dot(dc.direction))<oy?this.object.lookAt(this.target):(tp.setFromNormalAndCoplanarPoint(this.object.up,this.target),dc.intersectPlane(tp,this.target))))}else if(this.object.isOrthographicCamera){let o=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),o!==this.object.zoom&&(this.object.updateProjectionMatrix(),r=!0)}return this._scale=1,this._performCursorZoom=!1,r||this._lastPosition.distanceToSquared(this.object.position)>wu||8*(1-this._lastQuaternion.dot(this.object.quaternion))>wu||this._lastTargetPosition.distanceToSquared(this.target)>wu?(this.dispatchEvent(ep),this._lastPosition.copy(this.object.position),this._lastQuaternion.copy(this.object.quaternion),this._lastTargetPosition.copy(this.target),!0):!1}_getAutoRotationAngle(e){return e!==null?Sn/60*this.autoRotateSpeed*e:Sn/60/60*this.autoRotateSpeed}_getZoomScale(e){let t=Math.abs(e*.01);return Math.pow(.95,this.zoomSpeed*t)}_rotateLeft(e){this._sphericalDelta.theta-=e}_rotateUp(e){this._sphericalDelta.phi-=e}_panLeft(e,t){$t.setFromMatrixColumn(t,0),$t.multiplyScalar(-e),this._panOffset.add($t)}_panUp(e,t){this.screenSpacePanning===!0?$t.setFromMatrixColumn(t,1):($t.setFromMatrixColumn(t,0),$t.crossVectors(this.object.up,$t)),$t.multiplyScalar(e),this._panOffset.add($t)}_pan(e,t){let n=this.domElement;if(this.object.isPerspectiveCamera){let i=this.object.position;$t.copy(i).sub(this.target);let r=$t.length();r*=Math.tan(this.object.fov/2*Math.PI/180),this._panLeft(2*e*r/n.clientHeight,this.object.matrix),this._panUp(2*t*r/n.clientHeight,this.object.matrix)}else this.object.isOrthographicCamera?(this._panLeft(e*(this.object.right-this.object.left)/this.object.zoom/n.clientWidth,this.object.matrix),this._panUp(t*(this.object.top-this.object.bottom)/this.object.zoom/n.clientHeight,this.object.matrix)):(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - pan disabled."),this.enablePan=!1)}_dollyOut(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale/=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_dollyIn(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale*=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_updateZoomParameters(e,t){if(!this.zoomToCursor)return;this._performCursorZoom=!0;let n=this.domElement.getBoundingClientRect(),i=e-n.left,r=t-n.top,o=n.width,a=n.height;this._mouse.x=i/o*2-1,this._mouse.y=-(r/a)*2+1,this._dollyDirection.set(this._mouse.x,this._mouse.y,1).unproject(this.object).sub(this.object.position).normalize()}_clampDistance(e){return Math.max(this.minDistance,Math.min(this.maxDistance,e))}_handleMouseDownRotate(e){this._rotateStart.set(e.clientX,e.clientY)}_handleMouseDownDolly(e){this._updateZoomParameters(e.clientX,e.clientX),this._dollyStart.set(e.clientX,e.clientY)}_handleMouseDownPan(e){this._panStart.set(e.clientX,e.clientY)}_handleMouseMoveRotate(e){this._rotateEnd.set(e.clientX,e.clientY),this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Sn*this._rotateDelta.x/t.clientHeight),this._rotateUp(Sn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd),this.update()}_handleMouseMoveDolly(e){this._dollyEnd.set(e.clientX,e.clientY),this._dollyDelta.subVectors(this._dollyEnd,this._dollyStart),this._dollyDelta.y>0?this._dollyOut(this._getZoomScale(this._dollyDelta.y)):this._dollyDelta.y<0&&this._dollyIn(this._getZoomScale(this._dollyDelta.y)),this._dollyStart.copy(this._dollyEnd),this.update()}_handleMouseMovePan(e){this._panEnd.set(e.clientX,e.clientY),this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd),this.update()}_handleMouseWheel(e){this._updateZoomParameters(e.clientX,e.clientY),e.deltaY<0?this._dollyIn(this._getZoomScale(e.deltaY)):e.deltaY>0&&this._dollyOut(this._getZoomScale(e.deltaY)),this.update()}_handleKeyDown(e){let t=!1;switch(e.code){case this.keys.UP:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(Sn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,this.keyPanSpeed),t=!0;break;case this.keys.BOTTOM:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(-Sn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,-this.keyPanSpeed),t=!0;break;case this.keys.LEFT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(Sn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(this.keyPanSpeed,0),t=!0;break;case this.keys.RIGHT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(-Sn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(-this.keyPanSpeed,0),t=!0;break}t&&(e.preventDefault(),this.update())}_handleTouchStartRotate(e){if(this._pointers.length===1)this._rotateStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._rotateStart.set(n,i)}}_handleTouchStartPan(e){if(this._pointers.length===1)this._panStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panStart.set(n,i)}}_handleTouchStartDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyStart.set(0,r)}_handleTouchStartDollyPan(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enablePan&&this._handleTouchStartPan(e)}_handleTouchStartDollyRotate(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enableRotate&&this._handleTouchStartRotate(e)}_handleTouchMoveRotate(e){if(this._pointers.length==1)this._rotateEnd.set(e.pageX,e.pageY);else{let n=this._getSecondPointerPosition(e),i=.5*(e.pageX+n.x),r=.5*(e.pageY+n.y);this._rotateEnd.set(i,r)}this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Sn*this._rotateDelta.x/t.clientHeight),this._rotateUp(Sn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd)}_handleTouchMovePan(e){if(this._pointers.length===1)this._panEnd.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panEnd.set(n,i)}this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd)}_handleTouchMoveDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyEnd.set(0,r),this._dollyDelta.set(0,Math.pow(this._dollyEnd.y/this._dollyStart.y,this.zoomSpeed)),this._dollyOut(this._dollyDelta.y),this._dollyStart.copy(this._dollyEnd);let o=(e.pageX+t.x)*.5,a=(e.pageY+t.y)*.5;this._updateZoomParameters(o,a)}_handleTouchMoveDollyPan(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enablePan&&this._handleTouchMovePan(e)}_handleTouchMoveDollyRotate(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enableRotate&&this._handleTouchMoveRotate(e)}_addPointer(e){this._pointers.push(e.pointerId)}_removePointer(e){delete this._pointerPositions[e.pointerId];for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId){this._pointers.splice(t,1);return}}_isTrackingPointer(e){for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId)return!0;return!1}_trackPointer(e){let t=this._pointerPositions[e.pointerId];t===void 0&&(t=new Ue,this._pointerPositions[e.pointerId]=t),t.set(e.pageX,e.pageY)}_getSecondPointerPosition(e){let t=e.pointerId===this._pointers[0]?this._pointers[1]:this._pointers[0];return this._pointerPositions[t]}_customWheelEvent(e){let t=e.deltaMode,n={clientX:e.clientX,clientY:e.clientY,deltaY:e.deltaY};switch(t){case 1:n.deltaY*=16;break;case 2:n.deltaY*=100;break}return e.ctrlKey&&!this._controlActive&&(n.deltaY*=10),n}};function ay(s){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(s.pointerId),this.domElement.ownerDocument.addEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.addEventListener("pointerup",this._onPointerUp)),!this._isTrackingPointer(s)&&(this._addPointer(s),s.pointerType==="touch"?this._onTouchStart(s):this._onMouseDown(s),this._cursorStyle==="grab"&&(this.domElement.style.cursor="grabbing")))}function ly(s){this.enabled!==!1&&(s.pointerType==="touch"?this._onTouchMove(s):this._onMouseMove(s))}function cy(s){switch(this._removePointer(s),this._pointers.length){case 0:this.domElement.releasePointerCapture(s.pointerId),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.dispatchEvent(np),this.state=Ct.NONE,this._cursorStyle==="grab"&&(this.domElement.style.cursor="grab");break;case 1:let e=this._pointers[0],t=this._pointerPositions[e];this._onTouchStart({pointerId:e,pageX:t.x,pageY:t.y});break}}function hy(s){let e;switch(s.button){case 0:e=this.mouseButtons.LEFT;break;case 1:e=this.mouseButtons.MIDDLE;break;case 2:e=this.mouseButtons.RIGHT;break;default:e=-1}switch(e){case ts.DOLLY:if(this.enableZoom===!1)return;this._handleMouseDownDolly(s),this.state=Ct.DOLLY;break;case ts.ROTATE:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Ct.PAN}else{if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Ct.ROTATE}break;case ts.PAN:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Ct.ROTATE}else{if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Ct.PAN}break;default:this.state=Ct.NONE}this.state!==Ct.NONE&&this.dispatchEvent(Tu)}function uy(s){switch(this.state){case Ct.ROTATE:if(this.enableRotate===!1)return;this._handleMouseMoveRotate(s);break;case Ct.DOLLY:if(this.enableZoom===!1)return;this._handleMouseMoveDolly(s);break;case Ct.PAN:if(this.enablePan===!1)return;this._handleMouseMovePan(s);break}}function dy(s){this.enabled===!1||this.enableZoom===!1||this.state!==Ct.NONE||(s.preventDefault(),this.dispatchEvent(Tu),this._handleMouseWheel(this._customWheelEvent(s)),this.dispatchEvent(np))}function fy(s){this.enabled!==!1&&this._handleKeyDown(s)}function py(s){switch(this._trackPointer(s),this._pointers.length){case 1:switch(this.touches.ONE){case ns.ROTATE:if(this.enableRotate===!1)return;this._handleTouchStartRotate(s),this.state=Ct.TOUCH_ROTATE;break;case ns.PAN:if(this.enablePan===!1)return;this._handleTouchStartPan(s),this.state=Ct.TOUCH_PAN;break;default:this.state=Ct.NONE}break;case 2:switch(this.touches.TWO){case ns.DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchStartDollyPan(s),this.state=Ct.TOUCH_DOLLY_PAN;break;case ns.DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchStartDollyRotate(s),this.state=Ct.TOUCH_DOLLY_ROTATE;break;default:this.state=Ct.NONE}break;default:this.state=Ct.NONE}this.state!==Ct.NONE&&this.dispatchEvent(Tu)}function my(s){switch(this._trackPointer(s),this.state){case Ct.TOUCH_ROTATE:if(this.enableRotate===!1)return;this._handleTouchMoveRotate(s),this.update();break;case Ct.TOUCH_PAN:if(this.enablePan===!1)return;this._handleTouchMovePan(s),this.update();break;case Ct.TOUCH_DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchMoveDollyPan(s),this.update();break;case Ct.TOUCH_DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchMoveDollyRotate(s),this.update();break;default:this.state=Ct.NONE}}function gy(s){this.enabled!==!1&&s.preventDefault()}function xy(s){s.key==="Control"&&(this._controlActive=!0,this.domElement.getRootNode().addEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}function _y(s){s.key==="Control"&&(this._controlActive=!1,this.domElement.getRootNode().removeEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}var pc=class extends bs{constructor(){super(),this.name="RoomEnvironment",this.position.y=-3.5;let e=new ui;e.deleteAttribute("uv");let t=new vn({side:Kt}),n=new vn,i=new jn(16777215,900,28,2);i.position.set(.418,16.199,.3),this.add(i);let r=new Ke(e,t);r.position.set(-.757,13.219,.717),r.scale.set(31.713,28.305,28.591),this.add(r);let o=new _n(e,n,6),a=new Tt;a.position.set(-10.906,2.009,1.846),a.rotation.set(0,-.195,0),a.scale.set(2.328,7.905,4.651),a.updateMatrix(),o.setMatrixAt(0,a.matrix),a.position.set(-5.607,-.754,-.758),a.rotation.set(0,.994,0),a.scale.set(1.97,1.534,3.955),a.updateMatrix(),o.setMatrixAt(1,a.matrix),a.position.set(6.167,.857,7.803),a.rotation.set(0,.561,0),a.scale.set(3.927,6.285,3.687),a.updateMatrix(),o.setMatrixAt(2,a.matrix),a.position.set(-2.017,.018,6.124),a.rotation.set(0,.333,0),a.scale.set(2.002,4.566,2.064),a.updateMatrix(),o.setMatrixAt(3,a.matrix),a.position.set(2.291,-.756,-2.621),a.rotation.set(0,-.286,0),a.scale.set(1.546,1.552,1.496),a.updateMatrix(),o.setMatrixAt(4,a.matrix),a.position.set(-2.193,-.369,-5.547),a.rotation.set(0,.516,0),a.scale.set(3.875,3.487,2.986),a.updateMatrix(),o.setMatrixAt(5,a.matrix),this.add(o);let l=new Ke(e,Nr(50));l.position.set(-16.116,14.37,8.208),l.scale.set(.1,2.428,2.739),this.add(l);let c=new Ke(e,Nr(50));c.position.set(-16.109,18.021,-8.207),c.scale.set(.1,2.425,2.751),this.add(c);let h=new Ke(e,Nr(17));h.position.set(14.904,12.198,-1.832),h.scale.set(.15,4.265,6.331),this.add(h);let u=new Ke(e,Nr(43));u.position.set(-.462,8.89,14.52),u.scale.set(4.38,5.441,.088),this.add(u);let d=new Ke(e,Nr(20));d.position.set(3.235,11.486,-12.541),d.scale.set(2.5,2,.1),this.add(d);let f=new Ke(e,Nr(100));f.position.set(0,20,0),f.scale.set(1,.1,1),this.add(f)}dispose(){let e=new Set;this.traverse(t=>{t.isMesh&&(e.add(t.geometry),e.add(t.material))});for(let t of e)t.dispose()}};function Nr(s){return new xo({color:0,emissive:16777215,emissiveIntensity:s})}var Fi={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};var Dn=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},vy=new mi(-1,1,1,-1,0,1),Eu=class extends ot{constructor(){super(),this.setAttribute("position",new tt([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new tt([0,2,0,0,2,0],2))}},yy=new Eu,vi=class{constructor(e){this._mesh=new Ke(yy,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,vy)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var Ur=class extends Dn{constructor(e,t="tDiffuse"){super(),this.textureID=t,this.uniforms=null,this.material=null,e instanceof ct?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=ei.clone(e.uniforms),this.material=new ct({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new vi(this.material)}render(e,t,n){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=n.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Ko=class extends Dn{constructor(e,t){super(),this.scene=e,this.camera=t,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,t,n){let i=e.getContext(),r=e.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let o,a;this.inverse?(o=0,a=1):(o=1,a=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(i.REPLACE,i.REPLACE,i.REPLACE),r.buffers.stencil.setFunc(i.ALWAYS,o,4294967295),r.buffers.stencil.setClear(a),r.buffers.stencil.setLocked(!0),e.setRenderTarget(n),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(t),this.clear&&e.clear(),e.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(i.EQUAL,1,4294967295),r.buffers.stencil.setOp(i.KEEP,i.KEEP,i.KEEP),r.buffers.stencil.setLocked(!0)}},mc=class extends Dn{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var gc=class{constructor(e,t){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),t===void 0){let n=e.getSize(new Ue);this._width=n.width,this._height=n.height,t=new Bt(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:Yt}),t.texture.name="EffectComposer.rt1"}else this._width=t.width,this._height=t.height;this.renderTarget1=t,this.renderTarget2=t.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Ur(Fi),this.copyPass.material.blending=zn,this.timer=new To}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,t){this.passes.splice(t,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let t=this.passes.indexOf(e);t!==-1&&this.passes.splice(t,1)}isLastEnabledPass(e){for(let t=e+1;t<this.passes.length;t++)if(this.passes[t].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let t=this.renderer.getRenderTarget(),n=!1;for(let i=0,r=this.passes.length;i<r;i++){let o=this.passes[i];if(o.enabled!==!1){if(o.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(i),o.render(this.renderer,this.writeBuffer,this.readBuffer,e,n),o.needsSwap){if(n){let a=this.renderer.getContext(),l=this.renderer.state.buffers.stencil;l.setFunc(a.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),l.setFunc(a.EQUAL,1,4294967295)}this.swapBuffers()}Ko!==void 0&&(o instanceof Ko?n=!0:o instanceof mc&&(n=!1))}}this.renderer.setRenderTarget(t)}reset(e){if(e===void 0){let t=this.renderer.getSize(new Ue);this._pixelRatio=this.renderer.getPixelRatio(),this._width=t.width,this._height=t.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,t){this._width=e,this._height=t;let n=this._width*this._pixelRatio,i=this._height*this._pixelRatio;this.renderTarget1.setSize(n,i),this.renderTarget2.setSize(n,i);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(n,i)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var ip={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Ce(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var Fr=class s extends Dn{constructor(e,t=1,n,i){super(),this.strength=t,this.radius=n,this.threshold=i,this.resolution=e!==void 0?new Ue(e.x,e.y):new Ue(256,256),this.clearColor=new Ce(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);this.renderTargetBright=new Bt(r,o,{type:Yt,depthBuffer:!1}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let h=0;h<this.nMips;h++){let u=new Bt(r,o,{type:Yt,depthBuffer:!1});u.texture.name="UnrealBloomPass.h"+h,u.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(u);let d=new Bt(r,o,{type:Yt,depthBuffer:!1});d.texture.name="UnrealBloomPass.v"+h,d.texture.generateMipmaps=!1,this.renderTargetsVertical.push(d),r=Math.round(r/2),o=Math.round(o/2)}let a=ip;this.highPassUniforms=ei.clone(a.uniforms),this.highPassUniforms.luminosityThreshold.value=i,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new ct({uniforms:this.highPassUniforms,vertexShader:a.vertexShader,fragmentShader:a.fragmentShader}),this.separableBlurMaterials=[];let l=[6,10,14,18,22];r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);for(let h=0;h<this.nMips;h++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(l[h])),this.separableBlurMaterials[h].uniforms.invSize.value=new Ue(1/r,1/o),r=Math.round(r/2),o=Math.round(o/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=t,this.compositeMaterial.uniforms.bloomRadius.value=.1;let c=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=c,this.bloomTintColors=[new W(1,1,1),new W(1,1,1),new W(1,1,1),new W(1,1,1),new W(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=ei.clone(Fi.uniforms),this.blendMaterial=new ct({uniforms:this.copyUniforms,vertexShader:Fi.vertexShader,fragmentShader:Fi.fragmentShader,premultipliedAlpha:!0,blending:Ut,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new Ce,this._oldClearAlpha=1,this._basic=new wt,this._fsQuad=new vi(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,t){let n=Math.round(e/2),i=Math.round(t/2);this.renderTargetBright.setSize(n,i);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(n,i),this.renderTargetsVertical[r].setSize(n,i),this.separableBlurMaterials[r].uniforms.invSize.value=new Ue(1/n,1/i),n=Math.round(n/2),i=Math.round(i/2)}render(e,t,n,i,r){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let o=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),r&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=n.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=n.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let a=this.renderTargetBright;for(let l=0;l<this.nMips;l++)this._fsQuad.material=this.separableBlurMaterials[l],this.separableBlurMaterials[l].uniforms.colorTexture.value=a.texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[l]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[l].uniforms.colorTexture.value=this.renderTargetsHorizontal[l].texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[l]),e.clear(),this._fsQuad.render(e),a=this.renderTargetsVertical[l];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(n),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=o}_getSeparableBlurMaterial(e){let t=[],n=e/3;for(let o=0;o<e;o++)t.push(.39894*Math.exp(-.5*o*o/(n*n))/n);let i=[],r=[];for(let o=1;o<e;o+=2){let a=t[o],l=o+1<e?t[o+1]:0,c=a+l;i.push((o*a+(o+1)*l)/c),r.push(c)}return new ct({defines:{KERNEL_PAIRS:i.length},uniforms:{colorTexture:{value:null},invSize:{value:new Ue(.5,.5)},direction:{value:new Ue(.5,.5)},centerWeight:{value:t[0]},gaussianOffsets:{value:i},gaussianWeights:{value:r}},vertexShader:`

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

				}`})}_getCompositeMaterial(e){return new ct({defines:{NUM_MIPS:e},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`

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

				}`})}};Fr.BlurDirectionX=new Ue(1,0);Fr.BlurDirectionY=new Ue(0,1);var jo={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var xc=class extends Dn{constructor(){super(),this.isOutputPass=!0,this.uniforms=ei.clone(jo.uniforms),this.material=new _r({name:jo.name,uniforms:this.uniforms,vertexShader:jo.vertexShader,fragmentShader:jo.fragmentShader}),this._fsQuad=new vi(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,t,n){this.uniforms.tDiffuse.value=n.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},ht.getTransfer(this._outputColorSpace)===St&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Po?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===Io?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===Lo?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===Cs?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===No?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Uo?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Do&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var ti=512,My=7.5;function Au(s,e=My){let t=new po,n=s.map(([a,l])=>new W(a,0,l)),i=[],r=[],o=n.length;for(let a=0;a<o;a++){let l=n[a],c=n[(a+o-1)%o],h=n[(a+1)%o],u=Math.min(e,l.distanceTo(c)/2,l.distanceTo(h)/2);i.push(l.clone().addScaledVector(c.clone().sub(l).normalize(),u)),r.push(l.clone().addScaledVector(h.clone().sub(l).normalize(),u))}for(let a=0;a<o;a++)t.add(new Kn(i[a],n[a],r[a])),t.add(new gr(r[a],i[(a+1)%o]));return t}function by(s){let e=Au(s),t=e.getSpacedPoints(ti),n=[];for(let i=0;i<ti;i++){let r=t[(i+ti-1)%ti],o=t[(i+1)%ti];n.push(new W(o.x-r.x,0,o.z-r.z).normalize())}return{points:t,tangents:n,length:e.getLength()}}function sp(s){let e=[],t=(n,i,r,o)=>{let a=Math.cos(n.angle),l=Math.sin(n.angle);e.push({x:n.x+i*a+r*l,z:n.z-i*l+r*a,r:o,asset:n.asset})};for(let n of s)if(!(n.z<-100))if(n.asset==="data-tram")for(let i of[-2.4,0,2.4])t(n,i,0,1.5);else n.asset==="server-rack"?t(n,0,0,.8):n.asset==="street-lamp"?t(n,0,0,.5):n.asset==="planter"&&(t(n,-.9,0,.85),t(n,.9,0,.85));return e}function rp({routes:s,obstacles:e=[],robotRadius:t=1.3,lanes:n=[2.2,0,-2.2,3.9,-3.9],defaultLane:i=2.2,speeds:r=[],blocked:o=()=>!1}){let a=s.map(by),l=s.map((b,N)=>({route:N,s:a[N].length*(N*.173%1),dir:1,lane:i,laneTarget:i,speed:0,target:0,laneVel:0,cruise:r[N]??5.1+N*.43,state:"cruise",heading:0,wait:0,cooldown:0,x:0,z:0,fx:0,fz:1,rx:1,rz:0,vx:0,vz:0,turns:0})),c=new W,h=new W,u=new W;function d(b,N,w,H,F){let L=(N/b.length%1+1)%1*ti,O=Math.floor(L)%ti,V=L-Math.floor(L);H.lerpVectors(b.points[O],b.points[(O+1)%ti],V),F.lerpVectors(b.tangents[O],b.tangents[(O+1)%ti],V).normalize().multiplyScalar(w)}let f=b=>Math.atan2(Math.sin(b),Math.cos(b));function g(b){d(a[b.route],b.s,b.dir,c,h),b.fx=h.x,b.fz=h.z,b.rx=-h.z,b.rz=h.x,b.x=c.x+b.rx*b.lane,b.z=c.z+b.rz*b.lane}function v(b,N){for(let w of e){let H=w.r+t;if((b-w.x)**2+(N-w.z)**2<H*H)return!0}return!1}function m(b,N){let w=a[b.route];for(let H of[-2.5,0,1,3,6,9,12,15,18])if(d(w,b.s+b.dir*H,b.dir,c,h),u.set(c.x-h.z*N,0,c.z+h.x*N),v(u.x,u.z)||H>=0&&o(b,u.x,u.z))return!1;return!0}function p(b){b.dir=-b.dir,b.lane=-b.lane,b.laneTarget=i,b.laneVel=0,b.state="turn",b.cooldown=4,b.wait=0,b.target=0,b.turns++}function M(b,N){if(b.held){b.target=0,b.speed=0,b.laneVel=0;return}if(b.cooldown>0&&(b.cooldown-=N),b.state==="turn"){if(b.target=0,b.speed<.05){let L=Math.atan2(b.fx,b.fz),O=f(L-b.heading);b.heading+=O*Math.min(1,N*3.2),Math.abs(O)<.1&&(b.heading=L,b.state="cruise")}return}let w=null,H=[...n].sort((L,O)=>Math.abs(L-i)-Math.abs(O-i)||O-L);for(let L of H)if(m(b,L)){w=L;break}if(w===null){b.cooldown<=0?p(b):b.target=0;return}b.laneTarget=w,b.target=b.cruise,Math.abs(b.laneTarget-b.lane)>.5&&!m(b,b.lane)&&(b.target=Math.min(b.target,2.4));let F=!1;for(let L of l){if(L===b)continue;let O=C(b,b.laneTarget,L);if(!O)continue;F=!0;let V=L.fx*b.fx+L.fz*b.fz,R=L.speed<.3;if(R||V<-.2){let G=x.get(L)[T.length-1],oe=R?0:Math.sign((G.x-b.x)*b.rx+(G.z-b.z)*b.rz)||-1,k=null;for(let re of H)if(!(oe&&Math.sign(re-b.lane)===oe)&&m(b,re)&&!C(b,re,L)){k=re;break}k!==null?b.laneTarget=k:b.target=0}else V>.5?O.da>=O.db&&(b.target=Math.min(b.target,O.da<3.8?0:L.speed*.9)):(O.da>O.db||O.da===O.db&&b.route>L.route)&&(b.target=Math.min(b.target,O.da<=4?0:2))}F&&b.target===0?b.wait+=N:b.wait=0,b.wait>2.5&&b.cooldown<=0&&p(b)}let T=[0,2,4,6,8,10,12],x=new Map,y=new W;function S(b,N,w,H){d(a[b.route],b.s+b.dir*N,b.dir,c,h),H.set(c.x-h.z*w,0,c.z+h.x*w)}function C(b,N,w){let H=x.get(w),F=(t*2+.6)**2,L=w.speed<.3;for(let O=0;O<T.length;O++){S(b,T[O],N,y);let V=T[O]/Math.max(b.speed,1.5);for(let R=0;R<T.length&&!(L&&R>0);R++){let G=T[R]/Math.max(w.speed,1.5);if(!(T[O]>4&&Math.abs(V-G)>1.6)&&y.distanceToSquared(H[R])<F)return{da:T[O],db:T[R]}}}return null}function _(b){if(b>0){for(let N of l){g(N),x.has(N)||x.set(N,T.map(()=>new W));let w=x.get(N);T.forEach((H,F)=>S(N,N.state==="cruise"?H:0,N.lane,w[F]))}for(let N of l)M(N,b);for(let N=0;N<l.length;N++)for(let w=N+1;w<l.length;w++){let H=l[N],F=l[w],L=F.x-H.x,O=F.z-H.z,V=Math.hypot(L,O),R=t*2+.2;if(V>=R)continue;let G=H.state==="turn"||H.speed<.3,oe=F.state==="turn"||F.speed<.3,k=G===oe?.5:G?0:1,re=Math.min(R-Math.max(V,.01),2.5*b),Ae=L*H.rx+O*H.rz,Le=-(L*F.rx+O*F.rz);H.lane=_t.clamp(H.lane-Math.sign(Ae||1)*re*k,-4.2,4.2),F.lane=_t.clamp(F.lane-Math.sign(Le||1)*re*(1-k),-4.2,4.2),H.target=Math.min(H.target,.5),F.target=Math.min(F.target,.5)}for(let N of l){let w=N.x,H=N.z,F=N.target>N.speed?6:9;if(N.held){N.vx=N.vz=0;continue}N.speed+=_t.clamp(N.target-N.speed,-F*b,F*b),N.speed<0&&(N.speed=0),N.s+=N.dir*N.speed*b;let L=N.laneTarget-N.lane,O=N.state==="turn"?0:Math.min(3,N.speed*.6),V=Math.sign(L)*Math.min(O,Math.sqrt(8*Math.abs(L)),Math.abs(L)/b);if(N.laneVel+=_t.clamp(V-N.laneVel,-4*b,4*b),N.lane+=N.laneVel*b,g(N),N.vx=(N.x-w)/b,N.vz=(N.z-H)/b,N.state==="cruise"){let R=N.speed>.25?Math.atan2(N.vx,N.vz):Math.atan2(N.fx,N.fz);N.heading+=f(R-N.heading)*Math.min(1,b*(N.speed>.25?8:3.2))}}}}let I=[];for(let b of l){for(let N=0;N<600&&(g(b),!(I.every(H=>Math.hypot(b.x-H.x,b.z-H.z)>t*2+6)&&m(b,b.lane)));N++)b.s+=1;b.heading=Math.atan2(b.fx,b.fz),I.push(b)}return{agents:l,step:_,hitsObstacle:v,seek(b,N){b.s=N,g(b),b.heading=Math.atan2(b.fx,b.fz)},adopt(b,N){let w=a[b.route],H=0,F=1/0;for(let L=0;L<ti;L++){let O=w.points[L],V=Math.hypot(O.x-N.x,O.z-N.z);V<F&&(F=V,H=L)}b.s=H/ti*w.length,d(w,b.s,b.dir,c,h),b.lane=(N.x-c.x)*-h.z+(N.z-c.z)*h.x,b.laneTarget=i,b.speed=0,b.laneVel=0,b.state="cruise",b.heading=N.heading,g(b)},stats:()=>({states:l.map(b=>b.state),turns:l.reduce((b,N)=>b+N.turns,0),lanes:l.map(b=>Math.round(b.lane*10)/10)})}}var Sy=(s,e,t)=>Math.max(e,Math.min(t,s)),yc=(s,e)=>Math.atan2(Math.sin(e-s),Math.cos(e-s)),Jo=s=>({x:s.x,y:s.y||0,z:s.z,heading:s.heading||0});function Oi(s,e=1){let t=s.min.map(d=>d*e),n=s.max.map(d=>d*e),i=(n[0]-t[0])/2,r=(n[2]-t[2])/2,o=i>r,a=Math.max(i,r),l=Math.min(i,r),c=Math.min(7,Math.max(1,Math.ceil(a/Math.max(.3,l)))),h=a/c,u=[];for(let d=0;d<c;d++){let f=-a+h+d*h*2;u.push({x:(t[0]+n[0])/2+(o?f:0),z:(t[2]+n[2])/2+(o?0:f),r:Math.hypot(l,h)})}return{circles:u,minY:t[1],maxY:n[1],reach:Math.max(...u.map(d=>Math.hypot(d.x,d.z)+d.r))}}function Or(s,e){let t=Math.cos(e.heading),n=Math.sin(e.heading);return{x:e.x+s.x*t+s.z*n,z:e.z-s.x*n+s.z*t,r:s.r}}function vc(s,e,t,n,i){let r=e-s;if(Math.abs(r)<1e-10)return s>=t&&s<=n;let o=(t-s)/r,a=(n-s)/r;return i[0]=Math.max(i[0],Math.min(o,a)),i[1]=Math.min(i[1],Math.max(o,a)),i[0]<=i[1]}function wy(s,e,t,n,i,r){let o=[0,1];return vc(e.y-i.y,t.y-r.y,n.minY-s.maxY+.06,n.maxY-s.minY-.06,o)?o:null}function _c(s,e,t,n,i,r){let o=s.reach+n.reach+.06;if(Math.min(e.x,t.x)-Math.max(i.x,r.x)>o||Math.min(i.x,r.x)-Math.max(e.x,t.x)>o||Math.min(e.z,t.z)-Math.max(i.z,r.z)>o||Math.min(i.z,r.z)-Math.max(e.z,t.z)>o)return!1;let a=wy(s,e,t,n,i,r);if(!a)return!1;for(let l of s.circles)for(let c of n.circles){let h=.06+Math.abs(yc(e.heading,t.heading))*Math.hypot(l.x,l.z)+Math.abs(yc(i.heading,r.heading))*Math.hypot(c.x,c.z),u=Or(l,e),d=Or(l,t),f=Or(c,i),g=Or(c,r),v=u.x-f.x,m=u.z-f.z,p=d.x-g.x-v,M=d.z-g.z-m,T=Sy(-(v*p+m*M)/Math.max(1e-12,p*p+M*M),...a),x=l.r+c.r+h;if((v+p*T)**2+(m+M*T)**2<x*x)return!0}return!1}function Ru(s,e,t,n){let i=s.reach+.06;if(n.world&&(Math.min(e.x,t.x)-i>n.world[2]||Math.max(e.x,t.x)+i<n.world[0]||Math.min(e.z,t.z)-i>n.world[3]||Math.max(e.z,t.z)+i<n.world[1])||Math.min(e.y,t.y)+s.minY>=n.max[1]||Math.max(e.y,t.y)+s.maxY<=n.min[1])return!1;let r=Math.cos(n.heading||0),o=Math.sin(n.heading||0),a=l=>({x:(l.x-n.x)*r-(l.z-n.z)*o,z:(l.x-n.x)*o+(l.z-n.z)*r});for(let l of s.circles){let c=a(Or(l,e)),h=a(Or(l,t)),u=l.r+.06+Math.abs(yc(e.heading,t.heading))*Math.hypot(l.x,l.z),d=[0,1];if(vc(e.y,t.y,n.min[1]-s.maxY+.06,n.max[1]-s.minY-.06,d)&&vc(c.x,h.x,n.min[0]-u,n.max[0]+u,d)&&vc(c.z,h.z,n.min[2]-u,n.max[2]+u,d))return!0}return!1}function Mc(){let s=new Map,e=new Map,t=new Map,n=0,i=0,r=0,o=0,a=(g,v)=>g.id===v.id||g.ignore===v.id||v.ignore===g.id||g.enabled===!1||v.enabled===!1;function l(g,v,m,p=!0){if(![m.x,m.y,m.z,m.heading].every(Number.isFinite))return"invalid-pose";for(let M of e.values())if(M.enabled!==!1&&!(M.owner&&(M.owner===g.ignore||M.owner===g.id))&&Ru(g,v,m,M))return M.id;if(p){for(let M of s.values())if(!a(g,M)&&_c(g,v,m,M,M,M))return M.id}return null}let c=(g,v,m,p=!0)=>!l(g,v,m,p);function h(g,v,m,p=0){if(s.has(g))return s.get(g);let M={id:g,...v,...Jo(m),priority:p,enabled:!0,blocked:0};return c(M,M,M)?(s.set(g,M),M):null}function u(g,v,m=12){let p={...g,enabled:!0};for(let M=0;M<=m;M+=.5)for(let T=0;T<(M?24:1);T++){let x={...Jo(v),x:v.x+Math.cos(T*Math.PI/12)*M,z:v.z+Math.sin(T*Math.PI/12)*M};if(c(p,x,x))return x}return null}function d(g,v,m=12){let p=u(g,v,m);return p?(Object.assign(g,p),!0):!1}function f(g){let v=[...s.values()].filter(m=>m.enabled!==!1).map(m=>{let p=t.get(m.id),M=p?.next||Jo(m);return{body:m,from:Jo(m),next:M,request:p,accepted:c(m,m,M,!1)}});for(let m=0;m<=v.length;m++){let p=!1;for(let M of v)M.accepted&&M.body.follow&&v.some(T=>T.body.id===M.body.follow&&!T.accepted)&&(M.accepted=!1,p=!0);for(let M=0;M<v.length;M++)for(let T=M+1;T<v.length;T++){let x=v[M],y=v[T];if(a(x.body,y.body)||!x.accepted&&!y.accepted)continue;let S=x.accepted?x.next:x.from,C=y.accepted?y.next:y.from;if(!_c(x.body,x.from,S,y.body,y.from,C))continue;let _=G=>G.request&&G.accepted&&(Math.hypot(G.next.x-G.from.x,G.next.y-G.from.y,G.next.z-G.from.z)>1e-8||Math.abs(yc(G.from.heading,G.next.heading))>1e-8),I=_(x),b=_(y);if(!I&&!b)continue;let N=x.next.x-x.from.x,w=x.next.z-x.from.z,H=y.next.x-y.from.x,F=y.next.z-y.from.z,L=N*H+w*F>.8*Math.hypot(N,w)*Math.hypot(H,F),O=I&&!_c(x.body,x.from,x.next,y.body,y.from,y.from),V=b&&!_c(x.body,x.from,x.from,y.body,y.from,y.next),R=I?b?O&&!V?y:V&&!O?x:x.body.priority!==y.body.priority?x.body.priority<y.body.priority?x:y:L?(y.from.x-x.from.x)*N+(y.from.z-x.from.z)*w>0?x:y:x.body.id>y.body.id?x:y:x:y;R.accepted=!1,p=!0}if(!p)break}for(let m of v)m.accepted?(Object.assign(m.body,m.next),m.body.blocked=0,m.body.obstruction=null):m.request&&(m.body.blocked+=g,m.body.obstruction=l(m.body,m.from,m.next),n++);for(let m of v)m.request?.commit?.(m.accepted,m.body);t.clear(),i++,r+=g}return{register:h,relocate:d,findFree:u,clear:c,obstruction:l,solve:f,begin(){t.clear()},propose(g,v,m){g?.enabled!==!1&&g&&t.set(g.id,{next:Jo(v),commit:m})},remove(g){s.delete(g),t.delete(g)},solid(g,v){let m=e.get(g),p={id:g,x:0,z:0,heading:0,...v};if(m&&m.x===p.x&&m.z===p.z&&m.heading===p.heading&&m.enabled===p.enabled&&m.min.every((S,C)=>S===p.min[C])&&m.max.every((S,C)=>S===p.max[C]))return;let M=Math.cos(p.heading),T=Math.sin(p.heading),x=[],y=[];for(let S of[p.min[0],p.max[0]])for(let C of[p.min[2],p.max[2]])x.push(p.x+S*M+C*T),y.push(p.z-S*T+C*M);p.world=[Math.min(...x),Math.min(...y),Math.max(...x),Math.max(...y)],e.set(g,p),o++},removeSolid(g){e.delete(g)&&o++},removeOwner(g){for(let[v,m]of e)m.owner===g&&(e.delete(v),o++)},revision:()=>o,solidClear(g){return[...s.values()].every(v=>v.enabled===!1||g.owner&&v.ignore===g.owner||!Ru(v,v,v,{x:0,z:0,heading:0,...g}))},ceiling(g,v,m){let p=0;for(let M of e.values())M.enabled!==!1&&Ru(g,{x:v,y:M.min[1],z:m,heading:0},{x:v,y:M.min[1],z:m,heading:0},M)&&(p=Math.max(p,M.max[1]-g.minY+1));return p},body:g=>s.get(g),neighbors(g,v=12){return[...s.values()].filter(m=>!a(g,m)&&Math.abs(m.y-g.y)<4&&Math.hypot(m.x-g.x,m.z-g.z)<v)},stats:()=>({bodies:s.size,solids:e.size,stops:n,steps:i,time:r,poses:[...s.values()].filter(g=>g.enabled!==!1).map(g=>({id:g.id,x:g.x,y:g.y,z:g.z,heading:g.heading,blocked:g.blocked,obstruction:g.obstruction}))}),dispose(){s.clear(),e.clear(),t.clear()}}}function op(s,e=1/60){let t=0;return n=>{if(!(n>0))return t=0,0;t=Math.min(t+n,e*6);let i=0;for(;t>=e-1e-9;)s(e),t-=e,i++;return i}}var ap=[[[-67,-77],[-18,-77],[-18,-32],[-67,-32]],[[-18,-77],[18,-77],[18,-32],[-18,-32]],[[18,-77],[67,-77],[67,-32],[18,-32]],[[18,13],[18,-32],[67,-32],[67,13]],[[-67,-77],[-18,-77],[-18,-32],[-67,-32]]];function lp(s,e,t){let n=!1,i=0,r=!1,o=!1,a=0,l=!1,c=new lt;c.name="city-life",s.add(c);let h=t.traffic,u=[],d=rp({routes:ap,obstacles:t.obstacles||[],blocked:(de,he,ce)=>{let Se=u[de.route],fe=Se?{...Se,x:he,z:ce}:null;return Se?!h.clear(Se,fe,fe):!1}}),f=new Set,g=new Set,v=new Set,m=[],p=new AbortController,M=[],T=[],x=de=>(f.add(de),de),y=de=>(g.add(de),de),S=x(new Cn(1,.013,5,64));for(let de of e){let he=y(new wt({color:7452878,transparent:!0,opacity:.08,depthWrite:!1,toneMapped:!1})),ce=new Ke(S,he);ce.rotation.x=Math.PI/2,ce.position.set(de.x,.6,de.z),ce.scale.setScalar(de.radius*1.12),c.add(ce),T.push({id:de.id,ring:ce,material:he,state:"unknown",eventUntil:0,color:new Ce(7438733)})}let C=new Map,_=[],I=new Map,b=Date.now(),N=3e3,w=e.find(de=>de.id==="agent"),H=new W(0,0,1),F=new W,L={memory:12690431,graph:16765844,infra:7978495,integrations:7728086,missions:10268415,operations:16758915};if(w){for(let de of e)if(de!==w){let he=new W(w.x,w.height+3,w.z),ce=new W(de.x,de.height+4,de.z),Se=he.clone().lerp(ce,.5);Se.y=Math.max(he.y,ce.y)+12;let fe=new Kn(he,Se,ce),ue=y(new hi({color:L[de.id],transparent:!0,opacity:0,depthWrite:!1,blending:Ut,toneMapped:!1})),Me=new Ci(x(new ot().setFromPoints(fe.getPoints(48))),ue);Me.visible=!1,c.add(Me),C.set(de.id,{curve:fe,line:Me})}}let O=x(new Cn(1,.06,5,40,Math.PI*.95));for(let de=0;de<12;de++){let he=[];for(let ce=0;ce<4;ce++){let Se=y(new wt({color:10345983,transparent:!0,opacity:0,depthWrite:!1,blending:Ut,toneMapped:!1})),fe=new Ke(ce===3?S:O,Se);fe.visible=!1,c.add(fe),he.push(fe)}_.push({at:-1/0,waves:he,link:null})}function V(de,he){if(!l||t.active?.()===!1||de.at<b||he-de.at>N||de.at>he||!["started","succeeded","failed","sanitized","progress"].includes(de.state))return;let ce=de.to==="agent",Se=ce?de.from:de.to;if(!ce&&de.from!=="agent"||!C.has(Se))return;let fe=Se+":"+ce;if(de.at-(I.get(fe)??-1/0)<450)return;I.set(fe,de.at);let ue=_.reduce((Me,Ee)=>Me.at<Ee.at?Me:Ee);Object.assign(ue,{at:de.at,link:C.get(Se),incoming:ce,from:de.from,to:de.to,state:de.state}),ue.waves.forEach(Me=>Me.material.color.setHex(de.state==="failed"?16737881:L[Se]))}function R(de){let he=Date.now();C.forEach(ce=>{ce.line.visible=!1});for(let ce of _){(!de||t.active?.()===!1)&&(ce.at=-1/0);let Se=(he-ce.at)/N,fe=Se>=0&&Se<1;if(ce.waves.forEach(Me=>{Me.visible=!1}),!fe||!ce.link)continue;ce.link.line.visible=!0,ce.link.line.material.opacity=.12*Math.sin(Se*Math.PI);for(let Me=0;Me<3;Me++){let Ee=(Se-Me*.075)/.75;if(Ee<0||Ee>1)continue;let Ge=ce.incoming?1-Ee:Ee,qe=ce.waves[Me];ce.link.curve.getPoint(Ge,qe.position),ce.link.curve.getTangent(Ge,F),ce.incoming&&F.negate(),qe.quaternion.setFromUnitVectors(H,F),qe.rotateZ(Math.PI*.525),qe.scale.setScalar(1.2+Math.sin(Ee*Math.PI)*3.5),qe.material.opacity=Math.sin(Ee*Math.PI)*.8,qe.visible=!0}let ue=(Se-.74)/.26;if(ue>0){let Me=ce.waves[3];ce.link.curve.getPoint(ce.incoming?0:1,Me.position),Me.rotation.set(-Math.PI/2,0,0),Me.scale.setScalar(1+ue*8),Me.material.opacity=(1-ue)*.6,Me.visible=!0}}}let G=x(new nn(7,7)),oe=y(new ct({transparent:!0,depthWrite:!1,blending:Ut,uniforms:{color:{value:new Ce(5495284)}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"varying vec2 vUv;uniform vec3 color;void main(){float r=length(vUv-.5)*2.;float a=exp(-r*r*5.)*.23*(1.-smoothstep(.65,1.,r));gl_FragColor=vec4(color,a);}"})),k=x(new Ts(.42,1.3,12,1,!0)),re=y(new wt({color:9629439,transparent:!0,opacity:.22,depthWrite:!1,toneMapped:!1,blending:Ut}));ap.forEach((de,he)=>{let ce=new lt;ce.name="city-white-robot-"+(he+1);let Se=new lt;ce.add(Se),c.add(ce);let fe=new Ke(G,oe);fe.rotation.x=-Math.PI/2,fe.position.y=.58,c.add(fe);let ue=new Ke(k,re);ue.rotation.z=Math.PI,ue.position.y=-.45,ce.add(ue),ce.visible=!1,fe.visible=!1,M.push({root:ce,body:Se,glow:fe,jet:ue,agent:d.agents[he]})});function Ae(de){de.traverse(he=>{if(he.isMesh){f.add(he.geometry);for(let ce of Array.isArray(he.material)?he.material:[he.material]){g.add(ce);for(let Se of Object.values(ce))Se?.isTexture&&v.add(Se)}}})}function Le(){f.forEach(he=>he.dispose()),g.forEach(he=>he.dispose());let de=new Set;v.forEach(he=>{he.image&&de.add(he.image),he.dispose()}),de.forEach(he=>he.close?.()),f.clear(),g.clear(),v.clear()}async function be(){let de=setTimeout(()=>p.abort(),15e3);try{let he=await fetch(t.robotURL,{signal:p.signal});if(!he.ok)throw Error("Robot unavailable");let ce=await he.arrayBuffer();if(n)return;let{scene:Se}=await new as().parseAsync(ce,"");if(Ae(Se),n){Le();return}let fe=new Zt().setFromObject(Se),ue=fe.getSize(new W),Me=Math.max(ue.x,ue.y,ue.z);if(!Number.isFinite(Me)||Me<=0)throw Error("Invalid robot bounds");Se.scale.setScalar(6/Me),Se.updateMatrixWorld(!0),fe.setFromObject(Se);let Ee=fe.getCenter(new W);Se.position.set(-Ee.x,-fe.min.y,-Ee.z),Se.rotation.y=-Math.PI/2,Se.updateMatrixWorld(!0),fe.setFromObject(Se),fe.expandByScalar(.4);let Ge=Oi({min:fe.min.toArray(),max:fe.max.toArray()});Se.traverse(qe=>{qe.isMesh&&(qe.castShadow=!1,qe.receiveShadow=!0)}),M.forEach((qe,ke)=>{if(qe.body.add(Se.clone(!0)),qe.root.visible=!0,qe.glow.visible=!0,h){let Ye=qe.agent,Qe=null;for(let Q=0;Q<600&&!Qe;Q++)Qe=h.register("patrol-"+ke,Ge,{x:Ye.x,y:1.6,z:Ye.z,heading:Ye.heading},1),Qe||d.seek(Ye,Ye.s+1);Qe?(u[ke]=Qe,t.society?.patrol(Qe.id,Qe,qe.root,(Q,st)=>{Ye.held=Q,st&&d.adopt(Ye,st)})):(qe.root.visible=!1,qe.glow.visible=!1,Ye.held=!0)}}),r=!0,ge(0,!1)}catch(he){n||(o=!0,t.onError?.(he))}finally{clearTimeout(de)}}function Fe(de){m.forEach(ce=>{ce.dispose(),g.delete(ce)}),m.length=0,de.children.find(ce=>ce.userData.district==="operations")?.getObjectByName("signal")?.traverse(ce=>{if(!ce.isMesh)return;let Se=fe=>{let ue=y(fe.clone());return m.push(ue),ue};ce.material=Array.isArray(ce.material)?ce.material.map(Se):Se(ce.material)})}function le(de,he=[]){for(let Se of T){let fe=de.find(ue=>ue.id===Se.id);Se.state=!fe||fe.stale?"unknown":fe.state,Se.color.setHex(Se.state==="error"?16730430:Se.state==="running"?7400403:Se.state==="unknown"?7438733:7452878),Se.material.color.copy(Se.color)}let ce=Date.now();for(let Se of[...he].reverse())if(Se.id>a&&(V(Se,ce),l&&Se.at>=b&&t.active?.()!==!1&&t.society?.event(Se),ce-Se.at<6e3)){let fe=T.find(ue=>ue.id===Se.district);fe&&(fe.eventUntil=Se.at+5e3)}a=Math.max(a,...he.map(Se=>Se.id))}function ge(de,he){if(n)return;l=he,R(he);let ce=he?Math.min(.1,Math.max(0,de)):0,Se=h?d.agents.map(fe=>({...fe})):null;he&&ce>0&&(i+=ce,d.step(ce));for(let[fe,ue]of M.entries()){let Me=ue.agent,Ee=Math.hypot(Me.vx,Me.vz),Ge=(!he||Me.held)&&u[fe]?u[fe]:Me;ue.root.position.set(Ge.x,1.6+Math.sin(i*1.6+fe*1.9)*.18,Ge.z),ue.root.rotation.y=Ge.heading;let qe=(Me.vx*Me.rx+Me.vz*Me.rz)/Math.max(1,Ee);if(ue.body.rotation.z=Math.sin(i*.85+fe)*.035-qe*.08,ue.body.rotation.x=-.035+Math.sin(i*1.1+fe)*.018-Math.min(.09,Ee*.015),ue.glow.position.x=Ge.x,ue.glow.position.z=Ge.z,ue.jet.scale.y=1+Math.sin(i*4+fe)*.12+Ee*.04,he&&u[fe]&&!Me.held){let ke=u[fe],Ye={x:Me.x,y:1.6,z:Me.z,heading:Me.heading};ue.escape&&i<ue.escape.until?(Ye.x=ke.x+ue.escape.x*ce*1.8,Ye.z=ke.z+ue.escape.z*ce*1.8,Ye.heading=ke.heading):ue.escape=null,!h.clear(ke,ke,Ye,!1)&&h.clear(ke,ke,{...Ye,heading:ke.heading},!1)&&(Ye.heading=ke.heading);let Qe=Math.hypot(Ye.x-ke.x,Ye.z-ke.z);h.propose(ke,Ye,(Q,st)=>{if(ue.wait=Q&&Qe>.002?0:(ue.wait||0)+ce,ue.escape&&Q&&d.adopt(Me,st),ue.wait>2.5){let $e=Math.sin(st.heading),X=Math.cos(st.heading);for(let[E,ae]of[[X,-$e],[-X,$e],[$e,X],[-$e,-X]])if(h.clear(st,st,{...st,x:st.x+E*2.5,z:st.z+ae*2.5})){ue.escape={x:E,z:ae,until:i+2};break}ue.wait=0}if(Q)Me.trafficWait=0,Me.heading=st.heading;else{let $e=(Me.trafficWait||0)+ce;Object.assign(Me,Se[fe]),Me.speed=0,Me.vx=Me.vz=0,Me.trafficWait=$e,$e>2.5&&(Me.state!=="turn"?(Me.dir=-Me.dir,Me.lane=-Me.lane,Me.laneTarget=2.2,Me.state="turn",Me.turns++):Me.state="cruise",Me.trafficWait=0)}ue.root.position.x=st.x,ue.root.position.z=st.z,ue.root.rotation.y=st.heading,ue.glow.position.x=st.x,ue.glow.position.z=st.z})}}for(let fe of T){let ue=he?.5+.5*Math.sin(i*(fe.state==="error"?3.2:1.8)):.5,Me=fe.state==="error"||fe.state==="running"||fe.eventUntil>Date.now();if(fe.material.opacity=fe.state==="unknown"?.03:Me?.2+ue*.46:.065,fe.ring.scale.setScalar(e.find(Ee=>Ee.id===fe.id).radius*1.12*(1+(Me&&he?ue*.035:0))),fe.id==="operations")for(let Ee of m)Ee.color.copy(fe.color),Ee.emissive?.copy(fe.color),Ee.emissiveIntensity=fe.state==="error"?1.1+ue*2.2:fe.state==="unknown"?.08:.55}}function Re(){n||(n=!0,p.abort(),t.signal?.removeEventListener("abort",Re),c.removeFromParent(),Le(),M.length=0,m.length=0,u.forEach(de=>h.remove(de.id)))}return t.signal?.addEventListener("abort",Re,{once:!0}),t.signal?.aborted?Re():be(),{update:ge,setData:le,attachLandmarks:Fe,dispose:Re,stats:()=>({robots:r?M.filter(de=>de.root.visible).length:0,robotError:o,time:i,navigation:d.stats(),positions:M.map(de=>de.root.position.toArray()),transmissions:_.filter(de=>de.waves.some(he=>he.visible)).map(de=>({from:de.from,to:de.to,state:de.state,age:Date.now()-de.at})),signals:T.map(de=>({id:de.id,state:de.state,intensity:de.material.opacity}))})}}var $o='Geist, "Segoe UI", system-ui, sans-serif',bc=13,Ty=s=>{let e=2166136261;for(let t=0;t<s.length;t++)e=Math.imul(e^s.charCodeAt(t),16777619);return(e>>>0).toString(16).padStart(8,"0")},Cu={vertex:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:`uniform sampler2D map;uniform float time,glitch,fade,gain;uniform vec3 tint;varying vec2 vUv;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){vec2 uv=vUv;float row=floor(uv.y*44.0),frame=floor(time*24.0);
float g=glitch*step(0.7,hash(row+frame*0.37));uv.x+=(hash(row*3.1+frame)-0.5)*0.14*g;
float split=0.0025+glitch*0.02;vec4 c=texture2D(map,uv);
float r=texture2D(map,uv+vec2(split,0.0)).r,b=texture2D(map,uv-vec2(split,0.0)).b;
float a=max(c.a,max(texture2D(map,uv+vec2(split,0.0)).a,texture2D(map,uv-vec2(split,0.0)).a));
float lines=0.84+0.16*sin(uv.y*420.0-time*9.0);float flicker=0.95+0.05*sin(time*31.0)*sin(time*7.3);
float bx=(fract(uv.y*0.55-time*0.11)-0.5)*22.0;float band=0.35*exp(-bx*bx);
float edge=smoothstep(0.0,0.05,uv.x)*smoothstep(0.0,0.05,1.0-uv.x)*smoothstep(0.0,0.09,uv.y)*smoothstep(0.0,0.09,1.0-uv.y);
vec3 col=vec3(r,c.g,b)*tint*(lines*flicker+band)*gain;gl_FragColor=vec4(col*a*fade*edge,1.0);}`},Ey={vertex:"varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",fragment:`uniform float time,strength;uniform vec3 base,top;varying vec2 vUv;varying vec3 vNormal,vView;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){float h=vUv.y;float rim=pow(clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0),1.5);
float fall=pow(clamp(1.0-h,0.0,1.0),1.35)*0.75+0.25*clamp(1.0-h,0.0,1.0);float scan=0.7+0.3*sin(h*64.0-time*5.5);
float noise=0.85+0.15*hash(floor(h*96.0)+floor(time*18.0));float shimmer=0.8+0.2*sin(vUv.x*40.0+time*2.0);
float a=(0.28+0.72*rim)*fall*scan*noise*shimmer*strength;gl_FragColor=vec4(mix(base,top,h)*a,1.0);}`},Ay={vertex:Cu.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float r=length(vUv-0.5)*2.0;float rings=smoothstep(0.35,1.0,sin(r*16.0-time*2.6));
float core=exp(-r*r*7.0);float a=(core*1.2+pow(max(0.0,1.0-r),1.8)*(0.25+0.75*rings)*0.6)*strength*(1.0-smoothstep(0.85,1.0,r));
gl_FragColor=vec4(color*a,1.0);}`},cp={vertex:Cu.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float dash=step(0.45,fract(vUv.x*28.0+time*0.35));float pulse=0.75+0.25*sin(vUv.x*6.2831*3.0-time*4.0);
gl_FragColor=vec4(color*dash*pulse*strength,1.0);}`},Ry={vertex:`attribute float seed;uniform float time,pointScale;varying float vLife;
void main(){float speed=0.55+seed*0.9;float life=fract(seed*3.17+time*speed/13.0);float y=life*13.0;
float ang=seed*6.2831+time*(0.35+seed*0.4)+life*2.2;float rad=mix(1.8,6.8,life)*(0.3+0.7*fract(seed*7.13));
vec4 mv=modelViewMatrix*vec4(cos(ang)*rad,y,sin(ang)*rad,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.09+0.16*fract(seed*3.7))*pointScale/max(1.0,-mv.z);vLife=life;}`,fragment:`uniform vec3 color;uniform float strength;varying float vLife;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,2.0)*sin(vLife*3.1416)*strength;gl_FragColor=vec4(color*a,1.0);}`};function Br(s,e,t={}){return new ct({uniforms:e,vertexShader:s.vertex,fragmentShader:s.fragment,transparent:!0,depthWrite:!1,blending:Ut,side:Rt,...t})}function Cy(s,e=24){let t=[],n=new Set;for(let i of Array.isArray(s)?s:[]){if(typeof i!="string")continue;let r=i.replace(/[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u2028-\u202e\u2066-\u2069]/g," ").replace(/\s+/g," ").trim().slice(0,140);if(!(r.length<4||n.has(r))&&(n.add(r),t.push(r),t.length>=e))break}return t}function hp(s,e,t,n){let i=[],r=c=>s.measureText(c).width<=t,o="";for(let c of e.split(" ")){if(i.length>=n)break;let h=o?o+" "+c:c;if(r(h)){o=h;continue}if(o&&(i.push(o),o="",i.length>=n))break;let u=c;for(;!r(u)&&i.length<n;){let d=u.length-1;for(;d>1&&!r(u.slice(0,d));)d--;i.push(u.slice(0,d)),u=u.slice(d)}o=u}o&&i.length<n&&(i.push(o),o="");let a=i.join("").replace(/\s/g,"").length,l=e.replace(/\s/g,"").length;if(a<l&&i.length){let c=i[i.length-1].replace(/[\s,.;:]+$/,"");for(;c.length&&!r(c+"\u2026");)c=c.slice(0,-1);i[i.length-1]=c+"\u2026"}return i.length?i:[""]}function up(s,e,t={}){let n=t.roof??21,i=String(t.label||"MEMORY").toUpperCase(),r=new lt;r.name="memory-hologram",r.position.set(e.x,n,e.z),s.add(r);let o=[],a=[],l=[],c=(ce,Se)=>(o.push(ce),a.push(Se),new Ke(ce,Se)),h=new Ce(6544639),u=new Ce(10980351),d=new Ce(14218751),f={time:{value:0}},g=!1,v=0,m=[],p=[],M=0,T="none",x=!1,y=c(new nn(11,11),Br(Ay,{time:f.time,strength:{value:.9},color:{value:h}}));y.rotation.x=-Math.PI/2,y.position.y=.18,r.add(y);let S=c(new $i(7.4,2.3,bc,56,1,!0),Br(Ey,{time:f.time,strength:{value:.32},base:{value:h},top:{value:u}}));S.position.y=bc/2+.3,r.add(S);let C=c(new Cn(7.5,.055,6,128),Br(cp,{time:f.time,strength:{value:1.6},color:{value:h}}));C.rotation.x=Math.PI/2,C.position.y=bc+.3,r.add(C);let _=c(new Cn(2.6,.05,6,96),Br(cp,{time:f.time,strength:{value:2},color:{value:d}}));_.rotation.x=Math.PI/2,_.position.y=.55,r.add(_);let I=new xr(1.35,1),b=new go(I),N=new hi({color:10217983,transparent:!0,opacity:.75,blending:Ut,depthWrite:!1,toneMapped:!1}),w=new Ss(b,N);w.position.y=4.6,r.add(w),o.push(I,b),a.push(N);let H=new wt({color:4175871,transparent:!0,opacity:.12,blending:Ut,depthWrite:!1,toneMapped:!1}),F=c(new xr(1.1,1),H);w.add(F);let L=160,O=new Float32Array(L);for(let ce=0;ce<L;ce++)O[ce]=ce*.618033988749895%1;let V=new ot;V.setAttribute("position",new dt(new Float32Array(L*3),3)),V.setAttribute("seed",new dt(O,1)),V.boundingSphere=new cn(new W(0,bc/2,0),12);let R=Br(Ry,{time:f.time,pointScale:{value:800},color:{value:d},strength:{value:.9}}),G=new tn(V,R);G.frustumCulled=!1,r.add(G),o.push(V),a.push(R);let oe=new jn(6478079,26,46,2);oe.position.y=6,r.add(oe);function k(ce,Se,fe,ue){let Me=document.createElement("canvas");Me.width=fe,Me.height=ue;let Ee=Me.getContext("2d"),Ge=new ws(Me);Ge.colorSpace=Ft,Ge.generateMipmaps=!1,Ge.minFilter=Ot,l.push(Ge);let qe=Br(Cu,{map:{value:Ge},time:f.time,glitch:{value:0},fade:{value:0},gain:{value:1.35},tint:{value:new Ce(16777215)}},{side:Bn}),ke=c(new nn(ce,Se),qe);return{canvas:Me,context:Ee,texture:Ge,material:qe,mesh:ke,text:"",targetFade:1,next:0}}let re=k(17.5,8.55,1024,500),Ae=[k(8.6,2.1,640,156),k(8.6,2.1,640,156)];re.mesh.position.y=10.4,r.add(re.mesh),Ae.forEach((ce,Se)=>{ce.mesh.position.y=Se?14.4:5.9,ce.orbit=Se?-.22:.27,ce.angle=Se*2.3,r.add(ce.mesh)});function Le(ce,Se,fe){let{context:ue,canvas:Me}=re,Ee=Me.width,Ge=Me.height;ue.clearRect(0,0,Ee,Ge),ue.fillStyle="rgba(48,150,214,0.14)",ue.beginPath(),ue.roundRect(14,14,Ee-28,Ge-28,22),ue.fill(),ue.strokeStyle="rgba(150,230,255,0.85)",ue.lineWidth=3;for(let[qe,ke,Ye,Qe]of[[18,18,1,1],[Ee-18,18,-1,1],[18,Ge-18,1,-1],[Ee-18,Ge-18,-1,-1]])ue.beginPath(),ue.moveTo(qe,ke+Qe*42),ue.lineTo(qe,ke),ue.lineTo(qe+Ye*42,ke),ue.stroke();ue.fillStyle="rgba(150,230,255,0.55)",ue.fillRect(48,108,Ee-96,2),ue.font="600 27px "+$o,ue.textBaseline="middle",ue.fillStyle="rgba(160,232,255,0.92)",ue.textAlign="left",ue.fillText("\u258C "+i+(fe?"  \xB7  "+String(Se+1).padStart(2,"0")+" / "+String(fe).padStart(2,"0"):""),50,72),ue.textAlign="right",ue.font="500 25px "+$o,ue.fillStyle="rgba(190,150,255,0.85)",ue.fillText("0x"+Ty(ce||i).toUpperCase(),Ee-52,72),ue.textAlign="left",ue.shadowColor="rgba(120,225,255,0.9)",ue.shadowBlur=16,ce?(ue.font="600 54px "+$o,ue.fillStyle="rgba(232,250,255,0.97)",hp(ue,ce,Ee-110,4).forEach((ke,Ye)=>ue.fillText(ke,54,172+Ye*72))):(ue.font="600 40px "+$o,ue.fillStyle="rgba(180,235,255,0.7)",ue.fillText("\u25AE \u25AE \u25AF \u25AE \u25AF \u25AF \u25AE \u25AF \u25AE \u25AE \u25AF \u25AE",54,250)),ue.shadowBlur=0,re.texture.needsUpdate=!0}function be(ce,Se){let{context:fe,canvas:ue}=ce,Me=ue.width,Ee=ue.height;fe.clearRect(0,0,Me,Ee),fe.fillStyle="rgba(48,150,214,0.12)",fe.beginPath(),fe.roundRect(6,6,Me-12,Ee-12,14),fe.fill(),fe.fillStyle="rgba(190,150,255,0.8)",fe.fillRect(20,26,6,Ee-52),fe.font="600 42px "+$o,fe.textBaseline="middle",fe.textAlign="left",fe.shadowColor="rgba(160,140,255,0.9)",fe.shadowBlur=12,fe.fillStyle="rgba(236,240,255,0.95)",fe.fillText(Se?hp(fe,Se,Me-70,1)[0]:"\u25AF \u25AE \u25AF \u25AE \u25AF",42,Ee/2),fe.shadowBlur=0,ce.texture.needsUpdate=!0}function Fe(){return m.length?(M>=p.length&&(p=m.map((ce,Se)=>Se).sort(()=>Math.random()-.5),M=0),m[p[M++]]):""}let le=.8,ge=[3.5,6.5];function Re(ce){let Se=Fe();re.text=Se,v++,Le(Se,m.indexOf(Se),m.length),re.material.uniforms.glitch.value=ce?1:0,re.material.uniforms.fade.value=ce?.35:1,le=g?12:4.5+Math.random()*3}function de(ce,Se){let fe=Ae[ce],ue=Fe();fe.text=ue,be(fe,ue.length>46?ue.slice(0,44).replace(/\s+\S*$/,"")+"\u2026":ue),fe.material.uniforms.glitch.value=Se?.7:0,fe.material.uniforms.fade.value=Se?.3:1,ge[ce]=g?15:6+Math.random()*4}Le("",0,0),Ae.forEach(ce=>be(ce,"")),document.fonts?.ready?.then(()=>{x||(Le(re.text,m.indexOf(re.text),m.length),Ae.forEach(ce=>be(ce,ce.text)))});let he=new W;return{group:r,setTexts(ce,Se="live"){let fe=Cy(ce),ue=fe.length!==m.length||fe.some((Me,Ee)=>Me!==m[Ee]);m=fe,T=fe.length?Se:"none",ue&&(p=[],M=0,(!re.text||!m.includes(re.text))&&(le=Math.min(le,.6)))},setPointScale(ce){R.uniforms.pointScale.value=ce},setReducedMotion(ce){g=!!ce},update(ce,Se,fe){if(x)return;let ue=fe&&!g,Me=ue?Math.min(.1,Math.max(0,ce)):0;f.time.value+=Me,le-=ce,le<=0&&Re(ue),ge.forEach((Ee,Ge)=>{ge[Ge]=Ee-ce,ge[Ge]<=0&&de(Ge,ue)});for(let Ee of[re,...Ae]){let Ge=Ee.material.uniforms;Ge.glitch.value=Math.max(0,Ge.glitch.value-ce*2.4),Ge.fade.value=Math.min(1,Ge.fade.value+ce*1.8),Ee.mesh.quaternion.copy(Se.quaternion)}Ae.forEach(Ee=>{Ee.angle+=Ee.orbit*Me,Ee.mesh.position.x=Math.cos(Ee.angle)*6.4,Ee.mesh.position.z=Math.sin(Ee.angle)*6.4,he.copy(Ee.mesh.position).add(r.position).sub(Se.position).normalize();let Ge=-(he.x*Math.cos(Ee.angle)+he.z*Math.sin(Ee.angle));Ee.material.uniforms.gain.value=1.35*_t.clamp(.35+Ge*.9,.15,1.2)}),re.mesh.position.y=9.6+Math.sin(f.time.value*.9)*.22,w.rotation.y+=Me*.7,w.rotation.x+=Me*.31,F.rotation.y-=Me*1.1,C.rotation.z+=Me*.18,_.rotation.z-=Me*.42,oe.intensity=26+Math.sin(f.time.value*2.1)*6+re.material.uniforms.glitch.value*22},stats:()=>({artifacts:m.length,source:T,switches:v,shown:re.text?m.indexOf(re.text):-1}),dispose(){x||(x=!0,r.removeFromParent(),o.forEach(ce=>ce.dispose()),a.forEach(ce=>ce.dispose()),l.forEach(ce=>ce.dispose()),oe.dispose())}}}var Py=`uniform sampler2D tDiffuse;uniform float time,vignette,grain,aberration,streak,shafts,letterbox,aspect,contrast,saturation;
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
gl_FragColor=vec4(c,1.0);}`;function dp(s){let e={tDiffuse:{value:null},time:s,vignette:{value:.5},grain:{value:.03},aberration:{value:.0024},streak:{value:.16},shafts:{value:0},letterbox:{value:0},aspect:{value:1.7777777777777777},contrast:{value:.22},saturation:{value:1.08},lightPos:{value:new Ue(.5,.5)},shaftColor:{value:new Ce(12376319)},shadowTint:{value:new Ce(6686)},highlightTint:{value:new Ce(1444864)}},t=new Ur(new ct({uniforms:e,vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:Py}));t.enabled=!1;let n=new W,i=new Ce(16761992),r=new Ce(12376319),o=!1;return{pass:t,setCinematic(a){o=!!a},update(a,l,c,{day:h,dusk:u,cloud:d,animated:f}){e.aspect.value=l.aspect;let g=o?1:0;e.letterbox.value=f?e.letterbox.value+(g-e.letterbox.value)*Math.min(1,a*2.2):g,n.copy(c).multiplyScalar(1e3).add(l.position).project(l);let v=(n.x+1)/2,m=(n.y+1)/2,p=n.z<1?Math.max(0,1-Math.max(0,Math.abs(v-.5)-.5,Math.abs(m-.5)-.5)*4):0;e.lightPos.value.set(v,m),e.shafts.value=p*(.06+u*.14+(1-h)*.03)*(1-d*.75),e.shaftColor.value.copy(r).lerp(i,u),e.shadowTint.value.setRGB(0,.018+.01*(1-h),.024+.01*(1-h)),e.highlightTint.value.setRGB(.02+u*.03,.01+u*.006,0)},stats:()=>({enabled:t.enabled,letterbox:+e.letterbox.value.toFixed(2),shafts:+e.shafts.value.toFixed(2)}),dispose(){t.material.dispose(),t.dispose?.()}}}var Iu=`float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`,Pu="varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}",Iy="float fbm(vec2 p){float v=0.0,a=0.5;for(int i=0;i<5;i++){if(float(i)>=octaves)break;v+=a*vnoise(p);p=p*2.03+vec2(1.7,9.2);a*=0.5;}return v;}",Ly=`uniform float time,day,dusk,cloud,flash,octaves;uniform vec3 zenith,horizon,haze,warm,sunDir,glow;varying vec3 vWorld;${Iu}${Iy}
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
gl_FragColor=vec4(col,1.0);}`,fp={vertex:`attribute float phase,speed,size;uniform float time,pointScale,cloud;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0)*(1.0-cloud*0.85);
float k=fract(phase*3.1);vTint=k>0.86?vec3(1.0,0.84,0.66):k>0.7?vec3(0.7,0.82,1.0):vec3(0.82,0.9,1.0);}`,fragment:"varying float vAlpha;varying vec3 vTint;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vTint*a,1.0);}"},pp={vertex:`varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,fragment:`uniform float time,day;uniform vec3 deep,shallow,sky,sunDir,sunColor,glow;uniform vec4 island;varying vec3 vWorld;
#include <fog_pars_fragment>
${Iu}
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
}`},Dy=`uniform float time,strength;uniform vec3 color;uniform vec4 island;varying vec3 vWorld;${Iu}
void main(){vec2 p=vWorld.xz;float n=vnoise(p*0.012+vec2(time*0.017,-time*0.011))*0.6+vnoise(p*0.031-vec2(time*0.02,time*0.013))*0.4;
float mist=smoothstep(0.32,0.82,n);vec2 d2=max(abs(p-island.xy)-island.zw,0.0);float outside=smoothstep(0.0,70.0,length(d2));
float far=1.0-smoothstep(500.0,850.0,length(p));gl_FragColor=vec4(color,mist*outside*far*strength);}`,mp={vertex:`attribute vec3 seed;uniform float time,pointScale;varying float vAlpha;
void main(){vec3 p=position;p.x+=sin(time*0.11*seed.x+seed.y*6.28)*9.0+time*0.35*(seed.z-0.5);p.y+=sin(time*0.13+seed.z*6.28)*3.0;
p.z+=cos(time*0.09*seed.y+seed.x*6.28)*9.0;p.x=mod(p.x+90.0,180.0)-90.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.06+0.11*seed.x)*pointScale/max(1.0,-mv.z);vAlpha=0.55+0.45*sin(time*(1.0+seed.y)+seed.z*6.28);}`,fragment:`uniform vec3 color;uniform float strength;varying float vAlpha;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,2.2)*vAlpha*strength,1.0);}`},Ny=`uniform float strength;uniform vec3 color;varying vec2 vUv;
void main(){float along=pow(clamp(1.0-vUv.x,0.0,1.0),2.4);float across=pow(clamp(1.0-abs(vUv.y-0.5)*2.0,0.0,1.0),1.7);gl_FragColor=vec4(color*along*across*strength,1.0);}`,Uy=`uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;
void main(){float rim=clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0);float a=pow(clamp(vUv.y,0.0,1.0),1.6)*(0.35+0.65*rim)*strength;gl_FragColor=vec4(color*a,1.0);}`,gp={vertex:"attribute float lift;varying float vLift;void main(){vLift=lift;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:"uniform vec3 haze;uniform float shade;varying float vLift;void main(){gl_FragColor=vec4(haze*mix(1.0,shade,smoothstep(0.0,0.6,vLift)),1.0);}"},xp={vertex:`attribute float phase;uniform float time,pointScale,strength;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=strength*(0.75+0.25*sin(time*(0.7+fract(phase)*1.3)+phase));gl_PointSize=clamp(2.2*pointScale/max(1.0,-mv.z),1.5,4.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(1.4,0.95,0.55)*pow(1.0-d,1.5)*vA,1.0);}"},_p={vertex:"varying vec2 vUv;varying float vRim,vNear;void main(){vUv=uv;vec3 n=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vRim=abs(dot(n,normalize(-mv.xyz)));vNear=smoothstep(40.0,160.0,-mv.z);gl_Position=projectionMatrix*mv;}",fragment:"uniform vec3 color;uniform float strength;varying vec2 vUv;varying float vRim,vNear;void main(){float along=pow(clamp(1.0-vUv.y,0.0,1.0),1.6)*smoothstep(0.0,0.03,vUv.y);gl_FragColor=vec4(color*along*pow(clamp(vRim,0.0,1.0),2.5)*vNear*strength,1.0);}"},vp={vertex:`attribute float phase;uniform float time,pointScale;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=0.22+0.78*pow(max(0.0,sin(time*1.9+phase)),10.0);gl_PointSize=clamp(1.5*pointScale/max(1.0,-mv.z),2.0,14.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(3.2,0.35,0.25)*pow(1.0-d,2.0)*vA,1.0);}"};function yp(s,e={}){let t=[],n=[],i=new lt;i.name="city-atmosphere",s.add(i);let r=(z,ee)=>(t.push(z),n.push(ee),new Ke(z,ee)),o={value:0},a={value:900},l={value:0},c={value:0},h={value:.25},u={value:0},d={value:5},f=new Ce(4860440),g=(e.sunDirection||new W(-70,145,85)).clone().normalize(),v=new Ce(330010),m=new Ce(1323596),p=new Ce(8011050),M=new Ce(s.fog?.color||528926),T=(z,ee,ne,Te={})=>new ct({uniforms:ne,vertexShader:z,fragmentShader:ee,...Te}),x={transparent:!0,depthWrite:!1,blending:Ut},y=r(new di(1500,48,24),T(Pu,Ly,{time:o,day:l,dusk:c,cloud:h,flash:u,octaves:d,glow:{value:f},zenith:{value:v},horizon:{value:m},haze:{value:M},warm:{value:p},sunDir:{value:g}},{side:Kt,depthWrite:!1,fog:!1}));y.frustumCulled=!1,y.renderOrder=-10,i.add(y);let S=1400,C=new Float32Array(S*3),_=new Float32Array(S),I=new Float32Array(S),b=new Float32Array(S),N=12345,w=()=>(N=Math.imul(N,1664525)+1013904223>>>0)/4294967296;for(let z=0;z<S;z++){let ee=w()*Math.PI*2,ne=.03+w()*.97,Te=Math.sqrt(1-ne*ne);C.set([Math.cos(ee)*Te*1450,ne*1450,Math.sin(ee)*Te*1450],z*3),_[z]=w()*Math.PI*2,I[z]=.4+w()*1.6,b[z]=8e-4+w()*w()*.0022}let H=new ot;H.setAttribute("position",new dt(C,3)),H.setAttribute("phase",new dt(_,1)),H.setAttribute("speed",new dt(I,1)),H.setAttribute("size",new dt(b,1));let F=T(fp.vertex,fp.fragment,{time:o,pointScale:a,cloud:h},{...x,fog:!1}),L=new tn(H,F);L.frustumCulled=!1,L.renderOrder=-9,i.add(L),t.push(H),n.push(F);let O=r(new nn(2800,2800),T(pp.vertex,pp.fragment,{time:o,day:l,glow:{value:f},island:{value:new gt(0,-7,85,87)},deep:{value:new Ce(398368)},shallow:{value:new Ce(930640)},sky:{value:m.clone().multiplyScalar(.7)},sunDir:{value:g},sunColor:{value:new Ce(14676223)},fogColor:{value:new Ce},fogDensity:{value:0},fogNear:{value:1},fogFar:{value:1e3}},{fog:!0}));O.rotation.x=-Math.PI/2,O.position.y=-3.2,i.add(O);let V=r(new nn(1900,1900),T(Pu,Dy,{time:o,strength:{value:.55},color:{value:new Ce(1717320)},island:{value:new gt(0,-7,85,87)}},{transparent:!0,depthWrite:!1,fog:!1}));V.rotation.x=-Math.PI/2,V.position.y=-2.4,i.add(V);let R=500,G=new Float32Array(R*3),oe=new Float32Array(R*3);for(let z=0;z<R;z++)G.set([(w()-.5)*180,2+w()*w()*75,-95+w()*170],z*3),oe.set([w(),w(),w()],z*3);let k=new ot;k.setAttribute("position",new dt(G,3)),k.setAttribute("seed",new dt(oe,3));let re=T(mp.vertex,mp.fragment,{time:o,pointScale:a,color:{value:new Ce(10475775)},strength:{value:.32}},x),Ae=new tn(k,re);Ae.frustumCulled=!1,i.add(Ae),t.push(k),n.push(re);let Le=new lt;Le.position.set(0,86.5,-12),i.add(Le);let be=T(Pu,Ny,{strength:{value:.45},color:{value:new Ce(9430783)}},{...x,side:Rt}),Fe=new nn(100,5.2);Fe.translate(50,0,0),t.push(Fe),n.push(be);let le=new Ke(Fe,be),ge=new Ke(Fe,be);ge.rotation.x=Math.PI/2;let Re=new lt;Re.add(le,ge),Re.rotation.z=-.07,Le.add(Re);let de=new wt({color:12579583,toneMapped:!1}),he=r(new di(.9,16,12),de);Le.add(he);let ce=(e.lamps||[]).slice(0,64),Se=T("varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",Uy,{color:{value:new Ce(16767392)},strength:{value:.3}},{...x,side:Rt}),fe=new Ts(2.7,6.1,18,1,!0),ue=new _n(fe,Se,Math.max(1,ce.length));t.push(fe),n.push(Se);let Me=new Tt;ce.forEach((z,ee)=>{Me.position.set(z.x+1.9*Math.cos(z.angle||0),3.35,z.z-1.9*Math.sin(z.angle||0)),Me.updateMatrix(),ue.setMatrixAt(ee,Me.matrix)}),ue.count=ce.length,ue.instanceMatrix.needsUpdate=!0,i.add(ue);let Ee=[],Ge=[],qe=[],ke=[],Ye=[],Qe=360,Q=z=>Math.max(0,.45+.3*Math.sin(z*5+1.3)+.2*Math.sin(z*13+.4)+.12*Math.sin(z*31+2.1))*(.35+.65*Math.max(0,Math.sin(z*2+.6)));for(let z=0;z<=Qe;z++){let ee=Math.PI*.8+Math.PI*1.45*z/Qe,ne=1180+40*Math.sin(ee*7),Te=6+74*Q(ee);if(Ee.push(Math.cos(ee)*ne,-3.5,Math.sin(ee)*ne,Math.cos(ee)*ne,Te,Math.sin(ee)*ne),Ge.push(0,Te/80),z<Qe){let ye=z*2;qe.push(ye,ye+1,ye+2,ye+1,ye+3,ye+2)}}for(let z=0;z<90;z++){let ee=Math.PI*.82+Math.PI*1.41*w(),ne=1170+40*Math.sin(ee*7);Q(ee)<.08||(ke.push(Math.cos(ee)*ne,1+w()*w()*28,Math.sin(ee)*ne),Ye.push(w()*20))}let st=new ot;st.setAttribute("position",new tt(Ee,3)),st.setAttribute("lift",new tt(Ge,1)),st.setIndex(qe);let $e={value:.8},X=r(st,T(gp.vertex,gp.fragment,{haze:{value:M},shade:$e},{fog:!1,side:Rt}));X.frustumCulled=!1,X.renderOrder=-8,i.add(X);let E=new ot;E.setAttribute("position",new tt(ke,3)),E.setAttribute("phase",new tt(Ye,1));let ae={value:0},se=T(xp.vertex,xp.fragment,{time:o,pointScale:a,strength:ae},{...x,fog:!1}),_e=new tn(E,se);_e.frustumCulled=!1,_e.renderOrder=-7,i.add(_e),t.push(E),n.push(se);let K=T(_p.vertex,_p.fragment,{color:{value:new Ce(11128063)},strength:{value:0}},{...x,side:Rt,fog:!1}),te=new $i(11,.7,280,20,1,!0);te.translate(0,140,0),t.push(te),n.push(K);let J=[[-72,-168,0],[64,-186,2.1],[4,-232,4.2]].map(([z,ee,ne])=>{let Te=new Ke(te,K);return Te.position.set(z,-3,ee),Te.rotation.order="YXZ",Te.userData.offset=ne,Te.frustumCulled=!1,i.add(Te),Te}),$=96,xe=new Float32Array($*3),ve=new Float32Array($);for(let z=0;z<$;z++)ve[z]=w()*Math.PI*2;let me=new ot;me.setAttribute("position",new dt(xe,3)),me.setAttribute("phase",new dt(ve,1)),me.setDrawRange(0,0);let pe=T(vp.vertex,vp.fragment,{time:o,pointScale:a},{...x,fog:!1}),we=new tn(me,pe);we.frustumCulled=!1,i.add(we),t.push(me),n.push(pe);let B=dp(o),q=B.pass,D=!1,Y="high",A=0;return{post:q,sea:O,setLighting(z,ee,ne){g.copy(ne).normalize(),l.value=z,c.value=Math.min(1,ee)*(1-z),v.set(330010).lerp(new Ce(3108776),z).lerp(new Ce(1909062),c.value*.8),m.set(1323596).lerp(new Ce(11126484),z).lerp(new Ce(12609598),c.value*.85),p.set(ee>.1?16747077:8011050),f.set(4860440).lerp(new Ce(7027234),c.value),M.copy(s.fog.color),L.visible=z<.35,O.material.uniforms.sky.value.copy(m).lerp(v,c.value*.65).multiplyScalar(.7),O.material.uniforms.deep.value.set(398368).lerp(new Ce(669270),z),O.material.uniforms.shallow.value.set(930640).lerp(new Ce(1929112),z),V.material.uniforms.strength.value=.55*(1-z*.85),$e.value=.82-.22*(1-z)-.2*c.value,ae.value=Math.max(0,1-z*1.6)*(1-c.value*.5),O.material.uniforms.sunColor.value.set(14676223).lerp(new Ce(16752736),c.value)},setWeather(z){h.value=z==="rain"?.95:z==="fog"?.7:.25},setFlash(z){u.value=Math.max(0,Math.min(1,z))},setCinematic(z){B.setCinematic(z)},setAviation(z){let ee=Math.min($,z.length);for(let ne=0;ne<ee;ne++)xe.set([z[ne].x,z[ne].y,z[ne].z],ne*3);me.attributes.position.needsUpdate=!0,me.setDrawRange(0,ee)},setTier(z){Y=z;let ee=Y!=="low";V.visible=ee,Ae.visible=ee,ue.visible=ee,J.forEach(ne=>{ne.visible=ee}),d.value=Y==="low"?3:Y==="medium"?4:5,q.enabled=Y==="high"||Y==="ultra"},setBusy(z){D=!!z},setPointScale(z){a.value=z},update(z,ee,ne,Te){let ye=Te?Math.min(.1,Math.max(0,z)):0;o.value+=ye,s.fog&&(O.material.uniforms.fogColor.value.copy(s.fog.color),s.fog.isFogExp2&&(O.material.uniforms.fogDensity.value=s.fog.density)),O.position.x=ne.position.x,O.position.z=ne.position.z;let je=D?1.15:.32,Je=Le.userData.rate??je;Le.userData.rate=Je+(je-Je)*Math.min(1,ye*1.5),Re.rotation.y+=ye*Le.userData.rate,be.uniforms.strength.value=D?.85:.45,he.scale.setScalar(1+Math.sin(o.value*(D?6:2.2))*.18),A+=ye;let Mt=1-l.value;K.uniforms.strength.value=Mt*Mt*.075*(1-c.value*.7)*(1-h.value*.45),J.forEach((nt,Pt)=>{let It=nt.userData.offset;nt.rotation.y=Math.PI+Math.sin(A*(.11+Pt*.025)+It)*.95,nt.rotation.x=.22+.14*Math.sin(A*.13+It*1.7)}),q.enabled&&B.update(ye,ne,g,{day:l.value,dusk:c.value,cloud:h.value,animated:Te})},stats:()=>({tier:Y,busy:D,stars:S,dust:R,lamps:ce.length,post:q.enabled,coast:qe.length/6,shoreLights:Ye.length,clouds:+h.value.toFixed(2),searchlights:J.length,aviation:me.drawRange.count,cinema:B.stats()}),dispose(){i.removeFromParent(),ue.dispose(),t.forEach(z=>z.dispose()),n.forEach(z=>z.dispose()),B.dispose()}}}var Qo=3,Fy=3,Oy=192,Mp=[9365742,7730385,16763274,12166399];function bp(s,e={}){let t=e.anchors||[],n=new lt;n.name="city-flow",s.add(n);let i=[];for(let y=0;y<t.length;y++)for(let S of[1,3]){let C=t[y],_=t[(y+S)%t.length];!C||!_||C.id===_.id||i.push({from:C,to:_,lift:22+(y+S)%4*11})}if(i.length<2)return n.removeFromParent(),{group:n,update(){},setTier(){},stats:()=>({routes:0,points:0,visible:!1}),dispose(){}};let r=Math.min(Oy/Qo,i.length*Fy),o=r*Qo,a=new Float32Array(o*3),l=new Float32Array(o),c=new Float32Array(o),h=new Float32Array(o*3),u=new ot;u.setAttribute("position",new dt(a,3)),u.setAttribute("alpha",new dt(l,1)),u.setAttribute("size",new dt(c,1)),u.setAttribute("tint",new dt(h,3));let d=new ct({transparent:!0,depthWrite:!1,blending:Ut,fog:!1,uniforms:{pointScale:{value:900},strength:{value:.4}},vertexShader:`attribute float alpha,size;attribute vec3 tint;uniform float pointScale;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=clamp(size*pointScale/max(1.0,-mv.z),1.0,12.0);vAlpha=alpha;vTint=tint;}`,fragmentShader:`uniform float strength;varying float vAlpha;varying vec3 vTint;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;
gl_FragColor=vec4(vTint*pow(1.0-d,2.4)*vAlpha*strength,1.0);}`}),f=new tn(u,d);f.frustumCulled=!1,f.renderOrder=2,n.add(f);let g=[],v=new Ce;for(let y=0;y<r;y++){let S=i[y%i.length];v.setHex(Mp[y%Mp.length]),g.push({route:S,t:y*.3819%1,speed:.045+y*7%9/190,tint:[v.r,v.g,v.b],size:.42+y*3%5*.08})}function m(y,S,C){let _=y.from,I=y.to,b=1-S,N=(_.x+I.x)/2,w=(_.z+I.z)/2,H=Math.max(_.height,I.height)+y.lift,F=_.height+4,L=I.height+4;return C.x=b*b*_.x+2*b*S*N+S*S*I.x,C.y=b*b*F+2*b*S*H+S*S*L,C.z=b*b*_.z+2*b*S*w+S*S*I.z,C}let p={x:0,y:0,z:0},M="high",T=.4;function x(y,S,C){if(!f.visible)return;let _=S?Math.min(y,.05):0;for(let I=0;I<g.length;I++){let b=g[I];b.t=(b.t+_*b.speed)%1;for(let N=0;N<Qo;N++){let w=I*Qo+N,H=(b.t-N*.012+1)%1;m(b.route,H,p),a[w*3]=p.x,a[w*3+1]=p.y,a[w*3+2]=p.z;let F=Math.min(1,Math.min(H,1-H)*7);l[w]=F*(1-N/Qo)*.9,c[w]=b.size*(1-N*.22),h[w*3]=b.tint[0],h[w*3+1]=b.tint[1],h[w*3+2]=b.tint[2]}}u.attributes.position.needsUpdate=!0,u.attributes.alpha.needsUpdate=!0,u.attributes.size.needsUpdate=!0,u.attributes.tint.needsUpdate=!0,T+=(.18+C*.82-T)*Math.min(1,Math.max(0,y)*2),d.uniforms.strength.value=T}return{group:n,update:x,setPointScale(y){d.uniforms.pointScale.value=y},setTier(y){M=y,f.visible=M!=="low",n.visible=M!=="low"},stats:()=>({routes:i.length,points:o,visible:f.visible,tier:M}),dispose(){n.removeFromParent(),u.dispose(),d.dispose()}}}var Sp=[{points:[[-62,34,-62],[8,44,-74],[62,38,-22],[56,47,42],[-8,41,62],[-64,36,12]],speed:9.5},{points:[[30,56,-12],[0,62,18],[-30,58,-12],[0,66,-42]],speed:8},{points:[[42,29,-56],[-28,33,-42],[-52,27,20],[18,31,52],[60,33,8]],speed:11}];function wp(s,e={}){let t=new lt;t.name="city-drones",s.add(t);let n=[],i=[],r=[],o=new W,a=new W,l=new W,c=new di(.16,8,6);n.push(c);let h=x=>{let y=new wt({color:x,toneMapped:!1});return i.push(y),y},u=h(16730684),d=h(5046154),f=h(16777215);[...Sp,...Sp].forEach((x,y)=>{let S=new mr(x.points.map(b=>new W(b[0],b[1]+(y>=3?8:0),b[2])),!0,"centripetal",.6),C=new lt;C.name="city-drone-"+(y+1),C.visible=!1;let _=new lt;C.add(_);let I=[new Ke(c,u),new Ke(c,d),new Ke(c,f)];I[0].position.set(-2.6,.8,0),I[1].position.set(2.6,.8,0),I[2].position.set(0,2.1,-.4),C.add(...I),t.add(C),r.push({root:C,body:_,lights:I,curve:S,length:S.getLength(),speed:x.speed,t:y*.37%1,rotors:[],roll:0,dir:1,waiting:0})});let g=!0,v="high",m=!1,p=new Map,M=x=>{if(!p.has(x)){let y=x.clone();e.surfaces?.apply(y,"object",x.name),p.set(x,y)}return p.get(x)};function T(x,y){let{curve:S,root:C}=x,_=x.t;if(x.t=(x.t+y*x.speed*x.dir/x.length+1)%1,S.getPointAt(x.t,C.position),S.getTangentAt(x.t,o),o.multiplyScalar(x.dir),x.collider&&e.traffic){let N=x.collider,w=e.traffic;S.getPointAt((x.t+x.dir*20/x.length+1)%1,l),x.avoidTime=Math.max(0,(x.avoidTime||0)-y);let H=Math.max(C.position.y,w.ceiling(N,C.position.x,C.position.z),w.ceiling(N,l.x,l.z),x.avoidTime?x.avoidHeight:0);C.position.y=N.y+_t.clamp(H-N.y,-y*5,y*7);let F=Math.atan2(o.x,o.z),L=N.heading+Math.atan2(Math.sin(F-N.heading),Math.cos(F-N.heading))*Math.min(1,y*3),O={...C.position,heading:L};if(x.obstruction=w.obstruction(N,N,O),x.obstruction){let V=w.body(x.obstruction),R=x.obstruction.startsWith("drone-")&&(Math.abs(N.y-V.y)>1?N.y<V.y:N.id<V.id);R||(x.avoidHeight=Math.min(170,N.y+8),x.avoidTime=8),O={...N,y:R?N.y:Math.min(170,N.y+y*5)},x.t=_}w.propose(N,O,(V,R)=>{V?x.waiting=0:(x.t=_,x.waiting+=y,x.waiting>2.5&&(x.dir=-x.dir,x.waiting=0)),C.position.set(R.x,R.y,R.z),C.rotation.set(0,R.heading,0)})}S.getTangentAt((x.t+.015)%1,a),x.collider||C.rotation.set(0,Math.atan2(o.x,o.z),0);let I=Math.atan2(a.x,a.z)-Math.atan2(o.x,o.z),b=Math.atan2(Math.sin(I),Math.cos(I));x.roll+=(_t.clamp(b*12,-.55,.55)-x.roll)*(y>0?Math.min(1,y*3):1),x.body.rotation.z=x.roll}return{setTier(x){v=x},setTemplate(x){if(!x)return;m=!0;let y=p;p=new Map;for(let[S,C]of r.entries()){C.body.clear(),C.rotors.length=0;let _=x.clone(!0);_.scale.setScalar(1.9),_.updateMatrixWorld(!0);let I=new Zt().setFromObject(_);if(I.expandByScalar(.5),I.min.y-=1,I.max.y+=1,_.traverse(b=>{b.isMesh&&(b.castShadow=!1,b.receiveShadow=!1,b.material=Array.isArray(b.material)?b.material.map(M):M(b.material)),/^rotor_/.test(b.name)&&C.rotors.push(b)}),C.body.add(_),C.root.visible=S<(v==="low"?2:v==="medium"?3:6),e.traffic&&!C.collider){T(C,0);let b=Oi({min:I.min.toArray(),max:I.max.toArray()});C.root.position.y=Math.max(C.root.position.y,e.traffic.ceiling(b,C.root.position.x,C.root.position.z)),C.collider=e.traffic.register("drone-"+S,b,{...C.root.position,heading:C.root.rotation.y},0),C.collider||(C.root.visible=!1)}}y.forEach(S=>S.dispose())},update(x,y,S){g=S;let C=S?Math.min(.1,Math.max(0,x)):0;r.forEach((_,I)=>{if(_.root.visible=m&&I<(v==="low"?2:v==="medium"?3:6)&&(!e.traffic||!!_.collider),_.collider&&(_.root.visible&&!_.collider.enabled&&!e.traffic.relocate(_.collider,_.collider)&&(_.root.visible=!1),_.collider.enabled=_.root.visible),!_.root.visible)return;T(_,C),_.rotors.forEach((N,w)=>{N.rotation.y+=C*(w%2?-46:46)});let b=Math.sin(y*5+I*1.7);_.lights[0].visible=_.lights[1].visible=!S||b>-.2,_.lights[2].visible=!S||b>.93,_.lights[2].scale.setScalar(S?1.6:1)})},stats:()=>({drones:r.filter(x=>x.root.visible).length,animated:g,positions:r.map(x=>x.root.position.toArray().map(y=>Math.round(y))),obstructions:r.map(x=>x.obstruction)}),dispose(){t.removeFromParent(),n.forEach(x=>x.dispose()),i.forEach(x=>x.dispose()),p.forEach(x=>x.dispose()),r.forEach(x=>{x.collider&&e.traffic.remove(x.collider.id)}),r.length=0}}}var wn={xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80},ft={ground:0,road:.12,pavement:.5,quay:.08,room:.16,gallery:4.16},un=["agent","memory","missions"].map((s,e)=>{let t=[0,-43,43][e],n=73;return{id:s,x:t,z:n,width:12,depth:12,front:-1,doorZ:n-6,liftX:t+4,liftZ:n+3}}),Vn=[{id:"infra",x:-67,z:-53,platformX:-74,platformZ:-53,angle:Math.PI/2},{id:"memory",x:-67,z:-10,platformX:-74,platformZ:-10,angle:Math.PI/2},{id:"graph",x:-43,z:59,platformX:-43,platformZ:64.5,angle:0},{id:"operations",x:0,z:59,platformX:0,platformZ:64.5,angle:0},{id:"missions",x:43,z:59,platformX:43,platformZ:64.5,angle:0},{id:"integrations",x:67,z:-10,platformX:74,platformZ:-10,angle:-Math.PI/2},{id:"agent",x:0,z:-77,platformX:0,platformZ:-84,angle:Math.PI}],Lu=[[-67,-77],[-67,59],[67,59],[67,-77]],Tp=(s,e)=>{let t=s*Math.PI/180;return{x:Math.sin(t)*e,z:-12-Math.cos(t)*e,angle:Math.PI-t}},Et={x:0,z:-12,floor:72.2,inner:4.2,outer:8,bridge:1,lift:{x:9.8,z:-12,base:.88,cab:.03,travel:71.32},telescopes:[45,135,225,315].map(s=>({id:"deck-"+s,...Tp(s,7.25)})),benches:[0,180,270].map(s=>Tp(s,6.6))};function Ap(s,e,t){let n=Math.hypot(s-Et.x,e-Et.z);return n>=Et.inner&&n<=Et.outer?!0:t&&s>=Et.outer-.5&&s<=Et.lift.x+1&&Math.abs(e-Et.lift.z)<Et.bridge}var yi={x:78,z:30},Sc=[{x:30.5,z:-55,scale:1},{x:49,z:-55,scale:1.25},{x:-3,z:-57,scale:1.2}];function Ep(s,e,t=wn.halfWidth){return s>=wn.minX&&s<=wn.maxX&&e>=wn.minZ&&e<=wn.maxZ&&(wn.xs.some(n=>Math.abs(s-n)<=t)||wn.zs.some(n=>Math.abs(e-n)<=t))}function Rp(s,e){return Ep(s,e)?Ep(s,e,wn.laneHalfWidth)?ft.road:ft.pavement:ft.ground}function By(s,e,t,n=0){return e>=s.x-5.85+n&&e<=s.x+2.4-n&&t>=s.z+n&&t<=s.z+5.85-n}function Cp(s,e,t,n=0){return Math.abs(e-s.liftX)<=1.6-n&&Math.abs(t-s.liftZ)<=1.6-n}function Pp(){let s=[],e=[2.4,1.4,5.6,4.6],t=(n,i,r,o)=>{r>n&&o>i&&s.push({x:(n+r)/2,z:(i+o)/2,sx:(r-n)/4,sz:(o-i)/4})};for(let n=-4;n<=4;n+=4)for(let i=-4;i<=4;i+=4){let[r,o,a,l]=[n-2,i-2,n+2,i+2],[c,h,u,d]=[Math.max(r,e[0]),Math.max(o,e[1]),Math.min(a,e[2]),Math.min(l,e[3])];if(c>=u||h>=d){t(r,o,a,l);continue}t(r,o,c,l),t(u,o,a,l),t(c,o,u,h),t(c,d,u,l)}return s}function Du(s,e,t,n){return By(s,e,t,.25)?!0:n?e>=s.x+1.8&&e<=s.liftX+1.25&&Math.abs(t-s.liftZ)<1.25:!1}var Nu={agent:[[-8,-8,8,8]],memory:[[-11.4,-10.4,11.4,8.4]],integrations:[[-11.4,-4.9,-4.6,4.9],[4.6,-4.9,11.4,4.9],[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]],missions:[[-13.4,-8.9,13.4,6.9]],infra:[[-10.7,-9.4,10.7,3.4],[-10,-2.6,10,8.5]]},zy={agent:[23,23],memory:[26,22],integrations:[26,16],missions:[30,21],infra:[25,23],graph:[24,24],operations:[9,9]},Uu={agent:[[5.9,8,13],[5.4,13,29],[4.7,29,42.5],[4.2,42.5,61.5],[3.6,61.5,72.2],[2.6,72.2,87]]};function Ip(s,e,t){for(let n of t){let i=zy[n.id];if(i&&Math.abs(s-n.x)<i[0]/2&&Math.abs(e-n.z)<i[1]/2)return .88}return 0}function ky(s,e,t){for(let n of t){let i=s-n.x,r=e-n.z;if(n.id==="graph"&&Math.hypot(i,r)<10.2||n.id==="operations"&&Math.hypot(i,r)<3||(Nu[n.id]||[]).some(([o,a,l,c])=>i>o&&i<l&&r>a&&r<c))return!0}return Sc.some(n=>Math.abs(s-n.x)<6&&Math.abs(e-n.z)<5)}function Us(s,e){return un.find(t=>Math.abs(s-t.x)<t.width/2&&Math.abs(e-t.z)<t.depth/2)}function ls(s,e){if(s>=-81.5&&s<=-78.5&&e>=46&&e<=52)return(52-e)/3;if(s>=-78.3&&s<=-75.3&&e>=46&&e<=52)return Math.min(2,Math.ceil((52-e)*2)/6);if(s>=-82&&s<=-75&&e>=34&&e<46)return 2;if(s>=-80&&s<=-77&&e>=28&&e<34)return(e-28)/3;if(Us(s,e))return ft.room;for(let t of Vn){let n=!!t.angle&&Math.abs(t.angle)!==Math.PI;if(Math.abs(s-t.platformX)<(n?2:4.5)&&Math.abs(e-t.platformZ)<(n?4.5:2))return ft.pavement+.3}return Math.max(Rp(s,e),e>=74&&e<=80&&s>=-80&&s<=80?ft.quay:ft.ground)}function Lp(s,e,t,n,i){if(Math.abs(s)>84||e<-90||e>79)return!1;let r=Us(s,e),o=Us(t.x,t.z);if(r||o){let a=r||o;return!(r?.id!==o?.id&&(Math.abs(s-a.x)>1.15||Math.abs(t.x-a.x)>1.15||Math.min((e-a.doorZ)*a.front,(t.z-a.doorZ)*a.front)<-.9||!n(a.id))||r&&(Math.abs(s-a.x)>5.55||(e-a.z)*a.front<-5.55))}return!ky(s,e,i)}function wc(s,e=new Date){let t=s==="day"?12:s==="evening"?18.5:s==="night"?0:e.getHours()+e.getMinutes()/60;return{hour:t,amount:Math.max(0,Math.min(1,Math.sin((t-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(t-18.5)/2)}}var cs=[{id:"repair-bay",district:"infra",x:-28,z:-62,role:"technician"},{id:"parcel-sorter",district:"missions",x:35,z:49,role:"courier"},{id:"relay-mast",district:"integrations",x:57,z:-20,role:"archivist"},{id:"kinetic-fountain",district:"graph",x:-28,z:47,role:"archivist"},{id:"glass-garden",district:"graph",x:40,z:-68,role:"technician"},{id:"meeting-charge",district:"operations",x:8,z:46,role:"courier"}],Dp=[{id:"data-monument",x:0,z:24,y:0,angle:0,clip:"spin"},{id:"neon-gate",x:-18,z:0,y:.5,angle:0,clip:"pulse"},{id:"sky-pod",x:56,z:47,y:0,angle:0,clip:"scan"},{id:"tidal-turbine",x:0,z:96,y:-3.2,angle:Math.PI,clip:"spin"}];function Np(s,e){let t=[],n=new Map,i=-1;for(let o of[-73,-61,-24,-12,12,24,61,73])for(let a of[-83,-71,-38,-26,7,19,53,65])t.push({x:o,z:a,y:e(o,a),heading:0});function r(o,a){i!==s.revision()&&(n.clear(),i=s.revision());let l=o.circles.map(M=>[M.x,M.z,M.r].join(",")).join(";")+":"+o.maxY;if(!n.has(l)){let M=t.filter(x=>s.clear(o,x,x,!1)),T=M.map(()=>[]);for(let x=0;x<M.length;x++)for(let y=x+1;y<M.length;y++){let S=M[x],C=M[y],_=Math.hypot(S.x-C.x,S.z-C.z);_<48&&s.clear(o,S,C,!1)&&(T[x].push([y,_]),T[y].push([x,_]))}n.set(l,{nodes:M,edges:T})}let c=n.get(l),h=[...c.nodes,{x:o.x,y:o.y,z:o.z,heading:o.heading},{...a,heading:0}],u=h.length-2,d=h.length-1,f=c.edges.map(M=>[...M]);f.push([],[]);for(let M of[u,d])for(let T=0;T<M;T++){let x=h[M],y=h[T],S=Math.hypot(x.x-y.x,x.z-y.z);(S<48||T===u)&&s.clear(o,x,y,!1)&&(f[M].push([T,S]),f[T].push([M,S]))}let g=h.map(()=>1/0),v=h.map(()=>-1),m=new Set(h.map((M,T)=>T));for(g[u]=0;m.size;){let M=-1;for(let T of m)(M<0||g[T]<g[M])&&(M=T);if(!Number.isFinite(g[M])||M===d)break;m.delete(M);for(let[T,x]of f[M])g[M]+x<g[T]&&(g[T]=g[M]+x,v[T]=M)}if(!Number.isFinite(g[d]))return[];let p=[];for(let M=d;M!==u;M=v[M]){if(M<0)return[];p.unshift(h[M])}return p}return{route:r,points:t}}function Up(s){let e=new lt;e.name="city-conversations",s.add(e);let t=new Cn(1,.025,5,40,Math.PI*1.45),n=new Qi(.92,1,48),i=new W(0,0,1),r=new W,o=[],a=0;for(let l=0;l<8;l++){let c=[];for(let h=0;h<4;h++){let u=new wt({color:8445392,transparent:!0,opacity:0,blending:Ut,depthWrite:!1,side:Rt,toneMapped:!1}),d=new Ke(h===3?n:t,u);d.visible=!1,e.add(d),c.push(d)}o.push({meshes:c,age:10,from:null,to:null})}return{send(l,c,h="ambient"){let u=o.find(d=>d.age>=2.4);return u?(Object.assign(u,{from:l,to:c,age:0,source:h}),a++,u.meshes.forEach(d=>d.material.color.setHex(h==="live"?16105851:8445392)),!0):!1},update(l,c=!0){for(let h of o){if((!c||h.from?.enabled===!1||h.to?.enabled===!1)&&(h.age=10),h.age+=l,h.meshes.forEach(m=>{m.visible=!1}),h.age>=2.4)continue;let u=h.from,d=h.to,f=d.x-u.x,g=d.z-u.z;r.set(f,d.y+Math.min(2,d.maxY*.7)-(u.y+Math.min(2,u.maxY*.7)),g).normalize();for(let m=0;m<3;m++){let p=(h.age-m*.18)/1.55;if(p<0||p>1)continue;let M=h.meshes[m];M.visible=!0,M.position.set(u.x+f*p,_t.lerp(u.y+Math.min(2,u.maxY*.7),d.y+Math.min(2,d.maxY*.7),p)+Math.sin(p*Math.PI)*.65,u.z+g*p),M.quaternion.setFromUnitVectors(i,r),M.rotateZ(m*.65),M.scale.setScalar(.25+Math.sin(p*Math.PI)*.65),M.material.opacity=Math.sin(Math.PI*p)*.75}let v=(h.age-1.55)/.85;if(v>0){let m=h.meshes[3];m.visible=!0,m.position.set(d.x,d.y+Math.min(2,d.maxY*.7),d.z),m.quaternion.setFromUnitVectors(i,r),m.scale.setScalar(1.25*(1-v)+.12),m.material.opacity=Math.sin(v*Math.PI)*.85}}},clear(){o.forEach(l=>{l.age=10,l.meshes.forEach(c=>{c.visible=!1})})},stats:()=>({sent:a,active:o.filter(l=>l.age<2.4).length,sources:o.filter(l=>l.age<2.4).map(l=>l.source)}),dispose(){e.removeFromParent(),t.dispose(),n.dispose(),o.forEach(l=>l.meshes.forEach(c=>c.material.dispose()))}}}var Fu=(s,e,t)=>s+Math.atan2(Math.sin(e-s),Math.cos(e-s))*Math.min(1,t*5);function Fp(s,{traffic:e,camera:t,floor:n,onSelect:i,onDemonstrate:r,active:o,onGait:a}){let l=[],c=new Map,h=[],u=[],d=Np(e,n),f=Up(s),g=0,v=null,m=!1,p="high",M=!1,T=0,x="",y=()=>p==="low"?3:p==="medium"?10:19;function S(L){L.slot&&(L.slot.owner=null,L.slot=null),L.path=[],L.goal=null}function C(L){return L?{id:L.id,role:L.role,state:L.state,source:L.source||"ambient",guided:!!L.guide}:null}function _(L){v=L?.id||null,x=L?.state||"",i?.(C(L))}function I(L,O,V){let R="resident-"+O,G=Oi(V,1.15),oe=["courier","technician","archivist"][O%3],k=null;G.circles=[{x:0,z:0,r:G.reach}];let Ae=[...h.filter(be=>be.role===oe).map(be=>({x:be.x,z:be.z+2.2,y:n(be.x,be.z+2.2),heading:Math.PI})),...d.points.map((be,Fe)=>d.points[(O*7+Fe)%d.points.length])];for(let be of Ae)if(k=e.register(R,G,be,1),k)break;if(!k)return L.node.visible=!1,null;let Le={...L,id:R,index:O,body:k,role:oe,state:"idle",path:[],until:O*.3,trip:O,cooldown:8+O*.7,source:"ambient",carrying:!1,parcels:[],soundX:k.x,soundZ:k.z};return L.node.traverse(be=>{be.isMesh&&/parcel/.test(be.name)&&(Le.parcels.push(be),be.visible=!1)}),l.push(Le),L.node.position.set(k.x,k.y,k.z),Le}function b(L,O,V,R){let G={id:L,body:O,node:V,pause:R,role:"patrol",state:"patrol",cooldown:7+l.length,path:[],soundX:O.x,soundZ:O.z};return l.push(G),G}function N(L){S(L),L.source="ambient",L.state="idle",L.until=g+2;let O=h.filter(V=>!V.owner&&(L.role==="courier"?V.place===(L.carrying?"meeting-charge":"parcel-sorter"):V.role===L.role||V.place==="meeting-charge"));for(let V=0;V<O.length;V++){let R=O[(L.trip+V)%O.length],G=d.route(L.body,R);if(G.length){R.owner=L.id,L.slot=R,L.path=G,L.goal=R,L.trip++,L.state=L.carrying?"carry":"walk";return}}for(let V=0;V<8;V++){let R=d.points[(L.trip++*13+L.index)%d.points.length],G=d.route(L.body,R);if(G.length){L.path=G,L.state="walk";return}}}function w(L){for(let O of[L.a,L.b])S(O),O.meeting=null,O.cooldown=g+12+(O.index||0),O.pause?.(!1),O.state=O.pause?"patrol":"idle",O.until=g+1}function H(){u.forEach(w),u.length=0,f.clear(),v=null,i?.(null);for(let L of l)L.guide=null,L.pause?.(!1,L.body),L.trafficYield=!1,L.yieldTram=null,L.pause||(S(L),L.state="idle",L.until=g+1)}function F(L,O){if(M)return;O&&(g+=L);for(let R of l){let G=R.pause||R.index<y();if(G&&!R.body.enabled){if(!e.relocate(R.body,R.body)){R.node.visible=!1;continue}R.body.enabled=!0,R.node.position.set(R.body.x,R.body.y,R.body.z)}!G&&R.body.enabled&&(S(R),R.body.enabled=!1,R.meeting&&w(R.meeting)),R.node.visible=!!G,R.soundX==null&&(R.soundX=R.body.x,R.soundZ=R.body.z);let oe=Math.hypot(R.body.x-R.soundX,R.body.z-R.soundZ);if(R.soundX=R.body.x,R.soundZ=R.body.z,a?.(R.id,R.body.x,R.body.y,R.body.z,oe>.004,!!R.carrying,R.role==="patrol"?"glide":""),!G||!O)continue;let k=e.neighbors(R.body,22).sort((re,Ae)=>Math.hypot(re.x-R.body.x,re.z-R.body.z)-Math.hypot(Ae.x-R.body.x,Ae.z-R.body.z)).find(re=>{if(!re.id.startsWith("tram-"))return!1;let Ae=R.body.x-re.x,Le=R.body.z-re.z,be=Math.cos(re.heading),Fe=Math.sin(re.heading);return Math.abs(Ae*be-Le*Fe)<R.body.reach+(R.trafficYield?4:2.3)&&Ae*Fe+Le*be>-8-R.body.reach&&Ae*Fe+Le*be<14});if(!k&&R.yieldTram){let re=e.body(R.yieldTram),Ae=re?R.body.x-re.x:0,Le=re?R.body.z-re.z:0;if(re?.enabled&&Math.hypot(Ae,Le)<26&&Ae*Math.sin(re.heading)+Le*Math.cos(re.heading)>-re.reach-R.body.reach-1){R.pause?.(!0),R.play?.("idle");continue}R.yieldTram=null}if(k){if(R.trafficYield=!0,R.yieldTram=k.id,R.meeting){let de=R.meeting;w(de),u.splice(u.indexOf(de),1)}R.pause?.(!0);let re=Math.cos(k.heading),Ae=Math.sin(k.heading),Le=Math.sign((R.body.x-k.x)*re-(R.body.z-k.z)*Ae)||1,be=(R.body.x-k.x)*re-(R.body.z-k.z)*Ae,Fe=Math.sign((R.body.x-k.x)*Ae+(R.body.z-k.z)*re)||1,le=[[re*Le,-Ae*Le,R.body.reach+4.5-Math.abs(be)],[-re*Le,Ae*Le,R.body.reach+4.5+Math.abs(be)],[Ae*Fe,re*Fe,1.2]],ge=Array.from({length:16},(de,he)=>[Math.cos(he*Math.PI/8),Math.sin(he*Math.PI/8),2]),Re=([de,he])=>de*(R.body.x-k.x)+he*(R.body.z-k.z);le.push(...ge.filter(de=>Re(de)>.2).sort((de,he)=>Re(he)-Re(de)));for(let[de,he,ce]of le){let Se={x:R.body.x+de*ce,y:R.body.y,z:R.body.z+he*ce,heading:R.body.heading};if(!e.clear(R.body,R.body,Se))continue;let fe={...Se,x:R.body.x+de*L*1.8,z:R.body.z+he*L*1.8};fe.y=R.pause?R.body.y:n(fe.x,fe.z),e.propose(R.body,fe,(ue,Me)=>{R.node.position.set(Me.x,Me.y,Me.z),R.play?.("walk")}),R.state="yield",R.cooldown=g+5;break}continue}if(R.trafficYield&&(R.trafficYield=!1,R.pause?.(!1,R.body),R.state=R.pause?"patrol":R.path.length?"walk":"idle"),!R.meeting){if(R.pause){R.state==="greet"&&g>R.until&&(R.pause(!1),R.state="patrol");continue}if(R.guide&&Math.hypot(t.position.x-R.body.x,t.position.z-R.body.z)>9){R.play("idle");continue}if(R.path.length){let re=R.path[0],Ae=re.x-R.body.x,Le=re.z-R.body.z,be=Math.hypot(Ae,Le),Fe=R.guide?1.9:1.65;if(be<.18){R.path.shift();continue}let le=Math.min(be,Fe*L),ge={x:Ae/be,z:Le/be},Re=R.body.x+ge.x*le,de=R.body.z+ge.z*le,he=Fu(R.body.heading,Math.atan2(Ae,Le),L),ce={x:R.body.x+ge.x*1.8,y:n(Re,de),z:R.body.z+ge.z*1.8,heading:he};if(!e.clear(R.body,R.body,ce)){let fe=!1;for(let[ue,Me]of[[ge.z,-ge.x],[-ge.z,ge.x],[-ge.x,-ge.z]]){let Ee={x:R.body.x+ue*1.3,y:R.body.y,z:R.body.z+Me*1.3,heading:R.body.heading};if(e.clear(R.body,R.body,Ee)){Re=R.body.x+ue*le,de=R.body.z+Me*le,he=Fu(R.body.heading,Math.atan2(ue,Me),L),fe=!0;break}}fe||(Re=R.body.x,de=R.body.z)}e.clear(R.body,R.body,{x:Re,y:n(Re,de),z:de,heading:he},!1)||(he=R.body.heading);let Se=Math.hypot(Re-R.body.x,de-R.body.z)>.001;e.propose(R.body,{x:Re,y:n(Re,de),z:de,heading:he},(fe,ue)=>{R.node.position.set(ue.x,ue.y,ue.z),R.node.rotation.y=ue.heading,!fe||!Se?(R.wait=(R.wait||0)+L,R.play("idle"),R.state="yield"):(R.wait=0,R.state=R.guide?"guide":R.carrying?"carry":"walk",R.play(R.carrying?"carry":"walk")),R.wait>2.5&&(R.wait=0,S(R),R.state=R.guide?"guide":"idle",R.until=g+.5,R.guide&&(R.path=d.route(R.body,R.guide)))})}else if(R.state==="walk"||R.state==="carry"||R.state==="guide"||R.state==="yield"){if(R.guide&&Math.hypot(R.body.x-R.guide.x,R.body.z-R.guide.z)>.5){R.play("idle"),g>R.until&&(R.path=d.route(R.body,R.guide),R.until=g+2.5);continue}R.guide=null,R.state=R.slot?.place==="meeting-charge"?"charge":R.slot?"work":"idle",R.until=g+5+R.index%4,R.play(R.state==="work"?"work":"idle"),R.slot&&r?.(R.slot.place,5),T++}else g>R.until&&(R.role==="courier"&&R.slot&&(R.state==="work"||R.state==="charge")&&(R.carrying=R.slot.place==="parcel-sorter",R.parcels.forEach(re=>{re.visible=R.carrying})),N(R))}}if(!O){f.update(0,!1);return}for(let R of l){if(!R.body.enabled||R.trafficYield||R.meeting||R.guide||R.state==="greet"||g<R.cooldown)continue;let G=l.find(k=>k!==R&&k.body.enabled&&!k.trafficYield&&k.state!=="greet"&&!k.meeting&&!k.guide&&g>=k.cooldown&&Math.abs(R.body.y-k.body.y)<2&&Math.hypot(R.body.x-k.body.x,R.body.z-k.body.z)>2.8&&Math.hypot(R.body.x-k.body.x,R.body.z-k.body.z)<6);if(!G)continue;let oe={a:R,b:G,at:g,sent:!1,replied:!1};u.push(oe);for(let k of[R,G])k.meeting=oe,k.state="exchange",k.pause?.(!0),k.play?.("greet")}for(let R=u.length-1;R>=0;R--){let G=u[R],oe=g-G.at;if(oe>4.8||!G.a.body.enabled||!G.b.body.enabled){w(G),u.splice(R,1);continue}for(let[k,re]of[[G.a,G.b],[G.b,G.a]])e.propose(k.body,{...k.body,heading:Fu(k.body.heading,Math.atan2(re.body.x-k.body.x,re.body.z-k.body.z),L)},(Ae,Le)=>{k.node.rotation.y=Le.heading});oe>.6&&!G.sent&&(f.send(G.a.body,G.b.body),G.sent=!0),oe>2.4&&!G.replied&&(f.send(G.b.body,G.a.body),G.replied=!0)}f.update(L,!0);let V=l.find(R=>R.id===v);V&&V.state!==x&&(x=V.state,i?.({...C(V),refresh:!0}))}return{add:I,patrol:b,update:F,suspend:H,addPlace(L,O){if(!c.has(L.id)){c.set(L.id,L);for(let V of O.navigation.workpoints||[])h.push({id:L.id+":"+V.id,place:L.id,role:L.role,x:L.x+V.position[0],y:n(L.x,L.z),z:L.z+V.position[2],owner:null})}},nearby(){let L=[];for(let O of l)if(O.body.enabled){let V=Math.hypot(t.position.x-O.body.x,t.position.z-O.body.z);V<5&&Math.abs(t.position.y-O.body.y)<8&&L.push({kind:"resident",id:O.id,distance:V,x:O.body.x,z:O.body.z})}for(let O of c.values()){let V=Math.hypot(t.position.x-O.x,t.position.z-O.z);V<7&&L.push({kind:"demonstrate",id:O.id,distance:V,x:O.x,z:O.z})}return L},inspect(L){let O=l.find(V=>V.id===L);return _(O),C(O)},action(L,O){let V=l.find(R=>R.id===v);if(!V)return null;if(V.meeting){let R=V.meeting,G=u.indexOf(R);w(R),G>=0&&u.splice(G,1)}if(L==="greet"&&(S(V),V.guide=null,V.pause?.(!0),V.state="greet",V.until=g+3,V.cooldown=g+5,V.play?.("greet")),L==="guide"&&!V.pause&&O){S(V);for(let R of[0,1.5,3]){for(let G=0;G<(R?8:1)&&!V.path.length;G++){let oe=O.x+Math.cos(G*Math.PI/4)*R,k=O.z+Math.sin(G*Math.PI/4)*R;V.path=d.route(V.body,{x:oe,z:k,y:n(oe,k)})}if(V.path.length)break}V.guide=V.path.length?V.path[V.path.length-1]:null,V.state=V.guide?"guide":"idle"}return L==="cancel"&&(S(V),V.guide=null,V.pause?.(!1),V.state=V.pause?"patrol":"idle",V.until=g+1),_(V),C(V)},demonstrate(L,O=!1){c.has(L)&&r?.(L,O?0:6)},setTier(L){p=L},setReplay(L){m!==L&&(m=L,H())},event(L){if(m||!o()||!["started","succeeded","failed","sanitized","progress"].includes(L.state)||!Number.isFinite(L.at)||Date.now()-L.at>2500||L.at>Date.now())return;let O=cs.find(V=>V.district===(L.to==="agent"?L.from:L.to));O&&c.has(O.id)&&r?.(O.id,2,L.state)},stats:()=>({completed:T,meetings:u.length,reservations:h.filter(L=>L.owner).length,signals:f.stats(),residents:l.filter(L=>L.body.enabled).map(L=>({...C(L),x:L.body.x,z:L.body.z})),places:[...c.keys()]}),dispose(){M=!0,H(),l.forEach(L=>e.remove(L.id)),l.length=0,c.clear(),h.length=0,f.dispose()}}}function Op(s){let e=new lt;e.name="living-machinery",s.add(e);let t=new Set,n=new Set,i=new Map,r=0,o="high",a=T=>(t.add(T),T),l=T=>(n.add(T),T),c=a(new Cn(1,.025,6,64)),h=a(new pr(1,48)),u=a(new ot),d=new Float32Array(288),f=new Float32Array(96);for(let T=0;T<96;T++)d[T*3]=Math.cos(T*2.399)*Math.sqrt(T/96),d[T*3+2]=Math.sin(T*2.399)*Math.sqrt(T/96),f[T]=T/96;u.setAttribute("position",new dt(d,3)),u.setAttribute("phase",new dt(f,1));for(let T of cs){let x=new lt;x.name=T.id,x.position.set(T.x,.12,T.z),x.visible=!1,e.add(x);let y=l(new wt({color:7529695,transparent:!0,opacity:.15,depthWrite:!1,blending:Ut,toneMapped:!1})),S=[];for(let I=0;I<3;I++){let b=new Ke(c,l(y.clone()));b.rotation.x=-Math.PI/2,x.add(b),S.push(b)}let C=new Ke(h,l(y.clone()));C.rotation.x=-Math.PI/2,C.scale.setScalar(2.5),C.position.y=.2,x.add(C);let _=new tn(u,l(new ct({transparent:!0,depthWrite:!1,blending:Ut,uniforms:{time:{value:0},strength:{value:0},color:{value:new Ce(10414816)}},vertexShader:"attribute float phase;uniform float time;uniform float strength;varying float a;void main(){float t=fract(phase+time*.31);vec3 p=position*vec3(2.,1.,2.);p.y=t*3.;p.xz*=1.-t*.7;vec4 v=modelViewMatrix*vec4(p,1.);a=sin(t*3.14159)*strength;gl_PointSize=clamp(65./max(1.,-v.z),1.,5.);gl_Position=projectionMatrix*v;}",fragmentShader:"uniform vec3 color;varying float a;void main(){float d=length(gl_PointCoord-.5);gl_FragColor=vec4(color,a*(1.-smoothstep(.05,.5,d)));}"})));x.add(_),i.set(T.id,{group:x,waves:S,base:C,motes:_,district:T.district,started:0,until:0,source:"ambient",model:null})}let g=new Ke(a(new nn(3.9,2.3)),l(new ct({transparent:!0,depthWrite:!1,side:Rt,blending:Ut,uniforms:{time:{value:0},strength:{value:0}},vertexShader:"varying vec2 q;void main(){q=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 q;uniform float time;uniform float strength;void main(){float line=pow(max(0.,1.-abs(q.y-fract(time*.4))*14.),3.);float grid=step(.94,fract(q.x*24.))*step(.9,fract(q.y*18.));gl_FragColor=vec4(.18,.9,.8,(line*.36+grid*.1)*sin(q.x*3.14159)*strength);}"})));g.position.set(0,1.4,.8),i.get("repair-bay").group.add(g);let v=i.get("kinetic-fountain"),m=l(new ct({transparent:!0,depthWrite:!1,side:Rt,uniforms:{time:{value:0}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 vUv;uniform float time;void main(){float pulse=.5+.5*sin(vUv.x*95.-time*9.);float edge=.35+.65*pow(max(0.,sin(vUv.y*3.14159)),2.);gl_FragColor=vec4(mix(vec3(.09,.35,.42),vec3(.6,.95,1.),pow(max(0.,pulse),5.)),edge*.56);}"}));for(let T=0;T<8;T++){let x=T*Math.PI/4,y=new W(Math.cos(x)*2.3,.5,Math.sin(x)*2.3),S=new W(Math.cos(x+.5)*.7,.55,Math.sin(x+.5)*.7),C=new Kn(y,new W(Math.cos(x)*1.5,3.7,Math.sin(x)*1.5),S);v.group.add(new Ke(a(new mo(C,24,.045,5,!1)),m))}let p=l(new ct({transparent:!0,depthWrite:!1,uniforms:{time:{value:0}},vertexShader:"varying vec2 q;uniform float time;void main(){q=position.xy;vec3 p=position;p.z+=sin(length(q)*18.-time*4.)*.012;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.);}",fragmentShader:"varying vec2 q;uniform float time;void main(){float r=length(q);float a=.5+.5*sin(r*16.-time*3.+sin(q.x*3.+q.y*2.+time*.7)*.35);float rim=1.-smoothstep(2.35,2.58,r);gl_FragColor=vec4(mix(vec3(.03,.13,.17),vec3(.22,.55,.58),pow(max(0.,a),9.)*.3),.78*rim);}"})),M=new Ke(a(new pr(2.58,64)),p);return M.rotation.x=-Math.PI/2,M.position.y=.3,v.group.add(M),{attach(T,x){let y=i.get(T);y&&(y.model=x,y.group.visible=!0)},demonstrate(T,x=6,y=null){let S=i.get(T);S&&(S.until<=r&&(S.started=r),S.until=r+x,S.source=y?"live":"ambient",S.status=y,S.model?.play("operate"))},clear(){i.forEach(T=>{T.until=0,T.source="ambient",T.status=null})},setTier(T){o=T},update(T,x){x&&(r+=T),m.uniforms.time.value=p.uniforms.time.value=g.material.uniforms.time.value=r;for(let[y,S]of i){let C=S.until>r,_=y==="kinetic-fountain",I=C?_t.smoothstep(r-S.started,0,.35)*_t.smoothstep(S.until-r,0,.65):0,b=_?Math.max(.25,I):I;C||(S.source="ambient",S.status=null);let N=S.status==="failed"?16737881:S.source==="live"?{infra:7978495,integrations:7728086,missions:10268415,graph:16765844,operations:16758915}[S.district]:7529695;if(S.waves.forEach((w,H)=>{w.visible=C;let F=(r*.6+H/3)%1;w.position.y=.4+F*2,w.scale.setScalar(.4+F*2.6),w.material.opacity=(1-F)*.22*I,w.material.color.setHex(N)}),y==="repair-bay"&&(g.visible=C,g.material.uniforms.strength.value=I),y==="relay-mast"&&C){let w=S.model?.node.getObjectByName("antenna");w&&(w.rotation.y=_t.damp(w.rotation.y,Math.atan2(-S.group.position.x,-12-S.group.position.z),3,x?T:0))}S.base.material.opacity=.08*I,S.motes.visible=o!=="low"&&b>0,S.motes.material.uniforms.time.value=r,S.motes.material.uniforms.strength.value=b,S.model?.mixer&&(S.model.mixer.timeScale=x&&(C||_)?.65:0)}},stats:()=>({places:[...i].filter(([,T])=>T.model).map(([T,x])=>({id:T,operating:x.until>r,source:x.source})),time:r}),dispose(){e.removeFromParent(),t.forEach(T=>T.dispose()),n.forEach(T=>T.dispose()),i.clear()}}}function Bp(s,e){let t=new lt;t.name="world-2",s.add(t);let n=new Map,i=new Set,r=new Set,o=[],a=[],l=[],c=new Map,h=new Map,u=[],d=new Map,f=new Map,g=new Set,v=[],m=new Set,p=[],M=new Map,T=new AbortController,x=[],y=0,S=!1,C=!1,_=e.tier||"high",I=null,b=0,N=0,w=null,H=null,F=0,L=new Set,O=null,V=0,R=new W,G=new W,oe=new W,k=e.camera,re={ready:!1,lift:null,cab:null,value:0,motion:null,idle:0},Ae=[],Le=new xn(0,0,0,"YXZ"),be=B=>Math.atan2(Math.sin(B),Math.cos(B)),Fe=B=>B<=0?0:B>=1?1:B*B*(3-2*B),le=e.traffic||Mc(),ge=Op(s),Re=new Map;function de(B,q){let D=ls(B,q);for(let Y of cs)for(let A of I?.assets.find(z=>z.id===Y.id)?.navigation.surfaces||[]){let[z,ee,ne,Te]=A.rect;B>=Y.x+z&&B<=Y.x+ne&&q>=Y.z+ee&&q<=Y.z+Te&&(D=Math.max(D,ls(Y.x,Y.z)+A.height))}return D}let he=Fp(s,{traffic:le,camera:k,floor:de,onSelect:e.onSociety,onDemonstrate:ge.demonstrate,active:e.active,onGait:e.onGait}),ce=0,Se=0;try{L=new Set(JSON.parse(localStorage.getItem("aurago.desktop.sysworld.discoveries")||"[]").filter(B=>Vn.some(q=>q.id===B)))}catch{}let fe=Au(Lu),ue=fe.getLength(),Me=Vn.map(B=>{let q=1/0,D=0;for(let Y=0;Y<3e3;Y++){fe.getPointAt(Y/3e3,R);let A=Math.hypot(R.x-B.x,R.z-B.z);A<q&&(q=A,D=Y/3e3)}return{...B,u:D}}),Ee=(B,q,D)=>e.onSound?.(B,q?.x||0,q?.y||0,q?.z||0,D),Ge=new vn({color:11834208,metalness:.8,roughness:.35});r.add(Ge);for(let B of[-1,1]){let q=[],D=[];for(let z=0;z<=512;z++){fe.getPointAt(z/512,R),fe.getTangentAt(z/512,G);for(let ee of[-.045,.045]){let ne=B*.85+ee;q.push(R.x-G.z*ne,.21,R.z+G.x*ne)}if(z<512){let ee=z*2;D.push(ee,ee+1,ee+2,ee+1,ee+3,ee+2)}}let A=new ot;A.setAttribute("position",new tt(q,3)),A.setIndex(D),A.computeVertexNormals(),i.add(A),t.add(new Ke(A,Ge))}async function qe(B,q=_==="low"?2:_==="medium"?1:0){let D=B+":"+q;return n.has(D)||n.set(D,(async()=>{let Y=I.assets.find(ye=>ye.id===B),A=Y?.lods.find(ye=>ye.level===q);if(!A||!/^[\w-]+\.lod[0-2]\.glb$/.test(A.file))throw Error("Invalid world asset");let z=await fetch(e.assetURL(A.file),{signal:T.signal});if(!z.ok)throw Error("World asset unavailable");let ee=await z.arrayBuffer();if(C)throw Error("Disposed");b+=ee.byteLength;let ne=await new as().parseAsync(ee,""),Te=ne.animations.length>0||/^(tram|service-cart|robot-|door|lift|sky-lift|telescope)/.test(B);if(ne.scene.traverse(ye=>{if(ye.isMesh){i.add(ye.geometry);for(let je of Array.isArray(ye.material)?ye.material:[ye.material])r.add(je);ye.castShadow=!0,ye.receiveShadow=!0,e.surfaces?.prepare(ye,Te)}}),C)throw i.forEach(ye=>ye.dispose()),r.forEach(ye=>ye.dispose()),Error("Disposed");return g.add(B),ne})()),n.get(D)}async function ke(B,q,D,Y=0,A=0,z=t,ee=[1,1,1],ne){let Te=await qe(B,ne);if(C)return null;let ye=Te.scene.clone(!0);ye.position.set(q,Y,D),ye.rotation.y=A,ye.scale.set(...ee),z.add(ye),ye.traverse(vt=>{vt.isMesh&&(vt.userData.worldAsset=B,vt.userData.worldPart=vt.name,v.push(vt))});let je=I.assets.find(vt=>vt.id===B),Je="furnishing:"+ce++;for(let[vt,dn]of(je.navigation.colliders||[]).entries())le.solid(Je+":"+vt,{owner:Je,x:q,z:D,heading:A,min:[dn[0]*ee[0],Y+dn[1]*ee[1],dn[2]*ee[2]],max:[dn[3]*ee[0],Y+dn[4]*ee[1],dn[5]*ee[2]]});let Mt=Te.animations.length?new Eo(ye):null;Mt&&o.push(Mt);let nt=null,Pt=null,It=/^robot-/.test(B);return{node:ye,mixer:Mt,owner:Je,bounds:je.motion_bounds||je.lods[0].bounds,clips:Te.animations,play(vt,dn=!1){if(!Mt)return;let ni=Te.animations.find(zt=>zt.name===vt);if(!ni)return;let rn=Mt.clipAction(ni);if(nt===rn)return rn;let sn=Pt;return rn.reset(),dn&&(rn.setLoop(ec,1),rn.clampWhenFinished=!0),rn.play(),nt&&rn.crossFadeFrom(nt,.25,!1),nt=rn,Pt=vt,It&&sn&&vt!==sn&&vt!=="walk"&&e.onSound?.("robot_"+vt,ye.position.x,ye.position.y,ye.position.z,Je),rn}}}function Ye(B){B.updateMatrixWorld(!0);let q=new Map;B.traverse(Y=>{if(!Y.isMesh)return;let A=Y.geometry.uuid+":"+(Array.isArray(Y.material)?Y.material.map(z=>z.uuid).join(","):Y.material.uuid);q.has(A)||q.set(A,{n:Y,matrices:[]}),q.get(A).matrices.push(Y.matrixWorld.clone())});let D=new lt;t.add(D);for(let{n:Y,matrices:A}of q.values()){let z=new _n(Y.geometry,Y.material,A.length);A.forEach((ee,ne)=>z.setMatrixAt(ne,ee)),z.castShadow=!0,z.receiveShadow=!0,z.userData={...Y.userData},v.push(z),D.add(z)}return B.traverse(Y=>{let A=v.indexOf(Y);A>=0&&v.splice(A,1)}),B.removeFromParent(),d.set(D.uuid,D),D}function Qe(B){if(f.has(B.id))return f.get(B.id);let q=(async()=>{let D=new lt;t.add(D);let Y=[];for(let A of Pp())Y.push(ke("floor",B.x+A.x,B.z+A.z,ft.room,0,D,[A.sx,1,A.sz]));for(let A=-4;A<=4;A+=4)for(let z=-4;z<=4;z+=4)Y.push(ke("ceiling",B.x+A,B.z+z,8.15+ft.room,0,D));for(let A=-4;A<=4;A+=4)Y.push(ke("wall",B.x+A,B.z+6,ft.room,0,D));for(let A=-4;A<=4;A+=4)for(let z of[-1,1])for(let ee of[0,4])Y.push(ke("window",B.x+z*6,B.z+A,ft.room+ee,Math.PI/2,D));for(let A=-4;A<=4;A+=4)for(let z of[-1,1])Y.push(ke("window",B.x+A,B.z+z*6,ft.gallery,0,D));for(let A of[-4,4])Y.push(ke("wall",B.x+A,B.doorZ,ft.room,0,D));for(let[A,z]of[[-4,1],[.2,1.1]])Y.push(ke("floor",B.x+A,B.z+3,ft.gallery,0,D,[z,1,1.5])),Y.push(ke("railing",B.x+A,B.z,ft.gallery,0,D,[z,1,1]));for(let A of[.7,5.3])Y.push(ke("railing",B.x+2.4,B.z+A,ft.gallery,Math.PI/2,D,[.35,1,1]));for(let A of[-5.6,1.9])Y.push(ke("wall",B.x+A,B.z+.15,ft.room,0,D,[.05,1,.6]));if(await Promise.all(Y),!C)return Ye(D)})();return f.set(B.id,q),q}async function Q(B){if(!(c.has(B.id)||C)){c.set(B.id,{ready:!1,lift:null,liftValue:0,liftTarget:0}),V++;try{await Qe(B);let q=new lt;t.add(q);let D=[];for(let A of[-3,0])D.push(ke(B.id==="missions"?"cargo":B.id==="memory"?"archive-shelf":"console",B.x+A,B.z+3,ft.room,Math.PI,q));if(D.push(ke("bench",B.x-4,B.z-1,ft.room,Math.PI/2,q)),D.push(ke("bench",B.x-4,B.z+4.8,ft.gallery,0,q)),D.push(ke("console",B.x,B.z+4.8,ft.gallery,Math.PI,q)),await Promise.all(D),C)return;Ye(q);let Y=c.get(B.id);Y.hologram=await ke("hologram",B.x-1,B.z,ft.room),Y.hologram?.play("operate"),B.id==="memory"&&X(),Y.lift=await ke("lift",B.liftX,B.liftZ,ft.room),Y.liftAction=Y.lift?.play("operate",!0),Y.liftAction&&(Y.liftAction.paused=!0),await E(B.id,B.x-2.2,B.z+.9,ft.gallery,Math.PI,{skip:null,limit:.9,pitch:[-.14,.18],eye:0}),Y.ready=!0}catch(q){C||e.onError?.(q)}finally{V--}}}async function st(){try{let B=await fetch(e.assetURL("manifest.json"),{signal:T.signal});if(!B.ok)throw Error("World manifest unavailable");I=await B.json();let q=new lt;t.add(q);let D=[];for(let A of Vn)D.push(ke("station",A.platformX,A.platformZ,ft.pavement,A.angle,q));for(let A of un){D.push(Qe(A));let z=await ke("door",A.x,A.doorZ,ft.room,Math.PI);if(C)return;let ee=z.play("open",!0);ee.paused=!0,h.set(A.id,{...z,action:ee,value:0,open:!1})}for(let A=-76;A<=76;A+=8)D.push(ke("quay",A,77,ft.quay,0,q));for(let[A,z]of[[-79,68],[-79,-67],[55,2],[8,40],[55,-24]])D.push(ke("garden",A,z,0,0,q)),D.push(ke("bench",A+3,z,0,Math.PI/2,q));for(let[A,z]of[[-30,23],[30,23],[-28,72]])D.push(ke("arcade",A,z,0,0,q));D.push(ke("bridge",-78.5,40,2,0,q,[1.75,1,1]),ke("ramp",-80,49,0,Math.PI,q),ke("stairs",-76.8,49,0,Math.PI,q),ke("ramp",-78.5,31,0,0,q)),D.push(ke("pad",yi.x,yi.z,0,0,q)),D.push(ke("sky-deck",Et.x,Et.z,Et.floor,0,q));for(let A of Et.benches)D.push(ke("bench",A.x,A.z,Et.floor,A.angle,q));for(let[A,z]of[[-60,-56],[-60,-46],[51,49]])D.push(ke("charger",A,z,0,0,q));if(await Promise.all(D),C||(Ye(q),re.lift=await ke("sky-lift",Et.lift.x,Et.lift.z,Et.lift.base),C))return;re.cab=re.lift?.node.getObjectByName("cab")||null,re.ready=!!re.cab;for(let A of Et.telescopes)await E(A.id,A.x,A.z,Et.floor,A.angle,{skip:"agent",limit:1.45,pitch:[-1.1,.3],eye:1.25});for(let[A,z]of[[-57,-61],[-57,-52]])(await ke("cooler",A,z))?.play("operate");for(let A of cs){let z=await ke(A.id,A.x,A.z,ls(A.x,A.z),0,t,[1,1,1],2);if(C)return;z&&(z.level=2,Re.set(A.id,z),ge.attach(A.id,z),he.addPlace(A,I.assets.find(ee=>ee.id===A.id)),z.play("operate"))}for(let A of Dp){let z=await ke(A.id,A.x,A.z,A.y,A.angle||0,t);if(!z){if(C)return;continue}z.play(A.clip),u.push({id:A.id,model:z})}let Y=await ke("service-cart",55,49,ft.ground,Math.PI/2);Y&&Y.play("open",!0);for(let A=0;A<19;A++){let z=["courier","technician","archivist"][A%3],ee=await ke("robot-"+z,0,0);if(!ee)return;ee.node.scale.setScalar(1.15),a.push(ee),he.add(ee,A,ee.bounds),ee.play("idle")}for(let A=0;A<2;A++){let z=await ke("tram",0,0);if(!z)return;let ee=z.play("open",!0);ee.paused=!0;let ne=Me[A?3:0].u;fe.getPointAt(ne,R),fe.getTangentAt(ne,G);let Te=le.register("tram-"+A,Oi(z.bounds),{x:R.x,y:ft.road+.03,z:R.z,heading:Math.atan2(G.x,G.z)},3);l.push({...z,body:Te,u:ne,speed:0,dwell:5,stop:A?3:0,door:ee})}for(let A=0;A<3;A++){let z=await ke("cargo",43,70);z&&(le.removeOwner(z.owner),z.node.visible=!1,p.push({...z,body:null,elapsed:9,direction:1}))}e.onReady?.()}catch(B){C||e.onError?.(B)}}st();async function $e(B,q,D){if(!(q.level===D||q.requested===D)){q.requested=D;try{let Y=await qe(B,D);if(C||q.requested!==D)return;let A=new Map;Y.scene.traverse(z=>{z.isMesh&&A.set(z.name,z)}),q.node.traverse(z=>{let ee=A.get(z.name);z.isMesh&&ee&&(z.geometry=ee.geometry,z.material=ee.material)}),q.level=D,q.requested=null,e.onReady?.()}catch(Y){q.requested=null,C||e.onError?.(Y)}}}function X(){let B=c.get("memory");if(!B?.hologram||!B.text&&!x.length)return;if(!B.text){let z=document.createElement("canvas");z.width=1024,z.height=512;let ee=new ws(z);ee.colorSpace=Ft,m.add(ee);let ne=new nn(4.8,2.4),Te=new wt({map:ee,transparent:!0,depthWrite:!1,side:Rt,toneMapped:!1});i.add(ne),r.add(Te);let ye=new Ke(ne,Te),je=un.find(Je=>Je.id==="memory");ye.position.set(je.x-1,3.6+ft.room,je.z+.3),ye.rotation.y=Math.PI,t.add(ye),B.text={canvas:z,texture:ee,mesh:ye}}let{canvas:q,texture:D,mesh:Y}=B.text,A=q.getContext("2d");A.clearRect(0,0,1024,512),Y.visible=x.length>0,A.fillStyle="rgba(5,24,32,.87)",A.fillRect(0,0,1024,512),A.fillStyle="#b9f4ef",A.font="28px sans-serif",x.slice(0,4).forEach((z,ee)=>{let ne=Array.from(z).slice(0,96);A.fillText(ne.slice(0,48).join(""),30,55+ee*118),A.fillText(ne.slice(48).join(""),30,94+ee*118)}),D.needsUpdate=!0}async function E(B,q,D,Y,A,z){let ee=await ke("telescope",q,D,Y,A);!ee||C||Ae.push({id:B,model:ee,tube:ee.node.getObjectByName("tube"),x:q,z:D,floor:Y,angle:A,...z,ex:q+Math.sin(A)*z.eye,ez:D+Math.cos(A)*z.eye})}function ae(B){let q=Ae.find(ne=>ne.id===B);if(!q)return;let D=be(q.angle+Math.PI),Y=q.floor+2.05,A=D,z=-.08,ee=1/0;for(let ne of e.districts||[]){if(ne.id===q.skip)continue;let Te=ne.x-q.ex,ye=ne.z-q.ez,je=Math.hypot(Te,ye),Je=Math.atan2(-Te,-ye),Mt=Math.abs(be(Je-D));je>12&&Mt<q.limit&&Mt<ee&&(ee=Mt,A=Je,z=_t.clamp(Math.atan2(ne.height*.45-Y,je),...q.pitch))}w={kind:"scope",id:B,scope:q,yaw0:D,fov:14,sent:"",target:null,back:{position:k.position.clone(),quaternion:k.quaternion.clone(),fov:k.fov}},q.model.node.visible=!1,k.position.set(q.ex,Y,q.ez),k.quaternion.setFromEuler(Le.set(z,A,0,"YXZ")),e.reduced()&&(k.fov=w.fov,k.updateProjectionMatrix()),e.onRide?.("scope"),Ee("door",k.position)}function se(B,q,D){let Y=D.radius*.6,A=B.x-D.x,z=B.z-D.z,ee=q.x*q.x+q.z*q.z;if(ee<1e-9)return 1/0;let ne=2*(A*q.x+z*q.z),Te=ne*ne-4*ee*(A*A+z*z-Y*Y);if(Te<0)return 1/0;let ye=(-ne-Math.sqrt(Te))/(2*ee);if(ye<0&&(ye=(-ne+Math.sqrt(Te))/(2*ee)),ye<0)return 1/0;let je=B.y+q.y*ye;return je>=0&&je<=D.height?ye:1/0}function _e(B){let q=w.scope;Le.setFromQuaternion(k.quaternion,"YXZ"),Le.y=w.yaw0+_t.clamp(be(Le.y-w.yaw0),-q.limit,q.limit),Le.x=_t.clamp(Le.x,...q.pitch),Le.z=0,k.quaternion.setFromEuler(Le),k.position.set(q.ex,q.floor+2.05,q.ez);let D=e.reduced()?w.fov:_t.damp(k.fov,w.fov,9,Math.min(B,.1));Math.abs(D-w.fov)<.02&&(D=w.fov),D!==k.fov&&(k.fov=D,k.updateProjectionMatrix()),k.getWorldDirection(R);let Y=null,A=1/0;for(let Te of e.districts||[]){if(Te.id===q.skip)continue;let ye=se(k.position,R,Te);ye>12&&ye<A&&(A=ye,Y=Te.id)}w.target=Y;let z=w.back.fov/k.fov,ee=(Math.atan2(R.x,-R.z)*180/Math.PI+360)%360,ne=[Y,Y?Math.round(A):0,z.toFixed(1),Math.round(ee)].join();ne!==w.sent&&(w.sent=ne,e.onScope?.({id:w.id,target:Y,distance:Y?Math.round(A):null,zoom:+z.toFixed(1),bearing:Math.round(ee)%360}))}function K(B,q,D){re.motion={from:re.value,to:B,t:0,duration:e.reduced()?0:q},re.idle=0,D&&Ee("lift",re.lift.node.position)}function te(){let B=Et.lift;k.position.set(B.x,B.base+B.cab+re.value+2.4,B.z)}function J(B){K(B,15,!0),w={kind:"sky",to:B},te(),k.quaternion.setFromEuler(Le.set(-.42,Math.PI,0,"YXZ")),e.onRide?.("sky")}function $(B,q){let D=Et.lift;if(!re.motion&&w?.kind!=="sky"&&q>0&&Math.hypot(k.position.x-D.x,k.position.z-D.z)>30&&(re.idle+=q,re.idle>28&&K(re.value>1?0:D.travel,15,!1)),re.motion){let Y=re.motion;Y.t+=Math.min(B,.1);let A=Y.duration?Y.t/Y.duration:1;re.value=Y.from+(Y.to-Y.from)*Fe(A),A>=1&&(re.value=Y.to,re.motion=null)}re.cab.position.y=re.value,w?.kind==="sky"&&(te(),re.motion||(w=null,e.onRide?.(null),Ee("lift",k.position)))}function xe(){if(w){if(w.kind==="tram"&&l[w.index].dwell<=1){w.exitRequested=!0;return}ve();return}if(H){if(H.kind==="telescope"){ae(H.id);return}if(H.kind==="skycall"){K(k.position.y>40?Et.lift.travel:0,7,!0);return}if(H.kind==="skyup"||H.kind==="skydown"){J(H.kind==="skyup"?Et.lift.travel:0);return}if(H.kind==="resident"){he.inspect(H.id);return}if(H.kind==="demonstrate"){he.demonstrate(H.id,e.reduced()),e.onSociety?.({id:H.id,role:"installation",state:e.reduced()?"idle":"work",source:"ambient"});return}if(H.kind==="door"){let B=h.get(H.id);if(B){B.open=!B.open,Ee("door",B.node.position);let q=un.find(D=>D.id===H.id);Q(q)}}if(H.kind==="discover"){L.add(H.id);try{localStorage.setItem("aurago.desktop.sysworld.discoveries",JSON.stringify([...L]))}catch{}e.onDiscover?.(H.id),Ee("discover",k.position)}if(H.kind==="tram"&&!e.reduced()&&(w={kind:"waiting",station:H.id},e.onRide?.("waiting")),H.kind==="terminal"&&e.onTerminal?.(H.id),H.kind==="drone"&&!e.reduced()&&(w={kind:"drone",elapsed:0,origin:k.position.clone()},e.onRide?.("drone"),Ee("tram",k.position)),H.kind==="lift"){let B=c.get(H.id);if(B){let q=k.position.y>5?4:0;if(Math.abs(B.liftValue-q)>.02){B.liftTarget=q;return}B.liftTarget=q?0:4,w={kind:"lift",id:H.id},e.onRide?.("lift"),Ee("lift",k.position)}}}}function ve(){if(w){if(w.kind==="lift"){let B=c.get(w.id),q=un.find(D=>D.id===w.id);B.liftTarget=B.liftValue<2?0:4,k.position.set(q.x+1.5,2.4+ft.room+B.liftTarget,q.liftZ)}if(w.kind==="tram"){let B=Vn.find(q=>q.id===w.station)||Vn[0];k.position.set(B.platformX,2.4+ft.pavement+.3,B.platformZ)}if(w.kind==="drone"&&k.position.copy(w.origin),w.kind==="sky"&&(re.motion=null,re.value=w.to,re.cab.position.y=re.value,te()),w.kind==="scope"){let B=w.scope;Le.setFromQuaternion(k.quaternion,"YXZ"),B.tube?.rotation.set(-Le.x,be(Le.y+Math.PI-B.angle),0,"YXZ");let q=Le.y;B.model.node.visible=!0,k.position.copy(w.back.position),Le.setFromQuaternion(w.back.quaternion,"YXZ"),Le.y=q,k.quaternion.setFromEuler(Le),k.fov=w.back.fov,k.updateProjectionMatrix(),e.onScope?.(null)}w=null,e.onRide?.(null)}}function me(){if(w)return[{kind:w.kind==="scope"?"unscope":"exit",id:w.kind,distance:0}];k.getWorldDirection(R);let B=he.nearby().filter(D=>(D.x-k.position.x)*R.x+(D.z-k.position.z)*R.z>D.distance*.3),q=(D,Y,A,z,ee)=>{let ne=Math.hypot(k.position.x-A,k.position.z-z);ne<ee&&B.push({kind:D,id:Y,distance:ne})};for(let D of un)k.position.y<5&&q("door",D.id,D.x,D.doorZ,3),c.get(D.id)?.ready&&(q("lift",D.id,D.liftX,D.liftZ,2.3),k.position.y<5&&q("terminal",D.id,D.x-2,D.z+3,2.5));if(k.position.y<5)for(let D of Vn)q("tram",D.id,D.platformX,D.platformZ,4),L.has(D.id)||q("discover",D.id,D.platformX+(D.angle===0?5:0),D.platformZ+(D.angle===0?0:5),3);if(q("drone","drone",yi.x,yi.z,5),re.ready){let D=Et.lift,Y=k.position.y>40,A=Math.hypot(k.position.x-D.x,k.position.z-D.z),z=Y?D.travel:0;A<(Y?3.6:2.4)&&(Y||k.position.y<10)&&B.push({kind:re.motion||Math.abs(re.value-z)>.05?"skycall":Y?"skydown":"skyup",id:"sky",distance:A})}for(let D of Ae)Math.abs(k.position.y-(D.floor+2.4))<1.5&&q("telescope",D.id,D.x,D.z,1.7);return B.sort((D,Y)=>D.distance-Y.distance)}function pe(B,q,D){if(C||!I)return;let Y=q?Math.min(B,.05):0;if(N+=Y,O=Us(k.position.x,k.position.z)?.id||null,Se+=B,Se>.5){Se=0;for(let[A,z]of Re){let ee=k.position.distanceTo(z.node.position),ne=_==="low"||ee>55?2:_==="medium"||ee>32?1:0;$e(A,z,ne)}}e.traffic||le.begin();for(let A of un)Math.hypot(k.position.x-A.x,k.position.z-A.z)<32&&Q(A);o.forEach(A=>{A.getRoot().visible&&A.update(Y)});for(let A of h.values()){let z=_t.damp(A.value,A.open?1:0,5,Math.min(B,.1));Math.abs(z-(A.open?1:0))<.001&&(z=A.open?1:0);let ee=ne=>[-1,1].map(Te=>({owner:A.owner,x:A.node.position.x,z:A.node.position.z,min:[Te*.8+Te*1.6*ne-.8,ft.room,-.13],max:[Te*.8+Te*1.6*ne+.8,ft.room+3.6,.13]}));(A.open||ee(z).every(ne=>le.solidClear(ne)))&&(A.value=z),ee(A.value).forEach((ne,Te)=>le.solid(A.owner+":panel:"+Te,ne)),A.action.time=A.value*A.action.getClip().duration,A.mixer.update(0)}re.ready&&$(B,Y),w?.kind==="scope"&&_e(B);for(let[A,z]of c)if(z.liftAction&&(z.liftValue=_t.damp(z.liftValue,z.liftTarget,1.8,Math.min(B,.1)),Math.abs(z.liftValue-z.liftTarget)<.02&&(z.liftValue=z.liftTarget),z.liftAction.time=z.liftValue/4*z.liftAction.getClip().duration,z.lift.mixer.update(0),w?.kind==="lift"&&w.id===A)){let ee=un.find(ne=>ne.id===A);k.position.set(ee.liftX,2.4+ft.room+z.liftValue,ee.liftZ),z.liftValue===z.liftTarget&&(w=null,e.onRide?.(null))}if(he.update(Y,q),ge.update(Y,q),l.forEach((A,z)=>{if(A.node.visible=!!A.body&&z<(_==="low"?1:2),A.body&&(A.node.visible&&!A.body.enabled&&!le.clear({...A.body,enabled:!0},A.body,A.body)&&(A.node.visible=!1),A.body.enabled=A.node.visible),!A.node.visible)return;let ee={u:A.u,dwell:A.dwell,stop:A.stop},ne=!1;if(A.dwell>0)A.dwell-=Y,A.speed=0;else{let Te=Pt=>(Pt.u-A.u+1)%1*ue,ye=Me.reduce((Pt,It)=>Te(It)>1e-5&&Te(It)<Te(Pt)?It:Pt,Me.find(Pt=>Te(Pt)>1e-5)||Me[0]),je=Math.min(11,Math.sqrt(Te(ye)*8));for(let Pt=1;Pt<=3;Pt++){let It=(A.u+(1.2+A.speed*.7)*Pt/3/ue)%1;fe.getPointAt(It,R),fe.getTangentAt(It,G);let vt={x:R.x,y:ft.road+.03,z:R.z,heading:Math.atan2(G.x,G.z)};if(!le.clear(A.body,vt,vt)){je=0;break}}A.speed+=_t.clamp(je-A.speed,-Y*8,Y*3.5);let Je=Y*A.speed/ue,Mt=A.u;A.u=(A.u+Je)%1;let nt=Me.findIndex(Pt=>(Pt.u-Mt+1)%1>0&&(Pt.u-Mt+1)%1<=Je+1e-5);nt>=0&&(A.stop=nt,A.u=Me[nt].u,A.dwell=6,A.speed=0,ne=!0)}if(fe.getPointAt(A.u,A.node.position),A.node.position.y=ft.road+.03,fe.getTangentAt(A.u,G),A.node.rotation.y=Math.atan2(G.x,G.z),A.body&&(A.body.enabled=A.node.visible,le.propose(A.body,{...A.node.position,heading:A.node.rotation.y},(Te,ye)=>{Te||(Object.assign(A,ee),A.speed=0),A.node.position.set(ye.x,ye.y,ye.z),A.node.rotation.y=ye.heading,Te&&ne&&Ee("tram",A.node.position)})),A.door.time=(A.dwell>1?1:0)*A.door.getClip().duration,A.mixer.update(0),w?.kind==="waiting"&&Me[A.stop].id===w.station&&A.dwell>1&&(w={kind:"tram",index:z,station:w.station,offset:0},e.onRide?.("tram")),w?.kind==="tram"&&w.index===z){if(A.dwell>1&&(w.station=Me[A.stop].id,w.exitRequested)){ve();return}k.position.copy(A.node.position).addScaledVector(G,w.offset),k.position.y+=.9+1.4,k.lookAt(A.node.position.x+G.x*16,k.position.y,A.node.position.z+G.z*16)}}),w?.kind==="drone"){w.elapsed+=Math.min(B,.1);let A=w.elapsed/40*Math.PI*2;k.position.set(Math.cos(A)*100,45+Math.sin(A*2)*12,Math.sin(A)*95-10),k.lookAt(0,22,-12),w.elapsed>=40&&ve()}if(H=D==="street"&&me()[0]||null,D==="street"&&!w){let A=k.position.distanceTo(oe);A<3&&(F+=A),F>1.8&&(Ee(O?"step_inside":"step",k.position),F=0)}oe.copy(k.position),e.onEnvironment?.(!!O);for(let[A,z]of p.entries()){if(z.elapsed+=Y,z.node.visible=q&&z.elapsed<8,!z.node.visible){z.body&&le.remove(z.body.id),z.body=null;continue}let ee=z.direction>0?z.elapsed/8:1-z.elapsed/8,ne={x:40+ee*3,y:ft.room,z:70,heading:0};if(z.body||(z.body=le.register("freight-"+A,Oi(z.bounds),ne,0)),!z.body){z.elapsed=9,z.node.visible=!1;continue}le.propose(z.body,ne,(Te,ye)=>{Te||(z.elapsed-=Y),z.node.position.set(ye.x,ye.y,ye.z)})}e.traffic||le.solve(Y),e.onInteraction?.(H,L.size,O)}async function we(B){if(_=B,!I)return;let q=B==="low"?2:B==="medium"?1:0;he.setTier(B),ge.setTier(B);try{let D=new Map(await Promise.all([...g].filter(Y=>!Re.has(Y)).map(async Y=>{let A=await qe(Y,q),z=new Map;return A.scene.traverse(ee=>{ee.isMesh&&z.set(ee.name,ee)}),[Y,z]})));if(C||_!==B)return;for(let Y of v){let A=D.get(Y.userData.worldAsset)?.get(Y.userData.worldPart);A&&(Y.geometry=A.geometry,Y.material=A.material)}e.onReady?.()}catch(D){C||e.onError?.(D)}}return{update:pe,interact:xe,endRide:ve,setTier:we,society:he,suspend(){he.suspend(),ge.clear(),S=!1;for(let B of p)B.elapsed=9,B.node.visible=!1,B.body&&le.remove(B.body.id),B.body=null},syncRide(){if(w?.kind==="tram"){let B=l[w.index];fe.getTangentAt(B.u,G),k.position.copy(B.node.position).addScaledVector(G,w.offset),k.position.y+=2.3,k.lookAt(B.node.position.x+G.x*16,k.position.y,B.node.position.z+G.z*16)}},socialAction(B,q){let D=Vn.find(Y=>Y.id===q);return he.action(e.reduced()&&B==="guide"?"cancel":B,D?{x:D.platformX,z:D.platformZ}:null)},setMemory(B){x=(Array.isArray(B)?B:[]).filter(q=>typeof q=="string").slice(0,8),X()},setWorld(B,q=!1){q?(he.setReplay(!0),ge.clear()):he.setReplay(!1);let D=B?.entities?.filter(A=>A.kind==="mission")||[],Y=new Set;for(let A of D){Y.add(A.id);let z=M.get(A.id);if(!q&&e.active()&&S&&z!=null&&z!==A.state&&["running","completed","failed","cancelled"].includes(A.state)&&Date.now()-A.at<3e4){let ee=p.find(ne=>ne.elapsed>=8);ee&&(ee.elapsed=0,ee.direction=A.state==="running"?1:-1,y++)}M.set(A.id,A.state)}for(let A of M.keys())Y.has(A)||M.delete(A);if(S=!q,q)for(let A of p)A.elapsed=9,A.node.visible=!1},walkRide(B,q,D=0){if(!w)return!1;if(w.kind==="tram"&&(w.offset=_t.clamp(w.offset+B*q*3,-2.5,2.5)),w.kind==="scope"){Le.setFromQuaternion(k.quaternion,"YXZ");let Y=q*.9*k.fov/w.back.fov;Le.x+=B*Y,Le.y-=D*Y,k.quaternion.setFromEuler(Le)}return!0},zoom(B){return w?.kind!=="scope"?!1:(w.fov=_t.clamp(w.fov*Math.exp(B*.0012),5,26),!0)},scoping:()=>w?.kind==="scope",move(B,q,D){if(w)return!1;if(D.y>40)return Ap(B,q,!re.motion&&re.value===Et.lift.travel);let Y=Us(D.x,D.z);return Y&&D.y>5?Math.abs(q-Y.z-4.8)<.85&&(Math.abs(B-Y.x)<1.1||Math.abs(B-Y.x+4)<1.75)?!1:Du(Y,B,q,c.get(Y.id)?.liftValue===4):Y&&D.y<5&&([-5.6,1.9].some(A=>Math.abs(B-Y.x-A)<.25&&Math.abs(q-Y.z-.15)<.25)||Cp(Y,B,q)&&!(B<Y.liftX+1.25&&Math.abs(q-Y.liftZ)<1.25&&c.get(Y.id)?.liftValue===0)||Math.hypot(B-(Y.x-1),q-Y.z)<1.55||[-3,0].some(A=>Math.abs(B-Y.x-A)<1.1&&Math.abs(q-Y.z-3)<.8))?!1:Lp(B,q,D,A=>h.get(A)?.value>.85&&!!c.get(A)?.ready,e.districts)},floor(B,q){if(k.position.y>40)return Et.floor;let D=Us(B,q);return D&&k.position.y>5&&Du(D,B,q,c.get(D.id)?.liftValue===4)?ft.gallery:Math.max(de(B,q),Ip(B,q,e.districts))},visit(B){ve(),w=null;let q=cs.find(Y=>Y.id===B);if(q){let Y=I?.assets.find(A=>A.id===B)?.navigation.interaction[0]?.position||[0,1.5,4];k.position.set(q.x,2.4+ls(q.x,q.z),q.z+Y[2]+1),k.lookAt(q.x,2,q.z);return}if(B==="drone"){k.position.set(yi.x,2.7,yi.z+3),k.lookAt(yi.x,2,yi.z);return}if(B==="skydeck"){let Y=57*Math.PI/180,A=Et.x+Math.sin(Y)*7.2,z=Et.z-Math.cos(Y)*7.2;k.position.set(A,Et.floor+2.4,z),k.lookAt(A,Et.floor+.4,z-10);return}let D=Vn.find(Y=>Y.id===B);if(D){let Y=L.has(B)?0:5,A=D.platformX+(D.angle===0?Y:0),z=D.platformZ+(D.angle===0?0:Y);k.position.set(A,2.4+ls(A,z),z),k.lookAt(D.platformX,2,D.platformZ===z?D.platformZ+1:D.platformZ)}},destination(B){let q=un.find(D=>D.id===B);if(q){ve(),w=null;let D=q.doorZ+q.front*4;k.position.set(q.x,2.4+ls(q.x,D),D),k.lookAt(q.x,2.4+ls(q.x,D),q.z)}},isRiding:()=>!!w,interaction:()=>H,rideBody:()=>w?.kind==="tram"?"tram-"+w.index:w?.kind==="scope"?w.scope.model.owner:w?.kind==="sky"&&re.lift?.owner||null,stats:()=>({society:he.stats(),machinery:ge.stats(),details:[...Re].map(([B,q])=>({id:B,level:q.level})),loaded:[...g],bytes:b,freightEvents:y,residents:a.filter(B=>B.node.visible).length,trams:l.filter(B=>B.node.visible).length,rooms:[...c].filter(([,B])=>B.ready).map(([B])=>B),inside:O,ride:w?.kind||null,station:w?.station||null,discovered:[...L],loading:V,interactions:H?{kind:H.kind,id:H.id}:null,landmarks:u.map(B=>B.id),sky:{ready:re.ready,value:+re.value.toFixed(2),moving:!!re.motion},telescopes:Ae.length,scope:w?.kind==="scope"?{id:w.id,target:w.target,fov:+k.fov.toFixed(1)}:null}),dispose(){C||(C=!0,T.abort(),he.dispose(),ge.dispose(),e.traffic||le.dispose(),x=[],M.clear(),t.removeFromParent(),o.forEach(B=>{B.stopAllAction(),B.uncacheRoot(B.getRoot())}),t.traverse(B=>{B.isInstancedMesh&&B.dispose()}),i.forEach(B=>B.dispose()),r.forEach(B=>B.dispose()),m.forEach(B=>B.dispose()),n.clear())}}}var Hy=s=>.5+(s-6)/12*3.62,zp=new W;function kp(s,{sun:e,rim:t,hemisphere:n,atmosphere:i,surfaces:r,onThunder:o}){let a="local",l="clear",c="high",h=!1,u=-100,d=!1,f=n.intensity,g=14,v=-1,m=0,p={value:0},M=new ot,T=[],x=317,y=()=>(x=Math.imul(x,1664525)+1013904223>>>0)/4294967296;for(let F=0;F<1200;F++)T.push((y()-.5)*170,y()*60,(y()-.5)*174-7);M.setAttribute("position",new tt(T,3));let S=new ct({transparent:!0,depthWrite:!1,uniforms:{time:p,flash:{value:0}},vertexShader:"uniform float time;varying float vFade;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;gl_PointSize=clamp(260.0/max(1.0,-mv.z),2.0,26.0);vFade=smoothstep(0.0,4.0,p.y)*smoothstep(460.0,60.0,-mv.z);}",fragmentShader:"uniform float flash;varying float vFade;void main(){vec2 c=gl_PointCoord-.5;float a=(1.0-smoothstep(0.0,0.07,abs(c.x)))*(1.0-smoothstep(0.2,0.5,abs(c.y)))*.42*vFade;if(a<.004)discard;gl_FragColor=vec4(mix(vec3(.6,.78,.86),vec3(.95,.97,1.0),flash),a*(1.0+flash));}"}),C=new tn(M,S);C.frustumCulled=!1,s.add(C);let _=new Ce(9417679),I=new Ce(528926),b=new Ce(2762296);function N(){let F=wc(a),L=F.amount,O=Hy(F.hour);e.intensity=.8+L*2.6,e.color.set(F.evening>.1?16757370:13164287),e.position.set(Math.cos(F.hour/24*Math.PI*2)*120,40+L*115,85),zp.set(Math.cos(O)*120,40-F.evening*18+L*115,Math.sin(O)*120),t.intensity=.7+F.evening*1.6,f=n.intensity=.45+L*.45,s.fog.color.copy(I).lerp(_,L).lerp(b,F.evening*(1-L)*.5),s.fog.density=l==="fog"?.018:l==="rain"?.008:.003-L*.0016,i.setLighting?.(L,F.evening,zp),i.setWeather?.(l),r?.setLighting(L,F.evening,l==="clear"?0:1,s.fog.color),r?.setWeather(l),C.visible=l==="rain"&&!h,M.setDrawRange(0,c==="low"?200:1200)}function w(F,L){if(l!=="rain"||h||!L){v>=0&&(v=-1,H(0));return}if(g-=F,v<0&&g<=0&&(v=0,m++,g=9+y()*16,o?.(.6+y()*3.2)),v<0)return;v+=F;let O=v<.09?1:v<.16?.25:v<.24?.75:Math.max(0,1-(v-.24)/.35)*.5;H(O),v>.6&&(v=-1,H(0))}function H(F){n.intensity=f+F*2.4,S.uniforms.flash.value=F,i.setFlash?.(F)}return N(),{set(F={}){["local","day","evening","night"].includes(F.time)&&(a=F.time),["clear","rain","fog"].includes(F.weather)&&(l=F.weather),N()},setTier(F){c=F,N()},setIndoor(F){h!==F&&(h=F,N())},update(F,L,O){if(!d)return C.visible=l==="rain"&&!h&&O,O&&(p.value+=Math.min(F,.1)),w(Math.min(F,.1),O),L-u>30?(u=L,N(),!0):!1},mood(){let F=wc(a);return{day:F.amount,evening:F.evening,weather:l,indoor:h}},stats:()=>({time:a,weather:l,indoor:h,daylight:wc(a).amount,strikes:m}),dispose(){d=!0,C.removeFromParent(),M.dispose(),S.dispose()}}}function Hp(s,e,t,n){for(let i of t){let r=e.get(i.asset).lods[0].bounds;(Nu[i.id]||(i.id==="graph"?[[-10.2,-10.2,10.2,10.2]]:[[-3,-3,3,3]])).forEach(([a,l,c,h],u)=>s.solid("district:"+i.id+":"+u,{x:i.x,z:i.z,min:[a,0,l],max:[c,8,h]})),Uu[i.id]?Uu[i.id].forEach(([a,l,c],h)=>s.solid("district:"+i.id+":upper:"+h,{x:i.x,z:i.z,min:[-a,l,-a],max:[a,c,a]})):s.solid("district:"+i.id+":upper",{x:i.x,z:i.z,min:[r.min[0],8,r.min[2]],max:r.max})}n.forEach((i,r)=>{if(i.asset==="street-tile"||i.asset==="street-crossing")return;let o=e.get(i.asset)?.lods[0].bounds;if(!o)return;let a=o.min.map((c,h)=>c*i.scale[h]),l=o.max.map((c,h)=>c*i.scale[h]);a[1]+=i.y,l[1]+=i.y,i.asset==="street-lamp"&&(s.solid("city:"+r+":pole",{x:i.x,z:i.z,min:[-.2,i.y,-.2],max:[.2,l[1],.2]}),a[1]=Math.max(a[1],l[1]-1)),s.solid("city:"+r,{x:i.x,z:i.z,heading:i.angle,min:a,max:l})})}var ea={"city.road":{top:"asphalt",side:"concrete",albedo:.55,rough:.7,bump:.012,wet:1,puddle:[.447,.442]},"city.stone":{top:"pavers",side:"concrete",albedo:.5,rough:.6,bump:.014,wet:1,puddle:[.523,.516]},"city.graphite":{top:"gravel",side:"panels",albedo:.42,rough:.5,bump:.016,wet:.8,metalTop:.3},"city.titanium":{top:"panels",side:"panels",albedo:.2,rough:.45,bump:.006,wet:.6},"city.bronze":{top:"panels",side:"panels",albedo:.18,rough:.4,bump:.006,wet:.6},"city.ceramic":{top:"concrete",side:"concrete",albedo:.3,rough:.4,bump:.008,wet:.8,puddle:[.464,.46]},"city.leaf":{top:"foliage",side:"foliage",albedo:.7,rough:.3,bump:.03,wet:.5},ground:{top:"pavers",side:"concrete",albedo:.55,rough:.6,bump:.014,wet:1,topScale:.5,puddle:[.523,.516],waterline:!0}},Vp=new Set(["city.ivory","city.warm"]),Vy={low:0,medium:1,high:2,ultra:2},Fs=s=>Number(s).toFixed(4),Gy=`varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
attribute float swSeed;varying float vSwSeed;
#endif`,Wy=`
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
#endif`,Xy=`uniform float swWet;uniform float swRain;uniform float swTime;uniform vec4 swWindow;uniform vec2 swFade;uniform vec4 swShelter[3];uniform vec3 swSky;
varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
varying float vSwSeed;
#endif
#ifdef SW_SURFACE
uniform sampler2D swTop;uniform sampler2D swSide;uniform vec2 swScale;
float swHash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
vec3 swPerturb(vec3 pos,vec3 n,vec2 dh,float face){vec3 sx=dFdx(pos),sy=dFdy(pos),r1=cross(sy,n),r2=cross(n,sx);float det=dot(sx,r1)*face;return normalize(abs(det)*n-sign(det)*(dh.x*r1+dh.y*r2));}
#endif`,qy=`
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
#endif`,Yy=`
#ifdef SW_SURFACE
diffuseColor.rgb*=swAlbedoF*mix(1.0,0.56,swWetness)*mix(1.0,0.7,swPuddle);
#endif`,Zy=`
#ifdef SW_SURFACE
roughnessFactor=clamp(roughnessFactor*swRoughF,0.04,1.0);roughnessFactor=mix(roughnessFactor,roughnessFactor*0.6,swWetness);roughnessFactor=mix(roughnessFactor,0.04,swPuddle);
#endif`,Ky=`
#ifdef SW_SURFACE
metalnessFactor*=mix(1.0,SW_METAL_TOP,swW.y);
#endif`,jy=`
#if SW_LEVEL>1
normal=swPerturb(-vViewPosition,normal,vec2(dFdx(swH),dFdy(swH))*SW_BUMP*swDetail,faceDirection);
#endif`,Jy=`
#ifdef SW_SURFACE
float swFresnel=pow(1.0-clamp(dot(normal,normalize(vViewPosition)),0.0,1.0),5.0)*0.95+0.05;
totalEmissiveRadiance+=swSky*swFresnel*(swPuddle*0.6+swWetness*0.1);
#endif
#ifdef SW_WINDOWS
float swPane=(1.0-smoothstep(0.35,0.65,abs(swN.y)))*(1.0-step(1.5,vSwSeed));
totalEmissiveRadiance*=mix(swWindow.w,mix(swWindow.z,swWindow.y,step(vSwSeed,swWindow.x)),swPane);
#endif`;function $y(s){let e=s?.getAttribute("position");if(!e||s.getAttribute("swSeed"))return;let t=e.count,n=new Int32Array(t),i=s.getIndex();for(let u=0;u<t;u++)n[u]=u;let r=u=>{for(;n[u]!==u;)n[u]=n[n[u]],u=n[u];return u},o=u=>i?i.getX(u):u,a=i?i.count:t;for(let u=0;u+2<a;u+=3){let d=r(o(u));n[r(o(u+1))]=d,n[r(o(u+2))]=d}let l=new Map,c=new Map;for(let u=0;u<t;u++){let d=r(u),f=c.get(d)||[0,0,0];l.set(d,(l.get(d)||0)+1),f[0]+=e.getX(u),f[1]+=e.getY(u),f[2]+=e.getZ(u),c.set(d,f)}let h=new Float32Array(t);for(let u=0;u<t;u++){let d=r(u),f=l.get(d),g=c.get(d);if(f>6){h[u]=2;continue}let v=Math.sin(g[0]/f*12.9898+g[1]/f*78.233+g[2]/f*37.719)*43758.5453;h[u]=v-Math.floor(v)}s.setAttribute("swSeed",new dt(h,1))}function Gp(s,e={}){let t=new AbortController,n=new Map,i=[],r=new ji(new Uint8Array([128,128,128,255]),1,1);r.colorSpace=kn,r.needsUpdate=!0;let o={},a={},l={};for(let w of Object.values(ea))for(let H of[w.top,w.side])o[H]||={value:r};for(let w of Object.keys(ea))l[w]={value:new Ue(1/4,1/4)};let c=(e.shelters||[]).slice(0,3).map(w=>new gt(...w));for(;c.length<3;)c.push(new gt(1e6,1e6,-1e6,-1e6));let h={swWet:{value:0},swRain:{value:0},swTime:{value:0},swShelter:{value:c},swSky:{value:new Ce(528926)},swWindow:{value:new gt(.8,1,.06,1)},swFade:{value:new Ue(35,140)}},u=2,d=null,f=!1,g=0,v=0,m=0,p=0,M=!1,T=s.capabilities.getMaxAnisotropy(),x=()=>u>1?Math.min(8,T):Math.min(4,T);function y(w,H){let{kind:F,windows:L,space:O}=n.get(H),V=ea[F];Object.assign(w.uniforms,h);let R=["#define SW_LEVEL "+(V?u:0)];O==="world"&&R.push("#define SW_WORLD"),L&&R.push("#define SW_WINDOWS"),V&&(w.uniforms.swTop=o[V.top],w.uniforms.swSide=o[V.side],w.uniforms.swScale=l[F],R.push("#define SW_SURFACE","#define SW_ALBEDO "+Fs(V.albedo),"#define SW_ROUGH "+Fs(V.rough),"#define SW_BUMP "+Fs(V.bump),"#define SW_WETK "+Fs(O==="world"?V.wet:V.wet*.5),"#define SW_METAL_TOP "+Fs(V.metalTop??1),"#define SW_PUDDLE_DRY "+Fs(V.puddle?.[0]??-1),"#define SW_PUDDLE_FULL "+Fs(V.puddle?.[1]??-2)),V.waterline&&R.push("#define SW_WATERLINE"));let G=R.join(`
`)+`
`;w.vertexShader=G+w.vertexShader.replace("#include <common>",`#include <common>
`+Gy).replace("#include <begin_vertex>","#include <begin_vertex>"+Wy),w.fragmentShader=G+w.fragmentShader.replace("#include <common>",`#include <common>
`+Xy).replace("#include <map_fragment>","#include <map_fragment>"+qy).replace("#include <color_fragment>","#include <color_fragment>"+Yy).replace("#include <roughnessmap_fragment>","#include <roughnessmap_fragment>"+Zy).replace("#include <metalnessmap_fragment>","#include <metalnessmap_fragment>"+Ky).replace("#include <normal_fragment_maps>","#include <normal_fragment_maps>"+jy).replace("#include <emissivemap_fragment>","#include <emissivemap_fragment>"+Jy)}function S(w,H="world",F=w?.name){if(!w?.isMeshStandardMaterial||n.has(w))return;let L=ea[F]?F:"",O=Vp.has(w.name);!L&&!O||(n.set(w,{kind:L,windows:O,space:H}),w.addEventListener("dispose",()=>n.delete(w)),w.onBeforeCompile=V=>y(V,w),w.customProgramCacheKey=()=>["sw",L,O,H,L?u:0].join("|"),w.needsUpdate=!0)}function C(w,H=!1){if(w?.isMesh)for(let F of Array.isArray(w.material)?w.material:[w.material])S(F,H?"object":"world"),Vp.has(F?.name)&&$y(w.geometry)}let _=w=>e.url?.(w),I=null;async function b(w){let H=await fetch(_(w.file),{signal:t.signal});if(!H.ok)throw Error("Surface texture unavailable");let F=await H.blob();if(f)return;let L=await createImageBitmap(F,{premultiplyAlpha:"none",colorSpaceConversion:"none"});if(f){L.close?.();return}let O=new Gt(L);O.wrapS=O.wrapT=ai,O.colorSpace=kn,O.flipY=!1,O.anisotropy=x(),O.needsUpdate=!0,i.push(O),o[w.id].value=O,m++,g+=F.size,e.onProgress?.(g,v)}function N(){return d||f||!e.url||(d=(async()=>{if(!I){let F=await fetch(_("manifest.json"),{signal:t.signal});if(!F.ok)throw Error("Surface manifest unavailable");I=await F.json();let L=(I.textures||[]).filter(O=>o[O.id]&&/^[a-z-]+\.webp$/.test(O.file)&&O.tile_metres>0);I.entries=L,v=L.reduce((O,V)=>O+(V.bytes||0),0);for(let O of L)a[O.id]=O.tile_metres;for(let[O,V]of Object.entries(ea))l[O].value.set(1/((a[V.top]||4)/(V.topScale||1)),1/(a[V.side]||4))}let w=I.entries.filter(F=>o[F.id].value===r),H=await Promise.allSettled(w.map(b));p+=H.filter(F=>F.status==="rejected").length})().catch(()=>{p++}).finally(()=>{d=null})),d}return{apply:S,prepare:C,setTier(w){let H=Vy[w]??2;if(H!==u){u=H;for(let[F,L]of n)L.kind&&(F.needsUpdate=!0)}for(let F of i)F.anisotropy!==x()&&(F.anisotropy=x(),F.needsUpdate=!0);u>0&&N()},setLighting(w,H,F=0,L=null){L&&h.swSky.value.copy(L).multiplyScalar(1.25);let O=1-w,V=_t.smoothstep(O,.1,.9);h.swWindow.value.set(Math.min(.9,.12+.72*V+.12*F),.22+O*.98+Math.min(1,H)*.1,.05,.55+.45*O)},setWeather(w){M=w==="rain",h.swRain.value=M?1:0},update(w,H){let F=M?1:0,L=h.swWet.value;if(!H){h.swWet.value=F;return}let O=Math.min(.1,Math.max(0,w));h.swTime.value+=O,h.swWet.value=L+(F-L)*Math.min(1,O/(F>L?14:70))},progress:()=>({bytes:g,expected:v}),stats:()=>({detail:u,textures:m,failures:p,bytes:g,materials:n.size,wet:+h.swWet.value.toFixed(2),windows:+h.swWindow.value.x.toFixed(2)}),dispose(){f=!0,t.abort();for(let w of i)w.dispose(),w.source.data?.close?.();i.length=0,r.dispose()}}}function Qy(s){let e=Math.max(64,Math.round(s.sampleRate*.03)||1440),t=[],n=0;for(let o=0;o<s.numberOfChannels;o++){let a=s.getChannelData(o);for(let l=0;l<a.length;l+=e){let c=0,h=Math.min(l+e,a.length);for(let u=l;u<h;u++){let d=a[u];if(!Number.isFinite(d))throw Error("Invalid voice");n=Math.max(n,Math.abs(d)),c+=d*d}t.push(Math.sqrt(c/(h-l)))}}if(n<1e-4)throw Error("Silent voice");let i=t.filter(o=>o>Math.max(.003,n*.003)).sort((o,a)=>o-a),r=i[Math.floor((i.length-1)*.7)]||n*.5;return Math.min(8,Math.max(1/n,.26/Math.max(r,.001)))}function Wp(s,e){let t=!1,n=!1,i=0,r=null,o=null,a=[],l=null,c=null,h=null,u=250,d=0,f=0,g=0,v="idle",m=null,p=()=>.04+.46/(1+(u/110)**2);function M(){let _=s.currentTime;l?.gain.setTargetAtTime(p(),_,.18),c?.frequency.setTargetAtTime(1800+4400/(1+u/95),_,.18),h?.pan.setTargetAtTime(d,_,.18)}function T(){if(o){o.onended=null;try{o.stop()}catch{}o=null}a.forEach(_=>_.disconnect()),a=[],l=c=h=null}function x(){f++,clearTimeout(i),i=0,r?.abort(),r=null,T(),v="idle"}function y(_=2e4+Math.random()*2e4){clearTimeout(i),t&&!n&&(i=setTimeout(()=>{i=0,C()},_))}function S(){if(m)return m;m=s.createBuffer(2,Math.ceil(s.sampleRate*.85),s.sampleRate);let _=42;for(let I=0;I<2;I++){let b=m.getChannelData(I),N=Math.round(s.sampleRate*.015),w=0;for(let F=0;F<b.length;F++){_=Math.imul(_,1664525)+1013904223>>>0;let L=(F-N)/(b.length-N);b[F]=F<N?0:(_/4294967296*2-1)*Math.pow(1-L,2.25),w+=b[F]*b[F]}let H=.65/Math.sqrt(w||1);for(let F=0;F<b.length;F++)b[F]*=H}return m}async function C(){if(!t||n||r||o||s.state!=="running"){y();return}let _=f,I=new AbortController;r=I,v="loading";let b=setTimeout(()=>I.abort(),32e3);try{let N=await fetch("/api/desktop/system-world/voice",{method:"POST",credentials:"same-origin",cache:"no-store",signal:I.signal});if(N.status===204){v="idle";return}if(!N.ok||!N.headers.get("Content-Type")?.startsWith("audio/"))throw Error("Voice unavailable");if(Number(N.headers.get("Content-Length"))>8*1024*1024)throw Error("Voice too large");let w=await N.arrayBuffer();if(w.byteLength>8*1024*1024)throw Error("Voice too large");if(_!==f||!t||n)return;let H=await s.decodeAudioData(w);if(_!==f||!t||n)return;if(!Number.isFinite(H.duration)||H.duration<=0||H.duration>25||H.numberOfChannels>2)throw Error("Invalid voice");let F=Qy(H),L=Fe=>(a.push(Fe),Fe);o=L(s.createBufferSource()),o.buffer=H;let O=L(s.createGain()),V=L(s.createBiquadFilter());O.gain.value=F,V.type="highpass",V.frequency.value=90,c=L(s.createBiquadFilter()),c.type="lowpass",c.Q.value=.55;let R=L(s.createGain()),G=L(s.createGain()),oe=L(s.createGain()),k=L(s.createGain());R.gain.value=.76,G.gain.value=.22,oe.gain.value=.28;let re=L(s.createDelay(.2)),Ae=L(s.createConvolver());re.delayTime.value=.085,Ae.normalize=!1,Ae.buffer=S();let Le=L(s.createWaveShaper()),be=new Float32Array(1024);for(let Fe=0;Fe<be.length;Fe++)be[Fe]=Math.max(-.75,Math.min(.75,Fe*2/(be.length-1)-1));Le.curve=be,l=L(s.createGain()),l.gain.value=p(),h=L(s.createStereoPanner()),h.pan.value=d,o.connect(O).connect(V).connect(c),c.connect(R).connect(k),c.connect(re).connect(G).connect(k),c.connect(Ae).connect(oe).connect(k),k.connect(Le).connect(l).connect(h).connect(e),M(),v="speaking",g++,o.onended=()=>{_!==f||!t||n||(v="tail",i=setTimeout(()=>{T(),v="idle",y()},900))},o.start()}catch{_===f&&!n&&(v=I.signal.aborted?"idle":"unavailable")}finally{clearTimeout(b),r===I&&(r=null),_===f&&!n&&!o&&y(v==="unavailable"?6e4:void 0)}}return{setActive(_){t===_||n||(t=_,t?y(3e3+Math.random()*2e3):x())},setListener(_,I,b,N,w){if(!Number.isFinite(_)||!Number.isFinite(I)||!Number.isFinite(b)||!Number.isFinite(N)||!Number.isFinite(w))return;let H=-_,F=-12-b,L=Math.max(0,Math.hypot(H,F)-13);u=Math.hypot(L,Math.max(0,I-87,-I));let O=Math.hypot(H,F),V=Math.hypot(N,w);d=O&&V?Math.max(-.8,Math.min(.8,(F*N-H*w)/O/V))*.8:0,M()},stats(){return{state:v,phrases:g,distance:u,gain:p(),pan:d,pending:!!r,scheduled:!!i}},dispose(){n||(n=!0,t=!1,x(),m=null)}}}function Xp(s,e,t=e){let n=new Set,i={x:0,z:0,fx:0,fz:-1},r=!1,o=!1,a="clear",l=s.createBuffer(1,s.sampleRate*2,s.sampleRate),c=31,h=l.getChannelData(0);for(let p=0;p<h.length;p++)c=Math.imul(c,1664525)+1013904223|0,h[p]=(c>>>0)/2147483648-1;let u=s.createBufferSource(),d=s.createBiquadFilter(),f=s.createGain();u.buffer=l,u.loop=!0,d.type="lowpass",d.frequency.value=450,f.gain.value=0,u.connect(d).connect(f).connect(t),u.start();let g=[{x:-57,z:-57,f:94,g:.055},{x:43,z:62,f:143,g:.035},{x:0,z:14,f:220,g:.016},{x:0,z:79,f:340,g:.06}].map((p,M)=>{let T=M===3?s.createBufferSource():s.createOscillator(),x=s.createGain(),y=s.createStereoPanner(),S=s.createBiquadFilter();return M===3?(T.buffer=l,T.loop=!0):(T.type="sine",T.frequency.value=p.f),S.type="lowpass",S.frequency.value=p.f*2,x.gain.value=0,T.connect(S).connect(x).connect(y).connect(t),T.start(),{...p,source:T,level:x,pan:y,low:S}});function v(){f.gain.setTargetAtTime(r?(a==="rain"?.07:.015)*(o?.12:1):0,s.currentTime,.1),d.frequency.setTargetAtTime(o?280:a==="rain"?2100:600,s.currentTime,.2)}function m(){for(let p of[...n]){try{p.source.stop()}catch{}p.release()}}return{setActive(p){if(r=p,v(),!p){m();for(let M of g)M.level.gain.setTargetAtTime(0,s.currentTime,.05)}},environment(p,M){o=p,M&&(a=M),v()},listener(p,M,T,x,y){Object.assign(i,{x:p,z:T,fx:x,fz:y});for(let S of g){let C=Math.hypot(p-S.x,T-S.z);S.level.gain.setTargetAtTime(r?S.g/(1+C*C*.025)*(o?.25:1):0,s.currentTime,.1),S.pan.pan.value=Math.max(-1,Math.min(1,((S.x-p)*-y+(S.z-T)*x)/Math.max(1,C)))}},play(p,M=0,T=0,x=0){if(!r||n.size>=16)return;let y=Math.hypot(M-i.x,x-i.z);if(y>90)return;let S={step:[.12,240,.16],step_inside:[.1,550,.12],door:[.7,850,.06],lift:[1.5,120,.05],tram:[.8,330,.08],discover:[.45,880,.08]}[p];if(!S)return;let[C,_,I]=S,b=p.startsWith("step")||p==="door"?s.createBufferSource():s.createOscillator();"buffer"in b?b.buffer=l:(b.type="sine",b.frequency.value=_);let N=s.createBiquadFilter(),w=s.createGain(),H=s.createStereoPanner();N.type="lowpass",N.frequency.value=o?_*.7:_,w.gain.setValueAtTime(0,s.currentTime),w.gain.linearRampToValueAtTime(I/(1+y*.08),s.currentTime+.015),w.gain.exponentialRampToValueAtTime(1e-4,s.currentTime+C),H.pan.value=Math.max(-1,Math.min(1,((M-i.x)*-i.fz+(x-i.z)*i.fx)/Math.max(y,1))),b.connect(N).connect(w).connect(H).connect(e);let F={source:b,release(){if(n.delete(F))for(let L of[b,N,w,H])L.disconnect()}};n.add(F),b.onended=F.release,b.start(),b.stop(s.currentTime+C+.02)},stats:()=>({voices:n.size,weather:a,inside:o}),dispose(){r=!1,m(),u.stop();for(let p of g){p.source.stop();for(let M of[p.source,p.level,p.pan,p.low])M.disconnect()}for(let p of[u,d,f])p.disconnect()}}}function qp(s,e,t=e){let o=new Set,a=new Map,l=new Map,c={x:0,y:0,z:0,fx:0,fz:-1},h=!1,u=!1,d=0,f=0,g=s.createBuffer(1,s.sampleRate*2,s.sampleRate),v=9371,m=g.getChannelData(0);for(let y=0;y<m.length;y++)v=Math.imul(v,1664525)+1013904223|0,m[y]=(v>>>0)/2147483648-1;let p={step:{gain:.05,layers:[{wave:"noise",f:1900,q:1.7,dur:.05,lvl:.5},{wave:"noise",f:520,q:.9,dur:.1,lvl:.4},{wave:"tone",f:104,f2:74,dur:.12,lvl:.3}]},step_carry:{gain:.055,layers:[{wave:"noise",f:1250,q:1.2,dur:.06,lvl:.45},{wave:"noise",f:340,q:.8,dur:.13,lvl:.5},{wave:"tone",f:88,f2:62,dur:.15,lvl:.36}]},glide:{gain:.02,layers:[{wave:"noise",f:700,q:.6,dur:.26,lvl:.8},{wave:"tone",f:150,f2:120,dur:.24,lvl:.18}]},greet:{gain:.038,layers:[{wave:"tone",f:520,f2:740,dur:.11,lvl:.5,type:"triangle"},{wave:"tone",f:790,f2:960,dur:.13,lvl:.3,type:"triangle",delay:.1}]},work:{gain:.034,layers:[{wave:"noise",f:2700,q:2.6,dur:.05,lvl:.6},{wave:"tone",f:240,f2:175,dur:.22,lvl:.26,type:"sawtooth"}]},carry:{gain:.026,layers:[{wave:"noise",f:950,q:.7,dur:.12,lvl:.7},{wave:"noise",f:2300,q:1.5,dur:.07,lvl:.25}]},idle:{gain:.016,layers:[{wave:"noise",f:3400,q:3.2,dur:.028,lvl:.7}]},turn:{gain:.02,layers:[{wave:"tone",f:900,f2:1440,dur:.18,lvl:.4,type:"sawtooth"}]}};function M(y,S,C,_,I){let b=y.startsWith("robot_")?y.slice(6):y,N=p[b];if(!N||!h||o.size>=10)return!1;let w=S-c.x,H=_-c.z,F=Math.hypot(w,H);if(F>45)return!1;let L=s.currentTime;if(I!=null){let be=l.get(I);if(be!=null&&L-be<.25)return!1;l.set(I,L)}let O=N.gain/(1+F*.16)*(u?.5:1);if(O<4e-4)return!1;let V=Math.max(-1,Math.min(1,(w*-c.fz+H*c.fx)/Math.max(F,1))),R=s.createGain(),G=s.createStereoPanner(),oe=s.createGain();R.gain.value=O,G.pan.value=V,oe.gain.value=u?.2:.4,R.connect(G).connect(e),G.connect(oe).connect(t);let k=[],re={release(){if(o.delete(re)){for(let be of k){try{be.stop?.()}catch{}try{be.disconnect()}catch{}}for(let be of[R,G,oe])try{be.disconnect()}catch{}}}};o.add(re);let Ae=null,Le=0;for(let be of N.layers){let Fe=L+(be.delay||0),le=Math.max(be.dur,(be.attack||.006)+.02),ge=s.createGain();ge.gain.setValueAtTime(0,Fe),ge.gain.linearRampToValueAtTime(be.lvl,Fe+(be.attack||.006)),ge.gain.exponentialRampToValueAtTime(1e-4,Fe+le),ge.connect(R);let Re;if(be.wave==="noise"){Re=s.createBufferSource(),Re.buffer=g,Re.loop=!0;let he=s.createBiquadFilter();he.type="bandpass",he.frequency.value=be.f,he.Q.value=be.q||1,Re.connect(he).connect(ge),k.push(he)}else Re=s.createOscillator(),Re.type=be.type||"sine",Re.frequency.setValueAtTime(be.f,Fe),be.f2&&Re.frequency.exponentialRampToValueAtTime(Math.max(20,be.f2),Fe+le),Re.connect(ge);k.push(Re,ge);let de=Fe+le+.02;de>Le&&(Le=de,Ae=Re),Re.start(Fe),Re.stop(de)}return Ae&&(Ae.onended=re.release),d++,!0}function T(y,S,C,_,I,b,N){if(!h)return;let w=a.get(y);if(!w){a.set(y,{x:S,z:_,phase:0});return}let H=S-w.x,F=_-w.z,L=Math.hypot(H,F);if(w.x=S,w.z=_,!I){w.phase=0;return}if(L>8)return;w.phase+=L;let O=N==="glide"?1.6:b?1.05:.8;w.phase<O||(w.phase-=O,M(N==="glide"?"glide":b?"step_carry":"step",S,C,_,y),f++)}function x(){for(let y of[...o])y.release();for(let[,y]of a)y.x=c.x,y.z=c.z,y.phase=0;l.clear()}return{setActive(y){h!==y&&(h=y,y||x())},environment(y){u=!!y},listener(y,S,C,_,I){Object.assign(c,{x:y,y:S,z:C,fx:_,fz:I})},action(y,S,C,_,I){return M(y,S,C,_,I)},gait:T,reset(){a.clear(),l.clear()},stats:()=>({voices:o.size,played:d,steps:f,gaits:a.size,inside:u}),dispose(){h=!1,x()}}}var Ou=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]],Yp=[440,523.25,587.33,659.25,783.99,880,1046.5],Bi=12,Tc={harbour:{x:0,z:79},quay:{x:0,z:72},tower:{x:0,z:-12},square:{x:8,z:40}},eM=[{kind:"chime",weight:15},{kind:"swoosh",weight:13},{kind:"gust",weight:7},{kind:"birds",weight:8,when:s=>s.day>.45&&s.weather==="clear"},{kind:"gull",weight:6,when:s=>s.day>.2&&s.weather==="clear"},{kind:"pigeon",weight:4,when:s=>s.day>.6&&!s.busy},{kind:"cricket",weight:6,when:s=>s.day<.25&&s.weather!=="rain"},{kind:"horn",weight:4,when:s=>s.day<.45&&s.weather!=="rain"},{kind:"bell",weight:3,when:s=>s.day<.7},{kind:"hammer",weight:4,when:s=>s.day>.3},{kind:"drip",weight:10,when:s=>s.weather==="rain"},{kind:"windchime",weight:4,when:s=>s.weather==="clear"&&!s.busy},{kind:"siren",weight:2,when:s=>s.evening>.3&&s.busy}];function tM(s,e,t){let n=Math.floor(s.sampleRate*e),i=s.createBuffer(2,n,s.sampleRate),r=977;for(let o=0;o<2;o++){let a=i.getChannelData(o);for(let l=0;l<n;l++)r=Math.imul(r,1664525)+1013904223|0,a[l]=((r>>>0)/2147483648-1)*Math.pow(1-l/n,t)}return i}function nM(s=()=>{}){let e=null,t=null,n=null,i=!1,r=!1,o=!1,a=0,l=null,c=[],h=[],u=new Float32Array(256),d=new Set,f=.18,g=null,v=null,m=null,p=null,M=null,T=!1,x="clear",y=[],S=null,C=null,_=null,I=null,b=null,N=null,w=null,H=null,F="0.000",L="0.000",O=0,V=0,R=0,G=0,oe={x:0,y:2.4,z:0,fx:0,fz:-1},k={busy:!1,day:0,evening:0,weather:"clear"},re={ambience:1,effects:1,voice:1};try{o=localStorage.getItem("aurago.desktop.sysworld.sound")==="true"}catch{}function Ae(){if(e||i)return;e=new(window.AudioContext||window.webkitAudioContext);let K=nt=>(c.push(nt),nt),te=nt=>(h.push(nt),K(nt));t=K(e.createGain()),t.gain.value=0,n=K(e.createAnalyser()),n.fftSize=512,t.connect(n),n.connect(e.destination),m=K(e.createGain()),m.gain.value=re.ambience,m.connect(t),p=K(e.createGain()),p.gain.value=re.effects,p.connect(t),M=K(e.createGain()),M.gain.value=re.voice,M.connect(t),b=K(e.createConvolver()),b.buffer=tM(e,3.4,2.6);let J=K(e.createGain());J.gain.value=.55,b.connect(J).connect(m),l=Wp(e,M),g=Xp(e,p,m),g.environment(T,x),v=qp(e,p,m),v.environment(T),v.listener(oe.x,oe.y,oe.z,oe.fx,oe.fz);for(let[nt,Pt]of[[55,.07],[82.4069,.022],[110,.01]]){let It=te(e.createOscillator()),vt=K(e.createGain());It.frequency.value=nt,vt.gain.value=Pt,It.connect(vt).connect(m),It.start()}S=K(e.createBiquadFilter()),S.type="lowpass",S.frequency.value=650,S.Q.value=.6,C=K(e.createGain()),C.gain.value=.042;let $=K(e.createGain());$.gain.value=.9,S.connect(C).connect(m),C.connect($).connect(b),y=Ou[0].map(nt=>[-6,6].map(Pt=>{let It=te(e.createOscillator()),vt=K(e.createGain());return It.type="sawtooth",It.frequency.value=nt,It.detune.value=Pt,vt.gain.value=.22,It.connect(vt).connect(S),It.start(),It}));let xe=te(e.createOscillator()),ve=K(e.createGain());xe.frequency.value=.05,ve.gain.value=180,xe.connect(ve).connect(S.frequency),xe.start();let me=e.createBuffer(1,e.sampleRate*4,e.sampleRate),pe=me.getChannelData(0),we=0;for(let nt=0;nt<pe.length;nt++)we=(we+(Math.random()*2-1)*.018)/1.018,pe[nt]=we;let B=pe[pe.length-1]-pe[0];for(let nt=0;nt<pe.length;nt++)pe[nt]-=B*nt/(pe.length-1);let q=te(e.createBufferSource());_=K(e.createBiquadFilter()),I=K(e.createGain()),q.buffer=me,q.loop=!0,_.type="lowpass",_.frequency.value=680,_.Q.value=.25,I.gain.value=.32,q.connect(_).connect(I).connect(m),q.start();let D=te(e.createOscillator()),Y=K(e.createGain());D.frequency.value=.075,Y.gain.value=160,D.connect(Y).connect(_.frequency),D.start(),N=e.createBuffer(1,e.sampleRate*2,e.sampleRate);let A=N.getChannelData(0);for(let nt=0;nt<A.length;nt++)A[nt]=Math.random()*2-1;let z=te(e.createBufferSource());z.buffer=N,z.loop=!0;let ee=K(e.createBiquadFilter());ee.type="bandpass",ee.frequency.value=620,ee.Q.value=.5;let ne=te(e.createOscillator()),Te=K(e.createGain());ne.frequency.value=.08,Te.gain.value=140,ne.connect(Te).connect(ee.frequency),ne.start(),w=K(e.createGain()),w.gain.value=0,z.connect(ee).connect(w).connect(m),z.start();let ye=te(e.createBufferSource());ye.buffer=N,ye.loop=!0;let je=K(e.createBiquadFilter());je.type="bandpass",je.frequency.value=320,je.Q.value=.7;let Je=te(e.createOscillator()),Mt=K(e.createGain());Je.frequency.value=.13,Mt.gain.value=90,Je.connect(Mt).connect(je.frequency),Je.start(),H=K(e.createGain()),H.gain.value=0,ye.connect(je).connect(H).connect(m),ye.start(),ae(),se()}let Le=()=>!!e&&o&&r&&!i&&f>0;function be(K){let te={release(){if(d.delete(te))for(let J of K){try{J.stop?.()}catch{}try{J.disconnect()}catch{}}}};return d.add(te),K[0].onended=te.release,G++,te}function Fe(K,te,J){let $=e.createStereoPanner(),xe=e.createGain();return $.pan.value=te,xe.gain.value=J,K.connect($).connect(m),$.connect(xe).connect(b),[$,xe]}function le(K){let te=K.x-oe.x,J=K.z-oe.z,$=Math.hypot(te,J);return{distance:$,pan:Math.max(-1,Math.min(1,(te*-oe.fz+J*oe.fx)/Math.max($,1))),fade:1/(1+$*.02)}}function ge(K){if(K==="horn"||K==="gull")return Tc.harbour;if(K==="windchime")return Tc.quay;if(K==="bell")return Tc.tower;if(K==="pigeon")return Tc.square;let te=Math.random()*Math.PI*2,J=24+Math.random()*44;return{x:oe.x+Math.sin(te)*J,z:oe.z+Math.cos(te)*J}}function Re(K){let te=e.currentTime,J=Yp[Math.floor(Math.random()*Yp.length)],$=e.createOscillator(),xe=e.createOscillator(),ve=e.createGain(),me=e.createGain(),{pan:pe,fade:we}=le(K);$.frequency.value=J,xe.frequency.value=J*2.76,ve.gain.value=.22,me.gain.setValueAtTime(0,te),me.gain.linearRampToValueAtTime(k.busy?.02:.014,te+.008),me.gain.exponentialRampToValueAtTime(1e-4,te+2.8),$.connect(me),xe.connect(ve).connect(me),be([$,xe,ve,me,...Fe(me,pe,1.4)]),$.start(te),xe.start(te),$.stop(te+2.9),xe.stop(te+2.9)}function de(K){let te=e.currentTime,J=e.createBufferSource(),$=e.createBiquadFilter(),xe=e.createGain(),{pan:ve,fade:me}=le(K);J.buffer=N,J.loop=!0,$.type="bandpass",$.Q.value=1.4,$.frequency.setValueAtTime(260,te),$.frequency.exponentialRampToValueAtTime(1100,te+1.6),$.frequency.exponentialRampToValueAtTime(380,te+3.4),xe.gain.setValueAtTime(0,te),xe.gain.linearRampToValueAtTime(.03*me,te+1.6),xe.gain.exponentialRampToValueAtTime(1e-4,te+3.6),J.connect($).connect(xe);let[pe,we]=Fe(xe,ve,.5),B=Math.max(-1,ve-.6),q=Math.min(1,ve+.6);pe.pan.setValueAtTime(B,te),pe.pan.linearRampToValueAtTime(q,te+3.4),be([J,$,xe,pe,we]),J.start(te),J.stop(te+3.7)}function he(){let K=e.currentTime,te=x==="rain"?.36:.32;_.frequency.cancelScheduledValues(K),I.gain.cancelScheduledValues(K),_.frequency.setTargetAtTime(1200+Math.random()*700,K,.8),_.frequency.setTargetAtTime(680,K+2.6,1.4),I.gain.setTargetAtTime(te*1.7,K,.7),I.gain.setTargetAtTime(te,K+2.6,1.5),G++}function ce(K){let te=e.currentTime,J=e.createBiquadFilter(),$=e.createGain(),{pan:xe,fade:ve}=le(K);J.type="lowpass",J.frequency.value=420,$.gain.setValueAtTime(0,te),$.gain.linearRampToValueAtTime(.028*ve,te+.9),$.gain.setValueAtTime(.028*ve,te+3.2),$.gain.exponentialRampToValueAtTime(1e-4,te+5.5);let me=[69.3,103.83].map(pe=>{let we=e.createOscillator();return we.type="sawtooth",we.frequency.value=pe,we.detune.value=Math.random()*8-4,we.connect(J),we});J.connect($),be([...me,J,$,...Fe($,xe,1.6)]),me.forEach(pe=>{pe.start(te),pe.stop(te+5.6)})}function Se(K){let te=e.currentTime,J=2+Math.floor(Math.random()*3),{pan:$,fade:xe}=le(K);for(let ve=0;ve<J&&d.size<Bi;ve++){let me=te+ve*(.12+Math.random()*.1),pe=e.createOscillator(),we=e.createGain(),B=2600+Math.random()*900;pe.frequency.setValueAtTime(B,me),pe.frequency.exponentialRampToValueAtTime(B*1.5,me+.07),we.gain.setValueAtTime(0,me),we.gain.linearRampToValueAtTime(.006*xe,me+.015),we.gain.exponentialRampToValueAtTime(1e-4,me+.13),pe.connect(we),be([pe,we,...Fe(we,$,.6)]),pe.start(me),pe.stop(me+.15)}}function fe(K){let te=e.currentTime,J=2+Math.floor(Math.random()*3),$=760+Math.random()*280,{pan:xe,fade:ve}=le(K);for(let me=0;me<J&&d.size<Bi;me++){let pe=te+me*(.2+Math.random()*.14),we=e.createOscillator(),B=e.createBiquadFilter(),q=e.createGain();we.type="sawtooth",we.frequency.setValueAtTime($,pe),we.frequency.exponentialRampToValueAtTime($*.5,pe+.26),B.type="bandpass",B.frequency.value=1500,B.Q.value=3.6,q.gain.setValueAtTime(0,pe),q.gain.linearRampToValueAtTime(.012*ve,pe+.04),q.gain.exponentialRampToValueAtTime(1e-4,pe+.34),we.connect(B).connect(q),be([we,B,q,...Fe(q,xe,.9)]),we.start(pe),we.stop(pe+.36)}}function ue(K){let te=e.currentTime,J=2+Math.floor(Math.random()*2),$=420+Math.random()*90,{pan:xe,fade:ve}=le(K);for(let me=0;me<J&&d.size<Bi;me++){let pe=te+me*(.17+Math.random()*.08),we=e.createOscillator(),B=e.createBiquadFilter(),q=e.createGain();we.type="triangle",we.frequency.setValueAtTime($,pe),we.frequency.linearRampToValueAtTime($*.72,pe+.12),B.type="lowpass",B.frequency.value=1100,q.gain.setValueAtTime(0,pe),q.gain.linearRampToValueAtTime(.01*ve,pe+.03),q.gain.exponentialRampToValueAtTime(1e-4,pe+.16),we.connect(B).connect(q),be([we,B,q,...Fe(q,xe,.5)]),we.start(pe),we.stop(pe+.18)}}function Me(K){let te=e.currentTime,J=3+Math.floor(Math.random()*4),$=3900+Math.random()*900,{pan:xe,fade:ve}=le(K);for(let me=0;me<J&&d.size<Bi;me++){let pe=te+me*.055,we=e.createOscillator(),B=e.createGain();we.type="square",we.frequency.value=$,B.gain.setValueAtTime(0,pe),B.gain.linearRampToValueAtTime(.005*ve,pe+.008),B.gain.exponentialRampToValueAtTime(1e-4,pe+.035),we.connect(B),be([we,B,...Fe(B,xe,.35)]),we.start(pe),we.stop(pe+.05)}}function Ee(K){let te=e.currentTime,J=196,{pan:$,fade:xe}=le(K),ve=e.createGain();ve.gain.setValueAtTime(0,te),ve.gain.linearRampToValueAtTime(.02*xe,te+.012),ve.gain.exponentialRampToValueAtTime(1e-4,te+4.2);let me=[1,2.02,2.99,4.18,5.43].map((pe,we)=>{let B=e.createOscillator(),q=e.createGain();return B.type="sine",B.frequency.value=J*pe,q.gain.value=.5/(we+1),B.connect(q).connect(ve),B});be([...me,ve,...Fe(ve,$,1.8)]),me.forEach(pe=>{pe.start(te),pe.stop(te+4.3)})}function Ge(K){let te=e.currentTime,J=2+Math.floor(Math.random()*3),{pan:$,fade:xe}=le(K);for(let ve=0;ve<J&&d.size<Bi;ve++){let me=te+ve*(.5+Math.random()*.35),pe=e.createBufferSource(),we=e.createBiquadFilter(),B=e.createGain(),q=e.createOscillator(),D=e.createGain();pe.buffer=N,pe.loop=!0,we.type="bandpass",we.frequency.value=420,we.Q.value=1.1,B.gain.setValueAtTime(0,me),B.gain.linearRampToValueAtTime(.02*xe,me+.006),B.gain.exponentialRampToValueAtTime(1e-4,me+.42),q.type="sine",q.frequency.value=1180,D.gain.value=.2,pe.connect(we).connect(B),q.connect(D).connect(B),be([pe,we,B,q,D,...Fe(B,$,.9)]),pe.start(me),pe.stop(me+.44),q.start(me),q.stop(me+.44)}}function qe(K){let te=e.currentTime,J=2+Math.floor(Math.random()*4),{pan:$,fade:xe}=le(K);for(let ve=0;ve<J&&d.size<Bi;ve++){let me=te+ve*(.06+Math.random()*.14),pe=e.createOscillator(),we=e.createGain();pe.type="sine",pe.frequency.setValueAtTime(900+Math.random()*700,me),pe.frequency.exponentialRampToValueAtTime(320,me+.06),we.gain.setValueAtTime(0,me),we.gain.linearRampToValueAtTime(.008*xe,me+.005),we.gain.exponentialRampToValueAtTime(1e-4,me+.12),pe.connect(we),be([pe,we,...Fe(we,$,.4)]),pe.start(me),pe.stop(me+.14)}}function ke(K){let te=e.currentTime,J=3+Math.floor(Math.random()*4),$=[1046.5,1174.7,1396.9,1567.98,1760,2093],{pan:xe,fade:ve}=le(K);for(let me=0;me<J&&d.size<Bi;me++){let pe=te+me*(.09+Math.random()*.13),we=$[Math.floor(Math.random()*$.length)],B=e.createOscillator(),q=e.createGain();B.type="sine",B.frequency.value=we,q.gain.setValueAtTime(0,pe),q.gain.linearRampToValueAtTime(.006*ve,pe+.004),q.gain.exponentialRampToValueAtTime(1e-4,pe+.9),B.connect(q),be([B,q,...Fe(q,xe,1.5)]),B.start(pe),B.stop(pe+.95)}}function Ye(K){let te=e.currentTime,{pan:J,fade:$}=le(K),xe=e.createOscillator(),ve=e.createGain();xe.type="sine",xe.frequency.setValueAtTime(420,te),xe.frequency.linearRampToValueAtTime(620,te+1.8),xe.frequency.linearRampToValueAtTime(430,te+3.6),ve.gain.setValueAtTime(0,te),ve.gain.linearRampToValueAtTime(.01*$,te+.5),ve.gain.setValueAtTime(.01*$,te+2.8),ve.gain.exponentialRampToValueAtTime(1e-4,te+4.2),xe.connect(ve),be([xe,ve,...Fe(ve,J,1.2)]),xe.start(te),xe.stop(te+4.3)}let Qe={chime:Re,swoosh:de,gust:he,horn:ce,birds:Se,gull:fe,pigeon:ue,cricket:Me,bell:Ee,hammer:Ge,drip:qe,windchime:ke,siren:Ye};function Q(){let K=eM.filter($=>!$.when||$.when(k));if(!K.length)return"chime";let te=0;for(let $ of K)te+=$.weight;let J=Math.random()*te;for(let $ of K)if(J-=$.weight,J<=0)return $.kind;return K[K.length-1].kind}function st(){if(d.size>=Bi)return;let K=Q(),te=ge(K);Qe[K]?.(te)}function $e(){clearTimeout(O),O=0,Le()&&(O=setTimeout(()=>{O=0,Le()&&(st(),$e())},(k.busy?1900:3600)+Math.random()*9e3))}function X(){clearTimeout(V),V=0,Le()&&(V=setTimeout(()=>{V=0,Le()&&(R=(R+1)%Ou.length,y.forEach((K,te)=>K.forEach(J=>J.frequency.setTargetAtTime(Ou[R][te],e.currentTime,1.8))),X())},16e3))}function E(){clearTimeout(O),clearTimeout(V),O=V=0;for(let K of[...d])K.release()}function ae(){if(!e)return;let K=e.currentTime;S.frequency.setTargetAtTime(k.busy?1500:650+k.day*450+k.evening*150,K,2.5),C.gain.setTargetAtTime((k.busy?.055:.042)*(k.weather==="rain"?.75:1),K,2)}function se(){if(!e||!w||!H)return;let K=Math.max(-76,Math.min(76,oe.x)),te=Math.hypot(oe.x-K,oe.z-77),J=Math.max(0,1-(te-6)/42),$=(T?0:J*.05*(x==="rain"?1.2:1)).toFixed(3),xe=((k.busy?.03:.012)*(.35+k.day*.65)*(T?.25:1)).toFixed(3);if($===F&&xe===L)return;F=$,L=xe;let ve=e.currentTime;w.gain.setTargetAtTime(Number($),ve,.5),H.gain.setTargetAtTime(Number(xe),ve,.8)}function _e(K=!1){if(clearTimeout(a),!i){if(K&&o)try{Ae()}catch{o=!1,s(!1);return}e&&(l?.setActive(o&&r&&f>0&&re.voice>0),g?.setActive(o&&r&&f>0),v?.setActive(o&&r&&f>0&&re.effects>0),o&&r?(e.resume().catch(()=>{}),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(f,e.currentTime,.25),O||$e(),V||X()):(E(),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(0,e.currentTime,.035),a=setTimeout(()=>{!i&&(!o||!r)&&e.suspend().catch(()=>{})},180)))}}return{enabled:()=>o,toggle(){o=!o;try{localStorage.setItem("aurago.desktop.sysworld.sound",String(o))}catch{}_e(!0),s(o)},unlock(){_e(!0)},setActive(K){r!==K&&(r=K,_e())},setVolume(K){Number.isFinite(K)&&(f=Math.max(0,Math.min(.35,K)),_e())},setListener(K,te,J,$,xe){Object.assign(oe,{x:K,y:te,z:J,fx:$,fz:xe}),l?.setListener(K,te,J,$,xe),g?.listener(K,te,J,$,xe),v?.listener(K,te,J,$,xe),se()},setChannel(K,te){if(!(K in re)||!Number.isFinite(te))return;re[K]=Math.max(0,Math.min(1,te));let J={ambience:m,effects:p,voice:M}[K];J&&J.gain.setTargetAtTime(re[K],e.currentTime,.05),_e()},effect(K,te,J,$,xe){if(String(K).startsWith("robot_")){v?.action(K,te,J,$,xe);return}g?.play(K,te,J,$)},gait(K,te,J,$,xe,ve,me){v?.gait(K,te,J,$,xe,ve,me)},setEnvironment(K,te){T=!!K,["clear","rain","fog"].includes(te)&&(x=te),g?.environment(T,x),v?.environment(T),se()},setMood(K={}){Object.assign(k,{busy:!!K.busy,day:Number.isFinite(K.day)?K.day:k.day,evening:Number.isFinite(K.evening)?K.evening:k.evening,weather:["clear","rain","fog"].includes(K.weather)?K.weather:k.weather}),ae(),se()},thunder(K=1){if(!Le()||T||d.size>=Bi)return;let te=Math.max(0,Math.min(5,K)),J=e.currentTime+te,$=Math.min(1,te/4),xe=e.createBufferSource(),ve=e.createBiquadFilter(),me=e.createGain();xe.buffer=N,xe.loop=!0,ve.type="lowpass",ve.Q.value=.7,ve.frequency.setValueAtTime(900-$*500,J),ve.frequency.exponentialRampToValueAtTime(70,J+3.5),me.gain.setValueAtTime(0,J),me.gain.linearRampToValueAtTime(.09*(1-$*.5),J+.06+$*.25),me.gain.exponentialRampToValueAtTime(.025,J+1.3),me.gain.exponentialRampToValueAtTime(1e-4,J+5.5),xe.connect(ve).connect(me),be([xe,ve,me,...Fe(me,Math.random()*.8-.4,.8)]),xe.start(J),xe.stop(J+5.6)},stats(){let K=0;if(n&&e.state==="running"){n.getFloatTimeDomainData(u);for(let te of u)K+=te*te}return{voice:l?.stats(),effects:g?.stats(),foley:v?.stats(),channels:{...re},enabled:o,active:r,state:e?.state||"uninitialized",volume:f,rms:Math.sqrt(K/u.length),soundscape:{chord:R,events:G,transient:d.size,scheduled:!!O,mood:{...k},beds:{water:Number(F),murmur:Number(L)},listener:{x:oe.x,z:oe.z}}}},dispose(){i||(i=!0,E(),v?.dispose(),g?.dispose(),l?.dispose(),clearTimeout(a),h.forEach(K=>{try{K.stop()}catch{}}),c.forEach(K=>K.disconnect()),e&&e.close().catch(()=>{}))}}}var Qt=[{id:"agent",asset:"agent-spire",x:0,z:-12,height:87,radius:13},{id:"infra",asset:"compute-foundry",x:-43,z:-53,height:24,radius:18},{id:"integrations",asset:"integration-gate",x:43,z:-10,height:34,radius:14},{id:"missions",asset:"mission-terminal",x:43,z:36,height:23,radius:15},{id:"memory",asset:"memory-archive",x:-43,z:-10,height:37,radius:14},{id:"graph",asset:"knowledge-atrium",x:-43,z:36,height:28,radius:17},{id:"operations",asset:"operations-beacon",x:0,z:36,height:22,radius:6}],Ec={low:{lod:2,dpr:1,shadow:0,bloom:!1},medium:{lod:1,dpr:1.25,shadow:1024,bloom:!1},high:{lod:0,dpr:1.5,shadow:2048,bloom:!0},ultra:{lod:0,dpr:2,shadow:4096,bloom:!0}},Bu=new W(122,106,183),zu=class extends Dn{constructor(e,t,n){super(),this.scene=e,this.camera=t,this.needsSwap=!1,this.target=new Bt(1,1,{type:Yt,samples:n}),this.quad=new vi(new ct({uniforms:ei.clone(Fi.uniforms),vertexShader:Fi.vertexShader,depthTest:!1,depthWrite:!1,fragmentShader:"uniform sampler2D tDiffuse;varying vec2 vUv;void main(){vec4 c=texture2D(tDiffuse,vUv);if(any(isnan(c))||any(isinf(c)))c=vec4(0.0,0.0,0.0,1.0);gl_FragColor=c;}"}))}render(e,t,n){let i=e.autoClear;e.autoClear=!1,e.setRenderTarget(this.target),e.clear(),e.render(this.scene,this.camera),this.quad.material.uniforms.tDiffuse.value=this.target.texture,e.setRenderTarget(this.renderToScreen?null:n),this.quad.render(e),e.autoClear=i}setSize(e,t){this.target.setSize(e,t)}dispose(){this.target.dispose(),this.quad.material.dispose(),this.quad.dispose()}},zr=new W(0,22,-12),Os=[];{let s=(r,o,a,l=0,c=0,h=[1,1,1])=>Os.push({asset:r,x:o,z:a,y:l,angle:c,scale:h}),{xs:e,zs:t}=wn;for(let r of t){for(let a of e)s("street-crossing",a,r);let o=[wn.minX,...e,wn.maxX];for(let a=0;a<o.length-1;a++){let l=o[a]+(a?6:0),c=o[a+1]-(a<o.length-2?6:0);s("street-tile",(l+c)/2,r,0,0,[(c-l)/16,1,1])}}for(let r of e){let o=[wn.minZ,...t,wn.maxZ];for(let a=0;a<o.length-1;a++){let l=o[a]+(a?6:0),c=o[a+1]-(a<o.length-2?6:0);s("street-tile",r,(l+c)/2,0,Math.PI/2,[(c-l)/16,1,1])}}for(let r of t)for(let o of[-55,-31,31,55])s("street-lamp",o,r-4.5,.45),s("planter",o+4,r-4.5,.45);for(let{x:r,z:o,scale:a}of Sc)s("data-tower-a",r,o,0,0,[1,a,1]);s("skybridge",39.75,-55,15),s("server-rack",-55,-36.7,.5),s("server-rack",-49,-36.7,.5);let n=7919,i=()=>(n=Math.imul(n,1664525)+1013904223>>>0)/4294967296;for(let r=0;r<4;r++)for(let o=0;o<12;o++){let a=i(),l=i(),c=i(),h=i(),u=i(),d=i();if(h<.12)continue;let f=(o-5.5)*22+(a-.5)*12,g=-143-r*29+(l-.5)*14,v=1-Math.min(1,Math.abs(f)/150),m=(.5+c*c+.45*v)*(r?1:.85),p=.7+.34*u;s(a>.5?"data-tower-a":"data-tower-b",f,g,-3,d<.3?Math.PI/2:0,[p,m,p])}for(let r=0;r<16;r++){let o=(r-7.5)*34+(i()-.5)*16,a=-268-i()*46;s(i()<.5?"data-tower-a":"data-tower-b",o,a,-3,0,[1.1,.55+i()*1.1,1.1])}}var iM=[...sp(Os),...un.flatMap(s=>[-3,3].flatMap(e=>[-3,3].map(t=>({x:s.x+e,z:s.z+t,r:4.25,asset:"interior"}))))];async function T1(s,e){let t=!1,n=!0,i="orbit",r=e.quality||"auto",o=r==="auto"?"high":r,a=0,l=null,c=0,h=0,u=0,d=0,f=0,g=0,v=0,m=null,p=e.reducedMotion,M=1,T=1,x=[],y=!1,S=null,C=null,_=new AbortController,I=new Set,b=new Map,N=new Map,w=new Set,H=new Set,F=new Set,L=[],O=new ac({antialias:!0,alpha:!1});O.toneMapping=Cs,O.toneMappingExposure=.98,O.shadowMap.type=dl,O.shadowMap.autoUpdate=!1,O.info.autoReset=!1;let V=O.domElement;V.className="sysworld-gl",V.tabIndex=0,V.setAttribute("aria-label",e.label),s.append(V);let R=new bs;R.background=new Ce(528926),R.fog=new so(528926,.004);let G=new qt(43,1,.3,1800);G.position.copy(Bu);let oe=Mc(),k=oe.register("visitor",{circles:[{x:0,z:0,r:.38}],minY:-.4,maxY:.4,reach:.38},G.position,100),re=new fc(G,V);re.target.copy(zr),re.enableDamping=!0,re.dampingFactor=.085,re.minDistance=12,re.maxDistance=410,re.maxPolarAngle=Math.PI*.485,re.update();let Ae=0,Le=()=>{let P=be?.progress();e.onProgress?.(f+(P?.bytes||0),Ae+(P?.expected||0))},be=Gp(O,{url:P=>e.resourceURL("/3d/system-world/textures/v1/"+P),onProgress:()=>Le(),shelters:un.map(P=>[P.x-P.width/2,P.z-P.depth/2,P.x+P.width/2,P.z+P.depth/2])}),Fe=new Ir(O),le=new pc,ge=Fe.fromScene(le,.05);R.environment=ge.texture,R.environmentIntensity=.48,le.dispose(),Fe.dispose();let Re=new Mo(12639487,1056813,.8);R.add(Re);let de=new Ni(13164287,3.2);de.position.set(-70,145,85),de.castShadow=!0,Object.assign(de.shadow.camera,{left:-108,right:108,top:108,bottom:-108,near:1,far:360}),de.shadow.normalBias=.09,de.shadow.bias=-15e-5,de.shadow.radius=2.4,R.add(de);let he=new Ni(16758130,2.4);he.position.set(90,80,-120),R.add(he);let ce=new Ni(14195306,.42);ce.position.set(-10,-40,30),R.add(ce);let Se=new jn(6479871,0,75,2);Se.position.set(0,34,-12),R.add(Se);let fe=new Bt(1,1,{type:Yt}),ue=new gc(O,fe);ue.addPass(new zu(R,G,Math.min(4,O.capabilities.maxSamples)));let Me=new Fr(new Ue(1,1),.28,.68,1.06);ue.addPass(Me),ue.addPass(new xc);let Ee=new lt,Ge=new lt,qe=new lt;R.add(Ee),Ee.add(Ge,qe);function ke(P,Z){return w.add(P),H.add(Z),new Ke(P,Z)}let Ye=ke(new ui(170,3,174),new vn({color:1911345,metalness:.05,roughness:.82}));Ye.position.set(0,-1.5,-7),Ye.receiveShadow=!0,Ee.add(Ye),be.apply(Ye.material,"world","ground");let Qe=yp(R,{sunDirection:de.position,lamps:Os.filter(P=>P.asset==="street-lamp")});ue.addPass(Qe.post);let Q=bp(R,{anchors:Qt.map(P=>({id:P.id,x:P.x,z:P.z,height:P.height}))}),st=Qt.find(P=>P.id==="memory"),$e=up(R,st,{roof:21,label:e.memoryLabel}),X=wp(R,{traffic:oe,surfaces:be}),E=kp(R,{sun:de,rim:he,hemisphere:Re,atmosphere:Qe,surfaces:be,onThunder:P=>e.onThunder?.(P)}),ae="",se=null,_e={value:0},K=new Qi(1,1.045,96);w.add(K);let te=(P,Z)=>{let j=new ct({transparent:!0,depthWrite:!1,side:Rt,uniforms:{color:{value:new Ce(9365742)},time:_e,opacity:{value:P},dashes:{value:Z}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float time,opacity,dashes;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;float dash=dashes>0.5?step(0.3,fract(a*dashes-time*0.35)):1.0;gl_FragColor=vec4(color,opacity*dash*(0.8+0.2*sin(time*2.2)));}"});return H.add(j),j},J=new Ke(K,te(.9,18)),$=new Ke(K,te(.38,0));for(let P of[J,$])P.rotation.x=-Math.PI/2,P.visible=!1,Ee.add(P);let xe=new ct({transparent:!0,depthWrite:!1,side:Rt,uniforms:{color:{value:new Ce(9365742)},alpha:{value:0}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float alpha;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;gl_FragColor=vec4(color,alpha*(0.55+0.45*sin(a*3.0)));}"});H.add(xe);let ve=new Ke(new Qi(.9,1,96),xe);w.add(ve.geometry),ve.rotation.x=-Math.PI/2,ve.visible=!1,Ee.add(ve);let me=2,pe={error:16742504,running:7730385,stale:8096667,idle:8175587},we={},B=()=>J.material.uniforms.color.value.setHex(pe[we[m]]??9365742),q=new wt({color:6742230,toneMapped:!1}),D=new _n(new di(.65,10,6),q,Qt.length);w.add(D.geometry),H.add(q),Ee.add(D);let Y=new Tt,A=new W,z=new Ao,ee=new Ue;Qt.forEach((P,Z)=>{Y.position.set(P.x,P.height+2,P.z),Y.updateMatrix(),D.setMatrixAt(Z,Y.matrix),D.setColorAt(Z,new Ce(8425630))}),e.signal?.addEventListener("abort",U,{once:!0});let ne;try{if(e.signal?.aborted)throw Error("Disposed");let P=await fetch(e.assetURL("manifest.json"),{signal:_.signal});if(!P.ok)throw Error("City manifest unavailable");ne=await P.json()}catch(P){throw U(),P}let Te=new Map(ne.assets.map(P=>[P.id,P]));Hp(oe,Te,Qt,Os),se=Bp(R,{camera:G,districts:Qt,tier:o,traffic:oe,reduced:()=>p,active:()=>n&&!y&&i!=="map"&&!p&&!e.replaying?.(),assetURL:P=>e.resourceURL("/3d/system-world/v2/"+P),surfaces:be,onInteraction:e.onInteraction,onScope:P=>e.onScope?.(P),onDiscover:e.onDiscover,onSound:e.onSound,onGait:e.onGait,onTerminal:e.onTerminal,onSociety:e.onSociety,onEnvironment:P=>{E.setIndoor(P),e.onEnvironment?.(P)},onError:e.onError,onReady:()=>{O.shadowMap.needsUpdate=!0}});async function ye(P,Z){let j=P+":"+Z;return b.has(j)||b.set(j,(async()=>{let ie=Te.get(P)?.lods.find(Pe=>Pe.level===Z);if(!ie||!/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(ie.file))throw Error("Invalid city asset");let Ie=new AbortController;I.add(Ie);let De=setTimeout(()=>Ie.abort(),15e3);try{let Pe=await fetch(e.assetURL(ie.file),{signal:Ie.signal});if(!Pe.ok)throw Error("City asset unavailable");let Oe=await Pe.arrayBuffer();if(t)throw Error("Disposed");let Ne=await new as().parseAsync(Oe,"");if(t)throw Ne.scene.traverse(He=>{He.isMesh&&(He.geometry.dispose(),He.material.dispose())}),Error("Disposed");return f+=Oe.byteLength,Le(),Ne.scene.traverse(He=>{if(!He.isMesh)return;w.add(He.geometry),He.castShadow=!0,He.receiveShadow=!0;let Be=(Array.isArray(He.material)?He.material:[He.material]).map(We=>{if(N.has(We.name)){let at=We;We=N.get(We.name),at.dispose()}else N.set(We.name,We),H.add(We);return We});He.material=Array.isArray(He.material)?Be:Be[0],be.prepare(He)}),Ne.scene}finally{clearTimeout(De),I.delete(Ie)}})().catch(ie=>{throw b.delete(j),ie})),b.get(j)}function je(P){P.traverse(Z=>{Z.isInstancedMesh&&Z.dispose()}),P.clear()}async function Je(){let P=++a,Z=Ec[o],j=Z.lod,ie=[...new Set([...Os.map(De=>De.asset),...Qt.map(De=>De.asset),"service-drone"])],Ie=(De,Pe)=>Te.get(De)?.lods.find(Oe=>Oe.level===Pe)?.bytes||0;Ae=ie.reduce((De,Pe)=>De+(b.has(Pe+":"+j)?0:Ie(Pe,j)),f)+(j===2?0:["data-tower-a","data-tower-b"].reduce((De,Pe)=>De+(b.has(Pe+":2")?0:Ie(Pe,2)),0));try{let De=new Map(await Promise.all(ie.map(async ze=>[ze,await ye(ze,j)]))),Pe=new Map(await Promise.all(["data-tower-a","data-tower-b"].map(async ze=>[ze,await ye(ze,2)])));if(t||P!==a)return;je(Ge),je(qe),x=[];let Oe=new Map;for(let ze of Os){let Be=(ze.z<-100?Pe:De).get(ze.asset);Be.updateMatrixWorld(!0);let We=new it().compose(new W(ze.x,ze.y,ze.z),new Ht().setFromAxisAngle(new W(0,1,0),ze.angle),new W(...ze.scale));Be.traverse(at=>{if(!at.isMesh)return;let yt=at.uuid;Oe.has(yt)||Oe.set(yt,{node:at,matrices:[]}),Oe.get(yt).matrices.push(new it().multiplyMatrices(We,at.matrixWorld))})}for(let{node:ze,matrices:Be}of Oe.values()){let We=new _n(ze.geometry,ze.material,Be.length);Be.forEach((at,yt)=>We.setMatrixAt(yt,at)),We.castShadow=!0,We.receiveShadow=!0,Ge.add(We)}for(let ze of Qt){let Be=De.get(ze.asset).clone(!0);Be.position.set(ze.x,0,ze.z),Be.userData.district=ze.id,qe.add(Be),x.push(Be)}C?.attachLandmarks(qe),X.setTemplate(De.get("service-drone"));let Ne=new Zt,He=new Map;for(let ze of["data-tower-a","data-tower-b"])He.set(ze,Ne.setFromObject(Pe.get(ze)).max.y);Qe.setAviation(Os.filter(ze=>He.has(ze.asset)&&(ze.z<-100||ze.scale[1]>1.1)).map(ze=>({x:ze.x,y:ze.y+He.get(ze.asset)*ze.scale[1]+.6,z:ze.z}))),O.shadowMap.needsUpdate=!0,e.onReady?.()}catch(De){!t&&P===a&&e.onError?.(De)}}function Mt(){if(t)return;M=Math.max(1,s.clientWidth),T=Math.max(1,s.clientHeight);let P=Math.min(devicePixelRatio||1,Ec[o].dpr,Math.sqrt(3840*2160/(M*T)));O.setPixelRatio(P),O.setSize(M,T,!1),ue.setPixelRatio(P),ue.setSize(M,T),G.aspect=M/T,G.updateProjectionMatrix(),$e.setPointScale(T*P),Qe.setPointScale(T*P),Q.setPointScale(T*P)}function nt(P){return r=P in Ec||P==="auto"?P:"auto",o=r==="auto"?"high":r,Pt(),Je()}function Pt(){let P=Ec[o];O.shadowMap.enabled=P.shadow>0,P.shadow&&de.shadow.mapSize.x!==P.shadow&&(de.shadow.mapSize.set(P.shadow,P.shadow),de.shadow.map?.dispose(),de.shadow.map=null),O.shadowMap.needsUpdate=!0,Me.enabled=P.bloom,be.setTier(o),Qe.setTier(o),se.setTier(o),E.setTier(o),X.setTier(o),Q.setTier(o),Mt(),e.onQuality?.(r,o)}function It(P,Z){l=null;let j=oe.findFree(k,P);if(!j)return;if(P=new W(j.x,j.y,j.z),p){G.position.copy(P),Object.assign(k,j),re.target.copy(Z),re.update(),l=null;return}let ie=(Oe,Ne)=>oe.clear(k,{...Oe,heading:0},{...Ne,heading:0},!1),Ie=G.position.clone(),De=Math.max(Ie.y,P.y,110),Pe=[Ie,P];if(!ie(Ie,P)){let Oe=[Ie],Ne=new W(P.x,De,P.z),He=[-1],ze=[0],Be=-1;if(!ie(Ne,P))return;for(let We of[3,8,16])for(let at=0;at<8;at++)Oe.push(new W(Ie.x+Math.cos(at*Math.PI/4)*We,Ie.y,Ie.z+Math.sin(at*Math.PI/4)*We));for(let We of un)if(Math.hypot(Ie.x-We.x,Ie.z-We.z)<24)for(let at of[We.z,We.doorZ-We.front*2,We.doorZ+We.front*3])Oe.push(new W(We.x,Ie.y,at));for(;ze.length;){let We=ze.shift(),at=Oe[We],yt=new W(at.x,De,at.z);if(ie(at,yt)&&ie(yt,Ne)){Be=We;break}for(let xt=1;xt<Oe.length;xt++)He[xt]===void 0&&at.distanceTo(Oe[xt])<=20&&ie(at,Oe[xt])&&(He[xt]=We,ze.push(xt))}if(Be<0)return;Pe=[];for(let We=Be;We>=0;We=He[We])Pe.unshift(Oe[We]);Pe.push(new W(Oe[Be].x,De,Oe[Be].z),Ne,P)}l={start:G.position.clone(),targetStart:re.target.clone(),end:P.clone(),target:Z.clone(),time:0,waypoints:Pe}}function vt(){oe.relocate(k,G.position),G.position.set(k.x,k.y,k.z)}function dn(P){let Z=Qt.find(j=>j.id===P);Z&&(m=P,J.position.set(Z.x,.55,Z.z),J.scale.setScalar(Z.radius*1.3),J.visible=!0,B(),ve.position.set(Z.x,.62,Z.z),ve.scale.setScalar(Z.radius*1.3*.35),xe.uniforms.color.value.setHex(pe[we[P]]??9365742),xe.uniforms.alpha.value=0,me=0,ve.visible=!p,$.visible=!!ii&&ii!==m,i==="street"?(e.onCut?.(),G.position.set(Z.x,2.4,Z.z+Z.radius+7),G.lookAt(Z.x,Z.height*.4,Z.z),vt()):It(new W(Z.x+Z.radius*2.7,Z.height*.7+18,Z.z+Z.radius*4),new W(Z.x,Z.height*.4,Z.z)))}function ni(P){se.endRide(),se.society.suspend(),F.clear(),l=null,i=P,re.enabled=i==="orbit"||i==="tour",k.minY=i==="street"?-2.3:-.4,k.ignore=k.follow=null,k.circles[0].r=k.reach=i==="street"?.24:.38,document.pointerLockElement===V&&document.exitPointerLock(),i==="street"?(e.onCut?.(),Hr(),G.position.set(18,2.4,57),G.lookAt(0,26,-12),vt()):i!=="map"&&(vt(),It(Bu.clone().multiplyScalar(G.aspect<1?1.3:1),zr)),g=0,v=0,e.onMode?.(i)}let rn=()=>{l=null,i==="tour"&&(i="orbit",e.onMode?.(i))};re.addEventListener("start",rn);function sn(P,Z,j,ie){P.addEventListener(Z,j,ie),L.push(()=>P.removeEventListener(Z,j,ie))}let zt=null,hs=!1,zi=new xn(0,0,0,"YXZ");function ta(P,Z){let j=.0025*G.fov/43;zi.setFromQuaternion(G.quaternion),zi.y-=P*j,zi.x=_t.clamp(zi.x-Z*j,-1.35,1.35),G.quaternion.setFromEuler(zi)}sn(V,"pointerdown",P=>{V.focus({preventScroll:!0}),rn(),zt=[P.clientX,P.clientY],hs=!1,i==="street"&&V.setPointerCapture(P.pointerId)});let ii=null,kr=-1e9;function us(P){if(P===ii)return;ii=P,V.style.cursor=P?"pointer":"";let Z=Qt.find(j=>j.id===P);$.visible=!!Z&&P!==m,Z&&($.position.set(Z.x,.56,Z.z),$.scale.setScalar(Z.radius*1.3)),e.onHover?.(P)}function Hr(){us(null)}function na(P){if(i==="street"||i==="map"||zt||P.timeStamp-kr<80)return;kr=P.timeStamp;let Z=V.getBoundingClientRect();ee.set((P.clientX-Z.left)/Z.width*2-1,1-(P.clientY-Z.top)/Z.height*2),z.setFromCamera(ee,G);let j=z.intersectObjects(x,!0)[0]?.object;for(;j&&!j.userData.district;)j=j.parent;us(j?.userData.district||null)}sn(V,"pointerleave",Hr),sn(V,"pointermove",P=>{i==="street"&&(document.pointerLockElement===V||zt)&&ta(P.movementX,P.movementY),na(P),zt&&Math.hypot(P.clientX-zt[0],P.clientY-zt[1])>5&&(hs=!0)}),sn(V,"pointerup",P=>{if(zt&&!hs&&i!=="street"){let Z=V.getBoundingClientRect();ee.set((P.clientX-Z.left)/Z.width*2-1,1-(P.clientY-Z.top)/Z.height*2),z.setFromCamera(ee,G);let j=z.intersectObjects(x,!0)[0];if(j){let ie=j.object;for(;ie&&!ie.userData.district;)ie=ie.parent;ie&&e.onSelect?.(ie.userData.district)}}zt=null}),sn(V,"pointercancel",()=>{zt=null,F.clear()}),sn(V,"keydown",P=>{if(P.key==="Escape"){se.scoping()?se.endRide():ni("orbit"),P.preventDefault();return}if(i==="street"&&["+","=","-"].includes(P.key)&&se.zoom(P.key==="-"?240:-240)){P.preventDefault();return}if(i==="street"&&P.code==="KeyE"&&!P.repeat){se.interact(),P.preventDefault();return}i==="street"&&["KeyW","KeyA","KeyS","KeyD","ArrowUp","ArrowDown","ArrowLeft","ArrowRight","ShiftLeft"].includes(P.code)&&(F.add(P.code),P.preventDefault(),P.stopPropagation())}),sn(V,"keyup",P=>F.delete(P.code)),sn(V,"wheel",P=>{i==="street"&&se.zoom(P.deltaY)&&P.preventDefault()},{passive:!1}),sn(V,"blur",()=>F.clear()),sn(window,"blur",()=>{F.clear(),zt=null}),sn(V,"webglcontextlost",P=>{P.preventDefault(),y=!0,F.clear(),e.onContextLost?.()}),S=new ResizeObserver(Mt),S.observe(s);function Ac(P){let Z=(F.has("ShiftLeft")?25:11)*P,j=Number(F.has("KeyW")||F.has("ArrowUp"))-Number(F.has("KeyS")||F.has("ArrowDown")),ie=Number(F.has("KeyD")||F.has("ArrowRight"))-Number(F.has("KeyA")||F.has("ArrowLeft"));if(!j&&!ie||se.walkRide(j,P,ie))return;let Ie=Z/Math.hypot(j,ie);G.getWorldDirection(A),A.y=0,A.normalize();let De=(A.x*j-A.z*ie)*Ie,Pe=(A.z*j+A.x*ie)*Ie,Oe=(Ne,He)=>se.move(Ne,He,G.position)&&oe.clear(k,k,{x:Ne,y:2.4+se.floor(Ne,He),z:He,heading:0});Oe(G.position.x+De,G.position.z)&&(G.position.x+=De),Oe(G.position.x,G.position.z+Pe)&&(G.position.z+=Pe),se.isRiding()||(G.position.y=2.4+se.floor(G.position.x,G.position.z))}let ia=0,sa=op(P=>{oe.begin(),ia+=p?0:P,i==="street"&&Ac(P),se.update(P,!p,i);let Z=k.follow;k.follow=se.rideBody(),k.ignore=k.follow||Z,C?.update(P,!p),X.update(P,ia,!p),oe.propose(k,{...G.position,heading:0},(j,ie)=>G.position.set(ie.x,ie.y,ie.z)),oe.solve(P),se.syncRide?.(),k.follow&&Object.assign(k,{x:G.position.x,y:G.position.y,z:G.position.z}),k.ignore=k.follow});function Rc(P,Z){if(t||!n||y||i==="map")return;let j=null,ie=0,Ie=null;if(i!=="street"){if(i==="tour"&&(g-=P,g<=0)){let Ne=Qt[v++%Qt.length].id;dn(Ne),e.onTourFocus?.(Ne),g=7}if(l){j=l,ie=l.time,l.time+=P;let Ne=Math.min(1,l.time/(l.duration||(l.waypoints.length===2?1.1:3.2))),He=Ne*Ne*(3-2*Ne),ze=Math.min(l.waypoints.length-1-1e-6,He*(l.waypoints.length-1)),Be=Math.floor(ze);G.position.lerpVectors(l.waypoints[Be],l.waypoints[Be+1],ze-Be),re.target.lerpVectors(l.targetStart,l.target,He),G.lookAt(re.target),Ie=G.position.clone(),Ne===1&&(l=null)}else re.update()}sa(Math.min(.1,Math.max(0,P)))||(oe.begin(),oe.propose(k,{...G.position,heading:0},(Ne,He)=>G.position.set(He.x,He.y,He.z)),oe.solve(0)),j&&(G.position.distanceToSquared(Ie)>1e-6?(j.wait=(j.wait||0)+P,j.time=ie,l=j.wait<6?j:null):j.wait=0),G.getWorldDirection(A),e.onListener?.(G.position.x,G.position.y,G.position.z,A.x,A.z);let De=!!e.busy?.();E.update(P,Z,!p)&&(O.shadowMap.needsUpdate=!0),Se.intensity=p?0:De?180+Math.sin(Z*2)*35:0,Qe.setBusy(De),Qe.setCinematic(i==="tour"),Qe.update(P,Z,G,!p),be.update(P,!p);let Pe=E.mood();if(Q.update(P,!p,1-(Pe.day||0)),!p){if(me<1.4){me+=P;let Ne=Math.min(1,me/1.4),He=Qt.find(ze=>ze.id===m);He&&(ve.scale.setScalar(He.radius*1.3*(.35+Ne*1.05)),xe.uniforms.alpha.value=(1-Ne)*.55),Ne>=1&&(ve.visible=!1)}for(let Ne=0;Ne<Qt.length;Ne++){let He=Qt[Ne];Y.position.set(He.x,He.height+2,He.z),Y.scale.setScalar(1+Math.sin(Z*1.7+Ne*1.13)*.13),Y.updateMatrix(),D.setMatrixAt(Ne,Y.matrix)}D.instanceMatrix.needsUpdate=!0}p||(_e.value+=Math.min(.1,Math.max(0,P)));let Oe=[De,Pe.day.toFixed(1),Pe.evening.toFixed(1),Pe.weather].join();if(Me.strength=.2+.15*(1-Pe.day),O.toneMappingExposure=.95+.05*(1-Pe.day),Oe!==ae&&(ae=Oe,e.onMood?.({busy:De,day:Pe.day,evening:Pe.evening,weather:Pe.weather})),$e.update(P,G,!p),O.info.reset(),ue.render(),c++,r==="auto"&&P>0&&P<.2&&Z-d>12&&(h+=P,u++,u>=180)){let Ne=h/u,He=["low","medium","high"],ze=He.indexOf(o),Be=Ne>.028&&ze>0?He[ze-1]:Ne<.017&&ze<2?He[ze+1]:o;u=0,h=0,Be!==o&&(o=Be,d=Z,Pt(),Je())}}function ra(P,Z){C?.setData(P,Z),Qt.forEach((j,ie)=>{let Ie=P.find(De=>De.id===j.id);we[j.id]=!Ie||Ie.stale?"stale":Ie.state==="error"?"error":Ie.state==="running"?"running":"idle",D.setColorAt(ie,new Ce(pe[we[j.id]]))}),D.instanceColor.needsUpdate=!0,B()}function U(){t||(t=!0,a++,_.abort(),I.forEach(P=>P.abort()),F.clear(),document.pointerLockElement===V&&document.exitPointerLock(),e.signal?.removeEventListener("abort",U),L.forEach(P=>P()),S?.disconnect(),re.dispose(),C?.dispose(),se?.dispose(),E.dispose(),$e.dispose(),X.dispose(),oe.dispose(),Qe.dispose(),be.dispose(),Q.dispose(),je(Ge),je(qe),D.dispose(),w.forEach(P=>P.dispose()),H.forEach(P=>P.dispose()),ue.passes.forEach(P=>P.dispose?.()),ue.dispose(),ge.dispose(),de.shadow.dispose(),O.dispose(),O.forceContextLoss(),V.remove(),b.clear(),N.clear())}C=lp(R,Qt,{traffic:oe,society:se.society,robotURL:e.resourceURL("/3d/system-world/white-robot.glb"),signal:e.signal,onError:e.onRobotError,active:()=>n&&!y&&i!=="map"&&!e.replaying?.(),obstacles:iM}),$e.setReducedMotion(!!p);try{Pt(),await Je()}catch(P){throw U(),P}return{districts:Qt,canvas:V,update:Rc,focus(P){rn(),dn(P)},setMode:ni,setQuality:nt,setData:ra,dispose:U,interact(){se.interact()},socialAction:se.socialAction,visit(P){ni("street"),se.visit(P),vt()},enter(P){ni("street"),se.destination(P),vt()},setEnvironment(P){E.set(P),O.shadowMap.needsUpdate=!0},setVisible(P){n=P,P||(sa(0),se.suspend(),C?.update(0,!1),F.clear(),zt=null,document.pointerLockElement===V&&document.exitPointerLock())},setReducedMotion(P){p=P,$e.setReducedMotion(!!P),P&&(l=null,se.endRide(),se.society.suspend()),P&&i==="tour"&&ni("orbit")},setHologram(P,Z){$e.setTexts(P,Z),se.setMemory(P)},setWorld(P,Z){se.setWorld(P,Z)},moveKey(P,Z){Z?F.add(P):F.delete(P)},lockPointer(){if(i==="street")return V.requestPointerLock()},project(P){let Z=Qt.find(ie=>ie.id===P);if(!Z)return null;A.set(Z.x,Z.height+4,Z.z);let j=G.position.distanceTo(A);return A.project(G),{x:(A.x+1)*M/2,y:(1-A.y)*T/2,visible:A.z<1&&A.z>-1,distance:j}},capture(){return t||y?Promise.resolve(null):(ue.render(),new Promise(P=>V.toBlob(P,"image/png")))},intro(){if(p||i!=="orbit"||t)return;let P=Bu.clone().multiplyScalar(G.aspect<1?1.3:1);G.position.copy(P).sub(zr).applyAxisAngle(new W(0,1,0),-.55).multiplyScalar(1.22).add(zr),G.position.y+=34,G.lookAt(zr),vt(),It(P,zr),l&&(l.duration=3.6)},stats(){return{flying:!!l,traffic:oe.stats(),experience:se.stats(),weather:E.stats(),life:C?.stats(),hologram:$e.stats(),atmosphere:Qe.stats(),surfaces:be.stats(),drones:X.stats(),flow:Q.stats(),frames:c,tier:o,mode:i,focusedDistrict:m,hovered:ii,loadedBytes:f+se.stats().bytes+be.stats().bytes,cachedModels:b.size,calls:O.info.render.calls,triangles:O.info.render.triangles,geometries:O.info.memory.geometries,position:G.position.toArray(),renderer:O.getContext().getParameter(O.getContext().getExtension("WEBGL_debug_renderer_info")?.UNMASKED_RENDERER_WEBGL||O.getContext().RENDERER),disposed:t}}}}export{T1 as createCity,nM as createCityAmbience,Qt as districts,iM as obstacles,Os as placements};
/*! Bundled license information:

three/build/three.core.js:
three/build/three.module.js:
  (**
   * @license
   * Copyright 2010-2026 Three.js Authors
   * SPDX-License-Identifier: MIT
   *)
*/
